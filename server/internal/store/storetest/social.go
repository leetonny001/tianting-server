package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/store"
)

// PRD-social 8.7 节的并发测试（BE-117，B6 前置）：在 SQLite 上同样运行，但只有在 Postgres 上才有判别力；
// Postgres 实现（BE-070）上线前必须全部通过。每个用例断言：结果正确、没有死锁、没有意外错误。

const day = int64(24 * 3600 * 1000)

func socialConcurrencyCases() []struct {
	name string
	fn   func(*testing.T, Opener)
} {
	return []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"FriendMutualApplyRace", testFriendMutualApplyRace},
		{"FriendLimitRace", testFriendLimitRace},
		{"FriendMutualAcceptNoDeadlock", testFriendMutualAcceptNoDeadlock},
		{"StrangerLimitRace", testStrangerLimitRace},
		{"MemberCountRace", testMemberCountRace},
		{"AdminImportVsApplyNoDeadlock", testAdminImportVsApplyNoDeadlock},
		{"SocialVerConcurrent", testSocialVerConcurrent},
		{"GroupInviteVsDissolveRace", testGroupInviteVsDissolveRace},
		{"GroupLastTwoLeaveRace", testGroupLastTwoLeaveRace},
		{"GroupOwnerLeaveVsTransferRace", testGroupOwnerLeaveVsTransferRace},
	}
}

func mustUsers(t *testing.T, s store.Store, ids ...string) {
	t.Helper()
	us := make([]store.User, len(ids))
	for i, id := range ids {
		us[i] = store.User{UserID: id}
	}
	if err := s.UpsertUsers(context.Background(), us); err != nil {
		t.Fatal(err)
	}
}

func applyP(from, to string, now int64) store.ApplyFriendParams {
	return store.ApplyFriendParams{From: from, To: to, Source: "other", Now: now, ExpireAt: now + 7*day, MaxFriends: 2000}
}

// fullState 读出用户的全量同步数据（不分页上限）。
func fullState(t *testing.T, s store.Store, uid string) store.SocialItems {
	t.Helper()
	var all store.SocialItems
	q := store.SocialPageQuery{UID: uid, First: true, Limit: 500}
	for {
		p, err := s.SocialPage(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		all.Friends = append(all.Friends, p.Items.Friends...)
		all.Requests = append(all.Requests, p.Items.Requests...)
		all.Blocks = append(all.Blocks, p.Items.Blocks...)
		if !p.HasMore {
			return all
		}
		q = store.SocialPageQuery{UID: uid, Full: p.Full, SinceVer: p.SinceVer, UntilVer: p.UntilVer, After: p.Last, Limit: 500}
	}
}

func hasFriend(it store.SocialItems, peer string) bool {
	for _, f := range it.Friends {
		if f.Peer == peer && !f.Deleted {
			return true
		}
	}
	return false
}

// AC-SOC-02-13：100 组新用户对互相同时申请，每组都成为好友、没有遗留 pending。
func testFriendMutualApplyRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	var wg sync.WaitGroup
	for i := range 100 {
		x, y := fmt.Sprintf("x%03d", i), fmt.Sprintf("y%03d", i)
		mustUsers(t, s, x, y)
		for _, p := range [][2]string{{x, y}, {y, x}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, _, err := s.ApplyFriend(ctx, applyP(p[0], p[1], 1)); err != nil {
					t.Error(err)
				}
			}()
		}
	}
	wg.Wait()
	for i := range 100 {
		x, y := fmt.Sprintf("x%03d", i), fmt.Sprintf("y%03d", i)
		for _, pair := range [][2]string{{x, y}, {y, x}} {
			it := fullState(t, s, pair[0])
			if !hasFriend(it, pair[1]) {
				t.Fatalf("%s has no friend %s", pair[0], pair[1])
			}
			for _, r := range it.Requests {
				if r.Status != store.ReqAccepted {
					t.Fatalf("%s: request %+v left %s", pair[0], r, r.Status)
				}
			}
		}
	}
}

