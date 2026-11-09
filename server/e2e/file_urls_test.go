package e2e

// BE-435：批量取地址和访问控制（TC-FILE-04、01-25、05-05）。

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// fileModCaller 打开第二批管理功能（后台删除），返回有 msg.delete 的调用方。
func fileModCaller(c *app.Config) testCaller {
	mod := hmacCaller("mod", adminauth.ScopeMsgDelete, adminauth.ScopeMsgMeta)
	c.V13AdminFeatures, c.QuarantineKey = true, testQuarantineKey
	c.AdminCallers = append(c.AdminCallers, mod.cfg)
	return mod
}

func (e *fileEnv) adminDelete(mod testCaller, convID, msgID string, at time.Time) {
	e.t.Helper()
	code, b := e.signed(mod, http.MethodPost, "/admin/messages/delete", map[string]any{"convId": convID, "msgIds": []string{msgID}, "reason": "violation"},
		signOpts{ts: at, operator: "op1", requestID: fmt.Sprintf("del-%d", at.UnixNano())})
	if code != http.StatusOK {
		e.t.Fatalf("admin delete: %d %s", code, b)
	}
}

// TC-FILE-04-01（AC-FILE-04-1）：单聊 u1、u2 的图片，u3 申请地址：HTTP 200，这一项 code=4048，和"不存在的 fileId"那一项完全相同。
// TC-FILE-04-08（AC-FILE-04-8）：ready 未绑定：上传者成功，其他人 4048。
func TestTC_FILE_04_01_08_Access(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2, tok3 := e.token("u1"), e.token("u2"), e.token("u3")
	id := e.uploadImage(tok1, "a.png", fakePNG(10, 10, 1024), smallThumb())
	e.mustURL(tok1, id, "original") // 绑定前上传者可以访问
	e.denied(tok2, id, "original")
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "acc-0000001", "image", id, nil))
	e.mustURL(tok2, id, "thumb")
	code, items, _ := e.urls(tok3, [2]string{id, "original"}, [2]string{"doesnotexist00000", "original"})
	if code != 200 || len(items) != 2 {
		t.Fatalf("u3: %d %+v", code, items)
	}
	a, _ := urlItemKey(items[0])
	b, _ := urlItemKey(urlItem{FileID: id, Variant: "original", Code: 4048})
	if items[0].Code != 4048 || a != b || items[1].Code != 4048 || items[1].URL != "" {
		t.Fatalf("u3 items = %+v", items)
	}
	// 文件没有缩略图：thumb 也是 4048。
	fid := e.uploadFile(tok1, "a.txt", "text/plain", []byte("hello"))
	e.denied(tok1, fid, "thumb")
}

func urlItemKey(v urlItem) (string, error) {
	return fmt.Sprintf("%d|%s|%d", v.Code, v.URL, v.ExpiresAt), nil
}

// TC-FILE-04-02（AC-FILE-04-2）：群 g1 的文件，成员申请成功；移出群后 4048。
// TC-FILE-06-10（AC-FILE-06-11）：u1 退群时其他人照常；解散后 4048；对象都还在。
func TestTC_FILE_04_02_GroupMembership(t *testing.T) {
	console := consoleCaller()
	e := newFileEnv(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, console.cfg) })
	tok1, tok2, tok3 := e.token("u1"), e.token("u2"), e.token("u3")
	e.createGroup("g1", "u1", "u2", "u3")
	id := e.uploadFile(tok1, "a.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 1024))
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "group", "g1", "grp-0000001", "file", id, nil))
	e.mustURL(tok2, id, "original")
	e.removeMembers("g1", "u2")
	e.denied(tok2, id, "original")
	// u1（发送者）退群：其他成员照常。
	e.removeMembers("g1", "u1")
	e.mustURL(tok3, id, "original")
	code, b := e.signed(console, http.MethodDelete, "/admin/groups/g1", nil, signOpts{operator: "op1", requestID: "dissolve-g1"})
	if code/100 != 2 {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	e.denied(tok3, id, "original")
	if keys := e.s3.Keys(e.s3.Private, "final/"+id+"/"); len(keys) != 1 {
		t.Fatalf("objects after dissolve: %v", keys)
	}
}

