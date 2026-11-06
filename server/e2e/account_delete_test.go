package e2e

// F-ACC-08 注销账号（BE-420）和 AC-ACC-04-22 的共用计数。

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

func (e *acctEnv) cleanup() {
	e.t.Helper()
	if code, b := e.adminPost("/admin/test/jobs/account_cleanup/run", map[string]any{}, adminKey); code != http.StatusOK {
		e.t.Fatalf("cleanup: %d %s", code, b)
	}
}

func (e *acctEnv) deleteAccount(bearer, password string) (int, []byte) {
	e.t.Helper()
	return e.bearerPost("/v1/account/delete", bearer, e.freshIP(), map[string]any{"password": password})
}

func (e *acctEnv) mustDelete(bearer, password string) {
	e.t.Helper()
	if code, b := e.deleteAccount(bearer, password); code != http.StatusOK {
		e.t.Fatalf("delete: %d %s", code, b)
	}
}

// TC-ACC-08-01（AC-ACC-08-1）：错密码 401；对了全部踢下线；第 3 天登录 403 带 deleteAt；旧 token accountDeleted；第 8 天清理后登录 401，注册 409。
func TestTC_ACC_08_01_DeleteFlow(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.register("alice_1", nil)
	d1 := e.authOK("d1", r.AccessToken, testPlatform)
	code, b := e.deleteAccount(r.AccessToken, "wrong-password-1")
	wantErr(t, "wrong", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	d1.expectNoMsg(50 * time.Millisecond)
	e.mustDelete(r.AccessToken, goodPassword)
	waitKicked(d1, protocol.KickAccountDeleted, protocol.CloseTokenRevoked)
	requested := clk.now.UnixMilli()
	clk.advance(3 * oneDay)
	code, b = e.login("alice_1", goodPassword, e.freshIP())
	got := wantErr(t, "day 3", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountDeletionPending)
	if got.Details["deleteAt"] != float64(requested+7*oneDay.Milliseconds()) || strings.Contains(string(b), "accessToken") {
		t.Errorf("%s", b)
	}
	e.authKicked("old", r.AccessToken, protocol.KickAccountDeleted, protocol.CloseTokenRevoked)
	e.wantRefreshErr("old refresh", r.RefreshToken, protocol.ReasonAccountDeleted)
	clk.advance(5 * oneDay)
	e.cleanup()
	code, b = e.login("alice_1", goodPassword, e.freshIP())
	wantErr(t, "day 8 login", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	code, b, _ = e.post("/v1/account/register", e.freshIP(), regBody("alice_1", nil))
	wantErr(t, "day 8 register", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
	if n := e.count(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = 'alice_1'`); n != 0 {
		t.Errorf("recovery codes %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE user_id = 'alice_1'`); n != 0 {
		t.Errorf("sessions %d", n)
	}
}

// TC-ACC-08-02（AC-ACC-08-2）：注销到期后历史还在，昵称"已注销用户"，发消息 4042，签发 404。
func TestTC_ACC_08_02_AfterCleanup(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	u1, u2 := e.register("alice_1", nil), e.register("bobby_1", nil)
	e.createGroup("g1", "alice_1", "bobby_1")
	c1 := e.authOK("u1", u1.AccessToken, testPlatform)
	if f := c1.sendText("group", "g1", "msg-0001", "hello"); f.Error != nil {
		t.Fatalf("%+v", f.Error)
	}
	if f := c1.sendText("c2c", "bobby_1", "msg-0002", "hi"); f.Error != nil {
		t.Fatalf("%+v", f.Error)
	}
	e.mustDelete(u1.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.cleanup()
	login := e.mustLogin("bobby_1", goodPassword, testPlatform)
	_ = u2
	c2 := e.authOK("u2", login.AccessToken, testPlatform)
	if n := e.count(`SELECT COUNT(*) FROM messages WHERE from_uid = 'alice_1' AND type = 'text'`); n != 2 {
		t.Errorf("messages %d", n)
	}
	var nick string
	if err := e.rawDB().QueryRow(`SELECT nickname FROM users WHERE user_id = 'alice_1'`).Scan(&nick); err != nil || nick != "已注销用户" {
		t.Errorf("%q %v", nick, err)
	}
	if f := c2.sendText("c2c", "alice_1", "msg-0003", "hi"); f.Error == nil || f.Error.Code != protocol.CodeUserNotFound {
		t.Errorf("%+v", f.Error)
	}
	code, b := e.adminPost("/admin/users/token", map[string]any{"userId": "alice_1"}, adminKey)
	wantErr(t, "issue", code, b, http.StatusNotFound, protocol.CodeUserNotFound, "")
}

// TC-ACC-08-03（AC-ACC-08-3）：群主注销：自建群转给最早的管理员，托管群回到无群主。
func TestTC_ACC_08_03_Owner(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	u1 := e.register("owner_1", nil)
	e.register("admin_1", nil)
	e.register("member_1", nil)
	e.createGroup("g2", "owner_1", "member_1") // 托管群
	e.rawExec(`UPDATE chat_groups SET owner_id = 'owner_1' WHERE group_id = 'g2'`)
	e.rawExec(`UPDATE group_members SET role = 'owner' WHERE group_id = 'g2' AND user_id = 'owner_1'`)
	// 自建群：created_by 不为空（Managed() 为假）。
	const g1 = "g1"
	e.createGroup(g1, "owner_1", "admin_1", "member_1")
	e.rawExec(`UPDATE chat_groups SET owner_id = 'owner_1', created_by = 'owner_1' WHERE group_id = ?`, g1)
	e.rawExec(`UPDATE group_members SET role = 'owner' WHERE group_id = ? AND user_id = 'owner_1'`, g1)
	e.rawExec(`UPDATE group_members SET role = 'admin', role_at = 1 WHERE group_id = ? AND user_id = 'admin_1'`, g1)
	e.mustDelete(u1.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.cleanup()
	var owner string
	if err := e.rawDB().QueryRow(`SELECT owner_id FROM chat_groups WHERE group_id = ?`, g1).Scan(&owner); err != nil || owner != "admin_1" {
		t.Errorf("g1 owner %q %v", owner, err)
	}
	if err := e.rawDB().QueryRow(`SELECT owner_id FROM chat_groups WHERE group_id = 'g2'`).Scan(&owner); err != nil || owner != "" {
		t.Errorf("g2 owner %q %v", owner, err)
	}
	if n := e.count(`SELECT COUNT(*) FROM group_members WHERE user_id = 'owner_1'`); n != 0 {
		t.Errorf("still in %d groups", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM messages WHERE conv_id = ? AND type = 'tip' AND body LIKE '%ownerChanged%'`, protocol.GroupConvID(g1)); n != 1 {
		t.Errorf("ownerChanged tips %d", n)
	}
}

// TC-ACC-08-04（AC-ACC-08-4）：匿名撤销：错密码 401；对了可以登录，密码不变；清理不处理。
func TestTC_ACC_08_04_Cancel(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.register("alice_1", nil)
	e.mustDelete(r.AccessToken, goodPassword)
	clk.advance(3 * oneDay)
	code, b, _ := e.post("/v1/account/delete/cancel", e.freshIP(), map[string]any{"username": "alice_1", "password": "wrong-password-1"})
	wantErr(t, "wrong", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	code, nb, _ := e.post("/v1/account/delete/cancel", e.freshIP(), map[string]any{"username": "nouser_1", "password": "wrong-password-1"})
	if code != http.StatusUnauthorized || string(nb) != string(b) {
		t.Errorf("not the same response: %d %s / %s", code, nb, b)
	}
	if code, b, _ := e.post("/v1/account/delete/cancel", e.freshIP(), map[string]any{"username": "alice_1", "password": goodPassword}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.mustLogin("alice_1", goodPassword, "ios")
	clk.advance(5 * oneDay)
	e.cleanup()
	e.mustLogin("alice_1", goodPassword, "ios")
}

// TC-ACC-08-05（AC-ACC-08-5，服务端部分）：600 个群的清理中途失败可继续，最终全部退出，没有重复提示。P99 只作开发机参考，见报告。
func TestTC_ACC_08_05_ResumeCleanup(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	u := e.register("alice_1", nil)
	e.register("bobby_1", nil)
	groups := 600
	if testing.Short() {
		groups = 60
	}
	for i := range groups {
		e.createGroup(fmt.Sprintf("g%d", i), "alice_1", "bobby_1")
	}
	e.rawExec(`UPDATE chat_groups SET owner_id = 'alice_1'`)
	e.rawExec(`UPDATE group_members SET role = 'owner' WHERE user_id = 'alice_1'`)
	e.mustDelete(u.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.fault(map[string]any{"cleanupFailAfter": groups / 2})
	if code, _ := e.adminPost("/admin/test/jobs/account_cleanup/run", map[string]any{}, adminKey); code == http.StatusOK {
		t.Log("the job endpoint reports the failure in its body")
	}
	left := e.count(`SELECT COUNT(*) FROM group_members WHERE user_id = 'alice_1'`)
	if left == 0 || left == groups {
		t.Fatalf("not interrupted: %d left", left)
	}
	e.cleanup()
	if n := e.count(`SELECT COUNT(*) FROM group_members WHERE user_id = 'alice_1'`); n != 0 {
		t.Errorf("left %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM (SELECT conv_id FROM messages WHERE type = 'tip' AND body LIKE '%ownerChanged%' GROUP BY conv_id HAVING COUNT(*) > 1)`); n != 0 {
		t.Errorf("duplicate tips in %d groups", n)
	}
	if n := e.count(`SELECT cleanup_done_at FROM accounts WHERE user_id = 'alice_1'`); n == 0 {
		t.Error("cleanup not finished")
	}
}

// TC-ACC-05-15（AC-ACC-05-16，RC2-02）：攻击者改密后申请注销：第 3 天旧码找回，注销撤销，第 8 天清理不处理；攻击者的 token 都失效。
func TestTC_ACC_05_15_RecoverCancelsDeletion(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	v := e.regAcked("victim_1")
	x0 := e.mustLogin("victim_1", goodPassword, "ios")
	x := e.mustChangePassword(x0.AccessToken, goodPassword, "Xx-pass-2026-a!")
	e.mustDelete(x.AccessToken, "Xx-pass-2026-a!")
	clk.advance(3 * oneDay)
	r := e.mustRecover("victim_1", v.RecoveryCode, newPassword)
	if n := e.count(`SELECT delete_requested_at FROM accounts WHERE user_id = 'victim_1'`); n != 0 {
		t.Errorf("delete_requested_at %d", n)
	}
	clk.advance(5 * oneDay)
	e.cleanup()
	_ = r
	e.authOK("victim", e.mustLogin("victim_1", newPassword, "ios").AccessToken, "ios")
	// 攻击者的会话是申请注销时吊销的：第一个吊销原因保留，日期按账号状态算（规则 6a）。
	e.wantRefreshErr("attacker", x.RefreshToken, protocol.ReasonAccountDeleted)
	e.authKicked("attacker", x.AccessToken, protocol.KickAccountDeleted, protocol.CloseTokenRevoked)
}

// TC-ACC-04-21（AC-ACC-04-22，RS4-11）：logout-others、recovery-code、delete 和登录共用计数，第 6 次正确密码也 429。
func TestTC_ACC_04_21_SharedCounter(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	const ip = "203.0.113.50"
	wrong := map[string]any{"password": "wrong-password-1"}
	for i, path := range []string{"logout-others", "recovery-code", "delete", "logout-others", "recovery-code"} {
		code, b := e.bearerPost("/v1/account/"+path, r.AccessToken, ip, wrong)
		wantErr(t, fmt.Sprint(i, path), code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	}
	code, b := e.bearerPost("/v1/account/delete", r.AccessToken, ip, map[string]any{"password": goodPassword})
	wantErr(t, "6th", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	if n := e.count(`SELECT delete_requested_at FROM accounts WHERE user_id = 'alice_1'`); n != 0 {
		t.Errorf("deleted: %d", n)
	}
}

// TC-ACC-04-24 序列乙（RS5-03）：改密后注销，受害者第 1 天上线：kicked/accountDeleted 和 4019 都带第 7 天。
func TestTC_ACC_04_24_DeleteSequence(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	day7 := clk.now.Add(7 * oneDay).UnixMilli()
	v := e.regAcked("victim_1")
	x0 := e.mustLogin("victim_1", goodPassword, "ios")
	x := e.mustChangePassword(x0.AccessToken, goodPassword, newPassword)
	e.mustDelete(x.AccessToken, newPassword)
	clk.advance(oneDay)
	ev := waitKicked(e.connect("victim", v.AccessToken, testPlatform), protocol.KickAccountDeleted, protocol.CloseTokenRevoked)
	if ev.RecoverBefore == nil || *ev.RecoverBefore != day7 {
		t.Errorf("kicked %v, want %d", ev.RecoverBefore, day7)
	}
	got := e.refreshErrAny(v.RefreshToken)
	if got.Details["recoverBefore"] != float64(day7) {
		t.Errorf("4019 %+v, want %d", got, day7)
	}
}

// TC-ACC-01-07 的注销前置：走真实的注销和 account_cleanup（前一段用 MarkUserDeleted 代替）。
func TestTC_ACC_01_07_DeletedUsernameViaCleanup(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.register("gone_1", nil)
	e.mustDelete(r.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.cleanup()
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("gone_1", nil))
	wantErr(t, "deleted name", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
	code, b, _ = e.post("/v1/account/register", e.freshIP(), regBody("GONE_1", nil))
	wantErr(t, "case", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
}

// F-ACC-04 规则 8：第 ① 步的条件包含"注销申请没有被撤销"。列出到期账号之后申请被撤销，第 ① 步不做。
func TestFinishDeletionRespectsCancel(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	e.mustDelete(r.AccessToken, goodPassword)
	now := time.Now().Add(8 * oneDay).UnixMilli()
	due, err := e.st.DueDeletions(t.Context(), now, 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("%v %v", due, err)
	}
	e.rawExec(`UPDATE accounts SET delete_requested_at = 0 WHERE user_id = 'alice_1'`)
	ok, err := e.st.FinishDeletion(t.Context(), "alice_1", now, now)
	if err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
	e.mustLogin("alice_1", goodPassword, "ios")
}
