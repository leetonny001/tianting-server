// Package admin 是给业务服务端和 console-server 调用的管理接口（只在管理端口上）：
// 调用方签名鉴权 + scope（PRD-admin F-KEY-02、03），旧密钥 X-Admin-Key 只能访问 v1.2 已有接口（F-KEY-05）。
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/auth"
	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/sessions"
	"tianting/server/internal/store"
)

const (
	defaultTokenTTL = 7 * 24 * time.Hour
	maxTokenTTL     = 30 * 24 * time.Hour
	minTokenTTL     = 60 * time.Second // 非测试模式的下限（PRD C-13）
	minTestTokenTTL = time.Second      // 测试模式的下限（PRD 6.1 节）
	maxGroupMembers = 2000             // PRD F-GRP-01 规则 2
	maxMemberBatch  = 500              // 加 / 移除成员接口单次上限（建群不受此限，C-04）
	maxGroupNameLen = 50               // 去首尾空白后按码点计数（C-38）
	maxImportBatch  = 500
	maxNicknameLen  = 32
	maxAvatarLen    = 512
	demoTokenTTL    = 24 * time.Hour // PRD C-21
)

// Admin 注册 /admin/* 路由。
type Admin struct {
	auth     AuthConfig
	callers  map[string]*adminauth.Caller
	routes   []RouteInfo
	signer   *auth.Signer
	store    store.Store
	router   delivery.Router
	inv      cache.Invalidator
	notifier ConvNotifier
	social   SocialService // nil 时不注册社交路由
	sessions sessions.Source
	faults   TestFaults
	jobs     JobRunner
	mod      *ModerationConfig // 第二批开启时非 nil（IM_V13_ADMIN_FEATURES）
	pusher   PushService       // 系统推送开启时非 nil（IM_PUSH_ENABLED）
	clock    clock.Clock
	log      *slog.Logger
	testMode bool
	// auditFail 由测试接口 /admin/test/faults 打开，模拟 admin_audit 写失败（AC-KEY-04-2）。
	auditFail atomic.Bool
}

// TestFaults 是测试模式下 /admin/test/faults 能打开的节点级故障（由 app.App 实现）。
type TestFaults interface {
	SetOpsDelay(ms int64)
	SetPingFail(fail bool)
}

// JobRunner 手动触发定时任务（测试模式 POST /admin/test/jobs/{name}/run，BE-220）。
type JobRunner interface {
	Run(ctx context.Context, name string) error
}

// WithJobs 接上定时任务。
func (a *Admin) WithJobs(j JobRunner) { a.jobs = j }

// WithTestFaults 接上节点级故障注入。
func (a *Admin) WithTestFaults(f TestFaults) { a.faults = f }

// ConvNotifier 在群成员变动后给相关用户下发 convAdded / convRemoved（由 logic.Service 实现）。
type ConvNotifier interface {
	NotifyConvAdded(ctx context.Context, convID string, userIDs []string)
	NotifyConvRemoved(convID string, userIDs []string, reason string)
	// NotifyGroupChanged 提醒在线成员群资料或成员变了（M4，合并发送）；DeliverTip 投递管理接口产生的 ownerChanged。
	NotifyGroupChanged(groupID string)
	DeliverTip(ctx context.Context, tip *store.Message)
}

// inv 用于在群成员变动后通知各节点的成员缓存失效（logic.TopicGroupMembers）；
// notifier 用于给被加入 / 移出的用户下发会话事件。
func New(ac AuthConfig, signer *auth.Signer, st store.Store, router delivery.Router, inv cache.Invalidator, notifier ConvNotifier, clk clock.Clock, testMode bool, log *slog.Logger) *Admin {
	if ac.Nonces == nil {
		ac.Nonces = adminauth.NewMemoryNonces(defaultNonceCapacity)
	}
	callers := make(map[string]*adminauth.Caller, len(ac.Callers))
	for i := range ac.Callers {
		callers[ac.Callers[i].ID] = &ac.Callers[i]
	}
	return &Admin{auth: ac, callers: callers, signer: signer, store: st, router: router, inv: inv, notifier: notifier, clock: clk, testMode: testMode, log: log}
}

const defaultNonceCapacity = 1_000_000 // PRD-admin F-KEY-03 规则 5

