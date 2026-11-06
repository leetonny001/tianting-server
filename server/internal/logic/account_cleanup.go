package logic

import (
	"context"

	"tianting/server/internal/protocol"
)

// deletedNickname 是注销清理后的昵称（PRD-chat F-ACC-08 规则 3）。userId 留在消息里，界面显示这个昵称。
const deletedNickname = "已注销用户"

const cleanupGroupPage = 100

// CleanupDeletedUser 是 account_cleanup 的第 ② 步（规则 3）：删好友、黑名单，昵称改掉，逐群退出（群主转让按 F-SOC-23 规则 4，托管群回到无群主）。
// 最终一致，可以重试：每个群一个事务，已经退出的不会再处理，所以不会有重复的提示消息。头像对象随 BE-437。
func (s *Service) CleanupDeletedUser(ctx context.Context, userID string, check func() error) error {
	bumped, err := s.store.RemoveUserSocial(ctx, userID, deletedNickname, s.nowMs())
	if err != nil {
		return err
	}
	s.NotifySocialChanged(bumped, "", "")
	for {
		groups, _, err := s.store.ListUserGroups(ctx, userID, "", cleanupGroupPage)
		if err != nil {
			return err
		}
		if len(groups) == 0 {
			return nil
		}
		for _, g := range groups {
			if err := check(); err != nil {
				return err
			}
			res, err := s.store.LeaveGroup(ctx, g.GroupID, userID, s.nowMs())
			if err != nil {
				return err
			}
			s.invalidateGroup(g.GroupID)
			s.pushEvent(userID, protocol.ConvRemovedEvent{Type: protocol.EventConvRemoved, ConvID: protocol.GroupConvID(g.GroupID), Reason: protocol.RemovedLeft}, "")
			if !res.Dissolved {
				s.deliverTip(ctx, res.Tip, "")
				s.NotifyGroupChanged(g.GroupID)
			}
		}
	}
}
