-- PRD-social 0004（M3 社交关系，BE-100）。编号按合入顺序顺延为 0006：0004 / 0005 已被 X-5 审计、成员数占用（勘误 E-03）。
-- admin_audit 用 X-5 的表（勘误 E-04）；chat_groups.member_count 已在 0005 加好并回填。
-- 只加表、加列、加索引；不用触发器、不用自增主键；时间全是毫秒（ARCH 3.3、PRD-social 8.2 节）。

-- 业务方配置（5.1 节），key = 'social'，value 为 JSON。没有行时用默认值。
CREATE TABLE IF NOT EXISTS app_settings (
	key        TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);

-- 每用户的关系版本号、计数器；也是用户级锁对象（8.3 节）。行不存在视为全 0。
CREATE TABLE IF NOT EXISTS user_social (
	user_id          TEXT PRIMARY KEY,
	ver              INTEGER NOT NULL DEFAULT 0,
	purged_ver       INTEGER NOT NULL DEFAULT 0,
	request_seen_ver INTEGER NOT NULL DEFAULT 0,
	searchable       INTEGER,                    -- NULL = 跟随 searchableDefault
	friend_count     INTEGER NOT NULL DEFAULT 0,
	block_count      INTEGER NOT NULL DEFAULT 0,
	updated_at       INTEGER NOT NULL DEFAULT 0
);

-- 有向好友边，存在 owner 一侧；删除写墓碑。peer_id 不加外键（防枚举：可以对不存在的用户写行）。
CREATE TABLE IF NOT EXISTS friends (
	owner_id   TEXT NOT NULL,
	peer_id    TEXT NOT NULL,
	remark     TEXT NOT NULL DEFAULT '',
	source     TEXT NOT NULL DEFAULT 'other',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	deleted    INTEGER NOT NULL DEFAULT 0,
	ver        INTEGER NOT NULL,
	PRIMARY KEY (owner_id, peer_id)
);
CREATE INDEX IF NOT EXISTS friends_owner_ver ON friends (owner_id, ver, peer_id);
CREATE INDEX IF NOT EXISTS friends_purge ON friends (deleted, updated_at);

-- 好友申请：一条申请按双方各存一行，in 行是唯一真相。
CREATE TABLE IF NOT EXISTS friend_requests (
	owner_id      TEXT NOT NULL,
	peer_id       TEXT NOT NULL,
	direction     TEXT NOT NULL,              -- in | out
	status        TEXT NOT NULL,              -- pending | accepted | rejected（expired 读取时按 expire_at 计算）
	message       TEXT NOT NULL DEFAULT '',
	source        TEXT NOT NULL DEFAULT 'other',
	source_ref    TEXT NOT NULL DEFAULT '',
	preset_remark TEXT NOT NULL DEFAULT '',
	apply_count   INTEGER NOT NULL DEFAULT 0,
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL,
	expire_at     INTEGER NOT NULL,
	ver           INTEGER NOT NULL,
	PRIMARY KEY (owner_id, peer_id, direction)
);
CREATE INDEX IF NOT EXISTS friend_requests_owner_ver ON friend_requests (owner_id, ver, peer_id, direction);
CREATE INDEX IF NOT EXISTS friend_requests_updated ON friend_requests (updated_at);

-- 黑名单；删除写墓碑。
CREATE TABLE IF NOT EXISTS blocks (
	owner_id   TEXT NOT NULL,
	peer_id    TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	deleted    INTEGER NOT NULL DEFAULT 0,
	ver        INTEGER NOT NULL,
	PRIMARY KEY (owner_id, peer_id)
);
CREATE INDEX IF NOT EXISTS blocks_owner_ver ON blocks (owner_id, ver, peer_id);
CREATE INDEX IF NOT EXISTS blocks_purge ON blocks (deleted, updated_at);

-- 陌生人限额：我在这个单聊里发过几条（不封顶），SaveMessage 同一事务 +1。
ALTER TABLE user_conv ADD COLUMN sent_count INTEGER NOT NULL DEFAULT 0;
UPDATE user_conv SET sent_count = (
	SELECT count(*) FROM messages m WHERE m.conv_id = user_conv.conv_id AND m.from_uid = user_conv.user_id
);

-- 官方账号（F-SOC-13）。
ALTER TABLE users ADD COLUMN official INTEGER NOT NULL DEFAULT 0;

-- 解散时间（从 PRD-social 0005 提前到 M3，解散群 BE-122 需要）；0 = 未解散。
ALTER TABLE chat_groups ADD COLUMN dissolved_at INTEGER NOT NULL DEFAULT 0;

-- 现状缺失：ListConversations 的 gm.user_id = ?、myGroups、同群判断都依赖它。
CREATE INDEX IF NOT EXISTS group_members_user ON group_members (user_id);

-- 按成员表重算一次成员数（0005 已回填；这里保证在只有 0003 数据的库上同样正确，AC-SOC-11-5）。
UPDATE chat_groups SET member_count = (SELECT count(*) FROM group_members gm WHERE gm.group_id = chat_groups.group_id);
