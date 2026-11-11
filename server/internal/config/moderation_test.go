package config

import (
	"encoding/base64"
	"maps"
	"slices"
	"strings"
	"testing"
)

// C6 的配置和启动检查（PRD-chat 第 4.1 节，BE-492 第 2、3 条）。测试密钥一看就是假的（明文 fake-report-key-for-tests-000000）。
const (
	fakeReportKey  = "ZmFrZS1yZXBvcnQta2V5LWZvci10ZXN0cy0wMDAwMDA="
	fakeReportKey2 = "ZmFrZS1yZXBvcnQta2V5LWZvci10ZXN0cy0xMTExMTE="
)

func TestModerationConfigDefaults(t *testing.T) {
	cfg, _, err := Load(envOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	if m := cfg.App.Moderation; m.Enabled || m.ClientCustomOff || m.ReportDaily != 0 || len(cfg.App.ReportKey) != 0 {
		t.Errorf("development defaults: %+v (switch off, custom on, 0 means the logic default)", m)
	}
	cfg, _, err = Load(envOf(map[string]string{
		"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": fakeReportKey, "IM_REPORT_DAILY": "5", "IM_REPORT_EVIDENCE_DAYS": "30", "IM_MOD_HITS_DAYS": "7",
		"IM_CLIENT_CUSTOM_MSG": "0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if m := cfg.App.Moderation; !m.Enabled || !m.ClientCustomOff || m.ReportDaily != 5 || m.EvidenceDays != 30 || m.HitsDays != 7 ||
		string(cfg.App.ReportKey) != "fake-report-key-for-tests-000000" {
		t.Errorf("parsed = %+v", m)
	}
	for key, bad := range map[string]string{"IM_MOD_ENABLED": "yes", "IM_REPORT_DAILY": "0", "IM_REPORT_EVIDENCE_DAYS": "x", "IM_MOD_HITS_DAYS": "3651", "IM_CLIENT_CUSTOM_MSG": "on"} {
		if _, _, err := Load(envOf(map[string]string{key: bad})); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%s: err = %v", key, bad, err)
		}
	}
	// IM_CLIENT_CUSTOM_MSG 生产默认关（第 4.1 节），不跟 IM_MOD_ENABLED 走（backend-chat 第 105 条）。
	cfg, _, err = Load(envOf(prodBase()))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.App.Moderation.ClientCustomOff || cfg.App.Moderation.Enabled {
		t.Errorf("production default: %+v", cfg.App.Moderation)
	}
}

// TC-MOD-01-10 ③（AC-MOD-01-10，backend-chat 第 99 条）：开关打开时缺 IM_REPORT_KEY、长度不对、和隔离密钥或 JWT 密钥相同，
// 任何环境都拒绝启动；错误信息不含密钥。开关关闭时设置了也校验格式。
func TestTC_MOD_01_10_ReportKeyChecks(t *testing.T) {
	short := "c2hvcnQta2V5" // "short-key"
	sameAsQuarantine := strongQuarantine
	for name, env := range map[string]map[string]string{
		"缺 IM_REPORT_KEY":             {"IM_MOD_ENABLED": "1"},
		"长度不对":                        {"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": short},
		"不是 base64":                   {"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": "not base64 !!!"},
		"等于开发默认的隔离密钥":                 {"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": DevQuarantineKey[:32]},
		"等于 IM_QUARANTINE_KEY（解码后）":   {"IM_MOD_ENABLED": "1", "IM_QUARANTINE_KEY": "fake-report-key-for-tests-000000", "IM_REPORT_KEY": fakeReportKey},
		"等于 IM_QUARANTINE_KEY（原文）":    {"IM_MOD_ENABLED": "1", "IM_QUARANTINE_KEY": fakeReportKey, "IM_REPORT_KEY": fakeReportKey},
		"等于 IM_JWT_SECRET":            {"IM_MOD_ENABLED": "1", "IM_JWT_SECRET": "fake-report-key-for-tests-000000", "IM_REPORT_KEY": fakeReportKey},
		"开关关闭、长度不对":                   {"IM_REPORT_KEY": short},
		"生产、等于 IM_QUARANTINE_KEY 的原文": merged(prodBase(), map[string]string{"IM_MOD_ENABLED": "1", "IM_V13_ADMIN_FEATURES": "1", "IM_PRIVACY_NOTICE_VERSION": "v1", "IM_REPORT_KEY": sameAsQuarantine}),
	} {
		_, _, err := Load(envOf(env))
		if err == nil || !strings.Contains(err.Error(), "IM_REPORT_KEY") {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		for _, secret := range []string{fakeReportKey, "fake-report-key-for-tests", short, DevQuarantineKey[:32], sameAsQuarantine} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("%s: the error prints a key: %v", name, err)
			}
		}
	}
	// 等于开发默认隔离密钥的那一条要真的命中"相同"：开发环境 IM_QUARANTINE_KEY 没设置时用 DevQuarantineKey。
	if _, _, err := Load(envOf(map[string]string{"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": b64(DevQuarantineKey[:32]), "IM_QUARANTINE_KEY": DevQuarantineKey[:32]})); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Errorf("same as the quarantine key: %v", err)
	}
	if _, _, err := Load(envOf(map[string]string{"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": fakeReportKey})); err != nil {
		t.Errorf("valid key: %v", err)
	}
	// URL 安全、无填充的写法也接受。
	if cfg, _, err := Load(envOf(map[string]string{"IM_REPORT_KEY": strings.TrimRight(fakeReportKey2, "=")})); err != nil || len(cfg.App.ReportKey) != 32 {
		t.Errorf("unpadded key: %v", err)
	}
}

// TC-MOD-01-10 ①②（AC-MOD-01-10）：生产 + IM_MOD_ENABLED=1 + V13 关拒绝启动，错误信息指出缺 IM_V13_ADMIN_FEATURES；打开 V13 后通过。
// TC-ACC-10-05（AC-ACC-10-5）：生产 + 开关开 + 隐私说明版本为空拒绝启动；补上后通过。
func TestTC_MOD_01_10_ProductionNeedsV13AndNotice(t *testing.T) {
	env := merged(prodBase(), map[string]string{"IM_MOD_ENABLED": "1", "IM_REPORT_KEY": fakeReportKey, "IM_PRIVACY_NOTICE_VERSION": "2026-11"})
	_, _, err := Load(envOf(env))
	if err == nil || !strings.Contains(err.Error(), "IM_MOD_ENABLED=1 requires IM_V13_ADMIN_FEATURES=1") || strings.Contains(err.Error(), fakeReportKey) {
		t.Fatalf("V13 off: %v", err)
	}
	env["IM_V13_ADMIN_FEATURES"] = "1"
	cfg, _, err := Load(envOf(env))
	if err != nil {
		t.Fatalf("V13 on: %v", err)
	}
	if c := checkByID(Summary(cfg, "v", "n1").Checks, "moderation"); !c.Passed || !strings.Contains(c.Detail, "IM_REPORT_KEY：已设置") || strings.Contains(c.Detail, fakeReportKey) {
		t.Errorf("moderation check = %+v", c)
	}
	delete(env, "IM_PRIVACY_NOTICE_VERSION")
	if _, _, err := Load(envOf(env)); err == nil || !strings.Contains(err.Error(), "IM_MOD_ENABLED=1 requires IM_PRIVACY_NOTICE_VERSION") {
		t.Fatalf("no privacy notice: %v", err)
	}
	// 开关关闭时两项都不要求。
	if _, _, err := Load(envOf(prodBase())); err != nil {
		t.Fatalf("switch off: %v", err)
	}
}

// TC-ACC-01-22（AC-ACC-01-9）：生产 open 模式逐项检查，错误信息分别指出缺的那一条；四项都满足时通过 config（词库非空在 app.New 里读库，
// 见 e2e 的同名用例）。AC-ACC-01-19 的输入不满足这些条件，仍是拒绝（backend-chat 第 106 条）。
func TestTC_ACC_01_22_ProductionOpenChecks(t *testing.T) {
	full := merged(prodBase(), map[string]string{
		"IM_REGISTER_MODE": "open", "IM_MOD_ENABLED": "1", "IM_V13_ADMIN_FEATURES": "1", "IM_PUBLIC_OPEN_ACK": "1",
		"IM_REPORT_KEY": fakeReportKey, "IM_PRIVACY_NOTICE_VERSION": "2026-11",
	})
	cfg, _, err := Load(envOf(full))
	if err != nil {
		t.Fatalf("all four: %v", err)
	}
	if !cfg.App.RequireWords {
		t.Error("open mode in production must check the word list at startup")
	}
	for drop, want := range map[string]string{
		"IM_MOD_ENABLED":        "IM_REGISTER_MODE=open requires IM_MOD_ENABLED=1",
		"IM_V13_ADMIN_FEATURES": "IM_REGISTER_MODE=open requires IM_V13_ADMIN_FEATURES=1",
		"IM_PUBLIC_OPEN_ACK":    "IM_REGISTER_MODE=open requires IM_PUBLIC_OPEN_ACK=1",
	} {
		env := maps.Clone(full)
		delete(env, drop)
		_, _, err := Load(envOf(env))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("without %s: %v", drop, err)
			continue
		}
		for other, msg := range map[string]string{"IM_MOD_ENABLED": "requires IM_MOD_ENABLED=1", "IM_V13_ADMIN_FEATURES": "open requires IM_V13", "IM_PUBLIC_OPEN_ACK": "IM_PUBLIC_OPEN_ACK=1"} {
			if other != drop && strings.Contains(err.Error(), msg) {
				t.Errorf("without %s the error also blames %s: %v", drop, other, err)
			}
		}
		if strings.Contains(err.Error(), fakeReportKey) || strings.Contains(err.Error(), strongJWT) {
			t.Errorf("the error prints a key: %v", err)
		}
	}
	// 开发环境不做这些检查，也不读库。
	cfg, _, err = Load(envOf(map[string]string{"IM_REGISTER_MODE": "open"}))
	if err != nil || cfg.App.RequireWords {
		t.Errorf("development open: %v, requireWords=%v", err, cfg.App.RequireWords)
	}
}

