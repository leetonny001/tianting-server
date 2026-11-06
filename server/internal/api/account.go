package api

import (
	"context"
	"net/http"

	"tianting/server/internal/account"
	"tianting/server/internal/protocol"
)

// AccountService 是 /v1/account/* 用到的账号服务。
type AccountService interface {
	Register(ctx context.Context, in account.RegisterInput, ip string) (*account.AuthResponse, *protocol.Error)
	Login(ctx context.Context, in account.LoginInput, ip string) (*account.AuthResponse, *protocol.Error)
	Events(ctx context.Context, userID, sid string) (*account.EventsResponse, *protocol.Error)
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
