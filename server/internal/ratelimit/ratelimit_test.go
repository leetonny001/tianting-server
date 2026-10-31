package ratelimit

import (
	"testing"
	"time"

	"im-mvp/server/internal/clock"
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
