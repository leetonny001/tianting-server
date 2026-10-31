# 测试用例：会话（CONV）

> 依据：[PRD v1.0](../prd/PRD-core-v1.0.md) F-CONV-01～F-CONV-03、[ERRORS.md](../../protocol/ERRORS.md)。`convs` 命令及应答结构以 PRD §8 / F-CONV-01 为准。
>
> 通用约定：
> - 每个用例使用独立数据库，服务端测试模式。
> - 用户 `alice`、`bob`、`carol`、`dave` 已导入；`ghost` 不存在。
> - 未读数 = maxSeq − max(readSeq, minSeq − 1)。
> - "UI 自动化"为 Playwright 多浏览器上下文；"后端 e2e"为测试 WS 客户端直连。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-CONV-01-01 | convs 返回 2 个单聊 + 2 个群（含无消息的群），排序正确、字段完整 | AC-CONV-01-1 | 后端 e2e | P0 |
| TC-CONV-01-02 | 会话入选规则：自聊在列表中，发送失败（4042）的单聊不在列表中 | AC-CONV-01-1 | 后端 e2e | P0 |
| TC-CONV-01-03 | 接收方视角：单聊 name 为对方，lastMsg 为对方发的消息 | AC-CONV-01-1 | 后端 e2e | P0 |
| TC-CONV-01-04 | 会话数 500 时全部返回 | AC-CONV-01-1 | 后端 e2e | P0 |
| TC-CONV-01-05 | 收到新消息的会话移到顶部，摘要和时间更新 | AC-CONV-01-2 | UI 自动化 | P0 |
| TC-CONV-01-06 | 新浏览器上下文登录，会话列表与原设备一致 | AC-CONV-01-3 | UI 自动化 | P0 |
| TC-CONV-01-07 | updatedAt 相同时按 convId 升序（ASCII） | AC-CONV-01-4 | 后端 e2e | P0 |
| TC-CONV-01-08 | 客户端时钟快 1 小时，排序和显示时间不受影响 | AC-CONV-01-5 | UI 自动化 | P0 |
| TC-CONV-01-09 | 没有会话时显示空状态并引导"发起单聊" | AC-CONV-01-6 | UI 自动化 | P0 |
| TC-CONV-02-01 | bob 离线时收到 3 条，登录后未读 3 | AC-CONV-02-1 | UI 自动化 | P0 |
| TC-CONV-02-02 | 打开会话后未读变 0，重新登录后仍为 0 | AC-CONV-02-2 | UI 自动化 | P0 |
| TC-CONV-02-03 | 已读上报节流：1 秒内最多 1 次，值为最新 seq | AC-CONV-02-2 | SDK 单测 | P0 |
| TC-CONV-02-04 | 发言即已读：bob 发 1 条后未读为 0 | AC-CONV-02-3 | 后端 e2e | P0 |
| TC-CONV-02-05 | 未读 99 显示"99"，100 显示"99+" | AC-CONV-02-4 | UI 自动化 | P0 |
| TC-CONV-02-06 | 已读只进不退：先报 5 再报 3，仍为 5 | AC-CONV-02-5 | 后端 e2e | P0 |
| TC-CONV-02-07 | 对无权限会话上报已读返回 4032 / 4033 / 4043 | AC-CONV-02-5 | 后端 e2e | P0 |
| TC-CONV-02-08 | 上报 seq 超过 maxSeq 时截断为 maxSeq，未读不为负 | AC-CONV-02-6 | 后端 e2e | P0 |
| TC-CONV-02-09 | 群成员未读按 minSeq 计算，已读低于 minSeq 不影响 | AC-CONV-02-6 | 后端 e2e | P0 |
| TC-CONV-02-10 | 标签页在后台时不上报已读，切回后上报并清零 | AC-CONV-02-7 | UI 自动化 | P0 |
| TC-CONV-03-01 | 输入合法 userId 进入聊天页 | AC-CONV-03-1 | UI 自动化 | P0 |
| TC-CONV-03-02 | 未发消息的会话不进入服务端会话列表 | AC-CONV-03-1 | UI 自动化 | P0 |
| TC-CONV-03-03 | 输入格式不合法提示"账号格式不正确"，不跳转 | AC-CONV-03-2 | UI 自动化 | P0 |
| TC-CONV-03-04 | 给不存在的用户发第一条消息提示"用户不存在"，会话不进入服务端列表 | AC-CONV-03-3 | UI 自动化 | P0 |

