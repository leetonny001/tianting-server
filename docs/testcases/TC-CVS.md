# 测试用例：会话置顶、免打扰、删除（CVS，C5）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4.1、4.5 节，第 9 节 F-CVS-01～04，第 11～15 节的 C5 部分。任务单：[backend-chat](../tasks/backend-chat.md) BE-482～BE-490，[console-chat](../tasks/console-chat.md) CS-205，[frontend-chat](../tasks/frontend-chat.md) FE-680～FE-686。
> 范围：标了【C5】的 AC-CVS（Frank 2026-11-10"开始 C5"，QUEUE Q29）。**TC-CVS-02-01、02-08、04-03 依赖 C3 的推送 worker 和假通道（BE-490），C3 恢复前记"未执行（等 C3）"**；TC-CVS-02-01 的 `unread` 部分在 C5 先验（记"部分通过"）。同样挡在 C3 的 AC-MEN-03-2、03-3 在 [TC-MEN](TC-MEN.md)（TC-MEN-03-02、03-03），AC-APN-04-4 在 [TC-APN](TC-APN.md)（TC-APN-04-04）；【全部】的 C5 部分在 [TC-CHAT](TC-CHAT.md)（TC-CHAT-90-35～39）。
>
> 通用约定：
> - 环境：**E1**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库，`IM_CONV_SETTINGS_ENABLED=1`；涉及 @ 的用例再加 `IM_MSG_ACTIONS_ENABLED=1` 和测试里现场生成的 `IM_QUARANTINE_KEY`）；**E1 + 假 S3**（再加 `objstoretest`，`IM_FILES_ENABLED=1`）；**E1 + console-server**（`CON_TEST_MODE=1`，Postgres，`IM_V13_ADMIN_FEATURES=1`）；**E1 双节点**（`e2e/social_twonode_test.go` 的 `twoNodes`：两个 im-server 共享同一个 SQLite 文件，测试模式 Invalidator 互发）；**E1 + 假通道（等 C3）**（BE-406 的假 APNs、假 FCM，`IM_OFFLINE_PUSH_ENABLED=1`，推送用例用账号登录的 token）；**SDK 单测**（三套 SDK：`FakeTransport` + 假时钟 + 本地库，向量按 `protocol/vectors`）；**Playwright（H5）**、**模拟器**（Android 模拟器、iOS Simulator，连 E1）；**E1 + 三端客户端**（三端先后连同一个 E1）。开发机不安装软件。
> - **CI 现在不运行（PENDING P-21）**，结果都是本地结果，记录写明环境和提交号。真机在 P-13。
> - 层级：**后端 e2e**（Go e2e，对 `/ws`、`/v1/*`、`/admin/*` 断言）、**SDK 单测**、**Playwright**、**原生 UI 测试**（Compose UI Test、XCUITest）、**端到端**（真实 im-server + 客户端）。标了 Playwright 和原生 UI 测试的用例 H5、Android、iOS 都要跑，某端不适用的按 AC 的括号。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算。限流用例之间用 `/admin/test/limits/reset` 清计数。
> - 特征串：`ZXQ-CVS-1`（单聊旧消息正文）、`ZXQ-CVS-2`（旧文件名和缓存文件内容）。"搜不到"指对原始字节做子串搜索（SQLite 先 `PRAGMA wal_checkpoint(TRUNCATE)`）。
> - 记号：`u1`、`u2` 单聊双方；`D1`、`D2` 是 u1 的两台设备；群 `g1`：群主 `o`、成员 `u1`、`u2`；`u9` 不是成员；负向的"没有事件 / 没有调用"用哨兵法（之后再发一条普通消息或给哨兵用户 `s` 发一条，收到它之后断言前面没有）。


| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-CVS-01-01 | 置顶排序：后置顶的在前，其余按时间 | AC-CVS-01-1 | 后端 e2e | P0 |
| TC-CVS-01-02 | 置顶上限：第 21 个 4220；已置顶的再次置顶成功、pinnedAt 不变 | AC-CVS-01-2 | 后端 e2e | P0 |
| TC-CVS-01-03 | 501 个会话、最旧的置顶：返回 500 个且含它，truncated=true | AC-CVS-01-3 | 后端 e2e | P0 |
| TC-CVS-01-04 | convSet 的权限和格式：4032、4033、4040、4043、4000 | AC-CVS-01-4 | 后端 e2e | P0 |
| TC-CVS-01-05 | 移出再加入：置顶和免打扰都没有了 | AC-CVS-01-5 | 后端 e2e | P0 |
| TC-CVS-01-06 | 会话菜单：长按、右键、键盘菜单键；三项；置顶背景区分 | AC-CVS-01-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CVS-02-01 | 单聊免打扰、离线：令牌无调用，unread 增加（推送部分等 C3） | AC-CVS-02-1 | 后端 e2e | P0 |
| TC-CVS-02-02 | 普通未读 2、3，免打扰未读 5：角标和标题显示 5 | AC-CVS-02-2 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-CVS-02-03 | 免打扰会话未读 5：小红点、"[5 条]"、免打扰图标 | AC-CVS-02-3 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CVS-02-04 | 未读全在免打扰会话：document.title 为"天听" | AC-CVS-02-4 | Playwright（H5） | P0 |
| TC-CVS-02-05 | 免打扰未读 5 其中 1 条 @ 我，普通未读 2：角标 3 | AC-CVS-02-5 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-CVS-02-06 | App 前台收到免打扰会话的消息：列表更新，不响铃不弹通知 | AC-CVS-02-6 | 原生 UI 测试 | P0 |
| TC-CVS-02-07 | 开关关：convSet、convDelete 403 / 4030 featureDisabled | AC-CVS-02-7 | 后端 e2e | P0 |
| TC-CVS-02-08 | 设置读取故障：按没有免打扰推送，消息正常（等 C3） | AC-CVS-02-8 | 后端 e2e | P0 |
| TC-CVS-03-01 | 删除单聊：自己列表没有、sync 空；对方和后台查询不受影响 | AC-CVS-03-1 | 后端 e2e | P0 |
| TC-CVS-03-02 | 删除后对方再发 1 条：重新出现，未读 1，只看到这 1 条，minSeq=11 | AC-CVS-03-2 | 端到端 + 后端 e2e | P0 |
| TC-CVS-03-03 | upToSeq 大于 maxSeq 清到 maxSeq；小于时清到 upToSeq、未读 2、不消失 | AC-CVS-03-3 | 后端 e2e | P0 |
| TC-CVS-03-04 | 删除群会话：仍是成员、在 myGroups、能发；deleted 含 g1 | AC-CVS-03-4 | 后端 e2e | P0 |
| TC-CVS-03-05 | D2 离线时 D1 删除 g1：D2 重连后 g1 消失，不出现"你已不在该群" | AC-CVS-03-5 | SDK 单测（三套）+ 共享向量 | P0 |
| TC-CVS-03-06 | 免打扰且置顶的群删除后有新消息：不再置顶，仍免打扰 | AC-CVS-03-6 | 后端 e2e | P0 |
| TC-CVS-03-07 | 清除范围内的图片：自己 4048，对方仍可以 | AC-CVS-03-7 | 后端 e2e | P0 |
| TC-CVS-03-08 | 确认删除清除草稿、失败消息、本地缓存；取消时不变 | AC-CVS-03-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CVS-03-09 | convDelete 的权限：4032、4033、4043 | AC-CVS-03-9 | 后端 e2e | P0 |
| TC-CVS-03-10 | D2 离线、本地有含特征串的单聊：D1 删除后 D2 上线，本地库和缓存搜不到 | AC-CVS-03-10 | SDK 单测（sdk-kotlin、sdk-swift）+ 原生 UI 测试 | P0 |
| TC-CVS-03-11 | 只设免打扰的空群照常显示；删除空群隐藏、deleted 含它、有消息后重新出现 | AC-CVS-03-11 | 后端 e2e | P0 |
| TC-CVS-03-12 | 删除群会话后移出再加入（historyVisible=true）：看到全部历史，deleted 不再含 g1 | AC-CVS-03-12 | 后端 e2e | P0 |
| TC-CVS-03-13 | 只有 tip 也重新出现；clearedSeq 不回退；重复删除幂等 | AC-CVS-03-13 | 后端 e2e | P0 |
| TC-CVS-03-14 | upToSeq=10 的删除和 seq 11 的发送并发：会话最终可见，seq 11 能拉到 | AC-CVS-03-14 | 后端 e2e | P0 |
| TC-CVS-03-15 | u1 删除会话后其他成员申请文件地址照常，对象还在 | AC-CVS-03-15 | 后端 e2e | P0 |
| TC-CVS-04-01 | D1 置顶、免打扰：D2 2 秒内更新 | AC-CVS-04-1 | 端到端 + 后端 e2e | P0 |
| TC-CVS-04-02 | D2 离线时 D1 置顶并删除另一个会话：D2 重连后和 D1 一致 | AC-CVS-04-2 | 端到端 + 后端 e2e | P0 |
| TC-CVS-04-03 | 双节点：节点 1 设免打扰后断开，节点 2 的假通道无调用（等 C3） | AC-CVS-04-3 | 后端 e2e | P0 |
| TC-CVS-04-04 | 1 秒内 6 次 convSet：第 6 次 4290 | AC-CVS-04-4 | 后端 e2e | P0 |
| TC-CVS-04-05 | D1 删除会话：D2 2 秒内收到 convDeleted，会话消失，本地消息清除 | AC-CVS-04-5 | 端到端 + 后端 e2e | P0 |

