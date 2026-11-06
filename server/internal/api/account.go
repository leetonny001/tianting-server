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
