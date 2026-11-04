package e2e

// PRD-social 第 7 节（M4，BE-131～BE-138）：群角色、客户端群管理、提示消息、托管群交接。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// mutual 让 owner 与 peers 互为好友。
func (s *socEnv) mutual(owner string, peers ...string) {
	s.t.Helper()
	var fs []any
	for _, p := range peers {
		fs = append(fs, map[string]any{"owner": owner, "peer": p}, map[string]any{"owner": p, "peer": owner})
	}
	s.adminPost("/admin/test/social/seed", map[string]any{"friends": fs, "silent": true}, adminKey)
}

var reqSeq atomic.Int64 // 权限矩阵的子测试并行

func createG(t *testing.T, c *client, members ...string) protocol.GroupCreateAck {
	t.Helper()
	return mustAck[protocol.GroupCreateAck](t, c.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": fmt.Sprintf("req-%08d", reqSeq.Add(1)), "members": members}))
}

func gInfo(t *testing.T, c *client, gid string) protocol.GroupInfoAck {
	t.Helper()
	return mustAck[protocol.GroupInfoAck](t, c.call(protocol.CmdGroupInfo, map[string]any{"groupId": gid}))
}

func convOf(t *testing.T, c *client, gid string) protocol.Conversation {
	t.Helper()
	for _, x := range mustAck[protocol.ConvsAck](t, c.call(protocol.CmdConvs, map[string]any{})).Convs {
		if x.ConvID == "group:"+gid {
			return x
		}
	}
	t.Fatalf("%s: no conversation for %s", c.uid, gid)
	return protocol.Conversation{}
}

func tips(t *testing.T, c *client, gid string) []protocol.Message {
	t.Helper()
	sync := mustAck[protocol.SyncAck](t, c.call(protocol.CmdSync, map[string]any{"convId": "group:" + gid, "afterSeq": 0, "limit": 200}))
	var out []protocol.Message
	for _, m := range sync.Messages {
		if m.Type == protocol.MsgTip {
			out = append(out, m)
		}
	}
	return out
}

func newGroupEnv(t *testing.T, users ...string) (*socEnv, map[string]*client) {
	s := newSocEnv(t)
	s.usersNamed(users...)
	cs := map[string]*client{}
	for _, u := range users {
		cs[u] = s.dial(u, "web")
	}
	return s, cs
}

// TC-SOC-20-01～03、21-01（AC-SOC-20-1、20-2、20-3、21-1）
func TestTCSOC2001RolesAndInfo(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol")
	s.mutual("alice", "bob", "carol")
	pc := s.dial("alice", "pc")
	ack := createG(t, cs["alice"], "bob", "carol")
	gid := ack.GroupID
	if !strings.HasPrefix(gid, "ug-") || len(gid) != 35 || ack.Conv.MyRole != "owner" {
		t.Fatalf("ack = %+v", ack)
	}
	for _, c := range []*client{cs["bob"], cs["carol"], pc} {
		var ev protocol.ConvAddedEvent
		json.Unmarshal(c.waitEvent(protocol.EventConvAdded), &ev)
		if ev.Conv.ConvID != "group:"+gid {
			t.Fatalf("%s: convAdded %+v", c.uid, ev)
		}
	}
	info := gInfo(t, cs["alice"], gid)
	if info.Owner != "alice" || info.MyRole != "owner" || info.Managed || info.MemberCount != 3 || info.MaxMembers != 500 {
		t.Fatalf("info = %+v", info)
	}
	if got := convOf(t, cs["bob"], gid); got.MyRole != "member" || got.LastMsg == nil || got.LastMsg.Text != "alice 邀请 bob、carol 加入了群聊" || got.Unread != 1 {
		t.Fatalf("bob conv = %+v lastMsg=%+v", got, got.LastMsg)
	}
	tp := tips(t, cs["bob"], gid)
	if len(tp) != 1 || tp[0].Seq != 1 || tp[0].Body.Tip.Kind != "created" || tp[0].Body.Tip.Actor != "alice" || len(tp[0].Body.Tip.Targets) != 2 {
		t.Fatalf("tips = %+v", tp)
	}
	// 托管群
	s.createGroup("g2", "bob")
	info = gInfo(t, cs["bob"], "g2")
	if info.Owner != "" || !info.Managed || info.MyRole != "member" {
		t.Fatalf("managed info = %+v", info)
	}
	dave := s.dial("dave", "web")
	mustErr(t, dave.call(protocol.CmdGroupInfo, map[string]any{"groupId": gid}), protocol.CodeNotGroupMember)
	mustErr(t, dave.call(protocol.CmdGroupInfo, map[string]any{"groupId": "nope"}), protocol.CodeGroupNotFound)
	if g := mustAck[protocol.MyGroupsAck](t, cs["alice"].call(protocol.CmdMyGroups, map[string]any{})); g.Groups[0].MyRole != "owner" {
		t.Fatalf("myGroups = %+v", g)
	}
}

// TC-SOC-21-*（AC-SOC-21-2～21-11）
func TestTCSOC2102CreateRules(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave", "frank")
	s.mutual("alice", "bob", "carol", "frank")
	a := cs["alice"]
	create := func(req string, members ...string) frame {
		return a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": req, "members": members})
	}
	// 同一 clientReqId 只建一个群
	first := mustAck[protocol.GroupCreateAck](t, create("req-same-0001", "bob", "carol"))
	for range 2 {
		if again := mustAck[protocol.GroupCreateAck](t, create("req-same-0001", "bob", "carol")); again.GroupID != first.GroupID {
			t.Fatalf("second create = %s, first %s", again.GroupID, first.GroupID)
		}
	}
	if c := mustAck[protocol.MyGroupsAck](t, a.call(protocol.CmdMyGroups, map[string]any{})); len(c.Groups) != 1 {
		t.Fatalf("groups = %+v", c.Groups)
	}
	mustErr(t, create("req-one-00001", "bob"), protocol.CodeBadRequest)
	mustErr(t, create("req-self-0001", "alice", "bob"), protocol.CodeBadRequest)

	// 不是好友 / 拉黑了我 / 不存在：同构的 4035，带 userId，没有人收到事件
	mustAck[socialAck](t, cs["frank"].call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	cs["bob"].expectNoFrameDrain()
	time.Sleep(50 * time.Millisecond)
	cs["bob"].expectNoFrameDrain()
	bodies := map[string]string{}
	for _, u := range []string{"dave", "frank", "nobody"} {
		f := create("req-bad-"+u, "bob", u)
		mustErr(t, f, protocol.CodeNotFriend)
		if f.Error.UserID != u {
			t.Fatalf("userId = %q, want %s", f.Error.UserID, u)
		}
		bodies[u] = strings.Replace(errBody(f), u, "X", 1)
	}
	if bodies["dave"] != bodies["frank"] || bodies["dave"] != bodies["nobody"] {
		t.Fatalf("error bodies differ: %v", bodies)
	}
	cs["bob"].expectNoFrame(noFrameWindow)

	// 失败不记录：成为好友后用同一个 clientReqId 重试成功（AC-SOC-21-11）
	s.resetLimits("alice")
	mustErr(t, create("req-retry-001", "bob", "dave"), protocol.CodeNotFriend)
	s.mutual("alice", "dave")
	mustAck[protocol.GroupCreateAck](t, create("req-retry-001", "bob", "dave"))

	// 默认群名用昵称，不用备注（AC-SOC-21-7、29-9）
	mustAck[socialAck](t, a.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "老板"}))
	got := createG(t, a, "bob", "carol")
	if got.Conv.Name != "alice、bob、carol" || strings.Contains(got.Conv.LastMsg.Text, "老板") {
		t.Fatalf("name %q lastMsg %q", got.Conv.Name, got.Conv.LastMsg.Text)
	}
}

