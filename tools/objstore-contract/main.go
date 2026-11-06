// objstore-contract 检查一个 S3 兼容存储是否满足 PRD-chat 第 21 节的必需能力（AC-FILE-90-1、90-2）。
//
// 只用标准库。凭证只从环境变量读，不打印凭证和签名地址。详见 README.md。
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type cred struct{ access, secret string }

type store struct {
	endpoint *url.URL
	region   string
	client   *http.Client
	skew     time.Duration
}

func awsEscape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && keepSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func canonicalQuery(q map[string]string) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, awsEscape(k, false)+"="+awsEscape(q[k], false))
	}
	return strings.Join(parts, "&")
}

func hmacSHA(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func (s *store) signature(c cred, day, stamp, canonical string) string {
	scope := day + "/" + s.region + "/s3/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + sha([]byte(canonical))
	k := hmacSHA([]byte("AWS4"+c.secret), day)
	k = hmacSHA(k, s.region)
	k = hmacSHA(k, "s3")
	k = hmacSHA(k, "aws4_request")
	return hex.EncodeToString(hmacSHA(k, toSign))
}

func objectPath(bucket, key string) string {
	p := "/" + bucket
	if key != "" {
		p += "/" + key
	}
	return p
}

func canonicalHeaders(h map[string]string) (string, string) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + ":" + strings.TrimSpace(h[k]) + "\n")
	}
	return b.String(), strings.Join(keys, ";")
}

// presign 生成预签名地址。headers 里的头会被签入，请求时必须原样带上。
func (s *store) presign(c cred, method, bucket, key string, expires int, headers, query map[string]string) string {
	now := time.Now().UTC().Add(s.skew)
	day, stamp := now.Format("20060102"), now.Format("20060102T150405Z")
	h := map[string]string{"host": s.endpoint.Host}
	for k, v := range headers {
		h[strings.ToLower(k)] = v
	}
	ch, signed := canonicalHeaders(h)
	q := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    c.access + "/" + day + "/" + s.region + "/s3/aws4_request",
		"X-Amz-Date":          stamp,
		"X-Amz-Expires":       fmt.Sprint(expires),
		"X-Amz-SignedHeaders": signed,
	}
	for k, v := range query {
		q[k] = v
	}
	path := objectPath(bucket, key)
	canon := method + "\n" + awsEscape(path, true) + "\n" + canonicalQuery(q) + "\n" + ch + "\n" + signed + "\nUNSIGNED-PAYLOAD"
	return s.endpoint.Scheme + "://" + s.endpoint.Host + awsEscape(path, true) + "?" + canonicalQuery(q) + "&X-Amz-Signature=" + s.signature(c, day, stamp, canon)
}

type result struct {
	status int
	header http.Header
	body   []byte
}

