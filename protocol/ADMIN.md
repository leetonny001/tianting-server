# 管理接口

管理接口**只在单独的管理端口上提供**（`IM_ADMIN_ADDR`，默认 `127.0.0.1:8081`，`off` 关闭）；对外端口（`IM_ADDR`）上请求 `/admin/*` 返回 404。生产环境管理地址不是回环地址时必须配置 TLS（`IM_ADMIN_TLS_CERT` / `IM_ADMIN_TLS_KEY`），可以用 `IM_ADMIN_ALLOWED_CIDRS` 限制来源网段（只看 TCP 对端地址，不解析 `X-Forwarded-For`），见 PRD-admin F-KEY-01。

请求和响应都是 JSON。错误统一返回 `{"code": 4xxx, "msg": "..."}`，HTTP 状态码取 `errors.json` 的 `http` 字段（见 [ERRORS.md](ERRORS.md)）。
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
| `GET /admin/users/{userId}` | — | `{userId, nickname, avatar, createdAt, official, online}` | 不存在：404 / 4042。`online` 来自在线路由（单节点是本机），不缓存 |
| `POST /admin/users/{userId}/force-logout` | `{}` | `{}` | 踢下线全部连接，此前签发的 token 全部作废；重新签发即可解封 |

## 账号（PRD-chat 第 11.6 节，随 BE-421 实现）

| 接口 | scope | 请求 | 成功响应 | 说明 |
|---|---|---|---|---|
| `POST /admin/invites` | `account.invite` | `{count, maxUses, expiresAt, note?}` | `{invites:[{inviteId, code, last4, maxUses, expiresAt}]}` | 明文只在这里出现一次；服务端只存哈希和后 4 位。`count` ≤ 100（4131），`maxUses` 1～1000（4000） |
| `GET /admin/invites` | `account.invite` | — | `{invites:[…]}` | 没有明文；显示已用次数、是否停用、是否自动停用 |
| `POST /admin/invites/{id}/disable` | `account.invite` | `{}` | `{}` | 立即生效，第一批不能重新启用，用新建恢复 |
| `GET /admin/accounts/{userId}` | `user.read` | — | `{hasAccount, createdAt?, lastLoginAt?, sessionCount?}` | 没有任何哈希和令牌 |

**没有 `account.reset`，没有任何改密码的管理接口**（PRD-chat F-ACC-05 规则 4）。

## 群

| 接口 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `POST /admin/groups` | `{groupId, name, members?, historyVisible?, owner?}` | 201 `{}` | 建群（托管群，默认没有群主）。初始成员 ≤ 2000（不受 500 批量限制）。`ug-` 前缀保留给客户端建的群：400 / 4004。`owner` 需要 `social.settings`，必须在 `members` 里（否则 404 / 4046） |
| `GET /admin/groups/{groupId}` | — | `{groupId, name, historyVisible, createdAt, memberCount, members, owner, admins, notice, createdBy, managed, allMuted}` | 查询群；`managed=true` 表示托管群，`owner` 为空表示没有群主；`allMuted` 是全群禁言状态（PRD-admin F-ADM-42，`IM_V13_ADMIN_FEATURES` 关闭时恒为 `false`） |
| `PATCH /admin/groups/{groupId}` | `{name?, historyVisible?, notice?, owner?}` | `{}` | 修改设置；至少传一项。`historyVisible` 在查询时生效。`owner`：字符串指定 / 更换群主（必须是成员，否则 404 / 4046），`null` 收回；需要同时有 `group.write` 和 `social.settings`，旧密钥 403 / 4030；生成 `ownerChanged` 提示消息（`from=""`）。其他字段不产生提示消息 |
| `POST /admin/groups/{groupId}/members` | `{members}` | `{added}` | 加成员，单次 ≤ 500；已经是成员的忽略（不重置入群位置） |
| `DELETE /admin/groups/{groupId}/members` | `{members}` | `{removed}` | 移除成员，单次 ≤ 500；不是成员的忽略。被移出的人收到 `convRemoved {reason: "removed"}` |
| `DELETE /admin/groups/{groupId}` | — | `{}` | 解散群（PRD-social F-SOC-26、PRD-admin G-ADM-11）：删除全部成员，消息保留；全部在线成员收到 `convRemoved {reason: "dissolved"}`。之后对该群的查询、加成员、改设置都是 404 / 4043，成员的 `send` / `sync` / `read` 返回 4043；**群 ID 不能复用**（再建 409 / 4090）。不存在或已解散：404 / 4043 |

