package logic

import (
	"errors"
	"os"
	"sync"
)

// errInjectedPushWrite 是 push-fail-at 注入的写失败。
var errInjectedPushWrite = errors.New("injected push write failure")

// Faults 是测试模式下的故障注入开关（PRD 6.1 节，BE-003）。
// 每个开关按 clientMsgId 登记，命中一次后自动清除。生产环境不创建（nil），所有方法都是空操作。
type Faults struct {
	mu           sync.Mutex
	dropAck      map[string]bool
	skipDelivery map[string]bool
	dropEvents   map[string]int // userId + "/" + 事件类型 → 还要丢弃几个（PRD-social 5.6 节）
	// 系统推送（PRD-live 8.2 节）：都命中一次后自动清除。
	pushSkipDelivery map[string]bool // pushId
	pushExit         map[string]bool // pushId
	pushFailAt       *int
}

func NewFaults() *Faults {
	return &Faults{dropAck: map[string]bool{}, skipDelivery: map[string]bool{}, dropEvents: map[string]int{},
		pushSkipDelivery: map[string]bool{}, pushExit: map[string]bool{}}
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

// DropEvents：丢弃接下来发给 userID 的 count 个 typ 类型事件（/admin/test/social/drop-events）。
func (f *Faults) DropEvents(userID, typ string, count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropEvents[userID+"/"+typ] = count
}

func (f *Faults) takeDropEvent(userID, typ string) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	k := userID + "/" + typ
	if f.dropEvents[k] <= 0 {
		return false
	}
	f.dropEvents[k]--
	return true
}

// PushSkipDelivery：提交后跳过这个 pushId 的实时下发（AC-PUSH-11-2）。
func (f *Faults) PushSkipDelivery(pushID string) { f.set(f.pushSkipDelivery, pushID) }

// PushExitBeforeBroadcast：幂等记录写入后、广播前退出进程（AC-PUSH-06-7）。
func (f *Faults) PushExitBeforeBroadcast(pushID string) { f.set(f.pushExit, pushID) }

// PushFailAt：下一次收件箱写入在第 index 个用户（按 userId 字节序，从 0 起）失败（AC-PUSH-11-6）。
func (f *Faults) PushFailAt(index int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushFailAt = &index
}

func (f *Faults) takePushSkipDelivery(pushID string) bool {
	if f == nil {
		return false
	}
	return f.take(f.pushSkipDelivery, pushID)
}

func (f *Faults) exitBeforeBroadcast(pushID string) {
	if f != nil && f.take(f.pushExit, pushID) {
		os.Exit(3) // 模拟进程崩溃：不跑 defer，不做优雅退出
	}
}

func (f *Faults) takePushFailAt() (int, bool) {
	if f == nil {
		return 0, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pushFailAt == nil {
		return 0, false
	}
	i := *f.pushFailAt
	f.pushFailAt = nil
	return i, true
}
