package logic

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/store"
)

// 举报提交（F-MOD-03，BE-495）。校验顺序：格式 → 开关（content.go 的 report）→ 每天限额 → 对象校验 → 幂等 → 写入。
// 被举报人、举报人都不收任何帧（规则 7）；日志只记 reportId、targetType、reason，不记 note、正文、文件名（AC-MOD-03-8）。

// EvidenceHead 是证据的第 0 条（idx 0）：举报人的 note（按正文对待，只在密文里，backend-chat 第 122 条）和读的时候要的口径。
type EvidenceHead struct {
	Note *string `json:"note,omitempty"`
	// C2C：单聊消息举报和用户举报按单聊对待，读证据时看 IM_C2C_BODY_ACCESS（P-48 ①）。
	C2C bool `json:"c2c"`
}

// EvidenceMessage 是一条被举报消息的快照（idx 1..）。元数据（类型、长度、大小）一直有；Body 只在群消息、或单聊在
// IM_C2C_BODY_ACCESS=on 时有（F-MOD-03 规则 5）；已撤回、被删除的用隔离原文，隔离原文已过期的没有 Body（AC-MOD-03-12）。
type EvidenceMessage struct {
	MsgID      string            `json:"msgId"`
	Seq        int64             `json:"seq"`
	From       string            `json:"from"`
	Type       string            `json:"type"`
	TS         int64             `json:"ts"`
	BodyLength int               `json:"bodyLength"`
	FileSize   int64             `json:"fileSize,omitempty"`
	Body       *protocol.MsgBody `json:"body,omitempty"`
	FileID     string            `json:"fileId,omitempty"` // 证据引用的文件（POST /admin/files/{fileId}/fetch 的 report.evidence 路径）
	Recalled   bool              `json:"recalled,omitempty"`
	RecalledAt int64             `json:"recalledAt,omitempty"`
	RecalledBy string            `json:"recalledBy,omitempty"`
	Deleted    bool              `json:"deleted,omitempty"`
}

// evidenceAAD 是证据密文的附加数据第二段：quarantine.Cipher 的 AAD 是 "a|b"，这里 a 是 reportId、b 是 idx，
// 密文搬到别的举报或别的位置上解不开。
func evidenceAAD(idx int) string { return strconv.Itoa(idx) }

// OpenEvidence 解开一条举报的证据（管理接口用）。密钥不匹配（换了 IM_REPORT_KEY）或密文坏了返回错误，由调用方显示"证据无法解密"。
func OpenEvidence(c *quarantine.Cipher, reportID string, rows []store.EvidenceRow) (EvidenceHead, []EvidenceMessage, error) {
	var head EvidenceHead
	var msgs []EvidenceMessage
	if c == nil {
		return head, nil, errors.New("IM_REPORT_KEY is not configured")
	}
	for _, r := range rows {
		plain, err := c.Open(reportID, evidenceAAD(r.Idx), r.Nonce, r.Ciphertext, r.KID)
		if err != nil {
			return EvidenceHead{}, nil, err
		}
		if r.Idx == 0 {
			if err := json.Unmarshal(plain, &head); err != nil {
				return EvidenceHead{}, nil, err
			}
			continue
		}
		var m EvidenceMessage
		if err := json.Unmarshal(plain, &m); err != nil {
			return EvidenceHead{}, nil, err
		}
		msgs = append(msgs, m)
	}
	return head, msgs, nil
}

func reportTargetInvalid() *protocol.Error {
	// 不存在、不是参与者、看不到、没有关联：响应逐字节相同（F-MOD-03 规则 2、3，AC-MOD-03-2、03-7、03-13）。
	return protocol.WithReason(protocol.CodeNotFound, protocol.ReasonReportTargetInvalid, "report target not found")
}

// reportPlan 是对象校验通过后要写的东西。
type reportPlan struct {
	report  store.Report
	head    EvidenceHead
	msgs    []EvidenceMessage
	fileIDs []string
}

func shanghaiDay(t time.Time) int64 {
	y, m, d := t.In(shanghai).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, shanghai).UnixMilli()
}

var errInjectedEvidence = errors.New("injected evidence write failure")

