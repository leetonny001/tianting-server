package e2e

// F-ACC-09 的管理接口（BE-421）。

import (
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/config"
	"tianting/server/internal/protocol"
)

// inviteEnv 带一个 account.invite 调用方和一个没有这个 scope 的调用方。
func inviteEnv(t *testing.T, logs **syncBuffer) (*acctEnv, testCaller, testCaller) {
	t.Helper()
	op := hmacCaller("op", adminauth.ScopeAccountInvite, adminauth.ScopeUserRead, adminauth.ScopeAuditRead)
	support := hmacCaller("support", adminauth.ScopeUserRead)
	e := newAcctEnv(t, func(c *app.Config) {
		c.AdminCallers = []adminauth.Caller{op.cfg, support.cfg}
		c.Account.RegisterMode = "invite"
		if logs != nil {
			*logs = captureLogs(c)
		}
	})
	return e, op, support
}

type inviteList struct {
	Invites []struct {
		InviteID  string `json:"inviteId"`
		Code      string `json:"code"`
		CodeLast4 string `json:"codeLast4"`
		UsedCount int    `json:"usedCount"`
		MaxUses   int    `json:"maxUses"`
	} `json:"invites"`
}

func (e *acctEnv) createInvite(c testCaller, count, maxUses int) inviteList {
	e.t.Helper()
	code, b := e.signed(c, http.MethodPost, "/admin/invites", map[string]any{"count": count, "maxUses": maxUses, "expiresAt": time.Now().Add(oneDay).UnixMilli(), "note": "n"}, signOpts{})
	if code != http.StatusOK {
		e.t.Fatalf("%d %s", code, b)
	}
	var l inviteList
	mustJSON(e.t, b, &l)
	return l
}

// TC-ACC-09-01（AC-ACC-09-1，后端部分）：maxUses=2，三个用户注册：前两个成功，第三个 403；已用 2 次，列表只有后 4 位。
func TestTC_ACC_09_01_MaxUses(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	inv := e.createInvite(op, 1, 2).Invites[0]
	for i, u := range []string{"user_1", "user_2"} {
		if c, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(u, map[string]any{"inviteCode": inv.Code})); c != http.StatusOK {
			t.Fatalf("%d: %d %s", i, c, b)
		}
	}
	c, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("user_3", map[string]any{"inviteCode": inv.Code}))
	wantErr(t, "third", c, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonInviteInvalid)
	code, lb := e.signed(op, http.MethodGet, "/admin/invites", nil, signOpts{})
	var l inviteList
	mustJSON(t, lb, &l)
	if code != http.StatusOK || l.Invites[0].UsedCount != 2 || l.Invites[0].Code != "" || l.Invites[0].CodeLast4 != inv.Code[len(inv.Code)-4:] {
		t.Errorf("%d %s", code, lb)
	}
}

