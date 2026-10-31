// Command im-server 启动 IM 服务端。
//
// 配置走环境变量：
//
//	IM_ADDR  监听地址，默认 :8080
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"im-mvp/server/internal/api"
)

// version 构建时用 -ldflags "-X main.version=x.y.z" 注入。
var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := os.Getenv("IM_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(api.Info{Version: version, StartedAt: time.Now()}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 收到 Ctrl+C / SIGTERM 时优雅退出：停止接新连接，等已有请求处理完。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("im-server listening", "addr", addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
}
