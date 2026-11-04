-- PRD-social 0005（M4 群角色，BE-130）。编号按合入顺序顺延为 0007（勘误 E-26）；dissolved_at 已在 0006。
-- ug- 前缀保留给自建群：执行前由 migrate.go 的预检确认已有群 ID 没有占用（8.5 节第 4 条）。

-- owner_id：群主，空 = 无群主。created_by：建群者，空 = 托管群（业务方建的）。max_members：自建群建群时固定。
ALTER TABLE chat_groups ADD COLUMN owner_id    TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN created_by  TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN notice      TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN notice_by   TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_groups ADD COLUMN notice_at   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chat_groups ADD COLUMN max_members INTEGER NOT NULL DEFAULT 2000;

-- role：owner / admin / member。群主同时记在 chat_groups.owner_id 和成员行上，两处在同一事务更新。
ALTER TABLE group_members ADD COLUMN role    TEXT NOT NULL DEFAULT 'member';
ALTER TABLE group_members ADD COLUMN role_at INTEGER NOT NULL DEFAULT 0;

-- 建群去重：只记成功的建群，7 天后清理。
CREATE TABLE IF NOT EXISTS client_group_req (
	user_id       TEXT NOT NULL,
	client_req_id TEXT NOT NULL,
	group_id      TEXT NOT NULL,
	created_at    INTEGER NOT NULL,
	PRIMARY KEY (user_id, client_req_id)
);
CREATE INDEX IF NOT EXISTS client_group_req_created ON client_group_req (created_at);
