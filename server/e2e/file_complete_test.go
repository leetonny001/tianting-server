package e2e

// BE-433：complete 的租约、嗅探、条件 Copy（TC-FILE-01、02-11、05-01、05-04）。F-FILE-01 的攻击序列（替换对象、并发、崩溃）也在这里。

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/files"
	"tianting/server/internal/protocol"
)

// TC-FILE-01-01（AC-FILE-01-1）：1 MiB 的 png 和缩略图：ready，最终键形如 final/{fileId}/{attemptId}/original，不含文件名和 userId，
// 和 PUT 的键不同，临时键已不存在；响应是服务端读到的真实值；PUT 地址签入 Content-Length 和 Content-Type。
func TestTC_FILE_01_01_HappyPath(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("alice_u1")
	orig := fakePNG(1280, 960, 1<<20)
	r := e.mustRequest(tok, map[string]any{"kind": "image", "name": "secret-name.png", "size": len(orig), "mime": "image/png",
		"thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
	pu, _ := url.Parse(r.Put.URL)
	if !strings.HasPrefix(pu.Path, "/private/tmp/") || !strings.Contains(pu.Query().Get("X-Amz-SignedHeaders"), "content-length") ||
		!strings.Contains(pu.Query().Get("X-Amz-SignedHeaders"), "content-type") || r.Put.Headers["Content-Type"] != "image/png" {
		t.Fatalf("put = %+v", r.Put)
	}
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, smallThumb())
	got := e.mustComplete(tok, r.FileID)
	if got.Size != int64(len(orig)) || got.Mime != "image/png" || got.Width != 1280 || got.Height != 960 {
		t.Fatalf("complete = %+v", got)
	}
	f := e.file(r.FileID)
	if f.State != "ready" || !strings.HasPrefix(f.ObjectKey, "final/"+r.FileID+"/") || !strings.HasSuffix(f.ObjectKey, "/original") ||
		!strings.HasSuffix(f.ThumbKey, "/thumb") || f.ObjectKey == f.TmpKey {
		t.Fatalf("file = %+v", f)
	}
	for _, k := range []string{f.ObjectKey, f.ThumbKey} {
		if strings.Contains(k, "secret") || strings.Contains(k, "alice") {
			t.Errorf("final key %s contains the name or userId", k)
		}
	}
	if _, ok := e.s3.Get(e.s3.Private, f.TmpKey); ok {
		t.Error("tmp key still exists")
	}
	if _, ok := e.s3.Get(e.s3.Private, f.TmpThumbKey); ok {
		t.Error("tmp thumb key still exists")
	}
	if o, ok := e.s3.Get(e.s3.Private, f.ObjectKey); !ok || !bytes.Equal(o.Body, orig) || o.ContentType != "image/png" {
		t.Error("final object differs")
	}
	// 幂等：已 ready 再 complete 返回已有记录。
	if again := e.mustComplete(tok, r.FileID); again != got {
		t.Errorf("second complete = %+v", again)
	}
}

// TC-FILE-01-02（AC-FILE-01-2）：声明 1 MiB，存储里实际 2 MiB（假 S3 测试接口直接写入）：400 uploadMismatch，对象被删，没有扣额度。
func TestTC_FILE_01_02_SizeMismatch(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	r := e.mustRequest(tok, fileReq("a.bin", 1<<20, "application/octet-stream"))
	tmp := e.file(r.FileID).TmpKey
	e.s3.Put(e.s3.Private, tmp, bytes.Repeat([]byte{1}, 2<<20), "application/octet-stream")
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "size mismatch", code, b, 400, protocol.CodeBadRequest, protocol.ReasonUploadMismatch)
	if _, ok := e.s3.Get(e.s3.Private, tmp); ok {
		t.Error("tmp object not deleted")
	}
	if used, _ := e.st.UsedUploadBytes(t.Context(), "u1", shanghaiMidnight(t).Format("2006-01-02")); used != 0 {
		t.Errorf("charged %d", used)
	}
}

