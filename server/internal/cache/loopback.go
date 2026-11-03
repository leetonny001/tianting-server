package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HTTPLoopback 是测试模式下的跨进程实现（PRD-social 5.6 节"双节点测试环境"，BE-104）：
// 先在本机回调，再把失效消息 POST 给其他进程的 /admin/test/invalidate。peers 为空时只在本机生效
// （IM_TEST_INVALIDATION=off，用来验证 TTL 兜底）。只验证配置和缓存失效；跨节点的事件投递等 B6 的消息总线。
type HTTPLoopback struct {
	*Local
	adminKey string
	client   *http.Client

	off bool // IM_TEST_INVALIDATION=off：不广播

	mu    sync.RWMutex
	peers []string // 其他进程管理端口的基地址，如 http://127.0.0.1:18081
}

func NewHTTPLoopback(peers []string, adminKey string, off bool) *HTTPLoopback {
	h := &HTTPLoopback{Local: NewLocal(), adminKey: adminKey, off: off, client: &http.Client{Timeout: 500 * time.Millisecond}}
	h.SetPeers(peers)
	return h
}

// SetPeers 替换广播目标（e2e 在两个进程都启动之后才知道彼此的地址）。
func (h *HTTPLoopback) SetPeers(peers []string) {
	if h.off {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers = peers
}

// Invalidate 本机回调后同步广播给其他进程（测试模式，同步发送让"1 秒内生效"的断言确定）。失败只丢弃：TTL 兜底。
func (h *HTTPLoopback) Invalidate(topic, key string) {
	h.Local.Invalidate(topic, key)
	h.mu.RLock()
	peers := h.peers
	h.mu.RUnlock()
	body, _ := json.Marshal(map[string]string{"topic": topic, "key": key})
	for _, p := range peers {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, p+"/admin/test/invalidate", bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Admin-Key", h.adminKey)
		if resp, err := h.client.Do(req); err == nil {
			resp.Body.Close()
		}
	}
}

// InvalidateLocal 处理其他进程的广播：只在本机回调、不再转发；off 时忽略（只靠 TTL 兜底）。
func (h *HTTPLoopback) InvalidateLocal(topic, key string) {
	if !h.off {
		h.Local.Invalidate(topic, key)
	}
}
