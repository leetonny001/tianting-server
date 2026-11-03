package e2e

// 同步模型测试（PRD-social 16 节、AC-SOC-06-15，BE-119）：Go 参考客户端（socialmodel）+ 随机操作生成器。
// 两个用户各两个连接，随机执行申请 / 同意 / 拒绝 / 删除 / 备注 / 拉黑 / 取消拉黑 / 隐私 / 标记已读 / 管理接口导入删除，
// 随机断线重连（保留或清空本地状态）、丢弃事件、推进测试时钟并清理，同步 limit 随机；
// 静止后断言每个端的本地状态与服务端全量逐项相同、本地 ver 等于服务端 ver。失败时收缩成最小复现序列并输出种子。

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/e2e/socialmodel"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type modelOp struct {
	Kind  string
	Actor int // 0 / 1：哪个用户；Conn：哪个连接
	Conn  int
	Peer  int // 0 / 1 / 2（2 = 不存在的用户）
	Arg   string
	N     int
}

func (o modelOp) String() string {
	return fmt.Sprintf("%s(u%d.c%d → p%d %q %d)", o.Kind, o.Actor, o.Conn, o.Peer, o.Arg, o.N)
}

var modelKinds = []string{"apply", "apply", "accept", "accept", "reject", "delete", "remark", "block", "unblock", "privacy", "seen",
	"import", "remove", "reconnect", "refresh", "drop", "advance", "purge", "sync", "sync"}

func genOps(r *rand.Rand, n int) []modelOp {
	ops := make([]modelOp, n)
	for i := range ops {
		ops[i] = modelOp{Kind: modelKinds[r.IntN(len(modelKinds))], Actor: r.IntN(2), Conn: r.IntN(2), Peer: r.IntN(3), N: r.IntN(500) + 1}
		ops[i].Arg = []string{"", "r1", "r2", "x"}[r.IntN(4)]
		if ops[i].N > 50 && r.IntN(2) == 0 {
			ops[i].N = r.IntN(4) + 1 // 小页更容易出现跨页
		}
	}
	return ops
}

type modelRun struct {
	t       *testing.T
	s       *socEnv
	users   [3]string
	conns   [2][2]*client
	models  [2][2]*socialmodel.Client
	dropped bool
	fail    string
}

func (m *modelRun) failf(format string, args ...any) {
	if m.fail == "" {
		m.fail = fmt.Sprintf(format, args...)
	}
}

func (m *modelRun) call(c *client, cmd string, data any) frame {
	f := c.call(cmd, data)
	return f
}

// drain 把已经到达的事件交给模型（不等待）。
func (m *modelRun) drain(u, k int) {
	c, mdl := m.conns[u][k], m.models[u][k]
	handle := func(f frame) {
		if f.Cmd != protocol.CmdEvent {
			return
		}
		var ev protocol.SocialChangedEvent
		json.Unmarshal(f.Data, &ev)
		if ev.Type == protocol.EventSocialChanged {
			mdl.OnSocialChanged(ev.Ver)
		}
	}
	for _, f := range c.pending {
		handle(f)
	}
	c.pending = nil
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			handle(f)
		default:
			return
		}
	}
}

// syncRounds 按 10.2 节跑同步轮，直到不再需要。
func (m *modelRun) syncRounds(u, k, limit int) {
	c, mdl := m.conns[u][k], m.models[u][k]
	for round := 0; round < 20; round++ {
		since := mdl.BeginRound()
		data := map[string]any{"sinceVer": since, "limit": limit}
		for page := 0; ; page++ {
			if page > 10000 {
				m.failf("round never ends")
				return
			}
			f := m.call(c, protocol.CmdSocialSync, data)
			if f.Error != nil && f.Error.Code == protocol.CodeRateLimited {
				m.s.resetLimits(m.users[u])
				continue
			}
			if f.Error != nil {
				m.failf("socialSync: %+v", f.Error)
				return
			}
			var p protocol.SocialSyncAck
			json.Unmarshal(f.Data, &p)
			switch mdl.ApplyPage(p) {
			case socialmodel.Continue:
				data = map[string]any{"cursor": p.Cursor, "limit": limit}
				continue
			case socialmodel.Restart:
				data = map[string]any{"sinceVer": since, "limit": limit}
				continue
			}
			break
		}
		m.drain(u, k)
		if !mdl.NeedSync {
			return
		}
	}
	m.failf("more than 20 rounds in a row")
}

func (m *modelRun) dial(u, k int) {
	m.conns[u][k] = m.s.dial(m.users[u], "web")
}

