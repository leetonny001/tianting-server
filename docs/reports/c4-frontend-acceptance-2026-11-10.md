# C4 前端验收报告（FE-672，2026-11-10）

| 项 | 内容 |
|---|---|
| 范围 | frontend-chat.md 里 FE-660～FE-672 的 `- AC：` 行（41 条，含后台页面 AC-RCL-04-3、04-7、04-9 和 AC-CHAT-90-10），加上 PRD-chat v1.7 第 15 节【全部】里 C4 增加、挂在后端任务上的 4 条（AC-CHAT-90-1、90-2、90-5、90-17，状态引用后端报告），共 45 条。后端 AC 见 [c4-backend-acceptance-2026-11-10.md](c4-backend-acceptance-2026-11-10.md)，这里只补端到端 AC 的客户端部分和 AC-CHAT-90-10 的 sdk-swift 部分 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**：H5 和管理后台是"Chrome，本地"；iOS 是"模拟器，本地"（iPhone 16 Pro，iOS 18.4）。Android 是"模拟器，本地"（emulator-5556，AVD `Pixel_10_Pro_XL`，`-read-only`）。第一轮 AVD 被 emulator-5554 非只读占用，`-read-only` 实例起不来；Frank 关掉 5554 后同一天**补跑了 Android**（第 1a 节），本报告的结论已按补跑更新。**没有执行**：真机（P-13）、WebKit、原生 CI（P-15）、推送（C3 暂停，P-43 ②）、新消息播报（P-43 ③） |
| 基线 | 页面：tianting-web@8923a5b、tianting-android@41d5c87、tianting-ios@f6c0411，后台 tianting-admin@aa47b1f；SDK：tianting-sdk-ts@55d39b1、tianting-sdk-kotlin@686e14f、tianting-sdk-swift@b73dd2e；服务端 tianting-server（im-mvp）@bea63a7、tianting-console-server@b11828d。本次先落实问题 52 和 P-45（第 7 节），验收在改完的代码上做：tianting-server@6c14e2c、tianting-sdk-swift@c97d08e、tianting-sdk-kotlin@d6603f5、tianting-sdk-ts@7739c55、tianting-ios@ee522bc、tianting-web@3cd6e96、tianting-android@fd5058a（补跑时 @def3170，只加了登录用时的日志） |
| 结论 | **通过 39、部分通过 4、未执行 2、失败 0**（第 5、6 节；第一轮是通过 16、部分通过 27，补跑 Android 后 23 条有了实测证据，改为通过）。跨端互发互收 6 个方向都通过（第 2 节） |

## 1. 本次运行

各平台按顺序跑，没有同时开两端的重任务。开始时磁盘可用 60 GiB，结束时 60 GiB。

