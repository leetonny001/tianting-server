package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const fileColumns = `file_id, owner, kind, state, name, declared_size, declared_mime, thumb_declared_size, thumb_declared_mime,
	tmp_key, tmp_thumb_key, object_key, thumb_key, size, mime, thumb_size, thumb_mime, width, height,
	COALESCE(completing_until, 0), completing_attempt, public_key, avatar_replaced_at, msg_id, conv_id, retain_until, evidence_until,
	scan_state, put_expires_at, created_at, ready_at, attached_at, revoked_at, objects_deleted`

func scanFile(r rowScanner) (File, error) {
	var f File
	err := r.Scan(&f.FileID, &f.Owner, &f.Kind, &f.State, &f.Name, &f.DeclaredSize, &f.DeclaredMime, &f.ThumbDeclaredSize, &f.ThumbDeclaredMime,
		&f.TmpKey, &f.TmpThumbKey, &f.ObjectKey, &f.ThumbKey, &f.Size, &f.Mime, &f.ThumbSize, &f.ThumbMime, &f.Width, &f.Height,
		&f.CompletingUntil, &f.CompletingAttempt, &f.PublicKey, &f.AvatarReplacedAt, &f.MsgID, &f.ConvID, &f.RetainUntil, &f.EvidenceUntil,
		&f.ScanState, &f.PutExpiresAt, &f.CreatedAt, &f.ReadyAt, &f.AttachedAt, &f.RevokedAt, &f.ObjectsDeleted)
	return f, err
}

func (s *SQLite) CreateUpload(ctx context.Context, f File, lim UploadLimits) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var used, declared int64
	err = tx.QueryRowContext(ctx, `SELECT bytes, declared FROM upload_quota WHERE user_id = ? AND day = ?`, f.Owner, lim.Day).Scan(&used, &declared)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	size := f.DeclaredSize + f.ThumbDeclaredSize
	// 规则 6：已用 + 声明超过额度；规则 6a：声明合计超过上限。
	if used+size > lim.DailyBytes {
		return ErrUploadQuota
	}
	if declared+size > lim.DailyDeclared {
		return ErrUploadDeclared
	}
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE owner = ? AND state = 'pending' AND put_expires_at > ?`, f.Owner, lim.NowReal).Scan(&pending); err != nil {
		return err
	}
	if pending >= lim.PendingMax {
		return ErrUploadPendingLimit
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_quota (user_id, day, declared) VALUES (?, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET declared = upload_quota.declared + excluded.declared`, f.Owner, lim.Day, size); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO files (file_id, owner, kind, state, name, declared_size, declared_mime, thumb_declared_size, thumb_declared_mime,
		tmp_key, tmp_thumb_key, put_expires_at, created_at) VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.FileID, f.Owner, f.Kind, f.Name, f.DeclaredSize, f.DeclaredMime, f.ThumbDeclaredSize, f.ThumbDeclaredMime,
		f.TmpKey, f.TmpThumbKey, f.PutExpiresAt, f.CreatedAt); err != nil {
		return fmt.Errorf("insert file: %w", err)
	}
	return tx.Commit()
}

func (s *SQLite) GetFile(ctx context.Context, fileID string) (File, error) {
	f, err := scanFile(s.db.QueryRowContext(ctx, `SELECT `+fileColumns+` FROM files WHERE file_id = ?`, fileID))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return f, err
}

