// Package storetest 是 store.Store 的契约测试（ARCH 3.2）：任何存储实现（SQLite、Postgres……）
// 都必须通过同一套用例，才能证明换库后上层行为不变。
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/store"
)

// Opener 为每个用例打开一个全新的、空的、已迁移的存储，并负责在用例结束时关闭。
type Opener func(t *testing.T) store.Store

// Run 运行全部契约用例。
func Run(t *testing.T, open Opener) {
	for _, tc := range []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"SaveMessageAssignsSeqPerConversation", testSaveMessageAssignsSeqPerConversation},
		{"SaveMessageDeduplicates", testSaveMessageDeduplicates},
		{"SaveMessageConcurrentSeqIsContiguous", testSaveMessageConcurrentSeqIsContiguous},
		{"ListMessagesPaging", testListMessagesPaging},
		{"Groups", testGroups},
		{"GroupSettings", testGroupSettings},
		{"GroupJoinSeq", testGroupJoinSeq},
		{"GroupMemberLimit", testGroupMemberLimit},
		{"MissingUsers", testMissingUsers},
		{"ReadSeqOnlyMovesForward", testReadSeqOnlyMovesForward},
		{"ListMessagesBefore", testListMessagesBefore},
		{"GroupExists", testGroupExists},
		{"Users", testUsers},
		{"FindByClientMsgID", testFindByClientMsgID},
		{"BodyRoundTrip", testBodyRoundTrip},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, open) })
	}
}

func newMsg(id, conv, from, clientID string) store.Message {
	return store.Message{MsgID: id, ClientMsgID: clientID, ConvID: conv, From: from, Type: "text", Body: []byte(`{"text":"hi"}`), TS: 1}
}

func testSaveMessageAssignsSeqPerConversation(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()

	for i, tc := range []struct {
		conv    string
		wantSeq int64
	}{{"c2c:a:b", 1}, {"c2c:a:b", 2}, {"group:g1", 1}, {"c2c:a:b", 3}} {
		m, dup, err := s.SaveMessage(ctx, newMsg(fmt.Sprint("m", i), tc.conv, "a", fmt.Sprint("client-0", i)))
		if err != nil || dup {
			t.Fatalf("save %d: dup=%v err=%v", i, dup, err)
		}
		if m.Seq != tc.wantSeq {
			t.Errorf("save %d in %s: seq=%d want %d", i, tc.conv, m.Seq, tc.wantSeq)
		}
	}
}

func testSaveMessageDeduplicates(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()

	first, _, err := s.SaveMessage(ctx, newMsg("m1", "c2c:a:b", "a", "client-01"))
	if err != nil {
		t.Fatal(err)
	}
	// 重发：服务端生成了新 msgId，但 clientMsgId 相同 → 返回第一次的结果。
	again, dup, err := s.SaveMessage(ctx, newMsg("m2", "c2c:a:b", "a", "client-01"))
	if err != nil {
		t.Fatal(err)
	}
	if !dup || again.MsgID != "m1" || again.Seq != first.Seq {
		t.Fatalf("dup=%v again=%+v first=%+v", dup, again, first)
	}
	// 不同发送者可以用同一个 clientMsgId。
	if _, dup, _ := s.SaveMessage(ctx, newMsg("m3", "c2c:a:b", "b", "client-01")); dup {
		t.Error("different sender must not be treated as duplicate")
	}
	msgs, _, _ := s.ListMessages(ctx, "c2c:a:b", 0, 10)
	if len(msgs) != 2 {
		t.Errorf("stored %d messages, want 2", len(msgs))
	}
}

func testSaveMessageConcurrentSeqIsContiguous(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	const n = 50

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.SaveMessage(ctx, newMsg(fmt.Sprint("m", i), "group:g", fmt.Sprint("u", i), "client-xx"))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	msgs, hasMore, err := s.ListMessages(ctx, "group:g", 0, n)
	if err != nil || hasMore || len(msgs) != n {
		t.Fatalf("len=%d hasMore=%v err=%v", len(msgs), hasMore, err)
	}
	for i, m := range msgs {
		if m.Seq != int64(i+1) {
			t.Fatalf("msgs[%d].Seq = %d, want %d (seq must be contiguous)", i, m.Seq, i+1)
		}
	}
}