| 端 | 命令（本地） | 结果 |
|---|---|---|
| 共享服务端（跨端用） | `TT_SERVER_STATE_DIR=<scratchpad> TT_S3_PORT=19200 TT_S3_ORIGINS=http://127.0.0.1:15373 tianting-android/scripts/test-server.sh start`（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`，SQLite，假 S3）；scratchpad 里的 `coord.mjs` 用管理接口建 3 个用户、7 个群、9 条预置消息（运行号 `c41549`）；最后 `test-server.sh stop` | 三端轮流连它 |
| im-server | `go vet ./...`、`go test -count=1 ./...`（P-45 之后） | 全部包通过 |
| tianting-console-server | `go test -count=1 ./...`（`CON_TEST_REQUIRE_DB=1`，临时 Postgres 17.11，`LC_ALL=C`、`lc_messages=C`，结束后停掉并删除数据目录） | 全部包通过（e2e 166 秒） |
| sdk-ts | `vitest --project unit`、`--project integration`、`typecheck`、`lint`、`check:protocol` | 单测 766 过、2 跳过（冷启动，H5 没有持久化）；集成 63 过 |
| sdk-kotlin | `./gradlew --max-workers=2 :sdk-core:check :sdk-core:integrationTest :sdk-android:testDebugUnitTest` | 单测 1017 过；集成 82 过；Android 适配层 5 过 |
| sdk-swift | `swift test` | 单测 482 过；集成 82 过 |
| H5 | `typecheck`、`lint`、`format:check`、`check`；`vitest run`；`npx playwright test`（全量，真实 im-server） | 检查通过；单测 354 过；e2e 176 过、6 跳过（2 个要单独环境变量的 TC-ACC-03-12、03-45，4 个跨端用例没有运行号） |
| 管理后台 | `typecheck`、`lint`、`format:check`；`vitest run`；`npx playwright test`（真实 Postgres + im-server + console-server） | 单测 125 过；e2e 58 过、3 跳过（截图巡检 2 个、邀请链路 1 个，都要单独开关） |
| Android（第一轮） | 模拟器：`emulator -avd Pixel_10_Pro_XL -port 5556 -read-only` | **没有起来**：AVD 被 emulator-5554（非只读启动）占着，进程报错退出。设备上的用例改到第 1a 节补跑 |
| Android（不要设备的部分） | `./gradlew --max-workers=2 checkTianting :app:lintDebug :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest` | 通过；单测 64 过；新的跨端用例 `CrossPlatformC4E2e` 编译通过 |
| iOS | `scripts/run-e2e.sh --device 46E6F7B9-…`（全量 XCUITest，真实 im-server）；`check_sources`、`check_tokens`、`gen_tokens --check`；Kit 单测 | 全量 184 个：175 过、7 个按设计跳过（跨端用例没有运行号等）、2 个失败：`FilesE2ETests.testTC_FILE_04_10_…`（200 张图，"滚到第一条"的断言超时，单独重跑通过）；`AccountE2ETests.testTC_ACC_03_40_real4290StaysHere` 单独重跑仍失败，把 sdk-swift 切回改动前的 b73dd2e 再跑同样失败，和本次改动无关，记问题 53。`check_e2e_leftovers` 通过（CFNetwork 带文件名的行 0，带签名的行 13，I11；辅助功能日志 0）。Kit 单测 148 个（144 过、4 个原有跳过）；三个检查脚本通过 |
| iOS 跨端 | `TT_E2E_EXTERNAL=1 … TT_E2E_XP=c41549 scripts/run-e2e.sh --only CrossPlatformC4E2ETests/testXP4a_send`，再 `TT_E2E_XP_FROM=h5 … /testXP4b_receive` | 都通过；`check_e2e_leftovers` 通过（CFNetwork 带文件名的行 0、带签名的行 0） |
| 追溯 | `python3 -I tools/trace.py --check --summary` | AC 1598，gaps 0 |

GitHub Actions 没有手动触发（push 时自动排的没有等）。

## 1a. Android 补跑（同日，Frank 关掉 emulator-5554 之后）

`emulator -avd Pixel_10_Pro_XL -port 5556 -read-only`（有窗口），`ANDROID_SERIAL=emulator-5556`，`GRADLE_OPTS=-Dorg.gradle.workers.max=2`（`run-tests.sh` 里的 Gradle 也只用 2 个 worker）；跑完 `adb -s emulator-5556 emu kill`。开始时磁盘可用 64 GiB。

| 命令（本地） | 结果 |
|---|---|
| `scripts/run-tests.sh ui` | **278 个全过**（含 `C4UiTest`、`FilesUiTest`） |
| `scripts/run-tests.sh e2e com.tianting.app.ServerE2eTest,…AccountE2eTest,…FilesE2eTest,…C4E2eTest`（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`） | 71 个：67 过；2 个按设计跳过（TC-ACC-03-12、03-45 要单独的 `IM_ACCESS_TTL`、`IM_REFRESH_ABSOLUTE`）；**2 个失败**，都在 C2 的 `FilesE2eTest`：`tc_file_03_05_…`（`appLogin` 15 秒内没进到首页）、`tc_file_03_01_…`（15 秒内列表里没出现新群）。这两个单独重跑都通过。`C4E2eTest` 14 个全过 |
| `scripts/run-tests.sh kill` | 6 个场景全过（含 `C4KillE2e.recallWhileKilled`、`FilesKillE2e.uploadKilled`） |
| 跨端：`TT_SERVER_RUNNING=1 TT_XP=c41806 scripts/run-tests.sh e2e "com.tianting.app.CrossPlatformC4E2e#xp_c4_send"`，`… #xp_c4_receive`（`TT_XP_FROM=h5`、`ios` 各一次） | 都通过；反证见第 2 节 |

