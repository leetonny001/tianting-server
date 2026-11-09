package api

import (
	"context"
	"net/http"

	"tianting/server/internal/files"
	"tianting/server/internal/protocol"
)

// FileService 是 /v1/files/* 用到的文件服务（PRD-chat 第 11.2 节，F-FILE-01、04）。
type FileService interface {
	CreateUpload(ctx context.Context, uid string, in files.UploadInput) (*files.UploadResponse, *protocol.Error)
	Complete(ctx context.Context, uid, fileID string) (*files.CompleteResponse, *protocol.Error)
	CancelUpload(ctx context.Context, uid, fileID string) *protocol.Error
	URLs(ctx context.Context, uid string, in files.URLsInput) (*files.URLsResponse, *protocol.Error)
}

// RegisterFiles 注册 /v1/files/*。只在 IM_FILES_ENABLED 开启时调用：关闭时这些路径都是 404（第 4.1 节）。
// 鉴权是 Bearer（和 WS 的 auth 相同），不要求 sid：演示和管理接口签发的 token 也能发图片。
func (v *V1) RegisterFiles(mux *http.ServeMux, svc FileService) {
	v.Handle(mux, "POST /v1/files/uploads", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := v.Bearer(w, r)
		if !ok {
			return
		}
		var in files.UploadInput
		if v.ReadBody(w, r, &in) {
			writeResult(w, firstOf(svc.CreateUpload(r.Context(), claims.UserID, in)))
		}
	})
	v.Handle(mux, "POST /v1/files/{fileId}/complete", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := v.Bearer(w, r)
		if !ok {
			return
		}
		// 请求体可以为空（没有参数）；有的话也读完，受 64 KB 上限约束。
		var ignored struct{}
		if r.ContentLength != 0 && !v.ReadBody(w, r, &ignored) {
			return
		}
		writeResult(w, firstOf(svc.Complete(r.Context(), claims.UserID, r.PathValue("fileId"))))
	})
	v.Handle(mux, "DELETE /v1/files/uploads/{fileId}", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := v.Bearer(w, r)
		if !ok {
			return
		}
		if e := svc.CancelUpload(r.Context(), claims.UserID, r.PathValue("fileId")); e != nil {
			WriteError(w, e)
			return
		}
		WriteJSON(w, struct{}{})
	})
	v.Handle(mux, "POST /v1/files/urls", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := v.Bearer(w, r)
		if !ok {
			return
		}
		var in files.URLsInput
		if v.ReadBody(w, r, &in) {
			writeResult(w, firstOf(svc.URLs(r.Context(), claims.UserID, in)))
		}
	})
}
