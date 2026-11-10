package logic

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"tianting/server/internal/delivery"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// C4 撤回、引用、@（PRD-chat 第 8 节，BE-471～BE-477）。

// MsgActionsConfig 是第 4.1 节的 C4 配置。0 表示默认值（都未校准）。
type MsgActionsConfig struct {
	Enabled           bool          // IM_MSG_ACTIONS_ENABLED，默认关
	RecallWindow      time.Duration // IM_RECALL_WINDOW_SEC，默认 120 秒
	GroupRecallWindow time.Duration // IM_GROUP_RECALL_WINDOW_SEC，默认 7 天
	MentionMax        int           // IM_MENTION_MAX，默认 20
	MentionAllHourly  int           // IM_MENTION_ALL_HOURLY，默认 3（滚动 60 分钟，backend-chat 第 58 条）
	MentionAllDaily   int           // IM_MENTION_ALL_DAILY，默认 10（Asia/Shanghai 自然日）
	// PRD 没有变量名（backend-chat 第 53 条）：撤回每人每分钟 20 次，成员搜索（groupMembers 带 keyword）每人每分钟 30 次。
	RecallRatePerMin       int // IM_RECALL_RATE_PER_MIN
	MemberSearchRatePerMin int // IM_MEMBER_SEARCH_RATE_PER_MIN
}

// 默认值（第 4.1 节）。
const (
	DefaultRecallWindow           = 120 * time.Second
	DefaultGroupRecallWindow      = 7 * 24 * time.Hour
	DefaultMentionMax             = 20
	DefaultMentionAllHourly       = 3
	DefaultMentionAllDaily        = 10
	DefaultRecallRatePerMin       = 20
	DefaultMemberSearchRatePerMin = 30
	defaultQuarantineDays         = 30 // CH-17：撤回原文和文件对象保留 30 天（和 IM_QUARANTINE_DAYS 一致）
	quoteTextLen                  = 100
)

// WithDefaults 填上默认值。
func (c MsgActionsConfig) WithDefaults() MsgActionsConfig {
	c.RecallWindow = cmp.Or(c.RecallWindow, DefaultRecallWindow)
	c.GroupRecallWindow = cmp.Or(c.GroupRecallWindow, DefaultGroupRecallWindow)
	c.MentionMax = cmp.Or(c.MentionMax, DefaultMentionMax)
	c.MentionAllHourly = cmp.Or(c.MentionAllHourly, DefaultMentionAllHourly)
	c.MentionAllDaily = cmp.Or(c.MentionAllDaily, DefaultMentionAllDaily)
	c.RecallRatePerMin = cmp.Or(c.RecallRatePerMin, DefaultRecallRatePerMin)
	c.MemberSearchRatePerMin = cmp.Or(c.MemberSearchRatePerMin, DefaultMemberSearchRatePerMin)
	return c
}

type msgActions struct {
	cfg            MsgActionsConfig
	cipher         *quarantine.Cipher
	quarantineDays int
	recallRate     *ratelimit.TokenBucket
	searchRate     *ratelimit.TokenBucket // 成员搜索：开关关闭时也用（backend-chat 第 59 条）
}

func newMsgActions(cfg MsgActionsConfig, s *Service) *msgActions {
	cfg = cfg.WithDefaults()
	return &msgActions{
		cfg:        cfg,
		recallRate: ratelimit.NewPer(time.Minute/time.Duration(cfg.RecallRatePerMin), cfg.RecallRatePerMin, s.clock),
		searchRate: ratelimit.NewPer(time.Minute/time.Duration(cfg.MemberSearchRatePerMin), cfg.MemberSearchRatePerMin, s.clock),
	}
}

// ConfigureMsgActions 设置 C4（只在启动时调用）。开启时 cipher 不能为 nil（启动检查保证有 IM_QUARANTINE_KEY，AC-RCL-01-10）；
// 撤回不依赖 IM_V13_ADMIN_FEATURES（第 4.1 节 RC2-15）。
func (s *Service) ConfigureMsgActions(cfg MsgActionsConfig, cipher *quarantine.Cipher, quarantineDays int) error {
	if cfg.Enabled && cipher == nil {
		return errors.New("IM_MSG_ACTIONS_ENABLED=1 requires IM_QUARANTINE_KEY")
	}
	a := newMsgActions(cfg, s)
	a.cipher, a.quarantineDays = cipher, cmp.Or(quarantineDays, defaultQuarantineDays)
	s.actions = a
	return nil
}