规则：
- 群名去首尾空白后 1～50 个字符（按 Unicode 码点）；群 ID、userId 为字母、数字、`_`、`-`，1～64 位。
- 成员中有不存在的用户：404 / 4042，**整批不生效**。同一请求里重复的 userId 按 1 个计。
- 多个错误同时出现时的优先级：ID 格式 4004 → 其他参数 4000 → 群不存在 4043（建群为已存在 4090）→ 单次超过 500 人 4131 → 用户不存在 4042 → 总人数超过 2000 4130。
- 管理接口对所有群（包括客户端建的群）保留全部能力，不受好友、`max_members`、`maxJoinedGroups` 限制（仍受 2000）；除群主变化外不产生提示消息。移除的人是群主时：自建群自动转让给最早的管理员 / 最早入群的成员，托管群回到无群主。
- `historyVisible=false`（默认）：新成员只能看到入群之后的消息；`true`：可以看到全部历史。被移除后再加入，按重新加入的时刻计算。

scope：`POST /admin/users/token` → `token.issue`；`POST /admin/users/import` → `user.write`；`GET /admin/users/{userId}` → `user.read`；`POST /admin/users/{userId}/force-logout` → `user.kick`；`GET /admin/groups/{groupId}` → `group.read`；建群、改设置、加 / 移除成员 → `group.write`；解散 → `group.dissolve`（旧密钥不能调用）；`/admin/test/*` → `test`。完整 scope 目录见 PRD-admin F-KEY-02 规则 2。

## 后台查询（PRD-admin 第一批）

| 接口 | 请求 / 查询参数 | 响应 | scope 与说明 |
|---|---|---|---|
| `GET /admin/users` | `q`、`official=true`、`cursor`、`limit` | `{users:[{userId, nickname?, avatar?, createdAt?, official, online}], nextCursor}` | `user.read`，不写审计。`q` 去首尾空白后 1～64 字符：userId 前缀（区分大小写）或昵称前缀（ASCII 不区分大小写，其他字符按码点精确）；`%`、`_` 不是通配符。没有 `q` 时必须 `official=true`。按 userId 升序 |
| `POST /admin/users/sessions/query` | `{userId}` | `{sessions:[{connId, platform, connectedAt, node, ip, queueLen}], partial, scope}` | `user.sessions`，写审计（result 的 summary 带 `count`）。`ip` 已脱敏（IPv4 `a.b.*.*`，IPv6 前 3 组），完整 IP 拿不到。B6 之前 `partial: true, scope: "localNode"`：只覆盖接收请求的节点，**不能据此下"不在线"的结论**。用户不存在 404 / 4042 |
| `GET /admin/groups` | `q`、`includeDissolved=true`、`cursor`、`limit` | `{groups:[{groupId, name, type, owner, memberCount, historyVisible, createdAt, dissolvedAt?}], nextCursor}` | `group.read`，不写审计。`q` 可选：groupId 前缀或群名前缀。`type` 为 `managed`（托管）/ `user`（客户端自建）；`owner` 没有时为 `null`。按创建时间倒序，默认不含已解散的群 |
| `GET /admin/groups/{groupId}/members` | `cursor`、`limit`、`prefix` | `{members:[{userId, role, joinedAt}], nextCursor}` | `group.read`。按 userId 升序（和客户端 `groupMembers` 的顺序不同），`prefix` 是 userId 前缀。已解散或不存在 404 / 4043 |
| `POST /admin/convs/messages/query` | `{convId, beforeSeq?, limit?, from?, sinceTs?, untilTs?}` | `{messages:[{seq, msgId, from, type, bodyLength, fileSize?, ts, deleted?, deletedAt?, deletedBy?, recalled?, recalledAt?, recalledBy?}], hasMore}` | `msg.meta`，写审计。**响应没有正文**；撤回的消息（C4，PRD-chat F-RCL-04 规则 3）带 `recalled`、`recalledAt`、`recalledBy`，`deleted` 只表示后台删除，`bodyLength` 为 0、没有 `fileSize`（和后台删除一致，文件名按正文对待，R-CS-14）；`bodyLength` 是文本的码点数（非文本消息是 body JSON 的码点数）。`fileSize` 是图片、文件消息的原文件字节数（不含缩略图），其他消息和已删除的不出现（PRD-chat v1.4 第 14.1 节）。`limit` 1～50，默认 50；按 seq 倒序；时间是闭区间。不受成员关系和 `historyVisible` 限制，已解散的群照常返回；会话不存在返回空列表；`convId` 格式错误 400 / 4007 |

