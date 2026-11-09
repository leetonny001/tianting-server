-- C2 图片和文件（PRD-chat 第 12.1 节，BE-431）。只加新表和有默认值的列；时间是毫秒；SQL 兼容 SQLite 和 Postgres。
-- 一个 file_id 两个变体（original、thumb），没有 sha256。状态见 F-FILE-06 规则 2：
-- pending → ready → attached → expired（对象已删，行保留给消息显示"已过期"）；attached → revoked（后台删除，30 天后删对象）；
-- pending / ready 超过 24 小时未绑定、取消、换头像 7 天后、注销清理 → purging（删公开对象和 final/{fileId}/ 前缀之后删行）。
CREATE TABLE IF NOT EXISTS files (
	file_id             TEXT PRIMARY KEY,
	owner               TEXT NOT NULL,
	kind                TEXT NOT NULL,               -- image / file / avatar
	state               TEXT NOT NULL,
	name                TEXT NOT NULL DEFAULT '',    -- 清洗后的文件名，按正文对待；过期、撤回、后台删除时清空（R-CS-23）
	declared_size       INTEGER NOT NULL,
	declared_mime       TEXT NOT NULL,
	thumb_declared_size INTEGER NOT NULL DEFAULT 0,
	thumb_declared_mime TEXT NOT NULL DEFAULT '',
	tmp_key             TEXT NOT NULL,
	tmp_thumb_key       TEXT NOT NULL DEFAULT '',
	object_key          TEXT NOT NULL DEFAULT '',
	thumb_key           TEXT NOT NULL DEFAULT '',
	size                INTEGER NOT NULL DEFAULT 0,  -- complete 时读到的真实值
	mime                TEXT NOT NULL DEFAULT '',
	thumb_size          INTEGER NOT NULL DEFAULT 0,
	thumb_mime          TEXT NOT NULL DEFAULT '',
	width               INTEGER NOT NULL DEFAULT 0,
	height              INTEGER NOT NULL DEFAULT 0,
	completing_until    INTEGER,                     -- complete 的租约（F-FILE-01 规则 3），NULL 表示没有租约
	completing_attempt  TEXT NOT NULL DEFAULT '',
	public_key          TEXT NOT NULL DEFAULT '',    -- 头像在公开桶的键：先写这里再 Copy（RS6-04）
	avatar_replaced_at  INTEGER NOT NULL DEFAULT 0,
	msg_id              TEXT NOT NULL DEFAULT '',
	conv_id             TEXT NOT NULL DEFAULT '',
	retain_until        INTEGER NOT NULL DEFAULT 0,  -- 0 表示不过期（头像、IM_FILE_RETENTION_DAYS=0）
	evidence_until      INTEGER NOT NULL DEFAULT 0,  -- 举报证据（C6），先建列
	scan_state          TEXT NOT NULL DEFAULT 'skipped', -- 第一批不做病毒扫描（F-FILE-05 规则 2）
	put_expires_at      INTEGER NOT NULL,            -- 预签名 PUT 的到期时间（真实时钟）：未完成数只统计没过期的（规则 6a）
	created_at          INTEGER NOT NULL,
	ready_at            INTEGER NOT NULL DEFAULT 0,
	attached_at         INTEGER NOT NULL DEFAULT 0,
	revoked_at          INTEGER NOT NULL DEFAULT 0,
	objects_deleted     INTEGER NOT NULL DEFAULT 0   -- expired、revoked 的对象已经删除
);
CREATE INDEX IF NOT EXISTS files_owner_state ON files (owner, state, put_expires_at);
CREATE INDEX IF NOT EXISTS files_state_created ON files (state, created_at);
CREATE INDEX IF NOT EXISTS files_state_retain ON files (state, retain_until);
CREATE INDEX IF NOT EXISTS files_msg ON files (msg_id);

-- 每人每天（Asia/Shanghai 自然日）的上传量：bytes 是 complete 成功时按实际大小扣的，declared 是申请时声明的合计（规则 6、6a）。
CREATE TABLE IF NOT EXISTS upload_quota (
	user_id  TEXT NOT NULL,
	day      TEXT NOT NULL,
	bytes    INTEGER NOT NULL DEFAULT 0,
	declared INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (user_id, day)
);

-- 会话内已有消息的展示内容版本（F-RCL-03 规则 2，C2 起：文件过期时递增）。
ALTER TABLE conv_seq ADD COLUMN mod_ver INTEGER NOT NULL DEFAULT 0;
