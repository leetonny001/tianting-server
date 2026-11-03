package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

type config struct {
	bin, dir           string
	conns              int
	throughputDuration time.Duration
	groupDuration      time.Duration
	hold               time.Duration
	killAfter          time.Duration
}

func ids(prefix string, n, width int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%0*d", prefix, width, i+1)
	}
	return out
}

// connectAll 以 perSecond 的速率并发建立连接。
func connectAll(s *server, users []string, tokens map[string]string, perSecond int) (map[string]*client, []error) {
	out := make(map[string]*client, len(users))
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	tick := time.NewTicker(time.Second / time.Duration(perSecond))
	defer tick.Stop()
	for _, uid := range users {
		<-tick.C
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			c, err := dial(ctx, s.wsURL(), uid, tokens[uid])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", uid, err))
				return
			}
			out[uid] = c
		}()
	}
	wg.Wait()
	return out, errs
}

func setupUsers(s *server, users []string) (map[string]string, error) {
	if err := s.importUsers(users); err != nil {
		return nil, err
	}
	return s.tokens(users)
}

func closeAll(cs map[string]*client) {
	for _, c := range cs {
		c.close()
	}
}

// ---------------------------------------------------------------------------
// TC-PERF-00-01：双方在线端到端延迟（连续 100 条，间隔 60ms）。

func scenarioLatency(cfg config) result {
	r := result{Case: "TC-PERF-00-01", Title: "单聊端到端延迟", Target: "100 条全部送达、seq 1～100 无缺无重，P99 ≤ 1000ms"}
	s := newServer(cfg.bin, cfg.dir, "latency")
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	tokens, err := setupUsers(s, []string{"alice", "bob"})
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	cs, errs := connectAll(s, []string{"alice", "bob"}, tokens, 100)
	if len(errs) > 0 {
		r.note("连接失败：%v", errs[0])
		return r
	}
	defer closeAll(cs)

	var lat latencies
	var mu sync.Mutex
	seen := map[int64]int{}
	fn := func(m protocol.Message, at time.Time) {
		if d, ok := latencyOf(m.Body.Text, at); ok {
			lat.add(d)
		}
		mu.Lock()
		seen[m.Seq]++
		mu.Unlock()
	}
	cs["bob"].onMsg.Store(&fn)
	for i := range 100 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, _, err := cs["alice"].sendText(ctx, protocol.ConvC2C, "bob", fmt.Sprintf("lat-%04d", i), 0); err != nil {
			r.note("第 %d 条发送失败：%v", i+1, err)
		}
		cancel()
		time.Sleep(60 * time.Millisecond)
	}
	time.Sleep(time.Second)
	mu.Lock()
	missing, dups := 0, 0
	for seq := int64(1); seq <= 100; seq++ {
		switch seen[seq] {
		case 0:
			missing++
		case 1:
		default:
			dups++
		}
	}
	mu.Unlock()
	sum := lat.summary()
	r.row("收到", "%d / 100（缺 %d，重复 %d）", sum.N, missing, dups)
	r.row("端到端延迟", "P50 %s，P99 %s，最大 %s", ms(sum.P50), ms(sum.P99), ms(sum.Max))
	r.Pass = sum.N == 100 && missing == 0 && dups == 0 && sum.P99 <= time.Second
	return r
}

// ---------------------------------------------------------------------------
// TC-PERF-00-04：sync 返回 200 条的耗时。会话预置 10 万条（每条约 100 字），直接写库：
// 每人每秒只能发 20 条，通过接口灌 10 万条要 40 多分钟。

func seed(dbPath, conv string, users []string, n, textLen int) (time.Duration, error) {
	st, err := store.OpenSQLite(dbPath, store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		return 0, err
	}
	defer st.Close()
	text := strings.Repeat("字", textLen)
	start := time.Now()
	for i := range n {
		from := users[i%len(users)]
		_, _, err := st.SaveMessage(context.Background(), store.Message{
			MsgID: fmt.Sprintf("seed-%s-%07d", conv, i), ClientMsgID: fmt.Sprintf("seed-%07d-%s", i, conv),
			ConvID: conv, From: from, Type: "text", Body: []byte(`{"text":"` + text + `"}`),
			TS: time.Now().UnixMilli(), Summary: protocol.Summary(text), ConvUsers: users,
		})
		if err != nil {
			return 0, err
		}
	}
	return time.Since(start), nil
}

