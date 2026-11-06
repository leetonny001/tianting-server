//go:build imhooks

package testhook

import "sync"

// Compiled 表示这个二进制带钩子。
const Compiled = true

// Pause 是一个已布置的暂停点。
type Pause struct {
	reached chan struct{}
	release chan struct{}
}

// Reached 在请求停到暂停点时关闭。
func (p *Pause) Reached() <-chan struct{} { return p.reached }

// Release 放行。
func (p *Pause) Release() { close(p.release) }

var (
	mu    sync.Mutex
	armed = map[string]*Pause{}
)

// Arm 让 key 的下一个请求停在 AfterArgon。
func Arm(key string) *Pause {
	p := &Pause{reached: make(chan struct{}), release: make(chan struct{})}
	mu.Lock()
	defer mu.Unlock()
	armed[key] = p
	return p
}

// AfterArgon 在 Argon2 计算之后、提交事务之前调用。命中一次后自动清除。
func AfterArgon(key string) {
	mu.Lock()
	p := armed[key]
	delete(armed, key)
	mu.Unlock()
	if p != nil {
		close(p.reached)
		<-p.release
	}
}

// ArmConn 让这个用户的下一条连接停在 BeforeRegister。
func ArmConn(userID string) *Pause { return Arm("conn:" + userID) }

// BeforeRegister 在连接校验之后、入表之前调用。命中一次后自动清除。
func BeforeRegister(userID string) { AfterArgon("conn:" + userID) }
