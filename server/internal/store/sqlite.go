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
	// secure_delete=ON：被删消息的原文不留在数据库文件的空闲页里（AC-ADM-32-1）。WAL 里的旧帧要等检查点之后才被覆盖。
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=secure_delete(ON)"
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

	// 陌生人限额（L6）：SQLite 单连接下整个事务串行；Postgres 实现要先锁 conv_seq 行再读 sent_count。
	if m.StrangerLimit > 0 && len(m.ConvUsers) == 2 {
		peer := m.ConvUsers[0]
		if peer == m.From {
			peer = m.ConvUsers[1]
		}
		mine, err := sentCount(ctx, tx, m.From, m.ConvID)
		if err != nil {
			return Message{}, false, err
		}
		theirs, err := sentCount(ctx, tx, peer, m.ConvID)
		if err != nil {
			return Message{}, false, err
		}
		if theirs == 0 && mine >= m.StrangerLimit {
			return Message{}, false, ErrStrangerMsgLimit
		}
	}

	// 分配 seq，同时记下会话的最后一条消息（同一行、同一个事务）。
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO conv_seq (conv_id, max_seq, last_msg_id, last_from, last_type, last_text, last_ts)
		 VALUES (?, 1, ?, ?, ?, ?, ?)
		 ON CONFLICT (conv_id) DO UPDATE SET max_seq = max_seq + 1,
		   last_msg_id = excluded.last_msg_id, last_from = excluded.last_from, last_type = excluded.last_type,
		   last_text = excluded.last_text, last_ts = excluded.last_ts, last_deleted = 0
		 RETURNING max_seq`, m.ConvID, m.MsgID, m.From, m.Type, m.Summary, m.TS).Scan(&m.Seq); err != nil {
		return Message{}, false, fmt.Errorf("allocate seq: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (msg_id, conv_id, seq, from_uid, client_msg_id, type, body, ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.MsgID, m.ConvID, m.Seq, m.From, m.ClientMsgID, m.Type, string(m.Body), m.TS); err != nil {
		return Message{}, false, fmt.Errorf("insert message: %w", err)
	}
	// 单聊：会话进入双方的会话列表。
	for i, uid := range m.ConvUsers {
		peer := uid
		if len(m.ConvUsers) == 2 {
			peer = m.ConvUsers[1-i]
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_conv (user_id, conv_id, peer_id, created_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT DO NOTHING`, uid, m.ConvID, peer, m.TS); err != nil {
			return Message{}, false, fmt.Errorf("add conversation: %w", err)
		}
	}
	// 单聊：发送者在这个会话里的条数 +1（陌生人限额，不封顶）。
	if len(m.ConvUsers) > 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE user_conv SET sent_count = sent_count + 1 WHERE user_id = ? AND conv_id = ?`, m.From, m.ConvID); err != nil {
			return Message{}, false, fmt.Errorf("count sent: %w", err)
		}
	}
	// 发言即已读：发送者的已读位置推进到这条消息。
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO read_seq (user_id, conv_id, seq) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, conv_id) DO UPDATE SET seq = MAX(seq, excluded.seq)`,
		m.From, m.ConvID, m.Seq); err != nil {
		return Message{}, false, fmt.Errorf("mark read: %w", err)
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
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM chat_groups WHERE group_id = ? AND dissolved_at = 0`, groupID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *SQLite) DissolveGroup(ctx context.Context, groupID string, now int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := groupExistsTx(ctx, tx, groupID); err != nil { // L5：Postgres 实现在这里 SELECT … FOR UPDATE
		return nil, err
	}
	members, err := dissolveMembersTx(ctx, tx, groupID, now)
	if err != nil {
		return nil, err
	}
	return members, tx.Commit()
}

// dissolveMembersTx 解散并返回被移出的成员（按 userId 排序）。
func dissolveMembersTx(ctx context.Context, tx *sql.Tx, groupID string, now int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM group_members WHERE group_id = ? ORDER BY user_id`, groupID)
	if err != nil {
		return nil, err
	}
	var members []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return nil, err
		}
		members = append(members, uid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, dissolveTx(ctx, tx, groupID, now)
}

func (s *SQLite) CreateGroup(ctx context.Context, g Group, members []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := g.CreatedAt
	if now == 0 {
		now = time.Now().UnixMilli()
	}
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
	if err := updateMemberCount(ctx, tx, g.GroupID); err != nil {
		return err
	}
	return tx.Commit()
}

