package e2e

// X-5 管理接口收紧（PRD-admin F-KEY-02、F-KEY-03、F-KEY-05）：调用方 + scope、请求签名与防重放、旧密钥过渡。

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/admin"
	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// testCaller 是测试里的签名调用方：服务端配置 + 客户端持有的密钥。
type testCaller struct {
	cfg    adminauth.Caller
	secret []byte             // HMAC
	priv   ed25519.PrivateKey // Ed25519
}

func hmacCaller(id string, scopes ...adminauth.Scope) testCaller {
	secret := make([]byte, 32)
	rand.Read(secret)
	return testCaller{cfg: adminauth.Caller{ID: id, Alg: adminauth.AlgHMAC, Secret: secret, Scopes: scopeSet(scopes)}, secret: secret}
}

func edCaller(id string, scopes ...adminauth.Scope) testCaller {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return testCaller{cfg: adminauth.Caller{ID: id, Alg: adminauth.AlgEd25519, PublicKey: pub, Scopes: scopeSet(scopes)}, priv: priv}
}

func scopeSet(scopes []adminauth.Scope) map[adminauth.Scope]bool {
	m := map[adminauth.Scope]bool{}
	for _, s := range scopes {
		m[s] = true
	}
	return m
}

// nonBodyScopes 是除了正文类以外的全部 scope（正文类只能单独配置在 bodyAccess 调用方上）。
func nonBodyScopes(except adminauth.Scope) []adminauth.Scope {
	var out []adminauth.Scope
	for _, s := range adminauth.AllScopes {
		if s != except && !adminauth.IsBodyScope(s) {
			out = append(out, s)
		}
	}
	return out
}

// console 按 PRD-admin F-KEY-02 规则 4 配置（第一批）。
func consoleCaller() testCaller {
	return edCaller("console", adminauth.ScopeUserRead, adminauth.ScopeUserWrite, adminauth.ScopeUserKick, adminauth.ScopeUserSessions,
		adminauth.ScopeGroupRead, adminauth.ScopeGroupWrite, adminauth.ScopeGroupDissolve, adminauth.ScopeSocialSettings,
		adminauth.ScopeSocialFriends, adminauth.ScopeMsgMeta, adminauth.ScopeAuditRead)
}

func newEnvWithCallers(t *testing.T, callers ...testCaller) *env {
	return newEnvCfg(t, func(c *app.Config) {
		for _, tc := range callers {
			c.AdminCallers = append(c.AdminCallers, tc.cfg)
		}
	})
}

// signOpts 调整签名请求，用来构造各种失败情况。
type signOpts struct {
	operator, requestID, node string
	ts                        time.Time // 零值表示现在
	nonce                     string    // 空表示随机
	sendBody                  []byte    // 非 nil 时发送这个 body（签名仍按原 body 计算）
	sendOperator              *string   // 非 nil 时发送这个 Operator（签名仍按原值）
	sendQuery                 *string
	badSignature              bool
	dropSignature             bool
}

