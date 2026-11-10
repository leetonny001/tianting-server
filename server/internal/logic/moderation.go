package logic

import (
	"context"
	"encoding/json"
	"errors"
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

// TopicGroupAllMutes 是全群禁言缓存的失效主题，key 为 groupId（F-ADM-42）。
const TopicGroupAllMutes = "groupAllMutes"

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

	groupAll    map[string]*groupAllEntry // groupId → 全群禁言；没有记录的群也缓存，发送路径不用每条都查库
	groupAllGen map[string]uint64
}

type groupAllEntry struct {
	mute    *store.Mute
	expires time.Time
}

type muteEntry struct {
	user    *store.Mute
	groups  map[string]store.Mute
	expires time.Time
}

func newMuteCache(st store.Store, clk clock.Clock, inv cache.Invalidator) *muteCache {
	c := &muteCache{
		store: st, clock: clk, entries: map[string]*muteEntry{}, gen: map[string]uint64{}, byGroup: map[string]map[string]struct{}{},
		groupAll: map[string]*groupAllEntry{}, groupAllGen: map[string]uint64{},
	}
	inv.Subscribe(TopicMutes, c.invalidateUser)
	inv.Subscribe(TopicGroupAllMutes, c.invalidateGroupAll)
	// 成员离开群时存储层已经在同一事务删了禁言记录；这里让缓存跟上，再入群不继承（F-ADM-41 规则 4）。
	inv.Subscribe(TopicGroupMembers, c.invalidateGroup)
	return c
}

func (c *muteCache) invalidateUser(uid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropLocked(uid)
}

func (c *muteCache) invalidateGroupAll(gid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropGroupAllLocked(gid)
}

func (c *muteCache) dropGroupAllLocked(gid string) {
	delete(c.groupAll, gid)
	c.groupAllGen[gid]++
}

func (c *muteCache) invalidateGroup(gid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropGroupAllLocked(gid) // 解散时存储层删了全群禁言
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

// getGroupAll 返回群的全群禁言记录（可能已过期），没有时为 nil。
func (c *muteCache) getGroupAll(ctx context.Context, gid string) (*store.Mute, error) {
	now := c.clock.Now()
	c.mu.Lock()
	if e, ok := c.groupAll[gid]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e.mute, nil
	}
	gen := c.groupAllGen[gid]
	c.mu.Unlock()

	m, err := c.store.GroupAllMute(ctx, gid)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.groupAllGen[gid] != gen { // 加载期间被失效：本次照常用，不放进缓存
		return m, nil
	}
	if len(c.groupAll) >= maxMuteEntries {
		c.groupAll = map[string]*groupAllEntry{}
	}
	c.groupAll[gid] = &groupAllEntry{mute: m, expires: now.Add(muteCacheTTL)}
	return m, nil
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
		return s.checkGroupAllMuted(ctx, uid, req.To, now)
	}
	return nil
}

// checkGroupAllMuted：全群禁言时只有普通成员不能发言，群主、管理员不受限（F-ADM-42）。
// 角色直接查库，不读缓存：只有开启了全群禁言的群才走到这里，而转让、设管理员要立即生效。
// 不是成员的继续往下走，由成员校验返回群不存在或不是成员，不用 4031 暴露群的状态。
func (s *Service) checkGroupAllMuted(ctx context.Context, uid, gid string, now int64) *protocol.Error {
	m, err := s.mutes.getGroupAll(ctx, gid)
	if err != nil {
		return s.internal("load group mute", err)
	}
	if !m.Active(now) {
		return nil
	}
	role, err := s.memberRole(ctx, gid, uid)
	switch {
	case errors.Is(err, store.ErrNotGroupMember):
		return nil
	case err != nil:
		return s.internal("load member role", err)
	case role == store.RoleOwner || role == store.RoleAdmin:
		return nil
	}
	return mutedErr(map[string]any{"scope": protocol.MuteScopeGroupAll, "groupId": gid, "mutedUntil": m.Until})
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

// applyGroupMutes 给会话列表里的群补上 mutedUntil（F-ADM-41 规则 5）和 allMuted（F-ADM-42）。
func (s *Service) applyGroupMutes(ctx context.Context, uid string, convs []protocol.Conversation) *protocol.Error {
	if s.mutes == nil {
		return nil
	}
	e, err := s.mutes.get(ctx, uid)
	if err != nil {
		return s.internal("load mutes", err)
	}
	// 全群禁言一次查出来：会话列表最多 500 个群，不能逐个查。
	all, err := s.store.UserGroupAllMutes(ctx, uid)
	if err != nil {
		return s.internal("load group mutes", err)
	}
	if len(e.groups) == 0 && len(all) == 0 {
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
		if m, ok := all[conv.A]; ok && m.Active(now) {
			convs[i].AllMuted = true
		}
	}
	return nil
}

// withAllMuted 给 convAdded 的会话补上 allMuted：全群禁言期间入群的成员不用等下一次 convs 就能禁用输入框。查询失败只记日志，不影响入群通知。
func (s *Service) withAllMuted(ctx context.Context, gid string) bool {
	if s.mutes == nil {
		return false
	}
	m, err := s.mutes.getGroupAll(ctx, gid)
	if err != nil {
		s.log.Error("load group mute", "err", err, "group", gid)
		return false
	}
	return m.Active(s.clock.Now().UnixMilli())
}

// NotifyMuted 只发给本人（F-ADM-40 规则 4、F-ADM-41 规则 5）。
func (s *Service) NotifyMuted(uid, scope, groupID string, until *int64) {
	s.pushEvent(uid, protocol.MutedEvent{Type: protocol.EventMuted, Scope: scope, GroupID: groupID, MutedUntil: until}, "")
}

// NotifyUnmuted 只发给本人。
func (s *Service) NotifyUnmuted(uid, scope, groupID string) {
	s.pushEvent(uid, protocol.UnmutedEvent{Type: protocol.EventUnmuted, Scope: scope, GroupID: groupID}, "")
}

// NotifyGroupAllMuted 发给全群在线成员（F-ADM-42），含不受限的群主、管理员：是否能发言由客户端结合 myRole 显示、由服务端在发送时判断。
func (s *Service) NotifyGroupAllMuted(ctx context.Context, gid string, until *int64) {
	s.pushGroupEvent(ctx, gid, protocol.MutedEvent{Type: protocol.EventMuted, Scope: protocol.MuteScopeGroupAll, GroupID: gid, MutedUntil: until})
}

// NotifyGroupAllUnmuted 发给全群在线成员。
func (s *Service) NotifyGroupAllUnmuted(ctx context.Context, gid string) {
	s.pushGroupEvent(ctx, gid, protocol.UnmutedEvent{Type: protocol.EventUnmuted, Scope: protocol.MuteScopeGroupAll, GroupID: gid})
}

func (s *Service) pushGroupEvent(ctx context.Context, gid string, ev any) {
	users, err := s.store.GroupMembers(ctx, gid)
	if err != nil {
		s.log.Error("load group members for mute event", "err", err, "group", gid)
		return
	}
	frame, err := json.Marshal(protocol.Event(ev))
	if err != nil {
		s.log.Error("marshal mute event", "err", err)
		return
	}
	for _, uid := range users {
		s.router.DeliverToUser(uid, frame, "")
	}
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

// ToProto 是下行版序列化（和 sync、msg 帧相同：撤回的消息 deleted=true）。后台正文接口用 ToAdminProto（RC2-21）。
func ToProto(m store.Message) protocol.Message { return toProto(m) }
