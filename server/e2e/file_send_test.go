package e2e

// BE-434：send 的 image、file 和判定顺序（TC-FILE-02、03-01、06-03、06-06、06-07、07-08）。

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-FILE-02-01（AC-FILE-02-1）后端部分、TC-FILE-02-02（AC-FILE-02-2）、TC-FILE-02-11（AC-FILE-02-11）send 部分：
// u1 给 u2 发 2 MiB 的 jpeg（客户端伪造 size=1、mime=text/html、width=1）；u2 收到 type=image，下发的是服务端记录的值；
// sync 同样；先取缩略图再取原图，两边 size、mime 一致；lastMsg 是 type=image、text=""。
func TestTC_FILE_02_01_SendImage(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2 := e.token("u1"), e.token("u2")
	orig := fakeJPEG(1600, 1200, 2<<20)
	id := e.uploadImage(tok1, "photo.jpg", orig, smallThumb())
	c1, c2 := e.dial("u1", "web"), e.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, sendFile(c1, "c2c", "u2", "img-0000001", "image", id, map[string]any{"size": 1, "mime": "text/html", "width": 1, "height": 1}))
	m := c2.waitCmd(protocol.CmdMsg).msg(t)
	want := protocol.ImageBody{FileID: id, Width: 1600, Height: 1200, Size: int64(len(orig)), Mime: "image/jpeg"}
	if m.Type != "image" || m.Body.Image == nil || *m.Body.Image != want || m.Body.Text != "" {
		t.Fatalf("msg = %+v body %+v", m, m.Body.Image)
	}
	msgs := syncAll(c2, ack.ConvID)
	if len(msgs) != 1 || *msgs[0].Body.Image != want {
		t.Fatalf("sync = %+v", msgs)
	}
	thumb := e.mustURL(tok2, id, "thumb")
	big := e.mustURL(tok2, id, "original")
	_, th, tb := download(t, thumb.URL)
	_, oh, ob := download(t, big.URL)
	if !bytes.Equal(tb, smallThumb()) || th.Get("Content-Type") != "image/jpeg" || !bytes.Equal(ob, orig) || oh.Get("Content-Type") != want.Mime ||
		int64(len(ob)) != want.Size {
		t.Fatal("downloaded content or type differs")
	}
	convs := mustAck[protocol.ConvsAck](t, c2.call(protocol.CmdConvs, map[string]any{}))
	if len(convs.Convs) != 1 || convs.Convs[0].LastMsg == nil || convs.Convs[0].LastMsg.Type != "image" || convs.Convs[0].LastMsg.Text != "" {
		t.Fatalf("lastMsg = %+v", convs.Convs[0].LastMsg)
	}
	if f := e.file(id); f.State != "attached" || f.MsgID != ack.MsgID || f.ConvID != ack.ConvID || f.RetainUntil == 0 {
		t.Fatalf("file after send = %+v", f)
	}
}

// TC-FILE-03-01（AC-FILE-03-1）后端部分：u1 在群 g1 发 5 MiB 的 pdf，成员 u2 下载：文件名和大小正确，SHA-256 相同；
// lastMsg 不含文件名（F-FILE-03 规则 5）；下载是 attachment，Content-Type 是服务端记录的值。
func TestTC_FILE_03_01_GroupFile(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2 := e.token("u1"), e.token("u2")
	e.createGroup("g1", "u1", "u2")
	data := pad([]byte("%PDF-1.7\n"), 5<<20)
	id := e.uploadFile(tok1, "季度报告-SECRETNAME.pdf", "application/pdf", data)
	c1, c2 := e.dial("u1", "web"), e.dial("u2", "web")
	mustAck[protocol.SendAck](t, sendFile(c1, "group", "g1", "file-0000001", "file", id, nil))
	m := c2.waitCmd(protocol.CmdMsg).msg(t)
	if m.Body.File == nil || m.Body.File.Name != "季度报告-SECRETNAME.pdf" || m.Body.File.Size != int64(len(data)) || m.Body.File.Mime != "application/pdf" {
		t.Fatalf("file body = %+v", m.Body.File)
	}
	u := e.mustURL(tok2, id, "original")
	if d := time.Until(time.UnixMilli(u.ExpiresAt)); d > 300*time.Second || d < 290*time.Second {
		t.Errorf("file url expires in %v, want 300s", d)
	}
	code, h, body := download(t, u.URL)
	if code != 200 || sha256.Sum256(body) != sha256.Sum256(data) {
		t.Fatalf("download = %d, sha256 differs", code)
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") || h.Get("Content-Type") != "application/pdf" {
		t.Fatalf("headers = %v", h)
	}
	convs := mustAck[protocol.ConvsAck](t, c2.call(protocol.CmdConvs, map[string]any{}))
	raw, _ := json.Marshal(convs)
	if strings.Contains(string(raw), "SECRETNAME") {
		t.Fatalf("lastMsg contains the file name: %s", raw)
	}
}

