package storetest

import (
	"errors"
	"fmt"
	"testing"

	"tianting/server/internal/store"
)

// C5 会话置顶、免打扰、删除的契约用例（BE-483 第 1 条）。Postgres 实现随 B6 跑同一套。

func convSettingCases() []struct {
	name string
	fn   func(*testing.T, Opener)
} {
	return []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"ConvSettingsPinAndNotify", testConvSettingsPinAndNotify},
		{"ConvSettingsDelete", testConvSettingsDelete},
		{"ConvSettingsDeletedList", testConvSettingsDeletedList},
		{"ConvSettingsMemberCleanup", testConvSettingsMemberCleanup},
		{"ConvSettingsNotifyOffUsers", testConvSettingsNotifyOffUsers},
		{"ConvSettingsTotalUnread", testConvSettingsTotalUnread},
		{"ConvSettingsListOrder", testConvSettingsListOrder},
		{"ConvSettingsHidden", testConvSettingsHidden},
	}
}

func ptr[T any](v T) *T { return &v }

func mustSet(t *testing.T, s store.Store, req store.ConvSetReq) store.ConvSetting {
	t.Helper()
	if req.PinMax == 0 {
		req.PinMax = 20
	}
	got, err := s.SetConvSettings(ctx(), req)
	if err != nil {
		t.Fatalf("SetConvSettings %+v: %v", req, err)
	}
	return got
}

// 置顶：第一次写服务端时间，再次置顶 pinnedAt 不变；名额按当前置顶数在同一事务里算（ErrPinLimit），取消后能再置顶；免打扰独立。
// 群会话不是成员 ErrNotGroupMember，不写行。ConvSettings 一次取齐。
func testConvSettingsPinAndNotify(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := range 3 {
		saveAt(t, s, fmt.Sprintf("c2c:a:p%d", i), "a", fmt.Sprintf("m%d", i), int64(100+i), "hi")
	}
	got := mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p0", Pinned: ptr(true), Now: 1000, PinMax: 2})
	if got.PinnedAt != 1000 || got.NotifyOff {
		t.Fatalf("first pin = %+v", got)
	}
	if again := mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p0", Pinned: ptr(true), Now: 2000, PinMax: 2}); again.PinnedAt != 1000 {
		t.Errorf("re-pin changed pinnedAt: %+v", again)
	}
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p1", Pinned: ptr(true), Now: 3000, PinMax: 2})
	if _, err := s.SetConvSettings(c, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p2", Pinned: ptr(true), NotifyOff: ptr(true), Now: 4000, PinMax: 2}); !errors.Is(err, store.ErrPinLimit) {
		t.Fatalf("third pin with max 2: %v", err)
	}
	// 被拒绝的请求整体不写（免打扰也没有写进去）。
	if all, _ := s.ConvSettings(c, "a"); len(all) != 2 || all["c2c:a:p2"].NotifyOff {
		t.Errorf("after ErrPinLimit: %+v", all)
	}
	// 已置顶的再次置顶不受名额限制。
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p1", Pinned: ptr(true), Now: 5000, PinMax: 2})
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p0", Pinned: ptr(false), Now: 6000, PinMax: 2})
	if got := mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p2", Pinned: ptr(true), NotifyOff: ptr(true), Now: 7000, PinMax: 2}); got.PinnedAt != 7000 || !got.NotifyOff {
		t.Errorf("pin after unpin = %+v", got)
	}
	// 只改免打扰，置顶不变。
	if got := mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:p2", NotifyOff: ptr(false), Now: 8000}); got.PinnedAt != 7000 || got.NotifyOff {
		t.Errorf("notifyOff only = %+v", got)
	}
	all, err := s.ConvSettings(c, "a")
	if err != nil || len(all) != 3 || all["c2c:a:p0"].PinnedAt != 0 || all["c2c:a:p1"].PinnedAt != 3000 || all["c2c:a:p2"].PinnedAt != 7000 {
		t.Errorf("ConvSettings = %+v, %v", all, err)
	}
	if other, _ := s.ConvSettings(c, "p0"); len(other) != 0 {
		t.Errorf("settings leak to the peer: %+v", other)
	}

	if err := s.CreateGroup(c, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetConvSettings(c, store.ConvSetReq{UserID: "x", ConvID: "group:g1", NotifyOff: ptr(true), Now: 1, PinMax: 20}); !errors.Is(err, store.ErrNotGroupMember) {
		t.Errorf("non-member: %v", err)
	}
	if _, err := s.DeleteConv(c, store.ConvDeleteReq{UserID: "x", ConvID: "group:g1", UpToSeq: 1, Now: 1}); !errors.Is(err, store.ErrNotGroupMember) {
		t.Errorf("non-member delete: %v", err)
	}
	if other, _ := s.ConvSettings(c, "x"); len(other) != 0 {
		t.Errorf("non-member got a row: %+v", other)
	}
}

