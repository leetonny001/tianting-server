# IM 系统整体方案：服务端 + 移动端 + PC + H5

> 目标：自研一套 IM 服务与多端客户端。
> 1. **公共库**：IM 服务端 + 各端 SDK，未来替换 fx-sport / ab77 / android 里的腾讯云 IM（直播聊天室、站内推送）。
> 2. **独立产品**：基于公共库做一个 IM 聊天软件（移动端、PC、H5）。
> 3. **分享会**：全过程用 AI 完成，并记录可演示的素材。
>
> 不使用 Flutter。

---

## 0. 整体布局（2026-11-02 更新）

| # | 部分 | 内容 | 技术 | 使用者 |
|---|---|---|---|---|
| 1 | **服务端** | IM 服务：长连接、消息、群、会话、聊天室、推送；管理接口、运维接口 | Go 单二进制（可扩展为集群，见 [ARCH-v1](arch/ARCH-v1.md)） | — |
| 2a | **管理后台 H5** | 业务运营：用户、群、消息查询与删除、禁言、系统推送、敏感词 | React（`consoles/admin`） | 运营 / 客服 |
| 2b | **运维后台 H5** | 运维：节点与在线连接、消息量与延迟指标、错误码分布、备份记录、配置检查、审计日志 | React（`consoles/ops`） | 运维 / 开发 |
| 3 | **统一 SDK** | 移动端 + PC + H5 共用的通信网络层和数据层：连接与重连、鉴权、收发、同步、会话、未读、本地缓存 | `sdk-ts`（H5、PC）；`sdk-kotlin`（Android）；`sdk-swift`（iOS）；三套 SDK 跑同一批协议用例 | 各端 App |
| 4 | **客户端 App** | 接入 SDK 开发页面，UI 和基本功能参考微信：会话列表、聊天页、通讯录、我 | H5（React）→ PC（Tauri 壳）→ Android（Compose）→ iOS（SwiftUI） | 最终用户 |

**顺序**：先把核心业务做扎实——服务端核心（B0～B5）→ SDK → H5 客户端；之后是 PC、移动端，以及两个后台。
两个后台和"参考微信的 UI"是新的需求范围，和核心业务一样走完整流程：**产品规划（PRD）→ 需求评审 → 需求分发（后端、前端任务单）→ 测试用例（每个功能点）**，见 [docs/process](process/README.md)。

**推进方式**：AI 按 [工作队列](process/QUEUE.md) 自主推进，遵守 [自主工作规则](process/AUTONOMY.md)；每天早上出日报（`docs/reports/daily/`），需要 Frank 授权或拍板的事项集中在 [待授权清单](reports/PENDING.md)。

---

## 1. 产品范围（参考主流应用）

| 能力 | 参考 | P0 (MVP) | P1 | P2 |
|---|---|---|---|---|
| 账号：注册 / 登录 / 资料 | 微信 | ✅ | 头像、扫码登录（PC） | |
| 单聊 / 群聊 | 微信 | ✅ 文本 | 图片、文件、语音、表情 | 视频 |
| 直播聊天室 | 腾讯 AVChatRoom、抖音直播间 | ✅ 进出、在线人数、最近消息 | 弹幕限流、礼物 / 点赞消息 | |
| 系统推送（业务通知） | 腾讯 REST 系统消息 | ✅ 按用户 / 房间 / 全员 | 定时、灰度 | |
| 会话列表 / 未读数 | 微信 | ✅ | 置顶、免打扰 | |
| 历史消息 / 断线补拉 | 全部 | ✅ | 云端搜索 | |
| 禁言 / 删消息 | 腾讯 IM | ✅ 管理接口 | 群管理员 | 敏感词审核 |
| 撤回 / 编辑 / 引用回复 | 微信 / Telegram | | ✅ | |
| @提醒 / 正在输入 / 已读回执 | Slack / WhatsApp | | ✅ | |
| 好友关系 / 黑名单 | 微信 | | ✅ | |
| 多端同步（手机 + PC + H5 同时在线） | Telegram | ✅ 基础（各端都收到） | 已读状态跨端同步 | |
| 离线推送 | 全部 | | ✅ APNs / FCM | 国内厂商通道 |
| 话题 / 子频道 | Slack / Discord | | | ✅ |
| 音视频通话 | 微信 | | | ✅ WebRTC |
| 端到端加密 | Signal | | | ✅ 私聊 |

