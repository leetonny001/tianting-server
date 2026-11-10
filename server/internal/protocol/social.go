package protocol

// 社交关系命令与数据结构（PRD-social 9.1、9.2 节，M3）。字段与 frame.schema.json 一一对应。

// 社交命令名。
const (
	CmdFriendApply        = "friendApply"
	CmdFriendRespond      = "friendRespond"
	CmdFriendDelete       = "friendDelete"
	CmdFriendRemark       = "friendRemark"
	CmdFriendRequestsSeen = "friendRequestsSeen"
	CmdSocialSync         = "socialSync"
	CmdBlock              = "block"
	CmdUnblock            = "unblock"
	CmdUserSearch         = "userSearch"
	CmdSetPrivacy         = "setPrivacy"
	CmdUsers              = "users"
	CmdMyGroups           = "myGroups"
)

// 群命令（M4）。
const (
	CmdGroupCreate   = "groupCreate"
	CmdGroupInvite   = "groupInvite"
	CmdGroupLeave    = "groupLeave"
	CmdGroupKick     = "groupKick"
	CmdGroupTransfer = "groupTransfer"
	CmdGroupSetAdmin = "groupSetAdmin"
	CmdGroupDissolve = "groupDissolve"
	CmdGroupUpdate   = "groupUpdate"
	CmdGroupInfo     = "groupInfo"
	CmdGroupMembers  = "groupMembers"
)

// 群相关固定上限（5.1 节）。
const (
	MaxGroupOpBatch     = 50
	MaxNoticeLen        = 1000
	MaxGroupNameLen     = 50
	DefaultMembersLimit = 50
	MaxMembersLimit     = 100
)

// SocialCmds 是全部社交命令（指标标签白名单用）。
var SocialCmds = []string{
	CmdFriendApply, CmdFriendRespond, CmdFriendDelete, CmdFriendRemark, CmdFriendRequestsSeen, CmdSocialSync,
	CmdBlock, CmdUnblock, CmdUserSearch, CmdSetPrivacy, CmdUsers, CmdMyGroups,
}

// 社交事件。
const (
	EventSocialChanged         = "socialChanged"
	EventSocialSettingsChanged = "socialSettingsChanged"
)

// 申请来源（F-SOC-02 规则 1）。
const (
	SourceSearch = "search"
	SourceGroup  = "group"
	SourceCard   = "card"
	SourceConv   = "conv"
	SourceOther  = "other"
	SourceAdmin  = "admin" // 只出现在好友项里（管理接口导入）
)

// ValidApplySource 判断客户端可以使用的申请来源。
func ValidApplySource(s string) bool {
	switch s {
	case SourceSearch, SourceGroup, SourceCard, SourceConv, SourceOther:
		return true
	}
	return false
}

// 搜索结果里的关系（与 SDK relationOf 同一枚举，按优先级取第一个满足的）。
const (
	RelSelf       = "self"
	RelBlocked    = "blocked"
	RelFriend     = "friend"
	RelPendingIn  = "pendingIn"
	RelPendingOut = "pendingOut"
	RelNone       = "none"
)

// 申请状态（expired 由服务端按 expireAt 计算）。
const StatusExpired = "expired"

// 固定上限（5.1 节）。
const (
	MaxApplyMessageLen = 50
	MaxRemarkLen       = 32
	MaxUsersBatch      = 100
	DefaultSocialLimit = 500
	MaxSocialLimit     = 500
	DefaultGroupsLimit = 200
	MaxGroupsLimit     = 200
)

type FriendApplyReq struct {
	To        string  `json:"to"`
	Message   *string `json:"message"`
	Source    *string `json:"source"`
	SourceRef *string `json:"sourceRef"`
	Remark    *string `json:"remark"`
}

type FriendRespondReq struct {
	From   string  `json:"from"`
	Action string  `json:"action"`
	Remark *string `json:"remark"`
}

