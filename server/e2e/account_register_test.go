package e2e

// 注册（PRD-chat F-ACC-01，BE-413）。用例名带 TC 编号，见 docs/testcases/TC-ACC.md。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

const inviteA = "invite-code-aaaaaaaaaaaaaaaaaaaaaaaa"

func farFuture() time.Time { return time.Now().Add(100 * 24 * time.Hour) }

// TC-ACC-01-01：邀请码模式注册成功，用户名规范化为小写，token 能通过 WS auth。
// 第 3 步 recovery-code/ack 接口随 BE-418，这里只用存储层确认状态。
func TestTC_ACC_01_01_RegisterWithInvite(t *testing.T) {
	e := newAcctEnv(t, nil)
	e.newInvite(inviteA, 1, farFuture())
	code, b, _ := e.post("/v1/account/register", e.freshIP(), map[string]any{"username": "Alice_1", "password": goodPassword, "inviteCode": inviteA, "platform": "android", "deviceName": "Pixel"})
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	validateHTTP(t, "AuthResponse", b)
	var r authResp
	json.Unmarshal(b, &r)
	if r.UserID != "alice_1" || r.AccessToken == "" || r.RefreshToken == "" || r.RecoveryCode == "" || r.RecoveryCodeGen != 1 {
		t.Fatalf("response %s", b)
	}
	// auth ack 带 recoveryCodeUnacked（F-ACC-05 规则 1）。
	c := e.dialRaw("alice_1", e.platformURL("android"), nil)
	defer c.ws.CloseNow()
	ack := mustAck[protocol.AuthAck](t, c.call(protocol.CmdAuth, map[string]any{"token": r.AccessToken}))
	if ack.UserID != "alice_1" || !ack.RecoveryCodeUnacked {
		t.Errorf("auth ack %+v", ack)
	}
	e.ack(r.AccessToken, r.RecoveryCodeGen)
	c2 := e.dialRaw("alice_1", e.platformURL("android"), nil)
	defer c2.ws.CloseNow()
	if ack := mustAck[protocol.AuthAck](t, c2.call(protocol.CmdAuth, map[string]any{"token": r.AccessToken})); ack.RecoveryCodeUnacked {
		t.Errorf("acked: %+v", ack)
	}
	if n := e.count(`SELECT COUNT(*) FROM accounts WHERE username_lower = 'alice_1'`); n != 1 {
		t.Errorf("accounts %d", n)
	}
	if n := e.count(`SELECT used_count FROM invites WHERE code_hash = ?`, account.InviteHash(inviteA)); n != 1 {
		t.Errorf("used_count %d", n)
	}
}

// F-ACC-03 规则 1、3：token 和会话的结构，下一段（BE-415）在它上面做轮换。
func TestRegisterSessionShape(t *testing.T) {
	e := openEnv(t, nil)
	e.setClock(shanghaiMidnight(t))
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	now := shanghaiMidnight(t).UnixMilli()
	r := e.register("alice_1", nil)
	parts := strings.Split(r.RefreshToken, ".")
	if len(parts) != 5 || parts[0] != "t1" || parts[2] != "1" {
		t.Fatalf("refresh token format: %q", r.RefreshToken)
	}
	sid := parts[1]
	var gen, next, rowVer, exp, abs int64
	var hash string
	if err := e.rawDB().QueryRow(`SELECT current_gen, next_gen, row_version, expires_at, absolute_expires_at, current_hash FROM device_sessions WHERE sid = ?`, sid).Scan(&gen, &next, &rowVer, &exp, &abs, &hash); err != nil {
		t.Fatal(err)
	}
	if gen != 1 || next != 2 || rowVer != 1 {
		t.Errorf("gen=%d next=%d row=%d", gen, next, rowVer)
	}
	if hash == r.RefreshToken || strings.Contains(hash, parts[3]) {
		t.Error("only the hash of the refresh token is stored")
	}
	const day = int64(24 * 3600 * 1000)
	if exp != now+90*day || abs != now+365*day || r.RefreshExpiresAt != exp || r.AbsoluteExpiresAt != abs {
		t.Errorf("exp=%d abs=%d resp=%+v", exp-now, abs-now, r)
	}
	if r.AccessExpiresAt != now+day {
		t.Errorf("access ttl %d", r.AccessExpiresAt-now)
	}
	// H5 的 refresh 更短（第 4.1 节）。
	code, b, _ := e.post("/v1/account/register", e.freshIP(), map[string]any{"username": "web_user", "password": goodPassword, "platform": "web"})
	var web authResp
	json.Unmarshal(b, &web)
	if code != http.StatusOK || web.RefreshExpiresAt != now+day {
		t.Errorf("web: %d %s", code, b)
	}
}

