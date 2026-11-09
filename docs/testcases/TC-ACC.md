# 测试用例：账号注册和登录（ACC，C1；头像部分 C2）

> 依据：[PRD-chat v1.4](../prd/PRD-chat-v1.4.md)（冻结）第 4、5 节；[ERRORS.md](../../protocol/ERRORS.md)。任务单：[backend-chat](../tasks/backend-chat.md)、[console-chat](../tasks/console-chat.md)、[frontend-chat](../tasks/frontend-chat.md)。
> 范围：标了【C1】【C2】的 AC-ACC（CH-27）。C3～C6 的 AC-ACC 不在这里。
>
> 通用约定：
> - 环境：**E1 单节点**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库；`IM_TRUSTED_PROXIES` 配成测试客户端，换 IP 用 `X-Forwarded-For`，PRD 第 4.5 节）；**E1 + 假 S3**（再加 `objstoretest` 独立进程，`IM_FILES_ENABLED=1`，公开桶和私有桶都在假 S3 上）；**E1 + console-server + tianting-admin**（`CON_TEST_MODE=1`，Postgres）；**真实 MinIO**（`deploy/testenv-ci.sh` 的固定版本）；**EP 性能**（目标机，P-12）；**Postgres**（B6 之后）；**模拟器**（Android 模拟器、iOS Simulator）；**真机**（P-13）。开发机不安装软件。
> - **CI 现在不运行（PENDING P-21）**，所有结果都是本地结果，记录时写明环境和提交号。性能用例在开发机的结果只记录，**不算通过**，发布判定等 P-12。真机用例在 P-13 到位前记未执行。
> - 层级：**后端 e2e**（Go e2e，对 `/v1/*`、`/ws`、`/admin/*` 断言）、**单测**、**SDK 单测**（三套 SDK，假 HTTP + `FakeTransport` + 假时钟，向量用例按 `protocol/vectors`）、**Playwright**（H5，Chromium + WebKit，连真实 im-server 测试模式）、**原生 UI 测试**（Compose UI Test、XCUITest，模拟器）、**端到端**（真实 im-server + 客户端）、**合同测试**（`tools/objstore-contract`）、**性能**、**手工**（留截图或录屏）。标了 Playwright 和原生 UI 测试的前端用例，H5、Android、iOS 都要跑，某端不适用的按 AC 的括号。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算，避免跨日（R-CQ-41）；**[SDK 钟]** 各 SDK 的 `Clock` 注入；**[真实钟]** 依赖对象存储的真实时间。**预签名一律用真实时钟**，推进 [IM 钟] 不影响签名（第 4.4 节）。
> - 负向断言"假通道、接口无调用"用哨兵法（第 4.4 节）。每个用例使用独立数据库；用例之间用 `/admin/test/limits/reset {scope:"auth"}` 清计数。
> - 批次字段：C1、C2 按 AC 的标记；【全部】的 AC 只验证到 C2 为止的部分（PRD 第 15 节累计表）。
> - 账号和记号：`V` 受害者，`X` 攻击者（持偷来的凭证或知道密码），`D1`、`D2` 同一账号的设备，`u1`、`u2` 普通用户。`A1`、`R1` 登录得到的 access、refresh，`A2`、`R2` 下一代，以此类推；`C1`、`C2` 恢复码的代。
> - 请求简写：`refresh(Rx)` = `POST /v1/account/token/refresh {refreshToken:Rx}`；`auth(Ax)` = 用 Ax 连 `/ws` 发第一帧 `auth`；`login(u, p)` = `POST /v1/account/login`；`recover(u, c, p)` = `POST /v1/account/recover {username:u, recoveryCode:c, newPassword:p, platform}`；`password` = `POST /v1/account/password`。除特别说明，账号都已确认恢复码。
> - 钩子：**账号并发钩子**（第 4.5 节，Argon2 之后、提交之前暂停）、**Argon2 阻塞钩子**（占满闸门）、`POST /admin/test/faults` 的故障注入。
> - 窗口：`IM_REFRESH_OVERLAP`=2 分钟。"T+3 分钟"都是推进 [IM 钟] 得到的；攻击序列的时间点写在步骤里。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-ACC-01-01 | 邀请码模式注册成功，用户名规范化为小写，token 能通过 WS auth | AC-ACC-01-1 | 后端 e2e | P0 |
| TC-ACC-01-02 | 边界：用户名 4 位、32 位，密码 8、128 个码点都能注册 | AC-ACC-01-1 | 后端 e2e | P0 |
| TC-ACC-01-03 | 四种无效邀请码的应答逐字节相同，不暴露用户名是否存在 | AC-ACC-01-2 | 后端 e2e | P0 |
| TC-ACC-01-04 | closed 模式拒绝注册 | AC-ACC-01-3 | 后端 e2e | P0 |
| TC-ACC-01-05 | 不合法用户名：太短、太长、数字开头、含空格、含中文 | AC-ACC-01-4 | 后端 e2e | P0 |
| TC-ACC-01-06 | 只有注册不区分大小写：演示用户 Bob 占用 bob，导入 BOB 不受影响 | AC-ACC-01-5 | 后端 e2e | P0 |
| TC-ACC-01-07 | 保留名、已注销的用户名不能注册 | AC-ACC-01-6 | 后端 e2e | P0 |
| TC-ACC-01-08 | 弱密码五种情况都返回 passwordWeak | AC-ACC-01-7 | 后端 e2e | P0 |
| TC-ACC-01-09 | 同一用户名 20 个并发注册，只有一个成功，邀请码只扣一次 | AC-ACC-01-8 | 后端 e2e | P0 |
| TC-ACC-01-10 | 同一 IP 第 21 次注册 429，换 IP 不受影响，两种模式都限 | AC-ACC-01-10 | 后端 e2e | P0 |
| TC-ACC-01-11 | 账号表和日志没有明文密码、恢复码 | AC-ACC-01-11 | 后端 e2e | P0 |
| TC-ACC-01-12 | 注册成功进入保存恢复码页，确认前进不了会话列表 | AC-ACC-01-12 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-01-13 | 服务端返回 4095 时提示在用户名框下方，其他输入保留 | AC-ACC-01-13 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-01-14 | 一个邀请码最多探测 20 个用户名，之后自动停用 | AC-ACC-01-14 | 后端 e2e | P0 |
| TC-ACC-01-15 | 第 1 次注册事务失败，用户不产生，邀请码不扣 | AC-ACC-01-15 | 后端 e2e | P0 |
| TC-ACC-01-16 | 匿名请求占满闸门：refresh、logout、改密不受影响，再来的匿名注册 1 秒后 5030 | AC-ACC-01-16 | 后端 e2e | P0 |
| TC-ACC-01-17 | platform 不合法、deviceName 超长都是 4000 | AC-ACC-01-17 | 后端 e2e | P0 |
| TC-ACC-01-18 | 注册页各种错误的文案，已填内容保留 | AC-ACC-01-18 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-01-19 | 生产环境 open 模式拒绝启动（C6 未上线） | AC-ACC-01-19 | 单测 | P0 |
| TC-ACC-01-20 | open 模式注册 7 天内的账号，每日上传额度 200 MiB | AC-ACC-01-20 | 后端 e2e | P0 |
| TC-ACC-01-21 | 邀请码自动停用按窗口计：跨窗口的 38 次 4095 不停用 | AC-ACC-01-21 | 后端 e2e | P0 |
| TC-ACC-02-01 | 大小写不同的用户名能登录，token 可以连接 | AC-ACC-02-1 | 后端 e2e | P0 |
| TC-ACC-02-02 | 四种登录失败的响应完全相同 | AC-ACC-02-2 | 后端 e2e | P0 |
| TC-ACC-02-03 | 时间侧信道：不存在账号和密码错误的耗时中位数相差不超过 5% | AC-ACC-02-3 | 后端 e2e | P0 |
| TC-ACC-02-04 | 连续 5 次失败后锁定，第 6 次正确密码也 429，15 分钟后恢复 | AC-ACC-02-4 | 后端 e2e | P0 |
| TC-ACC-02-05 | 锁定按"用户名 + IP"，换 IP 用正确密码成功 | AC-ACC-02-5 | 后端 e2e | P0 |
| TC-ACC-02-06 | 封禁用户：密码错 401，密码对 403 带 bannedUntil | AC-ACC-02-6 | 后端 e2e | P0 |
| TC-ACC-02-07 | Android 已登录，再从 iOS 登录，不踢 | AC-ACC-02-7 | 后端 e2e | P0 |
| TC-ACC-02-08 | 4018 的提示：密码框清空，用户名保留 | AC-ACC-02-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-02-09 | 4294 的提示：显示分钟数，不自动重试 | AC-ACC-02-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-02-10 | IP 限流：1 分钟第 21 次 429，IPv6 按 /64 合并 | AC-ACC-02-10 | 后端 e2e | P0 |
| TC-ACC-02-11 | 不存在的用户名同样累计锁定 | AC-ACC-02-11 | 后端 e2e | P0 |
| TC-ACC-02-12 | 误输进用户名框的内容不进日志和 account_events | AC-ACC-02-12 | 后端 e2e | P0 |
| TC-ACC-02-13 | 新设备登录：在线设备 2 秒内收到 securityNotice，账号安全页有记录 | AC-ACC-02-13 | 端到端 | P0 |
| TC-ACC-02-14 | 锁定计数在数据库：重启后仍锁定 | AC-ACC-02-14 | 后端 e2e | P0 |
| TC-ACC-03-01 | 正常刷新：返回新 access 和新 refresh，新 access 能连接 | AC-ACC-03-1 | 后端 e2e | P0 |
| TC-ACC-03-02 | 转正后推进 3 分钟用旧值：重放，吊销整条会话 | AC-ACC-03-2 | 后端 e2e | P0 |
| TC-ACC-03-03 | 待确认代超过 4 个淘汰最旧，会话不吊销；窗口内外用被淘汰的值 | AC-ACC-03-3 | 后端 e2e | P0 |
| TC-ACC-03-04 | refresh token 91 天没用过期；89 天还能用 | AC-ACC-03-4 | 后端 e2e | P0 |
| TC-ACC-03-05 | 管理接口 token 没有 sid，行为和现在一样 | AC-ACC-03-5 | 后端 e2e | P0 |
| TC-ACC-03-06 | 强制下线、封禁后，旧 refresh 都 4019，不是 4034 | AC-ACC-03-6 | 后端 e2e | P0 |
| TC-ACC-03-07 | access 过期、refresh 有效冷启动：先刷新再 auth，不弹登录页 | AC-ACC-03-7 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-08 | refresh 已吊销冷启动：回登录页 | AC-ACC-03-8 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-09 | H5 刷新页面仍登录，新标签页要重新登录，localStorage 没有 token | AC-ACC-03-9 | Playwright（H5） | P0 |
| TC-ACC-03-10 | 连接中剩余不足 6 小时：自动刷新、1000 断开重连、不提示 | AC-ACC-03-10 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-11 | 飞行模式 + access 过期冷启动：显示缓存，不回登录页；恢复网络后连接 | AC-ACC-03-11 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-12 | 刷新返回 503 留在当前页退避；kicked/tokenExpired 且刷新成功不弹窗 | AC-ACC-03-12 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-13 | 新 token 落盘后、使用前杀 App：重启用新 token，不被吊销 | AC-ACC-03-13 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-14 | H5 sessionStorage 被复制：新页面回登录页，原页面不受影响 | AC-ACC-03-14 | Playwright（H5） | P0 |
| TC-ACC-03-15 | H5 登录的 refresh 到期是 24 小时，超过绝对上限 401 | AC-ACC-03-15 | 后端 e2e | P0 |
| TC-ACC-03-16 | 账号用户的无 sid token：改密后拒绝，之后签发的能连；不能调账号接口 | AC-ACC-03-16 | 后端 e2e | P0 |
| TC-ACC-03-17 | 入表前被吊销：连接随后被踢，不留在线 | AC-ACC-03-17 | 后端 e2e | P0 |
| TC-ACC-03-18 | 老版本留下的 Session 按演示处理 | AC-ACC-03-18 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-19 | 响应丢失：5 分钟、1 小时、3 天后用旧值重试都成功 | AC-ACC-03-19 | 后端 e2e | P0 |
| TC-ACC-03-20 | 刷新请求发出后杀 App：5 分钟后用旧值重试成功 | AC-ACC-03-20 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-03-21 | 同一 R1 两个并发：都不吊销，先用的转正，另一个作废 | AC-ACC-03-21 | 后端 e2e | P0 |
| TC-ACC-03-22 | 1 分钟连续重试 3 次，用最新的转正；3 分钟后再用 R1 按重放 | AC-ACC-03-22 | 后端 e2e | P0 |
| TC-ACC-03-23 | 四个入口合并成一次刷新，迟到响应丢弃 | AC-ACC-03-23 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-24 | 用新 access auth 也算转正，之后 R1 按重放 | AC-ACC-03-24 | 后端 e2e | P0 |
| TC-ACC-03-25 | 被盗场景：攻击者用偷来的 R1 重发并使用，受害者上线触发吊销 | AC-ACC-03-25 | 后端 e2e | P0 |
| TC-ACC-03-26 | replayDetected：回登录页前弹窗提示 | AC-ACC-03-26 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-27 | session_purge 清除过期 31 天的会话 | AC-ACC-03-27 | 后端 e2e | P0 |
| TC-ACC-03-28 | 被淘汰的代的 access：WS 4011、HTTP 4011 tokenSuperseded，不转正 | AC-ACC-03-28 | 后端 e2e | P0 |
| TC-ACC-03-29 | 过期或验签失败的 token 不触发转正 | AC-ACC-03-29 | 后端 e2e | P0 |
| TC-ACC-03-30 | 攻击者转正，在线的受害者立刻被踢；窗口内 superseded，窗口后重放 | AC-ACC-03-30 | 后端 e2e | P0 |
| TC-ACC-03-31 | 乱序到达：Q2 先得 R3、Q1 后得 R4，窗口内外的处理 | AC-ACC-03-31 | 后端 e2e | P0 |
| TC-ACC-03-32 | 只知道 sid 吊销不了会话 | AC-ACC-03-32 | 后端 e2e | P0 |
| TC-ACC-03-33 | sid 原值不进日志和后台响应 | AC-ACC-03-33 | 后端 e2e | P0 |
| TC-ACC-03-34 | 离线受害者遇到攻击者改密：上线都看到找回截止日期 | AC-ACC-03-34 | 端到端 | P0 |
| TC-ACC-03-35 | 新 IP 前缀写事件；账号安全有其他会话数量和最近活跃时间 | AC-ACC-03-35 | 后端 e2e | P0 |
| TC-ACC-03-36 | 入表后复查数据库报错：1011 断开，SDK 重连，不回登录页 | AC-ACC-03-36 | 后端 e2e | P0 |
| TC-ACC-03-37 | 周期复查：丢失的踢线下一次复查补上；连续失败 3 次才断开 | AC-ACC-03-47 | 后端 e2e | P0 |
| TC-ACC-03-38 | 攻击者用满签发次数压制不了受害者的重放判定 | AC-ACC-03-48 | 后端 e2e | P0 |
| TC-ACC-03-39 | 第 21 次重发 4290 不回登录页；100 天不转正到期；新 access 不超过绝对上限 | AC-ACC-03-37 | 端到端 | P0 |
| TC-ACC-03-40 | 连续转正不能续开窗口：受害者的旧值马上重放 | AC-ACC-03-38 | 后端 e2e | P0 |
| TC-ACC-03-41 | superseded 重试：按 retryAfter 重试，3 次后回登录页 | AC-ACC-03-39 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-42 | 代号不复用：已作废的 access 不会复活 | AC-ACC-03-40 | 后端 e2e | P0 |
| TC-ACC-03-43 | 校验 gen=1 后暂停，另一请求转正：入表后复查发现不符 | AC-ACC-03-41 | 后端 e2e | P0 |
| TC-ACC-03-44 | 在途请求带旧 token 得到 4011：用本地新 token 重试，不多发刷新 | AC-ACC-03-42 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-45 | access 到期等于绝对上限：不再刷新，到期回登录页 | AC-ACC-03-43 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-03-46 | 已吊销的会话遇旧值不算重放；logout 带被替换的 Bearer 和有效 refresh 能退出 | AC-ACC-03-44 | 后端 e2e | P0 |
| TC-ACC-03-47 | 连续 6 小时刷满：有事件，其他在线设备收到通知 | AC-ACC-03-45 | 后端 e2e | P0 |
| TC-ACC-03-48 | refresh 密钥轮换后旧 token 可用；提前删旧密钥配置检查报错 | AC-ACC-03-46 | 后端 e2e | P0 |
| TC-ACC-04-01 | D1 退出：D1 的 refresh、access 都失效，D2 不受影响 | AC-ACC-04-1 | 后端 e2e | P0 |
| TC-ACC-04-02 | D1 改密：换新 sid 1 秒内重连；D2 被踢；两边旧 refresh 都 credentialsChanged | AC-ACC-04-2 | 后端 e2e | P0 |
| TC-ACC-04-03 | 旧密码错误：401，什么都不变，计入失败次数 | AC-ACC-04-3 | 后端 e2e | P0 |
| TC-ACC-04-04 | 强制下线吊销全部设备会话 | AC-ACC-04-4 | 后端 e2e | P0 |
| TC-ACC-04-05 | D2 收到 passwordChanged：不重连，弹窗显示日期，确定后清空回登录页 | AC-ACC-04-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-04-06 | 断网退出：本地立即退出，联网后补发 logout | AC-ACC-04-6 | 原生 UI 测试（Android、iOS） | P0 |
| TC-ACC-04-07 | 吊销写库失败：改密整体回滚，旧密码仍有效 | AC-ACC-04-7 | 后端 e2e | P0 |
| TC-ACC-04-08 | D2 刷新和 D1 改密并发：不留下改密后还有效的 D2 会话 | AC-ACC-04-8 | 后端 e2e | P0 |
| TC-ACC-04-09 | 退出其他设备：密码错 401；密码对踢 D2，D1 换 sid 重连，密码不变 | AC-ACC-04-10 | 端到端 | P0 |
| TC-ACC-04-10 | logout 重复调用都成功 | AC-ACC-04-11 | 后端 e2e | P0 |
| TC-ACC-04-11 | 新密码不合规：400，密码和恢复码都不变 | AC-ACC-04-12 | 后端 e2e | P0 |
| TC-ACC-04-12 | 旧码保留 7 天：第 6 天成功、第 8 天失败；旧码优先 | AC-ACC-04-13 | 后端 e2e | P0 |
| TC-ACC-04-13 | 待退出：只有 refresh 也能退出，旧代值不算重放 | AC-ACC-04-14 | 后端 e2e | P0 |
| TC-ACC-04-14 | 攻击者和 D1 共用 sid：D1 改密把攻击者清除 | AC-ACC-04-15 | 后端 e2e | P0 |
| TC-ACC-04-15 | 改密响应丢失：旧 refresh 不算重放，按规则 6a 回登录页 | AC-ACC-04-16 | 端到端 | P0 |
| TC-ACC-04-16 | 定时刷新和改密同时触发：串行，不误吊销 | AC-ACC-04-17 | SDK 单测（三套） | P0 |
| TC-ACC-04-17 | 登录在 Argon2 后暂停，recover 提交后放行：登录 401 | AC-ACC-04-18 | 后端 e2e | P0 |
| TC-ACC-04-18 | 改密在 Argon2 后暂停，recover 提交后放行：改密 401，攻击者没有恢复码 | AC-ACC-04-19 | 后端 e2e | P0 |
| TC-ACC-04-19 | 同一恢复码两个并发 recover，恰好一个成功 | AC-ACC-04-20 | 后端 e2e | P0 |
| TC-ACC-04-20 | 撤销注销和在途 delete：delete 401，清理不处理 | AC-ACC-04-21 | 后端 e2e | P1 |
| TC-ACC-04-21 | 所有校验密码的接口共用失败计数 | AC-ACC-04-22 | 后端 e2e | P0 |
| TC-ACC-04-22 | 攻击者循环改密压制不了受害者找回 | AC-ACC-04-23 | 后端 e2e | P0 |
| TC-ACC-04-23 | 锁定和哈希并发都挡不住正确恢复码；错误的仍受限制 | AC-ACC-04-24 | 后端 e2e | P0 |
| TC-ACC-04-24 | 四条序列都带 recoverBefore，都是第 7 天 | AC-ACC-04-25 | 端到端 | P0 |
| TC-ACC-04-25 | 改密的四种响应时序：踢线缓存、超时、顺序 | AC-ACC-04-27 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-ACC-04-26 | delete/cancel 暂停后 recover 再申请注销：撤销 401，新申请有效 | AC-ACC-04-28 | 后端 e2e | P1 |
| TC-ACC-04-27 | 【未执行】Postgres：recover 持锁期间，三种请求都等待后 401 | AC-ACC-04-29 | 后端 e2e（Postgres 并发） | P0 |
| TC-ACC-04-28 | 专用位公平：攻击者循环持新码，受害者和其他账号都在时限内成功 | AC-ACC-04-30 | 后端 e2e | P0 |
| TC-ACC-04-29 | 成功次数上限只约束最新一代，受害者手里的旧代不受限 | AC-ACC-04-31 | 后端 e2e | P0 |
| TC-ACC-05-01 | 正确恢复码找回：新密码可用，旧设备下线，旧码作废 | AC-ACC-05-1 | 后端 e2e | P0 |
| TC-ACC-05-02 | 用户名不存在、恢复码错、已用过：响应相同 | AC-ACC-05-2 | 后端 e2e | P0 |
| TC-ACC-05-03 | 恢复码错误 5 次后再错 429；正确的不受限 | AC-ACC-05-3 | 后端 e2e | P0 |
| TC-ACC-05-04 | 找回成功：显示新恢复码，确认后进入会话列表 | AC-ACC-05-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-05-05 | 注册响应丢失：登录时发新码，注册时的码失效，确认后不再带 | AC-ACC-05-6 | 后端 e2e | P0 |
| TC-ACC-05-06 | 在保存页杀 App：再次进入保存页，新码，确认前 token 没有持久化 | AC-ACC-05-7 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-05-07 | 新密码不合规、账号封禁：恢复码和密码都不变 | AC-ACC-05-8 | 后端 e2e | P0 |
| TC-ACC-05-08 | 后台没有任何改密码的接口 | AC-ACC-05-9 | 单测 + 后端 e2e | P0 |
| TC-ACC-05-09 | 三个页面都有"丢失后无法找回"，没有客服入口 | AC-ACC-05-10 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-05-10 | 时间侧信道：恢复码错误和账号不存在，中位数相差不超过 5% | AC-ACC-05-11 | 后端 e2e | P0 |
| TC-ACC-05-11 | 重新生成恢复码：要密码；第 6 天旧码可用，第 8 天不可用 | AC-ACC-05-12 | 后端 e2e | P0 |
| TC-ACC-05-12 | 改密响应丢失：用新密码登录拿到更新的码，旧已确认码还有效 | AC-ACC-05-13 | 后端 e2e | P0 |
| TC-ACC-05-13 | 两台设备的确认：旧 gen 不算确认 | AC-ACC-05-14 | 后端 e2e | P0 |
| TC-ACC-05-14 | recoveryCodeStale：提示后回登录页 | AC-ACC-05-15 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-05-15 | 攻击者改密后申请注销：第 3 天旧码找回，全部撤销 | AC-ACC-05-16 | 后端 e2e | P1 |
| TC-ACC-05-16 | 攻击者用新码找回再改密：受害者第 6 天旧码清除一切；第 8 天不延长 | AC-ACC-05-17 | 后端 e2e | P0 |
| TC-ACC-05-17 | 匿名占满闸门：错误码快速 401，正确码走专用位 | AC-ACC-05-18 | 后端 e2e | P0 |
| TC-ACC-05-18 | 最新一代未确认：已登录的强制进保存页；改密后杀 App 回登录页 | AC-ACC-05-19 | 端到端 | P0 |
| TC-ACC-06-01 | 改昵称后其他人能查到 | AC-ACC-06-1 | 后端 e2e | P0 |
| TC-ACC-06-02 | 昵称超长、空白、边界 | AC-ACC-06-2 | 后端 e2e | P0 |
| TC-ACC-06-03 | avatarFileId：avatar 成功；file 和别人的 404 | AC-ACC-06-3 | 后端 e2e | P0 |
| TC-ACC-06-04 | 头像类型和大小限制 | AC-ACC-06-4 | 后端 e2e | P0 |
| TC-ACC-06-05 | 改昵称：本端立即显示，其他端 10 分钟后更新 | AC-ACC-06-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-06-06 | 选图、裁剪、保存：上传 512×512 | AC-ACC-06-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-06-07 | 头像的私有和公开：绑定前不公开，绑定后可读，不能列举 | AC-ACC-06-7 | 后端 e2e | P0 |
| TC-ACC-06-08 | 已绑定头像不受 24 小时清理；换头像 8 天后旧对象删除 | AC-ACC-06-8 | 后端 e2e | P0 |
| TC-ACC-06-09 | 对象存储不可用：昵称可改，头像 503 | AC-ACC-06-9 | 后端 e2e | P0 |
| TC-ACC-06-10 | 未绑定头像 25 小时后清除 | AC-ACC-06-10 | 后端 e2e | P0 |
| TC-ACC-06-11 | 绑定幂等、头像不过期、5 天内绑回不删 | AC-ACC-06-11 | 后端 e2e | P0 |
| TC-ACC-06-12 | 公开 Copy 后杀进程：重试只有一个对象；换头像、注销清除三处 | AC-ACC-06-12 | 后端 e2e | P0 |
| TC-ACC-06-13 | 公开 Copy 后杀进程不重试：25 小时后三处都清除 | AC-ACC-06-13 | 后端 e2e | P0 |
| TC-ACC-07-01 | 演示模式下现有 e2e 不改代码全过 | AC-ACC-07-1 | 后端 e2e | P0 |
| TC-ACC-07-02 | 演示用户不能密码登录；账号用户不能演示登录 | AC-ACC-07-2 | 后端 e2e | P0 |
| TC-ACC-07-03 | 生产带 token.issue：不确认就不启动；确认后告警 | AC-ACC-07-3 | 后端 e2e + 单测 | P0 |
| TC-ACC-07-04 | GET /v1/config 和配置一致，没有密钥 | AC-ACC-07-4 | 后端 e2e | P0 |
| TC-ACC-07-05 | demoLogin=false、invite：账号登录表单，没有演示表单和"业务方" | AC-ACC-07-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-07-06 | demoLogin=true：演示表单照常 | AC-ACC-07-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-07-07 | 构建开关是上限：任一为假都不显示演示表单 | AC-ACC-07-7 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-07-08 | 演示会话没有"账号安全" | AC-ACC-07-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-07-09 | closed 没有注册入口；config 失败时不猜测模式 | AC-ACC-07-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-08-01 | 注销全流程：错密码、踢线、冷静期、到期 | AC-ACC-08-1 | 后端 e2e | P1 |
| TC-ACC-08-02 | 注销到期后：历史还在，发消息和签发 404 | AC-ACC-08-2 | 后端 e2e | P1 |
| TC-ACC-08-03 | 群主注销：自建群转让，托管群回到无群主 | AC-ACC-08-3 | 后端 e2e | P1 |
| TC-ACC-08-04 | 撤销注销：匿名接口，错密码 401，对后恢复 | AC-ACC-08-4 | 后端 e2e | P1 |
| TC-ACC-08-05 | 600 个群的清理：中途失败可继续，不影响其他用户消息 | AC-ACC-08-5 | 后端 e2e + 性能 | P1 |
| TC-ACC-08-06 | 注销确认框的文字；密码错误原地提示 | AC-ACC-08-6 | Playwright（H5）+ 原生 UI 测试 | P1 |
| TC-ACC-08-07 | 注销清理删除头像对象 | AC-ACC-08-7 | 后端 e2e | P1 |
| TC-ACC-09-01 | 运营生成 maxUses=2，三个用户注册，后台显示已用 2 次 | AC-ACC-09-1 | 端到端 | P0 |
| TC-ACC-09-02 | 生成邀请码：库、审计、日志都没有明文 | AC-ACC-09-2 | 后端 e2e | P0 |
| TC-ACC-09-03 | 已停用的邀请码不能用 | AC-ACC-09-5 | 后端 e2e | P0 |
| TC-ACC-09-04 | 权限：support、auditor 403，operator 成功 | AC-ACC-09-6 | 后端 e2e | P0 |
| TC-ACC-09-05 | 单次 101 个、maxUses=1001 被拒；100、1000 的边界 | AC-ACC-09-7 | 后端 e2e | P0 |
| TC-ACC-09-06 | 审计写失败：邀请码不产生 | AC-ACC-09-8 | 后端 e2e | P0 |
| TC-ACC-09-07 | maxUses=1，10 个并发注册只成功 1 个 | AC-ACC-09-9 | 后端 e2e | P0 |
| TC-ACC-09-08 | 邀请码至少 128 位随机 | AC-ACC-09-10 | 单测 | P0 |
| TC-ACC-09-09 | 用户详情显示账号信息，没有重置密码 | AC-ACC-09-11 | Playwright（tianting-admin） | P0 |
| TC-ACC-09-10 | 用完 91 天的邀请码被清除 | AC-ACC-09-12 | 后端 e2e | P0 |
| TC-ACC-09-11 | 账号信息接口：没有哈希和令牌；演示用户显示没有账号 | AC-ACC-09-13 | 后端 e2e | P0 |
| TC-ACC-09-12 | 邀请总量超过 500 告警 | AC-ACC-09-14 | 后端 e2e | P0 |
| TC-ACC-10-01 | 三个入口都能打开隐私说明；加载失败的提示 | AC-ACC-10-1 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-10-02 | 注册记录同意的版本 | AC-ACC-10-2 | 后端 e2e | P0 |
| TC-ACC-10-03 | 版本更新：下次登录提示一次 | AC-ACC-10-3 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-ACC-03-49 | 吊销之后 refresh 的 reason 按原因返回 | AC-ACC-03-49 | 后端 e2e | P0 |
| TC-ACC-03-50 | 恢复数据库后吊销全部会话 | AC-ACC-03-50 | 后端 e2e | P0 |

