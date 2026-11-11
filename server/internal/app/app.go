// Package app 把各模块组装成一个可运行的服务，main 和 e2e 测试共用。
package app

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime"
	"sync/atomic"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/admin"
	"tianting/server/internal/adminauth"
	"tianting/server/internal/api"
	"tianting/server/internal/auth"
	"tianting/server/internal/cache"
	"tianting/server/internal/clientip"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/files"
	"tianting/server/internal/gateway"
	"tianting/server/internal/hub"
	"tianting/server/internal/logic"
	"tianting/server/internal/metrics"
	"tianting/server/internal/moderation"
	"tianting/server/internal/objstore"
	"tianting/server/internal/opsapi"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/scheduler"
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
	// V13AdminFeatures 对应 IM_V13_ADMIN_FEATURES（默认关闭，PRD-admin 11.1 节）：开启后才注册第二批管理接口、下发第二批事件。
	// sdk-ts 对应版本发布之后再打开；关闭时禁言不生效，但已有的封禁仍然生效（封禁是安全措施，不跟开关走）。
	V13AdminFeatures bool
	// Push 是系统推送（PRD-live v1.0 L1）：默认关闭；和 V13AdminFeatures 互相独立。
	Push PushConfig
	// TestDisableJobs（IM_TEST_DISABLE_JOBS，只在测试模式生效）：这些任务不再被定时触发，手动触发不受影响（PRD-live 8.2 节）。
	TestDisableJobs []string
	// TestRouterWrap 只给测试替身用（AC-PUSH-04-6：配置移除广播能力的构建）；只在测试模式生效。
	TestRouterWrap func(delivery.Router) delivery.Router
	// QuarantineKey / QuarantineDays：被删原文的加密隔离（IM_QUARANTINE_KEY、IM_QUARANTINE_DAYS，BE-224）。
	QuarantineKey  string
	QuarantineDays int
	// Account 是账号功能的配置（PRD-chat 第 4.1 节）。
	Account account.Config
	// Files 是 C2 图片和文件（PRD-chat 第 4.1 节）；Files.Enabled 关闭时 /v1/files/* 是 404，send 的 image、file 返回 4000。
	Files files.Config
	// ObjStore 是对象存储（第 11.3 节），只在 Files.Enabled 时使用。
	ObjStore objstore.Config
	// MsgActions 是 C4 撤回、引用、@（PRD-chat 第 4.1 节）：MsgActions.Enabled 关闭时 recall、带 quote / mentions 的 send 返回 4030 featureDisabled。
	// 开启要求 QuarantineKey（AC-RCL-01-10），不依赖 V13AdminFeatures（RC2-15）。
	MsgActions logic.MsgActionsConfig
	// ConvSettings 是 C5 会话置顶、免打扰、删除（PRD-chat 第 4.1 节）：ConvSettings.Enabled 关闭时 convSet、convDelete 返回 4030 featureDisabled，
	// 已有的设置照常生效（P-46 ②）。
	ConvSettings logic.ConvSettingsConfig
	// Moderation 是 C6 敏感词和举报（PRD-chat 第 4.1 节，BE-492）：Moderation.Enabled 关闭时不检查敏感词、report 返回 4030 featureDisabled，
	// 管理接口和清理任务照常（P-48 ④）。ClientCustomOff 是 IM_CLIENT_CUSTOM_MSG=0，不跟开关走。
	Moderation logic.ModConfig
	// ReportKey 是 IM_REPORT_KEY 解码后的 32 字节（举报证据的 AES-256-GCM 密钥）。开关打开时必须有（backend-chat 第 99 条）；
	// 关闭时有就照常初始化，开过又关掉之后已有的证据还能读（P-48 ④）。
	ReportKey []byte
	// RequireWords：生产 open 模式要求词库非空（F-ACC-01 规则 2，AC-ACC-01-9），迁移之后读库判断。由 config 包设置。
	RequireWords bool
	// TestWordsStartupFail：测试模式下启动加载词库失败（PRD-chat 第 4.5 节，AC-MOD-01-8 第 2 种；IM_TEST_WORDS_STARTUP_FAIL）。
	TestWordsStartupFail bool
	// RequireHTTPS：生产环境 /v1/* 只接受 HTTPS（PRD-chat 第 4.2 节第 7 条）。
	RequireHTTPS bool
	TestMode     bool // 开启故障注入、时钟注入、短有效期 token（PRD 6.1 节），生产必须关闭
	DemoLogin    bool // 开启演示登录 /demo/login（PRD F-AUTH-04），生产必须关闭
	SendRate     int  // 每个用户每秒可发消息数，0 表示默认值 20（PRD F-MSG-07）
	Version      string
	Logger       *slog.Logger
	// 节点运维接口（PRD-ops F-OPS-16）：OpsKeys 为空时不提供。ConfigSummary 生成脱敏的配置摘要（由 config 包提供）。
	OpsKeys         []opsapi.Key
	OpsAllowedCIDRs []netip.Prefix
	ConfigSummary   func(version, nodeID string) any
}

