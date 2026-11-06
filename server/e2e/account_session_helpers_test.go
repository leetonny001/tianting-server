package e2e

// 第二段账号 e2e 的公共设施：刷新、时钟、指标、解 access 声明。

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// frozenClock 冻结 [IM 钟]，从 Asia/Shanghai 当天 00:00 起算（R-CQ-41）。
type frozenClock struct {
	e   *acctEnv
	now time.Time
}

func (e *acctEnv) freeze() *frozenClock {
	e.t.Helper()
	c := &frozenClock{e: e, now: shanghaiMidnight(e.t)}
	e.setClock(c.now)
	return c
}

func (c *frozenClock) advance(d time.Duration) {
	c.e.t.Helper()
	c.now = c.now.Add(d)
	c.e.setClock(c.now)
}

const oneDay = 24 * time.Hour

func (e *acctEnv) refresh(token string) (int, []byte) {
	e.t.Helper()
	code, b, _ := e.post("/v1/account/token/refresh", e.freshIP(), map[string]any{"refreshToken": token})
	return code, b
}

func (e *acctEnv) mustRefresh(token string) authResp {
	e.t.Helper()
	code, b := e.refresh(token)
	if code != http.StatusOK {
		e.t.Fatalf("refresh: %d %s", code, b)
	}
	validateHTTP(e.t, "AuthResponse", b)
	var r authResp
	if err := json.Unmarshal(b, &r); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// wantRefreshErr 断言 401 / 4019 和 reason，返回错误。
func (e *acctEnv) wantRefreshErr(label, token, reason string) apiErr {
	e.t.Helper()
	code, b := e.refresh(token)
	return wantErr(e.t, label, code, b, http.StatusUnauthorized, protocol.CodeRefreshTokenInvalid, reason)
}

// sidOf 从 refresh token 取 sid。只有测试这样解。
func sidOf(token string) string { return strings.Split(token, ".")[1] }

func genOf(t *testing.T, token string) int64 {
	t.Helper()
	g, err := strconv.ParseInt(strings.Split(token, ".")[2], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func (e *acctEnv) session(sid string) store.DeviceSession {
	e.t.Helper()
	d, err := e.st.GetDeviceSession(context.Background(), sid)
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

// claimsOf 解 access token 的载荷（不验签）。
func claimsOf(t *testing.T, access string) map[string]any {
	t.Helper()
	parts := strings.Split(access, ".")
	if len(parts) != 3 {
		t.Fatalf("not a jwt: %q", access)
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// metric 读一个没有标签的计数器。
func (e *acctEnv) metric(name string) float64 {
	e.t.Helper()
	for line := range strings.Lines(e.metricsText()) {
		if v, ok := strings.CutPrefix(line, name+" "); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				e.t.Fatal(err)
			}
			return f
		}
	}
	e.t.Fatalf("metric %s not found", name)
	return 0
}

// authOK 用 token 连接并断言成功（没有 kicked）。
func (e *acctEnv) authOK(label, token, platform string) *client {
	e.t.Helper()
	c := e.connect(label, token, platform)
	if len(c.pending) != 0 {
		e.t.Fatalf("%s: auth rejected: %s", label, c.pending[0].Data)
	}
	e.t.Cleanup(func() { c.ws.CloseNow() })
	return c
}

// authKicked 用 token 连接并断言被踢。
func (e *acctEnv) authKicked(label, token, reason string, closeCode int) {
	e.t.Helper()
	e.connect(label, token, testPlatform).expectKicked(reason, closeCode, waitTimeout)
}

func (e *acctEnv) events(uid, kind string) []store.AccountEvent {
	e.t.Helper()
	evs, err := e.st.ListAccountEvents(context.Background(), uid, 200)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []store.AccountEvent
	for _, ev := range evs {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	return out
}

// bearerPost 带 Bearer 发 /v1/*。
func (e *acctEnv) bearerPost(path, bearer, ip string, body any) (int, []byte) {
	e.t.Helper()
	return e.bearerDo(http.MethodPost, path, bearer, ip, body)
}

// waitKicked 等待已连接的客户端被踢，并返回事件（用来断言 recoverBefore）。
func waitKicked(c *client, reason string, closeCode int) protocol.KickedEvent {
	c.t.Helper()
	f := c.waitCmdTimeout(protocol.CmdEvent, waitTimeout)
	var ev protocol.KickedEvent
	json.Unmarshal(f.Data, &ev)
	if ev.Type != protocol.EventKicked || ev.Reason != reason {
		c.t.Fatalf("%s: got event %s, want kicked/%s", c.uid, f.Data, reason)
	}
	if code := c.closeCode(waitTimeout); int(code) != closeCode {
		c.t.Fatalf("%s: close code %d, want %d", c.uid, code, closeCode)
	}
	return ev
}

func (e *acctEnv) bearerDo(method, path, bearer, ip string, body any) (int, []byte) {
	e.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		e.t.Fatal(err)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func mustJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func defaultGateway() gateway.Config { return gateway.DefaultConfig() }

func checkFailed(checks []app.DynamicCheck, id string) bool {
	for _, c := range checks {
		if c.ID == id {
			return !c.Passed
		}
	}
	return false
}

// execLedger 把台账里已退役的 k1 推到 at（模拟时间过去）。
func execLedger(path string, at int64) error {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`UPDATE refresh_key_ledger SET retired_at = ? WHERE kid = 'k1'`, at)
	return err
}

// ban 用存储层封禁（封禁接口要签名调用方，这里只关心吊销）。
func (e *acctEnv) ban(uid string) {
	e.t.Helper()
	if err := e.st.BanUser(context.Background(), store.Ban{UserID: uid, Reason: "test", By: "t", At: time.Now().UnixMilli()}); err != nil {
		e.t.Fatal(err)
	}
}