func scenarioSync(cfg config) result {
	r := result{Case: "TC-PERF-00-04", Title: "sync 200 条耗时", Target: "会话 10 万条、每条约 100 字；afterSeq / beforeSeq 各随机 100 次，P99 ≤ 200ms"}
	s := newServer(cfg.bin, cfg.dir, "sync")
	took, err := seed(s.db, "c2c:alice:bob", []string{"alice", "bob"}, 100_000, 100)
	if err != nil {
		r.note("预置数据失败：%v", err)
		return r
	}
	r.note("预置 10 万条用时 %s（每条一个事务、synchronous=FULL）", took.Round(time.Second))
	if _, err := seed(s.db, "c2c:alice:carol", []string{"alice", "carol"}, 200, 5000); err != nil {
		r.note("预置 5000 字会话失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	tokens, err := setupUsers(s, []string{"alice", "bob", "carol"})
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	cs, errs := connectAll(s, []string{"bob", "alice"}, tokens, 100)
	if len(errs) > 0 {
		r.note("连接失败：%v", errs[0])
		return r
	}
	defer closeAll(cs)
	bob := cs["bob"]

	run := func(name string, req func() (map[string]any, int64)) summary {
		var lat latencies
		bad := 0
		for range 100 {
			q, wantFirst := req()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			start := time.Now()
			a, err := bob.syncReq(ctx, q)
			lat.add(time.Since(start))
			cancel()
			if err != nil || len(a.Messages) != 200 || a.Messages[0].Seq != wantFirst || !a.HasMore {
				bad++
			}
		}
		if bad > 0 {
			r.note("%s：%d 次结果不符合预期（条数、起点或 hasMore）", name, bad)
		}
		return lat.summary()
	}
	after := run("afterSeq", func() (map[string]any, int64) {
		x := rand.Int64N(99_000) // #nosec G404 -- 随机选 sync 起点，与安全无关；保证后面还有更多，hasMore=true
		return map[string]any{"convId": "c2c:alice:bob", "afterSeq": x, "limit": 200}, x + 1
	})
	before := run("beforeSeq", func() (map[string]any, int64) {
		y := 1000 + rand.Int64N(99_000) // #nosec G404 -- 随机选 sync 起点，与安全无关
		return map[string]any{"convId": "c2c:alice:bob", "beforeSeq": y, "limit": 200}, y - 200
	})
	r.row("afterSeq", "P50 %s，P99 %s，最大 %s", ms(after.P50), ms(after.P99), ms(after.Max))
	r.row("beforeSeq", "P50 %s，P99 %s，最大 %s", ms(before.P50), ms(before.P99), ms(before.Max))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	start := time.Now()
	big, err := cs["alice"].syncAfter(ctx, "c2c:alice:carol", 0, 200)
	cancel()
	if err != nil {
		r.note("5000 字会话 sync 失败：%v", err)
	} else {
		r.row("参考：200 条 × 5000 字", "%s，%d 条（约 3MB 单帧，不作为通过条件）", ms(time.Since(start)), len(big.Messages))
	}
	r.Pass = len(r.Notes) == 1 && after.P99 <= 200*time.Millisecond && before.P99 <= 200*time.Millisecond
	return r
}

// ---------------------------------------------------------------------------
// TC-PERF-00-07：200 人群，10 人各 5 条/秒，所有成员收到的延迟。

func scenarioGroup(cfg config) result {
	r := result{Case: "TC-PERF-00-07", Title: "200 人群扩散", Target: fmt.Sprintf("10 人各 5 条/秒、持续 %s；所有成员收到的 P99 ≤ 1000ms，无缺口、无重复，无 4400", cfg.groupDuration)}
	s := newServer(cfg.bin, cfg.dir, "group")
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	members := ids("m", 200, 3)
	tokens, err := setupUsers(s, members)
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	if err := s.createGroup("g200", members); err != nil {
		r.note("建群失败：%v", err)
		return r
	}
	cs, errs := connectAll(s, members, tokens, 200)
	if len(errs) > 0 {
		r.note("连接失败 %d 个：%v", len(errs), errs[0])
		return r
	}
	defer closeAll(cs)

	var lat latencies
	var mu sync.Mutex
	got := make(map[string]map[int64]int, len(members)) // 成员 → seq → 收到次数
	for _, uid := range members {
		got[uid] = map[int64]int{}
		fn := func(m protocol.Message, at time.Time) {
			if d, ok := latencyOf(m.Body.Text, at); ok {
				lat.add(d)
			}
			mu.Lock()
			got[uid][m.Seq]++
			mu.Unlock()
		}
		cs[uid].onMsg.Store(&fn)
	}
	acked := map[int64]string{} // seq → 发送者
	var ackLat latencies
	var errCodes counter
	m := startMonitor(s, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.groupDuration)
	var wg sync.WaitGroup
	for _, sender := range members[:10] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pace(ctx, 5, func(i int) {
				sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer scancel()
				a, d, err := cs[sender].sendText(sctx, protocol.ConvGroup, "g200", fmt.Sprintf("%s-%06d", sender, i), 0)
				if err != nil {
					errCodes.inc(errKey(err))
					return
				}
				ackLat.add(d)
				mu.Lock()
				acked[a.Seq] = sender
				mu.Unlock()
			})
		}()
	}
	wg.Wait()
	cancel()
	time.Sleep(2 * time.Second) // 等最后的投递到达
	m.finish()

	mu.Lock()
	gaps, dups := 0, 0
	for _, uid := range members {
		for seq, from := range acked {
			if from == uid {
				continue // 只有一个连接：自己发的消息只收到 ack，不会收到 msg
			}
			switch got[uid][seq] {
			case 0:
				gaps++
			case 1:
			default:
				dups++
			}
		}
	}
	total := len(acked)
	mu.Unlock()
	slow := 0
	for _, c := range cs {
		if reason := c.unexpectedClose(); reason != "" {
			slow++
			r.note("连接意外断开：%s", reason)
		}
	}
	sum := lat.summary()
	r.row("成功发送", "%d 条（%.1f 条/秒）", total, float64(total)/cfg.groupDuration.Seconds())
	r.row("投递次数", "%d（消息 × 接收成员）", sum.N)
	r.row("投递延迟", "P50 %s，P99 %s，最大 %s", ms(sum.P50), ms(sum.P99), ms(sum.Max))
	r.row("ack 延迟", "%s", ackLat.summary())
	r.row("缺失 / 重复", "%d / %d", gaps, dups)
	r.row("发送错误", "%s", errCodes.String())
	r.row("意外断开", "%d", slow)
	r.row("服务端", "CPU 平均 %.0f%%，Go 内存峰值 %s（RSS 峰值 %s，仅供参考），goroutine 峰值 %.0f", m.avgCPU()*100, mb(m.peakHeap), mb(m.peakRSS), m.peakGoroutines)
	r.Pass = total > 0 && gaps == 0 && dups == 0 && slow == 0 && errCodes.total() == 0 && sum.P99 <= time.Second
	return r
}

