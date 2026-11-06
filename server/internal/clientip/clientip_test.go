package clientip

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

// TC-ACC-02-10 的前提（R-CS-06）：IPv6 按 /64 聚合，IPv4 是地址本身；映射地址按 IPv4。
func TestLimitKey(t *testing.T) {
	for in, want := range map[string]string{
		"203.0.113.7":         "203.0.113.7/32",
		"::ffff:203.0.113.7":  "203.0.113.7/32",
		"2001:db8:1:1::1":     "2001:db8:1:1::/64",
		"2001:db8:1:1:ffff::": "2001:db8:1:1::/64",
		"2001:db8:1:2::1":     "2001:db8:1:2::/64",
		"fe80::1%en0":         "fe80::/64",
		"not an ip":           "not an ip",
	} {
		if got := LimitKey(in); got != want {
			t.Errorf("LimitKey(%q) = %q, want %q", in, got, want)
		}
	}
	if got := EventPrefix("203.0.113.7"); got != "203.0.113.0/24" {
		t.Errorf("EventPrefix = %q", got)
	}
}

func TestIPUsesForwardedForOnlyBehindTrustedProxy(t *testing.T) {
	res := Resolver{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	for _, tc := range []struct {
		remote, xff, want string
		fromProxy         bool
	}{
		{"203.0.113.9:1234", "1.2.3.4", "203.0.113.9", false},
		{"10.0.0.1:1234", "1.2.3.4", "1.2.3.4", true},
		{"10.0.0.1:1234", "9.9.9.9, 1.2.3.4, 10.0.0.2", "1.2.3.4", true},
		{"10.0.0.1:1234", "garbage", "10.0.0.1", true},
		{"10.0.0.1:1234", "", "10.0.0.1", true},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := res.IP(r); got != tc.want {
			t.Errorf("%s %q: %q, want %q", tc.remote, tc.xff, got, tc.want)
		}
		if got := res.FromTrustedProxy(r); got != tc.fromProxy {
			t.Errorf("%s: from proxy %v", tc.remote, got)
		}
	}
}