// 删除：cleared = min(upToSeq, maxSeq)，只增不减；deleted_at 是服务端时间；已读推进到 clearedSeq（只前进）；清除置顶、保留免打扰。
func testConvSettingsDelete(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := 1; i <= 10; i++ {
		saveAt(t, s, "c2c:a:b", "b", fmt.Sprintf("m%02d", i), int64(100+i), "hi")
	}
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:b", Pinned: ptr(true), NotifyOff: ptr(true), Now: 500})
	got, err := s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:b", UpToSeq: 8, Now: 600})
	if err != nil || got.ClearedSeq != 8 || got.DeletedAt != 600 || got.PinnedAt != 0 || !got.NotifyOff {
		t.Fatalf("delete upTo 8 = %+v, %v", got, err)
	}
	if r, _ := s.ReadSeq(c, "a", "c2c:a:b"); r != 8 {
		t.Errorf("readSeq = %d, want 8", r)
	}
	if got, _ := s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:b", UpToSeq: 3, Now: 700}); got.ClearedSeq != 8 {
		t.Errorf("clearedSeq went back: %+v", got)
	}
	if got, _ := s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:b", UpToSeq: 999, Now: 800}); got.ClearedSeq != 10 || got.DeletedAt != 800 {
		t.Errorf("upTo 999 = %+v", got)
	}
	if cs, err := s.ConvClearedSeq(c, "a", "c2c:a:b"); err != nil || cs != 10 {
		t.Errorf("ConvClearedSeq = %d, %v", cs, err)
	}
	if cs, _ := s.ConvClearedSeq(c, "b", "c2c:a:b"); cs != 0 {
		t.Errorf("peer clearedSeq = %d", cs)
	}
	// 已读不后退：先读到 10 的人删除到 5，已读仍是 10。
	if _, _, err := s.SetReadSeq(c, "b", "c2c:a:b", 10); err != nil {
		t.Fatal(err)
	}
	s.DeleteConv(c, store.ConvDeleteReq{UserID: "b", ConvID: "c2c:a:b", UpToSeq: 5, Now: 900})
	if r, _ := s.ReadSeq(c, "b", "c2c:a:b"); r != 10 {
		t.Errorf("peer readSeq = %d, want 10", r)
	}
	// 没有消息的群：upToSeq=0 也写一行（隐藏条件 deleted_at > 0）。
	if err := s.CreateGroup(c, store.Group{GroupID: "g0", Name: "g0", CreatedAt: 1}, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "group:g0", UpToSeq: 0, Now: 1000}); err != nil || got.ClearedSeq != 0 || got.DeletedAt != 1000 {
		t.Errorf("empty group = %+v, %v", got, err)
	}
}

