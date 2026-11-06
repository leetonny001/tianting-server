package e2e

// F-ACC-03 规则 3、4 的后端用例（BE-415）。时钟都是 [IM 钟]，窗口 IM_REFRESH_OVERLAP=2 分钟。

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// TC-ACC-03-01（AC-ACC-03-1）：正常刷新。新 access 能连接，它的代号成为 current。
func TestTC_ACC_03_01_Refresh(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	r2 := e.mustRefresh(r1.RefreshToken)
	if r2.AccessToken == r1.AccessToken || r2.RefreshToken == r1.RefreshToken || r2.AccessExpiresAt == 0 || r2.RefreshExpiresAt == 0 || r2.AbsoluteExpiresAt == 0 {
		t.Errorf("%+v", r2)
	}
	sid := sidOf(r1.RefreshToken)
	if d := e.session(sid); d.CurrentGen != 1 || len(d.Pendings) != 1 {
		t.Errorf("before use: current %d pendings %d", d.CurrentGen, len(d.Pendings))
	}
	e.authOK("alice_1", r2.AccessToken, testPlatform)
	if d := e.session(sid); d.CurrentGen != genOf(t, r2.RefreshToken) || len(d.Pendings) != 0 {
		t.Errorf("after use: current %d pendings %d", d.CurrentGen, len(d.Pendings))
	}
}

// TC-ACC-03-02（AC-ACC-03-2）：R2 转正，推进 3 分钟用 R1：重放，吊销整条会话，R2 和新 access 都失效。
func TestTC_ACC_03_02_ReplayRevokes(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	r2 := e.mustRefresh(r1.RefreshToken)
	r3 := e.mustRefresh(r2.RefreshToken)
	if d := e.session(sidOf(r1.RefreshToken)); d.OverlapUntil != clk.now.Add(2*time.Minute).UnixMilli() {
		t.Errorf("overlapUntil %d", d.OverlapUntil)
	}
	before := e.metric("im_refresh_replay_total")
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", r1.RefreshToken, protocol.ReasonReplayDetected)
	if got := e.metric("im_refresh_replay_total"); got != before+1 {
		t.Errorf("replay metric %v -> %v", before, got)
	}
	e.wantRefreshErr("after replay", r2.RefreshToken, protocol.ReasonReplayDetected)
	e.authKicked("alice_1", r3.AccessToken, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
}

// TC-ACC-03-03（AC-ACC-03-3）：待确认代只按数量淘汰。被淘汰的值窗口内 superseded，窗口外重放。
func TestTC_ACC_03_03_PendingEviction(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	build := func(name string) (r1, r2, r6 authResp) {
		r1 = e.register(name, nil)
		r2 = e.mustRefresh(r1.RefreshToken)
		var last authResp
		for range 4 {
			last = e.mustRefresh(r1.RefreshToken)
		}
		d := e.session(sidOf(r1.RefreshToken))
		if d.RevokedAt != 0 || len(d.Pendings) != 4 || d.Pendings[0].Gen != genOf(t, r2.RefreshToken)+1 {
			t.Fatalf("%s pendings: %+v", name, d)
		}
		return r1, r2, last
	}
	_, r2, r6 := build("alice_1")
	e.mustRefresh(r6.RefreshToken) // R6 转正
	clk.advance(time.Minute)
	got := e.wantRefreshErr("evicted in window", r2.RefreshToken, protocol.ReasonRefreshSuperseded)
	if got.Details["retryAfterSeconds"] == nil {
		t.Errorf("no retryAfterSeconds: %+v", got)
	}
	if d := e.session(sidOf(r2.RefreshToken)); d.RevokedAt != 0 {
		t.Error("superseded must not revoke")
	}
	_, r2b, r6b := build("bobby_1")
	e.mustRefresh(r6b.RefreshToken)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("evicted after window", r2b.RefreshToken, protocol.ReasonReplayDetected)
	if d := e.session(sidOf(r2b.RefreshToken)); d.RevokedAt == 0 {
		t.Error("must revoke after the window")
	}
}

// TC-ACC-03-04（AC-ACC-03-4）：91 天没用过期，不算重放；89 天还能用。
func TestTC_ACC_03_04_RefreshExpiry(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	android := e.register("alice_1", nil)
	ios := e.mustLogin("alice_1", goodPassword, "ios")
	before := e.metric("im_refresh_replay_total")
	clk.advance(89 * oneDay)
	e.mustRefresh(ios.RefreshToken)
	clk.advance(2 * oneDay)
	e.wantRefreshErr("expired", android.RefreshToken, protocol.ReasonRefreshExpired)
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric changed: %v", got)
	}
}

