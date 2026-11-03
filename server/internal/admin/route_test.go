package admin

import (
	"net/http"

	"strings"
	"testing"
	"tianting/server/internal/adminauth"
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
	tests := 0
	for _, r := range routes {
		if r.Scope == "" {
			t.Errorf("%s %s has no scope", r.Method, r.Pattern)
		}
		isTest := r.Scope == adminauth.ScopeTest
		if isTest {
			tests++
		}
		if want := legacyRoutes[r.Method+" "+r.Pattern] || isTest; r.Legacy != want {
			t.Errorf("%s %s: legacy = %v, want %v", r.Method, r.Pattern, r.Legacy, want)
		}
	}
	if tests < 5 {
		t.Errorf("only %d test routes registered", tests)
	}
}
