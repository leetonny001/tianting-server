package store

import (
	"context"
	"errors"
)

// 账号存储（PRD-chat 第 12.1 节，BE-410）。
var (
	// ErrInviteInvalid：邀请码不存在、过期、用完或停用，调用方不区分（F-ACC-01 规则 1）。
	ErrInviteInvalid = errors.New("invite invalid")
	// ErrCredentialsChanged：提交事务时 credentials_version 或账号状态已经变了（F-ACC-04 规则 8）。
	ErrCredentialsChanged = errors.New("credentials changed")
	// ErrRecoveryCodeStale：确认的不是最新一代恢复码（F-ACC-05 规则 1）。
	ErrRecoveryCodeStale = errors.New("recovery code is not the latest")
)

// RevokeReasonRevoked 是自己退出、强制下线、封禁、sessions revoke-all 的吊销原因（F-ACC-03 规则 3 的 reason=revoked）。
const RevokeReasonRevoked = "revoked"

// Account 是一行账号。
type Account struct {
	UserID               string
	UsernameLower        string
	PasswordHash         string
	CredentialsVersion   int64
	CredentialsChangedAt int64
	DeleteRequestedAt    int64 // 0 = 没有申请注销
	NoticeVersion        string
	CreatedAt            int64
	LastLoginAt          int64
	DeletedAt            int64 // users.deleted_at；0 = 没有注销
}

// PendingGen 是一个待确认代（F-ACC-03 规则 4）。
type PendingGen struct {
	Gen  int64  `json:"gen"`
	Hash string `json:"hash"`
}

// DeviceSession 是一条设备会话（F-ACC-03 规则 1）。Hash 都是整个 refresh token 的哈希。
type DeviceSession struct {
	SID               string
	UserID            string
	Platform          string
	DeviceName        string
	CurrentHash       string
	CurrentGen        int64
	NextGen           int64
	Pendings          []PendingGen
	OverlapUntil      int64
	OverlapMinGen     int64
	RowVersion        int64
	SeenIPPrefixes    []string
	CreatedAt         int64
	LastSeenAt        int64
	ExpiresAt         int64
	AbsoluteExpiresAt int64
	RevokedAt         int64
	RevokeReason      string
	// 签发频率（规则 4）：当前窗口的开始时间和次数；最近一个触发限流的窗口和连续触发的小时数。
	IssueWindowStart int64
	IssueCount       int
	LimitedWindow    int64
	LimitedStreak    int
}

// RefreshKeyEntry 是 refresh 密钥台账的一行（F-ACC-03 规则 3，RS5-04）。
type RefreshKeyEntry struct {
	KID                           string
	AddedAt, RetiredAt, Forgotten int64
}

// AccountEvent 是一条账号事件（F-ACC-02 规则 3b）。SIDHash 是 sid 的哈希（F-ACC-03 规则 1）。
type AccountEvent struct {
	EventID, UserID, Kind, Platform, IPPrefix, SIDHash string
	At                                                 int64
}

// Invite 是一个邀请码（F-ACC-09）。
type Invite struct {
	InviteID, CodeHash, CodeLast4, Note, CreatedBy string
	MaxUses, UsedCount                             int
	ExpiresAt, CreatedAt, DisabledAt               int64
	AutoDisabled                                   bool
}

// NewAccount 是注册事务的内容（F-ACC-01 规则 8）：同一事务写用户、账号、恢复码、设备会话，扣邀请码。
type NewAccount struct {
	Account      Account
	Nickname     string
	InviteHash   string // 空表示不需要邀请码
	RecoveryHash string
	Session      DeviceSession
	// BeforeCommit 在提交前调用，返回错误时整个事务回滚（测试模式的故障注入，AC-ACC-01-15）。可以为 nil。
	BeforeCommit func() error
}

// LoginCommit 是登录事务的内容（F-ACC-02、F-ACC-04 规则 8）。
type LoginCommit struct {
	UserID             string
	CredentialsVersion int64 // 校验密码时读到的版本，第一条语句按它锁行
	Now                int64
	Session            DeviceSession
	Event              AccountEvent
	// NewRecoveryHash 只在最新一代恢复码没有确认时写入：那一代作废，换新的一代（F-ACC-05 规则 1）。
	NewRecoveryHash string
	// ThrottleKey 非空时，提交成功后这个失败计数一并清零。
	ThrottleKey  string
	BeforeCommit func() error
}

// LoginResult 是登录事务的结果。RecoveryGen > 0 表示写入了新的一代恢复码，响应要带上明文。
type LoginResult struct{ RecoveryGen int64 }

// ThrottleHit 是一次计数之后的状态。
type ThrottleHit struct {
	Count       int
	WindowStart int64
	LockedUntil int64
}

// OtherSessions 是这条会话之外的有效会话（F-ACC-03 规则 4 的"账号安全"）。
type OtherSessions struct {
	Count        int
	LastActiveAt int64
}

