package files

import (
	"context"
	"errors"
	"strings"
	"time"

	"tianting/server/internal/objstore"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// CompleteResponse 是 complete 的响应：服务端读到的真实值。
type CompleteResponse struct {
	FileID string `json:"fileId"`
	Size   int64  `json:"size"`
	Mime   string `json:"mime"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

func completeResponse(f store.File) *CompleteResponse {
	return &CompleteResponse{FileID: f.FileID, Size: f.Size, Mime: f.Mime, Width: f.Width, Height: f.Height}
}

func uploadMismatch() *protocol.Error {
	return protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonUploadMismatch, "upload mismatch")
}

func fileNotReady() *protocol.Error {
	return protocol.Errorf(protocol.CodeFileNotReady, "file not ready")
}

// variant 是一个变体的输入和这次尝试的结果。
type variant struct {
	name         string // original / thumb
	tmpKey       string
	declaredSize int64
	declaredMime string
	// 结果
	finalKey string
	size     int64
	mime     string
	w, h     int
}

// attempt 是一次 complete 尝试。
type attempt struct {
	id     string
	fileID string
	copied []string // 这次尝试已经写下的最终键：失败时只删这些（败者只删除自己的键）
}

// Complete 是 POST /v1/files/{fileId}/complete（F-FILE-01 规则 3、3b、6，F-FILE-05 规则 1）。幂等：已 ready 返回已有记录。
func (s *Service) Complete(ctx context.Context, uid, fileID string) (*CompleteResponse, *protocol.Error) {
	if !protocol.ValidID(fileID) {
		return nil, fileNotFound()
	}
	f, err := s.store.GetFile(ctx, fileID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && f.Owner != uid) {
		return nil, fileNotFound()
	}
	if err != nil {
		return nil, s.internal("get file", err)
	}
	switch f.State {
	case store.FileReady, store.FileAttached:
		return completeResponse(f), nil
	case store.FilePending:
	default: // 取消、清理中、过期：按不存在处理（RS5-16，AC-FILE-01-28）
		return nil, fileNotFound()
	}
	a := &attempt{id: randomHex(8), fileID: fileID}
	now := s.now()
	f, ok, err := s.store.AcquireCompleteLease(ctx, fileID, uid, a.id, now.Add(s.cfg.CompleteLease).UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, s.internal("acquire complete lease", err)
	}
	if !ok {
		return s.reread(ctx, f, protocol.WithReason(protocol.CodeObjectStoreUnavailable, protocol.ReasonCompleteInProgress, "complete in progress"))
	}
	resp, perr := s.runAttempt(ctx, f, a)
	if perr == errCrashErr {
		return nil, protocol.Errorf(protocol.CodeInternal, "internal error") // 模拟崩溃：不释放租约、不清理
	}
	// 每次尝试结束（包括失败）按 attempt 条件释放租约（RS5-07）。不用请求上下文。
	if err := s.store.ReleaseCompleteLease(context.Background(), fileID, a.id); err != nil {
		s.log.Error("release complete lease", "fileId", fileID, "err", err)
	}
	return resp, perr
}

// errCrashErr 是 errCrash 在协议错误层的标记。
var errCrashErr = &protocol.Error{Code: protocol.CodeInternal, Msg: "simulated crash"}

// reread 在拿不到租约、HEAD 404、提交失败时重读状态（RS5-07）：已 ready 就返回记录；还在 pending 返回 otherwise；其他按不存在。
func (s *Service) reread(ctx context.Context, f store.File, otherwise *protocol.Error) (*CompleteResponse, *protocol.Error) {
	cur, err := s.store.GetFile(ctx, f.FileID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, fileNotFound()
	case err != nil:
		return nil, s.internal("reread file", err)
	}
	switch cur.State {
	case store.FileReady, store.FileAttached:
		return completeResponse(cur), nil
	case store.FilePending:
		return nil, otherwise
	}
	return nil, fileNotFound()
}

func (s *Service) runAttempt(ctx context.Context, f store.File, a *attempt) (*CompleteResponse, *protocol.Error) {
	// Copy 前先查一次额度，超额的不做（RS5-06）。
	used, err := s.store.UsedUploadBytes(ctx, f.Owner, s.day())
	if err != nil {
		return nil, s.internal("used upload bytes", err)
	}
	daily, perr := s.dailyLimit(ctx, f.Owner)
	if perr != nil {
		return nil, perr
	}
	if used >= daily {
		return nil, protocol.Errorf(protocol.CodeQuotaExceeded, "daily upload quota exceeded")
	}
	vs := []*variant{{name: "original", tmpKey: f.TmpKey, declaredSize: f.DeclaredSize, declaredMime: f.DeclaredMime}}
	if f.Kind == store.KindImage {
		vs = append(vs, &variant{name: "thumb", tmpKey: f.TmpThumbKey, declaredSize: f.ThumbDeclaredSize, declaredMime: f.ThumbDeclaredMime})
	}
	for _, v := range vs {
		if perr := s.processVariant(ctx, f, a, v); perr != nil {
			if perr == errCrashErr {
				return nil, perr
			}
			s.cleanupAttempt(a)
			if perr.Code == protocol.CodeFileNotReady {
				// 临时对象不存在：可能另一个尝试已经提交并删除了临时键（AC-FILE-01-27），先重读。
				return s.reread(ctx, f, perr)
			}
			return nil, perr
		}
	}
	if err := s.hooks.at(PointAfterCopy, f.FileID); err != nil {
		return nil, errCrashErr
	}
	orig := vs[0]
	r := store.CompleteResult{Attempt: a.id, ObjectKey: orig.finalKey, Size: orig.size, Mime: orig.mime, Width: orig.w, Height: orig.h,
		Day: s.day(), DailyBytes: daily, Now: s.now().UnixMilli()}
	if len(vs) > 1 {
		r.ThumbKey, r.ThumbSize, r.ThumbMime = vs[1].finalKey, vs[1].size, vs[1].mime
	}
	err = s.store.CommitComplete(context.Background(), f.FileID, r)
	switch {
	case errors.Is(err, store.ErrUploadQuota):
		// 提交时复查额度（规则 6）：临时对象留给生命周期清除，只删这次尝试写的最终键。
		s.cleanupAttempt(a)
		return nil, protocol.Errorf(protocol.CodeQuotaExceeded, "daily upload quota exceeded")
	case errors.Is(err, store.ErrStateChanged):
		// 输掉竞争（另一个尝试先提交、被取消、被 file_purge 改成 purging）：只删自己的键，重读状态（RS4-23、RS5-07）。
		s.cleanupAttempt(a)
		return s.reread(ctx, f, fileNotFound())
	case err != nil:
		s.cleanupAttempt(a)
		s.log.Error("commit complete", "fileId", f.FileID, "err", err)
		return s.reread(ctx, f, protocol.Errorf(protocol.CodeInternal, "internal error"))
	}
	if s.metrics != nil {
		s.metrics.FileUploadedBytes.Add(float64(r.Size + r.ThumbSize))
	}
	// 提交之后：删临时键；按 final/{fileId}/ 前缀删除其他尝试（崩溃或输掉竞争留下的，RS5-06）。不用请求上下文。
	s.afterCommit(f, a)
	done, err := s.store.GetFile(ctx, f.FileID)
	if err != nil {
		return nil, s.internal("get file", err)
	}
	return completeResponse(done), nil
}

// processVariant：① HEAD 核对大小记 ETag → ② If-Match 的 Range 读 64 KB 嗅探、解析真实宽高 → ③ 条件 Copy 到本次尝试的键 →
// 再 HEAD 核对大小和 Copy 返回的 ETag。412 内部重试 1 次，仍失败 uploadMismatch，临时对象不删（规则 3）。
func (s *Service) processVariant(ctx context.Context, f store.File, a *attempt, v *variant) *protocol.Error {
	var perr *protocol.Error
	for try := 0; try < 2; try++ {
		var retry bool
		retry, perr = s.tryVariant(ctx, f, a, v)
		if !retry {
			return perr
		}
	}
	return uploadMismatch()
}

// tryVariant 返回 retry=true 表示 ETag 条件不满足（412），可以从 HEAD 重来一次。
func (s *Service) tryVariant(ctx context.Context, f store.File, a *attempt, v *variant) (bool, *protocol.Error) {
	if v.tmpKey == "" {
		return false, fileNotReady()
	}
	info, err := s.obj.Head(ctx, v.tmpKey)
	switch {
	case errors.Is(err, objstore.ErrNotFound):
		return false, fileNotReady() // 图片缺缩略图、临时对象已不存在（任何 kind）：4096，状态不变（RS4-23）
	case err != nil:
		return false, s.storeErr("head upload", f.FileID, err)
	}
	if info.Size != v.declaredSize {
		s.deleteKeys(f.FileID, v.tmpKey)
		return false, uploadMismatch()
	}
	head, err := s.obj.GetRange(ctx, v.tmpKey, sniffBytes, info.ETag)
	switch {
	case errors.Is(err, objstore.ErrPrecondition):
		return true, nil
	case errors.Is(err, objstore.ErrNotFound):
		return false, fileNotReady()
	case err != nil:
		return false, s.storeErr("read upload", f.FileID, err)
	}
	sn := sniff(head)
	mt, w, h, ok := s.judge(f.Kind, v.name, v.declaredMime, sn)
	if !ok {
		s.deleteKeys(f.FileID, v.tmpKey)
		return false, typeNotAllowed()
	}
	if err := s.hooks.at(PointAfterSniff, f.FileID); err != nil {
		return false, errCrashErr
	}
	final := "final/" + f.FileID + "/" + a.id + "/" + v.name
	etag, err := s.obj.Copy(ctx, v.tmpKey, s.obj.Bucket(), final, info.ETag, mt)
	switch {
	case errors.Is(err, objstore.ErrPrecondition):
		return true, nil
	case errors.Is(err, objstore.ErrNotFound):
		return false, fileNotReady()
	case err != nil:
		return false, s.storeErr("copy upload", f.FileID, err)
	}
	a.copied = append(a.copied, final)
	fi, err := s.obj.Head(ctx, final)
	if err != nil {
		return false, s.storeErr("head final", f.FileID, err)
	}
	if fi.Size != v.declaredSize || fi.ETag != etag {
		return false, uploadMismatch()
	}
	v.finalKey, v.size, v.mime, v.w, v.h = final, fi.Size, mt, w, h
	return false, nil
}

// judge 按种类和变体判断嗅探结果，返回服务端确定的 Content-Type 和真实宽高（F-FILE-01 规则 3 ②、3b，F-FILE-02 规则 1，F-FILE-05 规则 1）。
func (s *Service) judge(kind, variantName, declared string, sn sniffResult) (string, int, int, bool) {
	if kind == store.KindFile {
		// 文件：禁止的真实类型拒绝；认不出的按 application/octet-stream 放行（F-FILE-05 异常）。
		if sn.Forbidden {
			return "", 0, 0, false
		}
		return sn.Mime, 0, 0, true
	}
	allowed := imageMimes
	if kind == store.KindAvatar {
		allowed = avatarMimes
	}
	// 图片和头像是白名单：必须是认得出的图片、和声明一致、解析得到尺寸（RS4-21）。
	if !allowed[sn.Mime] || sn.Mime != declared || sn.Width < 1 || sn.Height < 1 {
		return "", 0, 0, false
	}
	if sn.Width > maxSide || sn.Height > maxSide || sn.Width*sn.Height > maxPixels || sn.Frames > gifMaxFrames {
		return "", 0, 0, false
	}
	long := max(sn.Width, sn.Height)
	if (variantName == "thumb" && long > thumbMaxSide) || (kind == store.KindAvatar && long > avatarMaxSide) {
		return "", 0, 0, false
	}
	return sn.Mime, sn.Width, sn.Height, true
}

// cleanupAttempt 删除这次尝试写下的最终键（失败或输掉竞争，只删自己的）。
func (s *Service) cleanupAttempt(a *attempt) {
	if len(a.copied) > 0 {
		s.deleteKeys(a.fileID, a.copied...)
	}
}

func (s *Service) afterCommit(f store.File, a *attempt) {
	s.deleteKeys(f.FileID, f.TmpKey, f.TmpThumbKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	keys, err := s.obj.List(ctx, "final/"+f.FileID+"/")
	if err != nil {
		s.log.Warn("list other attempts", "fileId", f.FileID, "err", err)
		return
	}
	mine := "final/" + f.FileID + "/" + a.id + "/"
	for _, k := range keys {
		if !strings.HasPrefix(k, mine) {
			if err := s.obj.Delete(ctx, s.obj.Bucket(), k); err != nil {
				s.log.Warn("delete other attempt", "fileId", f.FileID, "err", err)
			}
		}
	}
}
