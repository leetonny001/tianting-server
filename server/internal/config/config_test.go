package config

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tianting/server/internal/app"
)

// callersFile 写一个调用方配置文件（权限 0600）并返回路径。
func callersFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "callers.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

var bizSecret = base64.StdEncoding.EncodeToString([]byte("biz1-hmac-secret-0123456789abcdef!"))

// envOf：生产环境的用例默认带上节点 ID 并关闭运维接口，只验证各自关心的项；要测这两项的用例显式传值（"-" 表示不设置）。
func envOf(m map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := m[k]; ok {
			if v == "-" {
				return ""
			}
			return v
		}
		if m["IM_ENV"] == "production" {
			switch k {
			case "IM_NODE_ID":
				return "n1"
			case "IM_OPS_ADDR":
				return "off"
			}
		}
		return ""
	}
}

const (
	strongJWT   = "jwt-0123456789abcdef0123456789abcdef"
	strongAdmin = "adm-0123456789abcdef0123456789abcdef"
)

// 开发环境开箱即用：缺密钥时用默认值并告警；自动迁移默认开启。
func TestDevelopmentDefaults(t *testing.T) {
	cfg, warnings, err := Load(envOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != EnvDevelopment || cfg.App.JWTSecret != DevJWTSecret || cfg.App.AdminKey != DevAdminKey || !cfg.App.AutoMigrate {
		t.Errorf("cfg = %+v", cfg)
	}
	if len(warnings) != 4 {
		t.Errorf("warnings = %v, want 4 (default keys, ops API not listening)", warnings)
	}
	if cfg.App.Gateway.MaxConnsPerUser != 10 || len(cfg.App.Gateway.AllowedOrigins) == 0 {
		t.Errorf("gateway defaults = %+v", cfg.App.Gateway)
	}
}

// ARCH S4：生产环境的每一种不安全配置都拒绝启动，并在一条错误里全部列出。
func TestProductionRejectsUnsafeConfig(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{
		"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine,
		"IM_JWT_SECRET":      DevJWTSecret,
		"IM_ADMIN_KEY":       "short",
		"IM_TEST_MODE":       "1",
		"IM_DEMO_LOGIN":      "1",
		"IM_ALLOWED_ORIGINS": "*",
	}))
	if err == nil {
		t.Fatal("unsafe production config accepted")
	}
	for _, want := range []string{
		"IM_JWT_SECRET must not be the development default",
		"IM_ADMIN_KEY must be at least 32 bytes",
		"IM_TEST_MODE must not be enabled",
		"IM_DEMO_LOGIN must not be enabled",
		"IM_ALLOWED_ORIGINS must not contain *",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestProductionRequiresKeys(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{"IM_ENV": "production"}))
	if err == nil || !strings.Contains(err.Error(), "IM_JWT_SECRET is required") || !strings.Contains(err.Error(), "admin callers are required in production") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = Load(envOf(map[string]string{"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine, "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongJWT, "IM_ADMIN_LEGACY": "1"}))
	if err == nil || !strings.Contains(err.Error(), "must be different") {
		t.Fatalf("same secret for jwt and admin: err = %v", err)
	}
}