---

## F-CVS-01 置顶

### TC-CVS-01-01 置顶排序：后置顶的在前，其余按时间
- 关联 AC：AC-CVS-01-1
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 有 3 个会话 c1、c2、c3，按 [IM 钟] 先后各有一条消息（c1 最旧，c3 最新）；再加一个 c4 比 c3 更新
- 步骤：
  1. u1 `convSet {convId:c1, pinned:true}`，推进 [IM 钟] 1 秒后 `convSet {convId:c2, pinned:true}`
  2. u1 `convs`
- 预期结果：
  1. 两次 ack 的 `conv.pinned=true`，`pinnedAt` 是服务端时间，c2 的更大
  2. `convs` 顺序 c2、c1、c4、c3（置顶按 `pinnedAt` 倒序在前，其余按 `updatedAt` 倒序）
  3. 再造两个 `updatedAt` 相同的普通会话，按 `convId` 升序（AC-CONV-01-4 的规则不变）

### TC-CVS-01-02 置顶上限：第 21 个 4220；已置顶的再次置顶成功、pinnedAt 不变
- 关联 AC：AC-CVS-01-2
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1（`IM_CONV_PIN_MAX` 默认 20）
- 前置条件：u1 有 21 个有消息的会话，已置顶其中 20 个
- 步骤：
  1. 置顶第 21 个
  2. 对已置顶的 c1 再次 `convSet {pinned:true}`，记下前后的 `pinnedAt`
  3. 取消 c1 的置顶后再置顶第 21 个
- 预期结果：
  1. 第 1 步 ack 错误 4220，`convs` 里第 21 个没有置顶
  2. 第 2 步成功，`pinnedAt` 不变
  3. 第 3 步成功（名额按当前置顶数算）

### TC-CVS-01-03 501 个会话、最旧的置顶：返回 500 个且含它，truncated=true
- 关联 AC：AC-CVS-01-3
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 有 501 个会话（用管理接口批量造），最旧的是 c-old
- 步骤：
  1. 置顶 c-old
  2. `convs`
- 预期结果：
  1. 返回 500 个，`truncated=true`
  2. c-old 在第一个且 `pinned=true`；其余 499 个是最近更新的（第二旧的那个不在结果里）

### TC-CVS-01-04 convSet 的权限和格式：4032、4033、4040、4043、4000
- 关联 AC：AC-CVS-01-4
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1、u2 的单聊有消息；u9 不是这个单聊的参与方，也不是 g1 的成员；u1 和 u3 的单聊没有消息；g2 已解散
- 步骤：
  1. u9 对 `c2c:u1:u2` 发 `convSet {pinned:true}`
  2. u9 对 g1 发 `convSet {notifyOff:true}`
  3. u1 对 `c2c:u1:u3` 发 `convSet {pinned:true}`
  4. u1 对 g2 发 `convSet {notifyOff:true}`
  5. u1 对 `c2c:u1:u2` 发 `convSet {convId}`（没有任何字段）
- 预期结果：
  1. 依次 4032、4033、4040、4043、4000（第 5 步 400 / 4000）
  2. 都没有写入：`convs` 里这些会话没有设置，u1 的其他连接没有 `convSettingsChanged`（哨兵法）

### TC-CVS-01-05 移出再加入：置顶和免打扰都没有了
- 关联 AC：AC-CVS-01-5
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 把 g1 置顶并设免打扰
- 步骤：
  1. 群主 o 把 u1 移出 g1，再拉回来
  2. u1 `convs`
  3. 另做一遍：u1 自己退群后被重新邀请；以及 g1 解散（只查数据库里没有行）
- 预期结果：
  1. g1 的 `pinned=false`（没有 `pinnedAt`）、`notifyOff=false`
  2. 数据库里 u1 在 g1 的 `conv_settings` 行不存在；解散后整个群的行都不存在