**C4E2eTest 的登录重试有没有藏住慢**：FE-667 时 `appLogin` 改成了在 30 秒内循环（每 0.7 秒看一次，"登录"按钮还在就再点、出现"配置重试"就点），这可能把一次很慢的登录藏起来。本次在循环里记下每次登录的用时、点"登录"的次数和点"配置重试"的次数（只有数字，logcat 标签 `TT-E2E-LOGIN`，tianting-android@def3170），全量 e2e 里 `C4E2eTest` 的 14 次登录：**用时 1.2～2.5 秒**；11 次只点了 1 下，3 次点了 2 下（第二下发生在 0.7 秒时登录还没完成，不是失败后重试）；**"配置重试"一次都没有出现**。跨端用例的登录也一样（1.2 秒，1 下）。结论：这一轮重试没有掩盖慢登录，用时离 30 秒的上限很远；30 秒的上限本身仍然偏宽，今后如果用时明显变长，日志里能看出来。`FilesE2eTest` 的 `appLogin` 是另一份（15 秒），这次的一次失败就是在这里超时，见上表。

## 2. 跨端互发互收（AC-RCL-01-1、02-4、03-3，AC-QUO-01-1，AC-MEN-01-1 的客户端部分）

三端不能同时运行，所以"先发后收"，连同一个服务端。约定写在三个用例文件开头（tianting-web `e2e/xp-c4.spec.ts`、tianting-android `CrossPlatformC4E2e.kt`、tianting-ios `UITests/CrossPlatformC4E2ETests.swift`）：

- 每个发送端 p 有两个群（三人都在，群主是 p）：「C4引用提及-{p}」「C4撤回-{p}」。协调脚本先让另外两端在里面各放好消息。
- **发**（p 用自己的界面）：① 给另外两端单聊各发一条再长按（H5 右键）撤回；② 在引用提及群引用另一端预置的消息回复，再输入 @ 从选择里点另外两人、加正文发送（上行 `mentions` 是那两人）；③ 在撤回群以群主身份撤回两个成员的消息。
- **收**（me 用自己的界面）：单聊显示"对方撤回了一条消息"、页面（和 H5 的消息列表）里没有原文，列表摘要是撤回文案；引用提及群列表有"[有人@我]"，回复气泡顶部是"{被引用者}: 原文"，提及消息标记为提到我（H5 `data-mentioned`、iOS 读作"…，提到了你"、Android `data-mark=mention`），SDK 里 `mentions.userIds` 含我；撤回群按 F-RCL-02 规则 1a 第 3 行：自己那条"{p}撤回了你的一条消息"、另一条"{p}撤回了一条成员消息"，列表摘要按最后一条。接收方在撤回发生时不在线，重连后拉到撤回状态，所以同时是 AC-RCL-03-3 的跨端部分。

| 收 \ 发 | H5 | Android | iOS |
|---|---|---|---|
| H5 | — | 通过（补跑，运行号 c41806） | 通过（c41549；c41806 又通过一次） |
| Android | 通过（补跑） | — | 通过（补跑） |
| iOS | 通过（c41549） | 通过（补跑，c41806；同一次也收了 H5） | — |

