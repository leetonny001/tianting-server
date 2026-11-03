package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/protocol"
)

// frame 是解析下行帧用的宽松结构。
type frame struct {
	ID    string          `json:"id"`
	Cmd   string          `json:"cmd"`
	Data  json.RawMessage `json:"data"`
	Error *protocol.Error `json:"error"`
}

// client 是压测用的一条 WebSocket 连接：首帧 auth、按 id 匹配 ack、定时 ping。
type client struct {
	uid string
	ws  *websocket.Conn

	mu      sync.Mutex
	pending map[string]chan frame

	reqN    atomic.Int64
	onMsg   atomic.Pointer[func(protocol.Message, time.Time)]
	done    chan struct{}
	closeMu sync.Mutex
	closeBy string // 连接结束的原因（"client" 表示压测主动关闭）
}

var errClosed = errors.New("connection closed")

// dial 建立连接并完成首帧 auth（PRD C-52）。
func dial(ctx context.Context, wsURL, uid, token string) (*client, error) {
	ws, _, err := websocket.Dial(ctx, wsURL+"?platform=web", nil)
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(64 << 20) // 下行不限帧大小（C-07）
	c := &client{uid: uid, ws: ws, pending: map[string]chan frame{}, done: make(chan struct{})}
	b, _ := json.Marshal(map[string]any{"id": "auth", "cmd": protocol.CmdAuth, "data": map[string]string{"token": token}})
	if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
		ws.CloseNow()
		return nil, err
	}
	_, data, err := ws.Read(ctx)
	if err != nil {
		ws.CloseNow()
		return nil, fmt.Errorf("auth: %w", err)
	}
	var f frame
	if json.Unmarshal(data, &f) != nil || f.ID != "auth" || f.Error != nil {
		ws.CloseNow()
		return nil, fmt.Errorf("auth rejected: %s", data)
	}
	// 读循环和心跳的生命周期是整条连接，不能跟着 dial 的 ctx（建连超时）结束。
	go c.readLoop() // #nosec G118 -- connection-scoped goroutine, outlives the dial context
	go c.pingLoop() // #nosec G118 -- connection-scoped goroutine, outlives the dial context
	return c, nil
}

func (c *client) readLoop() {
	defer close(c.done)
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			c.closeMu.Lock()
			if c.closeBy == "" {
				c.closeBy = fmt.Sprintf("server/network: %d %v", websocket.CloseStatus(err), err)
			}
			c.closeMu.Unlock()
			c.mu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
		now := time.Now()
		var f frame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Cmd {
		case protocol.CmdAck:
			c.mu.Lock()
			ch := c.pending[f.ID]
			delete(c.pending, f.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- f
			}
		case protocol.CmdMsg:
			if fn := c.onMsg.Load(); fn != nil {
				var m protocol.Message
				if json.Unmarshal(f.Data, &m) == nil {
					(*fn)(m, now)
				}
			}
		}
	}
}

// pingLoop 按客户端默认间隔（30 秒）发心跳，服务端 90 秒收不到任何帧会断开。
func (c *client) pingLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = c.ws.Write(ctx, websocket.MessageText, []byte(`{"cmd":"ping"}`))
			cancel()
		}
	}
}

// call 发送请求并等待 ack。
func (c *client) call(ctx context.Context, cmd string, data any) (frame, error) {
	id := "r" + strconv.FormatInt(c.reqN.Add(1), 10)
	ch := make(chan frame, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	b, _ := json.Marshal(map[string]any{"id": id, "cmd": cmd, "data": data})
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return frame{}, err
	}
	select {
	case f, ok := <-ch:
		if !ok {
			return frame{}, errClosed
		}
		return f, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return frame{}, ctx.Err()
	}
}

// sendText 发一条文本消息，正文里带上发送时刻（纳秒），接收方据此计算端到端延迟。
func (c *client) sendText(ctx context.Context, convType, to, clientMsgID string, pad int) (protocol.SendAck, time.Duration, error) {
	text := stampText(time.Now(), pad)
	start := time.Now()
	f, err := c.call(ctx, protocol.CmdSend, map[string]any{
		"convType": convType, "to": to, "clientMsgId": clientMsgID, "type": "text", "body": map[string]string{"text": text},
	})
	lat := time.Since(start)
	if err != nil {
		return protocol.SendAck{}, lat, err
	}
	if f.Error != nil {
		return protocol.SendAck{}, lat, f.Error
	}
	var a protocol.SendAck
	err = json.Unmarshal(f.Data, &a)
	return a, lat, err
}

func (c *client) syncAfter(ctx context.Context, convID string, after int64, limit int) (protocol.SyncAck, error) {
	return c.syncReq(ctx, map[string]any{"convId": convID, "afterSeq": after, "limit": limit})
}

func (c *client) syncReq(ctx context.Context, req map[string]any) (protocol.SyncAck, error) {
	f, err := c.call(ctx, protocol.CmdSync, req)
	if err != nil {
		return protocol.SyncAck{}, err
	}
	if f.Error != nil {
		return protocol.SyncAck{}, f.Error
	}
	var a protocol.SyncAck
	err = json.Unmarshal(f.Data, &a)
	return a, err
}

// syncAll 拉取会话的全部消息（按 200 条一页）。
func (c *client) syncAll(ctx context.Context, convID string) ([]protocol.Message, error) {
	var out []protocol.Message
	var after int64
	for {
		a, err := c.syncAfter(ctx, convID, after, 200)
		if err != nil {
			return nil, err
		}
		out = append(out, a.Messages...)
		if !a.HasMore || len(a.Messages) == 0 {
			return out, nil
		}
		after = a.Messages[len(a.Messages)-1].Seq
	}
}

func (c *client) close() {
	c.closeMu.Lock()
	if c.closeBy == "" {
		c.closeBy = "client"
	}
	c.closeMu.Unlock()
	c.ws.Close(websocket.StatusNormalClosure, "")
}

// unexpectedClose 返回非压测主动关闭的原因；连接仍然在线时返回空字符串。
func (c *client) unexpectedClose() string {
	select {
	case <-c.done:
	default:
		return ""
	}
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closeBy == "client" {
		return ""
	}
	return c.closeBy
}

const stampPrefix = "lt:"

func stampText(t time.Time, pad int) string {
	s := stampPrefix + strconv.FormatInt(t.UnixNano(), 10) + ":"
	if pad > len(s) {
		s += strings.Repeat("x", pad-len(s))
	}
	return s
}

// latencyOf 从消息正文里取出发送时刻，返回端到端延迟。
func latencyOf(text string, recvAt time.Time) (time.Duration, bool) {
	rest, ok := strings.CutPrefix(text, stampPrefix)
	if !ok {
		return 0, false
	}
	ns, _, _ := strings.Cut(rest, ":")
	v, err := strconv.ParseInt(ns, 10, 64)
	if err != nil {
		return 0, false
	}
	return recvAt.Sub(time.Unix(0, v)), true
}
