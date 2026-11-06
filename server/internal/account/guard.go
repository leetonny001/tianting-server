package account

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"sync"
	"time"

	"tianting/server/internal/clientip"
	"tianting/server/internal/clock"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 失败限制的默认值（F-ACC-02 规则 3），未校准。
const (
	loginFailWindow   = 15 * time.Minute
	loginFailLock     = 15 * time.Minute
	defaultFailLimit  = 5
	loginIPLimit      = 20 // 每 IP 每分钟
	loginIPWindow     = time.Minute
	progressiveBase   = 100 * time.Millisecond // 按用户名的渐进延迟：第 n 次失败之后 base × 2^(n-1)
	progressiveMax    = 3 * time.Second
	inflightRetrySecs = 1
)

// ThrottleStore 是 Guard 用到的存储。
type ThrottleStore interface {
	HitThrottle(ctx context.Context, key string, now, windowMs int64, limit int, lockMs int64) (store.ThrottleHit, error)
	PeekThrottle(ctx context.Context, key string, now, windowMs int64) (store.ThrottleHit, error)
	ClearThrottle(ctx context.Context, key string) error
}

// Guard 是所有校验密码的接口共用的失败限制（F-ACC-02 规则 3，RS4-11、RS4-12）：登录、password、logout-others、recovery-code、delete
// 用同一个计数。计数存数据库（auth_throttle），重启不清零，多节点一致。键里只有用户名的哈希和 IP 前缀（规则 3a）。
type Guard struct {
	st       ThrottleStore
	clk      clock.Clock
	disabled bool // IM_TEST_DISABLE_AUTH_LOCK
	limit    int
	// 可信出口网段的 IP 总量额度放大（第 12.2 节，RS4-15）。不放大"用户名 + IP"的失败次数。
	egress       []netip.Prefix
	egressFactor int

	mu       sync.Mutex
	inflight map[string]bool // 用户名哈希：同一用户名同时只算 1 个哈希（单节点内）
}

// NewGuard 创建失败限制。
func NewGuard(st ThrottleStore, clk clock.Clock, disabled bool, failLimit int) *Guard {
	if failLimit <= 0 {
		failLimit = defaultFailLimit
	}
	return &Guard{st: st, clk: clk, disabled: disabled, limit: failLimit, inflight: map[string]bool{}}
}

// UsernameHash 是用户名的哈希。日志和事件只记它，用户可能把密码误输进用户名框（规则 3a）。
func UsernameHash(normalized string) string {
	sum := sha256.Sum256([]byte("username\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

// Attempt 是一次校验密码的尝试。
type Attempt struct {
	g            *Guard
	UsernameHash string
	pairKey      string
	userKey      string
	holding      bool
}

func unavailable() *protocol.Error {
	return protocol.Errorf(protocol.CodeServiceUnavailable, "service unavailable")
}

func lockedErr(retryAfterSeconds int64) *protocol.Error {
	e := protocol.Errorf(protocol.CodeAccountLocked, "too many attempts")
	e.Details = map[string]any{"retryAfterSeconds": retryAfterSeconds}
	return e
}

func rateLimited(retryAfterSeconds int64) *protocol.Error {
	e := protocol.Errorf(protocol.CodeRateLimited, "too many requests")
	e.Details = map[string]any{"retryAfterSeconds": retryAfterSeconds}
	return e
}

func ceilSeconds(ms int64) int64 { return max((ms+999)/1000, 1) }

// LimitIP 是按 IP 的窗口计数，用完 429 / 4290。先于校验和闸门调用。
func (g *Guard) LimitIP(ctx context.Context, scope, ip string, limit int, window time.Duration) *protocol.Error {
	if g.disabled {
		return nil
	}
	limit = g.ipLimit(ip, limit)
	now, windowMs := g.clk.Now().UnixMilli(), window.Milliseconds()
	h, err := g.st.HitThrottle(ctx, "ip:"+scope+":"+clientip.LimitKey(ip), now, windowMs, limit, 0)
	if err != nil {
		return unavailable()
	}
	if h.Count > limit {
		return rateLimited(ceilSeconds(h.WindowStart + windowMs - now))
	}
	return nil
}

// ipLimit 是这个 IP 的总量额度：可信出口网段放大。
func (g *Guard) ipLimit(ip string, limit int) int {
	addr, err := netip.ParseAddr(ip)
	if err != nil || len(g.egress) == 0 {
		return limit
	}
	addr = addr.Unmap()
	for _, p := range g.egress {
		if p.Contains(addr) {
			return limit * max(g.egressFactor, 1)
		}
	}
	return limit
}

// TrustEgress 配置可信出口网段。
func (g *Guard) TrustEgress(prefixes []netip.Prefix, factor int) {
	g.egress, g.egressFactor = prefixes, factor
}

// Begin 是校验密码之前的公共流程。顺序：IP 限流 → 用户名 + IP 锁定 → 渐进延迟（在闸门之外）→ 同一用户名只算 1 个哈希。
// 不存在的用户名走同样的流程（R-CS-20）。返回错误时已经不用调用 Done。
func (g *Guard) Begin(ctx context.Context, usernameLower, ip string) (*Attempt, *protocol.Error) {
	hash := UsernameHash(usernameLower)
	a := &Attempt{g: g, UsernameHash: hash, pairKey: pairThrottleKey(usernameLower, ip), userKey: "pwuser:" + hash}
	if g.disabled {
		return a, nil
	}
	if e := g.LimitIP(ctx, "pw", ip, loginIPLimit, loginIPWindow); e != nil {
		return nil, e
	}
	now := g.clk.Now().UnixMilli()
	pair, err := g.st.PeekThrottle(ctx, a.pairKey, now, loginFailWindow.Milliseconds())
	if err != nil {
		return nil, unavailable()
	}
	if pair.LockedUntil > now {
		return nil, lockedErr(ceilSeconds(pair.LockedUntil - now))
	}
	user, err := g.st.PeekThrottle(ctx, a.userKey, now, loginFailWindow.Milliseconds())
	if err != nil {
		return nil, unavailable()
	}
	// 用真实时间等，不受测试时钟影响。
	if d := progressiveDelay(user.Count); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return nil, unavailable()
		}
	}
	// 只在算哈希时占着这个用户名（RS4-12）：如果把上面的延迟也算进去，一个 IP 用满每分钟 20 次就能一直占住别人的用户名，
	// 等于规则 3 不允许的"任何人都能把别人锁死"。
	g.mu.Lock()
	busy := g.inflight[hash]
	if !busy {
		g.inflight[hash] = true
	}
	g.mu.Unlock()
	if busy {
		return nil, lockedErr(inflightRetrySecs)
	}
	a.holding = true
	return a, nil
}

// progressiveDelay：累计失败越多，响应越慢，最多 3 秒（R-CS-06）。
func progressiveDelay(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}
	if failures > 8 {
		return progressiveMax
	}
	return min(progressiveBase<<(failures-1), progressiveMax)
}

