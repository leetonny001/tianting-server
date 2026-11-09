# 前端任务单：天听聊天产品第一批（三套 SDK、H5、Android、iOS、管理后台页面，C1、C2）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.3](../prd/PRD-chat-v1.3.md)（冻结）第 5、6、13、14.1、15 节；[PRD-ui v1.2](../prd/PRD-ui-v1.3.md)（界面规则 B-1～B-8，第 6 节视觉）；[SDK 设计说明](../sdk/README.md)；[errors.json](../../protocol/errors.json) |
| 范围 | **只有 C1、C2 和公共部分**（CH-27）。撤回、引用、@、置顶、举报、设备通知的接口和界面不做；【全部】的 AC 只验证到 C2 为止的页面（第 15 节累计表） |
| 负责 | 前端（AI） |
| 仓库 | tianting-sdk-ts、tianting-sdk-kotlin（`sdk-core` 纯 JVM）、tianting-sdk-swift、tianting-web（H5）、tianting-android、tianting-ios、tianting-admin |
| 编号 | **FE-600 起**。公共 FE-600～602，C1 FE-610～620，C2 FE-630～640 |
| 端的范围 | PC（Tauri）排除：`tianting-web` 宽屏界面照常实现，不验收打包形态（第 2.5 节）。Web Push、拍照不做 |
| 完成定义 | 任务内的 AC 有通过的测试（SDK：假 HTTP + `FakeTransport` + 假时钟 + 共享向量；H5：Vitest、Playwright；原生：Compose UI Test、XCUITest），用例编号见 [TC-ACC](../testcases/TC-ACC.md)、[TC-FILE](../testcases/TC-FILE.md)、[TC-CHAT](../testcases/TC-CHAT.md)。**CI 现在不运行（P-21），原生 CI 也缺 P-15，真机和其他浏览器在 P-13**，结果都要标明环境 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。工具只查"有一个任务"，**端到端 AC 和【前端】AC 按端拆开列**，所以同一条 AC 会出现在多个任务里。标了（Android、iOS）的只进原生任务，标了（H5）的只进 H5 任务，（三套 SDK）的只进 SDK 任务。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| FE-600 | ✅ 完成（sdk-ts 部分） | 假 HTTP、`client` scope 错误码（TC-CHAT-90-12）、`frames.json` 的 Schema 校验（TC-CHAT-90-09，不新增依赖）、`account-session.json` 执行器（TC-CHAT-90-13，随 FE-610 提交）。AC-CHAT-90-10、90-14、90-15 是三套 SDK 的 AC，这里只是 sdk-ts；`chat-sync.json` 和附件状态机随 FE-630。结果都是本地结果：Node 26、macOS，tianting-sdk-ts@a5dbdef，CI 不运行 |
| FE-610 | 🟡 SDK 部分完成 | `AccountClient` 14 个接口、`tokenProvider`、单飞、串行队列、踢线缓存、4011 比代号都做了，单测 563 个、真实 im-server 集成 46 个（本地，SQLite，tianting-sdk-ts@ba226d7，CI 不运行）通过，typecheck、lint、format:check 通过。变异确认：首轮 57 项全部被测试发现（@b7b45ec）；统一语义和恢复码换代（问题 S1、S3、S4、K5、K7）又做 24 项，23 项被发现，1 项存活是等价变异（定时器不看 `shortOnArrival`，到点后决定刷新的逻辑仍会拦住）。**没有验收的部分**：AC 里的弹窗、页面、"我"页显示属于 FE-613～618，这里只验证 SDK 给出的事件和数据；AC-ACC-03-42 的 `/v1/files/urls` 随 C2 才有，现在用 `/v1/account/events` 代替，FE-630 要补；AC-ACC-03-37 的"100 天到期"和 AC-ACC-04-25 的四条序列在服务端 e2e（BE-422），SDK 只验证 4290 退避和同样显示日期；向量里 `source=kicked` 直接调入口，经过 FakeTransport 的路径在 TC 用例里 |
| FE-601 | ✅ 完成（sdk-kotlin 部分） | 假 HTTP（`sdk-core` 的 `FakeHttp`）、`Clock` 注入点（`AccountClientOptions.clock`，主代码只有默认时钟读系统时间）、`client` scope 错误码（TC-CHAT-90-12）、C1 帧的 Schema 校验（TC-CHAT-90-09）、`account-session.json` 执行器（TC-CHAT-90-13，随 FE-611 提交）。协议副本同步到 1aa4365。`chat-sync.json` 随 FE-631。结果都是本地结果：JDK 21、macOS，tianting-sdk-kotlin@11f1a51、@9495872，CI 不运行 |
| FE-611 | 🟡 SDK 部分完成 | `AccountClient` 14 个接口、`tokenProvider`、单飞、串行队列、踢线缓存、4011 比代号、先落盘再使用、恢复码确认前只放内存、老 Session 按 demo、"待退出"持久化和补发、系统令牌注销的调用点（`SystemTokenRevoker`）、OkHttp 不隐式重发都做了。单测 756 个、真实 im-server 集成 65 个（其中账号 19 个；本地，SQLite，CI 不运行）通过，`:sdk-android:compileDebugKotlin` 通过，tianting-sdk-kotlin@114fc54。变异确认：@b1edf51 之前 128 项全部被测试发现（首轮 122 项，5 项存活、1 项写错，补测试后重跑；只加了测试，实现没有再改）；统一语义和恢复码换代（问题 S1、S3、S4、K5、K7）又做 22 项，21 项被发现，1 项存活是等价变异（同 FE-610）。**没有验收的部分**：AC-ACC-03-7、03-11、03-13、03-18、03-20、04-6 的层级是原生 UI 测试（模拟器），这里没有运行，只验证了 SDK 这一层，页面和杀进程属于 FE-615、FE-616；Keystore 存储的实现在 App；系统令牌注销只验证调用点存在于替身，FCM 随 C3；AC-ACC-03-42 的 `/v1/files/urls` 随 C2，现在用 `/v1/account/events` 代替，FE-631 要补；AC-ACC-03-37、04-25 只验证到 SDK 部分（同 FE-610）；AC-ACC-03-20 的 5 分钟只推进了 SDK 钟和假服务端，真实服务端上没有推进时钟，时间部分看 TC-ACC-03-19；集成测试开了 `IM_TEST_DISABLE_AUTH_LOCK=1`，真实限流没有经过 SDK；`sdk-android` 只编译和跑了 Robolectric 单测，仪器测试没有跑 |
| FE-602 | ✅ 完成（sdk-swift 部分） | 假 HTTP（`TiantingIMTesting` 的 `FakeHttp`）、`IMClock` 注入点（`AccountClientOptions.clock`）、`client` scope 错误码（TC-CHAT-90-12）、C1 帧的 Schema 校验（TC-CHAT-90-09）、`account-session.json` 执行器（TC-CHAT-90-13，随 FE-612 提交）。AC-CHAT-90-10、90-14、90-15 是三套 SDK 的 AC，这里只是 sdk-swift；`chat-sync.json` 和附件状态机随 FE-632。本地结果：macOS，`swift test`，tianting-sdk-swift@2d65015，CI 不运行 |
| FE-612 | 🟡 SDK 部分完成 | `AccountClient` 14 个接口、`tokenProvider`、单飞、串行队列、踢线缓存、4011 比代号、先落盘再使用、恢复码确认前不给落盘回调、老 Session 按 demo、"待退出"通过回调持久化和补发、系统令牌注销的接口（`unregisterSystemPushToken`）、URLSession 不隐式重发都做了。`swift test` 全量通过：单测 368 个、真实 im-server 集成 67 个（其中账号 16 个；本地，macOS，SQLite，CI 不运行），tianting-sdk-swift@54e4f04。变异确认：@49b73d9 之前 113 项全部被发现，其中 6 项是测试挂起到 240 秒超时才发现；首轮 5 项存活，只加了测试后重跑这 5 项。之后又加了一个只改测试的用例，实现没有再改。统一语义和恢复码换代（问题 S1、S3、S4、K5、K7）又做 20 项全部被发现，其中 2 项是用例在假对象的 `fatalError` 上崩溃才发现。**没有验收的部分**：AC-ACC-03-7、03-11、03-13、03-18、03-20、04-6 的层级是原生 UI 测试（模拟器），这里没有运行，只验证了 SDK 这一层，页面和杀进程属于 FE-617、FE-618；Keychain 存储的实现在 App；`SystemEventsMonitor` 接到 `AccountClient` 的联网和回前台没有在 iOS 上运行过；系统令牌注销只验证调用点，APNs 随 C3；AC-ACC-03-42 的 `/v1/files/urls` 随 C2，现在用 `/v1/account/events` 代替，FE-632 要补；AC-ACC-03-37、04-25 只验证到 SDK 部分（同 FE-610）；AC-ACC-03-20 的 5 分钟只推进了 SDK 钟和假服务端；URLSession 的结论只在 macOS、回环、HTTP/1.1 上测过，iOS、HTTP/2、HTTP/3、真实网关没有测；集成测试开了 `IM_TEST_DISABLE_AUTH_LOCK=1` |
| FE-619 | ✅ 完成（页面部分） | tianting-admin@1f31770（**Chrome，本地；CI 不运行**）。邀请码页（列表、生成、停用，`account.invite` 守卫菜单和路由）、用户详情的账号信息、审计动作名。Vitest 103 个、Playwright 54 个通过（2 个截图用例默认跳过），对真实 console-server、im-server（invite 模式）、Postgres，生产 CSP 响应头；lint、typecheck、format:check、check:dist 通过。变异确认 30 项全部被发现。覆盖 TC-ACC-09-01（页面和后台部分）、09-03、09-04、09-05（前端范围）、09-06、09-09、09-11。**没有验收的部分**：AC-ACC-09-1 的注册在这里直接调 `/v1/account/register`，三端注册页和"邀请码无效"提示属于 FE-614、FE-616、FE-618、FE-620；`accountUnavailable` 在页面上只有 Vitest 和截图（响应是改写的），真实故障的接口部分在 console-server 的 `TestTC_ACC_09_11_UserDetail`；`pagehide` 清明文只有 Vitest，真实浏览器的往返缓存没有验证；后台没有深色模式，只走查了亮色；Edge、Safari、Firefox 没有运行（P-13）。新问题见 A1～A6 |
| FE-613 | ✅ 完成（H5） | tianting-web@3daa769（**Chrome，本地；CI 不运行**）。登录、注册、忘记密码、保存恢复码、撤销注销、隐私说明、配置失败不猜模式。和 FE-614 合计 Vitest 259 个、Playwright 117 个（真实 im-server，SQLite），变异 34 个都能抓到。没有执行：WebKit（未安装）；AC-ACC-10-3 的确认记录在本机，不在服务端（H5）。依赖 tianting-sdk-ts@ba226d7 重新构建的 `dist`（H8）。问题见 H1～H12 |
| FE-614 | ✅ 完成（H5） | tianting-web@3daa769（**Chrome，本地；CI 不运行**）。会话、多标签页互斥（真实复制标签页、冻结页面都测了）、账号安全、改昵称、退出登录。**只用假接口或部分验证**：AC-ACC-03-26（4019 是假响应）；AC-ACC-03-37（只验第 21 次 4290，100 天和绝对上限不在这里）；AC-ACC-03-39、03-43、04-17、04-27（Vitest 假 HTTP）；AC-ACC-03-42（用 `/v1/account/events` 代替 C2 的 `files/urls`，1 个在途请求）；AC-ACC-09-1（H5 注册部分，用量查管理接口，后台页面在 FE-619）；AC-CHAT-90-8、90-9（只有 Chrome，三端在 FE-620）。**H1 和 CH-26 不完全相符，待 Frank 决定** |
| FE-615 | 🟡 部分完成（Android） | tianting-android@1a9f2f9（代码），用例在 @980f358（**模拟器，本地；CI 不运行，真机（P-13）未执行**，Pixel_10_Pro_XL，API 37）。登录、注册、忘记密码、保存恢复码（确认前不写存储，返回键拦截）、撤销注销、隐私说明、配置失败不猜模式、构建开关和 `demoLogin` 同时为真才显示演示表单。`AccountUiTest` 39 个（假服务端），`AccountE2eTest` 里的注册、登录、找回用例（真实 im-server，SQLite），真实杀进程用 `scripts/run-tests.sh kill`。**没有验证**：AC-ACC-10-3 的服务端记录（没有接口，见 H5、A2），确认记在本机；AC-ACC-09-1 只验证 Android 注册（真实邀请码，第 3 次提示"邀请码无效"），后台页面属于 FE-619，没有和 console 一起跑；隐私说明打开成功用的是测试地址 `/readyz`，法律文字还没有 |
| FE-616 | 🟡 部分完成（Android） | tianting-android@980f358（**模拟器，本地；CI 不运行，真机（P-13）未执行**）。Keystore 加密文件存完整 Session 和"待退出"，账号安全（改密、重新生成、退出其他设备、最近登录记录、注销）、弹窗（`fatalText` 的 `Session` 分支，`Kicked` 带日期）、`securityNotice`、"我"页改昵称、断网退出、界面不等 `logout()`（K2）。`SessionUiTest` 38 个（假服务端 + [SDK 钟]），`AccountE2eTest` 19 个（真实 im-server，含真实飞行模式），`AccountKillE2e` 4 个场景（真实杀进程：落盘后、响应到达前、保存页、logout 等响应时）。变异 22 项全部被发现（首轮 21 项，1 项存活，补用例后重跑这一项）。**没有验证或只验证了一部分**：AC-ACC-03-37 只有场景一（4290 留在当前页），场景二、三在服务端和 SDK；AC-ACC-03-42 用 `/v1/account/events` 代替 `/v1/files/urls`，FE-636 要补；AC-ACC-04-6 的系统令牌注销随 C3；AC-ACC-04-25 跑了甲、丙、丁，乙没有在 App 上跑；AC-ACC-06-5 其他端的 10 分钟没有在界面上推进钟，只查了服务端的新昵称，TTL 看 SDK 的用例；AC-ACC-02-13 的"2 秒"测试留了 1 秒余量（3 秒内），不是严格 2 秒；AC-CHAT-90-9 没有运行 Espresso AccessibilityChecks（A10）；AC-ACC-03-10、03-23、03-39、03-43、04-27 只对假服务端跑。问题见 A1～A11 |
| FE-617 | 🟡 部分完成（iOS） | tianting-ios@387e52e（代码），用例在 @fe90c53（**模拟器，本地；CI 不运行，真机（P-13）未执行**，iPhone 16 Pro，iOS 18.4）。登录、注册、忘记密码、保存恢复码（确认前不写 Keychain，根视图替换全部内容，下面没有页面，返回手势没有去处）、撤销注销、隐私说明、配置失败不猜模式、`Config.swift` 和 `demoLogin` 同时为真才显示演示表单。`AccountFakeServerTests` 46 个（测试进程里的假服务端，HTTP + WebSocket），`AccountE2ETests` 18 个（真实 im-server，SQLite）。**没有验证**：AC-ACC-10-3 的服务端记录（没有接口，同 H5），确认记在本机，按用户分开；AC-ACC-09-1 只验证 iOS 注册（真实邀请码，第 3 次提示"邀请码无效"），后台页面属于 FE-619，没有和 console 一起跑；隐私说明打开成功用的是假服务端的页面，法律文字还没有；新密码框用 `.oneTimeCode` 关强密码覆盖（I8）只在模拟器验证 |
| FE-618 | 🟡 部分完成（iOS） | tianting-ios@fe90c53（**模拟器，本地；CI 不运行，真机（P-13）未执行**）。Keychain（`AfterFirstUnlockThisDeviceOnly`，读属性确认）存完整 Session，"待退出"是另一条记录，`clear` 后还在；账号安全（改密、重新生成、退出其他设备、最近登录记录、注销）、弹窗（`fatalText` 的 `session` 分支，`kicked` 带日期）、`securityNotice`、"我"页改昵称、断网退出、界面不等 `logout()`（K2）。全量 XCUITest 113 个通过（含 Chat、Group、Social 等回归和账号 64 个），`check_e2e_leftovers` 通过；单测 109 个通过、4 个跳过（真实 Keychain 用例，无宿主测试包没有权限，Keychain 行为由 XCUITest 经调试探针验证）。变异 16 项全部被发现（首轮 15 项，1 项存活，补断言后重跑这一项）。**没有验证或只验证了一部分**：`SystemEventsMonitor` 回前台用真实通知验证了，**网络路径变化没有验证**（模拟器共用 Mac 的网络，单测只跑到 `NWPathMonitor` 的首次更新）；AC-ACC-03-11 的"网络不可用"靠调试开关 `TT_TEST_OFFLINE` 加断开假服务端，不是真实飞行模式；AC-ACC-03-37、03-23、04-17 只有 SDK 用例，App 层没有单独跑；AC-ACC-03-42 用 `/v1/account/events` 代替 `/v1/files/urls`，FE-638 要补；AC-ACC-04-6 的系统令牌注销随 C3；AC-ACC-04-25 只在 App 上跑了"另一台设备改密"，四条序列看服务端和 SDK；AC-ACC-06-5 其他端的 10 分钟没有在界面上推进钟，只查了服务端的新昵称；AC-ACC-03-10、03-12、03-13、03-26、03-39、03-43、04-27 只对假服务端跑，用真实时间，不是 [SDK 钟]；AC-CHAT-90-9 只跑了系统审计 `.sufficientElementDescription`，没有用 Accessibility Inspector。问题见 I1～I9 |
| FE-620 | 🟡 验收完成，有待决定项 | 报告 [c1-acceptance-2026-11-07.md](../reports/c1-acceptance-2026-11-07.md)：C1 和【全部】的 AC 共 177 条，通过 153、部分通过 20、未执行 4（AC-ACC-03-43 由协调方核对后从"通过"改为"部分通过"，见 F2；**Chrome，本地；模拟器，本地；CI 不运行，WebKit 和真机（P-13）未执行**）。最终提交：tianting-web@db2dccf、tianting-admin@76cd2c0、tianting-android@8757053、tianting-ios@2fabcce、tianting-sdk-kotlin@e65b21a、tianting-sdk-swift@01c7c32、tianting-console-server@9dc3f5c。补了真实服务端的 03-10、03-26、03-41、03-45、04-25 甲乙丙丁和场景二、09-1 后台到 H5 的完整链路、三端提醒和踢线同时出现的截图，sdk-kotlin、sdk-swift 的六种提醒。**待决定**：iOS 两条新用例（03-08、03-12）和真实服务端不符，没改期望，没提交（见报告第 3 节）；F2、A16～A18、I10；其他仓库 README 里的产品名 |
| FE-630 | 🟡 SDK 部分完成 | tianting-sdk-ts@32ba99f（**本地结果，CI 不运行**：Node、macOS；单测 603 个、真实 im-server 集成 57 个，其中文件 5 个对真实 im-server（`IM_FILES_ENABLED=1`，SQLite）+ `test/integration/fake_s3.py`）。`FileClient`（申请、PUT、complete 的 5032 退避 3 次、`DELETE` 取消、`/v1/files/urls` 同一事件循环合并成一批 ≤ 50 并缓存地址）、`StaticTokenAuth`（演示 token）、`AccountClient.authorized`（文件接口的 4011 按规则 7）；`IMClient.sendImage/sendFile/cancelUpload/getFileUrls`，状态机处理中 → 上传中 → 待发送（`queued`）→ 发送中 → 已发送，全局并发 1、按选择顺序，文字可以越过，重发复用已 complete 的 `fileId`，`send` 4096 有源就重新上传（不占自动重发），4292、`uploadPendingLimit` 不重试；Store 单调合并（`deleted`、文件 `expired` 只从假变真，`expiredIds` 跨重置保留）；重连只重拉 `modVer` 变了的会话（没有 `modVer` 时全部重拉）；`filesExpired` 事件，已知过期的不申请地址。AC-ACC-03-42 用原接口 `/v1/files/urls` 补了单测（在途 4011 用本地新 token 重试 1 次，不刷新）。真实服务端：TC-FILE-02-01、03-01 的 sdk-ts 部分（收发两边 `size`、`mime`、宽高一致，下载 SHA-256 一致）、非参与者 4048、真实 4292、上传中取消后 `complete` 404。变异 23 项 22 项被发现，存活 1 项是等价变异（迟到的 `expired=false` 时重新标记过期）。**没有验收的部分**：提示文案、页面、懒加载、H5 的 XHR 进度（`XhrUploadTransport` 只写了，Node 里没有 XHR，没有运行）属于 FE-633、FE-634；`chat-sync.json` 还没有（BE-403），合并和 `modVer` 只有各 SDK 自己的单测；假 S3 不验签名、只验调度。问题见 D1～D14 |
| FE-631 | 🟡 SDK 部分完成 | tianting-sdk-kotlin@c4d7fa1（**本地结果，CI 不运行**：JDK 21、macOS；`:sdk-core:test` 803 个、`:sdk-core:integrationTest` 76 个，其中文件 5 个对真实 im-server + 假 S3；`:sdk-android:compileDebugKotlin` 和 Robolectric 4 个通过）。语义同 FE-630；另有本地库升到第 2 版（`attachments`、`mod_vers` 两张表，第 1 版原地升级，原有会话和消息保留，有用例），`PROCESSING`、`UPLOADING`；冷启动处理中、上传中转失败、不自动上传；沙盒：SDK 把 App 交来的文件复制进 `attachmentDir`，发出后删除，失败保留，退出登录清空；下载缓存键 `fileId + variant`（`fileCacheDir`），失败删半截文件，重试重新申请地址、从 0 开始，过期和删除时清除；`DeletedRefs` 带 `expired`；`modVer` 基线存在本地库。核对第 21 节 8：Kotlin 唯一的发送入口是 `send(convId, text)`，`dispatch` 只发 text、image、file，`Protocol.kt` 的 CUSTOM 只用于解码，**没有发送 custom 的路径**。变异 20 项全部被发现（首轮 1 项存活，只补用例后重跑）。**没有验收的部分**：Android 上的 SQLite 升级只在 JVM SQLite（jdbc）跑过；`OkHttpObjectTransport` 的进度和流式写文件只在 JVM 回环跑过；页面、真实杀进程、相册删除原图属于 FE-635、FE-636 |
| FE-632 | 🟡 SDK 部分完成 | tianting-sdk-swift@2f8501c（**本地结果，CI 不运行**：macOS，`swift test`；单测 423 个、集成 76 个，其中文件 3 个对真实 im-server + 假 S3）。语义同 FE-631：本地库第 2 版（`outbox` 表，只存没发出的附件和沙盒文件名，不存地址），沙盒 `attachmentDirectory`（按用户分目录），下载缓存 `fileCacheDirectory`，`download(fileId:variant:progress:)` 取消 Task 即中止。URLSession（接着问题 S2，真实 socket 计数，macOS、回环、HTTP/1.1）：每次 PUT 用新的会话，断在半路的 PUT 只到达 1 次；**GET 没有响应时 URLSession 会自己重发**（2 次 GET 到达 4 次），没有阻止（D9）。变异 23 项 22 项被发现，存活 1 项是等价变异。L1 替换了原来断言"上限前被踢还会刷新 1 次"的 C1 单测（改成 `testTC_ACC_03_45_L1_kickedAtAbsoluteLimitEndsWithoutRefresh`），依据是 Frank 的决定。全量集成有一次卡在已有用例 `SocialIntegrationTests.testSearchablePrivacyAcrossDevices`（900 秒超时），单独跑和再跑全量都过，没有复现。**没有验收的部分**：iOS 上的 URLSession 进度、HTTP/2；页面和杀进程属于 FE-637、FE-638 |
| 其余 | ⬜ 未开始 | Kotlin、Swift 照 [SDK 设计说明第 6 节](../sdk/README.md) 做 |