---

## F-ACC-01 注册

### TC-ACC-01-01 邀请码模式注册成功，用户名规范化为小写，token 能通过 WS auth
- 关联 AC：AC-ACC-01-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_REGISTER_MODE=invite`；运营生成有效邀请码 I1（maxUses=1）
- 步骤：
  1. `POST /v1/account/register {username:"Alice_1", password:"Tt-pass-2026!", inviteCode:I1, platform:"android", deviceName:"Pixel"}`
  2. 用响应的 `accessToken` 连 `/ws` 发 `auth`
  3. 用响应的 `recoveryCode`、`recoveryCodeGen` 调 `POST /v1/account/recovery-code/ack {gen}`
- 预期结果：
  1. 第 1 步 200，`userId=alice_1`，带 `accessToken`、`accessExpiresAt`、`refreshToken`、`refreshExpiresAt`、`recoveryCode`、`recoveryCodeGen`
  2. 第 2 步 `auth` 成功，ack 的 userId 是 `alice_1`
  3. 第 3 步成功；`accounts.username_lower=alice_1`，邀请码 `usedCount=1`

### TC-ACC-01-02 边界：用户名 4 位、32 位，密码 8、128 个码点都能注册
- 关联 AC：AC-ACC-01-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`open` 模式（测试环境）
- 步骤：
  1. 分别注册 `abcd`、32 位的 `a` + 31 个 `b`，密码分别取 8 个码点、128 个码点（含中文）
- 预期结果：
  1. 都成功；密码按码点计数，不按字节

### TC-ACC-01-03 四种无效邀请码的应答逐字节相同，不暴露用户名是否存在
- 关联 AC：AC-ACC-01-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`invite` 模式；已注册 `taken1`；I2 过期时间为 T+1 小时；I3 maxUses=1 且已用完；每次请求换 IP
- 步骤：
  1. 用用户名 `taken1` 依次注册：不带 `inviteCode`；`inviteCode=不存在的值`；把 [IM 钟] 推进 2 小时后带 I2；带 I3
  2. 用未占用的用户名 `free1` 重复这 4 种
- 预期结果：
  1. 8 次都是 403 / 4030，`reason=inviteInvalid`；状态码、响应体、响应头名集合完全相同
  2. 用户表没有 `free1`，邀请码计数不变

### TC-ACC-01-04 closed 模式拒绝注册
- 关联 AC：AC-ACC-01-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_REGISTER_MODE=closed`；有一个有效邀请码
- 步骤：
  1. 分别带、不带邀请码注册 `closed1`