### TC-CVS-01-06 会话菜单：长按、右键、键盘菜单键；三项；置顶背景区分
- 关联 AC：AC-CVS-01-6
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5，窄屏 390 和宽屏 1280）+ 模拟器
- 前置条件：u1 有 3 个会话，其中一个已置顶
- 步骤：
  1. Android、iOS 长按会话项；H5 窄屏长按、宽屏右键；H5 用 Tab 聚焦会话项后按菜单键（和 Shift+F10）
  2. 对未置顶的会话打开菜单；对已置顶的打开菜单
  3. 选"置顶"
- 预期结果：
  1. 每种方式都打开同一个菜单（同一 `data-testid` / `testTag`），有"置顶 / 取消置顶""免打扰 / 取消免打扰""删除"，文字按当前状态
  2. 置顶后这一项移到最前，背景色和普通项不同（取 token，深色模式也不同）
  3. `features.convSettings=false` 时长按、右键、菜单键都不打开菜单

## F-CVS-02 免打扰

### TC-CVS-02-01 单聊免打扰、离线：令牌无调用，unread 增加（推送部分等 C3）
- 关联 AC：AC-CVS-02-1
- 优先级：P0
- 批次：C5（推送部分随 BE-490，等 C3）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3）；`unread` 部分 E1
- 前置条件：u2 用账号登录、注册了令牌，把和 u1 的单聊设为免打扰后离线；哨兵 `s` 离线、有令牌
- 步骤：
  1. u1 给 u2 发 1 条
  2. 给 `s` 发 1 条，等假通道记录到 `s` 的调用
  3. u2 上线 `convs`
- 预期结果：
  1. u2 的令牌没有调用
  2. u2 和 u1 的会话 `unread` 增加 1，`notifyOff=true`
  3. C3 恢复前：第 3 步在 E1 先验（记"部分通过"），第 1、2 步记"未执行（等 C3）"

### TC-CVS-02-02 普通未读 2、3，免打扰未读 5：角标和标题显示 5
- 关联 AC：AC-CVS-02-2
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：Playwright（H5）+ 模拟器；SDK 单测（三套）
- 前置条件：u1 有两个普通会话未读 2、3，一个免打扰会话未读 5（没有 @）
- 步骤：
  1. 看"消息"标签角标；H5 看 `document.title`
  2. SDK 单测：`totalUnread()` 和 `totalUnread` 事件
- 预期结果：
  1. 角标显示"5"；H5 标题"(5) 天听"
  2. `totalUnread()=5`；取消免打扰后变成 10

### TC-CVS-02-03 免打扰会话未读 5：小红点、"[5 条]"、免打扰图标
- 关联 AC：AC-CVS-02-3
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器
- 前置条件：u1 的免打扰会话未读 5，没有 @、没有草稿
- 步骤：
  1. 看会话列表这一项
  2. 给这个会话写草稿；再让对方 @ u1（群）
- 预期结果：
  1. 头像右上角是小红点（没有数字），摘要前"[5 条]"，右侧有免打扰图标（有可读名称）
  2. 有草稿时前缀是"[草稿]"；有 @ 时是"[有人@我]"（F-MEN-03 规则 2 的优先级）

### TC-CVS-02-04 未读全在免打扰会话：document.title 为"天听"
- 关联 AC：AC-CVS-02-4
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）
- 环境：Playwright（H5）
- 前置条件：u1 只有免打扰会话有未读（合计 7，没有 @），普通会话未读 0
- 步骤：
  1. 看 `document.title` 和"消息"标签角标
- 预期结果：
  1. `document.title` 为"天听"；角标不显示

### TC-CVS-02-05 免打扰未读 5 其中 1 条 @ 我，普通未读 2：角标 3
- 关联 AC：AC-CVS-02-5
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：Playwright（H5）+ 模拟器（`IM_MSG_ACTIONS_ENABLED=1`）；SDK 单测（三套）
- 前置条件：免打扰的群 g1 未读 5，其中 1 条 @ u1（`mentionUnread=1`）；普通会话未读 2
- 步骤：
  1. 看"消息"标签角标；H5 看标题
  2. SDK 单测：`totalUnread()`
- 预期结果：
  1. 角标"3"，H5 标题"(3) 天听"
  2. `totalUnread()=3`；u1 读到 @ 那条之后变成 2