func TestProductionValid(t *testing.T) {
	cfg, warnings, err := Load(envOf(map[string]string{
		"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine,
		"IM_JWT_KEYS":        "k1:" + strongJWT + ",k2:" + strongJWT + "x",
		"IM_JWT_ACTIVE_KID":  "k2",
		"IM_ADMIN_CALLERS":   callersFile(t, `[{"id":"biz1","alg":"hmac-sha256","secret":"`+bizSecret+`","scopes":["token.issue"]}]`),
		"IM_ALLOWED_ORIGINS": "https://im.example.com, *.example.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
	if cfg.App.AutoMigrate {
		t.Error("production must not auto-migrate by default")
	}
	if len(cfg.App.AdminCallers) != 1 || !cfg.App.AdminLegacyDisabled {
		t.Errorf("callers = %+v legacyDisabled=%v", cfg.App.AdminCallers, cfg.App.AdminLegacyDisabled)
	}
	if len(cfg.App.JWTKeys) != 2 || cfg.App.JWTActiveKID != "k2" {
		t.Errorf("keys = %+v active=%q", cfg.App.JWTKeys, cfg.App.JWTActiveKID)
	}
	if got := cfg.App.Gateway.AllowedOrigins; len(got) != 2 || got[1] != "*.example.com" {
		t.Errorf("origins = %v", got)
	}
}

func TestInvalidValues(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"env":         {"IM_ENV": "prod"},
		"bool":        {"IM_TEST_MODE": "yes"},
		"idle":        {"IM_IDLE_TIMEOUT": "soon"},
		"max conns":   {"IM_MAX_CONNS_PER_USER": "0"},
		"keys format": {"IM_JWT_KEYS": "nocolon"},
		"active kid":  {"IM_JWT_KEYS": "k1:" + strongJWT + ",k2:" + strongJWT},
		"proxies":     {"IM_TRUSTED_PROXIES": "10.0.0.0/8,not-an-ip"},
		"hs rate":     {"IM_HANDSHAKE_RATE": "0"},
	} {
		if _, _, err := Load(envOf(env)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// 错误信息里不能出现密钥本身。
func TestErrorsDoNotLeakSecrets(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{"IM_JWT_KEYS": "k1:supersecretvalue,broken-secret-entry"}))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecretvalue") || strings.Contains(err.Error(), "broken-secret-entry") {
		t.Errorf("secret leaked in error: %v", err)
	}
}

func TestTrustedProxies(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{"IM_TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.10, ::1"}))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.App.Gateway.Guard.TrustedProxies
	if len(got) != 3 || got[0].String() != "10.0.0.0/8" || got[1].String() != "192.168.1.10/32" || got[2].String() != "::1/128" {
		t.Errorf("trusted proxies = %v", got)
	}
}

func TestMetricsAddr(t *testing.T) {
	for env, want := range map[string]string{"": "127.0.0.1:9090", "off": "", "10.0.0.5:9100": "10.0.0.5:9100"} {
		cfg, _, err := Load(envOf(map[string]string{"IM_METRICS_ADDR": env}))
		if err != nil || cfg.MetricsAddr != want {
			t.Errorf("IM_METRICS_ADDR=%q: got %q, want %q (%v)", env, cfg.MetricsAddr, want, err)
		}
	}
}

func TestHandshakeLimits(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{"IM_HANDSHAKE_RATE": "2000", "IM_HANDSHAKE_BURST": "5000"}))
	if err != nil || cfg.App.Gateway.Guard.HandshakeRate != 2000 || cfg.App.Gateway.Guard.HandshakeBurst != 5000 {
		t.Errorf("guard = %+v, %v", cfg.App.Gateway.Guard, err)
	}
}

// PRD-admin F-KEY-01：管理端口默认 127.0.0.1:8081，off 关闭；来源网段解析。
func TestAdminAddr(t *testing.T) {
	cfg, _, err := Load(envOf(nil))
	if err != nil || cfg.AdminAddr != "127.0.0.1:8081" {
		t.Fatalf("default admin addr = %q, err %v", cfg.AdminAddr, err)
	}
	cfg, _, err = Load(envOf(map[string]string{"IM_ADMIN_ADDR": "off", "IM_ADMIN_ALLOWED_CIDRS": "10.0.0.0/8, 192.168.1.5"}))
	if err != nil || cfg.AdminAddr != "" || len(cfg.App.AdminAllowedCIDRs) != 2 {
		t.Fatalf("cfg = %+v, err %v", cfg, err)
	}
	if _, _, err := Load(envOf(map[string]string{"IM_ADMIN_ALLOWED_CIDRS": "10.0.0.0/33"})); err == nil {
		t.Error("invalid CIDR accepted")
	}
	if _, _, err := Load(envOf(map[string]string{"IM_ADMIN_TLS_CERT": "c.pem"})); err == nil {
		t.Error("TLS cert without key accepted")
	}
}

