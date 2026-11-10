package logic

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"time"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// C5 会话置顶、免打扰、删除（PRD-chat 第 9 节，BE-483～BE-486）。

// TopicConvSettings 是会话设置的失效主题（第 11.4 节、F-CVS-04 规则 3），key 是 userId。convSet、convDelete 和退群、被移出、
// 解散的清理提交后发布。C5 没有订阅者（convs 每次读库）；订阅者是 C3 的角标缓存（BE-490 第 3 条，backend-chat 第 89 条）。
const TopicConvSettings = "convSettings"

// ConvSettingsConfig 是第 4.1 节的 C5 配置。0 表示默认值（都未校准）。
type ConvSettingsConfig struct {
	Enabled bool // IM_CONV_SETTINGS_ENABLED，默认关
	PinMax  int  // IM_CONV_PIN_MAX，默认 20
	// PRD 没有变量名（backend-chat 第 73 条）：convSet、convDelete 共用每人每秒 5 次；convs 的 deleted 最多 2000 个。
	RatePerSec int // IM_CONV_SETTINGS_RATE_PER_SEC
	DeletedMax int // IM_CONV_DELETED_MAX
}

// 默认值（第 4.1 节）。
const (
	DefaultConvPinMax       = 20
	DefaultConvSettingsRate = 5
	DefaultConvDeletedMax   = 2000
)

// WithDefaults 填上默认值。
func (c ConvSettingsConfig) WithDefaults() ConvSettingsConfig {
	c.PinMax = cmp.Or(c.PinMax, DefaultConvPinMax)
	c.RatePerSec = cmp.Or(c.RatePerSec, DefaultConvSettingsRate)
	c.DeletedMax = cmp.Or(c.DeletedMax, DefaultConvDeletedMax)
	return c
}

type convSettings struct {
	cfg  ConvSettingsConfig
	rate *ratelimit.TokenBucket // convSet、convDelete 共用，不计入发送限流（F-CVS-04 规则 4）
}

func newConvSettings(cfg ConvSettingsConfig, s *Service) *convSettings {
	cfg = cfg.WithDefaults()
	return &convSettings{cfg: cfg, rate: ratelimit.NewPer(time.Second/time.Duration(cfg.RatePerSec), cfg.RatePerSec, s.clock)}
}

// ConfigureConvSettings 设置 C5（只在启动时调用）。开关关闭时已有的置顶、免打扰、删除照常生效，只拦新命令（P-46 ②，backend-chat 第 81 条）。
func (s *Service) ConfigureConvSettings(cfg ConvSettingsConfig) { s.cvs = newConvSettings(cfg, s) }

// ConvSettingsEnabled 报告 C5 开关（GET /v1/config 的 features.convSettings）。
func (s *Service) ConvSettingsEnabled() bool { return s.cvs.cfg.Enabled }

// publishConvSettings 在会话设置变化（含退群、被移出、解散的清理）提交后发布失效（BE-483 第 4 条）。
func (s *Service) publishConvSettings(userIDs ...string) {
	for _, uid := range userIDs {
		s.inv.Invalidate(TopicConvSettings, uid)
	}
}

