package e2e

// C4 撤回（BE-472、BE-473 的服务端部分）：TC-RCL-01、02、03 里【后端】的用例。环境 E1（c4Env）。本地结果，CI 不运行。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// c2cPair：u1（web、ios 两端）和 u2 互为好友的单聊。
func (e *c4Env) c2cPair() (u1, u1b, u2 *client, convID string) {
	e.t.Helper()
	e.usersNamed("u1", "u2", "u3")
	e.mutual("u1", "u2")
	u1, u1b, u2 = e.dial("u1", "web"), e.dial("u1", "ios"), e.dial("u2", "web")
	return u1, u1b, u2, protocol.C2CConvID("u1", "u2")
}

// TC-RCL-01-01（AC-RCL-01-1 的服务端部分）、TC-RCL-03-07（AC-RCL-03-7）：60 秒内撤回，ack {}；对方和自己另一端收到带 recalled 的 msgDeleted，
// 发起的连接只收 ack；真实的事件帧、sync 应答、msg 帧都符合 Schema，撤回的消息没有 body，deleted、recalled 都是 true。
func TestTC_RCL_01_01_RecallWithin60s(t *testing.T) {
	e := newC4Env(t, nil)
	schema := frameSchema(t)
	u1, u1b, u2, conv := e.c2cPair()
	ack := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0101", "hello ZXQ-RCL-1"))
	msgFrame := u2.waitCmd(protocol.CmdMsg)
	validFrame(t, schema, "msg frame", msgFrame)
	u1b.waitCmd(protocol.CmdMsg)
	e.advance(60 * time.Second)
	f := recallCall(u1, conv, ack.MsgID)
	if f.Error != nil || string(f.Data) != "{}" {
		t.Fatalf("recall ack = %s %+v", f.Data, f.Error)
	}
	validFrame(t, schema, "recall ack", f)
	for _, c := range []*client{u2, u1b} {
		ev, raw := waitMsgDeleted(t, c)
		if ev.ConvID != conv || len(ev.MsgIDs) != 1 || ev.MsgIDs[0] != ack.MsgID || ev.Seqs[0] != ack.Seq || ev.Recalled == nil || ev.Recalled.By != "u1" || ev.Recalled.At != e.now {
			t.Errorf("%s: event %s", c.uid, raw)
		}
		validFrame(t, schema, "msgDeleted event", map[string]any{"cmd": "event", "data": json.RawMessage(raw)})
	}
	expectNoMsgDeleted(t, u1, func() { mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-r0102", "sentinel")) })
	raw := syncRaw(t, u2, conv)
	validFrame(t, schema, "sync ack", map[string]any{"id": "r1", "cmd": "ack", "data": raw})
	if bytes.Contains(raw, []byte("ZXQ-RCL-1")) {
		t.Errorf("sync still has the original text: %s", raw)
	}
	m := msgBySeq(t, syncConv(t, u2, conv).Messages, ack.Seq)
	if !m.Deleted || !m.Recalled || m.RecalledBy != "u1" || m.RecalledAt != e.now || m.Body.Text != "" || m.Seq != ack.Seq {
		t.Errorf("recalled message in sync: %+v", m)
	}
}

// TC-RCL-01-02（AC-RCL-01-2）：ts 后 119 秒、恰好 120 秒成功，121 秒 4097（details.windowSeconds=120），消息不变。
func TestTC_RCL_01_02_WindowBoundary(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	var acks []protocol.SendAck
	for i := range 3 {
		acks = append(acks, mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", fmt.Sprintf("cmid-r0201-%d", i), fmt.Sprintf("m%d", i))))
	}
	t0 := e.now
	e.setClock(t0 + 119_000)
	mustRecall(t, u1, conv, acks[0].MsgID)
	e.setClock(t0 + 120_000)
	mustRecall(t, u1, conv, acks[1].MsgID)
	e.setClock(t0 + 121_000)
	f := recallCall(u1, conv, acks[2].MsgID)
	mustErr(t, f, protocol.CodeRecallWindowExpired)
	if w, _ := f.Error.Details["windowSeconds"].(float64); w != 120 {
		t.Errorf("details = %+v", f.Error.Details)
	}
	if m := msgBySeq(t, syncConv(t, u2, conv).Messages, acks[2].Seq); m.Deleted || m.Recalled || m.Body.Text != "m2" {
		t.Errorf("message changed after 4097: %+v", m)
	}
}

// TC-RCL-01-03（AC-RCL-01-3）：u2 撤回 u1 的单聊消息 4030；非参与方 4032；msgId 属于其他会话 4044（和不存在的相同）；tip 4000。
// 每种情况消息都不变、没有事件（哨兵法）。
func TestTC_RCL_01_03_Errors(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	e.mutual("u1", "u3")
	u3 := e.dial("u3", "web")
	m1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0301", "one"))
	x := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u3", "cmid-r0302", "other conv"))
	u2.drain()
	mustErr(t, recallCall(u2, conv, m1.MsgID), protocol.CodeForbidden)
	mustErr(t, recallCall(u3, conv, m1.MsgID), protocol.CodeNotConvParticipant)
	other := recallCall(u1, conv, x.MsgID)
	ghost := recallCall(u1, conv, "0193a7c4-ffff-7000-8000-000000000000")
	mustErr(t, other, protocol.CodeMessageNotFound)
	mustErr(t, ghost, protocol.CodeMessageNotFound)
	if a, b := errBody(other), errBody(ghost); a != b {
		t.Errorf("other conversation %+v differs from a missing message %+v", other.Error, ghost.Error)
	}
	gid, cs := e.selfGroup("o", nil, "u9")
	tip := tips(t, cs["o"], gid)
	mustErr(t, recallCall(cs["o"], "group:"+gid, tip[0].MsgID), protocol.CodeBadRequest)
	expectNoMsgDeleted(t, u2, func() { mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0303", "sentinel")) })
	if m := msgBySeq(t, syncConv(t, u2, conv).Messages, m1.Seq); m.Deleted || m.Body.Text != "one" {
		t.Errorf("message changed: %+v", m)
	}
}

// TC-RCL-01-04（AC-RCL-01-4）：重复撤回成功，不再发事件。
func TestTC_RCL_01_04_Idempotent(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0401", "one"))
	mustRecall(t, u1, conv, m1.MsgID)
	waitMsgDeleted(t, u2)
	mustRecall(t, u1, conv, m1.MsgID)
	expectNoMsgDeleted(t, u2, func() { mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0402", "sentinel")) })
}

// TC-RCL-01-05（AC-RCL-01-5）：被禁言、被对方拉黑仍能撤回自己 60 秒前的消息，其他参与者照样收到 msgDeleted。
func TestTC_RCL_01_05_MutedAndBlocked(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	gm := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r0501", "before mute"))
	u1, _, u2, conv := e.c2cPair()
	cm := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r0502", "before block"))
	e.must("/admin/mute", map[string]any{"scope": "group", "userId": "m", "groupId": gid, "durationSeconds": 3600, "reason": "spam"})
	mustAck[socialAck](t, u2.social(protocol.CmdBlock, map[string]any{"userId": "u1"}))
	e.advance(60 * time.Second)
	cs["o"].drain()
	u2.drain()
	mustRecall(t, cs["m"], "group:"+gid, gm.MsgID)
	mustRecall(t, u1, conv, cm.MsgID)
	if ev, _ := waitMsgDeleted(t, cs["o"]); ev.Recalled == nil || ev.Recalled.By != "m" {
		t.Errorf("owner event: %+v", ev)
	}
	if ev, _ := waitMsgDeleted(t, u2); ev.Recalled == nil || ev.Recalled.By != "u1" {
		t.Errorf("blocker event: %+v", ev)
	}
}

// TC-RCL-01-07（AC-RCL-01-7）：撤回不减未读，updatedAt 和会话顺序不变。
func TestTC_RCL_01_07_UnreadUnchanged(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	var acks []protocol.SendAck
	for i := range 3 {
		acks = append(acks, mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", fmt.Sprintf("cmid-r0701-%d", i), "x")))
	}
	e.advance(time.Second)
	e.mutual("u3", "u2")
	u3 := e.dial("u3", "web")
	mustAck[protocol.SendAck](t, u3.sendText("c2c", "u2", "cmid-r0702", "newer conversation"))
	before := mustAck[protocol.ConvsAck](t, u2.call(protocol.CmdConvs, map[string]any{}))
	e.advance(10 * time.Second)
	mustRecall(t, u1, conv, acks[1].MsgID)
	after := mustAck[protocol.ConvsAck](t, u2.call(protocol.CmdConvs, map[string]any{}))
	if len(after.Convs) != len(before.Convs) {
		t.Fatalf("convs: %+v", after)
	}
	for i := range before.Convs {
		b, a := before.Convs[i], after.Convs[i]
		if a.ConvID != b.ConvID || a.UpdatedAt != b.UpdatedAt || a.Unread != b.Unread {
			t.Errorf("conv %d changed: %+v -> %+v", i, b, a)
		}
	}
	if c := convByID(t, u2, conv); c.Unread != 3 {
		t.Errorf("unread = %d", c.Unread)
	}
}

// TC-RCL-01-09（AC-RCL-01-9）：给自己的消息、custom 可以撤回；1 分钟内第 21 次 4290；撤回的频率不和发送限流共用。
func TestTC_RCL_01_09_SelfCustomAndRate(t *testing.T) {
	e := newC4Env(t, func(c *app.Config) { c.MsgActions.RecallRatePerMin = 0 }) // 默认每分钟 20 次
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	self := protocol.C2CConvID("u1", "u1")
	s := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u1", "cmid-r0901", "note to self"))
	custom := mustAck[protocol.SendAck](t, u1.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u1", "clientMsgId": "cmid-r0902", "type": "custom", "body": map[string]any{"data": map[string]any{"k": 1}}}))
	mustRecall(t, u1, self, s.MsgID)
	mustRecall(t, u1, self, custom.MsgID)
	var ids []string
	for i := range 19 {
		ids = append(ids, mustAck[protocol.SendAck](t, u1.sendText("c2c", "u1", fmt.Sprintf("cmid-r0903-%02d", i), "x")).MsgID)
	}
	for _, id := range ids[:18] {
		mustRecall(t, u1, self, id)
	}
	// 第 21 次：前面 20 次（含重试）都计数。
	mustErr(t, recallCall(u1, self, ids[18]), protocol.CodeRateLimited)
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u1", "cmid-r0904", "sending still works"))
	e.advance(3 * time.Second) // 每 3 秒恢复 1 次
	mustRecall(t, u1, self, ids[18])
}

// TC-RCL-01-11（AC-RCL-01-11）：隔离写失败 500 / 5000，消息不变、没有事件；恢复后重试两次都成功，只发一次事件。
func TestTC_RCL_01_11_QuarantineWriteFault(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r1101", "keep me"))
	u2.waitCmd(protocol.CmdMsg)
	fault := func(on bool) {
		if code, b := e.adminPost("/admin/test/faults", map[string]any{"quarantineWrite": on}, adminKey); code != http.StatusOK {
			t.Fatalf("faults: %d %s", code, b)
		}
	}
	fault(true)
	mustErr(t, recallCall(u1, conv, m1.MsgID), protocol.CodeInternal)
	if m := msgBySeq(t, syncConv(t, u2, conv).Messages, m1.Seq); m.Deleted || m.Recalled || m.Body.Text != "keep me" {
		t.Errorf("message changed after a failed recall: %+v", m)
	}
	var n int
	e.db().QueryRow(`SELECT count(*) FROM message_quarantine WHERE msg_id = ?`, m1.MsgID).Scan(&n)
	if n != 0 {
		t.Errorf("quarantine rows = %d", n)
	}
	fault(false)
	mustRecall(t, u1, conv, m1.MsgID)
	mustRecall(t, u1, conv, m1.MsgID)
	if ev, _ := waitMsgDeleted(t, u2); ev.Recalled == nil {
		t.Errorf("event: %+v", ev)
	}
	expectNoMsgDeleted(t, u2, func() { mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r1102", "sentinel")) })
}

// TC-RCL-02-01（AC-RCL-02-1）：群主撤回管理员、管理员撤回成员成功；管理员撤回管理员、管理员撤回群主、成员撤回管理员 4030。
func TestTC_RCL_02_01_RoleMatrix(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a", "a2"}, "m")
	conv := "group:" + gid
	msg := map[string]string{}
	for _, u := range []string{"o", "a", "a2", "m"} {
		msg[u] = mustAck[protocol.SendAck](t, cs[u].sendText("group", gid, "cmid-r21-"+u, "by "+u)).MsgID
	}
	// a 的第二条：给"m 撤回 a 的"用。a 的第一条已被 o 撤回，按 F-RCL-01 规则 3 的顺序"已撤回"在"权限"之前，任何参与者再撤回都是幂等成功
	// （backend-chat 第 63 条），所以这一步用 a 没被撤回的消息。
	msg["a-2"] = mustAck[protocol.SendAck](t, cs["a"].sendText("group", gid, "cmid-r21-a-2", "by a again")).MsgID
	mustRecall(t, cs["o"], conv, msg["a"])
	mustRecall(t, cs["a"], conv, msg["m"])
	for _, tc := range []struct{ who, target string }{{"a", "a2"}, {"a", "o"}, {"m", "a-2"}} {
		if f := recallCall(cs[tc.who], conv, msg[tc.target]); f.Error == nil || f.Error.Code != protocol.CodeForbidden {
			t.Errorf("%s recalls %s's message: %+v", tc.who, tc.target, f.Error)
		}
	}
	mustRecall(t, cs["m"], conv, msg["a"]) // 已撤回的：幂等成功，不再发事件
	got := syncConv(t, cs["o"], conv).Messages
	for _, m := range got {
		if m.Type == protocol.MsgTip {
			continue
		}
		recalled := m.MsgID == msg["a"] || m.MsgID == msg["m"]
		if m.Recalled != recalled {
			t.Errorf("%s recalled=%v", m.MsgID, m.Recalled)
		}
	}
}

// TC-RCL-02-02（AC-RCL-02-2）：群主撤回成员 6 天前的成功，8 天前的 4097（windowSeconds=604800）。
func TestTC_RCL_02_02_OwnerWindow(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	day0 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r2201", "day 0"))
	e.advance(48 * time.Hour)
	day2 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r2202", "day 2"))
	e.advance(6 * 24 * time.Hour)
	mustRecall(t, cs["o"], conv, day2.MsgID)
	f := recallCall(cs["o"], conv, day0.MsgID)
	mustErr(t, f, protocol.CodeRecallWindowExpired)
	if w, _ := f.Error.Details["windowSeconds"].(float64); w != 604800 {
		t.Errorf("details = %+v", f.Error.Details)
	}
}

// TC-RCL-02-03（AC-RCL-02-3）：托管群没有群主时成员不能撤回他人的；后台指定群主之后按规则 1。
func TestTC_RCL_02_03_ManagedGroup(t *testing.T) {
	e := newC4Env(t, nil)
	e.usersNamed("p", "q")
	e.createGroup("h1", "p", "q")
	p, q := e.dial("p", "web"), e.dial("q", "web")
	mq := mustAck[protocol.SendAck](t, q.sendText("group", "h1", "cmid-r2301", "by q"))
	mustErr(t, recallCall(p, "group:h1", mq.MsgID), protocol.CodeForbidden)
	e.mustAdm(http.MethodPatch, "/admin/groups/h1", map[string]any{"owner": "p"})
	mustRecall(t, p, "group:h1", mq.MsgID)
	// 群主设了管理员 a 之后被收回：群又没有群主，管理员也不能撤回他人的（规则 3）。
	e.usersNamed("a")
	if code, b := e.adminPost("/admin/groups/h1/members", map[string]any{"members": []string{"a"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add a: %d %s", code, b)
	}
	mustAck[protocol.Empty](t, p.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": "h1", "userId": "a", "admin": true}))
	mq2 := mustAck[protocol.SendAck](t, q.sendText("group", "h1", "cmid-r2302", "by q again"))
	e.mustAdm(http.MethodPatch, "/admin/groups/h1", map[string]any{"owner": nil})
	a := e.dial("a", "web")
	mustErr(t, recallCall(a, "group:h1", mq2.MsgID), protocol.CodeForbidden)
	mustErr(t, recallCall(p, "group:h1", mq2.MsgID), protocol.CodeForbidden)
}

// TC-RCL-02-06（AC-RCL-02-6）：被撤回者已退群仍可撤回；解散的群 4043；角色刚被取消（查库，不用缓存）4030。
func TestTC_RCL_02_06_LeftDissolvedDemoted(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a"}, "m", "n")
	conv := "group:" + gid
	mm := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r2601", "by m"))
	mn := mustAck[protocol.SendAck](t, cs["n"].sendText("group", gid, "cmid-r2602", "by n"))
	mustAck[protocol.Empty](t, cs["m"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	mustRecall(t, cs["o"], conv, mm.MsgID)
	mustAck[protocol.Empty](t, cs["o"].call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "a", "admin": false}))
	mustErr(t, recallCall(cs["a"], conv, mn.MsgID), protocol.CodeForbidden)

	g2, c2 := e.selfGroup("o2", nil, "x")
	m2 := mustAck[protocol.SendAck](t, c2["o2"].sendText("group", g2, "cmid-r2603", "soon dissolved"))
	mustAck[protocol.Empty](t, c2["o2"].call(protocol.CmdGroupDissolve, map[string]any{"groupId": g2}))
	mustErr(t, recallCall(c2["o2"], "group:"+g2, m2.MsgID), protocol.CodeGroupNotFound)
}

// TC-RCL-02-07（AC-RCL-02-7，管理员一行按 backend-chat 第 57 条）：群主、管理员撤回自己 1 天前的消息成功（取更宽的时限）；普通成员 4097。
func TestTC_RCL_02_07_OwnWiderWindow(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a"}, "m")
	conv := "group:" + gid
	ids := map[string]string{}
	for _, u := range []string{"o", "a", "m"} {
		ids[u] = mustAck[protocol.SendAck](t, cs[u].sendText("group", gid, "cmid-r27-"+u, "mine")).MsgID
	}
	e.advance(24 * time.Hour)
	mustRecall(t, cs["o"], conv, ids["o"])
	mustRecall(t, cs["a"], conv, ids["a"])
	f := recallCall(cs["m"], conv, ids["m"])
	mustErr(t, f, protocol.CodeRecallWindowExpired)
	if w, _ := f.Error.Details["windowSeconds"].(float64); w != 120 {
		t.Errorf("member window = %+v", f.Error.Details)
	}
}

// TC-RCL-04-02（AC-RCL-04-2）：撤回已被后台删除的消息 4044，状态不变（deleted，没有 recalled）。
func TestTC_RCL_04_02_RecallAdminDeleted(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4201", "violation"))
	e.must("/admin/messages/delete", map[string]any{"convId": conv, "msgIds": []string{m1.MsgID}, "reason": "violation"})
	mustErr(t, recallCall(cs["m"], conv, m1.MsgID), protocol.CodeMessageNotFound)
	if m := msgBySeq(t, syncConv(t, cs["o"], conv).Messages, m1.Seq); !m.Deleted || m.Recalled || m.RecalledBy != "" {
		t.Errorf("state after 4044: %+v", m)
	}
}

// TC-RCL-03-05（AC-RCL-03-5）、TC-RCL-02-04（AC-RCL-02-4 的服务端部分）：管理员撤回成员的最后一条：lastMsg 带 deleted、recalled、recalledBy=a，
// 没有 text；modVer 增加；事件的 by 是 a。三方文案由客户端按 F-RCL-02 规则 1a 生成（FE-665、667、669）。
func TestTC_RCL_03_05_LastMsgRecalled(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a"}, "m", "n")
	conv := "group:" + gid
	last := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r3501", "ZXQ-RCL-1 last"))
	before := convByID(t, cs["n"], conv)
	for _, c := range cs {
		c.drain()
	}
	mustRecall(t, cs["a"], conv, last.MsgID)
	for _, u := range []string{"m", "n", "o"} {
		if ev, _ := waitMsgDeleted(t, cs[u]); ev.Recalled == nil || ev.Recalled.By != "a" {
			t.Errorf("%s event: %+v", u, ev)
		}
	}
	for _, u := range []string{"a", "m", "n"} {
		f := cs[u].call(protocol.CmdConvs, map[string]any{})
		if bytes.Contains(f.Data, []byte("ZXQ-RCL-1")) {
			t.Errorf("%s: convs still has the text: %s", u, f.Data)
		}
		c := convByID(t, cs[u], conv)
		lm := c.LastMsg
		if lm == nil || !lm.Deleted || !lm.Recalled || lm.RecalledBy != "a" || lm.Text != "" || lm.MsgID != last.MsgID {
			t.Errorf("%s lastMsg = %+v", u, lm)
		}
		if c.ModVer <= before.ModVer {
			t.Errorf("%s modVer %d -> %d", u, before.ModVer, c.ModVer)
		}
	}
	var lm map[string]any
	for _, x := range mustAck[struct {
		Convs []struct {
			ConvID  string         `json:"convId"`
			LastMsg map[string]any `json:"lastMsg"`
		} `json:"convs"`
	}](t, cs["n"].call(protocol.CmdConvs, map[string]any{})).Convs {
		if x.ConvID == conv {
			lm = x.LastMsg
		}
	}
	if _, has := lm["text"]; has || lm["recalled"] != true || lm["deleted"] != true {
		t.Errorf("lastMsg JSON = %v", lm)
	}
	// 新消息之后摘要恢复，不再带 recalled。
	mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r3502", "fresh"))
	if lm := convByID(t, cs["n"], conv).LastMsg; lm.Recalled || lm.Deleted || lm.Text != "fresh" {
		t.Errorf("after a new message: %+v", lm)
	}
}

// TC-RCL-03-03（AC-RCL-03-3 的服务端部分）：u2 加载后断开，这时 u1 撤回；u2 重连后 convs 的 modVer 变了，sync 返回撤回状态。
// SDK 只重拉这个会话、显示撤回文案随 FE-660 起。
func TestTC_RCL_03_03_OfflineRecall(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r3301", "ZXQ-RCL-2"))
	if m := msgBySeq(t, syncConv(t, u2, conv).Messages, m1.Seq); m.Body.Text != "ZXQ-RCL-2" {
		t.Fatalf("loaded: %+v", m)
	}
	before := convByID(t, u2, conv).ModVer
	u2.ws.CloseNow()
	mustRecall(t, u1, conv, m1.MsgID)
	u2 = e.dial("u2", "web")
	if after := convByID(t, u2, conv).ModVer; after <= before {
		t.Errorf("modVer %d -> %d", before, after)
	}
	raw := syncRaw(t, u2, conv)
	if bytes.Contains(raw, []byte("ZXQ-RCL-2")) {
		t.Errorf("sync after reconnect has the original text: %s", raw)
	}
	if m := msgBySeq(t, syncConv(t, u2, conv).Messages, m1.Seq); !m.Deleted || !m.Recalled || m.RecalledBy != "u1" {
		t.Errorf("after reconnect: %+v", m)
	}
}
