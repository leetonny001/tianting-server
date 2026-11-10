package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var _ MsgActionStore = (*SQLite)(nil)

func (s *SQLite) GetMessage(ctx context.Context, msgID string) (Message, error) {
	m, err := scanMessage(s.db.QueryRowContext(ctx, `SELECT `+msgColumns+` FROM messages WHERE msg_id = ?`, msgID))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	return m, err
}

func (s *SQLite) GetMessages(ctx context.Context, msgIDs []string) (map[string]Message, error) {
	out := make(map[string]Message, len(msgIDs))
	ids := slices.Compact(slices.Sorted(slices.Values(msgIDs)))
	for chunk := range slices.Chunk(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		// #nosec G202 -- 拼接的只有占位符，取值全部走参数绑定
		rows, err := s.db.QueryContext(ctx, `SELECT `+msgColumns+` FROM messages WHERE msg_id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out[m.MsgID] = m
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *SQLite) RecallMessage(ctx context.Context, req RecallReq) (RecallResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RecallResult{}, err
	}
	defer tx.Rollback()
	var conv, typ, body, quote string
	var seq, deletedAt, recalledAt int64
	err = tx.QueryRowContext(ctx, `SELECT conv_id, seq, type, body, deleted_at, recalled_at, quote_msg_id FROM messages WHERE msg_id = ?`, req.MsgID).
		Scan(&conv, &seq, &typ, &body, &deletedAt, &recalledAt, &quote)
	switch {
	case errors.Is(err, sql.ErrNoRows), err == nil && conv != req.ConvID:
		return RecallResult{}, ErrNotFound // msgId 属于其他会话也是 4044（AC-RCL-01-3）
	case err != nil:
		return RecallResult{}, err
	case typ == "tip":
		return RecallResult{}, ErrMsgNotDeletable
	case recalledAt > 0:
		return RecallResult{Seq: seq, Already: true}, nil // 幂等：不改任何东西，不再发事件（AC-RCL-01-4）
	case deletedAt > 0:
		return RecallResult{}, ErrNotFound // 已被后台删除：4044（F-RCL-04 规则 2）
	}
	// 加锁顺序 conv_seq → 文件行（第 4.3 节）：先递增 mod_ver（F-RCL-03 规则 2），再处理文件。
	if _, err := tx.ExecContext(ctx, `UPDATE conv_seq SET mod_ver = mod_ver + 1 WHERE conv_id = ?`, req.ConvID); err != nil {
		return RecallResult{}, err
	}
	// 文件消息（F-FILE-06 规则 4 的撤回路径）：文件名和原文一起加密，files.name 清空，立即停止发放地址（revoked），
	// 对象保留到隔离期结束后由 file_purge 物理删除（AC-FILE-06-3）。
	sealed, err := revokeMessageFileTx(ctx, tx, req.MsgID, body, req.Now, req.ExpiresAt)
	if err != nil {
		return RecallResult{}, err
	}
	nonce, ct, kid, err := req.Seal(req.MsgID, req.ConvID, withQuote(sealed, quote))
	if err != nil {
		return RecallResult{}, fmt.Errorf("seal message %s: %w", req.MsgID, err) // 隔离写失败整个事务回滚（AC-RCL-01-11）
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO message_quarantine (msg_id, conv_id, nonce, ciphertext, kid, deleted_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.MsgID, req.ConvID, nonce, ct, kid, req.Now, req.ExpiresAt); err != nil {
		return RecallResult{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE messages SET body = '{}', recalled_at = ?, recalled_by = ?, quote_msg_id = '' WHERE msg_id = ?`, max(req.Now, 1), req.By, req.MsgID); err != nil {
		return RecallResult{}, err
	}
	// 最后一条：摘要清空，原文不留在 conv_seq 里（AC-RCL-01-6、AC-RCL-03-5）。
	if _, err := tx.ExecContext(ctx,
		`UPDATE conv_seq SET last_text = '', last_recalled = 1, last_recalled_by = ? WHERE conv_id = ? AND last_msg_id = ?`, req.By, req.ConvID, req.MsgID); err != nil {
		return RecallResult{}, err
	}
	// 被撤回的提及不再计入（F-MEN-03 规则 1，AC-MEN-03-4）；@ 所有人的次数不退（单独计数，R-CB-07）。
	if err := deleteMentionsTx(ctx, tx, req.ConvID, seq); err != nil {
		return RecallResult{}, err
	}
	return RecallResult{Seq: seq}, tx.Commit()
}

// withQuote 把引用的 msgId 放进要加密保存的原文（消息行里的 quote_msg_id 随正文一起清空）。
func withQuote(sealed []byte, quote string) []byte {
	if quote == "" {
		return sealed
	}
	var b map[string]any
	if json.Unmarshal(sealed, &b) != nil || b == nil {
		return sealed
	}
	b["quote"] = map[string]string{"msgId": quote}
	out, err := json.Marshal(b)
	if err != nil {
		return sealed
	}
	return out
}

// deleteMentionsTx 删除一条消息的提及计数（撤回、后台删除）。
func deleteMentionsTx(ctx context.Context, tx *sql.Tx, convID string, seq int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM conv_mentions WHERE conv_id = ? AND seq = ?`, convID, seq); err != nil {
		return err
	}
	if gid, ok := strings.CutPrefix(convID, "group:"); ok {
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_mention_all WHERE group_id = ? AND seq = ?`, gid, seq); err != nil {
			return err
		}
	}
	return nil
}

