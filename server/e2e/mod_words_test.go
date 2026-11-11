package e2e

// C6 词库（BE-493）：TC-MOD-01-01～04、01-06、01-08、01-09 的后端部分，词库接口的 scope（TC-MOD-01-05 的 im-server 一侧）。
// "10 秒后"按规则 2 的生效时间轮询（最多 10 秒），不是固定睡 10 秒。本地结果，CI 不运行。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
)

type wordOut struct {
	WordID    int64  `json:"wordId"`
	Word      string `json:"word"`
	State     string `json:"state"`
	CreatedAt int64  `json:"createdAt"`
}

func (e *c6Env) addWord(word string) wordOut {
	e.t.Helper()
	var r struct {
		Word    wordOut `json:"word"`
		Created bool    `json:"created"`
	}
	e.mustAs(e.cons, http.MethodPost, "/admin/moderation/words", map[string]any{"word": word}, &r)
	return r.Word
}

func (e *c6Env) setWordState(id int64, state string) {
	e.t.Helper()
	e.mustAs(e.cons, http.MethodPatch, fmt.Sprintf("/admin/moderation/words/%d", id), map[string]any{"state": state}, nil)
}

// within 在 d 之内反复执行 try，直到它返回 true（规则 2：变更 10 秒内生效）。
func within(t *testing.T, d time.Duration, what string, try func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !try() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within %s", what, d)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

var cmidSeq struct {
	sync.Mutex
	n int
}

func nextCmid(prefix string) string {
	cmidSeq.Lock()
	defer cmidSeq.Unlock()
	cmidSeq.n++
	return fmt.Sprintf("%s-%06d", prefix, cmidSeq.n)
}

// sendWait 发一条文本，返回 ack 帧。
func sendWait(c *client, convType, to, text string) frame {
	c.t.Helper()
	return c.sendText(convType, to, nextCmid("cm"), text)
}

func blockedFrame(f frame) bool {
	return f.Error != nil && f.Error.Code == protocol.CodeContentBlocked
}

// 4510 没有 details，错误信息不含词（规则 6）。
func assertBlocked(t *testing.T, label string, f frame, words ...string) {
	t.Helper()
	if !blockedFrame(f) {
		t.Fatalf("%s: want 4510, got %+v (%s)", label, f.Error, f.Data)
	}
	if f.Error.Details != nil || f.Error.Reason != "" {
		t.Errorf("%s: 4510 carries details or reason: %+v", label, f.Error)
	}
	for _, w := range words {
		if strings.Contains(f.Error.Msg, w) {
			t.Errorf("%s: the error names the word: %+v", label, f.Error)
		}
	}
}

// TC-MOD-01-01（AC-MOD-01-1，吸收 AC-ADM-60-1）：后台添加"违规词"，10 秒内发送含它的消息 4510，消息不落库（sync 没有，conv_seq 没增加）。
// TC-MOD-01-02（AC-MOD-01-2）：停用后 10 秒内发送成功、u2 收到原文；再启用后又是 4510。
func TestTC_MOD_01_01_AddDisableEnable(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	first := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "你好"))
	w := e.addWord("违规词")
	if w.WordID == 0 || w.Word != "违规词" || w.State != "active" {
		t.Fatalf("add = %+v", w)
	}
	var last frame
	within(t, 10*time.Second, "word takes effect", func() bool { last = sendWait(u1, "c2c", "u2", "这是违规词"); return blockedFrame(last) })
	assertBlocked(t, "after add", last, "违规词")
	if got := syncConv(t, u2, first.ConvID); len(got.Messages) != 1 {
		t.Errorf("blocked message stored: %+v", got.Messages)
	}
	if n := e.count(`SELECT max_seq FROM conv_seq WHERE conv_id = ?`, first.ConvID); n != 1 {
		t.Errorf("conv_seq = %d", n)
	}
	e.setWordState(w.WordID, "disabled")
	var ok frame
	within(t, 10*time.Second, "disable takes effect", func() bool { ok = sendWait(u1, "c2c", "u2", "这是违规词"); return ok.Error == nil })
	for {
		if m := u2.waitCmd(protocol.CmdMsg).msg(t); m.Body.Text == "这是违规词" {
			break
		}
	}
	e.setWordState(w.WordID, "active")
	within(t, 10*time.Second, "enable takes effect", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "这是违规词")) })
}

