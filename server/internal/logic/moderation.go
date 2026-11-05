package logic

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// TopicMutes 是禁言缓存的失效主题，key 为 userId（ARCH 2.2 约束 2，F-ADM-40 规则 5）。
const TopicMutes = "mutes"

const (
	muteCacheTTL   = time.Minute
	maxMuteEntries = 200_000 // 超过时整体清空：只是缓存，下一次发送重新加载
)

// muteCache 缓存每个用户的禁言记录，发送路径不用每条消息都查库。
type muteCache struct {
	store store.Store
	clock clock.Clock

	mu      sync.Mutex
	entries map[string]*muteEntry
	gen     map[string]uint64
	byGroup map[string]map[string]struct{} // groupId → 缓存里在该群有禁言的用户
}

type muteEntry struct {
	user    *store.Mute
	groups  map[string]store.Mute
	expires time.Time
}

func newMuteCache(st store.Store, clk clock.Clock, inv cache.Invalidator) *muteCache {
	c := &muteCache{store: st, clock: clk, entries: map[string]*muteEntry{}, gen: map[string]uint64{}, byGroup: map[string]map[string]struct{}{}}
	inv.Subscribe(TopicMutes, c.invalidateUser)
	// 成员离开群时存储层已经在同一事务删了禁言记录；这里让缓存跟上，再入群不继承（F-ADM-41 规则 4）。
	inv.Subscribe(TopicGroupMembers, c.invalidateGroup)
	return c
}

func (c *muteCache) invalidateUser(uid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropLocked(uid)
}

func (c *muteCache) invalidateGroup(gid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for uid := range c.byGroup[gid] {
		c.dropLocked(uid)
	}
}

func (c *muteCache) dropLocked(uid string) {
	if e := c.entries[uid]; e != nil {
		for gid := range e.groups {
			delete(c.byGroup[gid], uid)
		}
		delete(c.entries, uid)
	}
	c.gen[uid]++
}

func (c *muteCache) get(ctx context.Context, uid string) (*muteEntry, error) {
	now := c.clock.Now()
	c.mu.Lock()
	if e, ok := c.entries[uid]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e, nil
	}
	gen := c.gen[uid]
	c.mu.Unlock()

	rows, err := c.store.UserMutes(ctx, uid)
	if err != nil {
		return nil, err
	}
	e := &muteEntry{expires: now.Add(muteCacheTTL)}
	for _, m := range rows {
		if m.Scope == store.MuteUser {
			u := m
			e.user = &u
			continue
		}
		if e.groups == nil {
			e.groups = map[string]store.Mute{}
		}
		e.groups[m.GroupID] = m
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen[uid] != gen { // 加载期间被失效：本次照常用，不放进缓存
		return e, nil
	}
	if len(c.entries) >= maxMuteEntries {
		c.entries, c.byGroup = map[string]*muteEntry{}, map[string]map[string]struct{}{}
	}
	if old := c.entries[uid]; old != nil {
		for gid := range old.groups {
			delete(c.byGroup[gid], uid)
		}
	}
	c.entries[uid] = e
	for gid := range e.groups {
		if c.byGroup[gid] == nil {
			c.byGroup[gid] = map[string]struct{}{}
		}
		c.byGroup[gid][uid] = struct{}{}
	}
	return e, nil
}

// EnableAdminFeatures 打开第二批的禁言检查（IM_V13_ADMIN_FEATURES）。只在启动时调用。
func (s *Service) EnableAdminFeatures() { s.mutes = newMuteCache(s.store, s.clock, s.inv) }

// checkMuted 在发送限流之前调用（C-29）：4031 不扣令牌。用户级优先（F-ADM-41 规则 2）。
func (s *Service) checkMuted(ctx context.Context, uid string, req protocol.SendReq) *protocol.Error {
	if s.mutes == nil {
		return nil
	}
	e, err := s.mutes.get(ctx, uid)
	if err != nil {
		return s.internal("load mutes", err)
	}
	now := s.clock.Now().UnixMilli()
	if e.user.Active(now) {
		return mutedErr(map[string]any{"scope": protocol.MuteScopeUser, "mutedUntil": e.user.Until})
	}
	if req.ConvType == protocol.ConvGroup {
		if m, ok := e.groups[req.To]; ok && m.Active(now) {
			return mutedErr(map[string]any{"scope": protocol.MuteScopeGroup, "groupId": req.To, "mutedUntil": m.Until})
		}
	}
	return nil
}

