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
	DeletedAt   int64  // 软删除的时间（第二批）；0 = 没有删除。删除后 Body 是 {}
	DeletedBy   string // {caller}:{operator}，只给管理接口，不下发给客户端
	RecalledAt  int64  // 撤回的时间（C4，F-RCL-01 规则 5）；0 = 没有撤回。撤回后 Body 是 {}，和 DeletedAt 分开
	RecalledBy  string // 撤回的操作者 userId
	QuoteMsgID  string // 引用的消息（C4，F-QUO-01 规则 3）：只存 msgId，不把内容复制进 Body

	// 以下字段只在 SaveMessage 时传入，不随消息读出。
	Summary   string   // 会话列表里的摘要（调用方截断好，前 100 个码点）
	ConvUsers []string // 单聊：双方 userId，让会话出现在双方的会话列表里；群消息为空
	// StrangerLimit > 0 时做陌生人限额（PRD-social F-SOC-08 规则 4、8.3 节 L6）：在同一事务里、分配 seq 之前，
	// 对方在这个会话里从未发过消息且我已发 >= StrangerLimit 条时返回 ErrStrangerMsgLimit（不占 seq）。
	// 是否适用（开放模式、不互为好友、双方都不是官方账号）由调用方判断。
	StrangerLimit int
	// AttachFileID 非空时在同一事务里绑定文件（PRD-chat 第 4.3 节第 10 步）：本人、ready 的文件改成 attached，
	// 影响 0 行返回 ErrFileNotReady，消息不保存。加锁顺序 conv_seq → 文件行。AttachRetainUntil 是保留期到期时间，0 表示不过期。
	AttachFileID      string
	AttachRetainUntil int64
	// Mentions 是要写进 conv_mentions 的 @ 个人（C4，F-MEN-03 规则 1，调用方已过滤成当前成员；发送者自己不写，规则 6）。
	// MentionAll 非 nil 时在同一事务里扣 @ 所有人次数（第 4.3 节第 10 步，加锁顺序 conv_seq → 文件行 → 计数行）并写一行 group_mention_all。
	// 次数用完返回 *MentionAllLimitError，计数不可用 ErrMentionAllCounter，提及表写失败 ErrMentionWrite，消息都不保存。
	Mentions   []string
	MentionAll *MentionAllQuota
	// Fault 是测试模式的故障注入（FaultMentionWrite、FaultMentionAllCounter），返回错误时按对应的失败处理。生产为 nil。
	Fault func(point string) error
}

// Group 是群（PRD F-GRP-01）。
type Group struct {
	GroupID        string
	Name           string
	HistoryVisible bool // 新成员能否看到入群前的历史（D2），查询时生效
	CreatedAt      int64
	MemberCount    int // 成员数，加 / 减成员时在同一个事务里维护（只读，CreateGroup 忽略传入值）
	// 以下 M4（PRD-social F-SOC-20）。OwnerID 为空 = 无群主；CreatedBy 为空 = 托管群（业务方建的）。只读，用 SetGroupOwner 等修改。
	OwnerID, CreatedBy string
	Notice, NoticeBy   string
	NoticeAt           int64
	MaxMembers         int
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
	Notice         *string // 管理接口改公告（不产生提示消息，F-SOC-30 规则 3）
	Now            int64   // 改公告时记录 notice_at
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
	MyRole         string // 群里我的角色（M4）；单聊为空
	// UpdatedAt 是排序依据：最后一条**该用户可见**的消息的时间；没有可见消息的群用入群时间。
	UpdatedAt int64
	// LastVisible 为 false 时没有可展示的最后一条消息（群里没有消息，或者只有入群前、且历史不可见的消息）。
	LastVisible bool
	Last        LastMessage
	ModVer      int64 // 会话内已有消息展示内容的版本（PRD-chat F-RCL-03 规则 2，C2：文件过期）
	// 会话设置（C5，conv_settings 的这一行；没有行时都是 0）。
	PinnedAt   int64
	NotifyOff  bool
	DeletedAt  int64
	ClearedSeq int64
}

// LastMessage 是会话最后一条消息的摘要。
type LastMessage struct {
	MsgID      string
	From       string
	Type       string
	Text       string
	TS         int64
	Deleted    bool   // 被删除（第二批）：Text 为空
	Recalled   bool   // 被撤回（C4，F-RCL-03 规则 4）：Text 为空
	RecalledBy string // 撤回的操作者
}

// User 是 IM 用户（PRD F-USER-01）。
type User struct {
	UserID    string
	Nickname  string
	Avatar    string
	CreatedAt int64
	Official  bool // 官方账号（PRD-social F-SOC-13）；UpsertUsers 忽略，用 SetOfficial 修改
}

// Store 是 IM 服务端用到的全部持久化操作。
type Store interface {
	AccountStore
	AdminQueryStore
	SocialStore
	GroupStore
	ModerationStore
	PushStore
	FileStore
	MsgActionStore
	ConvSettingsStore
	WordStore
	ReportStore

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
	// 被移除的人里有群主时按 F-SOC-30 规则 4 处理：自建群自动转让，托管群回到无群主，tip 是这条 ownerChanged；其他情况 tip 为 nil。
	RemoveGroupMembers(ctx context.Context, groupID string, members []string, now int64) (removed []string, tip *Message, err error)
	// DissolveGroup 解散群（PRD-social F-SOC-26 规则 2～4）：一个事务里删除全部成员行、写 dissolved_at、member_count=0，
	// 消息保留；返回被移出的成员。群不存在或已解散返回 ErrNotFound。客户端解散、最后一人退群（M4）共用这个方法。
	// 解散之后：GroupExists / GetGroup / 加减成员 / 改设置都按"不存在"处理，CreateGroup 同一个 ID 返回 ErrExists（不复用）。
	DissolveGroup(ctx context.Context, groupID string, now int64) (members []string, err error)
	// GetGroup 不存在（或已解散）时返回 ErrNotFound。
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
	RevokeTokens(ctx context.Context, userID string, now int64) error
	// TokenVersion 返回用户当前的令牌版本号；用户不存在时为 0。
	TokenVersion(ctx context.Context, userID string) (int64, error)

	// SetReadSeq 记录已读位置：超过会话 maxSeq 的按 maxSeq 记，只前进不后退（PRD F-CONV-02 规则 4）。
	// 返回记录后的已读位置，以及这次是否前进了（用于推送 readUpdated）。
	SetReadSeq(ctx context.Context, userID, convID string, seq int64) (readSeq int64, advanced bool, err error)
	ReadSeq(ctx context.Context, userID, convID string) (int64, error)

	// ListConversations 返回用户的会话列表：置顶的在最前（PinnedAt 倒序，C5），其余按 UpdatedAt 倒序、ConvID 升序（字节序），最多 limit 个；
	// 超过时 truncated=true（C-03）。包括发过或收到过消息的单聊、所在的全部群；不含删除后还没有新消息的会话（C5，F-CVS-03 规则 2）。
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
