# 测试用例：聊天产品非功能（CHAT，C1、C2、C3、C4 部分）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 15 节 F-CHAT-90。任务单：[backend-chat](../tasks/backend-chat.md)、[frontend-chat](../tasks/frontend-chat.md)。
> 范围：标了【C1】【C2】【C3】【C4】【全部】的 AC-CHAT（CH-27）。**【全部】的 AC 每个里程碑发布前跑一次，这里写到 C3 为止的范围**（第 15 节累计表）：C1、C2 是 TC-CHAT-90-01～17，C3 增加的部分是 TC-CHAT-90-19～25（C3 已暂停），C4 的 AC-CHAT-90-7 和 C4 增加的部分是 TC-CHAT-90-26～34（AC-CHAT-90-3 在 C4 没有新增依赖，不另写），C5、C6 增加的部分随对应里程碑。
> 判定（R-CQ-22）：主判据用绝对值，增幅只记录；每条写明负载和样本数（≥ 1000），生产配置，3 次取最差，报告写基线提交号。**所有数字都未校准；开发机结果只记录，不算通过，发布判定等 P-12**。
>
> 通用约定：
> - 环境：**E1 单节点**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库；`IM_TRUSTED_PROXIES` 配成测试客户端，换 IP 用 `X-Forwarded-For`，PRD 第 4.5 节）；**E1 + 假 S3**（再加 `objstoretest` 独立进程，`IM_FILES_ENABLED=1`，公开桶和私有桶都在假 S3 上）；**E1 + console-server + tianting-admin**（`CON_TEST_MODE=1`，Postgres）；**真实 MinIO**（`deploy/testenv-ci.sh` 的固定版本）；**EP 性能**（目标机，P-12）；**Postgres**（B6 之后）；**模拟器**（Android 模拟器、iOS Simulator）；**真机**（P-13）。开发机不安装软件。
> - **CI 现在不运行（PENDING P-21）**，所有结果都是本地结果，记录时写明环境和提交号。性能用例在开发机的结果只记录，**不算通过**，发布判定等 P-12。真机用例在 P-13 到位前记未执行。
> - 层级：**后端 e2e**（Go e2e，对 `/v1/*`、`/ws`、`/admin/*` 断言）、**单测**、**SDK 单测**（三套 SDK，假 HTTP + `FakeTransport` + 假时钟，向量用例按 `protocol/vectors`）、**Playwright**（H5，Chromium + WebKit，连真实 im-server 测试模式）、**原生 UI 测试**（Compose UI Test、XCUITest，模拟器）、**端到端**（真实 im-server + 客户端）、**合同测试**（`tools/objstore-contract`）、**性能**、**手工**（留截图或录屏）。标了 Playwright 和原生 UI 测试的前端用例，H5、Android、iOS 都要跑，某端不适用的按 AC 的括号。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算，避免跨日（R-CQ-41）；**[SDK 钟]** 各 SDK 的 `Clock` 注入；**[真实钟]** 依赖对象存储的真实时间。**预签名一律用真实时钟**，推进 [IM 钟] 不影响签名（第 4.4 节）。
> - 负向断言"假通道、接口无调用"用哨兵法（第 4.4 节）。每个用例使用独立数据库；用例之间用 `/admin/test/limits/reset {scope:"auth"}` 清计数。
> - 批次字段：C1、C2 按 AC 的标记；【全部】的 AC 只验证到 C2 为止的部分（PRD 第 15 节累计表）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-CHAT-90-01 | ack P99：账号 token 单聊文本 | AC-CHAT-90-1 | 性能 | P0 |
| TC-CHAT-90-02 | 开关全关：现有 e2e 全过，TC-PERF 判定通过 | AC-CHAT-90-2 | 后端 e2e + 性能 | P0 |
| TC-CHAT-90-03 | 对象存储不可用：文本收发不受影响，/readyz 仍就绪 | AC-CHAT-90-3 | 性能 | P0 |
| TC-CHAT-90-04 | 日志和审计搜不到特征串（C1 流程） | AC-CHAT-90-5 | 后端 e2e + 部署 e2e | P0 |
| TC-CHAT-90-05 | 日志和审计搜不到特征串（C2 流程） | AC-CHAT-90-5 | 后端 e2e + 部署 e2e | P0 |
| TC-CHAT-90-06 | 迁移：全新库和 0014 库都能升级，大小写用户不变 | AC-CHAT-90-6 | 后端 e2e + 存储契约测试 | P0 |
| TC-CHAT-90-07 | 新增界面符合 B-1 和 B-3，图标在清单 | AC-CHAT-90-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CHAT-90-08 | axe-core、深色模式、键盘（C1、C2 页面） | AC-CHAT-90-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CHAT-90-09 | 全部新帧向量（C1、C2）：服务端和三套 SDK 都跑 | AC-CHAT-90-10 | 单测（四方） | P0 |
| TC-CHAT-90-10 | 登录 P99 和内存 | AC-CHAT-90-11 | 性能 | P0 |
| TC-CHAT-90-11 | 申请上传 P99 | AC-CHAT-90-12 | 性能 | P0 |
| TC-CHAT-90-12 | 错误码一致性纳入 client scope | AC-CHAT-90-14 | 单测（四方） | P0 |
| TC-CHAT-90-13 | 共享用例（C1、C2） | AC-CHAT-90-15 | SDK 单测（三套） | P0 |
| TC-CHAT-90-14 | /v1 的基础约束：65 KB、慢请求、http、过期和被下线的 token | AC-CHAT-90-16 | 后端 e2e | P0 |
| TC-CHAT-90-15 | 指标和配置检查（C1） | AC-CHAT-90-17 | 后端 e2e | P0 |
| TC-CHAT-90-16 | 指标和配置检查（C2） | AC-CHAT-90-17 | 后端 e2e | P0 |
| TC-CHAT-90-17 | 生产模式拒绝测试模式和测试接口 | AC-CHAT-90-18 | 后端单测 + e2e | P0 |
| TC-CHAT-90-18 | 推送：入队到假通道收到 P99 | AC-CHAT-90-13 | 性能 | P0 |
| TC-CHAT-90-19 | 开关全关（C3：推送开关） | AC-CHAT-90-2 | 后端 e2e + 性能 | P0 |
| TC-CHAT-90-20 | APNs、FCM 不可用：文本收发不受影响，/readyz 仍就绪 | AC-CHAT-90-3 | 性能 | P0 |
| TC-CHAT-90-21 | 日志和审计搜不到特征串（C3 推送流程） | AC-CHAT-90-5 | 后端 e2e + 部署 e2e | P0 |
| TC-CHAT-90-22 | 通知设置页符合 B-1 和 B-3 | AC-CHAT-90-8 | 原生 UI 测试 | P0 |
| TC-CHAT-90-23 | 通知设置页：无障碍、深色模式 | AC-CHAT-90-9 | 原生 UI 测试 | P0 |
| TC-CHAT-90-24 | 新帧向量（C3）：服务端和三套 SDK 都跑 | AC-CHAT-90-10 | 单测（四方） | P0 |
| TC-CHAT-90-25 | 指标和配置检查（C3） | AC-CHAT-90-17 | 后端 e2e | P0 |
| TC-CHAT-90-26 | 老版本三套 SDK 收到撤回：不崩溃，显示已删除，搜不到原文 | AC-CHAT-90-7 | SDK 单测（三套，老版本） | P0 |
| TC-CHAT-90-27 | ack P99：带引用的单聊、带 @ 的群聊 | AC-CHAT-90-1 | 性能 | P0 |
| TC-CHAT-90-28 | 开关全关（C4：IM_MSG_ACTIONS_ENABLED 关） | AC-CHAT-90-2 | 后端 e2e + 性能 | P0 |
| TC-CHAT-90-29 | 日志和审计搜不到特征串（C4：撤回、引用、@） | AC-CHAT-90-5 | 后端 e2e + 部署 e2e | P0 |
| TC-CHAT-90-30 | 菜单和引用条符合 B-1 和 B-3，图标在清单 | AC-CHAT-90-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CHAT-90-31 | C4 页面：axe-core、深色模式、键盘打开消息菜单 | AC-CHAT-90-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-CHAT-90-32 | 新帧向量（C4）：服务端和三套 SDK 都跑 | AC-CHAT-90-10 | 单测（四方） | P0 |
| TC-CHAT-90-33 | 共享用例（C4：撤回、引用、提及） | AC-CHAT-90-15 | SDK 单测（三套） | P0 |
| TC-CHAT-90-34 | 指标和配置检查（C4） | AC-CHAT-90-17 | 后端 e2e | P0 |

