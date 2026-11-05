package store

import (
	"context"
	"errors"
)

// 第二批管理操作（PRD-admin F-ADM-08、13、31、32、40、41）的存储层错误。
var (
	ErrMsgWrongConv     = errors.New("message is not in this conversation") // 4000
	ErrMsgNotDeletable  = errors.New("message cannot be deleted")           // 4000：群提示消息
	ErrMsgGrantQuota    = errors.New("message read grant quota exceeded")   // 4292
	ErrMsgGrantNotFound = errors.New("message read grant not found")        // 4039
)

// Ban 是一条封禁记录。Until 为 nil 表示永久。
type Ban struct {
	UserID string
	Until  *int64
	Reason string
	By     string
	At     int64
}

// Active 报告封禁在 now（毫秒）时是否还有效。
func (b *Ban) Active(now int64) bool { return b != nil && (b.Until == nil || *b.Until > now) }

// 禁言范围（和协议一致）。
const (
	MuteUser  = "user"
	MuteGroup = "group"
)

// Mute 是一条禁言记录。Scope 为 user 时 GroupID 为空；Until 为 nil 表示永久。
type Mute struct {
	Scope   string
	UserID  string
	GroupID string
	Until   *int64
	Reason  string
	By      string
	At      int64
}

// Active 报告禁言在 now（毫秒）时是否还有效。
func (m *Mute) Active(now int64) bool { return m != nil && (m.Until == nil || *m.Until > now) }

// SealFunc 加密被删消息的原文（BE-224）。在事务里调用，只做计算，不做 IO。
type SealFunc func(msgID, convID string, body []byte) (nonce, ciphertext []byte, kid string, err error)

// DeleteMessagesReq 是删除消息的参数。
type DeleteMessagesReq struct {
	ConvID    string
	MsgIDs    []string
	By        string
	Now       int64
	ExpiresAt int64 // 隔离原文的到期时间
	Seal      SealFunc
}

// DeletedMessage 是这次真正被删除的一条消息（已经删除过的不算）。
type DeletedMessage struct {
	MsgID string
	Seq   int64
}

// QuarantinedMessage 是隔离区里的一条原文（密文）和它的元数据。
type QuarantinedMessage struct {
	Message    Message // Body 为空：正文被清空了
	Nonce      []byte
	Ciphertext []byte
	KID        string
	ExpiresAt  int64
}

// MsgGrant 是一次正文查看授权。
type MsgGrant struct {
	GrantID    string
	Caller     string
	Operator   string
	ConvID     string
	Quarantine bool
	Reason     string
	Ticket     string
	CreatedAt  int64
	ExpiresAt  int64
}

// MsgGrantQuota 是创建授权时的配额：DayStart 是当天零点（毫秒）。
type MsgGrantQuota struct {
	DayStart    int64
	PerOperator int
	PerCaller   int
}

// ModerationStore 是第二批管理操作用到的存储。
type ModerationStore interface {
	// BanUser 写入或覆盖封禁，同一事务里令牌版本号加 1（此前签发的 token 作废）。用户不存在返回 ErrNotFound。
	BanUser(ctx context.Context, b Ban) error
	// UnbanUser 解除封禁，返回是否真有记录。用户不存在返回 ErrNotFound。
	UnbanUser(ctx context.Context, userID string) (removed bool, err error)
	// UserBan 返回封禁记录（可能已过期，由调用方判断）；没有时为 nil。
	UserBan(ctx context.Context, userID string) (*Ban, error)
	// IssueState 在一条查询里读出令牌版本号和封禁记录：两者必须一起读，
	// 否则"查封禁"和"读版本号"之间发生封禁时，会签发出一个版本号是新的、又绕过封禁检查的 token。
	IssueState(ctx context.Context, userID string) (version int64, ban *Ban, err error)
	// ActiveBan 返回用户在 now 时是否被封禁及到期时间（握手时区分 banned / suspended）。
	ActiveBan(ctx context.Context, userID string, now int64) (banned bool, until *int64, err error)

	// DeleteMessages 在一个事务里软删除、写隔离表、更新会话摘要。任何一条校验失败，整批都不删。
	// 错误：ErrNotFound（msgId 不存在）、ErrMsgWrongConv、ErrMsgNotDeletable。已经删除的跳过，不算错误。
	DeleteMessages(ctx context.Context, req DeleteMessagesReq) ([]DeletedMessage, error)
	// ListQuarantined 返回 seq < beforeSeq、在 now 时还没到期的隔离原文（按 seq 升序，最近的 limit 条）。
	ListQuarantined(ctx context.Context, convID string, beforeSeq, now int64, limit int) (msgs []QuarantinedMessage, hasMore bool, err error)

	// SetMute 写入或覆盖禁言。错误：用户级用户不存在、群内群不存在 ErrNotFound；群内对象不是成员 ErrTargetNotMember。
	SetMute(ctx context.Context, m Mute) error
	// DeleteMute 解除禁言，返回被删的记录（没有时为 nil）。用户级用户不存在、群内群不存在返回 ErrNotFound。
	DeleteMute(ctx context.Context, scope, userID, groupID string) (*Mute, error)
	// UserMutes 返回用户的全部禁言记录（用户级和群内，可能已过期）。
	UserMutes(ctx context.Context, userID string) ([]Mute, error)
	// GroupMutes 返回群里的全部禁言记录（可能已过期），key 是 userId。
	GroupMutes(ctx context.Context, groupID string) (map[string]Mute, error)

	// CreateMsgGrant 在一个事务里检查配额并创建授权；超出返回 ErrMsgGrantQuota。
	CreateMsgGrant(ctx context.Context, g MsgGrant, q MsgGrantQuota) error
	// GetMsgGrant 不存在时返回 ErrMsgGrantNotFound。
	GetMsgGrant(ctx context.Context, grantID string) (MsgGrant, error)
}