// TC-FILE-01-03（AC-FILE-01-3）：声明 png，内容是 HTML：415，对象被删。TC-FILE-05-01（AC-FILE-05-1）第 4 个：扩展名 .pdf 但内容是 PE：415 且对象被删。
// TC-FILE-05-04（AC-FILE-05-4）：认不出真实类型的 kind=file 通过，mime=application/octet-stream；同样内容 kind=image 415。
func TestTC_FILE_01_03_SniffRejects(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	html := pad([]byte("<!DOCTYPE html><html><script>alert(1)</script>"), 2048)
	r := e.mustRequest(tok, imageReq("a.png", len(html)))
	e.mustPut(r.Put, html)
	e.mustPut(*r.ThumbPut, smallThumb())
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "html as png", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	if _, ok := e.s3.Get(e.s3.Private, e.file(r.FileID).TmpKey); ok {
		t.Error("html object not deleted")
	}

	pe := pad([]byte("MZ\x90\x00\x03\x00\x00\x00PE"), 4096)
	r = e.mustRequest(tok, fileReq("report.pdf", int64(len(pe)), "application/pdf"))
	e.mustPut(r.Put, pe)
	code, b = e.complete(tok, r.FileID)
	wantErr(t, "PE as pdf", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	if _, ok := e.s3.Get(e.s3.Private, e.file(r.FileID).TmpKey); ok {
		t.Error("PE object not deleted")
	}

	unknown := pad([]byte{0x01, 0x02, 0x03, 0x04, 0xF0}, 4096)
	id := e.uploadFile(tok, "data.dat", "application/x-custom", unknown)
	if f := e.file(id); f.Mime != "application/octet-stream" {
		t.Errorf("unknown kind=file mime = %s", f.Mime)
	}
	r = e.mustRequest(tok, map[string]any{"kind": "image", "name": "x.png", "size": len(unknown), "mime": "image/png", "thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
	e.mustPut(r.Put, unknown)
	e.mustPut(*r.ThumbPut, smallThumb())
	code, b = e.complete(tok, r.FileID)
	wantErr(t, "unknown as image", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
}

// TC-FILE-01-13（AC-FILE-01-13）：kind=image 不带 thumb 400（见 file_upload_test）；只 PUT 了原图 complete 4096；缩略图是 HTML 415。
func TestTC_FILE_01_13_ThumbErrors(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	orig := fakePNG(100, 100, 2048)
	r := e.mustRequest(tok, imageReq("a.png", len(orig)))
	e.mustPut(r.Put, orig)
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "missing thumb", code, b, 409, protocol.CodeFileNotReady, "")
	if st := e.file(r.FileID).State; st != "pending" {
		t.Errorf("state = %s, want pending", st)
	}
	html := pad([]byte("<html><body>x</body></html>"), 4096)
	e.mustPut(*r.ThumbPut, html)
	code, b = e.complete(tok, r.FileID)
	wantErr(t, "html thumb", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
}

// TC-FILE-01-22（AC-FILE-01-22）：缩略图 6000×6000、头像 6000×6000、头部解析不到尺寸的 JPEG：415。
// TC-FILE-02-11（AC-FILE-02-11）complete 部分：png 头部写 20000×20000：415。
func TestTC_FILE_01_22_Dimensions(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	try := func(label string, orig, thumb []byte, kind string) {
		t.Helper()
		body := map[string]any{"kind": kind, "name": "a", "size": len(orig), "mime": sniffMime(orig)}
		if thumb != nil {
			body["thumb"] = map[string]any{"size": len(thumb), "mime": sniffMime(thumb)}
		}
		r := e.mustRequest(tok, body)
		e.mustPut(r.Put, orig)
		if thumb != nil {
			e.mustPut(*r.ThumbPut, thumb)
		}
		code, b := e.complete(tok, r.FileID)
		wantErr(t, label, code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	}
	try("6000x6000 thumb", fakePNG(800, 600, 4096), fakePNG(6000, 6000, 4096), "image")
	try("6000x6000 avatar", fakePNG(6000, 6000, 4096), nil, "avatar")
	try("jpeg without size", fakeJPEG(0, 0, 4096), smallThumb(), "image")
	try("20000x20000 png", fakePNG(20000, 20000, 4096), smallThumb(), "image")
}

// TC-FILE-01-08（AC-FILE-01-8）：100 MiB 文件，PUT 目标是对象存储（假 S3，独立端口），假 S3 收到全部字节，im-server 对外端口收到的请求体都 ≤ 64 KB。
func TestTC_FILE_01_08_BytesBypassServer(t *testing.T) {
	var maxBody int64
	var mu sync.Mutex
	e := newFileEnv(t, nil)
	inner := e.srv.Config.Handler
	e.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		maxBody = max(maxBody, r.ContentLength)
		mu.Unlock()
		inner.ServeHTTP(w, r)
	})
	tok := e.token("u1")
	data := bytes.Repeat([]byte{0x00, 0x11, 0x22, 0x33}, 25<<20) // 100 MiB
	r := e.mustRequest(tok, fileReq("big.bin", int64(len(data)), "application/octet-stream"))
	pu, _ := url.Parse(r.Put.URL)
	if !strings.HasPrefix(e.s3.URL(), pu.Scheme+"://"+pu.Host) || strings.Contains(e.srv.URL, pu.Host) {
		t.Fatalf("PUT target %s is not the object store", pu.Host)
	}
	before := e.s3.PutBytes.Load()
	e.mustPut(r.Put, data)
	e.mustComplete(tok, r.FileID)
	if got := e.s3.PutBytes.Load() - before; got != int64(len(data)) {
		t.Errorf("object store received %d bytes", got)
	}
	if maxBody > 64<<10 {
		t.Errorf("im-server received a %d-byte body", maxBody)
	}
}

// TC-FILE-01-09（AC-FILE-01-9）：complete 成功后用原 PUT 地址再 PUT 恶意内容，接收方下载到的是 complete 时的内容；1 天后临时前缀被生命周期规则清除（假 S3 模拟）。
func TestTC_FILE_01_09_OverwriteAfterComplete(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	good := []byte("%PDF-1.4 good content")
	r := e.mustRequest(tok, fileReq("a.pdf", int64(len(good)), "application/pdf"))
	e.mustPut(r.Put, good)
	e.mustComplete(tok, r.FileID)
	evil := []byte("<html>evil content!!!")[:len(good)]
	if code := put(t, r.Put, evil, nil); code != http.StatusOK {
		t.Fatalf("re-PUT = %d", code)
	}
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "msg-ovr-0001", "file", r.FileID, nil))
	u := e.mustURL(e.token("u2"), r.FileID, "original")
	if code, _, body := download(t, u.URL); code != 200 || !bytes.Equal(body, good) {
		t.Fatalf("download = %d %q", code, body)
	}
	if n := e.s3.ExpireTmp(); n < 1 || len(e.s3.Keys(e.s3.Private, "tmp/")) != 0 {
		t.Fatalf("lifecycle removed %d, left %v", n, e.s3.Keys(e.s3.Private, "tmp/"))
	}
}

// TC-FILE-01-10（AC-FILE-01-10）：complete 顺序重复 2 次额度只扣 1 次；原图 Copy 成功、缩略图 Copy 失败后重试：
// 原图和缩略图都重新 Copy，上次的随机键被删除；全部成功后临时键才被删除。
func TestTC_FILE_01_10_RetryRedoesAll(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	orig, thumb := fakePNG(300, 200, 3000), smallThumb()
	r := e.mustRequest(tok, imageReq("a.png", len(orig)))
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, thumb)
	f := e.file(r.FileID)
	// 原图的 Copy 成功之后，让下一次 Copy（缩略图）失败。
	copies := 0
	e.s3.After(func(method, bucket, key string, _ url.Values) {
		if method == http.MethodPut && strings.HasPrefix(key, "final/") {
			copies++
			if copies == 1 {
				e.s3.FailCopies(1)
			}
		}
	})
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "thumb copy failed", code, b, 503, protocol.CodeObjectStoreUnavailable, "")
	if keys := e.s3.Keys(e.s3.Private, "final/"); len(keys) != 0 {
		t.Fatalf("failed attempt left %v", keys)
	}
	if _, ok := e.s3.Get(e.s3.Private, f.TmpKey); !ok {
		t.Fatal("tmp deleted before success")
	}
	e.s3.After(nil)
	e.mustComplete(tok, r.FileID)
	e.mustComplete(tok, r.FileID)
	f = e.file(r.FileID)
	keys := e.s3.Keys(e.s3.Private, "final/")
	if len(keys) != 2 || keys[0] != f.ObjectKey && keys[1] != f.ObjectKey {
		t.Fatalf("final keys = %v, record %s %s", keys, f.ObjectKey, f.ThumbKey)
	}
	if used, _ := e.st.UsedUploadBytes(t.Context(), "u1", e.dayOf(time.Now())); used != int64(len(orig)+len(thumb)) {
		t.Errorf("charged %d, want %d once", used, len(orig)+len(thumb))
	}
	if len(e.s3.Keys(e.s3.Private, "tmp/")) != 0 {
		t.Error("tmp keys left after success")
	}
}

