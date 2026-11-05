# 前端（SDK）任务单：社交关系与群聊补缺

| 项 | 内容 |
|---|---|
| 依据 | [PRD-social v1.0](../prd/PRD-social-v1.0.md) 第 10 节（SDK 变更）、9.1～9.4 节（协议与错误码）、[SDK 设计说明](../sdk/README.md)、[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md) |
| 负责 | 前端（AI） |
| 仓库 | FE-100～FE-129：tianting-sdk-ts（`/Users/frank/Desktop/workspace/tianting-sdk-ts`，M3 / M4）；FE-130～FE-139：tianting-sdk-kotlin（M5）；FE-140～FE-149：tianting-sdk-swift（M6） |
| 范围 | 只含 SDK。社交与群管理**页面**属于 PRD-ui（H5 `tianting-web`、PC `tianting-desktop`、原生 App），由 PRD-ui 任务单负责；本任务单中【前端】AC 的 SDK 部分（数据、事件、错误码、缓存）在这里完成，界面断言在 PRD-ui 任务里完成 |
| 开工条件 | 后端 BE-105（M3 协议定稿）完成；M4 部分在 BE-132（tip 与 `Error.userId` 协议）完成后开工；原生 SDK 首版（M5 / M6）直接包含 M3 + M4 全部社交接口（PRD 10.4 节） |
| 完成定义 | 任务内列出的 AC 的 SDK 部分有通过的测试（假传输层 + 假时钟单测、对真实 im-server 的集成测试）；`protocol/vectors/social.json`、`errors.json` 一致性测试三套 SDK 全过；sdk-ts `npm test` 全过；Kotlin `./gradlew test`、Swift `swift test` 全过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| FE-100 ～ FE-103（sdk-ts，M3） | ✅ 2026-11-04 | tianting-sdk-ts：`SocialState` 纯状态机（与服务端 Go 参考客户端逐条对应，跑 `vectors/social.json`）、同步轮驱动（连接后先 `convs`、200ms 去抖、`restart`、ack 不连续补轮、4290 后 6 秒重试）、`friends` / `friendRequests` / `blocks` / `socialSettings` 快照与事件、好友 / 拉黑 / 搜索 / 隐私命令、`relationOf`、资料缓存（10 分钟，同一事件循环合并、每批 100，没返回的人 10 分钟内不重查）、`loadMyGroups`、`messageFor`、`refreshSocial`。单元测试 142 个、集成测试 9 个 |
| FE-110 ～ FE-111（sdk-ts，M4） | ✅ 2026-11-04 | tianting-sdk-ts `9c1ae4d`：群接口、`IMError.userId`（同时解析 `details.userId`）、`tip` 消息、`tipText`（`vectors/social.json` 的 `tips` 用例）、`groupChanged` 合并在途请求并把 `GroupInfo` 带给界面。与任务描述的差异：提示消息的会话摘要先由 SDK 按服务端同一规则本地生成，下次 `convs` 时覆盖，不在每条提示后重拉 `convs`（大群里一次操作会让所有在线成员同时请求）。 |
| FE-130 ～ FE-133（sdk-kotlin，M5） | ✅ 2026-11-05 | `4fe4934`。FE-131 写的 Room 改成自有 `LocalDatabase` 接口（核心模块保持纯 JVM，JVM 和 Android 执行同一份 SQL）。未做：Android 上杀进程的仪器测试（JVM 重开文件和真实 server 冷启动做了）、ktlint / detekt、Maven 发布 |
| FE-140 ～ FE-143（sdk-swift，M6） | ✅ 2026-11-05 | `4815019`。FE-141 写的 GRDB 改成系统 SQLite3。未验证：真实弱网和前后台切换、App 挂起恢复、主线程写库的真机性能 |

## 总览

