-- C5 会话置顶、免打扰、删除（PRD-chat 第 12.1 节，BE-483）。只加新表；时间是毫秒；SQL 兼容 SQLite 和 Postgres。
-- 迁移号：C3 的迁移没有提交（C3 暂停），C4 用了 0018，C5 用 0019；C3 恢复时用下一个空号（backend-chat 第 76 条）。

-- 只有设置过的会话才有行。pinned_at > 0 表示置顶（F-CVS-01）；notify_off 是免打扰（F-CVS-02）；
-- deleted_at、cleared_seq 是删除会话（F-CVS-03 规则 2）：cleared_seq 只增不减，隐藏条件 deleted_at > 0 AND max_seq <= cleared_seq。
-- 退群、被移出、解散时在同一事务里删除这一行（F-CVS-01 规则 6、F-CVS-03 规则 6a）。
CREATE TABLE IF NOT EXISTS conv_settings (
	user_id     TEXT NOT NULL,
	conv_id     TEXT NOT NULL,
	pinned_at   INTEGER NOT NULL DEFAULT 0,
	notify_off  INTEGER NOT NULL DEFAULT 0,
	deleted_at  INTEGER NOT NULL DEFAULT 0,
	cleared_seq INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id, conv_id)
);
CREATE INDEX IF NOT EXISTS conv_settings_pinned ON conv_settings (user_id, pinned_at);
CREATE INDEX IF NOT EXISTS conv_settings_deleted ON conv_settings (user_id, deleted_at);
