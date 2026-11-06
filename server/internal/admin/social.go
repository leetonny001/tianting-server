package admin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/social"
	"tianting/server/internal/store"
)

// 社交管理接口（PRD-social F-SOC-01、F-SOC-12、F-SOC-13，BE-114、BE-115）。
// X-5 已落地：只注册在管理端口，按 scope 判定（social.settings / social.friends），写接口经 withAudit 审计
// （摘要只记字段名，不记备注原文）。

const (
	maxFriendPairs      = 500
	defaultFriendsLimit = 100
	maxFriendsLimit     = 500
)

// SocialService 是管理接口用到的社交能力（由 logic.Service 实现）。
type SocialService interface {
	SocialSettings(ctx context.Context) (social.Settings, *protocol.Error)
	PatchSocialSettings(ctx context.Context, patch []byte) (old, cur social.Settings, perr *protocol.Error)
	SocialMaxFriends(ctx context.Context) (int, *protocol.Error)
	NotifySocialChanged(bumped map[string]int64, actor, exceptConn string)
	PurgeSocialNow(ctx context.Context) (int, error)
	ResetLimits(uid string)
	DropEvents(uid, typ string, count int)
	SeedSocial(ctx context.Context, sd store.SocialSeed, silent bool) error
}

// WithSocial 设置社交能力并注册社交管理路由（app 组装时调用）。
func (a *Admin) WithSocial(svc SocialService) { a.social = svc }

func (a *Admin) registerSocial(mux *http.ServeMux) {
	if a.social == nil {
		return
	}
	a.route(mux, "GET", "/admin/settings/social", adminauth.ScopeSocialSettings, a.getSocialSettings)
	a.route(mux, "PATCH", "/admin/settings/social", adminauth.ScopeSocialSettings, a.patchSocialSettings)
	a.route(mux, "POST", "/admin/friends/import", adminauth.ScopeSocialFriends, a.importFriends)
	a.route(mux, "DELETE", "/admin/friends", adminauth.ScopeSocialFriends, a.removeFriends)
	a.route(mux, "GET", "/admin/users/{userId}/friends", adminauth.ScopeSocialFriends, a.listFriends)
	a.route(mux, "PUT", "/admin/users/{userId}/official", adminauth.ScopeSocialSettings, a.setOfficial)
}

func (a *Admin) getSocialSettings(w http.ResponseWriter, r *http.Request) {
	cfg, perr := a.social.SocialSettings(r.Context())
	if perr != nil {
		writeErr(w, perr)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *Admin) patchSocialSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "request body too large or unreadable"))
		return
	}
	_, cur, perr := a.social.PatchSocialSettings(r.Context(), body)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	writeJSON(w, http.StatusOK, cur)
}

type friendPairJSON struct {
	A       string  `json:"a"`
	B       string  `json:"b"`
	RemarkA *string `json:"remarkA"`
	RemarkB *string `json:"remarkB"`
}

// friendPairs 校验并去重（同一对按最后一条为准；{a,b} 与 {b,a} 是同一对）。错误优先级 4004 → 4000 → 4131。
func friendPairs(in []friendPairJSON, withRemarks bool) ([]store.FriendPair, *protocol.Error) {
	for _, p := range in {
		if !protocol.ValidID(p.A) || !protocol.ValidID(p.B) {
			return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
		}
	}
	if len(in) == 0 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "pairs is empty")
	}
	for _, p := range in {
		if p.A == p.B {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "a and b must differ")
		}
		if !withRemarks && (p.RemarkA != nil || p.RemarkB != nil) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "remarks are not allowed here")
		}
		for _, rm := range []*string{p.RemarkA, p.RemarkB} {
			if rm != nil && protocol.TextLen(protocol.TrimText(*rm)) > protocol.MaxRemarkLen {
				return nil, protocol.Errorf(protocol.CodeBadRequest, "remark longer than %d", protocol.MaxRemarkLen)
			}
		}
	}
	if len(in) > maxFriendPairs {
		return nil, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d pairs per request", maxFriendPairs)
	}
	idx := map[[2]string]int{}
	var out []store.FriendPair
	for _, p := range in {
		fp := store.FriendPair{A: p.A, B: p.B, RemarkA: trimPtr(p.RemarkA), RemarkB: trimPtr(p.RemarkB)}
		if fp.A > fp.B {
			fp.A, fp.B, fp.RemarkA, fp.RemarkB = fp.B, fp.A, fp.RemarkB, fp.RemarkA
		}
		k := [2]string{fp.A, fp.B}
		if i, ok := idx[k]; ok {
			out[i] = fp
			continue
		}
		idx[k] = len(out)
		out = append(out, fp)
	}
	return out, nil
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := protocol.TrimText(*s)
	return &v
}

func (a *Admin) importFriends(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pairs []friendPairJSON `json:"pairs"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	pairs, perr := friendPairs(req.Pairs, true)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	maxFriends, perr := a.social.SocialMaxFriends(r.Context())
	if perr != nil {
		writeErr(w, perr)
		return
	}
	changed, failed, err := a.store.ImportFriends(r.Context(), pairs, a.clock.Now().UnixMilli(), maxFriends)
	switch {
	case errors.Is(err, store.ErrUserMissing):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", failed))
		return
	case errors.Is(err, store.ErrImportFriendCap):
		writeErr(w, protocol.Errorf(protocol.CodeFriendLimitExceeded, "user %s would exceed the friend limit", failed))
		return
	case err != nil:
		a.internal(w, "import friends", err)
		return
	}
	a.social.NotifySocialChanged(changed, "", "")
	writeJSON(w, http.StatusOK, struct {
		Imported int `json:"imported"`
	}{len(pairs)})
}

func (a *Admin) removeFriends(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pairs []friendPairJSON `json:"pairs"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	pairs, perr := friendPairs(req.Pairs, false)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	changed, err := a.store.RemoveFriends(r.Context(), pairs, a.clock.Now().UnixMilli())
	if err != nil {
		a.internal(w, "remove friends", err)
		return
	}
	a.social.NotifySocialChanged(changed, "", "")
	writeJSON(w, http.StatusOK, struct {
		Removed int `json:"removed"`
	}{len(pairs)})
}

