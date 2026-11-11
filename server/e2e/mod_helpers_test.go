package e2e

// C6 敏感词和举报 e2e 的公共设施（BE-492～BE-496）。环境 E1（TC-MOD 通用约定）：测试模式、SQLite、IM_MOD_ENABLED=1、
// IM_V13_ADMIN_FEATURES=1、测试里的 IM_REPORT_KEY（一看就是假的）；调用方 console（moderation.words、report.read、report.handle、
// msg.meta 等）、console-msgread（bodyAccess：msg.body.group、msg.quarantine、report.evidence）；IM_C2C_BODY_ACCESS 默认 off。
// **只用特征词**（F-MOD-01 规则 5）：违规词、badword、ZXQWORD 和 ZXQ-* 特征串。本地结果，CI 不运行。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
)

// testReportKey 是测试用的 IM_REPORT_KEY（解码后的 32 字节），明文就是"假的"。testReportKey2 用来造"换了一把密钥"（AC-MOD-04-14）。
const (
	testReportKey  = "fake-report-key-for-tests-000000"
	testReportKey2 = "fake-report-key-for-tests-111111"
)

type c6Env struct {
	*socEnv
	cons   testCaller // console 主凭证
	reader testCaller // console-msgread（bodyAccess）
	sup    testCaller // 用例自己配的其他调用方（没有某个 scope 的）
	dbPath string
}

func c6Callers() (cons, reader testCaller) {
	cons = hmacCaller("console", adminauth.ScopeModerationWords, adminauth.ScopeReportRead, adminauth.ScopeReportHandle, adminauth.ScopeMsgMeta,
		adminauth.ScopeMsgDelete, adminauth.ScopeAuditRead, adminauth.ScopeUserRead, adminauth.ScopeUserWrite, adminauth.ScopeGroupRead, adminauth.ScopeGroupWrite,
		adminauth.ScopeMuteWrite, adminauth.ScopeUserBan)
	reader = hmacCaller("console-msgread", adminauth.ScopeMsgBodyGroup, adminauth.ScopeMsgBodyC2C, adminauth.ScopeMsgQuarantine, adminauth.ScopeReportEvidence)
	reader.cfg.BodyAccess = true
	return cons, reader
}

// applyC6 是 E1 的 C6 部分（TC-MOD 通用约定）。
func applyC6(c *app.Config, cons, reader testCaller) {
	c.V13AdminFeatures = true
	c.QuarantineKey = testQuarantineKey
	c.Moderation.Enabled = true
	c.ReportKey = []byte(testReportKey)
	c.SendRate = 10000
	c.AdminCallers = append(c.AdminCallers, cons.cfg, reader.cfg)
}

func newC6Env(t *testing.T, mutate func(*app.Config)) *c6Env {
	t.Helper()
	cons, reader := c6Callers()
	var db string
	s := newSocEnvCfg(t, func(c *app.Config) {
		applyC6(c, cons, reader)
		if mutate != nil {
			mutate(c)
		}
		db = c.DBPath
	})
	return &c6Env{socEnv: s, cons: cons, reader: reader, dbPath: db}
}

// as 用调用方 c 签名调用管理接口（操作人 op1）。
func (e *c6Env) as(c testCaller, method, path string, body any) (int, []byte) {
	e.t.Helper()
	return e.asOp(c, "op1", method, path, body)
}

func (e *c6Env) asOp(c testCaller, operator, method, path string, body any) (int, []byte) {
	e.t.Helper()
	return e.signed(c, method, path, body, signOpts{ts: timeMs(e.now), operator: operator, requestID: fmt.Sprintf("r-%d", reqSeq.Add(1))})
}

func (e *c6Env) mustAs(c testCaller, method, path string, body any, out any) {
	e.t.Helper()
	code, b := e.as(c, method, path, body)
	if code != http.StatusOK {
		e.t.Fatalf("%s %s: %d %s", method, path, code, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			e.t.Fatalf("%s %s: %v %s", method, path, err, b)
		}
	}
}

func (e *c6Env) db() *sql.DB {
	e.t.Helper()
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { db.Close() })
	return db
}

func (e *c6Env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db().QueryRow(query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// faults 打开或关闭 C6 的故障注入（/admin/test/faults 的 wordsReload、evidenceWrite）。
func (e *env) faults(body map[string]any) {
	e.t.Helper()
	if code, b := e.adminPost("/admin/test/faults", body, adminKey); code != http.StatusOK {
		e.t.Fatalf("faults: %d %s", code, b)
	}
}

// c6FileEnv 是 E1 + 假 S3（IM_FILES_ENABLED=1），时钟冻结在 Asia/Shanghai 当天 01:00。
type c6FileEnv struct {
	*fileEnv
	cons, reader testCaller
	now          time.Time
}

func newC6FileEnv(t *testing.T, mutate func(*app.Config)) *c6FileEnv {
	t.Helper()
	cons, reader := c6Callers()
	e := newFileEnv(t, func(c *app.Config) {
		applyC6(c, cons, reader)
		if mutate != nil {
			mutate(c)
		}
	})
	f := &c6FileEnv{fileEnv: e, cons: cons, reader: reader}
	f.now = e.freezeDay()
	return f
}

func (e *c6FileEnv) advanceTo(ts time.Time) {
	e.t.Helper()
	e.setClock(ts)
	e.now = ts
}

func (e *c6FileEnv) as(c testCaller, method, path string, body any) (int, []byte) {
	e.t.Helper()
	return e.asOp(c, "op1", method, path, body)
}

func (e *c6FileEnv) asOp(c testCaller, operator, method, path string, body any) (int, []byte) {
	e.t.Helper()
	return e.signed(c, method, path, body, signOpts{ts: e.now, operator: operator, requestID: fmt.Sprintf("r-%d", reqSeq.Add(1))})
}

// addWord 加一个词并等它生效（同一节点的失效回调是异步重建）。
func (e *c6FileEnv) addWord(word string) {
	e.t.Helper()
	if code, b := e.as(e.cons, http.MethodPost, "/admin/moderation/words", map[string]any{"word": word}); code != http.StatusOK {
		e.t.Fatalf("add word: %d %s", code, b)
	}
	time.Sleep(200 * time.Millisecond)
}

// callV1 发 /v1/*，带 Bearer。
func (e *env) callV1(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func newDebugLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// blockedMetric 读 im_blocked_total{scene="..."}（没有这个序列时是 0）。
func blockedMetric(t *testing.T, a *app.App, scene string) float64 {
	t.Helper()
	srv := httptest.NewServer(a.MetricsHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	prefix := `im_blocked_total{scene="` + scene + `"} `
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			var f float64
			json.Unmarshal([]byte(v), &f)
			return f
		}
	}
	return 0
}

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
