// Package storetest 是 store.Store 的契约测试（ARCH 3.2）：任何存储实现（SQLite、Postgres……）
// 都必须通过同一套用例，才能证明换库后上层行为不变。
package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
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
		{"Conversations", testConversations},
		{"ConversationVisibility", testConversationVisibility},
		{"ConversationLimit", testConversationLimit},
		{"GroupConversations", testGroupConversations},
		{"SpeakingMarksRead", testSpeakingMarksRead},
		{"ReadSeqOnlyMovesForward", testReadSeqOnlyMovesForward},
		{"ListMessagesBefore", testListMessagesBefore},
		{"GroupExists", testGroupExists},
		{"Users", testUsers},
		{"FindByClientMsgID", testFindByClientMsgID},
		{"BodyRoundTrip", testBodyRoundTrip},
		{"AdminAuditAppendAndQuery", testAdminAuditAppendAndQuery},
		{"MemberCountMaintained", testMemberCountMaintained},
		{"SetNickname", testSetNickname},
	} {
		t.Run(tc.name, func(t *testing.T) { tc.fn(t, open) })
	}
	for _, tc := range socialConcurrencyCases() {
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
	added, err := s.AddGroupMembers(ctx, "g1", []string{"b", "c"}, 2000, nowMs())
	if err != nil || fmt.Sprint(added) != "[c]" {
		t.Fatalf("add: added=%v err=%v (b is already a member)", added, err)
	}
	if _, err := s.AddGroupMembers(ctx, "nope", []string{"a"}, 2000, nowMs()); !errors.Is(err, store.ErrNotFound) {
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

	removed, _, err := s.RemoveGroupMembers(ctx, "g1", []string{"b", "z"}, 1)
	if err != nil || fmt.Sprint(removed) != "[b]" {
		t.Fatalf("remove: removed=%v err=%v (z is not a member)", removed, err)
	}
	if _, _, err := s.RemoveGroupMembers(ctx, "nope", []string{"a"}, 1); !errors.Is(err, store.ErrNotFound) {
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
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000, nowMs())
	if joinSeq("d") != 10 {
		t.Errorf("d joinSeq = %d, want 10", joinSeq("d"))
	}
	send(5, "n")
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000, nowMs()) // 幂等：不重置
	if joinSeq("d") != 10 {
		t.Errorf("re-adding an existing member reset joinSeq to %d", joinSeq("d"))
	}
	s.RemoveGroupMembers(ctx, "g1", []string{"d"}, 1)
	send(2, "o")
	s.AddGroupMembers(ctx, "g1", []string{"d"}, 2000, nowMs())
	if joinSeq("d") != 17 {
		t.Errorf("rejoin joinSeq = %d, want 17", joinSeq("d"))
	}
}

// 总人数上限在事务内检查：超出时一人都不加；并发加人也不能突破上限。
func testGroupMemberLimit(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g"}, []string{"a", "b", "c"})
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"d", "e"}, 4, nowMs()); !errors.Is(err, store.ErrLimitExceeded) {
		t.Fatalf("over limit: %v", err)
	}
	if members, _ := s.GroupMembers(ctx, "g1"); len(members) != 3 {
		t.Fatalf("partial add on limit error: %v", members)
	}
	// 重复成员不占名额。
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"a", "d"}, 4, nowMs()); err != nil {
		t.Fatalf("exactly at limit: %v", err)
	}

	s.CreateGroup(ctx, store.Group{GroupID: "race", Name: "g"}, nil)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AddGroupMembers(ctx, "race", []string{fmt.Sprintf("u%da", i), fmt.Sprintf("u%db", i)}, 5, nowMs())
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

