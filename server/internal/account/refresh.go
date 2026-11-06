package account

import (
	"context"
	"errors"
	"slices"
	"time"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 账号事件和 securityNotice 的 kind（F-ACC-03 规则 4）。
const (
	EventRefreshReplay      = "refreshReplay"
	EventRefreshRateLimited = "refreshRateLimited"
	EventNewIPPrefix        = "newIpPrefix"
)

const (
	issueWindow      = time.Hour
	limitedStreakMax = 6  // 连续 6 小时触发限流才提醒其他设备（未校准，RS5-14）
	seenPrefixMax    = 64 // seen_ip_prefixes 的上限：只用来判断"新前缀"，超出时丢最旧的
	casRetries       = 16 // 条件更新失败后重读的次数（RC2-04）
)

// RefreshInput 是 POST /v1/account/token/refresh。
type RefreshInput struct {
	RefreshToken string `json:"refreshToken"`
}

// refreshInvalid：mac 错、格式不对、kid 不认识。不吊销，不计重放，不带 recoverBefore（RS4-05）。
func refreshInvalid() *protocol.Error {
	return protocol.WithReason(protocol.CodeRefreshTokenInvalid, protocol.ReasonRefreshInvalid, "refresh token is invalid")
}

// refreshErr 是 mac 正确时的 4019：都可以带 recoverBefore（规则 6a）。
func (s *Service) refreshErr(ctx context.Context, userID, reason string) *protocol.Error {
	e := protocol.WithReason(protocol.CodeRefreshTokenInvalid, reason, "refresh token is not usable")
	if userID != "" {
		if at := s.recoverBefore(ctx, userID); at != nil {
			e.Details = map[string]any{"recoverBefore": *at}
		}
	}
	return e
}

// revokeReasonOf 把吊销原因映射成 4019 的 reason（规则 4 第 ③ 步）。重放吊销之后再来也是 replayDetected。
func revokeReasonOf(stored string) string {
	switch stored {
	case protocol.ReasonCredentialsChanged, protocol.ReasonLoggedOutOthers, protocol.ReasonAccountDeleted, protocol.ReasonReplayDetected:
		return stored
	}
	return protocol.ReasonRefreshRevoked
}

// Refresh 是 F-ACC-03 规则 4 的 refresh 判定，顺序逐字按规格（RS5-18、RS6-05）：
// ① mac（错：invalid）→ ② 会话存在 → ③ 已吊销（按吊销原因，不算重放）→ ④ 已过期 → ⑤ 代号 → ⑥ 签发频率。
// 第 ⑥ 步只在将要签发 token 的分支：mac 错误、已吊销、已过期、superseded、replayDetected 都走不到它，否则攻击者用满次数就能让受害者到不了代号判定。
//
// 代号：current 或 pending 才签发；其他只有 gen ≥ overlapMinGen 且在 overlapUntil 之前才 superseded，其余一律按重放吊销。
// gen 受 mac 保护，连续转正时 overlapMinGen 前移，旧值马上低于它（RS5-01）。
func (s *Service) Refresh(ctx context.Context, in RefreshInput, ip string) (*AuthResponse, *protocol.Error) {
	if !s.available() {
		return nil, unavailable()
	}
	if e := s.guard.LimitIP(ctx, "refresh", ip, s.cfg.RefreshIPRate, time.Minute); e != nil {
		return nil, e
	}
	claims, err := s.refresh.parse(in.RefreshToken)
	if err != nil {
		return nil, refreshInvalid()
	}
	hash := refreshHash(in.RefreshToken)
	for range casRetries {
		d, err := s.Store.GetDeviceSession(ctx, claims.SID)
		if errors.Is(err, store.ErrNotFound) {
			// mac 对但行没了：只有 session_purge 和注销清理会删，都是早已失效的会话。
			return nil, s.refreshErr(ctx, "", protocol.ReasonRefreshExpired)
		}
		if err != nil {
			s.Log.Error("load device session", "err", err)
			return nil, unavailable() // 规则 3 异常：503 / 5030，不回登录页
		}
		if d.RevokedAt != 0 {
			return nil, s.refreshErr(ctx, d.UserID, revokeReasonOf(d.RevokeReason))
		}
		now := s.Clock.Now()
		ms := now.UnixMilli()
		if ms >= d.ExpiresAt {
			return nil, s.refreshErr(ctx, d.UserID, protocol.ReasonRefreshExpired)
		}
		pendingIdx := slices.IndexFunc(d.Pendings, func(p store.PendingGen) bool { return p.Gen == claims.Gen && p.Hash == hash })
		isCurrent := claims.Gen == d.CurrentGen && hash == d.CurrentHash
		if !isCurrent && pendingIdx < 0 {
			// 规则 4：只有代号 ≥ overlapMinGen 且在窗口内才软拒绝，其他情况一律按重放处理。
			if claims.Gen >= d.OverlapMinGen && ms < d.OverlapUntil {
				e := s.refreshErr(ctx, d.UserID, protocol.ReasonRefreshSuperseded)
				if e.Details == nil {
					e.Details = map[string]any{}
				}
				e.Details["retryAfterSeconds"] = ceilSeconds(d.OverlapUntil - ms)
				return nil, e
			}
			revoked, e := s.replay(ctx, d, ip, now)
			if revoked {
				return nil, e
			}
			continue
		}
		resp, again, e := s.issueRefresh(ctx, d, pendingIdx, ip, now)
		if !again {
			return resp, e
		}
	}
	s.Log.Error("refresh: too many concurrent updates")
	return nil, unavailable()
}

// replay 吊销这条设备会话。条件更新失败时返回 false，调用方重新判定（可能已经被转正或吊销）。
// 踢线和提醒放在事务提交之后（规则 2）。
func (s *Service) replay(ctx context.Context, d store.DeviceSession, ip string, now time.Time) (bool, *protocol.Error) {
	ms := now.UnixMilli()
	ok, err := s.Store.RevokeDeviceSession(ctx, d.SID, protocol.ReasonReplayDetected, ms, d.RowVersion, store.AccountEvent{
		UserID: d.UserID, Kind: EventRefreshReplay, At: ms, Platform: d.Platform, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(d.SID)})
	if err != nil {
		s.Log.Error("revoke on replay", "err", err, "uid", d.UserID)
		return true, unavailable()
	}
	if !ok {
		return false, nil
	}
	inc(s.RefreshReplay)
	before := s.recoverBefore(ctx, d.UserID)
	s.Router.KickSession(d.UserID, d.SID, protocol.KickSessionRevoked, before)
	s.notify(d.UserID, d.SID, EventRefreshReplay, d.Platform, ms)
	s.Log.Warn("refresh replay detected", "uid", d.UserID, "sid_hash", SIDHash(d.SID))
	return true, s.refreshErr(ctx, d.UserID, protocol.ReasonReplayDetected)
}

// promote 把 pending 代变成 current（规则 4）：旧 current 和其他 pending 全部作废，到期时间滑动（只有转正才滑动，RS4-10）。
func (s *Service) promote(d *store.DeviceSession, p store.PendingGen, now time.Time) {
	d.OverlapUntil = now.Add(s.cfg.RefreshOverlap).UnixMilli()
	d.OverlapMinGen = d.CurrentGen
	d.CurrentGen, d.CurrentHash = p.Gen, p.Hash
	d.Pendings = nil
	d.ExpiresAt = s.slidExpiry(*d, now)
}

// seeIP 比较 IP 前缀（规则 4 的检测盲区缓解）：新前缀写事件，提交之后用 notifyNewPrefix 提醒，只提醒不吊销。
func seeIP(d *store.DeviceSession, ip string, ms int64) []store.AccountEvent {
	prefix := clientip.EventPrefix(ip)
	if slices.Contains(d.SeenIPPrefixes, prefix) {
		return nil
	}
	seen := append(slices.Clone(d.SeenIPPrefixes), prefix)
	if len(seen) > seenPrefixMax {
		seen = seen[len(seen)-seenPrefixMax:]
	}
	d.SeenIPPrefixes = seen
	return []store.AccountEvent{{UserID: d.UserID, Kind: EventNewIPPrefix, At: ms, Platform: d.Platform, IPPrefix: prefix, SIDHash: SIDHash(d.SID)}}
}

// notifyNewPrefix 在事务提交之后提醒其他设备（F-ACC-02 规则 3b）。PRD 和安全复查都没有规定频率，这里不限频（问题表 A13）：
// 每条会话每个前缀最多一次，弱网切换回见过的前缀不再提醒。
func (s *Service) notifyNewPrefix(events []store.AccountEvent, sid string) {
	for _, ev := range events {
		s.notify(ev.UserID, sid, ev.Kind, ev.Platform, ev.At)
	}
}

// issueRefresh 是第 ⑥ 步和签发。again=true 表示条件更新失败，要重新判定。
func (s *Service) issueRefresh(ctx context.Context, d store.DeviceSession, pendingIdx int, ip string, now time.Time) (*AuthResponse, bool, *protocol.Error) {
	ms := now.UnixMilli()
	nd := d
	if d.IssueWindowStart+issueWindow.Milliseconds() <= ms {
		nd.IssueWindowStart, nd.IssueCount = ms, 0
	}
	if nd.IssueCount >= s.cfg.RefreshIssueRate {
		return s.issueLimited(ctx, nd, now)
	}
	version, err := s.Store.TokenVersion(ctx, d.UserID)
	if err != nil {
		s.Log.Error("token version", "err", err, "uid", d.UserID)
		return nil, false, unavailable()
	}
	gen := nd.NextGen
	token, err := s.refresh.issue(d.SID, gen)
	if err != nil {
		s.Log.Error("issue refresh token", "err", err)
		return nil, false, internalErr()
	}
	reissue := pendingIdx < 0 && len(d.Pendings) > 0
	if pendingIdx >= 0 {
		s.promote(&nd, d.Pendings[pendingIdx], now)
	}
	nd.NextGen++ // 严格单调，永不复用（RS5-05）
	nd.IssueCount++
	// 不用确定性派生：每次都是新值（RS4-04）。待确认代只按数量淘汰（RS5-14、RS5-21）。
	pendings := append(slices.Clone(nd.Pendings), store.PendingGen{Gen: gen, Hash: refreshHash(token)})
	if len(pendings) > s.cfg.RefreshMaxPending {
		pendings = pendings[len(pendings)-s.cfg.RefreshMaxPending:]
	}
	nd.Pendings = pendings
	nd.LastSeenAt = ms
	events := seeIP(&nd, ip, ms)
	ok, err := s.Store.UpdateDeviceSession(ctx, nd, events...)
	if err != nil {
		s.Log.Error("update device session", "err", err, "uid", d.UserID)
		return nil, false, unavailable()
	}
	if !ok {
		return nil, true, nil
	}
	s.notifyNewPrefix(events, d.SID)
	if reissue {
		inc(s.RefreshReissue)
	}
	if pendingIdx >= 0 {
		s.Router.KickStaleGen(d.UserID, d.SID, nd.CurrentGen)
	}
	issued := nd
	issued.CurrentGen = gen
	resp, err := s.issue(issued, version, token, now)
	if err != nil {
		s.Log.Error("issue access token", "err", err)
		return nil, false, internalErr()
	}
	if unacked, err := s.Store.RecoveryUnacked(ctx, d.UserID); err != nil {
		s.Log.Warn("recovery state", "err", err, "uid", d.UserID)
	} else {
		resp.RecoveryCodeUnacked = unacked
	}
	return resp, false, nil
}

// issueLimited：429 / 4290，不吊销（规则 4）。一个窗口只写一次事件；连续 6 小时触发时提醒其他设备，防止持旧值的人静默压制受害者（RS5-14）。
func (s *Service) issueLimited(ctx context.Context, d store.DeviceSession, now time.Time) (*AuthResponse, bool, *protocol.Error) {
	ms := now.UnixMilli()
	retry := rateLimited(ceilSeconds(d.IssueWindowStart + issueWindow.Milliseconds() - ms))
	if d.LimitedWindow == d.IssueWindowStart {
		return nil, false, retry
	}
	if d.LimitedWindow > 0 && d.IssueWindowStart-d.LimitedWindow < 2*issueWindow.Milliseconds() {
		d.LimitedStreak++
	} else {
		d.LimitedStreak = 1
	}
	d.LimitedWindow = d.IssueWindowStart
	ok, err := s.Store.UpdateDeviceSession(ctx, d, store.AccountEvent{UserID: d.UserID, Kind: EventRefreshRateLimited, At: ms, Platform: d.Platform, SIDHash: SIDHash(d.SID)})
	if err != nil {
		s.Log.Error("record refresh limit", "err", err, "uid", d.UserID)
		return nil, false, retry // 限流本身不依赖这次写入
	}
	if !ok {
		return nil, true, nil
	}
	if d.LimitedStreak >= limitedStreakMax {
		s.notify(d.UserID, d.SID, EventRefreshRateLimited, d.Platform, ms)
	}
	return nil, false, retry
}

// LogoutInput 是 POST /v1/account/logout。
type LogoutInput struct {
	RefreshToken string `json:"refreshToken"`
}

// Logout 是 F-ACC-04 规则 1：Bearer 和 refreshToken 有一个有效就执行，幂等。refreshToken 的 mac 必须正确（RS4-05），
// 任何一代的值都按退出处理，不算重放。bearer 是 Bearer 的判定结果（没带时为 nil）。
func (s *Service) Logout(ctx context.Context, in LogoutInput, bearer *auth.Result) *protocol.Error {
	if in.RefreshToken != "" && s.available() {
		if claims, err := s.refresh.parse(in.RefreshToken); err == nil {
			return s.logoutSID(ctx, claims.SID)
		}
	}
	if bearer != nil && bearer.Verdict == auth.VerdictOK && bearer.Claims.SID != "" {
		return s.logoutSID(ctx, bearer.Claims.SID)
	}
	if in.RefreshToken != "" {
		return refreshInvalid()
	}
	return nil
}

func (s *Service) logoutSID(ctx context.Context, sid string) *protocol.Error {
	d, err := s.Store.GetDeviceSession(ctx, sid)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		s.Log.Error("load device session", "err", err)
		return unavailable()
	}
	ok, err := s.Store.RevokeDeviceSession(ctx, sid, store.RevokeReasonRevoked, s.Clock.Now().UnixMilli(), 0)
	if err != nil {
		s.Log.Error("revoke session on logout", "err", err, "uid", d.UserID)
		return unavailable()
	}
	if ok {
		s.Router.KickSession(d.UserID, sid, protocol.KickSessionRevoked, s.recoverBefore(ctx, d.UserID))
	}
	return nil
}

