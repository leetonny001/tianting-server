package admin

import (
	"net/http"
	"strings"
	"testing"
)

// AC-KEY-02-3：没有声明 scope 的管理路由在注册阶段 panic。
func TestRouteWithoutScopePanics(t *testing.T) {
	a := New(AuthConfig{LegacyKey: "k"}, nil, nil, nil, nil, nil, nil, false, nil)
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "route without scope") {
			t.Fatalf("recover() = %v, want panic about missing scope", r)
		}
	}()
	a.route(http.NewServeMux(), "GET", "/admin/new-thing", "", func(http.ResponseWriter, *http.Request) {})
}

// 每个已注册路由都有 scope；v1.2 的 9 个和测试接口允许旧密钥，其他不允许。
func TestRegisteredRoutesHaveScopes(t *testing.T) {
	a := New(AuthConfig{LegacyKey: "k"}, nil, nil, nil, nil, nil, nil, true, nil)
	mux := http.NewServeMux()
	a.Register(mux)
	a.RegisterTestRoutes(mux, nil, nil)
	routes := a.Routes()
	legacy := 0
	for _, r := range routes {
		if r.Scope == "" {
			t.Errorf("%s %s has no scope", r.Method, r.Pattern)
		}
		if r.Legacy {
			legacy++
		}
	}
	if legacy != len(legacyRoutes)+4 {
		t.Errorf("legacy routes = %d, want %d (v1.2 routes + 4 test routes)", legacy, len(legacyRoutes)+4)
	}
}