| 阶段 | 任务（人日） | 人日 | 说明 |
|---|---|---|---|
| T1 sdk-ts 社交（M3） | FE-100（1）、FE-101（2）、FE-102（1）、FE-103（1） | 5 | 协议类型与错误映射、社交同步引擎、好友 / 拉黑 / 搜索 API 与红点、资料缓存与通讯录 |
| T2 sdk-ts 群（M4） | FE-110（1.5）、FE-111（1） | 2.5 | 群 API、提示消息与群事件 |
| K sdk-kotlin（M5） | FE-130（1）、FE-131（2）、FE-132（1.5）、FE-133（1.5） | 6 | 首版包含 M3 + M4 社交 |
| W sdk-swift（M6） | FE-140（1）、FE-141（2）、FE-142（1.5）、FE-143（1.5） | 6 | 首版包含 M3 + M4 社交 |

**人日合计与 PRD 第 15 节对照**

| 里程碑 | 本任务单 | PRD 第 15 节 | 说明 |
|---|---|---|---|
| M3 | 5（sdk-ts） | sdk-ts 5 | 一致 |
| M4 | 2.5（sdk-ts） | sdk-ts 2.5 | 一致 |
| M5 / M6 | 12（Kotlin 6 + Swift 6） | 原生各 6 | 一致；含在 M5 / M6 首版内，不单独排期 |

后端人日见 [backend-social.md](backend-social.md)（M3 26.5、M4 12.5、解散群 1.5 计入 PRD-admin）。三份合计：M3 31.5、M4 15、M5 / M6 12；PRD 15 节的 H5 页面（M3 6 / M4 4.5）、文档（M3 2.5 / M4 1）、TC-SOC 编写（M3 1.5 / M4 1）不在这两份任务单里。

**依赖顺序**：BE-105 → FE-100 → FE-101 → FE-102、FE-103；BE-132 → FE-110 → FE-111。原生：FE-130 → FE-131 → FE-132 → FE-133（Swift 同序）；原生任务以 sdk-ts 的行为为基准，跑同一份 `vectors/social.json`。

**与其他 PRD 的交接**

| 依赖 | 用在 | 说明 |
|---|---|---|
| PRD-ui F-UI-13 通讯录、"新的朋友"、搜索、资料卡、黑名单、隐私设置（M3） | FE-101 ～ FE-103 | 页面调用本任务单的接口；AC-SOC-04-5、06-14、08-8、10-7、11-3、11-4、13-7 的界面断言在 PRD-ui 任务里 |
| PRD-ui 群设置、群成员、邀请、发起群聊、提示消息样式（M4） | FE-110、FE-111 | AC-SOC-27-4、28-5、29-5 的界面断言在 PRD-ui 任务里 |
| PRD-core 12.3 节 D-1（`Error.userId` 还是 `details.userId`） | FE-110 | 未决定前两种都解析 |

---

## T1 sdk-ts 社交（M3）

### FE-100 社交协议类型、错误码与"命令 + 错误码"文案映射
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：BE-105
- AC：AC-SOC-08-8
- 内容：
  1. `src/protocol.ts`：M3 新命令请求 / ack、`Profile / FriendItem / RequestItem / BlockItem`、事件 `socialChanged`、`socialSettingsChanged`、`Conversation.memberCount`、`convRemoved.reason`（`removed` / `dissolved`；不认识的值按 `removed` 处理）；**不认识的事件类型忽略**。
  2. `src/errors.gen.ts` 由 `errors.json` 重新生成，含 9.4 节 12 个新码；4292 `retryable=false`（不走 429x 兜底）；`send` 收到 4035 / 4036 / 4137 / 4292 标记失败、错误码透传、**不自动重发**（与 4042 相同）。
  3. `src/errors.ts` 新增 `messageFor(cmd, code, ctx)`：按 9.4 节"命令 + 错误码"表，未列出的命令用默认文案；4137 的 N 取本地 `strangerMsgLimit`，4035 / 4135 的昵称取 `error.userId` 的资料。
  4. 测试：`vectors/social.json` 的文案用例、错误码一致性测试自动覆盖新码。

