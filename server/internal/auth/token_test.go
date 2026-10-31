package auth

import (
	"testing"
	"time"

	"im-mvp/server/internal/clock"
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
