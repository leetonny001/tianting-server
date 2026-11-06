package e2e

// 登录和账号事件（PRD-chat F-ACC-02，BE-414），以及 /v1 基座的真实接口用例（AC-CHAT-90-16）。

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// ackLatest 确认最新一代恢复码，这样登录响应不再带 recoveryCode（F-ACC-05 规则 1）。
func (e *acctEnv) ackLatest(uid string, gen int64) {
	e.t.Helper()
	if err := e.st.AckRecoveryCode(e.t.Context(), uid, gen, 1); err != nil {
		e.t.Fatal(err)
	}
}

// TC-ACC-02-01（AC-ACC-02-1）：大小写不同的用户名能登录，token 可以连接；最新一代已确认时响应不带 recoveryCode。
func TestTC_ACC_02_01_Login(t *testing.T) {
	e := openEnv(t, nil)
	e.ackLatest(e.register("alice_1", nil).UserID, 1)
	code, b := e.login("ALICE_1", goodPassword, e.freshIP())
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	validateHTTP(t, "AuthResponse", b)
	var r authResp
	json.Unmarshal(b, &r)
	if r.UserID != "alice_1" || r.RecoveryCode != "" || r.RecoveryCodeGen != 0 {
		t.Errorf("%s", b)
	}
	if strings.Contains(string(b), "recoveryCode") {
		t.Errorf("no recoveryCode fields: %s", b)
	}
	c := e.connect("alice_1", r.AccessToken, "ios")
	defer c.ws.CloseNow()
	if len(c.pending) != 0 {
		t.Errorf("auth rejected: %+v", c.pending)
	}
}

