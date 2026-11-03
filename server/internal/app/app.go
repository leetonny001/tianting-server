// Package app 把各模块组装成一个可运行的服务，main 和 e2e 测试共用。
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"

	"tianting/server/internal/admin"
	"tianting/server/internal/adminauth"
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
	"tianting/server/internal/social"
	"tianting/server/internal/store"
)

// Config 是服务配置。
type Config struct {
	DBPath       string
	AutoMigrate  bool       // 启动时自动执行数据库迁移；生产默认关闭，由发布流程执行 migrate up（ARCH 3.1）
	JWTSecret    string     // 单把签名密钥（kid 为 auth.DefaultKeyID）；配置了 JWTKeys 时忽略
	JWTKeys      []auth.Key // 多把签名密钥（密钥轮换，ARCH S5）
	JWTActiveKID string     // 签发新 token 用的密钥 ID
	AdminKey     string     // 旧的共享管理密钥（调用方 legacy，只能访问 v1.2 已有接口，PRD-admin F-KEY-05）
	// AdminLegacyDisabled 为 true 时不接受旧密钥：生产环境默认如此，除非设置 IM_ADMIN_LEGACY=1。
	AdminLegacyDisabled bool
	// AdminCallers 是签名调用方（IM_ADMIN_CALLERS，PRD-admin F-KEY-02）。
	AdminCallers []adminauth.Caller
	// ReportLegacyAdmin 为 true 时 /readyz 带 legacyAdminKey: true（生产环境开启了旧密钥时的告警）。
	ReportLegacyAdmin bool
	// C2CBodyAccess 对应 IM_C2C_BODY_ACCESS（默认关闭）：只能改配置重启，不能通过任何管理接口打开（X-7）。
	C2CBodyAccess bool
	// AdminAllowedCIDRs 非空时，管理端口只接受来自这些网段的 TCP 连接（只看对端地址，不解析 X-Forwarded-For）。
	AdminAllowedCIDRs []netip.Prefix
	Gateway           gateway.Config
	// SocialDefaults 是 IM_SOCIAL_DEFAULTS（JSON，字段同 PRD-social 5.1 节）：只在库里没有配置行时写入一次。
	SocialDefaults string
	// TestPeers / TestInvalidationOff：测试模式的双节点环境（IM_TEST_PEERS、IM_TEST_INVALIDATION=off，PRD-social 5.6 节）。
	TestPeers           []string
	TestInvalidationOff bool
	TestMode            bool // 开启故障注入、时钟注入、短有效期 token（PRD 6.1 节），生产必须关闭
	DemoLogin           bool // 开启演示登录 /demo/login（PRD F-AUTH-04），生产必须关闭
	SendRate            int  // 每个用户每秒可发消息数，0 表示默认值 20（PRD F-MSG-07）
	Version             string
	Logger              *slog.Logger
}

const defaultSendRate = 20

// App 是组装好的服务。
type App struct {
	handler http.Handler
	admin   http.Handler
	// adminRoutes 是已注册的管理路由（测试遍历全部路由检查 scope）。
	adminRoutes []admin.RouteInfo
	store       *store.SQLite
	hub         *hub.Hub
	metrics     *metrics.Metrics
	draining    atomic.Bool
	stop        chan struct{} // 关闭后台任务（社交清理）
	loopback    *cache.HTTPLoopback
}

// SetTestPeers 设置双节点测试环境的广播目标（只在测试模式下有效；TestInvalidationOff 时忽略）。
func (a *App) SetTestPeers(peers []string) {
	if a.loopback != nil {
		a.loopback.SetPeers(peers)
	}
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
	if cfg.AdminKey == "" && len(cfg.AdminCallers) == 0 {
		return nil, fmt.Errorf("admin credentials are required (IM_ADMIN_CALLERS or IM_ADMIN_KEY)")
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
	var inv cache.Invalidator = cache.NewLocal() // 单节点：失效通知直接在本机回调；集群时换成经消息总线广播的实现
	var loopback *cache.HTTPLoopback
	if cfg.TestMode { // 测试模式：可以组成双节点环境（没有 peers 时与 Local 相同）
		loopback = cache.NewHTTPLoopback(cfg.TestPeers, cfg.AdminKey, cfg.TestInvalidationOff)
		inv = loopback
	}
	svc := logic.New(st, h, clk, faults, ratelimit.New(sendRate, sendRate, clk), inv, m, cfg.Logger)
	if cfg.SocialDefaults != "" {
		if err := initSocialDefaults(st, cfg.SocialDefaults, clk.Now().UnixMilli()); err != nil {
			st.Close()
			return nil, err
		}
	}

	if cfg.TestMode { // 测试模式启动时抽样校验计数器，不一致记 ERROR（PRD-social 8.5 节，BE-100）
		if n, err := st.CounterDrift(context.Background(), 100); err != nil || n > 0 {
			cfg.Logger.Error("social counters drifted; run `im-server repair counters`", "rows", n, "err", err)
		}
	}

	a := &App{store: st, hub: h, metrics: m, stop: make(chan struct{}), loopback: loopback}
	go svc.RunSocialPurge(a.stop)
	info := api.Info{Version: cfg.Version, StartedAt: time.Now(), Ready: a.ready}
	if cfg.ReportLegacyAdmin {
		info.Details = map[string]any{"legacyAdminKey": true}
	}
	mux := api.NewRouter(info)
	gw := cfg.Gateway
	if gw.NodeID == "" {
		gw.NodeID = randomNodeID()
	}
	mux.Handle("GET /ws", gateway.New(gw, signer, st, clk, h, svc, m, cfg.Logger))
	// 管理接口只注册在单独的管理 mux 上（PRD-admin F-KEY-01）：对外端口上请求 /admin/* 是 404。
	adm := admin.New(admin.AuthConfig{
		LegacyKey: cfg.AdminKey, LegacyDisabled: cfg.AdminLegacyDisabled, Callers: cfg.AdminCallers,
		NodeID: gw.NodeID, Failures: m.AdminAuthFailures,
	}, signer, st, h, inv, svc, clk, cfg.TestMode, cfg.Logger)
	adm.WithSocial(svc)
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
	a.adminRoutes = adm.Routes()
	return a, nil
}

// Handler 是对外端口：/ws、/healthz、/readyz，以及演示环境的 /demo/login。
func (a *App) Handler() http.Handler { return a.handler }

// AdminHandler 是管理端口（IM_ADMIN_ADDR）上的 /admin/*，由 main 单独监听。
func (a *App) AdminHandler() http.Handler { return a.admin }

// AdminRoutes 返回已注册的管理路由及其 scope。
func (a *App) AdminRoutes() []admin.RouteInfo { return a.adminRoutes }

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

func (a *App) Close() error {
	close(a.stop)
	return a.store.Close()
}

// initSocialDefaults 把启动配置合并到默认值上，只在没有配置行时写入（之后一律以数据库为准，AC-SOC-01-9）。
func initSocialDefaults(st *store.SQLite, raw string, now int64) error {
	s, err := social.Patch(social.Defaults(), []byte(raw))
	if err != nil {
		return fmt.Errorf("IM_SOCIAL_DEFAULTS: %w", err)
	}
	b, _ := json.Marshal(s)
	_, err = st.InitSocialSettings(context.Background(), string(b), now)
	return err
}

// randomNodeID 在没有配置 IM_NODE_ID 时生成节点标识，例如 "n3f9a1c"。
func randomNodeID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return "n" + hex.EncodeToString(b)
}
