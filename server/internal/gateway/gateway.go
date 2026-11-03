// Package gateway 负责 WebSocket 长连接：鉴权、读写循环、心跳超时、慢连接保护。
// 业务处理交给 Handler，网关本身不理解 send / sync 等命令。
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/auth"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/hub"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
)

// Handler 处理一条已解析的上行帧（ping 除外），返回要回给客户端的 ack；nil 表示不回复。
type Handler interface {
	Handle(ctx context.Context, s delivery.Session, in protocol.InFrame) *protocol.OutFrame
}

// TokenVerifier 校验 token。
type TokenVerifier interface {
	Verify(token string) (auth.Claims, error)
}

// RevocationChecker 返回用户当前的令牌版本号；token 里的版本号更小，说明是强制下线前签发的。
type RevocationChecker interface {
	TokenVersion(ctx context.Context, userID string) (int64, error)
}

// Config 是网关参数。
type Config struct {
	IdleTimeout     time.Duration // 多久收不到任何帧就断开（协议约定 90s）
	WriteTimeout    time.Duration // 单帧写超时
	SendBuffer      int           // 每个连接最多积压多少帧待发送，超过视为慢连接并断开（队列按需分配，不预占内存）
	ReadLimit       int64         // 单帧最大字节数
	NodeID          string        // 本节点标识，作为连接 ID 的前缀，保证集群内连接 ID 不重复
	AuthTimeout     time.Duration // 连接后多久内必须发送 auth（PRD C-52）
	MaxConnsPerUser int           // 每个用户的连接上限，超出时踢掉最早的连接（C-53）；<= 0 不限
	// AllowedOrigins 是浏览器 Origin 白名单（C-54），如 "https://im.example.com"、"*.example.com"、"localhost:*"。
	// 包含 "*" 时不校验（仅限开发）。没有 Origin 头的原生端、与服务端同 host 的页面总是允许。
	AllowedOrigins []string
	Guard          GuardConfig // 按 IP 的握手限流与鉴权失败封禁（ARCH S7）
}

// LocalOrigins 是未配置白名单时的默认值：只允许本机页面（本地开发 H5）。
var LocalOrigins = []string{"localhost", "localhost:*", "127.0.0.1", "127.0.0.1:*", "[::1]", "[::1]:*"}

// DefaultConfig 是生产默认值。
func DefaultConfig() Config {
	return Config{
		IdleTimeout:     90 * time.Second,
		WriteTimeout:    10 * time.Second,
		SendBuffer:      256,
		ReadLimit:       64 << 10,
		AuthTimeout:     5 * time.Second,
		MaxConnsPerUser: 10,
		AllowedOrigins:  LocalOrigins,
		Guard:           DefaultGuardConfig(),
	}
}

var platforms = map[string]bool{"android": true, "ios": true, "web": true, "pc": true}

// Gateway 是 /ws 的 http.Handler。
type Gateway struct {
	cfg        Config
	verifier   TokenVerifier
	revocation RevocationChecker
	clock      clock.Clock
	hub        *hub.Hub
	handler    Handler
	log        *slog.Logger
	metrics    *metrics.Metrics
	guard      *guard
	nextID     atomic.Uint64
}

func New(cfg Config, verifier TokenVerifier, revocation RevocationChecker, clk clock.Clock, h *hub.Hub, handler Handler, m *metrics.Metrics, log *slog.Logger) *Gateway {
	return &Gateway{cfg: cfg, verifier: verifier, revocation: revocation, clock: clk, hub: h, handler: handler, metrics: m, log: log, guard: newGuard(cfg.Guard, clk)}
}