// deleted：只列当前隐藏的（deleted_at > 0 AND max_seq <= cleared_seq），按 deleted_at 倒序，超出 limit 时 truncated；
// 有新消息后不再在里面；只设免打扰的不在里面。
func testConvSettingsDeletedList(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := range 4 {
		saveAt(t, s, fmt.Sprintf("c2c:a:d%d", i), "a", fmt.Sprintf("m%d", i), int64(100+i), "hi")
	}
	saveAt(t, s, "c2c:a:d0", "a", "m0b", 105, "hi") // d0 有 2 条
	for i := range 4 {
		up := int64(1)
		if i == 0 {
			up = 2
		}
		if _, err := s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: fmt.Sprintf("c2c:a:d%d", i), UpToSeq: up, Now: int64(1000 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	saveAt(t, s, "c2c:a:d3", "d3", "m3b", 200, "back")                                           // d3 重新出现
	s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:d0", UpToSeq: 1, Now: 1500}) // 只更新 deleted_at，cleared 仍是 2
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:d1", NotifyOff: ptr(true), Now: 1600})
	got, truncated, err := s.DeletedConvs(c, "a", 10)
	if err != nil || truncated || len(got) != 3 || got[0].ConvID != "c2c:a:d0" || got[0].ClearedSeq != 2 || got[1].ConvID != "c2c:a:d2" || got[2].ConvID != "c2c:a:d1" {
		t.Fatalf("DeletedConvs = %+v, %v, %v", got, truncated, err)
	}
	if got, truncated, _ := s.DeletedConvs(c, "a", 2); len(got) != 2 || !truncated {
		t.Errorf("limit 2 = %+v, %v", got, truncated)
	}
	if got, truncated, _ := s.DeletedConvs(c, "a", 3); len(got) != 3 || truncated {
		t.Errorf("limit 3 = %+v, %v", got, truncated)
	}
	if got, _, _ := s.DeletedConvs(c, "d0", 10); len(got) != 0 {
		t.Errorf("peer = %+v", got)
	}
}

// 退群、被移出、解散：同一事务删除设置（F-CVS-01 规则 6、F-CVS-03 规则 6a）；其他群、单聊的行不受影响。
func testConvSettingsMemberCleanup(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for _, g := range []string{"g1", "g2", "g3"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: g, CreatedAt: 1}, []string{"a", "b", "c"}); err != nil {
			t.Fatal(err)
		}
		for _, u := range []string{"a", "b"} {
			mustSet(t, s, store.ConvSetReq{UserID: u, ConvID: "group:" + g, Pinned: ptr(true), NotifyOff: ptr(true), Now: 10})
		}
	}
	saveAt(t, s, "c2c:a:z", "a", "m1", 100, "hi")
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:z", NotifyOff: ptr(true), Now: 10})
	has := func(u, conv string) bool {
		all, err := s.ConvSettings(c, u)
		if err != nil {
			t.Fatal(err)
		}
		_, ok := all[conv]
		return ok
	}
	if _, _, err := s.RemoveGroupMembers(c, "g1", []string{"a"}, 20); err != nil {
		t.Fatal(err)
	}
	if has("a", "group:g1") || !has("b", "group:g1") {
		t.Errorf("RemoveGroupMembers: a %v, b %v", has("a", "group:g1"), has("b", "group:g1"))
	}
	if _, err := s.LeaveGroup(c, "g2", "a", 30); err != nil {
		t.Fatal(err)
	}
	if has("a", "group:g2") || !has("b", "group:g2") {
		t.Errorf("LeaveGroup: a %v, b %v", has("a", "group:g2"), has("b", "group:g2"))
	}
	if _, err := s.DissolveGroup(c, "g3", 40); err != nil {
		t.Fatal(err)
	}
	if has("a", "group:g3") || has("b", "group:g3") {
		t.Error("DissolveGroup left rows")
	}
	if !has("a", "c2c:a:z") || !has("b", "group:g1") || !has("b", "group:g2") {
		t.Error("cleanup removed unrelated rows")
	}
	// 重新加入不继承。
	if _, err := s.AddGroupMembers(c, "g1", []string{"a"}, 100, 50); err != nil {
		t.Fatal(err)
	}
	if has("a", "group:g1") {
		t.Error("rejoin inherited settings")
	}
	if cv, ok, _ := s.GetConversation(c, "a", "group:g1"); !ok || cv.PinnedAt != 0 || cv.NotifyOff || cv.ClearedSeq != 0 {
		t.Errorf("rejoined conversation = %+v", cv)
	}
}

