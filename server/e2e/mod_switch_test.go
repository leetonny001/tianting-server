package e2e

// C6 开关、启动检查、配置检查（BE-492）：TC-CHAT-90-41、TC-MOD-03-14 后半、TC-MOD-01-10、TC-ACC-01-22、TC-ACC-10-05、TC-MOD-04-08 的 e2e 部分。
// 本地结果，CI 不运行。

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
	"tianting/server/internal/protocol"
)

// TC-CHAT-90-41（AC-CHAT-90-2 的 C6 部分）、TC-MOD-03-14 ②（AC-MOD-03-14 后半）：IM_MOD_ENABLED=0（默认）时 report 403 / 4030
// featureDisabled，格式错误仍先报 4000（格式在开关之前）；features.moderation=false；IM_CLIENT_CUSTOM_MSG 用开发默认（1），
// 客户端发 custom 照常成功（TC-MOD-02-11，现有 e2e 不改代码全过是同一次 go test ./e2e 的结果）。
func TestTC_CHAT_90_41_ModSwitchOff(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("u1", "u2")
	u1, u2 := s.dial("u1", "web"), s.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-ms0001", "hi"))
	for label, data := range map[string]map[string]any{
		"message": {"targetType": "message", "convId": ack.ConvID, "msgIds": []string{ack.MsgID}, "reason": "abuse"},
		"user":    {"targetType": "user", "userId": "u1", "reason": "spam"},
		"group":   {"targetType": "group", "groupId": "g1", "reason": "spam"},
	} {
		f := u2.call(protocol.CmdReport, data)
		if f.Error == nil || f.Error.Code != protocol.CodeForbidden || f.Error.Reason != protocol.ReasonFeatureDisabled {
			t.Errorf("%s: %+v", label, f.Error)
		}
	}
	mustErr(t, u2.call(protocol.CmdReport, map[string]any{"targetType": "user", "userId": "u1", "reason": "xxx"}), protocol.CodeBadRequest)
	if f := publicConfig(t, s.env)["features"].(map[string]any); f["moderation"] != false {
		t.Errorf("features = %v", f)
	}
	f := u1.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u2", "clientMsgId": "cmid-ms0002", "type": protocol.MsgCustom, "body": map[string]any{"data": map[string]any{"k": "v"}}})
	if f.Error != nil {
		t.Fatalf("custom with the development default: %+v", f.Error)
	}
	for {
		if m := u2.waitCmd(protocol.CmdMsg).msg(t); m.ClientMsgID == "cmid-ms0002" {
			if m.Type != protocol.MsgCustom {
				t.Errorf("u2 got %+v", m)
			}
			break
		}
	}
}

// 开关打开：features.moderation=true（只控制举报入口，backend-chat 第 96 条）。
func TestModSwitchOnFeature(t *testing.T) {
	e := newC6Env(t, nil)
	if f := publicConfig(t, e.env)["features"].(map[string]any); f["moderation"] != true {
		t.Errorf("features = %v", f)
	}
}

