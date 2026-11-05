package e2e

// 系统推送（PRD-live v1.0 L1，BE-228、BE-229）。用例 ID 见 docs/testcases/TC-PUSH.md，AC 是判据。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
)

const (
	day     = 24 * time.Hour
	hourMs  = int64(3600 * 1000)
	pushTTL = int64(604800)
)

type pushEnv struct {
	*socEnv
	pu  testCaller // 只有 push.user，没有 pushTypes
	pa  testCaller // push.user、push.all、push.room；pushTypes=[notice, goal]
	mod testCaller // 封禁
}

func newPushEnv(t *testing.T, mutate func(*app.Config)) *pushEnv {
	t.Helper()
	pu := hmacCaller("push-user", adminauth.ScopePushUser)
	pa := hmacCaller("push-all", adminauth.ScopePushUser, adminauth.ScopePushAll, adminauth.ScopePushRoom)
	pa.cfg.PushTypes = []string{"notice", "goal"}
	mod := hmacCaller("mod", adminauth.ScopeUserBan)
	s := newSocEnvCfg(t, func(c *app.Config) {
		c.Push.Enabled = true
		c.TestDisableJobs = []string{"push_purge"}
		c.AdminCallers = append(c.AdminCallers, pu.cfg, pa.cfg, mod.cfg)
		if mutate != nil {
			mutate(c)
		}
	})
	return &pushEnv{socEnv: s, pu: pu, pa: pa, mod: mod}
}

type pushResp struct {
	PushID   string `json:"pushId"`
	Created  bool   `json:"created"`
	Accepted *int   `json:"accepted"`
}

func (p *pushEnv) push(c testCaller, body map[string]any) (int, []byte) {
	p.t.Helper()
	return p.signed(c, http.MethodPost, "/admin/push", body, signOpts{ts: timeMs(p.now)})
}

func (p *pushEnv) mustPush(c testCaller, body map[string]any) pushResp {
	p.t.Helper()
	code, b := p.push(c, body)
	if code != http.StatusOK {
		p.t.Fatalf("push %v: %d %s", body["pushId"], code, b)
	}
	var r pushResp
	if err := json.Unmarshal(b, &r); err != nil {
		p.t.Fatal(err)
	}
	return r
}

func userPush(pushID string, users ...string) map[string]any {
	return map[string]any{"target": "user", "userIds": users, "type": "notice", "data": map[string]any{"text": "hello"}, "pushId": pushID}
}

func allPush(pushID string) map[string]any {
	return map[string]any{"target": "all", "type": "notice", "data": map[string]any{"text": "hello"}, "pushId": pushID}
}

func with(m map[string]any, kv ...any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

type pushRow struct {
	PushSeq   int64
	PushID    string
	Type      string
	Data      json.RawMessage
	CreatedAt int64
	ExpiresAt int64
}

func (p *pushEnv) inbox(uid string) []pushRow {
	p.t.Helper()
	code, b := p.adminDo(http.MethodGet, "/admin/test/push-inbox/"+uid, nil, adminKey)
	if code != http.StatusOK {
		p.t.Fatalf("inbox: %d %s", code, b)
	}
	var r struct{ Rows []pushRow }
	if err := json.Unmarshal(b, &r); err != nil {
		p.t.Fatal(err)
	}
	return r.Rows
}

func (p *pushEnv) runPurge() {
	p.t.Helper()
	if code, b := p.adminPost("/admin/test/jobs/push_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		p.t.Fatalf("purge: %d %s", code, b)
	}
}

func (p *pushEnv) fault(name string, body map[string]any) {
	p.t.Helper()
	if code, b := p.adminPost("/admin/test/faults/"+name, body, adminKey); code != http.StatusOK {
		p.t.Fatalf("fault %s: %d %s", name, code, b)
	}
}

func (c *client) waitPush() protocol.PushData {
	c.t.Helper()
	f := c.waitCmd(protocol.CmdPush)
	var d protocol.PushData
	if err := json.Unmarshal(f.Data, &d); err != nil {
		c.t.Fatalf("decode push: %v (%s)", err, f.Data)
	}
	return d
}

// expectNoPush 断言 d 内收不到 push 帧。
func (c *client) expectNoPush(d time.Duration) {
	c.t.Helper()
	for _, f := range c.pending {
		if f.Cmd == protocol.CmdPush {
			c.t.Fatalf("%s: unexpected push %s", c.uid, f.Data)
		}
	}
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			if f.Cmd == protocol.CmdPush {
				c.t.Fatalf("%s: unexpected push %s", c.uid, f.Data)
			}
			c.pending = append(c.pending, f)
		case <-deadline:
			return
		}
	}
}

func (c *client) pushSync(data map[string]any) protocol.PushSyncAck {
	c.t.Helper()
	return mustAck[protocol.PushSyncAck](c.t, c.call(protocol.CmdPushSync, data))
}

func pushIDs(in []protocol.PushData) []string {
	out := make([]string, len(in))
	for i, p := range in {
		out[i] = p.PushID
	}
	return out
}

func userList(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%04d", prefix, i)
	}
	return out
}

func (p *pushEnv) importAll(ids []string) {
	p.t.Helper()
	for c := range slices.Chunk(ids, 500) {
		p.users(c...)
	}
}

// ---- F-PUSH-01、F-PUSH-02 ----

// AC-PUSH-01-1（下发部分）、AC-ADM-50-1、AC-PUSH-02-1、AC-PUSH-07-1（协议客户端）：两人在线 3 秒内收到；字段正确；多端每个连接各一次。
func TestPushUserDelivered(t *testing.T) {
	p := newPushEnv(t, nil)
	u1web, u1pc, u2 := p.dial("u1", "web"), p.dial("u1", "pc"), p.dial("u2", "web")
	r := p.mustPush(p.pu, userPush("p1", "u1", "u2"))
	if !r.Created || r.PushID != "p1" || r.Accepted == nil || *r.Accepted != 2 {
		t.Fatalf("resp = %+v", r)
	}
	for _, c := range []*client{u1web, u1pc, u2} {
		d := c.waitPush() // waitTimeout 是 3 秒
		if d.PushID != "p1" || d.Type != "notice" || string(d.Data) != `{"text":"hello"}` || d.TS != t0 || d.Target != "user" || d.PushSeq != 1 || d.RoomID != "" {
			t.Errorf("%s got %+v", c.uid, d)
		}
		c.expectNoPush(50 * time.Millisecond) // 每个连接只收一次
	}
	a := mustPush(t, p, "all-1", p.pa, allPush("a1"))
	if !a.Created || a.Accepted != nil {
		t.Fatalf("target=all resp = %+v", a)
	}
	if d := u1web.waitPush(); d.Target != "all" || d.PushSeq != 0 || d.PushID != "a1" {
		t.Fatalf("target=all frame = %+v", d)
	}
}

func mustPush(t *testing.T, p *pushEnv, label string, c testCaller, body map[string]any) pushResp {
	t.Helper()
	code, b := p.push(c, body)
	if code != http.StatusOK {
		t.Fatalf("%s: %d %s", label, code, b)
	}
	var r pushResp
	json.Unmarshal(b, &r)
	// 响应里 all 不能有 accepted 字段（不只是 null）。
	if body["target"] == "all" && bytes.Contains(b, []byte("accepted")) {
		t.Fatalf("%s: target=all response has accepted: %s", label, b)
	}
	return r
}

