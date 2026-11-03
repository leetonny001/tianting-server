// Package config 从环境变量读取服务配置，并做安全校验（ARCH S4）。
//
// 原则：开发环境尽量开箱即用（缺密钥时用开发默认值并告警）；
// 生产环境（IM_ENV=production）任何不安全的配置都拒绝启动，并一次性列出全部问题。
package config

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/auth"
	"tianting/server/internal/gateway"
)

// 运行环境。
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// 开发默认密钥：只允许在非生产环境使用，生产环境检测到这两个值会拒绝启动。
const (
	DevJWTSecret = "dev-secret-change-me-0123456789"
	DevAdminKey  = "dev-admin-key"
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
	AdminAddr    string
	AdminTLSCert string
	AdminTLSKey  string
	// OpsAddr、PprofAddr 预留给运维接口和 pprof（PRD-ops F-OPS-16、F-OPS-17），目前只参与地址冲突检查。
	OpsAddr   string
	PprofAddr string
	App       app.Config
}

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
	cfg.OpsAddr = addrEnv(getenv, "IM_OPS_ADDR", "")
	cfg.PprofAddr = addrEnv(getenv, "IM_PPROF_ADDR", "")
	cfg.AdminTLSCert, cfg.AdminTLSKey = getenv("IM_ADMIN_TLS_CERT"), getenv("IM_ADMIN_TLS_KEY")
	if (cfg.AdminTLSCert == "") != (cfg.AdminTLSKey == "") {
		bad("IM_ADMIN_TLS_CERT and IM_ADMIN_TLS_KEY must be set together")
	}
	problems = append(problems, addrConflicts(map[string]string{
		"IM_ADDR": cfg.Addr, "IM_ADMIN_ADDR": cfg.AdminAddr, "IM_OPS_ADDR": cfg.OpsAddr,
		"IM_METRICS_ADDR": cfg.MetricsAddr, "IM_PPROF_ADDR": cfg.PprofAddr,
	})...)
	a := &cfg.App
	a.DBPath = cmp.Or(getenv("IM_DB"), "im.db")
	a.AutoMigrate = boolEnv(getenv, "IM_AUTO_MIGRATE", !prod, bad) // 生产默认由发布流程执行 migrate up
	a.TestMode = boolEnv(getenv, "IM_TEST_MODE", false, bad)
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

	if !prod {
		if a.JWTSecret == "" && len(a.JWTKeys) == 0 {
			a.JWTSecret = DevJWTSecret
			warnings = append(warnings, "IM_JWT_SECRET not set, using the development default")
		}
		if a.AdminKey == "" {
			a.AdminKey = DevAdminKey
			warnings = append(warnings, "IM_ADMIN_KEY not set, using the development default")
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
	return cfg, warnings, nil
}

// productionProblems 列出生产环境下不允许的配置（ARCH S4）。
func productionProblems(cfg Server) []string {
	var out []string
	a := cfg.App
	checkSecret := func(name, value, devDefault string) {
		switch {
		case value == "":
			out = append(out, name+" is required in production")
		case value == devDefault:
			out = append(out, name+" must not be the development default")
		case len(value) < MinProductionSecretLen:
			out = append(out, fmt.Sprintf("%s must be at least %d bytes in production", name, MinProductionSecretLen))
		}
	}
	if len(a.JWTKeys) > 0 {
		for _, k := range a.JWTKeys {
			checkSecret("IM_JWT_KEYS["+k.ID+"]", string(k.Secret), DevJWTSecret)
		}
	} else {
		checkSecret("IM_JWT_SECRET", a.JWTSecret, DevJWTSecret)
	}
	checkSecret("IM_ADMIN_KEY", a.AdminKey, DevAdminKey)
	if a.JWTSecret != "" && a.JWTSecret == a.AdminKey {
		out = append(out, "IM_JWT_SECRET and IM_ADMIN_KEY must be different")
	}
	if a.TestMode {
		out = append(out, "IM_TEST_MODE must not be enabled in production")
	}
	if a.DemoLogin {
		out = append(out, "IM_DEMO_LOGIN must not be enabled in production")
	}
	if containsWildcard(a.Gateway.AllowedOrigins) {
		out = append(out, "IM_ALLOWED_ORIGINS must not contain * in production")
	}
	// 管理端口只有绑定回环地址（同机部署的业务服务端）时允许明文（PRD-admin F-KEY-01 规则 2）。
	if cfg.AdminAddr != "" && cfg.AdminTLSCert == "" && !isLoopback(cfg.AdminAddr) {
		out = append(out, "IM_ADMIN_ADDR "+cfg.AdminAddr+" is not a loopback address: IM_ADMIN_TLS_CERT and IM_ADMIN_TLS_KEY are required in production")
	}
	return out
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
