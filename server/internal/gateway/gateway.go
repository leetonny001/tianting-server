// Package gateway 负责 WebSocket 长连接：鉴权、读写循环、心跳超时、慢连接保护。
// 业务处理交给 Handler，网关本身不理解 send / sync 等命令。
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/auth"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/disconnects"
	"tianting/server/internal/hub"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/sessions"
)

// Handler 处理一条已解析的上行帧（ping 除外），返回要回给客户端的 ack；nil 表示不回复。
type Handler interface {
	Handle(ctx context.Context, s delivery.Session, in protocol.InFrame) *protocol.OutFrame
	// AuthMutedUntil 是 auth ack 的 mutedUntil（第二批）：没有禁言时返回 nil，永久禁言是 null。
	AuthMutedUntil(ctx context.Context, uid string) json.RawMessage
}

// TokenVerifier 校验 token。
type TokenVerifier = auth.TokenVerifier

// RevocationChecker 返回用户当前的令牌版本号；token 里的版本号更小，说明是强制下线前签发的。
type RevocationChecker interface {
	auth.Checker
	RecoveryUnacked(ctx context.Context, userID string) (bool, error)
	// ActiveBan 区分"被强制下线"和"被封禁"：两者都让旧 token 失效，给客户端的原因不同（F-ADM-13）。
	ActiveBan(ctx context.Context, userID string, now int64) (banned bool, until *int64, err error)
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
	disconnects *disconnects.Buffer
	cfg         Config
	verifier    TokenVerifier
	revocation  RevocationChecker
	clock       clock.Clock
	hub         *hub.Hub
	handler     Handler
	log         *slog.Logger
	metrics     *metrics.Metrics
	guard       *guard
	nextID      atomic.Uint64
}

func New(cfg Config, verifier TokenVerifier, revocation RevocationChecker, clk clock.Clock, h *hub.Hub, handler Handler, m *metrics.Metrics, log *slog.Logger) *Gateway {
	return &Gateway{cfg: cfg, verifier: verifier, revocation: revocation, clock: clk, hub: h, handler: handler, metrics: m, log: log, guard: newGuard(cfg.Guard, clk), disconnects: disconnects.New()}
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
	maskedIP := sessions.MaskIP(ip)
	if !g.guard.allowHandshake(ip) {
		g.handshake(metrics.HandshakeRateLimited)
		http.Error(w, "too many connection attempts", http.StatusTooManyRequests)
		g.log.Info("conn rejected", "phase", disconnects.PhaseHandshake, "reason", "handshake rate limited", "ip", maskedIP)
		return
	}
	ws, err := websocket.Accept(w, r, g.acceptOptions())
	if err != nil {
		// 库只在 Origin 不在白名单时用这句话（有 e2e 守护：升级依赖时文案变了会被发现）。
		if strings.Contains(err.Error(), "not authorized") {
			g.handshake(metrics.HandshakeOriginDenied)
			g.log.Info("conn rejected", "phase", disconnects.PhaseHandshake, "reason", "origin not allowed", "ip", maskedIP)
			return
		}
		g.log.Warn("websocket accept failed", "err", err)
		return
	}
	ws.SetReadLimit(g.cfg.ReadLimit)

	platform := r.URL.Query().Get("platform")
	if !platforms[platform] {
		g.handshake(metrics.HandshakeBadPlatform)
		ws.Close(protocol.CloseInvalidPlatform, "invalid platform")
		// 平台来自请求参数，不进日志原样输出（可能很长或带控制字符）。
		g.handshakeRejected(handshakeReject{code: protocol.CloseInvalidPlatform, reason: "invalid platform", platform: "unknown", ip: maskedIP})
		return
	}
	claims, authID, ok := g.authenticate(r.Context(), ws, ip, maskedIP, platform)
	if !ok {
		return
	}
	uid := claims.UserID

	c := &conn{
		id:          g.cfg.NodeID + "-" + strconv.FormatUint(g.nextID.Add(1), 10),
		uid:         uid,
		claims:      claims,
		platform:    platform,
		connectedAt: g.clock.Now().UnixMilli(),
		maskedIP:    maskedIP,
		// 剩余有效期在鉴权时按注入的时钟算好：auth 的 ack 发出之后测试可能马上改时钟，
		// 如果等 run 开始时再算，就会和改时钟赛跑（race 检测下变慢时会被立即踢下线）。
		ttl:  claims.ExpiresAt.Sub(g.clock.Now()),
		ws:   ws,
		done: make(chan struct{}),
		gw:   g,
	}
	// auth 的 ack 先入队、再登记到在线表：保证客户端先收到 ack，再收到任何推送。
	ack := protocol.AuthAck{UserID: uid, ServerTime: g.clock.Now().UnixMilli(), MutedUntil: g.handler.AuthMutedUntil(r.Context(), uid)}
	if claims.SID != "" {
		// PRD-chat F-ACC-05 规则 1：App 据此强制进保存页。查询失败不影响登录，下次连接再带。
		if unacked, err := g.revocation.RecoveryUnacked(r.Context(), uid); err != nil {
			g.log.Warn("recovery code state", "err", err, "uid", uid)
		} else {
			ack.RecoveryCodeUnacked = unacked
		}
	}
	c.write(protocol.AckOK(authID, ack))
	for _, old := range g.hub.Register(c, g.cfg.MaxConnsPerUser) {
		old.Kick(protocol.KickTooManyConns)
		g.log.Info("conn evicted", "conn", old.ConnID(), "uid", uid, "reason", protocol.KickTooManyConns)
	}
	// 校验版本号和进入在线表之间可能刚好发生封禁或强制下线：那次踢人踢不到这个连接，这里再查一次，不留漏网的连接。
	if v, err := g.revocation.TokenVersion(r.Context(), uid); err == nil && claims.Version < v {
		g.kickRevoked(r.Context(), c, uid)
	}
	g.handshake(metrics.HandshakeOK)
	g.metrics.Connections.WithLabelValues(platform).Inc()
	g.log.Info("conn open", "conn", c.id, "uid", uid, "platform", platform)
	opened := time.Now()

	cr := c.run(context.Background())

	g.hub.Unregister(c)
	g.metrics.Connections.WithLabelValues(platform).Dec()
	g.metrics.Disconnects.WithLabelValues(metrics.Code(cr.code), cr.by, disconnects.PhaseSession).Inc()
	now := g.clock.Now().UnixMilli()
	g.disconnects.Add(disconnects.Record{
		UserID: uid, ConnID: c.id, Phase: disconnects.PhaseSession, Platform: platform, Code: cr.code,
		Reason: strings.TrimPrefix(cr.reason, kickedPrefix), By: cr.by, TS: now, DurationMs: max(now-c.connectedAt, 0), IP: maskedIP,
	})
	// 记录关闭码、原因和关闭方，用于排查"为什么掉线"（BE-051、AC-AUTH-05-1）。
	g.log.Info("conn closed", "conn", c.id, "uid", uid, "platform", platform, "phase", disconnects.PhaseSession, "ip", maskedIP,
		"code", cr.code, "reason", cr.reason, "by", cr.by, "duration", time.Since(opened).Round(time.Millisecond))
}

