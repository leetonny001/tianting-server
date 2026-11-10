# 测试用例：@ 提及（MEN，C4；TC-MEN-03-02、03-03 是 C5）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4.1、4.3 节第 8、10 步、第 8 节 F-MEN-01～03、第 11～13 节的 C4 部分。任务单：[backend-chat](../tasks/backend-chat.md) BE-470、BE-471、BE-476、BE-477、BE-478、BE-480，[frontend-chat](../tasks/frontend-chat.md) FE-660～FE-672。
> 范围：标了【C4】的 AC-MEN。**AC-MEN-03-2、03-3 标的是【C5】（免打扰推送），用例 TC-MEN-03-02、03-03 是 C5 分发时写的，依赖 C3 的推送 worker 和假通道（backend-chat BE-490），C3 恢复后执行**；环境 **E1 + 假通道（等 C3）** 见 [TC-CVS](TC-CVS.md)。
>
> 通用约定：环境、层级、时钟、哨兵法同 [TC-RCL](TC-RCL.md)（**E1** 是 `IM_TEST_MODE=1`、SQLite、`IM_MSG_ACTIONS_ENABLED=1`）；**EP 性能**是目标机（P-12），开发机只记录，不算通过。故障注入 `mentionWrite`、`mentionAllCounter`（BE-471）。**CI 现在不运行（PENDING P-21）**，结果都是本地结果。
> - 记号：群 `g1`：群主 `o`、管理员 `a`、成员 `m`、`u1`、`u2`、`u3`；u9 不是成员。


| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-MEN-01-01 | u1 @ u2：u2 收到 mentions，列表出现"[有人@我]"，u3 没有 | AC-MEN-01-1 | 端到端 + 后端 e2e | P0 |
| TC-MEN-01-02 | mentions 含非成员：成功，只保留成员 | AC-MEN-01-2 | 后端 e2e | P0 |
| TC-MEN-01-03 | 21 个 userId 4131；单聊带 mentions 4000；image 带 mentions 4000 | AC-MEN-01-3 | 后端 e2e | P0 |
| TC-MEN-01-04 | 正文写 @u2、没有 mentions：u2 不被标记 | AC-MEN-01-4 | 后端 e2e | P0 |
| TC-MEN-01-05 | 选了 u2 又删掉这段文字：mentions 不含 u2 | AC-MEN-01-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-01-06 | 单聊输入 @：不打开成员选择 | AC-MEN-01-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-01-07 | 插入用昵称不用备注；选择列表显示备注 | AC-MEN-01-7 | Playwright（H5）+ 原生 UI 测试 + 后端 e2e | P0 |
| TC-MEN-01-08 | a@b.com 不触发；行首＠触发；粘贴的 @u2 不产生 mentions | AC-MEN-01-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-01-09 | 系统中文输入法组字中输入 @：不打开选择，不错位（手工） | AC-MEN-01-9 | 手工 | P0 |
| TC-MEN-01-10 | 草稿带 @ 和引用：离开再回来都恢复，发送后 u2 被提及 | AC-MEN-01-10 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-01-11 | 2000 人群 keyword 前缀搜索；重复 userId 去重 | AC-MEN-01-11 | 后端 e2e | P0 |
| TC-MEN-01-12 | 光标在 mention 后按一次退格：整段删除 | AC-MEN-01-12 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-02-01 | @ 所有人：成员 4030 mentionAllForbidden，管理员、群主成功 | AC-MEN-02-1 | 后端 e2e | P0 |
| TC-MEN-02-02 | 次数：1 小时内第 4 次 4293；1 小时后成功；当天第 11 次 4293 | AC-MEN-02-2 | 后端 e2e | P0 |
| TC-MEN-02-03 | 2000 人、在线 200，1000 次 @ 所有人：ack P99 ≤ 200 ms | AC-MEN-02-3 | 性能 | P0 |
| TC-MEN-02-04 | 只有管理员的选择列表里有"所有人" | AC-MEN-02-4 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-02-05 | 服务端 4293：消息失败，提示次数已用完，不自动重发 | AC-MEN-02-5 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-MEN-02-06 | @ 所有人只写一行 group_mention_all，不按成员展开 | AC-MEN-02-6 | 后端 e2e | P0 |
| TC-MEN-02-07 | 托管群没有群主：成员 @ 所有人 403；全群禁言时群主可以 | AC-MEN-02-7 | 后端 e2e | P0 |
| TC-MEN-02-08 | 撤回不退次数；计数器故障 503 | AC-MEN-02-8 | 后端 e2e | P0 |
| TC-MEN-02-09 | 并发 10 条 @ 所有人：恰好 3 条成功 | AC-MEN-02-9 | 后端 e2e | P0 |
| TC-MEN-03-01 | mentionUnread 和 firstMentionSeq：含 @ 所有人，已读后前移 | AC-MEN-03-1 | 后端 e2e | P0 |
| TC-MEN-03-02 | 免打扰群：只有 @ 我推送，@ 所有人和普通消息不推（等 C3） | AC-MEN-03-2 | 后端 e2e | P0 |
| TC-MEN-03-03 | 被我拉黑的人在免打扰群 @ 我：mentionUnread 增加，令牌无调用（推送部分等 C3） | AC-MEN-03-3 | 后端 e2e | P0 |
| TC-MEN-03-04 | @ u2 的消息被撤回：mentionUnread 减 1 | AC-MEN-03-4 | 后端 e2e | P0 |
| TC-MEN-03-05 | 有提及又有草稿：列表显示"[有人@我]" | AC-MEN-03-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-03-06 | 未读 100 条，提及在第 10 条：跳转按钮 | AC-MEN-03-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MEN-03-07 | 另一端已读、对方撤回提及：5 秒内重新拉取后"[有人@我]"消失（三套 SDK） | AC-MEN-03-7 | SDK 单测（三套）+ 共享向量 | P0 |
| TC-MEN-03-08 | 自己 @ 自己不计；已退群的被过滤；提及表写失败 500 | AC-MEN-03-8 | 后端 e2e | P0 |

