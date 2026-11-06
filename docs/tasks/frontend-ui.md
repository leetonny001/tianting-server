# 前端任务单：客户端 UI 与交互（PRD-ui v1.0）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-ui v1.0](../prd/PRD-ui-v1.0.md)（冻结，280 条 AC）；背景：[PRD-core v1.3](../prd/PRD-core-v1.3.md)、[PRD-social v1.0](../prd/PRD-social-v1.1.md)（社交页面的数据和规则）、[PRD-admin v1.0](../prd/PRD-admin-v1.0.md)（第二批客户端提示）、[SDK 设计说明](../sdk/README.md)、[PLAN](../PLAN.md) 第 4 节（多仓库） |
| 负责 | 前端（AI） |
| 技术栈 | tianting-web：React + Vite + TypeScript，测试 Vitest + Testing Library（组件）、Playwright ≥ 1.48（Chromium + WebKit，`page.routeWebSocket`）、axe-core；tianting-sdk-ts：TypeScript，Vitest + `FakeTransport` / `FakeClock`；tianting-desktop：Tauri 2（macOS WKWebView、Windows WebView2）；原生端：Kotlin + Compose（Compose UI Test）、Swift + SwiftUI（XCUITest） |
| 在已有任务之上 | H5 基础 FE-010～FE-020（[frontend.md](frontend.md)，Q13）不重复；本单的任务在其上补充。PRD-social 的 sdk-ts 社交协议层（好友、申请、黑名单、搜索、`users`、`myGroups`、M4 群命令、`tip`、`convRemoved.reason`）在 [frontend-social.md](frontend-social.md) 的 FE-100～FE-149，本单只做 PRD-ui 第 7 节的 SDK 缺口 G-UI-xx 和界面 |
| 开工条件 | M3：FE-001 的 `web/` 部分（FE-IMP-2）与 FE-220 一起完成；sdk-ts 社交任务（frontend-social.md）可与 FE-200～FE-210 并行 |
| 完成定义 | 任务内列出的 AC 都有通过的测试（Vitest / Playwright / 端到端）并能在 [TC-UI](../testcases/TC-UI.md)、[TC-UI-M4](../testcases/TC-UI-M4.md)、[TC-UI-B2](../testcases/TC-UI-B2.md) 中找到对应用例；（手工）AC 有截图或录屏记录；`npm test`、`npm run e2e`、CI 检查（对比度、颜色字面量、品牌词、包体积、CSP）全部通过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。【Web】AC 只在 Web 上验证，原生端按 PRD-ui 3.6 节对照表在 FE-380～FE-399 中执行。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| FE-203～FE-206、FE-210 | ✅ 完成 | tianting-sdk-ts `0561001`、`6ced393`：快照引用稳定、`memberCount` + `refreshConversations`（30 秒节流）+ `truncated()`、被移出快照（`reason`、重连推断、截断不推断）、打开未知会话向服务端确认、`avatarColorIndex` / `avatarInitial` 跑 `avatar.json` |
| FE-207、FE-209 | — | 编号空缺，没有任务 |
| FE-208 | ✅ 2026-11-05 | sdk-ts `38ed458`（第二批事件）、`f78d5f1`（`pushes` 流）；`push-sync.json` 适用的 66 个用例全部通过，CI 含真实 im-server 集成。原生端 2026-11-05 完成：sdk-kotlin `7a44785`、sdk-swift `8d29fa9`、`3454975`，`push-sync.json` 都是 71/71 |
| FE-223、FE-226、FE-228、FE-234、FE-245～FE-247 | ✅ 完成（Web） | tianting-web `ee5c73d`：昵称登录、截断提示、"群名（N）"与聊天信息页、被移出 / 解散 / 未知会话、axe 扫描零违规、CSP + `uiPrefs`、500 会话 4 倍降速中位数约 376ms。手工项（VoiceOver、iOS Safari）未做 |
| FE-306～FE-311 | ✅ 完成（Web） | tianting-web `06be970`：发起群聊、群设置（角色 × 托管矩阵）、群公告、成员列表与管理模式、邀请、提示消息、`reason=left`；真实服务端 e2e 覆盖 AC-UI-06-8、33-7、33-8、35-1、36-5～7、37-1、38-1、38-2、38-4、38-5、39-1，新页面已加入 axe 扫描。同时把解散后的禁用条改为 F-UI-33 规则 4 的"群聊已解散"（M3 多写了"，无法发送消息"）。F-UI-39 规则 3 的多条结果合并成一条轻提示（轻提示同时只显示 1 条）。未验证：WebKit、真机 VoiceOver（同 FE-248）。 |
| FE-238～FE-244 | ✅ 完成（Web） | tianting-web：通讯录（拼音分组，`protocol/vectors/pinyin-groups.json`）、资料卡、新的朋友、添加朋友、申请、备注、黑名单、隐私、私聊权限提示条、群聊列表；群消息发送者昵称、头像图片、官方标识、会话列表备注优先。组件测试 43 个、端到端 6 个、axe 覆盖新页面；2000 好友刷新后约 475ms 显示完成。未做：好友制 / 陌生人限额的提示条端到端（测试服务端是默认开放模式，这两条只有组件测试，服务端行为由 `TestTCSOC08*` 覆盖）、AC-UI-13-9 在 WebKit 上跑一次 |
| FE-248（M3 手工验收） | ⬜ 未开始 | iOS Safari、VoiceOver 需要人工 |
| FE-300～FE-305 | ✅ 2026-11-05（Web） | tianting-web `eb802bf`、sdk-ts `50b4071`，CI 通过。只跑 Chromium，WebKit、真机、VoiceOver、真实拼音输入法没测。已知：Tab 会经过消息头像，消息多时要按很多次才到输入框；SDK 不监听 offline 事件，真实断网时状态条最长约 35 秒才出现 |
| FE-312、FE-313 Tauri | ⏸ 等 Rust 授权（P-14） | |
| FE-350～FE-352 | ✅ 2026-11-05 | 随 FE-431～433 实现，AC-UI-41、42、43 已核对有用例 |
| FE-353 | ✅ 2026-11-05 | tianting-web `edb54df`（系统通知页 `/me/notices`、横幅、未查看数），CI 通过。真实多标签页竞态只有组件测试 |
| FE-354 | ✅ 2026-11-06 | tianting-web `d3dcfdb`、sdk-ts `2ca3dfe`、`da58578`；原生 sdk-kotlin `c64724e`、Android `0594b17`、sdk-swift `e039ec8`、iOS `0f77714`。SDK 按 `myRole` 放行群主和管理员，和 F-UI-41 规则 2 字面不同（P-22）；CI 未执行（P-21） |
| FE-380～FE-382、FE-386 Android | 🟡 2026-11-05 首版 | tianting-sdk-kotlin `4fe4934`、tianting-android `95b963b`。本机：JVM 21 个、Compose UI 120 个、真实服务端端到端 7 个（3 轮里 1 轮有 1 个失败，判断是测试框架竞态，未证实）。**未验证**：CI 的 android job（等 P-15）、真机、Pixel 7 API 34、输入法组字（AC-UI-10-2、10-7）、多端红点（23-4）、关闭可被搜索（30-1）、600 群（32-2）、断网状态条 **2026-11-05 系统通知**：tianting-android `95b963b`、`06c470d`、`31dd1d6`；Compose 138 个、真实服务端端到端 9 个、R8 logcat 检查通过。 |
| FE-383～FE-385、FE-387 iOS | 🟡 2026-11-05 首版 | tianting-sdk-swift `4815019`、tianting-ios `fba33c7`。适用 AC 223 条，173 条有编号引用（grep 统计，不等于逐条复核）；约 37 条已实现但没有自动化（要改写 ack 或造数据，见 tianting-ios 提交记录）。**未验证**：CI 的 build-and-test（等 P-15）、真机、iOS 17、输入法组字（AC-UI-10-2、10-7）、多端红点（23-4）、Release 真实收发 **2026-11-06 系统通知与修复**：tianting-ios `c5ae142`、`df6efe7`；单测 77 个，XCUITest 47 个在 iOS 27.0、iOS 18.5 模拟器各跑 1 轮全过。修复了真实 Bug：群里第一条和发送失败的单聊第一条，发出后要滚动才显示（iOS 27 上复现；根因只定位到 `LazyVStack` + `defaultScrollAnchor` 的表现，修复是实验得出）。 |
| FE-200～FE-202 SDK 缺口 | ✅ 完成 | tianting-sdk-ts `572312a`、`3eb87f7`：`parseConvIdStrict`（`protocol/vectors/convid.json`）、`peerId`、`totalUnread` / `pendingCount` 事件、`sendFailed` |
| FE-220～FE-222、FE-224、FE-229～FE-232、FE-235、FE-237（M3 部分） | 🟡 第一批 | tianting-web `3859f45`：工程与 CI 检查（token、对比度、颜色字面量、品牌词、首屏体积）、应用壳、通用组件、气泡、发送状态、消息菜单、输入区、状态条 / 被踢弹窗 / 标题、我与设置。社交页面 FE-238～FE-244 等 PRD-social 服务端 |

## 实现约定（前端 UI）