// AC-KEY-01-2、AC-KEY-01-3：生产环境管理地址不是回环地址时必须配置 TLS；回环地址允许明文。
func TestProductionAdminTLS(t *testing.T) {
	prod := func(extra map[string]string) error {
		m := map[string]string{"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine, "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongAdmin, "IM_ADMIN_LEGACY": "1"}
		for k, v := range extra {
			m[k] = v
		}
		_, _, err := Load(envOf(m))
		return err
	}
	if err := prod(map[string]string{"IM_ADMIN_ADDR": "10.0.0.5:8081"}); err == nil || !strings.Contains(err.Error(), "IM_ADMIN_TLS_CERT") {
		t.Errorf("non-loopback admin addr without TLS: err = %v", err)
	}
	if err := prod(map[string]string{"IM_ADMIN_ADDR": ":8081"}); err == nil {
		t.Error("admin addr on all interfaces without TLS accepted")
	}
	for _, ok := range []string{"127.0.0.1:8081", "[::1]:8081", "localhost:8081"} {
		if err := prod(map[string]string{"IM_ADMIN_ADDR": ok}); err != nil {
			t.Errorf("loopback %s rejected: %v", ok, err)
		}
	}
	if err := prod(map[string]string{"IM_ADMIN_ADDR": "10.0.0.5:8081", "IM_ADMIN_TLS_CERT": "c.pem", "IM_ADMIN_TLS_KEY": "k.pem"}); err != nil {
		t.Errorf("non-loopback with TLS rejected: %v", err)
	}
}

// AC-KEY-01-5：已启用的监听地址两两不能相同，错误里指出冲突的两项。
func TestAddrConflicts(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"IM_ADMIN_ADDR": "127.0.0.1:9090", "IM_METRICS_ADDR": "127.0.0.1:9090"}, "IM_ADMIN_ADDR and IM_METRICS_ADDR"},
		{map[string]string{"IM_ADMIN_ADDR": "127.0.0.1:8081", "IM_OPS_ADDR": "127.0.0.1:8081"}, "IM_ADMIN_ADDR and IM_OPS_ADDR"},
		{map[string]string{"IM_METRICS_ADDR": "127.0.0.1:7000", "IM_PPROF_ADDR": "127.0.0.1:7000"}, "IM_METRICS_ADDR and IM_PPROF_ADDR"},
	} {
		_, _, err := Load(envOf(tc.env))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: err = %v, want mention of %q", tc.env, err, tc.want)
		}
	}
	// 关闭的地址不参与比较。
	if _, _, err := Load(envOf(map[string]string{"IM_ADMIN_ADDR": "off", "IM_METRICS_ADDR": "off"})); err != nil {
		t.Errorf("two disabled addresses: %v", err)
	}
}

// PRD-admin F-KEY-05 规则 3：生产环境只有 IM_ADMIN_LEGACY=1 才接受旧密钥，并告警；/readyz 带 legacyAdminKey。
func TestProductionLegacyAdminKey(t *testing.T) {
	base := map[string]string{"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine, "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongAdmin,
		"IM_ADMIN_CALLERS": callersFile(t, `[{"id":"biz1","alg":"hmac-sha256","secret":"`+bizSecret+`","scopes":["token.issue"]}]`)}
	cfg, warnings, err := Load(envOf(base))
	if err != nil || !cfg.App.AdminLegacyDisabled || cfg.App.ReportLegacyAdmin || len(warnings) != 0 {
		t.Fatalf("without IM_ADMIN_LEGACY: cfg=%+v warnings=%v err=%v", cfg.App, warnings, err)
	}
	base["IM_ADMIN_LEGACY"] = "1"
	cfg, warnings, err = Load(envOf(base))
	if err != nil || cfg.App.AdminLegacyDisabled || !cfg.App.ReportLegacyAdmin || len(warnings) != 1 {
		t.Fatalf("with IM_ADMIN_LEGACY=1: cfg=%+v warnings=%v err=%v", cfg.App, warnings, err)
	}
	// 开发环境始终接受旧密钥（现有测试和 e2e 不改）。
	if cfg, _, _ := Load(envOf(nil)); cfg.App.AdminLegacyDisabled {
		t.Error("development must accept the legacy key")
	}
}