func testConvSettingsNotifyOffUsers(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	if err := s.CreateGroup(c, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "group:g1", NotifyOff: ptr(true), Now: 1})
	mustSet(t, s, store.ConvSetReq{UserID: "b", ConvID: "group:g1", Pinned: ptr(true), Now: 1})
	got, err := s.NotifyOffUsers(c, "group:g1", []string{"a", "b", "c", "nobody"})
	if err != nil || len(got) != 1 || !got["a"] {
		t.Errorf("NotifyOffUsers = %v, %v", got, err)
	}
	if got, _ := s.NotifyOffUsers(c, "group:g1", nil); len(got) != 0 {
		t.Errorf("empty = %v", got)
	}
}

// 总未读（F-CVS-02 规则 4，AC-APN-04-4 的数据）：普通会话 3 条，免打扰群 2 条其中 1 条 @ 我，再来 1 条普通 → 5。
func testConvSettingsTotalUnread(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	if err := s.CreateGroup(c, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"u", "o"}); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		saveAt(t, s, "c2c:p:u", "p", fmt.Sprintf("n%d", i), int64(100+i), "hi")
	}
	mustSet(t, s, store.ConvSetReq{UserID: "u", ConvID: "group:g1", NotifyOff: ptr(true), Now: 1})
	plain := groupMsg("g-1", "o", "g-1", "plain")
	if _, _, err := s.SaveMessage(c, plain); err != nil {
		t.Fatal(err)
	}
	at := groupMsg("g-2", "o", "g-2", "@u")
	at.Mentions = []string{"u"}
	if _, _, err := s.SaveMessage(c, at); err != nil {
		t.Fatal(err)
	}
	saveAt(t, s, "c2c:q:u", "q", "n9", 200, "hi")
	if n, err := s.TotalUnread(c, "u"); err != nil || n != 5 {
		t.Errorf("TotalUnread = %d, %v, want 5", n, err)
	}
	// 取消免打扰：普通会话照常全部计入（3 + 2 + 1）。
	mustSet(t, s, store.ConvSetReq{UserID: "u", ConvID: "group:g1", NotifyOff: ptr(false), Now: 2})
	if n, _ := s.TotalUnread(c, "u"); n != 6 {
		t.Errorf("without notifyOff = %d, want 6", n)
	}
	// 删除会话：已读推进，隐藏的会话不计入。
	if _, err := s.DeleteConv(c, store.ConvDeleteReq{UserID: "u", ConvID: "c2c:p:u", UpToSeq: 3, Now: 3}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.TotalUnread(c, "u"); n != 3 {
		t.Errorf("after delete = %d, want 3", n)
	}
	if n, _ := s.TotalUnread(c, "nobody"); n != 0 {
		t.Errorf("nobody = %d", n)
	}
}

