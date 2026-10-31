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
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/logic"
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/store"
)

const (
	defaultTokenTTL = 7 * 24 * time.Hour
	maxTokenTTL     = 30 * 24 * time.Hour
	minTokenTTL     = 60 * time.Second // 非测试模式的下限（PRD C-13）
	minTestTokenTTL = time.Second      // 测试模式的下限（PRD 6.1 节）
	maxGroupMembers = 2000
)

// Admin 注册 /admin/* 路由。
type Admin struct {
	key      string
	signer   *auth.Signer
	store    store.Store
	log      *slog.Logger
	testMode bool
}

func New(key string, signer *auth.Signer, st store.Store, testMode bool, log *slog.Logger) *Admin {
	return &Admin{key: key, signer: signer, store: st, testMode: testMode, log: log}
}

// Register 把管理路由挂到 mux 上。
func (a *Admin) Register(mux *http.ServeMux) {
	mux.Handle("POST /admin/users/token", a.guard(a.issueToken))
	mux.Handle("POST /admin/groups", a.guard(a.createGroup))
	mux.Handle("POST /admin/groups/{groupId}/members", a.guard(a.addMembers))
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
	tok, exp, err := a.signer.Issue(req.UserID, a.tokenTTL(req.TTLSeconds))
	if err != nil {
		a.internal(w, "issue token", err)
		return
	}
	writeJSON(w, http.StatusOK, tokenResp{Token: tok, ExpiresAt: exp.UnixMilli()})
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

type groupReq struct {
	GroupID string   `json:"groupId"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

func (a *Admin) createGroup(w http.ResponseWriter, r *http.Request) {
	var req groupReq
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.GroupID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	if !validMembers(w, req.Members) {
		return
	}
	err := a.store.CreateGroup(r.Context(), req.GroupID, req.Name, req.Members)
	switch {
	case errors.Is(err, store.ErrExists):
		writeErr(w, protocol.Errorf(protocol.CodeGroupExists, "group already exists"))
	case err != nil:
		a.internal(w, "create group", err)
	default:
		writeJSON(w, http.StatusCreated, struct{}{})
	}
}

type membersReq struct {
	Members []string `json:"members"`
}

func (a *Admin) addMembers(w http.ResponseWriter, r *http.Request) {
	var req membersReq
	if !readJSON(w, r, &req) {
		return
	}
	if !validMembers(w, req.Members) {
		return
	}
	err := a.store.AddGroupMembers(r.Context(), r.PathValue("groupId"), req.Members)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case err != nil:
		a.internal(w, "add members", err)
	default:
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

func validMembers(w http.ResponseWriter, members []string) bool {
	if len(members) > maxGroupMembers {
		writeErr(w, protocol.Errorf(protocol.CodeGroupMemberLimitExceeded, "too many members"))
		return false
	}
	for _, m := range members {
		if !protocol.ValidID(m) {
			writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid member id %s", m))
			return false
		}
	}
	return true
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
