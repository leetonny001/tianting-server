# 前端任务单：天听聊天产品第一批（三套 SDK、H5、Android、iOS、管理后台页面，C1、C2、C3、C4、C5、C6）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 5～10、13、14.1、15、20.3 节；[PRD-ui v1.2](../prd/PRD-ui-v1.6.md)（界面规则 B-1～B-8，第 6 节视觉）；[SDK 设计说明](../sdk/README.md)；[errors.json](../../protocol/errors.json) |
| 范围 | **C1、C2、C3、C4 和公共部分**（CH-27）。C3 是 Frank 2026-11-10 决定按顺序继续的，同一天又决定"推送后续做，先做功能"，**C3 已暂停**（任务和用例保留）；C4（撤回、引用、@）同一天决定接着做；**C5（置顶、免打扰、删除会话）是 Frank 2026-11-10 说"开始 C5"后分发的**（QUEUE Q29）；**C6（敏感词和举报）是 Frank 2026-11-11 说"开始 C6"后分发的**（PENDING P-47，QUEUE Q30）；设备通知（C3）只有原生端，H5 没有离线通知（第 13.2 节）；【全部】的 AC 验证到 C6 为止的页面（第 15 节累计表，C3 那一列等 C3 恢复） |
| 负责 | 前端（AI） |
| 仓库 | tianting-sdk-ts、tianting-sdk-kotlin（`sdk-core` 纯 JVM）、tianting-sdk-swift、tianting-web（H5）、tianting-android、tianting-ios、tianting-admin |
| 编号 | **FE-600 起**。公共 FE-600～602，C1 FE-610～620，C2 FE-630～640，C3 FE-650～656，C4 FE-660～672，C5 FE-680～686，C6 FE-690～698 |
| 端的范围 | PC（Tauri）排除：`tianting-web` 宽屏界面照常实现，不验收打包形态（第 2.5 节）。Web Push、拍照不做 |
| 完成定义 | 任务内的 AC 有通过的测试（SDK：假 HTTP + `FakeTransport` + 假时钟 + 共享向量；H5：Vitest、Playwright；原生：Compose UI Test、XCUITest），用例编号见 [TC-ACC](../testcases/TC-ACC.md)、[TC-FILE](../testcases/TC-FILE.md)、[TC-APN](../testcases/TC-APN.md)、[TC-RCL](../testcases/TC-RCL.md)、[TC-QUO](../testcases/TC-QUO.md)、[TC-MEN](../testcases/TC-MEN.md)、[TC-CVS](../testcases/TC-CVS.md)、[TC-MOD](../testcases/TC-MOD.md)、[TC-CHAT](../testcases/TC-CHAT.md)。**CI 现在不运行（P-21），原生 CI 也缺 P-15，真机和其他浏览器在 P-13**，结果都要标明环境。**C3 收到真实系统通知要 Frank 先做第 20.3 节的 APNs、FCM 步骤**（PENDING P-40），之前原生端用令牌和授权的测试替身 + 服务端假通道 |

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
| FE-633 | 🟡 H5 部分完成 | tianting-web@b3b2a03（**Chrome，本地；CI 不运行**，WebKit 和真机（P-13）未执行）。聊天页"+"（`features.files` 关时不显示）、`<input type=file>`、拖放、粘贴（图片按图片、其他按文件），一次最多 9 个（多了整批不发，H16）；XHR 上传进度、上传中/待发送可取消（发 `DELETE`）；canvas 处理：先看头部宽高（像素上限）、`createImageBitmap` 按 EXIF 方向转正后重新编码（去掉 EXIF、GPS）、长边 4096、缩略图长边 480 ≤ 512 KiB、GIF 原样、透明 png 保留透明、webp 浏览器不能编码时 png、HEIC 在 Chrome 解不了提示用文件方式；用"文件"方式选图片先提示保留原始信息；失败文案（4292、`uploadPendingLimit`、处理失败、4139、4150、原文件不可用，网络失败的文案是实现时加的，H15）；刷新前离开确认（沿用 `pendingCount`）；头像：选图 → 裁剪（拖动、缩放、方向键）→ 512×512 jpeg → `kind=avatar` 上传 → `PATCH profile`，本次登录马上显示（H13）。D13 已处理：4096 的文案用"原文件已不可用，请重新选择"，Vitest 260 个原有用例全过。Vitest 301 个全过（原有 260 个，新增 41 个：`media.test.ts` 18、`files.test.tsx` 21、`tabGuard.test.ts` 2），typecheck、lint、format:check、check（颜色、对比度、品牌）通过，Playwright 对真实 im-server（`IM_FILES_ENABLED=1`）+ 假 S3（`e2e/fake_s3.py`，跨域、CORS 预检）：TC-FILE-01-12、01-17、02-01（H5 部分）、02-04、02-05、02-13、07-02、07-03、07-04、07-06、07-07、AC-ACC-06-6 通过。全量 Playwright（默认服务端）155 个：153 过、2 个跳过（03-12、03-45 要单独配置的服务端），之后新加的 2 个 axe 用例（TC-CHAT-90-08 的图片和文件页面，浅色、深色）单独跑过；TC-FILE-07-10 按 PRD 写、标成预期失败（H15），计在“过”里；变异 12 项 11 项被发现，存活 1 项是等价变异（过期的图片不排队取地址：SDK 对已知过期的不发请求）。**没有验收的部分**：AC-FILE-07-10 的 H5 部分（H14、H15：没刷新时用内存里的源重新上传，但现在的服务端清理后回 4048，用例按 PRD 写、标成预期失败）；AC-FILE-02-4 的"EXIF 解析工具"和 MinIO 复跑（H18）；AC-FILE-07-7 只验证到"失败 5 次都取消、没有 complete"（H19）；AC-FILE-02-10 是手工 + 真机（P-13），HEIC 只有 Vitest 替身 |
| FE-634 | 🟡 H5 部分完成 | tianting-web@b3b2a03（**Chrome，本地；CI 不运行**）。图片气泡按宽高占位、进入视口才取缩略图（30 毫秒内合成一批，SDK 再按 50 个一批）、取到的内容先看头部宽高再交给浏览器（解码炸弹显示"图片无法显示"）、按 `fileId + variant` 缓存成 `blob:`（最多 300 个，`filesExpired` 时丢掉）、加载失败点击重试、地址过期（403/400）重新申请一次；全屏看原图（缩放、保存、Esc）；GIF 播放；过期显示"图片已过期""文件已过期"且不申请地址；文件气泡：类型图标、文件名纯文本中间省略（扩展名可见）、大小，浏览器下载（签进去的 `attachment`），别人发的文件每个会话第一次提示来源（只在内存，H17）；摘要"[图片]""[文件]"；CSP：`connect-src` 只加构建变量 `VITE_STORAGE_ORIGINS` 里的具体源（不接受 `https:`、通配），`img-src` 加 `blob:`，nginx 示例的响应头改成 include（原来 `location /` 的 `add_header` 让 CSP、`Referrer-Policy` 没有发出去，H21）。Playwright：TC-FILE-02-06、02-07、02-08、02-09、02-12、02-14（H5）、03-01（H5）、03-03、03-04、03-07（中文文件名，在 03-01 里）、03-08、03-10（生产构建）、04-10（200 张，52 批，没有 4290，每张缩略图只下载 1 次）、04-11、05-03、06-05（IM 钟 181 天 + `file_purge`）通过。**没有验收的部分**：AC-CHAT-90-8、90-9：axe 扫了图片气泡、失败提示、“+”菜单、全屏查看、来源提示、头像裁剪框（浅色、深色，修了一处 `label-content-name-mismatch`），新消息播报（“收到一张图片”）H5 原来就没有，属于 FE-640；"保存权限被拒""存储空间不足"在 H5 没有对应状态；WebKit 未执行 |
| FE-639 | ✅ 完成（页面部分） | tianting-admin@f2281c1（**Chrome，本地；CI 不运行**）。元数据页类型显示中文（图片、文件），只有类型和正文长度，没有文件名（响应里本来就没有）；正文页文件显示"[文件] 文件名，大小（不可预览）"、图片显示"[图片] 宽×高，大小（不可预览）"，过期显示"已过期"，按纯文本，不请求、不内联渲染文件，后台 CSP 没有改。Vitest 108 个（新增 5 个）；全量 Playwright 59 个：56 过、3 个跳过（截图等默认跳过的用例），其中新增 `e2e/files.spec.ts` 2 个对真实 console-server + im-server（`IM_FILES_ENABLED=1`，假 S3）+ Postgres，生产 CSP 响应头，没有 CSP 违规、没有请求对象存储。**没有验收的部分**：PRD 第 14.1 节元数据"显示大小"：im-server 的元数据接口只有正文长度，没有文件大小（console-chat 问题 12） |
| FE-635 | 🟡 部分完成（Android） | tianting-android@778f042，SDK tianting-sdk-kotlin@3575060（**模拟器，本地；CI 不运行，真机（P-13）未执行**，emulator-5556，Pixel_10_Pro_XL，API 37）。聊天页"+"（`features.files` 关时没有）：系统照片选择器（`PickMultipleVisualMedia(9)`，不要存储权限）、SAF（多选超过 9 个整批不发，同 H16）；选中的内容先复制进 App 私有目录，图片轮到它时在后台处理（`ImageDecoder` 按 EXIF 方向转正后重新编码、去掉 EXIF/GPS，长边 4096，缩略图 480 ≤ 512 KiB，透明的输出 png，GIF 原样），交给 SDK 复制进沙盒（发出后删除，失败保留，退出登录清空）；处理中、上传中 N%、等待发送、取消（发 `DELETE`）、失败原因（PRD 文案 + D12 同 H5 的网络失败文案）；"文件"方式选图片先提示保留原始信息；"我"页点头像：裁剪（拖动、双指缩放）→ 512×512 jpeg → `kind=avatar` → `PATCH profile`，本次登录用本地文件显示（A30）。D3、H13 在 sdk-kotlin 处理（见问题表）。用例：`FilesUiTest` 15 个（假服务端 + 假对象存储）、`FilesE2eTest` 16 个（真实 im-server `IM_FILES_ENABLED=1` + `scripts/fake_s3.py`）、`FilesKillE2e`（真实杀进程，连跑 3 次过）、`MediaTest` 5 个（JVM）。通过的 AC：AC-ACC-06-6、AC-FILE-01-12、01-17、02-1、02-4（同 H18 的方法，没有 exiftool、没有 MinIO 复跑）、02-5、02-13、03-1、03-6、04-11（Logcat 扫描）、07-1、07-4、07-5（A26：测试目录的文件代替相册）、07-6、07-7（按 H19 推断：5 次 PUT 失败，没有新的最终对象）、07-9。**没有验收的部分**：AC-FILE-07-10 按 PRD 断言，服务端回 4048（A28 = H15，P-37 等 Frank），用例写成预期失败；AC-FILE-07-2 只有 SDK 单测（A27）；AC-FILE-02-10 手工 + 真机未执行，HEIC 和 API 28 以下没有跑（A22）。全量回归：`ui` 242 个（241 过，1 个是 `NavLoginTest` 的偶发，见 A33）、`e2e`（ServerE2eTest + AccountE2eTest + FilesE2eTest）53 个过（2 个按设计跳过：03-12、03-45 要单独配置的服务端）、`kill` 5 个场景过；`checkTianting`、`lintDebug`、`testDebugUnitTest` 41 个通过。问题见 A19～A33 |
| FE-636 | 🟡 部分完成（Android） | tianting-android@778f042（**模拟器，本地；CI 不运行**）。图片气泡按宽高占位（4000×3000 → 200×150 dp）、进入组合才取缩略图（LazyColumn，SDK 同一轮合并地址申请）、交给解码器前先看头部（像素上限、认不出的显示"图片无法显示"）、Coil 只解码 SDK 缓存里的本地文件（内存键 `fileId:variant`，A24）、加载失败"加载失败，点击重试"、过期"图片已过期""文件已过期"且不申请地址；全屏看原图（双指缩放、双击还原、保存到相册：API 29+ MediaStore，28 及以下申请权限，被拒提示"没有相册权限"和"前往设置"）；GIF 首帧 + "GIF"标记；文件气泡：类型标签、文件名纯文本中间省略、大小，App 内 OkHttp 下载到私有缓存（"下载中 N%"、取消、"下载失败，点击重试"从头再下），别人发的文件每个会话第一次提示来源（只在内存，同 H17），下载后 pdf、txt、图片才有"打开"（`FileProvider`），都有"保存"（SAF）；会话列表摘要"[图片]""[文件]"。通过的 AC：AC-FILE-02-1、02-6（下一条消息位移 ≤ 1 px）、02-7、02-8、02-9（只取视口附近的缩略图、0 次原图）、02-12、02-14、03-1（SHA-256 一致，两个方向）、03-4、03-5（A20：假 S3 断开连接代替断网）、03-8、03-9（只验证操作项，没有拉起其他 App，A25）、04-10（200 张，滚到顶再回来没有加载失败，缩略图下载 ≤ 200 次，再进一次全部命中缓存）、05-3、06-6。**没有验收的部分**：AC-CHAT-90-8、90-9 只有可读名称，新消息播报和 AccessibilityChecks 没有做（A29、A10）；"保存权限被拒"在 API 37 上走不到（A23）；"存储空间不足"没有制造磁盘满（A32） |
| FE-637 | 🟡 部分完成（iOS） | tianting-ios@52c6f2f，SDK tianting-sdk-swift@662e4f2（**模拟器，本地；CI 不运行，真机（P-13）未执行**，iPhone 16 Pro，iOS 18.4）。聊天页"+"（`features.files` 关时没有）：PHPicker（不要相册完整权限，一次最多 9 张）、`UIDocumentPicker`（多选超过 9 个提示"一次最多选择 9 个"、整批不发，v1.4 规则 6）；选中的先复制到临时目录，图片轮到它时在后台用 ImageIO 处理（先看头部像素上限，按 EXIF 方向转正后只写像素，再删掉 ImageIO 自己加的 Exif 段（I13），长边 4096，缩略图 480 ≤ 512 KiB，透明的输出 png，GIF 原样，HEIC 转 jpeg），交给 SDK 复制进沙盒；处理中、上传中 N%、等待发送、取消（发 `DELETE`）、失败原因（PRD v1.4 规则 9 的文案，同 H5、Android）；"文件"方式选图片先提示保留原始信息；"我"页点头像：裁剪（拖动、双指缩放）→ 512×512 jpeg → `kind=avatar` → `PATCH profile`，本次登录用本地文件显示（同 A30）。sdk-swift 的 H13、P-37 ① 已处理（见问题表）。用例：Kit 单测 `MediaTests` 14 个；XCUITest `FilesE2ETests` 25 个、`ZzFilesClockE2ETests` 2 个（真实 im-server `IM_FILES_ENABLED=1` + `scripts/fake_s3.py`）。通过的 AC：AC-ACC-06-6、AC-FILE-01-12、01-17、02-1、02-4（同 H18 的方法，没有 exiftool、没有 MinIO 复跑）、02-5、02-13、03-1、03-6、07-1、07-4、07-5（I14）、07-6（I15）、07-7（同 H19 推断：5 次 PUT 失败，没有新的最终对象）、07-9、07-10（v1.4 正常断言：有沙盒副本的重新上传后送达，没有的提示"原文件已不可用，请重新选择"、只能删除）。**没有验收的部分**：AC-FILE-04-11 部分通过：App 和 SDK 自己的日志里没有，系统 CFNetwork 的任务失败日志带完整预签名地址（I11，待决定）；AC-FILE-07-2 只有 SDK 单测（I18，同 A27）；AC-FILE-02-10 手工 + 真机未执行（I19）。全量回归：XCUITest 159 个（154 过、3 个按设计跳过、2 个新用例 02-09、04-10 碰到陌生人限额 4137 失败；改成先加好友、合并同一张图的并发加载（I23）之后，文件用例 27 个整组重跑全过），`check_e2e_leftovers` 通过（I11、I12 的系统日志单独计数）；Kit 单测 126 个（122 过、4 个跳过，同 FE-618）；`check_sources`、`check_tokens`、`gen_tokens --check` 通过。问题见 I11～I22 |
| FE-638 | 🟡 部分完成（iOS） | tianting-ios@52c6f2f（**模拟器，本地；CI 不运行，真机（P-13）未执行**）。图片气泡按宽高占位（4000×3000 → 200×150 pt）、行建出来才取缩略图（LazyVStack，SDK 同一轮合并地址申请）、交给解码器前先看头部（像素上限，认不出或超限"图片无法显示"）、解码后的图按 `fileId:variant` 缓存（最多 300 张，`filesExpired` 时丢掉）、加载失败"加载失败，点击重试"、过期"图片已过期""文件已过期"且不申请地址；全屏看原图（双指缩放、拖动、双击还原、保存到相册：只要"添加照片"权限，被拒提示"没有相册权限"和"前往设置"，I16）；GIF 首帧 + "GIF"标记；文件气泡：类型标签、文件名纯文本中间省略、大小，SDK 的 URLSession 下载到私有缓存（"下载中 N%"、取消、"下载失败，点击重试"从头再下、重新申请地址，文案同 Android，见 A34），别人发的文件每个会话第一次提示来源（只在内存，同 H17），下载后 pdf、txt、图片才有"打开"（Quick Look），都有"保存"（分享面板，I21）；会话列表摘要"[图片]""[文件]"。通过的 AC：AC-FILE-02-1、02-6（下一条消息位移 ≤ 1 pt）、02-7、02-8、02-9（只取视口附近的缩略图、0 次原图）、02-12、02-14、03-1（SHA-256 一致，两个方向）、03-4、03-5（I15）、03-8、03-9（pdf 有"打开""保存"、`.xyz` 只有"保存"；打开了 Quick Look，分享面板里的目标没有验证）、04-10（200 张，滚到最早一条再回来没有 4290，缩略图下载 ≤ 200 次，再进一次 0 次下载）、05-3、06-6（IM 钟 181 天 + `file_purge`）。**没有验收或只验证一部分**：AC-CHAT-90-8、90-9：可读名称（"+"是"发送图片或文件"，图片"图片"，文件"文件 文件名，大小"，头像"更换头像"，查看器"关闭"）、系统审计 `.sufficientElementDescription`（文件名单独的文字被审计判为"不可读"，改成由气泡统一读出）、浅色深色截图做了；新消息播报（"收到一张图片"）iOS 原来就没有，没有做（同 A29）；"存储空间不足"没有制造磁盘满（同 A32）；"保存权限被拒"在模拟器上用系统弹窗"不允许"走通，真机未执行 |
| FE-640 | 🟡 部分完成（验收已做，报告见 [c2-frontend-acceptance-2026-11-09](../reports/c2-frontend-acceptance-2026-11-09.md)） | **本地结果，CI 不运行**（Chrome；emulator-5556 Pixel_10_Pro_XL API 37；iPhone 16 Pro iOS 18.4）。48 条（FE-630～640 的 42 条 + 后端【全部】6 条）：通过 33、部分通过 12、未执行 3、失败 0。AC-FILE-02-1、03-1 三端互发互收 6 个方向通过：三端先后连同一个 im-server + 假 S3，各端用自己的界面发图片（1600×1200 纯色 jpeg）和群文件，再各端用自己的界面和 SDK 收（缩略图、原图、颜色、`size`，文件 SHA-256），用例 tianting-web@12ee72e `e2e/xp-files.spec.ts`、tianting-android@79893a7 `CrossPlatformE2e`、tianting-ios@7f1b997 `CrossPlatformE2ETests`（不是同时在线互发，是先发后收）。AC-CHAT-90-8、90-9：Android 新增文件页面的名字检查和浅色深色截图（`FilesUiTest` 2 个），发现能点的缩略图没有名字并修了（`Attachments.kt` 一行，`FilesUiTest` 17 个、`FilesE2eTest` 17 个过）；新消息播报三端都没有，**未执行**；图标清单没有逐个核对，记部分通过。**未执行**：AC-FILE-02-10（真机 P-13）、AC-FILE-90-2（云厂商 CH-05）、AC-CHAT-90-1（P-12）。AC-CHAT-90-15 按 PRD 第 15 节 C2 要 `chat-sync.json`，没有（BE-403），记部分通过。需要 Frank 决定的事见报告第 7 节 |
| FE-650～FE-656 | ⏸ 暂停（Frank 2026-11-10：推送后续做） | 2026-11-10 分发（C3），没有开工。真实通道和真机的部分挡在第 20.3 节和 P-13，见 FE-656 |
| FE-660 | ✅ 完成（sdk-ts） | tianting-sdk-ts@13fc3b5（**本地结果，CI 不运行**：Node、macOS；push 时 GitHub Actions 按 `on: push` 自动排了一次，没有手动触发）。协议副本同步到 tianting-server@bfbd2ba。`recall`（成功后本地立即撤回，服务端不给发起的连接发 `msgDeleted`）、`canRecall` / `messageActions`（ts + `auth` ack 校正后的服务端时间）、`send(convId, text, {quote?, mentions?})`（上行只发 `quote.msgId`，待发气泡本地拼引用条；单聊带 `mentions` 抛错）、`searchGroupMembers`（`keyword`）、`recallNotice` / `recallText`（F-RCL-02 规则 1a）、`quoteSummary` / `quoteLabel`、`messageFor` 增加 `recall:4097`（按 `windowSeconds`）、`recall:4030`、`send:4293`；`IMError.reason`（4040 `quoteUnavailable` 进 `errorReason`）。Store 单调合并带上 `recalled`、`quote`；`msgDeleted` 清除本会话所有引用它的 `quote.text`（含待发的、之后迟到的）；`mentionUnread` 实时 +1、本端读到最新清零，`readUpdated` / `msgDeleted` 节流 5 秒重拉 `convs`。AC 18 条都有用例：单测 `msg-actions.test.ts` 22 个、`legacy-c4-frames.test.ts` 1 个（TC-CHAT-90-26，同一文件在 `git archive f417580` 解出的老版本上也通过）、`chat-sync.json` C4 6 个（TC-CHAT-90-33，13 个用例全部通过）、`frames.json` C4 帧 54 个 + SDK 发出的 C4 帧 1 个（TC-CHAT-90-32）；集成 `test/integration/msg-actions.test.ts` 5 个（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`）。单测 762 个通过、2 个跳过（原有），集成 62 个通过。变异 21 项：19 项被发现（"已删除的消息在 sync 里补上 recalled"首轮存活，补用例后被发现），2 项等价（"自己发的 @ 也计数"被"自己发言即已读"挡住、只去掉已删除分支时 `deletedIds` 仍然挡住回写，都换了写法或补了断言）。**没有做**：AC-RCL-01-8 菜单期间过期的界面提示、AC-RCL-02-4 / 03-6 的页面文案在 FE-665；撤回播报按问题 14 不做。新问题 21～27 |
| FE-661 | ✅ 完成（sdk-kotlin） | tianting-sdk-kotlin@ea6994e（**本地结果，CI 不运行**：JDK 21、macOS；push 时 GitHub Actions 按 `on: push` 自动排了一次，没有手动触发）。协议副本同步到 tianting-server@c0d36d9（和 sdk-ts 的副本逐字节相同）。`recall`（ack 后本地立即撤回，问题 27）、`canRecall` / `messageActions`（ts + `auth` ack 校正后的服务端时间，非挂起、任意线程可调）、`send(convId, text, quote = QuoteRef, mentions = Mentions)`（上行只发 `quote.msgId`，待发气泡本地拼引用条）、`recallNotice` / `recallText`、纯函数 `recallNoticeOf` / `recallText` / `quoteSummary` / `quoteLabel`、`messageFor` 增加 `recall:4097`（`windowSeconds`）、`recall:4030`、`send:4293`；`IMError.reason`（`AccountError.reason` 改为继承，源码兼容），发送失败带 `errorReason`。Store 单调合并带上 `recalled`、`quote.state`；`msgDeleted` 清除本会话所有引用它的 `quote.text`（含待发的、之后迟到的）；`convs` 的 `lastMsg.recalled` 也用来补撤回（问题 32）。**本地库第 3 版**：`deleted_refs` 多 `kind='recalled'`（JSON 存 msgId / seq、撤回者、时间），第 1、2 版原地升级、旧行保留（问题 34）；撤回、清引用照旧走 `secure_delete=ON` + `wal_checkpoint(TRUNCATE)`（不支持时 VACUUM），用例搜数据库文件原始字节（问题 35）。下载中的文件所在消息被撤回或删除：中止、删部分文件，`DownloadAbortedError` + `IMEvent.DownloadAborted(fileId, recalled)`（问题 33）。`sdk-android`：升级路径全在 `LocalDb`，`AndroidLocalDatabase` 不用改，Robolectric 原生 SQLite 加了第 2 版 → 第 3 版和撤回清除的用例。AC 14 条都有用例：单测 `MsgActionsTest` 29 个（含 FE-662 的 6 个）、`MsgActionsPersistenceTest` 9 个（TC-RCL-03-02、03-04、03-12 的 SDK 部分、03-13、TC-QUO-02-02、升级、冷启动）、`LegacyC4FramesTest` 1 个（TC-CHAT-90-26，同一文件在 `git archive 2819ce6` 解出的老版本上也通过，问题 31）、`chat-sync.json` 13 个用例各跑内存和 SQLite 两遍（26 个，TC-CHAT-90-33 / 13，全部通过；SQLite 那遍最后搜数据库文件里的特征串）、`frames.json` C4 帧 54 个 + SDK 发出的 C4 帧、新字段解码各 1 个（TC-CHAT-90-32）、`AndroidAdapterTest` 1 个；集成 `MsgActionsIntegrationTest` 6 个（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`，含冷启动 TC-RCL-03-04）。全量：单测 1020 个（sdk-core 1015、sdk-android 5）通过，`:sdk-core:check` 通过；集成 82 个通过（第一轮全量里已有的 `AccountIntegrationTest`"攻击者转正……窗口后 replayDetected"失败 1 次，单独连跑 2 次、全量再跑 1 次都通过，没有复现）。变异 37 项（和 FE-662 合计）：36 项被发现（首轮 5 项存活：摘要从删除升级为撤回 2 项、`convs` 的 `lastMsg.recalled`、已读之前的 @，补用例后被发现；另 3 项首轮写法编译不过，换写法后被发现），1 项等价（"自己发的 @ 也计数"被"自己发言即已读"挡住，同 sdk-ts）。**没有做**：AC-RCL-03-12 的提示和界面、AC-RCL-01-8 菜单期间过期的提示在 FE-667。新问题 31～35 |
| FE-662 | ✅ 完成（sdk-kotlin） | 同 FE-661 的提交（tianting-sdk-kotlin@ea6994e，**本地结果**）。`Conversation.mentionUnread`、`firstMentionSeq`（随会话 json 进本地库，冷启动恢复）；实时 @ 我 / @ 所有人本地 +1（自己发的、已读位置之前的、服务端没给过计数的不推算），本端读到最新清零；有提及的会话收到 `readUpdated`、`msgDeleted` 时节流 5 秒重拉 `convs`（[SDK 钟]，窗口内的推迟到窗口结束，断线时不拉，退出登录清定时器，问题 23）；`searchGroupMembers(groupId, keyword, cursor)`（空串不带，4290 不自动重试）；`mentions` 去重，单聊带 mentions 入队前抛 `IllegalArgumentException`；4293、4131 标记失败不自动重发。AC-MEN-01-1、02-5（SDK 部分）、03-7 都有用例（单测 6 个 + `chat-sync.json` 提及 2 个 × 2 + 集成的群用例）。变异见 FE-661 |
| FE-665 | ✅ 完成（H5） | tianting-web@dcbc5b5（问题编号的注释在 @d72e5d4）（**Chrome，本地；CI 不运行**，WebKit 和真机（P-13）未执行），SDK tianting-sdk-ts@13fc3b5（`file:` 依赖，在 sdk-ts 重新 `npm run build` 了被忽略的 `dist/`，同 H8，HEAD 不变）。消息菜单按 `features.msgActions`：关时只有"复制"（e2e 用改写 `/v1/config` 验证，AC-UI-09-1），开时按 `messageActions`（"复制""引用""撤回"），菜单项在打开时算好，开着过了时限点"撤回"按 4097 提示；4030"你没有权限撤回这条消息"；撤回、删除的消息和提示行没有菜单。键盘入口：有菜单的气泡可 Tab 到，菜单键、Shift+F10 打开，方向键、Home、End、回车，Esc 关闭后焦点回到气泡。撤回文案和列表摘要用 `recallText`（规则 1a 三行都测了）；全屏看的图片被撤回时退出并提示"该消息已被撤回"；文件下载开始前再看一次状态（问题 16）。引用条（可取消，草稿只存 msgId，跟着撤回、删除变）、气泡顶部引用（四种状态 + 不认识的，纯文本）、点击跳转（200 条以内翻页高亮，更远"消息太早，请翻页查看"，拉不到"消息不存在"）。没有新图标（取消引用用已有的 `close`），新样式只用已有 token。Vitest 新增 51 个（`c4.test.tsx` 30 个、`mentions.test.ts` 21 个，和 FE-666 合计），全量 352 个通过；typecheck、lint、format:check、`npm run check`（token、对比度、颜色字面量、品牌）、build、check-dist、check-assets、check-bundle 通过。Playwright 新增 `e2e/c4.spec.ts` 21 个（真实 im-server，`IM_MSG_ACTIONS_ENABLED=1`，SQLite + 假 S3），全部通过；全量 180 个：175 过、4 跳过（原有）、1 失败：`wide.spec.ts`"只用键盘…（AC-UI-90-9）"第一下 Tab 没落到导航栏，把本次改动 `git stash` 掉、只留新的 sdk-ts `dist` 重跑同样失败，不是本次引入，没有修（后来查明：C4 之前的 tianting-web@12ee72e + sdk-ts f417580 同样失败，原因是本机 Chrome 11-10 08:37 自动升到 157，登录输入框被移除后第一下 Tab 离开页面；tianting-web@8923a5b 改成从刚打开的会话列表起步，全量通过）。TC-RCL-01-08 的"设备时钟快 10 分钟"在 Vitest（假时钟 + `serverTime` 偏移），e2e 用调慢 IM 钟造 125 秒、112 秒前的消息。覆盖 AC-RCL-01-1、01-8、02-4、02-5、03-3、03-6、03-8、03-10、AC-QUO-01-5、02-3、02-5、AC-CHAT-90-8（没有新图标，脚本检查通过）、90-9（axe 浅色、深色，键盘走查）。**没有做**：撤回的新消息播报（问题 14、P-43）。变异见 FE-666 一行。问题 36、37、41 |
| FE-666 | ✅ 完成（H5，AC-MEN-01-9 手工未执行） | 同 FE-665 的提交。输入模型"文本 + `[{start, len, userId}]`"（`lib/mentions.ts`）：行首或空白后敲 `@`、`＠` 才打开（`a@b.com`、粘贴的不打开），组字中不改模型、不打开（组字结束再看），光标不进 @ 内部，退格一次整段删除并去掉 mentions，单聊不打开。成员选择（`keyword` 搜索、加载中、无结果、加载失败可重试，关掉后 @ 是普通文字、可以继续发），显示备注、头像、userId，插入公开昵称（R-CF-03，e2e 抓上行帧搜不到备注）；"所有人"只给群主、管理员。结构化草稿 `{text, mentions, quote}`（仍只在内存，问题 15）；列表前缀"[有人@我]" > "[草稿]"；被 @ 的气泡整体样式提示；"有人@我"按钮（点了翻页滚动并高亮后消失）；4293 消息失败、提示"@所有人次数已用完"一次、断线重连后不重发（e2e 真实 4293）。覆盖 AC-MEN-01-1、01-5、01-6、01-7、01-8、01-10、01-12、02-4、02-5、03-5、03-6、AC-CHAT-90-8、90-9；AC-MEN-01-9 只有自动化部分（模拟 composition 事件），macOS 拼音输入法录屏要人手做，**未执行**。变异（FE-665、FE-666 一起）32 项：Vitest 31 项、e2e 1 项（全屏不跟着撤回关闭，被 TC-RCL-03-08 发现）；31 项被发现，1 项存活：去掉聊天页的 4293 提示测试照过，原因是 App 本来就统一弹发送失败、聊天页那一份是重复的（会弹两次），已删掉并断言只提示一次，再把"App 不弹"作为变异被发现。问题 38～40 |
| FE-663 | ✅ 完成（sdk-swift） | tianting-sdk-swift@b73dd2e（**本地结果，CI 不运行**：`swift test`，macOS、Xcode 工具链；push 时 GitHub Actions 按 `on: push` 自动排了一次，没有手动触发，也没有等它）。协议副本同步到 tianting-server@9a3c40d（`schema/`、`vectors/`、`errors.json` 和 sdk-ts、sdk-kotlin 的副本逐字节相同）。`recall(_:_:)`（ack 后本地立即撤回，问题 27）、`canRecall` / `messageActions`（ts + `auth` ack 校正后的服务端时间）、`send(_:_:quote:mentions:)`（上行只发 `quote.msgId`，待发气泡本地拼引用条；单聊带 `mentions` 抛 `IMArgumentError`）、`recallNotice` / `recallText`（F-RCL-02 规则 1a，聊天页和列表同一张表；`RecallNotice` 的操作者字段叫 `operatorId`，Swift 关键字）、`quoteSummary` / `quoteLabel`、`IMError.reason` / `Message.errorReason`、`messageFor(cmd:code:windowSeconds:)`；`AccountConfig.Features.msgActions: Bool?`（问题 36，老服务端 `nil`）。合并：`recalled`、`quote.state` 单调，`deletedIds` 优先，`msgDeleted` 带 `recalled` 时清本会话所有引用它的 `quote.text`（含待发），`convs` 的 `lastMsg.recalled` 直接按撤回清（问题 32）。**问题 17 核实**：`deletedIds` 原来没有跨冷启动持久化（只有已加载的行以 `deleted=true` 写回），本地库第 3 版新增 `deleted_refs` 表（id、seq、recalled，和 Kotlin 的 kind 一致），第 1、2 版原地升级（问题 34、42）；清引用和撤回一样 `secure_delete`，不支持时 VACUUM（问题 35）。下载中被撤回、删除、过期：取消传输 Task、删半截文件，抛 `DownloadAbortedError(fileId, recalled)`、发 `.downloadAborted(fileId:recalled:)`（问题 33，Swift 用 enum case 写法）。用例 50 个（单测 44：`MsgActionsTests` 30、`MsgActionsPersistenceTests` 11、`LegacyC4FramesTests` 1、`ChatSyncVectorTests` +2；真实 im-server 集成 6：`MsgActionsIntegrationTests`，`IM_MSG_ACTIONS_ENABLED=1`，go build 到临时目录、结束按进程停掉），名字带 TC 编号；全部 476 个单测、82 个集成测试通过。`chat-sync.json` 13 个用例内存和 SQLite 各跑一遍（26 次）通过，最后搜数据库文件里的特征串。TC-CHAT-90-26：同一个 `LegacyC4FramesTests.swift` 放进 `git archive c8ec6ff` 解出的目录（换上新的协议副本）跑过，通过（老版本 `MsgBody` 没有 `quote`，解码时丢掉，和问题 31 的 Kotlin 相同）。变异检查 28 项：27 项被发现，1 项等价（`mentionsMe` 去掉"不是自己发的"：自己的消息先把 readSeq 推到这条，`bumpMention` 本来就不加）；中途 3 项存活（VACUUM 回退、断线时拉 convs、下载不中止时用例挂住），补强用例后都能发现 |
| FE-664 | ✅ 完成（sdk-swift） | 同 FE-663 的提交（tianting-sdk-swift@b73dd2e，**本地结果**）。`Conversation.mentionUnread`、`firstMentionSeq`（在会话 JSON 里进本地库，冷启动恢复）；实时 @ 我 / @ 所有人本地 +1（自己发的、已读位置之前的、服务端没给过计数的不推算），本端读到最新清零；有提及的会话收到 `readUpdated`、`msgDeleted` 时节流 5 秒重拉 `convs`（[SDK 钟]，窗口内的推迟到窗口结束，断线时不拉，退出登录清定时器，问题 23）；`searchGroupMembers(_:keyword:cursor:)`（空串不带 `keyword`，4290 不自动重试）；`send` 带 `mentions` 的 4293、4131 标记失败不自动重发。AC-MEN-01-1、02-5、03-7 有单测，01-1、02-5、03-7 另有真实 im-server 集成用例（`IM_MENTION_ALL_HOURLY=1`） |
| FE-667 | ✅ 完成（Android） | tianting-android@41d5c87，SDK tianting-sdk-kotlin@686e14f（composite build，同级目录检出在这个提交；**模拟器，本地；CI 不运行，真机（P-13）未执行**，emulator-5556，Pixel_10_Pro_XL）。消息菜单按 `features.msgActions`：关时只有"复制"和失败消息的"重发""删除"（老服务端没有这个字段按关，问题 36）；开时按 SDK `messageActions`，**长按时现算**（撤回按 `ts` 和校正后的服务端时间；菜单开着过了时限，点了按 4097 的 `windowSeconds` 提示，AC-RCL-01-8）；4030"你没有权限撤回这条消息"、4044"消息不存在或已过期"、超时按服务异常，不自动重试（问题 26、37）；撤回、删除的提示行没有菜单；已发送的图片、文件也能长按（引用、撤回），长按带可读名称"消息操作"（问题 47）。撤回文案用 SDK `recallNotice` + `recallText`（聊天页提示行和列表摘要同一张表，显示名按本端规则），后台删除仍是"该消息已被删除"；全屏看的图片被撤回、删除时关闭并提示；下载中被撤回提示"该消息已被撤回"、不留部分文件（`DownloadAbortedError`，`recalled=false` 不提示，问题 46）；冷启动后本地库搜不到原文（真实杀进程）。引用条（输入框上方，可取消，只记 msgId，找不到时"引用内容不可用"，跟着撤回、删除变）、气泡顶部引用（`quoteLabel` 四种状态，Compose `Text` 按纯文本），点击跳转：200 条以内一次最多 200 条往上翻页后滚动并高亮 2 秒，更远"消息太早，请翻页查看"，拉不到"消息不存在"（问题 48）。被 @ 的气泡品牌色描边。颜色都是已有 token，图标只用了清单里已有的 `x`（AC-CHAT-90-8，`checkTianting` 照扫文案和颜色）；没有加 `espresso-accessibility`，语义树检查能点的节点都有名字，浅色、深色截图（AC-CHAT-90-9）；新消息播报按 P-43 不做。用例：JVM `C4Test` 23 个；假服务端 `C4UiTest` 32 个 + `FilesUiTest` 2 个（TC-RCL-03-08、03-12）；真实 im-server（`IM_MSG_ACTIONS_ENABLED=1`，服务端编译到仓库外的临时目录，结束按 PID 停）`C4E2eTest` 14 个 + `FilesE2eTest` 3 个（全屏撤回、限速下载中撤回、引用图片和文件）；真实杀进程 `C4KillE2e`（TC-RCL-03-04，`run-tests.sh kill`）。三个 e2e 用例读本进程 Logcat：撤回原文、引用原文、@ 的内容、备注、token 都搜不到。全量（本地）：ui 278 个通过；e2e 71 个（`ServerE2eTest`、`AccountE2eTest`、`FilesE2eTest`、`C4E2eTest`）68 通过、2 个按设计跳过（03-12、03-45 要单独的环境变量）、1 个 `C4E2eTest` 用例登录等待超时（不是 C4 功能），之后 `C4E2eTest` 又连跑两遍，第二遍 14/14（第一遍又一次登录超时，加了重试和诊断）；kill 6 个场景通过；`checkTianting`、`lintDebug`、`testDebugUnitTest` 64 个通过。测试脚本：`test-server.sh` 默认打开 `IM_MSG_ACTIONS_ENABLED`、调用方加 `group.write`（托管群指定群主）、`TT_SERVER_STATE_DIR`；e2e 把 IM 端口 `adb reverse` 到模拟器，对方经这条路连接，App 开飞行模式时它不断（TC-RCL-03-03、TC-MEN-02-05）。变异 27 项（和 FE-668 合计）：26 项被发现（首轮 5 项存活或写法编译不过：toMentions 的 @ 检查、组字中的 @、组字结束的基准、长按时现算菜单、全屏撤回，补用例或换写法后被发现），1 项等价（"有人@我"点了不隐藏：那条进了视口按钮同样消失）。新问题 45～48 |
| FE-668 | ✅ 完成（Android，AC-MEN-01-9 手工未执行） | 同 FE-667 的提交（tianting-android@41d5c87，**模拟器，本地**）。输入模型 `core/Mentions.kt`（文本 + `[{start, len, userId}]`，和 H5 的 `lib/mentions.ts` 同语义）：行首或空白后敲 `@`、`＠` 才打开（`a@b.com`、一次进来多个字的粘贴不打开），**组字中（`composition` 不为空）不改模型、不打开**，上屏后按组字前的文字算一次；光标不进 @ 内部；没有选区、光标在 @ 后的退格整段删除并去掉 mentions；单聊、开关关闭时不打开。成员选择是底部面板：`searchGroupMembers`（`keyword` 停 300 毫秒再搜）、加载中、无结果、加载失败（点了重试；关掉后 @ 留作普通文字，可以继续发）、"加载更多"，列表显示备注优先的显示名、头像和 `userId`，不含自己；插入公开昵称（没有用 `userId`），不用备注；"所有人"只给群主、管理员（AC-MEN-02-4）。草稿结构化（`Draft{text, mentions, quote}`，只在内存，问题 15）。列表前缀"[有人@我]" > "[草稿]"，有提及时跟最后一条摘要（问题 38）；打开会话时只取一次 `firstMentionSeq`，"有人@我"按钮点了一页页翻到那条、滚动并高亮，点了或那条进了视口都消失（问题 40）。4293 由 App 统一提示"@所有人次数已用完"，消息失败，断网重连后不自动重发（真实服务端核对）。AC 都有用例（TC 编号在用例名里，用例数见 FE-667）。**未执行（要人工）**：AC-MEN-01-9 系统中文输入法组字中输入 @（模拟器 + 真机 P-13），自动化部分是 JVM 的组字用例；真实输入法退格时重新组字的情况见问题 45 |
| FE-669 | ✅ 完成（iOS） | tianting-ios@f6c0411，SDK tianting-sdk-swift@b73dd2e（本地 SPM 路径依赖，同级目录在这个提交；**模拟器 iPhone 16 Pro（iOS 18.4），本地；CI 不运行，真机（P-13）未执行**）。消息菜单是系统长按菜单，按 `features.msgActions`：关时只有"复制"和失败消息的"重发""删除"（没有这个字段、老缓存都按关，问题 36）；开时按 SDK `messageActions`。SwiftUI 的菜单随行一起建好，所以每行按 `ts` 和校正后的服务端时间在 2 分钟、7 天到点时重画一次，下次长按就没有"撤回"；菜单开着时（用自定义 preview 的出现、消失判断）不重画，开着过了时限点"撤回"按服务端 4097 的 `windowSeconds` 提示（AC-RCL-01-8，问题 49）。4030"你没有权限撤回这条消息"、4044"消息不存在或已过期"、超时按服务异常，不自动重试；撤回、删除的提示行没有菜单；已发送的图片、文件也有菜单（引用、撤回）；菜单项同时是 VoiceOver 的"操作"（问题 49）。撤回文案用 SDK `recallNotice` + `recallText`（聊天页提示行和列表摘要同一张表，显示名备注优先）；全屏看的图片移到聊天页管理，被撤回、删除时关闭并提示；下载中被撤回提示"该消息已被撤回"（`.downloadAborted`，`recalled=false` 不提示，问题 46），SDK 的缓存和 `.part-` 不留，但 URLSession 自己的临时文件会留下（问题 52）；冷启动后数据目录里搜不到原文（真实杀进程）。引用条（输入框上方，可取消，只记 msgId，找不到时"引用内容不可用"）、气泡顶部引用（`quoteLabel` 四种状态，`Text(verbatim:)` 纯文本），点击跳转：200 条以内翻页后滚到中间并高亮 2 秒（`surfaceSelected`），更远"消息太早，请翻页查看"，拉不到"消息不存在"。没有新图标（取消引用用 `close`），颜色都是已有 token（`check_sources.py`、`check_tokens.py` 通过）；XCUITest `performAccessibilityAudit(.sufficientElementDescription)` 在聊天页、引用条、@ 选择浅色和深色下都没有问题（AC-CHAT-90-8、90-9）；新消息播报按 P-43 不做。用例：单测 `C4Tests` 22 个（全量 148 个通过、4 个原有跳过）；真实 im-server（`IM_MSG_ACTIONS_ENABLED=1`，scripts/e2e-server.sh 默认打开，服务端编译到 scratchpad，跑完由脚本停掉）`C4E2ETests` 18 个、`C4FilesE2ETests` 3 个，全部通过（断网用已有的 `TCPProxy` 切断 App 的连接，问题 50）。变异 20 项：单测 17 项（@ 触发的空白规则、组字中改模型、整段退格、交叠区间、光标进 @、发送时的 @ 检查、开关关闭、提示行有菜单、列表撤回文案、前缀优先级、4097 时限、跳转 200 条、引用条撤回文案、后台删除也提示、自己 @ 自己、老缓存按开、只有引用的草稿），首轮 15 项被发现，2 项存活（发送时的 @ 检查、老缓存按开），补用例后被发现；页面 3 项用端到端（全屏不跟着撤回关闭、草稿丢引用、不发 mentions）都被发现。另外开发中两种写法被 TC-RCL-01-08 发现：菜单只在建行时算（过了 2 分钟长按仍有"撤回"）、到点就重画（开着的菜单里"撤回"消失）。全量回归（本地）：`ChatE2ETests`、`FilesE2ETests`、`ModerationE2ETests` 和 C4 两个类同一轮 66 个，65 通过，`testImageSaveToPhotosAndRefusedPermission` 失败一次、单独重跑通过；1 个按问题 52 标记的预期失败。 新问题 49～52 |
| FE-670 | ✅ 完成（iOS，AC-MEN-01-9 手工未执行） | 同 FE-669 的提交。输入模型 `Core/Mentions.swift`（文本 + `[{start, len, userId}]`，按 UTF-16 下标，和 H5、Android 同语义）：行首或空白后敲 `@`、`＠` 才打开（`a@b.com`、粘贴的不打开）；`markedTextRange` 不为空（组字中）不改模型、不打开，上屏后按组字前的文字算一次；光标不进 @ 内部；光标在 @ 后的退格整段删除；单聊、开关关闭时不打开。成员选择是底部面板（`searchGroupMembers`，`GroupMembersModel` 加了 keyword，停 300 毫秒再搜）：加载中、无结果、加载失败（点了重试；关掉后 @ 留作普通文字）、"加载更多"，显示备注优先的显示名、头像和 userId，不含自己；插入公开昵称，不用备注；"所有人"只给群主、管理员。草稿结构化（`Draft{text, mentions, quote}`，只在内存，问题 15）。列表前缀"[有人@我]" > "[草稿]"，有提及时跟最后一条摘要（问题 38，读屏文本同步）；被 @ 的气泡品牌色描边，读屏多读"提到了你"；打开会话时只取一次 `firstMentionSeq`，"有人@我"按钮点了一页页翻到那条、滚动并高亮，点了或那条进了视口都消失（问题 40）。4293 由 App 统一提示"@所有人次数已用完"，消息失败，断线重连后不自动重发（真实服务端）。AC 都有用例（TC 编号在用例名里，用例数见 FE-669）。**未执行（要人工）**：AC-MEN-01-9 系统中文输入法组字中输入 @（模拟器 + 真机 P-13），自动化部分是单测 `testTC_MEN_01_09_…`（组字事件）；XCUITest 用英文键盘，没有真实组字 |
| FE-672 | ✅ 完成（AC-MEN-01-9 手工未执行，推送等 C3） | 报告 [c4-frontend-acceptance-2026-11-10](../reports/c4-frontend-acceptance-2026-11-10.md)（**本地；CI 不运行**，H5、后台 Chrome，iOS 模拟器 iPhone 16 Pro；真机、WebKit 未执行）。先落实问题 52（sdk-swift@c953dde、tianting-ios@4e6ecdb）和 P-45（PRD-chat v1.7，tianting-server@8527b7b，sdk-swift@c97d08e、sdk-kotlin@d6603f5、sdk-ts@7739c55、tianting-ios@62e3355）。45 条 AC：**通过 39、部分通过 4、未执行 2、失败 0**（第一轮 AVD 被 emulator-5554 占用，通过 16、部分通过 27；Frank 关掉 5554 后同日补跑 Android，23 条有了实测证据改为通过）。跨端（新用例 tianting-web@3cd6e96 `xp-c4.spec.ts`、tianting-android@fd5058a `CrossPlatformC4E2e`、tianting-ios@ee522bc `CrossPlatformC4E2ETests`）：6 个方向都通过；反证三次（H5、iOS、Android 各一次）。全量回归（本地）：im-server、console-server 全过；sdk-ts 766 + 集成 63，sdk-kotlin 1017 + 集成 82 + Android 5，sdk-swift 482 + 集成 82；H5 单测 354、e2e 176 过；后台单测 125、e2e 58 过；Android 单测 64、lint、`checkTianting` 过，补跑 ui 278 全过、e2e 71 个 67 过 2 跳过 2 失败（C2 的 `FilesE2eTest` 两个，单独重跑通过）、kill 6 个全过；C4E2eTest 的登录重试实测没有掩盖慢登录（14 次 1.2～2.5 秒，tianting-android@def3170 记录用时）；iOS XCUITest 184 个 175 过、2 失败（04-10 重跑通过；ACC-03-40 改动前也失败，问题 53）、Kit 148 个通过。AC-MEN-01-9 手工未执行（要人工）；推送等 C3（P-43 ②），新消息播报不做（P-43 ③）。新问题 53 |
| FE-671 | ✅ 完成（页面部分） | tianting-admin@f3db9c6（**Chrome，本地；CI 不运行**：push 时 GitHub Actions 按 `on: push` 自动排了一次，没有手动触发）。元数据页状态列显示"已撤回（撤回人，时间）"，和"已删除"分开，没有文件名、引用文字（AC-RCL-04-7）；正文页普通授权的撤回显示"该消息已被撤回（撤回人，时间）"、不显示正文（响应里就没有，万一带了也不显示），勾选删除照旧（问题 29），隔离授权显示撤回原文和"撤回人 …，时间"；`quote` 显示"引用：{摘要}"（F-QUO-02 规则 3 的文案）、`mentions` 显示"提及：所有人、userId…"（去重），都按纯文本（AC-RCL-04-3）。隔离模式下撤回或后台删除的图片、文件有"下载查看"：`POST /console/api/files/{fileId}/fetch`（`{convId, msgId, variant:"original"}`，CS-202），字节进内存后用隐藏链接按 `attachment` 另存，文件名取隔离正文里的文件名（console 问题 16，图片见问题 28），文件名不进 URL、请求和日志，blob 地址 1 秒后撤销，不内联渲染，CSP 没有改；4039 弹"工单号 + 理由 + 动态码"，授权后接着下载；4048 提示"文件不存在或已过了保留期"；传到一半断了按网络错误，不保存残缺文件。`apiBlob` 和 `api` 共用 CSRF、4037 重放、4038 step-up。Vitest 123 个（新增 12 个，`src/pages/Recall.test.tsx`，用例名带 TC-RCL-04-03、04-07、04-09）；全量 Playwright 61 个：58 过、3 个跳过（截图等默认跳过的用例），其中新增 `e2e/recall.spec.ts` 2 个对真实 console-server@427131e + im-server（`IM_MSG_ACTIONS_ENABLED=1`、`IM_FILES_ENABLED=1`，假 S3）+ 临时 Postgres 17，生产 CSP：下载的文件名和 SHA-256 与原文件一致，代理响应 `attachment`、`nosniff`，请求 URL 和请求体、控制台都没有文件名和原文，blob 地址都撤销了，没有 CSP 违规、没有请求对象存储。e2e 环境打开了 `IM_MSG_ACTIONS_ENABLED`（测试隔离密钥也加进了 dist 和 HAR 扫描）。typecheck、lint、format:check、build、check:dist 通过。**变异 26 项全部被 Vitest 发现**（撤回状态显示、撤回人字段、普通/隔离分支、撤回文案、引用摘要、提及去重、引用改成 HTML、按钮的隔离/撤回/过期/类型条件、请求体 `variant`、文件名进 URL、4039 后不续下载、日志打印文件名、撤回的不能勾选、不撤销 blob、立即撤销、链接不移除、不设 `download`、文件名不清理、错误响应当文件、断流不按网络错误、4048 文案），其中 5 项另用 e2e 跑过，也都被发现。**没有验收的部分**：后台删除（未撤回）的图片、文件下载只有 Vitest，e2e 只做了撤回的；单聊 `IM_C2C_BODY_ACCESS=off` 没有在页面另测（console 透传 4030，页面显示"单聊正文查看未开启"照旧） |
| FE-680 | ✅ 完成（sdk-ts） | tianting-sdk-ts@97124bd、4f2f4cc、b58d8ec、a536459（**本地结果，CI 不运行**）。协议副本同步到 tianting-server@b2d0d97（含 BE-487 修正版 ec5f78a）。`setConvPinned`、`setConvNotifyOff`：乐观更新（新置顶用校正后的服务端时间排最前，已置顶再置顶 `pinnedAt` 不变），ack 的 `conv` 覆盖本地（本地有更新的消息时只取设置）；5000、4220、4290、4030、超时、未连接一律回滚并抛出，不自动重试（问题 56），等待期间其他端事件改过的不回滚。`deleteConv`：`upToSeq` 取本地看到的最大 seq，ack 前先从列表消失、失败恢复；成功后 `minSeq = clearedSeq + 1`、`readSeq = max(readSeq, clearedSeq)`、取消置顶、保留免打扰，内存清除 `seq < minSeq` 的消息（文件经 `filesExpired`，迟到的 sync、ack 不写回），上传中的附件取消、排队和失败的消息丢弃（问题 59），发 `convDeleted` 让页面清草稿（问题 55）；保留下来的消息里的引用摘要照常显示（问题 62）；本机发送中（含超时标记失败）的消息，迟到的 ack 落在清除范围里时一起去掉，附件经 `filesExpired`，只认同一发送者的 clientMsgId（问题 68）。`convSettingsChanged`、`convDeleted` 事件覆盖本地，其他设备删除时不丢本机未发出的消息（问题 63）；删除后来了新消息，会话按删除时的状态重新出现（AC-CVS-03-2）。`convs` 的 `deleted` 里的群不发 `convRemoved`、不生成快照（AC-CVS-03-5），`truncated` 或 `deletedTruncated` 为 true 时不推断（问题 54）。服务端 `minSeq` 比本地下限小时（重新入群且 historyVisible，含离线期间移出又加入）放低 `MessageList.floor`，来源 `convs` 和 `convAdded`，`convAdded` 同时按 `minSeq` 清除（问题 65）。`conversations()` 置顶在前按 `pinnedAt` 倒序（相同时 `updatedAt` 倒序、`convId` 升序，同 Go `chatmodel.Conversations`），其余按 `updatedAt` 倒序、同时间按 `convId`；`totalUnread()` = 普通会话 `unread` 之和 + 免打扰会话 `mentionUnread` 之和。`features.convSettings`（P-46 ② 关闭时照常应用已有设置）。单测 870 个：868 通过、2 个原有跳过（`conv-settings.test.ts` 43 个；`frames.json` C5 用例 51 个按 Schema 校验即 TC-CHAT-90-38；`chat-sync.json` C5 用例 6 个全部通过即 TC-CHAT-90-39）；typecheck、lint、check:protocol、prettier、build 通过；变异累计 39 项全部被发现。真实 im-server 集成 69 个全部通过（`conv-settings.test.ts` 含 TC-CVS-04-01、04-05 加 03-02、03-05、04-02、03-12 的客户端侧；服务端 df461f0 的提交内容） |
| FE-681 | ✅ 完成（sdk-kotlin） | tianting-sdk-kotlin@7f13c3f（**本地结果，CI 不运行**）。协议副本同步到 tianting-server@0685375。语义同 FE-680：`setConvPinned`、`setConvNotifyOff` 乐观更新、ack 覆盖、5000/4220/4290/4030/超时/未连接回滚并抛出、不自动重试（问题 56）；`deleteConv`（upToSeq 取本地最大 seq，ack 前隐藏、失败恢复，取消上传、丢弃排队和失败消息，问题 59），`IMEvent.ConvDeleted`（问题 55）；`convSettingsChanged`、`convDeleted`（不丢本机未发出的，问题 63）；`convs` 的 `deleted` 不推断移出（AC-CVS-03-5），`truncated`/`deletedTruncated` 不推断（问题 54）；排序置顶在前；`totalUnread` 同口径；`Features.convSettings`（P-46 ②）；服务端 `minSeq` 变小时（重新入群且 historyVisible）放低本地可见下限（问题 65）。**本地库物理清除**（AC-CVS-03-10）：任何会话 `seq < minSeq` 的行、未发出消息里的引用原文（保留消息的引用摘要照常，问题 62）、缓存文件（`FilesExpired`），secure_delete/checkpoint/VACUUM 同 C4。**本地库第 4 版**（原地升级，有用例）：`cleared_convs`（隐藏的会话）、`pending_purges`（"待清除"，清除失败下次 `open()` 补清，含缓存文件）；置顶、免打扰进库，冷启动先显示。`sdk-android` 走同一份 SQL（Robolectric 系统 SQLite：v3→v4 升级、清除后数据库文件搜不到特征串、没有 -wal）。单测 sdk-core 1114 个（新增 44 个）、sdk-android 6 个（新增 1 个），check、lint 通过；变异 37 项全部被发现。真实 im-server 集成 `ConvSettingsIntegrationTest` 6 个通过（含 TC-CVS-03-10 字节搜索），全部集成 88 个通过（服务端构建自 BE-486 eddee5a 时的工作区，BE-487 合入后重跑）。补做见 tianting-sdk-kotlin@6835069（**本地结果，CI 不运行**）：协议副本同步到 tianting-server@dff3378（含 BE-487 修正 ec5f78a）；`ChatSyncVectorsTest` 跑 `chat-sync.json` 的 6 个 C5 用例，内存和 SQLite 各一遍共 12 个通过（TC-CHAT-90-39）；置顶时间相同时按 `updatedAt` 倒序、`convId` 升序（同 Go 参考实现）；问题 65 改为全覆盖（`convs`、`convAdded` 的 `minSeq` 比本地下限小就放低，不要求有快照；`convAdded` 按 `minSeq` 清除）；问题 68（本机待发消息含超时失败的，迟到的 ack 落在清除范围里时去掉，同时删本地库的行，附件经 `FilesExpired`，只认同一发送者的 clientMsgId；实时 `msg` 帧触发时页面要等下次刷新才消失，记录）。sdk-core 1134 个（新增 20 个），check、lint 通过；新增 9 项变异全部被发现；真实 im-server 集成 88 个通过（服务端 dff3378 的干净工作区） |
| FE-682 | ✅ 完成（sdk-swift） | tianting-sdk-swift@94fda7b、141357f（**本地结果，CI 不运行**）。协议副本同步到 tianting-server@ec5f78a（BE-487 修正版）。`setConvPinned`、`setConvNotifyOff`、`deleteConv`（`async throws`）语义同 FE-680：乐观更新；ack 的 `conv` 覆盖本地；5000、4220、4290、4030、超时、未连接一律回滚并抛出，不自动重试（问题 56）；等待期间其他端改过的不回滚。本端删除时取消上传、丢弃排队和失败的消息（问题 59）；其他设备删除时保留本机没发出的消息（问题 63）；发 `.convDeleted`（问题 55）。保留下来的消息里引用摘要照常显示（问题 62）。迟到的 ack 落在清除范围里的本机待发消息一起去掉（问题 68）。`convs` 的 `deleted` 里的群不推断为"已不在该群"，`truncated` 或 `deletedTruncated` 时不推断（问题 54）。服务端 `minSeq` 比本地下限小时放低下限（`convs`、`convAdded`，问题 65）；`convAdded` 按 `minSeq` 清除。排序：置顶的按 `pinnedAt` 倒序，相同再按 `updatedAt` 倒序、`convId` 升序（同 Go `convListOrder`）。`totalUnread()` 新口径。`convId` 本地先严格校验，不合法直接抛 `ConvIdError`。**本地库物理清除**（AC-CVS-03-10）：清除 `seq < minSeq` 的消息行、空闲页（不支持 secure_delete 时 VACUUM）和缓存文件；本地库升到第 4 版，`pending_purges` 先记后清，中途失败或被杀时下次启动补做；第 1～3 版原地升级，有用例。单测 531 个全部通过（新增 49 个，含 frames.json C5 用例 51 个按 Schema 校验即 TC-CHAT-90-38；chat-sync.json C5 用例 6 个在内存和 SQLite 上都通过，即 TC-CHAT-90-39）。反证 39 处全部抓到。真实 im-server（dfa4338 的提交内容）集成：C5 的 7 个（TC-CVS-04-01、04-05 加 03-02、03-05、04-02、03-10，AC-CVS-03-12）全部通过，集成共 89 个通过 |
| FE-683 | ✅ 完成（H5） | tianting-web@78c2b2f，e2e 帧记录去重在 @a05c5c5（**Chrome，本地；CI 不运行**，WebKit 和真机未执行），SDK tianting-sdk-ts@97124bd（`file:`），服务端 im-mvp@541bebd 已提交的内容（含 BE-485、486、487，不含当时未提交的 BE-488）。会话菜单按 `features.convSettings`（关闭和老服务端都没有菜单，已有置顶、免打扰照常显示，P-46 ②）：窄屏长按 500ms（移动 10px 取消，松手不进入）、宽屏右键、会话项可 Tab 到，菜单键和 Shift+F10 打开，方向键、Home、End、回车，Esc 关闭后焦点回到会话项；三项文字按当前状态。SDK 回滚后提示，4220"最多置顶 20 个会话"（真实服务端 21 个群），4290"操作过于频繁"，不自动重试（问题 56）。置顶背景用新 token `color.surface.pinned`（浅 #F3F4F8 / 深 #23262E，对比度组合 27～30），宽屏选中盖过它；免打扰显示小红点、"[N 条]"（N 照实）、`bell-off` 图标（可读名称"免打扰"）；前缀只显示一个，"[有人@我]" > "[草稿]" > "[N 条]"，有 @ 也是小红点（问题 61）。标签角标和标题按 SDK 新口径（AC-CVS-02-2 显示 5、02-4 标题"天听"、02-5 显示 3）。删除有确认框，取消时都不变；`convDeleted`（本端或其他设备）清这个标签页的草稿和开着的输入框（问题 55），上传中附件的预览 `blob:` 和进度释放（问题 59），图片缓存靠 `filesExpired`；宽屏删除开着的会话回到列表。顺带修了两处原有缺陷：不在列表里的空群一直"加载中"（问题 66），宽屏导航有角标时可读名称不带数字（axe label-content-name-mismatch，问题 67）。Vitest 新增 27 个，全量 381 通过；typecheck、lint、format:check、`npm run check`、build、check-dist、check-assets、check-bundle 通过。Playwright 新增 `e2e/c5.spec.ts` 21 个全部通过（连跑 3 遍 63 个都过；首次在 78c2b2f 时 6 个因服务端还没有 BE-485、486 失败，重跑全过；重跑中 TC-CVS-03-08 暴露测试脚本重复记帧，a05c5c5 按 id+cmd 去重，断言不变）；全量 e2e 在 78c2b2f 时 203 个：191 过、6 跳过（原有）、6 失败即上述 6 个，之后只重跑了 c5.spec。覆盖 AC-CVS-01-6、02-2、02-3、02-4、02-5、03-2、03-8、04-1、04-2、04-5，AC-CHAT-90-8、90-9。axe 浅色深色、键盘走查（TC-CHAT-90-37）通过。变异 Vitest 36 项、e2e 3 项全部被发现。PRD-ui 的 token、对比度和图标已由 BE-488 写入 v1.5 |
| FE-684 | ✅ 完成（Android；AC-CVS-02-6 等 C3 补跑） | tianting-android@f0556b1，SDK tianting-sdk-kotlin@7f13c3f（composite build；**模拟器，本地；CI 不运行，真机（P-13）未执行**，emulator-5556，Pixel_10_Pro_XL）。会话菜单按 `features.convSettings`（关闭或老服务端没有菜单、没有辅助操作，已有置顶、免打扰照常显示，P-46 ②）：长按（名称"会话操作"）出"置顶 / 取消置顶""免打扰 / 取消免打扰""删除"，文字按当前状态；同样三项也是辅助操作；"删除"先确认。SDK 乐观更新，页面只提示、不重试（问题 56）：4220"最多置顶 20 个会话"，convSet / convDelete 的 4290"操作过于频繁"（问题 71）。置顶背景 `color.surface.pinned`（同 H5、PRD-ui v1.5）；免打扰小红点、"[N 条]"（照实）、`bell-off`（可读名称"免打扰"）；前缀"[有人@我]" > "[草稿]" > "[N 条]"（问题 61）。"消息"标签角标用 SDK `totalUnread`。`IMEvent.ConvDeleted` 清本机草稿和打开着的输入框，关掉这个会话的菜单和确认框，丢掉被丢弃附件的上传进度（问题 55、59）；`FilesExpired` 丢掉下载状态和内存里的图。AC-CVS-02-6：`AppDeps.alert` 是留给 C3 的出口（不调用），用例断言没有通知、替身没调用；C3 后 FE-653 补跑。用例：JVM `C5Test` 5 个 + 对比度 5 组；`C5UiTest` 16 个；真实 im-server + 假 S3 `C5E2eTest` 6 个（04-01 两个方向、03-02、04-05、04-02 断网、03-08 缓存和沙盒副本、规则 4）；`C5KillE2e`（AC-CVS-03-10 界面部分，D2 飞行模式离线，冷启动后本地库和缓存搜不到特征串）。全量（本地）：ui 294 通过；e2e 77 个：74 通过、2 按设计跳过、1 个 `C4E2eTest.tc_rcl_01_08` 失败（模拟器时钟比 Mac 慢 9 秒，问题 72，和 C5 无关）；kill 7 个场景通过；`checkTianting`、`lintDebug`、单测 69 个通过。变异 16 项全部被发现，e2e、kill 反证各 1 项按预期失败。**补（问题 72）**：tianting-android@b1466ff（只改测试）。`tc_rcl_01_08` 改用 SDK 校正后的服务端时间 `serverNow()`，2 分钟时限不变；单独连跑 3/3 通过（这次模拟器时钟差 0 秒，修复靠原理成立）；反证 `IM_RECALL_WINDOW_SEC=60` 时失败（见问题 73）。`C5E2eTest` 03-08、04-05 查草稿改为等 ack 之后的 `ConvDeleted`（04-05 仍限 2 秒，结果要求不变）。用 sdk-kotlin@6835069 重跑全量（模拟器，本地）：单测 69、lint、`checkTianting` 通过；ui 294 通过；e2e 77 个：73 过、2 按设计跳过、03-08 时序失败 1 次（已修，之后 6/6 连跑 2 次）、`FilesE2eTest.tc_file_02_06` 偶发（问题 74）；kill 7 个通过。**补（问题 74）**：tianting-android@17b2ee7（只改测试）。`tc_file_02_06` 先调慢假 S3 再让对方发图；原纯色缩略图只有 1797 字节（假 S3 每 16 KiB 才等一次，调慢不起作用），改用大于 48 KiB 的噪点缩略图，宣称尺寸和其他断言不变，另加"量位置时还在加载"（更严格）。单独连跑 5/5，加检查后再 5/5；反证去掉调慢后 3 次里 2 次看不到加载状态；`FilesE2eTest` 整个类 20/20 通过 |
| FE-685 | ✅ 完成（iOS） | tianting-ios@2b06bda，SDK tianting-sdk-swift@141357f（本地 SPM 路径依赖；**模拟器 iPhone 16 Pro，本地；CI 不运行，真机（P-13）未执行**），服务端 im-mvp@df461f0 已提交的内容（`git archive` 导出构建）。会话菜单按 `features.convSettings`（关闭、老服务端、老缓存都没有菜单，已有置顶、免打扰照常显示，P-46 ②）：长按系统菜单三项，文字按当前状态，同样三项是 VoiceOver 的"操作"；4220"最多置顶 20 个会话"，convSet、convDelete 的 4290"操作过于频繁"（问题 56、71），不自动重试。置顶背景 `color.surface.pinned`（同 H5，对比度 L/D27～31 进 `check_tokens.py`）；免打扰小红点、"[N 条]"、Lucide `bell-off`（PRD-ui 6.6、B-3；协调方任务说明误写成 SF Symbol，按 PRD 执行），整行读"免打扰"；前缀"[有人@我]" > "[草稿]" > "[N 条]"（问题 61）。"消息"标签角标按 SDK `totalUnread()`，可读名称"消息，5 条未读"（问题 67）。删除确认框；`.convDeleted` 清草稿、开着的输入框和权限提示，关掉开着的确认框（问题 55）。修了空群一直"加载中…"（问题 66）。AC-CVS-02-6：提示音、震动、本地通知只能写在 `Core/Alerts.swift`（`check_sources.py` 检查），用例断言通知为 0、替身没有调用；C3 后 FE-655 补跑；App 图标角标随 C3（问题 60）。Kit 单测 161 个：157 通过、4 原有跳过（新增 `C5Tests` 13）；XCUITest C5 13 个全部通过（`C5E2ETests` 10、`C5FakeServerTests` 1、`C5FilesE2ETests` 2），覆盖 AC-CVS-01-6、02-2、02-3、02-5、02-6、03-2、03-8、03-10（界面部分）、04-1、04-2、04-5，AC-CHAT-90-8、90-9。反证 15 项，14 项被发现（"列表不随事件更新"对 03-10 存活：冷启动后状态靠 SDK 清除，FE-682 已反证）。全量 XCUITest 197 个：186 通过、7 原有跳过、4 失败（问题 53 的 `testTC_ACC_03_40`；`testTC_ACC_10_01`、`testImageSaveToPhotosAndRefusedPermission`、`testTC_FILE_04_10` 单独重跑都通过，偶发）；`check_e2e_leftovers.py` CFNetwork 带文件名为 0 |
| FE-686 | 🟡 完成（AC-CVS-02-6 的通知部分等 C3） | 报告 [c5-frontend-acceptance-2026-11-11.md](../reports/c5-frontend-acceptance-2026-11-11.md)（**本地结果，CI 不运行**；Chrome、模拟器，真机和 WebKit 未执行）。18 条 AC：通过 14、部分通过 4（AC-CVS-02-6 通知部分等 C3；AC-CHAT-90-8、90-9 原生图标清单和 axe 等价；AC-CHAT-90-2 等 P-12）、未执行 0、失败 0。跨端 6 个方向（同一用户 D1、D2 分在两端同时在线：置顶、免打扰 2 秒内，删除消失并清草稿，重新出现只算新消息，离线删除重连后一致）都通过，三个接收端反证都按预期失败；跨端用例 tianting-web@145d0fe、tianting-android@b81c592、tianting-ios@b80cd93。没有改实现。全量：三套 SDK 重跑通过；H5、管理后台、Android ui/e2e、iOS 全量采用同日第一段的日志；Android kill 偶发崩溃记问题 75；iOS 问题 53 仍稳定失败。问题 73 结论：PRD 不要求客户端跟随服务端配置 |
| FE-690 | ✅ 完成（sdk-ts） | tianting-sdk-ts@d5fb273、7a8272a（**本地结果，CI 不运行**）。协议副本从 tianting-server@66ad0e5 已提交的内容同步（`git archive`，问题 90），`frames.json` C6 用例 13 个合法、32 个非法（含 6ef4217 新增的 5 个）。`report({targetType, convId?, msgIds?, userId?, groupId?, reason, note?})` → `reportId`：本地先校验（`targetType` 互斥（空串、null 也算带了）、`reason` 六选一、`msgIds` 1～20 条且格式合法、`note` ≤ 200 码点、不能是聊天室、单聊自己必须是参与者、不认识的字段也拒绝），不合法抛 4000（`details.field`），不发帧；4292、4040、4030、4290、5000、超时原样抛出，不自动重试；`note` 不进日志和错误信息。`canReport(m)`（第 77、88、89、92 条）：别人发的、已发送的 `text`、`image`、`file`，且 `seq ≤` 会话的 `readSeq`（会话不在本地列表时没有）；撤回提示行只在别人撤回的消息上有（自己以群主、管理员身份撤回的、自己的消息被别人撤回的没有，`recalledBy` 为空按别人撤回）；不看 `features.moderation`。4510：`send`、`sendImage`、`sendFile` 标记失败，不自动重发、不重新上传，`canRetry()` 为 false（第 81 条，`sourceMissing` 同样）；迟到的 4510 改失败原因、不再发 `sendFailed`（第 94 条）；账号接口抛 `AccountError {451, 4510}`。`messageFor` 文案按 P-48 ⑥、第 83 条。`features.moderation`（第 76 条）。单测 937 个：935 通过、2 原有跳过；typecheck、lint、check:protocol、prettier、build 通过；反证 29 项全部被发现。真实 im-server（66ad0e5）集成 `report.test.ts` 6 个全部通过（举报成功、2 秒内双方无帧、幂等；举报用户、群、群消息；非成员举报群消息和不存在的用户都是 4040 且响应相同，单聊非参与者本地 4000 不发帧（问题 91）；发送被拦截 4510；建群名 4510；注册、改昵称 451/4510），集成全量 75 个通过 |
| FE-691 | ✅ 完成（sdk-kotlin） | tianting-sdk-kotlin@9e31f6d、128e789（**本地结果，CI 不运行**）。协议副本同步到 tianting-server@d91a48b（从已提交内容 `git archive` 导出，问题 90）。`IMClient.report(ReportTarget.Messages/User/Group, ReportReason, note?)` → `reportId`：本地校验口径和顺序同 FE-690，不合法抛 4000、不发帧；服务端错误原样抛出、不自动重试；`note` 不进日志。`canReport(m)`（第 77、88、89、92 条）：别人发的、已发送的 text、image、file，且 `seq ≤` 会话的 `readSeq`（会话不在本地列表、没有 seq 的不显示）；撤回提示行只在别人撤回的别人的消息上有。4510：标记失败、不自动重发、不重新上传，`canRetry()` 为 false（第 81 条），迟到的 4510 改失败原因、不重复发 `SendFailed`（第 94 条）；失败原因存本地库，冷启动后仍不能重发；账号接口抛 `AccountError(451, 4510)`。`messageFor` 同 FE-690。`Features.moderation`。单测 sdk-core 1205 个全部通过（`frames.json` C6 用例 46 个即 TC-CHAT-90-46），sdk-android 7 个通过；反证 38 项全部被发现。真实 im-server（d91a48b，BE-492～498）集成 `ReportIntegrationTest` 7 个全部通过（`features.moderation`、举报消息、用户和群、4040、第 92 条的 readSeq、发送 4510、建群 4510、注册和改昵称 451/4510） |
| FE-692 | 🔄 进行中（sdk-swift） | — |
| FE-693 | ✅ 完成（H5） | tianting-web@dd82a82、53f36d1（**Chrome，本地；CI 不运行**，WebKit 和真机未执行），SDK tianting-sdk-ts@7a8272a，服务端 im-mvp@d10d7bb 已提交的内容（含 BE-492～498）。入口按 `features.moderation`（第 76 条）：消息菜单"举报"排在"撤回"后；撤回提示行"举报"（点击区域不小于 44×44，字号和提示行文字不变，第 93 条）；资料卡菜单（自己的没有）；群信息页（只对当前成员）。消息菜单和撤回提示行调用 SDK 的 `canReport(m)`，页面不自己判断（第 77、88、89、92 条）。举报单：6 个理由默认不选；补充说明按码点数到 200；提示"所选内容将提交给平台审核"；成功"已提交，感谢你的反馈"，单聊消息和资料卡询问拉黑（已拉黑、群消息和群举报不问）；4292、4040、4030 提示后关闭；5000、4290、超时、断线保留内容、手动再提交，不自动重试。4510：失败气泡按 `canRetry` 只有"复制""删除"；非消息场景提示"内容包含不当信息，请修改后重试"，表单内容保留；迟到的 4510 不再弹提示（第 94 条）。图标 Lucide `flag`（PRD-ui v1.6 6.6 节）。顺带修了窄屏右键时长按计时把菜单再打开一次。Vitest 全量 410 通过（c6 29 个）；各项检查通过。`e2e/c6.spec.ts` 12 个全部通过（含真实 report、真实发送拦截和群名拦截；连跑 3 遍 36 个都过）；全量 e2e 217 个：209 过、8 原有跳过、0 失败。变异 Vitest 27 项、e2e 6 项、44px 1 项全部被发现 |
| FE-694 | 🟡 页面完成（Android；`ChatTest` 偶发待查） | tianting-android@013a3d3（实现）、3586530（第 92 条用例），SDK tianting-sdk-kotlin@128e789（composite build；**模拟器，本地；CI 不运行，真机（P-13）未执行**，emulator-5556，Pixel_10_Pro_XL），服务端 im-mvp@60cc2a5 已提交的内容（BE-492～498）。入口按 `features.moderation`（第 76 条）：消息菜单"举报"（SDK `canReport`，页面不自己判断，第 77、88、89、92 条）；别人撤回的提示行"举报"（点击区域 ≥ 44 dp，读到后跟着出现）；资料卡菜单、群信息页。举报单、4510 的处理同 H5（第 78、79、81、83、84 条，P-48）。图标 Lucide `flag`。`C6E2eTest` 6 个全部通过（开关、4510 发送拦截、单聊举报并询问拉黑、撤回的群消息举报、资料卡和群举报、群名 4510）。全量：单测 76、lint、`checkTianting` 通过；ui 312 通过（第二次全量；第一次 3 个偶发，单独重跑通过）；e2e 83 个：77 过、2 按设计跳过、3 个 `FilesE2eTest` 偶发（类重跑 20/20）；kill 7 个（`FilesKillE2e.uploadKilled` 偶发 1 次，同问题 75）。反证 24 项全部被发现。**待查**：`ChatTest.send_failure_toasts_once_each_attempt` 在本版 16 次失败 4 次、b81c592 上 10/10 通过，正在对照排查 |
| FE-695 | ⬜ 未开始（等 sdk-swift FE-692） | 2026-11-11 分发（C6，QUEUE Q30）。不依赖 C3；开发和测试只用特征词 |
| FE-696 | ✅ 完成（后台页面） | tianting-admin@7ce064f、802742c（**Chrome，本地；CI 不运行**），console-server@9f00aba，im-server 用 im-mvp@72230ec 已提交的内容 + 临时 Postgres 17。新页面"词库"（`/moderation/words`，按 `moderation.words` 守卫）：列表（原文纯文本、状态、更新时间）、搜索、单个添加（先确认，PRD-admin 5.3 节；已有的提示"状态不变"）、停用 / 启用、删除（都有确认框）；批量导入 txt（UTF-8，一行一个、去首尾空白，空行交服务端计入无效），超过 5000 行提示不提交，确认后显示新增、重复、无效（AC-MOD-01-7，第 86 条）。命中统计只对有 `report.read` 的显示（P-48 ③）。词列表不进本地存储。e2e `words.spec.ts` 2 个通过（含真实 4510 拦截、运营看不到命中统计）；反证单测 10 项、e2e 5 项全部被发现 |
| FE-697 | ✅ 完成（后台页面） | tianting-admin@6a40d90、29d24f9（环境同 FE-696）。新页面"举报"（`report.read`）：队列默认只看"待处理"、可切"全部"，按状态、理由、时间筛选，相同对象聚合"N 次"（AC-MOD-04-6），单聊消息不显示会话。打开用 `POST .../view`（console 第 28 条）：单聊消息举报先填理由（每天 50 个，4292 提示在框内）；元数据没有 note、文件名。举报详情有命中统计入口（backend 第 121 条、P-48）：只对有 `report.read` 的显示，查被举报人，点了才查，群举报没有。证据（`report.evidence`）：4039 弹授权框（理由 + 当次动态码，工单号固定为举报 ID）；正文、隔离原文（AC-MOD-03-11）、补充说明、文件名纯文本（AC-MOD-04-15），平铺水印（账号 + 时间到分钟 + 授权前 8 位）；图片同源 POST 取后用 `data:` 显示（console 第 29 条，CSP 不变），其他文件只有下载（AC-MOD-04-12）；c2cBodyDisabled、`undecryptable` 分别提示。结案：已处理 / 驳回 + 理由，4098 提示"举报已被处理"并刷新；处置只是链接，不自动执行。单测全量 169 个通过；全量 e2e 68 个：65 过、3 原有跳过（`reports.spec.ts` 5 个，`securitypolicyviolation` 为空）；反证单测 21 项、e2e 12 项全部被发现。AC-MOD-04-1 完整流程随 FE-698 |
| FE-698 | ⬜ 未开始 | 2026-11-11 分发（C6，QUEUE Q30） |
| 其余 | ⬜ 未开始 | Kotlin、Swift 照 [SDK 设计说明第 6 节](../sdk/README.md) 做 |

