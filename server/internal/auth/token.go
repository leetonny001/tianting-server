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

// MinSecretLen 是签名密钥的最小长度；生产环境另有更严格的要求（config 包，32 字节）。
const MinSecretLen = 16

// Key 是一把签名密钥。ID 写进 token 头部的 kid 字段，校验时据此选择密钥。
type Key struct {
	ID     string
	Secret []byte
}

// Signer 用当前密钥签发 token，用任意一把已配置的密钥校验（ARCH S5 密钥轮换）。
//
// 轮换步骤：① 新增密钥 k2，当前密钥仍是 k1（所有节点都能校验 k2 签的 token）；
// ② 当前密钥切到 k2；③ 等 k1 签发的 token 全部过期（最长 30 天）后删除 k1。全程用户不用重新登录。
type Signer struct {
	keys   map[string][]byte
	active string
	clock  clock.Clock
}

// DefaultKeyID 是只配置了一把密钥（IM_JWT_SECRET）时使用的 kid。
const DefaultKeyID = "k1"

// NewSigner 用单把密钥创建 Signer，kid 为 DefaultKeyID。
func NewSigner(secret string, clk clock.Clock) (*Signer, error) {
	return NewKeyringSigner([]Key{{ID: DefaultKeyID, Secret: []byte(secret)}}, DefaultKeyID, clk)
}

// NewKeyringSigner 用多把密钥创建 Signer，activeID 是签发新 token 用的密钥。
func NewKeyringSigner(keys []Key, activeID string, clk clock.Clock) (*Signer, error) {
	s := &Signer{keys: map[string][]byte{}, active: activeID, clock: clk}
	for _, k := range keys {
		if k.ID == "" {
			return nil, errors.New("jwt key id must not be empty")
		}
		if len(k.Secret) < MinSecretLen {
			return nil, fmt.Errorf("jwt key %q must be at least %d bytes", k.ID, MinSecretLen)
		}
		if _, dup := s.keys[k.ID]; dup {
			return nil, fmt.Errorf("duplicate jwt key id %q", k.ID)
		}
		s.keys[k.ID] = k.Secret
	}
	if _, ok := s.keys[activeID]; !ok {
		return nil, fmt.Errorf("active jwt key %q is not configured", activeID)
	}
	return s, nil
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
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t.Header["kid"] = s.active
	tok, err := t.SignedString(s.keys[s.active])
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
		s.keyFor,
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

// keyFor 按 token 头部的 kid 选择密钥；没有 kid 或 kid 未配置（例如已经轮换掉）都视为无效。
func (s *Signer) keyFor(t *jwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	key, ok := s.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	return key, nil
}
