# C2 前端验收报告（FE-640，2026-11-09）

| 项 | 内容 |
|---|---|
| 范围 | frontend-chat.md 里 FE-630～FE-640 的 `- AC：` 行（42 条），加上 PRD-chat v1.4 第 15 节其余【全部】的 AC（6 条，后端的，状态引用后端报告），共 48 条。后端 AC 见 [c2-backend-acceptance-2026-11-09.md](c2-backend-acceptance-2026-11-09.md)，这里不重复 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**：H5 是"Chrome，本地"；Android 是"模拟器，本地"（emulator-5556，AVD Pixel_10_Pro_XL，API 37，`-read-only`）；iOS 是"模拟器，本地"（iPhone 16 Pro，iOS 18.4）。**没有执行**：真机（P-13）、WebKit 和其他浏览器（P-13，不安装）、原生 CI（P-15）、云厂商（CH-05）。对象存储都是假 S3：**只验调度，不证明厂商行为** |
| 基线 | 页面代码：tianting-web@bdf0cd8、tianting-android@d263b6b（本次只改了一处图片的可读名称，见第 3 节）、tianting-ios@52c6f2f；SDK：tianting-sdk-ts@4f4312d、tianting-sdk-kotlin@1f5eccb、tianting-sdk-swift@662e4f2；后台 tianting-admin@4b18297；服务端 im-mvp@43eea52。本次新增用例：tianting-web@12ee72e、tianting-android@79893a7、tianting-ios@7f1b997 |
| 结论 | **通过 35、部分通过 10、未执行 3、失败 0**（第 6 节；2026-11-10 BE-403 之后 AC-CHAT-90-10、90-15 由部分通过改为通过）。AC-FILE-02-1、03-1 三端互发互收 6 个方向都通过（第 2 节） |

## 1. 本次运行

各平台按顺序跑，Android 模拟器和 iOS 模拟器没有同时开。开始时磁盘可用 73 GiB、进程 698 个；结束时 72 GiB、630 个。

| 端 | 命令 | 结果 |
|---|---|---|
| 共享服务端 | `TT_S3_PORT=19200 TT_S3_ORIGINS=http://127.0.0.1:15373 tianting-android/scripts/test-server.sh start`（真实 im-server，`IM_FILES_ENABLED=1`，SQLite，假 S3 `scripts/fake_s3.py`）；scratchpad 里的脚本用管理接口建 3 个用户和 1 个群（运行号 `r2325`）；跑完 `test-server.sh stop` | 三端轮流连它 |
| H5 发 | `TT_E2E_PORT_OFFSET=200 TT_E2E_ADMIN_KEY=… TT_XP=r2325 TT_XP_PHASE=send npx playwright test -c playwright.xp.config.ts` | 1 过（另一条按阶段跳过） |
| Android 发、收 H5 | `ANDROID_SERIAL=emulator-5556 TT_SERVER_RUNNING=1 TT_XP=r2325 scripts/run-tests.sh e2e com.tianting.app.CrossPlatformE2e#xp_send`；再加 `TT_XP_FROM=h5` 跑 `#xp_receive` | 各 1 过 |
| Android 反证 | 同上，`TT_XP_FROM=ios`（iOS 还没发） | 1 失败（"超时：收到 ios 的图片"），证明接收用例不会空过 |
| Android 页面 | `scripts/run-tests.sh class com.tianting.app.FilesUiTest`（新增 2 个无障碍用例） | 第 1 次 17 个里 2 个失败（发现问题，见第 3 节），修了以后 17 过 |
| iOS 发、收 | `TT_E2E_EXTERNAL=1 TT_E2E_HTTP=http://localhost:18280 … TT_E2E_XP=r2325 scripts/run-e2e.sh --device 46E6F7B9-… --only CrossPlatformE2ETests/testXP1_send --only …/testXP2_receive` | 发 1 过；收第 1 次失败（用例自己的问题：脚本把没设的 `TT_E2E_XP_FROM` 传成空串，改成空串按默认），重跑收 1 过；`check_e2e_leftovers` 两次都通过（CFNetwork 带签名的系统日志 0 行，I11；辅助功能日志 0 行，I12） |
| H5 收 | `… TT_XP_PHASE=receive npx playwright test -c playwright.xp.config.ts` | 1 过 |
| Android 收 iOS | 重新启动模拟器，`TT_XP_FROM=ios` 跑 `#xp_receive`；再不带 `TT_XP_FROM`（收 H5、iOS 两个）跑 1 次 | 各 1 过 |
| Android 回归（改了页面代码） | `TT_SERVER_RUNNING=1 scripts/run-tests.sh e2e com.tianting.app.FilesE2eTest`；`./gradlew --max-workers=2 checkTianting :app:lintDebug :app:testDebugUnitTest` | `FilesE2eTest` 17 过；单测 41 过；lint、`checkTianting` 通过 |
| iOS 检查 | `xcodebuild build-for-testing`；`check_sources.py`、`check_tokens.py`、`gen_tokens.py --check` | 通过 |
| H5 检查 | `npm run typecheck`、`lint`、`format:check`、`check`（颜色、对比度、品牌） | 通过 |
| 追溯 | `python3 -I tools/trace.py --check --summary` | gaps 0 |

