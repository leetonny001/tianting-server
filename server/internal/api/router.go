// Package api 提供客户端和运维用的 HTTP 接口。
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Info 是服务的静态信息，由 main 在启动时传入。
type Info struct {
	Version   string
	StartedAt time.Time
	// Ready 返回 nil 表示可以接收流量（/readyz），由负载均衡、K8s 探测（ARCH 2.4）。
	// 为 nil 时 /readyz 总是就绪。
	Ready func(ctx context.Context) error
}

// NewRouter 注册客户端和运维用的 HTTP 路由；/ws、/admin 由 app 包另外挂载。
func NewRouter(info Info) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(info))
	mux.HandleFunc("GET /readyz", readyz(info))
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

// readyz 与 healthz 分开：healthz 只说明进程活着；readyz 说明依赖可用、且没有在下线，
// 负载均衡据此决定是否把新连接分过来。
func readyz(info Info) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if info.Ready != nil {
			if err := info.Ready(r.Context()); err != nil {
				// 不把内部错误细节返回给调用方。
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
