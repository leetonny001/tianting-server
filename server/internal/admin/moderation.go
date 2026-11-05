package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/store"
)

// 第二批管理接口（PRD-admin F-ADM-08、13、31、32、40、41）。IM_V13_ADMIN_FEATURES 关闭时不注册。

const (
	maxModReasonLen   = 200
	maxTicketLen      = 64
	minTimedSeconds   = 60
	maxTimedSeconds   = 30 * 24 * 3600
	maxDeleteBatch    = 50
	maxBodyPage       = 50
	msgGrantTTL       = 15 * time.Minute
	grantsPerOperator = 20  // F-ADM-08 规则 5
	grantsPerCaller   = 200 // F-ADM-08 规则 5
)

// 配额按 Asia/Shanghai 自然日。中国不使用夏令时，用固定时区，不依赖运行环境的 tzdata。
var shanghai = time.FixedZone("Asia/Shanghai", 8*3600)

func dayStart(t time.Time) int64 {
	y, m, d := t.In(shanghai).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, shanghai).UnixMilli()
}

// ModerationService 是业务层提供的事件下发（由 logic.Service 实现）。
type ModerationService interface {
	NotifyMuted(uid, scope, groupID string, until *int64)
	NotifyUnmuted(uid, scope, groupID string)
	NotifyGroupAllMuted(ctx context.Context, groupID string, until *int64)
	NotifyGroupAllUnmuted(ctx context.Context, groupID string)
	NotifyMsgDeleted(ctx context.Context, convID string, msgIDs []string, seqs []int64)
}

// ModerationConfig 是第二批管理接口的依赖。
type ModerationConfig struct {
	Service        ModerationService
	Cipher         *quarantine.Cipher
	QuarantineDays int
	C2CBodyAccess  bool
}

// WithModeration 打开第二批管理接口。要在 Register 之前调用。
func (a *Admin) WithModeration(cfg ModerationConfig) { a.mod = &cfg }

func (a *Admin) registerModeration(mux *http.ServeMux) {
	if a.mod == nil {
		return
	}
	a.route(mux, "POST", "/admin/users/{userId}/ban", adminauth.ScopeUserBan, a.banUser)
	a.route(mux, "POST", "/admin/users/{userId}/unban", adminauth.ScopeUserBan, a.unbanUser)
	a.route(mux, "POST", "/admin/messages/delete", adminauth.ScopeMsgDelete, a.deleteMessages)
	a.route(mux, "POST", "/admin/mute", adminauth.ScopeMuteWrite, a.mute)
	a.route(mux, "POST", "/admin/unmute", adminauth.ScopeMuteWrite, a.unmute)
	bodyScopes := []adminauth.Scope{adminauth.ScopeMsgBodyGroup, adminauth.ScopeMsgBodyC2C, adminauth.ScopeMsgQuarantine}
	a.routeAny(mux, "POST", "/admin/msg-grants", bodyScopes, a.createMsgGrant)
	a.routeAny(mux, "POST", "/admin/convs/messages/body", bodyScopes, a.readMessageBody)
}

// durationField 区分"没传"和"传了 null（永久）"：漏传时长不能变成永久封禁。
type durationField struct {
	set     bool
	seconds *int64
}

func (d *durationField) UnmarshalJSON(b []byte) error {
	d.set = true
	if string(b) == "null" {
		d.seconds = nil
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	d.seconds = &n
	return nil
}

// until 把时长换算成到期时间；nil 表示永久。
func (d durationField) until(now time.Time) (*int64, *protocol.Error) {
	if !d.set {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "durationSeconds is required (null for permanent)")
	}
	if d.seconds == nil {
		return nil, nil
	}
	if *d.seconds < minTimedSeconds || *d.seconds > maxTimedSeconds {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "durationSeconds must be %d..%d or null", minTimedSeconds, maxTimedSeconds)
	}
	ms := now.Add(time.Duration(*d.seconds) * time.Second).UnixMilli()
	return &ms, nil
}