// AC-SOC-21-6、21-8、21-9、21-10
func TestTCSOC2106CreateLimits(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol")
	s.mutual("alice", "bob", "carol")
	a := cs["alice"]
	s.settings(map[string]any{"clientGroupMaxMembers": 3})
	var four []string
	for i := range 3 {
		four = append(four, fmt.Sprintf("x%d", i))
	}
	mustErr(t, a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-limit-001", "members": four}), protocol.CodeGroupMemberLimitExceeded)
	s.settings(map[string]any{"clientGroupEnabled": false})
	mustErr(t, a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-off-00001", "members": []string{"bob", "carol"}}), protocol.CodeForbidden)
	s.settings(map[string]any{"clientGroupEnabled": true, "maxJoinedGroups": 10})

	var ms []any
	for i := range 10 {
		ms = append(ms, map[string]any{"groupId": fmt.Sprintf("full%d", i), "userId": "bob"})
	}
	s.adminPost("/admin/test/social/seed", map[string]any{"groupMembers": ms}, adminKey)
	f := a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-full-0001", "members": []string{"bob", "carol"}})
	mustErr(t, f, protocol.CodeJoinedGroupLimitExceeded)
	if f.Error.UserID != "bob" {
		t.Fatalf("userId = %q", f.Error.UserID)
	}
	s.settings(map[string]any{"maxJoinedGroups": 500})
	for range 10 {
		createG(t, a, "bob", "carol")
	}
	mustErr(t, a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-quota-001", "members": []string{"bob", "carol"}}), protocol.CodeQuotaExceeded)

	code, body := s.adminPost("/admin/groups", map[string]any{"groupId": "ug-123", "name": "x"}, adminKey)
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
}

// TC-SOC-22-*（AC-SOC-22-1～22-6、29-7）
func TestTCSOC2201Invite(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave", "eve", "frank")
	s.mutual("alice", "bob", "carol")
	s.mutual("bob", "carol", "dave", "frank")
	gid := createG(t, cs["alice"], "bob", "carol").GroupID
	mustAck[socialAck](t, cs["frank"].call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	carol, bob := cs["carol"], cs["bob"]
	bobPc := s.dial("bob", "pc")
	ackI := mustAck[protocol.GroupInviteAck](t, bob.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"dave", "eve", "frank", "nobody", "carol"}}))
	if fmt.Sprint(ackI.Added) != "[dave]" || len(ackI.Failed) != 3 {
		t.Fatalf("ack = %+v", ackI)
	}
	for _, f := range ackI.Failed {
		if f.Code != protocol.CodeNotFriend {
			t.Fatalf("failed = %+v", f)
		}
	}
	var ev protocol.ConvAddedEvent
	json.Unmarshal(cs["dave"].waitEvent(protocol.EventConvAdded), &ev)
	if ev.Conv.MinSeq != 2 || ev.Conv.Unread != 1 { // created 是 seq 1，dave 从入群提示开始
		t.Fatalf("dave conv = %+v", ev.Conv)
	}
	var m protocol.Message
	for {
		m = carol.waitCmd(protocol.CmdMsg).msg(t)
		if m.Body.Tip != nil && m.Body.Tip.Kind == "joined" {
			break
		}
	}
	if m.Body.Tip.Actor != "bob" || fmt.Sprint(m.Body.Tip.Targets) != "[dave]" {
		t.Fatalf("tip = %+v", m.Body.Tip)
	}
	// 邀请人的已读推进，另一个连接收到 readUpdated（AC-SOC-29-7）
	var ru protocol.ReadUpdatedEvent
	json.Unmarshal(bobPc.waitEvent(protocol.EventReadUpdated), &ru)
	if ru.ReadSeq != m.Seq {
		t.Fatalf("readUpdated = %+v, tip seq %d", ru, m.Seq)
	}
	// 已在群里：什么都不变
	before := convOf(t, carol, gid).MaxSeq
	empty := mustAck[protocol.GroupInviteAck](t, bob.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"carol"}}))
	if len(empty.Added)+len(empty.Failed) != 0 || convOf(t, carol, gid).MaxSeq != before {
		t.Fatalf("invite existing: %+v", empty)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = fmt.Sprintf("u%d", i)
	}
	mustErr(t, bob.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": many}), protocol.CodeBadRequest)
	mustErr(t, cs["eve"].call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"dave"}}), protocol.CodeNotGroupMember)
	s.createGroup("g2", "bob")
	mustErr(t, bob.call(protocol.CmdGroupInvite, map[string]any{"groupId": "g2", "members": []string{"dave"}}), protocol.CodeForbidden)
}

