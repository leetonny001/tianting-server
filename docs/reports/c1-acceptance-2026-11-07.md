# C1 账号批次验收记录（FE-620，2026-11-07 起，2026-11-09 完成）

环境：本地开发机（macOS，10 核）。**CI 不运行，所有结果都是本地结果**：H5 和后台是"Chrome，本地"，原生是"模拟器，本地"（Android：AVD Pixel_10_Pro_XL，API 37；iOS：iPhone 16 Pro，iOS 18.4）。**没有执行**：WebKit（没装，不下载）、真机（P-13）、原生 CI（P-15）。所以"通过"只代表写明的环境。

后端验收见 [account-c1-acceptance-2026-11-06.md](account-c1-acceptance-2026-11-06.md)（BE-422）。

## 1. 全量运行（最终代码）

各端按顺序跑，没有并行。每一步前记了进程数（基线约 630，最高约 1060，没有失控）。

| 端 | 提交 | 命令 | 结果 |
|---|---|---|---|
| im-server | im-mvp@24469d5（代码） | `go test -race -count=1 ./...` | 27 个包全过 |
| console-server | tianting-console-server@9dc3f5c | `CON_TEST_REQUIRE_DB=1 go test -race -count=1 ./...`（本机 Postgres 17，临时启动，跑完停掉） | 15 个包全过。第一次运行 1 个失败，原因见第 3 节 |
| sdk-ts | tianting-sdk-ts@c583243 | `npm run build`、`npm test`、`npm run test:integration` | 单测 573、集成 52，全过 |
| sdk-kotlin | tianting-sdk-kotlin@e65b21a | `:sdk-core:test`、`:sdk-core:integrationTest` | 单测 768、集成 71（新增 6 个），全过 |
| sdk-swift | tianting-sdk-swift@01c7c32 | `swift test` | 单测 380、集成 73（新增 6 个），全过 |
| H5 | tianting-web@db2dccf | `npx vitest run`；完整 Playwright；`IM_REFRESH_ABSOLUTE=25s` 单独跑 TC-ACC-03-45 | Vitest 260；Playwright 128 过、1 跳过（就是要单独跑的那条）；单独跑 1 过 |
| 管理后台 | tianting-admin@76cd2c0 | `npx vitest run`；`npx playwright test`（`E2E_IM_WS_PORT=18280 E2E_IM_ADMIN_PORT=18281`）；邀请码链路单独跑 | Vitest 103；Playwright 54 过、3 跳过（2 个截图、1 个链路）；链路 1 过 |
| Android | tianting-android@8757053 | `:app:testDebugUnitTest`；`scripts/run-tests.sh ui`、`e2e`、`kill`、`e2e com.tianting.app.AccountE2eTest`；`IM_REFRESH_ABSOLUTE=40s` 单独跑 TC-ACC-03-45 | 单测 36；UI 226 全过（最后一次）；`ServerE2eTest` 10 个连跑 3 次全过；`kill` 4 个场景全过；`AccountE2eTest` 26 个全过（1 个按设计跳过，单独跑也过了）。中途的失败见第 3 节 |
| iOS | tianting-ios@2fabcce | Kit：`xcodebuild test -scheme TiantingAppKit`；全量 `scripts/run-e2e.sh --device 46E6F7B9-…`；两条要配服务端的单独跑 | Kit 112（4 个跳过，是真实 Keychain 用例）；全量 131 个：125 过、4 失败、2 跳过（就是要单独跑的两条）；`check_e2e_leftovers` 通过。失败里 2 个是选择器写错，改了以后单独跑通过；另外 2 个是新写的用例，期望值和真实服务端不符，**没有改期望，也没有提交**，见第 3 节 |

`UI_CLASSES` 包含 `AccountUiTest`、`SessionUiTest` 等全部 UI 类，没有漏掉新类；e2e 类用 `e2e` 的参数跑。

## 2. 补上的缺口

- **只对假服务端跑过的 AC，能接真实服务端的都接上了**：03-10（三端真实定时器）、03-26（三端真实重放）、03-41（三端真实 superseded 1 次）、03-45（三端真实绝对上限）、04-25 场景二（三端真实"晚于 30 秒"）、03-13（iOS 真实杀进程）、03-40（iOS 真实 4290）。造不出来的：连续 3 次 superseded、改密"失败"和"响应先到"、503。
- **AC-ACC-04-25 甲乙丙丁**：三端逐端对真实服务端跑。Android 补了乙，iOS 补了甲、乙、丙、丁。
- **AC-ACC-09-1**：H5 走后台页面真实生成 → H5 注册 → 后台"2 / 2"的完整链路；Android、iOS 补了"管理接口显示已用 2 次"的断言。
- **AC-CHAT-90-15**：三套 SDK 的 `account-session.json` 逐套留了结果（见第 1 节）。
- **securityNotice**：三端 App 对真实服务端验证了 `newDeviceLogin`、`recoveryCodeRegenerated`、`passwordChanged`（和 `kicked` 同时），H5 还验了 `passwordRecovered`。sdk-kotlin、sdk-swift 补了 `newIpPrefix`、`refreshReplay`、`refreshRateLimited`、`passwordRecovered`、`passwordChanged`、`recoveryCodeRegenerated` 的真实服务端用例，和 sdk-ts 同一组。
- **AC-CHAT-90-8、90-9**：见逐条表。

## 3. 失败和根因

