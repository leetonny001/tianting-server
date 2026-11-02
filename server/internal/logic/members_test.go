package logic

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"im-mvp/server/internal/cache"
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/store"
)

// fakeStore 只实现成员缓存用到的两个方法；其余方法调用到会 panic（嵌入的 nil 接口）。
type fakeStore struct {
	store.Store
	members atomic.Value // []string
	loads   atomic.Int32
	during  func() // 每次加载成员时调用，用来模拟"加载期间发生了变更"
}

func (f *fakeStore) GroupExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakeStore) GroupMembers(context.Context, string) ([]string, error) {
	f.loads.Add(1)
	m := f.members.Load().([]string)
	if f.during != nil {
		f.during()
	}
	return m, nil
}

func TestMemberCacheHitsAndInvalidates(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{}
	st.members.Store([]string{"a"})
	inv := cache.NewLocal()
	clk := clock.New()
	c := newMemberCache(st, clk, inv)

	for range 3 {
		e, _ := c.get(ctx, "g1")
		if !e.has("a") {
			t.Fatal("a should be a member")
		}
	}
	if n := st.loads.Load(); n != 1 {
		t.Fatalf("loads = %d, want 1 (cache hits)", n)
	}

	st.members.Store([]string{"a", "b"})
	inv.Invalidate(TopicGroupMembers, "g1")
	if e, _ := c.get(ctx, "g1"); !e.has("b") {
		t.Fatal("invalidation did not reload")
	}
	inv.Invalidate(TopicGroupMembers, "other") // 其他群的失效不影响 g1
	c.get(ctx, "g1")
	if n := st.loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want 2", n)
	}
}

// 加载期间收到失效通知：本次读到的可能是旧数据，不能放进缓存，否则会一直用旧成员列表。
func TestMemberCacheDiscardsLoadRacingWithInvalidation(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{}
	st.members.Store([]string{"a", "removed"})
	inv := cache.NewLocal()
	c := newMemberCache(st, clock.New(), inv)

	st.during = func() {
		// 加载已经读到了旧列表，此时管理接口移除了成员并发出失效通知。
		st.members.Store([]string{"a"})
		inv.Invalidate(TopicGroupMembers, "g1")
		st.during = nil
	}
	if e, _ := c.get(ctx, "g1"); !e.has("removed") {
		t.Fatal("precondition: the racing load returns the old list")
	}
	if e, _ := c.get(ctx, "g1"); e.has("removed") {
		t.Fatal("stale member list was cached after a concurrent invalidation")
	}
}

// 兜底 TTL：即使漏掉了失效通知，过期后也会重新加载。
func TestMemberCacheTTL(t *testing.T) {
	ctx := context.Background()
	st := &fakeStore{}
	st.members.Store([]string{"a"})
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0)
	c := newMemberCache(st, clk, cache.NewLocal())

	c.get(ctx, "g1")
	st.members.Store([]string{"a", "b"}) // 变更了但没有发失效通知
	clk.Freeze(t0.Add(memberCacheTTL - time.Second))
	if e, _ := c.get(ctx, "g1"); e.has("b") {
		t.Fatal("reloaded before TTL")
	}
	clk.Freeze(t0.Add(memberCacheTTL))
	if e, _ := c.get(ctx, "g1"); !e.has("b") {
		t.Fatal("not reloaded after TTL")
	}
}