func (e *fileEnv) dayOf(ts time.Time) string {
	return ts.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02")
}

// TC-FILE-01-15（AC-FILE-01-15）：假 S3 在 HEAD 之后、嗅探之前替换对象；在嗅探之后、Copy 之前替换对象：都失败（内部重试 1 次），
// 最终键没有被替换的内容，临时对象没有被删。
func TestTC_FILE_01_15_ReplaceDuringVerify(t *testing.T) {
	for _, after := range []string{"HEAD", "GET"} {
		t.Run("after "+after, func(t *testing.T) {
			e := newFileEnv(t, nil)
			tok := e.token("u1")
			good := pad([]byte("%PDF-1.4 "), 4096)
			r := e.mustRequest(tok, fileReq("a.pdf", int64(len(good)), "application/pdf"))
			e.mustPut(r.Put, good)
			tmp := e.file(r.FileID).TmpKey
			// 攻击者换上去的内容能通过嗅探（pdf 头加 HTML），只能靠 ETag 条件发现"校验的不是 Copy 的"。
			evil := pad([]byte("%PDF-1.4 <html><script>x</script>"), 4096)
			n := 0
			e.s3.After(func(method, bucket, key string, _ url.Values) {
				if method == after && key == tmp {
					n++
					// 每次校验都被替换成不同的内容（攻击者持续 PUT 换对象），ETag 条件永远不满足。
					e.s3.Put(e.s3.Private, tmp, append(evil[:len(evil)-1:len(evil)-1], byte('0'+n)), "text/html")
				}
			})
			code, b := e.complete(tok, r.FileID)
			wantErr(t, "replaced", code, b, 400, protocol.CodeBadRequest, protocol.ReasonUploadMismatch)
			if n != 2 {
				t.Errorf("replaced %d times, want 2 (one internal retry)", n)
			}
			if keys := e.s3.Keys(e.s3.Private, "final/"); len(keys) != 0 {
				t.Errorf("final keys: %v", keys)
			}
			if _, ok := e.s3.Get(e.s3.Private, tmp); !ok {
				t.Error("tmp object deleted")
			}
		})
	}
	// PRD-chat v1.3 AC-FILE-01-15 补充：换成嗅探通不过的内容（禁止的真实类型 PE）时按规则 3 ② 415 并删除临时对象。
	t.Run("replaced with content that fails the sniff", func(t *testing.T) {
		e := newFileEnv(t, nil)
		tok := e.token("u1")
		good := pad([]byte("%PDF-1.4 "), 4096)
		r := e.mustRequest(tok, fileReq("a.pdf", int64(len(good)), "application/pdf"))
		e.mustPut(r.Put, good)
		tmp := e.file(r.FileID).TmpKey
		pe := pad([]byte("MZ\x90\x00\x03\x00\x00\x00PE"), 4096)
		n := 0
		e.s3.After(func(method, bucket, key string, _ url.Values) {
			if method == "HEAD" && key == tmp && n == 0 {
				n++
				e.s3.Put(e.s3.Private, tmp, pe, "application/pdf")
			}
		})
		code, b := e.complete(tok, r.FileID)
		wantErr(t, "replaced with PE", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
		if n != 1 {
			t.Errorf("replaced %d times, want 1", n)
		}
		if keys := e.s3.Keys(e.s3.Private, "final/"); len(keys) != 0 {
			t.Errorf("final keys: %v", keys)
		}
		if _, ok := e.s3.Get(e.s3.Private, tmp); ok {
			t.Error("tmp object with the forbidden type not deleted")
		}
	})
}

// TC-FILE-01-18（AC-FILE-01-18）[IM 钟]：剩余 1 MiB，3 个 1 MiB 上传依次 complete：第 1 个成功，后面 429；
// 调大声明上限，23 小时时集中 complete 一批：超额不超过一个文件。
func TestTC_FILE_01_18_QuotaRecheck(t *testing.T) {
	const fileSize = 64 << 10
	e := newFileEnv(t, func(c *app.Config) {
		c.Files.DailyBytes = 10 * fileSize
		c.Files.DailyDeclared = 1000 * fileSize
		c.Files.PendingMax = 100
		c.Files.UploadRate = 1000
	})
	t0 := e.freezeDay()
	tok := e.token("u1")
	data := bytes.Repeat([]byte("x"), fileSize)
	for i := 0; i < 9; i++ {
		e.uploadFile(tok, "a.txt", "text/plain", data) // 剩 1 个文件的额度
	}
	var ids []string
	for i := 0; i < 3; i++ {
		r := e.mustRequest(tok, fileReq("b.txt", fileSize, "text/plain"))
		e.mustPut(r.Put, data)
		ids = append(ids, r.FileID)
	}
	e.mustComplete(tok, ids[0])
	for _, id := range ids[1:] {
		code, b := e.complete(tok, id)
		wantErr(t, "over quota", code, b, 429, protocol.CodeQuotaExceeded, "")
	}
	// 次日：每 15 分钟申请一批，23 小时时集中 complete。
	day2 := t0.Add(23 * time.Hour) // 次日 00:00，23 小时时是当天 23:00
	var batch []string
	for i := 0; i < 23*4; i++ {
		e.setClock(day2.Add(time.Duration(i) * 15 * time.Minute))
		r := e.mustRequest(tok, fileReq("c.txt", fileSize, "text/plain"))
		e.mustPut(r.Put, data)
		batch = append(batch, r.FileID)
		if len(batch) == 20 {
			break
		}
	}
	e.setClock(day2.Add(23 * time.Hour))
	// 集中 complete：全部尝试都过了 Copy 前的额度检查、停在提交之前，再一起放行，只有提交时的复查能挡住。
	var wg sync.WaitGroup
	var pauses []*files.Pause
	for _, id := range batch {
		p := e.app.Files().Hooks().Arm(files.PointAfterCopy, id)
		pauses = append(pauses, p)
		wg.Add(1)
		go func(id string) { defer wg.Done(); e.complete(tok, id) }(id)
		<-p.Reached()
	}
	for _, p := range pauses {
		p.Release()
	}
	wg.Wait()
	used, _ := e.st.UsedUploadBytes(t.Context(), "u1", e.dayOf(day2))
	if used > 10*fileSize+fileSize {
		t.Fatalf("used %d, more than one file over the quota %d", used, 10*fileSize)
	}
}

// TC-FILE-01-19（AC-FILE-01-19）：两个并发 complete，第一个校验后攻击者替换对象：第二个 503 completeInProgress；
// 记录的元数据和最终对象一致；没有 ready 但对象缺失的状态。
func TestTC_FILE_01_19_ConcurrentComplete(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	orig := fakePNG(400, 300, 4096)
	r := e.mustRequest(tok, imageReq("a.png", len(orig)))
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, smallThumb())
	p := e.app.Files().Hooks().Arm(files.PointAfterSniff, r.FileID)
	done := make(chan struct{})
	var c1 int
	var b1 []byte
	go func() { c1, b1 = e.complete(tok, r.FileID); close(done) }()
	<-p.Reached()
	e.s3.Put(e.s3.Private, e.file(r.FileID).TmpKey, fakeJPEG(10, 10, 4096), "image/jpeg") // 校验之后替换
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "second complete", code, b, 503, protocol.CodeObjectStoreUnavailable, protocol.ReasonCompleteInProgress)
	p.Release()
	<-done
	f := e.file(r.FileID)
	if c1 == 200 {
		o, ok := e.s3.Get(e.s3.Private, f.ObjectKey)
		if f.State != "ready" || !ok || !bytes.Equal(o.Body, orig) || f.Mime != "image/png" || f.Width != 400 {
			t.Fatalf("ready record does not match the object: %+v", f)
		}
	} else {
		// 条件 Copy 发现替换（412），重试时读到的是替换后的 jpeg，和声明的 png 矛盾：失败，没有最终对象，状态仍是 pending。
		if f.State != "pending" || len(e.s3.Keys(e.s3.Private, "final/")) != 0 || (c1 != 400 && c1 != 415) {
			t.Fatalf("first complete %d %s; state %s, final %v", c1, b1, f.State, e.s3.Keys(e.s3.Private, "final/"))
		}
	}
}

