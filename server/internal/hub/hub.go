// Package hub 维护本节点的在线连接表：用户 → 该用户的所有连接（多端同时在线）。
// Hub 同时是 delivery.Router 的单节点实现。
package hub

import (
	"sync"
	"sync/atomic"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/sessions"
)

// Client 是一个在线连接。Send、Kick 都不能阻塞。
type Client interface {
	delivery.Session
	Send(frame []byte) bool // 发不出去时返回 false
	Kick(reason string)     // 下发 kicked 并关闭连接
	// KickWith 下发带 recoverBefore 的 kicked 并关闭连接（PRD-chat F-ACC-03 规则 6a）。
	KickWith(reason string, recoverBefore *int64)
	Suspend(until *int64) // 下发 kicked {reason: suspended, until} 并关闭连接
	Platform() string
	// SessionInfo 返回连接元数据（IP 已脱敏，PRD-admin F-ADM-14 规则 2）。
	SessionInfo() sessions.Conn
}

// Hub 是并发安全的在线表。
type Hub struct {
	mu    sync.RWMutex
	users map[string]map[string]entry
	// platforms 是按平台的连接数（运维接口读取，不遍历 users，PRD-ops S-OPS-13）。
	platforms map[string]int
	seq       uint64 // 登记顺序，用于找出最早的连接
	// dropKicks 是测试模式的故障注入（PRD-chat 第 4.5 节"丢弃吊销事件"）：吊销事件丢了，靠周期复查补上（RS4-09）。
	dropKicks atomic.Bool
}

type entry struct {
	c   Client
	seq uint64
}

func New() *Hub {
	return &Hub{users: make(map[string]map[string]entry), platforms: make(map[string]int)}
}

// Register 登记连接。用户的连接数超过 max（<= 0 表示不限）时，把最早的连接移出在线表并返回，
// 由调用方踢下线（ARCH S6）。被移出的连接不会再收到投递。
//
// 注意：这个上限按本节点计算；集群部署时要改为按在线路由（Presence）统计全部节点。
func (h *Hub) Register(c Client, max int) (evicted []Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.users[c.UserID()]
	if conns == nil {
		conns = make(map[string]entry)
		h.users[c.UserID()] = conns
	}
	h.seq++
	conns[c.ConnID()] = entry{c: c, seq: h.seq}
	h.platforms[c.Platform()]++
	for max > 0 && len(conns) > max {
		var oldest entry
		for _, e := range conns {
			if oldest.c == nil || e.seq < oldest.seq {
				oldest = e
			}
		}
		delete(conns, oldest.c.ConnID())
		h.decPlatform(oldest.c.Platform())
		evicted = append(evicted, oldest.c)
	}
	return evicted
}

// Unregister 移除连接；用户没有连接时从表里删掉。
func (h *Hub) Unregister(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.users[c.UserID()]
	if _, ok := conns[c.ConnID()]; ok { // 被挤掉的连接已经移除过，不能再减一次
		delete(conns, c.ConnID())
		h.decPlatform(c.Platform())
	}
	if len(conns) == 0 {
		delete(h.users, c.UserID())
	}
}

func (h *Hub) decPlatform(p string) {
	if h.platforms[p]--; h.platforms[p] <= 0 {
		delete(h.platforms, p)
	}
}

// PlatformCounts 返回本节点按平台的连接数。
func (h *Hub) PlatformCounts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int, len(h.platforms))
	for p, n := range h.platforms {
		out[p] = n
	}
	return out
}

var _ delivery.Router = (*Hub)(nil)

// DeliverToUser 实现 delivery.Router。
func (h *Hub) DeliverToUser(userID string, frame []byte, exceptConn string) int {
	return h.SendToUser(userID, frame, exceptConn)
}

// SetDropKicks 打开或关闭"丢弃吊销事件"，只有测试模式的路由会调用。
func (h *Hub) SetDropKicks(on bool) { h.dropKicks.Store(on) }

func (h *Hub) userClients(userID string, keep func(Client) bool) []Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	targets := make([]Client, 0, len(h.users[userID]))
	for _, e := range h.users[userID] {
		if keep(e.c) {
			targets = append(targets, e.c)
		}
	}
	return targets
}

