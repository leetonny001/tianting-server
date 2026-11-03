package adminauth

import (
	"errors"
	"sync"
	"time"
)

// NonceTTL 是 nonce 的去重窗口（PRD-admin F-KEY-03 规则 4）。它大于时间戳窗口（±5 分钟），
// 所以时间戳仍在窗口内的重放一定能被识别。
const NonceTTL = 10 * time.Minute

// ErrNonceStoreFull 表示 nonce 存储已满：拒绝新请求（503 / 5030），而不是放弃去重。
var ErrNonceStoreFull = errors.New("nonce store full")

// NonceStore 登记已使用的 nonce。Add 在 nonce 第一次出现时返回 true；
// 只应在签名校验通过之后调用，防止用垃圾请求挤占存储。集群（B6）换成 Redis 实现（SET NX EX 600）。
type NonceStore interface {
	Add(caller, nonce string, now time.Time) (bool, error)
}

// MemoryNonces 是单节点的内存实现。重启后清空：重启后 5 分钟内的重放窗口靠时间戳窗口兜底（PRD-admin 13.2 节已接受风险）。
type MemoryNonces struct {
	mu       sync.Mutex
	capacity int
	seen     map[string]time.Time // caller + "\x00" + nonce → 过期时间
	nextGC   time.Time
}

// NewMemoryNonces 创建容量为 capacity 的存储（PRD：100 万）。
func NewMemoryNonces(capacity int) *MemoryNonces {
	return &MemoryNonces{capacity: capacity, seen: map[string]time.Time{}}
}

func (m *MemoryNonces) Add(caller, nonce string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := caller + "\x00" + nonce
	if exp, ok := m.seen[key]; ok && now.Before(exp) {
		return false, nil
	}
	if len(m.seen) >= m.capacity || now.After(m.nextGC) {
		m.gc(now)
	}
	if len(m.seen) >= m.capacity {
		return false, ErrNonceStoreFull
	}
	m.seen[key] = now.Add(NonceTTL)
	return true, nil
}

// gc 删除过期的 nonce；最多每分钟一次（满时立即执行）。
func (m *MemoryNonces) gc(now time.Time) {
	for k, exp := range m.seen {
		if !now.Before(exp) {
			delete(m.seen, k)
		}
	}
	m.nextGC = now.Add(time.Minute)
}

// Len 返回当前登记的 nonce 数（测试和指标用）。
func (m *MemoryNonces) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.seen)
}
