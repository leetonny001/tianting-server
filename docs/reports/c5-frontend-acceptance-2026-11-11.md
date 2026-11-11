# C5 前端验收报告（FE-686，2026-11-11）

| 项 | 内容 |
|---|---|
| 范围 | frontend-chat.md 里 FE-680～FE-686 的 `- AC：` 行（17 条：AC-CVS-01-6、02-2～02-6、03-2、03-5、03-8、03-10、04-1、04-2、04-5，AC-CHAT-90-8、90-9、90-10、90-15），加上 PRD-chat v1.7 第 15 节【全部】C5 列里挂在后端任务上的 AC-CHAT-90-2（状态引用后端报告），共 **18 条**。后端 AC 见 [c5-backend-acceptance-2026-11-11.md](c5-backend-acceptance-2026-11-11.md)；那份报告里 AC-CVS-03-2、04-1、04-2、04-5 的客户端部分在这里验 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**：H5 和管理后台是"Chrome，本地"；iOS 是"模拟器，本地"（iPhone 16 Pro，46E6F7B9-…）；Android 是"模拟器，本地"（emulator-5556，AVD `Pixel_10_Pro_XL`，`-read-only`，有窗口）。**没有执行**：真机（P-13）、WebKit、原生 CI（P-15）、推送（C3 暂停，BE-490）、新消息播报（P-39 ①） |
| 基线 | 服务端 tianting-server（im-mvp）@b11adaa（main 现在是 79b6250，多出的只有日报文档）、tianting-console-server@57da60f；SDK：tianting-sdk-ts@c9b3127、tianting-sdk-kotlin@ae12932、tianting-sdk-swift@7137fdf（三套都把协议副本同步到 b11adaa，只多了问题 69 的说明文字）；页面：tianting-web@a05c5c5、tianting-android@17b2ee7、tianting-ios@2b06bda、tianting-admin@aa47b1f（C5 没有改动）。本次只新增跨端用例：tianting-web@145d0fe、tianting-android@b81c592、tianting-ios@b80cd93（第 7 节），没有改实现 |
| 结论 | **通过 14、部分通过 4、未执行 0、失败 0**（第 5、6 节）。跨端 6 个方向都通过，三个接收端各做了反证（第 2 节） |

## 1. 本次运行

开始时磁盘可用 62 GiB，结束时 62 GiB。各平台按顺序跑；只有 Android↔iOS 两个跨端方向要两台设备同时在线（同一用户的 D1、D2 分在两端，第 2 节），其余时间同一时刻只开一台设备。

**FE-686 由两段完成**：第一段（被中途停掉）跑完了大部分全量，日志在会话 scratchpad 的 `fe686/`；第二段（本次）逐个核对这些日志是不是完整、是不是在上表的 HEAD 上跑的，能用的直接采用，其余重跑，日志在 `fe686r/`。下表"来源"一栏写明。两个目录都在 `/private/tmp/claude-501/-Users-frank-Desktop-workspace/0f7162ba-5fa9-4705-8b51-f8909b58f7e9/scratchpad/` 下。

