package store

import (
	"context"
	"fmt"
	"strings"
)

func (s *SQLite) SearchUsers(ctx context.Context, q UserSearch) ([]User, string, error) {
	where, args := []string{"user_id > ?"}, []any{q.After}
	if q.Q != "" {
		fold := asciiFold(q.Q)
		where = append(where, "((user_id >= ? AND user_id < ?) OR (nickname_fold >= ? AND nickname_fold < ?))")
		args = append(args, q.Q, prefixEnd(q.Q), fold, prefixEnd(fold))
	}
	if q.OfficialOnly {
		where = append(where, "official = 1")
	}
	args = append(args, q.Limit+1)
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, nickname, avatar, created_at, official FROM users WHERE `+
		strings.Join(where, " AND ")+` ORDER BY user_id LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Nickname, &u.Avatar, &u.CreatedAt, &u.Official); err != nil {
			return nil, "", err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) > q.Limit {
		return out[:q.Limit], out[q.Limit-1].UserID, nil
	}
	return out, "", nil
}

func (s *SQLite) SearchGroups(ctx context.Context, q GroupSearch) ([]GroupListItem, *GroupCursor, error) {
	where, args := []string{"1 = 1"}, []any{}
	if !q.IncludeDissolved {
		where = append(where, "dissolved_at = 0")
	}
	if q.Q != "" {
		fold := asciiFold(q.Q)
		where = append(where, "((group_id >= ? AND group_id < ?) OR (name_fold >= ? AND name_fold < ?))")
		args = append(args, q.Q, prefixEnd(q.Q), fold, prefixEnd(fold))
	}
	if q.After != nil {
		where = append(where, "(created_at, group_id) < (?, ?)")
		args = append(args, q.After.CreatedAt, q.After.GroupID)
	}
	args = append(args, q.Limit+1)
	// #nosec G202 -- where 和列名是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT `+groupColumns+`, dissolved_at FROM chat_groups WHERE `+
		strings.Join(where, " AND ")+` ORDER BY created_at DESC, group_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []GroupListItem{}
	for rows.Next() {
		var g GroupListItem
		if err := rows.Scan(&g.GroupID, &g.Name, &g.HistoryVisible, &g.CreatedAt, &g.MemberCount, &g.OwnerID, &g.CreatedBy,
			&g.Notice, &g.NoticeBy, &g.NoticeAt, &g.MaxMembers, &g.DissolvedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(out) > q.Limit {
		last := out[q.Limit-1]
		return out[:q.Limit], &GroupCursor{CreatedAt: last.CreatedAt, GroupID: last.GroupID}, nil
	}
	return out, nil, nil
}

func (s *SQLite) ListGroupMembersByID(ctx context.Context, groupID, after, prefix string, limit int) ([]GroupMemberRow, string, error) {
	where, args := "group_id = ? AND user_id > ?", []any{groupID, after}
	if prefix != "" {
		where += " AND user_id >= ? AND user_id < ?"
		args = append(args, prefix, prefixEnd(prefix))
	}
	args = append(args, limit+1)
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, role, joined_at, role_at FROM group_members WHERE `+where+` ORDER BY user_id LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []GroupMemberRow{}
	for rows.Next() {
		var r GroupMemberRow
		if err := rows.Scan(&r.UserID, &r.Role, &r.JoinedAt, &r.RoleAt); err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) > limit {
		return out[:limit], out[limit-1].UserID, nil
	}
	return out, "", nil
}

func (s *SQLite) ListMessageMeta(ctx context.Context, q MessageMetaQuery) ([]MessageMeta, bool, error) {
	where, args := []string{"conv_id = ?"}, []any{q.ConvID}
	if q.BeforeSeq > 0 {
		where = append(where, "seq < ?")
		args = append(args, q.BeforeSeq)
	}
	if q.From != "" {
		where = append(where, "from_uid = ?")
		args = append(args, q.From)
	}
	if q.SinceTS > 0 {
		where = append(where, "ts >= ?")
		args = append(args, q.SinceTS)
	}
	if q.UntilTS > 0 {
		where = append(where, "ts <= ?")
		args = append(args, q.UntilTS)
	}
	args = append(args, q.Limit+1)
	// SQLite 的 length() 对 TEXT 按字符（码点）计数；正文只参与计算长度，不离开数据库。
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	// 文件大小取 files.size（complete 时读到的真实值）；过期、撤回后行仍保留（F-FILE-06 规则 2a），大小照常返回，已删除的消息为 0。
	rows, err := s.db.QueryContext(ctx, `SELECT seq, msg_id, from_uid, type,
		CASE WHEN deleted_at > 0 THEN 0 ELSE COALESCE(length(json_extract(body, '$.text')), length(body)) END,
		CASE WHEN deleted_at > 0 THEN 0 ELSE COALESCE((SELECT f.size FROM files f WHERE f.msg_id = messages.msg_id AND f.conv_id = messages.conv_id AND f.kind IN ('image', 'file')), 0) END,
		ts, deleted_at, deleted_by
		FROM messages WHERE `+strings.Join(where, " AND ")+` ORDER BY seq DESC LIMIT ?`, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []MessageMeta{}
	for rows.Next() {
		var m MessageMeta
		if err := rows.Scan(&m.Seq, &m.MsgID, &m.From, &m.Type, &m.BodyLength, &m.FileSize, &m.TS, &m.DeletedAt, &m.DeletedBy); err != nil {
			return nil, false, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > q.Limit {
		return out[:q.Limit], true, nil
	}
	return out, false, nil
}

var _ AdminQueryStore = (*SQLite)(nil)

// PurgeAdminAuditBefore 物理删除 ts < before 的审计记录（保留 1 年，BE-220）。
// 故意不在 Store 接口上：审计对业务代码只有追加和查询，只有清理任务拿到这个窄接口（storetest 断言）。
func (s *SQLite) PurgeAdminAuditBefore(ctx context.Context, before int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM admin_audit WHERE ts < ?`, before)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *SQLite) ListUserGroups(ctx context.Context, userID, after string, limit int) ([]UserGroupRow, string, error) {
	cols := "g." + strings.ReplaceAll(groupColumns, ", ", ", g.")
	// #nosec G202 -- cols 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT `+cols+`, gm.role, gm.joined_at
		FROM group_members gm JOIN chat_groups g ON g.group_id = gm.group_id AND g.dissolved_at = 0
		WHERE gm.user_id = ? AND gm.group_id > ? ORDER BY gm.group_id LIMIT ?`, userID, after, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []UserGroupRow{}
	for rows.Next() {
		var g UserGroupRow
		if err := rows.Scan(&g.GroupID, &g.Name, &g.HistoryVisible, &g.CreatedAt, &g.MemberCount, &g.OwnerID, &g.CreatedBy,
			&g.Notice, &g.NoticeBy, &g.NoticeAt, &g.MaxMembers, &g.Role, &g.JoinedAt); err != nil {
			return nil, "", err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) > limit {
		return out[:limit], out[limit-1].GroupID, nil
	}
	return out, "", nil
}

func (s *SQLite) ListUserConvMeta(ctx context.Context, userID string, after *ConvCursor, limit int) ([]ConvRow, *ConvCursor, error) {
	inner := fmt.Sprintf(c2cConvSelect, "uc.user_id = ?") + " UNION ALL " + fmt.Sprintf(groupConvSelect, "gm.user_id = ?")
	args := []any{userID, userID}
	where := ""
	if after != nil {
		where = ` WHERE updated_at < ? OR (updated_at = ? AND conv_id > ?)`
		args = append(args, after.UpdatedAt, after.UpdatedAt, after.ConvID)
	}
	args = append(args, limit+1)
	// #nosec G202 -- 拼接的都是常量片段，取值全部走占位符
	q := `SELECT * FROM (SELECT ` + convColumns + ` FROM (` + inner + `))` + where + ` ORDER BY updated_at DESC, conv_id ASC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []ConvRow{}
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(out) > limit {
		last := out[limit-1]
		return out[:limit], &ConvCursor{UpdatedAt: last.UpdatedAt, ConvID: last.ConvID}, nil
	}
	return out, nil, nil
}

func (s *SQLite) AdminStats(ctx context.Context, from, to int64) (AdminStats, error) {
	var st AdminStats
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM users WHERE created_at >= ? AND created_at < ?),
		(SELECT count(*) FROM messages WHERE ts >= ? AND ts < ? AND type <> 'tip'),
		(SELECT count(*) FROM chat_groups WHERE dissolved_at = 0)`, from, to, from, to).Scan(&st.NewUsers, &st.Messages, &st.Groups)
	return st, err
}
