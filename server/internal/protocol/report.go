package protocol

// C6 举报命令（PRD-chat 第 10 节 F-MOD-03、第 11.1 节，BE-491）。字段和 frame.schema.json 的 ReportReq 一一对应。
// 命令的分发、开关、限额、对象校验和证据随 BE-495；在那之前发 report 仍是 4001。

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

const CmdReport = "report" // IM_MOD_ENABLED 关闭时 4030 featureDisabled（AC-MOD-03-14）

// 举报对象（F-MOD-03 规则 1）。
const (
	ReportTargetMessage = "message"
	ReportTargetUser    = "user"
	ReportTargetGroup   = "group"
)

// ReportReasons 是 reason 的取值（F-MOD-03 规则 1），顺序同 Schema 的 enum。
var ReportReasons = []string{"spam", "abuse", "porn", "fraud", "illegal", "other"}

const (
	// MaxReportMsgs 是一次举报的消息条数上限（F-MOD-03 规则 2），超出 4131（AC-MOD-03-2）。Schema 不写这个上限。
	MaxReportMsgs = 20
	// MaxReportNoteLen 是 note 的码点数上限（F-MOD-03 规则 1），超出 4000（AC-MOD-03-13）。Schema 的长度不是按码点算的，只由服务端判。
	MaxReportNoteLen = 200
)

// ReportReq 是 report 的 data。按 targetType 互斥：message 只能带 convId、msgIds；user 只能带 userId；group 只能带 groupId。
// note 是举报人的附言，按正文对待（F-MOD-04 规则 2）：只进加密的证据，不进 reports 行、日志和审计（backend-chat 第 122 条，P-48 ①）。
type ReportReq struct {
	TargetType string   `json:"targetType"`
	ConvID     string   `json:"convId,omitempty"`
	MsgIDs     []string `json:"msgIds,omitempty"`
	UserID     string   `json:"userId,omitempty"`
	GroupID    string   `json:"groupId,omitempty"`
	Reason     string   `json:"reason"`
	Note       *string  `json:"note,omitempty"`

	// present 是请求里出现了的字段名（UnmarshalJSON 记下）：Schema 按"有没有这个字段"互斥，空串和 null 也算带了，
	// 所以 user 举报带 "convId": "" 要和 Schema 一样拒绝（BE-491 遗留，BE-492 修）。
	present map[string]bool
}

// UnmarshalJSON 记下出现了的字段名，其余按默认规则解码（未知字段仍由调用方的 DisallowUnknownFields 拒绝）。
func (r *ReportReq) UnmarshalJSON(b []byte) error {
	type plain ReportReq
	var p plain
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	*r = ReportReq(p)
	r.present = make(map[string]bool, len(keys))
	for k, v := range keys {
		r.present[k] = true
		if string(v) == "null" { // Schema 里这些字段都不能是 null
			r.present["null:"+k] = true
		}
	}
	return nil
}

// has 报告字段在请求里出现过（解码得到的）或有非零值（直接构造的）。
func (r *ReportReq) has(key string, nonZero bool) bool { return nonZero || r.present[key] }

// Validate 是格式校验（BE-495 第 1 条的第一步）：targetType、reason 不合法 4000；按 targetType 缺字段或带了别的对象字段 4000；
// convId 格式 4007、聊天室 4000；msgIds 为空或有不合法的 4000、超过 20 条 4131；userId、groupId 格式 4004；note 超过 200 个码点 4000。
func (r *ReportReq) Validate() *Error {
	if !slices.Contains(ReportReasons, r.Reason) {
		return Errorf(CodeBadRequest, "invalid reason")
	}
	for k := range r.present {
		if strings.HasPrefix(k, "null:") {
			return Errorf(CodeBadRequest, "%s must not be null", strings.TrimPrefix(k, "null:"))
		}
	}
	if r.Note != nil && TextLen(*r.Note) > MaxReportNoteLen {
		return Errorf(CodeBadRequest, "note longer than %d", MaxReportNoteLen)
	}
	hasConv, hasMsgIDs := r.has("convId", r.ConvID != ""), r.has("msgIds", r.MsgIDs != nil)
	hasUser, hasGroup := r.has("userId", r.UserID != ""), r.has("groupId", r.GroupID != "")
	hasMsg := hasConv || hasMsgIDs
	switch r.TargetType {
	case ReportTargetMessage:
		if hasUser || hasGroup {
			return Errorf(CodeBadRequest, "message reports only allow convId and msgIds")
		}
		if r.ConvID == "" || len(r.MsgIDs) == 0 {
			return Errorf(CodeBadRequest, "message reports need convId and msgIds")
		}
		conv, err := ParseConvID(r.ConvID)
		if err != nil {
			return err
		}
		if conv.Type == ConvRoom {
			return Errorf(CodeBadRequest, "room messages cannot be reported")
		}
		for _, id := range r.MsgIDs {
			if !ValidMsgID(id) {
				return Errorf(CodeBadRequest, "invalid msgIds")
			}
		}
		if len(r.MsgIDs) > MaxReportMsgs {
			return Errorf(CodeBatchLimitExceeded, "at most %d messages per report", MaxReportMsgs)
		}
	case ReportTargetUser:
		if hasMsg || hasGroup {
			return Errorf(CodeBadRequest, "user reports only allow userId")
		}
		if !ValidID(r.UserID) {
			return Errorf(CodeInvalidID, "invalid userId")
		}
	case ReportTargetGroup:
		if hasMsg || hasUser {
			return Errorf(CodeBadRequest, "group reports only allow groupId")
		}
		if !ValidID(r.GroupID) {
			return Errorf(CodeInvalidID, "invalid groupId")
		}
	default:
		return Errorf(CodeBadRequest, "invalid targetType")
	}
	return nil
}

// ReportAck 是 report 的应答。24 小时内对同一对象的重复提交返回第一次的 reportId（F-MOD-03 规则 4）。
type ReportAck struct {
	ReportID string `json:"reportId"`
}
