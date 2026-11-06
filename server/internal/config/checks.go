package config

import (
	"cmp"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"tianting/server/internal/account"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/testhook"
)

// Check 是一项生产启动保护（PRD-ops F-OPS-10、S-OPS-16）。ID 稳定；Detail 是"实际检查了什么"，不含任何密钥的值或哈希。
// 启动保护和配置检查接口共用同一份列表，新增检查项时接口自动出现（AC-OPS-10-9）。
type Check struct {
	ID         string `json:"id"`
	Passed     bool   `json:"passed"`
	Applicable bool   `json:"applicable"` // 非生产环境为 false，页面显示"不适用"
	Detail     string `json:"detail"`

	problems []string // 没通过时拒绝启动的原因
}

// productionChecks 逐项检查，一次列出全部问题。
func productionChecks(cfg Server) []Check {
	a := cfg.App
	var out []Check
	add := func(id, detail string, problems ...string) {
		out = append(out, Check{ID: id, Passed: len(problems) == 0, Applicable: true, Detail: detail, problems: problems})
	}
	secretProblems := func(name, value, devDefault string) []string {
		switch {
		case value == "":
			return []string{name + " is required in production"}
		case value == devDefault:
			return []string{name + " must not be the development default"}
		case len(value) < MinProductionSecretLen:
			return []string{fmt.Sprintf("%s must be at least %d bytes in production", name, MinProductionSecretLen)}
		}
		return nil
	}

	var jwt, distinct []string
	if len(a.JWTKeys) > 0 {
		shortest := math.MaxInt
		for _, k := range a.JWTKeys {
			jwt = append(jwt, secretProblems("IM_JWT_KEYS["+k.ID+"]", string(k.Secret), DevJWTSecret)...)
			shortest = min(shortest, len(k.Secret))
			// 逐把和管理密钥比较（修复 R-OPS-30）：错误信息只报 kid。
			if a.AdminKey != "" && string(k.Secret) == a.AdminKey {
				distinct = append(distinct, "IM_JWT_KEYS["+k.ID+"] and IM_ADMIN_KEY must be different")
			}
		}
		add("jwt_keys", fmt.Sprintf("IM_JWT_KEYS：%d 把，最短 %d 字节", len(a.JWTKeys), shortest), jwt...)
	} else {
		jwt = secretProblems("IM_JWT_SECRET", a.JWTSecret, DevJWTSecret)
		add("jwt_keys", fmt.Sprintf("IM_JWT_SECRET：%d 字节", len(a.JWTSecret)), jwt...)
		if a.JWTSecret != "" && a.JWTSecret == a.AdminKey {
			distinct = append(distinct, "IM_JWT_SECRET and IM_ADMIN_KEY must be different")
		}
	}
	add("jwt_admin_distinct", "JWT 密钥与管理密钥逐把比较", distinct...)

	// 生产环境的管理鉴权：签名调用方（IM_ADMIN_CALLERS），或显式开启的旧密钥（IM_ADMIN_LEGACY=1 + IM_ADMIN_KEY）。
	var admin []string
	switch {
	case a.AdminKey != "":
		admin = append(admin, secretProblems("IM_ADMIN_KEY", a.AdminKey, DevAdminKey)...)
	case !a.AdminLegacyDisabled:
		admin = append(admin, "IM_ADMIN_LEGACY=1 requires IM_ADMIN_KEY")
	}
	if cfg.AdminAddr != "" && len(a.AdminCallers) == 0 && (a.AdminLegacyDisabled || a.AdminKey == "") {
		admin = append(admin, "admin callers are required in production: set IM_ADMIN_CALLERS (or IM_ADMIN_KEY with IM_ADMIN_LEGACY=1)")
	}
	add("admin_auth", fmt.Sprintf("签名调用方 %d 个；旧密钥%s", len(a.AdminCallers), enabledText(!a.AdminLegacyDisabled)), admin...)

	// 被删原文的隔离密钥（AC-ADM-32-4）：不管第二批开没开都检查，避免打开开关时才发现没配。
	var qk []string
	qk = append(qk, secretProblems("IM_QUARANTINE_KEY", a.QuarantineKey, DevQuarantineKey)...)
	if a.QuarantineKey != "" {
		if len(a.QuarantineKey) < quarantine.MinKeyLen {
			qk = append(qk, fmt.Sprintf("IM_QUARANTINE_KEY must be at least %d bytes", quarantine.MinKeyLen))
		}
		same := a.QuarantineKey == a.JWTSecret || a.QuarantineKey == a.AdminKey
		for _, k := range a.JWTKeys {
			same = same || a.QuarantineKey == string(k.Secret)
		}
		for _, c := range a.AdminCallers {
			same = same || a.QuarantineKey == string(c.Secret)
		}
		if same {
			qk = append(qk, "IM_QUARANTINE_KEY must differ from the JWT keys, IM_ADMIN_KEY and caller secrets")
		}
	}
	add("quarantine_key", fmt.Sprintf("IM_QUARANTINE_KEY：%d 字节，与其他密钥逐把比较", len(a.QuarantineKey)), qk...)

	// refresh 密钥（PRD-chat F-ACC-03 规则 3）：独立于其他密钥。没配时账号接口 503，其他功能不受影响，所以不拒绝启动。
	var rk []string
	for _, k := range a.Account.RefreshKeys {
		rk = append(rk, secretProblems("IM_REFRESH_KEYS["+k.ID+"]", string(k.Secret), DevRefreshKey)...)
		same := string(k.Secret) == a.JWTSecret || string(k.Secret) == a.AdminKey || string(k.Secret) == a.QuarantineKey
		for _, j := range a.JWTKeys {
			same = same || string(k.Secret) == string(j.Secret)
		}
		for _, c := range a.AdminCallers {
			same = same || string(k.Secret) == string(c.Secret)
		}
		if same {
			rk = append(rk, "IM_REFRESH_KEYS["+k.ID+"] must differ from the JWT keys, IM_ADMIN_KEY, IM_QUARANTINE_KEY and caller secrets")
		}
	}
	add("refresh_keys", fmt.Sprintf("IM_REFRESH_KEYS：%d 把，未配置时账号接口不可用", len(a.Account.RefreshKeys)), rk...)
	// 没配可信代理时 /v1/* 只接受本进程终止的 TLS，经代理的请求都被拒绝。不拒绝启动：不用账号功能的部署不受影响。
	add("v1_https", fmt.Sprintf("/v1/* 只接受 HTTPS；可信代理 %d 个网段（0 个时经代理的请求都被拒绝）", len(a.Gateway.Guard.TrustedProxies)))

	// 开放注册（PRD-chat F-ACC-01 规则 2，AC-ACC-01-19）：C6 上线前 IM_MOD_ENABLED 没有实现，所以一律拒绝。逐项报错随 C6。
	flag0 := []string(nil)
	if a.Account.RegisterMode == account.ModeOpen {
		flag0 = append(flag0, "IM_REGISTER_MODE=open is refused in production: it requires content moderation (IM_MOD_ENABLED), which is not available yet")
	}
	add("register_mode", "IM_REGISTER_MODE："+cmp.Or(a.Account.RegisterMode, account.ModeInvite), flag0...)

	var tls []string
	// 管理端口只有绑定回环地址（同机部署的业务服务端）时允许明文（PRD-admin F-KEY-01 规则 2）。
	if cfg.AdminAddr != "" && cfg.AdminTLSCert == "" && !isLoopback(cfg.AdminAddr) {
		tls = append(tls, "IM_ADMIN_ADDR "+cfg.AdminAddr+" is not a loopback address: IM_ADMIN_TLS_CERT and IM_ADMIN_TLS_KEY are required in production")
	}
	add("admin_tls", "IM_ADMIN_ADDR："+tlsText(cfg.AdminAddr, cfg.AdminTLSCert), tls...)

	var flag []string
	if a.TestMode {
		flag = append(flag, "IM_TEST_MODE must not be enabled in production")
	}
	add("test_mode_off", "IM_TEST_MODE："+enabledText(a.TestMode), flag...)
	// 测试设施的开关不跟 IM_TEST_MODE 走：生产配置里出现就拒绝启动（PRD-chat R-CQ-01，AC-CHAT-90-18）。只报名字，不报值。
	flag = nil
	for _, name := range cfg.TestOverrides {
		flag = append(flag, name+" must not be set in production")
	}
	if testhook.Compiled {
		flag = append(flag, "this binary was built with -tags imhooks and must not run in production")
	}
	add("test_overrides_off", fmt.Sprintf("测试设施开关：%d 项；账号并发钩子：%s", len(cfg.TestOverrides), cmpText(testhook.Compiled, "已编入", "未编入")), flag...)
	flag = nil
	if a.DemoLogin {
		flag = append(flag, "IM_DEMO_LOGIN must not be enabled in production")
	}
	add("demo_login_off", "IM_DEMO_LOGIN："+enabledText(a.DemoLogin), flag...)
	flag = nil
	if containsWildcard(a.Gateway.AllowedOrigins) {
		flag = append(flag, "IM_ALLOWED_ORIGINS must not contain * in production")
	}
	// 写实际情况：开发环境允许 *，检查项的说明不能照抄"不含 *"。
	add("origins_no_wildcard", fmt.Sprintf("IM_ALLOWED_ORIGINS：%d 项，%s", len(a.Gateway.AllowedOrigins), cmpText(containsWildcard(a.Gateway.AllowedOrigins), "含 *", "不含 *")), flag...)

	// PRD-ops S-OPS-19。
	flag = nil
	if a.Gateway.NodeID == "" {
		flag = append(flag, "IM_NODE_ID is required in production")
	}
	add("node_id", "IM_NODE_ID："+cmpText(a.Gateway.NodeID != "", "已配置", "未配置"), flag...)

	flag = nil
	if cfg.OpsAddr != "" && len(a.OpsKeys) == 0 {
		flag = append(flag, "IM_OPS_PUBKEYS is required in production unless IM_OPS_ADDR=off")
	}
	add("ops_pubkeys", fmt.Sprintf("IM_OPS_PUBKEYS：%d 把合法的 Ed25519 公钥", len(a.OpsKeys)), flag...)

	flag = nil
	if cfg.OpsAddr != "" && !isLoopback(cfg.OpsAddr) && cfg.OpsTLSCert == "" {
		flag = append(flag, "IM_OPS_ADDR "+cfg.OpsAddr+" is not a loopback address: IM_OPS_TLS_CERT and IM_OPS_TLS_KEY are required in production")
	}
	add("ops_tls", "IM_OPS_ADDR："+tlsText(cfg.OpsAddr, cfg.OpsTLSCert), flag...)

	flag = nil
	if cfg.OpsAddr != "" && !isLoopback(cfg.OpsAddr) && len(a.OpsAllowedCIDRs) == 0 {
		flag = append(flag, "IM_OPS_ADDR "+cfg.OpsAddr+" is not a loopback address: IM_OPS_ALLOWED_CIDRS is required in production")
	}
	for _, c := range a.OpsAllowedCIDRs {
		if c.Bits() == 0 {
			flag = append(flag, "IM_OPS_ALLOWED_CIDRS must not contain "+c.String()+" in production")
		}
	}
	add("ops_cidrs", fmt.Sprintf("IM_OPS_ALLOWED_CIDRS：%d 个网段", len(a.OpsAllowedCIDRs)), flag...)

	flag = nil
	if cfg.PprofAddr != "" && !isLoopback(cfg.PprofAddr) {
		flag = append(flag, "IM_PPROF_ADDR "+cfg.PprofAddr+" must be a loopback address")
	}
	add("pprof_loopback", "IM_PPROF_ADDR："+cmpText(cfg.PprofAddr == "", "关闭", cfg.PprofAddr), flag...)

	// 地址冲突在所有环境都拒绝启动（Load 里已经报了），这里只给页面显示结果。
	conflicts := addrConflicts(map[string]string{
		"IM_ADDR": cfg.Addr, "IM_ADMIN_ADDR": cfg.AdminAddr, "IM_OPS_ADDR": cfg.OpsAddr,
		"IM_METRICS_ADDR": cfg.MetricsAddr, "IM_PPROF_ADDR": cfg.PprofAddr,
	})
	out = append(out, Check{ID: "addrs_distinct", Passed: len(conflicts) == 0, Applicable: true, Detail: "已启用的监听地址两两不同"})
	return out
}

