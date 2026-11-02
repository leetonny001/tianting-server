-- B4 会话（BE-040、BE-041）。
--
-- conv_seq 增加最后一条消息的摘要：和 seq 分配在同一行、同一个事务里更新，会话列表不用再查 messages。
ALTER TABLE conv_seq ADD COLUMN last_msg_id TEXT NOT NULL DEFAULT '';
ALTER TABLE conv_seq ADD COLUMN last_from   TEXT NOT NULL DEFAULT '';
ALTER TABLE conv_seq ADD COLUMN last_type   TEXT NOT NULL DEFAULT '';
ALTER TABLE conv_seq ADD COLUMN last_text   TEXT NOT NULL DEFAULT ''; -- 摘要，前 100 个码点
ALTER TABLE conv_seq ADD COLUMN last_ts     INTEGER NOT NULL DEFAULT 0;

-- user_conv：单聊会话出现在哪些用户的会话列表里（发过或收到过消息，PRD F-CONV-01 规则 2）。
-- 群会话直接来自 group_members，不在这里重复存一份。
CREATE TABLE IF NOT EXISTS user_conv (
	user_id    TEXT NOT NULL,
	conv_id    TEXT NOT NULL,
	peer_id    TEXT NOT NULL, -- 对方 userId（自己和自己聊时为自己）
	created_at INTEGER NOT NULL,
	PRIMARY KEY (user_id, conv_id)
);

-- 回填已有数据：最后一条消息。
UPDATE conv_seq SET
	last_msg_id = m.msg_id,
	last_from   = m.from_uid,
	last_type   = m.type,
	last_text   = substr(COALESCE(json_extract(m.body, '$.text'), ''), 1, 100),
	last_ts     = m.ts
FROM messages m
WHERE m.conv_id = conv_seq.conv_id AND m.seq = conv_seq.max_seq;

-- 回填已有数据：单聊双方。conv_id 形如 c2c:{a}:{b}。
INSERT OR IGNORE INTO user_conv (user_id, conv_id, peer_id, created_at)
SELECT a, conv_id, b, ts FROM (
	SELECT conv_id, min(ts) AS ts,
	       substr(conv_id, 5, instr(substr(conv_id, 5), ':') - 1) AS a,
	       substr(conv_id, 5 + instr(substr(conv_id, 5), ':')) AS b
	FROM messages WHERE conv_id LIKE 'c2c:%' GROUP BY conv_id
)
UNION
SELECT b, conv_id, a, ts FROM (
	SELECT conv_id, min(ts) AS ts,
	       substr(conv_id, 5, instr(substr(conv_id, 5), ':') - 1) AS a,
	       substr(conv_id, 5 + instr(substr(conv_id, 5), ':')) AS b
	FROM messages WHERE conv_id LIKE 'c2c:%' GROUP BY conv_id
);