| 端 | 用例 | 次数 | 根因 | 处理 |
|---|---|---|---|---|
| H5 | TC-ACC-03-10 | 单独连跑 5 次，失败 4 次 | 测试竞争：调快 page.clock 后打开聊天页，刷新回来的 token 按本地钟不足 6 小时，启动时按 S1 的规则再刷新一次；连上之前按回车，H11 的处理是保留输入框内容，所以没有消息 | `sendTo` 改成连上之前再按回车，连跑 8 次通过。产品行为符合 S1 |
| H5 | 新加的 axe 用例 | 第一次 2 个失败 | 用例写错了昵称页的路由（`/me/nickname`，实际是从"我"页进入） | 改了进入方式，axe 没有问题 |
| console | `TestTC_CHAT_90_04_NoSecretsInLogsAndAudit` | 1 次 | 随机 marker 可能以 `-` 开头，argparse 当成选项 | 改成 `--marker=…`（tianting-console-server@9dc3f5c），重跑这一条和全量都通过 |
| Android | `ChatTest.send_states_and_retry_confirm` | UI 全量第 1 次 | `waitText` 被输入框里还没清空的同一段文字先满足，消息气泡还没出现 | 改成等消息气泡（选择器），之后 UI 全量通过 |
| Android | `ChatTest.menu_copy_retry_delete` | ChatTest 整类连跑 7 次，失败 2 次；单独跑 6 次都过 | 删除后 5 秒内"会失败"仍在。**根因没有查明** | 没有改，记为问题 A16 |
| Android | `ChatTest.at_bottom_follows_new_message` | 7 次里 1 次 | Compose `LazyColumn` 测量时 `IndexOutOfBoundsException`（`MutableIntervalList.get`，区间列表为空却要第 0 项），App 里是聊天页的消息列表。**根因没有查明**，真实用户可能遇到崩溃 | 没有改，记为问题 A16 |
| Android | `ConvListTest.start_chat_self_and_friends_mode_warning` | 整类连跑多次，约 1/3 失败 | 已经跳转到聊天页（路径是 `/chat/c2c:alice:bob`，关系是好友），但 5 秒内没有 `chat-title`。**根因没有查明**；试过"重复点击""先等好友关系"都不能解决，都已撤销 | 没有改，记为问题 A17 |
| Android | `ServerE2eTest.live_notice_…_logcat` | 1 次 | 测试读 `logcat -d -b all` 全部缓冲区，模拟器开得久了超过测试进程内存（OutOfMemoryError） | 用例开头先清空 Logcat（@8757053），之后连跑 3 次通过。**这样检查范围变小了**，见问题 A18 |
| Android | `ServerE2eTest.group_chat_with_sender_names` | 4 次里 1 次 | 15 秒内 carol 没收到"大家好"。**根因没有查明**，那次没留下日志 | 没有改 |
| Android | `AccountE2eTest` 新用例 03-26、03-41 | 第一次失败 | 偷 refresh 时 App 还在线，服务端复查发现代号旧了，App 自己先刷新，抢在测试前面；第二次是重复关闭客户端被拒 | 偷之前先停 App；重复关闭不再报错。之后通过 |
| Android | `AccountE2eTest` 新用例 04-25 场景二 | 第一次失败 | 断言写错：踢线生效后会话结束、存储清空，不会等于旧值 | 改成"存储为空或仍是旧值，不能被迟到的响应换掉"，见第 6 节 |
| Android | TC-ACC-03-45 单独跑 | 第一次失败 | 模拟器的钟比 Mac 慢约 4.4 秒，"上限前 1.5 秒"实际已经过了上限 | 检查点提前到上限前 8 秒，见第 6 节 |
| iOS | 新用例 04-15、04-25 场景二 | 全量 1 次 | 选择器写错（`password-old`，iOS 是 `security-current`） | 改了选择器，单独跑通过 |
| iOS | 新用例 TC-ACC-03-08（真实服务端） | 全量 1 次 | 期望 AC 的文案"登录已失效，请重新登录"，实际"账号已被下线"：iOS 测试改不了 App 的本地到期，本地认为 access 没过期，先发 `auth`，服务端回 `kicked`（I3） | **没有改期望，没有提交**，留在工作区，待决定 |
| iOS | 新用例 TC-ACC-03-12（在线时 access 过期） | 全量 1 次 | 调 IM 钟 25 小时、回前台后 60 秒内没有刷新：服务端在复查间隔内没有踢这条连接，回前台按本地钟也不刷新。用例的触发方式不成立 | **没有提交**，留在工作区，待决定 |

点名要确认的用例：Android `group_all_send_4031_shows_bar_and_failed_icon`（ModerationUiTest 整类连跑 3 次 + UI 全量 2 次都过）、`start_chat_self_and_friends_mode_warning`（仍不稳定，见上）、`send_failure_toasts_once_each_attempt`（ChatTest 连跑 7 次 + UI 全量都过）、`ServerE2eTest`（见上）；iOS `testTC_ACC_04_16_credentialsChangedIsNotAnAnomaly` 全量通过。

## 4. 截图走查

- **H5**：提醒 + `kicked` 弹窗同时出现（浅色、深色）：提醒在遮罩上方，不挡"确定"，axe 没有问题；`recoveryCodeRegenerated`、`passwordRecovered`。
- **Android**：之前没看的登录错误、改昵称、改密、退出其他设备（浅色、深色）都正常。提醒 + 弹窗：刚出现时隐私说明更新的弹窗正在淡出，和踢线弹窗叠在一起（"知道了 / 查看"透出来）；1.5 秒后再截一张，只剩踢线弹窗，提醒在遮罩下方，能看清。
- **iOS**：踢线弹窗从底部出，提醒和弹窗在同一位置，弹窗出来后提醒被盖住（XCUITest 确认"确定"能点）。刚出现时的截图里遮罩正在展开。这不算失败，但用户可能看不到"你的账号密码已被修改"这条提醒，记为问题 I10。

## 5. 文档

