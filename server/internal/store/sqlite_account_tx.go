package store

import (
	"context"
	"database/sql"
	"errors"
)

// 用密码和恢复码的提交事务（PRD-chat F-ACC-04 规则 8）。加锁顺序一律 accounts → recovery_codes → device_sessions（第 4.3 节，RS6-01）。

// lockAccount 是用密码的提交事务的第一条语句（RS7-01）：不改值的条件 UPDATE 拿 accounts 行锁，持锁后才校验版本和账号状态。
// 影响 0 行说明版本或状态变了。Postgres 下这条 UPDATE 同样拿行锁，等到锁后重新求值。
func lockAccount(ctx context.Context, tx *sql.Tx, userID string, version int64) error {
	res, err := tx.ExecContext(ctx, `UPDATE accounts SET credentials_version = credentials_version
		WHERE user_id = ? AND credentials_version = ? AND delete_requested_at = 0 AND password_hash <> ''`, userID, version)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCredentialsChanged
	}
	return nil
}

// rotateRecovery 换代（F-ACC-05 规则 1、5）：已确认、没用过、没作废的旧码写 retire_until（只写一次，之后不延长），
// 没人确认的代作废（它没有 7 天保留）。返回新的代号。
func rotateRecovery(ctx context.Context, tx *sql.Tx, userID, hash string, now, retireUntil int64) (int64, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE recovery_codes SET retire_until = ?
		WHERE user_id = ? AND acked_at <> 0 AND used_at = 0 AND revoked_at = 0 AND retire_until = 0`, retireUntil, userID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE recovery_codes SET revoked_at = ?
		WHERE user_id = ? AND acked_at = 0 AND used_at = 0 AND revoked_at = 0`, now, userID); err != nil {
		return 0, err
	}
	var gen int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(gen), 0) + 1 FROM recovery_codes WHERE user_id = ?`, userID).Scan(&gen); err != nil {
		return 0, err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, gen, hash, created_at) VALUES (?, ?, ?, ?)`, userID, gen, hash, now)
	return gen, err
}

