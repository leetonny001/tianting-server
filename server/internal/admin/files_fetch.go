package admin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/objstore"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// ObjectReader 读对象存储私有桶里的对象（由 objstore.Client 实现）。
type ObjectReader interface {
	Get(ctx context.Context, key string) (io.ReadCloser, objstore.Info, error)
}

// fetchFile 是 POST /admin/files/{fileId}/fetch（PRD-chat v1.6 第 11.6 节，F-RCL-04，AC-RCL-04-9；形状在 BE-470 第 4 条定，
// v1.6 加可选 grantId，PENDING P-44 ④）：
// 审核员看被撤回或被后台删除的消息里的图片、文件，console-server 代理（CS-202）。im-server 从对象存储读出来返回，不发预签名地址。
// 要 msg.quarantine（C4；report.evidence 随 C6）且这个会话有有效的隔离授权（同一调用方、操作人），没有 403 / 4039。
// 带 grantId 时只认这一个授权：要属于这个调用方 + 签名的操作人 + convId、是隔离授权、没过期，任何一条不符合（含不存在）都是 403 / 4039，
// 不再按会话找别的授权；不带时按调用方 + 操作人 + 会话查（兼容）。
// 单聊受 IM_C2C_BODY_ACCESS（403 / 4030 c2cBodyDisabled）。fileId 不属于 msgId、消息没被撤回或删除、对象过了保留期：404 / 4048，不区分。
// 审计只记 fileId、msgId、variant，不记文件名。
func (a *Admin) fetchFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var req struct {
		GrantID string `json:"grantId"`
		ConvID  string `json:"convId"`
		MsgID   string `json:"msgId"`
		Variant string `json:"variant"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	fileID := r.PathValue("fileId")
	conv, perr := protocol.ParseConvID(req.ConvID)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	if conv.Type != protocol.ConvC2C && conv.Type != protocol.ConvGroup {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "only c2c and group conversations"))
		return
	}
	if !protocol.ValidMsgID(req.MsgID) || !protocol.ValidMsgID(fileID) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid msgId or fileId"))
		return
	}
	variant := req.Variant
	if variant == "" {
		variant = "original"
	}
	if variant != "original" && variant != "thumb" {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "variant must be original or thumb"))
		return
	}
	noteAuditFields(r.Context(), map[string]any{"fileId": fileID, "msgId": req.MsgID, "variant": variant})
	if perr := a.grantScope(r, conv, true); perr != nil { // scope 和单聊开关，和读隔离原文相同
		writeErr(w, perr)
		return
	}
	info := AuthFrom(r.Context())
	now := a.clock.Now().UnixMilli()
	ok, err := a.fetchGrantOK(r.Context(), req.GrantID, info, req.ConvID, now)
	if err != nil {
		a.internal(w, "check msg grant", err)
		return
	}
	if !ok {
		writeErr(w, grantRequired())
		return
	}
	notFound := protocol.Errorf(protocol.CodeFileNotFound, "file not found")
	f, err := a.store.GetFile(r.Context(), fileID)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, notFound)
		return
	}
	if err != nil {
		a.internal(w, "get file", err)
		return
	}
	m, err := a.store.GetMessage(r.Context(), req.MsgID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, notFound)
		return
	case err != nil:
		a.internal(w, "get message", err)
		return
	}
	key, mime := f.ObjectKey, f.Mime
	if variant == "thumb" {
		key, mime = f.ThumbKey, f.ThumbMime
	}
	// 只给被撤回或被后台删除的消息（它们的文件是 revoked），对象还在保留期（file_purge 删了之后 objects_deleted=1）。
	if f.MsgID != req.MsgID || f.ConvID != req.ConvID || m.ConvID != req.ConvID || (m.RecalledAt == 0 && m.DeletedAt == 0) ||
		f.State != store.FileRevoked || f.ObjectsDeleted || key == "" {
		writeErr(w, notFound)
		return
	}
	body, oi, err := a.mod.Objects.Get(r.Context(), key)
	switch {
	case errors.Is(err, objstore.ErrNotFound):
		writeErr(w, notFound)
		return
	case err != nil:
		a.log.Warn("fetch quarantined file", "fileId", fileID, "err", err) // 不记文件名
		writeErr(w, protocol.Errorf(protocol.CodeServiceUnavailable, "object store unavailable"))
		return
	}
	defer body.Close()
	h := w.Header()
	h.Set("Content-Type", mime)
	h.Set("Content-Disposition", "attachment") // 不带文件名：文件名按正文对待（R-CS-14），console 自己决定
	h.Set("X-Content-Type-Options", "nosniff")
	if oi.Size > 0 {
		h.Set("Content-Length", strconv.FormatInt(oi.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// fetchGrantOK 判断 fetch 的授权（PRD-chat v1.6 第 11.6 节）。grantID 非空时只看这一个授权，和正文读取的条件相同再加上必须是隔离授权；
// 为空时按调用方 + 操作人 + 会话找有效的隔离授权。
func (a *Admin) fetchGrantOK(ctx context.Context, grantID string, info AuthInfo, convID string, now int64) (bool, error) {
	if grantID == "" {
		return a.store.HasActiveMsgGrant(ctx, info.Caller, info.Operator, convID, true, now)
	}
	g, err := a.store.GetMsgGrant(ctx, grantID)
	if errors.Is(err, store.ErrMsgGrantNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return g.Caller == info.Caller && g.Operator == info.Operator && g.ConvID == convID && g.Quarantine && g.ExpiresAt > now, nil
}

// registerFetch 在第二批管理接口和对象存储都开着时注册（隔离授权只有第二批才有）。
func (a *Admin) registerFetch(mux *http.ServeMux) {
	if a.mod == nil || a.mod.Objects == nil {
		return
	}
	a.routeAny(mux, "POST", "/admin/files/{fileId}/fetch", []adminauth.Scope{adminauth.ScopeMsgQuarantine}, a.fetchFile)
}
