# IM 协议（v1）

协议的唯一真相源，对应 [PRD v1.2](../docs/prd/PRD-core-v1.2.md) 和 [PRD-chat v1.1](../docs/prd/PRD-chat-v1.1.md)。服务端和三套 SDK（[tianting-sdk-ts](https://github.com/leetonny001/tianting-sdk-ts)、tianting-sdk-kotlin、tianting-sdk-swift）都以这里为准，并跑同一批用例。SDK 仓库里放的是副本，协议只在这里改，改完通知各 SDK 仓库运行 `scripts/sync-protocol.sh`。

| 文件 | 内容 |
|---|---|
| `schema/frame.schema.json` | WebSocket 帧的 JSON Schema（2020-12） |
| `schema/http-client.schema.json` | 客户端 HTTP 接口 `/v1/*` 的 Schema（PRD-chat 第 11.2 节） |
| `vectors/frames.json` | 单帧校验用例：`valid` 必须通过，`invalid` 必须拒绝 |
| `errors.json` / [ERRORS.md](ERRORS.md) | 错误码、关闭码、被踢原因 |
| [ADMIN.md](ADMIN.md) | 管理接口（只在管理端口） |

## 1. 连接

```
ws(s)://{host}/ws?platform={android|ios|web|pc}
→ 第一帧：{"id":"a1","cmd":"auth","data":{"token":"<jwt>"}}
← {"id":"a1","cmd":"ack","data":{"userId":"alice","serverTime":1700000000000}}
```

- **token 不放在 URL 里**（会被代理、CDN、浏览器历史记录），连接后第一帧用 `auth` 发送（PRD C-52）。收到 auth 的 ack 才算上线。
- 5 秒内没有发 `auth`，或第一帧不是 `auth`：以 4401 关闭（可以重连），不发 `kicked`。
- token 有三个来源（PRD-chat F-ACC-07 规则 1），校验相同：① 账号登录 `POST /v1/account/login`、`register`（带 `sid` 和代号 `gen`，有 refresh token）；② 演示登录 `POST /demo/login`（开发和演示环境）；③ 管理接口 `POST /admin/users/token`（没有 `sid`，最高权限，生产环境默认不放行）。
- 凭证问题：下发 `{"cmd":"event","data":{"type":"kicked","reason":"tokenInvalid|tokenExpired|banned|passwordChanged|sessionRevoked|accountDeleted"}}`，再以 4010 / 4011 / 4012 关闭（检查顺序 4010 → 4012 → 4011）。后三个原因可以带 `recoverBefore`。
- platform 缺失或不合法：握手后立即以 4009 关闭，不发 `kicked`（最先检查）。
- 浏览器的 Origin 必须在白名单内（`IM_ALLOWED_ORIGINS`），否则握手返回 HTTP 403。
- 同一用户最多 10 个连接，超出时最早的连接收到 `kicked/tooManyConnections` 并以 4429 关闭（不重连）。
- 服务端下线前会下发 `kicked/serverShutdown` 并以 1001 关闭：客户端**按退避重连**。
- 每个 WebSocket 消息是一帧 JSON（text frame），一帧一个对象。上行单帧最大 64KB；下行不限。

## 2. 命令

| 命令 | 方向 | 用途 | ack 数据 |
|---|---|---|---|
| `auth` | 上行 | 鉴权，必须是第一帧；成功后再发返回 4000 | `{userId, serverTime, recoveryCodeUnacked?}` |
| `send` | 上行 | 发消息 | `{msgId, convId, seq, ts}` |
| `sync` | 上行 | 拉消息：`afterSeq`（补拉之后的）或 `beforeSeq`（翻页拉之前的），二选一 | `{messages, hasMore}`，按 seq 升序 |
| `convs` | 上行 | 拉会话列表：按 `updatedAt` 倒序、`convId` 升序，最多 500 个 | `{convs, truncated}` |
| `read` | 上行 | 上报已读位置 | `{}` |
| `ping` | 上行 | 心跳 | 回 `pong`（不是 ack） |
| `msg` | 下行 | 新消息 | — |
| `event` | 下行 | `kicked`；`convAdded`（被拉进群，内容与 convs 的一项相同）、`convRemoved {convId, reason?}`（被移出群 `removed` / 群被解散 `dissolved` / 自己在其他端退群 `left`）；`readUpdated`（自己在其他端的已读位置前进了，包括发言即已读）；`socialChanged {ver}`（关系数据变了，只发给 ver 变化的用户，排除发起连接）、`socialSettingsChanged {mode, strangerMsgLimit}`（业务方改了社交配置）；`groupChanged {groupId}`（群资料或成员变了，同一个群至少间隔 1 秒合并，收到后重新 `groupInfo`） | — |
| `friendApply` / `friendRespond` / `friendDelete` / `friendRemark` | 上行 | 好友申请、处理申请、单向删除、备注（PRD-social F-SOC-02、03、05） | `{status?, ver, changed, items}` |
| `block` / `unblock` | 上行 | 拉黑、取消拉黑（F-SOC-07） | `{ver, changed, items}` |
| `friendRequestsSeen` | 上行 | "新的朋友"标记已读（F-SOC-04） | `{requestSeenVer, ver, changed}` |
| `socialSync` | 上行 | 关系同步：第一页 `{sinceVer}`，之后 `{cursor}`（F-SOC-06） | `{full, untilVer, latestVer, hasMore, cursor, restart?, friends, requests, blocks, requestSeenVer, settings, mode, strangerMsgLimit, official}` |
| `userSearch` / `setPrivacy` | 上行 | 精确搜索 userId；"可被搜索"开关（F-SOC-09） | `{user}`；`{ver, changed}` |
| `users` / `myGroups` | 上行 | 批量查资料（只返回可见范围内的人）；我的群聊分页（F-SOC-10、11） | `{users}`；`{groups, nextCursor}` |
| `groupCreate` | 上行 | 建群：`{clientReqId, name?, members}`，同一 `clientReqId` 只建一次（PRD-social F-SOC-21） | `{groupId, conv}` |
| `groupInvite` / `groupKick` | 上行 | 邀请（部分成功）/ 移出，单次 ≤ 50 人 | `{added, failed:[{userId, code}]}`；`{removed}` |
| `groupLeave` / `groupTransfer` / `groupSetAdmin` / `groupDissolve` / `groupUpdate` | 上行 | 退群、转让群主、设置管理员、解散、改群名 / 公告 | `{}` |
| `groupInfo` | 上行 | 群资料 | `{groupId, name, notice, noticeBy, noticeAt, owner, managed, myRole, memberCount, maxMembers, historyVisible, createdAt}` |
| `groupMembers` | 上行 | 成员分页：群主 → 管理员 → 成员，`limit` 1～100 | `{members, nextCursor}` |
| `joinRoom` / `leaveRoom` | — | 二期（聊天室） | — |
| `push` | 下行 | 系统推送：`{pushId, type, data, ts, target, pushSeq?, roomId?}`。`pushSeq` 只在进收件箱的推送上出现（`target=user` 且 `ttlSeconds` ≠ 0）；`target=all` 和 `ttlSeconds=0` 只投当前在线连接，不补收（[PRD-live v1.0](../docs/prd/PRD-live-v1.0.md) 第 4 节）。SDK 行为用例 `vectors/push-sync.json` | — |
| `pushSync` | 上行 | 按用户推送的离线补收：`{afterSeq, limit?}`（`limit` 1～100，默认 50，SDK 固定用 100）或 `{fromLatest: true}`（只取最新序号建立游标，不回放），二选一。只能拉自己的收件箱，只返回未过期的。`IM_PUSH_ENABLED` 关闭时 4001，SDK 静默忽略；每用户默认 120 次/分钟，超出 4290 | `{pushes, hasMore, latestSeq}`，三者在同一读事务内取得；`hasMore=true` 时 `pushes` 非空 |

## 3. 交互规则

1. 带 `id` 的请求，服务端必定回一个同 `id` 的 `ack`：成功时带 `data`，失败时带 `error`，两者不会同时出现。请求帧无法解析时，ack 不带 `id`（错误码 4002 / 4003）。
2. `msg` / `event` 是服务端主动下发的，没有 `id`，客户端不需要回复。
3. 心跳：客户端每 30 秒发 `ping`；服务端 90 秒收不到任何帧，就以 4408 关闭连接；客户端 10 秒收不到 `pong` 视为断线。
4. 发消息：客户端生成 `clientMsgId`（8～64 位）。超时后**用同一个 `clientMsgId` 重发**，服务端返回与第一次相同的 ack；同一个 `clientMsgId` 用于另一个会话时返回 4008。
5. 文本：服务端去掉首尾空白（Unicode `White_Space`）后，按码点计数，范围 1～5000；存储的是去空白后的内容。
6. 重连流程：先 `convs`，再对"服务端 maxSeq > 本地 maxSeq"的会话 `sync`；某个会话缺口超过 200 条时，直接加载最新一页（`beforeSeq = maxSeq + 1, limit = 20`）。
7. **实时下发的 `msg` 不保证按 `seq` 顺序到达**（多人同时发言时，投递和落库不在同一把锁里）。客户端必须按 `seq` 排序插入；发现缺口后等 500ms，还没补上就 `sync`。
8. 同一用户多端在线时，自己发出的消息会以 `msg` 下发到自己的**其他**端，发送的那个连接只收到 `ack`。
9. 权限：单聊非参与方 4032；群不存在 4043；非群成员 4033。单聊发送权限（PRD-social F-SOC-08 规则 7）按顺序判定：格式 → 去重命中直接返回第一次的 ack → 发送限流 4290 → 新对象（会话里还没有消息）预扣 `newPeerReject`，扣不到 4292 → 新对象且开了 `newPeerHourlyLimit`（官方账号发送方豁免）4292 → 对方不存在 4042 → 好友制下不互为好友（任一方官方账号豁免）4035 → 对方拉黑了我 4036 → 陌生人限额 4137（保存事务内判定）。给自己发永远允许。**客户端遇到不认识的事件类型必须忽略。**
11. 群提示消息：`type=tip`，由服务端生成、占用 seq、计入未读，`body.tip = {kind, actor, targets, count?, text}`；`kind` 为 `created`、`joined`、`ownerChanged`、`adminSet`、`adminUnset`、`renamed`、`noticeUpdated`，不认识的显示"[群通知]"。系统操作的 `from`、`actor` 为空字符串。提示消息下发给全部在线成员，包括操作者发起这次操作的连接（操作的 ack 里没有提示消息本身）。客户端不能发送 `type=tip`，`clientMsgId` 不能以 `tip-` 开头（4000）。权限不足 4030；操作对象不是成员 4046。建群失败时 `error.userId` 指出是哪个人。
10. 关系同步：每个用户一个 `ver`；修改类命令的 ack 带 `{ver, changed, items}`，`changed=true` 时 `items` 是本次改动的全部行；收到 `socialChanged {ver}` 且 `ver > 本地 ver` 时从本地 ver 开始一轮 `socialSync`，直到 `hasMore=false`；`restart=true` 时用同一个 `sinceVer` 重新开始；全量轮收齐后整体替换本地数据。

## 4. 变更规则

- 新增可选字段：向后兼容，直接加。
- 删除或改名字段、改变语义：协议版本号 +1，服务端同时兼容新旧两个版本，至少保留一个发版周期。
- 每次修改协议，都要同步更新 `vectors/` 里的用例；错误码只增不改（`action` 是给调用方的处理建议，改文案不算改码）。不认识的顶层 `cmd` 和不认识的事件类型、字段、枚举值一样，三套 SDK 都要忽略。
