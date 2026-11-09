// Package objstoretest 是测试用的假 S3（PRD-chat 第 4.5 节，BE-404 的 im-server 部分）。
//
// 它校验 SigV4（请求头签名和预签名两种）、预签名的过期（真实时钟）、签入的 Content-Length 和 Content-Type、
// 没有签入的 x-amz-* 头（拒绝）、ETag 条件（Range GET 的 If-Match、Copy 的 x-amz-copy-source-if-match）、
// Copy 的元数据替换、response-* 头、匿名访问和列举的权限、服务端凭证的按前缀列举权限。
// 可以注入：不可用（拒绝连接、黑洞、503）、删除失败、Copy 失败、任意请求之后的回调（HEAD 之后替换对象，RC2-07）、
// 直接写入对象、推进临时前缀生命周期。
//
// **假 S3 只验调度逻辑，不证明厂商行为**：厂商能力由合同测试验（AC-FILE-90-1、90-2，tools/objstore-contract）。
// 它和 im-server 在同一个测试进程里，但监听独立端口，上传的字节只经过它，不经过 im-server（AC-FILE-01-8）。
package objstoretest

import (
	"crypto/md5" // #nosec G501 -- 只用来生成 ETag，和 S3 的做法一样
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tianting/server/internal/objstore"
)

// Test 和 testing.TB 的子集。
type Test interface {
	Cleanup(func())
	Helper()
}

// Object 是桶里的一个对象。
type Object struct {
	Body               []byte
	ETag               string
	ContentType        string
	ContentEncoding    string
	ContentDisposition string
	Meta               map[string]string // x-amz-meta-*、x-amz-tagging 等
	Created            time.Time
}

// 不可用的方式（AC-CHAT-90-3：拒绝连接和黑洞）。
const (
	UpNormal    = ""
	DownRefuse  = "refuse"    // 立即断开连接
	DownBlack   = "blackhole" // 不响应，直到客户端超时
	Down503     = "503"
	defaultPage = 1000
)

// Server 是假 S3。
type Server struct {
	AccessKey, SecretKey string
	Region               string
	Private, Public      string // 桶名
	// ListPrefixes 是服务端凭证可以列举的前缀（第 11.3 节：只给最终前缀）。
	ListPrefixes []string
	PageSize     int

	srv     *httptest.Server
	mu      sync.Mutex
	buckets map[string]map[string]*Object
	down    string
	failDel int // 接下来几次 DELETE 返回 500
	failCp  int // 接下来几次 Copy 返回 500
	after   func(method, bucket, key string, query url.Values)
	etagSeq int
	// PutBytes 是收到的 PUT 请求体字节数合计（AC-FILE-01-8）。
	PutBytes atomic.Int64
	requests []string
}

// New 启动假 S3，有私有桶 private 和公开读桶 public。
func New(t Test) *Server {
	t.Helper()
	s := &Server{
		AccessKey: "fake-access-key", SecretKey: "fake-secret-key-for-tests-only", Region: "us-east-1",
		Private: "private", Public: "public", ListPrefixes: []string{"final/"},
		buckets: map[string]map[string]*Object{"private": {}, "public": {}},
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.srv.Close)
	return s
}

// URL 是 endpoint。
func (s *Server) URL() string { return s.srv.URL }

// Config 返回指向这个假 S3 的客户端配置。
func (s *Server) Config() objstore.Config {
	return objstore.Config{Endpoint: s.srv.URL, Region: s.Region, Bucket: s.Private, PublicBucket: s.Public, AccessKey: s.AccessKey, SecretKey: s.SecretKey, Timeout: 2 * time.Second}
}

// SetDown 让存储不可用（DownRefuse、DownBlack、Down503），UpNormal 恢复。
func (s *Server) SetDown(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.down = mode
}

// FailDeletes 让接下来 n 次 DELETE 失败（500）。
func (s *Server) FailDeletes(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failDel = n
}

// FailCopies 让接下来 n 次 Copy 失败（500）。
func (s *Server) FailCopies(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failCp = n
}

// After 设置每个请求处理完之后、响应之前的回调（例如 HEAD 之后替换对象）。传 nil 清除。回调里可以调 Put 等方法。
func (s *Server) After(f func(method, bucket, key string, query url.Values)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.after = f
}

// Put 直接写入对象（测试接口，不经过签名，AC-FILE-01-2）。
func (s *Server) Put(bucket, key string, body []byte, contentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putLocked(bucket, key, &Object{Body: append([]byte(nil), body...), ContentType: contentType})
}

func (s *Server) putLocked(bucket, key string, o *Object) {
	s.etagSeq++
	sum := md5.Sum(o.Body) // #nosec G401 -- ETag
	o.ETag = `"` + hex.EncodeToString(sum[:]) + `"`
	o.Created = time.Now()
	s.buckets[bucket][key] = o
}

