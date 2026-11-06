package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/protocol"
)

// /v1/* 的基础约定（PRD-chat 第 4.2 节，BE-411）。
const (
	maxV1Body       = 64 << 10         // 请求体上限（第 4 条）
	defaultV1Read   = 10 * time.Second // 读请求超时，未校准（第 5 条）：现有服务端不设 ReadTimeout，否则会断开 WebSocket
	v1WriteTimeout  = 30 * time.Second // 写响应超时，防止慢读的客户端一直占着处理函数
	hstsHeaderValue = "max-age=31536000"
)

// V1 是 /v1/* 的基座：HTTPS、读超时、请求体上限、错误格式、Bearer。
type V1 struct {
	// RequireHTTPS 是生产环境（第 7 条，ARCH S2）：只接受 TLS，或可信代理转发的 X-Forwarded-Proto=https。
	RequireHTTPS bool
	Proxies      clientip.Resolver
	Verifier     auth.TokenVerifier
	Checker      auth.Checker
	// ReadTimeout 只有测试才改，0 值是 10 秒。
	ReadTimeout time.Duration
	Log         interface {
		Error(msg string, args ...any)
	}
}

// Handle 注册一个 /v1/* 路由。处理函数只拿到通过了 HTTPS 检查、设好了读写超时的请求。
func (v *V1) Handle(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if v.RequireHTTPS {
			if !v.isHTTPS(r) {
				// PRD 只写了"拒绝"，没有错误码；用 403 / 4030，不执行任何业务。
				WriteError(w, protocol.Errorf(protocol.CodeForbidden, "https required"))
				return
			}
			w.Header().Set("Strict-Transport-Security", hstsHeaderValue)
		}
		rc := http.NewResponseController(w)
		// 这些截止时间是这个请求的；net/http 在下一个请求开始前会重设，所以不影响 keep-alive。
		timeout := v.ReadTimeout
		if timeout <= 0 {
			timeout = defaultV1Read
		}
		_ = rc.SetReadDeadline(time.Now().Add(timeout))
		_ = rc.SetWriteDeadline(time.Now().Add(v1WriteTimeout))
		h(w, r)
	})
}

func (v *V1) isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return v.Proxies.FromTrustedProxy(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// ReadBody 读请求体到 dst。读超时：断开连接（AC-CHAT-90-16）；超过 64 KB 或 JSON 不合法：400 / 4000，返回 false。
// 错误信息是固定的：解析错误可能带上请求内容，请求里有密码。
func (v *V1) ReadBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxV1Body))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			WriteError(w, protocol.Errorf(protocol.CodeBadRequest, "request body too large"))
			return false
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			panic(http.ErrAbortHandler) // 断开连接，不写响应，net/http 不记日志
		}
		WriteError(w, protocol.Errorf(protocol.CodeBadRequest, "cannot read request"))
		return false
	}
	// 请求体读完了：取消读截止时间，否则到点后 net/http 的后台读会取消这个请求的 context（Argon2 排队可能要一段时间）。
	_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
	if json.Unmarshal(b, dst) != nil {
		WriteError(w, protocol.Errorf(protocol.CodeBadRequest, "invalid json"))
		return false
	}
	return true
}

// Bearer 校验 Authorization: Bearer，判定和 WS auth 相同（第 3 条）。token 不从 URL 读。失败时已经写好响应。
func (v *V1) Bearer(w http.ResponseWriter, r *http.Request) (auth.Claims, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		WriteError(w, protocol.Errorf(protocol.CodeTokenInvalid, "missing bearer token"))
		return auth.Claims{}, false
	}
	res := auth.Check(r.Context(), v.Verifier, v.Checker, token, v.Proxies.IP(r))
	if res.Verdict == auth.VerdictOK {
		return res.Claims, true
	}
	v.writeVerdict(w, res)
	return auth.Claims{}, false
}

// writeVerdict 把不通过的判定写成响应。
func (v *V1) writeVerdict(w http.ResponseWriter, res auth.Result) {
	switch res.Verdict {
	case auth.VerdictInvalid:
		WriteError(w, protocol.Errorf(protocol.CodeTokenInvalid, "invalid token"))
	case auth.VerdictExpired:
		WriteError(w, protocol.Errorf(protocol.CodeTokenExpired, "token expired"))
	case auth.VerdictSuperseded:
		WriteError(w, protocol.WithReason(protocol.CodeTokenExpired, protocol.ReasonTokenSuperseded, "token superseded"))
	case auth.VerdictRevoked, auth.VerdictSessionRevoked, auth.VerdictAccountDeleted, auth.VerdictCredentialsOld:
		WriteError(w, protocol.Errorf(protocol.CodeTokenRevoked, "token revoked"))
	default:
		if v.Log != nil {
			v.Log.Error("check bearer token", "err", res.Err)
		}
		WriteError(w, protocol.Errorf(protocol.CodeServiceUnavailable, "service unavailable"))
	}
}

// PeekBearer 判定 Authorization: Bearer，不写响应（logout：Bearer 和 refresh token 有一个有效就执行，F-ACC-04 规则 1）。
func (v *V1) PeekBearer(r *http.Request) (auth.Result, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return auth.Result{}, false
	}
	return auth.Check(r.Context(), v.Verifier, v.Checker, token, v.Proxies.IP(r)), true
}

// WriteError 写 {code, msg, reason?, details?}，HTTP 状态取 errors.json。
func WriteError(w http.ResponseWriter, e *protocol.Error) {
	writeJSON(w, protocol.HTTPStatus(e.Code), e)
}

// WriteJSON 写成功响应。
func WriteJSON(w http.ResponseWriter, v any) { writeJSON(w, http.StatusOK, v) }
