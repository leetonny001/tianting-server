// Package disconnects 是节点内存里的断开记录（PRD-ops F-OPS-09 规则 5）：每个用户最多 50 条，
// 全节点最多 2 万条，超出时淘汰全节点最旧的一条；节点重启后清空。更早的记录查日志。
//
// 写入 O(1)，读取时复制，都只短暂持锁，不影响连接收尾。
package disconnects

import "sync"

const (
	// MaxPerUser 是每个用户保留的条数。
	MaxPerUser = 50
	// MaxTotal 是全节点保留的条数。
	MaxTotal = 20_000
)

// 阶段。
const (
	PhaseHandshake = "handshake"
	PhaseSession   = "session"
)

// Record 是一条断开记录。只存这些字段：不存 token、消息和完整 IP。
type Record struct {
	UserID     string `json:"userId"`
	ConnID     string `json:"connId"` // 握手阶段为空
	Phase      string `json:"phase"`
	Platform   string `json:"platform"`
	Code       int    `json:"code"`
	Reason     string `json:"reason"`
	By         string `json:"by"` // client / server
	TS         int64  `json:"ts"`
	DurationMs int64  `json:"durationMs"` // 握手阶段为 0
	IP         string `json:"ip"`         // 脱敏后的值
}

type slot struct {
	rec   Record
	alive bool
}

// Buffer 是并发安全的环形缓冲。
type Buffer struct {
	mu    sync.Mutex
	limit int
	slots []slot // 环形，next 指向下一个写入位置（也是最旧的一条，写满后）
	next  int
	full  bool
	users map[string][]int // userId → 槽位下标，从旧到新
}

// New 创建缓冲。
func New() *Buffer { return newBuffer(MaxTotal) }

func newBuffer(limit int) *Buffer {
	return &Buffer{limit: limit, users: make(map[string][]int)}
}

// Add 写入一条。
func (b *Buffer) Add(r Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.full {
		if old := &b.slots[b.next]; old.alive {
			b.dropOldest(old.rec.UserID)
		}
		b.slots[b.next] = slot{}
	}
	idx := b.next
	if b.full {
		b.slots[idx] = slot{rec: r, alive: true}
	} else {
		b.slots = append(b.slots, slot{rec: r, alive: true})
	}
	b.next++
	if b.next == b.limit {
		b.next, b.full = 0, true
	}
	list := append(b.users[r.UserID], idx)
	if len(list) > MaxPerUser {
		b.slots[list[0]].alive = false
		b.slots[list[0]].rec = Record{} // 释放字符串
		list = append(list[:0], list[1:]...)
	}
	b.users[r.UserID] = list
}

// dropOldest 从用户的列表里去掉最旧的一条；用户没有记录时删掉整个条目，用户数不随历史增长。
func (b *Buffer) dropOldest(userID string) {
	list := b.users[userID]
	if len(list) <= 1 {
		delete(b.users, userID)
		return
	}
	b.users[userID] = append(list[:0], list[1:]...)
}

// Query 返回用户最近的记录，新的在前。limit 超出范围时按 50。
func (b *Buffer) Query(userID string, limit int) []Record {
	if limit <= 0 || limit > MaxPerUser {
		limit = MaxPerUser
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.users[userID]
	out := make([]Record, 0, min(limit, len(list)))
	for i := len(list) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, b.slots[list[i]].rec)
	}
	return out
}

// Oldest 返回缓冲里最早一条的时间；缓冲为空时返回 0。
func (b *Buffer) Oldest() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(b.slots)
	start := 0
	if b.full {
		start = b.next
	}
	for i := range n {
		if s := &b.slots[(start+i)%n]; s.alive {
			return s.rec.TS
		}
	}
	return 0
}

// Len 返回当前保留的条数。
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	total := 0
	for _, l := range b.users {
		total += len(l)
	}
	return total
}