// TC-ACC-09-02（AC-ACC-09-2）：库、审计、日志都没有明文。
func TestTC_ACC_09_02_NoPlaintext(t *testing.T) {
	var logs *syncBuffer
	e, op, _ := inviteEnv(t, &logs)
	code := e.createInvite(op, 1, 1).Invites[0].Code
	if n := e.count(`SELECT COUNT(*) FROM invites WHERE code_hash = ? OR code_last4 = ?`, code, code); n != 0 {
		t.Errorf("plaintext in invites: %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM admin_audit WHERE summary_json LIKE ? OR targets_json LIKE ?`, "%"+code+"%", "%"+code+"%"); n != 0 {
		t.Errorf("plaintext in audit: %d", n)
	}
	if strings.Contains(logs.String(), code) {
		t.Error("plaintext in logs")
	}
}

// TC-ACC-09-03（AC-ACC-09-5）：已停用的邀请码 403；重复停用幂等。
func TestTC_ACC_09_03_Disabled(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	inv := e.createInvite(op, 1, 5).Invites[0]
	for range 2 {
		if c, b := e.signed(op, http.MethodPost, "/admin/invites/"+inv.InviteID+"/disable", map[string]any{}, signOpts{}); c != http.StatusOK {
			t.Fatalf("%d %s", c, b)
		}
	}
	c, b, _ := e.post("/v1/account/register", e.freshIP(), regBody("user_1", map[string]any{"inviteCode": inv.Code}))
	wantErr(t, "disabled", c, b, http.StatusForbidden, protocol.CodeForbidden, protocol.ReasonInviteInvalid)
	if c, _ := e.signed(op, http.MethodPost, "/admin/invites/nope/disable", map[string]any{}, signOpts{}); c != http.StatusNotFound {
		t.Errorf("missing: %d", c)
	}
}

// AC-ACC-09-6 的管理接口部分：没有 account.invite 的调用方 403，旧密钥也不能调用。console 角色映射随 CS-200。
func TestInviteScope(t *testing.T) {
	e, _, support := inviteEnv(t, nil)
	if c, _ := e.signed(support, http.MethodPost, "/admin/invites", map[string]any{"maxUses": 1, "expiresAt": time.Now().Add(oneDay).UnixMilli()}, signOpts{}); c != http.StatusForbidden {
		t.Errorf("support: %d", c)
	}
	if c, _ := e.adminPost("/admin/invites", map[string]any{"maxUses": 1, "expiresAt": time.Now().Add(oneDay).UnixMilli()}, adminKey); c != http.StatusForbidden {
		t.Errorf("legacy key: %d", c)
	}
}

// TC-ACC-09-05（AC-ACC-09-7）：101 个 4131、maxUses=1001 4000；边界成功。
func TestTC_ACC_09_05_Limits(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	exp := time.Now().Add(oneDay).UnixMilli()
	c, b := e.signed(op, http.MethodPost, "/admin/invites", map[string]any{"count": 101, "maxUses": 1, "expiresAt": exp}, signOpts{})
	wantErr(t, "101", c, b, http.StatusBadRequest, protocol.CodeBatchLimitExceeded, "")
	c, b = e.signed(op, http.MethodPost, "/admin/invites", map[string]any{"count": 1, "maxUses": 1001, "expiresAt": exp}, signOpts{})
	wantErr(t, "1001", c, b, http.StatusBadRequest, protocol.CodeBadRequest, "")
	if l := e.createInvite(op, 100, 1000); len(l.Invites) != 100 {
		t.Errorf("%d", len(l.Invites))
	}
}

// TC-ACC-09-07（AC-ACC-09-9）：maxUses=1，10 个并发注册恰好 1 个成功。
func TestTC_ACC_09_07_Concurrent(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	inv := e.createInvite(op, 1, 1).Invites[0]
	var wg sync.WaitGroup
	codes := make([]int, 10)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _, _ = e.post("/v1/account/register", e.freshIP(), regBody("conc_"+string(rune('a'+i))+"x", map[string]any{"inviteCode": inv.Code}))
		}()
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		} else if c != http.StatusForbidden {
			t.Errorf("%d", c)
		}
	}
	if ok != 1 {
		t.Errorf("%d succeeded: %v", ok, codes)
	}
}

// TC-ACC-09-08（AC-ACC-09-10）：至少 128 位随机，没有重复。
func TestTC_ACC_09_08_Entropy(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	seen := map[string]bool{}
	for range 10 {
		for _, inv := range e.createInvite(op, 100, 1).Invites {
			if len(inv.Code) < 22 || seen[inv.Code] { // base64url：16 字节是 22 个字符
				t.Fatalf("%q", inv.Code)
			}
			seen[inv.Code] = true
		}
	}
}

