-- B3 群（BE-030、BE-032）：
-- history_visible：新成员能否看到入群前的历史（D2），查询时生效。
-- join_seq：成员入群时会话的 maxSeq；historyVisible=false 时只能看到 seq > join_seq 的消息。
--           被移除时删除整行，重新加入时按新的 maxSeq 计算（PRD F-GRP-01 规则 7）。
ALTER TABLE chat_groups ADD COLUMN history_visible INTEGER NOT NULL DEFAULT 0;
ALTER TABLE group_members ADD COLUMN join_seq INTEGER NOT NULL DEFAULT 0;