// Register 把管理路由挂到 mux 上。
func (a *Admin) Register(mux *http.ServeMux) {
	a.route(mux, "POST", "/admin/users/token", adminauth.ScopeTokenIssue, a.issueToken)
	a.route(mux, "POST", "/admin/users/import", adminauth.ScopeUserWrite, a.importUsers)
	a.route(mux, "GET", "/admin/users/{userId}", adminauth.ScopeUserRead, a.getUser)
	a.route(mux, "POST", "/admin/users/{userId}/force-logout", adminauth.ScopeUserKick, a.forceLogout)
	a.route(mux, "POST", "/admin/groups", adminauth.ScopeGroupWrite, a.createGroup)
	a.route(mux, "GET", "/admin/groups/{groupId}", adminauth.ScopeGroupRead, a.getGroup)
	a.route(mux, "PATCH", "/admin/groups/{groupId}", adminauth.ScopeGroupWrite, a.updateGroup)
	a.route(mux, "POST", "/admin/groups/{groupId}/members", adminauth.ScopeGroupWrite, a.addMembers)
	a.route(mux, "DELETE", "/admin/groups/{groupId}/members", adminauth.ScopeGroupWrite, a.removeMembers)
	a.route(mux, "DELETE", "/admin/groups/{groupId}", adminauth.ScopeGroupDissolve, a.dissolveGroup)
	a.route(mux, "GET", "/admin/audit", adminauth.ScopeAuditRead, a.queryAudit)
	a.registerSocial(mux)
	a.registerQueries(mux)
	a.registerModeration(mux)
	a.registerPush(mux)
}