## 总览

| 第 18 节的列 | 公共 | C1 | C2 | 任务 |
|---|---|---|---|---|
| sdk-ts | 1 | 3.5 | 3.5 | FE-600、FE-610、FE-630 |
| sdk-kotlin | 1 | 4 | 5.5 | FE-601、FE-611、FE-631 |
| sdk-swift | 1 | 4 | 5.5 | FE-602、FE-612、FE-632 |
| H5 | 0 | 5 | 6 | FE-613（2.5）、FE-614（2.5）；FE-633（3）、FE-634（3） |
| Android | 0 | 5.5 | 8 | FE-615（2.5）、FE-616（3）；FE-635（4）、FE-636（4） |
| iOS | 0 | 5.5 | 8 | FE-617（2.5）、FE-618（3）；FE-637（4）、FE-638（4） |
| console + 后台页面（页面部分） | 0 | 1 | 1 | FE-619、FE-639；其余 CS-200～202 |
| 测试（前端部分） | 0 | 3.5 | 4 | FE-620、FE-640；其余 BE-422、BE-440 |
| 文档（前端部分） | 0 | 0.5 | 0 | FE-620 |
| **本单合计** | **3** | **32.5** | **41.5** | **77** |

**差异说明**：和第 18 节一致（按列拆分，不是独立重估，**低置信**）。三份任务单合计 145，对照见 [backend-chat](backend-chat.md)。把每个端拆成两个任务只是为了每个任务不超过 4 人日，两个任务的 SDK 接口是同一套。

