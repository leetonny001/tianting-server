package e2e

// PRD-admin P1（BE-240、BE-241、BE-243）：全群禁言、数据概览、用户所在群与用户会话列表。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type p1Env struct {
	*modEnv
	stats testCaller // 只有 stats.read
	reads testCaller // 只有 group.read
	meta  testCaller // 只有 msg.meta
	other testCaller // 除 stats.read、group.read、msg.meta 以外的 scope
}

func newP1Env(t *testing.T) *p1Env {
	t.Helper()
	p := &p1Env{
		stats: hmacCaller("stats", adminauth.ScopeStatsRead),
		reads: hmacCaller("reads", adminauth.ScopeGroupRead),
		meta:  hmacCaller("meta", adminauth.ScopeMsgMeta),
		other: hmacCaller("other", adminauth.ScopeUserRead, adminauth.ScopeUserSessions, adminauth.ScopeMuteWrite, adminauth.ScopeAuditRead),
	}
	p.modEnv = newModEnv(t, func(c *app.Config) {
		c.AdminCallers = append(c.AdminCallers, p.stats.cfg, p.reads.cfg, p.meta.cfg, p.other.cfg)
	})
	return p
}

func (p *p1Env) getAs(c testCaller, path string) (int, []byte) {
	p.t.Helper()
	return p.signed(c, http.MethodGet, path, nil, signOpts{ts: timeMs(p.now), requestID: fmt.Sprintf("r-%d", reqSeq.Add(1))})
}

// muteAllGroup 建一个自建群：alice 群主、carol 管理员、bob 成员，三人都在线（事件已经收完）。
func (p *p1Env) muteAllGroup() (gid string, alice, carol, bob *client) {
	p.t.Helper()
	p.usersNamed("alice", "bob", "carol")
	p.mutual("alice", "bob", "carol")
	alice, carol, bob = p.dial("alice", "web"), p.dial("carol", "web"), p.dial("bob", "web")
	gid = createG(p.t, alice, "bob", "carol").GroupID
	carol.waitEvent(protocol.EventConvAdded)
	bob.waitEvent(protocol.EventConvAdded)
	mustAck[protocol.Empty](p.t, alice.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	if got := convOf(p.t, carol, gid).MyRole; got != "admin" {
		p.t.Fatalf("carol role = %q", got)
	}
	drain(alice, carol, bob)
	return gid, alice, carol, bob
}

func (p *p1Env) muteAll(gid string) {
	p.t.Helper()
	p.must("/admin/mute", map[string]any{"scope": "groupAll", "groupId": gid, "reason": "整顿"})
}

func (p *p1Env) unmuteAll(gid string) {
	p.t.Helper()
	p.must("/admin/unmute", map[string]any{"scope": "groupAll", "groupId": gid, "reason": "结束"})
}

// drain 等连接上的提示消息、群变动事件都到齐并丢掉。
func drain(cs ...*client) {
	for _, c := range cs {
		c.expectNoFrameDrain()
	}
}

func ackMutedDetails(t *testing.T, f frame) map[string]any {
	t.Helper()
	if f.Error == nil || f.Error.Code != protocol.CodeMuted {
		t.Fatalf("error = %+v, want 4031", f.Error)
	}
	b, _ := json.Marshal(f.Error)
	return errDetails(t, b)
}

func wantGroupAll(t *testing.T, label string, f frame, gid string) {
	t.Helper()
	d := ackMutedDetails(t, f)
	if d["scope"] != "groupAll" || d["groupId"] != gid {
		t.Errorf("%s: details = %v", label, d)
	}
	if v, ok := d["mutedUntil"]; !ok || v != nil {
		t.Errorf("%s: mutedUntil = %v, want null", label, d)
	}
}

// TC-ADM-42-01（AC-ADM-42-1）：全群禁言期间群主、管理员能发，成员 4031（scope=groupAll）；关闭后恢复。
func TestTCADM4201GroupAllMuteSend(t *testing.T) {
	p := newP1Env(t)
	gid, alice, carol, bob := p.muteAllGroup()
	mustAck[protocol.SendAck](t, bob.sendText("group", gid, "m-all-0001", "before"))

	p.muteAll(gid)
	mustAck[protocol.SendAck](t, alice.sendText("group", gid, "m-all-0002", "owner"))
	mustAck[protocol.SendAck](t, carol.sendText("group", gid, "m-all-0003", "admin"))
	wantGroupAll(t, "bob", bob.sendText("group", gid, "m-all-0004", "member"), gid)

	// 被拒的消息没有落库：seq 只有 tip、before、owner、admin。
	sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "group:" + gid, "afterSeq": 0, "limit": 50}))
	for _, m := range sync.Messages {
		if m.From == "bob" && m.Body.Text != "before" {
			t.Errorf("rejected message was stored: %+v", m)
		}
	}
	// 全群禁言只管这个群：bob 的单聊和别的群照常。
	p.createGroup("g2", "bob")
	mustAck[protocol.SendAck](t, bob.sendText("group", "g2", "m-all-0005", "g2"))
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-all-0006", "c2c"))

	p.unmuteAll(gid)
	mustAck[protocol.SendAck](t, bob.sendText("group", gid, "m-all-0007", "after"))
}

