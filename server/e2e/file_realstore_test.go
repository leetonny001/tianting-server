package e2e

// AC-FILE-90-1 的实现部分（BE-439 第 1 条）：把 BE-430 的对象存储客户端和 files 流程跑到**真实**的 S3 兼容存储上。
// 默认跳过：只有设置了 IM_E2E_S3_ENDPOINT 才运行，go test ./... 不受影响。凭证从环境变量读，不写进文件，也不打印。
//
//	IM_E2E_S3_ENDPOINT、IM_E2E_S3_REGION（默认 us-east-1）
//	IM_E2E_S3_BUCKET、IM_E2E_S3_PUBLIC_BUCKET、IM_E2E_S3_PUBLIC_BASE（可选，公开桶的独立域名）
//	IM_E2E_S3_ACCESS_KEY、IM_E2E_S3_SECRET_KEY：im-server 的最小权限凭证（deploy/objstore/policy-im-server.json）
//
// 和 tools/objstore-contract 的分工：合同工具直接验存储的能力；这里验 im-server 在这个存储上按 F-FILE-01、03、04 工作。
// 生命周期的实际清除和 CORS 限定来源这里不验（见 deploy/FILES-RUNBOOK.md 第 7 节）。

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/objstore"
	"tianting/server/internal/objstore/objstoretest"
)

func realStoreConfig(t *testing.T) objstore.Config {
	t.Helper()
	ep := os.Getenv("IM_E2E_S3_ENDPOINT")
	if ep == "" {
		t.Skip("set IM_E2E_S3_ENDPOINT (and the other IM_E2E_S3_* variables) to run against a real S3-compatible store (AC-FILE-90-1)")
	}
	c := objstore.Config{
		Endpoint: ep, Region: os.Getenv("IM_E2E_S3_REGION"), Bucket: os.Getenv("IM_E2E_S3_BUCKET"), PublicBucket: os.Getenv("IM_E2E_S3_PUBLIC_BUCKET"),
		PublicBaseURL: os.Getenv("IM_E2E_S3_PUBLIC_BASE"), AccessKey: os.Getenv("IM_E2E_S3_ACCESS_KEY"), SecretKey: os.Getenv("IM_E2E_S3_SECRET_KEY"),
		Timeout: 10 * time.Second,
	}
	if c.Bucket == "" || c.PublicBucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		t.Fatal("IM_E2E_S3_BUCKET, IM_E2E_S3_PUBLIC_BUCKET, IM_E2E_S3_ACCESS_KEY and IM_E2E_S3_SECRET_KEY are required")
	}
	return c
}

func newRealStoreEnv(t *testing.T, oc objstore.Config, mutate func(*app.Config)) *fileEnv {
	t.Helper()
	e := newAcctEnv(t, func(c *app.Config) {
		c.Files.Enabled = true
		c.ObjStore = oc
		c.Account.RegisterMode = account.ModeOpen
		if mutate != nil {
			mutate(c)
		}
	})
	return &fileEnv{acctEnv: e}
}

// TC-FILE-90-01 的 im-server 部分：在真实存储上走一遍上传、complete、下载、头像，以及 AC-FILE-90-1 列出的各项里 im-server 用到的能力。
func TestTC_FILE_90_01_RealStore(t *testing.T) {
	realStoreChecks(t, realStoreConfig(t))
}

// 同一套检查先在假 S3 上跑一遍，证明用例本身是对的（**不算 AC-FILE-90-1 通过**：假 S3 不证明厂商行为）。
func TestRealStoreChecksOnFake(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps for presigned expiry")
	}
	oc := objstoretest.New(t).Config()
	oc.Timeout = 2 * time.Second
	realStoreChecks(t, oc)
}