// c6TwoNodes：两个 C6 节点共享一个 SQLite 文件（现有 twoNodes 的做法，PRD-social 5.6 节），测试模式的 Invalidator 互发。
func c6TwoNodes(t *testing.T, off bool) (*c6Env, *c6Env) {
	db := filepath.Join(t.TempDir(), "shared.db")
	n1 := newC6Env(t, func(c *app.Config) { c.DBPath = db })
	n2 := newC6Env(t, func(c *app.Config) { c.DBPath, c.TestInvalidationOff = db, off })
	n1.app.SetTestPeers([]string{n2.adminSrv.URL})
	n2.app.SetTestPeers([]string{n1.adminSrv.URL})
	return n1, n2
}

// settle 让节点在测试时钟下先完成一次重建（启动时按真实时钟加载，把时钟拨到 t0 之后第一次匹配会触发 TTL 重建），之后的断言只看失效通知。
func settle(t *testing.T, c *client, to string) {
	t.Helper()
	mustAck[protocol.SendAck](t, sendWait(c, "c2c", to, "settle"))
	time.Sleep(300 * time.Millisecond)
}

// TC-MOD-01-03（AC-MOD-01-3）：节点 1 添加，10 秒内从节点 2 发送被拦截。时钟冻结，TTL 不会触发，生效靠的是 blockedWords 主题的广播。
func TestTC_MOD_01_03_TwoNodes(t *testing.T) {
	n1, n2 := c6TwoNodes(t, false)
	n1.usersNamed("u1", "u2")
	u1 := n2.dial("u1", "web")
	settle(t, u1, "u2")
	n1.addWord("违规词")
	within(t, 10*time.Second, "node 2 blocks", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "这是违规词")) })
}

// 规则 2 的 TTL 兜底：节点 2 关掉广播，[IM 钟] 推进 59 秒仍是旧词库，过了 60 秒的下一次匹配触发重建，之后被拦截。
func TestModWordsTTLFallback(t *testing.T) {
	n1, n2 := c6TwoNodes(t, true)
	n1.usersNamed("u1", "u2")
	u1 := n2.dial("u1", "web")
	settle(t, u1, "u2")
	n1.addWord("违规词")
	time.Sleep(300 * time.Millisecond)
	n2.advance(59 * time.Second)
	mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "这是违规词"))
	n2.advance(2 * time.Second)
	within(t, 5*time.Second, "TTL reload", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "这是违规词")) })
}

