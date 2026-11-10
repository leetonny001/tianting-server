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
	a.route(mux, "GET", "/admin/users/{userId}/groups", adminauth.ScopeGroupRead, a.listUserGroups)
	a.route(mux, "POST", "/admin/users/convs/query", adminauth.ScopeMsgMeta, a.queryUserConvs)
	a.route(mux, "GET", "/admin/stats", adminauth.ScopeStatsRead, a.stats)
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

func toGroupListJSON(g store.Group, dissolvedAt int64) groupListJSON {
	item := groupListJSON{
		GroupID: g.GroupID, Name: g.Name, Type: "managed", MemberCount: g.MemberCount,
		HistoryVisible: g.HistoryVisible, CreatedAt: g.CreatedAt, DissolvedAt: dissolvedAt,
	}
	if g.CreatedBy != "" {
		item.Type = "user"
	}
	if g.OwnerID != "" {
		owner := g.OwnerID
		item.Owner = &owner
	}
	return item
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
		out[i] = toGroupListJSON(g.Group, g.DissolvedAt)
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
	FileSize   int64  `json:"fileSize,omitempty"`
	TS         int64  `json:"ts"`
	Deleted    bool   `json:"deleted,omitempty"`
	DeletedAt  int64  `json:"deletedAt,omitempty"`
	DeletedBy  string `json:"deletedBy,omitempty"`
	// 撤回（C4，PRD-chat F-RCL-04 规则 3、AC-RCL-04-7）：deleted 只表示后台删除；撤回的 bodyLength 为 0、没有 fileSize。
	Recalled   bool   `json:"recalled,omitempty"`
	RecalledAt int64  `json:"recalledAt,omitempty"`
	RecalledBy string `json:"recalledBy,omitempty"`
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
		out[i] = messageMetaJSON{m.Seq, m.MsgID, m.From, m.Type, m.BodyLength, m.FileSize, m.TS, m.DeletedAt > 0, m.DeletedAt, m.DeletedBy,
			m.RecalledAt > 0, m.RecalledAt, m.RecalledBy}
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		Messages []messageMetaJSON `json:"messages"`
		HasMore  bool              `json:"hasMore"`
	}{out, more})
}

// listUserGroups 是 GET /admin/users/{userId}/groups（G-ADM-07）：用户当前所在的群，不含已解散的，按 groupId 升序。只读列表，不写审计（F-KEY-04 规则 1）。
func (a *Admin) listUserGroups(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
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
	if !a.userMustExist(w, r, uid) {
		return
	}
	rows, next, err := a.store.ListUserGroups(r.Context(), uid, after, limit)
	if err != nil {
		a.internal(w, "list user groups", err)
		return
	}
	type item struct {
		groupListJSON
		Role     string `json:"role"`
		JoinedAt int64  `json:"joinedAt"`
	}
	out := make([]item, len(rows))
	for i, g := range rows {
		out[i] = item{toGroupListJSON(g.Group, 0), g.Role, g.JoinedAt}
	}
	cursor := ""
	if next != "" {
		cursor = encodeCursor(next)
	}
	writeJSON(w, http.StatusOK, struct {
		Groups     []item `json:"groups"`
		NextCursor string `json:"nextCursor"`
	}{out, cursor})
}

func (a *Admin) userMustExist(w http.ResponseWriter, r *http.Request, uid string) bool {
	_, err := a.store.GetUser(r.Context(), uid)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user not found"))
	case err != nil:
		a.internal(w, "get user", err)
	default:
		return true
	}
	return false
}

