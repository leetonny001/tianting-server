package config

import (
	"strings"
	"testing"
)

// C5 的配置（PRD-chat 第 4.1 节，BE-483 第 2 条；频率和 deleted 上限的变量名见 backend-chat 第 73 条）。

func TestConvSettingsConfig(t *testing.T) {
	cfg, _, err := Load(envOf(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if c := cfg.App.ConvSettings; c.Enabled || c.PinMax != 0 || c.RatePerSec != 0 || c.DeletedMax != 0 {
		t.Errorf("default = %+v (switch off, 0 means the logic default)", c)
	}
	cfg, _, err = Load(envOf(map[string]string{
		"IM_CONV_SETTINGS_ENABLED": "1", "IM_CONV_PIN_MAX": "30", "IM_CONV_SETTINGS_RATE_PER_SEC": "7", "IM_CONV_DELETED_MAX": "100",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c := cfg.App.ConvSettings; !c.Enabled || c.PinMax != 30 || c.RatePerSec != 7 || c.DeletedMax != 100 {
		t.Errorf("parsed = %+v", c)
	}
	for key, bad := range map[string]string{
		"IM_CONV_SETTINGS_ENABLED": "yes", "IM_CONV_PIN_MAX": "501", "IM_CONV_SETTINGS_RATE_PER_SEC": "0", "IM_CONV_DELETED_MAX": "x",
	} {
		if _, _, err := Load(envOf(map[string]string{key: bad})); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%s: err = %v", key, bad, err)
		}
	}
}