// Get 返回对象的副本；不存在时 ok=false。
func (s *Server) Get(bucket, key string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.buckets[bucket][key]
	if !ok {
		return Object{}, false
	}
	return *o, true
}

// Keys 返回桶里 prefix 下的全部键（排序）。
func (s *Server) Keys(bucket, prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k := range s.buckets[bucket] {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// ExpireTmp 模拟临时前缀的生命周期规则（F-FILE-01 规则 3a，1 天）：删除私有桶 tmp/ 下的全部对象。真实行为见 AC-FILE-90-1。
func (s *Server) ExpireTmp() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k := range s.buckets[s.Private] {
		if strings.HasPrefix(k, "tmp/") {
			delete(s.buckets[s.Private], k)
			n++
		}
	}
	return n
}

// Requests 返回收到的请求（"METHOD bucket/key"，不含查询串），用于断言调用顺序。
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func fail(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<Error><Code>%s</Code></Error>", code)
}

type principal int

const (
	anonymous principal = iota
	server              // 请求头签名，服务端凭证
	presigned           // 预签名
)

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	down := s.down
	s.mu.Unlock()
	switch down {
	case DownRefuse:
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				c.Close()
				return
			}
		}
		fail(w, http.StatusServiceUnavailable, "ServiceUnavailable")
		return
	case DownBlack:
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
		return
	case Down503:
		fail(w, http.StatusServiceUnavailable, "ServiceUnavailable")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, http.StatusBadRequest, "IncompleteBody")
		return
	}
	if r.Method == http.MethodPut {
		s.PutBytes.Add(int64(len(body)))
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(path, "/")
	s.mu.Lock()
	_, known := s.buckets[bucket]
	s.requests = append(s.requests, r.Method+" "+bucket+"/"+key)
	s.mu.Unlock()
	if !known {
		fail(w, http.StatusNotFound, "NoSuchBucket")
		return
	}
	who, signedHeaders, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	switch {
	case who == anonymous:
		// 公开桶只能匿名读对象；匿名不能列举、不能写（AC-ACC-06-7、AC-FILE-04-6）。
		if bucket != s.Public || key == "" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			fail(w, http.StatusForbidden, "AccessDenied")
			return
		}
	case who == presigned:
		// 预签名不能列举（F-FILE-01 规则 9）；也不能 Copy、删除（这里只签 PUT、GET）。
		if key == "" || (r.Method != http.MethodPut && r.Method != http.MethodGet) || r.Header.Get("X-Amz-Copy-Source") != "" {
			fail(w, http.StatusForbidden, "AccessDenied")
			return
		}
	}
	switch {
	case r.Method == http.MethodGet && key == "":
		s.list(w, r, bucket)
	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		s.copyObject(w, r, bucket, key)
	case r.Method == http.MethodPut:
		s.putObject(w, r, bucket, key, body, who, signedHeaders)
	case r.Method == http.MethodHead || r.Method == http.MethodGet:
		s.getObject(w, r, bucket, key, who)
	case r.Method == http.MethodDelete:
		s.mu.Lock()
		if s.failDel > 0 {
			s.failDel--
			s.mu.Unlock()
			fail(w, http.StatusInternalServerError, "InternalError")
			return
		}
		delete(s.buckets[bucket], key)
		s.mu.Unlock()
		s.runAfter(r, bucket, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		fail(w, http.StatusMethodNotAllowed, "MethodNotAllowed")
	}
}

func (s *Server) runAfter(r *http.Request, bucket, key string) {
	s.mu.Lock()
	f := s.after
	s.mu.Unlock()
	if f != nil {
		f(r.Method, bucket, key, r.URL.Query())
	}
}

// authenticate 校验签名，返回身份和签入的头。失败时已经写好响应。
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (principal, map[string]bool, bool) {
	q := r.URL.Query()
	auth := r.Header.Get("Authorization")
	switch {
	case q.Get("X-Amz-Signature") != "":
		return s.authPresigned(w, r, q)
	case auth != "":
		return s.authHeader(w, r, auth)
	}
	return anonymous, nil, true
}

func headerValue(r *http.Request, name string) string {
	switch name {
	case "host":
		return r.Host
	case "content-length":
		return strconv.FormatInt(r.ContentLength, 10)
	}
	return r.Header.Get(name)
}

