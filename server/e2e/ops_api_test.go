package e2e

// 节点运维接口（PRD-ops F-OPS-16、F-OPS-08、F-OPS-09、F-OPS-10、F-OPS-11，BE-300～BE-306）。

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/auth"
	"tianting/server/internal/clock"
	"tianting/server/internal/config"
	"tianting/server/internal/gateway"
	"tianting/server/internal/opsapi"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

type opsEnv struct {
	*env
	ops  *httptest.Server
	key  ed25519.PrivateKey
	node string
	logs *syncBuffer
}

func newOpsEnv(t *testing.T, mutate func(*app.Config)) *opsEnv {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	logs := &syncBuffer{}
	gw := gateway.DefaultConfig()
	gw.NodeID = "n1"
	gw.AuthTimeout = 150 * time.Millisecond
	e := newEnvCfg(t, func(c *app.Config) {
		c.Gateway = gw
		c.Version = "v-test"
		c.OpsKeys = []opsapi.Key{{ID: "k1", Public: pub}}
		c.Logger = slog.New(slog.NewTextHandler(logs, nil))
		c.ConfigSummary = func(version, nodeID string) any {
			return map[string]string{"version": version, "nodeId": nodeID}
		}
		if mutate != nil {
			mutate(c)
		}
	})
	ops := httptest.NewServer(e.app.OpsHandler())
	t.Cleanup(ops.Close)
	return &opsEnv{env: e, ops: ops, key: priv, node: "n1", logs: logs}
}

type opsReq struct {
	key       ed25519.PrivateKey
	caller    string
	node      *string
	ts        time.Time
	nonce     string
	sendBody  []byte
	operator  string
	requestID string
	noSign    bool
	badSig    bool
}

func (e *opsEnv) call(method, path string, body any, o opsReq) (int, []byte) {
	e.t.Helper()
	return callOps(e.t, e.ops.URL, e.key, e.node, method, path, body, o)
}

