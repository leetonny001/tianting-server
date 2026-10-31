# 测试用例：多端同步（SYNC）

> 依据：[PRD v1.0](../prd/PRD-core-v1.0.md) F-SYNC-01、F-SYNC-02，[protocol/README.md](../../protocol/README.md) 交互规则 8。
>
> 通用约定：
> - 每个用例使用独立数据库，服务端测试模式。
> - 用户 `alice`、`bob`、`carol` 已导入。
> - "web 端 / pc 端"指同一账号 alice 分别以 `platform=web`、`platform=pc` 建立的连接。UI 自动化中用两个浏览器上下文分别登录 alice（pc 端如何在 H5 上指定 platform 见"疑问"Q1）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-SYNC-01-01 | bob 给 alice 发消息，alice 的 web、pc 连接都收到 | AC-SYNC-01-1 | 后端 e2e | P0 |
| TC-SYNC-01-02 | bob 给 alice 发消息，alice 两个页面都显示（界面） | AC-SYNC-01-1 | UI 自动化 | P0 |
| TC-SYNC-01-03 | 同一平台两个连接（web×2）都收到 | AC-SYNC-01-1 | 后端 e2e | P0 |
| TC-SYNC-01-04 | 群消息：alice 的三个连接都收到 | AC-SYNC-01-1 | 后端 e2e | P0 |
| TC-SYNC-01-05 | alice 在 web 发消息：pc 收到 msg，web 只收到 ack | AC-SYNC-01-2 | 后端 e2e | P0 |
| TC-SYNC-01-06 | alice 在 web 发消息：pc 显示为自己发的，web 不重复显示（界面） | AC-SYNC-01-2 | UI 自动化 | P0 |
| TC-SYNC-01-07 | 自己在另一端发的消息不增加本端未读 | AC-SYNC-01-2 | UI 自动化 | P0 |
| TC-SYNC-01-08 | pc 端离线期间 web 端发的消息，pc 重连后补齐且显示为自己发的 | AC-SYNC-01-2 | UI 自动化 | P0 |
| TC-SYNC-02-01 | pc 打开 3 条未读的会话，web 端 2 秒内未读变 0（界面） | AC-SYNC-02-1 | UI 自动化 | P1 |
| TC-SYNC-02-02 | pc 上报已读后，web 连接 2 秒内获得新的已读位置 | AC-SYNC-02-1 | 后端 e2e | P1 |
| TC-SYNC-02-03 | pc 端发言即已读，web 端未读同步清零 | AC-SYNC-02-1 | UI 自动化 | P1 |
| TC-SYNC-02-04 | 两端上报不同已读位置，以较大者为准 | AC-SYNC-02-1 | 后端 e2e | P1 |

---

## F-SYNC-01 多端消息同步

### TC-SYNC-01-01 bob 给 alice 发消息，alice 的 web、pc 连接都收到
- 关联 AC：AC-SYNC-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice(web)、alice(pc)、bob 在线。
- 步骤：bob 以 `cid-sync-0001` 给 alice 发"hi alice"。
- 预期结果：alice(web)、alice(pc) 各收到恰好 1 个 `msg`，两者 msgId、seq（=1）、convId（`c2c:alice:bob`）、from（bob）、clientMsgId 完全相同。

### TC-SYNC-01-02 bob 给 alice 发消息，alice 两个页面都显示（界面）
- 关联 AC：AC-SYNC-01-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：上下文 A1（alice，web）、A2（alice，pc）都打开与 bob 的聊天页；上下文 B 登录 bob。
- 步骤：bob 发送"两端都要收到"。
- 预期结果：A1、A2 都在 1 秒内显示该消息（对方消息），各只显示 1 条。

### TC-SYNC-01-03 同一平台两个连接（web×2）都收到
- 关联 AC：AC-SYNC-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 platform=web 建立两个连接 W1、W2；bob 在线。
- 步骤：bob 给 alice 发 1 条。
- 预期结果：W1、W2 各收到 1 个 `msg`，msgId 相同。

### TC-SYNC-01-04 群消息：alice 的三个连接都收到
- 关联 AC：AC-SYNC-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob；alice 以 web、pc、ios 在线。
- 步骤：bob 给 g1 发 1 条；alice(ios) 再给 g1 发 1 条。
- 预期结果：
  1. bob 的消息：alice 三个连接各收到 1 次。
  2. alice(ios) 的消息：alice(web)、alice(pc)、bob 各收到 1 次；alice(ios) 只收到 ack。

### TC-SYNC-01-05 alice 在 web 发消息：pc 收到 msg，web 只收到 ack
- 关联 AC：AC-SYNC-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice(web)、alice(pc)、bob 在线。
- 步骤：alice(web) 以 `cid-sync-0002` 给 bob 发"我在 web 发的"。
- 预期结果：
  1. alice(pc) 收到 1 个 `msg`：from=alice、convId=`c2c:alice:bob`、msgId=ack.msgId、clientMsgId=`cid-sync-0002`。
  2. alice(web) 只收到 ack，500ms 内没有 `msg` 帧。
  3. bob 收到 1 个 `msg`。

