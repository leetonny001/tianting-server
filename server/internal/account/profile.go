package account

import (
	"context"
	"errors"

	"tianting/server/internal/moderation"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// ProfileInput 是 PATCH /v1/account/profile（F-ACC-06 规则 1）。至少一项。
type ProfileInput struct {
	Nickname     *string `json:"nickname"`
	AvatarFileID *string `json:"avatarFileId"`
}

// AvatarBinder 绑定头像（files.Service）。
type AvatarBinder interface {
	BindAvatar(ctx context.Context, uid, fileID string) *protocol.Error
}

// UpdateProfile 改昵称（规则 2，规则同注册）和头像（规则 4，BE-437）。至少一项。
// 先改昵称再绑头像：对象存储不可用时昵称照常成功，头像 503 / 5032（F-ACC-06 异常，AC-ACC-06-9）。
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) *protocol.Error {
	if in.Nickname == nil && in.AvatarFileID == nil {
		return protocol.Errorf(protocol.CodeBadRequest, "nothing to update")
	}
	if in.Nickname != nil {
		nickname := protocol.TrimText(*in.Nickname)
		if nickname == "" || protocol.TextLen(nickname) > NicknameMaxLen {
			return protocol.Errorf(protocol.CodeBadRequest, "invalid nickname")
		}
		// 改昵称过敏感词（C6）：在格式之后、写入之前；命中时昵称不变（AC-MOD-02-5）。
		if s.Content != nil && s.Content.CheckContent(ctx, userID, moderation.SceneNickname, nickname) {
			return contentBlocked()
		}
		switch err := s.Store.UpdateNickname(ctx, userID, nickname); {
		case errors.Is(err, store.ErrNotFound):
			return protocol.Errorf(protocol.CodeUserNotFound, "user not found")
		case err != nil:
			s.Log.Error("update nickname", "err", err, "uid", userID)
			return unavailable()
		}
	}
	if in.AvatarFileID != nil {
		if s.Avatars == nil {
			return protocol.Errorf(protocol.CodeFileNotFound, "file not found")
		}
		return s.Avatars.BindAvatar(ctx, userID, *in.AvatarFileID)
	}
	return nil
}

// ContentChecker 是敏感词检查（C6，F-MOD-02 规则 2，由 logic.Service 实现）：命中返回 true，命中记录和指标由实现方负责。
// uid 为空表示还没有用户（注册），只计指标不写记录（backend-chat 第 113 条）。
type ContentChecker interface {
	CheckContent(ctx context.Context, uid, scene, text string) bool
}

// contentBlocked 是 451 / 4510，不带 details、不说命中了哪个词（F-MOD-02 规则 6）。
func contentBlocked() *protocol.Error {
	return protocol.Errorf(protocol.CodeContentBlocked, "content blocked")
}
