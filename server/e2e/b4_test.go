package e2e

// B4 会话（BE-040、BE-041、BE-034、BE-042、BE-043）的端到端测试。用例名标注对应的 AC / TC。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func convsOf(t *testing.T, c *client) protocol.ConvsAck {
	t.Helper()
	return mustAck[protocol.ConvsAck](t, c.call(protocol.CmdConvs, nil))
}

func findConv(t *testing.T, ack protocol.ConvsAck, convID string) protocol.Conversation {
	t.Helper()
	for _, c := range ack.Convs {
		if c.ConvID == convID {
			return c
		}
	}
	t.Fatalf("conversation %s not in list %v", convID, ids(ack))
	return protocol.Conversation{}
}

func ids(ack protocol.ConvsAck) []string {
	out := make([]string, len(ack.Convs))
	for i, c := range ack.Convs {
		out[i] = c.ConvID
	}
	return out
}

func readOK(t *testing.T, c *client, conv string, seq int64) {
	t.Helper()
	mustAck[protocol.Empty](t, c.call(protocol.CmdRead, map[string]any{"convId": conv, "seq": seq}))
}

// waitEvent 等待下一个指定类型的 event 帧。
func (c *client) waitEvent(typ string) json.RawMessage {
	c.t.Helper()
	for {
		f := c.waitCmd(protocol.CmdEvent)
		var head struct{ Type string }
		json.Unmarshal(f.Data, &head)
		if head.Type == typ {
			return f.Data
		}
	}
}

// expectNoEvent 断言 d 内收不到指定类型的 event（其他帧放进 pending）。
func (c *client) expectNoEvent(typ string, d time.Duration) {
	c.t.Helper()
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			if f.Cmd == protocol.CmdEvent {
				var head struct{ Type string }
				json.Unmarshal(f.Data, &head)
				if head.Type == typ {
					c.t.Fatalf("%s: unexpected event %s", c.uid, f.Data)
				}
			}
			c.pending = append(c.pending, f)
		case <-deadline:
			return
		}
	}
}

func (e *env) freezeAt(ts time.Time) {
	e.t.Helper()
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": ts.UnixMilli()}, adminKey)
	e.t.Cleanup(func() { e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey) })
}

// AC-CONV-01-1（TC-CONV-01-01、03）：2 个单聊 + 2 个群（含无消息的群），排序和字段正确。
func TestConvsListing(t *testing.T) {
	e := newEnv(t)
	t0 := time.UnixMilli(1_800_000_000_000)
	e.freezeAt(t0)
	e.createGroup("g1", "alice", "bob") // T1
	e.adminPatchName("g1", "空群")
	alice, bob, carol := e.dial("alice", "web"), e.dial("bob", "web"), e.dial("carol", "web")

	e.freezeAt(t0.Add(10 * time.Millisecond))
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-to-bob-01", "to bob"))
	e.freezeAt(t0.Add(20 * time.Millisecond))
	mustAck[protocol.SendAck](t, carol.sendText(protocol.ConvC2C, "alice", "cid-to-alice1", "to alice"))
	e.freezeAt(t0.Add(30 * time.Millisecond))
	e.createGroup("g2", "alice", "carol")
	e.adminPatchName("g2", "热闹群")
	g2msg := mustAck[protocol.SendAck](t, carol.sendText(protocol.ConvGroup, "g2", "cid-g2-msg-01", "g2 msg"))

	ack := convsOf(t, alice)
	if strings.Join(ids(ack), ",") != "group:g2,c2c:alice:carol,c2c:alice:bob,group:g1" || ack.Truncated {
		t.Fatalf("order = %v truncated=%v", ids(ack), ack.Truncated)
	}
	g2 := findConv(t, ack, "group:g2")
	if g2.Type != "group" || g2.Name != "热闹群" || g2.MaxSeq != 1 || g2.Unread != 1 || g2.MinSeq != 1 ||
		g2.LastMsg == nil || g2.LastMsg.Text != "g2 msg" || g2.LastMsg.From != "carol" || g2.LastMsg.TS != g2msg.TS || g2.UpdatedAt != g2msg.TS {
		t.Errorf("g2 = %+v lastMsg=%+v", g2, g2.LastMsg)
	}
	if ac := findConv(t, ack, "c2c:alice:carol"); ac.Type != "c2c" || ac.Name != "carol" || ac.MaxSeq != 1 || ac.ReadSeq != 0 || ac.Unread != 1 {
		t.Errorf("c2c:alice:carol = %+v", ac)
	}
	if ab := findConv(t, ack, "c2c:alice:bob"); ab.Name != "bob" || ab.ReadSeq != 1 || ab.Unread != 0 {
		t.Errorf("c2c:alice:bob = %+v (发言即已读)", ab)
	}
	g1 := findConv(t, ack, "group:g1")
	if g1.MaxSeq != 0 || g1.MinSeq != 1 || g1.Unread != 0 || g1.LastMsg != nil || g1.UpdatedAt != t0.UnixMilli() {
		t.Errorf("g1 = %+v", g1)
	}
	// 每个字段都在 JSON 里（lastMsg 为 null 也要出现）。
	raw := alice.call(protocol.CmdConvs, nil).Data
	for _, field := range []string{"convId", "type", "name", "maxSeq", "readSeq", "minSeq", "unread", "lastMsg", "updatedAt"} {
		if !strings.Contains(string(raw), `"`+field+`"`) {
			t.Errorf("field %s missing in %s", field, raw)
		}
	}
	// 接收方视角（TC-CONV-01-03）：bob 的单聊 name 是 alice，lastMsg 是 alice 发的。
	if ba := findConv(t, convsOf(t, bob), "c2c:alice:bob"); ba.Name != "alice" || ba.LastMsg.From != "alice" || ba.Unread != 1 {
		t.Errorf("bob's view = %+v", ba)
	}
}

