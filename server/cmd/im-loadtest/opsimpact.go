package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
)

// PRD-ops S-OPS-25：运维流量对 IM 主流程的影响。同一轮压测里按"关 / 开 / 关 / 开 / 关 / 开"交替，
// 比较 3 个开段和 3 个关段 P99 的中位数（AC-OPS-90-1）。
//
// 默认的运维流量直接对节点运维接口施加。完整验收（BE-321）加 -ops-idle 10000 -ops-phase-file：
// 流量由 console-server 的 tools/ops-accept 驱动 20 个测试账号，这里只写开关段，两边在同一轮里交替。

const opsKeyID = "loadtest"

type opsDriver struct {
	base string
	node string
	key  ed25519.PrivateKey
	sent atomic.Int64
	bad  atomic.Int64
}

func (d *opsDriver) do(method, path string, body []byte) {
	nonce := make([]byte, 18)
	if _, err := rand.Read(nonce); err != nil {
		d.bad.Add(1)
		return
	}
	ts, n := strconv.FormatInt(time.Now().UnixMilli(), 10), base64.RawURLEncoding.EncodeToString(nonce)
	sts := adminauth.StringToSign(adminauth.Request{Method: method, Path: path, Timestamp: ts, Nonce: n, Body: body, Operator: "ops:loadtest", TargetNode: d.node})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, d.base+path, bytes.NewReader(body))
	if err != nil {
		d.bad.Add(1)
		return
	}
	req.Header.Set("X-IM-Caller", "console-ops")
	req.Header.Set("X-IM-Timestamp", ts)
	req.Header.Set("X-IM-Nonce", n)
	req.Header.Set("X-IM-Operator", "ops:loadtest")
	req.Header.Set("X-IM-Target-Node", d.node)
	req.Header.Set("X-IM-Signature", adminauth.EncodeSignature(ed25519.Sign(d.key, []byte(sts))))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		d.bad.Add(1)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	d.sent.Add(1)
	// 并发闸门的 429 是预期之内的保护，不算错误。
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusTooManyRequests {
		d.bad.Add(1)
	}
}

// run 模拟 20 个测试账号：每秒一次节点探测 + 随机的排查查询和配置检查。
func (d *opsDriver) run(ctx context.Context, users []string) {
	pace(ctx, 20, func(i int) {
		u := users[mrand.IntN(len(users))] // #nosec G404 -- 随机选查询对象，与安全无关
		switch i % 5 {
		case 0, 1:
			d.do("GET", "/internal/v1/node", nil)
		case 2:
			d.do("POST", "/internal/v1/connections:query", []byte(`{"userId":"`+u+`"}`))
		case 3:
			d.do("POST", "/internal/v1/disconnects:query", []byte(`{"userId":"`+u+`"}`))
		default:
			if i%2 == 0 {
				d.do("GET", "/internal/v1/config", nil)
			} else {
				d.do("GET", "/internal/v1/db", nil)
			}
		}
	})
}

type opsSegment struct {
	on         bool
	p99        time.Duration
	throughput float64
}

func median(ds []time.Duration) time.Duration {
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[len(s)/2]
}

