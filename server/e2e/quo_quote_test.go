package e2e

// C4 引用（BE-475）：TC-QUO 里【后端】的用例。环境 E1（c4Env）；图片、文件的用 E1 + 假 S3（c4FileEnv）。本地结果，CI 不运行。

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func quoteOf(t *testing.T, m protocol.Message) protocol.Quote {
	t.Helper()
	if m.Body.Quote == nil {
		t.Fatalf("message %s has no quote: %+v", m.MsgID, m.Body)
	}
	return *m.Body.Quote
}

// TC-QUO-01-01（AC-QUO-01-1 的服务端部分）：u1 引用 u2 的消息回复，u2 收到的 msg 带 quote，state=normal，text 是原文前 100 个码点。
func TestTC_QUO_01_01_QuoteReply(t *testing.T) {
	e := newC4Env(t, nil)
	_, _, _, conv := e.c2cPair()
	u1, u2 := e.dial("u1", "pc"), e.dial("u2", "pc")
	long := strings.Repeat("长", 120)
	m1 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-q0101", long))
	u2b := e.dial("u2", "android")
	ack := mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-q0102", "回复", m1.MsgID))
	got := u2b.waitCmd(protocol.CmdMsg).msg(t)
	want := protocol.Quote{MsgID: m1.MsgID, Seq: m1.Seq, From: "u2", Type: "text", State: "normal", Text: strings.Repeat("长", 100)}
	if got.MsgID != ack.MsgID || got.Body.Text != "回复" || quoteOf(t, got) != want {
		t.Errorf("msg = %+v quote %+v", got, got.Body.Quote)
	}
	if q := quoteOf(t, msgBySeq(t, syncConv(t, u2b, conv).Messages, ack.Seq)); q != want {
		t.Errorf("sync quote = %+v", q)
	}
}

// TC-QUO-01-02（AC-QUO-01-2）：其他会话的、不存在的、已撤回的、后台删除的、tip、发送者入群前的（historyVisible=false）：
// 都是 4040 reason=quoteUnavailable，响应除 id 外完全相同，没有新消息行、seq 不前进。
func TestTC_QUO_01_02_Unavailable(t *testing.T) {
	e := newC4Env(t, nil)
	e.usersNamed("u1", "u2", "d")
	e.createGroup("g1", "u1", "u2")
	e.createGroup("g9", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	before := mustAck[protocol.SendAck](t, u2.sendText("group", "g1", "cmid-q0201", "before d joined"))
	if code, b := e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"d"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add d: %d %s", code, b)
	}
	d := e.dial("d", "web")
	other := mustAck[protocol.SendAck](t, u2.sendText("group", "g9", "cmid-q0202", "other conversation"))
	rec := mustAck[protocol.SendAck](t, u2.sendText("group", "g1", "cmid-q0203", "recalled"))
	mustRecall(t, u2, "group:g1", rec.MsgID)
	del := mustAck[protocol.SendAck](t, u2.sendText("group", "g1", "cmid-q0204", "deleted"))
	e.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{del.MsgID}, "reason": "violation"})
	e.mustAdm(http.MethodPatch, "/admin/groups/g1", map[string]any{"owner": "u1"}) // 产生一条 tip
	tip := tips(t, u1, "g1")
	maxBefore := convByID(t, u1, "group:g1").MaxSeq
	cases := []struct {
		label string
		c     *client
		id    string
	}{
		{"other conversation", u1, other.MsgID},
		{"missing", u1, "0193a7c4-0000-7000-8000-00000000beef"},
		{"recalled", u1, rec.MsgID},
		{"admin deleted", u1, del.MsgID},
		{"tip", u1, tip[len(tip)-1].MsgID},
		{"before joining", d, before.MsgID},
	}
	var first string
	for i, tc := range cases {
		f := sendQuote(tc.c, "group", "g1", "cmid-q021"+string(rune('0'+i)), "reply", tc.id)
		if f.Error == nil || f.Error.Code != protocol.CodeNotFound || f.Error.Reason != protocol.ReasonQuoteUnavailable {
			t.Errorf("%s: %+v", tc.label, f.Error)
			continue
		}
		if first == "" {
			first = errBody(f)
		} else if errBody(f) != first {
			t.Errorf("%s: response %s differs from %s", tc.label, errBody(f), first)
		}
	}
	if got := convByID(t, u1, "group:g1").MaxSeq; got != maxBefore {
		t.Errorf("seq moved: %d -> %d", maxBefore, got)
	}
	var n int
	e.db().QueryRow(`SELECT count(*) FROM messages WHERE client_msg_id LIKE 'cmid-q021%'`).Scan(&n)
	if n != 0 {
		t.Errorf("rejected messages were saved: %d", n)
	}
}

// TC-QUO-01-04（AC-QUO-01-4）：同一 clientMsgId 重发（原消息已保存，被引用的已撤回）返回第一次的 ack（去重优先），不是 4040。
func TestTC_QUO_01_04_DedupFirst(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, conv := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-q0401", "original"))
	first := mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-q0402", "re", m1.MsgID))
	mustRecall(t, u2, conv, m1.MsgID)
	again := mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-q0402", "re", m1.MsgID))
	if again != first {
		t.Errorf("resend ack %+v, first %+v", again, first)
	}
}