- 预期结果：
  1. 都是 403 / 4030，`reason=registerClosed`；`users`、`accounts` 没有 `closed1`

### TC-ACC-01-05 不合法用户名：太短、太长、数字开头、含空格、含中文
- 关联 AC：AC-ACC-01-4
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`open` 模式
- 步骤：
  1. 分别注册 `abc`、33 位字母、`1abc`、`a b c`、`张三abc`
  2. 再注册 `  Valid_User  `（首尾空白）
- 预期结果：
  1. 前 5 个都是 400 / 4004，用户未创建
  2. 最后一个成功，`userId=valid_user`（去空白后判断）

### TC-ACC-01-06 只有注册不区分大小写：演示用户 Bob 占用 bob，导入 BOB 不受影响
- 关联 AC：AC-ACC-01-5
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`open` 模式，`/demo/login {username:"Bob"}` 产生用户 `Bob`
- 步骤：
  1. 注册 `bob`
  2. 用管理接口导入用户 `BOB`
  3. 查 `users` 表
- 预期结果：
  1. 第 1 步 409 / 4095
  2. 第 2 步成功；`Bob`、`BOB` 并存，TC-USER-01-16 的行为不变

### TC-ACC-01-07 保留名、已注销的用户名不能注册
- 关联 AC：AC-ACC-01-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`open` 模式；保留名配置含 `admin`；`gone1` 已注销且清理完成（[IM 钟] 推进 8 天后运行 `account_cleanup`）
- 步骤：
  1. 注册 `admin`、`ADMIN`、`gone1`
- 预期结果：
  1. 都是 409 / 4095，响应相同

### TC-ACC-01-08 弱密码五种情况都返回 passwordWeak
- 关联 AC：AC-ACC-01-7
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`open` 模式；弱口令表含 `password1`
- 步骤：
  1. 用户名 `weak_user`，依次用密码：7 个码点、129 个码点、`weak_user`、`aaaaaaaa`、`password1`
- 预期结果：
  1. 都是 400 / 4000，`reason=passwordWeak`；用户未创建

### TC-ACC-01-09 同一用户名 20 个并发注册，只有一个成功，邀请码只扣一次
- 关联 AC：AC-ACC-01-8
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`invite` 模式；邀请码 maxUses=20；Argon2 闸门用默认值
- 步骤：
  1. 20 个连接同时注册 `race1`，带同一邀请码
  2. 统计结果，查 `accounts`、邀请码
- 预期结果：
  1. 恰好 1 个 200，其余是 4095 或 5030
  2. `accounts` 只有 1 行，`usedCount=1`

### TC-ACC-01-10 同一 IP 第 21 次注册 429，换 IP 不受影响，两种模式都限
- 关联 AC：AC-ACC-01-10
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_TRUSTED_PROXIES` 配测试客户端；`X-Forwarded-For: 203.0.113.1`；先后在 `open`、`invite` 模式各跑一次，中间 `limits/reset scope=auth`
- 步骤：
  1. 同一 IP 发 20 次注册（用户名不合法也算），第 21 次用合法信息
  2. 换 `X-Forwarded-For: 203.0.113.2` 再注册
- 预期结果：
  1. 第 21 次 429 / 4290；换 IP 成功

### TC-ACC-01-11 账号表和日志没有明文密码、恢复码
- 关联 AC：AC-ACC-01-11
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：密码用特征串 `ZXQPW-1a2b`
- 步骤：
  1. 注册并记下 `recoveryCode`
  2. 查 `accounts`、`recovery_codes`；扫描服务端全部日志
- 预期结果：
  1. `password_hash` 以 `$argon2id$` 开头，参数 m=64 MiB、t=3；`recovery_codes.hash` 不等于明文
  2. 库和日志都搜不到密码特征串和恢复码明文

### TC-ACC-01-12 注册成功进入保存恢复码页，确认前进不了会话列表
- 关联 AC：AC-ACC-01-12
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：`invite` 模式，三端各一次
- 步骤：
  1. 打开注册页，填合法信息提交
  2. 查看保存页；点"复制"；尝试返回（Android 返回键、iOS 手势、H5 后退）
  3. 点"我已保存"
- 预期结果：
  1. 显示恢复码、"复制"按钮、"恢复码丢失后账号无法找回"
  2. 只有一个"我已保存"按钮；返回操作不进入会话列表
  3. 点击后发 `recovery-code/ack`，进入会话列表

### TC-ACC-01-13 服务端返回 4095 时提示在用户名框下方，其他输入保留
- 关联 AC：AC-ACC-01-13
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：用假接口或已占用的用户名让注册返回 4095
- 步骤：
  1. 填用户名、密码、邀请码后提交
- 预期结果：
  1. 用户名框下方显示"这个用户名不能使用"；密码、邀请码还在；留在注册页

### TC-ACC-01-14 一个邀请码最多探测 20 个用户名，之后自动停用
- 关联 AC：AC-ACC-01-14
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`invite` 模式；邀请码 I1（maxUses=1）；已注册 `taken01`～`taken20`；告警接收用测试替身
- 步骤：
  1. 用户 A（探测者）持 I1，每次换一个 `X-Forwarded-For`，依次注册 `taken01`～`taken20`
  2. A 再用未占用的 `free2` 和 I1 注册
  3. 查后台邀请码状态和告警
- 预期结果：
  1. 前 20 次 409 / 4095，I1 没有被扣减
  2. 第 21 次 403 / 4030，`reason=inviteInvalid`，用户未创建
  3. 邀请码显示"已停用（自动）"，产生一条告警

### TC-ACC-01-15 第 1 次注册事务失败，用户不产生，邀请码不扣
- 关联 AC：AC-ACC-01-15
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`POST /admin/test/faults` 注入"注册事务失败"一次；邀请码 maxUses=1
- 步骤：
  1. 注册 `txfail1`
  2. 不改任何参数再注册一次
- 预期结果：
  1. 第 1 次 500 / 5000，`users`、`accounts` 没有行，`usedCount=0`
  2. 第 2 次成功，`usedCount=1`

### TC-ACC-01-16 匿名请求占满闸门：refresh、logout、改密不受影响，再来的匿名注册 1 秒后 5030
- 关联 AC：AC-ACC-01-16
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：已登录账号 u1；Argon2 阻塞钩子打开
- 步骤：
  1. 4 个匿名注册（不同用户名、不同 IP）进入闸门后被钩子停住，确认 4 个位占满
  2. 同时发 u1 的 `token/refresh`、u1 另一设备的 `logout`、u1 的 `password`（已登录改密）
  3. 再发第 5 个匿名注册，记录耗时
  4. 关闭钩子
- 预期结果：
  1. refresh、logout 不等闸门，立即返回；改密使用专用位，成功
  2. 第 5 个注册约 1 秒后 503 / 5030（在 0.9～1.5 秒内）
  3. 关闭钩子后前 4 个注册正常完成

### TC-ACC-01-17 platform 不合法、deviceName 超长都是 4000
- 关联 AC：AC-ACC-01-17
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `platform=tv` 注册
  2. `deviceName` 65 个码点注册
  3. 边界：`deviceName` 64 个码点注册；不传 `deviceName` 注册
- 预期结果：
  1. 前两个 400 / 4000；后两个成功

### TC-ACC-01-18 注册页各种错误的文案，已填内容保留
- 关联 AC：AC-ACC-01-18
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：用假接口依次返回 4004、4000（`passwordWeak`）、4030（`inviteInvalid`）、4290、5030；再断网
- 步骤：
  1. 每种情况提交一次，每次都先填好全部内容
- 预期结果：
  1. 依次显示"用户名格式不正确""密码太简单""邀请码无效""操作太频繁，请稍后再试""服务暂不可用""网络异常，请稍后重试"
  2. 已填内容都在；按钮从"提交中"恢复可点

### TC-ACC-01-19 生产环境 open 模式拒绝启动（C6 未上线）
- 关联 AC：AC-ACC-01-19
- 优先级：P0
- 批次：C1
- 层级：单测
- 环境：单测
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `IM_ENV=production`、`IM_REGISTER_MODE=open`，其他配置齐全，启动 im-server
  2. 同样配置改为 `invite` 再启动
- 预期结果：
  1. 第 1 次非 0 退出，错误信息指出开放注册的条件不满足
  2. 第 2 次通过这一项检查

### TC-ACC-01-20 open 模式注册 7 天内的账号，每日上传额度 200 MiB
- 关联 AC：AC-ACC-01-20
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：`open` 模式（测试环境）；`IM_FILES_ENABLED=1`；u3 注册后把 [IM 钟] 推进 3 天；u8 注册后推进 8 天，时钟从 Asia/Shanghai 当天 00:00 起算
- 步骤：
  1. 用 `seed` 把 u3、u8 当日已用都设为 150 MiB
  2. u3 申请 `kind=file`、`size=60 MiB`（单个文件不能超过 100 MiB，所以用已用量凑超额）
  3. u8 申请同样大小
- 预期结果：
  1. u3 429 / 4292；u8 成功拿到 `put`
- 说明：`invite` 模式没有这条限制，归入 AC-FILE-01-4 的默认额度

### TC-ACC-01-21 邀请码自动停用按窗口计：跨窗口的 38 次 4095 不停用
- 关联 AC：AC-ACC-01-21
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`invite` 模式；邀请码 I1（maxUses=1000）；已注册 `taken01`～`taken19`；每次请求换 IP
- 步骤：
  1. 用 I1 注册 `taken01`～`taken19`（19 次 4095）
  2. 把 [IM 钟] 推进 25 小时，再重复 19 次
  3. 用 I1 注册未占用的 `free3`
- 预期结果：
  1. 38 次都是 409 / 4095，I1 没有被停用
  2. 最后一次成功

## F-ACC-02 登录

### TC-ACC-02-01 大小写不同的用户名能登录，token 可以连接
- 关联 AC：AC-ACC-02-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：已注册 `alice_1`
- 步骤：
  1. `POST /v1/account/login {username:"ALICE_1", password:正确, platform:"ios"}`
  2. 用 token `auth`
- 预期结果：
  1. 200，`userId=alice_1`，响应不带 `recoveryCode`（最新一代已确认）；`auth` 成功

### TC-ACC-02-02 四种登录失败的响应完全相同
- 关联 AC：AC-ACC-02-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：演示用户 `demo1`（无账号记录）；已注销用户 `gone1`；账号 `real1`；每种用不同用户名，避免触发锁定
- 步骤：
  1. 分别登录：不存在的 `nouser1`；`demo1`；`gone1`；`real1` + 错误密码
- 预期结果：
  1. 都是 401 / 4018，状态码、响应体、响应头名集合相同

### TC-ACC-02-03 时间侧信道：不存在账号和密码错误的耗时中位数相差不超过 5%
- 关联 AC：AC-ACC-02-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_TEST_DISABLE_AUTH_LOCK=1`）
- 前置条件：隔离的测试环境，无其他负载，`IM_TEST_DISABLE_AUTH_LOCK=1`
- 步骤：
  1. 交替发"不存在账号"和"存在账号密码错误"登录，各 50 次，记录每次耗时
- 预期结果：
  1. 两组中位数相差 ≤ 5%；都是 4018
- 说明：和 AC-CON-03-3 同口径；渐进延迟在锁定关闭时也关闭

### TC-ACC-02-04 连续 5 次失败后锁定，第 6 次正确密码也 429，15 分钟后恢复
- 关联 AC：AC-ACC-02-4
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：账号 u1；IP 固定；[IM 钟] 冻结
- 步骤：
  1. 同一用户名 + IP 错误密码登录 5 次
  2. 第 6 次用正确密码
  3. 推进 14 分 59 秒，正确密码再试
  4. 再推进 2 秒，正确密码再试
- 预期结果：
  1. 前 5 次 401 / 4018；第 6 次 429 / 4294，带 `details.retryAfterSeconds`
  2. 第 3 步仍 429；第 4 步成功

### TC-ACC-02-05 锁定按"用户名 + IP"，换 IP 用正确密码成功
- 关联 AC：AC-ACC-02-5
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 被 IP-A 用 5 次错误密码锁定
- 步骤：
  1. 从 IP-B（`X-Forwarded-For` 换值）用正确密码登录 u1
- 预期结果：
  1. 成功；IP-A 仍然 429

### TC-ACC-02-06 封禁用户：密码错 401，密码对 403 带 bannedUntil
- 关联 AC：AC-ACC-02-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：u1 被管理接口封禁到 T+7 天
- 步骤：
  1. u1 用错误密码登录
  2. u1 用正确密码登录
- 预期结果：
  1. 第 1 步 401 / 4018（和普通失败相同，不暴露封禁）
  2. 第 2 步 403 / 4034，`details.bannedUntil`=T+7 天，没有 token

### TC-ACC-02-07 Android 已登录，再从 iOS 登录，不踢
- 关联 AC：AC-ACC-02-7
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. u1 `platform=android` 登录并连接
  2. u1 `platform=ios` 登录并连接
  3. u2 给 u1 发一条消息
- 预期结果：
  1. 两条连接都在线，都收到消息；`device_sessions` 有两行，`sid` 不同

### TC-ACC-02-08 4018 的提示：密码框清空，用户名保留
- 关联 AC：AC-ACC-02-8
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 输入用户名和错误密码，提交
- 预期结果：
  1. 提示"用户名或密码错误"；密码框为空；用户名还在

### TC-ACC-02-09 4294 的提示：显示分钟数，不自动重试
- 关联 AC：AC-ACC-02-9
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：假接口返回 429 / 4294，`retryAfterSeconds=600`
- 步骤：
  1. 提交登录，等待 5 秒
- 预期结果：
  1. 提示"尝试次数过多，请 10 分钟后再试"；网络记录只有 1 个登录请求

### TC-ACC-02-10 IP 限流：1 分钟第 21 次 429，IPv6 按 /64 合并
- 关联 AC：AC-ACC-02-10
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：[IM 钟] 冻结；`IM_TRUSTED_PROXIES` 已配
- 步骤：
  1. 同一 IPv4 用 21 个不同用户名登录
  2. `limits/reset scope=auth` 后，`2001:db8:1:1::1`～`::21` 共 21 个地址各登录 1 次（同一 /64）
  3. 第 22 次用 `2001:db8:1:2::1`（另一个 /64）
- 预期结果：
  1. 第 1、2 步的第 21 次都是 429 / 4290
  2. 第 3 步不受影响（4018）

### TC-ACC-02-11 不存在的用户名同样累计锁定
- 关联 AC：AC-ACC-02-11
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 同一 IP 用不存在的 `nouser2` 登录 6 次
  2. 同一 IP 用已存在的 `real2` + 错误密码登录 6 次
- 预期结果：
  1. 两组前 5 次 401 / 4018，第 6 次都是 429 / 4294，响应结构相同

### TC-ACC-02-12 误输进用户名框的内容不进日志和 account_events
- 关联 AC：AC-ACC-02-12
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 用户名 `ZXQ-USER-1`（不合法也算）、错误密码登录
  2. 扫描服务端日志；查 `account_events`、`auth_throttle`
- 预期结果：
  1. 都搜不到 `ZXQ-USER-1`；`auth_throttle` 的键是哈希

### TC-ACC-02-13 新设备登录：在线设备 2 秒内收到 securityNotice，账号安全页有记录
- 关联 AC：AC-ACC-02-13
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：u1 在 D1（H5、Android、iOS 三端各跑一次）在线
- 步骤：
  1. u1 用 sdk-ts 脚本从新设备 D2 登录（`platform=android`）
  2. D1 观察；打开"账号安全 → 最近登录记录"；`GET /v1/account/events`
- 预期结果：
  1. D1 2 秒内收到 `securityNotice {kind, at, platform:"android"}`，界面提示"你的账号在新设备登录"
  2. 列表和接口都有这条记录，响应没有 IP 全文、`sid` 原值

### TC-ACC-02-14 锁定计数在数据库：重启后仍锁定
- 关联 AC：AC-ACC-02-14
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 被 5 次失败锁定
- 步骤：
  1. `kill -TERM` im-server，用同一数据库重启，同一 [IM 钟] 时间
  2. 同一用户名 + IP 用正确密码登录
- 预期结果：
  1. 429 / 4294；`auth_throttle` 有这行

## F-ACC-03 会话续期与设备会话

### TC-ACC-03-01 正常刷新：返回新 access 和新 refresh，新 access 能连接
- 关联 AC：AC-ACC-03-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 登录得到 (A1, R1)
- 步骤：
  1. `refresh(R1)`
  2. `auth(A2)`