func (e *env) adminPatchName(gid, name string) {
	e.t.Helper()
	e.mustOK(e.patchGroup(gid, map[string]any{"name": name}))
}

// TC-CONV-01-02：自聊在列表里；发送失败（4042）的单聊不在列表里。
func TestConvsInclusionRules(t *testing.T) {
	e := newEnv(t)
	alice := e.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "alice", "cid-self-0001", "备忘"))
	mustErr(t, alice.sendText(protocol.ConvC2C, "ghost", "cid-ghost-001", "?"), protocol.CodeUserNotFound)
	ack := convsOf(t, alice)
	if strings.Join(ids(ack), ",") != "c2c:alice:alice" || ack.Convs[0].Unread != 0 || ack.Convs[0].Name != "alice" {
		t.Errorf("convs = %+v", ack.Convs)
	}
}

// AC-CONV-01-4（TC-CONV-01-07）：updatedAt 相同时按 convId 字节序升序。
func TestConvsTieBreakByConvID(t *testing.T) {
	e := newEnv(t)
	e.freezeAt(time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC))
	e.createGroup("gB", "alice")
	e.createGroup("ga", "alice")
	e.users("bob")
	alice := e.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-tie-00001", "x"))
	if got := strings.Join(ids(convsOf(t, alice)), ","); got != "c2c:alice:bob,group:gB,group:ga" {
		t.Errorf("order = %s", got)
	}
}

