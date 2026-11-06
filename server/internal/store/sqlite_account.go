package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var _ AccountStore = (*SQLite)(nil)

const accountColumns = `a.user_id, a.username_lower, a.password_hash, a.credentials_version, a.credentials_changed_at,
	a.delete_requested_at, a.notice_version, a.created_at, a.last_login_at, COALESCE(u.deleted_at, 0)`

func scanAccount(row *sql.Row) (Account, error) {
	var a Account
	err := row.Scan(&a.UserID, &a.UsernameLower, &a.PasswordHash, &a.CredentialsVersion, &a.CredentialsChangedAt,
		&a.DeleteRequestedAt, &a.NoticeVersion, &a.CreatedAt, &a.LastLoginAt, &a.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	return a, err
}

func (s *SQLite) GetAccountByUsername(ctx context.Context, usernameLower string) (Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx,
		`SELECT `+accountColumns+` FROM accounts a LEFT JOIN users u ON u.user_id = a.user_id WHERE a.username_lower = ?`, usernameLower))
}

func (s *SQLite) GetAccount(ctx context.Context, userID string) (Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx,
		`SELECT `+accountColumns+` FROM accounts a LEFT JOIN users u ON u.user_id = a.user_id WHERE a.user_id = ?`, userID))
}

// 走 users 上 lower(user_id) 的索引。查询和事务里共用一条语句（R-CB-01）。
const usernameTakenSQL = `SELECT EXISTS (SELECT 1 FROM accounts WHERE username_lower = ?1) OR EXISTS (SELECT 1 FROM users WHERE lower(user_id) = ?1)`

func (s *SQLite) UsernameTaken(ctx context.Context, usernameLower string) (bool, error) {
	var taken bool
	err := s.db.QueryRowContext(ctx, usernameTakenSQL, usernameLower).Scan(&taken)
	return taken, err
}

