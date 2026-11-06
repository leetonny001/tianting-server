package e2e

import (
	"net/http"
	"testing"

	"tianting/server/internal/app"
)

// TC-CHAT-90-17（AC-CHAT-90-18）：没有开测试模式时，第 4.5 节的测试接口不存在。生产配置拒绝启动的部分见 config 包的单测。
func TestTC_CHAT_90_17_TestRoutesAbsentWithoutTestMode(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) { c.TestMode = false })
	for _, path := range []string{"/admin/test/faults", "/admin/test/limits/reset", "/admin/test/clock", "/admin/test/account/seed"} {
		code, _ := e.adminPost(path, map[string]any{}, adminKey)
		if code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}
}
