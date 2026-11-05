-- 禁言（PRD-admin F-ADM-40、F-ADM-41，BE-226、BE-227）。scope = user 时 group_id 为空字符串。
-- 到期不删行，发送时按 [IM 钟] 判断；移出群、退群、解散时在同一事务删除该群的记录。
CREATE TABLE IF NOT EXISTS mutes (
	scope    TEXT NOT NULL,
	user_id  TEXT NOT NULL,
	group_id TEXT NOT NULL DEFAULT '',
	until    INTEGER,                 -- 毫秒；NULL = 永久
	reason   TEXT NOT NULL,
	by_who   TEXT NOT NULL,
	at       INTEGER NOT NULL,
	PRIMARY KEY (scope, user_id, group_id)
);
CREATE INDEX IF NOT EXISTS mutes_group ON mutes (group_id, user_id);