// TC-FILE-01-20（AC-FILE-01-20）：图片只传原图 complete 4096；替换临时原图（类型不同），再传缩略图，再 complete：最终内容、mime、宽高都来自第二次。
func TestTC_FILE_01_20_ContentFromLastAttempt(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	png := fakePNG(500, 400, 4096)
	r := e.mustRequest(tok, imageReq("a.png", len(png)))
	e.mustPut(r.Put, png)
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "missing thumb", code, b, 409, protocol.CodeFileNotReady, "")
	// 临时原图换成 jpeg（申请时声明的是 png）：类型和声明矛盾 → 415；最终键不会出现第一次的内容。
	jpg := fakeJPEG(640, 480, len(png))
	e.s3.Put(e.s3.Private, e.file(r.FileID).TmpKey, jpg, "image/jpeg")
	e.mustPut(*r.ThumbPut, smallThumb())
	code, b = e.complete(tok, r.FileID)
	wantErr(t, "type changed", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	if keys := e.s3.Keys(e.s3.Private, "final/"); len(keys) != 0 {
		t.Fatalf("final keys: %v", keys)
	}
	// 换成另一张 png（宽高不同）：最终内容、宽高来自第二次。
	png2 := fakePNG(900, 700, len(png))
	r2 := e.mustRequest(tok, imageReq("b.png", len(png)))
	e.mustPut(r2.Put, png)
	code, _ = e.complete(tok, r2.FileID)
	if code != 409 {
		t.Fatalf("first complete = %d", code)
	}
	e.s3.Put(e.s3.Private, e.file(r2.FileID).TmpKey, png2, "image/png")
	e.mustPut(*r2.ThumbPut, smallThumb())
	got := e.mustComplete(tok, r2.FileID)
	f := e.file(r2.FileID)
	o, _ := e.s3.Get(e.s3.Private, f.ObjectKey)
	if got.Width != 900 || got.Height != 700 || f.Mime != "image/png" || !bytes.Equal(o.Body, png2) {
		t.Fatalf("complete = %+v, object from the first PUT: %v", got, bytes.Equal(o.Body, png))
	}
}