// AC-PUSH-02-14：开关关闭时路由未注册（404），pushSync 4001；两个开关互相独立。
func TestPushDisabled(t *testing.T) {
	for _, v13 := range []bool{false, true} {
		t.Run(fmt.Sprintf("v13=%v", v13), func(t *testing.T) {
			c := hmacCaller("push-all", adminauth.ScopePushUser, adminauth.ScopePushAll) // 没有 pushTypes：开关关闭时不校验（AC-PUSH-09-9）
			s := newSocEnvCfg(t, func(cfg *app.Config) {
				cfg.V13AdminFeatures = v13
				cfg.QuarantineKey = testQuarantineKey
				cfg.AdminCallers = append(cfg.AdminCallers, c.cfg)
			})
			code, b := s.signed(c, http.MethodPost, "/admin/push", userPush("p1", "u1"), signOpts{ts: timeMs(s.now)})
			if code != http.StatusNotFound {
				t.Errorf("POST /admin/push = %d %s, want 404", code, b)
			}
			if code, _ := s.adminDo(http.MethodGet, "/admin/test/push-inbox/u1", nil, adminKey); code != http.StatusNotFound {
				t.Errorf("test route = %d, want 404", code)
			}
			mustErr(t, s.dial("u1", "web").call(protocol.CmdPushSync, map[string]any{"afterSeq": 0}), protocol.CodeUnknownCmd)
		})
	}
	// 反过来：IM_PUSH_ENABLED 开、第二批关（newPushEnv 默认如此）。
	p := newPushEnv(t, nil)
	p.users("u1")
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.dial("u1", "web").pushSync(map[string]any{"afterSeq": 0})
}

