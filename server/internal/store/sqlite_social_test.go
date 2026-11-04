package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TC-SOC-11-06（AC-SOC-11-5）、BE-100：在只有 0003 数据的库上执行到最新：成员数、单聊 sent_count 回填正确。
func TestMigration0006SocialBackfill(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)
	all, err := SQLiteMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all[:3]); err != nil {
		t.Fatal(err)
	}
	for g, n := range map[string]int{"g2": 2, "g10": 10, "g2000": 2000} {
		if _, err := db.Exec(`INSERT INTO chat_groups (group_id, name, created_at) VALUES (?, ?, 1)`, g, g); err != nil {
			t.Fatal(err)
		}
		for i := range n {
			if _, err := db.Exec(`INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, 1)`, g, fmt.Sprintf("u%d", i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 单聊：alice 发 3 条、bob 发 1 条（user_conv 由 0003 按 messages 回填）。
	for i, from := range []string{"alice", "alice", "bob", "alice"} {
		if _, err := db.Exec(`INSERT INTO messages (msg_id, conv_id, seq, from_uid, client_msg_id, type, body, ts) VALUES (?, 'c2c:alice:bob', ?, ?, ?, 'text', '{}', 1)`,
			fmt.Sprintf("m%d", i), i+1, from, fmt.Sprintf("c%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO user_conv (user_id, conv_id, peer_id, created_at) VALUES ('alice', 'c2c:alice:bob', 'bob', 1), ('bob', 'c2c:alice:bob', 'alice', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all); err != nil {
		t.Fatal(err)
	}
	for g, want := range map[string]int{"g2": 2, "g10": 10, "g2000": 2000} {
		var n int
		db.QueryRow(`SELECT member_count FROM chat_groups WHERE group_id = ?`, g).Scan(&n)
		if n != want {
			t.Errorf("%s member_count = %d, want %d", g, n, want)
		}
	}
	for uid, want := range map[string]int{"alice": 3, "bob": 1} {
		var n int
		db.QueryRow(`SELECT sent_count FROM user_conv WHERE user_id = ?`, uid).Scan(&n)
		if n != want {
			t.Errorf("%s sent_count = %d, want %d", uid, n, want)
		}
	}
}

func openTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "s.db"), SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TC-SOC-11-07（BE-100 第 3 条）：人为改坏计数 → 修复 → 再执行一次无变化。
func TestRepairCounters(t *testing.T) {
	ctx := context.Background()
	s := openTestSQLite(t)
	if err := s.UpsertUsers(ctx, []User{{UserID: "alice"}, {UserID: "bob"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"alice", "bob"}, {"alice", "carol"}}, Blocks: []SeedEdge{{"alice", "x"}},
		GroupMembers: []SeedMember{{"g1", "alice"}, {"g1", "bob"}}, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveMessage(ctx, Message{MsgID: "m1", ClientMsgID: "c1", ConvID: "c2c:alice:bob", From: "alice", Type: "text", Body: []byte(`{}`), TS: 1, ConvUsers: []string{"alice", "bob"}}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE user_social SET friend_count = 7, block_count = 0 WHERE user_id = 'alice'`,
		`UPDATE chat_groups SET member_count = 99`,
		`UPDATE user_conv SET sent_count = 5 WHERE user_id = 'alice'`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.RepairCounters(ctx)
	if err != nil || n != 4 {
		t.Fatalf("repair = %d, %v; want 4 rows", n, err)
	}
	st, _ := s.SocialState(ctx, "alice")
	g, _ := s.GetGroup(ctx, "g1")
	var sent int
	s.db.QueryRow(`SELECT sent_count FROM user_conv WHERE user_id = 'alice'`).Scan(&sent)
	if st.FriendCount != 2 || st.BlockCount != 1 || g.MemberCount != 2 || sent != 1 {
		t.Fatalf("after repair: %+v members=%d sent=%d", st, g.MemberCount, sent)
	}
	if n, err := s.RepairCounters(ctx); err != nil || n != 0 {
		t.Fatalf("second repair = %d, %v; want 0", n, err)
	}
}

// FriendApplyTable（TC-SOC-02-17）：判定表第 3～9 行在存储层逐行。
func TestFriendApplyTable(t *testing.T) {
	ctx := context.Background()
	const day = int64(24 * 3600 * 1000)
	apply := func(s *SQLite, from, to string, now int64) (string, SocialChange, error) {
		return s.ApplyFriend(ctx, ApplyFriendParams{From: from, To: to, Source: "other", Now: now, ExpireAt: now + 7*day, MaxFriends: 2000})
	}
	users := func(s *SQLite) {
		if err := s.UpsertUsers(ctx, []User{{UserID: "a"}, {UserID: "b"}}); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("3 I blocked peer", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		s.Block(ctx, "a", "b", 1, 1000)
		if _, _, err := apply(s, "a", "b", 2); err != ErrIBlockedPeer {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("4 already mutual", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"a", "b"}, {"b", "a"}}, UpdatedAt: 1})
		st, c, err := apply(s, "a", "b", 2)
		if err != nil || st != ReqAccepted || c.Changed {
			t.Fatalf("%s %+v %v", st, c, err)
		}
	})
	t.Run("5 my friends full", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		var es []SeedEdge
		for i := range 2000 {
			es = append(es, SeedEdge{"a", fmt.Sprintf("f%d", i)})
		}
		s.SeedSocial(ctx, SocialSeed{Friends: es, UpdatedAt: 1})
		if _, _, err := apply(s, "a", "nobody", 2); err != ErrFriendLimit {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("6 peer missing or blocks me: only my out row", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		s.Block(ctx, "b", "a", 1, 1000)
		for _, peer := range []string{"b", "nobody"} {
			st, c, err := apply(s, "a", peer, 2)
			if err != nil || st != ReqPending || len(c.Bumped) != 1 || len(c.Items.Requests) != 1 || c.Items.Requests[0].Direction != DirOut {
				t.Fatalf("%s: %s %+v %v", peer, st, c, err)
			}
		}
		var n int
		s.db.QueryRow(`SELECT count(*) FROM friend_requests WHERE owner_id <> 'a'`).Scan(&n)
		if n != 0 {
			t.Fatalf("%d rows written for the peer", n)
		}
	})
	t.Run("7 restore my edge", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"b", "a"}}, UpdatedAt: 1})
		st, c, err := apply(s, "a", "b", 2)
		if err != nil || st != ReqAccepted || len(c.Bumped) != 1 || len(c.Items.Friends) != 1 {
			t.Fatalf("%s %+v %v", st, c, err)
		}
	})
	t.Run("8 auto accept", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		apply(s, "b", "a", 1)
		st, c, err := apply(s, "a", "b", 2)
		if err != nil || st != ReqAccepted || len(c.Bumped) != 2 {
			t.Fatalf("%s %+v %v", st, c, err)
		}
		var pending int
		s.db.QueryRow(`SELECT count(*) FROM friend_requests WHERE status <> 'accepted'`).Scan(&pending)
		if pending != 0 {
			t.Fatalf("%d requests not accepted", pending)
		}
	})
	t.Run("8 expired in row falls through to 9", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		apply(s, "b", "a", 1)
		st, _, err := apply(s, "a", "b", 8*day)
		if err != nil || st != ReqPending {
			t.Fatalf("%s %v", st, err)
		}
	})
	t.Run("9 new then merge", func(t *testing.T) {
		s := openTestSQLite(t)
		users(s)
		apply(s, "a", "b", 1)
		st, c, err := apply(s, "a", "b", 2)
		if err != nil || st != ReqPending || len(c.Bumped) != 2 || c.Items.Requests[0].ApplyCount != 2 {
			t.Fatalf("%s %+v %v", st, c, err)
		}
	})
}

// GroupDissolveNoReuse（TC-SOC-26-04，AC-SOC-26-4）：F-SOC-26 规则 3 的每个查询一条断言。
func TestGroupDissolveNoReuse(t *testing.T) {
	ctx := context.Background()
	s := openTestSQLite(t)
	if err := s.UpsertUsers(ctx, []User{{UserID: "alice"}, {UserID: "bob"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(ctx, Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveMessage(ctx, Message{MsgID: "m1", ClientMsgID: "c1", ConvID: "group:g1", From: "alice", Type: "text", Body: []byte(`{}`), TS: 1}); err != nil {
		t.Fatal(err)
	}
	members, err := s.DissolveGroup(ctx, "g1", 5)
	if err != nil || len(members) != 2 {
		t.Fatalf("dissolve = %v, %v", members, err)
	}
	if _, err := s.DissolveGroup(ctx, "g1", 6); err != ErrNotFound {
		t.Errorf("dissolve twice: %v", err)
	}
	check := func(name string, ok bool) {
		t.Helper()
		if !ok {
			t.Errorf("%s still sees the dissolved group", name)
		}
	}
	exists, _ := s.GroupExists(ctx, "g1")
	check("GroupExists", !exists)
	_, err = s.GetGroup(ctx, "g1")
	check("GetGroup", err == ErrNotFound)
	_, err = s.AddGroupMembers(ctx, "g1", []string{"alice"}, 2000, 7)
	check("AddGroupMembers", err == ErrNotFound)
	_, _, err = s.RemoveGroupMembers(ctx, "g1", []string{"alice"}, 1)
	check("RemoveGroupMembers", err == ErrNotFound)
	name := "x"
	check("UpdateGroup", s.UpdateGroup(ctx, "g1", GroupUpdate{Name: &name}) == ErrNotFound)
	gm, _ := s.GroupMembers(ctx, "g1")
	check("GroupMembers", len(gm) == 0)
	_, ok, _ := s.GroupMember(ctx, "g1", "alice")
	check("GroupMember", !ok)
	convs, _, _ := s.ListConversations(ctx, "alice", 500)
	check("ListConversations", len(convs) == 0)
	_, ok, _ = s.GetConversation(ctx, "alice", "group:g1")
	check("GetConversation", !ok)
	gc, _ := s.GroupConversations(ctx, "g1", []string{"alice", "bob"})
	check("GroupConversations", len(gc) == 0)
	mg, _, _ := s.MyGroups(ctx, "alice", "", 200)
	check("MyGroups", len(mg) == 0)
	vu, _ := s.VisibleUsers(ctx, "alice", []string{"bob"}, 0)
	check("VisibleUsers", len(vu) == 0)
	check("CreateGroup reuse", s.CreateGroup(ctx, Group{GroupID: "g1", Name: "again", CreatedAt: 8}, nil) == ErrExists)
	var msgs, dissolvedAt int64
	s.db.QueryRow(`SELECT count(*) FROM messages WHERE conv_id = 'group:g1'`).Scan(&msgs)
	s.db.QueryRow(`SELECT dissolved_at FROM chat_groups WHERE group_id = 'g1'`).Scan(&dissolvedAt)
	if msgs != 1 || dissolvedAt == 0 {
		t.Errorf("messages=%d dissolved_at=%d", msgs, dissolvedAt)
	}
}

// 加锁顺序断言（BE-101 第 2 条）：跑一遍全部社交写操作，没有违反 8.3 节全局顺序；变异测试：故意反序必须被发现。
func TestSocialLockOrder(t *testing.T) {
	ctx := context.Background()
	var violations []string
	onLockOrderViolation = func(seq string) { violations = append(violations, seq) }
	t.Cleanup(func() { onLockOrderViolation = nil })

	s := openTestSQLite(t)
	if err := s.UpsertUsers(ctx, []User{{UserID: "zed"}, {UserID: "amy"}, {UserID: "kim"}}); err != nil {
		t.Fatal(err)
	}
	const d = int64(24 * 3600 * 1000)
	s.ApplyFriend(ctx, ApplyFriendParams{From: "zed", To: "amy", Source: "other", Now: 1, ExpireAt: 7 * d, MaxFriends: 2000})
	s.ApplyFriend(ctx, ApplyFriendParams{From: "amy", To: "zed", Source: "other", Now: 2, ExpireAt: 7 * d, MaxFriends: 2000})
	s.ApplyFriend(ctx, ApplyFriendParams{From: "kim", To: "amy", Source: "other", Now: 3, ExpireAt: 7 * d, MaxFriends: 2000})
	s.RespondFriend(ctx, RespondFriendParams{Me: "amy", From: "kim", Accept: true, Now: 4, MaxFriends: 2000})
	s.ImportFriends(ctx, []FriendPair{{A: "kim", B: "zed"}, {A: "amy", B: "kim"}}, 5, 2000)
	s.RemoveFriends(ctx, []FriendPair{{A: "zed", B: "amy"}}, 6)
	s.Block(ctx, "zed", "kim", 7, 1000)
	s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"zed", "x"}, {"amy", "y"}}, Requests: []SeedRequest{{From: "zed", To: "kim"}}, UpdatedAt: 8})
	if len(violations) > 0 {
		t.Fatalf("lock order violations: %v", violations)
	}

	tx := &socialTx{}
	tx.lock(lockKindUser, "kim")
	tx.lock(lockKindUser, "amy") // 反序
	tx.lock(lockKindGroup, "g1") // 用户之后再锁群
	if len(violations) != 2 {
		t.Fatalf("mutation not detected: %v", violations)
	}
}

// 测试模式启动时的抽样校验（BE-100 第 3 条）。
func TestCounterDrift(t *testing.T) {
	ctx := context.Background()
	s := openTestSQLite(t)
	s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"a", "b"}}, UpdatedAt: 1})
	if n, err := s.CounterDrift(ctx, 100); err != nil || n != 0 {
		t.Fatalf("clean db drift = %d, %v", n, err)
	}
	s.db.Exec(`UPDATE user_social SET friend_count = 9`)
	if n, _ := s.CounterDrift(ctx, 100); n != 1 {
		t.Fatalf("drift = %d, want 1", n)
	}
}

// TipSameTx（AC-SOC-29-8）：提示消息写入失败时，邀请整体回滚：成员表、member_count、会话 seq 都不变。
func TestTipSameTx(t *testing.T) {
	ctx := context.Background()
	s := openTestSQLite(t)
	if err := s.UpsertUsers(ctx, []User{{UserID: "a"}, {UserID: "b"}, {UserID: "c"}, {UserID: "d"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SeedSocial(ctx, SocialSeed{Friends: []SeedEdge{{"a", "b"}, {"b", "a"}, {"a", "c"}, {"c", "a"}, {"a", "d"}, {"d", "a"}}, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	gid, _, _, _, err := s.CreateClientGroup(ctx, CreateClientGroupParams{UserID: "a", ClientReqID: "r1", GroupID: "ug-1", Members: []string{"b", "c"}, MaxMembers: 500, MaxJoined: 500, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("tip write failed")
	failTipWrite = func() error { return boom }
	defer func() { failTipWrite = nil }()
	if _, _, _, err := s.InviteMembers(ctx, gid, "a", []string{"d"}, 500, 2); !errors.Is(err, boom) {
		t.Fatalf("invite err = %v", err)
	}
	if _, _, _, _, err := s.CreateClientGroup(ctx, CreateClientGroupParams{UserID: "a", ClientReqID: "r2", GroupID: "ug-2", Members: []string{"b", "c"}, MaxMembers: 500, MaxJoined: 500, Now: 3}); !errors.Is(err, boom) {
		t.Fatalf("create err = %v", err)
	}
	failTipWrite = nil
	g, _ := s.GetGroup(ctx, gid)
	ms, _ := s.GroupMembers(ctx, gid)
	var seq int64
	s.db.QueryRow(`SELECT max_seq FROM conv_seq WHERE conv_id = ?`, "group:"+gid).Scan(&seq)
	if g.MemberCount != 3 || len(ms) != 3 || seq != 1 {
		t.Fatalf("not rolled back: count=%d members=%d seq=%d", g.MemberCount, len(ms), seq)
	}
	if ok, _ := s.GroupExists(ctx, "ug-2"); ok {
		t.Fatal("create not rolled back")
	}
	// 失败的建群不记录 clientReqId，可以重试（AC-SOC-21-11）
	if _, dup, _, _, err := s.CreateClientGroup(ctx, CreateClientGroupParams{UserID: "a", ClientReqID: "r2", GroupID: "ug-3", Members: []string{"b", "c"}, MaxMembers: 500, MaxJoined: 500, Now: 4}); err != nil || dup {
		t.Fatalf("retry dup=%v err=%v", dup, err)
	}
}

// 8.5 节第 4 条：已有群 ID 占用 ug- 前缀时迁移中止并列出。
func TestMigrationGroupRolesPrecheck(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)
	all, err := SQLiteMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all[:6]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO chat_groups (group_id, name, created_at) VALUES ('ug-old', 'x', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all); err == nil || !strings.Contains(err.Error(), "ug-old") {
		t.Fatalf("err = %v", err)
	}
	if _, err := db.Exec(`UPDATE chat_groups SET group_id = 'renamed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all); err != nil {
		t.Fatal(err)
	}
}
