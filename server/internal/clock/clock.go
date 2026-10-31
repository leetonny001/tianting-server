// Package clock 提供可注入的时钟。生产环境使用真实时间；
// 测试模式下可以把时间冻结在某一刻（PRD 6.1 节、C-43、IMP-7）。
package clock

import (
	"sync"
	"time"
)

// Clock 返回当前时间。
type Clock interface {
	Now() time.Time
}

// Settable 默认返回真实时间；Freeze 之后返回冻结的时间，直到 Unfreeze。
type Settable struct {
	mu     sync.RWMutex
	frozen *time.Time
}

func New() *Settable { return &Settable{} }

func (c *Settable) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.frozen != nil {
		return *c.frozen
	}
	return time.Now()
}

// Freeze 把时间固定在 t。
func (c *Settable) Freeze(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frozen = &t
}

// Unfreeze 恢复真实时间。
func (c *Settable) Unfreeze() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frozen = nil
}
