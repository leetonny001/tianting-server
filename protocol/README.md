# IM 协议（v1）

协议的唯一真相源，对应 [PRD v1.2](../docs/prd/PRD-core-v1.2.md)。服务端和三套 SDK（[tianting-sdk-ts](https://github.com/leetonny001/tianting-sdk-ts)、tianting-sdk-kotlin、tianting-sdk-swift）都以这里为准，并跑同一批用例。SDK 仓库里放的是副本，协议只在这里改，改完通知各 SDK 仓库运行 `scripts/sync-protocol.sh`。

| 文件 | 内容 |
|---|---|
| `schema/frame.schema.json` | WebSocket 帧的 JSON Schema（2020-12） |
| `vectors/frames.json` | 单帧校验用例：`valid` 必须通过，`invalid` 必须拒绝 |
| `errors.json` / [ERRORS.md](ERRORS.md) | 错误码、关闭码、被踢原因 |
| [ADMIN.md](ADMIN.md) | 管理接口（给业务服务端） |

## 1. 连接

```
ws(s)://{host}/ws?platform={android|ios|web|pc}
→ 第一帧：{"id":"a1","cmd":"auth","data":{"token":"<jwt>"}}
← {"id":"a1","cmd":"ack","data":{"userId":"alice","serverTime":1700000000000}}
```

- **token 不放在 URL 里**（会被代理、CDN、浏览器历史记录），连接后第一帧用 `auth` 发送（PRD C-52）。收到 auth 的 ack 才算上线。
- 5 秒内没有发 `auth`，或第一帧不是 `auth`：以 4401 关闭（可以重连），不发 `kicked`。
- token 由业务服务端调用 `POST /admin/users/token` 获取（对应腾讯 IM 的 userSig）。
- 凭证问题：下发 `{"cmd":"event","data":{"type":"kicked","reason":"tokenInvalid|tokenExpired|banned"}}`，再以 4010 / 4011 / 4012 关闭（检查顺序 4010 → 4012 → 4011）。
- platform 缺失或不合法：握手后立即以 4009 关闭，不发 `kicked`（最先检查）。
- 浏览器的 Origin 必须在白名单内（`IM_ALLOWED_ORIGINS`），否则握手返回 HTTP 403。
- 同一用户最多 10 个连接，超出时最早的连接收到 `kicked/tooManyConnections` 并以 4429 关闭（不重连）。
- 服务端下线前会下发 `kicked/serverShutdown` 并以 1001 关闭：客户端**按退避重连**。
- 每个 WebSocket 消息是一帧 JSON（text frame），一帧一个对象。上行单帧最大 64KB；下行不限。

## 2. 命令

| 命令 | 方向 | 用途 | ack 数据 |
|---|---|---|---|
| `auth` | 上行 | 鉴权，必须是第一帧；成功后再发返回 4000 | `{userId, serverTime}` |
| `send` | 上行 | 发消息 | `{msgId, convId, seq, ts}` |
| `sync` | 上行 | 拉消息：`afterSeq`（补拉之后的）或 `beforeSeq`（翻页拉之前的），二选一 | `{messages, hasMore}`，按 seq 升序 |
| `convs` | 上行 | 拉会话列表：按 `updatedAt` 倒序、`convId` 升序，最多 500 个 | `{convs, truncated}` |
| `read` | 上行 | 上报已读位置 | `{}` |
| `ping` | 上行 | 心跳 | 回 `pong`（不是 ack） |
| `msg` | 下行 | 新消息 | — |
| `event` | 下行 | `kicked`；`convAdded`（被拉进群，内容与 convs 的一项相同）、`convRemoved`（被移出群）；`readUpdated`（自己在其他端的已读位置前进了，包括发言即已读）；`socialChanged {ver}`（关系数据变了，只发给 ver 变化的用户，排除发起连接）、`socialSettingsChanged {mode, strangerMsgLimit}`（业务方改了社交配置） | — |
| `friendApply` / `friendRespond` / `friendDelete` / `friendRemark` | 上行 | 好友申请、处理申请、单向删除、备注（PRD-social F-SOC-02、03、05） | `{status?, ver, changed, items}` |
| `block` / `unblock` | 上行 | 拉黑、取消拉黑（F-SOC-07） | `{ver, changed, items}` |
| `friendRequestsSeen` | 上行 | "新的朋友"标记已读（F-SOC-04） | `{requestSeenVer, ver, changed}` |
| `socialSync` | 上行 | 关系同步：第一页 `{sinceVer}`，之后 `{cursor}`（F-SOC-06） | `{full, untilVer, latestVer, hasMore, cursor, restart?, friends, requests, blocks, requestSeenVer, settings, mode, strangerMsgLimit, official}` |
| `userSearch` / `setPrivacy` | 上行 | 精确搜索 userId；"可被搜索"开关（F-SOC-09） | `{user}`；`{ver, changed}` |
| `users` / `myGroups` | 上行 | 批量查资料（只返回可见范围内的人）；我的群聊分页（F-SOC-10、11） | `{users}`；`{groups, nextCursor}` |
| `joinRoom` / `leaveRoom` / `push` | — | 二期（聊天室、系统推送） | — |

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
10. 关系同步：每个用户一个 `ver`；修改类命令的 ack 带 `{ver, changed, items}`，`changed=true` 时 `items` 是本次改动的全部行；收到 `socialChanged {ver}` 且 `ver > 本地 ver` 时从本地 ver 开始一轮 `socialSync`，直到 `hasMore=false`；`restart=true` 时用同一个 `sinceVer` 重新开始；全量轮收齐后整体替换本地数据。

## 4. 变更规则

- 新增可选字段：向后兼容，直接加。
- 删除或改名字段、改变语义：协议版本号 +1，服务端同时兼容新旧两个版本，至少保留一个发版周期。
- 每次修改协议，都要同步更新 `vectors/` 里的用例；错误码只增不改。