// TC-FILE-04-03（AC-FILE-04-3）：historyVisible=false，D 入群前的图片：4048；改为 true 后成功。
func TestTC_FILE_04_03_HistoryVisible(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tokD := e.token("u1"), e.token("dd")
	e.users("u2")
	code, b := e.createGroupWith(map[string]any{"groupId": "g1", "name": "g1", "members": []string{"u1", "u2"}, "historyVisible": false})
	e.mustOK(code, b)
	id := e.uploadImage(tok1, "a.png", fakePNG(10, 10, 1024), smallThumb())
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "group", "g1", "hist-000001", "image", id, nil))
	e.addMembers("g1", "dd")
	e.denied(tokD, id, "original")
	e.patchGroup("g1", map[string]any{"historyVisible": true})
	e.mustURL(tokD, id, "original")
}

// TC-FILE-04-04（AC-FILE-04-4）[真实钟]：IM_FILE_URL_TTL_IMAGE=5，取地址后等 6 秒 GET 被对象存储拒绝；默认配置的 expiresAt：图片 +120 秒，文件 +300 秒。
func TestTC_FILE_04_04_URLExpiry(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.Files.URLTTLImage = 5 * time.Second })
	tok := e.token("u1")
	id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	u := e.mustURL(tok, id, "original")
	if code, _, _ := download(t, u.URL); code != 200 {
		t.Fatalf("fresh url = %d", code)
	}
	time.Sleep(6 * time.Second)
	if code, _, _ := download(t, u.URL); code != http.StatusForbidden {
		t.Fatalf("6s later = %d, want rejected", code)
	}
	d := newFileEnv(t, nil)
	tok = d.token("u1")
	img := d.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	fil := d.uploadFile(tok, "a.txt", "text/plain", []byte("hello"))
	now := time.Now()
	for id, want := range map[string]time.Duration{img: 120 * time.Second, fil: 300 * time.Second} {
		got := time.UnixMilli(d.mustURL(tok, id, "original").ExpiresAt).Sub(now)
		if got < want-2*time.Second || got > want+time.Second {
			t.Errorf("expiresAt = now + %v, want + %v", got, want)
		}
	}
	tu := d.mustURL(tok, img, "thumb")
	if got := time.UnixMilli(tu.ExpiresAt).Sub(now); got > 121*time.Second {
		t.Errorf("thumb expiresAt = now + %v", got)
	}
}

// TC-FILE-04-05（AC-FILE-04-5）：图片消息被后台删除，参与者申请地址：4048。
func TestTC_FILE_04_05_AdminDeleted(t *testing.T) {
	var mod testCaller
	e := newFileEnv(t, func(c *app.Config) { mod = fileModCaller(c) })
	tok1, tok2 := e.token("u1"), e.token("u2")
	id := e.uploadImage(tok1, "a.png", fakePNG(10, 10, 1024), smallThumb())
	c := e.dial("u1", "web")
	ack := mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "del-0000001", "image", id, nil))
	e.mustURL(tok2, id, "original")
	e.adminDelete(mod, ack.ConvID, ack.MsgID, time.Now())
	e.denied(tok2, id, "original")
	e.denied(tok1, id, "thumb")
	if f := e.file(id); f.State != "revoked" || f.Name != "" {
		t.Fatalf("file after admin delete = %s %q", f.State, f.Name)
	}
}

// TC-FILE-04-09（AC-FILE-04-9）：一批 50 个，其中 1 个无权：49 个有地址，1 个 code=4048，HTTP 200；51 个 400 / 4131。
func TestTC_FILE_04_09_Batch(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2 := e.token("u1"), e.token("u2")
	var items [][2]string
	for i := 0; i < 49; i++ {
		id := e.uploadFile(tok1, "a.txt", "text/plain", []byte("hello"))
		items = append(items, [2]string{id, "original"})
	}
	items = append(items, [2]string{e.uploadFile(tok2, "b.txt", "text/plain", []byte("hello")), "original"})
	code, res, b := e.urls(tok1, items...)
	if code != 200 || len(res) != 50 {
		t.Fatalf("50 items: %d %s", code, b)
	}
	ok, denied := 0, 0
	for i, r := range res {
		if r.FileID != items[i][0] {
			t.Fatalf("item %d order differs", i)
		}
		if r.URL != "" {
			ok++
		} else if r.Code == 4048 {
			denied++
		}
	}
	if ok != 49 || denied != 1 || res[49].Code != 4048 {
		t.Fatalf("ok=%d denied=%d", ok, denied)
	}
	code, _, b = e.urls(tok1, append(items, items[0])...)
	wantErr(t, "51 items", code, b, 400, protocol.CodeBatchLimitExceeded, "")
}

