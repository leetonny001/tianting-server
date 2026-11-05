# 错误码规范

唯一真相源：[`errors.json`](errors.json)。服务端的 Go 常量、TS SDK 和 KMP SDK 的错误枚举都必须与它一致，由测试保证。本文是说明文档。

## 1. 编码规则

错误码是 4 位整数，**前三位借用 HTTP 状态码的语义**，第四位是细分序号：

| 区段 | 含义 | 客户端兜底处理（遇到不认识的错误码时） |
|---|---|---|
| 400x～401x 前段 | 请求参数 / 格式错误 | 不重试，记录日志 |
| 4010～4012 | 用户凭证问题 | 不重连，回登录页 |
| 4013、4017 | 管理接口凭证（旧密钥、调用方签名） | — |
| 4014～4016 | 后台会话（只出现在 console 通道） | 跳登录 / 设置页 |
| 403x | 无权限 | 不重试，提示无权限 |
| 404x | 资源不存在 | 不重试，提示不存在 |
| 409x | 冲突 | 不重试 |
| 413x | 数量超出上限（HTTP 状态用 400，沿用 4130、4131） | 不重试 |
| 429x | 频率限制 | 稍后重试（例外：4291 登录锁定、4292 配额用完不自动重试） |
| 500x～503x | 服务端错误 | 可以重试 |
| 504x | 结果未知 | **不**自动重试（操作可能已经生效） |

400x、403x 已用满，之后的细分用 4000 / 4030 + `reason`。

## 2. 返回形式

| 通道 | 形式 |
|---|---|
| WebSocket 请求 | `{"id":"r1","cmd":"ack","error":{"code":4042,"msg":"用户不存在"}}` |
| 管理接口（HTTP） | HTTP 状态码取 `errors.json` 的 `http` 字段，响应体为 `{"code":4042,"msg":"用户不存在"}` |
| 后台接口（console） | HTTP 状态码取 `http` 字段，响应体 `{code, msg, reason?, details?}`；`details` 是可选的扩展对象。可以透传任何 `admin` 通道的码，透传码不需要逐个加 `console`；只有 console-server 自己产生的码才写 `scope: console` |
| 凭证类致命错误（4010～4012） | 先下发 `{"cmd":"event","data":{"type":"kicked","reason":"tokenExpired"}}`，再用同号关闭码（如 4011）关闭连接 |
| 平台不合法（4009） | 不发 `kicked`，直接以关闭码 4009 关闭（开发期缺陷） |

`msg` 只用于日志和排查，**客户端展示给用户的文案要根据"命令 + `code`"映射**（PRD-social 9.4 节），未单独列出的命令用该码的默认文案（第 3 节"客户端处理"列），不要直接展示 `msg`。

| 命令 | code | 文案 |
|---|---|---|
| `send` | 4290 | 发送太频繁，请稍后再试 |
| 其他命令 | 4290 | 操作太频繁，请稍后再试 |
| `send` | 4035 | 对方开启了好友验证，你还不是他的好友（附"发送好友申请"入口） |
| `send` | 4036 | 消息已发出，但被对方拒收了 |
| `send` | 4137 | 对方回复之前，最多只能发送 N 条消息（N 取最近一次同步 / 事件中的 `strangerMsgLimit`） |
| `send` | 4292 | 今天发起新聊天的次数已用完，请稍后再试 |
| `friendApply` / `friendRespond` | 4132 | 你的好友数量已达上限 |
| `friendRespond` / `friendApply` | 4133 | 对方好友数量已达上限 |
| `friendRespond` | 4045 | 该申请已过期（刷新列表） |
| `block` | 4134 | 黑名单已满 |
| `groupCreate` / `groupInvite` | 4035 | {昵称} 不是你的好友，无法邀请（按 `error.userId` / `failed[].userId`） |
| `groupCreate` / `groupInvite` | 4135 | {昵称} 加入的群聊已达上限 |
| `groupTransfer` / `groupSetAdmin` | 4046 | 对方已不在群里（刷新成员列表） |
| `groupSetAdmin` | 4136 | 管理员最多 10 人 |
| `friendApply` / `userSearch` / `block` / `unblock` | 4292 | 今天的次数已用完，明天再试 |

