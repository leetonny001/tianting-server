package e2e

// PRD-social 5.4 节逐行（BE-118，TC-SOC-06-14，AC-SOC-03-7、06-9、06-11）：每种操作谁的 ver 变化、改了哪些行（同一个 ver）、
// 谁收到 socialChanged、谁收不到任何帧。A 有两个连接（A1 发起、A2 旁观），B 一个连接。

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

type verRow struct {
	name   string
	setup  func(s *socEnv, a1, b *client)
	op     func(s *socEnv, a1 *client) frame // 返回 ack；管理接口 / 清理返回零值
	da, db int64                             // A、B 的 ver 变化
	admin  bool                              // 管理接口操作：发起方没有连接，A1 也收事件
}

func TestTCSOC0614VerPerOperation(t *testing.T) {
	apply := func(from *client, to string) { from.apply(to, nil) }
	cmd := func(c string, d map[string]any) func(*socEnv, *client) frame {
		return func(_ *socEnv, a1 *client) frame { return a1.call(c, d) }
	}
	rows := []verRow{
		{name: "1 apply new", op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"}), da: 1, db: 1},
		{name: "1 apply again after reject", setup: func(s *socEnv, a1, b *client) {
			apply(a1, "bob")
			mustAck[socialAck](t, b.respond("alice", "reject", nil))
		}, op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"}), da: 1, db: 1},
		{name: "2 apply to someone who blocked me", setup: func(s *socEnv, a1, b *client) {
			mustAck[socialAck](t, b.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
		}, op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"}), da: 1},
		{name: "2 apply to nobody", op: cmd(protocol.CmdFriendApply, map[string]any{"to": "nobody"}), da: 1},
		{name: "3 apply when already friends", setup: func(s *socEnv, _, _ *client) { s.importFriends([2]string{"alice", "bob"}) },
			op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"})},
		{name: "4 restore one-way edge", setup: func(s *socEnv, a1, _ *client) {
			s.importFriends([2]string{"alice", "bob"})
			mustAck[socialAck](t, a1.call(protocol.CmdFriendDelete, map[string]any{"userId": "bob"}))
		}, op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"}), da: 1},
		{name: "5 auto accept", setup: func(s *socEnv, _, b *client) { apply(b, "alice") },
			op: cmd(protocol.CmdFriendApply, map[string]any{"to": "bob"}), da: 1, db: 1},
		{name: "6 accept", setup: func(s *socEnv, _, b *client) { apply(b, "alice") },
			op: cmd(protocol.CmdFriendRespond, map[string]any{"from": "bob", "action": "accept"}), da: 1, db: 1},
		{name: "7 reject", setup: func(s *socEnv, _, b *client) { apply(b, "alice") },
			op: cmd(protocol.CmdFriendRespond, map[string]any{"from": "bob", "action": "reject"}), da: 1},
		{name: "8 repeated reject", setup: func(s *socEnv, a1, b *client) {
			apply(b, "alice")
			mustAck[socialAck](t, a1.respond("bob", "reject", nil))
		}, op: cmd(protocol.CmdFriendRespond, map[string]any{"from": "bob", "action": "reject"})},
		{name: "9 delete friend", setup: func(s *socEnv, _, _ *client) { s.importFriends([2]string{"alice", "bob"}) },
			op: cmd(protocol.CmdFriendDelete, map[string]any{"userId": "bob"}), da: 1},
		{name: "10 delete non-friend", op: cmd(protocol.CmdFriendDelete, map[string]any{"userId": "bob"})},
		{name: "10 remark unchanged", setup: func(s *socEnv, a1, _ *client) {
			s.importFriends([2]string{"alice", "bob"})
			mustAck[socialAck](t, a1.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "B"}))
		}, op: cmd(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "B"})},
		{name: "11 remark changed", setup: func(s *socEnv, _, _ *client) { s.importFriends([2]string{"alice", "bob"}) },
			op: cmd(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "B"}), da: 1},
		{name: "12 block (rejects pending in row)", setup: func(s *socEnv, _, b *client) { apply(b, "alice") },
			op: cmd(protocol.CmdBlock, map[string]any{"userId": "bob"}), da: 1},
		{name: "13 unblock", setup: func(s *socEnv, a1, _ *client) {
			mustAck[socialAck](t, a1.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
		}, op: cmd(protocol.CmdUnblock, map[string]any{"userId": "bob"}), da: 1},
		{name: "14 repeated block", setup: func(s *socEnv, a1, _ *client) {
			mustAck[socialAck](t, a1.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
		}, op: cmd(protocol.CmdBlock, map[string]any{"userId": "bob"})},
		{name: "14 unblock not listed", op: cmd(protocol.CmdUnblock, map[string]any{"userId": "bob"})},
		{name: "15 setPrivacy changed", op: cmd(protocol.CmdSetPrivacy, map[string]any{"searchable": false}), da: 1},
		{name: "16 requestsSeen advances", setup: func(s *socEnv, _, b *client) { apply(b, "alice") },
			op: cmd(protocol.CmdFriendRequestsSeen, map[string]any{"ver": 99}), da: 1},
		{name: "17 admin import", admin: true, op: func(s *socEnv, _ *client) frame {
			s.importFriends([2]string{"alice", "bob"})
			return frame{}
		}, da: 1, db: 1},
		{name: "17 admin delete", admin: true, setup: func(s *socEnv, _, _ *client) { s.importFriends([2]string{"alice", "bob"}) },
			op: func(s *socEnv, _ *client) frame {
				s.mustAdm(http.MethodDelete, "/admin/friends", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}})
				return frame{}
			}, da: 1, db: 1},
		{name: "18 purge", admin: true, setup: func(s *socEnv, a1, _ *client) {
			s.importFriends([2]string{"alice", "bob"})
			mustAck[socialAck](t, a1.call(protocol.CmdFriendDelete, map[string]any{"userId": "bob"}))
			s.advance(31 * 24 * time.Hour)
		}, op: func(s *socEnv, _ *client) frame { s.purge(); return frame{} }},
		{name: "19 official mark", admin: true, op: func(s *socEnv, _ *client) frame { s.official("alice", true); return frame{} }},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			s := newSocEnv(t)
			s.usersNamed("alice", "bob")
			a1, a2, b := s.dial("alice", "web"), s.dial("alice", "pc"), s.dial("bob", "web")
			if row.setup != nil {
				row.setup(s, a1, b)
			}
			if row.name == "18 purge" { // 推进时钟后 token 可能过期：重新连接
				a1, a2, b = s.dial("alice", "web"), s.dial("alice", "pc"), s.dial("bob", "web")
			}
			s.resetLimits("alice")
			s.resetLimits("bob")
			av, bv := a2.ver(), b.ver()
			time.Sleep(50 * time.Millisecond) // 让 setup 的事件都到达，再清空
			for _, c := range []*client{a1, a2, b} {
				c.expectNoFrameDrain()
			}
			f := row.op(s, a1)
			if f.ID != "" {
				if f.Error != nil {
					t.Fatalf("op error %+v", f.Error)
				}
				var ack socialAck
				if json.Unmarshal(f.Data, &ack) == nil && ack.Changed {
					for _, x := range ack.Items.Friends {
						assertSameVer(t, x.Ver, ack.Ver)
					}
					for _, x := range ack.Items.Requests {
						assertSameVer(t, x.Ver, ack.Ver)
					}
					for _, x := range ack.Items.Blocks {
						assertSameVer(t, x.Ver, ack.Ver)
					}
				}
			}
			expectEvent := func(c *client, want bool) {
				t.Helper()
				if want {
					c.waitSocialChanged()
				} else {
					c.expectNoFrame(noFrameWindow)
				}
			}
			expectEvent(a2, row.da > 0)
			expectEvent(b, row.db > 0)
			expectEvent(a1, row.admin && row.da > 0) // 发起操作的连接从 ack 拿结果，不收事件
			s.resetLimits("alice")
			s.resetLimits("bob")
			if got := a2.ver() - av; got != row.da {
				t.Errorf("alice ver +%d, want +%d", got, row.da)
			}
			if got := b.ver() - bv; got != row.db {
				t.Errorf("bob ver +%d, want +%d", got, row.db)
			}
		})
	}
}

func assertSameVer(t *testing.T, got, want int64) {
	t.Helper()
	if got != want {
		t.Errorf("item ver %d, ack ver %d: rows of one operation must share one ver", got, want)
	}
}

// expectNoFrameDrain 丢弃已经到达的帧（不等待）。
func (c *client) expectNoFrameDrain() {
	c.pending = nil
	for {
		select {
		case _, ok := <-c.in:
			if !ok {
				return
			}
		default:
			return
		}
	}
}
