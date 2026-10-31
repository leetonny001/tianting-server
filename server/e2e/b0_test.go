package e2e

// B0 基础对齐（BE-001～BE-006）新增能力的端到端测试。用例名标注对应的 TC / AC。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"im-mvp/server/internal/app"
	"im-mvp/server/internal/gateway"
	"im-mvp/server/internal/protocol"
)

// BE-003：故障注入和时钟接口只在测试模式下存在。
func TestTestRoutesOnlyInTestMode(t *testing.T) {
	prod := newEnvWith(t, gateway.DefaultConfig(), false)
	for _, path := range []string{"/admin/test/drop-ack", "/admin/test/skip-delivery", "/admin/test/clock"} {
		if code, _ := prod.adminPost(path, map[string]any{}, adminKey); code != http.StatusNotFound {
			t.Errorf("non-test mode %s = %d, want 404", path, code)
		}
	}
	test := newEnv(t)
	if code, body := test.adminPost("/admin/test/drop-ack", map[string]any{"clientMsgId": "x"}, adminKey); code != http.StatusOK {
		t.Errorf("test mode drop-ack = %d %s", code, body)
	}
}

// AC-MSG-03-4：服务端已保存但 ack 丢失，用原 clientMsgId 重发，对方只收到一条，msgId 不变。
func TestDropAckThenResendKeepsSingleMessage(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.dial("alice", "web"), e.dial("bob", "web")
	e.adminPost("/admin/test/drop-ack", map[string]any{"clientMsgId": "cid-lost-ack"}, adminKey)

	b, _ := json.Marshal(map[string]any{"id": "lost", "cmd": "send", "data": map[string]any{
		"convType": "c2c", "to": "bob", "clientMsgId": "cid-lost-ack", "type": "text", "body": map[string]any{"text": "hi"},
	}})
	alice.writeRaw(b)
	delivered := bob.waitCmd(protocol.CmdMsg).msg(t) // 消息已保存并投递
	alice.expectNoAck("lost", 300*time.Millisecond)  // 但 ack 被丢弃

	again := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-lost-ack", "hi"))
	if again.MsgID != delivered.MsgID || again.Seq != delivered.Seq {
		t.Fatalf("resend ack %+v, first saved %+v", again, delivered)
	}
	bob.expectNoMsg(200 * time.Millisecond)
}

// 为 AC-MSG-04-3 制造 seq 缺口：被跳过的消息不实时投递，但能 sync 到。
func TestSkipDeliveryCreatesSeqGap(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.dial("alice", "web"), e.dial("bob", "web")
	e.adminPost("/admin/test/skip-delivery", map[string]any{"clientMsgId": "cid-gap-0002"}, adminKey)
	for i := 1; i <= 3; i++ {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-gap-%04d", i), fmt.Sprint(i)))
	}
	if s := bob.waitCmd(protocol.CmdMsg).msg(t).Seq; s != 1 {
		t.Fatalf("first realtime seq = %d", s)
	}
	if s := bob.waitCmd(protocol.CmdMsg).msg(t).Seq; s != 3 {
		t.Fatalf("second realtime seq = %d, want 3 (seq 2 skipped)", s)
	}
	sync := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 1}))
	if len(sync.Messages) != 2 || sync.Messages[0].Seq != 2 {
		t.Fatalf("sync after gap = %+v", sync.Messages)
	}
}

// C-43 / IMP-7：冻结服务端时钟后，消息时间戳等于冻结的时间。
func TestClockInjection(t *testing.T) {
	e := newEnv(t)
	e.users("bob")
	alice := e.dial("alice", "web")
	const frozen = int64(1_700_000_000_000)
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": frozen}, adminKey)
	a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-clock-001", "t"))
	if a.TS != frozen {
		t.Fatalf("ts = %d, want frozen %d", a.TS, frozen)
	}
	e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey)
	b := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-clock-002", "t"))
	if time.Since(time.UnixMilli(b.TS)) > 5*time.Second {
		t.Fatalf("clock not restored: ts = %d", b.TS)
	}
}