---

## F-CHAT-90 非功能

### TC-CHAT-90-01 ack P99：账号 token 单聊文本
- 关联 AC：AC-CHAT-90-1
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：性能
- 环境：EP（目标机 P-12）
- 前置条件：C1 范围：单聊文本，账号 token；C2 同 C1（表里"不变"）。生产配置；记录基线提交号。**开发机结果只记录，不算通过；发布判定等 P-12**
- 步骤：
  1. `im-loadtest` 500 条/秒，≥ 1000 条，跑 3 次
  2. 每次记录 ack P99、错误率；和基线的增幅
- 预期结果：
  1. 3 次最差 ack P99 ≤ 200 ms（未校准），错误率 0；增幅只记录
- 说明：C3～C6 增加的范围不在本批

### TC-CHAT-90-02 开关全关：现有 e2e 全过，TC-PERF 判定通过
- 关联 AC：AC-CHAT-90-2
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：后端 e2e + 性能
- 环境：E1；EP（P-12）
- 前置条件：C1：C1 之外的开关本来就不存在；C2：`IM_FILES_ENABLED=0`。其他开关随 C3～C6 实现，本批不验证
- 步骤：
  1. 跑 `go test ./e2e/...`，确认现有文件没有改动
  2. 跑 TC-PERF（开发机记录；目标机判定）