// ServeHTTP 处理一条新连接：
//  1. 校验 Origin（不在白名单时库直接返回 HTTP 403，C-54）；
//  2. 升级后检查平台标识（4009，不发 kicked，C-05）；
//  3. 等待第一帧 auth（C-52），token 不从 URL 读取；
//  4. 凭证有问题时下发 kicked 再关闭：浏览器拿不到握手阶段的 HTTP 401，
//     只能靠 kicked 事件和关闭码区分"凭证问题"和"断网"（PRD F-AUTH-02 规则 3）。
//     检查顺序（C-08）：签名 4010 → 强制下线 4012 → 过期 4011。
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 在升级之前拒绝：被限流的 IP 不占用 WebSocket 连接和 goroutine。
	ip := g.guard.clientIP(r)
	if !g.guard.allowHandshake(ip) {
		g.handshake(metrics.HandshakeRateLimited)
		http.Error(w, "too many connection attempts", http.StatusTooManyRequests)
		g.log.Info("conn rejected", "reason", "handshake rate limited", "ip", ip)
		return
	}
	ws, err := websocket.Accept(w, r, g.acceptOptions())
	if err != nil {
		g.log.Warn("websocket accept failed", "err", err)
		return
	}
	ws.SetReadLimit(g.cfg.ReadLimit)

	platform := r.URL.Query().Get("platform")
	if !platforms[platform] {
		g.handshake(metrics.HandshakeBadPlatform)
		ws.Close(protocol.CloseInvalidPlatform, "invalid platform")
		g.log.Info("conn rejected", "reason", "invalid platform", "platform", platform)
		return
	}
	claims, authID, ok := g.authenticate(r.Context(), ws, ip)
	if !ok {
		return
	}
	uid := claims.UserID

	c := &conn{
		id:       g.cfg.NodeID + "-" + strconv.FormatUint(g.nextID.Add(1), 10),
		uid:      uid,
		platform: platform,
		// 剩余有效期在鉴权时按注入的时钟算好：auth 的 ack 发出之后测试可能马上改时钟，
		// 如果等 run 开始时再算，就会和改时钟赛跑（race 检测下变慢时会被立即踢下线）。
		ttl:  claims.ExpiresAt.Sub(g.clock.Now()),
		ws:   ws,
		done: make(chan struct{}),
		gw:   g,
	}
	// auth 的 ack 先入队、再登记到在线表：保证客户端先收到 ack，再收到任何推送。
	c.write(protocol.AckOK(authID, protocol.AuthAck{UserID: uid, ServerTime: g.clock.Now().UnixMilli()}))
	for _, old := range g.hub.Register(c, g.cfg.MaxConnsPerUser) {
		old.Kick(protocol.KickTooManyConns)
		g.log.Info("conn evicted", "conn", old.ConnID(), "uid", uid, "reason", protocol.KickTooManyConns)
	}
	g.handshake(metrics.HandshakeOK)
	g.metrics.Connections.WithLabelValues(platform).Inc()
	g.log.Info("conn open", "conn", c.id, "uid", uid, "platform", platform)
	opened := time.Now()

	cr := c.run(context.Background())

	g.hub.Unregister(c)
	g.metrics.Connections.WithLabelValues(platform).Dec()
	g.metrics.Disconnects.WithLabelValues(metrics.Code(cr.code), cr.by).Inc()
	// 记录关闭码、原因和关闭方，用于排查"为什么掉线"（BE-051、AC-AUTH-05-1）。
	g.log.Info("conn closed", "conn", c.id, "uid", uid, "platform", platform,
		"code", cr.code, "reason", cr.reason, "by", cr.by, "duration", time.Since(opened).Round(time.Millisecond))
}

func (g *Gateway) handshake(result string) { g.metrics.Handshakes.WithLabelValues(result).Inc() }

func (g *Gateway) acceptOptions() *websocket.AcceptOptions {
	if slices.Contains(g.cfg.AllowedOrigins, "*") {
		return &websocket.AcceptOptions{InsecureSkipVerify: true}
	}
	return &websocket.AcceptOptions{OriginPatterns: g.cfg.AllowedOrigins}
}

