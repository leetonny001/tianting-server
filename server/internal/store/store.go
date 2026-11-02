// Package store 定义持久化接口。MVP 用 SQLite 实现，后续可换 Postgres。
package store

import (
	"context"
	"encoding/json"
	"errors"
)

// 存储层错误。
var (
	ErrNotFound      = errors.New("not found")
	ErrExists        = errors.New("already exists")
	ErrLimitExceeded = errors.New("limit exceeded")
)

// Message 是持久化的一条消息。Body 是原样保存的 JSON。
type Message struct {
	MsgID       string
	ClientMsgID string
	ConvID      string
	Seq         int64
	From        string
	Type        string
	Body        json.RawMessage
	TS          int64
}

// Group 是群（PRD F-GRP-01）。
type Group struct {
	GroupID        string
	Name           string
	HistoryVisible bool // 新成员能否看到入群前的历史（D2），查询时生效
	CreatedAt      int64
}

// GroupMember 是群成员。JoinSeq 是入群时会话的 maxSeq。
type GroupMember struct {
	UserID  string
	JoinSeq int64
}

// GroupUpdate 是修改群设置的参数，nil 表示不修改。
type GroupUpdate struct {
	Name           *string
	HistoryVisible *bool
}

// User 是 IM 用户（PRD F-USER-01）。
type User struct {
	UserID    string
	Nickname  string
	Avatar    string
	CreatedAt int64
}

// Store 是 IM 服务端用到的全部持久化操作。
type Store interface {
	// SaveMessage 为消息分配会话内递增 seq 并保存。
	// 同一发送者重复提交同一 ClientMsgID 时不新建，返回第一次保存的消息且 dup=true。
	SaveMessage(ctx context.Context, m Message) (saved Message, dup bool, err error)
	// FindByClientMsgID 按（发送者，clientMsgId）查找已保存的消息，用于在限流前先识别重发。
	FindByClientMsgID(ctx context.Context, from, clientMsgID string) (Message, bool, error)
	// ListMessages 返回 seq > afterSeq 的消息（升序，最多 limit 条），以及后面是否还有。
	ListMessages(ctx context.Context, convID string, afterSeq int64, limit int) (msgs []Message, hasMore bool, err error)
	// ListMessagesBefore 返回 minSeq <= seq < beforeSeq 中最近的 limit 条（按 seq 升序），以及更早的是否还有。
	ListMessagesBefore(ctx context.Context, convID string, beforeSeq, minSeq int64, limit int) (msgs []Message, hasMore bool, err error)

	// CreateGroup 建群。群已存在返回 ErrExists。初始成员的 JoinSeq 为会话当前的 maxSeq（新群为 0）。
	CreateGroup(ctx context.Context, g Group, members []string) error
	// AddGroupMembers 加成员，返回本次新加入的成员（已经是成员的不变，不重置 JoinSeq，C-36）。
	// 新成员的 JoinSeq 为会话当前的 maxSeq，必须和 seq 分配互斥地读取。
	// 群不存在返回 ErrNotFound；加完后总人数超过 maxTotal 返回 ErrLimitExceeded，一人都不加。
	// 总人数在同一个事务里检查，并发加人也不会突破上限。
	AddGroupMembers(ctx context.Context, groupID string, members []string, maxTotal int) (added []string, err error)
	// RemoveGroupMembers 移除成员，返回实际被移除的（不是成员的忽略，C-36）。群不存在返回 ErrNotFound。
	RemoveGroupMembers(ctx context.Context, groupID string, members []string) (removed []string, err error)
	// GetGroup 不存在时返回 ErrNotFound。
	GetGroup(ctx context.Context, groupID string) (Group, error)
	// UpdateGroup 修改群设置。不存在时返回 ErrNotFound。
	UpdateGroup(ctx context.Context, groupID string, u GroupUpdate) error
	GroupExists(ctx context.Context, groupID string) (bool, error)
	// GroupMembers 返回全部成员 ID（按入群时间）。
	GroupMembers(ctx context.Context, groupID string) ([]string, error)
	// GroupMember 返回某个成员的信息；不是成员时 ok=false。
	GroupMember(ctx context.Context, groupID, userID string) (m GroupMember, ok bool, err error)

	// UpsertUsers 批量导入用户：已存在的更新昵称和头像。整批在一个事务里。
	UpsertUsers(ctx context.Context, users []User) error
	// EnsureUser 用户不存在时注册（签发凭证时调用）。
	EnsureUser(ctx context.Context, userID string) error
	// GetUser 不存在时返回 ErrNotFound。
	GetUser(ctx context.Context, userID string) (User, error)
	UserExists(ctx context.Context, userID string) (bool, error)
	// MissingUsers 返回 userIDs 中不存在的用户（保持输入顺序）。
	MissingUsers(ctx context.Context, userIDs []string) ([]string, error)
	// RevokeTokens 把用户的令牌版本号加 1，此前签发的 token 全部失效（强制下线）。
	// 用版本号而不是签发时间：同一毫秒内的"签发"和"强制下线"无法靠时间区分。
	RevokeTokens(ctx context.Context, userID string) error
	// TokenVersion 返回用户当前的令牌版本号；用户不存在时为 0。
	TokenVersion(ctx context.Context, userID string) (int64, error)

	// SetReadSeq 记录已读位置，只前进不后退。
	SetReadSeq(ctx context.Context, userID, convID string, seq int64) error
	ReadSeq(ctx context.Context, userID, convID string) (int64, error)

	Close() error
}
