-- X-5 管理接口审计（PRD-admin F-KEY-04，BE-203）。
--
-- 每次写接口（以及 user.sessions、msg.meta、正文读取、audit.read）在执行前写一条 intent，执行后写一条 result（同一 request_id）。
-- 只追加：Store 接口只有追加和查询方法，没有更新 / 删除（SQLite 阶段在应用层保证；迁 Postgres 后用数据库权限）。
-- 过期清理（保留 1 年）由定时任务通过单独的窄接口执行（BE-220）。
CREATE TABLE IF NOT EXISTS admin_audit (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	ts           INTEGER NOT NULL,              -- 毫秒，[IM 钟]
	caller       TEXT NOT NULL,                 -- 调用方 id；旧密钥为 legacy
	operator     TEXT NOT NULL DEFAULT '',      -- 签名过的 X-IM-Operator；旧密钥为空
	request_id   TEXT NOT NULL DEFAULT '',
	route        TEXT NOT NULL,                 -- "METHOD /admin/路由模式"
	targets_json TEXT NOT NULL DEFAULT '{}',    -- {"ids":[前 50 个],"count":总数}
	summary_json TEXT NOT NULL DEFAULT '{}',    -- 脱敏摘要：只有数量、旧值 → 新值，不记正文 / 备注 / 推送原文
	phase        TEXT NOT NULL,                 -- intent | result
	code         INTEGER NOT NULL DEFAULT 0     -- result 的业务结果码（0 成功）；intent 为 0
);
CREATE INDEX IF NOT EXISTS admin_audit_ts ON admin_audit (ts);
CREATE INDEX IF NOT EXISTS admin_audit_request ON admin_audit (request_id);
CREATE INDEX IF NOT EXISTS admin_audit_caller_ts ON admin_audit (caller, ts);
