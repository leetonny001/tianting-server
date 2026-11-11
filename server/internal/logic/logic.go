// Package logic 处理上行命令：发消息、补拉、已读。负责权限校验、落库和在线投递。
package logic

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/delivery"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// Service 实现 gateway.Handler。
type Service struct {
	store   store.Store
	router  delivery.Router
	log     *slog.Logger
	clock   clock.Clock
	faults  *Faults // 仅测试模式非 nil
	limiter ratelimit.Limiter
	members *memberCache
	social  *socialState
	inv     cache.Invalidator
	metrics *metrics.Metrics
	newID   func() string
	changed *groupChangedMerger
	mutes   *muteCache // 第二批开启时非 nil（IM_V13_ADMIN_FEATURES）
	push    *pushState // 系统推送开启时非 nil（IM_PUSH_ENABLED）
	files   filesConfig
	cvs     *convSettings // C5 置顶、免打扰、删除：一定非 nil，开关在 cvs.cfg.Enabled
	actions *msgActions   // C4 撤回、引用、@：一定非 nil，开关在 actions.cfg.Enabled
	mod     *modState     // C6 敏感词和举报：一定非 nil，开关在 mod.cfg.Enabled
}

// 配额、@ 所有人次数按 Asia/Shanghai 自然日。中国不使用夏令时，用固定时区，不依赖运行环境的 tzdata。
var shanghai = time.FixedZone("Asia/Shanghai", 8*3600)

// New 创建 Service。limiter 按用户限制发送速率（所有连接合计，PRD F-MSG-07）。
// inv 用于订阅群成员缓存的失效通知（TopicGroupMembers）。
func New(st store.Store, router delivery.Router, clk clock.Clock, faults *Faults, limiter ratelimit.Limiter, inv cache.Invalidator, m *metrics.Metrics, log *slog.Logger) *Service {
	s := &Service{
		store:   st,
		router:  router,
		log:     log,
		clock:   clk,
		faults:  faults,
		limiter: limiter,
		members: newMemberCache(st, clk, inv),
		social:  newSocialState(st, clk, inv),
		inv:     inv,
		metrics: m,
		// UUIDv7 按时间递增，作为主键插入时对 B 树更友好。
		newID: func() string { return uuid.Must(uuid.NewV7()).String() },
	}
	s.changed = newGroupChangedMerger(s.deliverGroupChanged)
	s.actions = newMsgActions(MsgActionsConfig{}, s)
	s.cvs = newConvSettings(ConvSettingsConfig{}, s)
	s.mod = &modState{cfg: ModConfig{}.WithDefaults()}
	return s
}

// Handle 分发一条上行帧，返回要回给客户端的 ack；返回 nil 表示不回复（仅故障注入时）。
func (s *Service) Handle(ctx context.Context, c delivery.Session, in protocol.InFrame) *protocol.OutFrame {
	if in.ID == "" {
		return ack(protocol.AckErr("", protocol.Errorf(protocol.CodeMissingRequestID, "missing id")))
	}
	var (
		data any
		err  *protocol.Error
	)
	switch in.Cmd {
	case protocol.CmdSend:
		var drop bool
		data, drop, err = s.send(ctx, c, in.Data)
		if drop {
			return nil
		}
	case protocol.CmdSync:
		data, err = s.sync(ctx, c, in.Data)
	case protocol.CmdRead:
		data, err = s.read(ctx, c, in.Data)
	case protocol.CmdConvs:
		data, err = s.convs(ctx, c)
	case protocol.CmdPushSync:
		data, err = s.pushSync(ctx, c, in.Data)
	case protocol.CmdRecall:
		data, err = s.recall(ctx, c, in.Data)
	case protocol.CmdConvSet:
		data, err = s.convSet(ctx, c, in.Data)
	case protocol.CmdConvDelete:
		data, err = s.convDelete(ctx, c, in.Data)
	case protocol.CmdReport:
		data, err = s.report(ctx, c, in.Data)
	case protocol.CmdJoinRoom, protocol.CmdLeaveRoom:
		err = protocol.Errorf(protocol.CodeBadRequest, "%s is not available yet", in.Cmd)
	default:
		var ok bool
		if data, err, ok = s.handleSocial(ctx, c, in); !ok {
			err = protocol.Errorf(protocol.CodeUnknownCmd, "unknown cmd %q", in.Cmd)
		}
	}
	if err != nil {
		return ack(protocol.AckErr(in.ID, err))
	}
	return ack(protocol.AckOK(in.ID, data))
}