---

## F-MEN-01 @ 成员

### TC-MEN-01-01 u1 @ u2：u2 收到 mentions，列表出现"[有人@我]"，u3 没有
- 关联 AC：AC-MEN-01-1
- 优先级：P0
- 批次：C4
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端
- 前置条件：g1 里 u1、u2、u3，三端轮换 u1、u2 的端
- 步骤：
  1. u1 在输入框 @ u2 发送
  2. u2、u3 看会话列表；查 `convs`
- 预期结果：
  1. u2 收到的 `msg.body.mentions.userIds=[u2]`
  2. u2 的列表摘要前缀"[有人@我]"、`mentionUnread=1`；u3 没有前缀、`mentionUnread=0`

### TC-MEN-01-02 mentions 含非成员：成功，只保留成员
- 关联 AC：AC-MEN-01-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u9 不在 g1
- 步骤：
  1. u1 发 `mentions {userIds:[u9, u2]}`
  2. u2 `sync`
- 预期结果：
  1. ack 成功
  2. 保存和下发的 `userIds=[u2]`

### TC-MEN-01-03 21 个 userId 4131；单聊带 mentions 4000；image 带 mentions 4000
- 关联 AC：AC-MEN-01-3
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：g1 有 25 个成员；单聊 u1、u2；一个 `ready` 的图片
- 步骤：
  1. 21 个 userId 发送
  2. 单聊带 `mentions`
  3. `type=image` 带 `mentions`
- 预期结果：
  1. 4131
  2. 4000
  3. 4000；三次都没有保存消息

### TC-MEN-01-04 正文写 @u2、没有 mentions：u2 不被标记
- 关联 AC：AC-MEN-01-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1
- 步骤：
  1. u1 发 `text:"@u2 看一下"`，不带 `mentions`
  2. u2 查 `convs`
- 预期结果：
  1. u2 的 `mentionUnread=0`，`conv_mentions` 没有行

### TC-MEN-01-05 选了 u2 又删掉这段文字：mentions 不含 u2
- 关联 AC：AC-MEN-01-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：g1 群聊输入框
- 步骤：
  1. 输入 `@` 选 u2
  2. 删除插入的"@小王 "
  3. 发送，抓上行帧
- 预期结果：
  1. 上行没有 `mentions`（或 `userIds` 不含 u2）

### TC-MEN-01-06 单聊输入 @：不打开成员选择
- 关联 AC：AC-MEN-01-6
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：单聊输入框
- 步骤：
  1. 输入 `@`