// AC-CONV-01-7（TC-CONV-01-04、10、11）：500 个全部返回；501 个返回最近的 500 个并截断；被截掉的有新消息后重新进入。
func TestConvsTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 501 conversations")
	}
	e := newEnvCfg(t, func(c *app.Config) { c.SendRate = 10000 })
	t0 := time.UnixMilli(1_800_000_000_000)
	peers := make([]string, 501)
	for i := range peers {
		peers[i] = fmt.Sprintf("p%03d", i+1)
	}
	for i := 0; i < len(peers); i += 500 {
		e.users(peers[i:min(i+500, len(peers))]...)
	}
	alice := e.dial("alice", "web")
	send := func(i int, tag string) {
		e.freezeAt(t0.Add(time.Duration(i) * time.Second))
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, peers[i], fmt.Sprintf("cid-%s-%04d", tag, i), "hi"))
	}
	for i := range 500 {
		send(i, "a")
	}
	if ack := convsOf(t, alice); len(ack.Convs) != 500 || ack.Truncated {
		t.Fatalf("500 conversations: got %d truncated=%v", len(ack.Convs), ack.Truncated)
	}
	send(500, "a")
	ack := convsOf(t, alice)
	if len(ack.Convs) != 500 || !ack.Truncated || ack.Convs[0].ConvID != "c2c:alice:p501" {
		t.Fatalf("501 conversations: got %d truncated=%v first=%s", len(ack.Convs), ack.Truncated, ack.Convs[0].ConvID)
	}
	for _, c := range ack.Convs {
		if c.ConvID == "c2c:alice:p001" {
			t.Fatal("the oldest conversation must be cut off")
		}
	}
	// p001 给 alice 发新消息：重新进入，p002 被挤出；被截掉的会话仍可以 sync。
	e.freezeAt(t0.Add(502 * time.Second))
	p001 := e.dial("p001", "web")
	mustAck[protocol.SendAck](t, p001.sendText(protocol.ConvC2C, "alice", "cid-back-0001", "back"))
	ack = convsOf(t, alice)
	if ack.Convs[0].ConvID != "c2c:alice:p001" || ack.Convs[0].Unread != 1 || !ack.Truncated {
		t.Fatalf("after p001 message: first=%+v truncated=%v", ack.Convs[0], ack.Truncated)
	}
	for _, c := range ack.Convs {
		if c.ConvID == "c2c:alice:p002" {
			t.Error("p002 should have been pushed out")
		}
	}
	if got := syncAfter(t, alice, "c2c:alice:p002", 0); len(got.Messages) != 1 {
		t.Errorf("truncated conversation sync: %d messages", len(got.Messages))
	}
}

// TC-CONV-01-12：lastMsg 摘要由服务端截断为前 100 个码点。
func TestConvsSummaryTruncated(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	alice := e.dial("alice", "web")
	long := strings.Repeat("😀", 60) + strings.Repeat("字", 60) // 120 个码点
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-long-0001", long))
	c := findConv(t, convsOf(t, alice), "c2c:alice:bob")
	if got := []rune(c.LastMsg.Text); len(got) != 100 || string(got) != string([]rune(long)[:100]) {
		t.Errorf("summary has %d code points", len(got))
	}
	// sync 里的消息正文不截断。
	if got := syncAfter(t, alice, "c2c:alice:bob", 0); got.Messages[0].Body.Text != long {
		t.Error("message body must not be truncated")
	}
}

// AC-CONV-02-1、AC-CONV-02-3（TC-CONV-02-04）：离线收到 3 条，未读 3；回 1 条后未读 0（发言即已读）。
func TestUnreadAndSpeakingMarksRead(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	alice := e.dial("alice", "web")
	for i := range 3 {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-off-%04d", i), "while you were away"))
	}
	bob := e.dial("bob", "web")
	if c := findConv(t, convsOf(t, bob), "c2c:alice:bob"); c.Unread != 3 || c.ReadSeq != 0 {
		t.Fatalf("offline messages: %+v", c)
	}
	mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", "cid-reply-001", "back"))
	if c := findConv(t, convsOf(t, bob), "c2c:alice:bob"); c.Unread != 0 || c.ReadSeq != 4 {
		t.Errorf("after replying: %+v", c)
	}
	// 已经读过一部分（有已读记录）之后再回复，同样清零。
	for i := range 2 {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-more-%04d", i), "more"))
	}
	readOK(t, bob, "c2c:alice:bob", 5)
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-more-0002", "and more"))
	mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", "cid-reply-002", "ok"))
	if c := findConv(t, convsOf(t, bob), "c2c:alice:bob"); c.Unread != 0 || c.ReadSeq != 8 {
		t.Errorf("reply after a partial read: %+v", c)
	}
	// 重新登录后仍为 0。
	if c := findConv(t, convsOf(t, e.dial("bob", "pc")), "c2c:alice:bob"); c.Unread != 0 {
		t.Errorf("after relogin: unread %d", c.Unread)
	}
}