func (s *Service) submitReport(ctx context.Context, c delivery.Session, req protocol.ReportReq) (any, *protocol.Error) {
	uid := c.UserID()
	now := s.clock.Now()
	day := shanghaiDay(now)
	// 每天限额（规则 4，第 115 条）：先预检；写入事务里还会条件加 1，并发也不超。
	used, err := s.store.ReportQuotaUsed(ctx, uid, day)
	if err != nil {
		return nil, s.internal("report quota", err)
	}
	if used >= s.mod.cfg.ReportDaily {
		return nil, quotaExceeded()
	}
	plan, ok, perr := s.reportTarget(ctx, uid, req, now)
	if perr != nil {
		return nil, perr
	}
	if !ok {
		// 4040 也计次数（防止用举报试探关系，响应本来就相同，第 115 条）。
		if err := s.store.AddReportQuota(ctx, uid, day, s.mod.cfg.ReportDaily); err != nil {
			return nil, s.internal("report quota", err)
		}
		return nil, reportTargetInvalid()
	}
	r := plan.report
	r.ReportID = "rp_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	r.Reporter, r.Reason, r.CreatedAt = uid, req.Reason, now.UnixMilli()
	plan.head.Note = req.Note
	rows, err := s.sealEvidence(r.ReportID, plan.head, plan.msgs)
	if err != nil {
		return nil, s.internal("seal evidence", err)
	}
	var fault func() error
	if s.faults.evidenceWriteFails() {
		fault = func() error { return errInjectedEvidence }
	}
	expires := now.Add(time.Duration(s.mod.cfg.EvidenceDays) * 24 * time.Hour).UnixMilli()
	got, dup, err := s.store.CreateReport(ctx, store.NewReport{
		Report: r, Evidence: rows, ExpiresAt: expires, FileIDs: plan.fileIDs, DedupSince: now.Add(-24 * time.Hour).UnixMilli(),
		DayStart: day, DailyLimit: s.mod.cfg.ReportDaily, Fault: fault,
	})
	switch {
	case errors.Is(err, store.ErrReportQuota):
		return nil, quotaExceeded()
	case err != nil:
		// 证据写失败：举报失败（5000），没有记录、不计次数（同一事务，AC-MOD-03-14）。
		return nil, s.internal("create report", err)
	}
	s.log.Info("report", "reportId", got.ReportID, "targetType", got.TargetType, "reason", got.Reason, "duplicate", dup)
	return protocol.ReportAck{ReportID: got.ReportID}, nil
}

func (s *Service) sealEvidence(reportID string, head EvidenceHead, msgs []EvidenceMessage) ([]store.EvidenceRow, error) {
	parts := make([]any, 0, len(msgs)+1)
	parts = append(parts, head)
	for _, m := range msgs {
		parts = append(parts, m)
	}
	rows := make([]store.EvidenceRow, len(parts))
	for i, p := range parts {
		plain, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		nonce, ct, kid, err := s.mod.evidence.Seal(reportID, evidenceAAD(i), plain)
		if err != nil {
			return nil, err
		}
		rows[i] = store.EvidenceRow{Idx: i, Nonce: nonce, Ciphertext: ct, KID: kid}
	}
	return rows, nil
}

// reportTarget 是对象校验（规则 2、2a、3）。ok=false 是 4040；perr 只用于内部错误。
func (s *Service) reportTarget(ctx context.Context, uid string, req protocol.ReportReq, now time.Time) (reportPlan, bool, *protocol.Error) {
	switch req.TargetType {
	case protocol.ReportTargetUser:
		if req.UserID == uid {
			return reportPlan{}, false, nil
		}
		ok, err := s.store.ReportableUser(ctx, uid, req.UserID, protocol.C2CConvID(uid, req.UserID))
		if err != nil {
			return reportPlan{}, false, s.internal("reportable user", err)
		}
		// 用户举报只存元数据（对象 ID、举报时间），不快照昵称（P-48 ①）；note 按单聊对待。
		return reportPlan{report: store.Report{TargetType: req.TargetType, TargetUser: req.UserID, DedupKey: "u:" + req.UserID}, head: EvidenceHead{C2C: true}}, ok, nil
	case protocol.ReportTargetGroup:
		// 第一批只要求当前成员（规则 3）：非成员和不存在、已解散的群响应相同。不快照群名、公告（P-48 ①）。
		_, ok, err := s.store.GroupMember(ctx, req.GroupID, uid)
		if err != nil {
			return reportPlan{}, false, s.internal("group member", err)
		}
		return reportPlan{report: store.Report{TargetType: req.TargetType, GroupID: req.GroupID, DedupKey: "g:" + req.GroupID}}, ok, nil
	}
	return s.reportMessages(ctx, uid, req, now)
}