| # | 问题 | 约定 |
|---|---|---|
| FE-UI-IMP-1 | PRD-ui 没有规定设计 token 的"唯一来源"文件放在哪里 | 放在 tianting-web `src/theme/tokens.json`（键名同 6.1 节，如 `color.brand`），由脚本生成 `--tt-*` CSS 变量；对比度 CI（AC-UI-90-3）、色相 CI（AC-UI-90-5）读这一份；原生端（FE-381、FE-384）复制该文件生成 `TiantingTheme` / `Tokens.swift`，跑同一个校验脚本 |
| FE-UI-IMP-2 | 非 `send` 命令的 4290 文案：PRD-core 默认文案是"发送太频繁，请稍后再试"，但 F-UI-26 规则 5、AC-UI-30-2 要求 `userSearch` / `setPrivacy` 显示"操作太频繁，请稍后再试" | 文案表中 `send` 4290 →"发送太频繁，请稍后再试"；其他命令 4290 →"操作太频繁，请稍后再试"（按"命令 + 码"表实现，FE-225） |
| FE-UI-IMP-3 | `parseConvIdStrict` 三套 SDK 共用用例 | 新增 `protocol/vectors/convid.json`（AC-UI-33-13 的全部输入和期望），sdk-ts / sdk-kotlin / sdk-swift 都跑 |

## 总览

| 阶段 | 任务 | 仓库 | 任务数 | 人日 |
|---|---|---|---|---|
| U0 SDK 缺口（M3） | FE-200～FE-206、FE-210 | tianting-sdk-ts、tianting-server（`protocol/vectors`） | 8 | 10 |
| U1 H5 工程与基础（M3） | FE-220～FE-225 | tianting-web | 6 | 10.5 |
| U2 H5 基础页面（M3） | FE-226～FE-237 | tianting-web | 10 | 15 |
| U3 H5 社交页面（M3） | FE-238～FE-244 | tianting-web | 7 | 12.5 |
| U4 非功能与回归（M3） | FE-245～FE-248 | tianting-web | 4 | 5.5 |
| **M3 小计** | | | **35** | **53.5** |
| U5 宽屏、深色、表情、草稿（M4） | FE-300～FE-305 | tianting-web（FE-305 含 tianting-sdk-ts `removeFailed`） | 6 | 10 |
| U6 客户端群管理（M4） | FE-306～FE-311 | tianting-web | 6 | 10.5 |
| U7 PC（M4） | FE-312～FE-313 | tianting-desktop | 2 | 3.5 |
| **M4 小计** | | | **14** | **24** |
| U8 第二批 | FE-208、FE-350～FE-353 | tianting-sdk-ts（1）、tianting-web（4） | **5** | **7.5** |
| P1 | FE-354 | tianting-web | 1 | 0.5 |
| **合计（sdk-ts + H5 + PC）** | | | **55** | **85.5** |
| U9 原生端（M5 / M6，总任务，不计入合计） | FE-380～FE-387 | tianting-sdk-kotlin、tianting-android、tianting-sdk-swift、tianting-ios | 8 | 约 34 |

各任务人日：

| 任务 | 人日 | 任务 | 人日 | 任务 | 人日 | 任务 | 人日 |
|---|---|---|---|---|---|---|---|
| FE-200 | 1 | FE-224 | 2 | FE-238 | 2.5 | FE-305 | 1.5 |
| FE-201 | 1 | FE-225 | 1 | FE-239 | 2 | FE-306 | 2 |
| FE-202 | 1 | FE-226 | 1.5 | FE-240 | 1.5 | FE-307 | 2.5 |
| FE-203 | 1 | FE-227 | 1 | FE-241 | 2.5 | FE-308 | 1 |
| FE-204 | 1 | FE-228 | 2 | FE-242 | 2 | FE-309 | 2.5 |
| FE-205 | 2.5 | FE-229 | 1.5 | FE-243 | 1 | FE-310 | 1 |
| FE-206 | 1.5 | FE-230 | 1 | FE-244 | 1 | FE-311 | 1.5 |
| FE-210 | 1 | FE-231 | 1.5 | FE-245 | 1.5 | FE-312 | 2.5 |
| FE-220 | 2 | FE-232 | 1.5 | FE-246 | 1.5 | FE-313 | 1 |
| FE-221 | 2 | FE-234 | 2 | FE-247 | 1.5 | FE-208 | 2.5 |
| FE-222 | 2 | FE-235 | 1.5 | FE-248 | 1 | FE-350 | 1.5 |
| FE-223 | 1.5 | FE-237 | 1.5 | FE-300 | 3 | FE-351 | 1 |
| — | — | — | — | FE-301 | 1.5 | FE-352 | 1 |
| — | — | — | — | FE-302 | 1.5 | FE-353 | 1.5 |
| — | — | — | — | FE-303 | 1.5 | FE-354 | 0.5 |
| — | — | — | — | FE-304 | 1 | FE-380～387 | 5 / 8 / 3 / 5 / 8 / 3 / 1 / 1 |

编号空缺（FE-207、FE-209、FE-211～219、FE-233、FE-236 等）留给后续拆分，不复用。

### 与已有估算对照

PRD-ui v1.0 本身**没有给出人日估算**。可对照的是其他文档中与本 PRD 范围重叠的估算：

| 里程碑 | 本单 | 已有估算 | 差异说明 |
|---|---|---|---|
| M3 | 53.5（SDK 缺口 10 + H5 43.5） | PRD-social 15.1 节：H5 社交页面 6、H5 基础 Q13 约 8（FE-010～018，已在 frontend.md，不计入本单）；PRD-admin 第 14 节：sdk-ts + H5 `convRemoved.reason` 1 | 已有估算只算"社交页面 + 基础页面"，没有计入 PRD-ui v1.0 新增的：SDK 缺口 G-UI-03 / 15 / 19～22 / 24 / 26（10）、设计 token 与对比度 / 色相 / 颜色字面量 CI（2）、路由与回跳校验、布局、头像、共享测试数据（约 5）、F-UI-33 全部边界（SDK 4 + 界面 2）、可访问性 / 安全 / 性能三项非功能 AC（4.5）。社交页面本单 12.5，比 PRD-social 的 6 多一倍：PRD-ui 给社交页面写了 66 条 AC（F-UI-13、14、25～32），PRD-social 估算时 PRD-ui v0.2 交互规格还没写（REVIEW-social R-SOC-17 ②已指出这一点） |
| M4 | 24 | PRD-social 15.2 节：H5 群页面 4.5（sdk-ts 群 API 2.5 在 frontend-social.md） | 群页面本单 10.5（F-UI-35～40 共 34 条 AC，含权限矩阵和提示消息文案）；宽屏三栏、深色、表情、草稿、Tauri 共 13.5 在已有估算中都没有 |
| 第二批 | 7.5 | PRD-admin 第 14 节：sdk-ts + H5 4 | 本单把 sdk-ts 事件（2.5）与四个界面（5）分开估；push 协议未定稿，FE-208 / FE-353 有返工风险 |

估算口径同 PRD-social 15.3 节：传统人日，用于排序和比较规模；按 Q1～Q7 的实际速度，SDK 部分可能更快，H5 与测试设施没有历史数据，不打折。M3 第一周结束后用实际数据校准。

### 依赖关系（M3 关键路径）

FE-001（`web/`）→ FE-220 → FE-221 → FE-222 → FE-224 → 页面任务；SDK 缺口 FE-200～FE-206 与页面并行，F-UI-33（FE-234）等待 FE-200、FE-205、FE-206；社交页面（FE-238～FE-244）等待 frontend-social.md 的 sdk-ts 社交任务和 PRD-social 服务端。

---

## U0 SDK 缺口（tianting-sdk-ts，M3）

### FE-200 严格会话 ID 解析 `parseConvIdStrict` 与 `peerId`（G-UI-03）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts、tianting-server（`protocol/vectors/convid.json`）
- 依赖：FE-002
- AC：AC-UI-33-13、AC-UI-33-1、AC-UI-33-2
- 内容：
  1. 新增 `parseConvIdStrict(convId, self)`：`c2c` 恰好三段、两个 ID 符合 userId 格式、第一个按 ASCII 字节序严格小于第二个，否则抛 `ConvIdError{kind:'invalid'}`；两个都不是 `self` 抛 `notParticipant`；`group` 恰好两段且 groupId 合法；`room` 和其他前缀一律 `invalid`。成功返回 `{convType, to}`。
  2. 新增 `peerId(conv)`；现有 `parseConvId` 保留给 SDK 内部，加注释"界面不得用它判断路由"。
  3. 共享用例 `protocol/vectors/convid.json`（FE-UI-IMP-3），包含 AC-UI-33-13 列出的全部输入。
  4. 测试：Vitest 跑 `convid.json`；导出项加入 `index.ts` 的公开 API 快照测试。

### FE-201 `totalUnread()` 与 `pendingCount()`（G-UI-15、G-UI-20）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-005、FE-006
- AC：AC-UI-23-1、AC-UI-23-2、AC-UI-23-3、AC-UI-16-2
- 内容：
  1. `totalUnread()` = 全部会话未读之和，变化时发 `totalUnread` 事件（与会话事件同一轮，避免闪烁）。
  2. `pendingCount()` 跨会话统计 `queued` / `sending` / `failed` 条数，变化时发 `pendingCount` 事件；FE-017 的 `beforeunload` 改为读它。
  3. 测试：Vitest（`FakeTransport` 推三个会话的消息、打开会话已读、发送失败 / 重发），断言事件次数和取值。

### FE-202 `sendFailed` 事件（G-UI-22）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-005
- AC：AC-UI-08-3、AC-UI-08-5
- 内容：每次发送失败恰好发出一次 `sendFailed(convId, clientMsgId, error)`（`error` 含 `code` 或 `timeout` / `kicked`）；重发后再次失败再发一次；会话列表刷新、`conversations` 回调不会重复触发。测试：Vitest 覆盖 4290 两次失败、超时、被踢三种情况的事件计数。