---

## 2. 总体架构

```
┌──────────── 客户端 ──────────────────────────────────────────────┐
│ Android (Kotlin+Compose)   iOS (SwiftUI)   PC (Tauri)   H5 (Web) │
│   Kotlin SDK          Swift SDK          └─ TS SDK（共享）─┘   │
└──────────────────────────┬───────────────────────────────────────┘
                           │ WebSocket（JSON → Protobuf） + HTTPS
┌──────────────────────────▼──────────── 服务端（Go）──────────────┐
│ gateway   长连接、心跳、鉴权、多端会话                              │
│ logic     单聊 / 群 / 聊天室路由、seq 分配、禁言、撤回              │
│ api       客户端 HTTP（登录、资料、历史、上传凭证）                 │
│ admin     业务服务端 REST（发推送、禁言、删消息、签 token）+ Webhook │
│ push      离线推送（APNs / FCM / 厂商）                            │
└───────┬───────────────┬───────────────┬───────────────┬──────────┘
     SQLite→Postgres   内存→Redis      内存→NATS       本地→S3/MinIO
     消息、关系          在线路由、房间    节点间投递        图片文件
```

> 可扩展、可迁移、安全优先的详细设计见 [`docs/arch/ARCH-v1.md`](arch/ARCH-v1.md)。

**演进原则**：P0 是单个 Go 进程，所有依赖都可以换成内存或 SQLite 实现，`go run` 一条命令启动。
每个依赖都隔着一层接口，P1 再换成 Postgres、Redis、NATS，拆成多节点部署。

### 2.1 核心机制

| 机制 | 设计 |
|---|---|
| 会话 ID | `c2c:{小uid}:{大uid}`、`group:{gid}`、`room:{rid}` |
| 消息顺序 | 每个会话单调递增 `seq`（DB 事务分配；P1 改 Redis INCR） |
| 不丢 | 客户端记录每个会话的 `maxSeq`；重连或发现 seq 断档时 `sync` 补拉 |
| 不重 | 客户端生成 `clientMsgId`，服务端按 (sender, clientMsgId) 唯一索引去重，重发返回原 ACK |
| 群消息 | 读扩散：消息只存一份，按 `group:{gid}` + seq 拉取；未读 = 会话 maxSeq − 用户 readSeq |
| 聊天室 | 不持久成员、不写会话；内存订阅表 + 环形缓冲（最近 50 条）；高峰时合并广播、限流 |
| 多端 | 一个用户可同时有多个连接（platform 标识）；自己发的消息也同步到自己其他端 |
| 心跳 | 客户端每 30s ping，服务端 90s 无心跳断开；客户端指数退避重连（1s→30s） |
| 鉴权 | 业务服务端用 admin key 换取用户 JWT（对应腾讯 `userSig`）；连接时校验 |

### 2.2 协议（WebSocket 帧，P0 用 JSON）

```jsonc
// 请求
{ "id": "r1", "cmd": "send", "data": { "convType": "c2c|group|room", "to": "u2", "clientMsgId": "uuid", "type": "text", "body": { "text": "hi" } } }
{ "id": "r2", "cmd": "sync", "data": { "convId": "group:g1", "afterSeq": 41, "limit": 50 } }
{ "id": "r3", "cmd": "joinRoom", "data": { "roomId": "match-123" } }
{ "id": "r4", "cmd": "read", "data": { "convId": "c2c:u1:u2", "seq": 42 } }
{ "cmd": "ping" }

// 响应 / 下行
{ "id": "r1", "cmd": "ack",  "data": { "msgId": "m1", "seq": 42, "ts": 1730000000000 } }
{ "id": "r1", "cmd": "ack",  "error": { "code": 4031, "msg": "muted" } }
{ "cmd": "msg",   "data": { "msgId": "m1", "convId": "...", "seq": 42, "from": "u1", "type": "text", "body": {...}, "ts": ... } }
{ "cmd": "push",  "data": { "type": "goal", "data": {...} } }
{ "cmd": "event", "data": { "type": "msgDeleted|muted|roomOnline|convUpdated", ... } }
```

