package e2e

// PRD-social F-SOC-06（BE-108、BE-109）：关系同步、分页、墓碑与清理、并发、参数校验、ack。

import (
	"sync"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

// TC-SOC-06-01（AC-SOC-06-1）
func TestTCSOC0601OfflineCatchup(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	bob := s.dial("bob", "web")
	bob.apply("alice", nil)
	v0 := bob.ver()
	bob.ws.CloseNow()
	s.dial("carol", "web").apply("bob", nil)
	s.dial("dave", "web").apply("bob", nil)
	mustAck[socialAck](t, s.dial("alice", "web").respond("bob", "accept", nil))

	bob = s.dial("bob", "pc")
	p := mustAck[protocol.SocialSyncAck](t, bob.call(protocol.CmdSocialSync, map[string]any{"sinceVer": v0}))
	if p.Full || p.UntilVer != v0+3 || p.HasMore || len(p.Friends)+len(p.Requests) != 4 {
		t.Fatalf("page = %+v", p)
	}
	c, _ := findReq(p.Requests, "carol", "in")
	d, _ := findReq(p.Requests, "dave", "in")
	out, _ := findReq(p.Requests, "alice", "out")
	f, _ := findFriend(p.Friends, "alice")
	if c.Ver != v0+1 || d.Ver != v0+2 || out.Ver != v0+3 || out.Status != "accepted" || out.Profile == nil || f.Ver != v0+3 {
		t.Fatalf("vers: carol %d dave %d out %+v friend %+v", c.Ver, d.Ver, out, f)
	}
}

// TC-SOC-06-04、06-05（AC-SOC-06-3）：事件被丢弃时，重连或下一次事件触发的同步轮补齐。
func TestTCSOC0604DroppedEvents(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	alice := s.dial("alice", "web")
	v0 := alice.ver()
	alice.apply("bob", nil)
	s.adminPost("/admin/test/social/drop-events", map[string]any{"userId": "alice", "type": "socialChanged", "count": 1}, adminKey)
	mustAck[socialAck](t, s.dial("bob", "web").respond("alice", "accept", nil))
	alice.expectNoFrame(noFrameWindow)
	s.dial("carol", "web").apply("alice", nil) // 这次事件没有被丢弃
	ver := alice.waitSocialChanged()
	pages := alice.syncRound(v0+1, 500)
	p := pages[0]
	if p.UntilVer != ver || len(p.Friends) != 1 || len(p.Requests) != 2 {
		t.Fatalf("round from %d: %+v", v0+1, p)
	}
	// 重连路径：新连接从同一个 ver 开始同样能拿到
	again := s.dial("alice", "pc").syncRound(v0+1, 500)[0]
	if len(again.Friends) != 1 || len(again.Requests) != 2 {
		t.Fatalf("reconnect round: %+v", again)
	}
}

// TC-SOC-06-06（AC-SOC-06-4）
func TestTCSOC0606SameVerAcrossPages(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	alice := s.dial("alice", "web")
	v := alice.ver()
	s.importFriends([2]string{"alice", "bob"}, [2]string{"alice", "carol"}, [2]string{"alice", "dave"})
	pages := alice.syncRound(v, 2)
	if len(pages) != 2 || len(pages[0].Friends) != 2 || !pages[0].HasMore || len(pages[1].Friends) != 1 || pages[1].HasMore {
		t.Fatalf("pages = %+v", pages)
	}
	seen := map[string]bool{}
	for _, p := range pages {
		for _, f := range p.Friends {
			if seen[f.UserID] || f.Ver != v+1 || f.Ver != p.UntilVer {
				t.Fatalf("item %+v (untilVer %d)", f, p.UntilVer)
			}
			seen[f.UserID] = true
		}
	}
}

// TC-SOC-06-07（AC-SOC-06-5）
func TestTCSOC0607FullPaging(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	s.settings(map[string]any{"maxFriends": 5000})
	s.seedFriends("alice", "f", 2500)
	pages := s.dial("alice", "web").syncRound(0, 500)
	if len(pages) != 5 {
		t.Fatalf("%d pages, want 5", len(pages))
	}
	seen := map[string]bool{}
	for i, p := range pages {
		if !p.Full || p.UntilVer != pages[0].UntilVer || p.HasMore != (i < 4) {
			t.Fatalf("page %d: full=%v until=%d hasMore=%v", i, p.Full, p.UntilVer, p.HasMore)
		}
		for _, f := range p.Friends {
			if seen[f.UserID] {
				t.Fatalf("duplicate %s", f.UserID)
			}
			seen[f.UserID] = true
		}
	}
	if len(seen) != 2500 {
		t.Fatalf("%d friends, want 2500", len(seen))
	}
}

// TC-SOC-06-08、06-09（AC-SOC-06-6）：轮内变化不进入本轮，下一轮增量拿到。
func TestTCSOC0608ChangesDuringRound(t *testing.T) {
	for _, victim := range []string{"f1100", "f0100"} { // 排在第 3 页 / 第 1 页已返回
		t.Run(victim, func(t *testing.T) {
			s := newSocEnv(t)
			s.usersNamed("alice", "y")
			s.seedFriends("alice", "f", 1200)
			alice := s.dial("alice", "web")
			first := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 0, "limit": 500}))
			local := map[string]bool{}
			for _, f := range first.Friends {
				local[f.UserID] = true
			}
			mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": victim}))
			s.importFriends([2]string{"alice", "y"})
			cur, last := first.Cursor, first
			for last.HasMore {
				last = mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"cursor": cur, "limit": 500}))
				cur = last.Cursor
				for _, f := range last.Friends {
					if f.UserID == "y" || f.UserID == victim && victim == "f1100" {
						t.Fatalf("round contains %s", f.UserID)
					}
					local[f.UserID] = true
				}
			}
			if last.LatestVer != last.UntilVer+2 {
				t.Fatalf("latestVer %d, untilVer %d", last.LatestVer, last.UntilVer)
			}
			for _, p := range alice.syncRound(last.UntilVer, 500) {
				for _, f := range p.Friends {
					if f.Deleted {
						delete(local, f.UserID)
					} else {
						local[f.UserID] = true
					}
				}
			}
			if len(local) != 1200 || local[victim] || !local["y"] {
				t.Fatalf("final: %d friends, victim=%v y=%v", len(local), local[victim], local["y"])
			}
		})
	}
}