const defaultSendRate = 20

// PushConfig 是系统推送的开关和参数（都未校准，PRD-live F-PUSH-09）。0 表示默认值。
type PushConfig struct {
	Enabled    bool // IM_PUSH_ENABLED
	AllRate    int  // IM_PUSH_ALL_RATE：target=all，每调用方每分钟，默认 20
	UserRate   int  // IM_PUSH_USER_RATE：target=user，每调用方每分钟，默认 600
	SyncRate   int  // IM_PUSH_SYNC_RATE：pushSync，每用户每分钟，默认 120
	InboxMax   int  // IM_PUSH_INBOX_MAX：单用户收件箱上限，默认 1000
	PurgeBatch int  // IM_PUSH_PURGE_BATCH：push_purge 每批行数，默认 1000
}

const (
	defaultPushAllRate    = 20
	defaultPushUserRate   = 600
	defaultPushSyncRate   = 120
	defaultPushInboxMax   = 1000
	defaultPushPurgeBatch = 1000
)

// App 是组装好的服务。
type App struct {
	handler http.Handler
	admin   http.Handler
	// adminRoutes 是已注册的管理路由（测试遍历全部路由检查 scope）。
	adminRoutes   []admin.RouteInfo
	store         *store.SQLite
	hub           *hub.Hub
	metrics       *metrics.Metrics
	draining      atomic.Bool
	stop          chan struct{} // 关闭后台任务（定时任务）
	loopback      *cache.HTTPLoopback
	ops           *opsapi.Server
	opsDB         *sql.DB // 运维接口的只读连接，不和消息写入共用
	dynamicChecks func(context.Context) []DynamicCheck
	// 测试模式的故障注入（PRD-ops S-OPS-24）：生产环境没有设置它们的路由。
	opsDelay atomic.Int64
	pingFail atomic.Bool
	files    *files.Service     // IM_FILES_ENABLED 关闭时为 nil
	words    *moderation.Engine // IM_MOD_ENABLED 关闭时为 nil
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
	// 开关关闭时不校验，升级不会因此启动失败（PRD-live F-PUSH-09 规则 3）。错误信息只有调用方 ID。
	if cfg.Push.Enabled {
		if ids := adminauth.MissingPushTypes(cfg.AdminCallers); len(ids) > 0 {
			return nil, fmt.Errorf("IM_PUSH_ENABLED: callers with push.all must have a non-empty pushTypes: %v", ids)
		}
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
	var router delivery.Router = h
	if cfg.TestMode && cfg.TestRouterWrap != nil {
		router = cfg.TestRouterWrap(h)
	}
	svc := logic.New(st, router, clk, faults, ratelimit.New(sendRate, sendRate, clk), inv, m, cfg.Logger)
	if cfg.Push.Enabled {
		if err := svc.EnablePush(context.Background(), logic.PushConfig{
			AllRate:  cmp.Or(cfg.Push.AllRate, defaultPushAllRate),
			UserRate: cmp.Or(cfg.Push.UserRate, defaultPushUserRate),
			SyncRate: cmp.Or(cfg.Push.SyncRate, defaultPushSyncRate),
			InboxMax: cmp.Or(cfg.Push.InboxMax, defaultPushInboxMax),
		}); err != nil {
			st.Close()
			return nil, err
		}
	}
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

	// 隔离加密器：第二批的后台删除和 C4 的撤回都用它，二者之一开启就初始化（第 4.1 节 RC2-15）。
	var cipher *quarantine.Cipher
	if cfg.V13AdminFeatures || cfg.MsgActions.Enabled {
		if cipher, err = quarantine.New(cfg.QuarantineKey); err != nil {
			st.Close()
			return nil, fmt.Errorf("IM_QUARANTINE_KEY: %w", err)
		}
	}
	if cfg.V13AdminFeatures {
		svc.EnableAdminFeatures()
	}
	if err := svc.ConfigureMsgActions(cfg.MsgActions, cipher, cmp.Or(cfg.QuarantineDays, defaultQuarantineDays)); err != nil {
		st.Close()
		return nil, err
	}
	svc.ConfigureConvSettings(cfg.ConvSettings)
	evidence, err := configureModeration(st, cfg)
	if err != nil {
		st.Close()
		return nil, err
	}
	words, err := startWords(st, cfg, clk, faults, inv, m)
	if err != nil {
		st.Close()
		return nil, err
	}
	modCfg := cfg.Moderation
	modCfg.C2CBodyAccess = cfg.C2CBodyAccess
	svc.ConfigureModeration(modCfg, evidence, words)
	var fileSvc *files.Service
	var obj *objstore.Client
	if cfg.Files.Enabled {
		var hooks *files.Hooks
		if cfg.TestMode {
			hooks = files.NewHooks() // complete 的暂停点只在测试模式（第 4.5 节）
		}
		fcfg := cfg.Files
		fcfg.RegisterOpen = cfg.Account.WithDefaults().RegisterMode == "open"
		ocfg := cfg.ObjStore
		ocfg.Observe = func(op string, err error) {
			m.ObjStoreRequests.WithLabelValues(op).Inc()
			if objstore.IsError(err) {
				m.ObjStoreErrors.WithLabelValues(op).Inc()
			}
		}
		obj = objstore.New(ocfg)
		if fileSvc, err = files.New(files.Deps{Config: fcfg, Store: st, Obj: obj, Clock: clk, Hooks: hooks, Log: cfg.Logger, Metrics: m}); err != nil {
			st.Close()
			return nil, err
		}
		svc.EnableFiles(*fileSvc.Config().RetentionDays)
	}
	var avatars account.AvatarBinder
	if fileSvc != nil {
		avatars = fileSvc
	}
	var accountFaults *account.Faults
	if cfg.TestMode {
		accountFaults = account.NewFaults()
	} else {
		cfg.Account.TestDisableLock = false // 只在测试模式生效（第 4.5 节）
	}
	accounts, err := account.New(account.Deps{
		Config: cfg.Account, Store: st, Signer: signer, Router: router, Clock: clk, Faults: accountFaults, Log: cfg.Logger,
		Logins: m.AccountLogins, Registers: m.Registers, InviteAutoDisabled: m.InviteAutoDisabled,
		RefreshReissue: m.RefreshReissue, RefreshReplay: m.RefreshReplay, Cleaner: svc, Avatars: avatars, Content: svc,
	})
	if err != nil {
		st.Close()
		return nil, err
	}
	// refresh 密钥台账（RS5-04）：先查后写，配置里删掉的行写入时不会被改动。
	ledger, err := accounts.LedgerProblems(context.Background())
	if err != nil {
		st.Close()
		return nil, err
	}
	for _, p := range ledger {
		cfg.Logger.Warn("refresh key ledger", "problem", p)
	}
	if len(ledger) > 0 && cfg.Account.EnforceKeyLedger {
		st.Close()
		return nil, fmt.Errorf("refresh key ledger: %v (key compromised? see the runbook: im-server refresh-keys forget)", ledger)
	}
	if err := accounts.SyncLedger(context.Background()); err != nil {
		st.Close()
		return nil, err
	}
	a := &App{store: st, hub: h, metrics: m, stop: make(chan struct{}), loopback: loopback, files: fileSvc, words: words}
	a.dynamicChecks = func(ctx context.Context) []DynamicCheck {
		problems, err := accounts.LedgerProblems(ctx)
		if err != nil {
			problems = append(problems, "cannot read the refresh key ledger")
		}
		detail := "refresh 密钥台账：配置里没有的密钥都已退役满 IM_REFRESH_ABSOLUTE"
		if len(problems) > 0 {
			detail = fmt.Sprintf("refresh 密钥台账：%v", problems)
		}
		checks := []DynamicCheck{{ID: "refresh_key_ledger", Passed: len(problems) == 0, Detail: detail}}
		// 公开传播一个大额码等于开放注册（F-ACC-09 规则 1，RC2-18）：只告警。
		warnTotal := int64(cmp.Or(cfg.Account.InviteWarnTotal, defaultInviteWarnTotal))
		total, err := st.InviteRemainingTotal(ctx, clk.Now().UnixMilli())
		if err != nil {
			checks = append(checks, DynamicCheck{ID: "invite_total", Detail: "邀请码总量：查询失败"})
		} else {
			checks = append(checks, DynamicCheck{ID: "invite_total", Passed: total <= warnTotal, Detail: fmt.Sprintf("有效邀请码剩余 %d 次，告警线 %d（IM_INVITE_WARN_TOTAL）", total, warnTotal)})
		}
		return append(checks, objstoreCheck(ctx, obj), wordsCheck(ctx, st, cfg.Moderation.Enabled))
	}
	jobs := scheduler.New(clk, nil, m.JobRuns, m.JobDuration, cfg.Logger)
	registerJobs(jobs, st, svc, clk, cmp.Or(cfg.Push.PurgeBatch, defaultPushPurgeBatch))
	jobs.Register("account_events_purge", jobPeriod, accounts.PurgeEvents)
	jobs.Register("auth_throttle_purge", jobPeriod, accounts.PurgeThrottle)
	jobs.Register("session_purge", 24*time.Hour, accounts.PurgeSessions)
	jobs.Register("account_cleanup", jobPeriod, accounts.Cleanup)
	if fileSvc != nil {
		// 每小时一批（F-FILE-06 规则 7）。和其他定时任务一样只在一个节点运行（集群随 B6）。
		jobs.Register("file_purge", jobPeriod, fileSvc.Purge)
	}
	jobs.Register("invite_purge", 24*time.Hour, func(ctx context.Context) (int, error) {
		return st.PurgeInvites(ctx, clk.Now().Add(-invitePurgeAfter).UnixMilli())
	})
	if cfg.TestMode {
		for _, name := range cfg.TestDisableJobs {
			if err := jobs.Disable(name); err != nil {
				st.Close()
				return nil, fmt.Errorf("IM_TEST_DISABLE_JOBS %s: %w", name, err)
			}
		}
	}
	go jobs.Start(a.stop)
	info := api.Info{Version: cfg.Version, StartedAt: time.Now(), Ready: a.ready}
	if cfg.ReportLegacyAdmin {
		info.Details = map[string]any{"legacyAdminKey": true}
	}
	mux := api.NewRouter(info)
	gw := cfg.Gateway
	if gw.NodeID == "" {
		gw.NodeID = randomNodeID()
	}
	access := accounts.Access()
	gws := gateway.New(gw, signer, access, clk, h, svc, m, cfg.Logger)
	if f, ok := accounts.ConnFaults(); ok {
		gws.WithFaults(f)
	}
	go gws.StartRecheck(a.stop)
	mux.Handle("GET /ws", gws)
	// /v1/*（PRD-chat 第 4.2 节）：只在对外端口。按 IP 的限制和 /ws 共用可信代理配置（R-CS-35）。
	v1 := &api.V1{RequireHTTPS: cfg.RequireHTTPS, Proxies: clientip.Resolver{TrustedProxies: gw.Guard.TrustedProxies}, Verifier: signer, Checker: access, Log: cfg.Logger}
	publicConfig := cfg.Account.PublicConfig(cfg.DemoLogin)
	if fileSvc != nil {
		l := fileSvc.Config().Limits()
		publicConfig.Features.Files = true
		publicConfig.Limits.ImageMaxBytes, publicConfig.Limits.ThumbMaxBytes, publicConfig.Limits.FileMaxBytes = l.ImageMaxBytes, l.ThumbMaxBytes, l.FileMaxBytes
		publicConfig.Limits.AvatarMaxBytes, publicConfig.Limits.UploadDailyBytes = l.AvatarMaxBytes, l.UploadDailyBytes
	}
	publicConfig.Features.MsgActions = cfg.MsgActions.Enabled     // 客户端按它隐藏撤回、引用、@ 的入口（第 4.1 节共同规则）
	publicConfig.Features.ConvSettings = cfg.ConvSettings.Enabled // 客户端按它隐藏置顶、免打扰、删除的入口（backend-chat 第 75 条）
	publicConfig.Features.Moderation = cfg.Moderation.Enabled     // 只控制举报入口（backend-chat 第 96 条）
	v1.Handle(mux, "GET /v1/config", func(w http.ResponseWriter, r *http.Request) { api.WriteJSON(w, publicConfig) })
	v1.RegisterAccount(mux, accounts)
	if fileSvc != nil {
		v1.RegisterFiles(mux, fileSvc)
	}
	// im_reports_pending（F-MOD-04 规则 7）：抓取时现算超过 24 小时未处理的数量。不跟 IM_MOD_ENABLED：关掉之后已有的举报也要处理完（P-48 ④）。
	m.RegisterReportsPending(func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		n, err := st.CountPendingReportsBefore(ctx, clk.Now().Add(-24*time.Hour).UnixMilli())
		if err != nil {
			return -1 // 查询失败不能当成 0（没有积压）
		}
		return float64(n)
	})
	m.RegisterNode(cfg.Version, runtime.Version(), gw.NodeID, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return a.ready(ctx) == nil
	}, a.draining.Load)
	if len(cfg.OpsKeys) > 0 {
		if a.opsDB, err = store.OpenSQLiteReadOnly(cfg.DBPath); err != nil {
			st.Close()
			return nil, err
		}
		a.ops = opsapi.New(opsapi.Deps{
			NodeID: gw.NodeID, Version: cfg.Version, StartedAt: time.Now(), Keys: cfg.OpsKeys, AllowedCIDRs: cfg.OpsAllowedCIDRs,
			Clock: clk, Metrics: m, Log: cfg.Logger.With("log", "ops_access"), Ready: a.ready, Draining: a.draining.Load,
			Platforms: h.PlatformCounts, Sessions: h, Disconnects: gws.Disconnects(), TestDelay: testDelay(cfg.TestMode, &a.opsDelay),
			Config: func() any {
				if cfg.ConfigSummary == nil {
					return struct{}{}
				}
				out := cfg.ConfigSummary(cfg.Version, gw.NodeID)
				// 配置摘要由 config 包生成（config 依赖 app，反过来不行）：查库的检查项经这个接口并进去。
				if w, ok := out.(interface {
					WithDynamicCheck(id, detail string, passed bool) any
				}); ok {
					for _, c := range a.DynamicChecks(context.Background()) {
						out = w.WithDynamicCheck(c.ID, c.Detail, c.Passed)
						w, _ = out.(interface {
							WithDynamicCheck(id, detail string, passed bool) any
						})
					}
				}
				return out
			},
			DB: func(ctx context.Context) (opsapi.DB, error) { return a.dbStatus(ctx, cfg.DBPath) },
		})
	}
	// 管理接口只注册在单独的管理 mux 上（PRD-admin F-KEY-01）：对外端口上请求 /admin/* 是 404。
	adm := admin.New(admin.AuthConfig{
		LegacyKey: cfg.AdminKey, LegacyDisabled: cfg.AdminLegacyDisabled, Callers: cfg.AdminCallers,
		NodeID: gw.NodeID, Failures: m.AdminAuthFailures,
	}, signer, st, h, inv, svc, clk, cfg.TestMode, cfg.Logger)
	adm.WithSocial(svc)
	adm.WithContent(svc)
	adm.WithSessions(h)
	adm.WithTestFaults(a)
	if cfg.TestMode {
		adm.WithAccountTest(accounts, h.SetDropKicks)
	}
	adm.WithJobs(jobs)
	if cfg.V13AdminFeatures {
		mc := admin.ModerationConfig{Service: svc, Cipher: cipher, QuarantineDays: cmp.Or(cfg.QuarantineDays, defaultQuarantineDays), C2CBodyAccess: cfg.C2CBodyAccess}
		if obj != nil {
			mc.Objects = obj // POST /admin/files/{fileId}/fetch（C4）
		}
		adm.WithModeration(mc)
	}
	// C6 举报处理（BE-496）：不跟开关（P-48 ④）。IM_REPORT_KEY 没配置时读证据按"无法解密"。
	rc := admin.ReportConfig{Cipher: evidence, C2CBodyAccess: cfg.C2CBodyAccess, Undecryptable: m.ReportUndecryptable}
	if obj != nil {
		rc.Objects = obj
	}
	adm.WithReports(rc)
	if cfg.Push.Enabled {
		adm.WithPush(svc)
	}
	adminMux := http.NewServeMux()
	adm.Register(adminMux)
	adm.RegisterAccounts(adminMux)
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