---

## F-CONV-01 会话列表

### TC-CONV-01-01 convs 返回 2 个单聊 + 2 个群（含无消息的群），排序正确、字段完整
- 关联 AC：AC-CONV-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：按以下顺序造数据，每步间隔 ≥ 10ms，保证服务端时间不同：
  1. T1：建群 g1（name="空群"，成员 alice、bob），不发消息。
  2. T2：alice 给 bob 发"to bob"（seq=1）。
  3. T3：carol 给 alice 发"to alice"（seq=1）。
  4. T4：建群 g2（name="热闹群"，成员 alice、carol），carol 发"g2 msg"（seq=1）。
- 步骤：alice 执行 `convs`。
- 预期结果：
  1. 返回 4 个会话，顺序：`group:g2`、`c2c:alice:carol`、`c2c:alice:bob`、`group:g1`（updatedAt 倒序；g1 用入群时间 T1）。
  2. 每项都有 `convId`、`type`、`name`、`maxSeq`、`readSeq`、`minSeq`、`unread`、`lastMsg`、`updatedAt`。
  3. 具体值：
     - g2：type=group、name="热闹群"、maxSeq=1、unread=1、lastMsg={摘要"g2 msg"、发送者 carol、时间 = 该消息 ts}、updatedAt = 该消息 ts。
     - c2c:alice:carol：type=c2c、name="carol"（或 carol 的昵称）、maxSeq=1、readSeq=0、unread=1。
     - c2c:alice:bob：name="bob"、maxSeq=1、readSeq=1、unread=0（发言即已读）。
     - g1：maxSeq=0、minSeq=1、unread=0、lastMsg 为空（具体表示见"疑问"Q2）、updatedAt=入群时间。

### TC-CONV-01-02 会话入选规则：自聊在列表中，发送失败（4042）的单聊不在列表中
- 关联 AC：AC-CONV-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 无任何会话。
- 步骤：alice 给自己发"备忘"；alice 给 ghost 发消息（返回 4042）；alice 执行 convs。
- 预期结果：只返回 1 个会话 `c2c:alice:alice`，unread=0；没有 `c2c:alice:ghost`。

### TC-CONV-01-03 接收方视角：单聊 name 为对方，lastMsg 为对方发的消息
- 关联 AC：AC-CONV-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发 2 条，最后一条"第二条"。
- 步骤：bob 执行 convs。
- 预期结果：返回 `c2c:alice:bob`，name="alice"（或昵称），maxSeq=2，readSeq=0，unread=2，lastMsg 摘要"第二条"、发送者 alice。

### TC-CONV-01-04 会话数 500 时全部返回
- 关联 AC：AC-CONV-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：导入 p001～p500，每人给 alice 发 1 条（多发送者，不触发单用户限流）。
- 步骤：alice 执行 convs。
- 预期结果：返回 500 个会话，按 updatedAt 倒序（第 1 个是最后发消息的人）。（第 501 个会话时的行为见"疑问"Q1。）

### TC-CONV-01-05 收到新消息的会话移到顶部，摘要和时间更新
- 关联 AC：AC-CONV-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在会话列表页；列表顺序为 `group:g2`、`c2c:alice:carol`、`c2c:alice:bob`；其中 bob 的会话摘要为"旧消息"、时间为昨天。
- 步骤：bob 给 alice 发"新消息"。
- 预期结果：1 秒内 `c2c:alice:bob` 移到第 1 位；摘要变为"新消息"；时间显示为该消息服务端时间的 `HH:mm`；未读 +1；其他两个会话顺序不变。