**依赖顺序**：BE-400 → FE-600 → FE-601、FE-602 → FE-610 → FE-611、FE-612 → FE-613、FE-615、FE-617 → FE-614、FE-616、FE-618 → FE-619 → FE-620 →（C1 校准）→ BE-403 → FE-630 → FE-631、FE-632 → FE-633、FE-635、FE-637 → FE-634、FE-636、FE-638 → FE-639 → FE-640。

---

## 公共部分

### FE-600 sdk-ts：假 HTTP、client 错误码、共享向量执行器
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-ts
- 依赖：BE-400、BE-402
- AC：AC-CHAT-90-10、AC-CHAT-90-14、AC-CHAT-90-15
- 内容：
  1. 第 4.5、13.1 节：假 HTTP（现有 `FakeTransport` 只有 WS）；`Clock` 已有，确认 HTTP 也用它。
  2. 错误枚举加入 `client` scope 的码（F-SYS-01 一致性）；`kickReasons` 忽略不认识的值。
  3. 向量执行器能跑 `account-session.json`，C2 再加 `chat-sync.json`（FE-630）。sdk-ts 现在只验下行不抛异常，增加 Schema 校验（AC-CHAT-90-10）。
- 工作量：1 人日（sdk-ts，公共）

### FE-601 sdk-kotlin：假 HTTP、SDK 钟注入、共享向量
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-kotlin
- 依赖：FE-600
- AC：AC-CHAT-90-10、AC-CHAT-90-14、AC-CHAT-90-15
- 内容：
  1. `sdk-core` 增加假 HTTP 和 [SDK 钟] 注入点（第 4.4 节，sdk-ts 已有，其他两套要有同样的注入点）；不引入 Android 依赖。
  2. 错误枚举、向量执行器，同 FE-600。
  3. Play 服务可用性替身、系统通知授权替身（第 4.5 节）属于 C3，不在这里。
- 工作量：1 人日（sdk-kotlin，公共）

### FE-602 sdk-swift：假 HTTP、SDK 钟注入、共享向量
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-swift
- 依赖：FE-600
- AC：AC-CHAT-90-10、AC-CHAT-90-14、AC-CHAT-90-15
- 内容：同 FE-601，放在 Swift 包里。
- 工作量：1 人日（sdk-swift，公共）

---

## C1 账号

### FE-610 sdk-ts：AccountClient、tokenProvider、刷新单飞
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-ts
- 依赖：FE-600、BE-400
- AC：AC-ACC-02-13、AC-ACC-03-10、AC-ACC-03-12、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-05-7、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-06-5、AC-ACC-09-1、AC-CHAT-90-15
- 内容：
  1. 第 13.1 节：`AccountClient.register/login/refresh/logout/logoutOthers/changePassword/recover/regenerateRecoveryCode/ackRecoveryCode/updateProfile/getConfig/listEvents/requestDelete/cancelDelete`；`IMClient.connect(tokenProvider)`，保留 `connect(token)`。
  2. F-ACC-03 规则 5：不足 6 小时刷新；启动、回前台、定时器、`kicked/tokenExpired` 四个入口单飞；30 秒超时，迟到响应丢弃；启动已过期先刷新再 `auth`；换 token 用 1000 断开后立即重连；改密、找回、退出其他设备和刷新进同一个串行队列，队列内的改密要先刷新时直接执行；`accessExpiresAt == absoluteExpiresAt` 时不再刷新；HTTP 和 WS 共用 token 来源。
  3. 规则 6：`superseded` 按 `retryAfterSeconds` 最多 3 次；`replayDetected` 弹窗回调；429、网络错误、5xx 留在当前页。规则 6a：`recoverBefore` 统一给 App。
  4. 规则 7：收到 4011 先比较本地代号，比被拒绝的新就直接重试；**三套 SDK 现在收到 4011 都不重连，要改**；演示模式不变。
  5. F-ACC-04 规则 3：改密请求进行中把踢线缓存，等响应（30 秒），成功先换 token 再忽略旧 `sid` 的踢线。
  6. 日志不打印 token、refresh、恢复码（第 13.1 节）。H5 的待退出不适用（只存 `sessionStorage`）。
- 工作量：3.5 人日（sdk-ts）

### FE-611 sdk-kotlin：AccountClient、tokenProvider、刷新单飞
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-kotlin
- 依赖：FE-601、FE-610
- AC：AC-ACC-02-13、AC-ACC-03-7、AC-ACC-03-10、AC-ACC-03-11、AC-ACC-03-12、AC-ACC-03-13、AC-ACC-03-18、AC-ACC-03-20、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-6、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-05-7、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-06-5、AC-ACC-09-1、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-610（第 13.1 节），放在 `sdk-core`。
  2. **新 token 先落盘再使用**（R-CF-09）；确认恢复码之前 token 只放内存（F-ACC-05 规则 1，RS5-13）。Session 增加 `refreshToken`、`mode`；老 `{userId, token}` 按 `demo` 处理（R-CF-11）。
  3. "待退出"持久化 `sid` 和 refresh，联网补发（F-ACC-04 规则 1）。系统令牌注销的调用点留接口，FCM 接入随 C3。
  4. OkHttp 的隐式重试要确认不会把同一个 refresh 请求多发（第 21 节 16）；调试日志不打印 URL 查询串。`IMClient.kt` 第 916～932 行的重连入口和 4011 分支要改。
- 工作量：4 人日（sdk-kotlin）

### FE-612 sdk-swift：AccountClient、tokenProvider、刷新单飞
- 优先级：P0
- 批次：C1
- 仓库：tianting-sdk-swift
- 依赖：FE-602、FE-610
- AC：AC-ACC-02-13、AC-ACC-03-7、AC-ACC-03-10、AC-ACC-03-11、AC-ACC-03-12、AC-ACC-03-13、AC-ACC-03-18、AC-ACC-03-20、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-6、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-05-7、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-06-5、AC-ACC-09-1、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-610、FE-611。`Client.swift` 第 509 行起的重连和 4011 分支要改。
  2. URLSession 隐式重试同样确认。Session 由 App 的 Keychain 保存，SDK 只给回调。
- 工作量：4 人日（sdk-swift）

### FE-613 H5：登录、注册、恢复码、配置
- 优先级：P0
- 批次：C1
- 仓库：tianting-web
- 依赖：FE-610、BE-408
- AC：AC-ACC-01-12、AC-ACC-01-13、AC-ACC-01-18、AC-ACC-02-8、AC-ACC-02-9、AC-ACC-02-13、AC-ACC-05-5、AC-ACC-05-7、AC-ACC-05-10、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-07-5、AC-ACC-07-6、AC-ACC-07-7、AC-ACC-07-8、AC-ACC-07-9、AC-ACC-10-1、AC-ACC-10-3
- 内容：
  1. 第 13.3 节登录页：账号登录、注册、忘记密码、保存恢复码、撤销注销、隐私说明入口；演示表单按 `demoLogin` 和 `VITE_DEMO_LOGIN` 两者同时为真才显示（F-ACC-07 规则 4）。AC-UI-03-9 的页面作废。
  2. 第 13.4 节页面状态：提交中、4004、4000、4030、4095、4290、4294、5030、网络错误；复制成功和失败。
  3. `GET /v1/config` 失败不猜测模式；已登录用缓存（F-ACC-07 规则 4、异常）。
  4. `tianting-web/deploy/nginx.conf`、`vite.config.ts` 转发 `/v1/`（第 4.2 节规则 6）。
  5. 隐私说明版本更新提示一次（F-ACC-10 规则 2）。
- 工作量：2.5 人日（H5）

### FE-614 H5：会话、多标签页、账号安全
- 优先级：P0
- 批次：C1
- 仓库：tianting-web
- 依赖：FE-613
- AC：AC-ACC-03-9、AC-ACC-03-10、AC-ACC-03-12、AC-ACC-03-14、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-06-5、AC-ACC-08-6、AC-ACC-09-1、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. `src/lib/session.ts`：只放 `sessionStorage`（CH-26）；确认恢复码前只放内存。
  2. **多标签页**（F-ACC-03 规则 9）：启动时 `navigator.locks` 按 `sid` 取互斥锁，拿不到清 token 回登录页；没有这个接口退回 `BroadcastChannel`；刷新前再确认。冻结页面的表现要实测（第 21 节 16，和 CH-26 不符报告 Frank）。
  3. "账号安全"：改密、重新生成恢复码、退出其他设备、最近登录记录、注销；演示会话隐藏。弹窗文案按 F-ACC-03 规则 4、6a。`securityNotice` 提示。"我"页改昵称。
  4. 头像编辑入口随 FE-633。AC-UI-15-3、AC-UI-16-6 的改写由 BE-408 落实。
- 工作量：2.5 人日（H5）

### FE-615 Android：登录、注册、恢复码、配置
- 优先级：P0
- 批次：C1
- 仓库：tianting-android
- 依赖：FE-611、BE-408
- AC：AC-ACC-01-12、AC-ACC-01-13、AC-ACC-01-18、AC-ACC-02-8、AC-ACC-02-9、AC-ACC-02-13、AC-ACC-05-5、AC-ACC-05-7、AC-ACC-05-10、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-07-5、AC-ACC-07-6、AC-ACC-07-7、AC-ACC-07-8、AC-ACC-07-9、AC-ACC-10-1、AC-ACC-10-3
- 内容：
  1. 同 FE-613 的页面和状态；构建开关 `build.gradle.kts`。
  2. 恢复码复制用系统剪贴板；保存页拦截返回键。确认前 token 不写 `SessionStore.kt`。
- 工作量：2.5 人日（Android）

### FE-616 Android：会话、账号安全、断网退出
- 优先级：P0
- 批次：C1
- 仓库：tianting-android
- 依赖：FE-615
- AC：AC-ACC-03-7、AC-ACC-03-8、AC-ACC-03-10、AC-ACC-03-11、AC-ACC-03-12、AC-ACC-03-13、AC-ACC-03-18、AC-ACC-03-20、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-6、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-06-5、AC-ACC-08-6、AC-ACC-09-1、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. `core/SessionStore.kt`：Keystore 加密文件增加 `refreshToken`、`mode`（F-ACC-03 规则 8）。
  2. 飞行模式显示缓存和"网络不可用"；冷启动和回前台触发检查。
  3. 断网退出（F-ACC-04 规则 1）；"账号安全"；弹窗；`securityNotice`；"我"页改昵称。
  4. `setAppState`、通知设置属于 C3。
- 工作量：3 人日（Android）

### FE-617 iOS：登录、注册、恢复码、配置
- 优先级：P0
- 批次：C1
- 仓库：tianting-ios
- 依赖：FE-612、BE-408
- AC：AC-ACC-01-12、AC-ACC-01-13、AC-ACC-01-18、AC-ACC-02-8、AC-ACC-02-9、AC-ACC-02-13、AC-ACC-05-5、AC-ACC-05-7、AC-ACC-05-10、AC-ACC-05-15、AC-ACC-05-19、AC-ACC-07-5、AC-ACC-07-6、AC-ACC-07-7、AC-ACC-07-8、AC-ACC-07-9、AC-ACC-10-1、AC-ACC-10-3
- 内容：
  1. 同 FE-613 的页面和状态；构建开关 `Config.swift`。
  2. 保存页禁用返回手势。确认前 token 不写 Keychain。
- 工作量：2.5 人日（iOS）

### FE-618 iOS：会话、账号安全、断网退出
- 优先级：P0
- 批次：C1
- 仓库：tianting-ios
- 依赖：FE-617
- AC：AC-ACC-03-7、AC-ACC-03-8、AC-ACC-03-10、AC-ACC-03-11、AC-ACC-03-12、AC-ACC-03-13、AC-ACC-03-18、AC-ACC-03-20、AC-ACC-03-23、AC-ACC-03-26、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-03-39、AC-ACC-03-42、AC-ACC-03-43、AC-ACC-04-5、AC-ACC-04-6、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-17、AC-ACC-04-25、AC-ACC-04-27、AC-ACC-06-5、AC-ACC-08-6、AC-ACC-09-1、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：同 FE-616。`Core/SessionStore.swift`，Keychain `AfterFirstUnlockThisDeviceOnly` 不变。
- 工作量：3 人日（iOS）

