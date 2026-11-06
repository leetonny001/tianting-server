package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/protocol"
)

func v1Mux(v *V1, h http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	v.Handle(mux, "POST /v1/x", h)
	return mux
}

func echoBody(v *V1) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if v.ReadBody(w, r, &body) {
			WriteJSON(w, map[string]any{"ok": true})
		}
	}
}

func errCode(t *testing.T, b []byte) protocol.Error {
	t.Helper()
	var e protocol.Error
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return e
}

// TC-CHAT-90-14（AC-CHAT-90-16）：65 KB 400 / 4000，64 KB 进入业务校验。e2e 用真实的登录接口再验一遍。
func TestV1BodyLimit(t *testing.T) {
	v := &V1{}
	srv := httptest.NewServer(v1Mux(v, echoBody(v)))
	defer srv.Close()
	post := func(size int) (int, []byte) {
		payload := `{"x":"` + strings.Repeat("a", size-len(`{"x":""}`)) + `"}`
		resp, err := http.Post(srv.URL+"/v1/x", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	if code, b := post(64 << 10); code != 200 {
		t.Errorf("64 KB: %d %s", code, b)
	}
	code, b := post(65 << 10)
	if code != 400 || errCode(t, b).Code != protocol.CodeBadRequest {
		t.Errorf("65 KB: %d %s", code, b)
	}
}

// 错误信息是固定的，不回显请求内容（请求里有密码）。
func TestV1InvalidJSONDoesNotEcho(t *testing.T) {
	v := &V1{}
	srv := httptest.NewServer(v1Mux(v, echoBody(v)))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/x", "application/json", strings.NewReader(`{"password": ZXQPW-secret}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 400 || bytes.Contains(b, []byte("ZXQPW")) {
		t.Errorf("%d %s", resp.StatusCode, b)
	}
}

// TC-CHAT-90-14：慢请求到时间就断开。这里用短超时验机制；e2e 有一条真实 10 秒的。
func TestV1SlowRequestDisconnected(t *testing.T) {
	v := &V1{ReadTimeout: 200 * time.Millisecond}
	srv := httptest.NewServer(v1Mux(v, echoBody(v)))
	defer srv.Close()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("POST /v1/x HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n{")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(make([]byte, 64))
	if n != 0 || !errors.Is(err, io.EOF) {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatal("the server did not close the connection")
		}
		t.Errorf("read %d bytes, err %v; want EOF without a response", n, err)
	}
}

// 处理时间超过读超时不能取消请求的 context：读完请求体之后要取消读截止时间。
func TestV1HandlerOutlivesReadTimeout(t *testing.T) {
	v := &V1{ReadTimeout: 100 * time.Millisecond}
	srv := httptest.NewServer(v1Mux(v, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if !v.ReadBody(w, r, &body) {
			return
		}
		select {
		case <-r.Context().Done():
			WriteError(w, protocol.Errorf(protocol.CodeInternal, "context canceled"))
		case <-time.After(400 * time.Millisecond):
			WriteJSON(w, map[string]any{"ok": true})
		}
	}))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/v1/x", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("%d %s", resp.StatusCode, b)
	}
}

