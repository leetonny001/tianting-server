-- 数据概览（PRD-admin G-ADM-17，BE-241）：首页每 60 秒查一次"今日新增用户"和"今日消息数"，不能每次全表扫描。
-- 消息索引带 type 是覆盖索引：统计要排除群提示消息，不用回表。
CREATE INDEX IF NOT EXISTS idx_users_created ON users (created_at);
CREATE INDEX IF NOT EXISTS idx_messages_ts_type ON messages (ts, type);
