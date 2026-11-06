package storetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"tianting/server/internal/store"
)

func accountCases() []struct {
	name string
	fn   func(*testing.T, Opener)
} {
	return []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"AccountCreateAtomic", testAccountCreateAtomic},
		{"AccountUsernameCaseInsensitive", testAccountUsernameCaseInsensitive},
		{"AccountCreateConcurrent", testAccountCreateConcurrent},
		{"AccountInviteLimits", testAccountInviteLimits},
		{"AccountInviteMismatchWindow", testAccountInviteMismatchWindow},
		{"AccountCommitLogin", testAccountCommitLogin},
		{"AccountRecoveryAck", testAccountRecoveryAck},
		{"AccountDeviceSessions", testAccountDeviceSessions},
		{"AccountThrottle", testAccountThrottle},
		{"AccountEvents", testAccountEvents},
		{"AccountDeletedUsers", testAccountDeletedUsers},
	}
}

func newAccount(uid string) store.NewAccount {
	return store.NewAccount{
		Account:      store.Account{UserID: uid, UsernameLower: uid, PasswordHash: "$argon2id$x", NoticeVersion: "v1", CreatedAt: 1000, LastLoginAt: 1000},
		Nickname:     uid,
		RecoveryHash: "rc-" + uid,
		Session:      newSession("sid-"+uid, uid),
	}
}

func newSession(sid, uid string) store.DeviceSession {
	return store.DeviceSession{SID: sid, UserID: uid, Platform: "android", DeviceName: "d", CurrentHash: "h1", CurrentGen: 1, NextGen: 2,
		SeenIPPrefixes: []string{"203.0.113.0/24"}, CreatedAt: 1000, LastSeenAt: 1000, ExpiresAt: 9000, AbsoluteExpiresAt: 99000}
}

func invite(hash string, maxUses int) store.Invite {
	return store.Invite{InviteID: "id-" + hash, CodeHash: hash, CodeLast4: "abcd", MaxUses: maxUses, ExpiresAt: 100000, CreatedAt: 1}
}

// 事务失败时什么都不留：用户、账号、恢复码、会话、邀请码（AC-ACC-01-15）。
func testAccountCreateAtomic(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateInvite(ctx, invite("i1", 1)); err != nil {
		t.Fatal(err)
	}
	n := newAccount("alice_1")
	n.InviteHash = "i1"
	boom := errors.New("boom")
	n.BeforeCommit = func() error { return boom }
	if err := s.CreateAccount(ctx, n); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if ok, _ := s.UserExists(ctx, "alice_1"); ok {
		t.Error("user must not exist after rollback")
	}
	if _, err := s.GetAccount(ctx, "alice_1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("account: %v", err)
	}
	if _, err := s.GetDeviceSession(ctx, "sid-alice_1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("session: %v", err)
	}
	if i, _ := s.GetInviteByHash(ctx, "i1"); i.UsedCount != 0 {
		t.Errorf("invite used %d", i.UsedCount)
	}
	n.BeforeCommit = nil
	if err := s.CreateAccount(ctx, n); err != nil {
		t.Fatal(err)
	}
	a, err := s.GetAccountByUsername(ctx, "alice_1")
	if err != nil || a.UserID != "alice_1" || a.NoticeVersion != "v1" || a.CredentialsVersion != 1 {
		t.Fatalf("account %+v err %v", a, err)
	}
	if i, _ := s.GetInviteByHash(ctx, "i1"); i.UsedCount != 1 {
		t.Errorf("invite used %d", i.UsedCount)
	}
	if un, _ := s.RecoveryUnacked(ctx, "alice_1"); !un {
		t.Error("a new recovery code is unacked")
	}
}

// 注册不区分大小写，users 不加唯一约束：Bob 占用 bob，导入 BOB 不受影响（AC-ACC-01-5，C-18）。
func testAccountUsernameCaseInsensitive(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.UpsertUsers(ctx, []store.User{{UserID: "Bob", Nickname: "b"}}); err != nil {
		t.Fatal(err)
	}
	if taken, err := s.UsernameTaken(ctx, "bob"); err != nil || !taken {
		t.Fatalf("taken=%v err=%v", taken, err)
	}
	if err := s.CreateAccount(ctx, newAccount("bob")); !errors.Is(err, store.ErrExists) {
		t.Fatalf("err = %v", err)
	}
	if err := s.UpsertUsers(ctx, []store.User{{UserID: "BOB", Nickname: "B"}}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"Bob", "BOB"} {
		if ok, _ := s.UserExists(ctx, u); !ok {
			t.Errorf("%s must exist", u)
		}
	}
	if ok, _ := s.UserExists(ctx, "bob"); ok {
		t.Error("userId stays case sensitive")
	}
}

