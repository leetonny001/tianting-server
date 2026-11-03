# 管理接口（给业务服务端）

所有接口都需要请求头 `X-Admin-Key`；请求和响应都是 JSON。错误统一返回 `{"code": 4xxx, "msg": "..."}`，HTTP 状态码由错误码前三位决定（见 [ERRORS.md](ERRORS.md)）。
管理密钥只能放在业务服务端，**不能下发到任何客户端**。

## 用户与凭证

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `POST /admin/users/token` | `{userId, ttlSeconds?}` | `{userId, token, expiresAt}` | 签发 token（对应腾讯 `userSig`），用户不存在时自动注册。有效期默认 7 天，范围 60 秒～30 天 |
| `POST /admin/users/import` | `{users:[{userId, nickname?, avatar?}]}` | `{imported}` | 批量导入 / 更新，单次 ≤ 500；任何一项不合法整批不写入 |
| `GET /admin/users/{userId}` | — | `{userId, nickname, avatar, createdAt}` | 不存在：404 / 4042 |
| `POST /admin/users/{userId}/force-logout` | `{}` | `{}` | 踢下线全部连接，此前签发的 token 全部作废；重新签发即可解封 |

## 群

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `POST /admin/groups` | `{groupId, name, members?, historyVisible?}` | 201 `{}` | 建群。初始成员 ≤ 2000（不受 500 批量限制） |
| `GET /admin/groups/{groupId}` | — | `{groupId, name, historyVisible, createdAt, members}` | 查询群 |
| `PATCH /admin/groups/{groupId}` | `{name?, historyVisible?}` | `{}` | 修改设置；至少传一项。`historyVisible` 在查询时生效 |
| `POST /admin/groups/{groupId}/members` | `{members}` | `{added}` | 加成员，单次 ≤ 500；已经是成员的忽略（不重置入群位置） |
| `DELETE /admin/groups/{groupId}/members` | `{members}` | `{removed}` | 移除成员，单次 ≤ 500；不是成员的忽略 |

规则：
- 群名去首尾空白后 1～50 个字符（按 Unicode 码点）；群 ID、userId 为字母、数字、`_`、`-`，1～64 位。
- 成员中有不存在的用户：404 / 4042，**整批不生效**。同一请求里重复的 userId 按 1 个计。
- 多个错误同时出现时的优先级：ID 格式 4004 → 其他参数 4000 → 群不存在 4043（建群为已存在 4090）→ 单次超过 500 人 4131 → 用户不存在 4042 → 总人数超过 2000 4130。
- `historyVisible=false`（默认）：新成员只能看到入群之后的消息；`true`：可以看到全部历史。被移除后再加入，按重新加入的时刻计算。

## 运维

| 接口 | 说明 |
|---|---|
| `GET /healthz` | 进程存活 |
| `GET /readyz` | 可以接收流量（数据库可用、没有在下线）；不可用时 503 |
| `GET /metrics` | Prometheus 指标。**只在单独的内网地址上提供**（`IM_METRICS_ADDR`，默认 `127.0.0.1:9090`，`off` 关闭），对外端口上没有：在线连接数（按平台）、握手结果、上行命令数和错误码分布、命令处理耗时、新消息数、关闭码分布（按关闭方）、被踢原因、慢连接数，以及 Go 运行时和进程指标 |

备份、恢复、数据库迁移通过命令行完成（`im-server backup / restore / migrate`），不提供 HTTP 接口。

## 仅测试 / 演示环境

`POST /admin/test/drop-ack`、`/admin/test/skip-delivery`、`/admin/test/clock`（`IM_TEST_MODE=1`），`POST /demo/login`（`IM_DEMO_LOGIN=1`，不需要管理密钥）。生产环境（`IM_ENV=production`）开启这两个开关会拒绝启动。
