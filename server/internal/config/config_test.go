package config

import (
	"strings"
	"testing"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

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
	if len(warnings) != 2 {
		t.Errorf("warnings = %v, want 2 (default keys)", warnings)
	}
	if cfg.App.Gateway.MaxConnsPerUser != 10 || len(cfg.App.Gateway.AllowedOrigins) == 0 {
		t.Errorf("gateway defaults = %+v", cfg.App.Gateway)
	}
}

// ARCH S4：生产环境的每一种不安全配置都拒绝启动，并在一条错误里全部列出。
func TestProductionRejectsUnsafeConfig(t *testing.T) {
	_, _, err := Load(envOf(map[string]string{
		"IM_ENV":             "production",
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
	if err == nil || !strings.Contains(err.Error(), "IM_JWT_SECRET is required") || !strings.Contains(err.Error(), "IM_ADMIN_KEY is required") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = Load(envOf(map[string]string{"IM_ENV": "production", "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongJWT}))
	if err == nil || !strings.Contains(err.Error(), "must be different") {
		t.Fatalf("same secret for jwt and admin: err = %v", err)
	}
}

func TestProductionValid(t *testing.T) {
	cfg, warnings, err := Load(envOf(map[string]string{
		"IM_ENV":             "production",
		"IM_JWT_KEYS":        "k1:" + strongJWT + ",k2:" + strongJWT + "x",
		"IM_JWT_ACTIVE_KID":  "k2",
		"IM_ADMIN_KEY":       strongAdmin,
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
