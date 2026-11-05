-- 封禁（PRD-admin F-ADM-13，BE-222）。until 为 NULL 表示永久；到期不删行，签发时按 [IM 钟] 判断。
CREATE TABLE IF NOT EXISTS user_bans (
	user_id TEXT PRIMARY KEY,
	until   INTEGER,                -- 毫秒；NULL = 永久
	reason  TEXT NOT NULL,
	by_who  TEXT NOT NULL,          -- {caller}:{operator}
	at      INTEGER NOT NULL
);
