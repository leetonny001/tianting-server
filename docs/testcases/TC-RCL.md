# 测试用例：撤回（RCL，C4）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4.1、4.3、4.5、8 节 F-RCL-01～04，第 11～15 节的 C4 部分。任务单：[backend-chat](../tasks/backend-chat.md) BE-470～BE-481，[console-chat](../tasks/console-chat.md) CS-202、CS-204，[frontend-chat](../tasks/frontend-chat.md) FE-660～FE-672。
> 范围：标了【C4】的 AC-RCL（CH-27，Frank 2026-11-10"先做功能"）。**TC-RCL-03-09 依赖 C3 的推送 worker 和假通道（BE-481），C3 恢复前记"未执行（等 C3）"**。引用和提及见 [TC-QUO](TC-QUO.md)、[TC-MEN](TC-MEN.md)；AC-FILE-06-3 在 [TC-FILE](TC-FILE.md)（TC-FILE-06-13），AC-APN-03-3 在 [TC-APN](TC-APN.md)（TC-APN-03-03），【全部】的 C4 部分在 [TC-CHAT](TC-CHAT.md)（TC-CHAT-90-26～34）。
>
> 通用约定：
> - 环境：**E1**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库，`IM_MSG_ACTIONS_ENABLED=1`，`IM_QUARANTINE_KEY` 用测试里现场生成的随机值）；**E1 + 假 S3**（再加 `objstoretest`，`IM_FILES_ENABLED=1`）；**E1 + 假 S3 + console-server**（`CON_TEST_MODE=1`，Postgres，`IM_V13_ADMIN_FEATURES=1`）；**SDK 单测**（三套 SDK：`FakeTransport` + 假时钟 + 本地库，向量按 `protocol/vectors`）；**Playwright（H5）**、**模拟器**（Android 模拟器、iOS Simulator，连 E1）；**E1 + 三端客户端**（三端先后连同一个 E1）；**E1 + 假通道（等 C3）**。开发机不安装软件。
> - **CI 现在不运行（PENDING P-21）**，结果都是本地结果，记录写明环境和提交号。真机在 P-13。
> - 层级：**后端 e2e**（Go e2e，对 `/ws`、`/admin/*` 断言）、**SDK 单测**、**Playwright**、**原生 UI 测试**（Compose UI Test、XCUITest）、**端到端**（真实 im-server + 客户端）。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算；**[SDK 钟]** 各 SDK 的 `Clock`。故障注入 `POST /admin/test/faults`（`quarantineWrite`，BE-471）。
> - 特征串：`ZXQ-RCL-1`（正文）、`ZXQ-RCL-2`（SDK 内存）、`ZXQ-RCL-3`（文件名）。"搜不到"指对原始字节做子串搜索（SQLite 先 `PRAGMA wal_checkpoint(TRUNCATE)`）。
> - 记号：`u1`、`u2` 单聊双方；群 `g1`：群主 `o`、管理员 `a`、`a2`、成员 `m`、`n`；`u1'` 是 u1 的另一台设备；负向的"没有事件"用哨兵法（之后再发一条普通消息，收到它之后断言前面没有事件）。


| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-RCL-01-01 | 60 秒内撤回：ack 成功，对方和自己另一端 2 秒内收到带 recalled 的 msgDeleted | AC-RCL-01-1 | 端到端 + 后端 e2e | P0 |
| TC-RCL-01-02 | 时限边界：119 秒、120 秒成功，121 秒 4097 | AC-RCL-01-2 | 后端 e2e | P0 |
| TC-RCL-01-03 | 无权限、非参与方、其他会话、tip：依次 4030、4032、4044、4000 | AC-RCL-01-3 | 后端 e2e | P0 |
| TC-RCL-01-04 | 重复撤回：成功，不再发事件 | AC-RCL-01-4 | 后端 e2e | P0 |
| TC-RCL-01-05 | 被禁言、被拉黑：仍能撤回自己的消息 | AC-RCL-01-5 | 后端 e2e | P0 |
| TC-RCL-01-06 | 撤回后 SQLite 原始字节搜不到正文和文件名，隔离表只有密文 | AC-RCL-01-6 | 后端 e2e | P0 |
| TC-RCL-01-07 | 撤回不减未读，updatedAt 不变 | AC-RCL-01-7 | 后端 e2e | P0 |
| TC-RCL-01-08 | 撤回菜单按服务端时间：60 秒内有，121 秒后没有，菜单打开时过期提示 4097 | AC-RCL-01-8 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-RCL-01-09 | 给自己的消息、custom 可以撤回；1 分钟内第 21 次 4290 | AC-RCL-01-9 | 后端 e2e | P0 |
| TC-RCL-01-10 | 开关关 4030；开启缺隔离密钥启动失败；不依赖 V13 | AC-RCL-01-10 | 后端 e2e + 单测 | P0 |
| TC-RCL-01-11 | 隔离写失败：500，消息不变；恢复后重试成功，只发一次事件 | AC-RCL-01-11 | 后端 e2e | P0 |
| TC-RCL-02-01 | 角色矩阵：群主撤管理员、管理员撤成员成功，其余 4030 | AC-RCL-02-1 | 后端 e2e | P0 |
| TC-RCL-02-02 | 群主撤回 6 天前成功，8 天前 4097 | AC-RCL-02-2 | 后端 e2e | P0 |
| TC-RCL-02-03 | 托管群：后台指定群主后按矩阵；没有群主时成员不能撤回他人 | AC-RCL-02-3 | 后端 e2e | P0 |
| TC-RCL-02-04 | 管理员撤回成员消息：三方看到的文案和列表摘要 | AC-RCL-02-4 | 端到端 + 后端 e2e | P0 |
| TC-RCL-02-05 | 管理员撤回其他管理员：提示"你没有权限撤回这条消息" | AC-RCL-02-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-RCL-02-06 | 被撤回者已退群可以撤回；解散的群 4043；角色刚被取消 4030 | AC-RCL-02-6 | 后端 e2e | P0 |
| TC-RCL-02-07 | 群主、管理员撤回自己 1 天前的消息：取更宽时限 | AC-RCL-02-7 | 后端 e2e | P0 |
| TC-RCL-03-01 | sdk-ts：收到撤回后内存搜不到原文 | AC-RCL-03-1 | SDK 单测（sdk-ts） | P0 |
| TC-RCL-03-02 | sdk-kotlin、sdk-swift：本地库和缓存目录搜不到原文和缩略图 | AC-RCL-03-2 | SDK 单测（sdk-kotlin、sdk-swift） | P0 |
| TC-RCL-03-03 | 断网期间被撤回：重连后变撤回文案 | AC-RCL-03-3 | 端到端 + SDK 单测 | P0 |
| TC-RCL-03-04 | App 被杀时被撤回：冷启动后变撤回文案，本地库搜不到原文 | AC-RCL-03-4 | 原生 UI 测试 + SDK 单测 | P0 |
| TC-RCL-03-05 | 最后一条被撤回：lastMsg 带 deleted、recalled，没有 text，modVer 增加 | AC-RCL-03-5 | 后端 e2e | P0 |
| TC-RCL-03-06 | 群里撤回自己的最后一条：列表摘要按规则 1a 第 2 行 | AC-RCL-03-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-RCL-03-07 | 撤回的 msg、sync、事件帧符合 Schema | AC-RCL-03-7 | 单测（服务端）+ 后端 e2e | P0 |
| TC-RCL-03-08 | 全屏看的图片被撤回：退出全屏并提示 | AC-RCL-03-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-RCL-03-09 | 隐藏模式消息被撤回：假通道没有调用（等 C3） | AC-RCL-03-9 | 后端 e2e | P0 |
| TC-RCL-03-10 | 单聊撤回文案；提示行没有长按菜单（三套 SDK） | AC-RCL-03-10 | SDK 单测（三套） | P0 |
| TC-RCL-03-11 | 重连只对 modVer 变了的会话发 sync（三套 SDK） | AC-RCL-03-11 | SDK 单测（三套） | P0 |
| TC-RCL-03-12 | 文件下载中被撤回：中止，提示，不留部分文件 | AC-RCL-03-12 | 原生 UI 测试（Android、iOS） | P0 |
| TC-RCL-03-13 | 迟到的旧 sync 不恢复原文和引用（三套 SDK） | AC-RCL-03-13 | SDK 单测（三套）+ 共享向量 | P0 |
| TC-RCL-04-01 | 后台删除已撤回的消息：200，状态仍是撤回，没有事件，有审计 | AC-RCL-04-1 | 后端 e2e | P0 |
| TC-RCL-04-02 | 撤回已被后台删除的消息：4044 | AC-RCL-04-2 | 后端 e2e | P0 |
| TC-RCL-04-03 | 群撤回消息：普通正文授权没有 body、deleted=false；隔离授权返回原文 | AC-RCL-04-3 | 后端 e2e | P0 |
| TC-RCL-04-04 | 31 天后 quarantine_purge 删除撤回的隔离记录 | AC-RCL-04-4 | 后端 e2e | P0 |
| TC-RCL-04-05 | 单聊撤回、IM_C2C_BODY_ACCESS=off：申请隔离授权 403 | AC-RCL-04-5 | 后端 e2e | P0 |
| TC-RCL-04-06 | 撤回的图片：参与者申请地址 404 | AC-RCL-04-6 | 后端 e2e | P0 |
| TC-RCL-04-07 | 元数据查询：有 recalled*，没有 body 和文件名 | AC-RCL-04-7 | 后端 e2e | P0 |
| TC-RCL-04-08 | 撤回的文件：files.name 为空，文件名在隔离区（密文） | AC-RCL-04-8 | 后端 e2e | P0 |
| TC-RCL-04-09 | fetch：没有授权 4039、隔离授权成功、普通调用方 4030 | AC-RCL-04-9 | 后端 e2e | P0 |