// AC-PUSH-02-2、AC-PUSH-02-10：4096 字节通过，4097 字节 400 且不投递；恰好 1000 人通过。
func TestPushSizeLimits(t *testing.T) {
	p := newPushEnv(t, nil)
	u1 := p.dial("u1", "web")
	data := func(n int) json.RawMessage { return json.RawMessage(`"` + strings.Repeat("x", n-2) + `"`) }
	code, b := p.push(p.pu, with(userPush("big", "u1"), "data", data(4097)))
	assertAdminErr(t, code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	u1.expectNoPush(noFrameWindow)
	if rows := p.inbox("u1"); len(rows) != 0 {
		t.Fatalf("inbox = %v", rows)
	}
	if r := p.mustPush(p.pu, with(userPush("big", "u1"), "data", data(4096))); !r.Created { // 4000 不占 pushId（4.4 节）
		t.Fatalf("resp = %+v", r)
	}
	if d := u1.waitPush(); len(d.Data) != 4096 {
		t.Fatalf("data length = %d", len(d.Data))
	}
	ids := userList("m", 1000)
	p.importAll(ids)
	if r := p.mustPush(p.pu, userPush("thousand", ids...)); r.Accepted == nil || *r.Accepted != 1000 {
		t.Fatalf("resp = %+v", r)
	}
}

// AC-PUSH-02-3、AC-PUSH-02-4、AC-ADM-50-6、AC-PUSH-06-8：不存在的用户 4042 整批不发，不占 pushId；1001 人 4131。
func TestPushUnknownUsersAndBatchLimit(t *testing.T) {
	p := newPushEnv(t, nil)
	u1 := p.dial("u1", "web")
	code, b := p.push(p.pu, userPush("p1", "u1", "ghost1", "ghost2"))
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	if got := fmt.Sprint(errDetails(t, b)["userIds"]); got != "[ghost1 ghost2]" {
		t.Errorf("details.userIds = %s", got)
	}
	u1.expectNoPush(noFrameWindow)
	if rows := p.inbox("u1"); len(rows) != 0 {
		t.Fatalf("u1 inbox = %v", rows)
	}
	// 修正后用同一个 pushId：第二次 created=true。
	if r := p.mustPush(p.pu, userPush("p1", "u1")); !r.Created {
		t.Fatalf("after fix: %+v", r)
	}
	code, b = p.push(p.pu, userPush("p2", userList("x", 1001)...))
	assertAdminErr(t, code, b, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	// 非法格式的 userId 也按不存在处理。
	code, b = p.push(p.pu, userPush("p3", "a b"))
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeUserNotFound)
}

// AC-PUSH-02-5、02-6、02-8、02-9、02-11：参数和 scope。
func TestPushParamValidation(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	for name, tc := range map[string]struct {
		caller     testCaller
		body       map[string]any
		status, cd int
	}{
		"all with ttl":        {p.pa, with(allPush("x"), "ttlSeconds", 3600), 400, 4000},
		"user ttl too big":    {p.pu, with(userPush("x", "u1"), "ttlSeconds", 604801), 400, 4000},
		"user ttl negative":   {p.pu, with(userPush("x", "u1"), "ttlSeconds", -1), 400, 4000},
		"user ttl fraction":   {p.pu, with(userPush("x", "u1"), "ttlSeconds", 1.5), 400, 4000},
		"all without scope":   {p.pu, allPush("x"), 403, 4030},
		"room, scope, off":    {p.pa, map[string]any{"target": "room", "roomId": "r1", "type": "notice", "data": map[string]any{}, "pushId": "x"}, 400, 4000},
		"room without scope":  {p.pu, map[string]any{"target": "room", "roomId": "r1", "type": "notice", "data": map[string]any{}, "pushId": "x"}, 403, 4030},
		"type empty":          {p.pu, with(userPush("x", "u1"), "type", ""), 400, 4000},
		"type 65":             {p.pu, with(userPush("x", "u1"), "type", strings.Repeat("t", 65)), 400, 4000},
		"pushId space":        {p.pu, userPush("p 1", "u1"), 400, 4000},
		"pushId 65":           {p.pu, userPush(strings.Repeat("p", 65), "u1"), 400, 4000},
		"pushId empty":        {p.pu, userPush("", "u1"), 400, 4000},
		"data missing":        {p.pu, map[string]any{"target": "user", "userIds": []string{"u1"}, "type": "notice", "pushId": "x"}, 400, 4000},
		"data null":           {p.pu, with(userPush("x", "u1"), "data", nil), 400, 4000},
		"target unknown":      {p.pu, with(userPush("x", "u1"), "target", "group"), 400, 4000},
		"unknown field":       {p.pu, with(userPush("x", "u1"), "foo", 1), 400, 4000},
		"user empty userIds":  {p.pa, userPush("x"), 400, 4000},
		"room without roomId": {p.pa, map[string]any{"target": "room", "type": "notice", "data": map[string]any{}, "pushId": "x"}, 400, 4000},
		"all with roomId":     {p.pa, with(allPush("x"), "roomId", "r1"), 400, 4000},
		"user with roomId":    {p.pu, with(userPush("x", "u1"), "roomId", "r1"), 400, 4000},
		"all with userIds":    {p.pa, with(allPush("x"), "userIds", []string{"u1"}), 400, 4000},
	} {
		code, b := p.push(tc.caller, tc.body)
		if code != tc.status || !bytes.Contains(b, []byte(fmt.Sprintf(`"code":%d`, tc.cd))) {
			t.Errorf("%s: %d %s, want %d / %d", name, code, b, tc.status, tc.cd)
		}
	}
	// 没有一次占了 pushId "x"：现在可以用。
	if r := p.mustPush(p.pu, userPush("x", "u1")); !r.Created {
		t.Fatalf("pushId x was occupied: %+v", r)
	}
	// 边界：64 字符 / 64 个汉字的 type、64 位 pushId。
	p.mustPush(p.pu, with(userPush(strings.Repeat("p", 64), "u1"), "type", strings.Repeat("类", 64)))
	// AC-PUSH-02-11：[u1,u1] 去重后 accepted=1，只收到一条。
	u1 := p.dial("u1", "web")
	if r := p.mustPush(p.pu, userPush("dup", "u1", "u1")); r.Accepted == nil || *r.Accepted != 1 {
		t.Fatalf("resp = %+v", r)
	}
	u1.waitPush()
	u1.expectNoPush(noFrameWindow)
	if rows := p.inbox("u1"); len(rows) != 3 {
		t.Fatalf("inbox rows = %d, want 3", len(rows))
	}
}

// AC-PUSH-02-12、AC-PUSH-02-13、AC-PUSH-03-5、AC-PUSH-04-3：不传和传 0 含义不同；im-server 不校验模板。
func TestPushTTLSemantics(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u2")
	u1 := p.dial("u1", "web")

	p.mustPush(p.pa, allPush("a1"))
	if d := u1.waitPush(); d.PushSeq != 0 {
		t.Fatalf("all has pushSeq: %+v", d)
	}
	if rows := p.inbox("u1"); len(rows) != 0 {
		t.Fatalf("target=all wrote to the inbox: %v", rows)
	}

	p.mustPush(p.pu, with(userPush("z1", "u1", "u2"), "ttlSeconds", 0))
	if d := u1.waitPush(); d.PushSeq != 0 || d.Target != "user" || d.PushID != "z1" {
		t.Fatalf("ttl=0 frame = %+v", d)
	}
	if rows := p.inbox("u1"); len(rows) != 0 {
		t.Fatalf("ttl=0 wrote to the inbox: %v", rows)
	}
	u2 := p.dial("u2", "web") // 离线时的 ttl=0 收不到
	if a := u2.pushSync(map[string]any{"afterSeq": 0}); len(a.Pushes) != 0 || a.LatestSeq != 0 {
		t.Fatalf("u2 sync = %+v", a)
	}

	r := p.mustPush(p.pu, with(userPush("d1", "u1"), "data", json.RawMessage(`["不符合任何模板",1,null]`))) // 不传 ttl：进收件箱，7 天
	if !r.Created {
		t.Fatalf("resp = %+v", r)
	}
	rows := p.inbox("u1")
	if len(rows) != 1 || rows[0].PushSeq != 1 || rows[0].ExpiresAt != rows[0].CreatedAt+pushTTL*1000 {
		t.Fatalf("rows = %+v", rows)
	}
}

// F-PUSH-02 规则 7 的优先级：白名单 → 4000 → 4094 / 幂等 → 4131 → 4042 → 4290。
func TestPushErrorPriority(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.UserRate = 1 })
	p.users("u1")
	// 白名单先于参数。
	code, b := p.push(p.pa, with(userPush("bad id", "u1"), "type", "other"))
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
	// 4000 先于 4094。
	p.mustPush(p.pu, userPush("p1", "u1"))
	code, b = p.push(p.pu, with(userPush("p1", "u1"), "ttlSeconds", -1))
	assertAdminErr(t, code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	// 4094 先于 4131、4042。
	code, b = p.push(p.pu, userPush("p1", userList("x", 1001)...))
	assertAdminErr(t, code, b, http.StatusConflict, protocol.CodePushIDConflict)
	code, b = p.push(p.pu, userPush("p1", "ghost"))
	assertAdminErr(t, code, b, http.StatusConflict, protocol.CodePushIDConflict)
	// 4131 先于 4042，4042 先于 4290（这时令牌已经用完）。
	code, b = p.push(p.pu, userPush("p2", userList("x", 1001)...))
	assertAdminErr(t, code, b, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	code, b = p.push(p.pu, userPush("p2", "ghost"))
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	code, b = p.push(p.pu, userPush("p2", "u1"))
	assertAdminErr(t, code, b, http.StatusTooManyRequests, protocol.CodeRateLimited)
	// 被 4042 拒绝的不扣令牌：推进一分钟补回 1 个，用掉 4042 后还在。
	p.advance(time.Minute)
	code, b = p.push(p.pu, userPush("p3", "ghost"))
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	if r := p.mustPush(p.pu, userPush("p3", "u1")); !r.Created {
		t.Fatalf("resp = %+v", r)
	}
}

// ---- F-PUSH-06 幂等 ----

// AC-PUSH-06-1、06-2、06-9、06-16：同内容 created=false 只投一次；内容不同 4094；键顺序和 userIds 顺序不算不同；超过 2^53 的整数不会被判成相同。
func TestPushIdempotentContent(t *testing.T) {
	p := newPushEnv(t, nil)
	u1, u2 := p.dial("u1", "web"), p.dial("u2", "web")
	first := with(userPush("p1", "u1", "u2"), "data", json.RawMessage(`{"a":1,"b":{"x":1,"y":2}}`))
	if r := p.mustPush(p.pu, first); !r.Created {
		t.Fatalf("first = %+v", r)
	}
	u1.waitPush()
	u2.waitPush()
	same := with(userPush("p1", "u2", "u1", "u1"), "data", json.RawMessage(`{"b":{"y":2,"x":1},"a":1}`), "ttlSeconds", pushTTL)
	r := p.mustPush(p.pu, same)
	if r.Created || r.Accepted == nil || *r.Accepted != 2 {
		t.Fatalf("replay = %+v", r)
	}
	u1.expectNoPush(noFrameWindow)
	if rows := p.inbox("u1"); len(rows) != 1 {
		t.Fatalf("inbox = %v", rows)
	}
	for name, body := range map[string]map[string]any{
		"data":    with(first, "data", json.RawMessage(`{"a":2,"b":{"x":1,"y":2}}`)),
		"users":   with(first, "userIds", []string{"u1"}),
		"type":    with(first, "type", "other"),
		"ttl":     with(first, "ttlSeconds", 3600),
		"target":  with(allPush("p1"), "data", json.RawMessage(`{"a":1,"b":{"x":1,"y":2}}`)),
		"ttlZero": with(first, "ttlSeconds", 0),
	} {
		c := p.pu
		if name == "target" {
			c = p.pa
		}
		code, b := p.push(c, body)
		assertAdminErr(t, code, b, http.StatusConflict, protocol.CodePushIDConflict)
		_ = name
	}
	u1.expectNoPush(noFrameWindow)
	// 超过 2^53 的两个整数：float64 下相等，保留原文后不同。
	p.mustPush(p.pu, with(userPush("big", "u1"), "data", json.RawMessage(`{"n":9007199254740993}`)))
	if d := u1.waitPush(); string(d.Data) != `{"n":9007199254740993}` {
		t.Fatalf("number was rewritten: %s", d.Data)
	}
	code, b := p.push(p.pu, with(userPush("big", "u1"), "data", json.RawMessage(`{"n":9007199254740992}`)))
	assertAdminErr(t, code, b, http.StatusConflict, protocol.CodePushIDConflict)
}

// AC-PUSH-06-11：pushId 全局唯一，别的调用方内容相同也是 4094。
func TestPushIDGlobalAcrossCallers(t *testing.T) {
	p := newPushEnv(t, nil)
	u1 := p.dial("u1", "web")
	p.mustPush(p.pu, userPush("p1", "u1"))
	u1.waitPush()
	for _, body := range []map[string]any{userPush("p1", "u1"), with(userPush("p1", "u1"), "data", map[string]any{"text": "other"})} {
		code, b := p.push(p.pa, body)
		assertAdminErr(t, code, b, http.StatusConflict, protocol.CodePushIDConflict)
	}
	u1.expectNoPush(noFrameWindow)
}

// AC-PUSH-06-4：全员推送 done 后重试不再广播。
func TestPushAllDoneNotRebroadcast(t *testing.T) {
	p := newPushEnv(t, nil)
	u1 := p.dial("u1", "web")
	p.mustPush(p.pa, allPush("a1"))
	u1.waitPush()
	if r := mustPush(t, p, "retry", p.pa, allPush("a1")); r.Created {
		t.Fatalf("retry = %+v", r)
	}
	u1.expectNoPush(noFrameWindow)
	// ttlSeconds=0 同样。
	p.mustPush(p.pu, with(userPush("z1", "u1"), "ttlSeconds", 0))
	u1.waitPush()
	if r := p.mustPush(p.pu, with(userPush("z1", "u1"), "ttlSeconds", 0)); r.Created || r.Accepted == nil || *r.Accepted != 1 {
		t.Fatalf("retry = %+v", r)
	}
	u1.expectNoPush(noFrameWindow)
}

// AC-PUSH-06-12：6 天内重试 created=false；9 天并清理后 created=true，不撞收件箱唯一键。
func TestPushIdempotencyRetention(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.advance(6 * day)
	if r := p.mustPush(p.pu, userPush("p1", "u1")); r.Created {
		t.Fatalf("6 days: %+v", r)
	}
	p.advance(3 * day)
	p.runPurge()
	r := p.mustPush(p.pu, userPush("p1", "u1"))
	if !r.Created {
		t.Fatalf("9 days after purge: %+v", r)
	}
	if rows := p.inbox("u1"); len(rows) != 1 || rows[0].PushSeq != 2 {
		t.Fatalf("rows = %+v", rows)
	}
}

// AC-PUSH-06-14：清理落后时（批大小 1，只跑一次，还剩一条收件箱行），幂等记录不能被删，重试不是 5000。
func TestPushPurgeKeepsRecordWhileInboxRemains(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.PurgeBatch = 1 })
	p.users("u1", "u2")
	p.mustPush(p.pu, userPush("p1", "u1", "u2"))
	p.advance(8*day + time.Hour)
	p.runPurge()
	if n := len(p.inbox("u1")) + len(p.inbox("u2")); n != 1 {
		t.Fatalf("inbox rows after one batch = %d, want 1", n)
	}
	r := p.mustPush(p.pu, userPush("p1", "u1", "u2"))
	if r.Created {
		t.Fatalf("retry = %+v, want created=false", r)
	}
	// 第二批清完之后，记录也可以删了。
	p.runPurge()
	if r := p.mustPush(p.pu, userPush("p1", "u1", "u2")); !r.Created {
		t.Fatalf("after second batch = %+v", r)
	}
}