func (s *Server) authPresigned(w http.ResponseWriter, r *http.Request, q url.Values) (principal, map[string]bool, bool) {
	cred := q.Get("X-Amz-Credential")
	access, _, _ := strings.Cut(cred, "/")
	if access != s.AccessKey || q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		fail(w, http.StatusForbidden, "InvalidAccessKeyId")
		return 0, nil, false
	}
	ts, err := time.Parse("20060102T150405Z", q.Get("X-Amz-Date"))
	exp, err2 := strconv.Atoi(q.Get("X-Amz-Expires"))
	if err != nil || err2 != nil || exp < 1 || exp > 7*24*3600 {
		fail(w, http.StatusForbidden, "AuthorizationQueryParametersError")
		return 0, nil, false
	}
	// 过期按真实时钟（第 4.4 节）。
	if time.Now().After(ts.Add(time.Duration(exp) * time.Second)) {
		fail(w, http.StatusForbidden, "AccessDenied") // Request has expired
		return 0, nil, false
	}
	signed := map[string]bool{}
	headers := map[string]string{}
	for _, name := range strings.Split(q.Get("X-Amz-SignedHeaders"), ";") {
		signed[name] = true
		headers[name] = headerValue(r, name)
	}
	// 没有签入的 x-amz-* 头：签名不通过（S3 的行为，AC-FILE-01-21）。
	for name := range r.Header {
		if l := strings.ToLower(name); strings.HasPrefix(l, "x-amz-") && !signed[l] {
			fail(w, http.StatusForbidden, "AccessDenied")
			return 0, nil, false
		}
	}
	sq := url.Values{}
	for k, v := range q {
		if k != "X-Amz-Signature" {
			sq[k] = v
		}
	}
	want := objstore.Signature(objstore.SignInput{
		Method: r.Method, Path: r.URL.Path, Query: sq, Headers: headers, PayloadSHA: objstore.UnsignedPayload,
		Region: s.Region, Service: "s3", SecretKey: []byte(s.SecretKey), Time: ts,
	})
	if want != q.Get("X-Amz-Signature") {
		fail(w, http.StatusForbidden, "SignatureDoesNotMatch")
		return 0, nil, false
	}
	return presigned, signed, true
}