func ack(f protocol.OutFrame) *protocol.OutFrame { return &f }

// send 返回 ack 数据；drop=true 表示按故障注入丢弃 ack。
func (s *Service) send(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, bool, *protocol.Error) {
	req, perr := decode[protocol.SendReq](raw)
	if perr != nil {
		return nil, false, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, false, perr
	}
	isFile := req.Type == protocol.MsgImage || req.Type == protocol.MsgFile
	if isFile && !s.files.enabled {
		// IM_FILES_ENABLED 关闭：4000，不是共同规则的 4030（F-FILE 的写法，AC-FILE-01-14）。
		return nil, false, protocol.Errorf(protocol.CodeBadRequest, "file messages are disabled")
	}
	// C4 的格式（第 4.3 节第 1 步）：开关关闭时带 quote、mentions 是 4030（第 4.1 节共同规则）；@ 的人数超限 4131（去重后，AC-MEN-01-3）。
	if req.Body.Quote != nil || req.Body.Mentions != nil {
		if !s.actions.cfg.Enabled {
			return nil, false, featureDisabled()
		}
		if m := req.Body.Mentions; m != nil && distinctCount(m.UserIDs) > s.actions.cfg.MentionMax {
			return nil, false, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d mentions", s.actions.cfg.MentionMax)
		}
	}
	uid := c.UserID()

	// 先识别重发（IMP-3）：命中时直接返回第一次的结果，不扣令牌、不重复投递。
	if prev, found, err := s.store.FindByClientMsgID(ctx, uid, req.ClientMsgID); err != nil {
		return nil, false, s.internal("find by clientMsgId", err)
	} else if found {
		if prev.ConvID != targetConvID(req, uid) {
			return nil, false, protocol.Errorf(protocol.CodeClientMsgIDConflict, "clientMsgId already used in another conversation")
		}
		return protocol.SendAck{MsgID: prev.MsgID, ConvID: prev.ConvID, Seq: prev.Seq, TS: prev.TS}, false, nil
	}
	// 禁言在限流之前（C-29）：4031 不扣令牌。
	if perr := s.checkMuted(ctx, uid, req); perr != nil {
		return nil, false, perr
	}
	// 限流（C-29）：被拒绝的请求不扣令牌。
	if !s.limiter.Allow(uid) {
		return nil, false, protocol.Errorf(protocol.CodeRateLimited, "too many messages")
	}

	var convID string
	var recipients, convUsers []string
	strangerLimit, onSaved, onReject := 0, func() {}, func() {}
	switch req.ConvType {
	case protocol.ConvC2C:
		convID = protocol.C2CConvID(uid, req.To)
		// 单聊权限（PRD-social F-SOC-08 规则 7）：新对象配额 → 对方存在（D5）→ 好友制 → 拉黑；给自己发永远允许。
		if req.To != uid {
			var perr *protocol.Error
			if strangerLimit, onSaved, onReject, perr = s.c2cPermission(ctx, uid, req.To, convID); perr != nil {
				return nil, false, perr
			}
		}
		recipients = []string{req.To}
		convUsers = []string{uid, req.To}
		if req.To == uid {
			convUsers = []string{uid}
		}
	case protocol.ConvGroup:
		members, perr := s.groupMembersFor(ctx, req.To, uid)
		if perr != nil {
			return nil, false, perr
		}
		convID = protocol.GroupConvID(req.To)
		recipients = members
	default:
		return nil, false, protocol.Errorf(protocol.CodeBadRequest, "rooms are not available yet")
	}

	// 第 6 步附件预检（PRD-chat 第 4.3 节）。第 6～10 步失败退还新对象配额（R-CB-20）。
	stored := req.Body
	var attach string
	if isFile {
		if perr := s.precheckFile(ctx, uid, req); perr != nil {
			onReject()
			return nil, false, perr
		}
		stored, attach = storedFileBody(req), req.Body.FileID()
	}
	// 第 7 步引用校验（4040 quoteUnavailable）、第 8 步 @ 所有人的权限和过滤 userIds（4030 mentionAllForbidden）。
	// 只保存 quote_msg_id，不把被引用的内容写进 body（AC-QUO-01-6）；mentions 保存过滤后的结果（下发要带，撤回时随正文清除）。
	var quoteID string
	if q := req.Body.Quote; q != nil {
		if perr := s.checkQuote(ctx, uid, convID, q.MsgID); perr != nil {
			onReject()
			return nil, false, perr
		}
		quoteID = q.MsgID
	}
	var mentions mentionPlan
	if m := req.Body.Mentions; m != nil {
		var perr *protocol.Error
		if mentions, perr = s.planMentions(ctx, uid, req.To, m, recipients); perr != nil {
			onReject()
			return nil, false, perr
		}
	}
	// 第 9 步敏感词（F-MOD-02 规则 5）：在已有的全部权限判定之后，被拒绝的用户探测不到词库；已扣发送令牌，第 6～10 步失败退还新对象配额
	// （AC-MOD-02-12）；@ 所有人次数在保存事务里才扣，这里拦下就不扣（AC-MOD-02-16）。
	if perr := s.checkSendContent(ctx, uid, req); perr != nil {
		onReject()
		return nil, false, perr
	}
	if req.Type == protocol.MsgText {
		stored = protocol.MsgBody{Text: req.Body.Text, Mentions: mentions.body}
	}
	now := s.clock.Now()
	body, err := json.Marshal(stored)
	if err != nil {
		return nil, false, s.internal("marshal body", err)
	}
	saved, dup, err := s.store.SaveMessage(ctx, store.Message{
		MsgID:         s.newID(),
		ClientMsgID:   req.ClientMsgID,
		ConvID:        convID,
		From:          uid,
		Type:          req.Type,
		Body:          body,
		TS:            now.UnixMilli(),
		Summary:       protocol.Summary(req.Body.Text), // custom、image、file 没有文本摘要（lastMsg.text=""，不含文件名），客户端按 type 显示
		ConvUsers:     convUsers,
		StrangerLimit: strangerLimit,
		// 第 10 步：同一事务里绑定文件，影响 0 行 4096，消息不保存，文件保持 ready（AC-FILE-07-8）。
		AttachFileID:      attach,
		AttachRetainUntil: s.retainUntil(now),
		// 第 10 步（C4）：@ 所有人次数在事务里扣，提及计数和消息同一事务（R-CB-07、F-MEN-03 规则 1）。
		QuoteMsgID: quoteID,
		Mentions:   mentions.users,
		MentionAll: mentions.all,
		Fault:      s.mentionFault(),
	})
	var limited *store.MentionAllLimitError
	switch {
	case errors.Is(err, store.ErrStrangerMsgLimit):
		onReject()
		return nil, false, protocol.Errorf(protocol.CodeStrangerMsgLimit, "stranger message limit reached")
	case errors.Is(err, store.ErrFileNotReady):
		onReject()
		return nil, false, protocol.Errorf(protocol.CodeFileNotReady, "file not ready")
	case errors.As(err, &limited):
		onReject()
		e := protocol.Errorf(protocol.CodeMentionAllLimited, "mention-all limit reached")
		e.Details = map[string]any{"retryAfterSeconds": limited.RetryAfter}
		return nil, false, e
	case errors.Is(err, store.ErrMentionAllCounter):
		onReject()
		return nil, false, protocol.Errorf(protocol.CodeServiceUnavailable, "mention-all counter unavailable") // 不放行（AC-MEN-02-8）
	case errors.Is(err, store.ErrMentionWrite):
		onReject()
		return nil, false, s.internal("save message", err) // 提及表写失败（AC-MEN-03-8）：消息不保存，5000
	case err != nil:
		return nil, false, s.internal("save message", err)
	}
	onSaved()
	if dup && saved.ConvID != convID {
		return nil, false, protocol.Errorf(protocol.CodeClientMsgIDConflict, "clientMsgId already used in another conversation")
	}
	// 重发的消息第一次已经投递过，不再投递；没收到的端会靠 sync 补齐。
	if !dup {
		s.metrics.Messages.WithLabelValues(req.ConvType).Inc()
	}
	if !dup && !s.faults.takeSkipDelivery(req.ClientMsgID) {
		s.fanout(ctx, saved, recipients, c.ConnID())
		// 发言即已读：存储层已经把发送者的已读位置推进到这条消息，通知发送者的其他端（BE-042）。
		s.pushReadUpdated(uid, saved.ConvID, saved.Seq, c.ConnID())
	}
	if s.faults.takeDropAck(req.ClientMsgID) {
		return nil, true, nil
	}
	return protocol.SendAck{MsgID: saved.MsgID, ConvID: saved.ConvID, Seq: saved.Seq, TS: saved.TS}, false, nil
}