### FE-203 会话快照引用稳定（G-UI-19）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-004
- AC：AC-UI-90-7
- 内容：`store.ts` 去掉每次 `map` 生成新对象；未变化的会话在两次 `conversations` 回调之间保持同一引用（`Object.is`）。测试：Vitest 构造 500 个会话，推 1 条消息后断言只有 1 个会话对象引用变化；界面侧的 Profiler 断言在 FE-247。

### FE-204 `refreshConversations()` 与 `memberCount`（G-UI-24、G-UI-02）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-004；PRD-social 服务端 `memberCount`（协议 `Conversation.memberCount`，backend 任务）
- AC：AC-UI-06-6、AC-UI-06-2
- 内容：`Conversation` 类型增加可选 `memberCount`（缺失时保持 `undefined`，不填 0）；`refreshConversations()` 重新拉 `convs`，SDK 内 30 秒节流（节流期内直接返回当前快照）。测试：Vitest + `FakeClock`（29 秒内第二次不发 `convs`、30 秒后发）；缺失字段的帧解析。

### FE-205 被移出群的会话快照（G-UI-21）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-004；frontend-social.md 的 sdk-ts `convRemoved.reason` 解析；PRD-admin 第一批管理接口解散群（F-ADM-24）
- AC：AC-UI-33-4、AC-UI-33-5、AC-UI-33-9、AC-UI-33-11、AC-UI-33-12、AC-UI-33-14、AC-UI-12-5
- 内容：
  1. 收到 `convRemoved`：从会话列表移除，保留快照 `{convId, name, type, removed: true, reason?}` 和已加载消息，提供 `conversationSnapshot(convId)`；保留到本次登录结束（退出、被踢、刷新清除）。
  2. **重连 / `refreshConversations` / 内部补拉**：新旧 `convs` 比对，原来在、新列表没有的**群会话**生成快照（无 `reason`）并发 `convRemoved`；单聊不处理；`truncated: true` 时不推断（修 `client.ts` 第 279～281 行 `setConversations` 整体覆盖的问题）。
  3. 收到同一群 `convAdded` 时清除快照。
  4. `reason` 透传原值（`removed` / `dissolved` / `left` / 不认识的值），由界面决定文案。
  5. 测试：Vitest——在线移出、离线移出后重连比对、`truncated` 不推断、单聊不推断、`convAdded` 清除、退出登录清除、三种 `reason`。

### FE-206 打开本地没有的会话时请求最新一页（G-UI-26）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-sdk-ts
- 依赖：FE-004、FE-200
- AC：AC-UI-33-10、AC-UI-33-6、AC-UI-33-3、AC-UI-32-4
- 内容：会话不在列表、也没有已加载消息时，`loadHistory` 发 `sync {convId, beforeSeq: Number.MAX_SAFE_INTEGER, limit}`，不按本地 `minSeq` 提前返回（修 `client.ts` 第 222～226 行）；4032 / 4033 / 4043 以带 `code` 的错误交给界面，不吞、不重试；成功时把会话补进列表（同 `refreshConversationsSoon`）；单聊无消息时保持本地临时会话。测试：Vitest 断言发出的 `sync` 帧参数、三种错误码透传、成功后会话出现在列表。

### FE-210 共享测试数据：头像与拼音分组
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server（`protocol/vectors/avatar.json`、`protocol/vectors/pinyin-groups.json`、生成脚本）、tianting-sdk-ts
- 依赖：FE-001
- AC：AC-UI-21-2、AC-UI-13-9
- 内容：
  1. `avatar.json`：脚本按 FNV-1a-32(UTF-8(id)) mod 8 生成 id → 颜色下标，显示名 → 首字（拉丁转大写、emoji 取整个码点），至少 30 条（含中文、emoji、大小写、群 ID）。
  2. `pinyin-groups.json`：名字 → 期望字母，至少含 AC-UI-13-9 列出的 10 条，以及 I / U / V 边界、多音字常见姓氏、全角字符。锚点汉字表随文件提交。
  3. sdk-ts 导出 `avatarColorIndex(id)`、`avatarInitial(name)` 并跑 `avatar.json`；各 SDK 仓库通过 `sync-protocol.sh` 同步。

---

## U1 H5 工程与基础（tianting-web，M3）

### FE-220 tianting-web 工程与测试设施
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-001（FE-IMP-2 推迟的 `web/` 与 Playwright 部分）
- AC：AC-UI-90-15、AC-UI-90-8
- 内容：
  1. React + Vite + TS；路由用 React Router（按 3.2 节路由表一次建好全部路由，M4 页面先占位）；Vite proxy 同源转发 `/demo/login`、`/ws`（2.2 节）。
  2. 构建变量：`VITE_DEMO_LOGIN`、版本号（`package.json`）、构建号；生产构建关闭 source map 中的源码内容。
  3. Playwright：Chromium + WebKit，视口预设（窄屏 375×812、最小 320×568、桌面 1280×800、宽屏）；工具函数 `routeWs(page, rules)`（基于 `page.routeWebSocket` 延迟 / 吞掉 / 改写 ack）、`countWebSockets`、`longPress`（CDP `Input.dispatchTouchEvent`）、剪贴板桩；CI 启动 `IM_TEST_MODE=1 IM_DEMO_LOGIN=1` 的 im-server。
  4. 组件测试：Vitest + Testing Library，SDK 用 `@im-mvp/sdk/testing`（发布名以 sdk-ts 为准）的 `FakeTransport` 注入。
  5. CI 检查：首屏 JS gzip ≤ 200KB（AC-UI-90-8，超出即失败）；品牌词扫描 `src/`、`index.html`、`public/`、`dist/` 不出现"微信 / WeChat / weixin"（AC-UI-90-15）。
  6. `data-testid` 命名约定写入 README（原生端 `testTag` / `accessibilityIdentifier` 同名）。

### FE-221 设计 token、浅色主题与视觉 CI
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-220
- AC：AC-UI-90-3、AC-UI-90-5、AC-UI-22-3、AC-UI-22-4、AC-UI-07-6、AC-UI-90-14
- 内容：
  1. `src/theme/tokens.json`（FE-UI-IMP-1）：6.1 节全部颜色（浅色 + 深色）、6.3 节字号 / 间距 / 圆角 / 尺寸、6.4 节头像调色板；脚本生成 `--tt-*` CSS 变量（B-6）。M3 只输出浅色，`prefers-color-scheme: dark` 下仍为浅色（AC-UI-22-4）。
  2. 对比度 CI（AC-UI-90-3）：脚本内置 6.2 节组合清单 L01～L26、D01～D26、头像调色板 8 色，按 WCAG 2.1 公式计算，低于要求或清单中的 token 名在文件里不存在即失败；同时输出表格与 6.2 节比对（两位小数一致）。
  3. 色相 CI（AC-UI-90-5）：`color.brand*`、`color.bubble.self*` 的 HSL 色相不在 120°～165° 或饱和度 ≤ 40%。
  4. 颜色字面量 lint（AC-UI-22-3）：扫描 `src/`（豁免 token 文件、许可证文件），禁止十六进制 / `rgb()` / `hsl()`；SVG `fill` / `stroke` 只允许 `currentColor` / `none`。
  5. Lucide 图标按 6.6 节清单引入（ISC 许可证文件随代码提交），生成 svg 白名单供 AC-UI-11-3 使用。
  6. `prefers-reduced-motion: reduce` 时除转圈图标外的过渡时长为 0（AC-UI-90-14）。
  7. 测试：CI 脚本单测（故意改坏一个值 / 删一个 token 必须失败）；Playwright 读取计算样式（自己气泡对比度 AC-UI-07-6、深色系统下背景 `#F2F3F7`、减少动态效果）。

### FE-222 应用壳：路由守卫、一级导航、布局容器
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-010、FE-221
- AC：AC-UI-01-1、AC-UI-01-3、AC-UI-01-4、AC-UI-01-5、AC-UI-01-6、AC-UI-01-7、AC-UI-01-8、AC-UI-02-5、AC-UI-02-6、AC-UI-02-8
- 内容：
  1. 底部标签栏（消息 / 通讯录 / 我，高 56 + `env(safe-area-inset-bottom)`），选中态 `color.brand`；二级页面隐藏标签栏。
  2. 守卫：已登录访问 `/login` → `/chats`；未登录 → `/login?redirect=…`；回跳地址按 F-UI-01 规则 3 校验并必须匹配路由表，否则 `/chats`（单测覆盖 AC-UI-01-7 的四种恶意值）。
  3. 会话列表滚动位置在返回时恢复（误差 ≤ 72px）；点击已选中的"消息"平滑滚动到顶部。
  4. 容器 `min(100vw, 480px)` 居中、两侧 1px `color.divider`、外部 `color.bg`；320 宽无横向滚动。
  5. 主区组件（聊天页、资料卡、设置内容）与页面外壳分离，M4 复用（F-UI-02 规则 3）。
  6. 测试：Playwright（路由、回跳、滚动恢复、320 宽 `scrollWidth`、1280 宽容器几何）；（手工）iPhone 15 Safari 安全区截图（AC-UI-02-8）。