- `docs/sdk/README.md`：账号接口、token 提供者用法、关闭码在账号模式的例外、绝对上限和 F2、三套的集成测试命令、`dist/` 要先构建、securityNotice 的真实服务端用例三套都有。
- `tianting-ios/README.md`：`check_sources.py` 报的是提交 516a578 写进 README 的"微信式 IM 产品"（B-1 不允许），改成"独立 IM 产品"。其他仓库的 README 和 `package.json` 里同样的字样没有改，待 Frank 决定。

## 6. 这一轮在测试里删除或改弱的地方

| 位置 | 改了什么 | 现在 |
|---|---|---|
| tianting-android `ServerE2eTest.live_notice_banner_list_and_no_token_or_data_in_logcat`（@8757053） | 开头加了 `logcat -b all -c`。**之前用例写进 Logcat 的 token 不再被这条顺带发现**，只检查它自己运行期间的日志。原因是 OOM | 已提交，问题 A18 |
| tianting-android `AccountE2eTest.tc_acc_04_25_real_response_after_31_seconds`（新用例） | "存储里的 refresh 等于旧值"改成"为空或等于旧值"（踢线生效后本来就清空）；不再在断言信息里带 token 值 | 已提交 |
| tianting-android `AccountE2eTest.tc_acc_03_45_real_absolute_limit`（新用例） | "上限前 1.5 秒没有刷新"改成"上限前 8 秒" | 已提交 |
| tianting-web `account-real.spec.ts` TC-ACC-03-45（新用例） | "到期后刷新次数仍是 0"改成"上限前 1.5 秒仍是 0"（F2） | 已提交 |
| tianting-ios `testTC_ACC_03_45_realAbsoluteLimit`（新用例） | 同上，改成"上限前一段时间没有刷新" | 已提交 |
| tianting-ios `testTC_ACC_03_08_realRevokedRefreshColdStart`、`testTC_ACC_03_12_expiredWhileOnlineRefreshesQuietly`（新用例） | 曾经从文件里删掉，**已恢复**；期望没有改。这两条失败，没有放进提交，留在工作区 | 待决定 |
| tianting-android `ConvListTest`、`TestSupport.kt` | 试过加重复点击、先等好友关系 | **已撤销**，文件和 HEAD 相同 |

## 7. 逐条结果

