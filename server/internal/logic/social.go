package logic

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/social"
	"tianting/server/internal/store"
)

// 社交命令（PRD-social M3，BE-106～BE-113）。判定顺序统一为 5.3 节：格式 → 限流 / 配额 → 存在性 → 关系 → 数量上限。
// 日志不记录附言、备注、搜索关键字（F-SOC-02 规则 7、F-SOC-09 规则 6）；错误 msg 不拼入用户输入。

// retention 是申请记录和墓碑的保留期（5.1 节固定值）。
const retention = 30 * 24 * time.Hour

// socialState 是社交相关的配置缓存和限流器（5.2 节）。
type socialState struct {
	settings *social.Cache

	friendApply *ratelimit.TokenBucket // 每秒 1、突发 5
	search      *ratelimit.TokenBucket // 每分钟 10、突发 10
	users       *ratelimit.TokenBucket // 每分钟 30、突发 30（勘误 E-24：按 AC-SOC-10-3）
	ops         *ratelimit.TokenBucket // social:{uid}，每秒 5、突发 20
	full        *ratelimit.TokenBucket // socialFull:{uid}，每分钟 10 轮、突发 10（勘误 E-24：按 AC-SOC-06-13）
	groupRead   *ratelimit.TokenBucket // 每秒 10、突发 30

	// 随配置变化的配额：配置变化时 Reconfigure（立即生效，5.2 节规则 3）。
	mu             sync.Mutex
	applied        social.Settings
	applyDay       *ratelimit.TokenBucket // friendApplyDay:{uid}
	applyTargetDay *ratelimit.TokenBucket // friendApplyDay:{uid}:{peer}
	blockDay       *ratelimit.TokenBucket
	searchDay      *ratelimit.TokenBucket
	newPeerHour    *ratelimit.TokenBucket
	newPeerReject  *ratelimit.TokenBucket

	cursorKey []byte // 同步游标的 HMAC 密钥，启动时生成
}

func newSocialState(st store.Store, clk clock.Clock, inv cache.Invalidator) *socialState {
	d := social.Defaults()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return &socialState{
		settings:       social.NewCache(st, clk, inv),
		friendApply:    ratelimit.New(1, 5, clk),
		search:         ratelimit.NewPer(6*time.Second, 10, clk),
		users:          ratelimit.NewPer(2*time.Second, 30, clk),
		ops:            ratelimit.New(5, 20, clk),
		full:           ratelimit.NewPer(6*time.Second, 10, clk),
		groupRead:      ratelimit.New(10, 30, clk),
		applied:        d,
		applyDay:       ratelimit.NewDaily(d.ApplyDailyLimit, clk),
		applyTargetDay: ratelimit.NewDaily(d.ApplyPerTargetDailyLimit, clk),
		blockDay:       ratelimit.NewDaily(d.BlockDailyLimit, clk),
		searchDay:      ratelimit.NewDaily(d.SearchDailyLimit, clk),
		newPeerHour:    ratelimit.NewHourly(max(d.NewPeerHourlyLimit, 1), clk),
		newPeerReject:  ratelimit.NewDaily(d.NewPeerRejectDailyLimit, clk),
		cursorKey:      key,
	}
}

func (ss *socialState) buckets() []*ratelimit.TokenBucket {
	return []*ratelimit.TokenBucket{ss.friendApply, ss.search, ss.users, ss.ops, ss.full, ss.groupRead,
		ss.applyDay, ss.applyTargetDay, ss.blockDay, ss.searchDay, ss.newPeerHour, ss.newPeerReject}
}

