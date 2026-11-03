# M3 发布与回滚手册（PRD-social 8.5 节，BE-100）

M3 增加迁移 `0006_social.sql`（社交关系表、`user_conv.sent_count`、`users.official`、`chat_groups.dissolved_at`、`group_members(user_id)` 索引）。**不支持新旧二进制混跑**：旧版本二进制遇到数据库里有它不认识的迁移会拒绝启动，也不会维护新的计数器。

## 升级（停机，分钟级）

1. 停止所有 im-server 节点（M3 是单节点）。
2. 备份数据库：`im-server backup -db im.db -o im-YYYYMMDD-HHMM.bak`（ARCH 3.4；备份文件不能已存在）。
3. 执行迁移：`im-server migrate -db im.db up`，确认输出里有 `6`；`im-server migrate -db im.db status` 没有 pending。
4. 需要预设好友模式时设置 `IM_SOCIAL_DEFAULTS`（例如 `{"mode":"friends"}`）。它只在库里还没有社交配置时写入一次，之后以数据库为准（改配置用 `PATCH /admin/settings/social`）。格式或范围非法时进程拒绝启动。
5. 启动新版本，检查 `/readyz`、`/metrics` 的命令错误码分布（不应出现大量 4035 / 4036 / 4137 / 4292）。

## 回滚 = 恢复升级前的备份

不提供 `migrate down`。回滚步骤：停止新版本 → `im-server restore -db im.db -i im-YYYYMMDD-HHMM.bak` → 启动旧版本。

**数据损失范围：升级之后的全部写入**（消息、关系、群、已读位置）。因此 M3 发布后先在演示环境运行 24 小时，再接入真实业务方。

## 计数器修复

`im-server repair -db im.db counters` 按基础表重算 `user_social.friend_count / block_count`、`chat_groups.member_count`、`user_conv.sent_count`，输出修复条数；幂等，可以在线执行。恢复备份之后、或发现计数漂移（例如好友上限判断异常）时运行；第二次运行应输出 0。