### FE-223 登录页补充：昵称、生产构建、错误与防重
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-010、FE-222；演示登录 `nickname`（G-UI-23，PRD-ui 8.2 节，后端任务）
- AC：AC-UI-03-1、AC-UI-03-2、AC-UI-03-3、AC-UI-03-4、AC-UI-03-5、AC-UI-03-6、AC-UI-03-7、AC-UI-03-8、AC-UI-03-10
- 内容：Logo 与"天听"、`document.title = 天听`；用户名去空白为空禁用、回车提交、加载态防重；昵称 0～32 码点（`countCodePoints`），超出提示"昵称最多 32 个字"；404 →"登录服务未开启"、网络错误 / 10 秒超时 →"网络异常，请稍后重试"；`VITE_DEMO_LOGIN=false` 只显示"请从业务方入口进入天听"；输入框字号 16px。测试：Playwright（`page.route` 拦截 `/demo/login`）、组件测试（昵称计数）；端到端 AC-UI-03-7 在真实 im-server 上跑。

### FE-224 通用组件：头像、显示名、轻提示、确认框、骨架屏、官方标识
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-210、FE-221、FE-019；frontend-social.md 的 `users` 资料缓存
- AC：AC-UI-21-1、AC-UI-21-3、AC-UI-21-4、AC-UI-21-5
- 内容：
  1. `<Avatar id name src size shape>`：占位首字 + 8 色（`avatarColorIndex`），用户圆形、群圆角 10；图片只加载 `https://`，`referrerpolicy="no-referrer"`、`loading="lazy"`，失败回落占位。
  2. `useDisplayName(userId)`：备注 > 昵称（单聊 `conv.name` / 同步 `profile` / `users` 缓存）> `userId`（2.1 节"显示名"）。
  3. `<Toast>`（2 秒、同时 1 条、新替换旧）、`<ConfirmDialog>`、`<BottomSheet>`、`<Skeleton>`、`<OfficialBadge>`（`color.brand` 字、`color.surface.selected` 底、圆角 4）。
  4. 测试：组件测试跑 `avatar.json`、http / 空串 / 404 回落；Playwright 断言 `<img>` 属性与"http 头像不发请求"。

### FE-225 错误文案按"命令 + 错误码"映射
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-016、FE-224
- AC：AC-UI-19-1、AC-UI-19-2、AC-UI-19-3、AC-UI-19-4、AC-UI-19-5
- 内容：FE-016 的文案表改为先查"命令 + 码"再查码默认值（F-UI-19 规则 2～4、PRD-social 9.4 节、FE-UI-IMP-2）；补 4001、4000 / 4002 / 4003 / 4007 / 4008（只写日志 + "请求出错"）、4005、4030、4040、4045、4091、4132～4134、4292；从不展示服务端 `msg`。单测遍历 `errors.json` 全部 ws 码都有非空文案、遍历 `vectors/social.json` 文案用例逐条相等（AC-UI-19-3，也是 X-10 的 CI 守卫）。Playwright：两个错误连发只留 1 条轻提示、2 秒消失。

---

## U2 H5 基础页面（tianting-web，M3）

### FE-226 会话列表补充：摘要规则、截断提示、骨架屏
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-011、FE-203、FE-224
- AC：AC-UI-04-1、AC-UI-04-2、AC-UI-04-3、AC-UI-04-4、AC-UI-04-5、AC-UI-04-6、AC-UI-04-7、AC-UI-04-9、AC-UI-20-1
- 内容：会话项 72 高、头像 48、单行省略；摘要规则（群聊他人消息加"显示名: "且随备注变化、`lastMsg=null` 不显示摘要和时间、`custom` →"[暂不支持的消息]"）；角标在头像右上、>99 →"99+"；`truncated` 时底部"仅显示最近 500 个会话"；`convs` 未返回时 6 条骨架屏，空列表显示"还没有聊天"+"发起单聊"。会话项 `React.memo` 以 `convId` 为 key（配合 FE-203）。M4 的 `tip` / 草稿摘要在 FE-304、FE-311，第二批"该消息已被删除"在 FE-351。测试：组件测试（摘要规则表）、Playwright（`routeWs` 延迟 `convs`、角标几何）。

### FE-227 "+"菜单与发起单聊补充
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-012、FE-226；frontend-social.md 的 `socialSettings`、`relationOf`
- AC：AC-UI-05-1、AC-UI-05-2、AC-UI-05-3、AC-UI-05-5、AC-UI-05-6、AC-UI-05-7
- 内容："+"菜单（M3：添加朋友、发起单聊）；弹窗单例（同一 `data-testid`），空输入禁用、回车提交、点遮罩关闭；输入自己允许；好友制且非好友时弹窗内提示"对方不是你的好友，可能无法发送消息"及"添加朋友"（来源 `other`）/"仍然发起"。测试：组件测试 + Playwright（两种模式用 `routeWs` 改写 `socialSync` 应答头的 `mode`）。

### FE-228 聊天页结构与聊天信息页（只读）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-013、FE-204、FE-224
- AC：AC-UI-06-1、AC-UI-06-2、AC-UI-06-3、AC-UI-06-4、AC-UI-06-6、AC-UI-06-7、AC-UI-12-1、AC-UI-12-2、AC-UI-12-3
- 内容：
  1. 顶栏 48：返回、标题（单聊显示名 + 官方标识；群聊"群名（N）"，群名省略、"（N）"始终完整，`memberCount` 缺失只显示群名）、"更多"。
  2. 进入群聊天页 / 信息页时 `refreshConversations()`（M3 方案，M4 由 FE-307 替换）。
  3. `setActiveConversation`：只有聊天页实际可见时设置，进入信息页 / 资料卡设为 `null`，返回时恢复。
  4. 进入时贴底；新消息胶囊样式（`color.brand.fill`，输入区上方 12px）。
  5. 聊天信息页 `/chat/:convId/info`：单聊头像 64、显示名、"账号：{userId}"，点头部进入资料卡；群聊群头像、群名、"群成员（N）"，没有任何操作入口和成员网格。
  6. 测试：Playwright（标题省略、`routeWs` 去掉 `memberCount`、100 条消息贴底、未读计数）；组件测试（注入 SDK 桩记录 `setActiveConversation` 调用顺序）；端到端 AC-UI-06-6。

### FE-229 消息气泡与时间分隔
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-014、FE-224
- AC：AC-UI-07-1、AC-UI-07-2、AC-UI-07-3、AC-UI-07-4、AC-UI-07-5、AC-UI-07-7
- 内容：左右气泡、头像 40、间距 8、列表内边距 12（距边 60px）；群聊他人消息上方显示发送者显示名（`users` 缓存未就绪先显示 `userId`）；最大宽度 70%；`custom` / 未知类型 →"[暂不支持的消息]"；纯文本、不识别链接；窄屏 `user-select: none`、`-webkit-touch-callout: none`；点头像进入资料卡。测试：Playwright（几何测量、XSS 文本、`custom` 帧无 console error）。

### FE-230 发送状态与失败重发确认
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-202、FE-229、FE-225
- AC：AC-UI-08-1、AC-UI-08-2、AC-UI-08-3、AC-UI-08-4、AC-UI-08-5
- 内容：状态图标在气泡靠中线一侧（转圈 / 无 / "!" 44×44 点击区）；`data-status` 暴露 SDK 状态；点"!"弹"重发该消息？"，"重发"调 `retry`；失败轻提示只由 `sendFailed` 触发（4035 / 4036 / 4137 交给 FE-243 的提示条，`timeout` / `kicked` 不提示）。测试：Playwright（`routeWs` 延迟 3 秒 / 返回 4290 / 不返回）、组件测试（重渲染不重复提示）。

### FE-231 消息操作菜单（M3：复制、重发）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-229、FE-230
- AC：AC-UI-09-1、AC-UI-09-3、AC-UI-09-4、AC-UI-09-6、AC-UI-09-7、AC-UI-09-8
- 内容：Pointer Events 实现长按 500ms、移动 >10px 取消；菜单优先在气泡上方，空间不足放下方且完全在视口内；菜单项按状态（复制：全部文本消息；重发：`failed`，不再确认）；`custom` 无菜单；复制写剪贴板，成功"已复制"、失败"复制失败，请长按文字手动复制"。M4 右键与"删除"在 FE-305。测试：组件测试（假定时器的 pointer 序列）、Playwright（CDP 长按、Chromium 读剪贴板 / WebKit 桩）。

### FE-232 输入区补充：自动增高、超长、输入法组字
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-013、FE-014
- AC：AC-UI-10-2、AC-UI-10-3、AC-UI-10-4、AC-UI-10-6、AC-UI-10-7、AC-UI-10-8
- 内容：窄屏多行自动增高最多 5 行；超过 5000 码点显示"已超出 N 字"（`color.danger`）并禁用发送（V-UI-07，"消息过长"只用于 4006）；`enterkeyhint="send"`；`isComposing` / `keyCode 229` 不发送，`compositionend` 后 50ms 内的 Enter 忽略；`send` 返回 `ok:false` 不清空；字号 16px。测试：组件测试（合成键盘事件序列、`FakeTransport` 返回 tooLong）、Playwright（粘贴 5003 码点）；（手工）macOS / iOS Safari 拼音输入（AC-UI-10-7）。