// TC-FILE-01-21（AC-FILE-01-21）：PUT 带 Content-Type: text/html、Content-Encoding、Content-Disposition、x-amz-meta-*、x-amz-acl、
// x-amz-website-redirect-location、x-amz-tagging：未签入的被拒绝或丢弃；最终对象和下载响应只有服务端写入的 Content-Type。
func TestTC_FILE_01_21_MaliciousHeaders(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	orig := fakePNG(64, 64, 2048)
	r := e.mustRequest(tok, imageReq("a.png", len(orig)))
	for _, h := range [][2]string{{"Content-Type", "text/html"}, {"x-amz-meta-x", "1"}, {"x-amz-acl", "public-read"},
		{"x-amz-website-redirect-location", "/evil"}, {"x-amz-tagging", "a=b"}} {
		if code := put(t, r.Put, orig, map[string]string{h[0]: h[1]}); code == http.StatusOK {
			t.Errorf("PUT with %s accepted", h[0])
		}
	}
	// 没有签入、也不是 x-amz-* 的头：S3 接受并保存，complete 的 Copy 把它们丢掉。
	if code := put(t, r.Put, orig, map[string]string{"Content-Encoding": "gzip", "Content-Disposition": "inline; filename=x.html"}); code != 200 {
		t.Fatalf("PUT = %d", code)
	}
	e.mustPut(*r.ThumbPut, smallThumb())
	e.mustComplete(tok, r.FileID)
	f := e.file(r.FileID)
	o, _ := e.s3.Get(e.s3.Private, f.ObjectKey)
	if o.ContentType != "image/png" || o.ContentEncoding != "" || o.ContentDisposition != "" || len(o.Meta) != 0 {
		t.Fatalf("final object metadata: %+v", o)
	}
	u := e.mustURL(tok, r.FileID, "original")
	_, h, _ := download(t, u.URL)
	if h.Get("Content-Type") != "image/png" || h.Get("Content-Encoding") != "" || h.Get("Content-Disposition") != "inline" {
		t.Fatalf("download headers: %v", h)
	}
}

