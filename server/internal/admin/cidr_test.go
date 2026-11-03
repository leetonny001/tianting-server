package admin

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

// AC-KEY-01-4：来源不在允许网段时 403 / 4030，且不进入业务 handler；X-Forwarded-For 不起作用。
func TestAllowCIDRs(t *testing.T) {
	called := false
	h := AllowCIDRs([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	for _, tc := range []struct {
		remote string
		want   int
	}{
		{"192.168.1.10:51000", http.StatusForbidden},
		{"10.1.2.3:51000", http.StatusOK},
		{"[::ffff:10.1.2.3]:51000", http.StatusOK},
		{"[2001:db8::1]:51000", http.StatusForbidden},
	} {
		called = false
		req := httptest.NewRequest(http.MethodPost, "/admin/users/u1/force-logout", nil)
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want || called != (tc.want == http.StatusOK) {
			t.Errorf("%s: status %d called %v, want %d", tc.remote, rec.Code, called, tc.want)
		}
		if tc.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), `"code":4030`) {
			t.Errorf("%s: body %s", tc.remote, rec.Body)
		}
	}
	// 没有配置网段时不限制。
	req := httptest.NewRequest(http.MethodGet, "/admin/users/u1", nil)
	req.RemoteAddr = "192.168.1.10:51000"
	rec := httptest.NewRecorder()
	AllowCIDRs(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("no prefixes: status %d, want 200", rec.Code)
	}
}