P1（PRD-admin G-ADM-07、G-ADM-13、G-ADM-17）：

| 接口 | 请求 / 查询参数 | 响应 | scope 与说明 |
|---|---|---|---|
| `GET /admin/users/{userId}/groups` | `cursor`、`limit` | `{groups:[{groupId, name, type, owner, memberCount, historyVisible, createdAt, role, joinedAt}], nextCursor}` | `group.read`，不写审计。用户当前所在的群，不含已解散的，按 groupId 升序。`limit` 只接受 20 / 50 / 100，默认 50。用户不存在 404 / 4042 |
| `POST /admin/users/convs/query` | `{userId, cursor?, limit?}` | `{convs:[{convId, type, peerId?, groupId?, groupName?, myRole?, memberCount?, maxSeq, readSeq, updatedAt, lastTs?, lastFrom?, lastType?}], nextCursor}` | `msg.meta`，写审计（`targets` 是 userId，result 的 summary 带 `count`）。userId 只在请求体里，不进 URL。**响应没有正文和摘要文本**。排序和客户端会话列表相同：`updatedAt` 倒序，同一时间按 convId 升序。用户已退出的群、已解散的群不在列表里。`limit` 只接受 20 / 50 / 100，默认 50，`cursor` 解不开 400 / 4000。用户不存在 404 / 4042。理由（F-ADM-30 规则 3）由 console-server 负责 |
| `GET /admin/stats` | — | `{online:{total, byPlatform:{web?, pc?, ios?, android?}, partial, scope}, newUsersToday, messagesToday, groupsTotal, dayStart, serverTime}` | `stats.read`，不写审计。不读 `/metrics`。没有连接的平台不出现在 `byPlatform` 里。B6 之前 `partial: true, scope: "localNode"`：连接数只覆盖接收请求的节点。"今日"是 Asia/Shanghai 自然日（`dayStart` 是当天零点的毫秒时间戳，IM 钟）。`messagesToday` 不含群提示消息，含之后被删除的消息；`groupsTotal` 不含已解散的群 |

`limit`（前四个接口）只接受 20 / 50 / 100，默认 50；`cursor` 是上一页响应里的 `nextCursor`，对调用方不透明，空字符串表示没有更多，解不开 400 / 4000。这些接口旧密钥都不能调用（403 / 4030）。

## 封禁、禁言、删除、正文查看（PRD-admin 第二批）

`IM_V13_ADMIN_FEATURES` 关闭（默认）时这些路由都不存在（404），也不下发新事件。关闭时禁言不生效，已有的封禁仍然生效。

