package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/gateway"
	"im-mvp/server/internal/protocol"
)

func TestRejectsInvalidToken(t *testing.T) {
	e := newEnv(t)
	_, resp, err := websocket.Dial(context.Background(), e.wsURL("bad-token", "web"), nil)
	if err == nil {
		t.Fatal("dial with bad token should fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", resp)
	}
}

func TestAdminRequiresKey(t *testing.T) {
	e := newEnv(t)
	// TC-AUTH-01-*：密钥错误 401 / 4013，userId 不合法 400 / 4004（AC-AUTH-01-2、AC-AUTH-01-3）。
	for _, key := range []string{"", "wrong"} {
		code, body := e.adminPost("/admin/users/token", map[string]any{"userId": "u1"}, key)
		assertAdminErr(t, code, body, http.StatusUnauthorized, protocol.CodeAdminKeyInvalid)
	}
	for _, uid := range []string{"", "bad id", "中文", "a.b", "a@b", strings.Repeat("x", 65)} {
		code, body := e.adminPost("/admin/users/token", map[string]any{"userId": uid}, adminKey)
		assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	}
	for _, uid := range []string{"a", strings.Repeat("x", 64)} {
		if code, body := e.adminPost("/admin/users/token", map[string]any{"userId": uid}, adminKey); code != http.StatusOK {
			t.Errorf("userId len %d: %d %s", len(uid), code, body)
		}
	}
}

func TestC2CDeliveryToReceiverAndSenderOtherDevices(t *testing.T) {
	e := newEnv(t)
	alicePhone := e.dial("alice", "ios")
	alicePC := e.dial("alice", "pc")
	bob := e.dial("bob", "android")

	ack := mustAck[protocol.SendAck](t, alicePhone.sendText(protocol.ConvC2C, "bob", "cid-00000001", "hi bob"))
	if ack.Seq != 1 || ack.MsgID == "" || ack.TS == 0 {
		t.Fatalf("ack = %+v", ack)
	}

	for _, c := range []*client{bob, alicePC} {
		m := c.waitCmd(protocol.CmdMsg).msg(t)
		if m.MsgID != ack.MsgID || m.Seq != 1 || m.From != "alice" || m.ConvID != "c2c:alice:bob" || m.Body.Text != "hi bob" {
			t.Errorf("%s got %+v", c.uid, m)
		}
	}
	// 发送端自己不会再收到一遍。
	alicePhone.expectNoMsg(200 * time.Millisecond)

	// 回复走同一个会话，seq 继续递增。
	reply := mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", "cid-00000002", "hi alice"))
	if reply.Seq != 2 {
		t.Errorf("reply seq = %d, want 2", reply.Seq)
	}
}

func TestResendWithSameClientMsgIDIsDeduplicated(t *testing.T) {
	e := newEnv(t)
	alice := e.dial("alice", "web")
	bob := e.dial("bob", "web")

	first := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-retry-01", "once"))
	// 模拟 ack 丢失后客户端用同一个 clientMsgId 重发。
	again := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-retry-01", "once"))
	if again != first {
		t.Fatalf("resend ack %+v != first %+v", again, first)
	}
	// TC-MSG-04-*：重复提交 5 次只保存 1 条（AC-MSG-04-1）。
	for range 3 {
		if a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-retry-01", "once")); a != first {
			t.Fatalf("resend ack %+v != first %+v", a, first)
		}
	}

	bob.waitCmd(protocol.CmdMsg)
	bob.expectNoMsg(200 * time.Millisecond)

	sync := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0}))
	if len(sync.Messages) != 1 {
		t.Fatalf("stored %d messages, want 1", len(sync.Messages))
	}
}

func TestOfflineReceiverSyncsAfterConnect(t *testing.T) {
	e := newEnv(t)
	alice := e.dial("alice", "web")
	for i := 1; i <= 5; i++ {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-offline-%d", i), fmt.Sprint("m", i)))
	}

	bob := e.dial("bob", "android") // 上线后按 seq 补拉
	page1 := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0, "limit": 3}))
	if len(page1.Messages) != 3 || !page1.HasMore || page1.Messages[0].Seq != 1 || page1.Messages[2].Seq != 3 {
		t.Fatalf("page1 = %+v", page1)
	}
	page2 := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 3}))
	if len(page2.Messages) != 2 || page2.HasMore || page2.Messages[1].Body.Text != "m5" {
		t.Fatalf("page2 = %+v", page2)
	}
}

func TestSyncAndReadRequireParticipation(t *testing.T) {
	e := newEnv(t)
	carol := e.dial("carol", "web")
	mustErr(t, carol.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0}), protocol.CodeNotConvParticipant)
	mustErr(t, carol.call(protocol.CmdRead, map[string]any{"convId": "c2c:alice:bob", "seq": 1}), protocol.CodeNotConvParticipant)
	mustAck[protocol.Empty](t, carol.call(protocol.CmdRead, map[string]any{"convId": "c2c:alice:carol", "seq": 1}))
}

