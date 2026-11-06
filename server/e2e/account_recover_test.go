package e2e

// F-ACC-05 的后端用例和评审里的找回攻击序列（BE-418）。

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func recoverBody(user, code, pw string) map[string]any {
	return map[string]any{"username": user, "recoveryCode": code, "newPassword": pw, "platform": testPlatform}
}

func (e *acctEnv) recover(user, code, pw, ip string) (int, []byte) {
	e.t.Helper()
	if ip == "" {
		ip = e.freshIP()
	}
	c, b, _ := e.post("/v1/account/recover", ip, recoverBody(user, code, pw))
	return c, b
}

func (e *acctEnv) mustRecover(user, code, pw string) authResp {
	e.t.Helper()
	c, b := e.recover(user, code, pw, "")
	if c != http.StatusOK {
		e.t.Fatalf("recover: %d %s", c, b)
	}
	validateHTTP(e.t, "AuthResponse", b)
	var r authResp
	mustJSON(e.t, b, &r)
	return r
}

func (e *acctEnv) wantRecoverFail(label, user, code, pw string) {
	e.t.Helper()
	c, b := e.recover(user, code, pw, "")
	wantErr(e.t, label, c, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
}

// regAcked 注册并确认恢复码。
func (e *acctEnv) regAcked(user string) authResp {
	e.t.Helper()
	r := e.register(user, nil)
	e.ack(r.AccessToken, r.RecoveryCodeGen)
	return r
}

// liveSessions 是账号没有吊销的会话数。
func (e *acctEnv) liveSessions(user string) int {
	e.t.Helper()
	return e.count(`SELECT COUNT(*) FROM device_sessions WHERE user_id = ? AND revoked_at = 0`, user)
}

// TC-ACC-05-01（AC-ACC-05-1）：正确恢复码找回：新密码可用，旧设备下线，响应带新恢复码，旧码再用 401。
func TestTC_ACC_05_01_Recover(t *testing.T) {
	e := openEnv(t, nil)
	r := e.regAcked("alice_1")
	c := e.authOK("d1", r.AccessToken, testPlatform)
	got := e.mustRecover("alice_1", r.RecoveryCode, newPassword)
	if got.RecoveryCode == "" || got.RecoveryCodeGen != 2 || sidOf(got.RefreshToken) == sidOf(r.RefreshToken) {
		t.Errorf("%+v", got)
	}
	waitKicked(c, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	e.authOK("new", got.AccessToken, testPlatform)
	e.wantRefreshErr("old", r.RefreshToken, protocol.ReasonCredentialsChanged)
	e.mustLogin("alice_1", newPassword, "ios")
	if code, _ := e.login("alice_1", goodPassword, e.freshIP()); code != http.StatusUnauthorized {
		t.Errorf("old password %d", code)
	}
	e.wantRecoverFail("used", "alice_1", r.RecoveryCode, "Another-pass-2026!")
}

// TC-ACC-05-02（AC-ACC-05-2）：用户名不存在、恢复码错、已用过：响应体和响应头名集合相同。
func TestTC_ACC_05_02_SameResponse(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.regAcked("real1")
	used := e.regAcked("real2")
	e.mustRecover("real2", used.RecoveryCode, newPassword)
	do := func(user, code string) (int, string, []string) {
		c, b, h := e.post("/v1/account/recover", e.freshIP(), recoverBody(user, code, "Another-pass-2026!"))
		var names []string
		for k := range h {
			if k != "Date" {
				names = append(names, k)
			}
		}
		return c, string(b), names
	}
	_ = r1
	c1, b1, h1 := do("nouser", "AAAA-BBBB")
	c2, b2, h2 := do("real1", "AAAA-BBBB")
	c3, b3, h3 := do("real2", used.RecoveryCode)
	if c1 != 401 || c2 != 401 || c3 != 401 || b1 != b2 || b2 != b3 || len(h1) != len(h2) || len(h2) != len(h3) {
		t.Errorf("%d %s %v; %d %s %v; %d %s %v", c1, b1, h1, c2, b2, h2, c3, b3, h3)
	}
	if !strings.Contains(b1, fmt.Sprint(protocol.CodeAccountCredentials)) {
		t.Error(b1)
	}
}

// TC-ACC-05-03（AC-ACC-05-3，RS5-02）：错误恢复码 5 次后再错 429；正确的不受限。
func TestTC_ACC_05_03_MissLimited(t *testing.T) {
	e := openEnv(t, nil)
	r := e.regAcked("alice_1")
	const ip = "203.0.113.20"
	for i := range 5 {
		c, b := e.recover("alice_1", "AAAA-BBBB", newPassword, ip)
		wantErr(t, fmt.Sprint("miss ", i), c, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	}
	c, b := e.recover("alice_1", "AAAA-BBBB", newPassword, ip)
	wantErr(t, "6th", c, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	c, b = e.recover("alice_1", r.RecoveryCode, newPassword, ip)
	if c != http.StatusOK {
		t.Fatalf("%d %s", c, b)
	}
	// 找回成功清掉这个 IP 的计数，受害者能用新密码登录。
	if c, b := e.login("alice_1", newPassword, ip); c != http.StatusOK {
		t.Errorf("%d %s", c, b)
	}
}

// TC-ACC-05-05（AC-ACC-05-6）：注册响应丢失：登录发新码，注册时的码失效，确认后不再带。
func TestTC_ACC_05_05_RegisterResponseLost(t *testing.T) {
	e := openEnv(t, nil)
	reg := e.register("alice_1", nil)
	l := e.mustLogin("alice_1", goodPassword, "ios")
	if l.RecoveryCode == "" || l.RecoveryCodeGen != 2 {
		t.Fatalf("%+v", l)
	}
	e.wantRecoverFail("register code", "alice_1", reg.RecoveryCode, newPassword)
	e.ack(l.AccessToken, l.RecoveryCodeGen)
	if again := e.mustLogin("alice_1", goodPassword, "ios"); again.RecoveryCode != "" {
		t.Errorf("%+v", again)
	}
}

// TC-ACC-05-07（AC-ACC-05-8）：新密码不合规、账号封禁：恢复码和密码都不变，之后还能用。
func TestTC_ACC_05_07_WeakAndBanned(t *testing.T) {
	e := openEnv(t, nil)
	a := e.regAcked("alice_1")
	c, b := e.recover("alice_1", a.RecoveryCode, "aaaaaaaa", "")
	wantErr(t, "weak", c, b, http.StatusBadRequest, protocol.CodeBadRequest, protocol.ReasonPasswordWeak)
	bb := e.regAcked("bobby_1")
	e.ban("bobby_1")
	c, b = e.recover("bobby_1", bb.RecoveryCode, newPassword, "")
	wantErr(t, "banned", c, b, http.StatusForbidden, protocol.CodeUserBanned, "")
	if n := e.count(`SELECT COUNT(*) FROM recovery_codes WHERE used_at <> 0`); n != 0 {
		t.Errorf("used %d", n)
	}
	e.mustRecover("alice_1", a.RecoveryCode, newPassword)
}

// TC-ACC-05-10（AC-ACC-05-11）：存在账号恢复码错误和不存在账号各 50 次，中位数相差 ≤ 5%。关闭锁定。
func TestTC_ACC_05_10_Timing(t *testing.T) {
	if raceEnabled {
		t.Skip("竞态检测下耗时被放大 20 倍以上，时序判定看不带 -race 的运行：go test -run TestTC_ACC_05_10_Timing ./e2e")
	}
	e := openEnv(t, func(c *app.Config) { c.Account.TestDisableLock = true })
	e.regAcked("alice_1")
	one := func(user string) time.Duration {
		start := time.Now()
		c, b, _ := e.post("/v1/account/recover", "203.0.113.30", recoverBody(user, "AAAA-BBBB", newPassword))
		d := time.Since(start)
		wantErr(t, user, c, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
		return d
	}
	// 先预热，再交替先后顺序取样：冷启动和固定顺序的差异比被测的差异还大。
	for range 100 {
		one("alice_1")
		one("nouser1")
	}
	// 中位数只有几十微秒，机器忙时会被噪声盖过，所以最多测 3 轮：真正的差异每一轮都超标。
	var hit, miss []time.Duration
	for round := range 3 {
		hit, miss = hit[:0], miss[:0]
		for i := range 400 {
			if i%2 == 0 {
				hit = append(hit, one("alice_1"))
				miss = append(miss, one("nouser1"))
			} else {
				miss = append(miss, one("nouser1"))
				hit = append(hit, one("alice_1"))
			}
		}
		if medianClose(hit, miss, timingTolerance) {
			return
		}
		t.Logf("round %d over tolerance", round)
	}
	assertMedianClose(t, hit, miss, timingTolerance)
}

// TC-ACC-05-11（AC-ACC-05-12）：重新生成：错密码 401 码不变；对了新一代；第 6 天旧码可用，第 8 天不可用。
func TestTC_ACC_05_11_Regenerate(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	u1, u2 := e.regAcked("alice_1"), e.regAcked("bobby_1")
	c, b := e.bearerPost("/v1/account/recovery-code", u1.AccessToken, e.freshIP(), map[string]any{"password": "wrong-password-1"})
	wantErr(t, "wrong", c, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	if n := e.count(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = 'alice_1'`); n != 1 {
		t.Errorf("rows %d", n)
	}
	for _, u := range []authResp{u1, u2} {
		c, b := e.bearerPost("/v1/account/recovery-code", u.AccessToken, e.freshIP(), map[string]any{"password": goodPassword})
		if c != http.StatusOK {
			t.Fatalf("%d %s", c, b)
		}
		validateHTTP(t, "RecoveryCodeResponse", b)
		var r authResp
		mustJSON(t, b, &r)
		if r.RecoveryCodeGen != 2 {
			t.Errorf("%s", b)
		}
		e.ack(u.AccessToken, r.RecoveryCodeGen)
	}
	clk.advance(6 * oneDay)
	e.mustRecover("alice_1", u1.RecoveryCode, newPassword)
	clk.advance(2 * oneDay)
	e.wantRecoverFail("day 8", "bobby_1", u2.RecoveryCode, newPassword)
}

// TC-ACC-05-12（AC-ACC-05-13）：改密响应丢失：登录拿到更新的码，旧已确认码还有效。
func TestTC_ACC_05_12_PasswordResponseLost(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.regAcked("alice_1")
	lost := e.mustChangePassword(r.AccessToken, goodPassword, newPassword)
	l := e.mustLogin("alice_1", newPassword, "ios")
	if l.RecoveryCodeGen != 3 {
		t.Fatalf("%+v", l)
	}
	e.wantRecoverFail("lost code", "alice_1", lost.RecoveryCode, "Another-pass-2026!")
	e.ack(l.AccessToken, 3)
	if again := e.mustLogin("alice_1", newPassword, "ios"); again.RecoveryCode != "" {
		t.Errorf("%+v", again)
	}
	clk.advance(6 * oneDay)
	e.mustRecover("alice_1", r.RecoveryCode, "Another-pass-2026!")
}

// TC-ACC-05-13（AC-ACC-05-14）：旧 gen 不算确认。
func TestTC_ACC_05_13_StaleAck(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c, b := e.bearerPost("/v1/account/recovery-code/ack", d1.AccessToken, e.freshIP(), map[string]any{"gen": 1})
	wantErr(t, "stale", c, b, http.StatusBadRequest, protocol.CodeBadRequest, protocol.ReasonRecoveryCodeStale)
	if code, b := e.get("/v1/account/events", d2.AccessToken); code != 200 || !strings.Contains(string(b), `"recoveryCodeUnacked":true`) {
		t.Errorf("%d %s", code, b)
	}
	e.ack(d2.AccessToken, 2)
	if code, b := e.get("/v1/account/events", d2.AccessToken); code != 200 || strings.Contains(string(b), `recoveryCodeUnacked`) {
		t.Errorf("%d %s", code, b)
	}
}

// TC-ACC-05-16（AC-ACC-05-17，RS4-01）：攻击者用新码找回再改密：受害者第 6 天旧码清除一切；第 8 天不延长。
func TestTC_ACC_05_16_OldCodeWins(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	attack := func(user string) (c1 string, codes []string, xs []authResp) {
		v := e.regAcked(user)
		x := e.mustLogin(user, goodPassword, "ios")
		p := e.mustChangePassword(x.AccessToken, goodPassword, "Xx-pass-2026-a!") // C2
		e.ack(p.AccessToken, p.RecoveryCodeGen)
		rec := e.mustRecover(user, p.RecoveryCode, "Xx-pass-2026-b!") // C3
		e.ack(rec.AccessToken, rec.RecoveryCodeGen)
		p2 := e.mustChangePassword(rec.AccessToken, "Xx-pass-2026-b!", "Xx-pass-2026-c!") // C4
		e.ack(p2.AccessToken, p2.RecoveryCodeGen)
		return v.RecoveryCode, []string{p.RecoveryCode, rec.RecoveryCode, p2.RecoveryCode}, []authResp{p2}
	}
	c1, later, xs := attack("victim_1")
	c1b, _, _ := attack("victim_2")
	clk.advance(6 * oneDay)
	v := e.mustRecover("victim_1", c1, newPassword)
	for _, x := range xs {
		e.wantRefreshErr("attacker", x.RefreshToken, protocol.ReasonCredentialsChanged)
	}
	if n := e.liveSessions("victim_1"); n != 1 {
		t.Errorf("sessions %d", n)
	}
	e.authOK("victim", v.AccessToken, testPlatform)
	for i, code := range later {
		e.wantRecoverFail(fmt.Sprint("later ", i), "victim_1", code, "Another-pass-2026!")
	}
	clk.advance(2 * oneDay)
	e.wantRecoverFail("day 8 not extended", "victim_2", c1b, newPassword)
}

// TC-ACC-04-12（AC-ACC-04-13，RS4-01）：第 6 天成功，C2 随即作废；第 8 天失败；没有用过 C1 时 C2 可用。
func TestTC_ACC_04_12_OldCodeRetention(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	u1, u2 := e.regAcked("alice_1"), e.regAcked("bobby_1")
	p1, p2 := e.mustChangePassword(u1.AccessToken, goodPassword, newPassword), e.mustChangePassword(u2.AccessToken, goodPassword, newPassword)
	e.ack(p1.AccessToken, 2)
	e.ack(p2.AccessToken, 2)
	clk.advance(6 * oneDay)
	e.mustRecover("alice_1", u1.RecoveryCode, "Another-pass-2026!")
	e.wantRecoverFail("c2 revoked", "alice_1", p1.RecoveryCode, "Another-pass-2026?")
	clk.advance(2 * oneDay)
	e.wantRecoverFail("day 8", "bobby_1", u2.RecoveryCode, "Another-pass-2026!")
	e.mustRecover("bobby_1", p2.RecoveryCode, "Another-pass-2026!")
}

// TC-ACC-04-19（AC-ACC-04-20，RS4-06 丙）：同一个码并发，恰好一个成功，最终密码是成功那个的。
func TestTC_ACC_04_19_ConcurrentSameCode(t *testing.T) {
	e := openEnv(t, nil)
	for round := range 10 {
		user := fmt.Sprintf("alice_%d", round)
		r := e.regAcked(user)
		pws := []string{"Concurrent-pass-1!", "Concurrent-pass-2!"}
		codes := make([]int, 2)
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], _ = e.recover(user, r.RecoveryCode, pws[i], "")
			}()
		}
		wg.Wait()
		winner := -1
		for i, c := range codes {
			if c == http.StatusOK {
				if winner >= 0 {
					t.Fatalf("round %d: both succeeded", round)
				}
				winner = i
			} else if c != http.StatusUnauthorized {
				t.Fatalf("round %d: %d", round, c)
			}
		}
		if winner < 0 {
			t.Fatalf("round %d: none succeeded", round)
		}
		e.mustLogin(user, pws[winner], "ios")
	}
}

// TC-ACC-04-22（AC-ACC-04-23，RS5-02 甲）：攻击者循环改密和登录，压制不了受害者：第一次就成功，攻击者全部吊销。
func TestTC_ACC_04_22_PasswordLoopCannotSuppress(t *testing.T) {
	e := openEnv(t, nil)
	v := e.regAcked("victim_1")
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	var rounds int
	go func() {
		defer wg.Done()
		pw := goodPassword
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			code, b, _ := e.post("/v1/account/login", e.freshIP(), map[string]any{"username": "victim_1", "password": pw, "platform": "web"})
			if code != http.StatusOK {
				continue // 受害者已经成功，或者这一轮撞上单哈希：继续
			}
			var l authResp
			mustJSON(t, b, &l)
			next := fmt.Sprintf("Xx-loop-pass-%d!", i)
			code, _ = e.bearerPost("/v1/account/password", l.AccessToken, e.freshIP(), map[string]any{"oldPassword": pw, "newPassword": next})
			if code == http.StatusOK {
				pw = next
				rounds++
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	code, b := e.recover("victim_1", v.RecoveryCode, newPassword, "")
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
	if code != http.StatusOK {
		t.Fatalf("first try failed after %d attacker rounds: %d %s", rounds, code, b)
	}
	if rounds == 0 {
		t.Error("the attacker loop did not run; the test proved nothing")
	}
	var r authResp
	mustJSON(t, b, &r)
	// 找回之后攻击者手里没有有效密码，账号只有受害者的会话。
	if n := e.liveSessions("victim_1"); n != 1 {
		t.Errorf("sessions %d", n)
	}
	e.authOK("victim", r.AccessToken, testPlatform)
	e.mustLogin("victim_1", newPassword, "ios")
}

// TC-ACC-05-17（AC-ACC-05-18，RS4-12）：匿名占满闸门：错误码快速 401 不等闸门；正确码走专用位成功。
func TestTC_ACC_05_17_GateFull(t *testing.T) {
	e := openEnv(t, nil)
	r := e.regAcked("alice_1")
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait) // 后进先出：先关钩子，再等
	e.fault(map[string]any{"argonBlock": true})
	t.Cleanup(func() { e.fault(map[string]any{"argonBlock": false}) })
	// 占满 4 个普通位。
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.login(fmt.Sprintf("nouser_%d", i), goodPassword, e.freshIP())
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, b := e.adminPost("/admin/test/faults", map[string]any{}, adminKey)
		if strings.Contains(string(b), `"argonBlocked":4`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("gate not full: %s", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	e.wantRecoverFail("wrong", "alice_1", "AAAA-BBBB", newPassword)
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("wrong code waited %v", d)
	}
	e.mustRecover("alice_1", r.RecoveryCode, newPassword)
}

// TC-ACC-04-29（AC-ACC-04-31，RS7-02）：成功次数上限只约束最新一代，受害者手里的旧代不受限。
func TestTC_ACC_04_29_LimitOnlyLatest(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	v := e.regAcked("victim_1")
	c2 := e.mustChangePassword(v.AccessToken, goodPassword, "Vv-pass-2026-a!") // V 持 C2，C1 第 7 天到期
	e.ack(c2.AccessToken, 2)
	clk.advance(oneDay)
	x0 := e.mustLogin("victim_1", "Vv-pass-2026-a!", "web")
	x := e.mustChangePassword(x0.AccessToken, "Vv-pass-2026-a!", "Xx-pass-2026-a!") // X 得到 C3
	e.ack(x.AccessToken, 3)
	code := x.RecoveryCode
	for i := range 6 {
		r := e.mustRecover("victim_1", code, fmt.Sprintf("Xx-loop-pass-%d!", i))
		code = r.RecoveryCode
		e.ack(r.AccessToken, r.RecoveryCodeGen)
	}
	got, b := e.recover("victim_1", code, "Xx-loop-pass-7!", "")
	wantErr(t, "7th", got, b, http.StatusTooManyRequests, protocol.CodeRateLimited, protocol.ReasonRecoverLimited)
	// 同一小时内受害者的 C2（不是最新）不受限：这是规格的重点，不等第 2 天。
	sameHour := e.mustRecover("victim_1", c2.RecoveryCode, newPassword)
	if sameHour.RecoveryCode == "" {
		t.Error("no new code")
	}
	e.wantRecoverFail("attacker code revoked", "victim_1", code, "Xx-loop-pass-8!")
}

// RS7-02 的第二个方向：单码账号（最新就是最老）也受上限约束，攻击者不能靠"最老的码"绕过。
func TestRecoverLimitSingleCode(t *testing.T) {
	e := openEnv(t, nil)
	e.freeze()
	r := e.regAcked("alice_1")
	code := r.RecoveryCode
	for i := range 6 {
		got := e.mustRecover("alice_1", code, fmt.Sprintf("Loop-pass-2026-%d!", i))
		code = got.RecoveryCode
		e.ack(got.AccessToken, got.RecoveryCodeGen)
	}
	c, b := e.recover("alice_1", code, "Loop-pass-2026-7!", "")
	wantErr(t, "7th", c, b, http.StatusTooManyRequests, protocol.CodeRateLimited, protocol.ReasonRecoverLimited)
}

// TC-ACC-04-28（AC-ACC-04-30，RS6-02）：攻击者循环持新码占位，受害者旧码和其他账号都在时限内成功；调度单测见 internal/account。
func TestTC_ACC_04_28_FairSlot(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.Account.Argon.MemoryKiB = 8 * 1024 })
	v := e.regAcked("victim_1")
	x := e.mustChangePassword(v.AccessToken, goodPassword, "Xx-pass-2026-a!")
	e.ack(x.AccessToken, 2)
	u8, u9 := e.regAcked("user_8"), e.regAcked("user_9")
	stop := make(chan struct{})
	var wg sync.WaitGroup
	loop := func(user, start string) {
		// 一个码循环 20 并发；成功后换新码。
		var mu sync.Mutex
		code := start
		for i := range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for n := 0; ; n++ {
					select {
					case <-stop:
						return
					default:
					}
					mu.Lock()
					cur := code
					mu.Unlock()
					c, b, _ := e.post("/v1/account/recover", e.freshIP(), recoverBody(user, cur, fmt.Sprintf("Loop-pass-2026-%d-%d!", i, n)))
					if c == http.StatusOK {
						var r authResp
						mustJSON(t, b, &r)
						mu.Lock()
						code = r.RecoveryCode
						mu.Unlock()
					}
				}
			}()
		}
	}
	loop("victim_1", x.RecoveryCode)
	loop("user_8", u8.RecoveryCode)
	time.Sleep(200 * time.Millisecond)
	for _, tc := range []struct{ user, code string }{{"victim_1", v.RecoveryCode}, {"user_9", u9.RecoveryCode}} {
		start := time.Now()
		c, b := e.recover(tc.user, tc.code, newPassword, "")
		if c != http.StatusOK {
			t.Errorf("%s: %d %s", tc.user, c, b)
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%s took %v", tc.user, d)
		}
	}
	close(stop)
	wg.Wait()
}

// TC-ACC-04-24（AC-ACC-04-25，RS5-03）：四条序列都带 recoverBefore，都是第 7 天。
func TestTC_ACC_04_24_RecoverBeforeSequences(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	day7 := clk.now.Add(7 * oneDay).UnixMilli()
	setup := func(user string) (v, x authResp) {
		v = e.regAcked(user)
		x = e.mustLogin(user, goodPassword, "ios")
		return v, x
	}
	check := func(label string, v authResp, kick string) {
		t.Helper()
		ev := waitKicked(e.connect(label, v.AccessToken, testPlatform), kick, protocol.CloseTokenRevoked)
		if ev.RecoverBefore == nil || *ev.RecoverBefore != day7 {
			t.Errorf("%s kicked recoverBefore %v, want %d", label, ev.RecoverBefore, day7)
		}
		got := e.refreshErrAny(v.RefreshToken)
		if got.Details["recoverBefore"] != float64(day7) {
			t.Errorf("%s 4019 recoverBefore %+v, want %d", label, got, day7)
		}
	}
	// 甲：先 logout-others 再改密。第一次吊销的原因保留，日期不受影响。
	va, xa := setup("seq_a")
	code, b := e.bearerPost("/v1/account/logout-others", xa.AccessToken, e.freshIP(), map[string]any{"password": goodPassword})
	if code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	var xa2 authResp
	mustJSON(t, b, &xa2)
	e.mustChangePassword(xa2.AccessToken, goodPassword, newPassword)
	// 乙：改密后注销（注销接口随 BE-420，见 TestTC_ACC_04_24_DeleteSequence）。
	// 丙：第 0 天改密，第 5 天再改一次，日期不推后。
	vc, xc := setup("seq_c")
	xc2 := e.mustChangePassword(xc.AccessToken, goodPassword, newPassword)
	e.ack(xc2.AccessToken, xc2.RecoveryCodeGen)
	// 丁：受害者在线时 logout-others：C1 要先换代才有日期。
	vd, xd := setup("seq_d")
	xd2 := e.mustChangePassword(xd.AccessToken, goodPassword, newPassword)
	vd2 := e.mustLogin("seq_d", newPassword, testPlatform)
	e.ack(vd2.AccessToken, vd2.RecoveryCodeGen)
	online := e.authOK("seq_d online", vd2.AccessToken, testPlatform)
	_ = vd
	code, b = e.bearerPost("/v1/account/logout-others", xd2.AccessToken, e.freshIP(), map[string]any{"password": newPassword})
	if code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	ev := waitKicked(online, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	if ev.RecoverBefore == nil || *ev.RecoverBefore != day7 {
		t.Errorf("seq_d: %v, want %d", ev.RecoverBefore, day7)
	}
	clk.advance(oneDay)
	check("seq_a", va, protocol.KickSessionRevoked)
	clk.advance(4 * oneDay)
	xc3 := e.mustLogin("seq_c", newPassword, "web")
	e.mustChangePassword(xc3.AccessToken, newPassword, "Second-pass-2026!")
	clk.advance(oneDay)
	check("seq_c", vc, protocol.KickPasswordChanged)
	// 找回之后不再提示（规则 6a）。
	vrec := e.mustRecover("seq_c", vc.RecoveryCode, "Third-pass-2026!")
	e.ack(vrec.AccessToken, vrec.RecoveryCodeGen)
	got := e.refreshErrAny(vc.RefreshToken)
	if got.Details["recoverBefore"] != nil {
		t.Errorf("after recovery: %+v", got)
	}
}

// TC-ACC-01-16（AC-ACC-01-16）：匿名占满闸门：refresh、logout 不等闸门，改密用专用位；第 5 个匿名注册约 1 秒后 5030；关钩子后前 4 个完成。
func TestTC_ACC_01_16_GateFullOthersFine(t *testing.T) {
	e := openEnv(t, nil)
	u1 := e.register("alice_1", nil)
	other := e.mustLogin("alice_1", goodPassword, "ios")
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	e.fault(map[string]any{"argonBlock": true})
	released := false
	release := func() {
		if !released {
			released = true
			e.fault(map[string]any{"argonBlock": false})
		}
	}
	t.Cleanup(release)
	codes := make([]int, 4)
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _, _ = e.post("/v1/account/register", e.freshIP(), regBody(fmt.Sprintf("anon_%d", i), nil))
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, b := e.adminPost("/admin/test/faults", map[string]any{}, adminKey)
		if strings.Contains(string(b), `"argonBlocked":4`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("gate not full: %s", b)
		}
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	e.mustRefresh(u1.RefreshToken)
	if c, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": other.RefreshToken}); c != http.StatusOK {
		t.Fatalf("logout %d %s", c, b)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("refresh and logout waited %v", d)
	}
	e.mustChangePassword(u1.AccessToken, goodPassword, newPassword)
	start = time.Now()
	c, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("anon_5", nil))
	wantErr(t, "5th", c, b, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable, "")
	if d := time.Since(start); d < 900*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("5th register took %v", d)
	}
	release()
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("register %d: %d", i, c)
		}
	}
}

// TC-ACC-04-27（AC-ACC-04-29）：recover 事务第一条语句之后，三种请求都等待后 401。
// 只有 Postgres 实现后才能跑：仓库只有 SQLite，事务串行，这条交错不可达。**未执行，不算通过**（RS6-01、RS7-01）。
func TestTC_ACC_04_27_NotExecuted(t *testing.T) {
	t.Skip("未执行：只有 Postgres 下可达，C1 验收不算通过（AC-ACC-04-29）")
}
