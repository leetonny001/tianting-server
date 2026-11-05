package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 客户端群管理命令（PRD-social 第 7 节，BE-131～BE-135）。权限在存储层持有群锁时判定（store.GroupCan），不缓存角色。
// 日志不记录公告原文（F-SOC-27 规则 5）。

func (s *Service) handleGroup(ctx context.Context, c delivery.Session, in protocol.InFrame) (data any, err *protocol.Error, ok bool) {
	switch in.Cmd {
	case protocol.CmdGroupCreate:
		data, err = s.groupCreate(ctx, c, in.Data)
	case protocol.CmdGroupInvite:
		data, err = s.groupInvite(ctx, c, in.Data)
	case protocol.CmdGroupLeave:
		data, err = s.groupLeave(ctx, c, in.Data)
	case protocol.CmdGroupKick:
		data, err = s.groupKick(ctx, c, in.Data)
	case protocol.CmdGroupTransfer:
		data, err = s.groupTransfer(ctx, c, in.Data)
	case protocol.CmdGroupSetAdmin:
		data, err = s.groupSetAdmin(ctx, c, in.Data)
	case protocol.CmdGroupDissolve:
		data, err = s.groupDissolve(ctx, c, in.Data)
	case protocol.CmdGroupUpdate:
		data, err = s.groupUpdate(ctx, c, in.Data)
	case protocol.CmdGroupInfo:
		data, err = s.groupInfo(ctx, c, in.Data)
	case protocol.CmdGroupMembers:
		data, err = s.groupMembersCmd(ctx, c, in.Data)
	default:
		return nil, nil, false
	}
	return data, err, true
}

// groupErr 把存储层错误映射为协议错误。
func (s *Service) groupErr(op string, err error) *protocol.Error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return protocol.Errorf(protocol.CodeGroupNotFound, "group not found")
	case errors.Is(err, store.ErrNotGroupMember):
		return protocol.Errorf(protocol.CodeNotGroupMember, "not a group member")
	case errors.Is(err, store.ErrGroupForbidden):
		return protocol.Errorf(protocol.CodeForbidden, "no permission")
	case errors.Is(err, store.ErrTargetNotMember):
		return protocol.Errorf(protocol.CodeTargetNotGroupMember, "target is not a group member")
	case errors.Is(err, store.ErrLimitExceeded):
		return protocol.Errorf(protocol.CodeGroupMemberLimitExceeded, "group member limit exceeded")
	case errors.Is(err, store.ErrAdminLimit):
		return protocol.Errorf(protocol.CodeGroupAdminLimitExceeded, "group admin limit exceeded")
	}
	return s.internal(op, err)
}

