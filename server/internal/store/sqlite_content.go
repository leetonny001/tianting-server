package store

import (
	"context"
	"database/sql"
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
