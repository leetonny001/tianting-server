package account

import (
	"context"
	"errors"
	"time"

	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
	"tianting/server/internal/testhook"
)

// 注册的限制（F-ACC-01 规则 7），未校准。
const (
	registerIPLimit      = 20 // 每 IP 每小时，所有模式
	registerIPWindow     = time.Hour
	inviteMismatchLimit  = 20 // 同一邀请码 24 小时内 4095 达到这个次数，自动停用（RC2-31）
	inviteMismatchWindow = 24 * time.Hour
)

// RegisterInput 是 POST /v1/account/register。
type RegisterInput struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	InviteCode string `json:"inviteCode"`
	Platform   string `json:"platform"`
	DeviceName string `json:"deviceName"`
}

func inviteInvalid() *protocol.Error {
	// 缺失、错误、过期、用完、停用的应答完全相同（AC-ACC-01-2）。
	return protocol.WithReason(protocol.CodeForbidden, protocol.ReasonInviteInvalid, "invite code is invalid")
}

func usernameTaken() *protocol.Error {
	return protocol.Errorf(protocol.CodeUsernameTaken, "username cannot be used")
}

func internalErr() *protocol.Error { return protocol.Errorf(protocol.CodeInternal, "internal error") }

// Register 是 F-ACC-01。顺序：IP 限流 → 模式 → 格式 → 邀请码 → 用户名占用 → 闸门和哈希 → 事务。
// 邀请码在用户名之前（规则 7）：没有有效邀请码的人得不到"用户名是否存在"的信号。
func (s *Service) Register(ctx context.Context, in RegisterInput, ip string) (*AuthResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	resp, result, e := s.register(ctx, in, ip)
	s.count(s.Registers, result)
	return resp, e
}

func (s *Service) register(ctx context.Context, in RegisterInput, ip string) (*AuthResponse, string, *protocol.Error) {
	// 所有模式都按 IP 限制（规则 7），不合法的请求也算（TC-ACC-01-10）。
	if e := s.guard.LimitIP(ctx, "register", ip, registerIPLimit, registerIPWindow); e != nil {
		return nil, limitResult(e), e
	}
	if s.cfg.RegisterMode == ModeClosed {
		return nil, metrics.AccountClosed, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonRegisterClosed, "registration is closed")
	}
	if e := validateDevice(in.Platform, in.DeviceName); e != nil {
		return nil, metrics.AccountBadRequest, e
	}
	if !ValidUsername(in.Username) {
		return nil, metrics.AccountBadRequest, protocol.Errorf(protocol.CodeInvalidID, "invalid username")
	}
	userID := NormalizeUsername(in.Username)
	if !passwordOK(in.Password, userID) {
		return nil, metrics.AccountBadRequest, protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonPasswordWeak, "password is too weak")
	}
	nickname := protocol.TrimText(in.Nickname)
	if nickname == "" {
		nickname = userID
	}
	if protocol.TextLen(nickname) > NicknameMaxLen {
		return nil, metrics.AccountBadRequest, protocol.Errorf(protocol.CodeBadRequest, "nickname too long")
	}

	var inviteHash string
	if s.cfg.RegisterMode == ModeInvite {
		inviteHash = InviteHash(in.InviteCode)
		ok := false
		if in.InviteCode != "" {
			var err error
			if ok, err = s.Store.InviteUsable(ctx, inviteHash, s.Clock.Now().UnixMilli()); err != nil {
				s.Log.Error("check invite", "err", err)
				return nil, metrics.AccountError, unavailable()
			}
		}
		if !ok {
			return nil, metrics.AccountInviteInvalid, inviteInvalid()
		}
	}
	if s.reserved[userID] {
		return nil, metrics.AccountTaken, s.taken(ctx, inviteHash)
	}
	// 事务里还会再查一次；这里先查，已占用的用户名不用白算一次哈希。
	switch taken, err := s.Store.UsernameTaken(ctx, userID); {
	case err != nil:
		s.Log.Error("check username", "err", err)
		return nil, metrics.AccountError, unavailable()
	case taken:
		return nil, metrics.AccountTaken, s.taken(ctx, inviteHash)
	}

	free, err := s.gate.Acquire(ctx, GateGeneral)
	if err != nil {
		return nil, metrics.AccountBusy, gateErr(err)
	}
	hash, err := hashPassword(s.cfg.Argon, in.Password)
	free()
	if err != nil {
		s.Log.Error("hash password", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	testhook.AfterArgon(UsernameHash(userID))

	now := s.Clock.Now()
	session, refreshToken, err := s.newDeviceSession(userID, in.Platform, in.DeviceName, ip, now)
	if err != nil {
		s.Log.Error("new device session", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	code, err := newRecoveryCode()
	if err != nil {
		s.Log.Error("new recovery code", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	err = s.Store.CreateAccount(ctx, store.NewAccount{
		Account: store.Account{UserID: userID, UsernameLower: userID, PasswordHash: hash, NoticeVersion: s.cfg.PrivacyVersion,
			CreatedAt: now.UnixMilli(), LastLoginAt: now.UnixMilli()},
		Nickname: nickname, InviteHash: inviteHash, RecoveryHash: RecoveryHash(userID, code), Session: session,
		BeforeCommit: s.Faults.takeRegisterTx,
	})
	switch {
	case errors.Is(err, store.ErrExists):
		return nil, metrics.AccountTaken, s.taken(ctx, inviteHash)
	case errors.Is(err, store.ErrInviteInvalid):
		return nil, metrics.AccountInviteInvalid, inviteInvalid()
	case err != nil:
		// 事务失败：邀请码不扣减，用户不产生（规则 8 的异常）。
		s.Log.Error("create account", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	resp, err := s.issue(session, 0, refreshToken, now)
	if err != nil {
		s.Log.Error("issue access token", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	resp.RecoveryCode, resp.RecoveryCodeGen = code, 1
	s.Log.Info("account registered", "uid", userID)
	return resp, metrics.AccountOK, nil
}

// taken 返回 4095，邀请码模式下计一次探测（规则 7）。计数失败不影响这次应答。
func (s *Service) taken(ctx context.Context, inviteHash string) *protocol.Error {
	if inviteHash != "" {
		disabled, err := s.Store.RecordInviteMismatch(ctx, inviteHash, s.Clock.Now().UnixMilli(), inviteMismatchWindow.Milliseconds(), inviteMismatchLimit)
		if err != nil {
			s.Log.Error("record invite mismatch", "err", err)
		} else if disabled {
			// 告警到人（第 12.2 节）：邀请码只记哈希。第一批不做重新启用，用新建邀请码恢复。
			s.Log.Warn("invite auto disabled: too many taken usernames", "invite_hash", inviteHash)
			if s.InviteAutoDisabled != nil {
				s.InviteAutoDisabled.Inc()
			}
		}
	}
	return usernameTaken()
}

func limitResult(e *protocol.Error) string {
	switch e.Code {
	case protocol.CodeRateLimited:
		return metrics.AccountRateLimited
	case protocol.CodeAccountLocked:
		return metrics.AccountLocked
	}
	return metrics.AccountError
}