// UserIDReq 用于 friendDelete、block、unblock。
type UserIDReq struct {
	UserID string `json:"userId"`
}

type FriendRemarkReq struct {
	UserID string  `json:"userId"`
	Remark *string `json:"remark"`
}

type RequestsSeenReq struct {
	Ver *int64 `json:"ver"`
}

type SocialSyncReq struct {
	SinceVer *int64  `json:"sinceVer"`
	Cursor   *string `json:"cursor"`
	Limit    *int    `json:"limit"`
}

type UserSearchReq struct {
	Keyword string `json:"keyword"`
}

type SetPrivacyReq struct {
	Searchable *bool `json:"searchable"`
}

type UsersReq struct {
	UserIDs []string `json:"userIds"`
}

type MyGroupsReq struct {
	Cursor *string `json:"cursor"`
	Limit  *int    `json:"limit"`
}

// Profile 是资料（应答时的最新值）。
type Profile struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Official bool   `json:"official"`
}

type FriendItem struct {
	UserID    string   `json:"userId"`
	Remark    string   `json:"remark"`
	Source    string   `json:"source"`
	CreatedAt int64    `json:"createdAt"`
	Ver       int64    `json:"ver"`
	Deleted   bool     `json:"deleted,omitempty"`
	Profile   *Profile `json:"profile"`
}

type RequestItem struct {
	UserID     string   `json:"userId"`
	Direction  string   `json:"direction"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Source     string   `json:"source"`
	ApplyCount int      `json:"applyCount"`
	CreatedAt  int64    `json:"createdAt"`
	UpdatedAt  int64    `json:"updatedAt"`
	ExpireAt   int64    `json:"expireAt"`
	Ver        int64    `json:"ver"`
	Profile    *Profile `json:"profile"`
}

type BlockItem struct {
	UserID    string   `json:"userId"`
	CreatedAt int64    `json:"createdAt"`
	Ver       int64    `json:"ver"`
	Deleted   bool     `json:"deleted,omitempty"`
	Profile   *Profile `json:"profile"`
}

// SocialItems 是 {friends, requests, blocks}；三个数组总是存在（可以为空）。
type SocialItems struct {
	Friends  []FriendItem  `json:"friends"`
	Requests []RequestItem `json:"requests"`
	Blocks   []BlockItem   `json:"blocks"`
}

// SocialChangeAck 是修改类命令的 ack（F-SOC-06 规则 7）；friendApply 另带 status。
type SocialChangeAck struct {
	Status  string      `json:"status,omitempty"`
	Ver     int64       `json:"ver"`
	Changed bool        `json:"changed"`
	Items   SocialItems `json:"items"`
}

// VerAck 是 setPrivacy 的 ack。
type VerAck struct {
	Ver     int64 `json:"ver"`
	Changed bool  `json:"changed"`
}

type RequestsSeenAck struct {
	RequestSeenVer int64 `json:"requestSeenVer"`
	Ver            int64 `json:"ver"`
	Changed        bool  `json:"changed"`
}

type PrivacySettings struct {
	Searchable bool `json:"searchable"`
}

// SocialSyncAck 是 socialSync 一页的应答（F-SOC-06 规则 4）。
type SocialSyncAck struct {
	Full             bool            `json:"full"`
	UntilVer         int64           `json:"untilVer"`
	LatestVer        int64           `json:"latestVer"`
	HasMore          bool            `json:"hasMore"`
	Cursor           string          `json:"cursor"`
	Restart          bool            `json:"restart,omitempty"`
	Friends          []FriendItem    `json:"friends"`
	Requests         []RequestItem   `json:"requests"`
	Blocks           []BlockItem     `json:"blocks"`
	RequestSeenVer   int64           `json:"requestSeenVer"`
	Settings         PrivacySettings `json:"settings"`
	Mode             string          `json:"mode"`
	StrangerMsgLimit int             `json:"strangerMsgLimit"`
	Official         bool            `json:"official"`
}

// SearchUser 是 userSearch 的结果。
type SearchUser struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Official bool   `json:"official"`
	Relation string `json:"relation"`
}

type UserSearchAck struct {
	User *SearchUser `json:"user"`
}

type UserProfile struct {
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Official bool   `json:"official"`
}

type UsersAck struct {
	Users []UserProfile `json:"users"`
}

type MyGroupItem struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount"`
	JoinedAt    int64  `json:"joinedAt"`
	MyRole      string `json:"myRole"`
}