// targetConvID 计算请求要发往的会话 ID（聊天室在二期实现）。
func targetConvID(req protocol.SendReq, uid string) string {
	switch req.ConvType {
	case protocol.ConvC2C:
		return protocol.C2CConvID(uid, req.To)
	case protocol.ConvGroup:
		return protocol.GroupConvID(req.To)
	}
	return protocol.RoomConvID(req.To)
}

// groupMembersFor 返回群成员（走缓存）；群不存在返回 4043，uid 不是成员返回 4033。
func (s *Service) groupMembersFor(ctx context.Context, gid, uid string) ([]string, *protocol.Error) {
	e, err := s.members.get(ctx, gid)
	if err != nil {
		return nil, s.internal("load group members", err)
	}
	if !e.exists {
		return nil, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", gid)
	}
	if !e.has(uid) {
		return nil, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of group %s", gid)
	}
	return e.members, nil
}

// fanout 投递给在线的接收者，以及发送者自己的其他设备。离线用户不处理，上线后 sync。
func (s *Service) fanout(ctx context.Context, m store.Message, recipients []string, senderConn string) {
	pm := []protocol.Message{toProto(m)}
	if err := EnrichFiles(ctx, s.store, pm); err != nil {
		s.log.Error("load files for msg frame", "err", err)
	}
	if err := EnrichQuotes(ctx, s.store, pm); err != nil {
		s.log.Error("load quotes for msg frame", "err", err)
	}
	frame, err := json.Marshal(protocol.OutFrame{Cmd: protocol.CmdMsg, Data: pm[0]})
	if err != nil {
		s.log.Error("marshal msg frame", "err", err)
		return
	}
	for _, uid := range recipients {
		if uid != m.From {
			s.router.DeliverToUser(uid, frame, "")
		}
	}
	s.router.DeliverToUser(m.From, frame, senderConn)
}