---

## F-RCL-01 发送者撤回

### TC-RCL-01-01 60 秒内撤回：ack 成功，对方和自己另一端 2 秒内收到带 recalled 的 msgDeleted
- 关联 AC：AC-RCL-01-1
- 优先级：P0
- 批次：C4
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端
- 前置条件：u1、u1'、u2 在线；u1 给 u2 发文本 m1；[IM 钟] 推进 60 秒
- 步骤：
  1. u1 发 `recall {convId, msgId:m1}`
  2. 记录 u2、u1' 收到的事件和时间
  3. 三端各做一次 u1 撤回（H5、Android、iOS 互为对方）
- 预期结果：
  1. ack `{}`
  2. u2、u1' 在 2 秒内收到 `msgDeleted {convId, msgIds:[m1], seqs, recalled:{by:u1, at}}`
  3. 三端页面都变成撤回文案（u1 看"你撤回了一条消息"，u2 看"对方撤回了一条消息"）

### TC-RCL-01-02 时限边界：119 秒、120 秒成功，121 秒 4097
- 关联 AC：AC-RCL-01-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 发 3 条消息 m1、m2、m3（同一个 `ts`，[IM 钟] 停住）
- 步骤：
  1. 推进到 `ts`+119 秒撤回 m1
  2. 推进到 `ts`+120 秒撤回 m2
  3. 推进到 `ts`+121 秒撤回 m3
  4. u2 `sync`
- 预期结果：
  1. 前两个 ack 成功
  2. 第三个错误 4097，`details.windowSeconds=120`
  3. m3 原样，正文还在

### TC-RCL-01-03 无权限、非参与方、其他会话、tip：依次 4030、4032、4044、4000
- 关联 AC：AC-RCL-01-3
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1、u2 单聊消息 m1；u3 不是参与方；会话 c2 的消息 x；群里的 `tip` 消息 t
- 步骤：
  1. u2 撤回 m1
  2. u3 撤回 m1（`convId` 写 u1、u2 的单聊）
  3. u1 用 c1 的 `convId` 撤回 x
  4. 群主撤回 t
- 预期结果：
  1. 4030
  2. 4032
  3. 4044（和不存在的 msgId 响应相同）
  4. 4000；每种情况消息都不变，没有事件（哨兵法）

