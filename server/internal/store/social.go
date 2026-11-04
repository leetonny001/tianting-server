package store

import (
	"context"
	"errors"
)

// 社交关系（PRD-social M3）的存储层错误。由 logic / admin 映射为协议错误码。
var (
	ErrIBlockedPeer     = errors.New("peer is blocked by me")           // 4000：申请自己拉黑的人
	ErrFriendLimit      = errors.New("friend limit exceeded")           // 4132
	ErrPeerFriendLimit  = errors.New("peer friend limit exceeded")      // 4133
	ErrBlockLimit       = errors.New("block limit exceeded")            // 4134
	ErrRequestNotFound  = errors.New("friend request not found")        // 4045：不存在、已过期
	ErrRequestHandled   = errors.New("friend request accepted")         // 4091：已同意的再拒绝
	ErrNotFriend        = errors.New("not a friend")                    // 4040：给非好友设备注
	ErrStrangerMsgLimit = errors.New("stranger message limit")          // 4137：SaveMessage 内判定
	ErrUserMissing      = errors.New("user does not exist")             // 管理接口导入：4042
	ErrImportFriendCap  = errors.New("friend limit exceeded on import") // 管理接口导入：4132
)

// 申请行的方向与状态（`expired` 不存储，读取时按 ExpireAt 计算）。
const (
	DirIn  = "in"
	DirOut = "out"

	ReqPending  = "pending"
	ReqAccepted = "accepted"
	ReqRejected = "rejected"
)

// FriendRow 是一条有向好友边（owner → peer）。
type FriendRow struct {
	Owner, Peer, Remark, Source string
	CreatedAt, UpdatedAt        int64
	Deleted                     bool
	Ver                         int64
}

// RequestRow 是一条申请行（owner 一侧）。
type RequestRow struct {
	Owner, Peer, Direction, Status string
	Message, Source, SourceRef     string
	PresetRemark                   string
	ApplyCount                     int
	CreatedAt, UpdatedAt, ExpireAt int64
	Ver                            int64
}

// BlockRow 是一条黑名单行。
type BlockRow struct {
	Owner, Peer          string
	CreatedAt, UpdatedAt int64
	Deleted              bool
	Ver                  int64
}

// SocialItems 是某个用户的一组同步项（同步页、修改类 ack 的 items）。
type SocialItems struct {
	Friends  []FriendRow
	Requests []RequestRow
	Blocks   []BlockRow
}

// SocialChange 是一次修改类操作的结果（PRD-social F-SOC-06 规则 7、5.4 节）。
type SocialChange struct {
	Ver     int64       // Changed 时为本次给我分配的新 ver，否则为当前 ver
	Changed bool        // 我的同步数据是否变化
	Items   SocialItems // 我本次改动的全部行（都带 Ver）
	// Bumped 是 ver 变化了的全部用户（含我）→ 新 ver，用于下发 socialChanged（5.4 节规则 2）。
	Bumped map[string]int64
}

// ApplyFriendParams 是 friendApply 的参数（格式已校验）。
type ApplyFriendParams struct {
	From, To                   string
	Message, Source, SourceRef string
	Remark                     string // 预设备注
	Now, ExpireAt              int64
	MaxFriends                 int
}

// RespondFriendParams 是 friendRespond 的参数。
type RespondFriendParams struct {
	Me, From   string
	Accept     bool
	Remark     string
	Now        int64
	MaxFriends int
}

// SocialState 是用户的同步头信息。
type SocialState struct {
	Ver, PurgedVer, RequestSeenVer int64
	Searchable                     *bool // nil = 跟随默认
	FriendCount, BlockCount        int
	Official                       bool
}

// SortKey 是同步项的排序键 (ver, 类型, 对方 userId, 方向)，类型顺序 好友 0 < 申请 1 < 黑名单 2。
type SortKey struct {
	Ver  int64
	Type int
	Peer string
	Dir  string
}

// 同步项类型（排序键第二位）。
const (
	ItemFriend  = 0
	ItemRequest = 1
	ItemBlock   = 2
)

// SocialPageQuery 是 socialSync 的一页。First=true 时由存储层判定增量 / 全量并读出 untilVer；
// 否则 Full / SinceVer / UntilVer / After 来自游标。
type SocialPageQuery struct {
	UID      string
	First    bool
	SinceVer int64
	Full     bool
	UntilVer int64
	After    *SortKey
	Limit    int
	// RequestCutoff：全量轮只返回 updated_at >= RequestCutoff 的申请（30 天，查询时过滤，与清理任务无关）。
	RequestCutoff int64
}

// SocialPage 是同步的一页。
type SocialPage struct {
	Restart  bool // 增量轮中途清理过（purgedVer > sinceVer），客户端用同一个 sinceVer 重新开始
	Full     bool
	SinceVer int64
	UntilVer int64
	HasMore  bool
	Last     *SortKey // 本页最后一项的排序键（HasMore 时用于游标）
	Items    SocialItems
	State    SocialState // 读这一页时的状态（LatestVer = State.Ver）
}

// Relation 是发送路径一次读出的关系（PRD-social F-SOC-08，8.3 节 L7：普通读、不加锁）。
type Relation struct {
	PeerExists         bool
	MeToPeer, PeerToMe bool // 好友边（非墓碑）
	PeerBlocksMe       bool
	MeOfficial         bool
	PeerOfficial       bool
	HasMessages        bool // 单聊会话里已有消息（不是新对象）
	MySent, PeerSent   int  // 双方在这个会话里的 sent_count
}

