package logic

import (
	"cmp"
	"context"
	"encoding/json"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
)

// C6 敏感词和举报（PRD-chat 第 10 节，BE-492～BE-496）。

// ModConfig 是第 4.1 节的 C6 配置。0 表示默认值（都未校准）。
type ModConfig struct {
	Enabled      bool // IM_MOD_ENABLED，默认关：关时不检查敏感词（AC-MOD-02-10），report 4030 featureDisabled（AC-MOD-03-14）
	ReportDaily  int  // IM_REPORT_DAILY，默认 20（F-MOD-03 规则 4）
	EvidenceDays int  // IM_REPORT_EVIDENCE_DAYS，默认 180（F-MOD-03 规则 6）
	HitsDays     int  // IM_MOD_HITS_DAYS（PRD 没有变量名，backend-chat 第 98 条），默认 90（F-MOD-02 规则 7）
	// ClientCustomOff 是 IM_CLIENT_CUSTOM_MSG=0（F-MOD-02 规则 4）：客户端 send 的 custom 返回 4000 customDisabled。
	// 生产默认关、开发和测试默认开（第 4.1 节），不跟 Enabled 走（backend-chat 第 105 条）。字段取反，零值就是开发默认。
	ClientCustomOff bool
}

// 默认值（第 4.1 节）。
const (
	DefaultReportDaily  = 20
	DefaultEvidenceDays = 180
	DefaultModHitsDays  = 90
)

// WithDefaults 填上默认值。
func (c ModConfig) WithDefaults() ModConfig {
	c.ReportDaily = cmp.Or(c.ReportDaily, DefaultReportDaily)
	c.EvidenceDays = cmp.Or(c.EvidenceDays, DefaultEvidenceDays)
	c.HitsDays = cmp.Or(c.HitsDays, DefaultModHitsDays)
	return c
}

type modState struct {
	cfg ModConfig
	// evidence 加密举报证据（IM_REPORT_KEY，AES-256-GCM，复用 quarantine.Cipher 的实现、不同的密钥）。开关打开时一定非 nil。
	evidence *quarantine.Cipher
}

// ConfigureModeration 设置 C6（只在启动时调用）。IM_MOD_ENABLED 关闭时只拦客户端的 report 和敏感词检查，
// 管理接口和清理任务照常（P-48 ④，backend-chat 第 108 条）。
func (s *Service) ConfigureModeration(cfg ModConfig, evidence *quarantine.Cipher) {
	s.mod = &modState{cfg: cfg.WithDefaults(), evidence: evidence}
}

// ModerationEnabled 报告 C6 开关（GET /v1/config 的 features.moderation，backend-chat 第 96 条）。
func (s *Service) ModerationEnabled() bool { return s.mod.cfg.Enabled }

// report：F-MOD-03。校验顺序（BE-495 第 1 条）：格式 → 开关 → 每天限额 → 对象校验 → 幂等 → 写入。
func (s *Service) report(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.ReportReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if !s.mod.cfg.Enabled {
		return nil, featureDisabled()
	}
	return s.submitReport(ctx, c, req)
}
