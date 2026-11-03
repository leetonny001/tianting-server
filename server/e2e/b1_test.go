package e2e

// B1 用户与鉴权（BE-010～BE-014）的端到端测试。用例名标注对应的 AC。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func (e *env) importUsers(users ...map[string]any) (int, []byte) {
	return e.adminPost("/admin/users/import", map[string]any{"users": users}, adminKey)
}

func (e *env) getUser(uid string) (int, map[string]any) {
	e.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.adminSrv.URL+"/admin/users/"+uid, nil)
	req.Header.Set("X-Admin-Key", adminKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// AC-USER-01-1、AC-USER-01-2：导入、查询、重复导入更新昵称。
func TestImportAndGetUsers(t *testing.T) {
	e := newEnv(t)
	code, body := e.importUsers(
		map[string]any{"userId": "u1", "nickname": "小一"},
		map[string]any{"userId": "u2", "avatar": "https://x/2.png"},
		map[string]any{"userId": "u3"},
	)
	if code != http.StatusOK {
		t.Fatalf("import: %d %s", code, body)
	}
	if code, u := e.getUser("u1"); code != http.StatusOK || u["nickname"] != "小一" {
		t.Fatalf("get u1: %d %v", code, u)
	}
	e.importUsers(map[string]any{"userId": "u1", "nickname": "改名了"})
	if _, u := e.getUser("u1"); u["nickname"] != "改名了" {
		t.Errorf("nickname not updated: %v", u)
	}
	if code, u := e.getUser("ghost"); code != http.StatusNotFound || u["code"] != float64(protocol.CodeUserNotFound) {
		t.Errorf("get missing user: %d %v", code, u) // IMP-1
	}
}

// AC-USER-01-3：单次最多 500 个，501 个返回 4131。
func TestImportBatchLimit(t *testing.T) {
	e := newEnv(t)
	batch := func(n int) []map[string]any {
		out := make([]map[string]any, n)
		for i := range out {
			out[i] = map[string]any{"userId": fmt.Sprintf("b%04d", i)}
		}
		return out
	}
	code, body := e.importUsers(batch(501)...)
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	if code, body := e.importUsers(batch(500)...); code != http.StatusOK {
		t.Fatalf("500 users: %d %s", code, body)
	}
}

// AC-USER-01-4、C-16、C-17：任何一项不合法整批不写入；空数组 4000；同批重复以最后一条为准。
func TestImportValidation(t *testing.T) {
	e := newEnv(t)
	code, body := e.importUsers(map[string]any{"userId": "ok1"}, map[string]any{"userId": "bad id"})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	if code, _ := e.getUser("ok1"); code != http.StatusNotFound {
		t.Error("valid user in a rejected batch must not be written")
	}

	code, body = e.importUsers()
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	code, body = e.importUsers(map[string]any{"userId": "n1", "nickname": strings.Repeat("名", 33)})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	code, body = e.importUsers(map[string]any{"userId": "a1", "avatar": strings.Repeat("x", 513)})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	if code, body := e.importUsers(map[string]any{"userId": "n2", "nickname": strings.Repeat("名", 32)}); code != http.StatusOK {
		t.Errorf("32-char nickname: %d %s", code, body)
	}

	e.importUsers(map[string]any{"userId": "dup", "nickname": "first"}, map[string]any{"userId": "dup", "nickname": "last"})
	if _, u := e.getUser("dup"); u["nickname"] != "last" {
		t.Errorf("duplicate in batch: %v, want last", u)
	}
}

// AC-USER-01-5：签发凭证时自动注册。
func TestTokenAutoRegistersUser(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.getUser("newbie"); code != http.StatusNotFound {
		t.Fatal("precondition: newbie must not exist")
	}
	e.token("newbie")
	if code, _ := e.getUser("newbie"); code != http.StatusOK {
		t.Error("token issuance must register the user")
	}
}

// AC-AUTH-02-3：已过期的 token 连接 → kicked/tokenExpired，4011。
func TestExpiredTokenIsKicked(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenTTL("alice", 1)
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": time.Now().Add(time.Minute).UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	e.dialURL("alice", e.wsURL(tok, "web")).expectKicked(protocol.KickTokenExpired, protocol.CloseTokenExpired, waitTimeout)
}

// AC-AUTH-02-6、C-05、C-14：平台缺失 / 不合法 / 大写 → 直接以 4009 关闭，不发 kicked；
// C-08：平台优先于 token 检查。
func TestInvalidPlatformClosedWith4009(t *testing.T) {
	e := newEnv(t)
	good := e.token("alice")
	for _, tc := range []struct{ token, platform string }{
		{good, ""}, {good, "tv"}, {good, "WEB"}, {"bad-token", "tv"},
	} {
		c := e.dialURL("p:"+tc.platform, e.wsURL(tc.token, tc.platform))
		if code := c.closeCode(waitTimeout); code != protocol.CloseInvalidPlatform {
			t.Errorf("platform %q token ok=%v: close %d, want 4009", tc.platform, tc.token == good, code)
		}
		for _, f := range c.pending {
			t.Errorf("platform %q: unexpected frame %s %s", tc.platform, f.Cmd, f.Data)
		}
	}
}

// AC-AUTH-03-1：连接期间 token 到期 → 3 秒内 kicked/tokenExpired，4011。
func TestTokenExpiresWhileConnected(t *testing.T) {
	e := newEnv(t)
	c := e.dialURL("alice", e.wsURL(e.tokenTTL("alice", 2), "web"))
	c.writeRaw([]byte(`{"cmd":"ping"}`))
	c.waitCmd(protocol.CmdPong) // 先确认连接可用
	c.expectKicked(protocol.KickTokenExpired, protocol.CloseTokenExpired, 3*time.Second)
}

// AC-AUTH-03-2～AC-AUTH-03-4、C-11、C-12：强制下线。
func TestForceLogout(t *testing.T) {
	e := newEnv(t)
	oldTok := e.token("alice")
	web := e.dialURL("alice-web", e.wsURL(oldTok, "web"))
	pc := e.dialURL("alice-pc", e.wsURL(oldTok, "pc"))
	bob := e.dial("bob", "web")
	e.users("carol")

	code, body := e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	if code != http.StatusOK {
		t.Fatalf("force-logout: %d %s", code, body)
	}
	web.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)
	pc.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)

	// 其他用户不受影响。
	mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "carol", "cid-bob-after", "still here"))

	// 旧 token 重连 → banned / 4012。
	e.dialURL("alice-old", e.wsURL(oldTok, "web")).expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)

	// 强制下线后立即重新签发的 token 可以使用（毫秒精度，C-11）。
	fresh := e.dialURL("alice-new", e.wsURL(e.token("alice"), "web"))
	mustAck[protocol.SendAck](t, fresh.sendText(protocol.ConvC2C, "bob", "cid-alice-back", "back"))

	// 不存在的用户 → 404 / 4042。
	code, body = e.adminPost("/admin/users/ghost/force-logout", map[string]any{}, adminKey)
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
}