### FE-619 管理后台：邀请码页面和用户详情
- 优先级：P0
- 批次：C1
- 仓库：tianting-admin
- 依赖：CS-200
- AC：AC-ACC-09-1、AC-ACC-09-11
- 内容：
  1. 新页面"邀请码"（第 14.1 节）：列表（后 4 位、已用次数、到期、备注、状态含"已停用（自动）"）、生成（数量 ≤ 100、`maxUses` 1～1000）、停用。明文只显示一次，提示复制。权限点 `account.invite` 守卫菜单和路由。
  2. 用户详情增加账号信息；**没有"重置密码"类操作**。
  3. 错误码走现有 API 客户端（FE-400）的表；5031 "审计服务不可用，操作未执行"。
- 工作量：1 人日（console + 后台页面列）

### FE-620 C1 三端验收和 SDK 文档
- 优先级：P0
- 批次：C1
- 仓库：tianting-web、tianting-android、tianting-ios、tianting-server（`docs/sdk/README.md`）
- 依赖：FE-614、FE-616、FE-618、FE-619
- AC：AC-ACC-02-13、AC-ACC-03-34、AC-ACC-03-37、AC-ACC-04-10、AC-ACC-04-16、AC-ACC-04-25、AC-ACC-05-19、AC-ACC-09-1、AC-CHAT-90-8、AC-CHAT-90-9、AC-CHAT-90-15
- 内容：
  1. 按 TC-ACC 落地 Playwright（Chromium + WebKit）、Compose UI Test、XCUITest；三端 × 三套 SDK 的端到端 AC 逐端留结果。
  2. AC-CHAT-90-8：扫描账号页面的文案和图标。AC-CHAT-90-9：axe-core、深色模式、键盘（C4 的消息菜单不在这批）。
  3. 更新过期的 `docs/sdk/README.md`（还写着 Kotlin、Swift"待实现"）：账号、token 提供者、刷新规则（第 16.1 节，文档 0.5）。
  4. 真机回归（P-13）、原生 CI（P-15）没到位：结果写"模拟器，本地"。
- 工作量：4 人日（测试 3.5 + 文档 0.5）

---

## C2 图片和文件

### C1 遗留（P-35，Frank 2026-11-09 "都按建议"）
- L1（随 FE-630、FE-631、FE-632）：access 到期等于 `absoluteExpiresAt` 时收到 `kicked/tokenExpired`，SDK 直接走登录失效、**不再发刷新**。三端 TC-ACC-03-45 到期后的断言改回"刷新请求数为 0、不重连"（AC-ACC-03-43），通过后验收报告里 03-43 改为通过。
  - **L1 处理结果（2026-11-09）**：三套 SDK 收到 `kicked/tokenExpired`（或只有关闭码 4011）、本地 token 的到期等于 `absoluteExpiresAt` 时直接按登录失效结束（`sessionExpired`），不发刷新：tianting-sdk-ts@32ba99f、tianting-sdk-kotlin@c4d7fa1、tianting-sdk-swift@2f8501c，三套都有"上限前最后一秒被踢""只有 4011""普通 token 仍刷新 1 次"的用例，去掉实现后前两个失败。三端 TC-ACC-03-45 到期后改回"刷新请求数为 0、不重连"：tianting-web@f7b76a8（`IM_REFRESH_ABSOLUTE=25s`，Chrome，连跑 2 次过，刷新响应 `[]`，1 个 WebSocket）、tianting-android@fd32318（`40s`，emulator-5556，连跑 2 次过，`refresh answers []`，状态只有 CONNECTED → DISCONNECTED）、tianting-ios@bf7374e（`40s`，iPhone 16 Pro，连跑 2 次过）。验收报告 AC-ACC-03-43 改为通过。边界见 D14。本地结果，CI 不运行
- L2（随 FE-635、FE-636 等改到的页面）：Android 组合里直接调 SDK 挂起函数的其余约 30 处，统一改成回主线程再写状态（同 A16、A17 的修法）。
- L3（FE-640 之前）：Android、H5 补真实服务端在线踢线用例（`IM_ACCESS_TTL=30s`，同 iOS 的 03-12）。补不上，AC-ACC-03-12 改回"部分通过"。

### FE-630 sdk-ts：文件接口、状态机、单调合并
- 优先级：P0
- 批次：C2
- 仓库：tianting-sdk-ts
- 依赖：FE-620、BE-401、BE-403
- AC：AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-5、AC-FILE-03-1、AC-FILE-04-10、AC-FILE-04-11、AC-FILE-06-6、AC-FILE-06-12、AC-FILE-07-2、AC-FILE-07-3、AC-FILE-07-4、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-10、AC-CHAT-90-15
- 内容：
  1. 第 13.1 节：`sendImage(conv, processed, {onProgress, signal})`、`sendFile`、`getFileUrls`、`cancelUpload`。图片处理不在 SDK（R-CF-24）。
  2. **F-FILE-07 状态机**：处理中 → 上传中 → 待发送 → 发送中 → 已发送；任何一步可转失败。规则 3、4、5、6：冷启动规则、手动重发复用 `fileId`、4096 回到上传中、同一会话串行、文字可以越过、并发 1。`complete` 5032 退避 3 次。
  3. **F-RCL-03 规则 2、2a 从 C2 开始**（RC2-13）：`store.ts` 第 96～99 行改单调合并（`deleted`、`expired` 只能从假变真，`deletedIds` 优先）；`recheckDeleted` 改成只重拉 `modVer` 变化的会话（`client.ts` 第 467 行）。`recalled`、`quote` 的字段随 C4。
  4. `4292`、`uploadPendingLimit` 不自动重试。批量取地址：一屏合并成一批；在途 4011 用本地新 token 重试（AC-ACC-03-42 已验）。
  5. 日志不打印预签名地址和文件名。
- 工作量：3.5 人日（sdk-ts）

### FE-631 sdk-kotlin：文件接口、状态机、单调合并、本地库升级
- 优先级：P0
- 批次：C2
- 仓库：tianting-sdk-kotlin
- 依赖：FE-630
- AC：AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-5、AC-FILE-03-1、AC-FILE-03-5、AC-FILE-04-10、AC-FILE-04-11、AC-FILE-06-6、AC-FILE-06-12、AC-FILE-07-1、AC-FILE-07-2、AC-FILE-07-4、AC-FILE-07-5、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-9、AC-FILE-07-10、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-630。`Store.kt` 第 17 行增加处理中、上传中状态，本地库升级；第 154～157 行改单调合并；`DeletedRefs` 带 `expired`。
  2. `IMClient.kt` 第 919 行起改成只重拉 `modVer` 变化的会话（现在每次重连都重拉）。
  3. 文件缓存键 `fileId + variant`（F-FILE-04 规则 5），过期时清除。预签名地址不持久化；OkHttp 日志不打印。
  4. 确认 Kotlin 没有发送 `custom` 的路径（第 21 节 8），`Protocol.kt` 第 55 行。
- 工作量：5.5 人日（sdk-kotlin）

### FE-632 sdk-swift：文件接口、状态机、单调合并、本地库升级
- 优先级：P0
- 批次：C2
- 仓库：tianting-sdk-swift
- 依赖：FE-630
- AC：AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-5、AC-FILE-03-1、AC-FILE-03-5、AC-FILE-04-10、AC-FILE-04-11、AC-FILE-06-6、AC-FILE-06-12、AC-FILE-07-1、AC-FILE-07-2、AC-FILE-07-4、AC-FILE-07-5、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-9、AC-FILE-07-10、AC-CHAT-90-15
- 内容：同 FE-631。`Store.swift` 第 120～123 行；`Protocol.swift` 第 17 行；URLSession 日志。
- 工作量：5.5 人日（sdk-swift）

### FE-633 H5：选图、处理、发送、取消、头像
- 优先级：P0
- 批次：C2
- 仓库：tianting-web
- 依赖：FE-630
- AC：AC-ACC-06-6、AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-4、AC-FILE-02-5、AC-FILE-02-10、AC-FILE-02-13、AC-FILE-03-1、AC-FILE-03-6、AC-FILE-04-11、AC-FILE-07-2、AC-FILE-07-3、AC-FILE-07-4、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-10
- 内容：
  1. 聊天页"+"（`features.files` 关时隐藏）；`<input type=file>`、拖放、粘贴；一次最多 9 个。上传进度用 `XMLHttpRequest`。
  2. F-FILE-02 规则 2、3、4：canvas 重新编码，先按 EXIF 方向旋转再去掉，长边 4096，缩略图长边 480 ≤ 512 KiB；格式矩阵；GIF 不重编码；HEIC 解不了提示用文件方式。解码前查像素上限。
  3. 刷新前有离开确认；File 对象只在内存；源文件没了提示"原文件已不可用，请重新选择"（F-FILE-07 规则 4）。
  4. "文件"方式发 jpeg 的提示（F-FILE-03 规则 2）。头像裁剪 512×512（F-ACC-06 规则 3）。
- 工作量：3 人日（H5）

### FE-634 H5：图片和文件的显示、下载、CSP
- 优先级：P0
- 批次：C2
- 仓库：tianting-web
- 依赖：FE-633
- AC：AC-FILE-02-1、AC-FILE-02-6、AC-FILE-02-7、AC-FILE-02-8、AC-FILE-02-9、AC-FILE-02-12、AC-FILE-02-14、AC-FILE-03-1、AC-FILE-03-3、AC-FILE-03-4、AC-FILE-03-7、AC-FILE-03-8、AC-FILE-03-10、AC-FILE-04-10、AC-FILE-05-3、AC-FILE-06-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 气泡（第 13.4 节状态）：按宽高预留占位；懒加载；全屏缩放和保存；过期显示；解码失败占位；`ChatPage.tsx` 第 551 行的"[暂不支持的消息]"保留给未知类型。GIF 播放。
  2. 浏览器下载，无进度；来源提示每会话首次（F-FILE-05 规则 3）。摘要"[图片]""[文件]"。文件名按纯文本，中间省略。
  3. 图片缓存键 `fileId + variant`。
  4. CSP（NF-UI-14）：`connect-src` 只加具体存储域名（构建变量注入，不能放宽成 `https:`），`img-src` 加 `blob:`；`Referrer-Policy: no-referrer`（第 13.2 节）。
- 工作量：3 人日（H5）

### FE-635 Android：选图、处理、发送、沙盒、头像
- 优先级：P0
- 批次：C2
- 仓库：tianting-android
- 依赖：FE-631
- AC：AC-ACC-06-6、AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-4、AC-FILE-02-5、AC-FILE-02-10、AC-FILE-02-13、AC-FILE-03-1、AC-FILE-03-6、AC-FILE-04-11、AC-FILE-07-1、AC-FILE-07-2、AC-FILE-07-4、AC-FILE-07-5、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-9、AC-FILE-07-10
- 内容：
  1. 系统照片选择器（不要存储权限）、SAF。F-FILE-07 规则 2：**发送前复制到 App 沙盒**，发送成功后删除，失败的保留，退出登录清除；磁盘不足提示"存储空间不足"。
  2. 系统图片接口重编码，EXIF 方向后去掉；API 28+ 尝试 HEIC（系统解码能力未核实，实现时实测）。
  3. App 退到后台不保证继续上传（规则 7）。Manifest 通知权限属于 C3。开发环境 http MinIO 用 debug 例外（F-FILE-01 规则 10）。
- 工作量：4 人日（Android）

### FE-636 Android：显示、下载、打开、保存
- 优先级：P0
- 批次：C2
- 仓库：tianting-android
- 依赖：FE-635
- AC：AC-FILE-02-1、AC-FILE-02-6、AC-FILE-02-7、AC-FILE-02-8、AC-FILE-02-9、AC-FILE-02-12、AC-FILE-02-14、AC-FILE-03-1、AC-FILE-03-4、AC-FILE-03-5、AC-FILE-03-8、AC-FILE-03-9、AC-FILE-04-10、AC-FILE-05-3、AC-FILE-06-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 气泡状态；Coil 用 `fileId + variant` 做缓存键；解码前查像素上限；GIF 首帧 + 标记（不加依赖）。`ChatPage.kt` 第 590 行。
  2. **App 内 OkHttp 下载到私有缓存**，有进度、可取消，不用 `DownloadManager`；SAF 交出；pdf、txt、图片白名单才"打开"（F-FILE-03 规则 3）。
  3. 图片保存：权限被拒提示"没有相册权限"和前往设置（API 29 以下未核实，实测）。
- 工作量：4 人日（Android）