// AC-KEY-02-5：调用方密钥与 JWT 密钥相同，拒绝启动；错误信息不含密钥。
func TestCallerSecretMustDifferFromJWT(t *testing.T) {
	jwt := "shared-secret-0123456789abcdef0123"
	shared := base64.StdEncoding.EncodeToString([]byte(jwt))
	_, _, err := Load(envOf(map[string]string{
		"IM_JWT_SECRET":    jwt,
		"IM_ADMIN_CALLERS": callersFile(t, `[{"id":"biz1","alg":"hmac-sha256","secret":"`+shared+`","scopes":["token.issue"]}]`),
	}))
	if err == nil || !strings.Contains(err.Error(), "must differ from IM_JWT_SECRET") || strings.Contains(err.Error(), jwt) {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := Load(envOf(map[string]string{"IM_C2C_BODY_ACCESS": "maybe"})); err == nil {
		t.Error("invalid IM_C2C_BODY_ACCESS accepted")
	}
}

// AC-SOC-01-9 的启动部分：IM_SOCIAL_DEFAULTS 格式或范围非法拒绝启动，合法时原样带给 app。
func TestSocialDefaults(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{"IM_SOCIAL_DEFAULTS": `{"mode":"friends"}`}))
	if err != nil || cfg.App.SocialDefaults != `{"mode":"friends"}` {
		t.Fatalf("valid defaults: %v %q", err, cfg.App.SocialDefaults)
	}
	for _, bad := range []string{`{"mode":"x"}`, `{"maxFriends":1}`, `not json`, `{"foo":1}`} {
		if _, _, err := Load(envOf(map[string]string{"IM_SOCIAL_DEFAULTS": bad})); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

var opsKeyB64 = base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))

func prodOps(t *testing.T, extra map[string]string) (Server, error) {
	t.Helper()
	m := map[string]string{
		"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine, "IM_JWT_SECRET": strongJWT, "IM_ALLOWED_ORIGINS": "https://im.example.com",
		"IM_ADMIN_CALLERS": callersFile(t, `[{"id":"biz1","alg":"hmac-sha256","secret":"`+bizSecret+`","scopes":["token.issue"]}]`),
		"IM_OPS_PUBKEYS":   "k1:" + opsKeyB64,
	}
	for k, v := range extra {
		m[k] = v
	}
	cfg, _, err := Load(envOf(m))
	return cfg, err
}