// apply 把配置里的配额同步到限流器（只在值变化时 Reconfigure）。
func (ss *socialState) apply(s social.Settings) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	old := ss.applied
	if old == s {
		return
	}
	daily := func(b *ratelimit.TokenBucket, o, n int) {
		if o != n {
			b.Reconfigure(24*time.Hour/time.Duration(max(n, 1)), max(n, 1))
		}
	}
	daily(ss.applyDay, old.ApplyDailyLimit, s.ApplyDailyLimit)
	daily(ss.applyTargetDay, old.ApplyPerTargetDailyLimit, s.ApplyPerTargetDailyLimit)
	daily(ss.blockDay, old.BlockDailyLimit, s.BlockDailyLimit)
	daily(ss.searchDay, old.SearchDailyLimit, s.SearchDailyLimit)
	daily(ss.newPeerReject, old.NewPeerRejectDailyLimit, s.NewPeerRejectDailyLimit)
	if old.NewPeerHourlyLimit != s.NewPeerHourlyLimit {
		n := max(s.NewPeerHourlyLimit, 1) // 0 表示不检查，桶照常保留
		ss.newPeerHour.Reconfigure(time.Hour/time.Duration(n), n)
	}
	ss.applied = s
}

// socialSettings 读配置（缓存），并把配额同步到限流器。
func (s *Service) socialSettings(ctx context.Context) (social.Settings, *protocol.Error) {
	cfg, err := s.social.settings.Get(ctx)
	if err != nil {
		return social.Settings{}, s.internal("load social settings", err)
	}
	s.social.apply(cfg)
	return cfg, nil
}

func (s *Service) nowMs() int64 { return s.clock.Now().UnixMilli() }

func (s *Service) cutoff() int64 { return s.clock.Now().Add(-retention).UnixMilli() }

func rateLimited() *protocol.Error {
	return protocol.Errorf(protocol.CodeRateLimited, "too many requests")
}

func quotaExceeded() *protocol.Error {
	return protocol.Errorf(protocol.CodeQuotaExceeded, "daily quota exceeded")
}

// trimmedText 去首尾空白后按码点检查长度。
func trimmedText(p *string, maxLen int, field string) (string, *protocol.Error) {
	if p == nil {
		return "", nil
	}
	v := protocol.TrimText(*p)
	if protocol.TextLen(v) > maxLen {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s longer than %d", field, maxLen)
	}
	return v, nil
}

// handleSocial 分发社交命令；不是社交命令时 ok=false。
func (s *Service) handleSocial(ctx context.Context, c delivery.Session, in protocol.InFrame) (data any, err *protocol.Error, ok bool) {
	switch in.Cmd {
	case protocol.CmdFriendApply:
		data, err = s.friendApply(ctx, c, in.Data)
	case protocol.CmdFriendRespond:
		data, err = s.friendRespond(ctx, c, in.Data)
	case protocol.CmdFriendDelete:
		data, err = s.friendDelete(ctx, c, in.Data)
	case protocol.CmdFriendRemark:
		data, err = s.friendRemark(ctx, c, in.Data)
	case protocol.CmdFriendRequestsSeen:
		data, err = s.friendRequestsSeen(ctx, c, in.Data)
	case protocol.CmdSocialSync:
		data, err = s.socialSync(ctx, c, in.Data)
	case protocol.CmdBlock:
		data, err = s.block(ctx, c, in.Data, true)
	case protocol.CmdUnblock:
		data, err = s.block(ctx, c, in.Data, false)
	case protocol.CmdUserSearch:
		data, err = s.userSearch(ctx, c, in.Data)
	case protocol.CmdSetPrivacy:
		data, err = s.setPrivacy(ctx, c, in.Data)
	case protocol.CmdUsers:
		data, err = s.usersCmd(ctx, c, in.Data)
	case protocol.CmdMyGroups:
		data, err = s.myGroups(ctx, c, in.Data)
	default:
		return nil, nil, false
	}
	return data, err, true
}

// ---- 好友申请（F-SOC-02） ----