func validReason(reason string) (string, *protocol.Error) {
	r := protocol.TrimText(reason)
	if r == "" || protocol.TextLen(r) > maxModReasonLen {
		return "", protocol.Errorf(protocol.CodeBadRequest, "reason is required and must be at most %d characters", maxModReasonLen)
	}
	return r, nil
}

func byOf(info AuthInfo) string { return info.Caller + ":" + info.Operator }

// bannedErr 是 403 / 4034，details.bannedUntil 永久为 null（F-ADM-13 规则 3）。
func bannedErr(until *int64) *protocol.Error {
	e := protocol.Errorf(protocol.CodeUserBanned, "user is banned")
	e.Details = map[string]any{"bannedUntil": until}
	return e
}

type banJSON struct {
	Until  *int64 `json:"until"`
	Reason string `json:"reason"`
	By     string `json:"by"`
	At     int64  `json:"at"`
}

func (a *Admin) banUser(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	var req struct {
		DurationSeconds durationField `json:"durationSeconds"`
		Reason          string        `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	reason, perr := validReason(req.Reason)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	now := a.clock.Now()
	until, perr := req.DurationSeconds.until(now)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	b := store.Ban{UserID: uid, Until: until, Reason: reason, By: byOf(AuthFrom(r.Context())), At: now.UnixMilli()}
	switch err := a.store.BanUser(r.Context(), b); {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", uid))
		return
	case err != nil:
		a.internal(w, "ban user", err)
		return
	}
	a.router.SuspendUser(uid, until)
	writeJSON(w, http.StatusOK, struct {
		UserID string  `json:"userId"`
		Ban    banJSON `json:"ban"`
	}{uid, banJSON{until, reason, b.By, b.At}})
}

func (a *Admin) unbanUser(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if _, perr := validReason(req.Reason); perr != nil {
		writeErr(w, perr)
		return
	}
	switch _, err := a.store.UnbanUser(r.Context(), uid); {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", uid))
	case err != nil:
		a.internal(w, "unban user", err)
	default:
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

// userModeration 是用户详情和列表追加的 ban、mutedUntil。第二批关闭时都是 nil（字段不出现）。
// ban：null 或对象，字段一定出现；mutedUntil：只有在禁言中才出现，永久为 null。过期的记录按没有处理。
func (a *Admin) userModeration(ctx context.Context, uid string) (ban, mutedUntil json.RawMessage, err error) {
	if a.mod == nil {
		return nil, nil, nil
	}
	now := a.clock.Now().UnixMilli()
	b, err := a.store.UserBan(ctx, uid)
	if err != nil {
		return nil, nil, err
	}
	ban = json.RawMessage("null")
	if b.Active(now) {
		if ban, err = json.Marshal(banJSON{b.Until, b.Reason, b.By, b.At}); err != nil {
			return nil, nil, err
		}
	}
	mutes, err := a.store.UserMutes(ctx, uid)
	if err != nil {
		return nil, nil, err
	}
	for i := range mutes {
		if mutes[i].Scope == store.MuteUser && mutes[i].Active(now) {
			mutedUntil = protocol.NullableMs(mutes[i].Until)
		}
	}
	return ban, mutedUntil, nil
}

func (a *Admin) deleteMessages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConvID string   `json:"convId"`
		MsgIDs []string `json:"msgIds"`
		Reason string   `json:"reason"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	conv, perr := protocol.ParseConvID(req.ConvID)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	if conv.Type != protocol.ConvC2C && conv.Type != protocol.ConvGroup {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "only c2c and group messages can be deleted"))
		return
	}
	if len(req.MsgIDs) == 0 {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "msgIds is empty"))
		return
	}
	if len(req.MsgIDs) > maxDeleteBatch {
		writeErr(w, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d messages per request", maxDeleteBatch))
		return
	}
	if _, perr := validReason(req.Reason); perr != nil {
		writeErr(w, perr)
		return
	}
	seen := make(map[string]bool, len(req.MsgIDs))
	ids := make([]string, 0, len(req.MsgIDs))
	for _, id := range req.MsgIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	now := a.clock.Now()
	deleted, err := a.store.DeleteMessages(r.Context(), store.DeleteMessagesReq{
		ConvID: req.ConvID, MsgIDs: ids, By: byOf(AuthFrom(r.Context())), Now: now.UnixMilli(),
		ExpiresAt: now.Add(time.Duration(a.mod.QuarantineDays) * 24 * time.Hour).UnixMilli(), Seal: a.mod.Cipher.Seal,
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeMessageNotFound, "message not found"))
		return
	case errors.Is(err, store.ErrMsgWrongConv):
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "message is not in conversation %s", req.ConvID))
		return
	case errors.Is(err, store.ErrMsgNotDeletable):
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "tip messages cannot be deleted"))
		return
	case err != nil:
		a.internal(w, "delete messages", err)
		return
	}
	type item struct {
		MsgID string `json:"msgId"`
		Seq   int64  `json:"seq"`
	}
	out := make([]item, len(deleted))
	msgIDs, seqs := make([]string, len(deleted)), make([]int64, len(deleted))
	for i, d := range deleted {
		out[i], msgIDs[i], seqs[i] = item{d.MsgID, d.Seq}, d.MsgID, d.Seq
	}
	if len(deleted) > 0 { // 全部已删除：幂等成功，不再发事件
		a.mod.Service.NotifyMsgDeleted(r.Context(), req.ConvID, msgIDs, seqs)
	}
	noteAuditCount(r.Context(), len(deleted))
	writeJSON(w, http.StatusOK, struct {
		Deleted []item `json:"deleted"`
	}{out})
}

