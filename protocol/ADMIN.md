# 管理接口（给业务服务端）

管理接口**只在单独的管理端口上提供**（`IM_ADMIN_ADDR`，默认 `127.0.0.1:8081`，`off` 关闭）；对外端口（`IM_ADDR`）上请求 `/admin/*` 返回 404。生产环境管理地址不是回环地址时必须配置 TLS（`IM_ADMIN_TLS_CERT` / `IM_ADMIN_TLS_KEY`），可以用 `IM_ADMIN_ALLOWED_CIDRS` 限制来源网段（只看 TCP 对端地址，不解析 `X-Forwarded-For`），见 PRD-admin F-KEY-01。

请求和响应都是 JSON。错误统一返回 `{"code": 4xxx, "msg": "..."}`，HTTP 状态码由错误码前三位决定（见 [ERRORS.md](ERRORS.md)）。
密钥只能放在业务服务端 / console-server，**不能下发到任何客户端或浏览器**。

## 鉴权：调用方签名（PRD-admin F-KEY-02、F-KEY-03）

每个调用方（业务服务端、console-server）一把独立密钥，配置在 `IM_ADMIN_CALLERS` 指向的 JSON 文件（权限必须是 0600，改了要重启）：

```json
[
  {"id": "biz1", "alg": "hmac-sha256", "secret": "<base64，至少 32 字节>", "scopes": ["token.issue", "user.read", "user.write", "user.kick", "group.read", "group.write"]},
  {"id": "console", "alg": "ed25519", "publicKey": "<base64，32 字节公钥>", "scopes": ["user.read", "group.read"], "allowedCIDRs": ["10.0.0.0/8"]}
]
```

- `id` 为 `[a-z0-9-]{3,32}`；不同调用方的密钥不能相同，也不能等于 JWT 密钥或旧管理密钥（否则拒绝启动）。Ed25519 调用方只配置公钥。
- 每个接口要求一个 scope（下表"scope"列）；scope 不足 403 / 4030。正文类 scope（`msg.body.*`、`msg.quarantine`）只能配置在唯一一个 `"bodyAccess": true` 的调用方上，且该调用方不能有其他 scope。
- 可选 `allowedCIDRs`：来源不在范围内 403 / 4030；`disabled: true` 的调用方 401 / 4017。

**请求头**：`X-IM-Caller`（调用方 id）、`X-IM-Timestamp`（毫秒）、`X-IM-Nonce`（16～64 个 base64url 字符，每次随机）、`X-IM-Signature`（base64url 无填充）；可选 `X-IM-Operator`（后台操作人，如 `admin:a_01J…`）、`X-Request-Id`（≤ 64）、`X-IM-Target-Node`（目标节点，必须等于服务端 `IM_NODE_ID`）。

**待签名串**（UTF-8，8 行，`\n` 分隔，末尾没有换行）：

```
METHOD                      大写，如 POST
PATH[?规范化查询串]          查询串：解码后按键名（相同键按值）以字节序排序，键和值按 RFC 3986 编码（只保留 A-Z a-z 0-9 - . _ ~），& 连接
X-IM-Timestamp
X-IM-Nonce
hex(SHA-256(请求体))         没有请求体时用空串的哈希
X-IM-Operator               没有为空串
X-Request-Id                没有为空串
X-IM-Target-Node            没有为空串
```

HMAC：`HMAC-SHA256(secret, 待签名串)`；Ed25519：私钥对待签名串签名。操作人和请求 ID 都在签名范围内，中间人改不了。

**校验**：时间戳与服务端时间相差超过 300 秒拒绝；同一调用方 10 分钟内重复的 nonce 拒绝（只有签名正确的请求才登记 nonce）。失败统一 401 / 4017（`msg` 说明原因，只用于排查），并计入指标 `im_admin_auth_failures_total{reason}`。

**示例与测试向量**：[`vectors/admin-sign.json`](vectors/admin-sign.json)（HMAC、Ed25519 各 3 组）。参考实现：Go [`server/internal/adminauth`](../server/internal/adminauth/sign.go)、Node.js [`tools/admin-sign/sign.mjs`](../tools/admin-sign/sign.mjs)、Java [`tools/admin-sign/AdminSign.java`](../tools/admin-sign/AdminSign.java)，三者在 CI 里都跑这组向量。

## 旧密钥 `X-Admin-Key`（过渡，PRD-admin F-KEY-05）