### TC-CONV-01-06 新浏览器上下文登录，会话列表与原设备一致
- 关联 AC：AC-CONV-01-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：数据同 TC-CONV-01-01；上下文 A 中 alice 已登录。
- 步骤：新建浏览器上下文 B，alice 登录；比较 A、B 两个列表。
- 预期结果：B 中会话数（4）、顺序、每个会话的名称、摘要、时间、未读数与 A 完全一致。

### TC-CONV-01-07 updatedAt 相同时按 convId 升序（ASCII）
- 关联 AC：AC-CONV-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有 3 个会话 `group:gB`、`group:ga`、`c2c:alice:bob`，通过测试库（每个用例独立数据库）把三者的 updatedAt 设为同一毫秒值（见"疑问"Q3）。
- 步骤：alice 执行 convs。
- 预期结果：顺序为 `c2c:alice:bob`、`group:gB`、`group:ga`（按 ASCII 字节序：'c' < 'g'；'B'=0x42 < 'a'=0x61）。

### TC-CONV-01-08 客户端时钟快 1 小时，排序和显示的时间不受影响
- 关联 AC：AC-CONV-01-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 上下文用 `page.clock` 把浏览器时间设为服务端时间 + 1 小时；alice 有会话 bob（最后消息 10:00，服务端时间）、carol（最后消息 10:05）。
- 步骤：
  1. 打开会话列表，记录顺序和时间。
  2. alice 给 bob 发一条消息，等待 ack。
  3. 进入与 bob 的聊天页查看该消息时间。
- 预期结果：
  1. 初始顺序 carol、bob，时间分别显示"10:05""10:00"（不是 11:05 / 11:00）。
  2. 发送后 bob 会话移到顶部，时间显示为 ack 的服务端时间（如 10:10），而不是客户端的 11:10。
  3. 聊天页中该消息 ack 后显示服务端时间；"发送中"期间允许临时显示客户端时间。

### TC-CONV-01-09 没有会话时显示空状态并引导"发起单聊"
- 关联 AC：AC-CONV-01-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：新用户 newbie，没有任何会话。
- 步骤：newbie 登录；点击空状态中的引导按钮。
- 预期结果：显示空状态和"发起单聊"引导；点击后弹出发起单聊弹窗。

## F-CONV-02 未读数与标记已读

### TC-CONV-02-01 bob 离线时收到 3 条，登录后未读 3
- 关联 AC：AC-CONV-02-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 未登录。
- 步骤：alice 给 bob 发 3 条；bob 登录。
- 预期结果：会话列表中与 alice 的会话未读角标为"3"。

### TC-CONV-02-02 打开会话后未读变 0，重新登录后仍为 0
- 关联 AC：AC-CONV-02-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：接 TC-CONV-02-01。
- 步骤：
  1. bob 打开与 alice 的会话，等待 1.5 秒，返回列表。
  2. bob 退出登录再登录（或新上下文登录）。
- 预期结果：
  1. bob 发出 `read {convId:"c2c:alice:bob", seq:3}`；返回列表后未读角标消失。
  2. 重新登录后未读仍为 0（服务端 readSeq=3）。

### TC-CONV-02-03 已读上报节流：1 秒内最多 1 次，值为最新 seq
- 关联 AC：AC-CONV-02-2
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟；当前会话可见并停留在该会话；本地 maxSeq=0。
- 步骤：100ms 内依次推送 seq 1～10；假时钟前进 2 秒。
- 预期结果：
  1. 任意 1 秒窗口内 `read` 帧不超过 1 个。
  2. 最后一个 `read` 的 seq=10；总计不超过 2 个 `read` 帧。

### TC-CONV-02-04 发言即已读：bob 发 1 条后未读为 0
- 关联 AC：AC-CONV-02-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发 3 条（seq 1～3），bob 未上报已读，convs 中 unread=3。
- 步骤：bob 发 1 条（seq=4），不调用 read；bob 执行 convs。
- 预期结果：readSeq=4，unread=0。

