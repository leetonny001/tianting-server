package store

import (
	"context"
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
	rows, err := s.db.QueryContext(ctx, `SELECT seq, msg_id, from_uid, type,
		COALESCE(length(json_extract(body, '$.text')), length(body)), ts
		FROM messages WHERE `+strings.Join(where, " AND ")+` ORDER BY seq DESC LIMIT ?`, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []MessageMeta{}
	for rows.Next() {
		var m MessageMeta
		if err := rows.Scan(&m.Seq, &m.MsgID, &m.From, &m.Type, &m.BodyLength, &m.TS); err != nil {
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
