package admin

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
)

// 签名请求头（PRD-admin F-KEY-03 规则 1）。
const (
	HeaderCaller     = "X-IM-Caller"
	HeaderTimestamp  = "X-IM-Timestamp"
	HeaderNonce      = "X-IM-Nonce"
	HeaderSignature  = "X-IM-Signature"
	HeaderOperator   = "X-IM-Operator"
	HeaderRequestID  = "X-Request-Id"
	HeaderTargetNode = "X-IM-Target-Node"
)

const (
	signatureWindow = 5 * time.Minute // |服务端时间 − X-IM-Timestamp| 的上限（[IM 钟]）
	maxBodyBytes    = 1 << 20
	maxRequestIDLen = 64
	maxOperatorLen  = 128
)

var nonceRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// 签名失败原因（指标 im_admin_auth_failures_total 的 reason 标签）。
const (
	failMissing   = "missing"
	failCaller    = "caller"
	failSignature = "signature"
	failWindow    = "window"
	failNonce     = "nonce"
	failNode      = "node"
)

// AuthConfig 是管理接口的鉴权配置。
type AuthConfig struct {
	// LegacyKey 是旧的共享密钥 IM_ADMIN_KEY（调用方 legacy），只能访问 v1.2 已有的路由（PRD-admin F-KEY-05）。
	LegacyKey string
	// LegacyDisabled 为 true 时不接受旧密钥（生产环境默认，除非设置 IM_ADMIN_LEGACY=1）。
	LegacyDisabled bool
	Callers        []adminauth.Caller
	Nonces         adminauth.NonceStore
	NodeID         string                 // 带 X-IM-Target-Node 的请求必须等于本节点
	Failures       *prometheus.CounterVec // im_admin_auth_failures_total{reason}，可以为 nil
}

// RouteInfo 描述一个管理路由。
type RouteInfo struct {
	Method, Pattern string
	Scope           adminauth.Scope
	Legacy          bool // 旧密钥也能访问
}

// legacyRoutes 是 v1.2 已有的 9 个接口：旧密钥只能访问它们（按"方法 + 路由"判断，不按 scope，
// 避免以后新路由复用旧 scope 时被旧密钥访问），另外测试模式下的 /admin/test/* 也可以。
var legacyRoutes = map[string]bool{
	"POST /admin/users/token":                 true,
	"POST /admin/users/import":                true,
	"GET /admin/users/{userId}":               true,
	"POST /admin/users/{userId}/force-logout": true,
	"POST /admin/groups":                      true,
	"GET /admin/groups/{groupId}":             true,
	"PATCH /admin/groups/{groupId}":           true,
	"POST /admin/groups/{groupId}/members":    true,
	"DELETE /admin/groups/{groupId}/members":  true,
}

type ctxKey struct{}

// AuthInfo 是鉴权通过后记在请求上下文里的调用信息，审计（F-KEY-04）用。
type AuthInfo struct {
	Caller    string // 调用方 id；旧密钥为 "legacy"
	Operator  string // 签名过的 X-IM-Operator；旧密钥为空
	RequestID string
}

// AuthFrom 取出请求的鉴权信息。
func AuthFrom(ctx context.Context) AuthInfo {
	info, _ := ctx.Value(ctxKey{}).(AuthInfo)
	return info
}

// route 注册一个管理路由，必须声明所需 scope（PRD-admin F-KEY-02 规则 2）；没有声明的在注册阶段 panic。
func (a *Admin) route(mux *http.ServeMux, method, pattern string, scope adminauth.Scope, h http.HandlerFunc) {
	if scope == "" {
		panic("admin: route without scope: " + method + " " + pattern)
	}
	key := method + " " + pattern
	legacy := legacyRoutes[key] || scope == adminauth.ScopeTest
	a.routes = append(a.routes, RouteInfo{Method: method, Pattern: pattern, Scope: scope, Legacy: legacy})
	mux.Handle(key, a.authenticate(scope, legacy, h))
}

// Routes 返回已注册的管理路由（测试用：遍历全部路由检查 scope）。
func (a *Admin) Routes() []RouteInfo { return append([]RouteInfo(nil), a.routes...) }

