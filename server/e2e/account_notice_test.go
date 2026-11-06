package e2e

import (
	"encoding/json"
	"net/http"
	"runtime"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/protocol"
)

// noticeQuiet 是断言"没有提醒"时等的时间：真实连接上的提醒在几毫秒内到达。
const noticeQuiet = 150 * time.Millisecond

// noticesIn 收集 d 内到达的 securityNotice，其他帧留在 pending。
func noticesIn(c *client, d time.Duration) []protocol.SecurityNoticeEvent {
	c.t.Helper()
	var out []protocol.SecurityNoticeEvent
	take := func(f frame) bool {
		var ev protocol.SecurityNoticeEvent
		if f.Cmd == protocol.CmdEvent && json.Unmarshal(f.Data, &ev) == nil && ev.Type == protocol.EventSecurityNotice {
			out = append(out, ev)
			return true
		}
		return false
	}
	kept := c.pending[:0]
	for _, f := range c.pending {
		if !take(f) {
			kept = append(kept, f)
		}
	}
	c.pending = kept
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return out
			}
			if !take(f) {
				c.pending = append(c.pending, f)
			}
		case <-deadline:
			return out
		}
	}
}

// waitNotice 等这个 kind 的提醒，跳过别的 kind（freshIP 换前缀时前面可能有 newIpPrefix）。
func waitNotice(c *client, kind string) protocol.SecurityNoticeEvent {
	c.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		for _, ev := range noticesIn(c, 20*time.Millisecond) {
			if ev.Kind == kind {
				return ev
			}
		}
	}
	c.t.Fatalf("%s: no securityNotice %s", c.uid, kind)
	return protocol.SecurityNoticeEvent{}
}

func wantNoNotice(t *testing.T, c *client, kinds ...string) {
	t.Helper()
	for _, ev := range noticesIn(c, noticeQuiet) {
		if len(kinds) == 0 {
			t.Errorf("%s: unexpected notice %+v", c.uid, ev)
		}
		for _, k := range kinds {
			if ev.Kind == k {
				t.Errorf("%s: unexpected notice %+v", c.uid, ev)
			}
		}
	}
}

func (e *acctEnv) refreshFrom(ip, token string) authResp {
	e.t.Helper()
	code, b, _ := e.post("/v1/account/token/refresh", ip, map[string]any{"refreshToken": token})
	if code != http.StatusOK {
		e.t.Fatalf("refresh from %s: %d %s", ip, code, b)
	}
	var r authResp
	mustJSON(e.t, b, &r)
	return r
}

// F-ACC-02 规则 3b（A13）：刷新来自这条会话没见过的前缀：其他设备收到 newIpPrefix，发起的会话收不到。
// 见过的前缀不提醒；事务没有提交（刷新失败）不提醒。
func TestNotice_NewIPPrefix_Refresh(t *testing.T) {
	e := openEnv(t, nil)
	code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("alice_1", nil))
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var r1 authResp
	mustJSON(t, b, &r1)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	self := e.authOK("self", r1.AccessToken, testPlatform)
	other := e.authOK("other", d2.AccessToken, "ios")

	before := time.Now().UnixMilli()
	r2 := e.refreshFrom("192.0.2.1", r1.RefreshToken)
	ev := waitNotice(other, account.EventNewIPPrefix)
	if ev.Platform != testPlatform || ev.At < before {
		t.Errorf("%+v", ev)
	}
	wantNoNotice(t, self)

	// 同一前缀里的另一个地址、已经见过的旧前缀：都不提醒。
	r3 := e.refreshFrom("192.0.2.200", r2.RefreshToken)
	r4 := e.refreshFrom("203.0.113.9", r3.RefreshToken)
	wantNoNotice(t, other)
	if n := len(e.events("alice_1", account.EventNewIPPrefix)); n != 1 {
		t.Errorf("events %d", n)
	}

	// 刷新被拒绝（mac 错）时什么都没提交，新前缀也不提醒。
	code, _, _ = e.post("/v1/account/token/refresh", "192.0.3.1", map[string]any{"refreshToken": r4.RefreshToken + "x"})
	if code == http.StatusOK {
		t.Fatal("bad token accepted")
	}
	wantNoNotice(t, other)
}

