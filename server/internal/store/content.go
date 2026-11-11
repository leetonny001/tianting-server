package store

import "context"

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