func testListMessagesPaging(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i := range 5 {
		s.SaveMessage(ctx, newMsg(fmt.Sprint("m", i), "c2c:a:b", "a", fmt.Sprint("client-0", i)))
	}

	page, more, _ := s.ListMessages(ctx, "c2c:a:b", 0, 2)
	if len(page) != 2 || !more || page[0].Seq != 1 {
		t.Fatalf("page1 len=%d more=%v", len(page), more)
	}
	page, more, _ = s.ListMessages(ctx, "c2c:a:b", 2, 10)
	if len(page) != 3 || more || page[0].Seq != 3 {
		t.Fatalf("page2 len=%d more=%v", len(page), more)
	}
	page, more, _ = s.ListMessages(ctx, "c2c:x:y", 0, 10)
	if len(page) != 0 || more {
		t.Fatalf("unknown conv len=%d", len(page))
	}
}

func testGroups(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()

	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "Group 1"}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "dup"}, nil); !errors.Is(err, store.ErrExists) {
		t.Errorf("duplicate create: %v", err)
	}
	added, err := s.AddGroupMembers(ctx, "g1", []string{"b", "c"}, 2000)
	if err != nil || fmt.Sprint(added) != "[c]" {
		t.Fatalf("add: added=%v err=%v (b is already a member)", added, err)
	}
	if _, err := s.AddGroupMembers(ctx, "nope", []string{"a"}, 2000); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("add to missing group: %v", err)
	}
	members, _ := s.GroupMembers(ctx, "g1")
	if fmt.Sprint(members) != "[a b c]" {
		t.Errorf("members = %v", members)
	}
	if _, ok, _ := s.GroupMember(ctx, "g1", "c"); !ok {
		t.Error("c should be member")
	}
	if _, ok, _ := s.GroupMember(ctx, "g1", "z"); ok {
		t.Error("z should not be member")
	}

	removed, err := s.RemoveGroupMembers(ctx, "g1", []string{"b", "z"})
	if err != nil || fmt.Sprint(removed) != "[b]" {
		t.Fatalf("remove: removed=%v err=%v (z is not a member)", removed, err)
	}
	if _, err := s.RemoveGroupMembers(ctx, "nope", []string{"a"}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("remove from missing group: %v", err)
	}
	if members, _ := s.GroupMembers(ctx, "g1"); fmt.Sprint(members) != "[a c]" {
		t.Errorf("members after remove = %v", members)
	}
}