// TC-MOD-01-04（AC-MOD-01-4）：一次导入 5001 条 400 / 4131、词库没变；100 条（80 个不同、10 个空行、10 个重复，其中 2 个只是大小写或全角不同）
// 结果 {added:80, duplicate:10, invalid:10}，词库正好 80 个（按 norm 去重，backend-chat 第 100 条）。
// TC-MOD-01-09（AC-MOD-01-9）：导入 64 个码点和 65 个码点（含中文和 emoji）的词：added 1、invalid 1；单个添加 65 个码点 400 / 4000（第 102 条）。
func TestTC_MOD_01_04_Import(t *testing.T) {
	e := newC6Env(t, nil)
	big := make([]string, 5001)
	for i := range big {
		big[i] = fmt.Sprintf("ZXQ-W-%04d", i)
	}
	code, body := e.as(e.cons, http.MethodPost, "/admin/moderation/words/import", map[string]any{"words": big})
	expectCode(t, "5001", code, body, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	if n := e.count(`SELECT COUNT(*) FROM blocked_words`); n != 0 {
		t.Fatalf("5001: %d words written", n)
	}
	var words []string
	for i := range 80 {
		words = append(words, fmt.Sprintf("ZXQ-W-%04d", i))
	}
	for i := range 10 { // 空行（含只有空白的）
		words = append(words, strings.Repeat(" ", i%3))
	}
	for i := range 8 {
		words = append(words, fmt.Sprintf("ZXQ-W-%04d", i))
	}
	words = append(words, "zxq-w-0010", "ＺＸＱ-Ｗ-0011") // 大小写、全角不同
	var r struct{ Added, Duplicate, Invalid int }
	e.mustAs(e.cons, http.MethodPost, "/admin/moderation/words/import", map[string]any{"words": words}, &r)
	if r.Added != 80 || r.Duplicate != 10 || r.Invalid != 10 {
		t.Errorf("import 100 = %+v", r)
	}
	if n := e.count(`SELECT COUNT(*) FROM blocked_words`); n != 80 {
		t.Errorf("words = %d", n)
	}
	// TC-MOD-01-09：按码点数。
	w64 := strings.Repeat("举", 32) + strings.Repeat("😀", 31) + "x"
	w65 := w64 + "y"
	e.mustAs(e.cons, http.MethodPost, "/admin/moderation/words/import", map[string]any{"words": []string{w64, w65}}, &r)
	if r.Added != 1 || r.Invalid != 1 || r.Duplicate != 0 {
		t.Errorf("64 / 65 code points = %+v", r)
	}
	code, body = e.as(e.cons, http.MethodPost, "/admin/moderation/words", map[string]any{"word": w65})
	expectCode(t, "single 65", code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	code, body = e.as(e.cons, http.MethodPost, "/admin/moderation/words", map[string]any{"word": "  "})
	expectCode(t, "single empty", code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	if bytes.Contains(body, []byte("举")) {
		t.Errorf("error echoes the word: %s", body)
	}
}

// 词库列表（第 11.6 节）：按 norm 子串搜索、按状态、倒序分页；原文原样返回（后台按纯文本显示，AC-MOD-01-7 的数据部分）。单个添加重复的返回已有的那条。
// TC-MOD-01-05 的 im-server 一侧：没有 moderation.words 的调用方 403 / 4030，词库不变。不存在的 wordId 404 / 4040。
func TestModWordsListAndScope(t *testing.T) {
	sup := hmacCaller("support", adminauth.ScopeUserRead, adminauth.ScopeMsgMeta, adminauth.ScopeReportRead)
	e := newC6Env(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, sup.cfg) })
	a := e.addWord("<b>ZXQ-W-1</b>")
	b := e.addWord("ZXQ-W-2")
	e.setWordState(b.WordID, "disabled")
	if again := e.addWord("zxq-w-2"); again.WordID != b.WordID || again.State != "disabled" || again.Word != "ZXQ-W-2" {
		t.Errorf("duplicate add = %+v", again)
	}
	var page struct {
		Words      []wordOut `json:"words"`
		NextCursor int64     `json:"nextCursor"`
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/moderation/words?q=ＺＸＱ-w&limit=1", nil, &page)
	if len(page.Words) != 1 || page.Words[0].WordID != b.WordID || page.NextCursor != b.WordID {
		t.Fatalf("page 1 = %+v", page)
	}
	e.mustAs(e.cons, http.MethodGet, fmt.Sprintf("/admin/moderation/words?q=zxq-w&limit=1&cursor=%d", page.NextCursor), nil, &page)
	if len(page.Words) != 1 || page.Words[0].Word != "<b>ZXQ-W-1</b>" || page.NextCursor != 0 {
		t.Fatalf("page 2 = %+v", page)
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/moderation/words?state=active", nil, &page)
	if len(page.Words) != 1 || page.Words[0].WordID != a.WordID {
		t.Errorf("active = %+v", page)
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/admin/moderation/words"}, {http.MethodPost, "/admin/moderation/words/import"},
		{http.MethodPatch, fmt.Sprintf("/admin/moderation/words/%d", a.WordID)}, {http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", a.WordID)},
		{http.MethodGet, "/admin/moderation/words"},
	} {
		code, body := e.as(sup, c.method, c.path, map[string]any{"word": "ZXQ-W-3", "words": []string{"ZXQ-W-4"}, "state": "disabled"})
		expectCode(t, c.method+" "+c.path, code, body, http.StatusForbidden, protocol.CodeForbidden)
	}
	if n := e.count(`SELECT COUNT(*) FROM blocked_words`); n != 2 {
		t.Errorf("words changed by a caller without scope: %d", n)
	}
	code, body := e.as(e.cons, http.MethodPatch, "/admin/moderation/words/999", map[string]any{"state": "disabled"})
	expectCode(t, "patch missing", code, body, http.StatusNotFound, protocol.CodeNotFound)
	e.mustAs(e.cons, http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", a.WordID), nil, nil)
	code, body = e.as(e.cons, http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", a.WordID), nil)
	expectCode(t, "delete twice", code, body, http.StatusNotFound, protocol.CodeNotFound)
}

// TC-MOD-01-06（AC-MOD-01-6，im-server 一侧）：添加 ZXQWORD、导入 3 个带它前缀的、停用、删除：admin_audit 每个操作都有记录（路由、wordId 或数量），
// admin_audit、日志、库里除 blocked_words 本身以外都搜不到 ZXQWORD。
func TestTC_MOD_01_06_AuditHasNoWords(t *testing.T) {
	var logs lockedBuf
	e := newC6Env(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	w := e.addWord("ZXQWORD")
	var r struct{ Added int }
	e.mustAs(e.cons, http.MethodPost, "/admin/moderation/words/import", map[string]any{"words": []string{"ZXQWORD-1", "ZXQWORD-2", "ZXQWORD-3"}}, &r)
	e.setWordState(w.WordID, "disabled")
	e.mustAs(e.cons, http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", w.WordID), nil, nil)
	rows, err := e.db().Query(`SELECT route, phase, targets_json, summary_json FROM admin_audit ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var all strings.Builder
	results := map[string]string{}
	for rows.Next() {
		var route, phase, targets, summary string
		if err := rows.Scan(&route, &phase, &targets, &summary); err != nil {
			t.Fatal(err)
		}
		all.WriteString(route + targets + summary + "\n")
		if phase == "result" {
			results[route] += summary
		}
	}
	for route, want := range map[string]string{
		"POST /admin/moderation/words":            fmt.Sprintf(`"wordId":%d`, w.WordID),
		"POST /admin/moderation/words/import":     `"added":3`,
		"PATCH /admin/moderation/words/{wordId}":  fmt.Sprintf(`"wordId":%d`, w.WordID),
		"DELETE /admin/moderation/words/{wordId}": fmt.Sprintf(`"wordId":%d`, w.WordID),
	} {
		if !strings.Contains(results[route], want) {
			t.Errorf("audit of %s = %q, want %s", route, results[route], want)
		}
	}
	if strings.Contains(all.String(), "ZXQWORD") {
		t.Errorf("admin_audit contains the word:\n%s", all.String())
	}
	if strings.Contains(logs.String(), "ZXQWORD") {
		t.Errorf("logs contain the word")
	}
}

// TC-MOD-01-08（AC-MOD-01-8）：① 故障注入 wordsReload 后添加 badword：违规词仍 4510（旧词库生效），badword 成功，im_mod_reload_errors_total 增加；
// 解除注入后 10 秒内 badword 也被拦截（失败后自动重试）。③ 词库为空：配置检查告警，发送不拦截。② 启动加载失败见下一个用例。
func TestTC_MOD_01_08_ReloadFailure(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1 := e.dial("u1", "web")
	old := e.addWord("违规词")
	within(t, 10*time.Second, "old word", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "违规词")) })
	before := metricValue(t, e.app.MetricsHandler(), "im_mod_reload_errors_total")
	e.faults(map[string]any{"wordsReload": true})
	e.addWord("badword")
	within(t, 5*time.Second, "reload error counted", func() bool {
		return metricValue(t, e.app.MetricsHandler(), "im_mod_reload_errors_total") > before
	})
	assertBlocked(t, "old list", sendWait(u1, "c2c", "u2", "违规词"))
	if f := sendWait(u1, "c2c", "u2", "badword"); f.Error != nil {
		t.Fatalf("the new word must not be loaded yet: %+v", f.Error)
	}
	e.faults(map[string]any{"wordsReload": false})
	within(t, 10*time.Second, "recovered", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "badword")) })
	// ③ 清空词库：不拦截，配置检查告警。
	e.mustAs(e.cons, http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", old.WordID), nil, nil)
	var page struct{ Words []wordOut }
	e.mustAs(e.cons, http.MethodGet, "/admin/moderation/words", nil, &page)
	for _, w := range page.Words {
		e.mustAs(e.cons, http.MethodDelete, fmt.Sprintf("/admin/moderation/words/%d", w.WordID), nil, nil)
	}
	within(t, 10*time.Second, "empty list does not block", func() bool { return sendWait(u1, "c2c", "u2", "违规词 badword").Error == nil })
	for _, c := range e.app.DynamicChecks(t.Context()) {
		if c.ID == "moderation_words" && (c.Passed || !strings.Contains(c.Detail, "词库为空")) {
			t.Errorf("check = %+v", c)
		}
	}
}

// TC-MOD-01-08 ②（AC-MOD-01-8 第 2 种）：IM_MOD_ENABLED=1 时启动加载失败，im-server 启动失败。两种造法：故障注入
// IM_TEST_WORDS_STARTUP_FAIL（只在测试模式，backend-chat 第 123 条），以及真的读不了词库（表被删掉）。开关关闭时不加载，照常启动。
func TestTC_MOD_01_08_StartupLoadFailure(t *testing.T) {
	db := filepath.Join(t.TempDir(), "im.db")
	start := func(mutate func(*app.Config)) error {
		cfg := app.Config{DBPath: db, AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey, Gateway: gateway.DefaultConfig(),
			TestMode: true, Moderation: modOn(), ReportKey: []byte(testReportKey), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		mutate(&cfg)
		a, err := app.New(cfg)
		if err == nil {
			a.Close()
		}
		return err
	}
	if err := start(func(c *app.Config) {}); err != nil {
		t.Fatalf("normal start: %v", err)
	}
	if err := start(func(c *app.Config) { c.TestWordsStartupFail = true }); err == nil || !strings.Contains(err.Error(), "loading the blocked word list failed") {
		t.Errorf("injected: %v", err)
	}
	if err := start(func(c *app.Config) { c.TestWordsStartupFail, c.TestMode = true, false }); err != nil {
		t.Errorf("the fault must only work in test mode: %v", err)
	}
	sqlDB, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`DROP TABLE blocked_words`); err != nil {
		t.Fatal(err)
	}
	if err := start(func(c *app.Config) {}); err == nil || !strings.Contains(err.Error(), "loading the blocked word list failed") {
		t.Errorf("unreadable word list: %v", err)
	}
	if err := start(func(c *app.Config) { c.Moderation.Enabled = false }); err != nil {
		t.Errorf("switch off does not load: %v", err)
	}
}

func modOn() logic.ModConfig { return logic.ModConfig{Enabled: true} }

// metricValue 读一个没有标签的指标的值。
func metricValue(t *testing.T, h http.Handler, name string) float64 {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), name+" "); ok {
			var f float64
			json.Unmarshal([]byte(v), &f)
			return f
		}
	}
	t.Fatalf("no metric %s", name)
	return 0
}
