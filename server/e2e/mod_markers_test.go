package e2e

// C6 的特征串检查（BE-499 第 1 条：ZXQWORD、ZXQ-RPT-*、ZXQ-MOD-* 单独一个文件）：AC-CHAT-90-5 的 C6 部分（TC-CHAT-90-43 的 im-server 一侧），
// 以及 TC-MOD 通用约定的"搜不到"：对日志、im-server 审计 admin_audit、SQLite 主库和 WAL（先 wal_checkpoint(TRUNCATE)）做字节搜索。
// 各条 AC 自己的断言在 mod_*_test.go（TC-MOD-01-06 审计不记词、TC-MOD-02-08 命中不记全文、TC-MOD-03-05 / 03-08 证据加密），这里把 C6 的
// 全部流程串起来再扫一遍。console-server 的日志和 console 审计在 tianting-console-server 的 e2e（mod_words_test.go、mod_reports_test.go、
// mod_evidence_test.go 的 noneContains）。本地结果，CI 不运行。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-CHAT-90-43（im-server 部分）：E1 + 假 S3 + C4，DEBUG 日志。
//  1. 后台添加 ZXQWORD 和"违规词"（词库审计）；
//  2. 好友附言：被拦截一次（ZXQ-MOD-APPLY-1，发给存在和不存在的人）、成功一次（ZXQ-MOD-APPLY-2）；
//  3. 被拦截的消息（单聊、群聊，正文 ZXQ-MOD-BODY）；
//  4. 举报群文本 ZXQ-RPT-1（note ZXQ-MOD-NOTE）和群文件 ZXQ-MOD-FILE.pdf，单聊消息 ZXQ-RPT-2（off，只存元数据）；
//  5. 审核员看详情、授权、读证据（读到原文、note、文件名是对照）、fetch 文件、结案，查命中记录。
//
// 预期：
//   - 日志和 admin_audit 搜不到任何特征串（另用 deploy/ops/scan-logs 扫，对照：植入的特征串会被报出）；
//   - 主库和 WAL 的原始字节搜不到 ZXQ-MOD-BODY、ZXQ-MOD-APPLY-1、ZXQ-MOD-NOTE（被拦截的没有写入；note 只在密文里）；
//   - 逐表：ZXQWORD 只在 blocked_words；ZXQ-RPT-1、ZXQ-RPT-2 只在 messages、conv_seq（原消息本身和会话的最后一条）；ZXQ-MOD-FILE 只在 files（文件消息的 body
//     不带文件名，以服务端记录为准）；ZXQ-MOD-APPLY-2 只在 friend_requests（reports、report_evidence、moderation_hits、admin_audit 都没有）。
func TestTC_CHAT_90_43_ModMarkers(t *testing.T) {
	var logs *syncBuffer
	e := newC6FileEnv(t, func(c *app.Config) {
		logs = captureLogs(c)
		c.MsgActions.Enabled = true
		c.QuarantineKey = testQuarantineKey
	})
	tok := e.token("u1")
	e.token("u2")
	e.token("u3")
	e.token("u9")
	e.createGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")

	// 1. 词库。
	for _, w := range []string{"ZXQWORD", "违规词"} {
		e.addWord(w)
	}
	within(t, 10*time.Second, "words", func() bool { return blockedFrame(sendWait(u1, "c2c", "u2", "probe 违规词 ZXQWORD")) })

	// 2. 好友附言。
	for _, to := range []string{"u9", "nouser"} {
		assertBlocked(t, "friendApply "+to, u2.call(protocol.CmdFriendApply, map[string]any{"to": to, "message": "ZXQ-MOD-APPLY-1 违规词"}), "违规词")
	}
	mustAck[socialAck](t, u2.call(protocol.CmdFriendApply, map[string]any{"to": "u9", "message": "ZXQ-MOD-APPLY-2 你好"}))

	// 3. 被拦截的消息和群名。
	assertBlocked(t, "c2c", sendWait(u1, "c2c", "u2", "ZXQ-MOD-BODY ZXQWORD"), "ZXQWORD")
	assertBlocked(t, "group", sendWait(u1, "group", "g1", "ZXQ-MOD-BODY 违规词"), "违规词")

	// 4. 举报。
	doc := e.uploadFile(tok, "ZXQ-MOD-FILE.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	mText := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "ZXQ-RPT-1 群消息"))
	mDoc := mustAck[protocol.SendAck](t, sendFile(u1, "group", "g1", "mk-doc-0001", "file", doc, nil))
	mC2C := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "ZXQ-RPT-2 单聊"))
	r1 := mustReport(t, reportMsg(u2, mText.ConvID, []string{mText.MsgID}, "abuse", strp("ZXQ-MOD-NOTE")))
	r2 := mustReport(t, reportMsg(u2, mDoc.ConvID, []string{mDoc.MsgID}, "spam", strp("ZXQ-MOD-NOTE file")))
	r3 := mustReport(t, reportMsg(u2, mC2C.ConvID, []string{mC2C.MsgID}, "fraud", strp("ZXQ-MOD-NOTE c2c")))
	mustReport(t, reportCall(u2, map[string]any{"targetType": "user", "userId": "u1", "reason": "other", "note": "ZXQ-MOD-NOTE user"}))

	// 5. 审核员。
	grant := func(rid string) string {
		code, b := e.as(e.reader, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": rid, "reason": "核查"})
		var g struct{ GrantID string }
		if code != http.StatusOK || json.Unmarshal(b, &g) != nil || g.GrantID == "" {
			t.Fatalf("grant %s: %d %s", rid, code, b)
		}
		return g.GrantID
	}
	for _, rid := range []string{r1, r2, r3} {
		if code, b := e.as(e.cons, http.MethodGet, "/admin/reports/"+rid, nil); code != http.StatusOK {
			t.Fatalf("detail: %d %s", code, b)
		}
	}
	g1, g2 := grant(r1), grant(r2)
	_, ev1 := e.as(e.reader, http.MethodPost, "/admin/reports/"+r1+"/evidence", map[string]any{"grantId": g1})
	_, ev2 := e.as(e.reader, http.MethodPost, "/admin/reports/"+r2+"/evidence", map[string]any{"grantId": g2})
	if !bytes.Contains(ev1, []byte("ZXQ-RPT-1")) || !bytes.Contains(ev1, []byte("ZXQ-MOD-NOTE")) || !bytes.Contains(ev2, []byte("ZXQ-MOD-FILE.pdf")) {
		t.Fatalf("control: the evidence should have the text, note and file name: %s %s", ev1, ev2)
	}
	code, b := e.as(e.reader, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": r3, "reason": "核查"})
	expectCode(t, "c2c grant", code, b, http.StatusForbidden, protocol.CodeForbidden)
	if code, b := e.as(e.reader, http.MethodPost, "/admin/files/"+doc+"/fetch", map[string]any{"convId": mDoc.ConvID, "msgId": mDoc.MsgID, "grantId": g2}); code != http.StatusOK || len(b) != 2048 {
		t.Fatalf("control: fetch the evidence file: %d", code)
	}
	for _, rid := range []string{r1, r2, r3} {
		if code, b := e.as(e.cons, http.MethodPost, "/admin/reports/"+rid+"/resolve", map[string]any{"result": "handled", "reason": "已核实"}); code != http.StatusOK {
			t.Fatalf("resolve: %d %s", code, b)
		}
	}
	if code, b := e.as(e.cons, http.MethodGet, "/admin/moderation/hits?userId=u1", nil); code != http.StatusOK || !bytes.Contains(b, []byte(`"text"`)) {
		t.Fatalf("hits: %d %s", code, b)
	}

	markers := []string{"ZXQWORD", "ZXQ-MOD-APPLY-1", "ZXQ-MOD-APPLY-2", "ZXQ-MOD-BODY", "ZXQ-MOD-NOTE", "ZXQ-MOD-FILE", "ZXQ-RPT-1", "ZXQ-RPT-2"}

	// 日志、admin_audit。
	audit := dumpTable(t, e.dbPath, "admin_audit")
	for _, id := range []string{r1, r2, doc, g1[:8]} {
		if !strings.Contains(audit, id) {
			t.Errorf("control: admin_audit has no %s", id)
		}
	}
	text := logs.String()
	for _, want := range []string{"content blocked", r1} {
		if !strings.Contains(text, want) {
			t.Fatalf("control: the logs have no %q, the scan would prove nothing", want)
		}
	}
	for _, m := range markers {
		if strings.Contains(text, m) {
			t.Errorf("logs contain %s", m)
		}
		if strings.Contains(audit, m) {
			t.Errorf("admin_audit contains %s", m)
		}
	}
	dir := t.TempDir()
	logPath, auditPath := filepath.Join(dir, "im.log"), filepath.Join(dir, "admin_audit.txt")
	if err := os.WriteFile(logPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(audit), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("python3", "-I", "../../deploy/ops/scan-logs", "--marker=ZXQWORD", "--marker=ZXQ-MOD", "--marker=ZXQ-RPT", logPath, auditPath).CombinedOutput(); err != nil {
		t.Errorf("scan-logs: %v\n%s", err, out)
	}
	ctl := filepath.Join(dir, "control.txt")
	os.WriteFile(ctl, []byte("x ZXQ-MOD-NOTE y\n"), 0o600)
	if out, err := exec.Command("python3", "-I", "../../deploy/ops/scan-logs", "--marker=ZXQ-MOD", ctl).CombinedOutput(); err == nil {
		t.Errorf("control: scan-logs did not report a planted marker: %s", out)
	}

	// 主库和 WAL 的原始字节。
	raw := rawDB(t, e.dbPath)
	for _, m := range []string{"ZXQ-MOD-BODY", "ZXQ-MOD-APPLY-1", "ZXQ-MOD-NOTE"} {
		if bytes.Contains(raw, []byte(m)) {
			t.Errorf("raw database bytes contain %s", m)
		}
	}
	if !bytes.Contains(raw, []byte("ZXQ-RPT-1")) {
		t.Fatal("control: the group message should be in the raw bytes")
	}

	// 逐表。
	allowed := map[string][]string{
		"ZXQWORD":         {"blocked_words"},
		"ZXQ-RPT-1":       {"messages", "conv_seq"},
		"ZXQ-RPT-2":       {"messages", "conv_seq"},
		"ZXQ-MOD-FILE":    {"files"},
		"ZXQ-MOD-APPLY-2": {"friend_requests"},
	}
	found := map[string][]string{}
	for _, table := range sqliteTables(t, e.dbPath) {
		dump := dumpTable(t, e.dbPath, table)
		for m := range allowed {
			if strings.Contains(dump, m) {
				found[m] = append(found[m], table)
			}
		}
	}
	for m, want := range allowed {
		got := found[m]
		sort.Strings(got)
		if len(got) == 0 {
			t.Errorf("control: %s is in no table, the scan would prove nothing", m)
		}
		for _, table := range got {
			if !slices.Contains(want, table) {
				t.Errorf("%s is in table %s (tables %v), want only %v", m, table, got, want)
			}
		}
	}
}

// sqliteTables 列出库里的全部表（扫描用）。
func sqliteTables(t *testing.T, path string) []string {
	t.Helper()
	db := openDB(t, path)
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}
