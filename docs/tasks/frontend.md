# 前端任务单（一期：TS SDK + H5）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-core v1.3](../prd/PRD-core-v1.3.md)、[错误码](../../protocol/ERRORS.md)、[协议](../../protocol/README.md) |
| 负责 | 前端（AI） |
| 仓库 | 多仓库（PLAN 第 4 节）：SDK 在仓库 **tianting-sdk-ts**（包名 `@tianting/sdk`，本地目录 `../tianting-sdk-ts/`），H5 在仓库 **tianting-web**（本地目录 `../tianting-web/`，依赖 `@tianting/sdk`）；两个仓库的 `protocol/` 是本仓库 `protocol/` 的副本，用各自的 `scripts/sync-protocol.sh` 同步，不在副本里改 |
| 技术栈 | TypeScript；SDK 不依赖任何框架（tianting-sdk-ts）；H5 用 React + Vite（tianting-web）；测试：Vitest（SDK）、Playwright（UI） |
| 开工条件 | 后端 BE-001（协议定稿）完成 |
| 完成定义 | 任务内列出的 AC 都有通过的测试（SDK 单测或 Playwright）；手工 AC 有验证记录；各仓库的 `npm test`（tianting-sdk-ts、tianting-web）和 `npm run e2e`（tianting-web）全部通过；tianting-sdk-ts 的 CI 为该仓库的 `.github/workflows/ci.yml` |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由脚本从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| FE-001 工程脚手架 | 🟡 SDK 部分完成 | tianting-sdk-ts 仓库（npm，FE-IMP-1）、TypeScript 6、Vitest、ESLint、Prettier、CI（tianting-sdk-ts 的 `.github/workflows/ci.yml`）；tianting-web 和 Playwright 推迟到 Q13 页面开发时（FE-IMP-2） |
| FE-002 传输层、帧编解码、错误码 | ✅ 完成 | `errors.gen.ts` 由 `errors.json` 生成并校验；区段兜底（C-09）；标准 WebSocket 不限制下行帧大小 |
| FE-003 连接管理 | ✅ 完成 | 首帧 auth、心跳与 pong 超时、退避 + 抖动、online 立即重连、按关闭码 / kicked 决定是否重连、退出 1000、日志不含 token；连接提示 1 秒延迟（`banner` 事件） |
| FE-004 同步引擎 | ✅ 完成 | convs → 补拉（起点 max(local, minSeq−1)、超过 200 只取最新 20）、500ms 缺口检测、msgId 去重、clientMsgId 合并、历史翻页、会话事件、临时会话刷新（FE-IMP-4） |
| FE-005 发送队列 | ✅ 完成 | 状态机、10 秒超时、离线排队自动补发一次、手动重发原 clientMsgId、晚到 ack（C-30）、被踢全部失败、错误码透传 |
| FE-006 已读与未读 | ✅ 完成 | 可见 + 停在会话才报、1 秒节流、本地未读与服务端公式一致（已读基线） |
| FE-007 测试替身 | ✅ 完成 | `FakeTransport`、`FakeClock`，从 `@tianting/sdk/testing` 导出 |
| FE-008 文本校验 | ✅ 完成 | 与服务端共用 `protocol/vectors/text.json`（37 条） |
| FE-010～FE-018 H5 基础 | 🟡 主体完成 | tianting-web `3859f45`：登录 + sessionStorage + 刷新重连 + 退出确认、会话列表与连接状态条、发起单聊、聊天页（历史、滚动、时间分隔、发送状态 / 重发、超长）、纯文本渲染（lint 禁止 innerHTML）、群聊发送者 / 被移出、错误文案（命令 + 码）、离开页面提示、演示面板。缺：发送者昵称（FE-103）、群成员数（FE-204）、管理密钥 HAR 检查脚本、iOS Safari 手工验证 |

测试：单元 82 个（假传输层 + 假时钟）、集成 6 个（真实 im-server）；变异测试验证退避上限、缺口等待。设计说明见 [docs/sdk/README.md](../sdk/README.md)。

## 实现约定（前端）