// AC-PUSH-06-10：调用方已触发 4290，用已成功的 pushId 重放：200，不扣限速。
func TestPushReplayIgnoresRateLimit(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.UserRate = 1 })
	p.users("u1")
	p.mustPush(p.pu, userPush("p1", "u1"))
	code, b := p.push(p.pu, userPush("p2", "u1"))
	assertAdminErr(t, code, b, http.StatusTooManyRequests, protocol.CodeRateLimited)
	if r := p.mustPush(p.pu, userPush("p1", "u1")); r.Created {
		t.Fatalf("replay = %+v", r)
	}
	// 8.2 节：limits/reset 同时清推送限速。
	p.resetLimits("push-user")
	if r := p.mustPush(p.pu, userPush("p2", "u1")); !r.Created {
		t.Fatalf("after reset = %+v", r)
	}
}

// ---- F-PUSH-03 离线补收 ----

// AC-PUSH-03-1、AC-ADM-50-2：第 6 天上线收到；8 天后收不到（定时清理关闭，证明靠读取时过滤）。
func TestPushOfflineWindow(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u3", "u4")
	p.mustPush(p.pu, userPush("p3", "u3"))
	p.mustPush(p.pu, userPush("p4", "u4"))
	p.advance(6 * day)
	a := p.dial("u3", "web").pushSync(map[string]any{"afterSeq": 0})
	if fmt.Sprint(pushIDs(a.Pushes)) != "[p3]" || a.HasMore || a.LatestSeq != 1 || a.Pushes[0].TS != t0 || a.Pushes[0].Target != "user" || a.Pushes[0].PushSeq != 1 {
		t.Fatalf("day 6: %+v", a)
	}
	p.advance(2 * day)
	a = p.dial("u4", "web").pushSync(map[string]any{"afterSeq": 0})
	if len(a.Pushes) != 0 || a.HasMore || a.LatestSeq != 1 {
		t.Fatalf("day 8: %+v", a)
	}
	if rows := p.inbox("u4"); len(rows) != 1 {
		t.Fatalf("row was purged, the test proves nothing: %v", rows)
	}
}

// AC-PUSH-03-3：120 条，limit=50 翻页：共 120 条，升序，无重复；最后一页 hasMore=false。
func TestPushSyncPaging(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	for i := range 120 {
		p.mustPush(p.pu, userPush(fmt.Sprintf("p%d", i), "u1"))
	}
	c := p.dial("u1", "web")
	var seqs []int64
	after, pages := int64(0), 0
	for {
		a := c.pushSync(map[string]any{"afterSeq": after, "limit": 50})
		pages++
		for _, d := range a.Pushes {
			seqs = append(seqs, d.PushSeq)
		}
		if a.LatestSeq != 120 {
			t.Fatalf("latestSeq = %d", a.LatestSeq)
		}
		if !a.HasMore {
			break
		}
		if len(a.Pushes) == 0 {
			t.Fatal("hasMore=true with an empty page")
		}
		after = a.Pushes[len(a.Pushes)-1].PushSeq
	}
	if pages != 3 || len(seqs) != 120 {
		t.Fatalf("pages=%d rows=%d", pages, len(seqs))
	}
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seqs[%d] = %d", i, s)
		}
	}
}