// AC-SOC-03-8：bob 有 1999 个好友，两个申请同时被同意，每轮恰好一个成功、一个 4132，好友数从不超过 2000。
func testFriendLimitRace(t *testing.T, open Opener) {
	for round := range 50 {
		s, ctx := open(t), context.Background()
		mustUsers(t, s, "bob", "carol", "dave")
		edges := make([]store.SeedEdge, 1999)
		for i := range edges {
			edges[i] = store.SeedEdge{Owner: "bob", Peer: fmt.Sprintf("f%04d", i)}
		}
		if _, err := s.SeedSocial(ctx, store.SocialSeed{Friends: edges, UpdatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		for _, from := range []string{"carol", "dave"} {
			if _, _, err := s.ApplyFriend(ctx, applyP(from, "bob", 1)); err != nil {
				t.Fatal(err)
			}
		}
		var mu sync.Mutex
		ok, limited := 0, 0
		var wg sync.WaitGroup
		for _, from := range []string{"carol", "dave"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.RespondFriend(ctx, store.RespondFriendParams{Me: "bob", From: from, Accept: true, Now: 2, MaxFriends: 2000})
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, store.ErrFriendLimit):
					limited++
				default:
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		st, _ := s.SocialState(ctx, "bob")
		if ok != 1 || limited != 1 || st.FriendCount != 2000 {
			t.Fatalf("round %d: ok=%d limited=%d friends=%d", round, ok, limited, st.FriendCount)
		}
	}
}

// AC-SOC-03-9：双方互有待处理申请、同时同意对方，100 轮都成为好友、各一条边、10 秒内完成。
func testFriendMutualAcceptNoDeadlock(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	start := time.Now()
	for i := range 100 {
		a, b := fmt.Sprintf("a%03d", i), fmt.Sprintf("b%03d", i)
		mustUsers(t, s, a, b)
		if _, err := s.SeedSocial(ctx, store.SocialSeed{Requests: []store.SeedRequest{{From: a, To: b, ExpireAt: 7 * day}, {From: b, To: a, ExpireAt: 7 * day}}, UpdatedAt: 1}); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for _, p := range [][2]string{{a, b}, {b, a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := s.RespondFriend(ctx, store.RespondFriendParams{Me: p[0], From: p[1], Accept: true, Now: 2, MaxFriends: 2000}); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		for _, p := range [][2]string{{a, b}, {b, a}} {
			it := fullState(t, s, p[0])
			if len(it.Friends) != 1 || !hasFriend(it, p[1]) {
				t.Fatalf("%s friends %+v", p[0], it.Friends)
			}
			for _, r := range it.Requests {
				if r.Status != store.ReqAccepted {
					t.Fatalf("%s request %+v", p[0], r)
				}
			}
		}
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("100 rounds took %v", d)
	}
}

// AC-SOC-08-12：两个写者同时给陌生人各发 10 条（限额 3），恰好 3 条成功、seq 1～3。
func testStrangerLimitRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	var mu sync.Mutex
	ok := 0
	var wg sync.WaitGroup
	for w := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 10 {
				m := newMsg(fmt.Sprintf("m%d-%d", w, i), "c2c:alice:bob", "alice", fmt.Sprintf("c%d-%d", w, i))
				m.ConvUsers, m.StrangerLimit = []string{"alice", "bob"}, 3
				_, _, err := s.SaveMessage(ctx, m)
				mu.Lock()
				switch {
				case err == nil:
					ok++
				case !errors.Is(err, store.ErrStrangerMsgLimit):
					t.Error(err)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	msgs, _, err := s.ListMessages(ctx, "c2c:alice:bob", 0, 100)
	if err != nil || ok != 3 || len(msgs) != 3 || msgs[2].Seq != 3 {
		t.Fatalf("ok=%d msgs=%d err=%v", ok, len(msgs), err)
	}
}

// AC-SOC-11-6：并发 10 个加成员、10 个移除成员，memberCount 等于实际成员数。
func testMemberCountRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	var initial []string
	for i := range 10 {
		initial = append(initial, fmt.Sprintf("old%d", i))
	}
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, initial); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := s.AddGroupMembers(ctx, "g1", []string{fmt.Sprintf("new%d", i)}, 2000, 2); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, _, err := s.RemoveGroupMembers(ctx, "g1", []string{initial[i]}, 1); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	g, _ := s.GetGroup(ctx, "g1")
	ms, _ := s.GroupMembers(ctx, "g1")
	if g.MemberCount != len(ms) || len(ms) != 10 {
		t.Fatalf("memberCount=%d members=%d", g.MemberCount, len(ms))
	}
}

// 8.3 节 L3：管理接口批量导入与申请、同意交错 1000 次，没有死锁、上限不超、计数一致。
func testAdminImportVsApplyNoDeadlock(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	users := []string{"u0", "u1", "u2", "u3", "u4", "u5"}
	mustUsers(t, s, users...)
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 250 {
				a, b := users[(i+w)%6], users[(i*5+w+1)%6]
				if a == b {
					continue
				}
				var err error
				switch (i + w) % 4 {
				case 0:
					_, _, err = s.ImportFriends(ctx, []store.FriendPair{{A: min(a, b), B: max(a, b)}}, int64(i+1), 2000)
				case 1:
					_, err = s.RemoveFriends(ctx, []store.FriendPair{{A: a, B: b}}, int64(i+1))
				case 2:
					_, _, err = s.ApplyFriend(ctx, applyP(a, b, int64(i+1)))
				default:
					_, err = s.RespondFriend(ctx, store.RespondFriendParams{Me: a, From: b, Accept: true, Now: int64(i + 1), MaxFriends: 2000})
					if errors.Is(err, store.ErrRequestNotFound) || errors.Is(err, store.ErrRequestHandled) {
						err = nil
					}
				}
				if err != nil && !errors.Is(err, store.ErrIBlockedPeer) {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if n, err := s.RepairCounters(ctx); err != nil || n != 0 {
		t.Fatalf("counters drifted: %d rows, %v", n, err)
	}
}

// AC-SOC-06-9：同一用户 8 个并发写 × 125 次，ver 1～1000 连续无重复。
func testSocialVerConcurrent(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	mustUsers(t, s, "alice")
	var mu sync.Mutex
	seen := map[int64]bool{}
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 125 {
				c, err := s.Block(ctx, "alice", fmt.Sprintf("p%d-%d", w, i), int64(i+1), 5000)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				if seen[c.Ver] {
					t.Errorf("ver %d allocated twice", c.Ver)
				}
				seen[c.Ver] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	for v := int64(1); v <= 1000; v++ {
		if !seen[v] {
			t.Fatalf("ver %d missing", v)
		}
	}
}

// ---- M4 并发（BE-137，B6 前置）----

func friendsAll(t *testing.T, s store.Store, users ...string) {
	t.Helper()
	var es []store.SeedEdge
	for _, a := range users {
		for _, b := range users {
			if a != b {
				es = append(es, store.SeedEdge{Owner: a, Peer: b})
			}
		}
	}
	mustUsers(t, s, users...)
	if _, err := s.SeedSocial(context.Background(), store.SocialSeed{Friends: es, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
}

func newClientGroup(t *testing.T, s store.Store, gid, owner string, members ...string) {
	t.Helper()
	if _, _, _, _, err := s.CreateClientGroup(context.Background(), store.CreateClientGroupParams{
		UserID: owner, ClientReqID: "req-" + gid, GroupID: gid, Members: members, MaxMembers: 500, MaxJoined: 500, Now: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

// AC-SOC-22-7：邀请与解散并发：要么邀请 ErrNotFound，要么邀请成功后被解散；不会有"已解散但还有成员"。
func testGroupInviteVsDissolveRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	friendsAll(t, s, "a", "b", "c", "d")
	for i := range 50 {
		gid := fmt.Sprintf("ug-%d", i)
		newClientGroup(t, s, gid, "a", "b", "c")
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, _, _, err := s.InviteMembers(ctx, gid, "b", []string{"d"}, 500, 2); err != nil && !errors.Is(err, store.ErrNotFound) {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := s.ClientDissolveGroup(ctx, gid, "a", 2); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		if ms, _ := s.GroupMembers(ctx, gid); len(ms) != 0 {
			t.Fatalf("dissolved group %s still has members %v", gid, ms)
		}
	}
}

// AC-SOC-23-8：自建群最后两人同时退群，每轮都解散。
func testGroupLastTwoLeaveRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	friendsAll(t, s, "a", "b", "c")
	for i := range 50 {
		gid := fmt.Sprintf("ug-%d", i)
		newClientGroup(t, s, gid, "a", "b", "c")
		if _, err := s.LeaveGroup(ctx, gid, "c", 2); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for _, u := range []string{"a", "b"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := s.LeaveGroup(ctx, gid, u, 3); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if ok, _ := s.GroupExists(ctx, gid); ok {
			t.Fatalf("%s not dissolved", gid)
		}
	}
}

// AC-SOC-23-9：群主退群与转让并发：恰好一个群主，且是成员。
func testGroupOwnerLeaveVsTransferRace(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	friendsAll(t, s, "a", "b", "c")
	for i := range 50 {
		gid := fmt.Sprintf("ug-%d", i)
		newClientGroup(t, s, gid, "a", "b", "c")
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := s.LeaveGroup(ctx, gid, "a", 2); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := s.TransferOwner(ctx, gid, "a", "b", 2)
			if err != nil && !errors.Is(err, store.ErrNotGroupMember) && !errors.Is(err, store.ErrGroupForbidden) {
				t.Error(err)
			}
		}()
		wg.Wait()
		g, err := s.GetGroup(ctx, gid)
		if err != nil {
			t.Fatal(err)
		}
		rows, _, err := s.ListGroupMembers(ctx, gid, nil, 100)
		if err != nil {
			t.Fatal(err)
		}
		owners := 0
		for _, r := range rows {
			if r.Role == store.RoleOwner {
				owners++
				if r.UserID != g.OwnerID {
					t.Fatalf("owner row %s, chat_groups.owner_id %s", r.UserID, g.OwnerID)
				}
			}
		}
		if owners != 1 {
			t.Fatalf("%s has %d owners: %+v", gid, owners, rows)
		}
	}
}