type muteReq struct {
	Scope           string        `json:"scope"`
	UserID          string        `json:"userId"`
	GroupID         string        `json:"groupId"`
	DurationSeconds durationField `json:"durationSeconds"`
	Reason          string        `json:"reason"`
}

func (m *muteReq) validate() (string, *protocol.Error) {
	switch m.Scope {
	case protocol.MuteScopeUser:
		if m.GroupID != "" {
			return "", protocol.Errorf(protocol.CodeBadRequest, "groupId is only for scope group")
		}
	case protocol.MuteScopeGroup:
		if !protocol.ValidID(m.GroupID) {
			return "", protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
		}
	case protocol.MuteScopeGroupAll: // 全群禁言针对群，没有 userId；是开关，没有时长（F-ADM-42）
		if !protocol.ValidID(m.GroupID) {
			return "", protocol.Errorf(protocol.CodeInvalidID, "invalid groupId")
		}
		if m.UserID != "" {
			return "", protocol.Errorf(protocol.CodeBadRequest, "userId is not for scope groupAll")
		}
		if m.DurationSeconds.seconds != nil {
			return "", protocol.Errorf(protocol.CodeBadRequest, "durationSeconds is not for scope groupAll")
		}
		return validReason(m.Reason)
	default:
		return "", protocol.Errorf(protocol.CodeBadRequest, "scope must be user, group or groupAll")
	}
	if !protocol.ValidID(m.UserID) {
		return "", protocol.Errorf(protocol.CodeInvalidID, "invalid userId")
	}
	return validReason(m.Reason)
}

// writeMuteErr 把存储层的错误映射成错误码；err 为 nil 时返回 false。
func (a *Admin) writeMuteErr(w http.ResponseWriter, m *muteReq, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound) && (m.Scope == protocol.MuteScopeGroup || m.Scope == protocol.MuteScopeGroupAll):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group %s not found", m.GroupID))
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", m.UserID))
	case errors.Is(err, store.ErrTargetNotMember):
		writeErr(w, protocol.Errorf(protocol.CodeTargetNotGroupMember, "%s is not a member of %s", m.UserID, m.GroupID))
	default:
		a.internal(w, "mute", err)
	}
	return true
}

