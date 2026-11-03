package e2e

// B3 群（BE-030～BE-033）的端到端测试。用例名标注对应的 AC / TC。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

type groupInfo struct {
	GroupID        string   `json:"groupId"`
	Name           string   `json:"name"`
	HistoryVisible bool     `json:"historyVisible"`
	Members        []string `json:"members"`
}

func (e *env) createGroupWith(body map[string]any) (int, []byte) {
	e.t.Helper()
	return e.adminPost("/admin/groups", body, adminKey)
}

func (e *env) addMembers(gid string, members ...string) (int, []byte) {
	e.t.Helper()
	return e.adminPost("/admin/groups/"+gid+"/members", map[string]any{"members": members}, adminKey)
}

func (e *env) removeMembers(gid string, members ...string) (int, []byte) {
	e.t.Helper()
	return e.adminDo(http.MethodDelete, "/admin/groups/"+gid+"/members", map[string]any{"members": members}, adminKey)
}

func (e *env) patchGroup(gid string, body map[string]any) (int, []byte) {
	e.t.Helper()
	return e.adminDo(http.MethodPatch, "/admin/groups/"+gid, body, adminKey)
}

func (e *env) getGroup(gid string) (int, groupInfo) {
	e.t.Helper()
	code, body := e.adminDo(http.MethodGet, "/admin/groups/"+gid, nil, adminKey)
	var g groupInfo
	json.Unmarshal(body, &g)
	return code, g
}

func (e *env) mustOK(code int, body []byte) {
	e.t.Helper()
	if code/100 != 2 {
		e.t.Fatalf("admin call failed: %d %s", code, body)
	}
}

// batchUsers 返回 u0001～u{n} 并分批注册（每批 500）。
func (e *env) batchUsers(from, to int) []string {
	e.t.Helper()
	var ids []string
	for i := from; i <= to; i++ {
		ids = append(ids, fmt.Sprintf("u%04d", i))
	}
	for i := 0; i < len(ids); i += 500 {
		e.users(ids[i:min(i+500, len(ids))]...)
	}
	return ids
}

// sendN 让 c 往群里发 n 条消息。
func sendN(t *testing.T, c *client, gid, prefix string, n int) {
	t.Helper()
	for i := range n {
		mustAck[protocol.SendAck](t, c.sendText(protocol.ConvGroup, gid, fmt.Sprintf("cid-%s-%04d", prefix, i), fmt.Sprintf("%s %d", prefix, i)))
	}
}

func seqs(msgs []protocol.Message) []int64 {
	out := make([]int64, len(msgs))
	for i, m := range msgs {
		out[i] = m.Seq
	}
	return out
}

func syncAfter(t *testing.T, c *client, conv string, after int64) protocol.SyncAck {
	t.Helper()
	return mustAck[protocol.SyncAck](t, c.call(protocol.CmdSync, map[string]any{"convId": conv, "afterSeq": after, "limit": 200}))
}

func syncBefore(t *testing.T, c *client, conv string, before int64, limit int) protocol.SyncAck {
	t.Helper()
	return mustAck[protocol.SyncAck](t, c.call(protocol.CmdSync, map[string]any{"convId": conv, "beforeSeq": before, "limit": limit}))
}

// AC-GRP-01-1（TC-GRP-01-01/02）：建群后初始成员都能发消息；空群建好后加入的成员也能发。
func TestCreateGroupMembersCanSend(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice", "bob")
	for _, uid := range []string{"alice", "bob"} {
		c := e.dial(uid, "web")
		mustAck[protocol.SendAck](t, c.sendText(protocol.ConvGroup, "g1", "cid-"+uid+"-0001", "hi"))
	}
	e.createGroup("empty")
	e.users("carol")
	e.mustOK(e.addMembers("empty", "carol"))
	mustAck[protocol.SendAck](t, e.dial("carol", "web").sendText(protocol.ConvGroup, "empty", "cid-carol-0001", "first"))
}

// AC-GRP-01-2（TC-GRP-01-03）：重复建群 409 / 4090，原群不变。
func TestCreateGroupConflict(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "bob")
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1", "name": "原群", "members": []string{"alice"}}))
	code, body := e.createGroupWith(map[string]any{"groupId": "g1", "name": "新群", "members": []string{"bob"}})
	assertAdminErr(t, code, body, http.StatusConflict, protocol.CodeGroupExists)
	if _, g := e.getGroup("g1"); g.Name != "原群" || strings.Join(g.Members, ",") != "alice" {
		t.Errorf("original group changed: %+v", g)
	}
}

