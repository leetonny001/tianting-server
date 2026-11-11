package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// C6 敏感词和举报的 SQLite 实现（BE-492～BE-496）。

func (s *SQLite) CountActiveWords(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blocked_words WHERE state = 'active'`).Scan(&n)
	return n, err
}

func (s *SQLite) CountPendingReportsBefore(ctx context.Context, before int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE state = 'pending' AND created_at < ?`, before).Scan(&n)
	return n, err
}

// ---- 词库（BE-493） ----

const wordColumns = `word_id, word, norm, compact, state, created_at, updated_at`

func scanWord(row interface{ Scan(...any) error }) (BlockedWord, error) {
	var w BlockedWord
	err := row.Scan(&w.WordID, &w.Word, &w.Norm, &w.Compact, &w.State, &w.CreatedAt, &w.UpdatedAt)
	return w, err
}

func (s *SQLite) ActiveWords(ctx context.Context) ([]WordKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT word_id, norm, compact FROM blocked_words WHERE state = 'active' ORDER BY word_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WordKey
	for rows.Next() {
		var k WordKey
		if err := rows.Scan(&k.WordID, &k.Norm, &k.Compact); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// insertWordTx 插入一个词；norm 已存在时不写，返回 false。word_id 在事务里取 MAX + 1（SQLite 只有一个写者；Postgres 随 B6 换成序列）。
func insertWordTx(ctx context.Context, tx *sql.Tx, w BlockedWord) (int64, bool, error) {
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(word_id), 0) + 1 FROM blocked_words`).Scan(&id); err != nil {
		return 0, false, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO blocked_words (`+wordColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (norm) DO NOTHING`,
		id, w.Word, w.Norm, w.Compact, WordActive, w.CreatedAt, w.CreatedAt)
	if err != nil {
		return 0, false, err
	}
	n, _ := res.RowsAffected()
	return id, n == 1, nil
}

func (s *SQLite) AddWord(ctx context.Context, w BlockedWord) (BlockedWord, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BlockedWord{}, false, err
	}
	defer tx.Rollback()
	_, created, err := insertWordTx(ctx, tx, w)
	if err != nil {
		return BlockedWord{}, false, err
	}
	got, err := scanWord(tx.QueryRowContext(ctx, `SELECT `+wordColumns+` FROM blocked_words WHERE norm = ?`, w.Norm))
	if err != nil {
		return BlockedWord{}, false, err
	}
	return got, created, tx.Commit()
}

func (s *SQLite) ImportWords(ctx context.Context, words []BlockedWord) (int, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	added, dup := 0, 0
	for _, w := range words {
		_, created, err := insertWordTx(ctx, tx, w)
		if err != nil {
			return 0, 0, err
		}
		if created {
			added++
		} else {
			dup++
		}
	}
	return added, dup, tx.Commit()
}

func (s *SQLite) SetWordState(ctx context.Context, wordID int64, state string, now int64) (BlockedWord, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE blocked_words SET state = ?, updated_at = ? WHERE word_id = ?`, state, now, wordID)
	if err != nil {
		return BlockedWord{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return BlockedWord{}, ErrNotFound
	}
	return scanWord(s.db.QueryRowContext(ctx, `SELECT `+wordColumns+` FROM blocked_words WHERE word_id = ?`, wordID))
}

func (s *SQLite) DeleteWord(ctx context.Context, wordID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM blocked_words WHERE word_id = ?`, wordID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// likeEscape 转义 LIKE 的通配符（ESCAPE '\'）。
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *SQLite) ListWords(ctx context.Context, q WordQuery) ([]BlockedWord, int64, error) {
	where := []string{"1 = 1"}
	var args []any
	if q.Q != "" {
		where = append(where, `norm LIKE ? ESCAPE '\'`)
		args = append(args, "%"+likeEscape(q.Q)+"%")
	}
	if q.State != "" {
		where = append(where, `state = ?`)
		args = append(args, q.State)
	}
	if q.Cursor > 0 {
		where = append(where, `word_id < ?`)
		args = append(args, q.Cursor)
	}
	args = append(args, q.Limit+1)
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT `+wordColumns+` FROM blocked_words WHERE `+strings.Join(where, " AND ")+` ORDER BY word_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []BlockedWord
	for rows.Next() {
		w, err := scanWord(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var next int64
	if len(out) > q.Limit {
		out = out[:q.Limit]
		next = out[len(out)-1].WordID
	}
	return out, next, nil
}

// ---- 命中记录（BE-494） ----

func (s *SQLite) RecordHit(ctx context.Context, h ModHit) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO moderation_hits (user_id, scene, word_id, at) VALUES (?, ?, ?, ?)`, h.UserID, h.Scene, h.WordID, h.At)
	return err
}

func (s *SQLite) UserHits(ctx context.Context, userID string, since int64, limit int) (map[string]int, []ModHit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT scene, COUNT(*) FROM moderation_hits WHERE user_id = ? AND at >= ? GROUP BY scene`, userID, since)
	if err != nil {
		return nil, nil, err
	}
	counts := map[string]int{}
	for rows.Next() {
		var scene string
		var n int
		if err := rows.Scan(&scene, &n); err != nil {
			rows.Close()
			return nil, nil, err
		}
		counts[scene] = n
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT user_id, scene, word_id, at FROM moderation_hits WHERE user_id = ? AND at >= ? ORDER BY at DESC, rowid DESC LIMIT ?`, userID, since, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var recent []ModHit
	for rows.Next() {
		var h ModHit
		if err := rows.Scan(&h.UserID, &h.Scene, &h.WordID, &h.At); err != nil {
			return nil, nil, err
		}
		recent = append(recent, h)
	}
	return counts, recent, rows.Err()
}

func (s *SQLite) PurgeHits(ctx context.Context, before int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM moderation_hits WHERE at < ?`, before)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---- 举报（BE-495、BE-496） ----

const reportColumns = `report_id, reporter, target_type, conv_id, target_user, group_id, msg_ids, reason, dedup_key, state, conclusion, handled_by, handled_at, created_at`

func scanReport(row interface{ Scan(...any) error }) (Report, error) {
	var r Report
	var ids string
	err := row.Scan(&r.ReportID, &r.Reporter, &r.TargetType, &r.ConvID, &r.TargetUser, &r.GroupID, &ids, &r.Reason, &r.DedupKey, &r.State,
		&r.Conclusion, &r.HandledBy, &r.HandledAt, &r.CreatedAt)
	if err == nil {
		err = json.Unmarshal([]byte(ids), &r.MsgIDs)
	}
	return r, err
}

func (s *SQLite) ReportQuotaUsed(ctx context.Context, userID string, day int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count FROM report_quota WHERE user_id = ? AND day = ?`, userID, day).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func addQuotaTx(ctx context.Context, tx *sql.Tx, userID string, day int64, limit int) (bool, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO report_quota (user_id, day, count) VALUES (?, ?, 0) ON CONFLICT (user_id, day) DO NOTHING`, userID, day); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `UPDATE report_quota SET count = count + 1 WHERE user_id = ? AND day = ? AND count < ?`, userID, day, limit)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *SQLite) AddReportQuota(ctx context.Context, userID string, day int64, limit int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := addQuotaTx(ctx, tx, userID, day, limit); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) CreateReport(ctx context.Context, n NewReport) (Report, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Report{}, false, err
	}
	defer tx.Rollback()
	r := n.Report
	prev, err := scanReport(tx.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE reporter = ? AND dedup_key = ? AND created_at > ?
		ORDER BY created_at DESC LIMIT 1`, r.Reporter, r.DedupKey, n.DedupSince))
	switch {
	case err == nil:
		return prev, true, nil // 幂等：不产生新记录，不计次数
	case !errors.Is(err, sql.ErrNoRows):
		return Report{}, false, err
	}
	ok, err := addQuotaTx(ctx, tx, r.Reporter, n.DayStart, n.DailyLimit)
	if err != nil {
		return Report{}, false, err
	}
	if !ok {
		return Report{}, false, ErrReportQuota
	}
	ids, _ := json.Marshal(r.MsgIDs)
	if r.MsgIDs == nil {
		ids = []byte("[]")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reports (`+reportColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', '', '', 0, ?)`,
		r.ReportID, r.Reporter, r.TargetType, r.ConvID, r.TargetUser, r.GroupID, string(ids), r.Reason, r.DedupKey, r.CreatedAt); err != nil {
		return Report{}, false, err
	}
	if n.Fault != nil {
		if err := n.Fault(); err != nil {
			return Report{}, false, err
		}
	}
	for _, e := range n.Evidence {
		if _, err := tx.ExecContext(ctx, `INSERT INTO report_evidence (report_id, idx, nonce, ciphertext, kid, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
			r.ReportID, e.Idx, e.Nonce, e.Ciphertext, e.KID, n.ExpiresAt); err != nil {
			return Report{}, false, err
		}
	}
	for _, id := range n.FileIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE files SET evidence_until = MAX(evidence_until, ?) WHERE file_id = ?`, n.ExpiresAt, id); err != nil {
			return Report{}, false, err
		}
	}
	r.State = ReportPending
	return r, false, tx.Commit()
}