| # | 问题 | 约定 |
|---|---|---|
| FE-IMP-1 | 任务单原来写的是 pnpm workspace | 本机没有 pnpm / corepack，全局安装属于系统级改动（需授权），改用 Node 自带的 npm；拆成多仓库后 SDK 与 H5 各自一个 npm 项目，不再使用 workspace |
| FE-IMP-2 | FE-001 包含 tianting-web 和 Playwright 双上下文 | 推迟到 Q13（H5 页面）时一起搭：现在没有页面，Playwright 还要下载浏览器 |
| FE-IMP-3 | 首次登录是否为每个会话补拉 | 不补拉；只补拉本地已经加载过消息的会话，其他会话打开时 `loadHistory`。否则 500 个会话首次登录就要拉 10 万条 |
| FE-IMP-4 | 收到本地没有的会话的消息（第一次和某人聊、群消息比 convAdded 先到） | 先补临时会话，200ms 后重新拉 convs 用服务端数据替换（昵称、minSeq、未读）；集成测试发现首次聊天的会话名显示 userId 而不是昵称后加上 |
| FE-IMP-5 | 开发依赖 minimatch（eslint 间接依赖）有中危漏洞 | 用 npm `overrides` 升到已修复的 11.x，验证 eslint 正常；npm 建议的"降级 eslint 到 4.0.0"不采用 |

## 总览

| 阶段 | 任务 | 说明 |
|---|---|---|
| F0 工程 | FE-001 | 工程脚手架、测试框架 |
| F1 SDK | FE-002 ～ FE-008 | 传输层、错误码、连接管理、同步引擎、发送队列、已读、测试替身 |
| F2 页面 | FE-010 ～ FE-018 | 登录、会话列表、发起单聊、聊天页、文本展示、群聊界面、文案、离开确认、演示面板 |
| F3 P1 | FE-019 ～ FE-020 | 昵称头像、已读多端同步 |
| F9 v1.3 核心同步 | FE-090 | PRD-core v1.3 中不属于四份新 PRD 的前端改动（第 12 节） |

SDK 先行（F1 可以用假传输层独立开发和测试），页面依赖 SDK 的接口。

---

## F0 工程

