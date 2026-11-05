package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"unicode/utf8"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 系统推送接口（PRD-live v1.0 4.2 节、F-PUSH-02、PRD-admin F-ADM-50 规则 8）。IM_PUSH_ENABLED 关闭时不注册（404）。

var pushIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// PushService 是业务层提供的推送（由 logic.Service 实现）。
type PushService interface {
	Push(ctx context.Context, cmd logic.PushCommand) (logic.PushResult, *protocol.Error)
	ListPushInbox(ctx context.Context, uid string) ([]store.PushInboxRow, error)
}

// WithPush 打开系统推送接口。要在 Register 之前调用。
func (a *Admin) WithPush(p PushService) { a.pusher = p }

func (a *Admin) registerPush(mux *http.ServeMux) {
	if a.pusher == nil {
		return
	}
	// 进入路由只要有其中任一 scope，具体 scope 按 target 判断（F-KEY-02 规则 2）。
	a.routeAny(mux, "POST", "/admin/push", []adminauth.Scope{adminauth.ScopePushUser, adminauth.ScopePushAll, adminauth.ScopePushRoom}, a.handlePush)
}

var pushTargetScope = map[string]adminauth.Scope{
	protocol.PushTargetUser: adminauth.ScopePushUser,
	protocol.PushTargetAll:  adminauth.ScopePushAll,
	protocol.PushTargetRoom: adminauth.ScopePushRoom,
}

