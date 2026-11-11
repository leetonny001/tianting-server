# C6 前端验收报告（FE-698，2026-11-11）

| 项 | 内容 |
|---|---|
| 范围 | frontend-chat.md 里 FE-690～FE-698 的 `- AC：` 行（12 条：AC-MOD-01-7、02-9、03-1、03-9、03-11、04-1、04-6、04-12、04-15，AC-CHAT-90-8、90-9、90-10）。其中 AC-MOD-03-1、04-1 是端到端，服务端部分见 [c6-backend-acceptance-2026-11-11.md](c6-backend-acceptance-2026-11-11.md)（BE-499，那份报告记"部分通过，客户端和后台部分随 FE-698"），这里验客户端和管理后台部分；AC-CHAT-90-10 的服务端部分也在后端报告。PRD-admin v1.5、PRD-ui v1.6 只改规则说明，没有新的【C6】AC（BE-497）|
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**：H5 和管理后台是"Chrome，本地"；iOS 是"模拟器，本地"（iPhone 16 Pro，46E6F7B9-…）；Android 是"模拟器，本地"（emulator-5556，AVD `Pixel_10_Pro_XL`，`-read-only`，有窗口）。**没有执行**：真机（P-13）、WebKit、原生 CI（P-15）、推送（C3 暂停，BE-490）、新消息播报（P-39 ①）、性能（P-12） |
| 服务端 | 一律用 `git archive` 从已提交的内容导出后构建，不用工作区：im-server = tianting-server（im-mvp）@596de46（含 BE-499 的 d2ab9de），console-server = tianting-console-server@9f00aba |
| 基线 | SDK：tianting-sdk-ts@7a8272a、tianting-sdk-kotlin@128e789、tianting-sdk-swift@7cd52e3；页面：tianting-web@53f36d1、tianting-android@3586530、tianting-ios@02eda7d、tianting-admin@29d24f9。本次提交见第 7 节（没有改实现，只有同步脚本、用例和测试环境） |
| 结论 | **通过 10、部分通过 2、未执行 0、失败 0**（第 5、6 节）。AC-MOD-04-1 的完整流程（三端各举报一次 → 后台聚合 → 授权看证据带水印 → 删除消息 → 结案）通过；跨端三项都通过，每个接收端都做了反证，反证都按预期失败（第 2、3 节） |

## 1. 本次运行

开始时磁盘可用 57 GiB。各端全量按顺序跑；跨端要 Android、iOS 两台设备同时在线（被举报人在两端同时登录，第 2 节），只有这一段两台同时开着。日志在会话 scratchpad 的 `fe698/logs/`、`fe698/xp/logs/`（`/private/tmp/claude-501/-Users-frank-Desktop-workspace/0f7162ba-5fa9-4705-8b51-f8909b58f7e9/scratchpad/` 下）。

