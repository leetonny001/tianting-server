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
	actions          msgActionFaults
	mod              modFaults
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

// C4 的故障注入（PRD-chat 第 4.5 节，BE-471 第 4 条）：开关式，打开后一直生效，直到关掉（AC-RCL-01-11 的"恢复后重试"）。
// 只在测试模式有 Faults；生产环境 f 为 nil，全部是空操作。
type msgActionFaults struct {
	quarantineWrite, mentionWrite, mentionAllCounter bool
}

var errInjectedFault = errors.New("injected fault")

// SetMsgActionFaults 设置 C4 的三个故障；nil 表示不改。
func (f *Faults) SetMsgActionFaults(quarantineWrite, mentionWrite, mentionAllCounter *bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range []struct {
		dst *bool
		src *bool
	}{{&f.actions.quarantineWrite, quarantineWrite}, {&f.actions.mentionWrite, mentionWrite}, {&f.actions.mentionAllCounter, mentionAllCounter}} {
		if p.src != nil {
			*p.dst = *p.src
		}
	}
}

func (f *Faults) msgActions() msgActionFaults {
	if f == nil {
		return msgActionFaults{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.actions
}

// C6 的故障注入（PRD-chat 第 4.5 节，BE-492 第 5 条）：开关式，打开后一直生效到关掉。只在测试模式有 Faults；生产 f 为 nil，全部是空操作。
// 启动加载失败（AC-MOD-01-8 第 2 种）在进程启动之前就要生效，不经这个接口，用 IM_TEST_WORDS_STARTUP_FAIL；证据密钥不匹配用换一把
// IM_REPORT_KEY 重启来造，不加注入（BE-492 第 5 条）。
type modFaults struct {
	wordsReload, evidenceWrite bool
}

// SetModFaults 设置 C6 的两个故障；nil 表示不改。
func (f *Faults) SetModFaults(wordsReload, evidenceWrite *bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if wordsReload != nil {
		f.mod.wordsReload = *wordsReload
	}
	if evidenceWrite != nil {
		f.mod.evidenceWrite = *evidenceWrite
	}
}

// WordsReloadFails 报告词库重新加载是否要失败（AC-MOD-01-8 第 1 种）：词库引擎在读库之前调用。
func (f *Faults) WordsReloadFails() bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mod.wordsReload
}

// evidenceWriteFails 报告举报的证据写入是否要失败（AC-MOD-03-14 前半）。
func (f *Faults) evidenceWriteFails() bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mod.evidenceWrite
}