// saveMentionsTx 是 SaveMessage 里的第 10 步：先扣 @ 所有人次数，再写提及计数。
func saveMentionsTx(ctx context.Context, tx *sql.Tx, m Message) error {
	if m.MentionAll == nil && len(m.Mentions) == 0 {
		return nil
	}
	if m.MentionAll != nil {
		if err := chargeMentionAllTx(ctx, tx, m); err != nil {
			return err
		}
	}
	if m.Fault != nil && m.Fault(FaultMentionWrite) != nil {
		return ErrMentionWrite
	}
	if m.MentionAll != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_mention_all (group_id, seq, ts, from_uid) VALUES (?, ?, ?, ?)`,
			m.MentionAll.GroupID, m.Seq, m.TS, m.From); err != nil {
			return err
		}
	}
	for _, uid := range m.Mentions {
		if uid == m.From {
			continue // 自己 @ 自己不算提及（F-MEN-03 规则 6）
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO conv_mentions (user_id, conv_id, seq) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, uid, m.ConvID, m.Seq); err != nil {
			return err
		}
	}
	return nil
}

const mentionHourSeconds = 3600

// chargeMentionAllTx 扣一次 @ 所有人：滚动 60 分钟内 < Hourly 且当天 < Daily 才放行。SQLite 单连接下整个事务串行（AC-MEN-02-9）；
// Postgres 实现要先锁 conv_seq 行（已在分配 seq 时锁住），再读计数行。
func chargeMentionAllTx(ctx context.Context, tx *sql.Tx, m Message) error {
	q := m.MentionAll
	if m.Fault != nil && m.Fault(FaultMentionAllCounter) != nil {
		return ErrMentionAllCounter
	}
	nowSec := m.TS / 1000
	if _, err := tx.ExecContext(ctx, `DELETE FROM mention_all_quota WHERE group_id = ? AND ((win = 'hour' AND bucket <= ?) OR (win = 'day' AND bucket < ?))`,
		q.GroupID, nowSec-mentionHourSeconds, q.Day); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT win, bucket, count FROM mention_all_quota WHERE group_id = ? ORDER BY bucket`, q.GroupID)
	if err != nil {
		return err
	}
	type bucket struct{ at, n int64 }
	var hour []bucket
	var hourTotal, day int64
	for rows.Next() {
		var win string
		var b bucket
		if err := rows.Scan(&win, &b.at, &b.n); err != nil {
			rows.Close()
			return err
		}
		switch {
		case win == "hour":
			hour = append(hour, b)
			hourTotal += b.n
		case win == "day" && b.at == q.Day:
			day += b.n
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if hourTotal >= int64(q.Hourly) || day >= int64(q.Daily) {
		var wait int64
		if day >= int64(q.Daily) {
			wait = (q.DayEnd - m.TS + 999) / 1000
		}
		if hourTotal >= int64(q.Hourly) {
			left := hourTotal
			for _, b := range hour { // 最早的先出窗口：出到剩下的少于上限为止
				left -= b.n
				if left < int64(q.Hourly) {
					wait = max(wait, b.at+mentionHourSeconds-nowSec)
					break
				}
			}
		}
		return &MentionAllLimitError{RetryAfter: max(wait, 1)}
	}
	for _, w := range []struct {
		win string
		at  int64
	}{{"hour", nowSec}, {"day", q.Day}} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO mention_all_quota (group_id, win, bucket, count) VALUES (?, ?, ?, 1)
			ON CONFLICT (group_id, win, bucket) DO UPDATE SET count = count + 1`, q.GroupID, w.win, w.at); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite) MentionCounts(ctx context.Context, userID string) (map[string]MentionCount, error) {
	// 只算当前所在的群（已退群的不计算，F-MEN-03 异常）、seq > max(已读, 入群)；@ 个人和 @ 所有人按 seq 去重。
	rows, err := s.db.QueryContext(ctx, `
		SELECT x.conv_id, count(DISTINCT x.seq), min(x.seq) FROM (
			SELECT cm.conv_id AS conv_id, cm.seq AS seq FROM conv_mentions cm WHERE cm.user_id = ?1
			UNION ALL
			SELECT 'group:' || a.group_id AS conv_id, a.seq AS seq FROM group_mention_all a
			JOIN group_members gm0 ON gm0.group_id = a.group_id AND gm0.user_id = ?1
			WHERE a.from_uid <> ?1
		) x
		JOIN group_members gm ON 'group:' || gm.group_id = x.conv_id AND gm.user_id = ?1
		LEFT JOIN read_seq rs ON rs.user_id = ?1 AND rs.conv_id = x.conv_id
		WHERE x.seq > max(COALESCE(rs.seq, 0), gm.join_seq)
		GROUP BY x.conv_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]MentionCount{}
	for rows.Next() {
		var conv string
		var c MentionCount
		if err := rows.Scan(&conv, &c.Unread, &c.First); err != nil {
			return nil, err
		}
		out[conv] = c
	}
	return out, rows.Err()
}

// PurgeMentions 删除已读之前的提及计数行（第 12.1 节"清理到已读之后"），和不在群里的人的行。不在 Store 接口上，只给定时任务 mention_purge。
func (s *SQLite) PurgeMentions(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM conv_mentions WHERE
		seq <= COALESCE((SELECT rs.seq FROM read_seq rs WHERE rs.user_id = conv_mentions.user_id AND rs.conv_id = conv_mentions.conv_id), 0)
		OR NOT EXISTS (SELECT 1 FROM group_members gm WHERE 'group:' || gm.group_id = conv_mentions.conv_id AND gm.user_id = conv_mentions.user_id)`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