// KickSession 实现 delivery.Router。
func (h *Hub) KickSession(userID, sid, reason string, recoverBefore *int64) {
	if h.dropKicks.Load() {
		return
	}
	for _, c := range h.userClients(userID, func(c Client) bool { return c.SID() == sid }) {
		c.KickWith(reason, recoverBefore)
	}
}

// KickStaleGen 实现 delivery.Router。
func (h *Hub) KickStaleGen(userID, sid string, currentGen int64) {
	if h.dropKicks.Load() {
		return
	}
	for _, c := range h.userClients(userID, func(c Client) bool { return c.SID() == sid && c.Gen() != currentGen }) {
		c.Kick(protocol.KickTokenExpired)
	}
}

// KickUserExcept 实现 delivery.Router。
func (h *Hub) KickUserExcept(userID, exceptSID, reason string, recoverBefore *int64) {
	if h.dropKicks.Load() {
		return
	}
	for _, c := range h.userClients(userID, func(c Client) bool { return exceptSID == "" || c.SID() != exceptSID }) {
		c.KickWith(reason, recoverBefore)
	}
}

// Clients 返回本节点全部连接的快照（周期复查用）。
func (h *Hub) Clients() []Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []Client
	for _, conns := range h.users {
		for _, e := range conns {
			out = append(out, e.c)
		}
	}
	return out
}

// SuspendUser 实现 delivery.Router。
func (h *Hub) SuspendUser(userID string, until *int64) {
	if h.dropKicks.Load() {
		return
	}
	h.mu.RLock()
	targets := make([]Client, 0, len(h.users[userID]))
	for _, e := range h.users[userID] {
		targets = append(targets, e.c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		c.Suspend(until)
	}
}

// KickUser 实现 delivery.Router。
func (h *Hub) KickUser(userID, reason string) {
	if h.dropKicks.Load() {
		return
	}
	h.KickLocal(userID, reason)
}

// SendToUser 把帧发给本节点上用户的所有连接，跳过 exceptConn（空字符串表示不跳过）。
// 返回成功投递的连接数。
func (h *Hub) SendToUser(userID string, frame []byte, exceptConn string) int {
	h.mu.RLock()
	targets := make([]Client, 0, len(h.users[userID]))
	for id, e := range h.users[userID] {
		if id != exceptConn {
			targets = append(targets, e.c)
		}
	}
	h.mu.RUnlock()

	// 在锁外发送：Send 失败时连接会自行关闭并回调 Unregister，持锁会死锁。
	n := 0
	for _, c := range targets {
		if c.Send(frame) {
			n++
		}
	}
	return n
}

// KickLocal 踢掉本节点上用户的所有连接，返回连接数。
func (h *Hub) KickLocal(userID, reason string) int {
	h.mu.RLock()
	targets := make([]Client, 0, len(h.users[userID]))
	for _, e := range h.users[userID] {
		targets = append(targets, e.c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		c.Kick(reason)
	}
	return len(targets)
}

// ConnCount 返回用户当前的连接数。
func (h *Hub) ConnCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users[userID])
}

// ListUserConns 返回用户在本节点的连接元数据（sessions.Source）。持锁时不碰连接的发送队列以外的东西，不会阻塞。
func (h *Hub) ListUserConns(userID string) []sessions.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns := h.users[userID]
	out := make([]sessions.Conn, 0, len(conns))
	for _, e := range conns {
		out = append(out, e.c.SessionInfo())
	}
	return out
}

// DeliverToAll 实现 delivery.Broadcaster：发给本节点上的全部连接。
func (h *Hub) DeliverToAll(frame []byte) int {
	h.mu.RLock()
	var targets []Client
	for _, conns := range h.users {
		for _, e := range conns {
			targets = append(targets, e.c)
		}
	}
	h.mu.RUnlock()
	n := 0
	for _, c := range targets {
		if c.Send(frame) {
			n++
		}
	}
	return n
}

// KickAll 踢掉本节点上的全部连接（优雅下线），返回连接数。
func (h *Hub) KickAll(reason string) int {
	h.mu.RLock()
	var targets []Client
	for _, conns := range h.users {
		for _, e := range conns {
			targets = append(targets, e.c)
		}
	}
	h.mu.RUnlock()
	for _, c := range targets {
		c.Kick(reason)
	}
	return len(targets)
}