// TC-ACC-03-15（AC-ACC-03-15）：web 到期 24 小时；循环转正到期时间都不超过绝对上限；超过 365 天 401。
func TestTC_ACC_03_15_WebAbsolute(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	e.register("alice_1", nil)
	r := e.mustLogin("alice_1", goodPassword, "web")
	if want := clk.now.Add(24 * time.Hour).UnixMilli(); r.RefreshExpiresAt != want {
		t.Errorf("web refresh expiry %d, want %d", r.RefreshExpiresAt, want)
	}
	abs := r.AbsoluteExpiresAt
	if want := clk.now.Add(365 * oneDay).UnixMilli(); abs != want {
		t.Errorf("absolute %d, want %d", abs, want)
	}
	cur := r
	// 每步 23 小时，跑到距绝对上限不足 24 小时为止。
	for clk.now.Add(23*time.Hour).UnixMilli() < abs {
		clk.advance(23 * time.Hour)
		next := e.mustRefresh(cur.RefreshToken)
		if next.AccessExpiresAt > abs || next.RefreshExpiresAt > abs || next.AbsoluteExpiresAt != abs {
			t.Fatalf("over the absolute limit: %+v", next)
		}
		// 用新 access 转正。这里不连 WS，走 HTTP，省 381 次握手。
		if code, b := e.get("/v1/account/events", next.AccessToken); code != http.StatusOK {
			t.Fatalf("promote: %d %s", code, b)
		}
		cur = next
	}
	clk.now = time.UnixMilli(abs).Add(time.Minute)
	e.setClock(clk.now)
	e.wantRefreshErr("over absolute", cur.RefreshToken, protocol.ReasonRefreshExpired)
}

// TC-ACC-03-19（AC-ACC-03-19）：响应丢失，5 分钟、1 小时、3 天后用旧值都成功，指标增加。推送令牌部分 C3 上线时验。
func TestTC_ACC_03_19_ResponseLost(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	before := e.metric("im_refresh_reissue_total")
	e.mustRefresh(r1.RefreshToken)
	for _, d := range []time.Duration{5 * time.Minute, 55 * time.Minute, 71 * time.Hour} {
		clk.advance(d)
		e.mustRefresh(r1.RefreshToken)
	}
	if got := e.metric("im_refresh_reissue_total"); got != before+3 {
		t.Errorf("reissue %v -> %v", before, got)
	}
	if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 || d.CurrentGen != 1 {
		t.Errorf("%+v", d)
	}
}

// TC-ACC-03-21（AC-ACC-03-21）：同一 R1 两个并发：都不吊销，代号不同；先用的转正，另一个作废；只有一条链。
func TestTC_ACC_03_21_ConcurrentRefresh(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	var wg sync.WaitGroup
	out := make([]authResp, 2)
	for i := range out {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = e.mustRefresh(r1.RefreshToken)
		}()
	}
	wg.Wait()
	g0, g1 := genOf(t, out[0].RefreshToken), genOf(t, out[1].RefreshToken)
	if g0 == g1 {
		t.Fatalf("same gen %d", g0)
	}
	sid := sidOf(r1.RefreshToken)
	d := e.session(sid)
	if len(d.Pendings) != 2 || d.RevokedAt != 0 {
		t.Fatalf("%+v", d)
	}
	v := d.RowVersion
	e.authOK("a", out[0].AccessToken, testPlatform)
	e.authKicked("b", out[1].AccessToken, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	if d := e.session(sid); len(d.Pendings) != 0 || d.CurrentGen != g0 || d.RowVersion <= v {
		t.Errorf("%+v", d)
	}
}