- 预期结果：
  1. e2e 全过；TC-PERF 开发机结果只记录，发布判定等 P-12

### TC-CHAT-90-03 对象存储不可用：文本收发不受影响，/readyz 仍就绪
- 关联 AC：AC-CHAT-90-3
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：性能
- 环境：ES；EP（P-12）
- 前置条件：C1 不适用（`AC-CHAT-90-3` 表里写明），C2 只验对象存储。假 S3 分别按"拒绝连接"（停进程）和"黑洞"（`iptables` 或假 S3 接受连接不回应）不可用
- 步骤：
  1. 每种方式：文本收发 5 分钟，≥ 1000 条
  2. 期间请求 `/readyz`；申请一次上传；`complete` 一次（确认不阻塞发送）
- 预期结果：
  1. 全部成功；ack P99 ≤ 200 ms（未校准，开发机不算通过）
  2. `/readyz` 200；申请 200；`complete` 503 / 5032，不占连接池到超时

### TC-CHAT-90-04 日志和审计搜不到特征串（C1 流程）
- 关联 AC：AC-CHAT-90-5
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：后端 e2e + 部署 e2e
- 环境：E1 + console-server
- 前置条件：C1 范围：注册、登录、刷新、改密、找回。特征串：密码 `ZXQPW`、恢复码、邀请码 `ZXQIN`、access、refresh、`deviceName` `ZXQDEV`
- 步骤：
  1. 走完这 5 个流程；含失败路径
  2. 用 `deploy/ops/scan-logs` 扫 im-server、console-server 日志和两边审计
- 预期结果：
  1. 都搜不到；`sid` 原值也搜不到

