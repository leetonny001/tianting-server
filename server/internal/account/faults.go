package account

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// errInjected 是故障注入产生的错误。
var errInjected = errors.New("injected failure")

// Faults 是账号功能的故障注入（PRD-chat 第 4.5 节，BE-405）。生产环境不创建（nil），所有方法都是空操作。
// 由 POST /admin/test/faults 打开，那个路由只在测试模式注册。
//
// 本批接上了会用到它们的代码的只有 Argon2 阻塞和注册事务失败。第 4.5 节的其他项（吊销写失败、入表前吊销、入表后和周期复查失败、
// 丢弃吊销事件、入表前暂停）随 BE-416、BE-417 和用到它们的代码一起加，不提供没有接上的开关。
type Faults struct {
	mu           sync.Mutex
	argonGate    chan struct{} // 非 nil 时普通闸门拿到位之后停住，close 放行
	argonBlocked atomic.Int64
	registerTx   int // 还要失败几次注册事务
}

// NewFaults 创建测试模式的故障注入。
func NewFaults() *Faults { return &Faults{} }

// SetArgonBlock 打开或关闭 Argon2 阻塞钩子。关闭时放行已经停住的请求。
func (f *Faults) SetArgonBlock(on bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case on && f.argonGate == nil:
		f.argonGate = make(chan struct{})
	case !on && f.argonGate != nil:
		close(f.argonGate)
		f.argonGate = nil
	}
}

// ArgonBlocked 返回现在停在钩子上的请求数，测试用它确认闸门占满。
func (f *Faults) ArgonBlocked() int {
	if f == nil {
		return 0
	}
	return int(f.argonBlocked.Load())
}

func (f *Faults) waitArgon(ctx context.Context) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	gate := f.argonGate
	f.mu.Unlock()
	if gate == nil {
		return nil
	}
	f.argonBlocked.Add(1)
	defer f.argonBlocked.Add(-1)
	select {
	case <-gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FailRegisterTx 让接下来 n 次注册事务在提交前失败（AC-ACC-01-15）。
func (f *Faults) FailRegisterTx(n int) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerTx = n
}

func (f *Faults) takeRegisterTx() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.registerTx <= 0 {
		return nil
	}
	f.registerTx--
	return errInjected
}
