// Package account 是账号功能（PRD-chat 第 5 节）：注册、登录、失败限制、Argon2 闸门、设备会话和 refresh token。
// HTTP 在 api 包，存储在 store 包。不依赖 hub（ARCH 2.2）：投递只走 delivery.Router。
package account

import (
	"cmp"
	"net/netip"
	"time"
)

// 注册模式（PRD-chat 第 4.1 节，F-ACC-01 规则 1）。
const (
	ModeClosed = "closed"
	ModeInvite = "invite"
	ModeOpen   = "open"
)

// 用户名、密码、昵称、设备名的长度（F-ACC-01 规则 1、3、5、6），都按码点计数。`GET /v1/config` 的 limits 也用它们。
const (
	UsernameMinLen   = 4
	UsernameMaxLen   = 32
	PasswordMinLen   = 8
	PasswordMaxLen   = 128
	NicknameMaxLen   = 32
	DeviceNameMaxLen = 64
)

// RefreshKey 是一把 refresh 密钥（F-ACC-03 规则 3）：独立于 JWT 密钥。退役台账见 ledger.go。
type RefreshKey struct {
	ID     string
	Secret []byte
}

// Config 是账号功能的配置（第 4.1 节），都可以用环境变量配置，都未校准。0 值取默认值（WithDefaults）。
type Config struct {
	RegisterMode string
	// RefreshKeys 为空时账号接口返回 503 / 5030，其他功能不受影响（老部署升级后不会因此启动失败）。
	RefreshKeys     []RefreshKey
	RefreshActive   string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	RefreshTTLWeb   time.Duration
	RefreshAbsolute time.Duration
	// EnforceKeyLedger：生产环境台账有问题时拒绝启动（F-ACC-03 规则 3，TC-ACC-03-48）。
	EnforceKeyLedger bool

	// RefreshOverlap 是转正后的软拒绝窗口，默认 2 分钟（必须大于 SDK 超时 + 网关超时，配置校验）。
	RefreshOverlap    time.Duration
	RefreshMaxPending int // 待确认代上限，默认 4
	RefreshIssueRate  int // 每条会话每小时签发次数，默认 20
	RefreshIPRate     int // token/refresh 每 IP 每分钟，默认 60
	RecoveryOldDays   int // 旧恢复码保留天数，默认 7
	InviteWarnTotal   int // 有效邀请码剩余次数合计的告警线，默认 500（app 里取默认值）

	// TrustedEgress、TrustedEgressFactor 是 IM_TRUSTED_EGRESS_CIDRS、IM_TRUSTED_EGRESS_FACTOR（第 12.2 节，RS4-15）。
	TrustedEgress       []netip.Prefix
	TrustedEgressFactor int

	ArgonSlots   int           // 普通闸门，默认 4
	RecoverSlots int           // recover 专用位，默认 1
	GateQueue    time.Duration // 排队时限，默认 1 秒
	RecoverQueue time.Duration // recover 排队时限，默认 5 秒
	Argon        ArgonParams   // 0 值是 m=64 MiB、t=3、p=1，只有测试才改

	PrivacyVersion string // IM_PRIVACY_NOTICE_VERSION，写入 accounts.notice_version
	PrivacyURL     string // IM_PRIVACY_NOTICE_URL
	ExtraReserved  []string

	// TestDisableLock 对应 IM_TEST_DISABLE_AUTH_LOCK：关闭账号锁定、IP 限流和渐进延迟，只在测试模式生效（第 4.5 节）。
	TestDisableLock bool
}

// 第 4.1 节的默认值。
const (
	defaultAccessTTL       = 24 * time.Hour
	defaultRefreshTTL      = 90 * 24 * time.Hour
	defaultRefreshTTLWeb   = 24 * time.Hour
	defaultRefreshAbsolute = 365 * 24 * time.Hour
	defaultArgonSlots      = 4
	defaultRecoverSlots    = 1
	defaultGateQueue       = time.Second
	defaultRecoverQueue    = 5 * time.Second
	defaultRefreshOverlap  = 2 * time.Minute
	defaultMaxPending      = 4
	defaultIssueRate       = 20
	defaultRefreshIPRate   = 60
	defaultRecoveryOldDays = 7
)