### TC-CHAT-90-05 日志和审计搜不到特征串（C2 流程）
- 关联 AC：AC-CHAT-90-5
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：后端 e2e + 部署 e2e
- 环境：ES + console-server
- 前置条件：C2 范围：上传、下载。特征串：预签名查询串、文件名 `ZXQ-FILE`
- 步骤：
  1. 上传、取地址、下载、过期清理；含失败路径（415、4292）
  2. 再扫一遍
- 预期结果：
  1. 都搜不到
- 说明：推送令牌、引用文字、举报备注、好友附言属于 C3～C6，本批不验证

### TC-CHAT-90-06 迁移：全新库和 0014 库都能升级，大小写用户不变
- 关联 AC：AC-CHAT-90-6
- 优先级：P0
- 批次：C1
- 层级：后端 e2e + 存储契约测试
- 环境：E1 单节点
- 前置条件：夹具 A：全新 SQLite；夹具 B：`0014` 版本库，含 `Alice`、`alice`，有消息和群
- 步骤：
  1. 分别 `im-server migrate up`
  2. 比较 B 升级前后的 `users`、`messages`、`members` 行
  3. 用 `0014` 版本的二进制启动升级后的库
- 预期结果：
  1. 都成功；已有数据不变；`Alice`、`alice` 都在，`accounts` 没有行
  2. 旧版本二进制按现有规则拒绝启动
- 说明：Postgres 随 B6 跑同一组

### TC-CHAT-90-07 新增界面符合 B-1 和 B-3，图标在清单
- 关联 AC：AC-CHAT-90-8
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：构建产物扫描 + Playwright
- 前置条件：C1：账号页面；C2：文件页面。菜单和其他入口属于 C3～C6
- 步骤：
  1. 跑现有 AC-UI-90-15、AC-UI-11-3 的扫描（文案、品牌、资源）
  2. 对照 PRD-ui 6.6 节图标清单
- 预期结果：
  1. 没有其他产品的品牌和截图；新增图标都在清单；文案符合 B-3

### TC-CHAT-90-08 axe-core、深色模式、键盘（C1、C2 页面）
- 关联 AC：AC-CHAT-90-9
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright + 原生无障碍检查
- 前置条件：范围：注册、找回、保存恢复码、"账号安全"、图片和文件气泡（第 13.4 节）。"键盘能打开消息菜单"属于 C4，本批只查新页面的键盘可达性
- 步骤：
  1. 跑 axe-core；原生用 Accessibility Inspector、Espresso AccessibilityChecks
  2. 深色模式截图；扫描颜色是否来自 token
  3. H5 只用键盘走完注册、找回、保存恢复码、图片全屏
- 预期结果：
  1. 没有 `button-name` 违规；新增颜色都来自 token
  2. 键盘能完成这些流程，焦点可见

### TC-CHAT-90-09 全部新帧向量（C1、C2）：服务端和三套 SDK 都跑
- 关联 AC：AC-CHAT-90-10
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：单测（四方）
- 环境：各仓库
- 前置条件：C1：`kickReasons`、`kicked.recoverBefore`、`securityNotice`；C2：`image`、`file`、`MsgBody` 收紧、`modVer`
- 步骤：
  1. 服务端跑 `protocol/vectors/frames.json` 校验（合法 + 非法各一组）
  2. sdk-ts、sdk-kotlin、sdk-swift 分别跑；sdk-ts 增加 Schema 校验
  3. `MsgBody` 夹带字段的非法帧逐条确认被拒绝
- 预期结果：
  1. 全部通过；旧 frames 用例不改

### TC-CHAT-90-10 登录 P99 和内存
- 关联 AC：AC-CHAT-90-11
- 优先级：P0
- 批次：C1
- 层级：性能
- 环境：EP（目标机 P-12）
- 前置条件：生产配置，Argon2 m=64 MiB、t=3。**开发机只记录，不算通过**
- 步骤：
  1. 串行 ≥ 1000 次；4 并发 ≥ 1000 次；每种 3 次取最差
  2. 记录 P99、峰值存活内存（`go_gc_heap_live_bytes`）增量