func (s *Service) sync(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.SyncReq](raw)
	if perr != nil {
		return nil, perr
	}
	limit, perr := req.Validate()
	if perr != nil {
		return nil, perr
	}
	minSeq, perr := s.authorize(ctx, c.UserID(), req.ConvID)
	if perr != nil {
		return nil, perr
	}

	// 群历史可见性（BE-032）：两个方向都只返回 seq >= minSeq 的消息，翻到 minSeq 时 hasMore=false。
	var msgs []store.Message
	var hasMore bool
	var err error
	if req.AfterSeq != nil {
		msgs, hasMore, err = s.store.ListMessages(ctx, req.ConvID, max(*req.AfterSeq, minSeq-1), limit)
	} else {
		msgs, hasMore, err = s.store.ListMessagesBefore(ctx, req.ConvID, *req.BeforeSeq, minSeq, limit)
	}
	if err != nil {
		return nil, s.internal("list messages", err)
	}
	out := protocol.SyncAck{Messages: make([]protocol.Message, 0, len(msgs)), HasMore: hasMore}
	for _, m := range msgs {
		out.Messages = append(out.Messages, toProto(m))
	}
	if err := EnrichFiles(ctx, s.store, out.Messages); err != nil {
		return nil, s.internal("load files", err)
	}
	// 引用按当前状态组装（F-QUO-02 规则 4，AC-QUO-02-1）：一页批量读，不逐条查库。
	if err := EnrichQuotes(ctx, s.store, out.Messages); err != nil {
		return nil, s.internal("load quotes", err)
	}
	return out, nil
}

