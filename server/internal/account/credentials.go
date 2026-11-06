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

// 账号事件和 securityNotice 的 kind（F-ACC-04、F-ACC-05）。
const (
	EventPasswordChanged  = "passwordChanged"
	EventPasswordRecover  = "passwordRecovered"
	EventLoggedOutOthers  = "loggedOutOthers"
	EventRecoveryRegen    = "recoveryCodeRegenerated"
	EventDeletionRequest  = "deletionRequested"
	EventDeletionCanceled = "deletionCanceled"
)

// PasswordInput 是 POST /v1/account/password。
type PasswordInput struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// PasswordOnlyInput 是 logout-others、recovery-code、delete。
type PasswordOnlyInput struct {
	Password string `json:"password"`
}

// verified 是一次通过的密码校验。release 放开闸门；attempt.Done 放开用户名。
type verified struct {
	acct    store.Account
	attempt *Attempt
	release func()
}

func (v *verified) done() {
	v.release()
	v.attempt.Done()
}

// verifyPassword 是所有校验密码的接口共用的前半段（F-ACC-02 规则 3，RS4-11）：限流和锁定 → 闸门 → Argon2（不存在也算一次）→ 失败计数。
// 成功时闸门的位还占着（改密还要算新哈希），调用方用 done 放开。
func (s *Service) verifyPassword(ctx context.Context, username, password, ip string, kind GateKind) (*verified, *protocol.Error) {
	attempt, e := s.guard.Begin(ctx, username, ip)
	if e != nil {
		return nil, e
	}
	free, err := s.gate.Acquire(ctx, kind)
	if err != nil {
		attempt.Done()
		return nil, gateErr(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			free()
		}
	}
	fail := func(e *protocol.Error) (*verified, *protocol.Error) {
		release()
		attempt.Done()
		return nil, e
	}
	acct, err := s.Store.GetAccountByUsername(ctx, username)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("load account", "err", err)
		return fail(unavailable())
	}
	if acct.DeletedAt != 0 || acct.PasswordHash == "" {
		exists = false
	}
	encoded := s.dummy
	if exists {
		encoded = acct.PasswordHash
	}
	ok, _, err := verifyPassword(s.cfg.Argon, encoded, password)
	if err != nil {
		s.Log.Error("verify password", "err", err, "username_hash", attempt.UsernameHash)
		return fail(internalErr())
	}
	if !exists || !ok {
		release()
		if err := attempt.Fail(ctx); err != nil {
			s.Log.Error("record failure", "err", err)
			attempt.Done()
			return nil, unavailable()
		}
		attempt.Done()
		return nil, credentialsInvalid()
	}
	return &verified{acct: acct, attempt: attempt, release: release}, nil
}

// commitErr：ErrCredentialsChanged 是在途请求输给了 recover 或改密，401 / 4018，不计失败（F-ACC-04 规则 8）。
// 其他失败整体回滚（异常段：吊销写失败 5000，旧密码仍有效）。
func (s *Service) commitErr(err error, uid string) *protocol.Error {
	if errors.Is(err, store.ErrCredentialsChanged) {
		return credentialsInvalid()
	}
	s.Log.Error("commit credentials", "err", err, "uid", uid)
	return internalErr()
}

// newSessionFor 给当前设备发新 sid（规则 3、6，RS4-03）：和当前设备共用 sid 的攻击者也被清除。平台和设备名沿用旧会话。
func (s *Service) newSessionFor(claims auth.Claims, ctx context.Context, ip string, now time.Time) (store.DeviceSession, string, *protocol.Error) {
	old, err := s.Store.GetDeviceSession(ctx, claims.SID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.DeviceSession{}, "", credentialsInvalid()
		}
		return store.DeviceSession{}, "", unavailable()
	}
	d, token, err := s.newDeviceSession(claims.UserID, old.Platform, old.DeviceName, ip, now)
	if err != nil {
		s.Log.Error("new device session", "err", err)
		return store.DeviceSession{}, "", internalErr()
	}
	return d, token, nil
}

func (s *Service) retireUntil(now time.Time) int64 {
	return now.Add(time.Duration(s.cfg.RecoveryOldDays) * 24 * time.Hour).UnixMilli()
}

