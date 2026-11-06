package account

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// sidBytes 是 sid 的随机字节数：不少于 128 位（F-ACC-03 规则 1）。sid 不进日志、指标标签和后台响应。
const (
	sidBytes   = 16
	nonceBytes = 16
)

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func newSID() (string, error) { return randomToken(sidBytes) }

// SIDHash 是 account_events 里存的 sid 的哈希（RS4-05）。
func SIDHash(sid string) string {
	sum := sha256.Sum256([]byte("sid\x00" + sid))
	return hex.EncodeToString(sum[:])
}

// refreshCodec 生成和校验 refresh token：kid.sid.gen.nonce.mac（F-ACC-03 规则 3）。
// mac 用独立的 refresh 密钥对 kid、sid、gen、nonce 计算，不和 JWT 共用密钥（RS5-04）。
// nonce 每次随机，不用确定性派生（RS4-04）。判定和轮换在 refresh.go。
type refreshCodec struct {
	keys   map[string][]byte
	active string
}

func newRefreshCodec(keys []RefreshKey, active string) (refreshCodec, error) {
	c := refreshCodec{keys: map[string][]byte{}, active: active}
	for _, k := range keys {
		if k.ID == "" || strings.Contains(k.ID, ".") || len(k.Secret) == 0 {
			return refreshCodec{}, fmt.Errorf("refresh key %q: id must not contain a dot, secret must not be empty", k.ID)
		}
		c.keys[k.ID] = k.Secret
	}
	if _, ok := c.keys[active]; !ok {
		return refreshCodec{}, fmt.Errorf("active refresh key %q is not configured", active)
	}
	return c, nil
}

func refreshMAC(key []byte, kid, sid, gen, nonce string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(kid + "." + sid + "." + gen + "." + nonce))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (c refreshCodec) issue(sid string, gen int64) (string, error) {
	nonce, err := randomToken(nonceBytes)
	if err != nil {
		return "", err
	}
	g := strconv.FormatInt(gen, 10)
	return c.active + "." + sid + "." + g + "." + nonce + "." + refreshMAC(c.keys[c.active], c.active, sid, g, nonce), nil
}

// RefreshClaims 是 mac 校验通过的 refresh token 的内容。
type RefreshClaims struct {
	KID, SID string
	Gen      int64
}

// errRefreshMAC：格式不对、kid 不认识或 mac 错误，都是 4019 reason=invalid，不吊销，不计重放（RS4-05）。
var errRefreshMAC = errors.New("refresh token invalid")

// parse 先验 mac。只知道 sid 的人构造不出能通过这一步的值。
func (c refreshCodec) parse(token string) (RefreshClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 5 {
		return RefreshClaims{}, errRefreshMAC
	}
	kid, sid, g, nonce, mac := parts[0], parts[1], parts[2], parts[3], parts[4]
	key, ok := c.keys[kid]
	if !ok || !hmac.Equal([]byte(mac), []byte(refreshMAC(key, kid, sid, g, nonce))) {
		return RefreshClaims{}, errRefreshMAC
	}
	gen, err := strconv.ParseInt(g, 10, 64)
	if err != nil || gen < 1 {
		return RefreshClaims{}, errRefreshMAC
	}
	return RefreshClaims{KID: kid, SID: sid, Gen: gen}, nil
}

// refreshHash 是 device_sessions 里存的值：整个 token 的哈希（F-ACC-03 规则 3）。
func refreshHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// 恢复码（F-ACC-05 规则 1）：128 位随机，Crockford Base32（没有 I、L、O、U，不容易抄错），每 4 位一组。
var recoveryEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func newRecoveryCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := recoveryEncoding.EncodeToString(b)
	var out strings.Builder
	for i, r := range raw {
		if i > 0 && i%4 == 0 {
			out.WriteByte('-')
		}
		out.WriteRune(r)
	}
	return out.String(), nil
}

// normalizeRecoveryCode 规范化（F-ACC-05 规则 2）：大小写、去分隔符；容易抄错的字母按 Crockford 的规则映射。
func normalizeRecoveryCode(code string) string {
	return strings.NewReplacer("-", "", " ", "", "O", "0", "I", "1", "L", "1").Replace(strings.ToUpper(code))
}

// RecoveryHash 是 SHA-256(userId, code)，用长度前缀拼接（F-ACC-05 规则 1、2）。码有 128 位随机，不怕离线猜，不用 Argon2 和 HMAC。
func RecoveryHash(userID, code string) string {
	norm := normalizeRecoveryCode(code)
	h := sha256.New()
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(userID)))
	h.Write(n[:])
	h.Write([]byte(userID))
	binary.BigEndian.PutUint64(n[:], uint64(len(norm)))
	h.Write(n[:])
	h.Write([]byte(norm))
	return hex.EncodeToString(h.Sum(nil))
}

// InviteHash 是邀请码的确定性哈希（F-ACC-09 规则 1）：只存它和后 4 位。邀请码有 128 位随机。
func InviteHash(code string) string {
	sum := sha256.Sum256([]byte("invite\x00" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
