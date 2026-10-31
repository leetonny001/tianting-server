// Package gateway 负责 WebSocket 长连接：鉴权、读写循环、心跳超时、慢连接保护。
// 业务处理交给 Handler，网关本身不理解 send / sync 等命令。
package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/auth"
	"im-mvp/server/internal/hub"
	"im-mvp/server/internal/protocol"
)

// Handler 处理一条已解析的上行帧（ping 除外），返回要回给客户端的 ack；nil 表示不回复。
type Handler interface {
	Handle(ctx context.Context, c hub.Client, in protocol.InFrame) *protocol.OutFrame
}

// TokenVerifier 校验 token。
type TokenVerifier interface {
	Verify(token string) (auth.Claims, error)
}

// Config 是网关参数。
type Config struct {
	IdleTimeout  time.Duration // 多久收不到任何帧就断开（协议约定 90s）
	WriteTimeout time.Duration // 单帧写超时
	SendBuffer   int           // 每个连接的待发送队列长度，满了视为慢连接并断开
	ReadLimit    int64         // 单帧最大字节数
}

// DefaultConfig 是生产默认值。
func DefaultConfig() Config {
	return Config{IdleTimeout: 90 * time.Second, WriteTimeout: 10 * time.Second, SendBuffer: 256, ReadLimit: 64 << 10}
}

var platforms = map[string]bool{"android": true, "ios": true, "web": true, "pc": true}

// Gateway 是 /ws 的 http.Handler。
type Gateway struct {
	cfg      Config
	verifier TokenVerifier
	hub      *hub.Hub
	handler  Handler
	log      *slog.Logger
	nextID   atomic.Uint64
}

func New(cfg Config, verifier TokenVerifier, h *hub.Hub, handler Handler, log *slog.Logger) *Gateway {
	return &Gateway{cfg: cfg, verifier: verifier, hub: h, handler: handler, log: log}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	claims, err := g.verifier.Verify(token)
	uid := claims.UserID
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(protocol.Errorf(protocol.CodeTokenInvalid, "invalid token"))
		return
	}
	platform := r.URL.Query().Get("platform")
	if !platforms[platform] {
		platform = "unknown"
	}

	// MVP 先不校验 Origin，方便 H5 本地调试；上线前要改成 OriginPatterns 白名单。
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		g.log.Warn("websocket accept failed", "err", err)
		return
	}
	ws.SetReadLimit(g.cfg.ReadLimit)

	c := &conn{
		id:       g.nextID.Add(1),
		uid:      uid,
		platform: platform,
		ws:       ws,
		send:     make(chan []byte, g.cfg.SendBuffer),
		done:     make(chan struct{}),
		gw:       g,
	}
	g.hub.Register(c)
	g.log.Info("conn open", "conn", c.id, "uid", uid, "platform", platform)

	reason := c.run(context.Background())

	g.hub.Unregister(c)
	g.log.Info("conn closed", "conn", c.id, "uid", uid, "reason", reason)
}

// conn 是一个 WebSocket 连接，实现 hub.Client。
type conn struct {
	id       uint64
	uid      string
	platform string
	ws       *websocket.Conn
	send     chan []byte
	done     chan struct{} // 关闭后 Send 一律返回 false
	gw       *Gateway
}

func (c *conn) ConnID() uint64 { return c.id }
func (c *conn) UserID() string { return c.uid }

// Send 非阻塞入队。队列满说明客户端读得太慢，直接断开它，由客户端重连后 sync 补齐。
func (c *conn) Send(frame []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- frame:
		return true
	case <-c.done:
		return false
	default:
		go c.ws.Close(protocol.CloseSlowConsumer, "slow consumer")
		return false
	}
}

// run 阻塞直到连接结束，返回结束原因（用于日志）。
func (c *conn) run(parent context.Context) string {
	ctx, cancel := context.WithCancel(parent)
	defer func() {
		close(c.done)
		cancel()
		c.ws.CloseNow()
	}()
	go c.writeLoop(ctx)

	// 空闲计时：IdleTimeout 内收不到任何帧（包括 ping）就以 4408 关闭。
	// 不用 Read 的 ctx 超时：coder/websocket 在 ctx 到期时会直接断开底层连接，
	// 客户端只能看到异常断开（1006），拿不到 4408。
	var idleFired atomic.Bool
	idle := time.AfterFunc(c.gw.cfg.IdleTimeout, func() {
		idleFired.Store(true)
		c.ws.Close(protocol.CloseIdleTimeout, "idle timeout")
	})
	defer idle.Stop()

	for {
		typ, data, err := c.ws.Read(ctx)
		if err != nil {
			if idleFired.Load() {
				return "idle timeout"
			}
			return websocket.CloseStatus(err).String()
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
	if in.Cmd == protocol.CmdPing {
		c.write(protocol.OutFrame{Cmd: protocol.CmdPong})
		return
	}
	if out := c.gw.handler.Handle(ctx, c, in); out != nil {
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

func (c *conn) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, c.gw.cfg.WriteTimeout)
			err := c.ws.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				c.ws.CloseNow() // 让读循环退出，统一在 run 里收尾
				return
			}
		}
	}
}