// F-ACC-02 规则 3b（A13）：转正来自新前缀也提醒。access 首次使用才转正，用 HTTP 发起，ip 走 X-Forwarded-For。
func TestNotice_NewIPPrefix_Promote(t *testing.T) {
	e := openEnv(t, nil)
	code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("alice_1", nil))
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var r1 authResp
	mustJSON(t, b, &r1)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	other := e.authOK("other", d2.AccessToken, "ios")

	r2 := e.refreshFrom("203.0.113.1", r1.RefreshToken) // 见过的前缀：只发新一代
	wantNoNotice(t, other)
	if code, b := e.bearerDo(http.MethodGet, "/v1/account/events", r2.AccessToken, "192.0.2.1", nil); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if ev := waitNotice(other, account.EventNewIPPrefix); ev.Platform != testPlatform {
		t.Errorf("%+v", ev)
	}
	// 转正只发生一次：再用同一个 access 不再提醒。
	if code, b := e.bearerDo(http.MethodGet, "/v1/account/events", r2.AccessToken, "192.0.4.1", nil); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	wantNoNotice(t, other)
}

// F-ACC-05 规则 5（A14）：重新生成恢复码：发起的会话收不到，其他设备收到。
func TestNotice_RecoveryRegen_ExcludesInitiator(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	e.ack(d1.AccessToken, d1.RecoveryCodeGen)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c1, c2 := e.authOK("d1", d1.AccessToken, testPlatform), e.authOK("d2", d2.AccessToken, "ios")
	wantNoNotice(t, c2) // 登录发生在连接之前
	if code, b := e.bearerPost("/v1/account/recovery-code", d1.AccessToken, e.freshIP(), map[string]any{"password": goodPassword}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if ev := waitNotice(c2, account.EventRecoveryRegen); ev.Platform != testPlatform {
		t.Errorf("%+v", ev)
	}
	wantNoNotice(t, c1)
}

// A14：新设备登录：已经在线的设备收到，kind 和平台是新设备的。
func TestNotice_NewDeviceLogin_ToOthers(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	c1 := e.authOK("d1", d1.AccessToken, testPlatform)
	e.mustLogin("alice_1", goodPassword, "ios")
	if ev := waitNotice(c1, account.EventNewDeviceLogin); ev.Platform != "ios" {
		t.Errorf("%+v", ev)
	}
}

// A14、AC-ACC-03-25：重放吊销：这条会话自己的连接只收到踢线，其他设备收到提醒。
func TestNotice_Replay_ExcludesThatSession(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("victim_1", nil)
	d2 := e.mustLogin("victim_1", goodPassword, "ios")
	other := e.authOK("d2", d2.AccessToken, "ios")
	x := e.mustRefresh(r1.RefreshToken)
	xc := e.authOK("attacker", x.AccessToken, testPlatform)
	wantNoNotice(t, xc)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("victim", r1.RefreshToken, protocol.ReasonReplayDetected)
	if ev := waitNotice(other, account.EventRefreshReplay); ev.Platform != testPlatform {
		t.Errorf("%+v", ev)
	}
	waitKicked(xc, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	for _, f := range xc.pending {
		t.Errorf("attacker got %s", f.Data)
	}
}

// A14、AC-ACC-03-45：连续 6 小时限流：这条会话自己的连接收不到，其他设备第 6 小时才收到。
func TestNotice_RateLimited_ExcludesThatSession(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	const ip = "203.0.113.1"
	code, b, _ := e.post("/v1/account/register", ip, regBody("alice_1", nil))
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var r1 authResp
	mustJSON(t, b, &r1)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	self := e.authOK("self", r1.AccessToken, testPlatform)
	other := e.authOK("other", d2.AccessToken, "ios")
	for hour := 1; hour <= 6; hour++ {
		for i := range 21 {
			code, b, _ := e.post("/v1/account/token/refresh", ip, map[string]any{"refreshToken": r1.RefreshToken})
			if i == 20 {
				wantErr(t, "21st", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
			} else if code != http.StatusOK {
				t.Fatalf("hour %d round %d: %d %s", hour, i, code, b)
			}
		}
		if hour == 5 {
			wantNoNotice(t, other)
		}
		if hour < 6 {
			clk.advance(time.Hour)
		}
	}
	if ev := waitNotice(other, account.EventRefreshRateLimited); ev.Platform != testPlatform {
		t.Errorf("%+v", ev)
	}
	wantNoNotice(t, self)
}

// wantNoticeThenKick：提醒必须排在踢线之前（踢线之后连接就关了），带着 kind 和平台。
func wantNoticeThenKick(t *testing.T, c *client, kind, platform, reason string) {
	t.Helper()
	first := c.next()
	var ev protocol.SecurityNoticeEvent
	mustJSON(t, first.Data, &ev)
	if ev.Type != protocol.EventSecurityNotice || ev.Kind != kind || ev.Platform != platform || ev.At == 0 {
		t.Fatalf("%s: first frame %s, want securityNotice/%s from %s", c.uid, first.Data, kind, platform)
	}
	waitKicked(c, reason, protocol.CloseTokenRevoked)
}

// F-ACC-02 规则 3b、F-ACC-05 规则 5（A13）：改密：其他设备先收到提醒再被踢；发起的连接只收到踢线。
// 提醒走发送队列的写协程，踢线另起协程。单核跑多轮才稳定暴露两者的竞态：后起的协程先运行，踢线抢在提醒之前。
func TestNotice_PasswordChanged_BeforeKick(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	for round := range 25 {
		e := openEnv(t, nil)
		d1 := e.register("alice_1", nil)
		e.ack(d1.AccessToken, d1.RecoveryCodeGen)
		d2 := e.mustLogin("alice_1", goodPassword, "ios")
		c1, c2 := e.authOK("d1", d1.AccessToken, testPlatform), e.authOK("d2", d2.AccessToken, "ios")
		e.mustChangePassword(d1.AccessToken, goodPassword, newPassword)
		wantNoticeThenKick(t, c2, account.EventPasswordChanged, testPlatform, protocol.KickPasswordChanged)
		first := c1.next()
		var head protocol.KickedEvent
		mustJSON(t, first.Data, &head)
		if head.Type != protocol.EventKicked || head.Reason != protocol.KickPasswordChanged {
			t.Fatalf("round %d initiator: %s", round, first.Data)
		}
	}
}

// F-ACC-02 规则 3b（A13）：找回：发起方没有连接，所有在线设备都先收到提醒再被踢。平台是发起找回的设备。
func TestNotice_PasswordRecovered_BeforeKick(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	e.ack(d1.AccessToken, d1.RecoveryCodeGen)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c1, c2 := e.authOK("d1", d1.AccessToken, testPlatform), e.authOK("d2", d2.AccessToken, "ios")
	code, b, _ := e.post("/v1/account/recover", e.freshIP(), map[string]any{"username": "alice_1", "recoveryCode": d1.RecoveryCode, "newPassword": newPassword, "platform": "web"})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	wantNoticeThenKick(t, c1, account.EventPasswordRecover, "web", protocol.KickPasswordChanged)
	wantNoticeThenKick(t, c2, account.EventPasswordRecover, "web", protocol.KickPasswordChanged)
}

// 规则 3b 没有列 logout-others：只有 kicked（F-ACC-04 规则 6），没有提醒。
func TestNotice_LogoutOthers_OnlyKick(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	e.ack(d1.AccessToken, d1.RecoveryCodeGen)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c2 := e.authOK("d2", d2.AccessToken, "ios")
	if code, b := e.bearerPost("/v1/account/logout-others", d1.AccessToken, e.freshIP(), map[string]any{"password": goodPassword}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	first := c2.next()
	var head protocol.KickedEvent
	mustJSON(t, first.Data, &head)
	if head.Type != protocol.EventKicked || head.Reason != protocol.KickSessionRevoked {
		t.Fatalf("%s", first.Data)
	}
}