func (s *Server) authHeader(w http.ResponseWriter, r *http.Request, auth string) (principal, map[string]bool, bool) {
	var access, signedHeaders string
	for _, part := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ", ") {
		if v, ok := strings.CutPrefix(part, "Credential="); ok {
			access, _, _ = strings.Cut(v, "/")
		}
		if v, ok := strings.CutPrefix(part, "SignedHeaders="); ok {
			signedHeaders = v
		}
	}
	if access != s.AccessKey {
		fail(w, http.StatusForbidden, "InvalidAccessKeyId")
		return 0, nil, false
	}
	ts, err := time.Parse("20060102T150405Z", r.Header.Get("X-Amz-Date"))
	if err != nil || time.Since(ts).Abs() > 15*time.Minute {
		fail(w, http.StatusForbidden, "RequestTimeTooSkewed")
		return 0, nil, false
	}
	headers := map[string]string{}
	signed := map[string]bool{}
	for _, name := range strings.Split(signedHeaders, ";") {
		headers[name] = headerValue(r, name)
		signed[name] = true
	}
	want := objstore.AuthHeader(objstore.SignInput{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Headers: headers, PayloadSHA: r.Header.Get("X-Amz-Content-Sha256"),
		Region: s.Region, Service: "s3", AccessKey: s.AccessKey, SecretKey: []byte(s.SecretKey), Time: ts,
	})
	if want != auth {
		fail(w, http.StatusForbidden, "SignatureDoesNotMatch")
		return 0, nil, false
	}
	return server, signed, true
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request, bucket, key string, body []byte, who principal, signed map[string]bool) {
	o := &Object{Body: body, ContentType: r.Header.Get("Content-Type"), ContentEncoding: r.Header.Get("Content-Encoding"),
		ContentDisposition: r.Header.Get("Content-Disposition"), Meta: map[string]string{}}
	for name, v := range r.Header {
		if l := strings.ToLower(name); strings.HasPrefix(l, "x-amz-meta-") || l == "x-amz-tagging" || l == "x-amz-acl" || l == "x-amz-website-redirect-location" {
			o.Meta[l] = v[0]
		}
	}
	if who == presigned && (!signed["content-length"] || !signed["content-type"]) {
		fail(w, http.StatusForbidden, "AccessDenied") // 本项目的预签名 PUT 一律签入大小和类型
		return
	}
	s.mu.Lock()
	s.putLocked(bucket, key, o)
	etag := o.ETag
	s.mu.Unlock()
	s.runAfter(r, bucket, key)
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) copyObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	src, err := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
	if err != nil {
		fail(w, http.StatusBadRequest, "InvalidArgument")
		return
	}
	sb, sk, _ := strings.Cut(strings.TrimPrefix(src, "/"), "/")
	s.mu.Lock()
	if s.failCp > 0 {
		s.failCp--
		s.mu.Unlock()
		fail(w, http.StatusInternalServerError, "InternalError")
		return
	}
	srcBucket, ok := s.buckets[sb]
	var so *Object
	if ok {
		so = srcBucket[sk]
	}
	if so == nil {
		s.mu.Unlock()
		fail(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	if m := r.Header.Get("X-Amz-Copy-Source-If-Match"); m != "" && m != so.ETag {
		s.mu.Unlock()
		fail(w, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	}
	o := &Object{Body: append([]byte(nil), so.Body...), Meta: map[string]string{}}
	if r.Header.Get("X-Amz-Metadata-Directive") == "REPLACE" {
		o.ContentType = r.Header.Get("Content-Type")
	} else {
		o.ContentType, o.ContentEncoding, o.ContentDisposition = so.ContentType, so.ContentEncoding, so.ContentDisposition
		for k, v := range so.Meta {
			if strings.HasPrefix(k, "x-amz-meta-") {
				o.Meta[k] = v
			}
		}
	}
	if r.Header.Get("X-Amz-Tagging-Directive") != "REPLACE" {
		if v, ok := so.Meta["x-amz-tagging"]; ok {
			o.Meta["x-amz-tagging"] = v
		}
	}
	s.putLocked(bucket, key, o)
	// 加密下 Copy 的 ETag 可能和源不同（RS4-18）：假 S3 故意给一个不同的值，客户端只能用响应里的。
	o.ETag = fmt.Sprintf(`"%s-c%d"`, strings.Trim(o.ETag, `"`), s.etagSeq)
	etag := o.ETag
	s.mu.Unlock()
	s.runAfter(r, bucket, key)
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, "<CopyObjectResult><ETag>%s</ETag></CopyObjectResult>", xmlEscape(etag))
}

func xmlEscape(v string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request, bucket, key string, who principal) {
	s.mu.Lock()
	o, ok := s.buckets[bucket][key]
	var cp Object
	if ok {
		cp = *o
	}
	s.mu.Unlock()
	if !ok {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fail(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	if m := r.Header.Get("If-Match"); m != "" && m != cp.ETag {
		fail(w, http.StatusPreconditionFailed, "PreconditionFailed")
		return
	}
	body := cp.Body
	status := http.StatusOK
	if rg := r.Header.Get("Range"); rg != "" && r.Method == http.MethodGet {
		if v, ok := strings.CutPrefix(rg, "bytes=0-"); ok {
			if end, err := strconv.Atoi(v); err == nil && end+1 < len(body) {
				body = body[:end+1]
			}
			status = http.StatusPartialContent
		}
	}
	h := w.Header()
	h.Set("ETag", cp.ETag)
	if cp.ContentType != "" {
		h.Set("Content-Type", cp.ContentType)
	}
	if cp.ContentEncoding != "" {
		h.Set("Content-Encoding", cp.ContentEncoding)
	}
	if cp.ContentDisposition != "" {
		h.Set("Content-Disposition", cp.ContentDisposition)
	}
	for k, v := range cp.Meta {
		if strings.HasPrefix(k, "x-amz-meta-") {
			h.Set(k, v)
		}
	}
	// response-* 只对签名请求生效（S3 的规则）。
	if who != anonymous {
		q := r.URL.Query()
		if v := q.Get("response-content-type"); v != "" {
			h.Set("Content-Type", v)
		}
		if v := q.Get("response-content-disposition"); v != "" {
			h.Set("Content-Disposition", v)
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(cp.Body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(status)
		s.runAfter(r, bucket, key)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	// 回调在写响应之前：HEAD、Range GET 之后替换对象的注入要在客户端拿到结果之前生效（RC2-07）。
	s.runAfter(r, bucket, key)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request, bucket string) {
	q := r.URL.Query()
	prefix := q.Get("prefix")
	allowed := false
	for _, p := range s.ListPrefixes {
		if strings.HasPrefix(prefix, p) {
			allowed = true
		}
	}
	if bucket != s.Private || !allowed {
		fail(w, http.StatusForbidden, "AccessDenied") // 全桶列举不给 im-server（第 11.3 节）
		return
	}
	start, _ := strconv.Atoi(q.Get("continuation-token"))
	keys := s.Keys(bucket, prefix)
	page := s.PageSize
	if page <= 0 {
		page = defaultPage
	}
	type content struct{ Key string }
	out := struct {
		XMLName               xml.Name `xml:"ListBucketResult"`
		IsTruncated           bool
		NextContinuationToken string `xml:",omitempty"`
		Contents              []content
	}{}
	for i := start; i < len(keys); i++ {
		if i-start == page {
			out.IsTruncated, out.NextContinuationToken = true, strconv.Itoa(i)
			break
		}
		out.Contents = append(out.Contents, content{keys[i]})
	}
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(out)
}
