package logic

import (
	"context"
	"sync"
	"time"

	"im-mvp/server/internal/cache"
	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/store"
)

// TopicGroupMembers 是群成员缓存的失效主题，key 为 groupId。
// 建群、加成员、移除成员之后必须发送失效通知（admin 包负责）。
const TopicGroupMembers = "groupMembers"

// memberCacheTTL 是兜底过期时间（ARCH 2.2 约束 2）：即使漏掉了失效通知，最多 1 分钟后也会重新加载。
const memberCacheTTL = time.Minute

// memberCache 缓存每个群的成员集合（BE-033），避免每条群消息都查一次全部成员。
// 只用于发送时的成员校验和投递；sync / read 的权限检查直接查库（还需要 joinSeq，且频率低）。
type memberCache struct {
	store store.Store
	clock clock.Clock

	mu      sync.Mutex
	entries map[string]*memberEntry
	gen     map[string]uint64 // 每个群的失效次数，用于丢弃"加载期间被失效"的结果
}

type memberEntry struct {
	exists  bool
	members []string
	set     map[string]struct{}
	expires time.Time
}

func newMemberCache(st store.Store, clk clock.Clock, inv cache.Invalidator) *memberCache {
	c := &memberCache{store: st, clock: clk, entries: map[string]*memberEntry{}, gen: map[string]uint64{}}
	inv.Subscribe(TopicGroupMembers, c.invalidate)
	return c
}

func (c *memberCache) invalidate(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, groupID)
	c.gen[groupID]++
}

// get 返回群是否存在及其成员。命中时不查库。
func (c *memberCache) get(ctx context.Context, groupID string) (*memberEntry, error) {
	now := c.clock.Now()
	c.mu.Lock()
	if e, ok := c.entries[groupID]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e, nil
	}
	gen := c.gen[groupID]
	c.mu.Unlock()

	e := &memberEntry{expires: now.Add(memberCacheTTL)}
	exists, err := c.store.GroupExists(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if e.exists = exists; exists {
		if e.members, err = c.store.GroupMembers(ctx, groupID); err != nil {
			return nil, err
		}
	}
	e.set = make(map[string]struct{}, len(e.members))
	for _, m := range e.members {
		e.set[m] = struct{}{}
	}

	c.mu.Lock()
	// 加载期间如果收到了失效通知，这次读到的可能是旧数据：本次照常使用，但不放进缓存。
	if c.gen[groupID] == gen {
		c.entries[groupID] = e
	}
	c.mu.Unlock()
	return e, nil
}

func (e *memberEntry) has(uid string) bool {
	_, ok := e.set[uid]
	return ok
}