// Disconnects 返回断开记录缓冲（运维接口读取）。
func (g *Gateway) Disconnects() *disconnects.Buffer { return g.disconnects }

const kickedPrefix = "kicked: "

// handshakeReject 是一次握手阶段的拒绝。uid 只在 token 签名有效时填写（4011、4012、1011）：
// 4010 的 token 里自称的 userId 不可信，不进日志也不写断开记录（PRD-ops F-OPS-09 规则 3）。
type handshakeReject struct {
	code     int
	reason   string
	platform string
	ip       string
	uid      string
}

func (g *Gateway) handshakeRejected(r handshakeReject) {
	g.metrics.Disconnects.WithLabelValues(metrics.Code(r.code), "server", disconnects.PhaseHandshake).Inc()
	attrs := []any{"code", r.code, "phase", disconnects.PhaseHandshake, "reason", r.reason, "platform", r.platform, "ip", r.ip}
	if r.uid != "" {
		attrs = append(attrs, "uid", r.uid)
		g.disconnects.Add(disconnects.Record{
			UserID: r.uid, Phase: disconnects.PhaseHandshake, Platform: r.platform, Code: r.code, Reason: r.reason,
			By: "server", TS: g.clock.Now().UnixMilli(), IP: r.ip,
		})
	}
	g.log.Info("conn rejected", attrs...)
}

func (g *Gateway) handshake(result string) { g.metrics.Handshakes.WithLabelValues(result).Inc() }

func (g *Gateway) acceptOptions() *websocket.AcceptOptions {
	if slices.Contains(g.cfg.AllowedOrigins, "*") {
		return &websocket.AcceptOptions{InsecureSkipVerify: true}
	}
	return &websocket.AcceptOptions{OriginPatterns: g.cfg.AllowedOrigins}
}

