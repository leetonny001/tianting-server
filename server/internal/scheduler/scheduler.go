// Package scheduler 是进程内定时任务（PRD-admin F-ADM-32 规则 2，BE-220）。
//
// 到期判断用注入的时钟：测试把 [IM 钟] 推进之后，任务在下一次检查时运行，也可以用 Run 手动触发。
// 集群下每个任务只在一个节点运行：单节点用 Always，B6 换成 Postgres advisory lock / Redis 锁，任务本身不用改。
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"tianting/server/internal/clock"
)

// maxDrainRuns 是分批任务一次定时触发最多跑多少批，避免一直占着检查循环、挡住其他到期的任务。
const maxDrainRuns = 1000

// CheckInterval 是检查任务是否到期的间隔（真实时间）。任务周期都是小时级，这个粒度足够。
const CheckInterval = time.Minute

// Locker 决定这次运行轮不轮到本节点。
type Locker interface {
	TryRun(ctx context.Context, job string) (run bool, release func(), err error)
}

// Always 是单节点的 Locker：永远轮到本节点。
type Always struct{}

// TryRun 实现 Locker。
func (Always) TryRun(context.Context, string) (bool, func(), error) { return true, func() {}, nil }

// Func 是任务本体，返回处理的行数（只进日志）。
type Func func(ctx context.Context) (int, error)

// ErrUnknownJob 表示 Run 的任务名没有注册。
var ErrUnknownJob = errors.New("unknown job")

// 指标的 result 标签。
const (
	ResultOK      = "ok"
	ResultError   = "error"
	ResultSkipped = "skipped" // 这一轮不在本节点运行
)

type job struct {
	name   string
	period time.Duration
	fn     Func
	next   time.Time
	drain  bool       // 定时触发时跑到没有可处理的行；手动触发只跑一批
	off    bool       // 定时触发时跳过；手动触发照常（测试配置 IM_TEST_DISABLE_JOBS）
	mu     sync.Mutex // 同一个任务不并发运行（定时和手动触发可能撞上）
}

// Scheduler 持有全部任务。Register 只在启动阶段调用。
type Scheduler struct {
	clock    clock.Clock
	locker   Locker
	log      *slog.Logger
	runs     *prometheus.CounterVec
	duration *prometheus.HistogramVec

	mu   sync.Mutex
	jobs map[string]*job
}

// New 创建 Scheduler。locker 为 nil 时用 Always。
func New(clk clock.Clock, locker Locker, runs *prometheus.CounterVec, duration *prometheus.HistogramVec, log *slog.Logger) *Scheduler {
	if locker == nil {
		locker = Always{}
	}
	return &Scheduler{clock: clk, locker: locker, log: log, runs: runs, duration: duration, jobs: map[string]*job{}}
}

// Register 注册任务，第一次在一个周期之后运行。重名是编程错误，panic。
func (s *Scheduler) Register(name string, period time.Duration, fn Func) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.jobs[name]; dup {
		panic("scheduler: duplicate job " + name)
	}
	s.jobs[name] = &job{name: name, period: period, fn: fn, next: s.clock.Now().Add(period)}
}

// RegisterDrain 注册分批任务：fn 每次处理一批并返回行数。定时触发时重复调用直到返回 0（每批之间释放数据库连接），
// 手动触发（Run）只跑一批。
func (s *Scheduler) RegisterDrain(name string, period time.Duration, fn Func) {
	s.Register(name, period, fn)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[name].drain = true
}

// Disable 让任务不再被定时触发，手动触发不受影响。任务名没有注册时返回 ErrUnknownJob。
func (s *Scheduler) Disable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[name]
	if j == nil {
		return ErrUnknownJob
	}
	j.off = true
	return nil
}

// Names 返回全部任务名（测试用）。
func (s *Scheduler) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.jobs))
	for n := range s.jobs {
		out = append(out, n)
	}
	return out
}

// Start 启动检查循环，直到 stop 关闭。
func (s *Scheduler) Start(stop <-chan struct{}) {
	t := time.NewTicker(CheckInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			s.Tick(context.Background())
		}
	}
}

// Tick 运行所有到期的任务。一个任务失败不影响其他任务，也不影响它自己的下一次（下一次时间在运行前算好）。
func (s *Scheduler) Tick(ctx context.Context) {
	now := s.clock.Now()
	s.mu.Lock()
	var due []*job
	for _, j := range s.jobs {
		if !j.off && !now.Before(j.next) {
			j.next = now.Add(j.period)
			due = append(due, j)
		}
	}
	s.mu.Unlock()
	for _, j := range due {
		for i := 0; ; i++ {
			n, err := s.run(ctx, j)
			if !j.drain || err != nil || n == 0 || i+1 >= maxDrainRuns {
				break
			}
		}
	}
}

// Run 立即运行一个任务（测试接口 POST /admin/test/jobs/{name}/run），返回任务的错误。
func (s *Scheduler) Run(ctx context.Context, name string) error {
	s.mu.Lock()
	j := s.jobs[name]
	s.mu.Unlock()
	if j == nil {
		return ErrUnknownJob
	}
	_, err := s.run(ctx, j)
	return err
}

func (s *Scheduler) run(ctx context.Context, j *job) (n int, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	go1, release, err := s.locker.TryRun(ctx, j.name)
	if err != nil {
		s.count(j.name, ResultError)
		s.log.Error("job lock", "job", j.name, "err", err)
		return 0, err
	}
	if !go1 {
		s.count(j.name, ResultSkipped)
		return 0, nil
	}
	defer release()
	started := time.Now()
	defer func() {
		if p := recover(); p != nil { // 一个任务崩了不能带走整个进程和其他任务
			s.log.Error("job panicked", "job", j.name, "panic", p)
			err = errors.New("job panicked")
			s.count(j.name, ResultError)
		}
	}()
	n, err = j.fn(ctx)
	elapsed := time.Since(started)
	if s.duration != nil {
		s.duration.WithLabelValues(j.name).Observe(elapsed.Seconds())
	}
	if err != nil {
		s.count(j.name, ResultError)
		s.log.Error("job failed", "job", j.name, "err", err, "duration", elapsed.Round(time.Millisecond))
		return 0, err
	}
	s.count(j.name, ResultOK)
	s.log.Info("job done", "job", j.name, "rows", n, "duration", elapsed.Round(time.Millisecond))
	return n, nil
}

func (s *Scheduler) count(name, result string) {
	if s.runs != nil {
		s.runs.WithLabelValues(name, result).Inc()
	}
}
