package store

import (
	"context"
	"encoding/json"
)

// 系统推送的存储（PRD-live v1.0 4.3、4.4 节）。

// 幂等记录的状态。
const (
	PushPending = "pending" // all、ttlSeconds=0：记录已写，广播还没完成，重试时重新广播
	PushDone    = "done"
)

// PushRecord 是一条幂等记录。Accepted 只有 target=user 才有。
type PushRecord struct {
	PushID      string
	Caller      string
	ContentHash string
	Target      string
	Accepted    *int
	State       string
	FirstTS     int64
	CreatedAt   int64
}

// PushInboxWrite 是一次进收件箱的提交：幂等记录、序号、收件箱在同一事务写入。
type PushInboxWrite struct {
	Record    PushRecord
	UserIDs   []string // 存储层按字节序加锁，调用方不用排序
	Type      string
	Data      json.RawMessage
	ExpiresAt int64
	InboxMax  int // 每个用户收件箱的上限，超出淘汰最旧的；<= 0 不限
	// FailAt 只给故障注入用（PRD-live 8.2 节）：写第 i 个用户（按字节序，从 0 起）前调用，返回错误时整批回滚。
	FailAt func(i int) error
}

// PushInboxResult 是一次提交的结果。Existing 非 nil 表示这个 pushId 已有记录（并发提交），这次什么都没写。
type PushInboxResult struct {
	Seqs     map[string]int64
	Evicted  int
	Existing *PushRecord
}

// PushInboxRow 是收件箱的一行。
type PushInboxRow struct {
	UserID    string
	PushSeq   int64
	PushID    string
	Type      string
	Data      json.RawMessage
	CreatedAt int64
	ExpiresAt int64
}

// PushStore 是系统推送用到的存储。
type PushStore interface {
	// GetPushRecord 不存在时返回 ErrNotFound。
	GetPushRecord(ctx context.Context, pushID string) (PushRecord, error)
	// WritePushInbox 在同一事务里写幂等记录（state=done）、分配序号、写收件箱并淘汰超限的行。
	WritePushInbox(ctx context.Context, w PushInboxWrite) (PushInboxResult, error)
	// CreatePendingPush 写一条 state=pending 的幂等记录。pushId 已有记录时什么都不写，返回那条记录。
	CreatePendingPush(ctx context.Context, r PushRecord) (existing *PushRecord, err error)
	// MarkPushDone 把幂等记录改成 done。
	MarkPushDone(ctx context.Context, pushID string) error
	// ReadPushInbox 在同一读事务里返回 pushSeq > afterSeq、在 now 时还没到期的行（升序，最多 limit 条）、后面是否还有和 latestSeq。
	ReadPushInbox(ctx context.Context, userID string, afterSeq int64, limit int, now int64) (rows []PushInboxRow, hasMore bool, latestSeq int64, err error)
	// LatestPushSeq 返回用户分配过的最大序号，从来没有时为 0。不随清理回退。
	LatestPushSeq(ctx context.Context, userID string) (int64, error)
	// ListPushInbox 返回用户收件箱的全部行，包括已到期还没清理的（测试接口用）。
	ListPushInbox(ctx context.Context, userID string) ([]PushInboxRow, error)
	// CountPushInbox 返回收件箱总行数。
	CountPushInbox(ctx context.Context) (int64, error)
	// PurgePush 清理一批：先删 expires_at <= now 的收件箱行（最多 batch 行），再删 created_at <= recordsBefore 且没有
	// 对应收件箱行的幂等记录（最多 batch 行）。先后顺序保证清理落后时不会撞 (user_id, push_id) 唯一键（4.4 节）。
	PurgePush(ctx context.Context, now, recordsBefore int64, batch int) (inbox, records int, err error)
}