// TC-MOD-02-15（AC-MOD-02-15）：生产 IM_CLIENT_CUSTOM_MSG=1 能启动，配置检查的 moderation 项告警；默认关，没有这条告警。
func TestTC_MOD_02_15_CustomWarning(t *testing.T) {
	cfg, _, err := Load(envOf(merged(prodBase(), map[string]string{"IM_CLIENT_CUSTOM_MSG": "1"})))
	if err != nil {
		t.Fatal(err)
	}
	if c := checkByID(Summary(cfg, "v", "n1").Checks, "moderation"); c.Passed || !c.Applicable || !strings.Contains(c.Detail, "告警：IM_CLIENT_CUSTOM_MSG=1") {
		t.Errorf("custom on: %+v", c)
	}
	cfg, _, err = Load(envOf(prodBase()))
	if err != nil {
		t.Fatal(err)
	}
	if c := checkByID(Summary(cfg, "v", "n1").Checks, "moderation"); !c.Passed || strings.Contains(c.Detail, "告警") || !strings.Contains(c.Detail, "IM_CLIENT_CUSTOM_MSG：关闭") {
		t.Errorf("default: %+v", c)
	}
}

// 第 4.5 节：IM_TEST_WORDS_STARTUP_FAIL 只在测试模式生效，生产配置里出现就拒绝启动（AC-CHAT-90-18 的做法）。
func TestWordsStartupFaultIsTestOnly(t *testing.T) {
	cfg, warnings, err := Load(envOf(map[string]string{"IM_TEST_WORDS_STARTUP_FAIL": "1"}))
	if err != nil || cfg.App.TestWordsStartupFail || !slices.ContainsFunc(warnings, func(w string) bool { return strings.Contains(w, "IM_TEST_WORDS_STARTUP_FAIL") }) {
		t.Errorf("without test mode: %v %v %v", err, cfg.App.TestWordsStartupFail, warnings)
	}
	cfg, _, err = Load(envOf(map[string]string{"IM_TEST_WORDS_STARTUP_FAIL": "1", "IM_TEST_MODE": "1"}))
	if err != nil || !cfg.App.TestWordsStartupFail {
		t.Errorf("test mode: %v %v", err, cfg.App.TestWordsStartupFail)
	}
	if _, _, err := Load(envOf(merged(prodBase(), map[string]string{"IM_TEST_WORDS_STARTUP_FAIL": "1"}))); err == nil || !strings.Contains(err.Error(), "IM_TEST_WORDS_STARTUP_FAIL must not be set in production") {
		t.Errorf("production: %v", err)
	}
}

func prodBase() map[string]string {
	return map[string]string{
		"IM_ENV": "production", "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongAdmin, "IM_ADMIN_LEGACY": "1",
		"IM_QUARANTINE_KEY": strongQuarantine, "IM_NODE_ID": "n1", "IM_OPS_ADDR": "off",
	}
}

func merged(a, b map[string]string) map[string]string {
	out := maps.Clone(a)
	maps.Copy(out, b)
	return out
}

func checkByID(cs []Check, id string) Check {
	for _, c := range cs {
		if c.ID == id {
			return c
		}
	}
	return Check{ID: "missing " + id}
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
