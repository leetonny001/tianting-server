package e2e

// BE-431、BE-432：开关、申请上传、配额、取消、文件名清洗、类型和大小（TC-FILE-01、02-16、03-02、05，TC-ACC-01-20、06-04）。

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/files"
	"tianting/server/internal/objstore/objstoretest"
	"tianting/server/internal/protocol"
)

func imageReq(name string, size int) map[string]any {
	return map[string]any{"kind": "image", "name": name, "size": size, "mime": "image/png", "thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}}
}

func fileReq(name string, size int64, mime string) map[string]any {
	return map[string]any{"kind": "file", "name": name, "size": size, "mime": mime}
}

// TC-FILE-01-14（AC-FILE-01-14）、TC-ACC-07-04 的 C2 部分（AC-ACC-07-4）：开关关闭时 /v1/files/* 404，send 的 image 4000；
// 开关打开后发的文件消息，关闭后照常 sync（同一个库重启）。
func TestTC_FILE_01_14_SwitchOff(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok, "a.png", fakePNG(64, 48, 2048), smallThumb())
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "msg-on-0001", "image", id, nil))
	code, b := e.get("/v1/config", "")
	var cfg map[string]any
	_ = json.Unmarshal(b, &cfg)
	if code != 200 || cfg["features"].(map[string]any)["files"] != true || cfg["limits"].(map[string]any)["imageMaxBytes"] == nil {
		t.Fatalf("config with files on: %s", b)
	}

	off := newEnvCfg(t, func(c *app.Config) { c.DBPath = e.dbPath })
	for _, p := range []string{"/v1/files/uploads", "/v1/files/" + id + "/complete", "/v1/files/urls"} {
		req, _ := http.NewRequest(http.MethodPost, off.srv.URL+p, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+off.token("u1"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s with files off = %d, want 404", p, resp.StatusCode)
		}
	}
	c2 := off.dial("u1", "web")
	mustErr(t, sendFile(c2, "c2c", "u2", "msg-off-0001", "image", id, nil), protocol.CodeBadRequest)
	mustErr(t, sendFile(c2, "c2c", "u2", "msg-off-0002", "file", id, nil), protocol.CodeBadRequest)
	msgs := syncAll(off.dial("u2", "web"), protocol.C2CConvID("u1", "u2"))
	if len(msgs) != 1 || msgs[0].Type != "image" || msgs[0].Body.Image == nil || msgs[0].Body.Image.FileID != id {
		t.Fatalf("existing image message not synced with files off: %+v", msgs)
	}
	code, b = off.do(off.srv.URL, http.MethodGet, "/v1/config", nil, "")
	_ = json.Unmarshal(b, &cfg)
	if code != 200 || cfg["features"].(map[string]any)["files"] != false {
		t.Fatalf("config with files off: %s", b)
	}
}

// TC-FILE-01-06（AC-FILE-01-6）：未登录 401 / 4010；别人的 fileId complete 404 / 4048。
func TestTC_FILE_01_06_AuthAndOwner(t *testing.T) {
	e := newFileEnv(t, nil)
	code, b := e.call(http.MethodPost, "/v1/files/uploads", "", fileReq("a.pdf", 10, "application/pdf"))
	wantErr(t, "no token", code, b, 401, protocol.CodeTokenInvalid, "")
	r := e.mustRequest(e.token("u1"), fileReq("a.pdf", 10, "application/pdf"))
	e.mustPut(r.Put, []byte("%PDF-1.4\n1"))
	code, b = e.complete(e.token("u2"), r.FileID)
	wantErr(t, "other's fileId", code, b, 404, protocol.CodeFileNotFound, "")
	code, b = e.complete("", r.FileID)
	wantErr(t, "complete without token", code, b, 401, protocol.CodeTokenInvalid, "")
}

// TC-FILE-01-07（AC-FILE-01-7）：文件名清洗；查数据库。
func TestTC_FILE_01_07_NameSanitize(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	for name, want := range map[string]string{"../../etc/pass\u202Ewd.": "etcpasswd", "a\nb": "ab", "\x01\x02\x7f\x1b": "file"} {
		r := e.mustRequest(tok, fileReq(name, 10, "text/plain"))
		if got := e.file(r.FileID).Name; got != want {
			t.Errorf("%q stored as %q, want %q", name, got, want)
		}
	}
	long := strings.Repeat("a", 252) + ".txt" // 256 字节
	code, _, b := e.requestUpload(tok, fileReq(long, 10, "text/plain"))
	wantErr(t, "256 bytes", code, b, 400, protocol.CodeBadRequest, protocol.ReasonFileNameTooLong)
	ok := strings.Repeat("a", 251) + ".txt" // 255 字节
	if code, _, b := e.requestUpload(tok, fileReq(ok, 10, "text/plain")); code != 200 {
		t.Errorf("255 bytes: %d %s", code, b)
	}
}