| AC | 批次 | 层级 | 用例（TC-ACC-） | 状态 | 环境或缺什么 |
|---|---|---|---|---|---|
| AC-ACC-01-1 | C1 | 后端 | 01-01、01-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-2 | C1 | 后端 | 01-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-3 | C1 | 后端 | 01-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-4 | C1 | 后端 | 01-05 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-5 | C1 | 后端 | 01-06 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-6 | C1 | 后端 | 01-07 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-7 | C1 | 后端 | 01-08 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-8 | C1 | 后端 | 01-09 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-10 | C1 | 后端 | 01-10 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-11 | C1 | 后端 | 01-11 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-12 | C1 | 前端 | 01-12 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-01-13 | C1 | 前端 | 01-13 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-01-14 | C1 | 后端 | 01-14 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-15 | C1 | 后端 | 01-15 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-16 | C1 | 后端 | 01-16 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-17 | C1 | 后端 | 01-17 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-18 | C1 | 前端 | 01-18 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-01-19 | C1 | 后端 | 01-19 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-01-21 | C1 | 后端 | 01-21 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-1 | C1 | 后端 | 02-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-2 | C1 | 后端 | 02-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-3 | C1 | 后端 | 02-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-4 | C1 | 后端 | 02-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-5 | C1 | 后端 | 02-05 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-6 | C1 | 后端 | 02-06 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-7 | C1 | 后端 | 02-07 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-8 | C1 | 前端 | 02-08 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-02-9 | C1 | 前端 | 02-09 | 通过 | H5 Playwright，Android 真实服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-02-10 | C1 | 后端 | 02-10 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-11 | C1 | 后端 | 02-11 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-12 | C1 | 后端 | 02-12 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-02-13 | C1 | 端到端 | 02-13 | 通过 | 三端真实服务端，2 秒内提示 + 账号安全页记录（Android 留了 1 秒取节点余量）；三套 SDK 集成 |
| AC-ACC-02-14 | C1 | 后端 | 02-14 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-1 | C1 | 后端 | 03-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-2 | C1 | 后端 | 03-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-3 | C1 | 后端 | 03-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-4 | C1 | 后端 | 03-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-5 | C1 | 后端 | 03-05 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-6 | C1 | 后端 | 03-06 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-7 | C1 | 前端 | 03-07 | 通过 | Android、iOS 真实服务端冷启动（access 在服务端过期）进会话列表；"先刷新再 auth"的帧顺序在假服务端用例和 SDK 用例（模拟器，本地） |
| AC-ACC-03-8 | C1 | 前端 | 03-08 | 通过 | PRD-chat v1.2 两种文案都认（I3）。Android 改本地到期后冷启动：先刷新得 4019，显示"登录已失效，请重新登录"（真实服务端，模拟器，本地）。iOS 真实服务端（tianting-ios@54caa87）：用例经代理记下实际路径，本次是先连上、服务端以 `kicked/banned` 下线（admin 强制下线），断言对应的"账号已被下线"；若走刷新 4019 则断言另一句。4019 那条路径 iOS 由假服务端用例覆盖（模拟器，本地） |
| AC-ACC-03-9 | C1 | 前端 | 03-09 | 通过 | Chrome，本地，真实 im-server。CH-26 的差异见 H1 |
| AC-ACC-03-10 | C1 | 前端 | 03-10 | 通过 | H5 page.clock + 真实服务端；Android 真实定时器（改存储里的到期）；iOS 真实定时器（`IM_ACCESS_TTL=6h20s`）。（Chrome，本地；模拟器，本地） |
| AC-ACC-03-11 | C1 | 前端 | 03-11 | 部分通过 | Android 真实飞行模式通过（模拟器，本地）；iOS 只能用调试开关 `TT_TEST_OFFLINE`，模拟器共用 Mac 的网络 |
| AC-ACC-03-12 | C1 | 前端 | 03-12 | 通过 | 503：假响应（三端、三套 SDK；真实服务端造不出 503）。在线时 `kicked/tokenExpired` 且刷新成功：假服务端三端和 SDK 通过；**真实服务端在线路径已补上**（iOS，`IM_ACCESS_TTL=30s`，服务端真实到期计时器踢 `tokenExpired` → 刷新 200 → 新连接 101，不弹窗，连跑 3 次过，模拟器，本地）。Android、H5 的在线踢线仍是假服务端 + 冷启动 4011 的真实路径 |
| AC-ACC-03-13 | C1 | 前端 | 03-13 | 通过 | Android、iOS 真实杀进程 + 真实服务端（模拟器，本地） |
| AC-ACC-03-14 | C1 | 前端 | 03-14 | 通过 | Chrome，本地，真实 im-server，真实"复制标签页" |
| AC-ACC-03-15 | C1 | 后端 | 03-15 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-16 | C1 | 后端 | 03-16 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-17 | C1 | 后端 | 03-17 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-18 | C1 | 前端 | 03-18 | 通过 | Android 假服务端、iOS 真实服务端（模拟器，本地） |
| AC-ACC-03-19 | C1 | 后端 | 03-19 | 部分通过 | 服务端部分通过（Go e2e / 单测（`go test -race`，SQLite，本地））；推送令牌部分随 C3，没有验证 |
| AC-ACC-03-20 | C1 | 前端 | 03-20 | 部分通过 | Android、iOS 真实杀进程 + 真实服务端；5 分钟靠 IM 钟推进，App 的钟没动 |
| AC-ACC-03-21 | C1 | 后端 | 03-21 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-22 | C1 | 后端 | 03-22 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-23 | C1 | 前端 | 03-23 | 部分通过 | 四个入口同时 + 超时 + 迟到响应：SDK 三套、H5 Vitest、Android 假服务端；真实服务端只验了 Android 启动和回前台合并成 1 次。**iOS App 层没有单独跑** |
| AC-ACC-03-24 | C1 | 后端 | 03-24 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-25 | C1 | 后端 | 03-25 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-26 | C1 | 前端 | 03-26 | 通过 | 三端真实服务端造出重放，弹窗文案一致（Chrome，本地；模拟器，本地） |
| AC-ACC-03-27 | C1 | 后端 | 03-27 | 部分通过 | 服务端部分通过（Go e2e / 单测（`go test -race`，SQLite，本地））；推送令牌部分随 C3，没有验证 |
| AC-ACC-03-28 | C1 | 后端 | 03-28 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-29 | C1 | 后端 | 03-29 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-30 | C1 | 后端 | 03-30 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-31 | C1 | 后端 | 03-31 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-32 | C1 | 后端 | 03-32 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-33 | C1 | 后端 | 03-33 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-03-34 | C1 | 端到端 | 03-34 | 通过 | 三端真实服务端，access 未过期、已过期两种（Chrome，本地；模拟器，本地）；三套 SDK 集成 |
| AC-ACC-03-35 | C1 | 后端 | 03-35 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-36 | C1 | 后端 | 03-36 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-47 | C1 | 后端 | 03-37 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-48 | C1 | 后端 | 03-38 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-49 | C1 | 后端 | 03-49 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-50 | C1 | 后端 | 03-50 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-37 | C1 | 端到端 | 03-39 | 部分通过 | 第 21 次 4290：三套 SDK、H5 Vitest、Android 假服务端，iOS 真实服务端；100 天和绝对上限在服务端 e2e。H5、Android 没有对真实服务端跑 4290 |
| AC-ACC-03-38 | C1 | 后端 | 03-40 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-39 | C1 | 前端 | 03-41 | 部分通过 | 连续 3 次 superseded 只有假响应（三端、三套 SDK）。真实服务端造不出连续 3 次（窗口结束就是重放），只验了 1 次 superseded + 按时重试 + 重放（三端） |
| AC-ACC-03-40 | C1 | 后端 | 03-42 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-41 | C1 | 后端 | 03-43 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-42 | C1 | 前端 | 03-44 | 部分通过 | `/v1/files/urls` 随 C2，用 `/v1/account/events` 代替：SDK 三套、H5 Vitest、Android 假服务端；**iOS App 层没有跑** |
| AC-ACC-03-43 | C1 | 前端 | 03-45 | 部分通过 | [SDK 钟] 假接口用例三端通过。真实服务端（F2 已修，im-mvp@ce4ee6b；H5 `IM_REFRESH_ABSOLUTE=25s`，原生 40s）：上限前没有刷新、没有重连；**到期后没有成功的刷新、没有重连**，回登录页。**缺口**：服务端在上限那一秒（JWT 的 `exp` 只到秒）踢 `tokenExpired`，三端 SDK 收到后都会发 1 次刷新，服务端按到期回 401 / 4019（三端实测都是 `401/4019` 一次）。用例断言"最多这 1 次且被拒绝"，不是"请求数为 0"；要不要让 SDK 在 access 到期等于上限时收到 `tokenExpired` 直接回登录页，待 Frank 决定（见第 10 节）。Chrome，本地；模拟器，本地 |
| AC-ACC-03-44 | C1 | 后端 | 03-46 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-45 | C1 | 后端 | 03-47 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-03-46 | C1 | 后端 | 03-48 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-1 | C1 | 后端 | 04-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-2 | C1 | 后端 | 04-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-3 | C1 | 后端 | 04-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-4 | C1 | 后端 | 04-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-5 | C1 | 前端 | 04-05 | 通过 | 三端真实服务端踢线和弹窗日期（Chrome，本地；模拟器，本地） |
| AC-ACC-04-6 | C1 | 前端 | 04-06 | 部分通过 | 断网退出、恢复后吊销：Android 真实飞行模式，iOS 用 TCP 代理；系统令牌注销随 C3 |
| AC-ACC-04-7 | C1 | 后端 | 04-07 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-8 | C1 | 后端 | 04-08 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-10 | C1 | 端到端 | 04-09 | 通过 | 三端真实服务端（Chrome，本地；模拟器，本地）；三套 SDK 集成 |
| AC-ACC-04-11 | C1 | 后端 | 04-10 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-12 | C1 | 后端 | 04-11 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-13 | C1 | 后端 | 04-12 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-14 | C1 | 后端 | 04-13 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-15 | C1 | 后端 | 04-14 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-16 | C1 | 端到端 | 04-15 | 通过 | 三端真实服务端：响应丢失后提示失败，踢线生效，不弹"凭证异常"（Chrome，本地；模拟器，本地）；`credentialsChanged` 和指标不变在服务端 e2e |
| AC-ACC-04-17 | C1 | 前端 | 04-16 | 通过 | TC 的层级是 SDK 单测（三套，本地）；H5 Vitest、Android、iOS 假服务端也有 |
| AC-ACC-04-18 | C1 | 后端 | 04-17 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-19 | C1 | 后端 | 04-18 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-20 | C1 | 后端 | 04-19 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-21 | C1 | 后端 | 04-20 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-22 | C1 | 后端 | 04-21 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-23 | C1 | 后端 | 04-22 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-24 | C1 | 后端 | 04-23 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-25 | C1 | 端到端 | 04-24 | 通过 | 甲乙丙丁三端逐端对真实服务端（Chrome，本地；模拟器，本地），客户端都显示第 7 天；服务端判定在 Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-27 | C1 | 前端 | 04-25 | 部分通过 | 四种时序在 SDK 和假服务端（三端）；真实服务端验了"响应丢失"和"晚于 30 秒"（三端）；"失败"和"响应先到"真实服务端造不出 |
| AC-ACC-04-28 | C1 | 后端 | 04-26 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-29 | C1 | 后端 | 04-27 | 未执行 | 只有 Postgres 下可达，仓库只有 SQLite，PRD 写明 C1 记未执行 |
| AC-ACC-04-30 | C1 | 后端 | 04-28 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-04-31 | C1 | 后端 | 04-29 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-1 | C1 | 后端 | 05-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-2 | C1 | 后端 | 05-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-3 | C1 | 后端 | 05-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-5 | C1 | 前端 | 05-04 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-05-6 | C1 | 后端 | 05-05 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-7 | C1 | 前端 | 05-06 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-05-8 | C1 | 后端 | 05-07 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-9 | C1 | 后端 | 05-08 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-05-10 | C1 | 前端 | 05-09 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-05-11 | C1 | 后端 | 05-10 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-12 | C1 | 后端 | 05-11 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-13 | C1 | 后端 | 05-12 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-14 | C1 | 后端 | 05-13 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-15 | C1 | 前端 | 05-14 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-05-16 | C1 | 后端 | 05-15 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-17 | C1 | 后端 | 05-16 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-18 | C1 | 后端 | 05-17 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-05-19 | C1 | 端到端 | 05-18 | 通过 | 两个场景三端真实服务端（H5 刷新，Android、iOS 真实杀进程） |
| AC-ACC-06-1 | C1 | 后端 | 06-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-06-2 | C1 | 后端 | 06-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-06-5 | C1 | 前端 | 06-05 | 部分通过 | 本端立即显示：三端；其他端 10 分钟：H5 page.clock 和三套 SDK 通过，Android、iOS 界面没推进钟 |
| AC-ACC-07-1 | C1 | 后端 | 07-01 | 通过 | `server/e2e` 现有用例不改代码全过（Go e2e / 单测（`go test -race`，SQLite，本地）），依据是这次全量运行，没有单独的用例名 |
| AC-ACC-07-2 | C1 | 后端 | 07-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-07-3 | C1 | 后端 | 07-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-07-4 | C1 | 后端 | 07-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-07-5 | C1 | 前端 | 07-05 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-07-6 | C1 | 前端 | 07-06 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-07-7 | C1 | 前端 | 07-07 | 通过 | H5 Playwright，Android 假服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-07-8 | C1 | 前端 | 07-08 | 通过 | H5 Playwright，Android 假服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-07-9 | C1 | 前端 | 07-09 | 通过 | H5 Playwright，Android 真实服务端，iOS 假服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-08-1 | C1 | 后端 | 08-01 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-08-2 | C1 | 后端 | 08-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-08-3 | C1 | 后端 | 08-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-08-4 | C1 | 后端 | 08-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-08-5 | C1 | 后端 | 08-05 | 部分通过 | 恢复和不重复通过（Go e2e / 单测（`go test -race`，SQLite，本地））；清理期间的 ack P99 没有测 |
| AC-ACC-08-6 | C1 | 前端 | 08-06 | 通过 | H5 Playwright，Android 真实服务端，iOS 真实服务端（Chrome，本地；模拟器，本地） |
| AC-ACC-09-1 | C1 | 端到端 | 09-01 | 通过 | H5：后台页面真实生成 → H5 注册三个用户 → 后台"2 / 2"（`zz-invite-chain.spec.ts`，Chrome，本地）。Android、iOS：im-server 签名接口生成（console 调的同一个接口），App 注册，第 3 次"邀请码无效"，管理接口已用 2 次（模拟器，本地） |
| AC-ACC-09-2 | C1 | 后端 | 09-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-5 | C1 | 后端 | 09-03 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-6 | C1 | 后端 | 09-04 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-7 | C1 | 后端 | 09-05 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-8 | C1 | 后端 | 09-06 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-9 | C1 | 后端 | 09-07 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-09-10 | C1 | 后端 | 09-08 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-09-11 | C1 | 前端 | 09-09 | 通过 | Chrome，本地（tianting-admin，真实 console-server、Postgres、im-server） |
| AC-ACC-09-12 | C1 | 后端 | 09-10 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-09-13 | C1 | 后端 | 09-11 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地）；console-server 真实 Postgres |
| AC-ACC-09-14 | C1 | 后端 | 09-12 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-10-1 | C1 | 前端 | 10-01 | 通过 | H5、Android 真实地址，iOS 假服务端页面（Chrome，本地；模拟器，本地）；法律文字还没有 |
| AC-ACC-10-2 | C1 | 后端 | 10-02 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-ACC-10-3 | C1 | 前端 | 10-03 | 部分通过 | 提示一次、确认后不再提示：三端；**服务端没有接口记录确认**（H5、A2、I1） |
| AC-CHAT-90-1 | 全部 | 后端 | CHAT-90-01 | 未执行 | 开发机只记录，不算通过，发布判定等 P-12 |
| AC-CHAT-90-2 | 全部 | 后端 | CHAT-90-02 | 部分通过 | C1 没有其他开关，现有 e2e 全过（Go e2e / 单测（`go test -race`，SQLite，本地））；TC-PERF 判定等 P-12 |
| AC-CHAT-90-3 | 全部 | 后端 | CHAT-90-03 | 未执行 | 第 15 节累计表：C1 不适用 |
| AC-CHAT-90-5 | 全部 | 后端 | CHAT-90-04、CHAT-90-05 | 部分通过 | C1 流程的后端 e2e 通过（im-server；console-server 真实 Postgres，含 `scan-logs`，本地）；部署 e2e（TC-CHAT-90-04 第二层）未执行 |
| AC-CHAT-90-6 | C1 | 后端 | CHAT-90-06 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-CHAT-90-8 | 全部 | 前端 | CHAT-90-07 | 部分通过 | 界面、文案、构建产物：H5 `check-brand`、`check-assets`、`npm run check`，Android `checkTianting`，iOS `check_sources.py`，后台 `check:dist` 都通过；账号页面新增图标只有 Android 的 `back`（清单内）。**缺**：B-3 的（手工）评审没做；B-1 说仓库文档也不能有，tianting-android、tianting-admin、tianting-sdk-swift、tianting-ops 的 README 和 tianting-web 的 `package.json` 还有，待 Frank 决定（只改了 iOS README） |
| AC-CHAT-90-9 | 全部 | 前端 | CHAT-90-08 | 部分通过 | H5：axe-core（依赖里已有 `@axe-core/playwright`）扫了全部账号页面，没有 `button-name` 和严重问题；深色截图、键盘走查做了（Chrome，本地）。**原生没有 axe**：Android 只查语义树的按钮名（Espresso AccessibilityChecks 要新增依赖，A10），iOS 只有系统审计 `.sufficientElementDescription`；深色截图两端都看了。H5 消息菜单的键盘走查属于 C4 |
| AC-CHAT-90-10 | 全部 | 后端 | CHAT-90-09 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-CHAT-90-11 | C1 | 后端 | CHAT-90-10 | 未执行 | 开发机只记录，不算通过（样本和峰值内存都不够），发布判定等 P-12 |
| AC-CHAT-90-14 | C1 | 后端 | CHAT-90-12 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-CHAT-90-15 | 全部 | 前端 | CHAT-90-13 | 通过 | `account-session.json` 三套 SDK 都跑了（sdk-ts、sdk-kotlin、sdk-swift，macOS，本地）；`chat-sync.json` 属于 C2 |
| AC-CHAT-90-16 | C1 | 后端 | CHAT-90-14 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-CHAT-90-17 | 全部 | 后端 | CHAT-90-15、CHAT-90-16 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |
| AC-CHAT-90-18 | C1 | 后端 | CHAT-90-17 | 通过 | Go e2e / 单测（`go test -race`，SQLite，本地） |

