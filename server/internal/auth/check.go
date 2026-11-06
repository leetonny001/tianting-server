package auth

import (
	"context"
	"errors"
	"slices"
)

// Verdict 是一个 token 的判定结果，WS auth 和 /v1/* 的 Bearer 共用（PRD-chat 第 4.2 节第 3 条）。
type Verdict int

const (
	VerdictOK             Verdict = iota
	VerdictInvalid                // 4010
	VerdictRevoked                // 4012：令牌版本号落后（强制下线、封禁）
	VerdictSessionRevoked         // 4012：设备会话被吊销，原因在 RevokeReason
	VerdictAccountDeleted         // 4012：账号在注销冷静期或已注销（F-ACC-08 异常），不看 token 的其他状态
	VerdictCredentialsOld         // 4012：没有 sid 的 token 早于 credentials_changed_at（F-ACC-03 规则 11）
	VerdictExpired                // 4011
	VerdictSuperseded             // 4011：被替换的代（WS kicked/tokenExpired；HTTP reason=tokenSuperseded）
	VerdictError                  // 查库失败，调用方按内部错误处理，不当成凭证问题
)

// SessionState 是判定 access token 需要的设备会话状态。
type SessionState struct {
	Found        bool
	RevokedAt    int64
	RevokeReason string
	CurrentGen   int64
	PendingGens  []int64
}

// AccountState 是判定 token 需要的账号状态。演示和导入用户是零值。
type AccountState struct {
	CredentialsChangedAt int64
	Deleting             bool // 注销冷静期内或已注销
}

// Checker 提供判定需要的状态。
type Checker interface {
	TokenVersion(ctx context.Context, userID string) (int64, error)
	// AccessSessionState 只在 token 有 sid 时调用。
	AccessSessionState(ctx context.Context, sid string) (SessionState, error)
	AccountAccessState(ctx context.Context, userID string) (AccountState, error)
	// Promote 把 pending 代转正（F-ACC-03 规则 4）：只在 token 验签通过、没过期、代号属于 pending 时调用。
	// 返回 VerdictOK 表示已经是 current；其他结果原样作为判定结果（并发下可能已经被替换或吊销）。
	Promote(ctx context.Context, claims Claims, ip string) Result
}

// TokenVerifier 校验 token 的签名和有效期。
type TokenVerifier interface {
	Verify(token string) (Claims, error)
}

// Result 是 Check 的结果。
type Result struct {
	Claims       Claims
	Verdict      Verdict
	RevokeReason string // VerdictSessionRevoked 时的吊销原因
	Err          error  // VerdictError 时的原因
}

// Check 按固定顺序判定（C-08 + PRD-chat F-ACC-03 规则 2、4、11）：签名 4010 → 账号状态 4012（RC2-05）→ 版本号 4012 → 无 sid 的改密时间 4012 →
// 会话吊销 4012 → 过期 4011 → 代号：current 放行，pending 转正放行，其他 4011，不转正，不吊销。
//
// 过期或验签失败的 token 走不到转正。没有 sid 的 token 没有代号判定，和以前一样。ip 是转正时比较 IP 前缀用的（规则 4）。
func Check(ctx context.Context, v TokenVerifier, c Checker, token, ip string) Result {
	claims, verr := v.Verify(token)
	if errors.Is(verr, ErrInvalidToken) {
		return Result{Verdict: VerdictInvalid}
	}
	st, res, done := checkState(ctx, c, claims)
	if done {
		return res
	}
	if errors.Is(verr, ErrExpiredToken) {
		return Result{Claims: claims, Verdict: VerdictExpired}
	}
	if claims.SID == "" || claims.Gen == st.CurrentGen {
		return Result{Claims: claims, Verdict: VerdictOK}
	}
	if slices.Contains(st.PendingGens, claims.Gen) {
		return c.Promote(ctx, claims, ip)
	}
	return Result{Claims: claims, Verdict: VerdictSuperseded}
}

// Recheck 是连接入表后和周期复查（规则 2，RS5-08）：覆盖 sid、代号、版本号和 credentials_changed_at，不看过期（有到期定时器），不转正。
// 查库失败是 VerdictError：调用方决定断不断开（失败关闭，RS5-09）。
func Recheck(ctx context.Context, c Checker, claims Claims) Result {
	st, res, done := checkState(ctx, c, claims)
	if done {
		return res
	}
	if claims.SID != "" && claims.Gen != st.CurrentGen {
		return Result{Claims: claims, Verdict: VerdictSuperseded}
	}
	return Result{Claims: claims, Verdict: VerdictOK}
}

// checkState 是和 token 有效期、代号无关的部分。done=true 时 res 就是结果。
func checkState(ctx context.Context, c Checker, claims Claims) (st SessionState, res Result, done bool) {
	fail := func(err error) (SessionState, Result, bool) {
		return st, Result{Claims: claims, Verdict: VerdictError, Err: err}, true
	}
	acct, err := c.AccountAccessState(ctx, claims.UserID)
	if err != nil {
		return fail(err)
	}
	if acct.Deleting {
		return st, Result{Claims: claims, Verdict: VerdictAccountDeleted}, true
	}
	version, err := c.TokenVersion(ctx, claims.UserID)
	if err != nil {
		return fail(err)
	}
	if claims.Version < version {
		return st, Result{Claims: claims, Verdict: VerdictRevoked}, true
	}
	if claims.SID == "" {
		if acct.CredentialsChangedAt != 0 && claims.IssuedAt.UnixMilli() < acct.CredentialsChangedAt {
			return st, Result{Claims: claims, Verdict: VerdictCredentialsOld}, true
		}
		return st, Result{}, false
	}
	if st, err = c.AccessSessionState(ctx, claims.SID); err != nil {
		return fail(err)
	}
	if !st.Found || st.RevokedAt != 0 {
		return st, Result{Claims: claims, Verdict: VerdictSessionRevoked, RevokeReason: st.RevokeReason}, true
	}
	return st, Result{}, false
}
