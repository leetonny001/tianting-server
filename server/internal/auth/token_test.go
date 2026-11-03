package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/clock"
)

func newSigner(t *testing.T, secret string, clk clock.Clock) *Signer {
	t.Helper()
	s, err := NewSigner(secret, clk)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIssueAndVerify(t *testing.T) {
	clk := clock.New()
	at := time.UnixMilli(1_700_000_000_123)
	clk.Freeze(at)
	s := newSigner(t, "0123456789abcdef", clk)

	tok, exp, err := s.Issue("u1", 3, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(at.Add(time.Hour)) {
		t.Errorf("exp = %v", exp)
	}
	c, err := s.Verify(tok)
	if err != nil || c.UserID != "u1" {
		t.Fatalf("Verify = %+v, %v", c, err)
	}
	if c.Version != 3 {
		t.Errorf("Version = %d, want 3", c.Version)
	}
	if !c.IssuedAt.Equal(at) {
		t.Errorf("IssuedAt = %v, want millisecond precision %v", c.IssuedAt, at)
	}
}

func TestVerifyDistinguishesExpired(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_700_000_000_000))
	s := newSigner(t, "0123456789abcdef", clk)

	tok, _, _ := s.Issue("u1", 0, 2*time.Second)
	if _, err := s.Verify(tok); err != nil {
		t.Fatalf("fresh token: %v", err)
	}
	clk.Freeze(time.UnixMilli(1_700_000_003_000)) // 时间前进 3 秒
	c, err := s.Verify(tok)
	if err != ErrExpiredToken {
		t.Fatalf("err = %v, want ErrExpiredToken", err)
	}
	if c.UserID != "u1" || c.IssuedAt.IsZero() {
		t.Errorf("expired token must still return claims, got %+v", c)
	}
}

func TestVerifyRejects(t *testing.T) {
	s := newSigner(t, "0123456789abcdef", clock.New())
	other := newSigner(t, "fedcba9876543210", clock.New())
	foreign, _, _ := other.Issue("u1", 0, time.Hour)

	for name, tok := range map[string]string{
		"wrong secret":   foreign,
		"garbage":        "not-a-jwt",
		"empty":          "",
		"alg none token": "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1MSIsImlzcyI6ImltLXNlcnZlciJ9.",
	} {
		if _, err := s.Verify(tok); err != ErrInvalidToken {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func TestNewSignerRejectsShortSecret(t *testing.T) {
	if _, err := NewSigner("short", clock.New()); err == nil {
		t.Error("expected error for short secret")
	}
}

// BE-067：密钥轮换。新密钥签发后旧 token 仍然有效；删除旧密钥后旧 token 失效；没有 kid 的 token 无效。
func TestKeyRotation(t *testing.T) {
	clk := clock.New()
	k1 := Key{ID: "k1", Secret: []byte("secret-one-0123456789")}
	k2 := Key{ID: "k2", Secret: []byte("secret-two-0123456789")}

	old, err := NewKeyringSigner([]Key{k1}, "k1", clk)
	if err != nil {
		t.Fatal(err)
	}
	oldTok, _, _ := old.Issue("u1", 0, time.Hour)

	// 第 2 步：两把密钥都在，当前密钥切到 k2。
	both, err := NewKeyringSigner([]Key{k1, k2}, "k2", clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := both.Verify(oldTok); err != nil {
		t.Errorf("token signed by k1 must still verify during rotation: %v", err)
	}
	newTok, _, _ := both.Issue("u1", 0, time.Hour)
	if _, err := old.Verify(newTok); err != ErrInvalidToken {
		t.Errorf("node without k2 must reject k2 token, got %v", err)
	}

	// 第 3 步：删除 k1。
	only2, _ := NewKeyringSigner([]Key{k2}, "k2", clk)
	if _, err := only2.Verify(newTok); err != nil {
		t.Errorf("k2 token: %v", err)
	}
	if _, err := only2.Verify(oldTok); err != ErrInvalidToken {
		t.Errorf("k1 token after k1 removed: %v, want ErrInvalidToken", err)
	}

	// kid 被篡改成另一把密钥的 ID：签名对不上。
	if _, err := both.Verify(swapKid(t, newTok, "k1")); err != ErrInvalidToken {
		t.Errorf("tampered kid: %v", err)
	}
	// 没有 kid。
	if _, err := both.Verify(swapKid(t, newTok, "")); err != ErrInvalidToken {
		t.Errorf("missing kid: %v", err)
	}
}

func TestKeyringValidation(t *testing.T) {
	good := Key{ID: "k1", Secret: []byte("0123456789abcdef")}
	for name, tc := range map[string]struct {
		keys   []Key
		active string
	}{
		"short":          {[]Key{{ID: "k1", Secret: []byte("short")}}, "k1"},
		"empty id":       {[]Key{{ID: "", Secret: good.Secret}}, ""},
		"duplicate":      {[]Key{good, good}, "k1"},
		"missing active": {[]Key{good}, "k9"},
	} {
		if _, err := NewKeyringSigner(tc.keys, tc.active, clock.New()); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

// swapKid 改写 token 头部的 kid（签名不变）。
func swapKid(t *testing.T, tok, kid string) string {
	t.Helper()
	parts := strings.SplitN(tok, ".", 3)
	hdr := map[string]any{"alg": "HS256", "typ": "JWT"}
	if kid != "" {
		hdr["kid"] = kid
	}
	b, _ := json.Marshal(hdr)
	return base64.RawURLEncoding.EncodeToString(b) + "." + parts[1] + "." + parts[2]
}
