package account_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/clock"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

func newGuard(t *testing.T, disabled bool) (*account.Guard, *clock.Settable) {
	t.Helper()
	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "g.db"), store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_800_000_000_000))
	return account.NewGuard(st, clk, disabled, 0), clk
}

func fail(t *testing.T, g *account.Guard, name, ip string) {
	t.Helper()
	a, e := g.Begin(context.Background(), name, ip)
	if e != nil {
		t.Fatalf("begin: %v", e)
	}
	if err := a.Fail(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.Done()
}

// TC-ACC-04-21（AC-ACC-04-22，计数部分）：所有校验密码的接口都走同一个 Guard，计数按"用户名 + IP"，和接口无关。
// 接口部分（logout-others、recovery-code、delete）随 BE-417、BE-418、BE-420。
func TestTC_ACC_04_21_SharedCounter(t *testing.T) {
	g, clk := newGuard(t, false)
	for range 5 {
		fail(t, g, "alice_1", "203.0.113.1")
	}
	_, e := g.Begin(context.Background(), "alice_1", "203.0.113.1")
	if e == nil || e.Code != protocol.CodeAccountLocked || e.Details["retryAfterSeconds"] != int64(900) {
		t.Fatalf("e = %+v", e)
	}
	// 换 IP 不受影响；IPv6 /64 内合并。
	if a, e := g.Begin(context.Background(), "alice_1", "203.0.113.2"); e != nil {
		t.Errorf("other ip: %v", e)
	} else {
		a.Done()
	}
	clk.Freeze(clk.Now().Add(14*time.Minute + 59*time.Second))
	if _, e := g.Begin(context.Background(), "alice_1", "203.0.113.1"); e == nil {
		t.Error("still locked")
	}
	clk.Freeze(clk.Now().Add(2 * time.Second))
	if a, e := g.Begin(context.Background(), "alice_1", "203.0.113.1"); e != nil {
		t.Errorf("after the lock: %v", e)
	} else {
		a.Done()
	}
}

// 同一用户名同时只算 1 个哈希，其余直接按失败限制返回（RS4-12）；放开之后可以继续。
func TestSingleHashPerUsername(t *testing.T) {
	g, _ := newGuard(t, false)
	a, e := g.Begin(context.Background(), "alice_1", "203.0.113.1")
	if e != nil {
		t.Fatal(e)
	}
	_, e = g.Begin(context.Background(), "alice_1", "203.0.113.2")
	if e == nil || e.Code != protocol.CodeAccountLocked {
		t.Fatalf("e = %+v", e)
	}
	if b, e := g.Begin(context.Background(), "bob_1", "203.0.113.2"); e != nil {
		t.Errorf("another username: %v", e)
	} else {
		b.Done()
	}
	a.Done()
	if b, e := g.Begin(context.Background(), "alice_1", "203.0.113.2"); e != nil {
		t.Errorf("after done: %v", e)
	} else {
		b.Done()
	}
}

// 渐进延迟：按用户名累计，换 IP 也算；不存在的用户名同样（R-CS-20）；关闭锁定时也关闭（TC-ACC-02-03 说明）。
func TestProgressiveDelay(t *testing.T) {
	g, _ := newGuard(t, false)
	for i := range 3 {
		fail(t, g, "nouser_1", "203.0.113."+string(rune('1'+i)))
	}
	start := time.Now()
	a, e := g.Begin(context.Background(), "nouser_1", "203.0.113.9")
	if e != nil {
		t.Fatal(e)
	}
	a.Done()
	if d := time.Since(start); d < 400*time.Millisecond || d > 900*time.Millisecond {
		t.Errorf("delay after 3 failures = %v, want about 400ms", d)
	}

	off, _ := newGuard(t, true)
	for range 10 {
		fail(t, off, "nouser_1", "203.0.113.1")
	}
	start = time.Now()
	a, e = off.Begin(context.Background(), "nouser_1", "203.0.113.1")
	if e != nil {
		t.Fatalf("disabled lock: %v", e)
	}
	a.Done()
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("no delay when the lock is off: %v", d)
	}
}

// 规则 3a：键和哈希里没有原始用户名。
func TestUsernameHashHidesTheName(t *testing.T) {
	h := account.UsernameHash("zxq-user-1")
	if strings.Contains(h, "zxq") || len(h) != 64 {
		t.Errorf("hash %q", h)
	}
	if account.UsernameHash("a") == account.UsernameHash("b") {
		t.Error("hash collision")
	}
}