func (s *SQLite) CreateAccount(ctx context.Context, n NewAccount) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var taken bool
	if err := tx.QueryRowContext(ctx, usernameTakenSQL, n.Account.UsernameLower).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return ErrExists
	}
	a := n.Account
	// 查重之后还可能被并发的注册抢先：accounts.username_lower 的唯一索引是最后一道（异常段，只有一个成功）。
	res, err := tx.ExecContext(ctx, `INSERT INTO accounts (user_id, username_lower, password_hash, notice_version, created_at, last_login_at)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, a.UserID, a.UsernameLower, a.PasswordHash, a.NoticeVersion, a.CreatedAt, a.LastLoginAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrExists
	}
	res, err = tx.ExecContext(ctx, `INSERT INTO users (user_id, nickname, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, a.UserID, n.Nickname, a.CreatedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrExists
	}
	if n.InviteHash != "" {
		// 超过 maxUses 的按无效处理（F-ACC-09 异常）。
		res, err := tx.ExecContext(ctx, `UPDATE invites SET used_count = used_count + 1,
				exhausted_at = CASE WHEN used_count + 1 >= max_uses THEN ?1 ELSE exhausted_at END
			WHERE code_hash = ?2 AND disabled_at = 0 AND expires_at > ?1 AND used_count < max_uses`, a.CreatedAt, n.InviteHash)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrInviteInvalid
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, gen, hash, created_at) VALUES (?, 1, ?, ?)`, a.UserID, n.RecoveryHash, a.CreatedAt); err != nil {
		return err
	}
	if err := insertDeviceSession(ctx, tx, n.Session); err != nil {
		return err
	}
	if n.BeforeCommit != nil {
		if err := n.BeforeCommit(); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertDeviceSession(ctx context.Context, tx *sql.Tx, d DeviceSession) error {
	pendings, err := marshalList(d.Pendings)
	if err != nil {
		return err
	}
	seen, err := marshalList(d.SeenIPPrefixes)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO device_sessions (sid, user_id, platform, device_name, current_hash, current_gen, next_gen, pendings,
			seen_ip_prefixes, created_at, last_seen_at, expires_at, absolute_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.SID, d.UserID, d.Platform, d.DeviceName, d.CurrentHash, d.CurrentGen, d.NextGen, pendings, seen, d.CreatedAt, d.LastSeenAt, d.ExpiresAt, d.AbsoluteExpiresAt)
	return err
}

// marshalList 把 nil 写成 []，读的时候不用区分。
func marshalList[T any](v []T) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}

func (s *SQLite) CommitLogin(ctx context.Context, c LoginCommit) (LoginResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginResult{}, err
	}
	defer tx.Rollback()
	// 第一条语句锁 accounts 行，持锁后才校验版本和账号状态（F-ACC-04 规则 8，RS7-01）。Postgres 下这条 UPDATE 同样拿行锁。
	res, err := tx.ExecContext(ctx, `UPDATE accounts SET last_login_at = ? WHERE user_id = ? AND credentials_version = ? AND delete_requested_at = 0`,
		c.Now, c.UserID, c.CredentialsVersion)
	if err != nil {
		return LoginResult{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return LoginResult{}, ErrCredentialsChanged
	}
	var result LoginResult
	if c.NewRecoveryHash != "" {
		var gen, acked int64
		if err := tx.QueryRowContext(ctx, `SELECT gen, acked_at FROM recovery_codes WHERE user_id = ? ORDER BY gen DESC LIMIT 1`, c.UserID).Scan(&gen, &acked); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return LoginResult{}, err
		}
		if gen > 0 && acked == 0 {
			// 没人确认的代没有 7 天保留，作废（F-ACC-05 规则 1）。
			if _, err := tx.ExecContext(ctx, `UPDATE recovery_codes SET revoked_at = ? WHERE user_id = ? AND gen = ?`, c.Now, c.UserID, gen); err != nil {
				return LoginResult{}, err
			}
			result.RecoveryGen = gen + 1
			if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, gen, hash, created_at) VALUES (?, ?, ?, ?)`, c.UserID, result.RecoveryGen, c.NewRecoveryHash, c.Now); err != nil {
				return LoginResult{}, err
			}
		}
	}
	if err := insertDeviceSession(ctx, tx, c.Session); err != nil {
		return LoginResult{}, err
	}
	if err := insertAccountEvent(ctx, tx, c.Event); err != nil {
		return LoginResult{}, err
	}
	if c.ThrottleKey != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_throttle WHERE throttle_key = ?`, c.ThrottleKey); err != nil {
			return LoginResult{}, err
		}
	}
	if c.BeforeCommit != nil {
		if err := c.BeforeCommit(); err != nil {
			return LoginResult{}, err
		}
	}
	return result, tx.Commit()
}

func insertAccountEvent(ctx context.Context, tx *sql.Tx, e AccountEvent) error {
	if e.EventID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		e.EventID = id.String()
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO account_events (event_id, user_id, kind, at, platform, ip_prefix, sid_hash) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.EventID, e.UserID, e.Kind, e.At, e.Platform, e.IPPrefix, e.SIDHash)
	return err
}

func (s *SQLite) UpdatePasswordHash(ctx context.Context, userID string, version int64, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET password_hash = ? WHERE user_id = ? AND credentials_version = ?`, hash, userID, version)
	return err
}