// authenticate 等待并校验第一帧 auth。失败时已经关闭连接，返回 ok=false。
func (g *Gateway) authenticate(ctx context.Context, ws *websocket.Conn, ip, maskedIP, platform string) (claims auth.Claims, reqID string, ok bool) {
	reject := func(code int, reason, uid string) {
		g.handshakeRejected(handshakeReject{code: code, reason: reason, platform: platform, ip: maskedIP, uid: uid})
	}
	// 用定时器而不是 Read 的 ctx 超时：ctx 到期时库会直接断开底层连接，客户端拿不到 4401（同空闲超时）。
	timer := time.AfterFunc(g.cfg.AuthTimeout, func() {
		ws.Close(protocol.CloseAuthRequired, "auth timeout")
	})
	typ, data, err := ws.Read(ctx)
	timedOut := !timer.Stop()
	if timedOut || err != nil {
		g.handshake(metrics.HandshakeAuthRequired)
		if timedOut {
			reject(protocol.CloseAuthRequired, "no auth frame", "")
		} else { // 客户端在发 auth 之前就断开了：不是服务端关的，不计入断开指标
			g.log.Info("conn rejected", "phase", disconnects.PhaseHandshake, "reason", "client left before auth", "platform", platform, "ip", maskedIP)
		}
		return auth.Claims{}, "", false
	}
	var in protocol.InFrame
	if typ != websocket.MessageText || json.Unmarshal(data, &in) != nil || in.Cmd != protocol.CmdAuth || in.ID == "" {
		g.handshake(metrics.HandshakeAuthRequired)
		ws.Close(protocol.CloseAuthRequired, "first frame must be auth")
		reject(protocol.CloseAuthRequired, "first frame is not auth", "")
		return auth.Claims{}, "", false
	}
	var req protocol.AuthReq
	_ = json.Unmarshal(in.Data, &req) // 解析失败时 token 为空，按无效凭证处理

	// 判定顺序和 /v1/* 的 Bearer 共用一个函数（PRD-chat 第 4.2 节第 3 条）。
	res := auth.Check(ctx, g.verifier, g.revocation, req.Token, ip)
	claims = res.Claims
	switch res.Verdict {
	case auth.VerdictOK:
		return claims, in.ID, true
	case auth.VerdictInvalid:
		// 只统计"凭证无效"：过期和被强制下线是正常用户也会遇到的情况，不算攻击。
		if g.guard.authFailed(ip) {
			g.log.Warn("ip blocked", "reason", "too many invalid tokens", "ip", maskedIP, "for", g.cfg.Guard.BlockFor)
		}
		g.handshake(metrics.HandshakeTokenInvalid)
		g.kick(ws, protocol.KickTokenInvalid)
		reject(protocol.CloseTokenInvalid, protocol.KickTokenInvalid, "")
	case auth.VerdictError:
		g.log.Error("check revocation", "err", res.Err, "uid", claims.UserID)
		g.handshake(metrics.HandshakeError)
		ws.Close(websocket.StatusInternalError, "internal error")
		reject(protocol.CloseInternalError, "revocation check failed", claims.UserID)
	case auth.VerdictRevoked:
		g.handshake(metrics.HandshakeTokenRevoked)
		ev := g.revokedEvent(ctx, claims.UserID)
		g.kickEvent(ws, ev)
		reject(protocol.CloseTokenRevoked, ev.Reason, claims.UserID)
	case auth.VerdictSessionRevoked:
		// recoverBefore 随 BE-416（F-ACC-03 规则 6a）。
		kick := protocol.SessionKickReason(res.RevokeReason)
		g.handshake(metrics.HandshakeTokenRevoked)
		g.kick(ws, kick)
		reject(protocol.CloseTokenRevoked, kick, claims.UserID)
	case auth.VerdictAccountDeleted, auth.VerdictCredentialsOld:
		reason := protocol.KickAccountDeleted
		if res.Verdict == auth.VerdictCredentialsOld {
			reason = protocol.KickPasswordChanged
		}
		g.handshake(metrics.HandshakeTokenRevoked)
		g.kick(ws, reason)
		reject(protocol.CloseTokenRevoked, reason, claims.UserID)
	default: // 过期、被替换的代：都是 kicked/tokenExpired（F-ACC-03 规则 4）
		g.handshake(metrics.HandshakeTokenExpired)
		g.kick(ws, protocol.KickTokenExpired)
		reject(protocol.CloseTokenExpired, protocol.KickTokenExpired, claims.UserID)
	}
	return auth.Claims{}, "", false
}

// kick 下发 kicked 事件，**写完后**再用对应关闭码关闭连接（PRD F-AUTH-03 规则 3）。
// 不走异步发送队列：先入队再关闭的话，kicked 帧可能还没发出去连接就断了。
func (g *Gateway) kick(ws *websocket.Conn, reason string) {
	g.kickEvent(ws, protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason})
}

func (g *Gateway) kickEvent(ws *websocket.Conn, ev protocol.KickedEvent) {
	g.metrics.Kicks.WithLabelValues(ev.Reason).Inc()
	b, _ := json.Marshal(protocol.Event(ev))
	ctx, cancel := context.WithTimeout(context.Background(), g.cfg.WriteTimeout)
	defer cancel()
	_ = ws.Write(ctx, websocket.MessageText, b) // coder/websocket 的 Write 可以并发调用
	ws.Close(websocket.StatusCode(protocol.KickCloseCode[ev.Reason]), ev.Reason)
}

