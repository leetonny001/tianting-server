package gateway

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"im-mvp/server/internal/clock"
)

func TestClientIP(t *testing.T) {
	g := newGuard(GuardConfig{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, clock.New())
	for _, tc := range []struct {
		name, remote, xff, want string
	}{
		{"direct, no header", "203.0.113.5:1234", "", "203.0.113.5"},
		// 直连地址不可信：伪造的 X-Forwarded-For 被忽略，否则可以轻易绕过按 IP 限流。
		{"untrusted peer spoofs header", "203.0.113.5:1234", "1.2.3.4", "203.0.113.5"},
		{"trusted proxy", "10.0.0.2:80", "198.51.100.7", "198.51.100.7"},
		// 客户端在最左边塞了伪造值：取最右边第一个不可信的地址。
		{"client prepends fake hop", "10.0.0.2:80", "1.2.3.4, 198.51.100.7", "198.51.100.7"},
		{"chain of trusted proxies", "10.0.0.2:80", "198.51.100.7, 10.0.0.9", "198.51.100.7"},
		{"garbage hop", "10.0.0.2:80", "not-an-ip", "10.0.0.2"},
		{"only trusted hops", "10.0.0.2:80", "10.0.0.3", "10.0.0.2"},
	} {
		r := httptest.NewRequest("GET", "/ws", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := g.clientIP(r); got != tc.want {
			t.Errorf("%s: clientIP = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestGuardBlocksAfterAuthFailures(t *testing.T) {
	clk := clock.New()
	t0 := time.UnixMilli(1_700_000_000_000)
	clk.Freeze(t0)
	g := newGuard(GuardConfig{AuthFailBurst: 2, AuthFailPer: time.Minute, BlockFor: 5 * time.Minute}, clk)

	if g.authFailed("1.1.1.1") || g.authFailed("1.1.1.1") {
		t.Fatal("blocked within the allowed failures")
	}
	if !g.authFailed("1.1.1.1") {
		t.Fatal("third failure must block")
	}
	if g.allowHandshake("1.1.1.1") {
		t.Error("blocked IP allowed")
	}
	if !g.allowHandshake("2.2.2.2") {
		t.Error("other IPs must not be affected")
	}
	clk.Freeze(t0.Add(5 * time.Minute))
	if !g.allowHandshake("1.1.1.1") {
		t.Error("block must expire")
	}
}

func TestGuardHandshakeRate(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	g := newGuard(GuardConfig{HandshakeRate: 1, HandshakeBurst: 3}, clk)
	for i := range 3 {
		if !g.allowHandshake("ip") {
			t.Fatalf("handshake %d rejected", i)
		}
	}
	if g.allowHandshake("ip") {
		t.Error("4th handshake in the same instant must be rejected")
	}
}