// pace 以 perSecond 的速率调用 fn（每次在新的 goroutine 里，不因为等 ack 而降速），直到 ctx 结束。
func pace(ctx context.Context, perSecond float64, fn func(i int)) {
	t := time.NewTicker(time.Duration(float64(time.Second) / perSecond))
	defer t.Stop()
	var wg sync.WaitGroup
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-t.C:
			wg.Add(1)
			go func() { defer wg.Done(); fn(i) }()
		}
	}
}

func errKey(err error) string {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		return fmt.Sprintf("%d", pe.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "超时"
	}
	return "连接错误"
}

// ---------------------------------------------------------------------------
// TC-PERF-00-02 / 03 / 05：1 万连接在线 + 500～600 条/秒 + 保持。

// load 是吞吐压测的发送计划：50 对单聊各 8 条/秒，5 个 10 人群各 2 人 10 条/秒，合计 500 条/秒；scale 按比例加压。
type load struct {
	users  []string
	pairs  [][2]string // 发送者 → 接收者
	groups map[string][]string
}

func newLoad() load {
	l := load{users: ids("s", 100, 3), groups: map[string][]string{}}
	for i := 0; i < 100; i += 2 {
		l.pairs = append(l.pairs, [2]string{l.users[i], l.users[i+1]})
	}
	for g := range 5 {
		l.groups[fmt.Sprintf("tg%d", g)] = l.users[g*10 : g*10+10]
	}
	return l
}