**没有重跑的**：三端全量回归、三套 SDK 全量。H5、iOS、三套 SDK 本次只加了用例（没有改产品代码），所以沿用进度表里各任务的结果（同一个提交）；Android 改了一行页面代码，重跑了文件相关的两组（上表）。GitHub Actions 没有触发。

## 2. AC-FILE-02-1、03-1 三端互发互收

做法："一端发、另一端按消息拉取并下载校验字节"，三端不同时运行，连同一个服务端和假 S3。

- **发**（都走各端自己的界面和图片处理）：每端给另外两端各发一张 1600×1200 的纯色 jpeg（单聊；H5 红、Android 绿、iOS 蓝），往三人群里发一个 `跨端-{端}.pdf`（1 MiB 多一点，内容按公式生成，三端同一个公式）。
- **收**（各端自己的界面 + 自己的 SDK）：单聊里对方的图片先显示缩略图、点开显示原图；消息里 `width=1600`、`height=1200`、`mime=image/jpeg`；下载的原图字节数等于 `size`，解出来 1600×1200、中心是发送方的颜色；缩略图长边 ≤ 480、颜色一致。群里对方的文件名、大小、`application/pdf` 对，界面点下载（第一次有来源提示）后，下载内容的 SHA-256 等于按公式算出的值。

| 收 \ 发 | H5 | Android | iOS |
|---|---|---|---|
| H5（sdk-ts，页面内存；原图在全屏查看器里取颜色；文件走浏览器下载） | — | 通过 | 通过 |
| Android（sdk-kotlin，App 自己的 `IMClient` 下载和缓存） | 通过 | — | 通过 |
| iOS（界面是 App；字节用测试进程里的 sdk-swift 以同一个用户登录下载；文件 SHA 是 App 自己下载的） | 通过 | 通过 | — |

用例：`tianting-web/e2e/xp-files.spec.ts`、`tianting-android/.../CrossPlatformE2e.kt`、`tianting-ios/UITests/CrossPlatformE2ETests.swift`，同一段约定写在三个文件开头。没有运行号（`TT_XP` / `ttXp` / `TT_E2E_XP`）时跳过，普通的 e2e 不受影响。为了让三端连同一个服务端，加了三个开关：Android `run-tests.sh` 的 `TT_SERVER_RUNNING=1`、`test-server.sh` 的 `TT_S3_ORIGINS`；iOS `run-e2e.sh` 的 `TT_E2E_EXTERNAL=1`；H5 的 `playwright.xp.config.ts`（只起开发服务器）和 `TT_E2E_ADMIN_KEY`。

局限：
- 不是"同时"在线互发，是先发后收；实时推送的到达在各端自己的 TC-FILE-02-01（对方是同平台的 SDK）里验过。
- iOS 的图片字节是测试进程里的 sdk-swift 下载的（XCUITest 读不到 App 进程的内存），App 自己的显示只验了"缩略图已加载、能打开原图"。
- 只有 jpeg 和 pdf。HEIC、实况照片属于 AC-FILE-02-10（真机）。

