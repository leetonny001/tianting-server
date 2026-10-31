// Package e2e 启动完整服务（真实 SQLite + WebSocket），用多个客户端跑端到端场景。
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/app"
	"im-mvp/server/internal/gateway"
	"im-mvp/server/internal/protocol"
)

const (
	adminKey    = "e2e-admin-key"
	waitTimeout = 3 * time.Second
)

type env struct {
	t   *testing.T
	srv *httptest.Server
}

// newEnv 启动测试模式的服务（PRD 6.1 节：故障注入、时钟注入、短有效期 token 可用）。
func newEnv(t *testing.T) *env { return newEnvWith(t, gateway.DefaultConfig(), true) }

func newEnvWithGateway(t *testing.T, gw gateway.Config) *env { return newEnvWith(t, gw, true) }

func newEnvWith(t *testing.T, gw gateway.Config, testMode bool) *env {
	t.Helper()
	a, err := app.New(app.Config{
		DBPath:    filepath.Join(t.TempDir(), "im.db"),
		JWTSecret: "e2e-secret-0123456789",
		AdminKey:  adminKey,
		Gateway:   gw,
		TestMode:  testMode,
		Version:   "e2e",
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler())
	t.Cleanup(func() {
		srv.Close()
		a.Close()
	})
	return &env{t: t, srv: srv}
}

// adminPost 调用管理接口，返回状态码和响应体。
func (e *env) adminPost(path string, body any, key string) (int, []byte) {
	e.t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-Admin-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (e *env) token(uid string) string {
	e.t.Helper()
	code, body := e.adminPost("/admin/users/token", map[string]any{"userId": uid}, adminKey)
	if code != http.StatusOK {
		e.t.Fatalf("issue token: %d %s", code, body)
	}
	var r struct{ Token string }
	json.Unmarshal(body, &r)
	return r.Token
}

func (e *env) createGroup(gid string, members ...string) {
	e.t.Helper()
	code, body := e.adminPost("/admin/groups", map[string]any{"groupId": gid, "name": gid, "members": members}, adminKey)
	if code != http.StatusCreated {
		e.t.Fatalf("create group: %d %s", code, body)
	}
}

func (e *env) wsURL(token, platform string) string {
	return strings.Replace(e.srv.URL, "http", "ws", 1) + "/ws?token=" + token + "&platform=" + platform
}

// frame 是测试里解析下行帧用的宽松结构。
type frame struct {
	ID    string          `json:"id"`
	Cmd   string          `json:"cmd"`
	Data  json.RawMessage `json:"data"`
	Error *protocol.Error `json:"error"`
}

func (f frame) msg(t *testing.T) protocol.Message {
	t.Helper()
	var m protocol.Message
	if err := json.Unmarshal(f.Data, &m); err != nil {
		t.Fatalf("decode msg: %v (%s)", err, f.Data)
	}
	return m
}

type client struct {
	t        *testing.T
	uid      string
	ws       *websocket.Conn
	in       chan frame
	pending  []frame // 等 ack 时顺带收到的其他帧
	reqN     int
	closeErr error // 读循环结束的原因；in 关闭之后才可读
}

// closeCode 等待连接关闭并返回关闭码。
func (c *client) closeCode(timeout time.Duration) websocket.StatusCode {
	c.t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-c.in:
			if !ok {
				return websocket.CloseStatus(c.closeErr)
			}
		case <-deadline:
			c.t.Fatalf("%s: connection not closed within %v", c.uid, timeout)
		}
	}
}

func (e *env) dial(uid, platform string) *client {
	e.t.Helper()
	ws, _, err := websocket.Dial(context.Background(), e.wsURL(e.token(uid), platform), nil)
	if err != nil {
		e.t.Fatalf("dial %s: %v", uid, err)
	}
	c := &client{t: e.t, uid: uid, ws: ws, in: make(chan frame, 1024)}
	go func() {
		defer close(c.in)
		for {
			_, b, err := ws.Read(context.Background())
			if err != nil {
				c.closeErr = err
				return
			}
			var f frame
			if err := json.Unmarshal(b, &f); err == nil {
				c.in <- f
			}
		}
	}()
	e.t.Cleanup(func() { ws.CloseNow() })
	return c
}

