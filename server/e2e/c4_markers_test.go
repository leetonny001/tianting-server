package e2e

// C4 的特征串检查（BE-480 第 1 条：ZXQ-RCL-*、ZXQ-QUO-*、ZXQ-MEN-* 单独一个文件）：AC-CHAT-90-5 的 C4 部分，TC-CHAT-90-29 的 im-server 一侧。
// 撤回后库文件原始字节搜不到原文是 TC-RCL-01-06、04-08（rcl_files_test.go），引用不复制原文是 TC-QUO-01-06（quo_quote_test.go），这里不重复。
// console-server 的日志和审计在 tianting-console-server 的 e2e（TestTC_CHAT_90_29_ConsoleLogsAndAudit）。本地结果，CI 不运行。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-CHAT-90-29（im-server 日志部分）：撤回（含隔离写失败的错误路径）、引用、@、@ 所有人（含次数用完和计数故障）走一遍，
// DEBUG 级别的日志里搜不到正文和引用文字的特征串。
func TestTC_CHAT_90_29_LogsHaveNoText(t *testing.T) {
	var buf syncBuffer
	e := newC4Env(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	gid, cs := e.selfGroup("o", []string{"a"}, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-l0001", "ZXQ-RCL-1 body"))
	mustAck[protocol.SendAck](t, sendQuote(cs["o"], "group", gid, "cmid-l0002", "ZXQ-QUO-2 reply", m1.MsgID))
	mustAck[protocol.SendAck](t, sendMentions(cs["o"], gid, "cmid-l0003", "ZXQ-MEN-1 @m", []string{"m"}, false))
	for i := range 4 {
		sendMentions(cs["a"], gid, "cmid-l001"+string(rune('0'+i)), "ZXQ-MEN-2 @all", nil, true)
	}
	e.adminPost("/admin/test/faults", map[string]any{"quarantineWrite": true, "mentionWrite": true}, adminKey)
	mustErr(t, recallCall(cs["m"], conv, m1.MsgID), protocol.CodeInternal)
	mustErr(t, sendMentions(cs["o"], gid, "cmid-l0020", "ZXQ-MEN-3 @m", []string{"m"}, false), protocol.CodeInternal)
	e.adminPost("/admin/test/faults", map[string]any{"quarantineWrite": false, "mentionWrite": false}, adminKey)
	mustRecall(t, cs["m"], conv, m1.MsgID)
	mustRecall(t, cs["a"], conv, m1.MsgID)
	if code, b := e.readBody(e.mustGrant(conv, true), conv); code != http.StatusOK {
		t.Fatalf("quarantine read: %d %s", code, b)
	}
	logs := []byte(buf.String())
	if !bytes.Contains(logs, []byte("recall message")) {
		t.Fatalf("control: the injected failure should be logged:\n%s", logs)
	}
	for _, s := range []string{"ZXQ-RCL-1", "ZXQ-QUO-2", "ZXQ-MEN-1", "ZXQ-MEN-2", "ZXQ-MEN-3"} {
		if bytes.Contains(logs, []byte(s)) {
			t.Errorf("log has %s", s)
		}
	}
}

// TC-CHAT-90-29（im-server 日志和审计）：E1 + 假 S3 + 后台接口。正文、文件名、引用文字、@ 的正文都带特征串，走
// 单聊撤回、群里管理员撤回成员、群主撤回文件、引用（含引用被撤回的消息后 sync）、@、@ 所有人、后台删除带引用的消息、
// 隔离授权读正文（读到原文是对照）、fetch 已撤回的文件（拿到字节是对照），然后用 TC 指定的 deploy/ops/scan-logs
// 扫 DEBUG 日志和 admin_audit 全部行：搜不到 ZXQ-RCL、ZXQ-QUO、ZXQ-MEN；审计里有 msgId、fileId 这些标识。
func TestTC_CHAT_90_29_MarkersInLogsAndAudit(t *testing.T) {
	var logs *syncBuffer
	grp := hmacCaller("grp", adminauth.ScopeGroupWrite, adminauth.ScopeSocialSettings)
	e := newC4FileEnv(t, func(c *app.Config) {
		logs = captureLogs(c)
		c.AdminCallers = append(c.AdminCallers, grp.cfg)
	})
	// 单聊：文本带 ZXQ-RCL-1，u1 自己撤回；u2 引用一条 ZXQ-QUO-1 的消息。
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	c2c := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-z0001", "c2c ZXQ-RCL-1 text"))
	mustRecall(t, u1, c2c.ConvID, c2c.MsgID)
	q1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-z0002", "ZXQ-QUO-1 quoted"))
	mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-z0003", "reply ZXQ-QUO-2", q1.MsgID))

	// 群：o 群主、a 管理员、m 成员。
	e.users("o", "a", "m")
	if code, b, _ := e.call(grp, "/admin/groups", map[string]any{"groupId": "g1", "name": "g1", "members": []string{"o", "a", "m"}, "owner": "o"}); code != http.StatusCreated {
		t.Fatalf("create group: %d %s", code, b)
	}
	o, a, m := e.dial("o", "web"), e.dial("a", "web"), e.dial("m", "web")
	mustAck[protocol.Empty](t, o.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": "g1", "userId": "a", "admin": true}))
	const conv = "group:g1"
	docID := e.uploadFile(e.token("m"), "minutes-ZXQ-RCL-3.pdf", "application/pdf", bytes.Repeat([]byte("%PDF-1.4 "), 100))
	doc := mustAck[protocol.SendAck](t, sendFile(m, "group", "g1", "cmid-z0101", "file", docID, nil))
	gtext := mustAck[protocol.SendAck](t, m.sendText("group", "g1", "cmid-z0102", "group ZXQ-RCL-1 text"))
	gq := mustAck[protocol.SendAck](t, m.sendText("group", "g1", "cmid-z0103", "ZXQ-QUO-1 group original"))
	quoting := mustAck[protocol.SendAck](t, sendQuote(o, "group", "g1", "cmid-z0104", "ZXQ-QUO-2 group reply", gq.MsgID))
	mustAck[protocol.SendAck](t, sendQuote(a, "group", "g1", "cmid-z0105", "quote the file", doc.MsgID))
	mustAck[protocol.SendAck](t, sendMentions(o, "g1", "cmid-z0106", "ZXQ-MEN-1 @m", []string{"m"}, false))
	mustAck[protocol.SendAck](t, sendMentions(a, "g1", "cmid-z0107", "ZXQ-MEN-2 @all", nil, true))
	mustRecall(t, a, conv, gtext.MsgID) // 管理员撤回成员
	mustRecall(t, o, conv, doc.MsgID)   // 群主撤回文件
	mustRecall(t, m, conv, gq.MsgID)    // 被引用的消息被撤回，之后 sync 里引用没有文字
	if raw := syncRaw(t, a, conv); bytes.Contains(raw, []byte("ZXQ-QUO-1")) || bytes.Contains(raw, []byte("ZXQ-RCL-3")) {
		t.Errorf("sync after recall still has the quoted text or the file name: %s", raw)
	}
	e.adminDelete(e.mod, conv, quoting.MsgID, e.now) // 后台删除带引用的消息

	// 隔离授权读正文、fetch 已撤回的文件。
	code, b, _ := e.call(e.body, "/admin/msg-grants", map[string]any{"convId": conv, "operator": "op1", "reason": "case review", "ticket": "T-29", "quarantine": true})
	if code != http.StatusOK {
		t.Fatalf("grant: %d %s", code, b)
	}
	var g struct {
		GrantID string `json:"grantId"`
	}
	json.Unmarshal(b, &g)
	code, b, _ = e.call(e.body, "/admin/convs/messages/body", map[string]any{"grantId": g.GrantID, "convId": conv, "limit": 50})
	if code != http.StatusOK || !bytes.Contains(b, []byte("group ZXQ-RCL-1 text")) || !bytes.Contains(b, []byte("minutes-ZXQ-RCL-3.pdf")) {
		t.Fatalf("control: the quarantine read should return the recalled text and file name: %d %s", code, b)
	}
	code, b, _ = e.call(e.body, "/admin/files/"+docID+"/fetch", map[string]any{"convId": conv, "msgId": doc.MsgID})
	if code != http.StatusOK || !bytes.HasPrefix(b, []byte("%PDF-1.4")) {
		t.Fatalf("control: fetch the recalled file: %d (%d bytes)", code, len(b))
	}

	// 对照：审计里有标识。
	audit := dumpTable(t, e.dbPath, "admin_audit")
	for _, id := range []string{docID, doc.MsgID, quoting.MsgID} {
		if !strings.Contains(audit, id) {
			t.Errorf("control: admin_audit has no %s", id)
		}
	}
	if logs.String() == "" {
		t.Fatal("no logs captured: the scan would prove nothing")
	}
	dir := t.TempDir()
	logPath, auditPath := filepath.Join(dir, "im.log"), filepath.Join(dir, "admin_audit.txt")
	if err := os.WriteFile(logPath, []byte(logs.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(audit), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-I", "../../deploy/ops/scan-logs", "--marker=ZXQ-RCL", "--marker=ZXQ-QUO", "--marker=ZXQ-MEN", logPath, auditPath}
	if out, err := exec.Command("python3", args...).CombinedOutput(); err != nil {
		t.Errorf("scan-logs: %v\n%s", err, out)
	}
	// 对照：同一个工具在含特征串的文件上会报出来（证明扫描本身有效）。
	ctl := filepath.Join(dir, "control.txt")
	os.WriteFile(ctl, []byte("x ZXQ-QUO-1 y\n"), 0o600)
	if out, err := exec.Command("python3", "-I", "../../deploy/ops/scan-logs", "--marker=ZXQ-QUO", ctl).CombinedOutput(); err == nil {
		t.Errorf("control: scan-logs did not report a planted marker: %s", out)
	}
}

// dumpTable 把一张表逐行转成文本（扫描用）。
func dumpTable(t *testing.T, path, table string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rs, err := db.Query(`SELECT * FROM "` + table + `"`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	var b strings.Builder
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		b.WriteString(table)
		for _, v := range vals {
			if bs, ok := v.([]byte); ok {
				v = string(bs)
			}
			fmt.Fprintf(&b, "\t%v", v)
		}
		b.WriteString("\n")
	}
	return b.String()
}