// Access 实现 auth.Checker 和 gateway.RevocationChecker：判定 token 要的状态都来自存储，转正要写库和踢线。
type Access struct {
	s *Service
}

// Access 返回判定 access token 用的检查器。没有 refresh 密钥时也能用（转正不需要 refresh 密钥）。
func (s *Service) Access() *Access { return &Access{s: s} }

func (a *Access) TokenVersion(ctx context.Context, userID string) (int64, error) {
	return a.s.Store.TokenVersion(ctx, userID)
}

func (a *Access) AccessSessionState(ctx context.Context, sid string) (auth.SessionState, error) {
	return a.s.Store.AccessSessionState(ctx, sid)
}

func (a *Access) AccountAccessState(ctx context.Context, userID string) (auth.AccountState, error) {
	return a.s.Store.AccountAccessState(ctx, userID)
}

func (a *Access) RecoveryUnacked(ctx context.Context, userID string) (bool, error) {
	return a.s.Store.RecoveryUnacked(ctx, userID)
}

func (a *Access) ActiveBan(ctx context.Context, userID string, now int64) (bool, *int64, error) {
	return a.s.Store.ActiveBan(ctx, userID, now)
}

// RecoverBefore 是 kicked 帧的 recoverBefore（规则 6a）。
func (a *Access) RecoverBefore(ctx context.Context, userID string) *int64 {
	return a.s.recoverBefore(ctx, userID)
}