- 预期结果：
  1. 没有成员选择，`@` 作为普通字符留在输入框

### TC-MEN-01-07 插入用昵称不用备注；选择列表显示备注
- 关联 AC：AC-MEN-01-7
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试 + 后端 e2e
- 环境：Playwright（H5）+ 模拟器（Android、iOS）；E1
- 前置条件：u1 给 u2 设备注"老板"，u2 昵称"小王"
- 步骤：
  1. u1 输入 `@`，看选择列表
  2. 选 u2 发送；抓上行帧，查服务端 `messages.body`
- 预期结果：
  1. 选择列表里 u2 显示"老板"（备注）
  2. 正文含"@小王"，上行帧和服务端存储都搜不到"老板"

### TC-MEN-01-08 a@b.com 不触发；行首＠触发；粘贴的 @u2 不产生 mentions
- 关联 AC：AC-MEN-01-8
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：g1 群聊输入框
- 步骤：
  1. 输入 `a@b.com`
  2. 清空后行首输入全角 `＠`
  3. 粘贴"@u2"后发送
- 预期结果：
  1. 不打开选择
  2. 打开选择
  3. 上行没有 `mentions`

### TC-MEN-01-09 系统中文输入法组字中输入 @：不打开选择，不错位（手工）
- 关联 AC：AC-MEN-01-9
- 优先级：P0
- 批次：C4
- 层级：手工
- 环境：H5：macOS Chrome + 系统拼音；Android、iOS：模拟器 + 系统中文输入法，真机在 P-13
- 前置条件：g1 群聊输入框，写法同 AC-UI-10-2
- 步骤：
  1. 组字过程中输入 `@`
  2. 回车上屏，再在空白后输入 `@` 选人
- 预期结果：
  1. 组字时不打开选择，上屏的文字和光标位置正确
  2. 之后选人正常；录屏留档，真机未到位记未执行

### TC-MEN-01-10 草稿带 @ 和引用：离开再回来都恢复，发送后 u2 被提及
- 关联 AC：AC-MEN-01-10
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：g1
- 步骤：
  1. 输入文字、@ u2、引用一条消息，不发送
  2. 离开会话再回来
  3. 发送
- 预期结果：
  1. 文字、mention 区间、引用条都恢复（列表里显示"[草稿]"）
  2. 上行带 `mentions:[u2]` 和 `quote`；u2 被提及

### TC-MEN-01-11 2000 人群 keyword 前缀搜索；重复 userId 去重
- 关联 AC：AC-MEN-01-11
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：2000 人群；成员里 `zhang1`、`zhao2` 和昵称以"zh"开头的若干，其余不匹配
- 步骤：
  1. `groupMembers {groupId, keyword:"zh"}`
  2. 发 `mentions {userIds:[u2,u2,u2]}`
  3. 1 分钟内第 31 次搜索
- 预期结果：
  1. 只返回 `userId` 或昵称以"zh"开头的成员
  2. 保存的 `userIds=[u2]`
  3. 4290

### TC-MEN-01-12 光标在 mention 后按一次退格：整段删除
- 关联 AC：AC-MEN-01-12
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：输入框里有"你好 @小王 "（mention 区间），光标在其后
- 步骤：
  1. 按一次退格
  2. 发送
- 预期结果：
  1. "@小王 "整段消失
  2. 上行没有 `mentions`

## F-MEN-02 @ 所有人

### TC-MEN-02-01 @ 所有人：成员 4030 mentionAllForbidden，管理员、群主成功
- 关联 AC：AC-MEN-02-1
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1
- 步骤：
  1. m、a、o 分别发 `mentions {all:true}`
- 预期结果：
  1. m：4030，`reason=mentionAllForbidden`，消息未保存
  2. a、o 成功

### TC-MEN-02-02 次数：1 小时内第 4 次 4293；1 小时后成功；当天第 11 次 4293
- 关联 AC：AC-MEN-02-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：[IM 钟] 从 Asia/Shanghai 当天 00:00 起
- 步骤：
  1. o 在 10 分钟内发 4 次 @ 所有人
  2. 推进 1 小时后再发
  3. 继续，每次间隔 61 分钟，直到当天第 11 次