func (s *SQLite) RecoveryUnacked(ctx context.Context, userID string) (bool, error) {
	var acked int64
	err := s.db.QueryRowContext(ctx, `SELECT acked_at FROM recovery_codes WHERE user_id = ? ORDER BY gen DESC LIMIT 1`, userID).Scan(&acked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return acked == 0, err
}

func (s *SQLite) AckRecoveryCode(ctx context.Context, userID string, gen, now int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE recovery_codes SET acked_at = CASE WHEN acked_at = 0 THEN ?1 ELSE acked_at END
		WHERE user_id = ?2 AND gen = ?3 AND gen = (SELECT MAX(gen) FROM recovery_codes WHERE user_id = ?2)`, now, userID, gen)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRecoveryCodeStale
	}
	return nil
}

func (s *SQLite) GetDeviceSession(ctx context.Context, sid string) (DeviceSession, error) {
	var d DeviceSession
	var pendings, seen string
	err := s.db.QueryRowContext(ctx, `SELECT sid, user_id, platform, device_name, current_hash, current_gen, next_gen, pendings, overlap_until,
			overlap_min_gen, row_version, seen_ip_prefixes, created_at, last_seen_at, expires_at, absolute_expires_at, revoked_at, revoke_reason
		FROM device_sessions WHERE sid = ?`, sid).Scan(&d.SID, &d.UserID, &d.Platform, &d.DeviceName, &d.CurrentHash, &d.CurrentGen, &d.NextGen, &pendings,
		&d.OverlapUntil, &d.OverlapMinGen, &d.RowVersion, &seen, &d.CreatedAt, &d.LastSeenAt, &d.ExpiresAt, &d.AbsoluteExpiresAt, &d.RevokedAt, &d.RevokeReason)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceSession{}, ErrNotFound
	}
	if err != nil {
		return DeviceSession{}, err
	}
	if err := json.Unmarshal([]byte(pendings), &d.Pendings); err != nil {
		return DeviceSession{}, fmt.Errorf("device session pendings: %w", err)
	}
	if err := json.Unmarshal([]byte(seen), &d.SeenIPPrefixes); err != nil {
		return DeviceSession{}, fmt.Errorf("device session seen prefixes: %w", err)
	}
	return d, nil
}

func (s *SQLite) OtherDeviceSessions(ctx context.Context, userID, exceptSID string, now int64) (OtherSessions, error) {
	var o OtherSessions
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(last_seen_at), 0) FROM device_sessions
		WHERE user_id = ? AND sid <> ? AND revoked_at = 0 AND expires_at > ?`, userID, exceptSID, now).Scan(&o.Count, &o.LastActiveAt)
	return o, err
}

func (s *SQLite) HitThrottle(ctx context.Context, key string, now, windowMs int64, limit int, lockMs int64) (ThrottleHit, error) {
	var h ThrottleHit
	// 窗口过了就从 1 重新计；已经锁定的不延长。过期时间留够窗口和锁定都结束。
	err := s.db.QueryRowContext(ctx, `INSERT INTO auth_throttle (throttle_key, count, window_start, locked_until, expires_at)
		VALUES (?1, 1, ?2, CASE WHEN ?4 <= 1 AND ?5 > 0 THEN ?2 + ?5 ELSE 0 END, ?2 + ?3 + ?5)
		ON CONFLICT (throttle_key) DO UPDATE SET
			count = CASE WHEN auth_throttle.window_start + ?3 <= ?2 THEN 1 ELSE auth_throttle.count + 1 END,
			locked_until = CASE
				WHEN auth_throttle.locked_until > ?2 THEN auth_throttle.locked_until
				WHEN ?5 > 0 AND (CASE WHEN auth_throttle.window_start + ?3 <= ?2 THEN 1 ELSE auth_throttle.count + 1 END) >= ?4 THEN ?2 + ?5
				ELSE 0 END,
			window_start = CASE WHEN auth_throttle.window_start + ?3 <= ?2 THEN ?2 ELSE auth_throttle.window_start END,
			expires_at = ?2 + ?3 + ?5
		RETURNING count, window_start, locked_until`, key, now, windowMs, limit, lockMs).Scan(&h.Count, &h.WindowStart, &h.LockedUntil)
	return h, err
}

func (s *SQLite) PeekThrottle(ctx context.Context, key string, now, windowMs int64) (ThrottleHit, error) {
	var h ThrottleHit
	err := s.db.QueryRowContext(ctx, `SELECT count, window_start, locked_until FROM auth_throttle WHERE throttle_key = ?`, key).Scan(&h.Count, &h.WindowStart, &h.LockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return ThrottleHit{}, nil
	}
	if err != nil {
		return ThrottleHit{}, err
	}
	if h.WindowStart+windowMs <= now {
		h.Count = 0
	}
	if h.LockedUntil <= now {
		h.LockedUntil = 0
	}
	return h, nil
}