## 3. AC-CHAT-90-8、90-9（文件页面）

| 项 | H5 | Android | iOS |
|---|---|---|---|
| 自动检查 | axe-core：图片气泡、失败提示、"+"菜单、全屏查看、来源提示、头像裁剪框（浅色、深色），修过 1 处 `label-content-name-mismatch`（FE-634） | **本次新增** `FilesUiTest.tc_chat_90_08_files_pages_named_light/_dark`：聊天页（图片、文件气泡）、"+"面板、全屏查看、来源提示、发送失败，每个窗口的能点节点都要有名字（`button-name` 的等价检查，没有 espresso-accessibility，A10） | 系统审计 `performAccessibilityAudit` 的 `.sufficientElementDescription`（FE-638） |
| 替代文本 | 图片"图片"，文件读文件名和大小 | 图片"图片"，文件"文件 报告.pdf，2 KB"（本次断言） | 图片"图片"，文件"文件 文件名，大小"，"+"是"发送图片或文件" |
| 深色截图 | 有（axe 用例浅色、深色） | **本次新增**，浅色、深色各 5 张，人工看过深色"发送失败"一张：气泡、失败图标、提示可读 | 有（FE-638） |
| 颜色来自 token | `npm run check` 通过 | `checkTianting` 通过 | `check_tokens`、`check_sources` 通过 |
| 键盘 | 全屏查看 Esc 关闭、来源提示键盘下载（FE-634） | 不适用 | 不适用 |
| 新消息播报（"收到一张图片"） | **未执行**：没有实现 | **未执行**：没有实现（A29） | **未执行**：没有实现 |

**本次发现并修了一处**：Android 已加载的缩略图是能点的（打开全屏），但它自己没有名字（"图片"在外框上，外框不是能点的节点），读屏会读成没有名字的按钮。新用例第一次运行在这里失败（浅色、深色都是）；改成能点的缩略图本身也读"图片"（tianting-android@79893a7，`Attachments.kt` 一行），之后 17 个全过，`FilesE2eTest` 17 个也过。

AC 的"新增图标都在清单里"（B-3、AC-UI-11-3）只由各端已有的检查覆盖（H5 `check-brand`，原生没有单独的图标清单检查），本次没有逐个核对。所以 90-8、90-9 都记**部分通过**。

## 4. 未执行的原因

- **AC-FILE-02-10**（手工）：要 Android、iOS 真机相册里的 HEIC 和实况照片，H5 拖入 HEIC（Chrome 只有 Vitest 替身）。真机在 P-13，开发机没有。
- **AC-FILE-90-2**（手工）：要 Frank 选定的云厂商（CH-05），上线前跑合同测试，并用生产 CORS、CSP 走查三端上传下载。现在没有厂商、没有生产配置。AC-FILE-90-1 已在真实 MinIO 上通过（后端报告第 3 节），不能代替。
- **AC-CHAT-90-1**：性能，等目标机（P-12），见后端报告。

**AC-CHAT-90-15、90-10（2026-11-10 更新，BE-403）**：原来记部分通过，因为 `protocol/vectors/` 没有 `chat-sync.json`，`frames.json` 没有 `image`、`file`、`MsgBody` 的用例（D10）。BE-403 补上了（tianting-server@6c0829f）：`chat-sync.json` 6 个用例（`expired` 单调合并、`modVer` 重拉，Go 参考客户端 `server/e2e/chatmodel` 生成），`account-session.json` 加附件状态机 11 个用例（F-FILE-07 规则 1、3、4），`frames.json` 加 11 个合法、18 个非法用例。服务端和三套 SDK 都跑并通过（**本地结果，CI 不运行**；sdk-ts 跳过 2 个冷启动用例，H5 没有持久化）。按第 15 节累计表 C2 的范围，两条都记**通过**。

## 5. 逐条结果

"依据"写用例和仓库提交；没写提交的就是表头"基线"里的提交。