### TC-CVS-02-06 App 前台收到免打扰会话的消息：列表更新，不响铃不弹通知
- 关联 AC：AC-CVS-02-6
- 优先级：P0
- 批次：C5（C3 恢复加通知后由 FE-653、FE-655 补跑）
- 层级：原生 UI 测试
- 环境：模拟器（Android、iOS），连 E1
- 前置条件：u1 的 App 在前台、停在会话列表；和 u2 的单聊、群 g1 都设了免打扰
- 步骤：
  1. u2 在单聊、在 g1 各发 1 条
  2. 观察会话列表；Android 查 `NotificationManager.getActiveNotifications()`，iOS 查 `UNUserNotificationCenter` 已送达的通知；两端用提示音替身记录调用
- 预期结果：
  1. 列表 2 秒内更新（摘要、"[N 条]"、小红点）
  2. 没有系统通知，提示音替身没有调用，没有震动
  3. 记录这次运行时 App 里有没有通知代码（C5 时没有），C3 恢复后同一用例补跑

### TC-CVS-02-07 开关关：convSet、convDelete 403 / 4030 featureDisabled
- 关联 AC：AC-CVS-02-7
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1，`IM_CONV_SETTINGS_ENABLED=0`
- 前置条件：u1、u2 的单聊有消息
- 步骤：
  1. u1 `convSet {pinned:true}`、`convSet {notifyOff:true}`
  2. u1 `convDelete {upToSeq:1}`
  3. `GET /v1/config`
- 预期结果：
  1. 都是 403 / 4030，`reason=featureDisabled`；没有写入
  2. `features.convSettings=false`
  3. `convs` 的输出和打开开关前的 C4 版本相同（没有 `deleted`、`pinned` 等字段）

### TC-CVS-02-08 设置读取故障：按没有免打扰推送，消息正常（等 C3）
- 关联 AC：AC-CVS-02-8
- 优先级：P0
- 批次：C5（随 BE-490，等 C3）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3）
- 前置条件：u2 把和 u1 的单聊设为免打扰后离线，有令牌；`POST /admin/test/faults` 打开 `convSettingsRead`（BE-490 第 2 条）
- 步骤：
  1. u1 给 u2 发 1 条
  2. 关闭故障注入
- 预期结果：
  1. 发送 ack 成功，消息保存
  2. u2 的令牌有 1 次调用（按没有免打扰推送）
  3. C3 恢复前记"未执行（等 C3）"

## F-CVS-03 删除会话

### TC-CVS-03-01 删除单聊：自己列表没有、sync 空；对方和后台查询不受影响
- 关联 AC：AC-CVS-03-1
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1；第 4 步 E1 + console-server（CS-205）
- 前置条件：u1、u2 单聊 10 条，第 3 条含特征串 `ZXQ-CVS-1`
- 步骤：
  1. u1 `convDelete {upToSeq:10}`
  2. u1 `convs`、`sync {afterSeq:0}`
  3. u2 `convs`、`sync`
  4. 客服用 `msg.meta` 查这个会话；授权后读正文
- 预期结果：
  1. ack `{clearedSeq:10}`
  2. u1 的列表没有这个会话，`deleted` 含 `{convId, clearedSeq:10}`；`sync` 返回空
  3. u2 的会话和 10 条消息都在，未读不变
  4. 后台元数据和正文照常返回 10 条（F-CVS-03 规则 2"后台查询不受影响"，没有单独的 AC，在这里验，backend-chat 第 87 条）

### TC-CVS-03-02 删除后对方再发 1 条：重新出现，未读 1，只看到这 1 条，minSeq=11
- 关联 AC：AC-CVS-03-2
- 优先级：P0
- 批次：C5
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端；后端部分 E1
- 前置条件：同 TC-CVS-03-01 第 1 步之后；三端轮换 u1 的端
- 步骤：
  1. u2 发 1 条
  2. u1 看会话列表、打开会话、向上翻页；查 `convs`
- 预期结果：
  1. u1 的列表重新出现这个会话，未读 1，摘要是新消息
  2. 打开后只有这 1 条，翻页不出现旧消息；`convs` 里 `minSeq=11`，`deleted` 不再含它
  3. 客户端内存（和原生的本地库）搜不到 `ZXQ-CVS-1`

### TC-CVS-03-03 upToSeq 大于 maxSeq 清到 maxSeq；小于时清到 upToSeq、未读 2、不消失
- 关联 AC：AC-CVS-03-3
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：两个单聊 A、B 各 10 条，u1 都没读
- 步骤：
  1. A：`convDelete {upToSeq:999}`
  2. B：`convDelete {upToSeq:8}`
  3. `convs`；B `sync {afterSeq:0}`
- 预期结果：
  1. A 的 ack `clearedSeq=10`，A 不在列表，在 `deleted`
  2. B 的 ack `clearedSeq=8`；B 仍在列表，`unread=2`、`minSeq=9`；`sync` 只返回 seq 9、10