反证：H5 在 Android 还没发时按 `TT_XP_FROM=android` 收，失败在"[有人@我]"找不到；iOS 按 `TT_E2E_XP_FROM=android` 收，同样失败；补跑时 Android 在 H5、iOS 都还没发时按 `TT_XP_FROM=h5,ios` 收，30 秒后同样失败在"[有人@我]"。三端的接收用例都不会空过。

补跑的顺序（三端不同时运行）：新运行号 c41806 → Android 反证、Android 发 → H5 发、H5 收 Android → Android 收 H5 → 关模拟器 → iOS 发、iOS 收 Android 和 H5 → 关 iOS 模拟器 → H5 收 iOS → 再开 Android 模拟器收 iOS。共享服务端的 `server.log` 里搜不到 `XP4-` 特征串。

发送端自己的界面也断言了：撤回后自己是"你撤回了一条消息"/"你撤回了 {成员} 的一条消息"，撤回群的列表摘要"你撤回了 {q2} 的一条消息"；引用条是"{q1}: 原文"。

局限：不是同时在线互发（实时到达在各端自己的 TC-RCL-01-01 里验过，对方是同进程的 SDK）。

## 3. AC-CHAT-90-8、90-9（菜单和引用条、@ 选择、"有人@我"）

| 项 | H5 | Android | iOS |
|---|---|---|---|
| 自动检查 | `c4.spec.ts` TC-CHAT-90-31（axe：菜单、引用条、@ 选择、"有人@我"、撤回提示，浅色、深色），本次全量通过 | `C4UiTest.tc_chat_90_31_*`（语义树的等价检查），补跑通过 | `testTC_CHAT_90_30_TC_CHAT_90_31_c4PagesNamedLightAndDark`（系统审计 `.sufficientElementDescription`，浅色、深色），本次全量通过 |
| 键盘走查 | TC-CHAT-90-31 键盘用例（Tab、Shift+F10、方向键、Esc）本次通过 | 不适用 | 不适用 |
| 颜色来自 token | `npm run check` 通过 | `checkTianting` 通过 | `check_tokens`、`check_sources` 通过 |
| 新消息播报 | 不做（P-43 ③） | 不做 | 不做 |

图标清单（B-3）没有逐个核对，和 C2 相同。所以两条都记**部分通过**。

## 4. 未执行、等 C3 的

- **AC-MEN-01-9**（手工）：三端系统中文输入法组字时输入 @。要人工在 Android、iOS 真机或模拟器上用系统中文输入法操作（P-13 的真机部分另算），本次**未执行（要人工）**。各端已有自动化的组字用例（H5 `compositionstart`、Android `C4UiTest` 组字中改模型、iOS `C4Tests`）不能代替系统输入法。
- **AC-RCL-03-9、AC-APN-03-3**（后端）：挡在 C3 的 BE-481，按 P-43 ② 记**未执行（等 C3）**，见后端报告，不计入本报告。推送相关的前端部分同样等 C3。
- **新消息播报**：按 P-43 ③（P-39 ①）继续推后，是已知缺口。
- **AC-CHAT-90-1**：性能，等 P-12（后端报告）。

## 5. 逐条结果

"依据"写用例；没写提交的是表头"基线"里改完之后的提交。"Android 补跑通过"指第 1a 节的 ui / e2e / kill 全量里对应用例通过（第一轮这些条记部分通过，只缺 Android）。

