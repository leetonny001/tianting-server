-- C4 撤回、引用、@（PRD-chat 第 12.1 节，BE-471）。只加有默认值的列和新表；时间是毫秒；SQL 兼容 SQLite 和 Postgres。
-- 迁移号：C3 的 0018_device_push.sql 没有提交（C3 暂停），C4 先用 0018（backend-chat 第 52 条）。

-- 撤回（F-RCL-01 规则 5）：和后台删除的 deleted_at 分开，正文同样清空成 '{}'，seq 不变。recalled_by 是操作者的 userId。
ALTER TABLE messages ADD COLUMN recalled_at INTEGER NOT NULL DEFAULT 0; -- 毫秒；0 = 没有撤回
ALTER TABLE messages ADD COLUMN recalled_by TEXT NOT NULL DEFAULT '';
-- 引用（F-QUO-01 规则 3）：只保存被引用的 msgId，不把内容复制进 body，下发时组装。
ALTER TABLE messages ADD COLUMN quote_msg_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS messages_quote ON messages (quote_msg_id);

-- 会话最后一条被撤回（F-RCL-03 规则 4）：摘要清空，lastMsg 带 deleted、recalled、recalledBy。
ALTER TABLE conv_seq ADD COLUMN last_recalled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE conv_seq ADD COLUMN last_recalled_by TEXT NOT NULL DEFAULT '';

-- 提及计数（F-MEN-03 规则 1）：只记 @ 个人，自己 @ 自己不写；和消息同一事务。正文里的 mentions 另存在 body（下发要带），
-- 这张表只用于 mentionUnread，撤回、后台删除时删行，已读之前的由 mention_purge 定时清理。
CREATE TABLE IF NOT EXISTS conv_mentions (
	user_id TEXT NOT NULL,
	conv_id TEXT NOT NULL,
	seq     INTEGER NOT NULL,
	PRIMARY KEY (user_id, conv_id, seq)
);
CREATE INDEX IF NOT EXISTS conv_mentions_conv_seq ON conv_mentions (conv_id, seq);

-- @ 所有人（F-MEN-02、AC-MEN-02-6）：一条消息一行，不按成员展开。from_uid 用来排除发的人自己（backend-chat 第 60 条）。
CREATE TABLE IF NOT EXISTS group_mention_all (
	group_id TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	ts       INTEGER NOT NULL,
	from_uid TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (group_id, seq)
);

-- @ 所有人的次数（F-MEN-02 规则 2，R-CB-07）：单独计数，撤回不退，所以不靠 group_mention_all（PRD 第 12.1 节）。
-- win='hour' 时 bucket 是成功那一秒的 Unix 秒（滚动 60 分钟，backend-chat 第 58 条）；win='day' 时 bucket 是 Asia/Shanghai 日期 yyyymmdd。
-- 在保存消息的事务里扣（第 4.3 节第 10 步），加锁顺序 conv_seq → 文件行 → 计数行。
CREATE TABLE IF NOT EXISTS mention_all_quota (
	group_id TEXT NOT NULL,
	win      TEXT NOT NULL,
	bucket   INTEGER NOT NULL,
	count    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (group_id, win, bucket)
);
