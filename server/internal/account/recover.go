package account

import (
	"context"
	"errors"
	"time"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
	"tianting/server/internal/testhook"
)

// 找回的限制（F-ACC-05 规则 2），未校准。
const (
	recoverIPLimit    = 20 // 每 IP 每分钟，查库之前，不看是否命中
	recoverIPWindow   = time.Minute
	recoverSuccessMax = 6 // 用最新一代的码，每账号每小时成功次数
	recoverWindow     = time.Hour
)

// RecoverInput 是 POST /v1/account/recover。
type RecoverInput struct {
	Username     string `json:"username"`
	RecoveryCode string `json:"recoveryCode"`
	NewPassword  string `json:"newPassword"`
	Platform     string `json:"platform"`
	DeviceName   string `json:"deviceName"`
}

// usableCode：没用过、没作废、没过期（规则 2）。提交事务里还会原子地再查一次。
func usableCode(r store.RecoveryCode, now int64) bool {
	return r.UsedAt == 0 && r.RevokedAt == 0 && (r.RetireUntil == 0 || r.RetireUntil > now)
}

func recoverLimited() *protocol.Error {
	e := protocol.WithReason(protocol.CodeRateLimited, protocol.ReasonRecoverLimited, "too many recoveries")
	e.Details = map[string]any{"retryAfterSeconds": int64(recoverWindow / time.Second)}
	return e
}

func recoverLimitKey(userID string) string { return "recover:" + UsernameHash(userID) }

// Recover 是 F-ACC-05 规则 2、3、5、5a。顺序：
//  1. 每 IP 总量限流（查库之前，不看是否命中）；
//  2. 格式和新密码规则：不查库，所以不泄露账号是否存在；
//  3. 按 (userId, code) 查哈希。账号不存在时走同样的两次查询（AC-ACC-05-11 的等时）；
//  4. 未命中：不占 Argon2 闸门，已锁定 429 / 4294，否则计入失败限制，401 / 4018，响应相同；
//  5. 命中：不受"用户名 + IP"锁定和单哈希限制（128 位码不怕猜，否则持有密码的攻击者能压制受害者，RS5-02）；
//  6. 专用位按 (userId, gen) 公平调度，进入前重查（RS6-02）；
//  7. 提交事务（store.CommitRecover）：不看版本，只看这一行码的状态。
func (s *Service) Recover(ctx context.Context, in RecoverInput, ip string) (*AuthResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	if e := s.guard.LimitIP(ctx, "recover", ip, recoverIPLimit, recoverIPWindow); e != nil {
		return nil, e
	}
	if e := validateDevice(in.Platform, in.DeviceName); e != nil {
		return nil, e
	}
	username := NormalizeUsername(in.Username)
	if !passwordOK(in.NewPassword, username) {
		return nil, protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonPasswordWeak, "password is too weak")
	}
	acct, err := s.Store.GetAccountByUsername(ctx, username)
	exists := err == nil && acct.DeletedAt == 0
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("load account", "err", err)
		return nil, unavailable()
	}
	codeHash := RecoveryHash(username, in.RecoveryCode)
	row, err := s.Store.FindRecoveryCode(ctx, username, codeHash)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("find recovery code", "err", err)
		return nil, unavailable()
	}
	found := err == nil
	if !exists || !found || !usableCode(row, s.Clock.Now().UnixMilli()) {
		if e := s.guard.RecoverMiss(ctx, username, ip); e != nil {
			return nil, e
		}
		s.Log.Info("account recover failed", "username_hash", UsernameHash(username))
		return nil, credentialsInvalid()
	}
	ban, err := s.Store.UserBan(ctx, acct.UserID)
	if err != nil {
		s.Log.Error("check ban", "err", err, "uid", acct.UserID)
		return nil, unavailable()
	}
	if ban.Active(s.Clock.Now().UnixMilli()) {
		e := protocol.Errorf(protocol.CodeUserBanned, "user is banned")
		e.Details = map[string]any{"bannedUntil": ban.Until}
		return nil, e
	}
	// 上限只约束最新一代（RS7-02）。这里只是省一次哈希，事务里还会查。
	if row.Gen == row.LatestGen {
		hit, err := s.Store.PeekThrottle(ctx, recoverLimitKey(acct.UserID), s.Clock.Now().UnixMilli(), recoverWindow.Milliseconds())
		if err != nil {
			return nil, unavailable()
		}
		if hit.Count >= recoverSuccessMax {
			return nil, recoverLimited()
		}
	}
	free, err := s.gate.AcquireRecover(ctx, acct.UserID, row.Gen)
	if err != nil {
		return nil, gateErr(err)
	}
	defer free()
	// 进入专用位前重查（RS6-02）：同一个码排队的请求不再白算。这里失败不算未命中：码曾经是对的。
	if row, err = s.Store.FindRecoveryCode(ctx, username, codeHash); err != nil || !usableCode(row, s.Clock.Now().UnixMilli()) {
		return nil, credentialsInvalid()
	}
	hash, err := hashPassword(s.cfg.Argon, in.NewPassword)
	if err != nil {
		s.Log.Error("hash password", "err", err)
		return nil, internalErr()
	}
	testhook.AfterArgon(UsernameHash(username))

	now := s.Clock.Now()
	session, refreshToken, err := s.newDeviceSession(acct.UserID, in.Platform, in.DeviceName, ip, now)
	if err != nil {
		return nil, internalErr()
	}
	code, err := newRecoveryCode()
	if err != nil {
		return nil, internalErr()
	}
	ms := now.UnixMilli()
	pairKey := ""
	if !s.guard.disabled {
		pairKey = pairThrottleKey(username, ip)
	}
	gen, err := s.Store.CommitRecover(ctx, store.RecoverCommit{
		UserID: acct.UserID, Gen: row.Gen, CodeHash: codeHash, PasswordHash: hash, Now: ms,
		NewRecoveryHash: RecoveryHash(acct.UserID, code), RevokeReason: protocol.ReasonCredentialsChanged, Session: session,
		LimitKey: recoverLimitKey(acct.UserID), LimitMax: recoverSuccessMax, LimitWindowMs: recoverWindow.Milliseconds(),
		ThrottleKey:  pairKey,
		Event:        store.AccountEvent{UserID: acct.UserID, Kind: EventPasswordRecover, At: ms, Platform: in.Platform, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(session.SID)},
		BeforeCommit: s.Faults.takeRevokeWrite,
	})
	switch {
	case errors.Is(err, store.ErrRecoveryCodeInvalid):
		return nil, credentialsInvalid() // 同一个码并发只有一个成功（AC-ACC-04-20）
	case errors.Is(err, store.ErrRecoverLimited):
		return nil, recoverLimited()
	case err != nil:
		s.Log.Error("commit recover", "err", err, "uid", acct.UserID)
		return nil, internalErr()
	}
	version, err := s.Store.TokenVersion(ctx, acct.UserID)
	if err != nil {
		return nil, unavailable()
	}
	resp, err := s.issue(session, version, refreshToken, now)
	if err != nil {
		return nil, internalErr()
	}
	resp.RecoveryCode, resp.RecoveryCodeGen = code, gen
	// 提醒排在踢线之前，踢线之后连接就关了（F-ACC-02 规则 3b）。发起找回的设备还没有连接。
	s.notify(acct.UserID, session.SID, EventPasswordRecover, in.Platform, ms)
	s.Router.KickUserExcept(acct.UserID, session.SID, protocol.KickPasswordChanged, s.recoverBefore(ctx, acct.UserID))
	s.Log.Info("account recovered", "uid", acct.UserID)
	return resp, nil
}