// AC-OPS-16-4：五种组合。
func TestProductionOpsGuards(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]string
		want  string // 空表示能启动
	}{
		{"non-loopback without TLS", map[string]string{"IM_OPS_ADDR": "10.0.0.5:9091", "IM_OPS_ALLOWED_CIDRS": "10.0.0.0/8"}, "IM_OPS_TLS_CERT"},
		{"TLS but no CIDRs", map[string]string{"IM_OPS_ADDR": "10.0.0.5:9091", "IM_OPS_TLS_CERT": "c", "IM_OPS_TLS_KEY": "k"}, "IM_OPS_ALLOWED_CIDRS"},
		{"loopback without TLS", map[string]string{"IM_OPS_ADDR": "127.0.0.1:9091"}, ""},
		{"bad public key", map[string]string{"IM_OPS_ADDR": "127.0.0.1:9091", "IM_OPS_PUBKEYS": "k9:not-a-key"}, "k9"},
		{"no node id", map[string]string{"IM_OPS_ADDR": "127.0.0.1:9091", "IM_NODE_ID": "-"}, "IM_NODE_ID"},
		{"ops on without keys", map[string]string{"IM_OPS_ADDR": "127.0.0.1:9091", "IM_OPS_PUBKEYS": "-"}, "IM_OPS_PUBKEYS"},
		{"three keys", map[string]string{"IM_OPS_ADDR": "127.0.0.1:9091", "IM_OPS_PUBKEYS": "a:" + opsKeyB64 + ",b:" + opsKeyB64 + ",c:" + opsKeyB64}, "at most 2"},
		{"pprof not loopback", map[string]string{"IM_PPROF_ADDR": "0.0.0.0:6060"}, "IM_PPROF_ADDR"},
		{"pprof loopback", map[string]string{"IM_PPROF_ADDR": "127.0.0.1:6060"}, ""},
	} {
		_, err := prodOps(t, tc.extra)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
	// 一次列出全部问题。
	_, err := prodOps(t, map[string]string{"IM_OPS_ADDR": "10.0.0.5:9091", "IM_NODE_ID": "-"})
	for _, want := range []string{"IM_NODE_ID", "IM_OPS_TLS_CERT", "IM_OPS_ALLOWED_CIDRS"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

// 默认值：pprof 开发开、生产关；运维地址默认 9091；console-ops 不能出现在管理接口调用方里（S-OPS-12）。
func TestOpsDefaults(t *testing.T) {
	dev, _, err := Load(envOf(nil))
	if err != nil || dev.PprofAddr != "127.0.0.1:6060" || dev.OpsAddr != "127.0.0.1:9091" {
		t.Errorf("dev = %q %q %v", dev.PprofAddr, dev.OpsAddr, err)
	}
	prod, err := prodOps(t, nil)
	if err != nil || prod.PprofAddr != "" {
		t.Errorf("prod pprof = %q %v", prod.PprofAddr, err)
	}
	_, _, err = Load(envOf(map[string]string{
		"IM_ADMIN_CALLERS": callersFile(t, `[{"id":"console-ops","alg":"hmac-sha256","secret":"`+bizSecret+`","scopes":["user.read"]}]`),
	}))
	if err == nil || !strings.Contains(err.Error(), "console-ops") {
		t.Errorf("console-ops as admin caller: %v", err)
	}
}

// AC-OPS-10-8：某一把 JWT 密钥和管理密钥相同：拒绝启动，只报 kid，不打印密钥。
func TestJWTKeySameAsAdminKey(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{
		"IM_ENV": "production", "IM_QUARANTINE_KEY": strongQuarantine, "IM_JWT_KEYS": "k1:" + strongJWT + ",k2:" + strongAdmin, "IM_JWT_ACTIVE_KID": "k1",
		"IM_ADMIN_KEY": strongAdmin, "IM_ADMIN_LEGACY": "1", "IM_ALLOWED_ORIGINS": "https://im.example.com",
	}))
	if err == nil || !strings.Contains(err.Error(), "IM_JWT_KEYS[k2]") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), strongAdmin) {
		t.Errorf("secret leaked: %v", err)
	}
}

