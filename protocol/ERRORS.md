# 错误码规范

唯一真相源：[`errors.json`](errors.json)。服务端的 Go 常量、TS SDK 和 KMP SDK 的错误枚举都必须与它一致，由测试保证。本文是说明文档。

## 1. 编码规则

错误码是 4 位整数，**前三位借用 HTTP 状态码的语义**，第四位是细分序号：

| 区段 | 含义 | 客户端兜底处理（遇到不认识的错误码时） |
|---|---|---|
| 400x～401x 前段 | 请求参数 / 格式错误 | 不重试，记录日志 |
| 4010～4012 | 用户凭证问题 | 不重连，回登录页（例外：账号模式的 4011 先刷新，见第 3 节） |
| 4018、4019 | 账号密码、refresh token（只出现在 client 通道） | 不重试 |
| 4013、4017 | 管理接口凭证（旧密钥、调用方签名） | — |
| 4014～4016 | 后台会话（只出现在 console 通道） | 跳登录 / 设置页 |
| 403x | 无权限 | 不重试，提示无权限 |
| 404x | 资源不存在 | 不重试，提示不存在 |
| 409x | 冲突 | 不重试 |
| 413x | 数量超出上限（HTTP 状态用 400，沿用 4130、4131；4139 文件过大用 413） | 不重试 |
| 415x | 不支持的类型（HTTP 415，C2 起） | 不重试 |
| 422x | 业务状态不允许（C5 起） | 不重试 |
| 429x | 频率限制 | 稍后重试（例外：4291、4294 登录锁定、4292 配额用完、4293 @ 所有人次数用完不自动重试） |
| 451x | 内容治理（C6 起，HTTP 451） | 不重试 |
| 500x～503x | 服务端错误 | 可以重试 |
| 504x | 结果未知 | **不**自动重试（操作可能已经生效） |

400x、403x 已用满，之后的细分用 4000 / 4030 + `reason`。

## 2. 返回形式

| 通道 | 形式 |
|---|---|
| WebSocket 请求 | `{"id":"r1","cmd":"ack","error":{"code":4042,"msg":"用户不存在"}}` |
| 管理接口（HTTP） | HTTP 状态码取 `errors.json` 的 `http` 字段，响应体为 `{"code":4042,"msg":"用户不存在"}` |
| 客户端 HTTP（`/v1/*`，client） | HTTP 状态码取 `http` 字段，响应体 `{code, msg, reason?, details?}`；需要登录的接口用 `Authorization: Bearer`（PRD-chat 第 4.2 节） |
| 后台接口（console） | HTTP 状态码取 `http` 字段，响应体 `{code, msg, reason?, details?}`；`details` 是可选的扩展对象。可以透传任何 `admin` 通道的码，透传码不需要逐个加 `console`；只有 console-server 自己产生的码才写 `scope: console` |
| 凭证类致命错误（4010～4012） | 先下发 `{"cmd":"event","data":{"type":"kicked","reason":"tokenExpired"}}`，再用同号关闭码（如 4011）关闭连接 |
| 平台不合法（4009） | 不发 `kicked`，直接以关闭码 4009 关闭（开发期缺陷） |

`msg` 只用于日志和排查，**客户端展示给用户的文案要根据"命令 + `code`"映射**（PRD-social 9.4 节），未单独列出的命令用该码的默认文案（第 3 节"客户端处理"列），不要直接展示 `msg`。

