package store

import (
	"context"
	"errors"
)

// 客户端群管理（PRD-social 第 7 节，M4，BE-130～BE-136）的类型、权限表和 Store 方法。

// 群管理的存储层错误（由 logic / admin 映射为协议错误码）。
var (
	ErrNotGroupMember    = errors.New("not a group member")        // 4033
	ErrGroupForbidden    = errors.New("group operation forbidden") // 4030
	ErrTargetNotMember   = errors.New("target not a group member") // 4046
	ErrAdminLimit        = errors.New("group admin limit")         // 4136
	ErrInviteeNotAllowed = errors.New("invitee not allowed")       // 4035：不存在、不是好友、拉黑了我（同码）
	ErrJoinedGroupLimit  = errors.New("joined group limit")        // 4135
	ErrCreateQuota       = errors.New("group create quota")        // 4292
)

// 群角色。
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// MaxGroupAdmins 是管理员上限（5.1 节固定值）。
const MaxGroupAdmins = 10

// GroupOp 是需要权限的群操作（F-SOC-20 规则 2 的矩阵）。发言、看资料、看成员、退群所有成员都可以，不在表里。
type GroupOp int

const (
	OpInvite GroupOp = iota
	OpUpdate         // 改群名、群公告
	OpKick
	OpManage // 设置 / 取消管理员、转让群主
	OpDissolve
)

// GroupCan 判定权限。不缓存角色：每次在持有群锁时读取（L5）。
func GroupCan(op GroupOp, role string, g Group) bool {
	if g.Managed() && g.OwnerID == "" {
		return false // 托管群（无群主）：任何成员都只能发言、查看、退群
	}
	switch op {
	case OpInvite:
		return true
	case OpUpdate, OpKick:
		return role == RoleOwner || role == RoleAdmin
	case OpManage:
		return role == RoleOwner
	case OpDissolve:
		return role == RoleOwner && !g.Managed() // 托管群的群主也不能解散
	}
	return false
}

// Managed 报告是否为托管群（业务方建的）。
func (g Group) Managed() bool { return g.CreatedBy == "" }

// CreateClientGroupParams 是 groupCreate 的参数（格式已校验）。
type CreateClientGroupParams struct {
	UserID, ClientReqID, GroupID string
	Name                         string // 空串：用前 3 人昵称生成（F-SOC-21 规则 6）
	Members                      []string
	MaxMembers, MaxJoined        int
	Now                          int64
	QuotaDenied                  bool // 今天的建群配额已用完：判重没命中时返回 ErrCreateQuota
}

// InviteFailure 是邀请失败的一项。
type InviteFailure struct {
	UserID string
	Err    error // ErrInviteeNotAllowed 或 ErrJoinedGroupLimit
}

// LeaveResult 是退群（或管理接口移除群主）的结果。
type LeaveResult struct {
	Dissolved bool     // 最后一人退出自建群：同一事务解散
	Tip       *Message // 群主退群时的 ownerChanged
}

// GroupMemberRow 是 groupMembers 的一项。
type GroupMemberRow struct {
	UserID, Role     string
	JoinedAt         int64
	RoleAt           int64
	Nickname, Avatar string
	Official         bool
}

// MemberKey 是成员列表的排序键：群主 0 → 管理员 1（role_at、userId）→ 成员 2（入群时间、userId）。
type MemberKey struct {
	Rank int
	T    int64
	User string
}

// GroupStore 是群管理方法（PRD-social 8.6 节 M4）。返回的 *Message 是同一事务里生成的提示消息，由调用方投递。
type GroupStore interface {
	// CreateClientGroup 同一用户同一 clientReqId 只建一次：dup=true 时返回第一次的 groupID，不再校验。
	// 被邀请人不满足条件时返回 ErrInviteeNotAllowed / ErrJoinedGroupLimit 和 failedUser（我自己的群数已满 failedUser=我）。
	CreateClientGroup(ctx context.Context, p CreateClientGroupParams) (groupID string, dup bool, tip *Message, failedUser string, err error)
	InviteMembers(ctx context.Context, groupID, actor string, members []string, maxJoined int, now int64) (added []string, failed []InviteFailure, tip *Message, err error)
	KickMembers(ctx context.Context, groupID, actor string, members []string) (removed []string, err error)
	LeaveGroup(ctx context.Context, groupID, actor string, now int64) (LeaveResult, error)
	TransferOwner(ctx context.Context, groupID, actor, to string, now int64) (*Message, error)
	// SetGroupRole 设置 / 取消管理员；已经是目标状态时 tip 为 nil。
	SetGroupRole(ctx context.Context, groupID, actor, target string, admin bool, now int64) (*Message, error)
	// ClientDissolveGroup 校验权限后解散，返回被移出的成员。
	ClientDissolveGroup(ctx context.Context, groupID, actor string, now int64) ([]string, error)
	// UpdateGroupProfile 改群名 / 公告，每改一项一条提示消息（renamed、noticeUpdated）。
	UpdateGroupProfile(ctx context.Context, groupID, actor string, name, notice *string, now int64) ([]Message, error)
	ListGroupMembers(ctx context.Context, groupID string, after *MemberKey, limit int) (rows []GroupMemberRow, next *MemberKey, err error)
	// SetGroupOwner 是管理接口指定 / 更换 / 收回群主（owner 为空串表示收回）。不是成员返回 ErrTargetNotMember；没有变化时 tip 为 nil。
	SetGroupOwner(ctx context.Context, groupID, owner string, now int64) (*Message, error)
	// PurgeGroupReqs 删除 created_at < before 的建群去重记录（保留 7 天）。
	PurgeGroupReqs(ctx context.Context, before int64) error
}
