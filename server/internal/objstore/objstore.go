// Package objstore 是 im-server 的 S3 兼容对象存储客户端（PRD-chat 第 11.3 节，BE-430）。
//
// 不复用 tianting-console-server/internal/objstore（另一个模块的 internal 包、强制 Object Lock、没有预签名），
// 按它的先例自己实现 SigV4，不引入 SDK：依赖越少，供应链风险越小。能力：预签名 PUT（签入 Content-Length、Content-Type）、
// 预签名 GET（签入 response-*）、HEAD、带 If-Match 的 Range GET、条件 CopyObject（元数据和标签替换）、DELETE、按前缀 List。
//
// **签名一律用真实时钟**（第 4.4 节）：对象存储按真实时间判断过期和时间偏差，推进 [IM 钟] 不影响签名。
// 错误里只有状态码和 S3 错误名，不带地址、请求头和签名：预签名查询串不能进日志（AC-FILE-04-7）。
package objstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config 是对象存储的配置。私有桶放上传和消息文件，公开读桶只放已绑定的头像（F-ACC-06 规则 4）。
type Config struct {
	Endpoint     string // https://s3.example.internal，只支持路径风格地址
	Region       string // 默认 us-east-1
	Bucket       string // 私有桶
	PublicBucket string // 公开读桶（头像）
	// PublicBaseURL 是公开桶的独立域名（https），头像地址 = PublicBaseURL + "/" + key；为空时用 Endpoint/PublicBucket。
	PublicBaseURL string
	AccessKey     string
	SecretKey     string
	// Timeout 是单次请求的超时（不含预签名，预签名不访问存储）。默认 30 秒。
	Timeout time.Duration
	HTTP    *http.Client
}

// Client 是对象存储客户端。
type Client struct {
	c    Config
	http *http.Client
}

// New 创建客户端。
func New(c Config) *Client {
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	c.Endpoint = strings.TrimRight(c.Endpoint, "/")
	c.PublicBaseURL = strings.TrimRight(c.PublicBaseURL, "/")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: c.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &Client{c: c, http: hc}
}

// Bucket 是私有桶的名字。
func (c *Client) Bucket() string { return c.c.Bucket }

// PublicBucket 是公开读桶的名字。
func (c *Client) PublicBucket() string { return c.c.PublicBucket }

// PublicURL 是公开桶里 key 的地址（users.avatar）。
func (c *Client) PublicURL(key string) string {
	if c.c.PublicBaseURL != "" {
		return c.c.PublicBaseURL + EscapePath("/"+key)
	}
	return c.c.Endpoint + EscapePath("/"+c.c.PublicBucket+"/"+key)
}

// 错误。
var (
	ErrNotFound     = errors.New("object store: not found")
	ErrPrecondition = errors.New("object store: precondition failed")
)

// Error 是对象存储返回的其他错误。只有状态码和错误名。
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return fmt.Sprintf("object store: %d %s", e.Status, e.Code) }

// UnavailableError 是网络错误（拒绝连接、超时）。不带地址。
type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return "object store unavailable: " + e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }

// IsUnavailable 判断是否应该按"对象存储不可用"处理（503 / 5032，可重试）：网络错误和 5xx。
func IsUnavailable(err error) bool {
	var u *UnavailableError
	if errors.As(err, &u) {
		return true
	}
	var e *Error
	return errors.As(err, &e) && e.Status >= 500
}

// Presigned 是一个预签名请求：Headers 已经签入，客户端必须原样带上。
type Presigned struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expiresAt"` // 毫秒，真实时钟
}

// PresignPut 预签名私有桶 key 的 PUT，签入 Content-Length 和 Content-Type（F-FILE-01 规则 1、4）。
// 存储按签名拒绝大小或类型不同的 PUT；没有签入的 x-amz-* 头会让签名不通过或被丢弃（AC-FILE-01-21）。
func (c *Client) PresignPut(key string, size int64, contentType string, ttl time.Duration) Presigned {
	now := time.Now().UTC()
	headers := map[string]string{"content-length": strconv.FormatInt(size, 10), "content-type": contentType}
	u := c.presign(http.MethodPut, c.c.Bucket, key, ttl, headers, nil, now)
	return Presigned{URL: u, Headers: map[string]string{"Content-Length": headers["content-length"], "Content-Type": contentType}, ExpiresAt: now.Add(ttl).UnixMilli()}
}

// PresignGet 预签名私有桶 key 的 GET，签入 response-content-type 和 response-content-disposition（F-FILE-03 规则 4）。
func (c *Client) PresignGet(key string, ttl time.Duration, contentType, disposition string) (string, int64) {
	now := time.Now().UTC()
	q := url.Values{}
	if contentType != "" {
		q.Set("response-content-type", contentType)
	}
	if disposition != "" {
		q.Set("response-content-disposition", disposition)
	}
	return c.presign(http.MethodGet, c.c.Bucket, key, ttl, nil, q, now), now.Add(ttl).UnixMilli()
}