## 8. 汇总

| 状态 | 数量 |
|---|---|
| 通过 | 155 |
| 部分通过 | 18 |
| 未执行 | 4 |
| 合计 | 177 |

"部分通过"都写了缺什么；"未执行"都写了原因。没有"基本通过"。第 10 节复测后：AC-ACC-03-8、AC-ACC-03-12 由部分通过改为通过；AC-ACC-03-43 仍是部分通过（原因换成第 10 节写的那 1 次被拒绝的刷新）。

## 9. 工作量校准（CH-27，协调方写）

**结论：人日估算没法直接换算成 AI 用时。C1 真正的瓶颈是本机资源和待决定项，不是写代码。** 下面的数据不全，置信度低。

| 部分 | 估算（人日，第 18 节） | 实际钟面时间（AI，含等待） | 新增行数（约） |
|---|---|---|---|
| 服务端 BE-410～424（两段） | 18.5 + 测试 5 + 部署文档 2.5 | 约 2 小时 33 分（70 + 83 分钟） | +16200 |
| sdk-ts / sdk-kotlin / sdk-swift | 3.5 / 4 / 4 | 约 26 分 / 45 分 / 没有准确记录 | +4400 / +5700 / +5900 |
| 三套 SDK 语义统一（K、S 系列） | 没有单独估 | 约 25 分 | 没有统计 |
| console CS-200 | 1.5（含后台页面） | 没有记录 | +1100 |
| 管理后台 FE-619 | 同上 | 约 17 分 | +1400 |
| H5 FE-613、614 | 5 | 约 1 小时 32 分（含磁盘写满时的等待） | 没有统计 |
| Android FE-615、616 | 5.5 | 约 2 小时 37 分（含等待） | +4200 |
| iOS FE-617、618 | 5.5 | 约 3 小时 27 分（含等待） | +4100 |
| 三端验收 FE-620 | 4 | 约 4 小时（其中约 30 分钟卡在进程耗尽） | +1750 |
| 公共 BE-400～408 | 17 | 本次没有记录 | — |

