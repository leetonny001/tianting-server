package adminauth

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// Request 是参与签名的请求字段（PRD-admin F-KEY-03 规则 2）。
type Request struct {
	Method     string
	Path       string // 转义后的路径（r.URL.EscapedPath()），不含查询串
	RawQuery   string
	Timestamp  string // 毫秒，十进制
	Nonce      string
	Body       []byte
	Operator   string // X-IM-Operator，没有为空串
	RequestID  string // X-Request-Id，没有为空串
	TargetNode string // X-IM-Target-Node，没有为空串
}

// StringToSign 返回待签名串：8 行，"\n" 分隔，最后一行后面没有换行。
//
//	METHOD
//	PATH[?按键名排序、RFC 3986 编码的查询串]
//	TIMESTAMP
//	NONCE
//	hex(SHA-256(body))
//	X-IM-Operator
//	X-Request-Id
//	X-IM-Target-Node
func StringToSign(r Request) string {
	path := r.Path
	if q := CanonicalQuery(r.RawQuery); q != "" {
		path += "?" + q
	}
	sum := sha256.Sum256(r.Body)
	return strings.Join([]string{
		strings.ToUpper(r.Method), path, r.Timestamp, r.Nonce, hex.EncodeToString(sum[:]),
		r.Operator, r.RequestID, r.TargetNode,
	}, "\n")
}

// CanonicalQuery 把查询串规范化：先解码，再按键名（相同键按值）排序，键和值都按 RFC 3986 编码
// （只保留 A-Z a-z 0-9 - . _ ~，其余 %XX 大写），用 & 连接。客户端发送时的参数顺序和编码风格不影响签名。
func CanonicalQuery(raw string) string {
	if raw == "" {
		return ""
	}
	type kv struct{ k, v string }
	var pairs []kv
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		dk, err1 := url.QueryUnescape(k)
		dv, err2 := url.QueryUnescape(v)
		if err1 != nil || err2 != nil {
			dk, dv = k, v // 无法解码的保留原样：签名会对不上，由调用方修正
		}
		pairs = append(pairs, kv{dk, dv})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = rfc3986(p.k) + "=" + rfc3986(p.v)
	}
	return strings.Join(out, "&")
}

func rfc3986(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&15])
	}
	return b.String()
}

// Verify 校验签名（base64url，无填充；带填充也接受）。HMAC 用常数时间比较。
func (c *Caller) Verify(stringToSign string, signature string) bool {
	sig, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(signature, "="))
	if err != nil {
		return false
	}
	switch c.Alg {
	case AlgHMAC:
		return hmac.Equal(sig, SignHMAC(c.Secret, stringToSign))
	case AlgEd25519:
		return len(c.PublicKey) == ed25519.PublicKeySize && ed25519.Verify(c.PublicKey, []byte(stringToSign), sig)
	}
	return false
}

// SignHMAC 计算 HMAC-SHA256 签名（原始字节）。调用方和测试用。
func SignHMAC(secret []byte, stringToSign string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(stringToSign))
	return m.Sum(nil)
}

// EncodeSignature 把签名编码成请求头 X-IM-Signature 的格式（base64url，无填充）。
func EncodeSignature(sig []byte) string { return base64.RawURLEncoding.EncodeToString(sig) }