旧的共享密钥 `IM_ADMIN_KEY` 视为调用方 `legacy`：不需要签名，但**只能访问 v1.2 已有的 9 个接口**（下面"用户与凭证""群"两张表）和测试模式下的 `/admin/test/*`，调用其他接口 403 / 4030；密钥错误 401 / 4013。请求带 `X-IM-Caller` 时按签名判定，忽略 `X-Admin-Key`。
生产环境（`IM_ENV=production`）默认不接受旧密钥，只有设置 `IM_ADMIN_LEGACY=1` 才接受，并在启动日志和 `/readyz`（`legacyAdminKey: true`）告警。新接入请直接使用调用方签名。

## 用户与凭证

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `POST /admin/users/token` | `{userId, ttlSeconds?}` | `{userId, token, expiresAt}` | 签发 token（对应腾讯 `userSig`），用户不存在时自动注册。有效期默认 7 天，范围 60 秒～30 天 |
| `POST /admin/users/import` | `{users:[{userId, nickname?, avatar?}]}` | `{imported}` | 批量导入 / 更新，单次 ≤ 500；任何一项不合法整批不写入 |
| `GET /admin/users/{userId}` | — | `{userId, nickname, avatar, createdAt, official}` | 不存在：404 / 4042 |
| `POST /admin/users/{userId}/force-logout` | `{}` | `{}` | 踢下线全部连接，此前签发的 token 全部作废；重新签发即可解封 |

## 群

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `POST /admin/groups` | `{groupId, name, members?, historyVisible?}` | 201 `{}` | 建群。初始成员 ≤ 2000（不受 500 批量限制） |
| `GET /admin/groups/{groupId}` | — | `{groupId, name, historyVisible, createdAt, memberCount, members}` | 查询群 |
| `PATCH /admin/groups/{groupId}` | `{name?, historyVisible?}` | `{}` | 修改设置；至少传一项。`historyVisible` 在查询时生效 |
| `POST /admin/groups/{groupId}/members` | `{members}` | `{added}` | 加成员，单次 ≤ 500；已经是成员的忽略（不重置入群位置） |
| `DELETE /admin/groups/{groupId}/members` | `{members}` | `{removed}` | 移除成员，单次 ≤ 500；不是成员的忽略。被移出的人收到 `convRemoved {reason: "removed"}` |
| `DELETE /admin/groups/{groupId}` | — | `{}` | 解散群（PRD-social F-SOC-26、PRD-admin G-ADM-11）：删除全部成员，消息保留；全部在线成员收到 `convRemoved {reason: "dissolved"}`。之后对该群的查询、加成员、改设置都是 404 / 4043，成员的 `send` / `sync` / `read` 返回 4043；**群 ID 不能复用**（再建 409 / 4090）。不存在或已解散：404 / 4043 |

规则：
- 群名去首尾空白后 1～50 个字符（按 Unicode 码点）；群 ID、userId 为字母、数字、`_`、`-`，1～64 位。
- 成员中有不存在的用户：404 / 4042，**整批不生效**。同一请求里重复的 userId 按 1 个计。
- 多个错误同时出现时的优先级：ID 格式 4004 → 其他参数 4000 → 群不存在 4043（建群为已存在 4090）→ 单次超过 500 人 4131 → 用户不存在 4042 → 总人数超过 2000 4130。
- `historyVisible=false`（默认）：新成员只能看到入群之后的消息；`true`：可以看到全部历史。被移除后再加入，按重新加入的时刻计算。

scope：`POST /admin/users/token` → `token.issue`；`POST /admin/users/import` → `user.write`；`GET /admin/users/{userId}` → `user.read`；`POST /admin/users/{userId}/force-logout` → `user.kick`；`GET /admin/groups/{groupId}` → `group.read`；建群、改设置、加 / 移除成员 → `group.write`；解散 → `group.dissolve`（旧密钥不能调用）；`/admin/test/*` → `test`。完整 scope 目录见 PRD-admin F-KEY-02 规则 2。

## 社交（PRD-social 9.3 节，M3）