| 命令 | code | 文案 |
|---|---|---|
| `send` | 4290 | 发送太频繁，请稍后再试 |
| 其他命令 | 4290 | 操作太频繁，请稍后再试 |
| `deviceNotificationRegister` | 4290 | 不提示（每账号每小时 10 次，下次回到前台再注册，C3） |
| `deviceNotificationRegister` | 4000 | 不提示（令牌为空或超过 4096 字节，开发期缺陷） |
| `send` | 4035 | 对方开启了好友验证，你还不是他的好友（附"发送好友申请"入口） |
| `send` | 4036 | 消息已发出，但被对方拒收了 |
| `send` | 4137 | 对方回复之前，最多只能发送 N 条消息（N 取最近一次同步 / 事件中的 `strangerMsgLimit`） |
| `send` | 4292 | 今天发起新聊天的次数已用完，请稍后再试 |
| `send` | 4293 | @所有人次数已用完（C4，消息标记失败，不自动重发） |
| `send` | 4040 | 引用的消息不可用（C4，`reason: quoteUnavailable`） |
| `send` | 4030 | 只有群主和管理员可以 @所有人（C4，`reason: mentionAllForbidden`；普通成员的客户端本来不显示"所有人"） |
| `recall` | 4097 | 按 `details.windowSeconds`：120 →「发送时间超过 2 分钟，无法撤回」；604800 →「发送时间超过 7 天，无法撤回」（C4） |
| `recall` | 4030 | 你没有权限撤回这条消息（C4；`reason: featureDisabled` 时隐藏入口） |
| `recall` | 4044 | 消息不存在或已被删除（C4） |
| `recall` | 4290 | 操作太频繁，请稍后再试（C4，每人每分钟 20 次） |
| `groupMembers`（带 `keyword`） | 4290 | 搜索太频繁，请稍后再试（C4，每人每分钟 30 次） |
| `convSet` | 4220 | 最多置顶 20 个会话（C5，文案固定，不带 `details`） |
| `convSet` | 4040 | 内容不存在（C5，没有消息的单聊） |
| `convSet` / `convDelete` | 4290 | 操作太频繁，请稍后再试（C5，每人每秒 5 次，两个命令共用） |
| `convSet` / `convDelete` | 4030 | 不提示，隐藏入口（C5，`reason: featureDisabled`） |
| `send` | 4510 | 消息包含不当内容，无法发送（C6，消息标记失败，不自动重发，输入框清空；不带 `details`） |
| 注册、改资料（`/v1/account/*`）/ `groupCreate` / `groupUpdate` / `friendApply` | 4510 | 内容包含不当信息，请修改后重试（C6，P-48 ⑥；`friendApply` 不管对方是否存在都是这个） |
| `send` | 4000 | 不提示，开发期缺陷（C6，`reason: customDisabled`：`IM_CLIENT_CUSTOM_MSG=0` 时客户端发 `custom`） |
| `report` | 4040 | 内容不存在（C6，默认文案；`reason: reportTargetInvalid`，几种原因响应相同） |
| `report` | 4292 | 今天的举报次数已用完（C6，RC2-20，每人每天 20 次） |
| `report` | 4131 | 不提示，开发期缺陷（C6，`msgIds` 超过 20 条；客户端第一批只举报单条） |
| `report` | 4030 | 不提示，隐藏入口（C6，`reason: featureDisabled`） |
| `friendApply` / `friendRespond` | 4132 | 你的好友数量已达上限 |
| `friendRespond` / `friendApply` | 4133 | 对方好友数量已达上限 |
| `friendRespond` | 4045 | 该申请已过期（刷新列表） |
| `block` | 4134 | 黑名单已满 |
| `groupCreate` / `groupInvite` | 4035 | {昵称} 不是你的好友，无法邀请（按 `error.userId` / `failed[].userId`） |
| `groupCreate` / `groupInvite` | 4135 | {昵称} 加入的群聊已达上限 |
| `groupTransfer` / `groupSetAdmin` | 4046 | 对方已不在群里（刷新成员列表） |
| `groupSetAdmin` | 4136 | 管理员最多 10 人 |
| 上传（`/v1/files/*`，C2） | 4292 | 今天的上传额度已用完 |
| `friendApply` / `userSearch` / `block` / `unblock` | 4292 | 今天的次数已用完，明天再试 |

**4293 同样不自动重试**（C4，PRD-chat F-MEN-02 规则 2）：不用 4292，避免和"今天发起新聊天的次数已用完"混淆。

**4292 是 429x 的例外**：`retryable=false`，带社交功能的 SDK 都认识它、不自动重试。老版本 SDK 只会在 `send` 上遇到 4292（新对象配额），按兜底重试同样返回 4292，受现有重试上限约束，不会放大。

## 3. 错误码一览