// AC-GRP-01-3（TC-GRP-01-04～08、26）：总人数 2000、单次加成员 500、建群不受 500 限制。
func TestGroupSizeLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("imports 2001 users")
	}
	e := newEnv(t)
	all := e.batchUsers(1, 2001)

	// 建群 2001 人：4130，群未创建；2000 人成功（也证明建群不受 500 批量限制）。
	code, body := e.createGroupWith(map[string]any{"groupId": "big", "name": "大群", "members": all})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeGroupMemberLimitExceeded)
	if code, _ := e.getGroup("big"); code != http.StatusNotFound {
		t.Fatal("group must not be created on 4130")
	}
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "big", "name": "大群", "members": all[:2000]}))

	// 1999 人时加 2 人：4130，一人都不加；加 1 人到 2000 成功，再加 1 人 4130。
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1999", "name": "g", "members": all[:1999]}))
	code, body = e.addMembers("g1999", all[1999], all[2000])
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeGroupMemberLimitExceeded)
	if _, g := e.getGroup("g1999"); len(g.Members) != 1999 {
		t.Fatalf("partial add on 4130: %d members", len(g.Members))
	}
	e.mustOK(e.addMembers("g1999", all[1999]))
	code, body = e.addMembers("g1999", all[2000])
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeGroupMemberLimitExceeded)

	// 单次加 501 人：4131；500 人成功。
	e.createGroup("batch")
	code, body = e.addMembers("batch", all[:501]...)
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	e.mustOK(e.addMembers("batch", all[:500]...))
	// IMP-5：重复的 userId 去重后按 1 个计数，500 个不同 + 重复不算超限。
	e.createGroup("dups")
	e.mustOK(e.addMembers("dups", slices.Concat(all[:500], all[:2])...))
}

// AC-GRP-01-4（TC-GRP-01-09）：被移除的成员发送 / 拉取 / 已读都 4033，也收不到新消息；其他成员正常。
func TestRemovedMemberLosesAccess(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice", "bob", "carol")
	alice, bob, carol := e.dial("alice", "web"), e.dial("bob", "web"), e.dial("carol", "web")
	sendN(t, alice, "g1", "before", 3)
	for _, c := range []*client{bob, carol} {
		for range 3 {
			c.waitCmd(protocol.CmdMsg)
		}
	}

	code, body := e.removeMembers("g1", "carol")
	e.mustOK(code, body)
	if !strings.Contains(string(body), `"removed":["carol"]`) {
		t.Errorf("remove response = %s", body)
	}
	a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "g1", "cid-after-0001", "carol is gone"))
	if m := bob.waitCmd(protocol.CmdMsg).msg(t); m.Seq != a.Seq {
		t.Errorf("bob got seq %d, want %d", m.Seq, a.Seq)
	}
	carol.expectNoMsg(500 * time.Millisecond)
	mustErr(t, carol.sendText(protocol.ConvGroup, "g1", "cid-carol-late", "hello?"), protocol.CodeNotGroupMember)
	mustErr(t, carol.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 0}), protocol.CodeNotGroupMember)
	mustErr(t, carol.call(protocol.CmdRead, map[string]any{"convId": "group:g1", "seq": 3}), protocol.CodeNotGroupMember)
}