// FriendPair 是管理接口导入 / 删除的一对。
type FriendPair struct {
	A, B             string
	RemarkA, RemarkB *string
}

// AdminFriend 是管理接口好友查询的一项（不含备注）。
type AdminFriend struct {
	UserID    string
	Source    string
	CreatedAt int64
}

// MyGroup 是 myGroups 的一项。
type MyGroup struct {
	GroupID     string
	Name        string
	MemberCount int
	JoinedAt    int64
	MyRole      string
}

// SocialSeed 是测试接口 /admin/test/social/seed 的数据（PRD-social 5.6 节）：不经限流和配额，
// 但按正常规则分配 ver、维护计数器。
type SocialSeed struct {
	Friends      []SeedEdge
	Blocks       []SeedEdge
	Requests     []SeedRequest
	GroupMembers []SeedMember
	UpdatedAt    int64 // 行的时间（造过期数据）；调用方必须传
}

type SeedEdge struct{ Owner, Peer string }

type SeedRequest struct {
	From, To, Status, Message, Source string
	ExpireAt                          int64
}

type SeedMember struct{ GroupID, UserID string }

// SocialStore 是社交关系的存储方法（PRD-social 8.6 节 M3）。每个修改都在一个事务里：
// 先按 8.3 节顺序锁用户，再判定、写行；每个有变化的用户只分配一个 ver。
type SocialStore interface {
	// SocialSettingsJSON 返回配置行（key=social）；没有时 ok=false。
	SocialSettingsJSON(ctx context.Context) (value string, ok bool, err error)
	// InitSocialSettings 只在没有配置行时写入（IM_SOCIAL_DEFAULTS），返回是否写入。
	InitSocialSettings(ctx context.Context, value string, now int64) (bool, error)
	// UpdateSocialSettings 在一个事务里读改写：fn 收到旧值（没有时为空字符串），返回新值或错误（错误时不写）。
	UpdateSocialSettings(ctx context.Context, now int64, fn func(old string) (string, error)) error

	ApplyFriend(ctx context.Context, p ApplyFriendParams) (status string, c SocialChange, err error)
	RespondFriend(ctx context.Context, p RespondFriendParams) (SocialChange, error)
	DeleteFriend(ctx context.Context, me, peer string, now int64) (SocialChange, error)
	SetRemark(ctx context.Context, me, peer, remark string, now int64) (SocialChange, error)
	Block(ctx context.Context, me, peer string, now int64, maxBlocks int) (SocialChange, error)
	Unblock(ctx context.Context, me, peer string, now int64) (SocialChange, error)
	// SetRequestSeen 把 requestSeenVer 设为 max(原值, min(ver, 当前 ver))，返回新的 requestSeenVer。
	SetRequestSeen(ctx context.Context, me string, ver int64, now int64) (seen int64, c SocialChange, err error)
	// SetSearchable 有效值变化时写入并分配 ver。
	SetSearchable(ctx context.Context, me string, searchable, def bool, now int64) (SocialChange, error)
	SocialState(ctx context.Context, uid string) (SocialState, error)
	SocialPage(ctx context.Context, q SocialPageQuery) (SocialPage, error)
	// PurgeSocial 物理删除 updated_at < before 的墓碑和申请行，推进 purgedVer；不分配 ver。返回删除行数。
	PurgeSocial(ctx context.Context, before int64) (int, error)

	// Relation 供 send 使用：一次读出 me 与 peer 的关系。convID 是二者的单聊会话。
	Relation(ctx context.Context, me, peer, convID string) (Relation, error)
	// SearchUser 精确查找用户与我和他的关系，用于 userSearch；同一条查询路径（防时序区分）。
	SearchUser(ctx context.Context, me, target string, now int64) (u User, exists bool, searchable *bool, rel SearchRelation, err error)
	// VisibleUsers 返回 ids 中 uid 可见的用户资料（F-SOC-10 规则 2）；不可见、不存在的不返回。
	VisibleUsers(ctx context.Context, uid string, ids []string, requestCutoff int64) ([]User, error)
	// Profiles 返回这些用户的资料（不存在的不返回）。
	Profiles(ctx context.Context, ids []string) (map[string]User, error)
	MyGroups(ctx context.Context, uid, after string, limit int) (groups []MyGroup, hasMore bool, err error)

	ImportFriends(ctx context.Context, pairs []FriendPair, now int64, maxFriends int) (changed map[string]int64, failedUser string, err error)
	RemoveFriends(ctx context.Context, pairs []FriendPair, now int64) (changed map[string]int64, err error)
	ListFriends(ctx context.Context, uid, after string, limit int) (items []AdminFriend, hasMore bool, err error)
	// SetOfficial 用户不存在返回 ErrNotFound。
	SetOfficial(ctx context.Context, uid string, official bool) error

	SeedSocial(ctx context.Context, s SocialSeed) (map[string]int64, error)
	// RepairCounters 按基础表重算 friend_count / block_count / member_count / sent_count，返回修复条数（幂等）。
	RepairCounters(ctx context.Context) (int, error)
	// CounterDrift 每种计数器随机抽 sample 行，返回与基础表不一致的行数（测试模式启动时校验，PRD-social 8.5 节）。
	CounterDrift(ctx context.Context, sample int) (int, error)
}

// SearchRelation 是搜索时我与对方的关系（userSearch 的 relation 由 logic 按优先级算出）。
type SearchRelation struct {
	IBlocked, IFriend, PendingIn, PendingOut bool
}