func (s *Service) friendApply(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.FriendApplyReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.To) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid to")
	}
	if req.To == uid {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "cannot add yourself")
	}
	msg, perr := trimmedText(req.Message, protocol.MaxApplyMessageLen, "message")
	if perr != nil {
		return nil, perr
	}
	remark, perr := trimmedText(req.Remark, protocol.MaxRemarkLen, "remark")
	if perr != nil {
		return nil, perr
	}
	source := protocol.SourceOther
	if req.Source != nil {
		source = *req.Source
	}
	if !protocol.ValidApplySource(source) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid source")
	}
	sourceRef := ""
	if source == protocol.SourceGroup {
		if req.SourceRef == nil || !protocol.ValidID(*req.SourceRef) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "source=group requires sourceRef=groupId")
		}
		sourceRef = *req.SourceRef
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	// 限流 → 两个每日配额（被拒绝的那一项不扣；排在前面已通过的照常扣，5.2 节规则 1）。
	if !s.social.friendApply.Allow(uid) {
		return nil, rateLimited()
	}
	if !s.social.applyDay.Allow(uid) || !s.social.applyTargetDay.Allow(uid+":"+req.To) {
		return nil, quotaExceeded()
	}
	now := s.clock.Now()
	status, ch, err := s.store.ApplyFriend(ctx, store.ApplyFriendParams{
		From: uid, To: req.To, Message: msg, Source: source, SourceRef: sourceRef, Remark: remark,
		Now: now.UnixMilli(), ExpireAt: now.Add(time.Duration(cfg.RequestExpireDays) * 24 * time.Hour).UnixMilli(),
		MaxFriends: cfg.MaxFriends,
	})
	switch {
	case errors.Is(err, store.ErrIBlockedPeer):
		return nil, protocol.Errorf(protocol.CodeBadRequest, "unblock the user before sending a friend request")
	case errors.Is(err, store.ErrFriendLimit):
		return nil, protocol.Errorf(protocol.CodeFriendLimitExceeded, "friend limit exceeded")
	case errors.Is(err, store.ErrPeerFriendLimit):
		return nil, protocol.Errorf(protocol.CodePeerFriendLimitExceeded, "peer friend limit exceeded")
	case err != nil:
		return nil, s.internal("apply friend", err)
	}
	s.log.Debug("friend apply", "messageLen", protocol.TextLen(msg), "remarkLen", protocol.TextLen(remark), "status", status, "changed", ch.Changed)
	return s.changeAck(ctx, c, ch, status)
}

// changeAck 下发 socialChanged 并组装修改类 ack（BE-106、BE-107、BE-110 共用）。
func (s *Service) changeAck(ctx context.Context, c delivery.Session, ch store.SocialChange, status string) (any, *protocol.Error) {
	s.NotifySocialChanged(ch.Bumped, c.UserID(), c.ConnID())
	items, perr := s.toItems(ctx, c.UserID(), ch.Items)
	if perr != nil {
		return nil, perr
	}
	return protocol.SocialChangeAck{Status: status, Ver: ch.Ver, Changed: ch.Changed, Items: items}, nil
}

// NotifySocialChanged 给 ver 变化了的用户下发 socialChanged {ver}（5.4 节规则 2）：发起操作的连接（actor 的 exceptConn）不收。
// 管理接口调用时 actor 为空，全部连接都收。
func (s *Service) NotifySocialChanged(bumped map[string]int64, actor, exceptConn string) {
	for uid, ver := range bumped {
		if s.faults.takeDropEvent(uid, protocol.EventSocialChanged) {
			continue
		}
		except := ""
		if uid == actor {
			except = exceptConn
		}
		s.pushEvent(uid, protocol.SocialChangedEvent{Type: protocol.EventSocialChanged, Ver: ver}, except)
	}
}

