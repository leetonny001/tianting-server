package e2e

// PRD-social F-SOC-07（拉黑，BE-110）、F-SOC-08（私聊权限，BE-111）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func cid(prefix string, i int) string { return fmt.Sprintf("%s-%06d", prefix, i) }

// newSocEnvFastSend 把发送限流调大，让"测试时钟不动"的批量发送用例不被 C-29 限流干扰。
func newSocEnvFastSend(t *testing.T) *socEnv {
	return newSocEnvCfg(t, func(c *app.Config) { c.SendRate = 10000 })
}

// ---- F-SOC-07 拉黑 ----

// TC-SOC-07-01（AC-SOC-07-1 服务端部分）、TC-SOC-07-03（AC-SOC-07-2）
func TestTCSOC0701BlockedSend(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	mustErr(t, bob.sendText("c2c", "alice", "m-blk-001", "hi"), protocol.CodeRejectedByPeer)
	alice.expectNoMsg(noFrameWindow)
	if sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0})); len(sync.Messages) != 0 {
		t.Fatalf("blocked message saved: %+v", sync.Messages)
	}
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-blk-002", "hi")) // 拉黑者自己照常发
}

// TC-SOC-07-04（AC-SOC-07-3）
func TestTCSOC0704GroupIgnoresBlock(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.createGroup("g1", "alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	mustAck[protocol.SendAck](t, bob.sendText("group", "g1", "m-grp-001", "hi"))
	if m := alice.waitCmd(protocol.CmdMsg).msg(t); m.From != "bob" {
		t.Fatalf("alice got %+v", m)
	}
}

// TC-SOC-07-05（AC-SOC-07-4）、TC-SOC-07-06（取消拉黑不恢复）
func TestTCSOC0705BlockRejectsPending(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	bob.apply("alice", nil)
	bv := bob.ver()
	ack := mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	if len(ack.Items.Blocks) != 1 || len(ack.Items.Requests) != 1 || ack.Items.Requests[0].Status != "rejected" ||
		ack.Items.Requests[0].Ver != ack.Items.Blocks[0].Ver {
		t.Fatalf("block ack = %+v", ack)
	}
	bob.expectNoFrame(noFrameWindow)
	all, _ := bob.state()
	if r, _ := findReq(all.Requests, "alice", "out"); r.Status != "pending" || bob.ver() != bv {
		t.Fatalf("bob side changed: %+v", r)
	}
	mustAck[socialAck](t, alice.call(protocol.CmdUnblock, map[string]any{"userId": "bob"}))
	aAll, _ := alice.state()
	if r, _ := findReq(aAll.Requests, "bob", "in"); r.Status != "rejected" {
		t.Fatalf("unblock restored the request: %+v", r)
	}
}

// TC-SOC-07-07（AC-SOC-07-5 服务端部分）
func TestTCSOC0707BlockMultiDevice(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	web, pc, bob := s.dial("alice", "web"), s.dial("alice", "pc"), s.dial("bob", "web")
	mustAck[socialAck](t, web.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	pc.waitSocialChanged()
	if all, _ := pc.state(); len(all.Blocks) != 1 || all.Blocks[0].UserID != "bob" {
		t.Fatalf("pc blocks %+v", all.Blocks)
	}
	mustAck[socialAck](t, web.call(protocol.CmdUnblock, map[string]any{"userId": "bob"}))
	pc.waitSocialChanged()
	if all, _ := pc.state(); len(all.Blocks) != 0 {
		t.Fatalf("pc blocks after unblock %+v", all.Blocks)
	}
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-ublk-01", "hi"))
}

// TC-SOC-07-08（AC-SOC-07-6）
func TestTCSOC0708BlockBounds(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	var bs []any
	for i := range 1000 {
		bs = append(bs, map[string]any{"owner": "alice", "peer": "b" + pad4(i)})
	}
	s.seed(map[string]any{"blocks": bs})
	alice := s.dial("alice", "web")
	mustErr(t, alice.call(protocol.CmdBlock, map[string]any{"userId": "alice"}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdBlock, map[string]any{"userId": "zz1001"}), protocol.CodeBlockLimitExceeded)
	v := alice.ver()
	if ack := mustAck[socialAck](t, alice.call(protocol.CmdUnblock, map[string]any{"userId": "never"})); ack.Changed || ack.Ver != v {
		t.Fatalf("unblock not-listed: %+v", ack)
	}
	if ack := mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "b0001"})); ack.Changed {
		t.Fatalf("repeat block: %+v", ack)
	}
}

