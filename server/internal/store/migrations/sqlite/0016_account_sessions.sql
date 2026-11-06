-- 账号第二段（PRD-chat F-ACC-03 规则 3、4，F-ACC-08 规则 3）。只加有默认值的列；时间是毫秒。
-- 签发频率存在会话行上，和轮换用同一个条件更新：不会因为两次写之间的并发多发 token（RS6-05、RS5-14）。
ALTER TABLE device_sessions ADD COLUMN issue_window_start INTEGER NOT NULL DEFAULT 0;
ALTER TABLE device_sessions ADD COLUMN issue_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE device_sessions ADD COLUMN limited_window INTEGER NOT NULL DEFAULT 0;
ALTER TABLE device_sessions ADD COLUMN limited_streak INTEGER NOT NULL DEFAULT 0;

-- account_cleanup 第 ② 步做完的时间：0 表示还要继续（第 ② 步幂等，可以重试）。
ALTER TABLE accounts ADD COLUMN cleanup_done_at INTEGER NOT NULL DEFAULT 0;

-- refresh 密钥台账（RS5-04）：退役条件是 IM_REFRESH_ABSOLUTE，配置检查据此拦截提前删除。只有 kid 和时间，没有密钥。
-- retired_at = 0 表示还是当前密钥；forgotten_at 是运维明确放弃这把密钥（泄露应急）的时间。
CREATE TABLE IF NOT EXISTS refresh_key_ledger (
	kid          TEXT PRIMARY KEY,
	added_at     INTEGER NOT NULL,
	retired_at   INTEGER NOT NULL DEFAULT 0,
	forgotten_at INTEGER NOT NULL DEFAULT 0
);