### TC-CVS-03-04 删除群会话：仍是成员、在 myGroups、能发；deleted 含 g1
- 关联 AC：AC-CVS-03-4
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1 有 5 条消息
- 步骤：
  1. u1 `convDelete {convId:g1, upToSeq:5}`
  2. 查 `groupMembers`、`myGroups`；u1 `send` 到 g1
- 预期结果：
  1. 删除后 `convs` 没有 g1，`deleted` 含 `{g1, 5}`
  2. u1 仍在成员列表，`myGroups` 里有 g1；`send` 成功
  3. 发送后 g1 回到 u1 的列表，只看到自己发的这条

### TC-CVS-03-05 D2 离线时 D1 删除 g1：D2 重连后 g1 消失，不出现"你已不在该群"
- 关联 AC：AC-CVS-03-5
- 优先级：P0
- 批次：C5
- 层级：SDK 单测（三套）+ 共享向量
- 环境：SDK 单测（`FakeTransport`）；三套 SDK 跑 `chat-sync.json` 的 `deleted` 判断用例（BE-487）；另在 E1 上做一次真实服务端集成
- 前置条件：D2 已加载 g1（有消息），然后离线
- 步骤：
  1. D1 删除 g1 的会话
  2. D2 重连，拉 `convs`（g1 不在 `convs`，在 `deleted`）
  3. D2 从通讯录（`myGroups`）进入 g1 发言
  4. 对照：另一个群 g2 既不在 `convs` 也不在 `deleted`
- 预期结果：
  1. g1 从 D2 的列表消失，**没有** `convRemoved` 事件，没有 `removed` 快照，不显示"你已不在该群"
  2. 进入 g1 是空的聊天页，可以发送
  3. g2 按 F-GRP-03 规则 4 推断为"你已不在该群"；`truncated` 或 `deletedTruncated` 为 true 时不推断（frontend-chat 第 54 条）

### TC-CVS-03-06 免打扰且置顶的群删除后有新消息：不再置顶，仍免打扰
- 关联 AC：AC-CVS-03-6
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 把 g1 置顶、设免打扰
- 步骤：
  1. u1 删除 g1 的会话
  2. o 在 g1 发 1 条
  3. u1 `convs`
- 预期结果：
  1. g1 重新出现，`pinned=false`、没有 `pinnedAt`，`notifyOff=true`
  2. g1 排在普通会话里（按 `updatedAt`），不占置顶名额

### TC-CVS-03-07 清除范围内的图片：自己 4048，对方仍可以
- 关联 AC：AC-CVS-03-7
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1、u2 单聊里 u2 发过一张图片（seq 3），之后还有 2 条
- 步骤：
  1. u1 `convDelete {upToSeq:5}`
  2. u1 `POST /v1/files/urls` 申请这张图片的原图和缩略图
  3. u2 申请同一张
- 预期结果：
  1. u1 逐项 `code=4048`，和文件不存在完全相同
  2. u2 拿到地址，能下载

### TC-CVS-03-08 确认删除清除草稿、失败消息、本地缓存；取消时不变
- 关联 AC：AC-CVS-03-8
- 优先级：P0
- 批次：C5
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器
- 前置条件：u1 和 u2 的会话有草稿、一条发送失败的消息（断网时发）、看过的图片（H5 有 `blob:`，原生有缓存文件，内容含 `ZXQ-CVS-2`）
- 步骤：
  1. 菜单选"删除"，看确认框，点"取消"
  2. 再选"删除"，点确认
  3. 从通讯录重新进入和 u2 的聊天页
- 预期结果：
  1. 确认框文字"删除后聊天记录不再显示，确定删除？"；取消后会话、草稿、失败消息、图片都还在
  2. 确认后会话从列表消失；草稿、失败消息没有了；H5 的 `blob:` 已释放，原生缓存目录搜不到 `ZXQ-CVS-2`
  3. 聊天页是空的，输入框没有草稿

### TC-CVS-03-09 convDelete 的权限：4032、4033、4043
- 关联 AC：AC-CVS-03-9
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u9 不是 `c2c:u1:u2` 的参与方，也不是 g1 的成员；g2 已解散
- 步骤：
  1. u9 `convDelete {convId:c2c:u1:u2, upToSeq:1}`
  2. u9 `convDelete {convId:g1, upToSeq:1}`
  3. u1 `convDelete {convId:g2, upToSeq:1}`
  4. u1 删除和 u3 没有消息的单聊（规则 8）