// TC-ADM-42-02（AC-ADM-42-2）：全部在线成员（含群主、管理员、同一用户的多个连接）收到 muted，非成员收不到；关闭后收到 unmuted；convs 带 allMuted。
func TestTCADM4202GroupAllMuteEvents(t *testing.T) {
	p := newP1Env(t)
	gid, alice, carol, bob := p.muteAllGroup()
	bobPC := p.dial("bob", "pc")
	p.usersNamed("eve")
	eve := p.dial("eve", "web")

	if c := convOf(t, bob, gid); c.AllMuted {
		t.Errorf("allMuted before mute: %+v", c)
	}
	p.muteAll(gid)
	for _, c := range []*client{alice, carol, bob, bobPC} {
		var ev protocol.MutedEvent
		json.Unmarshal(c.waitEvent(protocol.EventMuted), &ev)
		if ev.Scope != protocol.MuteScopeGroupAll || ev.GroupID != gid || ev.MutedUntil != nil {
			t.Errorf("%s: event = %+v", c.uid, ev)
		}
	}
	eve.expectNoEvent(protocol.EventMuted, noFrameWindow)
	for _, c := range []*client{alice, carol, bob} {
		if got := convOf(t, c, gid); !got.AllMuted {
			t.Errorf("%s: conv has no allMuted: %+v", c.uid, got)
		}
	}
	// 帧里确实是 allMuted: true（Schema 一致性由 frames.json 的向量覆盖）。
	raw := bob.call(protocol.CmdConvs, map[string]any{})
	if !bytes.Contains(raw.Data, []byte(`"allMuted":true`)) {
		t.Errorf("convs = %s", raw.Data)
	}
	// 重复开启幂等：不重复通知。
	p.muteAll(gid)
	bob.expectNoEvent(protocol.EventMuted, noFrameWindow)

	// 离线时开启：上线后 convs 带 allMuted（断开重连）。
	dave := p.dial("bob", "ios")
	if got := convOf(t, dave, gid); !got.AllMuted {
		t.Errorf("reconnected conv = %+v", got)
	}

	p.unmuteAll(gid)
	for _, c := range []*client{alice, carol, bob, bobPC, dave} {
		var ev protocol.UnmutedEvent
		json.Unmarshal(c.waitEvent(protocol.EventUnmuted), &ev)
		if ev.Scope != protocol.MuteScopeGroupAll || ev.GroupID != gid {
			t.Errorf("%s: event = %+v", c.uid, ev)
		}
	}
	if got := convOf(t, bob, gid); got.AllMuted {
		t.Errorf("allMuted after unmute: %+v", got)
	}
	raw = bob.call(protocol.CmdConvs, map[string]any{})
	if bytes.Contains(raw.Data, []byte(`allMuted`)) {
		t.Errorf("field must be absent when off: %s", raw.Data)
	}
	// 重复解除幂等：不通知。
	p.unmuteAll(gid)
	bob.expectNoEvent(protocol.EventUnmuted, noFrameWindow)
}

// 优先级和角色：单独禁言优先；角色变化立即生效；非成员不用 4031 暴露群的状态；托管群没有群主，成员全部受限。
func TestAdmGroupAllMuteRolesAndPriority(t *testing.T) {
	p := newP1Env(t)
	gid, alice, carol, bob := p.muteAllGroup()
	p.usersNamed("eve")
	eve := p.dial("eve", "web")
	p.muteAll(gid)
	drain(alice, carol, bob)

	mustErr(t, eve.sendText("group", gid, "m-rol-0001", "x"), protocol.CodeNotGroupMember)

	// 管理员被取消后立即受限；再设回立即恢复。
	mustAck[protocol.Empty](t, alice.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": false}))
	wantGroupAll(t, "demoted", carol.sendText("group", gid, "m-rol-0002", "x"), gid)
	mustAck[protocol.Empty](t, alice.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	mustAck[protocol.SendAck](t, carol.sendText("group", gid, "m-rol-0003", "x"))

	// 转让群主：bob 变成群主后能发，alice 变成普通成员后受限。
	mustAck[protocol.Empty](t, alice.call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "bob"}))
	mustAck[protocol.SendAck](t, bob.sendText("group", gid, "m-rol-0004", "x"))
	wantGroupAll(t, "ex-owner", alice.sendText("group", gid, "m-rol-0005", "x"), gid)

	// 被单独禁言的管理员、群主仍然不能发：范围更具体的记录优先于豁免（F-ADM-41 规则 2、3）。
	p.must("/admin/mute", map[string]any{"scope": "group", "groupId": gid, "userId": "carol", "durationSeconds": 600, "reason": "spam"})
	if d := ackMutedDetails(t, carol.sendText("group", gid, "m-rol-0006", "x")); d["scope"] != "group" {
		t.Errorf("group mute details = %v", d)
	}
	p.must("/admin/mute", map[string]any{"scope": "user", "userId": "bob", "durationSeconds": 600, "reason": "spam"})
	if d := ackMutedDetails(t, bob.sendText("group", gid, "m-rol-0007", "x")); d["scope"] != "user" {
		t.Errorf("user mute details = %v", d)
	}

	// 托管群：没有群主和管理员，成员全部受限。
	p.createGroup("g-man", "alice")
	p.muteAll("g-man")
	wantGroupAll(t, "managed", alice.sendText("group", "g-man", "m-rol-0008", "x"), "g-man")
}