func (s *SQLite) ClearThrottle(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_throttle WHERE throttle_key = ?`, key)
	return err
}

func (s *SQLite) ResetThrottle(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_throttle`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE invites SET mismatch_count = 0, mismatch_window_start = 0`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) PurgeThrottle(ctx context.Context, now int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_throttle WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *SQLite) CreateInvite(ctx context.Context, inv Invite) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO invites (invite_id, code_hash, code_last4, max_uses, expires_at, note, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, inv.InviteID, inv.CodeHash, inv.CodeLast4, inv.MaxUses, inv.ExpiresAt, inv.Note, inv.CreatedBy, inv.CreatedAt)
	return err
}

func (s *SQLite) InviteUsable(ctx context.Context, codeHash string, now int64) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM invites WHERE code_hash = ? AND disabled_at = 0 AND expires_at > ? AND used_count < max_uses)`, codeHash, now).Scan(&ok)
	return ok, err
}

func (s *SQLite) GetInviteByHash(ctx context.Context, codeHash string) (Invite, error) {
	var i Invite
	err := s.db.QueryRowContext(ctx, `SELECT invite_id, code_hash, code_last4, note, created_by, max_uses, used_count, expires_at, created_at, disabled_at, auto_disabled
		FROM invites WHERE code_hash = ?`, codeHash).Scan(&i.InviteID, &i.CodeHash, &i.CodeLast4, &i.Note, &i.CreatedBy, &i.MaxUses, &i.UsedCount, &i.ExpiresAt, &i.CreatedAt, &i.DisabledAt, &i.AutoDisabled)
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrNotFound
	}
	return i, err
}

func (s *SQLite) RecordInviteMismatch(ctx context.Context, codeHash string, now, windowMs int64, limit int) (bool, error) {
	var disabled int64
	err := s.db.QueryRowContext(ctx, `UPDATE invites SET
			mismatch_count = CASE WHEN mismatch_window_start + ?2 <= ?1 THEN 1 ELSE mismatch_count + 1 END,
			mismatch_window_start = CASE WHEN mismatch_window_start + ?2 <= ?1 THEN ?1 ELSE mismatch_window_start END,
			auto_disabled = CASE WHEN disabled_at = 0 AND (CASE WHEN mismatch_window_start + ?2 <= ?1 THEN 1 ELSE mismatch_count + 1 END) >= ?3 THEN 1 ELSE auto_disabled END,
			disabled_at = CASE WHEN disabled_at = 0 AND (CASE WHEN mismatch_window_start + ?2 <= ?1 THEN 1 ELSE mismatch_count + 1 END) >= ?3 THEN ?1 ELSE disabled_at END
		WHERE code_hash = ?4
		RETURNING CASE WHEN auto_disabled = 1 AND disabled_at = ?1 THEN 1 ELSE 0 END`, now, windowMs, limit, codeHash).Scan(&disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return disabled == 1, err
}

func (s *SQLite) ListAccountEvents(ctx context.Context, userID string, limit int) ([]AccountEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, user_id, kind, at, platform, ip_prefix, sid_hash FROM account_events
		WHERE user_id = ? ORDER BY at DESC, event_id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountEvent
	for rows.Next() {
		var e AccountEvent
		if err := rows.Scan(&e.EventID, &e.UserID, &e.Kind, &e.At, &e.Platform, &e.IPPrefix, &e.SIDHash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLite) PurgeAccountEvents(ctx context.Context, before int64) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM account_events WHERE at < ?`, before)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *SQLite) UserDeleted(ctx context.Context, userID string) (bool, error) {
	var deleted int64
	err := s.db.QueryRowContext(ctx, `SELECT deleted_at FROM users WHERE user_id = ?`, userID).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return deleted != 0, err
}

func (s *SQLite) MarkUserDeleted(ctx context.Context, userID string, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET password_hash = '' WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE users SET deleted_at = ?, token_version = token_version + 1 WHERE user_id = ?`, now, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