// MyGroupsAck：nextCursor 为空字符串表示没有更多。
type MyGroupsAck struct {
	Groups     []MyGroupItem `json:"groups"`
	NextCursor string        `json:"nextCursor"`
}

type SocialChangedEvent struct {
	Type string `json:"type"`
	Ver  int64  `json:"ver"`
}

type SocialSettingsChangedEvent struct {
	Type             string `json:"type"`
	Mode             string `json:"mode"`
	StrangerMsgLimit int    `json:"strangerMsgLimit"`
}

type GroupCreateReq struct {
	ClientReqID string   `json:"clientReqId"`
	Name        *string  `json:"name"`
	Members     []string `json:"members"`
}

type GroupCreateAck struct {
	GroupID string       `json:"groupId"`
	Conv    Conversation `json:"conv"`
}

type GroupMembersReq struct {
	GroupID string   `json:"groupId"`
	Members []string `json:"members"`
}

type InviteFailed struct {
	UserID string `json:"userId"`
	Code   int    `json:"code"`
}

type GroupInviteAck struct {
	Added  []string       `json:"added"`
	Failed []InviteFailed `json:"failed"`
}

type GroupKickAck struct {
	Removed []string `json:"removed"`
}

type GroupIDReq struct {
	GroupID string `json:"groupId"`
}

type GroupTransferReq struct {
	GroupID string `json:"groupId"`
	To      string `json:"to"`
}

type GroupSetAdminReq struct {
	GroupID string `json:"groupId"`
	UserID  string `json:"userId"`
	Admin   *bool  `json:"admin"`
}

type GroupUpdateReq struct {
	GroupID string  `json:"groupId"`
	Name    *string `json:"name"`
	Notice  *string `json:"notice"`
}

// GroupInfoAck 是 groupInfo 的应答（F-SOC-20 规则 3）。owner 为空字符串表示没有群主；managed=true 表示托管群。
type GroupInfoAck struct {
	GroupID        string `json:"groupId"`
	Name           string `json:"name"`
	Notice         string `json:"notice"`
	NoticeBy       string `json:"noticeBy"`
	NoticeAt       int64  `json:"noticeAt"`
	Owner          string `json:"owner"`
	Managed        bool   `json:"managed"`
	MyRole         string `json:"myRole"`
	MemberCount    int    `json:"memberCount"`
	MaxMembers     int    `json:"maxMembers"`
	HistoryVisible bool   `json:"historyVisible"`
	CreatedAt      int64  `json:"createdAt"`
}

type GroupMembersPageReq struct {
	GroupID string  `json:"groupId"`
	Cursor  *string `json:"cursor"`
	Limit   *int    `json:"limit"`
	// Keyword 是成员搜索（C4，PRD-chat F-MEN-01 规则 6）：userId 或昵称前缀；为空按没有处理。PRD 第 11.1 节写的是 GroupMembersReq（backend-chat 第 51 条）。
	Keyword *string `json:"keyword"`
}

type GroupMemberItem struct {
	UserID   string `json:"userId"`
	Role     string `json:"role"`
	JoinedAt int64  `json:"joinedAt"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Official bool   `json:"official"`
}

type GroupMembersAck struct {
	Members    []GroupMemberItem `json:"members"`
	NextCursor string            `json:"nextCursor"`
}

type GroupChangedEvent struct {
	Type    string `json:"type"`
	GroupID string `json:"groupId"`
}