| 端 | 命令（本地） | 结果 | 来源 |
|---|---|---|---|
| sdk-ts | `typecheck`、`lint`、`check:protocol`、`format:check`；`vitest run --project unit`；`--project integration`（真实 im-server，`TIANTING_SERVER_DIR=im-mvp/server`） | 检查都通过；单测 870 个：868 过、2 跳过（原有，冷启动，H5 没有持久化）；集成 69 过 | **重跑**（`fe686r/ts.log`、`ts-it.log`）。第一段的日志早于 c9b3127 的提交时间，判断不了是不是 HEAD |
| sdk-kotlin | `./gradlew --max-workers=2 --no-build-cache :sdk-core:check :sdk-core:test --rerun :sdk-core:integrationTest --rerun :sdk-android:testDebugUnitTest --rerun` | sdk-core 单测 1134 过；集成 88 过；sdk-android 6 过；`check` 通过 | **重跑**（`fe686r/kt.log`）。第一段的 `sdk-core:test` 是 FROM-CACHE，看不出数量 |
| sdk-swift | `swift test` | 单测 531 过；集成 89 过 | **重跑**（`fe686r/swift.log`）。第一段的日志只有集成那一段 |
| H5 | `typecheck`、`lint`、`format:check`、`npm run check`；`vitest run`；`build`、`check-dist`、`check-assets`、`check-bundle`；`npx playwright test`（全量，真实 im-server） | 检查通过（对比度 68 组、颜色都来自 token、首屏 157.5 KB gzip）；单测 381 过；e2e 203 个：197 过、6 跳过（原有：要单独环境变量的 TC-ACC-03-12、03-45 和没有运行号的跨端用例）。`c5.spec.ts` 21 个在里面全过 | **采用**第一段（`fe686/h5-*.log`，04:12～04:38，tianting-web@a05c5c5、sdk-ts@c9b3127） |
| 管理后台 | `typecheck`、`lint`、`format:check`；`vitest run`；`npx playwright test`（临时 Postgres `initdb -E UTF8 --locale=C`、`lc_messages=C`，+ im-server + console-server） | 单测 125 过；e2e 61 个：58 过、3 跳过（截图巡检 2 个、邀请链路 1 个，要单独开关） | **采用**第一段（`fe686/adm-*.log`；aa47b1f 自 11-10 没变）。临时库第一段已停掉并删除了数据目录（本次核对目录已不在、没有 postgres 进程） |
| Android（不要设备） | `./gradlew --max-workers=2 checkTianting :app:lintDebug :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest` | 通过；单测 69 过 | **采用**第一段（`fe686/and-check.log`，05:11） |
| Android ui | `scripts/run-tests.sh ui` | **294 全过** | **采用**第一段（`fe686/and-ui.log`） |
| Android e2e | `scripts/run-tests.sh e2e`（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`、`IM_CONV_SETTINGS_ENABLED=1`） | 77 个：75 过、2 按设计跳过（TC-ACC-03-12、03-45 要单独的 `IM_ACCESS_TTL`、`IM_REFRESH_ABSOLUTE`），`C5E2eTest` 全过 | **采用**第一段（`fe686/and-e2e.log`、`and-e2e-instrument.txt`） |
| Android kill | `scripts/run-tests.sh kill`（7 个场景） | **本次全量**：6 个过，`FilesKillE2e.uploadKilled` 第二阶段 App 进程崩溃 1 次（`Detected multithreaded access to SnapshotStateObserver`，第 8 节新问题 75）；单独连跑 `FilesKillE2e.uploadKilled`、`C4KillE2e.recallWhileKilled` 3 遍共 6 次都过。`C5KillE2e.deletedWhileKilled` 过 | **重跑**（`fe686r/and-kill.log`、`kill-rerun-{1,2,3}.log`、`kill-logcat.txt`）。第一段同一个崩溃出现在 `C4KillE2e` 上（全量 1 次、单独 2 次里 1 次，`fe686/and-kill.log`、`c4kill-*.log`），没有留下堆栈 |
| iOS Kit 单测 | `xcodebuild test`（TiantingAppKit） | 161 个：157 过、4 原有跳过 | **采用**第一段（`fe686/ios/kit.log`） |
| iOS 检查 | `check_sources`、`check_tokens`、`gen_tokens --check` | 通过（62 组对比度） | **采用**第一段（`fe686/ios/checks.log`）；提交前 `check_sources`、`check_tokens` 又跑一次通过 |
| iOS XCUITest 全量 | `scripts/run-e2e.sh --device 46E6F7B9-…`（真实 im-server） | 199 个：188 过、9 按设计跳过（跨端用例没有运行号等）、2 个失败：`AccountE2ETests.testTC_ACC_03_40_real4290StaysHere`（**问题 53，已知稳定失败**，本次单独重跑仍失败）；`FilesE2ETests.testImageSaveToPhotosAndRefusedPermission`（"gallery-denied" 等不到，**本次单独重跑通过**，偶发，FE-685 时也偶发过一次）。`check_e2e_leftovers`：**CFNetwork 带文件名的行 0**、带预签名地址的行 13（I11）、辅助功能日志带特征串 0、数据目录没有删除的原文 | 全量**采用**第一段（`fe686/ios/full.log`，08:30～09:26，tianting-ios@2b06bda + 未提交的跨端用例文件、sdk-swift@7137fdf）；两个失败用例**本次单独重跑**（`fe686r/ios/single-*.log`，CFNetwork 带文件名都是 0） |
| 跨端 | 第 2 节 | 6 个方向通过，3 个接收端反证都按预期失败 | **重跑**（`fe686r/xp/`）；第一段只做完 4 个方向，Android→iOS 被中断，iOS→Android 没跑 |
| 问题 64 核对 | `fe686r/xp/q64.mjs`（真实 im-server） | 第 8 节 | 本次 |
| 追溯 | `python3 -I tools/trace.py --check --summary` | AC 1635，gaps 0 | 本次（报告和任务单改完后） |

GitHub Actions 没有手动触发（push 时自动排的没有等）。

## 2. 跨端互发互收（AC-CVS-04-1、04-5、03-2、04-2 的客户端部分）

C5 的多端同步要同一用户的两台设备**同时在线**，所以和 C4 的"先发后收"不同：D1（操作端）、D2（观察端）分在两端同时跑，靠协调脚本的"信号板"对齐步骤。约定写在三个用例文件开头（tianting-web `e2e/xp-c5.spec.ts`、tianting-android `CrossPlatformC5E2e.kt`、tianting-ios `UITests/CrossPlatformC5E2ETests.swift`，三份同一段）：

- 协调脚本（scratchpad `fe686r/xp/coord5.mjs`）用管理接口建每个方向一个用户 U 和 3 个对方，群 G；预置：G 里 pa 一条、C（pc，含特征串）、B（pb，含特征串）、A（pa 两条，未读 2）。共享服务端是 tianting-android `scripts/test-server.sh start`（真实 im-server，`IM_CONV_SETTINGS_ENABLED=1`、`IM_MSG_ACTIONS_ENABLED=1`，SQLite，假 S3）。
- **D2**：打开 C、在 B 写草稿；收到 pin / mute / del 信号后 **2 秒内**（从 D2 拿到信号算起，D1 先贴信号再点，D1 的点击也算在 2 秒里）A 在最前且置顶、A 免打扰（图标、小红点、没有数字、"[2 条]"）、B 从列表消失（Android 另外断言草稿 2 秒内清除）；pb 再发一条后 B 重新出现、**未读 1**、摘要是新消息、没有 [草稿]，打开只有这 1 条、输入框是空的；然后断网（H5 断 WebSocket、Android 飞行模式、iOS 断中转），D1 置顶 G、删除 C；恢复后顺序、置顶、免打扰和 D1 贴的一样（G、A、B），C 不在列表、打开没有消息（Android 本地库文件、iOS 数据目录都搜不到 C 的特征串）。
- **D1**：长按（H5 右键）菜单操作；B 删除后 1 秒仍不在；B 重新出现时**自己这边也是未读 1**；D2 断网时置顶 G、删除 C，把自己列表的状态贴给 D2。

| D2（接收）\ D1（操作） | H5 | Android | iOS |
|---|---|---|---|
| H5 | — | 通过（c5270c） | 通过（c56cdc） |
| Android | 通过（c5270c） | — | 通过（c5270c） |
| iOS | 通过（c56cdc） | 通过（c5270c） | — |

2 秒内的实测（D2 端记录）：iOS 收 H5 72～141 ms、收 Android 269～460 ms；Android 收 H5 201～480 ms、收 iOS 762～892 ms（含草稿清除）。

**反证**（协调脚本 `--fake-sender`：这个方向没有 D1，D2 贴 d2-ready 1 秒后协调脚本自己贴 pin，实际没有人置顶）：H5 收（c5a21a，ios2h5）失败在"A 在最前且置顶"；iOS 收（c5a21a，h52ios）失败在"not within 2 seconds: A on top and pinned"；Android 收（c55d16，h52android）失败在"2 秒内没有：A 在最前且置顶"。三个接收端都不会空过。

顺序：c51469、c510ed（第 7 节的两轮修正）→ c56cdc：H5→iOS、iOS→H5 → c5a21a：H5、iOS 反证 → 开 Android 模拟器 → c5270c：Android→iOS、iOS→Android（两台同时在线）→ 关 iOS 模拟器 → H5→Android、Android→H5 → c55d16：Android 反证。共享服务端的 `server.log` 里搜不到 `XP5`、`ZXQ` 特征串。

局限：H5 是同一台机器上的 Chrome；所有设备都是模拟器。

## 3. AC-CHAT-90-8、90-9（会话菜单、置顶背景、免打扰图标、删除确认框）

| 项 | H5 | Android | iOS |
|---|---|---|---|
| 自动检查 | `c5.spec.ts` TC-CHAT-90-36（axe：会话菜单、置顶背景、免打扰图标、删除确认框，浅色、深色），全量通过 | `C5UiTest.tc_chat_90_36_37_c5_pages_named_light/dark`（语义树的等价检查），ui 全量通过 | `C5E2ETests` 的系统审计（`.sufficientElementDescription`，浅色、深色），全量通过 |
| 键盘 / 辅助操作 | TC-CHAT-90-37 键盘走查（Tab 到会话项、菜单键打开、方向键、回车）通过 | `tc_chat_90_37_accessibility_actions_pin_notify_delete`（辅助操作三项）通过 | VoiceOver"操作"三项（`C5E2ETests`）通过 |
| 颜色来自 token | `npm run check` 通过（`color.surface.pinned`） | `checkTianting` 通过 | `check_tokens`、`check_sources` 通过 |
| 角标可读名称（问题 67） | "消息，5 条未读"（FE-683 修） | 导航项合并语义里有"消息"和角标数字（读得到数字，措辞不是"N 条未读"），没有单独断言 | "消息，5 条未读"（FE-685） |
| 新消息播报 | 不做（P-39 ①） | 不做 | 不做 |

H5 的 `check-assets` 保证 SVG 只来自图标清单；原生两端的图标没有逐个和 PRD-ui 6.6 节核对，原生是等价检查不是 axe。所以两条都记**部分通过**，口径同 C4。

## 4. 未执行、等 C3 的

- **AC-CVS-02-6**（Android、iOS）：前台收到免打扰会话的消息，列表更新、通知替身没有调用、没有系统通知（Android `C5UiTest.tc_cvs_02_06_…`、iOS `testTC_CVS_02_06_…`）都通过。三端现在没有任何提示音和系统通知代码（问题 58），"不响铃不弹通知"要等 C3 加了通知后由 FE-653、FE-655 补跑 TC-CVS-02-06（P-46）。记**部分通过**。
- **推送相关的 AC**（AC-CVS-02-1 的推送一半、02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4）：后端，挡在 C3 的 BE-490，按 P-46 记"未执行（等 C3）"，见后端报告，不计入本报告。
- **新消息播报**：按 P-39 ① 仍是已知缺口。
- **AC-CHAT-90-2** 的 TC-PERF：等 P-12（后端报告）。

## 5. 逐条结果

"依据"写用例；没写提交的是表头"基线"里的提交。

| AC | 层级 | 状态 | 依据 | 说明 |
|---|---|---|---|---|
| AC-CVS-01-6 | 前端 | 通过 | H5 `c5.spec.ts` TC-CVS-01-06（窄屏长按、宽屏右键、菜单键，开关关闭时没有菜单）；Android `C5UiTest.tc_cvs_01_06_*`；iOS `testTC_CVS_01_06_menuPinMovesToTopWithBackground`、`testTC_CVS_01_06_noMenuWithTheSwitchOff` | 三项按状态，置顶背景 `color.surface.pinned`；跨端第 2 节三端都用自己的菜单操作 |
| AC-CVS-02-2 | 前端 | 通过 | H5 TC-CVS-02-02（角标 5，标题"(5) 天听"）；Android `tc_cvs_02_02_*`；iOS `testTC_CVS_02_02_TC_CVS_02_03_*` | |
| AC-CVS-02-3 | 前端 | 通过 | H5 TC-CVS-02-03；Android `tc_cvs_02_03_*`；iOS 同上；跨端第 2 节（D2 的"[2 条]"、小红点、图标、没有数字） | |
| AC-CVS-02-4 | 前端（H5） | 通过 | H5 TC-CVS-02-04（标题"天听"） | |
| AC-CVS-02-5 | 前端 | 通过 | H5 TC-CVS-02-05（角标 3）；Android `tc_cvs_02_05_*`；iOS `testTC_CVS_02_05_*` | |
| AC-CVS-02-6 | 前端（Android、iOS） | 部分通过 | Android `tc_cvs_02_06_*`；iOS `testTC_CVS_02_06_*` | 列表更新、没有通知通过；通知部分 C3 后补跑（第 4 节，P-46） |
| AC-CVS-03-2 | 端到端 | 通过 | 后端部分见后端报告；H5 `c5.spec.ts` TC-CVS-03-02；Android `C5E2eTest.tc_cvs_03_02_*`；iOS `testTC_CVS_03_02_backWithOnlyTheNewMessage`；三套 SDK 集成；**跨端第 2 节 6 个方向**（D1、D2 都是未读 1，D2 只有这 1 条） | 本地搜不到旧特征串：H5 页面文本、Android 本地库、iOS 数据目录 |
| AC-CVS-03-5 | 前端（三套 SDK） | 通过 | `chat-sync.json` C5 用例（sdk-ts `chat-sync-vectors`、sdk-kotlin `ChatSyncVectorsTest` 内存和 SQLite、sdk-swift `ChatSyncVectorTests`）；三套集成 TC-CVS-03-05；"从通讯录进入可以发言"：H5 删除群后从"我的群聊"进入、Android `f_cvs_03_rule_4_*`、iOS `testEmptyGroupFromMyGroupsIsAnEmptyChatPage`（问题 66 两端都检查了） | 本次三套 SDK 全量重跑 |
| AC-CVS-03-8 | 前端 | 通过 | H5 TC-CVS-03-08（真实 convDelete 和改写帧两个）；Android `C5E2eTest.tc_cvs_03_08_*`、`C5UiTest.tc_cvs_03_08_*`；iOS `testTC_CVS_03_08_*` | 草稿、失败消息、缓存清除，取消时不变 |
| AC-CVS-03-10 | 前端（sdk-kotlin、sdk-swift） | 通过 | sdk-kotlin `ConvSettingsIntegrationTest`（TC-CVS-03-10 字节搜索）、sdk-swift 集成；页面：Android `C5KillE2e.deletedWhileKilled`（本次 kill 全量通过）、iOS `testTC_CVS_03_10_coldStartOnlyNewNothingOldOnDisk` | |
| AC-CVS-04-1 | 端到端 | 通过 | 后端部分见后端报告；各端 TC-CVS-04-01（H5 两个标签页、Android、iOS 对同进程的 SDK）；**跨端第 2 节 6 个方向，2 秒内** | |
| AC-CVS-04-2 | 端到端 | 通过 | 各端 TC-CVS-04-02；**跨端第 2 节 6 个方向**（D2 断网时 D1 置顶并删除，恢复后和 D1 一致） | |
| AC-CVS-04-5 | 端到端 | 通过 | 各端 TC-CVS-04-05；**跨端第 2 节 6 个方向**（2 秒内消失，草稿清除；本地消息清除见 04-2 那一步的 C） | |
| AC-CHAT-90-8 | 前端【全部】 | 部分通过 | 第 3 节 | 原生图标没有逐个核对清单 |
| AC-CHAT-90-9 | 前端【全部】 | 部分通过 | 第 3 节 | 原生是等价检查不是 axe |
| AC-CHAT-90-10 | 前端【全部】（三套 SDK） | 通过 | `frames.json` C5 的 51 个用例：sdk-ts `frames-schema.test.ts`、sdk-kotlin `VectorsTest`、sdk-swift `VectorTests`（本次全量重跑）；服务端见后端报告 | |
| AC-CHAT-90-15 | 前端【全部】（三套 SDK） | 通过 | `chat-sync.json` 的 C5 用例 6 个（三套执行器，本次重跑） | |
| AC-CHAT-90-2 | 后端【全部】 | 部分通过 | 后端报告 | TC-PERF 等 P-12 |

## 6. 汇总

| 状态 | 数量 | AC |
|---|---|---|
| 通过 | 14 | AC-CVS-01-6、02-2、02-3、02-4、02-5、03-2、03-5、03-8、03-10、04-1、04-2、04-5，AC-CHAT-90-10、90-15 |
| 部分通过 | 4 | AC-CVS-02-6（通知部分等 C3）；AC-CHAT-90-8、90-9（原生图标清单没逐个核对、原生不是 axe、播报不做）；AC-CHAT-90-2（后端，P-12） |
| 未执行 | 0 | — |
| 失败 | 0 | — |
| 合计 | 18 | |

后端报告里 AC-CVS-03-2、04-1、04-2、04-5 的"客户端部分随 FE-686"本次补齐（三端用例 + 跨端 6 个方向），这四条可以改为通过；后端报告本身没有改。

没有删除或放宽已有断言。

## 7. 本次改动

**没有改实现**。三端各提交了跨端用例（第一段写好、本次核对后修正两处再跑通）：

| 仓库 | 提交 | 内容 |
|---|---|---|
| tianting-web | 145d0fe | `e2e/xp-c5.spec.ts`；`playwright.xp.config.ts` 匹配它 |
| tianting-android | b81c592 | `CrossPlatformC5E2e.kt`；`scripts/run-tests.sh` 加 `TT_XP_COORD_PORT`（`adb reverse` 信号板端口，飞行模式下也通） |
| tianting-ios | b80cd93 | `UITests/CrossPlatformC5E2ETests.swift`；`scripts/run-e2e.sh` 传 `TT_E2E_XP_COORD` |
| tianting-server（im-mvp） | 本报告所在提交 | 本报告；frontend-chat.md 的 FE-686 进度行、问题 64、70、73 的结论、新问题 75 |

核对时修的两处（都是用例写法，断言不变）：

1. **iOS 读列表的竞态**：第一段 Android→iOS 失败在"No matches found for Element at index 3"——`labels()` 先取行数再逐个按序号取，C 恰好在中间被删除。改为从一次树快照里读所有行的 label（取不到快照时当作空列表，外面的 `until` 会再看），断言不变。
2. **D1 的"未读 1"和 D2 打开会话的竞态**：c510ed 里 iOS→H5 的 iOS（D1）稳定看到 B 回来但没有未读。原因是 D2 一看到 B 回来就打开它，已读同步到 D1（同一用户多端已读同步，是正确行为），D1 的"未读 1"只剩一瞬间；第一段 c59ff2 那一轮能过是碰巧。约定加一个信号：D1 看到"B 回来、未读 1"后贴 `d1-back`，D2 等到它再打开 B。D1、D2 的"未读 1"断言都保留（iOS 把"等摘要"和"等未读 1"合成一个等待条件，再断言未读 1）。改完后 6 个方向都通过。

tianting-ios 的 `Config/Info-Debug.plist`、`Tianting.xcodeproj/project.pbxproj` 两处脏文件没有提交，也没有还原；远端没有新提交，直接推送。

## 8. 问题

**问题 73 的结论**（F-RCL-01 规则 1 的撤回时限要不要跟随服务端配置）：**PRD 不要求**。依据：F-RCL-01 规则 1 把"120 秒内"写成产品规则；AC-RCL-01-8 要求客户端按 `ts` 和校正后的服务端时间判断"60 秒内有、121 秒后没有"，并规定了菜单打开期间过了时限时按 4097 提示；错误码表 4097 的文案固定为"发送时间超过 2 分钟"（群主、管理员"7 天"）；`IM_RECALL_WINDOW_SEC` 只在第 4.1 节的服务端配置表里，`GET /v1/config` 的 `limits`（http-client.schema）不下发它，客户端没有渠道知道改过的值。所以客户端写死 120 秒符合 PRD；服务端改了这个配置时菜单和文案都会和服务端不一致，只靠 4097 兜底。不改代码。若以后要求可配，需要 PRD 增加下发字段（例如 `limits.recallWindowSec`）并改 4097 文案，属于新需求，现在不用 Frank 决定。

**问题 64 的核对**（删除后会话因本地有更新的消息没有消失，提及落在清除范围内）：真实 im-server 上群里 @ 我（seq 2）后又来两条，`convDelete upToSeq=2`：ack 只带 `clearedSeq`，不带会话；**紧接着的 `convs` 立即给出正确值**（`unread=2`、`mentionUnread=0`、没有 `firstMentionSeq`、`minSeq=3`）。服务端没有问题；客户端在下次 `convs` 之前多显示"[有人@我]"的小缺口仍按问题 64 记录。

**问题 70**：本次没有专门构造"隐藏的群在离线期间被移出、之后没再加回，再从通讯录进入"的场景，没有核对，仍是记录状态。

**新问题 75（Android，不是 C5）**：kill 用例第二阶段（冷启动新进程）App 偶发崩溃 `IllegalArgumentException: Detected multithreaded access to SnapshotStateObserver: previousThreadId=47, currentThread=main`，堆栈在主线程的 `AndroidComposeView.dispatchDraw → GraphicsLayerOwnerLayer.updateDisplayList → NodeCoordinator drawBlock → OwnerSnapshotObserver.observeReads`（`fe686r/kill-logcat.txt` 第 20203 行起）。出现在 C2 的 `FilesKillE2e.uploadKilled` 和 C4 的 `C4KillE2e.recallWhileKilled`：本次全量 1 次 / 单独 6 次都过；第一段全量 1 次、单独 2 次里 1 次。C5 的 `C5KillE2e` 没有出现。FE-684 时两次 kill 全量都过，之后 App 只改过测试，SDK 只同步了协议说明，判断是原有的偶发问题。线程 47 是谁在主线程之外做了绘制或测量没有查清（kill 用例本身没有离屏截图），没有改。建议另开任务：在 phase2 打开 `Thread` 名字日志或用 `StrictMode`，先确认是 App 代码还是测试框架。

本次没有发现 C5 的实现错误，没有新的待 Frank 决定的事。

## 9. 开放项

1. **C3 恢复后**：AC-CVS-02-6 的通知部分（FE-653、FE-655 补跑 TC-CVS-02-06），推送相关的后端 AC（BE-490）。
2. 新消息播报（P-39 ①）、原生图标清单逐个核对（AC-CHAT-90-8）、性能（P-12）、真机（P-13）、WebKit。
3. **问题 53**：iOS TC-ACC-03-40 稳定失败，本次单独重跑仍失败（不在 C5 范围）。
4. **问题 75**：Android kill 第二阶段偶发崩溃，要另查。
5. 问题 64 的客户端小缺口、问题 70 没有核对。
6. 后端报告里 AC-CVS-03-2、04-1、04-2、04-5 可以改为通过（客户端部分见第 2、5 节）。

## 10. 清理

- iOS 模拟器 iPhone 16 Pro（46E6F7B9-…）用完 `xcrun simctl shutdown` 成功，现在是 Shutdown；没有碰 iPhone 18 Pro、iPhone 11，也没有碰 PID 96273。
- Android：emulator-5556 由本次启动（只读、有窗口），用完 `adb -s emulator-5556 emu kill`，进程已退出，`adb devices` 为空；自己起的 `adb logcat` 按 PID 结束。Gradle 只用 2 个 worker，没有执行 `./gradlew --stop`，没有停任何 Gradle 进程。
- 第一段共享服务端留下的假 S3（PID 75559，占着 19200）用同一状态目录的 `test-server.sh stop` 停掉；本次的共享服务端、假 S3、协调脚本都已停掉，用过的端口（18280～18282、19200、18299、15373）都已释放。
- 临时 Postgres（第一段）已停，数据目录已不在，没有 postgres 进程。scratchpad 里的服务端二进制和库、iOS DerivedData、所有 `.xcresult` 已删除，只保留日志。
