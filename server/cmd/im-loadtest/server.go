package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fixedAddrs 是 -metrics-addr、-ops-addr、-pprof-addr（PRD-ops S-OPS-25）：外部的 Prometheus 和堆快照按固定端口采集。
var fixedAddrs struct{ metrics, ops, pprof string }

// serverEnv 是额外传给服务端的环境变量（-server-env），例如 GOMEMLIMIT。
var serverEnv []string

// external 让压测打到已经在运行的节点，不启动子进程（AC-OPS-90-5：测试环境里的 n1）。
var external struct{ addr, adminAddr string }

// adminKey 在 external 模式下换成 -external-admin-key-file 的内容。
var adminKey = "loadtest-admin-key-0123456789abcdef"

const jwtSecret = "loadtest-jwt-secret-0123456789abcdef"

// server 是由压测工具启动的 im-server 子进程。
type server struct {
	bin, db, logPath string
	addr, metrics    string
	adminAddr        string // 管理端口（IM_ADMIN_ADDR），和对外端口分开
	pprofAddr        string // pprof（IM_PPROF_ADDR），和指标端口分开
	opsAddr          string // 运维接口（IM_OPS_ADDR）；为空时不配置
	cmd              *exec.Cmd
	log              *os.File
	env              []string // 这个场景额外的环境变量（如 IM_TEST_MODE=1、IM_SOCIAL_DEFAULTS）
}

func newServer(bin, dir, name string) *server {
	return &server{bin: bin, db: filepath.Join(dir, name+".db"), logPath: filepath.Join(dir, name+".log")}
}

// start 启动服务端（复用同一个数据库文件，可以用来做 kill -9 后重启）。
// 压测机和服务端在同一台机器上，所有连接都来自 127.0.0.1，所以调高按 IP 的握手限流。
func (s *server) start() error {
	if external.addr != "" {
		s.addr, s.adminAddr = external.addr, external.adminAddr
		resp, err := http.Get("http://" + s.addr + "/readyz")
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("external server is not ready: %d", resp.StatusCode)
		}
		return nil
	}
	s.addr, s.metrics, s.adminAddr, s.pprofAddr = freeAddr(), cmp.Or(fixedAddrs.metrics, freeAddr()), freeAddr(), cmp.Or(fixedAddrs.pprof, freeAddr())
	if fixedAddrs.ops != "" && s.opsAddr != "" {
		s.opsAddr = fixedAddrs.ops
		s.env = append(s.env, "IM_OPS_ADDR="+s.opsAddr) // 后面的值覆盖场景里随机的地址
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	s.log = f
	s.cmd = exec.Command(s.bin) // #nosec G204 -- 可执行文件路径来自运维传入的 -bin 参数
	s.cmd.Env = append(os.Environ(),
		"IM_ADDR="+s.addr, "IM_DB="+s.db, "IM_METRICS_ADDR="+s.metrics, "IM_ADMIN_ADDR="+s.adminAddr, "IM_PPROF_ADDR="+s.pprofAddr,
		"IM_ADMIN_KEY="+adminKey, "IM_JWT_SECRET="+jwtSecret,
		"IM_HANDSHAKE_RATE=100000", "IM_HANDSHAKE_BURST=100000",
	)
	s.cmd.Env = append(s.cmd.Env, serverEnv...)
	s.cmd.Env = append(s.cmd.Env, s.env...)
	s.cmd.Stdout, s.cmd.Stderr = f, f
	if err := s.cmd.Start(); err != nil {
		return err
	}
	for range 200 {
		if resp, err := http.Get("http://" + s.addr + "/readyz"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server did not become ready (log: %s)", s.logPath)
}

func (s *server) stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { s.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			s.cmd.Process.Kill()
			<-done
		}
	}
	if s.log != nil {
		s.log.Close()
	}
}

// kill9 立即强杀服务端，不给优雅退出的机会。
func (s *server) kill9() {
	s.cmd.Process.Kill() // Unix 上是 SIGKILL
	s.cmd.Wait()
	s.log.Close()
}

func (s *server) wsURL() string { return "ws://" + s.addr + "/ws" }

func (s *server) admin(method, path string, body any) ([]byte, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, "http://"+s.adminAddr+path, bytes.NewReader(b))
	req.Header.Set("X-Admin-Key", adminKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, buf.String())
	}
	return buf.Bytes(), nil
}

