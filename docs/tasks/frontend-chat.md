# 前端任务单：天听聊天产品第一批（三套 SDK、H5、Android、iOS、管理后台页面，C1、C2）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.1](../prd/PRD-chat-v1.1.md)（冻结）第 5、6、13、14.1、15 节；[PRD-ui v1.2](../prd/PRD-ui-v1.3.md)（界面规则 B-1～B-8，第 6 节视觉）；[SDK 设计说明](../sdk/README.md)；[errors.json](../../protocol/errors.json) |
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
