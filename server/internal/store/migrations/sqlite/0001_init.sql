-- 一期初始表结构（B0～B2）。用 IF NOT EXISTS：迁移机制上线前创建的开发库也能直接升级。
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