- 预期结果：
  1. 依次 4032、4033、4043，没有写入
  2. 第 4 步幂等成功（`clearedSeq=0`）

### TC-CVS-03-10 D2 离线、本地有含特征串的单聊：D1 删除后 D2 上线，本地库和缓存搜不到
- 关联 AC：AC-CVS-03-10
- 优先级：P0
- 批次：C5
- 层级：SDK 单测（sdk-kotlin、sdk-swift）+ 原生 UI 测试
- 环境：SDK 单测（本地库是真实 SQLite 文件）；模拟器 + E1 + 假 S3
- 前置条件：u1 的 D2 本地库有和 u2 的单聊，正文含 `ZXQ-CVS-1`，缓存目录有一张内容含 `ZXQ-CVS-2` 的图片；D2 离线
- 步骤：
  1. D1 删除这个单聊
  2. u2 再发一条（不引用旧消息，backend-chat 第 86 条）
  3. D2 上线，等 `convs` 处理完
  4. 对 D2 的本地库文件（含 `-wal`）和缓存目录做字节搜索
- 预期结果：
  1. 搜不到 `ZXQ-CVS-1`、`ZXQ-CVS-2`
  2. D2 只显示新消息；冷启动后仍然只有新消息
  3. 本地清除失败（注入）时下次启动再清，最终搜不到

### TC-CVS-03-11 只设免打扰的空群照常显示；删除空群隐藏、deleted 含它、有消息后重新出现
- 关联 AC：AC-CVS-03-11
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：两个没有消息的托管群 g3、g4（管理接口建群，u1 是成员）
- 步骤：
  1. u1 对 g3 `convSet {notifyOff:true}`
  2. u1 对 g4 `convDelete {upToSeq:0}`
  3. `convs`
  4. g4 有一条消息（管理接口发系统消息或成员发言）后再 `convs`
- 预期结果：
  1. g3 在列表里（`lastMsg=null`，`notifyOff=true`）
  2. g4 不在列表，`deleted` 含 `{g4, 0}`
  3. 有消息后 g4 重新出现，`deleted` 不再含它

### TC-CVS-03-12 删除群会话后移出再加入（historyVisible=true）：看到全部历史，deleted 不再含 g1
- 关联 AC：AC-CVS-03-12
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1 `historyVisible=true`，有 5 条消息；u1 删除了 g1 的会话
- 步骤：
  1. o 把 u1 移出，再拉回来
  2. u1 `convs`、`sync {afterSeq:0}`
- 预期结果：
  1. `deleted` 不含 g1；g1 在列表里，`minSeq=1`
  2. `sync` 返回全部 5 条（加上移出和加入的 tip）

### TC-CVS-03-13 只有 tip 也重新出现；clearedSeq 不回退；重复删除幂等
- 关联 AC：AC-CVS-03-13
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1 有 6 条消息
- 步骤：
  1. u1 `convDelete {upToSeq:6}`
  2. o 改群名（只产生一条 `tip`）
  3. u1 `convs`
  4. u1 `convDelete {upToSeq:3}`（小于已有的 6）
  5. u1 `convDelete {upToSeq:7}` 两次
- 预期结果：
  1. 第 3 步 g1 重新出现，只能看到这条 tip，未读 1
  2. 第 4 步成功，ack `clearedSeq=6`（不回退），数据库里仍是 6
  3. 第 5 步两次都成功，`clearedSeq=7`，g1 隐藏

### TC-CVS-03-14 upToSeq=10 的删除和 seq 11 的发送并发：会话最终可见，seq 11 能拉到
- 关联 AC：AC-CVS-03-14
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1（`go test -race`）
- 前置条件：u1、u2 单聊 10 条，u1 本地看到的最大 seq 是 10
- 步骤：
  1. 同时：u1 `convDelete {upToSeq:10}`，u2 发第 11 条；重复 50 次（每次新会话），两种先后都要出现
  2. 每次结束后 u1 `convs`、`sync {afterSeq:10}`
- 预期结果：
  1. 每次会话都在 u1 的列表里，`clearedSeq=10`
  2. `sync` 都能拉到 seq 11；没有一次 seq 11 被清除

### TC-CVS-03-15 u1 删除会话后其他成员申请文件地址照常，对象还在
- 关联 AC：AC-CVS-03-15
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：g1 里 o 发过一个文件（seq 2）
- 步骤：
  1. u1 删除 g1 的会话（`upToSeq` 到最新）
  2. u2 申请这个文件的地址并下载
  3. 查假 S3 里的对象