| AC | 层级 | 状态 | 依据 | 说明 |
|---|---|---|---|---|
| AC-ACC-06-6 | 前端 | 通过 | H5 Playwright（FE-633）、Android `FilesE2eTest.tc_acc_06_06_…`、iOS `testTC_ACC_06_06_…` | 512×512 jpeg，`kind=avatar`；本次登录显示本地图片（H13、A30） |
| AC-FILE-01-12 | 前端 | 通过 | TC-FILE-01-12：三套 SDK 单测，H5 Playwright，Android `FilesUiTest`，iOS `testTC_FILE_01_12_quotaExceeded` | 真实 4292 在 sdk-ts 集成测试 |
| AC-FILE-01-17 | 前端 | 通过 | TC-FILE-01-17：三套 SDK，三端 | 取消发 `DELETE` |
| AC-FILE-02-1 | 端到端 | 通过 | 各端 TC-FILE-02-01；**本次跨端**（第 2 节）；后端 `TestTC_FILE_02_01_SendImage` | 6 个方向通过 |
| AC-FILE-02-4 | 前端 | 部分通过 | TC-FILE-02-04 三端 | 没有用 EXIF 解析工具（没有 exiftool，不安装），按段检查最终对象没有 APP1、`Exif`、型号串，方向用像素验证（H18）；MinIO 上没有复跑 |
| AC-FILE-02-5 | 前端 | 通过 | TC-FILE-02-05 三端（真实服务端） | |
| AC-FILE-02-6 | 前端 | 通过 | TC-FILE-02-06 三端 | 下一条消息位移 ≤ 1 px / pt |
| AC-FILE-02-7 | 前端 | 通过 | TC-FILE-02-07 三端 | |
| AC-FILE-02-8 | 前端 | 通过 | TC-FILE-02-08 三端 | |
| AC-FILE-02-9 | 前端 | 通过 | TC-FILE-02-09 三端 | 0 次原图 |
| AC-FILE-02-10 | 手工 | 未执行 | — | 第 4 节 |
| AC-FILE-02-12 | 前端 | 通过 | TC-FILE-02-12 三端 | |
| AC-FILE-02-13 | 前端 | 通过 | TC-FILE-02-13 三端 | |
| AC-FILE-02-14 | 前端 | 通过 | TC-FILE-02-14 Android、iOS（H5 播放也验了） | |
| AC-FILE-03-1 | 端到端 | 通过 | 各端 TC-FILE-03-01；**本次跨端**（第 2 节）；后端 `TestTC_FILE_03_01_GroupFile` | 群文件 SHA-256 一致，6 个方向 |
| AC-FILE-03-3 | 前端（H5） | 通过 | `files-real.spec.ts` TC-FILE-03-03 | |
| AC-FILE-03-4 | 前端 | 通过 | TC-FILE-03-04 三端 | |
| AC-FILE-03-5 | 前端（原生） | 通过 | Android `tc_file_03_05_…`、iOS `testTC_FILE_03_05_…` | "断网"用假 S3 断开正在传的 GET 代替（A20、I15） |
| AC-FILE-03-6 | 前端 | 通过 | TC-FILE-03-06 三端 | |
| AC-FILE-03-7 | 前端（H5） | 通过 | TC-FILE-03-01（H5，中文文件名） | |
| AC-FILE-03-8 | 前端 | 通过 | TC-FILE-03-08 三端 | |
| AC-FILE-03-9 | 前端（原生） | 通过 | Android、iOS TC-FILE-03-09 | 只验操作项，没有拉起其他 App（A25） |
| AC-FILE-03-10 | 前端（H5） | 部分通过 | TC-FILE-03-10（生产构建的 CSP，没有违规） | AC 写"生产 nginx 配置"：nginx 的响应头只有 Vitest 检查配置文本，没有在真实 nginx 上跑（H21） |
| AC-FILE-04-10 | 前端 | 通过 | TC-FILE-04-10 三端（200 张） | |
| AC-FILE-04-11 | 前端 | 部分通过 | H5 TC-FILE-04-11、Android Logcat 扫描、iOS `check_e2e_leftovers` | iOS 系统 CFNetwork 失败日志带完整预签名地址（I11，P-38 ② 接受为已知残留，真机是否仍是明文随 P-13）。本次两轮 iOS 运行里这类日志 0 行 |
| AC-FILE-05-3 | 前端 | 通过 | TC-FILE-05-03 三端 | |
| AC-FILE-06-6 | 前端 | 通过 | TC-FILE-06-05 三端 | |
| AC-FILE-06-12 | 前端（三套 SDK） | 通过 | TC-FILE-06-11：sdk-ts `files.test.ts`、sdk-kotlin `FilesTest`、sdk-swift `testTC_FILE_06_11_…`；服务端部分见后端报告 | 三套 SDK 单测（FakeTransport）；2026-11-10 起另有共享向量 `chat-sync.json`（BE-403） |
| AC-FILE-07-1 | 前端（原生） | 通过 | Android `FilesKillE2e.uploadKilled`（真实杀进程）、iOS `testTC_FILE_07_01_…` | |
| AC-FILE-07-2 | 前端 | 部分通过 | 三套 SDK 单测 | 三端页面都没有对应用例（A27、I18；H5 同样只有 SDK） |
| AC-FILE-07-3 | 前端（H5） | 通过 | `files-real.spec.ts` TC-FILE-07-03 | |
| AC-FILE-07-4 | 前端 | 通过 | TC-FILE-07-04 三端 | |
| AC-FILE-07-5 | 前端（原生） | 通过 | Android、iOS TC-FILE-07-05 | 用测试目录的源文件代替相册（A26、I14） |
| AC-FILE-07-6 | 前端 | 通过 | TC-FILE-07-06 三端 | Android 真实飞行模式；iOS 用假 S3 断开 PUT 代替（I15） |
| AC-FILE-07-7 | 前端 | 部分通过 | TC-FILE-07-07 三端 | 只验"5 次失败、没有 complete、没有最终对象"，额度按规则推断没扣，没有直接读额度（H19） |
| AC-FILE-07-9 | 前端（原生） | 通过 | Android、iOS TC-FILE-07-09 | |
| AC-FILE-07-10 | 前端 | 通过 | H5 `zz-im-clock.spec.ts`（tianting-web@bdf0cd8）、Android 两条（tianting-android@d263b6b）、iOS `ZzFilesClockE2ETests`（tianting-ios@52c6f2f） | v1.4（P-37 ①②），服务端回 4048 |
| AC-CHAT-90-8 | 前端【全部】 | 部分通过 | 第 3 节 | 图标清单没有逐个核对；新消息播报三端未执行 |
| AC-CHAT-90-9 | 前端【全部】 | 部分通过 | 第 3 节 | H5 axe；原生是等价检查（Android 语义树、iOS 系统审计），不是 axe；新消息播报未执行 |
| AC-CHAT-90-15 | 前端【全部】（三套 SDK） | 通过 | `account-session.json`（含 `attachmentCases`）、`chat-sync.json`：sdk-ts `account-vectors.test.ts`、`attachment-vectors.test.ts`、`chat-sync-vectors.test.ts`；sdk-kotlin `AccountVectorsTest`、`AttachmentVectorsTest`、`ChatSyncVectorsTest`；sdk-swift `AccountVectorTests`、`AttachmentVectorTests`、`ChatSyncVectorTests` | 2026-11-10 BE-403 后由部分通过改为通过，第 4 节。本地结果 |
| AC-ADM-30-7 | 后台页面 | 通过 | tianting-admin@4b18297 `e2e/files.spec.ts`（元数据页没有正文和文件名，大小用 `fileSize`） | 页面部分；后端部分见后端报告和 TC-FILE-91-01 |
| AC-FILE-90-2 | 手工 | 未执行 | — | 第 4 节（CH-05） |
| AC-CHAT-90-1 | 后端【全部】 | 未执行 | 后端报告 | 性能，P-12 |
| AC-CHAT-90-2 | 后端【全部】 | 部分通过 | 后端报告 | e2e 不回归；TC-PERF 等 P-12 |
| AC-CHAT-90-3 | 后端【全部】 | 部分通过 | 后端报告 | 功能部分通过，性能等 P-12 |
| AC-CHAT-90-5 | 后端【全部】 | 部分通过 | 后端报告 | im-server 部分通过，其余见后端报告 |
| AC-CHAT-90-10 | 后端【全部】 | 通过 | `frames.json`（C1 帧，加 BE-403 的 `image`、`file`、`MsgBody` 互斥、`modVer`）：服务端 `TestFrameVectors`（Schema）和 `TestFrameVectorsServerParse`（真实帧解析），sdk-ts `frames-schema.test.ts`（Schema 校验），sdk-kotlin `VectorsTest`，sdk-swift `VectorTests` | 2026-11-10 BE-403 后由部分通过改为通过，第 4 节；后端报告第 5 节同样补了这一条。本地结果 |
| AC-CHAT-90-17 | 后端【全部】 | 通过 | 后端报告 | |

