package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 系统推送的性能场景（PRD-live 8.1 节）：[P-开发机] 只做回归和趋势，不能作为发布结论；AC-PUSH-04-2 的发布结论在目标机（P-12）。
//
//	push-all   AC-PUSH-04-2  1 万连接 + 后台 600 条/秒单聊，每 30 秒全员推送一条，共 20 次
//	push-user  AC-PUSH-04-4  后台 600 条/秒单聊，按用户推送 1000 人 / 次、每秒 1 次，与不推送的基线交替，各 5 分钟
//	push-sync  AC-PUSH-04-5  收件箱 1000 条，pushSync {limit:100} 执行 1000 次

const pushLoadType = "loadtest"

// newPushCaller 写调用方文件（0600）并打开推送。密钥每次随机，只存在于这次压测的工作目录。
func newPushCaller(s *server, dir string, extraEnv ...string) (*adminClient, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	raw, err := json.Marshal([]map[string]any{{
		"id": "loadtest", "alg": "hmac-sha256", "secret": base64.StdEncoding.EncodeToString(secret),
		"scopes": []string{"push.user", "push.all"}, "pushTypes": []string{pushLoadType},
	}})
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "push-callers.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	s.env = append(s.env, "IM_ADMIN_CALLERS="+path, "IM_PUSH_ENABLED=1")
	s.env = append(s.env, extraEnv...)
	return &adminClient{secret: secret}, nil
}

// c2cLoad 是只有单聊的发送计划：50 对各 12 条/秒，scale 1.5 时合计 600 条/秒（AC 写的是"后台 600 条/秒单聊"）。
func c2cLoad() load {
	l := newLoad()
	l.groups = nil
	return l
}

const c2cScale = 1.5

// pushPost 调用 POST /admin/push，返回接口耗时。
func pushPost(c *adminClient, body map[string]any) (time.Duration, error) {
	start := time.Now()
	code, b, err := c.do(http.MethodPost, "/admin/push", "", body)
	d := time.Since(start)
	if err != nil {
		return d, err
	}
	if code != http.StatusOK {
		return d, fmt.Errorf("HTTP %d: %.200s", code, b)
	}
	return d, nil
}

// arrivals 按 pushId 记发送时刻，统计每条连接的到达耗时。
type arrivals struct {
	sent sync.Map
	lat  latencies
	n    atomic.Int64
}

func (a *arrivals) markSent(id string) { a.sent.Store(id, time.Now()) }

func (a *arrivals) attach(cs map[string]*client) {
	fn := func(raw json.RawMessage, at time.Time) {
		var d struct {
			PushID string `json:"pushId"`
		}
		if json.Unmarshal(raw, &d) != nil {
			return
		}
		if v, ok := a.sent.Load(d.PushID); ok {
			a.lat.add(at.Sub(v.(time.Time)))
		}
		a.n.Add(1)
	}
	for _, c := range cs {
		c.onPush.Store(&fn)
	}
}

func unexpectedCloses(groups ...map[string]*client) *counter {
	u := &counter{}
	for _, cs := range groups {
		for _, c := range cs {
			if reason := c.unexpectedClose(); reason != "" {
				u.inc(reason)
			}
		}
	}
	return u
}