// AC-OPS-10-9：配置摘要的检查项和启动保护是同一份；AC-OPS-10-4：生产"通过"，开发"不适用"。
func TestChecksShared(t *testing.T) {
	prod, err := prodOps(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	sum := Summary(prod, "v1", "n1")
	if len(sum.Checks) != len(productionChecks(prod)) {
		t.Errorf("checks %d vs %d", len(sum.Checks), len(productionChecks(prod)))
	}
	seen := map[string]bool{}
	for i, c := range sum.Checks {
		if c.ID != productionChecks(prod)[i].ID || seen[c.ID] || c.Detail == "" {
			t.Errorf("check %d = %+v", i, c)
		}
		seen[c.ID] = true
		if !c.Passed || !c.Applicable {
			t.Errorf("production check %s = %+v", c.ID, c)
		}
	}
	dev, _, err := Load(envOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range Summary(dev, "v1", "n1").Checks {
		if c.Applicable {
			t.Errorf("development check %s should not be applicable", c.ID)
		}
	}
}

// AC-OPS-10-1、10-5：摘要里没有密钥的值和哈希，只有 kid。
func TestSummaryHasNoSecrets(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{
		"IM_JWT_KEYS": "k1:" + strongJWT + ",k2:" + strongJWT + "x", "IM_JWT_ACTIVE_KID": "k2", "IM_ADMIN_KEY": strongAdmin,
		"IM_OPS_PUBKEYS": "ops1:" + opsKeyB64,
	}))
	if err != nil {
		t.Fatal(err)
	}
	sum := Summary(cfg, "v1", "n1")
	b, _ := json.Marshal(sum)
	for _, secret := range []string{strongJWT, strongAdmin} {
		h := sha256.Sum256([]byte(secret))
		for _, leak := range []string{secret, hex.EncodeToString(h[:])[:16], base64.StdEncoding.EncodeToString([]byte(secret))} {
			if strings.Contains(string(b), leak) {
				t.Errorf("summary leaks %q", leak)
			}
		}
	}
	if !slices.Equal(sum.Keys.JWTKids, []string{"k1", "k2"}) || sum.Keys.JWTActiveKid != "k2" || !slices.Equal(sum.Keys.OpsKids, []string{"ops1"}) {
		t.Errorf("keys = %+v", sum.Keys)
	}
}

const strongQuarantine = "quarantine-key-for-config-tests-0123456789"

// AC-ADM-32-4：生产环境没配、太短、开发默认值、和其他密钥相同，都拒绝启动。
func TestProductionQuarantineKey(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte(strongQuarantine))
	base := func() map[string]string {
		return map[string]string{"IM_ENV": "production", "IM_JWT_SECRET": strongJWT, "IM_ALLOWED_ORIGINS": "https://im.example.com", "IM_NODE_ID": "n1", "IM_OPS_ADDR": "off",
			"IM_ADMIN_CALLERS": callersFile(t, `[{"id":"biz1","alg":"hmac-sha256","secret":"`+secret+`","scopes":["token.issue"]}]`)}
	}
	for name, key := range map[string]string{"missing": "", "short": strings.Repeat("k", 31), "dev default": DevQuarantineKey, "same as jwt": strongJWT, "same as caller": strongQuarantine} {
		m := base()
		m["IM_QUARANTINE_KEY"] = key
		_, _, err := Load(envOf(m))
		if err == nil || !strings.Contains(err.Error(), "IM_QUARANTINE_KEY") {
			t.Errorf("%s: err = %v", name, err)
		}
		if err != nil && key != "" && strings.Contains(err.Error(), key) {
			t.Errorf("%s: key leaked into the error", name)
		}
	}
	m := base()
	m["IM_QUARANTINE_KEY"] = strings.Repeat("q", 40)
	cfg, _, err := Load(envOf(m))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.QuarantineDays != 30 {
		t.Errorf("days = %d", cfg.App.QuarantineDays)
	}
	for _, v := range []string{"0", "366", "x"} {
		m["IM_QUARANTINE_DAYS"] = v
		if _, _, err := Load(envOf(m)); err == nil {
			t.Errorf("IM_QUARANTINE_DAYS=%s accepted", v)
		}
	}
}