// 并发同名只有一个成功（F-ACC-01 异常）。
func testAccountCreateConcurrent(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateInvite(ctx, invite("i1", 20)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]error, 20)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := newAccount("race1")
			n.InviteHash = "i1"
			n.Session.SID = fmt.Sprint("sid-", i)
			results[i] = s.CreateAccount(ctx, n)
		}()
	}
	wg.Wait()
	okCount := 0
	for _, err := range results {
		switch {
		case err == nil:
			okCount++
		case !errors.Is(err, store.ErrExists):
			t.Errorf("err = %v", err)
		}
	}
	if okCount != 1 {
		t.Errorf("%d succeeded", okCount)
	}
	if i, _ := s.GetInviteByHash(ctx, "i1"); i.UsedCount != 1 {
		t.Errorf("invite used %d", i.UsedCount)
	}
}

func testAccountInviteLimits(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateInvite(ctx, invite("i1", 1)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.InviteUsable(ctx, "i1", 1000); !ok {
		t.Error("usable")
	}
	if ok, _ := s.InviteUsable(ctx, "i1", 100000); ok {
		t.Error("expired invite is not usable")
	}
	if ok, _ := s.InviteUsable(ctx, "nope", 1000); ok {
		t.Error("unknown invite is not usable")
	}
	a := newAccount("user1")
	a.InviteHash = "i1"
	if err := s.CreateAccount(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.InviteUsable(ctx, "i1", 1000); ok {
		t.Error("used up invite is not usable")
	}
	b := newAccount("user2")
	b.InviteHash = "i1"
	if err := s.CreateAccount(ctx, b); !errors.Is(err, store.ErrInviteInvalid) {
		t.Fatalf("err = %v", err)
	}
	if ok, _ := s.UserExists(ctx, "user2"); ok {
		t.Error("user2 must not exist")
	}
}

// 窗口内达到上限才自动停用，跨窗口重新计（AC-ACC-01-14、01-21）。
func testAccountInviteMismatchWindow(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateInvite(ctx, invite("i1", 1000)); err != nil {
		t.Fatal(err)
	}
	const window = int64(1000)
	hit := func(now int64, want bool) {
		t.Helper()
		got, err := s.RecordInviteMismatch(ctx, "i1", now, window, 3)
		if err != nil || got != want {
			t.Fatalf("now %d: disabled=%v want %v err=%v", now, got, want, err)
		}
	}
	hit(100, false)
	hit(101, false)
	hit(2000, false) // 新窗口
	hit(2001, false)
	if ok, _ := s.InviteUsable(ctx, "i1", 2002); !ok {
		t.Fatal("not disabled yet")
	}
	hit(2002, true)
	if ok, _ := s.InviteUsable(ctx, "i1", 2003); ok {
		t.Error("auto disabled invite is not usable")
	}
	if i, _ := s.GetInviteByHash(ctx, "i1"); !i.AutoDisabled || i.DisabledAt != 2002 {
		t.Errorf("invite %+v", i)
	}
}

// 第一条语句按版本锁行；不匹配什么都不写；最新一代没确认时换代（F-ACC-04 规则 8、F-ACC-05 规则 1）。
func testAccountCommitLogin(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateAccount(ctx, newAccount("user1")); err != nil {
		t.Fatal(err)
	}
	commit := func(sid string, version int64) (store.LoginResult, error) {
		return s.CommitLogin(ctx, store.LoginCommit{
			UserID: "user1", CredentialsVersion: version, Now: 2000, NewRecoveryHash: "rc-new-" + sid, Session: newSession(sid, "user1"),
			Event: store.AccountEvent{UserID: "user1", Kind: "newDeviceLogin", At: 2000, Platform: "ios", SIDHash: "sh"},
		})
	}
	if _, err := commit("stale", 9); !errors.Is(err, store.ErrCredentialsChanged) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.GetDeviceSession(ctx, "stale"); !errors.Is(err, store.ErrNotFound) {
		t.Error("a stale commit must not leave a session")
	}
	r, err := commit("s2", 1)
	if err != nil || r.RecoveryGen != 2 {
		t.Fatalf("result %+v err %v", r, err)
	}
	if err := s.AckRecoveryCode(ctx, "user1", 2, 2100); err != nil {
		t.Fatal(err)
	}
	r, err = commit("s3", 1)
	if err != nil || r.RecoveryGen != 0 {
		t.Fatalf("acked: result %+v err %v", r, err)
	}
	if a, _ := s.GetAccount(ctx, "user1"); a.LastLoginAt != 2000 {
		t.Errorf("last login %d", a.LastLoginAt)
	}
	if evs, _ := s.ListAccountEvents(ctx, "user1", 20); len(evs) != 2 {
		t.Errorf("events %d", len(evs))
	}
}

func testAccountRecoveryAck(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateAccount(ctx, newAccount("user1")); err != nil {
		t.Fatal(err)
	}
	if err := s.AckRecoveryCode(ctx, "user1", 7, 1); !errors.Is(err, store.ErrRecoveryCodeStale) {
		t.Fatalf("err = %v", err)
	}
	if err := s.AckRecoveryCode(ctx, "user1", 1, 1); err != nil {
		t.Fatal(err)
	}
	if un, _ := s.RecoveryUnacked(ctx, "user1"); un {
		t.Error("acked")
	}
}

func testAccountDeviceSessions(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateAccount(ctx, newAccount("user1")); err != nil {
		t.Fatal(err)
	}
	d, err := s.GetDeviceSession(ctx, "sid-user1")
	if err != nil {
		t.Fatal(err)
	}
	if d.CurrentGen != 1 || d.NextGen != 2 || d.RowVersion != 1 || len(d.Pendings) != 0 || d.SeenIPPrefixes[0] != "203.0.113.0/24" || d.AbsoluteExpiresAt != 99000 {
		t.Errorf("session %+v", d)
	}
	if o, err := s.OtherDeviceSessions(ctx, "user1", "sid-user1", 1000); err != nil || o.Count != 0 {
		t.Errorf("others %+v err %v", o, err)
	}
	if o, _ := s.OtherDeviceSessions(ctx, "user1", "other", 1000); o.Count != 1 || o.LastActiveAt != 1000 {
		t.Errorf("others %+v", o)
	}
	if o, _ := s.OtherDeviceSessions(ctx, "user1", "other", 9000); o.Count != 0 {
		t.Errorf("expired sessions do not count: %+v", o)
	}
}

// 固定窗口；到上限锁定，锁定期内不延长；窗口过了重新计；不同的键互不影响（F-ACC-02 规则 3）。
func testAccountThrottle(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	const window, lock = int64(1000), int64(1000)
	for i := 1; i <= 3; i++ {
		h, err := s.HitThrottle(ctx, "k1", 100, window, 3, lock)
		if err != nil || h.Count != i {
			t.Fatalf("hit %d: %+v err %v", i, h, err)
		}
		if want := int64(0); i == 3 {
			want = 1100
			if h.LockedUntil != want {
				t.Errorf("locked until %d", h.LockedUntil)
			}
		} else if h.LockedUntil != want {
			t.Errorf("hit %d locked %d", i, h.LockedUntil)
		}
	}
	if h, _ := s.PeekThrottle(ctx, "k2", 100, window); h.Count != 0 || h.LockedUntil != 0 {
		t.Errorf("other key %+v", h)
	}
	if h, _ := s.PeekThrottle(ctx, "k1", 1099, window); h.LockedUntil != 1100 {
		t.Errorf("peek %+v", h)
	}
	if h, _ := s.HitThrottle(ctx, "k1", 1099, window, 3, lock); h.LockedUntil != 1100 {
		t.Errorf("a hit while locked must not extend the lock: %+v", h)
	}
	if h, _ := s.PeekThrottle(ctx, "k1", 1100, window); h.LockedUntil != 0 || h.Count != 0 {
		t.Errorf("after lock and window %+v", h)
	}
	if h, _ := s.HitThrottle(ctx, "k1", 1200, window, 3, lock); h.Count != 1 || h.WindowStart != 1200 || h.LockedUntil != 0 {
		t.Errorf("new window %+v", h)
	}
	if err := s.ClearThrottle(ctx, "k1"); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.PeekThrottle(ctx, "k1", 1200, window); h.Count != 0 {
		t.Errorf("cleared %+v", h)
	}
	s.HitThrottle(ctx, "k3", 100, window, 3, lock)
	if n, err := s.PurgeThrottle(ctx, 100); err != nil || n != 0 {
		t.Errorf("purge early: %d %v", n, err)
	}
	if n, err := s.PurgeThrottle(ctx, 100+window+lock); err != nil || n != 1 {
		t.Errorf("purge: %d %v", n, err)
	}
	s.HitThrottle(ctx, "k4", 100, window, 3, lock)
	if err := s.ResetThrottle(ctx); err != nil {
		t.Fatal(err)
	}
	if h, _ := s.PeekThrottle(ctx, "k4", 100, window); h.Count != 0 {
		t.Errorf("reset %+v", h)
	}
}

func testAccountEvents(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateAccount(ctx, newAccount("user1")); err != nil {
		t.Fatal(err)
	}
	for i, at := range []int64{10, 20, 30} {
		_, err := s.CommitLogin(ctx, store.LoginCommit{UserID: "user1", CredentialsVersion: 1, Now: at, Session: newSession(fmt.Sprint("s", i), "user1"),
			Event: store.AccountEvent{UserID: "user1", Kind: "k", At: at}})
		if err != nil {
			t.Fatal(err)
		}
	}
	evs, err := s.ListAccountEvents(ctx, "user1", 2)
	if err != nil || len(evs) != 2 || evs[0].At != 30 || evs[1].At != 20 {
		t.Fatalf("events %+v err %v", evs, err)
	}
	if n, err := s.PurgeAccountEvents(ctx, 20); err != nil || n != 1 {
		t.Errorf("purge %d %v", n, err)
	}
}

// users.deleted_at 要改到的入口（F-ACC-08 规则 4，BE-410 第 4 条）。签发 token 和演示登录在 admin 包，由 e2e 验。
func testAccountDeletedUsers(t *testing.T, open Opener) {
	s, ctx := open(t), context.Background()
	if err := s.CreateAccount(ctx, newAccount("gone1")); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAccount(ctx, newAccount("here1")); err != nil {
		t.Fatal(err)
	}
	v0, _ := s.TokenVersion(ctx, "gone1")
	if err := s.MarkUserDeleted(ctx, "gone1", 5000); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUserDeleted(ctx, "nobody", 5000); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	if ok, _ := s.UserExists(ctx, "gone1"); ok {
		t.Error("UserExists must be false for a deleted user")
	}
	if d, _ := s.UserDeleted(ctx, "gone1"); !d {
		t.Error("UserDeleted")
	}
	if d, _ := s.UserDeleted(ctx, "nobody"); d {
		t.Error("UserDeleted for an unknown user")
	}
	if m, _ := s.MissingUsers(ctx, []string{"gone1", "here1"}); len(m) != 1 || m[0] != "gone1" {
		t.Errorf("MissingUsers %v", m)
	}
	if r, err := s.Relation(ctx, "here1", "gone1", "c2c:gone1:here1"); err != nil || r.PeerExists {
		t.Errorf("Relation %+v err %v", r, err)
	}
	if v, _ := s.TokenVersion(ctx, "gone1"); v != v0+1 {
		t.Errorf("token version %d, was %d", v, v0)
	}
	if a, err := s.GetAccount(ctx, "gone1"); err != nil || a.PasswordHash != "" || a.DeletedAt != 5000 {
		t.Errorf("account %+v err %v", a, err)
	}
	if taken, _ := s.UsernameTaken(ctx, "gone1"); !taken {
		t.Error("a deleted username cannot be reused")
	}
	if err := s.UpsertUsers(ctx, []store.User{{UserID: "gone1", Nickname: "back"}}); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.GetUser(ctx, "gone1"); u.Nickname == "back" {
		t.Error("import must not touch a deleted user")
	}
}