// TC-FILE-02-03（AC-FILE-02-3）：fileId 是别人的、是 kind=file、未 complete、已绑定到另一条消息、缩略图未上传：
// 依次 4048、4000 fileKindMismatch、4096、4096、4096；消息都没保存。
func TestTC_FILE_02_03_BadFileIDs(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2 := e.token("u1"), e.token("u2")
	others := e.uploadImage(tok2, "a.png", fakePNG(10, 10, 1024), smallThumb())
	kindFile := e.uploadFile(tok1, "a.txt", "text/plain", []byte("hello"))
	pending := e.mustRequest(tok1, imageReq("p.png", 1024)).FileID
	bound := e.uploadImage(tok1, "b.png", fakePNG(10, 10, 1024), smallThumb())
	noThumb := e.mustRequest(tok1, imageReq("n.png", 1024))
	e.mustPut(noThumb.Put, fakePNG(10, 10, 1024))
	e.complete(tok1, noThumb.FileID)
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "bound-00001", "image", bound, nil))
	for i, tc := range []struct {
		id     string
		code   int
		reason string
	}{{others, 4048, ""}, {kindFile, 4000, protocol.ReasonFileKindMismatch}, {pending, 4096, ""}, {bound, 4096, ""}, {noThumb.FileID, 4096, ""}} {
		f := sendFile(c, "c2c", "u2", "bad-"+strings.Repeat("x", 6)+string(rune('a'+i)), "image", tc.id, nil)
		if f.Error == nil || f.Error.Code != tc.code || f.Error.Reason != tc.reason {
			t.Errorf("case %d: %+v, want %d %s", i, f.Error, tc.code, tc.reason)
		}
	}
	// 头像不能当图片发。
	if msgs := syncAll(c, protocol.C2CConvID("u1", "u2")); len(msgs) != 1 {
		t.Fatalf("saved %d messages, want only the first", len(msgs))
	}
}

// TC-FILE-02-15（AC-FILE-02-15）：type=image 同时带 body.text、body.data；type=text 带 body.data、body.image：都是 400 / 4000。
func TestTC_FILE_02_15_BodyExclusive(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	c := e.dial("u1", "web")
	for i, body := range []map[string]any{
		{"image": map[string]any{"fileId": id}, "text": "x"},
		{"image": map[string]any{"fileId": id}, "data": map[string]any{"k": 1}},
	} {
		f := c.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u2", "clientMsgId": "excl-img-0" + string(rune('0'+i)), "type": "image", "body": body})
		mustErr(t, f, protocol.CodeBadRequest)
	}
	for i, body := range []map[string]any{{"text": "hi", "data": map[string]any{}}, {"text": "hi", "image": map[string]any{"fileId": id}}} {
		f := c.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u2", "clientMsgId": "excl-txt-0" + string(rune('0'+i)), "type": "text", "body": body})
		mustErr(t, f, protocol.CodeBadRequest)
	}
	if st := e.file(id).State; st != "ready" {
		t.Errorf("file state = %s", st)
	}
}

// TC-FILE-06-03（AC-FILE-06-4）：同一 fileId 再发一条消息 4096；用原 clientMsgId 重发得到原 ack。
func TestTC_FILE_06_03_OneMessagePerFile(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	c := e.dial("u1", "web")
	first := mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "once-000001", "image", id, nil))
	mustErr(t, sendFile(c, "c2c", "u2", "once-000002", "image", id, nil), protocol.CodeFileNotReady)
	again := mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "once-000001", "image", id, nil))
	if again != first {
		t.Fatalf("resend = %+v, want %+v", again, first)
	}
}

// TC-FILE-06-06（AC-FILE-06-7）：ready 的文件被 file_purge 置为 purging 之后 send 绑定：4096。
func TestTC_FILE_06_06_PurgingCannotBind(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadFile(tok, "a.txt", "text/plain", []byte("hello"))
	e.s3.FailDeletes(100) // 删除失败：行停在 purging
	e.setClock(t0.Add(25 * time.Hour))
	e.runPurgeCode()
	if st := e.file(id).State; st != "purging" {
		t.Fatalf("state = %s", st)
	}
	tok = e.token("u1") // 推进时钟后重新签发
	c := e.dial("u1", "web")
	mustErr(t, sendFile(c, "c2c", "u2", "purge-00001", "file", id, nil), protocol.CodeFileNotReady)
}