| 端 | 命令（本地） | 结果 |
|---|---|---|
| 三套 SDK 协议副本（问题 90） | 改过的 `scripts/sync-protocol.sh ../im-mvp 596de46`（第 7 节） | 三套都从 596de46 已提交的内容同步，内容和上次一致，只有 `protocol/SOURCE` 变化 |
| sdk-ts | `typecheck`、`lint`、`check:protocol`、`format:check`；`vitest run --project unit`；`--project integration`（真实 im-server，`TIANTING_SERVER_DIR` 指向导出的 596de46） | 检查都通过；单测 937 个：935 过、2 原有跳过；集成第 1 次 75 个里 **1 个失败**（`report.test.ts` 建群群名 4510 回了 4035，第 4 节新问题 96，用例前提错误），改用例后集成 75 个全过 |
| sdk-kotlin | `./gradlew --max-workers=2 --no-build-cache :sdk-core:check :sdk-core:test --rerun :sdk-core:integrationTest --rerun :sdk-android:testDebugUnitTest --rerun` | sdk-core 单测 1205 过；集成 95 过；sdk-android 7 过；`check` 通过 |
| sdk-swift | `swift test` | 第 1 次：单测 557 过；集成 95 个里 **1 个失败**（`BlockedIntegrationTests.testGroupNameWithTheWordIs4510WithoutDetails`，同问题 96）；改用例后整套重跑：单测 557、集成 95 全过 |
| H5 | `typecheck`、`lint`、`format:check`、`npm run check`；`vitest run`；`build`、`check-dist`、`check-assets`、`check-bundle`；`npx playwright test`（全量，真实 im-server） | 检查通过（对比度 68 组、颜色都来自 token）；单测 410 过；e2e 217 个：209 过、8 原有跳过（TC-ACC-03-12、03-45 要单独环境变量，6 个跨端用例没有运行号），`c6.spec.ts` 12 个全过 |
| 管理后台 | `typecheck`、`lint`、`format:check`、`check:dist`；`vitest run`；`npx playwright test`（临时 Postgres 17 `initdb -E UTF8 --locale=C`、`lc_messages=C`，+ im-server + console-server，生产 CSP） | 单测 169 过；e2e 69 个：65 过、4 跳过（截图 2 个、邀请链路 1 个要单独开关；新的跨端用例 `xp-c6.spec.ts` 没有运行号时跳过）。`words.spec.ts`、`reports.spec.ts` 全过 |
| Android（不要设备） | `./gradlew --max-workers=2 checkTianting :app:lintDebug :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest`，再 `--rerun` 一次 lint、单测、`checkTianting` | 通过；单测 76 过 |
| Android ui | `scripts/run-tests.sh ui` | **312 全过** |
| Android e2e | `scripts/run-tests.sh e2e "ServerE2eTest,AccountE2eTest,FilesE2eTest,C4E2eTest,C5E2eTest,C6E2eTest"`（真实 im-server，test-server.sh 默认开 C4～C6） | 83 个：81 过、2 按设计跳过（TC-ACC-03-12、03-45），`C6E2eTest` 6 个全过 |
| Android kill | `scripts/run-tests.sh kill`（7 个场景） | **7 个全过**（问题 75 的崩溃本次没有出现） |
| iOS Kit 单测 | `xcodebuild test -scheme TiantingAppKit` | 171 个：167 过、4 原有跳过（Keychain） |
| iOS 检查 | `check_sources`、`check_tokens`、`gen_tokens --check` | 通过（62 组对比度，token 是最新的） |
| iOS XCUITest 全量 | `scripts/run-e2e.sh --device 46E6F7B9-…`（真实 im-server） | 见下方"iOS 全量" |
| 跨端 | 第 2、3 节 | 3 个方向 + 最后一段通过；8 个反证都按预期失败 |
| 追溯 | `python3 -I tools/trace.py --check --summary` | 见第 9 节 |

**iOS 全量**（tianting-ios@02eda7d + 本次未提交时的跨端用例文件，后来提交为 66cd671；sdk-swift@d4c5da0）：212 个：198 过、12 按设计跳过（跨端用例没有运行号等，含新加的 3 个）、2 个失败：`AccountE2ETests.testTC_ACC_03_40_real4290StaysHere`（**问题 53，已知稳定失败**，本次单独重跑仍失败）；`FilesE2ETests.testImageSaveToPhotosAndRefusedPermission`（"gallery-denied" 等不到，**单独重跑通过**，FE-685、FE-686、FE-695 也偶发过）。`check_e2e_leftovers`：**CFNetwork 带文件名的行 0**（全量和两次单独重跑都是 0；跨端各次运行也都是 0）、带预签名地址的行 13（I11）、辅助功能日志带特征串 0、数据目录没有删除的原文。

GitHub Actions 没有手动触发（push 时自动排的没有等）。

## 2. 跨端（AC-MOD-03-1、02-9、03-11 的客户端部分）

三个用例文件开头是同一段约定（tianting-web `e2e/xp-c6.spec.ts`、tianting-android `CrossPlatformC6E2e.kt`、tianting-ios `UITests/CrossPlatformC6E2ETests.swift`、tianting-admin `e2e/xp-c6.spec.ts`）。要点：

