package files

import (
	"context"
	"errors"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// BindAvatar 是 PATCH /v1/account/profile 的头像部分（F-ACC-06 规则 4，BE-437）：
// 只接受本人、kind=avatar、已 complete 的文件（否则 404 / 4048）；公开键由服务端生成，**先写 files.public_key 再 Copy**，
// 重试复用同一个键（RS6-04）；Copy 到公开读桶后在一个事务里改 attached、写 users.avatar、给原来的头像记 avatar_replaced_at。
// 相同 avatarFileId 重复绑定按成功处理。对象存储不可用：503 / 5032（昵称照常修改，由调用方保证）。
// 绑定失败不删除公开对象（可能和并发重试撞键），由 file_purge 按 public_key 清除。
func (s *Service) BindAvatar(ctx context.Context, uid, fileID string) *protocol.Error {
	if !s.cfg.Enabled || !protocol.ValidID(fileID) {
		return fileNotFound()
	}
	f, err := s.store.GetFile(ctx, fileID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fileNotFound()
	case err != nil:
		return s.internal("get file", err)
	}
	if f.Owner != uid || f.Kind != store.KindAvatar || (f.State != store.FileReady && f.State != store.FileAttached) {
		return fileNotFound()
	}
	if f.State == store.FileAttached && f.PublicKey != "" && f.AvatarReplacedAt == 0 {
		if u, err := s.store.GetUser(ctx, uid); err == nil && u.Avatar == s.obj.PublicURL(f.PublicKey) {
			return nil // 响应丢失后的重试（AC-ACC-06-11）
		}
	}
	if f.PublicKey == "" {
		// 地址含随机 ID，和 fileId、userId 无关。
		if f, err = s.store.SetAvatarPublicKey(ctx, fileID, uid, "avatars/"+randomHex(16)); err != nil {
			return s.internal("set avatar public key", err)
		}
		if f.PublicKey == "" {
			return fileNotFound() // 期间被清理
		}
	}
	if _, err := s.obj.Copy(ctx, f.ObjectKey, s.obj.PublicBucket(), f.PublicKey, "", f.Mime); err != nil {
		return s.storeErr("copy avatar to the public bucket", fileID, err)
	}
	if err := s.hooks.at(PointAfterPublicCopy, fileID); err != nil {
		return protocol.Errorf(protocol.CodeInternal, "internal error") // 模拟崩溃
	}
	switch err := s.store.BindAvatar(ctx, fileID, uid, s.obj.PublicURL(f.PublicKey), s.now().UnixMilli()); {
	case errors.Is(err, store.ErrFileNotReady):
		return fileNotFound()
	case errors.Is(err, store.ErrNotFound):
		return protocol.Errorf(protocol.CodeUserNotFound, "user not found")
	case err != nil:
		return s.internal("bind avatar", err)
	}
	return nil
}
