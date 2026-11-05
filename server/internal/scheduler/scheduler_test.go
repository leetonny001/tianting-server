package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"tianting/server/internal/clock"
)

func newTest(t *testing.T, locker Locker) (*Scheduler, *clock.Settable, *prometheus.CounterVec) {
	t.Helper()
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	runs := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runs"}, []string{"job", "result"})
	dur := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "dur"}, []string{"job"})
	return New(clk, locker, runs, dur, slog.New(slog.NewTextHandler(io.Discard, nil))), clk, runs
}

// counter 读一个计数器的值（不引入 testutil，它会多带依赖）。
func counter(t *testing.T, runs *prometheus.CounterVec, job, result string) float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(runs)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["job"] == job && labels["result"] == result {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func TestRunsWhenClockReachesPeriod(t *testing.T) {
	s, clk, runs := newTest(t, nil)
	n := 0
	s.Register("a", time.Hour, func(context.Context) (int, error) { n++; return 0, nil })
	s.Tick(context.Background())
	if n != 0 {
		t.Fatal("ran before the period elapsed")
	}
	clk.Freeze(clk.Now().Add(59 * time.Minute))
	s.Tick(context.Background())
	if n != 0 {
		t.Fatal("ran one minute early")
	}
	clk.Freeze(clk.Now().Add(time.Minute))
	s.Tick(context.Background())
	s.Tick(context.Background()) // 同一刻再检查不重复运行
	if n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
	if got := counter(t, runs, "a", ResultOK); got != 1 {
		t.Fatalf("metric = %v", got)
	}
}

func TestFailureDoesNotAffectNextRunOrOtherJobs(t *testing.T) {
	s, clk, runs := newTest(t, nil)
	calls, other := 0, 0
	s.Register("bad", time.Hour, func(context.Context) (int, error) {
		calls++
		if calls == 1 {
			return 0, errors.New("boom")
		}
		if calls == 2 {
			panic("boom")
		}
		return 0, nil
	})
	s.Register("good", time.Hour, func(context.Context) (int, error) { other++; return 0, nil })
	for range 3 {
		clk.Freeze(clk.Now().Add(time.Hour))
		s.Tick(context.Background())
	}
	if calls != 3 || other != 3 {
		t.Fatalf("calls=%d other=%d", calls, other)
	}
	if got := counter(t, runs, "bad", ResultError); got != 2 {
		t.Fatalf("error metric = %v", got)
	}
	if got := counter(t, runs, "bad", ResultOK); got != 1 {
		t.Fatalf("ok metric = %v", got)
	}
}

type never struct{}

func (never) TryRun(context.Context, string) (bool, func(), error) { return false, nil, nil }

func TestLockerDecidesWhoRuns(t *testing.T) {
	s, _, runs := newTest(t, never{})
	s.Register("a", time.Hour, func(context.Context) (int, error) { t.Error("must not run"); return 0, nil })
	if err := s.Run(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if got := counter(t, runs, "a", ResultSkipped); got != 1 {
		t.Fatalf("skipped metric = %v", got)
	}
}

func TestRunUnknownAndReturnsError(t *testing.T) {
	s, _, _ := newTest(t, nil)
	if err := s.Run(context.Background(), "nope"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("err = %v", err)
	}
	boom := errors.New("boom")
	s.Register("a", time.Hour, func(context.Context) (int, error) { return 0, boom })
	if err := s.Run(context.Background(), "a"); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

// PRD-live F-PUSH-09 规则 4、AC-PUSH-06-14：分批任务定时触发时跑到清完，手动触发只跑一批。
func TestDrainJob(t *testing.T) {
	s, clk, runs := newTest(t, nil)
	left := 5
	calls := 0
	s.RegisterDrain("purge", time.Hour, func(context.Context) (int, error) {
		calls++
		n := min(left, 2)
		left -= n
		return n, nil
	})
	if err := s.Run(context.Background(), "purge"); err != nil || calls != 1 || left != 3 {
		t.Fatalf("manual run: calls=%d left=%d err=%v", calls, left, err)
	}
	clk.Freeze(clk.Now().Add(time.Hour))
	s.Tick(context.Background())
	if left != 0 || calls != 4 { // 2、1、0 三批，最后一批返回 0 才停
		t.Fatalf("tick: calls=%d left=%d", calls, left)
	}
	if got := counter(t, runs, "purge", ResultOK); got != 4 {
		t.Fatalf("ok runs = %v", got)
	}
}

// 分批任务出错就停，不原地重试；上限挡住一直有数据的情况。
func TestDrainJobStops(t *testing.T) {
	s, clk, _ := newTest(t, nil)
	calls := 0
	s.RegisterDrain("bad", time.Hour, func(context.Context) (int, error) { calls++; return 1, errors.New("boom") })
	busy := 0
	s.RegisterDrain("busy", time.Hour, func(context.Context) (int, error) { busy++; return 1, nil })
	clk.Freeze(clk.Now().Add(time.Hour))
	s.Tick(context.Background())
	if calls != 1 {
		t.Fatalf("calls after an error = %d", calls)
	}
	if busy != maxDrainRuns {
		t.Fatalf("busy runs = %d, want %d", busy, maxDrainRuns)
	}
}

// PRD-live 8.2 节：Disable 只挡定时触发，手动触发照常；未注册的名字报错。
func TestDisableJob(t *testing.T) {
	s, clk, _ := newTest(t, nil)
	n := 0
	s.Register("a", time.Hour, func(context.Context) (int, error) { n++; return 0, nil })
	if err := s.Disable("a"); err != nil {
		t.Fatal(err)
	}
	clk.Freeze(clk.Now().Add(2 * time.Hour))
	s.Tick(context.Background())
	if n != 0 {
		t.Fatal("disabled job ran on a tick")
	}
	if err := s.Run(context.Background(), "a"); err != nil || n != 1 {
		t.Fatalf("manual run: n=%d err=%v", n, err)
	}
	if err := s.Disable("ghost"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("ghost: %v", err)
	}
}