### FE-234 未知会话、被移出 / 解散的会话（F-UI-33，M3 部分）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-200、FE-205、FE-206、FE-228、FE-232
- AC：AC-UI-33-1、AC-UI-33-2、AC-UI-33-3、AC-UI-33-4、AC-UI-33-5、AC-UI-33-6、AC-UI-33-9、AC-UI-33-10、AC-UI-33-11、AC-UI-33-12、AC-UI-33-14、AC-UI-10-5、AC-UI-12-5
- 内容：
  1. 路由进入聊天页先 `parseConvIdStrict`：`invalid` / `notParticipant` → `/chats` + 轻提示"会话不存在"（不发请求）。
  2. 单聊不在列表：临时会话 + `loadHistory`（G-UI-26），有历史则补进列表；4032 按格式非法处理。
  3. 群聊不在列表且无快照：`sync` 返回前只显示加载状态（无输入框、无"不在该群"）；4033 → 标题"群聊"+"你已不在该群"；4043 → 标题"群聊"+"群聊不存在或已解散"（列表居中 + 禁用条）。
  4. 有快照（`removed: true`）：标题保留快照群名、不带成员数；按 `reason` 映射底部提示和禁用条（缺失 / `removed` / 不认识 →"你已不在该群" /"你已不在该群，无法发送消息"；`dissolved` →"群聊已解散"）；M3 构建收到 `left` 按不认识处理（M4 文案在 FE-311）。
  5. 停在该群时收到 `convAdded`：快照清除、输入区恢复；被移出时已输入的内容不保留。
  6. 聊天信息页在快照状态显示群名和"你已不在该群"。
  7. 测试：组件测试（`FakeTransport` 回放 4033 / 4043 / 三种 `reason`、断言 `sync` 帧和 4033 前的 DOM）；端到端（管理接口移出 / 重新拉入 / 解散、断网期间移出后重连、刷新后打开）——需要 PRD-core 群管理接口、PRD-admin 第一批 `DELETE /admin/groups/{id}` 与 `convRemoved.reason`。

### FE-235 连接状态条、被踢弹窗、未读总数与页面标题
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-011、FE-201、FE-224
- AC：AC-UI-17-1、AC-UI-18-1、AC-UI-18-2、AC-UI-18-3、AC-UI-23-1、AC-UI-23-2、AC-UI-23-3、AC-UI-23-4
- 内容：
  1. 状态条 36 高、`color.warning.*`，订阅 SDK `banner` 事件，出现时下推内容（会话列表、聊天页、通讯录都显示）。
  2. `fatal` 弹窗：只有"确定"，Esc / 遮罩 / 浏览器后退都不关闭（后退时 `pushState` 回原路由）；按 F-UI-18 文案表与关闭码兜底；"确定"后回登录页并确认 SDK 社交状态已清空。
  3. "消息"标签角标 = `totalUnread()`（>99 "99+"）；`document.title` = `({n}) 天听` / `天听`；"通讯录"角标 = `friendRequests.unseen`。
  4. 测试：Playwright（`context.setOffline`、`routeWs` 推 `kicked`、`document.title`）；端到端 AC-UI-23-4（两个标签页，`?platform=pc`，C-44）——需要 PRD-social 服务端红点多端同步。

### FE-237 "我"与设置
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-201、FE-224、FE-222
- AC：AC-UI-15-1、AC-UI-15-2、AC-UI-15-3、AC-UI-16-1、AC-UI-16-2、AC-UI-16-3、AC-UI-16-4、AC-UI-16-6
- 内容："我"页头像 64、显示名（`users` 缓存中的自己）、"账号：{userId}"+"复制"、官方标识，无编辑入口；设置页顺序"隐私、黑名单（人数）、关于天听（版本号 / 构建号 / 开源许可）、退出登录"；退出确认框，`pendingCount() > 0` 时正文"有 N 条消息未发送，退出后将丢失"；确认后执行 F-AUTH-05，`ui.*` 保留。原 PRD-core 会话列表右上角菜单移除（V-UI-01）。测试：Playwright（剪贴板、`sessionStorage`、版本号比对 `package.json`、取消后 2 秒内无 close）；端到端 AC-UI-15-2（演示登录昵称）。

---

## U3 H5 社交页面（tianting-web，M3）

> 数据和规则以 PRD-social 为准，SDK 接口（`friends`、`friendRequests`、`blocks`、`socialSettings`、`relationOf`、`applyFriend` 等）由 frontend-social.md 的 sdk-ts 社交任务提供；端到端 AC 需要 PRD-social 服务端 M3 功能。

### FE-238 通讯录：入口、好友列表、拼音分组
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-210、FE-224、FE-235；frontend-social.md 的 sdk-ts 社交同步
- AC：AC-UI-13-1、AC-UI-13-2、AC-UI-13-3、AC-UI-13-4、AC-UI-13-5、AC-UI-13-6、AC-UI-13-7、AC-UI-13-8、AC-UI-13-9、AC-UI-13-10、AC-UI-13-11、AC-UI-20-2
- 内容：顶部固定"新的朋友"（红点 = `unseen`）、"群聊"入口；好友列表（头像 40、显示名、官方、"已拉黑"标签）、底部"{N} 位朋友"；分组：`Intl.Collator('zh-Hans-CN-u-co-pinyin')` + 锚点汉字表，拉丁取大写，其他归"#"并排最后；不支持拼音时汉字归"#"且不报错；组内 Collator 比较，相同按 `userId` 字节序；首轮 `socialSync` 未结束时 6 条骨架屏；无好友时"还没有朋友"+"添加朋友"。不引入拼音库、不做虚拟列表。测试：组件测试跑 `pinyin-groups.json`（Chromium + WebKit 各一次，Playwright 组件模式）、`Intl.Collator` 桩；包体积 CI（AC-UI-13-11 与 AC-UI-90-8 共用）；端到端 AC-UI-13-4。

### FE-239 资料卡
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-224、FE-238
- AC：AC-UI-14-1、AC-UI-14-2、AC-UI-14-3、AC-UI-14-4、AC-UI-14-5、AC-UI-14-6、AC-UI-14-7、AC-UI-14-8、AC-UI-14-9、AC-UI-14-10、AC-UI-14-11
- 内容：头部（头像 64、显示名 22、"昵称：…"、"账号：…"、官方）；资料来源顺序（`profile` → `users` → 路由状态 → 只显示 `userId`），不可见用户与不存在用户 DOM 相同、不显示"用户不存在"；按 `relationOf` 的操作区表（F-UI-14 规则 3）；拉黑 / 删除确认框文案（好友制追加一句）；删除后停留并变为 `none`；申请来源 `group`（带 `sourceRef`）/ `conv` / `search` / `card` 通过路由状态传给 FE-241。测试：组件测试（六种 relation × 两种模式的按钮矩阵、DOM 等价比较）、Playwright（来源参数断言 `friendApply` 帧）、端到端 AC-UI-14-4 / 5 / 9。

### FE-240 新的朋友与列表空状态
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-224、FE-225、FE-238
- AC：AC-UI-25-1、AC-UI-25-2、AC-UI-25-3、AC-UI-25-4、AC-UI-25-5、AC-UI-25-6、AC-UI-25-7、AC-UI-25-8、AC-UI-20-3
- 内容：列表按 `updatedAt` 倒序；显示名（`in` 用 `profile`，`out` 用资料缓存，否则 `userId`）、第二行规则、右侧状态表；"同意"加载态；点 `in+pending` 行弹申请详情底部菜单（来源文案、同意 / 拒绝 / 查看资料）；进入页面调 `markFriendRequestsSeen()`，停留期间新申请 200ms 去抖再次调用；4045 →"该申请已过期"并触发同步。空状态："暂无新的朋友"、"黑名单为空"、"还没有加入群聊"（后两者在 FE-242、FE-244 使用同一组件）。测试：组件测试（`FakeTransport` 回放同步页、过期按服务端时间偏移）、Playwright（`routeWs` 返回 4045）、端到端 AC-UI-25-2 / 3。

### FE-241 添加朋友（搜索）与申请添加朋友
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-224、FE-225、FE-239
- AC：AC-UI-26-1、AC-UI-26-2、AC-UI-26-3、AC-UI-26-4、AC-UI-26-5、AC-UI-26-6、AC-UI-26-7、AC-UI-27-1、AC-UI-27-2、AC-UI-27-3、AC-UI-27-4、AC-UI-27-5、AC-UI-27-6、AC-UI-27-7、AC-UI-27-8
- 内容：
  1. `/contacts/add`：搜索框（`enterkeyhint="search"`、16px）、"我的账号"+复制；本地 userId 校验不合法不发请求；加载期间重复提交只发 1 次；`null` →"未找到该用户"（三种原因 DOM 完全相同）；结果卡片按 relation 显示操作，资料通过路由状态带给资料卡。
  2. `/contacts/apply/:userId`：附言默认"我是{我的显示名}"、0～50 码点计数、备注 0～32；超长提示并禁用；来源取路由状态，直接打开为 `other`；`pending` →"已发送"并返回，`accepted` →"已添加到通讯录"并替换为资料卡；防枚举（`pending` 表现一致）；打开自己 →"不能添加自己"返回；4000 →"请先将对方移出黑名单"。
  3. 测试：组件测试（relation 表、计数器）、Playwright（`routeWs` 拦截 `userSearch` / `friendApply`、DOM 等价）、端到端 AC-UI-26-1 / 2、AC-UI-27-2 / 4 / 5。

### FE-242 设置备注、黑名单、隐私设置
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-239、FE-240、FE-237
- AC：AC-UI-28-1、AC-UI-28-2、AC-UI-28-3、AC-UI-28-4、AC-UI-28-5、AC-UI-29-1、AC-UI-29-2、AC-UI-29-3、AC-UI-30-1、AC-UI-30-2、AC-UI-30-3
- 内容：备注页（非好友进入返回资料卡、预填、空白即清除、未变化不发请求、超 32 禁用、4040 →"对方已不在你的通讯录中"）；黑名单页（`createdAt` 倒序、`profile=null` 显示 `userId` + 占位、"移出"不确认、"已移出黑名单"、点行进资料卡）；隐私页（开关 + 说明文案、请求期间禁用、失败回滚并提示）。测试：组件测试 + Playwright（`routeWs` 4040 / 4290 / 不返回）；端到端 AC-UI-28-1、AC-UI-29-1 / 2、AC-UI-30-1（多标签页同步需要 PRD-social 服务端多端同步）。

