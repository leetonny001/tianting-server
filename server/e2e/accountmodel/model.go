// Package accountmodel 是账号会话的 Go 参考客户端（PRD-chat F-ACC-03 规则 5、6、6a、7，BE-402）：只做状态机，不做网络和存储。
// protocol/vectors/account-session.json 的期望结果由它生成，三套 SDK 以它为基准（FE-600～602）。
//
// 本批只覆盖客户端的判断：刷新单飞、迟到响应丢弃、失败分类、4011 比较代号。
// 规则 4 的服务端判定表（current、pending、被替换、窗口内外）和转正随 BE-415 追加，附件状态机随 BE-403。
package accountmodel

// 客户端的处理结果。
const (
	ActionNone      = "none"       // 没有用户可见的变化
	ActionRefresh   = "refresh"    // 发一个刷新请求
	ActionUseLocal  = "useLocal"   // 本地 token 比被拒绝的新：直接用本地值重试或重连，不刷新
	ActionBackoff   = "backoff"    // 留在当前页面，退避重试
	ActionRetry     = "retryAfter" // 留在当前页面，重读本地 token，按 retryAfterSeconds 重试
	ActionLogin     = "login"      // 回登录页
	ActionBanNotice = "banNotice"  // 显示封禁文案（第一批服务端不返回，F-ACC-03 规则 10 的兜底）
	ActionKeep      = "keepToken"  // 换上新 token
)

// 回登录页时的提示（规则 6、6a）。
const (
	DialogNone          = ""
	DialogExpired       = "sessionExpired"     // "登录已失效，请重新登录"
	DialogReplay        = "credentialAnomaly"  // "检测到登录凭证异常…"
	DialogRecoverBefore = "recoverBefore"      // "如果不是你本人操作，请在 {日期} 前用恢复码找回"
	DialogSupersededMax = "supersededExceeded" // superseded 重试用完
)

const (
	refreshBelowMs     = int64(6 * 3600 * 1000) // access 剩余不足 6 小时才刷新（规则 5）
	maxSupersededRetry = 3
)

// Session 是一个 SDK 实例里的账号会话。
type Session struct {
	Mode              string // account | demo
	AccessGen         int64
	AccessExpiresAt   int64
	AbsoluteExpiresAt int64
	Inflight          bool  // 有进行中的刷新请求
	Attempt           int64 // 当前请求的序号，超时放弃后递增，迟到的响应按序号丢弃
	Superseded        int   // 连续收到 superseded 的次数
}

// Outcome 是一步操作的结果。
type Outcome struct {
	Action            string `json:"action"`
	Dialog            string `json:"dialog,omitempty"`
	RecoverBefore     int64  `json:"recoverBefore,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	Attempt           int64  `json:"attempt,omitempty"` // action=refresh 时是这次请求的序号
}

// Trigger 处理四个刷新入口（启动、回前台、定时器、kicked/tokenExpired），合并成一次（规则 5）。
// source=kicked 时 rejectedGen 是这条连接用的代号（规则 7）。
func (s *Session) Trigger(source string, nowMs, rejectedGen int64) Outcome {
	if s.Mode != "account" {
		if source == "kicked" {
			return Outcome{Action: ActionLogin, Dialog: DialogExpired} // 演示模式没有 refresh token，行为不变
		}
		return Outcome{Action: ActionNone}
	}
	if source == "kicked" && s.AccessGen > rejectedGen {
		return Outcome{Action: ActionUseLocal}
	}
	if s.Inflight {
		return Outcome{Action: ActionNone}
	}
	if source != "kicked" {
		// access 到期等于绝对上限时不再主动刷新（RS5-11）。
		if s.AccessExpiresAt == s.AbsoluteExpiresAt || s.AccessExpiresAt-nowMs >= refreshBelowMs {
			return Outcome{Action: ActionNone}
		}
	}
	return s.startRefresh()
}

func (s *Session) startRefresh() Outcome {
	s.Inflight = true
	s.Attempt++
	return Outcome{Action: ActionRefresh, Attempt: s.Attempt}
}

// HTTP4011 处理 /v1/* 的 401 / 4011（规则 7、RS5-10）：本地比被拒绝的新就不刷新。
func (s *Session) HTTP4011(rejectedGen int64) Outcome {
	if s.AccessGen > rejectedGen {
		return Outcome{Action: ActionUseLocal}
	}
	if s.Inflight {
		return Outcome{Action: ActionNone}
	}
	return s.startRefresh()
}

// RequestTimeout：请求超时 30 秒放弃。之后这个序号的响应都丢弃（规则 5）。
func (s *Session) RequestTimeout() Outcome {
	if !s.Inflight {
		return Outcome{Action: ActionNone}
	}
	s.Inflight = false
	return Outcome{Action: ActionBackoff}
}

// Tokens 是刷新成功的响应。
type Tokens struct {
	AccessGen         int64 `json:"accessGen"`
	AccessExpiresAt   int64 `json:"accessExpiresAt"`
	AbsoluteExpiresAt int64 `json:"absoluteExpiresAt"`
}

// Failure 是刷新失败的响应。Code 为 0 表示网络错误。
type Failure struct {
	Code              int    `json:"code"`
	Reason            string `json:"reason,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	RecoverBefore     int64  `json:"recoverBefore,omitempty"`
}

// RefreshOK 处理成功响应。不是当前请求的响应直接丢弃（规则 5：迟到的响应不写回）。
func (s *Session) RefreshOK(attempt int64, t Tokens) Outcome {
	if !s.Inflight || attempt != s.Attempt {
		return Outcome{Action: ActionNone}
	}
	s.Inflight, s.Superseded = false, 0
	s.AccessGen, s.AccessExpiresAt, s.AbsoluteExpiresAt = t.AccessGen, t.AccessExpiresAt, t.AbsoluteExpiresAt
	return Outcome{Action: ActionKeep}
}

// RefreshFailed 按规则 6、6a 分类。
func (s *Session) RefreshFailed(attempt int64, f Failure) Outcome {
	if !s.Inflight || attempt != s.Attempt {
		return Outcome{Action: ActionNone}
	}
	s.Inflight = false
	switch {
	case f.Code == 0, f.Code == 4290, f.Code >= 5000:
		return Outcome{Action: ActionBackoff} // 留在当前页面，不回登录页
	case f.Code == 4034:
		return Outcome{Action: ActionBanNotice}
	case f.Code == 4019:
		switch f.Reason {
		case "superseded":
			s.Superseded++
			if s.Superseded >= maxSupersededRetry {
				return Outcome{Action: ActionLogin, Dialog: DialogSupersededMax}
			}
			return Outcome{Action: ActionRetry, RetryAfterSeconds: f.RetryAfterSeconds}
		case "replayDetected":
			return loginOutcome(DialogReplay, f.RecoverBefore)
		}
		return loginOutcome(DialogExpired, f.RecoverBefore)
	}
	return Outcome{Action: ActionLogin, Dialog: DialogExpired}
}

// loginOutcome：有 recoverBefore 就统一提示日期（规则 6a）。replayDetected 的弹窗不变，日期一起带上。
func loginOutcome(dialog string, recoverBefore int64) Outcome {
	if recoverBefore > 0 && dialog != DialogReplay {
		dialog = DialogRecoverBefore
	}
	return Outcome{Action: ActionLogin, Dialog: dialog, RecoverBefore: recoverBefore}
}