func testGroupSettings(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "初始", HistoryVisible: true}, nil)
	g, err := s.GetGroup(ctx, "g1")
	if err != nil || g.Name != "初始" || !g.HistoryVisible || g.CreatedAt == 0 {
		t.Fatalf("GetGroup = %+v, %v", g, err)
	}
	name, off := "改名", false
	if err := s.UpdateGroup(ctx, "g1", store.GroupUpdate{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.GetGroup(ctx, "g1"); g.Name != "改名" || !g.HistoryVisible {
		t.Errorf("after name update: %+v (historyVisible must be unchanged)", g)
	}
	s.UpdateGroup(ctx, "g1", store.GroupUpdate{HistoryVisible: &off})
	if g, _ := s.GetGroup(ctx, "g1"); g.Name != "改名" || g.HistoryVisible {
		t.Errorf("after visibility update: %+v", g)
	}
	if _, err := s.GetGroup(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get missing: %v", err)
	}
	if err := s.UpdateGroup(ctx, "nope", store.GroupUpdate{Name: &name}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}
}

// JoinSeq = 入群时会话的 maxSeq；重复添加不重置；移除后再加入按新的 maxSeq 计算（PRD F-GRP-01 规则 6、7，C-36）。
func testGroupJoinSeq(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g"}, []string{"a"})
	send := func(n int, prefix string) {
		for i := range n {
			if _, _, err := s.SaveMessage(ctx, newMsg(fmt.Sprintf("%s%d", prefix, i), protocol.GroupConvID("g1"), "a", fmt.Sprintf("%s-%04d", prefix, i))); err != nil {
				t.Fatal(err)
			}
		}
	}
	joinSeq := func(uid string) int64 {
		m, ok, err := s.GroupMember(ctx, "g1", uid)
		if err != nil || !ok {
			t.Fatalf("%s not a member: %v", uid, err)
		}
		return m.JoinSeq
	}
	if joinSeq("a") != 0 {
		t.Errorf("initial member joinSeq = %d", joinSeq("a"))
	}
	send(10, "m")
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000)
	if joinSeq("d") != 10 {
		t.Errorf("d joinSeq = %d, want 10", joinSeq("d"))
	}
	send(5, "n")
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000) // 幂等：不重置
	if joinSeq("d") != 10 {
		t.Errorf("re-adding an existing member reset joinSeq to %d", joinSeq("d"))
	}
	s.RemoveGroupMembers(ctx, "g1", []string{"d"})
	send(2, "o")
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000)
	if joinSeq("d") != 17 {
		t.Errorf("rejoin joinSeq = %d, want 17", joinSeq("d"))
	}
}

// 总人数上限在事务内检查：超出时一人都不加；并发加人也不能突破上限。
func testGroupMemberLimit(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g"}, []string{"a", "b", "c"})
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"d", "e"}, 4); !errors.Is(err, store.ErrLimitExceeded) {
		t.Fatalf("over limit: %v", err)
	}
	if members, _ := s.GroupMembers(ctx, "g1"); len(members) != 3 {
		t.Fatalf("partial add on limit error: %v", members)
	}
	// 重复成员不占名额。
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"a", "d"}, 4); err != nil {
		t.Fatalf("exactly at limit: %v", err)
	}

	s.CreateGroup(ctx, store.Group{GroupID: "race", Name: "g"}, nil)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AddGroupMembers(ctx, "race", []string{fmt.Sprintf("u%da", i), fmt.Sprintf("u%db", i)}, 5)
		}()
	}
	wg.Wait()
	if members, _ := s.GroupMembers(ctx, "race"); len(members) > 5 {
		t.Errorf("concurrent adds exceeded the limit: %d members", len(members))
	}
}

func testMissingUsers(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.UpsertUsers(ctx, []store.User{{UserID: "a"}, {UserID: "c"}})
	missing, err := s.MissingUsers(ctx, []string{"a", "x", "c", "y"})
	if err != nil || fmt.Sprint(missing) != "[x y]" {
		t.Errorf("missing = %v, %v", missing, err)
	}
}

func testReadSeqOnlyMovesForward(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.SetReadSeq(ctx, "a", "c2c:a:b", 5)
	s.SetReadSeq(ctx, "a", "c2c:a:b", 3)
	if got, _ := s.ReadSeq(ctx, "a", "c2c:a:b"); got != 5 {
		t.Errorf("read seq = %d, want 5", got)
	}
	if got, _ := s.ReadSeq(ctx, "a", "group:none"); got != 0 {
		t.Errorf("unknown read seq = %d", got)
	}
}