// 定时任务的周期和保留时间。
const (
	jobPeriod              = time.Hour
	auditRetention         = 365 * 24 * time.Hour // PRD-admin F-KEY-04：保留 1 年
	grantRetention         = 48 * time.Hour       // 授权只有 15 分钟、配额只看当天；审计另有 admin_audit
	defaultQuarantineDays  = 30
	invitePurgeAfter       = 90 * 24 * time.Hour // 过期或用完 90 天后清除（F-ACC-09 规则 3，未校准）
	defaultInviteWarnTotal = 500
)

// registerJobs 注册全部定时任务（BE-220）。任务名是测试接口和指标标签里的稳定名字。
func registerJobs(s *scheduler.Scheduler, st *store.SQLite, svc *logic.Service, clk clock.Clock, pushPurgeBatch int) {
	// 不跟 IM_PUSH_ENABLED：开关关掉之后，已有的推送也要回收。定时触发时跑到清完，每批之间释放数据库连接（F-PUSH-09 规则 4）。
	s.RegisterDrain("push_purge", jobPeriod, func(ctx context.Context) (int, error) { return svc.PurgePushBatch(ctx, pushPurgeBatch) })
	s.Register("social_purge", jobPeriod, svc.PurgeSocialNow)
	s.Register("group_req_purge", jobPeriod, svc.PurgeGroupReqs)
	s.Register("admin_audit_purge", jobPeriod, func(ctx context.Context) (int, error) {
		return st.PurgeAdminAuditBefore(ctx, clk.Now().Add(-auditRetention).UnixMilli())
	})
	// 不跟 IM_V13_ADMIN_FEATURES：开关关掉之后，已经隔离的原文也要按时物理删除。
	s.Register("quarantine_purge", jobPeriod, func(ctx context.Context) (int, error) {
		return st.PurgeQuarantine(ctx, clk.Now().UnixMilli())
	})
	s.Register("msg_grants_purge", jobPeriod, func(ctx context.Context) (int, error) {
		return st.PurgeMsgGrants(ctx, clk.Now().Add(-grantRetention).UnixMilli())
	})
	// 提及计数清理到已读之后（PRD-chat 第 12.1 节，C4）。不跟开关：关掉之后表里的行也要清。
	s.Register("mention_purge", jobPeriod, svc.PurgeMentions)
	// C6 命中记录保留 IM_MOD_HITS_DAYS（F-MOD-02 规则 7）。不跟 IM_MOD_ENABLED（P-48 ④：保留期是对用户的承诺）。
	s.Register("moderation_hits_purge", jobPeriod, svc.PurgeHits)
	// C6 举报证据和举报行保留 IM_REPORT_EVIDENCE_DAYS（F-MOD-03 规则 6，P-48 ②）。不跟开关。
	s.Register("report_purge", jobPeriod, svc.PurgeReports)
}