### FE-637 iOS：选图、处理、发送、沙盒、头像
- 优先级：P0
- 批次：C2
- 仓库：tianting-ios
- 依赖：FE-632
- AC：AC-ACC-06-6、AC-FILE-01-12、AC-FILE-01-17、AC-FILE-02-1、AC-FILE-02-4、AC-FILE-02-5、AC-FILE-02-10、AC-FILE-02-13、AC-FILE-03-1、AC-FILE-03-6、AC-FILE-04-11、AC-FILE-07-1、AC-FILE-07-2、AC-FILE-07-4、AC-FILE-07-5、AC-FILE-07-6、AC-FILE-07-7、AC-FILE-07-9、AC-FILE-07-10
- 内容：同 FE-635。PHPicker（不要完整相册权限）、`UIDocumentPicker`、ImageIO；HEIC 转 jpeg。`.entitlements` 属于 C3。
- 工作量：4 人日（iOS）

### FE-638 iOS：显示、下载、打开、保存
- 优先级：P0
- 批次：C2
- 仓库：tianting-ios
- 依赖：FE-637
- AC：AC-FILE-02-1、AC-FILE-02-6、AC-FILE-02-7、AC-FILE-02-8、AC-FILE-02-9、AC-FILE-02-12、AC-FILE-02-14、AC-FILE-03-1、AC-FILE-03-4、AC-FILE-03-5、AC-FILE-03-8、AC-FILE-03-9、AC-FILE-04-10、AC-FILE-05-3、AC-FILE-06-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：同 FE-636。`AsyncImage` 换成带 `fileId + variant` 缓存的加载；URLSession 下载；分享面板交出。`ChatPage.swift` 第 557 行。
- 工作量：4 人日（iOS）

### FE-639 管理后台：元数据和正文页配合文件消息
- 优先级：P0
- 批次：C2
- 仓库：tianting-admin
- 依赖：CS-201、CS-202
- AC：AC-ADM-30-7
- 内容：
  1. 元数据页显示类型和大小，**不显示文件名**（第 14.1 节，R-CS-14）。撤回状态随 C4。
  2. 正文页按授权显示文件名；内容经 console-server 代理（CS-202），后台 CSP 不放宽。页面不内联渲染文件。
- 工作量：1 人日（console + 后台页面列）

### FE-640 C2 三端验收和云厂商走查
- 优先级：P0
- 批次：C2
- 仓库：tianting-web、tianting-android、tianting-ios
- 依赖：FE-634、FE-636、FE-638、FE-639
- AC：AC-CHAT-90-8、AC-CHAT-90-9、AC-CHAT-90-15、AC-FILE-02-1、AC-FILE-02-10、AC-FILE-03-1、AC-FILE-90-2
- 内容：
  1. 按 TC-FILE 落地三端用例；AC-FILE-02-1、03-1 三端互发互收，三套 SDK 各留结果。
  2. **AC-FILE-02-10 依赖 P-13 真机**，未到位记未执行。
  3. **AC-FILE-90-2 等云厂商（CH-05）**：合同工具见 BE-439；这里是上线前用生产 CORS、CSP 走查 H5、Android、iOS 的上传下载。`trace.py` 要求手工 AC 有 FE 任务，这是实际内容，不是占位。
  4. AC-CHAT-90-8、90-9：文件页面的状态、图标、深色、无障碍（图片替代文本"图片"，文件读文件名和大小；新消息播报）。
- 工作量：4 人日（测试）

---

## 实现时发现的问题

