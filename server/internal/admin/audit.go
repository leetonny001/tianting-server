package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

const (
	maxAuditTargets = 50                  // 超过时只记前 50 个和总数（PRD-admin F-KEY-04 规则 2）
	maxAuditSpan    = 31 * 24 * time.Hour // 审计查询的最大时间范围
	maxAuditLimit   = 200
)

// auditedReadScopes 是虽然只读、也要审计的 scope（PRD-admin F-KEY-04 规则 1）：
// 在线设备、消息元数据、正文、审计本身。普通的 user.read / group.read 列表不记录。
var auditedReadScopes = map[adminauth.Scope]bool{
	adminauth.ScopeUserSessions: true, adminauth.ScopeMsgMeta: true, adminauth.ScopeMsgBodyGroup: true,
	adminauth.ScopeMsgBodyC2C: true, adminauth.ScopeMsgQuarantine: true, adminauth.ScopeAuditRead: true,
}

func audited(method string, scope adminauth.Scope) bool {
	if scope == adminauth.ScopeTest {
		return false
	}
	return method != http.MethodGet || auditedReadScopes[scope]
}

// withAudit 在执行前写 intent，执行后写 result（同一 requestId）。intent 写失败时不执行，返回 503 / 5030。
// 摘要用白名单构造：只记目标 ID、数量和请求里出现了哪些字段名，不记字段值（正文、备注、推送 data 都不会进审计）。
func (a *Admin) withAudit(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info := AuthFrom(r.Context())
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "request body too large or unreadable"))
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		targets, summary := auditDigest(r, body)
		entry := store.AdminAudit{
			TS: a.clock.Now().UnixMilli(), Caller: info.Caller, Operator: info.Operator, RequestID: info.RequestID,
			Route: route, TargetsJSON: targets, SummaryJSON: summary, Phase: store.AuditIntent,
		}
		if err := a.appendAudit(r, entry); err != nil {
			a.log.Error("admin audit intent write failed; request rejected", "route", route, "err", err)
			writeErr(w, protocol.Errorf(protocol.CodeServiceUnavailable, "audit unavailable, try again later"))
			return
		}
		rec := &codeRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		entry.TS, entry.Phase, entry.Code = a.clock.Now().UnixMilli(), store.AuditResult, rec.code()
		if err := a.appendAudit(r, entry); err != nil {
			// 业务已经执行完，不能再回滚：记日志告警，对账（console-server F-CON-09）会发现缺失的 result。
			a.log.Error("admin audit result write failed", "route", route, "requestId", info.RequestID, "err", err)
		}
	}
}

func (a *Admin) appendAudit(r *http.Request, e store.AdminAudit) error {
	if a.auditFail.Load() {
		return errInjectedAuditFailure
	}
	return a.store.AppendAdminAudit(r.Context(), e)
}

// auditDigest 返回 targets_json 和 summary_json。目标来自路径参数和请求体里的 userId / groupId / members / users[].userId。
func auditDigest(r *http.Request, body []byte) (targets, summary string) {
	var ids []string
	add := func(id string) {
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	add(r.PathValue("userId"))
	add(r.PathValue("groupId"))
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(body, &fields)
	var s string
	if json.Unmarshal(fields["userId"], &s) == nil {
		add(s)
	}
	if json.Unmarshal(fields["groupId"], &s) == nil {
		add(s)
	}
	var members []string
	if json.Unmarshal(fields["members"], &members) == nil {
		for _, m := range members {
			add(m)
		}
	}
	var users []struct {
		UserID string `json:"userId"`
	}
	if json.Unmarshal(fields["users"], &users) == nil {
		for _, u := range users {
			add(u.UserID)
		}
	}
	t := struct {
		IDs   []string `json:"ids"`
		Count int      `json:"count"`
	}{IDs: ids, Count: len(ids)}
	if len(ids) > maxAuditTargets {
		t.IDs = ids[:maxAuditTargets]
	}
	if t.IDs == nil {
		t.IDs = []string{}
	}
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	tb, _ := json.Marshal(t)
	sb, _ := json.Marshal(struct {
		Fields []string `json:"fields"`
	}{names})
	return string(tb), string(sb)
}

// codeRecorder 记下响应状态码和错误响应里的业务码（错误响应体是 {"code":N,"msg":...}）。
type codeRecorder struct {
	http.ResponseWriter
	status int
	head   []byte
}

func (c *codeRecorder) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *codeRecorder) Write(b []byte) (int, error) {
	if len(c.head) < 512 {
		c.head = append(c.head, b[:min(len(b), 512-len(c.head))]...)
	}
	return c.ResponseWriter.Write(b)
}

func (c *codeRecorder) code() int {
	if c.status < 400 {
		return 0
	}
	var e struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(c.head, &e) == nil && e.Code != 0 {
		return e.Code
	}
	return c.status
}

type auditError string

func (e auditError) Error() string { return string(e) }

const errInjectedAuditFailure = auditError("injected admin_audit write failure")

// queryAudit 是 GET /admin/audit（scope audit.read）：from / to 必填（毫秒）且跨度不超过 31 天，按 id 倒序游标分页。
func (a *Admin) queryAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err1 := strconv.ParseInt(q.Get("from"), 10, 64)
	to, err2 := strconv.ParseInt(q.Get("to"), 10, 64)
	if err1 != nil || err2 != nil || from > to || time.Duration(to-from)*time.Millisecond > maxAuditSpan {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "from and to (ms) are required, from <= to, and the range must be at most 31 days"))
		return
	}
	limit, cursor := 50, int64(0)
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxAuditLimit {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1..%d", maxAuditLimit))
			return
		}
		limit = n
	}
	if v := q.Get("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor"))
			return
		}
		cursor = n
	}
	entries, next, err := a.store.QueryAdminAudit(r.Context(), store.AdminAuditQuery{
		FromMs: from, ToMs: to, Caller: q.Get("caller"), Operator: q.Get("operator"), RequestID: q.Get("requestId"),
		Cursor: cursor, Limit: limit,
	})
	if err != nil {
		a.internal(w, "query admin audit", err)
		return
	}
	type item struct {
		ID        int64           `json:"id"`
		TS        int64           `json:"ts"`
		Caller    string          `json:"caller"`
		Operator  string          `json:"operator"`
		RequestID string          `json:"requestId"`
		Route     string          `json:"route"`
		Targets   json.RawMessage `json:"targets"`
		Summary   json.RawMessage `json:"summary"`
		Phase     string          `json:"phase"`
		Code      int             `json:"code"`
	}
	out := make([]item, len(entries))
	for i, e := range entries {
		out[i] = item{e.ID, e.TS, e.Caller, e.Operator, e.RequestID, e.Route, json.RawMessage(e.TargetsJSON), json.RawMessage(e.SummaryJSON), e.Phase, e.Code}
	}
	writeJSON(w, http.StatusOK, struct {
		Entries    []item `json:"entries"`
		NextCursor int64  `json:"nextCursor"`
	}{out, next})
}