- **能确定的：** C1 估算 60.5 人日，有记录的 AI 钟面时间合计约 16 小时，另有几段没记录；改动约 4.5 万行，其中大半是测试。
- **不能据此推出的：** "1 人日 = 16 分钟"这类换算。这些时间不包括 Frank 的决策等待、协调方的复核，也不包括返工（H12、I1～I9 等问题）。
- **实际卡住进度的：**
  1. 本机资源：磁盘两次写满、进程数耗尽一次、模拟器不稳定（A16、A17）。
  2. 待决定项越积越多，C1 结束时有 20 条部分通过。
  3. CI 不运行，只有本地结果，WebKit 和真机没有验证。
- **对 C2 的建议：** C2 估算 67.5 人日，按 C1 的经验，AI 钟面时间大约在 1～2 天这个量级（低置信）。真正的前提是三件事：开工前把 C1 的待决定项定掉；AC-FILE-90-1 要临时搭 MinIO，需要 Frank 再次同意；每段结束后跑清理脚本。

## 10. v1.2 决定后的复测（2026-11-09）

Frank 2026-11-09 对 C1 待决定项回复"都按建议"（PRD-chat v1.2）。按决定改了三端用例和产品代码，逐端顺序复测，没有并行。**所有结果都是本地结果**：H5 是"Chrome，本地"，原生是"模拟器，本地"（iOS：iPhone 16 Pro；Android：AVD Pixel_10_Pro_XL，emulator-5556，`-read-only`）。CI 没有运行，WebKit 和真机没有验证。