// AC-SOC-22-4：加完超过上限整次失败。
func TestTCSOC2204InviteLimit(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave", "eve")
	s.mutual("alice", "bob", "carol", "dave", "eve")
	s.settings(map[string]any{"clientGroupMaxMembers": 4})
	gid := createG(t, cs["alice"], "bob", "carol").GroupID
	mustErr(t, cs["alice"].call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"dave", "eve"}}), protocol.CodeGroupMemberLimitExceeded)
	if info := gInfo(t, cs["alice"], gid); info.MemberCount != 3 {
		t.Fatalf("memberCount = %d", info.MemberCount)
	}
}

// AC-SOC-23-*、24-*、25-*、26-1～3（退群、移出、转让、管理员、解散）
func TestTCSOC2301Leave(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave")
	s.mutual("alice", "bob", "carol", "dave")
	a := cs["alice"]
	gid := createG(t, a, "bob", "carol", "dave").GroupID
	bobPc := s.dial("bob", "pc")
	seqBefore := convOf(t, a, gid).MaxSeq
	mustAck[protocol.Empty](t, cs["dave"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	mustErr(t, cs["dave"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}), protocol.CodeNotGroupMember)
	mustErr(t, cs["dave"].sendText("group", gid, "m-left-0001", "hi"), protocol.CodeNotGroupMember)
	if got := convOf(t, a, gid); got.MaxSeq != seqBefore || *got.MemberCount != 3 {
		t.Fatalf("after leave: %+v", got)
	}
	mustAck[protocol.Empty](t, cs["bob"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	var rm protocol.ConvRemovedEvent
	json.Unmarshal(bobPc.waitEvent(protocol.EventConvRemoved), &rm)
	if rm.Reason != "left" {
		t.Fatalf("reason = %q", rm.Reason)
	}
}

func TestTCSOC2302OwnerLeaves(t *testing.T) {
	t.Run("转给最早的管理员", func(t *testing.T) {
		s, cs := newGroupEnv(t, "alice", "bob", "carol")
		s.mutual("alice", "bob", "carol")
		gid := createG(t, cs["alice"], "bob", "carol").GroupID
		mustAck[protocol.Empty](t, cs["alice"].call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
		mustAck[protocol.Empty](t, cs["alice"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
		if info := gInfo(t, cs["bob"], gid); info.Owner != "carol" {
			t.Fatalf("owner = %q", info.Owner)
		}
		tp := tips(t, cs["bob"], gid)
		last := tp[len(tp)-1]
		if last.Body.Tip.Kind != "ownerChanged" || last.From != "" || fmt.Sprint(last.Body.Tip.Targets) != "[carol]" {
			t.Fatalf("tip = %+v", last)
		}
	})
	t.Run("没有管理员给最早入群的成员", func(t *testing.T) {
		s, cs := newGroupEnv(t, "alice", "bob", "dave", "eve")
		s.mutual("alice", "bob", "dave", "eve")
		gid := createG(t, cs["alice"], "eve", "dave").GroupID
		s.advance(time.Second)
		s.mutual("alice", "bob")
		mustAck[protocol.GroupInviteAck](t, cs["alice"].call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"bob"}}))
		mustAck[protocol.Empty](t, cs["alice"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
		if info := gInfo(t, cs["bob"], gid); info.Owner != "dave" { // dave、eve 同时入群，按 userId
			t.Fatalf("owner = %q", info.Owner)
		}
	})
	t.Run("只剩一人退群：解散，ID 不能复用", func(t *testing.T) {
		s, cs := newGroupEnv(t, "alice", "bob", "carol")
		s.mutual("alice", "bob", "carol")
		gid := createG(t, cs["alice"], "bob", "carol").GroupID
		for _, u := range []string{"bob", "carol"} {
			mustAck[protocol.Empty](t, cs[u].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
		}
		mustAck[protocol.Empty](t, cs["alice"].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
		mustErr(t, cs["alice"].call(protocol.CmdGroupInfo, map[string]any{"groupId": gid}), protocol.CodeGroupNotFound)
		code, body := s.adminDo(http.MethodGet, "/admin/groups/"+gid, nil, adminKey)
		assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeGroupNotFound)
		code, body = s.adminPost("/admin/groups", map[string]any{"groupId": gid, "name": "x"}, adminKey)
		assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	})
	t.Run("托管群：最后一人退群保留空群", func(t *testing.T) {
		s, cs := newGroupEnv(t, "bob", "carol")
		s.createGroup("g2", "bob")
		mustAck[protocol.Empty](t, cs["bob"].call(protocol.CmdGroupLeave, map[string]any{"groupId": "g2"}))
		code, body := s.adminDo(http.MethodGet, "/admin/groups/g2", nil, adminKey)
		if code != http.StatusOK || !strings.Contains(string(body), `"members":[]`) {
			t.Fatalf("get group: %d %s", code, body)
		}
		mustErr(t, cs["carol"].sendText("group", "g2", "m-empty-001", "hi"), protocol.CodeNotGroupMember)
	})
}

// AC-SOC-24-*、20-5
func TestTCSOC2401Kick(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave", "eve")
	s.mutual("alice", "bob", "carol", "dave", "eve")
	a := cs["alice"]
	gid := createG(t, a, "bob", "carol", "dave").GroupID
	for _, u := range []string{"carol", "dave"} {
		mustAck[protocol.Empty](t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": u, "admin": true}))
	}
	seq := convOf(t, a, gid).MaxSeq
	carol := cs["carol"]
	mustErr(t, carol.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"dave"}}), protocol.CodeForbidden)
	mustErr(t, carol.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"alice"}}), protocol.CodeForbidden)
	mustErr(t, carol.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"carol"}}), protocol.CodeBadRequest)
	got := mustAck[protocol.GroupKickAck](t, carol.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"bob", "eve"}}))
	if fmt.Sprint(got.Removed) != "[bob]" {
		t.Fatalf("removed = %v", got.Removed)
	}
	var rm protocol.ConvRemovedEvent
	json.Unmarshal(cs["bob"].waitEvent(protocol.EventConvRemoved), &rm)
	if rm.Reason != "removed" {
		t.Fatalf("reason %q", rm.Reason)
	}
	mustErr(t, cs["bob"].sendText("group", gid, "m-kick-0001", "hi"), protocol.CodeNotGroupMember)
	if convOf(t, a, gid).MaxSeq != seq {
		t.Fatal("kick produced a tip")
	}
	// 被移出的管理员失去身份（AC-SOC-24-4）
	mustAck[protocol.GroupKickAck](t, a.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"dave"}}))
	mustAck[protocol.GroupInviteAck](t, a.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"dave"}}))
	if info := gInfo(t, cs["dave"], gid); info.MyRole != "member" {
		t.Fatalf("role = %q", info.MyRole)
	}
}