// C-08：同时被强制下线又已过期的 token，报 4012（强制下线优先于过期）。
func TestRevokedBeatsExpired(t *testing.T) {
	e := newEnv(t)
	tok := e.tokenTTL("alice", 1)
	e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": time.Now().Add(time.Minute).UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	e.dialURL("alice", e.wsURL(tok, "web")).expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)
}

// AC-AUTH-04-1、AC-AUTH-04-3、AC-AUTH-04-4、C-21：演示登录。
func TestDemoLogin(t *testing.T) {
	off := newEnv(t)
	if code, _ := off.publicPost("/demo/login", map[string]any{"username": "alice"}); code != http.StatusNotFound {
		t.Errorf("demo login disabled: %d, want 404", code)
	}

	e := newEnvCfg(t, func(c *app.Config) { c.DemoLogin = true })
	e.users("bob")
	code, body := e.publicPost("/demo/login", map[string]any{"username": "  alice  "}) // 不带管理密钥
	if code != http.StatusOK {
		t.Fatalf("demo login: %d %s", code, body)
	}
	var r struct {
		UserID    string
		Token     string
		ExpiresAt int64
	}
	json.Unmarshal(body, &r)
	if r.UserID != "alice" {
		t.Errorf("userId = %q, want trimmed", r.UserID)
	}
	if d := time.Until(time.UnixMilli(r.ExpiresAt)); d < 23*time.Hour || d > 25*time.Hour {
		t.Errorf("demo token ttl %v, want ~1 day", d)
	}
	c := e.dialURL("alice", e.wsURL(r.Token, "web"))
	mustAck[protocol.SendAck](t, c.sendText(protocol.ConvC2C, "bob", "cid-demo-0001", "hi"))

	for _, bad := range []string{"a b", "", "   "} {
		code, body := e.publicPost("/demo/login", map[string]any{"username": bad})
		assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	}
}

// BE-091（PRD-ui G-UI-23）：演示登录带昵称时写入昵称；33 个码点 400 / 4000 且不签发；不带昵称不改原昵称。
func TestDemoLoginNickname(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) { c.DemoLogin = true })
	if code, body := e.publicPost("/demo/login", map[string]any{"username": "zhangsan", "nickname": "  张三  "}); code != http.StatusOK {
		t.Fatalf("login with nickname: %d %s", code, body)
	}
	if _, u := e.getUser("zhangsan"); u["nickname"] != "张三" {
		t.Errorf("nickname = %v", u["nickname"])
	}
	code, body := e.publicPost("/demo/login", map[string]any{"username": "lisi", "nickname": strings.Repeat("字", 33)})
	if code != http.StatusBadRequest || !strings.Contains(string(body), `"code":4000`) || strings.Contains(string(body), "token") {
		t.Errorf("33 code points: %d %s", code, body)
	}
	if code, _ := e.getUser("lisi"); code != http.StatusNotFound {
		t.Errorf("rejected login must not register the user: %d", code)
	}
	e.publicPost("/demo/login", map[string]any{"username": "zhangsan"})
	if _, u := e.getUser("zhangsan"); u["nickname"] != "张三" {
		t.Errorf("login without nickname changed it: %v", u["nickname"])
	}
}