// AccountStore 是账号相关的持久化操作。
//
// 所有写事务按第 4.3 节的顺序加锁：accounts → recovery_codes → device_sessions。
type AccountStore interface {
	// GetAccountByUsername 按规范化后的用户名读账号；没有返回 ErrNotFound。
	GetAccountByUsername(ctx context.Context, usernameLower string) (Account, error)
	GetAccount(ctx context.Context, userID string) (Account, error)
	// UsernameTaken：accounts.username_lower 或 lower(users.user_id) 已有（含已注销的，F-ACC-01 规则 4）。
	UsernameTaken(ctx context.Context, usernameLower string) (bool, error)
	// CreateAccount 是注册事务。用户名已占用返回 ErrExists，邀请码无效返回 ErrInviteInvalid，都不留任何写入。
	CreateAccount(ctx context.Context, a NewAccount) error
	// CommitLogin 是登录事务。第一条语句按版本锁 accounts 行，不匹配或账号状态变了返回 ErrCredentialsChanged。
	CommitLogin(ctx context.Context, c LoginCommit) (LoginResult, error)
	// UpdatePasswordHash 只在版本没变时重写哈希（参数升级，F-ACC-02 异常）。不改版本，不算改密。
	UpdatePasswordHash(ctx context.Context, userID string, version int64, hash string) error
	// RecoveryUnacked：最新一代恢复码没有确认。
	RecoveryUnacked(ctx context.Context, userID string) (bool, error)
	// AckRecoveryCode 确认一代恢复码；不是最新一代返回 ErrRecoveryCodeStale（F-ACC-05 规则 1）。
	AckRecoveryCode(ctx context.Context, userID string, gen, now int64) error

	// GetDeviceSession 读会话；没有返回 ErrNotFound。
	GetDeviceSession(ctx context.Context, sid string) (DeviceSession, error)
	// UpdateDeviceSession 是条件更新（RC2-04）：WHERE sid=? AND row_version=? AND revoked_at=0，版本取 d.RowVersion。
	// 影响 0 行返回 false，调用方重新读一次再判断，不会出现两条链。events 同一事务写入。
	UpdateDeviceSession(ctx context.Context, d DeviceSession, events ...AccountEvent) (bool, error)
	// RevokeDeviceSession 吊销一条会话。expectVersion > 0 时带版本条件（重放判定用）；已吊销返回 false。
	RevokeDeviceSession(ctx context.Context, sid, reason string, now, expectVersion int64, events ...AccountEvent) (bool, error)
	// RevokeAllDeviceSessions 吊销全部会话（im-server sessions revoke-all），不计重放。
	RevokeAllDeviceSessions(ctx context.Context, reason string, now int64) (int, error)
	// PurgeDeviceSessions 删除过期时间早于 before 的会话（session_purge）。
	PurgeDeviceSessions(ctx context.Context, before int64) (int, error)
	// RecoverBefore 按账号状态算找回截止日期（F-ACC-03 规则 6a）：未过期、没用过、没作废的已确认旧码里最早的 retire_until；没有返回 0。
	RecoverBefore(ctx context.Context, userID string, now int64) (int64, error)
	// SyncRefreshKeyLedger 把配置写进台账：新 kid 增加，不再是当前密钥的写 retired_at（只写一次）。配置里没有的行不动。
	SyncRefreshKeyLedger(ctx context.Context, configured []string, active string, now int64) error
	RefreshKeyLedger(ctx context.Context) ([]RefreshKeyEntry, error)
	// ForgetRefreshKey 记下运维明确放弃一把密钥（泄露应急，接受持这把密钥的 token 全部失效）。kid 不存在返回 ErrNotFound。
	ForgetRefreshKey(ctx context.Context, kid string, now int64) error
	// OtherDeviceSessions 统计 exceptSID 之外没有吊销、没有过期的会话。
	OtherDeviceSessions(ctx context.Context, userID, exceptSID string, now int64) (OtherSessions, error)

	// HitThrottle 给 key 计一次（固定窗口）。计到 limit 时把 locked_until 设为 now + lockMs（lockMs = 0 不锁）。
	HitThrottle(ctx context.Context, key string, now, windowMs int64, limit int, lockMs int64) (ThrottleHit, error)
	// PeekThrottle 只读不计数；窗口已过的计数视为 0。
	PeekThrottle(ctx context.Context, key string, now, windowMs int64) (ThrottleHit, error)
	ClearThrottle(ctx context.Context, key string) error
	// ResetThrottle 清空全部计数和邀请码的失败计数（/admin/test/limits/reset scope=auth）。
	ResetThrottle(ctx context.Context) error
	PurgeThrottle(ctx context.Context, now int64) (int, error)

	CreateInvite(ctx context.Context, inv Invite) error
	// InviteUsable 判断邀请码现在能不能用。
	InviteUsable(ctx context.Context, codeHash string, now int64) (bool, error)
	GetInviteByHash(ctx context.Context, codeHash string) (Invite, error)
	// RecordInviteMismatch 记一次"有效邀请码 + 已占用的用户名"，窗口内达到 limit 就自动停用，返回这次是否停用了它（F-ACC-01 规则 7）。
	RecordInviteMismatch(ctx context.Context, codeHash string, now, windowMs int64, limit int) (disabled bool, err error)

	ListAccountEvents(ctx context.Context, userID string, limit int) ([]AccountEvent, error)
	PurgeAccountEvents(ctx context.Context, before int64) (int, error)

	// UserDeleted：users.deleted_at 不为 0（F-ACC-08 规则 3）。用户不存在返回 false。
	UserDeleted(ctx context.Context, userID string) (bool, error)
	// MarkUserDeleted 是注销到期的第 ① 步（F-ACC-08 规则 3）：同一事务标记 deleted_at、递增 token_version、删除密码和恢复码哈希。
	MarkUserDeleted(ctx context.Context, userID string, now int64) error
}
