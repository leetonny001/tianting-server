package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// C5 会话设置的 SQLite 实现（BE-483～BE-485）。SQL 兼容 SQLite 和 Postgres，Postgres 实现随 B6。

const convSettingColumns = `conv_id, pinned_at, notify_off <> 0, deleted_at, cleared_seq`

func scanConvSetting(r rowScanner) (ConvSetting, error) {
	var c ConvSetting
	err := r.Scan(&c.ConvID, &c.PinnedAt, &c.NotifyOff, &c.DeletedAt, &c.ClearedSeq)
	return c, err
}

func (s *SQLite) ConvSettings(ctx context.Context, userID string) (map[string]ConvSetting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+convSettingColumns+` FROM conv_settings WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ConvSetting{}
	for rows.Next() {
		c, err := scanConvSetting(rows)
		if err != nil {
			return nil, err
		}
		out[c.ConvID] = c
	}
	return out, rows.Err()
}

func convSettingTx(ctx context.Context, tx *sql.Tx, userID, convID string) (ConvSetting, error) {
	c, err := scanConvSetting(tx.QueryRowContext(ctx, `SELECT `+convSettingColumns+` FROM conv_settings WHERE user_id = ? AND conv_id = ?`, userID, convID))
	if errors.Is(err, sql.ErrNoRows) {
		return ConvSetting{ConvID: convID}, nil
	}
	return c, err
}

// requireMemberTx：群会话在写设置的同一事务里确认还是成员（并发退群时不留下设置行）。单聊由调用方判断参与方。
func requireMemberTx(ctx context.Context, tx *sql.Tx, userID, convID string) error {
	gid, ok := strings.CutPrefix(convID, "group:")
	if !ok {
		return nil
	}
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?`, gid, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotGroupMember
	}
	return err
}

func upsertConvSettingTx(ctx context.Context, tx *sql.Tx, userID string, c ConvSetting) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO conv_settings (user_id, conv_id, pinned_at, notify_off, deleted_at, cleared_seq) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, conv_id) DO UPDATE SET pinned_at = excluded.pinned_at, notify_off = excluded.notify_off,
		deleted_at = excluded.deleted_at, cleared_seq = excluded.cleared_seq`,
		userID, c.ConvID, c.PinnedAt, boolInt(c.NotifyOff), c.DeletedAt, c.ClearedSeq)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *SQLite) SetConvSettings(ctx context.Context, req ConvSetReq) (ConvSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConvSetting{}, err
	}
	defer tx.Rollback()
	if err := requireMemberTx(ctx, tx, req.UserID, req.ConvID); err != nil {
		return ConvSetting{}, err
	}
	c, err := convSettingTx(ctx, tx, req.UserID, req.ConvID)
	if err != nil {
		return ConvSetting{}, err
	}
	if req.Pinned != nil {
		switch {
		case !*req.Pinned:
			c.PinnedAt = 0
		case c.PinnedAt == 0: // 已置顶的再次置顶幂等成功、pinnedAt 不变（AC-CVS-01-2）
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM conv_settings WHERE user_id = ? AND pinned_at > 0`, req.UserID).Scan(&n); err != nil {
				return ConvSetting{}, err
			}
			if n >= req.PinMax {
				return ConvSetting{}, ErrPinLimit
			}
			c.PinnedAt = max(req.Now, 1)
		}
	}
	if req.NotifyOff != nil {
		c.NotifyOff = *req.NotifyOff
	}
	if err := upsertConvSettingTx(ctx, tx, req.UserID, c); err != nil {
		return ConvSetting{}, err
	}
	return c, tx.Commit()
}