type adminFriendJSON struct {
	UserID    string `json:"userId"`
	Source    string `json:"source"`
	CreatedAt int64  `json:"createdAt"`
}

func (a *Admin) listFriends(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	q := r.URL.Query()
	limit := defaultFriendsLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxFriendsLimit {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1-%d", maxFriendsLimit))
			return
		}
		limit = n
	}
	after := q.Get("cursor")
	if after != "" && !protocol.ValidID(after) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor"))
		return
	}
	items, more, err := a.store.ListFriends(r.Context(), uid, after, limit)
	if err != nil {
		a.internal(w, "list friends", err)
		return
	}
	out := struct {
		Friends    []adminFriendJSON `json:"friends"`
		NextCursor string            `json:"nextCursor"`
	}{Friends: make([]adminFriendJSON, 0, len(items))}
	for _, f := range items {
		out.Friends = append(out.Friends, adminFriendJSON(f)) // 不返回备注（备注只有本人可见）
	}
	if more {
		out.NextCursor = items[len(items)-1].UserID
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *Admin) setOfficial(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	var req struct {
		Official *bool `json:"official"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Official == nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "official is required"))
		return
	}
	err := a.store.SetOfficial(r.Context(), uid, *req.Official)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", uid))
	case err != nil:
		a.internal(w, "set official", err)
	default:
		// 改标记不改 ver、不发事件（5.4 节第 19 行）；发送路径每次读取，立即生效。
		writeJSON(w, http.StatusOK, struct {
			UserID   string `json:"userId"`
			Official bool   `json:"official"`
		}{uid, *req.Official})
	}
}

// registerSocialTestRoutes 是 PRD-social 5.6 节的测试设施（只在测试模式注册）。
func (a *Admin) registerSocialTestRoutes(mux *http.ServeMux) {
	if a.social == nil {
		return
	}
	a.route(mux, "POST", "/admin/test/social/drop-events", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"userId"`
			Type   string `json:"type"`
			Count  int    `json:"count"`
		}
		if readJSON(w, r, &req) {
			a.social.DropEvents(req.UserID, req.Type, req.Count)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	a.route(mux, "POST", "/admin/test/social/purge", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		n, err := a.social.PurgeSocialNow(r.Context())
		if err != nil {
			a.internal(w, "purge social", err)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Purged int `json:"purged"`
		}{n})
	})
	a.route(mux, "POST", "/admin/test/social/seed", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Friends []struct {
				Owner string `json:"owner"`
				Peer  string `json:"peer"`
			} `json:"friends"`
			Blocks []struct {
				Owner string `json:"owner"`
				Peer  string `json:"peer"`
			} `json:"blocks"`
			Requests []struct {
				From     string `json:"from"`
				To       string `json:"to"`
				Status   string `json:"status"`
				Message  string `json:"message"`
				Source   string `json:"source"`
				ExpireAt int64  `json:"expireAt"`
			} `json:"requests"`
			GroupMembers []struct {
				GroupID string `json:"groupId"`
				UserID  string `json:"userId"`
			} `json:"groupMembers"`
			UpdatedAt int64 `json:"updatedAt"`
			Silent    bool  `json:"silent"` // 不发 socialChanged（e2e 好友制矩阵预置数据用）
		}
		if !readJSON(w, r, &req) {
			return
		}
		sd := store.SocialSeed{UpdatedAt: req.UpdatedAt}
		for _, f := range req.Friends {
			sd.Friends = append(sd.Friends, store.SeedEdge{Owner: f.Owner, Peer: f.Peer})
		}
		for _, b := range req.Blocks {
			sd.Blocks = append(sd.Blocks, store.SeedEdge{Owner: b.Owner, Peer: b.Peer})
		}
		for _, q := range req.Requests {
			sd.Requests = append(sd.Requests, store.SeedRequest{From: q.From, To: q.To, Status: q.Status, Message: q.Message, Source: q.Source, ExpireAt: q.ExpireAt})
		}
		groups := map[string]bool{}
		for _, m := range req.GroupMembers {
			sd.GroupMembers = append(sd.GroupMembers, store.SeedMember{GroupID: m.GroupID, UserID: m.UserID})
			groups[m.GroupID] = true
		}
		if err := a.social.SeedSocial(r.Context(), sd, req.Silent); err != nil {
			a.internal(w, "seed social", err)
			return
		}
		for g := range groups {
			a.inv.Invalidate(logic.TopicGroupMembers, g)
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
	a.route(mux, "POST", "/admin/test/limits/reset", adminauth.ScopeTest, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"userId"`
			Scope  string `json:"scope"` // "auth"：清"用户名 + IP"、IP、邀请码失败计数（PRD-chat 第 4.5 节）
		}
		if !readJSON(w, r, &req) {
			return
		}
		if req.Scope == "auth" {
			if a.accountTest == nil {
				writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "account test facilities are not available"))
				return
			}
			if err := a.accountTest.ResetLimits(r.Context()); err != nil {
				a.internal(w, "reset auth limits", err)
				return
			}
			writeJSON(w, http.StatusOK, struct{}{})
			return
		}
		a.social.ResetLimits(req.UserID)
		writeJSON(w, http.StatusOK, struct{}{})
	})
}