// TC-ACC-09-10（AC-ACC-09-12）：用完 91 天、过期 91 天清除，用完 89 天还在。
func TestTC_ACC_09_10_Purge(t *testing.T) {
	e, _, _ := inviteEnv(t, nil)
	clk := e.freeze()
	now := clk.now
	far := now.Add(365 * oneDay)
	e.newInvite("code-exhausted-old1", 1, far)
	e.newInvite("code-exhausted-new1", 1, far)
	e.newInvite("code-expired-oldd1", 5, now.Add(time.Hour))
	e.rawExec(`UPDATE invites SET used_count = 1, exhausted_at = ? WHERE invite_id = 'inv-code-exhausted-old1'`, now.UnixMilli())
	e.rawExec(`UPDATE invites SET used_count = 1, exhausted_at = ? WHERE invite_id = 'inv-code-exhausted-new1'`, now.Add(2*oneDay).UnixMilli())
	clk.advance(91 * oneDay)
	if c, b := e.adminPost("/admin/test/jobs/invite_purge/run", map[string]any{}, adminKey); c != http.StatusOK {
		t.Fatalf("%d %s", c, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM invites`); n != 1 {
		t.Errorf("left %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM invites WHERE invite_id = 'inv-code-exhausted-new1'`); n != 1 {
		t.Error("the one used up 89 days ago must stay")
	}
}

// TC-ACC-09-11（AC-ACC-09-13）：账号用户有信息，响应搜不到 hash、token、sid；演示用户 hasAccount=false。TC-ACC-03-33 的后台响应部分。
func TestTC_ACC_09_11_AccountInfo(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	inv := e.createInvite(op, 1, 1).Invites[0]
	r := e.register("alice_1", map[string]any{"inviteCode": inv.Code})
	e.mustLogin("alice_1", goodPassword, "ios")
	c, b := e.signed(op, http.MethodGet, "/admin/accounts/alice_1", nil, signOpts{})
	if c != http.StatusOK || !hasAll(string(b), `"hasAccount":true`, `"sessions":2`, `"createdAt"`, `"lastLoginAt"`) {
		t.Errorf("%d %s", c, b)
	}
	lower := strings.ToLower(string(b))
	for _, bad := range []string{"hash", "token", "sid", sidOf(r.RefreshToken)} {
		if strings.Contains(lower, strings.ToLower(bad)) {
			t.Errorf("response contains %q: %s", bad, b)
		}
	}
	e.users("demo_1")
	c, b = e.signed(op, http.MethodGet, "/admin/accounts/demo_1", nil, signOpts{})
	if c != http.StatusOK || !strings.Contains(string(b), `"hasAccount":false`) {
		t.Errorf("%d %s", c, b)
	}
}

// TC-ACC-09-12（AC-ACC-09-14）：maxUses=600 告警，停用后消失。走 GET /internal/v1/config 用的同一份检查。
func TestTC_ACC_09_12_InviteTotal(t *testing.T) {
	e, op, _ := inviteEnv(t, nil)
	warn := func() bool { return checkFailed(e.app.DynamicChecks(t.Context()), "invite_total") }
	if warn() {
		t.Error("warned with no invites")
	}
	inv := e.createInvite(op, 1, 600).Invites[0]
	if !warn() {
		t.Error("600 must warn")
	}
	if c, b := e.signed(op, http.MethodPost, "/admin/invites/"+inv.InviteID+"/disable", map[string]any{}, signOpts{}); c != http.StatusOK {
		t.Fatalf("%d %s", c, b)
	}
	if warn() {
		t.Error("still warned after disabling")
	}
	// 配置摘要里出现这一项。
	s := config.ConfigSummary{}.WithDynamicCheck("invite_total", "d", false)
	if !strings.Contains(strings.ToLower(jsonOf(t, s)), "invite_total") {
		t.Error("summary has no dynamic check")
	}
}

// TC-ACC-05-08（AC-ACC-05-9，服务端部分）：枚举全部管理路由和 scope：没有能设置、重置用户密码或恢复码的接口。console-server 部分随 CS-200。
func TestTC_ACC_05_08_NoResetRoute(t *testing.T) {
	e, _, _ := inviteEnv(t, nil)
	for _, r := range e.app.AdminRoutes() {
		p := strings.ToLower(r.Pattern)
		for _, bad := range []string{"password", "recovery", "reset"} {
			if strings.Contains(p, bad) && !strings.HasPrefix(p, "/admin/test/") {
				t.Errorf("%s %s", r.Method, r.Pattern)
			}
		}
	}
	if slices.Contains(adminauth.AllScopes, adminauth.Scope("account.reset")) {
		t.Error("account.reset exists")
	}
}