// TC-FILE-05-01（AC-FILE-05-1）申请部分：a.exe、a.EXE、a.pdf.exe 申请时 415；TC-FILE-05-02（AC-FILE-05-2）：配置新增 xyz，重启后生效；先规范化再判断。
func TestTC_FILE_05_01_02_Blocklist(t *testing.T) {
	path := t.TempDir() + "/blocklist.txt"
	if err := writeFile(path, "# 运维加的\nxyz\n"); err != nil {
		t.Fatal(err)
	}
	e := newFileEnv(t, func(c *app.Config) { c.Files.BlocklistPath = path })
	tok := e.token("u1")
	for _, name := range []string{"a.exe", "a.EXE", "a.pdf.exe", "a.xyz", "a.html", "b.svg", "photo\u202Egpj.html", "a.exe.", "a.exe::$DATA", "x.docm", "y.svgz"} {
		code, _, b := e.requestUpload(tok, fileReq(name, 10, "application/octet-stream"))
		wantErr(t, name, code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	}
	for _, name := range []string{"a.pdf", "a.doc", "a.zip", "README"} {
		if code, _, b := e.requestUpload(tok, fileReq(name, 10, "application/octet-stream")); code != 200 {
			t.Errorf("%s: %d %s", name, code, b)
		}
	}
	// 没有配置文件的环境：xyz 可以。
	e2 := newFileEnv(t, nil)
	if code, _, b := e2.requestUpload(e2.token("u1"), fileReq("a.xyz", 10, "application/octet-stream")); code != 200 {
		t.Errorf("a.xyz without the config: %d %s", code, b)
	}
}

// TC-FILE-02-16（AC-FILE-02-16）：kind=image 的 svg、bmp、tiff 申请时 415。TC-ACC-06-04（AC-ACC-06-4）：头像 svg 415、3 MiB 413。
// TC-FILE-01-13（AC-FILE-01-13）第一步：kind=image 不带 thumb 400。
func TestTC_FILE_02_16_ImageWhitelist(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	for _, m := range []string{"image/svg+xml", "image/bmp", "image/tiff"} {
		code, _, b := e.requestUpload(tok, map[string]any{"kind": "image", "name": "a", "size": 100, "mime": m, "thumb": map[string]any{"size": 10, "mime": "image/jpeg"}})
		wantErr(t, m, code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	}
	code, _, b := e.requestUpload(tok, map[string]any{"kind": "image", "name": "a.png", "size": 100, "mime": "image/png"})
	wantErr(t, "image without thumb", code, b, 400, protocol.CodeBadRequest, "")
	code, _, b = e.requestUpload(tok, map[string]any{"kind": "avatar", "name": "a.svg", "size": 100, "mime": "image/svg+xml"})
	wantErr(t, "avatar svg", code, b, 415, protocol.CodeFileTypeNotAllowed, "")
	code, _, b = e.requestUpload(tok, map[string]any{"kind": "avatar", "name": "a.png", "size": 3 * files.MiB, "mime": "image/png"})
	wantErr(t, "avatar 3 MiB", code, b, 413, protocol.CodeFileTooLarge, "")
	if pe := errOf(b); pe.Details["maxBytes"] != float64(2*files.MiB) {
		t.Errorf("4139 details = %v", pe.Details)
	}
}

// TC-FILE-03-02（AC-FILE-03-2）：101 MiB 文件、21 MiB 图片 413，边界 100 MiB、20 MiB 通过。
func TestTC_FILE_03_02_SizeLimits(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	code, _, b := e.requestUpload(tok, fileReq("a.bin", 101*files.MiB, "application/octet-stream"))
	wantErr(t, "101 MiB file", code, b, 413, protocol.CodeFileTooLarge, "")
	code, _, b = e.requestUpload(tok, imageReq("a.png", 21*files.MiB))
	wantErr(t, "21 MiB image", code, b, 413, protocol.CodeFileTooLarge, "")
	if code, _, b := e.requestUpload(tok, fileReq("a.bin", 100*files.MiB, "application/octet-stream")); code != 200 {
		t.Errorf("100 MiB file: %d %s", code, b)
	}
	if code, _, b := e.requestUpload(tok, imageReq("a.png", 20*files.MiB)); code != 200 {
		t.Errorf("20 MiB image: %d %s", code, b)
	}
}

// TC-FILE-01-04（AC-FILE-01-4）[IM 钟]：当天已用满额度，申请 1 字节 429 / 4292；次日成功。
func TestTC_FILE_01_04_DailyQuota(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.Files.DailyBytes = 3000 })
	t0 := e.freezeDay()
	tok := e.token("u1")
	e.uploadFile(tok, "a.txt", "text/plain", []byte(strings.Repeat("x", 3000)))
	code, _, b := e.requestUpload(tok, fileReq("b.txt", 1, "text/plain"))
	wantErr(t, "quota used", code, b, 429, protocol.CodeQuotaExceeded, "")
	e.setClock(t0.Add(24 * time.Hour))
	if code, _, b := e.requestUpload(tok, fileReq("b.txt", 1, "text/plain")); code != 200 {
		t.Fatalf("next day: %d %s", code, b)
	}
}