// authenticate 等待并校验第一帧 auth。失败时已经关闭连接，返回 ok=false。
func (g *Gateway) authenticate(ctx context.Context, ws *websocket.Conn, ip string) (claims auth.Claims, reqID string, ok bool) {
	// 用定时器而不是 Read 的 ctx 超时：ctx 到期时库会直接断开底层连接，客户端拿不到 4401（同空闲超时）。
	timer := time.AfterFunc(g.cfg.AuthTimeout, func() {
		ws.Close(protocol.CloseAuthRequired, "auth timeout")
	})
	typ, data, err := ws.Read(ctx)
	if !timer.Stop() || err != nil {
		g.handshake(metrics.HandshakeAuthRequired)
		g.log.Info("conn rejected", "reason", "no auth frame", "err", err)
		return auth.Claims{}, "", false
	}
	var in protocol.InFrame
	if typ != websocket.MessageText || json.Unmarshal(data, &in) != nil || in.Cmd != protocol.CmdAuth || in.ID == "" {
		g.handshake(metrics.HandshakeAuthRequired)
		ws.Close(protocol.CloseAuthRequired, "first frame must be auth")
		g.log.Info("conn rejected", "reason", "first frame is not auth", "cmd", in.Cmd)
		return auth.Claims{}, "", false
	}
	var req protocol.AuthReq
	_ = json.Unmarshal(in.Data, &req) // 解析失败时 token 为空，按无效凭证处理

	claims, verr := g.verifier.Verify(req.Token)
	if errors.Is(verr, auth.ErrInvalidToken) {
		// 只统计"凭证无效"：过期和被强制下线是正常用户也会遇到的情况，不算攻击。
		if g.guard.authFailed(ip) {
			g.log.Warn("ip blocked", "reason", "too many invalid tokens", "ip", ip, "for", g.cfg.Guard.BlockFor)
		}
		g.handshake(metrics.HandshakeTokenInvalid)
		g.kick(ws, protocol.KickTokenInvalid)
		return auth.Claims{}, "", false
	}
	version, err := g.revocation.TokenVersion(ctx, claims.UserID)
	if err != nil {
		g.log.Error("check revocation", "err", err, "uid", claims.UserID)
		g.handshake(metrics.HandshakeError)
		ws.Close(websocket.StatusInternalError, "internal error")
		return auth.Claims{}, "", false
	}
	if claims.Version < version {
		g.handshake(metrics.HandshakeTokenRevoked)
		g.kick(ws, protocol.KickBanned)
		return auth.Claims{}, "", false
	}
	if errors.Is(verr, auth.ErrExpiredToken) {
		g.handshake(metrics.HandshakeTokenExpired)
		g.kick(ws, protocol.KickTokenExpired)
		return auth.Claims{}, "", false
	}
	return claims, in.ID, true
}

// kick 下发 kicked 事件，**写完后**再用对应关闭码关闭连接（PRD F-AUTH-03 规则 3）。
// 不走异步发送队列：先入队再关闭的话，kicked 帧可能还没发出去连接就断了。
func (g *Gateway) kick(ws *websocket.Conn, reason string) {
	g.metrics.Kicks.WithLabelValues(reason).Inc()
	b, _ := json.Marshal(protocol.Event(protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason}))
	ctx, cancel := context.WithTimeout(context.Background(), g.cfg.WriteTimeout)
	defer cancel()
	_ = ws.Write(ctx, websocket.MessageText, b) // coder/websocket 的 Write 可以并发调用
	ws.Close(websocket.StatusCode(protocol.KickCloseCode[reason]), reason)
}

// conn 是一个 WebSocket 连接，实现 hub.Client。
type conn struct {
	id       string
	uid      string
	platform string
	ttl      time.Duration // 鉴权时 token 的剩余有效期
	ws       *websocket.Conn
	done     chan struct{} // 关闭后 Send 一律返回 false

	// 待发送队列。按需分配、按需启动写协程，写空后协程退出：
	// 1 万条空闲连接如果各自预分配 256 格的 channel、常驻一个写协程，要多占约 60MB 堆和约 80MB 栈（Q5a 内存分析）。
	qmu      sync.Mutex
	queue    [][]byte
	writing  bool // 是否有写协程在运行；同一时刻最多一个，保证帧的顺序
	kickOnce sync.Once
	closed   atomic.Pointer[closeReason] // 服务端主动关闭的原因；为空表示是客户端关的
	gw       *Gateway
}

// closeReason 是连接结束的原因（用于日志和指标）。
type closeReason struct {
	code   int
	reason string
	by     string // "server" 或 "client"
}

// closeWith 由服务端主动关闭连接，并记下原因（只记第一次）。
func (c *conn) closeWith(code int, reason string) {
	c.closed.CompareAndSwap(nil, &closeReason{code: code, reason: reason, by: "server"})
	c.ws.Close(websocket.StatusCode(code), reason)
}

func (c *conn) ConnID() string { return c.id }
func (c *conn) UserID() string { return c.uid }

// Kick 踢下线（异步，不阻塞调用方），只生效一次。
func (c *conn) Kick(reason string) {
	c.kickOnce.Do(func() {
		c.closed.CompareAndSwap(nil, &closeReason{code: protocol.KickCloseCode[reason], reason: "kicked: " + reason, by: "server"})
		go c.gw.kick(c.ws, reason)
	})
}

