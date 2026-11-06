package account

import (
	"context"
	"errors"

	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
	"tianting/server/internal/testhook"
)

// 账号事件的 kind（F-ACC-02 规则 3b）。改密、找回、重放等随 BE-415～418。
const EventNewDeviceLogin = protocol.SecurityNewDeviceLogin

// deletionCoolingMs 是注销冷静期（F-ACC-08 规则 2），未校准。
const deletionCoolingMs = int64(7 * 24 * 3600 * 1000)

// LoginInput 是 POST /v1/account/login。
type LoginInput struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Platform   string `json:"platform"`
	DeviceName string `json:"deviceName"`
}

// credentialsInvalid：用户不存在、没有账号记录、密码错误、已注销的响应完全相同（F-ACC-02 规则 2）。
func credentialsInvalid() *protocol.Error {
	return protocol.Errorf(protocol.CodeAccountCredentials, "invalid username or password")
}

// Login 是 F-ACC-02。顺序：IP 限流 → 锁定 → 渐进延迟 → 单哈希 → 闸门 → 校验密码（不存在也算一次）→ 封禁和注销 → 事务。
func (s *Service) Login(ctx context.Context, in LoginInput, ip string) (*AuthResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	resp, result, e := s.login(ctx, in, ip)
	s.count(s.Logins, result)
	return resp, e
}