- 预期结果：
  1. P99 ≤ 500 ms（未校准）；峰值存活内存增加 ≤ 300 MiB（未校准）

### TC-CHAT-90-11 申请上传 P99
- 关联 AC：AC-CHAT-90-12
- 优先级：P0
- 批次：C2
- 层级：性能
- 环境：EP（目标机 P-12）
- 前置条件：假 S3 或 MinIO；开发机只记录
- 步骤：
  1. ≥ 1000 次 `POST /v1/files/uploads`，3 次取最差
- 预期结果：
  1. P99 ≤ 200 ms（未校准）

### TC-CHAT-90-12 错误码一致性纳入 client scope
- 关联 AC：AC-CHAT-90-14
- 优先级：P0
- 批次：C1
- 层级：单测（四方）
- 环境：各仓库
- 前置条件：`errors.json` 已加 4018、4019、4095、4294 等
- 步骤：
  1. 跑 F-SYS-01 一致性测试（服务端常量、三套 SDK 枚举）
  2. 故意从 sdk-ts 枚举删一个，确认测试失败，再还原
- 预期结果：
  1. 一致；删除时失败

### TC-CHAT-90-13 共享用例（C1、C2）
- 关联 AC：AC-CHAT-90-15
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：SDK 单测（三套）
- 环境：各 SDK 仓库
- 前置条件：C1：`account-session.json`；C2：`chat-sync.json` 的合并和过期、`account-session.json` 的附件状态机。撤回、引用、提及、置顶等用例随 C3～C6
- 步骤：
  1. Go 参考客户端产出期望值
  2. 三套 SDK 用假 HTTP + 假传输层 + 假时钟跑这些向量
- 预期结果：
  1. 全部通过；每个用例名在三套 SDK 的测试报告里都出现

### TC-CHAT-90-14 /v1 的基础约束：65 KB、慢请求、http、过期和被下线的 token
- 关联 AC：AC-CHAT-90-16
- 优先级：P0
- 批次：C1
- 层级：后端 e2e
- 环境：E1；生产配置
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `POST /v1/account/login` 请求体 65 KB；边界 64 KB
  2. 用慢速客户端发 11 秒（每秒 1 字节）
  3. 生产配置（`IM_ENV=production`）下直连 http 请求
  4. Bearer 用过期 token；被强制下线的 token 调 `GET /v1/account/events`
- 预期结果：
  1. 400 / 4000（64 KB 进入业务校验）；慢请求被断开；http 被拒绝
  2. 401 / 4011；401 / 4012

### TC-CHAT-90-15 指标和配置检查（C1）
- 关联 AC：AC-CHAT-90-17
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：后端 e2e
- 环境：E1 + `/metrics`
- 前置条件：C1：账号指标
- 步骤：
  1. 走注册、登录、刷新、重放
  2. 抓 `/metrics`、`GET /internal/v1/config`
  3. 跑 AC-OPS-90-4 的基数测试
- 预期结果：
  1. 出现 `im_account_logins_total{result}`、`im_registers_total{result}`、`im_refresh_reissue_total`、`im_refresh_replay_total`
  2. 配置检查有注册模式、邀请码总量告警、隐私说明版本、`token.issue` 放行告警
  3. 标签没有 userId 等无界字段

### TC-CHAT-90-16 指标和配置检查（C2）
- 关联 AC：AC-CHAT-90-17
- 优先级：P0
- 批次：C1、C2（【全部】，只验证本批部分）
- 层级：后端 e2e
- 环境：ES + `/metrics`
- 前置条件：C2：文件指标
- 步骤：
  1. 走上传、取地址、下载、失败路径
  2. 抓 `/metrics`、`GET /internal/v1/config`
- 预期结果：
  1. 出现 `im_file_uploads_total{kind,result}`、`im_file_bytes_total`、`im_file_url_issued_total`、`im_objstore_errors_total`
  2. 配置检查有对象存储连通和 HTTPS
  3. 标签没有 userId、fileId、文件名
