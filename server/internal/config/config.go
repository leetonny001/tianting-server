// Package config 从环境变量读取服务配置，并做安全校验（ARCH S4）。
//
// 原则：开发环境尽量开箱即用（缺密钥时用开发默认值并告警）；
// 生产环境（IM_ENV=production）任何不安全的配置都拒绝启动，并一次性列出全部问题。
package config

import (
	"cmp"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/auth"
	"tianting/server/internal/gateway"
	"tianting/server/internal/opsapi"
	"tianting/server/internal/social"
)

// 运行环境。
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// 开发默认密钥：只允许在非生产环境使用，生产环境检测到这两个值会拒绝启动。
const (
	DevJWTSecret = "dev-secret-change-me-0123456789"
	// DevQuarantineKey 只用于开发环境，生产启动保护拒绝（AC-ADM-32-4）。
	DevQuarantineKey = "dev-quarantine-key-change-me-0123456789"
	DevAdminKey      = "dev-admin-key"
	// DevRefreshKey 只用于开发环境，生产启动保护拒绝（PRD-chat F-ACC-03 规则 3）。
	DevRefreshKey = "dev-refresh-key-change-me-0123456789"
)

// MinProductionSecretLen 是生产环境密钥的最小长度（32 字节 ≈ 256 位）。
const MinProductionSecretLen = 32

// Server 是进程级配置。
type Server struct {
	Env  string
	Addr string
	// MetricsAddr 是 /metrics 的监听地址（BE-051），默认只监听本机；为空表示不开启。
	// 不和 Addr 共用端口：指标是内部运行数据，不能暴露到公网。
	MetricsAddr string
	// AdminAddr 是管理接口 /admin/* 的监听地址（PRD-admin F-KEY-01），默认 127.0.0.1:8081；为空表示不开启。
	// 对外端口上不注册任何管理路由。非回环地址在生产环境必须配置 TLS（AdminTLSCert / AdminTLSKey）。
	AdminAddr string
	// AllowTokenIssue 是 IM_ALLOW_TOKEN_ISSUE=1：生产环境显式放行带 token.issue 的调用方（PRD-chat F-ACC-07 规则 3）。
	AllowTokenIssue bool
	AdminTLSCert    string
	AdminTLSKey     string
	// OpsAddr 是节点运维接口的监听地址（PRD-ops F-OPS-16），默认 127.0.0.1:9091；为空表示不开启。
	// 没有配置 IM_OPS_PUBKEYS 时不监听（生产环境拒绝启动）。非回环地址在生产环境必须有 TLS 和来源网段。
	OpsAddr    string
	OpsTLSCert string
	OpsTLSKey  string
	// PprofAddr 是 pprof 的监听地址（PRD-ops F-OPS-17 规则 3）：开发默认 127.0.0.1:6060，生产默认关闭，设置时必须是回环地址。
	PprofAddr    string
	PlannedConns int    // IM_PLANNED_CONNS：规划连接数，只用于配置检查的内存风险提示
	GOGC         string // 原样显示
	DrainDelay   string // 原样显示
	// TestOverrides 是配置里出现了的测试设施开关的名字（testOverrideEnvs），生产环境拒绝启动。
	TestOverrides []string
	App           app.Config
}

// testOverrideEnvs 是第 4.5 节测试设施的开关（PRD-chat R-CQ-01）：只在测试模式生效，生产配置里出现就拒绝启动。
// IM_APNS_ENDPOINT、IM_FCM_ENDPOINT、IM_TEST_DISABLE_IDLE 的功能随 BE-406，这里先拦住。
var testOverrideEnvs = []string{"IM_TEST_DISABLE_AUTH_LOCK", "IM_APNS_ENDPOINT", "IM_FCM_ENDPOINT", "IM_TEST_DISABLE_IDLE"}