// TC-CHAT-90-14（第 3 步）：生产环境直连 http 拒绝；可信代理转发的 https 放行；不可信来源伪造的头不算。
func TestV1RequireHTTPS(t *testing.T) {
	v := &V1{RequireHTTPS: true}
	h := v1Mux(v, echoBody(v))
	do := func(remote, proto string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/x", strings.NewReader(`{}`))
		r.RemoteAddr = remote
		if proto != "" {
			r.Header.Set("X-Forwarded-Proto", proto)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	v.Proxies = clientip.Resolver{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	for _, tc := range []struct {
		remote, proto string
		want          int
	}{
		{"203.0.113.1:1", "", 403},
		{"203.0.113.1:1", "https", 403},
		{"10.0.0.1:1", "http", 403},
		{"10.0.0.1:1", "", 403},
		{"10.0.0.1:1", "https", 200},
	} {
		w := do(tc.remote, tc.proto)
		if w.Code != tc.want {
			t.Errorf("%s proto=%q: %d, want %d", tc.remote, tc.proto, w.Code, tc.want)
		}
		if hsts := w.Header().Get("Strict-Transport-Security"); (hsts != "") != (tc.want == 200) {
			t.Errorf("%s proto=%q: HSTS %q", tc.remote, tc.proto, hsts)
		}
	}
}

type fakeVerifier struct {
	claims auth.Claims
	err    error
}

func (f fakeVerifier) Verify(string) (auth.Claims, error) { return f.claims, f.err }

type fakeChecker struct {
	version int64
	err     error
	session auth.SessionState
}

func (f fakeChecker) TokenVersion(context.Context, string) (int64, error) { return f.version, f.err }
func (f fakeChecker) AccountAccessState(context.Context, string) (auth.AccountState, error) {
	return auth.AccountState{}, f.err
}
func (f fakeChecker) Promote(_ context.Context, c auth.Claims, _ string) auth.Result {
	return auth.Result{Claims: c, Verdict: auth.VerdictOK}
}
func (f fakeChecker) AccessSessionState(context.Context, string) (auth.SessionState, error) {
	return f.session, nil
}

// Bearer 的判定和 WS 一样（第 3 条）：4010、4012、4011；sid 的吊销和被替换的代；查库失败 5030 不算凭证问题。
func TestV1Bearer(t *testing.T) {
	sidClaims := auth.Claims{UserID: "u1", Version: 1, SID: "s1", Gen: 2}
	current := auth.SessionState{Found: true, CurrentGen: 2}
	for name, tc := range map[string]struct {
		header   string
		verifier fakeVerifier
		checker  fakeChecker
		code     int
		reason   string
	}{
		"没有头":       {"", fakeVerifier{}, fakeChecker{}, protocol.CodeTokenInvalid, ""},
		"不是 Bearer": {"Basic abc", fakeVerifier{}, fakeChecker{}, protocol.CodeTokenInvalid, ""},
		"签名错":       {"Bearer t", fakeVerifier{err: auth.ErrInvalidToken}, fakeChecker{}, protocol.CodeTokenInvalid, ""},
		"被下线":       {"Bearer t", fakeVerifier{claims: auth.Claims{UserID: "u1"}}, fakeChecker{version: 1}, protocol.CodeTokenRevoked, ""},
		"过期且被下线":    {"Bearer t", fakeVerifier{claims: auth.Claims{UserID: "u1"}, err: auth.ErrExpiredToken}, fakeChecker{version: 1}, protocol.CodeTokenRevoked, ""},
		"过期":        {"Bearer t", fakeVerifier{claims: auth.Claims{UserID: "u1"}, err: auth.ErrExpiredToken}, fakeChecker{}, protocol.CodeTokenExpired, ""},
		"会话吊销":      {"Bearer t", fakeVerifier{claims: sidClaims}, fakeChecker{version: 1, session: auth.SessionState{Found: true, RevokedAt: 5, CurrentGen: 2}}, protocol.CodeTokenRevoked, ""},
		"会话不存在":     {"Bearer t", fakeVerifier{claims: sidClaims}, fakeChecker{version: 1}, protocol.CodeTokenRevoked, ""},
		"被替换的代":     {"Bearer t", fakeVerifier{claims: sidClaims}, fakeChecker{version: 1, session: auth.SessionState{Found: true, CurrentGen: 3}}, protocol.CodeTokenExpired, protocol.ReasonTokenSuperseded},
		"查库失败":      {"Bearer t", fakeVerifier{claims: auth.Claims{UserID: "u1"}}, fakeChecker{err: errors.New("db")}, protocol.CodeServiceUnavailable, ""},
		"通过":        {"Bearer t", fakeVerifier{claims: sidClaims}, fakeChecker{version: 1, session: current}, 0, ""},
	} {
		t.Run(name, func(t *testing.T) {
			v := &V1{Verifier: tc.verifier, Checker: tc.checker}
			r := httptest.NewRequest("GET", "/v1/x?token=ignored", nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			_, ok := v.Bearer(w, r)
			if tc.code == 0 {
				if !ok {
					t.Fatalf("rejected: %s", w.Body)
				}
				return
			}
			e := errCode(t, w.Body.Bytes())
			if ok || e.Code != tc.code || e.Reason != tc.reason || w.Code != protocol.HTTPStatus(tc.code) {
				t.Errorf("ok=%v status=%d %+v", ok, w.Code, e)
			}
		})
	}
}