### FE-243 私聊权限提示条
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-202、FE-230、FE-241
- AC：AC-UI-31-1、AC-UI-31-2、AC-UI-31-3、AC-UI-31-4、AC-UI-31-5、AC-UI-31-6
- 内容：订阅 `sendFailed`：4035 在失败消息下插入"对方开启了好友验证，你还不是他的好友"+链接"发送好友申请"（来源 `conv`）；4036 "消息已发出，但被对方拒收了"；4137 "对方回复之前，最多只能发送 N 条消息"（N 取 `socialSettings.strangerMsgLimit`）；4292 只弹轻提示；相邻相同提示条只保留最后一条；提示条只在内存（刷新消失）、不计未读、不可长按。测试：组件测试（连续 3 次 4035）、Playwright（`routeWs` 4292、刷新）；端到端 AC-UI-31-1～3（好友制 / 拉黑 / 陌生人限额，需要 PRD-social 服务端 F-SOC-08）。

### FE-244 群聊列表
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-224、FE-238、FE-206；frontend-social.md 的 `loadMyGroups`
- AC：AC-UI-32-1、AC-UI-32-2、AC-UI-32-3、AC-UI-32-4
- 内容：每次进入调用 `loadMyGroups()`；骨架屏 / "加载失败，点击重试"；群头像 40、群名、"{memberCount} 人"；按 Collator 排序（同名按 `groupId`）、不分组；顶部"{N} 个群聊"；点击进入 `/chat/{groupConvId}`（不在 `convs` 时由 FE-234 / FE-206 处理）。测试：组件测试（排序）、Playwright（`routeWs` 返回 5000 后重试）；端到端 AC-UI-32-2 / 4（600 个群造数据）。

---

## U4 非功能与回归（tianting-web，M3）

### FE-245 可访问性：按钮名称、点击区域、`aria-live`、axe 扫描
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-226～FE-244
- AC：AC-UI-90-4、AC-UI-90-11
- 内容：所有图标按钮加 `aria-label`（NF-UI-7）；关键按钮 ≥ 44×44（NF-UI-8，组件测试）；当前聊天页 `aria-live="polite"`：单聊逐条"{发送者}：{内容}"，群聊 3 秒内合并"收到 N 条新消息"；Playwright + `@axe-core/playwright` 扫描登录、会话列表、聊天页、通讯录、新的朋友、资料卡、设置，`color-contrast` 与 `button-name` 零违规（M4 的深色扫描在 FE-302）；（手工）VoiceOver 抽查记录。

### FE-246 安全与隐私：纯文本渲染、存储、CSP、日志
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-220、FE-226～FE-244
- AC：AC-UI-90-1、AC-UI-90-2、AC-UI-90-12、AC-UI-90-13
- 内容：ESLint 禁止 `dangerouslySetInnerHTML` / `innerHTML`（NF-UI-12）；XSS 全页面扫描（昵称、群名、备注、附言、摘要、标题）；`localStorage` 只允许 `ui.*`（封装 `uiPrefs` 模块，lint 禁止直接调用 `localStorage`）；生产构建 logger 不输出消息、附言、备注、token；生产部署配置 CSP（`script-src 'self'`、`connect-src` 同源、`img-src 'self' https: data:`、`frame-ancestors 'none'`）——Vite preview 与部署 nginx 示例各一份。测试：Playwright（生产构建，监听 `console`、`securitypolicyviolation`、`page.on('request')`）。

### FE-247 性能：会话列表 500、单项重渲染、首屏体积
- 优先级：P1
- 里程碑：M3
- 仓库：tianting-web
- 依赖：FE-203、FE-226、FE-220
- AC：AC-UI-90-6、AC-UI-90-7、AC-UI-90-8
- 内容：`performance.mark` 在 `convs` ack 到达与最后一项 `useLayoutEffect` 打点（`convs-render`），Chrome CPU 4 倍降速、3 次中位数 ≤ 500ms，`longtask` 无 >100ms；React Profiler 统计收到 1 条消息时重渲染会话项 ≤ 2；首屏 JS gzip ≤ 200KB（与 FE-220 的 CI 共用脚本）。NF-UI-2（1000 条消息）同一套脚本记录基线。不达标时按 NF-UI-5 另开任务评估虚拟列表。

### FE-248 M3 手工验收与已有用例步骤更新
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-web、tianting-server（`docs/testcases`）
- 依赖：FE-222、FE-232
- AC：AC-UI-02-8、AC-UI-10-7
- 内容：按 PRD-ui 8.1 节更新受影响用例的步骤（TC-MSG-03-05/07、TC-AUTH-05-01～04、TC-CONV-03-01～04、TC-CONV-01-09、TC-MSG-01-03/04、TC-GRP 发送者断言）；iPhone 15 Safari 安全区截图、macOS / iOS Safari 拼音输入录屏，记录到测试报告；M3 演示路径彩排一次。

---

## U5 宽屏、深色、表情、草稿（M4）

### FE-300 宽屏三栏布局与实时切换
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-222、FE-228
- AC：AC-UI-01-2、AC-UI-02-1、AC-UI-02-2、AC-UI-02-3、AC-UI-02-4、AC-UI-02-7、AC-UI-04-8、AC-UI-06-5、AC-UI-17-2
- 内容：媒体查询 `(min-width:768px) and (min-height:500px) and (hover:hover) and (pointer:fine)`；左侧导航栏 64（头像 36、三个导航项、底部"设置"）、列表栏 280、主区 ≥ 424；主区空态 Logo +"选择一个会话开始聊天"；当前会话高亮 `color.surface.selected`；聊天页无返回、输入框自动聚焦；全局只有一条横跨列表栏和主区的状态条；视口跨过条件时实时切换，不改路由、不重连、输入框内容不丢（同一组件实例或草稿状态上提）；宽屏 `setActiveConversation` 规则（F-UI-06 规则 8）。AC-UI-02-6 改用 768×1024 `hover: none` 视口回归。测试：Playwright（`page.setViewportSize` + `emulateMedia`、`countWebSockets`）。

### FE-301 宽屏侧板、弹窗键盘操作与缩放
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-300
- AC：AC-UI-05-4、AC-UI-90-9、AC-UI-90-10
- 内容：聊天信息 / 成员 / 资料卡在宽屏为右侧侧板（宽 360，遮罩或 Esc 关闭）；弹窗 Esc 关闭并把焦点还给触发按钮（焦点陷阱）；Tab 顺序：导航栏 → 列表 → 主区输入框，Enter 打开会话；`:focus-visible` outline ≥ 2px；640×400 视口无横向滚动。测试：Playwright 键盘脚本。

### FE-302 深色模式与外观设置
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-221、FE-237、FE-246
- AC：AC-UI-22-1、AC-UI-22-2、AC-UI-22-5、AC-UI-16-5
- 内容：`/me/settings/appearance`（跟随系统 / 浅色 / 深色），偏好存 `localStorage` `ui.theme`；同源同步脚本 `/theme-init.js` 在 `<head>` 设置 `<html data-theme>`（CSP 不放宽）；跟随系统时监听 `matchMedia` 实时切换；退出登录保留 `ui.*`；设置页插入"外观"项（AC-UI-16-6 在 M4 构建中顺序变为"隐私、黑名单、外观、关于天听、退出登录"）；axe 深色再扫一遍（AC-UI-90-4 的 M4 部分）。测试：Playwright（`emulateMedia({colorScheme})`、生产构建 + CSP 头、`DOMContentLoaded` 时读取背景色）。

### FE-303 表情面板（系统 emoji）
- 优先级：P1
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-232
- AC：AC-UI-11-1、AC-UI-11-2、AC-UI-11-3、AC-UI-11-4
- 内容：输入区左侧表情按钮（`smile`）；窄屏替代键盘位置、宽屏浮层；固定列表 ≥ 80 个（≤ U+1FAFF、无 ZWJ），点击插入光标位置并把光标移到其后；系统字体渲染；构建产物无 >1KB 位图、svg 只在 6.6 节白名单内（CI）。V-UI-06 可被 Frank 否决，否决时整任务取消。测试：组件测试（光标插入、列表码点校验）、CI 脚本；端到端 AC-UI-11-2。

### FE-304 草稿
- 优先级：P1
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-226、FE-232、FE-300
- AC：AC-UI-24-1、AC-UI-24-2、AC-UI-24-3、AC-UI-24-4
- 内容：草稿存内存（按 `convId`，D4），离开聊天页 / 宽屏切换会话时保存、回来恢复；会话列表摘要"[草稿] …"（"[草稿]"用 `color.danger`），不影响排序；清空后离开删除草稿；退出登录清空。测试：组件测试 + Playwright。

### FE-305 消息菜单（M4）：右键、删除失败消息、宽屏 Enter 发送
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web、tianting-sdk-ts
- 依赖：FE-231、FE-232、FE-300
- AC：AC-UI-09-2、AC-UI-09-5、AC-UI-10-1
- 内容：sdk-ts 新增 `removeFailed(convId, clientMsgId)`（G-UI-14）：从本地移除失败消息并更新 `pendingCount`；宽屏右键打开菜单并 `preventDefault`，`custom` / `tip` / 已删除且非失败的消息右键也不出现浏览器菜单；失败消息菜单增加"删除"；宽屏输入区固定 120px，Enter 发送、Shift+Enter 换行，发送后保持焦点。测试：sdk-ts Vitest（`removeFailed`）、Playwright（右键、`pendingCount` 变化）。