// RecoveryCodeResponse 是 POST /v1/account/recovery-code。
type RecoveryCodeResponse struct {
	RecoveryCode    string `json:"recoveryCode"`
	RecoveryCodeGen int64  `json:"recoveryCodeGen"`
}

// RegenerateRecovery 是 F-ACC-05 规则 5：要当前密码，产生新的一代，已确认的旧码继续有效 7 天。其他在线设备收到提醒。
func (s *Service) RegenerateRecovery(ctx context.Context, claims auth.Claims, in PasswordOnlyInput, ip string) (*RecoveryCodeResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	v, e := s.verifyPassword(ctx, claims.UserID, in.Password, ip, GateGeneral)
	if e != nil {
		return nil, e
	}
	defer v.done()
	v.release()
	v.done() // 哈希算完了：不再占着这个用户名
	testhook.AfterArgon(v.attempt.UsernameHash)
	code, err := newRecoveryCode()
	if err != nil {
		return nil, internalErr()
	}
	now := s.Clock.Now()
	ms := now.UnixMilli()
	res, err := s.Store.CommitCredentials(ctx, store.CredentialCommit{
		UserID: claims.UserID, Version: v.acct.CredentialsVersion, CallerSID: claims.SID, Now: ms,
		NewRecoveryHash: RecoveryHash(claims.UserID, code), RetireUntil: s.retireUntil(now), ThrottleKey: v.attempt.PairKey(),
		Event: store.AccountEvent{UserID: claims.UserID, Kind: EventRecoveryRegen, At: ms, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(claims.SID)},
	})
	if err != nil {
		return nil, s.commitErr(err, claims.UserID)
	}
	if d, err := s.Store.GetDeviceSession(ctx, claims.SID); err == nil {
		s.notify(claims.UserID, claims.SID, EventRecoveryRegen, d.Platform, ms)
	}
	return &RecoveryCodeResponse{RecoveryCode: code, RecoveryCodeGen: res.RecoveryGen}, nil
}

// RecoveryAckInput 是 POST /v1/account/recovery-code/ack。
type RecoveryAckInput struct {
	Gen int64 `json:"gen"`
}

// AckRecovery 确认一代恢复码（规则 1）。不是最新一代：400 / 4000 recoveryCodeStale，不算确认。
func (s *Service) AckRecovery(ctx context.Context, userID string, in RecoveryAckInput) *protocol.Error {
	err := s.Store.AckRecoveryCode(ctx, userID, in.Gen, s.Clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrRecoveryCodeStale):
		return protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonRecoveryCodeStale, "recovery code is not the latest")
	case err != nil:
		s.Log.Error("ack recovery code", "err", err, "uid", userID)
		return unavailable()
	}
	return nil
}