// Send 非阻塞入队。积压超过 SendBuffer 说明客户端读得太慢，直接断开它，由客户端重连后 sync 补齐。
func (c *conn) Send(frame []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	c.qmu.Lock()
	if len(c.queue) >= c.gw.cfg.SendBuffer {
		c.qmu.Unlock()
		c.gw.metrics.SlowConsumers.Inc()
		go c.closeWith(protocol.CloseSlowConsumer, "slow consumer")
		return false
	}
	c.queue = append(c.queue, frame)
	start := !c.writing
	c.writing = true
	c.qmu.Unlock()
	if start {
		go c.drain()
	}
	return true
}

// drain 把队列写完后退出。写失败（超时、连接已断）时关闭连接，让读循环退出，统一在 run 里收尾；
// 此时 writing 保持 true，不会再启动新的写协程。
func (c *conn) drain() {
	for {
		c.qmu.Lock()
		batch := c.queue
		c.queue = nil // 释放底层数组，空闲时不占内存
		if len(batch) == 0 {
			c.writing = false
			c.qmu.Unlock()
			return
		}
		c.qmu.Unlock()
		for _, b := range batch {
			ctx, cancel := context.WithTimeout(context.Background(), c.gw.cfg.WriteTimeout)
			err := c.ws.Write(ctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				c.ws.CloseNow()
				return
			}
		}
	}
}

// run 阻塞直到连接结束，返回结束原因（用于日志和指标）。
func (c *conn) run(parent context.Context) closeReason {
	ctx, cancel := context.WithCancel(parent)
	defer func() {
		close(c.done)
		cancel()
		c.ws.CloseNow()
	}()

	// 空闲计时：IdleTimeout 内收不到任何帧（包括 ping）就以 4408 关闭。
	// 不用 Read 的 ctx 超时：coder/websocket 在 ctx 到期时会直接断开底层连接，
	// 客户端只能看到异常断开（1006），拿不到 4408。
	idle := time.AfterFunc(c.gw.cfg.IdleTimeout, func() {
		c.closeWith(protocol.CloseIdleTimeout, "idle timeout")
	})
	defer idle.Stop()

	// token 到期时踢下线（PRD F-AUTH-03 规则 1）。剩余时间在鉴权时按注入的时钟算好。
	expiry := time.AfterFunc(c.ttl, func() { c.Kick(protocol.KickTokenExpired) })
	defer expiry.Stop()

	for {
		typ, data, err := c.ws.Read(ctx)
		if err != nil {
			if cr := c.closed.Load(); cr != nil {
				return *cr
			}
			code := int(websocket.CloseStatus(err))
			if code < 0 { // 没有收到关闭帧：断网、进程被杀等
				return closeReason{code: int(websocket.StatusAbnormalClosure), reason: "abnormal: " + err.Error(), by: "client"}
			}
			return closeReason{code: code, reason: code2name(code), by: "client"}
		}
		idle.Reset(c.gw.cfg.IdleTimeout)
		if typ != websocket.MessageText {
			c.write(protocol.AckErr("", protocol.Errorf(protocol.CodeInvalidFrame, "only text frames are supported")))
			continue
		}
		c.handle(ctx, data)
	}
}

func (c *conn) handle(ctx context.Context, data []byte) {
	var in protocol.InFrame
	if err := json.Unmarshal(data, &in); err != nil || in.Cmd == "" {
		c.write(protocol.AckErr(in.ID, protocol.Errorf(protocol.CodeInvalidFrame, "invalid frame")))
		return
	}
	switch in.Cmd {
	case protocol.CmdPing:
		c.write(protocol.OutFrame{Cmd: protocol.CmdPong})
		return
	case protocol.CmdAuth:
		c.write(protocol.AckErr(in.ID, protocol.Errorf(protocol.CodeBadRequest, "already authenticated")))
		return
	}
	start := time.Now()
	out := c.gw.handler.Handle(ctx, c, in)
	cmd := metrics.CmdLabel(in.Cmd)
	c.gw.metrics.CommandDuration.WithLabelValues(cmd).Observe(time.Since(start).Seconds())
	result := 0
	if out != nil && out.Error != nil {
		result = out.Error.Code
	}
	c.gw.metrics.Commands.WithLabelValues(cmd, metrics.Code(result)).Inc()
	if out != nil {
		c.write(*out)
	}
}

func (c *conn) write(f protocol.OutFrame) {
	b, err := json.Marshal(f)
	if err != nil {
		c.gw.log.Error("marshal frame", "err", err, "cmd", f.Cmd)
		return
	}
	c.Send(b)
}

// code2name 返回关闭码的名称（用于日志）；不认识的返回空字符串。
func code2name(code int) string {
	return protocol.CloseCodeName(code)
}
