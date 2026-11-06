package auth

import (
	"context"
	"errors"
)

// Verdict 是一个 token 的判定结果，WS auth 和 /v1/* 的 Bearer 共用（PRD-chat 第 4.2 节第 3 条）。
type Verdict int

const (
	VerdictOK             Verdict = iota
	VerdictInvalid                // 4010
	VerdictRevoked                // 4012：令牌版本号落后（强制下线、封禁）
	VerdictSessionRevoked         // 4012：设备会话被吊销，原因在 RevokeReason
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
}

// Checker 提供判定需要的状态。
type Checker interface {
	TokenVersion(ctx context.Context, userID string) (int64, error)
	// AccessSessionState 只在 token 有 sid 时调用。
	AccessSessionState(ctx context.Context, sid string) (SessionState, error)
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

// Check 按固定顺序判定（C-08 + PRD-chat F-ACC-03 规则 2、4）：签名 4010 → 版本号 4012 → 会话吊销 4012 → 过期 4011 → 代号 4011。
//
// 代号只比较 current：BE-415 之前会话没有 pending。pending 的转正随 BE-416 加在这里，HTTP 和 WS 不用分别改。
// 没有 sid 的 token 只走版本号和过期，和以前一样。
func Check(ctx context.Context, v TokenVerifier, c Checker, token string) Result {
	claims, verr := v.Verify(token)
	if errors.Is(verr, ErrInvalidToken) {
		return Result{Verdict: VerdictInvalid}
	}
	version, err := c.TokenVersion(ctx, claims.UserID)
	if err != nil {
		return Result{Claims: claims, Verdict: VerdictError, Err: err}
	}
	if claims.Version < version {
		return Result{Claims: claims, Verdict: VerdictRevoked}
	}
	var st SessionState
	if claims.SID != "" {
		if st, err = c.AccessSessionState(ctx, claims.SID); err != nil {
			return Result{Claims: claims, Verdict: VerdictError, Err: err}
		}
		if !st.Found || st.RevokedAt != 0 {
			return Result{Claims: claims, Verdict: VerdictSessionRevoked, RevokeReason: st.RevokeReason}
		}
	}
	if errors.Is(verr, ErrExpiredToken) {
		return Result{Claims: claims, Verdict: VerdictExpired}
	}
	if claims.SID != "" && claims.Gen != st.CurrentGen {
		return Result{Claims: claims, Verdict: VerdictSuperseded}
	}
	return Result{Claims: claims, Verdict: VerdictOK}
}