### TC-RCL-01-04 重复撤回：成功，不再发事件
- 关联 AC：AC-RCL-01-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 已撤回 m1，u2 收到过一次 `msgDeleted`
- 步骤：
  1. u1 再发 `recall` m1
  2. u1 给 u2 再发一条哨兵消息
- 预期结果：
  1. ack 成功
  2. u2 收到哨兵消息之前没有第二个 `msgDeleted`

### TC-RCL-01-05 被禁言、被拉黑：仍能撤回自己的消息
- 关联 AC：AC-RCL-01-5
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：① 群 g1 里 m 发了消息后被群主禁言；② 单聊 u1 发了消息后被 u2 拉黑；[IM 钟] 推进 60 秒
- 步骤：
  1. ① m 撤回自己的消息
  2. ② u1 撤回自己的消息
- 预期结果：
  1. 都 ack 成功
  2. 其他参与者收到 `msgDeleted`（②的 u2 照样收到）

### TC-RCL-01-06 撤回后 SQLite 原始字节搜不到正文和文件名，隔离表只有密文
- 关联 AC：AC-RCL-01-6
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 发文本（含 `ZXQ-RCL-1`）和文件消息（文件名含 `ZXQ-RCL-3`）
- 步骤：
  1. u1 撤回两条
  2. 做检查点，读 SQLite 主库和 WAL 文件的原始字节
  3. 解密隔离记录（测试用同一把密钥）
- 预期结果：
  1. `messages`、`conv_seq`、`files` 和整个库文件里都搜不到两个特征串
  2. 隔离表的密文里搜不到明文
  3. 解密后正文和文件名都对得上（证明原文进了隔离区）

### TC-RCL-01-07 撤回不减未读，updatedAt 不变
- 关联 AC：AC-RCL-01-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 给 u2 发 3 条，u2 未读 3；记下 `convs` 的 `updatedAt` 和排序
- 步骤：
  1. u1 撤回第 2 条
  2. u2 查 `convs`
- 预期结果：
  1. `unread=3`
  2. `updatedAt` 和会话顺序不变

### TC-RCL-01-08 撤回菜单按服务端时间：60 秒内有，121 秒后没有，菜单打开时过期提示 4097
- 关联 AC：AC-RCL-01-8
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：Playwright（H5）+ 模拟器（Android、iOS）；SDK 单测
- 前置条件：设备时钟比服务端快 10 分钟（SDK 用 `auth` ack 的 `serverTime` 校正）；u1 发两条，一条 `ts` 是 60 秒前，一条 121 秒前
- 步骤：
  1. 长按两条消息
  2. 对 60 秒前那条打开菜单不点，[IM 钟] 推进 70 秒后点"撤回"
- 预期结果：
  1. 前者有"撤回"，后者没有（按校正后的服务端时间，不按设备时钟）
  2. 提示"发送时间超过 2 分钟，无法撤回"，消息不变
  3. SDK 单测：`canRecall` 用 [SDK 钟] 和服务端偏移，边界 120 秒

### TC-RCL-01-09 给自己的消息、custom 可以撤回；1 分钟内第 21 次 4290
- 关联 AC：AC-RCL-01-9
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1（`IM_CLIENT_CUSTOM_MSG` 测试环境打开）
- 前置条件：u1 给自己发一条文本；u1 发一条 `custom`；u1 准备 21 条消息
- 步骤：
  1. 撤回给自己的消息
  2. 撤回 `custom`
  3. 1 分钟内依次撤回 21 条
- 预期结果：
  1. 前两个成功
  2. 前 20 次成功，第 21 次 429 / 4290
  3. 频率不和发送限流共用：撤回到 4290 之后还能发消息

### TC-RCL-01-10 开关关 4030；开启缺隔离密钥启动失败；不依赖 V13
- 关联 AC：AC-RCL-01-10
- 优先级：P0
- 批次：C4
- 层级：后端 e2e + 单测
- 环境：E1
- 前置条件：三组配置：① `IM_MSG_ACTIONS_ENABLED=0`；② 开启、不设 `IM_QUARANTINE_KEY`（非生产）；③ 开启、`IM_V13_ADMIN_FEATURES=0`
- 步骤：
  1. ① 撤回
  2. ② 启动 im-server
  3. ③ 撤回
