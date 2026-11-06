package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tianting/server/internal/clock"
)

func TestBucket(t *testing.T) {
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0)
	l := New(20, 20, clk)

	ok := 0
	for range 25 {
		if l.Allow("u1") {
			ok++
		}
	}
	if ok != 20 {
		t.Fatalf("burst allowed %d, want 20 (bucket starts full)", ok)
	}
	if !l.Allow("u2") {
		t.Fatal("other users have their own bucket")
	}

	clk.Freeze(t0.Add(500 * time.Millisecond)) // 补充 10 个；之前被拒绝的 5 次没有扣令牌
	ok = 0
	for range 15 {
		if l.Allow("u1") {
			ok++
		}
	}
	if ok != 10 {
		t.Fatalf("after 0.5s allowed %d, want 10", ok)
	}

	clk.Freeze(t0.Add(10 * time.Second)) // 补满，但不超过容量
	ok = 0
	for range 30 {
		if l.Allow("u1") {
			ok++
		}
	}
	if ok != 20 {
		t.Fatalf("after long idle allowed %d, want 20 (capped at burst)", ok)
	}
}

func TestPruneIdleBuckets(t *testing.T) {
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0)
	l := New(20, 20, clk)
	l.Allow("idle")
	clk.Freeze(t0.Add(2 * time.Minute))
	l.mu.Lock()
	l.prune(clk.Now())
	_, still := l.buckets["idle"]
	l.mu.Unlock()
	if still {
		t.Error("full idle bucket should be pruned")
	}
}

func TestNewPerSlowRefill(t *testing.T) {
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0)
	l := NewPer(3*time.Second, 2, clk)
	if !l.Allow("ip") || !l.Allow("ip") || l.Allow("ip") {
		t.Fatal("burst of 2 expected")
	}
	clk.Freeze(t0.Add(2 * time.Second))
	if l.Allow("ip") {
		t.Fatal("refilled too early")
	}
	clk.Freeze(t0.Add(3 * time.Second))
	if !l.Allow("ip") {
		t.Fatal("one token expected after 3s")
	}
}

func TestReconfigureTruncatesWithoutRefill(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	l := NewPer(time.Second, 10, clk)
	for range 3 {
		l.Allow("u") // 剩 7
	}
	l.Reconfigure(time.Second, 5) // 截断到 5
	if got := l.Tokens("u"); got != 5 {
		t.Fatalf("after shrink: %v, want 5", got)
	}
	l.Reconfigure(time.Second, 20) // 调大不补满
	if got := l.Tokens("u"); got != 5 {
		t.Fatalf("after grow: %v, want 5", got)
	}
	clk.Freeze(clk.Now().Add(3 * time.Second))
	if got := l.Tokens("u"); got != 8 {
		t.Fatalf("after 3s at new rate: %v, want 8", got)
	}
}

func TestRefundAndReset(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	l := NewDaily(2, clk)
	if !l.Allow("u") || !l.Allow("u") || l.Allow("u") {
		t.Fatal("daily bucket of 2 should allow exactly 2")
	}
	l.Refund("u")
	if !l.Allow("u") || l.Allow("u") {
		t.Fatal("refund should give back exactly one token")
	}
	l.Refund("u")
	l.Refund("u")
	l.Refund("u")
	if got := l.Tokens("u"); got != 2 {
		t.Fatalf("refund must not exceed burst: %v", got)
	}
	l.Allow("u:bob")
	l.Allow("ux")
	l.ResetUser("u")
	if l.Tokens("u:bob") != 2 || l.Tokens("ux") != 1 {
		t.Fatalf("ResetUser cleared wrong keys: u:bob=%v ux=%v", l.Tokens("u:bob"), l.Tokens("ux"))
	}
}

func TestTakeIsAtomicUnderConcurrency(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	l := NewDaily(100, clk)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 500 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("u") {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 100 {
		t.Fatalf("granted %d, want 100", ok.Load())
	}
}

// 时钟回拨（NTP、/admin/test/clock）之后桶不能卡到真实时间追上：回拨不补令牌，之后从新时刻起正常补充。
func TestClockRollbackDoesNotStickBucket(t *testing.T) {
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0.Add(6 * 24 * time.Hour))
	l := New(1, 2, clk)
	if !l.Allow("u") || !l.Allow("u") || l.Allow("u") {
		t.Fatal("burst of 2, third request must be refused")
	}

	clk.Freeze(t0)
	if l.Allow("u") {
		t.Fatal("rollback itself must not add tokens")
	}

	clk.Freeze(t0.Add(time.Second))
	if !l.Allow("u") {
		t.Fatal("one second after the rollback the bucket must have refilled one token")
	}
	if l.Allow("u") {
		t.Fatal("only one token was refilled")
	}
}
