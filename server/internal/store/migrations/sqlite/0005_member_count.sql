-- 群成员数（PRD-social BE-113 第 3 条、PRD-core V-07、PRD-ui G-UI-02）：会话列表的群会话和群查询返回 memberCount。
-- 加 / 减成员在同一个事务里更新，读的时候不用 count(*)。
ALTER TABLE chat_groups ADD COLUMN member_count INTEGER NOT NULL DEFAULT 0;
UPDATE chat_groups SET member_count = (SELECT count(*) FROM group_members gm WHERE gm.group_id = chat_groups.group_id);