// AC-SOC-25-*
func TestTCSOC2501TransferAndAdmin(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "eve")
	s.mutual("alice", "bob", "carol")
	a := cs["alice"]
	gid := createG(t, a, "bob", "carol").GroupID
	mustErr(t, a.call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "eve"}), protocol.CodeTargetNotGroupMember)
	mustErr(t, a.call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "alice"}), protocol.CodeBadRequest)
	mustErr(t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "eve", "admin": true}), protocol.CodeTargetNotGroupMember)
	mustErr(t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "alice", "admin": true}), protocol.CodeBadRequest)
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	seq := convOf(t, a, gid).MaxSeq
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	if convOf(t, a, gid).MaxSeq != seq {
		t.Fatal("repeated setAdmin produced a tip")
	}
	mustErr(t, cs["carol"].call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "bob"}), protocol.CodeForbidden)
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "carol"}))
	info := gInfo(t, a, gid)
	if info.Owner != "carol" || info.MyRole != "member" {
		t.Fatalf("after transfer %+v", info)
	}
	if gInfo(t, cs["carol"], gid).MyRole != "owner" {
		t.Fatal("carol should be owner")
	}
	mustErr(t, a.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"bob"}}), protocol.CodeForbidden)
}

// AC-SOC-25-3：第 11 个管理员。
func TestTCSOC2503AdminLimit(t *testing.T) {
	users := []string{"alice"}
	var peers []string
	for i := range 12 {
		peers = append(peers, fmt.Sprintf("m%d", i))
	}
	s, cs := newGroupEnv(t, append(users, peers...)...)
	s.mutual("alice", peers...)
	a := cs["alice"]
	gid := createG(t, a, peers...).GroupID
	for i := range 10 {
		mustAck[protocol.Empty](t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": peers[i], "admin": true}))
	}
	mustErr(t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": peers[10], "admin": true}), protocol.CodeGroupAdminLimitExceeded)
}

// AC-SOC-26-1～3：客户端解散。
func TestTCSOC2601ClientDissolve(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol", "dave", "op1")
	s.mutual("alice", "bob", "carol", "dave")
	a := cs["alice"]
	gid := createG(t, a, "bob", "carol").GroupID
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	mustErr(t, cs["carol"].call(protocol.CmdGroupDissolve, map[string]any{"groupId": gid}), protocol.CodeForbidden)
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupDissolve, map[string]any{"groupId": gid}))
	for _, c := range []*client{cs["bob"], cs["carol"]} {
		var rm protocol.ConvRemovedEvent
		json.Unmarshal(c.waitEvent(protocol.EventConvRemoved), &rm)
		if rm.Reason != "dissolved" {
			t.Fatalf("%s reason %q", c.uid, rm.Reason)
		}
	}
	mustErr(t, cs["bob"].sendText("group", gid, "m-dis-00001", "hi"), protocol.CodeGroupNotFound)
	mustErr(t, cs["bob"].call(protocol.CmdGroupInfo, map[string]any{"groupId": gid}), protocol.CodeGroupNotFound)
	if c := mustAck[protocol.ConvsAck](t, s.dial("dave", "pc").call(protocol.CmdConvs, map[string]any{})); len(c.Convs) != 0 {
		t.Fatalf("convs = %+v", c.Convs)
	}
}