// TC-ACC-01-02：边界：用户名 4 位、32 位，密码 8、128 个码点都能注册；按码点计数，不按字节。
func TestTC_ACC_01_02_Boundaries(t *testing.T) {
	e := openEnv(t, nil)
	e.register("abcd", map[string]any{"password": "八个码点的中文密码"})
	e.register("a"+strings.Repeat("b", 31), map[string]any{"password": strings.Repeat("密码", 64)})
}

// TC-ACC-01-03（AC-ACC-01-2）：四种无效邀请码的应答逐字节相同，不暴露用户名是否存在。
func TestTC_ACC_01_03_InvalidInvitesIdentical(t *testing.T) {
	e := newAcctEnv(t, nil)
	now := shanghaiMidnight(t)
	e.setClock(now)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	e.newInvite("seed-invite-seed-invite-seed", 1, now.Add(100*time.Hour))
	e.register("taken1", map[string]any{"inviteCode": "seed-invite-seed-invite-seed"}) // 同时把这个邀请码用完（I3）
	e.newInvite("expiring-invite-expiring-inv", 5, now.Add(time.Hour))                 // I2
	if n := e.count(`SELECT COUNT(*) FROM device_sessions`); n != 1 {
		t.Fatal(n)
	}
	e.setClock(now.Add(2 * time.Hour))
	type resp struct {
		status  int
		body    string
		headers string
	}
	var got []resp
	for _, username := range []string{"taken1", "free1"} {
		for _, invite := range []any{nil, "no-such-invite", "expiring-invite-expiring-inv", "seed-invite-seed-invite-seed"} {
			body := regBody(username, nil)
			if invite != nil {
				body["inviteCode"] = invite
			}
			code, b, h := e.post("/v1/account/register", e.freshIP(), body)
			var names []string
			for k := range h {
				if k != "Date" {
					names = append(names, k)
				}
			}
			sort.Strings(names)
			got = append(got, resp{code, string(b), strings.Join(names, ",")})
		}
	}
	for i, r := range got {
		if r != got[0] {
			t.Errorf("response %d differs:\n%+v\n%+v", i, r, got[0])
		}
	}
	wantErr(t, "invalid invite", got[0].status, []byte(got[0].body), http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonInviteInvalid)
	if n := e.count(`SELECT COUNT(*) FROM users WHERE user_id = 'free1'`); n != 0 {
		t.Error("free1 must not exist")
	}
	if n := e.count(`SELECT used_count FROM invites WHERE code_hash = ?`, account.InviteHash("expiring-invite-expiring-inv")); n != 0 {
		t.Errorf("expired invite used %d", n)
	}
}

// TC-ACC-01-04（AC-ACC-01-3）：closed 模式拒绝注册，带不带邀请码都一样。
func TestTC_ACC_01_04_Closed(t *testing.T) {
	e := newAcctEnv(t, func(c *app.Config) { c.Account.RegisterMode = account.ModeClosed })
	e.newInvite(inviteA, 1, farFuture())
	for _, extra := range []map[string]any{nil, {"inviteCode": inviteA}} {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("closed1", extra))
		wantErr(t, "closed", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonRegisterClosed)
	}
	if e.count(`SELECT COUNT(*) FROM users`) != 0 || e.count(`SELECT COUNT(*) FROM accounts`) != 0 {
		t.Error("nothing may be created")
	}
}