func (s *Service) login(ctx context.Context, in LoginInput, ip string) (*AuthResponse, string, *protocol.Error) {
	if e := validateDevice(in.Platform, in.DeviceName); e != nil {
		return nil, metrics.AccountBadRequest, e
	}
	// 不判断格式：不合法的用户名就是不存在的用户名，走同样的路径（规则 2、3）。
	username := NormalizeUsername(in.Username)
	attempt, e := s.guard.Begin(ctx, username, ip)
	if e != nil {
		s.Log.Info("account login rejected", "code", e.Code)
		return nil, limitResult(e), e
	}
	defer attempt.Done()

	free, err := s.gate.Acquire(ctx, GateGeneral)
	if err != nil {
		return nil, metrics.AccountBusy, gateErr(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			free()
		}
	}
	defer release()

	acct, err := s.Store.GetAccountByUsername(ctx, username)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("load account", "err", err) // 数据库不可用：503 / 5030（异常段）
		return nil, metrics.AccountError, unavailable()
	}
	if acct.DeletedAt != 0 || acct.PasswordHash == "" {
		exists = false // 已注销：密码哈希已经删除，按不存在处理
	}
	encoded := s.dummy
	if exists {
		encoded = acct.PasswordHash
	}
	ok, stale, err := verifyPassword(s.cfg.Argon, encoded, in.Password)
	if err != nil {
		s.Log.Error("verify password", "err", err, "username_hash", attempt.UsernameHash)
		return nil, metrics.AccountError, internalErr()
	}
	if !exists || !ok {
		release()
		if err := attempt.Fail(ctx); err != nil {
			s.Log.Error("record login failure", "err", err)
			return nil, metrics.AccountError, unavailable()
		}
		// 只记哈希，不记原始用户名（规则 3a）。
		s.Log.Info("account login failed", "username_hash", attempt.UsernameHash)
		return nil, metrics.AccountInvalid, credentialsInvalid()
	}
	// 密码正确才判断封禁和注销（规则 4、规则 1）：不让没有密码的人知道账号状态。
	ban, err := s.Store.UserBan(ctx, acct.UserID)
	if err != nil {
		s.Log.Error("check ban", "err", err, "uid", acct.UserID)
		return nil, metrics.AccountError, unavailable()
	}
	now := s.Clock.Now()
	if ban.Active(now.UnixMilli()) {
		e := protocol.Errorf(protocol.CodeUserBanned, "user is banned")
		e.Details = map[string]any{"bannedUntil": ban.Until}
		return nil, metrics.AccountBanned, e
	}
	if acct.DeleteRequestedAt != 0 {
		e := protocol.WithReason(protocol.CodeForbidden, protocol.ReasonAccountDeletionPending, "account deletion is pending")
		e.Details = map[string]any{"deleteAt": acct.DeleteRequestedAt + deletionCoolingMs}
		return nil, metrics.AccountPending, e
	}
	// 旧哈希参数升级后重算（异常段），不改变应答。还占着闸门的位，失败也不影响这次登录。
	if stale {
		if h, err := hashPassword(s.cfg.Argon, in.Password); err != nil {
			s.Log.Error("rehash password", "err", err, "uid", acct.UserID)
		} else if err := s.Store.UpdatePasswordHash(ctx, acct.UserID, acct.CredentialsVersion, h); err != nil {
			s.Log.Error("store rehashed password", "err", err, "uid", acct.UserID)
		}
	}
	release()
	attempt.Done() // 哈希算完了：不再占着这个用户名
	// Argon2 在事务外（F-ACC-04 规则 8）：账号并发钩子停在这里，提交事务的第一条语句再校验版本。
	testhook.AfterArgon(attempt.UsernameHash)

	session, refreshToken, err := s.newDeviceSession(acct.UserID, in.Platform, in.DeviceName, ip, now)
	if err != nil {
		s.Log.Error("new device session", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	code, err := newRecoveryCode()
	if err != nil {
		s.Log.Error("new recovery code", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	version, err := s.Store.TokenVersion(ctx, acct.UserID)
	if err != nil {
		s.Log.Error("token version", "err", err, "uid", acct.UserID)
		return nil, metrics.AccountError, unavailable()
	}
	result, err := s.Store.CommitLogin(ctx, store.LoginCommit{
		UserID: acct.UserID, CredentialsVersion: acct.CredentialsVersion, Now: now.UnixMilli(), Session: session,
		NewRecoveryHash: RecoveryHash(acct.UserID, code), ThrottleKey: attempt.PairKey(),
		Event: store.AccountEvent{UserID: acct.UserID, Kind: EventNewDeviceLogin, At: now.UnixMilli(), Platform: in.Platform,
			IPPrefix: session.SeenIPPrefixes[0], SIDHash: SIDHash(session.SID)},
	})
	switch {
	case errors.Is(err, store.ErrCredentialsChanged):
		// 校验密码之后 recover 或改密提交了：401 / 4018，不计入失败次数（F-ACC-04 规则 8）。
		return nil, metrics.AccountInvalid, credentialsInvalid()
	case err != nil:
		s.Log.Error("commit login", "err", err, "uid", acct.UserID)
		return nil, metrics.AccountError, unavailable()
	}
	resp, err := s.issue(session, version, refreshToken, now)
	if err != nil {
		s.Log.Error("issue access token", "err", err)
		return nil, metrics.AccountError, internalErr()
	}
	// 最新一代恢复码没有确认：这一代作废，新的一代随响应返回（F-ACC-05 规则 1）。
	if result.RecoveryGen > 0 {
		resp.RecoveryCode, resp.RecoveryCodeGen = code, result.RecoveryGen
	}
	// 踢线和提醒都放在提交之后。不踢其他设备（规则 5）。
	s.notify(acct.UserID, EventNewDeviceLogin, in.Platform, now.UnixMilli())
	s.Log.Info("account login", "uid", acct.UserID, "platform", in.Platform)
	return resp, metrics.AccountOK, nil
}

// EventsResponse 是 GET /v1/account/events（第 11.2 节）。"账号安全"页显示最近 20 条（F-ACC-02 规则 3b）。
type EventsResponse struct {
	Events              []EventJSON       `json:"events"`
	OtherSessions       OtherSessionsJSON `json:"otherSessions"`
	RecoveryCodeUnacked bool              `json:"recoveryCodeUnacked,omitempty"`
}

// EventJSON 是一条账号事件。没有 sid 的哈希；ipPrefix 是前缀，不是完整地址（TC-ACC-02-13）。
type EventJSON struct {
	Kind     string `json:"kind"`
	At       int64  `json:"at"`
	Platform string `json:"platform,omitempty"`
	IPPrefix string `json:"ipPrefix,omitempty"`
}

// OtherSessionsJSON 是这条会话之外的会话数量和最近活跃时间（F-ACC-03 规则 4）。
type OtherSessionsJSON struct {
	Count        int   `json:"count"`
	LastActiveAt int64 `json:"lastActiveAt,omitempty"`
}

const recentEvents = 20

// Events 读"账号安全"页的数据。
func (s *Service) Events(ctx context.Context, userID, sid string) (*EventsResponse, *protocol.Error) {
	evs, err := s.Store.ListAccountEvents(ctx, userID, recentEvents)
	if err != nil {
		s.Log.Error("list account events", "err", err, "uid", userID)
		return nil, unavailable()
	}
	others, err := s.Store.OtherDeviceSessions(ctx, userID, sid, s.Clock.Now().UnixMilli())
	if err != nil {
		s.Log.Error("count sessions", "err", err, "uid", userID)
		return nil, unavailable()
	}
	unacked, err := s.Store.RecoveryUnacked(ctx, userID)
	if err != nil {
		s.Log.Error("recovery state", "err", err, "uid", userID)
		return nil, unavailable()
	}
	out := &EventsResponse{Events: make([]EventJSON, 0, len(evs)), OtherSessions: OtherSessionsJSON{Count: others.Count, LastActiveAt: others.LastActiveAt}, RecoveryCodeUnacked: unacked}
	for _, e := range evs {
		out.Events = append(out.Events, EventJSON{Kind: e.Kind, At: e.At, Platform: e.Platform, IPPrefix: e.IPPrefix})
	}
	return out, nil
}
