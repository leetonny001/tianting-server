package account

import (
	"context"
	"errors"
	"testing"
	"time"
)

// RS6-02：同一个码同时只算 1 个；按账号轮转；同账号里代号更老的优先。TC-ACC-04-28 的调度部分。
func TestRecoverSchedFairness(t *testing.T) {
	q := newRecoverSched(1)
	ctx := context.Background()
	free, err := q.acquire(ctx, "x", 9, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	done := make(chan string, 6)
	wait := func(uid string, gen int64, name string) {
		go func() {
			f, err := q.acquire(ctx, uid, gen, 5*time.Second)
			if err != nil {
				t.Error(err)
				done <- name
				return
			}
			done <- name
			f()
		}()
		// 等它进队，顺序确定。
		for {
			q.mu.Lock()
			n := 0
			for _, l := range q.waiting {
				n += len(l)
			}
			q.mu.Unlock()
			if n >= queued+1 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		queued++
	}
	queued = 0
	wait("x", 9, "x9-a") // 攻击者的新码排在前面
	wait("x", 9, "x9-b")
	wait("x", 1, "x1") // 受害者的旧码后到
	wait("y", 3, "y3") // 其他账号
	free()
	for range 4 {
		order = append(order, <-done)
	}
	// x 先轮到（先进队），同账号老码优先：x1；然后轮到 y；最后才是 x 的新码。
	want := []string{"x1", "y3", "x9-a", "x9-b"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order %v, want %v", order, want)
		}
	}
}

var queued int

// 同一个码不会同时算两个，即使有空位。
func TestRecoverSchedSameCodeSerial(t *testing.T) {
	q := newRecoverSched(2)
	free, err := q.acquire(context.Background(), "u", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.acquire(context.Background(), "u", 1, 30*time.Millisecond); !errors.Is(err, ErrGateBusy) {
		t.Fatalf("second holder of the same code: %v", err)
	}
	f2, err := q.acquire(context.Background(), "u", 2, time.Second)
	if err != nil {
		t.Fatalf("another code: %v", err)
	}
	f2()
	free()
	f3, err := q.acquire(context.Background(), "u", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	f3()
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != 0 || len(q.active) != 0 || len(q.waiting) != 0 || len(q.order) != 0 {
		t.Fatalf("leak: %+v", q)
	}
}

// 超时的请求退出队列，不占位。
func TestRecoverSchedTimeout(t *testing.T) {
	q := newRecoverSched(1)
	free, _ := q.acquire(context.Background(), "a", 1, time.Second)
	if _, err := q.acquire(context.Background(), "b", 1, 20*time.Millisecond); !errors.Is(err, ErrGateBusy) {
		t.Fatal(err)
	}
	free()
	f, err := q.acquire(context.Background(), "c", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	f()
}