## 6. 汇总

| 状态 | 数量 |
|---|---|
| 通过 | 35 |
| 部分通过 | 10 |
| 未执行 | 3 |
| 失败 | 0 |
| 合计 | 48（前端任务的 42 条 + 后端【全部】6 条） |

"部分通过"都写了缺什么，"未执行"都写了原因。没有删除或放宽已有断言；新增的检查（Android 文件页面的名字）发现了 1 处问题并修了。

## 7. 需要 Frank 决定的事（只列出，没有替你定）

1. **新消息播报**（FE-640 内容 4："收到一张图片"）三端都没有实现。C2 补做，还是记为已知缺口挪到后面？
2. ~~**`chat-sync.json` 和 `frames.json` 的 C2 用例（BE-403）**~~：2026-11-10 已做完，AC-CHAT-90-15、90-10 改为通过，不用再定。
3. **AC-FILE-02-10 真机**和 **WebKit / 其他浏览器**：等 P-13。
4. **AC-FILE-90-2 云厂商**：等 CH-05；届时一并确认后端问题 28（最小权限下不存在的键回 403 还是 404）和 I11 在真机上是否仍是明文。
5. **AC-FILE-02-4 的"EXIF 解析工具"**：要不要临时安装 exiftool（或接受现在按段检查的方法，H18）。
6. **AC-FILE-07-2 页面层**：三端都只有 SDK 单测（A27、I18），是否要补页面用例，还是接受 SDK 层。
7. **AC-FILE-07-7**：额度是推断的（H19），是否要加读额度的接口或接受推断。
8. **AC-FILE-03-10**：生产 nginx 头没有在真实 nginx 上跑（H21），是否接受配置检查。
9. **Android 无障碍自动检查**：完整的 AccessibilityChecks 要新增 `espresso-accessibility` 依赖（A10），现在是语义树的等价检查。要不要加依赖。
10. 旧的仍开放项：后台用户列表"注册时间"列名（问题表 A3，FE-619）；隐私说明的"打包页面"文字（H6，等你或法务）。