// configureModeration 是 C6 的启动检查（BE-492 第 3 条）：生产 open 模式要求词库非空（AC-ACC-01-9，迁移之后才能读库）；
// IM_REPORT_KEY 有就初始化证据加密器，开关打开时必须有（backend-chat 第 99 条，config 包另有同样的检查，这里防止绕过 config 直接组装）。
func configureModeration(st *store.SQLite, cfg Config) (*quarantine.Cipher, error) {
	if cfg.RequireWords {
		n, err := st.CountActiveWords(context.Background())
		if err != nil {
			return nil, fmt.Errorf("count blocked words: %w", err)
		}
		if n == 0 {
			return nil, errors.New("IM_REGISTER_MODE=open is refused in production: the blocked word list is empty (F-ACC-01 rule 2); import words through the admin API in invite mode first")
		}
	}
	if len(cfg.ReportKey) == 0 {
		if cfg.Moderation.Enabled {
			return nil, errors.New("IM_MOD_ENABLED=1 requires IM_REPORT_KEY (report evidence is encrypted; no development default)")
		}
		return nil, nil
	}
	if len(cfg.ReportKey) != ReportKeyLen {
		return nil, fmt.Errorf("IM_REPORT_KEY must decode to %d bytes", ReportKeyLen)
	}
	return quarantine.New(string(cfg.ReportKey))
}