// Load 读取并校验配置。getenv 通常是 os.Getenv，测试时可以传入假实现。
// warnings 是不影响启动、但需要人注意的问题（例如开发环境使用了默认密钥）。
func Load(getenv func(string) string) (cfg Server, warnings []string, err error) {
	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	cfg.Env = cmp.Or(getenv("IM_ENV"), EnvDevelopment)
	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		return Server{}, nil, fmt.Errorf("IM_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.Env)
	}
	prod := cfg.Env == EnvProduction

	cfg.Addr = cmp.Or(getenv("IM_ADDR"), ":8080")
	cfg.MetricsAddr = addrEnv(getenv, "IM_METRICS_ADDR", "127.0.0.1:9090")
	cfg.AdminAddr = addrEnv(getenv, "IM_ADMIN_ADDR", "127.0.0.1:8081")
	cfg.OpsAddr = addrEnv(getenv, "IM_OPS_ADDR", "127.0.0.1:9091")
	pprofDefault := "127.0.0.1:6060"
	if prod {
		pprofDefault = ""
	}
	cfg.PprofAddr = addrEnv(getenv, "IM_PPROF_ADDR", pprofDefault)
	cfg.OpsTLSCert, cfg.OpsTLSKey = getenv("IM_OPS_TLS_CERT"), getenv("IM_OPS_TLS_KEY")
	if (cfg.OpsTLSCert == "") != (cfg.OpsTLSKey == "") {
		bad("IM_OPS_TLS_CERT and IM_OPS_TLS_KEY must be set together")
	}
	cfg.GOGC, cfg.DrainDelay = getenv("GOGC"), getenv("IM_DRAIN_DELAY")
	if v := getenv("IM_PLANNED_CONNS"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 1 {
			bad("IM_PLANNED_CONNS: must be a positive integer, got %q", v)
		}
		cfg.PlannedConns = n
	}
	cfg.AdminTLSCert, cfg.AdminTLSKey = getenv("IM_ADMIN_TLS_CERT"), getenv("IM_ADMIN_TLS_KEY")
	if (cfg.AdminTLSCert == "") != (cfg.AdminTLSKey == "") {
		bad("IM_ADMIN_TLS_CERT and IM_ADMIN_TLS_KEY must be set together")
	}
	problems = append(problems, addrConflicts(map[string]string{
		"IM_ADDR": cfg.Addr, "IM_ADMIN_ADDR": cfg.AdminAddr, "IM_OPS_ADDR": cfg.OpsAddr,
		"IM_METRICS_ADDR": cfg.MetricsAddr, "IM_PPROF_ADDR": cfg.PprofAddr,
	})...)
	a := &cfg.App
	keys, kerr := parseOpsKeys(getenv("IM_OPS_PUBKEYS"))
	if kerr != nil {
		bad("IM_OPS_PUBKEYS: %v", kerr)
	}
	a.OpsKeys = keys
	if v := getenv("IM_OPS_ALLOWED_CIDRS"); v != "" {
		for _, item := range splitList(v) {
			p, perr := parsePrefix(item)
			if perr != nil {
				bad("IM_OPS_ALLOWED_CIDRS: %q is not an IP or CIDR", item)
				continue
			}
			a.OpsAllowedCIDRs = append(a.OpsAllowedCIDRs, p)
		}
	}
	a.DBPath = cmp.Or(getenv("IM_DB"), "im.db")
	a.AutoMigrate = boolEnv(getenv, "IM_AUTO_MIGRATE", !prod, bad) // 生产默认由发布流程执行 migrate up
	a.TestMode = boolEnv(getenv, "IM_TEST_MODE", false, bad)
	for _, name := range testOverrideEnvs {
		if getenv(name) != "" {
			cfg.TestOverrides = append(cfg.TestOverrides, name)
		}
	}
	// 只在测试模式生效（AC-ACC-02-3、05-11）。生产配置里出现时上面已经记下，productionChecks 拒绝启动。
	if boolEnv(getenv, "IM_TEST_DISABLE_AUTH_LOCK", false, bad) {
		if a.TestMode {
			a.Account.TestDisableLock = true
		} else if !prod {
			warnings = append(warnings, "IM_TEST_DISABLE_AUTH_LOCK is ignored without IM_TEST_MODE=1")
		}
	}
	if raw := getenv("IM_TEST_PEERS"); raw != "" { // 双节点测试环境（PRD-social 5.6 节），只在测试模式生效
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				a.TestPeers = append(a.TestPeers, strings.TrimRight(p, "/"))
			}
		}
	}
	switch v := getenv("IM_TEST_INVALIDATION"); v {
	case "", "on":
	case "off":
		a.TestInvalidationOff = true
	default:
		bad("IM_TEST_INVALIDATION must be on or off, got %q", v)
	}
	a.DemoLogin = boolEnv(getenv, "IM_DEMO_LOGIN", false, bad)

	// 签名密钥：IM_JWT_KEYS（轮换用，"k1:secret1,k2:secret2"）优先于 IM_JWT_SECRET。
	if raw := getenv("IM_JWT_KEYS"); raw != "" {
		keys, perr := parseKeys(raw)
		a.JWTKeys = keys
		a.JWTActiveKID = getenv("IM_JWT_ACTIVE_KID")
		if a.JWTActiveKID == "" && len(keys) == 1 {
			a.JWTActiveKID = keys[0].ID
		}
		switch {
		case perr != nil:
			bad("IM_JWT_KEYS: %v", perr)
		case a.JWTActiveKID == "":
			bad("IM_JWT_ACTIVE_KID is required when IM_JWT_KEYS has more than one key")
		}
	} else {
		a.JWTSecret = getenv("IM_JWT_SECRET")
	}
	a.AdminKey = getenv("IM_ADMIN_KEY")
	legacy := boolEnv(getenv, "IM_ADMIN_LEGACY", false, bad)
	// 社交配置的初始值（PRD-social F-SOC-01 规则 2）：只在库里没有 social 配置行时写入一次；格式或范围非法拒绝启动。
	if raw := getenv("IM_SOCIAL_DEFAULTS"); raw != "" {
		if _, err := social.Patch(social.Defaults(), []byte(raw)); err != nil {
			bad("IM_SOCIAL_DEFAULTS: %v", err)
		}
		a.SocialDefaults = raw
	}
	a.V13AdminFeatures = boolEnv(getenv, "IM_V13_ADMIN_FEATURES", false, bad)
	a.Push.Enabled = boolEnv(getenv, "IM_PUSH_ENABLED", false, bad)
	// 系统推送的参数（PRD-live 第 10 节）：不设置时用默认值，都未校准。
	for key, dst := range map[string]*int{
		"IM_PUSH_ALL_RATE": &a.Push.AllRate, "IM_PUSH_USER_RATE": &a.Push.UserRate, "IM_PUSH_SYNC_RATE": &a.Push.SyncRate,
		"IM_PUSH_INBOX_MAX": &a.Push.InboxMax, "IM_PUSH_PURGE_BATCH": &a.Push.PurgeBatch,
	} {
		if v := getenv(key); v != "" {
			n, perr := strconv.Atoi(v)
			if perr != nil || n < 1 {
				bad("%s: must be a positive integer, got %q", key, v)
				continue
			}
			*dst = n
		}
	}
	a.TestDisableJobs = splitList(getenv("IM_TEST_DISABLE_JOBS")) // 只在测试模式生效（PRD-live 8.2 节）
	a.QuarantineKey = getenv("IM_QUARANTINE_KEY")
	a.QuarantineDays = 30
	if v := getenv("IM_QUARANTINE_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			bad("IM_QUARANTINE_DAYS must be 1..365, got %q", v)
		} else {
			a.QuarantineDays = n
		}
	}
	switch v := getenv("IM_C2C_BODY_ACCESS"); v {
	case "", "off":
	case "on":
		a.C2CBodyAccess = true
	default:
		bad("IM_C2C_BODY_ACCESS must be on or off, got %q", v)
	}

	loadAccount(getenv, a, bad)

	a.Account.EnforceKeyLedger = prod
	cfg.AllowTokenIssue = getenv("IM_ALLOW_TOKEN_ISSUE") == "1"

	if !prod {
		if a.JWTSecret == "" && len(a.JWTKeys) == 0 {
			a.JWTSecret = DevJWTSecret
			warnings = append(warnings, "IM_JWT_SECRET not set, using the development default")
		}
		if a.AdminKey == "" {
			a.AdminKey = DevAdminKey
			warnings = append(warnings, "IM_ADMIN_KEY not set, using the development default")
		}
		if a.QuarantineKey == "" {
			a.QuarantineKey = DevQuarantineKey
			warnings = append(warnings, "IM_QUARANTINE_KEY not set, using the development default")
		}
		if len(a.Account.RefreshKeys) == 0 {
			a.Account.RefreshKeys = []account.RefreshKey{{ID: "dev", Secret: []byte(DevRefreshKey)}}
			a.Account.RefreshActive = "dev"
			warnings = append(warnings, "IM_REFRESH_KEYS not set, using the development default")
		}
	}

	// 签名调用方（PRD-admin F-KEY-02）。密钥不能与 JWT 密钥、旧管理密钥相同，所以放在默认密钥确定之后检查。
	if path := getenv("IM_ADMIN_CALLERS"); path != "" {
		others := [][]byte{[]byte(a.AdminKey), []byte(a.JWTSecret)}
		for _, k := range a.JWTKeys {
			others = append(others, k.Secret)
		}
		callers, cerr := adminauth.LoadCallersFile(path, others...)
		if cerr != nil {
			bad("IM_ADMIN_CALLERS: %v", cerr)
		}
		a.AdminCallers = callers
		// 运维凭证和管理接口作用域隔离（PRD-ops S-OPS-12）：console-ops 不能出现在管理接口的调用方里。
		for _, c := range callers {
			if c.ID == opsapi.CallerID {
				bad("IM_ADMIN_CALLERS: %q is reserved for the ops API and cannot call /admin/*", opsapi.CallerID)
			}
		}
	}
	// 旧密钥（PRD-admin F-KEY-05 规则 3）：生产环境只有显式 IM_ADMIN_LEGACY=1 才接受。
	if prod {
		a.AdminLegacyDisabled = !legacy
		a.ReportLegacyAdmin = legacy && a.AdminKey != ""
		if a.ReportLegacyAdmin {
			warnings = append(warnings, "IM_ADMIN_LEGACY=1: the shared IM_ADMIN_KEY is accepted on v1.2 admin APIs; move callers to IM_ADMIN_CALLERS")
		}
	}

	gw := gateway.DefaultConfig()
	if v := getenv("IM_IDLE_TIMEOUT"); v != "" {
		d, perr := time.ParseDuration(v)
		if perr != nil || d <= 0 {
			bad("IM_IDLE_TIMEOUT: invalid duration %q", v)
		}
		gw.IdleTimeout = d
	}
	if v := getenv("IM_MAX_CONNS_PER_USER"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 1 {
			bad("IM_MAX_CONNS_PER_USER: must be a positive integer, got %q", v)
		}
		gw.MaxConnsPerUser = n
	}
	if v := getenv("IM_ALLOWED_ORIGINS"); v != "" {
		gw.AllowedOrigins = splitList(v)
	}
	if v := getenv("IM_TRUSTED_PROXIES"); v != "" {
		for _, item := range splitList(v) {
			p, perr := parsePrefix(item)
			if perr != nil {
				bad("IM_TRUSTED_PROXIES: %q is not an IP or CIDR", item)
				continue
			}
			gw.Guard.TrustedProxies = append(gw.Guard.TrustedProxies, p)
		}
	}
	// 握手限流（ARCH S7）。压测时所有连接来自同一个 IP，需要调高。
	for key, dst := range map[string]*int{"IM_HANDSHAKE_RATE": &gw.Guard.HandshakeRate, "IM_HANDSHAKE_BURST": &gw.Guard.HandshakeBurst} {
		if v := getenv(key); v != "" {
			n, perr := strconv.Atoi(v)
			if perr != nil || n < 1 {
				bad("%s: must be a positive integer, got %q", key, v)
				continue
			}
			*dst = n
		}
	}
	gw.NodeID = getenv("IM_NODE_ID")
	a.Gateway = gw
	if v := getenv("IM_ADMIN_ALLOWED_CIDRS"); v != "" {
		for _, item := range splitList(v) {
			p, perr := parsePrefix(item)
			if perr != nil {
				bad("IM_ADMIN_ALLOWED_CIDRS: %q is not an IP or CIDR", item)
				continue
			}
			a.AdminAllowedCIDRs = append(a.AdminAllowedCIDRs, p)
		}
	}

	if prod {
		a.RequireHTTPS = true
		problems = append(problems, productionProblems(cfg)...)
	} else {
		if a.TestMode {
			warnings = append(warnings, "TEST MODE ENABLED: fault injection, clock injection and 1s tokens are available")
		}
		if containsWildcard(gw.AllowedOrigins) {
			warnings = append(warnings, "IM_ALLOWED_ORIGINS=* disables the WebSocket origin check")
		}
	}
	if len(problems) > 0 {
		return Server{}, warnings, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	if !prod && cfg.OpsAddr != "" && len(a.OpsKeys) == 0 {
		warnings = append(warnings, "IM_OPS_PUBKEYS not set: the node ops API is not listening")
	}
	snapshot := cfg
	a.ConfigSummary = func(version, nodeID string) any { return Summary(snapshot, version, nodeID) }
	return cfg, warnings, nil
}