PRD 冻结，没有改，都按最保守的理解实现，等 Frank 决定。第 1 条是协议文件和服务端不一致，SDK 跟服务端走。S1、S3、K5、K7 已处理，三套 SDK 的提交是 tianting-sdk-ts@20be2f0、tianting-sdk-kotlin@4feed19、tianting-sdk-swift@5f3f614，结果是本地的，CI 不运行。

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 1 | `http-client.schema.json` 的 `endpoints` | `POST /v1/account/recovery-code` 的 `response` 还写着 `AuthResponse`，服务端实际返回 `RecoveryCodeResponse`（backend-chat 问题 8 补了定义，没有改这一行） | SDK 按 `{recoveryCode, recoveryCodeGen}`，真实服务端跑通。没有改 protocol/，建议 BE 改这一行 |
| 2 | F-ACC-03 规则 7 | 只说"比较代号"。新 `sid` 的 `gen` 从 1 开始（`service.go`），改密后旧连接的 4011 只比 `gen` 会判成"本地不比它新"，多发一次刷新 | `sid` 不同算本地更新，相同才比 `gen`。`account-session.json` 没有这个用例，建议 BE-402 补 |
| 3 | F-ACC-03 规则 6 | 429、网络错误、5xx "退避重试"没有给参数 | 和重连相同：1、2、4…30 秒，±20% |
| 4 | F-ACC-03 规则 5、6 | 等退避或等 `retryAfterSeconds` 时回前台、收到 `kicked` 怎么办没写；`model.go` 没有等待状态，向量里的 `trigger timer` 就是到点 | 只有到点的定时器能发请求，其他入口不提前 |
| 5 | F-ACC-03 规则 6 | 4xx 但响应不是约定的 JSON（代理、强制门户的页面）没写；`model.go` 对没列出的码回登录页 | 没有 `code` 的响应按网络错误处理，不回登录页。有 `code` 且不认识的按 `model.go` 回登录页 |
| 6 | F-ACC-04 规则 3 | 只写了改密缓存踢线。规则 6 的退出其他设备"效果同规则 3"，找回和注销申请服务端也是先踢后回 | 这几个都缓存；退出登录也缓存（规则 1 要求先 logout 再 1000）。封禁等其他原因不缓存 |
| 7 | F-ACC-03 规则 5 | 串行队列只列了改密、找回、退出其他设备和刷新 | 登录、注册、注销申请也进队列。重新生成恢复码、确认、改资料不换 token，不进队列 |
| 8 | F-ACC-05 规则 1 | 响应带恢复码时存储里的旧 token 怎么办没写。留着的话，改密后在保存页杀 App，重启会用已吊销的旧值刷新，弹"凭证有变化"，和 AC-ACC-05-19"回登录页"不一致 | 立即清掉 |
| 9 | F-ACC-05 规则 1 | 恢复码未确认时退出其他设备，响应没有恢复码，新 token 是否落盘没写 | 继续只放内存 |
| 10 | AC-ACC-05-15 | 回登录页时这条服务端会话还有效，PRD 没要求退出 | 只清本地，不调 logout，会话到期后由 `session_purge` 清除 |
| 11 | F-ACC-03 规则 5 | 过期判断用哪个钟没写。`serverTime` 只在 `auth` ack 才有，启动判断时没有 | 用本地时钟。偏快：提前刷新；刷新回来仍然不足 6 小时的 token 不再主动刷新，也不按本地时间当成过期，见 S1。偏慢：服务端回 4011，走规则 7 |
| 12 | FE-600 第 2 条 | "`kickReasons` 忽略不认识的值"和现有用例（`moderation.test.ts`：不认识的 reason + 4012 时 `fatal` 带原 reason）在字面上冲突 | 关闭码可以重连就忽略这个值；否则按关闭码结束，`fatal` 带原值，账号模式按关闭码清 token。没有改现有用例 |
| 13 | AC-ACC-03-42 | `/v1/files/urls` 和 `getFileUrls` 随 C2，C1 测不了原文 | 用 `listEvents` 验证同样的语义，FE-630 要用原接口再跑一次 |
| K1 | F-ACC-04 规则 1、F-ACC-05 规则 1（FE-611） | 恢复码没确认时断网退出：确认前 token 只放内存（RS5-13），"待退出"又要持久化 refresh，两条冲突 | 退出请求成功时不写盘。断网才写"待退出"：这时已经退出，不算登录状态，不写的话这条会话只能等到期 |
| K2 | F-ACC-04 规则 1（FE-611） | "本地立即退出"和"先调 logout 再断开"：网络挂起（不是立刻失败）时 logout 要等 30 秒 | SDK 保持先发请求，最长等 30 秒；待退出先写、登录态先清，所以等待期间杀 App 也不会回到登录状态。界面不等 `logout()` 的返回，先回登录页（FE-616 要照做） |
| K3 | F-ACC-04 规则 1（FE-611） | "待退出"只说存 `sid` 和 refresh，"下次联网时补发"：一台设备有几条、补发失败怎么办、服务端明确拒绝怎么办都没写 | 列表，最多 8 条；启动、回前台、联网时补发，失败按退避重试 5 次后等下一次事件；网络错误、4290、5xx 留着，其他错误码（如 refresh 的 mac 不对）删除 |
| K4 | 第 21 节 16（OkHttp，FE-611） | 已核实：默认配置下，复用的连接断在半路时 POST 会被重发；**只关 `retryOnConnectionFailure` 不够**，`503 + Retry-After: 0` 还是会重发 | 关 `retryOnConnectionFailure` 和重定向，请求体标记 one-shot，`OkHttpRetryTest` 用裸 socket 数到达的请求数（含默认配置的对照组）。网关的隐式重试还没有核实 |
| K5 | F-ACC-04 规则 3（FE-611，sdk-swift 也要看） | sdk-ts 只有一条事件循环，"响应先到踢线后到"时旧连接的回调已失效。Kotlin 的连接和账号在两条线程上，踢线可能在缓存已经释放之后才到账号这边，会把刚换好的会话结束掉。第一次跑真实服务端就遇到了 | **已处理**。sdk-kotlin：`KickInfo` 带这条连接的 `sid`，和本地 `sid` 不同就忽略这个踢线，用新 token 重连（实现在 9495872，4feed19 加了经过连接的用例）。sdk-ts：**也有**，窗口是一个微任务：拨号读到旧 token，回调运行之前换了 `sid`，这时没有连接可断，随后用旧 token 连上，踢线结束新会话。逐拍错开复现后同样修（20be2f0）。sdk-swift：**没有**，依据：`Connection` 和 `AccountClient` 都在主 actor 上，`adopt` 同步调用"token 换了"，旧连接的回调在同一步失效；拨号读 token 到开连接之间没有挂起点。`testK5_dialAroundTheSidChangeNeverEndsTheSession` 逐拍验证（5f3f614），没有改实现，`KickInfo` 不带 `sid` |
| K6 | FE-611 第 4 条 | 任务单写 `IMClient.kt` 第 916～932 行是重连和 4011 分支，实际这几行是 `recheckDeleted`，重连和 4011 在 `Connection.kt` 的 `onClosed` | 改的是 `Connection.kt`，任务单没有改 |
| K7 | F-ACC-03 规则 5（SDK 接口） | 会话已经结束时调 `connect(tokenProvider)`（比如 `start()` 的刷新收到 4019 之后 App 才连接）：`IMClient` 还没绑定，收不到结束事件，状态停在连接中。sdk-ts 相同 | **已处理**（ts@20be2f0、kotlin@4feed19、swift@5f3f614，三套都复现后修）。发 session 类 `fatal`，状态回到断开，不开连接。`SessionEndedError` 加了可选的 `end`，`fatal` 带最近一次结束的原因，没有 `origin`；没登录过时只有 `action: login`。App 仍然可以先看 `session`，不看也不会卡住 |
| S1 | F-ACC-03 规则 5（FE-612，sdk-ts 和 sdk-kotlin 要看） | 刷新返回的 token 按本地时钟已经不足 6 小时（时钟偏快超过 18 小时，或 TTL 配得短）时，每次拨号的 `start` 和连接后的定时器都会再刷新一次，换 token 又重连，循环到服务端限流。问题 11 写的"不会循环"只对已过期的那条路径成立。sdk-ts 的一份副本上自动应答假 HTTP 复现了，循环到内存耗尽；真实网络上受延迟和每小时 20 次的限流约束。sdk-kotlin 的代码里没看到对应处理，没有实测 | **已处理**，统一为 sdk-swift 的做法：这种 token 不再主动刷新，也不按本地时间当成过期，等服务端的 4011（规则 7）；下一个时间正常的 token 恢复。向量没有这个用例。sdk-ts（20be2f0）、sdk-kotlin（4feed19）都用自动应答的假 HTTP 复现了（到测试设的 50 次应答上限，共 51 次请求）；sdk-swift 的实现没有改，补了同一组用例（5f3f614）。三套现在每个 4011 最多一次刷新。两个旧用例（`成功后清零`、`连续 3 轮刷新并重连`）用的是刷新回来就不足 6 小时的 token，依赖旧行为，改成正常 token 加推进时钟 |
| S2 | 第 21 节 16（URLSession，FE-612） | 已核实：带 `WWW-Authenticate` 的 401 会让 URLSession 再发一次 POST，默认处理、拒绝保护空间、空凭证都一样。刷新的 4019 和改密的 4018 都是 401。im-server 不发这个头，网关可能加。POST 在连接断开、超时、取消、`503`/`429` 带 `Retry-After: 0`、`408`、`421` 时都只到达 1 次；同样场景下 GET 会重发 | HTTP 认证质询直接取消请求，按网络错误处理，只发 1 次。代价是带这个头时 4019 读不到，会一直退避，所以**部署时网关不能给 `/v1/` 加这个头**。`URLSessionRetryTests` 用真实 socket 数到达的请求（macOS、回环、HTTP/1.1） |
| S3 | F-ACC-03 规则 5、F-ACC-05 规则 1（FE-612，sdk-ts 可能要补） | sdk-ts 的 `applyAuth` 落盘之后不再看会话是否已结束，`ackRecoveryCode` 先清只放内存的标记再写存储。存储回调可以挂起时，前者会把结束的会话救回来，后者可能写入旧 token 或者写失败后不再重试 | **已处理**，统一为 sdk-swift 的做法：落盘后比 `epoch`，结束了就清存储并报错；确认时写到"存的就是在用的"才清标记，写失败抛给调用方，再调一次能重写。存储调用按顺序执行，`clear` 不会被之前的 `save` 超过。sdk-ts（20be2f0）：两个窗口和顺序都复现后修，`logout` 开始时 `epoch` 加一（之前没有）。sdk-kotlin（4feed19）：核对后第一个窗口和顺序本来就对（`epoch`、`storeLock`），第二个窗口有：写失败后标记已经清了，再确认不写，复现后修 |
| S4 | F-ACC-05 规则 1、RS5-13（三套 SDK） | `ack(gen)` 进行中，改密、找回或重新生成换了新一代（未确认），之后旧一代的 ack 才成功返回：SDK 不看 `gen`，把新 token 写盘并清掉只放内存的标记，这时杀 App 不会回登录页，而新恢复码没有确认过。三套都复现了，每套 4 个用例：改密、重新生成、找回交错，以及 ack 正在写存储时改密 | **已处理**（ts@ba226d7、kotlin@114fc54、swift@54e4f04）。只放内存时记下等确认的代号，`gen` 和它相同才写盘、清标记，写完再比一次；不同就直接返回，没有错误（服务端已经确认了旧一代）。退出其他设备不换码，代号留着。重新生成的响应可能比之后的改密晚到，代号只进不退。用例：ts `account-unified.test.ts` 的"恢复码确认和换代交错"，kotlin `AccountUnifiedTest` 的"恢复码 …"，swift `AccountUnifiedTests.testAck_*`。公开接口没有变化 |
| A1 | console-chat 问题 3（FE-619） | 生成时 5040 只规定了提示。网络断开和网关 502 同样收不到响应，PRD 没有写。页面记的"结果未知"也可能被刷新清掉 | 5040、网络错误、502（不是 JSON）都按结果未知：关掉表单，锁住"生成"，提示到列表停用最新几条，操作人点"已核对并停用，重新生成"才解锁。**锁只在内存里，刷新后解除**：后台不用本地存储（README 约定）。4131、4000、5031、5000 是确定没有生成，留在框里可以再提交 |
| A2 | F-ACC-09 规则 1（`createdBy`，FE-619） | 列表接口只有 `console:admin:{账号ID}`，没有用户名；运营没有 `admin.manage`，不能查账号列表。推送页靠服务端给的 `createdByName` | 显示"管理员 {账号ID}"，其他形式显示"—"。账号 ID 是 UUID，一格占三行，运营也看不出是谁。建议 console-server 像推送一样加 `createdByName`，没有改 |
| A3 | PRD-admin F-ADM-10 规则 1（FE-619） | 用户列表和详情一直把顶层 `createdAt` 叫"注册时间"，实际是资料时间，演示用户和导入用户没有注册这回事 | 详情页改叫"资料创建时间"，账号区的"注册时间"用 `account.createdAt`。**列表的列名没有改**：PRD-admin 写的是"注册时间"，列表也没有账号数据。要不要改由 Frank 决定 |
| A4 | F-ACC-09 规则 1（到期时间，FE-619） | 没有写时区和状态按哪个钟算。后台显示固定北京时间，`datetime-local` 按本机时区，本机不是东八区时两边不同（走查时本机 UTC+7，相差 1 小时） | 输入框下面写明"列表里显示为 …（北京时间）"。状态按浏览器时钟算，边界和 im-server 一样（`expiresAt` 等于现在算过期）；浏览器时钟不准时显示可能和实际不同，注册结果以服务端为准 |
| A5 | 错误码表（FE-400）、CS-200 | 4131 在表里是"单个文件最多 5000 行"（用户导入），邀请码透传的 4131 没有 `reason`；4000 的通用文案说不清哪里错 | 生成框单独出文案（`inviteErrorText`），其他码走表。建议 console-server 给邀请码的 4131 加 `reason`，没有改 |
| A6 | F-ACC-09 规则 3（FE-619） | "可以停用未用完的邀请码"：已过期但没用完的算不算没有写；停用后列表刷新只回第一页 | 只有"可用"状态有停用按钮，过期的已经不能注册。接口仍然接受。翻到后面几页停用后列表回到第一页，没有保持位置 |
| H1 | 第 21 节 16、CH-26（FE-614，Frank 已决定） | **实测和 CH-26"关闭标签页后要重新登录"不完全相符**（Chrome，headless，本地）。① 关闭标签页后用恢复已关闭的标签页（`chrome.sessions.restore`，等于 Ctrl+Shift+T）：`sessionStorage` 恢复，token 还在，**不用重新登录**。② access 和 refresh 明文写进 profile 的 `Default/Session Storage/000003.log`（登录后等 40 秒查到），关闭、杀进程后文件还在，直到 Chrome 自己压缩。③ 崩溃后的会话恢复：headless 下标签页没有恢复，**未验证**，headed 没跑（会在 Frank 桌面开窗口） | 没有改，H5 只放 `sessionStorage`。web 的 refresh 24 小时到期（`IM_REFRESH_TTL_WEB`），影响到这个期限为止。要严格满足 CH-26 需要 Frank 决定（例如改成只放内存，代价是刷新页面就要重新登录）。**Frank 决定（2026-11-09，都按建议）**：接受现状：Chrome 恢复标签页是浏览器行为，已写进 PRD-chat v1.2 F-ACC-03 规则 8 |
| H2 | 第 21 节 16（navigator.locks，FE-614） | 已核实（Chrome，本地）：被 CDP 冻结的页面仍然占着锁，新页面 `ifAvailable` 拿不到；真实"复制标签页"（扩展调 `chrome.tabs.duplicate`）运行中和冻结时都复制 `sessionStorage`，新页面回登录页，原页面不受影响。**刷新页面时旧文档的锁可能还没放掉**，只试一次的话刷新可能被当成第二个标签页 | 拿不到时重试 6 次，间隔 100 毫秒，所以被复制的页面最多 500 毫秒后才回登录页。WebKit 未执行（未安装） |
| H3 | F-ACC-03 规则 9 | "拿不到锁"只说回登录页，没有提示文案；"刷新前再确认"失败怎么办没写 | 登录页显示"这个账号已在其他标签页打开，请重新登录"（文案是实现时加的）。确认失败时不发刷新、不调 logout（否则把另一个页面吊销），本地按"登录已失效"结束 |
| H4 | F-ACC-03 规则 9 | `navigator.locks` 和 `BroadcastChannel` 都没有的浏览器没写；`BroadcastChannel` 退路在对方页面被冻结时防不住（对方不回答） | 都没有时不拦。退路的局限如实保留，刷新前再问一次能发现大部分 |
| H5 | F-ACC-10 规则 2（server，BE） | "确认后记录"：`accounts.notice_version` 只在注册时写，**没有接口能更新**，登录响应也不带这个账号同意过的版本。TC-ACC-10-03 的"确认后 `notice_version=v2`"做不到 | 记在本机 `localStorage` 的 `ui.privacyAck.{用户名哈希}`，只存版本号。换浏览器或清数据会再提示一次（偏保守）。服务端字段没有更新，建议 BE 补接口。只记录，**协调方跟进** |
| H6 | F-ACC-10 规则 1、2 | "打包页面"没有实体（文字等 Frank 或法务）；只有版本没有地址时无处可开。其他域名的说明页探测不了（CSP `connect-src` 只有 `self`），"暂时打不开"只对同源地址有效 | 没有地址就隐藏入口。其他域名直接打开，失败时由浏览器显示。只接受 http、https |
| H7 | sdk-ts `SECURITY_NOTICE_KINDS`（FE-610，F-ACC-05 规则 5） | 服务端还会发 `recoveryCodeRegenerated`、`refreshReplay`、`refreshRateLimited`（`account/credentials.go`、`refresh.go`），SDK 白名单只有 `newDeviceLogin`，其余三种被丢弃，**界面收不到"重新生成恢复码"等提醒**。依据是读代码（`client.ts` 第 159、588 行），没有在真实服务端上跑 | 只记录，**协调方跟进**。没有改 SDK。H5 只提示 `newDeviceLogin`。PRD 只给了这一种的文案，另外三种的文案也要定 **已处理**：放行服务端实际发的四种，tianting-sdk-ts@d0b0b07，真实服务端验证了 `recoveryCodeRegenerated`。H5 按 kind 提示，tianting-web@63c9d0f。三个新文案见 A12 |
| H8 | tianting-sdk-ts 的 `dist/`（FE-613 接入时） | `@tianting/sdk` 是 `file:` 依赖，用的是 git 忽略的 `dist/`。开工时 `dist/` 还是 19:24 的旧构建，没有 S1、S3、S4、K5、K7。用它跑出刷新循环（21 次刷新到限流），看起来像 S1 没修，其实是 `dist/` 过期 | 在 `tianting-sdk-ts` 跑了 `npm run build`（只动被忽略的 `dist/`，HEAD ba226d7 没变）。**其他端和 CI 也要先构建再测** |
| H9 | F-ACC-08 规则 2 | 注销申请成功后回登录页的提示没写 | 弹窗"已申请注销账号。7 天内可以用密码登录并撤销"，文案是实现时加的 |
| H10 | F-ACC-05 规则 1 | 重新生成恢复码（要密码）的确认框文字没写；撤销注销成功后要不要自动登录没写 | 提示"生成新的恢复码后，旧恢复码 7 天后失效"；撤销后提示"已撤销注销，请重新登录"，不自动登录，密码框清空 |
| H11 | sdk-ts `IMClient.send`（FE-001 就有，FE-613 接入时发现） | 第一次连接成功之前 `send` 同步抛 `NotConnectedError`（还不知道自己是谁）。`ChatPage` 没有接住：单聊页按回车没有任何提示，浏览器里是未捕获的错误。群聊页连上之前要探测成员，没有输入框 | **已处理**（tianting-web@3daa769）。SDK 没有改。输入框内容保留，提示"正在连接，请稍后再发送"（文案是实现时加的），连上后再按回车能发出去。单测和 Playwright 各一个，去掉处理后单测失败 |
| H12 | `internal/ratelimit`（测试时发现） | 限流桶只在 `elapsed > 0` 时补充。IM 钟调快再调回后（`/admin/test/clock`），桶记着"未来"的时间，真实时间追上之前不再补充：每 IP 握手的 50 个令牌用完后所有连接 429。完整 e2e 因此失败 54 个（服务端日志 `handshake rate limited`）。生产上时钟回拨可能有同样的表现，没有验证 | **服务端已修**（回拨不补令牌，`last` 跟到当前时间，`TestClockRollbackDoesNotStickBucket`）。e2e 把调钟的用例放到 `e2e/zz-im-clock.spec.ts`，靠文件名和 `workers: 1` 最后跑，之后 `rate limited` 为 0 次 |
| A1 | F-ACC-04 规则 1（sdk-kotlin，**SDK 问题**，FE-615 发现） | `logout()` 等响应期间 `login()` 成功，新会话已经落盘；logout 返回时无条件 `endSession(LOGOUT)`，把新会话结束并清存储。`applyAuth` 记的 `epoch` 已经是 logout 加一之后的值，拦不住。复现（真实 `AccountClient` + 假 HTTP，模拟器上跑过）：登录 → logout 的 HTTP 不回 → 再 `login` 成功（`session` 不为空）→ 放行 logout → `session` 为空，`ended.action=LOGOUT`，存储为空。网络挂起时 logout 最长等 30 秒，窗口可以到达。sdk-ts、sdk-swift 没有核实 | App 绕开：登录、注册、找回先等上一次退出结束（`AppModel.authenticate`，用例 `login_waits_for_logout_in_flight`），代价是网络挂起时登录按钮最多多转 30 秒。SDK 没有改。建议 logout 结束时只结束它开始时的那条会话 **已处理**：三套都复现（ts、swift 也写了失败用例）。logout 只收尾它开始时的会话，tianting-sdk-ts@d0b0b07、tianting-sdk-kotlin@d627104、tianting-sdk-swift@f9a6a14；App 去掉等待，tianting-android@c6eb435、tianting-ios@6a071a6，用例 `login_works_at_once_while_logout_is_in_flight`、`testA1_loginWorksAtOnceWhileALogoutIsInFlight`。H5 本来不等（退出换 `AccountClient` 实例）。结果是本地的，CI 不运行 |
| A2 | F-ACC-10 规则 2（同 H5） | 同 H5：服务端没有接口记录确认的版本 | 确认记在本机 `ui.privacyVersion`，一台设备一个值，没有带用户（`UiPrefs` 只有 `ui.noticeSeen` 一个按账号的键）。后果：换设备登录再提示一次；同一台设备换账号不再提示。和 H5 按账号记不同，等服务端有接口后都要改 **已处理**：改成按账号记 `ui.privacyAck.{用户名哈希}`，和 H5 同名，tianting-android@c6eb435；旧键启动时删除，升级后每个账号再提示一次。iOS 本来按账号，但键里是明文用户 ID、退出后还在，也改成同样的键，tianting-ios@6a071a6。服务端没有接口这一点没变 |
| A3 | F-ACC-10 规则 1（同 H6） | 只有版本没有地址 | 同 H6：隐藏入口。有地址时用系统 WebView 打开，不开 JavaScript，主框架加载失败或 HTTP 错误提示"暂时打不开，请稍后重试" |
| A4 | F-ACC-05 规则 1、AC-ACC-05-19 | 强制进保存页要输入密码。密码也忘了就走不出去，PRD 没写 | 这一步（只有这一步）有"退出登录"，退出后走"忘记密码"。有恢复码的保存页仍然只有"我已保存"。**Frank 决定（2026-11-09，都按建议）**：同 I2，保留 |
| A5 | AC-UI-90-2 | 系统 WebView 自己写 `WebViewChromiumPrefs`、`AwOriginVisitLoggerPrefs`，不是 `ui.` 键。这条 AC 写的时候 App 没有 WebView | `shared_prefs_only_have_ui_keys` 排除这两类系统文件，App 自己写的仍然只有 `ui.`。文件内容 App 控制不了，建议 AC 补一句 |
| A6 | F-ACC-07 规则 4、异常 | "已登录用缓存"和"失败不猜测"，对成功过、回到登录页时又失败的情况没有分开说。断网退出后回到的登录页就是这种情况 | 登录页不用缓存，只显示"网络异常"和重试，回前台自动再拉一次；已登录时用缓存。代价：断网退出后看不到表单，本来也登录不了 |
| A7 | K2（FE-616） | 等 logout 响应时被杀：token 清了，本地库要等响应回来才清（先 logout 再断开），缓存会留到下次登录 | 退出时写标记 `ui.wipePending`，下次启动没有会话时补清（`logoutInFlight_phase2`，真实杀进程） |
| A8 | AC-UI-16-6、F-UI-16 规则 1 | AC 写"黑名单之后增加账号安全"，规则 1 的表顺序是隐私、黑名单、外观、账号安全 | 按表：外观之后 |
| A9 | F-ACC-08 规则 2、F-ACC-05 规则 1（同 H9、H10） | 文案和撤销后要不要自动登录没写 | 和 H5 一致：注销后"已申请注销账号。7 天内可以用密码登录并撤销"；撤销后提示"已撤销注销，请重新登录"，不自动登录，密码框清空；重新生成的说明"生成新的恢复码后，旧恢复码 7 天后失效" |
| A10 | AC-CHAT-90-9 | 原生用 Espresso AccessibilityChecks，要新增依赖 `espresso-accessibility`，这批不新增依赖 | 用语义树检查能点的节点都有名字（`tc_chat_90_08_new_pages_have_named_buttons`），不等于 AccessibilityChecks，**没有运行它**。输入框的名字是上方的文字，不是控件自己的标签，沿用现有 `TtInput` |
| A11 | sdk-kotlin `SECURITY_NOTICE_KINDS`（同 H7） | 白名单只有 `newDeviceLogin`，其他 kind 不发给 App（读代码，没有在真实服务端上跑其他 kind） | 界面只提示新设备登录，其他事件在"账号安全"的记录里能看到 **已处理**：同 H7，tianting-sdk-kotlin@d627104、tianting-android@c6eb435 |
| I1 | F-ACC-10 规则 2（同 H5、A2） | 同 H5：服务端没有接口记录确认的版本 | 确认记在本机 `ui.privacyAck.{用户名哈希}`（原来是明文的 `ui.noticeVersion.{userId}`，见 A2），按用户分开。换一台设备会再提示一次 |
| I2 | F-ACC-05 规则 1、AC-ACC-05-19（同 A4，Frank 已决定） | 强制进保存页要输入密码。密码也忘了，token 又已落盘，杀 App 也回到这里，走不出去，PRD 没写 | 只有"没有恢复码、要输密码"这一步有"退出登录"，退出后走"忘记密码"。有恢复码的那一步仍然只有"我已保存"（AC-ACC-01-12）。这是 PRD 之外的补充，要不要留由 Frank 定。**Frank 决定（2026-11-09，都按建议）**：保留"退出登录"出口，已写进 PRD-chat v1.2 F-ACC-05（AC-ACC-05-19 前的补充） |
| I3 | AC-ACC-03-8（Frank 已决定） | AC 写回登录页提示"登录已失效，请重新登录"。真实服务端上，access 未过期时被强制下线，`auth` 走 `kicked`，文案是"账号已被下线"；只有 access 已过期、走到刷新 4019 才是 AC 的文案 | 没有改 SDK 和 AC，两种都按实际。假服务端用例验证 4019 的文案，真实服务端用例按 `kicked` 断言。要统一文案还是改 AC 由 Frank 定。**Frank 决定（2026-11-09，都按建议）**：改 AC：两种文案都认（PRD-chat v1.2 AC-ACC-03-8，TC-ACC-03-08）。iOS 的 03-08 新用例按实际路径断言。**处理结果（2026-11-09，tianting-ios@54caa87）**：iOS 真实服务端 TC-ACC-03-08 经测试代理记下实际路径再断言对应文案；本次路径是先连上、服务端 `kicked/banned`，文案"账号已被下线"。验收报告第 10 节，AC-ACC-03-8 改为通过 |
| I4 | F-ACC-10 规则 1（同 H6、A3） | 只有版本没有地址 | 同 H6：隐藏入口。有地址时用 `WKWebView` 打开，不开 JavaScript，HTTP 错误或加载失败提示"暂时打不开，请稍后重试"，只接受 http(s) |
| I5 | F-ACC-07 规则 4、异常（同 A6） | "已登录用缓存"和"失败不猜测"，对成功过、回到登录页时又失败的情况没有分开说 | 未登录一律不用缓存，显示"网络异常，请稍后重试"和重试。所以断网退出后看到的是重试页，不是登录表单。缓存 `ui.config` 只给已登录的用户 |
| I6 | F-ACC-04 规则 1（sdk-swift，同 A1 的现象） | `logout()` 无条件 `endSession`，等响应期间 `login()` 成功的话，新会话会被一起结束（读代码，没有单独复现）。sdk-kotlin 的 A1 是同一类问题 | App 绕开：登录、注册、找回、演示登录都先等进行中的退出结束（`AppModel.logoutFinished`），网络挂起时最长等 30 秒，按钮显示"提交中"。sdk-swift 没有改，是否在 SDK 修由 SDK 任务决定 **已处理**：同 A1，tianting-sdk-swift@f9a6a14、tianting-ios@6a071a6 |
| I7 | K2（FE-618，同 A7） | 等 logout 响应时被杀：token 清了，本地库要等响应回来才清，缓存会留在磁盘上 | 冷启动没有 session 时删除本地库目录，不需要标记。推送游标在另一个目录，不受影响 |
| I8 | F-ACC-01 规则 5、AC-ACC-01-12（iOS，**产品问题，已修**） | 注册页只把密码的第一个字符发给服务端，真实服务端回 `passwordWeak`。根因：iOS 按整张表单判断这是注册，用 Automatic Strong Password 覆盖层盖住密码框（截图上是黄色的 "Automatic Strong Password cover view text"），之后的按键都丢失。`.newPassword`、不设类型、`.password` 都一样。逐键点屏幕键盘同样丢，所以不是 `typeText` 的问题；焦点没有离开，也不是 SwiftUI 重建。找回页和登录页不触发，判断依据在系统里，改标题、ID、去掉字段都不改变结果 | 新密码框（注册、找回、改密）用 `.textContentType(.oneTimeCode)`，这是 UIKit 关掉这个覆盖层的办法，代价是这些框不提供系统的强密码建议。`testTC_ACC_01_12_passwordTypedKeyByKeyArrivesWhole` 逐键输入并断言发出的整段文本，去掉修复后这个用例失败。真机上开了 iCloud 钥匙串时的表现没有验证（P-13） |
| I9 | sdk-swift `securityNoticeKinds`（同 H7、A11） | 白名单只有 `newDeviceLogin`，其他 kind 不发给 App（读代码，没有在真实服务端上跑其他 kind） | 界面只提示"你的账号在新设备登录"，其他事件在"最近登录记录"里能看到 **已处理**：同 H7，tianting-sdk-swift@f9a6a14、tianting-ios@6a071a6 |
| A12 | securityNotice 的文案（H7、A11、I9） | PRD 只给了 `newDeviceLogin` 的文案（AC-ACC-02-13） | **实现时加的文案，待确认**，三端相同：`recoveryCodeRegenerated` "你的恢复码已重新生成"；`refreshReplay` "检测到你的账号登录凭证异常"；`refreshRateLimited` "你的账号登录凭证刷新过于频繁"。不认识的 kind 不提示 这次新增，同样待确认，三端相同：`newIpPrefix` "你的账号正在新的网络使用"；`passwordChanged` "你的账号密码已被修改"；`passwordRecovered` "你的账号已通过恢复码重设密码"。**Frank 决定（2026-11-09，都按建议）**：按实现的文案确认，已写进 PRD-chat v1.2 F-ACC-02 规则 3b |
| A13 | F-ACC-02 规则 3b，F-ACC-05 规则 5（server） | PRD 要求来自新 IP 前缀的刷新或转正也提醒其他在线设备，服务端只写 `newIpPrefix` 事件，没有 `notify`（`refresh.go`）。规则 3b 也列了改密、找回，第 467 行也写了改密时收到 `securityNotice`，服务端只发 `kicked` | **已处理**：im-mvp@3ab750c。刷新和转正提交之后发 `newIpPrefix`；改密、找回发 `passwordChanged`、`passwordRecovered`，排在 `kicked` 之前。PRD 写得明确，所以补了，`kicked` 不代替它。提醒和踢线原来各走各的协程，踢线会抢先关连接，所以网关改成等已入队的帧写完再踢，新帧不再入队。`logout-others` 不在规则 3b 的列表里，只有 `kicked`。SDK 放行：tianting-sdk-ts@c583243、tianting-sdk-kotlin@278f8c4、tianting-sdk-swift@985b68c；文案：tianting-web@6b8632d、tianting-android@1c9ed56、tianting-ios@1e46ef7，见 A12。**频率待 Frank 决定**：PRD 和安全复查（RS4-08）都没有规定频率，F-ACC-03 规则 4 只写了弱网切换也会触发，所以只提醒不吊销。按要求没有自造限频：每条会话每个没见过的前缀提醒一次（`seen_ip_prefixes` 记 64 个，超出丢最旧的）。CGNAT 或移动网络下前缀常变的用户，其他设备可能频繁收到提醒，没有实测过频次 |
| A14 | F-ACC-05 规则 5（server） | `notify` 用 `DeliverToUser(userID, b, "")`，没有排除发起方。实测确认：重新生成恢复码的设备自己在线时会收到提醒，限流时这条会话的连接也会收到。PRD 写的是"其他设备" | **已处理**：im-mvp@3ab750c。发起方是 HTTP 请求，没有连接 ID，所以新增 `DeliverToUserExceptSID`，按 `sid` 排除，同一 `sid` 的其他连接也不提醒（多标签页已互斥）。`newDeviceLogin`、`passwordRecovered` 的发起方这时还没有连接，排除的是新 `sid`，没有实际效果 |
| A15 | A1 的修复，F-ACC-04 规则 1 | 旧 logout 失败时本来要撤销系统推送令牌，但新会话已经在用这个令牌 | 有新会话时不撤销，改成立刻补发旧 sid。补发成功之前，旧 sid 的推送还可能到这台设备（系统令牌接入随 C3，真实推送没有验证） |
| F2 | F-ACC-03 规则 5，AC-ACC-03-43（server，FE-620 发现） | 到绝对上限前不足 1 秒，服务端已经踢 `tokenExpired`（JWT 的 `exp` 只到秒），刷新还能返回 200，于是有几次多余的刷新和重连，随后到点回登录页。`IM_REFRESH_ABSOLUTE=25s` 实测 2 次 | 没有改：`model.go` 规定 `kicked` 可以刷新，服务端要不要在上限的同一秒不再签发由 Frank 决定。三端用例只断言"上限前没有刷新"。**Frank 决定（2026-11-09，都按建议）**：修服务端：距上限不足 1 秒的刷新按到期处理（PRD-chat v1.2 `IM_REFRESH_ABSOLUTE`，TC-ACC-03-51）；三端 03-45 用例改回"到期后也不刷新"。**处理结果（2026-11-09，tianting-ios@54caa87、tianting-android@d50c230、tianting-web@f0c5b6f）**：三端改回检查到期之后：上限前 0 次刷新，到期后没有成功的刷新、不重连。三端实测服务端在上限那一秒踢 `tokenExpired` 后 SDK 仍发 1 次刷新，服务端回 401 / 4019，用例断言"最多这 1 次且被拒绝"，不是"请求数为 0"。要 0 次需要改三套 SDK，待 Frank 决定（验收报告第 10 节），AC-ACC-03-43 仍是部分通过 |
| A16 | tianting-android `ChatTest`（FE-620 发现，不是 C1 范围） | 整类连跑 7 次失败 2 次：`menu_copy_retry_delete` 删除后 5 秒内消息还在；`at_bottom_follows_new_message` 在聊天页消息列表的 Compose `LazyColumn` 测量时 `IndexOutOfBoundsException`（区间列表为空却取第 0 项）。单独跑都通过 | 根因没有查明，没有改。后一个是 App 进程里的崩溃，真实用户可能遇到，建议单独排查。**Frank 决定（2026-11-09，都按建议）**：C2 开工前先查清并修复。**处理结果（2026-11-09，tianting-android@d50c230）**：崩溃的根因查明：聊天页在组合的上下文里收集 SDK 线程更新的 `messagesFlow`，Compose 测试的调度器不切线程，重组和 LazyColumn 测量跑在 `tianting-sdk` 线程上，和主线程并发（现场栈：`CalledFromWrongThreadException ... Calling: tianting-sdk`，随后 "performMeasureAndLayout called during measure layout"）。改成在主线程收集，聊天页的 SDK 挂起调用经 `onMain` 回主线程；新用例 `sdk_thread_updates_reach_ui_on_main_thread` 修前失败（34～35 次通知在 SDK 线程）、修后通过。正式运行的调度器会切回主线程，按代码判断真实用户碰不到。`menu_copy_retry_delete` 按代码判断是等待条件（重发后旧的失败图标先满足等待，删除时消息其实在发送中），改成等 SDK 里真的又失败；这一轮没有复现，根因未经现场证实。另外修了软键盘动画中点"发送"落空的等待条件。ChatTest 整类连跑 10 次全过（模拟器，本地） |
| A17 | tianting-android `ConvListTest.start_chat_self_and_friends_mode_warning`（FE-620 发现） | 约 1/3 失败：已经跳转到聊天页（路径对、对方是好友），5 秒内没有 `chat-title` | 根因没有查明。试过重复点击、先等好友关系，都没用，已撤销。**处理结果（2026-11-09，tianting-android@d50c230）**：和 A16 同一原因：发起单聊弹窗里 `relationOf` 在 SDK 线程上恢复后直接导航。改成经 `onMain`。ConvListTest 连跑 6 次全过（之前约 1/3 失败）；6 次不足以证明，只能说没有再出现 |
| A18 | tianting-android `ServerE2eTest.live_notice_banner_list_and_no_token_or_data_in_logcat`（tianting-android@8757053） | 用例读 `logcat -d -b all` 时测试进程 OutOfMemoryError（模拟器开得久，缓冲区大） | 开头先 `logcat -b all -c`。**这条用例现在只检查它自己运行期间的日志**，之前其他用例写进 Logcat 的 token 不再被它顺带发现；`AccountE2eTest.secrets_never_reach_logcat` 同样只看自己清空之后的日志。**Frank 决定（2026-11-09，都按建议）**：接受，改成只读本用例进程的日志（不再整体清空）。**处理结果（2026-11-09，tianting-android@d50c230）**：不再清空 Logcat。`scanOwnLogcat` 读 `logcat -d -b all --pid=<本进程> -T <用例开始>`，逐行查，不整段读进内存；断言不变，另加"读到了日志"。两条用例都过，没有 OOM（模拟器，本地） |
| I10 | tianting-ios 踢线弹窗和 securityNotice 提醒（FE-620 截图） | 改密时提醒先到、踢线后到；弹窗从底部出，和提醒在同一位置，弹窗出来后提醒被盖住，用户可能看不到"你的账号密码已被修改" | 没有改。"确定"能点（XCUITest 断言）。要不要调整位置由 Frank 决定。**Frank 决定（2026-11-09，都按建议）**：把提醒移到弹窗之上。**处理结果（2026-11-09，tianting-ios@54caa87）**：踢线弹窗出现时由弹窗把提醒画在遮罩和卡片之上、屏幕上方，不接收点击；根视图的提醒层这时不显示。用例断言提醒在、和卡片不重叠、"确定"可点，浅色、深色截图确认（模拟器，本地） |