// AC-SOC-27-*、29-1、29-2、29-4
func TestTCSOC2701UpdateAndTips(t *testing.T) {
	s, logs := newSocEnvWithLog(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	s.mutual("alice", "bob", "carol", "dave")
	a, bob, carol := s.dial("alice", "web"), s.dial("bob", "web"), s.dial("carol", "web")
	gid := createG(t, a, "bob", "carol").GroupID
	mustAck[protocol.GroupInviteAck](t, a.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"dave"}}))
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupTransfer, map[string]any{"groupId": gid, "to": "carol"}))
	mustAck[protocol.Empty](t, carol.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "bob", "admin": true}))

	mustErr(t, a.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "x"}), protocol.CodeForbidden)
	mustErr(t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid}), protocol.CodeBadRequest)
	mustErr(t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "  "}), protocol.CodeBadRequest)
	mustErr(t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": strings.Repeat("字", 51)}), protocol.CodeBadRequest)
	mustErr(t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "notice": strings.Repeat("字", 1001)}), protocol.CodeBadRequest)
	mustAck[protocol.Empty](t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "周末球局"}))
	notice := "ZXQ-NOTICE-321 <script>alert(1)</script>" + strings.Repeat("字", 960)
	mustAck[protocol.Empty](t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "notice": notice}))

	if got := convOf(t, a, gid); got.Name != "周末球局" {
		t.Fatalf("conv name = %q", got.Name)
	}
	info := gInfo(t, a, gid)
	if info.Notice != notice || info.NoticeBy != "bob" || info.NoticeAt != s.now {
		t.Fatalf("notice by=%q at=%d", info.NoticeBy, info.NoticeAt)
	}
	tp := tips(t, carol, gid)
	var kinds []string
	for i, m := range tp {
		kinds = append(kinds, m.Body.Tip.Kind)
		if m.Seq != int64(i+1) {
			t.Fatalf("seq gap: %+v", tp)
		}
	}
	if want := "[created joined ownerChanged adminSet renamed noticeUpdated]"; fmt.Sprint(kinds) != want {
		t.Fatalf("kinds = %v", kinds)
	}
	if tp[4].Body.Tip.Text != "周末球局" || protocol.TextLen(tp[5].Body.Tip.Text) != 100 {
		t.Fatalf("texts: %+v / %d", tp[4].Body.Tip, protocol.TextLen(tp[5].Body.Tip.Text))
	}
	if strings.Contains(logs.String(), "ZXQ-NOTICE-321") {
		t.Fatal("notice leaked into logs")
	}
	// 退群、移出不产生提示消息（AC-SOC-29-2）
	seq := convOf(t, a, gid).MaxSeq
	mustAck[protocol.Empty](t, a.call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	mustAck[protocol.GroupKickAck](t, carol.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"dave"}}))
	if convOf(t, carol, gid).MaxSeq != seq {
		t.Fatal("leave / kick produced a tip")
	}
	// 客户端不能发 tip（AC-SOC-29-4）
	f := carol.call(protocol.CmdSend, map[string]any{"convType": "group", "to": gid, "clientMsgId": "cid-000001", "type": "tip", "body": map[string]any{"text": "x"}})
	mustErr(t, f, protocol.CodeBadRequest)
	mustErr(t, carol.sendText("group", gid, "tip-abcdefgh", "x"), protocol.CodeBadRequest)
	f = carol.call(protocol.CmdSend, map[string]any{"convType": "group", "to": gid, "clientMsgId": "cid-000002", "type": "text", "body": map[string]any{"text": "x", "tip": map[string]any{"kind": "created"}}})
	mustErr(t, f, protocol.CodeBadRequest)
}