- 预期结果：
  1. 第 1 步 200，得到 A2、R2，带 `accessExpiresAt`、`refreshExpiresAt`、`absoluteExpiresAt`
  2. 第 2 步成功；A2 的 `gen` 成为 `current`

### TC-ACC-03-02 转正后推进 3 分钟用旧值：重放，吊销整条会话
- 关联 AC：AC-ACC-03-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 登录得到 (A1, R1)；[IM 钟] 冻结在 T
- 步骤：
  1. u1：`refresh(R1)` 得到 (A2, R2)
  2. u1：`refresh(R2)` 得到 (A3, R3)，R2 转正，`overlapUntil`=T+2 分钟
  3. 把 [IM 钟] 推进 3 分钟
  4. 重放者：`refresh(R1)`
  5. u1：`refresh(R2)`；`auth(A3)`
- 预期结果：
  1. 第 4 步 401 / 4019，`reason=replayDetected`；`im_refresh_replay_total` +1
  2. 第 5 步 refresh 401 / 4019（`mac` 正确，reason 不再是重放）；`auth` 收到 `kicked/sessionRevoked`，关闭码 4012

### TC-ACC-03-03 待确认代超过 4 个淘汰最旧，会话不吊销；窗口内外用被淘汰的值
- 关联 AC：AC-ACC-03-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 登录得到 R1；[IM 钟] 冻结在 T
- 步骤：
  1. u1：`refresh(R1)` 得到 R2；再 `refresh(R1)` 4 次得到 R3、R4、R5、R6（待确认代超过 4 个，R2 被淘汰）
  2. 查会话：`revoked_at` 为空，`pendings` 是 R3～R6
  3. u1：`refresh(R6)` 得到 R7，R6 转正，窗口到 T+2 分钟
  4. 窗口内（T+1 分钟）持 R2 的客户端：`refresh(R2)`
  5. 把另一条同样构造的会话推进到 T+3 分钟后：`refresh(R2)`
- 预期结果：
  1. 第 1 步 5 次都 200，不吊销
  2. 第 4 步 401 / 4019，`reason=superseded`，带 `retryAfterSeconds`，不发 token，不吊销
  3. 第 5 步 401 / 4019，`reason=replayDetected`，会话吊销

### TC-ACC-03-04 refresh token 91 天没用过期；89 天还能用
- 关联 AC：AC-ACC-03-4
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 登录 `platform=android`；另一条会话 `platform=ios`
- 步骤：
  1. ios 会话推进 89 天后 `refresh`
  2. android 会话推进到登录后 91 天 `refresh`
- 预期结果：
  1. 第 1 步成功；第 2 步 401 / 4019，会话未被按重放处理

### TC-ACC-03-05 管理接口 token 没有 sid，行为和现在一样
- 关联 AC：AC-ACC-03-5
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `POST /admin/users/token` 给 u1 签发 T，解码声明
  2. T 连接、发一条消息；管理接口强制下线后用 T 重连按现有规则
- 预期结果：
  1. 声明没有 `sid`、`gen`；连接和收发成功；`TC-AUTH` 相关行为不变

### TC-ACC-03-06 强制下线、封禁后，旧 refresh 都 4019，不是 4034
- 关联 AC：AC-ACC-03-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：u1、u2 各登录得到 R1、R1'
- 步骤：
  1. 管理接口强制下线 u1，`refresh(R1)`
  2. 管理接口封禁 u2，`refresh(R1')`
  3. u2 用正确密码登录
- 预期结果：
  1. 前两次都是 401 / 4019，不是 4034
  2. u2 登录才看到 403 / 4034

### TC-ACC-03-07 access 过期、refresh 有效冷启动：先刷新再 auth，不弹登录页
- 关联 AC：AC-ACC-03-7
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：App 已登录；用 [SDK 钟] 推进到 access 过期后
- 步骤：
  1. 杀掉 App，冷启动；抓帧和 HTTP 请求顺序
- 预期结果：
  1. 没有 `auth` 帧之前已经有 `token/refresh`；过程中不出现登录页和弹窗；进入会话列表，连接成功

### TC-ACC-03-08 refresh 已吊销冷启动：回登录页
- 关联 AC：AC-ACC-03-8
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：App 已登录后退出；管理接口强制下线这个用户
- 步骤：
  1. 冷启动 App
- 预期结果：
  1. 回登录页，本地 token 被清除。提示"登录已失效，请重新登录"（本地认为 access 已到期、先刷新得到 4019）或"账号已被下线"（access 未到期、先连上被 `kicked`），两种都算通过（v1.2，I3）。用例要断言实际走的是哪条路径，并核对对应文案

### TC-ACC-03-09 H5 刷新页面仍登录，新标签页要重新登录，localStorage 没有 token
- 关联 AC：AC-ACC-03-9
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 账号登录后刷新页面
  2. 关闭标签页，新开标签页打开同一地址
  3. 读 `localStorage` 全部键值，搜 token 特征
- 预期结果：
  1. 刷新后仍在会话列表；新标签页回登录页；`localStorage` 搜不到 access、refresh、恢复码

### TC-ACC-03-10 连接中剩余不足 6 小时：自动刷新、1000 断开重连、不提示
- 关联 AC：AC-ACC-03-10
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：账号会话连接中，access 还剩 6 小时 1 分钟；[SDK 钟] 可控
- 步骤：
  1. [SDK 钟] 推进 2 分钟
  2. 观察帧和页面；之后收发一条消息
- 预期结果：
  1. 发一次刷新；旧连接以 1000 关闭，1 秒内用新 token 重连
  2. 不弹窗，不出现"连接中"提示；消息正常收发

### TC-ACC-03-11 飞行模式 + access 过期冷启动：显示缓存，不回登录页；恢复网络后连接
- 关联 AC：AC-ACC-03-11
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：本地有缓存消息；access 已过期
- 步骤：
  1. 打开飞行模式，冷启动
  2. 关闭飞行模式
- 预期结果：
  1. 进入会话列表，显示缓存和"网络不可用"，不回登录页
  2. 恢复后自动刷新并连接

### TC-ACC-03-12 刷新返回 503 留在当前页退避；kicked/tokenExpired 且刷新成功不弹窗
- 关联 AC：AC-ACC-03-12
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假接口：第一次 `token/refresh` 返回 503 / 5030，之后 200
- 步骤：
  1. 触发刷新（[SDK 钟] 推进）
  2. 连接中由服务端发 `kicked/tokenExpired`（4011），刷新成功
- 预期结果：
  1. 第 1 步留在当前页，按退避重试，成功后恢复
  2. 第 2 步不弹窗，自动重连

### TC-ACC-03-13 新 token 落盘后、使用前杀 App：重启用新 token，不被吊销
- 关联 AC：AC-ACC-03-13
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：测试构建在"落盘后"放一个暂停点
- 步骤：
  1. 触发刷新，收到响应并落盘，在暂停点杀 App
  2. 重新启动，连接
- 预期结果：
  1. 用新 token 连接成功（转正），没有 4019；服务端 `im_refresh_replay_total` 不变

### TC-ACC-03-14 H5 sessionStorage 被复制：新页面回登录页，原页面不受影响
- 关联 AC：AC-ACC-03-14
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）
- 环境：E1 单节点
- 前置条件：已登录页面 P1
- 步骤：
  1. 场景一：P1 `window.open` 同源新窗口 P2
  2. 场景二：Playwright 复制标签页（不支持的浏览器标为未验证）得到 P2
  3. 场景三：P1 被 CDP 冻结（`Page.setWebLifecycleState frozen`）时复制得到 P2
  4. 每个场景观察 P2 和 P1，P1 随后发一条消息
- 预期结果：
  1. P2 启动时取 `navigator.locks` 失败，清除 token，回登录页
  2. P1 不出现 401，发消息成功；没有 `BroadcastChannel` 回退的路径另用桩页验证

### TC-ACC-03-15 H5 登录的 refresh 到期是 24 小时，超过绝对上限 401
- 关联 AC：AC-ACC-03-15
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`platform=web` 登录；[IM 钟] 冻结
- 步骤：
  1. 查 `refreshExpiresAt`
  2. 脚本循环：推进 23 小时、`refresh`、用新 access `auth` 转正，直到登录后 365 天
  3. 推进到 365 天零 1 分钟，`refresh`
- 预期结果：
  1. 第 1 步到期 = 现在 + 24 小时
  2. 循环中 `refreshExpiresAt`、`accessExpiresAt` 都不超过 `absoluteExpiresAt`
  3. 最后 401 / 4019

### TC-ACC-03-16 账号用户的无 sid token：改密后拒绝，之后签发的能连；不能调账号接口
- 关联 AC：AC-ACC-03-16
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 账号用户；管理接口签发 T（无 `sid`），T 的 `iat`=T0；[IM 钟] 冻结
- 步骤：
  1. 推进 1 秒，u1 改密
  2. `auth(T)`
  3. 推进 1 秒，管理接口再签发 T2，`auth(T2)`
  4. 用 T2 调 `POST /v1/account/password`
- 预期结果：
  1. 第 2 步收到 `kicked/passwordChanged`，关闭码 4012
  2. 第 3 步连接成功；`token_version` 没有变化
  3. 第 4 步 403 / 4030，`reason=accountSessionRequired`

### TC-ACC-03-17 入表前被吊销：连接随后被踢，不留在线
- 关联 AC：AC-ACC-03-17
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：故障注入"入表前吊销"，目标是 u1 的 `sid`
- 步骤：
  1. `auth(A1)`，校验通过后在入表前吊销
  2. 等 1 秒，查在线连接数
- 预期结果：
  1. 连接收到 `kicked/sessionRevoked`，以 4012 关闭；在线数 0

### TC-ACC-03-18 老版本留下的 Session 按演示处理
- 关联 AC：AC-ACC-03-18
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：用旧版本写入的 `{userId, token}`（没有 `mode`）
- 步骤：
  1. 升级后启动
  2. 打开"我 → 设置"
- 预期结果：
  1. 按演示会话连接成功；没有"账号安全"入口

### TC-ACC-03-19 响应丢失：5 分钟、1 小时、3 天后用旧值重试都成功
- 关联 AC：AC-ACC-03-19
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 登录得到 (A1, R1)；[IM 钟] 冻结；记录 `im_refresh_reissue_total`
- 步骤：
  1. `refresh(R1)`，丢弃响应（模拟丢包）
  2. 推进 5 分钟，`refresh(R1)`；推进到 1 小时，`refresh(R1)`；推进到 3 天，`refresh(R1)`
  3. 查 `device_sessions`
- 预期结果：
  1. 3 次都 200，`revoked_at` 为空，`im_refresh_reissue_total` 增加（首次之外的每次都算）
  2. 只有待确认代变化，`current_gen` 不变
- 说明：AC 里的"推送令牌还在"在 C1 不验证（`device_push` 随 C3 建表），C3 上线时补这一条断言

### TC-ACC-03-20 刷新请求发出后杀 App：5 分钟后用旧值重试成功
- 关联 AC：AC-ACC-03-20
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：用 HTTP 代理让响应晚 10 秒
- 步骤：
  1. 触发刷新，请求发出后杀 App
  2. [IM 钟] 推进 5 分钟，重新启动
- 预期结果：
  1. 用旧 refresh 重试成功，进入会话列表，不回登录页

### TC-ACC-03-21 同一 R1 两个并发：都不吊销，先用的转正，另一个作废
- 关联 AC：AC-ACC-03-21
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 持 R1
- 步骤：
  1. 客户端 C-a、C-b 同时 `refresh(R1)`，得到 (A2, R2)、(A3, R3)
  2. C-a：`auth(A2)`
  3. C-b：`auth(A3)`
  4. 查 `device_sessions`
- 预期结果：
  1. 第 1 步都 200；代号不同，都在 `pendings`
  2. 第 2 步成功，转正；第 3 步收到 `kicked/tokenExpired`（4011）
  3. 只有一条链：`row_version` 递增，`pendings` 已清空

### TC-ACC-03-22 1 分钟连续重试 3 次，用最新的转正；3 分钟后再用 R1 按重放
- 关联 AC：AC-ACC-03-22
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：[IM 钟] 冻结在 T；u1 持 R1
- 步骤：
  1. T+0、T+20 秒、T+40 秒各 `refresh(R1)`，得到 R2、R3、R4；每次检查会话未吊销
  2. T+1 分钟：`auth(A4)`（R4 这一代的 access）
  3. 推进到 T+4 分钟：`refresh(R1)`
- 预期结果：
  1. 3 次都成功；`auth` 转正
  2. 最后 401 / 4019，`reason=replayDetected`

### TC-ACC-03-23 四个入口合并成一次刷新，迟到响应丢弃
- 关联 AC：AC-ACC-03-23
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假 HTTP 记录请求；请求超时 30 秒；[SDK 钟] 可控
- 步骤：
  1. 同时触发：启动检查、回前台、定时器、`kicked/tokenExpired`
  2. 第一个请求不回应答，[SDK 钟] 推进 30 秒，SDK 放弃并重试，第二个成功得到 R3
  3. 第一个请求的迟到响应到达，带 R2
- 预期结果：
  1. 同一时间只有 1 个进行中的刷新请求
  2. 本地保存的是 R3；R2 被丢弃

### TC-ACC-03-24 用新 access auth 也算转正，之后 R1 按重放
- 关联 AC：AC-ACC-03-24
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：[IM 钟] 冻结在 T
- 步骤：
  1. u1：`refresh(R1)` 得到 (A2, R2)
  2. u1：`auth(A2)`
  3. 查 `current_gen`
  4. 推进 3 分钟：`refresh(R1)`
- 预期结果：
  1. `auth` 成功；`current_gen`=A2 的代号
  2. 最后 401 / 4019，`reason=replayDetected`

### TC-ACC-03-25 被盗场景：攻击者用偷来的 R1 重发并使用，受害者上线触发吊销
- 关联 AC：AC-ACC-03-25
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：受害者 V 在 D1 登录得到 (A1, R1)，然后 D1 离线；V 另有设备 D2 在线；攻击者 X 偷到 R1；[IM 钟] 冻结在 T
- 步骤：
  1. V 的 D1 之前已 `refresh(R1)` 得到 R2，响应已保存，但没有使用
  2. X：`refresh(R1)` 得到 (A3, R3)；X：`auth(A3)`，R3 这一代转正
  3. 把 [IM 钟] 推进 3 分钟
  4. V 的 D1 上线：分别在两条同样构造的会话上用 `refresh(R1)`、`refresh(R2)`
  5. 查指标和 `account_events`，观察 D2；X 用 R3 `refresh`
- 预期结果：
  1. V 的请求 401 / 4019，`reason=replayDetected`；`im_refresh_replay_total` +1
  2. `account_events` 有重放吊销记录（只有 `sid` 的哈希）；D2 收到 `securityNotice`
  3. X 的 refresh 也 401 / 4019，X 的连接被踢

### TC-ACC-03-26 replayDetected：回登录页前弹窗提示
- 关联 AC：AC-ACC-03-26
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假接口让刷新返回 401 / 4019，`reason=replayDetected`
- 步骤：
  1. 触发刷新
- 预期结果：
  1. 弹窗"检测到登录凭证异常，这台设备已下线。如果不是你本人操作，请尽快改密码"；确定后回登录页，本地 token 清除

### TC-ACC-03-27 session_purge 清除过期 31 天的会话
- 关联 AC：AC-ACC-03-27
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 的会话已过期；另一条过期 29 天
- 步骤：
  1. 把 [IM 钟] 推进到前者过期后 31 天，运行 `session_purge`
- 预期结果：
  1. 前者的行被删除；后者还在
- 说明：AC 里的"绑定的令牌被清除"在 C1 只验证会话行（`device_push` 随 C3 建表），C3 上线时补

### TC-ACC-03-28 被淘汰的代的 access：WS 4011、HTTP 4011 tokenSuperseded，不转正
- 关联 AC：AC-ACC-03-28
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 持 R1
- 步骤：
  1. `refresh(R1)` 连续 5 次，第一次得到 (A2, R2)，被淘汰
  2. `auth(A2)`
  3. 用 A2 调 `GET /v1/account/events`
  4. `refresh(R1)`
- 预期结果：
  1. 第 2 步 `kicked/tokenExpired`（4011）
  2. 第 3 步 401 / 4011，`reason=tokenSuperseded`
  3. 没有转正，`current_gen` 不变；第 4 步成功

