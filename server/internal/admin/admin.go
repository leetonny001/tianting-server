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
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/store"
)

const (
	defaultTokenTTL = 7 * 24 * time.Hour
	maxTokenTTL     = 30 * 24 * time.Hour
	maxGroupMembers = 2000
)

// Admin 注册 /admin/* 路由。
type Admin struct {
	key    string
	signer *auth.Signer
	store  store.Store
	log    *slog.Logger
}

func New(key string, signer *auth.Signer, st store.Store, log *slog.Logger) *Admin {
	return &Admin{key: key, signer: signer, store: st, log: log}
}

// Register 把管理路由挂到 mux 上。
func (a *Admin) Register(mux *http.ServeMux) {
	mux.Handle("POST /admin/users/token", a.guard(a.issueToken))
	mux.Handle("POST /admin/groups", a.guard(a.createGroup))
	mux.Handle("POST /admin/groups/{groupId}/members", a.guard(a.addMembers))
}

func (a *Admin) guard(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Admin-Key")
		if subtle.ConstantTimeCompare([]byte(got), []byte(a.key)) != 1 {
			writeErr(w, http.StatusUnauthorized, protocol.CodeUnauthorized, "invalid admin key")
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
		writeErr(w, http.StatusBadRequest, protocol.CodeBadRequest, "invalid userId")
		return
	}
	ttl := defaultTokenTTL
	if req.TTLSeconds > 0 {
		ttl = min(time.Duration(req.TTLSeconds)*time.Second, maxTokenTTL)
	}
	tok, exp, err := a.signer.Issue(req.UserID, ttl)
	if err != nil {
		a.internal(w, "issue token", err)
		return
	}
	writeJSON(w, http.StatusOK, tokenResp{Token: tok, ExpiresAt: exp.UnixMilli()})
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
		writeErr(w, http.StatusBadRequest, protocol.CodeBadRequest, "invalid groupId")
		return
	}
	if !validMembers(w, req.Members) {
		return
	}
	err := a.store.CreateGroup(r.Context(), req.GroupID, req.Name, req.Members)
	switch {
	case errors.Is(err, store.ErrExists):
		writeErr(w, http.StatusConflict, protocol.CodeBadRequest, "group already exists")
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
		writeErr(w, http.StatusNotFound, protocol.CodeNotFound, "group not found")
	case err != nil:
		a.internal(w, "add members", err)
	default:
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

func validMembers(w http.ResponseWriter, members []string) bool {
	if len(members) > maxGroupMembers {
		writeErr(w, http.StatusBadRequest, protocol.CodeBadRequest, "too many members")
		return false
	}
	for _, m := range members {
		if !protocol.ValidID(m) {
			writeErr(w, http.StatusBadRequest, protocol.CodeBadRequest, "invalid member id "+m)
			return false
		}
	}
	return true
}

func (a *Admin) internal(w http.ResponseWriter, op string, err error) {
	a.log.Error(op, "err", err)
	writeErr(w, http.StatusInternalServerError, protocol.CodeInternal, "internal error")
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, protocol.CodeBadRequest, "invalid json: "+err.Error())
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, status, code int, msg string) {
	writeJSON(w, status, protocol.Error{Code: code, Msg: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