// AC-SOC-28-*
func TestTCSOC2801MembersPaging(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "x")
	var peers []string
	for i := range 200 {
		peers = append(peers, fmt.Sprintf("m%03d", i))
	}
	for i := 0; i < len(peers); i += 100 {
		s.usersNamed(peers[i : i+100]...)
	}
	s.mutual("alice", "x")
	s.mutual("alice", peers...)
	s.settings(map[string]any{"clientGroupMaxMembers": 500})
	a := cs["alice"]
	ack := mustAck[protocol.GroupCreateAck](t, a.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-paging-01", "members": peers[:199]}))
	gid := ack.GroupID
	page := func(cursor string) protocol.GroupMembersAck {
		d := map[string]any{"groupId": gid, "limit": 50}
		if cursor != "" {
			d["cursor"] = cursor
		}
		s.resetLimits("alice")
		return mustAck[protocol.GroupMembersAck](t, a.call(protocol.CmdGroupMembers, d))
	}
	p1 := page("")
	if p1.Members[0].UserID != "alice" || p1.Members[0].Role != "owner" || len(p1.Members) != 50 {
		t.Fatalf("first page: %+v", p1.Members[0])
	}
	p2 := page(p1.NextCursor)
	// m010 在第 1 页已返回；m180 排在第 4 页、尚未返回：两人退群；x 入群排在最后。
	for _, u := range []string{"m010", "m180"} {
		c := s.dial(u, "web")
		mustAck[protocol.Empty](t, c.call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	}
	s.advance(time.Second)
	mustAck[protocol.GroupInviteAck](t, a.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"x"}}))
	seen := map[string]int{}
	for _, p := range []protocol.GroupMembersAck{p1, p2} {
		for _, m := range p.Members {
			seen[m.UserID]++
		}
	}
	cur := p2.NextCursor
	var last string
	for cur != "" {
		p := page(cur)
		for _, m := range p.Members {
			seen[m.UserID]++
			last = m.UserID
		}
		cur = p.NextCursor
	}
	for u, n := range seen {
		if n != 1 {
			t.Fatalf("%s appeared %d times", u, n)
		}
	}
	if seen["m010"] != 1 || seen["m180"] != 0 || last != "x" {
		t.Fatalf("m010=%d m180=%d last=%s", seen["m010"], seen["m180"], last)
	}
	dave := s.dial("dave", "web")
	mustErr(t, dave.call(protocol.CmdGroupMembers, map[string]any{"groupId": gid}), protocol.CodeNotGroupMember)
	mustErr(t, a.call(protocol.CmdGroupMembers, map[string]any{"groupId": gid, "limit": 101}), protocol.CodeBadRequest)
	mustErr(t, a.call(protocol.CmdGroupMembers, map[string]any{"groupId": gid, "cursor": "forged.sig"}), protocol.CodeBadRequest)
}

// AC-SOC-20-10：groupChanged 合并。
func TestTCSOC2010GroupChangedMerged(t *testing.T) {
	users := []string{"alice", "watcher"}
	var peers []string
	for i := range 10 {
		peers = append(peers, fmt.Sprintf("u%d", i))
	}
	s, cs := newGroupEnv(t, append(users, peers...)...)
	s.mutual("alice", append(peers, "watcher")...)
	a := cs["alice"]
	gid := createG(t, a, append(peers, "watcher")...).GroupID
	time.Sleep(1100 * time.Millisecond)
	w := cs["watcher"]
	w.expectNoFrameDrain()
	for _, u := range peers {
		mustAck[protocol.Empty](t, cs[u].call(protocol.CmdGroupLeave, map[string]any{"groupId": gid}))
	}
	n := 0
	deadline := time.After(2100 * time.Millisecond)
loop:
	for {
		select {
		case f := <-w.in:
			var ev struct{ Type string }
			json.Unmarshal(f.Data, &ev)
			if f.Cmd == protocol.CmdEvent && ev.Type == protocol.EventGroupChanged {
				n++
			}
		case <-deadline:
			break loop
		}
	}
	if n > 2 || n == 0 {
		t.Fatalf("groupChanged %d times", n)
	}
	if info := gInfo(t, w, gid); info.MemberCount != 2 {
		t.Fatalf("memberCount = %d", info.MemberCount)
	}
}

