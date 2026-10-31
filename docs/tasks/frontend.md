# 前端任务单（一期：TS SDK + H5）

| 项 | 内容 |
|---|---|
| 依据 | [PRD v1.0](../prd/PRD-core-v1.0.md)、[错误码](../../protocol/ERRORS.md)、[协议](../../protocol/README.md) |
| 负责 | 前端（AI） |
| 技术栈 | TypeScript；SDK 不依赖任何框架（`sdk-ts/`）；H5 用 React + Vite（`web/`）；测试：Vitest（SDK）、Playwright（UI） |
| 开工条件 | 后端 BE-001（协议定稿）完成 |
| 完成定义 | 任务内列出的 AC 都有通过的测试（SDK 单测或 Playwright）；手工 AC 有验证记录；`pnpm test` 和 `pnpm e2e` 全部通过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由脚本从这一行生成。

## 总览

| 阶段 | 任务 | 说明 |
|---|---|---|
| F0 工程 | FE-001 | 工程脚手架、测试框架 |
| F1 SDK | FE-002 ～ FE-008 | 传输层、错误码、连接管理、同步引擎、发送队列、已读、测试替身 |
| F2 页面 | FE-010 ～ FE-018 | 登录、会话列表、发起单聊、聊天页、文本展示、群聊界面、文案、离开确认、演示面板 |
| F3 P1 | FE-019 ～ FE-020 | 昵称头像、已读多端同步 |

SDK 先行（F1 可以用假传输层独立开发和测试），页面依赖 SDK 的接口。

---

## F0 工程