// updateMemberCount 在加 / 减成员的同一个事务里重算成员数，并发修改也不会算错。
func updateMemberCount(ctx context.Context, tx *sql.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE chat_groups SET member_count = (SELECT count(*) FROM group_members WHERE group_id = ?) WHERE group_id = ?`, groupID, groupID)
	return err
}

func (s *SQLite) AddGroupMembers(ctx context.Context, groupID string, members []string, maxTotal int, joinedAt int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := groupExistsTx(ctx, tx, groupID); err != nil {
		return nil, err
	}
	added, err := insertMembers(ctx, tx, groupID, members, joinedAt)
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
	if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET member_count = ? WHERE group_id = ?`, total, groupID); err != nil {
		return nil, err
	}
	return added, tx.Commit()
}

func (s *SQLite) RemoveGroupMembers(ctx context.Context, groupID string, members []string, now int64) ([]string, *Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	g, err := lockGroup(ctx, tx, groupID)
	if err != nil {
		return nil, nil, err
	}
	var removed []string
	ownerGone := false
	for _, uid := range members {
		res, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, uid)
		if err != nil {
			return nil, nil, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			if err := deleteGroupMutesTx(ctx, tx, groupID, uid); err != nil {
				return nil, nil, err
			}
			removed = append(removed, uid)
			ownerGone = ownerGone || uid == g.OwnerID
		}
	}
	if err := updateMemberCount(ctx, tx, groupID); err != nil {
		return nil, nil, err
	}
	var tip *Message
	if ownerGone {
		res, err := afterOwnerGone(ctx, tx, g, now)
		if err != nil {
			return nil, nil, err
		}
		tip = res.Tip
	}
	return removed, tip, tx.Commit()
}

func (s *SQLite) GetGroup(ctx context.Context, groupID string) (Group, error) {
	var g Group
	err := s.db.QueryRowContext(ctx,
		`SELECT `+groupColumns+` FROM chat_groups WHERE group_id = ? AND dissolved_at = 0`, groupID).
		Scan(&g.GroupID, &g.Name, &g.HistoryVisible, &g.CreatedAt, &g.MemberCount, &g.OwnerID, &g.CreatedBy, &g.Notice, &g.NoticeBy, &g.NoticeAt, &g.MaxMembers)
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
	if u.Notice != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET notice = ?, notice_by = '', notice_at = ? WHERE group_id = ?`, *u.Notice, u.Now, groupID); err != nil {
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
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM chat_groups WHERE group_id = ? AND dissolved_at = 0`, groupID).Scan(&one)
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
			 ON CONFLICT (user_id) DO UPDATE SET nickname = excluded.nickname, avatar = excluded.avatar WHERE users.deleted_at = 0`,
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
		`SELECT user_id, nickname, avatar, created_at, official FROM users WHERE user_id = ?`, userID).
		Scan(&u.UserID, &u.Nickname, &u.Avatar, &u.CreatedAt, &u.Official)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *SQLite) UserExists(ctx context.Context, userID string) (bool, error) {
	// 已注销的用户按不存在处理（PRD-chat F-ACC-08 规则 4），GetUser 仍然能读到，后台要显示。
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE user_id = ? AND deleted_at = 0)`, userID).Scan(&exists)
	return exists, err
}