协议唯一真相源：`protocol/` 目录下的 JSON Schema（P1 换成 `.proto`）+ **协议测试用例**（输入帧 → 期望输出），
服务端和三套 SDK 都必须跑通同一批用例。

### 2.3 管理接口（给业务服务端）

> 一期已实现的接口和规则见 [protocol/ADMIN.md](../protocol/ADMIN.md)；下表是整体规划。

| 接口 | 用途 | 替代腾讯 IM |
|---|---|---|
| `POST /admin/users/token` | 签发用户 token | `genUserSig` |
| `POST /admin/push` `{target: user\|room\|all, id, type, data}` | 系统推送 | `send_group_msg_with_attr` |
| `POST /admin/mute` | 禁言 | contentType 1 |
| `POST /admin/messages/delete` | 删消息并通知 | contentType 3 |
| `POST /admin/rooms` / `/admin/groups` | 建房间、建群、管理成员 | 群管理 REST |
| Webhook：发消息前 / 后 | 敏感词、业务审核 | 腾讯回调 |

---

## 3. 客户端方案

### 3.1 选型

| 端 | 技术 | 理由 |
|---|---|---|
| SDK（Android） | **Kotlin**：OkHttp WebSocket + kotlinx.serialization + 协程 / Flow + Room | 纯 Kotlin，Android 原生 |
| SDK（iOS） | **Swift**：URLSessionWebSocketTask + Codable + async/await + SQLite（GRDB） | 纯 Swift，无 KMP / XCFramework；接入 ab77 最简单 |
| Android | Kotlin + Jetpack Compose | 原生体验；顺便学习 Android |
| iOS | Swift + SwiftUI | 原生体验；对接 ab77 最直接 |
| SDK（Web） | **TypeScript**：原生 WebSocket + IndexedDB（Dexie） | H5 和 PC 共用 |
| H5 | React + Vite（移动端布局） | 生态最大，AI 生成质量最好 |
| PC | **Tauri 2** 包装同一套 React 代码（桌面布局） | 包体约 10MB（Electron 约 100MB）；需要装 Rust，但壳代码很少。装不了就退回 Electron |

> 2026-11-03 Frank 拍板：Android 全部 Kotlin、iOS 全部 Swift（含 SDK），不用 KMP。代价是移动端 SDK 多写一套；三套 SDK 行为一致靠 [SDK 设计说明](sdk/README.md) 和 `protocol/vectors` 共享用例保证。

为什么不做"一个核心给所有端用"（OpenIM 的 Go + gomobile + wasm 方案）：Go 编译成 wasm 体积有 10MB 以上，H5 首屏加载受不了。
所以选**每个平台一套原生 SDK（TS / Kotlin / Swift）+ 一份协议 + 共享测试用例**，靠协议用例保证各端行为一致。

### 3.2 SDK 统一接口（三套 SDK 语义相同；Swift 里 `Flow` 对应 `AsyncStream` / `@Published`）

```
IMClient
  connect(token) / disconnect()
  state: Flow<Disconnected|Connecting|Connected>
  send(conv, body) -> Message           // 本地先插入 sending 状态，ACK 后更新
  loadHistory(convId, beforeSeq, limit)
  joinRoom(roomId) / leaveRoom()
  markRead(convId, seq)
  conversations: Flow<List<Conversation>>
  messages(convId): Flow<List<Message>>
  pushes: Flow<Push>                    // 系统推送，业务方按 type 分发
  events: Flow<Event>
```