// AC-GRP-01-5（TC-GRP-01-10、11）：查询 / 修改 / 加 / 删；对不存在的群一律 4043，且不会隐式建群。
func TestGroupQueryAndUpdate(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "bob", "carol")
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1", "name": "活动群", "members": []string{"alice", "bob"}, "historyVisible": true}))
	code, g := e.getGroup("g1")
	if code != http.StatusOK || g.Name != "活动群" || !g.HistoryVisible || strings.Join(g.Members, ",") != "alice,bob" {
		t.Fatalf("get = %d %+v", code, g)
	}
	e.mustOK(e.patchGroup("g1", map[string]any{"name": "  新群名  "}))
	e.mustOK(e.addMembers("g1", "carol"))
	e.mustOK(e.removeMembers("g1", "bob"))
	if _, g := e.getGroup("g1"); g.Name != "新群名" || !g.HistoryVisible || strings.Join(g.Members, ",") != "alice,carol" {
		t.Errorf("after updates = %+v (name trimmed, historyVisible unchanged)", g)
	}

	for name, call := range map[string]func() (int, []byte){
		"get":    func() (int, []byte) { return e.adminDo(http.MethodGet, "/admin/groups/g404", nil, adminKey) },
		"add":    func() (int, []byte) { return e.addMembers("g404", "alice") },
		"remove": func() (int, []byte) { return e.removeMembers("g404", "alice") },
		"patch":  func() (int, []byte) { return e.patchGroup("g404", map[string]any{"historyVisible": true}) },
	} {
		code, body := call()
		if code != http.StatusNotFound || !strings.Contains(string(body), "4043") {
			t.Errorf("%s on missing group: %d %s", name, code, body)
		}
	}
	if code, _ := e.getGroup("g404"); code != http.StatusNotFound {
		t.Error("adding members must not create the group")
	}
	code, body := e.patchGroup("g1", map[string]any{})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
}

// AC-GRP-01-6（TC-GRP-01-12～14）：historyVisible=false，新成员只能看到入群之后的消息，两个方向都受限。
func TestHistoryHiddenFromNewMember(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	e.users("dave")
	alice := e.dial("alice", "web")
	sendN(t, alice, "g1", "old", 10)
	e.mustOK(e.addMembers("g1", "dave"))
	dave := e.dial("dave", "web")

	if got := syncAfter(t, dave, "group:g1", 0); len(got.Messages) != 0 || got.HasMore {
		t.Fatalf("dave sees history before joining: %v", seqs(got.Messages))
	}
	sendN(t, alice, "g1", "new", 3)
	if got := syncAfter(t, dave, "group:g1", 0); fmt.Sprint(seqs(got.Messages)) != "[11 12 13]" {
		t.Fatalf("afterSeq=0: %v, want [11 12 13]", seqs(got.Messages))
	}
	// 往前翻：打开会话取最新一页，再往上翻到 minSeq 时 hasMore=false。
	page := syncBefore(t, dave, "group:g1", 14, 2)
	if fmt.Sprint(seqs(page.Messages)) != "[12 13]" || !page.HasMore {
		t.Fatalf("latest page: %v hasMore=%v", seqs(page.Messages), page.HasMore)
	}
	page = syncBefore(t, dave, "group:g1", 12, 20)
	if fmt.Sprint(seqs(page.Messages)) != "[11]" || page.HasMore {
		t.Fatalf("top page: %v hasMore=%v, want [11] false", seqs(page.Messages), page.HasMore)
	}
	// 初始成员不受影响。
	if got := syncAfter(t, alice, "group:g1", 0); len(got.Messages) != 13 {
		t.Errorf("alice sees %d messages, want 13", len(got.Messages))
	}

	// TC-GRP-01-14：群里没有消息时入群，能看到之后的全部消息。
	e.createGroup("quiet", "alice")
	e.mustOK(e.addMembers("quiet", "dave"))
	sendN(t, alice, "quiet", "q", 2)
	if got := syncAfter(t, dave, "group:quiet", 0); fmt.Sprint(seqs(got.Messages)) != "[1 2]" {
		t.Errorf("joined empty group: %v", seqs(got.Messages))
	}
}

// AC-GRP-01-7、AC-GRP-01-8（TC-GRP-01-15～18）：historyVisible=true 能看全部；设置在查询时生效，可以来回切换。
func TestHistoryVisibilityToggle(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "dave")
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1", "name": "g", "members": []string{"alice"}, "historyVisible": true}))
	alice := e.dial("alice", "web")
	sendN(t, alice, "g1", "old", 10)
	e.mustOK(e.addMembers("g1", "dave"))
	dave := e.dial("dave", "web")

	if got := syncAfter(t, dave, "group:g1", 0); len(got.Messages) != 10 {
		t.Fatalf("historyVisible=true: dave sees %d, want 10", len(got.Messages))
	}
	e.mustOK(e.patchGroup("g1", map[string]any{"historyVisible": false}))
	if got := syncAfter(t, dave, "group:g1", 0); len(got.Messages) != 0 {
		t.Fatalf("after switching to false: dave still sees %v", seqs(got.Messages))
	}
	if page := syncBefore(t, dave, "group:g1", 11, 20); len(page.Messages) != 0 || page.HasMore {
		t.Fatalf("beforeSeq after switching to false: %v hasMore=%v", seqs(page.Messages), page.HasMore)
	}
	// 初始成员（joinSeq=0）切换前后都能看到全部。
	if got := syncAfter(t, alice, "group:g1", 0); len(got.Messages) != 10 {
		t.Errorf("initial member affected by the setting: %d", len(got.Messages))
	}
	e.mustOK(e.patchGroup("g1", map[string]any{"historyVisible": true}))
	if got := syncAfter(t, dave, "group:g1", 0); len(got.Messages) != 10 {
		t.Errorf("after switching back to true: %d, want 10", len(got.Messages))
	}
}

