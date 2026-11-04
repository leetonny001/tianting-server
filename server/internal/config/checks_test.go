package config

import (
	"strings"
	"testing"
)

// 检查项的说明要写实际情况：开发环境配了 * 时不能写"不含 *"。
func TestOriginsCheckDetailIsTruthful(t *testing.T) {
	for _, tc := range []struct {
		origins []string
		want    string
	}{{[]string{"*"}, "1 项，含 *"}, {[]string{"https://a", "https://b"}, "2 项，不含 *"}} {
		var cfg Server
		cfg.App.Gateway.AllowedOrigins = tc.origins
		found := false
		for _, c := range productionChecks(cfg) {
			if c.ID == "origins_no_wildcard" {
				found = true
				if !strings.Contains(c.Detail, tc.want) {
					t.Errorf("%v: detail = %q", tc.origins, c.Detail)
				}
			}
		}
		if !found {
			t.Fatal("check not found")
		}
	}
}
