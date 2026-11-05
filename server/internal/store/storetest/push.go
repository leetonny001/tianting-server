package storetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"tianting/server/internal/store"
)

// 系统推送的存储契约（PRD-live v1.0 4.3、4.4 节）。

const pushT0 = int64(1_000_000)

func pushWrite(pushID string, expires int64, users ...string) store.PushInboxWrite {
	n := len(users)
	return store.PushInboxWrite{
		Record:  store.PushRecord{PushID: pushID, Caller: "c1", ContentHash: "h-" + pushID, Target: "user", Accepted: &n, FirstTS: pushT0, CreatedAt: pushT0},
		UserIDs: users, Type: "notice", Data: json.RawMessage(`{"n":1}`), ExpiresAt: expires,
	}
}

func mustPushInbox(t *testing.T, s store.Store, w store.PushInboxWrite) store.PushInboxResult {
	t.Helper()
	res, err := s.WritePushInbox(context.Background(), w)
	if err != nil {
		t.Fatalf("WritePushInbox(%s): %v", w.Record.PushID, err)
	}
	return res
}

func pushSeqs(rows []store.PushInboxRow) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.PushSeq
	}
	return out
}

func testPushSeqAndRead(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for i := range 5 {
		res := mustPushInbox(t, s, pushWrite(fmt.Sprintf("p%d", i), pushT0+1000, "u2", "u1"))
		if res.Seqs["u1"] != int64(i+1) || res.Seqs["u2"] != int64(i+1) {
			t.Fatalf("p%d seqs = %v", i, res.Seqs)
		}
	}
	rows, more, latest, err := s.ReadPushInbox(ctx, "u1", 1, 2, pushT0)
	if err != nil || !more || latest != 5 || fmt.Sprint(pushSeqs(rows)) != "[2 3]" {
		t.Fatalf("page = %v more=%v latest=%d err=%v", pushSeqs(rows), more, latest, err)
	}
	if rows[0].PushID != "p1" || rows[0].Type != "notice" || string(rows[0].Data) != `{"n":1}` || rows[0].CreatedAt != pushT0 {
		t.Fatalf("row = %+v", rows[0])
	}
	// 到期：now == expires_at 不返回，早 1 毫秒返回（AC-PUSH-03-12）。
	if rows, _, _, _ := s.ReadPushInbox(ctx, "u1", 0, 10, pushT0+1000); len(rows) != 0 {
		t.Fatalf("at expires_at: %v", pushSeqs(rows))
	}
	if rows, _, _, _ := s.ReadPushInbox(ctx, "u1", 0, 10, pushT0+999); len(rows) != 5 {
		t.Fatalf("1ms before expires_at: %v", pushSeqs(rows))
	}
	// 没有推送过的用户：序号 0，结果是空切片。
	rows, more, latest, err = s.ReadPushInbox(ctx, "ghost", 0, 10, pushT0)
	if err != nil || more || latest != 0 || rows == nil || len(rows) != 0 {
		t.Fatalf("ghost = %v %v %d %v", rows, more, latest, err)
	}
}

func testPushExpiredGaps(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	mustPushInbox(t, s, pushWrite("a", pushT0+10, "u1"))
	mustPushInbox(t, s, pushWrite("b", pushT0+1000, "u1"))
	mustPushInbox(t, s, pushWrite("c", pushT0+10, "u1"))
	// 后面只剩过期的行时 hasMore=false，latestSeq 仍是 3（AC-PUSH-03-8）。
	rows, more, latest, err := s.ReadPushInbox(ctx, "u1", 0, 1, pushT0+500)
	if err != nil || more || latest != 3 || fmt.Sprint(pushSeqs(rows)) != "[2]" {
		t.Fatalf("rows=%v more=%v latest=%d err=%v", pushSeqs(rows), more, latest, err)
	}
}

