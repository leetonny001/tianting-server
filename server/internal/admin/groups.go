package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"tianting/server/internal/adminauth"

	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// 群管理接口（PRD F-GRP-01）。
//
// 多个错误同时出现时的优先级（C-08、IMP-6）：
// ID 格式 4004 → 其他参数 4000 → 群不存在 4043（建群为已存在 4090）→ 批量超限 4131 → 用户不存在 4042 → 总人数超限 4130。
// 同一请求里重复的 userId 先去重，按 1 个计数（IMP-5）。
// 成员变动成功后，通知成员缓存失效（BE-033）。

type createGroupReq struct {
	GroupID        string   `json:"groupId"`
	Name           string   `json:"name"`
	Members        []string `json:"members"`
	HistoryVisible bool     `json:"historyVisible"`
	Owner          string   `json:"owner"` // 托管群交给用户（PRD-social F-SOC-30），需要 social.settings
}

// reservedGroupPrefix 保留给客户端建的群（F-SOC-21 规则 5）。
const reservedGroupPrefix = "ug-"

// hasScope 报告当前调用方是否有 scope（旧密钥只有 v1.2 能力，一律没有）。用于同一路由里按字段判 scope（勘误 E-18）。
func (a *Admin) hasScope(r *http.Request, scope adminauth.Scope) bool {
	c := a.callers[AuthFrom(r.Context()).Caller]
	return c != nil && c.Scopes[scope]
}

func (a *Admin) createGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.GroupID) || strings.HasPrefix(req.GroupID, reservedGroupPrefix) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	if req.Owner != "" && !a.hasScope(r, adminauth.ScopeSocialSettings) {
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "owner requires scope social.settings"))
		return
	}
	members, perr := normalizeMembers(req.Members, true)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	name, perr := groupName(req.Name)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	ctx := r.Context()
	if exists, err := a.store.GroupExists(ctx, req.GroupID); err != nil {
		a.internal(w, "check group", err)
		return
	} else if exists {
		writeErr(w, protocol.Errorf(protocol.CodeGroupExists, "group already exists"))
		return
	}
	if !a.checkUsersExist(w, r, members) {
		return
	}
	if req.Owner != "" && !slices.Contains(members, req.Owner) {
		writeErr(w, protocol.Errorf(protocol.CodeTargetNotGroupMember, "owner must be a member"))
		return
	}
	if len(members) > maxGroupMembers {
		writeErr(w, protocol.Errorf(protocol.CodeGroupMemberLimitExceeded, "a group can have at most %d members", maxGroupMembers))
		return
	}
	g := store.Group{GroupID: req.GroupID, Name: name, HistoryVisible: req.HistoryVisible, CreatedAt: a.clock.Now().UnixMilli()}
	err := a.store.CreateGroup(ctx, g, members)
	switch {
	case errors.Is(err, store.ErrExists): // 并发建同一个群
		writeErr(w, protocol.Errorf(protocol.CodeGroupExists, "group already exists"))
	case err != nil:
		a.internal(w, "create group", err)
	default:
		a.inv.Invalidate(logic.TopicGroupMembers, req.GroupID) // 之前可能缓存过"群不存在"
		var tip *store.Message
		if req.Owner != "" {
			var err error
			if tip, err = a.store.SetGroupOwner(ctx, req.GroupID, req.Owner, a.clock.Now().UnixMilli()); err != nil {
				a.internal(w, "set group owner", err)
				return
			}
		}
		a.notifier.NotifyConvAdded(ctx, protocol.GroupConvID(req.GroupID), members) // 在线的初始成员收到 convAdded（C-35）
		a.notifier.DeliverTip(ctx, tip)
		writeJSON(w, http.StatusCreated, struct{}{})
	}
}

type membersReq struct {
	Members []string `json:"members"`
}

func (a *Admin) addMembers(w http.ResponseWriter, r *http.Request) {
	gid, members, ok := a.membersRequest(w, r)
	if !ok {
		return
	}
	if !a.checkUsersExist(w, r, members) {
		return
	}
	added, err := a.store.AddGroupMembers(r.Context(), gid, members, maxGroupMembers, a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case errors.Is(err, store.ErrLimitExceeded):
		writeErr(w, protocol.Errorf(protocol.CodeGroupMemberLimitExceeded, "a group can have at most %d members", maxGroupMembers))
	case err != nil:
		a.internal(w, "add members", err)
	default:
		if len(added) > 0 {
			a.inv.Invalidate(logic.TopicGroupMembers, gid)
			a.notifier.NotifyConvAdded(r.Context(), protocol.GroupConvID(gid), added) // 只通知新加入的（C-36）
			a.notifier.NotifyGroupChanged(gid)
		}
		writeJSON(w, http.StatusOK, struct {
			Added []string `json:"added"`
		}{nonNil(added)})
	}
}