// AC-AUTH-01-4、AC-AUTH-01-5、C-13：有效期默认 7 天、上限 30 天、下限 60 秒（测试模式 1 秒）。
func TestTokenTTLRules(t *testing.T) {
	expiresIn := func(e *env, ttl any) time.Duration {
		t.Helper()
		body := map[string]any{"userId": "u1"}
		if ttl != nil {
			body["ttlSeconds"] = ttl
		}
		code, raw := e.adminPost("/admin/users/token", body, adminKey)
		if code != http.StatusOK {
			t.Fatalf("issue token: %d %s", code, raw)
		}
		var r struct{ ExpiresAt int64 }
		json.Unmarshal(raw, &r)
		return time.Until(time.UnixMilli(r.ExpiresAt))
	}
	near := func(got, want time.Duration) bool { d := got - want; return d > -5*time.Second && d < 5*time.Second }

	prod := newEnvWith(t, gateway.DefaultConfig(), false)
	for _, tc := range []struct {
		ttl  any
		want time.Duration
	}{
		{nil, 7 * 24 * time.Hour}, {0, 7 * 24 * time.Hour}, {-1, 7 * 24 * time.Hour},
		{30 * 86400, 30 * 24 * time.Hour}, {31 * 86400, 30 * 24 * time.Hour},
		{1, 60 * time.Second},
	} {
		if got := expiresIn(prod, tc.ttl); !near(got, tc.want) {
			t.Errorf("ttl %v: expires in %v, want %v", tc.ttl, got, tc.want)
		}
	}
	if got := expiresIn(newEnv(t), 1); !near(got, time.Second) {
		t.Errorf("test mode ttl 1: expires in %v", got)
	}
}

// AC-MSG-01-4、AC-MSG-01-5：服务端去首尾空白后校验并存储。
func TestServerTrimsAndValidatesText(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.dial("alice", "web"), e.dial("bob", "web")

	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-trim-001", "  你好  "))
	if m := bob.waitCmd(protocol.CmdMsg).msg(t); m.Body.Text != "你好" {
		t.Fatalf("delivered %q, want trimmed", m.Body.Text)
	}
	mustErr(t, alice.sendText(protocol.ConvC2C, "bob", "cid-trim-002", " \n\t　 "), protocol.CodeTextEmpty)
	mustErr(t, alice.sendText(protocol.ConvC2C, "bob", "cid-trim-003", strings.Repeat("字", 5001)), protocol.CodeTextTooLong)
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-trim-004", " "+strings.Repeat("字", 5000)+" "))
}

// AC-MSG-04-2：同一 clientMsgId 用到另一个会话返回 4008。
func TestClientMsgIDReusedInAnotherConversation(t *testing.T) {
	e := newEnv(t)
	e.users("bob", "carol")
	alice := e.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", "cid-reuse-01", "to bob"))
	mustErr(t, alice.sendText(protocol.ConvC2C, "carol", "cid-reuse-01", "to carol"), protocol.CodeClientMsgIDConflict)
}

// AC-MSG-01-6、AC-MSG-04-7：ack 带 convId；下发的消息带 clientMsgId。
func TestAckHasConvIDAndMessagesHaveClientMsgID(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.dial("alice", "web"), e.dial("Bob", "web")
	a := mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "Bob", "cid-fields-1", "x"))
	if a.ConvID != "c2c:Bob:alice" { // ASCII：'B' < 'a'
		t.Errorf("ack convId = %q", a.ConvID)
	}
	if m := bob.waitCmd(protocol.CmdMsg).msg(t); m.ClientMsgID != "cid-fields-1" {
		t.Errorf("msg clientMsgId = %q", m.ClientMsgID)
	}
	sync := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": a.ConvID, "afterSeq": 0}))
	if sync.Messages[0].ClientMsgID != "cid-fields-1" {
		t.Errorf("sync clientMsgId = %q", sync.Messages[0].ClientMsgID)
	}
}