只在管理端口注册，按 scope 判定（旧密钥不能调用：403 / 4030）；写接口按下一节审计（摘要只有字段名，备注原文不会进审计）。

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `GET /admin/settings/social` | — | 5.1 节全部参数 | scope `social.settings` |
| `PATCH /admin/settings/social` | 只传要改的字段 | 修改后的全部参数 | scope `social.settings`。超范围、未知字段、`null`、`mode` 非法 400 / 4000，整次不生效；成功后各节点 1 秒内生效，给全部在线连接下发 `socialSettingsChanged` |
| `POST /admin/friends/import` | `{pairs:[{a, b, remarkA?, remarkB?}]}` | `{imported}` | scope `social.friends`。≤ 500 对，建立双向边（来源 `admin`），整批原子；错误优先级 4004 → 4000（`a == b`）→ 4131 → 4042 → 4132（`msg` 写出 userId）。幂等：备注值变化才算变化；同一请求里重复的对以最后一条为准；同时把这些对之间的申请置为 accepted；不受拉黑影响也不解除拉黑 |
| `DELETE /admin/friends` | `{pairs:[{a, b}]}` | `{removed}` | scope `social.friends`。删除双向边（墓碑），不是好友的忽略 |
| `GET /admin/users/{userId}/friends?cursor=&limit=` | — | `{friends:[{userId, source, createdAt}], nextCursor}` | scope `social.friends`。按对方 userId 升序，`limit` 1～500（默认 100），**不返回备注**；`nextCursor` 为空表示没有更多 |
| `PUT /admin/users/{userId}/official` | `{official}` | `{userId, official}` | scope `social.settings`。用户不存在 404 / 4042；立即生效，不改 ver、不发事件 |

`IM_SOCIAL_DEFAULTS`（JSON，字段同上）只在库里还没有社交配置时写入一次，之后以数据库为准；格式或范围非法拒绝启动。

## 审计（PRD-admin F-KEY-04）

每次写接口（以及在线设备、消息元数据、正文读取、审计查询）在执行**前**写一条 `intent`，执行后写一条 `result`（同一 `X-Request-Id`，`code` 为业务结果码，0 表示成功）；`intent` 写不进去时接口不执行，返回 503 / 5030。只读的 `user.read` / `group.read` 不记录。
记录字段：`caller`（旧密钥为 `legacy`）、`operator`（只取签名过的 `X-IM-Operator`）、`requestId`、`route`、`targets`（`{ids: 前 50 个, count}`）、`summary`（只有请求里出现的**字段名**，不记任何字段值）、`phase`、`code`。存储层只有追加和查询，没有修改和删除。

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `GET /admin/audit?from=&to=&caller=&operator=&requestId=&cursor=&limit=` | — | `{entries, nextCursor}` | scope `audit.read`（旧密钥不能调用）。`from` / `to` 为毫秒，必填，跨度 ≤ 31 天，否则 400 / 4000；按 id 倒序，`limit` 1～200（默认 50），`nextCursor` 为 0 表示没有更多 |

## 运维

| 接口 | 说明 |
|---|---|
| `GET /healthz` | 进程存活 |
| `GET /readyz` | 可以接收流量（数据库可用、没有在下线）；不可用时 503 |
| `GET /metrics` | Prometheus 指标。**只在单独的内网地址上提供**（`IM_METRICS_ADDR`，默认 `127.0.0.1:9090`，`off` 关闭），对外端口上没有：在线连接数（按平台）、握手结果、上行命令数和错误码分布、命令处理耗时、新消息数、关闭码分布（按关闭方）、被踢原因、慢连接数，以及 Go 运行时和进程指标 |

`IM_METRICS_ADDR` 上还提供 `/debug/pprof/`（Go 性能分析），同样只在内网。内存规划和 `GOMEMLIMIT` 设置见 [内存分析](../docs/reports/mem-2026-11-03.md)。

备份、恢复、数据库迁移、计数器修复通过命令行完成（`im-server backup / restore / migrate / repair counters`），不提供 HTTP 接口。

## 仅测试 / 演示环境

`POST /admin/test/drop-ack`、`/admin/test/skip-delivery`、`/admin/test/clock`、`/admin/test/faults`（`{adminAuditWrite: "fail" | ""}`，模拟审计写失败）、社交测试设施（PRD-social 5.6 节）`/admin/test/social/drop-events {userId, type, count}`、`/admin/test/social/purge`、`/admin/test/social/seed {friends?, blocks?, requests?, groupMembers?, updatedAt?}`、`/admin/test/limits/reset {userId}`、`/admin/test/invalidate {topic, key}`（双节点测试环境：其他进程转发来的缓存失效，`IM_TEST_PEERS` 配置转发目标，`IM_TEST_INVALIDATION=off` 时不发也不收）（`IM_TEST_MODE=1`），`POST /demo/login {username, nickname?}`（`IM_DEMO_LOGIN=1`，不需要管理密钥；`nickname` 去首尾空白后 1～32 个码点，登录时覆盖该用户的昵称，超长 400 / 4000，不传或为空不改）。生产环境（`IM_ENV=production`）开启这两个开关会拒绝启动。