---

## U6 客户端群管理（tianting-web，M4）

> 权限矩阵和提示消息规则以 PRD-social 第 7 节为准；SDK 群 API（`createGroup`、`getGroupInfo`、`loadGroupMembers`、`groupChanged`、`tip`、`convRemoved.reason = left` 等）在 frontend-social.md 的 M4 sdk-ts 任务；端到端 AC 需要 PRD-social M4 服务端。

### FE-306 发起群聊
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-238、FE-227；frontend-social.md 的 `createGroup`
- AC：AC-UI-05-8、AC-UI-35-1、AC-UI-35-2、AC-UI-35-3、AC-UI-35-4、AC-UI-35-5
- 内容："+"菜单最前面增加"发起群聊"；`/groups/new`：本地过滤好友（显示名或 `userId`，不区分大小写）、分组同通讯录、复选框、顶部已选头像、"完成（N）"至少 2 人；底部菜单填可选群名（1～50 码点）；成功进入新群；错误：4035 标红该人并提示、4135、4130、4292、4030 →"当前不允许创建群聊"。测试：组件测试 + Playwright（`routeWs` 4030 / 4035）；端到端 AC-UI-35-1 / 4。

### FE-307 群设置（按角色显示）与成员数实时更新
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-228、FE-301；frontend-social.md 的 `getGroupInfo`、`groupChanged`
- AC：AC-UI-36-1、AC-UI-36-2、AC-UI-36-3、AC-UI-36-4、AC-UI-36-5、AC-UI-36-6、AC-UI-36-7、AC-UI-36-8、AC-UI-06-8
- 内容：`/chat/:convId/info` 群聊改为群设置（AC-UI-12-3 在 M4 作废）：成员网格前 20 + "查看全部成员（N）"、"+"（托管群无群主时不显示）、"−"（owner / admin）、群聊名称（owner / admin 可改）、群公告前 2 行、群管理（owner）、"退出群聊"（群主正文不同）、"解散群聊"（自建群 owner）；4030 →"无权限"并重新 `getGroupInfo`；聊天页标题成员数改由 `groupChanged` + `groupInfo` 更新，删除 M3 进入页面时的 `refreshConversations`。测试：组件测试（`myRole` × `managed` × 是否有群主的入口矩阵）、Playwright（`routeWs` `groupUpdate` 4030）；端到端 AC-UI-36-5～7、AC-UI-06-8。

### FE-308 群公告
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-307
- AC：AC-UI-37-1、AC-UI-37-2、AC-UI-37-3、AC-UI-37-4
- 内容：`/chat/:convId/notice` 全文（纯文本、保留换行）、发布者显示名与时间、"暂无群公告"；owner / admin 编辑态（0～1000 码点，"已输入 N / 1000"，超出 `color.danger` 并禁用"完成"，空串清除）；成员显示"仅群主和管理员可编辑"。测试：组件测试 + Playwright（XSS）；端到端 AC-UI-37-1。

### FE-309 群成员列表与管理模式
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-307；frontend-social.md 的 `loadGroupMembers`、`kickFromGroup`、`setGroupAdmin`、`transferGroupOwner`
- AC：AC-UI-38-1、AC-UI-38-2、AC-UI-38-3、AC-UI-38-4、AC-UI-38-5、AC-UI-38-6、AC-UI-38-7
- 内容：分页 50、距底 5 项加载下一页；顺序以服务端为准、只按 `userId` 去重，不排序不过滤、无搜索框；角色标签、官方标识、点击进资料卡（来源 `group`）；管理模式：移出（owner 选管理员和成员、admin 只选成员，≤ 50，确认框）、设置管理员（4136 →"管理员最多 10 人"）、转让群主（单选 + 确认框）；4046 →"对方已不在群里"并刷新。测试：组件测试（可选性矩阵）、Playwright（`routeWs` 4136）；端到端 AC-UI-38-1 / 2 / 4 / 5（200 人群造数据）。

### FE-310 邀请成员
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-306、FE-309
- AC：AC-UI-39-1、AC-UI-39-2、AC-UI-39-3、AC-UI-39-4
- 内容：`/chat/:convId/invite` 好友多选（复用 FE-306 选择组件）；已加载成员显示"已在群里"不可选；第 51 人提示"一次最多邀请 50 人"且不勾选；`{added, failed}` 处理（"已邀请 N 人"、失败逐条最多 3 条 + "另有 M 人邀请失败"、全部失败停留）；4130 →"群成员已达上限"。测试：组件测试；端到端 AC-UI-39-1 / 2。

### FE-311 群提示消息与退群 / 解散提示（M4 部分）
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-web
- 依赖：FE-229、FE-234、FE-226；frontend-social.md 的 `tip` 消息、`convRemoved.reason = left`
- AC：AC-UI-40-1、AC-UI-40-2、AC-UI-40-3、AC-UI-40-4、AC-UI-40-5、AC-UI-40-6、AC-UI-33-7、AC-UI-33-8
- 内容：`tip` 渲染为居中胶囊（12 号、`color.text.tertiary`、`color.surface` 底），无头像 / 气泡 / 菜单，参与时间分隔、计入未读；按 F-UI-40 文案表渲染（"你"、本机备注、targets 最多 3 个 +"等 N 人"、不认识的 kind →"[群通知]"，`noticeUpdated` 可点进公告页）；会话列表摘要用 `lastMsg.text` 不加前缀；F-UI-33 `reason = left` →"你已退出该群聊"，客户端解散同样显示"群聊已解散"。测试：组件测试（文案表逐行）；端到端 AC-UI-33-7 / 8（需要 PRD-social M4 `groupDissolve`、`groupLeave` 与多端 `convRemoved`）。

---

## U7 PC（tianting-desktop，M4）

### FE-312 Tauri 壳：窗口、右键、开发者工具、capabilities
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-desktop
- 依赖：FE-300、FE-305、FE-246
- AC：AC-UI-34-1、AC-UI-34-2、AC-UI-34-3、AC-UI-34-4、AC-UI-34-5、AC-UI-34-7
- 内容：Tauri 2 打包 tianting-web 生产构建（只加载本地资源，`connect-src` 指向 IM 服务）；单窗口、最小 800×560、标题与安装包名"天听"；生产构建禁用默认右键菜单和开发者工具；`src-tauri/capabilities/*.json` 只保留需要的 `core:window:*`，CI 白名单校验（无 `shell:*`、`fs:*`、`http:*`、`process:*`）；Cmd/Ctrl+W 关闭窗口；token 沿用 `sessionStorage`（重启回登录页）。测试：CI 白名单脚本；Playwright 宽屏用例在 WebView（tauri-driver / WebDriver）回归；端到端 AC-UI-34-5 在 macOS 与 Windows 各装一次与 H5 互发消息。

### FE-313 PC 手工验收
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-desktop
- 依赖：FE-312
- AC：AC-UI-34-6
- 内容：macOS（WKWebView）与 Windows（WebView2）中用系统拼音输入法验证组字回车不发送（AC-UI-10-7 在 Tauri 中重复）；记录 800×560 截图、右键与开发者工具快捷键结果；整理 PC 安装说明。

---

## U8 第二批（PRD-admin 带来的客户端提示）

> 与 PRD-admin 第二批服务端（封禁、禁言、删除、推送）、协议（PRD-core v1.3 第二批条目）一起交付；错误码和字段以 errors.json / 协议定稿为准。

### FE-208 sdk-ts 第二批事件与错误（G-UI-25）
- 优先级：P0
- 里程碑：第二批
- 仓库：tianting-sdk-ts
- 依赖：PRD-admin 第二批协议（`muted` / `unmuted`、4031、`msgDeleted`、`deleted` 标记、`kicked{reason:"suspended", until}`、`push`）；FE-004、FE-005
- AC：AC-UI-41-4、AC-UI-42-1、AC-UI-42-3、AC-UI-43-4、AC-UI-44-2、AC-PUSH-01-1、AC-PUSH-01-3、AC-PUSH-01-4、AC-PUSH-01-5、AC-PUSH-03-4、AC-PUSH-03-7、AC-PUSH-03-9、AC-PUSH-03-10、AC-PUSH-03-11、AC-PUSH-03-15、AC-PUSH-03-17、AC-PUSH-03-18、AC-PUSH-03-19、AC-PUSH-03-20、AC-PUSH-03-24、AC-PUSH-03-25、AC-PUSH-03-26、AC-PUSH-03-27、AC-PUSH-03-28、AC-PUSH-03-29、AC-PUSH-03-30、AC-PUSH-03-31、AC-PUSH-03-32、AC-PUSH-03-33、AC-PUSH-03-35、AC-PUSH-06-5、AC-PUSH-06-18、AC-PUSH-07-1、AC-PUSH-07-2、AC-PUSH-07-3、AC-PUSH-07-5、AC-PUSH-07-6、AC-PUSH-08-1、AC-PUSH-08-2、AC-PUSH-08-3、AC-PUSH-08-4、AC-PUSH-08-5、AC-PUSH-08-6、AC-PUSH-08-8、AC-PUSH-08-11
- 内容：`muted` / `unmuted` 事件与禁言状态 `muteStateOf(convId)`（按服务端时间到期自动解除并发事件）；4031 带 `mutedUntil` / `scope` 透传、不自动重发；`msgDeleted` 与 `sync` 的 `deleted=true`：清除本地原文，消息标为 `deleted`，不改未读与 seq；`fatal` 事件携带 `suspended` + `until`；`pushes` 流；`/demo/login` 4034 的 `details.bannedUntil` 可读（错误枚举已含 4034）。测试：Vitest + `FakeTransport` / `FakeClock`。
- 内容（push，PRD-live v1.0）：单游标规则 C1～C12、暂存区、无 `pushSeq` 队列、账号隔离、`pushSynced`、`pushStart`、自动 `pushSync` 且不被 `convs` 阻塞，跑通 `protocol/vectors/push-sync.json`（`persistence=true` 的用例 sdk-ts 跳过）。