- 预期结果：
  1. u2 拿到地址，下载内容一致
  2. 对象还在；`files` 行状态不变

## F-CVS-04 多端同步

### TC-CVS-04-01 D1 置顶、免打扰：D2 2 秒内更新
- 关联 AC：AC-CVS-04-1
- 优先级：P0
- 批次：C5
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端（D1、D2 取不同端轮换，H5 两个标签页也算两个连接）；后端部分 E1
- 前置条件：u1 的 D1、D2 都在线
- 步骤：
  1. D1 置顶会话 c1，再把它设免打扰
  2. 观察 D2
- 预期结果：
  1. D2 2 秒内收到两条 `convSettingsChanged`（`pinned=true` 带 `pinnedAt`；`notifyOff=true`），列表里 c1 移到最前、有置顶背景和免打扰图标
  2. D1 自己只拿 ack，不收这个事件（哨兵法）

### TC-CVS-04-02 D2 离线时 D1 置顶并删除另一个会话：D2 重连后和 D1 一致
- 关联 AC：AC-CVS-04-2
- 优先级：P0
- 批次：C5
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端；后端部分 E1
- 前置条件：u1 的 D2 离线，D2 本地有 c1、c2 的消息
- 步骤：
  1. D1 置顶 c1，删除 c2
  2. D2 重连
- 预期结果：
  1. D2 的列表顺序、置顶、免打扰和 D1 完全相同；c2 不在列表，本地 c2 的消息被清除
  2. D2 冷启动时先显示本地的旧状态，`convs` 回来后以服务端为准

### TC-CVS-04-03 双节点：节点 1 设免打扰后断开，节点 2 的假通道无调用（等 C3）
- 关联 AC：AC-CVS-04-3
- 优先级：P0
- 批次：C5（随 BE-490，等 C3）
- 层级：后端 e2e
- 环境：E1 双节点 + 假通道（等 C3）
- 前置条件：u1 用账号登录、有令牌，连在节点 1；u2 连在节点 2；哨兵 `s` 离线、有令牌
- 步骤：
  1. u1 在节点 1 把和 u2 的单聊设为免打扰，然后断开
  2. 1 秒后 u2 在节点 2 给 u1 发 1 条；再给 `s` 发 1 条
- 预期结果：
  1. 节点 2 的假通道记录到 `s` 的调用后，u1 的令牌没有调用（节点 2 读到了新设置）
  2. C3 恢复前记"未执行（等 C3）"

### TC-CVS-04-04 1 秒内 6 次 convSet：第 6 次 4290
- 关联 AC：AC-CVS-04-4
- 优先级：P0
- 批次：C5
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 有 6 个会话；先 `/admin/test/limits/reset`
- 步骤：
  1. 1 秒内连发 6 次 `convSet`（每次不同会话）
  2. 等 1 秒后再发 1 次；再混合 `convSet` 和 `convDelete` 共 6 次
- 预期结果：
  1. 前 5 次成功，第 6 次 4290
  2. 1 秒后恢复；混合时第 6 次同样 4290（共用一个计数，backend-chat 第 73 条）
  3. 发送限流不受影响：同一秒里 `send` 照常成功

### TC-CVS-04-05 D1 删除会话：D2 2 秒内收到 convDeleted，会话消失，本地消息清除
- 关联 AC：AC-CVS-04-5
- 优先级：P0
- 批次：C5
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端；后端部分 E1
- 前置条件：u1 的 D1、D2 在线，D2 已加载和 u2 的单聊（含 `ZXQ-CVS-1`）
- 步骤：
  1. D1 删除这个单聊
  2. 观察 D2；原生端检查本地库
- 预期结果：
  1. D2 2 秒内收到 `convDeleted {convId, clearedSeq}`，会话从列表消失
  2. D2 的内存（原生还有本地库和缓存目录）搜不到 `ZXQ-CVS-1`；D2 的草稿也被清除（frontend-chat 第 55 条）

---

## 说明

- 用例编号按 AC 编号对齐（TC-CVS-0X-YY 对应 AC-CVS-0X-Y）。
- 等 C3 的 3 条（TC-CVS-02-01 推送部分、02-08、04-03）和 TC-MEN-03-02、03-03、TC-APN-04-04 一起在 BE-490 执行；C5 验收报告按"未执行（等 C3）"记录（backend-chat 第 72 条）。
- 第 4 步之类标了 backend-chat、frontend-chat 问题编号的断言，是按任务单"建议"一列写的口径，Frank 改口径时同步改用例。