// AC-PUSH-03-6、03-21、03-22、03-23：请求校验；fromLatest 不回放。
func TestPushSyncRequests(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.mustPush(p.pu, userPush("p2", "u1"))
	c := p.dial("u1", "web")
	for _, limit := range []int{1, 100} {
		if a := c.pushSync(map[string]any{"afterSeq": 0, "limit": limit}); len(a.Pushes) != min(limit, 2) {
			t.Errorf("limit %d: %+v", limit, a)
		}
	}
	for name, req := range map[string]map[string]any{
		"limit 0":          {"afterSeq": 0, "limit": 0},
		"limit 101":        {"afterSeq": 0, "limit": 101},
		"extra userId":     {"afterSeq": 0, "userId": "u2"},
		"limit only":       {"limit": 10},
		"both":             {"afterSeq": 1, "fromLatest": true},
		"fromLatest false": {"fromLatest": false},
		"latest and limit": {"fromLatest": true, "limit": 1},
		"negative":         {"afterSeq": -1},
		"neither":          {},
	} {
		f := c.call(protocol.CmdPushSync, req)
		if f.Error == nil || f.Error.Code != protocol.CodeBadRequest {
			t.Errorf("%s: %+v %s", name, f.Error, f.Data)
		}
	}
	mustErr(t, c.call(protocol.CmdPushSync, nil), protocol.CodeBadRequest)
	a := c.pushSync(map[string]any{"fromLatest": true})
	if a.Pushes == nil || len(a.Pushes) != 0 || a.HasMore || a.LatestSeq != 2 {
		t.Fatalf("fromLatest = %+v", a)
	}
}

// AC-PUSH-03-8：过期空洞：41（1 小时）、42（7 天）、43（1 小时），推进 2 小时只返回 42，hasMore=false，latestSeq=43。
func TestPushExpiredHoles(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	for i := 1; i <= 40; i++ {
		p.mustPush(p.pu, with(userPush(fmt.Sprintf("old%d", i), "u1"), "ttlSeconds", 60))
	}
	p.mustPush(p.pu, with(userPush("p41", "u1"), "ttlSeconds", 3600))
	p.mustPush(p.pu, userPush("p42", "u1"))
	p.mustPush(p.pu, with(userPush("p43", "u1"), "ttlSeconds", 3600))
	p.advance(2 * time.Hour)
	a := p.dial("u1", "web").pushSync(map[string]any{"afterSeq": 40, "limit": 1})
	if fmt.Sprint(pushIDs(a.Pushes)) != "[p42]" || a.Pushes[0].PushSeq != 42 || a.HasMore || a.LatestSeq != 43 {
		t.Fatalf("got %+v", a)
	}
}

// AC-PUSH-03-12：恰好等于 expires_at 不返回，早 1 毫秒返回。
func TestPushExpiryBoundary(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	p.mustPush(p.pu, with(userPush("p1", "u1"), "ttlSeconds", 3600))
	c := p.dial("u1", "web")
	p.setClock(t0 + hourMs - 1)
	if a := c.pushSync(map[string]any{"afterSeq": 0}); len(a.Pushes) != 1 {
		t.Fatalf("1ms before: %+v", a)
	}
	p.setClock(t0 + hourMs)
	if a := c.pushSync(map[string]any{"afterSeq": 0}); len(a.Pushes) != 0 || a.LatestSeq != 1 {
		t.Fatalf("at expiry: %+v", a)
	}
}

// AC-PUSH-03-13：全部过期并被清理后，序号继续递增，不回退。
func TestPushSeqSurvivesPurge(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	p.mustPush(p.pu, with(userPush("p1", "u1"), "ttlSeconds", 60))
	p.mustPush(p.pu, with(userPush("p2", "u1"), "ttlSeconds", 60))
	p.advance(time.Hour)
	p.runPurge()
	if rows := p.inbox("u1"); len(rows) != 0 {
		t.Fatalf("purge left %v", rows)
	}
	c := p.dial("u1", "web")
	if a := c.pushSync(map[string]any{"afterSeq": 0}); a.LatestSeq != 2 {
		t.Fatalf("latest after purge = %d", a.LatestSeq)
	}
	p.mustPush(p.pu, userPush("p3", "u1"))
	if d := c.waitPush(); d.PushSeq != 3 {
		t.Fatalf("pushSeq = %d, want 3", d.PushSeq)
	}
	if a := c.pushSync(map[string]any{"fromLatest": true}); a.LatestSeq != 3 {
		t.Fatalf("latest = %d", a.LatestSeq)
	}
}

// AC-PUSH-03-14：封禁期间的推送，解封后在有效期内补收。
func TestPushWhileBanned(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.V13AdminFeatures, c.QuarantineKey = true, testQuarantineKey })
	p.users("u1")
	opts := func() signOpts { return signOpts{ts: timeMs(p.now), operator: "op1"} }
	if code, b := p.signed(p.mod, http.MethodPost, "/admin/users/u1/ban", map[string]any{"durationSeconds": nil, "reason": "spam"}, opts()); code != http.StatusOK {
		t.Fatalf("ban: %d %s", code, b)
	}
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.advance(day)
	if code, b := p.signed(p.mod, http.MethodPost, "/admin/users/u1/unban", map[string]any{"reason": "ok"}, opts()); code != http.StatusOK {
		t.Fatalf("unban: %d %s", code, b)
	}
	if a := p.dial("u1", "web").pushSync(map[string]any{"afterSeq": 0}); fmt.Sprint(pushIDs(a.Pushes)) != "[p1]" {
		t.Fatalf("got %+v", a)
	}
}

// ---- F-PUSH-04 全员 ----

// AC-PUSH-04-1、AC-ADM-50-3：5 人在线都收到，离线的 u9 上线收不到。
func TestPushAllOnlineOnly(t *testing.T) {
	p := newPushEnv(t, nil)
	var online []*client
	for i := range 5 {
		online = append(online, p.dial(fmt.Sprintf("u%d", i), "web"))
	}
	p.users("u9")
	p.mustPush(p.pa, allPush("a1"))
	for _, c := range online {
		if d := c.waitPush(); d.Target != "all" || d.PushID != "a1" || d.PushSeq != 0 {
			t.Errorf("%s: %+v", c.uid, d)
		}
	}
	u9 := p.dial("u9", "web")
	if a := u9.pushSync(map[string]any{"afterSeq": 0}); len(a.Pushes) != 0 || a.LatestSeq != 0 {
		t.Fatalf("u9 = %+v", a)
	}
	u9.expectNoPush(noFrameWindow)
}

type noBroadcast struct{ delivery.Router }

// AC-PUSH-04-6：配置移除广播能力的构建：启动失败。
func TestPushRequiresBroadcast(t *testing.T) {
	cfg := app.Config{
		DBPath: t.TempDir() + "/im.db", AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey, TestMode: true,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Push:           app.PushConfig{Enabled: true},
		TestRouterWrap: func(r delivery.Router) delivery.Router { return noBroadcast{r} },
	}
	if _, err := app.New(cfg); err == nil || !strings.Contains(err.Error(), "broadcast") {
		t.Fatalf("err = %v", err)
	}
	// 同样的替身，开关关闭：正常启动，现有行为不变。
	cfg.Push.Enabled = false
	cfg.DBPath = t.TempDir() + "/im2.db"
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
}

// ---- F-PUSH-07 多端 ----