## 总览

| 第 18 节的列 | 公共 | C1 | C2 | C3 | C4 | C5 | C6 | 任务 |
|---|---|---|---|---|---|---|---|---|
| sdk-ts | 1 | 3.5 | 3.5 | 0 | 3 | 1.5 | 1 | FE-600、FE-610、FE-630；C3 只同步协议副本（BE-450）；C4 FE-660；C5 FE-680；C6 FE-690 |
| sdk-kotlin | 1 | 4 | 5.5 | 3 | 4.5 | 2.5 | 1 | FE-601、FE-611、FE-631、FE-650、FE-661（3）、FE-662（1.5）；FE-681（2.5）；FE-691（1） |
| sdk-swift | 1 | 4 | 5.5 | 3 | 4.5 | 2.5 | 1 | FE-602、FE-612、FE-632、FE-651、FE-663（3）、FE-664（1.5）；FE-682（2.5）；FE-692（1） |
| H5 | 0 | 5 | 6 | 0 | 5 | 2 | 1.5 | FE-613（2.5）、FE-614（2.5）；FE-633（3）、FE-634（3）；C3 没有（H5 无离线通知）；FE-665（3）、FE-666（2）；FE-683（2）；FE-693（1.5） |
| Android | 0 | 5.5 | 8 | 5 | 6 | 3 | 2 | FE-615（2.5）、FE-616（3）；FE-635（4）、FE-636（4）；FE-652（2.5）、FE-653（2.5）；FE-667（3.5）、FE-668（2.5）；FE-684（3）；FE-694（2） |
| iOS | 0 | 5.5 | 8 | 5 | 6 | 3 | 2 | FE-617（2.5）、FE-618（3）；FE-637（4）、FE-638（4）；FE-654（2.5）、FE-655（2.5）；FE-669（3.5）、FE-670（2.5）；FE-685（3）；FE-695（2） |
| console + 后台页面（页面部分） | 0 | 1 | 1 | 0 | 1 | 0 | 3 | FE-619、FE-639；其余 CS-200～203；C4 FE-671，console 部分 CS-204（CS-202 人日在 C2）；C5 没有页面改动，console 部分 CS-205（0.5）；C6 FE-696（1）、FE-697（2），console 部分 CS-206～208（3） |
| 测试（前端部分） | 0 | 3.5 | 4 | 2.5 | 3.5 | 1.5 | 2.5 | FE-620、FE-640、FE-656；其余 BE-422、BE-440、BE-459；C4 FE-672，其余 BE-478、BE-480；C5 FE-686，其余 BE-487、BE-489；C6 FE-698，其余 BE-499 |
| 文档（前端部分） | 0 | 0.5 | 0 | 0 | 0 | 0 | 0 | FE-620；C4 的 `docs/sdk/README.md` 在 BE-479；C5 的 `docs/sdk/README.md` 在 BE-488；C6 的 `docs/sdk/README.md` 在 BE-497 |
| **本单合计** | **3** | **32.5** | **41.5** | **18.5** | **33.5** | **16** | **14** | **145** |

