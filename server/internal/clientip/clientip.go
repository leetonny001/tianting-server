// Package clientip 解析请求的真实来源 IP（可信代理 + X-Forwarded-For），网关和 /v1/* 共用（PRD-chat 第 4.2 节 5a，R-CS-35）。
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Resolver 持有可信代理的网段。只有直连地址在其中时才读取 X-Forwarded-For，否则任何人都能伪造这个头来绕过按 IP 的限制。
type Resolver struct {
	TrustedProxies []netip.Prefix
}

// IP 返回请求的真实来源 IP。直连地址是可信代理时，从 X-Forwarded-For 里
// 从右往左取第一个不可信的地址（最右边的是离我们最近的一跳，左边的值客户端可以随意伪造）。
func (res Resolver) IP(r *http.Request) string {
	host := peer(r)
	if !res.Trusted(host) {
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
		if !res.Trusted(hop) {
			return hop
		}
	}
	return host
}

// FromTrustedProxy 表示这个请求的直连地址是可信代理（ARCH S2：只信任它的 X-Forwarded-Proto）。
func (res Resolver) FromTrustedProxy(r *http.Request) bool { return res.Trusted(peer(r)) }

// Trusted 判断地址是否在可信代理网段内。
func (res Resolver) Trusted(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range res.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func peer(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// LimitKey 是按 IP 限制时用的键：IPv4 是地址本身，IPv6 按 /64 聚合（R-CS-06）。解析失败时原样返回。
func LimitKey(ip string) string { return prefix(ip, 32, 64) }

// EventPrefix 是写进 account_events 和 seen_ip_prefixes 的前缀：IPv4 /24，IPv6 /64（F-ACC-03 规则 4）。不存完整地址。
func EventPrefix(ip string) string { return prefix(ip, 24, 64) }

func prefix(ip string, bits4, bits6 int) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap().WithZone("")
	bits := bits6
	if addr.Is4() {
		bits = bits4
	}
	p, err := addr.Prefix(bits)
	if err != nil {
		return ip
	}
	return p.String()
}
