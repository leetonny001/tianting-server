package auth

import (
	"testing"
	"time"
)

func TestIssueAndVerify(t *testing.T) {
	s, err := NewSigner("0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	tok, exp, err := s.Issue("u1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 59*time.Minute {
		t.Errorf("exp too early: %v", exp)
	}
	uid, err := s.Verify(tok)
	if err != nil || uid != "u1" {
		t.Fatalf("Verify = %q, %v", uid, err)
	}
}

func TestVerifyRejects(t *testing.T) {
	s, _ := NewSigner("0123456789abcdef")
	other, _ := NewSigner("fedcba9876543210")

	expired, _, _ := s.Issue("u1", -time.Minute)
	foreign, _, _ := other.Issue("u1", time.Hour)

	for name, tok := range map[string]string{
		"expired":        expired,
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
	if _, err := NewSigner("short"); err == nil {
		t.Error("expected error for short secret")
	}
}
