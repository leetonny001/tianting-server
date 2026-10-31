// Package protocol 定义 WebSocket 帧的 Go 类型、错误码和入参校验。
// 字段与 protocol/schema/frame.schema.json 一一对应，改这里要同步改 Schema 和用例。
package protocol

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// 命令名。
const (
	CmdSend      = "send"
	CmdSync      = "sync"
	CmdJoinRoom  = "joinRoom"
	CmdLeaveRoom = "leaveRoom"
	CmdRead      = "read"
	CmdPing      = "ping"

	CmdAck   = "ack"
	CmdMsg   = "msg"
	CmdPush  = "push"
	CmdEvent = "event"
	CmdPong  = "pong"
)

// 会话类型。
const (
	ConvC2C   = "c2c"
	ConvGroup = "group"
	ConvRoom  = "room"
)

// 消息类型。
const (
	MsgText   = "text"
	MsgCustom = "custom"
)

// 错误码，见 protocol/README.md。
const (
	CodeBadRequest   = 4000
	CodeUnknownCmd   = 4001
	CodeUnauthorized = 4010
	CodeForbidden    = 4030
	CodeMuted        = 4031
	CodeNotFound     = 4040
	CodeNotInRoom    = 4041
	CodeRateLimited  = 4290
	CodeInternal     = 5000
)

const (
	maxTextLen       = 5000
	defaultSyncLimit = 50
	maxSyncLimit     = 200
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidID 判断用户 / 群 / 房间 ID 是否合法。
func ValidID(s string) bool { return idPattern.MatchString(s) }

// Error 是 ack 里的错误，同时实现 error 接口，便于在业务层直接返回。
type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (e *Error) Error() string { return fmt.Sprintf("%d %s", e.Code, e.Msg) }

// Errorf 构造协议错误。
func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// InFrame 是客户端上行帧，data 延迟到具体命令再解析。
type InFrame struct {
	ID   string          `json:"id,omitempty"`
	Cmd  string          `json:"cmd"`
	Data json.RawMessage `json:"data,omitempty"`
}

// OutFrame 是服务端下行帧。
type OutFrame struct {
	ID    string `json:"id,omitempty"`
	Cmd   string `json:"cmd"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// AckOK / AckErr 构造对请求的应答。
func AckOK(id string, data any) OutFrame { return OutFrame{ID: id, Cmd: CmdAck, Data: data} }
func AckErr(id string, err *Error) OutFrame {
	return OutFrame{ID: id, Cmd: CmdAck, Error: err}
}

// MsgBody 是消息内容：text 消息用 Text，custom 消息用 Data。
type MsgBody struct {
	Text string          `json:"text,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Message 是下行的一条消息（msg 帧、sync 结果里都用它）。
type Message struct {
	MsgID       string  `json:"msgId"`
	ClientMsgID string  `json:"clientMsgId,omitempty"`
	ConvID      string  `json:"convId"`
	Seq         int64   `json:"seq,omitempty"`
	From        string  `json:"from"`
	Type        string  `json:"type"`
	Body        MsgBody `json:"body"`
	TS          int64   `json:"ts"`
}

// SendReq 对应 cmd=send。
type SendReq struct {
	ConvType    string  `json:"convType"`
	To          string  `json:"to"`
	ClientMsgID string  `json:"clientMsgId"`
	Type        string  `json:"type"`
	Body        MsgBody `json:"body"`
}

func (r *SendReq) Validate() *Error {
	switch r.ConvType {
	case ConvC2C, ConvGroup, ConvRoom:
	default:
		return Errorf(CodeBadRequest, "invalid convType %q", r.ConvType)
	}
	if !ValidID(r.To) {
		return Errorf(CodeBadRequest, "invalid to")
	}
	if n := len(r.ClientMsgID); n < 8 || n > 64 {
		return Errorf(CodeBadRequest, "clientMsgId length must be 8-64")
	}
	switch r.Type {
	case MsgText:
		if r.Body.Text == "" {
			return Errorf(CodeBadRequest, "text body is empty")
		}
		if len([]rune(r.Body.Text)) > maxTextLen {
			return Errorf(CodeBadRequest, "text longer than %d", maxTextLen)
		}
	case MsgCustom:
		if len(r.Body.Data) == 0 || string(r.Body.Data) == "null" {
			return Errorf(CodeBadRequest, "custom body.data is empty")
		}
	default:
		return Errorf(CodeBadRequest, "invalid type %q", r.Type)
	}
	return nil
}

// SendAck 是 send 成功的应答。聊天室消息没有 seq。
type SendAck struct {
	MsgID string `json:"msgId"`
	Seq   int64  `json:"seq,omitempty"`
	TS    int64  `json:"ts"`
}

// SyncReq 对应 cmd=sync。
type SyncReq struct {
	ConvID   string `json:"convId"`
	AfterSeq int64  `json:"afterSeq"`
	Limit    int    `json:"limit"`
}

func (r *SyncReq) Validate() *Error {
	if _, err := ParseConvID(r.ConvID); err != nil {
		return err
	}
	if r.AfterSeq < 0 {
		return Errorf(CodeBadRequest, "afterSeq must be >= 0")
	}
	if r.Limit == 0 {
		r.Limit = defaultSyncLimit
	}
	if r.Limit < 1 || r.Limit > maxSyncLimit {
		return Errorf(CodeBadRequest, "limit must be 1-%d", maxSyncLimit)
	}
	return nil
}

// SyncAck 是 sync 的应答，messages 按 seq 升序。
type SyncAck struct {
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"hasMore"`
}

// ReadReq 对应 cmd=read。
type ReadReq struct {
	ConvID string `json:"convId"`
	Seq    int64  `json:"seq"`
}

func (r *ReadReq) Validate() *Error {
	if _, err := ParseConvID(r.ConvID); err != nil {
		return err
	}
	if r.Seq < 1 {
		return Errorf(CodeBadRequest, "seq must be >= 1")
	}
	return nil
}

// Empty 是没有数据的成功应答。
type Empty struct{}

// Conv 是解析后的会话 ID。c2c 的 A <= B。
type Conv struct {
	Type string
	A, B string // c2c 两个用户；group / room 只用 A
}

// C2CConvID 生成单聊会话 ID，与参数顺序无关。
func C2CConvID(u1, u2 string) string {
	if u1 > u2 {
		u1, u2 = u2, u1
	}
	return ConvC2C + ":" + u1 + ":" + u2
}

// GroupConvID / RoomConvID 生成群聊 / 聊天室会话 ID。
func GroupConvID(gid string) string { return ConvGroup + ":" + gid }
func RoomConvID(rid string) string  { return ConvRoom + ":" + rid }

// ParseConvID 解析并校验会话 ID。
func ParseConvID(s string) (Conv, *Error) {
	bad := Errorf(CodeBadRequest, "invalid convId %q", s)
	parts := strings.Split(s, ":")
	switch {
	case len(parts) == 3 && parts[0] == ConvC2C:
		if !ValidID(parts[1]) || !ValidID(parts[2]) || parts[1] > parts[2] {
			return Conv{}, bad
		}
		return Conv{Type: ConvC2C, A: parts[1], B: parts[2]}, nil
	case len(parts) == 2 && (parts[0] == ConvGroup || parts[0] == ConvRoom):
		if !ValidID(parts[1]) {
			return Conv{}, bad
		}
		return Conv{Type: parts[0], A: parts[1]}, nil
	}
	return Conv{}, bad
}

// Has 判断用户是否是单聊会话的一方。
func (c Conv) Has(uid string) bool { return c.Type == ConvC2C && (c.A == uid || c.B == uid) }