- 预期结果：
  1. ① 403 / 4030，`reason=featureDisabled`
  2. ② 非 0 退出，错误信息指出缺 `IM_QUARANTINE_KEY`（不回退到开发默认值，backend-chat 第 54 条）
  3. ③ 成功

### TC-RCL-01-11 隔离写失败：500，消息不变；恢复后重试成功，只发一次事件
- 关联 AC：AC-RCL-01-11
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 的消息 m1；`POST /admin/test/faults {quarantineWrite:true}`
- 步骤：
  1. u1 撤回 m1
  2. u2 `sync`
  3. 关掉故障，u1 重试 `recall` 两次
  4. 哨兵消息
- 预期结果：
  1. 500 / 5000
  2. m1 原样，`recalled_at` 为空，没有事件
  3. 两次重试都成功，u2 只收到一次 `msgDeleted`

## F-RCL-02 群主、管理员撤回

### TC-RCL-02-01 角色矩阵：群主撤管理员、管理员撤成员成功，其余 4030
- 关联 AC：AC-RCL-02-1
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：自建群 g1：o、a、a2、m 各发一条
- 步骤：
  1. o 撤回 a 的
  2. a 撤回 m 的
  3. a 撤回 a2 的
  4. a 撤回 o 的
  5. m 撤回 a 的
- 预期结果：
  1. 前两个成功
  2. 后三个 4030，消息不变

### TC-RCL-02-02 群主撤回 6 天前成功，8 天前 4097
- 关联 AC：AC-RCL-02-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：m 在 [IM 钟] 第 0 天、第 2 天各发一条
- 步骤：
  1. 推进到第 8 天，o 撤回第 2 天那条（6 天前）
  2. o 撤回第 0 天那条（8 天前）
- 预期结果：
  1. 成功
  2. 4097，`details.windowSeconds=604800`

### TC-RCL-02-03 托管群：后台指定群主后按矩阵；没有群主时成员不能撤回他人
- 关联 AC：AC-RCL-02-3
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：托管群 h1（管理接口创建，没有群主），成员 p、q 各发一条
- 步骤：
  1. p 撤回 q 的
  2. 管理接口指定 p 为群主后，p 再撤回 q 的
- 预期结果：
  1. 4030
  2. 成功

### TC-RCL-02-04 管理员撤回成员消息：三方看到的文案和列表摘要
- 关联 AC：AC-RCL-02-4
- 优先级：P0
- 批次：C4
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端
- 前置条件：群 g1，m 发了最后一条消息；a、m、n 在三端（各端轮换一次）
- 步骤：
  1. a 撤回 m 的消息
  2. a、m、n 看聊天页和会话列表
- 预期结果：
  1. a："你撤回了 {m 显示名} 的一条消息"
  2. m："{a 显示名}撤回了你的一条消息"
  3. n："{a 显示名}撤回了一条成员消息"；列表摘要和聊天页一致
  4. 服务端：`msgDeleted.recalled.by=a`，`lastMsg.recalledBy=a`

### TC-RCL-02-05 管理员撤回其他管理员：提示"你没有权限撤回这条消息"
- 关联 AC：AC-RCL-02-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：a 是管理员，长按 a2 的消息
- 步骤：
  1. 点"撤回"
- 预期结果：
  1. 菜单里有"撤回"（客户端不知道对方角色）
  2. 服务端 4030 后提示"你没有权限撤回这条消息"，消息不变

### TC-RCL-02-06 被撤回者已退群可以撤回；解散的群 4043；角色刚被取消 4030
- 关联 AC：AC-RCL-02-6
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：m 发消息后退群；群 g2 已解散；a 发起撤回前被 o 取消管理员
- 步骤：
  1. o 撤回 m 的消息
  2. 对 g2 的消息撤回
  3. a 撤回 n 的消息
- 预期结果：
  1. 成功
  2. 4043
  3. 4030（角色查库，不用缓存）

### TC-RCL-02-07 群主、管理员撤回自己 1 天前的消息：取更宽时限
- 关联 AC：AC-RCL-02-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：o、a 各发一条，[IM 钟] 推进 1 天
- 步骤：
  1. o 撤回自己的
  2. a 撤回自己的