| 接口 | 请求 | 响应 | scope 与说明 |
|---|---|---|---|
| `POST /admin/users/{userId}/ban` | `{durationSeconds \| null, reason}` | `{userId, ban:{until, reason, by, at}}` | `user.ban`。`durationSeconds` 必须传：60 秒～30 天，`null` 表示永久。立即踢掉全部连接（`kicked {reason:"suspended", until}`，关闭码 4012），此前的 token 作废。不存在 404 / 4042。重复封禁以最后一次为准 |
| `POST /admin/users/{userId}/unban` | `{reason}` | `{}` | `user.ban`。解封后要重新签发 token |
| `POST /admin/mute` | `{scope:"user"\|"group", userId, groupId?, durationSeconds \| null, reason}` | `{mutedUntil}` | `mute.write`。时长同上。用户不存在 404 / 4042；群不存在 404 / 4043；不是群成员 404 / 4046。只有本人收到 `muted` 事件 |
| `POST /admin/mute`（全群，P1） | `{scope:"groupAll", groupId, reason}` | `{mutedUntil: null}` | `mute.write`。开关，手动关闭前一直生效：传 `userId` 或非 null 的 `durationSeconds` 是 400 / 4000。群不存在 404 / 4043。群主、管理员仍可发言，其他成员 `send` 返回 4031，`details = {scope:"groupAll", groupId, mutedUntil:null}`。全群在线成员（含群主、管理员）收到 `muted {scope:"groupAll", groupId, mutedUntil:null}`；已经开着时再开不重复通知。`convs` 和 `convAdded` 的群会话带 `allMuted: true`。用户级、群内禁言优先：被单独禁言的群主、管理员仍然 4031（`scope` 是 `user` / `group`）。解散群时清除 |
| `POST /admin/unmute` | `{scope, userId, groupId?, reason}` | `{}` | `mute.write`。幂等；真有一条生效中的禁言才发 `unmuted`。全群（P1）：`{scope:"groupAll", groupId, reason}`，全群在线成员收到 `unmuted {scope:"groupAll", groupId}` |
| `POST /admin/messages/delete` | `{convId, msgIds, reason}` | `{deleted:[{msgId, seq}]}` | `msg.delete`。单次 ≤ 50（4131）；不存在 404 / 4044；不在该会话或是群提示消息 400 / 4000，任何一条不合法整批不删。已删除的幂等成功，不再发事件。**已撤回的同样幂等成功**（C4，PRD-chat F-RCL-04 规则 1）：不出现在 `deleted` 里、不写 `deletedAt`、状态和文案保持撤回、不发事件，审计照写。原文加密进隔离区；被删的提及不再计入 `mentionUnread` |
| `POST /admin/msg-grants` | `{convId, operator, reason, ticket, quarantine?}` | `{grantId, expiresAt}` | 群 `msg.body.group`，单聊 `msg.body.c2c`，`quarantine=true` 要 `msg.quarantine`。`operator` 必须等于签名的 `X-IM-Operator`。授权 15 分钟。`IM_C2C_BODY_ACCESS=off` 时单聊 403 / 4030 `reason:"c2cBodyDisabled"`。配额（Asia/Shanghai 自然日）：每操作人 20、每调用方 200，超出 429 / 4292 |
| `POST /admin/convs/messages/body` | `{grantId, convId, beforeSeq?, limit?}` | `{messages, hasMore}` | 同上。`limit` ≤ 50。授权不存在、过期、换了会话或操作人 403 / 4039。**后台版序列化**（C4，RC2-21）：`deleted` 只表示后台删除；撤回的消息 `deleted=false`、`recalled=true`、`recalledAt`、`recalledBy`。普通授权：被删、撤回的消息没有 `body`。隔离授权：只返回被删、撤回的消息和原文（文件消息带文件名），一条都没有（含已到期）404 / 4044。响应 `Cache-Control: no-store` |
| `POST /admin/files/{fileId}/fetch` | `{convId, msgId, variant?: "original" \| "thumb"}` | 文件字节 | **C4 新增（PRD-chat 第 11.6 节，F-RCL-04，BE-470 第 4 条定的形状）**。审核员看被撤回或被后台删除的消息里的图片、文件，console-server 代理（CS-202），不给 console 发预签名地址。scope `msg.quarantine`（C4；C6 增加 `report.evidence`），**还要有这个会话有效的隔离授权**（同一调用方、同一 `X-IM-Operator`、`quarantine=true`、未过期，按会话查找，请求里不带 `grantId`），没有 403 / 4039；scope 不足 403 / 4030；单聊受 `IM_C2C_BODY_ACCESS`（关闭时 403 / 4030 `reason:"c2cBodyDisabled"`）。`fileId` 必须属于 `msgId`、`msgId` 属于 `convId`、消息已撤回或已被后台删除、对象还在保留期，否则 404 / 4048（不区分原因）；`variant` 缺省 `original`，`thumb` 只有图片有。响应 `Content-Type` 按服务端记录、`Content-Disposition: attachment`（不带文件名）、`X-Content-Type-Options: nosniff`、`Cache-Control: no-store`。写审计（`fileId`、`msgId`、`variant`，不含文件名） |