// AC-CONV-02-5、AC-CONV-02-6（TC-CONV-02-06～08、11）：只进不退；超过 maxSeq 截断；权限和参数错误。
func TestReadRules(t *testing.T) {
	e := newEnv(t)
	e.users("bob", "carol")
	alice := e.dial("alice", "web")
	for i := range 10 {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-read-%04d", i), "x"))
	}
	bob := e.dial("bob", "web")
	readOK(t, bob, "c2c:alice:bob", 5)
	readOK(t, bob, "c2c:alice:bob", 3)
	if c := findConv(t, convsOf(t, bob), "c2c:alice:bob"); c.ReadSeq != 5 || c.Unread != 5 {
		t.Errorf("after 5 then 3: %+v", c)
	}
	readOK(t, bob, "c2c:alice:bob", 999)
	if c := findConv(t, convsOf(t, bob), "c2c:alice:bob"); c.ReadSeq != 10 || c.Unread != 0 {
		t.Errorf("after 999 with maxSeq 10: %+v", c)
	}
	mustErr(t, bob.call(protocol.CmdRead, map[string]any{"convId": "c2c:alice:bob", "seq": 0}), protocol.CodeBadRequest)
	carol := e.dial("carol", "web")
	mustErr(t, carol.call(protocol.CmdRead, map[string]any{"convId": "c2c:alice:bob", "seq": 1}), protocol.CodeNotConvParticipant)
	mustErr(t, carol.call(protocol.CmdRead, map[string]any{"convId": "group:nope", "seq": 1}), protocol.CodeGroupNotFound)
	e.createGroup("g1", "alice")
	mustErr(t, carol.call(protocol.CmdRead, map[string]any{"convId": "group:g1", "seq": 1}), protocol.CodeNotGroupMember)
}

// AC-GRP-01-6、AC-CONV-02-6（TC-GRP-01-12、TC-CONV-02-09）：新成员 minSeq=11、未读 0、lastMsg 不泄露入群前的消息；
// 未读从 joinSeq 算起，已读低于 minSeq 不影响。
func TestGroupUnreadFromJoin(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	e.users("dave")
	alice := e.dial("alice", "web")
	sendN(t, alice, "g1", "secret", 10)
	e.mustOK(e.addMembers("g1", "dave"))
	dave := e.dial("dave", "web")

	c := findConv(t, convsOf(t, dave), "group:g1")
	if c.MinSeq != 11 || c.Unread != 0 || c.MaxSeq != 10 || c.LastMsg != nil {
		t.Fatalf("new member: %+v lastMsg=%+v (must not leak history)", c, c.LastMsg)
	}
	sendN(t, alice, "g1", "new", 5)
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 5 || c.LastMsg == nil || c.LastMsg.Text != "new 4" {
		t.Errorf("after 5 new messages: %+v", c)
	}
	readOK(t, dave, "group:g1", 3)
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 5 {
		t.Errorf("read below minSeq changed unread to %d", c.Unread)
	}
	readOK(t, dave, "group:g1", 13)
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 2 {
		t.Errorf("after read 13: unread %d, want 2", c.Unread)
	}
}

// AC-GRP-01-12（TC-GRP-01-30、31）：切换 historyVisible、重新入群都不改变"未读从入群时刻算起"。
func TestGroupUnreadIgnoresHistoryVisibility(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "dave")
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1", "name": "g", "members": []string{"alice"}, "historyVisible": true}))
	alice := e.dial("alice", "web")
	sendN(t, alice, "g1", "a", 10)
	e.mustOK(e.addMembers("g1", "dave"))
	dave := e.dial("dave", "web")
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 0 || c.MinSeq != 1 || c.LastMsg == nil {
		t.Fatalf("join with history visible: %+v", c)
	}
	sendN(t, alice, "g1", "b", 2)
	e.mustOK(e.patchGroup("g1", map[string]any{"historyVisible": false}))
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 2 || c.MinSeq != 11 {
		t.Errorf("after switching to false: %+v", c)
	}
	e.mustOK(e.removeMembers("g1", "dave"))
	sendN(t, alice, "g1", "c", 3)
	e.mustOK(e.patchGroup("g1", map[string]any{"historyVisible": true}))
	e.mustOK(e.addMembers("g1", "dave"))
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 0 || c.MinSeq != 1 {
		t.Errorf("rejoin: %+v", c)
	}
	readOK(t, dave, "group:g1", 5)
	if c := findConv(t, convsOf(t, dave), "group:g1"); c.Unread != 0 {
		t.Errorf("read 5 below joinSeq 15: unread %d, want 0", c.Unread)
	}
}