- 预期结果：
  1. 都成功（a 一行按 backend-chat 第 57 条的理解，Frank 改口径时改这一行）

## F-RCL-03 撤回的展示与同步

### TC-RCL-03-01 sdk-ts：收到撤回后内存搜不到原文
- 关联 AC：AC-RCL-03-1
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（sdk-ts）
- 环境：SDK 单测
- 前置条件：已加载含 `ZXQ-RCL-2` 的消息
- 步骤：
  1. `FakeTransport` 推 `msgDeleted` 带 `recalled`
  2. 把 SDK 的消息仓库、会话摘要序列化导出
- 预期结果：
  1. 导出内容搜不到 `ZXQ-RCL-2`
  2. 消息 `recalled=true`、`deleted=true`

### TC-RCL-03-02 sdk-kotlin、sdk-swift：本地库和缓存目录搜不到原文和缩略图
- 关联 AC：AC-RCL-03-2
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（sdk-kotlin、sdk-swift）
- 环境：SDK 单测
- 前置条件：本地库有含特征串的文本和一张已缓存缩略图的图片消息
- 步骤：
  1. 推两条 `msgDeleted`（带 `recalled`）
  2. 关闭数据库，读数据库文件（含 WAL）原始字节；列缓存目录
- 预期结果：
  1. 搜不到特征串
  2. 缩略图和原图的缓存文件不存在

### TC-RCL-03-03 断网期间被撤回：重连后变撤回文案
- 关联 AC：AC-RCL-03-3
- 优先级：P0
- 批次：C4
- 层级：端到端 + SDK 单测
- 环境：E1 + 三端客户端；SDK 单测
- 前置条件：u2 已加载 m1 后断网
- 步骤：
  1. u1 撤回 m1
  2. u2 恢复网络
- 预期结果：
  1. 重连后 `convs` 的 `modVer` 变了，SDK 只对这个会话重拉
  2. u2 看到"对方撤回了一条消息"，内存搜不到原文

### TC-RCL-03-04 App 被杀时被撤回：冷启动后变撤回文案，本地库搜不到原文
- 关联 AC：AC-RCL-03-4
- 优先级：P0
- 批次：C4
- 层级：原生 UI 测试 + SDK 单测
- 环境：模拟器（Android、iOS）；SDK 单测
- 前置条件：u2 本地库有 m1（含特征串），杀掉 App
- 步骤：
  1. u1 撤回 m1
  2. u2 冷启动并连接
  3. 读本地库文件
- 预期结果：
  1. m1 显示撤回文案
  2. 本地库搜不到特征串

### TC-RCL-03-05 最后一条被撤回：lastMsg 带 deleted、recalled，没有 text，modVer 增加
- 关联 AC：AC-RCL-03-5
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：会话最后一条是 m1；记下 `modVer`
- 步骤：
  1. 撤回 m1
  2. 查 `convs`
- 预期结果：
  1. `lastMsg.deleted=true`、`recalled=true`、`recalledBy`，没有 `text`
  2. `modVer` 比之前大

### TC-RCL-03-06 群里撤回自己的最后一条：列表摘要按规则 1a 第 2 行
- 关联 AC：AC-RCL-03-6
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：群 g1，u1 发了最后一条
- 步骤：
  1. u1 撤回
  2. u1 和 n 看会话列表
- 预期结果：
  1. u1："你撤回了一条消息"
  2. n："{u1 显示名}撤回了一条消息"

### TC-RCL-03-07 撤回的 msg、sync、事件帧符合 Schema
- 关联 AC：AC-RCL-03-7
- 优先级：P0
- 批次：C4
- 层级：单测（服务端）+ 后端 e2e
- 环境：E1
- 前置条件：`protocol/vectors/frames.json` 的 C4 用例（BE-470）
- 步骤：
  1. 服务端解析和编码全部撤回用例
  2. e2e 里抓真实的 `sync` 应答、`msgDeleted` 事件，用 Schema 校验
- 预期结果：
  1. 全部通过
  2. 没有 `body`，`deleted`、`recalled` 都是 true

