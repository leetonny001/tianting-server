package protocol

// C5 会话置顶、免打扰、删除的命令和事件（PRD-chat 第 9 节 F-CVS-01～04、第 11.1 节，BE-482）。
// 字段和 frame.schema.json 的 ConvSetReq、ConvDeleteReq、convSettingsChanged、convDeleted 一一对应。
// 字段叫 notifyOff，不用 muted：同一结构已有 mutedUntil、allMuted，事件 muted 和 4031 都表示禁言（F-CVS-01 规则 1，R-CB-26）。

const (
	CmdConvSet    = "convSet"    // IM_CONV_SETTINGS_ENABLED 关闭时 4030 featureDisabled
	CmdConvDelete = "convDelete" // 同上

	EventConvSettingsChanged = "convSettingsChanged"
	EventConvDeleted         = "convDeleted"
)

// ConvSetReq 是 convSet 的 data：pinned、notifyOff 至少一项（F-CVS-01 规则 1、异常）。ack 是 ConvSetAck。
type ConvSetReq struct {
	ConvID    string `json:"convId"`
	Pinned    *bool  `json:"pinned,omitempty"`
	NotifyOff *bool  `json:"notifyOff,omitempty"`
}

// Validate 是格式校验：会话 ID（4007）；聊天室没有会话设置（4000）；pinned、notifyOff 都没有（4000）。
func (r *ConvSetReq) Validate() *Error {
	if err := validSettingsConv(r.ConvID); err != nil {
		return err
	}
	if r.Pinned == nil && r.NotifyOff == nil {
		return Errorf(CodeBadRequest, "convSet needs pinned or notifyOff")
	}
	return nil
}

// ConvSetAck 是 convSet 的应答：完整的会话（和 convs 的一项相同）。
type ConvSetAck struct {
	Conv Conversation `json:"conv"`
}

// ConvDeleteReq 是 convDelete 的 data（F-CVS-03 规则 1）。UpToSeq 必须出现、非负；服务端取 min(upToSeq, maxSeq)。ack 是 ConvDeleteAck。
type ConvDeleteReq struct {
	ConvID  string `json:"convId"`
	UpToSeq *int64 `json:"upToSeq"`
}

// Validate 是格式校验：会话 ID（4007）；聊天室（4000）；upToSeq 缺失或为负（4000）。
func (r *ConvDeleteReq) Validate() *Error {
	if err := validSettingsConv(r.ConvID); err != nil {
		return err
	}
	if r.UpToSeq == nil {
		return Errorf(CodeBadRequest, "upToSeq is required")
	}
	if *r.UpToSeq < 0 {
		return Errorf(CodeBadRequest, "upToSeq must be >= 0")
	}
	return nil
}

// ConvDeleteAck 是 convDelete 的应答：清除到的 seq（不小于原有值，F-CVS-03 异常）。
type ConvDeleteAck struct {
	ClearedSeq int64 `json:"clearedSeq"`
}

func validSettingsConv(convID string) *Error {
	conv, err := ParseConvID(convID)
	if err != nil {
		return err
	}
	if conv.Type == ConvRoom {
		return Errorf(CodeBadRequest, "room conversations have no settings")
	}
	return nil
}

// DeletedConv 是 convs 的 deleted 一项（F-CVS-03 规则 6）：当前被我删除、还没有新消息的会话。
type DeletedConv struct {
	ConvID     string `json:"convId"`
	ClearedSeq int64  `json:"clearedSeq"`
}

// ConvSettingsChangedEvent 发给同一用户的其他在线连接（F-CVS-04 规则 1）。PinnedAt 只在 Pinned=true 时出现。
type ConvSettingsChangedEvent struct {
	Type      string `json:"type"`
	ConvID    string `json:"convId"`
	Pinned    bool   `json:"pinned"`
	PinnedAt  int64  `json:"pinnedAt,omitempty"`
	NotifyOff bool   `json:"notifyOff"`
}

// ConvDeletedEvent 发给同一用户的其他在线连接（F-CVS-04 规则 1）。它已经意味着已读到 ClearedSeq、置顶清除、
// minSeq = ClearedSeq + 1，不另发 readUpdated、convSettingsChanged（backend-chat 第 84 条）。
type ConvDeletedEvent struct {
	Type       string `json:"type"`
	ConvID     string `json:"convId"`
	ClearedSeq int64  `json:"clearedSeq"`
}
