package gateway

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/ratelimit"
)

// GuardConfig 是按 IP 的握手防护参数（ARCH S7）。
//
// 默认值偏宽松：运营商 NAT、公司出口会让很多用户共用一个 IP，限得太紧会误伤正常用户。
type GuardConfig struct {
	HandshakeRate  int           // 每个 IP 每秒可新建的连接数
	HandshakeBurst int           // 突发上限
	AuthFailBurst  int           // 每个 IP 允许连续鉴权失败的次数，超过即封禁
	AuthFailPer    time.Duration // 每隔多久恢复一次失败额度
	BlockFor       time.Duration // 封禁时长
	// TrustedProxies 是可信反向代理的网段。只有直连地址在其中时才读取 X-Forwarded-For，
	// 否则任何人都能伪造这个头来绕过按 IP 的限制。
	TrustedProxies []netip.Prefix
}

// DefaultGuardConfig 是生产默认值。
func DefaultGuardConfig() GuardConfig {
	return GuardConfig{HandshakeRate: 20, HandshakeBurst: 50, AuthFailBurst: 20, AuthFailPer: 3 * time.Second, BlockFor: 5 * time.Minute}
}

// guard 记录每个 IP 的握手速率和鉴权失败次数。
type guard struct {
	cfg        GuardConfig
	clock      clock.Clock
	handshakes *ratelimit.TokenBucket
	fails      *ratelimit.TokenBucket

	mu      sync.Mutex
	blocked map[string]time.Time // IP → 封禁到期时间
}

func newGuard(cfg GuardConfig, clk clock.Clock) *guard {
	g := &guard{cfg: cfg, clock: clk, blocked: map[string]time.Time{}}
	if cfg.HandshakeRate > 0 {
		g.handshakes = ratelimit.New(cfg.HandshakeRate, cfg.HandshakeBurst, clk)
	}
	if cfg.AuthFailBurst > 0 && cfg.AuthFailPer > 0 {
		// 每 AuthFailPer 恢复一次：换算成每秒速率时可能小于 1，用 NewPer 表达。
		g.fails = ratelimit.NewPer(cfg.AuthFailPer, cfg.AuthFailBurst, clk)
	}
	return g
}

// allowHandshake 判断这个 IP 现在能否新建连接：没有被封禁，且未超过握手速率。
func (g *guard) allowHandshake(ip string) bool {
	now := g.clock.Now()
	g.mu.Lock()
	until, ok := g.blocked[ip]
	if ok && !now.Before(until) {
		delete(g.blocked, ip)
		ok = false
	}
	g.mu.Unlock()
	if ok {
		return false
	}
	return g.handshakes == nil || g.handshakes.Allow(ip)
}

// authFailed 记录一次鉴权失败；额度用完时封禁该 IP。
func (g *guard) authFailed(ip string) (blocked bool) {
	if g.fails == nil || g.fails.Allow(ip) {
		return false
	}
	g.mu.Lock()
	g.blocked[ip] = g.clock.Now().Add(g.cfg.BlockFor)
	g.mu.Unlock()
	return true
}

// clientIP 返回请求的真实来源 IP。直连地址是可信代理时，从 X-Forwarded-For 里
// 从右往左取第一个不可信的地址（最右边的是离我们最近的一跳，左边的值客户端可以随意伪造）。
func (g *guard) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !g.trusted(host) {
		return host
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		if _, err := netip.ParseAddr(hop); err != nil {
			break // 格式不对的值不可信，退回到直连地址
		}
		if !g.trusted(hop) {
			return hop
		}
	}
	return host
}

func (g *guard) trusted(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range g.cfg.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