// scenarioPushAll：AC-PUSH-04-2。
func scenarioPushAll(cfg config) result {
	r := result{Case: "TC-PUSH-04-02", Title: "全员推送送达（P-开发机）", Target: fmt.Sprintf("%d 连接在线，后台 600 条/秒单聊，每 %s 全员推送一条，共 %d 次：合并后到达 P99 ≤ 5 秒、全部送达；同窗口单聊 ack P99 ≤ 1 秒", cfg.conns, cfg.pushInterval, cfg.pushRounds)}
	s := newServer(cfg.bin, cfg.dir, "push-all")
	ac, err := newPushCaller(s, cfg.dir, "IM_PUSH_ALL_RATE=100000")
	if err != nil {
		r.note("调用方配置失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	ac.base = "http://" + s.adminAddr
	idle := ids("c", cfg.conns, 5)
	l := c2cLoad()
	tokens, err := setupUsers(s, slices.Concat(idle, l.users))
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	m := startMonitor(s, 5*time.Second)
	idleConns, errs := connectAll(s, idle, tokens, 200)
	defer closeAll(idleConns)
	active, errs2 := connectAll(s, l.users, tokens, 200)
	defer closeAll(active)
	errs = append(errs, errs2...)
	total := len(idleConns) + len(active)
	r.row("在线连接", "%d（建立失败 %d）", total, len(errs))

	arr := &arrivals{}
	arr.attach(idleConns)
	arr.attach(active)
	warm, cancelWarm := context.WithTimeout(context.Background(), 10*time.Second)
	l.run(warm, active, c2cScale, "warm", &loadStats{perConv: map[string]int{}})
	cancelWarm()

	st := &loadStats{perConv: map[string]int{}, win: newWindows(10 * time.Second)}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); l.run(ctx, active, c2cScale, "main", st) }()
	var api latencies
	var apiErr counter
	mainStart := time.Now()
	for i := range cfg.pushRounds {
		id := fmt.Sprintf("all-%03d", i)
		arr.markSent(id)
		d, err := pushPost(ac, map[string]any{"target": "all", "type": pushLoadType, "data": map[string]any{"round": i}, "pushId": id})
		if err != nil {
			apiErr.inc(err.Error())
		} else {
			api.add(d)
		}
		time.Sleep(time.Until(mainStart.Add(time.Duration(i+1) * cfg.pushInterval)))
	}
	cancel()
	wg.Wait()
	mainSecs := time.Since(mainStart).Seconds()
	time.Sleep(2 * time.Second)
	m.finish()

	want := int64(total * cfg.pushRounds)
	got := arr.n.Load()
	lat := arr.lat.summary()
	ack := st.ackLat.summary()
	unexpected := unexpectedCloses(idleConns, active)
	rates := st.win.rates()
	minRate := 0.0
	if len(rates) > 0 {
		minRate = slices.Min(rates)
	}
	r.row("到达耗时（合并所有连接）", "%s", lat)
	r.row("送达", "%d / %d", got, want)
	r.row("POST /admin/push", "%s，失败 %d（%s）", api.summary(), apiErr.total(), apiErr.String())
	r.row("同窗口单聊 ack", "%s，错误 %d（%s）", ack, st.errs.total(), st.errs.String())
	r.row("单聊吞吐", "平均 %.0f 条/秒，10 秒窗口最低 %.0f", float64(len(st.acks))/mainSecs, minRate)
	r.row("意外断开", "%d（%s）", unexpected.total(), unexpected.String())
	r.row("服务端", "CPU 平均 %.0f%%，内存峰值（Go 堆 + 栈）%s", m.avgCPU()*100, mb(m.peakHeap))
	r.Pass = len(errs) == 0 && lat.P99 <= 5*time.Second && got == want && ack.P99 <= time.Second && st.errs.total() == 0 && apiErr.total() == 0 && unexpected.total() == 0
	return r
}