// TC-SOC-07-09（AC-SOC-07-7）
func TestTCSOC0709BlockKeepsFriend(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	alice := s.dial("alice", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	all, _ := alice.state()
	if len(all.Friends) != 1 || len(all.Blocks) != 1 || all.Blocks[0].Profile == nil {
		t.Fatalf("friends %+v blocks %+v", all.Friends, all.Blocks)
	}
}

// TC-SOC-07-10（AC-SOC-07-8）
func TestTCSOC0710BlockSilent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-sil-001", "hi"))
	alice.waitCmd(protocol.CmdMsg)
	search := string(bob.call(protocol.CmdUserSearch, map[string]any{"keyword": "alice"}).Data)
	users := string(bob.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice"}}).Data)
	bv := bob.ver()
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	bob.expectNoFrame(noFrameWindow)
	if bob.ver() != bv {
		t.Fatal("bob's ver changed")
	}
	if got := string(bob.call(protocol.CmdUserSearch, map[string]any{"keyword": "alice"}).Data); got != search {
		t.Errorf("search changed: %s vs %s", got, search)
	}
	if got := string(bob.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice"}}).Data); got != users {
		t.Errorf("users changed: %s vs %s", got, users)
	}
}

// TC-SOC-07-11（AC-SOC-07-9）
func TestTCSOC0711BlockNobodyIndistinguishable(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "eve")
	eve := s.dial("eve", "web")
	mustAck[protocol.VerAck](t, eve.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	alice := s.dial("alice", "web")
	a := alice.call(protocol.CmdBlock, map[string]any{"userId": "nobody"})
	b := alice.call(protocol.CmdBlock, map[string]any{"userId": "eve"})
	if stripVolatile(t, a.Data) != stripVolatile(t, b.Data) {
		t.Fatalf("nobody %s vs eve %s", a.Data, b.Data)
	}
	all, _ := alice.state()
	for _, bl := range all.Blocks {
		if bl.Profile != nil {
			t.Fatalf("block item %s has profile %+v", bl.UserID, bl.Profile)
		}
	}
}

// TC-SOC-07-12（AC-SOC-07-10）
func TestTCSOC0712BlockDailyQuota(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	for i := range 100 {
		s.advance(time.Second)
		cmd := protocol.CmdBlock
		if i%2 == 1 {
			cmd = protocol.CmdUnblock
		}
		mustAck[socialAck](t, alice.call(cmd, map[string]any{"userId": "bob"}))
	}
	s.advance(time.Second)
	mustErr(t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}), protocol.CodeQuotaExceeded)
}

// TC-SOC-07-13（AC-SOC-07-11）
func TestTCSOC0713BlockThenSend(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	for i := range 100 {
		s.resetLimits("alice")
		s.resetLimits("bob")
		mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
		mustErr(t, bob.sendText("c2c", "alice", cid("m-bts", i), "hi"), protocol.CodeRejectedByPeer)
		mustAck[socialAck](t, alice.call(protocol.CmdUnblock, map[string]any{"userId": "bob"}))
	}
	alice.expectNoMsg(noFrameWindow)
}

// ---- F-SOC-08 私聊权限 ----

// TC-SOC-08-01（AC-SOC-08-1）
func TestTCSOC0801FriendsMode(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.importFriends([2]string{"alice", "bob"}, [2]string{"alice", "carol"})
	s.settings(map[string]any{"mode": "friends"})
	alice, carol := s.dial("alice", "web"), s.dial("carol", "web")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-fm-0001", "hi"))
	mustAck[socialAck](t, carol.call(protocol.CmdFriendDelete, map[string]any{"userId": "alice"}))
	mustErr(t, alice.sendText("c2c", "carol", "m-fm-0002", "hi"), protocol.CodeNotFriend)
	if sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:carol", "afterSeq": 0})); len(sync.Messages) != 0 {
		t.Fatal("rejected message was saved")
	}
}

// TC-SOC-08-02（AC-SOC-08-2）、TC-SOC-08-03（回复解除是永久的）
func TestTCSOC0802StrangerLimit(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.settings(map[string]any{"strangerMsgLimit": 3})
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	for i := range 3 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", cid("m-sl", i), "hi"))
	}
	mustErr(t, alice.sendText("c2c", "bob", cid("m-sl", 3), "hi"), protocol.CodeStrangerMsgLimit)
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-sl-reply", "yo"))
	for i := range 10 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", cid("m-sl2", i), "hi"))
	}
	// 之后成为好友再删除（又变回陌生人）也不再受限
	s.importFriends([2]string{"alice", "bob"})
	s.mustAdm(http.MethodDelete, "/admin/friends", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}})
	s.resetLimits("alice")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-sl-after", "hi"))
}