// TC-QUO-01-06（AC-QUO-01-6）：m2 引用 m1（ZXQ-QUO-2）：m2 行的 body 没有被引用的内容，只有 quote_msg_id；库文件里特征串只在 m1 的行。
func TestTC_QUO_01_06_NotCopied(t *testing.T) {
	e := newC4Env(t, nil)
	u1, _, u2, _ := e.c2cPair()
	m1 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-q0601", "ZXQ-QUO-2 original"))
	m2 := mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-q0602", "my reply", m1.MsgID))
	db := e.db()
	var body, quote string
	if err := db.QueryRow(`SELECT body, quote_msg_id FROM messages WHERE msg_id = ?`, m2.MsgID).Scan(&body, &quote); err != nil {
		t.Fatal(err)
	}
	if quote != m1.MsgID || strings.Contains(body, "ZXQ-QUO-2") || strings.Contains(body, "quote") || body != `{"text":"my reply"}` {
		t.Errorf("m2 row: body %s quote_msg_id %q", body, quote)
	}
	var ids []string
	rows, _ := db.Query(`SELECT msg_id FROM messages WHERE body LIKE '%ZXQ-QUO-2%'`)
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != 1 || ids[0] != m1.MsgID {
		t.Errorf("rows with the string: %v", ids)
	}
	if n := bytes.Count(rawDB(t, e.dbPath), []byte("ZXQ-QUO-2")); n == 0 {
		t.Error("control: m1 should still be on disk")
	}
}

// TC-QUO-02-01（AC-QUO-02-1）：撤回 m1 后 sync m2：quote.state=recalled，没有 text，整个响应搜不到 ZXQ-QUO-1；
// 后台删除 m3 后 m4 的引用 state=deleted。撤回和后台删除都递增会话的 modVer，重拉能拿到新状态。
func TestTC_QUO_02_01_StateAfterRecallAndDelete(t *testing.T) {
	e := newC4Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m1 := mustAck[protocol.SendAck](t, u2.sendText("group", "g1", "cmid-q2101", "ZXQ-QUO-1 secret"))
	m2 := mustAck[protocol.SendAck](t, sendQuote(u1, "group", "g1", "cmid-q2102", "reply to m1", m1.MsgID))
	m3 := mustAck[protocol.SendAck](t, u2.sendText("group", "g1", "cmid-q2103", "will be deleted"))
	m4 := mustAck[protocol.SendAck](t, sendQuote(u1, "group", "g1", "cmid-q2104", "reply to m3", m3.MsgID))
	ver0 := convByID(t, u1, "group:g1").ModVer
	mustRecall(t, u2, "group:g1", m1.MsgID)
	raw := syncRaw(t, u1, "group:g1")
	if bytes.Contains(raw, []byte("ZXQ-QUO-1")) {
		t.Errorf("sync still has the quoted text: %s", raw)
	}
	msgs := syncConv(t, u1, "group:g1").Messages
	if q := quoteOf(t, msgBySeq(t, msgs, m2.Seq)); q.State != "recalled" || q.Text != "" || q.MsgID != m1.MsgID || q.Seq != m1.Seq {
		t.Errorf("m2 quote = %+v", q)
	}
	ver1 := convByID(t, u1, "group:g1").ModVer
	e.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{m3.MsgID}, "reason": "violation"})
	msgs = syncConv(t, u1, "group:g1").Messages
	if q := quoteOf(t, msgBySeq(t, msgs, m4.Seq)); q.State != "deleted" || q.Text != "" {
		t.Errorf("m4 quote = %+v", q)
	}
	if ver2 := convByID(t, u1, "group:g1").ModVer; !(ver0 < ver1 && ver1 < ver2) {
		t.Errorf("modVer %d -> %d -> %d", ver0, ver1, ver2)
	}
}

// TC-QUO-02-04（AC-QUO-02-4）：historyVisible=false，D 入群后 u1 引用入群前的 m1：D 收到的 m2 带 quote.text（F-QUO-02 规则 6 的已知限制）；
// D 直接 sync 拉不到 m1。
func TestTC_QUO_02_04_HistoryHidden(t *testing.T) {
	e := newC4Env(t, nil)
	e.usersNamed("u1", "u2", "d")
	e.createGroup("g1", "u1", "u2")
	u1 := e.dial("u1", "web")
	m1 := mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "cmid-q2401", "said before d joined"))
	if code, b := e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"d"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add d: %d %s", code, b)
	}
	d := e.dial("d", "web")
	mustAck[protocol.SendAck](t, sendQuote(u1, "group", "g1", "cmid-q2402", "look at this", m1.MsgID))
	got := d.waitCmd(protocol.CmdMsg).msg(t)
	if q := quoteOf(t, got); q.State != "normal" || q.Text != "said before d joined" {
		t.Errorf("d quote = %+v", q)
	}
	for _, m := range syncConv(t, d, "group:g1").Messages {
		if m.MsgID == m1.MsgID {
			t.Errorf("d can sync m1: %+v", m)
		}
	}
	page := mustAck[protocol.SyncAck](t, d.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "beforeSeq": m1.Seq + 1, "limit": 10}))
	if len(page.Messages) != 0 {
		t.Errorf("d pages back to m1: %+v", page.Messages)
	}
}