### TC-CONV-02-05 未读 99 显示"99"，100 显示"99+"
- 关联 AC：AC-CONV-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 离线；alice 给 bob 发 99 条（每秒不超过 20 条）；carol 给 bob 发 100 条；dave 给 bob 发 1 条。
- 步骤：bob 登录，查看会话列表。
- 预期结果：alice 会话显示"99"；carol 会话显示"99+"；dave 会话显示"1"；无未读的会话不显示角标。

### TC-CONV-02-06 已读只进不退：先报 5 再报 3，仍为 5
- 关联 AC：AC-CONV-02-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 10 条 alice 发的消息。
- 步骤：bob 上报 read seq=5；再上报 seq=3；bob 执行 convs。
- 预期结果：两次 read 都返回成功 ack（不报错）；convs 中 readSeq=5、unread=5。

### TC-CONV-02-07 对无权限会话上报已读返回 4032 / 4033 / 4043
- 关联 AC：AC-CONV-02-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有消息；g1 成员 alice、bob；g404 不存在。
- 步骤：carol 分别上报 read：`c2c:alice:bob` seq=1、`group:g1` seq=1、`group:g404` seq=1。
- 预期结果：依次返回 4032、4033、4043（PRD 只对 sync 规定了这组错误码，read 按同样规则预期，见"疑问"Q4）；alice、bob 的 readSeq 不受影响。

