package e2e

// B2 消息（BE-020～BE-024）的端到端测试。用例名标注对应的 AC。

import (
	"fmt"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

// AC-MSG-01-8、AC-CONV-03-3（服务端部分）：给不存在的用户发消息返回 4042，不保存。
func TestSendToUnknownUser(t *testing.T) {
	e := newEnv(t)
	alice := e.dial("alice", "web")
	mustErr(t, alice.sendText(protocol.ConvC2C, "ghost", "cid-ghost-001", "hello?"), protocol.CodeUserNotFound)

	sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:ghost", "afterSeq": 0}))
	if len(sync.Messages) != 0 {
		t.Fatalf("rejected message was stored: %+v", sync.Messages)
	}
	// 用户注册后可以发，而且 seq 从 1 开始（证明之前没有落库、也没有占用 seq）。
	e.users("ghost")
	if a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "ghost", "cid-ghost-002", "now")); a.Seq != 1 {
		t.Errorf("first stored seq = %d, want 1", a.Seq)
	}
}

// AC-MSG-01-7：可以给自己发消息，自己的其他设备也会收到。
func TestSendToSelf(t *testing.T) {
	e := newEnv(t)
	phone, pc := e.dial("alice", "ios"), e.dial("alice", "pc")
	a := mustAck[protocol.SendAck](t, phone.sendText(protocol.ConvC2C, "alice", "cid-self-0001", "备忘"))
	if a.ConvID != "c2c:alice:alice" || a.Seq != 1 {
		t.Fatalf("ack = %+v", a)
	}
	if m := pc.waitCmd(protocol.CmdMsg).msg(t); m.MsgID != a.MsgID {
		t.Errorf("pc got %+v", m)
	}
	phone.expectNoMsg(200 * time.Millisecond)
}

// AC-MSG-07-1、C-29、IMP-3、IMP-4：每人每秒 20 条，令牌桶初始为满；
// 被 4290 拒绝的请求和去重命中的重发都不计数。用冻结时钟保证结果确定。
func TestSendRateLimit(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	alice := e.dial("alice", "web")
	t0 := time.Now()
	freeze := func(d time.Duration) {
		e.adminPost("/admin/test/clock", map[string]any{"nowMs": t0.Add(d).UnixMilli()}, adminKey)
	}
	freeze(0)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)

	send := func(cid string) frame {
		return alice.sendText(protocol.ConvC2C, "bob", cid, "x")
	}
	var first protocol.SendAck
	for i := range 20 {
		a := mustAck[protocol.SendAck](t, send(fmt.Sprintf("cid-rate-%04d", i)))
		if i == 0 {
			first = a
		}
	}
	for i := 20; i < 25; i++ {
		mustErr(t, send(fmt.Sprintf("cid-rate-%04d", i)), protocol.CodeRateLimited)
	}
	// 桶空时重发已保存的消息：返回原 ack，而不是 4290。
	if again := mustAck[protocol.SendAck](t, send("cid-rate-0000")); again != first {
		t.Fatalf("dedup resend with empty bucket = %+v, want %+v", again, first)
	}
	// 同一用户的另一个连接共享同一个桶。
	pc := e.dial("alice", "pc")
	mustErr(t, pc.sendText(protocol.ConvC2C, "bob", "cid-rate-pc01", "x"), protocol.CodeRateLimited)

	// 0.5 秒后补充 10 个令牌（之前被拒绝的请求没有扣令牌）。
	freeze(500 * time.Millisecond)
	for i := range 10 {
		mustAck[protocol.SendAck](t, send(fmt.Sprintf("cid-rate-b%03d", i)))
	}
	mustErr(t, send("cid-rate-b010"), protocol.CodeRateLimited)

	// 被拒绝的消息没有落库：一共 30 条。
	sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0, "limit": 200}))
	if len(sync.Messages) != 30 {
		t.Errorf("stored %d messages, want 30", len(sync.Messages))
	}
}

// AC-MSG-07-1：限流按用户隔离。
func TestRateLimitIsPerUser(t *testing.T) {
	e := newEnv(t)
	e.users("carol")
	alice, bob := e.dial("alice", "web"), e.dial("bob", "web")
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": time.Now().UnixMilli()}, adminKey)
	defer e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	for i := range 20 {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "carol", fmt.Sprintf("cid-a-%04d", i), "x"))
	}
	mustErr(t, alice.sendText(protocol.ConvC2C, "carol", "cid-a-0020", "x"), protocol.CodeRateLimited)
	mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "carol", "cid-b-0000", "x"))
}