**差异说明**：和第 18 节一致（按列拆分，不是独立重估，**低置信**）。三份任务单合计 145（C1 + C2 + 公共），加 C3 38.5 共 183.5，再加 C4 48.5（本单 33.5、backend-chat 14.5、console-chat 0.5）共 232，再加 C5 24.5（本单 16、backend-chat 8、console-chat 0.5）共 256.5，再加 C6 28（本单 14、backend-chat 11、console-chat 3）共 284.5，和第 18 节合计一致，对照见 [backend-chat](backend-chat.md)。把每个端拆成两个任务只是为了每个任务不超过 4 人日，两个任务的 SDK 接口是同一套。

**依赖顺序**：BE-400 → FE-600 → FE-601、FE-602 → FE-610 → FE-611、FE-612 → FE-613、FE-615、FE-617 → FE-614、FE-616、FE-618 → FE-619 → FE-620 →（C1 校准）→ BE-403 → FE-630 → FE-631、FE-632 → FE-633、FE-635、FE-637 → FE-634、FE-636、FE-638 → FE-639 → FE-640 →（C2 校准，Frank 决定 C3）→ BE-450 → FE-650、FE-651（先用 `FakeTransport`，BE-452、BE-454 好了接真实 im-server）→ FE-652、FE-654 → FE-653、FE-655 → FE-656（真机部分等第 20.3 节和 P-13）。**C3 暂停后先做 C4**：BE-470、BE-478 → FE-660 → FE-661、FE-663（可以并行）→ FE-662、FE-664；FE-660 → FE-665 → FE-666；FE-661 → FE-667 → FE-668；FE-663 → FE-669 → FE-670；CS-204、CS-202 → FE-671；全部 → FE-672。**C5**：BE-482、BE-487 → FE-680 → FE-681、FE-682（可以并行）；FE-680 → FE-683；FE-681 → FE-684；FE-682 → FE-685；BE-489 和三端 → FE-686。**C6**：BE-491 → FE-690 → FE-691、FE-692（可以并行）；FE-690 → FE-693；FE-691 → FE-694；FE-692 → FE-695；CS-206 → FE-696；CS-207、CS-208 → FE-697；BE-499 和三端、后台 → FE-698。

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
  3. Play 服务可用性替身、系统通知授权替身（第 4.5 节）属于 C3，不在这里，见 FE-650。
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
  3. "待退出"持久化 `sid` 和 refresh，联网补发（F-ACC-04 规则 1）。系统令牌注销的调用点留接口，FCM 接入随 C3，见 FE-650、FE-652。
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
  4. `setAppState`、通知设置属于 C3，见 FE-652、FE-653。
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

