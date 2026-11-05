package storetest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"tianting/server/internal/store"
)

// 管理后台第一批查询的存储契约（PRD-admin BE-205、207、208、210）。

func userIDs(us []store.User) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.UserID
	}
	return out
}

func searchUsers(t *testing.T, s store.Store, q store.UserSearch) []string {
	t.Helper()
	if q.Limit == 0 {
		q.Limit = 50
	}
	us, _, err := s.SearchUsers(context.Background(), q)
	if err != nil {
		t.Fatalf("SearchUsers(%+v): %v", q, err)
	}
	return userIDs(us)
}

func testAdminSearchUsers(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.UpsertUsers(ctx, []store.User{
		{UserID: "u100"}, {UserID: "u101"}, {UserID: "u2"}, {UserID: "U100"},
		{UserID: "u_1"}, {UserID: "uX1"}, {UserID: "p%1"},
		{UserID: "n1", Nickname: "小明"}, {UserID: "n2", Nickname: "小明明"}, {UserID: "n3", Nickname: "Tom"}, {UserID: "n4", Nickname: "tommy"},
		{UserID: "kefu"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOfficial(ctx, "kefu", true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		q    string
		want []string
	}{
		{"u10", []string{"u100", "u101"}}, // 区分大小写（AC-ADM-10-1）
		{"U10", []string{"U100"}},
		{"小明", []string{"n1", "n2"}}, // 前缀（AC-ADM-10-2）
		{"明", []string{}},
		{"tom", []string{"n3", "n4"}}, // ASCII 不区分大小写
		{"TOM", []string{"n3", "n4"}},
		{"u_", []string{"u_1"}}, // 下划线不是通配符（AC-ADM-10-3）
		{"p%", []string{"p%1"}},
		{"%", []string{}},
		{"zzz", []string{}},
	} {
		if got := searchUsers(t, s, store.UserSearch{Q: tc.q}); !slices.Equal(got, tc.want) {
			t.Errorf("search %q = %v, want %v", tc.q, got, tc.want)
		}
	}
	if got := searchUsers(t, s, store.UserSearch{OfficialOnly: true}); !slices.Equal(got, []string{"kefu"}) {
		t.Errorf("official = %v", got)
	}
	if got := searchUsers(t, s, store.UserSearch{Q: "u", OfficialOnly: true}); len(got) != 0 {
		t.Errorf("official + q = %v", got)
	}
}

// userId 和昵称都命中的人只出现一次；翻页不重不漏（AC-ADM-10-7）。
func testAdminSearchUsersPaging(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	var users []store.User
	for i := range 250 {
		users = append(users, store.User{UserID: fmt.Sprintf("p%04d", i), Nickname: fmt.Sprintf("p%04d", i)})
	}
	if err := s.UpsertUsers(ctx, users); err != nil {
		t.Fatal(err)
	}
	var seen []string
	after := ""
	for page := 0; ; page++ {
		us, next, err := s.SearchUsers(ctx, store.UserSearch{Q: "p", After: after, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, userIDs(us)...)
		if next == "" {
			if page != 2 {
				t.Errorf("pages = %d", page+1)
			}
			break
		}
		after = next
	}
	if len(seen) != 250 || !slices.IsSorted(seen) || len(slices.Compact(slices.Clone(seen))) != 250 {
		t.Errorf("seen %d users, sorted=%v", len(seen), slices.IsSorted(seen))
	}
}

func testAdminSearchGroups(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i, g := range []store.Group{
		{GroupID: "g-match-1", Name: "周末球局"}, {GroupID: "g-match-2", Name: "Alpha"}, {GroupID: "g-other", Name: "alphabet"}, {GroupID: "g9", Name: "已解散"},
	} {
		g.CreatedAt = int64(100 + i)
		if err := s.CreateGroup(ctx, g, []string{"a", "b"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DissolveGroup(ctx, "g9", nowMs()); err != nil {
		t.Fatal(err)
	}
	ids := func(q store.GroupSearch) []string {
		t.Helper()
		q.Limit = 50
		gs, _, err := s.SearchGroups(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, g := range gs {
			out = append(out, g.GroupID)
		}
		return out
	}
	if got := ids(store.GroupSearch{Q: "g-match"}); !slices.Equal(got, []string{"g-match-2", "g-match-1"}) {
		t.Errorf("prefix = %v (newest first)", got)
	}
	if got := ids(store.GroupSearch{Q: "ALPHA"}); !slices.Equal(got, []string{"g-other", "g-match-2"}) {
		t.Errorf("name prefix = %v", got)
	}
	if got := ids(store.GroupSearch{}); slices.Contains(got, "g9") || len(got) != 3 {
		t.Errorf("default = %v", got)
	}
	gs, _, err := s.SearchGroups(ctx, store.GroupSearch{Q: "g9", IncludeDissolved: true, Limit: 10})
	if err != nil || len(gs) != 1 || gs[0].DissolvedAt == 0 || gs[0].MemberCount != 0 {
		t.Errorf("dissolved = %+v err=%v", gs, err)
	}
	// 翻页：同一创建时间也不重不漏。
	for i := range 5 {
		if err := s.CreateGroup(ctx, store.Group{GroupID: fmt.Sprintf("same-%d", i), Name: "x", CreatedAt: 500}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var seen []string
	var after *store.GroupCursor
	for {
		page, next, err := s.SearchGroups(ctx, store.GroupSearch{Q: "same-", After: after, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range page {
			seen = append(seen, g.GroupID)
		}
		if next == nil {
			break
		}
		after = next
	}
	if len(seen) != 5 || len(slices.Compact(slices.Sorted(slices.Values(seen)))) != 5 {
		t.Errorf("paged = %v", seen)
	}
}

func testAdminListMembers(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g"}, []string{"u10", "u11", "u2", "u_a", "uxa"}); err != nil {
		t.Fatal(err)
	}
	names := func(after, prefix string, limit int) ([]string, string) {
		t.Helper()
		rows, next, err := s.ListGroupMembersByID(ctx, "g1", after, prefix, limit)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, r := range rows {
			out = append(out, r.UserID)
		}
		return out, next
	}
	if got, _ := names("", "u1", 50); !slices.Equal(got, []string{"u10", "u11"}) {
		t.Errorf("prefix u1 = %v", got)
	}
	if got, _ := names("", "u_", 50); !slices.Equal(got, []string{"u_a"}) {
		t.Errorf("prefix u_ = %v", got)
	}
	got, next := names("", "", 2)
	if !slices.Equal(got, []string{"u10", "u11"}) || next != "u11" {
		t.Errorf("page 1 = %v next=%q", got, next)
	}
	if got, next := names(next, "", 50); !slices.Equal(got, []string{"u2", "u_a", "uxa"}) || next != "" {
		t.Errorf("page 2 = %v next=%q", got, next)
	}
}

func testAdminMessageMeta(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	const conv = "group:g1"
	for i, m := range []struct {
		from, body string
		ts         int64
	}{
		{"a", `{"text":"hello"}`, 100},
		{"b", `{"text":"你好世界"}`, 200},
		{"a", `{"text":"👍🏽x"}`, 300}, // 👍 + 肤色修饰符 + x = 3 个码点
		{"a", `{"data":{"k":"v"}}`, 400},
	} {
		msg := newMsg(fmt.Sprint("m", i), conv, m.from, fmt.Sprintf("client-%04d", i))
		msg.Body, msg.TS = []byte(m.body), m.ts
		if _, _, err := s.SaveMessage(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	all, more, err := s.ListMessageMeta(ctx, store.MessageMetaQuery{ConvID: conv, Limit: 50})
	if err != nil || more || len(all) != 4 {
		t.Fatalf("all = %+v more=%v err=%v", all, more, err)
	}
	if all[0].Seq != 4 || all[3].Seq != 1 {
		t.Errorf("order = %+v", all)
	}
	if all[3].BodyLength != 5 || all[2].BodyLength != 4 || all[1].BodyLength != 3 || all[0].BodyLength != len(`{"data":{"k":"v"}}`) {
		t.Errorf("code point lengths = %d %d %d %d", all[3].BodyLength, all[2].BodyLength, all[1].BodyLength, all[0].BodyLength)
	}
	seqs := func(q store.MessageMetaQuery) []int64 {
		t.Helper()
		q.ConvID, q.Limit = conv, 50
		ms, _, err := s.ListMessageMeta(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		out := []int64{}
		for _, m := range ms {
			out = append(out, m.Seq)
		}
		return out
	}
	if got := seqs(store.MessageMetaQuery{BeforeSeq: 3}); !slices.Equal(got, []int64{2, 1}) {
		t.Errorf("beforeSeq = %v", got)
	}
	if got := seqs(store.MessageMetaQuery{From: "a"}); !slices.Equal(got, []int64{4, 3, 1}) {
		t.Errorf("from = %v", got)
	}
	if got := seqs(store.MessageMetaQuery{SinceTS: 200, UntilTS: 300}); !slices.Equal(got, []int64{3, 2}) {
		t.Errorf("time range = %v (inclusive)", got)
	}
	page, more, err := s.ListMessageMeta(ctx, store.MessageMetaQuery{ConvID: conv, Limit: 3})
	if err != nil || !more || len(page) != 3 {
		t.Errorf("limit: %d more=%v err=%v", len(page), more, err)
	}
	empty, more, err := s.ListMessageMeta(ctx, store.MessageMetaQuery{ConvID: "c2c:x:y", Limit: 50})
	if err != nil || more || empty == nil || len(empty) != 0 {
		t.Errorf("missing conversation: %v more=%v err=%v", empty, more, err)
	}
	// 已解散的群照常可查（AC-ADM-24-4）。
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g"}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DissolveGroup(ctx, "g1", nowMs()); err != nil {
		t.Fatal(err)
	}
	if after, _, err := s.ListMessageMeta(ctx, store.MessageMetaQuery{ConvID: conv, Limit: 50}); err != nil || len(after) != 4 {
		t.Errorf("after dissolve: %d err=%v", len(after), err)
	}
}

func testAdminUserGroups(t *testing.T, open Opener) {
	s, c := open(t), context.Background()
	s.UpsertUsers(c, []store.User{{UserID: "u1"}, {UserID: "u2"}})
	for _, g := range []string{"g3", "g1", "g2", "gone"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: "n" + g, CreatedAt: 1}, []string{"u1"}); err != nil {
			t.Fatal(err)
		}
	}
	s.CreateGroup(c, store.Group{GroupID: "other", Name: "o", CreatedAt: 1}, []string{"u2"})
	if _, err := s.DissolveGroup(c, "gone", 5); err != nil {
		t.Fatal(err)
	}
	p1, next, err := s.ListUserGroups(c, "u1", "", 2)
	if err != nil || len(p1) != 2 || p1[0].GroupID != "g1" || p1[1].GroupID != "g2" || next != "g2" || p1[0].Name != "ng1" || p1[0].Role != store.RoleMember {
		t.Fatalf("page 1 = %+v next %q err %v", p1, next, err)
	}
	p2, next, err := s.ListUserGroups(c, "u1", next, 2)
	if err != nil || len(p2) != 1 || p2[0].GroupID != "g3" || next != "" {
		t.Fatalf("page 2 = %+v next %q err %v", p2, next, err)
	}
}

func testAdminUserConvMeta(t *testing.T, open Opener) {
	s, c := open(t), context.Background()
	s.UpsertUsers(c, []store.User{{UserID: "a"}, {UserID: "b"}, {UserID: "z"}})
	msg := func(id, conv string, ts int64, users ...string) {
		t.Helper()
		m := newMsg(id, conv, "a", "client-"+id)
		m.TS, m.ConvUsers = ts, users
		if _, _, err := s.SaveMessage(c, m); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []string{"g1", "g2"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: g, CreatedAt: 1}, []string{"a"}); err != nil {
			t.Fatal(err)
		}
	}
	msg("m1", "c2c:a:b", 300, "a", "b")
	msg("m2", "group:g1", 500)
	msg("m3", "group:g2", 300)
	msg("m4", "c2c:a:z", 100, "a", "z")
	var ids []string
	var after *store.ConvCursor
	for {
		rows, next, err := s.ListUserConvMeta(c, "a", after, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			ids = append(ids, r.ConvID)
		}
		if next == nil {
			break
		}
		after = next
	}
	// updatedAt 倒序；同为 300 时按 convId 升序（翻页也不丢）。
	if want := []string{"group:g1", "c2c:a:b", "group:g2", "c2c:a:z"}; !slices.Equal(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
}

func testAdminStats(t *testing.T, open Opener) {
	s, c := open(t), context.Background()
	s.UpsertUsers(c, []store.User{{UserID: "a"}, {UserID: "b"}})
	save := func(id string, ts int64, typ string) {
		t.Helper()
		m := newMsg(id, "c2c:a:b", "a", "client-"+id)
		m.TS, m.Type, m.ConvUsers = ts, typ, []string{"a", "b"}
		if _, _, err := s.SaveMessage(c, m); err != nil {
			t.Fatal(err)
		}
	}
	save("m0", 999, "text")    // 区间之前
	save("m1", 1000, "text")   // 下界算
	save("m2", 1500, "tip")    // 提示消息不算
	save("m3", 1999, "custom") // 自定义消息算
	save("m4", 2000, "text")   // 上界不算
	for _, g := range []string{"g1", "g2"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: g, CreatedAt: 1}, []string{"a"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DissolveGroup(c, "g2", 5); err != nil {
		t.Fatal(err)
	}
	st, err := s.AdminStats(c, 1000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if st.Messages != 2 || st.Groups != 1 {
		t.Errorf("stats = %+v, want 2 messages, 1 group", st)
	}
}

func testGroupAllMute(t *testing.T, open Opener) {
	s, c := open(t), context.Background()
	s.UpsertUsers(c, []store.User{{UserID: "a"}, {UserID: "b"}})
	for _, g := range []string{"g1", "g2"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: g, CreatedAt: 1}, []string{"a"}); err != nil {
			t.Fatal(err)
		}
	}
	mute := store.Mute{Scope: store.MuteGroupAll, GroupID: "g1", Reason: "r", By: "c:o", At: 7}
	if err := s.SetMute(c, mute); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMute(c, store.Mute{Scope: store.MuteGroupAll, GroupID: "nope", Reason: "r", By: "c:o", At: 7}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("mute missing group: %v", err)
	}
	if m, err := s.GroupAllMute(c, "g1"); err != nil || m == nil || m.Until != nil || m.UserID != "" || m.At != 7 {
		t.Fatalf("GroupAllMute = %+v, %v", m, err)
	}
	if m, err := s.GroupAllMute(c, "g2"); err != nil || m != nil {
		t.Errorf("g2 = %+v, %v", m, err)
	}
	// UserGroupAllMutes 只返回用户所在的群；UserMutes、GroupMutes 不受影响。
	if got, err := s.UserGroupAllMutes(c, "a"); err != nil || len(got) != 1 || got["g1"].At != 7 {
		t.Errorf("a = %+v, %v", got, err)
	}
	if got, err := s.UserGroupAllMutes(c, "b"); err != nil || len(got) != 0 {
		t.Errorf("b = %+v, %v", got, err)
	}
	if got, _ := s.UserMutes(c, "a"); len(got) != 0 {
		t.Errorf("UserMutes = %+v", got)
	}
	if got, _ := s.GroupMutes(c, "g1"); len(got) != 0 {
		t.Errorf("GroupMutes = %+v", got)
	}
	// 解除返回旧记录；再解除返回 nil。
	if old, err := s.DeleteMute(c, store.MuteGroupAll, "", "g1"); err != nil || old == nil || old.At != 7 {
		t.Errorf("DeleteMute = %+v, %v", old, err)
	}
	if old, err := s.DeleteMute(c, store.MuteGroupAll, "", "g1"); err != nil || old != nil {
		t.Errorf("second DeleteMute = %+v, %v", old, err)
	}
	// 解散清除：成员退群不清除全群禁言，解散才清除。
	s.SetMute(c, mute)
	if _, _, err := s.RemoveGroupMembers(c, "g1", []string{"a"}, 8); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.GroupAllMute(c, "g1"); m == nil {
		t.Error("removing a member must not clear the group mute")
	}
	if _, err := s.DissolveGroup(c, "g1", 9); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.GroupAllMute(c, "g1"); m != nil {
		t.Errorf("dissolve left %+v", m)
	}
}