// 已读只前进不后退；超过 maxSeq 按 maxSeq 记（PRD F-CONV-02 规则 4）；返回值说明是否前进。
func testReadSeqOnlyMovesForward(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i := range 10 {
		s.SaveMessage(ctx, newMsg(fmt.Sprint("m", i), "c2c:a:b", "b", fmt.Sprintf("client-%03d", i)))
	}
	if seq, adv, err := s.SetReadSeq(ctx, "a", "c2c:a:b", 5); err != nil || seq != 5 || !adv {
		t.Fatalf("read 5: seq=%d advanced=%v err=%v", seq, adv, err)
	}
	if seq, adv, _ := s.SetReadSeq(ctx, "a", "c2c:a:b", 3); seq != 5 || adv {
		t.Errorf("read 3 after 5: seq=%d advanced=%v, want 5 false", seq, adv)
	}
	if seq, adv, _ := s.SetReadSeq(ctx, "a", "c2c:a:b", 5); seq != 5 || adv {
		t.Errorf("repeat read 5: seq=%d advanced=%v, want 5 false", seq, adv)
	}
	if seq, adv, _ := s.SetReadSeq(ctx, "a", "c2c:a:b", 999); seq != 10 || !adv {
		t.Errorf("read 999 with maxSeq 10: seq=%d advanced=%v, want 10 true", seq, adv)
	}
	if seq, adv, _ := s.SetReadSeq(ctx, "a", "c2c:a:b", 999); seq != 10 || adv {
		t.Errorf("read 999 again: seq=%d advanced=%v, want 10 false (clamped value unchanged)", seq, adv)
	}
	if got, _ := s.ReadSeq(ctx, "a", "c2c:a:b"); got != 10 {
		t.Errorf("read seq = %d, want 10", got)
	}
	if got, _ := s.ReadSeq(ctx, "a", "group:none"); got != 0 {
		t.Errorf("unknown read seq = %d", got)
	}
	if seq, adv, _ := s.SetReadSeq(ctx, "a", "group:empty", 3); seq != 0 || adv {
		t.Errorf("read in a conversation without messages: seq=%d advanced=%v", seq, adv)
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

func nowMs() int64 { return time.Now().UnixMilli() }

// saveAt 以指定时间保存一条消息；c2c 会话自动带上双方。
func saveAt(t *testing.T, s store.Store, conv, from, cid string, ts int64, text string) store.Message {
	t.Helper()
	m := newMsg(cid, conv, from, cid)
	m.TS, m.Summary = ts, text
	if c, err := protocol.ParseConvID(conv); err == nil && c.Type == protocol.ConvC2C {
		m.ConvUsers = []string{c.A, c.B}
		if c.A == c.B {
			m.ConvUsers = []string{c.A}
		}
	}
	saved, _, err := s.SaveMessage(ctx(), m)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func ctx() context.Context { return context.Background() }

func convIDs(cs []store.ConvRow) string {
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.ConvID
	}
	return fmt.Sprint(ids)
}

// 会话列表：单聊对双方可见、自聊、群（含无消息的群）；排序、最后一条消息、发言即已读。
func testConversations(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	s.UpsertUsers(c, []store.User{{UserID: "b", Nickname: "小b"}})
	saveAt(t, s, "c2c:a:b", "b", "m1", 100, "hi a")
	saveAt(t, s, "c2c:a:c", "a", "m2", 300, "hi c")
	saveAt(t, s, "c2c:a:a", "a", "m3", 50, "note to self")
	s.CreateGroup(c, store.Group{GroupID: "g1", Name: "空群", CreatedAt: 200}, []string{"a"})
	s.CreateGroup(c, store.Group{GroupID: "g2", Name: "热闹群", CreatedAt: 10}, []string{"a", "b"})
	saveAt(t, s, "group:g2", "b", "m4", 400, "group hi")

	convs, truncated, err := s.ListConversations(c, "a", 500)
	if err != nil || truncated {
		t.Fatalf("list: truncated=%v err=%v", truncated, err)
	}
	if got := convIDs(convs); got != "[group:g2 c2c:a:c group:g1 c2c:a:b c2c:a:a]" {
		t.Fatalf("order = %s (by updatedAt desc; empty group sorted by join time)", got)
	}
	byID := map[string]store.ConvRow{}
	for _, cv := range convs {
		byID[cv.ConvID] = cv
	}
	ab := byID["c2c:a:b"]
	if ab.PeerID != "b" || ab.PeerNickname != "小b" || ab.MaxSeq != 1 || ab.ReadSeq != 0 || !ab.LastVisible ||
		ab.Last.From != "b" || ab.Last.Text != "hi a" || ab.Last.MsgID != "m1" || ab.UpdatedAt != 100 {
		t.Errorf("c2c:a:b = %+v", ab)
	}
	if ac := byID["c2c:a:c"]; ac.ReadSeq != 1 || ac.PeerID != "c" {
		t.Errorf("sender's read seq must advance on send (发言即已读): %+v", ac)
	}
	if self := byID["c2c:a:a"]; self.PeerID != "a" {
		t.Errorf("self chat peer = %q", self.PeerID)
	}
	if g1 := byID["group:g1"]; g1.GroupName != "空群" || g1.LastVisible || g1.MaxSeq != 0 || g1.UpdatedAt != 200 {
		t.Errorf("empty group = %+v", g1)
	}
	// 接收方视角：b 的列表里有 c2c:a:b（对方是 a）和 g2；c 的列表里有 c2c:a:c。
	if convs, _, _ := s.ListConversations(c, "b", 500); convIDs(convs) != "[group:g2 c2c:a:b]" {
		t.Errorf("b's list = %s", convIDs(convs))
	}
	if cv, ok, _ := s.GetConversation(c, "c", "c2c:a:c"); !ok || cv.PeerID != "a" || cv.ReadSeq != 0 {
		t.Errorf("c's view of c2c:a:c = %+v ok=%v", cv, ok)
	}
	if _, ok, _ := s.GetConversation(c, "c", "group:g2"); ok {
		t.Error("non-member must not see the group conversation")
	}

	// 同一时间按 convId 字节序升序。
	saveAt(t, s, "c2c:a:z", "a", "m5", 500, "z")
	saveAt(t, s, protocol.C2CConvID("a", "B"), "a", "m6", 500, "B") // "c2c:B:a"：大写字母的字节序更小
	if convs, _, _ := s.ListConversations(c, "a", 2); convIDs(convs) != "[c2c:B:a c2c:a:z]" {
		t.Errorf("tie order = %s, want byte order", convIDs(convs))
	}

	// 被移除后不再出现在列表里。
	s.RemoveGroupMembers(c, "g2", []string{"a"}, 1)
	if _, ok, _ := s.GetConversation(c, "a", "group:g2"); ok {
		t.Error("removed member still has the group conversation")
	}
}

// historyVisible=false 时，入群前的最后一条消息不可展示，按入群时间排序；设置改为 true 后立即可见。
func testConversationVisibility(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	s.CreateGroup(c, store.Group{GroupID: "g1", Name: "g", CreatedAt: 1}, []string{"a"})
	for i := range 10 {
		saveAt(t, s, "group:g1", "a", fmt.Sprintf("m%02d", i), int64(100+i), "secret before d joined")
	}
	s.AddGroupMembers(c, "g1", []string{"d"}, 2000, 500)
	cv, ok, _ := s.GetConversation(c, "d", "group:g1")
	if !ok || cv.JoinSeq != 10 || cv.MaxSeq != 10 || cv.LastVisible || cv.UpdatedAt != 500 {
		t.Fatalf("hidden history: %+v", cv)
	}
	on := true
	s.UpdateGroup(c, "g1", store.GroupUpdate{HistoryVisible: &on})
	if cv, _, _ := s.GetConversation(c, "d", "group:g1"); !cv.LastVisible || cv.UpdatedAt != 109 || !cv.HistoryVisible {
		t.Errorf("after historyVisible=true: %+v", cv)
	}
	off := false
	s.UpdateGroup(c, "g1", store.GroupUpdate{HistoryVisible: &off})
	saveAt(t, s, "group:g1", "a", "m10", 600, "after join")
	if cv, _, _ := s.GetConversation(c, "d", "group:g1"); !cv.LastVisible || cv.Last.Text != "after join" || cv.UpdatedAt != 600 {
		t.Errorf("message after joining: %+v", cv)
	}
}

// 超过 limit 时返回最近更新的 limit 个并标记截断；恰好 limit 个不截断。
func testConversationLimit(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := range 5 {
		saveAt(t, s, fmt.Sprintf("c2c:a:u%d", i), "a", fmt.Sprintf("m%d", i), int64(100+i), "x")
	}
	if convs, truncated, _ := s.ListConversations(c, "a", 5); len(convs) != 5 || truncated {
		t.Errorf("exactly at limit: %d truncated=%v", len(convs), truncated)
	}
	convs, truncated, _ := s.ListConversations(c, "a", 4)
	if len(convs) != 4 || !truncated || convs[3].ConvID != "c2c:a:u1" {
		t.Errorf("over limit: %s truncated=%v", convIDs(convs), truncated)
	}
	// 最旧的会话有了新消息，重新进入前 4，原来第 4 名被挤出。
	saveAt(t, s, "c2c:a:u0", "a", "m9", 999, "back")
	if convs, _, _ := s.ListConversations(c, "a", 4); convIDs(convs) != "[c2c:a:u0 c2c:a:u4 c2c:a:u3 c2c:a:u2]" {
		t.Errorf("after update: %s", convIDs(convs))
	}
}

// GroupConversations 的结果必须与逐个 GetConversation 一致；不是成员的不返回。
func testGroupConversations(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	s.CreateGroup(c, store.Group{GroupID: "g1", Name: "g", CreatedAt: 1}, []string{"a", "b"})
	saveAt(t, s, "group:g1", "a", "m1", 100, "hi")
	s.AddGroupMembers(c, "g1", []string{"d"}, 2000, 200)
	got, err := s.GroupConversations(c, "g1", []string{"a", "d", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows (%v), want a and d", len(got), got)
	}
	for _, uid := range []string{"a", "d"} {
		want, ok, _ := s.GetConversation(c, uid, "group:g1")
		if !ok || fmt.Sprint(got[uid]) != fmt.Sprint(want) {
			t.Errorf("%s: batch %+v, single %+v", uid, got[uid], want)
		}
	}
}

// 发言即已读：发送者已经有已读记录（之前读过一部分）时，发消息也要把已读推进到新消息。
func testSpeakingMarksRead(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	saveAt(t, s, "c2c:a:b", "b", "m1", 1, "x")
	saveAt(t, s, "c2c:a:b", "b", "m2", 2, "x")
	s.SetReadSeq(c, "a", "c2c:a:b", 1)
	saveAt(t, s, "c2c:a:b", "b", "m3", 3, "x")
	saveAt(t, s, "c2c:a:b", "a", "m4", 4, "reply")
	if got, _ := s.ReadSeq(c, "a", "c2c:a:b"); got != 4 {
		t.Errorf("sender read seq = %d, want 4", got)
	}
	if got, _ := s.ReadSeq(c, "b", "c2c:a:b"); got != 3 {
		t.Errorf("other side read seq = %d, want 3 (b's own last message)", got)
	}
}

// PRD-admin F-KEY-04：审计记录追加后可以按时间范围、调用方、操作人、请求 ID 查询，按 id 倒序分页。
func testAdminAuditAppendAndQuery(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i := 1; i <= 5; i++ {
		caller, op := "console", "admin:a1"
		if i%2 == 0 {
			caller, op = "legacy", ""
		}
		e := store.AdminAudit{TS: int64(i * 1000), Caller: caller, Operator: op, RequestID: fmt.Sprintf("r-%d", i),
			Route: "POST /admin/users/{userId}/force-logout", TargetsJSON: `{"ids":["u1"],"count":1}`, Phase: store.AuditIntent}
		if err := s.AppendAdminAudit(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	all, next, err := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 0, ToMs: 10_000, Limit: 2})
	if err != nil || len(all) != 2 || all[0].RequestID != "r-5" || all[1].RequestID != "r-4" || next == 0 {
		t.Fatalf("page 1 = %+v next=%d err=%v", all, next, err)
	}
	page2, next2, _ := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 0, ToMs: 10_000, Limit: 2, Cursor: next})
	if len(page2) != 2 || page2[0].RequestID != "r-3" || next2 == 0 {
		t.Fatalf("page 2 = %+v next=%d", page2, next2)
	}
	page3, next3, _ := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 0, ToMs: 10_000, Limit: 2, Cursor: next2})
	if len(page3) != 1 || page3[0].RequestID != "r-1" || next3 != 0 {
		t.Fatalf("page 3 = %+v next=%d", page3, next3)
	}
	if got, _, _ := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 2000, ToMs: 4000}); len(got) != 3 {
		t.Errorf("time range [2000,4000] = %d entries, want 3", len(got))
	}
	if got, _, _ := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 0, ToMs: 10_000, Caller: "legacy"}); len(got) != 2 || got[0].Operator != "" {
		t.Errorf("caller=legacy = %+v", got)
	}
	got, _, _ := s.QueryAdminAudit(ctx, store.AdminAuditQuery{FromMs: 0, ToMs: 10_000, RequestID: "r-3", Operator: "admin:a1"})
	if len(got) != 1 || got[0].TargetsJSON != `{"ids":["u1"],"count":1}` || got[0].SummaryJSON != "{}" || got[0].Phase != store.AuditIntent {
		t.Errorf("by requestId = %+v", got)
	}
}

