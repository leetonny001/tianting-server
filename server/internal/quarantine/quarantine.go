// Package quarantine 加密被删消息的原文（PRD-admin F-ADM-32，BE-224）：AES-256-GCM，每条随机 nonce，AAD = msgId|convId，
// 这样密文搬到另一条消息上解不开。
package quarantine

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// MinKeyLen 是 IM_QUARANTINE_KEY 的最小长度（字节）。
const MinKeyLen = 32

// Cipher 持有派生好的密钥。
type Cipher struct {
	aead cipher.AEAD
	kid  string
}

// New 从配置的密钥派生 AES-256 密钥。kid 是密钥指纹：换密钥后，旧记录能报出"密钥不对"，而不是一个说不清的解密失败。
func New(key string) (*Cipher, error) {
	if len(key) < MinKeyLen {
		return nil, fmt.Errorf("quarantine key must be at least %d bytes", MinKeyLen)
	}
	derived := sha256.Sum256([]byte("tianting-quarantine-v1\x00" + key))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	fp := sha256.Sum256(append([]byte("tianting-quarantine-kid\x00"), derived[:]...))
	return &Cipher{aead: aead, kid: hex.EncodeToString(fp[:4])}, nil
}

// KID 返回密钥指纹。
func (c *Cipher) KID() string { return c.kid }

func aad(msgID, convID string) []byte { return []byte(msgID + "|" + convID) }

// Seal 加密一条消息的原文。
func (c *Cipher) Seal(msgID, convID string, body []byte) (nonce, ciphertext []byte, kid string, err error) {
	nonce = make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, "", err
	}
	return nonce, c.aead.Seal(nil, nonce, body, aad(msgID, convID)), c.kid, nil
}

// ErrWrongKey 表示记录是用另一把密钥加密的。
var ErrWrongKey = errors.New("quarantine record was sealed with a different key")

// Open 解密。
func (c *Cipher) Open(msgID, convID string, nonce, ciphertext []byte, kid string) ([]byte, error) {
	if kid != c.kid {
		return nil, ErrWrongKey
	}
	return c.aead.Open(nil, nonce, ciphertext, aad(msgID, convID))
}