func (s *Service) read(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.ReadReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if _, perr := s.authorize(ctx, c.UserID(), req.ConvID); perr != nil {
		return nil, perr
	}
	readSeq, advanced, err := s.store.SetReadSeq(ctx, c.UserID(), req.ConvID, req.Seq)
	if err != nil {
		return nil, s.internal("set read seq", err)
	}
	// 已读位置前进了才通知其他端：重复、回退、截断后不变的上报都不推送（TC-SYNC-02-05）。
	if advanced {
		s.pushReadUpdated(c.UserID(), req.ConvID, readSeq, c.ConnID())
	}
	return protocol.Empty{}, nil
}

// maxConvs 是会话列表的上限（C-03）。
const maxConvs = 500

func (s *Service) convs(ctx context.Context, c delivery.Session) (any, *protocol.Error) {
	rows, truncated, err := s.store.ListConversations(ctx, c.UserID(), maxConvs)
	if err != nil {
		return nil, s.internal("list conversations", err)
	}
	out := protocol.ConvsAck{Convs: make([]protocol.Conversation, 0, len(rows)), Truncated: truncated}
	// 删除的会话（F-CVS-03 规则 6）：只列当前隐藏的，按删除时间倒序；读失败是 5000，不降级成"没有设置"（backend-chat 第 90 条）。
	deleted, deletedTruncated, err := s.store.DeletedConvs(ctx, c.UserID(), s.cvs.cfg.DeletedMax)
	if err != nil {
		return nil, s.internal("list deleted conversations", err)
	}
	for _, d := range deleted {
		out.Deleted = append(out.Deleted, protocol.DeletedConv{ConvID: d.ConvID, ClearedSeq: d.ClearedSeq})
	}
	out.DeletedTruncated = deletedTruncated
	for _, r := range rows {
		out.Convs = append(out.Convs, toConversation(r))
	}
	if perr := s.decorateConvs(ctx, c.UserID(), out.Convs); perr != nil {
		return nil, perr
	}
	return out, nil
}

// decorateConvs 补上禁言和提及计数：convs 和 convSet 的 ack 用同一份（BE-484 第 3 条）。
func (s *Service) decorateConvs(ctx context.Context, uid string, convs []protocol.Conversation) *protocol.Error {
	if perr := s.applyGroupMutes(ctx, uid, convs); perr != nil {
		return perr
	}
	return s.applyMentionCounts(ctx, uid, convs)
}

// toConversation 把存储层的原始数据算成协议里的会话（PRD F-CONV-01、F-CONV-02）。
func toConversation(r store.ConvRow) protocol.Conversation {
	conv, _ := protocol.ParseConvID(r.ConvID) // 来自我们自己写入的数据，格式可信
	out := protocol.Conversation{
		ConvID:    r.ConvID,
		Type:      conv.Type,
		MaxSeq:    r.MaxSeq,
		ReadSeq:   r.ReadSeq,
		MinSeq:    1,
		UpdatedAt: r.UpdatedAt,
		ModVer:    r.ModVer,
	}
	switch conv.Type {
	case protocol.ConvGroup:
		out.Name = r.GroupName
		n := r.MemberCount
		out.MemberCount = &n
		out.MinSeq = groupMinSeq(store.Group{HistoryVisible: r.HistoryVisible}, store.GroupMember{JoinSeq: r.JoinSeq})
		out.MyRole = r.MyRole
	default:
		// 单聊名称：对方有昵称用昵称，否则用 userId（BE-043；昵称修改后下次拉取生效，C-19）。
		out.Name = cmp.Or(r.PeerNickname, r.PeerID)
	}
	// 置顶、免打扰（C5）：开关关闭后照常输出（P-46 ②）；没有设置时都不出现（omitempty）。免打扰下未读照常计算（F-CVS-02 规则 3）。
	out.Pinned, out.PinnedAt, out.NotifyOff = r.PinnedAt > 0, r.PinnedAt, r.NotifyOff
	// 删除会话：我这边清除范围内的历史不再可见（F-CVS-03 规则 2），minSeq = max(原值, clearedSeq + 1)。
	out.MinSeq = max(out.MinSeq, r.ClearedSeq+1)
	// 未读 = maxSeq − max(readSeq, joinSeq)，从入群时刻算起，与 historyVisible 无关（C-02）。
	out.Unread = max(0, r.MaxSeq-max(r.ReadSeq, r.JoinSeq))
	if r.LastVisible {
		out.LastMsg = &protocol.LastMsg{MsgID: r.Last.MsgID, From: r.Last.From, Type: r.Last.Type, Text: r.Last.Text, TS: r.Last.TS,
			Deleted: r.Last.Deleted || r.Last.Recalled, Recalled: r.Last.Recalled, RecalledBy: r.Last.RecalledBy} // 撤回同时 deleted=true（F-RCL-03 规则 4）
	}
	return out
}