// startWords 在 IM_MOD_ENABLED=1 时启动词库引擎（BE-493）：同步加载一次，失败启动失败（F-MOD-01 异常，AC-MOD-01-8 第 2 种）；
// 订阅失效主题 blockedWords（规则 2），TTL 60 秒兜底在引擎里。开关关闭时不加载、不建自动机，管理接口照常（BE-493 第 7 条）。
func startWords(st *store.SQLite, cfg Config, clk clock.Clock, faults *logic.Faults, inv cache.Invalidator, m *metrics.Metrics) (*moderation.Engine, error) {
	if !cfg.Moderation.Enabled {
		return nil, nil
	}
	e := moderation.New(moderation.Deps{
		Load: func(ctx context.Context) ([]moderation.Word, error) {
			keys, err := st.ActiveWords(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]moderation.Word, len(keys))
			for i, k := range keys {
				out[i] = moderation.Word{ID: k.WordID, Norm: k.Norm, Compact: k.Compact}
			}
			return out, nil
		},
		Now:         clk.Now,
		ReloadFails: faults.WordsReloadFails, // 测试模式之外 faults 为 nil，永远不失败
		ReloadError: m.ModReloadErrors.Inc,
		Log:         cfg.Logger,
	})
	if err := e.Start(context.Background(), cfg.TestMode && cfg.TestWordsStartupFail); err != nil {
		return nil, fmt.Errorf("IM_MOD_ENABLED=1: loading the blocked word list failed: %w", err)
	}
	inv.Subscribe(moderation.TopicBlockedWords, func(string) { e.Invalidate() })
	return e, nil
}

