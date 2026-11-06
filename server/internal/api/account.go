package api

import (
	"context"
	"net/http"

	"tianting/server/internal/account"
	"tianting/server/internal/auth"
	"tianting/server/internal/protocol"
)

// AccountService 是 /v1/account/* 用到的账号服务。
type AccountService interface {
	Register(ctx context.Context, in account.RegisterInput, ip string) (*account.AuthResponse, *protocol.Error)
	Login(ctx context.Context, in account.LoginInput, ip string) (*account.AuthResponse, *protocol.Error)
	Events(ctx context.Context, userID, sid string) (*account.EventsResponse, *protocol.Error)
	Refresh(ctx context.Context, in account.RefreshInput, ip string) (*account.AuthResponse, *protocol.Error)
	Logout(ctx context.Context, in account.LogoutInput, bearer *auth.Result) *protocol.Error
	Password(ctx context.Context, claims auth.Claims, in account.PasswordInput, ip string) (*account.AuthResponse, *protocol.Error)
	LogoutOthers(ctx context.Context, claims auth.Claims, in account.PasswordOnlyInput, ip string) (*account.AuthResponse, *protocol.Error)
	Recover(ctx context.Context, in account.RecoverInput, ip string) (*account.AuthResponse, *protocol.Error)
	RegenerateRecovery(ctx context.Context, claims auth.Claims, in account.PasswordOnlyInput, ip string) (*account.RecoveryCodeResponse, *protocol.Error)
	AckRecovery(ctx context.Context, userID string, in account.RecoveryAckInput) *protocol.Error
}

// RegisterAccount 注册 /v1/account/*（PRD-chat 第 11.2 节）。
func (v *V1) RegisterAccount(mux *http.ServeMux, svc AccountService) {
	v.Handle(mux, "POST /v1/account/register", func(w http.ResponseWriter, r *http.Request) {
		var in account.RegisterInput
		if !v.ReadBody(w, r, &in) {
			return
		}
		writeResult(w, firstOf(svc.Register(r.Context(), in, v.Proxies.IP(r))))
	})
	v.Handle(mux, "POST /v1/account/login", func(w http.ResponseWriter, r *http.Request) {
		var in account.LoginInput
		if !v.ReadBody(w, r, &in) {
			return
		}
		writeResult(w, firstOf(svc.Login(r.Context(), in, v.Proxies.IP(r))))
	})
	v.Handle(mux, "POST /v1/account/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		var in account.RefreshInput
		if !v.ReadBody(w, r, &in) {
			return
		}
		writeResult(w, firstOf(svc.Refresh(r.Context(), in, v.Proxies.IP(r))))
	})
	// logout 不走 Bearer 中间件：Bearer 过期或被替换不影响（F-ACC-04 规则 1，RS5-18）。
	v.Handle(mux, "POST /v1/account/logout", func(w http.ResponseWriter, r *http.Request) {
		var in account.LogoutInput
		if !v.ReadBody(w, r, &in) {
			return
		}
		res, hasBearer := v.PeekBearer(r)
		var bearer *auth.Result
		if hasBearer {
			// 规则 11：没有 sid 的 token 不能调账号接口。带了有效 refresh 时不看 Bearer。
			if res.Verdict == auth.VerdictOK && res.Claims.SID == "" && in.RefreshToken == "" {
				WriteError(w, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonAccountSessionRequired, "account session required"))
				return
			}
			bearer = &res
		}
		if e := svc.Logout(r.Context(), in, bearer); e != nil {
			WriteError(w, e)
			return
		}
		if in.RefreshToken == "" && (bearer == nil || bearer.Verdict != auth.VerdictOK) {
			// 两个都没有：按 Bearer 的结果回错误（没带就是 4010）。
			if !hasBearer {
				WriteError(w, protocol.Errorf(protocol.CodeTokenInvalid, "missing bearer token"))
				return
			}
			v.writeVerdict(w, res)
			return
		}
		WriteJSON(w, struct{}{})
	})
	v.Handle(mux, "POST /v1/account/password", func(w http.ResponseWriter, r *http.Request) {
		var in account.PasswordInput
		claims, ok := v.accountBearer(w, r, &in)
		if ok {
			writeResult(w, firstOf(svc.Password(r.Context(), claims, in, v.Proxies.IP(r))))
		}
	})
	v.Handle(mux, "POST /v1/account/logout-others", func(w http.ResponseWriter, r *http.Request) {
		var in account.PasswordOnlyInput
		claims, ok := v.accountBearer(w, r, &in)
		if ok {
			writeResult(w, firstOf(svc.LogoutOthers(r.Context(), claims, in, v.Proxies.IP(r))))
		}
	})
	v.Handle(mux, "POST /v1/account/recover", func(w http.ResponseWriter, r *http.Request) {
		var in account.RecoverInput
		if v.ReadBody(w, r, &in) {
			writeResult(w, firstOf(svc.Recover(r.Context(), in, v.Proxies.IP(r))))
		}
	})
	v.Handle(mux, "POST /v1/account/recovery-code", func(w http.ResponseWriter, r *http.Request) {
		var in account.PasswordOnlyInput
		claims, ok := v.accountBearer(w, r, &in)
		if ok {
			writeResult(w, firstOf(svc.RegenerateRecovery(r.Context(), claims, in, v.Proxies.IP(r))))
		}
	})
	v.Handle(mux, "POST /v1/account/recovery-code/ack", func(w http.ResponseWriter, r *http.Request) {
		var in account.RecoveryAckInput
		claims, ok := v.accountBearer(w, r, &in)
		if !ok {
			return
		}
		if e := svc.AckRecovery(r.Context(), claims.UserID, in); e != nil {
			WriteError(w, e)
			return
		}
		WriteJSON(w, struct{}{})
	})
	v.Handle(mux, "GET /v1/account/events", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := v.Bearer(w, r)
		if !ok {
			return
		}
		// 第 11.2 节：要有 sid。管理接口和演示登录的 token 看不到账号事件。
		if claims.SID == "" {
			WriteError(w, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonAccountSessionRequired, "account session required"))
			return
		}
		writeResult(w, firstOf(svc.Events(r.Context(), claims.UserID, claims.SID)))
	})
}

// accountBearer 是要 sid 的账号接口（F-ACC-03 规则 11）：先验 Bearer，没有 sid 的 403 / 4030，再读请求体。失败时已经写好响应。
func (v *V1) accountBearer(w http.ResponseWriter, r *http.Request, in any) (auth.Claims, bool) {
	claims, ok := v.Bearer(w, r)
	if !ok {
		return claims, false
	}
	if claims.SID == "" {
		WriteError(w, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonAccountSessionRequired, "account session required"))
		return claims, false
	}
	return claims, in == nil || v.ReadBody(w, r, in)
}

type result struct {
	body any
	err  *protocol.Error
}

func firstOf[T any](body *T, err *protocol.Error) result {
	if err != nil {
		return result{err: err}
	}
	return result{body: body}
}

func writeResult(w http.ResponseWriter, r result) {
	if r.err != nil {
		WriteError(w, r.err)
		return
	}
	WriteJSON(w, r.body)
}