### P-37 落实（Frank 2026-11-09 "都按建议"，PRD-chat v1.4）
- ① 4048 按已清理处理（三套 SDK）：只对**这台设备自己上传得到的** `fileId`（SDK 记录过 `complete` 成功的），`send` 回 4048 时和 4096 一样：有源（沙盒副本 / H5 内存里的 File）回到"上传中"重新上传，不算自动重发次数；没有源转失败，`sourceMissing`。别人的 `fileId`（转发等）收到 4048 照常失败。各补单测（含"别人的 fileId 不重传"），Android A28、H5 H15 的预期失败用例改回正常断言。iOS 随 FE-637 那一波一起改。
- ③ 后台元数据页显示文件大小：im-server `/admin/convs/messages/query` 已加 `fileSize`（原文件字节数，已删除的不出现；TC-FILE-91-01 加了断言）；console 透传，tianting-admin 元数据页的"大小"对图片、文件显示 `fileSize`（人类可读），其他消息仍显示正文长度。
- ④ CS-202、FE-639 里"经代理查看内容"挪到 C4，随 `POST /admin/files/{fileId}/fetch`（AC-RCL-04-9）。
- ②⑤ 只改 PRD 和用例，不改代码。
  - **① 处理结果（sdk-ts、sdk-kotlin、H5、Android，2026-11-09）**：两套 SDK 记下 `complete` 成功得到的 `fileId`（sdk-ts 在内存，退出登录清空；sdk-kotlin 记在沙盒记录 `attachments` 的 json 里，跨冷启动，旧行按 null 解码，本地库版本不变），`send` 回 4048 时只有这个 `fileId` 按 4096 处理（有源回到上传中重新上传、不占自动重发次数；没有源 `sourceMissing`），别人的 `fileId` 照常失败（4048）。tianting-sdk-ts@4f4312d：Vitest 新增 4 个（自己上传的图片、文件重新上传成功且之后断线还会自动补发 1 次，源不在时 `sourceMissing`，别人的 `fileId` 不重传，TS 没有转发入口，这条直接往 SDK 的消息仓库放一条带别人 `fileId` 的失败消息），全量 607 个过；去掉实现时前 3 个失败，改成"所有 4048 都重传"时"别人的"那条失败；typecheck、lint、format 通过，tianting-web 经 `file:` 链接用重新构建的 dist。tianting-sdk-kotlin@1f5eccb：新增 5 个（同上，另有"冷启动后仍认得自己上传的 fileId"，别人的 `fileId` 用本地库里一条失败消息造），`:sdk-core:test` 812 个、`integrationTest` 76 个全过；两种变异各被发现（4 个、1 个失败）。H5（H15）：tianting-web@bdf0cd8，`zz-im-clock.spec.ts` 的 TC-FILE-07-10 去掉 `test.fail`，断言 send 错误只有 1 个（4096 或 4048，实测 `[4048]`）、2 个新 PUT、新 `fileId`、对方收到 1 张；IM 钟只在跑 `file_purge` 时推进 25 小时、重发前拨回（同 Android 的写法：演示登录的 token 固定 24 小时，钟停在 25 小时后重新上传的 `/v1/files/*` 只能 4011，见 H23）。单独连跑 3 次过；SDK 去掉实现时失败（`[4048]`、消息失败）；`files-real` + `zz-im-clock` 30 个过；全量 Playwright 157 个：154 过、2 跳过（03-12、03-45 要单独配置），1 个失败是 `wide.spec.ts` 的未读角标（和文件无关，单独重跑该文件 2 遍 40 个全过）；Vitest 301 个过。Android（A28）：tianting-android@d263b6b，预期失败用例改成两条普通断言：`tc_file_07_10_purged_upload_resend_reuploads_from_sandbox`（重发后已发送、新 `fileId`、PUT 多 2 个、对方只收到 1 张、沙盒副本删除）和`tc_file_07_10_purged_upload_without_sandbox_copy_says_source_missing`（`SourceMissing`、界面显示"原文件已不可用，请重新选择"、没有新 PUT）；两条一起跑 3 次过（第一次第二条在 `setup` 的 `resetLimits` 报 `unexpected end of stream`，单独和之后两次都过），`FilesE2eTest` 全类 17 个过（改文案后重跑；之前一次 02-06、02-09 各失败 1 次，单独重跑过），`FilesUiTest` 15 个过，`FilesKillE2e.uploadKilled` 3 次里第 1 次是 A33 的 SnapshotStateObserver 并发崩溃、后 2 次过，单测 41 个过。emulator-5556（Pixel_10_Pro_XL），本地结果，CI 不运行。iOS 不在这次范围
  - **① 处理结果（sdk-swift、iOS，2026-11-09）**：tianting-sdk-swift@662e4f2 记下这台设备 `complete` 成功的 `fileId`（`ownFileIds`，冷启动时本地库里待发的附件也算），`send` 回 4048 且是自己的就和 4096 一样处理；单测 4 个，含"别人的 fileId 不重传"。iOS TC-FILE-07-10（tianting-ios@52c6f2f，`ZzFilesClockE2ETests`）按 v1.4 正常断言：被拉黑得到 4036 的两张图、IM 钟推进 25 小时并跑 `file_purge`，有沙盒副本的那张重发后重新上传（PUT 多 2 次）并送达，冷启动前删掉沙盒的那张提示"原文件已不可用，请重新选择"、只能删除。本地服务端回的是 4048（模拟器，本地；CI 不运行）
  - **③ 处理结果（2026-11-09）**：console 的元数据是白名单结构，不加字段就不透传：tianting-console-server@62941c3 加 `fileSize`（`omitempty`），TC-FILE-91-01 加断言（文件、图片的 `fileSize` 等于原文件字节数，文本没有；仍然没有文件名、`body`），去掉字段时失败；`go test -race -count=1 ./...` 全过（临时 Postgres 17、真实 im-server）。tianting-admin@4b18297：元数据页"正文长度"一列改名"大小"，图片、文件有 `fileSize` 时显示人类可读的大小（1258291 字节显示"1.2 MB"），其他消息和没有 `fileSize` 的（已删除）照旧显示正文长度，不显示文件名；Vitest 111 个过（新增 `metaSize` 2 个、页面 1 个，页面改回正文长度时失败），e2e 59 个：56 过、3 跳过（截图、邀请链路要单独配置），`files.spec.ts` 的元数据页断言"1.2 MB"和接口响应里的 `fileSize`；lint、typecheck、`check:dist` 通过。本地结果
  - **⑤ 文案核对（2026-11-09）**：规则 9 的"等待发送""上传中 N%""图片上传失败，请检查网络后重发""文件上传失败，请检查网络后重发""图片无法显示""加载失败，点击重试""头像已更新"，规则 6 的"一次最多选择 9 个"（整批不发），规则 4 的"原文件已不可用，请重新选择"，H5 和 Android 一致。Android 的"处理中…""上传中…"（还没有进度时）带省略号，改成"处理中""上传中"（tianting-android@d263b6b，同 H5）。**没有改的差异**：Android 文件下载失败显示"下载失败，点击重试"（可以点重试，A32），规则 9 写"下载失败"，H5 只有提示"下载失败"；已有用例断言了现在的文案，改不改由 Frank 定（A34）

### C1 遗留（P-35，Frank 2026-11-09 "都按建议"）
- L1（随 FE-630、FE-631、FE-632）：access 到期等于 `absoluteExpiresAt` 时收到 `kicked/tokenExpired`，SDK 直接走登录失效、**不再发刷新**。三端 TC-ACC-03-45 到期后的断言改回"刷新请求数为 0、不重连"（AC-ACC-03-43），通过后验收报告里 03-43 改为通过。
  - **L1 处理结果（2026-11-09）**：三套 SDK 收到 `kicked/tokenExpired`（或只有关闭码 4011）、本地 token 的到期等于 `absoluteExpiresAt` 时直接按登录失效结束（`sessionExpired`），不发刷新：tianting-sdk-ts@32ba99f、tianting-sdk-kotlin@c4d7fa1、tianting-sdk-swift@2f8501c，三套都有"上限前最后一秒被踢""只有 4011""普通 token 仍刷新 1 次"的用例，去掉实现后前两个失败。三端 TC-ACC-03-45 到期后改回"刷新请求数为 0、不重连"：tianting-web@f7b76a8（`IM_REFRESH_ABSOLUTE=25s`，Chrome，连跑 2 次过，刷新响应 `[]`，1 个 WebSocket）、tianting-android@fd32318（`40s`，emulator-5556，连跑 2 次过，`refresh answers []`，状态只有 CONNECTED → DISCONNECTED）、tianting-ios@bf7374e（`40s`，iPhone 16 Pro，连跑 2 次过）。验收报告 AC-ACC-03-43 改为通过。边界见 D14。本地结果，CI 不运行
- L2（随 FE-635、FE-636 等改到的页面）：Android 组合里直接调 SDK 挂起函数的其余约 30 处，统一改成回主线程再写状态（同 A16、A17 的修法）。
- L3（FE-640 之前）：Android、H5 补真实服务端在线踢线用例（`IM_ACCESS_TTL=30s`，同 iOS 的 03-12）。补不上，AC-ACC-03-12 改回"部分通过"。
  - **L3 处理结果（H5，2026-11-09，tianting-web@47db681）**：`e2e/account-real.spec.ts` 的 TC-ACC-03-12（`IM_ACCESS_TTL=30s npx playwright test -g "TC-ACC-03-12"`，默认服务端上跳过）：登录后服务端按自己的定时器踢 `tokenExpired` → 刷新 200 → 新 WebSocket，不弹窗，之后能发消息；`--repeat-each=2` 连跑 2 次过（Chrome，本地）。第一次跑发现 H5 的问题（H20）：登录拿到的 access 已经不足 6 小时时 SDK 马上刷新，这时标签页锁还没拿到，刷新前的确认失败，本地按"登录已失效"结束；改成确认先等进行中的 claim，Vitest 加了用例（去掉修复时失败）。**Android 的 L3 不在这次范围**，AC-ACC-03-12 的状态没有改
  - **L2 处理结果（Android，2026-11-09，tianting-android@778f042）**：组合里的协程调 SDK / AccountClient 挂起函数的 8 处改成经 `onMain`（群资料快照、群资料、群成员两处、我的群聊、好友申请已读两处、最近登录记录），新页面（图片、文件、头像）也都经 `onMain` 或在 `ImSession` 的主线程 scope 里。`launchCmd` / `accountOp` 里的约 20 处本来就在 `Dispatchers.Main.immediate` 的 scope，没有改。演示登录那一处（`LoginApi` 不是 SDK）试过改成 `onMain`，`NavLoginTest` 4 次里 2 次各 1 个用例超时，撤回后 3 次全过，所以撤回、保持原样（A33）。顺带查清：测试线程上直接 `runBlocking` 会借用 Compose 测试规则外层 `runTest` 的事件循环，在测试线程上执行界面的帧，和主线程绘制并发（"Detected multithreaded access to SnapshotStateObserver"，`FilesKillE2e` 4 次里 2 次）；新用例里对方的调用都放到单独线程（`offThread`），之后连跑 3 次过。已有用例里同样的写法没有改（A33）
  - **L3 处理结果（Android，2026-11-09，tianting-android@778f042）**：`AccountE2eTest.tc_acc_03_12_real_online_kick_refreshes_quietly`（`IM_ACCESS_TTL=30s scripts/run-tests.sh e2e com.tianting.app.AccountE2eTest#tc_acc_03_12_real_online_kick_refreshes_quietly`，默认服务端上跳过）：真实 im-server 按自己的计时器踢 `tokenExpired` → 刷新 200 → 新 WebSocket 打开，不弹窗，连接恢复，存储换成新的 access；事件顺序由包装的 Transport 和 HttpTransport 记录（只记事件名、状态码、关闭码）：`[refresh:req, refresh:200, ws:open, ws:open, kicked:tokenExpired, ws:close:4011, refresh:req, refresh:200, ws:open]`。连跑 2 次过（emulator-5556，本地）。AC-ACC-03-12 的状态本来就是"通过"，没有改，验收报告没有改

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
  3. App 退到后台不保证继续上传（规则 7）。Manifest 通知权限属于 C3，见 FE-652。开发环境 http MinIO 用 debug 例外（F-FILE-01 规则 10）。
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
- 内容：同 FE-635。PHPicker（不要完整相册权限）、`UIDocumentPicker`、ImageIO；HEIC 转 jpeg。`.entitlements` 属于 C3，见 FE-654。
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
  1. 元数据页显示类型和大小，**不显示文件名**（第 14.1 节，R-CS-14）。撤回状态随 C4（FE-671）。
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

## C3 离线推送

> 2026-11-10 分发。依据 PRD 第 7 节、第 13.1、13.2、13.4、15 节的 C3 部分。**H5 没有离线通知**（第 13.2 节），sdk-ts 和 H5 在第 18 节都是 0。
> 测试替身（第 4.5 节）：系统通知授权替身、Google Play 服务可用性替身、系统令牌替身（假令牌字符串），配合服务端的假 APNs、假 FCM（BE-406）。**模拟器上的原生 UI 测试不需要 Frank 的账号**；收到真实系统通知要第 20.3 节的 APNs、FCM 和 P-13 真机，见 FE-656。
> 官方文档核对（backend-chat 文末第 34～46 条）里和客户端有关的：Android 13 起 `POST_NOTIFICATIONS` 是运行时权限、新安装默认关闭、被拒后系统不再自动弹（第 46 条）；Android 用 FCM 通知消息，`tag` 替换由 FCM 代为显示（第 40、43 条）。

### FE-650 sdk-kotlin：设备通知接口、presence、令牌缓存
- 优先级：P0
- 批次：C3
- 仓库：tianting-sdk-kotlin（`sdk-core`，纯 JVM）
- 依赖：BE-450
- AC：AC-CHAT-90-10、AC-APN-01-5、AC-APN-01-10、AC-APN-02-15、AC-APN-03-8、AC-APN-06-1、AC-ACC-04-6、AC-ACC-04-26
- 内容：
  1. 第 13.1 节：`deviceNotificationRegister`、`deviceNotificationUnregister`、`setDeviceNotificationDetail`（发 `deviceNotificationSettings`）、`setAppState`（发 `presence`）。名字避开现有系统通知的 `push`。
  2. F-APN-02 规则 3（R-CF-06）：**`auth` ack 之后第一帧上报当前前后台状态**，重连同样；之后状态变化才发。
  3. F-APN-01 规则 5、5a：授权和令牌由平台层通过接口提供（`NotificationAuthorization`、`PushTokenSource`、`PlayServicesAvailability`，第 4.5 节的替身就是这三个接口的测试实现）。授权关闭或 Play 服务不可用就不注册；每次回到前台对比授权和令牌，开了补注册、关了注销；令牌回调发生在没有连接时先缓存，连上后注册；距上次注册超过 30 天也重新注册一次（backend-chat 第 46 条）。
  4. `sid` 变化（改密、找回、重新登录）后用新连接重新注册（AC-ACC-04-26）；"显示发送者和内容"由服务端复制，SDK 重新读取一次。退出时调用 `SystemTokenRevoker`（FE-611 已留调用点，AC-ACC-04-6 的 C3 部分；有新会话时不撤销的规则见问题 A15）。
  5. `setDeviceNotificationDetail` 在没有令牌时也能调用，返回服务端当前值（AC-APN-03-8）；4290 不自动重试，下次回到前台再试；4030 `featureDisabled` 时隐藏入口（`GET /v1/config` 的 `features.offlinePush`）。
  6. SDK 日志不打印推送令牌（第 13.1 节）。`frames.json` 新命令的执行器（AC-CHAT-90-10）。
- 工作量：3 人日（sdk-kotlin）

### FE-651 sdk-swift：设备通知接口、presence、令牌缓存
- 优先级：P0
- 批次：C3
- 仓库：tianting-sdk-swift
- 依赖：BE-450、FE-650
- AC：AC-CHAT-90-10、AC-APN-01-5、AC-APN-01-10、AC-APN-02-15、AC-APN-03-8、AC-ACC-04-6、AC-ACC-04-26
- 内容：
  1. 语义同 FE-650（第 13.1 节），没有 Play 服务那一项；`env` 由 App 按构建配置传（debug `sandbox`，release `production`）。
  2. iOS 进入后台时那一帧 `presence` 可能发不出去（第 13.2 节、第 21 节第 6 条，未实测）：SDK 尽力发送，服务端靠连接断开和 45 秒窗口兜底；实现时在模拟器上记录实际表现，写进问题表。
  3. 退出时调用 `unregisterSystemPushToken`（FE-612 已留接口，AC-ACC-04-6 的 C3 部分）。
- 工作量：3 人日（sdk-swift）

### FE-652 Android：FCM 接入、授权、令牌、占位配置
- 优先级：P0
- 批次：C3
- 仓库：tianting-android
- 依赖：FE-650
- AC：AC-APN-01-5、AC-APN-01-6、AC-APN-01-10、AC-APN-01-11、AC-APN-01-12、AC-APN-02-15、AC-APN-05-3、AC-APN-06-1、AC-APN-06-2、AC-ACC-04-6
- 内容：
  1. Manifest 加 `POST_NOTIFICATIONS`；**首次登录进入会话列表后才请求授权**，不在首次启动弹。被拒不注册；系统不再弹时只给"前往系统设置"。Android 13 以下也按 `areNotificationsEnabled()` 判断（AC-APN-01-11，API 30 模拟器）。
  2. FCM 客户端（实现时按仓库规矩确认依赖和版本）；`onNewToken` 交给 SDK 缓存。**CI 和开发用占位 `google-services.json`，真实配置只在发布时注入，不进仓库**（R-CQ-21，AC-APN-01-12）；占位配置下 debug 构建和单测通过，拿不到真实令牌时用令牌替身。
  3. F-APN-06：启动时用 Google Play 服务可用性判断，不可用或被禁用就不注册，"设置 → 通知"显示"此设备暂不支持离线通知，打开 App 才能收到消息"；后来变为可用，下次启动注册。
  4. 通知渠道"消息"（F-APN-04 规则 3）；前后台切换调用 `setAppState`。
  5. 收到真实推送（AC-APN-01-6、05-3、06-2）**要 Frank 的 Firebase 项目、真实 `google-services.json`（第 20.3 节）和真机（P-13）**，见 FE-656。
- 工作量：2.5 人日（Android）

### FE-653 Android：通知设置页、点击、清除
- 优先级：P0
- 批次：C3
- 仓库：tianting-android
- 依赖：FE-652
- AC：AC-APN-01-5、AC-APN-03-4、AC-APN-03-5、AC-APN-03-8、AC-APN-04-1、AC-APN-04-2、AC-APN-04-3、AC-APN-04-6、AC-ACC-04-26、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. "设置 → 通知"（第 13.3、13.4 节）：系统授权状态和前往系统设置、"显示发送者和内容"开关；状态：系统授权关闭、令牌获取中、令牌获取失败、设备不支持（F-APN-06）、保存失败。打开开关前先提示"内容会经过第三方通道"，确认后才生效，重启后保留（AC-APN-03-4）。
  2. F-APN-04 规则 1：隐藏模式点通知进会话列表；详情模式走 `tianting://chat/{convId}`（PRD-ui 3.6 节，只接受已知路由），未登录先去登录，账号不同或 `convId` 不属于当前账号就进会话列表，不显示任何内容（AC-APN-04-3）。点通知时登录已失效按 F-ACC-03 刷新，失败回登录页。
  3. 回到前台清除本 App 已显示的通知（AC-APN-04-6）。
  4. AC-CHAT-90-8、90-9 的 C3 部分：通知设置页符合 B-1、B-3，深色模式、无障碍（开关和入口有可读名称）。
- 工作量：2.5 人日（Android）

### FE-654 iOS：APNs 接入、entitlements、授权、令牌
- 优先级：P0
- 批次：C3
- 仓库：tianting-ios
- 依赖：FE-651
- AC：AC-APN-01-5、AC-APN-01-6、AC-APN-01-10、AC-APN-02-15、AC-APN-05-3、AC-ACC-04-6
- 内容：
  1. 加 `.entitlements` 的 `aps-environment`（debug development、release production，F-APN-01 规则 5b）。真实签名和含推送能力的描述文件依赖 Frank 的 Apple Developer 账号（第 20.3 节）；没有之前，模拟器和单测用令牌替身。
  2. 首次登录进入会话列表后请求授权（含 `.badge`）；`registerForRemoteNotifications` 拿到的令牌按十六进制交给 SDK；回到前台对比授权和令牌（F-APN-01 规则 5a）。
  3. 前后台切换调用 `setAppState`；记录进入后台那一帧的实际表现（FE-651 第 2 条）。
  4. 收到真实推送（AC-APN-01-6、05-3）要 APNs Key（第 20.3 节）和真机（P-13），见 FE-656。
- 工作量：2.5 人日（iOS）

### FE-655 iOS：通知设置页、点击、角标、清除
- 优先级：P0
- 批次：C3
- 仓库：tianting-ios
- 依赖：FE-654
- AC：AC-APN-01-5、AC-APN-03-4、AC-APN-03-5、AC-APN-03-8、AC-APN-04-1、AC-APN-04-2、AC-APN-04-3、AC-APN-04-6、AC-ACC-04-26、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-653 第 1、2、4 条。
  2. F-APN-04 规则 2：角标 = 服务端发送时的总未读，App 回到前台时按本地总未读刷新；第一批不保证跨设备已读后清除已送达的通知。
  3. 回到前台清除本 App 已送达的通知（AC-APN-04-6）。
- 工作量：2.5 人日（iOS）

### FE-656 C3 三端验收和真机
- 优先级：P0
- 批次：C3
- 仓库：tianting-android、tianting-ios（tianting-web 只核对没有"通知"入口）
- 依赖：FE-653、FE-655、BE-459
- AC：AC-APN-01-6、AC-APN-03-5、AC-APN-04-1、AC-APN-04-2、AC-APN-05-3、AC-APN-06-2、AC-ACC-04-26、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 按 TC-APN 落地原生 UI 测试（Compose UI Test、XCUITest，模拟器 + 替身 + 真实 im-server 测试模式 + 假通道）。
  2. **手工 AC（AC-APN-01-6、03-5、04-1、04-2、05-3、06-2）依赖 P-13 真机**；其中**收到真实通知的还依赖第 20.3 节**：iOS 要 APNs Key 和带推送能力的签名，Android 要 Firebase 项目和真实 `google-services.json`，服务器要能访问 Apple、Google（AC-APN-05-3）；AC-APN-06-2 要一台没有谷歌服务的国内 Android 真机（不需要 FCM 账号）。未到位记未执行，不算通过。
  3. H5：设置页不出现"通知"（第 13.2 节没有离线通知；AC-UI-16-6 的改写见 BE-460）。
  4. 写验收报告 `docs/reports/c3-frontend-acceptance-<日期>.md`。
- 工作量：2.5 人日（测试）

## C4 撤回、引用、@

> 2026-11-10 分发（Frank"推送后续做，先做功能"，QUEUE Q28；C3 暂停）。依据 PRD 第 8 节、第 13.1～13.4、14.1、15 节的 C4 部分。服务端任务见 [backend-chat](backend-chat.md) BE-470～BE-481，后台接口见 [console-chat](console-chat.md) CS-202、CS-204。
> **不依赖 C3**：F-RCL-03 规则 5"已显示的系统通知按 F-APN-03 规则 4"在服务端（BE-481，等 C3）；客户端本批不碰通知。
> 现状（2026-11-10 调查，各仓 HEAD：sdk-ts `f417580`、sdk-kotlin `2819ce6`、sdk-swift `c8ec6ff`、tianting-web `12ee72e`、tianting-android `79893a7`、tianting-ios `7f1b997`、tianting-admin `4b18297`；三套 SDK 的 `protocol/SOURCE` 是 `e71a977`）：七个仓都**没有** `recall`、`recalled`、`quote`、`mentions`、`mentionUnread`、`firstMentionSeq`；C2 已经做了单调合并（`deleted`、`expired`）和按 `modVer` 重拉（失败的会话跳过、其余照拉，P-39 ②），C4 在这上面加 `recalled` 和 `quote`。三端草稿都是内存里的纯文本；消息菜单只有"复制"和失败消息的"重发""删除"；H5 消息菜单没有键盘入口；三端都没有新消息播报（P-39 ①）。
> 文案：撤回按 F-RCL-02 规则 1a 的表（聊天页和列表摘要相同）；引用条按 F-QUO-02 规则 3；4097 按 `details.windowSeconds` 选"发送时间超过 2 分钟，无法撤回"或"…7 天…"。

### FE-660 sdk-ts：撤回、引用、提及
- 优先级：P0
- 批次：C4
- 仓库：tianting-sdk-ts
- 依赖：BE-470、BE-478
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-02-4、AC-RCL-03-1、AC-RCL-03-3、AC-RCL-03-6、AC-RCL-03-10、AC-RCL-03-11、AC-RCL-03-13、AC-QUO-01-1、AC-QUO-02-2、AC-QUO-02-6、AC-MEN-01-1、AC-MEN-02-5、AC-MEN-03-7、AC-CHAT-90-7、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 模型（第 11.1 节）：`src/protocol.ts` 的 `MsgBody`（第 37～43 行）加 `quote`、`mentions`；`ServerMessage`（第 50～62 行）、`LastMsg`（第 64～72 行）加 `recalled`、`recalledAt`、`recalledBy`；`ServerConversation`（第 75～95 行）加 `mentionUnread`、`firstMentionSeq`。`src/store.ts` 的 `Message`（第 20～38 行）、`Conversation`（第 41～57 行）同步。
  2. 接口（第 13.1 节）：`recall(convId, msgId)`；`send(convId, text, opts?: {quote?, mentions?})`（`client.ts` 第 414 行现在固定 `{text}`，按 `sendImage` 第 441 行的写法加第三个参数，R-CF-26）；`groupMembers` 的 `keyword`（`loadGroupMembers` 第 1649 行旁边加搜索接口）；`canRecall(msg, myRole)` 按 `ts` 和 `auth` ack 校正后的服务端时间判断（AC-RCL-01-8，不用设备时钟）。4293、4097 不自动重试；4097 给出 `windowSeconds`。
  3. **合并规则**（F-RCL-03 规则 2a，`addServer` 第 127～155 行）：`recalled` 只能从假变真；`quote.state` 只能从 `normal` 往后走，`quote.text` 清除后不恢复；`deletedIds` 继续优先；`markDeleted`（第 162～173 行）、`deletedLast`（第 79～81 行）、`toView`（第 371～374 行）、`applyServerMessage`（第 418～434 行）带上 `recalled`、`recalledBy`。
  4. `msgDeleted` 带 `recalled`（`onMsgDeleted` 第 981～996 行）：清除消息原文（`wipe` 第 175～180 行）、**清除本地所有引用这条消息的 `quote.text`、状态改 `recalled`**（F-QUO-02 规则 4）、通过 `filesExpired` 让页面丢掉缓存的 `blob:`；本地 `mentionUnread > 0` 且收到 `msgDeleted` 或 `readUpdated` 时节流 5 秒重新拉 `convs`（F-MEN-03 规则 1，R-CF-18）。
  5. 撤回文案的判断数据（单聊 / 群、操作者是不是自己、被撤回者是不是自己、操作者显示名）由 SDK 给出结构，页面套 F-RCL-02 规则 1a 的表；列表摘要同一张表（AC-RCL-02-4、03-6、03-10）。
  6. 重连：已有的按 `modVer` 重拉（`recheckDeleted` 第 864～880 行）覆盖撤回（AC-RCL-03-3、03-11），不用改逻辑，补用例。
  7. 共享向量：同步 `protocol/` 副本到 BE-478 之后的提交，跑 `chat-sync.json` 的撤回、引用、提及用例（AC-CHAT-90-15，`test/chat-sync-vectors.test.ts`）和 `frames.json` 新帧的 Schema 校验（AC-CHAT-90-10，`test/frames-schema.test.ts`）。
  8. AC-CHAT-90-7（老版本 SDK）：用 C4 之前的 sdk-ts（`f417580`）跑新帧：不崩溃，撤回显示"该消息已被删除"，内存搜不到特征串（TC-CHAT-90-26）。
  9. 日志不打印引用文字、`mentions` 的昵称文本（第 13.1 节）。
- 工作量：3 人日（sdk-ts：撤回 1、引用 1、@ 1）

### FE-661 sdk-kotlin：撤回、引用、本地库
- 优先级：P0
- 批次：C4
- 仓库：tianting-sdk-kotlin（`sdk-core`）
- 依赖：FE-660
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-03-2、AC-RCL-03-3、AC-RCL-03-4、AC-RCL-03-10、AC-RCL-03-11、AC-RCL-03-13、AC-QUO-01-1、AC-QUO-02-2、AC-QUO-02-6、AC-CHAT-90-7、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-660 第 1～6 条。模型：`Protocol.kt` 的 `MsgBody`（第 95～108 行）、`ServerMessage`（第 111～123 行）、`LastMsg`（第 126～133 行）、`ServerConversation`（第 164 行起）；`Store.kt` 的 `Message`（第 70～87 行）、`Conversation`（第 91～105 行）。`send(convId, text)`（`IMClient.kt` 第 444 行）加默认参数 `quote`、`mentions`；`recall`。
  2. 合并：`addServer`（第 194～240 行）、`markDeleted`（第 254～266 行）、`wipe`（第 268～275 行）带 `recalled` 和 `quote` 的单调规则；**`DeletedRefs`（`LocalDatabase.kt` 第 26～33 行）加 `recalled`**（`deleted_refs` 的 `kind` 增加一种，本地库版本 +1，原地升级，旧行保留，有用例）。
  3. **本地库和缓存**（AC-RCL-03-2、03-4）：撤回时 `wipeMessage`（第 249～252 行）覆盖原文；引用这条消息的其他行的 `quote.text` 同样覆盖；缩略图和原图缓存走已有的 `forgetFiles`（`IMClient.kt` 第 923～931 行）。SQLite 的空闲页可能留原文：撤回后对本地库做 `secure_delete` 或 `VACUUM`（和 sdk-swift 的做法对齐，实现时选一个并在用例里搜数据库文件）。冷启动时从 `deleted_refs` 恢复撤回状态（`restoreRefs` 第 361～366 行）。
  4. 下载中的文件被撤回：取消下载、删掉半截文件，抛出"已撤回"事件给 App（AC-RCL-03-12 的 SDK 部分，界面在 FE-667）。
  5. 共享向量和 `frames.json`（`ChatSyncVectorsTest.kt`、`VectorsTest.kt`）；AC-CHAT-90-7 用 `2819ce6` 跑新帧，本地库搜不到特征串。
  6. `sdk-android` 的 `AndroidLocalDatabase.kt` 这次调查没细看，实现时核对升级路径和 JVM 一致。