// MsgActionsEnabled 报告 C4 开关（GET /v1/config 的 features.msgActions）。
func (s *Service) MsgActionsEnabled() bool { return s.actions.cfg.Enabled }

func featureDisabled() *protocol.Error {
	return protocol.WithReason(protocol.CodeForbidden, protocol.ReasonFeatureDisabled, "feature disabled")
}

// ---- 撤回（F-RCL-01、02，BE-472） ----

func (s *Service) recall(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	if !s.actions.cfg.Enabled {
		return nil, featureDisabled() // AC-RCL-01-10 ①
	}
	req, perr := decode[protocol.RecallReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	// 频率在格式之后、查库之前，幂等重试也计数（backend-chat 第 55 条）；不和发送限流共用（F-RCL-01 规则 4）。
	if !s.actions.recallRate.Allow(uid) {
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many recalls")
	}
	minSeq, perr := s.authorize(ctx, uid, req.ConvID) // 4032、4033、4043（解散的群 4043，AC-RCL-02-6）
	if perr != nil {
		return nil, perr
	}
	notFound := protocol.Errorf(protocol.CodeMessageNotFound, "message not found")
	m, err := s.store.GetMessage(ctx, req.MsgID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, notFound
	case err != nil:
		return nil, s.internal("get message", err)
	case m.ConvID != req.ConvID || m.Seq < minSeq:
		return nil, notFound // 属于其他会话也是 4044（AC-RCL-01-3）；看不到的（入群前、历史不可见）按不存在
	case m.Type == protocol.MsgTip:
		return nil, protocol.Errorf(protocol.CodeBadRequest, "tip messages cannot be recalled") // 在"消息存在"之后判断（第 56 条）
	case m.RecalledAt > 0:
		return protocol.Empty{}, nil // 幂等成功，不再发事件（AC-RCL-01-4）
	case m.DeletedAt > 0:
		return nil, notFound // 已被后台删除（AC-RCL-04-2）
	}
	by, window, perr := s.recallPermission(ctx, uid, m)
	if perr != nil {
		return nil, perr
	}
	now := s.clock.Now()
	if now.UnixMilli()-m.TS > window.Milliseconds() {
		e := protocol.Errorf(protocol.CodeRecallWindowExpired, "recall window expired")
		e.Details = map[string]any{"windowSeconds": int64(window / time.Second)}
		return nil, e
	}
	seal := s.actions.cipher.Seal
	if s.faults.msgActions().quarantineWrite { // AC-RCL-01-11：隔离写失败，整个事务回滚
		seal = func(string, string, []byte) ([]byte, []byte, string, error) { return nil, nil, "", errInjectedFault }
	}
	res, err := s.store.RecallMessage(ctx, store.RecallReq{
		ConvID: req.ConvID, MsgID: req.MsgID, By: uid, Now: now.UnixMilli(),
		ExpiresAt: now.Add(time.Duration(s.actions.quarantineDays) * 24 * time.Hour).UnixMilli(), Seal: seal,
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, notFound // 判断之后被后台删除了
	case errors.Is(err, store.ErrMsgNotDeletable):
		return nil, protocol.Errorf(protocol.CodeBadRequest, "tip messages cannot be recalled")
	case err != nil:
		return nil, s.internal("recall message", err) // 日志只有错误，没有原文和文件名（AC-CHAT-90-5）
	}
	if res.Already {
		return protocol.Empty{}, nil // 并发的另一次撤回已经发过事件
	}
	s.metrics.Recalls.WithLabelValues(by).Inc()
	// 群内自治，不写后台审计（F-RCL-02 规则 5）；消息行保留 recalled_by。未读、updatedAt 不变（规则 6）。
	s.notifyRecalled(ctx, req.ConvID, req.MsgID, res.Seq, uid, now.UnixMilli(), c.ConnID())
	return protocol.Empty{}, nil
}

// recallPermission 是 F-RCL-01、02 的权限：返回指标的 by 和适用的时限。角色每次查库（F-RCL-02 规则 4），不用成员缓存。
func (s *Service) recallPermission(ctx context.Context, uid string, m store.Message) (string, time.Duration, *protocol.Error) {
	cfg := s.actions.cfg
	conv, _ := protocol.ParseConvID(m.ConvID) // 来自我们自己写入的数据
	forbidden := protocol.Errorf(protocol.CodeForbidden, "no permission to recall this message")
	if conv.Type != protocol.ConvGroup {
		if m.From != uid {
			return "", 0, forbidden // AC-RCL-01-3：单聊只能撤回自己的
		}
		return metrics.RecallBySender, cfg.RecallWindow, nil
	}
	g, err := s.store.GetGroup(ctx, conv.A)
	if errors.Is(err, store.ErrNotFound) {
		return "", 0, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", conv.A)
	}
	if err != nil {
		return "", 0, s.internal("get group", err)
	}
	role, err := s.memberRole(ctx, conv.A, uid)
	switch {
	case errors.Is(err, store.ErrNotGroupMember):
		return "", 0, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", m.ConvID)
	case err != nil:
		return "", 0, s.internal("load member role", err)
	}
	if m.From == uid {
		// 撤回自己的：取发送者规则和角色规则中更宽的时限（F-RCL-02 规则 1；管理员同样，第 57 条）。
		w := cfg.RecallWindow
		if role == store.RoleOwner || role == store.RoleAdmin {
			w = max(w, cfg.GroupRecallWindow)
		}
		return metrics.RecallBySender, w, nil
	}
	// 撤回别人的：托管群没有群主时谁都不行（规则 3）；群主可以撤回任何人（含管理员），管理员只能撤回普通成员。
	if g.OwnerID == "" {
		return "", 0, forbidden
	}
	switch role {
	case store.RoleOwner:
		return metrics.RecallByOwner, cfg.GroupRecallWindow, nil
	case store.RoleAdmin:
		target, err := s.memberRole(ctx, conv.A, m.From)
		switch {
		case errors.Is(err, store.ErrNotGroupMember): // 被撤回者已退群：按普通成员（F-RCL-02 异常）
		case err != nil:
			return "", 0, s.internal("load member role", err)
		case target == store.RoleOwner || target == store.RoleAdmin:
			return "", 0, forbidden
		}
		return metrics.RecallByAdmin, cfg.GroupRecallWindow, nil
	}
	return "", 0, forbidden
}

// notifyRecalled 给会话的全部在线参与者（含操作者的其他端）下发 msgDeleted 带 recalled（F-RCL-03 规则 1）。操作者发起的连接只收 ack。
func (s *Service) notifyRecalled(ctx context.Context, convID, msgID string, seq int64, by string, at int64, exceptConn string) {
	users, err := s.convUsers(ctx, convID)
	if err != nil {
		s.log.Error("load conversation users for recall", "err", err, "conv", convID)
		return
	}
	frame, err := json.Marshal(protocol.Event(protocol.MsgDeletedEvent{
		Type: protocol.EventMsgDeleted, ConvID: convID, MsgIDs: []string{msgID}, Seqs: []int64{seq},
		Recalled: &protocol.RecalledInfo{By: by, At: at},
	}))
	if err != nil {
		s.log.Error("marshal msgDeleted", "err", err)
		return
	}
	for _, uid := range users {
		except := ""
		if uid == by {
			except = exceptConn
		}
		s.router.DeliverToUser(uid, frame, except)
	}
}

// convUsers 是会话的参与者：单聊双方，群的当前成员。
func (s *Service) convUsers(ctx context.Context, convID string) ([]string, error) {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return nil, perr
	}
	switch conv.Type {
	case protocol.ConvC2C:
		if conv.A == conv.B {
			return []string{conv.A}, nil
		}
		return []string{conv.A, conv.B}, nil
	case protocol.ConvGroup:
		return s.store.GroupMembers(ctx, conv.A)
	}
	return nil, nil
}

// ---- 引用（F-QUO-01、02，BE-475） ----

// checkQuote 是第 4.3 节第 7 步：被引用的消息同一会话、seq ≥ 发送者的 minSeq、没有撤回或删除、不是 tip。
// 几种原因的响应完全相同（AC-QUO-01-2）。
func (s *Service) checkQuote(ctx context.Context, uid, convID, quoteID string) *protocol.Error {
	unavailable := protocol.WithReason(protocol.CodeNotFound, protocol.ReasonQuoteUnavailable, "quoted message unavailable")
	minSeq, perr := s.authorize(ctx, uid, convID)
	if perr != nil {
		return perr
	}
	q, err := s.store.GetMessage(ctx, quoteID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return unavailable
	case err != nil:
		return s.internal("get quoted message", err)
	case q.ConvID != convID, q.Seq < minSeq, q.RecalledAt > 0, q.DeletedAt > 0, q.Type == protocol.MsgTip:
		return unavailable
	}
	return nil
}

// EnrichQuotes 组装下行的引用（F-QUO-02）：一页里批量读被引用的消息和文件，不逐条查库。
// 不把被引用的内容写进日志（AC-CHAT-90-5）。
func EnrichQuotes(ctx context.Context, st store.Store, msgs []protocol.Message) error {
	var ids []string
	for _, m := range msgs {
		if q := m.Body.Quote; q != nil {
			ids = append(ids, q.MsgID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	quoted, err := st.GetMessages(ctx, ids)
	if err != nil {
		return err
	}
	var fileIDs []string
	bodies := make(map[string]protocol.MsgBody, len(quoted))
	for id, q := range quoted {
		var b protocol.MsgBody
		if q.DeletedAt == 0 && q.RecalledAt == 0 {
			_ = json.Unmarshal(q.Body, &b) // 我们自己写入的
			if fid := b.FileID(); fid != "" {
				fileIDs = append(fileIDs, fid)
			}
		}
		bodies[id] = b
	}
	files := map[string]store.File{}
	if len(fileIDs) > 0 {
		if files, err = st.GetFiles(ctx, fileIDs); err != nil {
			return err
		}
	}
	for i := range msgs {
		ref := msgs[i].Body.Quote
		if ref == nil {
			continue
		}
		v := protocol.Quote{MsgID: ref.MsgID, State: protocol.QuoteDeleted} // 原消息已不存在：按 deleted（F-QUO-02 异常）
		if q, ok := quoted[ref.MsgID]; ok {
			v.Seq, v.From, v.Type = q.Seq, q.From, q.Type
			switch {
			case q.RecalledAt > 0:
				v.State = protocol.QuoteRecalled
			case q.DeletedAt > 0:
				v.State = protocol.QuoteDeleted
			default:
				v.State, v.Text = quoteState(q.Type, bodies[ref.MsgID], files)
			}
		}
		msgs[i].Body.Quote = &v
	}
	return nil
}

// quoteState：文本取前 100 个码点（带引用的消息只取它自己的文字，AC-QUO-01-7）；图片「[图片]」；文件「[文件] {文件名}」；
// 文件过期（或已清除）是 expired，没有文字和文件名（AC-QUO-02-7）。
func quoteState(typ string, b protocol.MsgBody, files map[string]store.File) (string, string) {
	switch typ {
	case protocol.MsgText:
		return protocol.QuoteNormal, protocol.Summary(b.Text)
	case protocol.MsgImage, protocol.MsgFile:
		f, ok := files[b.FileID()]
		if !ok || f.State != store.FileAttached {
			return protocol.QuoteExpired, ""
		}
		if typ == protocol.MsgImage {
			return protocol.QuoteNormal, "[图片]"
		}
		return protocol.QuoteNormal, protocol.Summary("[文件] " + f.Name)
	}
	return protocol.QuoteNormal, "" // custom：没有文字
}

// ---- @（F-MEN-01、02，BE-476） ----

// mentionPlan 是这条消息要保存的提及：Body 里的 mentions（过滤后）和存储层的计数参数。
type mentionPlan struct {
	body  *protocol.Mentions
	users []string
	all   *store.MentionAllQuota
}

// planMentions 是第 4.3 节第 8 步：@ 所有人的权限（4030 mentionAllForbidden，角色查库），过滤 userIds（静默去掉不是当前成员的）。
// members 是当前成员（缓存，和发送校验用同一份）。
func (s *Service) planMentions(ctx context.Context, uid, gid string, in *protocol.Mentions, members []string) (mentionPlan, *protocol.Error) {
	var p mentionPlan
	if in.All {
		g, err := s.store.GetGroup(ctx, gid)
		if err != nil {
			return p, s.groupErr("get group", err)
		}
		role, err := s.memberRole(ctx, gid, uid)
		if err != nil {
			return p, s.groupErr("load member role", err)
		}
		// 托管群没有群主时任何人都不能（F-MEN-02 规则 1）；全群禁言不影响群主、管理员（AC-MEN-02-7）。
		if g.OwnerID == "" || (role != store.RoleOwner && role != store.RoleAdmin) {
			return p, protocol.WithReason(protocol.CodeForbidden, protocol.ReasonMentionAllForbidden, "only the owner and admins can mention all")
		}
		now := s.clock.Now().In(shanghai)
		y, mo, d := now.Date()
		day, _ := strconv.ParseInt(fmt.Sprintf("%04d%02d%02d", y, mo, d), 10, 64)
		p.all = &store.MentionAllQuota{
			GroupID: gid, Hourly: s.actions.cfg.MentionAllHourly, Daily: s.actions.cfg.MentionAllDaily,
			Day: day, DayEnd: time.Date(y, mo, d+1, 0, 0, 0, 0, shanghai).UnixMilli(),
		}
	}
	isMember := make(map[string]bool, len(members))
	for _, m := range members {
		isMember[m] = true
	}
	ids := make([]string, 0, len(in.UserIDs))
	for _, id := range in.UserIDs {
		if isMember[id] && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	p.users = ids
	if len(ids) > 0 || in.All {
		p.body = &protocol.Mentions{UserIDs: ids, All: in.All} // 全部被过滤又不是 @ 所有人：按没有 mentions 保存（F-MEN-01 异常）
	}
	return p, nil
}

// distinctCount 是去重后的个数（4131 按去重后判断，AC-MEN-01-11 的重复 userId 不算超限）。
func distinctCount(ids []string) int {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	return len(seen)
}

// mentionFault 把测试模式的故障注入接到存储层（AC-MEN-03-8、AC-MEN-02-8）。
func (s *Service) mentionFault() func(string) error {
	f := s.faults.msgActions()
	if !f.mentionWrite && !f.mentionAllCounter {
		return nil
	}
	return func(point string) error {
		if (point == store.FaultMentionWrite && f.mentionWrite) || (point == store.FaultMentionAllCounter && f.mentionAllCounter) {
			return errInjectedFault
		}
		return nil
	}
}

// applyMentionCounts 给会话列表补上 mentionUnread、firstMentionSeq（F-MEN-03 规则 1）：一次查询，不按会话逐条查。开关关闭时不带。
func (s *Service) applyMentionCounts(ctx context.Context, uid string, convs []protocol.Conversation) *protocol.Error {
	if !s.actions.cfg.Enabled || len(convs) == 0 {
		return nil
	}
	counts, err := s.store.MentionCounts(ctx, uid)
	if err != nil {
		return s.internal("mention counts", err)
	}
	for i := range convs {
		c := counts[convs[i].ConvID]
		n := c.Unread
		convs[i].MentionUnread = &n
		if n > 0 {
			convs[i].FirstMentionSeq = c.First
		}
	}
	return nil
}

// PurgeMentions 是定时任务 mention_purge（第 12.1 节"清理到已读之后"）。
func (s *Service) PurgeMentions(ctx context.Context) (int, error) {
	p, ok := s.store.(interface {
		PurgeMentions(context.Context) (int, error)
	})
	if !ok {
		return 0, nil
	}
	return p.PurgeMentions(ctx)
}