// TC-FILE-01-26（AC-FILE-01-26）[IM 钟]：Copy 之后、提交之前杀进程，留下 final/{fileId}/{attemptId}；重试 complete 成功，提交后残留被删除，
// 下载到的是新一次的内容；另一个文件杀进程后不再重试，推进 25 小时运行 file_purge，对象和记录都被删除。
func TestTC_FILE_01_26_CrashLeftovers(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	data := pad([]byte("%PDF-1.4 v1"), 2048)
	r := e.mustRequest(tok, fileReq("a.pdf", int64(len(data)), "application/pdf"))
	e.mustPut(r.Put, data)
	e.app.Files().Hooks().Crash(files.PointAfterCopy, r.FileID)
	if code, _ := e.complete(tok, r.FileID); code != 500 {
		t.Fatalf("crashed complete = %d", code)
	}
	left := e.s3.Keys(e.s3.Private, "final/"+r.FileID+"/")
	if len(left) != 1 {
		t.Fatalf("leftovers = %v", left)
	}
	// 新内容（同样大小），租约过期之后重试。
	data2 := pad([]byte("%PDF-1.4 v2"), 2048)
	e.s3.Put(e.s3.Private, e.file(r.FileID).TmpKey, data2, "application/pdf")
	e.setClock(t0.Add(61 * time.Second))
	e.mustComplete(tok, r.FileID)
	f := e.file(r.FileID)
	if keys := e.s3.Keys(e.s3.Private, "final/"+r.FileID+"/"); len(keys) != 1 || keys[0] != f.ObjectKey {
		t.Fatalf("after retry: %v (record %s)", keys, f.ObjectKey)
	}
	if o, _ := e.s3.Get(e.s3.Private, f.ObjectKey); !bytes.Equal(o.Body, data2) {
		t.Fatal("content is not from the retry")
	}

	r2 := e.mustRequest(tok, fileReq("b.pdf", int64(len(data)), "application/pdf"))
	e.mustPut(r2.Put, data)
	e.app.Files().Hooks().Crash(files.PointAfterCopy, r2.FileID)
	e.complete(tok, r2.FileID)
	if len(e.s3.Keys(e.s3.Private, "final/"+r2.FileID+"/")) != 1 {
		t.Fatal("no leftover for the second file")
	}
	e.setClock(t0.Add(25 * time.Hour))
	e.runPurge()
	if !e.fileGone(r2.FileID) || len(e.s3.Keys(e.s3.Private, "final/"+r2.FileID+"/")) != 0 {
		t.Fatalf("crashed upload not purged: gone=%v keys=%v", e.fileGone(r2.FileID), e.s3.Keys(e.s3.Private, "final/"+r2.FileID+"/"))
	}
}