// groupOpAllowed 校验群写命令的公共部分：groupId 格式 → 限流 groupOp。
func (s *Service) groupOpAllowed(uid, gid string) *protocol.Error {
	if !protocol.ValidID(gid) {
		return protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	if !s.social.groupOp.Allow(uid) {
		return rateLimited()
	}
	return nil
}

// normalizeGroupMembers 校验并去重（保持顺序）。
func normalizeGroupMembers(members []string) ([]string, *protocol.Error) {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if !protocol.ValidID(m) {
			return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid member id")
		}
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out, nil
}

// deliverTip 把同一事务里生成的提示消息投递给在线成员。
func (s *Service) deliverTip(ctx context.Context, tip *store.Message, exceptConn string) {
	if tip == nil {
		return
	}
	conv, _ := protocol.ParseConvID(tip.ConvID)
	members, err := s.store.GroupMembers(ctx, conv.A)
	if err != nil {
		s.log.Error("load members for tip", "err", err)
		return
	}
	s.fanout(*tip, members, "") // 操作者的连接也要收到：ack 里没有提示消息本身（AC-UI-39-1）
	if tip.From != "" {
		s.pushReadUpdated(tip.From, tip.ConvID, tip.Seq, exceptConn) // 发言即已读（F-SOC-29 规则 5）
	}
}

func (s *Service) invalidateGroup(gid string) { s.inv.Invalidate(TopicGroupMembers, gid) }

// groupCreate：F-SOC-21。
func (s *Service) groupCreate(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupCreateReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if n := len(req.ClientReqID); n < 8 || n > 64 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "clientReqId length must be 8-64")
	}
	members, perr := normalizeGroupMembers(req.Members)
	if perr != nil {
		return nil, perr
	}
	if slices.Contains(req.Members, uid) { // AC-SOC-21-3：members 含自己 4000（测试 `[自己, bob]`：去掉自己后只有 1 人）
		members = slices.DeleteFunc(members, func(m string) bool { return m == uid })
	}
	if len(members) < 2 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "at least 2 members besides yourself")
	}
	name := ""
	if req.Name != nil {
		name = protocol.TrimText(*req.Name)
		if name == "" || protocol.TextLen(name) > protocol.MaxGroupNameLen {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "group name must be 1-%d", protocol.MaxGroupNameLen)
		}
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	if !s.social.groupOp.Allow(uid) {
		return nil, rateLimited()
	}
	if !cfg.ClientGroupEnabled {
		return nil, protocol.Errorf(protocol.CodeForbidden, "creating groups is disabled")
	}
	if len(members)+1 > cfg.ClientGroupMaxMembers {
		return nil, protocol.Errorf(protocol.CodeGroupMemberLimitExceeded, "group member limit exceeded")
	}
	// 配额在存储层判重之后才生效：重试已经成功的建群不受配额影响（规则 1）；失败和判重命中都退还。
	allowed := s.social.groupCreateDay.Allow(uid)
	gid := "ug-" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	got, dup, tip, failed, err := s.store.CreateClientGroup(ctx, store.CreateClientGroupParams{
		UserID: uid, ClientReqID: req.ClientReqID, GroupID: gid, Name: name, Members: members,
		MaxMembers: cfg.ClientGroupMaxMembers, MaxJoined: cfg.MaxJoinedGroups, Now: s.nowMs(), QuotaDenied: !allowed,
	})
	switch {
	case errors.Is(err, store.ErrCreateQuota):
		return nil, quotaExceeded()
	case errors.Is(err, store.ErrInviteeNotAllowed):
		s.social.groupCreateDay.Refund(uid) // 配额只算成功的建群
		e := protocol.Errorf(protocol.CodeNotFriend, "not allowed to invite this user")
		e.UserID = failed
		return nil, e
	case errors.Is(err, store.ErrJoinedGroupLimit):
		s.social.groupCreateDay.Refund(uid)
		e := protocol.Errorf(protocol.CodeJoinedGroupLimitExceeded, "joined group limit exceeded")
		e.UserID = failed
		return nil, e
	case err != nil:
		return nil, s.internal("create group", err)
	}
	if dup && allowed {
		s.social.groupCreateDay.Refund(uid)
	}
	convID := protocol.GroupConvID(got)
	if !dup {
		s.invalidateGroup(got)
		s.notifyConvAdded(ctx, convID, append(slices.Clone(members), uid), uid, c.ConnID())
		s.deliverTip(ctx, tip, c.ConnID())
	}
	row, ok, err := s.store.GetConversation(ctx, uid, convID)
	if err != nil {
		return nil, s.internal("load conversation", err)
	}
	if !ok { // 重复提交时群可能已经解散或我已退出
		return nil, protocol.Errorf(protocol.CodeGroupNotFound, "group not found")
	}
	return protocol.GroupCreateAck{GroupID: got, Conv: toConversation(row)}, nil
}

// groupInvite：F-SOC-22。
func (s *Service) groupInvite(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupMembersReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.GroupID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	members, perr := normalizeGroupMembers(req.Members)
	if perr != nil {
		return nil, perr
	}
	if len(members) == 0 || len(members) > protocol.MaxGroupOpBatch {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "members must have 1-%d items", protocol.MaxGroupOpBatch)
	}
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	cfg, perr := s.socialSettings(ctx)
	if perr != nil {
		return nil, perr
	}
	added, failed, tip, err := s.store.InviteMembers(ctx, req.GroupID, uid, members, cfg.MaxJoinedGroups, s.nowMs())
	if err != nil {
		return nil, s.groupErr("invite members", err)
	}
	out := protocol.GroupInviteAck{Added: added, Failed: make([]protocol.InviteFailed, 0, len(failed))}
	if out.Added == nil {
		out.Added = []string{}
	}
	for _, f := range failed {
		code := protocol.CodeNotFriend
		if errors.Is(f.Err, store.ErrJoinedGroupLimit) {
			code = protocol.CodeJoinedGroupLimitExceeded
		}
		out.Failed = append(out.Failed, protocol.InviteFailed{UserID: f.UserID, Code: code})
	}
	if len(added) > 0 {
		s.invalidateGroup(req.GroupID)
		s.NotifyConvAdded(ctx, protocol.GroupConvID(req.GroupID), added)
		s.deliverTip(ctx, tip, c.ConnID())
		s.NotifyGroupChanged(req.GroupID)
	}
	return out, nil
}