// F-ACC-05 规则 1：最新一代没有确认时登录：那一代作废，新的一代在响应里返回，旧码不能再用来确认。
func TestLoginReissuesUnackedRecoveryCode(t *testing.T) {
	e := openEnv(t, nil)
	reg := e.register("alice_1", nil)
	r := e.mustLogin("alice_1", goodPassword, "ios")
	if r.RecoveryCode == "" || r.RecoveryCode == reg.RecoveryCode || r.RecoveryCodeGen != 2 {
		t.Fatalf("login: %+v", r)
	}
	if n := e.count(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = 'alice_1' AND gen = 1 AND revoked_at <> 0`); n != 1 {
		t.Error("the unacked generation must be revoked")
	}
	if err := e.st.AckRecoveryCode(t.Context(), "alice_1", 1, 1); err != store.ErrRecoveryCodeStale {
		t.Errorf("ack of an old generation: %v", err)
	}
	e.ackLatest("alice_1", 2)
	if r := e.mustLogin("alice_1", goodPassword, "ios"); r.RecoveryCode != "" {
		t.Errorf("acked: %+v", r)
	}
}

// F-ACC-03 规则 1：sid 至少 128 位随机；每次登录一条新会话，互不相同。
func TestLoginCreatesDistinctSessions(t *testing.T) {
	e := openEnv(t, nil)
	reg := e.register("alice_1", nil)
	seen := map[string]bool{strings.Split(reg.RefreshToken, ".")[1]: true}
	for range 3 {
		sid := strings.Split(e.mustLogin("alice_1", goodPassword, "web").RefreshToken, ".")[1]
		if len(sid) < 22 || seen[sid] {
			t.Errorf("sid %q", sid)
		}
		seen[sid] = true
	}
	if n := e.count(`SELECT COUNT(*) FROM device_sessions WHERE user_id = 'alice_1'`); n != 4 {
		t.Errorf("sessions %d", n)
	}
}

// TC-ACC-02-02（AC-ACC-02-2）：四种登录失败的响应完全相同（状态码、响应体、响应头名集合）。
func TestTC_ACC_02_02_IdenticalFailures(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.DemoLogin = true })
	if code, b := e.publicPost("/demo/login", map[string]any{"username": "demo1"}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.register("gone1", nil)
	if err := e.st.MarkUserDeleted(t.Context(), "gone1", 1); err != nil {
		t.Fatal(err)
	}
	e.register("real1", nil)
	type resp struct {
		status        int
		body, headers string
	}
	var got []resp
	for _, u := range []string{"nouser1", "demo1", "gone1", "real1"} {
		code, b, h := e.post("/v1/account/login", e.freshIP(), map[string]any{"username": u, "password": "wrong-password-1", "platform": "ios"})
		var names []string
		for k := range h {
			if k != "Date" {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		got = append(got, resp{code, string(b), strings.Join(names, ",")})
	}
	for i, r := range got {
		if r != got[0] {
			t.Errorf("failure %d differs:\n%+v\n%+v", i, r, got[0])
		}
	}
	wantErr(t, "login", got[0].status, []byte(got[0].body), http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
}

// TC-ACC-02-03（AC-ACC-02-3）：时间侧信道：不存在账号和存在账号密码错误，交替 50 次，耗时中位数相差不超过 5%。真实参数，关闭锁定。
func TestTC_ACC_02_03_TimingEqual(t *testing.T) {
	e := openEnv(t, func(c *app.Config) {
		c.Account.Argon = timingArgon()
		c.Account.TestDisableLock = true
	})
	e.register("real1", nil)
	var missing, wrong []time.Duration
	measure := func(user string) time.Duration {
		start := time.Now()
		code, b, _ := e.post("/v1/account/login", "203.0.113.1", map[string]any{"username": user, "password": "wrong-password-1", "platform": "ios"})
		d := time.Since(start)
		if code != http.StatusUnauthorized || parseErr(t, b).Code != protocol.CodeAccountCredentials {
			t.Fatalf("%s: %d %s", user, code, b)
		}
		return d
	}
	// 机器忙时噪声可能超过 5%，最多测 3 轮（每轮都是 AC 要求的各 50 次）：真正的差异每一轮都超标。
	for range 3 {
		missing, wrong = missing[:0], wrong[:0]
		for range 50 {
			missing = append(missing, measure("nouser1"))
			wrong = append(wrong, measure("real1"))
		}
		if medianClose(missing, wrong, timingTolerance) {
			break
		}
	}
	median := func(v []time.Duration) time.Duration {
		s := slices.Clone(v)
		slices.Sort(s)
		return s[len(s)/2]
	}
	a, b := median(missing), median(wrong)
	diff := max(a, b) - min(a, b)
	t.Logf("median: missing %v, wrong password %v, diff %v", a, b, diff)
	if float64(diff) > 0.05*float64(max(a, b)) {
		t.Errorf("medians differ by more than 5%%: missing %v, wrong password %v", a, b)
	}
}

// TC-ACC-02-04（AC-ACC-02-4）：连续 5 次失败后锁定，第 6 次正确密码也 429，15 分钟后恢复。
func TestTC_ACC_02_04_Lockout(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real1", nil)
	now := shanghaiMidnight(t)
	e.setClock(now)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	const ip = "203.0.113.1"
	for i := range 5 {
		code, b := e.login("real1", "wrong-password-1", ip)
		wantErr(t, fmt.Sprint("failure ", i), code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	}
	code, b := e.login("real1", goodPassword, ip)
	er := wantErr(t, "6th", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	if v, _ := er.Details["retryAfterSeconds"].(float64); v != 900 {
		t.Errorf("retryAfterSeconds %v", er.Details)
	}
	e.setClock(now.Add(14*time.Minute + 59*time.Second))
	code, b = e.login("real1", goodPassword, ip)
	wantErr(t, "14:59", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	e.setClock(now.Add(15*time.Minute + time.Second))
	if code, b := e.login("real1", goodPassword, ip); code != http.StatusOK {
		t.Errorf("after 15 minutes: %d %s", code, b)
	}
	// 成功之后这个 IP 的失败计数清零。
	if code, b := e.login("real1", "wrong-password-1", ip); code != http.StatusUnauthorized {
		t.Errorf("after success: %d %s", code, b)
	}
}

// TC-ACC-02-05（AC-ACC-02-5）：锁定按"用户名 + IP"，换 IP 用正确密码成功；原来的 IP 仍然 429。
func TestTC_ACC_02_05_LockIsPerIP(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real1", nil)
	for range 5 {
		e.login("real1", "wrong-password-1", "203.0.113.1")
	}
	if code, b := e.login("real1", goodPassword, "203.0.113.2"); code != http.StatusOK {
		t.Errorf("other ip: %d %s", code, b)
	}
	code, b := e.login("real1", goodPassword, "203.0.113.1")
	wantErr(t, "same ip", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
}

// TC-ACC-02-06（AC-ACC-02-6）：封禁用户：密码错 401（和普通失败相同），密码对 403 带 bannedUntil，没有 token。
func TestTC_ACC_02_06_Banned(t *testing.T) {
	e := openEnv(t, func(c *app.Config) {
		c.V13AdminFeatures = true
		c.QuarantineKey = "e2e-quarantine-key-0123456789abcdef"
	})
	e.register("real1", nil)
	until := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	if err := e.st.BanUser(t.Context(), store.Ban{UserID: "real1", Until: &until, Reason: "test", By: "t", At: 1}); err != nil {
		t.Fatal(err)
	}
	ip := e.freshIP()
	code, b := e.login("real1", "wrong-password-1", ip)
	wantErr(t, "wrong password", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	code, b = e.login("real1", goodPassword, ip)
	er := wantErr(t, "right password", code, b, http.StatusForbidden, protocol.CodeUserBanned, "")
	if v, _ := er.Details["bannedUntil"].(float64); int64(v) != until || strings.Contains(string(b), "accessToken") {
		t.Errorf("%s", b)
	}
	// 封禁同时吊销注册时的那条会话（F-ACC-04 规则 4，BE-417 起），所以这里只数行。
	if n := e.count(`SELECT COUNT(*) FROM device_sessions`); n != 1 {
		t.Errorf("banned login must not create a session: %d", n)
	}
}

// F-ACC-02 规则 1：注销冷静期内密码正确：403 / 4030 accountDeletionPending，details.deleteAt，没有 token；密码错误 401。
func TestLoginDuringDeletionCoolingOff(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real1", nil)
	const requested = int64(1_800_000_000_000)
	e.rawExec(`UPDATE accounts SET delete_requested_at = ? WHERE user_id = 'real1'`, requested)
	ip := e.freshIP()
	code, b := e.login("real1", "wrong-password-1", ip)
	wantErr(t, "wrong", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	code, b = e.login("real1", goodPassword, ip)
	er := wantErr(t, "pending", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountDeletionPending)
	if v, _ := er.Details["deleteAt"].(float64); int64(v) != requested+7*24*3600*1000 {
		t.Errorf("%s", b)
	}
}

// TC-ACC-02-07（AC-ACC-02-7）：Android 已登录，再从 iOS 登录，不踢；两条连接都收到消息，sid 不同。
func TestTC_ACC_02_07_MultiDevice(t *testing.T) {
	e := openEnv(t, nil)
	reg := e.register("alice_1", nil)
	d1 := e.connect("alice_1", reg.AccessToken, "android")
	defer d1.ws.CloseNow()
	login := e.mustLogin("alice_1", goodPassword, "ios")
	d2 := e.connect("alice_1", login.AccessToken, "ios")
	defer d2.ws.CloseNow()
	if len(d1.pending)+len(d2.pending) != 0 {
		t.Fatalf("rejected: %+v %+v", d1.pending, d2.pending)
	}
	bob := e.dial("bob_1", "web")
	defer bob.ws.CloseNow()
	if f := bob.sendText("c2c", "alice_1", "cid-00000001", "hi"); f.Error != nil {
		t.Fatalf("%+v", f.Error)
	}
	d1.waitCmd(protocol.CmdMsg)
	d2.waitCmd(protocol.CmdMsg)
	if n := e.count(`SELECT COUNT(DISTINCT sid) FROM device_sessions WHERE user_id = 'alice_1'`); n != 2 {
		t.Errorf("sessions %d", n)
	}
}

// TC-ACC-02-10（AC-ACC-02-10）：IP 限流：1 分钟第 21 次 429，IPv6 按 /64 合并，另一个 /64 不受影响。
func TestTC_ACC_02_10_IPLimit(t *testing.T) {
	e := openEnv(t, nil)
	e.setClock(shanghaiMidnight(t))
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	for i := range 20 {
		code, b := e.login(fmt.Sprintf("user%03d", i), "wrong-password-1", "203.0.113.1")
		wantErr(t, "under the limit", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	}
	code, b := e.login("user999", "wrong-password-1", "203.0.113.1")
	wantErr(t, "21st", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
	if code, _ := e.adminPost("/admin/test/limits/reset", map[string]any{"scope": "auth"}, adminKey); code != http.StatusOK {
		t.Fatal(code)
	}
	for i := 1; i <= 20; i++ {
		code, b := e.login(fmt.Sprintf("v6user%03d", i), "wrong-password-1", fmt.Sprintf("2001:db8:1:1::%x", i))
		wantErr(t, "v6", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	}
	code, b = e.login("v6user999", "wrong-password-1", "2001:db8:1:1::21")
	wantErr(t, "same /64", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
	code, b = e.login("v6user999", "wrong-password-1", "2001:db8:1:2::1")
	wantErr(t, "other /64", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	// 等窗口过去就恢复。
	e.setClock(shanghaiMidnight(t).Add(time.Minute + time.Second))
	code, b = e.login("v6user999", "wrong-password-1", "2001:db8:1:1::21")
	wantErr(t, "next minute", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
}

// TC-ACC-02-11（AC-ACC-02-11）：不存在的用户名同样累计锁定：两组第 6 次都是 429，响应结构相同。
func TestTC_ACC_02_11_MissingUsernameLocksToo(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real2", nil)
	var last [][]byte
	for _, u := range []string{"nouser2", "real2"} {
		ip := e.freshIP()
		for range 5 {
			code, b := e.login(u, "wrong-password-1", ip)
			wantErr(t, u, code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
		}
		code, b := e.login(u, "wrong-password-1", ip)
		wantErr(t, u+" 6th", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
		last = append(last, b)
	}
	if string(last[0]) != string(last[1]) {
		t.Errorf("responses differ: %s vs %s", last[0], last[1])
	}
}

// TC-ACC-02-12（AC-ACC-02-12）：误输进用户名框的内容不进日志和 account_events；auth_throttle 的键是哈希。
func TestTC_ACC_02_12_UsernameNotLogged(t *testing.T) {
	var logs *syncBuffer
	e := openEnv(t, func(c *app.Config) { logs = captureLogs(c) })
	code, b := e.login("ZXQ-USER-1", "wrong-password-1", e.freshIP())
	wantErr(t, "login", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	for _, s := range []string{"ZXQ-USER-1", "zxq-user-1", "wrong-password-1"} {
		if strings.Contains(logs.String(), s) {
			t.Errorf("log contains %q", s)
		}
	}
	if !strings.Contains(logs.String(), account.UsernameHash("zxq-user-1")) {
		t.Error("the failure is logged with the hash")
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM account_events WHERE user_id LIKE '%zxq%' OR kind LIKE '%zxq%'`,
		`SELECT COUNT(*) FROM auth_throttle WHERE throttle_key LIKE '%zxq%'`,
	} {
		if n := e.count(q); n != 0 {
			t.Errorf("%s = %d", q, n)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM auth_throttle WHERE throttle_key LIKE ?`, "%"+account.UsernameHash("zxq-user-1")+"%"); n != 2 {
		t.Errorf("throttle rows keyed by the hash: %d", n)
	}
}

// TC-ACC-02-13（AC-ACC-02-13，服务端部分）：新设备登录：在线设备 2 秒内收到 securityNotice，接口有这条记录，
// 响应没有 IP 全文、sid 原值。三端界面随 FE-610～612。
func TestTC_ACC_02_13_SecurityNotice(t *testing.T) {
	e := openEnv(t, nil)
	reg := e.register("alice_1", nil)
	d1 := e.connect("alice_1", reg.AccessToken, "ios")
	defer d1.ws.CloseNow()
	const ip = "203.0.113.77"
	code, b, _ := e.post("/v1/account/login", ip, map[string]any{"username": "alice_1", "password": goodPassword, "platform": "android"})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var d2 authResp
	json.Unmarshal(b, &d2)
	f := d1.waitCmdTimeout(protocol.CmdEvent, 2*time.Second)
	var ev protocol.SecurityNoticeEvent
	if err := json.Unmarshal(f.Data, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "securityNotice" || ev.Kind != "newDeviceLogin" || ev.Platform != "android" || ev.At == 0 {
		t.Errorf("event %s", f.Data)
	}
	code, b = e.get("/v1/account/events", d2.AccessToken)
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	validateHTTP(t, "EventsResponse", b)
	var events struct {
		Events []struct {
			Kind, Platform, IPPrefix string
			At                       int64
		}
		OtherSessions struct {
			Count        int
			LastActiveAt int64
		}
	}
	json.Unmarshal(b, &events)
	if len(events.Events) != 1 || events.Events[0].Kind != "newDeviceLogin" || events.Events[0].Platform != "android" || events.Events[0].IPPrefix != "203.0.113.0/24" {
		t.Errorf("events %s", b)
	}
	if events.OtherSessions.Count != 1 || events.OtherSessions.LastActiveAt == 0 {
		t.Errorf("other sessions %s", b)
	}
	sid := strings.Split(d2.RefreshToken, ".")[1]
	if strings.Contains(string(b), ip) || strings.Contains(string(b), sid) || strings.Contains(string(b), account.SIDHash(sid)) {
		t.Errorf("response leaks: %s", b)
	}
	if n := e.count(`SELECT COUNT(*) FROM account_events WHERE sid_hash = ?`, account.SIDHash(sid)); n != 1 {
		t.Errorf("the event stores the hash of sid: %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM account_events WHERE sid_hash = ?`, sid); n != 0 {
		t.Error("the event must not store sid itself")
	}
}

// 第 11.2 节：events 要有 sid；管理接口和演示的 token 403 / 4030。保留 180 天（account_events_purge）；只返回最近 20 条。
func TestAccountEventsRules(t *testing.T) {
	e := openEnv(t, nil)
	e.register("alice_1", nil)
	code, b := e.get("/v1/account/events", e.token("alice_1"))
	wantErr(t, "no sid", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountSessionRequired)
	code, b = e.get("/v1/account/events", "")
	wantErr(t, "no token", code, b, http.StatusUnauthorized, protocol.CodeTokenInvalid, "")

	now := shanghaiMidnight(t)
	e.setClock(now)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	var last authResp
	for range 22 {
		last = e.mustLogin("alice_1", goodPassword, "web")
	}
	code, b = e.get("/v1/account/events", last.AccessToken)
	if code != http.StatusOK || strings.Count(string(b), `"kind"`) != 20 {
		t.Errorf("%d %s", code, b)
	}
	e.setClock(now.Add(181 * 24 * time.Hour))
	if code, b := e.adminPost("/admin/test/jobs/account_events_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM account_events`); n != 0 {
		t.Errorf("events after 181 days: %d", n)
	}
	if code, b := e.adminPost("/admin/test/jobs/auth_throttle_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
}

// TC-ACC-02-14（AC-ACC-02-14）：锁定计数在数据库：用同一数据库启动一个新实例（没有任何内存状态），同一时钟，正确密码仍然 429。
func TestTC_ACC_02_14_LockSurvivesRestart(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real1", nil)
	now := shanghaiMidnight(t)
	e.setClock(now)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	const ip = "203.0.113.1"
	for range 5 {
		e.login("real1", "wrong-password-1", ip)
	}
	restarted := &acctEnv{dbPath: e.dbPath, st: e.st, env: newEnvCfg(t, func(c *app.Config) {
		c.DBPath = e.dbPath
		c.Account.RegisterMode = account.ModeOpen
		c.Account.RefreshKeys = []account.RefreshKey{{ID: "t1", Secret: []byte("e2e-refresh-key-0123456789abcdef")}}
		c.Account.RefreshActive = "t1"
		c.Account.Argon = lightArgon
		c.Gateway.Guard.TrustedProxies = trustLocal
	})}
	restarted.setClock(now)
	code, b := restarted.login("real1", goodPassword, ip)
	wantErr(t, "after restart", code, b, http.StatusTooManyRequests, protocol.CodeAccountLocked, "")
	if n := e.count(`SELECT COUNT(*) FROM auth_throttle WHERE locked_until > 0`); n != 1 {
		t.Errorf("throttle rows %d", n)
	}
}

// F-ACC-02 异常：旧哈希参数升级后，登录成功时重算，不改变应答；错误密码不重算。
func TestLoginRehashesOldParams(t *testing.T) {
	var path string
	var refresh = []account.RefreshKey{{ID: "t1", Secret: []byte("e2e-refresh-key-0123456789abcdef")}}
	oldParams := account.ArgonParams{MemoryKiB: 128, Time: 1, Threads: 1}
	e := openEnv(t, func(c *app.Config) { c.Account.Argon = oldParams; path = c.DBPath })
	e.register("real1", nil)
	var before string
	e.rawDB().QueryRow(`SELECT password_hash FROM accounts`).Scan(&before)
	if !strings.Contains(before, "m=128,") {
		t.Fatalf("hash %q", before)
	}
	upgraded := &acctEnv{dbPath: path, st: e.st, env: newEnvCfg(t, func(c *app.Config) {
		c.DBPath = path
		c.Account.RegisterMode = account.ModeOpen
		c.Account.RefreshKeys, c.Account.RefreshActive = refresh, "t1"
		c.Account.Argon = lightArgon
		c.Gateway.Guard.TrustedProxies = trustLocal
	})}
	upgraded.login("real1", "wrong-password-1", upgraded.freshIP())
	var hash string
	e.rawDB().QueryRow(`SELECT password_hash FROM accounts`).Scan(&hash)
	if hash != before {
		t.Error("a failed login must not rehash")
	}
	if code, b := upgraded.login("real1", goodPassword, upgraded.freshIP()); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	e.rawDB().QueryRow(`SELECT password_hash FROM accounts`).Scan(&hash)
	if !strings.Contains(hash, fmt.Sprintf("m=%d,", lightArgon.MemoryKiB)) {
		t.Errorf("hash not upgraded: %q", hash)
	}
	if code, b := upgraded.login("real1", goodPassword, upgraded.freshIP()); code != http.StatusOK {
		t.Errorf("after rehash: %d %s", code, b)
	}
}

// 指标（BE-414 第 4 条）：标签只有枚举值；账号相关的值都能出现。
func TestAccountMetrics(t *testing.T) {
	e := openEnv(t, nil)
	e.register("real1", nil)
	e.login("real1", "wrong-password-1", e.freshIP())
	e.login("real1", goodPassword, e.freshIP())
	text := e.metricsText()
	for _, want := range []string{`im_registers_total{result="ok"} 1`, `im_account_logins_total{result="ok"} 1`, `im_account_logins_total{result="invalid"} 1`} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %s", want)
		}
	}
	if strings.Contains(text, "real1") {
		t.Error("metrics contain a username")
	}
}