- 说明：推送、撤回、治理的指标和配置检查属于 C3～C6

### TC-CHAT-90-17 生产模式拒绝测试模式和测试接口
- 关联 AC：AC-CHAT-90-18
- 优先级：P0
- 批次：C1
- 层级：后端单测 + e2e
- 环境：E1 单节点
- 前置条件：生产配置
- 步骤：
  1. 设 `IM_TEST_MODE=1` 启动
  2. 正常生产配置启动，请求第 4.5 节的测试接口和钩子
- 预期结果：
  1. 前者拒绝启动并报明原因；后者接口不存在，钩子不生效

### TC-CHAT-90-18 推送：入队到假通道收到 P99
- 关联 AC：AC-CHAT-90-13
- 优先级：P0
- 批次：C3
- 层级：性能
- 环境：E1 + 假通道（[TC-APN](TC-APN.md) 的环境，开发机只记录）；EP（P-12，判定）
- 前置条件：生产配置（只有通道端点指向假通道）；接收方离线、有令牌，每条消息发给不同接收方或超过合并窗口，避免被合并
- 步骤：
  1. 发 ≥ 1000 条，3 次取最差
  2. 记录每条的入队时间（服务端测试埋点）和假通道收到请求的时间
- 预期结果：
  1. P99 ≤ 2 秒（未校准）；开发机结果只记录，不算通过

### TC-CHAT-90-19 开关全关（C3：推送开关）
- 关联 AC：AC-CHAT-90-2
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：后端 e2e + 性能
- 环境：E1；EP（P-12）
- 前置条件：`IM_FILES_ENABLED=0`、`IM_OFFLINE_PUSH_ENABLED=0`（第 15 节累计表：C3 增加"推送开关关"）
- 步骤：
  1. 跑 `go test ./e2e/...`，确认现有文件没有改动
  2. 跑 TC-PERF（开发机记录；目标机判定）
- 预期结果：
  1. e2e 全过；新命令返回 4030 `featureDisabled`；TC-PERF 开发机结果只记录

### TC-CHAT-90-20 APNs、FCM 不可用：文本收发不受影响，/readyz 仍就绪
- 关联 AC：AC-CHAT-90-3
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：性能
- 环境：E1 + 假通道；EP（P-12）
- 前置条件：接收方一半离线有令牌；假 APNs、假 FCM 分别按"拒绝连接"（停监听）和"黑洞"（接受连接不回应）不可用
- 步骤：
  1. 每种方式：文本收发 5 分钟，≥ 1000 条
  2. 期间请求 `/readyz`
- 预期结果：
  1. 全部成功；ack P99 ≤ 200 ms（未校准，开发机不算通过）
  2. `/readyz` 200；推送按重试用尽计数，队列不超过上限

### TC-CHAT-90-21 日志和审计搜不到特征串（C3 推送流程）
- 关联 AC：AC-CHAT-90-5
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：后端 e2e + 部署 e2e
- 环境：E1 + 假通道 + console-server
- 前置条件：C3 范围：推送。特征串：推送令牌 `ZXQTOKEN…`、消息正文 `ZXQ-PUSH-BODY`、昵称 `ZXQ-NICK`
- 步骤：
  1. 注册、推送（隐藏和详情两种）、通道返回 410 / `UNREGISTERED` 删令牌、注销、经 console 强制下线
  2. 用 `deploy/ops/scan-logs` 扫 im-server、console-server 日志和两边审计
- 预期结果：
  1. 都搜不到（令牌只有 provider 和哈希前 8 位）

### TC-CHAT-90-22 通知设置页符合 B-1 和 B-3
- 关联 AC：AC-CHAT-90-8
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器 + 替身
- 前置条件：第 15 节累计表：C3 增加"通知设置"页面。H5 没有这个页面（第 13.2 节）
- 步骤：
  1. 扫描通知设置页的文案、图标和构建产物
- 预期结果：
  1. 符合 PRD-ui B-1（AC-UI-90-15）和 B-3（AC-UI-11-3）；新增图标都在清单里

