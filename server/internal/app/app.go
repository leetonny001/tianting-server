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
	"net/netip"
	"sync/atomic"
	"time"

	"tianting/server/internal/admin"
	"tianting/server/internal/api"
	"tianting/server/internal/auth"
	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/gateway"
	"tianting/server/internal/hub"
	"tianting/server/internal/logic"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// Config 是服务配置。
type Config struct {
	DBPath       string
	AutoMigrate  bool       // 启动时自动执行数据库迁移；生产默认关闭，由发布流程执行 migrate up（ARCH 3.1）
	JWTSecret    string     // 单把签名密钥（kid 为 auth.DefaultKeyID）；配置了 JWTKeys 时忽略
	JWTKeys      []auth.Key // 多把签名密钥（密钥轮换，ARCH S5）
	JWTActiveKID string     // 签发新 token 用的密钥 ID
	AdminKey     string
	// AdminAllowedCIDRs 非空时，管理端口只接受来自这些网段的 TCP 连接（只看对端地址，不解析 X-Forwarded-For）。
	AdminAllowedCIDRs []netip.Prefix
	Gateway           gateway.Config
	TestMode          bool // 开启故障注入、时钟注入、短有效期 token（PRD 6.1 节），生产必须关闭
	DemoLogin         bool // 开启演示登录 /demo/login（PRD F-AUTH-04），生产必须关闭
	SendRate          int  // 每个用户每秒可发消息数，0 表示默认值 20（PRD F-MSG-07）
	Version           string
	Logger            *slog.Logger
}

const defaultSendRate = 20

// App 是组装好的服务。
type App struct {
	handler  http.Handler
	admin    http.Handler
	store    *store.SQLite
	hub      *hub.Hub
	metrics  *metrics.Metrics
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
	m := metrics.New()
	inv := cache.NewLocal() // 单节点：失效通知直接在本机回调；集群时换成经消息总线广播的实现
	svc := logic.New(st, h, clk, faults, ratelimit.New(sendRate, sendRate, clk), inv, m, cfg.Logger)

	a := &App{store: st, hub: h, metrics: m}
	mux := api.NewRouter(api.Info{Version: cfg.Version, StartedAt: time.Now(), Ready: a.ready})
	gw := cfg.Gateway
	if gw.NodeID == "" {
		gw.NodeID = randomNodeID()
	}
	mux.Handle("GET /ws", gateway.New(gw, signer, st, clk, h, svc, m, cfg.Logger))
	// 管理接口只注册在单独的管理 mux 上（PRD-admin F-KEY-01）：对外端口上请求 /admin/* 是 404。
	adm := admin.New(cfg.AdminKey, signer, st, h, inv, svc, clk, cfg.TestMode, cfg.Logger)
	adminMux := http.NewServeMux()
	adm.Register(adminMux)
	if cfg.TestMode {
		adm.RegisterTestRoutes(adminMux, faults, clk)
	}
	if cfg.DemoLogin {
		adm.RegisterDemoLogin(mux) // 演示登录给浏览器用，留在对外端口
	}

	a.handler = mux
	a.admin = admin.AllowCIDRs(cfg.AdminAllowedCIDRs, adminMux)
	return a, nil
}

// Handler 是对外端口：/ws、/healthz、/readyz，以及演示环境的 /demo/login。
func (a *App) Handler() http.Handler { return a.handler }

// AdminHandler 是管理端口（IM_ADMIN_ADDR）上的 /admin/*，由 main 单独监听。
func (a *App) AdminHandler() http.Handler { return a.admin }

// MetricsHandler 是 /metrics。由 main 挂在单独的内网地址上（IM_METRICS_ADDR），不和对外端口共用。
func (a *App) MetricsHandler() http.Handler { return a.metrics.Handler() }

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