- `GET /admin/users/{userId}`、`GET /admin/users` 增加 `ban`（`null` 或 `{until, reason, by, at}`）和 `mutedUntil`（禁言中才出现，永久为 `null`）；`GET /admin/groups/{groupId}/members` 的成员增加 `mutedUntil`；`POST /admin/convs/messages/query` 的消息增加 `deleted`、`deletedAt`、`deletedBy`（正文读取的响应也有）；C4 再增加 `recalled`、`recalledAt`、`recalledBy`（`recalledBy` 是用户 ID，撤回是群内自治，不写后台审计，F-RCL-02 规则 5）。过期的记录按没有处理。
- `POST /admin/users/token`、`POST /demo/login`：封禁中 403 / 4034，`details.bannedUntil`（永久为 `null`）。到期自动解除。
- `by`、`deletedBy` 的格式是 `{caller}:{operator}`。审计记录这些接口的 `reason`、`msgIds`、`ticket`、时长，不记正文；正文读取的 summary 带 `seqFrom`、`seqTo`、`count`。
- 配置：`IM_QUARANTINE_KEY`（≥ 32 字节，不能与其他密钥相同，生产必填）、`IM_QUARANTINE_DAYS`（默认 30，1～365）、`IM_C2C_BODY_ACCESS`（默认 `off`，没有管理接口能改）。
- 已知限制：删除后 WAL 里的旧帧要等检查点才被覆盖；备份里的原文按备份保留期留存（F-ADM-31 规则 9）。

## 系统推送（PRD-live v1.0 L1，PRD-admin F-ADM-50）

`IM_PUSH_ENABLED` 关闭（默认）时路由不存在（404），`pushSync` 返回 4001，SDK 静默忽略；这个开关和 `IM_V13_ADMIN_FEATURES` 互相独立。旧密钥不能调用（403 / 4030）。语义见 [PRD-live v1.0](../docs/prd/PRD-live-v1.0.md) 第 4 节。

| 接口 | 请求 | 响应 | scope 与说明 |
|---|---|---|---|
| `POST /admin/push` | `{target, userIds?, roomId?, type, data, pushId, ttlSeconds?}` | `{pushId, created, accepted?}` | `target=user` 要 `push.user`，`all` 要 `push.all`，`room` 要 `push.room`（接口 L2 开放，L1 返回 400 / 4000）。写审计 |

字段规则：

- `pushId`：1～64 位 `[A-Za-z0-9_-]`，调用方生成，**全局唯一**（建议 UUID）。过期后也不应复用。
- `type`：1～64 字符，服务端不解释。`data`：必填，序列化后 ≤ 4096 字节，im-server 不校验模板。
- `target=user`：`userIds` 不能为空，去重后 ≤ 1000（400 / 4131），`accepted` 是去重后的人数；有不存在的用户 404 / 4042，`details.userIds` 列出，整批不发。`target=all` 不能带 `userIds`、`roomId`，响应没有 `accepted`。
- `ttlSeconds`：0～604800。`target=user` 不传是 604800（进收件箱，离线可补收），传 0 只投在线（没有 `pushSeq`）；其他 `target` 不传是 0，传非 0 400 / 4000。

幂等：同一调用方、同样内容（`target`、去重排序后的 `userIds`、`type`、`data` 按键排序、生效的 `ttlSeconds`）重试返回 200、`created=false`，不再次投递，不计入限速；调用方或内容不同 409 / 4094。4000、4131、4042、4030、4290 都不占 `pushId`。`target=all` 和 `ttlSeconds=0` 是先写 `pending` 记录再广播，崩溃后同号重试会重新广播，帧的 `ts` 沿用首次受理时间，靠 SDK 按 `pushId` 去重。幂等记录保留 8 天，还有对应收件箱行时不删。