// AC-SOC-20-8（BE-138）：权限矩阵逐格。数据从 PRD F-SOC-20 规则 2 手抄，与服务端 store.GroupCan 分开维护。
func TestTCSOC2008PermissionMatrix(t *testing.T) {
	ok, deny := true, false
	ops := []string{"send", "invite", "rename", "notice", "kick", "manage", "dissolve"}
	matrix := map[string][]bool{
		//              发言 邀请 改名  改公告 移出  管理  解散
		"owner":        {ok, ok, ok, ok, ok, ok, ok},
		"admin":        {ok, ok, ok, ok, ok, deny, deny},
		"member":       {ok, ok, deny, deny, deny, deny, deny},
		"managedOwner": {ok, ok, ok, ok, ok, ok, deny},
		"managedNone":  {ok, deny, deny, deny, deny, deny, deny},
	}
	soc := []string{"owner1", "admin1", "mem1", "mem2", "friend"}
	for role, want := range matrix {
		for i, op := range ops {
			t.Run(role+"/"+op, func(t *testing.T) {
				t.Parallel()
				s, cs := newGroupEnv(t, soc...)
				var gid, actor string
				switch role {
				case "managedOwner":
					gid, actor = "g1", "owner1"
					s.createGroup(gid, soc[:4]...)
					s.mustAdm(http.MethodPatch, "/admin/groups/"+gid, map[string]any{"owner": "owner1"})
				case "managedNone":
					gid, actor = "g1", "mem1"
					s.createGroup(gid, soc[:4]...)
				default:
					s.mutual("owner1", "admin1", "mem1", "mem2")
					gid = createG(t, cs["owner1"], "admin1", "mem1", "mem2").GroupID
					mustAck[protocol.Empty](t, cs["owner1"].call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "admin1", "admin": true}))
					actor = map[string]string{"owner": "owner1", "admin": "admin1", "member": "mem1"}[role]
				}
				s.mutual(actor, "friend")
				c := cs[actor]
				kickTarget := "mem2"
				if actor == "mem2" {
					kickTarget = "mem1"
				}
				var f frame
				switch op {
				case "send":
					f = c.sendText("group", gid, "m-matrix-01", "hi")
				case "invite":
					f = c.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"friend"}})
				case "rename":
					f = c.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "新名"})
				case "notice":
					f = c.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "notice": "公告"})
				case "kick":
					f = c.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{kickTarget}})
				case "manage":
					f = c.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": kickTarget, "admin": true})
				case "dissolve":
					f = c.call(protocol.CmdGroupDissolve, map[string]any{"groupId": gid})
				}
				if want[i] {
					if f.Error != nil {
						t.Fatalf("want ok, got %+v", f.Error)
					}
				} else {
					mustErr(t, f, protocol.CodeForbidden)
				}
			})
		}
	}
}

