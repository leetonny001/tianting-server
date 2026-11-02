// Package app 把各模块组装成一个可运行的服务，main 和 e2e 测试共用。
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"im-mvp/server/internal/admin"
	"im-mvp/server/internal/api"
	"im-mvp/server/internal/auth"
	"im-mvp/server/internal/cache"
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/gateway"
	"im-mvp/server/internal/hub"
	"im-mvp/server/internal/logic"
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/ratelimit"
	"im-mvp/server/internal/store"
)

// Config 是服务配置。
type Config struct {
	DBPath       string
	AutoMigrate  bool       // 启动时自动执行数据库迁移；生产默认关闭，由发布流程执行 migrate up（ARCH 3.1）
	JWTSecret    string     // 单把签名密钥（kid 为 auth.DefaultKeyID）；配置了 JWTKeys 时忽略
	JWTKeys      []auth.Key // 多把签名密钥（密钥轮换，ARCH S5）
	JWTActiveKID string     // 签发新 token 用的密钥 ID
	AdminKey     string
	Gateway      gateway.Config
	TestMode     bool // 开启故障注入、时钟注入、短有效期 token（PRD 6.1 节），生产必须关闭
	DemoLogin    bool // 开启演示登录 /demo/login（PRD F-AUTH-04），生产必须关闭
	SendRate     int  // 每个用户每秒可发消息数，0 表示默认值 20（PRD F-MSG-07）
	Version      string
	Logger       *slog.Logger
}

const defaultSendRate = 20

// App 是组装好的服务。
type App struct {
	handler  http.Handler
	store    *store.SQLite
	hub      *hub.Hub
	draining atomic.Bool
}

func New(cfg Config) (*App, error) {
	clk := clock.New()
	var signer *auth.Signer
	var err error
	if len(cfg.JWTKeys) > 0 {
		signer, err = auth.NewKeyringSigner(cfg.JWTKeys, cfg.JWTActiveKID, clk)
	} else {
		signer, err = auth.NewSigner(cfg.JWTSecret, clk)
	}
	if err != nil {
		return nil, err
	}
	if cfg.AdminKey == "" {
		return nil, fmt.Errorf("admin key is required")
	}
	st, err := store.OpenSQLite(cfg.DBPath, store.SQLiteOptions{AutoMigrate: cfg.AutoMigrate})
	if err != nil {
		return nil, err
	}

	var faults *logic.Faults
	if cfg.TestMode {
		faults = logic.NewFaults()
	}
	h := hub.New()
	sendRate := cfg.SendRate
	if sendRate <= 0 {
		sendRate = defaultSendRate
	}
	inv := cache.NewLocal() // 单节点：失效通知直接在本机回调；集群时换成经消息总线广播的实现
	svc := logic.New(st, h, clk, faults, ratelimit.New(sendRate, sendRate, clk), inv, cfg.Logger)

	a := &App{store: st, hub: h}
	mux := api.NewRouter(api.Info{Version: cfg.Version, StartedAt: time.Now(), Ready: a.ready})
	gw := cfg.Gateway
	if gw.NodeID == "" {
		gw.NodeID = randomNodeID()
	}
	mux.Handle("GET /ws", gateway.New(gw, signer, st, clk, h, svc, cfg.Logger))
	adm := admin.New(cfg.AdminKey, signer, st, h, inv, svc, clk, cfg.TestMode, cfg.Logger)
	adm.Register(mux)
	if cfg.TestMode {
		adm.RegisterTestRoutes(mux, faults, clk)
	}
	if cfg.DemoLogin {
		adm.RegisterDemoLogin(mux)
	}

	a.handler = mux
	return a, nil
}

func (a *App) Handler() http.Handler { return a.handler }

func (a *App) ready(ctx context.Context) error {
	if a.draining.Load() {
		return errors.New("draining")
	}
	return a.store.Ping(ctx)
}

// StopAccepting 把 /readyz 置为不可用，让负载均衡不再分配新连接（优雅下线第一步）。
func (a *App) StopAccepting() { a.draining.Store(true) }

// KickAll 通知本节点全部在线连接服务端即将下线（kicked/serverShutdown，客户端退避重连），返回连接数。
func (a *App) KickAll() int { return a.hub.KickAll(protocol.KickShutdown) }

func (a *App) Close() error { return a.store.Close() }

// randomNodeID 在没有配置 IM_NODE_ID 时生成节点标识，例如 "n3f9a1c"。
func randomNodeID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return "n" + hex.EncodeToString(b)
}