// groupKick：F-SOC-24。
func (s *Service) groupKick(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupMembersReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.GroupID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	members, perr := normalizeGroupMembers(req.Members)
	if perr != nil {
		return nil, perr
	}
	if len(members) == 0 || len(members) > protocol.MaxGroupOpBatch {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "members must have 1-%d items", protocol.MaxGroupOpBatch)
	}
	if slices.Contains(members, uid) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "use groupLeave to leave the group")
	}
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	removed, err := s.store.KickMembers(ctx, req.GroupID, uid, members)
	if err != nil {
		return nil, s.groupErr("kick members", err)
	}
	if removed == nil {
		removed = []string{}
	}
	if len(removed) > 0 {
		s.invalidateGroup(req.GroupID)
		s.NotifyConvRemoved(protocol.GroupConvID(req.GroupID), removed, protocol.RemovedByAdmin)
		s.NotifyGroupChanged(req.GroupID)
	}
	return protocol.GroupKickAck{Removed: removed}, nil
}

// groupLeave：F-SOC-23。
func (s *Service) groupLeave(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupIDReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	res, err := s.store.LeaveGroup(ctx, req.GroupID, uid, s.nowMs())
	if err != nil {
		return nil, s.groupErr("leave group", err)
	}
	s.invalidateGroup(req.GroupID)
	convID := protocol.GroupConvID(req.GroupID)
	s.pushEvent(uid, protocol.ConvRemovedEvent{Type: protocol.EventConvRemoved, ConvID: convID, Reason: protocol.RemovedLeft}, c.ConnID())
	if !res.Dissolved {
		s.deliverTip(ctx, res.Tip, "")
		s.NotifyGroupChanged(req.GroupID)
	}
	return protocol.Empty{}, nil
}

// groupTransfer：F-SOC-25 规则 1。
func (s *Service) groupTransfer(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupTransferReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.To) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid to")
	}
	if req.To == uid {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "cannot transfer to yourself")
	}
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	tip, err := s.store.TransferOwner(ctx, req.GroupID, uid, req.To, s.nowMs())
	if err != nil {
		return nil, s.groupErr("transfer owner", err)
	}
	s.deliverTip(ctx, tip, c.ConnID())
	s.NotifyGroupChanged(req.GroupID)
	return protocol.Empty{}, nil
}

// groupSetAdmin：F-SOC-25 规则 2。
func (s *Service) groupSetAdmin(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupSetAdminReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.UserID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
	}
	if req.Admin == nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "admin is required")
	}
	if req.UserID == uid {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "cannot change your own role")
	}
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	tip, err := s.store.SetGroupRole(ctx, req.GroupID, uid, req.UserID, *req.Admin, s.nowMs())
	if err != nil {
		return nil, s.groupErr("set group role", err)
	}
	if tip != nil {
		s.deliverTip(ctx, tip, c.ConnID())
		s.NotifyGroupChanged(req.GroupID)
	}
	return protocol.Empty{}, nil
}

// groupDissolve：F-SOC-26 规则 1。
func (s *Service) groupDissolve(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupIDReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	members, err := s.store.ClientDissolveGroup(ctx, req.GroupID, uid, s.nowMs())
	if err != nil {
		return nil, s.groupErr("dissolve group", err)
	}
	s.invalidateGroup(req.GroupID)
	convID := protocol.GroupConvID(req.GroupID)
	for _, m := range members {
		except := ""
		if m == uid {
			except = c.ConnID()
		}
		s.pushEvent(m, protocol.ConvRemovedEvent{Type: protocol.EventConvRemoved, ConvID: convID, Reason: protocol.RemovedDissolved}, except)
	}
	return protocol.Empty{}, nil
}

