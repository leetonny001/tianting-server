package admin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	info := AuthFrom(r.Context())
	now := a.clock.Now()
	// grantId 是举报授权时走 report.evidence 路径（C6，backend-chat 第 109 条）；是隔离授权或者不带时走 msg.quarantine 路径（C4 不变）。
	evidence, perr := a.fetchViaReport(r, req.GrantID, info, conv, req.ConvID, req.MsgID, fileID, now)
	if perr != nil {
		writeErr(w, perr)
		return
	}
	if !evidence {
		if a.mod == nil {
			writeErr(w, protocol.Errorf(protocol.CodeForbidden, "caller lacks scope %s", adminauth.ScopeMsgQuarantine))
			return
		}
		if perr := a.grantScope(r, conv, true); perr != nil { // scope 和单聊开关，和读隔离原文相同
			writeErr(w, perr)
			return
		}
		ok, err := a.fetchGrantOK(r.Context(), req.GrantID, info, req.ConvID, now.UnixMilli())
		if err != nil {
			a.internal(w, "check msg grant", err)
			return
		}
		if !ok {
			writeErr(w, grantRequired())
			return
		}
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
	// 隔离路径只给被撤回或被后台删除的消息（它们的文件是 revoked），对象还在保留期（file_purge 删了之后 objects_deleted=1）。
	// 举报路径按 evidence_until 保留，原消息是否撤回、删除都不影响（证据引用已经在 fetchViaReport 里核对过）。
	if f.MsgID != req.MsgID || f.ConvID != req.ConvID || m.ConvID != req.ConvID || f.ObjectsDeleted || key == "" ||
		(!evidence && ((m.RecalledAt == 0 && m.DeletedAt == 0) || f.State != store.FileRevoked)) {
		writeErr(w, notFound)
		return
	}
	body, oi, err := a.objects().Get(r.Context(), key)
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

// objects 是对象存储：第二批和举报处理拿到的是同一个客户端。
func (a *Admin) objects() ObjectReader {
	if a.mod != nil && a.mod.Objects != nil {
		return a.mod.Objects
	}
	if a.reports != nil {
		return a.reports.Objects
	}
	return nil
}

// fetchViaReport 判断这次 fetch 是不是 report.evidence 路径：grantId 指向一个举报授权（msg_grants 里 conv_id 是 report:{id}）。
// 是的话还要：调用方有 report.evidence（否则 403 / 4030）、单聊 off 时 403 / 4030 c2cBodyDisabled（AC-MOD-04-13）、
// 授权属于这个调用方 + 操作人、没过期、这条举报的证据引用了 fileId、msgId、convId（否则 403 / 4039，AC-MOD-04-16）。
func (a *Admin) fetchViaReport(r *http.Request, grantID string, info AuthInfo, conv protocol.Conv, convID, msgID, fileID string, now time.Time) (bool, *protocol.Error) {
	c := a.callers[info.Caller]
	hasEvidence := a.reports != nil && c != nil && c.Has(adminauth.ScopeReportEvidence)
	if grantID == "" {
		if hasEvidence && (c == nil || !c.Has(adminauth.ScopeMsgQuarantine)) {
			return true, grantRequired() // report.evidence 路径 grantId 必填（第 109 条）
		}
		return false, nil
	}
	g, err := a.store.GetMsgGrant(r.Context(), grantID)
	switch {
	case errors.Is(err, store.ErrMsgGrantNotFound):
		if hasEvidence && (c == nil || !c.Has(adminauth.ScopeMsgQuarantine)) {
			return true, grantRequired()
		}
		return false, nil
	case err != nil:
		a.log.Error("get grant for fetch", "err", err)
		return true, protocol.Errorf(protocol.CodeInternal, "internal error")
	}
	if !strings.HasPrefix(g.ConvID, reportGrantPrefix) {
		return false, nil
	}
	if !hasEvidence {
		return true, protocol.Errorf(protocol.CodeForbidden, "caller lacks scope %s", adminauth.ScopeReportEvidence)
	}
	if conv.Type == protocol.ConvC2C && !a.reports.C2CBodyAccess {
		return true, c2cBodyDisabled()
	}
	ok, err := a.evidenceFileOK(r.Context(), g, info, convID, msgID, fileID, now)
	if err != nil {
		a.log.Error("check report grant for fetch", "err", err)
		return true, protocol.Errorf(protocol.CodeInternal, "internal error")
	}
	if !ok {
		return true, grantRequired()
	}
	return true, nil
}

// registerFetch 在对象存储开着、并且第二批管理接口（隔离授权）或举报处理（举报授权）至少一个可用时注册。
func (a *Admin) registerFetch(mux *http.ServeMux) {
	if a.objects() == nil {
		return
	}
	scopes := []adminauth.Scope{adminauth.ScopeReportEvidence}
	if a.mod != nil {
		scopes = []adminauth.Scope{adminauth.ScopeMsgQuarantine, adminauth.ScopeReportEvidence}
	}
	a.routeAny(mux, "POST", "/admin/files/{fileId}/fetch", scopes, a.fetchFile)
}