// AC-PUSH-07-2、AC-PUSH-07-4：web 在线收到，不影响离线的 android 补收；u1 的 pushSync 不含 u2 的。
func TestPushMultiDeviceAndIsolation(t *testing.T) {
	p := newPushEnv(t, nil)
	web := p.dial("u1", "web")
	p.users("u2")
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.mustPush(p.pu, userPush("other", "u2"))
	web.waitPush()
	android := p.dial("u1", "android")
	a := android.pushSync(map[string]any{"afterSeq": 0})
	if fmt.Sprint(pushIDs(a.Pushes)) != "[p1]" || a.LatestSeq != 1 {
		t.Fatalf("android = %+v", a)
	}
	// 帧里没有目标用户字段：web 上也不会看到 u2 的。
	web.expectNoPush(noFrameWindow)
}

// ---- F-PUSH-09 审计、限速、白名单 ----

// AC-PUSH-09-1、AC-ADM-50-8（im-server 审计部分）：有 pushId、type、目标数量、长度和 SHA-256，没有 data 明文。
func TestPushAudit(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1", "u2")
	const secret = "SENSITIVE-PAYLOAD-MARKER"
	data := json.RawMessage(`{"text":"` + secret + `"}`)
	body := with(userPush("p1", "u1", "u2", "u2"), "data", data)
	if code, b := p.signed(p.pu, http.MethodPost, "/admin/push", body, signOpts{ts: timeMs(p.now), requestID: "req-push-1", operator: "op1"}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	sum := sha256.Sum256(data)
	audit := p.mustAdm(http.MethodGet, fmt.Sprintf("/admin/audit?from=%d&to=%d&requestId=req-push-1", t0-1000, t0+1000), nil)
	if bytes.Contains(audit, []byte(secret)) || bytes.Contains(audit, []byte(`"u1"`)) {
		t.Fatalf("audit leaks data or userIds: %s", audit)
	}
	var r struct {
		Entries []struct {
			Phase, Route string
			Code         int
			Caller       string
			Summary      map[string]any
			Targets      map[string]any
		}
	}
	if err := json.Unmarshal(audit, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Entries) != 2 {
		t.Fatalf("entries = %s", audit)
	}
	for _, e := range r.Entries {
		if e.Route != "POST /admin/push" || e.Caller != "push-user" || e.Summary["pushId"] != "p1" || e.Summary["type"] != "notice" || e.Summary["target"] != "user" {
			t.Errorf("entry = %+v", e)
		}
		if e.Phase == "result" {
			if e.Code != 0 || e.Summary["targetCount"] != float64(2) || e.Summary["dataLength"] != float64(len(data)) || e.Summary["dataSha256"] != hex.EncodeToString(sum[:]) {
				t.Errorf("result summary = %+v", e.Summary)
			}
		}
	}
	// 失败的请求同样有 result，错误码是业务码。
	p.signed(p.pu, http.MethodPost, "/admin/push", with(userPush("p1", "u1"), "data", map[string]any{"x": 1}), signOpts{ts: timeMs(p.now), requestID: "req-push-2"})
	audit = p.mustAdm(http.MethodGet, fmt.Sprintf("/admin/audit?from=%d&to=%d&requestId=req-push-2", t0-1000, t0+1000), nil)
	if !bytes.Contains(audit, []byte(`"code":4094`)) {
		t.Errorf("conflict not audited: %s", audit)
	}
}

// AC-PUSH-09-2：全员第 21 次 429；推进 1 分钟恢复。
func TestPushAllRateLimit(t *testing.T) {
	p := newPushEnv(t, nil)
	for i := range 20 {
		p.mustPush(p.pa, allPush(fmt.Sprintf("a%d", i)))
	}
	code, b := p.push(p.pa, allPush("a20"))
	assertAdminErr(t, code, b, http.StatusTooManyRequests, protocol.CodeRateLimited)
	p.advance(time.Minute)
	if r := p.mustPush(p.pa, allPush("a20")); !r.Created {
		t.Fatalf("after a minute: %+v", r)
	}
}

// AC-PUSH-09-3：target=user 限速配置为 2，第 3 次 429。按调用方计：另一个调用方不受影响。
func TestPushUserRateLimit(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.UserRate = 2 })
	p.users("u1")
	p.mustPush(p.pu, userPush("p1", "u1"))
	p.mustPush(p.pu, userPush("p2", "u1"))
	code, b := p.push(p.pu, userPush("p3", "u1"))
	assertAdminErr(t, code, b, http.StatusTooManyRequests, protocol.CodeRateLimited)
	p.mustPush(p.pa, userPush("p4", "u1"))
}

// AC-PUSH-09-4：pushSync 限速配置为 2：u1 第 3 次 ack 错误 4290；u2 不受影响；limits/reset 清零。
func TestPushSyncRateLimit(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.SyncRate = 2 })
	u1, u2 := p.dial("u1", "web"), p.dial("u2", "web")
	u1.pushSync(map[string]any{"afterSeq": 0})
	u1.pushSync(map[string]any{"fromLatest": true})
	mustErr(t, u1.call(protocol.CmdPushSync, map[string]any{"afterSeq": 0}), protocol.CodeRateLimited)
	// 参数错误不计入：先报参数错误。
	mustErr(t, u1.call(protocol.CmdPushSync, map[string]any{"limit": 1}), protocol.CodeBadRequest)
	u2.pushSync(map[string]any{"afterSeq": 0})
	p.resetLimits("u1")
	u1.pushSync(map[string]any{"afterSeq": 0})
}

// AC-PUSH-09-5：收件箱上限 3，推 5 条：只返回最新 3 条，序号 3、4、5。
func TestPushInboxMax(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.InboxMax = 3 })
	p.users("u1")
	for i := range 5 {
		p.mustPush(p.pu, userPush(fmt.Sprintf("p%d", i), "u1"))
	}
	a := p.dial("u1", "web").pushSync(map[string]any{"afterSeq": 0})
	if len(a.Pushes) != 3 || a.Pushes[0].PushSeq != 3 || a.Pushes[2].PushSeq != 5 || a.LatestSeq != 5 || a.HasMore {
		t.Fatalf("got %+v", a)
	}
	if rows := p.inbox("u1"); len(rows) != 3 {
		t.Fatalf("rows = %v", rows)
	}
}

// AC-PUSH-09-6、09-8：pushTypes 对所有 target 生效，不占 pushId；没有 push.all 也没配 pushTypes 的调用方不受限制。
func TestPushTypeWhitelist(t *testing.T) {
	gl := hmacCaller("goal-only", adminauth.ScopePushUser, adminauth.ScopePushAll)
	gl.cfg.PushTypes = []string{"goal"}
	p := newPushEnv(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, gl.cfg) })
	u1 := p.dial("u1", "web")
	if r := p.mustPush(gl, with(allPush("g1"), "type", "goal")); !r.Created {
		t.Fatalf("goal: %+v", r)
	}
	u1.waitPush()
	code, b := p.push(gl, allPush("n1"))
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.push(gl, userPush("n2", "u1"))
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
	u1.expectNoPush(noFrameWindow)
	// 不占 pushId：这两个号别的内容还能用。
	if r := p.mustPush(gl, with(allPush("n1"), "type", "goal")); !r.Created {
		t.Fatalf("n1 was occupied: %+v", r)
	}
	if r := p.mustPush(p.pu, with(userPush("n2", "u1"), "type", "任意类型")); !r.Created {
		t.Fatalf("n2: %+v", r)
	}
}