// TC-FILE-01-27（AC-FILE-01-27）[IM 钟]：缺缩略图 4096 后立即补传再 complete：立即执行，不是 503；Q1 租约过期，Q2 成功并删除临时键，
// Q1 继续：重读状态后返回记录，不是 4096；两个尝试都已 Copy、同时提交：恰好一个成功，败者只删除自己的键。
func TestTC_FILE_01_27_LeaseAndRace(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	orig := fakePNG(320, 240, 4096)
	r := e.mustRequest(tok, imageReq("a.png", len(orig)))
	e.mustPut(r.Put, orig)
	if code, _ := e.complete(tok, r.FileID); code != 409 {
		t.Fatal("want 4096 first")
	}
	e.mustPut(*r.ThumbPut, smallThumb())
	e.mustComplete(tok, r.FileID) // 租约已经释放：不是 503

	// Q1 停在校验之后；租约过期；Q2 成功（删除临时键）；Q1 继续：Copy 找不到临时键 → 重读 → 返回记录。
	r = e.mustRequest(tok, imageReq("b.png", len(orig)))
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, smallThumb())
	p := e.app.Files().Hooks().Arm(files.PointAfterSniff, r.FileID)
	done := make(chan struct{})
	var c1 int
	var b1 []byte
	go func() { c1, b1 = e.complete(tok, r.FileID); close(done) }()
	<-p.Reached()
	e.setClock(t0.Add(2 * time.Minute))
	e.mustComplete(tok, r.FileID)
	p.Release()
	<-done
	if c1 != 200 {
		t.Fatalf("Q1 = %d %s, want the record", c1, b1)
	}

	// 两个尝试都已 Copy（都停在提交之前），同时提交：恰好一个成功，败者的键被删除，最终只剩胜者的两个键。
	r = e.mustRequest(tok, imageReq("c.png", len(orig)))
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, smallThumb())
	hooks := e.app.Files().Hooks()
	pa := hooks.Arm(files.PointAfterCopy, r.FileID)
	doneA := make(chan int)
	go func() { c, _ := e.complete(tok, r.FileID); doneA <- c }()
	<-pa.Reached()
	e.setClock(t0.Add(5 * time.Minute)) // A 的租约过期，B 拿到租约
	pb := hooks.Arm(files.PointAfterCopy, r.FileID)
	doneB := make(chan int)
	go func() { c, _ := e.complete(tok, r.FileID); doneB <- c }()
	<-pb.Reached()
	if n := len(e.s3.Keys(e.s3.Private, "final/"+r.FileID+"/")); n != 4 {
		t.Fatalf("both attempts should have copied: %d keys", n)
	}
	pa.Release()
	pb.Release()
	ca, cb := <-doneA, <-doneB
	if ca != 200 || cb != 200 {
		t.Fatalf("A=%d B=%d, both should return the record", ca, cb)
	}
	f := e.file(r.FileID)
	keys := e.s3.Keys(e.s3.Private, "final/"+r.FileID+"/")
	if len(keys) != 2 || (keys[0] != f.ObjectKey && keys[1] != f.ObjectKey) {
		t.Fatalf("keys after the race = %v, record %s", keys, f.ObjectKey)
	}
}
