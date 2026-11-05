-- 删除违规消息与加密隔离（PRD-admin F-ADM-31、F-ADM-32，BE-223、BE-224）。
-- 软删除：保留 seq、msgId、发送者、时间、类型，body 清空成 '{}'，seq 不留空洞。
ALTER TABLE messages ADD COLUMN deleted_at INTEGER NOT NULL DEFAULT 0; -- 毫秒；0 = 没有删除
ALTER TABLE messages ADD COLUMN deleted_by TEXT NOT NULL DEFAULT '';   -- {caller}:{operator}
ALTER TABLE conv_seq ADD COLUMN last_deleted INTEGER NOT NULL DEFAULT 0;

-- 原文只以密文保存，AES-256-GCM，AAD = msgId|convId。到期由定时任务物理删除。
CREATE TABLE IF NOT EXISTS message_quarantine (
	msg_id     TEXT PRIMARY KEY,
	conv_id    TEXT NOT NULL,
	nonce      BLOB NOT NULL,
	ciphertext BLOB NOT NULL,
	kid        TEXT NOT NULL,
	deleted_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS message_quarantine_expires ON message_quarantine (expires_at);
CREATE INDEX IF NOT EXISTS message_quarantine_conv ON message_quarantine (conv_id);