func (s *store) do(req *http.Request) (result, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return result{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return result{resp.StatusCode, resp.Header, b}, err
}

// call 发一个用请求头签名的请求。
func (s *store) call(c cred, method, bucket, key string, query, headers map[string]string, body []byte) (result, error) {
	now := time.Now().UTC().Add(s.skew)
	day, stamp := now.Format("20060102"), now.Format("20060102T150405Z")
	h := map[string]string{"host": s.endpoint.Host, "x-amz-date": stamp, "x-amz-content-sha256": sha(body)}
	for k, v := range headers {
		h[strings.ToLower(k)] = v
	}
	ch, signed := canonicalHeaders(h)
	path := objectPath(bucket, key)
	cq := canonicalQuery(query)
	canon := method + "\n" + awsEscape(path, true) + "\n" + cq + "\n" + ch + "\n" + signed + "\n" + h["x-amz-content-sha256"]
	u := s.endpoint.Scheme + "://" + s.endpoint.Host + awsEscape(path, true)
	if cq != "" {
		u += "?" + cq
	}
	req, err := http.NewRequest(method, u, bytes.NewReader(body))
	if err != nil {
		return result{}, err
	}
	for k, v := range h {
		if k != "host" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.access+"/"+day+"/"+s.region+"/s3/aws4_request, SignedHeaders="+signed+", Signature="+s.signature(c, day, stamp, canon))
	return s.do(req)
}

func (s *store) raw(method, u string, headers map[string]string, body []byte) (result, error) {
	req, err := http.NewRequest(method, u, bytes.NewReader(body))
	if err != nil {
		return result{}, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return s.do(req)
}

func (s *store) list(c cred, bucket, prefix string) ([]string, int, error) {
	q := map[string]string{"list-type": "2"}
	if prefix != "" {
		q["prefix"] = prefix
	}
	r, err := s.call(c, "GET", bucket, "", q, nil, nil)
	if err != nil {
		return nil, 0, err
	}
	if r.status != 200 {
		return nil, r.status, nil
	}
	var parsed struct {
		Contents []struct{ Key string }
	}
	if err := xml.Unmarshal(r.body, &parsed); err != nil {
		return nil, r.status, err
	}
	keys := make([]string, 0, len(parsed.Contents))
	for _, k := range parsed.Contents {
		keys = append(keys, k.Key)
	}
	return keys, r.status, nil
}

// ---------- 结果 ----------

const (
	required  = "必需"
	preferred = "优先"
	info      = "记录"
)

type check struct{ level, name, outcome, detail string }

var checks []check

func record(level, name string, ok bool, detail string) {
	out := "通过"
	if !ok {
		out = "失败"
		if level == info {
			out = "记录"
		}
	}
	checks = append(checks, check{level, name, out, detail})
}

func note(name, detail string) { checks = append(checks, check{info, name, "记录", detail}) }

func randID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fmt.Fprintln(os.Stderr, "缺少环境变量 "+k)
		os.Exit(2)
	}
	return v
}

func st(r result, err error) string {
	if err != nil {
		return "请求错误: " + err.Error()
	}
	return fmt.Sprintf("HTTP %d", r.status)
}