// signedRequest 构造一个签名请求；path 可以带查询串。
func (e *env) signedRequest(c testCaller, method, path string, body any, o signOpts) *http.Request {
	e.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	p, q, _ := strings.Cut(path, "?")
	ts := o.ts
	if ts.IsZero() {
		ts = time.Now()
	}
	nonce := o.nonce
	if nonce == "" {
		b := make([]byte, 18)
		rand.Read(b)
		nonce = base64.RawURLEncoding.EncodeToString(b)
	}
	tsStr := strconv.FormatInt(ts.UnixMilli(), 10)
	sts := adminauth.StringToSign(adminauth.Request{Method: method, Path: p, RawQuery: q, Timestamp: tsStr, Nonce: nonce,
		Body: raw, Operator: o.operator, RequestID: o.requestID, TargetNode: o.node})
	var sig []byte
	if c.priv != nil {
		sig = ed25519.Sign(c.priv, []byte(sts))
	} else {
		sig = adminauth.SignHMAC(c.secret, sts)
	}
	if o.badSignature {
		sig[0] ^= 0xff
	}
	send := raw
	if o.sendBody != nil {
		send = o.sendBody
	}
	if o.sendQuery != nil {
		q = *o.sendQuery
	}
	url := e.adminSrv.URL + p
	if q != "" {
		url += "?" + q
	}
	req, _ := http.NewRequest(method, url, bytes.NewReader(send))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(admin.HeaderCaller, c.cfg.ID)
	req.Header.Set(admin.HeaderTimestamp, tsStr)
	req.Header.Set(admin.HeaderNonce, nonce)
	if !o.dropSignature {
		req.Header.Set(admin.HeaderSignature, adminauth.EncodeSignature(sig))
	}
	op := o.operator
	if o.sendOperator != nil {
		op = *o.sendOperator
	}
	for k, v := range map[string]string{admin.HeaderOperator: op, admin.HeaderRequestID: o.requestID, admin.HeaderTargetNode: o.node} {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	return req
}

func (e *env) send(req *http.Request) (int, []byte) {
	e.t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (e *env) signed(c testCaller, method, path string, body any, o signOpts) (int, []byte) {
	e.t.Helper()
	return e.send(e.signedRequest(c, method, path, body, o))
}

func expectCode(t *testing.T, label string, status int, body []byte, wantStatus, wantCode int) {
	t.Helper()
	if status != wantStatus {
		t.Errorf("%s: status %d, want %d (%s)", label, status, wantStatus, body)
		return
	}
	if wantCode != 0 && !bytes.Contains(body, []byte(fmt.Sprintf(`"code":%d`, wantCode))) {
		t.Errorf("%s: body %s, want code %d", label, body, wantCode)
	}
}

// AC-KEY-02-1：scope 不足返回 403 / 4030，业务不执行。
func TestCallerScopeDenied(t *testing.T) {
	biz := hmacCaller("biz1", adminauth.ScopeTokenIssue)
	e := newEnvWithCallers(t, biz)
	status, body := e.signed(biz, http.MethodPost, "/admin/groups", map[string]any{"groupId": "g1", "name": "g1"}, signOpts{})
	expectCode(t, "create group without group.write", status, body, http.StatusForbidden, protocol.CodeForbidden)
	if code, _ := e.adminDo(http.MethodGet, "/admin/groups/g1", nil, adminKey); code != http.StatusNotFound {
		t.Errorf("group was created: %d", code)
	}
	// 有 scope 的接口正常：biz1 能签发 token。
	status, body = e.signed(biz, http.MethodPost, "/admin/users/token", map[string]any{"userId": "alice"}, signOpts{})
	expectCode(t, "issue token", status, body, http.StatusOK, 0)
}

// adminSample 是一个管理路由的合法请求样本。新增路由必须在这里加样本，否则 TestEveryAdminRouteChecksScope 失败。
type adminSample struct {
	setup  func(e *env)
	method string
	path   string
	body   any
	want   int // 有 scope 时的成功状态码
}

var adminSamples = map[string]adminSample{
	"POST /admin/users/token":                 {nil, "POST", "/admin/users/token", map[string]any{"userId": "alice"}, 200},
	"POST /admin/users/import":                {nil, "POST", "/admin/users/import", map[string]any{"users": []any{map[string]any{"userId": "alice"}}}, 200},
	"GET /admin/users/{userId}":               {func(e *env) { e.users("alice") }, "GET", "/admin/users/alice", nil, 200},
	"POST /admin/users/{userId}/force-logout": {func(e *env) { e.users("alice") }, "POST", "/admin/users/alice/force-logout", map[string]any{}, 200},
	"POST /admin/groups":                      {nil, "POST", "/admin/groups", map[string]any{"groupId": "g1", "name": "g1"}, 201},
	"GET /admin/groups/{groupId}":             {func(e *env) { e.createGroup("g1") }, "GET", "/admin/groups/g1", nil, 200},
	"PATCH /admin/groups/{groupId}":           {func(e *env) { e.createGroup("g1") }, "PATCH", "/admin/groups/g1", map[string]any{"name": "g2"}, 200},
	"POST /admin/groups/{groupId}/members":    {func(e *env) { e.createGroup("g1"); e.users("bob") }, "POST", "/admin/groups/g1/members", map[string]any{"members": []string{"bob"}}, 200},
	"DELETE /admin/groups/{groupId}/members":  {func(e *env) { e.createGroup("g1", "bob") }, "DELETE", "/admin/groups/g1/members", map[string]any{"members": []string{"bob"}}, 200},
	"POST /admin/test/drop-ack":               {nil, "POST", "/admin/test/drop-ack", map[string]any{"clientMsgId": "cid-00000001"}, 200},
	"POST /admin/test/skip-delivery":          {nil, "POST", "/admin/test/skip-delivery", map[string]any{"clientMsgId": "cid-00000001"}, 200},
	"POST /admin/test/clock":                  {nil, "POST", "/admin/test/clock", map[string]any{"nowMs": nil}, 200},
	"POST /admin/test/faults":                 {nil, "POST", "/admin/test/faults", map[string]any{"adminAuditWrite": ""}, 200},
	"GET /admin/audit":                        {nil, "GET", "/admin/audit?from=0&to=1000", nil, 200},
	"GET /admin/users":                        {func(e *env) { e.users("alice") }, "GET", "/admin/users?q=al", nil, 200},
	"POST /admin/users/sessions/query":        {func(e *env) { e.users("alice") }, "POST", "/admin/users/sessions/query", map[string]any{"userId": "alice"}, 200},
	"GET /admin/groups":                       {func(e *env) { e.createGroup("g1") }, "GET", "/admin/groups?q=g", nil, 200},
	"GET /admin/groups/{groupId}/members":     {func(e *env) { e.createGroup("g1") }, "GET", "/admin/groups/g1/members", nil, 200},
	"POST /admin/convs/messages/query":        {nil, "POST", "/admin/convs/messages/query", map[string]any{"convId": "group:g1"}, 200},
	"DELETE /admin/groups/{groupId}":          {func(e *env) { e.createGroup("g1") }, "DELETE", "/admin/groups/g1", nil, 200},
	// PRD-social M3（BE-114、BE-115、5.6 节测试设施）
	"GET /admin/settings/social":          {nil, "GET", "/admin/settings/social", nil, 200},
	"PATCH /admin/settings/social":        {nil, "PATCH", "/admin/settings/social", map[string]any{"mode": "open"}, 200},
	"POST /admin/friends/import":          {func(e *env) { e.users("alice", "bob") }, "POST", "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}}, 200},
	"DELETE /admin/friends":               {nil, "DELETE", "/admin/friends", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}}, 200},
	"GET /admin/users/{userId}/friends":   {nil, "GET", "/admin/users/alice/friends", nil, 200},
	"PUT /admin/users/{userId}/official":  {func(e *env) { e.users("kefu") }, "PUT", "/admin/users/kefu/official", map[string]any{"official": true}, 200},
	"POST /admin/test/social/drop-events": {nil, "POST", "/admin/test/social/drop-events", map[string]any{"userId": "alice", "type": "socialChanged", "count": 1}, 200},
	"POST /admin/test/social/purge":       {nil, "POST", "/admin/test/social/purge", map[string]any{}, 200},
	"POST /admin/test/social/seed":        {nil, "POST", "/admin/test/social/seed", map[string]any{"friends": []any{map[string]any{"owner": "alice", "peer": "bob"}}}, 200},
	"POST /admin/test/invalidate":         {nil, "POST", "/admin/test/invalidate", map[string]any{"topic": "socialSettings", "key": ""}, 200},
	"POST /admin/test/limits/reset":       {nil, "POST", "/admin/test/limits/reset", map[string]any{"userId": "alice"}, 200},
}

// AC-KEY-02-2：遍历全部管理路由 × "有该 scope / 只有其他 scope"两种调用方。
func TestEveryAdminRouteChecksScope(t *testing.T) {
	routes := newEnv(t).app.AdminRoutes()
	if len(routes) == 0 {
		t.Fatal("no admin routes")
	}
	for _, r := range routes {
		key := r.Method + " " + r.Pattern
		s, ok := adminSamples[key]
		if !ok {
			t.Errorf("route %s (scope %s) has no sample in adminSamples", key, r.Scope)
			continue
		}
		with := hmacCaller("with", r.Scope)
		without := hmacCaller("without", nonBodyScopes(r.Scope)...)
		e := newEnvWithCallers(t, with, without)
		if s.setup != nil {
			s.setup(e)
		}
		status, body := e.signed(without, s.method, s.path, s.body, signOpts{})
		expectCode(t, key+" without "+string(r.Scope), status, body, http.StatusForbidden, protocol.CodeForbidden)
		status, body = e.signed(with, s.method, s.path, s.body, signOpts{})
		expectCode(t, key+" with "+string(r.Scope), status, body, s.want, 0)
	}
}

// AC-KEY-02-6：console 调用方永远没有 token.issue。AC-KEY-02-8：停用的调用方 401 / 4017。
func TestConsoleCannotIssueTokensAndDisabledCaller(t *testing.T) {
	console := consoleCaller()
	off := hmacCaller("biz1", adminauth.ScopeUserRead)
	off.cfg.Disabled = true
	e := newEnvWithCallers(t, console, off)
	status, body := e.signed(console, http.MethodPost, "/admin/users/token", map[string]any{"userId": "alice"}, signOpts{})
	expectCode(t, "console issues token", status, body, http.StatusForbidden, protocol.CodeForbidden)
	e.users("alice")
	status, body = e.signed(off, http.MethodGet, "/admin/users/alice", nil, signOpts{})
	expectCode(t, "disabled caller", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	status, body = e.signed(hmacCaller("ghost", adminauth.ScopeUserRead), http.MethodGet, "/admin/users/alice", nil, signOpts{})
	expectCode(t, "unknown caller", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
}

// AC-KEY-03-2：篡改 body / Operator / 查询参数值，签名都对不上。
func TestSignatureTampering(t *testing.T) {
	c := edCaller("console", adminauth.ScopeUserRead, adminauth.ScopeUserWrite)
	e := newEnvWithCallers(t, c)
	e.users("alice")
	body := map[string]any{"users": []any{map[string]any{"userId": "alice", "nickname": "A"}}}
	other := "admin:a2"
	q := "x=2"
	for name, o := range map[string]signOpts{
		"body":     {sendBody: []byte(`{"users":[{"userId":"alice","nickname":"B"}]}`)},
		"operator": {operator: "admin:a1", sendOperator: &other},
	} {
		status, resp := e.signed(c, http.MethodPost, "/admin/users/import", body, o)
		expectCode(t, "tampered "+name, status, resp, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	}
	status, resp := e.signed(c, http.MethodGet, "/admin/users/alice?x=1", nil, signOpts{sendQuery: &q})
	expectCode(t, "tampered query value", status, resp, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	// 只是参数顺序不同，签名仍然有效。
	reordered := "y=2&x=1"
	status, resp = e.signed(c, http.MethodGet, "/admin/users/alice?x=1&y=2", nil, signOpts{sendQuery: &reordered})
	expectCode(t, "reordered query", status, resp, http.StatusOK, 0)
	if _, b := e.getUser("alice"); b["nickname"] != nil && b["nickname"] != "" {
		t.Errorf("tampered import was applied: %v", b)
	}
}

// AC-KEY-03-3：时间戳窗口 ±300 秒，按 [IM 钟] 判断。
func TestSignatureWindow(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead)
	e := newEnvWithCallers(t, c)
	e.users("alice")
	now := time.UnixMilli(1762156800000)
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": now.UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	for _, d := range []time.Duration{-301 * time.Second, 301 * time.Second} {
		status, body := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{ts: now.Add(d)})
		expectCode(t, fmt.Sprintf("ts %v", d), status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	}
	status, body := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{ts: now.Add(-299 * time.Second)})
	expectCode(t, "ts -299s", status, body, http.StatusOK, 0)
}

// AC-KEY-03-4：原样重放成功请求，401 / 4017，业务只执行一次（强制下线只踢一次）。
func TestSignatureReplay(t *testing.T) {
	c := edCaller("console", adminauth.ScopeUserKick, adminauth.ScopeAuditRead)
	e := newEnvWithCallers(t, c)
	first := e.dial("alice", "web")
	req := e.signedRequest(c, http.MethodPost, "/admin/users/alice/force-logout", map[string]any{}, signOpts{requestID: "r-1"})
	raw, _ := io.ReadAll(req.Body)
	replay := req.Clone(req.Context())
	req.Body, replay.Body = io.NopCloser(bytes.NewReader(raw)), io.NopCloser(bytes.NewReader(raw))
	status, body := e.send(req)
	expectCode(t, "first", status, body, http.StatusOK, 0)
	first.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)

	second := e.dial("alice", "web") // 重新签发 token 后上线
	status, body = e.send(replay)
	expectCode(t, "replay", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	second.expectNoEvent(protocol.EventKicked, 300*time.Millisecond)
	// 审计里这个请求只有一组 intent / result（重放在签名阶段就被拒绝，不写审计）。
	if got := e.auditEntries(c, "requestId=r-1"); len(got) != 2 {
		t.Errorf("audit entries for r-1 = %+v, want intent + result", got)
	}
}

// AC-KEY-03-5：签名错误的请求不登记 nonce，之后同一 nonce 的正确请求成功。
func TestBadSignatureDoesNotBurnNonce(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead)
	e := newEnvWithCallers(t, c)
	e.users("alice")
	const nonce = "same-nonce-0123456789"
	for i := 0; i < 1000; i++ {
		if status, _ := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{nonce: nonce, badSignature: true}); status != http.StatusUnauthorized {
			t.Fatalf("bad signature #%d: %d", i, status)
		}
	}
	status, body := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{nonce: nonce})
	expectCode(t, "good signature after 1000 bad ones", status, body, http.StatusOK, 0)
	status, body = e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{nonce: nonce})
	expectCode(t, "nonce reuse", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	if v := e.metric(`im_admin_auth_failures_total{reason="signature"}`); v != "1000" {
		t.Errorf("signature failures = %q, want 1000", v)
	}
	if v := e.metric(`im_admin_auth_failures_total{reason="nonce"}`); v != "1" {
		t.Errorf("nonce failures = %q, want 1", v)
	}
}

