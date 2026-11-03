package admin

import (
	"errors"
	"net/http"

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
}

func (a *Admin) createGroup(w http.ResponseWriter, r *http.Request) {
	var req createGroupReq
	if !readJSON(w, r, &req) {
		return
	}
	if !protocol.ValidID(req.GroupID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid groupId"))
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
		a.inv.Invalidate(logic.TopicGroupMembers, req.GroupID)                      // 之前可能缓存过"群不存在"
		a.notifier.NotifyConvAdded(ctx, protocol.GroupConvID(req.GroupID), members) // 在线的初始成员收到 convAdded（C-35）
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
	removed, err := a.store.RemoveGroupMembers(r.Context(), gid, members)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case err != nil:
		a.internal(w, "remove members", err)
	default:
		if len(removed) > 0 {
			a.inv.Invalidate(logic.TopicGroupMembers, gid)
			a.notifier.NotifyConvRemoved(protocol.GroupConvID(gid), removed)
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
	if req.Name == nil && req.HistoryVisible == nil {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "nothing to update"))
		return
	}
	upd := store.GroupUpdate{HistoryVisible: req.HistoryVisible}
	if req.Name != nil {
		name, perr := groupName(*req.Name)
		if perr != nil {
			writeErr(w, perr)
			return
		}
		upd.Name = &name
	}
	// historyVisible 在查询时生效（每次 sync 现算 minSeq），不需要失效任何缓存。
	err := a.store.UpdateGroup(r.Context(), gid, upd)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeGroupNotFound, "group not found"))
	case err != nil:
		a.internal(w, "update group", err)
	default:
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
	writeJSON(w, http.StatusOK, groupResp{
		GroupID: g.GroupID, Name: g.Name, HistoryVisible: g.HistoryVisible, CreatedAt: g.CreatedAt, MemberCount: g.MemberCount, Members: nonNil(members),
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