C2 三套 SDK（FE-630～632）实现时发现的。PRD 冻结，没有改；按"现在的实现"一列做了，**要 Frank 决定的标了"待决定"**。结果都是本地的，CI 不运行。

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| D1 | F-RCL-03 规则 2（server，已修） | 规则 2 写"撤回、后台删除、文件过期时递增 `mod_ver`"，服务端现在只在文件过期时递增（`sqlite_files.go`），后台删除（`DeleteMessages`）不递增。SDK 按任务单改成只重拉 `modVer` 变了的会话后，**断线期间被后台删除的、不是最后一条的消息，重连后不再被清掉**（原来每次重连全部重拉能清掉；最后一条仍靠 `convs` 的 `lastMsg.deleted`）。后台删除要 `IM_V13_ADMIN_FEATURES=1`，默认关闭 | 三套 SDK 都按规则 2 做了；`convs` 没有 `modVer`（老服务端）时仍全部重拉。建议 BE 在 `DeleteMessages` 同一事务递增 `mod_ver`（撤回随 C4 本来就要递增），这是后端改动，本次没有做。**2026-11-09 已修（照 PRD 补实现，不是新决定）**：`DeleteMessages` 同一事务先递增 `conv_seq.mod_ver`（已删除的重复删除不再递增）；TC-FILE-06-12 的 e2e 加了断言（删的不是最后一条，modVer +1；重复删除不变），去掉修复时该用例失败 |
| D2 | AC-FILE-07-10 H5 一侧、F-FILE-07 规则 4 | AC 写 H5 同样场景（页面没刷新）提示"原文件已不可用"；规则 4 说"有沙盒副本的回到上传中"，源文件没了的例子是"H5 刷新后"。sdk-ts 在内存里留着 File 直到发出，没刷新时 4096 会重新上传；刷新后整条消息已经不在（D4，AC-FILE-07-3），所以 `sourceMissing` 在 H5 上实际碰不到 | sdk-ts 有源就重新上传，没有源时 `error='sourceMissing'`。H5 页面怎么对 AC 由 FE-633、FE-634 定 |
| D3 | F-FILE-07 规则 3（按 PRD 字面统一） | "待发送的按现有规则自动发 1 次"：三套 SDK 的"现有规则"不同。sdk-kotlin 冷启动时 QUEUED、SENDING 一律转失败（原有规则，不在用户不知情时发旧消息），所以冷启动后的待发送附件也转失败；sdk-swift 冷启动时待发送的、以及已有 `fileId` 的发送中，按排队自动发 1 次（服务端按 `clientMsgId` 去重）。同一进程里断线重连三套都自动发 1 次 | 各自按原有规则，没有统一。**2026-11-09 处理**：PRD 规则 3 明写"待发送的……自动发 1 次"，第 820 行也说现有 QUEUED 重连后自动发，所以按字面统一成 Swift 的做法：sdk-kotlin 冷启动时**待发送的附件**（以及已有 `fileId` 的发送中）自动发 1 次，其他 QUEUED、SENDING 的原有规则不动。随 FE-635 做 |
| D4 | F-FILE-07 规则 3 | 冷启动时处理中、上传中"转为失败"没有给原因 | sdk-kotlin 记 `SendError.Network`，sdk-swift 记 `.interrupted`（sdk-ts 只在内存，没有这个场景）。名字不同，App 文案都是"发送失败"一类，三端页面时再统一 |
| D5 | F-FILE-07、第 13.2 节 | 上传的 PUT 没有规定超时 | 三套 SDK 都没有 SDK 层的超时（OkHttp 读写各 60 秒没有进展才失败，URLSession 每段 60 秒、整体 3600 秒，fetch / XHR 没有）；卡住时一直显示"上传中"，用户可以取消 |
| D6 | F-FILE-07 规则 2（sdk-kotlin） | 规则 2 只说退出登录时清除沙盒副本；被踢时 sdk-kotlin 本来就清本地库 | 被踢时沙盒副本和下载缓存一起删，之后重发失败的附件得到 `SourceMissing` |
| D7 | F-FILE-07 规则 3（原生） | 处理中被杀进程：App 的处理函数只在内存里，冷启动后没有源 | 这条消息转失败，重发得到 `SourceMissing`（"原文件已不可用，请重新选择"）。已经复制进沙盒的（上传中被杀）不受影响，重发从头上传 |
| D8 | F-FILE-07 异常（sdk-swift） | 复制进沙盒失败只写了"存储空间不足"；App 给的源文件已经不存在也会复制失败 | 同样记 `.storage`，文案对不上，页面任务再定 |
| D9 | 第 21 节 16（sdk-swift，接着 S2） | 实测 URLSession 对没有响应的 GET 会自己重发（2 次 GET 到达 4 次）；PUT 每次用新的会话，断在半路只到达 1 次 | 下载不阻止重发：只读、同一个预签名地址、只有 2xx 才写文件。只在 macOS、回环、HTTP/1.1 测过 |
| D10 | BE-403、第 13.1 节共享用例 | `protocol/vectors/chat-sync.json` 还没有（BE-403 负责） | 单调合并、`modVer` 只重拉变了的会话只有三套 SDK 各自的单测，没有共享向量；frames.json 的 image、file 用例同样等 BE-403。本次没有加 |
| D11 | F-FILE-01 规则 6a | 只说用户取消时发 `DELETE`；上传失败（PUT 失败、complete 报错）后留下的 `pending` 要等 PUT 地址过期才释放名额 | 三套 SDK 在没完成的上传失败时也尽力发一次 `DELETE`（幂等，失败只记日志），重发时从头申请新的 `fileId` |
| D12 | F-FILE-07、F-FILE-02 规则 2 | 失败原因里只有部分有 PRD 文案（4292、`uploadPendingLimit`、处理失败、存储空间不足、原文件不可用），网络或对象存储失败没有 | SDK 给出 `error`（错误码，或 `network`、`process`、`storage`、`sourceMissing` 等）和 `errorReason`，文案由页面任务定 |
| D13 | tianting-web Vitest（FE-633、FE-634） | 协议副本同步后 `errors.json` 的 4096 是 `ws` 通道，H5 的错误文案表没有 4096，用例"errors.json 里每个 ws 通道的错误码都有文案"失败 1 个（Vitest 260 个，259 过） | 没有改 H5 页面（不在这次范围），FE-633 补文案（候选是 PRD 的"原文件已不可用，请重新选择"） |
| D14 | P-35 L1 的边界 | L1 只看本地 token 的到期是否等于 `absoluteExpiresAt`，不看时间：最后一段 access（被上限截短的那个）有效期内，如果收到的 `kicked/tokenExpired` 不是到期而是被替换的代（不是自己刷新造成的），也直接回登录页 | 按决定实现。这种情况下刷新最多也只能换来同一个上限的 token；自己刷新造成的被替换仍按"本地更新"用本地值。三套 SDK 都有"普通 token 被踢仍刷新 1 次"的用例 |
