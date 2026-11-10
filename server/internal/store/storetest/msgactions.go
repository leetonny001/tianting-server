package storetest

import (
	"context"
	"errors"
	"testing"

	"tianting/server/internal/store"
)

// C4 撤回、引用、@ 的契约用例（BE-471 第 1 条）。Postgres 实现随 B6 跑同一套。

func msgActionCases() []struct {
	name string
	fn   func(*testing.T, Opener)
} {
	return []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"MsgActionsRecall", testMsgActionsRecall},
		{"MsgActionsRecallRollback", testMsgActionsRecallRollback},
		{"MsgActionsMentionCounts", testMsgActionsMentionCounts},
		{"MsgActionsMentionAllQuota", testMsgActionsMentionAllQuota},
		{"MsgActionsQuoteColumn", testMsgActionsQuoteColumn},
	}
}

func sealOK(msgID, convID string, body []byte) ([]byte, []byte, string, error) {
	return []byte("nonce"), append([]byte("ct:"), byte(len(body))), "k1", nil
}

func groupMsg(id, from, client, text string) store.Message {
	return store.Message{MsgID: id, ClientMsgID: client, ConvID: "group:g1", From: from, Type: "text", Body: []byte(`{"text":"` + text + `"}`), TS: 1000, Summary: text}
}

func testMsgActionsRecall(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	m1, _, err := s.SaveMessage(ctx, groupMsg("m1", "a", "client-01", "secret-1"))
	if err != nil {
		t.Fatal(err)
	}
	before, _, _ := s.GetConversation(ctx, "b", "group:g1")
	req := store.RecallReq{ConvID: "group:g1", MsgID: "m1", By: "a", Now: 2000, ExpiresAt: 9000, Seal: sealOK}
	res, err := s.RecallMessage(ctx, req)
	if err != nil || res.Already || res.Seq != m1.Seq {
		t.Fatalf("recall: %+v %v", res, err)
	}
	got, err := s.GetMessage(ctx, "m1")
	if err != nil || got.RecalledAt != 2000 || got.RecalledBy != "a" || got.DeletedAt != 0 || string(got.Body) != "{}" || got.Seq != m1.Seq {
		t.Fatalf("after recall: %+v %v", got, err)
	}
	conv, _, _ := s.GetConversation(ctx, "b", "group:g1")
	if !conv.Last.Recalled || conv.Last.RecalledBy != "a" || conv.Last.Text != "" || conv.Last.Deleted || conv.ModVer != before.ModVer+1 || conv.UpdatedAt != before.UpdatedAt {
		t.Errorf("conversation after recall: %+v (before %+v)", conv, before)
	}
	q, _, err := s.ListQuarantined(ctx, "group:g1", 100, 0, 10)
	if err != nil || len(q) != 1 || q[0].Message.RecalledAt != 2000 || q[0].ExpiresAt != 9000 {
		t.Fatalf("quarantine: %+v %v", q, err)
	}
	// 幂等：不改任何东西。
	if res, err := s.RecallMessage(ctx, req); err != nil || !res.Already {
		t.Errorf("second recall: %+v %v", res, err)
	}
	if again, _, _ := s.GetConversation(ctx, "b", "group:g1"); again.ModVer != conv.ModVer {
		t.Errorf("idempotent recall bumped mod_ver: %d -> %d", conv.ModVer, again.ModVer)
	}
	// 其他会话、不存在：ErrNotFound。
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g2", MsgID: "m1", By: "a", Seal: sealOK}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other conversation: %v", err)
	}
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: "ghost", By: "a", Seal: sealOK}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	// 后台删除过的：ErrNotFound；已撤回的后台删除：幂等，不写 deleted_at。
	if _, _, err := s.SaveMessage(ctx, groupMsg("m2", "a", "client-02", "two")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteMessages(ctx, store.DeleteMessagesReq{ConvID: "group:g1", MsgIDs: []string{"m2"}, By: "x:op", Now: 3000, ExpiresAt: 9000, Seal: sealOK}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: "m2", By: "a", Seal: sealOK}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("recall of an admin-deleted message: %v", err)
	}
	del, err := s.DeleteMessages(ctx, store.DeleteMessagesReq{ConvID: "group:g1", MsgIDs: []string{"m1"}, By: "x:op", Now: 3000, ExpiresAt: 9000, Seal: sealOK})
	if err != nil || len(del) != 0 {
		t.Errorf("delete of a recalled message: %+v %v", del, err)
	}
	if got, _ := s.GetMessage(ctx, "m1"); got.DeletedAt != 0 || got.RecalledAt == 0 {
		t.Errorf("recalled message after admin delete: %+v", got)
	}
	// tip：ErrMsgNotDeletable。
	tip := groupMsg("t1", "", "tip-0001", "")
	tip.Type, tip.Body = "tip", []byte(`{"tip":{"kind":"renamed"}}`)
	if _, _, err := s.SaveMessage(ctx, tip); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: "t1", By: "a", Seal: sealOK}); !errors.Is(err, store.ErrMsgNotDeletable) {
		t.Errorf("tip: %v", err)
	}
}