// 全群禁言期间入群的成员：convAdded 带 allMuted，发言受限。
func TestAdmGroupAllMuteNewMember(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("u1", "u2")
	p.createGroup("g1", "u1")
	p.muteAll("g1")
	u2 := p.dial("u2", "web")
	code, b := p.adm(http.MethodPost, "/admin/groups/g1/members", map[string]any{"members": []string{"u2"}})
	if code != http.StatusOK {
		t.Fatalf("add member: %d %s", code, b)
	}
	var ev protocol.ConvAddedEvent
	json.Unmarshal(u2.waitEvent(protocol.EventConvAdded), &ev)
	if !ev.Conv.AllMuted {
		t.Errorf("convAdded = %+v", ev.Conv)
	}
	wantGroupAll(t, "new member", u2.sendText("group", "g1", "m-new-0001", "x"), "g1")
}

// 解散时清除全群禁言；解散后再查群不存在。
func TestAdmGroupAllMuteClearedOnDissolve(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("u1")
	p.createGroup("g1", "u1")
	u1 := p.dial("u1", "web")
	p.muteAll("g1")
	code, b := p.dissolve("g1")
	if code != http.StatusOK {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	mustErr(t, u1.sendText("group", "g1", "m-dis-0001", "x"), protocol.CodeGroupNotFound)
	code, b = p.call(p.mod, "/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "x"})
	if code != http.StatusNotFound {
		t.Errorf("unmute dissolved: %d %s", code, b)
	}
}

// 接口校验、scope 拒绝和审计。
func TestAdmGroupAllMuteAdminAPI(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("u1")
	p.createGroup("g1", "u1")
	for _, tc := range []struct {
		name string
		path string
		req  map[string]any
		st   int
		code int
	}{
		{"no group", "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "gx", "reason": "r"}, 404, protocol.CodeGroupNotFound},
		{"no groupId", "/admin/mute", map[string]any{"scope": "groupAll", "reason": "r"}, 400, protocol.CodeInvalidID},
		{"with userId", "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "userId": "u1", "reason": "r"}, 400, protocol.CodeBadRequest},
		{"with duration", "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "durationSeconds": 600, "reason": "r"}, 400, protocol.CodeBadRequest},
		{"no reason", "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1"}, 400, protocol.CodeBadRequest},
		{"unmute no group", "/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "gx", "reason": "r"}, 404, protocol.CodeGroupNotFound},
		{"unmute with userId", "/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "g1", "userId": "u1", "reason": "r"}, 400, protocol.CodeBadRequest},
		{"unmute with duration", "/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "g1", "durationSeconds": nil, "reason": "r"}, 400, protocol.CodeBadRequest},
	} {
		code, b := p.call(p.mod, tc.path, tc.req)
		expectCode(t, tc.name, code, b, tc.st, tc.code)
	}
	// 只读调用方没有 mute.write。
	code, b := p.call(p.stats, "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "r"})
	expectCode(t, "mute without scope", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.call(p.stats, "/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "r"})
	expectCode(t, "unmute without scope", code, b, http.StatusForbidden, protocol.CodeForbidden)

	code, b = p.call(p.mod, "/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "durationSeconds": nil, "reason": "整顿"})
	if code != http.StatusOK || !bytes.Contains(b, []byte(`"mutedUntil":null`)) {
		t.Errorf("mute: %d %s", code, b)
	}
	var mute []auditEntry
	for _, e := range p.auditOf(p.mod) {
		if e.Route == "POST /admin/mute" && strings.Contains(auditText(e), "整顿") {
			mute = append(mute, e)
		}
	}
	if len(mute) != 2 { // intent、result
		t.Fatalf("audit = %+v", mute)
	}
	if !strings.Contains(auditText(mute[0]), `groupAll`) || mute[0].Targets.Count != 1 || mute[0].Targets.IDs[0] != "g1" {
		t.Errorf("audit entry = %s", auditText(mute[0]))
	}
}

// BE-244（AC-ADM-42-3）：GET /admin/groups/{id} 返回 allMuted，开、关之后立刻反映真实状态，只读 group.read 就够。
func TestTCADM4203GroupDetailAllMuted(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("u1")
	p.createGroup("g1", "u1")
	p.createGroup("g2", "u1")
	allMuted := func(gid string) bool {
		t.Helper()
		code, b := p.getAs(p.reads, "/admin/groups/"+gid)
		if code != http.StatusOK {
			t.Fatalf("get group: %d %s", code, b)
		}
		var g map[string]any
		if err := json.Unmarshal(b, &g); err != nil {
			t.Fatal(err)
		}
		v, ok := g["allMuted"].(bool)
		if !ok {
			t.Fatalf("allMuted missing: %s", b)
		}
		return v
	}
	if allMuted("g1") {
		t.Error("new group is muted")
	}
	p.muteAll("g1")
	if !allMuted("g1") {
		t.Error("allMuted = false after mute")
	}
	if allMuted("g2") {
		t.Error("another group is muted")
	}
	p.unmuteAll("g1")
	if allMuted("g1") {
		t.Error("allMuted = true after unmute")
	}
}

// 第二批开关关闭时禁言不生效，详情也不能显示"全员禁言中"。
func TestAdmGroupDetailAllMutedFeatureOff(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1")
	code, b := e.adminDo(http.MethodGet, "/admin/groups/g1", nil, adminKey)
	if code != http.StatusOK || !bytes.Contains(b, []byte(`"allMuted":false`)) {
		t.Errorf("get group: %d %s", code, b)
	}
}

// 双节点：在节点 1 开启，连在节点 2 的成员 1 秒内被拒；关闭后 1 秒内恢复（主题 groupAllMutes）。
func TestAdmGroupAllMuteTwoNodes(t *testing.T) {
	db := t.TempDir() + "/shared.db"
	n1 := newModEnv(t, func(c *app.Config) { c.DBPath = db })
	n2 := newModEnv(t, func(c *app.Config) { c.DBPath = db })
	n1.app.SetTestPeers([]string{n2.adminSrv.URL})
	n2.app.SetTestPeers([]string{n1.adminSrv.URL})
	n1.usersNamed("u1")
	n1.createGroup("g1", "u1")
	u1 := n2.dial("u1", "web")
	mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "m-2n-0001", "cached")) // 节点 2 缓存了"没有全群禁言"

	n1.must("/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "r"})
	deadline := time.Now().Add(time.Second)
	for i := 2; ; i++ {
		f := u1.sendText("group", "g1", fmt.Sprintf("m-2n-%04d", i), "x")
		if f.Error != nil && f.Error.Code == protocol.CodeMuted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("node 2 still allows after 1s: %+v", f.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	n1.must("/admin/unmute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "r"})
	deadline = time.Now().Add(time.Second)
	for i := 100; ; i++ {
		f := u1.sendText("group", "g1", fmt.Sprintf("m-2n-%04d", i), "x")
		if f.Error == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("node 2 still rejects after 1s: %+v", f.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TTL 兜底：广播丢失时，缓存最多 60 秒后读到新状态（ARCH 2.2 约束 2）。
func TestAdmGroupAllMuteCacheTTL(t *testing.T) {
	db := t.TempDir() + "/shared.db"
	n1 := newModEnv(t, func(c *app.Config) { c.DBPath = db })
	n2 := newModEnv(t, func(c *app.Config) { c.DBPath, c.TestInvalidationOff = db, true })
	n1.usersNamed("u1")
	n1.createGroup("g1", "u1")
	u1 := n2.dial("u1", "web")
	mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "m-ttl-0001", "cached"))
	n1.must("/admin/mute", map[string]any{"scope": "groupAll", "groupId": "g1", "reason": "r"})
	n2.advance(59 * time.Second)
	mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "m-ttl-0002", "stale"))
	n2.advance(2 * time.Second)
	if f := u1.sendText("group", "g1", "m-ttl-0003", "x"); f.Error == nil || f.Error.Code != protocol.CodeMuted {
		t.Fatalf("still allowed after TTL: %+v", f.Error)
	}
}

type statsResp struct {
	Online struct {
		Total      int            `json:"total"`
		ByPlatform map[string]int `json:"byPlatform"`
		Partial    bool           `json:"partial"`
		Scope      string         `json:"scope"`
	} `json:"online"`
	NewUsersToday int64 `json:"newUsersToday"`
	MessagesToday int64 `json:"messagesToday"`
	GroupsTotal   int64 `json:"groupsTotal"`
	DayStart      int64 `json:"dayStart"`
	ServerTime    int64 `json:"serverTime"`
}

func (p *p1Env) statsNow() statsResp {
	p.t.Helper()
	code, b := p.getAs(p.stats, "/admin/stats")
	if code != http.StatusOK {
		p.t.Fatalf("stats: %d %s", code, b)
	}
	var s statsResp
	if err := json.Unmarshal(b, &s); err != nil {
		p.t.Fatal(err)
	}
	return s
}

// TC-ADM-70-01（AC-ADM-70-1 的服务端部分）：3 个 web、2 个 pc 连接在线，stats 返回在线 5（web 3、pc 2）；连接断开后减少。
func TestTCADM7001StatsOnline(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("a", "b", "c", "d", "e")
	p.dial("a", "web")
	p.dial("b", "web")
	p.dial("c", "web")
	p.dial("d", "pc")
	pc := p.dial("e", "pc")
	s := p.statsNow()
	if s.Online.Total != 5 || s.Online.ByPlatform["web"] != 3 || s.Online.ByPlatform["pc"] != 2 || len(s.Online.ByPlatform) != 2 {
		t.Errorf("online = %+v", s.Online)
	}
	if !s.Online.Partial || s.Online.Scope != "localNode" {
		t.Errorf("online scope = %+v", s.Online)
	}
	if s.ServerTime != p.now {
		t.Errorf("serverTime = %d, want %d", s.ServerTime, p.now)
	}
	pc.ws.CloseNow()
	deadline := time.Now().Add(2 * time.Second)
	for {
		s = p.statsNow()
		if s.Online.Total == 4 && s.Online.ByPlatform["pc"] == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("online after close = %+v", s.Online)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// BE-241：stats.read 之外都 403，旧密钥 403；只读接口不写审计。响应字段完整。
func TestAdmStatsScopeAndNoAudit(t *testing.T) {
	p := newP1Env(t)
	code, b := p.getAs(p.other, "/admin/stats")
	expectCode(t, "without scope", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.adminDo(http.MethodGet, "/admin/stats", nil, adminKey)
	expectCode(t, "legacy key", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.getAs(p.stats, "/admin/stats")
	if code != http.StatusOK {
		t.Fatalf("stats: %d %s", code, b)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw)
	for _, k := range []string{"online", "newUsersToday", "messagesToday", "groupsTotal", "dayStart", "serverTime"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing %s in %s", k, b)
		}
	}
	for _, e := range p.auditOf(p.stats) {
		t.Errorf("stats wrote audit: %s", auditText(e))
	}
}

// BE-241：今日消息数按 Asia/Shanghai 自然日。UTC 16:00 是上海零点，前后 1 毫秒的两条消息分属两天（UTC 下是同一天）。
func TestAdmStatsShanghaiBoundaryMessages(t *testing.T) {
	p := newP1Env(t)
	midnight := time.Date(2027, 1, 15, 16, 0, 0, 0, time.UTC).UnixMilli() // 上海 2027-01-16 00:00
	p.setClock(midnight - 1)
	p.usersNamed("u1", "u2", "u3")
	p.mutual("u1", "u2", "u3")
	u1 := p.dial("u1", "web")
	gid := createG(t, u1, "u2", "u3").GroupID // 产生一条 created 提示消息：不计入
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "m-day-0001", "last ms of day 1"))

	s := p.statsNow()
	if s.MessagesToday != 1 {
		t.Errorf("day 1: messagesToday = %d, want 1 (tip excluded)", s.MessagesToday)
	}
	if want := midnight - 86_400_000; s.DayStart != want {
		t.Errorf("day 1: dayStart = %d, want %d", s.DayStart, want)
	}
	if s.GroupsTotal != 1 {
		t.Errorf("groupsTotal = %d", s.GroupsTotal)
	}

	p.setClock(midnight)
	if s = p.statsNow(); s.MessagesToday != 0 || s.DayStart != midnight {
		t.Errorf("midnight: %+v, want 0 messages (UTC would count the previous one)", s)
	}
	mustAck[protocol.SendAck](t, u1.sendText("group", gid, "m-day-0002", "first ms of day 2"))
	mustAck[protocol.SendAck](t, u1.sendText("group", gid, "m-day-0003", "second"))

	p.setClock(midnight + 86_400_000 - 1)
	if s = p.statsNow(); s.MessagesToday != 2 {
		t.Errorf("end of day 2: messagesToday = %d, want 2", s.MessagesToday)
	}
	p.setClock(midnight + 86_400_000)
	if s = p.statsNow(); s.MessagesToday != 0 || s.DayStart != midnight+86_400_000 {
		t.Errorf("day 3: %+v", s)
	}
	// 回到前一天：当天之后写入的不算（上界）。
	p.setClock(midnight - 86_400_000 - 1)
	if s = p.statsNow(); s.MessagesToday != 0 {
		t.Errorf("before: %+v", s)
	}

	// 群总数不含已解散的。
	p.setClock(midnight)
	p.createGroup("g2")
	if s = p.statsNow(); s.GroupsTotal != 2 {
		t.Errorf("groupsTotal = %d, want 2", s.GroupsTotal)
	}
	if code, b := p.dissolve("g2"); code != http.StatusOK {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	if s = p.statsNow(); s.GroupsTotal != 1 {
		t.Errorf("groupsTotal after dissolve = %d, want 1", s.GroupsTotal)
	}
}

// BE-241：今日新增用户按 Asia/Shanghai。users.created_at 用的是真实时间（存储层没有 IM 钟），所以从用户实际的 createdAt 推算要测的时刻：
// 选一个和它在同一个 UTC 日、但不在同一个上海日的 IM 时间，只有按上海日才判定为 0。
func TestAdmStatsShanghaiBoundaryNewUsers(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("fresh")
	code, b := p.signed(p.mod, http.MethodGet, "/admin/users/fresh", nil, signOpts{ts: timeMs(p.now)})
	if code != http.StatusOK {
		t.Fatalf("get user: %d %s", code, b)
	}
	var u struct {
		CreatedAt int64 `json:"createdAt"`
	}
	json.Unmarshal(b, &u)
	created := time.UnixMilli(u.CreatedAt).UTC()
	shanghai := time.FixedZone("Asia/Shanghai", 8*3600)
	y, mo, d := created.In(shanghai).Date()
	dayStart := time.Date(y, mo, d, 0, 0, 0, 0, shanghai).UnixMilli()

	p.setClock(dayStart)
	if s := p.statsNow(); s.NewUsersToday < 1 {
		t.Errorf("start of the user's day: %+v", s)
	}
	p.setClock(dayStart + 86_400_000 - 1)
	if s := p.statsNow(); s.NewUsersToday < 1 {
		t.Errorf("end of the user's day: %+v", s)
	}
	p.setClock(dayStart + 86_400_000)
	if s := p.statsNow(); s.NewUsersToday != 0 {
		t.Errorf("next day: %+v", s)
	}
	p.setClock(dayStart - 1)
	if s := p.statsNow(); s.NewUsersToday != 0 {
		t.Errorf("previous day: %+v", s)
	}
	uy, um, ud := created.Date()
	utcDay := time.Date(uy, um, ud, 0, 0, 0, 0, time.UTC)
	boundary := utcDay.Add(16 * time.Hour).UnixMilli() // 这个 UTC 日里的上海零点
	probe := boundary
	if u.CreatedAt >= boundary {
		probe = boundary - 1
	}
	p.setClock(probe)
	if s := p.statsNow(); s.NewUsersToday != 0 {
		t.Errorf("same UTC day, other Shanghai day (created %d, now %d): %+v", u.CreatedAt, probe, s)
	}
}

// G-ADM-07（BE-243）：用户所在群。scope group.read，不写审计；不含已退出和已解散的群；分页。
func TestAdmUserGroups(t *testing.T) {
	p := newP1Env(t)
	p.usersNamed("alice", "bob", "carol")
	p.mutual("alice", "bob", "carol")
	alice := p.dial("alice", "web")
	ug := createG(t, alice, "bob", "carol").GroupID
	p.createGroup("g1", "bob")
	p.createGroup("g2", "bob", "carol")
	p.createGroup("g3", "carol") // bob 不在
	p.createGroup("gone", "bob")
	if code, b := p.dissolve("gone"); code != http.StatusOK {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	p.createGroup("left", "bob", "carol")
	bob := p.dial("bob", "web")
	mustAck[protocol.Empty](t, bob.call(protocol.CmdGroupLeave, map[string]any{"groupId": "left"}))

	type group struct {
		GroupID     string  `json:"groupId"`
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Owner       *string `json:"owner"`
		MemberCount int     `json:"memberCount"`
		Role        string  `json:"role"`
		JoinedAt    int64   `json:"joinedAt"`
	}
	type page struct {
		Groups     []group `json:"groups"`
		NextCursor string  `json:"nextCursor"`
	}
	list := func(path string) page {
		t.Helper()
		code, b := p.getAs(p.reads, path)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, code, b)
		}
		var out page
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	got := list("/admin/users/bob/groups")
	var ids []string
	for _, g := range got.Groups {
		ids = append(ids, g.GroupID)
	}
	if strings.Join(ids, ",") != "g1,g2,"+ug || got.NextCursor != "" {
		t.Fatalf("groups = %+v", got)
	}
	if g := got.Groups[2]; g.Type != "user" || g.Owner == nil || *g.Owner != "alice" || g.Role != "member" || g.MemberCount != 3 || g.JoinedAt != p.now {
		t.Errorf("client group = %+v", g)
	}
	if g := got.Groups[0]; g.Type != "managed" || g.Owner != nil || g.Role != "member" {
		t.Errorf("managed group = %+v", g)
	}
	if got := list("/admin/users/alice/groups"); len(got.Groups) != 1 || got.Groups[0].Role != "owner" {
		t.Errorf("alice = %+v", got)
	}

	// 分页：limit 只接受 20 / 50 / 100。
	p.usersNamed("many")
	for i := range 21 {
		p.createGroup(fmt.Sprintf("pg-%02d", i), "many")
	}
	p1 := list("/admin/users/many/groups?limit=20")
	if len(p1.Groups) != 20 || p1.NextCursor == "" || p1.Groups[0].GroupID != "pg-00" {
		t.Fatalf("page 1 = %d, cursor %q", len(p1.Groups), p1.NextCursor)
	}
	p2 := list("/admin/users/many/groups?limit=20&cursor=" + p1.NextCursor)
	if len(p2.Groups) != 1 || p2.Groups[0].GroupID != "pg-20" || p2.NextCursor != "" {
		t.Fatalf("page 2 = %+v", p2)
	}

	code, b := p.getAs(p.reads, "/admin/users/many/groups?limit=33")
	expectCode(t, "bad limit", code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	code, b = p.getAs(p.reads, "/admin/users/many/groups?cursor=%21%21")
	expectCode(t, "bad cursor", code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	code, b = p.getAs(p.reads, "/admin/users/nobody/groups")
	expectCode(t, "no user", code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	code, b = p.getAs(p.other, "/admin/users/bob/groups")
	expectCode(t, "without scope", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.adminDo(http.MethodGet, "/admin/users/bob/groups", nil, adminKey)
	expectCode(t, "legacy key", code, b, http.StatusForbidden, protocol.CodeForbidden)
	for _, e := range p.auditOf(p.reads) {
		t.Errorf("group.read list wrote audit: %s", auditText(e))
	}
}

// G-ADM-13（BE-243）：用户会话列表。scope msg.meta，写审计；响应没有正文和摘要；排序和分页。
func TestAdmUserConvs(t *testing.T) {
	const secret = "ZXQ-BODY-123"
	p := newP1Env(t)
	p.usersNamed("alice", "bob", "carol")
	p.createGroup("g1", "bob", "carol")
	p.createGroup("empty", "bob")
	p.createGroup("left", "bob")
	bob := p.dial("bob", "web")
	mustAck[protocol.Empty](t, bob.call(protocol.CmdGroupLeave, map[string]any{"groupId": "left"}))
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-cnv-0001", secret))
	p.advance(time.Second)
	mustAck[protocol.SendAck](t, bob.sendText("group", "g1", "m-cnv-0002", secret))
	p.advance(time.Second)

	type conv struct {
		ConvID      string `json:"convId"`
		Type        string `json:"type"`
		PeerID      string `json:"peerId"`
		GroupID     string `json:"groupId"`
		GroupName   string `json:"groupName"`
		MyRole      string `json:"myRole"`
		MemberCount int    `json:"memberCount"`
		MaxSeq      int64  `json:"maxSeq"`
		ReadSeq     int64  `json:"readSeq"`
		UpdatedAt   int64  `json:"updatedAt"`
		LastTS      int64  `json:"lastTs"`
		LastFrom    string `json:"lastFrom"`
		LastType    string `json:"lastType"`
	}
	type page struct {
		Convs      []conv `json:"convs"`
		NextCursor string `json:"nextCursor"`
	}
	query := func(body map[string]any, o signOpts) (page, []byte) {
		t.Helper()
		o.ts = timeMs(p.now)
		code, b := p.signed(p.meta, http.MethodPost, "/admin/users/convs/query", body, o)
		if code != http.StatusOK {
			t.Fatalf("query: %d %s", code, b)
		}
		var out page
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out, b
	}
	got, raw := query(map[string]any{"userId": "bob"}, signOpts{operator: "admin:cs1", requestID: "req-convs"})
	if bytes.Contains(raw, []byte(secret)) || bytes.Contains(raw, []byte(`"body"`)) || bytes.Contains(raw, []byte(`"text":`)) || bytes.Contains(raw, []byte(`lastMsg`)) {
		t.Errorf("response leaks message content: %s", raw)
	}
	var ids []string
	for _, c := range got.Convs {
		ids = append(ids, c.ConvID)
	}
	// updatedAt 倒序：g1 的消息最新，然后是单聊，最后是没有消息的群（按入群时间）。
	if strings.Join(ids, ",") != "group:g1,c2c:alice:bob,group:empty" {
		t.Fatalf("order = %v", ids)
	}
	if c := got.Convs[0]; c.Type != "group" || c.GroupID != "g1" || c.GroupName != "g1" || c.MyRole != "member" || c.MemberCount != 2 || c.MaxSeq != 1 || c.ReadSeq != 1 ||
		c.LastFrom != "bob" || c.LastType != "text" || c.LastTS != c.UpdatedAt || c.PeerID != "" {
		t.Errorf("group conv = %+v", c)
	}
	if c := got.Convs[1]; c.Type != "c2c" || c.PeerID != "alice" || c.GroupID != "" || c.MyRole != "" || c.MaxSeq != 1 || c.LastTS == 0 {
		t.Errorf("c2c conv = %+v", c)
	}
	if c := got.Convs[2]; c.LastTS != 0 || c.LastFrom != "" || c.MaxSeq != 0 {
		t.Errorf("empty conv = %+v", c)
	}

	// 审计：intent、result 两条，目标是 userId，result 带条数；没有正文。
	audit := p.auditOf(p.meta)
	if len(audit) != 2 {
		t.Fatalf("audit = %+v", audit)
	}
	result, intent := audit[0], audit[1]
	if intent.Phase != "intent" || result.Phase != "result" || result.Route != "POST /admin/users/convs/query" || result.Operator != "admin:cs1" ||
		result.RequestID != "req-convs" || result.Code != 0 || len(result.Targets.IDs) != 1 || result.Targets.IDs[0] != "bob" {
		t.Errorf("audit = %s / %s", auditText(intent), auditText(result))
	}
	if !bytes.Contains(result.Summary, []byte(`"count":3`)) || strings.Contains(auditText(result), secret) {
		t.Errorf("summary = %s", result.Summary)
	}

	// 错误：scope、旧密钥、用户不存在、参数不合法；审计写失败不执行。
	code, b := p.call(p.other, "/admin/users/convs/query", map[string]any{"userId": "bob"})
	expectCode(t, "without scope", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.adminPost("/admin/users/convs/query", map[string]any{"userId": "bob"}, adminKey)
	expectCode(t, "legacy key", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = p.call(p.meta, "/admin/users/convs/query", map[string]any{"userId": "nobody"})
	expectCode(t, "no user", code, b, http.StatusNotFound, protocol.CodeUserNotFound)
	code, b = p.call(p.meta, "/admin/users/convs/query", map[string]any{"userId": "a b"})
	expectCode(t, "bad id", code, b, http.StatusBadRequest, protocol.CodeInvalidID)
	code, b = p.call(p.meta, "/admin/users/convs/query", map[string]any{"userId": "bob", "limit": 33})
	expectCode(t, "bad limit", code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	code, b = p.call(p.meta, "/admin/users/convs/query", map[string]any{"userId": "bob", "cursor": "!!"})
	expectCode(t, "bad cursor", code, b, http.StatusBadRequest, protocol.CodeBadRequest)
	if code, b := p.adminPost("/admin/test/faults", map[string]any{"adminAuditWrite": "fail"}, adminKey); code != http.StatusOK {
		t.Fatalf("faults: %d %s", code, b)
	}
	code, b = p.call(p.meta, "/admin/users/convs/query", map[string]any{"userId": "bob"})
	expectCode(t, "audit unavailable", code, b, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable)
	p.adminPost("/admin/test/faults", map[string]any{"adminAuditWrite": ""}, adminKey)

	// 分页：21 个群，每页 20。同一时间按 convId 升序，翻页不重复不遗漏。
	p.usersNamed("many")
	for i := range 21 {
		p.createGroup(fmt.Sprintf("pg-%02d", i), "many")
	}
	first, _ := query(map[string]any{"userId": "many", "limit": 20}, signOpts{})
	if len(first.Convs) != 20 || first.NextCursor == "" || first.Convs[0].ConvID != "group:pg-00" {
		t.Fatalf("page 1 = %d, cursor %q", len(first.Convs), first.NextCursor)
	}
	second, _ := query(map[string]any{"userId": "many", "limit": 20, "cursor": first.NextCursor}, signOpts{})
	if len(second.Convs) != 1 || second.Convs[0].ConvID != "group:pg-20" || second.NextCursor != "" {
		t.Fatalf("page 2 = %+v", second)
	}
}