// AC-GRP-03-1、AC-GRP-03-2（TC-GRP-03-01～03、06）、C-35、C-37：群成员变动事件。
func TestGroupMembershipEvents(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "bob", "carol")
	aliceWeb, alicePC := e.dial("alice", "web"), e.dial("alice", "pc")
	bob, carol := e.dial("bob", "web"), e.dial("carol", "web")

	// 建群时在线的初始成员收到 convAdded（C-35）。
	e.mustOK(e.createGroupWith(map[string]any{"groupId": "g1", "name": "新群", "members": []string{"alice", "bob"}}))
	for _, c := range []*client{aliceWeb, alicePC, bob} {
		var ev protocol.ConvAddedEvent
		json.Unmarshal(c.waitEvent(protocol.EventConvAdded), &ev)
		if ev.Conv.ConvID != "group:g1" || ev.Conv.Name != "新群" || ev.Conv.Type != "group" || ev.Conv.MinSeq != 1 {
			t.Errorf("%s convAdded = %+v", c.uid, ev.Conv)
		}
	}
	carol.expectNoEvent(protocol.EventConvAdded, 200*time.Millisecond)
	sendN(t, aliceWeb, "g1", "m", 3)

	// 被拉进群：1 秒内所有连接收到 convAdded，内容和 convs 里的这一项相同。
	start := time.Now()
	e.mustOK(e.addMembers("g1", "carol"))
	var ev protocol.ConvAddedEvent
	json.Unmarshal(carol.waitEvent(protocol.EventConvAdded), &ev)
	if time.Since(start) > time.Second {
		t.Errorf("convAdded took %v", time.Since(start))
	}
	if want := findConv(t, convsOf(t, carol), "group:g1"); !reflect.DeepEqual(ev.Conv, want) {
		t.Errorf("convAdded %+v differs from convs entry %+v", ev.Conv, want)
	}
	if ev.Conv.MaxSeq != 3 || ev.Conv.MinSeq != 4 || ev.Conv.Unread != 0 || ev.Conv.LastMsg != nil {
		t.Errorf("convAdded for a member who can't see history: %+v", ev.Conv)
	}

	// 幂等的加、删不发事件（TC-GRP-03-06）。
	e.mustOK(e.addMembers("g1", "carol"))
	carol.expectNoEvent(protocol.EventConvAdded, 200*time.Millisecond)

	// 被移出：所有连接收到 convRemoved，其他成员收不到；convs 里不再出现（C-37）。
	e.mustOK(e.removeMembers("g1", "alice"))
	for _, c := range []*client{aliceWeb, alicePC} {
		var rm protocol.ConvRemovedEvent
		json.Unmarshal(c.waitEvent(protocol.EventConvRemoved), &rm)
		if rm.ConvID != "group:g1" {
			t.Errorf("%s convRemoved = %+v", c.uid, rm)
		}
	}
	bob.expectNoEvent(protocol.EventConvRemoved, 200*time.Millisecond)
	for _, c := range convsOf(t, aliceWeb).Convs {
		if c.ConvID == "group:g1" {
			t.Error("removed member still sees the group in convs")
		}
	}
	e.mustOK(e.removeMembers("g1", "alice"))
	aliceWeb.expectNoEvent(protocol.EventConvRemoved, 200*time.Millisecond)
}

// AC-CONN-01-7（TC-CONN-01-16 服务端部分）：离线期间被拉进新群并有消息，上线后 convs 里有这个群，消息能拉到。
func TestAddedToGroupWhileOffline(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	e.createGroup("g1", "alice")
	alice := e.dial("alice", "web")
	e.mustOK(e.addMembers("g1", "bob"))
	sendN(t, alice, "g1", "hello", 2)
	bob := e.dial("bob", "web")
	c := findConv(t, convsOf(t, bob), "group:g1")
	if c.MaxSeq != 2 || c.Unread != 2 || c.LastMsg == nil {
		t.Fatalf("bob's group conversation: %+v", c)
	}
	if got := syncAfter(t, bob, "group:g1", c.MinSeq-1); len(got.Messages) != 2 {
		t.Errorf("sync from minSeq-1: %d messages", len(got.Messages))
	}
}