| AC | 层级 | 状态 | 依据 | 说明 |
|---|---|---|---|---|
| AC-RCL-01-1 | 端到端 | 通过 | 各端 TC-RCL-01-01（H5 `c4.spec.ts`、iOS `C4E2ETests`）；**跨端**第 2 节；后端 `TestTC_RCL_01_01_RecallWithin60s` | 三端通过（Android 补跑），跨端 6 个方向通过 |
| AC-RCL-01-8 | 前端 | 通过 | TC-RCL-01-08（H5、iOS） | Android 补跑通过 |
| AC-RCL-02-4 | 端到端 | 通过 | TC-RCL-02-04（H5、iOS）；跨端第 2 节撤回群 | 三方的聊天页和列表摘要按规则 1a 第 3 行；Android 补跑通过 |
| AC-RCL-02-5 | 前端 | 通过 | TC-RCL-02-05（H5、iOS） | Android 补跑通过 |
| AC-RCL-03-1 | 前端（sdk-ts） | 通过 | `msg-actions.test.ts` | |
| AC-RCL-03-2 | 前端（sdk-kotlin、sdk-swift） | 通过 | `MsgActionsPersistenceTest`、`MsgActionsPersistenceTests` | 本地库文件和缓存目录搜不到 |
| AC-RCL-03-3 | 端到端 | 通过 | TC-RCL-03-03（H5 断开 WebSocket、iOS `TCPProxy` 断开）；跨端第 2 节（接收方离线时被撤回） | Android 补跑通过 |
| AC-RCL-03-4 | 前端（sdk-kotlin、sdk-swift） | 通过 | `MsgActionsPersistenceTest`、`MsgActionsIntegrationTest`、sdk-swift 同名用例；iOS `testTC_RCL_03_04_…`（真实杀进程） | Android 页面的真实杀进程 `C4KillE2e.recallWhileKilled` 补跑通过 |
| AC-RCL-03-6 | 前端 | 通过 | TC-RCL-03-06（H5、iOS） | Android 补跑通过 |
| AC-RCL-03-8 | 前端 | 通过 | TC-RCL-03-08（H5、iOS `C4FilesE2ETests`） | Android 补跑通过 |
| AC-RCL-03-10 | 前端（三套 SDK） | 通过 | 三套 SDK 单测；页面 TC-RCL-01-01（H5、iOS） | |
| AC-RCL-03-11 | 前端（三套 SDK） | 通过 | `msg-actions.test.ts`、`MsgActionsTest`、`MsgActionsTests`，`chat-sync.json` | |
| AC-RCL-03-12 | 前端（Android、iOS） | 通过 | iOS `testTC_RCL_03_12_…`；sdk-swift `ObjectStoreDownloadTests`；Android `FilesE2eTest`、`FilesUiTest`（补跑） | **iOS 现在全部做到**：问题 52 修好后去掉了 `XCTExpectFailure`，URLSession 自己的临时文件也不留（第 7 节）。Android 补跑通过 |
| AC-RCL-03-13 | 前端（三套 SDK） | 通过 | 三套 SDK 单测和 `chat-sync.json` | |
| AC-RCL-04-3 | 后台页面 | 通过 | tianting-admin `e2e/recall.spec.ts` TC-RCL-04-03 | 后端部分见后端报告 |
| AC-RCL-04-7 | 后台页面 | 通过 | `recall.spec.ts` TC-RCL-04-07 | 同上 |
| AC-RCL-04-9 | 后台页面 | 通过 | `recall.spec.ts` TC-RCL-04-03 TC-RCL-04-09（经 console 代理下载） | 同上 |
| AC-QUO-01-1 | 端到端 | 通过 | TC-QUO-01-05（H5、iOS，对方收到 `quote.state=normal`、原文摘要）；跨端第 2 节 | Android 补跑通过，跨端 6 个方向通过 |
| AC-QUO-01-5 | 前端 | 通过 | TC-QUO-01-05（H5、iOS，`C4FilesE2ETests` 图片和文件的摘要） | Android 补跑通过 |
| AC-QUO-02-2 | 前端（三套 SDK） | 通过 | 三套 SDK `MsgActions*`、持久化用例 | |
| AC-QUO-02-3 | 前端 | 通过 | TC-QUO-02-03（H5、iOS） | Android 补跑通过 |
| AC-QUO-02-5 | 前端 | 通过 | TC-QUO-02-05（H5、iOS） | Android 补跑通过 |
| AC-QUO-02-6 | 前端（三套 SDK） | 通过 | `chat-sync.json`（三套执行器） | |
| AC-MEN-01-1 | 端到端 | 通过 | TC-MEN-01-01（H5、iOS）；跨端第 2 节 | Android 补跑通过，跨端 6 个方向通过 |
| AC-MEN-01-5 | 前端 | 通过 | TC-MEN-01-05（H5、iOS） | Android 补跑通过 |
| AC-MEN-01-6 | 前端 | 通过 | TC-MEN-01-06（H5、iOS） | Android 补跑通过 |
| AC-MEN-01-7 | 前端 | 通过 | TC-MEN-01-07（H5、iOS） | Android 补跑通过 |
| AC-MEN-01-8 | 前端 | 通过 | TC-MEN-01-08（H5、iOS） | Android 补跑通过 |
| AC-MEN-01-9 | 手工 | 未执行 | — | 要人工，第 4 节 |
| AC-MEN-01-10 | 前端 | 通过 | TC-MEN-01-10（H5、iOS） | Android 补跑通过 |
| AC-MEN-01-12 | 前端 | 通过 | TC-MEN-01-12（H5、iOS） | Android 补跑通过 |
| AC-MEN-02-4 | 前端 | 通过 | TC-MEN-02-04（H5、iOS） | Android 补跑通过 |
| AC-MEN-02-5 | 前端 | 通过 | TC-MEN-02-05（H5、iOS） | Android 补跑通过 |
| AC-MEN-03-5 | 前端 | 通过 | TC-MEN-03-05（H5、iOS） | Android 补跑通过 |
| AC-MEN-03-6 | 前端 | 通过 | TC-MEN-03-06（H5、iOS） | Android 补跑通过 |
| AC-MEN-03-7 | 前端（三套 SDK） | 通过 | 三套 SDK 单测、`chat-sync.json`、sdk-kotlin / sdk-swift 集成 | |
| AC-CHAT-90-7 | 前端（老版本三套 SDK） | 通过 | TC-CHAT-90-26：sdk-ts `legacy-c4-frames.test.ts`、sdk-kotlin `LegacyC4FramesTest`、sdk-swift `LegacyC4FramesTests` | |
| AC-CHAT-90-8 | 前端【全部】 | 部分通过 | 第 3 节 | 图标清单没有逐个核对；新消息播报不做（Android `C4UiTest` 补跑通过） |
| AC-CHAT-90-9 | 前端【全部】 | 部分通过 | 第 3 节 | 原生是等价检查不是 axe（Android 补跑通过） |
| AC-CHAT-90-10 | 后端【全部】 | 通过 | `frames.json`：服务端（后端报告）；sdk-ts `frames-schema.test.ts`、sdk-kotlin `VectorsTest`、**sdk-swift `VectorTests`** | **本次确认 sdk-swift**：`testFrameVectorsAgainstSchema` 和 `testAllDownstreamFramesAreSafe` 逐个跑 `frames.json` 的全部 122 个合法、109 个非法用例，其中 C4 的 25 个合法、29 个非法（和服务端 `TestTC_CHAT_90_32` 的数一致），本次 `swift test` 通过；三套 SDK 的 `protocol/vectors` 和 tianting-server@8527b7b 逐字节相同。后端报告里这条的"部分通过"可以改为通过 |
| AC-CHAT-90-15 | 前端【全部】（三套 SDK） | 通过 | `chat-sync.json` 的 C4 用例：sdk-ts `chat-sync-vectors.test.ts`、sdk-kotlin `ChatSyncVectorsTest`、sdk-swift `ChatSyncVectorTests` | |
| AC-CHAT-90-1 | 后端【全部】 | 未执行 | 后端报告 | 性能，P-12 |
| AC-CHAT-90-2 | 后端【全部】 | 部分通过 | 后端报告 | TC-PERF 等 P-12 |
| AC-CHAT-90-5 | 后端【全部】 | 部分通过 | 后端报告 | 部署 e2e 没做；iOS 系统 CFNetwork 日志里的文件名已由 P-45 去掉（第 7 节） |
| AC-CHAT-90-17 | 后端【全部】 | 通过 | 后端报告 | |