// TC-ACC-01-05（AC-ACC-01-4）：不合法用户名都是 400 / 4004；首尾空白去掉后判断。
func TestTC_ACC_01_05_InvalidUsernames(t *testing.T) {
	e := openEnv(t, nil)
	for _, name := range []string{"abc", strings.Repeat("a", 33), "1abc", "a b c", "张三abc", "Kabc"} {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(name, nil))
		wantErr(t, name, code, b, http.StatusBadRequest, protocol.CodeInvalidID, "")
	}
	if e.count(`SELECT COUNT(*) FROM users`) != 0 {
		t.Error("nothing may be created")
	}
	if r := e.register("  Valid_User  ", nil); r.UserID != "valid_user" {
		t.Errorf("userId %q", r.UserID)
	}
}

// TC-ACC-01-06（AC-ACC-01-5）：只有注册不区分大小写：演示用户 Bobby 占用 bobby，导入 BOBBY 不受影响（C-18、TC-USER-01-16）。
// AC 和 TC 原文用 Bob / bob，但 bob 只有 3 位，按 F-ACC-01 规则 3 是 400 / 4004 走不到 4095（PRD 问题，已在汇报列出）。这里换成 5 位，验证目的不变。
func TestTC_ACC_01_06_CaseInsensitiveOnlyForRegister(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.DemoLogin = true })
	if code, b := e.publicPost("/demo/login", map[string]any{"username": "Bobby"}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("bobby", nil))
	wantErr(t, "bobby", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
	if code, b := e.adminPost("/admin/users/import", map[string]any{"users": []map[string]any{{"userId": "BOBBY"}}}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM users WHERE user_id IN ('Bobby', 'BOBBY')`); n != 2 {
		t.Errorf("users %d", n)
	}
	if e.count(`SELECT COUNT(*) FROM accounts`) != 0 {
		t.Error("no account may be created")
	}
}

// TC-ACC-01-07（AC-ACC-01-6）：保留名、已注销的用户名不能注册，响应相同。account_cleanup 随 BE-420，这里直接用第 ① 步。
func TestTC_ACC_01_07_ReservedAndDeleted(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.Account.ExtraReserved = []string{"Custom_Name"} })
	e.register("gone1", nil)
	if err := e.st.MarkUserDeleted(t.Context(), "gone1", 1); err != nil {
		t.Fatal(err)
	}
	var bodies []string
	for _, name := range []string{"admin", "ADMIN", "custom_name", "gone1", "kefu"} {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(name, nil))
		wantErr(t, name, code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
		bodies = append(bodies, string(b))
	}
	for _, b := range bodies {
		if b != bodies[0] {
			t.Errorf("responses differ: %s vs %s", b, bodies[0])
		}
	}
}

// TC-ACC-01-08（AC-ACC-01-7）：弱密码五种情况都返回 passwordWeak，用户未创建。
func TestTC_ACC_01_08_WeakPasswords(t *testing.T) {
	e := openEnv(t, nil)
	for _, pw := range []string{"1234567", strings.Repeat("a1", 64) + "b", "weak_user", "aaaaaaaa", "password1"} {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("weak_user", map[string]any{"password": pw}))
		wantErr(t, pw, code, b, http.StatusBadRequest, protocol.CodeBadRequest, protocol.ReasonPasswordWeak)
	}
	if e.count(`SELECT COUNT(*) FROM users`) != 0 {
		t.Error("user must not be created")
	}
}

// TC-ACC-01-09（AC-ACC-01-8）：同一用户名 20 个并发注册，只有一个成功，其余 4095 或 5030，邀请码只扣一次。闸门用默认值。
func TestTC_ACC_01_09_ConcurrentRegister(t *testing.T) {
	e := newAcctEnv(t, nil)
	e.newInvite(inviteA, 20, farFuture())
	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("race1", map[string]any{"inviteCode": inviteA}))
			var er apiErr
			json.Unmarshal(b, &er)
			codes[i] = er.Code
		}()
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		switch c {
		case 0:
			ok++
		case protocol.CodeUsernameTaken, protocol.CodeServiceUnavailable:
		default:
			t.Errorf("code %d", c)
		}
	}
	if ok != 1 {
		t.Errorf("%d succeeded: %v", ok, codes)
	}
	if e.count(`SELECT COUNT(*) FROM accounts`) != 1 || e.count(`SELECT used_count FROM invites`) != 1 {
		t.Error("exactly one account and one invite use")
	}
}

// TC-ACC-01-10（AC-ACC-01-10）：同一 IP 第 21 次注册 429（不合法的请求也算），换 IP 不受影响，两种模式都限。
func TestTC_ACC_01_10_RegisterIPLimit(t *testing.T) {
	for _, mode := range []string{account.ModeOpen, account.ModeInvite} {
		t.Run(mode, func(t *testing.T) {
			e := newAcctEnv(t, func(c *app.Config) { c.Account.RegisterMode = mode })
			e.newInvite(inviteA, 5, farFuture())
			extra := map[string]any{"inviteCode": inviteA}
			for i := range 20 {
				code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("x", extra)) // 用户名不合法
				if code != http.StatusBadRequest {
					t.Fatalf("attempt %d: %d %s", i, code, b)
				}
			}
			code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("valid_1", extra))
			er := wantErr(t, "21st", code, b, http.StatusTooManyRequests, protocol.CodeRateLimited, "")
			if er.Details["retryAfterSeconds"] == nil {
				t.Errorf("no retryAfterSeconds: %s", b)
			}
			if code, b, _ := e.post("/v1/account/register", "203.0.113.2", regBody("valid_1", extra)); code != http.StatusOK {
				t.Errorf("other ip: %d %s", code, b)
			}
			// limits/reset scope=auth 清计数（第 4.5 节）。
			if code, _ := e.adminPost("/admin/test/limits/reset", map[string]any{"scope": "auth"}, adminKey); code != http.StatusOK {
				t.Fatal(code)
			}
			if code, b, _ := e.post("/v1/account/register", "203.0.113.1", regBody("valid_2", extra)); code != http.StatusOK {
				t.Errorf("after reset: %d %s", code, b)
			}
		})
	}
}

// TC-ACC-01-11（AC-ACC-01-11）：账号表和日志没有明文密码、恢复码。真实参数：password_hash 是 Argon2id，m=64 MiB、t=3。
func TestTC_ACC_01_11_NoPlaintext(t *testing.T) {
	var logs *syncBuffer
	e := openEnv(t, func(c *app.Config) {
		logs = captureLogs(c)
		c.Account.Argon = account.ArgonParams{} // 默认值
	})
	const pw = "ZXQPW-1a2b"
	r := e.register("alice_1", map[string]any{"password": pw, "deviceName": "ZXQDEV-1"})
	var hash, rc string
	if err := e.rawDB().QueryRow(`SELECT password_hash FROM accounts WHERE user_id = 'alice_1'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Errorf("hash %q", hash)
	}
	if err := e.rawDB().QueryRow(`SELECT hash FROM recovery_codes WHERE user_id = 'alice_1'`).Scan(&rc); err != nil {
		t.Fatal(err)
	}
	if rc == r.RecoveryCode || rc != account.RecoveryHash("alice_1", r.RecoveryCode) {
		t.Errorf("recovery hash %q", rc)
	}
	// 整个数据库文件和日志都搜不到（第 12.2 节）。
	for _, secret := range []string{pw, r.RecoveryCode, strings.ReplaceAll(r.RecoveryCode, "-", ""), r.RefreshToken, r.AccessToken} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("log contains %q", secret)
		}
		for _, q := range []string{`SELECT password_hash FROM accounts`, `SELECT hash FROM recovery_codes`, `SELECT current_hash FROM device_sessions`} {
			var v string
			e.rawDB().QueryRow(q).Scan(&v)
			if strings.Contains(v, secret) {
				t.Errorf("%s contains %q", q, secret)
			}
		}
	}
	// deviceName 不进日志（规则 1）。sid 不进日志（F-ACC-03 规则 1）。
	if strings.Contains(logs.String(), "ZXQDEV-1") || strings.Contains(logs.String(), strings.Split(r.RefreshToken, ".")[1]) {
		t.Errorf("log contains the device name or sid: %s", logs.String())
	}
}