### TC-CHAT-90-23 通知设置页：无障碍、深色模式
- 关联 AC：AC-CHAT-90-9
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器 + 替身
- 前置条件：第 13.4 节的 5 种状态：系统授权关闭、令牌获取中、令牌获取失败、设备不支持、保存失败
- 步骤：
  1. 每种状态：无障碍检查（Android Accessibility Test Framework、iOS `performAccessibilityAudit`）
  2. 深色模式截图
- 预期结果：
  1. 开关、"前往系统设置"有可读名称；没有违规
  2. 新增颜色都来自 token

### TC-CHAT-90-24 新帧向量（C3）：服务端和三套 SDK 都跑
- 关联 AC：AC-CHAT-90-10
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：单测（四方）
- 环境：本地（服务端、sdk-ts、sdk-kotlin、sdk-swift）
- 前置条件：第 15 节累计表：C3 增加"通知命令、`presence`"；`frames.json` 已有 BE-450 的用例
- 步骤：
  1. 服务端、三套 SDK 跑 `protocol/vectors/frames.json`；sdk-ts 做 Schema 校验
- 预期结果：
  1. 全部通过；sdk-ts 没有设备通知接口，只校验帧

### TC-CHAT-90-25 指标和配置检查（C3）
- 关联 AC：AC-CHAT-90-17
- 优先级：P0
- 批次：C3（【全部】，C3 增加的部分）
- 层级：后端 e2e
- 环境：E1 + 假通道 + `/metrics`
- 前置条件：C3：推送指标和配置检查项（第 14.2 节）
- 步骤：
  1. 走成功推送、重试用尽、令牌失效、队列满
  2. 抓 `/metrics`、`GET /internal/v1/config`
- 预期结果：
  1. 出现 `im_offline_push_total{provider,result}`、`im_offline_push_dropped_total`、`im_offline_push_queue_len`
  2. 配置检查有 APNs / FCM 启用和密钥状态
  3. 标签没有 userId、令牌、`convId`

### TC-CHAT-90-26 老版本三套 SDK 收到撤回：不崩溃，显示已删除，搜不到原文
- 关联 AC：AC-CHAT-90-7
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套，老版本）
- 环境：本地：sdk-ts@f417580、sdk-kotlin@2819ce6、sdk-swift@c8ec6ff（C4 之前的提交）+ 新的 `frames.json`
- 前置条件：老版本 SDK 已加载含特征串的消息
- 步骤：
  1. 喂 `image`、`file` 消息，带 `recalled` 的 `msgDeleted`，带 `deleted`+`recalled` 的 `sync`，带 `quote`、`mentions` 的消息，`mentionUnread` 等新字段
  2. 导出内存；原生读本地库文件
- 预期结果：
  1. 不崩溃，不认识的字段忽略
  2. 图片、文件显示"[暂不支持的消息]"；撤回的消息显示"该消息已被删除"
  3. 内存和本地库搜不到原文特征串

### TC-CHAT-90-27 ack P99：带引用的单聊、带 @ 的群聊
- 关联 AC：AC-CHAT-90-1
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：性能
- 环境：EP 性能（P-12）；开发机只记录
- 前置条件：第 15 节累计表 C4：带引用的单聊、带 @ 的群聊；500 条/秒，≥ 1000 条；生产配置，`IM_MSG_ACTIONS_ENABLED=1`
- 步骤：
  1. 分别压测两种消息，各跑 3 次
- 预期结果：
  1. 3 次取最差，ack P99 ≤ 200 ms（未校准），错误率 0；增幅只记录
  2. 开发机结果不算通过

### TC-CHAT-90-28 开关全关（C4：IM_MSG_ACTIONS_ENABLED 关）
- 关联 AC：AC-CHAT-90-2
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：后端 e2e + 性能
- 环境：E1；TC-PERF 在 EP（P-12）
- 前置条件：`IM_MSG_ACTIONS_ENABLED=0`，其他新开关也关
- 步骤：
  1. 跑现有 `server/e2e` 全部用例（不改代码）
  2. 跑 TC-PERF
