// Package logic 处理上行命令：发消息、补拉、已读。负责权限校验、落库和在线投递。
package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"im-mvp/server/internal/clock"
	"im-mvp/server/internal/hub"
	"im-mvp/server/internal/protocol"
	"im-mvp/server/internal/ratelimit"
	"im-mvp/server/internal/store"
)

// Service 实现 gateway.Handler。
type Service struct {
	store   store.Store
	hub     *hub.Hub
	log     *slog.Logger
	clock   clock.Clock
	faults  *Faults // 仅测试模式非 nil
	limiter *ratelimit.Limiter
	newID   func() string
}

// New 创建 Service。sendRate 为每个用户每秒可发的消息数（所有连接合计，PRD F-MSG-07）。
func New(st store.Store, h *hub.Hub, clk clock.Clock, faults *Faults, sendRate int, log *slog.Logger) *Service {
	return &Service{
		store:   st,
		hub:     h,
		log:     log,
		clock:   clk,
		faults:  faults,
		limiter: ratelimit.New(sendRate, sendRate, clk),
		// UUIDv7 按时间递增，作为主键插入时对 B 树更友好。
		newID: func() string { return uuid.Must(uuid.NewV7()).String() },
	}
}

// Handle 分发一条上行帧，返回要回给客户端的 ack；返回 nil 表示不回复（仅故障注入时）。
func (s *Service) Handle(ctx context.Context, c hub.Client, in protocol.InFrame) *protocol.OutFrame {
	if in.ID == "" {
		return ack(protocol.AckErr("", protocol.Errorf(protocol.CodeMissingRequestID, "missing id")))
	}
	var (
		data any
		err  *protocol.Error
	)
	switch in.Cmd {
	case protocol.CmdSend:
		var drop bool
		data, drop, err = s.send(ctx, c, in.Data)
		if drop {
			return nil
		}
	case protocol.CmdSync:
		data, err = s.sync(ctx, c, in.Data)
	case protocol.CmdRead:
		data, err = s.read(ctx, c, in.Data)
	case protocol.CmdConvs, protocol.CmdJoinRoom, protocol.CmdLeaveRoom:
		err = protocol.Errorf(protocol.CodeBadRequest, "%s is not available yet", in.Cmd)
	default:
		err = protocol.Errorf(protocol.CodeUnknownCmd, "unknown cmd %q", in.Cmd)
	}
	if err != nil {
		return ack(protocol.AckErr(in.ID, err))
	}
	return ack(protocol.AckOK(in.ID, data))
}

func ack(f protocol.OutFrame) *protocol.OutFrame { return &f }

// send 返回 ack 数据；drop=true 表示按故障注入丢弃 ack。
func (s *Service) send(ctx context.Context, c hub.Client, raw json.RawMessage) (any, bool, *protocol.Error) {
	req, perr := decode[protocol.SendReq](raw)
	if perr != nil {
		return nil, false, perr
	}
	if perr := req.Validate(); perr != nil {
		return nil, false, perr
	}
	uid := c.UserID()

	// 先识别重发（IMP-3）：命中时直接返回第一次的结果，不扣令牌、不重复投递。
	if prev, found, err := s.store.FindByClientMsgID(ctx, uid, req.ClientMsgID); err != nil {
		return nil, false, s.internal("find by clientMsgId", err)
	} else if found {
		if prev.ConvID != targetConvID(req, uid) {
			return nil, false, protocol.Errorf(protocol.CodeClientMsgIDConflict, "clientMsgId already used in another conversation")
		}
		return protocol.SendAck{MsgID: prev.MsgID, ConvID: prev.ConvID, Seq: prev.Seq, TS: prev.TS}, false, nil
	}
	// 限流（C-29）：被拒绝的请求不扣令牌。
	if !s.limiter.Allow(uid) {
		return nil, false, protocol.Errorf(protocol.CodeRateLimited, "too many messages")
	}

	var convID string
	var recipients []string
	switch req.ConvType {
	case protocol.ConvC2C:
		// 对方必须存在（D5）；给自己发不用查。
		if req.To != uid {
			exists, err := s.store.UserExists(ctx, req.To)
			if err != nil {
				return nil, false, s.internal("check user", err)
			}
			if !exists {
				return nil, false, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", req.To)
			}
		}
		convID = protocol.C2CConvID(uid, req.To)
		recipients = []string{req.To}
	case protocol.ConvGroup:
		members, perr := s.groupMembersFor(ctx, req.To, uid)
		if perr != nil {
			return nil, false, perr
		}
		convID = protocol.GroupConvID(req.To)
		recipients = members
	default:
		return nil, false, protocol.Errorf(protocol.CodeBadRequest, "rooms are not available yet")
	}

	body, err := json.Marshal(req.Body)
	if err != nil {
		return nil, false, s.internal("marshal body", err)
	}
	saved, dup, err := s.store.SaveMessage(ctx, store.Message{
		MsgID:       s.newID(),
		ClientMsgID: req.ClientMsgID,
		ConvID:      convID,
		From:        uid,
		Type:        req.Type,
		Body:        body,
		TS:          s.clock.Now().UnixMilli(),
	})
	if err != nil {
		return nil, false, s.internal("save message", err)
	}
	if dup && saved.ConvID != convID {
		return nil, false, protocol.Errorf(protocol.CodeClientMsgIDConflict, "clientMsgId already used in another conversation")
	}
	// 重发的消息第一次已经投递过，不再投递；没收到的端会靠 sync 补齐。
	if !dup && !s.faults.takeSkipDelivery(req.ClientMsgID) {
		s.fanout(saved, recipients, c.ConnID())
	}
	if s.faults.takeDropAck(req.ClientMsgID) {
		return nil, true, nil
	}
	return protocol.SendAck{MsgID: saved.MsgID, ConvID: saved.ConvID, Seq: saved.Seq, TS: saved.TS}, false, nil
}

