package e2e

// C5 的特征串检查（BE-489 第 1 条："只影响自己"和清除的特征串检查 ZXQ-CVS-* 单独一个文件）。
// TC-CVS 通用约定：ZXQ-CVS-1 是单聊旧消息正文，ZXQ-CVS-2 是旧文件名；"搜不到"指对原始字节做子串搜索。
// 各条 AC 自己的断言在 cvs_delete_test.go、cvs_sync_test.go，这里只从 u1 能拿到的全部下行字节（convs、sync 两个方向、
// 事件、文件地址）看清除范围，从 u2、后台和库看"只影响自己"，再用 deploy/ops/scan-logs 扫日志和 admin_audit。
// 客户端本地库和缓存的部分是 SDK 用例（TC-CVS-03-10、共享向量），不在这里。本地结果，CI 不运行。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-CVS-03-01、03-02、03-07、03-15、04-05 的特征串部分：
// u2 给 u1 发正文含 ZXQ-CVS-1 的文本、图片、文件名含 ZXQ-CVS-2 的文件，再发一条普通文本；u1 在 D1 删除到 seq 4。
//  1. u1 的 D2 收到的 convDeleted 只有 convId、clearedSeq，没有特征串。
//  2. u1（D1、D2）的 convs、sync（afterSeq 和 beforeSeq）都搜不到特征串，图片和文件地址 4048。
//  3. u2 再发一条：u1 的会话重新出现，lastMsg 是新消息，sync 两个方向仍然搜不到特征串。
//  4. 对照（只影响自己）：u2 的 sync 里两个特征串都在，图片和文件地址照常；后台授权读正文有 ZXQ-CVS-1；库里原文还在（删除会话不删消息）。
//  5. 群里同样：u1 删除后搜不到，u2 照常。
//  6. DEBUG 日志和 admin_audit 用 scan-logs 扫，搜不到 ZXQ-CVS；对照：删除的日志行在，扫描工具对植入的特征串会报错。
func TestTC_CVS_03_01_MarkersOnlyAffectSelf(t *testing.T) {
	var logs *syncBuffer
	e := newC4FileEnv(t, func(c *app.Config) {
		logs = captureLogs(c)
		c.C2CBodyAccess = true
		c.ConvSettings.Enabled = true
		c.ConvSettings.RatePerSec = 10000
	})
	markers := [][]byte{[]byte("ZXQ-CVS-1"), []byte("ZXQ-CVS-2")}
	hasAny := func(b []byte) bool {
		for _, m := range markers {
			if bytes.Contains(b, m) {
				return true
			}
		}
		return false
	}
	hasAll := func(b []byte) bool {
		for _, m := range markers {
			if !bytes.Contains(b, m) {
				return false
			}
		}
		return true
	}
	convsRaw := func(c *client) []byte {
		f := c.call(protocol.CmdConvs, map[string]any{})
		if f.Error != nil {
			t.Fatalf("convs: %+v", f.Error)
		}
		return f.Data
	}
	beforeRaw := func(c *client, conv string) []byte {
		f := c.call(protocol.CmdSync, map[string]any{"convId": conv, "beforeSeq": 1000, "limit": 200})
		if f.Error != nil {
			t.Fatalf("sync beforeSeq: %+v", f.Error)
		}
		return f.Data
	}

	tok1, tok2 := e.token("u1"), e.token("u2")
	d1, d2, u2 := e.dial("u1", "web"), e.dial("u1", "ios"), e.dial("u2", "web")
	m1 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-zc01", "old ZXQ-CVS-1 text"))
	conv := m1.ConvID
	img := e.uploadImage(tok2, "photo-ZXQ-CVS-2.jpg", fakeJPEG(800, 600, 32<<10), smallThumb())
	mustAck[protocol.SendAck](t, sendFile(u2, "c2c", "u1", "cmid-zc02", "image", img, nil))
	doc := e.uploadFile(tok2, "minutes-ZXQ-CVS-2.pdf", "application/pdf", bytes.Repeat([]byte("%PDF-1.4 "), 100))
	mustAck[protocol.SendAck](t, sendFile(u2, "c2c", "u1", "cmid-zc03", "file", doc, nil))
	mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-zc04", "plain four"))
	// 对照：删除之前 u1 能看到两个特征串（文件消息带文件名；图片消息不带文件名）、能拿到图片和文件地址。
	if raw := syncRaw(t, d1, conv); !hasAll(raw) {
		t.Fatalf("control: u1 sync before the delete should have both markers: %s", raw)
	}
	e.mustURL(tok1, img, "original")
	e.mustURL(tok1, doc, "original")
	d2.drain()

	if got := mustConvDelete(t, d1, conv, 4); got != 4 {
		t.Fatalf("clearedSeq = %d", got)
	}
	ev := d2.waitEvent(protocol.EventConvDeleted)
	if hasAny(ev) {
		t.Errorf("convDeleted has a marker: %s", ev)
	}
	for _, c := range []*client{d1, d2} {
		if raw := convsRaw(c); hasAny(raw) {
			t.Errorf("%s convs after the delete has a marker: %s", c.uid, raw)
		}
		if raw := syncRaw(t, c, conv); hasAny(raw) {
			t.Errorf("%s sync after the delete has a marker: %s", c.uid, raw)
		}
		if raw := beforeRaw(c, conv); hasAny(raw) {
			t.Errorf("%s sync beforeSeq after the delete has a marker: %s", c.uid, raw)
		}
	}
	e.denied(tok1, img, "original")
	e.denied(tok1, img, "thumb")
	e.denied(tok1, doc, "original")

	m5 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-zc05", "new five"))
	raw := convsRaw(d1)
	if !bytes.Contains(raw, []byte(m5.MsgID)) || hasAny(raw) {
		t.Errorf("u1 convs after the new message: %s", raw)
	}
	for _, c := range []*client{d1, d2} {
		if raw := syncRaw(t, c, conv); !bytes.Contains(raw, []byte("new five")) || hasAny(raw) {
			t.Errorf("%s sync after the new message: %s", c.uid, raw)
		}
		if raw := beforeRaw(c, conv); hasAny(raw) {
			t.Errorf("%s sync beforeSeq after the new message has a marker: %s", c.uid, raw)
		}
	}
	e.denied(tok1, img, "original")

	// 只影响自己：u2、后台、库。
	if raw := syncRaw(t, u2, conv); !hasAll(raw) {
		t.Errorf("u2 sync lost a marker: %s", raw)
	}
	e.mustURL(tok2, img, "original")
	e.mustURL(tok2, doc, "original")
	code, b, _ := e.call(e.body, "/admin/msg-grants", map[string]any{"convId": conv, "operator": "op1", "reason": "case review", "ticket": "T-CVS", "quarantine": false})
	if code != http.StatusOK {
		t.Fatalf("grant: %d %s", code, b)
	}
	var g struct {
		GrantID string `json:"grantId"`
	}
	json.Unmarshal(b, &g)
	code, b, _ = e.call(e.body, "/admin/convs/messages/body", map[string]any{"grantId": g.GrantID, "convId": conv, "limit": 50})
	if code != http.StatusOK || !bytes.Contains(b, []byte("old ZXQ-CVS-1 text")) {
		t.Errorf("admin body after u1's delete: %d %s", code, b)
	}
	if !bytes.Contains(rawDB(t, e.dbPath), []byte("ZXQ-CVS-1")) {
		t.Error("control: the message row should still be in the database (deleting a conversation is per user)")
	}

	// 群：o 发一条含 ZXQ-CVS-1 的文本，u1 删除到最新。
	e.createGroup("g1", "o", "u1", "u2")
	o := e.dial("o", "web")
	gs := mustAck[protocol.SendAck](t, o.sendText("group", "g1", "cmid-zc11", "group ZXQ-CVS-1 text"))
	mustConvDelete(t, d1, "group:g1", gs.Seq)
	if raw := syncRaw(t, d1, "group:g1"); hasAny(raw) {
		t.Errorf("u1 group sync after the delete has a marker: %s", raw)
	}
	if raw := beforeRaw(d1, "group:g1"); hasAny(raw) {
		t.Errorf("u1 group sync beforeSeq after the delete has a marker: %s", raw)
	}
	if raw := convsRaw(d1); hasAny(raw) {
		t.Errorf("u1 convs after the group delete has a marker: %s", raw)
	}
	if raw := syncRaw(t, u2, "group:g1"); !bytes.Contains(raw, []byte("group ZXQ-CVS-1 text")) {
		t.Errorf("u2 group sync lost the text: %s", raw)
	}

	// 日志和审计。
	if !bytes.Contains([]byte(logs.String()), []byte("conversation deleted")) {
		t.Fatalf("control: the delete should be logged:\n%s", logs.String())
	}
	dir := t.TempDir()
	logPath, auditPath := filepath.Join(dir, "im.log"), filepath.Join(dir, "admin_audit.txt")
	if err := os.WriteFile(logPath, []byte(logs.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(dumpTable(t, e.dbPath, "admin_audit")), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("python3", "-I", "../../deploy/ops/scan-logs", "--marker=ZXQ-CVS", logPath, auditPath).CombinedOutput(); err != nil {
		t.Errorf("scan-logs: %v\n%s", err, out)
	}
	ctl := filepath.Join(dir, "control.txt")
	os.WriteFile(ctl, []byte("x ZXQ-CVS-2 y\n"), 0o600)
	if out, err := exec.Command("python3", "-I", "../../deploy/ops/scan-logs", "--marker=ZXQ-CVS", ctl).CombinedOutput(); err == nil {
		t.Errorf("control: scan-logs did not report a planted marker: %s", out)
	}
}