func (s *SQLite) GetFiles(ctx context.Context, fileIDs []string) (map[string]File, error) {
	out := make(map[string]File, len(fileIDs))
	for start := 0; start < len(fileIDs); start += 500 {
		chunk := fileIDs[start:min(len(fileIDs), start+500)]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, `SELECT `+fileColumns+` FROM files WHERE file_id IN (`+placeholders(len(chunk))+`)`, args...) // #nosec G202 -- placeholders only
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			f, err := scanFile(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out[f.FileID] = f
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *SQLite) FileAccessInfo(ctx context.Context, fileIDs []string) (map[string]FileAccess, error) {
	files, err := s.GetFiles(ctx, fileIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]FileAccess, len(files))
	for id, f := range files {
		a := FileAccess{File: f}
		if f.MsgID != "" {
			// 撤回的消息和删除的一样不可访问（C4，AC-RCL-04-6）；撤回时文件已经是 revoked，这里再挡一次。
			err := s.db.QueryRowContext(ctx, `SELECT seq, deleted_at > 0 OR recalled_at > 0 FROM messages WHERE msg_id = ?`, f.MsgID).Scan(&a.MsgSeq, &a.MsgDeleted)
			switch {
			case err == nil:
				a.MsgFound = true
			case !errors.Is(err, sql.ErrNoRows):
				return nil, err
			}
		}
		out[id] = a
	}
	return out, nil
}

func (s *SQLite) AcquireCompleteLease(ctx context.Context, fileID, owner, attempt string, until, now int64) (File, bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE files SET completing_until = ?, completing_attempt = ?
		WHERE file_id = ? AND owner = ? AND state = 'pending' AND (completing_until IS NULL OR completing_until < ?)`,
		until, attempt, fileID, owner, now)
	if err != nil {
		return File{}, false, err
	}
	n, _ := res.RowsAffected()
	f, err := s.GetFile(ctx, fileID)
	if err != nil {
		return File{}, false, err
	}
	return f, n == 1, nil
}

func (s *SQLite) ReleaseCompleteLease(ctx context.Context, fileID, attempt string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE files SET completing_until = NULL WHERE file_id = ? AND completing_attempt = ?`, fileID, attempt)
	return err
}

func (s *SQLite) CommitComplete(ctx context.Context, fileID string, r CompleteResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner, state string
	if err := tx.QueryRowContext(ctx, `SELECT owner, state FROM files WHERE file_id = ?`, fileID).Scan(&owner, &state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrStateChanged
		}
		return err
	}
	if state != FilePending {
		return ErrStateChanged
	}
	// 规则 6：提交时在同一事务复查，已用量已达额度就拒绝，所以当日最多超额一个文件（RS4-19）。
	var used int64
	if err := tx.QueryRowContext(ctx, `SELECT bytes FROM upload_quota WHERE user_id = ? AND day = ?`, owner, r.Day).Scan(&used); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if used >= r.DailyBytes {
		return ErrUploadQuota
	}
	res, err := tx.ExecContext(ctx, `UPDATE files SET state = 'ready', object_key = ?, thumb_key = ?, size = ?, mime = ?, thumb_size = ?, thumb_mime = ?,
		width = ?, height = ?, ready_at = ?, completing_until = NULL WHERE file_id = ? AND state = 'pending'`,
		r.ObjectKey, r.ThumbKey, r.Size, r.Mime, r.ThumbSize, r.ThumbMime, r.Width, r.Height, r.Now, fileID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrStateChanged
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO upload_quota (user_id, day, bytes) VALUES (?, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET bytes = upload_quota.bytes + excluded.bytes`, owner, r.Day, r.Size+r.ThumbSize); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) UsedUploadBytes(ctx context.Context, userID, day string) (int64, error) {
	var used int64
	err := s.db.QueryRowContext(ctx, `SELECT bytes FROM upload_quota WHERE user_id = ? AND day = ?`, userID, day).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return used, err
}

func (s *SQLite) CancelUpload(ctx context.Context, fileID, owner string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE files SET state = 'purging' WHERE file_id = ? AND owner = ? AND state = 'pending'`, fileID, owner)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *SQLite) MarkStalePurging(ctx context.Context, before, now int64, limit int) (int, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE files SET state = 'purging' WHERE file_id IN (
		SELECT file_id FROM files WHERE created_at < ? AND (state = 'ready' OR (state = 'pending' AND (completing_until IS NULL OR completing_until < ?)))
		LIMIT ?)`, before, now, limit)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *SQLite) MarkReplacedAvatarsPurging(ctx context.Context, before int64, limit int) (int, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE files SET state = 'purging' WHERE file_id IN (
		SELECT file_id FROM files WHERE kind = 'avatar' AND state = 'attached' AND avatar_replaced_at > 0 AND avatar_replaced_at < ? LIMIT ?)`, before, limit)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *SQLite) listFiles(ctx context.Context, where string, args ...any) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+fileColumns+` FROM files WHERE `+where, args...) // #nosec G202 -- constant fragments only
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *SQLite) ListPurging(ctx context.Context, limit int) ([]File, error) {
	return s.listFiles(ctx, `state = 'purging' ORDER BY created_at LIMIT ?`, limit)
}