// Promote 实现 auth.Checker：验签通过且未过期的 pending 代 access 首次使用（规则 4）。
// 会话的 refresh 已过期时不转正：转正会把到期时间滑回去，等于用 access 复活一条过期的会话（比规格更保守，见任务单）。
func (a *Access) Promote(ctx context.Context, claims auth.Claims, ip string) auth.Result {
	s := a.s
	for range casRetries {
		d, err := s.Store.GetDeviceSession(ctx, claims.SID)
		if errors.Is(err, store.ErrNotFound) {
			return auth.Result{Claims: claims, Verdict: auth.VerdictSessionRevoked}
		}
		if err != nil {
			return auth.Result{Claims: claims, Verdict: auth.VerdictError, Err: err}
		}
		if d.RevokedAt != 0 {
			return auth.Result{Claims: claims, Verdict: auth.VerdictSessionRevoked, RevokeReason: d.RevokeReason}
		}
		if claims.Gen == d.CurrentGen {
			return auth.Result{Claims: claims, Verdict: auth.VerdictOK}
		}
		i := slices.IndexFunc(d.Pendings, func(p store.PendingGen) bool { return p.Gen == claims.Gen })
		if i < 0 {
			return auth.Result{Claims: claims, Verdict: auth.VerdictSuperseded}
		}
		now := s.Clock.Now()
		ms := now.UnixMilli()
		if ms >= d.ExpiresAt {
			return auth.Result{Claims: claims, Verdict: auth.VerdictExpired}
		}
		nd := d
		s.promote(&nd, d.Pendings[i], now)
		nd.LastSeenAt = ms
		events := seeIP(&nd, ip, ms)
		ok, err := s.Store.UpdateDeviceSession(ctx, nd, events...)
		if err != nil {
			return auth.Result{Claims: claims, Verdict: auth.VerdictError, Err: err}
		}
		if ok {
			// 断开这条 sid 上代号不是新 current 的连接：被攻击者转正时，在线的受害者马上发现。
			s.Router.KickStaleGen(d.UserID, d.SID, nd.CurrentGen)
			s.notifyNewPrefix(events, d.SID)
			return auth.Result{Claims: claims, Verdict: auth.VerdictOK}
		}
	}
	return auth.Result{Claims: claims, Verdict: auth.VerdictError, Err: errors.New("promote: too many concurrent updates")}
}
