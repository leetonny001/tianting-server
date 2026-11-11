package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/store"
)

// C6 举报处理（PRD-chat F-MOD-04，BE-496；形状见 protocol/ADMIN.md"敏感词和举报"一节）。不跟 IM_MOD_ENABLED 注册：关掉开关之后已有的举报
// 也要能处理完（P-48 ④）。举报人、被举报人都不收任何帧（规则 6）；结案不自动执行任何处置（规则 5）。
// 审计只记 reportId、targetType、reason（理由枚举）和授权前 8 位，不记 note、正文、文件名（AC-MOD-03-8、AC-CHAT-90-5）。

// ReportConfig 是举报处理的依赖。
type ReportConfig struct {
	Cipher        *quarantine.Cipher // IM_REPORT_KEY；没配置时读证据按"无法解密"
	C2CBodyAccess bool               // 读的时候再查一次单聊开关（F-MOD-04 规则 4）
	Objects       ObjectReader       // 对象存储（IM_FILES_ENABLED 开启时非 nil）：证据文件
	Undecryptable prometheus.Counter // im_report_undecryptable_total
}

// WithReports 接上举报处理。要在 Register 之前调用。
func (a *Admin) WithReports(cfg ReportConfig) { a.reports = &cfg }

const (
	reportGrantPrefix   = "report:" // 举报证据授权存在 msg_grants 里（配额合并，第 132 条），conv_id 是 report:{reportId}，不会和真的会话 ID 相同
	defaultReportsPage  = 50
	maxReportsScan      = 10000
	maxConclusionLength = 200
)

func (a *Admin) registerReports(mux *http.ServeMux) {
	if a.reports == nil {
		return
	}
	a.route(mux, "GET", "/admin/reports", adminauth.ScopeReportRead, a.listReports)
	a.route(mux, "GET", "/admin/reports/{reportId}", adminauth.ScopeReportRead, a.getReport)
	a.route(mux, "POST", "/admin/reports/{reportId}/resolve", adminauth.ScopeReportHandle, a.resolveReport)
	a.route(mux, "POST", "/admin/report-grants", adminauth.ScopeReportEvidence, a.createReportGrant)
	a.route(mux, "POST", "/admin/reports/{reportId}/evidence", adminauth.ScopeReportEvidence, a.readEvidence)
}

// reportIsC2C：单聊消息举报和用户举报按单聊对待（P-48 ①），读证据时看 IM_C2C_BODY_ACCESS。
func reportIsC2C(r store.Report) bool {
	return r.TargetType == protocol.ReportTargetUser || strings.HasPrefix(r.ConvID, protocol.ConvC2C+":")
}

func convTypeOf(r store.Report) string {
	if r.ConvID == "" {
		return ""
	}
	if c, err := protocol.ParseConvID(r.ConvID); err == nil {
		return c.Type
	}
	return ""
}

func c2cBodyDisabled() *protocol.Error {
	e := protocol.Errorf(protocol.CodeForbidden, "c2c message body access is disabled")
	e.Reason = "c2cBodyDisabled"
	return e
}

func reportNotFound() *protocol.Error {
	return protocol.Errorf(protocol.CodeNotFound, "report not found")
}

// ---- 队列（规则 1） ----

type reportBrief struct {
	ReportID  string `json:"reportId"`
	Reason    string `json:"reason"`
	State     string `json:"state"`
	CreatedAt int64  `json:"createdAt"`
}

type reportGroupJSON struct {
	TargetKey  string        `json:"targetKey"`
	TargetType string        `json:"targetType"`
	ConvType   string        `json:"convType,omitempty"`
	ConvID     string        `json:"convId,omitempty"`
	MsgIDs     []string      `json:"msgIds,omitempty"`
	UserID     string        `json:"userId,omitempty"`
	GroupID    string        `json:"groupId,omitempty"`
	Count      int           `json:"count"`
	LatestAt   int64         `json:"latestAt"`
	Reports    []reportBrief `json:"reports"`
}