// Fail 记一次密码错误：用户名 + IP 的计数（到上限锁定）和用户名的计数（渐进延迟）。
// F-ACC-04 规则 8 的版本不匹配不算失败，不要调用它。
func (a *Attempt) Fail(ctx context.Context) error {
	if a.g.disabled {
		return nil
	}
	now, windowMs := a.g.clk.Now().UnixMilli(), loginFailWindow.Milliseconds()
	if _, err := a.g.st.HitThrottle(ctx, a.pairKey, now, windowMs, a.g.limit, loginFailLock.Milliseconds()); err != nil {
		return err
	}
	_, err := a.g.st.HitThrottle(ctx, a.userKey, now, windowMs, 0, 0)
	return err
}

// pairThrottleKey 是"用户名 + IP"的计数键。
func pairThrottleKey(usernameLower, ip string) string {
	return "pw:" + UsernameHash(usernameLower) + ":" + clientip.LimitKey(ip)
}

// RecoverMiss 处理恢复码未命中（F-ACC-05 规则 2）：已锁定 429 / 4294；否则计入和密码共用的失败次数。不算渐进延迟，也不占单哈希：
// 未命中不算 Argon2，要快速返回。返回 nil 时调用方回 401 / 4018。
func (g *Guard) RecoverMiss(ctx context.Context, usernameLower, ip string) *protocol.Error {
	if g.disabled {
		return nil
	}
	now := g.clk.Now().UnixMilli()
	key := pairThrottleKey(usernameLower, ip)
	pair, err := g.st.PeekThrottle(ctx, key, now, loginFailWindow.Milliseconds())
	if err != nil {
		return unavailable()
	}
	if pair.LockedUntil > now {
		return lockedErr(ceilSeconds(pair.LockedUntil - now))
	}
	if _, err := g.st.HitThrottle(ctx, key, now, loginFailWindow.Milliseconds(), g.limit, loginFailLock.Milliseconds()); err != nil {
		return unavailable()
	}
	if _, err := g.st.HitThrottle(ctx, "pwuser:"+UsernameHash(usernameLower), now, loginFailWindow.Milliseconds(), 0, 0); err != nil {
		return unavailable()
	}
	return nil
}

// PairKey 是用户名 + IP 的计数键：登录成功时在同一事务里清零。
func (a *Attempt) PairKey() string {
	if a.g.disabled {
		return ""
	}
	return a.pairKey
}

// Done 结束这次尝试，放开这个用户名。
func (a *Attempt) Done() {
	if !a.holding {
		return
	}
	a.holding = false
	a.g.mu.Lock()
	delete(a.g.inflight, a.UsernameHash)
	a.g.mu.Unlock()
}