// AC-SOC-30-*、23-7、29-3（托管群交接）
func TestTCSOC3001ManagedHandover(t *testing.T) {
	grp := hmacCaller("grp", adminauth.ScopeGroupWrite, adminauth.ScopeSocialSettings, adminauth.ScopeGroupRead, adminauth.ScopeAuditRead)
	s2 := newSocEnvCfg(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, grp.cfg) })
	s2.usersNamed("bob", "carol", "eve", "op1", "dave")
	sign := func(m, p string, body any) (int, []byte) {
		return s2.signed(grp, m, p, body, signOpts{ts: timeMs(s2.now)})
	}
	bob, carol := s2.dial("bob", "web"), s2.dial("carol", "web")
	if code, b := sign(http.MethodPost, "/admin/groups", map[string]any{"groupId": "g3", "name": "g3", "owner": "bob", "members": []string{"bob", "carol"}}); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	if i := gInfo(t, bob, "g3"); i.MyRole != "owner" || !i.Managed {
		t.Fatalf("info %+v", i)
	}
	mustAck[protocol.Empty](t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": "g3", "name": "改名"}))
	// 旧密钥不能设 owner
	code, body := s2.adminDo(http.MethodPatch, "/admin/groups/g3", map[string]any{"owner": "carol"}, adminKey)
	assertAdminErr(t, code, body, http.StatusForbidden, protocol.CodeForbidden)
	// 非成员 404 / 4046
	code, body = sign(http.MethodPatch, "/admin/groups/g3", map[string]any{"owner": "eve"})
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeTargetNotGroupMember)
	// 收回
	if code, b := sign(http.MethodPatch, "/admin/groups/g3", map[string]any{"owner": nil}); code != http.StatusOK {
		t.Fatalf("revoke: %d %s", code, b)
	}
	if i := gInfo(t, bob, "g3"); i.Owner != "" || !i.Managed || i.MyRole != "member" {
		t.Fatalf("after revoke %+v", i)
	}
	mustErr(t, bob.call(protocol.CmdGroupUpdate, map[string]any{"groupId": "g3", "name": "x"}), protocol.CodeForbidden)
	tp := tips(t, carol, "g3")
	if last := tp[len(tp)-1]; last.Body.Tip.Kind != "ownerChanged" || last.From != "" {
		t.Fatalf("tip %+v", last)
	}
	raw := string(func() []byte {
		_, b := sign(http.MethodGet, "/admin/audit?from="+fmt.Sprint(s2.now-60000)+"&to="+fmt.Sprint(s2.now+1000), nil)
		return b
	}())
	if !strings.Contains(raw, "PATCH /admin/groups/{groupId}") {
		t.Fatalf("audit = %s", raw)
	}
	// 托管群主退群：回到无群主（AC-SOC-23-7）
	if code, b := sign(http.MethodPatch, "/admin/groups/g3", map[string]any{"owner": "carol"}); code != http.StatusOK {
		t.Fatalf("set owner: %d %s", code, b)
	}
	mustAck[protocol.Empty](t, carol.call(protocol.CmdGroupLeave, map[string]any{"groupId": "g3"}))
	if i := gInfo(t, bob, "g3"); i.Owner != "" || !i.Managed {
		t.Fatalf("after owner left %+v", i)
	}
	tp = tips(t, bob, "g3")
	if last := tp[len(tp)-1]; len(last.Body.Tip.Targets) != 0 || last.Body.Tip.Text != "群主已退出群聊" {
		t.Fatalf("tip %+v", last.Body.Tip)
	}
	// 管理接口不产生提示消息、不受好友限制（AC-SOC-29-3、30-4）
	if code, b := sign(http.MethodPost, "/admin/groups", map[string]any{"groupId": "g4", "name": "g4", "members": []string{"bob"}}); code != http.StatusCreated {
		t.Fatalf("g4: %d %s", code, b)
	}
	sign(http.MethodPost, "/admin/groups/g4/members", map[string]any{"members": []string{"dave"}})
	sign(http.MethodPatch, "/admin/groups/g4", map[string]any{"name": "新名", "historyVisible": true, "notice": "公告"})
	if m := convOf(t, bob, "g4"); m.MaxSeq != 0 {
		t.Fatalf("g4 maxSeq = %d", m.MaxSeq)
	}
	_, gb := sign(http.MethodGet, "/admin/groups/g4", nil)
	if !strings.Contains(string(gb), `"managed":true`) || !strings.Contains(string(gb), `"notice":"公告"`) {
		t.Fatalf("get group = %s", gb)
	}
}

// AC-SOC-30-5、30-6：管理接口移除群主。
func TestTCSOC3005AdminRemovesOwner(t *testing.T) {
	s, cs := newGroupEnv(t, "alice", "bob", "carol")
	s.mutual("alice", "bob", "carol")
	gid := createG(t, cs["alice"], "bob", "carol").GroupID
	mustAck[protocol.Empty](t, cs["alice"].call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": gid, "userId": "carol", "admin": true}))
	// 管理接口不受 max_members、好友限制（AC-SOC-30-4）
	s.usersNamed("stranger")
	if code, b := s.adminPost("/admin/groups/"+gid+"/members", map[string]any{"members": []string{"stranger"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add stranger: %d %s", code, b)
	}
	s.adminDo(http.MethodDelete, "/admin/groups/"+gid+"/members", map[string]any{"members": []string{"alice"}}, adminKey)
	if i := gInfo(t, cs["bob"], gid); i.Owner != "carol" {
		t.Fatalf("owner = %q", i.Owner)
	}
}

// B6 前置并发（BE-137）：在 SQLite 上运行。
func TestTCSOC2009ConcurrentInviteKick(t *testing.T) {
	var peers []string
	for i := range 30 {
		peers = append(peers, fmt.Sprintf("p%02d", i))
	}
	s, cs := newGroupEnv(t, append([]string{"alice", "a2"}, peers...)...)
	s.mutual("alice", append(peers, "a2")...)
	s.settings(map[string]any{"clientGroupMaxMembers": 500})
	a := cs["alice"]
	gid := createG(t, a, append(slices.Clone(peers[:10]), "a2")...).GroupID
	a2 := s.dial("alice", "pc")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 20 {
			s.resetLimits("alice") // 群写命令限流每秒 5、突发 20，时钟冻结时不恢复
			mustAck[protocol.GroupInviteAck](t, a.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{peers[10+i]}}))
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 10 {
			s.resetLimits("alice")
			mustAck[protocol.GroupKickAck](t, a2.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{peers[i]}}))
		}
	}()
	wg.Wait()
	a.pending, a2.pending = nil, nil
	s.resetLimits("alice")
	total, cur := 0, ""
	for {
		d := map[string]any{"groupId": gid, "limit": 100}
		if cur != "" {
			d["cursor"] = cur
		}
		p := mustAck[protocol.GroupMembersAck](t, a.call(protocol.CmdGroupMembers, d))
		total += len(p.Members)
		if cur = p.NextCursor; cur == "" {
			break
		}
	}
	if info := gInfo(t, a, gid); info.MemberCount != total || total != 22 {
		t.Fatalf("memberCount=%d total=%d", info.MemberCount, total)
	}
}