### TC-SYNC-01-06 alice 在 web 发消息：pc 显示为自己发的，web 不重复显示（界面）
- 关联 AC：AC-SYNC-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：A1（web）、A2（pc）都打开与 bob 的聊天页。
- 步骤：A1 发送"同步测试"。
- 预期结果：
  1. A2 在 1 秒内显示"同步测试"，位于"自己发的"一侧（右侧样式）。
  2. A1 只显示 1 条"同步测试"，状态为已发送。

### TC-SYNC-01-07 自己在另一端发的消息不增加本端未读
- 关联 AC：AC-SYNC-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：A1（web）在聊天页；A2（pc）停留在会话列表页，与 bob 的会话未读 0。
- 步骤：A1 给 bob 发 2 条。
- 预期结果：A2 列表中与 bob 的会话移到顶部，摘要更新为第 2 条，未读仍为 0（发言即已读）。

### TC-SYNC-01-08 pc 端离线期间 web 端发的消息，pc 重连后补齐且显示为自己发的
- 关联 AC：AC-SYNC-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：A1（web）、A2（pc）都在与 bob 的聊天页。
- 步骤：A2 `setOffline(true)`；A1 发 3 条；bob 发 1 条；A2 `setOffline(false)`。
- 预期结果：A2 重连后按 seq 顺序显示 4 条，其中 3 条为自己发的（右侧），1 条为 bob 发的；无重复；未读 0（A2 正在看该会话）。

## F-SYNC-02 已读状态多端同步

### TC-SYNC-02-01 pc 打开 3 条未读的会话，web 端 2 秒内未读变 0（界面）
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：alice 离线时 bob 发来 3 条；A1（web）、A2（pc）登录 alice，都在会话列表页，与 bob 的会话未读 3。
- 步骤：A2 打开与 bob 的会话，记录时间 T0。
- 预期结果：A1 列表中与 bob 的会话未读在 T0+2s 内变为 0，无需刷新。

### TC-SYNC-02-02 pc 上报已读后，web 连接 2 秒内获得新的已读位置
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 3 条 bob 发的消息；alice(web)、alice(pc) 在线。
- 步骤：alice(pc) 上报 `read seq=3`。
- 预期结果：
  1. alice(web) 在 2 秒内收到已读同步通知，内容为 convId=`c2c:alice:bob`、readSeq=3（通知格式 PRD 未定义，后端任务单拟为 `event/readUpdated {convId, readSeq}`，见"疑问"Q2）。
  2. alice(pc) 自身不收到该通知（或收到也无副作用）。
  3. alice(web) 执行 convs：unread=0。

### TC-SYNC-02-03 pc 端发言即已读，web 端未读同步清零
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：A1（web）在会话列表页，与 bob 的会话未读 3；A2（pc）通过测试 WS 连接直接给 bob 发 1 条（不打开会话、不上报 read）。
- 步骤：A2 发送消息，记录时间 T0。
- 预期结果：A1 中与 bob 的会话在 T0+2s 内未读变为 0，摘要变为刚发的消息（见"疑问"Q3）。

### TC-SYNC-02-04 两端上报不同已读位置，以较大者为准
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：会话 maxSeq=10，alice readSeq=0；alice(web)、alice(pc) 在线。
- 步骤：alice(pc) 上报 read seq=8；随后 alice(web) 上报 read seq=5。
- 预期结果：服务端 readSeq=8；alice 两端最终的未读都是 2；web 不会因为自己上报 5 而把本地未读显示为 5。

---

## 覆盖检查

| AC | 覆盖用例 |
|---|---|
| AC-SYNC-01-1 | TC-SYNC-01-01、TC-SYNC-01-02、TC-SYNC-01-03、TC-SYNC-01-04 |
| AC-SYNC-01-2 | TC-SYNC-01-05、TC-SYNC-01-06、TC-SYNC-01-07、TC-SYNC-01-08 |
| AC-SYNC-02-1 | TC-SYNC-02-01、TC-SYNC-02-02、TC-SYNC-02-03、TC-SYNC-02-04 |

共 3 条 AC，全部覆盖。

## 疑问

- Q1（H5 的 pc 端）：一期 H5 "PC 复用"，浏览器里登录时 platform 固定为 web 还是可以选 pc？若 UI 无法以 platform=pc 登录，AC-SYNC-01-1 / AC-SYNC-02-1 的界面用例只能用两个 web 连接代替，pc 平台只在后端 e2e 中验证。
- Q2（已读同步机制未定义）：F-SYNC-02 只有 AC、没有业务规则，PRD §8 协议变更清单也没有已读同步事件。后端任务单 BE-042 拟新增 `event/readUpdated {convId, readSeq}`，但这属于协议变更，需要补进 PRD 或协议文档；否则 web 端只能靠轮询 convs，无法保证 2 秒。
- Q3（发言即已读是否也要多端同步）：在一端发消息推进了 readSeq，另一端是否需要同步清零未读？AC-SYNC-02-1 只写了"打开会话"这一种触发方式；TC-SYNC-02-03 暂按"需要同步"预期。
