package files

import (
	"cmp"
	"time"
)

// Config 是 C2 的文件配置（PRD-chat 第 4.1 节，都未校准）。0 值表示默认值。
type Config struct {
	Enabled            bool          // IM_FILES_ENABLED
	ImageMaxBytes      int64         // IM_IMAGE_MAX_BYTES，20 MiB
	ThumbMaxBytes      int64         // IM_THUMB_MAX_BYTES，512 KiB
	FileMaxBytes       int64         // IM_FILE_MAX_BYTES，100 MiB
	AvatarMaxBytes     int64         // IM_AVATAR_MAX_BYTES，2 MiB
	DailyBytes         int64         // IM_UPLOAD_DAILY_BYTES，2 GiB
	NewAccountDaily    int64         // open 模式下注册 7 天内的账号，200 MiB
	UploadRate         int           // IM_UPLOAD_RATE，每分钟 60 次
	PendingMax         int           // IM_UPLOAD_PENDING_MAX，20 个
	PutTTL             time.Duration // IM_UPLOAD_PUT_TTL，15 分钟
	CompleteLease      time.Duration // IM_UPLOAD_COMPLETE_LEASE，60 秒
	DailyDeclared      int64         // IM_UPLOAD_DAILY_DECLARED，每日额度的 4 倍
	URLTTLImage        time.Duration // IM_FILE_URL_TTL_IMAGE，120 秒
	URLTTLFile         time.Duration // IM_FILE_URL_TTL_FILE，300 秒
	RetentionDays      *int          // IM_FILE_RETENTION_DAYS，180；0 表示不过期
	OrphanHours        int           // IM_UPLOAD_ORPHAN_HOURS，24
	URLRate            int           // 每人每分钟 120 批（F-FILE-04 规则 6）
	PurgeBatch         int           // file_purge 每批 1000 个
	AvatarReplacedDays int           // 换头像 7 天后删除旧的（F-ACC-06 规则 5）
	RevokeDays         int           // 后台删除后对象保留 30 天（F-FILE-06 规则 4，和隔离期相同）
	BlocklistPath      string        // IM_FILE_BLOCKLIST
	// RegisterOpen：IM_REGISTER_MODE=open 时注册 7 天内的账号额度是 NewAccountDaily（第 4.1 节）。
	RegisterOpen bool
}

const (
	MiB = 1 << 20
	GiB = 1 << 30
)

// 不可配置的常量（都未校准）。
const (
	sniffBytes     = 64 << 10 // 嗅探读取前 64 KB（F-FILE-01 规则 3 ②）
	maxSide        = 10000    // 真实宽高上限（F-FILE-02 规则 1）
	maxPixels      = 40_000_000
	thumbMaxSide   = 512 // 缩略图长边（规则 3b）
	avatarMaxSide  = 1024
	gifMaxFrames   = 300 // GIF 在前 64 KB 内能看到的帧数上限（规则 3b，未校准）
	maxURLItems    = 50
	newAccountDays = 7
)

// WithDefaults 填好默认值。
func (c Config) WithDefaults() Config {
	c.ImageMaxBytes = cmp.Or(c.ImageMaxBytes, 20*MiB)
	c.ThumbMaxBytes = cmp.Or(c.ThumbMaxBytes, 512<<10)
	c.FileMaxBytes = cmp.Or(c.FileMaxBytes, 100*MiB)
	c.AvatarMaxBytes = cmp.Or(c.AvatarMaxBytes, 2*MiB)
	c.DailyBytes = cmp.Or(c.DailyBytes, 2*GiB)
	c.NewAccountDaily = cmp.Or(c.NewAccountDaily, 200*MiB)
	c.UploadRate = cmp.Or(c.UploadRate, 60)
	c.PendingMax = cmp.Or(c.PendingMax, 20)
	c.PutTTL = cmp.Or(c.PutTTL, 15*time.Minute)
	c.CompleteLease = cmp.Or(c.CompleteLease, 60*time.Second)
	c.DailyDeclared = cmp.Or(c.DailyDeclared, 4*c.DailyBytes)
	c.URLTTLImage = cmp.Or(c.URLTTLImage, 120*time.Second)
	c.URLTTLFile = cmp.Or(c.URLTTLFile, 300*time.Second)
	if c.RetentionDays == nil {
		d := 180
		c.RetentionDays = &d
	}
	c.OrphanHours = cmp.Or(c.OrphanHours, 24)
	c.URLRate = cmp.Or(c.URLRate, 120)
	c.PurgeBatch = cmp.Or(c.PurgeBatch, 1000)
	c.AvatarReplacedDays = cmp.Or(c.AvatarReplacedDays, 7)
	c.RevokeDays = cmp.Or(c.RevokeDays, 30)
	return c
}

// PublicLimits 是 GET /v1/config 的文件限制（features.files=true 时出现）。
type PublicLimits struct {
	ImageMaxBytes    int64 `json:"imageMaxBytes"`
	ThumbMaxBytes    int64 `json:"thumbMaxBytes"`
	FileMaxBytes     int64 `json:"fileMaxBytes"`
	AvatarMaxBytes   int64 `json:"avatarMaxBytes"`
	UploadDailyBytes int64 `json:"uploadDailyBytes"`
}

// Limits 返回客户端可见的限制。
func (c Config) Limits() PublicLimits {
	c = c.WithDefaults()
	return PublicLimits{ImageMaxBytes: c.ImageMaxBytes, ThumbMaxBytes: c.ThumbMaxBytes, FileMaxBytes: c.FileMaxBytes, AvatarMaxBytes: c.AvatarMaxBytes, UploadDailyBytes: c.DailyBytes}
}