func testListMessagesBefore(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i := range 100 {
		s.SaveMessage(ctx, newMsg(fmt.Sprint("m", i), "c2c:a:b", "a", fmt.Sprintf("client-%03d", i)))
	}

	// 打开会话：beforeSeq = maxSeq + 1，取最新 20 条，升序返回。
	page, more, err := s.ListMessagesBefore(ctx, "c2c:a:b", 101, 1, 20)
	if err != nil || len(page) != 20 || !more || page[0].Seq != 81 || page[19].Seq != 100 {
		t.Fatalf("latest page: len=%d more=%v first=%d err=%v", len(page), more, page[0].Seq, err)
	}
	// 向上翻到最早一页。
	page, more, _ = s.ListMessagesBefore(ctx, "c2c:a:b", 21, 1, 20)
	if len(page) != 20 || more || page[0].Seq != 1 {
		t.Fatalf("first page: len=%d more=%v first=%d", len(page), more, page[0].Seq)
	}
	// minSeq 限制可见范围：只能看到 seq >= 91。
	page, more, _ = s.ListMessagesBefore(ctx, "c2c:a:b", 101, 91, 20)
	if len(page) != 10 || more || page[0].Seq != 91 {
		t.Fatalf("minSeq page: len=%d more=%v first=%d", len(page), more, page[0].Seq)
	}
}

func testGroupExists(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.CreateGroup(ctx, store.Group{GroupID: "empty", Name: "空群"}, nil)
	if ok, _ := s.GroupExists(ctx, "empty"); !ok {
		t.Error("group with no members must exist")
	}
	if ok, _ := s.GroupExists(ctx, "nope"); ok {
		t.Error("nope must not exist")
	}
}

func testUsers(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.UpsertUsers(ctx, []store.User{{UserID: "a", Nickname: "阿"}, {UserID: "b"}}); err != nil {
		t.Fatal(err)
	}
	s.UpsertUsers(ctx, []store.User{{UserID: "a", Nickname: "改名", Avatar: "http://x/a.png"}})
	u, err := s.GetUser(ctx, "a")
	if err != nil || u.Nickname != "改名" || u.Avatar != "http://x/a.png" {
		t.Fatalf("GetUser = %+v, %v", u, err)
	}
	if _, err := s.GetUser(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing user: %v", err)
	}

	s.EnsureUser(ctx, "c")
	s.EnsureUser(ctx, "a") // 已存在：不覆盖昵称
	if u, _ := s.GetUser(ctx, "a"); u.Nickname != "改名" {
		t.Error("EnsureUser must not overwrite existing user")
	}
	if ok, _ := s.UserExists(ctx, "c"); !ok {
		t.Error("c should exist after EnsureUser")
	}

	if v, _ := s.TokenVersion(ctx, "a"); v != 0 {
		t.Errorf("initial token version = %d", v)
	}
	s.RevokeTokens(ctx, "a")
	s.RevokeTokens(ctx, "a")
	if v, _ := s.TokenVersion(ctx, "a"); v != 2 {
		t.Errorf("token version = %d, want 2", v)
	}
	if err := s.RevokeTokens(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoke missing user: %v", err)
	}
}

func testFindByClientMsgID(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	saved, _, err := s.SaveMessage(ctx, newMsg("m1", "c2c:a:b", "a", "client-01"))
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := s.FindByClientMsgID(ctx, "a", "client-01")
	if err != nil || !found || got.MsgID != saved.MsgID || got.Seq != saved.Seq || got.ConvID != saved.ConvID {
		t.Fatalf("find = %+v found=%v err=%v, want %+v", got, found, err, saved)
	}
	if _, found, err := s.FindByClientMsgID(ctx, "b", "client-01"); err != nil || found {
		t.Errorf("other sender: found=%v err=%v", found, err)
	}
}

// 消息正文原样保存：中文、emoji、转义字符不能被改写。
func testBodyRoundTrip(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	m := newMsg("m1", "c2c:a:b", "a", "client-01")
	m.Body = []byte(`{"text":"你好 👋 \"引号\" \u200b"}`)
	if _, _, err := s.SaveMessage(ctx, m); err != nil {
		t.Fatal(err)
	}
	msgs, _, _ := s.ListMessages(ctx, "c2c:a:b", 0, 1)
	if len(msgs) != 1 || string(msgs[0].Body) != string(m.Body) {
		t.Fatalf("body = %s, want %s", msgs[0].Body, m.Body)
	}
}
