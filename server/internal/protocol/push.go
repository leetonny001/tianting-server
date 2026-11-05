package protocol

import "encoding/json"

// 系统推送（PRD-live v1.0 第 4 节）。字段与 frame.schema.json 的 PushData、PushSyncReq 一一对应。

// CmdPushSync 是离线补收命令。
const CmdPushSync = "pushSync"

// 推送目标。
const (
	PushTargetUser = "user"
	PushTargetAll  = "all"
	PushTargetRoom = "room" // 帧格式 L1 冻结，接口 L2 才开放
)

const (
	MaxPushDataBytes   = 4096 // F-PUSH-02 规则 2：data 序列化后的上限
	MaxPushTypeLen     = 64   // 按码点计
	MaxPushUsers       = 1000 // F-PUSH-02 规则 3：去重后的上限
	MaxPushTTLSeconds  = 604800
	defaultPushSyncLim = 50
	maxPushSyncLimit   = 100
)

// PushData 是下行 push 帧的 data，也是 pushSync 应答 pushes 的一项。
type PushData struct {
	PushID  string          `json:"pushId"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
	TS      int64           `json:"ts"`
	Target  string          `json:"target"`
	PushSeq int64           `json:"pushSeq,omitempty"`
	RoomID  string          `json:"roomId,omitempty"`
}

// PushSyncReq 是 pushSync 的请求：{afterSeq, limit?} 或 {fromLatest: true}，二选一。
type PushSyncReq struct {
	AfterSeq   *int64 `json:"afterSeq"`
	Limit      *int   `json:"limit"`
	FromLatest *bool  `json:"fromLatest"`
}

// Validate 校验请求并返回生效的 limit（fromLatest 时为 0）。两个都不传、同时传、fromLatest:false 都是 4000（4.2 节）。
func (r *PushSyncReq) Validate() (int, *Error) {
	if r.FromLatest != nil {
		if !*r.FromLatest || r.AfterSeq != nil || r.Limit != nil {
			return 0, Errorf(CodeBadRequest, "fromLatest must be true and cannot be combined with afterSeq or limit")
		}
		return 0, nil
	}
	if r.AfterSeq == nil {
		return 0, Errorf(CodeBadRequest, "afterSeq or fromLatest is required")
	}
	if *r.AfterSeq < 0 {
		return 0, Errorf(CodeBadRequest, "afterSeq must be >= 0")
	}
	if r.Limit == nil {
		return defaultPushSyncLim, nil
	}
	if *r.Limit < 1 || *r.Limit > maxPushSyncLimit {
		return 0, Errorf(CodeBadRequest, "limit must be 1..%d", maxPushSyncLimit)
	}
	return *r.Limit, nil
}

// PushSyncAck 是 pushSync 的应答：pushes、hasMore、latestSeq 在同一读事务内取得。
type PushSyncAck struct {
	Pushes    []PushData `json:"pushes"`
	HasMore   bool       `json:"hasMore"`
	LatestSeq int64      `json:"latestSeq"`
}