// TC-ACC-01-20（AC-ACC-01-20）[IM 钟]：open 模式注册 3 天的账号申请超过 200 MiB 的当日上传 429；8 天的成功。
// 单个文件上限是 100 MiB，这里把 IM_FILE_MAX_BYTES 调大，用一次申请直接越过 200 MiB（额度判定是"已用 + 声明"）。
func TestTC_ACC_01_20_NewAccountQuota(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) {
		c.Account.RegisterMode = account.ModeOpen
		c.Files.FileMaxBytes = 300 * files.MiB
	})
	t0 := e.freezeDay()
	e.register("olduser1", nil)
	e.setClock(t0.Add(5 * 24 * time.Hour))
	e.register("younguser1", nil)
	e.setClock(t0.Add(8 * 24 * time.Hour)) // old 注册 8 天，young 注册 3 天；access token 只有 24 小时，重新登录
	old, young := e.mustLogin("olduser1", goodPassword, testPlatform), e.mustLogin("younguser1", goodPassword, testPlatform)
	code, _, b := e.requestUpload(young.AccessToken, fileReq("a.bin", 201*files.MiB, "application/octet-stream"))
	wantErr(t, "3-day account", code, b, 429, protocol.CodeQuotaExceeded, "")
	if code, _, b := e.requestUpload(old.AccessToken, fileReq("a.bin", 201*files.MiB, "application/octet-stream")); code != 200 {
		t.Fatalf("8-day account: %d %s", code, b)
	}
}

// TC-FILE-01-11（AC-FILE-01-11）[真实钟]：放弃的上传不扣额度；边申请边取消直到频率第 61 次 429；不取消、15 分钟内未完成的第 21 个 429 uploadPendingLimit。
func TestTC_FILE_01_11_RateAndPending(t *testing.T) {
	e := newFileEnv(t, nil)
	e.freezeDay() // 频率的令牌桶跟 [IM 钟]：冻结之后不会在用例执行期间补充
	tok := e.token("u1")
	for i := 0; i < 3; i++ {
		r := e.mustRequest(tok, fileReq("a.txt", 1000, "text/plain"))
		e.mustPut(r.Put, []byte(strings.Repeat("x", 1000))) // 放弃：不 complete
	}
	if used, _ := e.st.UsedUploadBytes(context.Background(), "u1", shanghaiMidnight(t).Format("2006-01-02")); used != 0 {
		t.Fatalf("abandoned uploads charged %d bytes", used)
	}
	// 频率：已经用了 3 次，再 57 次（边申请边取消，不占未完成名额）到 60 次，第 61 次 429。
	for i := 3; i < 60; i++ {
		r := e.mustRequest(tok, fileReq("a.txt", 10, "text/plain"))
		if code, b := e.call(http.MethodDelete, "/v1/files/uploads/"+r.FileID, tok, nil); code != 200 {
			t.Fatalf("cancel: %d %s", code, b)
		}
	}
	code, _, b := e.requestUpload(tok, fileReq("a.txt", 10, "text/plain"))
	wantErr(t, "61st request", code, b, 429, protocol.CodeRateLimited, "")

	// 未完成数：另一个用户，20 个不取消的 pending，第 21 个 uploadPendingLimit。
	tok2 := e.token("u2")
	for i := 0; i < 20; i++ {
		e.mustRequest(tok2, fileReq("a.txt", 10, "text/plain"))
	}
	code, _, b = e.requestUpload(tok2, fileReq("a.txt", 10, "text/plain"))
	wantErr(t, "21st pending", code, b, 429, protocol.CodeRateLimited, protocol.ReasonUploadPendingLimit)
}

