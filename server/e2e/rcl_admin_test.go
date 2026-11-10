package e2e

// C4 撤回和后台（BE-474）：TC-RCL-04 里不需要文件的用例。环境 E1 + IM_V13_ADMIN_FEATURES=1（c4Env）。本地结果，CI 不运行。
// console-server 一侧（CS-204、CS-202）不在这一波。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type metaItem struct {
	Seq        int64  `json:"seq"`
	MsgID      string `json:"msgId"`
	BodyLength int    `json:"bodyLength"`
	FileSize   *int64 `json:"fileSize"`
	Deleted    bool   `json:"deleted"`
	Recalled   bool   `json:"recalled"`
	RecalledAt int64  `json:"recalledAt"`
	RecalledBy string `json:"recalledBy"`
}

func (e *c4Env) meta(conv string) ([]metaItem, []byte) {
	e.t.Helper()
	b := e.must("/admin/convs/messages/query", map[string]any{"convId": conv})
	var r struct {
		Messages []metaItem `json:"messages"`
	}
	json.Unmarshal(b, &r)
	return r.Messages, b
}

func metaOf(t *testing.T, items []metaItem, msgID string) metaItem {
	t.Helper()
	for _, m := range items {
		if m.MsgID == msgID {
			return m
		}
	}
	t.Fatalf("no meta for %s", msgID)
	return metaItem{}
}

