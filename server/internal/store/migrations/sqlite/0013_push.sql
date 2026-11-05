-- 系统推送（PRD-live v1.0 4.3、4.4 节，BE-228、BE-229）。
-- push_seq 不随清理删除：序号不回退，latestSeq 从这张表读。不复用 conv_seq（它带会话语义）。
CREATE TABLE IF NOT EXISTS push_seq (
	user_id TEXT PRIMARY KEY,
	max_seq INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS push_inbox (
	user_id    TEXT NOT NULL,
	push_seq   INTEGER NOT NULL,
	push_id    TEXT NOT NULL,
	type       TEXT NOT NULL,
	data       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, push_seq),
	UNIQUE (push_id, user_id) -- 既保证 (user_id, push_id) 唯一，也是 4.4 节幂等清理要的 push_id 索引，不多写一棵 B 树（AC-PUSH-04-4）
);
CREATE INDEX IF NOT EXISTS push_inbox_expires ON push_inbox (expires_at);
-- state：pending（all、ttlSeconds=0 已写记录、还没广播完）| done。accepted 只有 target=user 才有。
CREATE TABLE IF NOT EXISTS push_idempotency (
	push_id      TEXT PRIMARY KEY,
	caller       TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	target       TEXT NOT NULL,
	accepted     INTEGER,
	state        TEXT NOT NULL,
	first_ts     INTEGER NOT NULL,
	created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS push_idempotency_created ON push_idempotency (created_at);
