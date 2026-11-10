package store

import (
	"context"
	"errors"
)

// C4 撤回、引用、@ 的存储层（PRD-chat 第 8 节、第 12.1 节，BE-471～BE-477）。

// 撤回、@ 的存储层错误。
var (
	// ErrMentionAllCounter 是 @ 所有人的计数不可用（503 / 5030，不放行，F-MEN-02 规则 2）。
	ErrMentionAllCounter = errors.New("mention-all counter unavailable")
	// ErrMentionWrite 是提及表写失败（故障注入，AC-MEN-03-8）：整条消息不保存，5000。
	ErrMentionWrite = errors.New("mention write failed")
)

// MentionAllLimitError 是 @ 所有人次数用完（4293）。RetryAfter 是还要等的秒数（details.retryAfterSeconds）。
type MentionAllLimitError struct{ RetryAfter int64 }

func (e *MentionAllLimitError) Error() string { return "mention-all limit reached" }

// MentionAllQuota 是 SaveMessage 里扣 @ 所有人次数的参数（第 4.3 节第 10 步）：每群滚动 60 分钟 Hourly 次、
// Asia/Shanghai 自然日 Daily 次（backend-chat 第 58 条）。Day 是当天的 yyyymmdd，DayEnd 是次日 00:00 的毫秒。
type MentionAllQuota struct {
	GroupID       string
	Hourly, Daily int
	Day           int64
	DayEnd        int64
}

// 故障注入点（测试模式，第 4.5 节）：SaveMessage 的 Message.Fault 收到这些名字，返回错误时整个事务回滚。
const (
	FaultMentionWrite      = "mentionWrite"
	FaultMentionAllCounter = "mentionAllCounter"
)

// RecallReq 是撤回一条消息（F-RCL-01 规则 5）。权限、时限由调用方在之前判断；这里在同一个事务里再看一次状态。
type RecallReq struct {
	ConvID, MsgID string
	By            string // 操作者 userId
	Now           int64
	ExpiresAt     int64 // 隔离原文和文件对象的保留期到期时间（CH-17，30 天）
	Seal          SealFunc
}

// RecallResult：Already=true 表示已经撤回过（幂等，不再发事件）。
type RecallResult struct {
	Seq     int64
	Already bool
}

// MentionCount 是一个会话里未读的提及：数量和第一条的 seq（F-MEN-03 规则 1）。
type MentionCount struct {
	Unread int64
	First  int64
}

// MsgActionStore 是 C4 用到的存储。
type MsgActionStore interface {
	// GetMessage 读一条消息（含已删除、已撤回的）。不存在返回 ErrNotFound。
	GetMessage(ctx context.Context, msgID string) (Message, error)
	// GetMessages 批量读，不存在的不在结果里（组装引用，F-QUO-01 规则 3）。
	GetMessages(ctx context.Context, msgIDs []string) (map[string]Message, error)
	// RecallMessage 在一个事务里：清空正文、写 recalled_at / recalled_by（seq 不变）；原文和文件名加密进隔离区（Seal 失败整个事务回滚，
	// AC-RCL-01-11）；文件转 revoked、清空文件名；是最后一条时清空摘要、写 last_recalled*；删除这条的提及计数；递增 mod_ver。
	// 错误：ErrNotFound（不存在、不在这个会话、已被后台删除）、ErrMsgNotDeletable（tip）。已经撤回的返回 Already=true，不改任何东西。
	RecallMessage(ctx context.Context, req RecallReq) (RecallResult, error)
	// MentionCounts 返回用户所在的群里 seq > max(已读, 入群) 的提及（@ 个人 + 别人发的 @ 所有人），按会话。一次查询，不按会话逐条查。
	MentionCounts(ctx context.Context, userID string) (map[string]MentionCount, error)
}
