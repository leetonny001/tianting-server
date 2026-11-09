package logic

import (
	"context"
	"errors"
	"time"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// filesConfig 是 C2 在发送路径上的配置（PRD-chat 第 4.1 节）。
type filesConfig struct {
	enabled       bool
	retentionDays int // 0 表示不过期
}

// EnableFiles 打开 image、file 消息（IM_FILES_ENABLED）。关闭时 send 的 image、file 返回 4000（F-FILE 的写法，不是共同规则的 4030），
// 已有文件消息照常 sync（AC-FILE-01-14）。
func (s *Service) EnableFiles(retentionDays int) {
	s.files = filesConfig{enabled: true, retentionDays: retentionDays}
}

// fileKindFor 是消息类型要求的文件种类。
func fileKindFor(msgType string) string {
	if msgType == protocol.MsgImage {
		return store.KindImage
	}
	return store.KindFile
}

// precheckFile 是第 4.3 节第 6 步的附件预检：存在且属于发送者（否则 4048）、类型相符（4000 fileKindMismatch）、ready（否则 4096）。
// 真正的绑定在 SaveMessage 事务里（第 10 步），预检通过之后状态仍可能变化，那时同样 4096。
func (s *Service) precheckFile(ctx context.Context, uid string, req protocol.SendReq) *protocol.Error {
	f, err := s.store.GetFile(ctx, req.Body.FileID())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return protocol.Errorf(protocol.CodeFileNotFound, "file not found")
	case err != nil:
		return s.internal("get file", err)
	}
	if f.Owner != uid {
		return protocol.Errorf(protocol.CodeFileNotFound, "file not found")
	}
	if f.Kind != fileKindFor(req.Type) {
		return protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonFileKindMismatch, "file kind mismatch")
	}
	if f.State != store.FileReady {
		return protocol.Errorf(protocol.CodeFileNotReady, "file not ready")
	}
	return nil
}

// storedFileBody 是保存的正文：只有 fileId。文件名、大小、宽高在下发时按文件记录填写（F-FILE-02 规则 1），
// 这样过期时清空 files.name 就真正清除了文件名（F-FILE-06 规则 3，R-CS-23）。
func storedFileBody(req protocol.SendReq) protocol.MsgBody {
	id := req.Body.FileID()
	if req.Type == protocol.MsgImage {
		return protocol.MsgBody{Image: &protocol.ImageBody{FileID: id}}
	}
	return protocol.MsgBody{File: &protocol.FileBody{FileID: id}}
}

// retainUntil 是绑定时写入的保留期到期时间。
func (s *Service) retainUntil(now time.Time) int64 {
	if s.files.retentionDays <= 0 {
		return 0
	}
	return now.Add(time.Duration(s.files.retentionDays) * 24 * time.Hour).UnixMilli()
}

// EnrichFiles 用服务端记录填写 image、file 消息的 size、mime、宽高、文件名（F-FILE-02 规则 1，F-FILE-03 规则 1）。
// 文件已过期、被撤回或已清除时 expired=true，文件名为空（F-FILE-06 规则 3）。被删除的消息没有 body，不处理。
func EnrichFiles(ctx context.Context, st store.FileStore, msgs []protocol.Message) error {
	var ids []string
	for _, m := range msgs {
		if id := m.Body.FileID(); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	recs, err := st.GetFiles(ctx, ids)
	if err != nil {
		return err
	}
	for i := range msgs {
		b := &msgs[i].Body
		id := b.FileID()
		if id == "" {
			continue
		}
		f, ok := recs[id]
		expired := !ok || (f.State != store.FileAttached)
		switch {
		case b.Image != nil:
			img := protocol.ImageBody{FileID: id, Expired: expired}
			if ok {
				img.Width, img.Height, img.Size, img.Mime = f.Width, f.Height, f.Size, f.Mime
			}
			b.Image = &img
		case b.File != nil:
			fb := protocol.FileBody{FileID: id, Expired: expired}
			if ok {
				fb.Size, fb.Mime = f.Size, f.Mime
				if !expired {
					fb.Name = f.Name
				}
			}
			b.File = &fb
		}
	}
	return nil
}
