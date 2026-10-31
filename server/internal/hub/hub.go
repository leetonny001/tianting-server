// Package hub 维护在线连接表：用户 → 该用户的所有连接（多端同时在线）。
package hub

import "sync"

// Client 是一个在线连接。Send、Kick 都不能阻塞。
type Client interface {
	ConnID() uint64
	UserID() string
	Send(frame []byte) bool // 发不出去时返回 false
	Kick(reason string)     // 下发 kicked 并关闭连接
}

// Hub 是并发安全的在线表。
type Hub struct {
	mu    sync.RWMutex
	users map[string]map[uint64]Client
}

func New() *Hub {
	return &Hub{users: make(map[string]map[uint64]Client)}
}

// Register 登记连接。
func (h *Hub) Register(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.users[c.UserID()]
	if conns == nil {
		conns = make(map[uint64]Client)
		h.users[c.UserID()] = conns
	}
	conns[c.ConnID()] = c
}

// Unregister 移除连接；用户没有连接时从表里删掉。
func (h *Hub) Unregister(c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.users[c.UserID()]
	delete(conns, c.ConnID())
	if len(conns) == 0 {
		delete(h.users, c.UserID())
	}
}

// SendToUser 把帧发给用户的所有连接，跳过 exceptConn（0 表示不跳过）。
// 返回成功投递的连接数。
func (h *Hub) SendToUser(userID string, frame []byte, exceptConn uint64) int {
	h.mu.RLock()
	targets := make([]Client, 0, len(h.users[userID]))
	for id, c := range h.users[userID] {
		if id != exceptConn {
			targets = append(targets, c)
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

// KickUser 踢掉用户的所有连接，返回连接数。
func (h *Hub) KickUser(userID, reason string) int {
	h.mu.RLock()
	targets := make([]Client, 0, len(h.users[userID]))
	for _, c := range h.users[userID] {
		targets = append(targets, c)
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