// TC-MOD-01-10 ③ 的 app 层（backend-chat 第 99 条）：开关打开但没有 IM_REPORT_KEY、长度不对，im-server 启动失败（config 层见 internal/config）。
// 开关关闭、没有密钥照常启动。
func TestTC_MOD_01_10_AppNeedsReportKey(t *testing.T) {
	base := func(mutate func(*app.Config)) error {
		cfg := app.Config{DBPath: filepath.Join(t.TempDir(), "im.db"), AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey,
			Gateway: gateway.DefaultConfig(), TestMode: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		mutate(&cfg)
		a, err := app.New(cfg)
		if err == nil {
			a.Close()
		}
		return err
	}
	if err := base(func(c *app.Config) { c.Moderation.Enabled = true }); err == nil || !strings.Contains(err.Error(), "IM_REPORT_KEY") {
		t.Errorf("no key: %v", err)
	}
	if err := base(func(c *app.Config) { c.Moderation.Enabled, c.ReportKey = true, []byte("short") }); err == nil || !strings.Contains(err.Error(), "IM_REPORT_KEY") {
		t.Errorf("short key: %v", err)
	}
	if err := base(func(c *app.Config) {}); err != nil {
		t.Errorf("switch off without a key: %v", err)
	}
}

// TC-ACC-01-22 的 app 层（AC-ACC-01-9）：生产 open 模式要求词库非空，迁移之后读库判断；空词库拒绝启动，导入一个特征词后启动。
func TestTC_ACC_01_22_OpenNeedsWords(t *testing.T) {
	db := filepath.Join(t.TempDir(), "im.db")
	start := func() (*app.App, error) {
		return app.New(app.Config{DBPath: db, AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey, Gateway: gateway.DefaultConfig(),
			RequireWords: true, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	}
	if _, err := start(); err == nil || !strings.Contains(err.Error(), "blocked word list is empty") {
		t.Fatalf("empty word list: %v", err)
	}
	insertWord(t, db, "ZXQWORD", "disabled") // 停用的词不算
	if _, err := start(); err == nil {
		t.Fatal("only a disabled word: started")
	}
	insertWord(t, db, "ZXQ-OPEN-1", "active")
	a, err := start()
	if err != nil {
		t.Fatalf("one active word: %v", err)
	}
	a.Close()
}

// insertWord 直接写一个词（启动检查的用例：服务还没起来，不能走管理接口）。norm、compact 这里只要唯一。
func insertWord(t *testing.T, path, word, state string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO blocked_words (word_id, word, norm, compact, state, created_at, updated_at)
		VALUES ((SELECT COALESCE(MAX(word_id), 0) + 1 FROM blocked_words), ?, ?, ?, ?, 1, 1)`, word, strings.ToLower(word), strings.ToLower(word), state); err != nil {
		t.Fatal(err)
	}
}

// 配置检查的"词库是否为空"（F-MOD-01 规则 6，AC-MOD-01-8 第 3 种的检查部分，TC-CHAT-90-47 第 2 条）：为空时告警，Detail 只有数量。
func TestModWordsDynamicCheck(t *testing.T) {
	e := newC6Env(t, nil)
	check := func() app.DynamicCheck {
		for _, c := range e.app.DynamicChecks(t.Context()) {
			if c.ID == "moderation_words" {
				return c
			}
		}
		t.Fatal("no moderation_words check")
		return app.DynamicCheck{}
	}
	if c := check(); c.Passed || !strings.Contains(c.Detail, "词库为空") {
		t.Errorf("empty: %+v", c)
	}
	insertWord(t, e.dbPath, "ZXQWORD", "active")
	if c := check(); !c.Passed || !strings.Contains(c.Detail, "1 个启用的词") || strings.Contains(c.Detail, "ZXQWORD") {
		t.Errorf("one word: %+v", c)
	}
}

// TC-CHAT-90-47 的指标部分（BE-492 第 4 条）：/metrics 有 im_reports_pending、im_mod_reload_errors_total、im_report_undecryptable_total；
// im_blocked_total 在有命中之后出现（BE-494）。
func TestModMetricsExposed(t *testing.T) {
	e := newC6Env(t, nil)
	srv := httptest.NewServer(e.app.MetricsHandler())
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"im_reports_pending 0", "im_mod_reload_errors_total 0", "im_report_undecryptable_total 0"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("/metrics lacks %q", want)
		}
	}
	// 故障注入接口认识 C6 的两个字段（行为见 BE-493、BE-495 的用例）。
	e.faults(map[string]any{"wordsReload": false, "evidenceWrite": false})
}

// ---- 进程级（真实二进制，生产环境） ----

func c6ProdEnv(t *testing.T, db string) []string {
	return []string{
		"IM_ENV=production", "IM_NODE_ID=n1", "IM_OPS_ADDR=off", "IM_DB=" + db,
		"IM_JWT_SECRET=jwt-0123456789abcdef0123456789abcdef", "IM_ADMIN_KEY=adm-0123456789abcdef0123456789abcdef", "IM_ADMIN_LEGACY=1",
		"IM_QUARANTINE_KEY=ops-test-quarantine-key-0123456789abcdef",
		"IM_REPORT_KEY=ZmFrZS1yZXBvcnQta2V5LWZvci10ZXN0cy0wMDAwMDA=", // fake-report-key-for-tests-000000
	}
}

func migrated(t *testing.T, bin string) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "im.db")
	if out, err := runBin(t, bin, []string{"IM_DB=" + db}, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v\n%s", err, out)
	}
	return db
}

func refused(t *testing.T, bin, label string, env []string, want string) {
	t.Helper()
	out, err := runBin(t, bin, append(env, "IM_ADDR=127.0.0.1:0", "IM_ADMIN_ADDR=off", "IM_METRICS_ADDR=off"))
	if err == nil {
		t.Errorf("%s: started\n%s", label, out)
		return
	}
	if !strings.Contains(out, want) {
		t.Errorf("%s: output does not mention %q:\n%s", label, want, out)
	}
	for _, secret := range []string{"ZmFrZS1yZXBvcnQta2V5", "fake-report-key", "jwt-0123456789abcdef", "ops-test-quarantine-key"} {
		if strings.Contains(out, secret) {
			t.Errorf("%s: output prints a key:\n%s", label, out)
		}
	}
}

// TC-MOD-01-10 ①②（AC-MOD-01-10）、TC-ACC-10-05（AC-ACC-10-5）：真实二进制，生产 + IM_MOD_ENABLED=1：V13 关、隐私说明版本为空都非 0 退出，
// 错误信息指出缺的那一条，不打印密钥；补齐后正常启动。
func TestTC_MOD_01_10_ProductionBinary(t *testing.T) {
	bin := buildServer(t)
	db := migrated(t, bin)
	env := append(c6ProdEnv(t, db), "IM_MOD_ENABLED=1")
	refused(t, bin, "V13 off", append(env, "IM_PRIVACY_NOTICE_VERSION=2026-11"), "IM_MOD_ENABLED=1 requires IM_V13_ADMIN_FEATURES=1")
	refused(t, bin, "no privacy notice", append(env, "IM_V13_ADMIN_FEATURES=1"), "IM_MOD_ENABLED=1 requires IM_PRIVACY_NOTICE_VERSION")
	refused(t, bin, "report key equals quarantine key", append(env, "IM_V13_ADMIN_FEATURES=1", "IM_PRIVACY_NOTICE_VERSION=2026-11", "IM_REPORT_KEY=ops-test-quarantine-key-0123456789abcdef"), "IM_REPORT_KEY")
	startServer(t, bin, db, append(env, "IM_V13_ADMIN_FEATURES=1", "IM_PRIVACY_NOTICE_VERSION=2026-11")...)
}

// TC-ACC-01-22（AC-ACC-01-9）：真实二进制，生产 open 模式分别缺 IM_MOD_ENABLED、V13、IM_PUBLIC_OPEN_ACK、用空词库，都非 0 退出并指出缺的那一条；
// 四项都满足时正常启动。
func TestTC_ACC_01_22_ProductionOpenBinary(t *testing.T) {
	bin := buildServer(t)
	db := migrated(t, bin)
	full := append(c6ProdEnv(t, db), "IM_REGISTER_MODE=open", "IM_MOD_ENABLED=1", "IM_V13_ADMIN_FEATURES=1", "IM_PUBLIC_OPEN_ACK=1", "IM_PRIVACY_NOTICE_VERSION=2026-11")
	without := func(name string) []string {
		var out []string
		for _, kv := range full {
			if !strings.HasPrefix(kv, name+"=") {
				out = append(out, kv)
			}
		}
		return out
	}
	refused(t, bin, "no IM_MOD_ENABLED", without("IM_MOD_ENABLED"), "IM_REGISTER_MODE=open requires IM_MOD_ENABLED=1")
	refused(t, bin, "no V13", without("IM_V13_ADMIN_FEATURES"), "IM_REGISTER_MODE=open requires IM_V13_ADMIN_FEATURES=1")
	refused(t, bin, "no ack", without("IM_PUBLIC_OPEN_ACK"), "IM_REGISTER_MODE=open requires IM_PUBLIC_OPEN_ACK=1")
	refused(t, bin, "empty word list", full, "blocked word list is empty")
	insertWord(t, db, "ZXQ-OPEN-1", "active")
	startServer(t, bin, db, full...)
}

// TC-MOD-04-08（AC-MOD-04-8）：真实二进制，console 同时配 report.evidence 和 msg.meta 非 0 退出，指出正文类 scope 的限制；
// report.evidence 只在没有别的 scope 的 console-msgread 上时正常启动。
func TestTC_MOD_04_08_CallerConfigBinary(t *testing.T) {
	bin := buildServer(t)
	dir := t.TempDir()
	write := func(callers []map[string]any) string {
		b, _ := json.Marshal(callers)
		p := filepath.Join(dir, "callers.json")
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// fake-caller-secret-for-tests-001 / -002
	sec := map[string]string{"x": "ZmFrZS1jYWxsZXItc2VjcmV0LWZvci10ZXN0cy0wMDE=", "y": "ZmFrZS1jYWxsZXItc2VjcmV0LWZvci10ZXN0cy0wMDI="}
	db := filepath.Join(dir, "im.db")
	env := []string{"IM_DB=" + db, "IM_JWT_SECRET=e2e-secret-0123456789"}
	bad := write([]map[string]any{{"id": "console", "alg": "hmac-sha256", "secret": sec["x"], "bodyAccess": true, "scopes": []string{"msg.meta", "report.evidence"}}})
	refused(t, bin, "console with report.evidence and msg.meta", append(env, "IM_ADMIN_CALLERS="+bad), "message body scopes are only allowed on a separate bodyAccess caller")
	good := write([]map[string]any{
		{"id": "console", "alg": "hmac-sha256", "secret": sec["x"], "scopes": []string{"msg.meta", "moderation.words", "report.read", "report.handle"}},
		{"id": "console-msgread", "alg": "hmac-sha256", "secret": sec["y"], "bodyAccess": true, "scopes": []string{"msg.body.group", "msg.quarantine", "report.evidence"}},
	})
	startServer(t, bin, db, "IM_ADMIN_CALLERS="+good)
}