// reportMessages：举报人是参与者，消息都在这个会话、seq ≥ 他的 minSeq（authorize，C5 的删除会话也算在内）；已撤回、被删除的要
// readSeq ≥ seq（规则 2a，防止没看过的人只凭 msgId 把保留期延长）；tip 不能举报。
func (s *Service) reportMessages(ctx context.Context, uid string, req protocol.ReportReq, now time.Time) (reportPlan, bool, *protocol.Error) {
	minSeq, perr := s.authorize(ctx, uid, req.ConvID)
	if perr != nil {
		if perr.Code == protocol.CodeInternal {
			return reportPlan{}, false, perr
		}
		return reportPlan{}, false, nil
	}
	conv, _ := protocol.ParseConvID(req.ConvID)
	c2c := conv.Type == protocol.ConvC2C
	// 单聊在 IM_C2C_BODY_ACCESS=off 时只存元数据：不存正文、文件名和文件引用，也就不延长文件保留（F-MOD-03 规则 5，第 95 条 ①）。
	keepBody := !c2c || s.mod.cfg.C2CBodyAccess
	ids := slices.Clone(req.MsgIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	readSeq := int64(-1)
	plan := reportPlan{head: EvidenceHead{C2C: c2c}}
	for _, id := range ids {
		m, err := s.store.GetMessage(ctx, id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			return reportPlan{}, false, nil
		case err != nil:
			return reportPlan{}, false, s.internal("get message", err)
		}
		if m.ConvID != req.ConvID || m.Seq < minSeq || m.Type == protocol.MsgTip {
			return reportPlan{}, false, nil
		}
		gone := m.RecalledAt > 0 || m.DeletedAt > 0
		if gone {
			if readSeq < 0 {
				if readSeq, err = s.store.ReadSeq(ctx, uid, req.ConvID); err != nil {
					return reportPlan{}, false, s.internal("read seq", err)
				}
			}
			if readSeq < m.Seq {
				return reportPlan{}, false, nil
			}
		}
		em, fileID, perr := s.snapshotMessage(ctx, m, gone, keepBody, now)
		if perr != nil {
			return reportPlan{}, false, perr
		}
		if fileID != "" {
			plan.fileIDs = append(plan.fileIDs, fileID)
		}
		plan.msgs = append(plan.msgs, em)
	}
	slices.SortFunc(plan.msgs, func(a, b EvidenceMessage) int { return int(a.Seq - b.Seq) })
	plan.report = store.Report{TargetType: req.TargetType, ConvID: req.ConvID, MsgIDs: ids, TargetUser: plan.msgs[0].From,
		DedupKey: "m:" + req.ConvID + ":" + strings.Join(ids, ",")}
	return plan, true, nil
}

// snapshotMessage 生成一条消息的证据快照，返回证据引用的文件（只有存了正文时才有）。
func (s *Service) snapshotMessage(ctx context.Context, m store.Message, gone, keepBody bool, now time.Time) (EvidenceMessage, string, *protocol.Error) {
	em := EvidenceMessage{MsgID: m.MsgID, Seq: m.Seq, From: m.From, Type: m.Type, TS: m.TS,
		Recalled: m.RecalledAt > 0, RecalledAt: m.RecalledAt, RecalledBy: m.RecalledBy, Deleted: m.DeletedAt > 0}
	var body protocol.MsgBody
	have := false
	if !gone {
		_ = json.Unmarshal(m.Body, &body)
		have = true
		// 元数据：长度和管理接口的元数据一致（文本按码点，其他按 body JSON），文件取服务端记录的大小。
		if m.Type == protocol.MsgText {
			em.BodyLength = protocol.TextLen(body.Text)
		} else {
			em.BodyLength = protocol.TextLen(string(m.Body))
		}
		if id := body.FileID(); id != "" {
			if f, err := s.store.GetFile(ctx, id); err == nil {
				em.FileSize = f.Size
			} else if !errors.Is(err, store.ErrNotFound) {
				return em, "", s.internal("get file", err)
			}
		}
	}
	if !keepBody {
		return em, "", nil
	}
	if gone {
		// 隔离原文还在就用它（含文件名，AC-MOD-03-5）；已过期或者没有隔离加密器的只有元数据（AC-MOD-03-12 后半）。
		q, ok, err := s.store.Quarantined(ctx, m.MsgID, now.UnixMilli())
		if err != nil {
			return em, "", s.internal("quarantined", err)
		}
		if !ok || s.actions.cipher == nil {
			return em, "", nil
		}
		plain, err := s.actions.cipher.Open(m.MsgID, m.ConvID, q.Nonce, q.Ciphertext, q.KID)
		if err != nil {
			s.log.Warn("open quarantined message for report evidence", "msgId", m.MsgID, "err", err)
			return em, "", nil
		}
		body = protocol.MsgBody{}
		_ = json.Unmarshal(plain, &body)
		have = true
	} else {
		pm := []protocol.Message{ToAdminProto(m)}
		if err := EnrichFiles(ctx, s.store, pm); err != nil {
			return em, "", s.internal("load files", err)
		}
		body = pm[0].Body
	}
	if !have {
		return em, "", nil
	}
	body.Quote = nil // 不带上下文（F-MOD-04 规则 2）
	em.Body = &body
	em.FileID = body.FileID()
	return em, em.FileID, nil
}

// PurgeReports 是定时任务 report_purge（单节点）：证据过期（IM_REPORT_EVIDENCE_DAYS）的物理删除，reports 行和证据一起整条删除
// （P-48 ②，backend-chat 第 119 条）；次数表清两天前的。不跟 IM_MOD_ENABLED（P-48 ④）。
func (s *Service) PurgeReports(ctx context.Context) (int, error) {
	now := s.clock.Now()
	return s.store.PurgeReports(ctx, now.UnixMilli(), now.Add(-time.Duration(s.mod.cfg.EvidenceDays)*24*time.Hour).UnixMilli(),
		shanghaiDay(now.Add(-48*time.Hour)))
}