// WithDefaults 填上默认值。
func (c Config) WithDefaults() Config {
	c.RegisterMode = cmp.Or(c.RegisterMode, ModeInvite)
	c.AccessTTL = cmp.Or(c.AccessTTL, defaultAccessTTL)
	c.RefreshTTL = cmp.Or(c.RefreshTTL, defaultRefreshTTL)
	c.RefreshTTLWeb = cmp.Or(c.RefreshTTLWeb, defaultRefreshTTLWeb)
	c.RefreshAbsolute = cmp.Or(c.RefreshAbsolute, defaultRefreshAbsolute)
	c.RefreshOverlap = cmp.Or(c.RefreshOverlap, defaultRefreshOverlap)
	c.RefreshMaxPending = cmp.Or(c.RefreshMaxPending, defaultMaxPending)
	c.RefreshIssueRate = cmp.Or(c.RefreshIssueRate, defaultIssueRate)
	c.RefreshIPRate = cmp.Or(c.RefreshIPRate, defaultRefreshIPRate)
	c.RecoveryOldDays = cmp.Or(c.RecoveryOldDays, defaultRecoveryOldDays)
	c.ArgonSlots = cmp.Or(c.ArgonSlots, defaultArgonSlots)
	c.RecoverSlots = cmp.Or(c.RecoverSlots, defaultRecoverSlots)
	c.GateQueue = cmp.Or(c.GateQueue, defaultGateQueue)
	c.RecoverQueue = cmp.Or(c.RecoverQueue, defaultRecoverQueue)
	if c.Argon == (ArgonParams{}) {
		c.Argon = DefaultArgon
	}
	return c
}

// ArgonParams 是 Argon2id 参数。
type ArgonParams struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

// DefaultArgon 和 console-server 相同（F-ACC-01 规则 5）：m=64 MiB、t=3、p=1。
var DefaultArgon = ArgonParams{MemoryKiB: 64 * 1024, Time: 3, Threads: 1}

// ConfigResponse 是 GET /v1/config（F-ACC-07 规则 4，F-ACC-10 规则 1）。不含任何密钥。
type ConfigResponse struct {
	RegisterMode  string         `json:"registerMode"`
	DemoLogin     bool           `json:"demoLogin"`
	Features      Features       `json:"features"`
	Limits        Limits         `json:"limits"`
	PrivacyNotice *PrivacyNotice `json:"privacyNotice,omitempty"`
}

// Features 告诉客户端隐藏哪些入口。files 跟 IM_FILES_ENABLED，msgActions 跟 IM_MSG_ACTIONS_ENABLED（C4），
// convSettings 跟 IM_CONV_SETTINGS_ENABLED（C5，backend-chat 第 75 条），moderation 跟 IM_MOD_ENABLED（C6，第 96 条，
// 只控制举报入口；开关随 BE-492 接上，之前一直是 false）。
type Features struct {
	Files        bool `json:"files"`
	MsgActions   bool `json:"msgActions"`
	ConvSettings bool `json:"convSettings"`
	Moderation   bool `json:"moderation"`
}

// Limits 是客户端校验用的长度（PRD 没有定义内容，见任务单的进度说明）。
type Limits struct {
	UsernameMinLen   int `json:"usernameMinLen"`
	UsernameMaxLen   int `json:"usernameMaxLen"`
	PasswordMinLen   int `json:"passwordMinLen"`
	PasswordMaxLen   int `json:"passwordMaxLen"`
	NicknameMaxLen   int `json:"nicknameMaxLen"`
	DeviceNameMaxLen int `json:"deviceNameMaxLen"`
	// 文件的限制（PRD-chat 第 4.1 节）：只在 features.files=true 时出现。
	ImageMaxBytes    int64 `json:"imageMaxBytes,omitempty"`
	ThumbMaxBytes    int64 `json:"thumbMaxBytes,omitempty"`
	FileMaxBytes     int64 `json:"fileMaxBytes,omitempty"`
	AvatarMaxBytes   int64 `json:"avatarMaxBytes,omitempty"`
	UploadDailyBytes int64 `json:"uploadDailyBytes,omitempty"`
}

// PrivacyNotice 是隐私说明（F-ACC-10 规则 1）。配置为空时不出现，客户端隐藏入口。
type PrivacyNotice struct {
	Version string `json:"version"`
	URL     string `json:"url,omitempty"`
}

// PublicConfig 生成 GET /v1/config 的响应。
func (c Config) PublicConfig(demoLogin bool) ConfigResponse {
	out := ConfigResponse{
		RegisterMode: c.WithDefaults().RegisterMode,
		DemoLogin:    demoLogin,
		Limits: Limits{
			UsernameMinLen: UsernameMinLen, UsernameMaxLen: UsernameMaxLen, PasswordMinLen: PasswordMinLen,
			PasswordMaxLen: PasswordMaxLen, NicknameMaxLen: NicknameMaxLen, DeviceNameMaxLen: DeviceNameMaxLen,
		},
	}
	if c.PrivacyVersion != "" {
		out.PrivacyNotice = &PrivacyNotice{Version: c.PrivacyVersion, URL: c.PrivacyURL}
	}
	return out
}
