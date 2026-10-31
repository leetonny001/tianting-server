package logic

import "sync"

// Faults 是测试模式下的故障注入开关（PRD 6.1 节，BE-003）。
// 每个开关按 clientMsgId 登记，命中一次后自动清除。生产环境不创建（nil），所有方法都是空操作。
type Faults struct {
	mu           sync.Mutex
	dropAck      map[string]bool
	skipDelivery map[string]bool
}

func NewFaults() *Faults {
	return &Faults{dropAck: map[string]bool{}, skipDelivery: map[string]bool{}}
}

// DropAck：保存这条消息，但不回 ack（模拟"服务端已保存但 ack 丢失"）。
func (f *Faults) DropAck(clientMsgID string) { f.set(f.dropAck, clientMsgID) }

// SkipDelivery：保存这条消息，但不做实时投递（用来制造 seq 缺口）。
func (f *Faults) SkipDelivery(clientMsgID string) { f.set(f.skipDelivery, clientMsgID) }

func (f *Faults) set(m map[string]bool, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m[key] = true
}

func (f *Faults) take(m map[string]bool, key string) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	hit := m[key]
	delete(m, key)
	return hit
}

func (f *Faults) takeDropAck(clientMsgID string) bool {
	if f == nil {
		return false
	}
	return f.take(f.dropAck, clientMsgID)
}

func (f *Faults) takeSkipDelivery(clientMsgID string) bool {
	if f == nil {
		return false
	}
	return f.take(f.skipDelivery, clientMsgID)
}