// AC-KEY-03-6：缺签名头 401 / 4017，指标 reason="missing" 加 1。目标节点不符同样 401（reason="node"）。
func TestMissingSignatureAndTargetNode(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead)
	e := newEnvWithCallers(t, c)
	e.users("alice")
	status, body := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{dropSignature: true})
	expectCode(t, "missing signature", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	if v := e.metric(`im_admin_auth_failures_total{reason="missing"}`); v != "1" {
		t.Errorf("missing failures = %q, want 1", v)
	}
	status, body = e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{node: "some-other-node"})
	expectCode(t, "wrong target node", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
	if v := e.metric(`im_admin_auth_failures_total{reason="node"}`); v != "1" {
		t.Errorf("node failures = %q, want 1", v)
	}
}

// 调用方的 allowedCIDRs：来源不在范围内 403 / 4030（测试客户端来自 127.0.0.1）。
func TestCallerAllowedCIDRs(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead)
	c.cfg.AllowedCIDRs = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	e := newEnvWithCallers(t, c)
	e.users("alice")
	status, body := e.signed(c, http.MethodGet, "/admin/users/alice", nil, signOpts{})
	expectCode(t, "caller outside allowedCIDRs", status, body, http.StatusForbidden, protocol.CodeForbidden)
}