func (s *SQLite) CommitCredentials(ctx context.Context, c CredentialCommit) (CredentialResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CredentialResult{}, err
	}
	defer tx.Rollback()
	if err := lockAccount(ctx, tx, c.UserID, c.Version); err != nil {
		return CredentialResult{}, err
	}
	if c.CallerSID != "" {
		var revoked int64
		err := tx.QueryRowContext(ctx, `SELECT revoked_at FROM device_sessions WHERE sid = ? AND user_id = ?`, c.CallerSID, c.UserID).Scan(&revoked)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && revoked != 0) {
			return CredentialResult{}, ErrCredentialsChanged
		}
		if err != nil {
			return CredentialResult{}, err
		}
	}
	if c.NewPasswordHash != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET password_hash = ?, credentials_version = credentials_version + 1, credentials_changed_at = ? WHERE user_id = ?`,
			c.NewPasswordHash, c.Now, c.UserID); err != nil {
			return CredentialResult{}, err
		}
	}
	if c.RequestDeletion {
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET delete_requested_at = ? WHERE user_id = ?`, c.Now, c.UserID); err != nil {
			return CredentialResult{}, err
		}
	}
	var res CredentialResult
	if c.NewRecoveryHash != "" {
		if res.RecoveryGen, err = rotateRecovery(ctx, tx, c.UserID, c.NewRecoveryHash, c.Now, c.RetireUntil); err != nil {
			return CredentialResult{}, err
		}
	}
	if c.RevokeReason != "" {
		if err := revokeUserSessions(ctx, tx, c.UserID, c.RevokeReason, c.Now); err != nil {
			return CredentialResult{}, err
		}
	}
	if c.NewSession != nil {
		if err := insertDeviceSession(ctx, tx, *c.NewSession); err != nil {
			return CredentialResult{}, err
		}
	}
	if err := insertAccountEvent(ctx, tx, c.Event); err != nil {
		return CredentialResult{}, err
	}
	if c.ThrottleKey != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_throttle WHERE throttle_key = ?`, c.ThrottleKey); err != nil {
			return CredentialResult{}, err
		}
	}
	if c.BeforeCommit != nil {
		if err := c.BeforeCommit(); err != nil {
			return CredentialResult{}, err
		}
	}
	return res, tx.Commit()
}

func (s *SQLite) FindRecoveryCode(ctx context.Context, userID, hash string) (RecoveryCode, error) {
	var r RecoveryCode
	err := s.db.QueryRowContext(ctx, `SELECT gen, acked_at, retire_until, used_at, revoked_at,
			(SELECT MAX(gen) FROM recovery_codes m WHERE m.user_id = c.user_id)
		FROM recovery_codes c WHERE user_id = ? AND hash = ?`, userID, hash).Scan(&r.Gen, &r.AckedAt, &r.RetireUntil, &r.UsedAt, &r.RevokedAt, &r.LatestGen)
	if errors.Is(err, sql.ErrNoRows) {
		return RecoveryCode{}, ErrNotFound
	}
	return r, err
}

// CommitRecover 是找回的提交事务（F-ACC-04 规则 8，RS5-02、RS6-01）：
//  1. 第一条语句锁 accounts 行并递增版本，**不带版本条件**：攻击者循环改密让不了受害者的找回失败；
//  2. 原子检查"这一行恢复码没用过、没作废、没过期"；
//  3. 成功次数上限只约束最新一代（RS7-02）；
//  4. 只作废比所用代更新的代（RS4-01），吊销全部会话，发新的一代和新 sid，撤销注销申请（规则 5a）。
//
// 任一步不满足整个事务回滚，版本不变。
func (s *SQLite) CommitRecover(ctx context.Context, c RecoverCommit) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE accounts SET credentials_version = credentials_version + 1, password_hash = ?, credentials_changed_at = ?, delete_requested_at = 0
		WHERE user_id = ?`, c.PasswordHash, c.Now, c.UserID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrRecoveryCodeInvalid
	}
	res, err = tx.ExecContext(ctx, `UPDATE recovery_codes SET used_at = ?1
		WHERE user_id = ?2 AND gen = ?3 AND hash = ?4 AND used_at = 0 AND revoked_at = 0 AND (retire_until = 0 OR retire_until > ?1)`, c.Now, c.UserID, c.Gen, c.CodeHash)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrRecoveryCodeInvalid
	}
	var latest int64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(gen) FROM recovery_codes WHERE user_id = ?`, c.UserID).Scan(&latest); err != nil {
		return 0, err
	}
	var count int
	var windowStart int64
	err = tx.QueryRowContext(ctx, `SELECT count, window_start FROM auth_throttle WHERE throttle_key = ?`, c.LimitKey).Scan(&count, &windowStart)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if windowStart+c.LimitWindowMs <= c.Now {
		count = 0
	}
	if c.Gen == latest && count >= c.LimitMax {
		return 0, ErrRecoverLimited
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO auth_throttle (throttle_key, count, window_start, locked_until, expires_at) VALUES (?1, 1, ?2, 0, ?2 + ?3)
		ON CONFLICT (throttle_key) DO UPDATE SET
			count = CASE WHEN auth_throttle.window_start + ?3 <= ?2 THEN 1 ELSE auth_throttle.count + 1 END,
			window_start = CASE WHEN auth_throttle.window_start + ?3 <= ?2 THEN ?2 ELSE auth_throttle.window_start END,
			expires_at = ?2 + ?3`, c.LimitKey, c.Now, c.LimitWindowMs); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE recovery_codes SET revoked_at = ? WHERE user_id = ? AND gen > ? AND used_at = 0 AND revoked_at = 0`, c.Now, c.UserID, c.Gen); err != nil {
		return 0, err
	}
	gen := latest + 1
	if _, err := tx.ExecContext(ctx, `INSERT INTO recovery_codes (user_id, gen, hash, created_at) VALUES (?, ?, ?, ?)`, c.UserID, gen, c.NewRecoveryHash, c.Now); err != nil {
		return 0, err
	}
	if err := revokeUserSessions(ctx, tx, c.UserID, c.RevokeReason, c.Now); err != nil {
		return 0, err
	}
	if err := insertDeviceSession(ctx, tx, c.Session); err != nil {
		return 0, err
	}
	if err := insertAccountEvent(ctx, tx, c.Event); err != nil {
		return 0, err
	}
	if c.ThrottleKey != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_throttle WHERE throttle_key = ?`, c.ThrottleKey); err != nil {
			return 0, err
		}
	}
	if c.BeforeCommit != nil {
		if err := c.BeforeCommit(); err != nil {
			return 0, err
		}
	}
	return gen, tx.Commit()
}

// CancelDeletion 是匿名的 delete/cancel（F-ACC-08 规则 2）：第一条语句按版本和读到的申请时间锁行。
// 新申请的时间不同，所以在途的撤销不会撤掉 recover 之后的新申请（RS5-21）。
func (s *SQLite) CancelDeletion(ctx context.Context, userID string, version, requestedAt, now int64, event AccountEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE accounts SET delete_requested_at = 0 WHERE user_id = ? AND credentials_version = ? AND delete_requested_at = ?`, userID, version, requestedAt)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCredentialsChanged
	}
	if err := insertAccountEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}
