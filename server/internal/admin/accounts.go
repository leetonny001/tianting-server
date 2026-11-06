package admin

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"tianting/server/internal/account"
	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 邀请码的限制（PRD-chat F-ACC-09 规则 1）。
const (
	inviteBatchMax   = 100
	inviteMaxUsesMax = 1000
	inviteCodeBytes  = 16 // 128 位随机（R-CS-08）
	inviteListMax    = 200
)

// RegisterAccounts 注册邀请码和账号信息（第 11.6 节）。旧密钥不能调用。没有任何改用户密码或恢复码的接口（AC-ACC-05-9）。
func (a *Admin) RegisterAccounts(mux *http.ServeMux) {
	a.route(mux, "POST", "/admin/invites", adminauth.ScopeAccountInvite, a.createInvites)
	a.route(mux, "GET", "/admin/invites", adminauth.ScopeAccountInvite, a.listInvites)
	a.route(mux, "POST", "/admin/invites/{inviteId}/disable", adminauth.ScopeAccountInvite, a.disableInvite)
	a.route(mux, "GET", "/admin/accounts/{userId}", adminauth.ScopeUserRead, a.getAccount)
}

type inviteJSON struct {
	InviteID     string `json:"inviteId"`
	CodeLast4    string `json:"codeLast4"`
	Code         string `json:"code,omitempty"` // 明文只在创建响应出现一次
	MaxUses      int    `json:"maxUses"`
	UsedCount    int    `json:"usedCount"`
	ExpiresAt    int64  `json:"expiresAt"`
	Note         string `json:"note,omitempty"`
	CreatedBy    string `json:"createdBy,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	DisabledAt   int64  `json:"disabledAt,omitempty"`
	AutoDisabled bool   `json:"autoDisabled,omitempty"`
}

func inviteOf(i store.Invite, code string) inviteJSON {
	return inviteJSON{InviteID: i.InviteID, CodeLast4: i.CodeLast4, Code: code, MaxUses: i.MaxUses, UsedCount: i.UsedCount, ExpiresAt: i.ExpiresAt,
		Note: i.Note, CreatedBy: i.CreatedBy, CreatedAt: i.CreatedAt, DisabledAt: i.DisabledAt, AutoDisabled: i.AutoDisabled}
}

func (a *Admin) createInvites(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count     int    `json:"count"`
		MaxUses   int    `json:"maxUses"`
		ExpiresAt int64  `json:"expiresAt"`
		Note      string `json:"note"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Count == 0 {
		req.Count = 1
	}
	if req.Count < 1 || req.Count > inviteBatchMax {
		writeErr(w, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d invites at a time", inviteBatchMax))
		return
	}
	now := a.clock.Now().UnixMilli()
	if req.MaxUses < 1 || req.MaxUses > inviteMaxUsesMax {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "maxUses must be 1-%d", inviteMaxUsesMax))
		return
	}
	if req.ExpiresAt <= now {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "expiresAt must be in the future"))
		return
	}
	if protocol.TextLen(req.Note) > 200 {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "note too long"))
		return
	}
	by := byOf(AuthFrom(r.Context()))
	invites := make([]store.Invite, 0, req.Count)
	out := make([]inviteJSON, 0, req.Count)
	for range req.Count {
		b := make([]byte, inviteCodeBytes)
		if _, err := rand.Read(b); err != nil {
			a.internal(w, "generate invite", err)
			return
		}
		code := base64.RawURLEncoding.EncodeToString(b)
		id, err := uuid.NewV7()
		if err != nil {
			a.internal(w, "generate invite id", err)
			return
		}
		inv := store.Invite{InviteID: id.String(), CodeHash: account.InviteHash(code), CodeLast4: code[len(code)-4:], Note: req.Note, CreatedBy: by,
			MaxUses: req.MaxUses, ExpiresAt: req.ExpiresAt, CreatedAt: now}
		invites = append(invites, inv)
		out = append(out, inviteOf(inv, code))
	}
	if err := a.store.CreateInvites(r.Context(), invites); err != nil {
		a.internal(w, "create invites", err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Invites []inviteJSON `json:"invites"`
	}{out})
}

func (a *Admin) listInvites(w http.ResponseWriter, r *http.Request) {
	limit := inviteListMax
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > inviteListMax {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1-%d", inviteListMax))
			return
		}
		limit = n
	}
	list, err := a.store.ListInvites(r.Context(), r.URL.Query().Get("after"), limit)
	if err != nil {
		a.internal(w, "list invites", err)
		return
	}
	out := make([]inviteJSON, 0, len(list))
	for _, i := range list {
		out = append(out, inviteOf(i, ""))
	}
	writeJSON(w, http.StatusOK, struct {
		Invites []inviteJSON `json:"invites"`
	}{out})
}

func (a *Admin) disableInvite(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("inviteId"))
	switch err := a.store.DisableInvite(r.Context(), id, a.clock.Now().UnixMilli()); {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeNotFound, "invite not found"))
	case err != nil:
		a.internal(w, "disable invite", err)
	default:
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

// getAccount 是用户详情页的账号信息（规则 5）：不含任何哈希、令牌和 sid。
func (a *Admin) getAccount(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	type resp struct {
		HasAccount        bool  `json:"hasAccount"`
		CreatedAt         int64 `json:"createdAt,omitempty"`
		LastLoginAt       int64 `json:"lastLoginAt,omitempty"`
		DeleteRequestedAt int64 `json:"deleteRequestedAt,omitempty"`
		Sessions          int   `json:"sessions"`
	}
	info, err := a.store.AccountInfo(r.Context(), uid, a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusOK, resp{})
	case err != nil:
		a.internal(w, "account info", err)
	default:
		writeJSON(w, http.StatusOK, resp{HasAccount: true, CreatedAt: info.CreatedAt, LastLoginAt: info.LastLoginAt, DeleteRequestedAt: info.DeleteRequestedAt, Sessions: info.Sessions})
	}
}