func (s *SQLite) MissingUsers(ctx context.Context, userIDs []string) ([]string, error) {
	// 分块一次查询：系统推送一次最多 1000 人（PRD-live AC-PUSH-04-4），逐个查询会反复占用唯一的数据库连接。
	found := make(map[string]bool, len(userIDs))
	for ids := range slices.Chunk(userIDs, 500) {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		// #nosec G202 -- 拼接的只有占位符，取值全部走参数绑定
		rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM users WHERE deleted_at = 0 AND user_id IN (`+placeholders(len(ids))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			found[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	var missing []string
	for _, uid := range userIDs {
		if !found[uid] {
			missing = append(missing, uid)
		}
	}
	return missing, nil
}

func (s *SQLite) RevokeTokens(ctx context.Context, userID string, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET token_version = token_version + 1 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	// 同时吊销全部设备会话（PRD-chat F-ACC-04 规则 4），refresh 一律 4019（F-ACC-03 规则 10）。
	if err := revokeUserSessions(ctx, tx, userID, RevokeReasonRevoked, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) TokenVersion(ctx context.Context, userID string) (int64, error) {
	var v int64
	err := s.db.QueryRowContext(ctx, `SELECT token_version FROM users WHERE user_id = ?`, userID).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func (s *SQLite) SetReadSeq(ctx context.Context, userID, convID string, seq int64) (int64, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	var maxSeq, cur int64
	if err := tx.QueryRowContext(ctx, `SELECT max_seq FROM conv_seq WHERE conv_id = ?`, convID).Scan(&maxSeq); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT seq FROM read_seq WHERE user_id = ? AND conv_id = ?`, userID, convID).Scan(&cur); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	seq = min(seq, maxSeq) // 超过 maxSeq 的按 maxSeq 记，未读数不会变成负数
	if seq <= cur {
		return cur, false, nil // 只前进不后退
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO read_seq (user_id, conv_id, seq) VALUES (?, ?, ?)
		 ON CONFLICT (user_id, conv_id) DO UPDATE SET seq = excluded.seq`, userID, convID, seq); err != nil {
		return 0, false, err
	}
	return seq, true, tx.Commit()
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

// 会话列表查询：单聊来自 user_conv，群来自 group_members。
// 群的 minSeq = historyVisible ? 1 : join_seq+1；最后一条消息的 seq（= max_seq）小于 minSeq 时
// 不可展示（入群前、且历史不可见），此时按入群时间排序，避免在会话列表里泄露入群前的消息。
// 两个分支都带 user_id 列，%s 处填 WHERE 条件。
const (
	c2cConvSelect = `
	SELECT uc.user_id, uc.conv_id, cs.max_seq, COALESCE(rs.seq, 0) AS read_seq, 0 AS join_seq,
	       '' AS group_name, 0 AS history_visible, uc.peer_id, COALESCE(u.nickname, '') AS peer_nickname, 0 AS member_count, '' AS my_role,
	       cs.max_seq > 0 AS last_visible, uc.created_at AS joined_at,
	       cs.last_msg_id, cs.last_from, cs.last_type, cs.last_text, cs.last_ts, cs.last_deleted
	FROM user_conv uc
	JOIN conv_seq cs ON cs.conv_id = uc.conv_id
	LEFT JOIN read_seq rs ON rs.user_id = uc.user_id AND rs.conv_id = uc.conv_id
	LEFT JOIN users u ON u.user_id = uc.peer_id
	WHERE %s`

	groupConvSelect = `
	SELECT gm.user_id, 'group:' || gm.group_id AS conv_id, COALESCE(cs.max_seq, 0) AS max_seq,
	       COALESCE(rs.seq, 0) AS read_seq, gm.join_seq,
	       g.name AS group_name, g.history_visible, '' AS peer_id, '' AS peer_nickname, g.member_count, gm.role AS my_role,
	       COALESCE(cs.max_seq, 0) >= CASE WHEN g.history_visible THEN 1 ELSE gm.join_seq + 1 END AS last_visible,
	       gm.joined_at,
	       COALESCE(cs.last_msg_id, '') AS last_msg_id, COALESCE(cs.last_from, '') AS last_from,
	       COALESCE(cs.last_type, '') AS last_type, COALESCE(cs.last_text, '') AS last_text,
	       COALESCE(cs.last_ts, 0) AS last_ts, COALESCE(cs.last_deleted, 0) AS last_deleted
	FROM group_members gm
	JOIN chat_groups g ON g.group_id = gm.group_id AND g.dissolved_at = 0
	LEFT JOIN conv_seq cs ON cs.conv_id = 'group:' || gm.group_id
	LEFT JOIN read_seq rs ON rs.user_id = gm.user_id AND rs.conv_id = 'group:' || gm.group_id
	WHERE %s`

	// convColumns 是外层统一选出的列，顺序与 scanConv 一致（第一列 user_id）。
	convColumns = `user_id, conv_id, max_seq, read_seq, join_seq, group_name, history_visible, peer_id, peer_nickname, member_count, my_role,
	       last_visible, CASE WHEN last_visible THEN last_ts ELSE joined_at END AS updated_at,
	       last_msg_id, last_from, last_type, last_text, last_ts, last_deleted`
)

// convQuery 拼出外层查询：branches 是已经填好条件的分支（UNION ALL）。
// 拼接的只有代码里的常量片段，用户输入全部通过 ? 占位符绑定，没有注入风险。
func convQuery(branches ...string) string {
	return `SELECT ` + convColumns + ` FROM (` + strings.Join(branches, " UNION ALL ") + `) ORDER BY updated_at DESC, conv_id ASC` // #nosec G202 -- constant fragments only
}

func (s *SQLite) ListConversations(ctx context.Context, userID string, limit int) ([]ConvRow, bool, error) {
	q := convQuery(fmt.Sprintf(c2cConvSelect, "uc.user_id = ?"), fmt.Sprintf(groupConvSelect, "gm.user_id = ?")) + ` LIMIT ?` // #nosec G202 -- constant fragments only; values are bound with ?
	rows, err := s.db.QueryContext(ctx, q, userID, userID, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]ConvRow, 0, min(limit, 64))
	for rows.Next() {
		c, err := scanConv(rows)
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

func (s *SQLite) GetConversation(ctx context.Context, userID, convID string) (ConvRow, bool, error) {
	q := convQuery(
		fmt.Sprintf(c2cConvSelect, "uc.user_id = ? AND uc.conv_id = ?"),
		fmt.Sprintf(groupConvSelect, "gm.user_id = ? AND 'group:' || gm.group_id = ?"))
	c, err := scanConv(s.db.QueryRowContext(ctx, q, userID, convID, userID, convID))
	if errors.Is(err, sql.ErrNoRows) {
		return ConvRow{}, false, nil
	}
	return c, err == nil, err
}

func (s *SQLite) GroupConversations(ctx context.Context, groupID string, userIDs []string) (map[string]ConvRow, error) {
	want := make(map[string]bool, len(userIDs))
	for _, u := range userIDs {
		want[u] = true
	}
	rows, err := s.db.QueryContext(ctx, convQuery(fmt.Sprintf(groupConvSelect, "gm.group_id = ?")), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]ConvRow, len(userIDs))
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		if want[c.UserID] {
			out[c.UserID] = c
		}
	}
	return out, rows.Err()
}

func scanConv(r rowScanner) (ConvRow, error) {
	var c ConvRow
	err := r.Scan(&c.UserID, &c.ConvID, &c.MaxSeq, &c.ReadSeq, &c.JoinSeq, &c.GroupName, &c.HistoryVisible, &c.PeerID, &c.PeerNickname, &c.MemberCount, &c.MyRole,
		&c.LastVisible, &c.UpdatedAt, &c.Last.MsgID, &c.Last.From, &c.Last.Type, &c.Last.Text, &c.Last.TS, &c.Last.Deleted)
	return c, err
}

const msgColumns = `msg_id, client_msg_id, conv_id, seq, from_uid, type, body, ts, deleted_at, deleted_by`

type rowScanner interface{ Scan(dest ...any) error }

func scanMessage(r rowScanner) (Message, error) {
	var m Message
	var body string
	if err := r.Scan(&m.MsgID, &m.ClientMsgID, &m.ConvID, &m.Seq, &m.From, &m.Type, &body, &m.TS, &m.DeletedAt, &m.DeletedBy); err != nil {
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

func (s *SQLite) AppendAdminAudit(ctx context.Context, e AdminAudit) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_audit (ts, caller, operator, request_id, route, targets_json, summary_json, phase, code)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TS, e.Caller, e.Operator, e.RequestID, e.Route, cmpOr(e.TargetsJSON, "{}"), cmpOr(e.SummaryJSON, "{}"), e.Phase, e.Code)
	return err
}

func (s *SQLite) QueryAdminAudit(ctx context.Context, q AdminAuditQuery) ([]AdminAudit, int64, error) {
	where := []string{"ts >= ?", "ts <= ?"}
	args := []any{q.FromMs, q.ToMs}
	for _, f := range []struct {
		col, val string
	}{{"caller", q.Caller}, {"operator", q.Operator}, {"request_id", q.RequestID}} {
		if f.val != "" {
			where = append(where, f.col+" = ?")
			args = append(args, f.val)
		}
	}
	if q.Cursor > 0 {
		where = append(where, "id < ?")
		args = append(args, q.Cursor)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit+1)
	// #nosec G202 -- where 只由上面的固定列名拼接，取值全部走占位符
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, caller, operator, request_id, route, targets_json, summary_json, phase, code
		 FROM admin_audit WHERE `+strings.Join(where, " AND ")+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminAudit
	for rows.Next() {
		var e AdminAudit
		if err := rows.Scan(&e.ID, &e.TS, &e.Caller, &e.Operator, &e.RequestID, &e.Route, &e.TargetsJSON, &e.SummaryJSON, &e.Phase, &e.Code); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var next int64
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	return out, next, nil
}

func cmpOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func (s *SQLite) SetNickname(ctx context.Context, userID, nickname string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET nickname = ? WHERE user_id = ?`, nickname, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