func scenarioOpsImpact(cfg config) result {
	const pairs, perSecond = 50, 500.0
	caseName := "AC-OPS-90-1（im-server 部分）"
	if cfg.opsPhaseFile != "" {
		caseName = "AC-OPS-90-1（运维流量经 console-server）"
	}
	r := result{Case: caseName, Title: "运维接口对 IM 主流程的影响",
		Target: fmt.Sprintf("关 / 开交替 6 段、每段 %s；%d 对用户 %g 条/秒：开段 P99 中位数 ≤ 关段 + 20ms，每段 P99 ≤ 1 秒，意外断开 0，吞吐 ≥ 490 条/秒", cfg.opsSegment, pairs, perSecond)}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		r.note("生成密钥失败：%v", err)
		return r
	}
	s := newServer(cfg.bin, cfg.dir, "opsimpact")
	s.opsAddr = freeAddr()
	nodeID, pubkeys := "loadtest-node", opsKeyID+":"+base64.StdEncoding.EncodeToString(pub)
	external := cfg.opsPhaseFile != ""
	if external {
		nodeID, pubkeys = "n1", cfg.opsPubkey
	}
	s.env = []string{"IM_OPS_ADDR=" + s.opsAddr, "IM_NODE_ID=" + nodeID, "IM_OPS_PUBKEYS=" + pubkeys}
	setPhase := func(p string) {
		if !external {
			return
		}
		tmp := cfg.opsPhaseFile + ".tmp"
		if err := os.WriteFile(tmp, []byte(p), 0o600); err == nil {
			err = os.Rename(tmp, cfg.opsPhaseFile)
			if err != nil {
				r.note("写开关段失败：%v", err)
			}
		}
	}
	setPhase("off")
	defer setPhase("done")
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	senderIDs, receivers := ids("a", pairs, 3), ids("b", pairs, 3)
	users := slices.Concat(senderIDs, receivers)
	tokens, err := setupUsers(s, users)
	if err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	cs, errs := connectAll(s, users, tokens, 100)
	if len(errs) > 0 {
		r.note("连接失败：%v", errs[0])
		return r
	}
	defer closeAll(cs)
	if cfg.opsIdle > 0 {
		idle := ids("c", cfg.opsIdle, 5)
		idleTokens, err := setupUsers(s, idle)
		if err != nil {
			r.note("准备空闲用户失败：%v", err)
			return r
		}
		started := time.Now()
		idleConns, errs := connectAll(s, idle, idleTokens, 200)
		if len(errs) > 0 {
			r.note("空闲连接失败：%d 条，第一条：%v", len(errs), errs[0])
			return r
		}
		defer closeAll(idleConns)
		r.row("空闲长连接", "%d 条，用时 %s", len(idleConns), time.Since(started).Round(time.Second))
	}

	var mu sync.Mutex
	current := &latencies{}
	for _, b := range receivers {
		fn := func(m protocol.Message, at time.Time) {
			if d, ok := latencyOf(m.Body.Text, at); ok {
				mu.Lock()
				l := current
				mu.Unlock()
				l.add(d)
			}
		}
		cs[b].onMsg.Store(&fn)
	}

	ctx, stopSend := context.WithCancel(context.Background())
	var sent, failed atomic.Int64
	var senders sync.WaitGroup
	senders.Add(1)
	go func() {
		defer senders.Done()
		pace(ctx, perSecond, func(i int) {
			idx := i % pairs
			from, to := senderIDs[idx], receivers[idx]
			sendCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, _, err := cs[from].sendText(sendCtx, protocol.ConvC2C, to, fmt.Sprintf("ops-%09d", i), 0); err != nil {
				failed.Add(1)
				return
			}
			sent.Add(1)
		})
	}()

	driver := &opsDriver{base: "http://" + s.opsAddr, node: "loadtest-node", key: priv}
	var segments []opsSegment
	for i := range 6 {
		on := i%2 == 1
		l := &latencies{}
		mu.Lock()
		current = l
		mu.Unlock()
		startSent := sent.Load()
		segCtx, cancel := context.WithTimeout(context.Background(), cfg.opsSegment)
		switch {
		case on && external:
			setPhase("on")
			<-segCtx.Done()
			setPhase("off")
		case on:
			driver.run(segCtx, users)
		default:
			<-segCtx.Done()
		}
		cancel()
		sum := l.summary()
		seg := opsSegment{on: on, p99: sum.P99, throughput: float64(sent.Load()-startSent) / cfg.opsSegment.Seconds()}
		segments = append(segments, seg)
		label := "关"
		if on {
			label = "开"
		}
		r.row(fmt.Sprintf("第 %d 段（%s）", i+1, label), "P99 %s，吞吐 %.0f 条/秒", ms(seg.p99), seg.throughput)
	}
	stopSend()
	senders.Wait()

	var onP99, offP99 []time.Duration
	segOK := true
	for _, seg := range segments {
		if seg.on {
			onP99 = append(onP99, seg.p99)
		} else {
			offP99 = append(offP99, seg.p99)
		}
		segOK = segOK && seg.p99 <= time.Second && seg.throughput >= 490
	}
	// 压测全程没有人主动断开：连接数少于建立时的数量就是意外断开。
	disconnects := int(s.scrape()[`im_connections{platform="web"}`]) < len(users)+cfg.opsIdle
	diff := median(onP99) - median(offP99)
	r.row("P99 中位数", "开 %s，关 %s，差 %s", ms(median(onP99)), ms(median(offP99)), ms(diff))
	if !external {
		r.row("运维请求", "%d 次，异常 %d 次", driver.sent.Load(), driver.bad.Load())
	}
	r.row("发送失败", "%d 次", failed.Load())
	r.row("意外断开", "%v", disconnects)
	r.Pass = diff <= 20*time.Millisecond && segOK && !disconnects && driver.bad.Load() == 0 && failed.Load() == 0
	return r
}
