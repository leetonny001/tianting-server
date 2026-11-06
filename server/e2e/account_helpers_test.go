package e2e

// 账号 e2e 的公共设施（BE-405 第 3 条）。`/v1/*` 用 refresh 测试密钥；Argon2 默认用轻量参数省时间，
// 只有断言参数和耗时的用例用真实参数。换 IP 用 IM_TRUSTED_PROXIES + X-Forwarded-For（R-CQ-40）。

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/store"
)

// lightArgon 只用于不关心参数和耗时的用例：-race 下真实参数单次要 1 秒以上。
var lightArgon = account.ArgonParams{MemoryKiB: 256, Time: 1, Threads: 1}

type acctEnv struct {
	*env
	dbPath string
	st     *store.SQLite // 第二个连接：预置邀请码、已注销用户，读断言
	nextIP atomic.Int64
}

// newAcctEnv 启动账号环境：invite 模式、测试模式、信任本机代理。
func newAcctEnv(t *testing.T, mutate func(*app.Config)) *acctEnv {
	t.Helper()
	path := filepath.Join(t.TempDir(), "im.db")
	e := newEnvCfg(t, func(c *app.Config) {
		c.DBPath = path
		c.Account.RefreshKeys = []account.RefreshKey{{ID: "t1", Secret: []byte("e2e-refresh-key-0123456789abcdef")}}
		c.Account.RefreshActive = "t1"
		c.Account.Argon = lightArgon
		c.Gateway.Guard.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
		if mutate != nil {
			mutate(c)
		}
	})
	st, err := store.OpenSQLite(path, store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &acctEnv{env: e, dbPath: path, st: st}
}

// shanghaiMidnight 是 Asia/Shanghai 当天 00:00（R-CQ-41）：时钟用例从这里起算，避免跨日。
func shanghaiMidnight(t *testing.T) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	y, m, d := time.Now().In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func (e *acctEnv) setClock(ts time.Time) {
	e.t.Helper()
	if code, b := e.adminPost("/admin/test/clock", map[string]any{"nowMs": ts.UnixMilli()}, adminKey); code != http.StatusOK {
		e.t.Fatalf("set clock: %d %s", code, b)
	}
}

// post 发 /v1/*，ip 非空时带 X-Forwarded-For。返回状态码、响应体和响应头。
func (e *acctEnv) post(path, ip string, body any) (int, []byte, http.Header) {
	e.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		e.t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out, resp.Header
}

func (e *acctEnv) get(path, bearer string) (int, []byte) {
	e.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// freshIP 每次给一个没用过的地址，避开按 IP 的限制。
func (e *acctEnv) freshIP() string {
	n := e.nextIP.Add(1)
	return "198.51." + strconv.FormatInt(100+n/250, 10) + "." + strconv.FormatInt(1+n%250, 10)
}

type apiErr struct {
	Code    int            `json:"code"`
	Msg     string         `json:"msg"`
	Reason  string         `json:"reason"`
	Details map[string]any `json:"details"`
}

func parseErr(t *testing.T, b []byte) apiErr {
	t.Helper()
	var e apiErr
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return e
}

func wantErr(t *testing.T, label string, status int, body []byte, wantStatus, wantCode int, wantReason string) apiErr {
	t.Helper()
	e := parseErr(t, body)
	if status != wantStatus || e.Code != wantCode || e.Reason != wantReason {
		t.Errorf("%s: status %d, %s; want %d / %d reason %q", label, status, body, wantStatus, wantCode, wantReason)
	}
	return e
}

// rawDB 打开只读连接，查表做断言。
func (e *acctEnv) rawDB() *sql.DB {
	e.t.Helper()
	db, err := store.OpenSQLiteReadOnly(e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { db.Close() })
	return db
}

func (e *acctEnv) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.rawDB().QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

func hasAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// validateHTTP 用 protocol/schema/http-client.schema.json 校验响应（AC-CHAT-90-10）。
func validateHTTP(t *testing.T, def string, body []byte) {
	t.Helper()
	f, err := os.Open("../../protocol/schema/http-client.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	const id = "https://im-mvp.local/protocol/http-client.schema.json"
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id + "#/$defs/" + def)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	if err := s.Validate(v); err != nil {
		t.Errorf("%s does not match %s: %v", body, def, err)
	}
}

// 测试账号用的合法值。
const (
	goodPassword = "Tt-pass-2026!"
	testPlatform = "android"
)

type authResp struct {
	UserID            string `json:"userId"`
	AccessToken       string `json:"accessToken"`
	AccessExpiresAt   int64  `json:"accessExpiresAt"`
	RefreshToken      string `json:"refreshToken"`
	RefreshExpiresAt  int64  `json:"refreshExpiresAt"`
	AbsoluteExpiresAt int64  `json:"absoluteExpiresAt"`
	RecoveryCode      string `json:"recoveryCode"`
	RecoveryCodeGen   int64  `json:"recoveryCodeGen"`
}

func (e *acctEnv) newInvite(code string, maxUses int, expiresAt time.Time) {
	e.t.Helper()
	inv := store.Invite{InviteID: "inv-" + code, CodeHash: account.InviteHash(code), CodeLast4: code[len(code)-4:], MaxUses: maxUses, ExpiresAt: expiresAt.UnixMilli(), CreatedAt: 1}
	if err := e.st.CreateInvite(context.Background(), inv); err != nil {
		e.t.Fatal(err)
	}
}

func regBody(username string, extra map[string]any) map[string]any {
	b := map[string]any{"username": username, "password": goodPassword, "platform": testPlatform}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// register 注册并断言成功。env 是 invite 模式时要在 extra 里带 inviteCode。
func (e *acctEnv) register(username string, extra map[string]any) authResp {
	e.t.Helper()
	code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(username, extra))
	if code != http.StatusOK {
		e.t.Fatalf("register %s: %d %s", username, code, b)
	}
	var r authResp
	if err := json.Unmarshal(b, &r); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// openEnv 是 open 模式的账号环境（测试环境可以开，生产拒绝启动）。
func openEnv(t *testing.T, mutate func(*app.Config)) *acctEnv {
	t.Helper()
	return newAcctEnv(t, func(c *app.Config) {
		c.Account.RegisterMode = account.ModeOpen
		if mutate != nil {
			mutate(c)
		}
	})
}

func (e *acctEnv) login(username, password, ip string) (int, []byte) {
	e.t.Helper()
	code, b, _ := e.post("/v1/account/login", ip, map[string]any{"username": username, "password": password, "platform": testPlatform})
	return code, b
}

func (e *acctEnv) mustLogin(username, password, platform string) authResp {
	e.t.Helper()
	code, b, _ := e.post("/v1/account/login", e.freshIP(), map[string]any{"username": username, "password": password, "platform": platform})
	if code != http.StatusOK {
		e.t.Fatalf("login %s: %d %s", username, code, b)
	}
	var r authResp
	if err := json.Unmarshal(b, &r); err != nil {
		e.t.Fatal(err)
	}
	return r
}

// connect 用账号 token 连 /ws。
func (e *acctEnv) connect(label, token, platform string) *client {
	e.t.Helper()
	return e.dialURL(label, e.wsURL(token, platform))
}

// rawExec 用独立连接写库，只用于构造还没有接口的状态（吊销、改代号、改版本）。
func (e *acctEnv) rawExec(query string, args ...any) {
	e.t.Helper()
	db, err := sql.Open("sqlite", "file:"+e.dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
}

// captureLogs 把服务端日志写到 buffer。
func captureLogs(c *app.Config) *syncBuffer {
	buf := &syncBuffer{}
	c.Logger = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return buf
}

// metricsText 读 /metrics。
func (e *acctEnv) metricsText() string {
	e.t.Helper()
	w := httptest.NewRecorder()
	e.app.MetricsHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return w.Body.String()
}

var trustLocal = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
