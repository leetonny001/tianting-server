package storetest

import (
	"context"
	"errors"
	"testing"

	"tianting/server/internal/store"
)

// C2 文件的存储契约（PRD-chat 第 12.1 节、F-FILE-01、06，BE-431～BE-437）。

const fileT0 = int64(2_000_000_000_000)

func newUpload(id, owner string) store.File {
	return store.File{FileID: id, Owner: owner, Kind: store.KindFile, Name: "a.txt", DeclaredSize: 10, DeclaredMime: "text/plain",
		TmpKey: "tmp/" + id, PutExpiresAt: fileT0 + 900_000, CreatedAt: fileT0}
}

func limits(daily int64, pending int) store.UploadLimits {
	return store.UploadLimits{Day: "2033-05-18", DailyBytes: daily, DailyDeclared: 4 * daily, PendingMax: pending, NowReal: fileT0}
}

func commitResult(attempt string) store.CompleteResult {
	return store.CompleteResult{Attempt: attempt, ObjectKey: "final/x/" + attempt + "/original", Size: 10, Mime: "text/plain", Day: "2033-05-18", DailyBytes: 100, Now: fileT0}
}

// FilesUploadLimits：额度（已用 + 声明）、声明合计、未完成数（只数 PUT 地址没过期的）；scan_state 默认 skipped。
func testFilesUploadLimits(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateUpload(ctx, newUpload("f1", "u1"), limits(100, 2)); err != nil {
		t.Fatal(err)
	}
	f, err := s.GetFile(ctx, "f1")
	if err != nil || f.State != store.FilePending || f.ScanState != "skipped" || f.CompletingUntil != 0 {
		t.Fatalf("file = %+v, %v", f, err)
	}
	if err := s.CreateUpload(ctx, newUpload("f2", "u1"), limits(100, 2)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUpload(ctx, newUpload("f3", "u1"), limits(100, 2)); !errors.Is(err, store.ErrUploadPendingLimit) {
		t.Fatalf("third pending: %v", err)
	}
	lim := limits(100, 2)
	lim.NowReal = fileT0 + 900_001 // 前两个的 PUT 地址都过期了
	if err := s.CreateUpload(ctx, newUpload("f3", "u1"), lim); err != nil {
		t.Fatalf("after the PUT urls expired: %v", err)
	}
	big := newUpload("f4", "u2")
	big.DeclaredSize = 101
	if err := s.CreateUpload(ctx, big, limits(100, 20)); !errors.Is(err, store.ErrUploadQuota) {
		t.Fatalf("over quota: %v", err)
	}
	lim = limits(100, 20)
	lim.DailyDeclared = 25
	for i, id := range []string{"d1", "d2"} {
		if err := s.CreateUpload(ctx, newUpload(id, "u3"), lim); err != nil {
			t.Fatalf("declared %d: %v", i, err)
		}
	}
	if err := s.CreateUpload(ctx, newUpload("d3", "u3"), lim); !errors.Is(err, store.ErrUploadDeclared) {
		t.Fatalf("declared limit: %v", err)
	}
	if _, err := s.GetFile(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

// FilesCompleteLease：租约互斥、按 attempt 释放、过期后可以再取；提交只匹配 pending，复查额度（已用量已达额度才拒绝）。
func testFilesCompleteLease(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for _, id := range []string{"f1", "f2", "f3"} {
		if err := s.CreateUpload(ctx, newUpload(id, "u1"), limits(1000, 20)); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok, _ := s.AcquireCompleteLease(ctx, "f1", "u2", "a0", fileT0+60_000, fileT0); ok {
		t.Fatal("lease for someone else's file")
	}
	if _, ok, err := s.AcquireCompleteLease(ctx, "f1", "u1", "a1", fileT0+60_000, fileT0); !ok || err != nil {
		t.Fatalf("first lease: %v %v", ok, err)
	}
	if _, ok, _ := s.AcquireCompleteLease(ctx, "f1", "u1", "a2", fileT0+60_000, fileT0+1); ok {
		t.Fatal("second lease while the first is held")
	}
	_ = s.ReleaseCompleteLease(ctx, "f1", "a2") // 别人的 attempt 释放不了
	if _, ok, _ := s.AcquireCompleteLease(ctx, "f1", "u1", "a2", fileT0+60_000, fileT0+1); ok {
		t.Fatal("released by the wrong attempt")
	}
	if _, ok, _ := s.AcquireCompleteLease(ctx, "f1", "u1", "a2", fileT0+200_000, fileT0+60_001); !ok {
		t.Fatal("expired lease not reusable")
	}
	if err := s.ReleaseCompleteLease(ctx, "f1", "a2"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.AcquireCompleteLease(ctx, "f1", "u1", "a3", fileT0+60_000, fileT0+2); !ok {
		t.Fatal("released lease not reusable")
	}
	if err := s.CommitComplete(ctx, "f1", commitResult("a3")); err != nil {
		t.Fatal(err)
	}
	f, _ := s.GetFile(ctx, "f1")
	if f.State != store.FileReady || f.ObjectKey != "final/x/a3/original" || f.CompletingUntil != 0 {
		t.Fatalf("after commit: %+v", f)
	}
	if err := s.CommitComplete(ctx, "f1", commitResult("a4")); !errors.Is(err, store.ErrStateChanged) {
		t.Fatalf("second commit: %v", err)
	}
	if used, _ := s.UsedUploadBytes(ctx, "u1", "2033-05-18"); used != 10 {
		t.Fatalf("used = %d", used)
	}
	r := commitResult("b1")
	r.DailyBytes = 20
	r.Size = 15
	if err := s.CommitComplete(ctx, "f2", r); err != nil { // 已用 10 < 20：可以超额一个文件
		t.Fatal(err)
	}
	if err := s.CommitComplete(ctx, "f3", r); !errors.Is(err, store.ErrUploadQuota) { // 已用 25 ≥ 20
		t.Fatalf("over quota commit: %v", err)
	}
	if f, _ := s.GetFile(ctx, "f3"); f.State != store.FilePending {
		t.Fatalf("f3 = %s", f.State)
	}
}

// FilesCancelAndPurge：取消只匹配本人 pending；超过时限的 pending（无租约或租约过期）和 ready 改 purging；删行只删 purging。
func testFilesCancelAndPurge(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for _, id := range []string{"f1", "f2", "f3", "f4"} {
		if err := s.CreateUpload(ctx, newUpload(id, "u1"), limits(1000, 20)); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := s.CancelUpload(ctx, "f1", "u2"); ok {
		t.Fatal("cancelled someone else's upload")
	}
	if ok, _ := s.CancelUpload(ctx, "f1", "u1"); !ok {
		t.Fatal("cancel failed")
	}
	s.AcquireCompleteLease(ctx, "f2", "u1", "a", fileT0+10_000_000, fileT0)
	s.AcquireCompleteLease(ctx, "f3", "u1", "b", fileT0+60_000, fileT0)
	s.CommitComplete(ctx, "f3", commitResult("b"))
	n, err := s.MarkStalePurging(ctx, fileT0+1, fileT0+100_000, 100)
	if err != nil || n != 2 { // f3（ready）、f4（无租约）；f2 的租约还在
		t.Fatalf("marked %d, %v", n, err)
	}
	if f, _ := s.GetFile(ctx, "f2"); f.State != store.FilePending {
		t.Fatalf("leased pending marked: %s", f.State)
	}
	ps, _ := s.ListPurging(ctx, 100)
	if len(ps) != 3 {
		t.Fatalf("purging = %d", len(ps))
	}
	if err := s.DeleteFileRow(ctx, "f2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFile(ctx, "f2"); err != nil {
		t.Fatal("deleted a pending row")
	}
	s.DeleteFileRow(ctx, "f1")
	if _, err := s.GetFile(ctx, "f1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("purging row not deleted")
	}
}

// FilesBindAndExpire：SaveMessage 只绑定本人 ready 的文件，失败整条不保存；过期清空文件名并递增会话 mod_ver。
func testFilesBindAndExpire(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	for _, id := range []string{"f1", "f2"} {
		s.CreateUpload(ctx, newUpload(id, "a"), limits(1000, 20))
	}
	s.AcquireCompleteLease(ctx, "f1", "a", "x", fileT0+60_000, fileT0)
	s.CommitComplete(ctx, "f1", commitResult("x"))
	m := newMsg("m1", "c2c:a:b", "a", "client-01")
	m.Body, m.Type, m.AttachFileID, m.AttachRetainUntil = []byte(`{"file":{"fileId":"f2"}}`), "file", "f2", fileT0+1000
	m.ConvUsers = []string{"a", "b"}
	if _, _, err := s.SaveMessage(ctx, m); !errors.Is(err, store.ErrFileNotReady) {
		t.Fatalf("pending file bound: %v", err)
	}
	m.AttachFileID, m.Body = "f1", []byte(`{"file":{"fileId":"f1"}}`)
	m.From = "b"
	if _, _, err := s.SaveMessage(ctx, m); !errors.Is(err, store.ErrFileNotReady) {
		t.Fatalf("someone else's file bound: %v", err)
	}
	if msgs, _, _ := s.ListMessages(ctx, "c2c:a:b", 0, 10); len(msgs) != 0 {
		t.Fatal("message saved although the binding failed")
	}
	m.From = "a"
	saved, _, err := s.SaveMessage(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := s.GetFile(ctx, "f1")
	if f.State != store.FileAttached || f.MsgID != saved.MsgID || f.ConvID != "c2c:a:b" || f.RetainUntil != fileT0+1000 {
		t.Fatalf("bound file = %+v", f)
	}
	acc, _ := s.FileAccessInfo(ctx, []string{"f1"})
	if a := acc["f1"]; !a.MsgFound || a.MsgSeq != saved.Seq || a.MsgDeleted {
		t.Fatalf("access = %+v", a)
	}
	if out, _ := s.ExpireFiles(ctx, fileT0+1000, 10); len(out) != 0 {
		t.Fatal("expired at retain_until")
	}
	out, err := s.ExpireFiles(ctx, fileT0+1001, 10)
	if err != nil || len(out) != 1 {
		t.Fatalf("expire = %v, %v", out, err)
	}
	f, _ = s.GetFile(ctx, "f1")
	if f.State != store.FileExpired || f.Name != "" {
		t.Fatalf("expired file = %+v", f)
	}
	c, ok, _ := s.GetConversation(ctx, "a", "c2c:a:b")
	if !ok || c.ModVer != 1 {
		t.Fatalf("modVer = %d", c.ModVer)
	}
	del, _ := s.ListObjectsToDelete(ctx, fileT0+2000, 10)
	if len(del) != 1 {
		t.Fatalf("objects to delete = %d", len(del))
	}
	s.MarkObjectsDeleted(ctx, "f1")
	if del, _ := s.ListObjectsToDelete(ctx, fileT0+2000, 10); len(del) != 0 {
		t.Fatal("deleted objects listed again")
	}
}

// FilesAvatar：公开键只写一次；绑定改 attached，原来的头像记 avatar_replaced_at，绑回清零；换头像超过时限改 purging。
func testFilesAvatar(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.EnsureUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a1", "a2"} {
		f := newUpload(id, "u1")
		f.Kind = store.KindAvatar
		s.CreateUpload(ctx, f, limits(1000, 20))
		s.AcquireCompleteLease(ctx, id, "u1", "x", fileT0+60_000, fileT0)
		s.CommitComplete(ctx, id, commitResult("x"))
	}
	f, _ := s.SetAvatarPublicKey(ctx, "a1", "u1", "avatars/k1")
	if f.PublicKey != "avatars/k1" {
		t.Fatalf("key = %q", f.PublicKey)
	}
	if f, _ := s.SetAvatarPublicKey(ctx, "a1", "u1", "avatars/k2"); f.PublicKey != "avatars/k1" {
		t.Fatalf("key overwritten: %q", f.PublicKey)
	}
	if err := s.BindAvatar(ctx, "a1", "u1", "https://cdn/avatars/k1", fileT0); err != nil {
		t.Fatal(err)
	}
	if err := s.BindAvatar(ctx, "a2", "u1", "https://cdn/avatars/k2", fileT0+10); err != nil {
		t.Fatal(err)
	}
	if f, _ := s.GetFile(ctx, "a1"); f.AvatarReplacedAt != fileT0+10 {
		t.Fatalf("replaced_at = %d", f.AvatarReplacedAt)
	}
	if u, _ := s.GetUser(ctx, "u1"); u.Avatar != "https://cdn/avatars/k2" {
		t.Fatalf("avatar = %s", u.Avatar)
	}
	s.BindAvatar(ctx, "a1", "u1", "https://cdn/avatars/k1", fileT0+20) // 绑回
	if f, _ := s.GetFile(ctx, "a1"); f.AvatarReplacedAt != 0 {
		t.Fatal("bind back did not clear replaced_at")
	}
	if n, _ := s.MarkReplacedAvatarsPurging(ctx, fileT0+21, 10); n != 1 {
		t.Fatalf("marked %d", n)
	}
	if f, _ := s.GetFile(ctx, "a2"); f.State != store.FilePurging {
		t.Fatalf("a2 = %s", f.State)
	}
	if err := s.BindAvatar(ctx, "a2", "u1", "x", fileT0+30); !errors.Is(err, store.ErrFileNotReady) {
		t.Fatalf("bind a purging avatar: %v", err)
	}
}