func (s *SQLite) DeleteConv(ctx context.Context, req ConvDeleteReq) (ConvSetting, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConvSetting{}, err
	}
	defer tx.Rollback()
	// 加锁顺序 conv_seq → conv_settings（第 4.3 节；Postgres 实现在这里 SELECT … FOR UPDATE）。并发发送的 seq 不会被清除：
	// 这里读到的 maxSeq 是提交顺序里在本事务之前的最后一条（AC-CVS-03-14）。
	var maxSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT max_seq FROM conv_seq WHERE conv_id = ?`, req.ConvID).Scan(&maxSeq); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ConvSetting{}, err
	}
	if err := requireMemberTx(ctx, tx, req.UserID, req.ConvID); err != nil {
		return ConvSetting{}, err
	}
	c, err := convSettingTx(ctx, tx, req.UserID, req.ConvID)
	if err != nil {
		return ConvSetting{}, err
	}
	c.ClearedSeq = max(c.ClearedSeq, min(req.UpToSeq, maxSeq)) // 不回退（AC-CVS-03-13）
	c.DeletedAt = max(req.Now, 1)
	c.PinnedAt = 0 // 删除时清除置顶，隐藏的会话不占名额（backend-chat 第 74 条）；免打扰保留（F-CVS-03 规则 3）
	if err := upsertConvSettingTx(ctx, tx, req.UserID, c); err != nil {
		return ConvSetting{}, err
	}
	if c.ClearedSeq > 0 { // 已读推进到 clearedSeq，和 read 命令一样只前进不后退
		if _, err := tx.ExecContext(ctx, `INSERT INTO read_seq (user_id, conv_id, seq) VALUES (?, ?, ?)
			ON CONFLICT (user_id, conv_id) DO UPDATE SET seq = max(read_seq.seq, excluded.seq)`, req.UserID, req.ConvID, c.ClearedSeq); err != nil {
			return ConvSetting{}, err
		}
	}
	return c, tx.Commit()
}

func (s *SQLite) DeletedConvs(ctx context.Context, userID string, limit int) ([]ConvSetting, bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT st.conv_id, st.pinned_at, st.notify_off <> 0, st.deleted_at, st.cleared_seq
		FROM conv_settings st LEFT JOIN conv_seq cs ON cs.conv_id = st.conv_id
		WHERE st.user_id = ? AND st.deleted_at > 0 AND COALESCE(cs.max_seq, 0) <= st.cleared_seq
		ORDER BY st.deleted_at DESC, st.conv_id ASC LIMIT ?`, userID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []ConvSetting
	for rows.Next() {
		c, err := scanConvSetting(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(out) > limit
	if truncated {
		out = out[:limit]
	}
	return out, truncated, nil
}

func (s *SQLite) ConvClearedSeq(ctx context.Context, userID, convID string) (int64, error) {
	var v int64
	err := s.db.QueryRowContext(ctx, `SELECT cleared_seq FROM conv_settings WHERE user_id = ? AND conv_id = ?`, userID, convID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func (s *SQLite) NotifyOffUsers(ctx context.Context, convID string, userIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for len(userIDs) > 0 { // 分批，避免超过 SQLite 的参数上限
		n := min(len(userIDs), 500)
		args := []any{convID}
		for _, u := range userIDs[:n] {
			args = append(args, u)
		}
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT user_id FROM conv_settings WHERE conv_id = ? AND notify_off <> 0 AND user_id IN (%s)`, placeholders(n)), args...) // #nosec G201 -- placeholders only
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err != nil {
				rows.Close()
				return nil, err
			}
			out[u] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		userIDs = userIDs[n:]
	}
	return out, nil
}

func (s *SQLite) TotalUnread(ctx context.Context, userID string) (int64, error) {
	// 普通会话：unread = maxSeq − max(readSeq, joinSeq)（和 toConversation 相同）；免打扰会话只算 mentionUnread。
	q := `SELECT COALESCE(sum(CASE WHEN max_seq > max(read_seq, join_seq) THEN max_seq - max(read_seq, join_seq) ELSE 0 END), 0)
		FROM (` + fmt.Sprintf(c2cConvSelect, "uc.user_id = ?") + ` UNION ALL ` + fmt.Sprintf(groupConvSelect, "gm.user_id = ?") + `) WHERE notify_off = 0` // #nosec G202 -- constant fragments only
	var normal int64
	if err := s.db.QueryRowContext(ctx, q, userID, userID).Scan(&normal); err != nil {
		return 0, err
	}
	counts, err := s.MentionCounts(ctx, userID)
	if err != nil {
		return 0, err
	}
	off, err := s.ConvSettings(ctx, userID)
	if err != nil {
		return 0, err
	}
	for conv, c := range counts {
		if off[conv].NotifyOff {
			normal += c.Unread
		}
	}
	return normal, nil
}

// deleteConvSettingsTx 在成员离开群的同一事务里删除设置（F-CVS-01 规则 6、F-CVS-03 规则 6a）。userID 为空表示整个群（解散）。
func deleteConvSettingsTx(ctx context.Context, tx *sql.Tx, groupID, userID string) error {
	if userID == "" {
		_, err := tx.ExecContext(ctx, `DELETE FROM conv_settings WHERE conv_id = ?`, groupConvID(groupID))
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM conv_settings WHERE conv_id = ? AND user_id = ?`, groupConvID(groupID), userID)
	return err
}