// ReportKeyLen 是 IM_REPORT_KEY 解码后的长度（32 字节随机值，base64，第 4.1 节）。
const ReportKeyLen = 32

// DynamicCheck 是要查库的配置检查项（F-ACC-03 规则 3 的密钥台账、F-ACC-09 规则 1 的邀请码总量）。
// 启动保护里的检查项都是静态的（config.productionChecks），这些在启动后还会变，每次查询现算。
type DynamicCheck struct {
	ID     string
	Passed bool
	Detail string
}

// objstoreCheck 是配置检查的"对象存储连通"（PRD-chat 第 14.2 节）：按最终前缀列举 1 个键，最多等 5 秒。
// 只是检查项，不是 /readyz 的条件（第 15 节"可用性"：对象存储不可用不影响文本消息）。Detail 不含地址和凭证。
func objstoreCheck(ctx context.Context, obj *objstore.Client) DynamicCheck {
	if obj == nil {
		return DynamicCheck{ID: "objstore_reachable", Passed: true, Detail: "IM_FILES_ENABLED 关闭，不检查对象存储"}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := obj.Probe(ctx); err != nil {
		detail := "对象存储：不可达（" + err.Error() + "）"
		var se *objstore.Error
		if errors.As(err, &se) && se.Status/100 == 4 {
			detail = fmt.Sprintf("对象存储：可以连通，但按最终前缀列举返回 HTTP %d（凭证或权限，见 deploy/FILES-RUNBOOK.md 第 2 节）", se.Status)
		}
		return DynamicCheck{ID: "objstore_reachable", Detail: detail}
	}
	return DynamicCheck{ID: "objstore_reachable", Passed: true, Detail: "对象存储：可以连通，服务端凭证能按最终前缀列举"}
}

// wordsCheck 是配置检查的"词库是否为空"（F-MOD-01 规则 6，第 14.2 节）：为空时不拦截，只告警，不拒绝启动（open 模式另见 configureModeration）。
// Detail 只有数量，不含任何词。
func wordsCheck(ctx context.Context, st *store.SQLite, enabled bool) DynamicCheck {
	n, err := st.CountActiveWords(ctx)
	if err != nil {
		return DynamicCheck{ID: "moderation_words", Detail: "词库：查询失败"}
	}
	if n == 0 {
		return DynamicCheck{ID: "moderation_words", Detail: fmt.Sprintf("告警：词库为空，发送不拦截（IM_MOD_ENABLED：%s）", onOff(enabled))}
	}
	return DynamicCheck{ID: "moderation_words", Passed: true, Detail: fmt.Sprintf("词库：%d 个启用的词（IM_MOD_ENABLED：%s）", n, onOff(enabled))}
}

func onOff(on bool) string {
	if on {
		return "开启"
	}
	return "关闭"
}

// DynamicChecks 算一遍。查询失败算没通过，不当成通过。
func (a *App) DynamicChecks(ctx context.Context) []DynamicCheck {
	if a.dynamicChecks == nil {
		return nil
	}
	return a.dynamicChecks(ctx)
}

// Files 返回文件服务（测试用：暂停点等故障注入在 Files().Hooks()）；IM_FILES_ENABLED 关闭时为 nil。
func (a *App) Files() *files.Service { return a.files }

// Handler 是对外端口：/ws、/healthz、/readyz，以及演示环境的 /demo/login。
func (a *App) Handler() http.Handler { return a.handler }

// AdminHandler 是管理端口（IM_ADMIN_ADDR）上的 /admin/*，由 main 单独监听。
func (a *App) AdminHandler() http.Handler { return a.admin }

// AdminRoutes 返回已注册的管理路由及其 scope。
func (a *App) AdminRoutes() []admin.RouteInfo { return a.adminRoutes }

// MetricsHandler 是 /metrics。由 main 挂在单独的内网地址上（IM_METRICS_ADDR），不和对外端口共用。
func (a *App) MetricsHandler() http.Handler { return a.metrics.Handler() }

// OpsHandler 是运维端口（IM_OPS_ADDR）上的 /internal/v1/*；没有配置运维公钥时为 nil。
func (a *App) OpsHandler() http.Handler {
	if a.ops == nil {
		return nil
	}
	return a.ops.Handler()
}

// OpsRoutes 返回运维接口的路由（只读路由守护测试用）。
func (a *App) OpsRoutes() []string {
	if a.ops == nil {
		return nil
	}
	return a.ops.Routes()
}

// SetOpsDelay、SetPingFail 是测试模式的故障注入（admin.TestFaults）。
func (a *App) SetOpsDelay(ms int64)  { a.opsDelay.Store(ms) }
func (a *App) SetPingFail(fail bool) { a.pingFail.Store(fail) }

func testDelay(testMode bool, v *atomic.Int64) *atomic.Int64 {
	if testMode {
		return v
	}
	return nil
}

func (a *App) dbStatus(ctx context.Context, path string) (opsapi.DB, error) {
	ms, err := store.SQLiteMigrations()
	if err != nil {
		return opsapi.DB{}, err
	}
	info := opsapi.DB{Type: "sqlite", Location: path, Connected: !a.pingFail.Load() && a.opsDB.PingContext(ctx) == nil, Migrations: []store.MigrationRow{}, Pending: []store.MigrationRow{}}
	if !info.Connected {
		return info, nil
	}
	rows, err := store.MigrationReport(ctx, a.opsDB, ms)
	if err != nil {
		info.Connected = false
		return info, nil
	}
	var done, pending []store.MigrationRow
	for _, r := range rows {
		if r.Pending {
			pending = append(pending, r)
			continue
		}
		done = append(done, r)
		info.Version = max(info.Version, r.Version)
	}
	if done != nil {
		info.Migrations = done
	}
	if pending != nil {
		info.Pending = pending
	}
	return info, nil
}

func (a *App) ready(ctx context.Context) error {
	if a.pingFail.Load() {
		return errors.New("injected ping failure")
	}
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
	if a.opsDB != nil {
		a.opsDB.Close()
	}
	close(a.stop)
	if a.words != nil {
		a.words.Close()
	}
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