func (c *client) writeRaw(b []byte) {
	c.t.Helper()
	if err := c.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		c.t.Fatalf("%s write: %v", c.uid, err)
	}
}

func (c *client) next() frame {
	c.t.Helper()
	select {
	case f, ok := <-c.in:
		if !ok {
			c.t.Fatalf("%s: connection closed", c.uid)
		}
		return f
	case <-time.After(waitTimeout):
		c.t.Fatalf("%s: timed out waiting for frame", c.uid)
	}
	return frame{}
}

// call 发送请求并等待同 id 的 ack，期间收到的其他帧放进 pending。
func (c *client) call(cmd string, data any) frame {
	c.t.Helper()
	c.reqN++
	id := fmt.Sprintf("r%d", c.reqN)
	b, _ := json.Marshal(map[string]any{"id": id, "cmd": cmd, "data": data})
	c.writeRaw(b)
	for {
		f := c.next()
		if f.Cmd == protocol.CmdAck && f.ID == id {
			return f
		}
		c.pending = append(c.pending, f)
	}
}

// waitCmd 返回下一个指定 cmd 的下行帧（先看 pending）。
func (c *client) waitCmd(cmd string) frame {
	c.t.Helper()
	for i, f := range c.pending {
		if f.Cmd == cmd {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			return f
		}
	}
	for {
		f := c.next()
		if f.Cmd == cmd {
			return f
		}
		c.pending = append(c.pending, f)
	}
}

// expectNoMsg 断言在 d 内收不到 msg 帧。
func (c *client) expectNoMsg(d time.Duration) {
	c.t.Helper()
	for _, f := range c.pending {
		if f.Cmd == protocol.CmdMsg {
			c.t.Fatalf("%s: unexpected msg %s", c.uid, f.Data)
		}
	}
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			if f.Cmd == protocol.CmdMsg {
				c.t.Fatalf("%s: unexpected msg %s", c.uid, f.Data)
			}
			c.pending = append(c.pending, f)
		case <-deadline:
			return
		}
	}
}

func (c *client) sendText(convType, to, clientMsgID, text string) frame {
	c.t.Helper()
	return c.call(protocol.CmdSend, map[string]any{
		"convType": convType, "to": to, "clientMsgId": clientMsgID,
		"type": protocol.MsgText, "body": map[string]any{"text": text},
	})
}

func mustAck[T any](t *testing.T, f frame) T {
	t.Helper()
	if f.Error != nil {
		t.Fatalf("ack %s error: %+v", f.ID, f.Error)
	}
	var v T
	if err := json.Unmarshal(f.Data, &v); err != nil {
		t.Fatalf("decode ack: %v (%s)", err, f.Data)
	}
	return v
}

func assertAdminErr(t *testing.T, status int, body []byte, wantStatus, wantCode int) {
	t.Helper()
	var e protocol.Error
	json.Unmarshal(body, &e)
	if status != wantStatus || e.Code != wantCode {
		t.Errorf("admin error = %d %s, want HTTP %d code %d", status, body, wantStatus, wantCode)
	}
}

func mustErr(t *testing.T, f frame, code int) {
	t.Helper()
	if f.Error == nil || f.Error.Code != code {
		t.Fatalf("want error %d, got error=%+v data=%s", code, f.Error, f.Data)
	}
}

// expectNoAck 断言在 d 内收不到指定 id 的 ack（其他帧放进 pending）。
func (c *client) expectNoAck(id string, d time.Duration) {
	c.t.Helper()
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			if f.Cmd == protocol.CmdAck && f.ID == id {
				c.t.Fatalf("%s: unexpected ack %s", c.uid, id)
			}
			c.pending = append(c.pending, f)
		case <-deadline:
			return
		}
	}
}