// TC-ACC-01-14（AC-ACC-01-14）：一个邀请码最多探测 20 个用户名，之后自动停用，告警到人；邀请码没有被扣减。
func TestTC_ACC_01_14_InviteProbeLimit(t *testing.T) {
	var logs *syncBuffer
	e := newAcctEnv(t, func(c *app.Config) { logs = captureLogs(c) })
	e.newInvite(inviteA, 1, farFuture())
	e.newInvite("other-invite-other-invite-ot", 25, farFuture())
	for i := 1; i <= 20; i++ {
		e.register(fmt.Sprintf("taken%02d", i), map[string]any{"inviteCode": "other-invite-other-invite-ot"})
	}
	for i := 1; i <= 20; i++ {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(fmt.Sprintf("taken%02d", i), map[string]any{"inviteCode": inviteA}))
		wantErr(t, "probe", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
	}
	if n := e.count(`SELECT used_count FROM invites WHERE code_hash = ?`, account.InviteHash(inviteA)); n != 0 {
		t.Errorf("used_count %d", n)
	}
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("free2", map[string]any{"inviteCode": inviteA}))
	wantErr(t, "after probes", code, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonInviteInvalid)
	if e.count(`SELECT COUNT(*) FROM users WHERE user_id = 'free2'`) != 0 {
		t.Error("free2 must not exist")
	}
	if n := e.count(`SELECT auto_disabled FROM invites WHERE code_hash = ?`, account.InviteHash(inviteA)); n != 1 {
		t.Error("the invite must show as automatically disabled")
	}
	if l := logs.String(); !strings.Contains(l, "invite auto disabled") || strings.Contains(l, inviteA) {
		t.Errorf("alert missing or contains the code: %s", l)
	}
	if !strings.Contains(e.metricsText(), "im_invites_auto_disabled_total 1") {
		t.Error("metric not incremented")
	}
}