**4292 是 429x 的例外**：`retryable=false`，带社交功能的 SDK 都认识它、不自动重试。老版本 SDK 只会在 `send` 上遇到 4292（新对象配额），按兜底重试同样返回 4292，受现有重试上限约束，不会放大。

## 3. 错误码一览

| code | 名称 | HTTP | 通道 | 可重试 | 含义 | 客户端处理 |
|---|---|---|---|---|---|---|
| 4000 | BAD_REQUEST | 400 | ws / admin / console | 否 | 参数不合法 | 开发期缺陷；提示「请求出错」 |
| 4001 | UNKNOWN_CMD | — | ws | 否 | 未知命令 | 提示升级客户端；`pushSync` 的 4001 由 SDK 静默忽略 |
| 4002 | INVALID_FRAME | — | ws | 否 | 帧格式错误 | 记录日志 |
| 4003 | MISSING_REQUEST_ID | — | ws | 否 | 请求缺少 id | 记录日志 |
| 4004 | INVALID_ID | 400 | ws / admin / console | 否 | ID 格式不合法 | 提示「账号格式不正确」 |
| 4005 | TEXT_EMPTY | — | ws | 否 | 消息内容为空 | 兜底提示「不能发送空白消息」 |
| 4006 | TEXT_TOO_LONG | — | ws | 否 | 消息超过 5000 字 | 提示「消息过长」 |
| 4007 | INVALID_CONV_ID | 400 | ws / admin / console | 否 | 会话 ID 不合法 | 记录日志 |
| 4008 | CLIENT_MSG_ID_CONFLICT | — | ws | 否 | clientMsgId 已用于其他会话 | 重新生成后再发 |
| 4009 | INVALID_PLATFORM | — | ws | 否 | 平台标识不合法 | 不重连 |
| 4010 | TOKEN_INVALID | — | ws | 否 | 登录凭证无效 | 不重连，提示「登录已失效」 |
| 4011 | TOKEN_EXPIRED | — | ws | 否 | 登录凭证已过期 | 不重连，提示「登录已过期」 |
| 4012 | TOKEN_REVOKED | — | ws | 否 | 账号已被强制下线 | 不重连，提示「账号已被下线」 |
| 4013 | ADMIN_KEY_INVALID | 401 | admin | 否 | 管理密钥无效 | 业务服务端检查配置 |
| 4014 | CONSOLE_AUTH_REQUIRED | 401 | console | 否 | 没有有效的后台会话 | 跳登录页（前端只在这个码时跳登录页）；reason 见 PRD-admin F-CON-05 规则 6 |
| 4015 | CONSOLE_CREDENTIALS_INVALID | 401 | console | 否 | 账号、密码、动态码、恢复码或激活码错误 | 原地提示「账号、密码或动态码错误」，不跳转 |
| 4016 | CONSOLE_SETUP_REQUIRED | 401 | console | 否 | 会话处于首次设置状态（未绑定 TOTP） | 跳「绑定二次验证」页 |
| 4017 | ADMIN_SIGNATURE_INVALID | 401 | admin | 否 | 管理接口签名无效：缺签名头、调用方不存在或停用、签名错误、时间戳超出 5 分钟、nonce 重复、目标节点不符（PRD-admin F-KEY-03） | 业务服务端检查时钟、调用方密钥和签名 |
| 4030 | FORBIDDEN | 403 | ws / admin / console | 否 | 无权限（通用） | 提示「无权限」 |
| 4031 | MUTED | — | ws | 否 | 被禁言，不能发言 | 提示禁言及解除时间（details.mutedUntil）；ack 的 error 带 details: {scope, groupId?, mutedUntil} |
| 4032 | NOT_CONV_PARTICIPANT | — | ws | 否 | 不是该单聊的参与者 | 记录日志 |
| 4033 | NOT_GROUP_MEMBER | — | ws | 否 | 不是群成员 | 提示「你已不在该群」，禁用输入框 |
| 4034 | USER_BANNED | 403 | admin | 否 | 用户被封禁，不能签发 token / 演示登录（details.bannedUntil，永久为 null） | 业务方提示用户「账号已被封禁」 |
| 4035 | NOT_FRIEND | — | ws | 否 | 好友制下不是互为好友（`send`）；M4：建群 / 邀请对象不是好友、拉黑了我或不存在 | 见第 2 节映射表 |
| 4036 | REJECTED_BY_PEER | — | ws | 否 | 私聊被对方拉黑（只用于 `send`） | 消息标记失败，提示「消息已发出，但被对方拒收了」 |
| 4037 | CSRF_INVALID | 403 | console | 否 | CSRF 令牌缺失或错误，或 Origin 不匹配 | 重新取会话信息后重放一次，仍失败提示刷新 |
| 4038 | STEP_UP_REQUIRED | 403 | console | 否 | 该操作需要重新验证 | 弹 TOTP 框，通过后自动重放 |
| 4039 | MSG_READ_GRANT_REQUIRED | 403 | admin / console | 否 | 没有该会话有效的正文 / 隔离原文查看授权 | 弹「工单号 + 理由 + 动态码」框 |
| 4040 | NOT_FOUND | 404 | ws / admin / console | 否 | 资源不存在（通用） | 提示「内容不存在」 |
| 4041 | NOT_IN_ROOM | — | ws | 否 | 未加入聊天室（二期） | 重新进房 |
| 4042 | USER_NOT_FOUND | 404 | ws / admin | 否 | 用户不存在 | 提示「用户不存在」 |
| 4043 | GROUP_NOT_FOUND | 404 | ws / admin | 否 | 群不存在 | 提示「群聊不存在或已解散」 |
| 4044 | MESSAGE_NOT_FOUND | 404 | admin / console | 否 | 消息不存在（或隔离原文已过期删除） | 提示「消息不存在或已过期」 |
| 4045 | FRIEND_REQUEST_NOT_FOUND | — | ws | 否 | 好友申请不存在、已过期（申请时被我拉黑过同样如此，响应体相同） | 提示「该申请已过期」并刷新列表 |
| 4046 | TARGET_NOT_GROUP_MEMBER | 404 | ws / admin | 否 | 操作对象不是群成员（M4：转让、设管理员、指定群主） | 提示「对方已不在群里」 |
| 4090 | GROUP_EXISTS | 409 | admin | 否 | 群 ID 已存在 | 换一个群 ID |
| 4091 | FRIEND_REQUEST_HANDLED | — | ws | 否 | 申请已同意，不能再拒绝 | 已经是好友，改用删除好友 |
| 4092 | CONSOLE_ACCOUNT_EXISTS | 409 | console | 否 | 同一账号空间内账号名已存在 | 换一个账号名 |
| 4093 | CONSOLE_ACCOUNT_STATE_CONFLICT | 409 | console | 否 | 操作会让自己失去权限、使空间内没有启用的空间超管，或违反职责分离 | 提示具体原因（按 reason） |
| 4130 | GROUP_MEMBER_LIMIT_EXCEEDED | 400 | ws / admin / console | 否 | 群成员已达上限 | 提示「群成员已达上限」 |
| 4131 | BATCH_LIMIT_EXCEEDED | 400 | admin / console | 否 | 单次批量操作数量超限 | 分批调用 |
| 4132 | FRIEND_LIMIT_EXCEEDED | 400 | ws / admin | 否 | 自己的好友数已达上限；管理接口导入时 `msg` 写出 userId | 提示「你的好友数量已达上限」 |
| 4133 | PEER_FRIEND_LIMIT_EXCEEDED | — | ws | 否 | 对方的好友数已达上限 | 提示「对方好友数量已达上限」 |
| 4134 | BLOCK_LIMIT_EXCEEDED | — | ws | 否 | 黑名单已满 | 提示「黑名单已满」 |
| 4135 | JOINED_GROUP_LIMIT_EXCEEDED | — | ws | 否 | 自己或对方加入的群数已达上限（M4） | 提示「{昵称} 加入的群聊已达上限」 |
| 4136 | GROUP_ADMIN_LIMIT_EXCEEDED | — | ws | 否 | 管理员已满 10 人（M4） | 提示「管理员最多 10 人」 |
| 4137 | STRANGER_MSG_LIMIT | — | ws | 否 | 开放模式下对方回复前的条数用完 | 见第 2 节映射表 |
| 4290 | RATE_LIMITED | 429 | ws / admin / console | 是 | 操作过于频繁 | 提示「发送太频繁」，1 秒后可重试 |
| 4291 | CONSOLE_LOGIN_LOCKED | 429 | console | 否 | 登录失败过多，该账号名（或账号名 + IP）暂时锁定 | 提示「尝试次数过多，请 N 分钟后再试」，不自动重试 |
| 4292 | QUOTA_EXCEEDED | 429 | ws / admin / console | **否** | 配额用完：每日（好友申请、搜索、拉黑、新对象被拒）或每小时（新对象） | 见第 2 节映射表，不自动重试 |
| 5000 | INTERNAL | 500 | ws / admin / console | 是 | 服务器内部错误 | 消息标记失败，允许重发 |
| 5030 | SERVICE_UNAVAILABLE | 503 | ws / admin / console | 是 | 服务暂不可用 | 按退避策略重试 |
| 5031 | AUDIT_UNAVAILABLE | 503 | console | 是 | 审计写入失败，操作未执行 | 提示「审计服务不可用，操作未执行，请稍后重试」 |
| 5040 | UPSTREAM_RESULT_UNKNOWN | 504 | console | 否 | 调用 im-server 超时，操作结果未知 | 提示「操作结果未知，请刷新确认，不要重复提交」；`pushId` 幂等的接口由对账任务重放 |