// Password 是 F-ACC-04 规则 2、3。顺序：新密码规则 → 共用的失败限制 → 专用位校验旧密码、算新哈希 → 钩子 → 提交事务（第一条语句按版本锁 accounts 行）→ 踢线。
func (s *Service) Password(ctx context.Context, claims auth.Claims, in PasswordInput, ip string) (*AuthResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	if !passwordOK(in.NewPassword, claims.UserID) {
		return nil, protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonPasswordWeak, "password is too weak")
	}
	v, e := s.verifyPassword(ctx, claims.UserID, in.OldPassword, ip, GatePassword)
	if e != nil {
		return nil, e
	}
	defer v.done()
	hash, err := hashPassword(s.cfg.Argon, in.NewPassword)
	v.release()
	if err != nil {
		s.Log.Error("hash password", "err", err)
		return nil, internalErr()
	}
	v.done() // 哈希算完了：不再占着这个用户名
	testhook.AfterArgon(v.attempt.UsernameHash)

	now := s.Clock.Now()
	session, refreshToken, e := s.newSessionFor(claims, ctx, ip, now)
	if e != nil {
		return nil, e
	}
	code, err := newRecoveryCode()
	if err != nil {
		return nil, internalErr()
	}
	ms := now.UnixMilli()
	res, err := s.Store.CommitCredentials(ctx, store.CredentialCommit{
		UserID: claims.UserID, Version: v.acct.CredentialsVersion, CallerSID: claims.SID, Now: ms,
		NewPasswordHash: hash, NewRecoveryHash: RecoveryHash(claims.UserID, code), RetireUntil: s.retireUntil(now),
		RevokeReason: protocol.ReasonCredentialsChanged, NewSession: &session, ThrottleKey: v.attempt.PairKey(),
		Event:        store.AccountEvent{UserID: claims.UserID, Kind: EventPasswordChanged, At: ms, Platform: session.Platform, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(session.SID)},
		BeforeCommit: s.Faults.takeRevokeWrite,
	})
	if err != nil {
		return nil, s.commitErr(err, claims.UserID)
	}
	version, err := s.Store.TokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, unavailable()
	}
	resp, err := s.issue(session, version, refreshToken, now)
	if err != nil {
		return nil, internalErr()
	}
	resp.RecoveryCode, resp.RecoveryCodeGen = code, res.RecoveryGen
	// 提醒排在踢线之前，踢线之后连接就关了（F-ACC-02 规则 3b、F-ACC-05 规则 5）。发起的旧 sid 不提醒。新 sid 这时还没有连接。
	s.notify(claims.UserID, claims.SID, EventPasswordChanged, session.Platform, ms)
	// 踢线放在提交之后。旧 sid 也踢：SDK 缓存这个踢线等响应（规则 3）。新 sid 不踢。
	s.Router.KickUserExcept(claims.UserID, session.SID, protocol.KickPasswordChanged, s.recoverBefore(ctx, claims.UserID))
	s.Log.Info("account password changed", "uid", claims.UserID)
	return resp, nil
}

// LogoutOthers 是 F-ACC-04 规则 6：效果同规则 3 的吊销和换 sid，不改密码，revokeReason=loggedOutOthers。
func (s *Service) LogoutOthers(ctx context.Context, claims auth.Claims, in PasswordOnlyInput, ip string) (*AuthResponse, *protocol.Error) {
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
	now := s.Clock.Now()
	session, refreshToken, e := s.newSessionFor(claims, ctx, ip, now)
	if e != nil {
		return nil, e
	}
	ms := now.UnixMilli()
	if _, err := s.Store.CommitCredentials(ctx, store.CredentialCommit{
		UserID: claims.UserID, Version: v.acct.CredentialsVersion, CallerSID: claims.SID, Now: ms,
		RevokeReason: protocol.ReasonLoggedOutOthers, NewSession: &session, ThrottleKey: v.attempt.PairKey(),
		Event:        store.AccountEvent{UserID: claims.UserID, Kind: EventLoggedOutOthers, At: ms, Platform: session.Platform, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(session.SID)},
		BeforeCommit: s.Faults.takeRevokeWrite,
	}); err != nil {
		return nil, s.commitErr(err, claims.UserID)
	}
	version, err := s.Store.TokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, unavailable()
	}
	resp, err := s.issue(session, version, refreshToken, now)
	if err != nil {
		return nil, internalErr()
	}
	s.Router.KickUserExcept(claims.UserID, session.SID, protocol.KickSessionRevoked, s.recoverBefore(ctx, claims.UserID))
	return resp, nil
}
