package quarantine

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSealOpen(t *testing.T) {
	c, err := New(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	n1, ct1, kid, err := c.Seal("m1", "group:g1", []byte("ZXQ-QUAR-321"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct1, []byte("ZXQ-QUAR-321")) {
		t.Fatal("plaintext in ciphertext")
	}
	got, err := c.Open("m1", "group:g1", n1, ct1, kid)
	if err != nil || string(got) != "ZXQ-QUAR-321" {
		t.Fatalf("open = %q, %v", got, err)
	}
	n2, _, _, _ := c.Seal("m1", "group:g1", []byte("ZXQ-QUAR-321"))
	if bytes.Equal(n1, n2) {
		t.Fatal("nonce reused")
	}
	// AAD：密文搬到别的消息、别的会话上解不开。
	if _, err := c.Open("m2", "group:g1", n1, ct1, kid); err == nil {
		t.Fatal("opened with another msgId")
	}
	if _, err := c.Open("m1", "group:g2", n1, ct1, kid); err == nil {
		t.Fatal("opened with another convId")
	}
	// 换密钥：明确报错。
	other, _ := New(strings.Repeat("o", 32))
	if _, err := other.Open("m1", "group:g1", n1, ct1, kid); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("other key: %v", err)
	}
}

func TestShortKeyRejected(t *testing.T) {
	if _, err := New(strings.Repeat("k", 31)); err == nil {
		t.Fatal("31 bytes accepted")
	}
}