### TC-ACC-03-29 过期或验签失败的 token 不触发转正
- 关联 AC：AC-ACC-03-29
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：[IM 钟] 冻结；`refresh(R1)` 得到 (A2, R2)
- 步骤：
  1. 把时钟推进到 A2 过期后：`auth(A2)`
  2. 构造 A2 改一位签名：`auth`
  3. 查 `current_gen`
- 预期结果：
  1. 依次 4011、4010；`current_gen` 都不变

### TC-ACC-03-30 攻击者转正，在线的受害者立刻被踢；窗口内 superseded，窗口后重放
- 关联 AC：AC-ACC-03-30
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：受害者 V 用 A1 在线（持 R1）；攻击者 X 偷到 R1；[IM 钟] 冻结在 T
- 步骤：
  1. X：`refresh(R1)` 得到 (A2, R2)
  2. X：`auth(A2)`，转正，`overlapUntil`=T+2 分钟
  3. 观察 V 的连接
  4. V 重连：`auth(A1)`
  5. T+1 分钟 V：`refresh(R1)`
  6. 把时钟推进到 T+3 分钟，V：`refresh(R1)`
  7. X：`refresh(R2)`
- 预期结果：
  1. 第 3 步 V 收到 `kicked/tokenExpired`（4011）
  2. 第 4 步被拒绝（4011），不转正、不吊销
  3. 第 5 步 401 / 4019，`reason=superseded`，不发 token
  4. 第 6 步 `reason=replayDetected`，会话吊销；第 7 步 X 也 401 / 4019

### TC-ACC-03-31 乱序到达：Q2 先得 R3、Q1 后得 R4，窗口内外的处理
- 关联 AC：AC-ACC-03-31
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 持 R1；[IM 钟] 冻结在 T
- 步骤：
  1. Q2：`refresh(R1)` 得到 (A3, R3)
  2. T+1 分钟 Q1（同一个 R1 迟到）：`refresh(R1)` 得到 (A4, R4)
  3. u1 用 A3 `auth`，转正，窗口到 T+3 分钟
  4. T+2 分钟：`refresh(R1)`、`refresh(R4)`
  5. 把时钟推进到 T+4 分钟：`refresh(R4)`
- 预期结果：
  1. 第 3 步成功，未吊销
  2. 第 4 步都是 401 / 4019，`reason=superseded`，不发 token
  3. 第 5 步 `reason=replayDetected`，会话吊销

### TC-ACC-03-32 只知道 sid 吊销不了会话
- 关联 AC：AC-ACC-03-32
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 登录；攻击者 X 知道 V 的 `sid`，不知道 refresh 密钥
- 步骤：
  1. X 构造 `kid.sid.gen.nonce.错误mac`
  2. X：`POST /v1/account/token/refresh`
  3. X：`POST /v1/account/logout {refreshToken:构造值}`
  4. 查会话、指标；V `refresh`
- 预期结果：
  1. 两次都是 401 / 4019，`reason=invalid`，不带 `recoverBefore`
  2. `revoked_at` 为空，`im_refresh_replay_total` 不变，`account_events` 没有吊销事件；V 刷新成功

### TC-ACC-03-33 sid 原值不进日志和后台响应
- 关联 AC：AC-ACC-03-33
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 跑完注册、登录、刷新、退出、重放
  2. 从 token 解出 `sid` 原值；搜索全部日志；请求 `GET /admin/accounts/{userId}`；抓 `/metrics`
- 预期结果：
  1. 都搜不到 `sid` 原值

### TC-ACC-03-34 离线受害者遇到攻击者改密：上线都看到找回截止日期
- 关联 AC：AC-ACC-03-34
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：V 有两台设备：D-a（access 未过期）、D-b（access 已过期）都离线；攻击者 X 知道密码；V 的恢复码 C1 已确认；[IM 钟] 冻结在 T。三套 SDK 和三端各跑一次
- 步骤：
  1. X：`POST /v1/account/password {oldPassword, newPassword}`（从自己的设备），写入 C1 的 `retire_until`=T+7 天
  2. 推进 1 小时，D-a 上线：`auth`
  3. D-b 上线：先 `refresh`
- 预期结果：
  1. D-a 收到 `kicked/passwordChanged`，带 `recoverBefore`=T+7 天，以 4012 关闭
  2. D-b 收到 401 / 4019，`reason=credentialsChanged`，带 `recoverBefore`
  3. 两端都弹窗"你的账号凭证有变化。如果不是你本人操作，请在 {日期} 前用恢复码找回"，确定后回登录页

### TC-ACC-03-35 新 IP 前缀写事件；账号安全有其他会话数量和最近活跃时间
- 关联 AC：AC-ACC-03-35
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 `203.0.113.1` 登录；另一条会话在别的设备
- 步骤：
  1. 从 `198.51.100.1` `refresh`；用新 access 转正
  2. 查 `account_events`、账号安全响应（`GET /v1/account/events`）
  3. 再从同一前缀 `refresh` 一次
- 预期结果：
  1. 第 1 步后有"新 IP 前缀"事件，存的是前缀；响应带其他会话数量和最近活跃时间
  2. 第 3 步不再增加事件

### TC-ACC-03-36 入表后复查数据库报错：1011 断开，SDK 重连，不回登录页
- 关联 AC：AC-ACC-03-36
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：故障注入"入表后复查失败"一次
- 步骤：
  1. sdk-ts 客户端 `auth`
  2. 观察关闭码和 SDK 行为
- 预期结果：
  1. 连接以 1011 关闭，SDK 退避后重连成功；没有 4012；SDK 不进入已退出状态

