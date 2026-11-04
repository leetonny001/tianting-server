-- PRD-admin 第一批（BE-205、BE-207、BE-210、BE-211）：管理后台的搜索与元数据查询索引。
-- nickname_fold / name_fold 是 ASCII 小写化后的值（SQLite lower() 只处理 ASCII，其他字符按码点精确，PRD-admin 2.3 节）。
-- 用虚拟生成列：所有写路径不用改，也不会漂移。
ALTER TABLE users ADD COLUMN nickname_fold TEXT GENERATED ALWAYS AS (lower(nickname)) VIRTUAL;
CREATE INDEX idx_users_nickname_fold ON users (nickname_fold, user_id);
ALTER TABLE chat_groups ADD COLUMN name_fold TEXT GENERATED ALWAYS AS (lower(name)) VIRTUAL;
CREATE INDEX idx_groups_name_fold ON chat_groups (name_fold);
CREATE INDEX idx_groups_created ON chat_groups (created_at, group_id);
CREATE INDEX idx_messages_conv_from ON messages (conv_id, from_uid, seq);
CREATE INDEX idx_messages_conv_ts ON messages (conv_id, ts);