// groupUpdate：F-SOC-27。
func (s *Service) groupUpdate(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupUpdateReq](raw)
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !protocol.ValidID(req.GroupID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	if req.Name == nil && req.Notice == nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "nothing to update")
	}
	var name, notice *string
	if req.Name != nil {
		n := protocol.TrimText(*req.Name)
		if n == "" || protocol.TextLen(n) > protocol.MaxGroupNameLen {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "group name must be 1-%d", protocol.MaxGroupNameLen)
		}
		name = &n
	}
	if req.Notice != nil {
		n := protocol.TrimText(*req.Notice)
		if protocol.TextLen(n) > protocol.MaxNoticeLen {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "notice longer than %d", protocol.MaxNoticeLen)
		}
		notice = &n
	}
	if perr := s.groupOpAllowed(uid, req.GroupID); perr != nil {
		return nil, perr
	}
	tips, err := s.store.UpdateGroupProfile(ctx, req.GroupID, uid, name, notice, s.nowMs())
	if err != nil {
		return nil, s.groupErr("update group", err)
	}
	for i := range tips {
		s.deliverTip(ctx, &tips[i], c.ConnID())
	}
	s.NotifyGroupChanged(req.GroupID)
	return protocol.Empty{}, nil
}

// groupInfo：F-SOC-20 规则 3。
func (s *Service) groupInfo(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupIDReq](raw)
	if perr != nil {
		return nil, perr
	}
	if !protocol.ValidID(req.GroupID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	if !s.social.groupRead.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	g, err := s.store.GetGroup(ctx, req.GroupID)
	if err != nil {
		return nil, s.groupErr("get group", err)
	}
	role, err := s.memberRole(ctx, req.GroupID, c.UserID())
	if err != nil {
		return nil, s.groupErr("group member", err)
	}
	return protocol.GroupInfoAck{
		GroupID: g.GroupID, Name: g.Name, Notice: g.Notice, NoticeBy: g.NoticeBy, NoticeAt: g.NoticeAt, Owner: g.OwnerID,
		Managed: g.Managed(), MyRole: role, MemberCount: g.MemberCount, MaxMembers: g.MaxMembers,
		HistoryVisible: g.HistoryVisible, CreatedAt: g.CreatedAt,
	}, nil
}

func (s *Service) memberRole(ctx context.Context, gid, uid string) (string, error) {
	row, ok, err := s.store.GetConversation(ctx, uid, protocol.GroupConvID(gid))
	if err != nil {
		return "", err
	}
	if !ok {
		return "", store.ErrNotGroupMember
	}
	return row.MyRole, nil
}

// memberCursor 是成员列表的游标（不透明、HMAC，绑定群）。
type memberCursor struct {
	G string `json:"g"`
	R int    `json:"r"`
	T int64  `json:"t"`
	U string `json:"u"`
}

func (s *Service) signed(b []byte) string {
	mac := hmac.New(sha256.New, s.social.cursorKey)
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) verified(raw string) ([]byte, bool) {
	body, sig, ok := strings.Cut(raw, ".")
	if !ok {
		return nil, false
	}
	b, err1 := base64.RawURLEncoding.DecodeString(body)
	want, err2 := base64.RawURLEncoding.DecodeString(sig)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	mac := hmac.New(sha256.New, s.social.cursorKey)
	mac.Write(b)
	return b, hmac.Equal(mac.Sum(nil), want)
}