func mutedErr(details map[string]any) *protocol.Error {
	e := protocol.Errorf(protocol.CodeMuted, "muted")
	e.Details = details
	return e
}

// AuthMutedUntil 是 auth ack 的 mutedUntil：只有用户级禁言才带（F-ADM-40 规则 4）。查询失败不影响登录。
func (s *Service) AuthMutedUntil(ctx context.Context, uid string) json.RawMessage {
	if s.mutes == nil {
		return nil
	}
	e, err := s.mutes.get(ctx, uid)
	if err != nil {
		s.log.Error("load mutes for auth", "err", err)
		return nil
	}
	if !e.user.Active(s.clock.Now().UnixMilli()) {
		return nil
	}
	return protocol.NullableMs(e.user.Until)
}

// applyGroupMutes 给会话列表里的群补上 mutedUntil（F-ADM-41 规则 5）。
func (s *Service) applyGroupMutes(ctx context.Context, uid string, convs []protocol.Conversation) *protocol.Error {
	if s.mutes == nil {
		return nil
	}
	e, err := s.mutes.get(ctx, uid)
	if err != nil {
		return s.internal("load mutes", err)
	}
	if len(e.groups) == 0 {
		return nil
	}
	now := s.clock.Now().UnixMilli()
	for i := range convs {
		if convs[i].Type != protocol.ConvGroup {
			continue
		}
		conv, perr := protocol.ParseConvID(convs[i].ConvID)
		if perr != nil {
			continue
		}
		if m, ok := e.groups[conv.A]; ok && m.Active(now) {
			convs[i].MutedUntil = protocol.NullableMs(m.Until)
		}
	}
	return nil
}

// NotifyMuted 只发给本人（F-ADM-40 规则 4、F-ADM-41 规则 5）。
func (s *Service) NotifyMuted(uid, scope, groupID string, until *int64) {
	s.pushEvent(uid, protocol.MutedEvent{Type: protocol.EventMuted, Scope: scope, GroupID: groupID, MutedUntil: until}, "")
}

// NotifyUnmuted 只发给本人。
func (s *Service) NotifyUnmuted(uid, scope, groupID string) {
	s.pushEvent(uid, protocol.UnmutedEvent{Type: protocol.EventUnmuted, Scope: scope, GroupID: groupID}, "")
}

// NotifyMsgDeleted 发给会话的全部在线参与者（单聊双方、群全体成员，含发送者其他端；F-ADM-31 规则 5）。
func (s *Service) NotifyMsgDeleted(ctx context.Context, convID string, msgIDs []string, seqs []int64) {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return
	}
	var users []string
	switch conv.Type {
	case protocol.ConvC2C:
		users = []string{conv.A}
		if conv.B != conv.A {
			users = append(users, conv.B)
		}
	case protocol.ConvGroup:
		var err error
		if users, err = s.store.GroupMembers(ctx, conv.A); err != nil {
			s.log.Error("load group members for msgDeleted", "err", err, "conv", convID)
			return
		}
	default:
		return
	}
	frame, err := json.Marshal(protocol.Event(protocol.MsgDeletedEvent{Type: protocol.EventMsgDeleted, ConvID: convID, MsgIDs: slices.Clone(msgIDs), Seqs: slices.Clone(seqs)}))
	if err != nil {
		s.log.Error("marshal msgDeleted", "err", err)
		return
	}
	for _, uid := range users {
		s.router.DeliverToUser(uid, frame, "")
	}
}

// ToProto 把存储层的消息转成协议消息（管理接口读正文用）。
func ToProto(m store.Message) protocol.Message { return toProto(m) }