func (s *SQLite) DeleteFileRow(ctx context.Context, fileID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE file_id = ? AND state = 'purging'`, fileID)
	return err
}

func (s *SQLite) ClearPublicKey(ctx context.Context, fileID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE files SET public_key = '' WHERE file_id = ?`, fileID)
	return err
}

func (s *SQLite) ExpireFiles(ctx context.Context, now int64, limit int) ([]File, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+fileColumns+` FROM files WHERE state = 'attached' AND kind <> 'avatar' AND retain_until > 0 AND retain_until < ? LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	var out []File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 加锁顺序 conv_seq → 文件行（第 4.3 节）：先递增 mod_ver，再改文件。
	for _, f := range out {
		if f.ConvID != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE conv_seq SET mod_ver = mod_ver + 1 WHERE conv_id = ?`, f.ConvID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE files SET state = 'expired', name = '' WHERE file_id = ? AND state = 'attached'`, f.FileID); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit()
}

func (s *SQLite) ListObjectsToDelete(ctx context.Context, now int64, limit int) ([]File, error) {
	return s.listFiles(ctx, `objects_deleted = 0 AND (state = 'expired' OR (state = 'revoked' AND retain_until < ?)) LIMIT ?`, now, limit)
}

func (s *SQLite) MarkObjectsDeleted(ctx context.Context, fileID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE files SET objects_deleted = 1 WHERE file_id = ?`, fileID)
	return err
}

func (s *SQLite) SetAvatarPublicKey(ctx context.Context, fileID, owner, key string) (File, error) {
	if _, err := s.db.ExecContext(ctx, `UPDATE files SET public_key = ? WHERE file_id = ? AND owner = ? AND kind = 'avatar' AND public_key = '' AND state IN ('ready', 'attached')`,
		key, fileID, owner); err != nil {
		return File{}, err
	}
	return s.GetFile(ctx, fileID)
}

func (s *SQLite) BindAvatar(ctx context.Context, fileID, owner, url string, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE files SET state = 'attached', avatar_replaced_at = 0, retain_until = 0, attached_at = CASE WHEN attached_at = 0 THEN ? ELSE attached_at END
		WHERE file_id = ? AND owner = ? AND kind = 'avatar' AND state IN ('ready', 'attached')`, now, fileID, owner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrFileNotReady
	}
	if _, err := tx.ExecContext(ctx, `UPDATE files SET avatar_replaced_at = ?
		WHERE owner = ? AND kind = 'avatar' AND state = 'attached' AND avatar_replaced_at = 0 AND file_id <> ?`, now, owner, fileID); err != nil {
		return err
	}
	res, err = tx.ExecContext(ctx, `UPDATE users SET avatar = ? WHERE user_id = ? AND deleted_at = 0`, url, owner)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

// bindFileTx 是 SaveMessage 里的绑定（第 4.3 节第 10 步）：只匹配本人、ready 的文件，影响 0 行 → ErrFileNotReady。
func bindFileTx(ctx context.Context, tx *sql.Tx, m Message) error {
	res, err := tx.ExecContext(ctx, `UPDATE files SET state = 'attached', msg_id = ?, conv_id = ?, attached_at = ?, retain_until = ?
		WHERE file_id = ? AND owner = ? AND state = 'ready'`, m.MsgID, m.ConvID, m.TS, m.AttachRetainUntil, m.AttachFileID, m.From)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrFileNotReady
	}
	return nil
}
