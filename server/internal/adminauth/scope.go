// Package adminauth 是管理接口的调用方鉴权（PRD-admin F-KEY-02、F-KEY-03）：
// 调用方配置与 scope、请求签名（HMAC-SHA256 / Ed25519）、防重放的 nonce 存储。
// 节点运维接口（PRD-ops F-OPS-16）复用同一个包。
package adminauth

// Scope 是管理接口的权限范围。每个管理路由在注册时声明所需的 scope（PRD-admin F-KEY-02 规则 2）。
type Scope string

const (
	ScopeTokenIssue     Scope = "token.issue"
	ScopeUserRead       Scope = "user.read"
	ScopeUserWrite      Scope = "user.write"
	ScopeUserKick       Scope = "user.kick"
	ScopeUserSessions   Scope = "user.sessions"
	ScopeUserBan        Scope = "user.ban"
	ScopeGroupRead      Scope = "group.read"
	ScopeGroupWrite     Scope = "group.write"
	ScopeGroupDissolve  Scope = "group.dissolve"
	ScopeSocialSettings Scope = "social.settings"
	ScopeSocialFriends  Scope = "social.friends"
	ScopeMsgMeta        Scope = "msg.meta"
	ScopeMsgBodyGroup   Scope = "msg.body.group"
	ScopeMsgBodyC2C     Scope = "msg.body.c2c"
	ScopeMsgQuarantine  Scope = "msg.quarantine"
	ScopeMsgDelete      Scope = "msg.delete"
	ScopeMuteWrite      Scope = "mute.write"
	ScopePushUser       Scope = "push.user"
	ScopePushAll        Scope = "push.all"
	ScopePushRoom       Scope = "push.room" // 随 L1 进目录；L1 时 target=room 由开关挡住（PRD-live AC-PUSH-02-8）
	ScopeAuditRead      Scope = "audit.read"
	ScopeStatsRead      Scope = "stats.read"     // 数据概览（F-ADM-70，P1）
	ScopeAccountInvite  Scope = "account.invite" // 邀请码（PRD-chat F-ACC-09 规则 2）
	ScopeTest           Scope = "test"
	// C6 敏感词和举报（PRD-chat 第 11.6 节、第 14.1 节，BE-492）。report.evidence 是正文类（IsBodyScope），只能在 bodyAccess 调用方上。
	ScopeModerationWords Scope = "moderation.words" // 词库维护（operator、moderator）
	ScopeReportRead      Scope = "report.read"      // 举报队列、举报元数据、按用户查命中次数（moderator，P-48 ③）
	ScopeReportHandle    Scope = "report.handle"    // 结案（moderator）
	ScopeReportEvidence  Scope = "report.evidence"  // 证据授权、证据读取、证据文件（console-msgread）
)

// AllScopes 是 scope 目录；调用方配置里出现目录以外的 scope 拒绝启动（防止拼错后静默失效）。
var AllScopes = []Scope{
	ScopeTokenIssue, ScopeUserRead, ScopeUserWrite, ScopeUserKick, ScopeUserSessions, ScopeUserBan,
	ScopeGroupRead, ScopeGroupWrite, ScopeGroupDissolve, ScopeSocialSettings, ScopeSocialFriends,
	ScopeMsgMeta, ScopeMsgBodyGroup, ScopeMsgBodyC2C, ScopeMsgQuarantine, ScopeMsgDelete,
	ScopeMuteWrite, ScopePushUser, ScopePushAll, ScopePushRoom, ScopeAuditRead, ScopeStatsRead, ScopeAccountInvite, ScopeTest,
	ScopeModerationWords, ScopeReportRead, ScopeReportHandle, ScopeReportEvidence,
}

// IsBodyScope 报告 scope 是否能读消息正文。正文类 scope 只能配置在唯一的 bodyAccess 调用方上，
// 且该调用方不能有其他 scope（PRD-admin F-KEY-02 规则 3）。C6 起 report.evidence 也是（F-MOD-04 规则 3，AC-MOD-04-8）。
func IsBodyScope(s Scope) bool {
	return s == ScopeMsgBodyGroup || s == ScopeMsgBodyC2C || s == ScopeMsgQuarantine || s == ScopeReportEvidence
}

func knownScope(s Scope) bool {
	for _, k := range AllScopes {
		if k == s {
			return true
		}
	}
	return false
}