func TestGroupMessaging(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice", "bob", "carol")
	alice := e.dial("alice", "web")
	bob := e.dial("bob", "ios")
	carol := e.dial("carol", "android")
	dave := e.dial("dave", "pc")

	mustErr(t, dave.sendText(protocol.ConvGroup, "g1", "cid-dave-001", "let me in"), protocol.CodeNotGroupMember)
	mustErr(t, alice.sendText(protocol.ConvGroup, "nope", "cid-nope-001", "?"), protocol.CodeGroupNotFound)
	mustErr(t, alice.call(protocol.CmdSync, map[string]any{"convId": "group:nope", "afterSeq": 0}), protocol.CodeGroupNotFound)

	ack := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvGroup, "g1", "cid-group-01", "hello group"))
	for _, c := range []*client{bob, carol} {
		m := c.waitCmd(protocol.CmdMsg).msg(t)
		if m.MsgID != ack.MsgID || m.ConvID != "group:g1" || m.Seq != 1 {
			t.Errorf("%s got %+v", c.uid, m)
		}
	}
	dave.expectNoMsg(200 * time.Millisecond)

	sync := mustAck[protocol.SyncAck](t, carol.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 0}))
	if len(sync.Messages) != 1 {
		t.Errorf("carol synced %d messages", len(sync.Messages))
	}
	mustErr(t, dave.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 0}), protocol.CodeNotGroupMember)

	// 后加入的成员可以收发。
	if code, body := e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"dave"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add member: %d %s", code, body)
	}
	mustAck[protocol.SendAck](t, dave.sendText(protocol.ConvGroup, "g1", "cid-dave-002", "thanks"))
	if m := alice.waitCmd(protocol.CmdMsg).msg(t); m.From != "dave" || m.Seq != 2 {
		t.Errorf("alice got %+v", m)
	}
}

func TestConcurrentGroupSendersGetContiguousSeq(t *testing.T) {
	e := newEnv(t)
	const senders, perSender = 5, 20
	members := []string{"watcher"}
	for i := range senders {
		members = append(members, fmt.Sprint("s", i))
	}
	e.createGroup("busy", members...)
	watcher := e.dial("watcher", "web")

	var wg sync.WaitGroup
	for i := range senders {
		c := e.dial(fmt.Sprint("s", i), "web")
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range perSender {
				b := fmt.Sprintf(`{"id":"r%d","cmd":"send","data":{"convType":"group","to":"busy","clientMsgId":"cid-%d-%03d","type":"text","body":{"text":"x"}}}`, j, i, j)
				c.ws.Write(context.Background(), websocket.MessageText, []byte(b))
			}
		}()
	}
	wg.Wait()

	// 实时投递在并发下不保证按 seq 顺序到达，但必须一条不少、seq 不重复。
	seen := map[int64]bool{}
	for len(seen) < senders*perSender {
		m := watcher.waitCmd(protocol.CmdMsg).msg(t)
		if seen[m.Seq] {
			t.Fatalf("duplicate seq %d", m.Seq)
		}
		seen[m.Seq] = true
	}

	sync := mustAck[protocol.SyncAck](t, watcher.call(protocol.CmdSync, map[string]any{"convId": "group:busy", "afterSeq": 0, "limit": 200}))
	if len(sync.Messages) != senders*perSender {
		t.Fatalf("synced %d", len(sync.Messages))
	}
	for i, m := range sync.Messages {
		if m.Seq != int64(i+1) {
			t.Fatalf("sync[%d].Seq = %d: seq must be contiguous", i, m.Seq)
		}
	}
}

func TestProtocolErrors(t *testing.T) {
	e := newEnv(t)
	c := e.dial("alice", "web")

	c.writeRaw([]byte(`not json`))
	if f := c.next(); f.Cmd != protocol.CmdAck || f.ID != "" || f.Error == nil || f.Error.Code != protocol.CodeInvalidFrame {
		t.Errorf("invalid json -> %+v", f)
	}
	c.writeRaw([]byte(`{"cmd":"send","data":{}}`))
	if f := c.next(); f.Error == nil || f.Error.Code != protocol.CodeMissingRequestID {
		t.Errorf("missing id -> %+v", f)
	}

	mustErr(t, c.call("fly", map[string]any{}), protocol.CodeUnknownCmd)
	mustErr(t, c.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "bob"}), protocol.CodeBadRequest)
	mustErr(t, c.call(protocol.CmdSend, map[string]any{
		"convType": "c2c", "to": "bob", "clientMsgId": "cid-00000009", "type": "text", "body": map[string]any{"text": "x"}, "extra": 1,
	}), protocol.CodeBadRequest)
	mustErr(t, c.call(protocol.CmdSync, map[string]any{"convId": "c2c:bob:alice", "afterSeq": 0}), protocol.CodeInvalidConvID)
	mustErr(t, c.call(protocol.CmdJoinRoom, map[string]any{"roomId": "match-1"}), protocol.CodeBadRequest)
}

func TestHeartbeat(t *testing.T) {
	cfg := gateway.DefaultConfig()
	cfg.IdleTimeout = 300 * time.Millisecond
	e := newEnvWithGateway(t, cfg)

	// 持续 ping 的连接不会被断开。
	alive := e.dial("alice", "web")
	for range 6 {
		alive.writeRaw([]byte(`{"cmd":"ping"}`))
		if f := alive.waitCmd(protocol.CmdPong); f.Cmd != protocol.CmdPong {
			t.Fatal("expected pong")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// TC-CONN-01-*：不发任何帧的连接在空闲超时后被关闭，关闭码 4408（AC-CONN-01-2）。
	idle := e.dial("bob", "web")
	if code := idle.closeCode(2 * time.Second); code != protocol.CloseIdleTimeout {
		t.Fatalf("idle close code = %d, want %d", code, protocol.CloseIdleTimeout)
	}
}