- 工作量：3 人日（sdk-kotlin：撤回 1.5、引用 1.5）

### FE-662 sdk-kotlin：提及
- 优先级：P0
- 批次：C4
- 仓库：tianting-sdk-kotlin（`sdk-core`）
- 依赖：FE-661
- AC：AC-MEN-01-1、AC-MEN-02-5、AC-MEN-03-7
- 内容：
  1. `mentionUnread`、`firstMentionSeq` 进 `Conversation` 和本地库；节流 5 秒重新拉 `convs`（同 FE-660 第 4 条，[SDK 钟]）。
  2. `groupMembers` 的 `keyword` 搜索接口（`loadGroupMembers` 第 1349 行旁边），4290 不自动重试。
  3. 发送带 `mentions`：4293 标记失败、不自动重发（AC-MEN-02-5 的 SDK 部分），4131 同样。
  4. `chat-sync.json` 的提及计数用例。
- 工作量：1.5 人日（sdk-kotlin，@ 行）

### FE-663 sdk-swift：撤回、引用、本地库
- 优先级：P0
- 批次：C4
- 仓库：tianting-sdk-swift
- 依赖：FE-660
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-03-2、AC-RCL-03-3、AC-RCL-03-4、AC-RCL-03-10、AC-RCL-03-11、AC-RCL-03-13、AC-QUO-01-1、AC-QUO-02-2、AC-QUO-02-6、AC-CHAT-90-7、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-661。模型：`Protocol.swift` 的 `MsgBody`（第 107 行）、`ServerMessage`（第 145 行）、`LastMsg`（第 159 行）、`ServerConversation`（第 179 行）；`Store.swift` 的 `Message`（第 43～63 行）、`Conversation`（第 66～82 行）。`send(_:_:)`（`Client.swift` 第 408 行）加 `quote:`、`mentions:` 默认参数；`recall`。
  2. 合并：`addServer`（第 163～205 行）、`markDeleted`（第 239～247 行）、`wipe`（第 250 行起）带 `recalled`、`quote`（第 21 节第 9 条：Swift Store 合并"C4 实现时"逐行读）。**先核实 `deletedIds` 跨冷启动有没有单独持久化**（现在删除的行以 `deleted=true`、空 body 写回 `messages` 表，`LocalDatabase.swift` 第 113 行；问题 17）。
  3. 本地库：已有"有清除的行时 `VACUUM`"（第 210～214 行），撤回走同一路径；缓存走 `forgetFile`（`Client+Files.swift` 第 470～476 行）。下载中被撤回：取消 Task，删半截文件（AC-RCL-03-12 的 SDK 部分）。
  4. 共享向量（`ChatSyncVectorTests.swift`）、`frames.json`（`VectorTests.swift`、`SchemaValidator.swift`）；AC-CHAT-90-7 用 `c8ec6ff` 跑新帧。
- 工作量：3 人日（sdk-swift：撤回 1.5、引用 1.5）

### FE-664 sdk-swift：提及
- 优先级：P0
- 批次：C4
- 仓库：tianting-sdk-swift
- 依赖：FE-663
- AC：AC-MEN-01-1、AC-MEN-02-5、AC-MEN-03-7
- 内容：同 FE-662（`loadGroupMembers` 在 `Client+Social.swift` 第 339 行）。
- 工作量：1.5 人日（sdk-swift，@ 行）

### FE-665 H5：撤回和引用
- 优先级：P0
- 批次：C4
- 仓库：tianting-web
- 依赖：FE-660
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-02-4、AC-RCL-02-5、AC-RCL-03-3、AC-RCL-03-6、AC-RCL-03-8、AC-RCL-03-10、AC-QUO-01-5、AC-QUO-02-3、AC-QUO-02-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. **消息菜单**（F-UI-09 扩展，第 13.3 节）：`ChatPage.tsx` 第 401～447 行的菜单按状态显示"复制""引用""撤回"（"举报"随 C6）；`features.msgActions` 关时只有"复制"（AC-UI-09-1 的改写，BE-479）；撤回、删除的消息和提示行没有菜单（规则 6，第 549 行 `hasMenu`）。"撤回"按 `canRecall`：自己的 2 分钟内；群主 7 天内任何人的；管理员 7 天内别人的（4030 提示"你没有权限撤回这条消息"，AC-RCL-02-5）；菜单打开期间过了时限，点了按 4097 提示（AC-RCL-01-8）。
  2. **键盘入口**（第 13.4 节，现在只有 pointer 和 contextmenu，第 610～624 行）：气泡可聚焦（`tabIndex`），菜单键和 Shift+F10 打开，方向键选、Esc 关（AC-CHAT-90-9 的键盘走查）。
  3. 撤回显示：按 F-RCL-02 规则 1a 的表（聊天页提示行、`ChatsPage.tsx` 第 161～169 行的列表摘要），不再显示"该消息已被删除"（`lib/errors.ts` 第 74 行保留给后台删除）；全屏看的图片被撤回时退出全屏并提示"该消息已被撤回"（`Attach.tsx` 第 445 行 `ImageViewer`，AC-RCL-03-8）。
  4. **引用条**：长按 / 右键选"引用"，输入框上方显示引用条（可以取消），发送后气泡顶部显示引用；四种状态的文案（F-QUO-02 规则 3）；引用摘要和文件名**按纯文本渲染**（AC-QUO-02-5）。点击跳转：距已加载范围 200 条以内向上翻页后滚动并高亮，更远提示"消息太早，请翻页查看"，拉不到提示"消息不存在"（AC-QUO-02-3）。
  5. 新增颜色走 token，深色模式；图标进 PRD-ui 6.6 节清单（AC-CHAT-90-8、90-9，TC-CHAT-90-30、31）。撤回的新消息播报按 P-39 ① 不在本批（问题 14）。
- 工作量：3 人日（H5：撤回 1.5、引用 1.5）

### FE-666 H5：@ 成员和提及提醒
- 优先级：P0
- 批次：C4
- 仓库：tianting-web
- 依赖：FE-665
- AC：AC-MEN-01-1、AC-MEN-01-5、AC-MEN-01-6、AC-MEN-01-7、AC-MEN-01-8、AC-MEN-01-9、AC-MEN-01-10、AC-MEN-01-12、AC-MEN-02-4、AC-MEN-02-5、AC-MEN-03-5、AC-MEN-03-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. **输入模型**（F-MEN-01 规则 5，R-CF-16）：`Composer.tsx` 从纯文本改成"文本 + `[{start, len, userId}]`"；行首或空白后输入 `@`、`＠` 才打开选择（`a@b.com` 不触发）；**组字中（`compositionstart` 到 `compositionend`）不改模型、不打开**；光标不落进 mention 内部，退格一次删整段并移除对应 `mentions`；粘贴的 `@xxx` 是普通文字；单聊不打开（AC-MEN-01-6）。
  2. 插入文本用 `@{公开昵称} `，没有昵称用 `userId`，**不用备注**（R-CF-03）；选择列表显示备注、头像、`userId` 区分同名（问题 18）。选择列表：`keyword` 搜索、加载中、无结果、加载失败（可以继续发普通文本，第 13.4 节）；只有群主、管理员看到"所有人"（AC-MEN-02-4）。
  3. **结构化草稿**（`im/drafts.ts` 现在是 `Map<convId, string>`）：存 `{text, mentions, quote}`，离开会话再回来文字、`mentions`、引用条都恢复（AC-MEN-01-10）；仍只在内存（问题 15）。
  4. 提醒：列表摘要前缀优先级"[有人@我]" > "[草稿]"（`ChatsPage.tsx` 第 223～230 行，"[N 条]"随 C5）；被 @ 的消息整体样式提示；打开会话时 `firstMentionSeq` 不在视口显示"有人@我"按钮，点击滚动并高亮后消失（AC-MEN-03-6）。4293 提示"@所有人次数已用完"，消息失败，不自动重发（AC-MEN-02-5）。
  5. AC-MEN-01-9（手工）：macOS 拼音输入法录屏，同 AC-UI-10-2 的写法。
- 工作量：2 人日（H5，@ 行）

### FE-667 Android：撤回和引用
- 优先级：P0
- 批次：C4
- 仓库：tianting-android
- 依赖：FE-661
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-02-4、AC-RCL-02-5、AC-RCL-03-3、AC-RCL-03-4、AC-RCL-03-6、AC-RCL-03-8、AC-RCL-03-10、AC-RCL-03-12、AC-QUO-01-5、AC-QUO-02-3、AC-QUO-02-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 菜单同 FE-665 第 1 条：`ChatPage.kt` 的 `BubbleMenu`（第 619～652 行）加"引用""撤回"，`combinedClickable`（第 589～595 行）对已发送的文本、图片、文件都启用长按；`liveMessage`（第 343 行）已过滤删除的。
  2. 撤回显示：第 385 行 `CenterTip(MESSAGE_DELETED)` 分出撤回文案；`ChatsPage.kt` 第 184～190 行列表摘要；全屏查看（`ui/pages/Attachments.kt` 第 185 行 `ImageViewer`）被撤回时关闭并提示（AC-RCL-03-8）；**下载中的文件被撤回**：`core/Attachments.kt` 第 196 行 `cancelDownload`，提示"该消息已被撤回"，不留部分文件（AC-RCL-03-12）；冷启动后本地库里的原文已清（AC-RCL-03-4，`AccountKillE2e` 那套真实杀进程的写法）。
  3. 引用条同 FE-665 第 4 条（Compose `Text` 不解析 HTML，AC-QUO-02-5 照样跑）。
  4. 可读名称、深色、token；新消息播报按 P-39 ①（问题 14）。
- 工作量：3.5 人日（Android：撤回 2、引用 1.5）

### FE-668 Android：@ 成员和提及提醒
- 优先级：P0
- 批次：C4
- 仓库：tianting-android
- 依赖：FE-667、FE-662
- AC：AC-MEN-01-1、AC-MEN-01-5、AC-MEN-01-6、AC-MEN-01-7、AC-MEN-01-8、AC-MEN-01-9、AC-MEN-01-10、AC-MEN-01-12、AC-MEN-02-4、AC-MEN-02-5、AC-MEN-03-5、AC-MEN-03-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-666。`Composer`（第 677 行起，`BasicTextField` 第 764 行）改用 `TextFieldValue` + mention 区间；组字中（`composition` 不为空）不改模型；退格整段删除。
  2. 草稿：`core/ImSession.kt` 第 98 行 `drafts` 改成结构化（内存）。成员选择用 `GroupPages.kt` 的成员状态加 `keyword` 搜索。
  3. 列表前缀 `ChatsPage.kt` 第 229～233 行；"有人@我"按钮。
  4. AC-MEN-01-9（手工）：系统中文输入法，模拟器 + 真机（P-13）。
- 工作量：2.5 人日（Android，@ 行）

### FE-669 iOS：撤回和引用
- 优先级：P0
- 批次：C4
- 仓库：tianting-ios
- 依赖：FE-663
- AC：AC-RCL-01-1、AC-RCL-01-8、AC-RCL-02-4、AC-RCL-02-5、AC-RCL-03-3、AC-RCL-03-4、AC-RCL-03-6、AC-RCL-03-8、AC-RCL-03-10、AC-RCL-03-12、AC-QUO-01-5、AC-QUO-02-3、AC-QUO-02-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-667。`ChatPage.swift` 的 `.contextMenu`（第 652、673 行，`menu` 第 685～698 行，`hasMenu` 第 554 行）；撤回显示 `messageRow` 第 337～338 行；列表摘要 `Core/ChatLogic.swift` 第 80～89 行；全屏 `Attachments.swift` 第 267 行（`fullScreenCover` 第 194 行）；下载取消 `Model/MediaOps.swift` 第 231 行。
  2. 引用条、纯文本显示；可读名称、深色、token；新消息播报按 P-39 ①。
- 工作量：3.5 人日（iOS：撤回 2、引用 1.5）

### FE-670 iOS：@ 成员和提及提醒
- 优先级：P0
- 批次：C4
- 仓库：tianting-ios
- 依赖：FE-669、FE-664
- AC：AC-MEN-01-1、AC-MEN-01-5、AC-MEN-01-6、AC-MEN-01-7、AC-MEN-01-8、AC-MEN-01-9、AC-MEN-01-10、AC-MEN-01-12、AC-MEN-02-4、AC-MEN-02-5、AC-MEN-03-5、AC-MEN-03-6、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-666。`UI/Pages/Composer.swift` 的 `ComposerTextView`（第 30 行，UIKit）：用 `markedTextRange` 判断组字，mention 区间用属性文本标记，退格整段删除。
  2. 草稿 `Core/Drafts.swift` 改结构化（内存，`AppModel.swift` 第 38 行）；成员选择 `GroupPages.swift` 第 40 行 `GroupMembersModel` 加 `keyword`；列表前缀 `ChatsPage.swift` 第 164～167 行（读屏文本第 176～185 行同步）。
  3. AC-MEN-01-9（手工）：系统中文输入法，模拟器 + 真机（P-13）。
- 工作量：2.5 人日（iOS，@ 行）

### FE-671 管理后台：撤回状态、引用和提及、经代理查看文件
- 优先级：P0
- 批次：C4
- 仓库：tianting-admin
- 依赖：CS-204、CS-202
- AC：AC-RCL-04-3、AC-RCL-04-7、AC-RCL-04-9
- 内容：
  1. 元数据页（FE-639 第 1 条留的）：`api/types.ts` 的 `MessageMeta` 加 `recalled*`；`pages/Meta.tsx` 第 170～174 行旁边显示"已撤回（撤回人，时间）"，和"已删除"分开；没有文件名、引用文字（AC-RCL-04-7）。
  2. 正文页 `pages/MessageBody.tsx`：撤回的显示"该消息已被撤回（撤回人，时间）"、没有正文（第 254～257 行旁边），不能勾选删除以外的操作照旧；隔离模式（第 263～266 行）显示撤回原文；正文里的 `quote` 显示"引用：{摘要}"（纯文本）、`mentions` 显示被 @ 的 userId 列表。
  3. **经代理查看文件**（FE-639 第 2 条和 P-37 ④ 挪来的）：隔离模式下，撤回或后台删除的图片、文件有"下载查看"按钮，走 CS-202 的代理，浏览器按 `attachment` 下载，页面不内联渲染，后台 CSP 不放宽；没有授权弹"工单号 + 理由 + 动态码"（4039）。
- 工作量：1 人日（console + 后台页面列：撤回 0.5、@ 0.5）

### FE-672 C4 三端验收
- 优先级：P0
- 批次：C4
- 仓库：tianting-web、tianting-android、tianting-ios、三套 SDK
- 依赖：FE-666、FE-668、FE-670、FE-671、BE-480
- AC：AC-RCL-01-1、AC-RCL-02-4、AC-RCL-03-3、AC-QUO-01-1、AC-MEN-01-1、AC-MEN-01-9、AC-CHAT-90-7、AC-CHAT-90-8、AC-CHAT-90-9、AC-CHAT-90-15
- 内容：
  1. 按 TC-RCL、TC-QUO、TC-MEN 落地三端用例（Playwright、Compose UI Test、XCUITest，真实 im-server 测试模式，`IM_MSG_ACTIONS_ENABLED=1`）；端到端 AC（AC-RCL-01-1、02-4、03-3，AC-QUO-01-1，AC-MEN-01-1）三端互发互收，各留结果。
  2. 【全部】的 C4 部分：AC-CHAT-90-8、90-9（菜单和引用条、@ 选择、"有人@我"按钮，TC-CHAT-90-30、31）；90-15（TC-CHAT-90-33）；AC-CHAT-90-7（老版本 SDK，TC-CHAT-90-26）。
  3. 手工 AC-MEN-01-9 三端系统中文输入法，真机部分挡在 P-13；未到位记未执行。
  4. 写验收报告 `docs/reports/c4-frontend-acceptance-<日期>.md`，写明 AC-RCL-03-9、AC-APN-03-3 挡在 C3（P-43 ②），新消息播报按 P-39 ① 是已知缺口。
- 工作量：3.5 人日（测试：撤回 1.5、引用 0.5、@ 1.5）

---

## C5 会话置顶、免打扰、删除

> 2026-11-10 分发（Frank 2026-11-10"开始 C5"，QUEUE Q29）。依据 PRD 第 9 节 F-CVS-01～04，第 13.1～13.4、15 节的 C5 部分。服务端任务见 [backend-chat](backend-chat.md) BE-482～BE-490，后台见 [console-chat](console-chat.md) CS-205（管理后台页面没有改动）。
> **不依赖 C3**：免打扰对推送的影响、App 图标角标都在服务端和 C3（BE-490，第 60 条）；客户端本批只做 App 内的显示。AC-CVS-02-6（前台不响铃）留在本批（第 58 条）。
> 现状（2026-11-10 调查，各仓 HEAD：sdk-ts `7739c55`、sdk-kotlin `d6603f5`、sdk-swift `c97d08e`、tianting-web `3cd6e96`、tianting-android `def3170`、tianting-ios `ee522bc`）：七个仓都**没有** `pinned`、`notifyOff`、`convSet`、`convDelete`、`deleted`；排序只按 `updatedAt`（sdk-ts `store.ts` 第 474～477 行）；总未读是全部 `unread` 之和（sdk-ts `client.ts` 第 1551 行，sdk-kotlin `IMClient.kt` 第 2267 行，sdk-swift `Client.swift` 第 589 行）；`convs` 里消失的群一律推断为"你已不在该群"（sdk-ts `client.ts` 第 1303、1320 行，sdk-kotlin `IMClient.kt` 第 2040 行，sdk-swift `Client.swift` 第 1016 行）；`minSeq` 只用来算补拉起点，不清本地（F-CVS-03 规则 6 ②）。三端会话项没有长按、右键菜单；Android、iOS 没有任何系统通知、提示音代码。
> 文案：确认框"删除后聊天记录不再显示，确定删除？"；4220"最多置顶 20 个会话"；免打扰摘要前缀"[N 条]"，优先级"[有人@我]" > "[草稿]" > "[N 条]"（F-MEN-03 规则 2）。

### FE-680 sdk-ts：会话设置、删除、多端同步
- 优先级：P0
- 批次：C5
- 仓库：tianting-sdk-ts
- 依赖：BE-482、BE-487
- AC：AC-CVS-03-2、AC-CVS-03-5、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 模型（第 11.1 节）：`src/protocol.ts` 的 `ServerConversation` 加 `pinned`、`pinnedAt`、`notifyOff`，`convs` ack 加 `deleted`、`deletedTruncated`，事件 `convSettingsChanged`、`convDeleted`；`src/store.ts` 的 `Conversation` 同步。
  2. 接口（第 13.1 节）：`setConvPinned(convId, pinned)`、`setConvNotifyOff(convId, on)`、`deleteConv(convId)`（`upToSeq` 取本地看到的最大 seq）。**乐观更新**：先改本地，失败（5000、4220、4290、4030）回滚并把错误交给页面，不自动重试（第 56 条）；ack 的 `{conv}` 覆盖本地。
  3. 排序（F-CVS-01 规则 3）：`conversations()` 改成置顶在前（`pinnedAt` 倒序），其余 `updatedAt` 倒序，同时间 `convId`；稳定引用（G-UI-19）不变。
  4. 总未读（F-CVS-02 规则 4）：`totalUnread()` = 普通会话 `unread` 之和 + 免打扰会话 `mentionUnread` 之和；`totalUnread` 事件同口径。
  5. **`convs` 的 `deleted`**（F-CVS-03 规则 6）：`setConversations` 算出的"消失的会话"先查 `deleted`，在里面的从列表移除，**不发 `convRemoved`、不生成 `removed` 快照**（AC-CVS-03-5）；不在里面的群照旧推断；`truncated` 或 `deletedTruncated` 为 true 时不推断（第 54 条）。对 `deleted` 里的和列表里的每个会话：清除 `seq < minSeq` 的消息（**保留下来的消息里引用被清除消息的摘要照常显示**，P-46 ④、第 62 条；只有本端还没发出、引用了被清除消息的才清除原文），通过 `filesExpired` 让页面丢掉缓存的 `blob:`（sdk-ts 只有内存，D4）。
  6. 事件：`convSettingsChanged` 覆盖本地的 `pinned*`、`notifyOff`（AC-CVS-04-1）；`convDeleted`：`minSeq = clearedSeq + 1`、`readSeq = max(readSeq, clearedSeq)`、不再置顶、保留免打扰，按第 5 条清除，从列表移除，并发一个事件让页面清草稿（AC-CVS-04-5，第 55 条）。之后来了 seq > `clearedSeq` 的消息照常出现（AC-CVS-03-2）；重连靠 `convs`（AC-CVS-04-2）。
  7. 共享向量：同步 `protocol/` 副本到 BE-487 之后的提交，跑 `chat-sync.json` 的置顶、`deleted`、清除、总未读用例（AC-CHAT-90-15，`test/chat-sync-vectors.test.ts`）和 `frames.json` 新帧的 Schema 校验（AC-CHAT-90-10，`test/frames-schema.test.ts`）。
- 工作量：1.5 人日（sdk-ts）

### FE-681 sdk-kotlin：会话设置、删除、本地库物理清除
- 优先级：P0
- 批次：C5
- 仓库：tianting-sdk-kotlin（`sdk-core`、`sdk-android`）
- 依赖：FE-680
- AC：AC-CVS-03-2、AC-CVS-03-5、AC-CVS-03-10、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-680 第 1～6 条。模型在 `Protocol.kt`、`Store.kt`；`setConversations`（`Store.kt` 第 493 行，调用处 `IMClient.kt` 第 2040 行）；总未读（`IMClient.kt` 第 2267 行，现在是 `sumOf { it.unread }`）。
  2. **本地库物理清除**（F-CVS-03 规则 6 ②，AC-CVS-03-10，R-CB-02）：处理 `convs` 和 `convDeleted` 时对任何会话删除 `seq < minSeq` 的消息行和引用文字，缓存目录删掉对应的 `fileId + variant` 文件（已有的 `forgetFiles`）；SQLite 空闲页按 C4 的做法处理（同 FE-661 第 3 条）；用例对本地库文件（含 `-wal`）和缓存目录做字节搜索。本地清除失败下次启动再清（F-CVS-03 异常）：本地库记"待清除"（版本 +1，原地升级，有用例）。
  3. 置顶、免打扰进本地库，冷启动先显示，`convs` 回来以服务端为准（F-CVS-04 规则 2）。
  4. `sdk-android` 的 `AndroidLocalDatabase.kt` 走同样的升级和清除路径。
  5. 共享向量和 `frames.json`（`ChatSyncVectorsTest.kt`、`VectorsTest.kt`）。
- 工作量：2.5 人日（sdk-kotlin）

### FE-682 sdk-swift：会话设置、删除、本地库物理清除
- 优先级：P0
- 批次：C5
- 仓库：tianting-sdk-swift
- 依赖：FE-680
- AC：AC-CVS-03-2、AC-CVS-03-5、AC-CVS-03-10、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 语义同 FE-681。`setConversations`（`Store.swift` 第 556 行，调用处 `Client.swift` 第 1016 行）、`totalUnread()`（`Client.swift` 第 589 行）；`setConvPinned(_:_:)` 等用 `async throws`。
  2. 本地库：已有"有清除的行时 `VACUUM`"的路径，物理清除走同一路径；缓存走 `forgetFile`；"待清除"标记同 FE-681 第 2 条。
  3. 共享向量（`ChatSyncVectorTests.swift`）、`frames.json`（`VectorTests.swift`、`SchemaValidator.swift`）。
- 工作量：2.5 人日（sdk-swift）

### FE-683 H5：会话菜单、置顶和免打扰显示、删除、总未读
- 优先级：P0
- 批次：C5
- 仓库：tianting-web
- 依赖：FE-680
- AC：AC-CVS-01-6、AC-CVS-02-2、AC-CVS-02-3、AC-CVS-02-4、AC-CVS-02-5、AC-CVS-03-2、AC-CVS-03-8、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. **会话菜单**（F-UI-04 规则 9 改写，第 13.3 节）：`pages/ChatsPage.tsx` 的会话项窄屏长按、宽屏右键、键盘（会话项可聚焦，菜单键或 Shift+F10 打开，方向键选，Esc 关，同 C4 消息菜单的做法）；菜单"置顶 / 取消置顶""免打扰 / 取消免打扰""删除"，文字按当前状态；`features.convSettings` 关时没有菜单（AC-CVS-01-6）。4220 提示"最多置顶 20 个会话"，4290 提示"操作过于频繁"，其他失败按 SDK 回滚后提示。
  2. 置顶项背景区分（新 token，深色模式，第 57 条）；免打扰：未读显示小红点（无数字），摘要前"[N 条]"，右侧免打扰图标（有可读名称，AC-CVS-02-3）；前缀优先级和 C4 的"[有人@我]""[草稿]"合并（第 61 条）。
  3. 标签角标和 `document.title`（`App.tsx` 第 266 行）按 SDK 的 `totalUnread`：AC-CVS-02-2 显示 5，02-4 标题"天听"，02-5 显示 3。
  4. 删除：确认框；确认后清除草稿（`im/drafts.ts`）、失败消息、内存里的 `blob:`、这个会话正在上传和排队的附件（第 59 条）；取消时都不变（AC-CVS-03-8）。从"我的群聊"进入已删除的群显示空的聊天页，可以发言（规则 4）。收到 `convDeleted` 时同样清这个标签页的草稿。
  5. 多端：另一个标签页（独立连接）或设备的改动 2 秒内反映到列表（AC-CVS-04-1、04-5）；重连后和服务端一致（AC-CVS-04-2）；删除后对方再发，会话重新出现、只有新消息（AC-CVS-03-2）。
  6. 新增图标进 PRD-ui 6.6 节清单、颜色走 token（AC-CHAT-90-8、90-9，TC-CHAT-90-36、37）。
- 工作量：2 人日（H5）

### FE-684 Android：会话菜单、置顶和免打扰显示、删除、总未读
- 优先级：P0
- 批次：C5
- 仓库：tianting-android
- 依赖：FE-681
- AC：AC-CVS-01-6、AC-CVS-02-2、AC-CVS-02-3、AC-CVS-02-5、AC-CVS-02-6、AC-CVS-03-2、AC-CVS-03-8、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-683 第 1、2、4、5 条：`ui/pages/ChatsPage.kt` 的会话项加长按（`combinedClickable`）和菜单（`DropdownMenu`），辅助操作（`customActions`，第 13.4 节"长按菜单要有辅助操作"）；"消息"标签角标在 `ui/pages/Common.kt` 第 64～65 行，读 `ImSession.totalUnread`（SDK 新口径）。
  2. 删除：确认框；草稿（`core/ImSession.kt` 的 `drafts`）、失败消息、缓存文件（`core/Attachments.kt`）、正在上传和排队的附件都清除；AC-CVS-03-10 的界面部分（冷启动后只显示新消息，本地库搜不到特征串，`AccountKillE2e` 那套真实杀进程的写法）。
  3. **AC-CVS-02-6**：App 前台收到免打扰会话的消息，列表更新，不响铃、不弹通知。现在 App 没有通知和提示音代码，用例断言 `NotificationManager.getActiveNotifications()` 为空、提示音替身没有调用；C3 加了通知后 FE-653 补跑（第 58 条）。
  4. 可读名称、深色、token。
- 工作量：3 人日（Android）

### FE-685 iOS：会话菜单、置顶和免打扰显示、删除、总未读
- 优先级：P0
- 批次：C5
- 仓库：tianting-ios
- 依赖：FE-682
- AC：AC-CVS-01-6、AC-CVS-02-2、AC-CVS-02-3、AC-CVS-02-5、AC-CVS-02-6、AC-CVS-03-2、AC-CVS-03-8、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-684。`Kit/Sources/TiantingAppKit/UI/Pages/ChatsPage.swift` 的会话项加 `.contextMenu` 和 `accessibilityAction`；"消息"标签角标在 `UI/RootView.swift` 第 179 行，`Model/IMStore.swift` 第 55 行取 SDK 的 `totalUnread()`。
  2. 删除：草稿（`Core/Drafts.swift`）、失败消息、缓存、上传都清除；AC-CVS-03-10 的界面部分。
  3. **AC-CVS-02-6**：断言 `UNUserNotificationCenter` 已送达的通知为空、提示音替身没有调用；C3 加了通知后 FE-655 补跑。**App 图标角标 C5 不设**（随 C3，第 60 条）。
- 工作量：3 人日（iOS）

### FE-686 C5 三端验收
- 优先级：P0
- 批次：C5
- 仓库：tianting-web、tianting-android、tianting-ios、三套 SDK
- 依赖：FE-683、FE-684、FE-685、BE-489
- AC：AC-CVS-01-6、AC-CVS-03-2、AC-CVS-03-5、AC-CVS-03-10、AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CHAT-90-8、AC-CHAT-90-9、AC-CHAT-90-15
- 内容：
  1. 按 TC-CVS 落地三端用例（Playwright、Compose UI Test、XCUITest，真实 im-server 测试模式，`IM_CONV_SETTINGS_ENABLED=1`；涉及 @ 的再开 `IM_MSG_ACTIONS_ENABLED=1`）；端到端 AC（AC-CVS-03-2、04-1、04-2、04-5）三端轮换 D1、D2 和对方，各留结果。
  2. 【全部】的 C5 部分：AC-CHAT-90-8、90-9（会话菜单、置顶背景、免打扰图标、删除确认框，TC-CHAT-90-36、37）；90-15（TC-CHAT-90-39）。
  3. 写验收报告 `docs/reports/c5-frontend-acceptance-<日期>.md`，写明推送相关的 AC 挡在 C3（BE-490），AC-CVS-02-6 C3 加通知后补跑，新消息播报按 P-39 ① 仍是已知缺口。
