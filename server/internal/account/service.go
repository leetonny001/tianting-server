package account

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// Store 是账号功能用到的存储。
type Store interface {
	store.AccountStore
	TokenVersion(ctx context.Context, userID string) (int64, error)
	UserBan(ctx context.Context, userID string) (*store.Ban, error)
}

// Deps 是 Service 的依赖。
type Deps struct {
	Config Config
	Store  Store
	Signer *auth.Signer
	Router delivery.Router
	Clock  clock.Clock
	Faults *Faults // 生产环境为 nil
	Log    *slog.Logger
	// Logins、Registers 是 im_account_logins_total{result}、im_registers_total{result}（第 14.2 节）；InviteAutoDisabled 是自动停用的告警。
	Logins, Registers  *prometheus.CounterVec
	InviteAutoDisabled prometheus.Counter
}

// Service 实现注册和登录。
type Service struct {
	Deps
	cfg      Config
	gate     *Gate
	guard    *Guard
	refresh  refreshCodec
	reserved map[string]bool
	dummy    string // 不存在的账号也要校验的哈希（F-ACC-02 规则 2）
}

// New 创建账号服务。没有 refresh 密钥时账号接口返回 503，其他功能不受影响。
func New(d Deps) (*Service, error) {
	cfg := d.Config.WithDefaults()
	s := &Service{Deps: d, cfg: cfg, gate: NewGate(cfg, d.Faults), guard: NewGuard(d.Store, d.Clock, cfg.TestDisableLock, 0), reserved: reservedNames(cfg.ExtraReserved)}
	if len(cfg.RefreshKeys) == 0 {
		return s, nil
	}
	var err error
	if s.refresh, err = newRefreshCodec(cfg.RefreshKeys, cfg.RefreshActive); err != nil {
		return nil, err
	}
	if s.dummy, err = dummyHash(cfg.Argon); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) available() bool { return s.dummy != "" }

// ArgonBlock、ArgonBlocked、FailRegisterTx 是测试模式的故障注入（admin.AccountTest）。
func (s *Service) SetArgonBlock(on bool) { s.Faults.SetArgonBlock(on) }
func (s *Service) ArgonBlocked() int     { return s.Faults.ArgonBlocked() }
func (s *Service) FailRegisterTx(n int)  { s.Faults.FailRegisterTx(n) }

// ResetLimits 清"用户名 + IP"、IP、邀请码失败计数（/admin/test/limits/reset scope=auth）。
func (s *Service) ResetLimits(ctx context.Context) error { return s.Store.ResetThrottle(ctx) }

// PurgeEvents 是 account_events_purge：保留 180 天（第 12.1 节，未校准）。
func (s *Service) PurgeEvents(ctx context.Context) (int, error) {
	return s.Store.PurgeAccountEvents(ctx, s.Clock.Now().Add(-eventRetention).UnixMilli())
}

// PurgeThrottle 是 auth_throttle_purge：窗口和锁定都结束的行（第 12.1 节"定时清理"）。
func (s *Service) PurgeThrottle(ctx context.Context) (int, error) {
	return s.Store.PurgeThrottle(ctx, s.Clock.Now().UnixMilli())
}

const eventRetention = 180 * 24 * time.Hour

// AuthResponse 是注册、登录的响应（F-ACC-01 规则 8，F-ACC-02 规则 1）。
type AuthResponse struct {
	UserID            string `json:"userId"`
	AccessToken       string `json:"accessToken"`
	AccessExpiresAt   int64  `json:"accessExpiresAt"`
	RefreshToken      string `json:"refreshToken"`
	RefreshExpiresAt  int64  `json:"refreshExpiresAt"`
	AbsoluteExpiresAt int64  `json:"absoluteExpiresAt"`
	RecoveryCode      string `json:"recoveryCode,omitempty"`
	RecoveryCodeGen   int64  `json:"recoveryCodeGen,omitempty"`
}

// platforms 和 WS 的平台标识相同（PRD-core F-AUTH-02 规则 1）。
var platforms = map[string]bool{"android": true, "ios": true, "web": true, "pc": true}

// validateDevice 校验 platform 和 deviceName（F-ACC-01 规则 1）。deviceName 不进日志。
func validateDevice(platform, deviceName string) *protocol.Error {
	if !platforms[platform] {
		return protocol.Errorf(protocol.CodeBadRequest, "invalid platform")
	}
	if protocol.TextLen(deviceName) > DeviceNameMaxLen {
		return protocol.Errorf(protocol.CodeBadRequest, "deviceName too long")
	}
	return nil
}

// newDeviceSession 生成一条新会话和它的第一代 refresh token（F-ACC-03 规则 1、3）。代号从 1 开始，next_gen 从 2 开始。
func (s *Service) newDeviceSession(userID, platform, deviceName, ip string, now time.Time) (store.DeviceSession, string, error) {
	sid, err := newSID()
	if err != nil {
		return store.DeviceSession{}, "", err
	}
	token, err := s.refresh.issue(sid, 1)
	if err != nil {
		return store.DeviceSession{}, "", err
	}
	ttl := s.cfg.RefreshTTL
	if platform == "web" {
		ttl = s.cfg.RefreshTTLWeb // H5 放在 sessionStorage，所以更短（第 4.1 节）
	}
	absolute := now.Add(s.cfg.RefreshAbsolute).UnixMilli()
	ms := now.UnixMilli()
	return store.DeviceSession{
		SID: sid, UserID: userID, Platform: platform, DeviceName: deviceName, CurrentHash: refreshHash(token), CurrentGen: 1, NextGen: 2,
		SeenIPPrefixes: []string{clientip.EventPrefix(ip)}, CreatedAt: ms, LastSeenAt: ms,
		ExpiresAt: min(now.Add(ttl).UnixMilli(), absolute), AbsoluteExpiresAt: absolute,
	}, token, nil
}

// issue 签发 access token，组出响应。access 的到期时间不超过绝对上限（F-ACC-03 规则 3）。
func (s *Service) issue(d store.DeviceSession, version int64, refreshToken string, now time.Time) (*AuthResponse, error) {
	exp := now.Add(s.cfg.AccessTTL)
	if abs := time.UnixMilli(d.AbsoluteExpiresAt); exp.After(abs) {
		exp = abs
	}
	token, exp, err := s.Signer.IssueSession(d.UserID, version, d.SID, d.CurrentGen, exp)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{UserID: d.UserID, AccessToken: token, AccessExpiresAt: exp.UnixMilli(), RefreshToken: refreshToken,
		RefreshExpiresAt: d.ExpiresAt, AbsoluteExpiresAt: d.AbsoluteExpiresAt}, nil
}

// notify 给账号的其他在线设备下发 securityNotice（F-ACC-02 规则 3b）。尽力而为，事件已经写进 account_events。
func (s *Service) notify(userID, kind, platform string, at int64) {
	b, err := json.Marshal(protocol.Event(protocol.SecurityNoticeEvent{Type: protocol.EventSecurityNotice, Kind: kind, At: at, Platform: platform}))
	if err != nil {
		return
	}
	s.Router.DeliverToUser(userID, b, "")
}

// gateErr 把闸门的错误映射成响应。
func gateErr(err error) *protocol.Error {
	if errors.Is(err, ErrGateBusy) {
		return protocol.Errorf(protocol.CodeServiceUnavailable, "password hashing is busy")
	}
	return unavailable()
}

func (s *Service) count(c *prometheus.CounterVec, result string) {
	if c != nil {
		c.WithLabelValues(result).Inc()
	}
}
