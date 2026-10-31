package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func openTest(t *testing.T) *SQLite {
	t.Helper()
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "im.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newMsg(id, conv, from, clientID string) Message {
	return Message{MsgID: id, ClientMsgID: clientID, ConvID: conv, From: from, Type: "text", Body: []byte(`{"text":"hi"}`), TS: 1}
}

func TestSaveMessageAssignsSeqPerConversation(t *testing.T) {
	s, ctx := openTest(t), context.Background()

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

func TestSaveMessageDeduplicates(t *testing.T) {
	s, ctx := openTest(t), context.Background()

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

func TestSaveMessageConcurrentSeqIsContiguous(t *testing.T) {
	s, ctx := openTest(t), context.Background()
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

func TestListMessagesPaging(t *testing.T) {
	s, ctx := openTest(t), context.Background()
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

func TestGroups(t *testing.T) {
	s, ctx := openTest(t), context.Background()

	if err := s.CreateGroup(ctx, "g1", "Group 1", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateGroup(ctx, "g1", "dup", nil); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate create: %v", err)
	}
	if err := s.AddGroupMembers(ctx, "g1", []string{"b", "c"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddGroupMembers(ctx, "nope", []string{"a"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("add to missing group: %v", err)
	}
	members, _ := s.GroupMembers(ctx, "g1")
	if fmt.Sprint(members) != "[a b c]" {
		t.Errorf("members = %v", members)
	}
	if ok, _ := s.IsGroupMember(ctx, "g1", "c"); !ok {
		t.Error("c should be member")
	}
	if ok, _ := s.IsGroupMember(ctx, "g1", "z"); ok {
		t.Error("z should not be member")
	}
}

func TestReadSeqOnlyMovesForward(t *testing.T) {
	s, ctx := openTest(t), context.Background()
	s.SetReadSeq(ctx, "a", "c2c:a:b", 5)
	s.SetReadSeq(ctx, "a", "c2c:a:b", 3)
	if got, _ := s.ReadSeq(ctx, "a", "c2c:a:b"); got != 5 {
		t.Errorf("read seq = %d, want 5", got)
	}
	if got, _ := s.ReadSeq(ctx, "a", "group:none"); got != 0 {
		t.Errorf("unknown read seq = %d", got)
	}
}