// TC-ACC-01-21（AC-ACC-01-21）：邀请码自动停用按窗口计：跨窗口的 38 次 4095 不停用。
func TestTC_ACC_01_21_InviteMismatchWindow(t *testing.T) {
	e := newAcctEnv(t, nil)
	now := shanghaiMidnight(t)
	e.setClock(now)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	e.newInvite(inviteA, 1000, now.Add(1000*time.Hour))
	e.register("taken_19", map[string]any{"inviteCode": inviteA})
	probe := func() {
		for range 19 {
			code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("taken_19", map[string]any{"inviteCode": inviteA}))
			wantErr(t, "probe", code, b, http.StatusConflict, protocol.CodeUsernameTaken, "")
		}
	}
	probe()
	e.setClock(now.Add(25 * time.Hour))
	probe()
	if r := e.register("free3", map[string]any{"inviteCode": inviteA}); r.UserID != "free3" {
		t.Error(r)
	}
}

// TC-ACC-01-15（AC-ACC-01-15）：第 1 次注册事务失败：500 / 5000，用户不产生，邀请码不扣；重试成功。
func TestTC_ACC_01_15_TransactionFailure(t *testing.T) {
	e := newAcctEnv(t, nil)
	e.newInvite(inviteA, 1, farFuture())
	if code, b := e.adminPost("/admin/test/faults", map[string]any{"registerTxFail": 1}, adminKey); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("txfail1", map[string]any{"inviteCode": inviteA}))
	wantErr(t, "tx", code, b, http.StatusInternalServerError, protocol.CodeInternal, "")
	if e.count(`SELECT COUNT(*) FROM users`) != 0 || e.count(`SELECT COUNT(*) FROM accounts`) != 0 || e.count(`SELECT used_count FROM invites`) != 0 {
		t.Error("the failed transaction must not leave anything")
	}
	e.register("txfail1", map[string]any{"inviteCode": inviteA})
	if e.count(`SELECT used_count FROM invites`) != 1 {
		t.Error("used_count")
	}
}