这套接口也是将来接入 fx-sport / ab77 / android 的 `IMClient` 抽象层，业务代码只依赖它，不再直接依赖腾讯 SDK。

### 3.3 页面（各端相同）

登录 → 会话列表 → 聊天页（单聊 / 群聊）→ 聊天室页（模拟比赛直播间：在线人数 + 消息流 + 推送横幅）→ 通讯录 / 建群 → 我的。
PC 用三栏布局（导航 / 会话列表 / 聊天），H5 和移动端用单栏页面跳转。

---

## 4. 仓库结构（monorepo）

```
im-mvp/
├── docs/                    # 方案、ADR、分享会演示记录
├── protocol/                # JSON Schema / proto + 协议测试用例（共享）
├── server/                  # Go
│   ├── cmd/im-server/
│   ├── internal/{gateway,logic,api,admin,push,hub,store,auth}/
│   └── e2e/
├── sdk-kotlin/              # Kotlin SDK（Android）
├── sdk-swift/               # Swift SDK（iOS，Swift Package）
├── sdk-ts/                  # TypeScript SDK（H5 + PC）
├── android/                 # Compose App
├── ios/                     # SwiftUI App
├── web/                     # React：H5 + PC 共用（UI 参考微信）
├── desktop/                 # Tauri 壳，加载 web/
├── consoles/
│   ├── admin/               # 管理后台 H5（业务运营）
│   └── ops/                 # 运维后台 H5
└── tools/                   # 压测、模拟业务服务端（发推送脚本）
```

---

## 5. 测试策略

| 层 | 内容 |
|---|---|
| 服务端单测 | seq 分配、去重、禁言、路由（`go test`） |
| 服务端 e2e | 启动真实服务，多个 WS 客户端跑场景：断线重连不丢不重、聊天室进出、推送到达 |
| 协议一致性 | `protocol/` 用例，服务端 + TS / Kotlin / Swift 三套 SDK 都跑 |
| SDK 单测 | 用假服务端测重连、补拉、本地状态机 |
| UI | Android：Compose 测试；iOS：XCUITest 冒烟；Web：Playwright（H5 + PC 两个尺寸） |
| 压测 | Go 写压测工具：1 万连接、单房间 5000 人广播的延迟和 CPU |
| 联调 | Android 模拟器 + iOS 模拟器 + 浏览器 + PC 四端同时在一个聊天室 |

---

## 6. 里程碑

| 阶段 | 内容 | 验收 | 可演示的 AI 用法 |
|---|---|---|---|
| **M0 规划与骨架** | 本方案、装 Go、仓库骨架、`/healthz`、协议 Schema 初稿 | `go run` 能起 | 先规划后编码；让 AI 讲解 Go 项目结构 |
| **M1 服务端核心** | 鉴权、WS 网关、心跳、单聊 / 群聊、ACK、seq、去重、sync | e2e：断线重连不丢不重 | 用不熟的语言写代码；测试驱动 |
| **M2 聊天室 + 管理接口** | 聊天室、在线人数、推送、禁言、删消息、Webhook | 脚本推送能实时到达 | 照着腾讯 IM 的现有用法迁移需求 |
| **M3 Web SDK + H5** | TS SDK + React H5 | 两个浏览器标签页互聊、进聊天室 | 最快出可见效果；用截图驱动改 UI |
| **M4 PC** | Tauri 壳 + 桌面三栏布局 | macOS 安装包能跑 | 一份代码复用到多端 |
| **M5 Kotlin SDK + Android** | Kotlin SDK、Compose App | 模拟器和 H5 互聊 | 用 AI 学新平台（Android） |
| **M6 Swift SDK + iOS** | Swift SDK、SwiftUI App | 四端同时在一个聊天室 | 多端联调；排查跨端 bug |
| **M7 加固** | 压测、Protobuf、离线推送、README、部署脚本 | 压测报告 | 让 AI 分析性能瓶颈 |
| **M8 分享会** | 整理演示记录、幻灯片、现场演示脚本 | 彩排一次 | 让 AI 帮你做幻灯片 |
| 后续 | P1 功能、Postgres / Redis / NATS 集群化、接入 fx-sport / ab77 / android | | |