// TC-SOC-06-10（AC-SOC-06-7）
func TestTCSOC0610TombstoneAndPurge(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	alice := s.dial("alice", "web")
	del := mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "bob"}))
	v := del.Ver
	p := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": v - 1}))
	if p.Full || len(p.Friends) != 1 || !p.Friends[0].Deleted {
		t.Fatalf("tombstone round: %+v", p)
	}
	s.advance(31 * 24 * time.Hour)
	s.purge()
	alice = s.dial("alice", "pc")
	p = mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": v - 1}))
	if !p.Full || len(p.Friends) != 0 {
		t.Fatalf("after purge: %+v", p)
	}
	p = mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": p.LatestVer}))
	if p.Full || len(p.Friends)+len(p.Requests)+len(p.Blocks) != 0 {
		t.Fatalf("since=current: %+v", p)
	}
}

// TC-SOC-06-11（AC-SOC-06-8）
func TestTCSOC0611RestartMidRound(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave", "eve", "frank")
	s.seed(map[string]any{"requests": []any{map[string]any{"from": "carol", "to": "alice"}}, "updatedAt": t0}) // ver 1
	s.seed(map[string]any{"requests": []any{map[string]any{"from": "dave", "to": "alice"}}, "updatedAt": t0})  // ver 2
	s.advance(31 * 24 * time.Hour)
	s.importFriends([2]string{"alice", "bob"}, [2]string{"alice", "eve"}, [2]string{"alice", "frank"}) // ver 3
	alice := s.dial("alice", "web")
	p := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 1, "limit": 2}))
	if p.Full || !p.HasMore {
		t.Fatalf("first page: %+v", p)
	}
	s.purge()
	p2 := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"cursor": p.Cursor, "limit": 2}))
	if !p2.Restart || len(p2.Friends)+len(p2.Requests)+len(p2.Blocks) != 0 {
		t.Fatalf("second page should be restart: %+v", p2)
	}
	if p3 := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 1, "limit": 2})); !p3.Full {
		t.Fatalf("restart with same sinceVer should be full: %+v", p3)
	}
}