// toItems 把存储层的行转成协议同步项，并按 F-SOC-06 规则 9 带资料。
func (s *Service) toItems(ctx context.Context, uid string, it store.SocialItems) (protocol.SocialItems, *protocol.Error) {
	out := protocol.SocialItems{Friends: []protocol.FriendItem{}, Requests: []protocol.RequestItem{}, Blocks: []protocol.BlockItem{}}
	var ids, blockPeers []string
	for _, f := range it.Friends {
		ids = append(ids, f.Peer)
	}
	for _, r := range it.Requests {
		if r.Direction == store.DirIn || r.Status == store.ReqAccepted {
			ids = append(ids, r.Peer)
		}
	}
	for _, b := range it.Blocks {
		blockPeers = append(blockPeers, b.Peer)
	}
	profiles, err := s.store.Profiles(ctx, ids)
	if err != nil {
		return out, s.internal("load profiles", err)
	}
	visible := map[string]store.User{}
	if len(blockPeers) > 0 {
		vs, err := s.store.VisibleUsers(ctx, uid, blockPeers, s.cutoff())
		if err != nil {
			return out, s.internal("visible users", err)
		}
		for _, u := range vs {
			visible[u.UserID] = u
		}
	}
	prof := func(m map[string]store.User, id string) *protocol.Profile {
		u, ok := m[id]
		if !ok {
			return nil
		}
		return &protocol.Profile{Nickname: u.Nickname, Avatar: u.Avatar, Official: u.Official}
	}
	now := s.nowMs()
	for _, f := range it.Friends {
		out.Friends = append(out.Friends, protocol.FriendItem{UserID: f.Peer, Remark: f.Remark, Source: f.Source, CreatedAt: f.CreatedAt,
			Ver: f.Ver, Deleted: f.Deleted, Profile: prof(profiles, f.Peer)})
	}
	for _, r := range it.Requests {
		status := r.Status
		if status != store.ReqAccepted && r.ExpireAt <= now {
			status = protocol.StatusExpired
		}
		var p *protocol.Profile
		if r.Direction == store.DirIn || r.Status == store.ReqAccepted {
			p = prof(profiles, r.Peer)
		}
		out.Requests = append(out.Requests, protocol.RequestItem{UserID: r.Peer, Direction: r.Direction, Status: status, Message: r.Message,
			Source: r.Source, ApplyCount: r.ApplyCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ExpireAt: r.ExpireAt, Ver: r.Ver, Profile: p})
	}
	for _, b := range it.Blocks {
		out.Blocks = append(out.Blocks, protocol.BlockItem{UserID: b.Peer, CreatedAt: b.CreatedAt, Ver: b.Ver, Deleted: b.Deleted, Profile: prof(visible, b.Peer)})
	}
	return out, nil
}

// ---- 处理申请、删除、备注（F-SOC-03、F-SOC-05） ----

func (s *Service) friendRespond(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.FriendRespondReq](raw)
	if perr != nil {
		return nil, perr
	}
	if !protocol.ValidID(req.From) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid from")
	}
	if req.Action != "accept" && req.Action != "reject" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "action must be accept or reject")
	}
	remark, perr := trimmedText(req.Remark, protocol.MaxRemarkLen, "remark")
	if perr != nil {
		return nil, perr
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	if !s.social.ops.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	ch, err := s.store.RespondFriend(ctx, store.RespondFriendParams{
		Me: c.UserID(), From: req.From, Accept: req.Action == "accept", Remark: remark, Now: s.nowMs(), MaxFriends: cfg.MaxFriends,
	})
	switch {
	case errors.Is(err, store.ErrRequestNotFound):
		// 不存在、已过期、申请时被我拉黑过：同一个响应体（F-SOC-03 规则 6）。
		return nil, protocol.Errorf(protocol.CodeFriendRequestNotFound, "friend request not found or expired")
	case errors.Is(err, store.ErrRequestHandled):
		return nil, protocol.Errorf(protocol.CodeFriendRequestHandled, "already friends; delete the friend instead")
	case errors.Is(err, store.ErrFriendLimit):
		return nil, protocol.Errorf(protocol.CodeFriendLimitExceeded, "friend limit exceeded")
	case errors.Is(err, store.ErrPeerFriendLimit):
		return nil, protocol.Errorf(protocol.CodePeerFriendLimitExceeded, "peer friend limit exceeded")
	case err != nil:
		return nil, s.internal("respond friend", err)
	}
	return s.changeAck(ctx, c, ch, "")
}

func (s *Service) friendDelete(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.UserIDReq](raw)
	if perr != nil {
		return nil, perr
	}
	if !protocol.ValidID(req.UserID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
	}
	if !s.social.ops.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	ch, err := s.store.DeleteFriend(ctx, c.UserID(), req.UserID, s.nowMs())
	if err != nil {
		return nil, s.internal("delete friend", err)
	}
	return s.changeAck(ctx, c, ch, "")
}

