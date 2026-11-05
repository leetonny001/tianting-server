package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
	"tianting/server/internal/sessions"
	"tianting/server/internal/store"
)

// 管理后台第一批的查询接口（PRD-admin G-ADM-05、06、09、10、12）。

const (
	maxSearchQueryLen = 64
	defaultPageLimit  = 50
	maxMetaLimit      = 50
)

// WithSessions 接上连接查询（单节点是 hub）。
func (a *Admin) WithSessions(src sessions.Source) { a.sessions = src }

func (a *Admin) registerQueries(mux *http.ServeMux) {
	a.route(mux, "GET", "/admin/users", adminauth.ScopeUserRead, a.searchUsers)
	a.route(mux, "POST", "/admin/users/sessions/query", adminauth.ScopeUserSessions, a.querySessions)
	a.route(mux, "GET", "/admin/groups", adminauth.ScopeGroupRead, a.searchGroups)
	a.route(mux, "GET", "/admin/groups/{groupId}/members", adminauth.ScopeGroupRead, a.listGroupMembers)
	a.route(mux, "POST", "/admin/convs/messages/query", adminauth.ScopeMsgMeta, a.queryMessageMeta)
}

// pageLimit 取 limit 参数：只接受 20 / 50 / 100，默认 50。
func pageLimit(r *http.Request) (int, *protocol.Error) {
	switch r.URL.Query().Get("limit") {
	case "":
		return defaultPageLimit, nil
	case "20":
		return 20, nil
	case "50":
		return 50, nil
	case "100":
		return 100, nil
	}
	return 0, protocol.Errorf(protocol.CodeBadRequest, "limit must be 20, 50 or 100")
}

// encodeCursor / decodeCursor：游标对调用方不透明。解不开的游标是 4000，不当成第一页。
func encodeCursor(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func decodeCursor(r *http.Request) (string, *protocol.Error) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) == 0 {
		return "", protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
	}
	return string(b), nil
}

// searchQuery 取关键字：去首尾空白后不超过 64 字符。
func searchQuery(r *http.Request) (string, *protocol.Error) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > maxSearchQueryLen {
		return "", protocol.Errorf(protocol.CodeBadRequest, "q must be at most %d characters", maxSearchQueryLen)
	}
	return q, nil
}

func (a *Admin) online(userID string) bool {
	return a.sessions != nil && a.sessions.ConnCount(userID) > 0
}

// searchUsers 是 GET /admin/users：没有关键字时必须筛选官方账号（不提供遍历全部用户的入口）。
func (a *Admin) searchUsers(w http.ResponseWriter, r *http.Request) {
	q, perr := searchQuery(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	official := r.URL.Query().Get("official") == "true"
	if q == "" && !official {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "q is required"))
		return
	}
	limit, perr := pageLimit(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	after, perr := decodeCursor(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	users, next, err := a.store.SearchUsers(r.Context(), store.UserSearch{Q: q, OfficialOnly: official, After: after, Limit: limit})
	if err != nil {
		a.internal(w, "search users", err)
		return
	}
	out := make([]userOutJSON, len(users))
	for i, u := range users {
		if out[i], err = a.userOut(r.Context(), u); err != nil {
			a.internal(w, "user moderation", err)
			return
		}
	}
	cursor := ""
	if next != "" {
		cursor = encodeCursor(next)
	}
	writeJSON(w, http.StatusOK, struct {
		Users      []userOutJSON `json:"users"`
		NextCursor string        `json:"nextCursor"`
	}{out, cursor})
}

// querySessions 是 POST /admin/users/sessions/query：userId 只在请求体里（不进 URL 和访问日志）；写审计，结果条数进 summary。
func (a *Admin) querySessions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"userId"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.UserID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	if _, err := a.store.GetUser(r.Context(), req.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user not found"))
		} else {
			a.internal(w, "get user", err)
		}
		return
	}
	res := sessions.Query(a.sessions, req.UserID)
	noteAuditCount(r.Context(), len(res.Sessions))
	writeJSON(w, http.StatusOK, res)
}

