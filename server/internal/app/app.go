// Package app 把各模块组装成一个可运行的服务，main 和 e2e 测试共用。
package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"im-mvp/server/internal/admin"
	"im-mvp/server/internal/api"
	"im-mvp/server/internal/auth"
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/gateway"
	"im-mvp/server/internal/hub"
	"im-mvp/server/internal/logic"
	"im-mvp/server/internal/store"
)

// Config 是服务配置。
type Config struct {
	DBPath    string
	JWTSecret string
	AdminKey  string
	Gateway   gateway.Config
	TestMode  bool // 开启故障注入、时钟注入、短有效期 token（PRD 6.1 节），生产必须关闭
	DemoLogin bool // 开启演示登录 /demo/login（PRD F-AUTH-04），生产必须关闭
	Version   string
	Logger    *slog.Logger
}

// App 是组装好的服务。
type App struct {
	handler http.Handler
	store   *store.SQLite
}

func New(cfg Config) (*App, error) {
	clk := clock.New()
	signer, err := auth.NewSigner(cfg.JWTSecret, clk)
	if err != nil {
		return nil, err
	}
	if cfg.AdminKey == "" {
		return nil, fmt.Errorf("admin key is required")
	}
	st, err := store.OpenSQLite(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	var faults *logic.Faults
	if cfg.TestMode {
		faults = logic.NewFaults()
	}
	h := hub.New()
	svc := logic.New(st, h, clk, faults, cfg.Logger)

	mux := api.NewRouter(api.Info{Version: cfg.Version, StartedAt: time.Now()})
	mux.Handle("GET /ws", gateway.New(cfg.Gateway, signer, st, clk, h, svc, cfg.Logger))
	adm := admin.New(cfg.AdminKey, signer, st, h, clk, cfg.TestMode, cfg.Logger)
	adm.Register(mux)
	if cfg.TestMode {
		adm.RegisterTestRoutes(mux, faults, clk)
	}
	if cfg.DemoLogin {
		adm.RegisterDemoLogin(mux)
	}

	return &App{handler: mux, store: st}, nil
}

func (a *App) Handler() http.Handler { return a.handler }

func (a *App) Close() error { return a.store.Close() }