- 工作量：1.5 人日（测试）

---

## C6 敏感词和举报

> 2026-11-11 分发（Frank 2026-11-11"开始 C6"，PENDING P-47，QUEUE Q30）。依据 PRD 第 10 节 F-MOD-01～04，第 13.1～13.4、14.1、15 节的 C6 部分。服务端任务见 [backend-chat](backend-chat.md) BE-491～BE-499，console 见 [console-chat](console-chat.md) CS-206～CS-208。
> **不依赖 C3**（backend-chat 第 94 条）。开发和测试只用特征词（`违规词`、`badword`、`ZXQWORD`、`ZXQ-*`），**任何仓库、用例、截图里都不放真实词库**（F-MOD-01 规则 5）。
> 现状（2026-11-11 调查，各仓 HEAD：sdk-ts `c9b3127`、sdk-kotlin `ae12932`、sdk-swift `7137fdf`、tianting-web `145d0fe`、tianting-android `b81c592`、tianting-ios `b80cd93`、tianting-admin `aa47b1f`）：七个仓都**没有** `report`、4510、`features.moderation`；消息菜单的代码里都留了"举报随 C6"的注释（sdk-ts `client.ts` 第 298 行，tianting-web `pages/ChatPage.tsx` 第 66 行，tianting-android `ui/pages/ChatPage.kt` 第 218 行，tianting-ios `Core/ChatLogic.swift` 第 122 行）；管理后台没有词库页、举报页（`src/pages/` 现有 21 个页面），C4 的经代理看文件只有 `attachment` 下载（FE-671）。
> 文案（PRD 原文）：4510"消息包含不当内容，无法发送"；提交前"所选内容将提交给平台审核"；成功"已提交，感谢你的反馈"；4292"今天的举报次数已用完"；后台"单聊正文查看未开启""证据无法解密"。PRD 没写的文案按第 78、82、83 条。

### FE-690 sdk-ts：report、4510、features.moderation
- 优先级：P0
- 批次：C6
- 仓库：tianting-sdk-ts
- 依赖：BE-491
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-CHAT-90-10
- 内容：
  1. 模型（第 11.1 节）：`src/protocol.ts` 加 `ReportReq`（三种 `targetType`）和 ack `{reportId}`；错误枚举加 4510、4098（`client` scope 一致性，F-SYS-01）；`GET /v1/config` 的 `features.moderation`（老服务端没有按 false）。
  2. 接口（第 13.1 节）：`report({targetType, convId?, msgIds?, userId?, groupId?, reason, note?})` → `reportId`；参数在本地先校验（`note` ≤ 200 码点、`msgIds` 1～20），不合法直接抛 4000 不发帧；4292、4040、4030 原样交给页面，**不自动重试**。第一批页面只举报单条，接口保留数组（R-CF-21）。
  3. **4510**（F-MOD-02 规则 6、AC-MOD-02-9）：`send`、`sendImage`、`sendFile` 收到 4510 时消息标记失败、**不自动重发**，错误里不含任何词（服务端本来就不给）；失败原因可以让页面区分 4510（第 81 条：4510 的失败消息不提供"重发"）。账号接口（注册、改昵称）的 4510 原样抛出。
  4. 共享向量：同步 `protocol/` 副本到 BE-491 之后的提交，跑 `frames.json` 的 `report` 新帧和 4510 错误 ack（AC-CHAT-90-10，`test/frames-schema.test.ts`）。C6 没有 `chat-sync.json` 用例（第 15 节 AC-CHAT-90-15 的 C6 列是"—"）。
  5. SDK 日志不打印 `note`。
- 工作量：1 人日（sdk-ts）

### FE-691 sdk-kotlin：report、4510、features.moderation
- 优先级：P0
- 批次：C6
- 仓库：tianting-sdk-kotlin（`sdk-core`）
- 依赖：FE-690
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-CHAT-90-10
- 内容：
  1. 语义同 FE-690。`Protocol.kt` 加帧和错误码；`IMClient.report(...)`（`suspend`）；4510 的失败消息进本地库时带失败原因，冷启动后仍不提供"重发"。
  2. `frames.json`（`VectorsTest.kt`）。
- 工作量：1 人日（sdk-kotlin）

### FE-692 sdk-swift：report、4510、features.moderation
- 优先级：P0
- 批次：C6
- 仓库：tianting-sdk-swift
- 依赖：FE-690
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-CHAT-90-10
- 内容：
  1. 语义同 FE-691。`report(...)` 用 `async throws`；本地库的失败原因同 FE-691。
  2. `frames.json`（`VectorTests.swift`、`SchemaValidator.swift`）。
- 工作量：1 人日（sdk-swift）

### FE-693 H5：举报入口和举报单、4510 的提示
- 优先级：P0
- 批次：C6
- 仓库：tianting-web
- 依赖：FE-690
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-MOD-03-11、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. **入口**（F-MOD-03 规则 8，第 13.3 节，`features.moderation` 关时都没有，第 76 条）：消息菜单"举报"（按状态显示，显示条件见第 77 条，`pages/ChatPage.tsx` 第 66 行旁边）；撤回提示行"举报"（别人撤回的、本机看过的，AC-MOD-03-11）；资料卡、群信息页"举报"。
  2. **举报单**：理由单选（6 项，文案见第 78 条）、补充说明（可选，0～200 码点，计数显示）、提示"所选内容将提交给平台审核"；提交中禁用按钮；成功轻提示"已提交，感谢你的反馈"；**单聊消息和用户举报成功后询问是否拉黑**（第 79 条）；4292"今天的举报次数已用完"；4040 按第 83 条；网络错误可以重试（手动）。
  3. **4510**（AC-MOD-02-9）：消息在列表标记失败，**输入框清空**（和其他发送失败一致，可以从气泡复制），提示"消息包含不当内容，无法发送"，不自动重发，失败气泡没有"重发"（第 81 条）；注册、改昵称、建群、改群名和公告、好友申请的 4510 按第 82 条的文案提示，表单内容保留可修改。
  4. 无障碍和视觉：新增的"举报"图标进 PRD-ui 6.6 节清单（第 80 条），颜色走 token；举报单可以只用键盘完成（Tab、方向键选理由、Esc 关闭），菜单的键盘入口沿用 C4（AC-CHAT-90-8、90-9，TC-CHAT-90-44、45）。
- 工作量：1.5 人日（H5）

### FE-694 Android：举报入口和举报单、4510 的提示
- 优先级：P0
- 批次：C6
- 仓库：tianting-android
- 依赖：FE-691
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-MOD-03-11、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-693：消息菜单（`ui/pages/ChatPage.kt` 第 218 行旁边）、撤回提示行、资料卡、群信息页的"举报"；举报单用底部弹层（理由单选、补充说明、提示）；成功后单聊询问拉黑；4510 和其他场景的提示。
  2. 辅助操作：消息的长按菜单已有辅助操作（C4），撤回提示行的"举报"要有可读名称；深色、token。
- 工作量：2 人日（Android）

### FE-695 iOS：举报入口和举报单、4510 的提示
- 优先级：P0
- 批次：C6
- 仓库：tianting-ios
- 依赖：FE-692
- AC：AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-MOD-03-11、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 同 FE-694：`Core/ChatLogic.swift` 第 122 行的菜单项加"举报"；举报单用 sheet；撤回提示行、资料卡、群信息页的入口；`accessibilityAction`。
  2. 4510 和其他场景的提示；深色、token。
- 工作量：2 人日（iOS）

### FE-696 管理后台：词库页和命中统计
- 优先级：P0
- 批次：C6
- 仓库：tianting-admin
- 依赖：CS-206
- AC：AC-MOD-01-7
- 内容：
  1. 新页面"词库"（第 14.1 节，菜单和路由按 `moderation.words` 守卫，同 FE-401 的做法）：列表（原文、状态、更新时间），搜索，单个添加，停用 / 启用，删除（确认框）；**内容一律按纯文本渲染**（不用 `dangerouslySetInnerHTML`，lint 规则已有）。
  2. 批量导入：选择 txt 文件（UTF-8，一行一个），浏览器里拆成数组，超过 5000 行直接提示"一次最多 5000 条"不提交；提交后显示结果汇总：新增、重复、无效（AC-MOD-01-7，第 86 条）。
  3. 命中统计：按用户 ID 查次数（按场景）和最近记录，**只对有 `report.read` 的显示**（console-chat 第 27 条，要 Frank 定；定之前按这个做）。
  4. 页面不缓存词列表到本地存储；截图用例只用特征词。
- 工作量：1 人日（console + 后台页面列）

### FE-697 管理后台：举报队列、详情、证据、结案
- 优先级：P0
- 批次：C6
- 仓库：tianting-admin
- 依赖：CS-207、CS-208、FE-671
- AC：AC-MOD-03-1、AC-MOD-03-11、AC-MOD-04-1、AC-MOD-04-6、AC-MOD-04-12、AC-MOD-04-15
- 内容：
  1. 新页面"举报"（`report.read` 守卫）：按状态（待处理、已处理、已驳回）、理由、时间筛选；**相同对象聚合显示"N 次"**，点开看每条（AC-MOD-04-6，聚合口径见 backend-chat 第 117 条）；被举报内容、昵称、理由一律纯文本渲染。
  2. 详情：元数据（举报人、被举报人、时间、理由、消息类型和长度、该用户被举报的总次数）；单聊举报打开前弹理由框（每天 50 个，429 / 4292 的提示）；**没有授权时不显示 `note` 和文件名**。
  3. 证据：群消息"查看证据"→ 弹框（理由 + 当次动态码，工单号自动填举报 ID 不可改）→ 正文、隔离原文（撤回的，AC-MOD-03-11）、补充说明、文件名，**平铺水印**（同 FE-421：账号名 + 时间到分钟 + 授权前 8 位）；**图片同时显示缩略图和原图**，经 console 同源加载；其他文件只有"下载"（`attachment`），html 也是下载（AC-MOD-04-12）；文件名含 `<script>` 按纯文本显示（AC-MOD-04-15）；生产 CSP 下没有违规（Playwright 监听 `securitypolicyviolation`）；单聊显示"单聊正文查看未开启"，只能看元数据和被举报次数；`undecryptable` 显示"证据无法解密"，仍可结案；任何请求 4039 弹同一个授权框。
  4. 结案：结论（已处理 / 驳回）+ 理由；4098 提示"举报已被处理"并刷新；页面上有跳转到已有操作的链接（删除消息、禁言、封禁、强制下线、解散群），**不自动执行**（F-MOD-04 规则 5）。AC-MOD-04-1 的完整流程（读证据 → 删除消息 → 结案）随 FE-698。
- 工作量：2 人日（console + 后台页面列）

### FE-698 C6 三端和管理后台验收
- 优先级：P0
- 批次：C6
- 仓库：tianting-web、tianting-android、tianting-ios、tianting-admin、三套 SDK
- 依赖：FE-693、FE-694、FE-695、FE-696、FE-697、BE-499
- AC：AC-MOD-01-7、AC-MOD-02-9、AC-MOD-03-1、AC-MOD-03-9、AC-MOD-03-11、AC-MOD-04-1、AC-MOD-04-6、AC-MOD-04-12、AC-MOD-04-15、AC-CHAT-90-8、AC-CHAT-90-9
- 内容：
  1. 按 [TC-MOD](../testcases/TC-MOD.md) 落地三端用例（Playwright、Compose UI Test、XCUITest，真实 im-server 测试模式，`IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=1`、测试里现场生成的 `IM_REPORT_KEY`；涉及撤回的再开 `IM_MSG_ACTIONS_ENABLED=1`）和管理后台用例（Playwright，真实 console-server + im-server + Postgres，生产 CSP 响应头）。
  2. 端到端 AC：AC-MOD-03-1（三端各举报一次，后台队列出现，被举报人收不到帧）、AC-MOD-04-1（审核员在后台走完查看、授权、读证据、删除、结案，被删除的消息在三端消失）。
  3. 【全部】的 C6 部分：AC-CHAT-90-8、90-9（举报入口、举报单、后台两个新页面，TC-CHAT-90-44、45）。
  4. 写验收报告 `docs/reports/c6-frontend-acceptance-<日期>.md`：本地结果、CI 不运行；真机和 WebKit 未执行；新消息播报按 P-39 ① 仍是已知缺口；第 82 条文案等待决定时的实现。
- 工作量：2.5 人日（测试）

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
| A15 | A1 的修复，F-ACC-04 规则 1 | 旧 logout 失败时本来要撤销系统推送令牌，但新会话已经在用这个令牌 | 有新会话时不撤销，改成立刻补发旧 sid。补发成功之前，旧 sid 的推送还可能到这台设备（系统令牌接入随 C3，真实推送没有验证；C3 里由 FE-650、FE-651 第 4 条和 TC-APN-91-03 覆盖） |
| F2 | F-ACC-03 规则 5，AC-ACC-03-43（server，FE-620 发现） | 到绝对上限前不足 1 秒，服务端已经踢 `tokenExpired`（JWT 的 `exp` 只到秒），刷新还能返回 200，于是有几次多余的刷新和重连，随后到点回登录页。`IM_REFRESH_ABSOLUTE=25s` 实测 2 次 | 没有改：`model.go` 规定 `kicked` 可以刷新，服务端要不要在上限的同一秒不再签发由 Frank 决定。三端用例只断言"上限前没有刷新"。**Frank 决定（2026-11-09，都按建议）**：修服务端：距上限不足 1 秒的刷新按到期处理（PRD-chat v1.2 `IM_REFRESH_ABSOLUTE`，TC-ACC-03-51）；三端 03-45 用例改回"到期后也不刷新"。**处理结果（2026-11-09，tianting-ios@54caa87、tianting-android@d50c230、tianting-web@f0c5b6f）**：三端改回检查到期之后：上限前 0 次刷新，到期后没有成功的刷新、不重连。三端实测服务端在上限那一秒踢 `tokenExpired` 后 SDK 仍发 1 次刷新，服务端回 401 / 4019，用例断言"最多这 1 次且被拒绝"，不是"请求数为 0"。要 0 次需要改三套 SDK，待 Frank 决定（验收报告第 10 节），AC-ACC-03-43 仍是部分通过 |
| A16 | tianting-android `ChatTest`（FE-620 发现，不是 C1 范围） | 整类连跑 7 次失败 2 次：`menu_copy_retry_delete` 删除后 5 秒内消息还在；`at_bottom_follows_new_message` 在聊天页消息列表的 Compose `LazyColumn` 测量时 `IndexOutOfBoundsException`（区间列表为空却取第 0 项）。单独跑都通过 | 根因没有查明，没有改。后一个是 App 进程里的崩溃，真实用户可能遇到，建议单独排查。**Frank 决定（2026-11-09，都按建议）**：C2 开工前先查清并修复。**处理结果（2026-11-09，tianting-android@d50c230）**：崩溃的根因查明：聊天页在组合的上下文里收集 SDK 线程更新的 `messagesFlow`，Compose 测试的调度器不切线程，重组和 LazyColumn 测量跑在 `tianting-sdk` 线程上，和主线程并发（现场栈：`CalledFromWrongThreadException ... Calling: tianting-sdk`，随后 "performMeasureAndLayout called during measure layout"）。改成在主线程收集，聊天页的 SDK 挂起调用经 `onMain` 回主线程；新用例 `sdk_thread_updates_reach_ui_on_main_thread` 修前失败（34～35 次通知在 SDK 线程）、修后通过。正式运行的调度器会切回主线程，按代码判断真实用户碰不到。`menu_copy_retry_delete` 按代码判断是等待条件（重发后旧的失败图标先满足等待，删除时消息其实在发送中），改成等 SDK 里真的又失败；这一轮没有复现，根因未经现场证实。另外修了软键盘动画中点"发送"落空的等待条件。ChatTest 整类连跑 10 次全过（模拟器，本地） |
| A17 | tianting-android `ConvListTest.start_chat_self_and_friends_mode_warning`（FE-620 发现） | 约 1/3 失败：已经跳转到聊天页（路径对、对方是好友），5 秒内没有 `chat-title` | 根因没有查明。试过重复点击、先等好友关系，都没用，已撤销。**处理结果（2026-11-09，tianting-android@d50c230）**：和 A16 同一原因：发起单聊弹窗里 `relationOf` 在 SDK 线程上恢复后直接导航。改成经 `onMain`。ConvListTest 连跑 6 次全过（之前约 1/3 失败）；6 次不足以证明，只能说没有再出现 |
| A18 | tianting-android `ServerE2eTest.live_notice_banner_list_and_no_token_or_data_in_logcat`（tianting-android@8757053） | 用例读 `logcat -d -b all` 时测试进程 OutOfMemoryError（模拟器开得久，缓冲区大） | 开头先 `logcat -b all -c`。**这条用例现在只检查它自己运行期间的日志**，之前其他用例写进 Logcat 的 token 不再被它顺带发现；`AccountE2eTest.secrets_never_reach_logcat` 同样只看自己清空之后的日志。**Frank 决定（2026-11-09，都按建议）**：接受，改成只读本用例进程的日志（不再整体清空）。**处理结果（2026-11-09，tianting-android@d50c230）**：不再清空 Logcat。`scanOwnLogcat` 读 `logcat -d -b all --pid=<本进程> -T <用例开始>`，逐行查，不整段读进内存；断言不变，另加"读到了日志"。两条用例都过，没有 OOM（模拟器，本地） |
| I10 | tianting-ios 踢线弹窗和 securityNotice 提醒（FE-620 截图） | 改密时提醒先到、踢线后到；弹窗从底部出，和提醒在同一位置，弹窗出来后提醒被盖住，用户可能看不到"你的账号密码已被修改" | 没有改。"确定"能点（XCUITest 断言）。要不要调整位置由 Frank 决定。**Frank 决定（2026-11-09，都按建议）**：把提醒移到弹窗之上。**处理结果（2026-11-09，tianting-ios@54caa87）**：踢线弹窗出现时由弹窗把提醒画在遮罩和卡片之上、屏幕上方，不接收点击；根视图的提醒层这时不显示。用例断言提醒在、和卡片不重叠、"确定"可点，浅色、深色截图确认（模拟器，本地） |

C2 三套 SDK（FE-630～632）实现时发现的。PRD 冻结，没有改；按"现在的实现"一列做了，**要 Frank 决定的标了"待决定"**。结果都是本地的，CI 不运行。

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| D1 | F-RCL-03 规则 2（server，已修） | 规则 2 写"撤回、后台删除、文件过期时递增 `mod_ver`"，服务端现在只在文件过期时递增（`sqlite_files.go`），后台删除（`DeleteMessages`）不递增。SDK 按任务单改成只重拉 `modVer` 变了的会话后，**断线期间被后台删除的、不是最后一条的消息，重连后不再被清掉**（原来每次重连全部重拉能清掉；最后一条仍靠 `convs` 的 `lastMsg.deleted`）。后台删除要 `IM_V13_ADMIN_FEATURES=1`，默认关闭 | 三套 SDK 都按规则 2 做了；`convs` 没有 `modVer`（老服务端）时仍全部重拉。建议 BE 在 `DeleteMessages` 同一事务递增 `mod_ver`（撤回随 C4 本来就要递增），这是后端改动，本次没有做。**2026-11-09 已修（照 PRD 补实现，不是新决定）**：`DeleteMessages` 同一事务先递增 `conv_seq.mod_ver`（已删除的重复删除不再递增）；TC-FILE-06-12 的 e2e 加了断言（删的不是最后一条，modVer +1；重复删除不变），去掉修复时该用例失败 |
| D2 | AC-FILE-07-10 H5 一侧、F-FILE-07 规则 4 | AC 写 H5 同样场景（页面没刷新）提示"原文件已不可用"；规则 4 说"有沙盒副本的回到上传中"，源文件没了的例子是"H5 刷新后"。sdk-ts 在内存里留着 File 直到发出，没刷新时 4096 会重新上传；刷新后整条消息已经不在（D4，AC-FILE-07-3），所以 `sourceMissing` 在 H5 上实际碰不到 | sdk-ts 有源就重新上传，没有源时 `error='sourceMissing'`。H5 页面怎么对 AC 由 FE-633、FE-634 定。**P-37 ②（2026-11-09）已定**：H5 未刷新时重新上传后成功，提示只要求原生 |
| D3 | F-FILE-07 规则 3（按 PRD 字面统一） | "待发送的按现有规则自动发 1 次"：三套 SDK 的"现有规则"不同。sdk-kotlin 冷启动时 QUEUED、SENDING 一律转失败（原有规则，不在用户不知情时发旧消息），所以冷启动后的待发送附件也转失败；sdk-swift 冷启动时待发送的、以及已有 `fileId` 的发送中，按排队自动发 1 次（服务端按 `clientMsgId` 去重）。同一进程里断线重连三套都自动发 1 次 | 各自按原有规则，没有统一。**2026-11-09 处理**：PRD 规则 3 明写"待发送的……自动发 1 次"，第 820 行也说现有 QUEUED 重连后自动发，所以按字面统一成 Swift 的做法：sdk-kotlin 冷启动时**待发送的附件**（以及已有 `fileId` 的发送中）自动发 1 次，其他 QUEUED、SENDING 的原有规则不动。随 FE-635 做 |
| D4 | F-FILE-07 规则 3 | 冷启动时处理中、上传中"转为失败"没有给原因 | sdk-kotlin 记 `SendError.Network`，sdk-swift 记 `.interrupted`（sdk-ts 只在内存，没有这个场景）。名字不同，App 文案都是"发送失败"一类，三端页面时再统一 |
| D5 | F-FILE-07、第 13.2 节 | 上传的 PUT 没有规定超时 | 三套 SDK 都没有 SDK 层的超时（OkHttp 读写各 60 秒没有进展才失败，URLSession 每段 60 秒、整体 3600 秒，fetch / XHR 没有）；卡住时一直显示"上传中"，用户可以取消 |
| D6 | F-FILE-07 规则 2（sdk-kotlin） | 规则 2 只说退出登录时清除沙盒副本；被踢时 sdk-kotlin 本来就清本地库 | 被踢时沙盒副本和下载缓存一起删，之后重发失败的附件得到 `SourceMissing` |
| D7 | F-FILE-07 规则 3（原生） | 处理中被杀进程：App 的处理函数只在内存里，冷启动后没有源 | 这条消息转失败，重发得到 `SourceMissing`（"原文件已不可用，请重新选择"）。已经复制进沙盒的（上传中被杀）不受影响，重发从头上传 |
| D8 | F-FILE-07 异常（sdk-swift） | 复制进沙盒失败只写了"存储空间不足"；App 给的源文件已经不存在也会复制失败 | 同样记 `.storage`，文案对不上，页面任务再定 |
| D9 | 第 21 节 16（sdk-swift，接着 S2） | 实测 URLSession 对没有响应的 GET 会自己重发（2 次 GET 到达 4 次）；PUT 每次用新的会话，断在半路只到达 1 次 | 下载不阻止重发：只读、同一个预签名地址、只有 2xx 才写文件。只在 macOS、回环、HTTP/1.1 测过 |
| D10 | BE-403、第 13.1 节共享用例 | `protocol/vectors/chat-sync.json` 还没有（BE-403 负责） | **已处理（2026-11-10，BE-403，tianting-server@6c0829f，本地结果）**：新增 `chat-sync.json`（6 个，单调合并和 `modVer` 重拉），`account-session.json` 加附件状态机 `attachmentCases`（11 个），`frames.json` 加 image、file、MsgBody 互斥 29 个。三套 SDK 同步副本并加执行器（sdk-ts `chat-sync-vectors.test.ts`、`attachment-vectors.test.ts`；sdk-kotlin `ChatSyncVectorsTest`、`AttachmentVectorsTest`；sdk-swift `ChatSyncVectorTests`、`AttachmentVectorTests`），全部通过，没有发现 SDK bug。AC-CHAT-90-10、90-15 在 C2 验收报告里改为通过。一个会话重拉失败时其余会话拉不拉，见 backend-chat.md 问题 32（要 Frank 定） |
| D11 | F-FILE-01 规则 6a | 只说用户取消时发 `DELETE`；上传失败（PUT 失败、complete 报错）后留下的 `pending` 要等 PUT 地址过期才释放名额 | 三套 SDK 在没完成的上传失败时也尽力发一次 `DELETE`（幂等，失败只记日志），重发时从头申请新的 `fileId` |
| D12 | F-FILE-07、F-FILE-02 规则 2 | 失败原因里只有部分有 PRD 文案（4292、`uploadPendingLimit`、处理失败、存储空间不足、原文件不可用），网络或对象存储失败没有 | SDK 给出 `error`（错误码，或 `network`、`process`、`storage`、`sourceMissing` 等）和 `errorReason`，文案由页面任务定。**P-37 ⑤ 已定**：文案写进 PRD v1.4 F-FILE-07 规则 9 |
| D13 | tianting-web Vitest（FE-633、FE-634） | 协议副本同步后 `errors.json` 的 4096 是 `ws` 通道，H5 的错误文案表没有 4096，用例"errors.json 里每个 ws 通道的错误码都有文案"失败 1 个（Vitest 260 个，259 过） | 没有改 H5 页面（不在这次范围），FE-633 补文案（候选是 PRD 的"原文件已不可用，请重新选择"） **已处理**：tianting-web@b3b2a03，4096 用"原文件已不可用，请重新选择"（SDK 有源就重新上传，走到界面的只有没有源的情况），没有改用例 |
| D14 | P-35 L1 的边界 | L1 只看本地 token 的到期是否等于 `absoluteExpiresAt`，不看时间：最后一段 access（被上限截短的那个）有效期内，如果收到的 `kicked/tokenExpired` 不是到期而是被替换的代（不是自己刷新造成的），也直接回登录页 | 按决定实现。这种情况下刷新最多也只能换来同一个上限的 token；自己刷新造成的被替换仍按"本地更新"用本地值。三套 SDK 都有"普通 token 被踢仍刷新 1 次"的用例 |