// groupMembersCmd：F-SOC-28。
func (s *Service) groupMembersCmd(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	req, perr := decode[protocol.GroupMembersPageReq](raw)
	if perr != nil {
		return nil, perr
	}
	if !protocol.ValidID(req.GroupID) {
		return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
	}
	limit := protocol.DefaultMembersLimit
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 || limit > protocol.MaxMembersLimit {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1-%d", protocol.MaxMembersLimit)
	}
	var after *store.MemberKey
	if req.Cursor != nil && *req.Cursor != "" {
		b, ok := s.verified(*req.Cursor)
		var cur memberCursor
		if !ok || json.Unmarshal(b, &cur) != nil || cur.G != req.GroupID {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor")
		}
		after = &store.MemberKey{Rank: cur.R, T: cur.T, User: cur.U}
	}
	if !s.social.groupRead.Allow(c.UserID()) {
		return nil, rateLimited()
	}
	if _, err := s.store.GetGroup(ctx, req.GroupID); err != nil {
		return nil, s.groupErr("get group", err)
	}
	if _, err := s.memberRole(ctx, req.GroupID, c.UserID()); err != nil {
		return nil, s.groupErr("group member", err)
	}
	rows, next, err := s.store.ListGroupMembers(ctx, req.GroupID, after, limit)
	if err != nil {
		return nil, s.internal("list group members", err)
	}
	out := protocol.GroupMembersAck{Members: make([]protocol.GroupMemberItem, 0, len(rows))}
	for _, r := range rows {
		out.Members = append(out.Members, protocol.GroupMemberItem{UserID: r.UserID, Role: r.Role, JoinedAt: r.JoinedAt, Nickname: r.Nickname, Avatar: r.Avatar, Official: r.Official})
	}
	if next != nil {
		b, _ := json.Marshal(memberCursor{G: req.GroupID, R: next.Rank, T: next.T, U: next.User})
		out.NextCursor = s.signed(b)
	}
	return out, nil
}

// notifyConvAdded 同 NotifyConvAdded，但跳过发起操作的连接（ack 已经带了 conv）。
func (s *Service) notifyConvAdded(ctx context.Context, convID string, userIDs []string, actor, exceptConn string) {
	conv, perr := protocol.ParseConvID(convID)
	if perr != nil {
		return
	}
	rows, err := s.store.GroupConversations(ctx, conv.A, userIDs)
	if err != nil {
		s.log.Error("load conversations for convAdded", "err", err, "conv", convID)
		return
	}
	allMuted := s.withAllMuted(ctx, conv.A)
	for _, uid := range userIDs {
		if r, ok := rows[uid]; ok {
			except := ""
			if uid == actor {
				except = exceptConn
			}
			c := toConversation(r)
			c.AllMuted = allMuted
			s.pushEvent(uid, protocol.ConvAddedEvent{Type: protocol.EventConvAdded, Conv: c}, except)
		}
	}
}

// DeliverTip 给管理接口用：投递它生成的 ownerChanged（from=""）。
func (s *Service) DeliverTip(ctx context.Context, tip *store.Message) { s.deliverTip(ctx, tip, "") }

// ---- groupChanged 合并（F-SOC-20 规则 5） ----

const groupChangedWindow = time.Second

// groupChangedMerger：同一个群至少间隔 1 秒：窗口内第一次立即发，之后的合并成窗口结束时的一条。
// 只是提醒，丢了靠提示消息和下一次 convs；用真实时间（测试时钟冻结也要发）。
type groupChangedMerger struct {
	send func(gid string)
	mu   sync.Mutex
	win  map[string]*groupWindow
}

type groupWindow struct{ dirty bool }

func newGroupChangedMerger(send func(string)) *groupChangedMerger {
	return &groupChangedMerger{send: send, win: map[string]*groupWindow{}}
}

func (m *groupChangedMerger) notify(gid string) {
	m.mu.Lock()
	if w, ok := m.win[gid]; ok {
		w.dirty = true
		m.mu.Unlock()
		return
	}
	m.win[gid] = &groupWindow{}
	m.mu.Unlock()
	m.send(gid)
	time.AfterFunc(groupChangedWindow, func() { m.flush(gid) })
}

func (m *groupChangedMerger) flush(gid string) {
	m.mu.Lock()
	w := m.win[gid]
	if w == nil || !w.dirty {
		delete(m.win, gid)
		m.mu.Unlock()
		return
	}
	w.dirty = false
	m.mu.Unlock()
	m.send(gid)
	time.AfterFunc(groupChangedWindow, func() { m.flush(gid) })
}

// NotifyGroupChanged 提醒在线成员某个群的资料或成员变了（合并发送）。
func (s *Service) NotifyGroupChanged(gid string) { s.changed.notify(gid) }

func (s *Service) deliverGroupChanged(gid string) {
	members, err := s.store.GroupMembers(context.Background(), gid)
	if err != nil {
		s.log.Warn("load members for groupChanged", "err", err)
		return
	}
	for _, uid := range members {
		s.pushEvent(uid, protocol.GroupChangedEvent{Type: protocol.EventGroupChanged, GroupID: gid}, "")
	}
}