## 6. 汇总

| 状态 | 数量 | AC |
|---|---|---|
| 通过 | 39 | AC-RCL-01-1、01-8、02-4、02-5、03-1、03-2、03-3、03-4、03-6、03-8、03-10、03-11、03-12、03-13、04-3、04-7、04-9，AC-QUO-01-1、01-5、02-2、02-3、02-5、02-6，AC-MEN-01-1、01-5、01-6、01-7、01-8、01-10、01-12、02-4、02-5、03-5、03-6、03-7，AC-CHAT-90-7、90-10、90-15、90-17 |
| 部分通过 | 4 | AC-CHAT-90-8、90-9（图标清单没逐个核对、原生不是 axe、播报不做）；AC-CHAT-90-2、90-5（后端，见后端报告） |
| 未执行 | 2 | AC-MEN-01-9（要人工）；AC-CHAT-90-1（P-12） |
| 失败 | 0 | — |
| 合计 | 45 | |

没有删除或放宽已有断言。iOS 的 `XCTExpectFailure` 去掉后是普通断言；`check_e2e_leftovers.py` 的文件名检查从"计数"改成"有一行就失败"。

## 7. 本次改动

**问题 52（sdk-swift 下载的临时文件）**：`URLSessionObjectStoreTransport.get` 原来用 `URLSession.download(for:)`，取消时 URLSession 自己的 `CFNetworkDownload_*.tmp` 留在临时目录。改成 data task，边收边写 SDK 自己的 `.part-` 文件（只有 2xx 才建），任何失败或取消都关掉并删除，不再有系统的临时文件；顺带下载进度现在也能报上来（原来的 `download(for:)` 不回调进度）。新增 `ObjectStoreDownloadTests` 5 个（真实 socket：中途取消、中途断连、完整下载、403 不写文件、启动前取消），改回旧实现时其中 4 个失败。iOS `testTC_RCL_03_12_…` 去掉 `XCTExpectFailure`，`C4FilesE2ETests` 3 个通过。