// refreshKidRE：kid 是 refresh token 的第一段（kid.sid.gen.nonce.mac），不能含分隔符。
var refreshKidRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

const minRefreshOverlap = 60 * time.Second

// loadAccount 读取账号功能的配置（PRD-chat 第 4.1 节）。
func loadAccount(getenv func(string) string, a *app.Config, bad func(string, ...any)) {
	c := &a.Account
	switch v := getenv("IM_REGISTER_MODE"); v {
	case "", account.ModeClosed, account.ModeInvite, account.ModeOpen:
		c.RegisterMode = cmp.Or(v, account.ModeInvite)
	default:
		bad("IM_REGISTER_MODE must be closed, invite or open, got %q", v)
	}
	if raw := getenv("IM_REFRESH_KEYS"); raw != "" {
		keys, err := parseKeys(raw)
		if err != nil {
			bad("IM_REFRESH_KEYS: %v", err)
		}
		for _, k := range keys {
			if !refreshKidRE.MatchString(k.ID) {
				bad("IM_REFRESH_KEYS: key id must match %s", refreshKidRE)
			}
			c.RefreshKeys = append(c.RefreshKeys, account.RefreshKey{ID: k.ID, Secret: k.Secret})
		}
		c.RefreshActive = getenv("IM_REFRESH_ACTIVE_KID")
		if c.RefreshActive == "" && len(keys) == 1 {
			c.RefreshActive = keys[0].ID
		}
		if !slices.ContainsFunc(keys, func(k auth.Key) bool { return k.ID == c.RefreshActive }) && err == nil {
			bad("IM_REFRESH_ACTIVE_KID must name a key in IM_REFRESH_KEYS")
		}
	}
	for key, dst := range map[string]*time.Duration{
		"IM_ACCESS_TTL": &c.AccessTTL, "IM_REFRESH_TTL": &c.RefreshTTL, "IM_REFRESH_TTL_WEB": &c.RefreshTTLWeb,
		"IM_REFRESH_ABSOLUTE": &c.RefreshAbsolute, "IM_RECOVER_QUEUE": &c.RecoverQueue, "IM_REFRESH_OVERLAP": &c.RefreshOverlap,
	} {
		if v := getenv(key); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				bad("%s: invalid duration %q", key, v)
				continue
			}
			*dst = d
		}
	}
	if v := getenv("IM_SESSION_RECHECK"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			bad("IM_SESSION_RECHECK: invalid duration %q", v)
		} else {
			a.Gateway.RecheckInterval = d
		}
	}
	// 重叠窗口必须大于 SDK 请求超时 30 秒 + 网关超时（第 4.1 节）：否则超时重试的旧值落在窗口外，被当成重放。
	if c.RefreshOverlap != 0 && c.RefreshOverlap <= minRefreshOverlap {
		bad("IM_REFRESH_OVERLAP must be greater than %s (SDK request timeout 30s + gateway timeout 30s), got %s", minRefreshOverlap, c.RefreshOverlap)
	}
	for key, dst := range map[string]*int{
		"IM_ARGON_SLOTS": &c.ArgonSlots, "IM_ARGON_RECOVER_SLOTS": &c.RecoverSlots, "IM_REFRESH_MAX_PENDING": &c.RefreshMaxPending,
		"IM_REFRESH_ISSUE_RATE": &c.RefreshIssueRate, "IM_REFRESH_IP_RATE": &c.RefreshIPRate, "IM_RECOVERY_OLD_DAYS": &c.RecoveryOldDays,
		"IM_INVITE_WARN_TOTAL": &c.InviteWarnTotal,
	} {
		if v := getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				bad("%s: must be a positive integer, got %q", key, v)
				continue
			}
			*dst = n
		}
	}
	c.PrivacyVersion, c.PrivacyURL = getenv("IM_PRIVACY_NOTICE_VERSION"), getenv("IM_PRIVACY_NOTICE_URL")
	// 保留名可以用配置文件增加（F-ACC-01 规则 4）：一行一个，# 开头是注释。
	if path := getenv("IM_RESERVED_NAMES_FILE"); path != "" {
		b, err := os.ReadFile(path) // #nosec G304,G703 -- 路径来自运维配置
		if err != nil {
			bad("IM_RESERVED_NAMES_FILE: %v", err)
		}
		for line := range strings.Lines(string(b)) {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				c.ExtraReserved = append(c.ExtraReserved, line)
			}
		}
	}
}