// PRD-live 第 10 节：IM_PUSH_ENABLED 默认关闭；参数都要正整数；IM_TEST_DISABLE_JOBS 原样带给 app（是否生效看测试模式）。
func TestPushConfig(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Push != (app.PushConfig{}) || len(cfg.App.TestDisableJobs) != 0 {
		t.Errorf("defaults: %+v %v", cfg.App.Push, cfg.App.TestDisableJobs)
	}
	cfg, _, err = Load(envOf(map[string]string{
		"IM_PUSH_ENABLED": "1", "IM_PUSH_ALL_RATE": "1", "IM_PUSH_USER_RATE": "2", "IM_PUSH_SYNC_RATE": "3",
		"IM_PUSH_INBOX_MAX": "4", "IM_PUSH_PURGE_BATCH": "5", "IM_TEST_DISABLE_JOBS": "push_purge, social_purge",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Push != (app.PushConfig{Enabled: true, AllRate: 1, UserRate: 2, SyncRate: 3, InboxMax: 4, PurgeBatch: 5}) {
		t.Errorf("push = %+v", cfg.App.Push)
	}
	if strings.Join(cfg.App.TestDisableJobs, "|") != "push_purge|social_purge" {
		t.Errorf("jobs = %v", cfg.App.TestDisableJobs)
	}
	for _, key := range []string{"IM_PUSH_ALL_RATE", "IM_PUSH_USER_RATE", "IM_PUSH_SYNC_RATE", "IM_PUSH_INBOX_MAX", "IM_PUSH_PURGE_BATCH"} {
		for _, v := range []string{"0", "-1", "x"} {
			if _, _, err := Load(envOf(map[string]string{key: v})); err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s=%s: %v", key, v, err)
			}
		}
	}
	if _, _, err := Load(envOf(map[string]string{"IM_PUSH_ENABLED": "yes"})); err == nil {
		t.Error("IM_PUSH_ENABLED=yes accepted")
	}
}

// TC-CHAT-90-17（AC-CHAT-90-18）、BE-405 第 4 条（R-CQ-01）：测试设施的开关逐项拒绝启动，错误信息指出是哪一项。
func TestProductionRejectsTestOverrides(t *testing.T) {
	base := map[string]string{
		"IM_ENV": "production", "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongAdmin, "IM_ADMIN_LEGACY": "1",
		"IM_QUARANTINE_KEY": "qua-0123456789abcdef0123456789abcdef",
	}
	if _, _, err := Load(envOf(base)); err != nil {
		t.Fatalf("baseline production config must start: %v", err)
	}
	for _, name := range append([]string{"IM_TEST_MODE"}, testOverrideEnvs...) {
		env := maps.Clone(base)
		env[name] = "1"
		_, _, err := Load(envOf(env))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: err = %v, want a refusal naming it", name, err)
		}
	}
	// 设成 0 也算出现：生产配置里不应该有这些项。值不进错误信息。
	env := maps.Clone(base)
	env["IM_APNS_ENDPOINT"] = "https://secret-endpoint.invalid/"
	_, _, err := Load(envOf(env))
	if err == nil || strings.Contains(err.Error(), "secret-endpoint") {
		t.Errorf("err = %v, want a refusal without the value", err)
	}
}

// IM_TEST_DISABLE_AUTH_LOCK 只在测试模式生效（PRD-chat 第 4.5 节）。
func TestDisableAuthLockNeedsTestMode(t *testing.T) {
	cfg, warnings, err := Load(envOf(map[string]string{"IM_TEST_DISABLE_AUTH_LOCK": "1"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Account.TestDisableLock {
		t.Error("the lock must stay on without IM_TEST_MODE=1")
	}
	if !slices.ContainsFunc(warnings, func(w string) bool { return strings.Contains(w, "IM_TEST_DISABLE_AUTH_LOCK") }) {
		t.Errorf("warnings = %v", warnings)
	}
	cfg, _, err = Load(envOf(map[string]string{"IM_TEST_DISABLE_AUTH_LOCK": "1", "IM_TEST_MODE": "1"}))
	if err != nil || !cfg.App.Account.TestDisableLock {
		t.Errorf("test mode: lock disabled = %v, err = %v", cfg.App.Account.TestDisableLock, err)
	}
}