func (s *Service) friendRemark(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.FriendRemarkReq](raw)
	if perr != nil {
		return nil, perr
	}
	if !protocol.ValidID(req.UserID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
	}
	if req.Remark == nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "remark is required")
	}
	remark, perr := trimmedText(req.Remark, protocol.MaxRemarkLen, "remark")
	if perr != nil {
		return nil, perr
	}
	if !s.social.ops.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	ch, err := s.store.SetRemark(ctx, c.UserID(), req.UserID, remark, s.nowMs())
	if errors.Is(err, store.ErrNotFriend) {
		return nil, protocol.Errorf(protocol.CodeNotFound, "not a friend") // 不存在的人同样 4040（5.3 节）
	}
	if err != nil {
		return nil, s.internal("set remark", err)
	}
	return s.changeAck(ctx, c, ch, "")
}

// ---- 红点（F-SOC-04） ----

func (s *Service) friendRequestsSeen(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.RequestsSeenReq](raw)
	if perr != nil {
		return nil, perr
	}
	if req.Ver == nil || *req.Ver < 0 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "ver must be >= 0")
	}
	if !s.social.ops.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	seen, ch, err := s.store.SetRequestSeen(ctx, c.UserID(), *req.Ver, s.nowMs())
	if err != nil {
		return nil, s.internal("set request seen", err)
	}
	s.NotifySocialChanged(ch.Bumped, c.UserID(), c.ConnID())
	return protocol.RequestsSeenAck{RequestSeenVer: seen, Ver: ch.Ver, Changed: ch.Changed}, nil
}

// ---- 关系同步（F-SOC-06） ----

// syncCursor 是同步游标的内容（不透明：base64 JSON + HMAC，绑定 uid）。
type syncCursor struct {
	UID   string `json:"u"`
	Full  bool   `json:"f"`
	Since int64  `json:"s"`
	Until int64  `json:"t"`
	Ver   int64  `json:"v"`
	Type  int    `json:"k"`
	Peer  string `json:"p"`
	Dir   string `json:"d,omitempty"`
}

func (s *Service) encodeCursor(c syncCursor) string {
	b, _ := json.Marshal(c)
	mac := hmac.New(sha256.New, s.social.cursorKey)
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) decodeCursor(raw, uid string) (syncCursor, bool) {
	var c syncCursor
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return c, false
	}
	b, err1 := base64.RawURLEncoding.DecodeString(body)
	want, err2 := base64.RawURLEncoding.DecodeString(sig)
	if err1 != nil || err2 != nil {
		return c, false
	}
	mac := hmac.New(sha256.New, s.social.cursorKey)
	mac.Write(b)
	if !hmac.Equal(mac.Sum(nil), want) || json.Unmarshal(b, &c) != nil || c.UID != uid {
		return c, false
	}
	return c, true
}

func (s *Service) socialSync(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.SocialSyncReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if (req.SinceVer == nil) == (req.Cursor == nil) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "exactly one of sinceVer and cursor is required")
	}
	limit := protocol.DefaultSocialLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > protocol.MaxSocialLimit {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1-%d", protocol.MaxSocialLimit)
	}
	q := store.SocialPageQuery{UID: uid, Limit: limit, RequestCutoff: s.cutoff()}
	if req.SinceVer != nil {
		if *req.SinceVer < 0 {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "sinceVer must be >= 0")
		}
		q.First, q.SinceVer = true, *req.SinceVer
	} else {
		cur, ok := s.decodeCursor(*req.Cursor, uid)
		if !ok {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
		}
		q.Full, q.SinceVer, q.UntilVer = cur.Full, cur.Since, cur.Until
		q.After = &store.SortKey{Ver: cur.Ver, Type: cur.Type, Peer: cur.Peer, Dir: cur.Dir}
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	page, err := s.store.SocialPage(ctx, q)
	if err != nil {
		return nil, s.internal("social page", err)
	}
	// 全量轮在第一页计数（含服务端判定的全量）；被拒绝不扣（结果丢弃）。
	if q.First && page.Full && !s.social.full.Allow(uid) {
		return nil, rateLimited()
	}
	searchable := cfg.SearchableDefault
	if page.State.Searchable != nil {
		searchable = *page.State.Searchable
	}
	out := protocol.SocialSyncAck{
		Full: page.Full, UntilVer: page.UntilVer, LatestVer: page.State.Ver, HasMore: page.HasMore, Restart: page.Restart,
		RequestSeenVer: page.State.RequestSeenVer, Settings: protocol.PrivacySettings{Searchable: searchable},
		Mode: cfg.Mode, StrangerMsgLimit: cfg.StrangerMsgLimit, Official: page.State.Official,
	}
	items, perr := s.toItems(ctx, uid, page.Items)
	if perr != nil {
		return nil, perr
	}
	out.Friends, out.Requests, out.Blocks = items.Friends, items.Requests, items.Blocks
	if page.HasMore && page.Last != nil {
		out.Cursor = s.encodeCursor(syncCursor{UID: uid, Full: page.Full, Since: page.SinceVer, Until: page.UntilVer,
			Ver: page.Last.Ver, Type: page.Last.Type, Peer: page.Last.Peer, Dir: page.Last.Dir})
	}
	return out, nil
}