### FE-001 工程脚手架
- 优先级：P0
- 依赖：无
- AC：（工程任务）
- 内容：两个仓库，各自用 npm：tianting-sdk-ts（库 `@tianting/sdk`，Vitest，CI `.github/workflows/ci.yml`）、tianting-web（React + Vite，依赖 `@tianting/sdk`）；ESLint + Prettier；Playwright 配置两个浏览器上下文（模拟两个用户 / 两个端）；在 CI 脚本里启动测试模式的 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`）。

---

## F1 SDK（仓库 tianting-sdk-ts）

对外接口按 PLAN 3.2 节的 `IMClient`：`connect / disconnect / state / send / loadHistory / markRead / conversations / messages(convId) / events`。

### FE-002 传输层、帧编解码与错误码
- 优先级：P0
- 依赖：FE-001、BE-001
- AC：AC-SYS-01-3、AC-SYS-01-4
- 内容：
  1. 帧类型从 Schema 生成或手写，用协议的 `vectors/frames.json` 跑编解码测试。
  2. 请求 / 应答匹配：按 `id` 关联 ack，带超时。
  3. 错误枚举由 `errors.json` 生成，并加一致性测试；不认识的错误码按区段兜底（4xx 不重试、5xx 可重试）。
  4. **下行不限帧大小（C-07）**：WebSocket 客户端不能使用库的默认读取上限（常见是 32KB）。后端 B4 实测：500 个会话的 `convs` 应答约 125KB，默认上限下连接会以 1009 关闭。
- v1.3：AC-SYS-01-3 改写（跳过只含 `console` scope 的码、关闭码表含 1011）由 FE-090 承担；PRD-social 新码由 FE-100、第二批码由 FE-208 承担。

### FE-003 连接管理：心跳、重连、被踢
- 优先级：P0
- 依赖：FE-002
- AC：AC-CONN-01-3、AC-CONN-01-5、AC-CONN-01-6、AC-AUTH-02-4、AC-AUTH-03-5、AC-AUTH-05-1、AC-AUTH-02-10
- 内容：
  1. 状态机：断开 / 连接中 / 已连接。
  2. 每 30 秒发 ping，10 秒收不到 pong 判定断线。
  3. 重连按 1s 起指数退避，最长 30s，加 ±20% 抖动；浏览器 `online` 事件触发立即重连。
  4. 收到 `kicked`，或关闭码为 1000、4009～4012、4429 时不重连，向上层抛出原因；**例外**：`kicked/serverShutdown`（1001）按退避重连（C-55）。
  7. **鉴权（PRD v1.2 C-52）**：连接地址只带 `platform`；`open` 后立即发送 `auth` 帧携带 token，收到 auth 的 ack 才进入"已连接"；token 不得出现在 URL 和 SDK 日志里。4401 按普通断线退避重连。
  5. `disconnect()` 以 1000 关闭，并清空状态。
  6. 所有时间参数都可以配置（便于测试）。
- v1.3：关闭码 1011 按退避重连由 FE-090 承担；`kicked/suspended`（封禁，第二批）由 FE-208、FE-352 承担；退出入口改为"我 → 设置"见 FE-010。

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
- v1.3：`convRemoved.reason`（`removed` / `dissolved`，F-GRP-03 规则 2）和会话快照（F-GRP-03 规则 4、V-UI-13）由 FE-205、FE-234 承担；`convRemoved.reason` 类型见 FE-100；`memberCount` 见 FE-204、FE-103。

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
- v1.3：总未读数由 P1 提前到 M3（F-CONV-02 规则 5，VC-04），SDK `totalUnread()` 由 FE-201 承担，"消息"标签角标和标签页标题由 FE-235 承担（PRD-ui F-UI-23）。

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

## F2 页面（仓库 tianting-web）

### FE-010 登录页与会话持久化
- 优先级：P0
- 依赖：FE-003、BE-014
- AC：AC-AUTH-04-1、AC-AUTH-04-2、AC-AUTH-04-4、AC-AUTH-05-1
- 内容：输入用户名 → 调用 `/demo/login` → token 存入 `sessionStorage` → 连接；前端校验用户名格式（"账号格式不正确"）；刷新后用 `sessionStorage` 中的 token 自动重连，停留在原页面；"我 → 设置 → 退出登录"，点击后弹确认框，确认后以 1000 断开（F-AUTH-05 v1.3）；验证构建产物和 HAR 抓包中不含管理密钥（写成 CI 检查脚本）。
- v1.3：退出入口和确认框按 AC-AUTH-05-1（v1.3 改：V-UI-01）实现，"我"页 / 设置页本身由 FE-237 承担；登录页可选昵称（V-UI-15）、生产构建由 FE-223 承担（服务端 `nickname` 见 BE-091）；封禁 4034 的登录提示（第二批）由 FE-352 承担。

### FE-011 会话列表页与连接状态条
- 优先级：P0
- 依赖：FE-004、FE-006
- AC：AC-CONV-01-2、AC-CONV-01-3、AC-CONV-01-5、AC-CONV-01-6、AC-CONV-02-4、AC-CONN-02-1、AC-CONN-02-2、AC-CONN-02-3、AC-MSG-02-3
- 内容：按服务端 `updatedAt` 排序；收到新消息时会话移到顶部；未读角标（超过 99 显示"99+"）；空状态显示"还没有聊天"并保留"发起单聊"按钮；右上角"+"菜单 → "发起单聊"；顶部连接状态条（断开持续超过 1 秒才显示）。
- v1.3："+"菜单、空状态文案、骨架屏和摘要规则由 FE-226、FE-227 承担；总未读角标见 FE-006。

### FE-012 发起单聊弹窗
- 优先级：P0
- 依赖：FE-011
- AC：AC-CONV-03-1、AC-CONV-03-2、AC-CONV-03-3
- 内容：入口为会话列表"+"菜单 → "发起单聊"（空状态按钮保留）；输入对方 `userId` 并校验格式；临时会话只存在本地，发出第一条成功后才进入列表；收到 4042 时提示"用户不存在"。
- v1.3：入口变化见 F-CONV-03 规则 1（V-13、V-UI-02）；"+"菜单补充、好友制下的非阻断"先加好友"提示由 FE-227、FE-243 承担，从资料卡发起由 FE-239 承担。

### FE-013 聊天页：消息列表、发送、历史、滚动
- 优先级：P0
- 依赖：FE-004、FE-005、FE-006
- AC：AC-MSG-01-1、AC-MSG-01-2、AC-MSG-01-3、AC-MSG-02-2、AC-MSG-05-2、AC-MSG-05-6、AC-MSG-06-3、AC-MSG-06-4
- 内容：
  1. 消息气泡区分自己和对方，显示发送状态；失败消息点失败图标弹确认框，点"重发"后重发（AC-MSG-03-3 v1.3 改），长按菜单"重发"不弹确认。
  2. 上拉加载历史：加载中 / 没有更多了 / 加载失败点击重试；加载后可视位置不跳动。
  3. 时间分隔：间隔超过 5 分钟才显示，格式见 PRD F-MSG-06。
  4. 滚动规则：距底部不超过 100px 时自动滚动；否则显示"↓ N 条新消息"。
  5. 输入框按码点计数；为空时发送按钮不可点；超过 5000 时输入框下方实时显示"已超出 N 字"、发送按钮置灰、不发送；"消息过长"只在服务端返回 4006 时提示（C-28，AC-MSG-01-3 v1.3 改）。
- v1.3：失败重发确认框由 FE-230、长按菜单由 FE-231 承担；超长与输入法组字由 FE-232 承担；`convId` 严格解析与"会话不存在"（V-UI-14）由 FE-200、FE-234 承担。

### FE-014 文本渲染与输入行为
- 优先级：P0
- 依赖：FE-013
- AC：AC-MSG-06-1、AC-MSG-06-2、AC-MSG-06-5
- 内容：只用 React 文本节点渲染，禁止 `dangerouslySetInnerHTML`（加 lint 规则）；`white-space: pre-wrap` 加 `overflow-wrap: anywhere`；PC 上 Enter 发送、Shift+Enter 换行；在 iOS Safari 上手工验证键盘弹起时的表现（记录到测试报告）。

### FE-015 群聊界面
- 优先级：P0
- 依赖：FE-013
- AC：AC-GRP-02-4、AC-GRP-02-5、AC-GRP-03-3
- 内容：群消息显示发送者显示名（本机备注 > 昵称 > `userId`，AC-GRP-02-4 v1.3 改）；顶栏显示群名和成员数；收到 4033 或 `convRemoved`（`reason` 缺失或 `removed`）时提示"你已不在该群"并禁用输入框；收到 `convAdded` 时会话列表出现该群。
- v1.3：成员数取 `memberCount`（V-UI-03）、`reason: dissolved` 显示"群聊已解散"（M3）和会话快照由 FE-204、FE-205、FE-234 承担；发送者昵称经 `users` 命令获取由 FE-103 承担。

### FE-016 错误文案映射
- 优先级：P0
- 依赖：FE-002
- AC：AC-SYS-01-4、AC-MSG-01-9、AC-MSG-07-2
- 内容：按 PRD 第 5 节的文案表，从错误码映射到提示文案（集中在一个文件里维护）；不认识的错误码按区段兜底；不直接展示服务端返回的 `msg`。
- v1.3：文案改为按"命令 + 错误码"映射（V-17、V-UI-10、V-UI-11），由 FE-100（SDK `messageFor`）、FE-225（界面）承担；本任务的兜底规则（AC-SYS-01-4）不变。

### FE-017 离开页面确认
- 优先级：P0
- 依赖：FE-005
- AC：AC-MSG-03-5
- 内容：存在发送中或发送失败的消息时，注册 `beforeunload` 提示。
- v1.3：改用 SDK `pendingCount` 判断由 FE-201 承担。

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
- 内容：显示名 = 本机备注 > 昵称 > `userId`；头像为空时显示首字占位（第一个码点，英文转大写），背景色为 8 色调色板的第 `FNV-1a-32(UTF-8(userId)) mod 8` 个，用 `protocol/vectors/avatar.json` 做一致性测试（C-20 v1.3 改：V-UI-12）。
- v1.3：`users` 命令批量查资料（F-USER-02 规则 2）由 FE-103 承担；`avatar.json` 与 `avatarColorIndex` 由 FE-210 承担，头像组件（含群头像 `groupId`）由 FE-224 承担。

### FE-020 已读多端同步
- 优先级：P1
- 依赖：BE-042、FE-006
- AC：AC-SYNC-02-1
- 内容：处理 `readUpdated` 事件，更新本地 readSeq 和未读数。

---

## F9 v1.3 核心同步

> 只放 PRD-core v1.3 第 12 节里不属于四份新 PRD 的前端改动；与 PRD-social / ui / admin / ops 有关的改动在各自任务单里，上面的已有任务只加"v1.3：由 xxx 承担"一行。

### FE-090 错误枚举与关闭码表同步到 v1.3（跳过 console 码、1011 可重连）
- 优先级：P0
- 依赖：FE-002；后端 BE-211（`errors.json` 一次写入新码、`closeCodes` 增加 1011）完成后，用 tianting-sdk-ts 的 `scripts/sync-protocol.sh` 同步 `protocol/`
- AC：AC-SYS-01-3
- 内容：
  1. `scripts/gen-errors.mjs` 生成 `errors.gen.ts` 时**跳过只含 `console` scope 的码**（4014～4016、4037～4039、4092、4093、4291、5031、5040），`check:protocol` 与一致性单测按"非 console 码"比较（VA-14、RECHECK E-05）。
  2. 关闭码表增加 1011 `INTERNAL_ERROR`，`reconnect=true`，按普通断线退避重连（F-SYS-01 规则 5、F-CONN-01 规则 5；VC-02）。
  3. 一致性单测核对 `retryable` 与 `errors.json` 一致（4292 `retryable=false` 的实现在 FE-100，本任务只在一致性测试里断言）。
  4. 测试：TC-SYS-01-08 按 v1.3 改写后的断言通过；变异验证（临时给 `errors.json` 加一个只含 `console` 的码，生成结果和单测都不变）。
- 边界：PRD-social 12 个新码的生成与文案见 FE-100；第二批 4031 / 4034 / `suspended` 见 FE-208；本任务不重复。