func (c *Client) presign(method, bucket, key string, ttl time.Duration, headers map[string]string, extra url.Values, now time.Time) string {
	path := "/" + bucket + "/" + key
	u, _ := url.Parse(c.c.Endpoint)
	signed := map[string]string{"host": u.Host}
	for k, v := range headers {
		signed[k] = v
	}
	names := sortedKeys(signed)
	date := now.Format("20060102")
	q := url.Values{}
	for k, vs := range extra {
		q[k] = vs
	}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", c.c.AccessKey+"/"+date+"/"+c.c.Region+"/s3/aws4_request")
	q.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	q.Set("X-Amz-Expires", strconv.Itoa(int(ttl/time.Second)))
	q.Set("X-Amz-SignedHeaders", strings.Join(names, ";"))
	sig := Signature(SignInput{
		Method: method, Path: path, Query: q, Headers: signed, PayloadSHA: UnsignedPayload,
		Region: c.c.Region, Service: "s3", SecretKey: []byte(c.c.SecretKey), Time: now,
	})
	return c.c.Endpoint + EscapePath(path) + "?" + CanonicalQuery(q) + "&X-Amz-Signature=" + sig
}

// Info 是 HEAD 的结果。
type Info struct {
	Size        int64
	ETag        string
	ContentType string
}

// Head 返回私有桶 key 的大小、ETag 和类型。不存在：ErrNotFound。
func (c *Client) Head(ctx context.Context, key string) (Info, error) {
	resp, err := c.do(ctx, http.MethodHead, c.c.Bucket, key, nil, nil, nil)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	return Info{Size: resp.ContentLength, ETag: resp.Header.Get("ETag"), ContentType: resp.Header.Get("Content-Type")}, nil
}

// GetRange 读私有桶 key 的前 n 字节，带 If-Match（F-FILE-01 规则 3 ②）。ETag 不符：ErrPrecondition。
func (c *Client) GetRange(ctx context.Context, key string, n int64, ifMatch string) ([]byte, error) {
	h := map[string]string{"range": "bytes=0-" + strconv.FormatInt(n-1, 10)}
	if ifMatch != "" {
		h["if-match"] = ifMatch
	}
	resp, err := c.do(ctx, http.MethodGet, c.c.Bucket, key, nil, h, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, n))
	if err != nil {
		return nil, &UnavailableError{Err: errors.New("read body")}
	}
	return b, nil
}

// Copy 把私有桶的 srcKey 复制到 dstBucket 的 dstKey：带 x-amz-copy-source-if-match（ifMatch 非空时），
// 元数据和标签都 REPLACE，只写 contentType（F-FILE-01 规则 3 ③，RS4-17、RS5-21）。返回新对象的 ETag（不假设和源相同，RS4-18）。
func (c *Client) Copy(ctx context.Context, srcKey, dstBucket, dstKey, ifMatch, contentType string) (string, error) {
	h := map[string]string{
		"x-amz-copy-source":        EscapePath("/" + c.c.Bucket + "/" + srcKey),
		"x-amz-metadata-directive": "REPLACE",
		"x-amz-tagging-directive":  "REPLACE",
		"content-type":             contentType,
	}
	if ifMatch != "" {
		h["x-amz-copy-source-if-match"] = ifMatch
	}
	resp, err := c.do(ctx, http.MethodPut, dstBucket, dstKey, nil, h, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var out struct {
		XMLName xml.Name `xml:"CopyObjectResult"`
		ETag    string   `xml:"ETag"`
		Code    string   `xml:"Code"`
	}
	// S3 的 Copy 可能在 200 里返回错误（复制过程中失败），要看响应体。
	if err := xml.Unmarshal(data, &out); err != nil || out.ETag == "" {
		var e struct {
			Code string `xml:"Code"`
		}
		_ = xml.Unmarshal(data, &e)
		return "", &Error{Status: http.StatusInternalServerError, Code: cmpOr(e.Code, "InvalidCopyResponse")}
	}
	return out.ETag, nil
}

// Delete 删除 bucket 的 key。已经不存在算成功。
func (c *Client) Delete(ctx context.Context, bucket, key string) error {
	resp, err := c.do(ctx, http.MethodDelete, bucket, key, nil, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// List 列出私有桶 prefix 下的全部键，自动翻页。服务端凭证只有最终前缀的列举权限（第 11.3 节），全桶列举不给 im-server。
func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	var out []string
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		resp, err := c.do(ctx, http.MethodGet, c.c.Bucket, "", q, nil, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
			Contents              []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&page)
		_ = resp.Body.Close()
		if err != nil {
			return nil, &Error{Status: http.StatusBadGateway, Code: "InvalidListResponse"}
		}
		for _, k := range page.Contents {
			out = append(out, k.Key)
		}
		if !page.IsTruncated {
			return out, nil
		}
		if page.NextContinuationToken == "" {
			return nil, &Error{Status: http.StatusBadGateway, Code: "TruncatedWithoutToken"}
		}
		token = page.NextContinuationToken
	}
}

// DeletePrefix 按前缀列举再逐个删除（S3 风格的接口没有按前缀删除，第 11.3 节）。返回删除的个数。
func (c *Client) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	keys, err := c.List(ctx, prefix)
	if err != nil {
		return 0, err
	}
	for i, k := range keys {
		if err := c.Delete(ctx, c.c.Bucket, k); err != nil {
			return i, err
		}
	}
	return len(keys), nil
}

