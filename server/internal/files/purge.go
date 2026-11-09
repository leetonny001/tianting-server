package files

import (
	"context"
	"errors"
	"time"

	"tianting/server/internal/store"
)

// Purge 是定时任务 file_purge（F-FILE-06 规则 2、3、4、7，F-ACC-06 规则 4、5）：每小时一批，只在一个节点运行（和 quarantine_purge 相同）。
//  1. 超过 24 小时未绑定的 pending（租约过期或没有租约）、ready → purging；换头像超过 7 天的头像 → purging；
//  2. 保留期到了的 attached → expired（清空文件名、递增会话 mod_ver），再按 final/{fileId}/ 前缀删除对象；隔离期过了的 revoked 同样删除对象；
//  3. purging：public_key 不空先删公开对象，成功后清空；再按 final/{fileId}/ 前缀删除，尽力删临时键；最后删行。
//
// 对象存储删除失败：记录保留，返回错误（指标 im_job_runs_total{job="file_purge",result="error"}），下次重试，不影响消息。
// 清理不用请求上下文。返回处理的文件数。
func (s *Service) Purge(ctx context.Context) (int, error) {
	now := s.now()
	batch := s.cfg.PurgeBatch
	n, err := s.store.MarkStalePurging(ctx, now.Add(-time.Duration(s.cfg.OrphanHours)*time.Hour).UnixMilli(), now.UnixMilli(), batch)
	if err != nil {
		return 0, err
	}
	m, err := s.store.MarkReplacedAvatarsPurging(ctx, now.Add(-time.Duration(s.cfg.AvatarReplacedDays)*24*time.Hour).UnixMilli(), batch)
	if err != nil {
		return n, err
	}
	total := n + m
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	expired, err := s.store.ExpireFiles(ctx, now.UnixMilli(), batch)
	if err != nil {
		return total, err
	}
	total += len(expired)
	toDelete, err := s.store.ListObjectsToDelete(ctx, now.UnixMilli(), batch)
	if err != nil {
		return total, err
	}
	for _, f := range toDelete {
		if _, err := s.obj.DeletePrefix(ctx, "final/"+f.FileID+"/"); err != nil {
			s.log.Warn("file_purge: delete objects", "fileId", f.FileID, "err", err)
			note(err)
			continue
		}
		note(s.store.MarkObjectsDeleted(ctx, f.FileID))
		total++
	}
	purging, err := s.store.ListPurging(ctx, batch)
	if err != nil {
		return total, err
	}
	for _, f := range purging {
		if err := s.purgeOne(ctx, f); err != nil {
			s.log.Warn("file_purge: purge", "fileId", f.FileID, "err", err)
			note(err)
			continue
		}
		total++
	}
	return total, firstErr
}

func (s *Service) purgeOne(ctx context.Context, f store.File) error {
	// 任何路径进入 purging 的文件，public_key 不为空就先删公开对象，成功后才能删行（RS7-03）。
	if f.PublicKey != "" {
		if err := s.obj.Delete(ctx, s.obj.PublicBucket(), f.PublicKey); err != nil {
			return err
		}
		if err := s.store.ClearPublicKey(ctx, f.FileID); err != nil {
			return err
		}
	}
	// 物理删除按 final/{fileId}/ 前缀，不只删记录里的键（RS5-06）：崩溃留下的其他尝试也一起删掉。
	if _, err := s.obj.DeletePrefix(ctx, "final/"+f.FileID+"/"); err != nil {
		return err
	}
	// 临时键尽力删除，失败靠生命周期规则（F-FILE-01 规则 3a），不阻止删行。
	for _, k := range []string{f.TmpKey, f.TmpThumbKey} {
		if k != "" {
			if err := s.obj.Delete(ctx, s.obj.Bucket(), k); err != nil {
				s.log.Warn("file_purge: delete tmp object", "fileId", f.FileID, "err", err)
			}
		}
	}
	err := s.store.DeleteFileRow(ctx, f.FileID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}