### 10.1 运行结果

| 端 | 提交 | 命令 | 结果 |
|---|---|---|---|
| iOS | tianting-ios@54caa87 | Kit：`xcodebuild test -scheme TiantingAppKit`（在 `Kit/` 下）；全量 `scripts/run-e2e.sh --device 46E6F7B9-…`；`IM_ACCESS_TTL=30s` 单独跑 03-12；`IM_REFRESH_ABSOLUTE=40s` 单独跑 03-45；`check_sources.py` | Kit 112（4 个跳过，是真实 Keychain 用例）；全量 132 个：129 过、0 失败、3 跳过（就是要配服务端单独跑的 03-10、03-12、03-45）；`check_e2e_leftovers` 通过；03-12 单独连跑 3 次全过；03-45 单独跑 2 次全过 |
| Android | tianting-android@d50c230 | `:app:testDebugUnitTest :app:checkTianting`；`scripts/run-tests.sh ui`、`e2e`、`e2e com.tianting.app.AccountE2eTest`；`IM_REFRESH_ABSOLUTE=40s` 单独跑 03-45；`class com.tianting.app.ChatTest` 连跑 10 次；`class com.tianting.app.ConvListTest` 连跑 6 次 | 单测 36；`checkTianting` 通过；UI 227 全过（第一次 226 过、1 失败，原因见 10.2 的"发送按钮"，修了以后整套重跑全过）；`ServerE2eTest` 10 全过；`AccountE2eTest` 26 过（03-45 按设计跳过）；03-45 单独跑 2 次全过；ChatTest（33 个）连跑 10 次全过；ConvListTest（12 个）连跑 6 次全过 |
| H5 | tianting-web@f0c5b6f | `npm run check`、`check-assets.mjs`、`check-brand.mjs`；`npx vitest run`；`npx playwright test e2e/account-real.spec.ts`；`IM_REFRESH_ABSOLUTE=25s` 单独跑 TC-ACC-03-45 | 检查全过；Vitest 260；account-real 9 过、1 跳过（03-45）；03-45 单独跑 1 过 |
| 管理后台、运维后台 | tianting-admin@f8a2d94、tianting-ops@cb6731f | `npm run build`、`npm run check:dist` | 都通过（只改了 README） |

### 10.2 各项处理