// listReports：按对象聚合（第 117 条，同幂等键），count 是筛选范围内的条数，按 latestAt 倒序。单聊消息举报只有 convType（不带 convId、
// msgIds，参与者要在详情里填理由才看，F-MOD-04 规则 2）。没有举报人、note 和任何正文。游标是偏移量（不透明的字符串）。
func (a *Admin) listReports(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.ReportFilter{State: q.Get("state"), Reason: q.Get("reason"), TargetType: q.Get("targetType"), Limit: maxReportsScan}
	if f.State != "" && !slices.Contains([]string{store.ReportPending, store.ReportHandled, store.ReportRejected}, f.State) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "state must be pending, handled or rejected"))
		return
	}
	if f.Reason != "" && !slices.Contains(protocol.ReportReasons, f.Reason) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid reason"))
		return
	}
	if f.TargetType != "" && !slices.Contains([]string{protocol.ReportTargetMessage, protocol.ReportTargetUser, protocol.ReportTargetGroup}, f.TargetType) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid targetType"))
		return
	}
	for key, dst := range map[string]*int64{"from": &f.From, "to": &f.To} {
		if v := q.Get(key); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "%s must be milliseconds", key))
				return
			}
			*dst = n
		}
	}
	limit := defaultReportsPage
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || (n != 20 && n != 50 && n != 100) {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 20, 50 or 100"))
			return
		}
		limit = n
	}
	offset := 0
	if v := q.Get("cursor"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor"))
			return
		}
		offset = n
	}
	rows, err := a.store.ListReports(r.Context(), f)
	if err != nil {
		a.internal(w, "list reports", err)
		return
	}
	// rows 按 created_at 倒序：每个对象第一次出现的就是最新的，分组之后的顺序就是 latestAt 倒序。
	var groups []*reportGroupJSON
	byKey := map[string]*reportGroupJSON{}
	for _, row := range rows {
		g := byKey[row.DedupKey]
		if g == nil {
			sum := sha256.Sum256([]byte(row.DedupKey))
			g = &reportGroupJSON{TargetKey: hex.EncodeToString(sum[:12]), TargetType: row.TargetType, ConvType: convTypeOf(row), LatestAt: row.CreatedAt}
			switch row.TargetType {
			case protocol.ReportTargetUser:
				g.UserID = row.TargetUser
			case protocol.ReportTargetGroup:
				g.GroupID = row.GroupID
			default:
				if g.ConvType == protocol.ConvGroup {
					g.ConvID, g.MsgIDs = row.ConvID, row.MsgIDs
				}
			}
			byKey[row.DedupKey] = g
			groups = append(groups, g)
		}
		g.Count++
		g.Reports = append(g.Reports, reportBrief{row.ReportID, row.Reason, row.State, row.CreatedAt})
	}
	out := []reportGroupJSON{}
	for i := offset; i < len(groups) && i < offset+limit; i++ {
		out = append(out, *groups[i])
	}
	next := ""
	if offset+limit < len(groups) {
		next = strconv.Itoa(offset + limit)
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		Items      []reportGroupJSON `json:"items"`
		NextCursor string            `json:"nextCursor"`
	}{out, next})
}

// ---- 详情（规则 2） ----

type reportMsgMeta struct {
	MsgID      string `json:"msgId"`
	Seq        int64  `json:"seq,omitempty"`
	From       string `json:"from,omitempty"`
	Type       string `json:"type,omitempty"`
	BodyLength *int   `json:"bodyLength,omitempty"`
	FileSize   int64  `json:"fileSize,omitempty"`
}