func (a *Admin) removeMembers(w http.ResponseWriter, r *http.Request) {
	gid, members, ok := a.membersRequest(w, r)
	if !ok {
		return
	}
	removed, tip, err := a.store.RemoveGroupMembers(r.Context(), gid, members, a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case err != nil:
		a.internal(w, "remove members", err)
	default:
		if len(removed) > 0 {
			a.inv.Invalidate(logic.TopicGroupMembers, gid)
			a.notifier.NotifyConvRemoved(protocol.GroupConvID(gid), removed, protocol.RemovedByAdmin)
			a.notifier.DeliverTip(r.Context(), tip) // 移除的是群主：自动转让 / 回到无群主（F-SOC-30 规则 4）
			a.notifier.NotifyGroupChanged(gid)
		}
		writeJSON(w, http.StatusOK, struct {
			Removed []string `json:"removed"`
		}{nonNil(removed)})
	}
}

// membersRequest 解析并校验加 / 移除成员的公共部分：ID 格式 → 非空 → 群存在 → 单次 500。
func (a *Admin) membersRequest(w http.ResponseWriter, r *http.Request) (gid string, members []string, ok bool) {
	var req membersReq
	if !readJSON(w, r, &req) {
		return "", nil, false
	}
	gid = r.PathValue("groupId")
	if !protocol.ValidID(gid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return "", nil, false
	}
	members, perr := normalizeMembers(req.Members, false)
	if perr != nil {
		writeErr(w, perr)
		return "", nil, false
	}
	if exists, err := a.store.GroupExists(r.Context(), gid); err != nil {
		a.internal(w, "check group", err)
		return "", nil, false
	} else if !exists {
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
		return "", nil, false
	}
	if len(members) > maxMemberBatch {
		writeErr(w, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d members per request", maxMemberBatch))
		return "", nil, false
	}
	return gid, members, true
}

type updateGroupReq struct {
	Name           *string `json:"name"`
	HistoryVisible *bool   `json:"historyVisible"`
	Notice         *string `json:"notice"`
	// Owner：字符串指定 / 更换群主，null 收回（F-SOC-30 规则 2）；需要 social.settings（勘误 E-18）。
	Owner json.RawMessage `json:"owner"`
}

func (a *Admin) updateGroup(w http.ResponseWriter, r *http.Request) {
	var req updateGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	gid := r.PathValue("groupId")
	if !protocol.ValidID(gid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	if req.Name == nil && req.HistoryVisible == nil && req.Notice == nil && req.Owner == nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "nothing to update"))
		return
	}
	var owner *string
	if req.Owner != nil {
		if !a.hasScope(r, adminauth.ScopeSocialSettings) {
			writeErr(w, protocol.Errorf(protocol.CodeForbidden, "owner requires scope social.settings"))
			return
		}
		v := ""
		if string(req.Owner) != "null" {
			if err := json.Unmarshal(req.Owner, &v); err != nil || !protocol.ValidID(v) {
				writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid owner"))
				return
			}
		}
		owner = &v
	}
	upd := store.GroupUpdate{HistoryVisible: req.HistoryVisible, Now: a.clock.Now().UnixMilli()}
	if req.Notice != nil {
		n := protocol.TrimText(*req.Notice)
		if protocol.TextLen(n) > protocol.MaxNoticeLen {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "notice longer than %d", protocol.MaxNoticeLen))
			return
		}
		upd.Notice = &n
	}
	if req.Name != nil {
		name, perr := groupName(*req.Name)
		if perr != nil {
			writeErr(w, perr)
			return
		}
		upd.Name = &name
	}
	// historyVisible 在查询时生效（每次 sync 现算 minSeq），不需要失效任何缓存。
	var err error
	if upd.Name != nil || upd.HistoryVisible != nil || upd.Notice != nil {
		err = a.store.UpdateGroup(r.Context(), gid, upd)
	} else if _, gerr := a.store.GetGroup(r.Context(), gid); gerr != nil {
		err = gerr
	}
	var tip *store.Message
	if err == nil && owner != nil {
		tip, err = a.store.SetGroupOwner(r.Context(), gid, *owner, upd.Now)
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case errors.Is(err, store.ErrTargetNotMember):
		writeErr(w, protocol.Errorf(protocol.CodeTargetNotGroupMember, "owner must be a member"))
	case err != nil:
		a.internal(w, "update group", err)
	default:
		a.notifier.DeliverTip(r.Context(), tip)
		a.notifier.NotifyGroupChanged(gid)
		writeJSON(w, http.StatusOK, struct{}{})
	}
}

