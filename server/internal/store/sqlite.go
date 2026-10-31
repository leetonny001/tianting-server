package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go 的 SQLite 驱动，不需要 cgo
)

const schema = `
CREATE TABLE IF NOT EXISTS conv_seq (
	conv_id TEXT PRIMARY KEY,
	max_seq INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	msg_id        TEXT PRIMARY KEY,
	conv_id       TEXT NOT NULL,
	seq           INTEGER NOT NULL,
	from_uid      TEXT NOT NULL,
	client_msg_id TEXT NOT NULL,
	type          TEXT NOT NULL,
	body          TEXT NOT NULL,
	ts            INTEGER NOT NULL,
	UNIQUE (conv_id, seq),
	UNIQUE (from_uid, client_msg_id)
);
CREATE TABLE IF NOT EXISTS chat_groups (
	group_id   TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS group_members (
	group_id  TEXT NOT NULL REFERENCES chat_groups(group_id),
	user_id   TEXT NOT NULL,
	joined_at INTEGER NOT NULL,
	PRIMARY KEY (group_id, user_id)
);
CREATE TABLE IF NOT EXISTS users (
	user_id            TEXT PRIMARY KEY,
	nickname           TEXT NOT NULL DEFAULT '',
	avatar             TEXT NOT NULL DEFAULT '',
	created_at         INTEGER NOT NULL,
	token_version      INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS read_seq (
	user_id TEXT NOT NULL,
	conv_id TEXT NOT NULL,
	seq     INTEGER NOT NULL,
	PRIMARY KEY (user_id, conv_id)
);
`

// SQLite 是 Store 的 SQLite 实现。
//
// 只开一个连接：SQLite 同一时刻只允许一个写者，单连接让所有操作排队，
// 避免 SQLITE_BUSY，seq 分配也天然串行。MVP 够用，换 Postgres 时再放开。
type SQLite struct {
	db *sql.DB
}