type ackRecord struct {
	conv  string
	seq   int64
	msgID string
}

type loadStats struct {
	ackLat  latencies
	errs    counter
	win     *windows
	mu      sync.Mutex
	acks    []ackRecord
	perConv map[string]int
}

// run 按计划发送，直到 ctx 结束。group 发送者选每个群里的偶数位成员（单聊里的接收方），避免单人超过 20 条/秒的限流。
// phase 写进 clientMsgId：不同阶段的序号都从 0 开始，不带阶段会和前一阶段的 clientMsgId 重复，
// 服务端按去重规则返回第一次的 ack，工具会把它误当成新消息计数（冒烟测试时 sync 核对发现了这个问题）。
func (l load) run(ctx context.Context, cs map[string]*client, scale float64, phase string, st *loadStats) {
	send := func(c *client, convType, to string, i int) {
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a, d, err := c.sendText(sctx, convType, to, fmt.Sprintf("%s-%s-%s-%07d", c.uid, to, phase, i), 100)
		if err != nil {
			st.errs.inc(errKey(err))
			return
		}
		st.ackLat.add(d)
		if st.win != nil {
			st.win.add(time.Now())
		}
		st.mu.Lock()
		st.acks = append(st.acks, ackRecord{a.ConvID, a.Seq, a.MsgID})
		st.perConv[a.ConvID]++
		st.mu.Unlock()
	}
	var wg sync.WaitGroup
	for _, p := range l.pairs {
		wg.Add(1)
		go func() { defer wg.Done(); pace(ctx, 8*scale, func(i int) { send(cs[p[0]], protocol.ConvC2C, p[1], i) }) }()
	}
	for gid, members := range l.groups {
		for _, sender := range []string{members[1], members[3]} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pace(ctx, 10*scale, func(i int) { send(cs[sender], protocol.ConvGroup, gid, i) })
			}()
		}
	}
	wg.Wait()
}

// verify 压测结束后逐个会话 sync，核对条数和 seq 连续。
func (l load) verify(cs map[string]*client, st *loadStats) (bad []string) {
	convs := map[string]*client{}
	for _, p := range l.pairs {
		convs[protocol.C2CConvID(p[0], p[1])] = cs[p[1]]
	}
	for gid, members := range l.groups {
		convs[protocol.GroupConvID(gid)] = cs[members[0]]
	}
	for conv, c := range convs {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		msgs, err := c.syncAll(ctx, conv)
		cancel()
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: sync 失败 %v", conv, err))
			continue
		}
		for i, m := range msgs {
			if m.Seq != int64(i+1) {
				bad = append(bad, fmt.Sprintf("%s: 第 %d 条 seq=%d，不连续", conv, i+1, m.Seq))
				break
			}
		}
		st.mu.Lock()
		want := st.perConv[conv]
		st.mu.Unlock()
		if len(msgs) != want {
			bad = append(bad, fmt.Sprintf("%s: sync %d 条，ack %d 条", conv, len(msgs), want))
		}
	}
	return bad
}

