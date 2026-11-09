package objstore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/objstore"
	"tianting/server/internal/objstore/objstoretest"
)

// AWS 文档里预签名 GET 的公开示例（Authenticating Requests: Using Query Parameters）。签名实现和 AWS 一致。
func TestSignatureAWSPresignVector(t *testing.T) {
	ts, _ := time.Parse("20060102T150405Z", "20130524T000000Z")
	q := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {"AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request"},
		"X-Amz-Date":          {"20130524T000000Z"},
		"X-Amz-Expires":       {"86400"},
		"X-Amz-SignedHeaders": {"host"},
	}
	got := objstore.Signature(objstore.SignInput{
		Method: "GET", Path: "/test.txt", Query: q, Headers: map[string]string{"host": "examplebucket.s3.amazonaws.com"},
		PayloadSHA: objstore.UnsignedPayload, Region: "us-east-1", Service: "s3",
		SecretKey: []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"), Time: ts,
	})
	if want := "aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"; got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
}

func put(t *testing.T, p objstore.Presigned, body []byte, extra map[string]string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, p.URL, bytes.NewReader(body))
	for k, v := range p.Headers {
		if k != "Content-Length" {
			req.Header.Set(k, v)
		}
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func get(t *testing.T, u string) (int, http.Header, []byte) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

// BE-430：预签名 PUT 签入大小和类型；HEAD、带 If-Match 的 Range GET、条件 Copy（元数据替换）、DELETE、按前缀 List。
func TestClientOperations(t *testing.T) {
	fs := objstoretest.New(t)
	c := objstore.New(fs.Config())
	ctx := context.Background()
	body := []byte("hello object store")

	p := c.PresignPut("tmp/k1", int64(len(body)), "image/png", time.Minute)
	if code := put(t, p, body[:5], nil); code == http.StatusOK {
		t.Fatal("PUT with a different size must be rejected")
	}
	if code := put(t, objstore.Presigned{URL: p.URL, Headers: map[string]string{"Content-Type": "text/html"}}, body, nil); code == http.StatusOK {
		t.Fatal("PUT with a different content type must be rejected")
	}
	if code := put(t, p, body, map[string]string{"x-amz-meta-evil": "1"}); code == http.StatusOK {
		t.Fatal("PUT with an unsigned x-amz header must be rejected")
	}
	if code := put(t, p, body, map[string]string{"Content-Encoding": "gzip", "Content-Disposition": "inline"}); code != http.StatusOK {
		t.Fatalf("PUT = %d", code)
	}
	info, err := c.Head(ctx, "tmp/k1")
	if err != nil || info.Size != int64(len(body)) || info.ETag == "" {
		t.Fatalf("head = %+v, %v", info, err)
	}
	b, err := c.GetRange(ctx, "tmp/k1", 5, info.ETag)
	if err != nil || string(b) != "hello" {
		t.Fatalf("range = %q, %v", b, err)
	}
	if _, err := c.GetRange(ctx, "tmp/k1", 5, `"other"`); !errors.Is(err, objstore.ErrPrecondition) {
		t.Fatalf("range with a stale etag: %v", err)
	}
	if _, err := c.Copy(ctx, "tmp/k1", c.Bucket(), "final/f1/a/original", `"other"`, "image/png"); !errors.Is(err, objstore.ErrPrecondition) {
		t.Fatalf("copy with a stale etag: %v", err)
	}
	etag, err := c.Copy(ctx, "tmp/k1", c.Bucket(), "final/f1/a/original", info.ETag, "image/png")
	if err != nil || etag == "" || etag == info.ETag {
		t.Fatalf("copy etag = %q (source %q), %v", etag, info.ETag, err)
	}
	o, _ := fs.Get(fs.Private, "final/f1/a/original")
	if o.ContentType != "image/png" || o.ContentEncoding != "" || o.ContentDisposition != "" || len(o.Meta) != 0 {
		t.Fatalf("copied object keeps client metadata: %+v", o)
	}
	fi, err := c.Head(ctx, "final/f1/a/original")
	if err != nil || fi.ETag != etag {
		t.Fatalf("head after copy = %+v, %v (copy etag %s)", fi, err, etag)
	}
	if _, err := c.Head(ctx, "tmp/none"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("head missing: %v", err)
	}
	fs.Put(fs.Private, "final/f1/b/thumb", []byte("x"), "image/jpeg")
	keys, err := c.List(ctx, "final/f1/")
	if err != nil || len(keys) != 2 {
		t.Fatalf("list = %v, %v", keys, err)
	}
	if _, err := c.List(ctx, "tmp/"); err == nil {
		t.Fatal("the server credential must not list tmp/ (PRD 11.3)")
	}
	n, err := c.DeletePrefix(ctx, "final/f1/")
	if err != nil || n != 2 || len(fs.Keys(fs.Private, "final/")) != 0 {
		t.Fatalf("delete prefix = %d, %v, left %v", n, err, fs.Keys(fs.Private, ""))
	}
	if err := c.Delete(ctx, c.Bucket(), "final/f1/b/thumb"); err != nil {
		t.Fatalf("deleting a missing key is not an error: %v", err)
	}
	// 公开桶：Copy 过去之后匿名可读，列举被拒绝。
	if _, err := c.Copy(ctx, "tmp/k1", c.PublicBucket(), "avatars/x", info.ETag, "image/png"); err != nil {
		t.Fatal(err)
	}
	code, h, got := get(t, c.PublicURL("avatars/x"))
	if code != 200 || string(got) != string(body) || h.Get("Content-Type") != "image/png" {
		t.Fatalf("public get = %d %s %q", code, h.Get("Content-Type"), got)
	}
	if code, _, _ := get(t, fs.URL()+"/"+fs.Public+"?list-type=2"); code != http.StatusForbidden {
		t.Fatalf("anonymous list of the public bucket = %d", code)
	}
}

// TC-FILE-04-06（AC-FILE-04-6）：不带签名直接访问对象键、列举私有桶，都被拒绝；预签名也不能列举。
func TestAnonymousAccessDenied(t *testing.T) {
	fs := objstoretest.New(t)
	c := objstore.New(fs.Config())
	fs.Put(fs.Private, "final/f1/a/original", []byte("secret"), "image/png")
	for _, u := range []string{fs.URL() + "/private/final/f1/a/original", fs.URL() + "/private?list-type=2", fs.URL() + "/private?list-type=2&prefix=final/"} {
		if code, _, _ := get(t, u); code != http.StatusForbidden {
			t.Errorf("GET %s = %d, want 403", strings.TrimPrefix(u, fs.URL()), code)
		}
	}
	// 预签名一个 GET，再把路径改成桶：签名不对或被拒绝。
	u, _ := c.PresignGet("final/f1/a/original", time.Minute, "", "")
	if code, _, b := get(t, u); code != 200 || string(b) != "secret" {
		t.Fatalf("presigned get = %d", code)
	}
	if code, _, _ := get(t, strings.Replace(u, "/private/final/f1/a/original", "/private", 1)); code != http.StatusForbidden {
		t.Fatalf("presigned list = %d, want 403", code)
	}
}

// TC-FILE-04-04（AC-FILE-04-4）的客户端部分：预签名按真实时钟过期，response-* 头签入。
func TestPresignGetExpiresAndResponseHeaders(t *testing.T) {
	fs := objstoretest.New(t)
	c := objstore.New(fs.Config())
	fs.Put(fs.Private, "final/f/a/original", []byte("<html>"), "text/html")
	u, exp := c.PresignGet("final/f/a/original", time.Second, "image/png", `attachment; filename="a.txt"`)
	if d := time.Until(time.UnixMilli(exp)); d > time.Second || d < 0 {
		t.Fatalf("expiresAt in %v", d)
	}
	code, h, _ := get(t, u)
	if code != 200 || h.Get("Content-Type") != "image/png" || h.Get("Content-Disposition") != `attachment; filename="a.txt"` {
		t.Fatalf("get = %d %v", code, h)
	}
	// 篡改 response-content-type：签名不对。
	if code, _, _ := get(t, strings.Replace(u, "image%2Fpng", "text%2Fhtml", 1)); code != http.StatusForbidden {
		t.Fatalf("tampered response-content-type = %d", code)
	}
	time.Sleep(2100 * time.Millisecond)
	if code, _, _ := get(t, u); code != http.StatusForbidden {
		t.Fatalf("expired url = %d", code)
	}
}

// AC-CHAT-90-3 的客户端部分：拒绝连接、黑洞、503 都按不可用；404 不是不可用。
func TestUnavailableClassification(t *testing.T) {
	fs := objstoretest.New(t)
	cfg := fs.Config()
	cfg.Timeout = 300 * time.Millisecond
	c := objstore.New(cfg)
	for _, mode := range []string{objstoretest.DownRefuse, objstoretest.DownBlack, objstoretest.Down503} {
		fs.SetDown(mode)
		_, err := c.Head(context.Background(), "tmp/x")
		if !objstore.IsUnavailable(err) {
			t.Errorf("%s: err = %v, want unavailable", mode, err)
		}
		if err != nil && strings.Contains(err.Error(), "127.0.0.1") {
			t.Errorf("%s: error leaks the address: %v", mode, err)
		}
	}
	fs.SetDown(objstoretest.UpNormal)
	if _, err := c.Head(context.Background(), "tmp/x"); objstore.IsUnavailable(err) || !errors.Is(err, objstore.ErrNotFound) {
		t.Errorf("404 = %v", err)
	}
}

// BE-438 第 3 条：Observe 给指标用（op 有界，404 / 412 不算错误）；Probe 是配置检查的"对象存储连通"。
func TestObserveAndProbe(t *testing.T) {
	fs := objstoretest.New(t)
	cfg := fs.Config()
	cfg.Timeout = 300 * time.Millisecond
	var ops, errs []string
	cfg.Observe = func(op string, err error) {
		ops = append(ops, op)
		if objstore.IsError(err) {
			errs = append(errs, op)
		}
	}
	c := objstore.New(cfg)
	ctx := context.Background()
	if err := c.Probe(ctx); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if _, err := c.Head(ctx, "tmp/missing"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("head = %v", err)
	}
	fs.Put(fs.Private, "tmp/a", []byte("hello"), "text/plain")
	if _, err := c.Copy(ctx, "tmp/a", fs.Private, "final/f1/a1/original", `"wrong-etag"`, "text/plain"); !errors.Is(err, objstore.ErrPrecondition) {
		t.Fatalf("copy = %v", err)
	}
	if err := c.Delete(ctx, fs.Private, "tmp/a"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"list", "head", "copy", "delete"}; strings.Join(ops, ",") != strings.Join(want, ",") || len(errs) != 0 {
		t.Fatalf("ops = %v, errors = %v", ops, errs)
	}
	for _, mode := range []string{objstoretest.DownRefuse, objstoretest.DownBlack, objstoretest.Down503} {
		fs.SetDown(mode)
		errs = nil
		err := c.Probe(ctx)
		if err == nil || !objstore.IsUnavailable(err) || len(errs) != 1 || errs[0] != "list" {
			t.Errorf("%s: probe = %v, errors = %v", mode, err, errs)
		}
	}
	fs.SetDown(objstoretest.UpNormal)
	// 凭证错误：能连通但 403，算错误，不算不可用。
	bad := fs.Config()
	bad.SecretKey = "wrong-secret"
	if err := objstore.New(bad).Probe(ctx); err == nil || objstore.IsUnavailable(err) || !objstore.IsError(err) {
		t.Errorf("bad credentials: %v", err)
	}
}