type groupListJSON struct {
	GroupID        string  `json:"groupId"`
	Name           string  `json:"name"`
	Type           string  `json:"type"` // managed：业务方托管；user：客户端自建
	Owner          *string `json:"owner"`
	MemberCount    int     `json:"memberCount"`
	HistoryVisible bool    `json:"historyVisible"`
	CreatedAt      int64   `json:"createdAt"`
	DissolvedAt    int64   `json:"dissolvedAt,omitempty"`
}

// searchGroups 是 GET /admin/groups：默认排除已解散的群，按创建时间倒序。
func (a *Admin) searchGroups(w http.ResponseWriter, r *http.Request) {
	q, perr := searchQuery(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	limit, perr := pageLimit(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	raw, perr := decodeCursor(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	var after *store.GroupCursor
	if raw != "" {
		ts, id, ok := strings.Cut(raw, "|")
		n, err := strconv.ParseInt(ts, 10, 64)
		if !ok || err != nil {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor"))
			return
		}
		after = &store.GroupCursor{CreatedAt: n, GroupID: id}
	}
	groups, next, err := a.store.SearchGroups(r.Context(), store.GroupSearch{
		Q: q, IncludeDissolved: r.URL.Query().Get("includeDissolved") == "true", After: after, Limit: limit,
	})
	if err != nil {
		a.internal(w, "search groups", err)
		return
	}
	out := make([]groupListJSON, len(groups))
	for i, g := range groups {
		item := groupListJSON{
			GroupID: g.GroupID, Name: g.Name, Type: "managed", MemberCount: g.MemberCount,
			HistoryVisible: g.HistoryVisible, CreatedAt: g.CreatedAt, DissolvedAt: g.DissolvedAt,
		}
		if g.CreatedBy != "" {
			item.Type = "user"
		}
		if g.OwnerID != "" {
			owner := g.OwnerID
			item.Owner = &owner
		}
		out[i] = item
	}
	cursor := ""
	if next != nil {
		cursor = encodeCursor(strconv.FormatInt(next.CreatedAt, 10) + "|" + next.GroupID)
	}
	writeJSON(w, http.StatusOK, struct {
		Groups     []groupListJSON `json:"groups"`
		NextCursor string          `json:"nextCursor"`
	}{out, cursor})
}

// listGroupMembers 是 GET /admin/groups/{groupId}/members：按 userId 升序；已解散或不存在 404 / 4043。
func (a *Admin) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("groupId")
	if !protocol.ValidID(gid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	limit, perr := pageLimit(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	after, perr := decodeCursor(r)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	prefix := r.URL.Query().Get("prefix")
	if len(prefix) > maxSearchQueryLen {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "prefix must be at most %d characters", maxSearchQueryLen))
		return
	}
	if _, err := a.store.GetGroup(r.Context(), gid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", gid))
		} else {
			a.internal(w, "get group", err)
		}
		return
	}
	rows, next, err := a.store.ListGroupMembersByID(r.Context(), gid, after, prefix, limit)
	if err != nil {
		a.internal(w, "list group members", err)
		return
	}
	type member struct {
		UserID     string          `json:"userId"`
		Role       string          `json:"role"`
		JoinedAt   int64           `json:"joinedAt"`
		MutedUntil json.RawMessage `json:"mutedUntil,omitempty"` // 第二批：禁言中才出现，永久为 null
	}
	var mutes map[string]store.Mute
	if a.mod != nil {
		if mutes, err = a.store.GroupMutes(r.Context(), gid); err != nil {
			a.internal(w, "group mutes", err)
			return
		}
	}
	nowMs := a.clock.Now().UnixMilli()
	out := make([]member, len(rows))
	for i, m := range rows {
		out[i] = member{UserID: m.UserID, Role: m.Role, JoinedAt: m.JoinedAt}
		if mu, ok := mutes[m.UserID]; ok && mu.Active(nowMs) {
			out[i].MutedUntil = protocol.NullableMs(mu.Until)
		}
	}
	cursor := ""
	if next != "" {
		cursor = encodeCursor(next)
	}
	writeJSON(w, http.StatusOK, struct {
		Members    []member `json:"members"`
		NextCursor string   `json:"nextCursor"`
	}{out, cursor})
}

// messageMetaJSON 是响应项。故意没有 body 字段（AC-ADM-30-7），由单测断言。
type messageMetaJSON struct {
	Seq        int64  `json:"seq"`
	MsgID      string `json:"msgId"`
	From       string `json:"from"`
	Type       string `json:"type"`
	BodyLength int    `json:"bodyLength"`
	TS         int64  `json:"ts"`
	Deleted    bool   `json:"deleted,omitempty"`
}

// queryMessageMeta 是 POST /admin/convs/messages/query：不受成员关系和 historyVisible 限制，已解散的群照常返回；
// 会话不存在返回空列表（和存在但没有消息一样）。
func (a *Admin) queryMessageMeta(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConvID    string `json:"convId"`
		BeforeSeq *int64 `json:"beforeSeq"`
		Limit     *int   `json:"limit"`
		From      string `json:"from"`
		SinceTS   int64  `json:"sinceTs"`
		UntilTS   int64  `json:"untilTs"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if _, perr := protocol.ParseConvID(req.ConvID); perr != nil {
		writeErr(w, perr)
		return
	}
	q := store.MessageMetaQuery{ConvID: req.ConvID, From: req.From, SinceTS: req.SinceTS, UntilTS: req.UntilTS, Limit: maxMetaLimit}
	if req.Limit != nil {
		if *req.Limit < 1 || *req.Limit > maxMetaLimit {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1..%d", maxMetaLimit))
			return
		}
		q.Limit = *req.Limit
	}
	if req.BeforeSeq != nil {
		if *req.BeforeSeq < 1 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "beforeSeq must be positive"))
			return
		}
		q.BeforeSeq = *req.BeforeSeq
	}
	if req.From != "" && !protocol.ValidID(req.From) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid from"))
		return
	}
	if req.SinceTS < 0 || req.UntilTS < 0 || (req.UntilTS > 0 && req.SinceTS > req.UntilTS) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid time range"))
		return
	}
	msgs, more, err := a.store.ListMessageMeta(r.Context(), q)
	if err != nil {
		a.internal(w, "list message meta", err)
		return
	}
	out := make([]messageMetaJSON, len(msgs))
	for i, m := range msgs {
		out[i] = messageMetaJSON{m.Seq, m.MsgID, m.From, m.Type, m.BodyLength, m.TS, m.Deleted}
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		Messages []messageMetaJSON `json:"messages"`
		HasMore  bool              `json:"hasMore"`
	}{out, more})
}

// auditNote 让处理函数把结果条数交给审计（summary.count，PRD-admin F-ADM-14 规则 3）。
type auditNoteKey struct{}

type auditNote struct {
	count  *int
	fields map[string]any
}

// noteAuditFields 让处理函数把白名单字段并进 result 的 summary（如 seq 范围）。不能传正文。
func noteAuditFields(ctx context.Context, fields map[string]any) {
	if note, ok := ctx.Value(auditNoteKey{}).(*auditNote); ok {
		note.fields = fields
	}
}

func noteAuditCount(ctx context.Context, n int) {
	if note, ok := ctx.Value(auditNoteKey{}).(*auditNote); ok {
		note.count = &n
	}
}

// summaryWithFields 把字段并进 summary_json；解析不了时保持原样（审计不能因此丢失）。
func summaryWithFields(summary string, fields map[string]any) string {
	var s map[string]any
	if json.Unmarshal([]byte(summary), &s) != nil {
		return summary
	}
	for k, v := range fields {
		s[k] = v
	}
	b, err := json.Marshal(s)
	if err != nil {
		return summary
	}
	return string(b)
}