// revokedEvent：旧 token 失效的原因。正在封禁中是 suspended（带 until），否则是强制下线 banned。查询失败按 banned，反正都要断开。
func (g *Gateway) revokedEvent(ctx context.Context, uid string) protocol.KickedEvent {
	banned, until, err := g.revocation.ActiveBan(ctx, uid, g.clock.Now().UnixMilli())
	if err != nil {
		g.log.Error("check ban", "err", err, "uid", uid)
	}
	if !banned {
		return protocol.KickedEvent{Type: protocol.EventKicked, Reason: protocol.KickBanned}
	}
	return protocol.KickedEvent{Type: protocol.EventKicked, Reason: protocol.KickSuspended, Until: protocol.NullableMs(until)}
}

// kickRevoked 踢掉已经登记的连接，原因同 revokedEvent。
func (g *Gateway) kickRevoked(ctx context.Context, c *conn, uid string) {
	ev := g.revokedEvent(ctx, uid)
	if ev.Reason == protocol.KickSuspended {
		var until *int64
		_ = json.Unmarshal(ev.Until, &until)
		c.Suspend(until)
		return
	}
	c.Kick(ev.Reason)
}

// conn 是一个 WebSocket 连接，实现 hub.Client。
type conn struct {
	id       string
	uid      string
	platform string
	claims   auth.Claims // 这条连接用的 token：复查和按 sid、代号踢线都用它
	// 连接元数据（PRD-admin F-ADM-14）。maskedIP 是脱敏后的值：结构体里不保存完整 IP。
	connectedAt int64
	maskedIP    string
	ttl         time.Duration // 鉴权时 token 的剩余有效期
	ws          *websocket.Conn
	done        chan struct{} // 关闭后 Send 一律返回 false

	// 待发送队列。按需分配、按需启动写协程，写空后协程退出：
	// 1 万条空闲连接如果各自预分配 256 格的 channel、常驻一个写协程，要多占约 60MB 堆和约 80MB 栈（Q5a 内存分析）。
	qmu      sync.Mutex
	queue    [][]byte
	writing  bool // 是否有写协程在运行；同一时刻最多一个，保证帧的顺序
	kickOnce sync.Once
	closed   atomic.Pointer[closeReason] // 服务端主动关闭的原因；为空表示是客户端关的
	gw       *Gateway
}

// SessionInfo 实现 hub.Client。
func (c *conn) SessionInfo() sessions.Conn {
	c.qmu.Lock()
	queued := len(c.queue)
	c.qmu.Unlock()
	return sessions.Conn{
		ConnID: c.id, Platform: c.platform, ConnectedAt: c.connectedAt, Node: c.gw.cfg.NodeID, IP: c.maskedIP, QueueLen: queued,
	}
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

func (c *conn) ConnID() string   { return c.id }
func (c *conn) Platform() string { return c.platform }
func (c *conn) UserID() string   { return c.uid }
func (c *conn) SID() string      { return c.claims.SID }
func (c *conn) Gen() int64       { return c.claims.Gen }

// Kick 踢下线（异步，不阻塞调用方），只生效一次。
func (c *conn) Kick(reason string) {
	c.kickOnce.Do(func() {
		c.closed.CompareAndSwap(nil, &closeReason{code: protocol.KickCloseCode[reason], reason: kickedPrefix + reason, by: "server"})
		go c.gw.kick(c.ws, reason)
	})
}

// KickWith 带 recoverBefore 踢下线（PRD-chat F-ACC-03 规则 6a，异步，只生效一次）。
func (c *conn) KickWith(reason string, recoverBefore *int64) {
	c.kickOnce.Do(func() {
		c.closed.CompareAndSwap(nil, &closeReason{code: protocol.KickCloseCode[reason], reason: kickedPrefix + reason, by: "server"})
		go c.gw.kickEvent(c.ws, protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason, RecoverBefore: recoverBefore})
	})
}

// Suspend 封禁时踢下线（异步，只生效一次）。
func (c *conn) Suspend(until *int64) {
	c.kickOnce.Do(func() {
		reason := protocol.KickSuspended
		c.closed.CompareAndSwap(nil, &closeReason{code: protocol.KickCloseCode[reason], reason: kickedPrefix + reason, by: "server"})
		go c.gw.kickEvent(c.ws, protocol.KickedEvent{Type: protocol.EventKicked, Reason: reason, Until: protocol.NullableMs(until)})
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
				return closeReason{code: int(websocket.StatusAbnormalClosure), reason: "abnormal", by: "client"} // 不带 err 文本：里面有对端完整地址（PRD-ops S-OPS-09）
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
