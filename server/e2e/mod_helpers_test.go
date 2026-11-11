package e2e

// C6 敏感词和举报 e2e 的公共设施（BE-492～BE-496）。环境 E1（TC-MOD 通用约定）：测试模式、SQLite、IM_MOD_ENABLED=1、
// IM_V13_ADMIN_FEATURES=1、测试里的 IM_REPORT_KEY（一看就是假的）；调用方 console（moderation.words、report.read、report.handle、
// msg.meta 等）、console-msgread（bodyAccess：msg.body.group、msg.quarantine、report.evidence）；IM_C2C_BODY_ACCESS 默认 off。
// **只用特征词**（F-MOD-01 规则 5）：违规词、badword、ZXQWORD 和 ZXQ-* 特征串。本地结果，CI 不运行。

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
	dbPath string
}

func newC6Env(t *testing.T, mutate func(*app.Config)) *c6Env {
	t.Helper()
	cons := hmacCaller("console", adminauth.ScopeModerationWords, adminauth.ScopeReportRead, adminauth.ScopeReportHandle, adminauth.ScopeMsgMeta,
		adminauth.ScopeMsgDelete, adminauth.ScopeAuditRead, adminauth.ScopeUserRead, adminauth.ScopeUserWrite, adminauth.ScopeGroupRead, adminauth.ScopeGroupWrite)
	reader := hmacCaller("console-msgread", adminauth.ScopeMsgBodyGroup, adminauth.ScopeMsgBodyC2C, adminauth.ScopeMsgQuarantine, adminauth.ScopeReportEvidence)
	reader.cfg.BodyAccess = true
	var db string
	s := newSocEnvCfg(t, func(c *app.Config) {
		c.V13AdminFeatures = true
		c.QuarantineKey = testQuarantineKey
		c.Moderation.Enabled = true
		c.ReportKey = []byte(testReportKey)
		c.SendRate = 10000
		c.AdminCallers = append(c.AdminCallers, cons.cfg, reader.cfg)
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