// TC-RCL-04-01（AC-RCL-04-1，im-server 一侧）：后台删除已撤回的消息 200，不在 deleted 里；后台元数据 recalled 仍为 true、deleted 为 false；
// 没有事件（哨兵法）；审计有记录。
func TestTC_RCL_04_01_AdminDeleteRecalled(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4101", "ZXQ-RCL-1"))
	mustRecall(t, cs["m"], conv, m1.MsgID)
	waitMsgDeleted(t, cs["o"])
	b := e.must("/admin/messages/delete", map[string]any{"convId": conv, "msgIds": []string{m1.MsgID}, "reason": "violation"})
	if !bytes.Contains(b, []byte(`"deleted":[]`)) {
		t.Errorf("delete response = %s", b)
	}
	items, raw := e.meta(conv)
	if m := metaOf(t, items, m1.MsgID); !m.Recalled || m.Deleted || m.RecalledBy != "m" || m.RecalledAt == 0 {
		t.Errorf("meta = %+v", m)
	}
	if bytes.Contains(raw, []byte("ZXQ-RCL-1")) {
		t.Errorf("meta has the text: %s", raw)
	}
	expectNoMsgDeleted(t, cs["o"], func() { mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4102", "sentinel")) })
	found := false
	for _, a := range e.auditOf(e.mod) {
		s := auditText(a)
		if strings.Contains(s, "ZXQ-RCL-1") {
			t.Errorf("audit has the text: %s", s)
		}
		if strings.Contains(s, "/admin/messages/delete") && strings.Contains(s, conv) {
			found = true
		}
	}
	if !found {
		t.Error("no audit record for the delete")
	}
	// 用户侧仍是撤回状态。
	if m := msgBySeq(t, syncConv(t, cs["o"], conv).Messages, m1.Seq); !m.Recalled || !m.Deleted || m.DeletedAt != 0 {
		t.Errorf("client view = %+v", m)
	}
}

// TC-RCL-04-03（AC-RCL-04-3，im-server 一侧）：群消息被撤回：普通正文授权没有 body、recalled=true、deleted=false；隔离授权返回原文。
// 后台版序列化和下行版分开（RC2-21）。
func TestTC_RCL_04_03_AdminBodies(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4301", "ZXQ-RCL-1 secret"))
	mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4302", "kept"))
	mustRecall(t, cs["m"], conv, m1.MsgID)
	type bodyMsg struct {
		MsgID      string          `json:"msgId"`
		Deleted    bool            `json:"deleted"`
		Recalled   bool            `json:"recalled"`
		RecalledBy string          `json:"recalledBy"`
		Body       json.RawMessage `json:"body"`
	}
	read := func(grant string) ([]bodyMsg, []byte) {
		t.Helper()
		code, b := e.readBody(grant, conv)
		if code != 200 {
			t.Fatalf("read: %d %s", code, b)
		}
		var r struct {
			Messages []bodyMsg `json:"messages"`
		}
		json.Unmarshal(b, &r)
		return r.Messages, b
	}
	normal, raw := read(e.mustGrant(conv, false))
	if bytes.Contains(raw, []byte("ZXQ-RCL-1")) || !bytes.Contains(raw, []byte("kept")) {
		t.Errorf("normal grant: %s", raw)
	}
	for _, m := range normal {
		if m.MsgID == m1.MsgID && (m.Deleted || !m.Recalled || m.RecalledBy != "m" || m.Body != nil) {
			t.Errorf("recalled message in the normal read: %+v", m)
		}
	}
	quar, raw := read(e.mustGrant(conv, true))
	if len(quar) != 1 || quar[0].MsgID != m1.MsgID || quar[0].Deleted || !quar[0].Recalled || !bytes.Contains(quar[0].Body, []byte("ZXQ-RCL-1 secret")) {
		t.Errorf("quarantine read: %s", raw)
	}
	if bytes.Contains(raw, []byte("kept")) {
		t.Errorf("quarantine read returns messages that were not recalled: %s", raw)
	}
}

// TC-RCL-04-04（AC-RCL-04-4）：31 天后 quarantine_purge 物理删除撤回的隔离记录，隔离授权读取 404 / 4044。
func TestTC_RCL_04_04_QuarantinePurge(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4401", "ZXQ-RCL-1"))
	mustRecall(t, cs["m"], conv, m1.MsgID)
	e.advance(31 * 24 * time.Hour)
	if code, b := e.adminPost("/admin/test/jobs/quarantine_purge/run", map[string]any{}, adminKey); code != 200 {
		t.Fatalf("run job: %d %s", code, b)
	}
	var n int
	e.db().QueryRow(`SELECT count(*) FROM message_quarantine WHERE msg_id = ?`, m1.MsgID).Scan(&n)
	if n != 0 {
		t.Errorf("quarantine rows after purge = %d", n)
	}
	code, b := e.readBody(e.mustGrant(conv, true), conv)
	expectCode(t, "quarantine read after purge", code, b, 404, protocol.CodeMessageNotFound)
}

// TC-RCL-04-05（AC-RCL-04-5）：单聊撤回、IM_C2C_BODY_ACCESS=off：申请隔离授权 403 / 4030 reason=c2cBodyDisabled。
func TestTC_RCL_04_05_C2CBodyDisabled(t *testing.T) {
	e := newC4Env(t, func(c *app.Config) { c.C2CBodyAccess = false })
	u1, _, _, conv := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-r4501", "x"))
	mustRecall(t, u1, conv, m1.MsgID)
	code, b := e.grant(conv, true)
	expectCode(t, "c2c quarantine grant", code, b, 403, protocol.CodeForbidden)
	if !bytes.Contains(b, []byte(`"reason":"c2cBodyDisabled"`)) {
		t.Errorf("reason: %s", b)
	}
}

// TC-RCL-04-07（AC-RCL-04-7，文本部分；文件消息见 TC-RCL-04-08 所在的文件用例）：元数据有 recalled、recalledAt、recalledBy，没有 body，bodyLength 为 0。
func TestTC_RCL_04_07_MetaRecalled(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m")
	conv := "group:" + gid
	m1 := mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r4701", "ZXQ-RCL-1"))
	e.advance(time.Minute)
	mustRecall(t, cs["o"], conv, m1.MsgID)
	items, raw := e.meta(conv)
	m := metaOf(t, items, m1.MsgID)
	if !m.Recalled || m.RecalledAt != e.now || m.RecalledBy != "o" || m.Deleted || m.BodyLength != 0 || m.FileSize != nil {
		t.Errorf("meta = %+v", m)
	}
	if bytes.Contains(raw, []byte(`"body"`)) || bytes.Contains(raw, []byte("ZXQ-RCL-1")) {
		t.Errorf("meta response: %s", raw)
	}
}

// TC-CHAT-90-34（AC-CHAT-90-17 的 C4 部分）：发送者、群主、管理员各撤回一次，im_recalls_total{by} 三个取值都增加，标签没有用户和会话维度。
func TestTC_CHAT_90_34_RecallMetrics(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a"}, "m")
	conv := "group:" + gid
	var ids []string
	for i := range 3 {
		ids = append(ids, mustAck[protocol.SendAck](t, cs["m"].sendText("group", gid, "cmid-r9034-"+string(rune('a'+i)), "x")).MsgID)
	}
	before := map[string]float64{}
	for _, by := range []string{"sender", "owner", "admin"} {
		before[by] = e.metricValue(`im_recalls_total{by="` + by + `"}`)
	}
	mustRecall(t, cs["m"], conv, ids[0])
	mustRecall(t, cs["o"], conv, ids[1])
	mustRecall(t, cs["a"], conv, ids[2])
	mustRecall(t, cs["a"], conv, ids[2]) // 幂等重试不计
	for _, by := range []string{"sender", "owner", "admin"} {
		if got := e.metricValue(`im_recalls_total{by="` + by + `"}`); got != before[by]+1 {
			t.Errorf("im_recalls_total{by=%q} = %v, want %v", by, got, before[by]+1)
		}
	}
	rec := httptest.NewRecorder()
	e.app.MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, l := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(l, "im_recalls_total{") && (!strings.HasPrefix(l, `im_recalls_total{by="`) || strings.Count(l, "=") != 1) {
			t.Errorf("unexpected labels: %s", l)
		}
		for _, id := range append(ids, gid, "\"m\"", "\"o\"") {
			if strings.HasPrefix(l, "im_recalls_total") && strings.Contains(l, id) {
				t.Errorf("label carries an id: %s", l)
			}
		}
	}
	if got := e.metricValue(`im_commands_total{cmd="recall",code="0"}`); got < 3 {
		t.Errorf("im_commands_total for recall = %v", got)
	}
}