// OpenSQLite 打开（必要时创建）数据库文件并建表。
func OpenSQLite(path string) (*SQLite, error) {
	// synchronous=FULL：每次提交都 fsync，返回 ack 的消息一定已经落盘（PRD 第 6 节）。
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) SaveMessage(ctx context.Context, m Message) (Message, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, false, err
	}
	defer tx.Rollback()

	// 去重：同一发送者的同一 clientMsgId 只保存一次。
	existing, err := scanMessage(tx.QueryRowContext(ctx,
		`SELECT `+msgColumns+` FROM messages WHERE from_uid = ? AND client_msg_id = ?`,
		m.From, m.ClientMsgID))
	switch {
	case err == nil:
		return existing, true, tx.Commit()
	case !errors.Is(err, sql.ErrNoRows):
		return Message{}, false, err
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO conv_seq (conv_id, max_seq) VALUES (?, 1)
		 ON CONFLICT (conv_id) DO UPDATE SET max_seq = max_seq + 1
		 RETURNING max_seq`, m.ConvID).Scan(&m.Seq); err != nil {
		return Message{}, false, fmt.Errorf("allocate seq: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (msg_id, conv_id, seq, from_uid, client_msg_id, type, body, ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.MsgID, m.ConvID, m.Seq, m.From, m.ClientMsgID, m.Type, string(m.Body), m.TS); err != nil {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}
	return m, false, tx.Commit()
}

func (s *SQLite) FindByClientMsgID(ctx context.Context, from, clientMsgID string) (Message, bool, error) {
	m, err := scanMessage(s.db.QueryRowContext(ctx,
		`SELECT `+msgColumns+` FROM messages WHERE from_uid = ? AND client_msg_id = ?`, from, clientMsgID))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, false, nil
	}
	return m, err == nil, err
}

func (s *SQLite) ListMessages(ctx context.Context, convID string, afterSeq int64, limit int) ([]Message, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+msgColumns+` FROM messages WHERE conv_id = ? AND seq > ? ORDER BY seq LIMIT ?`,
		convID, afterSeq, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	msgs := make([]Message, 0, limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, false, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	return msgs, hasMore, nil
}

func (s *SQLite) ListMessagesBefore(ctx context.Context, convID string, beforeSeq, minSeq int64, limit int) ([]Message, bool, error) {
	// 先倒序取 limit+1 条，再翻转成升序。
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+msgColumns+` FROM messages WHERE conv_id = ? AND seq < ? AND seq >= ? ORDER BY seq DESC LIMIT ?`,
		convID, beforeSeq, minSeq, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	msgs := make([]Message, 0, limit+1)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, false, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	slices.Reverse(msgs)
	return msgs, hasMore, nil
}

func (s *SQLite) GroupExists(ctx context.Context, groupID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM chat_groups WHERE group_id = ?`, groupID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *SQLite) CreateGroup(ctx context.Context, groupID, name string, members []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chat_groups (group_id, name, created_at) VALUES (?, ?, ?)`,
		groupID, name, now); err != nil {
		if isUniqueViolation(err) {
			return ErrExists
		}
		return err
	}
	if err := insertMembers(ctx, tx, groupID, members, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) AddGroupMembers(ctx context.Context, groupID string, members []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM chat_groups WHERE group_id = ?`, groupID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := insertMembers(ctx, tx, groupID, members, time.Now().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) GroupMembers(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id FROM group_members WHERE group_id = ? ORDER BY joined_at, user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (s *SQLite) IsGroupMember(ctx context.Context, groupID, userID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *SQLite) UpsertUsers(ctx context.Context, users []User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	for _, u := range users {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO users (user_id, nickname, avatar, created_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT (user_id) DO UPDATE SET nickname = excluded.nickname, avatar = excluded.avatar`,
			u.UserID, u.Nickname, u.Avatar, now); err != nil {
			return fmt.Errorf("upsert user %s: %w", u.UserID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLite) EnsureUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (user_id, created_at) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		userID, time.Now().UnixMilli())
	return err
}

func (s *SQLite) GetUser(ctx context.Context, userID string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, nickname, avatar, created_at FROM users WHERE user_id = ?`, userID).
		Scan(&u.UserID, &u.Nickname, &u.Avatar, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *SQLite) UserExists(ctx context.Context, userID string) (bool, error) {
	_, err := s.GetUser(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *SQLite) RevokeTokens(ctx context.Context, userID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET token_version = token_version + 1 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) TokenVersion(ctx context.Context, userID string) (int64, error) {
	var v int64
	err := s.db.QueryRowContext(ctx, `SELECT token_version FROM users WHERE user_id = ?`, userID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func (s *SQLite) SetReadSeq(ctx context.Context, userID, convID string, seq int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO read_seq (user_id, conv_id, seq) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, conv_id) DO UPDATE SET seq = MAX(seq, excluded.seq)`,
		userID, convID, seq)
	return err
}

func (s *SQLite) ReadSeq(ctx context.Context, userID, convID string) (int64, error) {
	var seq int64
	err := s.db.QueryRowContext(ctx,
		`SELECT seq FROM read_seq WHERE user_id = ? AND conv_id = ?`, userID, convID).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return seq, err
}

const msgColumns = `msg_id, client_msg_id, conv_id, seq, from_uid, type, body, ts`

type rowScanner interface{ Scan(dest ...any) error }

func scanMessage(r rowScanner) (Message, error) {
	var m Message
	var body string
	if err := r.Scan(&m.MsgID, &m.ClientMsgID, &m.ConvID, &m.Seq, &m.From, &m.Type, &body, &m.TS); err != nil {
		return Message{}, err
	}
	m.Body = []byte(body)
	return m, nil
}

func insertMembers(ctx context.Context, tx *sql.Tx, groupID string, members []string, now int64) error {
	for _, uid := range members {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)
			 ON CONFLICT DO NOTHING`, groupID, uid, now); err != nil {
			return fmt.Errorf("add member %s: %w", uid, err)
		}
	}
	return nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
