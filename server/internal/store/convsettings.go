package store

import (
	"context"
	"errors"
)

// C5 会话置顶、免打扰、删除的存储层（PRD-chat 第 9 节、第 12.1 节，BE-483～BE-485）。

// ErrPinLimit 是置顶数已满（4220，F-CVS-01 规则 2）：在写入的同一事务里数当前置顶数。
var ErrPinLimit = errors.New("pin limit exceeded")

// ConvSetting 是 conv_settings 的一行。只有设置过的会话才有行；没有行时等于全 0 的值。
type ConvSetting struct {
	ConvID     string
	PinnedAt   int64 // > 0 表示置顶（服务端时间）
	NotifyOff  bool
	DeletedAt  int64 // > 0 表示删除过（F-CVS-03 规则 2）
	ClearedSeq int64 // 只增不减；minSeq = max(原值, ClearedSeq + 1)
}

// ConvSetReq 是 convSet 的写入（F-CVS-01、02）。Pinned、NotifyOff 为 nil 表示不改。
// 原来没有置顶时 pinned_at = Now；已置顶的再次置顶不变（AC-CVS-01-2）；置顶数超过 PinMax 返回 ErrPinLimit。
// 群会话在同一事务里再确认一次成员身份（不是成员返回 ErrNotGroupMember），不给已经离开的人留下设置。
type ConvSetReq struct {
	UserID, ConvID    string
	Pinned, NotifyOff *bool
	Now               int64
	PinMax            int
}

// ConvDeleteReq 是 convDelete 的写入（F-CVS-03 规则 1～3）：cleared = min(UpToSeq, maxSeq)，cleared_seq = max(原值, cleared)，
// deleted_at = Now，已读推进到 max(readSeq, cleared_seq)，清除置顶，保留免打扰。
type ConvDeleteReq struct {
	UserID, ConvID string
	UpToSeq        int64
	Now            int64
}

// ConvSettingsStore 是 C5 用到的存储。按成员、按群删除设置不单独暴露：退群、被移出、解散在各自的事务里一并删除
// （RemoveGroupMembers、KickMembers、LeaveGroup、解散，F-CVS-01 规则 6）。
type ConvSettingsStore interface {
	// ConvSettings 按用户一次取齐全部设置（convId → 设置）。
	ConvSettings(ctx context.Context, userID string) (map[string]ConvSetting, error)
	// SetConvSettings 写置顶、免打扰（一个事务），返回写入后的设置。错误：ErrPinLimit、ErrNotGroupMember。
	SetConvSettings(ctx context.Context, req ConvSetReq) (ConvSetting, error)
	// DeleteConv 删除会话（一个事务，加锁顺序 conv_seq → conv_settings），返回生效后的设置。群会话不是成员返回 ErrNotGroupMember。
	DeleteConv(ctx context.Context, req ConvDeleteReq) (ConvSetting, error)
	// DeletedConvs 返回当前隐藏的会话（deleted_at > 0 AND max_seq <= cleared_seq），按 deleted_at 倒序最多 limit 个，
	// 超出时 truncated=true（F-CVS-03 规则 6，backend-chat 第 79 条）。
	DeletedConvs(ctx context.Context, userID string, limit int) (convs []ConvSetting, truncated bool, err error)
	// ConvClearedSeq 返回用户在这个会话的 cleared_seq（没有行时 0）：authorize 和文件地址按它算 minSeq（F-CVS-03 规则 2）。
	ConvClearedSeq(ctx context.Context, userID, convID string) (int64, error)
	// NotifyOffUsers 按会话一次取一批用户里设了免打扰的（给 C3 的推送 worker，F-APN-02 规则 7，BE-490）。
	NotifyOffUsers(ctx context.Context, convID string, userIDs []string) (map[string]bool, error)
	// TotalUnread 是总未读的服务端口径（F-CVS-02 规则 4）：普通会话的 unread 之和 + 免打扰会话的 mentionUnread 之和。
	// 隐藏的会话未读是 0。C3 的角标用它（F-APN-04 规则 2，BE-490 第 3 条）。
	TotalUnread(ctx context.Context, userID string) (int64, error)
}
