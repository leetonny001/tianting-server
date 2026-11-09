package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

var _ ModerationStore = (*SQLite)(nil)

func nullMs(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func msPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func userExistsTx(ctx context.Context, tx *sql.Tx, userID string) error {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM users WHERE user_id = ?`, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *SQLite) BanUser(ctx context.Context, b Ban) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := userExistsTx(ctx, tx, b.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_bans (user_id, until, reason, by_who, at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (user_id) DO UPDATE SET until = excluded.until, reason = excluded.reason, by_who = excluded.by_who, at = excluded.at`,
		b.UserID, nullMs(b.Until), b.Reason, b.By, b.At); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET token_version = token_version + 1 WHERE user_id = ?`, b.UserID); err != nil {
		return err
	}
	// 封禁同时吊销全部设备会话（PRD-chat F-ACC-04 规则 4）。
	if err := revokeUserSessions(ctx, tx, b.UserID, RevokeReasonRevoked, b.At); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) UnbanUser(ctx context.Context, userID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := userExistsTx(ctx, tx, userID); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM user_bans WHERE user_id = ?`, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

func (s *SQLite) UserBan(ctx context.Context, userID string) (*Ban, error) {
	b := Ban{UserID: userID}
	var until sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT until, reason, by_who, at FROM user_bans WHERE user_id = ?`, userID).
		Scan(&until, &b.Reason, &b.By, &b.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.Until = msPtr(until)
	return &b, nil
}

func (s *SQLite) IssueState(ctx context.Context, userID string) (int64, *Ban, error) {
	var version int64
	var until, at sql.NullInt64
	var reason, by sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT u.token_version, b.until, b.reason, b.by_who, b.at FROM users u LEFT JOIN user_bans b ON b.user_id = u.user_id WHERE u.user_id = ?`, userID).
		Scan(&version, &until, &reason, &by, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	if !at.Valid {
		return version, nil, nil
	}
	return version, &Ban{UserID: userID, Until: msPtr(until), Reason: reason.String, By: by.String, At: at.Int64}, nil
}

func (s *SQLite) ActiveBan(ctx context.Context, userID string, now int64) (bool, *int64, error) {
	b, err := s.UserBan(ctx, userID)
	if err != nil || !b.Active(now) {
		return false, nil, err
	}
	return true, b.Until, nil
}

func (s *SQLite) DeleteMessages(ctx context.Context, req DeleteMessagesReq) ([]DeletedMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	type row struct {
		id   string
		seq  int64
		body string
	}
	var todo []row
	for _, id := range req.MsgIDs {
		var conv, typ, body string
		var seq, deletedAt int64
		err := tx.QueryRowContext(ctx, `SELECT conv_id, seq, type, body, deleted_at FROM messages WHERE msg_id = ?`, id).
			Scan(&conv, &seq, &typ, &body, &deletedAt)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("message %s: %w", id, ErrNotFound)
		case err != nil:
			return nil, err
		case conv != req.ConvID:
			return nil, fmt.Errorf("message %s: %w", id, ErrMsgWrongConv)
		case typ == "tip":
			return nil, fmt.Errorf("message %s: %w", id, ErrMsgNotDeletable)
		case deletedAt > 0:
			continue // 幂等：已经删除的不再处理，也不再发事件
		}
		todo = append(todo, row{id, seq, body})
	}
	out := make([]DeletedMessage, 0, len(todo))
	for _, m := range todo {
		// 文件消息（PRD-chat F-FILE-06 规则 4）：文件名移到隔离加密区（和原文一起加密），files.name 清空，立即停止发放地址（revoked），
		// 对象保留到隔离期结束后由 file_purge 物理删除。
		sealed, err := revokeMessageFileTx(ctx, tx, m.id, m.body, req.Now, req.ExpiresAt)
		if err != nil {
			return nil, err
		}
		nonce, ct, kid, err := req.Seal(m.id, req.ConvID, sealed)
		if err != nil {
			return nil, fmt.Errorf("seal message %s: %w", m.id, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO message_quarantine (msg_id, conv_id, nonce, ciphertext, kid, deleted_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			m.id, req.ConvID, nonce, ct, kid, req.Now, req.ExpiresAt); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE messages SET body = '{}', deleted_at = ?, deleted_by = ? WHERE msg_id = ?`, max(req.Now, 1), req.By, m.id); err != nil {
			return nil, err
		}
		// 被删的是会话最后一条：摘要改成占位，原文不留在 conv_seq 里。
		if _, err := tx.ExecContext(ctx,
			`UPDATE conv_seq SET last_text = '', last_deleted = 1 WHERE conv_id = ? AND last_msg_id = ?`, req.ConvID, m.id); err != nil {
			return nil, err
		}
		out = append(out, DeletedMessage{MsgID: m.id, Seq: m.seq})
	}
	slices.SortFunc(out, func(a, b DeletedMessage) int { return int(a.Seq - b.Seq) })
	return out, tx.Commit()
}

// revokeMessageFileTx 把消息绑定的文件改成 revoked，返回要加密保存的原文（文件消息带上文件名）。没有绑定文件时原样返回。
func revokeMessageFileTx(ctx context.Context, tx *sql.Tx, msgID, body string, now, until int64) ([]byte, error) {
	var fileID, kind, name string
	err := tx.QueryRowContext(ctx, `SELECT file_id, kind, name FROM files WHERE msg_id = ? AND state = 'attached'`, msgID).Scan(&fileID, &kind, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return []byte(body), nil
	}
	if err != nil {
		return nil, err
	}
	out := []byte(body)
	if kind == KindFile && name != "" {
		var b map[string]map[string]any
		if json.Unmarshal([]byte(body), &b) == nil && b["file"] != nil {
			b["file"]["name"] = name
			if raw, err := json.Marshal(b); err == nil {
				out = raw
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE files SET state = 'revoked', name = '', revoked_at = ?, retain_until = ? WHERE file_id = ? AND state = 'attached'`,
		now, until, fileID); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLite) ListQuarantined(ctx context.Context, convID string, beforeSeq, now int64, limit int) ([]QuarantinedMessage, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.msg_id, m.client_msg_id, m.conv_id, m.seq, m.from_uid, m.type, m.ts, m.deleted_at, m.deleted_by, q.nonce, q.ciphertext, q.kid, q.expires_at
		 FROM message_quarantine q JOIN messages m ON m.msg_id = q.msg_id
		 WHERE q.conv_id = ? AND q.expires_at > ? AND m.seq < ? ORDER BY m.seq DESC LIMIT ?`, convID, now, beforeSeq, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]QuarantinedMessage, 0, limit+1)
	for rows.Next() {
		var q QuarantinedMessage
		m := &q.Message
		if err := rows.Scan(&m.MsgID, &m.ClientMsgID, &m.ConvID, &m.Seq, &m.From, &m.Type, &m.TS, &m.DeletedAt, &m.DeletedBy, &q.Nonce, &q.Ciphertext, &q.KID, &q.ExpiresAt); err != nil {
			return nil, false, err
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	slices.Reverse(out)
	return out, more, nil
}

// PurgeQuarantine 物理删除到期的隔离原文（BE-224）。不在 Store 接口上，只给定时任务。
func (s *SQLite) PurgeQuarantine(ctx context.Context, now int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM message_quarantine WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// PurgeMsgGrants 物理删除 created_at < before 的授权记录：授权只有 15 分钟，配额只看当天，审计另有 admin_audit。
func (s *SQLite) PurgeMsgGrants(ctx context.Context, before int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM msg_grants WHERE created_at < ?`, before)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *SQLite) SetMute(ctx context.Context, m Mute) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := muteTargetTx(ctx, tx, m.Scope, m.UserID, m.GroupID); err != nil {
		return err
	}
	if m.Scope == MuteGroup {
		var one int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?`, m.GroupID, m.UserID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTargetNotMember
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mutes (scope, user_id, group_id, until, reason, by_who, at) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (scope, user_id, group_id) DO UPDATE SET until = excluded.until, reason = excluded.reason, by_who = excluded.by_who, at = excluded.at`,
		m.Scope, m.UserID, m.GroupID, nullMs(m.Until), m.Reason, m.By, m.At); err != nil {
		return err
	}
	return tx.Commit()
}