### TC-CONV-02-08 上报 seq 超过 maxSeq 时截断为 maxSeq，未读不为负
- 关联 AC：AC-CONV-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` maxSeq=10，bob readSeq=0。
- 步骤：
  1. bob 上报 read seq=999；bob 执行 convs。
  2. alice 再发 1 条（seq=11）；bob 执行 convs。
- 预期结果：
  1. readSeq=10、unread=0。
  2. readSeq=10、unread=1（证明存的是 10 而不是 999，新消息不会被"预先已读"）。

### TC-CONV-02-09 群成员未读按 minSeq 计算，已读低于 minSeq 不影响
- 关联 AC：AC-CONV-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（historyVisible=false）已有 10 条；dave 入群（minSeq=11）；之后群里再发 5 条（maxSeq=15）。
- 步骤：dave 执行 convs；dave 上报 read seq=3；再执行 convs；上报 read seq=13；再执行 convs。
- 预期结果：
  1. 第 1 次 unread=5（15 − max(0, 10)），不是 15。
  2. 上报 seq=3 后 unread 仍为 5。
  3. 上报 seq=13 后 unread=2。

### TC-CONV-02-10 标签页在后台时不上报已读，切回后上报并清零
- 关联 AC：AC-CONV-02-7
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 打开与 alice 的聊天页，未读 0；通过脚本把 `document.visibilityState` 模拟为 `hidden` 并派发 `visibilitychange`（或在同一上下文打开另一个标签页置前）。
- 步骤：
  1. alice 发 2 条；等待 2 秒；用 bob 的测试 WS 连接执行 convs。
  2. 把 bob 页面切回 `visible`；等待 1.5 秒；再执行 convs。
- 预期结果：
  1. 后台期间 bob 页面不发出 `read`；convs 中 unread=2（标题 / 角标为 P1，不作要求）。
  2. 切回后发出 `read seq=` 最新 seq；convs 中 unread=0；页面上该会话未读清零。

## F-CONV-03 发起单聊

### TC-CONV-03-01 输入合法 userId 进入聊天页
- 关联 AC：AC-CONV-03-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在会话列表页。
- 步骤：
  1. 点击"发起单聊"，输入 `bob`，确认。
  2. 返回，再次发起，输入 `Bob`（与 `alice` 比较时 'B' < 'a'）。
  3. 输入 64 个 `a`。
- 预期结果：
  1. 跳转到 `/chat/c2c:alice:bob`，顶栏显示 bob。
  2. 跳转到 `/chat/c2c:Bob:alice`。
  3. 校验通过并跳转（是否存在由发送时服务端判断）。

### TC-CONV-03-02 未发消息的会话不进入服务端会话列表
- 关联 AC：AC-CONV-03-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 与 dave 没有任何消息。
- 步骤：
  1. alice 发起与 dave 的单聊，进入聊天页后不发消息，返回列表。
  2. 用 alice 的测试 WS 连接执行 convs；alice 刷新页面。
  3. 再次进入与 dave 的聊天页发送"你好"；执行 convs。
- 预期结果：
  1. convs 中没有 `c2c:alice:dave`；刷新后列表中也没有该会话（本地临时会话不持久化）。
  2. 发送成功后 convs 中出现 `c2c:alice:dave`。

### TC-CONV-03-03 输入格式不合法提示"账号格式不正确"，不跳转
- 关联 AC：AC-CONV-03-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 打开发起单聊弹窗。
- 步骤：依次输入 `bad id`、`张三`、`a.b`、`a@b`、65 个 `a`、空字符串，点击确认。
- 预期结果：每次都提示"账号格式不正确"，URL 不变，不发出任何 WS 请求。

### TC-CONV-03-04 给不存在的用户发第一条消息提示"用户不存在"，会话不进入服务端列表
- 关联 AC：AC-CONV-03-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录；ghost 不存在。
- 步骤：
  1. 发起与 `ghost` 的单聊，发送"在吗"。
  2. 用测试 WS 连接执行 alice 的 convs；刷新页面。
- 预期结果：
  1. 消息标记为失败，提示"用户不存在"。
  2. convs 中没有 `c2c:alice:ghost`；刷新后列表中没有该会话。

---

## 覆盖检查

| AC | 覆盖用例 |
|---|---|
| AC-CONV-01-1 | TC-CONV-01-01、TC-CONV-01-02、TC-CONV-01-03、TC-CONV-01-04 |
| AC-CONV-01-2 | TC-CONV-01-05 |
| AC-CONV-01-3 | TC-CONV-01-06 |
| AC-CONV-01-4 | TC-CONV-01-07 |
| AC-CONV-01-5 | TC-CONV-01-08 |
| AC-CONV-01-6 | TC-CONV-01-09 |
| AC-CONV-02-1 | TC-CONV-02-01 |
| AC-CONV-02-2 | TC-CONV-02-02、TC-CONV-02-03 |
| AC-CONV-02-3 | TC-CONV-02-04 |
| AC-CONV-02-4 | TC-CONV-02-05 |
| AC-CONV-02-5 | TC-CONV-02-06、TC-CONV-02-07 |
| AC-CONV-02-6 | TC-CONV-02-08、TC-CONV-02-09 |
| AC-CONV-02-7 | TC-CONV-02-10 |
| AC-CONV-03-1 | TC-CONV-03-01、TC-CONV-03-02 |
| AC-CONV-03-2 | TC-CONV-03-03 |
| AC-CONV-03-3 | TC-CONV-03-04 |

共 16 条 AC，全部覆盖。

## 疑问

- Q1（超过 500 个会话）：`convs` 上限 500，第 501 个会话如何处理？截断返回最新的 500 个，还是报错？客户端是否需要提示？
- Q2（无消息群的 lastMsg）：没有消息的群，`lastMsg` 是 `null`、缺省还是空对象？列表摘要显示什么？
- Q3（相同 updatedAt 不可测）：§6.1 没有提供"可控服务端时钟"，黑盒很难让两个会话的 updatedAt 落在同一毫秒。TC-CONV-01-07 依赖直接改测试库，建议在 §6.1 增加服务端假时钟或测试钩子。
- Q4（read 的错误码）：F-CONV-02 没有规定对非参与单聊、非成员群、不存在的群上报 `read` 返回什么；用例按 sync 的 4032 / 4033 / 4043 预期。`read` 的 seq=0 或负数是否返回 4000 也未规定。
- Q5（lastMsg 摘要截断）：5000 字消息的摘要是否由服务端截断？截断长度多少？
- Q6（未读显示规则补充）：未读为 0 时是否隐藏角标？用例按"隐藏"预期。