func testPushIdempotencyRecord(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if _, err := s.GetPushRecord(ctx, "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing record: %v", err)
	}
	mustPushInbox(t, s, pushWrite("p1", pushT0+1000, "u1"))
	r, err := s.GetPushRecord(ctx, "p1")
	if err != nil || r.State != store.PushDone || r.Caller != "c1" || r.Accepted == nil || *r.Accepted != 1 || r.FirstTS != pushT0 {
		t.Fatalf("record = %+v, %v", r, err)
	}
	// 同一个 pushId 再写：返回已有记录，收件箱和序号都不变。
	res := mustPushInbox(t, s, pushWrite("p1", pushT0+1000, "u1", "u2"))
	if res.Existing == nil || res.Existing.PushID != "p1" {
		t.Fatalf("existing = %+v", res)
	}
	if latest, _ := s.LatestPushSeq(ctx, "u1"); latest != 1 {
		t.Fatalf("u1 latest = %d", latest)
	}
	if latest, _ := s.LatestPushSeq(ctx, "u2"); latest != 0 {
		t.Fatalf("u2 latest = %d", latest)
	}
	// pending：all 没有 accepted；再写返回已有记录；MarkPushDone 后是 done。
	all := store.PushRecord{PushID: "p2", Caller: "c1", ContentHash: "h", Target: "all", FirstTS: 7, CreatedAt: 7}
	if existing, err := s.CreatePendingPush(ctx, all); err != nil || existing != nil {
		t.Fatalf("create pending: %+v %v", existing, err)
	}
	existing, err := s.CreatePendingPush(ctx, store.PushRecord{PushID: "p2", Caller: "c2", ContentHash: "x", Target: "all", FirstTS: 8, CreatedAt: 8})
	if err != nil || existing == nil || existing.Caller != "c1" || existing.State != store.PushPending || existing.Accepted != nil || existing.FirstTS != 7 {
		t.Fatalf("second pending: %+v %v", existing, err)
	}
	if err := s.MarkPushDone(ctx, "p2"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.GetPushRecord(ctx, "p2"); r.State != store.PushDone {
		t.Fatalf("state = %s", r.State)
	}
}

func testPushRollback(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	users := make([]string, 450)
	for i := range users {
		users[i] = fmt.Sprintf("u%03d", i)
	}
	w := pushWrite("p1", pushT0+1000, users...)
	boom := errors.New("boom")
	w.FailAt = func(i int) error {
		if i == 449 { // 前面的多行语句都已经执行过
			return boom
		}
		return nil
	}
	if _, err := s.WritePushInbox(ctx, w); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.GetPushRecord(ctx, "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("record survived: %v", err)
	}
	if n, _ := s.CountPushInbox(ctx); n != 0 {
		t.Fatalf("inbox rows = %d", n)
	}
	if latest, _ := s.LatestPushSeq(ctx, "u000"); latest != 0 {
		t.Fatalf("seq survived: %d", latest)
	}
}

func testPushEvict(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	evicted := 0
	for i := range 5 {
		w := pushWrite(fmt.Sprintf("p%d", i), pushT0+1000, "u1")
		w.InboxMax = 3
		evicted += mustPushInbox(t, s, w).Evicted
	}
	rows, _, latest, err := s.ReadPushInbox(ctx, "u1", 0, 10, pushT0)
	if err != nil || latest != 5 || fmt.Sprint(pushSeqs(rows)) != "[3 4 5]" || evicted != 2 {
		t.Fatalf("rows=%v latest=%d evicted=%d err=%v", pushSeqs(rows), latest, evicted, err)
	}
}

func testPushPurge(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	mustPushInbox(t, s, pushWrite("p1", pushT0+10, "u1", "u2"))
	// 第一批只删一行：幂等记录还有对应收件箱行，不能删。
	inbox, records, err := s.PurgePush(ctx, pushT0+10, pushT0+10, 1)
	if err != nil || inbox != 1 || records != 0 {
		t.Fatalf("batch 1: %d %d %v", inbox, records, err)
	}
	if _, err := s.GetPushRecord(ctx, "p1"); err != nil {
		t.Fatalf("record deleted while an inbox row remains: %v", err)
	}
	// 第二批：收件箱清空，记录随后删除；序号不回退。
	inbox, records, err = s.PurgePush(ctx, pushT0+10, pushT0+10, 1)
	if err != nil || inbox != 1 || records != 1 {
		t.Fatalf("batch 2: %d %d %v", inbox, records, err)
	}
	if _, err := s.GetPushRecord(ctx, "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("record not purged: %v", err)
	}
	for _, u := range []string{"u1", "u2"} {
		if latest, _ := s.LatestPushSeq(ctx, u); latest != 1 {
			t.Fatalf("%s latest = %d after purge", u, latest)
		}
	}
	// 没到期的行、保留期内的记录不动。
	mustPushInbox(t, s, pushWrite("p2", pushT0+100, "u1"))
	inbox, records, err = s.PurgePush(ctx, pushT0+99, pushT0-1, 10)
	if err != nil || inbox != 0 || records != 0 {
		t.Fatalf("not due: %d %d %v", inbox, records, err)
	}
	if res := mustPushInbox(t, s, pushWrite("p3", pushT0+100, "u1")); res.Seqs["u1"] != 3 {
		t.Fatalf("seq after purge = %v", res.Seqs)
	}
}