// AC-PUSH-09-7、09-9：开关开启时持有 push.all 没有 pushTypes：启动失败，错误信息有调用方 ID，没有密钥；开关关闭时正常启动。
func TestPushStartupRequiresPushTypes(t *testing.T) {
	bad := hmacCaller("no-types", adminauth.ScopePushAll)
	empty := hmacCaller("empty-types", adminauth.ScopePushAll)
	empty.cfg.PushTypes = []string{}
	cfg := app.Config{
		DBPath: t.TempDir() + "/im.db", AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey, TestMode: true,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AdminCallers: []adminauth.Caller{bad.cfg, empty.cfg},
		Push: app.PushConfig{Enabled: true},
	}
	_, err := app.New(cfg)
	if err == nil || !strings.Contains(err.Error(), "no-types") || !strings.Contains(err.Error(), "empty-types") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), string(bad.secret)) {
		t.Fatal("error contains the secret")
	}
	cfg.Push.Enabled = false
	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("disabled: %v", err)
	}
	a.Close()
}

// AC-KEY-02-2 的推送部分：全部推送路由 × 有 / 没有 scope；push.room 按 AC-PUSH-02-8 判定（有 scope 400，没有 403）。
func TestEveryPushRouteChecksScope(t *testing.T) {
	samples := map[string]struct {
		method, path string
		body         any
		want         int
	}{
		"POST /admin/push":                                   {"POST", "/admin/push", userPush("p1", "alice"), 200},
		"GET /admin/test/push-inbox/{userId}":                {"GET", "/admin/test/push-inbox/alice", nil, 200},
		"POST /admin/test/faults/push-skip-delivery":         {"POST", "/admin/test/faults/push-skip-delivery", map[string]any{"pushId": "p1"}, 200},
		"POST /admin/test/faults/push-exit-before-broadcast": {"POST", "/admin/test/faults/push-exit-before-broadcast", map[string]any{"pushId": "never"}, 200},
		"POST /admin/test/faults/push-fail-at":               {"POST", "/admin/test/faults/push-fail-at", map[string]any{"index": 0}, 200},
	}
	env := func(callers ...testCaller) *socEnv {
		return newSocEnvCfg(t, func(c *app.Config) {
			c.Push.Enabled = true
			for _, tc := range callers {
				c.AdminCallers = append(c.AdminCallers, tc.cfg)
			}
		})
	}
	withPushTypes := func(c testCaller) testCaller {
		c.cfg.PushTypes = []string{"notice"}
		return c
	}
	seen := 0
	for _, r := range env().app.AdminRoutes() {
		key := r.Method + " " + r.Pattern
		if _, first := adminSamples[key]; first {
			continue
		}
		s, ok := samples[key]
		if !ok {
			t.Errorf("route %s (scope %s) has no sample", key, r.Scope)
			continue
		}
		seen++
		yes, no := withPushTypes(hmacCaller("with", r.Scope)), withPushTypes(hmacCaller("without", nonBodyScopes(r.Scope)...))
		e := env(yes, no)
		e.users("alice")
		code, body := e.signed(no, s.method, s.path, s.body, signOpts{ts: timeMs(e.now)})
		expectCode(t, key+" without", code, body, http.StatusForbidden, protocol.CodeForbidden)
		code, body = e.signed(yes, s.method, s.path, s.body, signOpts{ts: timeMs(e.now)})
		expectCode(t, key+" with", code, body, s.want, 0)
	}
	if seen != len(samples) {
		t.Errorf("checked %d routes, want %d", seen, len(samples))
	}
	// 按 target 判 scope：另外两个 scope 的样本；旧密钥不能调用（403）。
	for scope, tc := range map[adminauth.Scope]struct {
		body       map[string]any
		want, code int
	}{
		adminauth.ScopePushAll:  {allPush("a1"), 200, 0},
		adminauth.ScopePushRoom: {map[string]any{"target": "room", "roomId": "r1", "type": "notice", "data": map[string]any{}, "pushId": "r1"}, 400, protocol.CodeBadRequest},
	} {
		yes, no := withPushTypes(hmacCaller("with", scope)), withPushTypes(hmacCaller("without", nonBodyScopes(scope)...))
		e := env(yes, no)
		code, body := e.signed(no, http.MethodPost, "/admin/push", tc.body, signOpts{ts: timeMs(e.now)})
		expectCode(t, string(scope)+" without", code, body, http.StatusForbidden, protocol.CodeForbidden)
		code, body = e.signed(yes, http.MethodPost, "/admin/push", tc.body, signOpts{ts: timeMs(e.now)})
		expectCode(t, string(scope)+" with", code, body, tc.want, tc.code)
	}
	e := env()
	code, b := e.adminPost("/admin/push", userPush("p1", "alice"), adminKey)
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
}

// ---- F-PUSH-10 可观测 ----

