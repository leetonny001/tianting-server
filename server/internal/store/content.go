package store

import "context"

// C6 敏感词和举报的存储层（PRD-chat 第 10 节、第 12.1 节，BE-492～BE-496）。
// 接口名不用 ModerationStore（第二批的封禁、禁言、删除已经占用，backend-chat 第 50 条的教训），分成词库、命中、举报三块。

// WordStore 是词库（F-MOD-01，blocked_words）。
type WordStore interface {
	// CountActiveWords 返回启用的词数：生产 open 模式的启动检查（AC-ACC-01-9）和配置检查的"词库为空"告警（F-MOD-01 规则 6）用。
	CountActiveWords(ctx context.Context) (int, error)
}

// ReportStore 是举报（F-MOD-03、04，reports、report_evidence、report_quota）。
type ReportStore interface {
	// CountPendingReportsBefore 返回 created_at < before 的待处理举报数（im_reports_pending，F-MOD-04 规则 7）。
	CountPendingReportsBefore(ctx context.Context, before int64) (int, error)
}