### TC-RCL-03-08 全屏看的图片被撤回：退出全屏并提示
- 关联 AC：AC-RCL-03-8
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）（E1 + 假 S3）
- 前置条件：u2 全屏打开 u1 发的图片
- 步骤：
  1. u1 撤回这张图
- 预期结果：
  1. u2 退出全屏，提示"该消息已被撤回"
  2. 气泡变撤回文案，缓存的图片被丢掉（H5 `blob:` 已 revoke）

### TC-RCL-03-09 隐藏模式消息被撤回：假通道没有调用（等 C3）
- 关联 AC：AC-RCL-03-9
- 优先级：P0
- 批次：C4（依赖 C3，C3 恢复后执行）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3：BE-406、BE-455、BE-456）
- 前置条件：u2 离线、隐藏模式、有令牌；u1 发消息（产生一次推送）
- 步骤：
  1. u1 撤回
  2. 给哨兵 `s` 发一条
- 预期结果：
  1. `s` 被调用后，u2 的令牌除了原来那一次之外没有调用
  2. **C3 恢复前状态记"未执行（等 C3）"**（PENDING P-43 ②）

### TC-RCL-03-10 单聊撤回文案；提示行没有长按菜单（三套 SDK）
- 关联 AC：AC-RCL-03-10
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）
- 环境：SDK 单测
- 前置条件：u1、u2 单聊
- 步骤：
  1. u1 撤回
  2. 两边取 SDK 给出的撤回展示数据
  3. 对撤回提示行调用菜单判断
- 预期结果：
  1. u2 侧是"对方撤回了一条消息"，u1 侧是"你撤回了一条消息"
  2. 菜单项为空

### TC-RCL-03-11 重连只对 modVer 变了的会话发 sync（三套 SDK）
- 关联 AC：AC-RCL-03-11
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）
- 环境：SDK 单测
- 前置条件：u2 已加载 c1、c2、c3
- 步骤：
  1. 断开期间只有 c1 的 `modVer` 变化
  2. 重连
- 预期结果：
  1. 只对 c1 发 `sync` 重拉
  2. c2、c3 没有请求

### TC-RCL-03-12 文件下载中被撤回：中止，提示，不留部分文件
- 关联 AC：AC-RCL-03-12
- 优先级：P0
- 批次：C4
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器（Android、iOS）（E1 + 假 S3，假 S3 限速）
- 前置条件：u2 正在下载 u1 发的 20 MB 文件
- 步骤：
  1. u1 撤回
  2. 列下载缓存目录
- 预期结果：
  1. 下载中止，提示"该消息已被撤回"
  2. 缓存目录里没有半截文件

### TC-RCL-03-13 迟到的旧 sync 不恢复原文和引用（三套 SDK）
- 关联 AC：AC-RCL-03-13
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）+ 共享向量
- 环境：SDK 单测
- 前置条件：m1 已撤回、本地已清除；m2 引用 m1
- 步骤：
  1. 喂一个迟到的旧 `sync` 应答：m1 `deleted=false` 带原文，m2 `quote.state=normal` 带原文
  2. 读内存和本地库
- 预期结果：
  1. m1、m2 都不恢复原文
  2. 本地库搜不到特征串；`chat-sync.json` 的同名用例通过

## F-RCL-04 和后台删除的关系

### TC-RCL-04-01 后台删除已撤回的消息：200，状态仍是撤回，没有事件，有审计
- 关联 AC：AC-RCL-04-1
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：`IM_V13_ADMIN_FEATURES=1`，m1 已撤回
- 步骤：
  1. `POST /admin/messages/delete` m1（经 console 一次、直接调 im-server 一次）
  2. 查后台元数据
  3. 哨兵消息
- 预期结果：
  1. 200
  2. `recalled=true`、`deleted=false`
  3. 没有 `msgDeleted`
  4. im-server 和 console 审计都有记录

### TC-RCL-04-02 撤回已被后台删除的消息：4044
- 关联 AC：AC-RCL-04-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：m1 被后台删除
- 步骤：
  1. u1 在时限内撤回 m1
- 预期结果：
  1. 4044，状态不变（`deleted`，没有 `recalled`）