func scenarioCapacity(cfg config) []result {
	tc02 := result{Case: "TC-PERF-00-02", Title: "吞吐 ≥ 500 条/秒", Target: fmt.Sprintf("预热 30 秒 500 条/秒，再以 600 条/秒持续 %s：平均和任意 10 秒窗口都 ≥ 500 条/秒，错误 0，sync 核对无缺无重", cfg.throughputDuration)}
	tc03 := result{Case: "TC-PERF-00-03", Title: fmt.Sprintf("保持 %d 条长连接", cfg.conns), Target: fmt.Sprintf("全部建立成功，保持 %s 期间意外断开 0，抽样消息 P99 ≤ 1000ms，内存增长 ≤ 10%%", cfg.hold)}
	tc05 := result{Case: "TC-PERF-00-05", Title: "1 万连接 + 500 条/秒下的延迟", Target: "吞吐 ≥ 500 条/秒、观测连接对 P99 ≤ 1000ms、意外断开 0 同时满足"}
	fail := func(format string, args ...any) []result {
		for _, r := range []*result{&tc02, &tc03, &tc05} {
			r.note(format, args...)
		}
		return []result{tc02, tc03, tc05}
	}

	s := newServer(cfg.bin, cfg.dir, "capacity")
	if err := s.start(); err != nil {
		return fail("启动失败：%v", err)
	}
	defer s.stop()
	idle := ids("c", cfg.conns, 5)
	l := newLoad()
	all := slices.Concat(idle, l.users, []string{"obs1", "obs2"})
	prepStart := time.Now()
	tokens, err := setupUsers(s, all)
	if err != nil {
		return fail("准备用户失败：%v", err)
	}
	for gid, members := range l.groups {
		if err := s.createGroup(gid, members); err != nil {
			return fail("建群失败：%v", err)
		}
	}
	m := startMonitor(s, 5*time.Second)
	baseMem := s.liveMem()

	// 1. 以 200 个/秒建立 1 万条空闲连接。
	connStart := time.Now()
	idleConns, errs := connectAll(s, idle, tokens, 200)
	tc03.row("建立连接", "%d / %d 成功，用时 %s（准备用户和 token 用时 %s）", len(idleConns), len(idle), time.Since(connStart).Round(time.Second), connStart.Sub(prepStart).Round(time.Second))
	if len(errs) > 0 {
		tc03.note("建立失败 %d 条，第一条：%v", len(errs), errs[0])
	}
	defer closeAll(idleConns)
	time.Sleep(3 * time.Second)
	connMem := s.liveMem()
	tc03.row("每连接内存", "%.1f KB（GC 后存活的 Go 堆 + 栈：%s → %s）", (connMem-baseMem)/float64(max(1, len(idleConns)))/1024, mb(baseMem), mb(connMem))
	holdStart := time.Now()

	// 2. 吞吐（TC-02 / TC-05），同时用观测连接对每秒 1 条测延迟。
	active, errs := connectAll(s, slices.Concat(l.users, []string{"obs1", "obs2"}), tokens, 200)
	if len(errs) > 0 {
		return fail("活跃连接建立失败：%v", errs[0])
	}
	defer closeAll(active)
	var obsLat latencies
	obsFn := func(m protocol.Message, at time.Time) {
		if d, ok := latencyOf(m.Body.Text, at); ok {
			obsLat.add(d)
		}
	}
	active["obs2"].onMsg.Store(&obsFn)

	st := &loadStats{perConv: map[string]int{}}
	warm, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	l.run(warm, active, 1.0, "warm", st)
	cancel()
	warmAcks := len(st.acks)
	st.win = newWindows(10 * time.Second)
	mainCtx, cancel := context.WithTimeout(context.Background(), cfg.throughputDuration)
	obsDone := make(chan struct{})
	go func() {
		defer close(obsDone)
		pace(mainCtx, 1, func(i int) {
			sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer scancel()
			active["obs1"].sendText(sctx, protocol.ConvC2C, "obs2", fmt.Sprintf("obs-%06d", i), 0)
		})
	}()
	mainStart := time.Now()
	l.run(mainCtx, active, 1.2, "main", st)
	cancel()
	<-obsDone
	mainSecs := time.Since(mainStart).Seconds()
	time.Sleep(time.Second)
	// 内存稳定性从吞吐结束时算起：吞吐阶段增加的缓冲区、数据库缓存是正常的，要看的是之后保持期间有没有泄漏。
	afterLoadMem := s.liveMem()

	mainAcks := len(st.acks) - warmAcks
	rates := st.win.rates()
	minRate := 0.0
	if len(rates) > 0 {
		minRate = slices.Min(rates)
	}
	avg := float64(mainAcks) / mainSecs
	bad := l.verify(active, st)
	tc02.row("预热", "%d 条 / 30 秒（%.0f 条/秒）", warmAcks, float64(warmAcks)/30)
	tc02.row("加压阶段平均", "%.0f 条/秒（%d 条 / %.0f 秒）", avg, mainAcks, mainSecs)
	tc02.row("10 秒窗口最低", "%.0f 条/秒（共 %d 个窗口）", minRate, len(rates))
	tc02.row("ack 延迟", "%s", st.ackLat.summary())
	tc02.row("错误", "%s", st.errs.String())
	tc02.row("sync 核对", "%d 个会话，%d 处不一致", len(l.pairs)+len(l.groups), len(bad))
	for _, b := range bad {
		tc02.note("%s", b)
	}
	tc02.Pass = avg >= 500 && minRate >= 500 && st.errs.total() == 0 && len(bad) == 0

	obs := obsLat.summary()
	tc05.row("在线连接", "%d 空闲 + %d 活跃", len(idleConns), len(active))
	tc05.row("吞吐", "%.0f 条/秒（10 秒窗口最低 %.0f）", avg, minRate)
	tc05.row("观测延迟", "P50 %s，P99 %s，最大 %s（%d 条）", ms(obs.P50), ms(obs.P99), ms(obs.Max), obs.N)

	// 3. 继续保持到 hold，每分钟随机 100 对空闲用户互发 1 条（TC-03）。
	var sampleLat latencies
	var sampleSent atomic.Int64
	var sampleErr counter
	sampleFn := func(m protocol.Message, at time.Time) {
		if d, ok := latencyOf(m.Body.Text, at); ok {
			sampleLat.add(d)
		}
	}
	for _, c := range idleConns {
		c.onMsg.Store(&sampleFn)
	}
	round := 0
	for round == 0 || time.Since(holdStart) < cfg.hold { // 至少抽样一轮
		round++
		var wg sync.WaitGroup
		for k := range 100 {
			a, b := idle[rand.IntN(len(idle))], idle[rand.IntN(len(idle))] // #nosec G404 -- 随机抽样用户，与安全无关
			if a == b || idleConns[a] == nil || idleConns[b] == nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				sampleSent.Add(1)
				// clientMsgId 带上收件人和序号：同一轮可能随机到同一个发送者两次，ID 相同会被判为 4008。
				if _, _, err := idleConns[a].sendText(ctx, protocol.ConvC2C, b, fmt.Sprintf("hold-%s-%s-%d-%d", a, b, round, k), 0); err != nil {
					sampleErr.inc(errKey(err))
				}
			}()
		}
		wg.Wait()
		time.Sleep(min(time.Minute, max(0, cfg.hold-time.Since(holdStart))))
	}
	time.Sleep(2 * time.Second)
	m.finish()
	endMem := s.liveMem()

	unexpected := counter{}
	for _, cs := range []map[string]*client{idleConns, active} {
		for _, c := range cs {
			if reason := c.unexpectedClose(); reason != "" {
				unexpected.inc(reason)
			}
		}
	}
	samples := sampleLat.summary()
	growth := (endMem - afterLoadMem) / afterLoadMem * 100
	tc03.row("保持时长", "%s（期间做了吞吐压测 %s）", time.Since(holdStart).Round(time.Second), cfg.throughputDuration)
	tc03.row("抽样消息", "发送 %d 条，失败 %d（%s），收到 %d；P50 %s，P99 %s", sampleSent.Load(), sampleErr.total(), sampleErr.String(), samples.N, ms(samples.P50), ms(samples.P99))
	tc03.row("意外断开", "%d（%s）", unexpected.total(), unexpected.String())
	tc03.row("存活内存（GC 后）", "建立连接后 %s，吞吐结束 %s，保持结束 %s；吞吐结束后增长 %.1f%%", mb(connMem), mb(afterLoadMem), mb(endMem), growth)
	tc03.row("内存峰值", "Go 堆 + 栈 %s；进程 RSS %s（macOS 上波动大，仅供参考）", mb(m.peakHeap), mb(m.peakRSS))
	tc03.row("服务端", "CPU 平均 %.0f%%，goroutine 峰值 %.0f", m.avgCPU()*100, m.peakGoroutines)
	tc03.Pass = len(errs) == 0 && len(idleConns) == cfg.conns && unexpected.total() == 0 &&
		sampleErr.total() == 0 && samples.N == int(sampleSent.Load()) && samples.P99 <= time.Second && growth <= 10
	tc05.row("意外断开", "%d", unexpected.total())
	tc05.Pass = avg >= 500 && obs.P99 <= time.Second && unexpected.total() == 0
	return []result{tc02, tc03, tc05}
}

