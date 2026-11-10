package e2e

// AC-CHAT-90-5 的 C4 部分在 im-server 日志上的检查（BE-472 第 6 条、BE-475 第 5 条）：TC-CHAT-90-29 的 im-server 一侧。
// console-server 的日志、审计扫描（scan-logs）随 BE-480 验收。本地结果，CI 不运行。

import (
	"bytes"
	"log/slog"
	"net/http"
	"testing"

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