### TC-RCL-04-03 群撤回消息：普通正文授权没有 body、deleted=false；隔离授权返回原文
- 关联 AC：AC-RCL-04-3
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：群消息 m1（含特征串）被撤回
- 步骤：
  1. 普通正文授权读取 g1
  2. 隔离授权读取 g1
- 预期结果：
  1. 前者 m1 没有 `body`，`recalled=true`，`deleted=false`
  2. 后者返回原文

### TC-RCL-04-04 31 天后 quarantine_purge 删除撤回的隔离记录
- 关联 AC：AC-RCL-04-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：m1 被撤回
- 步骤：
  1. [IM 钟] 推进 31 天，运行 `quarantine_purge`
  2. 隔离授权读取
- 预期结果：
  1. 隔离记录被物理删除（查表）
  2. 404 / 4044

### TC-RCL-04-05 单聊撤回、IM_C2C_BODY_ACCESS=off：申请隔离授权 403
- 关联 AC：AC-RCL-04-5
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：单聊消息被撤回，`IM_C2C_BODY_ACCESS=off`
- 步骤：
  1. 申请隔离授权
- 预期结果：
  1. 403 / 4030，`reason=c2cBodyDisabled`

### TC-RCL-04-06 撤回的图片：参与者申请地址 404
- 关联 AC：AC-RCL-04-6
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 发图片，u2 能取到地址；u1 撤回
- 步骤：
  1. u2 `POST /v1/files/urls`
- 预期结果：
  1. 该项 `code=4048`，响应和不存在的 `fileId` 相同

### TC-RCL-04-07 元数据查询：有 recalled*，没有 body 和文件名
- 关联 AC：AC-RCL-04-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：撤回的文本和文件消息
- 步骤：
  1. `POST /admin/convs/messages/query`（im-server 和 console 各查一次）
- 预期结果：
  1. 有 `recalled`、`recalledAt`、`recalledBy`
  2. 没有 `body`；文件消息没有文件名，也没有 `fileSize`

### TC-RCL-04-08 撤回的文件：files.name 为空，文件名在隔离区（密文）
- 关联 AC：AC-RCL-04-8
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：文件名含 `ZXQ-RCL-3` 的文件消息被撤回
- 步骤：
  1. 查 `files.name`
  2. 查隔离表原始字节，再解密
- 预期结果：
  1. `files.name` 为空
  2. 原始字节搜不到文件名，解密后对得上

### TC-RCL-04-09 fetch：没有授权 4039、隔离授权成功、普通调用方 4030
- 关联 AC：AC-RCL-04-9
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：撤回的图片；调用方：没有授权的正文类调用方、有 `msg.quarantine` 且有有效隔离授权的、普通调用方
- 步骤：
  1. 分别 `POST /admin/files/{fileId}/fetch`
  2. 经 console 代理再走一遍（CS-202）
  3. 请求体带 `grantId`（PRD-chat v1.6 第 11.6 节，P-44 ④）：同一调用方 + 操作人在这个会话另有一个有效隔离授权时，分别带对的、别的会话的、非隔离的、别的操作人的、别的调用方的、不存在的、过期的 `grantId`；再不带 `grantId`
- 预期结果：
  1. 依次 403 / 4039、200（字节和原图 SHA-256 一致，`Content-Disposition: attachment`、`nosniff`）、403 / 4030
  2. console 响应同样带 `attachment`；console 审计有 `fileId`、`msgId`，搜不到文件名
  3. 只有对的 `grantId` 200，其余都是 403 / 4039（不回退到按会话找）；不带 `grantId` 行为和 v1.5 相同；console 转给 im-server 的请求体带这个浏览器会话的 `grantId`，URL 和日志里没有

---

## 说明

- 用例编号按 AC 编号对齐（TC-RCL-0X-YY 对应 AC-RCL-0X-Y）。
- TC-RCL-03-09 依赖 C3（BE-481），C3 恢复前不执行，见 PENDING P-43 ②。
- TC-RCL-01-10 ②、TC-RCL-02-07 管理员一行按 backend-chat 第 54、57 条的"建议"写，Frank 改口径时同步改用例，不改 AC。

