package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

var _ PushStore = (*SQLite)(nil)

// pushChunk 是一条多行语句带的用户数。1000 人的提交逐行写要三千条语句，单连接下这段时间单聊都在排队（AC-PUSH-04-4）。
const pushChunk = 200

const pushRecordColumns = `push_id, caller, content_hash, target, accepted, state, first_ts, created_at`

func scanPushRecord(row interface{ Scan(...any) error }) (PushRecord, error) {
	var r PushRecord
	var accepted sql.NullInt64
	err := row.Scan(&r.PushID, &r.Caller, &r.ContentHash, &r.Target, &accepted, &r.State, &r.FirstTS, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if accepted.Valid {
		n := int(accepted.Int64)
		r.Accepted = &n
	}
	return r, err
}

func (s *SQLite) GetPushRecord(ctx context.Context, pushID string) (PushRecord, error) {
	return scanPushRecord(s.db.QueryRowContext(ctx, `SELECT `+pushRecordColumns+` FROM push_idempotency WHERE push_id = ?`, pushID))
}

// insertPushRecord 写幂等记录；pushId 已有记录时返回那条记录，什么都不写。
func insertPushRecord(ctx context.Context, tx *sql.Tx, r PushRecord) (*PushRecord, error) {
	var accepted sql.NullInt64
	if r.Accepted != nil {
		accepted = sql.NullInt64{Int64: int64(*r.Accepted), Valid: true}
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO push_idempotency (`+pushRecordColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (push_id) DO NOTHING`,
		r.PushID, r.Caller, r.ContentHash, r.Target, accepted, r.State, r.FirstTS, r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil, nil
	}
	cur, err := scanPushRecord(tx.QueryRowContext(ctx, `SELECT `+pushRecordColumns+` FROM push_idempotency WHERE push_id = ?`, r.PushID))
	if err != nil {
		return nil, err
	}
	return &cur, nil
}

func (s *SQLite) CreatePendingPush(ctx context.Context, r PushRecord) (*PushRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r.State = PushPending
	existing, err := insertPushRecord(ctx, tx, r)
	if err != nil {
		return nil, err
	}
	return existing, tx.Commit()
}

func (s *SQLite) MarkPushDone(ctx context.Context, pushID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE push_idempotency SET state = ? WHERE push_id = ?`, PushDone, pushID)
	return err
}

func (s *SQLite) WritePushInbox(ctx context.Context, w PushInboxWrite) (PushInboxResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PushInboxResult{}, err
	}
	defer tx.Rollback()
	rec := w.Record
	rec.State = PushDone
	existing, err := insertPushRecord(ctx, tx, rec)
	if err != nil {
		return PushInboxResult{}, err
	}
	if existing != nil {
		return PushInboxResult{Existing: existing}, tx.Commit()
	}
	// 按 userId 字节序加锁（F-PUSH-11 规则 1）。SQLite 单连接下顺序写错不会死锁，只有这个断言能发现（规则 2）。
	users := slices.Clone(w.UserIDs)
	slices.Sort(users)
	users = slices.Compact(users)
	order := socialTx{}
	res := PushInboxResult{Seqs: make(map[string]int64, len(users))}
	for start := 0; start < len(users); start += pushChunk {
		chunk := users[start:min(start+pushChunk, len(users))]
		for i, uid := range chunk {
			order.lock(lockKindUser, uid)
			if w.FailAt != nil {
				if err := w.FailAt(start + i); err != nil {
					return PushInboxResult{}, err
				}
			}
		}
		if err := writePushChunk(ctx, tx, chunk, w, res.Seqs); err != nil {
			return PushInboxResult{}, err
		}
	}
	if w.InboxMax > 0 {
		for _, uid := range users {
			if res.Seqs[uid] <= int64(w.InboxMax) { // 序号连续分配：序号没超过上限，行数就不可能超过上限
				continue
			}
			n, err := evictPushInbox(ctx, tx, uid, w.InboxMax)
			if err != nil {
				return PushInboxResult{}, err
			}
			res.Evicted += n
		}
	}
	return res, tx.Commit()
}

func writePushChunk(ctx context.Context, tx *sql.Tx, users []string, w PushInboxWrite, seqs map[string]int64) error {
	args := make([]any, 0, len(users))
	for _, uid := range users {
		args = append(args, uid)
	}
	rows, err := tx.QueryContext(ctx,
		`INSERT INTO push_seq (user_id, max_seq) VALUES `+strings.TrimSuffix(strings.Repeat(`(?, 1),`, len(users)), ",")+
			` ON CONFLICT (user_id) DO UPDATE SET max_seq = max_seq + 1 RETURNING user_id, max_seq`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var uid string
		var seq int64
		if err := rows.Scan(&uid, &seq); err != nil {
			rows.Close()
			return err
		}
		seqs[uid] = seq
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	const row = `(?, ?, ?, ?, ?, ?, ?),`
	args = make([]any, 0, len(users)*7)
	data := string(w.Data)
	for _, uid := range users {
		args = append(args, uid, seqs[uid], w.Record.PushID, w.Type, data, w.Record.FirstTS, w.ExpiresAt)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO push_inbox (user_id, push_seq, push_id, type, data, created_at, expires_at) VALUES `+
		strings.TrimSuffix(strings.Repeat(row, len(users)), ","), args...)
	return err
}

// evictPushInbox 只留最新的 max 行，序号不变（F-PUSH-09 规则 4）。行数不到 max 时子查询为 NULL，什么都不删。
func evictPushInbox(ctx context.Context, tx *sql.Tx, uid string, max int) (int, error) {
	res, err := tx.ExecContext(ctx, `DELETE FROM push_inbox WHERE user_id = ? AND push_seq <
		(SELECT push_seq FROM push_inbox WHERE user_id = ? ORDER BY push_seq DESC LIMIT 1 OFFSET ?)`, uid, uid, max-1)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

const pushInboxColumns = `user_id, push_seq, push_id, type, data, created_at, expires_at`

func scanPushRows(rows *sql.Rows) ([]PushInboxRow, error) {
	defer rows.Close()
	out := []PushInboxRow{}
	for rows.Next() {
		var r PushInboxRow
		var data string
		if err := rows.Scan(&r.UserID, &r.PushSeq, &r.PushID, &r.Type, &data, &r.CreatedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLite) ReadPushInbox(ctx context.Context, userID string, afterSeq int64, limit int, now int64) ([]PushInboxRow, bool, int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, 0, err
	}
	defer tx.Rollback()
	var latest int64
	err = tx.QueryRowContext(ctx, `SELECT max_seq FROM push_seq WHERE user_id = ?`, userID).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, 0, err
	}
	// 读取时按 expires_at 过滤，不依赖定时清理（4.3 节）。多取一行判断 hasMore：后面只剩过期的行时是 false。
	rows, err := tx.QueryContext(ctx, `SELECT `+pushInboxColumns+` FROM push_inbox
		WHERE user_id = ? AND push_seq > ? AND expires_at > ? ORDER BY push_seq LIMIT ?`, userID, afterSeq, now, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	out, err := scanPushRows(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, latest, tx.Commit()
}

func (s *SQLite) LatestPushSeq(ctx context.Context, userID string) (int64, error) {
	var latest int64
	err := s.db.QueryRowContext(ctx, `SELECT max_seq FROM push_seq WHERE user_id = ?`, userID).Scan(&latest)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return latest, err
}

func (s *SQLite) ListPushInbox(ctx context.Context, userID string) ([]PushInboxRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+pushInboxColumns+` FROM push_inbox WHERE user_id = ? ORDER BY push_seq`, userID)
	if err != nil {
		return nil, err
	}
	return scanPushRows(rows)
}

func (s *SQLite) CountPushInbox(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM push_inbox`).Scan(&n)
	return n, err
}

func (s *SQLite) PurgePush(ctx context.Context, now, recordsBefore int64, batch int) (int, int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM push_inbox WHERE rowid IN
		(SELECT rowid FROM push_inbox WHERE expires_at <= ? ORDER BY expires_at LIMIT ?)`, now, batch)
	if err != nil {
		return 0, 0, err
	}
	inbox, _ := res.RowsAffected()
	res, err = s.db.ExecContext(ctx, `DELETE FROM push_idempotency WHERE push_id IN
		(SELECT i.push_id FROM push_idempotency i WHERE i.created_at <= ?
		 AND NOT EXISTS (SELECT 1 FROM push_inbox b WHERE b.push_id = i.push_id) ORDER BY i.created_at LIMIT ?)`, recordsBefore, batch)
	if err != nil {
		return int(inbox), 0, err
	}
	records, _ := res.RowsAffected()
	return int(inbox), int(records), nil
}
