-- C6 敏感词和举报（PRD-chat 第 12.1 节，BE-492）。只加新表；时间是毫秒（BIGINT）；SQL 兼容 SQLite 和 Postgres。
-- 迁移号：C3 的 BE-451 恢复时用下一个空号（backend-chat 第 52、76 条）。

-- 词库（F-MOD-01）。word 是原文（后台按纯文本显示）；norm、compact 是标准化后的两份（规则 3，backend-chat 第 101 条），
-- 只用于匹配和去重。按 norm 去重（第 100 条）。state 是 active / disabled，停用的词不进自动机。
CREATE TABLE IF NOT EXISTS blocked_words (
	word_id    BIGINT PRIMARY KEY,
	word       TEXT NOT NULL,
	norm       TEXT NOT NULL,
	compact    TEXT NOT NULL,
	state      TEXT NOT NULL DEFAULT 'active',
	created_at BIGINT NOT NULL,
	updated_at BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS blocked_words_norm ON blocked_words (norm);

-- 命中记录（F-MOD-02 规则 7）：不存消息全文；word_id 在删词后保留（不级联）。保留 IM_MOD_HITS_DAYS（90 天），moderation_hits_purge 清理。
-- scene 是 text、fileName、nickname、username、groupName、groupNotice、friendApply（第 110 条）。注册时还没有用户，不写行（第 113 条）。
CREATE TABLE IF NOT EXISTS moderation_hits (
	user_id TEXT NOT NULL,
	scene   TEXT NOT NULL,
	word_id BIGINT NOT NULL,
	at      BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS moderation_hits_user_at ON moderation_hits (user_id, at);
CREATE INDEX IF NOT EXISTS moderation_hits_at ON moderation_hits (at);

-- 举报（F-MOD-03、04）。只有 ID、类型、理由、状态等元数据；note、文件名、正文只在加密的 report_evidence 里（第 122 条）。
-- msg_ids 是排序后的 JSON 数组；dedup_key 是幂等键（第 117 条：消息是 convId + 排序后的 msgIds，用户是 userId，群是 groupId）。
-- target_user：消息的发送者或被举报的用户（群举报为空），"该用户被举报的总次数"按它算。
-- state 是 pending / handled / rejected；conclusion、handled_by、handled_at 是结案时写的（规则 5）。
-- 和证据一起在 IM_REPORT_EVIDENCE_DAYS（180 天）后整条删除（P-48 ②，第 119 条）。
CREATE TABLE IF NOT EXISTS reports (
	report_id   TEXT PRIMARY KEY,
	reporter    TEXT NOT NULL,
	target_type TEXT NOT NULL,
	conv_id     TEXT NOT NULL DEFAULT '',
	target_user TEXT NOT NULL DEFAULT '',
	group_id    TEXT NOT NULL DEFAULT '',
	msg_ids     TEXT NOT NULL DEFAULT '[]',
	reason      TEXT NOT NULL,
	dedup_key   TEXT NOT NULL,
	state       TEXT NOT NULL DEFAULT 'pending',
	conclusion  TEXT NOT NULL DEFAULT '',
	handled_by  TEXT NOT NULL DEFAULT '',
	handled_at  BIGINT NOT NULL DEFAULT 0,
	created_at  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS reports_state_created ON reports (state, created_at);
CREATE INDEX IF NOT EXISTS reports_dedup ON reports (reporter, dedup_key, created_at);
CREATE INDEX IF NOT EXISTS reports_target_user ON reports (target_user);

-- 证据快照（F-MOD-03 规则 5）：AES-256-GCM（IM_REPORT_KEY），AAD 绑定 report_id 和 idx。idx 0 是举报的附加信息（note 等），
-- 1.. 是被举报的消息，顺序同 msg_ids。和 reports 行同一事务写入；expires_at 到了由 report_purge 物理删除（规则 6）。
CREATE TABLE IF NOT EXISTS report_evidence (
	report_id  TEXT NOT NULL,
	idx        INTEGER NOT NULL,
	nonce      BLOB NOT NULL,
	ciphertext BLOB NOT NULL,
	kid        TEXT NOT NULL,
	expires_at BIGINT NOT NULL,
	PRIMARY KEY (report_id, idx)
);
CREATE INDEX IF NOT EXISTS report_evidence_expires ON report_evidence (expires_at);

-- 每人每天的举报次数（F-MOD-03 规则 4，第 115 条）：通过格式和开关之后的请求都算，包括 4040；幂等的重复、写入失败的不算。
-- day 是 Asia/Shanghai 当天 00:00 的毫秒。report_purge 顺带清掉两天前的行。
CREATE TABLE IF NOT EXISTS report_quota (
	user_id TEXT NOT NULL,
	day     BIGINT NOT NULL,
	count   INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id, day)
);