// AC-SYNC-02-1（TC-SYNC-02-02、04～06）：已读多端同步。
func TestReadUpdatedAcrossDevices(t *testing.T) {
	e := newEnv(t)
	e.users("alice", "bob")
	bob := e.dial("bob", "web")
	for i := range 10 {
		mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", fmt.Sprintf("cid-ru-%04d", i), "x"))
	}
	web, pc := e.dial("alice", "web"), e.dial("alice", "pc")

	readOK(t, pc, "c2c:alice:bob", 8)
	var ev protocol.ReadUpdatedEvent
	json.Unmarshal(web.waitEvent(protocol.EventReadUpdated), &ev)
	if ev.ConvID != "c2c:alice:bob" || ev.ReadSeq != 8 {
		t.Errorf("web readUpdated = %+v", ev)
	}
	pc.expectNoEvent(protocol.EventReadUpdated, 100*time.Millisecond) // 上报的那个连接不收

	readOK(t, web, "c2c:alice:bob", 5) // 回退：不前进、不推送
	pc.expectNoEvent(protocol.EventReadUpdated, 300*time.Millisecond)
	if c := findConv(t, convsOf(t, web), "c2c:alice:bob"); c.ReadSeq != 8 || c.Unread != 2 {
		t.Errorf("after web reported 5: %+v", c)
	}

	readOK(t, pc, "c2c:alice:bob", 10)
	web.waitEvent(protocol.EventReadUpdated)
	for _, seq := range []int64{10, 7, 999} { // 重复、回退、截断后不变
		readOK(t, pc, "c2c:alice:bob", seq)
	}
	web.expectNoEvent(protocol.EventReadUpdated, 500*time.Millisecond)

	// 发言即已读：在 pc 发消息，web 先收到消息，再收到 readUpdated。
	a := mustAck[protocol.SendAck](t, pc.sendText(protocol.ConvC2C, "bob", "cid-ru-reply1", "reply"))
	if m := web.waitCmd(protocol.CmdMsg).msg(t); m.Seq != a.Seq {
		t.Errorf("web got msg seq %d", m.Seq)
	}
	json.Unmarshal(web.waitEvent(protocol.EventReadUpdated), &ev)
	if ev.ReadSeq != a.Seq {
		t.Errorf("readUpdated after speaking = %+v, want seq %d", ev, a.Seq)
	}
	pc.expectNoEvent(protocol.EventReadUpdated, 100*time.Millisecond)
}

// AC-USER-02-1（BE-043 服务端部分）：单聊会话名优先用对方昵称；昵称修改后下次拉取生效（C-19）。
func TestConvNameUsesNickname(t *testing.T) {
	e := newEnv(t)
	e.importUsers(map[string]any{"userId": "bob", "nickname": "鲍勃"})
	alice := e.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-nick-0001", "hi"))
	if c := findConv(t, convsOf(t, alice), "c2c:alice:bob"); c.Name != "鲍勃" {
		t.Errorf("name = %q, want nickname", c.Name)
	}
	e.importUsers(map[string]any{"userId": "bob", "nickname": "Bob 2"})
	if c := findConv(t, convsOf(t, alice), "c2c:alice:bob"); c.Name != "Bob 2" {
		t.Errorf("name after nickname change = %q", c.Name)
	}
	if c := findConv(t, convsOf(t, e.dial("bob", "web")), "c2c:alice:bob"); c.Name != "alice" {
		t.Errorf("peer without nickname: name = %q, want userId", c.Name)
	}
}

// PRD-core V-07（BE-113 第 3 条）：会话列表的群会话带 memberCount，单聊不带；加人后再拉取更新。
func TestConvsMemberCount(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice", "bob", "carol")
	alice := e.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "cid-00000001", "hi"))
	var ack struct{ Convs []json.RawMessage }
	json.Unmarshal(alice.call(protocol.CmdConvs, nil).Data, &ack)
	byID := map[string]json.RawMessage{}
	for _, c := range ack.Convs {
		var x struct{ ConvID string }
		json.Unmarshal(c, &x)
		byID[x.ConvID] = c
	}
	if !strings.Contains(string(byID["group:g1"]), `"memberCount":3`) {
		t.Errorf("group conv = %s", byID["group:g1"])
	}
	if strings.Contains(string(byID["c2c:alice:bob"]), "memberCount") {
		t.Errorf("c2c conv must not carry memberCount: %s", byID["c2c:alice:bob"])
	}
	e.users("dave")
	e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"dave"}}, adminKey)
	code, body := e.adminDo(http.MethodGet, "/admin/groups/g1", nil, adminKey)
	if code != http.StatusOK || !strings.Contains(string(body), `"memberCount":4`) {
		t.Errorf("GET group = %d %s", code, body)
	}
}