// RegisterTestRoutes 注册故障注入和时钟接口，只能在测试模式下调用（PRD 6.1 节）。
func (a *Admin) RegisterTestRoutes(mux *http.ServeMux, faults *logic.Faults, clk *clock.Settable) {
	a.route(mux, "POST", "/admin/test/drop-ack", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientMsgID string `json:"clientMsgId"`
		}
		if readJSON(w, r, &req) {
			faults.DropAck(req.ClientMsgID)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	a.route(mux, "POST", "/admin/test/skip-delivery", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientMsgID string `json:"clientMsgId"`
		}
		if readJSON(w, r, &req) {
			faults.SkipDelivery(req.ClientMsgID)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	a.route(mux, "POST", "/admin/test/faults", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AdminAuditWrite string `json:"adminAuditWrite"` // "fail" 打开，"" 关闭
			OpsDelayMs      *int64 `json:"opsDelayMs"`      // 运维接口人为延迟（PRD-ops S-OPS-24）
			StorePing       string `json:"storePing"`       // "fail" 让就绪检查失败，"" 恢复
		}
		if readJSON(w, r, &req) {
			a.auditFail.Store(req.AdminAuditWrite == "fail")
			if a.faults != nil {
				a.faults.SetPingFail(req.StorePing == "fail")
				if req.OpsDelayMs != nil {
					a.faults.SetOpsDelay(*req.OpsDelayMs)
				}
			}
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	a.route(mux, "POST", "/admin/test/clock", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NowMs *int64 `json:"nowMs"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if req.NowMs == nil {
			clk.Unfreeze()
		} else {
			clk.Freeze(time.UnixMilli(*req.NowMs))
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
	a.route(mux, "POST", "/admin/test/invalidate", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Topic string `json:"topic"`
			Key   string `json:"key"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if l, ok := a.inv.(interface{ InvalidateLocal(topic, key string) }); ok {
			l.InvalidateLocal(req.Topic, req.Key) // 其他进程的广播：只在本机生效，不再转发
		} else {
			a.inv.Invalidate(req.Topic, req.Key)
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
	a.route(mux, "POST", "/admin/test/jobs/{name}/run", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		if a.jobs == nil {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "no scheduler"))
			return
		}
		if err := a.jobs.Run(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "job %s: %v", r.PathValue("name"), err))
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
	a.registerSocialTestRoutes(mux)
	a.registerPushTestRoutes(mux, faults)
}

type tokenReq struct {
	UserID     string `json:"userId"`
	TTLSeconds int64  `json:"ttlSeconds"`
}

type tokenResp struct {
	UserID    string `json:"userId"`
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (a *Admin) issueToken(w http.ResponseWriter, r *http.Request) {
	var req tokenReq
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.UserID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	a.issueFor(w, r, req.UserID, a.tokenTTL(req.TTLSeconds))
}

// issueFor 自动注册用户（PRD F-USER-01 规则 3）后签发 token。
func (a *Admin) issueFor(w http.ResponseWriter, r *http.Request, userID string, ttl time.Duration) {
	// 已注销的用户不能再签发（PRD-chat F-ACC-08 规则 4）：不自动注册，也不复活。
	if deleted, err := a.store.UserDeleted(r.Context(), userID); err != nil {
		a.internal(w, "check deleted", err)
		return
	} else if deleted {
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user not found"))
		return
	}
	if err := a.store.EnsureUser(r.Context(), userID); err != nil {
		a.internal(w, "ensure user", err)
		return
	}
	// 版本号和封禁一次读出（见 IssueState）；到期按 [IM 钟] 判断，不需要定时任务（F-ADM-13 规则 4）。
	version, ban, err := a.store.IssueState(r.Context(), userID)
	if err != nil {
		a.internal(w, "token version", err)
		return
	}
	if ban.Active(a.clock.Now().UnixMilli()) {
		writeErr(w, bannedErr(ban.Until))
		return
	}
	tok, exp, err := a.signer.Issue(userID, version, ttl)
	if err != nil {
		a.internal(w, "issue token", err)
		return
	}
	writeJSON(w, http.StatusOK, tokenResp{UserID: userID, Token: tok, ExpiresAt: exp.UnixMilli()})
}

type userJSON struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	Official  bool   `json:"official"` // PRD-social F-SOC-13；导入时忽略
	Online    bool   `json:"online"`   // 只出现在响应里：来自 hub（B6 之后是 Presence），不缓存（F-ADM-10 规则 3）
}

// userOutJSON 是响应里的用户：比导入请求多第二批的 ban、mutedUntil（第二批关闭时不出现）。
type userOutJSON struct {
	userJSON
	Ban        json.RawMessage `json:"ban,omitempty"`
	MutedUntil json.RawMessage `json:"mutedUntil,omitempty"`
}

func (a *Admin) userOut(ctx context.Context, u store.User) (userOutJSON, error) {
	ban, muted, err := a.userModeration(ctx, u.UserID)
	return userOutJSON{userJSON{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar, CreatedAt: u.CreatedAt, Official: u.Official, Online: a.online(u.UserID)}, ban, muted}, err
}

// importUsers 批量导入用户（PRD F-USER-01、C-16、C-17）。任何一项不合法则整批不写入。
func (a *Admin) importUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Users []userJSON `json:"users"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	switch {
	case len(req.Users) == 0:
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "users is empty"))
		return
	case len(req.Users) > maxImportBatch:
		writeErr(w, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d users per request", maxImportBatch))
		return
	}
	// 同一批里 userId 重复时以最后一条为准。
	byID := make(map[string]int, len(req.Users))
	users := make([]store.User, 0, len(req.Users))
	for _, u := range req.Users {
		switch {
		case !protocol.ValidID(u.UserID):
			writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId %q", u.UserID))
			return
		case protocol.TextLen(u.Nickname) > maxNicknameLen:
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "nickname of %s longer than %d", u.UserID, maxNicknameLen))
			return
		case len(u.Avatar) > maxAvatarLen:
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "avatar of %s longer than %d", u.UserID, maxAvatarLen))
			return
		}
		su := store.User{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar}
		if i, ok := byID[u.UserID]; ok {
			users[i] = su
			continue
		}
		byID[u.UserID] = len(users)
		users = append(users, su)
	}
	if err := a.store.UpsertUsers(r.Context(), users); err != nil {
		a.internal(w, "import users", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Imported int `json:"imported"`
	}{len(users)})
}

func (a *Admin) getUser(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	u, err := a.store.GetUser(r.Context(), uid)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", uid))
	case err != nil:
		a.internal(w, "get user", err)
	default:
		out, err := a.userOut(r.Context(), u)
		if err != nil {
			a.internal(w, "user moderation", err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// forceLogout 强制下线：此前签发的 token 全部作废，在线连接收到 kicked/banned（PRD F-AUTH-03 规则 2）。
func (a *Admin) forceLogout(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	err := a.store.RevokeTokens(r.Context(), uid)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", uid))
	case err != nil:
		a.internal(w, "revoke tokens", err)
	default:
		// 不返回踢掉了几个连接：集群下连接分布在多个节点上，这个数字无法准确得到。
		a.router.KickUser(uid, protocol.KickBanned)
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

// RegisterDemoLogin 注册演示登录接口（PRD F-AUTH-04）。默认不注册；只用于开发和演示。
func (a *Admin) RegisterDemoLogin(mux *http.ServeMux) {
	mux.HandleFunc("OPTIONS /demo/login", func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /demo/login", func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)
		var req struct {
			Username   string `json:"username"`
			Nickname   string `json:"nickname"`
			TTLSeconds int64  `json:"ttlSeconds"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		uid := protocol.TrimText(req.Username)
		if !protocol.ValidID(uid) {
			writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid username"))
			return
		}
		// 有账号的用户不能演示登录（PRD-chat F-ACC-07 规则 2，R-CB-18）：否则输入用户名就能接管账号。先于昵称判断，昵称不变。
		switch _, err := a.store.GetAccount(r.Context(), uid); {
		case err == nil:
			writeErr(w, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonAccountExists, "account exists"))
			return
		case !errors.Is(err, store.ErrNotFound):
			a.internal(w, "check account", err)
			return
		}
		// 已注销的用户同样先拒绝，不改昵称（F-ACC-08 规则 4）。
		if deleted, err := a.store.UserDeleted(r.Context(), uid); err != nil {
			a.internal(w, "check deleted", err)
			return
		} else if deleted {
			writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user not found"))
			return
		}
		// 被封禁的用户：先拒绝，不能先改了昵称再报错（封禁不改变资料，F-ADM-13 规则 5）。
		if ban, err := a.store.UserBan(r.Context(), uid); err != nil {
			a.internal(w, "check ban", err)
			return
		} else if ban.Active(a.clock.Now().UnixMilli()) {
			writeErr(w, bannedErr(ban.Until))
			return
		}
		// 可选昵称（BE-091、PRD-ui G-UI-23）：去首尾空白后 1～32 个码点，登录（自动注册）时覆盖原昵称；
		// 不传或为空不改。只在演示模式下可用：任何人都能改任何人的昵称，这是演示环境可以接受的已知行为。
		if nick := protocol.TrimText(req.Nickname); nick != "" {
			if protocol.TextLen(nick) > maxNicknameLen {
				writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "nickname longer than %d", maxNicknameLen))
				return
			}
			if err := a.store.EnsureUser(r.Context(), uid); err != nil {
				a.internal(w, "ensure user", err)
				return
			}
			if err := a.store.SetNickname(r.Context(), uid, nick); err != nil {
				a.internal(w, "set nickname", err)
				return
			}
		}
		ttl := demoTokenTTL
		if a.testMode && req.TTLSeconds > 0 {
			ttl = a.tokenTTL(req.TTLSeconds)
		}
		a.issueFor(w, r, uid, ttl)
	})
}

// allowCORS 允许 H5 开发服务器跨域调用演示登录（仅演示模式注册）。
func allowCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// tokenTTL：不传、0 或负数按 7 天；上限 30 天；下限非测试模式 60 秒、测试模式 1 秒。
func (a *Admin) tokenTTL(seconds int64) time.Duration {
	if seconds <= 0 {
		return defaultTokenTTL
	}
	lower := minTokenTTL
	if a.testMode {
		lower = minTestTokenTTL
	}
	return min(max(time.Duration(seconds)*time.Second, lower), maxTokenTTL)
}

func (a *Admin) internal(w http.ResponseWriter, op string, err error) {
	a.log.Error(op, "err", err)
	writeErr(w, protocol.Errorf(protocol.CodeInternal, "internal error"))
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid json: %v", err))
		return false
	}
	return true
}

// writeErr 按 errors.json 的映射写 HTTP 状态码和错误体。
func writeErr(w http.ResponseWriter, e *protocol.Error) {
	writeJSON(w, protocol.HTTPStatus(e.Code), e)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