// ---- 拉黑（F-SOC-07） ----

func (s *Service) block(ctx context.Context, c delivery.Session, raw json.RawMessage, on bool) (any, *protocol.Error) {
	req, perr := decode[protocol.UserIDReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.UserID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
	}
	if req.UserID == uid {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "cannot block yourself")
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	if !s.social.ops.Allow(uid) {
		return nil, rateLimited()
	}
	if !s.social.blockDay.Allow(uid) {
		return nil, quotaExceeded()
	}
	var ch store.SocialChange
	var err error
	if on {
		ch, err = s.store.Block(ctx, uid, req.UserID, s.nowMs(), cfg.MaxBlocks)
	} else {
		ch, err = s.store.Unblock(ctx, uid, req.UserID, s.nowMs())
	}
	if errors.Is(err, store.ErrBlockLimit) {
		return nil, protocol.Errorf(protocol.CodeBlockLimitExceeded, "block list is full")
	}
	if err != nil {
		return nil, s.internal("block", err)
	}
	return s.changeAck(ctx, c, ch, "")
}

// ---- 搜索、隐私（F-SOC-09） ----

func (s *Service) userSearch(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.UserSearchReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	kw := protocol.TrimText(req.Keyword)
	if !protocol.ValidID(kw) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "keyword must be a userId")
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	if !s.social.search.Allow(uid) {
		return nil, rateLimited()
	}
	if !s.social.searchDay.Allow(uid) {
		return nil, quotaExceeded()
	}
	// 每次都查用户、隐私和关系，再统一判断（规则 3：不存在 / 不可被搜索的应答与耗时都不可区分）。
	u, exists, searchable, rel, err := s.store.SearchUser(ctx, uid, kw, s.nowMs())
	if err != nil {
		return nil, s.internal("search user", err)
	}
	visible := exists && (kw == uid || cfg.SearchableDefault && searchable == nil || searchable != nil && *searchable)
	s.log.Debug("user search", "hit", visible)
	if !visible {
		return protocol.UserSearchAck{}, nil
	}
	relation := protocol.RelNone
	switch {
	case kw == uid:
		relation = protocol.RelSelf
	case rel.IBlocked:
		relation = protocol.RelBlocked
	case rel.IFriend:
		relation = protocol.RelFriend
	case rel.PendingIn:
		relation = protocol.RelPendingIn
	case rel.PendingOut:
		relation = protocol.RelPendingOut
	}
	return protocol.UserSearchAck{User: &protocol.SearchUser{UserID: kw, Nickname: u.Nickname, Avatar: u.Avatar, Official: u.Official, Relation: relation}}, nil
}

func (s *Service) setPrivacy(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.SetPrivacyReq](raw)
	if perr != nil {
		return nil, perr
	}
	if req.Searchable == nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "searchable is required")
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	if !s.social.ops.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	ch, err := s.store.SetSearchable(ctx, c.UserID(), *req.Searchable, cfg.SearchableDefault, s.nowMs())
	if err != nil {
		return nil, s.internal("set searchable", err)
	}
	s.NotifySocialChanged(ch.Bumped, c.UserID(), c.ConnID())
	return protocol.VerAck{Ver: ch.Ver, Changed: ch.Changed}, nil
}

