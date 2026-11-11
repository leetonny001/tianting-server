package store

import (
	"context"
	"errors"
)

// C6 敏感词和举报的存储层（PRD-chat 第 10 节、第 12.1 节，BE-492～BE-496）。
// 接口名不用 ModerationStore（第二批的封禁、禁言、删除已经占用，backend-chat 第 50 条的教训），分成词库、命中、举报三块。

// WordStore 是词库（F-MOD-01，blocked_words）。
type WordStore interface {
	// CountActiveWords 返回启用的词数：生产 open 模式的启动检查（AC-ACC-01-9）和配置检查的"词库为空"告警（F-MOD-01 规则 6）用。
	CountActiveWords(ctx context.Context) (int, error)
	// ActiveWords 返回全部启用的词（ID、norm、compact），给匹配引擎建快照。停用的词不进自动机（BE-493 第 7 条）。
	ActiveWords(ctx context.Context) ([]WordKey, error)
	// AddWord 单个添加。按 norm 去重（backend-chat 第 100 条）：已有的（含停用的）返回已有的那条、created=false、状态不变。
	AddWord(ctx context.Context, w BlockedWord) (got BlockedWord, created bool, err error)
	// ImportWords 一个事务批量添加，返回新增数；和已有的（含停用的）或同一批前面的 norm 相同的不写，计入 duplicate（第 103 条）。
	ImportWords(ctx context.Context, words []BlockedWord) (added, duplicate int, err error)
	// SetWordState 只改 state；不存在 ErrNotFound。
	SetWordState(ctx context.Context, wordID int64, state string, now int64) (BlockedWord, error)
	// DeleteWord 物理删除；moderation_hits 里的 word_id 保留，不级联。不存在 ErrNotFound。
	DeleteWord(ctx context.Context, wordID int64) error
	// ListWords 按 word_id 倒序分页；Q 非空时按 norm 子串过滤（调用方已标准化）。next 为 0 表示没有更多。
	ListWords(ctx context.Context, q WordQuery) (words []BlockedWord, next int64, err error)
}

// 词的状态。
const (
	WordActive   = "active"
	WordDisabled = "disabled"
)

// BlockedWord 是 blocked_words 的一行。Word 是原文（后台按纯文本显示）；Norm、Compact 由 moderation.Normalize 算好传入。
type BlockedWord struct {
	WordID               int64
	Word, Norm, Compact  string
	State                string
	CreatedAt, UpdatedAt int64
}

// WordKey 是匹配引擎要的部分。
type WordKey struct {
	WordID        int64
	Norm, Compact string
}

// WordQuery 是词库列表的条件；Cursor 是上一页最后一个 word_id（0 表示从最新开始）。
type WordQuery struct {
	Q, State string
	Cursor   int64
	Limit    int
}

// ReportStore 是举报（F-MOD-03、04，reports、report_evidence、report_quota）。
type ReportStore interface {
	// CountPendingReportsBefore 返回 created_at < before 的待处理举报数（im_reports_pending，F-MOD-04 规则 7）。
	CountPendingReportsBefore(ctx context.Context, before int64) (int, error)
	// ReportQuotaUsed 返回这个用户当天（day 是 Asia/Shanghai 当天 00:00 的毫秒）已经用掉的次数（BE-495 第 1 条的预检）。
	ReportQuotaUsed(ctx context.Context, userID string, day int64) (int, error)
	// AddReportQuota 给 4040 的举报计一次（backend-chat 第 115 条：通过格式和开关之后的请求都算），到上限就不再加。
	AddReportQuota(ctx context.Context, userID string, day int64, limit int) error
	// CreateReport 一个事务：幂等（同一举报人、同一 dedup_key、DedupSince 之后已有的，返回它和 dup=true，不计次数）→ 次数条件加 1
	// （到上限 ErrReportQuota）→ reports 行 → 证据（Fault 返回错误就整体回滚）→ 证据引用的文件 evidence_until 取较大值（F-FILE-06 规则 5）。
	CreateReport(ctx context.Context, n NewReport) (r Report, dup bool, err error)
	// GetReport 不存在 ErrNotFound。
	GetReport(ctx context.Context, reportID string) (Report, error)
	// ReportEvidence 返回没过期的证据密文，按 idx。
	ReportEvidence(ctx context.Context, reportID string, now int64) ([]EvidenceRow, error)
	// PurgeReports 是 report_purge：证据过期的物理删除，reports 行在 reportsBefore 之前的整条删除（P-48 ②），次数表清 quotaBefore 之前的。
	PurgeReports(ctx context.Context, evidenceBefore, reportsBefore, quotaBefore int64) (int, error)
	// ReportableUser 报告 reporter 能不能举报 target（F-MOD-03 规则 3）：target 存在，且是好友（任一方向）、或单聊有过消息
	// （conv_seq 有行且 max_seq > 0，含一方删除了会话，第 116 条）、或有共同群。c2cConvID 是两人的单聊会话 ID。
	ReportableUser(ctx context.Context, reporter, target, c2cConvID string) (bool, error)
	// Quarantined 返回一条消息没过期的隔离原文（密文）；没有时 ok=false（举报撤回的消息，AC-MOD-03-5、03-12）。
	Quarantined(ctx context.Context, msgID string, now int64) (q QuarantinedMessage, ok bool, err error)
	// ListReports 按 created_at 倒序返回符合条件的举报（最多 Limit 条），队列在调用方按对象聚合（F-MOD-04 规则 1）。
	ListReports(ctx context.Context, f ReportFilter) ([]Report, error)
	// CountReportsAgainst 是"该用户被举报的总次数"（所有类型，保留期内，第 117 条）。
	CountReportsAgainst(ctx context.Context, userID string) (int, error)
	// ResolveReport 结案：条件更新 state='pending'，没命中返回 ErrReportHandled（4098），不存在 ErrNotFound。
	ResolveReport(ctx context.Context, reportID, result, conclusion, by string, at int64) error
}