## 4. WebSocket 关闭码

| code | 名称 | 重连 | 含义 |
|---|---|---|---|
| 1000 | NORMAL | 否 | 客户端主动退出登录 |
| 1001 | GOING_AWAY | 是 | 服务端重启或下线 |
| 1009 | MESSAGE_TOO_BIG | 是 | 单帧超过 64KB（客户端缺陷） |
| 1011 | INTERNAL_ERROR | 是 | 服务端内部错误（如握手阶段查询吊销状态失败） |
| 4009～4012 | 同错误码 | 否 | 平台不合法 / 凭证无效 / 过期 / 被强制下线 |
| 4429 | TOO_MANY_CONNECTIONS | 否 | 同一用户连接数超过上限（默认 10），最早的连接被踢 |
| 4400 | SLOW_CONSUMER | 是 | 客户端读取太慢，下行队列积压 |
| 4401 | AUTH_REQUIRED | 是 | 连接后 5 秒内未发送 `auth`，或在 `auth` 前发送了其他命令（C-52） |
| 4408 | IDLE_TIMEOUT | 是 | 超过空闲时间未收到任何帧 |
| 其他（含 1006 异常断开） | — | 是 | 按退避策略重连 |

`kicked` 事件的原因与关闭码对应：`tokenInvalid` → 4010，`tokenExpired` → 4011，`banned` → 4012，`suspended`（封禁，带 until，第二批）→ 4012，`tooManyConnections` → 4429，`serverShutdown` → 1001（可以重连），`replaced` 为保留值（一期不实现）。

## 5. 与 M1 实现的差异（由后端任务修正）

| M1 现状 | 规范 |
|---|---|
| 非成员、非参与方统一返回 4030 | 分为 4033（不是群成员）和 4032（不是单聊参与方） |
| 群不存在靠"成员为空"来判断，返回 4040 | 返回 4043 |
| 建群冲突的响应体里 code 是 4000 | 返回 4090 |
| 参数类错误全部用 4000 | 按 4002～4008 细分 |
| token 无效时返回 HTTP 401，不升级 | 先升级，再发 `kicked` 并用关闭码 4010 关闭 |
| 管理密钥错误返回 4010 | 返回 4013 |
| 空闲超时、慢连接使用库默认的关闭码 | 使用 4408 / 4400 |
| 管理接口在对外端口 | 迁到 `IM_ADMIN_ADDR` |
| 旧密钥可调用全部接口 | 只能调用 v1.2 已有的接口 |

## 6. 变更规则

- **只增不改**：已发布的错误码不能改含义、不能复用。
- 新增错误码时同步更新 `errors.json`、本文档和三端枚举，由一致性测试保证。
- 客户端遇到不认识的错误码，按第 1 节的区段兜底处理。