### FE-001 工程脚手架
- 优先级：P0
- 依赖：无
- AC：（工程任务）
- 内容：pnpm workspace：`sdk-ts/`（库，Vitest）、`web/`（React + Vite，依赖 sdk-ts）；ESLint + Prettier；Playwright 配置两个浏览器上下文（模拟两个用户 / 两个端）；在 CI 脚本里启动测试模式的 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`）。

---

## F1 SDK（`sdk-ts/`）

对外接口按 PLAN 3.2 节的 `IMClient`：`connect / disconnect / state / send / loadHistory / markRead / conversations / messages(convId) / events`。

### FE-002 传输层、帧编解码与错误码
- 优先级：P0
- 依赖：FE-001、BE-001
- AC：AC-SYS-01-3、AC-SYS-01-4
- 内容：
  1. 帧类型从 Schema 生成或手写，用协议的 `vectors/frames.json` 跑编解码测试。
  2. 请求 / 应答匹配：按 `id` 关联 ack，带超时。
  3. 错误枚举由 `errors.json` 生成，并加一致性测试；不认识的错误码按区段兜底（4xx 不重试、5xx 可重试）。

### FE-003 连接管理：心跳、重连、被踢
- 优先级：P0
- 依赖：FE-002
- AC：AC-CONN-01-3、AC-CONN-01-5、AC-CONN-01-6、AC-AUTH-02-4、AC-AUTH-03-5、AC-AUTH-05-1
- 内容：
  1. 状态机：断开 / 连接中 / 已连接。
  2. 每 30 秒发 ping，10 秒收不到 pong 判定断线。
  3. 重连按 1s 起指数退避，最长 30s，加 ±20% 抖动；浏览器 `online` 事件触发立即重连。
  4. 收到 `kicked`，或关闭码为 1000、4009～4012 时不重连，向上层抛出原因。
  5. `disconnect()` 以 1000 关闭，并清空状态。
  6. 所有时间参数都可以配置（便于测试）。

### FE-004 同步引擎：会话列表、补拉、缺口、排序、合并
- 优先级：P0
- 依赖：FE-002、FE-003
- AC：AC-CONN-01-4、AC-CONN-01-7、AC-CONN-01-8、AC-MSG-02-4、AC-MSG-04-3、AC-MSG-04-4、AC-MSG-04-5、AC-MSG-05-1、AC-MSG-05-2、AC-CONV-01-3、AC-GRP-02-1、AC-SYNC-01-1、AC-SYNC-01-2、AC-MSG-02-1
- 内容：
  1. 连接（含重连）成功后：拉 `convs` → 对"服务端 maxSeq > 本地 maxSeq"的会话补拉；缺口超过 200 条时，只加载最新一页（`beforeSeq=maxSeq+1`）。
  2. 每个会话按 seq 有序存储；本地 maxSeq 同时计入 ack 带回的 seq。
  3. 缺口检测：出现缺口后等 500ms 仍未补上，就发 `sync`。
  4. 去重：按 `msgId`；用 `clientMsgId` 合并本地待发消息和服务端回来的消息。
  5. 处理 `convAdded` / `convRemoved` 事件。
  6. 打开会话时 `loadHistory`：首次用 `beforeSeq=maxSeq+1, limit=20`，之后继续向前翻页，`hasMore=false` 时停止。

### FE-005 发送队列与发送状态机
- 优先级：P0
- 依赖：FE-003、FE-004
- AC：AC-MSG-03-1、AC-MSG-03-2、AC-MSG-03-3、AC-MSG-03-4、AC-AUTH-03-6、AC-MSG-01-9、AC-MSG-07-2、AC-CONV-03-3
- 内容：按 PRD F-MSG-03 的状态机实现：发送中 / 已发送 / 发送失败；在线时 10 秒超时；离线时先排队，重连后自动发 1 次；手动重发沿用原 `clientMsgId`；被踢时把发送中的消息全部标记为失败；把错误码 4042 / 4290 / 4033 等透传给界面。

### FE-006 已读上报与未读数
- 优先级：P0
- 依赖：FE-004
- AC：AC-CONV-02-1、AC-CONV-02-2、AC-CONV-02-7
- 内容：只有页面可见（`visibilityState==='visible'`）并且停在该会话时才上报，节流为 1 秒一次；收到 ack 时推进本地 readSeq（发言即已读）；本地未读数计算公式与服务端一致。

### FE-007 测试替身：假传输层与假时钟
- 优先级：P0
- 依赖：FE-002
- AC：AC-CONN-02-3、AC-CONN-01-6
- 内容：`FakeTransport`：可以脚本化下发帧、丢弃 / 延迟 pong、模拟断开；`FakeClock`：控制心跳、超时、退避和节流的时间。SDK 构造函数支持注入这两个替身。

### FE-008 文本校验工具
- 优先级：P0
- 依赖：FE-001
- AC：AC-MSG-01-2、AC-MSG-01-3
- 内容：`trimText`（首尾空白含 `\n`、`\t`、U+3000）和 `countCodePoints`，与服务端规则一致；加边界测试：1、5000、5001 个字符，5000 个 emoji，ZWJ 组合 emoji。

---

## F2 页面（`web/`）

### FE-010 登录页与会话持久化
- 优先级：P0
- 依赖：FE-003、BE-014
- AC：AC-AUTH-04-1、AC-AUTH-04-2、AC-AUTH-04-4、AC-AUTH-05-1
- 内容：输入用户名 → 调用 `/demo/login` → token 存入 `sessionStorage` → 连接；前端校验用户名格式（"账号格式不正确"）；刷新后用 `sessionStorage` 中的 token 自动重连，停留在原页面；右上角菜单提供"退出登录"；验证构建产物和 HAR 抓包中不含管理密钥（写成 CI 检查脚本）。

### FE-011 会话列表页与连接状态条
- 优先级：P0
- 依赖：FE-004、FE-006
- AC：AC-CONV-01-2、AC-CONV-01-3、AC-CONV-01-5、AC-CONV-01-6、AC-CONV-02-4、AC-CONN-02-1、AC-CONN-02-2、AC-CONN-02-3、AC-MSG-02-3
- 内容：按服务端 `updatedAt` 排序；收到新消息时会话移到顶部；未读角标（超过 99 显示"99+"）；空状态引导"发起单聊"；顶部连接状态条（断开持续超过 1 秒才显示）。

### FE-012 发起单聊弹窗
- 优先级：P0
- 依赖：FE-011
- AC：AC-CONV-03-1、AC-CONV-03-2、AC-CONV-03-3
- 内容：输入对方 `userId` 并校验格式；临时会话只存在本地，发出第一条成功后才进入列表；收到 4042 时提示"用户不存在"。

### FE-013 聊天页：消息列表、发送、历史、滚动
- 优先级：P0
- 依赖：FE-004、FE-005、FE-006
- AC：AC-MSG-01-1、AC-MSG-01-2、AC-MSG-01-3、AC-MSG-02-2、AC-MSG-05-2、AC-MSG-05-6、AC-MSG-06-3、AC-MSG-06-4
- 内容：
  1. 消息气泡区分自己和对方，显示发送状态，失败消息可点击重发。
  2. 上拉加载历史：加载中 / 没有更多了 / 加载失败点击重试；加载后可视位置不跳动。
  3. 时间分隔：间隔超过 5 分钟才显示，格式见 PRD F-MSG-06。
  4. 滚动规则：距底部不超过 100px 时自动滚动；否则显示"↓ N 条新消息"。
  5. 输入框按码点计数；为空时发送按钮不可点；超过 5000 时提示"消息过长"。

### FE-014 文本渲染与输入行为
- 优先级：P0
- 依赖：FE-013
- AC：AC-MSG-06-1、AC-MSG-06-2、AC-MSG-06-5
- 内容：只用 React 文本节点渲染，禁止 `dangerouslySetInnerHTML`（加 lint 规则）；`white-space: pre-wrap` 加 `overflow-wrap: anywhere`；PC 上 Enter 发送、Shift+Enter 换行；在 iOS Safari 上手工验证键盘弹起时的表现（记录到测试报告）。

### FE-015 群聊界面
- 优先级：P0
- 依赖：FE-013
- AC：AC-GRP-02-4、AC-GRP-02-5、AC-GRP-03-3
- 内容：群消息显示发送者；顶栏显示群名和成员数；收到 4033 或 `convRemoved` 时提示"你已不在该群"并禁用输入框；收到 `convAdded` 时会话列表出现该群。

### FE-016 错误文案映射
- 优先级：P0
- 依赖：FE-002
- AC：AC-SYS-01-4、AC-MSG-01-9、AC-MSG-07-2
- 内容：按 PRD 第 5 节的文案表，从错误码映射到提示文案（集中在一个文件里维护）；不认识的错误码按区段兜底；不直接展示服务端返回的 `msg`。

### FE-017 离开页面确认
- 优先级：P0
- 依赖：FE-005
- AC：AC-MSG-03-5
- 内容：存在发送中或发送失败的消息时，注册 `beforeunload` 提示。

### FE-018 演示面板
- 优先级：P0
- 依赖：FE-003、FE-004
- AC：AC-DEMO-01-1、AC-DEMO-01-2
- 内容：URL 带 `?demo=1` 时显示：连接状态、各会话本地 maxSeq、"模拟断网"开关（调用 `disconnect` 并暂停自动重连；关闭开关时恢复连接）。

---

## F3 P1

### FE-019 昵称与头像
- 优先级：P1
- 依赖：BE-043、FE-011、FE-013
- AC：AC-USER-02-1
- 内容：优先显示昵称，没有昵称时显示 `userId`；头像为空时显示首字母占位。

### FE-020 已读多端同步
- 优先级：P1
- 依赖：BE-042、FE-006
- AC：AC-SYNC-02-1
- 内容：处理 `readUpdated` 事件，更新本地 readSeq 和未读数。