C2 H5 和后台页面（FE-633、FE-634、FE-639）实现时发现的。PRD 冻结，没有改；按"现在的实现"一列做了。结果都是本地的（Chrome），CI 不运行。

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| H13 | F-ACC-06、AC-ACC-06-6（sdk-ts） | `AccountClient.updateProfile` 只把新昵称同步进 `IMClient` 的资料缓存，头像地址（服务端写的公开桶地址）要等 10 分钟缓存过期或刷新页面才拿到；`PATCH profile` 的响应也没有地址 | H5 这次登录里用上传的那张（`blob:`）显示自己的头像（"我"页、编辑页、自己的消息）；刷新后用服务端地址。SDK 没有改，建议 SDK 改资料后重新拉自己的资料。另：H5 的头像只显示 https 地址，测试环境的假 S3 是 http，所以刷新后在测试环境不显示，生产要求 https（`config.go` 第 535 行） |
| H14 | AC-FILE-07-10 H5 一侧（接着 D2） | AC 写 H5"同样场景"提示"原文件已不可用"；页面没刷新时 File 还在内存里，按规则 4"有源的回到上传中"重新上传；刷新后整条消息不在了（AC-FILE-07-3），所以这句提示在 H5 上实际碰不到 | 按规则 4：有源就重新上传（和原生一样）。SDK 报 `sourceMissing` 时显示"原文件已不可用，请重新选择"，点失败图标只能删除（Vitest 只测了文案，界面状态造不出来）。AC-FILE-07-10 的 H5 部分要 Frank 定口径。**P-37 ② 已定**：AC-FILE-07-10 H5 一侧改为重新上传后成功（PRD v1.4）；用例已改（tianting-web@bdf0cd8） |
| H15 | F-FILE-07 规则 4、AC-FILE-07-10（**server**） | 实测（tianting-web `zz-im-clock.spec.ts` 的 TC-FILE-07-10）：待发送的图片（`complete` 过，`send` 没有 ack），IM 钟推进 25 小时、跑 `file_purge` 之后重发，`send` 回 **4048**，不是规则 4 写的 4096。`file_purge` 把超过 24 小时没绑定的文件整行删掉，`precheckFile`（`logic/files.go` 第 36～39 行）找不到行就回 4048。SDK 只对 4096 重新上传，4048 直接失败，再重发还是 4048，消息只能删除 | 没有改服务端和 SDK（不在范围）。用例按 PRD 断言 4096 并重新上传成功，标成预期失败（`test.fail`），日志里只有错误码 `[4048]`。建议服务端对"自己的、已清理的上传"回 4096，或 PRD 改口径，由 Frank 定。**P-37 ① 已定**：SDK 对自己上传的 `fileId` 收到 4048 也按已清理处理，服务端不改（PRD v1.4 F-FILE-07 规则 4）；落实见"P-37 落实"。**已落实**（tianting-sdk-ts@4f4312d，tianting-web@bdf0cd8：用例去掉 `test.fail`，重新上传后发送成功） |
| H16 | F-FILE-07 规则 6 | "一次最多选 9 个"：多选了怎么办没写 | 提示"一次最多选择 9 个"，这一批都不发（不替用户挑前 9 个）。文案是实现时加的。**P-37 ⑤ 已定**（PRD v1.4 F-FILE-07 规则 6） |
| H17 | F-FILE-05 规则 3 | "每个会话首次"提示来源：记在哪里没写 | 只在内存（按会话），刷新或重新登录后再提示一次（偏保守）；自己发的文件和图片不提示。**P-37 ⑤ 已定**（PRD v1.4 F-FILE-05 规则 3） |
| H18 | AC-FILE-02-4、TC-FILE-02-04 | 用例要求 exiftool 读 EXIF、环境 ES（MinIO 复跑）。本机没有 exiftool，不装软件；不跑 MinIO | 用假 S3 上的最终对象检查：JPEG 没有 APP1 段、搜不到 `Exif`、型号特征串；方向用像素验证（Orientation=6 的 40×20 左红右蓝，转正后 20×40 上红下蓝）。exiftool 和 MinIO 复跑没有执行 |
| H19 | AC-FILE-07-7 | "查当天额度"：客户端没有查额度的接口 | 弱网用 Playwright 让 PUT 连接重置 5 次：5 次申请、5 次 `DELETE`、0 次 `complete`。额度只在 `complete` 成功时扣（F-FILE-01 规则 6，服务端 TC-FILE-01-11 验过），所以按这个推断没有扣，不是直接读额度 |
| H20 | F-ACC-03 规则 9（H5，L3 发现，**已修**） | 登录（或改密、找回）换了 `sid` 之后，标签页锁的 claim 是异步的；如果拿到的 access 已经不足 6 小时（`IM_ACCESS_TTL` 配得短，或本机时钟快了 18 小时以上），SDK 马上刷新，刷新前的确认在 claim 完成之前执行，返回"没有占着"，本地按 4019 `revoked` 结束：一登录就弹"登录已失效" | tianting-web@47db681：确认先等进行中的 claim（`locksGuard`、`broadcastGuard` 都改了），Vitest 加了用例（交锁延迟 5 毫秒，去掉修复时失败）。默认 24 小时的 access 碰不到 |
| H21 | NF-UI-14、第 13.2 节（`deploy/nginx.conf`） | nginx 的 `add_header` 只在当前层没有 `add_header` 时继承上一层；示例里 `location /` 有自己的 `Cache-Control`，所以 `index.html` 的响应没有 CSP、`Referrer-Policy` 等头（meta 里有 CSP，`frame-ancestors` 和 `Referrer-Policy` 只能靠头，页面里有 `<meta name="referrer">`） | 安全头放进 `deploy/tianting-headers.conf`，每个带 `add_header` 的 location 都 include；Vitest 检查每个带 `add_header` 的 location 都 include 了，CSP 和构建的一致。没有在真实 nginx 上跑 |
| H22 | 第 13.4 节（H5） | "下载中""保存权限被拒""存储空间不足"在 H5 没有对应：浏览器下载没有进度（第 13.2 节），保存走浏览器 | 不显示这些状态。文案是实现时加的："图片无法显示"（解码失败、超像素上限）、"加载失败，点击重试"、"下载失败"、"等待发送""处理中""上传中 N%"、"头像已更新"、头像的错误提示、网络失败"图片上传失败，请检查网络后重发"/"文件上传失败，请检查网络后重发"（D12）。**P-37 ⑤ 已定**（PRD v1.4 F-FILE-07 规则 9） |
| H23 | AC-FILE-07-10（H5 用例，P-37 落实时发现） | 原来的用例在 IM 钟推进 25 小时后才重发：重新上传要调 `/v1/files/*`，演示登录的 token 固定 24 小时、不能刷新（PRD C-21），回 4011，消息停在失败；改用账号登录也一样（H5 的 refresh 也是 24 小时）。页面一直开着时 SDK 会按时刷新，用例碰不到这一步 | IM 钟只在跑 `file_purge` 时推进，重发前拨回（同 Android A28 的写法）。试过账号登录 + Playwright 页面时钟快进 19 小时让 SDK 自己刷新，也能过，但改动大，没有采用 |

C2 Android（FE-635、FE-636）和 sdk-kotlin（D3、H13）实现时发现的。PRD 冻结，没有改；按"现在的实现"一列做了。结果都是本地结果（模拟器 emulator-5556，Pixel_10_Pro_XL，API 37；CI 不运行，真机 P-13 未执行）

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| A19 | 第 13.2 节"开发环境 http MinIO 用 debug 例外"（F-FILE-01 规则 10） | 预签名地址的主机就是 `IM_OBJSTORE_ENDPOINT`，服务端自己也用它访问对象存储；模拟器访问 Mac 用 10.0.2.2，Mac 上没有这个地址，两边用不了同一个主机名 | 测试服务端的假 S3 监听 `127.0.0.1:18283`，`run-tests.sh` 用 `adb reverse`（只对 `ANDROID_SERIAL`）把模拟器的同一端口转到 Mac；debug 的网络安全配置只多放行 `127.0.0.1` 的明文（release、logcheck 不变） |
| A20 | AC-FILE-03-5"下载到一半断网" | 模拟器经 `adb reverse` 连假 S3，飞行模式断不掉这条连接（走 adbd，不走模拟器网络） | 由假 S3 断开正在传的 GET（`/__ctl?cut_gets=1`）代替断网；断网时选图（AC-FILE-07-6）的上传请求走 10.0.2.2，飞行模式有效，用的是真实飞行模式 |
| A21 | F-FILE-02 规则 2、AC-FILE-02-13 | 处理后的原图只降质量（90→40）不缩小（同 H5），长边 4096 内纯随机噪点的大图（实测 3000×3000 随机 RGB）在 q40 仍超过 20 MiB，提示"无法处理这张图片，请用文件方式发送"。PRD 没说压不下去时要不要再缩小 | 没有改，和 H5 一致。用例用像照片的内容（渐变 + 小幅噪点，6000×4000，png 源文件超过 20 MiB）验证"按压缩后计算"，成功 |
| A22 | F-FILE-02 规则 2 的格式矩阵（Android 列"系统"） | png（透明）、webp 的输出写的是"png / webp（保留透明）"，没说不透明的 webp 输出什么 | 有透明通道的输出 png，其他（含不透明 webp、HEIC）输出 jpeg；GIF 原样。API 28+ 用 `ImageDecoder`（实测按 EXIF 方向转正：Orientation=6 的 40×20 变成 20×40，上红下蓝），API 28 以下用 `BitmapFactory` + `ExifInterface` 自己转，**没有在 API 28 以下的设备上跑**；HEIC 没有样本，**未验证**（AC-FILE-02-10 是手工 + 真机） |
| A23 | F-FILE-02 规则 9、第 13.4 节"保存权限被拒" | API 29 起存相册用 MediaStore 不要权限，只有 API 28 及以下要 `WRITE_EXTERNAL_STORAGE`；模拟器是 API 37 | Manifest 加 `WRITE_EXTERNAL_STORAGE`（`maxSdkVersion=28`），被拒时提示"没有相册权限"和"前往设置"。**被拒这条路径没有执行**（API 37 上不会出现） |
| A24 | F-FILE-04 规则 5、FE-636 第 1 条"Coil 用 fileId + variant 做缓存键" | Coil 的磁盘缓存按地址，地址每次都变 | 文件先由 SDK 下载到 `fileCacheDir`（键 fileId + variant），Coil 只解码本地文件，内存缓存键 `fileId:variant`，不用 Coil 的网络和磁盘缓存；交给 Coil 之前先看头部宽高（像素上限），认不出或超限显示"图片无法显示" |
| A25 | AC-FILE-03-9、F-FILE-03 规则 3"打开" | 模拟器上有没有能打开 pdf 的 App 不确定 | 用 `FileProvider`（只暴露下载缓存目录）+ `ACTION_VIEW` 交出，没有 App 时提示"没有可以打开这个文件的应用"。用例只验证 pdf 有"打开""保存"、`.xyz` 只有"保存"，**没有真的拉起其他 App** |
| A26 | AC-FILE-07-5、AC-FILE-07-1 | 系统照片选择器和 SAF 是系统界面，测试点不了 | `AppDeps.pickForTest` / `saveForTest` 测试替身返回测试目录里的文件（正式运行为 null，走 `PickMultipleVisualMedia(9)`、`OpenMultipleDocuments`、`CreateDocument`）。AC-FILE-07-5 用"删掉测试目录里的源文件"代替"在相册里删除原图"。选中的内容先复制进 App 私有目录（`cache/tt-pick`），处理好后交给 SDK，SDK 再复制进 `files/tianting-attachments` |
| A27 | AC-FILE-07-2（`send` 超时后重发复用 fileId） | 真实服务端不能"不回 ack" | Android 页面上没有做这条，复用 fileId 的逻辑只有 sdk-kotlin 的单测（FilesTest，FE-631） |
| A28 | AC-FILE-07-10（Android 一侧，同 H15） | 实测：待发送（`send` 被 4036 拒、已 complete、沙盒副本在）的图片，IM 钟推进 25 小时并运行 `file_purge` 后重发，服务端回 **4048**，不是 4096，SDK 不重新上传，消息失败 | 没有改服务端和 SDK（P-37 等 Frank）。`FilesE2eTest.tc_file_07_10_…` 按 PRD 写成"预期失败"：服务端回 4048 时用例通过并记 `[4048]`，哪天回了 4096 且重新上传成功，用例失败提示改回普通断言。**P-37 已落实**（tianting-sdk-kotlin@1f5eccb，tianting-android@d263b6b）：改成两条普通断言，见"P-37 落实" |
| A29 | AC-CHAT-90-8、90-9（第 13.4 节无障碍） | 新消息播报（"收到一张图片"）Android 原来就没有播报（H5 同样没有）；Espresso AccessibilityChecks 要新依赖（A10） | 只做了可读名称："+"是"发送图片或文件"，图片"图片"，文件"文件 文件名，大小"，头像"更换头像"。播报和自动无障碍检查**没有做** |
| A30 | F-ACC-06 规则 4、AC-ACC-06-6（同 H13） | 测试环境的公开桶地址是 http，Android 头像只显示 https（AC-UI-21-5） | 本次登录用上传的那张本地文件显示自己的头像（"我"页、自己的消息）；重新登录后测试环境显示首字（生产是 https，按地址显示）。sdk-kotlin 改头像后重新拉自己的资料（H13，见下） |
| A31 | `scripts/test-server.sh` | TC-FILE-04-10 要对方连发 200 张图，申请上传默认 60 次/分钟（4290）；消息发送每人每秒 20 条 | Android 的测试服务端默认 `IM_UPLOAD_RATE=1000`（可以用环境变量改），用例里对方每张之间等 80 毫秒 |
| A32 | 第 13.4 节"存储空间不足" | 下载时磁盘满没有单独的状态 | 发送前复制失败（选中内容、处理结果、SDK 沙盒）提示"存储空间不足"；"保存到"、存相册失败时按 ENOSPC 区分；下载失败统一"下载失败，点击重试" |
| A33 | P-35 L2；Compose 测试环境 | "约 30 处直接调 SDK 挂起函数"；两种测试环境的并发 | 组合里的协程（`LaunchedEffect`、`produceState`）调 SDK / AccountClient 挂起函数的 8 处改成经 `onMain`（群资料快照、群资料、群成员两处、我的群聊、好友申请已读两处、最近登录记录）。其余约 20 处在 `ImSession.launchCmd` / `AppModel.accountOp` 里，那两个 scope 是 `Dispatchers.Main.immediate`，没有改。演示登录（`LoginApi`，不是 SDK）改成 `onMain` 后 `NavLoginTest` 4 次里 2 次各 1 个用例超时（`login-error`、`chat-title`），撤回后 3 次全过，保持原样，原因没有查明。另：测试线程上直接 `runBlocking` 会借用 Compose 测试规则外层 `runTest` 的事件循环，在测试线程上执行帧，和主线程绘制并发崩溃（实测 `FilesKillE2e` phase2 4 次里 2 次）；新用例改用 `offThread`，已有用例（ServerE2eTest 等）同样的写法没有改，可能是以前偶发失败的来源之一，建议之后统一。没有为这 8 处单独加"SDK 线程"用例，靠原有 UI 用例全量回归 |
| A34 | F-FILE-07 规则 9（v1.4 "下载失败"） | Android 文件消息下载失败显示"下载失败，点击重试"（点了重新下载，A32），规则 9 写"下载失败"；H5 没有下载状态，只有提示"下载失败" | 没有改：已有用例（`FilesE2eTest` TC-FILE-03-05）断言了现在的文案，不能自己放宽。是否改成"下载失败"由 Frank 定。**P-38 ① 已定（Frank 2026-11-09）**：保留可重试，PRD 下次小版本写成"下载失败，点击重试（原生；H5 只提示）" |
| D3（续） | F-FILE-07 规则 3（sdk-kotlin） | 冷启动待发送附件 | **已处理**（tianting-sdk-kotlin@3575060）：有 `fileId` 的 QUEUED、SENDING 附件冷启动后按排队，连上后自动发 1 次（同一 clientMsgId，不重新上传），再断线不再自动发；其他 QUEUED、SENDING 仍转失败（Timeout）。单测 2 个，去掉实现后失败 |
| H13（续） | F-ACC-06、AC-ACC-06-6（sdk-kotlin） | 改头像后自己的资料缓存 | **sdk-kotlin 已处理**（tianting-sdk-kotlin@3575060）：`updateProfile(avatarFileId=…)` 成功后 `IMClient` 马上用 `users` 重新拉自己的资料（没连接时连上后拉），拿到后发 `ProfileChanged`；只改昵称不重新拉（原有 TC-ACC-06-05 的请求数断言不变）。`TokenProvider.onProfileChanged` 的回调参数改成 `ProfileChange(nickname?, avatarChanged)`。单测 2 个（去掉实现后失败），Android 端到端 `tc_acc_06_06_…` 验证缓存里是新地址。sdk-ts、sdk-swift 没有改（不在这次范围） |

C2 iOS（FE-637、FE-638）和 sdk-swift（H13、P-37 ①）实现时发现的。按"现在的实现"一列做了，**要 Frank 决定的标了"待决定"**。结果都是本地结果（模拟器 iPhone 16 Pro，iOS 18.4；CI 不运行，真机 P-13 未执行）。

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| H13（续，sdk-swift） | F-ACC-06、AC-ACC-06-6 | 改头像后自己的资料缓存（同 sdk-ts、sdk-kotlin） | **sdk-swift 已处理**（tianting-sdk-swift@662e4f2）：`updateProfile(avatarFileId:)` 成功后 `IMClient` 马上用 `users` 重新拉自己的资料（没连接时连上后拉一次），拿到后发 `profileChanged`；只改昵称不重新拉（TC-ACC-06-05 的请求数不变）。`TokenProvider.onProfileChanged` 的参数改成 `ProfileChange(nickname, avatarChanged)`，同 sdk-kotlin。单测 2 个（去掉实现后失败）；iOS 端到端 TC-ACC-06-06 查了服务端给本人的 `avatar` 就是公开桶里那张 512×512 |
| P-37 ①（sdk-swift） | F-FILE-07 规则 4（v1.4）、AC-FILE-07-10 | 自己上传、被清理的文件 `send` 回 4048 | **已处理**（tianting-sdk-swift@662e4f2）：`ownFileIds` 记下这台设备 `complete` 成功的 `fileId`（冷启动时本地库里待发的附件也算，本地库只存自己上传的），`send` 回 4048 且是自己的：和 4096 一样，有沙盒副本回到上传中重新上传、不占自动重发，没有的 `sourceMissing`；别人的 `fileId` 照常失败。单测 4 个（含"别人的 fileId 不重传"：现在没有转发入口，用例里把 id 从集合里去掉来模拟），去掉判断或不记录时用例失败。iOS TC-FILE-07-10 按 v1.4 正常断言通过（本地服务端回的是 4048） |
| I11 | AC-FILE-04-11、F-FILE-04 规则 5（iOS） | 系统 CFNetwork 在任务取消或失败时写一行 `finished with error`，`NSErrorFailingURLStringKey` 是完整的预签名地址（含 `X-Amz-Signature`）。App 和 SDK 自己没有打印，这一行关不掉。`com.apple.network` 只记路径（服务端生成的随机键，没有文件名和签名）。全量回归里 15 行，只跑文件用例的一轮 98 行（取消上传、断开的下载）。这是模拟器上看到的明文；真机上动态字段一般显示 `<private>`，没有核实（P-13） | `check_e2e_leftovers.py` 把这类 CFNetwork 行单独计数报告、不算失败；其他位置出现签名仍然失败。AC-FILE-04-11 的 iOS 部分记为部分通过。**待 Frank 决定**：是否接受（系统日志，地址 120 / 300 秒过期）。**P-38 ② 已定**：接受为已知残留，真机验证随 P-13，真机仍是明文再定 |
| I12 | AC-UI-90-12（`check_e2e_leftovers.py`） | 测试代理唤起的系统辅助功能运行时在发布布局通知失败时，把元素的标签（含消息文字）写进 App 进程的日志：`[com.apple.Accessibility:AXRuntimeNotifications] Post notification failed ... data:<SwiftUI.AccessibilityNode>: 名字，ZXQ-next`。只跑文件用例的一轮碰到 1 行，全量回归 0 行。原检查只排除了 XCTest 代理自己的子系统 | 检查放宽：这一种行单独计数报告、不算失败，其他行照旧。理由：只在辅助功能运行时被唤起（测试代理、VoiceOver）时出现，不是 App 写的。**这是放宽已有检查，待 Frank 决定是否接受**。**P-38 ③ 已定**：接受 |
| I13 | F-FILE-02 规则 4（iOS） | ImageIO 不给任何属性也会写自己的 Exif（像素尺寸、色彩空间）和 APP13；给 Exif、TIFF 传 null 会生成只有 20 字节的坏文件；png 会写 `eXIf` 块 | 编码后按段删除：jpeg 去掉 APP1、APP3～APP13、APP15、COM（保留 JFIF、ICC、Adobe），png 只留显示需要的块。单测和端到端（对方下载的原图和缩略图）都查了没有 Exif 段、GPS、型号；第一次跑单测就是这里失败 |
| I14 | AC-FILE-07-5、07-1（同 A26） | PHPicker、`UIDocumentPicker` 是系统界面，XCUITest 点不了 | 调试构建的 `TT_TEST_PICK_DIR`：点"图片""文件"时直接取这个主机目录里的文件（正式构建没有）。07-5 用"删掉这个目录里的源文件"代替"在相册里删除原图"；`TT_TEST_DROP_SANDBOX` 冷启动前删掉沙盒，造 07-10 里没有沙盒副本的那一条 |
| I15 | AC-FILE-07-6、03-5（同 A20） | 模拟器和 Mac 共用网络，飞行模式断不了 | 假 S3 的 `fail_puts` 让预签名 PUT 直接断开，代替"断网时选图"；`cut_gets` 断开正在传的 GET，代替"下载到一半断网" |
| I16 | F-FILE-02 规则 9（iOS） | 存相册要 `NSPhotoLibraryAddUsageDescription`，没有就崩溃；说明文字 PRD 没写 | 两个 Info.plist 都加了"保存聊天里的图片到相册"（文案是实现时加的），只申请"添加照片"。`Config/Info-Debug.plist` 原有的来历不明改动没有提交，只提交了这一块 |
| I17 | D4（iOS 页面） | 冷启动打断的 `.interrupted` 没有文案 | 同 Android 把冷启动打断记成网络失败：显示"图片上传失败，请检查网络后重发"/"文件上传失败，请检查网络后重发" |
| I18 | AC-FILE-07-2（同 A27） | 真实服务端不能"不回 ack" | iOS 页面上没有做这条，复用 `fileId` 只有 sdk-swift 单测 |
| I19 | AC-FILE-02-10（同 A22） | 手工 + 真机 | 没有执行。HEIC 走 ImageIO 转 jpeg，没有样本；实况照片 PHPicker 给的是静态图，没有验证 |
| I20 | 测试设施（XCUITest） | 异步用例里读不存在元素的 `.label` 会让 XCTest 记失败后卡在 tearDown（实测卡了 27 分钟，按 PID 结束了这次的 xcodebuild） | 新用例只在存在之后读 `label`，`value()` 先看 `exists`。已有用例没有改 |
| I21 | F-FILE-03 规则 3"保存"、F-FILE-02 规则 9（iOS） | SDK 下载缓存的文件名是十六进制键，没有扩展名，交出去对方不认类型 | 交给分享面板、Quick Look、相册前复制一份（原文件名，清洗 `/`、`:`）到临时目录 `tt-share`，启动和退出登录时清掉 |
| I22 | `CoverageE2ETests`（已有用例）和陌生人限额 | 已有用例把 `strangerMsgLimit` 改成 2，结束时恢复成 20；之后对方给没回过话的人连发 40、200 张图碰到 4137（单独跑时没有碰到） | 02-09、04-10 两个新用例先加好友。已有用例没有改；恢复的 20 是不是服务端默认值没有核对 |
| I23 | AC-FILE-04-10（iOS） | 200 张来回滚动时，同一张缩略图的行在加载完成前被重建，又发起一次下载（201 次 GET） | `MediaOps.loadImage` 合并进行中的同一 `fileId:variant`，之后每张缩略图只下载 1 次、再进一次 0 次 |


