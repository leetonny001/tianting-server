package config

import (
	"maps"
	"strings"
	"testing"
	"time"
)

// C4 的配置（PRD-chat 第 4.1 节，BE-471）。

func prodC4(extra map[string]string) map[string]string {
	env := map[string]string{
		"IM_ENV": "production", "IM_JWT_SECRET": strongJWT, "IM_ADMIN_KEY": strongAdmin, "IM_ADMIN_LEGACY": "1",
		"IM_QUARANTINE_KEY": "qua-0123456789abcdef0123456789abcdef",
	}
	maps.Copy(env, extra)
	return env
}

// TC-ACC-10-04（AC-ACC-10-4）：生产、IM_MSG_ACTIONS_ENABLED=1、IM_PRIVACY_NOTICE_VERSION 为空拒绝启动，错误信息指出缺隐私说明、不含密钥；
// 补上版本正常启动；版本为空但开关关闭正常启动（C6 的同类检查是 AC-ACC-10-5）。
func TestTC_ACC_10_04_PrivacyNoticeRequired(t *testing.T) {
	_, _, err := Load(envOf(prodC4(map[string]string{"IM_MSG_ACTIONS_ENABLED": "1"})))
	if err == nil || !strings.Contains(err.Error(), "IM_PRIVACY_NOTICE_VERSION") {
		t.Fatalf("err = %v", err)
	}
	for _, secret := range []string{strongJWT, strongAdmin, "qua-0123456789abcdef0123456789abcdef"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error leaks a secret: %v", err)
		}
	}
	if _, _, err := Load(envOf(prodC4(map[string]string{"IM_MSG_ACTIONS_ENABLED": "1", "IM_PRIVACY_NOTICE_VERSION": "2026-11"}))); err != nil {
		t.Errorf("with a version: %v", err)
	}
	if _, _, err := Load(envOf(prodC4(nil))); err != nil {
		t.Errorf("switch off: %v", err)
	}
}

// TC-RCL-01-10 ②（AC-RCL-01-10，backend-chat 第 54 条）：开关打开但没有设置 IM_QUARANTINE_KEY，任何环境都启动失败，不回退到开发默认值。
func TestTC_RCL_01_10_QuarantineKeyRequired(t *testing.T) {
	for _, env := range []map[string]string{
		{"IM_MSG_ACTIONS_ENABLED": "1"},
		{"IM_MSG_ACTIONS_ENABLED": "1", "IM_TEST_MODE": "1"},
		prodC4(map[string]string{"IM_MSG_ACTIONS_ENABLED": "1", "IM_PRIVACY_NOTICE_VERSION": "v1", "IM_QUARANTINE_KEY": "-"}),
	} {
		_, _, err := Load(envOf(env))
		if err == nil || !strings.Contains(err.Error(), "IM_MSG_ACTIONS_ENABLED=1 requires IM_QUARANTINE_KEY") {
			t.Errorf("%v: err = %v", env, err)
		}
	}
	// 开关关闭：开发环境照旧回退到开发默认值（第二批的行为不变）。
	cfg, _, err := Load(envOf(map[string]string{}))
	if err != nil || cfg.App.QuarantineKey != DevQuarantineKey || cfg.App.MsgActions.Enabled {
		t.Errorf("default: %v %+v", err, cfg.App.MsgActions)
	}
	cfg, _, err = Load(envOf(map[string]string{"IM_MSG_ACTIONS_ENABLED": "1", "IM_QUARANTINE_KEY": "dev-quarantine-0123456789abcdef0123"}))
	if err != nil || !cfg.App.MsgActions.Enabled {
		t.Errorf("enabled with a key: %v", err)
	}
}

// 第 4.1 节的变量和第 53 条的两个频率；非法值拒绝启动。
func TestMsgActionsConfig(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{
		"IM_RECALL_WINDOW_SEC": "60", "IM_GROUP_RECALL_WINDOW_SEC": "86400", "IM_MENTION_MAX": "5",
		"IM_MENTION_ALL_HOURLY": "7", "IM_MENTION_ALL_DAILY": "9", "IM_RECALL_RATE_PER_MIN": "3", "IM_MEMBER_SEARCH_RATE_PER_MIN": "4",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m := cfg.App.MsgActions
	if m.RecallWindow != time.Minute || m.GroupRecallWindow != 24*time.Hour || m.MentionMax != 5 || m.MentionAllHourly != 7 ||
		m.MentionAllDaily != 9 || m.RecallRatePerMin != 3 || m.MemberSearchRatePerMin != 4 || m.Enabled {
		t.Errorf("config = %+v", m)
	}
	if d := m.WithDefaults(); d.RecallWindow != time.Minute {
		t.Errorf("WithDefaults overrides a set value: %+v", d)
	}
	for _, kv := range [][2]string{{"IM_RECALL_WINDOW_SEC", "0"}, {"IM_MENTION_MAX", "x"}, {"IM_MENTION_ALL_DAILY", "-1"}, {"IM_MSG_ACTIONS_ENABLED", "yes"}} {
		if _, _, err := Load(envOf(map[string]string{kv[0]: kv[1]})); err == nil || !strings.Contains(err.Error(), kv[0]) {
			t.Errorf("%s=%s: err = %v", kv[0], kv[1], err)
		}
	}
}

// TC-CHAT-90-34（AC-CHAT-90-17 的配置检查部分）：配置检查项 msg_actions 有开关状态、隔离密钥已设置、隐私说明版本，不含密钥。
func TestTC_CHAT_90_34_MsgActionsCheck(t *testing.T) {
	cfg, _, err := Load(envOf(prodC4(map[string]string{"IM_MSG_ACTIONS_ENABLED": "1", "IM_PRIVACY_NOTICE_VERSION": "2026-11"})))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range Summary(cfg, "v", "n1").Checks {
		if c.ID != "msg_actions" {
			continue
		}
		found = true
		if !c.Passed || !strings.Contains(c.Detail, "IM_MSG_ACTIONS_ENABLED：开启") || !strings.Contains(c.Detail, "IM_QUARANTINE_KEY：已设置") ||
			!strings.Contains(c.Detail, "IM_PRIVACY_NOTICE_VERSION：2026-11") || strings.Contains(c.Detail, "qua-0123") {
			t.Errorf("check = %+v", c)
		}
	}
	if !found {
		t.Error("no msg_actions check")
	}
}
