// Package ratelimit 按用户做令牌桶限流（PRD F-MSG-07、C-29、IMP-4）。
package ratelimit

import (
	"strings"
	"sync"
	"time"

	"tianting/server/internal/clock"
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
	l.refill(b, now)

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

// NewDaily 是"每天 n 次"的近似：每 24h/n 补 1 个、容量 n（PRD-social 5.2 节规则 2，不按自然日清零）。
func NewDaily(n int, clk clock.Clock) *TokenBucket {
	return NewPer(24*time.Hour/time.Duration(max(n, 1)), max(n, 1), clk)
}

// NewHourly 是"每小时 n 次"的近似。
func NewHourly(n int, clk clock.Clock) *TokenBucket {
	return NewPer(time.Hour/time.Duration(max(n, 1)), max(n, 1), clk)
}

// refill 把桶补到 now（调用方持锁）。时钟回拨时不补令牌，但 last 跟到 now，否则要等真实时间追上才会再补充。
func (l *TokenBucket) refill(b *bucket, now time.Time) {
	if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(l.burst, b.tokens+elapsed*l.rate)
	}
	b.last = now
}

// Reconfigure 修改速率与容量，对已有的桶立即生效（PRD-social 5.2 节规则 3）：先按旧速率补到当前时刻，
// 再把剩余令牌截断到新容量；不清零（已用量保留），调大时也不自动补满，按新速率补充。
func (l *TokenBucket) Reconfigure(interval time.Duration, burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	for _, b := range l.buckets {
		l.refill(b, now)
		b.tokens = min(b.tokens, float64(burst))
	}
	l.rate, l.burst = 1/interval.Seconds(), float64(burst)
}

// Refund 退还一个令牌（预扣 + 退还，PRD-social F-SOC-08 规则 5a），不超过容量。
func (l *TokenBucket) Refund(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		return // 桶已被回收说明本来就是满的
	}
	l.refill(b, l.clock.Now())
	b.tokens = min(l.burst, b.tokens+1)
}

// Tokens 返回 key 当前的剩余令牌数（测试用）。
func (l *TokenBucket) Tokens(key string) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		return l.burst
	}
	l.refill(b, l.clock.Now())
	return b.tokens
}

// ResetUser 清空某用户的桶：key 等于 uid 或以 "uid:" 开头（测试接口 /admin/test/limits/reset）。
func (l *TokenBucket) ResetUser(uid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.buckets {
		if k == uid || strings.HasPrefix(k, uid+":") {
			delete(l.buckets, k)
		}
	}
}