**P-45（原生下载地址不带文件名）**：PRD-chat 升 v1.7（只改 F-FILE-03 规则 4、F-FILE-04 规则 1，AC 总数 1598 不变）。`POST /v1/files/urls` 增加可选 `bareDisposition`（布尔，对整批生效，不按 User-Agent 猜），带 `true` 时文件只签 `attachment`、地址里没有文件名，图片仍是 `inline`，不带时和原来一样（H5 不带）。服务端 e2e `TestTC_FILE_01_25_BareDispositionForNative`（查询串原样和解码一次都搜不到文件名和它的百分号编码；`false`、不带时带文件名；非布尔 400 / 4000；去掉实现时失败）；TC-FILE-01-25 加第 3 步；协议 schema 同步到三套 SDK；sdk-kotlin、sdk-swift 每批带 `bareDisposition: true`（各加单测），sdk-ts 不带（加断言）。iOS `check_e2e_leftovers.py` 把"CFNetwork 带文件名的行"改成有一行就失败：`C4FilesE2ETests` 复跑 0 行（改之前同一用例 1 行）。系统 CFNetwork 记完整预签名地址本身（I11）仍在，只是里面不再有文件名。

| 仓库 | 提交 | 内容 |
|---|---|---|
| tianting-sdk-swift | c953dde | 问题 52：下载改 data task 写 `.part-`；`ObjectStoreDownloadTests` |
| tianting-sdk-swift | c97d08e | P-45：`bareDisposition: true`；协议同步 |
| tianting-ios | 4e6ecdb | 问题 52：TC-RCL-03-12 去掉 `XCTExpectFailure` |
| tianting-ios | 62e3355 | P-45：CFNetwork 带文件名的行判失败 |
| tianting-ios | ee522bc | 跨端用例 `CrossPlatformC4E2ETests` |
| tianting-server（im-mvp） | 8527b7b | P-45：服务端 `bareDisposition`、e2e、PRD-chat v1.7、引用和追溯更新、TC-FILE-01-25 第 3 步；问题 52 标已修 |
| tianting-server（im-mvp） | 6c14e2c | 问题 51 标 P-45 已落实 |
| tianting-server（im-mvp） | 本报告所在提交 | 本报告；frontend-chat.md 的 FE-672 进度行 |
| tianting-sdk-kotlin | d6603f5 | P-45：`bareDisposition: true`；协议同步 |
| tianting-sdk-ts | 7739c55 | P-45：协议同步；断言不带 `bareDisposition` |
| tianting-web | 3cd6e96 | 跨端用例 `e2e/xp-c4.spec.ts`，`playwright.xp.config.ts` 匹配它 |
| tianting-android | fd5058a | 跨端用例 `CrossPlatformC4E2e.kt` |
| tianting-android | def3170 | 补跑：`C4E2eTest`、`CrossPlatformC4E2e` 的登录记下用时和点击次数（logcat `TT-E2E-LOGIN`，只有数字），见第 1a 节 |

