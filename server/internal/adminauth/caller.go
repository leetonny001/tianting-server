package adminauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 签名算法。
const (
	AlgHMAC    = "hmac-sha256"
	AlgEd25519 = "ed25519"
)

// LegacyCallerID 是旧密钥 IM_ADMIN_KEY 对应的调用方（PRD-admin F-KEY-05），配置文件里不能用这个 id。
const LegacyCallerID = "legacy"

const (
	minHMACSecretLen = 32
	maxPushTypeLen   = 64 // PRD-live F-PUSH-02 规则 2
)

var callerIDRe = regexp.MustCompile(`^[a-z0-9-]{3,32}$`)

// Caller 是一个管理接口调用方（业务服务端、console-server 等）。
type Caller struct {
	ID           string
	Alg          string
	Secret       []byte            // HMAC 密钥
	PublicKey    ed25519.PublicKey // Ed25519 公钥；im-server 只保存公钥
	Scopes       map[Scope]bool
	AllowedCIDRs []netip.Prefix // 为空表示不额外限制来源
	Disabled     bool
	BodyAccess   bool
	// PushTypes 非空时，这个调用方只能推送这些 type（PRD-live F-PUSH-09 规则 3）。
	PushTypes []string
}

// Has 报告调用方是否有该 scope。
func (c *Caller) Has(s Scope) bool { return c.Scopes[s] }

// callerJSON 是配置文件里一个调用方的格式。secret / publicKey 用 base64（标准或 URL 安全编码均可）。
type callerJSON struct {
	ID           string   `json:"id"`
	Alg          string   `json:"alg"`
	Secret       string   `json:"secret"`
	PublicKey    string   `json:"publicKey"`
	Scopes       []Scope  `json:"scopes"`
	AllowedCIDRs []string `json:"allowedCIDRs"`
	Disabled     bool     `json:"disabled"`
	BodyAccess   bool     `json:"bodyAccess"`
	PushTypes    []string `json:"pushTypes"`
}

// LoadCallersFile 读取并校验调用方配置文件（IM_ADMIN_CALLERS）。otherSecrets 是 JWT 密钥和旧管理密钥：
// 调用方的 HMAC 密钥不能与它们相同（PRD-admin F-KEY-02 规则 1）。
// 返回的错误列出全部问题，但从不包含密钥内容。
func LoadCallersFile(path string, otherSecrets ...[]byte) ([]Caller, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s: permissions %o are too open, must be 0600 (contains keys)", path, info.Mode().Perm())
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- 路径来自运维配置的环境变量 IM_ADMIN_CALLERS
	if err != nil {
		return nil, err
	}
	var items []callerJSON
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	return parseCallers(items, otherSecrets)
}

func parseCallers(items []callerJSON, otherSecrets [][]byte) ([]Caller, error) {
	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	seen := map[string]bool{}
	var callers []Caller
	bodyCallers := 0
	for i, it := range items {
		name := it.ID
		if !callerIDRe.MatchString(it.ID) || it.ID == LegacyCallerID {
			// 只报序号和格式要求：id 可能被误填成密钥。
			bad("caller #%d: id must match [a-z0-9-]{3,32} and must not be %q", i+1, LegacyCallerID)
			name = fmt.Sprintf("#%d", i+1)
		} else if seen[it.ID] {
			bad("caller %s: duplicate id", it.ID)
		}
		seen[it.ID] = true
		c := Caller{ID: it.ID, Alg: it.Alg, Scopes: map[Scope]bool{}, Disabled: it.Disabled, BodyAccess: it.BodyAccess, PushTypes: it.PushTypes}
		for _, typ := range it.PushTypes {
			if n := utf8.RuneCountInString(typ); n < 1 || n > maxPushTypeLen {
				bad("caller %s: pushTypes entries must be 1..%d characters", name, maxPushTypeLen)
			}
		}
		switch it.Alg {
		case AlgHMAC:
			key, err := decodeB64(it.Secret)
			switch {
			case err != nil:
				bad("caller %s: secret must be base64", name)
			case len(key) < minHMACSecretLen:
				bad("caller %s: hmac secret must be at least %d bytes", name, minHMACSecretLen)
			}
			if it.PublicKey != "" {
				bad("caller %s: hmac-sha256 callers use secret, not publicKey", name)
			}
			c.Secret = key
		case AlgEd25519:
			key, err := decodeB64(it.PublicKey)
			if err != nil || len(key) != ed25519.PublicKeySize {
				bad("caller %s: publicKey must be a base64 %d-byte Ed25519 public key", name, ed25519.PublicKeySize)
			}
			if it.Secret != "" {
				bad("caller %s: ed25519 callers must not carry a secret (im-server keeps only the public key)", name)
			}
			c.PublicKey = key
		default:
			bad("caller %s: alg must be %q or %q", name, AlgHMAC, AlgEd25519)
		}
		hasBody, hasOther := false, false
		for _, s := range it.Scopes {
			if !knownScope(s) {
				bad("caller %s: unknown scope %q", name, s)
				continue
			}
			c.Scopes[s] = true
			if IsBodyScope(s) {
				hasBody = true
			} else {
				hasOther = true
			}
		}
		if hasBody || it.BodyAccess {
			bodyCallers++
		}
		switch {
		case hasBody && !it.BodyAccess:
			bad("caller %s: message body scopes are only allowed on a separate caller with bodyAccess: true", name)
		case hasBody && hasOther:
			bad("caller %s: message body scopes are only allowed on a separate bodyAccess caller with no other scopes", name)
		}
		for _, s := range it.AllowedCIDRs {
			p, err := parsePrefix(s)
			if err != nil {
				bad("caller %s: allowedCIDRs %q is not an IP or CIDR", name, s)
				continue
			}
			c.AllowedCIDRs = append(c.AllowedCIDRs, p)
		}
		callers = append(callers, c)
	}
	if bodyCallers > 1 {
		bad("at most one caller may have message body access (bodyAccess / msg.body.* / msg.quarantine), found %d", bodyCallers)
	}
	// 密钥两两不同，也不能与 JWT 密钥、旧管理密钥相同。
	for i := range callers {
		if callers[i].Secret == nil {
			continue
		}
		for j := i + 1; j < len(callers); j++ {
			if callers[j].Secret != nil && bytes.Equal(callers[i].Secret, callers[j].Secret) {
				bad("callers %s and %s must not share the same secret", callers[i].ID, callers[j].ID)
			}
		}
		for _, o := range otherSecrets {
			if len(o) > 0 && bytes.Equal(callers[i].Secret, o) {
				bad("caller %s: secret must differ from IM_JWT_SECRET / IM_ADMIN_KEY", callers[i].ID)
			}
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return callers, nil
}

// MissingPushTypes 返回持有 push.all、却没有配非空 pushTypes 的调用方 ID（F-PUSH-09 规则 3）。
func MissingPushTypes(callers []Caller) []string {
	var out []string
	for i := range callers {
		if callers[i].Has(ScopePushAll) && len(callers[i].PushTypes) == 0 {
			out = append(out, callers[i].ID)
		}
	}
	return out
}

func decodeB64(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		return p.Masked(), err
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}