// targetConvID 计算请求要发往的会话 ID（聊天室在二期实现）。
func targetConvID(req protocol.SendReq, uid string) string {
	switch req.ConvType {
	case protocol.ConvC2C:
		return protocol.C2CConvID(uid, req.To)
	case protocol.ConvGroup:
		return protocol.GroupConvID(req.To)
	}
	return protocol.RoomConvID(req.To)
}

// groupMembersFor 返回群成员；群不存在返回 4043，uid 不是成员返回 4033。
func (s *Service) groupMembersFor(ctx context.Context, gid, uid string) ([]string, *protocol.Error) {
	exists, err := s.store.GroupExists(ctx, gid)
	if err != nil {
		return nil, s.internal("check group", err)
	}
	if !exists {
		return nil, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", gid)
	}
	members, err := s.store.GroupMembers(ctx, gid)
	if err != nil {
		return nil, s.internal("load group members", err)
	}
	if !slices.Contains(members, uid) {
		return nil, protocol.Errorf(protocol.CodeNotGroupMember, "not a member of group %s", gid)
	}
	return members, nil
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
	limit, perr := req.Validate()
	if perr != nil {
		return nil, perr
	}
	if perr := s.authorize(ctx, c.UserID(), req.ConvID); perr != nil {
		return nil, perr
	}

	var msgs []store.Message
	var hasMore bool
	var err error
	if req.AfterSeq != nil {
		msgs, hasMore, err = s.store.ListMessages(ctx, req.ConvID, *req.AfterSeq, limit)
	} else {
		msgs, hasMore, err = s.store.ListMessagesBefore(ctx, req.ConvID, *req.BeforeSeq, 1, limit)
	}
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

// authorize 检查用户能否访问会话：单聊必须是其中一方（4032），群必须存在（4043）且是成员（4033）。
func (s *Service) authorize(ctx context.Context, uid, convID string) *protocol.Error {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return perr
	}
	switch conv.Type {
	case protocol.ConvC2C:
		if !conv.Has(uid) {
			return protocol.Errorf(protocol.CodeNotConvParticipant, "not a participant of %s", convID)
		}
	case protocol.ConvGroup:
		exists, err := s.store.GroupExists(ctx, conv.A)
		if err != nil {
			return s.internal("check group", err)
		}
		if !exists {
			return protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", conv.A)
		}
		ok, err := s.store.IsGroupMember(ctx, conv.A, uid)
		if err != nil {
			return s.internal("check group member", err)
		}
		if !ok {
			return protocol.Errorf(protocol.CodeNotGroupMember, "not a member of %s", convID)
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