// authenticate：带 X-IM-Caller 的走签名鉴权；不带的按旧密钥判定（PRD-admin F-KEY-03 规则 6、F-KEY-05 规则 5：
// 两种头同时出现时以签名为准，忽略 X-Admin-Key）。
func (a *Admin) authenticate(scope adminauth.Scope, legacy bool, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var info AuthInfo
		if r.Header.Get(HeaderCaller) != "" {
			var ok bool
			if info, ok = a.verifySigned(w, r, scope); !ok {
				return
			}
		} else {
			if a.auth.LegacyDisabled || a.auth.LegacyKey == "" ||
				subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Admin-Key")), []byte(a.auth.LegacyKey)) != 1 {
				writeErr(w, protocol.Errorf(protocol.CodeAdminKeyInvalid, "invalid admin key"))
				return
			}
			if !legacy {
				writeErr(w, protocol.Errorf(protocol.CodeForbidden, "the legacy admin key cannot call this API; use a signed caller"))
				return
			}
			info = AuthInfo{Caller: adminauth.LegacyCallerID}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, info)))
	})
}

// verifySigned 校验签名请求。顺序：调用方 → 来源网段 → 时间窗口 → 签名 → 目标节点 → nonce → scope。
// nonce 只在签名通过之后登记，垃圾请求挤不掉合法请求的 nonce；scope 在签名之后判断，未认证的请求探测不到权限配置。
func (a *Admin) verifySigned(w http.ResponseWriter, r *http.Request, scope adminauth.Scope) (AuthInfo, bool) {
	h := r.Header
	callerID, ts, nonce, sig := h.Get(HeaderCaller), h.Get(HeaderTimestamp), h.Get(HeaderNonce), h.Get(HeaderSignature)
	operator, requestID, node := h.Get(HeaderOperator), h.Get(HeaderRequestID), h.Get(HeaderTargetNode)
	if ts == "" || sig == "" || !nonceRe.MatchString(nonce) {
		a.authFail(w, failMissing, "missing or malformed signature headers")
		return AuthInfo{}, false
	}
	if len(requestID) > maxRequestIDLen || len(operator) > maxOperatorLen {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "X-Request-Id must be at most %d and X-IM-Operator at most %d characters", maxRequestIDLen, maxOperatorLen))
		return AuthInfo{}, false
	}
	c := a.callers[callerID]
	if c == nil || c.Disabled {
		a.authFail(w, failCaller, "unknown or disabled caller")
		return AuthInfo{}, false
	}
	if len(c.AllowedCIDRs) > 0 && !peerAllowed(r.RemoteAddr, c.AllowedCIDRs) {
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "source address not allowed for this caller"))
		return AuthInfo{}, false
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	now := a.clock.Now()
	if err != nil || absDuration(now.Sub(time.UnixMilli(ms))) > signatureWindow {
		a.authFail(w, failWindow, "timestamp outside the 5 minute window")
		return AuthInfo{}, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "request body too large or unreadable"))
		return AuthInfo{}, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	sts := adminauth.StringToSign(adminauth.Request{
		Method: r.Method, Path: r.URL.EscapedPath(), RawQuery: r.URL.RawQuery, Timestamp: ts, Nonce: nonce,
		Body: body, Operator: operator, RequestID: requestID, TargetNode: node,
	})
	if !c.Verify(sts, sig) {
		a.authFail(w, failSignature, "signature mismatch")
		return AuthInfo{}, false
	}
	if node != "" && node != a.auth.NodeID {
		a.authFail(w, failNode, "X-IM-Target-Node does not match this node")
		return AuthInfo{}, false
	}
	fresh, err := a.auth.Nonces.Add(c.ID, nonce, now)
	switch {
	case errors.Is(err, adminauth.ErrNonceStoreFull):
		a.log.Error("admin nonce store full, rejecting signed requests")
		writeErr(w, protocol.Errorf(protocol.CodeServiceUnavailable, "try again later"))
		return AuthInfo{}, false
	case err != nil:
		a.internal(w, "register nonce", err)
		return AuthInfo{}, false
	case !fresh:
		a.authFail(w, failNonce, "nonce already used")
		return AuthInfo{}, false
	}
	if !c.Has(scope) {
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "caller %s lacks scope %s", c.ID, scope))
		return AuthInfo{}, false
	}
	return AuthInfo{Caller: c.ID, Operator: operator, RequestID: requestID}, true
}

// authFail 统一返回 401 / 4017；msg 区分原因只用于排查。失败计入指标，不写审计（防刷爆，PRD-admin 13.2 节）。
func (a *Admin) authFail(w http.ResponseWriter, reason, msg string) {
	if a.auth.Failures != nil {
		a.auth.Failures.WithLabelValues(reason).Inc()
	}
	writeErr(w, protocol.Errorf(protocol.CodeAdminSignatureInvalid, "%s", fmt.Sprintf("%s (%s)", msg, reason)))
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