// TC-QUO-01-03（AC-QUO-01-3）、TC-QUO-01-07（AC-QUO-01-7）、TC-QUO-02-07（AC-QUO-02-7）：
// image 带 quote 4000，文件仍是 ready；m3 引用 m2（m2 引用 m1）时 quote.text 是 m2 自己的文字；引用图片「[图片]」、文件「[文件] {文件名}」；
// quote.msgId 格式错 4000；150 个码点（含 4 字节字符）截成前 100 个码点；文件过期后 state=expired，没有 text 和文件名。
func TestTC_QUO_01_07_Summaries(t *testing.T) {
	e := newC4FileEnv(t, func(c *app.Config) { r := 1; c.Files.RetentionDays = &r })
	tok1 := e.token("u1")
	imgID := e.uploadImage(tok1, "pic.jpg", fakeJPEG(10, 10, 4096), smallThumb())
	docID := e.uploadFile(tok1, "plan.pdf", "application/pdf", bytes.Repeat([]byte("%PDF-1.4 "), 50))
	spareID := e.uploadImage(tok1, "spare.jpg", fakeJPEG(10, 10, 4096), smallThumb())
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	conv := protocol.C2CConvID("u1", "u2")

	// image 带 quote：4000，文件仍是 ready。
	f := sendBody(u1, "c2c", "u2", "cmid-q0301", map[string]any{"text": "x"})
	text := mustAck[protocol.SendAck](t, f)
	bad := u1.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u2", "clientMsgId": "cmid-q0302", "type": "image",
		"body": map[string]any{"image": map[string]any{"fileId": spareID}, "quote": map[string]any{"msgId": text.MsgID}}})
	mustErr(t, bad, protocol.CodeBadRequest)
	if st := e.file(spareID).State; st != "ready" {
		t.Errorf("spare file state = %s", st)
	}

	m1 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-q0701", "the first one"))
	m2 := mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-q0702", "second, quoting first", m1.MsgID))
	m3 := mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-q0703", "third", m2.MsgID))
	img := mustAck[protocol.SendAck](t, sendFile(u1, "c2c", "u2", "cmid-q0704", "image", imgID, nil))
	doc := mustAck[protocol.SendAck](t, sendFile(u1, "c2c", "u2", "cmid-q0705", "file", docID, nil))
	long := strings.Repeat("😀", 60) + strings.Repeat("字", 90) // 150 个码点，前 60 个是 4 字节字符
	m150 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-q0706", long))
	qi := mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-q0707", "nice picture", img.MsgID))
	qf := mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-q0708", "read it", doc.MsgID))
	ql := mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-q0709", "long", m150.MsgID))
	mustErr(t, sendQuote(u2, "c2c", "u1", "cmid-q0710", "bad", "../x"), protocol.CodeBadRequest)

	msgs := syncConv(t, u1, conv).Messages
	if q := quoteOf(t, msgBySeq(t, msgs, m3.Seq)); q.Text != "second, quoting first" || q.MsgID != m2.MsgID {
		t.Errorf("chain not expanded: %+v", q)
	}
	if q := quoteOf(t, msgBySeq(t, msgs, qi.Seq)); q.State != "normal" || q.Text != "[图片]" || q.Type != "image" {
		t.Errorf("image quote = %+v", q)
	}
	if q := quoteOf(t, msgBySeq(t, msgs, qf.Seq)); q.State != "normal" || q.Text != "[文件] plan.pdf" || q.Type != "file" {
		t.Errorf("file quote = %+v", q)
	}
	want := string([]rune(long)[:100])
	if q := quoteOf(t, msgBySeq(t, msgs, ql.Seq)); q.Text != want || len([]rune(q.Text)) != 100 {
		t.Errorf("long quote = %d code points", len([]rune(q.Text)))
	}
	// 过期：推进到保留期之后跑 file_purge。
	e.advanceTo(e.now.Add(48 * time.Hour))
	e.runPurge()
	u1 = e.dial("u1", "web")
	raw := syncRaw(t, u1, conv)
	if bytes.Contains(raw, []byte("plan.pdf")) {
		t.Errorf("expired file name still in sync: %s", raw)
	}
	msgs = syncConv(t, u1, conv).Messages
	if q := quoteOf(t, msgBySeq(t, msgs, qf.Seq)); q.State != "expired" || q.Text != "" {
		t.Errorf("expired file quote = %+v", q)
	}
	if q := quoteOf(t, msgBySeq(t, msgs, qi.Seq)); q.State != "expired" || q.Text != "" {
		t.Errorf("expired image quote = %+v", q)
	}
}
