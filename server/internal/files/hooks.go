package files

import (
	"errors"
	"sync"
)

// 暂停点（PRD-chat 第 4.5 节"complete 的暂停点"，BE-404 第 3 条）。只在测试模式有 Hooks，生产为 nil，所有方法都是空操作。
const (
	PointAfterSniff      = "afterSniff"      // 校验之后、Copy 之前
	PointAfterCopy       = "afterCopy"       // 全部变体 Copy 之后、提交之前
	PointAfterPublicCopy = "afterPublicCopy" // 头像 Copy 到公开桶之后、绑定之前
)

// errCrash 模拟进程在暂停点被杀：这次尝试立刻结束，不释放租约、不删任何键（AC-FILE-01-26、AC-ACC-06-12、06-13）。
var errCrash = errors.New("simulated crash")

// Pause 是一个已布置的暂停点。
type Pause struct {
	reached chan struct{}
	release chan struct{}
}

// Reached 在请求停到暂停点时关闭。
func (p *Pause) Reached() <-chan struct{} { return p.reached }

// Release 放行。
func (p *Pause) Release() { close(p.release) }

// Hooks 是测试模式的故障注入。
type Hooks struct {
	mu     sync.Mutex
	pauses map[string]*Pause
	crash  map[string]bool
}

// NewHooks 只在测试模式调用。
func NewHooks() *Hooks { return &Hooks{pauses: map[string]*Pause{}, crash: map[string]bool{}} }

// Arm 让 fileID 的下一次尝试停在 point，命中一次后清除。
func (h *Hooks) Arm(point, fileID string) *Pause {
	p := &Pause{reached: make(chan struct{}), release: make(chan struct{})}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pauses[point+"/"+fileID] = p
	return p
}

// Crash 让 fileID 的下一次尝试在 point "被杀"，命中一次后清除。
func (h *Hooks) Crash(point, fileID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.crash[point+"/"+fileID] = true
}

func (h *Hooks) at(point, fileID string) error {
	if h == nil {
		return nil
	}
	k := point + "/" + fileID
	h.mu.Lock()
	p := h.pauses[k]
	delete(h.pauses, k)
	crash := h.crash[k]
	delete(h.crash, k)
	h.mu.Unlock()
	if p != nil {
		close(p.reached)
		<-p.release
	}
	if crash {
		return errCrash
	}
	return nil
}