func (m *modelRun) applyAck(u, k int, f frame) {
	if f.Error != nil {
		switch f.Error.Code {
		case protocol.CodeBadRequest, protocol.CodeFriendRequestNotFound, protocol.CodeFriendRequestHandled, protocol.CodeNotFound:
			return // 规则内的拒绝：申请自己拉黑的人、申请不存在、已同意再拒绝、给非好友备注
		}
		m.failf("unexpected error %+v", f.Error)
		return
	}
	var a socialAck
	json.Unmarshal(f.Data, &a)
	m.models[u][k].ApplyAck(a.Ver, a.Changed, a.Items)
}

func (m *modelRun) exec(op modelOp) {
	for u := range 2 {
		m.s.resetLimits(m.users[u])
	}
	u, k := op.Actor, op.Conn
	c := m.conns[u][k]
	peer := m.users[op.Peer]
	if op.Peer == u { // 不能对自己操作：换成另一个人
		peer = m.users[1-u]
	}
	switch op.Kind {
	case "apply":
		m.applyAck(u, k, m.call(c, protocol.CmdFriendApply, map[string]any{"to": peer, "message": op.Arg, "remark": op.Arg}))
	case "accept", "reject":
		m.applyAck(u, k, m.call(c, protocol.CmdFriendRespond, map[string]any{"from": peer, "action": op.Kind, "remark": op.Arg}))
	case "delete":
		m.applyAck(u, k, m.call(c, protocol.CmdFriendDelete, map[string]any{"userId": peer}))
	case "remark":
		m.applyAck(u, k, m.call(c, protocol.CmdFriendRemark, map[string]any{"userId": peer, "remark": op.Arg}))
	case "block":
		m.applyAck(u, k, m.call(c, protocol.CmdBlock, map[string]any{"userId": peer}))
	case "unblock":
		m.applyAck(u, k, m.call(c, protocol.CmdUnblock, map[string]any{"userId": peer}))
	case "privacy":
		on := op.N%2 == 0
		f := m.call(c, protocol.CmdSetPrivacy, map[string]any{"searchable": on})
		var a protocol.VerAck
		json.Unmarshal(f.Data, &a)
		m.models[u][k].Searchable = on
		m.models[u][k].ApplyVerOnly(a.Ver, a.Changed)
	case "seen":
		f := m.call(c, protocol.CmdFriendRequestsSeen, map[string]any{"ver": m.models[u][k].Ver})
		var a protocol.RequestsSeenAck
		json.Unmarshal(f.Data, &a)
		m.models[u][k].RequestSeenVer = a.RequestSeenVer
		m.models[u][k].ApplyVerOnly(a.Ver, a.Changed)
	case "import", "remove":
		pair := map[string]any{"a": m.users[0], "b": m.users[1]}
		method, path := http.MethodPost, "/admin/friends/import"
		if op.Kind == "remove" {
			method, path = http.MethodDelete, "/admin/friends"
		} else if op.Arg != "" {
			pair["remarkA"] = op.Arg
		}
		if code, b := m.s.adm(method, path, map[string]any{"pairs": []any{pair}}); code != http.StatusOK {
			m.failf("%s: %d %s", path, code, b)
		}
	case "reconnect", "refresh":
		closeGraceful(c)
		m.dial(u, k)
		if op.Kind == "refresh" { // sdk-ts 刷新页面：内存状态清空，全量轮
			m.models[u][k] = socialmodel.New()
		}
		m.syncRounds(u, k, op.N) // 每次连接成功都从本地 ver 同步一轮（规则 10）
	case "drop":
		m.s.adminPost("/admin/test/social/drop-events", map[string]any{"userId": m.users[u], "type": "socialChanged", "count": op.N%3 + 1}, adminKey)
		m.dropped = true
	case "advance":
		m.s.advance([]time.Duration{time.Hour, 24 * time.Hour, 8 * 24 * time.Hour, 31 * 24 * time.Hour}[op.N%4])
		if op.N%4 >= 2 { // 推进超过 token 有效期：全部重连（保留本地状态）
			for uu := range 2 {
				for kk := range 2 {
					closeGraceful(m.conns[uu][kk])
					m.dial(uu, kk)
					m.syncRounds(uu, kk, 500)
				}
			}
		}
	case "purge":
		m.s.purge()
	case "sync":
		m.syncRounds(u, k, op.N)
	}
	for uu := range 2 {
		for kk := range 2 {
			m.drain(uu, kk)
		}
	}
}

// quiesce 等事件静止，处理需要的同步轮；序列里丢过事件时每个端再从本地 ver 同步一轮（相当于下一次事件或重连）。
func (m *modelRun) quiesce() {
	for i := 0; i < 10; i++ {
		time.Sleep(30 * time.Millisecond)
		busy := false
		for u := range 2 {
			for k := range 2 {
				m.drain(u, k)
				if m.models[u][k].NeedSync || m.dropped && i == 0 {
					busy = true
					m.syncRounds(u, k, 500)
				}
			}
		}
		if !busy {
			return
		}
	}
}