// AC-ACC-07-2（提前到这一段：账号从这一段开始存在，演示登录不能接管它们）：TC-ACC-07-02。
func TestTC_ACC_07_02_DemoAndAccountAreSeparate(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.DemoLogin = true })
	if code, b := e.publicPost("/demo/login", map[string]any{"username": "demo1"}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	code, b := e.login("demo1", "any-password-1", e.freshIP())
	wantErr(t, "demo user", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")
	e.register("acc1", map[string]any{"nickname": "甲"})
	code, b = e.publicPost("/demo/login", map[string]any{"username": "acc1", "nickname": "乙"})
	wantErr(t, "account user", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonAccountExists)
	if strings.Contains(string(b), "token") {
		t.Errorf("no token: %s", b)
	}
	var nick string
	e.rawDB().QueryRow(`SELECT nickname FROM users WHERE user_id = 'acc1'`).Scan(&nick)
	if nick != "甲" {
		t.Errorf("nickname %q", nick)
	}
}

// F-ACC-08 规则 4：已注销的用户：签发 token、演示登录都是 4042，不复活。
func TestDeletedUserEntryPoints(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.DemoLogin = true })
	e.register("gone1", nil)
	if err := e.st.MarkUserDeleted(t.Context(), "gone1", 1); err != nil {
		t.Fatal(err)
	}
	code, b := e.adminPost("/admin/users/token", map[string]any{"userId": "gone1"}, adminKey)
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	code, b = e.publicPost("/demo/login", map[string]any{"username": "gone1", "nickname": "回来"})
	wantErr(t, "demo login", code, b, http.StatusNotFound, protocol.CodeUserNotFound, "")
	var nick string
	e.rawDB().QueryRow(`SELECT nickname FROM users WHERE user_id = 'gone1'`).Scan(&nick)
	if nick == "回来" {
		t.Error("demo login must not touch a deleted user")
	}
}

