package e2e

// F-ACC-03 规则 2、4、11 的连接和 HTTP 判定（BE-416）。

import (
	"net/http"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func recheckFast(c *app.Config) {
	c.Gateway.RecheckInterval = 10 * time.Minute
	c.Gateway.RecheckTick = 50 * time.Millisecond
}

func (e *acctEnv) fault(body map[string]any) {
	e.t.Helper()
	if code, b := e.adminPost("/admin/test/faults", body, adminKey); code != http.StatusOK {
		e.t.Fatalf("faults: %d %s", code, b)
	}
}

// TC-ACC-03-05（AC-ACC-03-5）：管理接口 token 没有 sid，连接和收发和现在一样；强制下线后按现有规则。
func TestTC_ACC_03_05_NoSIDToken(t *testing.T) {
	e := openEnv(t, nil)
	e.users("u1", "u2")
	token := e.token("u1")
	claims := claimsOf(t, token)
	if claims["sid"] != nil || claims["gen"] != nil {
		t.Errorf("%v", claims)
	}
	c := e.authOK("u1", token, testPlatform)
	if f := c.sendText("c2c", "u2", "msg-0001", "hi"); f.Error != nil {
		t.Errorf("%+v", f.Error)
	}
	if code, b := e.adminPost("/admin/users/u1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	waitKicked(c, protocol.KickBanned, protocol.CloseTokenRevoked)
	e.authKicked("u1", token, protocol.KickBanned, protocol.CloseTokenRevoked)
}

// TC-ACC-03-06（AC-ACC-03-6）：强制下线、封禁后旧 refresh 都 4019，不是 4034；用密码登录才看到 4034。
func TestTC_ACC_03_06_RevokedThen4019(t *testing.T) {
	e := openEnv(t, nil)
	u1, u2 := e.register("alice_1", nil), e.register("bobby_1", nil)
	if code, b := e.adminPost("/admin/users/alice_1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.wantRefreshErr("forced", u1.RefreshToken, protocol.ReasonRefreshRevoked)
	e.ban("bobby_1")
	e.wantRefreshErr("banned", u2.RefreshToken, protocol.ReasonRefreshRevoked)
	code, b := e.login("bobby_1", goodPassword, e.freshIP())
	wantErr(t, "login", code, b, http.StatusForbidden, protocol.CodeUserBanned, "")
}

// TC-ACC-03-17（AC-ACC-03-17）：入表前吊销：连接随后收到 sessionRevoked，不留在线。
func TestTC_ACC_03_17_RevokedBeforeRegister(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	e.fault(map[string]any{"revokeBeforeRegister": "alice_1"})
	c := e.connect("alice_1", r.AccessToken, testPlatform)
	waitKicked(c, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
}

// TC-ACC-03-28（AC-ACC-03-28）：被淘汰的代：WS tokenExpired，HTTP 4011 tokenSuperseded，不转正，R1 仍有效。
func TestTC_ACC_03_28_EvictedAccess(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	a2 := e.mustRefresh(r1.RefreshToken)
	for range 4 {
		e.mustRefresh(r1.RefreshToken)
	}
	e.authKicked("alice_1", a2.AccessToken, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	code, b := e.get("/v1/account/events", a2.AccessToken)
	wantErr(t, "http", code, b, http.StatusUnauthorized, protocol.CodeTokenExpired, protocol.ReasonTokenSuperseded)
	if d := e.session(sidOf(r1.RefreshToken)); d.CurrentGen != 1 || d.RevokedAt != 0 {
		t.Errorf("%+v", d)
	}
	e.mustRefresh(r1.RefreshToken)
}

// TC-ACC-03-29（AC-ACC-03-29）：过期或验签失败的 pending 不转正。
func TestTC_ACC_03_29_InvalidNoPromotion(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	a2 := e.mustRefresh(r1.RefreshToken)
	sid := sidOf(r1.RefreshToken)
	bad := a2.AccessToken[:len(a2.AccessToken)-2] + "xx"
	e.authKicked("alice_1", bad, protocol.KickTokenInvalid, protocol.CloseTokenInvalid)
	if d := e.session(sid); d.CurrentGen != 1 {
		t.Errorf("bad signature promoted: %d", d.CurrentGen)
	}
	clk.advance(25 * time.Hour)
	e.authKicked("alice_1", a2.AccessToken, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	if d := e.session(sid); d.CurrentGen != 1 {
		t.Errorf("expired token promoted: %d", d.CurrentGen)
	}
}

// TC-ACC-03-30（AC-ACC-03-30）：攻击者转正，在线的受害者立刻被踢；A1 被拒绝；窗口内 superseded，窗口后重放，攻击者也失效。
func TestTC_ACC_03_30_VictimKicked(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("victim_1", nil)
	v := e.authOK("victim", r1.AccessToken, testPlatform)
	x := e.mustRefresh(r1.RefreshToken)
	e.authOK("attacker", x.AccessToken, testPlatform)
	waitKicked(v, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	e.authKicked("victim", r1.AccessToken, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 || d.CurrentGen != genOf(t, x.RefreshToken) {
		t.Errorf("%+v", d)
	}
	clk.advance(time.Minute)
	e.wantRefreshErr("window", r1.RefreshToken, protocol.ReasonRefreshSuperseded)
	clk.advance(2 * time.Minute)
	e.wantRefreshErr("after window", r1.RefreshToken, protocol.ReasonReplayDetected)
	e.wantRefreshErr("attacker", x.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-36（AC-ACC-03-36）：入表后复查报错：1011 断开（失败关闭），不是 4012；重连成功。
func TestTC_ACC_03_36_EntryRecheckFailsClosed(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	e.fault(map[string]any{"entryRecheckFail": 1})
	c := e.connect("alice_1", r.AccessToken, testPlatform)
	if code := c.closeCode(waitTimeout); int(code) != protocol.CloseInternalError {
		t.Fatalf("close %d", code)
	}
	e.authOK("alice_1", r.AccessToken, testPlatform)
}

// TC-ACC-03-37（AC-ACC-03-47）：丢失的踢线下一次复查补上；复查报错 1 次不断开，连续 3 次 1011。
func TestTC_ACC_03_37_PeriodicRecheck(t *testing.T) {
	e := openEnv(t, recheckFast)
	clk := e.freeze()
	ra, rb := e.register("alice_1", nil), e.register("bobby_1", nil)
	ca, cb := e.authOK("a", ra.AccessToken, testPlatform), e.authOK("b", rb.AccessToken, testPlatform)
	e.fault(map[string]any{"dropRevokeEvents": true})
	if code, b := e.adminPost("/admin/users/alice_1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	ca.expectNoMsg(150 * time.Millisecond)
	e.fault(map[string]any{"dropRevokeEvents": false})
	e.fault(map[string]any{"periodicRecheckFail": map[string]any{"userId": "bobby_1", "count": 1}})
	clk.advance(10 * time.Minute)
	waitKicked(ca, protocol.KickBanned, protocol.CloseTokenRevoked)
	// bobby_1 报错 1 次不断开，之后成功把计数清零。
	cb.expectNoMsg(200 * time.Millisecond)
	if f := cb.call("users", map[string]any{"userIds": []string{"bobby_1"}}); f.Error != nil {
		t.Fatalf("%+v", f.Error)
	}
	clk.advance(10 * time.Minute) // 成功，清零
	cb.expectNoMsg(200 * time.Millisecond)
	e.fault(map[string]any{"periodicRecheckFail": map[string]any{"userId": "bobby_1", "count": 3}})
	for i := range 2 {
		clk.advance(10 * time.Minute)
		cb.expectNoMsg(200 * time.Millisecond)
		if f := cb.call("users", map[string]any{"userIds": []string{"bobby_1"}}); f.Error != nil {
			t.Fatalf("after failure %d: %+v", i+1, f.Error)
		}
	}
	clk.advance(10 * time.Minute)
	if code := cb.closeCode(waitTimeout); int(code) != protocol.CloseInternalError {
		t.Fatalf("close %d", code)
	}
}

// 周期复查也覆盖代号和吊销原因（RS5-08）：丢了转正的踢线，下一次复查补上；吊销带 recoverBefore 的路径在改密用例里。
func TestPeriodicRecheckCoversGen(t *testing.T) {
	e := openEnv(t, recheckFast)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	c := e.authOK("alice", r1.AccessToken, testPlatform)
	e.fault(map[string]any{"dropRevokeEvents": true})
	x := e.mustRefresh(r1.RefreshToken)
	e.authOK("other", x.AccessToken, testPlatform)
	c.expectNoMsg(150 * time.Millisecond)
	e.fault(map[string]any{"dropRevokeEvents": false})
	clk.advance(10 * time.Minute)
	waitKicked(c, protocol.KickTokenExpired, protocol.CloseTokenExpired)
}

// 账号状态先于其他判定（F-ACC-08 异常）：冷静期内旧 token 一律 accountDeleted，HTTP 4012。规则 11 的 iat 判定用已知时间构造。
func TestCheckOrderAccountState(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	e.rawExec(`UPDATE accounts SET delete_requested_at = 1 WHERE user_id = 'alice_1'`)
	e.authKicked("alice", r.AccessToken, protocol.KickAccountDeleted, protocol.CloseTokenRevoked)
	code, b := e.get("/v1/account/events", r.AccessToken)
	wantErr(t, "http", code, b, http.StatusUnauthorized, protocol.CodeTokenRevoked, "")
}