// AC-MSG-05-3、AC-MSG-05-4：sync 两个方向和 limit 边界。
func TestSyncDirectionsAndLimits(t *testing.T) {
	e := newEnvCfg(t, func(c *app.Config) { c.SendRate = 1000 }) // 要连发 100 条，限流另有专门用例
	e.users("bob")
	alice := e.dial("alice", "web")
	for i := 1; i <= 100; i++ {
		mustAck[protocol.SendAck](t, alice.sendText(protocol.ConvC2C, "bob", fmt.Sprintf("cid-page-%04d", i), fmt.Sprint(i)))
	}
	conv := "c2c:alice:bob"

	after := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": conv, "afterSeq": 0, "limit": 3}))
	if len(after.Messages) != 3 || !after.HasMore || after.Messages[0].Seq != 1 {
		t.Fatalf("afterSeq page = %d msgs hasMore=%v", len(after.Messages), after.HasMore)
	}
	before := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": conv, "beforeSeq": 101, "limit": 20}))
	if len(before.Messages) != 20 || !before.HasMore || before.Messages[0].Seq != 81 || before.Messages[19].Seq != 100 {
		t.Fatalf("beforeSeq page: first=%d last=%d hasMore=%v", before.Messages[0].Seq, before.Messages[19].Seq, before.HasMore)
	}
	first := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": conv, "beforeSeq": 21, "limit": 20}))
	if first.HasMore || first.Messages[0].Seq != 1 {
		t.Fatalf("earliest page hasMore=%v first=%d", first.HasMore, first.Messages[0].Seq)
	}
	all := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": conv, "afterSeq": 0, "limit": 200}))
	if len(all.Messages) != 100 || all.HasMore {
		t.Fatalf("limit 200: %d msgs hasMore=%v", len(all.Messages), all.HasMore)
	}
	for _, bad := range []map[string]any{
		{"convId": conv, "afterSeq": 0, "limit": 201},
		{"convId": conv, "afterSeq": 0, "limit": 0},
		{"convId": conv, "afterSeq": 0, "beforeSeq": 5},
		{"convId": conv},
	} {
		mustErr(t, alice.call(protocol.CmdSync, bad), protocol.CodeBadRequest)
	}
}

// AC-AUTH-02-5：同一账号同平台两个连接都在线，都能收到消息。
func TestSamePlatformTwoConnections(t *testing.T) {
	e := newEnv(t)
	web1, web2 := e.dial("alice", "web"), e.dial("alice", "web")
	bob := e.dial("bob", "web")
	mustAck[protocol.SendAck](t, bob.sendText(protocol.ConvC2C, "alice", "cid-twoweb-1", "hi"))
	web1.waitCmd(protocol.CmdMsg)
	web2.waitCmd(protocol.CmdMsg)
}

// AC-GRP-02-3：已存在但没有成员的群，非成员发消息返回 4033（不是 4043）。
func TestEmptyGroupNonMemberGets4033(t *testing.T) {
	e := newEnv(t)
	e.createGroup("empty")
	alice := e.dial("alice", "web")
	mustErr(t, alice.sendText(protocol.ConvGroup, "empty", "cid-empty-01", "hi"), protocol.CodeNotGroupMember)
}

// AC-SYS-01-2：管理接口错误码与 HTTP 状态码一致。
func TestAdminErrorCodes(t *testing.T) {
	e := newEnv(t)
	e.createGroup("g1", "alice")
	code, body := e.adminPost("/admin/groups", map[string]any{"groupId": "g1", "name": "dup"}, adminKey)
	assertAdminErr(t, code, body, http.StatusConflict, protocol.CodeGroupExists)
	code, body = e.adminPost("/admin/groups/nope/members", map[string]any{"members": []string{"alice"}}, adminKey)
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeGroupNotFound)
	code, body = e.adminPost("/admin/groups", map[string]any{"groupId": "bad id", "name": "x"}, adminKey)
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
}