- 预期结果：
  1. e2e 全过
  2. TC-PERF 判定不变（开发机只记录）

### TC-CHAT-90-29 日志和审计搜不到特征串（C4：撤回、引用、@）
- 关联 AC：AC-CHAT-90-5
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：后端 e2e + 部署 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：消息正文、文件名、引用文字都带特征串
- 步骤：
  1. 走撤回（单聊、群、管理员撤回、隔离读取、`fetch`）、引用、@、@ 所有人
  2. 扫描 im-server 和 console-server 的日志、两边的审计表（`scan-logs`）
- 预期结果：
  1. 都搜不到特征串
  2. 审计里只有 `msgId`、`fileId` 等标识

### TC-CHAT-90-30 菜单和引用条符合 B-1 和 B-3，图标在清单
- 关联 AC：AC-CHAT-90-8
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器
- 前置条件：C4 新增界面：消息菜单（复制、引用、撤回）、撤回提示行、引用条四种状态、@ 选择、"有人@我"按钮
- 步骤：
  1. 扫描文案和构建产物（PRD-ui B-1、B-3 的检查脚本）
  2. 核对新增图标
- 预期结果：
  1. 符合 B-1、B-3
  2. 新增图标都在 PRD-ui 6.6 节清单里

### TC-CHAT-90-31 C4 页面：axe-core、深色模式、键盘打开消息菜单
- 关联 AC：AC-CHAT-90-9
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器
- 前置条件：同 TC-CHAT-90-30 的页面
- 步骤：
  1. H5 跑 axe-core（浅色、深色）；原生跑等价检查（Android 语义树、iOS 系统审计）
  2. H5 只用键盘：Tab 到气泡，菜单键或 Shift+F10 打开菜单，方向键、回车、Esc
- 预期结果：
  1. 没有 `button-name` 违规；新增颜色都来自 token
  2. 键盘能打开菜单并执行"引用"
  3. 新消息播报按 P-39 ① 不在本批（frontend-chat 第 14 条），记已知缺口

### TC-CHAT-90-32 新帧向量（C4）：服务端和三套 SDK 都跑
- 关联 AC：AC-CHAT-90-10
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：单测（四方）
- 环境：本地（服务端、sdk-ts、sdk-kotlin、sdk-swift）
- 前置条件：第 15 节累计表 C4：`recall`、`quote`、`mentions`、`msgDeleted.recalled`；`frames.json` 已有 BE-470 的用例
- 步骤：
  1. 服务端、三套 SDK 跑 `protocol/vectors/frames.json`；sdk-ts 做 Schema 校验
- 预期结果：
  1. 全部通过

### TC-CHAT-90-33 共享用例（C4：撤回、引用、提及）
- 关联 AC：AC-CHAT-90-15
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：SDK 单测（三套）
- 环境：本地（三套 SDK）
- 前置条件：`chat-sync.json` 已有 BE-478 的用例
- 步骤：
  1. 三套 SDK 跑 `chat-sync.json`、`account-session.json`
- 预期结果：
  1. 全部通过，含撤回合并、引用清除、提及计数

### TC-CHAT-90-34 指标和配置检查（C4）
- 关联 AC：AC-CHAT-90-17
- 优先级：P0
- 批次：C4（【全部】，C4 增加的部分）
- 层级：后端 e2e
- 环境：E1 + `/metrics`
- 前置条件：C4：撤回指标、配置检查项 `msg_actions`
- 步骤：
  1. 发送者、群主、管理员各撤回一次
  2. 抓 `/metrics`、`GET /internal/v1/config`
- 预期结果：
  1. `im_recalls_total{by}` 三个取值都增加
  2. 配置检查有开关状态、隔离密钥已设置、隐私说明版本，不含密钥
  3. 标签没有 userId、`convId`、`msgId`
