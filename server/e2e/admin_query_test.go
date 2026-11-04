package e2e

// 管理后台第一批查询接口（PRD-admin BE-205～BE-210）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
)

type admEnv struct {
	*env
	c testCaller
}

func newAdmEnv(t *testing.T, mutate func(*app.Config)) *admEnv {
	c := hmacCaller("console", adminauth.ScopeUserRead, adminauth.ScopeUserSessions, adminauth.ScopeGroupRead,
		adminauth.ScopeMsgMeta, adminauth.ScopeAuditRead, adminauth.ScopeGroupDissolve, adminauth.ScopeSocialSettings)
	e := newEnvCfg(t, func(cfg *app.Config) {
		cfg.AdminCallers = append(cfg.AdminCallers, c.cfg)
		if mutate != nil {
			mutate(cfg)
		}
	})
	return &admEnv{env: e, c: c}
}

func (e *admEnv) get(path string, out any) {
	e.t.Helper()
	code, body := e.signed(e.c, "GET", path, nil, signOpts{})
	if code != http.StatusOK {
		e.t.Fatalf("GET %s: %d %s", path, code, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		e.t.Fatal(err)
	}
}

func (e *admEnv) post(path string, in, out any) {
	e.t.Helper()
	code, body := e.signed(e.c, "POST", path, in, signOpts{})
	if code != http.StatusOK {
		e.t.Fatalf("POST %s: %d %s", path, code, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		e.t.Fatal(err)
	}
}

type userPage struct {
	Users []struct {
		UserID   string `json:"userId"`
		Nickname string `json:"nickname"`
		Online   bool   `json:"online"`
		Official bool   `json:"official"`
	} `json:"users"`
	NextCursor string `json:"nextCursor"`
}

func (p userPage) ids() []string {
	out := []string{}
	for _, u := range p.Users {
		out = append(out, u.UserID)
	}
	return out
}

// TC-ADM-10-01/02/03（AC-ADM-10-1、10-2、10-3）
func TestTCADM10Search(t *testing.T) {
	e := newAdmEnv(t, nil)
	e.users("u100", "u101", "u2", "U100", "u_1", "uX1")
	e.mustOK(e.importUsers(
		map[string]any{"userId": "n1", "nickname": "小明"}, map[string]any{"userId": "n2", "nickname": "小明明"},
		map[string]any{"userId": "n3", "nickname": "Tom"}, map[string]any{"userId": "n4", "nickname": "tommy"},
	))
	for q, want := range map[string][]string{
		"u10":                {"u100", "u101"},
		"u_":                 {"u_1"},
		"%E5%B0%8F%E6%98%8E": {"n1", "n2"}, // 小明
		"%E6%98%8E":          {},           // 明：前缀匹配，不返回
		"tom":                {"n3", "n4"},
	} {
		var p userPage
		e.get("/admin/users?q="+q, &p)
		if !slices.Equal(p.ids(), want) {
			t.Errorf("q=%s: %v, want %v", q, p.ids(), want)
		}
	}
	for _, path := range []string{"/admin/users", "/admin/users?q=%20%20", "/admin/users?q=a&limit=30", "/admin/users?q=a&cursor=%21%21", "/admin/users?q=" + strings.Repeat("a", 65)} {
		code, body := e.signed(e.c, "GET", path, nil, signOpts{})
		expectCode(t, path, code, body, 400, 4000)
	}
}

// TC-ADM-10-06（AC-ADM-10-6）只看官方账号。
func TestTCADM10Official(t *testing.T) {
	e := newAdmEnv(t, nil)
	e.users("kefu", "a1", "a2")
	if code, body := e.signed(e.c, "PUT", "/admin/users/kefu/official", map[string]any{"official": true}, signOpts{}); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	var p userPage
	e.get("/admin/users?official=true", &p)
	if !slices.Equal(p.ids(), []string{"kefu"}) || !p.Users[0].Official {
		t.Errorf("%+v", p)
	}
}

// TC-ADM-10-07（AC-ADM-10-7）1000 个用户，limit=100 翻 10 页，不重不漏。
func TestTCADM10Paging(t *testing.T) {
	e := newAdmEnv(t, nil)
	var all []string
	for batch := range 2 {
		var us []map[string]any
		for i := range 500 {
			id := fmt.Sprintf("p%04d", batch*500+i)
			us = append(us, map[string]any{"userId": id})
			all = append(all, id)
		}
		e.mustOK(e.importUsers(us...))
	}
	var seen []string
	cursor, pages := "", 0
	for {
		var p userPage
		path := "/admin/users?q=p&limit=100"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		e.get(path, &p)
		seen = append(seen, p.ids()...)
		pages++
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	if pages != 10 || !slices.Equal(seen, all) {
		t.Errorf("pages=%d seen=%d", pages, len(seen))
	}
}

// TC-ADM-14-01（AC-ADM-14-1、AC-ADM-10-4）：两个连接；经可信代理的来源 IP 脱敏；写审计；全部断开后离线。
func TestTCADM14Sessions(t *testing.T) {
	gw := gateway.DefaultConfig()
	gw.Guard.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	gw.NodeID = "node-a"
	e := newAdmEnv(t, func(c *app.Config) { c.Gateway = gw })
	e.users("u1")
	dial := func(platform string) *client {
		u, _ := url.Parse(e.wsURL("", platform))
		q := u.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		c := e.dialRaw(platform, u.String(), http.Header{"X-Forwarded-For": {"10.1.2.3"}})
		if f := c.call("auth", map[string]any{"token": e.token("u1")}); f.Error != nil {
			t.Fatalf("auth: %+v", f.Error)
		}
		return c
	}
	web, pc := dial("web"), dial("pc")

	// auth 的 ack 先发、再登记到在线表，拨号返回后要等一下。
	var u struct {
		Online bool `json:"online"`
	}
	deadlineUp := time.Now().Add(3 * time.Second)
	for {
		var q struct {
			Sessions []any `json:"sessions"`
		}
		e.post("/admin/users/sessions/query", map[string]any{"userId": "u1"}, &q)
		if len(q.Sessions) == 2 {
			break
		}
		if time.Now().After(deadlineUp) {
			t.Fatalf("sessions = %d", len(q.Sessions))
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.get("/admin/users/u1", &u)
	if !u.Online {
		t.Error("user should be online")
	}
	var res struct {
		Sessions []struct {
			ConnID, Platform, Node, IP string
			ConnectedAt                int64
			QueueLen                   int
		} `json:"sessions"`
		Partial bool   `json:"partial"`
		Scope   string `json:"scope"`
	}
	code, body := e.signed(e.c, "POST", "/admin/users/sessions/query", map[string]any{"userId": "u1"}, signOpts{operator: "admin:cs1", requestID: "req-sessions"})
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	if strings.Contains(string(body), "10.1.2.3") {
		t.Errorf("full IP leaked: %s", body)
	}
	_ = json.Unmarshal(body, &res)
	platforms := []string{}
	for _, s := range res.Sessions {
		platforms = append(platforms, s.Platform)
		if s.IP != "10.1.*.*" || s.Node != "node-a" || s.ConnID == "" || s.ConnectedAt == 0 {
			t.Errorf("session = %+v", s)
		}
	}
	slices.Sort(platforms)
	if !slices.Equal(platforms, []string{"pc", "web"}) || !res.Partial || res.Scope != "localNode" {
		t.Errorf("res = %+v", res)
	}
	got := e.auditEntries(e.c, "requestId=req-sessions")
	if len(got) != 2 {
		t.Fatalf("audit = %+v", got)
	}
	var summary struct {
		Count *int `json:"count"`
	}
	result := got[0] // 倒序：result 在前
	if err := json.Unmarshal(result.Summary, &summary); err != nil || summary.Count == nil || *summary.Count != 2 {
		t.Errorf("result summary = %s", result.Summary)
	}
	if result.Operator != "admin:cs1" || !slices.Contains(result.Targets.IDs, "u1") {
		t.Errorf("audit entry = %+v", result)
	}

	web.ws.CloseNow()
	pc.ws.CloseNow()
	deadline := time.Now().Add(3 * time.Second)
	for {
		e.get("/admin/users/u1", &u)
		if !u.Online {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("still online after both connections closed")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// 不存在的用户 4042；userId 不合法 4004。
	code, body = e.signed(e.c, "POST", "/admin/users/sessions/query", map[string]any{"userId": "nobody"}, signOpts{})
	expectCode(t, "missing", code, body, 404, 4042)
	code, body = e.signed(e.c, "POST", "/admin/users/sessions/query", map[string]any{"userId": "a b"}, signOpts{})
	expectCode(t, "invalid", code, body, 400, 4004)
}

type groupPage struct {
	Groups []struct {
		GroupID     string  `json:"groupId"`
		Type        string  `json:"type"`
		Owner       *string `json:"owner"`
		MemberCount int     `json:"memberCount"`
		DissolvedAt int64   `json:"dissolvedAt"`
	} `json:"groups"`
	NextCursor string `json:"nextCursor"`
}

func (p groupPage) ids() []string {
	out := []string{}
	for _, g := range p.Groups {
		out = append(out, g.GroupID)
	}
	return out
}

// TC-ADM-20-01～04（AC-ADM-20-1～4）
func TestTCADM20Groups(t *testing.T) {
	e := newAdmEnv(t, nil)
	e.users("alice", "bob", "carol")
	e.createGroup("g-match-1", "alice")
	e.createGroup("g-match-2", "alice", "bob")
	e.createGroup("g-other", "alice")
	var p groupPage
	e.get("/admin/groups?q=g-match", &p)
	got := p.ids()
	slices.Sort(got)
	if !slices.Equal(got, []string{"g-match-1", "g-match-2"}) {
		t.Errorf("prefix = %v", p.ids())
	}

	// 成员数和详情一致（AC-ADM-20-2）。
	var members []string
	for i := range 37 {
		members = append(members, fmt.Sprintf("m%02d", i))
	}
	e.users(members...)
	e.createGroup("g37", members...)
	p = groupPage{}
	e.get("/admin/groups?q=g37", &p)
	_, detail := e.getGroup("g37")
	if len(p.Groups) != 1 || p.Groups[0].MemberCount != 37 || len(detail.Members) != 37 {
		t.Errorf("list = %+v detail members = %d", p, len(detail.Members))
	}
	if p.Groups[0].Type != "managed" || p.Groups[0].Owner != nil {
		t.Errorf("managed group = %+v", p.Groups[0])
	}

	// 解散（AC-ADM-20-3）。
	e.createGroup("g9", "alice")
	if code, body := e.signed(e.c, "DELETE", "/admin/groups/g9", nil, signOpts{}); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	p = groupPage{}
	e.get("/admin/groups?q=g9", &p)
	if len(p.Groups) != 0 {
		t.Errorf("dissolved shown by default: %v", p.ids())
	}
	p = groupPage{}
	e.get("/admin/groups?q=g9&includeDissolved=true", &p)
	if len(p.Groups) != 1 || p.Groups[0].DissolvedAt == 0 {
		t.Errorf("includeDissolved = %+v", p)
	}

	// 自建群（AC-ADM-20-4）。
	a := e.dial("alice", "web")
	e.adminPost("/admin/test/social/seed", map[string]any{"friends": []map[string]string{
		{"owner": "alice", "peer": "bob"}, {"owner": "bob", "peer": "alice"}, {"owner": "alice", "peer": "carol"}, {"owner": "carol", "peer": "alice"},
	}, "silent": true}, adminKey)
	ack := a.call("groupCreate", map[string]any{"clientReqId": "req-20-4-000000", "members": []string{"bob", "carol"}})
	if ack.Error != nil {
		t.Fatalf("groupCreate: %+v", ack.Error)
	}
	var created struct {
		GroupID string `json:"groupId"`
	}
	_ = json.Unmarshal(ack.Data, &created)
	p = groupPage{}
	e.get("/admin/groups?q=ug-", &p)
	if len(p.Groups) != 1 || p.Groups[0].GroupID != created.GroupID || p.Groups[0].Type != "user" || p.Groups[0].Owner == nil || *p.Groups[0].Owner != "alice" {
		t.Errorf("user group = %+v", p)
	}
}

type memberPage struct {
	Members []struct {
		UserID   string `json:"userId"`
		Role     string `json:"role"`
		JoinedAt int64  `json:"joinedAt"`
	} `json:"members"`
	NextCursor string `json:"nextCursor"`
}

// TC-ADM-22-01、22-06（AC-ADM-22-1、22-6）
func TestTCADM22Members(t *testing.T) {
	e := newAdmEnv(t, nil)
	members := []string{}
	for i := range 1500 {
		members = append(members, fmt.Sprintf("u%04d", i))
	}
	for i := 0; i < len(members); i += 500 {
		var batch []map[string]any
		for _, m := range members[i : i+500] {
			batch = append(batch, map[string]any{"userId": m})
		}
		e.mustOK(e.importUsers(batch...))
	}
	if code, body := e.createGroupWith(map[string]any{"groupId": "big", "name": "big", "members": members}); code != http.StatusCreated {
		t.Fatalf("create group: %d %s", code, body)
	}
	var seen []string
	cursor, pages := "", 0
	for {
		var p memberPage
		path := "/admin/groups/big/members?limit=50"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		e.get(path, &p)
		if len(p.Members) != 50 {
			t.Fatalf("page %d has %d members", pages, len(p.Members))
		}
		for _, m := range p.Members {
			seen = append(seen, m.UserID)
			if m.Role != "member" || m.JoinedAt == 0 {
				t.Fatalf("member = %+v", m)
			}
		}
		pages++
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	if pages != 30 || !slices.Equal(seen, members) {
		t.Errorf("pages=%d members=%d", pages, len(seen))
	}

	e.users("u10", "u11", "u2")
	e.createGroup("g1", "u10", "u11", "u2")
	var p memberPage
	e.get("/admin/groups/g1/members?prefix=u1", &p)
	if len(p.Members) != 2 || p.Members[0].UserID != "u10" || p.Members[1].UserID != "u11" {
		t.Errorf("prefix = %+v", p.Members)
	}

	code, body := e.signed(e.c, "GET", "/admin/groups/nope/members", nil, signOpts{})
	expectCode(t, "missing group", code, body, 404, 4043)
	if code, body := e.signed(e.c, "DELETE", "/admin/groups/g1", nil, signOpts{}); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	code, body = e.signed(e.c, "GET", "/admin/groups/g1/members", nil, signOpts{})
	expectCode(t, "dissolved group", code, body, 404, 4043)
}

// TC-ADM-24（AC-ADM-24-1、24-2、24-3、24-5）：管理侧解散的 scope、审计和读写路径。
func TestTCADM24Dissolve(t *testing.T) {
	e := newAdmEnv(t, nil)
	e.users("u1", "u2", "u3")
	e.createGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")

	// 旧密钥不能解散（AC-KEY-05-2）；没有 group.dissolve 的调用方不能解散。
	code, body := e.adminDo("DELETE", "/admin/groups/g1", nil, adminKey)
	expectCode(t, "legacy", code, body, 403, 4030)

	start := time.Now()
	if code, body := e.signed(e.c, "DELETE", "/admin/groups/g1", nil, signOpts{operator: "admin:op1", requestID: "req-dissolve"}); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	for _, c := range []*client{u1, u2} {
		f := c.waitCmd("event")
		var ev struct {
			Type, ConvID, Reason string
		}
		_ = json.Unmarshal(f.Data, &ev)
		if ev.Type != "convRemoved" || ev.ConvID != "group:g1" || ev.Reason != "dissolved" {
			t.Errorf("event = %s", f.Data)
		}
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("convRemoved took %v", d)
	}
	got := e.auditEntries(e.c, "requestId=req-dissolve")
	if len(got) != 2 || !slices.Contains(got[0].Targets.IDs, "g1") {
		t.Errorf("audit = %+v", got)
	}

	// 已解散：成员 send / sync / read → 4043；管理查询 404；同 ID 建群 409。
	if f := u1.sendText("group", "g1", "cid-after-dissolve-1", "x"); f.Error == nil || f.Error.Code != 4043 {
		t.Errorf("send = %+v", f.Error)
	}
	if f := u1.call("sync", map[string]any{"convId": "group:g1", "afterSeq": 0}); f.Error == nil || f.Error.Code != 4043 {
		t.Errorf("sync = %+v", f.Error)
	}
	if f := u1.call("read", map[string]any{"convId": "group:g1", "seq": 1}); f.Error == nil || f.Error.Code != 4043 {
		t.Errorf("read = %+v", f.Error)
	}
	code, body = e.signed(e.c, "GET", "/admin/groups/g1", nil, signOpts{})
	expectCode(t, "get", code, body, 404, 4043)
	code, body = e.createGroupWith(map[string]any{"groupId": "g1", "name": "again"})
	expectCode(t, "recreate", code, body, 409, 4090)

	// 解散时离线的成员上线后 convs 里没有这个群（AC-ADM-24-5）。
	u3 := e.dial("u3", "web")
	var convs struct {
		Convs []struct {
			ConvID string `json:"convId"`
		} `json:"convs"`
	}
	_ = json.Unmarshal(u3.call("convs", map[string]any{}).Data, &convs)
	for _, c := range convs.Convs {
		if c.ConvID == "group:g1" {
			t.Error("dissolved group listed for offline member")
		}
	}
}

type metaPage struct {
	Messages []map[string]any `json:"messages"`
	HasMore  bool             `json:"hasMore"`
}

// TC-ADM-30（AC-ADM-30-1～4、30-7、24-4）
func TestTCADM30MessageMeta(t *testing.T) {
	e := newAdmEnv(t, func(c *app.Config) { c.SendRate = 10000 })
	e.users("u1", "u2", "u3")
	e.createGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	for i := range 120 {
		c, text := u1, "ZXQ-BODY-123 你好"
		if i < 7 {
			c = u2
		}
		if f := c.sendText("group", "g1", fmt.Sprintf("cid-meta-%06d", i), text); f.Error != nil {
			t.Fatalf("send %d: %+v", i, f.Error)
		}
	}

	seqs := func(p metaPage) (first, last int) {
		return int(p.Messages[0]["seq"].(float64)), int(p.Messages[len(p.Messages)-1]["seq"].(float64))
	}
	var p metaPage
	code, body := e.signed(e.c, "POST", "/admin/convs/messages/query", map[string]any{"convId": "group:g1", "limit": 50}, signOpts{requestID: "req-meta"})
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	// 抓响应：既没有特征串，也没有 body 字段（AC-ADM-30-7）。
	if s := string(body); strings.Contains(s, "ZXQ-BODY-123") || strings.Contains(s, `"body"`) {
		t.Errorf("response leaks the message body: %.200s", s)
	}
	_ = json.Unmarshal(body, &p)
	if a, b := seqs(p); a != 120 || b != 71 || !p.HasMore {
		t.Errorf("page 1 = %d..%d more=%v", a, b, p.HasMore)
	}
	if p.Messages[0]["bodyLength"].(float64) != float64(len([]rune("ZXQ-BODY-123 你好"))) {
		t.Errorf("bodyLength = %v", p.Messages[0]["bodyLength"])
	}
	for _, k := range []string{"seq", "msgId", "from", "type", "bodyLength", "ts"} {
		if _, ok := p.Messages[0][k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if len(p.Messages[0]) != 6 {
		t.Errorf("unexpected fields: %v", p.Messages[0])
	}
	got := e.auditEntries(e.c, "requestId=req-meta")
	if len(got) != 2 || !slices.Contains(got[0].Targets.IDs, "group:g1") {
		t.Errorf("audit = %+v", got)
	}

	p = metaPage{}
	e.post("/admin/convs/messages/query", map[string]any{"convId": "group:g1", "beforeSeq": 71}, &p)
	if a, b := seqs(p); a != 70 || b != 21 || !p.HasMore {
		t.Errorf("page 2 = %d..%d", a, b)
	}
	p = metaPage{}
	e.post("/admin/convs/messages/query", map[string]any{"convId": "group:g1", "beforeSeq": 21}, &p)
	if a, b := seqs(p); a != 20 || b != 1 || p.HasMore {
		t.Errorf("page 3 = %d..%d more=%v", a, b, p.HasMore)
	}

	p = metaPage{}
	e.post("/admin/convs/messages/query", map[string]any{"convId": "group:g1", "from": "u2"}, &p)
	if len(p.Messages) != 7 {
		t.Errorf("from u2 = %d", len(p.Messages))
	}

	code, body = e.signed(e.c, "POST", "/admin/convs/messages/query", map[string]any{"convId": "c2c:a"}, signOpts{})
	expectCode(t, "bad convId", code, body, 400, 4007)
	code, body = e.signed(e.c, "POST", "/admin/convs/messages/query", map[string]any{"convId": "group:g1", "limit": 51}, signOpts{})
	expectCode(t, "bad limit", code, body, 400, 4000)
	code, body = e.signed(e.c, "POST", "/admin/convs/messages/query", map[string]any{"convId": "group:g1", "from": "a b"}, signOpts{})
	expectCode(t, "bad from", code, body, 400, 4004)

	p = metaPage{}
	e.post("/admin/convs/messages/query", map[string]any{"convId": "c2c:x:y"}, &p)
	if p.Messages == nil || len(p.Messages) != 0 || p.HasMore {
		t.Errorf("never-chatted = %+v", p)
	}

	// 已解散的群照常可查（AC-ADM-24-4）。
	if code, body := e.signed(e.c, "DELETE", "/admin/groups/g1", nil, signOpts{}); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	p = metaPage{}
	e.post("/admin/convs/messages/query", map[string]any{"convId": "group:g1"}, &p)
	if len(p.Messages) != 50 {
		t.Errorf("after dissolve = %d", len(p.Messages))
	}
}