// TC-SOC-08-04（AC-SOC-08-3）、TC-SOC-08-18（AC-SOC-08-15）：默认配置下成功发送不扣 newPeerReject。
func TestTCSOC0804DefaultsAndRefund(t *testing.T) {
	s := newSocEnvFastSend(t)
	var peers []string
	for i := range 150 {
		peers = append(peers, "u"+pad4(i))
	}
	s.usersNamed(append([]string{"alice", "bob"}, peers...)...)
	alice := s.dial("alice", "web")
	for i := range 10 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", cid("m-def", i), "hi"))
	}
	for i, p := range peers {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", p, cid("m-np", i), "hi"))
	}
	// newPeerReject 仍是满的：100 次静默失败都是 4042，第 101 次才 4292。
	for i := range 100 {
		mustErr(t, alice.sendText("c2c", "ghost"+pad4(i), cid("m-gh", i), "hi"), protocol.CodeUserNotFound)
	}
	mustErr(t, alice.sendText("c2c", "ghost9999", "m-gh-last", "hi"), protocol.CodeQuotaExceeded)
}

// TC-SOC-08-05（AC-SOC-08-4）
func TestTCSOC0805DedupBeforePermission(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	s.adminPost("/admin/test/drop-ack", map[string]any{"clientMsgId": "m-dedup-01"}, adminKey)
	b, _ := json.Marshal(map[string]any{"id": "x1", "cmd": "send", "data": map[string]any{"convType": "c2c", "to": "bob", "clientMsgId": "m-dedup-01", "type": "text", "body": map[string]any{"text": "hi"}}})
	alice.writeRaw(b)
	alice.expectNoAck("x1", noFrameWindow)
	first := bob.waitCmd(protocol.CmdMsg).msg(t)
	mustAck[socialAck](t, bob.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	again := mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-dedup-01", "hi"))
	if again.MsgID != first.MsgID || again.Seq != first.Seq {
		t.Fatalf("resend = %+v, first %+v", again, first)
	}
}

// TC-SOC-08-06（AC-SOC-08-5）
func TestTCSOC0806DecisionOrder(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[socialAck](t, bob.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	s.settings(map[string]any{"mode": "friends"})
	mustErr(t, alice.sendText("c2c", "nobody", "m-ord-001", "hi"), protocol.CodeUserNotFound)
	mustErr(t, alice.sendText("c2c", "bob", "m-ord-002", "hi"), protocol.CodeNotFriend)
	s.settings(map[string]any{"mode": "open"})
	mustErr(t, alice.sendText("c2c", "bob", "m-ord-003", "hi"), protocol.CodeRejectedByPeer)
}

// TC-SOC-08-07（AC-SOC-08-6）
func TestTCSOC0807NewPeerHourly(t *testing.T) {
	s := newSocEnvFastSend(t)
	var peers []string
	for i := range 32 {
		peers = append(peers, "u"+pad4(i))
	}
	s.usersNamed(append([]string{"alice", "carol"}, peers...)...)
	s.settings(map[string]any{"newPeerHourlyLimit": 30})
	alice := s.dial("alice", "web")
	mustAck[protocol.SendAck](t, s.dial("carol", "web").sendText("c2c", "alice", "m-nph-carol", "hi")) // 已有会话
	for i := range 30 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", peers[i], cid("m-nph", i), "hi"))
	}
	mustErr(t, alice.sendText("c2c", peers[30], "m-nph-0031", "hi"), protocol.CodeQuotaExceeded)
	mustErr(t, alice.sendText("c2c", "nobody", "m-nph-nobody", "hi"), protocol.CodeQuotaExceeded)
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "carol", "m-nph-carol2", "hi"))
	mustAck[protocol.SendAck](t, alice.sendText("c2c", peers[0], "m-nph-peer0", "hi")) // 聊过的不再是新对象
	s.advance(time.Hour)
	mustAck[protocol.SendAck](t, alice.sendText("c2c", peers[30], "m-nph-0032", "hi"))
}

// TC-SOC-08-08（AC-SOC-08-7）
func TestTCSOC0808SelfSend(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice")
	s.settings(map[string]any{"mode": "friends", "newPeerHourlyLimit": 1, "newPeerRejectDailyLimit": 10})
	alice := s.dial("alice", "web")
	for i := range 15 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "alice", cid("m-self", i), "note"))
	}
	// 给自己发不扣 newPeerHour：唯一的 1 个令牌还在，第 2 个新对象才 4292。
	mustErr(t, alice.sendText("c2c", "ghost0001", "m-self-ghost1", "hi"), protocol.CodeUserNotFound)
	mustErr(t, alice.sendText("c2c", "ghost0002", "m-self-ghost2", "hi"), protocol.CodeQuotaExceeded)
}