// F-FILE-04 规则 6：每人每分钟 120 批，超出 4290。
func TestFileURLRateLimit(t *testing.T) {
	e := newFileEnv(t, nil)
	e.freezeDay()
	tok := e.token("u1")
	id := e.uploadFile(tok, "a.txt", "text/plain", []byte("hello"))
	for i := 0; i < 120; i++ {
		e.mustURL(tok, id, "original")
	}
	code, _, b := e.urls(tok, [2]string{id, "original"})
	wantErr(t, "121st batch", code, b, 429, protocol.CodeRateLimited, "")
}

// TC-FILE-01-25（AC-FILE-01-25）：文件名 `evil.exe"; x=".txt`：response-content-disposition 用 filename* 百分号编码，ASCII 回退名不含 `"` `;` `%`。
func TestTC_FILE_01_25_Disposition(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	id := e.uploadFile(tok, `evil.exe"; x=".txt`, "text/plain", []byte("hello"))
	u := e.mustURL(tok, id, "original")
	pu, _ := url.Parse(u.URL)
	disp := pu.Query().Get("response-content-disposition")
	_, h, _ := download(t, u.URL)
	if h.Get("Content-Disposition") != disp {
		t.Fatalf("response header %q, signed %q", h.Get("Content-Disposition"), disp)
	}
	fallback := disp[strings.Index(disp, `filename="`)+10 : strings.Index(disp, `";`)]
	if !strings.HasPrefix(disp, "attachment;") || strings.ContainsAny(fallback, `";%`) || !strings.Contains(disp, "filename*=UTF-8''evil.exe%22%3B%20x%3D%22.txt") {
		t.Fatalf("disposition = %s", disp)
	}
	if !strings.HasSuffix(disp, ".txt") {
		t.Fatalf("the saved name would not end with .txt: %s", disp)
	}
}

// TC-FILE-05-05（AC-FILE-05-5）：开头是合法 png、后面是 HTML 的文件（kind=image），接收方取原图地址下载：Content-Type: image/png，不是 text/html。
func TestTC_FILE_05_05_PolyglotServedAsImage(t *testing.T) {
	e := newFileEnv(t, nil)
	tok1, tok2 := e.token("u1"), e.token("u2")
	poly := append(fakePNG(100, 100, 100), []byte("<html><script>alert(1)</script></html>")...)
	r := e.mustRequest(tok1, map[string]any{"kind": "image", "name": "a.png", "size": len(poly), "mime": "image/png", "thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
	put(t, r.Put, poly, map[string]string{})
	e.mustPut(*r.ThumbPut, smallThumb())
	e.mustComplete(tok1, r.FileID)
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "poly-000001", "image", r.FileID, nil))
	_, h, body := download(t, e.mustURL(tok2, r.FileID, "original").URL)
	if h.Get("Content-Type") != "image/png" || h.Get("Content-Disposition") != "inline" || !bytes.Equal(body, poly) {
		t.Fatalf("headers = %v", h)
	}
}

// TC-FILE-04-07（AC-FILE-04-7）、AC-CHAT-90-5 的上传和下载部分：申请上传、PUT、complete、发送、申请地址、下载之后，
// 扫描 im-server 日志（DEBUG 级别）：搜不到预签名查询串和文件名特征串。
func TestTC_FILE_04_07_NoSecretsInLogs(t *testing.T) {
	var mu sync.Mutex
	var logs bytes.Buffer
	w := writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return logs.Write(p) })
	e := newFileEnv(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
		c.ObjStore.Timeout = 300 * time.Millisecond
	})
	tok1, tok2 := e.token("u1"), e.token("u2")
	const marker = "FNAMEMARK-93kq"
	id := e.uploadFile(tok1, marker+".pdf", "application/pdf", pad([]byte("%PDF-1.4"), 1024))
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "log-0000001", "file", id, nil))
	u := e.mustURL(tok2, id, "original")
	download(t, u.URL)
	// 失败路径也会写日志：对象存储不可用、嗅探拒绝。
	r := e.mustRequest(tok1, fileReq(marker+"2.pdf", 1024, "application/pdf"))
	e.mustPut(r.Put, pad([]byte("MZ"), 1024))
	e.s3.SetDown("refuse")
	e.complete(tok1, r.FileID)
	e.s3.SetDown("")
	e.complete(tok1, r.FileID)
	pu, _ := url.Parse(u.URL)
	mu.Lock()
	out := logs.String()
	mu.Unlock()
	for _, s := range []string{marker, pu.Query().Get("X-Amz-Signature"), "X-Amz-Signature", "X-Amz-Credential", e.s3.SecretKey, pu.RawQuery} {
		if s != "" && strings.Contains(out, s) {
			t.Errorf("log contains %q", s)
		}
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

var _ io.Writer = writerFunc(nil)
