package account

import (
	"context"
	"time"

	"tianting/server/internal/auth"
	"tianting/server/internal/clientip"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
	"tianting/server/internal/testhook"
)

const cleanupBatch = 100

// Cleaner 做 account_cleanup 的第 ② 步（F-ACC-08 规则 3）：删好友、黑名单，逐群退出，昵称改"已注销用户"。由 logic 实现（账号包不依赖 logic）。
// check 在每一步之前调用，返回错误时中止（测试模式的故障注入）。幂等，可以重试。
type Cleaner interface {
	CleanupDeletedUser(ctx context.Context, userID string, check func() error) error
}

// Delete 是 POST /v1/account/delete（F-ACC-08 规则 1、2）：冷静期从现在算，立即吊销全部会话，在线连接 kicked/accountDeleted。
func (s *Service) Delete(ctx context.Context, claims auth.Claims, in PasswordOnlyInput, ip string) *protocol.Error {
	if !s.available() {
		return unavailable()
	}
	v, e := s.verifyPassword(ctx, claims.UserID, in.Password, ip, GateGeneral)
	if e != nil {
		return e
	}
	defer v.done()
	v.release()
	v.done() // 哈希算完了：不再占着这个用户名
	testhook.AfterArgon(v.attempt.UsernameHash)
	ms := s.Clock.Now().UnixMilli()
	if _, err := s.Store.CommitCredentials(ctx, store.CredentialCommit{
		UserID: claims.UserID, Version: v.acct.CredentialsVersion, CallerSID: claims.SID, Now: ms, RequestDeletion: true,
		RevokeReason: protocol.ReasonAccountDeleted, ThrottleKey: v.attempt.PairKey(),
		Event:        store.AccountEvent{UserID: claims.UserID, Kind: EventDeletionRequest, At: ms, IPPrefix: clientip.EventPrefix(ip), SIDHash: SIDHash(claims.SID)},
		BeforeCommit: s.Faults.takeRevokeWrite,
	}); err != nil {
		return s.commitErr(err, claims.UserID)
	}
	s.Router.KickUserExcept(claims.UserID, "", protocol.KickAccountDeleted, s.recoverBefore(ctx, claims.UserID))
	s.Log.Info("account deletion requested", "uid", claims.UserID)
	return nil
}

// DeleteCancelInput 是 POST /v1/account/delete/cancel。
type DeleteCancelInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CancelDelete 是匿名接口（规则 2，RC2-05）：和登录共用限流和相同响应。条件带版本和申请时间（RS5-21）：在途的撤销撤不掉 recover 之后的新申请。
// 没有申请时按成功处理：只有知道密码的人能走到这里。
func (s *Service) CancelDelete(ctx context.Context, in DeleteCancelInput, ip string) *protocol.Error {
	if !s.available() {
		return unavailable()
	}
	v, e := s.verifyPassword(ctx, NormalizeUsername(in.Username), in.Password, ip, GateGeneral)
	if e != nil {
		return e
	}
	defer v.done()
	v.release()
	v.done() // 哈希算完了：不再占着这个用户名
	testhook.AfterArgon(v.attempt.UsernameHash)
	if v.acct.DeleteRequestedAt == 0 {
		return nil
	}
	ms := s.Clock.Now().UnixMilli()
	err := s.Store.CancelDeletion(ctx, v.acct.UserID, v.acct.CredentialsVersion, v.acct.DeleteRequestedAt, ms,
		store.AccountEvent{UserID: v.acct.UserID, Kind: EventDeletionCanceled, At: ms, IPPrefix: clientip.EventPrefix(ip)})
	if err != nil {
		return s.commitErr(err, v.acct.UserID)
	}
	return nil
}

// Cleanup 是 account_cleanup（规则 3）：① 冷静期到了的账号，事务内标记 users.deleted_at、递增 token_version、删密码和恢复码哈希，
// 条件包含"注销申请没有被撤销"（F-ACC-04 规则 8）；② 幂等地做剩下的，下次任务继续。
func (s *Service) Cleanup(ctx context.Context) (int, error) {
	now := s.Clock.Now()
	due, err := s.Store.DueDeletions(ctx, now.Add(-time.Duration(deletionCoolingMs)*time.Millisecond).UnixMilli(), cleanupBatch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, uid := range due {
		ok, err := s.Store.FinishDeletion(ctx, uid, now.Add(-time.Duration(deletionCoolingMs)*time.Millisecond).UnixMilli(), now.UnixMilli())
		if err != nil {
			return n, err
		}
		if ok {
			n++
			s.Router.KickUserExcept(uid, "", protocol.KickAccountDeleted, nil)
		}
	}
	if s.Cleaner == nil {
		return n, nil
	}
	pending, err := s.Store.PendingCleanups(ctx, cleanupBatch)
	if err != nil {
		return n, err
	}
	for _, uid := range pending {
		if err := s.Cleaner.CleanupDeletedUser(ctx, uid, s.Faults.takeCleanupStep); err != nil {
			return n, err
		}
		if err := s.Store.MarkCleanupDone(ctx, uid, now.UnixMilli()); err != nil {
			return n, err
		}
	}
	return n, nil
}
