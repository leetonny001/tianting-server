# 测试用例：多端同步（SYNC）

> 依据：[PRD-core v1.3](../prd/PRD-core-v1.3.md) F-SYNC-01、F-SYNC-02（含 `event/readUpdated`，C-06）及 §9 补充规则（C-34、C-44），[protocol/README.md](../../protocol/README.md) 交互规则 8。
>
> 通用约定：
> - 每个用例使用独立数据库，服务端测试模式。
> - 用户 `alice`、`bob`、`carol` 已导入。
> - "web 端 / pc 端"指同一账号 alice 分别以 `platform=web`、`platform=pc` 建立的连接。UI 自动化中用两个浏览器上下文分别登录 alice：web 端用默认 URL，pc 端在 URL 上加 `?platform=pc`（C-44）。

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
| TC-SYNC-01-09 | SDK 收到 from 为自己的消息：不增加未读，推进本地 readSeq | AC-SYNC-01-2 | SDK 单测 | P0 |
| TC-SYNC-01-10 | H5 用 ?platform=pc 以 pc 身份登录，默认为 web | AC-SYNC-01-1 | UI 自动化 | P0 |
| TC-SYNC-02-01 | pc 打开 3 条未读的会话，web 端 2 秒内未读变 0（界面） | AC-SYNC-02-1 | UI 自动化 | P1 |
| TC-SYNC-02-02 | pc 上报已读后，web 连接 2 秒内获得新的已读位置 | AC-SYNC-02-1 | 后端 e2e | P1 |
| TC-SYNC-02-03 | pc 端发言即已读，web 端未读同步清零 | AC-SYNC-02-1 | UI 自动化 | P1 |
| TC-SYNC-02-04 | 两端上报不同已读位置，以较大者为准 | AC-SYNC-02-1 | 后端 e2e | P1 |
| TC-SYNC-02-05 | 已读未前进（重复 / 回退 / 截断后相同）时不推送 readUpdated | AC-SYNC-02-1 | 后端 e2e | P1 |
| TC-SYNC-02-06 | 发言即已读推送 readUpdated 给自己的其他连接 | AC-SYNC-02-1 | 后端 e2e | P1 |

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

### TC-SYNC-01-09 SDK 收到 from 为自己的消息：不增加未读，推进本地 readSeq
- 关联 AC：AC-SYNC-01-2
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 以 alice 登录，假传输层；会话 `c2c:alice:bob` 本地 maxSeq=3、readSeq=0、未读 3；当前未打开该会话。
- 步骤：
  1. 假传输层推送 `msg {from:"alice", seq:4}`（alice 在另一端发的）。
  2. 假传输层推送 `msg {from:"bob", seq:5}`。
- 预期结果：
  1. 第 1 步后本地 readSeq=4、未读 0（C-34），不发出 `read` 帧。
  2. 第 2 步后未读 1。

### TC-SYNC-01-10 H5 用 ?platform=pc 以 pc 身份登录，默认为 web
- 关联 AC：AC-SYNC-01-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：两个浏览器上下文。
- 步骤：上下文 A1 打开登录页（无参数）登录 alice；A2 打开 `/?platform=pc` 登录 alice；检查两者的 WS 连接 URL 和服务端连接日志。
- 预期结果：A1 的连接 `platform=web`，A2 的连接 `platform=pc`（C-44）；两个连接都保持在线。

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
  1. alice(web) 在 2 秒内收到 `event {type:"readUpdated", convId:"c2c:alice:bob", readSeq:3}`（C-06）。
  2. alice(pc)（上报的那个连接）不收到该事件，只收到 read 的 ack；bob 也不收到。
  3. alice(web) 执行 convs：unread=0。

### TC-SYNC-02-03 pc 端发言即已读，web 端未读同步清零
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：A1（web）在会话列表页，与 bob 的会话未读 3；A2（pc）通过测试 WS 连接直接给 bob 发 1 条（不打开会话、不上报 read）。
- 步骤：A2 发送消息，记录时间 T0。
- 预期结果：A1 收到该消息的 `msg` 和 `readUpdated {readSeq:4}`；与 bob 的会话在 T0+2s 内未读变为 0，摘要变为刚发的消息（发言即已读同样推送 readUpdated，C-06）。

### TC-SYNC-02-04 两端上报不同已读位置，以较大者为准
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：会话 maxSeq=10，alice readSeq=0；alice(web)、alice(pc) 在线。
- 步骤：alice(pc) 上报 read seq=8；随后 alice(web) 上报 read seq=5。
- 预期结果：服务端 readSeq=8；alice(web) 收到 `readUpdated {readSeq:8}`；alice(web) 上报 5 后已读位置没有前进，alice(pc) 不收到 readUpdated；两端最终的未读都是 2，web 不会因为自己上报 5 而把本地未读显示为 5。

### TC-SYNC-02-05 已读未前进（重复 / 回退 / 截断后相同）时不推送 readUpdated
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：会话 maxSeq=10，alice readSeq=10；alice(web)、alice(pc) 在线。
- 步骤：alice(pc) 依次上报 read seq=10、seq=7、seq=999；每次后观察 alice(web) 1 秒。
- 预期结果：三次 read 都成功；alice(web) 没有收到任何 `readUpdated`（已读位置都没有前进，999 截断为 10）。

### TC-SYNC-02-06 发言即已读推送 readUpdated 给自己的其他连接
- 关联 AC：AC-SYNC-02-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 3 条 bob 发的消息，alice readSeq=0；alice(web)、alice(pc)、bob 在线。
- 步骤：alice(pc) 给 bob 发 1 条（seq=4）。
- 预期结果：alice(web) 收到 seq=4 的 `msg` 和 `readUpdated {convId:"c2c:alice:bob", readSeq:4}`（C-06）；alice(pc) 只收到 ack；bob 不收到 readUpdated。

---

## 覆盖检查

| AC | 覆盖用例 |
|---|---|
| AC-SYNC-01-1 | TC-SYNC-01-01、TC-SYNC-01-02、TC-SYNC-01-03、TC-SYNC-01-04、TC-SYNC-01-10 |
| AC-SYNC-01-2 | TC-SYNC-01-05、TC-SYNC-01-06、TC-SYNC-01-07、TC-SYNC-01-08、TC-SYNC-01-09 |
| AC-SYNC-02-1 | TC-SYNC-02-01、TC-SYNC-02-02、TC-SYNC-02-03、TC-SYNC-02-04、TC-SYNC-02-05、TC-SYNC-02-06 |

共 3 条 AC，全部覆盖。

## 已澄清

| 原问题 | 结论 | 已更新 / 新增用例 |
|---|---|---|
| Q1 H5 的 pc 端 | C-44：URL 参数 `?platform=pc` 以 pc 身份登录（仅用于测试多端），默认 web | 更新通用约定；新增 TC-SYNC-01-10 |
| Q2 已读同步机制 | C-06：已读位置前进时向该用户其他在线连接推送 `event/readUpdated {convId, readSeq}`（PRD v1.1 F-SYNC-02 规则、§8） | 更新 TC-SYNC-02-02、TC-SYNC-02-04；新增 TC-SYNC-02-05 |
| Q3 发言即已读是否多端同步 | C-06：包括发言即已读 | 更新 TC-SYNC-02-03；新增 TC-SYNC-02-06 |
| （v1.1 新增）另一端自己发的消息算不算未读 | C-34：不算，客户端收到 `from` 为自己的消息时不增加未读并推进本地 readSeq | 新增 TC-SYNC-01-09；TC-SYNC-01-07 按此判定 |

## 疑问

无。
