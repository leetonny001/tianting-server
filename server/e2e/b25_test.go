package e2e

// B2.5 架构与安全基座（BE-064、BE-065）的端到端测试。用例名标注对应的 AC。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/app"
	"im-mvp/server/internal/protocol"
)

func (e *env) platformURL(platform string) string {
	return strings.Replace(e.srv.URL, "http", "ws", 1) + "/ws?platform=" + platform
}

func shortAuthTimeout(c *app.Config) { c.Gateway.AuthTimeout = 300 * time.Millisecond }

// AC-AUTH-02-7（TC-AUTH-02-15）：连接后不发 auth → 4401，关闭前不下发任何帧。
func TestAuthTimeout(t *testing.T) {
	e := newEnvCfg(t, shortAuthTimeout)
	c := e.dialRaw("silent", e.platformURL("web"), nil)
	if code := c.closeCode(waitTimeout); code != protocol.CloseAuthRequired {
		t.Fatalf("close %d, want 4401", code)
	}
	for _, f := range c.pending {
		t.Errorf("unexpected frame %s", f.Cmd)
	}
}

// AC-AUTH-02-7（TC-AUTH-02-16）：auth 之前发送其他命令 → 4401，命令不被执行。
func TestCommandBeforeAuth(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	bob := e.dial("bob", "web")
	for _, first := range []string{
		`{"id":"r1","cmd":"send","data":{"convType":"c2c","to":"bob","clientMsgId":"cid-pre-auth","type":"text","body":{"text":"x"}}}`,
		`{"cmd":"ping"}`,
		`not json`,
	} {
		c := e.dialRaw("pre-auth", e.platformURL("web"), nil)
		c.writeRaw([]byte(first))
		if code := c.closeCode(waitTimeout); code != protocol.CloseAuthRequired {
			t.Errorf("first frame %s: close %d, want 4401", first, code)
		}
	}
	bob.expectNoMsg(200 * time.Millisecond)
}

// AC-AUTH-02-8（TC-AUTH-02-17）：URL 里的 token 被忽略，不发 auth 不会上线。
func TestTokenInURLIsIgnored(t *testing.T) {
	e := newEnvCfg(t, shortAuthTimeout)
	c := e.dialRaw("url-token", e.wsURL(e.token("alice"), "web"), nil)
	if code := c.closeCode(waitTimeout); code != protocol.CloseAuthRequired {
		t.Fatalf("close %d, want 4401", code)
	}
}

// AC-AUTH-02-9（TC-AUTH-02-18）：auth 的 ack 带 userId 和服务端时间；再次 auth 返回 4000，连接不断开。
func TestAuthAck(t *testing.T) {
	e := newEnv(t)
	now := time.UnixMilli(1_800_000_000_000)
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": now.UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	tok := e.tokenTTL("alice", 3600)

	c := e.dialRaw("alice", e.platformURL("ios"), nil)
	a := mustAck[protocol.AuthAck](t, c.call(protocol.CmdAuth, map[string]any{"token": tok}))
	if a.UserID != "alice" || a.ServerTime != now.UnixMilli() {
		t.Fatalf("auth ack = %+v", a)
	}
	mustErr(t, c.call(protocol.CmdAuth, map[string]any{"token": tok}), protocol.CodeBadRequest)
	c.writeRaw([]byte(`{"cmd":"ping"}`))
	c.waitCmd(protocol.CmdPong)
}

// C-08 在首帧 auth 下仍然成立：auth 帧里 token 缺失或格式错误 → kicked/tokenInvalid + 4010。
func TestAuthFrameWithBadToken(t *testing.T) {
	e := newEnv(t)
	for _, data := range []string{`{}`, `{"token":""}`, `{"token":123}`, `"x"`} {
		c := e.dialRaw("bad", e.platformURL("web"), nil)
		c.writeRaw([]byte(`{"id":"a1","cmd":"auth","data":` + data + `}`))
		c.expectKicked(protocol.KickTokenInvalid, protocol.CloseTokenInvalid, waitTimeout)
	}
}

// AC-AUTH-02-10（TC-AUTH-02-19）、C-53：第 11 个连接上线时，最早的连接被踢（4429），之后收不到消息。
func TestTooManyConnections(t *testing.T) {
	e := newEnv(t)
	bob := e.dial("bob", "web")
	tok := e.token("alice")
	conns := make([]*client, 11)
	for i := range 10 {
		conns[i] = e.dialURL(fmt.Sprintf("alice-%d", i), e.wsURL(tok, "web"))
	}
	conns[1].expectNoMsg(100 * time.Millisecond) // 10 个以内谁都不会被踢

	conns[10] = e.dialURL("alice-10", e.wsURL(tok, "web"))
	conns[0].expectKicked(protocol.KickTooManyConns, protocol.CloseTooManyConns, waitTimeout)

	a := mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", "cid-many-001", "hi all"))
	for _, c := range conns[1:] {
		if m := c.waitCmd(protocol.CmdMsg).msg(t); m.MsgID != a.MsgID {
			t.Errorf("%s got %+v", c.uid, m)
		}
	}
}

