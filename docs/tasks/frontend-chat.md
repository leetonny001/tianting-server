# 前端任务单：天听聊天产品第一批（三套 SDK、H5、Android、iOS、管理后台页面，C1、C2）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.0](../prd/PRD-chat-v1.0.md)（冻结）第 5、6、13、14.1、15 节；[PRD-ui v1.2](../prd/PRD-ui-v1.2.md)（界面规则 B-1～B-8，第 6 节视觉）；[SDK 设计说明](../sdk/README.md)；[errors.json](../../protocol/errors.json) |
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
| 全部 | ⬜ 未开始 | 依赖 BE-400、BE-401 的协议；服务端没好时用假 HTTP 开发 |

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
