// Package auth 签发和校验用户 token（HS256 JWT），对应腾讯 IM 的 userSig。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "im-server"

// ErrInvalidToken 表示 token 无效、过期或签名不对。
var ErrInvalidToken = errors.New("invalid token")

// Signer 用同一个密钥签发和校验 token。
type Signer struct {
	secret []byte
	now    func() time.Time
}

// NewSigner 创建 Signer。secret 至少 16 字节。
func NewSigner(secret string) (*Signer, error) {
	if len(secret) < 16 {
		return nil, errors.New("jwt secret must be at least 16 bytes")
	}
	return &Signer{secret: []byte(secret), now: time.Now}, nil
}

// Issue 为用户签发 token，返回 token 和过期时间。
func (s *Signer) Issue(userID string, ttl time.Duration) (string, time.Time, error) {
	now := s.now()
	exp := now.Add(ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return tok, exp, nil
}

// Verify 校验 token，返回用户 ID。
func (s *Signer) Verify(token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