func muteTargetTx(ctx context.Context, tx *sql.Tx, scope, userID, groupID string) error {
	if scope == MuteGroup || scope == MuteGroupAll {
		return groupExistsTx(ctx, tx, groupID)
	}
	return userExistsTx(ctx, tx, userID)
}

func (s *SQLite) DeleteMute(ctx context.Context, scope, userID, groupID string) (*Mute, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := muteTargetTx(ctx, tx, scope, userID, groupID); err != nil {
		return nil, err
	}
	m := Mute{Scope: scope, UserID: userID, GroupID: groupID}
	var until sql.NullInt64
	err = tx.QueryRowContext(ctx, `DELETE FROM mutes WHERE scope = ? AND user_id = ? AND group_id = ? RETURNING until, reason, by_who, at`, scope, userID, groupID).
		Scan(&until, &m.Reason, &m.By, &m.At)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	m.Until = msPtr(until)
	return &m, tx.Commit()
}

func scanMutes(rows *sql.Rows) ([]Mute, error) {
	defer rows.Close()
	var out []Mute
	for rows.Next() {
		var m Mute
		var until sql.NullInt64
		if err := rows.Scan(&m.Scope, &m.UserID, &m.GroupID, &until, &m.Reason, &m.By, &m.At); err != nil {
			return nil, err
		}
		m.Until = msPtr(until)
		out = append(out, m)
	}
	return out, rows.Err()
}

