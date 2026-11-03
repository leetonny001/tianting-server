package e2e

// X-5 管理接口审计 admin_audit（PRD-admin F-KEY-04）与旧密钥审计（F-KEY-05 规则 2）。

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type auditEntry struct {
	Caller, Operator, RequestID, Route, Phase string
	Code                                      int
	Targets                                   struct {
		IDs   []string
		Count int
	}
	Summary json.RawMessage
}

// auditEntries 用签名调用方查询最近 1 天的审计（extra 是附加的查询参数，如 "requestId=r-1"），按 id 倒序。
func (e *env) auditEntries(c testCaller, extra string) []auditEntry {
	e.t.Helper()
	now := time.Now().UnixMilli()
	path := "/admin/audit?from=" + itoa(now-86_400_000) + "&to=" + itoa(now+60_000) + "&limit=200"
	if extra != "" {
		path += "&" + extra
	}
	status, body := e.signed(c, http.MethodGet, path, nil, signOpts{})
	if status != http.StatusOK {
		e.t.Fatalf("query audit: %d %s", status, body)
	}
	var r struct{ Entries []auditEntry }
	json.Unmarshal(body, &r)
	return r.Entries
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// AC-KEY-04-1：console 带操作人和请求 ID 强制下线，审计有 intent、result 两条，字段齐全。
func TestAdminAuditIntentAndResult(t *testing.T) {
	c := consoleCaller()
	e := newEnvWithCallers(t, c)
	e.users("u1")
	status, body := e.signed(c, http.MethodPost, "/admin/users/u1/force-logout", map[string]any{}, signOpts{operator: "admin:a1", requestID: "r-1"})
	expectCode(t, "force logout", status, body, http.StatusOK, 0)
	got := e.auditEntries(c, "requestId=r-1")
	if len(got) != 2 {
		t.Fatalf("entries = %+v, want 2", got)
	}
	result, intent := got[0], got[1]
	for _, en := range got {
		if en.Caller != "console" || en.Operator != "admin:a1" || en.Route != "POST /admin/users/{userId}/force-logout" ||
			len(en.Targets.IDs) != 1 || en.Targets.IDs[0] != "u1" {
			t.Errorf("entry = %+v", en)
		}
	}
	if intent.Phase != "intent" || result.Phase != "result" || result.Code != 0 {
		t.Errorf("phases = %s / %s, result code %d", intent.Phase, result.Phase, result.Code)
	}
	// 失败的写操作：result 记业务错误码。
	e.signed(c, http.MethodPost, "/admin/users/ghost/force-logout", map[string]any{}, signOpts{requestID: "r-2"})
	if got := e.auditEntries(c, "requestId=r-2"); len(got) != 2 || got[0].Code != protocol.CodeUserNotFound {
		t.Errorf("failed request audit = %+v", got)
	}
	// 只读的 user.read 不记录；audit.read 本身记录。
	e.signed(c, http.MethodGet, "/admin/users/u1", nil, signOpts{requestID: "r-3"})
	if got := e.auditEntries(c, "requestId=r-3"); len(got) != 0 {
		t.Errorf("user.read was audited: %+v", got)
	}
	if got := e.auditEntries(c, "caller=console"); len(got) == 0 || !strings.HasPrefix(got[0].Route, "GET /admin/audit") {
		t.Errorf("audit.read not audited: %+v", got)
	}
}

// AC-KEY-04-2：intent 写失败时返回 503 / 5030，业务不执行（连接没有被踢）。
func TestAdminAuditWriteFailureBlocksAction(t *testing.T) {
	e := newEnv(t)
	alice := e.dial("alice", "web")
	e.adminPost("/admin/test/faults", map[string]any{"adminAuditWrite": "fail"}, adminKey)
	status, body := e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	expectCode(t, "force logout with audit down", status, body, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable)
	alice.expectNoEvent(protocol.EventKicked, 300*time.Millisecond)
	e.adminPost("/admin/test/faults", map[string]any{"adminAuditWrite": ""}, adminKey)
	status, body = e.adminPost("/admin/users/alice/force-logout", map[string]any{}, adminKey)
	expectCode(t, "force logout after recovery", status, body, http.StatusOK, 0)
	alice.expectKicked(protocol.KickBanned, protocol.CloseTokenRevoked, waitTimeout)
}

// AC-KEY-04-3（第一批部分）：审计和日志里不出现请求字段的值（这里用导入用户的昵称做特征串）。
// 好友备注、推送 data 的同类检查随 PRD-social 好友导入和第二批推送接口补上。
func TestAdminAuditRedaction(t *testing.T) {
	var logs bytes.Buffer
	c := consoleCaller()
	e := newEnvCfg(t, func(cfg *app.Config) {
		cfg.AdminCallers = []adminauth.Caller{c.cfg}
		cfg.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	const marker = "ZXQ-NICK-789"
	status, body := e.signed(c, http.MethodPost, "/admin/users/import",
		map[string]any{"users": []any{map[string]any{"userId": "u1", "nickname": marker}}}, signOpts{requestID: "r-imp"})
	expectCode(t, "import", status, body, http.StatusOK, 0)
	got := e.auditEntries(c, "requestId=r-imp")
	if len(got) != 2 || got[0].Targets.IDs[0] != "u1" {
		t.Fatalf("import audit = %+v", got)
	}
	raw, _ := json.Marshal(got)
	if bytes.Contains(raw, []byte(marker)) || bytes.Contains(logs.Bytes(), []byte(marker)) {
		t.Errorf("marker leaked into audit or logs:\naudit: %s\nlogs: %s", raw, logs.String())
	}
	if !bytes.Contains(got[0].Summary, []byte(`"users"`)) {
		t.Errorf("summary should list field names: %s", got[0].Summary)
	}
}

// AC-KEY-04-5：查询必须带 from / to，跨度不超过 31 天。
func TestAdminAuditQueryRange(t *testing.T) {
	c := consoleCaller()
	e := newEnvWithCallers(t, c)
	day := int64(86_400_000)
	for name, q := range map[string]string{
		"no range":  "",
		"no to":     "?from=0",
		"32 days":   "?from=0&to=" + itoa(32*day),
		"inverted":  "?from=10&to=5",
		"bad limit": "?from=0&to=10&limit=1000",
	} {
		status, body := e.signed(c, http.MethodGet, "/admin/audit"+q, nil, signOpts{})
		expectCode(t, name, status, body, http.StatusBadRequest, protocol.CodeBadRequest)
	}
	status, body := e.signed(c, http.MethodGet, "/admin/audit?from=0&to="+itoa(31*day), nil, signOpts{})
	expectCode(t, "31 days", status, body, http.StatusOK, 0)
}

// AC-KEY-05-2（第一批已有的新接口）：旧密钥调用新增接口一律 403 / 4030。
// 同一条 AC 里的用户搜索、解散群、好友导入、消息元数据查询随各自的接口（BE-205、BE-209、PRD-social、BE-210）补测。
// AC-KEY-05-4：旧密钥的写操作照常审计，caller=legacy、operator 为空。
func TestLegacyKeyLimitsAndAudit(t *testing.T) {
	c := consoleCaller()
	e := newEnvWithCallers(t, c)
	status, body := e.adminDo(http.MethodGet, "/admin/audit?from=0&to=1000", nil, adminKey)
	expectCode(t, "legacy key on GET /admin/audit", status, body, http.StatusForbidden, protocol.CodeForbidden)

	e.users("u1")
	e.adminPost("/admin/users/u1/force-logout", map[string]any{}, adminKey)
	got := e.auditEntries(c, "caller=legacy") // 最近的两条是强制下线（之前 e.users 的导入也记录了）
	if len(got) < 2 || got[0].Operator != "" || got[0].Route != "POST /admin/users/{userId}/force-logout" || got[1].Phase != "intent" {
		t.Errorf("legacy audit = %+v", got)
	}
}
