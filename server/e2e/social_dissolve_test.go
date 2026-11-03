package e2e

// PRD-social F-SOC-26 的 M3 部分（BE-122）：管理接口解散群、convRemoved.reason、群 ID 不复用。

import (
	"encoding/json"
	"net/http"
	"testing"

	"tianting/server/internal/protocol"
)

func (s *socEnv) dissolve(gid string) (int, []byte) {
	s.t.Helper()
	return s.signed(s.dis, http.MethodDelete, "/admin/groups/"+gid, nil, signOpts{ts: timeMs(s.now)})
}

// TC-SOC-26-02、26-06（AC-SOC-26-2、26-6）、TC-SOC-26-05（AC-SOC-26-5）
func TestTCSOC2606AdminDissolve(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.createGroup("g2", "alice", "bob", "carol")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[protocol.SendAck](t, alice.sendText("group", "g2", "m-dis-0001", "hi"))
	bob.waitCmd(protocol.CmdMsg)
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"bob"}})); len(u.Users) != 1 {
		t.Fatal("bob should be visible through g2 before dissolving")
	}
	if code, b := s.dissolve("g2"); code != http.StatusOK {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	for _, c := range []*client{alice, bob} {
		var ev protocol.ConvRemovedEvent
		json.Unmarshal(c.waitEvent(protocol.EventConvRemoved), &ev)
		if ev.ConvID != "group:g2" || ev.Reason != "dissolved" {
			t.Fatalf("%s: %+v", c.uid, ev)
		}
	}
	mustErr(t, alice.sendText("group", "g2", "m-dis-0002", "hi"), protocol.CodeGroupNotFound)
	mustErr(t, alice.call(protocol.CmdSync, map[string]any{"convId": "group:g2", "afterSeq": 0}), protocol.CodeGroupNotFound)
	mustErr(t, alice.call(protocol.CmdRead, map[string]any{"convId": "group:g2", "seq": 1}), protocol.CodeGroupNotFound)
	if g := mustAck[protocol.MyGroupsAck](t, alice.call(protocol.CmdMyGroups, map[string]any{})); len(g.Groups) != 0 {
		t.Fatalf("myGroups = %+v", g.Groups)
	}
	if c := mustAck[protocol.ConvsAck](t, s.dial("carol", "web").call(protocol.CmdConvs, map[string]any{})); len(c.Convs) != 0 {
		t.Fatalf("offline member's convs = %+v", c.Convs)
	}
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"bob"}})); len(u.Users) != 0 {
		t.Fatal("bob still visible after dissolving")
	}
	code, b := s.adminDo(http.MethodGet, "/admin/groups/g2", nil, adminKey)
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeGroupNotFound)
	code, b = s.adminPost("/admin/groups/g2/members", map[string]any{"members": []string{"alice"}}, adminKey)
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeGroupNotFound)
	code, b = s.adminPost("/admin/groups", map[string]any{"groupId": "g2", "name": "again"}, adminKey)
	assertAdminErr(t, code, b, http.StatusConflict, protocol.CodeGroupExists)
	code, b = s.dissolve("g2")
	assertAdminErr(t, code, b, http.StatusNotFound, protocol.CodeGroupNotFound)
}

// 管理接口移出成员的 convRemoved 带 reason=removed；解散接口要 group.dissolve，旧密钥 403 / 4030。
func TestTCSOC2606RemoveReasonAndScope(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.createGroup("g1", "alice", "bob")
	bob := s.dial("bob", "web")
	s.adminDo(http.MethodDelete, "/admin/groups/g1/members", map[string]any{"members": []string{"bob"}}, adminKey)
	var ev protocol.ConvRemovedEvent
	json.Unmarshal(bob.waitEvent(protocol.EventConvRemoved), &ev)
	if ev.Reason != "removed" {
		t.Fatalf("reason = %q", ev.Reason)
	}
	code, b := s.adminDo(http.MethodDelete, "/admin/groups/g1", nil, adminKey)
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = s.signed(s.soc, http.MethodDelete, "/admin/groups/g1", nil, signOpts{ts: timeMs(s.now)})
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
}