// ErrReportHandled 是举报已经结案（409 / 4098，AC-MOD-04-5）。
var ErrReportHandled = errors.New("report already handled")

// ReportFilter 是举报队列的条件；空串、0 表示不过滤。From、To 是毫秒，闭区间。
type ReportFilter struct {
	State, Reason, TargetType string
	From, To                  int64
	Limit                     int
}

// ErrReportQuota 是当天的举报次数用完了（4292，AC-MOD-03-4）。
var ErrReportQuota = errors.New("report daily quota exceeded")

// 举报的状态（F-MOD-04 规则 1）。
const (
	ReportPending  = "pending"
	ReportHandled  = "handled"
	ReportRejected = "rejected"
)

// Report 是 reports 的一行：只有 ID、类型、理由、状态等元数据；note、文件名、正文只在证据密文里（backend-chat 第 122 条）。
type Report struct {
	ReportID, Reporter, TargetType string
	ConvID, TargetUser, GroupID    string
	MsgIDs                         []string
	Reason, DedupKey, State        string
	Conclusion, HandledBy          string
	HandledAt, CreatedAt           int64
}

// EvidenceRow 是 report_evidence 的一行（AES-256-GCM，AAD 绑定 report_id 和 idx）。
type EvidenceRow struct {
	Idx               int
	Nonce, Ciphertext []byte
	KID               string
}

// NewReport 是 CreateReport 的参数。
type NewReport struct {
	Report     Report
	Evidence   []EvidenceRow
	ExpiresAt  int64    // 证据到期（举报时间 + IM_REPORT_EVIDENCE_DAYS），也是文件的 evidence_until
	FileIDs    []string // 证据引用的文件（单聊 off 时为空，第 95 条 ①）
	DedupSince int64    // 幂等的窗口起点（举报时间 − 24 小时）
	DayStart   int64
	DailyLimit int
	Fault      func() error // 测试模式的 evidenceWrite（第 4.5 节）；生产为 nil
}

// HitStore 是命中记录（F-MOD-02 规则 7，moderation_hits）：只有用户、场景、词 ID、时间，不存内容。
type HitStore interface {
	// RecordHit 写一行。写入失败不影响拦截结果（调用方只记日志）。
	RecordHit(ctx context.Context, h ModHit) error
	// UserHits 返回 at >= since 的按场景次数和最近 limit 条（新的在前），给 GET /admin/moderation/hits（backend-chat 第 114 条）。
	UserHits(ctx context.Context, userID string, since int64, limit int) (counts map[string]int, recent []ModHit, err error)
	// PurgeHits 删除 at < before 的行（moderation_hits_purge），返回行数。
	PurgeHits(ctx context.Context, before int64) (int, error)
}

// ModHit 是 moderation_hits 的一行。
type ModHit struct {
	UserID, Scene string
	WordID, At    int64
}