- 预期结果：
  1. 第 4 次 4293，`details.retryAfterSeconds` > 0
  2. 成功
  3. 第 11 次 4293；推进到次日 00:00 后成功

### TC-MEN-02-03 2000 人、在线 200，1000 次 @ 所有人：ack P99 ≤ 200 ms
- 关联 AC：AC-MEN-02-3
- 优先级：P0
- 批次：C4
- 层级：性能
- 环境：EP 性能（P-12）；开发机只记录
- 前置条件：`IM_MENTION_ALL_HOURLY`、`IM_MENTION_ALL_DAILY` 调高；生产配置
- 步骤：
  1. o 发 1000 次 @ 所有人，跑 3 次
- 预期结果：
  1. 3 次取最差，ack P99 ≤ 200 ms（未校准），错误率 0；报告写基线提交号
  2. 开发机结果不算通过

### TC-MEN-02-04 只有管理员的选择列表里有"所有人"
- 关联 AC：AC-MEN-02-4
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：同一个群，m 和 a 分别登录
- 步骤：
  1. 各自输入 `@` 打开选择
- 预期结果：
  1. a 看到"所有人"，m 看不到

### TC-MEN-02-05 服务端 4293：消息失败，提示次数已用完，不自动重发
- 关联 AC：AC-MEN-02-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：Playwright（H5）+ 模拟器（Android、iOS）；SDK 单测
- 前置条件：o 已用完本小时的次数
- 步骤：
  1. o @ 所有人发送
  2. 断线重连一次
- 预期结果：
  1. 消息标记失败，提示"@所有人次数已用完"
  2. 重连后没有自动重发（抓帧）

### TC-MEN-02-06 @ 所有人只写一行 group_mention_all，不按成员展开
- 关联 AC：AC-MEN-02-6
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：500 人群
- 步骤：
  1. o @ 所有人一次
  2. 查 `conv_mentions`、`group_mention_all`
- 预期结果：
  1. `conv_mentions` 没有这条的行
  2. `group_mention_all` 一行

### TC-MEN-02-07 托管群没有群主：成员 @ 所有人 403；全群禁言时群主可以
- 关联 AC：AC-MEN-02-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：托管群 h1 没有群主；g1 开了全群禁言
- 步骤：
  1. h1 的成员 @ 所有人
  2. g1 的群主 @ 所有人
- 预期结果：
  1. 403 / 4030
  2. 成功

### TC-MEN-02-08 撤回不退次数；计数器故障 503
- 关联 AC：AC-MEN-02-8
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1
- 步骤：
  1. o 发 3 次 @ 所有人，再撤回这 3 条，发第 4 次
  2. `mentionAllCounter` 故障注入后再发
- 预期结果：
  1. 第 4 次仍 4293
  2. 503 / 5030，消息未保存（不放行）

### TC-MEN-02-09 并发 10 条 @ 所有人：恰好 3 条成功
- 关联 AC：AC-MEN-02-9
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1，o 的 10 个连接
- 步骤：
  1. 同时发 10 条 @ 所有人
- 预期结果：
  1. 恰好 3 条成功，7 条 4293；`group_mention_all` 3 行

## F-MEN-03 提醒与免打扰

### TC-MEN-03-01 mentionUnread 和 firstMentionSeq：含 @ 所有人，已读后前移
- 关联 AC：AC-MEN-03-1
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u2 未读里 seq 5、7 @ 他，seq 9 是 @ 所有人
- 步骤：
  1. u2 查 `convs`
  2. u2 已读到 5 后再查
- 预期结果：
  1. `mentionUnread=3`、`firstMentionSeq=5`
  2. `mentionUnread=2`、`firstMentionSeq=7`

### TC-MEN-03-02 免打扰群：只有 @ 我推送，@ 所有人和普通消息不推（等 C3）
- 关联 AC：AC-MEN-03-2
- 优先级：P0
- 批次：C5（随 backend-chat BE-490，等 C3）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3），`IM_CONV_SETTINGS_ENABLED=1`
- 前置条件：u2 用账号登录、有令牌，把 g1 设为免打扰后离线；哨兵 `s` 离线、有令牌
- 步骤：
  1. o 在 g1 依次发普通消息、@ 所有人、@ u2，每条之间用 [IM 钟] 隔开超过 `IM_PUSH_MERGE_SECONDS`
  2. 给 `s` 发一条，等假通道记录到 `s` 的调用
  3. u2 上线 `convs`