const muteColumns = `scope, user_id, group_id, until, reason, by_who, at`

func (s *SQLite) UserMutes(ctx context.Context, userID string) ([]Mute, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+muteColumns+` FROM mutes WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	return scanMutes(rows)
}

func (s *SQLite) GroupAllMute(ctx context.Context, groupID string) (*Mute, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+muteColumns+` FROM mutes WHERE scope = ? AND user_id = '' AND group_id = ?`, MuteGroupAll, groupID)
	if err != nil {
		return nil, err
	}
	list, err := scanMutes(rows)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func (s *SQLite) UserGroupAllMutes(ctx context.Context, userID string) (map[string]Mute, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+muteColumns+` FROM mutes WHERE scope = ? AND user_id = ''
		AND group_id IN (SELECT group_id FROM group_members WHERE user_id = ?)`, MuteGroupAll, userID)
	if err != nil {
		return nil, err
	}
	list, err := scanMutes(rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Mute, len(list))
	for _, m := range list {
		out[m.GroupID] = m
	}
	return out, nil
}

func (s *SQLite) GroupMutes(ctx context.Context, groupID string) (map[string]Mute, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+muteColumns+` FROM mutes WHERE group_id = ? AND scope = ?`, groupID, MuteGroup)
	if err != nil {
		return nil, err
	}
	list, err := scanMutes(rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Mute, len(list))
	for _, m := range list {
		out[m.UserID] = m
	}
	return out, nil
}

// deleteGroupMutesTx 在成员离开群的同一事务里清除禁言记录（F-ADM-41 规则 4）。userID 为空表示整个群（解散），全群禁言也一起清除。
func deleteGroupMutesTx(ctx context.Context, tx *sql.Tx, groupID, userID string) error {
	if userID == "" {
		_, err := tx.ExecContext(ctx, `DELETE FROM mutes WHERE group_id = ? AND scope IN (?, ?)`, groupID, MuteGroup, MuteGroupAll)
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM mutes WHERE group_id = ? AND user_id = ? AND scope = ?`, groupID, userID, MuteGroup)
	return err
}

func (s *SQLite) CreateMsgGrant(ctx context.Context, g MsgGrant, q MsgGrantQuota) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var byOperator, byCaller int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM msg_grants WHERE operator = ? AND created_at >= ?`, g.Operator, q.DayStart).Scan(&byOperator); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM msg_grants WHERE caller = ? AND created_at >= ?`, g.Caller, q.DayStart).Scan(&byCaller); err != nil {
		return err
	}
	if byOperator >= q.PerOperator || byCaller >= q.PerCaller {
		return ErrMsgGrantQuota
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO msg_grants (grant_id, caller, operator, conv_id, quarantine, reason, ticket, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.GrantID, g.Caller, g.Operator, g.ConvID, g.Quarantine, g.Reason, g.Ticket, g.CreatedAt, g.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) GetMsgGrant(ctx context.Context, grantID string) (MsgGrant, error) {
	g := MsgGrant{GrantID: grantID}
	err := s.db.QueryRowContext(ctx,
		`SELECT caller, operator, conv_id, quarantine, reason, ticket, created_at, expires_at FROM msg_grants WHERE grant_id = ?`, grantID).
		Scan(&g.Caller, &g.Operator, &g.ConvID, &g.Quarantine, &g.Reason, &g.Ticket, &g.CreatedAt, &g.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MsgGrant{}, ErrMsgGrantNotFound
	}
	return g, err
}