### TC-ACC-03-37 周期复查：丢失的踢线下一次复查补上；连续失败 3 次才断开
- 关联 AC：AC-ACC-03-47
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_SESSION_RECHECK=10m`；[IM 钟] 冻结；连接 C-a（u1 D1）、C-b（u2）
- 步骤：
  1. 故障注入丢弃吊销事件，强制下线 u1（C-a 没有被踢）
  2. 推进 10 分钟
  3. 故障注入 C-b 的周期复查数据库报错 1 次，推进 10 分钟；再连续报错 3 次，每次推进 10 分钟
- 预期结果：
  1. 第 2 步 C-a 收到踢线，以 4012 关闭
  2. 第 3 步报错 1 次不断开；连续第 3 次后以 1011 断开

### TC-ACC-03-38 攻击者用满签发次数压制不了受害者的重放判定
- 关联 AC：AC-ACC-03-48
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 持 R1；X 偷到 R1；[IM 钟] 冻结在 T
- 步骤：
  1. X：`refresh(R1)` 得到 (A2, R2)；X：`auth(A2)`，转正
  2. X：循环 `refresh(最新)` 直到这小时签发 20 次，第 21 次确认 429 / 4290
  3. 把时钟推进 3 分钟（仍在同一小时）
  4. V：`refresh(R1)`
- 预期结果：
  1. 第 4 步立即 401 / 4019，`reason=replayDetected`，不是 4290
  2. 会话吊销，X 的 token 失效

### TC-ACC-03-39 第 21 次重发 4290 不回登录页；100 天不转正到期；新 access 不超过绝对上限
- 关联 AC：AC-ACC-03-37
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：三套 SDK + 真实 im-server；[IM 钟]、[SDK 钟] 同步推进
- 步骤：
  1. 场景一：同一小时第 21 次 `refresh(R1)`，观察 SDK 和页面
  2. 场景二：另一条会话每 24 小时 `refresh(R1)` 一次不转正，共 100 天
  3. 场景三：距绝对上限 23 小时时 `refresh` 并转正
- 预期结果：
  1. 场景一 429 / 4290，SDK 退避，留在当前页；下一小时恢复
  2. 场景二在登录后 90 天之后 401 / 4019（重发不滑动），第 100 天仍是 4019
  3. 场景三 `accessExpiresAt` = `absoluteExpiresAt`

### TC-ACC-03-40 连续转正不能续开窗口：受害者的旧值马上重放
- 关联 AC：AC-ACC-03-38
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 持 R1，已 `refresh(R1)` 没有转正；X 偷到 R1；[IM 钟] 冻结在 T
- 步骤：
  1. X：`refresh(R1)` 得到 (A2, R2)；X：`auth(A2)`，gen2 转正，`overlapMinGen`=gen1
  2. T+1 分 50 秒 X：`refresh(R2)` 得到 (A3, R3)；X：`auth(A3)`，gen3 转正，`overlapMinGen`=gen2
  3. T+1 分 55 秒 V：`refresh(R1)`
  4. X：`refresh(R3)`
- 预期结果：
  1. 第 3 步立即 401 / 4019，`reason=replayDetected`（R1 的代号低于 `overlapMinGen`），会话吊销
  2. 第 4 步也 401 / 4019，X 的连接被踢

### TC-ACC-03-41 superseded 重试：按 retryAfter 重试，3 次后回登录页
- 关联 AC：AC-ACC-03-39
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假接口：`token/refresh` 一直返回 401 / 4019，`reason=superseded`，`retryAfterSeconds=120`
- 步骤：
  1. 触发刷新；[SDK 钟] 每次推进 120 秒
- 预期结果：
  1. 留在当前页，3 次请求的间隔都是 120 秒；每次重试前重读本地 token
  2. 第 3 次仍 `superseded` 后回登录页

### TC-ACC-03-42 代号不复用：已作废的 access 不会复活
- 关联 AC：AC-ACC-03-40
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 持 R1；X 偷到 R1
- 步骤：
  1. X：`refresh(R1)` 两次得到 A2（gen2）、A3（gen3）
  2. V：`refresh(R1)` 得到 A4（gen4），`auth(A4)` 转正（A2、A3 作废）
  3. V：`refresh(R4)` 得到 A5（gen5），`auth(A5)` 转正
  4. X：`auth(A3)`；查 `current_gen`、`next_gen`
- 预期结果：
  1. A3 一直被拒绝（4011）；新代号 > gen3，`next_gen` 严格递增

### TC-ACC-03-43 校验 gen=1 后暂停，另一请求转正：入表后复查发现不符
- 关联 AC：AC-ACC-03-41
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：账号并发钩子停在"连接校验之后、入表之前"；u1 用 A1 连接
- 步骤：
  1. C-a：`auth(A1)` 校验通过后暂停
  2. 同一会话另一客户端：`refresh(R1)` 得到 (A2, R2)，`auth(A2)` 转正
  3. 放行 C-a
- 预期结果：
  1. C-a 入表后复查发现代号不符，收到 `kicked/tokenExpired`（4011）；C-a 的连接没有留在线

### TC-ACC-03-44 在途请求带旧 token 得到 4011：用本地新 token 重试，不多发刷新
- 关联 AC：AC-ACC-03-42
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：假 HTTP：3 个 `/v1/files/urls` 带旧 token 时返回 401 / 4011（`tokenSuperseded`）；本地已换新 token
- 步骤：
  1. 换 token 的瞬间发起 3 个 `getFileUrls`
- 预期结果：
  1. 3 个都用新 token 重试成功；`token/refresh` 没有新增请求

### TC-ACC-03-45 access 到期等于绝对上限：不再刷新，到期回登录页
- 关联 AC：AC-ACC-03-43
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假接口响应 `accessExpiresAt`=`absoluteExpiresAt`，距今 5 小时；[SDK 钟] 可控
- 步骤：
  1. 推进 1 小时；再回前台一次
  2. 推进到到期后
- 预期结果：
  1. 没有新的刷新请求，也没有重连
  2. 到期后回登录页

### TC-ACC-03-46 已吊销的会话遇旧值不算重放；logout 带被替换的 Bearer 和有效 refresh 能退出
- 关联 AC：AC-ACC-03-44
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 改密，旧会话 `revokeReason=credentialsChanged`；另一账号 u2 持被替换的 A1 和有效的 R3
- 步骤：
  1. u1 的旧 R1：`refresh(R1)`
  2. u2：`POST /v1/account/logout {refreshToken:R3}`，`Authorization: Bearer A1`
  3. 查 `im_refresh_replay_total`；u2 `refresh(R3)`
- 预期结果：
  1. 第 1 步 401 / 4019，`reason=credentialsChanged`，指标不变
  2. 第 2 步成功；第 3 步 401 / 4019

### TC-ACC-03-47 连续 6 小时刷满：有事件，其他在线设备收到通知
- 关联 AC：AC-ACC-03-45
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 有 D1（在线）；持 R1 的攻击者 X；[IM 钟] 冻结
- 步骤：
  1. X：连续 6 个小时，每小时 `refresh(R1)` 21 次；每轮后推进 1 小时
  2. 查 `account_events`；观察 D1
- 预期结果：
  1. 每小时第 21 次 429 / 4290，不吊销
  2. 有限流事件；第 6 小时 D1 收到 `securityNotice`

### TC-ACC-03-48 refresh 密钥轮换后旧 token 可用；提前删旧密钥配置检查报错
- 关联 AC：AC-ACC-03-46
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：refresh 密钥 k1，持久化台账；用 k1 签发 R1
- 步骤：
  1. 轮换到 k2（k1 进台账，`retiredAt`=T），重启，`refresh(R1)`
  2. 配置里删除 k1（退役未满 365 天），运行配置检查（`GET /internal/v1/config`）
  3. 把 `retiredAt` 推到 366 天前再运行配置检查
- 预期结果：
  1. 第 1 步成功，新 token 的 `kid`=k2
  2. 第 2 步 `productionChecks` 报错，生产配置下拒绝启动
  3. 第 3 步通过

## F-ACC-04 退出、改密与下线

### TC-ACC-03-51 距绝对上限不足 1 秒的刷新按到期处理，不再签发
- 关联 AC：AC-ACC-03-15、AC-ACC-03-43
- 优先级：P0
- 批次：C1
- 层级：Go e2e（[IM 钟]）
- 环境：E1 单节点
- 前置条件：登录得到 R1，记下 `absoluteExpiresAt`（PRD-chat v1.2 第 4 节 `IM_REFRESH_ABSOLUTE`，F2）
- 步骤：
  1. [IM 钟] 推进到距上限 1.5 秒，用 R1 刷新
  2. 推进到距上限 0.5 秒，用上一步拿到的值刷新
  3. 推进到上限的同一秒内，再刷新一次
- 预期结果：
  1. 200，新 access 到期不超过 `absoluteExpiresAt`
  2. 401 / 4019，`reason=expired`；没有新的 refresh 签发，会话不前进
  3. 同 2

### TC-ACC-04-01 D1 退出：D1 的 refresh、access 都失效，D2 不受影响
- 关联 AC：AC-ACC-04-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 D1、D2 登录，都连接中
- 步骤：
  1. D1：`POST /v1/account/logout {refreshToken:R-d1}`，Bearer A-d1
  2. D1：`refresh(R-d1)`；`auth(A-d1)`
  3. D2 收发一条消息；`refresh(R-d2)`
- 预期结果：
  1. 退出成功；refresh 401 / 4019；`auth` 收到 `kicked/sessionRevoked`（4012）
  2. D2 不受影响，刷新成功

### TC-ACC-04-02 D1 改密：换新 sid 1 秒内重连；D2 被踢；两边旧 refresh 都 credentialsChanged
- 关联 AC：AC-ACC-04-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 D1、D2 在线，恢复码已确认
- 步骤：
  1. D1：`POST /v1/account/password {oldPassword, newPassword}`
  2. D1 用响应的新 token 重连，记录时间；观察 D1 的页面
  3. 观察 D2
  4. 用 D1、D2 的旧 refresh 分别 `refresh`；用新 token 查 `sid`
- 预期结果：
  1. 响应带新 token（新 `sid`）和新一代恢复码
  2. D1 1 秒内重连，无连接提示；D2 收到 `kicked/passwordChanged`，以 4012 关闭
  3. 旧 refresh 都 401 / 4019，`reason=credentialsChanged`，`im_refresh_replay_total` 不变

### TC-ACC-04-03 旧密码错误：401，什么都不变，计入失败次数
- 关联 AC：AC-ACC-04-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 D1、D2 在线
- 步骤：
  1. D1：`password {oldPassword:错误, newPassword:合法}`
  2. D2 收发消息；用原密码登录
- 预期结果：
  1. 401 / 4018；密码不变；D2 不受影响；`auth_throttle` 增加

### TC-ACC-04-04 强制下线吊销全部设备会话
- 关联 AC：AC-ACC-04-4
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 两条设备会话
- 步骤：
  1. 管理接口 `POST /admin/users/u1/force-logout`
  2. 两个 refresh 分别 `refresh`
- 预期结果：
  1. 都 401 / 4019；`device_sessions` 两行 `revoked_at` 不空

### TC-ACC-04-05 D2 收到 passwordChanged：不重连，弹窗显示日期，确定后清空回登录页
- 关联 AC：AC-ACC-04-5
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：假服务端发 `kicked/passwordChanged`，`recoverBefore`=某日期；另一场景不带 `recoverBefore`
- 步骤：
  1. 观察 5 秒；点确定
- 预期结果：
  1. 带日期时弹窗显示"…请在 {日期} 前用恢复码找回"；不带时显示现有文案
  2. 期间没有重连；确定后回登录页，本地数据和 token 清除

### TC-ACC-04-06 断网退出：本地立即退出，联网后补发 logout
- 关联 AC：AC-ACC-04-6
- 优先级：P0
- 批次：C1
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 模拟器
- 前置条件：已登录；断开网络
- 步骤：
  1. 点退出并确认
  2. 检查本地存储：Keystore 文件、Keychain
  3. 恢复网络，把 App 启动到登录页
  4. 查服务端
- 预期结果：
  1. 立即回登录页；登录 token 已清除，"待退出"只留 `sid` 和 refresh
  2. 恢复网络后服务端收到 `logout`，`sid` 吊销；"待退出"记录被清除
- 说明：AC 里"调用了系统的令牌注销"在 C1 不验证：FCM 和 APNs 注册随 C3 才接入，这里只验证调用点存在于替身；C3 上线时补

### TC-ACC-04-07 吊销写库失败：改密整体回滚，旧密码仍有效
- 关联 AC：AC-ACC-04-7
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：故障注入"吊销写失败"一次；u1 在 D1、D2 在线
- 步骤：
  1. D1 改密
  2. 用旧密码登录；D2 收发消息
- 预期结果：
  1. 500 / 5000；旧密码能登录；D2 连接不变；`credentials_version` 不变

### TC-ACC-04-08 D2 刷新和 D1 改密并发：不留下改密后还有效的 D2 会话
- 关联 AC：AC-ACC-04-8
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 D1、D2；重复 20 轮
- 步骤：
  1. 同时发 D2 `refresh` 和 D1 `password`
  2. 改密返回后，用 D2 刷新得到的 token（如果有）`auth`，再 `refresh`
- 预期结果：
  1. 每轮：刷新要么 200 然后新 token 被 4012 拒绝，要么 401 / 4019
  2. 没有一轮在改密后还有可用的 D2 会话

### TC-ACC-04-09 退出其他设备：密码错 401；密码对踢 D2，D1 换 sid 重连，密码不变
- 关联 AC：AC-ACC-04-10
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：u1 在 D1、D2 在线，三套 SDK 和三端（设置页入口）各跑一次
- 步骤：
  1. D1："账号安全 → 退出其他设备"，输入错误密码
  2. 再输入正确密码（`POST /v1/account/logout-others`）
  3. 观察 D2；用原密码登录
- 预期结果：
  1. 错误时 401 / 4018，原地提示，没有变化
  2. 正确时 D2 收到 `kicked/sessionRevoked`（4012）；D1 换新 `sid`，1 秒内重连，无连接提示
  3. 原密码还能登录

### TC-ACC-04-10 logout 重复调用都成功
- 关联 AC：AC-ACC-04-11
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 同一 refresh 连续调 2 次 `logout`
- 预期结果：
  1. 都 200

### TC-ACC-04-11 新密码不合规：400，密码和恢复码都不变
- 关联 AC：AC-ACC-04-12
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 改密 `newPassword="aaaaaaaa"`
  2. 用旧密码登录；查 `recovery_codes`
- 预期结果：
  1. 400 / 4000，`reason=passwordWeak`；旧密码可用；没有新的一代

### TC-ACC-04-12 旧码保留 7 天：第 6 天成功、第 8 天失败；旧码优先
- 关联 AC：AC-ACC-04-13
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 的 C1 已确认；[IM 钟] 冻结在第 0 天；另一账号 u2 同样准备
- 步骤：
  1. 第 0 天 u1 改密，得到并确认 C2；u2 同样
  2. 推进到第 6 天：`recover(u1, C1, 新密码)`
  3. 紧接着 `recover(u1, C2, 另一密码)`
  4. 推进到第 8 天：`recover(u2, C1', 新密码)`
  5. 第 8 天：`recover(u2, C2', 新密码)`
- 预期结果：
  1. 第 2 步成功；第 3 步 401 / 4018（C2 比 C1 新，随之作废）
  2. 第 4 步 401 / 4018；第 5 步成功（没有用过 C1 时 C2 可用）

### TC-ACC-04-13 待退出：只有 refresh 也能退出，旧代值不算重放
- 关联 AC：AC-ACC-04-14
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 持 R1，已转正到 R2；access 已过期（[IM 钟] 推进 25 小时）
- 步骤：
  1. `POST /v1/account/logout {refreshToken:R2}`，不带 Bearer
  2. 另一条同样的会话：`POST /v1/account/logout {refreshToken:R1}`（已作废的旧代，`mac` 正确）
  3. 查 `im_refresh_replay_total`；两个 `sid` 的状态
- 预期结果：
  1. 都 200，`sid` 吊销；指标不变，没有异常弹窗事件

### TC-ACC-04-14 攻击者和 D1 共用 sid：D1 改密把攻击者清除
- 关联 AC：AC-ACC-04-15
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 在 D1 登录（sid1）；D1 `refresh` 得到 (A2, R2) 并落盘；X 复制了设备上的 (A2, R2)。**X 必须持和 D1 同一代的值**：如果 X 转正的是更新的一代，D1 的 A2 已被替换，`password` 会先得到 401 / 4011，走不到改密（见文件末尾的说明）
- 步骤：
  1. X：`auth(A2)`，gen2 转正，保持连接
  2. D1：`auth(A2)`（同一代，放行）；D1：`POST /v1/account/password`，Bearer A2（access 未过期）
  3. 观察 X 的连接；X：`refresh(R2)`；X：`auth(A2)`
  4. 查 D1 响应的 `sid`
- 预期结果：
  1. `sid1` 吊销；X 的连接收到 `kicked/passwordChanged`（4012）；refresh 401 / 4019，`reason=credentialsChanged`；access 被拒绝（4012）
  2. D1 拿到不同于 `sid1` 的新 `sid`，新 token 可用

### TC-ACC-04-15 改密响应丢失：旧 refresh 不算重放，按规则 6a 回登录页
- 关联 AC：AC-ACC-04-16
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：D1 已登录，恢复码已确认；三套 SDK 和三端各跑一次；测试代理丢弃改密响应
- 步骤：
  1. D1 改密（服务端已提交，响应丢失）
  2. SDK 超时后，用旧 refresh 刷新
  3. 观察界面；查指标；用新密码登录
- 预期结果：
  1. 刷新 401 / 4019，`reason=credentialsChanged`，带 `recoverBefore`；不弹"凭证异常"
  2. `im_refresh_replay_total` 不变；弹"你的账号凭证有变化…"，确定后回登录页
  3. 用新密码能登录，登录响应带再新一代恢复码

### TC-ACC-04-16 定时刷新和改密同时触发：串行，不误吊销
- 关联 AC：AC-ACC-04-17
- 优先级：P0
- 批次：C1
- 层级：SDK 单测（三套）
- 环境：E1 单节点
- 前置条件：假 HTTP 记录顺序；[SDK 钟] 让刷新定时器和改密同一时刻触发
- 步骤：
  1. 调用 `changePassword`，同时定时器到期
- 预期结果：
  1. 同一时间只有一个请求；两者顺序固定（队列）；队列内改密如要先刷新，直接执行不重新排队；没有吊销

### TC-ACC-04-17 登录在 Argon2 后暂停，recover 提交后放行：登录 401
- 关联 AC：AC-ACC-04-18
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：账号 u1；X 知道当前密码；V 持有效恢复码 C1；账号并发钩子打开
- 步骤：
  1. X：`login(u1, 旧密码)`，钩子停在 Argon2 之后、提交之前
  2. V：`recover(u1, C1, 新密码)`，成功
  3. 放行 X
  4. 查 `device_sessions`
- 预期结果：
  1. X 的登录 401 / 4018，不计入失败次数
  2. 账号只有 V 的会话

### TC-ACC-04-18 改密在 Argon2 后暂停，recover 提交后放行：改密 401，攻击者没有恢复码
- 关联 AC：AC-ACC-04-19
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：X 用当前密码持有已登录会话；V 持 C1；钩子打开
- 步骤：
  1. X：`password {oldPassword:旧, newPassword:X 的}`，停在 Argon2 之后
  2. V：`recover(u1, C1, V 的新密码)`，成功
  3. 放行 X
  4. 用 V 的新密码、X 的新密码分别登录
- 预期结果：
  1. 改密 401 / 4018，响应没有恢复码
  2. V 的新密码能登录；X 的新密码 401；`recovery_codes` 没有 X 这次产生的代

### TC-ACC-04-19 同一恢复码两个并发 recover，恰好一个成功
- 关联 AC：AC-ACC-04-20
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 两个请求同时 `recover(u1, C1, 不同的新密码)`，重复 10 轮（每轮重新构造账号）
- 预期结果：
  1. 每轮恰好一个成功，另一个 401 / 4018；最终密码是成功那个请求的

### TC-ACC-04-20 撤销注销和在途 delete：delete 401，清理不处理
- 关联 AC：AC-ACC-04-21
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 申请注销后恢复码可用；另有一个 `delete` 请求（u1 的另一会话）停在 Argon2 之后
- 步骤：
  1. V：`recover(u1, C1, 新密码)`（撤销注销）
  2. 放行 `delete`
  3. 把 [IM 钟] 推进 8 天，运行 `account_cleanup`
- 预期结果：
  1. `delete` 401 / 4018
  2. `delete_requested_at` 为空；清理不处理 u1，能用新密码登录

### TC-ACC-04-21 所有校验密码的接口共用失败计数
- 关联 AC：AC-ACC-04-22
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：持有 u1 access 的 X
- 步骤：
  1. X 依次用错误密码调 `logout-others`、`recovery-code`、`delete`、`logout-others`、`recovery-code`
  2. 第 6 次 X 用正确密码调 `delete`
- 预期结果：
  1. 前 5 次 401 / 4018；第 6 次 429 / 4294，账号状态没有变化

### TC-ACC-04-22 攻击者循环改密压制不了受害者找回
- 关联 AC：AC-ACC-04-23
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 持有效旧码 C1；X 有当前密码；脚本 X：循环 `login` + `password`（每轮用上一轮的新密码），持续 5 秒以上
- 步骤：
  1. 脚本运行中，V：`recover(u1, C1, V 的新密码)`，只发一次
  2. 停止脚本；查会话和 X 的最后一个 `password` 响应
- 预期结果：
  1. V 第一次就成功
  2. X 的全部会话吊销；在途改密 401 / 4018；V 的新密码有效，X 的无效

### TC-ACC-04-23 锁定和哈希并发都挡不住正确恢复码；错误的仍受限制
- 关联 AC：AC-ACC-04-24
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 同一用户名 + IP 已锁定（5 次错误）；另一线程 X 对 u1 正在算哈希（钩子占住）；其他账号循环改密占住改密专用位
- 步骤：
  1. 从同一 IP：`recover(u1, 错误恢复码, 新密码)`
  2. 从同一 IP：`recover(u1, C1, 新密码)`
- 预期结果：
  1. 第 1 步 429 / 4294
  2. 第 2 步成功（用 `recover` 专用位）

### TC-ACC-04-24 四条序列都带 recoverBefore，都是第 7 天
- 关联 AC：AC-ACC-04-25
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：V 的 C1 第 0 天换代（攻击者第一次改密），C1 的 `retire_until`=第 7 天；V 的设备离线；三套 SDK 和三端各验一次显示
- 步骤：
  1. 序列甲：X 先 `logout-others`（当前密码）再 `password`；V 第 1 天上线
  2. 序列乙：X `password` 后 `delete`；V 第 1 天上线
  3. 序列丙：X 第 0 天 `password`，第 5 天再 `password`；V 第 6 天上线
  4. 序列丁：V 在线时 X `logout-others`
- 预期结果：
  1. 甲、乙、丙：V 的 access `auth` 得到 `kicked`，或 refresh 得到 4019，都带 `recoverBefore`=第 7 天
  2. 丁：V 收到 `kicked/sessionRevoked`，带 `recoverBefore`=第 7 天
  3. 序列丙第二次改密没有把日期推后；客户端都显示第 7 天

### TC-ACC-04-25 改密的四种响应时序：踢线缓存、超时、顺序
- 关联 AC：AC-ACC-04-27
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 单节点
- 前置条件：假服务端；[SDK 钟] 可控；SDK 请求超时 30 秒
- 步骤：
  1. 场景一：`password` 不回响应，收到踢线
  2. 场景二：响应在 31 秒到达
  3. 场景三：响应 500，同时收到踢线
  4. 场景四：响应先到，踢线后到
  5. 每个场景后再发一次普通命令，确认队列不堵
- 预期结果：
  1. 前三种提示失败，缓存的踢线随后生效（弹窗或回登录页）；迟到响应被丢弃
  2. 第四种不弹窗，用新 token 重连，旧 `sid` 的踢线被忽略

### TC-ACC-04-26 delete/cancel 暂停后 recover 再申请注销：撤销 401，新申请有效
- 关联 AC：AC-ACC-04-28
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 已申请注销；钩子打开
- 步骤：
  1. X：`POST /v1/account/delete/cancel {username, password}`，停在 Argon2 之后
  2. V：`recover(u1, C1, 新密码)`（撤销原申请）
  3. V 登录，`POST /v1/account/delete {password:新密码}`（新的申请）
  4. 放行 X
- 预期结果：
  1. X 401 / 4018
  2. 新申请不被撤销，`delete_requested_at` 是新值

### TC-ACC-04-27 【未执行】Postgres：recover 持锁期间，三种请求都等待后 401
- 关联 AC：AC-ACC-04-29
- 优先级：P0
- 批次：C1
- 层级：后端 e2e（Postgres 并发）
- 环境：Postgres（B6 后）
- 前置条件：**只有 Postgres 实现后才能跑**；仓库现在只有 SQLite，事务串行，这条交错不可达。**C1 验收时记为未执行，不算通过**；钩子 A 停在 `recover` 事务第一条语句之后；钩子 B 让 X 的改密、`logout-others`、登录都停在 Argon2 之后
- 步骤：
  1. V：`recover(u1, C1, 新密码)`，停在钩子 A（已持 `accounts` 行锁）
  2. 分别放行 X 的 `password`、`logout-others`、`login`，确认它们阻塞在行锁上（`pg_stat_activity`）
  3. 放行 `recover`
- 预期结果：
  1. 3 个请求都等待后 401 / 4018，不计失败次数
  2. X 没有拿到恢复码；提交后账号只有 V 的会话

### TC-ACC-04-28 专用位公平：攻击者循环持新码，受害者和其他账号都在时限内成功
- 关联 AC：AC-ACC-04-30
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 持 C1；X 改密后持 Cn（比 C1 新）；`IM_ARGON_RECOVER_SLOTS=1`，`IM_RECOVER_QUEUE=5s`；u9 是其他账号，持自己的码
- 步骤：
  1. X：20 并发循环 `recover(u1, Cn, …)`；另一脚本循环 `recover(u8, 码, …)`
  2. V：`recover(u1, C1, 新密码)`，记录耗时
  3. u9：`recover(u9, 码, 新密码)`，记录耗时
  4. 查 Argon2 钩子日志
- 预期结果：
  1. V、u9 都在 5 秒内成功
  2. 同一时刻 X 的码只算 1 个；V 的旧代优先于 X 的 Cn

### TC-ACC-04-29 成功次数上限只约束最新一代，受害者手里的旧代不受限
- 关联 AC：AC-ACC-04-31
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：[IM 钟] 冻结在第 0 天；V 的 C1 第 0 天换代，得到并确认 C2（V 持 C2）
- 步骤：
  1. 第 1 天 X（有密码）：`password`，得到 C3 并确认
  2. X：用当前最新一代循环 `recover`（每次用上次响应的新码），同一小时内 7 次
  3. 第 2 天 V：`recover(u1, C2, V 的新密码)`
  4. X：用最新的码 `recover`
- 预期结果：
  1. X 前 6 次成功，第 7 次 429 / 4290，`reason=recoverLimited`
  2. V 成功（C2 不受这个上限），比 C2 新的代都作废
  3. X 最后一次 401 / 4018

## F-ACC-05 恢复码与找回密码

### TC-ACC-05-01 正确恢复码找回：新密码可用，旧设备下线，旧码作废
- 关联 AC：AC-ACC-05-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 D1 在线，C1 已确认
- 步骤：
  1. `POST /v1/account/recover {username, recoveryCode:C1, newPassword, platform}`
  2. 新密码、旧密码分别登录；观察 D1
  3. 再用 C1 找回
- 预期结果：
  1. 响应带新 token、新恢复码；D1 收到 `kicked/passwordChanged`
  2. 新密码成功，旧密码 401 / 4018；再用 C1 401 / 4018

### TC-ACC-05-02 用户名不存在、恢复码错、已用过：响应相同
- 关联 AC：AC-ACC-05-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 分别 `recover`：`nouser` + 随机码；`real1` + 错误码；`real2` + 已用过的码（每种不同 IP，避免限流）
- 预期结果：
  1. 都是 401 / 4018，响应体和响应头名集合相同

### TC-ACC-05-03 恢复码错误 5 次后再错 429；正确的不受限
- 关联 AC：AC-ACC-05-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 同一用户名 + IP 错误恢复码 5 次
  2. 第 6 次错误恢复码
  3. 同一 IP 正确恢复码
- 预期结果：
  1. 第 6 次 429 / 4294；正确的成功

### TC-ACC-05-04 找回成功：显示新恢复码，确认后进入会话列表
- 关联 AC：AC-ACC-05-5
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 登录页"忘记密码"，填恢复码和新密码提交
  2. 查看保存页，点"我已保存"
- 预期结果：
  1. 显示新恢复码；确认后进入会话列表

### TC-ACC-05-05 注册响应丢失：登录时发新码，注册时的码失效，确认后不再带
- 关联 AC：AC-ACC-05-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 注册 u1，丢弃响应（`acked_at` 为空）
  2. 用密码登录；记下响应的 C2、`gen`
  3. 用注册时生成的码（从数据库无法还原，测试助手从丢弃的响应里取）`recover`
  4. ack `gen`；再登录
- 预期结果：
  1. 第 2 步响应带新恢复码；第 3 步 401 / 4018
  2. 确认后登录响应不带 `recoveryCode`

### TC-ACC-05-06 在保存页杀 App：再次进入保存页，新码，确认前 token 没有持久化
- 关联 AC：AC-ACC-05-7
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 登录触发保存页（最新一代未确认）；检查存储（Keystore、Keychain、`sessionStorage`）
  2. 杀 App（H5 刷新）
  3. 重新打开并登录
- 预期结果：
  1. 确认前存储里没有 token
  2. 杀掉后回登录页；再次进入保存页，显示新恢复码（和上次不同）

### TC-ACC-05-07 新密码不合规、账号封禁：恢复码和密码都不变
- 关联 AC：AC-ACC-05-8
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `recover(u1, C1, "aaaaaaaa")`
  2. 封禁 u2，`recover(u2, C2, 合法新密码)`
  3. u1 再用 C1 和合法密码找回
- 预期结果：
  1. 第 1 步 400 / 4000；第 2 步 403 / 4034；两者的恢复码、密码不变
  2. 第 3 步成功

### TC-ACC-05-08 后台没有任何改密码的接口
- 关联 AC：AC-ACC-05-9
- 优先级：P0
- 批次：C1
- 层级：单测 + 后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 枚举 `/admin/*` 路由表和 `console` 主凭证 scope
  2. 枚举 console-server 路由，按路径和请求字段搜 `password`、`recovery`、`reset`
- 预期结果：
  1. 没有接口能设置、重置用户密码或恢复码；没有 `account.reset`

### TC-ACC-05-09 三个页面都有"丢失后无法找回"，没有客服入口
- 关联 AC：AC-ACC-05-10
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 打开注册页、保存页、"忘记密码"页
- 预期结果：
  1. 都有"恢复码丢失后账号无法找回"；都没有联系客服的入口或链接

### TC-ACC-05-10 时间侧信道：恢复码错误和账号不存在，中位数相差不超过 5%
- 关联 AC：AC-ACC-05-11
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_TEST_DISABLE_AUTH_LOCK=1`）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 交替 `recover`：存在账号 + 错误码、不存在账号，各 50 次，记录耗时
- 预期结果：
  1. 两组中位数相差 ≤ 5%；都是 401 / 4018

### TC-ACC-05-11 重新生成恢复码：要密码；第 6 天旧码可用，第 8 天不可用
- 关联 AC：AC-ACC-05-12
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1、u2 的旧码都已确认；[IM 钟] 冻结在第 0 天
- 步骤：
  1. u1：`POST /v1/account/recovery-code {password:错误}`
  2. u1、u2：`recovery-code {password:正确}`，确认新码
  3. 第 6 天：`recover(u1, 旧码, 新密码)`
  4. 第 8 天：`recover(u2, 旧码, 新密码)`
- 预期结果：
  1. 错误时 401 / 4018，码不变
  2. 第 3 步成功；第 4 步 401 / 4018

### TC-ACC-05-12 改密响应丢失：用新密码登录拿到更新的码，旧已确认码还有效
- 关联 AC：AC-ACC-05-13
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 C1 已确认；测试代理丢弃改密响应（生成了没人见过的 C2）
- 步骤：
  1. 用新密码登录，得到 C3
  2. 确认 C3；再次登录
  3. 第 6 天用 C1 `recover`
- 预期结果：
  1. 第 1 步响应带 C3；C2 作废
  2. 第 2 步登录响应不带恢复码
  3. 第 3 步成功

### TC-ACC-05-13 两台设备的确认：旧 gen 不算确认
- 关联 AC：AC-ACC-05-14
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 最新一代未确认
- 步骤：
  1. D1 登录，显示 C1（`gen=1`）
  2. D2 登录，生成 C2（`gen=2`）
  3. D1：`recovery-code/ack {gen:1}`
  4. 查状态；D2：`ack {gen:2}`
- 预期结果：
  1. D1 400 / 4000，`reason=recoveryCodeStale`，状态仍未确认
  2. D2 确认成功

### TC-ACC-05-14 recoveryCodeStale：提示后回登录页
- 关联 AC：AC-ACC-05-15
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：假接口 ack 返回 400 / 4000，`reason=recoveryCodeStale`
- 步骤：
  1. 保存页点"我已保存"
- 预期结果：
  1. 提示"恢复码已更新，请重新登录查看"，回登录页

### TC-ACC-05-15 攻击者改密后申请注销：第 3 天旧码找回，全部撤销
- 关联 AC：AC-ACC-05-16
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 的 C1 已确认；X 有密码；[IM 钟] 第 0 天
- 步骤：
  1. X：`password`；X：`delete {password:X 的}`
  2. 第 3 天 V：`recover(u1, C1, V 的新密码)`
  3. 第 8 天运行 `account_cleanup`；V 登录
  4. X 的 refresh 和 access 分别使用
- 预期结果：
  1. V 成功，注销申请撤销
  2. 清理不处理；V 能登录
  3. X 的 refresh 401 / 4019，access 被拒绝

### TC-ACC-05-16 攻击者用新码找回再改密：受害者第 6 天旧码清除一切；第 8 天不延长
- 关联 AC：AC-ACC-05-17
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：V 的 C1 已确认；X 有密码；另一账号 u2 同样；[IM 钟] 第 0 天
- 步骤：
  1. X：`password` 得到 C2，确认；X：`recover(u1, C2, …)` 得到 C3，确认；X：`password` 得到 C4，确认
  2. 第 6 天 V：`recover(u1, C1, V 的新密码)`
  3. 依次 `recover(u1, C2…)`、`recover(u1, C3…)`、`recover(u1, C4…)`
  4. 查 X 的会话
  5. u2 按同样序列，第 8 天用 C1' `recover`
- 预期结果：
  1. 第 2 步成功；X 的会话全部吊销
  2. C2、C3、C4 都 401 / 4018；u2 第 8 天 401 / 4018（多次改密不延长）

### TC-ACC-05-17 匿名占满闸门：错误码快速 401，正确码走专用位
- 关联 AC：AC-ACC-05-18
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：Argon2 阻塞钩子占满 4 个位
- 步骤：
  1. `recover(u1, 错误码, 新密码)`，记录耗时
  2. `recover(u1, C1, 新密码)`，记录耗时
- 预期结果：
  1. 错误码 401 / 4018，耗时明显小于闸门排队 1 秒
  2. 正确码在专用位完成，成功

### TC-ACC-05-18 最新一代未确认：已登录的强制进保存页；改密后杀 App 回登录页
- 关联 AC：AC-ACC-05-19
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 单节点
- 前置条件：三套 SDK 和三端各跑一次
- 步骤：
  1. 场景一：已登录，`recovery-code` 响应丢失；杀 App 重开，连接；抓 `auth` ack 和账号接口响应
  2. 场景一继续：保存页输入密码重新生成
  3. 场景二：改密响应到达，在保存页杀 App；重开；用新密码登录
- 预期结果：
  1. 场景一：响应带 `recoveryCodeUnacked`，App 强制进保存页；重新生成后拿到新码，确认后解除
  2. 场景二：token 没有落盘，回登录页；用新密码登录，拿到新码

## F-ACC-06 资料与头像

### TC-ACC-06-01 改昵称后其他人能查到
- 关联 AC：AC-ACC-06-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. u1：`PATCH /v1/account/profile {nickname:"小明"}`
  2. u2：`users {userIds:["u1"]}`
- 预期结果：
  1. 200；u2 查到昵称"小明"

### TC-ACC-06-02 昵称超长、空白、边界
- 关联 AC：AC-ACC-06-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 分别改为 33 个码点、全空白；再改为 32 个码点、1 个码点
  2. 不带任何字段 `PATCH`
- 预期结果：
  1. 前两个 400 / 4000；32、1 个码点成功；空请求 400 / 4000

### TC-ACC-06-03 avatarFileId：avatar 成功；file 和别人的 404
- 关联 AC：AC-ACC-06-3
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 有 `kind=avatar` 的 `ready` 文件 F1、`kind=file` 的 `ready` 文件 F2；u2 有 `kind=avatar` 的 `ready` 文件 F3
- 步骤：
  1. u1 分别 `PATCH profile {avatarFileId}`：F1、F2、F3
  2. 查 `users.avatar`
- 预期结果：
  1. F1 成功，`avatar` 是 https 地址（≤ 512）；F2、F3 都 404 / 4048，响应相同

### TC-ACC-06-04 头像类型和大小限制
- 关联 AC：AC-ACC-06-4
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 申请 `kind=avatar`，`mime=image/svg+xml`
  2. 申请 `kind=avatar`，`mime=image/png`、`size=3 MiB`
  3. 边界：`size=2 MiB`
- 预期结果：
  1. 依次 415 / 4150、413 / 4139（`details.maxBytes`）、成功

### TC-ACC-06-05 改昵称：本端立即显示，其他端 10 分钟后更新
- 关联 AC：AC-ACC-06-5
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1（[SDK 钟]）
- 前置条件：u1 在 D1，u2 在 D2 已看过 u1 的昵称
- 步骤：
  1. D1 在"我"页改昵称；看"我"页和聊天页
  2. D2 把 [SDK 钟] 推进 9 分钟、再到 10 分钟后，打开和 u1 的聊天
- 预期结果：
  1. D1 立即显示；D2 9 分钟时还是旧值，10 分钟后更新

### TC-ACC-06-06 选图、裁剪、保存：上传 512×512
- 关联 AC：AC-ACC-06-6
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. "我"页选 1000×800 图片，裁剪后保存
  2. 从假 S3 取出头像；看页面
- 预期结果：
  1. 上传的是 512×512；页面显示新头像

### TC-ACC-06-07 头像的私有和公开：绑定前不公开，绑定后可读，不能列举
- 关联 AC：AC-ACC-06-7
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（MinIO 复跑）
- 前置条件：u1 有未 `complete` 的头像 A、已 `complete` 未绑定的头像 B
- 步骤：
  1. 不带签名 GET A 的临时键
  2. 公开桶搜 B 的 `public_key`（为空）；不带签名按公开域名猜测访问
  3. `PATCH profile {avatarFileId:B}`；不带签名 GET `users.avatar`
  4. 匿名 `GET 公开桶/?list-type=2`
- 预期结果：
  1. 第 1 步被拒绝；第 2 步公开桶没有对象
  2. 第 3 步 200，`Content-Type: image/*`，目标存储支持时带 `nosniff`；域名和消息文件不同
  3. 第 4 步被拒绝

### TC-ACC-06-08 已绑定头像不受 24 小时清理；换头像 8 天后旧对象删除
- 关联 AC：AC-ACC-06-8
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 绑定头像 F1；[IM 钟] 冻结
- 步骤：
  1. 推进 25 小时，运行 `file_purge`，GET F1 的公开地址
  2. u1 换头像 F2；推进 7 天减 1 小时，运行清理；再推进 2 小时，运行清理
- 预期结果：
  1. 第 1 步 F1 还在
  2. 第 2 步第一次运行 F1 还在；第二次 F1 的对象和 `final/` 前缀被删，F2 不受影响

### TC-ACC-06-09 对象存储不可用：昵称可改，头像 503
- 关联 AC：AC-ACC-06-9
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：假 S3 注入不可用；u1 有 `ready` 的头像 F1（不可用之前完成）
- 步骤：
  1. `PATCH profile {nickname:"新昵称"}`
  2. `PATCH profile {avatarFileId:F1}`
- 预期结果：
  1. 昵称成功；头像 503 / 5032，`users.avatar` 不变

### TC-ACC-06-10 未绑定头像 25 小时后清除
- 关联 AC：AC-ACC-06-10
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 `complete` 了头像 F1，没有 `PATCH profile`
- 步骤：
  1. [IM 钟] 推进 25 小时，运行 `file_purge`
  2. 查假 S3、`files`；再用 F1 `PATCH profile`
- 预期结果：
  1. 对象和记录都被删除；`PATCH` 404 / 4048

### TC-ACC-06-11 绑定幂等、头像不过期、5 天内绑回不删
- 关联 AC：AC-ACC-06-11
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：[IM 钟] 冻结
- 步骤：
  1. `PATCH profile {avatarFileId:F1}`，丢弃响应后重试
  2. 推进 181 天，运行清理；GET 头像地址
  3. 换 F2；5 天后绑回 F1；再推进 3 天（共第 8 天），运行清理
- 预期结果：
  1. 重试成功，`public_key` 不变；181 天后头像还在
  2. F1 的 `avatar_replaced_at` 被清空，第 8 天不删；F2 进入替换状态

### TC-ACC-06-12 公开 Copy 后杀进程：重试只有一个对象；换头像、注销清除三处
- 关联 AC：AC-ACC-06-12
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：假 S3 暂停点停在"公开桶 Copy 之后"；[IM 钟] 冻结
- 步骤：
  1. u1：`PATCH profile {avatarFileId:F1}`，到暂停点 `kill -9` im-server
  2. 重启，重试同一请求；列公开桶
  3. u1 换头像 F2；推进 8 天运行 `file_purge`；查公开桶、`final/F1/` 前缀、`files`
  4. u2（头像 F3）：`delete`；推进 8 天运行 `account_cleanup`、`file_purge`；查同样三处
- 预期结果：
  1. 第 2 步公开桶只有 1 个 F1 对象
  2. F1、F3 的公开对象、`final/` 前缀、`files` 行都被清除

### TC-ACC-06-13 公开 Copy 后杀进程不重试：25 小时后三处都清除
- 关联 AC：AC-ACC-06-13
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：同上，到暂停点 `kill -9`；重启后不重试
- 步骤：
  1. [IM 钟] 推进 25 小时，运行 `file_purge`
  2. 第一次运行让删除公开对象失败（假 S3 注入），再运行一次
- 预期结果：
  1. 第一次失败时 `files` 行还在（`purging`）
  2. 第二次后公开桶、`final/F1/`、`files` 行都已清除

## F-ACC-07 与演示登录、管理接口并存

### TC-ACC-07-01 演示模式下现有 e2e 不改代码全过
- 关联 AC：AC-ACC-07-1
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1（`IM_DEMO_LOGIN=1`）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `git diff` 确认 `server/e2e` 现有文件没有改动（新增文件除外）
  2. `IM_DEMO_LOGIN=1`，默认配置，`go test ./e2e/... `
- 预期结果：
  1. 全部通过；现有文件没有改动

### TC-ACC-07-02 演示用户不能密码登录；账号用户不能演示登录
- 关联 AC：AC-ACC-07-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_DEMO_LOGIN=1`；`/demo/login` 产生 `demo1`；已注册 `acc1`，昵称"甲"
- 步骤：
  1. `login(demo1, 任意密码)`
  2. `POST /demo/login {username:"acc1", nickname:"乙"}`
  3. 查 `acc1` 的昵称
- 预期结果：
  1. 第 1 步 401 / 4018
  2. 第 2 步 403 / 4030，`reason=accountExists`，没有 token；昵称还是"甲"

### TC-ACC-07-03 生产带 token.issue：不确认就不启动；确认后告警
- 关联 AC：AC-ACC-07-3
- 优先级：P0
- 批次：C1
- 层级：后端 e2e + 单测
- 环境：E1 单节点
- 前置条件：`IM_ENV=production`，调用方配置含 `token.issue`，其他生产配置齐全
- 步骤：
  1. 启动，不设 `IM_ALLOW_TOKEN_ISSUE`
  2. 设 `IM_ALLOW_TOKEN_ISSUE=1` 启动；`GET /internal/v1/config`
  3. 调用一次 `POST /admin/users/token`，查 `admin_audit`
- 预期结果：
  1. 第 1 次非 0 退出，指出这一项
  2. 第 2 次启动成功；`productionChecks` 有 `token_issue_caller` 告警；审计有记录

### TC-ACC-07-04 GET /v1/config 和配置一致，没有密钥
- 关联 AC：AC-ACC-07-4
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 分别用 `closed`、`invite`、`open`、`IM_DEMO_LOGIN=0/1`、`IM_FILES_ENABLED=0/1` 启动，不带登录请求 `GET /v1/config`
  2. 搜响应的 `key`、`secret`、JWT 密钥、refresh 密钥、对象存储密钥
- 预期结果：
  1. `registerMode`、`demoLogin`、`features`、`limits` 和配置一致；搜不到任何密钥

### TC-ACC-07-05 demoLogin=false、invite：账号登录表单，没有演示表单和"业务方"
- 关联 AC：AC-ACC-07-5
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 打开登录页；点"注册"
- 预期结果：
  1. 有用户名、密码、"注册"入口；注册页有邀请码输入框
  2. 没有演示表单；页面文本搜不到"业务方"

### TC-ACC-07-06 demoLogin=true：演示表单照常
- 关联 AC：AC-ACC-07-6
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 打开登录页；执行 TC-UI-03-01～08 对应的步骤
- 预期结果：
  1. 现有演示表单照常，有"使用账号登录"入口；AC-UI-03-1～8 通过

### TC-ACC-07-07 构建开关是上限：任一为假都不显示演示表单
- 关联 AC：AC-ACC-07-7
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 构建开关关（`VITE_DEMO_LOGIN=0`、Android `build.gradle.kts`、iOS `Config.swift`）+ `demoLogin=true`，打开登录页
  2. 构建开关开 + `demoLogin=false`，打开登录页
- 预期结果：
  1. 都不显示演示表单

### TC-ACC-07-08 演示会话没有"账号安全"
- 关联 AC：AC-ACC-07-8
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 演示登录，打开"我 → 设置"
  2. 账号登录，打开"我 → 设置"
- 预期结果：
  1. 演示会话没有"账号安全"；账号会话有

### TC-ACC-07-09 closed 没有注册入口；config 失败时不猜测模式
- 关联 AC：AC-ACC-07-9
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `registerMode=closed`，打开登录页
  2. 让 `GET /v1/config` 失败，打开登录页（从未成功过）
  3. 恢复后点重试
- 预期结果：
  1. 第 1 步没有注册入口
  2. 第 2 步显示"网络异常，请稍后重试"和重试按钮，不显示任何登录或演示表单
  3. 第 3 步按配置显示

## F-ACC-08 注销账号

### TC-ACC-08-01 注销全流程：错密码、踢线、冷静期、到期
- 关联 AC：AC-ACC-08-1
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 在 D1 在线；[IM 钟] 第 0 天
- 步骤：
  1. `POST /v1/account/delete {password:错误}`
  2. `delete {password:正确}`；观察 D1
  3. 第 3 天：`login(u1, 正确密码)`，不撤销
  4. 第 3 天：`auth(旧 token)`
  5. 第 8 天运行 `account_cleanup`：`login(u1, 正确密码)`；`register(u1)`
- 预期结果：
  1. 错误 401 / 4018，没有改动
  2. D1 收到 `kicked/accountDeleted`，以 4012 关闭
  3. 第 3 天登录 403 / 4030，`reason=accountDeletionPending`，带 `details.deleteAt`，没有 token；旧 token 收到 `kicked/accountDeleted`
  4. 第 8 天登录 401 / 4018；注册 409 / 4095

### TC-ACC-08-02 注销到期后：历史还在，发消息和签发 404
- 关联 AC：AC-ACC-08-2
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u2 和 u1 有单聊；g1 有 u1 的消息；u1 注销到期并清理
- 步骤：
  1. u2 `sync` 单聊和 g1
  2. u2 `users {userIds:["u1"]}`
  3. u2 给 u1 发消息
  4. `POST /admin/users/token {userId:"u1"}`
- 预期结果：
  1. 历史都在；昵称"已注销用户"
  2. 发消息 4042；签发 HTTP 404 / 4042

### TC-ACC-08-03 群主注销：自建群转让，托管群回到无群主
- 关联 AC：AC-ACC-08-3
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 是自建群 g1 的群主（有管理员 a1 和普通成员）；u1 是托管群 g2 的群主
- 步骤：
  1. u1 注销到期，运行 `account_cleanup`
  2. 查 g1、g2 成员和事件
- 预期结果：
  1. g1 群主转给最早的管理员 a1，成员收到 `ownerChanged`
  2. g2 回到无群主；u1 都不在成员里

### TC-ACC-08-04 撤销注销：匿名接口，错密码 401，对后恢复
- 关联 AC：AC-ACC-08-4
- 优先级：P1
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1 第 0 天申请注销
- 步骤：
  1. 第 3 天不带 token：`POST /v1/account/delete/cancel {username, password:错误}`
  2. 同样带正确密码
  3. `login(u1, 原密码)`
  4. 第 8 天运行 `account_cleanup`
- 预期结果：
  1. 错误 401 / 4018；正确后 200；可以登录，密码不变
  2. 清理不处理 u1

### TC-ACC-08-05 600 个群的清理：中途失败可继续，不影响其他用户消息
- 关联 AC：AC-ACC-08-5
- 优先级：P1
- 批次：C1
- 层级：后端 e2e + 性能
- 环境：E1（开发机只作参考）
- 前置条件：u1 在 600 个群；[IM 钟] 第 8 天
- 步骤：
  1. 让 `account_cleanup` 处理 300 个群时报错退出
  2. 再次运行；运行期间其他用户发 ≥ 1000 条消息，记录 ack 延迟
  3. 查 u1 所在群、提示消息
- 预期结果：
  1. 最终退出全部 600 个群；每个群只有一条提示消息
  2. 处理期间 ack P99 ≤ 200 ms（未校准）；开发机结果只作参考，目标机重测（P-12）

### TC-ACC-08-06 注销确认框的文字；密码错误原地提示
- 关联 AC：AC-ACC-08-6
- 优先级：P1
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. "账号安全 → 注销账号"，查看确认框
  2. 输入错误密码提交
- 预期结果：
  1. 确认框有"7 天后不可恢复，用户名不能再使用，已发的消息会保留"
  2. 错误时原地提示，留在页面，账号没有变化

### TC-ACC-08-07 注销清理删除头像对象
- 关联 AC：AC-ACC-08-7
- 优先级：P1
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 绑定了头像；记下公开地址
- 步骤：
  1. u1 注销，[IM 钟] 推进 8 天，运行 `account_cleanup`、`file_purge`
  2. GET 旧头像地址
- 预期结果：
  1. 对象不存在；`final/{fileId}/` 没有对象

## F-ACC-09 邀请码与后台账号信息

### TC-ACC-09-01 运营生成 maxUses=2，三个用户注册，后台显示已用 2 次
- 关联 AC：AC-ACC-09-1
- 优先级：P0
- 批次：C1
- 层级：端到端
- 环境：E1 + console-server + tianting-admin
- 前置条件：console-server + tianting-admin + im-server；`operator` 账号 op1 已登录；`invite` 模式；注册走 H5、Android、iOS 三端各一次（各端用不同邀请码）
- 步骤：
  1. op1 在"邀请码"页生成 maxUses=2 的邀请码，复制明文
  2. 用户 r1、r2 分别在 H5（或 Android、iOS）注册页填邀请码注册
  3. 用户 r3 用同一邀请码注册
  4. op1 刷新邀请码列表
- 预期结果：
  1. r1、r2 成功；r3 403 / 4030，页面提示"邀请码无效"
  2. 列表显示已用 2 次，只显示后 4 位，明文只出现一次

### TC-ACC-09-02 生成邀请码：库、审计、日志都没有明文
- 关联 AC：AC-ACC-09-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 + console-server + tianting-admin
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `POST /console/api/...` 或 `POST /admin/invites` 生成一个，记下明文
  2. 搜 `invites` 表、`admin_audit`、console 审计、两边日志
- 预期结果：
  1. 都搜不到明文；只有哈希和后 4 位

### TC-ACC-09-03 已停用的邀请码不能用
- 关联 AC：AC-ACC-09-5
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `POST /admin/invites/{id}/disable`
  2. 用这个邀请码注册；再次停用
- 预期结果：
  1. 注册 403 / 4030，`reason=inviteInvalid`；重复停用幂等

### TC-ACC-09-04 权限：support、auditor 403，operator 成功
- 关联 AC：AC-ACC-09-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 + console-server + tianting-admin
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 分别用 `support`、`auditor`、`operator` 账号生成邀请码
  2. 查审计
- 预期结果：
  1. 前两个 403 / 4030；第三个成功；三次都有审计记录（denied 按基座规则）

### TC-ACC-09-05 单次 101 个、maxUses=1001 被拒；100、1000 的边界
- 关联 AC：AC-ACC-09-7
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 + console-server + tianting-admin
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 生成 `count=101`；`maxUses=1001`
  2. 生成 `count=100`、`maxUses=1000`
- 预期结果：
  1. 依次 400 / 4131、400 / 4000；边界成功

### TC-ACC-09-06 审计写失败：邀请码不产生
- 关联 AC：AC-ACC-09-8
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 + console-server + tianting-admin
- 前置条件：console-server 故障注入审计写失败（`POST /console/test/faults {auditWrite}`）
- 步骤：
  1. 生成邀请码；查 `invites`
- 预期结果：
  1. 503 / 5031，没有新行

### TC-ACC-09-07 maxUses=1，10 个并发注册只成功 1 个
- 关联 AC：AC-ACC-09-9
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 10 个不同用户名、不同 IP 同时用同一邀请码注册
- 预期结果：
  1. 恰好 1 个成功，其余 403 / 4030；`usedCount=1`

### TC-ACC-09-08 邀请码至少 128 位随机
- 关联 AC：AC-ACC-09-10
- 优先级：P0
- 批次：C1
- 层级：单测
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 生成 1000 个，解码得到原始字节
- 预期结果：
  1. 每个 ≥ 16 字节，没有重复；代码用 `crypto/rand`

### TC-ACC-09-09 用户详情显示账号信息，没有重置密码
- 关联 AC：AC-ACC-09-11
- 优先级：P0
- 批次：C1
- 层级：Playwright（tianting-admin）
- 环境：E1 + console-server + tianting-admin
- 前置条件：客服 `support` 登录；u1 是账号用户，有 2 条会话
- 步骤：
  1. 打开 u1 的用户详情
- 预期结果：
  1. 显示有账号、注册时间、最近登录时间、设备会话数 2
  2. 页面没有"重置密码""重置恢复码"一类操作和链接

### TC-ACC-09-10 用完 91 天的邀请码被清除
- 关联 AC：AC-ACC-09-12
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：I1 用完；I2 用完 89 天；I3 过期 91 天；[IM 钟] 可控
- 步骤：
  1. 运行 `invite_purge`
- 预期结果：
  1. I1（用完 91 天）、I3 被清除，I2 还在

### TC-ACC-09-11 账号信息接口：没有哈希和令牌；演示用户显示没有账号
- 关联 AC：AC-ACC-09-13
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `GET /admin/accounts/{账号用户}`
  2. `GET /admin/accounts/{演示用户}`
- 预期结果：
  1. 前者有 `hasAccount=true`、注册和最近登录时间、会话数；响应搜不到 `hash`、`token`、`sid`
  2. 后者 `hasAccount=false`

### TC-ACC-09-12 邀请总量超过 500 告警
- 关联 AC：AC-ACC-09-14
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 生成 maxUses=600 的邀请码；`GET /internal/v1/config`
  2. 停用后再查
- 预期结果：
  1. `productionChecks` 有邀请总量告警；停用后消失

## F-ACC-10 隐私告知入口

### TC-ACC-10-01 三个入口都能打开隐私说明；加载失败的提示
- 关联 AC：AC-ACC-10-1
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：配置 `IM_PRIVACY_NOTICE_URL`、`IM_PRIVACY_NOTICE_VERSION=v1`
- 步骤：
  1. 分别从登录页底部、注册页、"我 → 设置 → 关于天听"打开
  2. 让地址返回 500，再打开
  3. 非生产环境配置为空，打开这些页面
- 预期结果：
  1. 都有入口，能打开
  2. 加载失败提示"暂时打不开，请稍后重试"，不阻止登录
  3. 配置为空时入口隐藏

### TC-ACC-10-02 注册记录同意的版本
- 关联 AC：AC-ACC-10-2
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：`IM_PRIVACY_NOTICE_VERSION=v1`
- 步骤：
  1. 注册 u1；查 `accounts`
- 预期结果：
  1. `notice_version=v1`

### TC-ACC-10-03 版本更新：下次登录提示一次
- 关联 AC：AC-ACC-10-3
- 优先级：P0
- 批次：C1
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 单节点
- 前置条件：u1 同意过 v1；配置改为 v2
- 步骤：
  1. 登录，查看提示，确认
  2. 退出再登录
- 预期结果：
  1. 第一次提示一次，确认后 `notice_version=v2`；第二次不提示
  2. 不确认也能继续使用（第一批不做强制）

---

### TC-ACC-03-49 吊销之后 refresh 的 reason 按原因返回
- 关联 AC：AC-ACC-03-49
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：5 条设备会话，每条保留 `mac` 正确的 refresh
- 步骤：
  1. 第 1 条 `logout`，第 2 条管理接口强制下线，第 3 条封禁用户
  2. 第 4 条推进到过期
  3. 第 5 条转正后推进 3 分钟，用旧值触发重放吊销，再用旧值请求一次
  4. 分别刷新
- 预期结果：
  1. 都是 401 / 4019，`reason` 依次 `revoked`、`revoked`、`revoked`、`expired`、`replayDetected`

### TC-ACC-03-50 恢复数据库后吊销全部会话
- 关联 AC：AC-ACC-03-50
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1 单节点
- 前置条件：u1、u2 各有 1 条在线设备会话
- 步骤：
  1. 运行 `im-server sessions revoke-all`
  2. 用旧 refresh 刷新，用旧 access `auth`
  3. 查 `im_refresh_replay_total`
  4. 用密码重新登录
- 预期结果：
  1. 在线连接被 `kicked/sessionRevoked` 踢下；刷新 4019 `reason=revoked`；指标不变；重新登录成功

## 说明

分发时发现的矛盾已在 [PRD-chat v1.1](../prd/PRD-chat-v1.1.md) 第 22 节修正；C1 验收发现的口径问题在 [v1.2](../prd/PRD-chat-v1.2.md) 第 22 节修正（TC-ACC-03-08 两种文案、新增 TC-ACC-03-51）。用例和 AC 一致。

- AC-ACC-04-29 只有 Postgres 实现后才能跑，C1 验收记未执行。
- AC-ACC-03-19、03-27、04-6 的推送令牌部分 C3 上线时验。
- AC-ACC-04-31 的"攻击者第 7 次 429"要发生在受害者找回之前，受害者成功后攻击者的码已作废。