- **I3 / AC-ACC-03-8**：iOS 真实服务端用例经测试代理记下实际路径（刷新的 4019，或者服务端的 `kicked`），再断言这条路径的文案；不是"出现任意一个就行"。本次实际路径：冷启动先连上，服务端以 `kicked/banned` 下线（admin 强制下线走用户级吊销），文案"账号已被下线"。
- **AC-ACC-03-12 在线时 `tokenExpired`**：原来的触发方式（调 IM 钟）不成立：服务端的到期计时器在 `auth` 时按剩余时间设好，走真实时间，调 IM 钟不会触发。改成 `IM_ACCESS_TTL=30s`：注册后 SDK 先刷新一次（不足 6 小时），新 token 到手就不足 6 小时（`shortOnArrival`），之后只有服务端踢线才刷新。用例断言代理看到的顺序：`kicked/tokenExpired` → 刷新 200 → 新的 WebSocket 101，存储的保存次数增加，不弹窗，会话还在。默认服务端上跳过。
- **I10**：iOS 踢线弹窗出现时，提醒由弹窗自己画在遮罩和卡片之上、屏幕上方，不接收点击；根视图的提醒层这时不显示（屏幕上只有一个提醒）。用例断言提醒还在、文字是"你的账号密码已被修改"、和卡片不重叠、"确定"可点；浅色、深色各截一张，两张都能看清提醒和弹窗（截图在弹窗展开动画中途，卡片还没到中间）。
- **F2 / 03-45（三端）**：改回检查到期之后。三端实测一致：上限前 0 次刷新、不重连；服务端在上限那一秒踢 `tokenExpired` 后，SDK 发 1 次刷新，服务端回 **401 / 4019**；到期后没有成功的刷新、没有重连，回登录页。用例断言"最多这 1 次且是 401/4019、没有成功的刷新、不重连"，**不是"请求数为 0"**。原因：PRD 第 5 条规定收到 `kicked/tokenExpired` 就刷新，SDK 照做；要做到 0 次，要改三套 SDK（access 到期等于上限时收到 `tokenExpired` 直接回登录页），这不在本次决定里，列为待决定。Android 用 Mac 钟算检查点的问题：上限前的检查点仍提前 8 秒（模拟器慢约 4.4 秒），到期后的检查看服务端的实际回答，不依赖模拟器的钟。
- **A16，`at_bottom_follows_new_message` 的崩溃（根因已查明，已修）**：抓到的现场是 `CalledFromWrongThreadException ... Calling: tianting-sdk`，栈从 `IMClient.emitMessages`（SDK 线程设置 `messagesFlow`）经 `StateFlow.setValue` → Compose 测试的调度器 → `Snapshot.sendApplyNotifications` → 重组 → `LazyListMeasure` 一路到 `place`，随后主线程报 "performMeasureAndLayout called during measure layout"。也就是聊天页在组合的上下文里收集 SDK 线程更新的流；Compose 测试环境的调度器不切线程，收集协程在 SDK 线程上恢复，重组和消息列表的测量跑在 SDK 线程上，和主线程同时进行，区间列表被并发改动，就会出现"区间列表为空却取第 0 项"。**改法**：`messagesFlow` 用 `collectAsState(context = Dispatchers.Main)`；聊天页里组合内协程调用的 SDK 挂起接口（快照、翻页、禁言）都经 `onMain` 回到主线程再写状态。新用例 `ChatTest.sdk_thread_updates_reach_ui_on_main_thread` 用快照应用通知的线程做判据：修之前 30 条推送有 34～35 次通知发生在 `tianting-sdk` 上（单独跑 2 次都失败），修之后通过。**对真实用户的影响**：正式运行时组合的调度器是 `AndroidUiDispatcher`，会切回主线程，按代码判断真实用户碰不到这个崩溃；修改让界面不再依赖这一点。
- **A16，`menu_copy_retry_delete`**：按代码判断是测试的等待条件：点"重发"后，重发的帧发出时消息已是"发送中"，界面上的失败图标可能还没换，`waitTag("failed-icon")` 被旧图标满足；这时点删除，`removeFailed` 按规则不动发送中的消息，随后失败回来，消息就留下了。改成等 SDK 里这条消息真的又失败了再删。这一轮修改前后它都没有再失败，所以这个根因**没有被现场证实**。
- **A16 调查中发现的"发送按钮"**：这台 AVD 显示软键盘（FE-620 用的那台不显示），输入后键盘弹出、输入栏上移，`waitForIdle` 不等键盘动画，动画中点"发送"会落在按钮外，消息没有发出（ChatTest、ModerationUiTest 里"首次发送后 5 秒内没有反应"的几次失败）。改成等按钮位置连续约 0.4 秒不变再点（`clickSendWhenSettled`），只改等待条件。
- **A17（顺手修）**：发起单聊的弹窗里 `relationOf` 在 SDK 线程上恢复后直接导航，和 A16 同一个原因；改成经 `onMain`（`rememberRelation` 也一样）。ConvListTest 连跑 6 次全过（之前约 1/3 失败）。连跑 6 次不足以证明，只能说没有再出现。
- **A18**：不再 `logcat -b all -c`。新的 `scanOwnLogcat` 读 `logcat -d -b all --pid=<本进程> -T <用例开始时间>`（App 和测试在同一个进程），逐行读，在最近 8～16 KiB 里查，不把整段读进内存；断言不变（token、通知内容、密码、恢复码、refresh 都不能出现），另加"确实读到了日志"。`ServerE2eTest`、`AccountE2eTest.secrets_never_reach_logcat` 都过，没有 OOM。范围仍是"本用例本进程"，Frank 已接受。
- **README 产品名**：tianting-android、tianting-admin、tianting-ops、tianting-sdk-swift 的 README 和 tianting-web 的 `package.json` 改成"独立 IM 产品"；tianting-desktop 的 README 有同样的字样，一起改了（tianting-desktop@06128ff）。

### 10.3 这一轮删除或改弱的地方

| 位置 | 改了什么 | 说明 |
|---|---|---|
| 三端 TC-ACC-03-45 真实服务端用例 | 到期后的检查是"最多 1 次刷新、且被 401/4019 拒绝、没有成功的刷新、不重连"，不是"刷新请求数为 0" | 比上一轮（到期后不检查）严，但没有达到"请求数为 0"；见 10.2 和待决定项 |
| tianting-android `ServerE2eTest`、`AccountE2eTest.secrets_never_reach_logcat` | 只看本进程从用例开始以来的日志 | Frank 决定接受（A18）；不再清空缓冲区，断言不变 |
| tianting-ios 03-08 | 两种文案都认，按实际路径断言 | PRD v1.2 明确改了（I3） |

没有删除或跳过安全相关的断言。调查时临时加在产品代码和测试里的诊断日志（`TTDIAG`）都已删除，没有提交。

### 10.4 还要 Frank 决定的

1. **03-45 那 1 次被拒绝的刷新**：要不要改三套 SDK，在 access 到期等于 `absoluteExpiresAt` 时收到 `kicked/tokenExpired` 直接回登录页（不再发刷新），让 AC-ACC-03-43 在真实服务端上也是"请求数为 0"。不改的话，建议 AC 写明"上限那一秒服务端踢线后的 1 次刷新被 4019 拒绝"可以接受。
2. **组合内协程的其他 SDK 调用**：这次只改了聊天页、发起单聊、`rememberRelation`。其他页面（群资料、通讯录等约 30 处）是同一种写法，正式运行没有问题，只在测试调度器下会在 SDK 线程上恢复。要不要统一改，由 Frank 定。