func testMsgActionsRecallRollback(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	m := groupMsg("m1", "a", "client-01", "secret-1")
	m.Mentions = []string{"b"}
	if _, _, err := s.SaveMessage(ctx, m); err != nil {
		t.Fatal(err)
	}
	before, _, _ := s.GetConversation(ctx, "b", "group:g1")
	fail := func(string, string, []byte) ([]byte, []byte, string, error) {
		return nil, nil, "", errors.New("injected")
	}
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: "m1", By: "a", Now: 2000, ExpiresAt: 9000, Seal: fail}); err == nil {
		t.Fatal("seal failure must fail the recall")
	}
	got, _ := s.GetMessage(ctx, "m1")
	after, _, _ := s.GetConversation(ctx, "b", "group:g1")
	if got.RecalledAt != 0 || string(got.Body) != `{"text":"secret-1"}` || after.ModVer != before.ModVer || after.Last.Recalled || after.Last.Text != "secret-1" {
		t.Errorf("recall not rolled back: %+v %+v", got, after)
	}
	if c, _ := s.MentionCounts(ctx, "b"); c["group:g1"].Unread != 1 {
		t.Errorf("mention removed by a rolled-back recall: %+v", c)
	}
}

func testMsgActionsMentionCounts(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	quota := &store.MentionAllQuota{GroupID: "g1", Hourly: 10, Daily: 10, Day: 20261110, DayEnd: 1 << 50}
	save := func(id, client string, mentions []string, all bool) store.Message {
		t.Helper()
		m := groupMsg(id, "a", client, "x")
		m.Mentions = mentions
		if all {
			m.MentionAll = quota
		}
		saved, _, err := s.SaveMessage(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	save("m1", "client-01", nil, false)
	m2 := save("m2", "client-02", []string{"b", "a"}, false) // a 自己不算
	save("m3", "client-03", nil, false)
	m4 := save("m4", "client-04", []string{"b"}, false)
	m5 := save("m5", "client-05", []string{"b"}, true) // 同一条 @ b 又 @ 所有人：只算一条
	counts := func(uid string) store.MentionCount {
		t.Helper()
		c, err := s.MentionCounts(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		return c["group:g1"]
	}
	if c := counts("b"); c.Unread != 3 || c.First != m2.Seq {
		t.Errorf("b: %+v", c)
	}
	if c := counts("c"); c.Unread != 1 || c.First != m5.Seq {
		t.Errorf("c (only @all): %+v", c)
	}
	if c := counts("a"); c.Unread != 0 {
		t.Errorf("a mentioned itself / sent @all: %+v", c)
	}
	// 已读前进后重算。
	if _, _, err := s.SetReadSeq(ctx, "b", "group:g1", m2.Seq); err != nil {
		t.Fatal(err)
	}
	if c := counts("b"); c.Unread != 2 || c.First != m4.Seq {
		t.Errorf("b after reading m2: %+v", c)
	}
	// 撤回、后台删除的不计入。
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: "m4", By: "a", Now: 2, ExpiresAt: 9, Seal: sealOK}); err != nil {
		t.Fatal(err)
	}
	if c := counts("b"); c.Unread != 1 || c.First != m5.Seq {
		t.Errorf("b after recall: %+v", c)
	}
	if _, err := s.DeleteMessages(ctx, store.DeleteMessagesReq{ConvID: "group:g1", MsgIDs: []string{"m5"}, By: "x:op", Now: 3, ExpiresAt: 9, Seal: sealOK}); err != nil {
		t.Fatal(err)
	}
	if c := counts("b"); c.Unread != 0 {
		t.Errorf("b after admin delete: %+v", c)
	}
	if c := counts("c"); c.Unread != 0 {
		t.Errorf("c after admin delete of @all: %+v", c)
	}
	// 提及表写失败：整条消息不保存。
	m := groupMsg("m6", "a", "client-06", "x")
	m.Mentions = []string{"b"}
	m.Fault = func(p string) error {
		if p == store.FaultMentionWrite {
			return errors.New("injected")
		}
		return nil
	}
	if _, _, err := s.SaveMessage(ctx, m); !errors.Is(err, store.ErrMentionWrite) {
		t.Errorf("mention write fault: %v", err)
	}
	if _, err := s.GetMessage(ctx, "m6"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("message saved despite the mention write failure: %v", err)
	}
	// 退群的不计算。
	if _, _, err := s.SaveMessage(ctx, func() store.Message { x := groupMsg("m7", "a", "client-07", "x"); x.Mentions = []string{"c"}; return x }()); err != nil {
		t.Fatal(err)
	}
	if c := counts("c"); c.Unread != 1 {
		t.Errorf("c before leaving: %+v", c)
	}
	if _, _, err := s.RemoveGroupMembers(ctx, "g1", []string{"c"}, 10); err != nil {
		t.Fatal(err)
	}
	if c := counts("c"); c.Unread != 0 {
		t.Errorf("c after leaving: %+v", c)
	}
}

func testMsgActionsMentionAllQuota(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	const t0 = int64(1_800_000_000_000)
	q := &store.MentionAllQuota{GroupID: "g1", Hourly: 3, Daily: 4, Day: 20270115, DayEnd: t0 + 10*3600_000}
	n := 0
	send := func(ts int64, fault func(string) error) (store.Message, error) {
		n++
		m := groupMsg("ma"+string(rune('a'+n)), "a", "client-a"+string(rune('a'+n)), "@all")
		m.TS, m.MentionAll, m.Fault = ts, q, fault
		saved, _, err := s.SaveMessage(ctx, m)
		return saved, err
	}
	for i := range 3 {
		if _, err := send(t0+int64(i)*60_000, nil); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	var lim *store.MentionAllLimitError
	_, err := send(t0+10*60_000, nil)
	if !errors.As(err, &lim) || lim.RetryAfter != 50*60 {
		t.Fatalf("4th within the hour: %v %+v (want retry after 3000s)", err, lim)
	}
	conv, _, _ := s.GetConversation(ctx, "b", "group:g1")
	if conv.MaxSeq != 3 {
		t.Errorf("rejected message took a seq: %d", conv.MaxSeq)
	}
	// 滚动 60 分钟：第一条出窗口后可以再发一次。
	if _, err := send(t0+3600_000, nil); err != nil {
		t.Errorf("after the first left the window: %v", err)
	}
	// 当天第 5 次：按自然日到次日零点。
	_, err = send(t0+3*3600_000, nil)
	if !errors.As(err, &lim) || lim.RetryAfter != 7*3600 {
		t.Errorf("daily limit: %v %+v", err, lim)
	}
	// 撤回不退次数。
	if _, err := s.RecallMessage(ctx, store.RecallReq{ConvID: "group:g1", MsgID: conv.Last.MsgID, By: "a", Now: t0, ExpiresAt: t0 + 1, Seal: sealOK}); err != nil {
		t.Fatal(err)
	}
	if _, err := send(t0+3*3600_000, nil); !errors.As(err, &lim) {
		t.Errorf("recall refunded the counter: %v", err)
	}
	// 计数不可用：不放行。
	q.Day, q.DayEnd = 20270116, t0+40*3600_000
	_, err = send(t0+30*3600_000, func(p string) error {
		if p == store.FaultMentionAllCounter {
			return errors.New("injected")
		}
		return nil
	})
	if !errors.Is(err, store.ErrMentionAllCounter) {
		t.Errorf("counter fault: %v", err)
	}
	if _, err := send(t0+30*3600_000, nil); err != nil {
		t.Errorf("next day: %v", err)
	}
}

func testMsgActionsQuoteColumn(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	m := groupMsg("m2", "a", "client-02", "reply")
	m.QuoteMsgID = "m1"
	if _, _, err := s.SaveMessage(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMessage(ctx, "m2")
	if err != nil || got.QuoteMsgID != "m1" || string(got.Body) != `{"text":"reply"}` {
		t.Fatalf("quote column: %+v %v", got, err)
	}
	many, err := s.GetMessages(ctx, []string{"m2", "ghost", "m2"})
	if err != nil || len(many) != 1 || many["m2"].QuoteMsgID != "m1" {
		t.Errorf("GetMessages: %+v %v", many, err)
	}
	list, _, _ := s.ListMessages(ctx, "group:g1", 0, 10)
	if len(list) != 1 || list[0].QuoteMsgID != "m1" {
		t.Errorf("ListMessages: %+v", list)
	}
}