另：问题表里 A34、I11、I12 三行的"现在的实现"列还写着"待决定"，实际已由 P-38 决定（A34 保留重试，I11、I12 接受），表格文字没有改。

## 8. 本次改动

| 仓库 | 提交 | 内容 |
|---|---|---|
| tianting-web | 12ee72e | `e2e/xp-files.spec.ts`、`playwright.xp.config.ts`；`e2e/helpers.ts` 管理密钥可用 `TT_E2E_ADMIN_KEY`；`tsconfig.json` 包含新配置 |
| tianting-android | 79893a7 | `CrossPlatformE2e.kt`；`FilesUiTest` 新增 2 个用例；`Attachments.kt` 能点的缩略图读"图片"；`run-tests.sh`（`TT_SERVER_RUNNING`、`TT_XP`）、`test-server.sh`（`TT_S3_ORIGINS`） |
| tianting-ios | 7f1b997 | `UITests/CrossPlatformE2ETests.swift`；`run-e2e.sh`（`TT_E2E_EXTERNAL`、`TT_E2E_XP`）。`Config/Info-Debug.plist`、`project.pbxproj` 里来历不明的改动没有提交，也没有还原 |
| im-mvp | 本报告所在提交 | 本报告；frontend-chat.md 的 FE-640 进度行；`traceability.md` 重新生成（PRD 名称 chat-v1.2 → chat-v1.4） |

SDK 仓库没有改。