// TC-ACC-01-16（AC-ACC-01-16，已有接口的部分）：4 个匿名注册占满闸门，第 5 个约 1 秒后 503 / 5030；关闭钩子后前 4 个完成。
// token/refresh、logout、已登录改密的部分要等 BE-415、BE-417；闸门的专用位见 internal/account 的单测。
func TestTC_ACC_01_16_GateFullOnRegister(t *testing.T) {
	e := openEnv(t, nil)
	e.adminPost("/admin/test/faults", map[string]any{"argonBlock": true}, adminKey)
	var wg sync.WaitGroup
	results := make([]int, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], _, _ = e.post("/v1/account/register", e.freshIP(), regBody(fmt.Sprintf("holder%d", i), nil))
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
		time.Sleep(5 * time.Millisecond)
	}
	start := time.Now()
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("fifth_1", nil))
	d := time.Since(start)
	wantErr(t, "fifth", code, b, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable, "")
	if d < 900*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("queued %v, want about 1s", d)
	}
	e.adminPost("/admin/test/faults", map[string]any{"argonBlock": false}, adminKey)
	wg.Wait()
	for i, c := range results {
		if c != http.StatusOK {
			t.Errorf("holder %d: %d", i, c)
		}
	}
}

// TC-ACC-01-17（AC-ACC-01-17）：platform 不合法、deviceName 超长都是 4000；边界和不传都成功。
func TestTC_ACC_01_17_DeviceFields(t *testing.T) {
	e := openEnv(t, nil)
	code, b, _ := e.post("/v1/account/register", e.freshIP(), map[string]any{"username": "plat_1", "password": goodPassword, "platform": "tv"})
	wantErr(t, "tv", code, b, http.StatusBadRequest, protocol.CodeBadRequest, "")
	code, b, _ = e.post("/v1/account/register", e.freshIP(), regBody("plat_1", map[string]any{"deviceName": strings.Repeat("机", 65)}))
	wantErr(t, "65", code, b, http.StatusBadRequest, protocol.CodeBadRequest, "")
	e.register("plat_1", map[string]any{"deviceName": strings.Repeat("机", 64)})
	e.register("plat_2", nil)
}

// TC-ACC-10-02（AC-ACC-10-2）：注册记录同意的隐私说明版本。昵称不传时等于用户名（F-ACC-01 规则 6）。
func TestTC_ACC_10_02_NoticeVersion(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.Account.PrivacyVersion = "v1" })
	e.register("alice_1", nil)
	var v, nick string
	if err := e.rawDB().QueryRow(`SELECT a.notice_version, u.nickname FROM accounts a JOIN users u ON u.user_id = a.user_id`).Scan(&v, &nick); err != nil {
		t.Fatal(err)
	}
	if v != "v1" || nick != "alice_1" {
		t.Errorf("notice %q nickname %q", v, nick)
	}
}

// PRD 第 4.1 节：没有配 refresh 密钥（老部署升级）时账号接口 503，其他接口不受影响。
func TestAccountAPIUnavailableWithoutRefreshKeys(t *testing.T) {
	e := newAcctEnv(t, func(c *app.Config) { c.Account.RefreshKeys = nil; c.Account.RefreshActive = "" })
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("alice_1", nil))
	wantErr(t, "register", code, b, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable, "")
	if code, b := e.get("/v1/config", ""); code != http.StatusOK {
		t.Errorf("config: %d %s", code, b)
	}
}