// TC-ACC-03-22（AC-ACC-03-22）：1 分钟内重试 3 次都成功，用最新的转正，3 分钟后 R1 重放。
func TestTC_ACC_03_22_RetriesThenReplay(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	var r4 authResp
	for range 3 {
		r4 = e.mustRefresh(r1.RefreshToken)
		clk.advance(20 * time.Second)
		if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 {
			t.Fatal("revoked while retrying")
		}
	}
	e.authOK("alice_1", r4.AccessToken, testPlatform)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", r1.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-24（AC-ACC-03-24）：新 access auth 也算转正，之后 R1 按重放。
func TestTC_ACC_03_24_AccessPromotes(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	r2 := e.mustRefresh(r1.RefreshToken)
	e.authOK("alice_1", r2.AccessToken, testPlatform)
	if d := e.session(sidOf(r1.RefreshToken)); d.CurrentGen != genOf(t, r2.RefreshToken) {
		t.Errorf("current %d", d.CurrentGen)
	}
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", r1.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-25（AC-ACC-03-25）：被盗场景。受害者用 R1、R2 都触发吊销；事件只有 sid 的哈希；其他设备收到提醒；攻击者也失效。
func TestTC_ACC_03_25_StolenToken(t *testing.T) {
	for _, useOld := range []bool{true, false} {
		e := openEnv(t, nil)
		clk := e.freeze()
		r1 := e.register("victim_1", nil)
		d2 := e.mustLogin("victim_1", goodPassword, "ios")
		other := e.authOK("victim_d2", d2.AccessToken, "ios")
		r2 := e.mustRefresh(r1.RefreshToken) // D1 保存了 R2，没有使用
		x := e.mustRefresh(r1.RefreshToken)  // X 用偷来的 R1 重发
		xc := e.authOK("attacker", x.AccessToken, testPlatform)
		before := e.metric("im_refresh_replay_total")
		clk.advance(3 * time.Minute)
		used := r1.RefreshToken
		if !useOld {
			used = r2.RefreshToken
		}
		e.wantRefreshErr("victim", used, protocol.ReasonReplayDetected)
		if got := e.metric("im_refresh_replay_total"); got != before+1 {
			t.Errorf("replay metric %v -> %v", before, got)
		}
		evs := e.events("victim_1", account.EventRefreshReplay)
		sid := sidOf(r1.RefreshToken)
		if len(evs) != 1 || evs[0].SIDHash != account.SIDHash(sid) {
			t.Errorf("events: %+v", evs)
		}
		f := other.waitCmd(protocol.CmdEvent)
		if !strings.Contains(string(f.Data), `"securityNotice"`) || !strings.Contains(string(f.Data), account.EventRefreshReplay) {
			t.Errorf("d2: %s", f.Data)
		}
		e.wantRefreshErr("attacker", x.RefreshToken, protocol.ReasonReplayDetected)
		waitKicked(xc, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	}
}

// TC-ACC-03-27（AC-ACC-03-27）：过期 31 天的会话被清除，29 天的还在。绑定令牌部分 C3 上线时验。
func TestTC_ACC_03_27_SessionPurge(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	old := e.register("alice_1", nil) // 90 天后过期
	clk.advance(2 * oneDay)
	young := e.mustLogin("alice_1", goodPassword, "ios")
	clk.advance((90 + 31 - 2) * oneDay)
	if code, b := e.adminPost("/admin/test/jobs/session_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE sid = ?`, sidOf(old.RefreshToken)); n != 0 {
		t.Error("31 days past expiry must be purged")
	}
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE sid = ?`, sidOf(young.RefreshToken)); n != 1 {
		t.Error("29 days past expiry must stay")
	}
	// 行已删除：mac 正确，不算 invalid。
	e.wantRefreshErr("purged", old.RefreshToken, protocol.ReasonRefreshExpired)
}

// TC-ACC-03-31（AC-ACC-03-31）：乱序到达。转正后窗口内 R1、R4 都 superseded，不发 token；窗口后 R4 重放。
func TestTC_ACC_03_31_OutOfOrder(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	r3 := e.mustRefresh(r1.RefreshToken)
	clk.advance(time.Minute)
	r4 := e.mustRefresh(r1.RefreshToken)
	e.authOK("alice_1", r3.AccessToken, testPlatform)
	clk.advance(time.Minute)
	e.wantRefreshErr("r1", r1.RefreshToken, protocol.ReasonRefreshSuperseded)
	e.wantRefreshErr("r4", r4.RefreshToken, protocol.ReasonRefreshSuperseded)
	if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 {
		t.Error("revoked inside the window")
	}
	clk.advance(2 * time.Minute)
	e.wantRefreshErr("r4 late", r4.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-32（AC-ACC-03-32）：只知道 sid：refresh、logout 都 invalid，不带 recoverBefore，不吊销，不计重放。
func TestTC_ACC_03_32_SIDOnly(t *testing.T) {
	e := openEnv(t, nil)
	v := e.register("victim_1", nil)
	sid := sidOf(v.RefreshToken)
	forged := []string{
		"t1." + sid + ".1.bm9uY2U.AAAA",
		"nokey." + sid + ".1.bm9uY2U.AAAA",
		"garbage",
		"t1." + sid + ".abc.bm9uY2U." + strings.Split(v.RefreshToken, ".")[4], // 代号改了，mac 就不对了
		"t1." + sid + ".2." + strings.Split(v.RefreshToken, ".")[3] + "." + strings.Split(v.RefreshToken, ".")[4],
	}
	before := e.metric("im_refresh_replay_total")
	for _, f := range forged {
		got := e.wantRefreshErr("refresh "+f, f, protocol.ReasonRefreshInvalid)
		if got.Details["recoverBefore"] != nil {
			t.Errorf("invalid must not carry recoverBefore: %+v", got)
		}
		code, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": f})
		wantErr(t, "logout "+f, code, b, http.StatusUnauthorized, protocol.CodeRefreshTokenInvalid, protocol.ReasonRefreshInvalid)
	}
	if d := e.session(sid); d.RevokedAt != 0 {
		t.Error("revoked by a forged token")
	}
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
	if evs := e.events("victim_1", account.EventRefreshReplay); len(evs) != 0 {
		t.Errorf("events: %+v", evs)
	}
	e.mustRefresh(v.RefreshToken)
}

// TC-ACC-03-33（AC-ACC-03-33）：跑完注册、登录、刷新、退出、重放，日志和指标搜不到 sid 原值。后台响应在 BE-421 的用例里断言。
func TestTC_ACC_03_33_SIDNotLogged(t *testing.T) {
	var logs *syncBuffer
	e := openEnv(t, func(c *app.Config) { logs = captureLogs(c) })
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	login := e.mustLogin("alice_1", goodPassword, "ios")
	r2 := e.mustRefresh(r1.RefreshToken)
	e.mustRefresh(r2.RefreshToken)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", r1.RefreshToken, protocol.ReasonReplayDetected)
	if code, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": login.RefreshToken}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	metrics := e.metricsText()
	for _, sid := range []string{sidOf(r1.RefreshToken), sidOf(login.RefreshToken)} {
		if strings.Contains(logs.String(), sid) {
			t.Errorf("sid in logs: %s", sid)
		}
		if strings.Contains(metrics, sid) {
			t.Errorf("sid in metrics: %s", sid)
		}
	}
}

// TC-ACC-03-35（AC-ACC-03-35）：新 IP 前缀写事件（存前缀），同一前缀不再增加；账号安全带其他会话数量和最近活跃时间。
func TestTC_ACC_03_35_NewPrefix(t *testing.T) {
	e := openEnv(t, nil)
	code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("alice_1", nil))
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var r1 authResp
	mustJSON(t, b, &r1)
	other := e.mustLogin("alice_1", goodPassword, "ios")
	refreshFrom := func(ip, token string) authResp {
		code, b, _ := e.post("/v1/account/token/refresh", ip, map[string]any{"refreshToken": token})
		if code != http.StatusOK {
			t.Fatalf("%d %s", code, b)
		}
		var r authResp
		mustJSON(t, b, &r)
		return r
	}
	r2 := refreshFrom("198.51.100.1", r1.RefreshToken)
	evs := e.events("alice_1", account.EventNewIPPrefix)
	if len(evs) != 1 || evs[0].IPPrefix != "198.51.100.0/24" {
		t.Fatalf("%+v", evs)
	}
	if code, b := e.bearerDo(http.MethodGet, "/v1/account/events", r2.AccessToken, "198.51.100.1", nil); code != http.StatusOK || !hasAll(string(b), `"otherSessions":{"count":1`, `"lastActiveAt"`, account.EventNewIPPrefix, "198.51.100.0/24") {
		t.Errorf("%d %s", code, b)
	}
	refreshFrom("198.51.100.77", r2.RefreshToken)
	if n := len(e.events("alice_1", account.EventNewIPPrefix)); n != 1 {
		t.Errorf("same prefix added an event: %d", n)
	}
	// 用另一个前缀的 access 转正也比较。
	r3 := refreshFrom("198.51.100.7", other.RefreshToken)
	n := len(e.events("alice_1", account.EventNewIPPrefix))
	if code, _ := e.bearerDo(http.MethodGet, "/v1/account/events", r3.AccessToken, "192.0.2.9", nil); code != http.StatusOK {
		t.Fatal(code)
	}
	if got := len(e.events("alice_1", account.EventNewIPPrefix)); got != n+1 {
		t.Errorf("promotion from a new prefix wrote no event: %d -> %d", n, got)
	}
}

// TC-ACC-03-39（AC-ACC-03-37，服务端部分）：第 21 次 4290；重发不滑动，90 天后到期；距上限不足 24 小时时 access 不超过上限。SDK 部分随 FE-600～602。
func TestTC_ACC_03_39_IssueRateAndNoSlide(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	for i := range 20 {
		code, b := e.refresh(r1.RefreshToken)
		if code != http.StatusOK {
			t.Fatalf("round %d: %d %s", i, code, b)
		}
	}
	code, b := e.refresh(r1.RefreshToken)
	got := wantErr(t, "21st", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
	if got.Details["retryAfterSeconds"] == nil {
		t.Errorf("%s", b)
	}
	if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 {
		t.Error("rate limit must not revoke")
	}
	clk.advance(time.Hour)
	e.mustRefresh(r1.RefreshToken)

	// 重发不滑动：每 24 小时一次，第 90 天之后 4019。
	r := e.mustLogin("alice_1", goodPassword, "ios")
	exp := r.RefreshExpiresAt
	for clk.now.Add(oneDay).UnixMilli() < exp {
		clk.advance(oneDay)
		next := e.mustRefresh(r.RefreshToken)
		if next.RefreshExpiresAt != exp {
			t.Fatalf("reissue slid the expiry: %d -> %d", exp, next.RefreshExpiresAt)
		}
	}
	clk.advance(2 * oneDay)
	e.wantRefreshErr("no slide", r.RefreshToken, protocol.ReasonRefreshExpired)

	// 距绝对上限 23 小时刷新并转正。
	s := e.mustLogin("alice_1", goodPassword, "android")
	for clk.now.Add(80*day).UnixMilli() < s.AbsoluteExpiresAt {
		clk.advance(80 * oneDay)
		s = e.mustRefresh(s.RefreshToken)
		s = e.mustRefresh(s.RefreshToken) // 上一代转正，到期滑动
	}
	clk.now = time.UnixMilli(s.AbsoluteExpiresAt).Add(-23 * time.Hour)
	e.setClock(clk.now)
	last := e.mustRefresh(s.RefreshToken)
	if last.AccessExpiresAt != last.AbsoluteExpiresAt {
		t.Errorf("access %d, absolute %d", last.AccessExpiresAt, last.AbsoluteExpiresAt)
	}
}

// TC-ACC-03-38（AC-ACC-03-48）：攻击者用满次数压制不了重放判定（RS6-05）。
func TestTC_ACC_03_38_LimitOnlyOnIssueBranch(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("victim_1", nil)
	x := e.mustRefresh(r1.RefreshToken)
	e.authOK("attacker", x.AccessToken, testPlatform)
	cur := x
	var limited bool
	for range 25 {
		code, b := e.refresh(cur.RefreshToken)
		if code == http.StatusTooManyRequests {
			limited = true
			break
		}
		if code != http.StatusOK {
			t.Fatalf("%d %s", code, b)
		}
		var next authResp
		mustJSON(t, b, &next)
		cur = next
	}
	if !limited {
		t.Fatal("never limited")
	}
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("victim", r1.RefreshToken, protocol.ReasonReplayDetected)
	e.wantRefreshErr("attacker", cur.RefreshToken, protocol.ReasonReplayDetected)
	// 已吊销、mac 错误同样不受限流影响。
	e.wantRefreshErr("forged", "t1."+sidOf(r1.RefreshToken)+".1.bm9uY2U.AAAA", protocol.ReasonRefreshInvalid)
}

// TC-ACC-03-40（AC-ACC-03-38）：连续转正不能续开窗口（RS5-01）：第二次转正后 R1 立即重放，攻击者也失效。
func TestTC_ACC_03_40_ConsecutivePromotion(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("victim_1", nil)
	e.mustRefresh(r1.RefreshToken) // V 没有转正
	x2 := e.mustRefresh(r1.RefreshToken)
	e.authOK("x2", x2.AccessToken, testPlatform)
	sid := sidOf(r1.RefreshToken)
	if d := e.session(sid); d.OverlapMinGen != 1 {
		t.Errorf("overlapMinGen %d", d.OverlapMinGen)
	}
	clk.advance(110 * time.Second)
	x3 := e.mustRefresh(x2.RefreshToken)
	e.authOK("x3", x3.AccessToken, testPlatform)
	if d := e.session(sid); d.OverlapMinGen != genOf(t, x2.RefreshToken) {
		t.Errorf("overlapMinGen %d, want %d", d.OverlapMinGen, genOf(t, x2.RefreshToken))
	}
	clk.advance(5 * time.Second)
	e.wantRefreshErr("victim", r1.RefreshToken, protocol.ReasonReplayDetected)
	e.wantRefreshErr("attacker", x3.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-42（AC-ACC-03-40）：代号不复用，作废的 access 不会复活（RS5-05）。
func TestTC_ACC_03_42_GenNotReused(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("victim_1", nil)
	e.mustRefresh(r1.RefreshToken)               // A2
	a3 := e.mustRefresh(r1.RefreshToken)         // A3
	a4 := e.mustRefresh(r1.RefreshToken)         // A4
	e.authOK("v4", a4.AccessToken, testPlatform) // A2、A3 作废
	a5 := e.mustRefresh(a4.RefreshToken)         // 新一代
	e.authOK("v5", a5.AccessToken, testPlatform)
	e.authKicked("x", a3.AccessToken, protocol.KickTokenExpired, protocol.CloseTokenExpired)
	d := e.session(sidOf(r1.RefreshToken))
	if d.CurrentGen <= genOf(t, a3.RefreshToken) || d.NextGen <= d.CurrentGen {
		t.Errorf("%+v", d)
	}
}

// TC-ACC-03-46（AC-ACC-03-44）：logout 带被替换的 Bearer 和有效 refresh 能退出。已吊销的旧值不算重放的部分在 BE-417 的改密用例里。
func TestTC_ACC_03_46_LogoutWithSupersededBearer(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	r2 := e.mustRefresh(r1.RefreshToken)
	e.authOK("alice_1", r2.AccessToken, testPlatform)
	r3 := e.mustRefresh(r2.RefreshToken)
	before := e.metric("im_refresh_replay_total")
	code, b := e.bearerPost("/v1/account/logout", r1.AccessToken, e.freshIP(), map[string]any{"refreshToken": r3.RefreshToken})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.wantRefreshErr("after logout", r3.RefreshToken, protocol.ReasonRefreshRevoked)
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
}

// TC-ACC-03-47（AC-ACC-03-45）：连续 6 小时刷满：每小时第 21 次 4290，有事件，第 6 小时其他在线设备收到提醒。
func TestTC_ACC_03_47_SustainedLimit(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r1 := e.register("alice_1", nil)
	d2 := e.mustLogin("alice_1", goodPassword, "ios")
	c := e.authOK("d2", d2.AccessToken, "ios")
	for hour := 1; hour <= 6; hour++ {
		for i := range 21 {
			code, b := e.refresh(r1.RefreshToken)
			if want := http.StatusOK; i == 20 {
				wantErr(t, "21st", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
			} else if code != want {
				t.Fatalf("hour %d round %d: %d %s", hour, i, code, b)
			}
		}
		if hour == 5 {
			c.expectNoMsg(100 * time.Millisecond)
			if len(c.pending) != 0 {
				t.Fatalf("notified too early: %s", c.pending[0].Data)
			}
		}
		if hour < 6 {
			clk.advance(time.Hour)
		}
	}
	if n := len(e.events("alice_1", account.EventRefreshRateLimited)); n != 6 {
		t.Errorf("events %d", n)
	}
	f := c.waitCmd(protocol.CmdEvent)
	if !strings.Contains(string(f.Data), account.EventRefreshRateLimited) {
		t.Errorf("%s", f.Data)
	}
	if d := e.session(sidOf(r1.RefreshToken)); d.RevokedAt != 0 {
		t.Error("must not revoke")
	}
}

// TC-ACC-03-48（AC-ACC-03-46）：轮换后旧 token 可用；提前删旧密钥配置检查报错；生产配置拒绝启动；366 天后通过。
func TestTC_ACC_03_48_KeyLedger(t *testing.T) {
	path := t.TempDir() + "/im.db"
	cfg := func(active string, keys ...string) func(*app.Config) {
		return func(c *app.Config) {
			c.DBPath = path
			c.Account.RefreshKeys = nil
			for _, k := range keys {
				c.Account.RefreshKeys = append(c.Account.RefreshKeys, account.RefreshKey{ID: k, Secret: []byte("refresh-key-" + k + "-0123456789abcdef")})
			}
			c.Account.RefreshActive = active
			c.Account.RegisterMode = account.ModeOpen
		}
	}
	e1 := newAcctEnv(t, cfg("k1", "k1"))
	r1 := e1.register("alice_1", nil)
	if !strings.HasPrefix(r1.RefreshToken, "k1.") {
		t.Fatal(r1.RefreshToken)
	}
	e2 := newAcctEnv(t, cfg("k2", "k1", "k2"))
	r2 := e2.mustRefresh(r1.RefreshToken)
	if !strings.HasPrefix(r2.RefreshToken, "k2.") {
		t.Errorf("new token kid: %s", r2.RefreshToken)
	}
	st, err := store.OpenSQLite(path, store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if entries, err := st.RefreshKeyLedger(context.Background()); err != nil || len(entries) != 2 || entries[0].RetiredAt == 0 || entries[1].RetiredAt != 0 {
		t.Fatalf("%+v %v", entries, err)
	}
	// 配置里删除 k1：配置检查报错，生产配置拒绝启动。
	base := func(c *app.Config) {
		c.AdminKey = adminKey
		c.JWTSecret = "e2e-secret-0123456789"
		c.Logger = discardLogger()
		c.AutoMigrate = true
		c.Gateway = defaultGateway()
	}
	strict := app.Config{}
	base(&strict)
	cfg("k2", "k2")(&strict)
	strict.Account.EnforceKeyLedger = true
	if _, err := app.New(strict); err == nil || !strings.Contains(err.Error(), "k1") {
		t.Fatalf("production must refuse: %v", err)
	}
	warn := strict
	warn.Account.EnforceKeyLedger = false
	a, err := app.New(warn)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.DynamicChecks(context.Background()); !checkFailed(got, "refresh_key_ledger") {
		t.Errorf("%+v", got)
	}
	a.Close()
	// retiredAt 推到 366 天前：通过。
	if err := execLedger(path, time.Now().Add(-366*day).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	a, err = app.New(strict)
	if err != nil {
		t.Fatalf("after 366 days: %v", err)
	}
	if got := a.DynamicChecks(context.Background()); checkFailed(got, "refresh_key_ledger") {
		t.Errorf("%+v", got)
	}
	a.Close()
	// 泄露应急：明确放弃未满 365 天的密钥后不再拦。
	if err := st.SyncRefreshKeyLedger(context.Background(), []string{"k3"}, "k2", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.New(strict); err == nil {
		t.Fatal("k3 was retired just now")
	}
	if err := st.ForgetRefreshKey(context.Background(), "k3", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	a, err = app.New(strict)
	if err != nil {
		t.Fatalf("forgotten: %v", err)
	}
	a.Close()
}

// TC-ACC-03-49（AC-ACC-03-49）：吊销原因映射。自己退出、强制下线、封禁、过期、重放依次 revoked、revoked、revoked、expired、replayDetected。
func TestTC_ACC_03_49_RevokeReasons(t *testing.T) {
	e := openEnv(t, func(c *app.Config) {
		c.V13AdminFeatures = true
		c.QuarantineKey = "e2e-quarantine-key-0123456789abcdef"
	})
	clk := e.freeze()
	logout := e.register("logout_1", nil)
	if code, b, _ := e.post("/v1/account/logout", e.freshIP(), map[string]any{"refreshToken": logout.RefreshToken}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.wantRefreshErr("logout", logout.RefreshToken, protocol.ReasonRefreshRevoked)
	forced := e.register("forced_1", nil)
	if code, b := e.adminPost("/admin/users/forced_1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.wantRefreshErr("forced", forced.RefreshToken, protocol.ReasonRefreshRevoked)
	banned := e.register("banned_1", nil)
	if err := e.st.BanUser(context.Background(), store.Ban{UserID: "banned_1", Reason: "test", By: "t", At: clk.now.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	e.wantRefreshErr("banned", banned.RefreshToken, protocol.ReasonRefreshRevoked)
	expired := e.register("expired_1", nil)
	replayed := e.register("replayed_1", nil)
	r2 := e.mustRefresh(replayed.RefreshToken)
	e.mustRefresh(r2.RefreshToken)
	clk.advance(91 * oneDay)
	e.wantRefreshErr("expired", expired.RefreshToken, protocol.ReasonRefreshExpired)
	// 重放要在有效期内：replayed 也过期了，换一个。
	fresh := e.mustLogin("replayed_1", goodPassword, "ios")
	f2 := e.mustRefresh(fresh.RefreshToken)
	e.mustRefresh(f2.RefreshToken)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", fresh.RefreshToken, protocol.ReasonReplayDetected)
	e.wantRefreshErr("replay again", fresh.RefreshToken, protocol.ReasonReplayDetected)
}

// TC-ACC-03-50（AC-ACC-03-50）：sessions revoke-all：全部 reason=revoked，access 收到 sessionRevoked，不计重放；用密码重新登录成功。
func TestTC_ACC_03_50_RevokeAll(t *testing.T) {
	e := openEnv(t, nil)
	u1, u2 := e.register("alice_1", nil), e.register("bobby_1", nil)
	before := e.metric("im_refresh_replay_total")
	n, err := e.st.RevokeAllDeviceSessions(context.Background(), "revoked", time.Now().UnixMilli())
	if err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	for _, r := range []authResp{u1, u2} {
		e.wantRefreshErr("refresh", r.RefreshToken, protocol.ReasonRefreshRevoked)
		e.authKicked("auth", r.AccessToken, protocol.KickSessionRevoked, protocol.CloseTokenRevoked)
	}
	if got := e.metric("im_refresh_replay_total"); got != before {
		t.Errorf("replay metric %v -> %v", before, got)
	}
	e.authOK("alice_1", e.mustLogin("alice_1", goodPassword, "ios").AccessToken, "ios")
}

// token/refresh 每 IP 每分钟 60 次（规则 3）。
func TestRefreshIPRate(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.Account.RefreshIPRate = 3 })
	for range 3 {
		code, _, _ := e.post("/v1/account/token/refresh", "203.0.113.9", map[string]any{"refreshToken": "x"})
		if code != http.StatusUnauthorized {
			t.Fatal(code)
		}
	}
	code, b, _ := e.post("/v1/account/token/refresh", "203.0.113.9", map[string]any{"refreshToken": "x"})
	wantErr(t, "4th", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
}
