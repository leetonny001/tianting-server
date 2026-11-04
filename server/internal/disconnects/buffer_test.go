package disconnects

import (
	"fmt"
	"runtime"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"
)

func rec(user string, ts int64) Record {
	return Record{UserID: user, TS: ts, Code: 1000, Phase: PhaseSession}
}

// AC-OPS-09-9：u1 写满不挤掉 u2。
func TestPerUserLimitDoesNotEvictOthers(t *testing.T) {
	b := New()
	for i := range 5 {
		b.Add(rec("u2", int64(i)))
	}
	for i := range 1000 {
		b.Add(rec("u1", int64(100+i)))
	}
	u1 := b.Query("u1", 50)
	if len(u1) != 50 || u1[0].TS != 1099 || u1[49].TS != 1050 {
		t.Fatalf("u1 = %d records, first %d last %d", len(u1), u1[0].TS, u1[49].TS)
	}
	if u2 := b.Query("u2", 50); len(u2) != 5 || u2[0].TS != 4 || u2[4].TS != 0 {
		t.Errorf("u2 = %+v", u2)
	}
	if got := b.Query("u1", 3); len(got) != 3 || got[0].TS != 1099 {
		t.Errorf("limit 3 = %+v", got)
	}
}

// 全节点上限：淘汰最旧的一条，同时从用户桶里移除；用户条目不残留。
func TestGlobalLimitEvictsOldest(t *testing.T) {
	b := newBuffer(10)
	b.Add(rec("old", 1))
	for i := range 10 {
		b.Add(rec(fmt.Sprint("u", i), int64(10+i)))
	}
	if got := b.Query("old", 50); len(got) != 0 {
		t.Errorf("old = %+v", got)
	}
	if _, ok := b.users["old"]; ok {
		t.Error("evicted user's entry left behind")
	}
	if b.Len() != 10 {
		t.Errorf("len = %d", b.Len())
	}
	if got := b.Oldest(); got != 10 {
		t.Errorf("oldest = %d", got)
	}
}

// 用户桶淘汰留下的空槽位被全局淘汰时不重复扣用户的记录。
func TestDeadSlotsDoNotCorruptUserLists(t *testing.T) {
	b := newBuffer(MaxPerUser + 3)
	for i := range MaxPerUser + 3 {
		b.Add(rec("u", int64(i)))
	}
	if got := b.Query("u", 50); len(got) != 50 || got[0].TS != int64(MaxPerUser+2) {
		t.Fatalf("got %d, first %d", len(got), got[0].TS)
	}
	for i := range 3 { // 覆盖掉前 3 个已经失效的槽位
		b.Add(rec("x", int64(100+i)))
	}
	if got := b.Query("u", 50); len(got) != 50 {
		t.Errorf("u lost live records: %d", len(got))
	}
	// 再写满一圈：u 的记录按序全部被淘汰，顺序从不错乱。
	for i := range MaxPerUser + 3 {
		b.Add(rec("y", int64(200+i)))
		list := b.Query("y", 50)
		if !slices.IsSortedFunc(list, func(a, c Record) int { return int(c.TS - a.TS) }) {
			t.Fatal("not newest first")
		}
	}
	if got := b.Query("u", 50); len(got) != 0 {
		t.Errorf("u = %d after a full cycle", len(got))
	}
	if b.Len() > MaxPerUser+3 {
		t.Errorf("len = %d", b.Len())
	}
}

func TestEmpty(t *testing.T) {
	b := New()
	if got := b.Query("u", 0); got == nil || len(got) != 0 {
		t.Errorf("query = %v", got)
	}
	if b.Oldest() != 0 {
		t.Error("oldest should be 0")
	}
}

func liveHeap() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// AC-OPS-09-8：10 万条（1 万个用户）和 2 万条的存活堆相差不超过 5%；条数和每用户条数都不超限。
func TestMemoryIsBounded(t *testing.T) {
	fill := func(b *Buffer, n int) {
		for i := range n {
			b.Add(Record{UserID: fmt.Sprintf("user-%05d", i%10_000), ConnID: fmt.Sprintf("node-%d", i), Phase: PhaseSession, Platform: "web", Reason: "normal", By: "client", TS: int64(i), IP: "10.1.*.*"})
		}
	}
	base := liveHeap()
	small := New()
	fill(small, 20_000)
	smallHeap := liveHeap() - base
	runtime.KeepAlive(small)
	small = nil
	runtime.KeepAlive(small)

	base = liveHeap()
	big := New()
	fill(big, 100_000)
	bigHeap := liveHeap() - base
	if big.Len() > MaxTotal {
		t.Errorf("len = %d", big.Len())
	}
	for _, list := range big.users {
		if len(list) > MaxPerUser {
			t.Fatalf("user has %d records", len(list))
		}
	}
	ratio := float64(bigHeap) / float64(smallHeap)
	if ratio > 1.05 || ratio < 0.95 {
		t.Errorf("live heap: 20k=%d 100k=%d ratio=%.3f", smallHeap, bigHeap, ratio)
	}
	runtime.KeepAlive(big)
}

// AC-OPS-09-12：1 万条 / 秒写入 + 每秒 10 次查询，写入 P99 ≤ 1ms，查询和写入一致。
func TestConcurrentWriteLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	b := New()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
				list := b.Query("user-7", 50)
				for i := 1; i < len(list); i++ {
					if list[i].TS >= list[i-1].TS {
						t.Error("query out of order")
						return
					}
				}
			}
		}
	}()
	const n = 10_000
	lat := make([]time.Duration, 0, n)
	start := time.Now()
	for i := range n {
		// 按 1 万条 / 秒的节奏写。
		if d := time.Duration(i) * 100 * time.Microsecond; time.Since(start) < d {
			time.Sleep(d - time.Since(start))
		}
		t0 := time.Now()
		b.Add(rec(fmt.Sprintf("user-%d", i%100), int64(i)))
		lat = append(lat, time.Since(t0))
	}
	close(stop)
	wg.Wait()
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	if p99 := lat[n*99/100]; p99 > time.Millisecond {
		t.Errorf("write P99 = %v", p99)
	}
}
