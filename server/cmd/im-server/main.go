// Command im-server 启动 IM 服务端，并提供数据库运维子命令。
//
//	im-server                          启动服务（同 im-server serve）
//	im-server migrate up|status        执行 / 查看数据库迁移（生产环境默认不自动迁移）
//	im-server backup -o FILE           在线备份（服务运行中也可以执行）
//	im-server restore -i FILE [-force] 从备份恢复（必须先停止服务）
//	im-server version
//
// 配置全部走环境变量，见 internal/config：
//
//	IM_ENV                development（默认）| production。production 下不安全的配置拒绝启动
//	IM_ADDR               监听地址，默认 :8080
//	IM_DB                 SQLite 文件路径，默认 im.db
//	IM_AUTO_MIGRATE       启动时自动迁移；development 默认 1，production 默认 0
//	IM_JWT_SECRET         用户 token 签名密钥（production 至少 32 字节）
//	IM_JWT_KEYS           多把签名密钥 "k1:secret1,k2:secret2"（密钥轮换），配合 IM_JWT_ACTIVE_KID
//	IM_ADMIN_KEY          管理接口密钥（请求头 X-Admin-Key，production 至少 32 字节）
//	IM_ALLOWED_ORIGINS    浏览器 Origin 白名单，逗号分隔；默认只允许本机
//	IM_MAX_CONNS_PER_USER 每个用户的连接上限，默认 10
//	IM_IDLE_TIMEOUT       空闲超时（如 90s），默认 90s
//	IM_NODE_ID            节点标识（集群部署时设置），默认随机
//	IM_METRICS_ADDR       Prometheus 指标和 pprof 的监听地址（只给内网），默认 127.0.0.1:9090；off 关闭
//	IM_HANDSHAKE_RATE / IM_HANDSHAKE_BURST  每个 IP 的握手速率 / 突发上限，默认 20 / 50（压测时调高）
//	IM_TRUSTED_PROXIES    可信反向代理（逗号分隔的 IP 或 CIDR），只信任它们传来的 X-Forwarded-For
//	IM_DRAIN_DELAY        下线时先摘流量再踢连接的等待时间，默认 0（production 建议 5s）
//	GOMEMLIMIT            Go 运行时的软内存上限，建议设为容器内存的 80% 左右，且至少是存活内存的 1.5～2 倍
//	                      （每连接约 32 KB，见 docs/reports/mem-2026-11-03.md）；设得比存活数据还低会让 GC 持续运行、CPU 打满
//	IM_TEST_MODE          设为 1 开启测试模式（production 禁止）
//	IM_DEMO_LOGIN         设为 1 开启演示登录（production 禁止）
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/config"
	"tianting/server/internal/store"
)

// version 构建时用 -ldflags "-X main.version=x.y.z" 注入。
var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cmd, args := "serve", os.Args[1:]
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(logger)
	case "migrate":
		err = migrate(args)
	case "backup":
		err = backup(args)
	case "restore":
		err = restore(args)
	case "version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q (serve | migrate | backup | restore | version)", cmd)
	}
	if err != nil {
		logger.Error("command failed", "cmd", cmd, "err", err)
		os.Exit(1)
	}
}

func serve(logger *slog.Logger) error {
	cfg, warnings, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		logger.Warn(w)
	}
	cfg.App.Version = version
	cfg.App.Logger = logger
	drainDelay, err := durationEnv("IM_DRAIN_DELAY")
	if err != nil {
		return err
	}

	a, err := app.New(cfg.App)
	if err != nil {
		return err
	}
	defer a.Close()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// 不设置 ReadTimeout / WriteTimeout：它们是整条连接的截止时间，会把 WebSocket 长连接一起断掉。
		// 管理接口的请求体已经用 MaxBytesReader 限制大小，WebSocket 有自己的空闲超时和写超时。
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)

	// /metrics 单独监听内网地址（BE-051）。
	var metricsSrv *http.Server
	if cfg.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", a.MetricsHandler())
		// pprof 只挂在这个内网地址上（不用空白导入：那会注册到全局 DefaultServeMux，容易被意外暴露到对外端口）。
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
		metricsSrv = &http.Server{Addr: cfg.MetricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			logger.Info("metrics listening", "addr", cfg.MetricsAddr)
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- fmt.Errorf("metrics: %w", err)
			}
		}()
	}
	go func() {
		logger.Info("im-server listening", "addr", cfg.Addr, "env", cfg.Env, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	// 优雅下线（ARCH 2.4）：先摘流量，等负载均衡生效，再通知在线连接，最后关闭。
	logger.Info("shutting down", "drainDelay", drainDelay)
	a.StopAccepting()
	time.Sleep(drainDelay)
	logger.Info("kicked connections for shutdown", "count", a.KickAll())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if metricsSrv != nil {
		_ = metricsSrv.Shutdown(shutdownCtx)
	}
	return srv.Shutdown(shutdownCtx)
}

func migrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	db := fs.String("db", dbPath(), "database file")
	fs.Parse(args)
	ctx := context.Background()
	switch fs.Arg(0) {
	case "up":
		done, err := store.MigrateSQLite(ctx, *db)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s) %v to %s\n", len(done), done, *db)
	case "status":
		st, err := store.SQLiteStatus(ctx, *db)
		if err != nil {
			return err
		}
		fmt.Printf("%s: applied %v\n", *db, st.Applied)
		for _, m := range st.Pending {
			fmt.Printf("  pending %04d_%s\n", m.Version, m.Name)
		}
	default:
		return errors.New("usage: im-server migrate [-db FILE] up|status")
	}
	return nil
}

func backup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	db := fs.String("db", dbPath(), "database file")
	out := fs.String("o", "", "backup file to create (must not exist)")
	fs.Parse(args)
	if *out == "" {
		return errors.New("usage: im-server backup -o FILE [-db FILE]")
	}
	ctx := context.Background()
	if err := store.BackupSQLite(ctx, *db, *out); err != nil {
		return err
	}
	if _, err := store.VerifySQLiteFile(ctx, *out); err != nil {
		return fmt.Errorf("backup written but failed verification: %w", err)
	}
	fmt.Printf("backup of %s written to %s (verified)\n", *db, *out)
	return nil
}

func restore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	db := fs.String("db", dbPath(), "database file to replace")
	in := fs.String("i", "", "backup file")
	force := fs.Bool("force", false, "replace an existing database (stop the server first)")
	fs.Parse(args)
	if *in == "" {
		return errors.New("usage: im-server restore -i FILE [-db FILE] [-force]")
	}
	if err := store.RestoreSQLite(context.Background(), *in, *db, *force); err != nil {
		return err
	}
	fmt.Printf("restored %s from %s\n", *db, *in)
	return nil
}

func dbPath() string {
	if v := os.Getenv("IM_DB"); v != "" {
		return v
	}
	return "im.db"
}

func durationEnv(key string) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s: invalid duration %q", key, v)
	}
	return d, nil
}