先做 H5（M3）再做原生端：Web 出效果最快，服务端协议可以在这一步先磨稳定，后面原生端返工少。

---

## 7. 环境准备

| 工具 | 现状 | 动作 |
|---|---|---|
| Go | 未安装 | `brew install go`（M0） |
| Node | v26 ✅ | 装 pnpm（M3） |
| Rust | 未安装 | `rustup`（M4，Tauri 需要） |
| Xcode | 27 ✅ | — |
| Android SDK / Java | ✅ | Android Studio（M5） |
| Docker | 未安装 | P0 不需要；集群化阶段再装 |

---

## 8. 风险

| 风险 | 应对 |
|---|---|
| 范围太大，四端做不完 | 严格按里程碑推进，每个阶段都能独立演示；分享会最低要求是 M3（服务端 + H5） |
| 三套 SDK 行为不一致 | 共享协议测试用例和 SDK 设计说明，CI 里服务端和三套 SDK 都要跑 |
| 自研 IM 稳定性不如腾讯 | 生产替换前先灰度：业务代码通过 `IMClient` 抽象层，同时保留腾讯实现，可随时切回 |
| 聊天室广播风暴 | 消息合并下发、限流、超大房间采样 |

---

## 9. 演示记录（分享会素材）

每个里程碑结束时补一条：