// convs order (F-CVS-01 rules 3, 4): pinned first by pinnedAt desc, then updatedAt desc, convId asc; with LIMIT the pinned ones stay.
func testConvSettingsListOrder(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := range 5 {
		saveAt(t, s, fmt.Sprintf("c2c:a:o%d", i), "a", fmt.Sprintf("m%d", i), int64(100+i), "hi")
	}
	saveAt(t, s, "c2c:a:o9", "a", "m9", 104, "tie") // same updatedAt as o4
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:o1", Pinned: ptr(true), Now: 1000})
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "c2c:a:o0", Pinned: ptr(true), Now: 2000}) // pinned later, older updatedAt
	convs, truncated, err := s.ListConversations(c, "a", 10)
	if err != nil || truncated || convIDs(convs) != "[c2c:a:o0 c2c:a:o1 c2c:a:o4 c2c:a:o9 c2c:a:o3 c2c:a:o2]" {
		t.Fatalf("order = %s, %v, %v", convIDs(convs), truncated, err)
	}
	if convs[0].PinnedAt != 2000 || convs[2].PinnedAt != 0 {
		t.Errorf("rows = %+v", convs[:3])
	}
	if convs, truncated, _ := s.ListConversations(c, "a", 3); !truncated || convIDs(convs) != "[c2c:a:o0 c2c:a:o1 c2c:a:o4]" {
		t.Errorf("limit 3 = %s, %v", convIDs(convs), truncated)
	}
	if convs, _, _ := s.ListConversations(c, "o0", 10); convIDs(convs) != "[c2c:a:o0]" || convs[0].PinnedAt != 0 {
		t.Errorf("peer sees the pin: %+v", convs)
	}
}

// Hidden condition (F-CVS-03 rule 2): deleted_at > 0 AND max_seq <= cleared_seq. A partial delete stays; an empty group with only
// notifyOff stays; a deleted empty group is hidden; a new message (seq > cleared) brings it back. GetConversation still finds hidden ones.
func testConvSettingsHidden(t *testing.T, open Opener) {
	s, c := open(t), ctx()
	for i := 1; i <= 3; i++ {
		saveAt(t, s, "c2c:a:h1", "h1", fmt.Sprintf("x%d", i), int64(100+i), "hi")
		saveAt(t, s, "c2c:a:h2", "h2", fmt.Sprintf("y%d", i), int64(100+i), "hi")
	}
	for _, g := range []string{"e1", "e2"} {
		if err := s.CreateGroup(c, store.Group{GroupID: g, Name: g, CreatedAt: 1}, []string{"a", "b"}); err != nil {
			t.Fatal(err)
		}
	}
	s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:h1", UpToSeq: 3, Now: 10})
	s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "c2c:a:h2", UpToSeq: 2, Now: 10})
	mustSet(t, s, store.ConvSetReq{UserID: "a", ConvID: "group:e1", NotifyOff: ptr(true), Now: 10})
	s.DeleteConv(c, store.ConvDeleteReq{UserID: "a", ConvID: "group:e2", UpToSeq: 0, Now: 10})
	convs, _, err := s.ListConversations(c, "a", 10)
	if err != nil || convIDs(convs) != "[c2c:a:h2 group:e1]" {
		t.Fatalf("visible = %s, %v", convIDs(convs), err)
	}
	if convs[0].ClearedSeq != 2 || convs[0].ReadSeq != 2 || !convs[1].NotifyOff {
		t.Errorf("rows = %+v", convs)
	}
	if cv, ok, _ := s.GetConversation(c, "a", "c2c:a:h1"); !ok || cv.ClearedSeq != 3 || cv.DeletedAt != 10 {
		t.Errorf("GetConversation of a hidden one = %+v, %v", cv, ok)
	}
	if convs, _, _ := s.ListConversations(c, "b", 10); len(convs) != 2 {
		t.Errorf("b affected: %s", convIDs(convs))
	}
	saveAt(t, s, "c2c:a:h1", "h1", "x4", 200, "back")
	if _, _, err := s.SaveMessage(c, store.Message{MsgID: "e2-1", ClientMsgID: "e2-1", ConvID: "group:e2", From: "b", Type: "text", Body: []byte(`{"text":"hi"}`), TS: 300}); err != nil {
		t.Fatal(err)
	}
	if convs, _, _ := s.ListConversations(c, "a", 10); convIDs(convs) != "[group:e2 c2c:a:h1 c2c:a:h2 group:e1]" {
		t.Errorf("after new messages = %s", convIDs(convs))
	}
}