- 共享环境（scratchpad `fe698/xp/stack6.sh`）：tianting-android `scripts/test-server.sh start`（真实 im-server，596de46 导出的源码，`IM_MOD_ENABLED=1`、现场生成的 `IM_REPORT_KEY`、`IM_MSG_ACTIONS_ENABLED=1`，SQLite，假 S3），`TT_EXTRA_CALLERS` 加 console-server 的两个 Ed25519 调用方；临时 Postgres 17；console-server（9f00aba 导出）托管 tianting-admin 的 dist。协调脚本 `fe698/xp/coord6.mjs` 建用户、群、加特征词 `zxqword{R}` 并等它生效、预置消息，再当信号板。
- 每个举报端 X 一个方向：举报人 rX、被举报人 vX；**vX 同时登录在另外两端**（同一个用户的两台设备）。接收端在两端都登录完之后才开始记录下行（H5：`routeWebSocket` 记服务端下行帧；Android：App 里 SDK 的 `IMClient.events`；iOS：中转上解析 WebSocket 帧）。
- **哨兵法**：举报端贴 reported（举报 ack 回来之后）后，协调脚本马上让第三人 s 给 vX 发哨兵；接收端要在 2 秒内看到哨兵，并且到 t0 + 2 秒为止，记录到的下行**只和哨兵的会话有关**，rX 的会话摘要、未读不变。4510 一项同样：举报端发含特征词的消息被拦截后贴 blocked，再用第二个哨兵，另外两端到 t0 + 2 秒只有哨兵，打开 rX 的会话只有原来那 1 条、没有含特征词的消息。
- 撤回：举报端打开群 G 看过「XP6-R-{X}」之后，协调脚本让 o 撤回，举报端点**最后一条撤回提示行的"举报"**提交成功、不问拉黑；再从消息菜单举报「XP6-M-{R}」（第 3 节的目标）。

**结果**（运行 c6d，全部一次通过）：

| 举报端 \ 接收端 | H5 | Android | iOS |
|---|---|---|---|
| H5 | — | 通过 | 通过 |
| Android | 通过 | — | 通过 |
| iOS | 通过 | 通过 | — |

三个举报端的举报（单聊消息、撤回的群消息、群消息）、4510 都通过。哨兵出现的时间（从接收端拿到信号算起）：H5 收 12～111 ms；Android 收 39～111 ms；iOS 收 44～125 ms（iOS 取自反证运行里通过的那一步，正式运行的 xcodebuild 日志被下一次覆盖，只留了通过结果）。共享服务端的 `server.log` 里搜不到 `XP6-`、`ZXQ`、`zxqword`。

**反证**（协调脚本 `--fake`，每个接收端两种各至少一次）：

| 反证 | 做法 | 接收端 | 结果 |
|---|---|---|---|
| `h5:report`（c6n1） | 没有举报端，协调脚本以 rX 给 vX 发一条再贴 reported | Android、iOS | 都在 ② 失败：Android "举报之后：被举报人收到了哨兵以外的事件：[MessagesChanged]"；iOS "after the report: the reported user got frames besides the sentinel's (1 of 2)" |
| `ios:report`（c6n1） | 同上 | H5、Android | 都在 ② 失败：H5 "举报之后：被举报人收到了哨兵以外的帧（2 帧里 1 帧）"；Android 同上 |
| `android:block`（c6n1） | 协调脚本真的举报一次（② 应该通过），再以 rX 发一条普通消息后贴 blocked | H5、iOS | ② 通过，③ 失败：H5 "4510 之后：被举报人收到了哨兵以外的帧（2 帧里 1 帧）"；iOS "after the 4510: … (1 of 2)"，rX 的行变成新消息、未读 1 |
| `h5:block`（c6n2） | 同上 | Android、iOS | ② 通过，③ 失败：Android "4510 之后：被举报人收到了哨兵以外的事件"；iOS 同上 |

所以三个接收端在两项上都不会空过；`block` 的反证同时说明"真的举报一次"确实不给被举报人任何帧（② 通过）。

调试过程（都是用例写法，断言没有放宽，修正后从新的运行号整套重跑）：c6a、c6b 两轮里接收端一开始断言 rX 的会话"未读 1"，但那条是 vX 自己发的，应该没有未读，改成"没有角标"；Android 接收端在列表上就读 SDK 的消息缓存（没打开会话时本来是空的），去掉这一句（打开后的断言保留）；后台一段的正文页工单号见第 4 节问题 97。

局限：H5 是同一台机器上的 Chrome；所有设备都是模拟器；Android 记的是 SDK 事件而不是原始帧。