// UnsignedPayload 是预签名请求的负载哈希。
const UnsignedPayload = "UNSIGNED-PAYLOAD"

const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// do 发一个用请求头签名的请求。2xx 返回响应（调用方关闭）；404 → ErrNotFound；412 → ErrPrecondition；其他 → *Error；网络错误 → *UnavailableError。
func (c *Client) do(ctx context.Context, method, bucket, key string, query url.Values, headers map[string]string, body []byte) (*http.Response, error) {
	path := "/" + bucket
	if key != "" {
		path += "/" + key
	}
	target := c.c.Endpoint + EscapePath(path)
	if len(query) > 0 {
		target += "?" + CanonicalQuery(query)
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, &Error{Status: 0, Code: "InvalidRequest"}
	}
	payload := emptySHA256
	if body != nil {
		sum := sha256.Sum256(body)
		payload = hex.EncodeToString(sum[:])
	}
	now := time.Now().UTC()
	signed := map[string]string{"host": req.URL.Host, "x-amz-date": now.Format("20060102T150405Z"), "x-amz-content-sha256": payload}
	for k, v := range headers {
		signed[k] = v
	}
	for k, v := range signed {
		if k != "host" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Authorization", AuthHeader(SignInput{
		Method: method, Path: path, Query: query, Headers: signed, PayloadSHA: payload,
		Region: c.c.Region, Service: "s3", AccessKey: c.c.AccessKey, SecretKey: []byte(c.c.SecretKey), Time: now,
	}))
	resp, err := c.http.Do(req)
	if err != nil {
		// 只留原因，不带地址（url.Error 的字符串里有完整地址）。
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			err = errors.New("timeout")
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.New("canceled")
		} else {
			err = errors.New("connection failed")
		}
		return nil, &UnavailableError{Err: err}
	}
	if resp.StatusCode/100 == 2 {
		return resp, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusPreconditionFailed:
		return nil, ErrPrecondition
	}
	var e struct {
		Code string `xml:"Code"`
	}
	_ = xml.Unmarshal(data, &e)
	return nil, &Error{Status: resp.StatusCode, Code: e.Code}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// SignInput 是一次 SigV4 签名的输入。Path 没有转义；Headers 的键是小写。
type SignInput struct {
	Method, Path    string
	Query           url.Values // 预签名时含 X-Amz-*，不含 X-Amz-Signature
	Headers         map[string]string
	PayloadSHA      string
	Region, Service string
	AccessKey       string
	SecretKey       []byte
	Time            time.Time
}

// Signature 返回 SigV4 签名（十六进制）。S3 的规则：路径只转义一次。
func Signature(in SignInput) string {
	names := sortedKeys(in.Headers)
	var canon strings.Builder
	for _, k := range names {
		canon.WriteString(k + ":" + strings.TrimSpace(in.Headers[k]) + "\n")
	}
	canonical := strings.Join([]string{in.Method, EscapePath(in.Path), CanonicalQuery(in.Query), canon.String(), strings.Join(names, ";"), in.PayloadSHA}, "\n")
	date := in.Time.UTC().Format("20060102")
	scope := date + "/" + in.Region + "/" + in.Service + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + in.Time.UTC().Format("20060102T150405Z") + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	key := hmacSHA256(append([]byte("AWS4"), in.SecretKey...), date)
	key = hmacSHA256(key, in.Region)
	key = hmacSHA256(key, in.Service)
	key = hmacSHA256(key, "aws4_request")
	return hex.EncodeToString(hmacSHA256(key, toSign))
}

// AuthHeader 返回 Authorization 头。
func AuthHeader(in SignInput) string {
	scope := in.Time.UTC().Format("20060102") + "/" + in.Region + "/" + in.Service + "/aws4_request"
	return "AWS4-HMAC-SHA256 Credential=" + in.AccessKey + "/" + scope + ", SignedHeaders=" + strings.Join(sortedKeys(in.Headers), ";") + ", Signature=" + Signature(in)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hmacSHA256(key []byte, msg string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

// EscapePath 按 SigV4 规则转义路径：只保留非保留字符和 /。
func EscapePath(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if c := p[i]; c == '/' {
			b.WriteByte(c)
		} else {
			b.WriteString(escapeByte(c))
		}
	}
	return b.String()
}

func escapeByte(c byte) string {
	if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
		return string(c)
	}
	return "%" + strings.ToUpper(strconv.FormatInt(int64(c)+256, 16)[1:])
}

// CanonicalQuery 是按键排序、严格转义的查询串。
func CanonicalQuery(q url.Values) string {
	parts := make([]string, 0, len(q))
	for k, vs := range q {
		for _, v := range vs {
			parts = append(parts, escapeQuery(k)+"="+escapeQuery(v))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func escapeQuery(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteString(escapeByte(s[i]))
	}
	return b.String()
}