// ---------------------------------------------------------------------------
// TC-PERF-00-06：500 条/秒发送中 kill -9，重启后已 ack 的消息一条不丢。

func scenarioKill9(cfg config) result {
	r := result{Case: "TC-PERF-00-06", Title: "压测中 kill -9 不丢已 ack 的消息", Target: fmt.Sprintf("以 500 条/秒发送 %s 后 SIGKILL，重启后每条已 ack 的消息都能 sync 到，msgId、seq 一致", cfg.killAfter)}
	s := newServer(cfg.bin, cfg.dir, "kill9")
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	l := newLoad()
	tokens, err := setupUsers(s, l.users)
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	for gid, members := range l.groups {
		s.createGroup(gid, members)
	}
	cs, errs := connectAll(s, l.users, tokens, 200)
	if len(errs) > 0 {
		r.note("连接失败：%v", errs[0])
		return r
	}
	st := &loadStats{perConv: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.run(ctx, cs, 1.0, "kill9", st); close(done) }()
	time.Sleep(cfg.killAfter)
	s.kill9()
	cancel()
	<-done
	st.mu.Lock()
	acks := slices.Clone(st.acks)
	st.mu.Unlock()

	s2 := &server{bin: s.bin, db: s.db, logPath: filepath.Join(cfg.dir, "kill9-restart.log")}
	if err := s2.start(); err != nil {
		r.note("重启失败：%v", err)
		return r
	}
	defer s2.stop()
	readers := map[string]*client{}
	got := map[string]map[int64]string{} // conv → seq → msgId
	missing, mismatch := 0, 0
	for _, a := range acks {
		if got[a.conv] == nil {
			conv, _ := protocol.ParseConvID(a.conv)
			reader := conv.A
			if conv.Type == protocol.ConvGroup {
				reader = l.groups[conv.A][0]
			}
			if readers[reader] == nil {
				c, err := dial(context.Background(), s2.wsURL(), reader, tokens[reader])
				if err != nil {
					r.note("重启后连接失败：%v", err)
					return r
				}
				readers[reader] = c
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			msgs, err := readers[reader].syncAll(ctx, a.conv)
			cancel()
			if err != nil {
				r.note("重启后 sync 失败：%v", err)
				return r
			}
			got[a.conv] = map[int64]string{}
			for _, m := range msgs {
				got[a.conv][m.Seq] = m.MsgID
			}
		}
		switch id, ok := got[a.conv][a.seq]; {
		case !ok:
			missing++
		case id != a.msgID:
			mismatch++
		}
	}
	closeAll(readers)
	stored := 0
	for _, m := range got {
		stored += len(m)
	}
	r.row("强杀前已 ack", "%d 条（%d 个会话）", len(acks), len(got))
	r.row("重启后 sync 到", "%d 条（多出的是已落盘但 ack 没来得及返回的，允许）", stored)
	r.row("丢失 / msgId 不一致", "%d / %d", missing, mismatch)
	r.Pass = len(acks) > 0 && missing == 0 && mismatch == 0
	return r
}