// getReport 返回元数据：举报人、被举报人、时间、理由、消息类型和长度、被举报人被举报的总次数。**没有 note、正文和文件名**
// （AC-MOD-04-10、04-11）：元数据从证据里解出来，但只取这几个字段。证据已过期或解不开时 messages 只有 msgId。每次调用写审计。
func (a *Admin) getReport(w http.ResponseWriter, r *http.Request) {
	rep, ok := a.loadReport(w, r)
	if !ok {
		return
	}
	noteAuditFields(r.Context(), map[string]any{"targetType": rep.TargetType, "reason": rep.Reason})
	var msgs []reportMsgMeta
	if rep.TargetType == protocol.ReportTargetMessage {
		meta := map[string]logic.EvidenceMessage{}
		if rows, err := a.store.ReportEvidence(r.Context(), rep.ReportID, a.clock.Now().UnixMilli()); err != nil {
			a.internal(w, "report evidence", err)
			return
		} else if _, ms, err := logic.OpenEvidence(a.reports.Cipher, rep.ReportID, rows); err == nil {
			for _, m := range ms {
				meta[m.MsgID] = m
			}
		}
		for _, id := range rep.MsgIDs {
			m, ok := meta[id]
			if !ok {
				msgs = append(msgs, reportMsgMeta{MsgID: id})
				continue
			}
			n := m.BodyLength
			msgs = append(msgs, reportMsgMeta{MsgID: id, Seq: m.Seq, From: m.From, Type: m.Type, BodyLength: &n, FileSize: m.FileSize})
		}
	}
	var count *int
	if rep.TargetUser != "" {
		n, err := a.store.CountReportsAgainst(r.Context(), rep.TargetUser)
		if err != nil {
			a.internal(w, "count reports", err)
			return
		}
		count = &n
	}
	out := struct {
		ReportID          string          `json:"reportId"`
		TargetType        string          `json:"targetType"`
		ConvType          string          `json:"convType,omitempty"`
		ConvID            string          `json:"convId,omitempty"`
		MsgIDs            []string        `json:"msgIds,omitempty"`
		UserID            string          `json:"userId,omitempty"`
		GroupID           string          `json:"groupId,omitempty"`
		Reporter          string          `json:"reporter"`
		TargetUser        string          `json:"targetUser,omitempty"`
		TargetReportCount *int            `json:"targetReportCount,omitempty"`
		Reason            string          `json:"reason"`
		State             string          `json:"state"`
		CreatedAt         int64           `json:"createdAt"`
		Messages          []reportMsgMeta `json:"messages,omitempty"`
		HandledBy         string          `json:"handledBy,omitempty"`
		HandledAt         int64           `json:"handledAt,omitempty"`
		Conclusion        string          `json:"conclusion,omitempty"`
	}{ReportID: rep.ReportID, TargetType: rep.TargetType, ConvType: convTypeOf(rep), ConvID: rep.ConvID, MsgIDs: rep.MsgIDs, GroupID: rep.GroupID,
		Reporter: rep.Reporter, TargetUser: rep.TargetUser, TargetReportCount: count, Reason: rep.Reason, State: rep.State, CreatedAt: rep.CreatedAt,
		Messages: msgs, HandledBy: rep.HandledBy, HandledAt: rep.HandledAt, Conclusion: rep.Conclusion}
	if rep.TargetType == protocol.ReportTargetUser {
		out.UserID = rep.TargetUser
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *Admin) loadReport(w http.ResponseWriter, r *http.Request) (store.Report, bool) {
	id := r.PathValue("reportId")
	if !protocol.ValidID(id) {
		writeErr(w, reportNotFound())
		return store.Report{}, false
	}
	rep, err := a.store.GetReport(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, reportNotFound())
		return store.Report{}, false
	case err != nil:
		a.internal(w, "get report", err)
		return store.Report{}, false
	}
	return rep, true
}

// ---- 结案（规则 5） ----