| code | 名称 | HTTP | 通道 | 可重试 | 含义 | 客户端处理 |
|---|---|---|---|---|---|---|
| 4000 | BAD_REQUEST | 400 | ws / admin / console / client | 否 | 参数不合法 | 开发期缺陷；提示「请求出错」 |
| 4001 | UNKNOWN_CMD | — | ws | 否 | 未知命令 | 提示升级客户端；`pushSync` 的 4001 由 SDK 静默忽略 |
| 4002 | INVALID_FRAME | — | ws | 否 | 帧格式错误 | 记录日志 |
| 4003 | MISSING_REQUEST_ID | — | ws | 否 | 请求缺少 id | 记录日志 |
| 4004 | INVALID_ID | 400 | ws / admin / console / client | 否 | ID 格式不合法 | 提示「账号格式不正确」 |
| 4005 | TEXT_EMPTY | — | ws | 否 | 消息内容为空 | 兜底提示「不能发送空白消息」 |
| 4006 | TEXT_TOO_LONG | — | ws | 否 | 消息超过 5000 字 | 提示「消息过长」 |
| 4007 | INVALID_CONV_ID | 400 | ws / admin / console | 否 | 会话 ID 不合法 | 记录日志 |
| 4008 | CLIENT_MSG_ID_CONFLICT | — | ws | 否 | clientMsgId 已用于其他会话 | 重新生成后再发 |
| 4009 | INVALID_PLATFORM | — | ws | 否 | 平台标识不合法 | 不重连 |
| 4010 | TOKEN_INVALID | 401 | ws / client | 否 | 登录凭证无效 | 不重连，提示「登录已失效」 |
| 4011 | TOKEN_EXPIRED | 401 | ws / client | 否 | 登录凭证已过期或是被替换的代（`reason: tokenSuperseded`） | 账号模式：先比较本地代号，本地更新就直接重试或重连，否则刷新一次，4019 才回登录页。演示模式：不重连，提示「登录已过期」 |
| 4012 | TOKEN_REVOKED | 401 | ws / client | 否 | 账号已被强制下线 | 不重连，提示「账号已被下线」 |
| 4013 | ADMIN_KEY_INVALID | 401 | admin | 否 | 管理密钥无效 | 业务服务端检查配置 |
| 4014 | CONSOLE_AUTH_REQUIRED | 401 | console | 否 | 没有有效的后台会话 | 跳登录页（前端只在这个码时跳登录页）；reason 见 PRD-admin F-CON-05 规则 6 |
| 4015 | CONSOLE_CREDENTIALS_INVALID | 401 | console | 否 | 账号、密码、动态码、恢复码或激活码错误 | 原地提示「账号、密码或动态码错误」，不跳转 |
| 4016 | CONSOLE_SETUP_REQUIRED | 401 | console | 否 | 会话处于首次设置状态（未绑定 TOTP） | 跳「绑定二次验证」页 |
| 4017 | ADMIN_SIGNATURE_INVALID | 401 | admin | 否 | 管理接口签名无效：缺签名头、调用方不存在或停用、签名错误、时间戳超出 5 分钟、nonce 重复、目标节点不符（PRD-admin F-KEY-03） | 业务服务端检查时钟、调用方密钥和签名 |
| 4018 | ACCOUNT_CREDENTIALS_INVALID | 401 | client | 否 | 用户名、密码或恢复码错误（不区分） | 提示「用户名或密码错误」 |
| 4019 | REFRESH_TOKEN_INVALID | 401 | client | 否 | refresh token 无效、过期、吊销或被重放 | 按 `reason`：`superseded` 留在当前页重试，其他回登录页（PRD-chat F-ACC-03 规则 6） |
| 4048 | FILE_NOT_FOUND | 404 | client、ws、admin | 否 | 文件不存在、无权访问、已过期或已撤回（不区分） | 提示「文件不存在或已过期」 |
| 4030 | FORBIDDEN | 403 | ws / admin / console / client | 否 | 无权限（通用） | 提示「无权限」 |
| 4031 | MUTED | — | ws | 否 | 被禁言，不能发言 | 提示禁言及解除时间（details.mutedUntil）；ack 的 error 带 details: {scope, groupId?, mutedUntil}；scope 是 user / group / groupAll（全群禁言，此时 mutedUntil 为 null，提示"群主已开启全员禁言"） |
| 4032 | NOT_CONV_PARTICIPANT | — | ws | 否 | 不是该单聊的参与者 | 记录日志 |
| 4033 | NOT_GROUP_MEMBER | — | ws | 否 | 不是群成员 | 提示「你已不在该群」，禁用输入框 |
| 4034 | USER_BANNED | 403 | admin / client | 否 | 用户被封禁，不能签发 token / 演示登录 / 账号登录（details.bannedUntil，永久为 null） | 业务方提示用户「账号已被封禁」 |
| 4035 | NOT_FRIEND | — | ws | 否 | 好友制下不是互为好友（`send`）；M4：建群 / 邀请对象不是好友、拉黑了我或不存在 | 见第 2 节映射表 |
| 4036 | REJECTED_BY_PEER | — | ws | 否 | 私聊被对方拉黑（只用于 `send`） | 消息标记失败，提示「消息已发出，但被对方拒收了」 |
| 4037 | CSRF_INVALID | 403 | console | 否 | CSRF 令牌缺失或错误，或 Origin 不匹配 | 重新取会话信息后重放一次，仍失败提示刷新 |
| 4038 | STEP_UP_REQUIRED | 403 | console | 否 | 该操作需要重新验证 | 弹 TOTP 框，通过后自动重放 |
| 4039 | MSG_READ_GRANT_REQUIRED | 403 | admin / console | 否 | 没有该会话有效的正文 / 隔离原文查看授权 | 弹「工单号 + 理由 + 动态码」框 |
| 4040 | NOT_FOUND | 404 | ws / admin / console | 否 | 资源不存在（通用）；`convSet`（C5）：没有消息的单聊；`report`（C6）：`reason: reportTargetInvalid` | 提示「内容不存在」 |
| 4041 | NOT_IN_ROOM | — | ws | 否 | 未加入聊天室（二期） | 重新进房 |
| 4042 | USER_NOT_FOUND | 404 | ws / admin | 否 | 用户不存在 | 提示「用户不存在」 |
| 4043 | GROUP_NOT_FOUND | 404 | ws / admin | 否 | 群不存在 | 提示「群聊不存在或已解散」 |
| 4044 | MESSAGE_NOT_FOUND | 404 | ws / admin / console | 否 | 消息不存在（或隔离原文已过期删除）；`recall`（C4）：不存在、属于其他会话、已被后台删除，不区分 | 提示「消息不存在或已过期」 |
| 4045 | FRIEND_REQUEST_NOT_FOUND | — | ws | 否 | 好友申请不存在、已过期（申请时被我拉黑过同样如此，响应体相同） | 提示「该申请已过期」并刷新列表 |
| 4046 | TARGET_NOT_GROUP_MEMBER | 404 | ws / admin | 否 | 操作对象不是群成员（M4：转让、设管理员、指定群主） | 提示「对方已不在群里」 |
| 4090 | GROUP_EXISTS | 409 | admin | 否 | 群 ID 已存在 | 换一个群 ID |
| 4091 | FRIEND_REQUEST_HANDLED | — | ws | 否 | 申请已同意，不能再拒绝 | 已经是好友，改用删除好友 |
| 4092 | CONSOLE_ACCOUNT_EXISTS | 409 | console | 否 | 同一账号空间内账号名已存在 | 换一个账号名 |
| 4093 | CONSOLE_ACCOUNT_STATE_CONFLICT | 409 | console | 否 | 操作会让自己失去权限、使空间内没有启用的空间超管，或违反职责分离 | 提示具体原因（按 reason） |
| 4094 | PUSH_ID_CONFLICT | 409 | admin | 否 | 同一 `pushId` 的调用方或内容不同 | 换一个 `pushId`；内容相同的重试不会得到这个错误 |
| 4095 | USERNAME_TAKEN | 409 | client | 否 | 用户名已存在或保留 | 提示「这个用户名不能使用」 |
| 4096 | FILE_NOT_READY | 409 | ws / client | 否 | 文件未完成上传，或已绑定其他消息 | 重新上传（F-FILE-07 规则 4） |
| 4097 | RECALL_WINDOW_EXPIRED | — | ws | 否 | 超过撤回时限（`details.windowSeconds`，C4） | 见第 2 节映射表 |
| 4098 | REPORT_ALREADY_HANDLED | 409 | admin / console | 否 | 举报已被处理（C6，两个审核员同时结案时后一个，状态不变） | 刷新列表 |
| 4130 | GROUP_MEMBER_LIMIT_EXCEEDED | 400 | ws / admin / console | 否 | 群成员已达上限 | 提示「群成员已达上限」 |
| 4131 | BATCH_LIMIT_EXCEEDED | 400 | ws / admin / console / client | 否 | 单次批量操作数量超限（client：`/v1/files/urls` 一次超过 50 项；ws：`send` 的 `mentions.userIds` 超过 `IM_MENTION_MAX`，C4；`report` 的 `msgIds` 超过 20 条，C6） | 分批调用 |
| 4132 | FRIEND_LIMIT_EXCEEDED | 400 | ws / admin | 否 | 自己的好友数已达上限；管理接口导入时 `msg` 写出 userId | 提示「你的好友数量已达上限」 |
| 4133 | PEER_FRIEND_LIMIT_EXCEEDED | — | ws | 否 | 对方的好友数已达上限 | 提示「对方好友数量已达上限」 |
| 4134 | BLOCK_LIMIT_EXCEEDED | — | ws | 否 | 黑名单已满 | 提示「黑名单已满」 |
| 4135 | JOINED_GROUP_LIMIT_EXCEEDED | — | ws | 否 | 自己或对方加入的群数已达上限（M4） | 提示「{昵称} 加入的群聊已达上限」 |
| 4136 | GROUP_ADMIN_LIMIT_EXCEEDED | — | ws | 否 | 管理员已满 10 人（M4） | 提示「管理员最多 10 人」 |
| 4137 | STRANGER_MSG_LIMIT | — | ws | 否 | 开放模式下对方回复前的条数用完 | 见第 2 节映射表 |
| 4139 | FILE_TOO_LARGE | 413 | client | 否 | 超过大小限制（`details.maxBytes`） | 提示「文件过大，最大 {N}」 |
| 4150 | FILE_TYPE_NOT_ALLOWED | 415 | client | 否 | 禁止的类型或内容与声明不符 | 提示「不支持发送这种文件」 |
| 4220 | PIN_LIMIT_EXCEEDED | — | ws | 否 | 置顶数量已满（C5，`IM_CONV_PIN_MAX`，不带 `details`） | 提示「最多置顶 20 个会话」 |
| 4290 | RATE_LIMITED | 429 | ws / admin / console / client | 是 | 操作过于频繁（C5 起含会话设置 `convSet`、`convDelete`，每人每秒 5 次） | 提示「发送太频繁」，1 秒后可重试 |
| 4291 | CONSOLE_LOGIN_LOCKED | 429 | console | 否 | 登录失败过多，该账号名（或账号名 + IP）暂时锁定 | 提示「尝试次数过多，请 N 分钟后再试」，不自动重试 |
| 4292 | QUOTA_EXCEEDED | 429 | ws / admin / console / client | **否** | 配额用完：每日（好友申请、搜索、拉黑、新对象被拒、举报（C6，每人每天 20 次））或每小时（新对象） | 见第 2 节映射表，不自动重试 |
| 4293 | MENTION_ALL_LIMITED | — | ws | **否** | @ 所有人次数用完（`details.retryAfterSeconds`，C4） | 提示「@所有人次数已用完」，不自动重发 |
| 4294 | ACCOUNT_LOGIN_LOCKED | 429 | client | **否** | 尝试过多，暂时锁定（`details.retryAfterSeconds`） | 提示「尝试次数过多，请 N 分钟后再试」，不自动重试 |
| 4510 | CONTENT_BLOCKED | 451 | ws / client | 否 | 内容被拦截（C6，PRD-chat F-MOD-02）：消息不保存、内容未写入；**不带 `details`**，不告诉命中了哪个词 | 见第 2 节映射表：`send`「消息包含不当内容，无法发送」，其他「内容包含不当信息，请修改后重试」 |
| 5000 | INTERNAL | 500 | ws / admin / console / client | 是 | 服务器内部错误 | 消息标记失败，允许重发 |
| 5030 | SERVICE_UNAVAILABLE | 503 | ws / admin / console / client | 是 | 服务暂不可用 | 按退避策略重试 |
| 5031 | AUDIT_UNAVAILABLE | 503 | console | 是 | 审计写入失败，操作未执行 | 提示「审计服务不可用，操作未执行，请稍后重试」 |
| 5032 | OBJECT_STORE_UNAVAILABLE | 503 | client | 是 | 对象存储暂不可用 | 退避重试；`complete` 的 5032 SDK 退避重试 3 次再转失败 |
| 5040 | UPSTREAM_RESULT_UNKNOWN | 504 | console | 否 | 调用 im-server 超时，操作结果未知 | 提示「操作结果未知，请刷新确认，不要重复提交」；`pushId` 幂等的接口由对账任务重放 |

