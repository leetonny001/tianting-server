// Command im-server 启动 IM 服务端。
//
// 配置走环境变量：
//
//	IM_ADDR        监听地址，默认 :8080
//	IM_DB          SQLite 文件路径，默认 im.db
//	IM_JWT_SECRET  用户 token 签名密钥（>= 16 字节），不设置时使用开发密钥
//	IM_ADMIN_KEY   管理接口密钥（请求头 X-Admin-Key），不设置时使用开发密钥
//	IM_TEST_MODE   设为 1 开启测试模式：故障注入、时钟注入、短有效期 token（生产禁止）
//	IM_IDLE_TIMEOUT 空闲超时（如 90s），默认 90s
package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"im-mvp/server/internal/app"
	"im-mvp/server/internal/gateway"
)

// version 构建时用 -ldflags "-X main.version=x.y.z" 注入。
var version = "dev"

const (
	devJWTSecret = "dev-secret-change-me-0123456789"
	devAdminKey  = "dev-admin-key"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := env("IM_ADDR", ":8080")
	jwtSecret := env("IM_JWT_SECRET", "")
	adminKey := env("IM_ADMIN_KEY", "")
	if jwtSecret == "" || adminKey == "" {
		logger.Warn("IM_JWT_SECRET / IM_ADMIN_KEY not set, using development defaults — never do this in production")
		jwtSecret = cmp.Or(jwtSecret, devJWTSecret)
		adminKey = cmp.Or(adminKey, devAdminKey)
	}

	gw := gateway.DefaultConfig()
	if v := os.Getenv("IM_IDLE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			logger.Error("invalid IM_IDLE_TIMEOUT", "value", v, "err", err)
			os.Exit(1)
		}
		gw.IdleTimeout = d
	}
	testMode := os.Getenv("IM_TEST_MODE") == "1"
	if testMode {
		logger.Warn("!!! TEST MODE ENABLED: fault injection, clock injection and 1s tokens are available — NEVER use in production !!!")
	}

	a, err := app.New(app.Config{
		DBPath:    env("IM_DB", "im.db"),
		JWTSecret: jwtSecret,
		AdminKey:  adminKey,
		Gateway:   gw,
		TestMode:  testMode,
		Version:   version,
		Logger:    logger,
	})
	if err != nil {
		logger.Error("init failed", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           a.Handler(),
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

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