### FE-101 社交状态与同步引擎
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-100、FE-004（同步引擎）
- AC：AC-SOC-01-6、AC-SOC-04-2、AC-SOC-05-4、AC-SOC-06-2、AC-SOC-06-14、AC-SOC-07-5、AC-SOC-09-7
- 内容：
  1. `src/social.ts`：内存状态（D4）`socialVer`、好友 / 申请 / 黑名单（按键 Map）、`requestSeenVer`、`settings`、`official`；被踢、退出登录清空。
  2. 同步轮：连接成功（含重连）先 `convs`，再从 `socialVer` 开始一轮；翻页直到 `hasMore=false`；增量逐页应用（同键保留 ver 较大，`deleted=true` 移除）；全量收齐后整体替换；轮结束 `socialVer = untilVer`；最后一页 `latestVer > untilVer` 或期间收到更大的 `socialChanged` 时立即下一轮；`restart=true` 用同一个 `sinceVer` 重来。
  3. 事件：`socialChanged {ver}` 且 `ver > socialVer` 时去抖 200ms 开始一轮，轮进行中收到的事件轮结束后处理；`socialSettingsChanged` 和每页应答头的 `mode`、`strangerMsgLimit` 更新本地并触发 `socialSettings` 回调。
  4. 自己发起的修改：`changed=true` 且 `ver = socialVer + 1` 应用 `items`；`ver > socialVer + 1` 先应用 `items` 再从 `socialVer` 同步一轮；`changed=false` 不处理。
  5. 30 天与过期判断用服务端时间偏移（auth ack 的 `serverTime`），超过 30 天的申请本地丢弃。
  6. 回调：`friends`、`friendRequests {items, unseen}`、`blocks`、`socialSettings`（快照 + 事件）。
  7. 性能：2000 好友全量轮应用耗时基准（为 AC-SOC-06-14 提供 SDK 部分，单批替换、一次回调）。
  8. 测试：`vectors/social.json` 全部用例（同 ver 跨页、全量替换、轮内事件、`restart`、`purgedVer` 全量、ack 不连续、30 天丢弃）；假传输层单测；对真实 im-server 的多端集成测试（两个 client 实例）。

### FE-102 好友、拉黑、搜索、隐私接口与红点
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-101
- AC：AC-SOC-02-1、AC-SOC-03-1、AC-SOC-04-1、AC-SOC-04-5、AC-SOC-05-1、AC-SOC-07-1、AC-SOC-08-8、AC-SOC-09-1、AC-SOC-13-7
- 内容：
  1. `applyFriend`、`respondFriend`、`deleteFriend`、`setFriendRemark`、`markFriendRequestsSeen`（本地立即清零、上报 `ver = socialVer`）、`block` / `unblock`、`searchUser`、`setSearchable`；失败抛出带错误码的 `ImError`。
  2. `relationOf(userId)`：只看本地，枚举与 `userSearch.relation` 一致（`self > blocked > friend > pendingIn > pendingOut > none`）。
  3. 红点 `unseen` = 收到的、`pending`、未过期（服务端时间）、`ver > requestSeenVer` 的条数；`friendRequests` 按 `updatedAt` 倒序。
  4. 发出的申请在对方同意前 `profile=null`：SDK 保留发起申请时手里已有的资料（搜索结果 / 资料缓存）供界面显示，没有时为 userId。
  5. 资料中的 `official` 透传给界面（"官方"标识的数据来源）。
  6. 测试：单测（红点规则、过期不计数、relationOf 优先级）；集成测试（申请 → 对方红点 → 同意 → 双方好友列表；拉黑后对方 `send` 收到 4036 并标记失败）。

### FE-103 资料缓存、批量查资料、我的群聊、会话成员数
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-101
- AC：AC-SOC-10-7、AC-SOC-10-8、AC-SOC-10-9、AC-SOC-11-3、AC-SOC-11-4、AC-SOC-26-1
- 内容：
  1. 资料缓存：每个 userId 一条，有效期 10 分钟（假时钟可测）；任何带资料的服务端应答（同步轮、搜索、`users`）直接覆盖；`convs` 刷新不清空。
  2. `getUsers(userIds)`：同一事件循环内的请求合并、每批 ≤ 100、命中缓存不请求；不返回的 userId 视为不可见（不报错）。
  3. `loadMyGroups()`：自动翻完 `myGroups` 全部页；会话列表群项透传 `memberCount`。
  4. 通讯录数据：好友列表（只来自 `socialSync`，开放模式下陌生人单聊不进入）+ 我的群聊 + 红点，供 PRD-ui 通讯录页使用。
  5. `convRemoved.reason`：`removed` / `dissolved` 区分并保留会话展示信息（DECISIONS 第 3 节），供界面显示"你已不在该群" / "群聊已解散"。
  6. 测试：缓存过期 / 覆盖单测；分批合并单测；集成测试（600 个群翻页、管理接口解散后收到 `dissolved`）。

