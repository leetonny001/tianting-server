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
	Version   string
	Logger    *slog.Logger
}

// App 是组装好的服务。
type App struct {
	handler http.Handler
	store   *store.SQLite
}

func New(cfg Config) (*App, error) {
	signer, err := auth.NewSigner(cfg.JWTSecret)
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

	h := hub.New()
	svc := logic.New(st, h, cfg.Logger)

	mux := api.NewRouter(api.Info{Version: cfg.Version, StartedAt: time.Now()})
	mux.Handle("GET /ws", gateway.New(cfg.Gateway, signer, h, svc, cfg.Logger))
	admin.New(cfg.AdminKey, signer, st, cfg.Logger).Register(mux)

	return &App{handler: mux, store: st}, nil
}

func (a *App) Handler() http.Handler { return a.handler }

func (a *App) Close() error { return a.store.Close() }