| 里程碑 | 关键提示词 | AI 做对了什么 | AI 出错 / 被纠正 | 耗时 |
|---|---|---|---|---|
| M0 | 「先规划整个计划方案 服务器端+移动端+PC+H5」→「开始 M0」 | 先看本机已经装了哪些工具，按"零外部依赖"来设计（SQLite、不用 Docker）；用协议测试保证 Schema 和用例一致（28 条用例） | ① 冒烟脚本里 curl 重试没有间隔，服务还没监听就判定失败；② 对后台进程发 SIGINT 会被忽略，导致脚本卡住；③ zsh 把 `===` 当成命令展开报错。三处都是读日志后自己定位并修正的 | 约 15 分钟 |
| M0 收尾 | 「代码提交到 github」 | 拒绝使用聊天里贴出的明文密码，改走 `gh auth login` 设备码流程；用户手动推送 | ① 终端交互卡在 "Press Enter"，AI 没法替用户按键，改成 `echo \| gh auth login` 绕开；② gh 默认用 SSH 地址，但公钥没上传，于是改用 HTTPS；③ 自动权限检查拦截了 AI 直接推送，改由用户执行；④ 一条被拒绝的命令其实已经执行，留下一把失效公钥，查出来后如实告知并删除 | 约 40 分钟 |
| M1 | 「开始 M1」 | 分层：protocol → auth/store → hub/gateway → logic/admin → app；14 个 e2e 场景（多端投递、重发去重、离线补拉、群权限、并发 seq、心跳超时）`-race` 下一次通过 | 一次全过反而可疑，于是做了**变异测试**：故意注入"消息回显给发送端"和"去重失效"两个 bug，确认测试会失败后再恢复代码。另外，并发测试暴露出"实时投递不保证 seq 顺序"，这一点已写进协议第 7 条 | 约 30 分钟 |
| 架构调整（B2 后） | 「后端架构要方便后期扩展分布式部署，数据库要方便后期迁移 备份 扩展，聊天软件还应该把安全放第一位」 | 没有直接改代码，先对照现有实现盘点 11 个差距（进程内在线表、写死的建表语句、token 在 URL 里等），写成架构与安全设计文档，给出待确认决策；发现 B3 的群成员缓存若按单机写会返工，建议把基座排在 B3 前 | — | — |
| B2.5 架构与安全基座 | 「按建议通过」 | 先改 PRD（v1.2）、协议、用例，再写代码；每项安全能力都配测试，并用变异测试证明测试真能抓到问题；gosec / govulncheck 扫描 | 自己写的配置测试抓到一处真实泄露：密钥配置格式写错时，错误信息会把密钥原文打印出来；关闭码 4013 和已有错误码撞号，改为 4429；用 json.dump 改 errors.json 把原来的对齐格式冲掉了，回退后改用文本编辑 | — |
| B3 群 | 「开始 B3」 | 存储层先写契约测试（含"并发加人不能突破 2000"）；成员缓存按 B2.5 的约束走失效接口，并处理"加载期间被失效"的竞态；对缓存失效、历史可见性、竞态处理分别做变异测试 | 测试代码里 `append(us[:500], "ghost")` 原地覆盖了共享切片，导致后面的用例混进不存在的用户——是测试 bug，不是服务端 bug，改用 `slices.Concat` | — |
| B4 会话 | 「先继续 B4」 | 先在存储层把会话列表、可见性、截断、迁移回填都写成契约测试；变异测试发现"发言即已读"在"之前已有已读记录"时没被覆盖，补了用例；发现建 2000 人群时逐个查会话（2000 条查询），改为一条批量查询（-race 下 55 秒 → 6 秒） | 500 个会话的应答约 125KB，测试客户端用库默认的 32KB 读取上限直接断开——同样的坑会出现在 SDK 里，已写进前端任务；批量查询的 SQL 漏了列别名，靠补契约测试定位；一条调试命令里多写了 `cat` 导致卡住 | — |
| B5 压测与可观测 | 「继续 Q5」 | 压测工具自己起服务端子进程，才能做 kill -9 用例；每个场景结束都用 sync 核对和按错误码统计来自证；7 条性能用例全部通过，余量 15～60 倍；如实记录内存峰值偏高 | 工具自己的 3 个 bug（两次 clientMsgId 重复、一次内存基准选错）都靠结果自检发现——如果没有 sync 核对，第一个 bug 会让吞吐"看起来"达标 | 全量压测 37 分钟 |
| Q5a 内存分析 | 「先做 Q5a」 | 先用数据区分"存活数据"和"GC 余量"，没有一上来就调参数；用 pprof 找到每连接预分配的 channel 和常驻写协程，修复后 goroutine 减半；用 GOMEMLIMIT 实验说明上限设错的代价（CPU 26% → 493%） | 第一次判断泄漏时用了进程 RSS，结果被 macOS 内存压缩的噪声误导（30 秒内 112MB ↔ 557MB），改用强制 GC 后的存活内存；对比实验的循环脚本第二轮没跑起来，单独重跑 | 约 1 小时 |
| Q6 统一 SDK（TS） | 「开始 Q6」 | 分层 + 时间全部可注入，82 个单元测试毫秒级跑完；和服务端共享错误码与文本用例（新增 `protocol/vectors/text.json`，Go 和 TS 跑同一份）；最后用真实 im-server 做集成测试 | 单元测试抓到"被踢后消息先标失败、异步回调又改回排队"的竞态；集成测试发现第一次聊天会话名显示 userId 而不是昵称（补了临时会话刷新）；第一版丢 ack 用例在发送后才注入故障、不确定，改成先固定 clientMsgId 再注入 | — |
| Q8～Q11 产品规划与评审 | 「开始产品规划」 | 三份 PRD 并行交给独立子代理起草，再由另外三个独立子代理评审（不让写的人审自己）；汇总时把约 40 个拍板项提炼成 14 个真正需要负责人决定的；关键评审意见都回到代码或原文核实（对比度复算、管理密钥和 /ws 同端口） | 草稿自己写"配色已按对比度选取"，复算深色主按钮只有 2.98:1；两份后台草稿都说"共用账号和审计"，细节却互相冲突，按其中一份的推荐管理后台超管能接管运维账号——这类问题只有跨文档评审才能发现；汇总时更新队列的脚本引号写错，第一次没改到，补提交 | — |
