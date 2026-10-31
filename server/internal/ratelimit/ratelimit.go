// Package ratelimit 按用户做令牌桶限流（PRD F-MSG-07、C-29、IMP-4）。
package ratelimit

import (
	"sync"
	"time"

	"im-mvp/server/internal/clock"
)

// idleTTL：桶已满且超过这么久没用，就从表里删掉，避免长期占用内存。
const idleTTL = time.Minute

// Limiter 每个 key 一个令牌桶：容量 burst，每秒补充 rate 个；新 key 的桶初始为满。
type Limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	clock   clock.Clock
	buckets map[string]*bucket
	calls   int
}

type bucket struct {
	tokens float64
	last   time.Time
}

func New(rate, burst int, clk clock.Clock) *Limiter {
	return &Limiter{rate: float64(rate), burst: float64(burst), clock: clk, buckets: map[string]*bucket{}}
}

// Allow 尝试扣一个令牌；桶空时返回 false，且不扣（被拒绝的请求不计数）。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()

	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	l.calls++
	if l.calls%1024 == 0 {
		l.prune(now)
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *Limiter) prune(now time.Time) {
	for k, b := range l.buckets {
		full := b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst
		if full && now.Sub(b.last) > idleTTL {
			delete(l.buckets, k)
		}
	}
}
