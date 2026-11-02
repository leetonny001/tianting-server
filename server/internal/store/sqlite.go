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

// SQLite 是 Store 的 SQLite 实现。
//
// 只开一个连接：SQLite 同一时刻只允许一个写者，单连接让所有操作排队，
// 避免 SQLITE_BUSY，seq 分配也天然串行。MVP 够用，换 Postgres 时再放开。
type SQLite struct {
	db *sql.DB
}

// SQLiteOptions 是打开 SQLite 时的选项。
type SQLiteOptions struct {
	// AutoMigrate 为 true 时打开即执行待执行的迁移（开发、测试）；为 false 时有待执行的迁移就报错，
	// 由发布流程显式执行 `im-server migrate up`（ARCH 3.1）。
	AutoMigrate bool
}

// OpenSQLite 打开（必要时创建）数据库文件，并检查或执行迁移。
func OpenSQLite(path string, opt SQLiteOptions) (*SQLite, error) {
	db, err := openSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	if err := ensureMigrated(context.Background(), db, opt.AutoMigrate); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func openSQLiteDB(path string) (*sql.DB, error) {
	// synchronous=FULL：每次提交都 fsync，返回 ack 的消息一定已经落盘（PRD 第 6 节）。
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// SQLiteMigrations 返回打包在二进制里的 SQLite 迁移。
func SQLiteMigrations() ([]Migration, error) {
	return LoadMigrations(migrationFS, "migrations/sqlite")
}

func ensureMigrated(ctx context.Context, db *sql.DB, auto bool) error {
	ms, err := SQLiteMigrations()
	if err != nil {
		return err
	}
	if auto {
		_, err := Migrate(ctx, db, ms)
		return err
	}
	st, err := Status(ctx, db, ms)
	if err != nil {
		return err
	}
	if len(st.Pending) > 0 {
		return fmt.Errorf("%d pending migration(s) starting at %04d_%s; run `im-server migrate up` first",
			len(st.Pending), st.Pending[0].Version, st.Pending[0].Name)
	}
	return nil
}

// MigrateSQLite 对数据库文件执行待执行的迁移（`im-server migrate up`）。
func MigrateSQLite(ctx context.Context, path string) ([]int, error) {
	db, err := openSQLiteDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ms, err := SQLiteMigrations()
	if err != nil {
		return nil, err
	}
	return Migrate(ctx, db, ms)
}

// SQLiteStatus 返回数据库文件的迁移状态（`im-server migrate status`）。
func SQLiteStatus(ctx context.Context, path string) (MigrationStatus, error) {
	db, err := openSQLiteDB(path)
	if err != nil {
		return MigrationStatus{}, err
	}
	defer db.Close()
	ms, err := SQLiteMigrations()
	if err != nil {
		return MigrationStatus{}, err
	}
	return Status(ctx, db, ms)
}

func (s *SQLite) Close() error { return s.db.Close() }

// Ping 检查数据库可用（/readyz）。
func (s *SQLite) Ping(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one)
}

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

func (s *SQLite) CreateGroup(ctx context.Context, g Group, members []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chat_groups (group_id, name, history_visible, created_at) VALUES (?, ?, ?, ?)`,
		g.GroupID, g.Name, g.HistoryVisible, now); err != nil {
		if isUniqueViolation(err) {
			return ErrExists
		}
		return err
	}
	if _, err := insertMembers(ctx, tx, g.GroupID, members, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) AddGroupMembers(ctx context.Context, groupID string, members []string, maxTotal int) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := groupExistsTx(ctx, tx, groupID); err != nil {
		return nil, err
	}
	added, err := insertMembers(ctx, tx, groupID, members, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM group_members WHERE group_id = ?`, groupID).Scan(&total); err != nil {
		return nil, err
	}
	if total > maxTotal {
		return nil, ErrLimitExceeded // 回滚：一人都不加
	}
	return added, tx.Commit()
}

func (s *SQLite) RemoveGroupMembers(ctx context.Context, groupID string, members []string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := groupExistsTx(ctx, tx, groupID); err != nil {
		return nil, err
	}
	var removed []string
	for _, uid := range members {
		res, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, uid)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			removed = append(removed, uid)
		}
	}
	return removed, tx.Commit()
}

func (s *SQLite) GetGroup(ctx context.Context, groupID string) (Group, error) {
	var g Group
	err := s.db.QueryRowContext(ctx,
		`SELECT group_id, name, history_visible, created_at FROM chat_groups WHERE group_id = ?`, groupID).
		Scan(&g.GroupID, &g.Name, &g.HistoryVisible, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

func (s *SQLite) UpdateGroup(ctx context.Context, groupID string, u GroupUpdate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := groupExistsTx(ctx, tx, groupID); err != nil {
		return err
	}
	if u.Name != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET name = ? WHERE group_id = ?`, *u.Name, groupID); err != nil {
			return err
		}
	}
	if u.HistoryVisible != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET history_visible = ? WHERE group_id = ?`, *u.HistoryVisible, groupID); err != nil {
			return err
		}
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

func (s *SQLite) GroupMember(ctx context.Context, groupID, userID string) (GroupMember, bool, error) {
	m := GroupMember{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT join_seq FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&m.JoinSeq)
	if errors.Is(err, sql.ErrNoRows) {
		return GroupMember{}, false, nil
	}
	return m, err == nil, err
}

func groupExistsTx(ctx context.Context, tx *sql.Tx, groupID string) error {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM chat_groups WHERE group_id = ?`, groupID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
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

func (s *SQLite) MissingUsers(ctx context.Context, userIDs []string) ([]string, error) {
	var missing []string
	for _, uid := range userIDs {
		ok, err := s.UserExists(ctx, uid)
		if err != nil {
			return nil, err
		}
		if !ok {
			missing = append(missing, uid)
		}
	}
	return missing, nil
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

// insertMembers 插入新成员并返回实际新加入的；已经是成员的不变（不重置 join_seq）。
// join_seq 取会话当前的 maxSeq，和消息的 seq 分配在同一个数据库里读取：
// SQLite 单连接天然互斥；Postgres 实现需要对 conv_seq 行加锁（FOR SHARE）再读。
func insertMembers(ctx context.Context, tx *sql.Tx, groupID string, members []string, now int64) ([]string, error) {
	var joinSeq int64
	err := tx.QueryRowContext(ctx, `SELECT max_seq FROM conv_seq WHERE conv_id = ?`, groupConvID(groupID)).Scan(&joinSeq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var added []string
	for _, uid := range members {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO group_members (group_id, user_id, joined_at, join_seq) VALUES (?, ?, ?, ?)
			 ON CONFLICT DO NOTHING`, groupID, uid, now, joinSeq)
		if err != nil {
			return nil, fmt.Errorf("add member %s: %w", uid, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added = append(added, uid)
		}
	}
	return added, nil
}

// groupConvID 与 protocol.GroupConvID 一致（store 不依赖 protocol 包）；契约测试会校验两者一致。
func groupConvID(groupID string) string { return "group:" + groupID }

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
