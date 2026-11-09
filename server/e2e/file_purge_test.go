package e2e

// BE-436：生命周期、file_purge 和 mod_ver（TC-FILE-06、01-24）。

import (
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/files"
	"tianting/server/internal/protocol"
)

// TC-FILE-06-01（AC-FILE-06-1）[IM 钟]：ready 但未绑定的上传、pending 的上传，推进 25 小时运行 file_purge：对象和记录都被删除。
func TestTC_FILE_06_01_Orphans(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	ready := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	pend := e.mustRequest(tok, fileReq("b.txt", 5, "text/plain"))
	e.mustPut(pend.Put, []byte("hello"))
	e.setClock(t0.Add(23 * time.Hour))
	e.runPurge()
	if e.fileGone(ready) || e.fileGone(pend.FileID) {
		t.Fatal("purged before 24 hours")
	}
	e.setClock(t0.Add(25 * time.Hour))
	e.runPurge()
	if !e.fileGone(ready) || !e.fileGone(pend.FileID) {
		t.Fatal("orphans not purged")
	}
	if keys := e.s3.Keys(e.s3.Private, ""); len(keys) != 0 {
		t.Fatalf("objects left: %v", keys)
	}
}

// TC-FILE-06-02（AC-FILE-06-2）[IM 钟]：已绑定的图片推进 181 天运行清理：对象不存在；sync 消息还在，带 expired=true；申请地址 4048。
// TC-FILE-06-09（AC-FILE-06-10）：过期后 files.name 为空。TC-FILE-06-11（AC-FILE-06-12）的服务端部分：过期时会话 modVer 递增，其他会话不变。
func TestTC_FILE_06_02_Expiry(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	e.token("u2")
	e.token("u3")
	img := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	fil := e.uploadFile(tok, "SECRET-name.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 1024))
	c := e.dial("u1", "web")
	ack := mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "exp-0000001", "image", img, nil))
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "exp-0000002", "file", fil, nil))
	mustAck[protocol.SendAck](t, c.sendText("c2c", "u3", "exp-txt-0001", "hi"))
	modVer := func(convID string) int64 {
		t.Helper()
		convs := mustAck[protocol.ConvsAck](t, c.call(protocol.CmdConvs, map[string]any{}))
		for _, cv := range convs.Convs {
			if cv.ConvID == convID {
				return cv.ModVer
			}
		}
		t.Fatalf("no conv %s", convID)
		return 0
	}
	if modVer(ack.ConvID) != 0 {
		t.Fatal("modVer before expiry")
	}
	e.setClock(t0.Add(179 * 24 * time.Hour))
	e.runPurge()
	if st := e.file(img).State; st != "attached" {
		t.Fatalf("expired too early: %s", st)
	}
	e.setClock(t0.Add(181 * 24 * time.Hour))
	e.runPurge()
	for _, id := range []string{img, fil} {
		f := e.file(id)
		if f.State != "expired" || f.Name != "" || !f.ObjectsDeleted {
			t.Fatalf("%s after expiry: %+v", id, f)
		}
		if keys := e.s3.Keys(e.s3.Private, "final/"+id+"/"); len(keys) != 0 {
			t.Fatalf("objects left: %v", keys)
		}
	}
	tok2 := e.token("u2")
	c2 := e.dial("u2", "web")
	msgs := syncAll(c2, ack.ConvID)
	if len(msgs) != 2 || msgs[0].Body.Image == nil || !msgs[0].Body.Image.Expired || msgs[1].Body.File == nil || !msgs[1].Body.File.Expired || msgs[1].Body.File.Name != "" {
		t.Fatalf("synced after expiry: %+v / %+v", msgs[0].Body.Image, msgs[1].Body.File)
	}
	e.denied(tok2, img, "original")
	e.denied(tok2, fil, "original")
	c = e.dial("u1", "web")
	if v := modVer(ack.ConvID); v < 1 {
		t.Fatalf("modVer = %d after expiry", v)
	}
	if v := modVer(protocol.C2CConvID("u1", "u3")); v != 0 {
		t.Fatalf("unrelated conv modVer = %d", v)
	}
}

