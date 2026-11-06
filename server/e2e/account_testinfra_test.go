package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
)

// TC-CHAT-90-17（AC-CHAT-90-18）：没有开测试模式时，第 4.5 节的测试接口不存在。生产配置拒绝启动的部分见 config 包的单测。
func TestTC_CHAT_90_17_TestRoutesAbsentWithoutTestMode(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) { c.TestMode = false })
	for _, path := range []string{"/admin/test/faults", "/admin/test/limits/reset", "/admin/test/clock"} {
		code, _ := e.adminPost(path, map[string]any{}, adminKey)
		if code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}
}

// TC-ACC-07-04（AC-ACC-07-4）：三种模式下 GET /v1/config 和配置一致，不需要登录，没有任何密钥。
// features.files 随 C2；IM_FILES_ENABLED 这一步在 BE-431 补。
func TestTC_ACC_07_04_PublicConfig(t *testing.T) {
	const refreshKey = "e2e-refresh-key-0123456789abcdef"
	for _, tc := range []struct {
		mode string
		demo bool
	}{{"closed", false}, {"invite", true}, {"open", false}} {
		e := newAcctEnv(t, func(c *app.Config) {
			c.Account.RegisterMode = tc.mode
			c.DemoLogin = tc.demo
			c.Account.PrivacyVersion, c.Account.PrivacyURL = "v1", "https://example.invalid/privacy"
		})
		code, b := e.get("/v1/config", "")
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.mode, code, b)
		}
		var got account.ConfigResponse
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got.RegisterMode != tc.mode || got.DemoLogin != tc.demo || got.Features.Files {
			t.Errorf("%s: %s", tc.mode, b)
		}
		if got.Limits.PasswordMaxLen != 128 || got.Limits.UsernameMinLen != 4 || got.PrivacyNotice == nil || got.PrivacyNotice.Version != "v1" {
			t.Errorf("%s: limits or privacy: %s", tc.mode, b)
		}
		validateHTTP(t, "ConfigResponse", b)
		for _, secret := range []string{refreshKey, "e2e-secret", adminKey, "secret", "key"} {
			if strings.Contains(strings.ToLower(string(b)), secret) {
				t.Errorf("%s: response contains %q: %s", tc.mode, secret, b)
			}
		}
	}
	// 没有配隐私说明时不出现（客户端隐藏入口，F-ACC-10 异常）。
	if _, b := newAcctEnv(t, nil).get("/v1/config", ""); strings.Contains(string(b), "privacyNotice") {
		t.Errorf("privacyNotice must be absent: %s", b)
	}
}

// BE-405：测试设施的接口。/admin/test/faults 的 argonBlock 可以打开和查询；limits/reset scope=auth 清计数。
func TestAccountTestFacilities(t *testing.T) {
	e := newAcctEnv(t, nil)
	code, b := e.adminPost("/admin/test/faults", map[string]any{"argonBlock": true}, adminKey)
	if code != http.StatusOK || !strings.Contains(string(b), `"argonBlocked":0`) {
		t.Errorf("%d %s", code, b)
	}
	if code, b := e.adminPost("/admin/test/faults", map[string]any{"argonBlock": false}, adminKey); code != http.StatusOK {
		t.Errorf("%d %s", code, b)
	}
	if code, b := e.adminPost("/admin/test/limits/reset", map[string]any{"scope": "auth"}, adminKey); code != http.StatusOK {
		t.Errorf("%d %s", code, b)
	}
}