// TC-SOC-08-11（AC-SOC-08-9）
func TestTCSOC0811RejectedNotCounted(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.settings(map[string]any{"strangerMsgLimit": 3})
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	mustErr(t, bob.sendText("c2c", "alice", "m-rnc-000", "hi"), protocol.CodeRejectedByPeer)
	mustAck[socialAck](t, alice.call(protocol.CmdUnblock, map[string]any{"userId": "bob"}))
	for i := range 3 {
		mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", cid("m-rnc", i+1), "hi"))
	}
	mustErr(t, bob.sendText("c2c", "alice", "m-rnc-004", "hi"), protocol.CodeStrangerMsgLimit)
}

// TC-SOC-08-12（AC-SOC-08-10）
func TestTCSOC0812NewPeerRejectQuota(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice", "bob", "carol")
	alice := s.dial("alice", "web")
	mustAck[protocol.SendAck](t, s.dial("carol", "web").sendText("c2c", "alice", "m-npr-carol", "hi"))
	for i := range 100 {
		mustErr(t, alice.sendText("c2c", "ghost"+pad4(i), cid("m-npr", i), "hi"), protocol.CodeUserNotFound)
	}
	a := alice.sendText("c2c", "ghost9999", "m-npr-0101", "hi")
	b := alice.sendText("c2c", "bob", "m-npr-bob1", "hi")
	mustErr(t, a, protocol.CodeQuotaExceeded)
	if errBody(a) != errBody(b) {
		t.Fatalf("nobody %s vs bob %s", errBody(a), errBody(b))
	}
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "carol", "m-npr-carol2", "hi"))
	s.advance(15 * time.Minute)
	mustErr(t, alice.sendText("c2c", "ghost8888", "m-npr-later", "hi"), protocol.CodeUserNotFound) // 恢复 1 个令牌，通过配额检查
}

// TC-SOC-08-13（AC-SOC-08-11）
func TestTCSOC0813FriendsModeNewPeerReject(t *testing.T) {
	s := newSocEnvFastSend(t)
	var peers []string
	for i := range 101 {
		peers = append(peers, "u"+pad4(i))
	}
	s.usersNamed(append([]string{"alice"}, peers...)...)
	s.settings(map[string]any{"mode": "friends"})
	alice := s.dial("alice", "web")
	for i := range 100 {
		mustErr(t, alice.sendText("c2c", peers[i], cid("m-fmr", i), "hi"), protocol.CodeNotFriend)
	}
	mustErr(t, alice.sendText("c2c", peers[100], "m-fmr-101", "hi"), protocol.CodeQuotaExceeded)
}

// TC-SOC-08-14（AC-SOC-08-12，B6 前置：SQLite 上运行）
func TestTCSOC0814StrangerLimitRace(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice", "bob")
	s.settings(map[string]any{"strangerMsgLimit": 3})
	c1, c2 := s.dial("alice", "web"), s.dial("alice", "pc")
	var mu sync.Mutex
	ok, limited := 0, 0
	var wg sync.WaitGroup
	for k, c := range []*client{c1, c2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 10 {
				f := c.sendText("c2c", "bob", cid(fmt.Sprintf("m-race%d", k), i), "hi")
				mu.Lock()
				if f.Error == nil {
					ok++
				} else if f.Error.Code == protocol.CodeStrangerMsgLimit {
					limited++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 3 || limited != 17 {
		t.Fatalf("ok=%d limited=%d, want 3 / 17", ok, limited)
	}
	sync := mustAck[protocol.SyncAck](t, s.dial("bob", "web").call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0}))
	for i, m := range sync.Messages {
		if m.Seq != int64(i+1) {
			t.Fatalf("seq gap: %+v", sync.Messages)
		}
	}
	if len(sync.Messages) != 3 {
		t.Fatalf("%d messages saved", len(sync.Messages))
	}
}

// TC-SOC-08-17（AC-SOC-08-14）
func TestTCSOC0817NewPeerRejectConcurrent(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice", "bob")
	conns := make([]*client, 10)
	for i := range conns {
		conns[i] = s.dial("alice", "web")
	}
	for i := range 99 {
		mustErr(t, conns[0].sendText("c2c", "ghost"+pad4(i), cid("m-npc", i), "hi"), protocol.CodeUserNotFound)
	}
	var mu sync.Mutex
	codes := map[int]int{}
	var wg sync.WaitGroup
	for i, c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := c.sendText("c2c", fmt.Sprintf("par%d", i), cid("m-par", i), "hi")
			mu.Lock()
			if f.Error != nil {
				codes[f.Error.Code]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[protocol.CodeUserNotFound] != 1 || codes[protocol.CodeQuotaExceeded] != 9 {
		t.Fatalf("codes = %v, want one 4042 and nine 4292", codes)
	}
	mustErr(t, conns[0].sendText("c2c", "bob", "m-npc-bob", "hi"), protocol.CodeQuotaExceeded)
}
