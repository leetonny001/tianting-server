// Package social 是业务方可配置的社交参数（PRD-social 5.1 节、F-SOC-01，BE-102）：
// 默认值、范围校验、PATCH 合并，以及带失效通知的进程内缓存。
package social

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
)

// 好友模式。
const (
	ModeOpen    = "open"
	ModeFriends = "friends"
)

// TopicSettings 是配置缓存的失效主题（F-SOC-01 规则 3）。
const TopicSettings = "socialSettings"

// cacheTTL 是兜底过期时间：漏掉失效通知时最多 60 秒后重新读库。
const cacheTTL = 60 * time.Second

// Settings 是 5.1 节的全部参数。M4 参数先定义、M4 生效。
type Settings struct {
	Mode                     string `json:"mode"`
	StrangerMsgLimit         int    `json:"strangerMsgLimit"`
	NewPeerHourlyLimit       int    `json:"newPeerHourlyLimit"`
	NewPeerRejectDailyLimit  int    `json:"newPeerRejectDailyLimit"`
	MaxFriends               int    `json:"maxFriends"`
	MaxBlocks                int    `json:"maxBlocks"`
	RequestExpireDays        int    `json:"requestExpireDays"`
	ApplyDailyLimit          int    `json:"applyDailyLimit"`
	ApplyPerTargetDailyLimit int    `json:"applyPerTargetDailyLimit"`
	BlockDailyLimit          int    `json:"blockDailyLimit"`
	SearchableDefault        bool   `json:"searchableDefault"`
	SearchDailyLimit         int    `json:"searchDailyLimit"`
	ClientGroupEnabled       bool   `json:"clientGroupEnabled"`
	ClientGroupMaxMembers    int    `json:"clientGroupMaxMembers"`
	MaxJoinedGroups          int    `json:"maxJoinedGroups"`
	GroupCreateDailyLimit    int    `json:"groupCreateDailyLimit"`
}

// Defaults 返回 5.1 节的默认值（AC-SOC-01-1）。
func Defaults() Settings {
	return Settings{
		Mode:                     ModeOpen,
		StrangerMsgLimit:         0,
		NewPeerHourlyLimit:       0,
		NewPeerRejectDailyLimit:  100,
		MaxFriends:               2000,
		MaxBlocks:                1000,
		RequestExpireDays:        7,
		ApplyDailyLimit:          30,
		ApplyPerTargetDailyLimit: 3,
		BlockDailyLimit:          100,
		SearchableDefault:        true,
		SearchDailyLimit:         100,
		ClientGroupEnabled:       true,
		ClientGroupMaxMembers:    500,
		MaxJoinedGroups:          500,
		GroupCreateDailyLimit:    10,
	}
}

// Validate 检查范围（F-SOC-01 规则 8）。
func (s Settings) Validate() error {
	if s.Mode != ModeOpen && s.Mode != ModeFriends {
		return fmt.Errorf("mode must be %q or %q", ModeOpen, ModeFriends)
	}
	for _, r := range []struct {
		name      string
		v, lo, hi int
	}{
		{"strangerMsgLimit", s.StrangerMsgLimit, 0, 100},
		{"newPeerHourlyLimit", s.NewPeerHourlyLimit, 0, 1000},
		{"newPeerRejectDailyLimit", s.NewPeerRejectDailyLimit, 10, 1000},
		{"maxFriends", s.MaxFriends, 100, 5000},
		{"maxBlocks", s.MaxBlocks, 100, 5000},
		{"requestExpireDays", s.RequestExpireDays, 1, 30},
		{"applyDailyLimit", s.ApplyDailyLimit, 1, 500},
		{"applyPerTargetDailyLimit", s.ApplyPerTargetDailyLimit, 1, 10},
		{"blockDailyLimit", s.BlockDailyLimit, 10, 1000},
		{"searchDailyLimit", s.SearchDailyLimit, 10, 1000},
		{"clientGroupMaxMembers", s.ClientGroupMaxMembers, 3, 2000},
		{"maxJoinedGroups", s.MaxJoinedGroups, 10, 2000},
		{"groupCreateDailyLimit", s.GroupCreateDailyLimit, 1, 100},
	} {
		if r.v < r.lo || r.v > r.hi {
			return fmt.Errorf("%s must be %d-%d", r.name, r.lo, r.hi)
		}
	}
	return nil
}

// Patch 把 patch（JSON 对象，只含要改的字段）合并到 base 上并校验；未知字段、类型不对、null、超范围都报错，整次不生效。
func Patch(base Settings, patch []byte) (Settings, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(patch, &fields); err != nil || fields == nil {
		return base, errors.New("body must be a JSON object")
	}
	for k, v := range fields {
		if string(bytes.TrimSpace(v)) == "null" {
			return base, fmt.Errorf("%s must not be null", k)
		}
	}
	out := base
	dec := json.NewDecoder(bytes.NewReader(patch))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return base, fmt.Errorf("invalid settings: %v", err)
	}
	if err := out.Validate(); err != nil {
		return base, err
	}
	return out, nil
}

// Parse 读取库里保存的配置（完整 JSON，由本包写入）；缺的字段用默认值补。
func Parse(stored string) (Settings, error) {
	s := Defaults()
	if stored == "" {
		return s, nil
	}
	if err := json.Unmarshal([]byte(stored), &s); err != nil {
		return Defaults(), err
	}
	return s, nil
}

// Store 是缓存用到的存储方法（store.SocialStore 的子集）。
type Store interface {
	SocialSettingsJSON(ctx context.Context) (string, bool, error)
}

// Cache 缓存配置：失效通知（cache.Invalidator，主题 TopicSettings）立即生效，TTL 60 秒兜底（用注入的时钟）。
type Cache struct {
	store Store
	clock clock.Clock

	mu      sync.Mutex
	val     Settings
	expires time.Time
	gen     uint64
}

func NewCache(st Store, clk clock.Clock, inv cache.Invalidator) *Cache {
	c := &Cache{store: st, clock: clk}
	inv.Subscribe(TopicSettings, func(string) { c.Invalidate() })
	return c
}

func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expires = time.Time{}
	c.gen++
}

// Get 返回当前配置；读库失败时返回错误（调用方按 5000 处理）。
func (c *Cache) Get(ctx context.Context) (Settings, error) {
	now := c.clock.Now()
	c.mu.Lock()
	if now.Before(c.expires) {
		v := c.val
		c.mu.Unlock()
		return v, nil
	}
	gen := c.gen
	c.mu.Unlock()

	raw, _, err := c.store.SocialSettingsJSON(ctx)
	if err != nil {
		return Settings{}, err
	}
	v, err := Parse(raw)
	if err != nil {
		return Settings{}, err
	}
	c.mu.Lock()
	if c.gen == gen { // 读库期间收到失效通知：本次照常使用，不放进缓存
		c.val, c.expires = v, now.Add(cacheTTL)
	}
	c.mu.Unlock()
	return v, nil
}