// AC-KEY-05-1 由现有 e2e（全部用旧密钥）覆盖。F-KEY-05 规则 5：两种头同时出现时以签名为准。
func TestSignedHeadersWinOverLegacyKey(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead)
	e := newEnvWithCallers(t, c)
	e.users("alice")
	req := e.signedRequest(c, http.MethodGet, "/admin/users/alice", nil, signOpts{badSignature: true})
	req.Header.Set("X-Admin-Key", adminKey)
	status, body := e.send(req)
	expectCode(t, "bad signature + valid legacy key", status, body, http.StatusUnauthorized, protocol.CodeAdminSignatureInvalid)
}

// AC-KEY-05-3：生产环境未开启 IM_ADMIN_LEGACY 时不接受旧密钥（401 / 4013）；开启时 /readyz 带告警。
func TestLegacyKeyDisabled(t *testing.T) {
	c := hmacCaller("biz1", adminauth.ScopeUserRead, adminauth.ScopeUserWrite)
	e := newEnvCfg(t, func(cfg *app.Config) {
		cfg.AdminLegacyDisabled = true
		cfg.AdminCallers = []adminauth.Caller{c.cfg}
	})
	e.signed(c, http.MethodPost, "/admin/users/import", map[string]any{"users": []any{map[string]any{"userId": "alice"}}}, signOpts{})
	status, body := e.adminDo(http.MethodGet, "/admin/users/alice", nil, adminKey)
	expectCode(t, "legacy key disabled", status, body, http.StatusUnauthorized, protocol.CodeAdminKeyInvalid)

	warn := newEnvCfg(t, func(cfg *app.Config) { cfg.ReportLegacyAdmin = true })
	resp, err := http.Get(warn.srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	if r["legacyAdminKey"] != true || r["status"] != "ready" {
		t.Errorf("readyz = %v", r)
	}
}
