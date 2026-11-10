package logic

import (
	"cmp"
	"context"
	"encoding/json"
	"time"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
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
	return nil, protocol.Errorf(protocol.CodeUnknownCmd, "convSet is not available yet")
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
	return nil, protocol.Errorf(protocol.CodeUnknownCmd, "convDelete is not available yet")
}