func callOps(t *testing.T, base string, defKey ed25519.PrivateKey, defNode, method, path string, body any, o opsReq) (int, []byte) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	key := o.key
	if key == nil {
		key = defKey
	}
	node := defNode
	if o.node != nil {
		node = *o.node
	}
	ts := o.ts
	if ts.IsZero() {
		ts = time.Now()
	}
	nonce := o.nonce
	if nonce == "" {
		b := make([]byte, 18)
		_, _ = rand.Read(b)
		nonce = base64.RawURLEncoding.EncodeToString(b)
	}
	caller := o.caller
	if caller == "" {
		caller = opsapi.CallerID
	}
	tsStr := strconv.FormatInt(ts.UnixMilli(), 10)
	sts := adminauth.StringToSign(adminauth.Request{Method: method, Path: path, Timestamp: tsStr, Nonce: nonce, Body: raw, Operator: o.operator, RequestID: o.requestID, TargetNode: node})
	send := raw
	if o.sendBody != nil {
		send = o.sendBody
	}
	req, _ := http.NewRequest(method, base+path, bytes.NewReader(send))
	if !o.noSign {
		sig := ed25519.Sign(key, []byte(sts))
		if o.badSig {
			sig[0] ^= 0xff
		}
		req.Header.Set("X-IM-Signature", adminauth.EncodeSignature(sig))
		req.Header.Set("X-IM-Caller", caller)
		req.Header.Set("X-IM-Timestamp", tsStr)
		req.Header.Set("X-IM-Nonce", nonce)
		req.Header.Set("X-IM-Target-Node", node)
	}
	if o.operator != "" {
		req.Header.Set("X-IM-Operator", o.operator)
	}
	if o.requestID != "" {
		req.Header.Set("X-Request-Id", o.requestID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (e *opsEnv) mustJSON(method, path string, body, out any) {
	e.t.Helper()
	code, b := e.call(method, path, body, opsReq{})
	if code != 200 {
		e.t.Fatalf("%s %s: %d %s", method, path, code, b)
	}
	if err := json.Unmarshal(b, out); err != nil {
		e.t.Fatal(err)
	}
}

// AC-OPS-16-1、16-3：5 种签名失败和篡改请求体都是 401 / 4017，不返回数据；访问日志写原因，不写签名。
func TestOPS16SignatureFailures(t *testing.T) {
	e := newOpsEnv(t, nil)
	e.users("u1")
	path := "/internal/v1/connections:query"
	body := map[string]any{"userId": "u1"}
	const nonce = "nonce-reused-0123456789"
	if code, b := e.call("POST", path, body, opsReq{nonce: nonce}); code != 200 {
		t.Fatalf("baseline: %d %s", code, b)
	}
	other := "n2"
	for _, tc := range []struct {
		name, failure string
		o             opsReq
	}{
		{"unsigned", "missing_signature", opsReq{noSign: true}},
		{"bad signature", "signature_mismatch", opsReq{badSig: true}},
		{"6 minutes early", "timestamp_expired", opsReq{ts: time.Now().Add(-6 * time.Minute)}},
		{"reused nonce", "nonce_reused", opsReq{nonce: nonce}},
		{"wrong node", "target_node_mismatch", opsReq{node: &other}},
		{"tampered body", "signature_mismatch", opsReq{sendBody: []byte(`{"userId":"u2"}`)}},
		{"other caller", "unknown_caller", opsReq{caller: "biz1"}},
	} {
		code, b := e.call("POST", path, body, tc.o)
		expectCode(t, tc.name, code, b, 401, 4017)
		if strings.Contains(string(b), "connections") {
			t.Errorf("%s returned data: %s", tc.name, b)
		}
		if !strings.Contains(e.logs.String(), "failure="+tc.failure) {
			t.Errorf("%s: no log with failure=%s", tc.name, tc.failure)
		}
	}
	if got := e.logs.String(); strings.Contains(got, "X-IM-Signature") {
		t.Error("signature header logged")
	}
}

// AC-OPS-16-6：两把公钥同时有效；去掉 k1 后 k1 签名的请求 401。
func TestOPS16KeyRotation(t *testing.T) {
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	e := newOpsEnv(t, nil)
	k1Pub := e.key.Public().(ed25519.PublicKey)
	both := newOpsEnv(t, func(c *app.Config) { c.OpsKeys = []opsapi.Key{{ID: "k1", Public: k1Pub}, {ID: "k2", Public: pub2}} })
	both.key = e.key
	for _, k := range []ed25519.PrivateKey{e.key, priv2} {
		if code, b := both.call("GET", "/internal/v1/node", nil, opsReq{key: k}); code != 200 {
			t.Errorf("both keys valid: %d %s", code, b)
		}
	}
	if !strings.Contains(both.logs.String(), "kid=k2") {
		t.Error("access log should name the key id")
	}
	onlyK2 := newOpsEnv(t, func(c *app.Config) { c.OpsKeys = []opsapi.Key{{ID: "k2", Public: pub2}} })
	code, b := onlyK2.call("GET", "/internal/v1/node", nil, opsReq{key: e.key})
	expectCode(t, "removed key", code, b, 401, 4017)
	if code, _ := onlyK2.call("GET", "/internal/v1/node", nil, opsReq{key: priv2}); code != 200 {
		t.Errorf("k2 = %d", code)
	}
}

// AC-OPS-16-7：作用域隔离。
func TestOPS16Isolation(t *testing.T) {
	e := newOpsEnv(t, nil)
	// 对外端口没有运维路径。
	if code, _ := e.do(e.srv.URL, "GET", "/internal/v1/node", nil, ""); code != 404 {
		t.Errorf("public port = %d", code)
	}
	// console-ops 不能调用管理接口（不在调用方列表里）。
	req := e.signedRequest(testCaller{cfg: adminauth.Caller{ID: opsapi.CallerID, Alg: adminauth.AlgEd25519}, priv: e.key}, "GET", "/admin/users/u1", nil, signOpts{})
	code, b := e.send(req)
	expectCode(t, "console-ops on admin", code, b, 401, 4017)
	// 管理调用方不能调用运维接口。
	code, b = e.call("GET", "/internal/v1/node", nil, opsReq{caller: "biz1"})
	expectCode(t, "admin caller on ops", code, b, 401, 4017)
}

// AC-OPS-16-8：同时最多 4 个，第 5 个立即 429 / 4290。
func TestOPS16ConcurrencyGate(t *testing.T) {
	e := newOpsEnv(t, nil)
	if code, b := e.adminPost("/admin/test/faults", map[string]any{"opsDelayMs": 500}, adminKey); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	results := make(chan int, 5)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := e.call("GET", "/internal/v1/config", nil, opsReq{})
			results <- code
		}()
	}
	time.Sleep(150 * time.Millisecond)
	start := time.Now()
	code, b := e.call("GET", "/internal/v1/config", nil, opsReq{})
	expectCode(t, "fifth", code, b, 429, 4290)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("fifth took %v, should be immediate", d)
	}
	wg.Wait()
	close(results)
	for c := range results {
		if c != 200 {
			t.Errorf("first four: %d", c)
		}
	}
	e.adminPost("/admin/test/faults", map[string]any{"opsDelayMs": 0}, adminKey)
	// 超过 2 秒处理超时：返回错误，不一直挂着。
	e.adminPost("/admin/test/faults", map[string]any{"opsDelayMs": 5000}, adminKey)
	code, b = e.call("GET", "/internal/v1/config", nil, opsReq{})
	expectCode(t, "timeout", code, b, 503, 5030)
}