C4 分发时发现的（2026-11-10，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。**第 14 条按 P-39 ① 处理，想改见 PENDING P-43 ③**；其余是缺口或记录。编号接着上面第 13 条：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 14 | 第 13.4 节、P-39 ① | 撤回的新消息播报"一条消息已被撤回"：三端都还没有新消息播报（C2 的 A29），P-39 ① 定的是"挪到 C3 之后和三端无障碍一起做"，现在 C4 先于 C3 | 仍按 P-39 ① 不在 C4 做，AC-CHAT-90-9 的 C4 部分只验 axe、深色、键盘，报告记已知缺口；Frank 想在 C4 一起做见 P-43 ③ |
| 15 | F-MEN-01 规则 5、AC-MEN-01-10 | "草稿改为结构化"：三端草稿现在是内存里的纯文本（H5 `im/drafts.ts`、Android `ImSession.kt` 第 98 行、iOS `Core/Drafts.swift`），PRD 没说要不要跨重启 | 改成内存里的 `{text, mentions:[{start,len,userId}], quote?}`，不持久化（和现在一致）；AC 只要求离开再回来 |
| 16 | F-RCL-03 规则 9、AC-RCL-03-12 | H5 文件下载交给浏览器（`Attach.tsx` 第 368 行），开始后不能中止；AC 只要求 Android、iOS | H5 只在下载开始前判断状态，已交给浏览器的不处理 |
| 17 | F-RCL-03 规则 2a（sdk-swift） | 调查时没能确认 sdk-swift 的 `deletedIds` 跨冷启动有没有单独持久化（删除的行以 `deleted=true`、空 body 写回 `messages` 表） | FE-663 先核实；没有的话和 Kotlin 一样加持久化引用（含 `recalled`），AC-RCL-03-4、03-13 覆盖。核实结果：没有，已加（见问题 42，tianting-sdk-swift@b73dd2e） |
| 18 | F-MEN-01 规则 4（R-CF-03） | 三端的显示名函数优先用备注，插入 `@{公开昵称}` 要另取昵称 | 插入文本用资料的 `nickname`，空时 `userId`；选择列表照常显示备注 |
| 19 | F-RCL-02 规则 1a | 表里的"{操作者显示名}"没说用不用备注 | 按各端现有显示名规则（备注只在本地显示，不上传），和 PRD-social 一致 |
| 20 | F-QUO-02 规则 5 | "距离已加载范围 200 条以内"按什么算没写 | 按 seq 差：`已加载的最早 seq - quote.seq ≤ 200`，中间有已删除的也按 seq 算 |
| 21 | AC-CHAT-90-7、F-RCL-03 规则 7（FE-660 实测） | 老版本 SDK（sdk-ts@f417580）不认识 `quote`：收到 m1 的 `msgDeleted` 后，本地引用 m1 的 m2 的 `quote.text` 仍是 m1 原文，要等下次按 `modVer` 重拉才变（服务端那时给 `state=recalled`）。TC-CHAT-90-26 的"内存搜不到原文"对被撤回的消息本身成立，对别的消息里的引用副本不成立 | 按 F-RCL-03 规则 7 接受（没有已发布的老版本）；`legacy-c4-frames.test.ts` 只断言被撤回的消息本身。**要 Frank 定**：AC-CHAT-90-7 是否要求老版本也清掉引用副本（做不到，只能靠最低版本限制） |
| 22 | F-QUO-02 规则 3 | `expired` 是"'[图片]'或'[文件]'加'已过期'"，中间要不要空格、用不用括号没写 | `quoteLabel` 返回"[图片] 已过期""[文件] 已过期"，页面可以自己拼 |
| 23 | F-MEN-03 规则 1 | "本地推算不了时刷新"没说哪些情况本地推算 | 实时收到 @ 我（或 @ 所有人）的未读消息本地 +1、`firstMentionSeq` 取较小的；本端读到最新清零；`readUpdated` 不在本地推算（和 `chat-sync.json` 一致），只节流重拉。重拉用整份 `convs` 更新会话（和 `refreshConversations` 相同），不只改提及字段 |
| 24 | F-RCL-03 规则 2（AC-RCL-03-11） | 会话第一次出现在本端是靠实时消息（没打开过、没 `loadHistory`）时，`convs` 到的时候已经算"已加载"，没有 `modVer` 基线，第一次重连会保守地重拉一次 | 保持 C2 的做法（宁可多拉一次也不漏撤回）；集成用例先 `loadHistory` 建基线再断网 |
| 25 | F-RCL-01 规则 1、F-RCL-02 规则 2、AC-RCL-01-8 | 服务端时限可以配置（`IM_RECALL_WINDOW_SEC`、`IM_GROUP_RECALL_WINDOW_SEC`），客户端拿不到 | `canRecall` 用 PRD 默认值 120 秒、7 天；不一致时以服务端 4097 为准。以后要改时限的话建议 `GET /v1/config` 下发 |
| 26 | F-RCL-01 异常"网络超时后客户端重试 recall：幂等" | 没说 SDK 要不要自动重试 | sdk-ts 不自动重试，超时抛 `TimeoutError` 给页面，用户再点一次（服务端幂等）；4097、4030、4044 同样不重试 |
| 27 | F-RCL-01 规则 5a、第 13.1 节 | 服务端不给发起撤回的连接发 `msgDeleted`（`notifyRecalled` 排除它），只回 ack | SDK 在 ack 后本地按撤回处理，`recalledBy` 是自己，`recalledAt` 用校正后的服务端时间估算，下次重拉时以服务端为准 |
| 28 | console 问题 16、F-ADM-32（FE-671） | 下载时"用正文接口里的文件名命名"只对文件成立：图片消息没有文件名，实测隔离正文里的 `image` 也没有 `mime` | 文件用隔离正文的 `file.name`（去掉路径分隔符和控制字符，空的用 `file`）；图片用"图片-{seq}.{扩展名}"，扩展名按正文的 `mime`，没有时按代理响应的 `Content-Type`（console 用 im-server 记录的类型），都不认识用 `.img` |
| 29 | FE-671 第 2 条、AC-RCL-04-1、F-ADM-31 规则 7 | 撤回的消息在正文页还能不能勾选删除：任务单写"照旧"；后台删除已撤回的是幂等 200、状态和用户端文案保持撤回，但页面提示"已删除 N 条，不再显示" | 照旧可以勾选（审计照写，和服务端一致）。**要 Frank 定**：撤回的是否改成不能勾选，或删除后提示改成"已撤回的保持撤回"。2026-11-10 P-44 ② 已落实（tianting-admin@aa47b1f：撤回的复选框禁用并说明"已撤回的消息不能删除"，删除请求和"已删除 N 条"只算当前选中且仍可删的；Vitest、e2e 本地） |
| 30 | F-ADM-32、第 11.6 节（FE-671） | 代理支持 `variant=thumb`，PRD 没说后台要不要看缩略图；blob 地址多久撤销没有写 | 只下载原图（`original`）；blob 地址在点击后 1 秒撤销（Chrome 点击时已拿到字节，留 1 秒给其他浏览器），字节不在页面里留着；断流时 `blob()` 失败，按网络错误提示，不保存残缺文件 |
| 31 | AC-CHAT-90-7、问题 21（FE-661 实测） | 老版本 sdk-kotlin@2819ce6 的 `MsgBody` 没有 `quote` 字段，解码时直接丢掉：引用原文从来不进内存和本地库。所以 Kotlin 老版本的 TC-CHAT-90-26 对引用副本也成立（和问题 21 的 sdk-ts 不同） | 不用处理；`LegacyC4FramesTest` 断言被撤回的消息和引用副本都搜不到，同一文件在 2819ce6 上通过 |
| 32 | F-RCL-03 规则 4（FE-661） | `convs` 的 `lastMsg` 带 `deleted`+`recalled`、本地已加载的那条还没清（错过了 `msgDeleted`）：sdk-ts 只按"已删除"清（`recalled` 等下次重拉），撤回文案晚到 | sdk-kotlin 用 `lastMsg.recalled`、`recalledBy` 直接按撤回清（单调，只能从假变真）；建议 sdk-ts、sdk-swift（FE-663）对齐，不改产品口径。已对齐（tianting-sdk-ts@55d39b1：`convs` 的 `lastMsg` 带 `recalled` 时按撤回清，单调；单测"问题 32"，本地） |
| 33 | F-RCL-03 规则 9、AC-RCL-03-12（FE-661） | "下载中止、提示已撤回"SDK 给 App 什么接口没写；被后台删除、文件过期时要不要同样中止也没写 | `download` 抛 `DownloadAbortedError(fileId, recalled)`，同时发 `IMEvent.DownloadAborted(fileId, recalled)`；后台删除、过期同样中止（`recalled=false`）。部分文件删除。建议 sdk-swift（FE-663）同名对齐，FE-667、FE-669 用它。已对齐（tianting-sdk-ts@55d39b1：`downloadUrl` 取地址期间被撤回、删除、过期抛 `DownloadAbortedError(fileId, recalled)`，同时发 `downloadAborted`，先于 `filesExpired`；H5 的下载交给浏览器后停不了（问题 16），所以中止的是取地址这一段；tianting-web@8923a5b 文件下载用它提示；TC-RCL-03-12 单测、H5 Vitest，本地） |
| 34 | FE-661 第 2 条 | 本地库第 3 版只多了 `deleted_refs` 的一种 `kind`，表结构没变；任务单要求版本 +1 | 照做：第 1、2 版原地升级；副作用是降级到 C4 之前的 SDK 时按"版本不同"重建本地库（只是缓存，服务端可以恢复，撤回过的原文不会因此回来） |
| 35 | FE-661 第 3 条（和 sdk-swift 对齐） | "撤回后 `secure_delete` 或 `VACUUM` 选一个" | sdk-kotlin 沿用 FE-435 的做法：`PRAGMA secure_delete=ON`，构建不支持时每次清除后 `VACUUM`；`journal_mode=DELETE`，提交后 `wal_checkpoint(TRUNCATE)`；清引用的行和撤回的行一样触发清理。FE-663 选型时参考 |
| 36 | F-UI-09 规则 4、BE-479（FE-665） | sdk-ts 的 `AccountConfig.features` 只声明了 `files`，没有 `msgActions`（服务端 `/v1/config` 已经下发） | 页面按服务端 JSON 读 `features.msgActions`，没有改 SDK；建议 sdk-ts（以及 Kotlin、Swift 的配置模型）补上这个字段。已对齐（tianting-sdk-ts@55d39b1 `features.msgActions?: boolean`；tianting-sdk-kotlin@686e14f `Features.msgActions` 默认 false；tianting-web@8923a5b 按类型读，没有字段按关闭；Swift 未改，随 FE-663；单测、集成测试"问题 36"，本地）。Swift 已补：tianting-sdk-swift@b73dd2e `Features.msgActions: Bool?`（单测 `testConfigFeaturesMsgActions`，本地） |
| 37 | AC-RCL-01-8、AC-RCL-02-5、问题 26（FE-665） | `recall` 超时、断线（不是 `IMError`）时页面提示什么没写 | 按 H5 现有约定（`errCode` 非 `IMError` 当 5000）提示"服务异常，请稍后重试"，消息不变，不自动重试；4044 用 errors.json 的"消息不存在或已过期"。菜单项在打开时算好，开着的时候过了时限"撤回"还在，点了按服务端 4097 提示（AC-RCL-01-8） |
| 38 | F-MEN-03 规则 2、AC-MEN-03-5（FE-666） | 只规定了前缀优先级，没说"[有人@我]"后面跟草稿还是最后一条消息 | 有提及时显示"[有人@我] {最后一条摘要}"（群聊带发送者前缀），草稿不显示；没有提及时照旧"[草稿] {草稿}"。"[有人@我]"用 `color.danger`，和"[草稿]"相同 |
| 39 | F-MEN-01 规则 5、6，第 13.4 节（FE-666） | 成员选择的交互（在输入框里接着打字过滤，还是单独的搜索框）、自己在不在列表里、关掉选择后 @ 怎么处理没写 | 敲 @ 后弹出底部面板（窄屏）/ 居中面板（宽屏），面板里有"搜索群成员"框（`keyword`，停 300 毫秒再搜，空串等于不搜），列表不含自己（F-MEN-03 规则 6），有"加载更多"；关掉（取消、Esc、点遮罩）时 @ 留作普通文字。光标进 @ 内部时：从后面往左移进来的去前端，其余去后端 |
| 40 | F-MEN-03 规则 3、AC-MEN-03-6（FE-666） | "有人@我"按钮：提及在已加载范围很远（超过 200 条）时怎么办、滚到那条时按钮要不要消失没写 | 引用跳转的 200 条限制（F-QUO-02 规则 5）不用在这里：按 200 条一页往上翻直到加载到那条（已加载范围仍连续）；点了、或那条进了视口，按钮都消失；打开会话时只取一次 `firstMentionSeq`（读到最新后服务端就清零了） |
| 41 | F-QUO-01 规则 2、F-QUO-02 规则 3（FE-665） | 发送时被引用的消息已不可用（4040 `quoteUnavailable`）要不要提示没写；引用条里那条消息不在已加载范围（只剩草稿里的 msgId）显示什么没写 | 4040 按 errors.json 提示"内容不存在"（和其他发送失败一样由 App 统一提示一次，F-UI-08 规则 3），消息标记失败，不自动重发；引用条显示已加载的那条（跟着撤回、删除变成"引用内容已被撤回 / 删除"），找不到时显示"引用内容不可用"，草稿里只存 msgId，不存原文 |
| 42 | F-RCL-03 规则 2a、问题 17、34（FE-663） | sdk-swift 的本地库没有 `deleted_refs`：没加载过的消息被删除、撤回，冷启动后迟到的旧应答能把原文写回来；Kotlin 第 3 版只是多一种 `kind`，Swift 要新建表 | 第 3 版新建 `deleted_refs`（`kind` 为 `id`、`seq`、`recalled`，`recalled` 的 ref 是 JSON，和 Kotlin 相同），第 1、2 版原地升级、旧行保留；`removedClient` 仍只在本次登录内（C1 以来的行为，没改）。不改产品口径 |
| 43 | AC-QUO-02-2、TC-QUO-02-02（FE-663） | sdk-swift 没发出的文本只在内存（C1 的 FE-141 口径），Kotlin 的待发文本写本地库；TC-QUO-02-02"本地库搜不到"对 Swift 的待发引用不适用 | 待发的引用在内存里清除（单测断言），本地库只断言已确认的消息；不改 |
| 44 | 问题 33、第 13.1 节（FE-663） | Swift 的接口写法和 Kotlin 不同 | `IMEvent.downloadAborted(fileId:recalled:)`（Kotlin `IMEvent.DownloadAborted`）、`DownloadAbortedError`（同名）；调用方自己取消下载仍抛 `FileTransferError.cancelled`、不发事件；`RecallNotice.operatorId`（Kotlin `operator`，Swift 关键字）；单聊带 `mentions` 抛 `IMArgumentError`（Kotlin `IllegalArgumentException`）；`Features.msgActions` 是 `Bool?`（老服务端 `nil`，Kotlin 默认 `false`，页面都按"没有 = 关"处理）。语义一致 |
| 45 | AC-MEN-01-9、AC-MEN-01-12、F-MEN-01 规则 5（FE-668） | Android 有的输入法（例如 Gboard）退格、移动光标时会把光标前的词重新放进组字区（`composition` 不为空），这时按"组字中不改模型"不能整段删除 @；上屏后这段 @ 文字已经残缺，区间被去掉（不会错位，也不会发出 mentions），但不是"一次退格整段删除" | 按 PRD 原意实现：只有不在组字中的退格整段删除（`core/Mentions.kt` 的 `applyEdit`，硬键盘和测试注入的退格都走这条），组字中的改动等上屏后按组字前的文字整体对一次。真实输入法下的表现记"未执行（要人工）"，随 AC-MEN-01-9 的手工检查一起看；不改产品口径 |
| 46 | AC-RCL-03-12、问题 33（FE-667） | SDK 对后台删除、文件过期也中止下载（`DownloadAbortedError.recalled=false`），页面要不要提示没写 | 只有 `recalled=true` 提示"该消息已被撤回"；`false` 时不另外提示（气泡自己变成"该消息已被删除"或"文件已过期"），下载状态清掉，不显示"下载失败"。和 H5 一致（H5 取地址前判断，后台删除提示"该消息已被删除"，Android 气泡已变成同一句） |
| 47 | 第 13.4 节"长按菜单要有辅助操作"、AC-CHAT-90-9（FE-667） | H5 有键盘入口；Android 只写了"长按"（第 13.2 节），辅助操作的形式没写 | 气泡的长按带可读名称"消息操作"（TalkBack 的双击并按住），菜单项都有文字；图片、文件里能点的地方也接长按。没有加 `espresso-accessibility`，用语义树检查（同问题 A10）。不改口径 |
| 48 | AC-QUO-02-3、AC-MEN-03-6（FE-667、FE-668） | 跳转后"滚动到原消息"的位置没写 | 列表是反向的，按下标滚到那条（落在列表可见区的底部），整行高亮 2 秒（`surface.selected`，和 H5 同色）；H5 是滚到中间。不改口径 |
| 49 | AC-RCL-01-8、第 13.4 节"长按菜单要有辅助操作"（FE-669） | iOS 的 SwiftUI 长按菜单随行一起建好，不是长按时才算：页面打开后消息过了 2 分钟，长按仍有"撤回"；菜单开着时重画行，打开的菜单也会跟着变（"撤回"消失，AC-RCL-01-8 第二步做不到）。辅助操作的形式也没写 | 每行在撤回时限（`ts` + 2 分钟、7 天，按服务端时间）到点时重画一次；菜单开着时（自定义 preview 出现到消失）推迟到关上以后再重画，所以开着过了时限点"撤回"仍按 4097 提示。菜单项同时作为 VoiceOver 的"操作"（`accessibilityActions`）。语义和 H5、Android 相同，不改口径 |
| 50 | TC-RCL-03-03、TC-MEN-02-05（FE-669、FE-670） | iOS 模拟器和 Mac 共用网络，测试不能给 App 断网 | App 经测试进程里的 TCP 代理（`UITests/AccountSupport.swift` 的 `TCPProxy`，C1 已有）连 im-server，"断网"是代理切断连接并拒绝新连接，对方直接连服务端；真机断网在 P-13。不改口径 |
| 51 | F-FILE-04 规则 5、AC-RCL-03-12（FE-669） | 下载被中止（撤回）时，系统的 CFNetwork 日志会记下被取消请求的完整预签名地址，地址里服务端加的 `response-content-disposition` 带着文件名。App 关不掉系统日志（同 C2 的 I11），C4 让"下载中被取消"变成常见情况 | iOS 端照记：`check_e2e_leftovers.py` 把 CFNetwork 带签名地址的行算系统日志，计数报告、不判失败（本次 1 行）；App 自己的日志没有文件名。**要 Frank 定**：要不要让服务端给原生客户端的下载地址不带文件名（客户端本来就不用它；H5 下载要用，可以按平台区分），这是协议和服务端的改动。**P-45 已落实（tianting-server@8527b7b、tianting-sdk-swift@c97d08e、tianting-sdk-kotlin@d6603f5、tianting-sdk-ts@7739c55、tianting-ios@62e3355）**：PRD-chat v1.7 F-FILE-03 规则 4，`/v1/files/urls` 可选 `bareDisposition`，两套原生 SDK 带 `true`，文件地址只签 `attachment`；`check_e2e_leftovers.py` 改成有一行就失败，`C4FilesE2ETests` 复跑 0 行（原来 1 行，本地） |
| 52 | AC-RCL-03-12、问题 33、44（FE-663 遗留，FE-669 发现） | sdk-swift 的 `URLSessionObjectStoreTransport.get` 用 `URLSession.download(for:)`：撤回时 SDK 取消下载，SDK 自己的缓存和 `.part-` 都删了，但 URLSession 放在 App 临时目录里的 `CFNetworkDownload_*.tmp` 留着（模拟器上每取消一次多一个，重启 App 也不清），所以 iOS 上"不留部分文件"只做到 SDK 管的部分 | 页面不碰 URLSession 的临时文件（同时在下载的别的文件也在那里）。`testTC_RCL_03_12_…` 断言 SDK 缓存不留，URLSession 临时文件用 `XCTExpectFailure` 标成已知问题。建议 sdk-swift 改成边下边写到 `.part-`（例如 `bytes(for:)`）或取消后删掉临时文件，另开任务；不改产品口径。**已修（tianting-sdk-swift@c953dde、tianting-ios@4e6ecdb）**：GET 改成 data task 边下边写 SDK 自己的 `.part-`，失败或取消时删除，不再有 `CFNetworkDownload_*.tmp`；sdk-swift `ObjectStoreDownloadTests` 5 个（真实 socket，取消、断连、完整、403、启动前取消；改回旧实现 4 个失败），`TC_RCL_03_12` 去掉 `XCTExpectFailure` 后通过（本地） |
| 53 | AC-ACC-03-37 前半（TC-ACC-03-40，FE-672 全量回归发现） | iOS `AccountE2ETests.testTC_ACC_03_40_real4290StaysHere` 在本机稳定失败：[IM 钟] 前进 61 分钟后 45 秒内等不到"退避重试成功"（会话保存次数没有增加）。单独重跑仍失败；把 sdk-swift 切回 C4 改动前的 b73dd2e 也失败，和问题 52、P-45 的改动无关 | 没有查清是用例、App 的退避还是服务端限流窗口（[IM 钟]）的问题，不属于 C4 范围，本次没有改。要另开任务查（先在 C1 验收时的提交上复跑确认是不是回归） |

C5 分发时发现的（2026-11-10，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。都是缺口或记录，**要 Frank 定的在 backend-chat 第 72、81、82、86 条**（第 86 条影响 AC-CVS-03-10 用例的写法）。编号接着上面第 53 条：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 54 | F-CVS-03 规则 6、F-GRP-03 规则 4 | 规则 6 只写了"`truncated` 时不推断"；`deletedTruncated=true` 时，不在 `convs` 也不在 `deleted` 里的群可能只是被截掉的删除记录 | `truncated` 或 `deletedTruncated` 为 true 都不推断"你已不在该群"（BE-487 的向量有用例） |
| 55 | F-CVS-03 规则 2、3、7，F-CVS-04 规则 1 | 收到 `convDeleted` 的其他设备怎么更新本地没写全 | `minSeq = clearedSeq + 1`、`readSeq = max(readSeq, clearedSeq)`、不再置顶、保留免打扰；按规则 6 ② 物理清除；**这台设备上这个会话的草稿也清除**（规则 7 的"清除本地草稿"按会话算，不只是操作的那台） |
| 56 | F-CVS-01 异常 | "写入失败 5000，客户端回滚乐观显示"；4290 在 `errors.json` 是可重试 | 乐观更新，任何错误（5000、4220、4290、4030）回滚并提示，**不自动重试**（自动重试加上"后到的覆盖"会让用户看到状态来回跳）；4290 提示"操作过于频繁" |
| 57 | 第 13.3 节、PRD-ui 第 6 节 | 置顶背景色、免打扰图标都不在 PRD-ui 的 token 和图标清单里 | 按第 13.3 节"新图标加入 6.6 节清单，新增的颜色要有深色 token"：BE-488 第 2 条在 PRD-ui 6.1、6.6 节加（带对比度），三端用同一个 token 名 |
| 58 | AC-CVS-02-6 | 留在 C5 还是挪到 C3：预期是"不响铃不弹通知" | 留在 C5：只验客户端前台行为，不要推送通道；三端现在没有任何系统通知、提示音代码（2026-11-10 调查），用例断言没有通知、提示音替身没有调用；C3 恢复加通知后 FE-653、FE-655 补跑 TC-CVS-02-06 |
| 59 | F-CVS-03 规则 7 | "清除本地消息、草稿、缓存文件"没提这个会话正在上传、排队的附件 | 取消正在上传的（`cancelUpload`，服务端删临时对象）、丢弃排队的，和失败消息一样清除 |
| 60 | F-APN-04 规则 2、F-CVS-02 规则 4 | iOS App 图标角标的口径写在推送里（C3） | C5 只做 App 内"消息"标签角标和 H5 标题；App 图标角标随 C3（要 `.badge` 授权，R-CF-20），C5 不设 |
| 61 | F-CVS-02 规则 5 | "[N 条]"的 N 超过 99 怎么显示没写；免打扰会话有 @ 我时是红点还是数字没写 | N 照实显示（这是摘要不是角标，F-UI-04 规则 4 的"99+"只管角标）；有 @ 我时仍是小红点，前缀"[有人@我]"（规则 5 没有例外，总未读里计入提及） |
| 62 | FE-680 第 5 条、P-46 ④（FE-680 实测） | 任务单第 5 条写"清除引用这些消息的 `quote.text`"，和 P-46 ④"删除后对方新消息里的引用摘要照常显示"冲突 | **协调方定（实现 Frank 已定的 P-46 ④，不是新决定）**：保留下来的消息（seq ≥ minSeq）里的引用摘要照常显示；只有本端还没发出、引用了被清除消息的才清除原文（状态 deleted）。第 5 条已改；FE-681、FE-682 同口径 |
| 63 | F-CVS-04 规则 2（FE-680 实测） | 其他设备删除会话（收到 `convDeleted`）时，本机上传中、排队、失败的消息要不要丢弃，PRD 没写 | **协调方定（实现细节，不改产品行为）**：不丢弃，只发事件让页面清草稿；这些消息发出后 seq > clearedSeq，本来就会让会话重新出现。只有本端主动 `deleteConv` 才按第 59 条取消和丢弃。三套 SDK 统一 |
| 64 | F-MEN-02、F-CVS-03（FE-680 实测） | 删除后会话因为本地有更新的消息没有消失，而 `firstMentionSeq` 落在清除范围内时，`mentionUnread` 要等下次 `convs` 才更正 | 小缺口，记录。**FE-686 核对**：真实 im-server 删除后紧接着的 `convs` 立即给出 `mentionUnread=0`、不带 `firstMentionSeq`（ack 只带 `clearedSeq`），服务端没问题；客户端到下次 `convs` 前的小缺口仍记录 |
| 65 | F-CVS-03 规则 6a、AC-CVS-03-12（FE-681 实测） | 删除会话后本地"可见下限"只升不降：同一次登录里被移出又重新入群（historyVisible=true），服务端 `minSeq` 变小，sdk-ts 的 `MessageList.floor` 仍会丢掉之前被清除范围里的旧历史 | **协调方定（实现 PRD 已写的规则 6a，不是新决定）**：服务端 `minSeq` 变小时放低本地下限，旧历史可以加载。sdk-kotlin 已按此做（7f13c3f）；sdk-ts 补改和用例，sdk-swift 同口径 |
| 66 | F-CVS-03 规则 4（FE-683 实测） | H5 原有缺陷：不在会话列表、也没有消息的群，聊天页一直显示"加载中…"；C5 删除群会话后就会出现 | 已修（tianting-web@78c2b2f）：显示空的聊天页，可以发言，标题取 `groupInfo` 的群名。Android、iOS 验收时同样检查 |
| 67 | AC-CHAT-90-9（FE-683 实测） | H5 原有缺陷：宽屏导航栏"消息"有角标时，可读名称不含数字（axe label-content-name-mismatch，serious），C5 用例让角标出现后才测出 | 已修（tianting-web@78c2b2f）："消息，5 条未读"。Android、iOS 验收时检查角标的可读名称 |
| 68 | F-CVS-03 规则 2（FE-682 实测） | 本机正在发送的消息，迟到的 ack 带回的 seq 落在清除范围里（`upToSeq` 之前已看到别人更新的消息时会出现）：sdk-ts 会被下限挡掉、一直停在"发送中" | **协调方定（实现细节）**：三套 SDK 按 sdk-swift@94fda7b 统一，这条消息一起去掉。sdk-ts、sdk-kotlin 补改 |
| 69 | BE-487（FE-682 实测） | chat-sync.json 的 format 说明写 `readSeq = maxSeq`，C5 用例带了 `unread`，实际按 `readSeq = maxSeq − unread` 展开（sdk-ts、sdk-swift 一致） | 说明文字随 BE-489 改正，用例数据不变。**已修**（BE-489，`5b38668`：生成器改说明后 `-update`，`cases` 逐字节不变，只有 `format.会话` 一句不同；三套 SDK 下次同步协议时带上） |
| 70 | F-CVS-03、F-GRP（FE-682 实测） | 被删除隐藏的群在离线期间被移出、之后没再加回：各 SDK 都不发 `convRemoved`（它本来就不在列表里） | 不影响显示，记录；FE-686 没有构造这个场景，未核对，仍是记录状态 |
| 71 | 问题 56（FE-684 实测） | convSet / convDelete 的 4290 页面写"操作过于频繁"（errors.json 的 message），SDK 通用 `*:4290` 是"操作太频繁，请稍后再试" | **协调方定**：C5 两个命令按问题 56 用"操作过于频繁"（H5、Android 已如此，iOS 同样）；其他命令照旧。三端一致即可，不改 SDK 通用表 |
| 72 | AC-RCL-01-8（FE-684 实测） | 模拟器时钟比 Mac 慢约 9 秒，C4 用例 `tc_rcl_01_08` 按设备时钟推算"112 秒前"，按服务端时间超过 2 分钟，稳定失败（余量只有约 8 秒） | **协调方定**：用例改按服务端时间推算（不放宽断言、不改设备时间设置）；FE-686 之前修 |
| 73 | F-RCL-01 规则 1（问题 72 反证时发现） | SDK 本地判断"撤回"菜单项的 120 秒是写死的，不读服务端的 `IM_RECALL_WINDOW_SEC`；服务端改了时限时，菜单仍显示"撤回"，点下去才回 4097 | 记录。默认配置下两边一致，不影响 C5。**FE-686 结论：PRD 不要求客户端跟随**（F-RCL-01 规则 1 写死 120 秒，AC-RCL-01-8 按 ts 和服务端时间判断，4097 文案固定"2 分钟"，`/v1/config` 的 `limits` 不下发时限），不改代码；要可配需 PRD 加下发字段，属新需求 |
| 74 | AC-FILE（FE-684 复跑时发现） | C2 用例 `FilesE2eTest.tc_file_02_06` 偶发失败：先打开聊天页、让对方发图，然后才调慢假 S3，缩略图可能已经正常下完，看不到"image-loading" | 用例写法问题，和 C5 无关。改成先调慢假 S3 再发图，断言不变；FE-686 之前修。**已修**（tianting-android@17b2ee7） |
| 75 | Android kill 用例第二阶段（FE-686 全量发现） | 冷启动新进程时 App 偶发崩溃 `Detected multithreaded access to SnapshotStateObserver: previousThreadId=47, currentThread=main`（主线程 `dispatchDraw` 的 `observeReads`）。出现在 C2 `FilesKillE2e.uploadKilled`、C4 `C4KillE2e.recallWhileKilled`，大约 10 次里 2～3 次；C5 `C5KillE2e` 没有出现；FE-684 时没有出现，之后 App 只改过测试 | 记录，没有改。不是 C5 的问题；谁在主线程之外做了绘制或测量没有查清，要另开任务（堆栈见 c5-frontend-acceptance-2026-11-11 第 8 节） |

C6 分发时发现的（2026-11-11，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。**要 Frank 定的是第 82 条（文案，不阻塞）**，另有 backend-chat 第 104、108、118、119、121 条影响后台页面和隐私清单；第 76 条是"PRD 别处已写明"；其余是缺口或记录：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 76 | 第 4.1 节共同规则、F-MOD-03 规则 8 | 举报入口什么时候隐藏 | **按 PRD 原文**：`features.moderation=false`（backend-chat 第 96 条）时三类入口都不显示；4510 的提示和开关无关（服务端返回了就显示） |
| 77 | F-MOD-03 规则 2、2a、8，F-UI-09 | 消息菜单"举报"对哪些消息显示没写 | 别人发的、已经发送成功的 `text`、`image`、`file`；自己的、`tip`、`custom`、发送中和失败的都没有；撤回提示行只在**别人**撤回的消息上有（隔离原文过期后点了仍会成功，证据只有元数据，AC-MOD-03-12）。资料卡对自己不显示；群信息页只对当前成员显示 |
| 78 | F-MOD-03 规则 1 | 6 个 `reason` 的界面文案没写 | 垃圾广告（`spam`）、辱骂骚扰（`abuse`）、色情低俗（`porn`）、诈骗（`fraud`）、违法违规（`illegal`）、其他（`other`），三端和后台一致；默认不选，必须选一个才能提交 |
| 79 | F-MOD-03 规则 8 | "提交成功后询问是否拉黑（单聊）"：举报用户时问不问、已经拉黑的问不问没写 | 单聊里的消息举报和从资料卡举报用户都问；对方已在黑名单、群消息、群举报不问；选"拉黑"走现有的拉黑接口，失败单独提示，不影响举报结果 |
| 80 | 第 13.3 节、PRD-ui 6.6 节 | "举报"的图标不在 6.6 节清单 | 用 Lucide `flag`，BE-497 第 4 条加进 PRD-ui 6.6 节（同第 57 条的做法） |
| 81 | F-MOD-02 规则 6 | 4510 的失败消息能不能"重发"没写（原样重发一定还是 4510） | 4510 的失败气泡只有"复制""删除"，没有"重发"；SDK 记失败原因（FE-690 第 3 条），原生端冷启动后也一样 |
| 82 | 第 11.5 节 4510 文案、F-MOD-02 规则 2 | 4510 的文案是"消息包含不当内容，无法发送"，昵称、用户名、群名、群公告、好友申请附言被拦截时用这句不贴切 | **要 Frank 定（界面文案，不阻塞）**。建议：消息用原文；其他场景"内容包含不当信息，请修改后重试"，表单内容保留；`errors.json` 的 `message` 不变，客户端按"命令 + 码"区分（同 4292 的做法，RC2-20）。定之前按建议实现。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 83 | 第 11.5 节 4040 `reportTargetInvalid` | 举报 4040 的文案没写 | "无法举报该内容"（不说明原因，响应本来相同）；4030 `featureDisabled` 时入口本来不显示，竞态下提示"举报功能暂不可用" |
| 84 | F-MOD-03 规则 1 | `note` 的 200 个码点在客户端怎么数 | 按码点数（和服务端一致），超出不能再输入；粘贴超长的截断到 200 并提示 |
| 85 | F-MOD-03 规则 2、R-CF-21 | 第一批客户端只举报单条，协议允许 20 条 | 三端只传 1 个 `msgId`；多选在后续批 |
| 86 | F-MOD-01 规则 1、AC-MOD-01-7 | 后台批量导入的文件格式没写 | txt、UTF-8、一行一个（去掉首尾空白，空行交给服务端计入无效），浏览器里拆成数组提交，超过 5000 行不提交（backend-chat 第 103 条） |
| 87 | F-MOD-04 规则 1、4 | 后台举报详情里"该用户被举报的次数"按什么算 | 按被举报用户，所有类型都算（backend-chat 第 117 条），保留期内（第 119 条定了之后可能变） |
| 88 | F-MOD-03 规则 1、第 77 条（FE-690 实测） | 哪些消息显示"举报"由三端页面各自判断，口径可能不一致 | **协调方定**：三套 SDK 统一提供 `canReport(m)`（判断依据照第 77 条），页面只调用它；sdk-ts 随集成重跑补，sdk-kotlin、sdk-swift 在 FE-691、692 里做 |
| 89 | 第 77、88 条（FE-691 实测） | `canReport` 的两处边界：自己以群主、管理员身份撤回的别人的消息；撤回者未知（老服务端 `recalledBy` 为空） | **协调方定**：前者不显示"举报"（是自己撤回的）；后者按"别人撤回"处理，显示"举报"。三端一致 |
| 90 | 各 SDK `scripts/sync-protocol.sh`（FE-691 实测） | 同步脚本复制的是 im-mvp 工作区，不是已提交内容；工作区里有别的代理未提交的改动时会同步到半成品 | 代理改为从已提交内容 `git archive` 导出后同步；脚本本身随 FE-698 前改成取已提交内容 |
| 91 | FE-690 集成用例（FE-691 实测） | sdk-ts `report.test.ts` 里"非参与者举报 c2c 消息期望 4040"会先被本地校验以 4000 拒绝，服务端完成后仍会失败 | 用例写法错误：改成用群消息测服务端 4040，c2c 情况断言本地 4000；sdk-ts 集成重跑时改 |
| 92 | 第 77、89 条（FE-693 实测） | `canReport` 还有两处没写：自己的消息被别人撤回后的提示行；本机没看过的消息（服务端校验 `readSeq ≥ seq`） | **协调方定**：前者不显示"举报"（消息本来是自己的）；只有 `seq ≤ conv.readSeq` 的才显示，会话不在本地列表时不显示。三套 SDK 的 `canReport` 都按此（sdk-kotlin 9e31f6d 之后补） |
| 93 | PRD-ui 触控区域（FE-693 实测） | 撤回提示行后面的"举报"跟着小字号，点击区域低于 44px | 按 PRD-ui 的最小触控区域补齐（扩大点击区域，不改提示行文字）；H5 随服务端完成后的重跑修，Android、iOS 同样检查 |
| 94 | F-MOD-02 规则 6（FE-693 实测） | 超时之后才到的 4510 只改失败原因，SDK 不再发 `sendFailed`，页面不会再弹"消息包含不当内容"提示 | 接受：气泡变成不能重发、失败图标读出原因，足够；不加事件 |
