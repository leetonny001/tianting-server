// Package auth 签发和校验用户 token（HS256 JWT），对应腾讯 IM 的 userSig。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"im-mvp/server/internal/clock"
)

const issuer = "im-server"

// 校验错误：无效（签名、格式、缺失）与过期分开，对应关闭码 4010 / 4011。
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims 是 token 里携带的信息。
type Claims struct {
	UserID    string
	Version   int64     // 签发时用户的令牌版本号；小于当前版本说明已被强制下线
	IssuedAt  time.Time // 毫秒精度，仅供日志和排查
	ExpiresAt time.Time
}

type jwtClaims struct {
	jwt.RegisteredClaims
	IatMs   int64 `json:"iatMs"`
	Version int64 `json:"tv"`
}

// Signer 用同一个密钥签发和校验 token。
type Signer struct {
	secret []byte
	clock  clock.Clock
}

// NewSigner 创建 Signer。secret 至少 16 字节。
func NewSigner(secret string, clk clock.Clock) (*Signer, error) {
	if len(secret) < 16 {
		return nil, errors.New("jwt secret must be at least 16 bytes")
	}
	return &Signer{secret: []byte(secret), clock: clk}, nil
}

// Issue 为用户签发 token，version 为用户当前的令牌版本号；返回 token 和过期时间。
func (s *Signer) Issue(userID string, version int64, ttl time.Duration) (string, time.Time, error) {
	now := s.clock.Now()
	exp := now.Add(ttl)
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		IatMs:   now.UnixMilli(),
		Version: version,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return tok, exp, nil
}

// Verify 校验 token。签名有效但已过期时返回 ErrExpiredToken，**同时返回 Claims**
// （调用方需要先判断是否被强制下线，PRD C-08）；其余问题返回 ErrInvalidToken。
func (s *Signer) Verify(token string) (Claims, error) {
	var c jwtClaims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.clock.Now),
	)
	expired := errors.Is(err, jwt.ErrTokenExpired)
	if (err != nil && !expired) || c.Subject == "" || c.IatMs == 0 || c.ExpiresAt == nil {
		return Claims{}, ErrInvalidToken
	}
	claims := Claims{
		UserID:    c.Subject,
		Version:   c.Version,
		IssuedAt:  time.UnixMilli(c.IatMs),
		ExpiresAt: c.ExpiresAt.Time,
	}
	if expired {
		return claims, ErrExpiredToken
	}
	return claims, nil
}