func main() {
	ep, err := url.Parse(mustEnv("OC_ENDPOINT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "OC_ENDPOINT 无效")
		os.Exit(2)
	}
	s := &store{endpoint: ep, region: mustEnv("OC_REGION"), client: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
	admin := cred{mustEnv("OC_ACCESS_KEY"), mustEnv("OC_SECRET_KEY")}
	scoped := cred{mustEnv("OC_SCOPED_ACCESS_KEY"), mustEnv("OC_SCOPED_SECRET_KEY")}
	bucket, public := mustEnv("OC_BUCKET"), mustEnv("OC_PUBLIC_BUCKET")
	publicBase := strings.TrimRight(os.Getenv("OC_PUBLIC_BASE"), "/")
	if publicBase == "" {
		publicBase = ep.Scheme + "://" + ep.Host + "/" + public
	}

	run := randID()
	tmp := "tmp/" + run + "/original"
	fid := randID()
	finalPrefix := "final/" + fid + "/"
	final := finalPrefix + randID() + "/original"
	original := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("a"), 4096)...)
	cl := fmt.Sprint(len(original))
	put := map[string]string{"Content-Length": cl, "Content-Type": "image/png"}
	onlyCT := map[string]string{"Content-Type": "image/png"}

	// 1. 预签名 PUT：Content-Length、Content-Type 签入
	u := s.presign(scoped, "PUT", bucket, tmp, 900, put, nil)
	r, e := s.raw("PUT", u, onlyCT, original)
	record(required, "预签名 PUT 到临时键", e == nil && r.status == 200, st(r, e))
	r, e = s.raw("PUT", u, onlyCT, append(append([]byte{}, original...), 'x'))
	record(required, "PUT 超过签入的 Content-Length 被拒绝", e == nil && r.status >= 400, st(r, e))
	r, e = s.raw("PUT", u, onlyCT, original[:10])
	record(required, "PUT 小于签入的 Content-Length 被拒绝", e == nil && r.status >= 400, st(r, e))
	r, e = s.raw("PUT", u, map[string]string{"Content-Type": "text/html"}, original)
	record(required, "PUT 改 Content-Type 被拒绝", e == nil && r.status >= 400, st(r, e))
	short := s.presign(scoped, "PUT", bucket, "tmp/"+run+"/expired", 1, put, nil)
	time.Sleep(2500 * time.Millisecond)
	r, e = s.raw("PUT", short, onlyCT, original)
	record(required, "过期的预签名 PUT 被拒绝", e == nil && r.status >= 400, st(r, e))

	// 2. 未签名的恶意头（AC-FILE-01-21）：逐个头记录存储的处理方式
	for _, bad := range [][2]string{
		{"Content-Encoding", "gzip"}, {"Content-Disposition", "inline; filename=x.html"}, {"x-amz-meta-evil", "1"},
		{"x-amz-acl", "public-read"}, {"x-amz-website-redirect-location", "/evil"}, {"x-amz-tagging", "evil=1"},
	} {
		k := "tmp/" + run + "/hdr-" + randID()
		r, e = s.raw("PUT", s.presign(scoped, "PUT", bucket, k, 900, put, nil), map[string]string{"Content-Type": "image/png", bad[0]: bad[1]}, original)
		stored := ""
		if e == nil && r.status == 200 {
			if h, herr := s.call(admin, "HEAD", bucket, k, nil, nil, nil); herr == nil {
				stored = h.header.Get(bad[0])
			}
		}
		note("预签名 PUT 带未签名的 "+bad[0], fmt.Sprintf("%s，存下的值=%q", st(r, e), stored))
	}

	// 不能指望 PUT 带上恶意头，所以直接造一个确实带恶意元数据和标签的源对象。
	// 第一项是测试前提：不成立时，后面的"只有服务端写入的元数据"就是空测。
	evil := "tmp/" + run + "/evil"
	r, e = s.call(admin, "PUT", bucket, evil, nil, map[string]string{
		"Content-Type": "text/html", "Content-Encoding": "gzip", "Content-Disposition": "inline; filename=x.html",
		"x-amz-meta-evil": "1", "x-amz-website-redirect-location": "/evil", "x-amz-tagging": "evil=1",
	}, original)
	put1 := st(r, e)
	r, e = s.call(admin, "HEAD", bucket, evil, nil, nil, nil)
	tags, tagErr := s.call(admin, "GET", bucket, evil, map[string]string{"tagging": ""}, nil, nil)
	premise := e == nil && r.header.Get("Content-Encoding") == "gzip" && r.header.Get("Content-Disposition") != "" && r.header.Get("x-amz-meta-evil") == "1" && r.header.Get("Content-Type") == "text/html" && tagErr == nil && bytes.Contains(tags.body, []byte("evil"))
	record(required, "测试前提：源对象确实带恶意元数据和标签", premise, fmt.Sprintf("PUT %s Content-Encoding=%q meta=%q redirect=%q", put1, r.header.Get("Content-Encoding"), r.header.Get("x-amz-meta-evil"), r.header.Get("x-amz-website-redirect-location")))

	// 对照：不替换时 Copy 会不会带上这些元数据。带上才说明 REPLACE 有必要，也说明下面的检查测得出差别。
	plain := "final/" + fid + "/plain/original"
	r, e = s.call(admin, "PUT", bucket, plain, nil, map[string]string{"x-amz-copy-source": "/" + bucket + "/" + evil}, nil)
	copyStatus := st(r, e)
	r, e = s.call(admin, "HEAD", bucket, plain, nil, nil, nil)
	plainTags, _ := s.call(admin, "GET", bucket, plain, map[string]string{"tagging": ""}, nil, nil)
	note("对照：不替换的 Copy 保留什么", fmt.Sprintf("%s Content-Type=%q Content-Encoding=%q meta=%q 标签=%v", copyStatus, r.header.Get("Content-Type"), r.header.Get("Content-Encoding"), r.header.Get("x-amz-meta-evil"), bytes.Contains(plainTags.body, []byte("evil"))))
	_, _ = s.call(admin, "DELETE", bucket, plain, nil, nil, nil)

	// 3. HEAD、ETag、If-Match Range
	r, e = s.call(scoped, "HEAD", bucket, tmp, nil, nil, nil)
	etag := ""
	if e == nil {
		etag = r.header.Get("ETag")
	}
	record(required, "HEAD 返回大小和 ETag", e == nil && r.status == 200 && etag != "" && r.header.Get("Content-Length") == cl, st(r, e))
	note("源对象服务端加密", "x-amz-server-side-encryption="+r.header.Get("x-amz-server-side-encryption"))
	r, e = s.call(scoped, "GET", bucket, tmp, nil, map[string]string{"Range": "bytes=0-63", "If-Match": etag}, nil)
	record(required, "Range GET + If-Match 正确", e == nil && r.status == 206 && len(r.body) == 64, st(r, e))
	r, e = s.call(scoped, "GET", bucket, tmp, nil, map[string]string{"Range": "bytes=0-63", "If-Match": `"00000000000000000000000000000000"`}, nil)
	record(required, "Range GET + If-Match 错误返回 412", e == nil && r.status == 412, st(r, e))

	// 4. 条件 Copy 和元数据替换
	copyHeaders := func(src, cond string) map[string]string {
		return map[string]string{
			"x-amz-copy-source":          "/" + bucket + "/" + src,
			"x-amz-copy-source-if-match": cond,
			"x-amz-metadata-directive":   "REPLACE",
			"x-amz-tagging-directive":    "REPLACE",
			"Content-Type":               "image/png",
		}
	}
	r, e = s.call(scoped, "PUT", bucket, final+".bad", nil, copyHeaders(tmp, `"00000000000000000000000000000000"`), nil)
	record(required, "条件 Copy：ETag 不符返回 412", e == nil && r.status == 412, st(r, e))
	r, e = s.call(scoped, "HEAD", bucket, final+".bad", nil, nil, nil)
	record(required, "条件 Copy 失败时目标不存在", e == nil && r.status == 404, st(r, e))

	// 对象被替换后，用旧 ETag Copy 必须失败
	swap := "tmp/" + run + "/swap"
	su := s.presign(scoped, "PUT", bucket, swap, 900, put, nil)
	_, _ = s.raw("PUT", su, onlyCT, original)
	r, _ = s.call(scoped, "HEAD", bucket, swap, nil, nil, nil)
	swapEtag := r.header.Get("ETag")
	changed := append([]byte{}, original...)
	changed[100] = 'b'
	_, _ = s.raw("PUT", su, onlyCT, changed)
	r, e = s.call(scoped, "PUT", bucket, finalPrefix+"swap/original", nil, copyHeaders(swap, swapEtag), nil)
	record(required, "HEAD 后替换对象，条件 Copy 返回 412", e == nil && r.status == 412, st(r, e))

	// 带恶意元数据的对象 Copy 到最终键
	src := evil
	r, _ = s.call(scoped, "HEAD", bucket, src, nil, nil, nil)
	srcEtag := r.header.Get("ETag")
	r, e = s.call(scoped, "PUT", bucket, final, nil, copyHeaders(src, srcEtag), nil)
	record(required, "条件 Copy + 元数据替换成功", e == nil && r.status == 200, st(r, e))
	var copied struct {
		ETag string `xml:"ETag"`
	}
	if e == nil {
		_ = xml.Unmarshal(r.body, &copied)
	}
	r, e = s.call(scoped, "HEAD", bucket, final, nil, nil, nil)
	finalEtag := ""
	if e == nil {
		finalEtag = r.header.Get("ETag")
	}
	clean := e == nil && r.status == 200 && r.header.Get("Content-Type") == "image/png" && r.header.Get("Content-Encoding") == "" &&
		r.header.Get("Content-Disposition") == "" && r.header.Get("x-amz-meta-evil") == "" && r.header.Get("x-amz-website-redirect-location") == ""
	record(required, "最终对象只有服务端写入的元数据", clean, fmt.Sprintf("%s Content-Type=%q Content-Encoding=%q Content-Disposition=%q meta=%q redirect=%q", st(r, e), r.header.Get("Content-Type"), r.header.Get("Content-Encoding"), r.header.Get("Content-Disposition"), r.header.Get("x-amz-meta-evil"), r.header.Get("x-amz-website-redirect-location")))
	record(required, "Copy 响应的 ETag 和 HEAD 最终键一致", copied.ETag != "" && strings.Trim(copied.ETag, `"`) == strings.Trim(finalEtag, `"`), "")
	note("Copy 前后 ETag 是否相同（PRD 不依赖）", fmt.Sprintf("相同=%v", strings.Trim(srcEtag, `"`) == strings.Trim(finalEtag, `"`)))
	note("最终对象服务端加密", "x-amz-server-side-encryption="+r.header.Get("x-amz-server-side-encryption"))
	r, e = s.call(scoped, "GET", bucket, final, map[string]string{"tagging": ""}, nil, nil)
	record(required, "最终对象没有标签", e == nil && r.status == 200 && !bytes.Contains(r.body, []byte("evil")), st(r, e))

	// complete 后再 PUT 临时键不影响最终键
	_, _ = s.raw("PUT", su, onlyCT, changed)
	_, _ = s.raw("PUT", u, onlyCT, changed)
	r, e = s.call(scoped, "GET", bucket, final, nil, nil, nil)
	record(required, "覆盖临时键不影响最终键", e == nil && r.status == 200 && bytes.Equal(r.body, original), st(r, e))

	// 5. 预签名 GET
	disp := "attachment; filename=\"file\"; filename*=UTF-8''%E6%96%87%E4%BB%B6.txt"
	gq := map[string]string{"response-content-type": "text/plain", "response-content-disposition": disp}
	gu := s.presign(scoped, "GET", bucket, final, 300, nil, gq)
	r, e = s.raw("GET", gu, nil, nil)
	record(required, "预签名 GET 的 response-* 头生效", e == nil && r.status == 200 && r.header.Get("Content-Type") == "text/plain" && r.header.Get("Content-Disposition") == disp, fmt.Sprintf("%s Content-Disposition=%q", st(r, e), r.header.Get("Content-Disposition")))
	record(preferred, "响应带 nosniff", e == nil && strings.EqualFold(r.header.Get("X-Content-Type-Options"), "nosniff"), "X-Content-Type-Options="+r.header.Get("X-Content-Type-Options"))
	r, e = s.raw("GET", strings.Replace(gu, "text%2Fplain", "text%2Fhtml", 1), nil, nil)
	record(required, "改 response-content-type 后签名失效", e == nil && r.status == 403, st(r, e))
	eg := s.presign(scoped, "GET", bucket, final, 1, nil, nil)
	time.Sleep(2500 * time.Millisecond)
	r, e = s.raw("GET", eg, nil, nil)
	record(required, "过期的预签名 GET 被拒绝", e == nil && r.status == 403, st(r, e))
	r, e = s.raw("GET", gu[:strings.Index(gu, "/"+bucket+"/")]+"/"+bucket+"/"+final, nil, nil)
	record(required, "无签名不能读私有对象", e == nil && r.status == 403, st(r, e))

	// 6. 列举
	r, e = s.raw("GET", ep.Scheme+"://"+ep.Host+"/"+bucket+"?list-type=2", nil, nil)
	record(required, "私有桶匿名列举被拒绝", e == nil && r.status == 403, st(r, e))
	keys, code, err := s.list(scoped, bucket, finalPrefix)
	record(required, "服务端凭证按前缀列举", err == nil && code == 200 && len(keys) >= 1, fmt.Sprintf("HTTP %d，%d 个键", code, len(keys)))
	_, code, _ = s.list(scoped, bucket, "")
	record(required, "服务端凭证全桶列举被拒绝", code == 403, fmt.Sprintf("HTTP %d", code))

	// 7. 跨桶 Copy 到公开桶（头像）
	pub := "avatar/" + randID()
	h := copyHeaders(final, finalEtag)
	r, e = s.call(scoped, "PUT", public, pub, nil, h, nil)
	record(required, "跨桶 Copy 到公开桶", e == nil && r.status == 200, st(r, e))
	r, e = s.raw("GET", publicBase+"/"+pub, nil, nil)
	record(required, "公开对象匿名可读，类型是服务端写入的", e == nil && r.status == 200 && r.header.Get("Content-Type") == "image/png", fmt.Sprintf("%s Content-Type=%q", st(r, e), r.header.Get("Content-Type")))
	record(preferred, "公开对象带 nosniff", e == nil && strings.EqualFold(r.header.Get("X-Content-Type-Options"), "nosniff"), "X-Content-Type-Options="+r.header.Get("X-Content-Type-Options"))
	r, e = s.raw("GET", publicBase+"?list-type=2", nil, nil)
	record(required, "公开桶匿名列举被拒绝", e == nil && r.status == 403, st(r, e))

	// 8. 生命周期：只验证配置生效，不等实际清除
	lc := []byte(`<LifecycleConfiguration><Rule><ID>tmp-1d</ID><Status>Enabled</Status><Filter><Prefix>tmp/</Prefix></Filter><Expiration><Days>1</Days></Expiration></Rule></LifecycleConfiguration>`)
	sum := md5.Sum(lc)
	r, e = s.call(admin, "PUT", bucket, "", map[string]string{"lifecycle": ""}, map[string]string{"Content-MD5": base64.StdEncoding.EncodeToString(sum[:]), "Content-Type": "application/xml"}, lc)
	ok := e == nil && r.status == 200
	r2, e2 := s.call(admin, "GET", bucket, "", map[string]string{"lifecycle": ""}, nil, nil)
	record(required, "临时前缀生命周期规则可以配置并读回（没有等实际清除）", ok && e2 == nil && r2.status == 200 && bytes.Contains(r2.body, []byte("tmp/")), st(r, e)+" / 读回 "+st(r2, e2))

	// 9. CORS
	r, e = s.raw("OPTIONS", s.presign(scoped, "PUT", bucket, "tmp/"+run+"/cors", 900, put, nil), map[string]string{"Origin": "https://app.example.com", "Access-Control-Request-Method": "PUT", "Access-Control-Request-Headers": "content-type"}, nil)
	acao := ""
	if e == nil {
		acao = r.header.Get("Access-Control-Allow-Origin")
	}
	record(required, "CORS 预检允许 PUT", e == nil && r.status < 300 && acao != "", fmt.Sprintf("%s Allow-Origin=%q", st(r, e), acao))
	note("CORS 能否限定来源", "Allow-Origin="+acao+"（* 表示没有限定，生产要配成 App 的来源）")

	// 10. 按前缀删除（S3 没有前缀删除接口，先列举再逐个删），用真实权限的凭证
	keys, _, _ = s.list(scoped, bucket, finalPrefix)
	allOK := len(keys) > 0
	for _, k := range keys {
		r, e = s.call(scoped, "DELETE", bucket, k, nil, nil, nil)
		allOK = allOK && e == nil && r.status == 204
	}
	left, _, _ := s.list(scoped, bucket, finalPrefix)
	record(required, "按前缀列举并逐个删除", allOK && len(left) == 0, fmt.Sprintf("删 %d 个，剩 %d 个", len(keys), len(left)))
	r, e = s.call(scoped, "DELETE", public, pub, nil, nil, nil)
	record(required, "服务端凭证能删公开桶对象", e == nil && r.status == 204, st(r, e))
	r, e = s.call(scoped, "DELETE", bucket, final, nil, nil, nil)
	record(required, "重复删除幂等", e == nil && r.status == 204, st(r, e))
	r, e = s.call(scoped, "GET", public, pub, nil, nil, nil)
	note("服务端凭证读公开桶（PRD 不需要）", st(r, e))

	// 清理临时对象
	if tk, _, _ := s.list(scoped, bucket, "tmp/"+run+"/"); tk != nil {
		for _, k := range tk {
			_, _ = s.call(scoped, "DELETE", bucket, k, nil, nil, nil)
		}
	}

	fail := 0
	for _, c := range checks {
		fmt.Printf("| %s | %s | %s | %s |\n", c.level, c.name, c.outcome, strings.ReplaceAll(c.detail, "|", "\\|"))
		if c.level == required && c.outcome == "失败" {
			fail++
		}
	}
	fmt.Printf("\n必需项失败 %d 个，共 %d 项\n", fail, len(checks))
	if fail > 0 {
		os.Exit(1)
	}
}
