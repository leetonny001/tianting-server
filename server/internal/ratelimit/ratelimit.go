// Package ratelimit 按用户做令牌桶限流（PRD F-MSG-07、C-29、IMP-4）。
package ratelimit

import (
	"sync"
	"time"

	"im-mvp/server/internal/clock"
)

// idleTTL：桶已满且超过这么久没用，就从表里删掉，避免长期占用内存。
const idleTTL = time.Minute

// Limiter 是限流接口（ARCH 2.2）。单节点用内存令牌桶；集群时换成 Redis 实现，
// 否则同一用户连到 N 个节点就有 N 倍配额。
type Limiter interface {
	// Allow 尝试扣一个令牌；不够时返回 false，且不扣（被拒绝的请求不计数）。
	Allow(key string) bool
}

var _ Limiter = (*TokenBucket)(nil)

// TokenBucket 是内存实现：每个 key 一个令牌桶，容量 burst，每秒补充 rate 个；新 key 的桶初始为满。
type TokenBucket struct {
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

// NewPer 创建每隔 interval 补充一个令牌的桶，用于每秒不到 1 个的低速率（例如每 3 秒恢复一次失败额度）。
func NewPer(interval time.Duration, burst int, clk clock.Clock) *TokenBucket {
	return &TokenBucket{rate: 1 / interval.Seconds(), burst: float64(burst), clock: clk, buckets: map[string]*bucket{}}
}

func New(rate, burst int, clk clock.Clock) *TokenBucket {
	return &TokenBucket{rate: float64(rate), burst: float64(burst), clock: clk, buckets: map[string]*bucket{}}
}

// Allow 尝试扣一个令牌；桶空时返回 false，且不扣（被拒绝的请求不计数）。
func (l *TokenBucket) Allow(key string) bool {
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

func (l *TokenBucket) prune(now time.Time) {
	for k, b := range l.buckets {
		full := b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst
		if full && now.Sub(b.last) > idleTTL {
			delete(l.buckets, k)
		}
	}
}