// AC-GRP-01-9（TC-GRP-01-19、20、32）：重复添加不重置 joinSeq；移除后再加入按重新加入的时刻算。
func TestRejoinResetsJoinSeq(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	e.users("dave")
	alice := e.dial("alice", "web")
	sendN(t, alice, "g1", "a", 10)
	e.mustOK(e.addMembers("g1", "dave")) // joinSeq=10
	sendN(t, alice, "g1", "b", 5)        // 11～15

	code, body := e.addMembers("g1", "dave") // 幂等
	e.mustOK(code, body)
	if !strings.Contains(string(body), `"added":[]`) {
		t.Errorf("re-adding existing member: %s, want added=[]", body)
	}
	dave := e.dial("dave", "web")
	if got := syncAfter(t, dave, "group:g1", 0); fmt.Sprint(seqs(got.Messages)) != "[11 12 13 14 15]" {
		t.Fatalf("after idempotent add: %v (joinSeq must stay 10)", seqs(got.Messages))
	}

	e.mustOK(e.removeMembers("g1", "dave"))
	sendN(t, alice, "g1", "c", 2) // 16～17，dave 不在群里
	e.mustOK(e.addMembers("g1", "dave"))
	sendN(t, alice, "g1", "d", 1) // 18
	if got := syncAfter(t, dave, "group:g1", 0); fmt.Sprint(seqs(got.Messages)) != "[18]" {
		t.Fatalf("after rejoin (false): %v, want [18]", seqs(got.Messages))
	}
	e.mustOK(e.patchGroup("g1", map[string]any{"historyVisible": true}))
	if got := syncAfter(t, dave, "group:g1", 0); len(got.Messages) != 18 {
		t.Errorf("after rejoin (true): %d, want all 18 including the time removed", len(got.Messages))
	}
}

// AC-GRP-01-10（TC-GRP-01-21、22）：成员包含不存在的用户：404 / 4042，整批不生效。
func TestGroupUnknownMembers(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	e.users("bob")
	code, body := e.addMembers("g1", "bob", "ghost")
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	if _, g := e.getGroup("g1"); strings.Join(g.Members, ",") != "alice" {
		t.Errorf("partial add: %v", g.Members)
	}
	code, body = e.createGroupWith(map[string]any{"groupId": "g2", "name": "g", "members": []string{"alice", "ghost"}})
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	if code, _ := e.getGroup("g2"); code != http.StatusNotFound {
		t.Error("group created despite unknown member")
	}
}

// AC-GRP-01-11（TC-GRP-01-23～25）：群名去空白后 1～50 个码点；群 ID 格式。
func TestGroupNameAndIDRules(t *testing.T) {
	e := newEnv(t)
	for i, bad := range []string{"", "   ", "　\t", strings.Repeat("群", 51)} {
		code, body := e.createGroupWith(map[string]any{"groupId": fmt.Sprintf("bad%d", i), "name": bad})
		assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	}
	for i, good := range []string{"a", strings.Repeat("群", 50), " " + strings.Repeat("😀", 50) + " "} {
		e.mustOK(e.createGroupWith(map[string]any{"groupId": fmt.Sprintf("ok%d", i), "name": good}))
	}
	code, body := e.patchGroup("ok0", map[string]any{"name": strings.Repeat("x", 51)})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)

	code, body = e.createGroupWith(map[string]any{"groupId": "bad id", "name": "g"})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	code, body = e.createGroupWith(map[string]any{"groupId": strings.Repeat("g", 65), "name": "g"})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	e.mustOK(e.createGroupWith(map[string]any{"groupId": strings.Repeat("g", 64), "name": "g"}))
	// IMP-6：ID 格式错误优先于群名错误。
	code, body = e.createGroupWith(map[string]any{"groupId": "bad id", "name": ""})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
}