// convSet：F-CVS-01、02。校验顺序：格式 → 开关 → 频率 → 会话权限 → 写入（BE-484 第 1 条）。
func (s *Service) convSet(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.ConvSetReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if !s.cvs.cfg.Enabled {
		return nil, featureDisabled() // AC-CVS-02-7
	}
	uid := c.UserID()
	if !s.cvs.rate.Allow(uid) { // 格式之后、查库之前；和 convDelete 共用，不计入发送限流（backend-chat 第 73 条）
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many conversation settings")
	}
	if _, perr := s.authorize(ctx, uid, req.ConvID); perr != nil { // 单聊非参与方 4032；群不存在或已解散 4043；非成员 4033
		return nil, perr
	}
	if _, ok, err := s.store.GetConversation(ctx, uid, req.ConvID); err != nil {
		return nil, s.internal("get conversation", err)
	} else if !ok {
		// 没有消息的单聊（F-CVS-01 规则 5）；和 convDelete 对没有消息的单聊幂等成功不同，两条都是原文（backend-chat 第 78 条）。
		return nil, protocol.Errorf(protocol.CodeNotFound, "conversation not found")
	}
	set, err := s.store.SetConvSettings(ctx, store.ConvSetReq{
		UserID: uid, ConvID: req.ConvID, Pinned: req.Pinned, NotifyOff: req.NotifyOff,
		Now: s.nowMs(), PinMax: s.cvs.cfg.PinMax,
	})
	switch {
	case errors.Is(err, store.ErrPinLimit):
		return nil, protocol.Errorf(protocol.CodePinLimitExceeded, "pin limit reached") // 不带 details，客户端文案固定（backend-chat 第 80 条）
	case errors.Is(err, store.ErrNotGroupMember):
		return nil, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", req.ConvID) // 判断之后退群了
	case err != nil:
		return nil, s.internal("set conversation settings", err)
	}
	s.publishConvSettings(uid)
	// 多端同步（F-CVS-04 规则 1）：同一用户的其他在线连接收到当前状态，操作的连接只拿 ack；幂等的重复操作也发（客户端按服务端覆盖）。
	s.pushEvent(uid, protocol.ConvSettingsChangedEvent{Type: protocol.EventConvSettingsChanged, ConvID: req.ConvID,
		Pinned: set.PinnedAt > 0, PinnedAt: set.PinnedAt, NotifyOff: set.NotifyOff}, c.ConnID())
	conv, perr := s.convFor(ctx, uid, req.ConvID)
	if perr != nil {
		return nil, perr
	}
	return protocol.ConvSetAck{Conv: conv}, nil
}

// convFor 返回和 convs 里这一项完全相同的会话（convSet 的 ack，BE-484 第 3 条）。
func (s *Service) convFor(ctx context.Context, uid, convID string) (protocol.Conversation, *protocol.Error) {
	row, ok, err := s.store.GetConversation(ctx, uid, convID)
	if err != nil {
		return protocol.Conversation{}, s.internal("get conversation", err)
	}
	if !ok {
		return protocol.Conversation{}, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", convID) // 写入之后退群了
	}
	convs := []protocol.Conversation{toConversation(row)}
	if perr := s.decorateConvs(ctx, uid, convs); perr != nil {
		return protocol.Conversation{}, perr
	}
	return convs[0], nil
}

// convDelete：F-CVS-03。校验顺序：格式 → 开关 → 频率 → 会话权限 → 写入（BE-485 第 1 条）。
func (s *Service) convDelete(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.ConvDeleteReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if !s.cvs.cfg.Enabled {
		return nil, featureDisabled()
	}
	uid := c.UserID()
	if !s.cvs.rate.Allow(uid) { // 和 convSet 共用一个计数（backend-chat 第 73 条）
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many conversation settings")
	}
	if _, perr := s.authorize(ctx, uid, req.ConvID); perr != nil { // 4032、4033、已解散的群 4043（AC-CVS-03-9）
		return nil, perr
	}
	if _, ok, err := s.store.GetConversation(ctx, uid, req.ConvID); err != nil {
		return nil, s.internal("get conversation", err)
	} else if !ok {
		return protocol.ConvDeleteAck{ClearedSeq: 0}, nil // 不存在的（没有消息的单聊）：幂等成功，不写（规则 8，第 78 条）
	}
	got, err := s.store.DeleteConv(ctx, store.ConvDeleteReq{UserID: uid, ConvID: req.ConvID, UpToSeq: *req.UpToSeq, Now: s.nowMs()})
	switch {
	case errors.Is(err, store.ErrNotGroupMember):
		return nil, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", req.ConvID) // 判断之后退群了
	case err != nil:
		return nil, s.internal("delete conversation", err) // 5000，客户端恢复显示；日志没有正文
	}
	s.log.Info("conversation deleted", "conv", req.ConvID, "clearedSeq", got.ClearedSeq) // 只记 convId、clearedSeq（BE-485 第 9 条）
	s.publishConvSettings(uid)
	// 只发 convDeleted：它已经意味着已读到 clearedSeq、置顶清除、minSeq = clearedSeq + 1，不另发 readUpdated、convSettingsChanged（第 84 条）。
	s.pushEvent(uid, protocol.ConvDeletedEvent{Type: protocol.EventConvDeleted, ConvID: req.ConvID, ClearedSeq: got.ClearedSeq}, c.ConnID())
	return protocol.ConvDeleteAck{ClearedSeq: got.ClearedSeq}, nil
}