// ---- 资料、我的群聊（F-SOC-10、F-SOC-11） ----

func (s *Service) usersCmd(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.UsersReq](raw)
	if perr != nil {
		return nil, perr
	}
	if len(req.UserIDs) == 0 || len(req.UserIDs) > protocol.MaxUsersBatch {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "userIds must have 1-%d items", protocol.MaxUsersBatch)
	}
	for _, id := range req.UserIDs {
		if !protocol.ValidID(id) {
			return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
		}
	}
	if !s.social.users.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	ids := slices.Compact(slices.Sorted(slices.Values(req.UserIDs)))
	us, err := s.store.VisibleUsers(ctx, c.UserID(), ids, s.cutoff())
	if err != nil {
		return nil, s.internal("visible users", err)
	}
	out := protocol.UsersAck{Users: make([]protocol.UserProfile, 0, len(us))}
	for _, u := range us {
		out.Users = append(out.Users, protocol.UserProfile{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar, Official: u.Official})
	}
	return out, nil
}

func (s *Service) myGroups(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	req, perr := decode[protocol.MyGroupsReq](raw)
	if perr != nil {
		return nil, perr
	}
	limit := protocol.DefaultGroupsLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > protocol.MaxGroupsLimit {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1-%d", protocol.MaxGroupsLimit)
	}
	after := ""
	if req.Cursor != nil && *req.Cursor != "" {
		if !protocol.ValidID(*req.Cursor) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
		}
		after = *req.Cursor
	}
	if !s.social.groupRead.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	gs, more, err := s.store.MyGroups(ctx, c.UserID(), after, limit)
	if err != nil {
		return nil, s.internal("my groups", err)
	}
	out := protocol.MyGroupsAck{Groups: make([]protocol.MyGroupItem, 0, len(gs))}
	for _, g := range gs {
		out.Groups = append(out.Groups, protocol.MyGroupItem{GroupID: g.GroupID, Name: g.Name, MemberCount: g.MemberCount, JoinedAt: g.JoinedAt})
	}
	if more {
		out.NextCursor = gs[len(gs)-1].GroupID
	}
	return out, nil
}

// ---- 私聊权限（F-SOC-08 规则 7 第 4～9 步） ----

// c2cPermission 在发送限流之后、保存之前判定单聊权限，返回 SaveMessage 的陌生人限额参数，
// 以及保存成功后要调用的 onSaved（退还 newPeerReject 预扣的令牌）。to == uid 时不调用。
func (s *Service) c2cPermission(ctx context.Context, uid, to, convID string) (strangerLimit int, onSaved func(), perr *protocol.Error) {
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return 0, nil, perr
	}
	rel, err := s.store.Relation(ctx, uid, to, convID) // L7：普通读、不缓存
	if err != nil {
		return 0, nil, s.internal("load relation", err)
	}
	newPeer := !rel.HasMessages
	onSaved = func() {}
	if newPeer {
		// 4：预扣 1 个 newPeerReject，扣不到 4292；保存成功后退还，静默失败（4042 / 4035 / 4036）不退。
		if !s.social.newPeerReject.Allow(uid) {
			return 0, nil, quotaExceeded()
		}
		onSaved = func() { s.social.newPeerReject.Refund(uid) }
		// 5：newPeerHour（官方账号作为发送方豁免）；被它拒绝不是静默失败，退还第 4 步的预扣。
		if cfg.NewPeerHourlyLimit > 0 && !rel.MeOfficial && !s.social.newPeerHour.Allow(uid) {
			s.social.newPeerReject.Refund(uid)
			return 0, nil, quotaExceeded()
		}
	}
	mutual := rel.MeToPeer && rel.PeerToMe
	official := rel.MeOfficial || rel.PeerOfficial
	switch {
	case !rel.PeerExists: // 6（D5）
		return 0, nil, protocol.Errorf(protocol.CodeUserNotFound, "user not found")
	case cfg.Mode == social.ModeFriends && !mutual && !official: // 7
		return 0, nil, protocol.Errorf(protocol.CodeNotFriend, "not friends")
	case rel.PeerBlocksMe: // 8
		return 0, nil, protocol.Errorf(protocol.CodeRejectedByPeer, "rejected by peer")
	}
	if cfg.Mode == social.ModeOpen && cfg.StrangerMsgLimit > 0 && !mutual && !official {
		strangerLimit = cfg.StrangerMsgLimit // 9：在 SaveMessage 事务内判定
	}
	return strangerLimit, onSaved, nil
}