// TC-FILE-06-07（AC-FILE-06-8）：同一 fileId，两个不同 clientMsgId 并发发送：恰好一个成功，另一个 4096，消息未保存。
func TestTC_FILE_06_07_ConcurrentBind(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	for round := 0; round < 5; round++ {
		id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
		ca, cb := e.dial("u1", "web"), e.dial("u1", "android")
		var wg sync.WaitGroup
		res := make([]frame, 2)
		for i, c := range []*client{ca, cb} {
			wg.Add(1)
			go func(i int, c *client) {
				defer wg.Done()
				res[i] = sendFile(c, "c2c", "u2", "race-"+string(rune('a'+round))+"-0000"+string(rune('0'+i)), "image", id, nil)
			}(i, c)
		}
		wg.Wait()
		ok, bad := 0, 0
		for _, f := range res {
			switch {
			case f.Error == nil:
				ok++
			case f.Error.Code == protocol.CodeFileNotReady:
				bad++
			}
		}
		if ok != 1 || bad != 1 {
			t.Fatalf("round %d: %d ok, %d 4096: %+v %+v", round, ok, bad, res[0].Error, res[1].Error)
		}
	}
	if msgs := syncAll(e.dial("u2", "web"), protocol.C2CConvID("u1", "u2")); len(msgs) != 5 {
		t.Fatalf("saved %d messages, want 5", len(msgs))
	}
}

// TC-FILE-07-08（AC-FILE-07-8）：kind=image 的 ready 文件，send 失败（4036，被对方拉黑）；取消拉黑后重发同 clientMsgId：文件仍是 ready，可以绑定。
func TestTC_FILE_07_08_ReadyAfterReject(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	c1, c2 := e.dial("u1", "web"), e.dial("u2", "web")
	mustAck[map[string]any](t, c2.call("block", map[string]any{"userId": "u1"}))
	mustErr(t, sendFile(c1, "c2c", "u2", "blocked-0001", "image", id, nil), protocol.CodeRejectedByPeer)
	if st := e.file(id).State; st != "ready" {
		t.Fatalf("state after 4036 = %s", st)
	}
	mustAck[map[string]any](t, c2.call("unblock", map[string]any{"userId": "u1"}))
	mustAck[protocol.SendAck](t, sendFile(c1, "c2c", "u2", "blocked-0001", "image", id, nil))
	if st := e.file(id).State; st != "attached" {
		t.Fatalf("state after resend = %s", st)
	}
}

// R-CB-20（PRD-chat 第 4.3 节）：第 6～10 步失败退还第 5 步预扣的 newPeerReject。新对象被拒配额只有 2 时，
// 附件预检失败（4048）不消耗它：配额 10，连续 12 次都是 4048（不退还的话第 11 次会变成 4292）。
func TestFileRejectRefundsNewPeerQuota(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.SocialDefaults = `{"newPeerRejectDailyLimit":10}` })
	e.token("u1")
	e.token("p1")
	c := e.dial("u1", "web")
	for i := 0; i < 12; i++ {
		mustErr(t, sendFile(c, "c2c", "p1", "nofile-00"+string(rune('a'+i)), "image", "doesnotexist0001", nil), protocol.CodeFileNotFound)
	}
	mustAck[protocol.SendAck](t, c.sendText("c2c", "p1", "txt-to-p1-01", "hi"))
}

// TC-FILE-91-01（AC-ADM-30-7 的扩展，C2 起）、BE-434 第 4 条：/admin/convs/messages/query 的文件消息只有类型和大小，没有文件名；
// /admin/convs/messages/body（正文类授权）带文件名。这里验 im-server 的元数据接口。
func TestTC_FILE_91_01_MetaHasNoFileName(t *testing.T) {
	console := consoleCaller()
	e := newFileEnv(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, console.cfg) })
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadFile(tok, "FEATURE-NAME-7x9.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "meta-000001", "file", id, nil))
	code, b := e.signed(console, "POST", "/admin/convs/messages/query", map[string]any{"convId": protocol.C2CConvID("u1", "u2")}, signOpts{requestID: "req-meta-file"})
	if code != http.StatusOK || !strings.Contains(string(b), `"type":"file"`) || strings.Contains(string(b), "FEATURE-NAME") {
		t.Fatalf("meta = %d %s", code, b)
	}
}