// scenarioPushUser：AC-PUSH-04-4。关 / 开交替 6 段，同一轮内对照。
func scenarioPushUser(cfg config) result {
	r := result{Case: "TC-PUSH-04-04", Title: "按用户推送（P-开发机）", Target: fmt.Sprintf("后台 600 条/秒单聊，1000 人 / 次、每秒 1 次，开、关各 3 段、每段 %s 交替：接口 P99 ≤ 2 秒；单聊 ack P99 ≤ 200ms", cfg.pushSegment)}
	s := newServer(cfg.bin, cfg.dir, "push-user")
	ac, err := newPushCaller(s, cfg.dir)
	if err != nil {
		r.note("调用方配置失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	ac.base = "http://" + s.adminAddr
	targets := ids("t", 1000, 4)
	l := c2cLoad()
	tokens, err := setupUsers(s, slices.Concat(targets, l.users))
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	targetConns, errs := connectAll(s, targets, tokens, 200)
	defer closeAll(targetConns)
	active, errs2 := connectAll(s, l.users, tokens, 200)
	defer closeAll(active)
	errs = append(errs, errs2...)
	arr := &arrivals{}
	arr.attach(targetConns)

	off, on := &loadStats{perConv: map[string]int{}}, &loadStats{perConv: map[string]int{}}
	var api latencies
	var apiErr counter
	var pushed atomic.Int64
	warm, cancelWarm := context.WithTimeout(context.Background(), 10*time.Second)
	l.run(warm, active, c2cScale, "warm", &loadStats{perConv: map[string]int{}})
	cancelWarm()
	for seg := range 6 {
		st := off
		pushing := seg%2 == 1
		if pushing {
			st = on
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.pushSegment)
		var wg sync.WaitGroup
		if pushing {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pace(ctx, 1, func(i int) {
					id := fmt.Sprintf("u-%d-%d", seg, i)
					arr.markSent(id)
					pushed.Add(1)
					d, err := pushPost(ac, map[string]any{"target": "user", "userIds": targets, "type": pushLoadType, "data": map[string]any{"text": strings.Repeat("x", 200)}, "pushId": id})
					if err != nil {
						apiErr.inc(err.Error())
						return
					}
					api.add(d)
				})
			}()
		}
		l.run(ctx, active, c2cScale, fmt.Sprintf("seg%d", seg), st)
		cancel()
		wg.Wait()
	}
	time.Sleep(2 * time.Second)
	offS, onS, apiS := off.ackLat.summary(), on.ackLat.summary(), api.summary()
	growth := (float64(onS.P99) - float64(offS.P99)) / float64(offS.P99) * 100
	want := pushed.Load() * int64(len(targetConns))
	unexpected := unexpectedCloses(targetConns, active)
	r.row("POST /admin/push", "%s，失败 %d（%s）", apiS, apiErr.total(), apiErr.String())
	r.row("基线单聊 ack（不推送）", "%s，错误 %d", offS, off.errs.total())
	r.row("推送时单聊 ack", "%s，错误 %d", onS, on.errs.total())
	r.row("P99 增幅（只记录，不判定）", "%.1f%%", growth)
	r.row("实时帧到达（1000 个在线目标）", "%s，%d / %d", arr.lat.summary(), arr.n.Load(), want)
	r.row("意外断开", "%d（%s）", unexpected.total(), unexpected.String())
	r.Pass = len(errs) == 0 && apiS.N > 0 && apiS.P99 <= 2*time.Second && onS.P99 <= 200*time.Millisecond && apiErr.total() == 0 && off.errs.total() == 0 && on.errs.total() == 0 && arr.n.Load() == want && unexpected.total() == 0
	return r
}

// scenarioPushSync：AC-PUSH-04-5。data 用上限 4096 字节，是最坏情况。
func scenarioPushSync(cfg config) result {
	r := result{Case: "TC-PUSH-04-05", Title: "pushSync 100 条（P-开发机）", Target: "收件箱 1000 条（每条 data 4096 字节），pushSync {limit:100} 执行 1000 次：P99 ≤ 200ms"}
	s := newServer(cfg.bin, cfg.dir, "push-sync")
	ac, err := newPushCaller(s, cfg.dir, "IM_PUSH_SYNC_RATE=100000000", "IM_PUSH_USER_RATE=100000")
	if err != nil {
		r.note("调用方配置失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	ac.base = "http://" + s.adminAddr
	tokens, err := setupUsers(s, []string{"u1"})
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	data := json.RawMessage(`"` + strings.Repeat("x", 4094) + `"`)
	for i := range 1000 {
		if _, err := pushPost(ac, map[string]any{"target": "user", "userIds": []string{"u1"}, "type": pushLoadType, "data": data, "pushId": fmt.Sprintf("s-%d", i)}); err != nil {
			r.note("写入收件箱失败：%v", err)
			return r
		}
	}
	dctx, cancelDial := context.WithTimeout(context.Background(), 30*time.Second)
	c, err := dial(dctx, s.wsURL(), "u1", tokens["u1"])
	cancelDial()
	if err != nil {
		r.note("连接失败：%v", err)
		return r
	}
	defer c.close()
	var lat latencies
	bad := 0
	for i := range 1000 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		start := time.Now()
		f, err := c.call(ctx, "pushSync", map[string]any{"afterSeq": (i % 10) * 100, "limit": 100})
		cancel()
		var a struct {
			Pushes []json.RawMessage `json:"pushes"`
		}
		if err != nil || f.Error != nil || json.Unmarshal(f.Data, &a) != nil || len(a.Pushes) != 100 {
			bad++
			continue
		}
		lat.add(time.Since(start))
	}
	sum := lat.summary()
	r.row("pushSync", "%s，失败 %d", sum, bad)
	r.Pass = bad == 0 && sum.P99 <= 200*time.Millisecond
	return r
}