type groupResp struct {
	GroupID        string   `json:"groupId"`
	Name           string   `json:"name"`
	HistoryVisible bool     `json:"historyVisible"`
	CreatedAt      int64    `json:"createdAt"`
	MemberCount    int      `json:"memberCount"`
	Members        []string `json:"members"`
	Owner          string   `json:"owner"`
	Admins         []string `json:"admins"`
	Notice         string   `json:"notice"`
	CreatedBy      string   `json:"createdBy"`
	Managed        bool     `json:"managed"`
	AllMuted       bool     `json:"allMuted"` // 全群禁言（F-ADM-42）。第二批开关关闭时禁言不生效，这里也是 false
}

func (a *Admin) getGroup(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("groupId")
	if !protocol.ValidID(gid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	g, err := a.store.GetGroup(r.Context(), gid)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
		return
	}
	if err != nil {
		a.internal(w, "get group", err)
		return
	}
	members, err := a.store.GroupMembers(r.Context(), gid)
	if err != nil {
		a.internal(w, "get group members", err)
		return
	}
	head, _, err := a.store.ListGroupMembers(r.Context(), gid, nil, store.MaxGroupAdmins+1) // 群主 → 管理员排在最前
	if err != nil {
		a.internal(w, "list group admins", err)
		return
	}
	admins := []string{}
	for _, m := range head {
		if m.Role == store.RoleAdmin {
			admins = append(admins, m.UserID)
		}
	}
	allMuted := false
	if a.mod != nil {
		// 直接查库，不走发言路径的缓存：后台刚开关完刷新详情，要看到真实状态。
		m, err := a.store.GroupAllMute(r.Context(), gid)
		if err != nil {
			a.internal(w, "get group mute", err)
			return
		}
		allMuted = m.Active(a.clock.Now().UnixMilli())
	}
	writeJSON(w, http.StatusOK, groupResp{
		GroupID: g.GroupID, Name: g.Name, HistoryVisible: g.HistoryVisible, CreatedAt: g.CreatedAt, MemberCount: g.MemberCount, Members: nonNil(members),
		Owner: g.OwnerID, Admins: admins, Notice: g.Notice, CreatedBy: g.CreatedBy, Managed: g.Managed(), AllMuted: allMuted,
	})
}

// checkUsersExist 成员中有不存在的用户时返回 404 / 4042（整批不生效）。
func (a *Admin) checkUsersExist(w http.ResponseWriter, r *http.Request, members []string) bool {
	missing, err := a.store.MissingUsers(r.Context(), members)
	if err != nil {
		a.internal(w, "check users", err)
		return false
	}
	if len(missing) > 0 {
		writeErr(w, protocol.Errorf(protocol.CodeUserNotFound, "user %s not found", missing[0]))
		return false
	}
	return true
}

// normalizeMembers 校验 ID 格式并去重（保持首次出现的顺序）。allowEmpty 为 false 时空列表返回 4000。
func normalizeMembers(members []string, allowEmpty bool) ([]string, *protocol.Error) {
	seen := make(map[string]bool, len(members))
	out := make([]string, 0, len(members))
	for _, m := range members {
		if !protocol.ValidID(m) {
			return nil, protocol.Errorf(protocol.CodeInvalidID, "invalid member id %q", m)
		}
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	if len(out) == 0 && !allowEmpty {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "members must not be empty")
	}
	return out, nil
}

// groupName 去首尾空白后按码点计数，1～50（C-38）。
func groupName(raw string) (string, *protocol.Error) {
	name := protocol.TrimText(raw)
	if n := protocol.TextLen(name); n < 1 || n > maxGroupNameLen {
		return "", protocol.Errorf(protocol.CodeBadRequest, "group name must be 1-%d characters", maxGroupNameLen)
	}
	return name, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// dissolveGroup 解散群（PRD-social F-SOC-26、PRD-admin G-ADM-11，BE-122）：删除全部成员、写 dissolved_at，消息保留；
// 群 ID 不能复用（再次建群 409 / 4090）。全部在线成员收到 convRemoved {reason: dissolved}。
func (a *Admin) dissolveGroup(w http.ResponseWriter, r *http.Request) {
	gid := r.PathValue("groupId")
	if !protocol.ValidID(gid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
		return
	}
	members, err := a.store.DissolveGroup(r.Context(), gid, a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case err != nil:
		a.internal(w, "dissolve group", err)
	default:
		a.inv.Invalidate(logic.TopicGroupMembers, gid)
		a.notifier.NotifyConvRemoved(protocol.GroupConvID(gid), members, protocol.RemovedDissolved)
		writeJSON(w, http.StatusOK, struct{}{})
	}
}
