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

	// 以下字段只在 SaveMessage 时传入，不随消息读出。
	Summary   string   // 会话列表里的摘要（调用方截断好，前 100 个码点）
	ConvUsers []string // 单聊：双方 userId，让会话出现在双方的会话列表里；群消息为空
}

// Group 是群（PRD F-GRP-01）。
type Group struct {
	GroupID        string
	Name           string
	HistoryVisible bool // 新成员能否看到入群前的历史（D2），查询时生效
	CreatedAt      int64
	MemberCount    int // 成员数，加 / 减成员时在同一个事务里维护（只读，CreateGroup 忽略传入值）
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

// ConvRow 是会话列表里的一项（原始数据，由 logic 计算 minSeq、未读数等）。
type ConvRow struct {
	UserID         string // 这是谁的会话
	ConvID         string
	MaxSeq         int64
	ReadSeq        int64
	JoinSeq        int64  // 群：入群时的 maxSeq；单聊为 0
	GroupName      string // 群名；单聊为空
	HistoryVisible bool   // 群设置；单聊为 false
	PeerID         string // 单聊对方；群为空
	PeerNickname   string // 单聊对方的昵称，可能为空
	MemberCount    int    // 群成员数；单聊为 0
	// UpdatedAt 是排序依据：最后一条**该用户可见**的消息的时间；没有可见消息的群用入群时间。
	UpdatedAt int64
	// LastVisible 为 false 时没有可展示的最后一条消息（群里没有消息，或者只有入群前、且历史不可见的消息）。
	LastVisible bool
	Last        LastMessage
}

// LastMessage 是会话最后一条消息的摘要。
type LastMessage struct {
	MsgID string
	From  string
	Type  string
	Text  string
	TS    int64
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
	// 同一个事务里还会：更新会话的最后一条消息；单聊把会话加入双方的会话列表（ConvUsers）；
	// 把发送者的已读位置推进到这条消息（发言即已读，PRD F-CONV-02 规则 2）。
	SaveMessage(ctx context.Context, m Message) (saved Message, dup bool, err error)
	// FindByClientMsgID 按（发送者，clientMsgId）查找已保存的消息，用于在限流前先识别重发。
	FindByClientMsgID(ctx context.Context, from, clientMsgID string) (Message, bool, error)
	// ListMessages 返回 seq > afterSeq 的消息（升序，最多 limit 条），以及后面是否还有。
	ListMessages(ctx context.Context, convID string, afterSeq int64, limit int) (msgs []Message, hasMore bool, err error)
	// ListMessagesBefore 返回 minSeq <= seq < beforeSeq 中最近的 limit 条（按 seq 升序），以及更早的是否还有。
	ListMessagesBefore(ctx context.Context, convID string, beforeSeq, minSeq int64, limit int) (msgs []Message, hasMore bool, err error)

	// CreateGroup 建群。群已存在返回 ErrExists。初始成员的 JoinSeq 为会话当前的 maxSeq（新群为 0），
	// 入群时间为 g.CreatedAt（由调用方按注入的时钟传入）。
	CreateGroup(ctx context.Context, g Group, members []string) error
	// AddGroupMembers 加成员，返回本次新加入的成员（已经是成员的不变，不重置 JoinSeq，C-36）。
	// 新成员的 JoinSeq 为会话当前的 maxSeq，必须和 seq 分配互斥地读取。
	// 群不存在返回 ErrNotFound；加完后总人数超过 maxTotal 返回 ErrLimitExceeded，一人都不加。
	// 总人数在同一个事务里检查，并发加人也不会突破上限。
	// joinedAt 是入群时间（没有消息的群在会话列表里按它排序）。
	AddGroupMembers(ctx context.Context, groupID string, members []string, maxTotal int, joinedAt int64) (added []string, err error)
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
	// SetNickname 只修改昵称（演示登录带昵称时用，BE-091）；用户不存在返回 ErrNotFound。
	SetNickname(ctx context.Context, userID, nickname string) error
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

	// SetReadSeq 记录已读位置：超过会话 maxSeq 的按 maxSeq 记，只前进不后退（PRD F-CONV-02 规则 4）。
	// 返回记录后的已读位置，以及这次是否前进了（用于推送 readUpdated）。
	SetReadSeq(ctx context.Context, userID, convID string, seq int64) (readSeq int64, advanced bool, err error)
	ReadSeq(ctx context.Context, userID, convID string) (int64, error)

	// ListConversations 返回用户的会话列表：按 UpdatedAt 倒序、ConvID 升序（字节序），最多 limit 个；
	// 超过时 truncated=true（C-03）。包括发过或收到过消息的单聊、所在的全部群。
	ListConversations(ctx context.Context, userID string, limit int) (convs []ConvRow, truncated bool, err error)
	// GetConversation 返回用户的某个会话；不在其会话列表里时 ok=false。
	GetConversation(ctx context.Context, userID, convID string) (c ConvRow, ok bool, err error)
	// GroupConversations 一次查出群里这些成员各自的会话（用于给新成员批量下发 convAdded）；
	// 不是成员的不出现在结果里。
	GroupConversations(ctx context.Context, groupID string, userIDs []string) (map[string]ConvRow, error)

	// AppendAdminAudit 追加一条管理接口审计记录（PRD-admin F-KEY-04）。审计只有追加和查询：
	// 接口里故意没有更新 / 删除方法，契约测试断言这一点。
	AppendAdminAudit(ctx context.Context, e AdminAudit) error
	// QueryAdminAudit 按时间范围 [FromMs, ToMs] 和可选条件查询，按 id 倒序；Cursor 为上一页最后一条的 id（0 表示从最新开始）。
	// 返回的 next 为下一页的游标，没有更多时为 0。
	QueryAdminAudit(ctx context.Context, q AdminAuditQuery) (entries []AdminAudit, next int64, err error)

	Close() error
}

// AdminAudit 是一条管理接口审计记录。TargetsJSON / SummaryJSON 由调用方构造（白名单字段），存储层原样保存。
type AdminAudit struct {
	ID          int64
	TS          int64
	Caller      string
	Operator    string
	RequestID   string
	Route       string
	TargetsJSON string
	SummaryJSON string
	Phase       string // AuditIntent | AuditResult
	Code        int
}

// 审计记录的阶段。
const (
	AuditIntent = "intent"
	AuditResult = "result"
)

// AdminAuditQuery 是审计查询条件；空字符串表示不按该字段过滤。
type AdminAuditQuery struct {
	FromMs, ToMs                int64
	Caller, Operator, RequestID string
	Cursor                      int64
	Limit                       int
}