// TC-SOC-06-13（AC-SOC-06-9）
func TestTCSOC0613ConcurrentAccepts(t *testing.T) {
	s := newSocEnv(t)
	peers := []string{"p0", "p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9"}
	s.usersNamed(append([]string{"alice"}, peers...)...)
	for _, p := range peers {
		s.dial(p, "web").apply("alice", nil)
	}
	c1, c2 := s.dial("alice", "web"), s.dial("alice", "pc")
	v0 := c1.ver()
	var wg sync.WaitGroup
	for i, c := range []*client{c1, c2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, p := range peers[i*5 : i*5+5] {
				c.respond(p, "accept", nil)
			}
		}()
	}
	wg.Wait()
	c1.pending, c2.pending = nil, nil
	p := c1.syncRound(v0, 500)[0]
	if p.UntilVer != v0+10 || len(p.Friends) != 10 || len(p.Requests) != 10 {
		t.Fatalf("round: until=%d friends=%d requests=%d", p.UntilVer, len(p.Friends), len(p.Requests))
	}
	perVer := map[int64]int{}
	for _, f := range p.Friends {
		perVer[f.Ver]++
	}
	for _, r := range p.Requests {
		perVer[r.Ver]++
	}
	for v, n := range perVer {
		if n != 2 {
			t.Fatalf("ver %d has %d items, want 2", v, n)
		}
	}
	if len(perVer) != 10 {
		t.Fatalf("%d distinct vers, want 10", len(perVer))
	}
}

// TC-SOC-06-15（AC-SOC-06-10）
func TestTCSOC0615SyncValidation(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.importFriends([2]string{"alice", "bob"}, [2]string{"alice", "carol"}, [2]string{"bob", "carol"})
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	for _, d := range []map[string]any{
		{"sinceVer": -1}, {"sinceVer": 1.5}, {}, {"sinceVer": 0, "cursor": "x"},
		{"sinceVer": 0, "limit": 0}, {"sinceVer": 0, "limit": 501}, {"cursor": "garbage"},
	} {
		mustErr(t, alice.call(protocol.CmdSocialSync, d), protocol.CodeBadRequest)
	}
	p := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 0, "limit": 1}))
	tampered := p.Cursor[:len(p.Cursor)-2] + "AA"
	mustErr(t, alice.call(protocol.CmdSocialSync, map[string]any{"cursor": tampered}), protocol.CodeBadRequest)
	mustErr(t, bob.call(protocol.CmdSocialSync, map[string]any{"cursor": p.Cursor}), protocol.CodeBadRequest) // 他人游标
	if q := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 999})); !q.Full {
		t.Fatal("sinceVer > current should be full")
	}
}

// TC-SOC-06-16（AC-SOC-06-11）
func TestTCSOC0616ChangeAck(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	bv := bob.ver()
	ack := mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	if !ack.Changed || ack.Ver != bv+1 || len(ack.Items.Friends) != 1 || len(ack.Items.Requests) != 1 {
		t.Fatalf("ack = %+v", ack)
	}
	if ack.Items.Friends[0].Ver != ack.Ver || ack.Items.Requests[0].Ver != ack.Ver || ack.Items.Requests[0].Direction != "in" {
		t.Fatalf("items ver: %+v", ack.Items)
	}
	again := mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	if again.Changed || len(again.Items.Friends)+len(again.Items.Requests)+len(again.Items.Blocks) != 0 {
		t.Fatalf("repeat ack = %+v", again)
	}
}

// TC-SOC-06-19（AC-SOC-06-13）
func TestTCSOC0619FullRoundRateLimit(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	for range 10 {
		mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 0}))
	}
	mustErr(t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 0}), protocol.CodeRateLimited)
	mustErr(t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 99}), protocol.CodeRateLimited) // 服务端判定的全量同样计数
}

// TC-SOC-06-19 增量部分：增量轮不受全量限流。
func TestTCSOC0619IncrementalNotLimited(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	alice := s.dial("alice", "web")
	for range 30 {
		p := mustAck[protocol.SocialSyncAck](t, alice.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 1}))
		if p.Full {
			t.Fatal("since=1 should be incremental")
		}
	}
}

// TC-SOC-06-14 的服务端可观测部分（5.4 节第 15 行）：setPrivacy 分配 ver、通知其他连接、同步头带设置。
func TestTCSOC0614PrivacyVer(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	web, pc := s.dial("alice", "web"), s.dial("alice", "pc")
	v := web.ver()
	ack := mustAck[protocol.VerAck](t, web.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	if !ack.Changed || ack.Ver != v+1 {
		t.Fatalf("ack %+v", ack)
	}
	pc.waitSocialChanged()
	if _, last := pc.state(); last.Settings.Searchable {
		t.Fatal("pc should see searchable=false")
	}
	if again := mustAck[protocol.VerAck](t, web.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false})); again.Changed || again.Ver != v+1 {
		t.Fatalf("repeat %+v", again)
	}
	// 回到默认值也是一次变化
	if back := mustAck[protocol.VerAck](t, web.call(protocol.CmdSetPrivacy, map[string]any{"searchable": true})); !back.Changed {
		t.Fatalf("back to default %+v", back)
	}
}