---

## T2 sdk-ts 群（M4）

### FE-110 群接口
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-sdk-ts
- 依赖：FE-103、BE-132
- AC：AC-SOC-21-1、AC-SOC-22-1、AC-SOC-23-1、AC-SOC-24-1、AC-SOC-25-1、AC-SOC-26-1、AC-SOC-27-1、AC-SOC-28-5
- 内容：
  1. `createGroup({name, members})`：自动生成 `clientReqId`，一次调用的所有超时重试使用同一个；`inviteToGroup` → `{added, failed}`；`leaveGroup`、`kickFromGroup`、`transferGroupOwner`、`setGroupAdmin`、`dissolveGroup`、`updateGroup`、`getGroupInfo`、`loadGroupMembers(groupId, cursor?)`（成员资料覆盖缓存）。
  2. 解析 `Error.userId`（同时兼容 `details.userId`，见 PRD-core D-1），`messageFor` 用该用户昵称生成 4035 / 4135 文案；`failed[].code` 逐项给出文案。
  3. `convs` / 群会话透传 `myRole`；`convRemoved.reason` 增加 `left`。
  4. 测试：单测（clientReqId 重试不变、错误 userId 解析）；集成测试（建群、邀请、退群多端 `convRemoved {left}`、移出、转让、解散）。

### FE-111 提示消息与群事件
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-sdk-ts
- 依赖：FE-110
- AC：AC-SOC-20-10、AC-SOC-27-4、AC-SOC-29-5
- 内容：
  1. `type=tip` 的消息正常入库、去重、计入未读；`body.tip {kind, actor, targets, count?, text}` 透传；不认识的 `kind` 标记为"[群通知]"；`from=""` 合法。
  2. SDK 生成的 `clientMsgId` 永远不以 `tip-` 开头（UUID 生成器加断言）。
  3. `tipText(msg, me, profiles)`：用 `actor`、`targets` 和本地资料（可用本机备注）生成个性化文案（"你邀请了 dave"、"alice 邀请 dave 加入了群聊"）；`noticeUpdated` 提供摘要，全文经 `getGroupInfo`。
  4. `groupChanged {groupId}`：调用 `groupInfo` 更新本地会话 `myRole`、`memberCount`、`name`，不重拉 `convs`；同一群在途请求合并。
  5. 测试：单测（未知 kind、tip 计入未读、groupChanged 合并请求）；`vectors/social.json` 的 tip 文案用例。

---

## K sdk-kotlin（M5，首版包含社交）

### FE-130 Kotlin：社交协议、错误码与文案映射
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin
- 依赖：FE-100（行为基准）、tianting-sdk-kotlin 首版的协议与连接模块
- AC：AC-SOC-08-8
- 内容：kotlinx.serialization 模型覆盖 M3 + M4 全部社交命令、事件、`tip`、`Error.userId`；错误枚举由 `errors.json` 生成，与 sdk-ts 一致性测试（PRD-core C-10 v1.3：Kotlin 自己一致）；`messageFor(cmd, code)` 映射表；4292 不重试；不认识的事件忽略；跑 `vectors/social.json` 文案用例。

### FE-131 Kotlin：社交状态持久化与同步引擎
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin
- 依赖：FE-130
- AC：AC-SOC-01-6、AC-SOC-04-2、AC-SOC-05-4、AC-SOC-06-2、AC-SOC-07-5、AC-SOC-09-7
- 内容：Room 表 `social_meta(socialVer, requestSeenVer, settings, official)`、`friends`、`friend_requests`、`blocks`；**`socialVer` 与数据在同一个 Room 事务里写入**（10.2 节）；同步轮规则同 FE-101（增量逐页应用、全量收齐后在一个事务里整体替换、`restart`、去抖 200ms、ack 规则、服务端时间偏移、30 天丢弃）；Flow 暴露 `friends`、`friendRequests`、`blocks`、`socialSettings`；被踢 / 退出清库。测试：`vectors/social.json` 全部用例；进程被杀后重启从持久化 ver 增量同步的仪器测试。

