// Package cache 定义进程内缓存的失效广播（ARCH 2.2 约束 2）。
//
// 规则：任何进程内缓存都必须订阅失效通知，并且自带 TTL 兜底。
// 单节点时失效通知直接在本机回调；集群时经消息总线发给所有节点，否则其他节点会一直读到旧数据。
package cache

import "sync"

// Invalidator 广播"某个 key 的数据变了"。
type Invalidator interface {
	// Invalidate 通知所有节点上订阅了 topic 的缓存删除 key。
	Invalidate(topic, key string)
	// Subscribe 注册失效回调。回调不能阻塞。
	Subscribe(topic string, fn func(key string))
}

// Local 是单节点实现：Invalidate 同步调用本机的订阅者。
type Local struct {
	mu   sync.RWMutex
	subs map[string][]func(string)
}

func NewLocal() *Local { return &Local{subs: map[string][]func(string){}} }

func (l *Local) Invalidate(topic, key string) {
	l.mu.RLock()
	fns := l.subs[topic]
	l.mu.RUnlock()
	for _, fn := range fns {
		fn(key)
	}
}

func (l *Local) Subscribe(topic string, fn func(key string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.subs[topic] = append(l.subs[topic], fn)
}