func (a *Admin) mute(w http.ResponseWriter, r *http.Request) {
	var req muteReq
	if !readJSON(w, r, &req) {
		return
	}
	reason, perr := req.validate()
	if perr != nil {
		writeErr(w, perr)
		return
	}
	now := a.clock.Now()
	if req.Scope == protocol.MuteScopeGroupAll {
		a.muteGroupAll(w, r, &req, reason, now)
		return
	}
	until, perr := req.DurationSeconds.until(now)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	err := a.store.SetMute(r.Context(), store.Mute{Scope: req.Scope, UserID: req.UserID, GroupID: req.GroupID, Until: until, Reason: reason, By: byOf(AuthFrom(r.Context())), At: now.UnixMilli()})
	if a.writeMuteErr(w, &req, err) {
		return
	}
	a.inv.Invalidate(logic.TopicMutes, req.UserID)
	a.mod.Service.NotifyMuted(req.UserID, req.Scope, req.GroupID, until)
	writeJSON(w, http.StatusOK, struct {
		MutedUntil *int64 `json:"mutedUntil"`
	}{until})
}

// muteGroupAll 开启全群禁言。重复开启幂等：已经开着时不再通知（和解除的"真有才通知"对称）。
func (a *Admin) muteGroupAll(w http.ResponseWriter, r *http.Request, req *muteReq, reason string, now time.Time) {
	old, err := a.store.GroupAllMute(r.Context(), req.GroupID)
	if err != nil {
		a.internal(w, "group mute", err)
		return
	}
	err = a.store.SetMute(r.Context(), store.Mute{Scope: req.Scope, GroupID: req.GroupID, Reason: reason, By: byOf(AuthFrom(r.Context())), At: now.UnixMilli()})
	if a.writeMuteErr(w, req, err) {
		return
	}
	a.inv.Invalidate(logic.TopicGroupAllMutes, req.GroupID)
	if !old.Active(now.UnixMilli()) {
		a.mod.Service.NotifyGroupAllMuted(r.Context(), req.GroupID, nil)
	}
	writeJSON(w, http.StatusOK, struct {
		MutedUntil *int64 `json:"mutedUntil"`
	}{nil})
}

