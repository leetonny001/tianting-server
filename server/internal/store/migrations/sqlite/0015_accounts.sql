-- 账号（PRD-chat 第 12.1 节，BE-410）。只加有默认值的列；时间是毫秒；不用触发器；SQL 和 Postgres 兼容。
-- users 不加唯一约束（R-CB-01）：C-18（userId 区分大小写）和演示、导入路径不变。lower(user_id) 的索引只给注册查重用。
ALTER TABLE users ADD COLUMN deleted_at INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS users_user_id_lower ON users (lower(user_id));

-- 演示和导入用户没有行（F-ACC-07 规则 2）。delete_requested_at = 0 表示没有申请注销。
CREATE TABLE IF NOT EXISTS accounts (
	user_id                TEXT PRIMARY KEY,
	username_lower         TEXT NOT NULL UNIQUE,
	password_hash          TEXT NOT NULL,
	credentials_version    INTEGER NOT NULL DEFAULT 1,
	credentials_changed_at INTEGER NOT NULL DEFAULT 0,
	delete_requested_at    INTEGER NOT NULL DEFAULT 0,
	notice_version         TEXT NOT NULL DEFAULT '',
	created_at             INTEGER NOT NULL,
	last_login_at          INTEGER NOT NULL DEFAULT 0
);

-- 每代一行（F-ACC-05 规则 1）。retire_until 换代时写一次，之后不延长；0 表示还没有换代。revoked_at 记录被作废。
CREATE TABLE IF NOT EXISTS recovery_codes (
	user_id      TEXT NOT NULL,
	gen          INTEGER NOT NULL,
	hash         TEXT NOT NULL,
	created_at   INTEGER NOT NULL,
	acked_at     INTEGER NOT NULL DEFAULT 0,
	retire_until INTEGER NOT NULL DEFAULT 0,
	used_at      INTEGER NOT NULL DEFAULT 0,
	revoked_at   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id, gen)
);
CREATE INDEX IF NOT EXISTS recovery_codes_hash ON recovery_codes (user_id, hash);

-- 失败和频率计数（F-ACC-02 规则 3，R-CB-16）：存数据库，重启不清零，多节点一致。键里只有哈希和 IP 前缀（规则 3a）。
CREATE TABLE IF NOT EXISTS auth_throttle (
	throttle_key TEXT PRIMARY KEY,
	count        INTEGER NOT NULL,
	window_start INTEGER NOT NULL,
	locked_until INTEGER NOT NULL DEFAULT 0,
	expires_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS auth_throttle_expires ON auth_throttle (expires_at);

-- 保留 180 天（account_events_purge）。不存 sid 原值（F-ACC-03 规则 1）。
CREATE TABLE IF NOT EXISTS account_events (
	event_id  TEXT PRIMARY KEY,
	user_id   TEXT NOT NULL,
	kind      TEXT NOT NULL,
	at        INTEGER NOT NULL,
	platform  TEXT NOT NULL DEFAULT '',
	ip_prefix TEXT NOT NULL DEFAULT '',
	sid_hash  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS account_events_user ON account_events (user_id, at);
CREATE INDEX IF NOT EXISTS account_events_at ON account_events (at);

-- 设备会话（F-ACC-03 规则 1）。代号由 next_gen 分配，严格单调，永不复用。pendings 和 seen_ip_prefixes 是 JSON 数组。
-- 只存 refresh token 的哈希。转正、轮换用条件更新 WHERE sid=? AND row_version=? AND revoked_at=0（BE-415）。
CREATE TABLE IF NOT EXISTS device_sessions (
	sid                 TEXT PRIMARY KEY,
	user_id             TEXT NOT NULL,
	platform            TEXT NOT NULL,
	device_name         TEXT NOT NULL DEFAULT '',
	current_hash        TEXT NOT NULL,
	current_gen         INTEGER NOT NULL,
	next_gen            INTEGER NOT NULL,
	pendings            TEXT NOT NULL DEFAULT '[]',
	overlap_until       INTEGER NOT NULL DEFAULT 0,
	overlap_min_gen     INTEGER NOT NULL DEFAULT 0,
	row_version         INTEGER NOT NULL DEFAULT 1,
	seen_ip_prefixes    TEXT NOT NULL DEFAULT '[]',
	created_at          INTEGER NOT NULL,
	last_seen_at        INTEGER NOT NULL,
	expires_at          INTEGER NOT NULL,
	absolute_expires_at INTEGER NOT NULL,
	revoked_at          INTEGER NOT NULL DEFAULT 0,
	revoke_reason       TEXT NOT NULL DEFAULT '',
	show_detail         INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS device_sessions_user ON device_sessions (user_id);
CREATE INDEX IF NOT EXISTS device_sessions_expires ON device_sessions (expires_at);

-- 邀请码（F-ACC-09）：只存哈希和后 4 位。mismatch_count 按 24 小时窗口计（RC2-31）。
CREATE TABLE IF NOT EXISTS invites (
	invite_id             TEXT PRIMARY KEY,
	code_hash             TEXT NOT NULL UNIQUE,
	code_last4            TEXT NOT NULL,
	max_uses              INTEGER NOT NULL,
	used_count            INTEGER NOT NULL DEFAULT 0,
	expires_at            INTEGER NOT NULL,
	note                  TEXT NOT NULL DEFAULT '',
	created_by            TEXT NOT NULL DEFAULT '',
	created_at            INTEGER NOT NULL,
	disabled_at           INTEGER NOT NULL DEFAULT 0,
	auto_disabled         INTEGER NOT NULL DEFAULT 0,
	mismatch_count        INTEGER NOT NULL DEFAULT 0,
	mismatch_window_start INTEGER NOT NULL DEFAULT 0,
	exhausted_at          INTEGER NOT NULL DEFAULT 0
);