func (m *modelRun) check() {
	for u := range 2 {
		m.s.resetLimits(m.users[u])
		truth := socialmodel.New()
		c := m.s.dial(m.users[u], "pc")
		data := map[string]any{"sinceVer": 0, "limit": 500}
		for {
			p := mustAck[protocol.SocialSyncAck](m.t, c.call(protocol.CmdSocialSync, data))
			truth.BeginRound()
			if truth.ApplyPage(p) == socialmodel.Done {
				break
			}
			data = map[string]any{"cursor": p.Cursor, "limit": 500}
		}
		closeGraceful(c)
		want := truth.View(m.s.now)
		for k := range 2 {
			if got := m.models[u][k].View(m.s.now); !reflect.DeepEqual(got, want) {
				m.failf("user %d conn %d differs from the server:\n got %+v\nwant %+v", u, k, got, want)
				return
			}
		}
	}
}

// runSeq 在 s 上用一组新用户跑一个序列，返回失败原因（空 = 通过）。
func runSeq(t *testing.T, s *socEnv, tag string, ops []modelOp) string {
	m := &modelRun{t: t, s: s, users: [3]string{"a" + tag, "b" + tag, "ghost" + tag}}
	s.usersNamed(m.users[0], m.users[1])
	for u := range 2 {
		for k := range 2 {
			m.dial(u, k)
			m.models[u][k] = socialmodel.New()
			m.syncRounds(u, k, 500)
		}
	}
	for _, op := range ops {
		if m.exec(op); m.fail != "" {
			return m.fail
		}
	}
	m.quiesce()
	if m.fail == "" {
		m.check()
	}
	for u := range 2 {
		for k := range 2 {
			closeGraceful(m.conns[u][k])
		}
	}
	return m.fail
}

// shrink 贪心地删掉不影响失败的操作，得到最小复现序列。
func shrink(t *testing.T, s *socEnv, ops []modelOp, tagN *int) []modelOp {
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(ops); i++ {
			cand := append(append([]modelOp(nil), ops[:i]...), ops[i+1:]...)
			*tagN++
			if runSeq(t, s, "s"+strconv.Itoa(*tagN), cand) != "" {
				ops, changed = cand, true
				i--
			}
		}
	}
	return ops
}

// TC-SOC-06-21（AC-SOC-06-15）。序列数：IM_SOCIAL_MODEL_SEQS（CI 的模型测试步骤为 1000，固定种子与随机种子各一半）；
// 默认 100（随普通 e2e 和 race 检测一起跑）；-short 时 20。
func TestTCSOC0621SyncModel(t *testing.T) {
	n := 100
	if testing.Short() {
		n = 20
	}
	if v, err := strconv.Atoi(os.Getenv("IM_SOCIAL_MODEL_SEQS")); err == nil && v > 0 {
		n = v
	}
	randomBase := uint64(time.Now().UnixNano())
	const workers = 8
	var mu sync.Mutex
	failures := 0
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := newSocEnvCfg(t, func(c *app.Config) { // 大量重连：放开按 IP 的握手限流（所有连接都来自 127.0.0.1）
				c.Gateway.Guard.HandshakeRate, c.Gateway.Guard.HandshakeBurst = 100000, 100000
			})
			tagN := 0
			for i := w; i < n; i += workers {
				seed := uint64(i) // 前一半固定种子，后一半随机种子
				if i >= n/2 {
					seed = randomBase + uint64(i)
				}
				ops := genOps(rand.New(rand.NewPCG(seed, 0x50c1a1)), 10+int(seed%20))
				tagN++
				if why := runSeq(t, s, fmt.Sprintf("w%dn%d", w, tagN), ops); why != "" {
					min := shrink(t, s, ops, &tagN)
					var lines []string
					for _, op := range min {
						lines = append(lines, op.String())
					}
					mu.Lock()
					failures++
					t.Errorf("seed %d failed: %s\nminimal sequence (%d ops):\n  %s", seed, why, len(min), strings.Join(lines, "\n  "))
					mu.Unlock()
					if failures >= 3 {
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}

// closeGraceful 走 WebSocket 关闭握手，让服务端先关 TCP：大量重连时 TIME_WAIT 留在服务端，
// 不会耗尽客户端的临时端口（macOS 上 1000 个序列会用掉上万个连接）。
func closeGraceful(c *client) {
	c.ws.Close(websocket.StatusNormalClosure, "")
}