func (a *Admin) unmute(w http.ResponseWriter, r *http.Request) {
	var req muteReq
	if !readJSON(w, r, &req) {
		return
	}
	if req.DurationSeconds.set {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "unmute does not take durationSeconds"))
		return
	}
	if _, perr := req.validate(); perr != nil {
		writeErr(w, perr)
		return
	}
	old, err := a.store.DeleteMute(r.Context(), req.Scope, req.UserID, req.GroupID)
	if a.writeMuteErr(w, &req, err) {
		return
	}
	if req.Scope == protocol.MuteScopeGroupAll {
		a.inv.Invalidate(logic.TopicGroupAllMutes, req.GroupID)
		if old.Active(a.clock.Now().UnixMilli()) {
			a.mod.Service.NotifyGroupAllUnmuted(r.Context(), req.GroupID)
		}
		writeJSON(w, http.StatusOK, struct{}{})
		return
	}
	a.inv.Invalidate(logic.TopicMutes, req.UserID)
	// 真有一条还在生效的禁言才通知：重复解除、过期后解除都不打扰客户端。
	if old.Active(a.clock.Now().UnixMilli()) {
		a.mod.Service.NotifyUnmuted(req.UserID, req.Scope, req.GroupID)
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

// grantScope 检查调用方对这个会话 / 隔离原文有没有 scope，以及单聊开关（X-7）。
func (a *Admin) grantScope(r *http.Request, conv protocol.Conv, wantQuarantine bool) *protocol.Error {
	c := a.callers[AuthFrom(r.Context()).Caller]
	scope := adminauth.ScopeMsgBodyGroup
	switch {
	case wantQuarantine:
		scope = adminauth.ScopeMsgQuarantine
	case conv.Type == protocol.ConvC2C:
		scope = adminauth.ScopeMsgBodyC2C
	}
	if c == nil || !c.Has(scope) {
		return protocol.Errorf(protocol.CodeForbidden, "caller lacks scope %s", scope)
	}
	if conv.Type == protocol.ConvC2C && !a.mod.C2CBodyAccess {
		e := protocol.Errorf(protocol.CodeForbidden, "c2c message body access is disabled")
		e.Reason = "c2cBodyDisabled"
		return e
	}
	return nil
}

func (a *Admin) createMsgGrant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConvID     string `json:"convId"`
		Operator   string `json:"operator"`
		Reason     string `json:"reason"`
		Ticket     string `json:"ticket"`
		Quarantine bool   `json:"quarantine"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	conv, perr := protocol.ParseConvID(req.ConvID)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	if conv.Type != protocol.ConvC2C && conv.Type != protocol.ConvGroup {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "only c2c and group conversations"))
		return
	}
	info := AuthFrom(r.Context())
	// 配额按操作人算：请求体里的 operator 必须和签过名的请求头一致，否则配额可以换个名字绕过。
	if req.Operator == "" || req.Operator != info.Operator {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "operator is required and must match X-IM-Operator"))
		return
	}
	reason, perr := validReason(req.Reason)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	ticket := strings.TrimSpace(req.Ticket)
	if ticket == "" || len(ticket) > maxTicketLen {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "ticket is required and must be at most %d characters", maxTicketLen))
		return
	}
	if perr := a.grantScope(r, conv, req.Quarantine); perr != nil {
		writeErr(w, perr)
		return
	}
	now := a.clock.Now()
	g := store.MsgGrant{
		GrantID: uuid.NewString(), Caller: info.Caller, Operator: req.Operator, ConvID: req.ConvID, Quarantine: req.Quarantine,
		Reason: reason, Ticket: ticket, CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(msgGrantTTL).UnixMilli(),
	}
	err := a.store.CreateMsgGrant(r.Context(), g, store.MsgGrantQuota{DayStart: dayStart(now), PerOperator: grantsPerOperator, PerCaller: grantsPerCaller})
	switch {
	case errors.Is(err, store.ErrMsgGrantQuota):
		writeErr(w, protocol.Errorf(protocol.CodeQuotaExceeded, "daily message read quota exceeded"))
		return
	case err != nil:
		a.internal(w, "create msg grant", err)
		return
	}
	noteAuditFields(r.Context(), map[string]any{"grantId": g.GrantID})
	writeJSON(w, http.StatusOK, struct {
		GrantID   string `json:"grantId"`
		ExpiresAt int64  `json:"expiresAt"`
	}{g.GrantID, g.ExpiresAt})
}

func grantRequired() *protocol.Error {
	return protocol.Errorf(protocol.CodeMsgReadGrantRequired, "no valid read grant for this conversation")
}

func (a *Admin) readMessageBody(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store") // 错误响应也不缓存（F-ADM-08 规则 8）
	var req struct {
		GrantID   string `json:"grantId"`
		ConvID    string `json:"convId"`
		BeforeSeq *int64 `json:"beforeSeq"`
		Limit     *int   `json:"limit"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	conv, perr := protocol.ParseConvID(req.ConvID)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	limit := maxBodyPage
	if req.Limit != nil {
		if *req.Limit < 1 || *req.Limit > maxBodyPage {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1..%d", maxBodyPage))
			return
		}
		limit = *req.Limit
	}
	before := int64(math.MaxInt64)
	if req.BeforeSeq != nil {
		if *req.BeforeSeq < 1 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "beforeSeq must be positive"))
			return
		}
		before = *req.BeforeSeq
	}
	g, err := a.store.GetMsgGrant(r.Context(), req.GrantID)
	if errors.Is(err, store.ErrMsgGrantNotFound) {
		writeErr(w, grantRequired())
		return
	}
	if err != nil {
		a.internal(w, "get msg grant", err)
		return
	}
	info := AuthFrom(r.Context())
	now := a.clock.Now().UnixMilli()
	if g.ConvID != req.ConvID || g.ExpiresAt <= now || g.Caller != info.Caller || g.Operator != info.Operator {
		writeErr(w, grantRequired())
		return
	}
	// 授权之后可能改了配置重启（关了单聊开关、收了 scope）：读的时候再查一次。
	if perr := a.grantScope(r, conv, g.Quarantine); perr != nil {
		writeErr(w, perr)
		return
	}
	var (
		msgs []bodyMessageJSON
		more bool
	)
	if g.Quarantine {
		msgs, more, perr = a.readQuarantine(r.Context(), req.ConvID, before, now, limit)
	} else {
		msgs, more, perr = a.readBodies(r.Context(), req.ConvID, before, limit)
	}
	if perr != nil {
		writeErr(w, perr)
		return
	}
	fields := map[string]any{"grantId": g.GrantID, "quarantine": g.Quarantine}
	if len(msgs) > 0 {
		fields["seqFrom"], fields["seqTo"] = msgs[0].Seq, msgs[len(msgs)-1].Seq
	}
	noteAuditFields(r.Context(), fields)
	noteAuditCount(r.Context(), len(msgs))
	writeJSON(w, http.StatusOK, struct {
		Messages []bodyMessageJSON `json:"messages"`
		HasMore  bool              `json:"hasMore"`
	}{msgs, more})
}