// BE-411 第 3 条、F-ACC-03 规则 2、4：有 sid 的 token，WS auth 和 Bearer 都检查会话和代号。转正随 BE-416。
func TestSessionVerdictsOnWSAndBearer(t *testing.T) {
	e := openEnv(t, nil)
	r := e.register("alice_1", nil)
	sid := strings.Split(r.RefreshToken, ".")[1]

	// 代号不是 current：WS kicked/tokenExpired；HTTP 401 / 4011 tokenSuperseded。
	e.rawExec(`UPDATE device_sessions SET current_gen = 5 WHERE sid = ?`, sid)
	e.connect("alice_1", r.AccessToken, "ios").expectKicked(protocol.KickTokenExpired, protocol.CloseTokenExpired, waitTimeout)
	code, b := e.get("/v1/account/events", r.AccessToken)
	wantErr(t, "superseded", code, b, http.StatusUnauthorized, protocol.CodeTokenExpired, protocol.ReasonTokenSuperseded)
	e.rawExec(`UPDATE device_sessions SET current_gen = 1 WHERE sid = ?`, sid)

	// 吊销原因映射（F-ACC-03 规则 2）：credentialsChanged → passwordChanged，其余 → sessionRevoked，都是 4012。
	for reason, kick := range map[string]string{"credentialsChanged": protocol.KickPasswordChanged, "revoked": protocol.KickSessionRevoked, "accountDeleted": protocol.KickAccountDeleted} {
		e.rawExec(`UPDATE device_sessions SET revoked_at = 1, revoke_reason = ? WHERE sid = ?`, reason, sid)
		e.connect("alice_1", r.AccessToken, "ios").expectKicked(kick, protocol.CloseTokenRevoked, waitTimeout)
	}
	code, b = e.get("/v1/account/events", r.AccessToken)
	wantErr(t, "revoked", code, b, http.StatusUnauthorized, protocol.CodeTokenRevoked, "")

	// 没有 sid 的 token 不受影响（AC-ACC-03-5 的行为不变）。
	c := e.dial("alice_1", "web")
	defer c.ws.CloseNow()
	if len(c.pending) != 0 {
		t.Errorf("a token without sid was rejected: %+v", c.pending)
	}
}