// pushRequest 是 POST /admin/push 的请求。TTLSeconds 区分"不传"和"传 0"（F-PUSH-02 规则 4）。
type pushRequest struct {
	Target     string          `json:"target"`
	UserIDs    []string        `json:"userIds"`
	RoomID     *string         `json:"roomId"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	PushID     string          `json:"pushId"`
	TTLSeconds *int64          `json:"ttlSeconds"`
}

// handlePush：错误优先级（F-PUSH-02 规则 7）签名和 scope → type 白名单 → 4000 参数 → 4094 / 幂等命中 → 4131 → 4042 → 4290。
// 后三步在 logic.Service.Push 里。
func (a *Admin) handlePush(w http.ResponseWriter, r *http.Request) {
	var req pushRequest
	if !readJSON(w, r, &req) {
		return
	}
	info := AuthFrom(r.Context())
	scope, known := pushTargetScope[req.Target]
	if !known {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "target must be user, all or room"))
		return
	}
	caller := a.callers[info.Caller]
	if caller == nil || !caller.Has(scope) {
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "caller %s lacks scope %s", info.Caller, scope))
		return
	}
	if len(caller.PushTypes) > 0 && !slices.Contains(caller.PushTypes, req.Type) {
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "caller %s may not push this type", info.Caller))
		return
	}
	ttl, perr := validatePush(&req)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	users := dedupStrings(req.UserIDs)
	sum := sha256.Sum256(req.Data)
	fields := map[string]any{"target": req.Target, "type": req.Type, "pushId": req.PushID, "dataLength": len(req.Data), "dataSha256": hex.EncodeToString(sum[:])}
	if req.Target == protocol.PushTargetUser {
		fields["targetCount"] = len(users)
	}
	noteAuditFields(r.Context(), fields)
	res, perr := a.pusher.Push(r.Context(), logic.PushCommand{
		Caller: info.Caller, Target: req.Target, UserIDs: users, Type: req.Type, Data: req.Data, PushID: req.PushID, TTL: ttl,
	})
	if perr != nil {
		writeErr(w, perr)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		PushID   string `json:"pushId"`
		Created  bool   `json:"created"`
		Accepted *int   `json:"accepted,omitempty"`
	}{req.PushID, res.Created, res.Accepted})
}

// validatePush 校验 4000 类的参数，返回生效的 ttlSeconds。
func validatePush(req *pushRequest) (int64, *protocol.Error) {
	bad := func(format string, args ...any) (int64, *protocol.Error) {
		return 0, protocol.Errorf(protocol.CodeBadRequest, format, args...)
	}
	// L1 没有房间：target=room 由开关挡住（F-PUSH-02 规则 6，AC-PUSH-02-8）。接口 L2 开放。
	if req.Target == protocol.PushTargetRoom {
		return bad("rooms are not enabled")
	}
	if req.RoomID != nil {
		return bad("roomId is only allowed with target=room")
	}
	if n := utf8.RuneCountInString(req.Type); n < 1 || n > protocol.MaxPushTypeLen {
		return bad("type must be 1..%d characters", protocol.MaxPushTypeLen)
	}
	if !pushIDRe.MatchString(req.PushID) {
		return bad("pushId must match [A-Za-z0-9_-]{1,64}")
	}
	if len(req.Data) == 0 || string(req.Data) == "null" {
		return bad("data is required")
	}
	if len(req.Data) > protocol.MaxPushDataBytes {
		return bad("data must be at most %d bytes", protocol.MaxPushDataBytes)
	}
	ttl := int64(0)
	switch req.Target {
	case protocol.PushTargetUser:
		if len(req.UserIDs) == 0 {
			return bad("userIds is required with target=user")
		}
		ttl = protocol.MaxPushTTLSeconds
		if req.TTLSeconds != nil {
			ttl = *req.TTLSeconds
		}
		if ttl < 0 || ttl > protocol.MaxPushTTLSeconds {
			return bad("ttlSeconds must be 0..%d", protocol.MaxPushTTLSeconds)
		}
	default:
		if req.UserIDs != nil {
			return bad("userIds is only allowed with target=user")
		}
		if req.TTLSeconds != nil && *req.TTLSeconds != 0 {
			return bad("ttlSeconds must be 0 or absent with target=%s", req.Target)
		}
	}
	return ttl, nil
}

func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// registerPushTestRoutes 是 PRD-live 8.2 节标 L1 的测试设施，只在测试模式注册。
func (a *Admin) registerPushTestRoutes(mux *http.ServeMux, faults *logic.Faults) {
	if a.pusher == nil {
		return
	}
	a.route(mux, "GET", "/admin/test/push-inbox/{userId}", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		rows, err := a.pusher.ListPushInbox(r.Context(), r.PathValue("userId"))
		if err != nil {
			a.internal(w, "list push inbox", err)
			return
		}
		type item struct {
			PushSeq   int64           `json:"pushSeq"`
			PushID    string          `json:"pushId"`
			Type      string          `json:"type"`
			Data      json.RawMessage `json:"data"`
			CreatedAt int64           `json:"createdAt"`
			ExpiresAt int64           `json:"expiresAt"`
		}
		out := make([]item, len(rows))
		for i, row := range rows {
			out[i] = item{row.PushSeq, row.PushID, row.Type, row.Data, row.CreatedAt, row.ExpiresAt}
		}
		writeJSON(w, http.StatusOK, struct {
			Rows []item `json:"rows"`
		}{out})
	})
	byPushID := func(set func(string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				PushID string `json:"pushId"`
			}
			if readJSON(w, r, &req) {
				set(req.PushID)
				writeJSON(w, http.StatusOK, struct{}{})
			}
		}
	}
	a.route(mux, "POST", "/admin/test/faults/push-skip-delivery", adminauth.ScopeTest, byPushID(faults.PushSkipDelivery))
	a.route(mux, "POST", "/admin/test/faults/push-exit-before-broadcast", adminauth.ScopeTest, byPushID(faults.PushExitBeforeBroadcast))
	a.route(mux, "POST", "/admin/test/faults/push-fail-at", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Index *int `json:"index"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if req.Index == nil || *req.Index < 0 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "index must be >= 0"))
			return
		}
		faults.PushFailAt(*req.Index)
		writeJSON(w, http.StatusOK, struct{}{})
	})
}
