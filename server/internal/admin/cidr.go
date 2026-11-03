package admin

import (
	"net"
	"net/http"
	"net/netip"

	"tianting/server/internal/protocol"
)

// AllowCIDRs 限制管理端口的来源网段（PRD-admin F-KEY-01 规则 3）。prefixes 为空时不限制。
// 只看 TCP 对端地址（r.RemoteAddr），不解析 X-Forwarded-For：管理端口前面不放反向代理，
// 信任这个头等于让任何人自称来自内网。
func AllowCIDRs(prefixes []netip.Prefix, next http.Handler) http.Handler {
	if len(prefixes) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !peerAllowed(r.RemoteAddr, prefixes) {
			writeErr(w, protocol.Errorf(protocol.CodeForbidden, "source address not allowed"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func peerAllowed(remoteAddr string, prefixes []netip.Prefix) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap() // IPv4 映射的 IPv6 地址（::ffff:10.0.0.1）按 IPv4 判断
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
