-- 正文查看授权（PRD-admin F-ADM-08、F-ADM-32，BE-225）。配额按 created_at 统计 Asia/Shanghai 自然日。
CREATE TABLE IF NOT EXISTS msg_grants (
	grant_id   TEXT PRIMARY KEY,
	caller     TEXT NOT NULL,
	operator   TEXT NOT NULL,
	conv_id    TEXT NOT NULL,
	quarantine INTEGER NOT NULL,
	reason     TEXT NOT NULL,
	ticket     TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS msg_grants_operator ON msg_grants (operator, created_at);
CREATE INDEX IF NOT EXISTS msg_grants_caller ON msg_grants (caller, created_at);