## 3. AC-MOD-04-1 的完整流程（管理后台，最后一段）

运行 c6d：第 2 节三个方向做完以后，三端同时以各自的举报人登录、打开群 G，管理后台（tianting-admin `e2e/xp-c6.spec.ts`，生产构建 + 生产 CSP，真实 console-server + im-server + Postgres）新建一个审核员：

1. **撤回的群消息**（AC-MOD-03-11 的审核员一侧）：队列里三条「XP6-R-{X}」各是一行、各"1 次"，举报人分别是 rh5、randroid、rios，理由"其他"；逐条授权（理由 + 当次动态码，工单号是举报 ID 且只读）后显示"已撤回（xp…o，…），以下是隔离原文"和原文，平铺水印（账号 + 时间到分钟 + 授权前 8 位，超过 10 块）。
2. **聚合**：「XP6-M-{R}」那一行"**3 次**"，点开 3 条都是"垃圾广告 · 待处理"（AC-MOD-04-6 的三端版本）。
3. **授权看证据**：授权前页面没有正文；授权后显示「XP6-M-{R}」和水印。
4. **删除消息**：举报详情的"删除消息"只是链接（F-MOD-04 规则 5），到正文页走现有的正文授权、勾选、理由、动态码确认；点"验证"之前贴 deleting。三端**都在 2 秒内**看不到原文、显示"该消息已被删除"：H5 89 ms、Android 173 ms、iOS 196 ms（之前的调试运行 c6c 是 94、201、381 ms）。
5. **结案**：三条都"已处理"，状态已处理；待处理队列里没有这一行，"全部"里仍是"3 次"。
6. **审计顺序**（TC-MOD-04-01）：审计员查这个审核员的 console 审计，`report.view` → `report.evidence.grant` → `msg.delete` → `report.handle` 的第一次按这个顺序。

页面没有 CSP 违规和 JS 报错（`watchPage`）。

## 4. 新问题

| # | 位置 | 问题 | 处理 |
|---|---|---|---|
| 96 | sdk-ts `test/integration/report.test.ts`、sdk-swift `ReportIntegrationTests.swift`（FE-698 全量发现） | "建群的群名含特征词 4510"用例邀请的不是好友。BE-499（d2ab9de，问题 137）把建群的敏感词检查移到被邀请人判定之后，于是先回 4035，用例失败。sdk-kotlin 的同名用例本来就先导入好友并有正常群名的对照，所以通过 | **用例前提错误，已修**：两套都先 `/admin/friends/import`，加正常群名的对照，4510 的断言不变（tianting-sdk-ts@ab9a88d、tianting-sdk-swift@d4c5da0）。不是实现错误 |
| 97 | 管理后台举报详情 → "删除消息" → 正文页（FE-698 跨端发现） | 跳到正文页后要另做一次正文授权，工单号必填且有格式要求，举报 ID（`rp_…`）不符合，审核员要自己再填一个工单号；证据授权那边工单号固定是举报 ID | 记录，不改：PRD-admin 的正文授权和举报证据授权本来是两套（F-MOD-04 规则 5 只要求"跳转到已有操作"），现在能走通。是否让跳转时带上举报 ID 当工单号属于体验优化，可以随后续批次再看，不阻塞 |

已知问题：**问题 53**（iOS TC-ACC-03-40）见第 1 节 iOS 全量；**问题 75**（Android kill 偶发崩溃）本次 kill 7 个全过，没有出现；**问题 95**（Android `ChatTest` 偶发）本次 ui 312 全过，没有出现。两条仍是记录状态。

本次**没有发现 C6 的实现错误**，没有新的要 Frank 决定的事。

## 5. 逐条结果

"依据"写用例；没写提交的是表头"基线"里的提交。