错误优先级：签名和 scope → `type` 白名单 403 / 4030 → 参数 400 / 4000 → 4094 / 幂等命中 → 4131 → 4042 → 限速 429 / 4290。

调用方配置增加可选的 `pushTypes: string[]`：配了就只允许名单内的 `type`，所有 `target` 都适用。**`IM_PUSH_ENABLED` 开启时，持有 `push.all` 的调用方必须配非空的 `pushTypes`，否则启动失败**（错误信息只有调用方 ID），不提供通配符。

审计：`summary` 记 `target`、`type`、`pushId`（intent 和 result 都有），result 另外记 `targetCount`（只有 `target=user`）、`dataLength`、`dataSha256`。不记 `userIds` 和 `data` 明文。

配置（都未校准）：`IM_PUSH_ALL_RATE`（`target=all`，每调用方每分钟，默认 20）、`IM_PUSH_USER_RATE`（`target=user`，每调用方每分钟，默认 600）、`IM_PUSH_SYNC_RATE`（`pushSync`，每用户每分钟，默认 120）、`IM_PUSH_INBOX_MAX`（单用户收件箱上限，默认 1000，超出淘汰最旧的，序号不变）、`IM_PUSH_PURGE_BATCH`（`push_purge` 每批行数，默认 1000；定时任务每小时跑到清完，每批之间释放数据库连接）。

指标：`im_pushes_total{target}`（新受理，不含重放）、`im_push_deliveries_total{target}`（接受投递的连接数，含重播）、`im_push_inbox_rows`；`pushSync` 的次数和耗时在 `im_commands_total`、`im_command_duration_seconds`（`cmd="pushSync"`）。

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

定时任务（进程内，每小时）：`admin_audit_purge`（审计保留 1 年）、`quarantine_purge`（隔离原文到期物理删除，含撤回的，C4）、`mention_purge`（已读之前的提及计数行，C4）、`msg_grants_purge`、`social_purge`、`group_req_purge`。指标 `im_job_runs_total{job,result}`、`im_job_duration_seconds{job}`。

| 接口 | 说明 |
|---|---|
| `GET /healthz` | 进程存活 |
| `GET /readyz` | 可以接收流量（数据库可用、没有在下线）；不可用时 503 |
| `GET /metrics` | Prometheus 指标。**只在单独的内网地址上提供**（`IM_METRICS_ADDR`，默认 `127.0.0.1:9090`，`off` 关闭），对外端口上没有：在线连接数（按平台）、握手结果、上行命令数和错误码分布、命令处理耗时、新消息数、关闭码分布（按关闭方）、被踢原因、慢连接数，以及 Go 运行时和进程指标 |

pprof 只在 `IM_PPROF_ADDR` 上提供（开发默认 `127.0.0.1:6060`，生产默认关闭，设置时必须是回环地址）；需要时通过 SSH 隧道访问。内存规划和 `GOMEMLIMIT` 设置见 [内存分析](../docs/reports/mem-2026-11-03.md)。

备份、恢复、数据库迁移、计数器修复通过命令行完成（`im-server backup / restore / migrate / repair counters`），不提供 HTTP 接口。

## 节点运维接口（PRD-ops F-OPS-16，只给 console-server）

独立监听 `IM_OPS_ADDR`（默认 `127.0.0.1:9091`，`off` 关闭），配置了 `IM_OPS_PUBKEYS=kid:base64`（Ed25519 公钥，最多 2 把，用于轮换）才监听。生产环境必须配置 `IM_NODE_ID`，且要么配置公钥要么设 `IM_OPS_ADDR=off`；非回环地址必须有 `IM_OPS_TLS_CERT` / `IM_OPS_TLS_KEY` 和 `IM_OPS_ALLOWED_CIDRS`。

签名和管理接口同一实现（8 行待签名串），但调用方固定为 `console-ops`，`X-IM-Target-Node` 必填且等于本节点 ID；管理接口的调用方不能访问，`console-ops` 也不能访问 `/admin/*`。向量：[`vectors/ops-sign.json`](vectors/ops-sign.json)。检查顺序：来源网段（403 / 4030）→ 并发闸门（同时最多 4 个，429 / 4290）→ 签名（401 / 4017）→ 处理超时 2 秒（503 / 5030）。

