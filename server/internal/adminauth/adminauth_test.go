package adminauth

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func secret(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

var (
	secretA = secret("caller-a-secret-0123456789abcdef!!")
	secretB = secret("caller-b-secret-0123456789abcdef!!")
	pubKey  = base64.StdEncoding.EncodeToString(make([]byte, 32))
)

func TestParseCallers(t *testing.T) {
	ok := []callerJSON{
		{ID: "biz1", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopeTokenIssue, ScopeUserRead}},
		{ID: "console", Alg: AlgEd25519, PublicKey: pubKey, Scopes: []Scope{ScopeUserRead, ScopeMsgMeta}, AllowedCIDRs: []string{"10.0.0.0/8"}},
		{ID: "console-msgread", Alg: AlgEd25519, PublicKey: pubKey, BodyAccess: true, Scopes: []Scope{ScopeMsgBodyGroup, ScopeMsgQuarantine}},
	}
	callers, err := parseCallers(ok, nil)
	if err != nil || len(callers) != 3 || !callers[0].Has(ScopeTokenIssue) || callers[0].Has(ScopeGroupWrite) || len(callers[1].AllowedCIDRs) != 1 {
		t.Fatalf("callers=%+v err=%v", callers, err)
	}

	for name, tc := range map[string]struct {
		items []callerJSON
		other [][]byte
		want  string
	}{
		// PRD-live F-PUSH-02 规则 2：pushTypes 的每一项都是合法的 type。
		"empty push type":    {[]callerJSON{{ID: "biz1", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopePushAll}, PushTypes: []string{""}}}, nil, "pushTypes"},
		"too long push type": {[]callerJSON{{ID: "biz1", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopePushAll}, PushTypes: []string{strings.Repeat("t", 65)}}}, nil, "pushTypes"},
		// AC-KEY-02-4：正文类 scope 和其他 scope 在同一个调用方上。
		"body scope mixed":              {[]callerJSON{{ID: "console", Alg: AlgEd25519, PublicKey: pubKey, BodyAccess: true, Scopes: []Scope{ScopeMsgMeta, ScopeMsgBodyGroup}}}, nil, "no other scopes"},
		"body scope without bodyAccess": {[]callerJSON{{ID: "console", Alg: AlgEd25519, PublicKey: pubKey, Scopes: []Scope{ScopeMsgBodyGroup}}}, nil, "bodyAccess: true"},
		"two body callers": {[]callerJSON{
			{ID: "r1", Alg: AlgEd25519, PublicKey: pubKey, BodyAccess: true, Scopes: []Scope{ScopeMsgBodyGroup}},
			{ID: "r2", Alg: AlgEd25519, PublicKey: pubKey, BodyAccess: true, Scopes: []Scope{ScopeMsgQuarantine}},
		}, nil, "at most one caller"},
		// AC-KEY-02-5：两个调用方同一把 HMAC 密钥；密钥等于 JWT 密钥。
		"shared secret":       {[]callerJSON{{ID: "a1x", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopeUserRead}}, {ID: "b1x", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopeUserRead}}}, nil, "must not share"},
		"secret equals jwt":   {[]callerJSON{{ID: "a1x", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{ScopeUserRead}}}, [][]byte{[]byte("caller-a-secret-0123456789abcdef!!")}, "must differ"},
		"short secret":        {[]callerJSON{{ID: "a1x", Alg: AlgHMAC, Secret: secret("short"), Scopes: []Scope{ScopeUserRead}}}, nil, "at least 32 bytes"},
		"bad id":              {[]callerJSON{{ID: "Biz_1", Alg: AlgHMAC, Secret: secretA}}, nil, "id must match"},
		"reserved id":         {[]callerJSON{{ID: "legacy", Alg: AlgHMAC, Secret: secretA}}, nil, "id must match"},
		"duplicate id":        {[]callerJSON{{ID: "biz1", Alg: AlgHMAC, Secret: secretA}, {ID: "biz1", Alg: AlgHMAC, Secret: secretB}}, nil, "duplicate id"},
		"unknown scope":       {[]callerJSON{{ID: "biz1", Alg: AlgHMAC, Secret: secretA, Scopes: []Scope{"users.read"}}}, nil, "unknown scope"},
		"bad alg":             {[]callerJSON{{ID: "biz1", Alg: "rsa"}}, nil, "alg must be"},
		"ed25519 with secret": {[]callerJSON{{ID: "biz1", Alg: AlgEd25519, PublicKey: pubKey, Secret: secretA}}, nil, "must not carry a secret"},
		"bad public key":      {[]callerJSON{{ID: "biz1", Alg: AlgEd25519, PublicKey: secret("short")}}, nil, "32-byte"},
	} {
		_, err := parseCallers(tc.items, tc.other)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "caller-a-secret") {
			t.Errorf("%s: error leaks the secret: %v", name, err)
		}
	}
}