- 预期结果：
  1. u2 的令牌只有 1 次调用，对应 @ u2 那条
  2. `unread=3`、`mentionUnread=2`（@ 所有人也计入，F-MEN-03 规则 1）
  3. C3 恢复前记"未执行（等 C3）"；`mentionUnread` 的计算 C4 已验（TC-MEN-03-01）

### TC-MEN-03-03 被我拉黑的人在免打扰群 @ 我：mentionUnread 增加，令牌无调用（推送部分等 C3）
- 关联 AC：AC-MEN-03-3
- 优先级：P0
- 批次：C5（推送部分随 backend-chat BE-490，等 C3）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3）；`mentionUnread` 部分 E1
- 前置条件：u2 拉黑了 u1，两人都在 g1；u2 用账号登录、有令牌，把 g1 设为免打扰后离线；哨兵 `s`
- 步骤：
  1. u1 在 g1 @ u2
  2. 给 `s` 发一条，等假通道记录到
  3. u2 上线 `convs`
- 预期结果：
  1. 消息正常保存（群里拉黑不生效，F-SOC-07）；u2 的 `mentionUnread` 增加 1，列表显示"[有人@我]"
  2. u2 的令牌没有调用（不打断免打扰，F-MEN-03 规则 5）
  3. C3 恢复前：第 3 步在 E1 先验（记"部分通过"），推送部分记"未执行（等 C3）"

### TC-MEN-03-04 @ u2 的消息被撤回：mentionUnread 减 1
- 关联 AC：AC-MEN-03-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u2 有 2 条未读提及
- 步骤：
  1. u1 撤回其中一条
  2. u2 查 `convs`
- 预期结果：
  1. `mentionUnread=1`，`firstMentionSeq` 是剩下那条的 seq

### TC-MEN-03-05 有提及又有草稿：列表显示"[有人@我]"
- 关联 AC：AC-MEN-03-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：u2 在 g1 有草稿，又被 @
- 步骤：
  1. 看会话列表
- 预期结果：
  1. 前缀是"[有人@我]"，不是"[草稿]"

### TC-MEN-03-06 未读 100 条，提及在第 10 条：跳转按钮
- 关联 AC：AC-MEN-03-6
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：u2 在 g1 有 100 条未读，第 10 条 @ u2
- 步骤：
  1. 打开会话
  2. 点"有人@我"
- 预期结果：
  1. 出现按钮
  2. 滚动到那条并高亮，按钮消失

### TC-MEN-03-07 另一端已读、对方撤回提及：5 秒内重新拉取后"[有人@我]"消失（三套 SDK）
- 关联 AC：AC-MEN-03-7
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）+ 共享向量
- 环境：SDK 单测（[SDK 钟]）
- 前置条件：本地 `mentionUnread=1`
- 步骤：
  1. ① 收到 `readUpdated`（另一端已读过提及位置）
  2. ② 另起一例：收到提及消息的 `msgDeleted`
  3. 推进 [SDK 钟] 5 秒
- 预期结果：
  1. 两种情况都在 5 秒内重新拉 `convs`（节流：5 秒内多个事件只拉 1 次）
  2. 拉取后 `mentionUnread=0`，前缀消失；`chat-sync.json` 的同名用例通过

### TC-MEN-03-08 自己 @ 自己不计；已退群的被过滤；提及表写失败 500
- 关联 AC：AC-MEN-03-8
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1；u3 已退群
- 步骤：
  1. u1 @ u1
  2. u1 @ u3
  3. `mentionWrite` 故障注入后 u1 @ u2
- 预期结果：
  1. u1 的 `mentionUnread` 不变
  2. `userIds` 里没有 u3
  3. 500 / 5000，消息未保存

---

## 说明

- 用例编号按 AC 编号对齐（TC-MEN-0X-YY 对应 AC-MEN-0X-Y）。TC-MEN-03-02、03-03（【C5】）是 C5 分发时补的，等 C3 恢复后执行。
- TC-MEN-02-02 的小时窗口按滚动 60 分钟写（backend-chat 第 58 条），Frank 改口径时同步改用例。

