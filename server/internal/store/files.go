package store

import (
	"context"
	"errors"
)

// 文件状态（PRD-chat F-FILE-06 规则 2）。
const (
	FilePending  = "pending"
	FileReady    = "ready"
	FileAttached = "attached"
	FileExpired  = "expired" // 保留期到了：对象删除，行保留（消息显示"已过期"），文件名清空
	FileRevoked  = "revoked" // 后台删除：立即停止发放地址，文件名清空，对象保留到隔离期结束
	FilePurging  = "purging" // 清理和绑定之间的中间态：删公开对象和 final/{fileId}/ 前缀之后删行
)

// 文件种类。
const (
	KindImage  = "image"
	KindFile   = "file"
	KindAvatar = "avatar"
)

// 文件相关的存储错误。
var (
	ErrUploadPendingLimit = errors.New("too many pending uploads")
	ErrUploadDeclared     = errors.New("daily declared upload bytes exceeded")
	ErrUploadQuota        = errors.New("daily upload quota exceeded")
	ErrFileNotReady       = errors.New("file not ready")
	ErrStateChanged       = errors.New("file state changed")
)

// File 是 files 表的一行（第 12.1 节）。
type File struct {
	FileID, Owner, Kind, State, Name string
	DeclaredSize                     int64
	DeclaredMime                     string
	ThumbDeclaredSize                int64
	ThumbDeclaredMime                string
	TmpKey, TmpThumbKey              string
	ObjectKey, ThumbKey              string
	Size                             int64
	Mime                             string
	ThumbSize                        int64
	ThumbMime                        string
	Width, Height                    int
	CompletingUntil                  int64 // 0 表示没有租约
	CompletingAttempt                string
	PublicKey                        string
	AvatarReplacedAt                 int64
	MsgID, ConvID                    string
	RetainUntil, EvidenceUntil       int64
	ScanState                        string
	PutExpiresAt                     int64
	CreatedAt, ReadyAt, AttachedAt   int64
	RevokedAt                        int64
	ObjectsDeleted                   bool
}

// UploadLimits 是申请上传时在同一个事务里检查的限制（F-FILE-01 规则 6、6a）。
type UploadLimits struct {
	Day           string // Asia/Shanghai 日期
	DailyBytes    int64  // 当日额度（complete 成功按实际大小扣的部分）
	DailyDeclared int64  // 当日声明字节数合计上限
	PendingMax    int    // PUT 地址没过期的 pending 上限
	NowReal       int64  // 真实时钟，和 put_expires_at 比较
}

// CompleteResult 是一次 complete 尝试读到的结果，提交时一次写入（规则 3 ④）。
type CompleteResult struct {
	Attempt             string
	ObjectKey, ThumbKey string
	Size                int64
	Mime                string
	ThumbSize           int64
	ThumbMime           string
	Width, Height       int
	Day                 string
	DailyBytes          int64
	Now                 int64
}

// FileAccess 是取地址时判断权限用的文件和消息（F-FILE-04 规则 2）。
type FileAccess struct {
	File
	MsgSeq     int64
	MsgDeleted bool
	MsgFound   bool
}

// FileStore 是文件相关的持久化操作。后台删除消息时的 revoked 在 DeleteMessages 的事务里（F-FILE-06 规则 4）。
type FileStore interface {
	// CreateUpload 在一个事务里检查额度、声明字节数、未完成数（顺序：额度 4292 → 声明 4292 → 未完成 4290），通过则插入 pending 行并累计声明。
	CreateUpload(ctx context.Context, f File, lim UploadLimits) error
	// GetFile 不存在时返回 ErrNotFound。
	GetFile(ctx context.Context, fileID string) (File, error)
	// GetFiles 批量读，不存在的不在结果里。
	GetFiles(ctx context.Context, fileIDs []string) (map[string]File, error)
	// FileAccessInfo 批量读文件和绑定的消息（seq、是否删除）。
	FileAccessInfo(ctx context.Context, fileIDs []string) (map[string]FileAccess, error)
	// AcquireCompleteLease 取 complete 的租约：本人、pending、没有租约或租约已过期。ok=false 时返回当前行（可能已不是 pending）。
	AcquireCompleteLease(ctx context.Context, fileID, owner, attempt string, until, now int64) (f File, ok bool, err error)
	// ReleaseCompleteLease 按 attempt 条件释放租约（每次尝试结束时，包括失败，RS5-07）。
	ReleaseCompleteLease(ctx context.Context, fileID, attempt string) error
	// CommitComplete 提交：同一事务复查额度（已用量已达额度 → ErrUploadQuota），WHERE state='pending' 写入结果、状态 ready、按实际大小扣额度；
	// 已不是 pending → ErrStateChanged。
	CommitComplete(ctx context.Context, fileID string, r CompleteResult) error
	// UsedUploadBytes 返回当日已扣的上传量。
	UsedUploadBytes(ctx context.Context, userID, day string) (int64, error)
	// CancelUpload 是 pending → purging 的条件更新（只匹配本人，不看租约，RS5-16）。
	CancelUpload(ctx context.Context, fileID, owner string) (bool, error)
	// MarkStalePurging 把 created_at 早于 before 的 pending（租约过期或没有租约）和 ready 改成 purging（F-FILE-06 规则 2）。头像绑定后是 attached，不受影响。
	MarkStalePurging(ctx context.Context, before, now int64, limit int) (int, error)
	// MarkReplacedAvatarsPurging 把换头像早于 before 的头像改成 purging。
	MarkReplacedAvatarsPurging(ctx context.Context, before int64, limit int) (int, error)
	// ListPurging 返回 purging 的文件（最多 limit 个）。
	ListPurging(ctx context.Context, limit int) ([]File, error)
	// DeleteFileRow 删除一行，只删 purging 的。
	DeleteFileRow(ctx context.Context, fileID string) error
	// ClearPublicKey 公开对象删除成功后清空 public_key。
	ClearPublicKey(ctx context.Context, fileID string) error
	// ExpireFiles 把 retain_until 已过的 attached 改成 expired，清空文件名，递增所在会话的 mod_ver（F-FILE-06 规则 3）。返回改了的行。
	ExpireFiles(ctx context.Context, now int64, limit int) ([]File, error)
	// ListObjectsToDelete 返回对象还没删的 expired 文件，和隔离期已过的 revoked 文件。
	ListObjectsToDelete(ctx context.Context, now int64, limit int) ([]File, error)
	// MarkObjectsDeleted 记录对象已删除。
	MarkObjectsDeleted(ctx context.Context, fileID string) error
	// SetAvatarPublicKey 写 public_key（只在为空时写，重试复用同一个键，RS6-04），返回生效的键和文件。
	SetAvatarPublicKey(ctx context.Context, fileID, owner, key string) (File, error)
	// BindAvatar 在一个事务里：文件改成 attached（retain_until 为空、avatar_replaced_at 清零），users.avatar = url，
	// 原来绑定的另一个头像文件写 avatar_replaced_at = now（如果还没写）。
	BindAvatar(ctx context.Context, fileID, owner, url string, now int64) error
}