// BE-113 第 3 条：建群、加人、移出后 member_count 与成员数一致，会话列表和群查询都带上。
func testMemberCountMaintained(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		g, err := s.GetGroup(ctx, "g1")
		if err != nil || g.MemberCount != want {
			t.Fatalf("GetGroup memberCount = %d (err %v), want %d", g.MemberCount, err, want)
		}
		convs, _, _ := s.ListConversations(ctx, "a", 10)
		if len(convs) != 1 || convs[0].MemberCount != want {
			t.Fatalf("conversation memberCount = %+v, want %d", convs, want)
		}
	}
	check(2)
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"c", "d", "a"}, 2000, 2); err != nil {
		t.Fatal(err)
	}
	check(4)
	if _, err := s.AddGroupMembers(ctx, "g1", []string{"e", "f"}, 5, 3); !errors.Is(err, store.ErrLimitExceeded) {
		t.Fatalf("over limit: %v", err)
	}
	check(4) // 超过上限整批回滚，成员数不变
	if _, _, err := s.RemoveGroupMembers(ctx, "g1", []string{"b", "x"}, 1); err != nil {
		t.Fatal(err)
	}
	check(3)
}

// BE-091：只改昵称；用户不存在返回 ErrNotFound。
func testSetNickname(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.SetNickname(ctx, "ghost", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing user: %v", err)
	}
	s.EnsureUser(ctx, "u1")
	if err := s.SetNickname(ctx, "u1", "张三"); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.GetUser(ctx, "u1"); u.Nickname != "张三" {
		t.Fatalf("nickname = %q", u.Nickname)
	}
}
