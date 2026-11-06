//go:build imhooks

package e2e

// 账号并发钩子（PRD-chat 第 4.5 节，RS4-06）：只有 -tags imhooks 才编译。internal/archtest 的 TestHookedSuitePasses 在普通的 go test 里跑它。

import (
	"net/http"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/protocol"
	"tianting/server/internal/testhook"
)

// F-ACC-04 规则 8：登录停在 Argon2 之后、提交之前，这时凭证版本变了（recover 或改密提交）：放行后 401 / 4018，不计入失败次数，没有留下会话。
// 这是 TC-ACC-04-17（AC-ACC-04-18）的服务端基础，完整序列要 recover 接口，随 BE-418。
func TestHookLoginAfterCredentialsChange(t *testing.T) {
	e := openEnv(t, nil)
	e.register("alice_1", nil)
	sessions := e.count(`SELECT COUNT(*) FROM device_sessions`)
	p := testhook.Arm(account.UsernameHash("alice_1"))
	const ip = "203.0.113.5"
	done := make(chan struct{})
	var code int
	var body []byte
	go func() {
		defer close(done)
		code, body = e.login("alice_1", goodPassword, ip)
	}()
	select {
	case <-p.Reached():
	case <-time.After(5 * time.Second):
		t.Fatal("the login did not reach the hook")
	}
	e.rawExec(`UPDATE accounts SET credentials_version = credentials_version + 1 WHERE user_id = 'alice_1'`)
	p.Release()
	<-done
	wantErr(t, "paused login", code, body, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	if n := e.count(`SELECT COUNT(*) FROM device_sessions`); n != sessions {
		t.Errorf("sessions %d, was %d", n, sessions)
	}
	if n := e.count(`SELECT COUNT(*) FROM auth_throttle WHERE throttle_key LIKE 'pw:%'`); n != 0 {
		t.Errorf("a credentials version mismatch must not count as a failure: %d", n)
	}
}

// 没有变化时放行后正常成功。
func TestHookLoginWithoutChange(t *testing.T) {
	e := openEnv(t, nil)
	e.register("alice_1", nil)
	p := testhook.Arm(account.UsernameHash("alice_1"))
	done := make(chan int)
	go func() {
		code, _ := e.login("alice_1", goodPassword, "203.0.113.5")
		done <- code
	}()
	<-p.Reached()
	p.Release()
	if code := <-done; code != http.StatusOK {
		t.Errorf("code %d", code)
	}
}