| AC | 层级 | 状态 | 依据 | 说明 |
|---|---|---|---|---|
| AC-MOD-01-7 | 前端（管理后台） | 通过 | tianting-admin `words.spec.ts`（添加、批量导入结果汇总、搜索、停用，纯文本）、`Words.test.tsx`；本次后台全量 | |
| AC-MOD-02-9 | 前端 | 通过 | H5 `c6.spec.ts` TC-MOD-02-09；Android `C6E2eTest.tc_mod_02_09_real_blocked_send`；iOS `testTC_MOD_02_09_…`；三套 SDK 集成（发送 4510）；**跨端第 2 节**三个举报端各发一次（失败、提示、输入框清空），另外两端收不到 | |
| AC-MOD-03-1 | 端到端 | 通过 | 服务端部分见后端报告；三端用例（H5 TC-MOD-03-09/03-01、Android `tc_mod_03_09_…`、iOS `testTC_MOD_03_09_…` 含队列）；**跨端第 2 节**：三端各举报一次，被举报人在另外两端 2 秒内只收到哨兵，4 组反证 | |
| AC-MOD-03-9 | 前端 | 通过 | H5 TC-MOD-03-09 两个；Android `tc_mod_03_09_…`、`real_report_user_and_group`；iOS `testTC_MOD_03_09_…`、`testIssue79…`；跨端第 2 节（提交前提示、成功提示、单聊询问拉黑、群不问） | |
| AC-MOD-03-11 | 前端 | 通过 | H5 TC-MOD-03-11；Android `tc_mod_03_11_…`；iOS `testTC_MOD_03_11_…`；后台 `reports.spec.ts`；**跨端第 2、3 节**：三端各从撤回提示行举报一次，审核员读到三条隔离原文 | |
| AC-MOD-04-1 | 端到端 | 通过 | 服务端部分见后端报告；**第 3 节**完整流程（三端各举报一次 → 聚合 → 授权读证据带水印 → 删除 → 三端消失 → 结案 → 审计顺序） | |
| AC-MOD-04-6 | 前端（管理后台） | 通过 | `reports.spec.ts`（同一用户被 3 人举报，"3 次"，纯文本）；第 3 节（同一条消息被三端举报，"3 次"） | |
| AC-MOD-04-12 | 前端（管理后台） | 通过 | `reports.spec.ts`（图片 `data:` 同源、字节一致，文件只能下载，CSP `img-src 'self' data:` 没有放宽，`securitypolicyviolation` 为空） | |
| AC-MOD-04-15 | 前端（管理后台） | 通过 | `reports.spec.ts`（文件名含 `<script>` 按纯文本，没有 img/iframe/script） | |
| AC-CHAT-90-8 | 前端【全部】 | 部分通过 | H5 TC-CHAT-90-44（axe 浅色、深色，图标 Lucide `flag`，颜色来自 token）；Android `C6UiTest.tc_chat_90_44_named_buttons_and_dark_tokens`；iOS `testTC_CHAT_90_44_namedLightAndDark`；`check-assets`、`checkTianting`、`check_tokens` | 口径同 C4、C5：原生两端没有逐个和 PRD-ui 6.6 节图标清单核对 |
| AC-CHAT-90-9 | 前端【全部】 | 部分通过 | H5 TC-CHAT-90-45（只用键盘完成举报、Esc 关闭）；Android 辅助操作、iOS VoiceOver 操作（FE-694、FE-695 用例，本次全量通过） | 原生是等价检查不是 axe；新消息播报不做（P-39 ①） |
| AC-CHAT-90-10 | 前端【全部】（三套 SDK） | 通过 | `frames.json` C6 用例（13 个合法、32 个非法）：sdk-ts `frames-schema.test.ts`、sdk-kotlin `VectorsTest`、sdk-swift `VectorTests`，本次全量重跑；三套副本从 596de46 已提交内容同步，和服务端一致；服务端见后端报告 | |

## 6. 汇总

| 状态 | 数量 | AC |
|---|---|---|
| 通过 | 10 | AC-MOD-01-7、02-9、03-1、03-9、03-11、04-1、04-6、04-12、04-15，AC-CHAT-90-10 |
| 部分通过 | 2 | AC-CHAT-90-8、90-9（原生图标清单没逐个核对、原生不是 axe、播报不做） |
| 未执行 | 0 | — |
| 失败 | 0 | — |
| 合计 | 12 | |

后端报告里 AC-MOD-03-1、04-1 的"客户端和后台页面部分随 FE-698"本次补齐，这两条可以改为通过；后端报告本身没有改。没有删除或放宽已有断言。

## 7. 本次改动

