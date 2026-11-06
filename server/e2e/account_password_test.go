package e2e

// F-ACC-04 的后端用例（BE-417）和依赖改密的 F-ACC-03 用例（BE-416 的 AC-ACC-03-16、03-34，BE-415 的 03-44）。

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/protocol"
)

const newPassword = "Tt-new-pass-2026!"

func (e *acctEnv) changePassword(bearer, oldPw, newPw string) (int, []byte) {
	e.t.Helper()
	return e.bearerPost("/v1/account/password", bearer, e.freshIP(), map[string]any{"oldPassword": oldPw, "newPassword": newPw})
}

func (e *acctEnv) mustChangePassword(bearer, oldPw, newPw string) authResp {
	e.t.Helper()
	code, b := e.changePassword(bearer, oldPw, newPw)
	if code != http.StatusOK {
		e.t.Fatalf("password: %d %s", code, b)
	}
	validateHTTP(e.t, "AuthResponse", b)
	var r authResp
	mustJSON(e.t, b, &r)
	return r
}

func (e *acctEnv) ack(bearer string, gen int64) {
	e.t.Helper()
	if code, b := e.bearerPost("/v1/account/recovery-code/ack", bearer, e.freshIP(), map[string]any{"gen": gen}); code != http.StatusOK {
		e.t.Fatalf("ack %d: %d %s", gen, code, b)
	}
}

// accessClaim 是 access token 里的 sid。
func accessSID(t *testing.T, access string) string {
	t.Helper()
	s, _ := claimsOf(t, access)["sid"].(string)
	return s
}

