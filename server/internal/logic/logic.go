// Package logic 处理上行命令：发消息、补拉、已读。负责权限校验、落库和在线投递。
package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"im-mvp/server/internal/hub"
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/store"
)

// Service 实现 gateway.Handler。
type Service struct {
	store store.Store
	hub   *hub.Hub
	log   *slog.Logger
	now   func() time.Time
	newID func() string
}

func New(st store.Store, h *hub.Hub, log *slog.Logger) *Service {
	return &Service{
		store: st,
		hub:   h,
		log:   log,
		now:   time.Now,
		// UUIDv7 按时间递增，作为主键插入时对 B 树更友好。
		newID: func() string { return uuid.Must(uuid.NewV7()).String() },
	}
}

// Handle 分发一条上行帧并返回 ack。
func (s *Service) Handle(ctx context.Context, c hub.Client, in protocol.InFrame) protocol.OutFrame {
	if in.ID == "" {
		return protocol.AckErr("", protocol.Errorf(protocol.CodeBadRequest, "missing id"))
	}
	var (
		data any
		err  *protocol.Error
	)
	switch in.Cmd {
	case protocol.CmdSend:
		data, err = s.send(ctx, c, in.Data)
	case protocol.CmdSync:
		data, err = s.sync(ctx, c, in.Data)
	case protocol.CmdRead:
		data, err = s.read(ctx, c, in.Data)
	case protocol.CmdJoinRoom, protocol.CmdLeaveRoom:
		err = protocol.Errorf(protocol.CodeBadRequest, "rooms are not available yet")
	default:
		err = protocol.Errorf(protocol.CodeUnknownCmd, "unknown cmd %q", in.Cmd)
	}
	if err != nil {
		return protocol.AckErr(in.ID, err)
	}
	return protocol.AckOK(in.ID, data)
}

func (s *Service) send(ctx context.Context, c hub.Client, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.SendReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	uid := c.UserID()

	var convID string
	var recipients []string
	switch req.ConvType {
	case protocol.ConvC2C:
		convID = protocol.C2CConvID(uid, req.To)
		recipients = []string{req.To}
	case protocol.ConvGroup:
		members, err := s.store.GroupMembers(ctx, req.To)
		if err != nil {
			return nil, s.internal("load group members", err)
		}
		if !slices.Contains(members, uid) {
			if len(members) == 0 {
				return nil, protocol.Errorf(protocol.CodeNotFound, "group %s not found", req.To)
			}
			return nil, protocol.Errorf(protocol.CodeForbidden, "not a member of group %s", req.To)
		}
		convID = protocol.GroupConvID(req.To)
		recipients = members
	default:
		return nil, protocol.Errorf(protocol.CodeBadRequest, "rooms are not available yet")
	}

	body, err := json.Marshal(req.Body)
	if err != nil {
		return nil, s.internal("marshal body", err)
	}
	saved, dup, err := s.store.SaveMessage(ctx, store.Message{
		MsgID:       s.newID(),
		ClientMsgID: req.ClientMsgID,
		ConvID:      convID,
		From:        uid,
		Type:        req.Type,
		Body:        body,
		TS:          s.now().UnixMilli(),
	})
	if err != nil {
		return nil, s.internal("save message", err)
	}
	// 重发的消息第一次已经投递过，不再投递；没收到的端会靠 sync 补齐。
	if !dup {
		s.fanout(saved, recipients, c.ConnID())
	}
	return protocol.SendAck{MsgID: saved.MsgID, Seq: saved.Seq, TS: saved.TS}, nil
}

// fanout 投递给在线的接收者，以及发送者自己的其他设备。离线用户不处理，上线后 sync。
func (s *Service) fanout(m store.Message, recipients []string, senderConn uint64) {
	frame, err := json.Marshal(protocol.OutFrame{Cmd: protocol.CmdMsg, Data: toProto(m)})
	if err != nil {
		s.log.Error("marshal msg frame", "err", err)
		return
	}
	for _, uid := range recipients {
		if uid != m.From {
			s.hub.SendToUser(uid, frame, 0)
		}
	}
	s.hub.SendToUser(m.From, frame, senderConn)
}

func (s *Service) sync(ctx context.Context, c hub.Client, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.SyncReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if perr := s.authorize(ctx, c.UserID(), req.ConvID); perr != nil {
		return nil, perr
	}
	msgs, hasMore, err := s.store.ListMessages(ctx, req.ConvID, req.AfterSeq, req.Limit)
	if err != nil {
		return nil, s.internal("list messages", err)
	}
	out := protocol.SyncAck{Messages: make([]protocol.Message, 0, len(msgs)), HasMore: hasMore}
	for _, m := range msgs {
		out.Messages = append(out.Messages, toProto(m))
	}
	return out, nil
}

func (s *Service) read(ctx context.Context, c hub.Client, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.ReadReq](raw)
	if perr != nil {
		return nil, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, perr
	}
	if perr := s.authorize(ctx, c.UserID(), req.ConvID); perr != nil {
		return nil, perr
	}
	if err := s.store.SetReadSeq(ctx, c.UserID(), req.ConvID, req.Seq); err != nil {
		return nil, s.internal("set read seq", err)
	}
	return protocol.Empty{}, nil
}

// authorize 检查用户能否访问会话：单聊必须是其中一方，群聊必须是成员。
func (s *Service) authorize(ctx context.Context, uid, convID string) *protocol.Error {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return perr
	}
	switch conv.Type {
	case protocol.ConvC2C:
		if !conv.Has(uid) {
			return protocol.Errorf(protocol.CodeForbidden, "not a participant of %s", convID)
		}
	case protocol.ConvGroup:
		ok, err := s.store.IsGroupMember(ctx, conv.A, uid)
		if err != nil {
			return s.internal("check group member", err)
		}
		if !ok {
			return protocol.Errorf(protocol.CodeForbidden, "not a member of %s", convID)
		}
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "room messages cannot be synced")
	}
	return nil
}

func (s *Service) internal(op string, err error) *protocol.Error {
	s.log.Error(op, "err", err)
	return protocol.Errorf(protocol.CodeInternal, "internal error")
}

// decode 严格解析 data：未知字段直接报错，和 Schema 的 additionalProperties=false 一致。
func decode[T any](raw json.RawMessage) (T, *protocol.Error) {
	var v T
	if len(raw) == 0 {
		return v, protocol.Errorf(protocol.CodeBadRequest, "missing data")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, protocol.Errorf(protocol.CodeBadRequest, "invalid data: %v", err)
	}
	return v, nil
}

func toProto(m store.Message) protocol.Message {
	var body protocol.MsgBody
	_ = json.Unmarshal(m.Body, &body) // Body 是我们自己写入的，格式可信
	return protocol.Message{
		MsgID:       m.MsgID,
		ClientMsgID: m.ClientMsgID,
		ConvID:      m.ConvID,
		Seq:         m.Seq,
		From:        m.From,
		Type:        m.Type,
		Body:        body,
		TS:          m.TS,
	}
}