tianting-ios 的 `Config/Info-Debug.plist`、`Tianting.xcodeproj/project.pbxproj` 里来历不明的改动没有提交，也没有还原；新用例文件在同步文件夹里，不用改 pbxproj。

本次没有发现 C4 的实现错误或口径问题。另记问题 53：iOS `testTC_ACC_03_40_real4290StaysHere`（C1 的用例，AC-ACC-03-37 前半）在本机稳定失败，改动前的 SDK 也失败，原因没有查清（用例、App 退避还是服务端的 [IM 钟] 限流窗口），不属于 C4 范围，没有改。

## 8. 开放项

1. ~~**Android 复跑**~~：已在第 1a 节补跑。`FilesE2eTest` 的两个用例在全量里各失败一次、单独重跑通过（C2 的用例，登录和列表等待 15 秒），记在这里，没有改。
2. **AC-MEN-01-9**：人工用三端系统中文输入法走一遍（真机部分随 P-13）。
3. **C3 恢复后**：AC-RCL-03-9、AC-APN-03-3（BE-481）和推送相关的前端部分。
4. 新消息播报（P-43 ③）、图标清单逐个核对（AC-CHAT-90-8）、性能（P-12）、真机（P-13）、WebKit、云厂商（CH-05）。
5. **问题 53**：iOS TC-ACC-03-40 稳定失败，要另查（不在 C4 范围）。
6. 后端报告里 AC-CHAT-90-10 的"sdk-swift 待确认"已在第 5 节确认，后端报告本身没有改。
7. iOS 系统 CFNetwork 日志里的完整预签名地址（I11）仍然存在，按 P-38 ② 是已接受的残留；文件名部分已由 P-45 去掉。

## 9. 清理

- iOS 模拟器 iPhone 16 Pro（46E6F7B9-…）每次用完都 `xcrun simctl shutdown` 成功，现在是 Shutdown；没有碰 iPhone 18 Pro、iPhone 11。
- Android：第一轮自己起的模拟器进程启动失败后已经退出，emulator-5554 没有碰。补跑时 emulator-5556 两次都用 `adb -s emulator-5556 emu kill` 关掉，进程已退出，`adb devices` 为空；自己起的 `adb logcat` 按 PID 结束。Gradle 只用 2 个 worker，没有停任何 Gradle 进程，没有执行 `./gradlew --stop`。
- 共享服务端和假 S3 用 `test-server.sh stop` 停掉；临时 Postgres 停掉并删除数据目录；没有遗留的 im-server、fake_s3、vite、postgres 进程，用过的端口都已释放。
- scratchpad 里大的产物（Kit 的 DerivedData、临时目录、服务端目录）已删除；本次产生的 iOS `.xcresult` 已删除。共享服务端的 `server.log` 里搜不到 `XP4-` 特征串。