func (s *Service) pushReadUpdated(uid, convID string, readSeq int64, exceptConn string) {
	s.pushEvent(uid, protocol.ReadUpdatedEvent{Type: protocol.EventReadUpdated, ConvID: convID, ReadSeq: readSeq}, exceptConn)
}

func (s *Service) pushEvent(uid string, ev any, exceptConn string) {
	frame, err := json.Marshal(protocol.Event(ev))
	if err != nil {
		s.log.Error("marshal event", "err", err)
		return
	}
	s.router.DeliverToUser(uid, frame, exceptConn)
}

// NotifyConvAdded 给被加入群的用户的所有在线连接下发 convAdded，内容与该用户 convs 里的这一项相同（BE-034）。
// 由管理接口在加成员、建群成功后调用；只传本次真正新加入的用户（幂等操作不发事件，C-36）。
// 一次查询拿到全部新成员的会话（建 2000 人群时不能逐个查）。
func (s *Service) NotifyConvAdded(ctx context.Context, convID string, userIDs []string) {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil || conv.Type != protocol.ConvGroup {
		return
	}
	rows, err := s.store.GroupConversations(ctx, conv.A, userIDs)
	if err != nil {
		s.log.Error("load conversations for convAdded", "err", err, "conv", convID)
		return
	}
	allMuted := s.withAllMuted(ctx, conv.A)
	for _, uid := range userIDs {
		if r, ok := rows[uid]; ok { // 不在结果里：刚加入又被移除了
			c := toConversation(r)
			c.AllMuted = allMuted
			s.pushEvent(uid, protocol.ConvAddedEvent{Type: protocol.EventConvAdded, Conv: c}, "")
		}
	}
}

// NotifyConvRemoved 给被移出群（或群被解散）的用户的所有在线连接下发 convRemoved {reason}（BE-034、BE-122）。
// 移出、解散时存储层已在同一事务里删除这些人的会话设置（F-CVS-01 规则 6），这里一并发布失效（BE-483 第 4 条）。
func (s *Service) NotifyConvRemoved(convID string, userIDs []string, reason string) {
	s.publishConvSettings(userIDs...)
	for _, uid := range userIDs {
		s.pushEvent(uid, protocol.ConvRemovedEvent{Type: protocol.EventConvRemoved, ConvID: convID, Reason: reason}, "")
	}
}

// authorize 检查用户能否访问会话，并返回用户能看到的最小 seq（minSeq）：
// 单聊必须是其中一方（4032），minSeq=1；群必须存在（4043）且是成员（4033），minSeq 见 groupMinSeq。
// 被移除的成员连以前看过的消息也拉不到（PRD AC-GRP-01-4）。删除过会话的再取 max(minSeq, clearedSeq + 1)（C5，F-CVS-03 规则 2）：
// sync、read、recall（4044）、quote（4040 quoteUnavailable）都经过这里（backend-chat 第 77 条）。
func (s *Service) authorize(ctx context.Context, uid, convID string) (minSeq int64, perr *protocol.Error) {
	if minSeq, perr = s.authorizeMember(ctx, uid, convID); perr != nil {
		return 0, perr
	}
	cleared, err := s.store.ConvClearedSeq(ctx, uid, convID)
	if err != nil {
		return 0, s.internal("conv cleared seq", err)
	}
	return max(minSeq, cleared+1), nil
}