// AC-OPS-16-9：来源网段不允许：403 / 4030，访问日志写原因，没有进入签名校验。
func TestOPS16CIDR(t *testing.T) {
	e := newOpsEnv(t, func(c *app.Config) { c.OpsAllowedCIDRs = []netip.Prefix{netip.MustParsePrefix("10.8.2.0/24")} })
	code, b := e.call("GET", "/internal/v1/node", nil, opsReq{})
	expectCode(t, "cidr", code, b, 403, 4030)
	if !strings.Contains(e.logs.String(), "failure=source_not_allowed") {
		t.Error("no log for cidr rejection")
	}
	// 没有签名也是 403：说明在签名校验之前就拒绝了。
	code, b = e.call("GET", "/internal/v1/node", nil, opsReq{noSign: true})
	expectCode(t, "cidr unsigned", code, b, 403, 4030)
}

// AC-OPS-16-10：节点接口不触发 STW。
func TestOPS16NodeNoSTW(t *testing.T) {
	e := newOpsEnv(t, nil)
	h := e.app.OpsHandler()
	sample := []metrics.Sample{{Name: "/sched/pauses/total/other:seconds"}}
	read := func() uint64 {
		metrics.Read(sample)
		var n uint64
		for _, c := range sample[0].Value.Float64Histogram().Counts {
			n += c
		}
		return n
	}
	// 直接调用 handler，排除 HTTP 客户端；签名验证照常。
	before := read()
	for i := range 1000 {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		nonce := fmt.Sprintf("nonce-for-stw-test-%06d", i)
		sts := adminauth.StringToSign(adminauth.Request{Method: "GET", Path: "/internal/v1/node", Timestamp: ts, Nonce: nonce, TargetNode: "n1"})
		r := httptest.NewRequest("GET", "/internal/v1/node", nil)
		r.Header.Set("X-IM-Caller", opsapi.CallerID)
		r.Header.Set("X-IM-Timestamp", ts)
		r.Header.Set("X-IM-Nonce", nonce)
		r.Header.Set("X-IM-Target-Node", "n1")
		r.Header.Set("X-IM-Signature", adminauth.EncodeSignature(ed25519.Sign(e.key, []byte(sts))))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	if d := read() - before; d != 0 {
		t.Errorf("/sched/pauses/total/other increased by %d", d)
	}
}

// AC-OPS-05-1、04-1、04-4：节点信息；按平台连接数；draining 字段独立于 ready。
func TestOPS05Node(t *testing.T) {
	e := newOpsEnv(t, nil)
	web1, _, pc := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u3", "pc")
	// auth 的 ack 先发、再登记到在线表（保证客户端先收到 ack），所以拨号返回后要等一下。
	var n opsapi.NodeInfo
	waitFor(t, "three connections", func() bool {
		e.mustJSON("GET", "/internal/v1/node", nil, &n)
		return n.TotalConns == 3
	})
	if n.NodeID != "n1" || n.Version != "v-test" || !n.Ready || n.Draining || n.StartedAt == 0 || n.Goroutines == 0 || n.LiveBytes == 0 {
		t.Errorf("node = %+v", n)
	}
	if n.TotalConns != 3 || n.Connections["web"] != 2 || n.Connections["pc"] != 1 {
		t.Errorf("connections = %+v", n.Connections)
	}
	web1.ws.CloseNow()
	waitFor(t, "web count drops", func() bool {
		var m opsapi.NodeInfo
		e.mustJSON("GET", "/internal/v1/node", nil, &m)
		return m.Connections["web"] == 1 && m.TotalConns == 2
	})
	_ = pc
	e.app.StopAccepting()
	e.mustJSON("GET", "/internal/v1/node", nil, &n)
	if n.Ready || !n.Draining {
		t.Errorf("draining node = ready %v draining %v", n.Ready, n.Draining)
	}
	e.waitMetric(`im_draining`, "1")
	e.waitMetric(`im_ready`, "0")
}

// waitConns 等连接登记到在线表（auth 的 ack 先于登记发出）。
func (e *opsEnv) waitConns(uid string, want int) {
	e.t.Helper()
	waitFor(e.t, uid+" connections", func() bool {
		var res struct {
			Connections []map[string]any `json:"connections"`
		}
		e.mustJSON("POST", "/internal/v1/connections:query", map[string]any{"userId": uid}, &res)
		return len(res.Connections) == want
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// AC-OPS-08-1、08-2、08-6、08-7：当前连接；不在线和不存在响应相同；字段白名单；访问日志不含 userId。
func TestOPS08Connections(t *testing.T) {
	e := newOpsEnv(t, nil)
	e.users("u1", "u2")
	e.dial("u1", "web")
	e.dial("u1", "pc")
	e.waitConns("u1", 2)
	code, raw := e.call("POST", "/internal/v1/connections:query", map[string]any{"userId": "u1"}, opsReq{operator: "ops:a1", requestID: "req-conn-1"})
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	var res struct {
		Connections []map[string]any `json:"connections"`
	}
	_ = json.Unmarshal(raw, &res)
	if len(res.Connections) != 2 {
		t.Fatalf("connections = %s", raw)
	}
	allowed := []string{"connId", "platform", "connectedAt", "node", "ip", "queueLen"}
	for _, c := range res.Connections {
		for k := range c {
			if !slices.Contains(allowed, k) {
				t.Errorf("unexpected field %s", k)
			}
		}
		if !strings.HasSuffix(c["ip"].(string), "*") || c["node"] != "n1" {
			t.Errorf("conn = %v", c)
		}
	}
	_, offline := e.call("POST", "/internal/v1/connections:query", map[string]any{"userId": "u2"}, opsReq{})
	_, missing := e.call("POST", "/internal/v1/connections:query", map[string]any{"userId": "u999"}, opsReq{})
	if string(offline) != string(missing) {
		t.Errorf("offline %s vs missing %s", offline, missing)
	}
	var line string
	for _, l := range strings.Split(e.logs.String(), "\n") {
		if strings.Contains(l, "requestId=req-conn-1") {
			line = l
		}
	}
	if line == "" || !strings.Contains(line, "count=2") || !strings.Contains(line, "operator=ops:a1") || !strings.Contains(line, "kid=k1") || !strings.Contains(line, "endpoint=connections") {
		t.Errorf("access log = %q", line)
	}
	if strings.Contains(line, "u1") {
		t.Errorf("access log contains the userId: %q", line)
	}
}

type disconnectRes struct {
	Records []struct {
		ConnID   string `json:"connId"`
		Phase    string `json:"phase"`
		Platform string `json:"platform"`
		Code     int    `json:"code"`
		Reason   string `json:"reason"`
		By       string `json:"by"`
		TS       int64  `json:"ts"`
		IP       string `json:"ip"`
	} `json:"records"`
	BufferFrom int64 `json:"bufferFrom"`
}

func (e *opsEnv) disconnects(uid string) disconnectRes {
	e.t.Helper()
	var r disconnectRes
	e.mustJSON("POST", "/internal/v1/disconnects:query", map[string]any{"userId": uid}, &r)
	return r
}

// AC-OPS-09-1、09-3、09-4、06-3、06-9：强制下线；旧 token 重连；过期 token。
func TestOPS09Kicked(t *testing.T) {
	e := newOpsEnv(t, nil)
	e.users("u1")
	old := e.token("u1")
	c := e.dial("u1", "web")
	e.adminPost("/admin/users/u1/force-logout", map[string]any{}, adminKey)
	c.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)
	waitFor(t, "session record", func() bool { return len(e.disconnects("u1").Records) == 1 })
	r := e.disconnects("u1").Records[0]
	if r.Phase != "session" || r.Code != 4012 || r.Reason != "banned" || r.By != "server" || r.ConnID == "" || r.Platform != "web" || !strings.HasSuffix(r.IP, "*") {
		t.Errorf("record = %+v", r)
	}
	e.waitMetric(`im_disconnects_total{by="server",code="4012",phase="session"}`, "1")
	e.waitMetric(`im_kicks_total{reason="banned"}`, "1")

	for range 3 {
		ws := e.dialURL("old", e.wsURL(old, "web"))
		ws.closeCode(waitTimeout)
	}
	waitFor(t, "handshake records", func() bool { return len(e.disconnects("u1").Records) == 4 })
	h := e.disconnects("u1").Records[0]
	if h.Phase != "handshake" || h.Code != 4012 || h.ConnID != "" || h.By != "server" {
		t.Errorf("handshake record = %+v", h)
	}
	e.waitMetric(`im_disconnects_total{by="server",code="4012",phase="handshake"}`, "3")

	// 过期 token（测试模式 1 秒）。
	e.users("u5")
	short := e.tokenTTL("u5", 1)
	time.Sleep(1100 * time.Millisecond)
	ws := e.dialURL("short", e.wsURL(short, "web"))
	ws.closeCode(waitTimeout)
	waitFor(t, "expired record", func() bool {
		rs := e.disconnects("u5").Records
		return len(rs) == 1 && rs[0].Code == 4011 && rs[0].Phase == "handshake"
	})
	e.waitMetric(`im_disconnects_total{by="server",code="4011",phase="handshake"}`, "1")
	if strings.Contains(e.metric(`im_disconnects_total{by="server",code="4011",phase="session"}`), "1") {
		t.Error("4011 counted in session phase")
	}
}

// AC-OPS-09-2、06-4：第 11 个连接挤掉最早的：4429 / tooManyConnections，连接 ID 是最早的那个。
func TestOPS09TooManyConnections(t *testing.T) {
	e := newOpsEnv(t, nil)
	e.users("u1")
	var conns []*client
	for range 10 {
		conns = append(conns, e.dial("u1", "web"))
	}
	e.waitConns("u1", 10)
	var first []map[string]any
	var res struct {
		Connections []map[string]any `json:"connections"`
	}
	e.mustJSON("POST", "/internal/v1/connections:query", map[string]any{"userId": "u1"}, &res)
	first = res.Connections
	e.dial("u1", "web")
	waitFor(t, "eviction record", func() bool { return len(e.disconnects("u1").Records) == 1 })
	r := e.disconnects("u1").Records[0]
	if r.Code != 4429 || r.Reason != "tooManyConnections" || r.ConnID != first[0]["connId"] {
		t.Errorf("record = %+v, oldest = %v", r, first[0]["connId"])
	}
	e.waitMetric(`im_kicks_total{reason="tooManyConnections"}`, "1")
	_ = conns
}

// AC-OPS-09-5、09-6：4401、4010 不写按用户的记录；4010 的日志不含 token 里自称的 userId。
func TestOPS09Unattributed(t *testing.T) {
	e := newOpsEnv(t, nil)
	e.users("victim")
	// 不发 auth：等超时（测试配置 150ms）。
	ws, _, err := websocket.Dial(context.Background(), e.wsURL("", "web"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	go ws.Read(context.Background()) // 要读才能完成关闭握手；不读的话服务端最多等 5 秒
	e.waitMetric(`im_disconnects_total{by="server",code="4401",phase="handshake"}`, "1")
	e.waitMetric(`im_handshakes_total{result="auth_required"}`, "1")
	if !strings.Contains(e.logs.String(), "code=4401") || !strings.Contains(e.logs.String(), "phase=handshake") {
		t.Errorf("no 4401 log line:\n%s", e.logs.String())
	}

	// 用错误密钥签名、声称是 victim 的 token。
	bad := forgedToken("victim")
	c := e.dialURL("forged", e.wsURL(bad, "web"))
	c.closeCode(waitTimeout)
	e.waitMetric(`im_disconnects_total{by="server",code="4010",phase="handshake"}`, "1")
	if got := e.disconnects("victim").Records; len(got) != 0 {
		t.Errorf("records for an unverified user: %+v", got)
	}
	for _, l := range strings.Split(e.logs.String(), "\n") {
		if strings.Contains(l, "code=4010") && strings.Contains(l, "victim") {
			t.Errorf("4010 log has the claimed userId: %s", l)
		}
	}

	// 平台不合法 4009。
	c = e.dialURL("badplatform", e.wsURL(e.token("victim"), "nonsense"))
	c.closeCode(waitTimeout)
	e.waitMetric(`im_disconnects_total{by="server",code="4009",phase="handshake"}`, "1")
}

// S-OPS-02、S-OPS-09：Origin 被拒计数；日志里的 IP 是脱敏的。依赖库的错误文案，升级时这里会发现。
func TestOPSOriginRejected(t *testing.T) {
	gw := gateway.DefaultConfig()
	gw.NodeID = "n1"
	gw.AllowedOrigins = []string{"https://im.example.com"}
	e := newOpsEnv(t, func(c *app.Config) { c.Gateway = gw })
	_, resp, err := websocket.Dial(context.Background(), e.wsURL("", "web"), &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example.org"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("resp = %v err = %v", resp, err)
	}
	e.waitMetric(`im_handshakes_total{result="origin_rejected"}`, "1")
	if strings.Contains(e.logs.String(), "ip=127.0.0.1") {
		t.Errorf("full IP in logs:\n%s", e.logs.String())
	}
}

// AC-OPS-09-7：连接并断开 60 次：只留最近 50 条，新的在前。
func TestOPS09Limit(t *testing.T) {
	e := newOpsEnv(t, func(c *app.Config) { c.Gateway.Guard.HandshakeRate, c.Gateway.Guard.HandshakeBurst = 1000, 1000 })
	e.users("u1")
	for range 60 {
		c := e.dial("u1", "web")
		c.ws.Close(websocket.StatusNormalClosure, "")
	}
	waitFor(t, "all 60 closed", func() bool {
		return e.metric(`im_disconnects_total{by="client",code="1000",phase="session"}`) == "60"
	})
	rs := e.disconnects("u1")
	if len(rs.Records) != 50 || rs.BufferFrom == 0 {
		t.Fatalf("records = %d from %d", len(rs.Records), rs.BufferFrom)
	}
	for i := 1; i < len(rs.Records); i++ {
		if rs.Records[i].TS > rs.Records[i-1].TS {
			t.Fatal("not newest first")
		}
	}
	code, b := e.call("POST", "/internal/v1/disconnects:query", map[string]any{"userId": "u1", "limit": 51}, opsReq{})
	expectCode(t, "limit", code, b, 400, 4000)
}

// AC-OPS-10-1：接口响应和日志都不含密钥及其哈希前缀（用真实的摘要函数）。
func TestOPS10NoSecrets(t *testing.T) {
	jwtMarker, adminMarker := "SECRET-MARKER-JWT-0123456789abcdef0123", "SECRET-MARKER-ADM-0123456789abcdef0123"
	var srvCfg config.Server
	e := newOpsEnv(t, func(c *app.Config) {
		pub := c.OpsKeys[0].Public
		loaded, _, err := config.Load(func(k string) string {
			switch k {
			case "IM_JWT_KEYS":
				return "k1:" + jwtMarker
			case "IM_ADMIN_KEY":
				return adminMarker
			case "IM_OPS_PUBKEYS":
				return "k1:" + base64.StdEncoding.EncodeToString(pub)
			}
			return ""
		})
		if err != nil {
			t.Fatal(err)
		}
		srvCfg = loaded
		c.JWTKeys, c.JWTActiveKID, c.AdminKey = loaded.App.JWTKeys, loaded.App.JWTActiveKID, loaded.App.AdminKey
		c.JWTSecret = ""
		c.ConfigSummary = loaded.App.ConfigSummary
	})
	_ = srvCfg
	code, body := e.call("GET", "/internal/v1/config", nil, opsReq{})
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	var sum config.ConfigSummary
	_ = json.Unmarshal(body, &sum)
	if len(sum.Checks) == 0 || !slices.Equal(sum.Keys.JWTKids, []string{"k1"}) || sum.Env.Version != "v-test" || sum.Env.NodeID != "n1" {
		t.Errorf("summary = %s", body)
	}
	for _, secret := range []string{jwtMarker, adminMarker} {
		h := sha256.Sum256([]byte(secret))
		for _, leak := range []string{secret, hex.EncodeToString(h[:])[:16]} {
			if strings.Contains(string(body), leak) || strings.Contains(e.logs.String(), leak) {
				t.Errorf("leaked %q", leak)
			}
		}
	}
}

// AC-OPS-11-1、11-2（迁移状态）、11-3（故障注入）、11-5（校验和不一致）。
func TestOPS11DB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "im.db")
	e := newOpsEnv(t, func(c *app.Config) { c.DBPath = dbPath })
	var db struct {
		Type       string               `json:"type"`
		Location   string               `json:"location"`
		Connected  bool                 `json:"connected"`
		Version    int                  `json:"version"`
		Migrations []store.MigrationRow `json:"migrations"`
		Pending    []store.MigrationRow `json:"pending"`
	}
	e.mustJSON("GET", "/internal/v1/db", nil, &db)
	ms, _ := store.SQLiteMigrations()
	if db.Type != "sqlite" || db.Location != dbPath || !db.Connected || len(db.Migrations) != len(ms) || len(db.Pending) != 0 || db.Version != ms[len(ms)-1].Version {
		t.Errorf("db = %+v", db)
	}
	for _, m := range db.Migrations {
		if !m.ChecksumOK || m.AppliedAt == 0 {
			t.Errorf("migration = %+v", m)
		}
	}
	// 和 CLI 同一个函数（AC-OPS-11-1）。
	rows, err := store.SQLiteMigrationReport(context.Background(), dbPath)
	if err != nil || len(rows) != len(db.Migrations) || rows[0] != db.Migrations[0] {
		t.Errorf("report = %+v err = %v", rows, err)
	}

	// 故障注入：/readyz 503，节点 ready=false、draining=false，db 不可用。
	e.adminPost("/admin/test/faults", map[string]any{"storePing": "fail"}, adminKey)
	code, _ := e.do(e.srv.URL, "GET", "/readyz", nil, "")
	if code != 503 {
		t.Errorf("readyz = %d", code)
	}
	var n opsapi.NodeInfo
	e.mustJSON("GET", "/internal/v1/node", nil, &n)
	if n.Ready || n.Draining {
		t.Errorf("node = ready %v draining %v", n.Ready, n.Draining)
	}
	e.mustJSON("GET", "/internal/v1/db", nil, &db)
	if db.Connected {
		t.Error("db should be disconnected")
	}
	e.adminPost("/admin/test/faults", map[string]any{"storePing": ""}, adminKey)
}

// AC-OPS-05-4、13-3：运维端口只有 GET 和两个只读 POST；任何端口都没有备份、恢复路径。
func TestOPSRouteGuard(t *testing.T) {
	e := newOpsEnv(t, nil)
	ops := e.app.OpsRoutes()
	if len(ops) != 5 {
		t.Errorf("ops routes = %v", ops)
	}
	readOnlyPost := []string{"POST /internal/v1/connections:query", "POST /internal/v1/disconnects:query"}
	all := slices.Clone(ops)
	for _, r := range ops {
		if !strings.HasPrefix(r, "GET ") && !slices.Contains(readOnlyPost, r) {
			t.Errorf("ops route changes state: %s", r)
		}
	}
	for _, r := range e.app.AdminRoutes() {
		all = append(all, r.Method+" "+r.Pattern)
	}
	for _, r := range all {
		low := strings.ToLower(r)
		if strings.Contains(low, "backup") || strings.Contains(low, "restore") {
			t.Errorf("backup path exposed: %s", r)
		}
	}
}

// AC-OPS-17-7（后半）：生产环境 pprof 不是回环地址时拒绝启动（进程级）。ops_test 的同类检查放在一起跑。
func TestOPSCLIMigrateStatusAndManifest(t *testing.T) {
	bin := buildServer(t)
	dir := t.TempDir()
	db := filepath.Join(dir, "im.db")
	if out, err := runBin(t, bin, nil, "migrate", "-db", db, "up"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	out, err := runBin(t, bin, nil, "migrate", "-db", db, "status")
	if err != nil || !strings.Contains(out, "0001_init  applied") || !strings.Contains(out, "checksum ok") {
		t.Errorf("status: %v\n%s", err, out)
	}
	// 校验和改错：CLI 报"不一致"并非 0 退出（AC-OPS-11-5）。
	st, err := store.OpenSQLiteReadOnly(db)
	if err != nil {
		t.Fatal(err)
	}
	_, roErr := st.Exec(`UPDATE schema_migrations SET checksum = 'x' WHERE version = 1`)
	st.Close()
	if roErr == nil || !strings.Contains(strings.ToLower(roErr.Error()), "readonly") {
		t.Errorf("read-only connection accepted a write: %v", roErr) // AC-OPS-11-4
	}
	rw, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`UPDATE schema_migrations SET checksum = 'x' WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	rows, rerr := store.SQLiteMigrationReport(context.Background(), db)
	if rerr != nil || rows[0].ChecksumOK {
		t.Errorf("rows[0] = %+v err = %v", rows[0], rerr)
	}
	if _, err := rw.Exec(`UPDATE schema_migrations SET checksum = (SELECT checksum FROM schema_migrations WHERE version = 2) WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	rw.Close()
	if out, err := runBin(t, bin, nil, "migrate", "-db", db, "status"); err == nil || !strings.Contains(out, "checksum mismatch") {
		t.Errorf("mismatch status: %v\n%s", err, out)
	}

	// 备份清单（AC-OPS-12-1 的 im-server 半边）。
	db2 := filepath.Join(dir, "two.db")
	if out, err := runBin(t, bin, nil, "migrate", "-db", db2, "up"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	backup, manifest := filepath.Join(dir, "backup.db"), filepath.Join(dir, "manifest.json")
	if out, err := runBin(t, bin, nil, "backup", "-db", db2, "-o", backup, "-manifest", manifest, "-trigger", "scheduled"); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	data, _ := os.ReadFile(backup)
	sum := sha256.Sum256(data)
	if m["source"] != "im" || m["trigger"] != "scheduled" || m["file"] != "backup.db" || m["sha256"] != hex.EncodeToString(sum[:]) || m["size"] != float64(len(data)) || m["verified"] != true || m["result"] != "ok" || m["migrationVersion"] == float64(0) {
		t.Errorf("manifest = %s", raw)
	}
	if strings.Contains(string(raw), dir) {
		t.Errorf("manifest contains the directory: %s", raw)
	}
	info, _ := os.Stat(manifest)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("manifest mode = %v", info.Mode())
	}
}

// forgedToken 用错误的密钥签一个声称是 userID 的 token。
func forgedToken(userID string) string {
	signer, err := auth.NewSigner("forged-secret-0123456789abcdef-0123", clock.New())
	if err != nil {
		panic(err)
	}
	tok, _, err := signer.Issue(userID, 0, time.Hour)
	if err != nil {
		panic(err)
	}
	return tok
}