// addrEnv 读取监听地址：未设置用默认值，"off" 表示不开启（返回空串）。
func addrEnv(getenv func(string) string, key, def string) string {
	switch v := getenv(key); v {
	case "":
		return def
	case "off":
		return ""
	default:
		return v
	}
}

// addrConflicts 检查已启用的监听地址两两不同（PRD-admin F-KEY-01 规则 4）。
func addrConflicts(addrs map[string]string) []string {
	var out []string
	keys := []string{"IM_ADDR", "IM_ADMIN_ADDR", "IM_OPS_ADDR", "IM_METRICS_ADDR", "IM_PPROF_ADDR"}
	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if addrs[a] != "" && addrs[a] == addrs[b] {
				out = append(out, fmt.Sprintf("%s and %s must be different (both %s)", a, b, addrs[a]))
			}
		}
	}
	return out
}

// isLoopback 判断监听地址是否只绑定本机（127.0.0.0/8、::1、localhost）；":8081" 这种监听全部网卡的不算。
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// parseOpsKeys 解析 IM_OPS_PUBKEYS（kid:base64，最多 2 把，用于轮换）。错误信息只报序号和 kid。
func parseOpsKeys(raw string) ([]opsapi.Key, error) {
	if raw == "" {
		return nil, nil
	}
	var keys []opsapi.Key
	for i, item := range splitList(raw) {
		id, b64, ok := strings.Cut(item, ":")
		if !ok || id == "" {
			return nil, fmt.Errorf("entry #%d must be kid:base64", i+1)
		}
		pub, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("key %s is not a valid Ed25519 public key", id)
		}
		keys = append(keys, opsapi.Key{ID: id, Public: ed25519.PublicKey(pub)})
	}
	if len(keys) > 2 {
		return nil, errors.New("at most 2 keys")
	}
	return keys, nil
}

func parseKeys(raw string) ([]auth.Key, error) {
	var keys []auth.Key
	for i, item := range splitList(raw) {
		id, secret, ok := strings.Cut(item, ":")
		if !ok || id == "" || secret == "" {
			// 只报序号：格式错的条目可能整条都是密钥（漏写了 "kid:"），不能打印出来。
			return nil, fmt.Errorf("entry #%d must be kid:secret", i+1)
		}
		keys = append(keys, auth.Key{ID: id, Secret: []byte(secret)})
	}
	if len(keys) == 0 {
		return nil, errors.New("no keys")
	}
	return keys, nil
}

func boolEnv(getenv func(string) string, key string, def bool, bad func(string, ...any)) bool {
	switch getenv(key) {
	case "":
		return def
	case "1", "true":
		return true
	case "0", "false":
		return false
	default:
		bad("%s must be 0 or 1, got %q", key, getenv(key))
		return def
	}
}

// parsePrefix 接受 CIDR（10.0.0.0/8）或单个地址（10.0.0.1，视为 /32 或 /128）。
func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		return p.Masked(), err
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func splitList(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}