// TC-FILE-01-16（AC-FILE-01-16）[真实钟]：20 个未完成上传，等 PUT 地址过期（调短到 1 秒）再申请成功；或取消其中 1 个再申请成功。
func TestTC_FILE_01_16_PendingFreed(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.Files.PutTTL = time.Second })
	tok, tok2 := e.token("u1"), e.token("u2")
	var first string
	for i := 0; i < 20; i++ {
		r := e.mustRequest(tok, fileReq("a.txt", 10, "text/plain"))
		e.mustRequest(tok2, fileReq("a.txt", 10, "text/plain"))
		if i == 0 {
			first = r.FileID
		}
	}
	code, _, b := e.requestUpload(tok, fileReq("a.txt", 10, "text/plain"))
	wantErr(t, "21st", code, b, 429, protocol.CodeRateLimited, protocol.ReasonUploadPendingLimit)
	if code, b := e.call(http.MethodDelete, "/v1/files/uploads/"+first, tok, nil); code != 200 {
		t.Fatalf("cancel: %d %s", code, b)
	}
	if code, _, b := e.requestUpload(tok, fileReq("a.txt", 10, "text/plain")); code != 200 {
		t.Fatalf("after cancel: %d %s", code, b)
	}
	time.Sleep(1200 * time.Millisecond)
	if code, _, b := e.requestUpload(tok2, fileReq("a.txt", 10, "text/plain")); code != 200 {
		t.Fatalf("after the PUT urls expired: %d %s", code, b)
	}
}

// TC-FILE-01-29（AC-FILE-01-29）：反复申请 100 MiB 再取消，声明合计超过每日额度的 4 倍后 429 / 4292。
func TestTC_FILE_01_29_DeclaredLimit(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.Files.DailyBytes = 200 * files.MiB }) // 4 倍 = 800 MiB
	tok := e.token("u1")
	for i := 0; i < 8; i++ {
		r := e.mustRequest(tok, fileReq("a.bin", 100*files.MiB, "application/octet-stream"))
		if code, b := e.call(http.MethodDelete, "/v1/files/uploads/"+r.FileID, tok, nil); code != 200 {
			t.Fatalf("cancel: %d %s", code, b)
		}
	}
	code, _, b := e.requestUpload(tok, fileReq("a.bin", 100*files.MiB, "application/octet-stream"))
	wantErr(t, "declared > 4x", code, b, 429, protocol.CodeQuotaExceeded, "")
}

// TC-FILE-01-23（AC-FILE-01-23）：ready、attached 的文件；别人的 pending；不存在的 fileId：DELETE 响应都相同，状态和对象不变。
func TestTC_FILE_01_23_CancelHidesExistence(t *testing.T) {
	e := newFileEnv(t, nil)
	tok, tok2 := e.token("u1"), e.token("u2")
	ready := e.uploadFile(tok, "a.txt", "text/plain", []byte("hello"))
	attached := e.uploadFile(tok, "b.txt", "text/plain", []byte("hello"))
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "msg-att-0001", "file", attached, nil))
	other := e.mustRequest(tok2, fileReq("c.txt", 5, "text/plain"))
	e.mustPut(other.Put, []byte("hello"))
	keysBefore := e.s3.Keys(e.s3.Private, "")
	var bodies []string
	for _, id := range []string{ready, attached, other.FileID, "doesnotexist0000"} {
		code, b := e.call(http.MethodDelete, "/v1/files/uploads/"+id, tok, nil)
		bodies = append(bodies, string(b))
		if code != 200 || string(b) != bodies[0] {
			t.Errorf("DELETE %s = %d %s, want the same 200 as the first", id, code, b)
		}
	}
	for id, want := range map[string]string{ready: "ready", attached: "attached", other.FileID: "pending"} {
		if got := e.file(id).State; got != want {
			t.Errorf("%s state = %s, want %s", id, got, want)
		}
	}
	if after := e.s3.Keys(e.s3.Private, ""); strings.Join(after, ",") != strings.Join(keysBefore, ",") {
		t.Errorf("objects changed: %v → %v", keysBefore, after)
	}
}