// TC-ACC-04-01（AC-ACC-04-1）：D1 退出：refresh、access 都失效，D2 不受影响。
func TestTC_ACC_04_01_Logout(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c1, c2 := e.authOK("d1", d1.AccessToken, testPlatform), e.authOK("d2", d2.AccessToken, "ios")
	code, b := e.bearerPost("/v1/account/logout", d1.AccessToken, e.freshIP(), map[string]any{"refreshToken": d1.RefreshToken})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	waitKicked(c1, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	e.wantRefreshErr("d1", d1.RefreshToken, protocol.ReasonRefreshRevoked)
	e.authKicked("d1", d1.AccessToken, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	c2.expectNoMsg(50 * time.Millisecond)
	e.mustRefresh(d2.RefreshToken)
}

// TC-ACC-04-02（AC-ACC-04-2）：D1 改密：响应带新 sid 和新恢复码，新 token 可用；D2 被踢；旧 refresh 都 credentialsChanged，不算重放。
func TestTC_ACC_04_02_PasswordChange(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	e.ack(d1.AccessToken, d1.RecoveryCodeGen)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c1, c2 := e.authOK("d1", d1.AccessToken, testPlatform), e.authOK("d2", d2.AccessToken, "ios")
	before := e.metric("im_refresh_replay_total")
	r := e.mustChangePassword(d1.AccessToken, goodPassword, newPassword)
	if r.RecoveryCode == "" || r.RecoveryCodeGen != 2 {
		t.Errorf("%+v", r)
	}
	if sidOf(r.RefreshToken) == sidOf(d1.RefreshToken) || accessSID(t, r.AccessToken) != sidOf(r.RefreshToken) {
		t.Error("must get a new sid")
	}
	waitKicked(c1, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	waitKicked(c2, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	e.authOK("d1 new", r.AccessToken, testPlatform)
	for _, old := range []string{d1.RefreshToken, d2.RefreshToken} {
		e.wantRefreshErr("old", old, protocol.ReasonCredentialsChanged)
	}
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
	if code, _ := e.login("alice_1", goodPassword, e.freshIP()); code != http.StatusUnauthorized {
		t.Errorf("old password: %d", code)
	}
	e.mustLogin("alice_1", newPassword, "ios")
}

// TC-ACC-04-03（AC-ACC-04-3）：旧密码错误 401，没有变化，计入失败次数。
func TestTC_ACC_04_03_WrongOldPassword(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c2 := e.authOK("d2", d2.AccessToken, "ios")
	code, b := e.changePassword(d1.AccessToken, "wrong-password-1", newPassword)
	wantErr(t, "wrong", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	c2.expectNoMsg(50 * time.Millisecond)
	if n := e.count(`SELECT COUNT(*) FROM auth_throttle WHERE throttle_key LIKE 'pw:%'`); n != 1 {
		t.Errorf("failure not counted: %d", n)
	}
	e.mustLogin("alice_1", goodPassword, "ios")
}

// TC-ACC-04-04（AC-ACC-04-4）：强制下线吊销全部设备会话。
func TestTC_ACC_04_04_ForceLogout(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	if code, b := e.adminPost("/admin/users/alice_1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.wantRefreshErr("d1", d1.RefreshToken, protocol.ReasonRefreshRevoked)
	e.wantRefreshErr("d2", d2.RefreshToken, protocol.ReasonRefreshRevoked)
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE revoked_at <> 0`); n != 2 {
		t.Errorf("revoked %d", n)
	}
}

// TC-ACC-04-07（AC-ACC-04-7）：吊销写库失败：500，整体回滚，旧密码仍有效，其他设备不受影响，版本不变。
func TestTC_ACC_04_07_RevokeWriteFails(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c2 := e.authOK("d2", d2.AccessToken, "ios")
	e.fault(map[string]any{"revokeWriteFail": 1})
	code, b := e.changePassword(d1.AccessToken, goodPassword, newPassword)
	wantErr(t, "fault", code, b, http.StatusInternalServerError, protocol.CodeInternal, "")
	c2.expectNoMsg(50 * time.Millisecond)
	if n := e.count(`SELECT credentials_version FROM accounts WHERE user_id = 'alice_1'`); n != 1 {
		t.Errorf("version %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE revoked_at <> 0`); n != 0 {
		t.Errorf("revoked %d", n)
	}
	e.mustLogin("alice_1", goodPassword, "ios")
	e.mustRefresh(d2.RefreshToken)
}

// TC-ACC-04-08（AC-ACC-04-8）：D2 刷新和 D1 改密并发 20 轮：没有一轮在改密后还有可用的 D2 会话。
func TestTC_ACC_04_08_RefreshRacePassword(t *testing.T) {
	e := openEnv(t, nil)
	e.register("alice_1", nil)
	cur := goodPassword
	for i := range 20 {
		d1 := e.mustLogin("alice_1", cur, testPlatform)
		d2 := e.mustLogin("alice_1", cur, "ios")
		var wg sync.WaitGroup
		var code int
		var body []byte
		next := newPassword + string(rune('a'+i))
		var r authResp
		wg.Add(2)
		go func() {
			defer wg.Done()
			code, body = e.refresh(d2.RefreshToken)
		}()
		go func() {
			defer wg.Done()
			r = e.mustChangePassword(d1.AccessToken, cur, next)
		}()
		wg.Wait()
		cur = next
		_ = r
		if code == http.StatusOK {
			var got authResp
			mustJSON(t, body, &got)
			e.authKicked("d2 new", got.AccessToken, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
			e.wantRefreshErr("d2 new", got.RefreshToken, protocol.ReasonCredentialsChanged)
		} else {
			wantErr(t, "d2", code, body, http.StatusUnauthorized, protocol.CodeRefreshTokenInvalid, protocol.ReasonCredentialsChanged)
		}
		if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE revoked_at = 0 AND sid = ?`, sidOf(d2.RefreshToken)); n != 0 {
			t.Fatalf("round %d: d2 session survived", i)
		}
	}
}

// TC-ACC-04-09（AC-ACC-04-10，服务端部分）：退出其他设备：错密码 401 没变化；正确时 D2 sessionRevoked，D1 换 sid，密码不变。
func TestTC_ACC_04_09_LogoutOthers(t *testing.T) {
	e := openEnv(t, nil)
	d1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c2 := e.authOK("d2", d2.AccessToken, "ios")
	code, b := e.bearerPost("/v1/account/logout-others", d1.AccessToken, e.freshIP(), map[string]any{"password": "wrong-password-1"})
	wantErr(t, "wrong", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	c2.expectNoMsg(50 * time.Millisecond)
	code, b = e.bearerPost("/v1/account/logout-others", d1.AccessToken, e.freshIP(), map[string]any{"password": goodPassword})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	validateHTTP(t, "AuthResponse", b)
	var r authResp
	mustJSON(t, b, &r)
	waitKicked(c2, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	if sidOf(r.RefreshToken) == sidOf(d1.RefreshToken) {
		t.Error("must get a new sid")
	}
	e.authOK("d1 new", r.AccessToken, testPlatform)
	e.wantRefreshErr("d2", d2.RefreshToken, protocol.ReasonLoggedOutOthers)
	e.wantRefreshErr("d1 old", d1.RefreshToken, protocol.ReasonLoggedOutOthers)
	e.mustLogin("alice_1", goodPassword, "ios")
}

// TC-ACC-04-10（AC-ACC-04-11）：logout 重复调用都成功。
func TestTC_ACC_04_10_LogoutTwice(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	for range 2 {
		if code, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": r.RefreshToken}); code != http.StatusOK {
			t.Fatalf("%d %s", code, b)
		}
	}
}

// TC-ACC-04-11（AC-ACC-04-12）：新密码不合规：400，密码和恢复码都不变。
func TestTC_ACC_04_11_WeakNewPassword(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	code, b := e.changePassword(r.AccessToken, goodPassword, "aaaaaaaa")
	wantErr(t, "weak", code, b, http.StatusBadRequest, protocol.CodeBadRequest, protocol.ReasonPasswordWeak)
	e.mustLogin("alice_1", goodPassword, "ios")
	if n := e.count(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = 'alice_1' AND gen > 2`); n != 0 {
		t.Errorf("new generation: %d", n)
	}
}

// TC-ACC-04-13（AC-ACC-04-14）：待退出：只有 refresh 也能退出，已作废的旧代值不算重放。
func TestTC_ACC_04_13_PendingLogout(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	build := func(name string) (authResp, authResp) {
		r1 := e.register(name, nil)
		r2 := e.mustRefresh(r1.RefreshToken)
		e.mustRefresh(r2.RefreshToken) // R2 转正
		return r1, r2
	}
	_, a2 := build("alice_1")
	b1, _ := build("bobby_1")
	clk.advance(25 * time.Hour)
	before := e.metric("im_refresh_replay_total")
	for _, tok := range []string{a2.RefreshToken, b1.RefreshToken} {
		if code, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": tok}); code != http.StatusOK {
			t.Fatalf("%d %s", code, b)
		}
		if d := e.session(sidOf(tok)); d.RevokedAt == 0 || d.RevokeReason != "revoked" {
			t.Errorf("%+v", d)
		}
	}
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
}

// TC-ACC-04-14（AC-ACC-04-15，RS4-03）：攻击者和 D1 共用 sid（同一代）：D1 改密把攻击者清除。
func TestTC_ACC_04_14_SharedSIDCleared(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("victim_1", nil)
	a2 := e.mustRefresh(r1.RefreshToken)
	x := e.authOK("attacker", a2.AccessToken, testPlatform)
	e.authOK("d1", a2.AccessToken, testPlatform)
	r := e.mustChangePassword(a2.AccessToken, goodPassword, newPassword)
	waitKicked(x, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	e.wantRefreshErr("x refresh", a2.RefreshToken, protocol.ReasonCredentialsChanged)
	e.authKicked("x access", a2.AccessToken, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	if sidOf(r.RefreshToken) == sidOf(a2.RefreshToken) {
		t.Error("same sid")
	}
	e.authOK("d1 new", r.AccessToken, testPlatform)
}

// TC-ACC-04-15（AC-ACC-04-16，服务端部分）：响应丢失，旧 refresh 4019 credentialsChanged，带 recoverBefore，不计重放；新密码能登录，登录带再新一代的码。
func TestTC_ACC_04_15_ResponseLost(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	d1 := e.register("alice_1", nil)
	e.ack(d1.AccessToken, d1.RecoveryCodeGen)
	before := e.metric("im_refresh_replay_total")
	e.mustChangePassword(d1.AccessToken, goodPassword, newPassword) // 响应丢弃
	got := e.wantRefreshErr("old", d1.RefreshToken, protocol.ReasonCredentialsChanged)
	want := float64(clk.now.Add(7 * oneDay).UnixMilli())
	if got.Details["recoverBefore"] != want {
		t.Errorf("recoverBefore %v, want %v", got.Details["recoverBefore"], want)
	}
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
	l := e.mustLogin("alice_1", newPassword, testPlatform)
	if l.RecoveryCode == "" || l.RecoveryCodeGen != 3 {
		t.Errorf("%+v", l)
	}
}

// TC-ACC-03-16（AC-ACC-03-16）：账号用户的无 sid token：改密后拒绝，之后签发的能连，不能调账号接口。
func TestTC_ACC_03_16_NoSIDAfterPassword(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.register("alice_1", nil)
	before := e.count(`SELECT token_version FROM users WHERE user_id = 'alice_1'`)
	t1 := e.token("alice_1")
	online := e.authOK("t1", t1, testPlatform)
	clk.advance(time.Second)
	e.mustChangePassword(r.AccessToken, goodPassword, newPassword)
	waitKicked(online, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	e.authKicked("t1", t1, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	clk.advance(time.Second)
	t2 := e.token("alice_1")
	e.authOK("t2", t2, testPlatform)
	if n := e.count(`SELECT token_version FROM users WHERE user_id = 'alice_1'`); n != before {
		t.Errorf("token_version changed: %d", n)
	}
	for _, path := range []string{"password", "logout", "logout-others", "recovery-code", "recovery-code/ack", "delete"} {
		code, b := e.bearerPost("/v1/account/"+path, t2, e.freshIP(), map[string]any{"oldPassword": newPassword, "newPassword": goodPassword, "password": newPassword})
		wantErr(t, path, code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountSessionRequired)
	}
	code, b := e.bearerDo(http.MethodPatch, "/v1/account/profile", t2, e.freshIP(), map[string]any{"nickname": "x"})
	wantErr(t, "profile", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountSessionRequired)
}

// TC-ACC-03-34（AC-ACC-03-34，服务端部分，RS4-07）：离线受害者遇到攻击者改密：access 未过期 kicked/passwordChanged，已过期 4019，都带 recoverBefore=T+7 天。
func TestTC_ACC_03_34_OfflineVictim(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	a := e.register("victim_1", nil)
	e.ack(a.AccessToken, a.RecoveryCodeGen)
	b := e.mustLogin("victim_1", goodPassword, "ios")
	x := e.mustLogin("victim_1", goodPassword, "web")
	want := clk.now.Add(7 * oneDay).UnixMilli()
	e.mustChangePassword(x.AccessToken, goodPassword, newPassword)
	clk.advance(time.Hour)
	c := e.connect("victim_1", a.AccessToken, testPlatform)
	ev := waitKicked(c, protocol.KickPasswordChanged, protocol.CloseTokenRevoked)
	if ev.RecoverBefore == nil || *ev.RecoverBefore != want {
		t.Errorf("kicked recoverBefore %v, want %d", ev.RecoverBefore, want)
	}
	clk.advance(25 * time.Hour)
	got := e.wantRefreshErr("access expired", b.RefreshToken, protocol.ReasonCredentialsChanged)
	if got.Details["recoverBefore"] != float64(want) {
		t.Errorf("4019 recoverBefore %v, want %d", got.Details["recoverBefore"], want)
	}
}

// TC-ACC-03-46（AC-ACC-03-44 的第一部分，RS5-18）：旧代值遇到 credentialsChanged 吊销的会话：不算重放。
func TestTC_ACC_03_46_OldGenAfterCredentialsChanged(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	r2 := e.mustRefresh(r1.RefreshToken)
	e.mustRefresh(r2.RefreshToken) // R1 已作废
	r3 := e.mustLogin("alice_1", goodPassword, "ios")
	clk.advance(3 * time.Minute)
	before := e.metric("im_refresh_replay_total")
	e.mustChangePassword(r3.AccessToken, goodPassword, newPassword)
	e.wantRefreshErr("old gen", r1.RefreshToken, protocol.ReasonCredentialsChanged)
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
}

// 改密接口的通用错误：缺 Bearer、被替换的 Bearer。
func TestPasswordBearerRequired(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	code, b := e.changePassword("", goodPassword, newPassword)
	wantErr(t, "no bearer", code, b, http.StatusUnauthorized, protocol.CodeTokenInvalid, "")
	r2 := e.mustRefresh(r1.RefreshToken)
	e.authOK("new", r2.AccessToken, testPlatform)
	code, b = e.changePassword(r1.AccessToken, goodPassword, newPassword)
	wantErr(t, "superseded", code, b, http.StatusUnauthorized, protocol.CodeTokenExpired, protocol.ReasonTokenSuperseded)
	if !strings.Contains(account.EventPasswordChanged, "password") {
		t.Fatal(account.EventPasswordChanged)
	}
}