// importUsers 分批（每批 500）注册用户。
func (s *server) importUsers(ids []string) error {
	for i := 0; i < len(ids); i += 500 {
		batch := make([]map[string]string, 0, 500)
		for _, id := range ids[i:min(i+500, len(ids))] {
			batch = append(batch, map[string]string{"userId": id})
		}
		if _, err := s.admin(http.MethodPost, "/admin/users/import", map[string]any{"users": batch}); err != nil {
			return err
		}
	}
	return nil
}

// tokens 并发签发 token。
func (s *server) tokens(ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	var mu sync.Mutex
	var firstErr error
	work := make(chan string)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				b, err := s.admin(http.MethodPost, "/admin/users/token", map[string]any{"userId": id, "ttlSeconds": 86400})
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				var r struct{ Token string }
				json.Unmarshal(b, &r)
				out[id] = r.Token
				mu.Unlock()
			}
		}()
	}
	for _, id := range ids {
		work <- id
	}
	close(work)
	wg.Wait()
	return out, firstErr
}

func (s *server) createGroup(gid string, members []string) error {
	_, err := s.admin(http.MethodPost, "/admin/groups", map[string]any{"groupId": gid, "name": gid, "members": members})
	return err
}

// scrape 读取 /metrics 里的若干指标（不带标签的取值，带标签的按完整序列名）。
func (s *server) scrape() map[string]float64 {
	out := map[string]float64{}
	resp, err := http.Get("http://" + s.metrics + "/metrics")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, val, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if v, err := strconv.ParseFloat(val, 64); err == nil {
			out[name] = v
		}
	}
	return out
}

// liveMem 先让服务端做一次 GC（/debug/pprof/heap?gc=1），再读 Go 堆和栈的占用，得到"真正存活"的内存。
// 不用进程 RSS 判断泄漏：macOS 会压缩、回收进程内存，同一进程 30 秒内 RSS 能在 110MB 和 560MB 之间跳（Q5a 实测）；
// GC 前的堆读数也混着还没回收的垃圾。
func (s *server) liveMem() float64 {
	if resp, err := http.Get("http://" + s.pprofAddr + "/debug/pprof/heap?gc=1"); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	v := s.scrape()
	return v["go_memstats_heap_alloc_bytes"] + v["go_memstats_stack_inuse_bytes"]
}

// monitor 每隔 interval 采样服务端的内存、CPU、goroutine，记录峰值。
type monitor struct {
	mu                 sync.Mutex
	peakRSS, firstRSS  float64 // 进程 RSS（macOS 上波动很大，只作参考）
	lastRSS            float64
	peakHeap           float64 // Go 堆占用峰值（含未回收的垃圾，反映 GC 前的最高水位）
	peakGoroutines     float64
	cpuStart, cpuEnd   float64
	wallStart, wallEnd time.Time
	stop               context.CancelFunc
	done               chan struct{}
}

func startMonitor(s *server, interval time.Duration) *monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &monitor{stop: cancel, done: make(chan struct{})}
	sample := func() {
		v := s.scrape()
		m.mu.Lock()
		defer m.mu.Unlock()
		rss := v["process_resident_memory_bytes"]
		if m.firstRSS == 0 {
			m.firstRSS, m.cpuStart, m.wallStart = rss, v["process_cpu_seconds_total"], time.Now()
		}
		m.lastRSS, m.cpuEnd, m.wallEnd = rss, v["process_cpu_seconds_total"], time.Now()
		m.peakRSS = max(m.peakRSS, rss)
		m.peakHeap = max(m.peakHeap, v["go_memstats_heap_inuse_bytes"]+v["go_memstats_stack_inuse_bytes"])
		m.peakGoroutines = max(m.peakGoroutines, v["go_goroutines"])
	}
	go func() {
		defer close(m.done)
		sample()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				sample()
				return
			case <-t.C:
				sample()
			}
		}
	}()
	return m
}

func (m *monitor) finish() {
	m.stop()
	<-m.done
}

// avgCPU 返回采样期间服务端进程的平均 CPU 占用（1.0 = 一个核满载）。
func (m *monitor) avgCPU() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if secs := m.wallEnd.Sub(m.wallStart).Seconds(); secs > 0 {
		return (m.cpuEnd - m.cpuStart) / secs
	}
	return 0
}

func freeAddr() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().String()
}
