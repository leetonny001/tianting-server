// Package api 提供客户端和运维用的 HTTP 接口。
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Info 是服务的静态信息，由 main 在启动时传入。
type Info struct {
	Version   string
	StartedAt time.Time
}

// NewRouter 注册客户端和运维用的 HTTP 路由；/ws、/admin 由 app 包另外挂载。
func NewRouter(info Info) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(info))
	return mux
}

type healthResp struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
}

func healthz(info Info) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResp{
			Status:        "ok",
			Version:       info.Version,
			UptimeSeconds: int64(time.Since(info.StartedAt).Seconds()),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