// productionProblems 列出生产环境下不允许的配置（ARCH S4）。
func productionProblems(cfg Server) []string {
	var out []string
	for _, c := range productionChecks(cfg) {
		out = append(out, c.problems...)
	}
	return out
}

func enabledText(on bool) string { return cmpText(on, "开启", "关闭") }

func cmpText(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func tlsText(addr, cert string) string {
	switch {
	case addr == "":
		return "关闭"
	case cert != "":
		return "已启用 TLS"
	case isLoopback(addr):
		return "回环地址，未启用 TLS"
	}
	return "非回环地址，未启用 TLS"
}

// ConfigSummary 是 GET /internal/v1/config 的响应（PRD-ops F-OPS-10 规则 1 的 7 组）。
// 密钥类只有 kid、数量和"已配置 / 长度达标"，没有值和哈希。
type ConfigSummary struct {
	Env      EnvSummary     `json:"env"`
	Checks   []Check        `json:"productionChecks"`
	Keys     KeysSummary    `json:"keys"`
	Network  NetworkSummary `json:"network"`
	Limits   LimitsSummary  `json:"limits"`
	Database DBSummary      `json:"database"`
	Switches SwitchSummary  `json:"switches"`
}

type EnvSummary struct {
	Env          string `json:"env"`
	Version      string `json:"version"`
	NodeID       string `json:"nodeId"`
	NodeIDSet    bool   `json:"nodeIdConfigured"`
	GoVersion    string `json:"goVersion"`
	GOMAXPROCS   int    `json:"gomaxprocs"`
	GOMEMLIMIT   int64  `json:"gomemlimit"` // 0 表示未设置
	GOGC         string `json:"gogc"`
	PlannedConns int    `json:"plannedConns"`
}

type KeysSummary struct {
	JWTKids       []string `json:"jwtKids"`
	JWTActiveKid  string   `json:"jwtActiveKid"`
	JWTLengthOK   bool     `json:"jwtLengthOk"`
	AdminKeySet   bool     `json:"adminKeyConfigured"`
	AdminKeyLenOK bool     `json:"adminKeyLengthOk"`
	AdminCallers  int      `json:"adminCallers"`
	OpsKids       []string `json:"opsKids"`
}

type NetworkSummary struct {
	Addr            string   `json:"addr"`
	MetricsAddr     string   `json:"metricsAddr"`
	AdminAddr       string   `json:"adminAddr"`
	OpsAddr         string   `json:"opsAddr"`
	OpsTLS          bool     `json:"opsTls"`
	OpsAllowedCIDRs []string `json:"opsAllowedCidrs"`
	PprofAddr       string   `json:"pprofAddr"`
	AllowedOrigins  []string `json:"allowedOrigins"`
	TrustedProxies  []string `json:"trustedProxies"`
}

type LimitsSummary struct {
	HandshakeRate   int    `json:"handshakeRate"`
	HandshakeBurst  int    `json:"handshakeBurst"`
	MaxConnsPerUser int    `json:"maxConnsPerUser"`
	SendRate        int    `json:"sendRate"`
	IdleTimeout     string `json:"idleTimeout"`
	DrainDelay      string `json:"drainDelay"`
}

type DBSummary struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

type SwitchSummary struct {
	TestMode    bool `json:"testMode"`
	DemoLogin   bool `json:"demoLogin"`
	AutoMigrate bool `json:"autoMigrate"`
}

// Summary 生成脱敏的配置摘要。
func Summary(cfg Server, version, nodeID string) ConfigSummary {
	a := cfg.App
	limit := debug.SetMemoryLimit(-1)
	if limit == math.MaxInt64 {
		limit = 0
	}
	checks := productionChecks(cfg)
	if cfg.Env != EnvProduction {
		for i := range checks {
			checks[i].Applicable = false
		}
	}
	keys := KeysSummary{
		JWTKids: []string{}, JWTActiveKid: a.JWTActiveKID, AdminKeySet: a.AdminKey != "",
		AdminKeyLenOK: len(a.AdminKey) >= MinProductionSecretLen, AdminCallers: len(a.AdminCallers), OpsKids: []string{},
		JWTLengthOK: len(a.JWTSecret) >= MinProductionSecretLen,
	}
	if len(a.JWTKeys) > 0 {
		keys.JWTLengthOK = true
		for _, k := range a.JWTKeys {
			keys.JWTKids = append(keys.JWTKids, k.ID)
			keys.JWTLengthOK = keys.JWTLengthOK && len(k.Secret) >= MinProductionSecretLen
		}
	}
	for _, k := range a.OpsKeys {
		keys.OpsKids = append(keys.OpsKids, k.ID)
	}
	cidrs := make([]string, 0, len(a.OpsAllowedCIDRs))
	for _, p := range a.OpsAllowedCIDRs {
		cidrs = append(cidrs, p.String())
	}
	proxies := make([]string, 0, len(a.Gateway.Guard.TrustedProxies))
	for _, p := range a.Gateway.Guard.TrustedProxies {
		proxies = append(proxies, p.String())
	}
	return ConfigSummary{
		Env: EnvSummary{
			Env: cfg.Env, Version: version, NodeID: nodeID, NodeIDSet: a.Gateway.NodeID != "", GoVersion: runtime.Version(),
			GOMAXPROCS: runtime.GOMAXPROCS(0), GOMEMLIMIT: limit, GOGC: cfg.GOGC, PlannedConns: cfg.PlannedConns,
		},
		Checks: checks,
		Keys:   keys,
		Network: NetworkSummary{
			Addr: cfg.Addr, MetricsAddr: cfg.MetricsAddr, AdminAddr: cfg.AdminAddr, OpsAddr: cfg.OpsAddr, OpsTLS: cfg.OpsTLSCert != "",
			OpsAllowedCIDRs: cidrs, PprofAddr: cfg.PprofAddr, AllowedOrigins: slices.Clone(a.Gateway.AllowedOrigins), TrustedProxies: proxies,
		},
		Limits: LimitsSummary{
			HandshakeRate: a.Gateway.Guard.HandshakeRate, HandshakeBurst: a.Gateway.Guard.HandshakeBurst,
			MaxConnsPerUser: a.Gateway.MaxConnsPerUser, SendRate: a.SendRate, IdleTimeout: a.Gateway.IdleTimeout.String(), DrainDelay: cfg.DrainDelay,
		},
		Database: DBSummary{Type: "sqlite", Location: sanitizeDBPath(a.DBPath)},
		Switches: SwitchSummary{TestMode: a.TestMode, DemoLogin: a.DemoLogin, AutoMigrate: a.AutoMigrate},
	}
}

// sanitizeDBPath：SQLite 是文件路径；连接串形式（B6 的 Postgres）只留主机和库名，由 BE-340 实现，这里至少不输出 @ 之前的凭证。
func sanitizeDBPath(p string) string {
	if i := strings.LastIndex(p, "@"); i >= 0 {
		return p[i+1:]
	}
	return p
}