// authorizeMember 是 authorize 的参与方、成员判断和不含删除位置的 minSeq。
func (s *Service) authorizeMember(ctx context.Context, uid, convID string) (int64, *protocol.Error) {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return 0, perr
	}
	switch conv.Type {
	case protocol.ConvC2C:
		if !conv.Has(uid) {
			return 0, protocol.Errorf(protocol.CodeNotConvParticipant, "not a participant of %s", convID)
		}
		return 1, nil
	case protocol.ConvGroup:
		g, err := s.store.GetGroup(ctx, conv.A)
		if errors.Is(err, store.ErrNotFound) {
			return 0, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", conv.A)
		}
		if err != nil {
			return 0, s.internal("get group", err)
		}
		m, ok, err := s.store.GroupMember(ctx, conv.A, uid)
		if err != nil {
			return 0, s.internal("check group member", err)
		}
		if !ok {
			return 0, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", convID)
		}
		return groupMinSeq(g, m), nil
	default:
		return 0, protocol.Errorf(protocol.CodeBadRequest, "room messages cannot be synced")
	}
}

// groupMinSeq：historyVisible 时可以看全部历史（1），否则只能看入群之后的（joinSeq+1）。
// 设置在查询时生效（PRD F-GRP-01 规则 6），所以每次现算，不存下来。
func groupMinSeq(g store.Group, m store.GroupMember) int64 {
	if g.HistoryVisible {
		return 1
	}
	return m.JoinSeq + 1
}

func (s *Service) internal(op string, err error) *protocol.Error {
	s.log.Error(op, "err", err)
	return protocol.Errorf(protocol.CodeInternal, "internal error")
}

// decode 严格解析 data：未知字段直接报错，和 Schema 的 additionalProperties=false 一致。
func decode[T any](raw json.RawMessage) (T, *protocol.Error) {
	var v T
	if len(raw) == 0 {
		return v, protocol.Errorf(protocol.CodeBadRequest, "missing data")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, protocol.Errorf(protocol.CodeBadRequest, "invalid data: %v", err)
	}
	return v, nil
}

// toProto 是下行版序列化：撤回的消息同时 deleted=true、recalled=true，没有 body（F-RCL-01 规则 5a）。后台版见 ToAdminProto。
func toProto(m store.Message) protocol.Message {
	out := ToAdminProto(m)
	out.Deleted = m.DeletedAt > 0 || m.RecalledAt > 0
	return out
}

// ToAdminProto 是后台版序列化（F-RCL-04 规则 3，RC2-21）：deleted 只表示后台删除，撤回单独用 recalled*；被删、撤回的都没有 body。
// 引用只有 msgId，由 EnrichQuotes 组装。
func ToAdminProto(m store.Message) protocol.Message {
	var body protocol.MsgBody
	if m.DeletedAt == 0 && m.RecalledAt == 0 { // 被删除、撤回的消息没有 body（F-MSG-05 规则 7）
		_ = json.Unmarshal(m.Body, &body) // Body 是我们自己写入的，格式可信
		if m.QuoteMsgID != "" {
			body.Quote = &protocol.Quote{MsgID: m.QuoteMsgID}
		}
	}
	return protocol.Message{
		MsgID:       m.MsgID,
		ClientMsgID: m.ClientMsgID,
		ConvID:      m.ConvID,
		Seq:         m.Seq,
		From:        m.From,
		Type:        m.Type,
		Body:        body,
		TS:          m.TS,
		Deleted:     m.DeletedAt > 0,
		DeletedAt:   m.DeletedAt,
		Recalled:    m.RecalledAt > 0,
		RecalledAt:  m.RecalledAt,
		RecalledBy:  m.RecalledBy,
	}
}