func (a *Admin) resolveReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Result != store.ReportHandled && req.Result != store.ReportRejected {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "result must be handled or rejected"))
		return
	}
	conclusion := protocol.TrimText(req.Reason)
	if conclusion == "" || protocol.TextLen(conclusion) > maxConclusionLength {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "reason is required and must be at most %d characters", maxConclusionLength))
		return
	}
	rep, ok := a.loadReport(w, r)
	if !ok {
		return
	}
	noteAuditFields(r.Context(), map[string]any{"targetType": rep.TargetType, "result": req.Result})
	// 条件更新 state='pending'：并发结案只有一个成功，第二个 409 / 4098、状态不变（AC-MOD-04-5）。
	err := a.store.ResolveReport(r.Context(), rep.ReportID, req.Result, conclusion, byOf(AuthFrom(r.Context())), a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrReportHandled):
		writeErr(w, protocol.Errorf(protocol.CodeReportAlreadyHandled, "report already handled"))
		return
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, reportNotFound())
		return
	case err != nil:
		a.internal(w, "resolve report", err)
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

// ---- 证据授权和读取（规则 3、4） ----

// createReportGrant：同 msg-grants 的流程，工单号自动用 reportId，绑定调用方 + 签名的操作人 + reportId，15 分钟，配额合并
// （同一张 msg_grants 表，每操作人 20、每调用方 200）。单聊消息举报和用户举报在 IM_C2C_BODY_ACCESS=off 时 403 c2cBodyDisabled，
// 不产生授权、不占配额（AC-MOD-04-3）；on 时只要 report.evidence（P-48 ⑤）。
func (a *Admin) createReportGrant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportID string `json:"reportId"`
		Reason   string `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	reason, perr := validReason(req.Reason)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	info := AuthFrom(r.Context())
	if info.Operator == "" {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "X-IM-Operator is required"))
		return
	}
	rep, err := a.store.GetReport(r.Context(), req.ReportID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, reportNotFound())
		return
	case err != nil:
		a.internal(w, "get report", err)
		return
	}
	if reportIsC2C(rep) && !a.reports.C2CBodyAccess {
		writeErr(w, c2cBodyDisabled())
		return
	}
	now := a.clock.Now()
	g := store.MsgGrant{
		GrantID: uuid.NewString(), Caller: info.Caller, Operator: info.Operator, ConvID: reportGrantPrefix + rep.ReportID,
		Reason: reason, Ticket: rep.ReportID, CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(msgGrantTTL).UnixMilli(),
	}
	err = a.store.CreateMsgGrant(r.Context(), g, store.MsgGrantQuota{DayStart: dayStart(now), PerOperator: grantsPerOperator, PerCaller: grantsPerCaller})
	switch {
	case errors.Is(err, store.ErrMsgGrantQuota):
		writeErr(w, protocol.Errorf(protocol.CodeQuotaExceeded, "daily message read quota exceeded"))
		return
	case err != nil:
		a.internal(w, "create report grant", err)
		return
	}
	noteAuditFields(r.Context(), map[string]any{"reportId": rep.ReportID, "grant": g.GrantID[:8]})
	writeJSON(w, http.StatusOK, struct {
		GrantID   string `json:"grantId"`
		ExpiresAt int64  `json:"expiresAt"`
	}{g.GrantID, g.ExpiresAt})
}

// reportGrantOK：授权要属于这个调用方 + 操作人 + 这条举报、没过期（任何一条不符合都是 403 / 4039）。
func (a *Admin) reportGrantOK(ctx context.Context, grantID, reportID string, info AuthInfo, now int64) (bool, error) {
	if grantID == "" {
		return false, nil
	}
	g, err := a.store.GetMsgGrant(ctx, grantID)
	if errors.Is(err, store.ErrMsgGrantNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return g.Caller == info.Caller && g.Operator == info.Operator && g.ConvID == reportGrantPrefix+reportID && g.ExpiresAt > now, nil
}

type evidenceMsgJSON struct {
	MsgID      string            `json:"msgId"`
	Seq        int64             `json:"seq"`
	From       string            `json:"from"`
	Type       string            `json:"type"`
	TS         int64             `json:"ts"`
	Body       *protocol.MsgBody `json:"body,omitempty"`
	Recalled   bool              `json:"recalled,omitempty"`
	RecalledAt int64             `json:"recalledAt,omitempty"`
	RecalledBy string            `json:"recalledBy,omitempty"`
}

// readEvidence：只含被举报的消息，不带上下文；body 同 WS 的 MsgBody（含文件名、撤回消息的隔离原文）。密钥不匹配返回
// {messages: [], undecryptable: true}，告警指标加 1，不影响结案（AC-MOD-04-14）。读的时候再查一次单聊开关（AC-MOD-03-10）。
func (a *Admin) readEvidence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var req struct {
		GrantID string `json:"grantId"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	rep, ok := a.loadReport(w, r)
	if !ok {
		return
	}
	info := AuthFrom(r.Context())
	now := a.clock.Now().UnixMilli()
	granted, err := a.reportGrantOK(r.Context(), req.GrantID, rep.ReportID, info, now)
	if err != nil {
		a.internal(w, "check report grant", err)
		return
	}
	if !granted {
		writeErr(w, grantRequired())
		return
	}
	if reportIsC2C(rep) && !a.reports.C2CBodyAccess {
		writeErr(w, c2cBodyDisabled())
		return
	}
	fields := map[string]any{"reportId": rep.ReportID, "grant": req.GrantID[:min(8, len(req.GrantID))]}
	noteAuditFields(r.Context(), fields)
	rows, err := a.store.ReportEvidence(r.Context(), rep.ReportID, now)
	if err != nil {
		a.internal(w, "report evidence", err)
		return
	}
	head, msgs, err := logic.OpenEvidence(a.reports.Cipher, rep.ReportID, rows)
	if err != nil {
		if a.reports.Undecryptable != nil {
			a.reports.Undecryptable.Inc()
		}
		a.log.Warn("report evidence cannot be decrypted", "reportId", rep.ReportID) // 不记错误细节以外的任何东西
		fields["undecryptable"] = true
		writeJSON(w, http.StatusOK, struct {
			Messages      []evidenceMsgJSON `json:"messages"`
			Undecryptable bool              `json:"undecryptable"`
		}{[]evidenceMsgJSON{}, true})
		return
	}
	out := make([]evidenceMsgJSON, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, evidenceMsgJSON{MsgID: m.MsgID, Seq: m.Seq, From: m.From, Type: m.Type, TS: m.TS, Body: m.Body,
			Recalled: m.Recalled, RecalledAt: m.RecalledAt, RecalledBy: m.RecalledBy})
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		Messages []evidenceMsgJSON `json:"messages"`
		Note     *string           `json:"note,omitempty"`
	}{out, head.Note})
}

// evidenceFileOK 是 fetch 的 report.evidence 路径（第 109 条）：授权是这个调用方 + 操作人的、没过期的举报授权，
// 而且这条举报的证据引用了 fileId、msgId、convId。返回 false 时调用方回 403 / 4039。
func (a *Admin) evidenceFileOK(ctx context.Context, g store.MsgGrant, info AuthInfo, convID, msgID, fileID string, now time.Time) (bool, error) {
	if g.Caller != info.Caller || g.Operator != info.Operator || g.ExpiresAt <= now.UnixMilli() {
		return false, nil
	}
	rep, err := a.store.GetReport(ctx, strings.TrimPrefix(g.ConvID, reportGrantPrefix))
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil || rep.ConvID != convID {
		return false, err
	}
	rows, err := a.store.ReportEvidence(ctx, rep.ReportID, now.UnixMilli())
	if err != nil {
		return false, err
	}
	_, msgs, err := logic.OpenEvidence(a.reports.Cipher, rep.ReportID, rows)
	if err != nil {
		return false, nil
	}
	for _, m := range msgs {
		if m.MsgID == msgID && m.FileID == fileID && fileID != "" {
			return true, nil
		}
	}
	return false, nil
}
