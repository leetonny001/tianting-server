// Package admin 是给业务服务端调用的管理接口，用 X-Admin-Key 鉴权。
package admin

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"im-mvp/server/internal/auth"
	"im-mvp/server/internal/cache"
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/delivery"
	"im-mvp/server/internal/logic"
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/store"
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
	key      string
	signer   *auth.Signer
	store    store.Store
	router   delivery.Router
	inv      cache.Invalidator
	clock    clock.Clock
	log      *slog.Logger
	testMode bool
}

// inv 用于在群成员变动后通知各节点的成员缓存失效（logic.TopicGroupMembers）。
func New(key string, signer *auth.Signer, st store.Store, router delivery.Router, inv cache.Invalidator, clk clock.Clock, testMode bool, log *slog.Logger) *Admin {
	return &Admin{key: key, signer: signer, store: st, router: router, inv: inv, clock: clk, testMode: testMode, log: log}
}

// Register 把管理路由挂到 mux 上。
func (a *Admin) Register(mux *http.ServeMux) {
	mux.Handle("POST /admin/users/token", a.guard(a.issueToken))
	mux.Handle("POST /admin/users/import", a.guard(a.importUsers))
	mux.Handle("GET /admin/users/{userId}", a.guard(a.getUser))
	mux.Handle("POST /admin/users/{userId}/force-logout", a.guard(a.forceLogout))
	mux.Handle("POST /admin/groups", a.guard(a.createGroup))
	mux.Handle("GET /admin/groups/{groupId}", a.guard(a.getGroup))
	mux.Handle("PATCH /admin/groups/{groupId}", a.guard(a.updateGroup))
	mux.Handle("POST /admin/groups/{groupId}/members", a.guard(a.addMembers))
	mux.Handle("DELETE /admin/groups/{groupId}/members", a.guard(a.removeMembers))
}

// RegisterTestRoutes 注册故障注入和时钟接口，只能在测试模式下调用（PRD 6.1 节）。
func (a *Admin) RegisterTestRoutes(mux *http.ServeMux, faults *logic.Faults, clk *clock.Settable) {
	mux.Handle("POST /admin/test/drop-ack", a.guard(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientMsgID string `json:"clientMsgId"`
		}
		if readJSON(w, r, &req) {
			faults.DropAck(req.ClientMsgID)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	}))
	mux.Handle("POST /admin/test/skip-delivery", a.guard(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientMsgID string `json:"clientMsgId"`
		}
		if readJSON(w, r, &req) {
			faults.SkipDelivery(req.ClientMsgID)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	}))
	mux.Handle("POST /admin/test/clock", a.guard(func(w http.ResponseWriter, r *http.Request) {
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
	}))
}

func (a *Admin) guard(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Admin-Key")
		if subtle.ConstantTimeCompare([]byte(got), []byte(a.key)) != 1 {
			writeErr(w, protocol.Errorf(protocol.CodeAdminKeyInvalid, "invalid admin key"))
			return
		}
		next(w, r)
	})
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
	if err := a.store.EnsureUser(r.Context(), userID); err != nil {
		a.internal(w, "ensure user", err)
		return
	}
	version, err := a.store.TokenVersion(r.Context(), userID)
	if err != nil {
		a.internal(w, "token version", err)
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
		writeJSON(w, http.StatusOK, userJSON{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar, CreatedAt: u.CreatedAt})
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
