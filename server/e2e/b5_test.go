package e2e

// B5 非功能（BE-051 可观测）的端到端测试。

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/app"
	"im-mvp/server/internal/protocol"
)

// metric 返回 /metrics 里某一行（名称 + 标签完全匹配）的值；没有这一行返回 ""。
func (e *env) metric(series string) string {
	e.t.Helper()
	rec := httptest.NewRecorder()
	e.app.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), series+" "); ok {
			return v
		}
	}
	return ""
}

func (e *env) waitMetric(series, want string) {
	e.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if e.metric(series) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("%s = %q, want %q", series, e.metric(series), want)
}

// BE-051：在线连接、握手结果、命令和错误码、消息数、关闭码、被踢原因都有指标。
func TestMetrics(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	alice := e.dial("alice", "web")
	bob := e.dial("bob", "pc")
	e.waitMetric(`im_connections{platform="web"}`, "1")
	e.waitMetric(`im_connections{platform="pc"}`, "1")
	if got := e.metric(`im_handshakes_total{result="ok"}`); got != "2" {
		t.Errorf("handshakes ok = %q", got)
	}

	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-metric-01", "hi"))
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-metric-01", "hi")) // 去重命中，不算新消息
	mustErr(t, alice.sendText(protocol.ConvC2C, "ghost", "cid-metric-02", "?"), protocol.CodeUserNotFound)
	alice.writeRaw([]byte(`{"id":"x1","cmd":"` + strings.Repeat("z", 200) + `"}`)) // 未知命令不能制造新的标签值
	alice.waitCmd(protocol.CmdAck)
	if got := e.metric(`im_messages_total{conv_type="c2c"}`); got != "1" {
		t.Errorf("messages = %q, want 1 (dedup resend excluded)", got)
	}
	if got := e.metric(`im_commands_total{cmd="send",code="0"}`); got != "2" {
		t.Errorf("send ok = %q", got)
	}
	if got := e.metric(`im_commands_total{cmd="send",code="4042"}`); got != "1" {
		t.Errorf("send 4042 = %q", got)
	}
	if got := e.metric(`im_commands_total{cmd="unknown",code="4001"}`); got != "1" {
		t.Errorf("unknown cmd = %q", got)
	}
	if got := e.metric(`im_command_duration_seconds_count{cmd="send"}`); got != "3" {
		t.Errorf("send duration count = %q", got)
	}

	e.dialURL("bad", e.wsURL("forged", "web")).expectKicked(protocol.KickTokenInvalid, protocol.CloseTokenInvalid, waitTimeout)
	if got := e.metric(`im_handshakes_total{result="token_invalid"}`); got != "1" {
		t.Errorf("token_invalid = %q", got)
	}
	if got := e.metric(`im_kicks_total{reason="tokenInvalid"}`); got != "1" {
		t.Errorf("kicks tokenInvalid = %q", got)
	}

	// 客户端正常退出（1000）和服务端踢下线（4012）分别计数。
	bob.ws.Close(websocket.StatusNormalClosure, "logout")
	e.waitMetric(`im_disconnects_total{by="client",code="1000"}`, "1")
	e.waitMetric(`im_connections{platform="pc"}`, "0")
	e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	alice.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)
	e.waitMetric(`im_disconnects_total{by="server",code="4012"}`, "1")
	e.waitMetric(`im_connections{platform="web"}`, "0")
}

// AC-AUTH-05-1（服务端部分）、BE-051：连接关闭的日志记录关闭码、原因和关闭方。
func TestCloseIsLogged(t *testing.T) {
	logs := &syncBuffer{}
	gw := newEnvCfg(t, func(c *app.Config) {
		c.Gateway.IdleTimeout = 300 * time.Millisecond
		c.Logger = newTextLogger(logs)
	})
	c := gw.dial("alice", "web")
	c.ws.Close(websocket.StatusNormalClosure, "bye")
	idle := gw.dial("bob", "web")
	idle.closeCode(waitTimeout)

	deadline := time.Now().Add(waitTimeout)
	for !(strings.Contains(logs.String(), "uid=bob") && strings.Count(logs.String(), `msg="conn closed"`) >= 2) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	out := logs.String()
	var aliceLine, bobLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `msg="conn closed"`) && strings.Contains(line, "uid=alice") {
			aliceLine = line
		}
		if strings.Contains(line, `msg="conn closed"`) && strings.Contains(line, "uid=bob") {
			bobLine = line
		}
	}
	for _, want := range []string{"code=1000", "by=client", "reason=NORMAL", "platform=web", "duration="} {
		if !strings.Contains(aliceLine, want) {
			t.Errorf("alice close log missing %q: %s", want, aliceLine)
		}
	}
	for _, want := range []string{"code=4408", "by=server", `reason="idle timeout"`} {
		if !strings.Contains(bobLine, want) {
			t.Errorf("bob close log missing %q: %s", want, bobLine)
		}
	}
}

// BE-051、Q5a：/metrics 和 /debug/pprof/ 只在单独的内网地址上提供，对外端口上没有。
func TestMetricsOnSeparateListener(t *testing.T) {
	bin := buildServer(t)
	metricsAddr := freeAddr(t)
	srv := startServer(t, bin, filepath.Join(t.TempDir(), "im.db"), "IM_METRICS_ADDR="+metricsAddr)
	for _, path := range []string{"/metrics", "/debug/pprof/", "/debug/pprof/heap"} {
		if resp, err := http.Get("http://" + srv.addr + path); err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("public port serves %s: %d", path, resp.StatusCode)
			}
		}
	}
	// pprof 在内网地址上可用（Q5a）。
	if resp, err := http.Get("http://" + metricsAddr + "/debug/pprof/heap?debug=1"); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("pprof on metrics listener: %v %v", resp, err)
	} else {
		resp.Body.Close()
	}
	ws := srv.dial(t, srv.token(t, "alice"))
	defer ws.Close(websocket.StatusNormalClosure, "")
	resp, err := http.Get("http://" + metricsAddr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{`im_connections{platform="web"} 1`, "go_goroutines", "process_open_fds"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}
