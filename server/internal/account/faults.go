package account

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"tianting/server/internal/auth"
)

// errInjected 是故障注入产生的错误。
var errInjected = errors.New("injected failure")

// Faults 是账号功能的故障注入（PRD-chat 第 4.5 节，BE-405）。生产环境不创建（nil），所有方法都是空操作。
// 由 POST /admin/test/faults 打开，那个路由只在测试模式注册。
//
// 入表前暂停是构建标签隔离的 testhook，丢弃吊销事件在 hub（SetDropKicks）。
type Faults struct {
	mu           sync.Mutex
	argonGate    chan struct{} // 非 nil 时普通闸门拿到位之后停住，close 放行
	argonBlocked atomic.Int64
	registerTx   int             // 还要失败几次注册事务
	revokeWrite  int             // 还要失败几次吊销写
	revokeBefore map[string]bool // 这些用户的下一条连接入表前吊销它的 sid
	cleanupSteps int             // >0 时 account_cleanup 第 ② 步再走这么多步就失败（AC-ACC-08-5）
	entryRecheck int             // 还要失败几次入表后复查
	periodic     map[string]int  // 用户 → 还要失败几次周期复查
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

// FailRevokeWrite 让接下来 n 次吊销写在提交前失败，整个事务回滚（AC-ACC-04-7）。
func (f *Faults) FailRevokeWrite(n int) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revokeWrite = n
}

func (f *Faults) takeRevokeWrite() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revokeWrite <= 0 {
		return nil
	}
	f.revokeWrite--
	return errInjected
}

// RevokeBeforeRegister 让这个用户的下一条连接在入表前被吊销（AC-ACC-03-17）。
func (f *Faults) RevokeBeforeRegister(userID string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revokeBefore == nil {
		f.revokeBefore = map[string]bool{}
	}
	f.revokeBefore[userID] = true
}

// FailEntryRechecks 让接下来 n 次入表后复查报错（AC-ACC-03-36）。
func (f *Faults) FailEntryRechecks(n int) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entryRecheck = n
}

// FailPeriodicRechecks 让这个用户接下来 n 次周期复查报错（AC-ACC-03-47）。
func (f *Faults) FailPeriodicRechecks(userID string, n int) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.periodic == nil {
		f.periodic = map[string]int{}
	}
	f.periodic[userID] = n
}

// ConnFaults 实现 gateway.Faults。
type ConnFaults struct {
	f      *Faults
	revoke func(ctx context.Context, sid string)
}

// BeforeRegister 吊销这条连接的 sid（入表前吊销）。
func (c ConnFaults) BeforeRegister(ctx context.Context, claims auth.Claims) {
	c.f.mu.Lock()
	armed := c.f.revokeBefore[claims.UserID]
	delete(c.f.revokeBefore, claims.UserID)
	c.f.mu.Unlock()
	if armed {
		c.revoke(ctx, claims.SID)
	}
}

// FailEntryRecheck 实现 gateway.Faults。
func (c ConnFaults) FailEntryRecheck() bool {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	if c.f.entryRecheck <= 0 {
		return false
	}
	c.f.entryRecheck--
	return true
}

// FailPeriodicRecheck 实现 gateway.Faults。
func (c ConnFaults) FailPeriodicRecheck(userID string) bool {
	c.f.mu.Lock()
	defer c.f.mu.Unlock()
	if c.f.periodic[userID] <= 0 {
		return false
	}
	c.f.periodic[userID]--
	return true
}

// FailCleanupAfter 让 account_cleanup 第 ② 步再走 n 步之后失败（AC-ACC-08-5）。
func (f *Faults) FailCleanupAfter(n int) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupSteps = n
}

func (f *Faults) takeCleanupStep() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupSteps == 0 {
		return nil
	}
	f.cleanupSteps--
	if f.cleanupSteps == 0 {
		return errInjected
	}
	return nil
}