// AC-PUSH-10-1：各 target 推送一次（room 在 L1 被挡住，不计数）；计数正确，标签没有 type、userId。
func TestPushMetrics(t *testing.T) {
	p := newPushEnv(t, nil)
	web, pc := p.dial("u1", "web"), p.dial("u1", "pc")
	p.mustPush(p.pu, with(userPush("p1", "u1"), "type", "notice"))
	web.waitPush()
	pc.waitPush()
	p.mustPush(p.pu, userPush("p1", "u1")) // 重放不计数
	p.mustPush(p.pu, with(userPush("z1", "u1"), "ttlSeconds", 0))
	p.mustPush(p.pa, allPush("a1"))
	web.pushSync(map[string]any{"afterSeq": 0})

	srv := httptest.NewServer(p.app.MetricsHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	text := string(raw)
	for _, want := range []string{
		`im_pushes_total{target="user"} 2`,
		`im_pushes_total{target="all"} 1`,
		`im_push_deliveries_total{target="user"} 4`, // p1、z1 各发给 2 个连接
		`im_push_deliveries_total{target="all"} 2`,
		`im_push_inbox_rows 1`,
		`im_commands_total{cmd="pushSync",code="0"} 1`,
		`im_command_duration_seconds_count{cmd="pushSync"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "im_push") {
			continue
		}
		if strings.Contains(line, "type=") || strings.Contains(line, "userId") || strings.Contains(line, "uid=") || strings.Contains(line, "notice") {
			t.Errorf("bad label: %s", line)
		}
	}
	// 清理之后行数指标减少。
	p.advance(8 * day)
	p.runPurge()
	resp2, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	raw, _ = io.ReadAll(resp2.Body)
	if !strings.Contains(string(raw), "im_push_inbox_rows 0") {
		t.Error("inbox gauge not updated after purge")
	}
}

// ---- F-PUSH-11 并发 ----

// AC-PUSH-11-2：提交后跳过下发：u1 没收到 p1 的帧，收到 p2 的帧（序号跳号），pushSync 能取到 p1。
func TestPushSkipDeliveryLeavesGap(t *testing.T) {
	p := newPushEnv(t, nil)
	u1 := p.dial("u1", "web")
	p.fault("push-skip-delivery", map[string]any{"pushId": "p1"})
	p.mustPush(p.pu, userPush("p1", "u1"))
	u1.expectNoPush(noFrameWindow)
	p.mustPush(p.pu, userPush("p2", "u1"))
	if d := u1.waitPush(); d.PushID != "p2" || d.PushSeq != 2 {
		t.Fatalf("frame = %+v", d)
	}
	a := u1.pushSync(map[string]any{"afterSeq": 0})
	if fmt.Sprint(pushIDs(a.Pushes)) != "[p1 p2]" {
		t.Fatalf("sync = %+v", a)
	}
}

// AC-PUSH-11-3：50 个 goroutine 给同一用户推送：pushSeq 连续无重复。
func TestPushConcurrentSeq(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, b := p.push(p.pu, userPush(fmt.Sprintf("p%d", i), "u1")); code != http.StatusOK {
				t.Errorf("p%d: %d %s", i, code, b)
			}
		}()
	}
	wg.Wait()
	a := p.dial("u1", "web").pushSync(map[string]any{"afterSeq": 0})
	if len(a.Pushes) != 50 || a.LatestSeq != 50 {
		t.Fatalf("rows=%d latest=%d", len(a.Pushes), a.LatestSeq)
	}
	for i, d := range a.Pushes {
		if d.PushSeq != int64(i+1) {
			t.Fatalf("pushes[%d].pushSeq = %d", i, d.PushSeq)
		}
	}
}

// AC-PUSH-11-4：同一 pushId 同内容并发 20 次：恰好一次 created=true，收件箱一条；全员推送同样。
func TestPushConcurrentSameID(t *testing.T) {
	p := newPushEnv(t, nil)
	p.users("u1")
	for _, tc := range []struct {
		caller testCaller
		body   map[string]any
	}{{p.pu, userPush("p1", "u1")}, {p.pa, allPush("a1")}, {p.pu, with(userPush("z1", "u1"), "ttlSeconds", 0)}} {
		var created atomic.Int32
		var wg sync.WaitGroup
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				code, b := p.push(tc.caller, tc.body)
				var r pushResp
				json.Unmarshal(b, &r)
				if code != http.StatusOK {
					t.Errorf("%d %s", code, b)
				}
				if r.Created {
					created.Add(1)
				}
			}()
		}
		wg.Wait()
		if created.Load() != 1 {
			t.Errorf("%v: created %d times", tc.body["pushId"], created.Load())
		}
	}
	if rows := p.inbox("u1"); len(rows) != 1 {
		t.Fatalf("inbox rows = %d", len(rows))
	}
}

// AC-PUSH-11-5：两组交叠的 userIds、顺序相反，并发 100 轮：全部成功。加锁顺序断言见 store 包 TestPushLockOrder。
func TestPushOverlappingBatches(t *testing.T) {
	p := newPushEnv(t, func(c *app.Config) { c.Push.UserRate = 100000 })
	a := userList("o", 30)
	b := slices.Clone(a[10:])
	b = append(b, userList("q", 10)...)
	slices.Reverse(b)
	p.importAll(a)
	p.importAll(b)
	for round := range 100 {
		var wg sync.WaitGroup
		for i, ids := range [][]string{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if code, body := p.push(p.pu, userPush(fmt.Sprintf("r%d-%d", round, i), ids...)); code != http.StatusOK {
					t.Errorf("round %d: %d %s", round, code, body)
				}
			}()
		}
		wg.Wait()
	}
	// 交叠的用户两组都收到：序号 200，连续。
	if rows := p.inbox(a[15]); len(rows) != 200 || rows[199].PushSeq != 200 {
		t.Fatalf("rows = %d", len(rows))
	}
}

// AC-PUSH-11-6：1000 人的提交，第 500 人写入失败：整批回滚，没有下发，没有幂等记录。
func TestPushWriteFailureRollsBack(t *testing.T) {
	p := newPushEnv(t, nil)
	ids := userList("m", 1000)
	p.importAll(ids)
	c := p.dial(ids[0], "web")
	p.fault("push-fail-at", map[string]any{"index": 499})
	code, b := p.push(p.pu, userPush("p1", ids...))
	assertAdminErr(t, code, b, http.StatusInternalServerError, protocol.CodeInternal)
	c.expectNoPush(noFrameWindow)
	for _, uid := range []string{ids[0], ids[498], ids[499], ids[999]} {
		if rows := p.inbox(uid); len(rows) != 0 {
			t.Fatalf("%s inbox = %v", uid, rows)
		}
	}
	if a := c.pushSync(map[string]any{"fromLatest": true}); a.LatestSeq != 0 {
		t.Fatalf("seq survived rollback: %d", a.LatestSeq)
	}
	// 没有幂等记录、注入只生效一次：同号再提交 created=true。
	if r := p.mustPush(p.pu, userPush("p1", ids...)); !r.Created || *r.Accepted != 1000 {
		t.Fatalf("retry = %+v", r)
	}
	if d := c.waitPush(); d.PushSeq != 1 {
		t.Fatalf("seq = %d", d.PushSeq)
	}
}

// 8.2 节：IM_TEST_DISABLE_JOBS 只影响定时触发，名字写错启动失败；测试模式之外忽略。
func TestPushDisableJobsConfig(t *testing.T) {
	cfg := app.Config{
		DBPath: t.TempDir() + "/im.db", AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey, TestMode: true,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), TestDisableJobs: []string{"no_such_job"},
	}
	if _, err := app.New(cfg); err == nil || !strings.Contains(err.Error(), "no_such_job") {
		t.Fatalf("err = %v", err)
	}
	cfg.TestMode = false
	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("outside test mode: %v", err)
	}
	a.Close()
}

// 安全要求：token、推送负载不进日志，包括失败路径。
func TestPushNotInLogs(t *testing.T) {
	var buf syncBuffer
	p := newPushEnv(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	const marker = "SENSITIVE-PAYLOAD-MARKER"
	data := json.RawMessage(`{"text":"` + marker + `"}`)
	tok := p.token("u1")
	c := p.dialURL("u1", p.wsURL(tok, "web"))
	p.mustPush(p.pu, with(userPush("p1", "u1"), "data", data))
	c.waitPush()
	p.mustPush(p.pa, with(allPush("a1"), "data", data))
	c.waitPush()
	c.pushSync(map[string]any{"afterSeq": 0})
	p.push(p.pu, with(userPush("p1", "u1"), "data", json.RawMessage(`{"other":"`+marker+`"}`))) // 4094
	p.fault("push-fail-at", map[string]any{"index": 0})
	p.push(p.pu, with(userPush("p2", "u1"), "data", data)) // 5000：走到 a.log.Error 的路径
	if !strings.Contains(buf.String(), "write push inbox") {
		t.Fatalf("the failure path did not run: %s", buf.String())
	}
	if s := buf.String(); strings.Contains(s, marker) || strings.Contains(s, tok) {
		t.Fatalf("log leaks payload or token:\n%s", s)
	}
}