func (s *SQLite) GetReport(ctx context.Context, reportID string) (Report, error) {
	r, err := scanReport(s.db.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE report_id = ?`, reportID))
	if errors.Is(err, sql.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	return r, err
}

func (s *SQLite) ReportEvidence(ctx context.Context, reportID string, now int64) ([]EvidenceRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT idx, nonce, ciphertext, kid FROM report_evidence WHERE report_id = ? AND expires_at > ? ORDER BY idx`, reportID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EvidenceRow
	for rows.Next() {
		var e EvidenceRow
		if err := rows.Scan(&e.Idx, &e.Nonce, &e.Ciphertext, &e.KID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLite) PurgeReports(ctx context.Context, evidenceBefore, reportsBefore, quotaBefore int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	total := 0
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM report_evidence WHERE expires_at <= ?`, []any{evidenceBefore}},
		{`DELETE FROM reports WHERE created_at <= ?`, []any{reportsBefore}},
		// 行和证据一起删（P-48 ②）：reports 行已经删了的，剩下的证据也删。
		{`DELETE FROM report_evidence WHERE report_id NOT IN (SELECT report_id FROM reports)`, nil},
		{`DELETE FROM report_quota WHERE day < ?`, []any{quotaBefore}},
	} {
		res, err := tx.ExecContext(ctx, q.sql, q.args...)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, tx.Commit()
}

func (s *SQLite) ReportableUser(ctx context.Context, reporter, target, c2cConvID string) (bool, error) {
	var ok int
	err := s.db.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM users WHERE user_id = ?) AND (
		EXISTS (SELECT 1 FROM friends WHERE (owner_id = ? AND peer_id = ?) OR (owner_id = ? AND peer_id = ?))
		OR EXISTS (SELECT 1 FROM conv_seq WHERE conv_id = ? AND max_seq > 0)
		OR EXISTS (SELECT 1 FROM group_members a JOIN group_members b ON a.group_id = b.group_id WHERE a.user_id = ? AND b.user_id = ?))`,
		target, reporter, target, target, reporter, c2cConvID, reporter, target).Scan(&ok)
	return ok == 1, err
}

func (s *SQLite) Quarantined(ctx context.Context, msgID string, now int64) (QuarantinedMessage, bool, error) {
	var q QuarantinedMessage
	err := s.db.QueryRowContext(ctx, `SELECT nonce, ciphertext, kid, expires_at FROM message_quarantine WHERE msg_id = ? AND expires_at > ?`, msgID, now).
		Scan(&q.Nonce, &q.Ciphertext, &q.KID, &q.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return q, false, nil
	}
	return q, err == nil, err
}

func (s *SQLite) ListReports(ctx context.Context, f ReportFilter) ([]Report, error) {
	where := []string{"1 = 1"}
	var args []any
	for _, c := range []struct {
		col, v string
	}{{"state", f.State}, {"reason", f.Reason}, {"target_type", f.TargetType}} {
		if c.v != "" {
			where = append(where, c.col+" = ?")
			args = append(args, c.v)
		}
	}
	if f.From > 0 {
		where = append(where, "created_at >= ?")
		args = append(args, f.From)
	}
	if f.To > 0 {
		where = append(where, "created_at <= ?")
		args = append(args, f.To)
	}
	args = append(args, f.Limit)
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT `+reportColumns+` FROM reports WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at DESC, report_id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) CountReportsAgainst(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE target_user = ?`, userID).Scan(&n)
	return n, err
}

func (s *SQLite) ResolveReport(ctx context.Context, reportID, result, conclusion, by string, at int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE reports SET state = ?, conclusion = ?, handled_by = ?, handled_at = ? WHERE report_id = ? AND state = 'pending'`,
		result, conclusion, by, at, reportID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return nil
	}
	if _, err := s.GetReport(ctx, reportID); err != nil {
		return err
	}
	return ErrReportHandled
}