func realStoreChecks(t *testing.T, oc objstore.Config) {
	e := newRealStoreEnv(t, oc, func(c *app.Config) {
		c.Files.PutTTL = 5 * time.Second
		c.Files.URLTTLFile = 3 * time.Second
	})
	cli := objstore.New(oc)
	ctx := context.Background()
	t0 := e.freezeDay()
	tok1, tok2 := e.token("u1"), e.token("u2")
	anon := func(u string) int {
		t.Helper()
		resp, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	t.Run("预签名 PUT 到临时键、complete（HEAD、If-Match Range、条件 Copy、元数据替换）、下载", func(t *testing.T) {
		orig, thumb := fakePNG(10, 10, 64<<10+100), smallThumb()
		id := e.uploadImage(tok1, "a.png", orig, thumb)
		f := e.file(id)
		if !strings.HasPrefix(f.ObjectKey, "final/"+id+"/") || f.ObjectKey == f.TmpKey {
			t.Fatalf("keys: %q %q", f.ObjectKey, f.TmpKey)
		}
		if _, err := cli.Head(ctx, f.TmpKey); !errors.Is(err, objstore.ErrNotFound) {
			t.Errorf("tmp key after complete: %v (403 here means the vendor hides 404 without list permission, see FILES-RUNBOOK 第 7 节)", err)
		}
		c := e.dial("u1", "web")
		mustAck[struct{}](t, sendFile(c, "c2c", "u2", "real-000001", "image", id, nil))
		code, h, body := download(t, e.mustURL(tok2, id, "original").URL)
		if code != 200 || !bytes.Equal(body, orig) || h.Get("Content-Type") != "image/png" {
			t.Fatalf("download: %d %q %d bytes", code, h.Get("Content-Type"), len(body))
		}
		// 无签名访问、列举都被拒绝。
		if got := anon(oc.Endpoint + objstore.EscapePath("/"+oc.Bucket+"/"+f.ObjectKey)); got != 403 {
			t.Errorf("anonymous GET = %d", got)
		}
		if got := anon(oc.Endpoint + "/" + oc.Bucket + "?list-type=2"); got != 403 {
			t.Errorf("anonymous list = %d", got)
		}
	})

	t.Run("超大 PUT、改类型的 PUT 被拒绝；过期 PUT 被拒绝", func(t *testing.T) {
		r := e.mustRequest(tok1, fileReq("b.txt", 5, "text/plain"))
		if code := put(t, r.Put, []byte("hello world"), nil); code/100 == 2 {
			t.Errorf("oversized PUT = %d", code)
		}
		if code := put(t, r.Put, []byte("hello"), map[string]string{"Content-Type": "text/html"}); code/100 == 2 {
			t.Errorf("PUT with another Content-Type = %d", code)
		}
		time.Sleep(6 * time.Second)
		if code := put(t, r.Put, []byte("hello"), nil); code/100 == 2 {
			t.Errorf("expired PUT = %d", code)
		}
	})

	t.Run("恶意头：最终对象和下载响应只有服务端写入的 Content-Type；complete 后覆盖不影响下载", func(t *testing.T) {
		data := pad([]byte("%PDF-1.4"), 2048)
		r := e.mustRequest(tok1, fileReq("c.pdf", int64(len(data)), "application/pdf"))
		code := put(t, r.Put, data, map[string]string{"Content-Encoding": "gzip", "Content-Disposition": "inline; filename=x.html",
			"x-amz-website-redirect-location": "/evil", "x-amz-tagging": "a=b", "x-amz-acl": "public-read"})
		if code/100 != 2 {
			t.Logf("PUT with unsigned headers = %d (rejected by the store), retrying clean", code)
			e.mustPut(r.Put, data)
		}
		e.mustComplete(tok1, r.FileID)
		e.mustPut(r.Put, pad([]byte("<html>evil"), 2048)) // 原地址再 PUT
		c := e.dial("u1", "web")
		mustAck[struct{}](t, sendFile(c, "c2c", "u2", "real-000002", "file", r.FileID, nil))
		u := e.mustURL(tok2, r.FileID, "original")
		code, h, body := download(t, u.URL)
		if code != 200 || !bytes.Equal(body, data) {
			t.Fatalf("download after overwrite: %d %d bytes", code, len(body))
		}
		if h.Get("Content-Encoding") != "" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || h.Get("Content-Type") != "application/pdf" {
			t.Errorf("headers: %v", h)
		}
		// response-* 被签入：改了就失效。
		if got := anon(strings.Replace(u.URL, "response-content-type=", "response-content-type=text%2Fhtml", 1)); got != 403 {
			t.Errorf("changed response-content-type = %d", got)
		}
		time.Sleep(4 * time.Second)
		if code, _, _ := download(t, u.URL); code != 403 {
			t.Errorf("expired GET = %d", code)
		}
	})

	t.Run("嗅探：声明 png 内容是 HTML，415", func(t *testing.T) {
		img := e.mustRequest(tok1, map[string]any{"kind": "image", "name": "d.png", "size": 1024, "mime": "image/png",
			"thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
		e.mustPut(img.Put, pad([]byte("<html><script>x</script>"), 1024))
		e.mustPut(*img.ThumbPut, smallThumb())
		code, b := e.complete(tok1, img.FileID)
		wantErr(t, "html as png", code, b, 415, 4150, "")
	})

	t.Run("头像：跨桶 Copy 到公开桶，匿名可读、类型是服务端写入的，公开桶不能列举", func(t *testing.T) {
		u := e.register("alice_1", nil)
		img := fakePNG(512, 512, 4096)
		r := e.mustRequest(u.AccessToken, map[string]any{"kind": "avatar", "name": "avatar.png", "size": len(img), "mime": "image/png"})
		e.mustPut(r.Put, img)
		e.mustComplete(u.AccessToken, r.FileID)
		if code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": r.FileID}); code != 200 {
			t.Fatalf("bind avatar: %d %s", code, b)
		}
		pub := cli.PublicURL(e.file(r.FileID).PublicKey)
		code, h, body := download(t, pub)
		if code != 200 || !bytes.Equal(body, img) || h.Get("Content-Type") != "image/png" {
			t.Fatalf("public avatar: %d %q", code, h.Get("Content-Type"))
		}
		t.Logf("public avatar X-Content-Type-Options=%q (preferred: nosniff)", h.Get("X-Content-Type-Options"))
		if got := anon(oc.Endpoint + "/" + oc.PublicBucket + "?list-type=2"); got != 403 {
			t.Errorf("anonymous list of the public bucket = %d", got)
		}
	})

	t.Run("服务端凭证按前缀列举并删除（file_purge），全桶列举被拒绝", func(t *testing.T) {
		data := []byte("orphan")
		r := e.mustRequest(tok1, fileReq("e.txt", int64(len(data)), "text/plain"))
		e.mustPut(r.Put, data)
		e.mustComplete(tok1, r.FileID)
		if keys, err := cli.List(ctx, "final/"+r.FileID+"/"); err != nil || len(keys) != 1 {
			t.Fatalf("list by prefix: %v %v", keys, err)
		}
		e.setClock(t0.Add(25 * time.Hour))
		e.runPurge()
		if keys, err := cli.List(ctx, "final/"+r.FileID+"/"); err != nil || len(keys) != 0 || !e.fileGone(r.FileID) {
			t.Errorf("after file_purge: %v %v gone=%v", keys, err, e.fileGone(r.FileID))
		}
		var se *objstore.Error
		if _, err := cli.List(ctx, ""); !errors.As(err, &se) || se.Status != 403 {
			t.Errorf("full-bucket list with the server credential: %v (must be 403)", err)
		}
		if err := cli.Probe(ctx); err != nil {
			t.Errorf("probe: %v", err)
		}
	})
}