**没有改实现**。

| 仓库 | 提交 | 内容 |
|---|---|---|
| tianting-sdk-ts | 71d0208、ab9a88d | `scripts/sync-protocol.sh` 改为用 `git archive` 取已提交内容（问题 90），协议副本同步到 596de46；集成用例问题 96 |
| tianting-sdk-kotlin | c8c86af | 同步脚本（问题 90），协议副本同步到 596de46 |
| tianting-sdk-swift | fcf22cf、d4c5da0 | 同步脚本（问题 90），协议副本同步到 596de46；集成用例问题 96 |
| tianting-web | 35d49a6 | `e2e/xp-c6.spec.ts`；`playwright.xp.config.ts` 匹配它 |
| tianting-android | 92652c9 | `CrossPlatformC6E2e.kt`；`scripts/test-server.sh` 加 `TT_EXTRA_CALLERS`（额外调用方，给 console-server 连同一个服务端） |
| tianting-ios | 66cd671 | `UITests/CrossPlatformC6E2ETests.swift`；`TCPProxy` 记下行 WebSocket 文本帧（`socketFrames`，只在测试内存里） |
| tianting-admin | af1bd46 | `e2e/xp-c6.spec.ts`、`playwright.xp.config.ts`（没有 webServer，连跨端环境） |
| tianting-server（im-mvp） | 本报告所在提交 | 本报告；frontend-chat.md 的 FE-698 进度行、问题 90 的结论、新问题 96、97 |

**问题 90 的改法**：三套 `scripts/sync-protocol.sh [检出目录] [提交，默认 HEAD]` 用 `git archive <提交> protocol/errors.json protocol/schema protocol/vectors` 解到临时目录再复制，`protocol/SOURCE` 写这个提交；目录不是 git 仓库（`git archive` 导出的）时必须给提交号。验证：传导出目录不给提交号报错退出；在一个工作区改了 `errors.json` 但没提交的临时仓库上同步，副本仍是已提交的内容。README 的用法同步改了。

tianting-ios 的 `Config/Info-Debug.plist`、`Tianting.xcodeproj/project.pbxproj` 两处脏文件没有提交，也没有还原；远端没有新提交，直接推送。

## 8. 开放项

1. C3 恢复后：推送相关（BE-490、BE-459 的 4510 / 举报不进推送队列的哨兵用例）。
2. 新消息播报（P-39 ①）、原生图标清单逐个核对（AC-CHAT-90-8）、性能（P-12）、真机（P-13）、WebKit、部署 e2e。
3. 问题 53、75、95 仍是记录状态；问题 97 记录。
4. 后端报告里 AC-MOD-03-1、04-1 可以改为通过（第 3、5 节）。
5. 上线门槛（`deploy/MODERATION-RUNBOOK.md` 第 6 节）都未确认，同后端报告。

## 9. 追溯

`python3 -I tools/trace.py --check --summary`（报告和任务单改完后）：AC 1694、covered 1694、**gaps 0**、TC 2045、unknown refs 0、dup 0。

## 10. 清理

- iOS 模拟器 iPhone 16 Pro（46E6F7B9-…）用完 `xcrun simctl shutdown`，现在是 Shutdown；没有碰 iPhone 18 Pro、iPhone 11，也没有碰 PID 96273。
- Android：emulator-5556 由本次启动（只读、有窗口），用完 `adb -s emulator-5556 emu kill`，进程已退出，`adb devices` 为空。Gradle 只用 2 个 worker，没有执行 `--stop`。
- 跨端的共享服务端、假 S3、console-server、协调脚本按 PID 停掉，临时 Postgres（两个：后台全量用的 55497、跨端用的 55498）已停并删除数据目录；用过的端口（18080、18081、18180、18181、18190、18280～18283、19200、18299、15373、55497、55498）都已释放。调试时停掉的旧一轮进程也是按 PID 停的。
- scratchpad 里导出的服务端和 console 源码、二进制、库、iOS DerivedData、所有 `.xcresult`（18 个）已删除，只保留日志；`tianting-admin/e2e/.state.json` 已删除。
- 没有运行 `~/Desktop/cleanup-dev-cache.sh`。结束时磁盘可用 55 GiB。
