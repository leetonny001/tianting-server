package account

import (
	"context"
	"errors"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// ProfileInput 是 PATCH /v1/account/profile（F-ACC-06 规则 1）。至少一项。
type ProfileInput struct {
	Nickname     *string `json:"nickname"`
	AvatarFileID *string `json:"avatarFileId"`
}

// UpdateProfile 改昵称（规则 2，规则同注册）。头像随 BE-437：C1 时带 avatarFileId 返回 404 / 4048。
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileInput) *protocol.Error {
	if in.AvatarFileID != nil {
		return protocol.Errorf(protocol.CodeFileNotFound, "file not found")
	}
	if in.Nickname == nil {
		return protocol.Errorf(protocol.CodeBadRequest, "nothing to update")
	}
	nickname := protocol.TrimText(*in.Nickname)
	if nickname == "" || protocol.TextLen(nickname) > NicknameMaxLen {
		return protocol.Errorf(protocol.CodeBadRequest, "invalid nickname")
	}
	switch err := s.Store.UpdateNickname(ctx, userID, nickname); {
	case errors.Is(err, store.ErrNotFound):
		return protocol.Errorf(protocol.CodeUserNotFound, "user not found")
	case err != nil:
		s.Log.Error("update nickname", "err", err, "uid", userID)
		return unavailable()
	}
	return nil
}