// AC-AUTH-02-11（TC-AUTH-02-21）、C-54：Origin 白名单。
func TestOriginAllowlist(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) { c.Gateway.AllowedOrigins = []string{"https://im.example.com"} })
	tok := e.token("alice")
	dialWith := func(origin string) (*websocket.Conn, *http.Response, error) {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		return websocket.Dial(context.Background(), e.platformURL("web"), &websocket.DialOptions{HTTPHeader: h})
	}

	for _, bad := range []string{"https://evil.example.com", "http://im.example.com", "http://localhost:5173"} {
		ws, resp, err := dialWith(bad)
		if err == nil {
			ws.CloseNow()
			t.Errorf("origin %s: handshake succeeded, want 403", bad)
			continue
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Errorf("origin %s: resp=%v err=%v, want 403", bad, resp, err)
		}
	}
	for _, good := range []string{"https://im.example.com", ""} { // 空：原生端没有 Origin 头
		ws, _, err := dialWith(good)
		if err != nil {
			t.Errorf("origin %q: %v", good, err)
			continue
		}
		ws.Write(context.Background(), websocket.MessageText, []byte(`{"id":"a1","cmd":"auth","data":{"token":"`+tok+`"}}`))
		_, b, err := ws.Read(context.Background())
		var f frame
		json.Unmarshal(b, &f)
		if err != nil || f.Error != nil || f.ID != "a1" {
			t.Errorf("origin %q auth: %s %v", good, b, err)
		}
		ws.CloseNow()
	}
}

// C-54：默认（未配置白名单）只允许本机页面。
func TestDefaultOriginsAreLocalOnly(t *testing.T) {
	e := newEnv(t)
	h := http.Header{"Origin": {"https://evil.example.com"}}
	if ws, resp, err := websocket.Dial(context.Background(), e.platformURL("web"), &websocket.DialOptions{HTTPHeader: h}); err == nil {
		ws.CloseNow()
		t.Error("foreign origin accepted by default")
	} else if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("foreign origin: %v %v", resp, err)
	}
	h.Set("Origin", "http://localhost:5173")
	ws, _, err := websocket.Dial(context.Background(), e.platformURL("web"), &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatalf("local dev origin rejected: %v", err)
	}
	ws.CloseNow()
}

// ARCH S7（BE-066）：按 IP 的握手限流在升级前返回 429；连续用无效 token 鉴权后整个 IP 被临时封禁，
// 封禁期间即使 token 有效也连不上。
func TestHandshakeGuard(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) {
		c.Gateway.Guard.HandshakeBurst = 1000
		c.Gateway.Guard.AuthFailBurst = 2
		c.Gateway.Guard.AuthFailPer = time.Hour
		c.Gateway.Guard.BlockFor = time.Minute
	})
	for range 2 {
		e.dialURL("attacker", e.wsURL("forged-token", "web")).expectKicked(protocol.KickTokenInvalid, protocol.CloseTokenInvalid, waitTimeout)
	}
	// 第 3 次失败触发封禁（这次的 kicked 仍然正常下发）。
	e.dialURL("attacker", e.wsURL("forged-token", "web")).expectKicked(protocol.KickTokenInvalid, protocol.CloseTokenInvalid, waitTimeout)
	if _, resp, err := websocket.Dial(context.Background(), e.platformURL("web"), nil); err == nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("blocked IP handshake: resp=%v err=%v, want 429", resp, err)
	}

	// 封禁到期后恢复。
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": time.Now().Add(2 * time.Minute).UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	c := e.dialRaw("alice", e.platformURL("web"), nil)
	mustAck[protocol.AuthAck](t, c.call(protocol.CmdAuth, map[string]any{"token": e.tokenTTL("alice", 3600)}))
}

func TestHandshakeRateLimit(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) {
		c.Gateway.Guard.HandshakeRate = 1
		c.Gateway.Guard.HandshakeBurst = 3
	})
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": time.Now().UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	for range 3 {
		ws, _, err := websocket.Dial(context.Background(), e.platformURL("web"), nil)
		if err != nil {
			t.Fatal(err)
		}
		ws.CloseNow()
	}
	if _, resp, err := websocket.Dial(context.Background(), e.platformURL("web"), nil); err == nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("4th handshake: resp=%v err=%v, want 429", resp, err)
	}
}

// syncBuffer 是并发安全的日志缓冲区。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ARCH S9（BE-066）：日志里不能出现 token、管理密钥和消息正文。
// 走一遍会产生日志的路径（签发、上线、发消息、被拒绝、强制下线、出错），再扫描全部日志输出。
func TestLogsDoNotLeakSecrets(t *testing.T) {
	logs := &syncBuffer{}
	e := newEnvCfg(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	const secretText = "TOP-SECRET-MESSAGE-BODY-7f3a"
	tok := e.token("alice")
	e.users("bob")
	alice := e.dialURL("alice", e.wsURL(tok, "web"))
	bob := e.dial("bob", "pc")
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-secret-01", secretText))
	bob.waitCmd(protocol.CmdMsg)
	mustErr(t, alice.sendText(protocol.ConvC2C, "nobody", "cid-secret-02", secretText), protocol.CodeUserNotFound)

	forged := tok[:len(tok)-4] + "AAAA" // 签名被篡改的 token
	e.dialURL("forged", e.wsURL(forged, "web")).expectKicked(protocol.KickTokenInvalid, protocol.CloseTokenInvalid, waitTimeout)
	e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	e.adminPost("/admin/users/token", map[string]any{"userId": "bad id"}, "wrong-admin-key-xyz")
	alice.closeCode(waitTimeout)

	out := logs.String()
	if !strings.Contains(out, "conn open") {
		t.Fatalf("logger not wired, nothing to check:\n%s", out)
	}
	for name, secret := range map[string]string{
		"token":         tok,
		"token payload": strings.Split(tok, ".")[1],
		"forged token":  forged,
		"admin key":     adminKey,
		"wrong key":     "wrong-admin-key-xyz",
		"message body":  secretText,
	} {
		if strings.Contains(out, secret) {
			t.Errorf("%s leaked into logs", name)
		}
	}
}