// 配置文件权限必须是 0600（不能被同组和其他用户读取）。
func TestLoadCallersFilePermissions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "callers.json")
	content := `[{"id":"biz1","alg":"hmac-sha256","secret":"` + secretA + `","scopes":["token.issue"]}]`
	os.WriteFile(p, []byte(content), 0o644)
	if _, err := LoadCallersFile(p); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Errorf("0644 file: err = %v", err)
	}
	os.Chmod(p, 0o600)
	if c, err := LoadCallersFile(p); err != nil || len(c) != 1 {
		t.Errorf("0600 file: %v %v", c, err)
	}
	os.WriteFile(p, []byte(`[{"id":"biz1","alg":"hmac-sha256","secret":"`+secretA+`","scope":["token.issue"]}]`), 0o600)
	if _, err := LoadCallersFile(p); err == nil {
		t.Error("unknown field accepted (typo would silently drop scopes)")
	}
}

// PRD-live F-PUSH-09 规则 3：配置文件带 pushTypes；push.room 在 scope 目录里；MissingPushTypes 只报持有 push.all 又没配的。
func TestCallerPushTypes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "callers.json")
	content := `[{"id":"biz1","alg":"hmac-sha256","secret":"` + secretA + `","scopes":["push.all","push.room"],"pushTypes":["goal"]},
		{"id":"biz2","alg":"hmac-sha256","secret":"` + secretB + `","scopes":["push.all"]},
		{"id":"biz3","alg":"hmac-sha256","secret":"` + secret("caller-c-secret-0123456789abcdef!!") + `","scopes":["push.user"]}]`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	callers, err := LoadCallersFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !callers[0].Has(ScopePushRoom) || len(callers[0].PushTypes) != 1 || callers[0].PushTypes[0] != "goal" {
		t.Errorf("biz1 = %+v", callers[0])
	}
	if got := MissingPushTypes(callers); len(got) != 1 || got[0] != "biz2" {
		t.Errorf("MissingPushTypes = %v", got)
	}
}

func TestCanonicalQuery(t *testing.T) {
	for raw, want := range map[string]string{
		"":                      "",
		"b=2&a=1":               "a=1&b=2",
		"q=a+b&q=%41":           "q=A&q=a%20b",
		"name=%E5%BC%A0&x=~._-": "name=%E5%BC%A0&x=~._-",
		"flag&z=":               "flag=&z=",
		"k=%2f":                 "k=%2F",
	} {
		if got := CanonicalQuery(raw); got != want {
			t.Errorf("CanonicalQuery(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestMemoryNonces(t *testing.T) {
	n := NewMemoryNonces(2)
	now := time.UnixMilli(1762156800000)
	if ok, _ := n.Add("c1", "n1", now); !ok {
		t.Fatal("first use rejected")
	}
	if ok, _ := n.Add("c1", "n1", now.Add(9*time.Minute)); ok {
		t.Error("replay within 10 minutes accepted")
	}
	if ok, _ := n.Add("c2", "n1", now); !ok {
		t.Error("same nonce for another caller rejected")
	}
	if _, err := n.Add("c1", "n2", now); err != ErrNonceStoreFull {
		t.Errorf("full store: err = %v", err)
	}
	// 过期后清理，空间可以复用。
	if ok, err := n.Add("c1", "n3", now.Add(11*time.Minute)); !ok || err != nil {
		t.Errorf("after expiry: ok=%v err=%v", ok, err)
	}
}