### FE-132 Kotlin：好友 / 拉黑 / 搜索 / 资料 / 通讯录接口
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin
- 依赖：FE-131
- AC：AC-SOC-02-1、AC-SOC-03-1、AC-SOC-04-1、AC-SOC-04-5、AC-SOC-05-1、AC-SOC-07-1、AC-SOC-09-1、AC-SOC-10-7、AC-SOC-10-8、AC-SOC-10-9、AC-SOC-11-3、AC-SOC-11-4、AC-SOC-13-7
- 内容：suspend 接口 `applyFriend`、`respondFriend`、`deleteFriend`、`setFriendRemark`、`markFriendRequestsSeen`、`relationOf`、`block` / `unblock`、`searchUser`、`setSearchable`、`getUsers`（合并、每批 100、10 分钟缓存、应答覆盖）、`loadMyGroups`；红点规则同 FE-102；`convRemoved.reason`；对真实 im-server 的集成测试。

### FE-133 Kotlin：群接口与提示消息
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin
- 依赖：FE-132
- AC：AC-SOC-21-1、AC-SOC-22-1、AC-SOC-23-1、AC-SOC-24-1、AC-SOC-25-1、AC-SOC-26-1、AC-SOC-27-1、AC-SOC-27-4、AC-SOC-28-5、AC-SOC-29-5、AC-SOC-20-10
- 内容：M4 群接口全部（`createGroup` 的 `clientReqId` 重试不变）；`tip` 入库、计入未读、未知 kind、`tipText`；`groupChanged` → `groupInfo`；`clientMsgId` 不以 `tip-` 开头；集成测试。

---

## W sdk-swift（M6，首版包含社交）

### FE-140 Swift：社交协议、错误码与文案映射
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift
- 依赖：FE-100（行为基准）、tianting-sdk-swift 首版的协议与连接模块
- AC：AC-SOC-08-8
- 内容：Codable 模型覆盖 M3 + M4 全部社交命令、事件、`tip`、`Error.userId`；错误枚举由 `errors.json` 生成（Swift 自己一致，C-10 v1.3）；`messageFor(cmd:code:)`；4292 不重试；不认识的事件忽略；跑 `vectors/social.json` 文案用例。

### FE-141 Swift：社交状态持久化与同步引擎
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift
- 依赖：FE-140
- AC：AC-SOC-01-6、AC-SOC-04-2、AC-SOC-05-4、AC-SOC-06-2、AC-SOC-07-5、AC-SOC-09-7
- 内容：GRDB 表同 FE-131，`socialVer` 与数据同一事务写入；同步轮规则同 FE-101；`AsyncStream` 暴露状态；被踢 / 退出清库。测试：`vectors/social.json` 全部用例；App 被杀后从持久化 ver 增量同步。

### FE-142 Swift：好友 / 拉黑 / 搜索 / 资料 / 通讯录接口
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift
- 依赖：FE-141
- AC：AC-SOC-02-1、AC-SOC-03-1、AC-SOC-04-1、AC-SOC-04-5、AC-SOC-05-1、AC-SOC-07-1、AC-SOC-09-1、AC-SOC-10-7、AC-SOC-10-8、AC-SOC-10-9、AC-SOC-11-3、AC-SOC-11-4、AC-SOC-13-7
- 内容：async 接口同 FE-132；资料缓存、分批、红点、`relationOf`、`convRemoved.reason`；集成测试。

### FE-143 Swift：群接口与提示消息
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift
- 依赖：FE-142
- AC：AC-SOC-21-1、AC-SOC-22-1、AC-SOC-23-1、AC-SOC-24-1、AC-SOC-25-1、AC-SOC-26-1、AC-SOC-27-1、AC-SOC-27-4、AC-SOC-28-5、AC-SOC-29-5、AC-SOC-20-10
- 内容：同 FE-133。