// C-08（TC-GRP-01-27～29）：格式 → 4043 → 4131 → 4042 → 4130。
func TestGroupErrorPriority(t *testing.T) {
	if testing.Short() {
		t.Skip("imports 2001 users")
	}
	e := newEnv(t)
	code, body := e.addMembers("g404", "bad id")
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	code, body = e.patchGroup("g404", map[string]any{"name": strings.Repeat("群", 51)})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	code, body = e.addMembers("g404", "ghost")
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeGroupNotFound)

	us := e.batchUsers(1, 2001)
	e.createGroup("gBatch")
	code, body = e.addMembers("g404", slices.Concat(us[:500], []string{"ghost"})...)
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeGroupNotFound)
	code, body = e.addMembers("gBatch", slices.Concat(us[:500], []string{"ghost"})...)
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	code, body = e.addMembers("gBatch", us[0], "ghost")
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	if _, g := e.getGroup("gBatch"); len(g.Members) != 0 {
		t.Errorf("gBatch has %d members after failed adds", len(g.Members))
	}

	e.mustOK(e.createGroupWith(map[string]any{"groupId": "gBig", "name": "g", "members": us[:1999]}))
	code, body = e.addMembers("gBig", us[1999], us[2000], "ghost")
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	code, body = e.createGroupWith(map[string]any{"groupId": "gNew", "name": "g", "members": slices.Concat(us[:2001], []string{"ghost"})})
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	// 建群：已存在 4090 优先于成员不存在 4042（IMP-6）。
	code, body = e.createGroupWith(map[string]any{"groupId": "gBig", "name": "g", "members": []string{"ghost"}})
	assertAdminErr(t, code, body, http.StatusConflict, protocol.CodeGroupExists)
}

// C-36（TC-GRP-01-33）：移除不是成员的用户是幂等的。
func TestRemoveNonMemberIsIdempotent(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice", "bob")
	e.users("carol")
	for _, uid := range []string{"carol", "bob", "bob"} {
		e.mustOK(e.removeMembers("g1", uid))
	}
	if _, g := e.getGroup("g1"); strings.Join(g.Members, ",") != "alice" {
		t.Errorf("members = %v", g.Members)
	}
	mustErr(t, e.dial("carol", "web").sendText(protocol.ConvGroup, "g1", "cid-carol-0001", "x"), protocol.CodeNotGroupMember)
}

// BE-033：成员缓存在加 / 移除成员后立即失效——新成员马上能发、马上能收；被移除的马上不能发、不能收。
func TestMemberCacheInvalidation(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	e.users("bob")
	alice := e.dial("alice", "web")
	bob := e.dial("bob", "web")
	// 先发一条，让 g1 的成员进入缓存（此时只有 alice）。
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "g1", "cid-warm-0001", "warm"))
	mustErr(t, bob.sendText(protocol.ConvGroup, "g1", "cid-bob-0001", "x"), protocol.CodeNotGroupMember)

	for round := range 3 {
		e.mustOK(e.addMembers("g1", "bob"))
		mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvGroup, "g1", fmt.Sprintf("cid-bob-in-%02d", round), "in"))
		a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "g1", fmt.Sprintf("cid-alice-in-%02d", round), "hi bob"))
		if m := bob.waitCmd(protocol.CmdMsg).msg(t); m.Seq != a.Seq {
			t.Fatalf("round %d: bob got seq %d, want %d", round, m.Seq, a.Seq)
		}

		e.mustOK(e.removeMembers("g1", "bob"))
		mustErr(t, bob.sendText(protocol.ConvGroup, "g1", fmt.Sprintf("cid-bob-out-%02d", round), "out"), protocol.CodeNotGroupMember)
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "g1", fmt.Sprintf("cid-alice-out-%02d", round), "bye"))
		bob.expectNoMsg(200 * time.Millisecond)
	}

	// 建群前先发过消息（缓存了"群不存在"），建群后立即可用。
	mustErr(t, alice.sendText(protocol.ConvGroup, "later", "cid-later-0001", "x"), protocol.CodeGroupNotFound)
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "later", "name": "g", "members": []string{"alice"}}))
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "later", "cid-later-0002", "x"))
}