// bodyMessageJSON 是正文读取的一条消息：比下发给客户端的多一个删除人。
type bodyMessageJSON struct {
	protocol.Message
	DeletedBy string `json:"deletedBy,omitempty"`
}

func (a *Admin) readBodies(ctx context.Context, convID string, before int64, limit int) ([]bodyMessageJSON, bool, *protocol.Error) {
	rows, more, err := a.store.ListMessagesBefore(ctx, convID, before, 1, limit)
	if err != nil {
		a.log.Error("list messages for body read", "err", err)
		return nil, false, protocol.Errorf(protocol.CodeInternal, "internal error")
	}
	out := make([]bodyMessageJSON, len(rows))
	for i, m := range rows {
		out[i] = bodyMessageJSON{logic.ToProto(m), m.DeletedBy}
	}
	return out, more, nil
}

// readQuarantine 解密隔离原文。一条都没有（从未删除或已到期）返回 404 / 4044（F-ADM-32 规则 4）。
func (a *Admin) readQuarantine(ctx context.Context, convID string, before, now int64, limit int) ([]bodyMessageJSON, bool, *protocol.Error) {
	rows, more, err := a.store.ListQuarantined(ctx, convID, before, now, limit)
	if err != nil {
		a.log.Error("list quarantined", "err", err)
		return nil, false, protocol.Errorf(protocol.CodeInternal, "internal error")
	}
	if len(rows) == 0 {
		return nil, false, protocol.Errorf(protocol.CodeMessageNotFound, "no quarantined messages")
	}
	out := make([]bodyMessageJSON, len(rows))
	for i, q := range rows {
		plain, err := a.mod.Cipher.Open(q.Message.MsgID, q.Message.ConvID, q.Nonce, q.Ciphertext, q.KID)
		if err != nil { // 不把 msgId 以外的东西写进日志：错误里没有明文
			a.log.Error("open quarantined message", "msgId", q.Message.MsgID, "err", fmt.Sprintf("%v", err))
			return nil, false, protocol.Errorf(protocol.CodeInternal, "internal error")
		}
		m := q.Message
		m.Body = plain
		out[i] = bodyMessageJSON{logic.ToProto(m), m.DeletedBy}
		out[i].Body = bodyOf(plain) // ToProto 对已删除的消息不带正文；这里是取证查看，要带上
	}
	return out, more, nil
}

func bodyOf(raw []byte) protocol.MsgBody {
	var b protocol.MsgBody
	_ = json.Unmarshal(raw, &b)
	return b
}