// userConvMetaJSON 是用户会话列表的一项。故意没有最后一条消息的文本：这个接口和消息元数据查询一样不返回正文（AC-ADM-30-7 的约束）。
type userConvMetaJSON struct {
	ConvID      string `json:"convId"`
	Type        string `json:"type"`
	PeerID      string `json:"peerId,omitempty"`
	GroupID     string `json:"groupId,omitempty"`
	GroupName   string `json:"groupName,omitempty"`
	MyRole      string `json:"myRole,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
	MaxSeq      int64  `json:"maxSeq"`
	ReadSeq     int64  `json:"readSeq"`
	UpdatedAt   int64  `json:"updatedAt"`
	LastTS      int64  `json:"lastTs,omitempty"`
	LastFrom    string `json:"lastFrom,omitempty"`
	LastType    string `json:"lastType,omitempty"`
}

// queryUserConvs 是 POST /admin/users/convs/query（G-ADM-13，元数据入口 ③）：userId 只在请求体里（不进 URL 和访问日志），写审计，结果条数进 summary。
// 和 F-ADM-30 一样不受 historyVisible 限制；用户已退出的群、已解散的群不在列表里。
func (a *Admin) queryUserConvs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"userId"`
		Cursor string `json:"cursor"`
		Limit  *int   `json:"limit"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.UserID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	limit := defaultPageLimit
	if req.Limit != nil {
		if *req.Limit != 20 && *req.Limit != 50 && *req.Limit != 100 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 20, 50 or 100"))
			return
		}
		limit = *req.Limit
	}
	after, perr := parseConvCursor(req.Cursor)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	if !a.userMustExist(w, r, req.UserID) {
		return
	}
	rows, next, err := a.store.ListUserConvMeta(r.Context(), req.UserID, after, limit)
	if err != nil {
		a.internal(w, "list user convs", err)
		return
	}
	out := make([]userConvMetaJSON, len(rows))
	for i, c := range rows {
		item := userConvMetaJSON{
			ConvID: c.ConvID, Type: protocol.ConvC2C, PeerID: c.PeerID, MyRole: c.MyRole, MemberCount: c.MemberCount,
			MaxSeq: c.MaxSeq, ReadSeq: c.ReadSeq, UpdatedAt: c.UpdatedAt, LastTS: c.Last.TS, LastFrom: c.Last.From, LastType: c.Last.Type,
		}
		if c.PeerID == "" {
			item.Type = protocol.ConvGroup
			item.GroupID, item.GroupName = strings.TrimPrefix(c.ConvID, "group:"), c.GroupName
		}
		out[i] = item
	}
	cursor := ""
	if next != nil {
		cursor = encodeCursor(strconv.FormatInt(next.UpdatedAt, 10) + "|" + next.ConvID)
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		Convs      []userConvMetaJSON `json:"convs"`
		NextCursor string             `json:"nextCursor"`
	}{out, cursor})
}

func parseConvCursor(raw string) (*store.ConvCursor, *protocol.Error) {
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
	}
	ts, id, ok := strings.Cut(string(b), "|")
	n, err := strconv.ParseInt(ts, 10, 64)
	if !ok || err != nil || id == "" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
	}
	return &store.ConvCursor{UpdatedAt: n, ConvID: id}, nil
}

// statsJSON 是 GET /admin/stats 的响应。
type statsJSON struct {
	Online        statsOnlineJSON `json:"online"`
	NewUsersToday int64           `json:"newUsersToday"`
	MessagesToday int64           `json:"messagesToday"`
	GroupsTotal   int64           `json:"groupsTotal"`
	DayStart      int64           `json:"dayStart"`
	ServerTime    int64           `json:"serverTime"`
}

type statsOnlineJSON struct {
	Total      int            `json:"total"`
	ByPlatform map[string]int `json:"byPlatform"`
	Partial    bool           `json:"partial"`
	Scope      string         `json:"scope"`
}

// stats 是 GET /admin/stats（G-ADM-17，F-ADM-70）。"今日"按 Asia/Shanghai 自然日和 [IM 钟]。不读 /metrics：连接数直接问 sessions.Source，其余查库。只读，不写审计。
// B6 之前连接数只覆盖接收请求的节点（partial: true，和在线设备查询一样）。
func (a *Admin) stats(w http.ResponseWriter, r *http.Request) {
	now := a.clock.Now()
	start := dayStart(now)
	st, err := a.store.AdminStats(r.Context(), start, start+24*3600*1000)
	if err != nil {
		a.internal(w, "admin stats", err)
		return
	}
	online := statsOnlineJSON{ByPlatform: map[string]int{}, Partial: true, Scope: sessions.ScopeLocalNode}
	if a.sessions != nil {
		online.ByPlatform = a.sessions.PlatformCounts()
		for _, n := range online.ByPlatform {
			online.Total += n
		}
	}
	writeJSON(w, http.StatusOK, statsJSON{online, st.NewUsers, st.Messages, st.Groups, start, now.UnixMilli()})
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
