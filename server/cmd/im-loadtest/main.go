// Command im-loadtest 是服务端压测工具（BE-050），对应 docs/testcases/TC-PERF.md。
//
// 工具自己启动 im-server 子进程（每个场景一个独立的数据库），这样才能做"压测中 kill -9 再重启"的场景。
// 压测客户端和服务端在同一台机器上，延迟在同一进程里计时（发送时刻写在消息正文里），没有跨机时钟误差。
//
//	go build -o /tmp/im-server ./cmd/im-server
//	go run ./cmd/im-loadtest -bin /tmp/im-server -scenarios latency,sync,group,capacity,kill9 -out ../docs/reports/perf.md
//
// 场景：
//
//	latency   TC-PERF-00-01  单聊端到端延迟（100 条）
//	sync      TC-PERF-00-04  sync 200 条耗时（会话预置 10 万条）
//	group     TC-PERF-00-07  200 人群扩散（P1）
//	capacity  TC-PERF-00-02/03/05  1 万连接在线 + 500～600 条/秒 + 保持
//	kill9     TC-PERF-00-06  压测中强杀，已 ack 的消息不丢
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	var cfg config
	var scenarios, out string
	flag.StringVar(&cfg.bin, "bin", "", "im-server 可执行文件（必填）")
	flag.StringVar(&cfg.dir, "dir", "", "工作目录：数据库和服务端日志（默认临时目录）")
	flag.StringVar(&scenarios, "scenarios", "latency,sync,group,capacity,kill9", "要跑的场景，逗号分隔")
	flag.IntVar(&cfg.conns, "conns", 10000, "capacity：空闲长连接数")
	flag.DurationVar(&cfg.throughputDuration, "throughput-duration", 5*time.Minute, "capacity：600 条/秒的加压时长")
	flag.DurationVar(&cfg.hold, "hold", 30*time.Minute, "capacity：长连接保持时长（从全部建立后算起，包含加压阶段）")
	flag.DurationVar(&cfg.groupDuration, "group-duration", 5*time.Minute, "group：发送时长")
	flag.DurationVar(&cfg.killAfter, "kill-after", time.Minute, "kill9：发送多久后强杀")
	flag.StringVar(&out, "out", "", "把报告（Markdown）写到这个文件")
	flag.Parse()
	if cfg.bin == "" {
		fmt.Fprintln(os.Stderr, "-bin is required")
		os.Exit(2)
	}
	if cfg.dir == "" {
		d, err := os.MkdirTemp("", "im-loadtest")
		if err != nil {
			panic(err)
		}
		cfg.dir = d
	}

	started := time.Now()
	var results []result
	for _, name := range strings.Split(scenarios, ",") {
		fmt.Fprintf(os.Stderr, "[%s] running %s ...\n", time.Now().Format("15:04:05"), name)
		switch strings.TrimSpace(name) {
		case "latency":
			results = append(results, scenarioLatency(cfg))
		case "sync":
			results = append(results, scenarioSync(cfg))
		case "group":
			results = append(results, scenarioGroup(cfg))
		case "capacity":
			results = append(results, scenarioCapacity(cfg)...)
		case "kill9":
			results = append(results, scenarioKill9(cfg))
		default:
			fmt.Fprintf(os.Stderr, "unknown scenario %q\n", name)
			os.Exit(2)
		}
		fmt.Fprint(os.Stderr, results[len(results)-1].markdown())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## 压测结果（%s，用时 %s）\n\n", started.Format("2006-01-02 15:04"), time.Since(started).Round(time.Second))
	fmt.Fprintf(&b, "环境：%s/%s，%d 核，Go %s；压测客户端与服务端在同一台机器；工作目录 `%s`（数据库和服务端日志）。\n\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), cfg.dir)
	b.WriteString("| 用例 | 结论 |\n|---|---|\n")
	failed := 0
	for _, r := range results {
		v := "✅ 通过"
		if !r.Pass {
			v = "❌ 未通过"
			failed++
		}
		fmt.Fprintf(&b, "| %s %s | %s |\n", r.Case, r.Title, v)
	}
	b.WriteString("\n")
	for _, r := range results {
		b.WriteString(r.markdown())
	}
	fmt.Print(b.String())
	if out != "" {
		if err := os.WriteFile(out, []byte(b.String()), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}