### FE-350 禁言提示
- 优先级：P0
- 里程碑：第二批
- 仓库：tianting-web
- 依赖：FE-208、FE-232
- AC：AC-UI-41-1、AC-UI-41-2、AC-UI-41-3、AC-UI-41-4、AC-UI-41-5
- 内容：输入区替换为禁用条：有 `mutedUntil` →"你已被禁言，解除时间：{MM-DD HH:mm}"，永久 →"你已被禁言"；`scope=user` 影响全部会话、`scope=group` 只影响该群；`unmuted` 或到期恢复；`send` 4031 标记失败不弹轻提示。测试：组件测试（`FakeClock` 到期）；端到端 AC-UI-41-1～3（需要 PRD-admin F-ADM-40、41 服务端）。

### FE-351 消息被删除
- 优先级：P0
- 里程碑：第二批
- 仓库：tianting-web
- 依赖：FE-208、FE-229、FE-226
- AC：AC-UI-42-1、AC-UI-42-2、AC-UI-42-3、AC-UI-42-4
- 内容：被删除消息原位替换为"该消息已被删除"（提示消息样式，无头像 / 气泡 / 菜单）；DOM 中不再有原文；会话列表摘要"该消息已被删除"无前缀。测试：组件测试；端到端 AC-UI-42-1～3（需要 PRD-admin F-ADM-31 服务端）。

### FE-352 封禁提示
- 优先级：P0
- 里程碑：第二批
- 仓库：tianting-web
- 依赖：FE-208、FE-223、FE-235
- AC：AC-UI-43-1、AC-UI-43-2、AC-UI-43-3、AC-UI-43-4
- 内容：登录页 HTTP 错误映射加 403 / 4034："账号已被封禁，解封时间：{YYYY-MM-DD HH:mm}" / "账号已被封禁"，停留登录页；被踢弹窗表加 `suspended` 行（有 / 无 `until`），不认识的 `reason` + 4012 →"账号已被下线"；不自动重连。测试：Playwright（`page.route` 返回 403）、组件测试（`FakeTransport` 推 `kicked`）；端到端 AC-UI-43-1 / 3（需要 PRD-admin F-ADM-13 服务端）。

### FE-353 系统通知
- 优先级：P0
- 里程碑：第二批
- 仓库：tianting-web
- 依赖：FE-208、FE-237
- AC：AC-UI-44-1、AC-UI-44-2、AC-UI-44-3、AC-PUSH-12-1、AC-PUSH-12-2、AC-PUSH-12-3、AC-PUSH-12-4、AC-PUSH-12-5、AC-PUSH-12-6、AC-PUSH-12-7、AC-PUSH-12-8、AC-PUSH-12-10、AC-PUSH-12-11
- 内容：只处理 `push.type = notice`：顶部 5 秒横幅"系统通知：{前 50 码点}"，点击进 `/me/notices`；内存列表（倒序，最多 100）；"我"页入口显示未查看条数；纯文本渲染；其他 `type` 只写日志。依赖二期 `push` 协议定稿，最后交付。测试：组件测试 + Playwright（XSS）；端到端 AC-UI-44-1（需要 PRD-admin F-ADM-50 推送）。**2026-11-05 PRD-ui v1.1**：列表、横幅、未查看和 `ui.noticeSeen.{userId}` 按 PRD-live F-PUSH-12；倒序按 `ts`；`uiPrefs.ts` 增加 `removePref` 和这个键的例外说明。

### FE-354 全群禁言提示（P1）
- 优先级：P1
- 里程碑：P1
- 仓库：tianting-web
- 依赖：FE-350；PRD-admin F-ADM-42 排期
- AC：AC-UI-41-6
- 内容：`scope = groupAll` 时禁用条"群主已开启全员禁言"；不属于第二批交付。测试：组件测试。

---

## U9 原生端总任务（M5 / M6）

> 原生端按 PRD-ui 的交互和视觉实现 M3 + M4 全部页面；未标【Web】的 AC 用 Compose UI Test / XCUITest 按同样输入和预期执行，【Web】AC 按 3.6 节对照表执行（下面 AC 行列出对照表左列的 AC；其余 AC 由各总任务引用 TC-UI / TC-UI-M4 的同号用例）。逐页拆分由原生端开工时的任务单细化。

### FE-380 sdk-kotlin：PRD-ui SDK 缺口
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin
- 依赖：sdk-kotlin 首版（核心 + 社交，M5）；FE-200～FE-206、FE-210 的共享用例
- AC：AC-UI-33-13、AC-UI-21-2、AC-UI-13-9
- 内容：与 sdk-ts 同语义实现 `parseConvIdStrict`、`peerId`、`totalUnread`、`pendingCount`、`sendFailed`、`refreshConversations`、会话快照（含重连比对与 `convAdded` 清除）、不可变数据 + `equals`（G-UI-19）、打开未知会话请求最新一页、`removeFailed`；跑 `convid.json`、`avatar.json`、`pinyin-groups.json`（`android.icu.text.Collator`）。约 5 人日。

### FE-381 Android：M3 页面（按 3.6 节对照表）
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-android
- 依赖：FE-380；FE-221 的 `tokens.json`
- AC：AC-UI-01-3、AC-UI-01-7、AC-UI-02-5、AC-UI-10-2、AC-UI-10-7、AC-UI-16-3、AC-UI-18-1、AC-UI-21-5、AC-UI-22-3、AC-UI-90-2、AC-UI-90-12
- 内容：Compose 实现 M3 全部页面（登录、会话列表、聊天、通讯录与社交、我 / 设置）；token 由 `tokens.json` 生成 `TiantingTheme`；3.5 节单位、平台惯例覆盖、200% 字体缩放；`testTag` 与 `data-testid` 同名；deep link `tianting://`；对照表写法：返回键不关被踢弹窗、Coil 只 https、主题文件外无 `Color(0x…)` lint、`SharedPreferences` 只有 `ui.` 键、release Logcat 无特征串。约 8 人日。

### FE-382 Android：M4 页面（群管理、深色、表情、草稿）
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-android
- 依赖：FE-381
- AC：AC-UI-22-5、AC-UI-36-1、AC-UI-40-1
- 内容：群设置 / 公告 / 成员 / 邀请 / 发起群聊、群提示消息、深色跟随系统 + 手动、表情面板、草稿、删除失败消息；宽屏不做（平板后续）。按 TC-UI-M4 中非【Web】用例执行 Compose UI Test。约 3 人日。

### FE-383 sdk-swift：PRD-ui SDK 缺口
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift
- 依赖：sdk-swift 首版（核心 + 社交，M6）
- AC：AC-UI-33-13、AC-UI-21-2、AC-UI-13-9
- 内容：同 FE-380 的 Swift 实现；拼音分组用 `CFStringTransform`，以 `pinyin-groups.json` 为准。约 5 人日。

### FE-384 iOS：M3 页面（按 3.6 节对照表）
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-ios
- 依赖：FE-383
- AC：AC-UI-01-3、AC-UI-01-7、AC-UI-02-5、AC-UI-10-2、AC-UI-10-7、AC-UI-16-3、AC-UI-18-1、AC-UI-21-5、AC-UI-22-3、AC-UI-90-2、AC-UI-90-12
- 内容：SwiftUI 实现 M3 全部页面；`Tokens.swift` 由 `tokens.json` 生成；Dynamic Type 到 `accessibility3`；`accessibilityIdentifier` 同名；`onOpenURL` deep link；`interactiveDismissDisabled()`；ATS；SwiftLint 颜色规则；Keychain / `UserDefaults` 检查；release `os_log` 无特征串。约 8 人日。

### FE-385 iOS：M4 页面
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-ios
- 依赖：FE-384
- AC：AC-UI-22-5、AC-UI-36-1、AC-UI-40-1
- 内容：同 FE-382 的 iOS 实现，XCUITest 执行 TC-UI-M4 中非【Web】用例。约 3 人日。

### FE-386 Android：第二批提示
- 优先级：P0
- 里程碑：M5
- 仓库：tianting-sdk-kotlin、tianting-android
- 依赖：FE-382；PRD-admin 第二批协议
- AC：AC-UI-41-5、AC-UI-42-4、AC-UI-43-4、AC-UI-44-2
- 内容：sdk-kotlin 第二批事件（PRD-admin 第 14 节估 3 人日）与界面（禁言条、已删除占位、封禁弹窗、系统通知）。若第二批晚于 M5，随第二批补交。约 1 人日界面（SDK 计入 PRD-admin）。

### FE-387 iOS：第二批提示
- 优先级：P0
- 里程碑：M6
- 仓库：tianting-sdk-swift、tianting-ios
- 依赖：FE-385；PRD-admin 第二批协议
- AC：AC-UI-41-5、AC-UI-42-4、AC-UI-43-4、AC-UI-44-2
- 内容：同 FE-386 的 iOS 实现。约 1 人日界面（SDK 计入 PRD-admin）。
