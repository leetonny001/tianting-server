//go:build imhooks

package e2e

// F-ACC-04 规则 8 的并发交错（RS4-06、RS5-02）：用账号并发钩子确定性构造，请求停在 Argon2 之后、提交之前。
// recover 事务内部的交错（AC-ACC-04-29）只有 Postgres 下可达，见 TestTC_ACC_04_27_NotExecuted。

import (
	"net/http"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/protocol"
	"tianting/server/internal/testhook"
)

// pausedCall 在 goroutine 里发请求，等它停到钩子上。
type pausedCall struct {
	p    *testhook.Pause
	done chan struct{}
	code int
	body []byte
}

func pause(t *testing.T, user string, f func() (int, []byte)) *pausedCall {
	t.Helper()
	c := &pausedCall{p: testhook.Arm(account.UsernameHash(user)), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		c.code, c.body = f()
	}()
	select {
	case <-c.p.Reached():
	case <-time.After(5 * time.Second):
		t.Fatal("the request did not reach the hook")
	}
	return c
}

func (c *pausedCall) release() {
	c.p.Release()
	<-c.done
}

// TC-ACC-04-17（AC-ACC-04-18，RS4-06 甲）：登录在 Argon2 后暂停，recover 提交后放行：登录 401，不计失败，账号只有受害者的会话。
func TestHookLoginVsRecover(t *testing.T) {
	e := openEnv(t, nil)
	v := e.regAcked("victim_1")
	const ip = "203.0.113.40"
	x := pause(t, "victim_1", func() (int, []byte) { return e.login("victim_1", goodPassword, ip) })
	e.mustRecover("victim_1", v.RecoveryCode, newPassword)
	x.release()
	wantErr(t, "login", x.code, x.body, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	if n := e.count(`SELECT COUNT(*) FROM auth_throttle WHERE throttle_key LIKE 'pw:%'`); n != 0 {
		t.Errorf("must not count as a failure: %d", n)
	}
	if n := e.liveSessions("victim_1"); n != 1 {
		t.Errorf("sessions %d", n)
	}
}

// TC-ACC-04-18（AC-ACC-04-19，RS4-06 乙）：改密在 Argon2 后暂停，recover 提交后放行：改密 401，没有恢复码；V 的密码有效，X 的无效。
func TestHookPasswordVsRecover(t *testing.T) {
	e := openEnv(t, nil)
	v := e.regAcked("victim_1")
	xs := e.mustLogin("victim_1", goodPassword, "ios")
	x := pause(t, "victim_1", func() (int, []byte) { return e.changePassword(xs.AccessToken, goodPassword, "Xx-pass-2026-a!") })
	gens := e.count(`SELECT MAX(gen) FROM recovery_codes WHERE user_id = 'victim_1'`)
	e.mustRecover("victim_1", v.RecoveryCode, newPassword)
	x.release()
	wantErr(t, "password", x.code, x.body, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	if n := e.count(`SELECT MAX(gen) FROM recovery_codes WHERE user_id = 'victim_1'`); n != gens+1 {
		t.Errorf("generations %d -> %d: the attacker got a code", gens, n)
	}
	e.mustLogin("victim_1", newPassword, "ios")
	if c, _ := e.login("victim_1", "Xx-pass-2026-a!", e.freshIP()); c != http.StatusUnauthorized {
		t.Errorf("attacker password: %d", c)
	}
}

// RS7-01：logout-others 不改 accounts，也要先锁行、持锁校验：放行后 401，不留新会话。
func TestHookLogoutOthersVsRecover(t *testing.T) {
	e := openEnv(t, nil)
	v := e.regAcked("victim_1")
	xs := e.mustLogin("victim_1", goodPassword, "ios")
	x := pause(t, "victim_1", func() (int, []byte) {
		return e.bearerPost("/v1/account/logout-others", xs.AccessToken, e.freshIP(), map[string]any{"password": goodPassword})
	})
	e.mustRecover("victim_1", v.RecoveryCode, newPassword)
	x.release()
	wantErr(t, "logout-others", x.code, x.body, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	if n := e.liveSessions("victim_1"); n != 1 {
		t.Errorf("sessions %d", n)
	}
}

// TC-ACC-04-23（AC-ACC-04-24，RS5-02 乙、丙）：用户名 + IP 已锁定、同一用户名正在算哈希、其他用户占住改密专用位：
// 错误恢复码 429，正确恢复码成功。
func TestHookLockedAndHashingRecover(t *testing.T) {
	e := openEnv(t, nil)
	v := e.regAcked("victim_1")
	other := e.regAcked("other_1")
	const ip = "203.0.113.41"
	for range 5 {
		e.login("victim_1", "wrong-password-1", ip)
	}
	if c, b := e.login("victim_1", goodPassword, ip); c != http.StatusTooManyRequests {
		t.Fatalf("not locked: %d %s", c, b)
	}
	// 同一用户名正在算哈希：从另一个 IP 登录，停在钩子上，单哈希一直占着。
	hashing := pause(t, "victim_1", func() (int, []byte) { return e.login("victim_1", goodPassword, "203.0.113.42") })
	// 改密专用位被占：其他用户的改密停在钩子之前算不到这里，用钩子占住用户名之后位已释放，所以这里占位用循环请求。
	stop := make(chan struct{})
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		pw, next := goodPassword, "Other-pass-2026-a!"
		token := other.AccessToken
		for {
			select {
			case <-stop:
				return
			default:
			}
			c, b := e.changePassword(token, pw, next)
			if c == http.StatusOK {
				var r authResp
				mustJSON(t, b, &r)
				token, pw, next = r.AccessToken, next, pw
			}
		}
	}()
	c, b := e.recover("victim_1", "AAAA-BBBB", newPassword, ip)
	wantErr(t, "wrong code", c, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	c, b = e.recover("victim_1", v.RecoveryCode, newPassword, ip)
	close(stop)
	<-loopDone
	hashing.release()
	if c != http.StatusOK {
		t.Fatalf("right code: %d %s", c, b)
	}
}