### 3.1 `reason`（PRD-chat 第 11.5 节，C1、C2、C4、C6 部分）

同一个错误码按 `reason` 细分。`errors.json` 的 `reasons` 是真相源，测试保证和 Go 常量一致。**客户端不认识的 `reason` 按这个码的默认处理**。

| code | reason | 含义 |
|---|---|---|
| 4000 | `passwordWeak` | 密码不符合规则（长度、等于用户名、全相同字符、弱口令表） |
| 4000 | `recoveryCodeStale` | 确认的不是最新一代恢复码，提示"恢复码已更新，请重新登录查看" |
| 4000 | `fileKindMismatch` | `send` 的 `type` 和文件的 `kind` 不符（C2） |
| 4000 | `uploadMismatch` | `complete` 时对象大小和声明不符，或校验期间对象被替换（C2） |
| 4000 | `fileNameTooLong` | 清洗后的文件名超过 255 字节（C2） |
| 4000 | `customDisabled` | `IM_CLIENT_CUSTOM_MSG=0`（生产默认）时客户端 `send` 的 `type=custom`，消息不保存（C6，F-MOD-02 规则 4）；只影响客户端 `send` |
| 4011 | `tokenSuperseded` | access token 是被替换的代（只在 `/v1/*`；WS 是 `kicked/tokenExpired`） |
| 4019 | `invalid` | `mac` 错误或格式不对。**只有 `mac` 正确才返回下面这些**（判定顺序见 F-ACC-03 规则 4） |
| 4019 | `expired` / `revoked` | 已过期 / 已吊销（自己退出、强制下线、封禁、`sessions revoke-all`） |
| 4019 | `superseded` | 重叠窗口内的旧值，`retryAfterSeconds` 后重读本地 token 重试，最多 3 次 |
| 4019 | `replayDetected` | 重放，设备会话已吊销，之后再来也是它 |
| 4019 | `credentialsChanged` / `loggedOutOthers` / `accountDeleted` | 按吊销原因 |
| 4030 | `registerClosed` | `IM_REGISTER_MODE=closed` |
| 4030 | `inviteInvalid` | 邀请码缺失、错误、过期、用完、停用，应答完全相同 |
| 4030 | `accountSessionRequired` | 没有 `sid` 的 token 调用账号接口 |
| 4030 | `accountExists` | 有账号的用户调用 `/demo/login` |
| 4030 | `accountDeletionPending` | 注销冷静期内密码正确（`details.deleteAt`） |
| 4030 | `featureDisabled` | 新命令的开关关闭（C2 起用；C3 的 `deviceNotificationRegister`、`deviceNotificationUnregister`、`deviceNotificationSettings`、`presence` 同样；C4 的 `recall`，以及 `send` 带 `quote`、`mentions`；C5 的 `convSet`、`convDelete`；C6 的 `report`（`IM_MOD_ENABLED` 关闭）） |
| 4030 | `mentionAllForbidden` | 普通成员 `mentions.all=true`，或托管群没有群主（C4，F-MEN-02 规则 1），消息不保存 |
| 4040 | `reportTargetInvalid` | 举报对象不存在、不是参与者、看不到这些消息、已撤回但没读到、和对象没有关联、不是群成员，响应完全相同（C6，F-MOD-03 规则 2、2a、3，防止探测） |
| 4040 | `quoteUnavailable` | 引用的消息在其他会话、不存在、已撤回、已删除、是 `tip` 或在发送者入群之前，响应完全相同（C4，F-QUO-01 规则 2），消息不保存 |
| 4290 | `recoverLimited` | 用最新一代恢复码成功 `recover` 超过每小时次数（**PRD 第 11.5 节有列，F-ACC-05 规则 2**） |
| 4290 | `uploadPendingLimit` | 未完成上传数超限（C2），SDK 不自动重试，提示"上传任务太多，请稍后再试" |
| 5032 | `completeInProgress` | 同一上传的 `complete` 正在进行（租约），可以重试（C2） |


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

`kicked` 事件的原因与关闭码对应：`tokenInvalid` → 4010，`tokenExpired` → 4011，`banned` → 4012，`passwordChanged`、`sessionRevoked`、`accountDeleted` → 4012（可选 `recoverBefore`，PRD-chat F-ACC-03 规则 6a），`suspended`（封禁，带 until，第二批）→ 4012，`tooManyConnections` → 4429，`serverShutdown` → 1001（可以重连），`replaced` 为保留值（一期不实现）。

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