// ---- 管理接口与测试接口用（admin 包经接口调用） ----

// SocialSettings 返回当前配置（GET /admin/settings/social）。
func (s *Service) SocialSettings(ctx context.Context) (social.Settings, *protocol.Error) {
	return s.socialSettings(ctx)
}

// PatchSocialSettings 校验并保存配置（整次原子），通知各节点失效，给全部在线连接下发 socialSettingsChanged。
func (s *Service) PatchSocialSettings(ctx context.Context, patch []byte) (old, cur social.Settings, perr *protocol.Error) {
	var verr error
	err := s.store.UpdateSocialSettings(ctx, s.nowMs(), func(raw string) (string, error) {
		base, err := social.Parse(raw)
		if err != nil {
			return "", err
		}
		next, err := social.Patch(base, patch)
		if err != nil {
			verr = err
			return "", err
		}
		old, cur = base, next
		b, _ := json.Marshal(next)
		return string(b), nil
	})
	if verr != nil {
		return old, cur, protocol.Errorf(protocol.CodeBadRequest, "%v", verr)
	}
	if err != nil {
		return old, cur, s.internal("update social settings", err)
	}
	s.inv.Invalidate(social.TopicSettings, "")
	s.social.apply(cur)
	if b, ok := s.router.(delivery.Broadcaster); ok {
		frame, err := json.Marshal(protocol.Event(protocol.SocialSettingsChangedEvent{
			Type: protocol.EventSocialSettingsChanged, Mode: cur.Mode, StrangerMsgLimit: cur.StrangerMsgLimit}))
		if err == nil {
			b.DeliverToAll(frame)
		}
	}
	return old, cur, nil
}

// PurgeSocialNow 按当前（测试）时钟执行一次清理任务（F-SOC-06 规则 5）。
func (s *Service) PurgeSocialNow(ctx context.Context) (int, error) {
	return s.store.PurgeSocial(ctx, s.cutoff())
}

// RunSocialPurge 每小时执行一次清理任务，直到 stop 关闭（单节点；集群化时改为只在一个节点上跑，记入 ARCH）。
func (s *Service) RunSocialPurge(stop <-chan struct{}) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if n, err := s.PurgeSocialNow(context.Background()); err != nil {
				s.log.Error("purge social", "err", err)
			} else if n > 0 {
				s.log.Info("purged social rows", "rows", n)
			}
		}
	}
}

// ResetLimits 清空某用户的全部令牌桶（/admin/test/limits/reset）：发送限流和 5.2 节全部键。
func (s *Service) ResetLimits(uid string) {
	if r, ok := s.limiter.(interface{ ResetUser(string) }); ok {
		r.ResetUser(uid)
	}
	for _, b := range s.social.buckets() {
		b.ResetUser(uid)
	}
}

// DropEvents 是测试接口 /admin/test/social/drop-events。
func (s *Service) DropEvents(uid, typ string, count int) { s.faults.DropEvents(uid, typ, count) }

// SocialMaxFriends 返回当前好友上限（管理接口导入用）。
func (s *Service) SocialMaxFriends(ctx context.Context) (int, *protocol.Error) {
	cfg, perr := s.socialSettings(ctx)
	return cfg.MaxFriends, perr
}

// SeedSocial 是测试接口 /admin/test/social/seed：写入后给 ver 变化的用户发 socialChanged。
func (s *Service) SeedSocial(ctx context.Context, sd store.SocialSeed) error {
	if sd.UpdatedAt == 0 {
		sd.UpdatedAt = s.nowMs()
	}
	bumped, err := s.store.SeedSocial(ctx, sd)
	if err != nil {
		return err
	}
	s.NotifySocialChanged(bumped, "", "")
	return nil
}