// TC-FILE-01-28（AC-FILE-01-28）：取消后再 complete 404 / 4048；取消和 complete 并发只有一个生效，状态和对象一致。
func TestTC_FILE_01_28_CancelAndComplete(t *testing.T) {
	e := newFileEnv(t, nil)
	tok := e.token("u1")
	r := e.mustRequest(tok, fileReq("a.txt", 5, "text/plain"))
	e.mustPut(r.Put, []byte("hello"))
	e.call(http.MethodDelete, "/v1/files/uploads/"+r.FileID, tok, nil)
	code, b := e.complete(tok, r.FileID)
	wantErr(t, "complete after cancel", code, b, 404, protocol.CodeFileNotFound, "")

	// 并发：complete 停在 Copy 之后、提交之前，此时取消；放行后 complete 失败并删除自己的键，状态 purging，没有最终对象。
	r2 := e.mustRequest(tok, fileReq("b.txt", 5, "text/plain"))
	e.mustPut(r2.Put, []byte("hello"))
	p := e.app.Files().Hooks().Arm(files.PointAfterCopy, r2.FileID)
	done := make(chan struct{})
	var cc int
	var cb []byte
	go func() { cc, cb = e.complete(tok, r2.FileID); close(done) }()
	<-p.Reached()
	if code, _ := e.call(http.MethodDelete, "/v1/files/uploads/"+r2.FileID, tok, nil); code != 200 {
		t.Fatal("cancel failed")
	}
	p.Release()
	<-done
	wantErr(t, "complete that lost to cancel", cc, cb, 404, protocol.CodeFileNotFound, "")
	if st := e.file(r2.FileID).State; st != "purging" {
		t.Errorf("state = %s, want purging", st)
	}
	if keys := e.s3.Keys(e.s3.Private, "final/"+r2.FileID+"/"); len(keys) != 0 {
		t.Errorf("final keys left: %v", keys)
	}
}

// TC-FILE-05-06（AC-FILE-05-6）：文件表 scanState=skipped。
func TestTC_FILE_05_06_ScanState(t *testing.T) {
	e := newFileEnv(t, nil)
	id := e.uploadFile(e.token("u1"), "a.txt", "text/plain", []byte("hello"))
	if s := e.file(id).ScanState; s != "skipped" {
		t.Fatalf("scan_state = %q", s)
	}
}

// TC-FILE-01-05（AC-FILE-01-5）：对象存储不可用（拒绝连接、黑洞）：complete 503 / 5032；申请上传成功；WS 文本收发不受影响。
func TestTC_FILE_01_05_StoreDown(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.ObjStore.Timeout = 300 * time.Millisecond })
	tok := e.token("u1")
	e.token("u2")
	r := e.mustRequest(tok, fileReq("a.txt", 5, "text/plain"))
	e.mustPut(r.Put, []byte("hello"))
	c1, c2 := e.dial("u1", "web"), e.dial("u2", "web")
	for _, mode := range []string{objstoretest.DownRefuse, objstoretest.DownBlack} {
		e.s3.SetDown(mode)
		code, b := e.complete(tok, r.FileID)
		wantErr(t, mode+" complete", code, b, 503, protocol.CodeObjectStoreUnavailable, "")
		if code, _, b := e.requestUpload(tok, fileReq("b.txt", 5, "text/plain")); code != 200 {
			t.Errorf("%s: request upload = %d %s", mode, code, b)
		}
		mustAck[protocol.SendAck](t, c1.sendText("c2c", "u2", "txt-"+mode+"-0001", "hi"))
		if m := c2.waitCmd(protocol.CmdMsg).msg(t); m.Body.Text != "hi" {
			t.Errorf("%s: text = %+v", mode, m)
		}
	}
	e.s3.SetDown(objstoretest.UpNormal)
	e.mustComplete(tok, r.FileID) // 可重试
}

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o600) }

// TC-CHAT-90-12（AC-CHAT-90-12）开发机参考：≥ 1000 次申请上传的耗时。只记录，**不算通过**（未校准，发布判定等 P-12 目标机）。
// 频率和未完成数调大，只测申请本身（只签名，不访问存储）。
func TestTC_CHAT_90_12_UploadRequestLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("perf reference")
	}
	e := newFileEnv(t, func(c *app.Config) {
		c.Files.UploadRate, c.Files.PendingMax, c.Files.DailyDeclared = 100000, 100000, 1<<50
	})
	tok := e.token("u1")
	lat := make([]time.Duration, 0, 1000)
	for i := 0; i < 1000; i++ {
		start := time.Now()
		if code, _, b := e.requestUpload(tok, fileReq("a.txt", 10, "text/plain")); code != 200 {
			t.Fatalf("request %d: %d %s", i, code, b)
		}
		lat = append(lat, time.Since(start))
	}
	slices.Sort(lat)
	t.Logf("PERF upload request n=%d p50=%v p99=%v (dev machine, sequential, reference only)", len(lat), lat[len(lat)/2], lat[len(lat)*99/100])
}