| 接口 | 请求 | 响应 |
|---|---|---|
| `GET /internal/v1/node` | — | `{nodeId, version, startedAt, ready, draining, connections:{平台:数量}, totalConns, liveBytes, heapBytes, stackBytes, goMemLimit, goroutines, disconnectBufferFrom}`（只读 `runtime/metrics`，不触发 STW） |
| `POST /internal/v1/connections:query` | `{userId}` | `{connections:[{connId, platform, connectedAt, node, ip, queueLen}]}`：用户不存在和不在线响应相同；`ip` 已脱敏 |
| `POST /internal/v1/disconnects:query` | `{userId, limit≤50}` | `{records:[{userId, connId, phase, platform, code, reason, by, ts, durationMs, ip}], bufferFrom}`：新的在前；每用户 50 条、全节点 2 万条，重启清空 |
| `GET /internal/v1/config` | — | 脱敏的配置摘要（`env`、`productionChecks`、`keys`、`network`、`limits`、`database`、`switches`）：密钥只有 kid、数量和是否达标 |
| `GET /internal/v1/db` | — | `{type, location, connected, version, migrations:[{version, name, appliedAt, checksumOk, pending}], pending}`：只读连接，和 `im-server migrate status` 同一个函数 |

每次请求写一行访问日志（`msg="ops request"`）：`endpoint`、`kid`、`operator`、`requestId`、`count`、`durationMs`、`peer`、`result`，失败时带 `failure`；不写 userId、密钥和签名。对账用 `deploy/ops/reconcile`。

## 仅测试 / 演示环境

`POST /admin/test/jobs/{name}/run`：立即运行一个定时任务。

系统推送（PRD-live 8.2 节，`IM_PUSH_ENABLED=1` 且 `IM_TEST_MODE=1` 才注册）：`GET /admin/test/push-inbox/{userId}`（`{rows}`，包括已到期还没清理的行）、`POST /admin/test/faults/push-skip-delivery {pushId}`（提交后跳过实时下发）、`POST /admin/test/faults/push-exit-before-broadcast {pushId}`（`all`、`ttlSeconds=0` 写完幂等记录后退出进程）、`POST /admin/test/faults/push-fail-at {index}`（下一次收件箱写入在第 `index` 个用户失败，按 userId 字节序从 0 起，整批回滚），都命中一次后清除。`IM_TEST_DISABLE_JOBS=push_purge`（逗号分隔，只在测试模式生效）让任务不再被定时触发，手动触发不受影响；手动触发 `push_purge` 只跑一批。`/admin/test/limits/reset {userId}` 同时清推送限速，`target=all`、`target=user` 按调用方，`userId` 传调用方 ID。

`POST /admin/test/drop-ack`、`/admin/test/skip-delivery`、`/admin/test/clock`、`/admin/test/faults`（`{adminAuditWrite: "fail" | "", opsDelayMs?, storePing: "fail" | "", quarantineWrite?, mentionWrite?, mentionAllCounter?}`：模拟审计写失败、运维接口延迟、就绪检查失败；C4 的三个是布尔值，不传不改：隔离写失败（撤回整个事务回滚 5000）、提及表写失败（消息不保存 5000）、@ 所有人计数不可用（5030））、社交测试设施（PRD-social 5.6 节）`/admin/test/social/drop-events {userId, type, count}`、`/admin/test/social/purge`、`/admin/test/social/seed {friends?, blocks?, requests?, groupMembers?, updatedAt?}`、`/admin/test/limits/reset {userId}`、`/admin/test/invalidate {topic, key}`（双节点测试环境：其他进程转发来的缓存失效，`IM_TEST_PEERS` 配置转发目标，`IM_TEST_INVALIDATION=off` 时不发也不收）（`IM_TEST_MODE=1`），`POST /demo/login {username, nickname?}`（`IM_DEMO_LOGIN=1`，不需要管理密钥；`nickname` 去首尾空白后 1～32 个码点，登录时覆盖该用户的昵称，超长 400 / 4000，不传或为空不改）。生产环境（`IM_ENV=production`）开启这两个开关会拒绝启动。