// TC-CHAT-90-14（AC-CHAT-90-16）：/v1 的基础约束，用真实的登录接口：65 KB 400 / 4000，64 KB 进入业务校验；
// 过期和被下线的 token 401。慢请求是真实的 10 秒（每秒 1 字节发 11 秒）。
func TestTC_CHAT_90_14_V1Base(t *testing.T) {
	e := openEnv(t, nil)
	pad := func(size int) map[string]any {
		base, _ := json.Marshal(map[string]any{"username": "alice_1", "password": "wrong-password-1", "platform": "ios", "x": ""})
		return map[string]any{"username": "alice_1", "password": "wrong-password-1", "platform": "ios", "x": strings.Repeat("a", size-len(base))}
	}
	code, b, _ := e.post("/v1/account/login", e.freshIP(), pad(65<<10))
	wantErr(t, "65 KB", code, b, http.StatusBadRequest, protocol.CodeBadRequest, "")
	code, b, _ = e.post("/v1/account/login", e.freshIP(), pad(64<<10))
	wantErr(t, "64 KB reaches the business rules", code, b, http.StatusUnauthorized, protocol.CodeAccountCredentials, "")

	code, b = e.get("/v1/account/events", e.tokenTTL("alice_x", 1))
	if code != http.StatusForbidden { // 还没过期：没有 sid
		t.Errorf("before expiry: %d %s", code, b)
	}
	expired := e.tokenTTL("alice_x", 1)
	e.setClock(time.Now().Add(time.Hour))
	code, b = e.get("/v1/account/events", expired)
	wantErr(t, "expired", code, b, http.StatusUnauthorized, protocol.CodeTokenExpired, "")
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)

	reg := e.register("kicked_1", nil)
	if code, b := e.adminPost("/admin/users/kicked_1/force-logout", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	code, b = e.get("/v1/account/events", reg.AccessToken)
	wantErr(t, "forced logout", code, b, http.StatusUnauthorized, protocol.CodeTokenRevoked, "")

	// 第 2 步：慢请求被断开，没有响应。
	conn, err := net.Dial("tcp", e.srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /v1/account/login HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\n")
	start := time.Now()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for time.Since(start) < 13*time.Second {
		select {
		case <-closed:
			if d := time.Since(start); d < 9*time.Second || d > 12*time.Second {
				t.Errorf("disconnected after %v, want about 10s", d)
			}
			return
		case <-ticker.C:
			conn.Write([]byte("a"))
		}
	}
	t.Error("the slow request was not disconnected")
}

// TC-CHAT-90-14 第 3 步：生产配置下直连 http 被拒绝，可信代理转发的 https 进入业务。
func TestTC_CHAT_90_14_ProductionHTTPSOnly(t *testing.T) {
	e := openEnv(t, func(c *app.Config) {
		c.RequireHTTPS = true
		c.Gateway = gateway.DefaultConfig() // 不信任任何代理：直连
	})
	code, b, _ := e.post("/v1/account/login", "", map[string]any{"username": "a", "password": "b", "platform": "ios"})
	wantErr(t, "http", code, b, http.StatusForbidden, protocol.CodeForbidden, "")
	code, b = e.get("/v1/config", "")
	wantErr(t, "config over http", code, b, http.StatusForbidden, protocol.CodeForbidden, "")
	// /ws、/healthz 不受影响（现有行为不回归）。
	if code, _ := e.do(e.srv.URL, http.MethodGet, "/healthz", nil, ""); code != http.StatusOK {
		t.Errorf("healthz %d", code)
	}
}