// TC-FILE-06-04（AC-FILE-06-5）：对象存储删除接口返回错误：记录保留，im_job_runs_total{job="file_purge",result="error"} 增加，下次重试成功。
func TestTC_FILE_06_04_DeleteFailure(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	id := e.uploadFile(e.token("u1"), "a.txt", "text/plain", []byte("hello"))
	e.setClock(t0.Add(25 * time.Hour))
	e.s3.FailDeletes(1000)
	if code := e.runPurgeCode(); code == 200 {
		t.Fatal("purge with a failing store reported success")
	}
	if e.fileGone(id) {
		t.Fatal("row deleted although the object delete failed")
	}
	if v := e.metric(`im_job_runs_total{job="file_purge",result="error"}`); v != 1 {
		t.Fatalf("error metric = %v", v)
	}
	e.s3.FailDeletes(0)
	e.runPurge()
	if !e.fileGone(id) || len(e.s3.Keys(e.s3.Private, "")) != 0 {
		t.Fatal("retry did not purge")
	}
}

// TC-FILE-06-08（AC-FILE-06-9）[IM 钟]：IM_FILE_RETENTION_DAYS=0，推进 400 天运行清理：文件不被删除。
func TestTC_FILE_06_08_NoRetention(t *testing.T) {
	zero := 0
	e := newFileEnv(t, func(c *app.Config) { c.Files.RetentionDays = &zero })
	t0 := e.freezeDay()
	tok := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok, "a.png", fakePNG(10, 10, 1024), smallThumb())
	mustAck[protocol.SendAck](t, sendFile(e.dial("u1", "web"), "c2c", "u2", "keep-000001", "image", id, nil))
	e.setClock(t0.Add(400 * 24 * time.Hour))
	e.runPurge()
	if f := e.file(id); f.State != "attached" || len(e.s3.Keys(e.s3.Private, "final/"+id+"/")) != 2 {
		t.Fatalf("file deleted with retention 0: %+v", f)
	}
}

// TC-FILE-06-12（AC-FILE-06-13）[IM 钟]：被后台删除的图片：第 29 天对象还在且用户申请 4048；第 31 天运行清理后对象被删除。
func TestTC_FILE_06_12_AdminDeletedPurge(t *testing.T) {
	var mod testCaller
	e := newFileEnv(t, func(c *app.Config) { mod = fileModCaller(c) })
	t0 := e.freezeDay()
	tok1 := e.token("u1")
	e.token("u2")
	id := e.uploadImage(tok1, "a.png", fakePNG(10, 10, 1024), smallThumb())
	ack := mustAck[protocol.SendAck](t, sendFile(e.dial("u1", "web"), "c2c", "u2", "adm-0000001", "image", id, nil))
	e.adminDelete(mod, ack.ConvID, ack.MsgID, t0)
	e.setClock(t0.Add(29 * 24 * time.Hour))
	e.runPurge()
	if keys := e.s3.Keys(e.s3.Private, "final/"+id+"/"); len(keys) != 2 {
		t.Fatalf("objects deleted before 30 days: %v", keys)
	}
	e.denied(e.token("u2"), id, "original")
	e.setClock(t0.Add(31 * 24 * time.Hour))
	e.runPurge()
	if keys := e.s3.Keys(e.s3.Private, "final/"+id+"/"); len(keys) != 0 {
		t.Fatalf("objects left after 31 days: %v", keys)
	}
}

// TC-FILE-01-24（AC-FILE-01-24）[IM 钟]：complete 停在 Copy 之后、提交之前，file_purge 把这条 pending 改成 purging（租约过期）：
// 放行后提交失败，它写的随机键被删除，没有永久孤儿对象。
func TestTC_FILE_01_24_PurgeBeatsComplete(t *testing.T) {
	e := newFileEnv(t, nil)
	t0 := e.freezeDay()
	tok := e.token("u1")
	r := e.mustRequest(tok, fileReq("a.txt", 5, "text/plain"))
	e.mustPut(r.Put, []byte("hello"))
	p := e.app.Files().Hooks().Arm(files.PointAfterCopy, r.FileID)
	done := make(chan int)
	go func() { c, _ := e.complete(tok, r.FileID); done <- c }()
	<-p.Reached()
	e.s3.FailDeletes(1000) // purge 删不掉对象：行停在 purging，确定性地观察 complete 的提交
	e.setClock(t0.Add(25 * time.Hour))
	e.runPurgeCode()
	if st := e.file(r.FileID).State; st != "purging" {
		t.Fatalf("state = %s", st)
	}
	e.s3.FailDeletes(0)
	p.Release()
	if code := <-done; code != 404 {
		t.Fatalf("complete after purge = %d, want 404", code)
	}
	if keys := e.s3.Keys(e.s3.Private, "final/"+r.FileID+"/"); len(keys) != 0 {
		t.Fatalf("orphan keys: %v", keys)
	}
	e.runPurge()
	if !e.fileGone(r.FileID) || len(e.s3.Keys(e.s3.Private, "")) != 0 {
		t.Fatal("not cleaned up")
	}
}
