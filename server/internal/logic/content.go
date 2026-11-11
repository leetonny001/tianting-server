package logic

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"time"

	"tianting/server/internal/delivery"
	"tianting/server/internal/moderation"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/store"
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
	// C2CBodyAccess 跟 IM_C2C_BODY_ACCESS（app 设置，不单独读环境变量）：存证据时读一次（F-MOD-03 规则 5，第 95 条 ①）。
	C2CBodyAccess bool
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
	// words 是词库引擎；开关关闭时为 nil（不加载、不检查，AC-MOD-02-10）。
	words *moderation.Engine
}

// ConfigureModeration 设置 C6（只在启动时调用）。IM_MOD_ENABLED 关闭时只拦客户端的 report 和敏感词检查，
// 管理接口和清理任务照常（P-48 ④，backend-chat 第 108 条）。
func (s *Service) ConfigureModeration(cfg ModConfig, evidence *quarantine.Cipher, words *moderation.Engine) {
	if !cfg.Enabled {
		words = nil
	}
	s.mod = &modState{cfg: cfg.WithDefaults(), evidence: evidence, words: words}
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

// 检查场景的名字在 moderation 包（account、logic 共用，backend-chat 第 110 条）。
const (
	SceneText        = moderation.SceneText
	SceneFileName    = moderation.SceneFileName
	SceneGroupName   = moderation.SceneGroupName
	SceneGroupNotice = moderation.SceneGroupNotice
	SceneFriendApply = moderation.SceneFriendApply
)

// contentBlocked 是 4510：不带 details，不告诉命中了哪个词（F-MOD-02 规则 6）。
func contentBlocked() *protocol.Error {
	return protocol.Errorf(protocol.CodeContentBlocked, "content blocked")
}

// blocked 检查一段客户端写入的内容（F-MOD-02 规则 2）。开关关闭、词库为空时不拦截。命中时 im_blocked_total{scene} 加 1。
func (s *Service) blocked(scene, text string) (moderation.Hit, bool) {
	if s.mod.words == nil || text == "" {
		return moderation.Hit{}, false
	}
	hit, ok := s.mod.words.Match(text)
	if ok {
		s.metrics.Blocked.WithLabelValues(scene).Inc()
	}
	return hit, ok
}

// CheckContent 检查一段客户端写入的内容（F-MOD-02 规则 2），命中返回 true。命中时：im_blocked_total{scene} 加 1；写 moderation_hits
// （uid 为空时不写，只计指标：注册时还没有用户，backend-chat 第 113 条）；日志只记场景和 word_id（AC-MOD-02-8），不记内容和用户。
// hits 写失败不影响拦截结果（规则 7 的异常）。account 包的注册、改昵称经这个方法检查。
func (s *Service) CheckContent(ctx context.Context, uid, scene, text string) bool {
	hit, ok := s.blocked(scene, text)
	if ok {
		s.recordHit(ctx, uid, scene, hit)
	}
	return ok
}

func (s *Service) recordHit(ctx context.Context, uid, scene string, hit moderation.Hit) {
	s.log.Info("content blocked", "scene", scene, "wordId", hit.WordID)
	if uid == "" {
		return
	}
	if err := s.store.RecordHit(ctx, store.ModHit{UserID: uid, Scene: scene, WordID: hit.WordID, At: s.clock.Now().UnixMilli()}); err != nil {
		s.log.Warn("record moderation hit", "scene", scene, "err", err)
	}
}

// checkSendContent 是 send 的第 9 步（F-MOD-02 规则 2、3）：text 的正文；image、file 查**服务端记录的文件名**（files.name，不是上行的字段，
// backend-chat 第 111 条）。不查引用摘要、tip、custom（生产禁止客户端发送）、图片和文件内容。
func (s *Service) checkSendContent(ctx context.Context, uid string, req protocol.SendReq, fileName string) *protocol.Error {
	switch req.Type {
	case protocol.MsgText:
		if s.CheckContent(ctx, uid, SceneText, req.Body.Text) {
			return contentBlocked()
		}
	case protocol.MsgImage, protocol.MsgFile:
		if s.CheckContent(ctx, uid, SceneFileName, fileName) {
			return contentBlocked()
		}
	}
	return nil
}

// PurgeHits 是定时任务 moderation_hits_purge（单节点，同 F-ADM-32）：删除 IM_MOD_HITS_DAYS 之前的命中记录（AC-MOD-02-14）。
// 不跟 IM_MOD_ENABLED（P-48 ④：保留期是对用户的承诺）。
func (s *Service) PurgeHits(ctx context.Context) (int, error) {
	return s.store.PurgeHits(ctx, s.clock.Now().Add(-time.Duration(s.mod.cfg.HitsDays)*24*time.Hour).UnixMilli())
}

// HitsDays 是命中记录的保留天数（GET /admin/moderation/hits 的 days）。
func (s *Service) HitsDays() int { return s.mod.cfg.HitsDays }

// errContentBlocked 让存储层的事务在权限判定之后、写入之前停下（groupCreate、groupUpdate，backend-chat 第 112、137 条）。
var errContentBlocked = errors.New("content blocked")
