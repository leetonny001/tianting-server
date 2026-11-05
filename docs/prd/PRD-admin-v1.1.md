# PRD：管理后台 H5（业务运营）与 console-server 公共基座

| 项 | 内容 |
|---|---|
| 版本 | **v1.1（已冻结，2026-11-05；变更见第 16 节）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-03 |
| 状态 | **已冻结（2026-11-03）**：由 v0.2 经评审、复审、一致性复审和冻结前核对后冻结，与 [PRD-core v1.3](PRD-core-v1.3.md) 同时冻结，是 PRD-core 的组成部分。修改需走变更流程（见 [docs/process](../process/README.md)）。评审与复审记录见 docs/review/ |
| 取代 | [PRD-admin-v0.1](PRD-admin-v0.1.md) |
| 产品名 | 天听（DECISIONS U-1） |
| 依据（已拍板） | X-1 ②（独立 `console-server`）、X-2 ②（共用登录服务、运维账号独立空间、自建账号 + 强制 TOTP、预留 SSO）、X-3 ②（Postgres）、X-4 ②（审计 P0、哈希链、写失败则操作失败、在线 1 年 + 加密归档 3 年）、X-5 ②（管理接口独立内网端口、分密钥、限定权限、请求签名）、X-6 ②（分两批；因 S-3，第一批含社交管理与解散群）、X-7（法务确认前单聊正文不可查看；群消息按 ②）、X-8 ②、X-9 ②；S-3、S-9、S-10、S-11 ②；2.5 节（Android Kotlin / iOS Swift，不用 KMP） |
| 背景材料 | [PRD-core v1.2](PRD-core-v1.2.md)（冻结）、[PRD-social v0.2](PRD-social-v0.2.md)、[PRD-ops v0.2](PRD-ops-v0.2.md) 与 [REVIEW-ops](../review/REVIEW-ops-v0.1.md)、[一致性复审](../review/RECHECK-v0.2-2026-11-03.md)、[ARCH-v1](../arch/ARCH-v1.md)、[ADMIN.md](../../protocol/ADMIN.md)、[ERRORS.md](../../protocol/ERRORS.md)、[errors.json](../../protocol/errors.json)、`server/internal/admin`、`server/internal/app/app.go` |
| 被引用 | **第 4 节"console-server 公共基座"是管理后台和运维后台的唯一规格**，PRD-ops v0.2 引用它，只写运维空间的角色和差异 |
| 修订记录 | 第 17 节（每条评审意见 → 已改 / 部分 / 不采纳 + 理由） |

---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 后台服务形态（X-1） | 新建独立服务 **`console-server`**（独立仓库，待建，Go），部署在内网 / VPN 后，**同时服务管理后台和运维后台**：两个账号空间、两个监听地址、两个域名，Cookie 天然按域名隔离。浏览器只和 console-server 通信，**永远拿不到 IM 管理密钥**；console-server 以"一个限定权限的调用方"身份调用 im-server |
| 前置服务端改造（X-5） | 第 3 节 F-KEY-01～05：`/admin/*` 全部移到独立内网监听地址 `IM_ADMIN_ADDR`；按调用方分密钥、限定 scope；请求签名（HMAC-SHA256 或 Ed25519）+ 时间戳窗口 + nonce 防重放；im-server 侧管理接口审计；旧 `X-Admin-Key` 只保留 v1.2 已有接口，生产默认关闭。**console-server 主凭证不含 `token.issue`、不含任何读正文的 scope** |
| 账号体系（X-2） | 共用登录服务；`admin` / `ops` 两个**账号空间**，一个账号只属于一个空间；运维账号只能由运维超管创建、重置；自建账号 + 强制 TOTP，账号表预留 `idp_subject`（SSO 为 P1） |
| 存储（X-3） | console-server 用 **Postgres**：`auth`、`audit`、`biz` 三个 schema；运行账号对审计表只有 INSERT + SELECT（只追加由数据库权限保证）；会话、失败计数都在库里，多实例一致 |
| 审计（X-4） | P0。两阶段（操作前写 intent，失败则不执行，返回 5031；操作后写 result）、全局哈希链（链头行锁串行化）、每小时把链头锚定到外部只追加存储、校验命令；在线 1 年 + 加密归档 3 年 |
| "需要重新验证" | **HTTP 403 / 4038**（不是 401）。前端只在 **4014** 时跳登录页；4038 弹 TOTP 框、通过后自动重放原请求；正文授权缺失或过期统一 **403 / 4039** |
| 分批（X-6、S-3） | **第一批**：X-5 前置、安全基座、用户（搜索、详情、在线设备、导入、强制下线）、社交管理（好友导入 / 删除 / 查询、好友模式配置、官方账号标记）、群（列表、建群、成员、设置、**解散**、群主交接）、消息元数据查询。**第二批**（PRD-core v1.3 变更，和协议、三套 SDK、PRD-ui 一起）：封禁、群消息正文查看、删除消息 + 原文隔离、用户 / 群成员禁言，**系统通知最后**（阻塞于二期 push 协议） |
| 正文查看（X-7） | 单聊正文：im-server 侧开关 `IM_C2C_BODY_ACCESS` 默认关闭，法务书面确认前任何凭证都读不到（403 / 4030）。群消息正文（第二批）：`moderator` + 工单号与理由 + 每次重新输入 TOTP + 审计 + 水印；走 im-server 侧的"查看授权" + 配额，使用单独的 `console-msgread` 凭证（第 6.4 节 F-ADM-08） |
| 封禁（X-8）/ 删除原文（X-9） | IM 和业务方都可以封禁；封禁期间签发 token 返回 **403 / 4034**。删除消息后原文加密隔离 30 天、只有 `moderator` 可按授权查看，到期物理删除 |
| 与 PRD-social 对齐 | 社交管理接口（S-11）在 X-5 之后上线且有审计；console 主凭证持有 `social.friends`、`social.settings`；解散群与 `groupDissolve` 共用 `DissolveGroup`；群列表 / 详情显示群主、类型、管理员；第二批禁言规则按"M4 之后有群管理员"修订（第 9.5 节交接点逐条落实） |
| 新错误码 | 14 个：4014、4015、4016、4017、4034、4037、4038、4039、4044、4092、4093、4291、5031、5040（第 10 节），与 errors.json、PRD-social v0.2 已占用的码逐个核对无冲突；新增 `scope: "console"`，SDK 生成器跳过 |
| 规模 | 42 个功能点（第一批 32 = 前置 5 + 基座 12 + 业务 15；第二批 7；P1 / P2 3），AC 共 278 条（第一批 214，含 4 条性能 AC；第二批 58；P1 / P2 6） |
| 工作量 | 约 **127 人日**：服务端 34、console-server 36.5、前端 26.5、SDK 与客户端 11、测试 19（第 14 节）；第一批约 75.5，第二批约 51.5 |
| 新增待拍板 | **无**。1 个确认项（第 15 节 Q-1：X-5"不能读正文"按"主凭证不能读、正文走单独凭证 + im-server 侧授权和配额"理解，第二批开工前确认，不阻塞第一批） |

---

## 1. 背景与目标

### 1.1 背景

一期的管理能力只有给业务服务端用的 `/admin/*`（静态密钥 `X-Admin-Key`），而且和 `/ws` 挂在**同一个对外端口**上（`app.go` 中 `admin.Register(mux)` 与 `GET /ws` 共用一个 mux）。运营、客服没有界面，替换腾讯云 IM（fx-sport / ab77 / android）需要覆盖腾讯 IM 控制台和 REST API 的常用能力：查用户、踢下线、封禁、群管理、禁言、删消息、系统消息；社交关系上线后（PRD-social）还要能导入好友、切换好友模式、标记官方账号。

后台操作的对象是全体用户的聊天数据和关系数据，**一次越权或一个泄露的会话，影响的是全体用户**。所以：
1. 浏览器只能拿到后台自己的会话，拿不到任何 IM 凭证；
2. 后台服务本身被攻破时，损失也要有边界（不能冒充终端用户、不能批量读正文）；
3. 每一次写操作、每一次敏感查看都要能追溯到人，且记录不能被事后删改。

### 1.2 目标

1. 给运营 / 客服 / 审核一个桌面浏览器后台，替代"找开发调接口"。
2. 建一个两个后台共用的安全基座（console-server）：独立账号空间 + 强制 TOTP、可叠加角色、不可篡改的审计、敏感数据默认不可见。
3. 先收紧 im-server 的管理接口（X-5），让任何调用方（包括后台、业务方、旧密钥）都只能做自己被授权的事。

### 1.3 成功标准

| 指标 | 目标 | 度量方法 |
|---|---|---|
| 密钥隔离 | 浏览器侧（构建产物、HAR、Cookie、Storage）找不到任何 IM 凭证或 console 签名私钥 | AC-CON-10-1 自动化 + 上线前渗透测试（ARCH S15） |
| 管控不可绕过 | 对外端口上没有 `/admin/*`；任何凭证调用超出 scope 的接口都被拒绝 | AC-KEY-01-1、AC-KEY-02-* 参数化用例 |
| 可追溯 | 100% 写操作、100% 正文 / 在线设备 / 单聊元数据查看在**操作前**有审计 intent 记录 | AC-CON-08-* 参数化用例（每类动作一条） |
| 防篡改 | 删除中间记录、截断尾部（早于最近一次锚定）、整链重算都能被校验命令发现 | AC-CON-08-5～8 |
| 效率 | 客服"查用户 → 填理由看在线设备 → 强制下线"全流程 ≤ 45 秒 | 手工走查（3 人各 3 次取中位数） |
| 性能 | 第 9 节的 P95 目标；Postgres 环境验证，SQLite 环境用缩小规模的目标 | TC-PERF-ADM（第 9 节） |

---

## 2. 范围与分批

### 2.1 功能点总览

| 编号 | 功能点 | 批次 | 优先级 | 涉及 |
|---|---|---|---|---|
| F-KEY-01 | 管理接口迁到独立内网监听地址 | 第一批（前置） | P0 | im-server |
| F-KEY-02 | 调用方密钥与权限范围（scope） | 第一批（前置） | P0 | im-server |
| F-KEY-03 | 请求签名与防重放 | 第一批（前置） | P0 | im-server / console-server / 接入文档 |
| F-KEY-04 | im-server 侧管理接口审计 | 第一批（前置） | P0 | im-server |
| F-KEY-05 | 旧 `X-Admin-Key` 的过渡规则 | 第一批（前置） | P0 | im-server |
| F-CON-01 | console-server 部署形态、网络边界与启动保护 | 第一批 | P0 | console-server / 部署 |
| F-CON-02 | 账号空间与账号生命周期（引导、创建、激活、停用、重置） | 第一批 | P0 | console-server / 前端 |
| F-CON-03 | 登录（密码 + TOTP 一步提交、恢复码） | 第一批 | P0 | console-server / 前端 |
| F-CON-04 | 密码策略、锁定与限流 | 第一批 | P0 | console-server |
| F-CON-05 | 会话（Cookie、过期、后台请求不续期、CSRF、并发上限） | 第一批 | P0 | console-server / 前端 |
| F-CON-06 | 角色与授权机制（可叠加角色、白名单、职责分离、应急授权） | 第一批 | P0 | console-server / 前端 |
| F-CON-07 | 重新验证（step-up） | 第一批 | P0 | console-server / 前端 |
| F-CON-08 | 审计记录（两阶段、哈希链、锚定、写失败策略） | 第一批 | P0 | console-server |
| F-CON-09 | 审计查看、对账、保留与归档 | 第一批 | P0 | console-server / 前端 |
| F-CON-10 | 浏览器侧安全（密钥隔离、XSS、响应头） | 第一批 | P0 | console-server / 前端 |
| F-CON-11 | 密钥材料与密码哈希参数 | 第一批 | P0 | console-server |
| F-CON-12 | 安全通知（站内） | 第一批 | P0 | console-server / 前端 |
| F-ADM-05 | 管理后台角色与权限矩阵 | 第一批 | P0 | console-server / 前端 |
| F-ADM-10 | 用户搜索与详情 | 第一批 | P0 | im-server / console-server / 前端 |
| F-ADM-11 | 用户导入 | 第一批 | P0 | console-server / 前端 |
| F-ADM-12 | 强制下线 | 第一批 | P0 | console-server / 前端 |
| F-ADM-14 | 在线设备查看（填理由、审计） | 第一批 | P0 | im-server / console-server / 前端 |
| F-ADM-15 | 好友关系管理（导入、删除、查询） | 第一批 | P0 | console-server / 前端（接口见 PRD-social F-SOC-12） |
| F-ADM-16 | 好友模式与社交配置 | 第一批 | P0 | console-server / 前端（接口见 F-SOC-01） |
| F-ADM-17 | 官方账号标记 | 第一批 | P0 | console-server / 前端（接口见 F-SOC-13） |
| F-ADM-20 | 群列表与搜索 | 第一批 | P0 | im-server / console-server / 前端 |
| F-ADM-21 | 建群 | 第一批 | P0 | console-server / 前端 |
| F-ADM-22 | 群详情与成员管理 | 第一批 | P0 | im-server / console-server / 前端 |
| F-ADM-23 | 群设置 | 第一批 | P0 | console-server / 前端 |
| F-ADM-24 | 解散群 | 第一批 | P0 | im-server / console-server / 前端 / 三套 SDK |
| F-ADM-25 | 指定 / 收回群主 | 第一批（随 M4 交付） | P0 | console-server / 前端（接口见 F-SOC-30） |
| F-ADM-30 | 会话消息元数据查询 | 第一批 | P0 | im-server / console-server / 前端 |
| F-ADM-08 | 群消息正文查看（单聊开关默认关闭） | 第二批 | P0 | im-server / console-server / 前端 |
| F-ADM-13 | 封禁 / 解封 | 第二批 | P0 | im-server / console-server / 前端 / SDK |
| F-ADM-31 | 删除违规消息 | 第二批 | P0 | im-server / console-server / 前端 / 三套 SDK |
| F-ADM-32 | 被删原文的加密隔离与取证查看 | 第二批 | P0 | im-server / console-server / 前端 |
| F-ADM-40 | 用户级禁言 | 第二批 | P0 | im-server / console-server / 前端 / 三套 SDK |
| F-ADM-41 | 群成员禁言 | 第二批 | P0 | im-server / console-server / 前端 / 三套 SDK |
| F-ADM-50 | 系统通知（按用户 / 全员） | 第二批（最后，阻塞于二期 push 协议） | P0 | im-server / console-server / 前端 / 三套 SDK |
| F-ADM-42 | 全群禁言 | 两批之外 | P1 | im-server / SDK |
| F-ADM-70 | 数据概览 | 两批之外 | P1 | im-server / console-server / 前端 |
| F-ADM-60 | 敏感词与内容审核 | 两批之外 | P2 | im-server |

共 42 个功能点：第一批 32 个（前置 5、基座 12、业务 15），第二批 7 个，P1 / P2 3 个。v0.1 的 F-ADM-01～04、06、07、09 合并进第 4 节公共基座（对应关系见第 17.3 节）。

### 2.2 分批与实现顺序

```
X-5（F-KEY-01～05，im-server，约 4.5 人日）
  └─► console-server 基座（F-CON-01～12）  ← 需要 Postgres（X-3：本机安装另行授权）
        └─► 第一批业务（F-ADM-05、10～12、14～17、20～25、30）
              │   im-server 侧：搜索 / 分页 / 在线设备查询 / 群列表 / 解散 / 元数据查询（第 8 节 G-ADM-xx）
              │   社交管理接口由 PRD-social M3 实现；F-ADM-25 随 M4
              └─► PRD-core v1.3 第二批条目定稿（含协议 Schema 兼容规则，第 11 节）
                    └─► im-server + sdk-ts 同步实现（sdk-kotlin / sdk-swift 在 M5 / M6 首版带上）
                          └─► PRD-ui v0.2 补客户端提示 → 第二批（F-ADM-13、08、31、32、40、41）
                                └─► 二期 push 协议定稿 → F-ADM-50（第二批最后）
```

规则：
1. **第一批也改客户端协议**（S-3 之后"第一批不改协议"的前提不成立）：本 PRD 第一批只改一处——`convRemoved` 增加 `reason`（F-ADM-24），并要求三套 SDK"忽略不认识的事件类型、字段和枚举值"（与 PRD-social V-18 同一条）。
2. 社交管理接口（F-ADM-15～17、25 调用的 im-server 接口）**在 F-KEY 上线之后**才在生产注册，并写审计（S-11 ②，PRD-social F-SOC-12 规则 8、9）。
3. 第二批的每个功能点都是 PRD-core v1.3 的变更（第 11.2 节），必须与协议、sdk-ts、PRD-ui 一起交付；不允许"服务端先上、客户端没有提示"。
4. F-ADM-50 依赖的二期 push 协议（离线补收、`pushId` 去重、多端语义）不在本 PRD 定义；协议评审通过前，F-ADM-50 的 AC 只作为验收草案（第 6.7 节）。

### 2.3 通用约定

| 项 | 约定 |
|---|---|
| 接口前缀 | 浏览器 → console-server：`/console/api/*`（同源，两个空间的域名各自一套）；console-server → im-server：`/admin/*`（内网监听地址，签名） |
| 列表 | 一律游标分页（`cursor` + `limit`），`limit` 取 20 / 50 / 100，默认 20；不提供总页数 |
| 搜索 | userId / groupId：精确或前缀匹配，**区分大小写**（C-18）；昵称 / 群名：前缀匹配，ASCII 字母**不区分大小写**，其他字符按码点精确比较；关键字去首尾空白后 1～64 字符；`%`、`_`、`\` 在服务端转义后再做 `LIKE`（`_` 是合法 userId 字符） |
| 敏感查询的参数 | 带理由的查询（在线设备、单聊元数据、正文）一律 `POST` + JSON 请求体，userId、会话 ID、理由**不出现在 URL** 里（不进浏览器历史和访问日志） |
| 时间 | 服务端毫秒时间戳；页面按浏览器时区显示到秒 |
| 理由（`reason`） | 去首尾空白后 4～200 个字符（按 Unicode 码点计数），且不能由同一个字符重复组成（两个后台统一，替代 v0.1 的 2～200）；写入审计。运维后台排查接口的说明字段同样叫 `reason`、规则相同 |
| 每日上限 | "每天"按 Asia/Shanghai 自然日计算；超出返回 429 / 4292（不可自动重试，两个后台统一） |
| 工单号（`ticket`） | 格式由 `CON_TICKET_PATTERN` 配置（默认 `^[A-Za-z0-9_-]{3,32}$`）；正文查看必填，其他操作选填 |
| 错误 | 响应体 `{code, msg, reason?, details?}`；HTTP 状态取 errors.json 的 `http`；前端按 `code`（必要时加 `reason`）显示文案，不直接展示 `msg`。console-server 把 im-server 返回的错误码**原样透传**（4042、4043、4090 等） |
| 前端对错误码的处理 | 4014 → 登录页；4016 → 首次设置页；4038 → TOTP 弹框，通过后自动重放原请求一次；4039 → "填工单号与理由 + TOTP"弹框；4037 → 重新 `GET /console/api/session` 取令牌后重放一次，仍失败则提示刷新；5031 → "审计服务不可用，操作未执行"；5040 → "操作结果未知，请刷新确认，不要重复提交"（系统通知例外："结果未知，系统会自动确认，请稍后查看发送记录"，F-ADM-50）；其他按码显示文案 |

### 2.4 测试设施与时钟

| 项 | 约定 |
|---|---|
| 两个时钟 | **[C 钟]**：console-server 的注入时钟（`CON_TEST_MODE=1` 时注册 `POST /console/test/clock {nowMs}`），驱动会话过期、锁定、激活码、正文授权、step-up 窗口。**[IM 钟]**：im-server 的 `POST /admin/test/clock`，驱动封禁 / 禁言到期、隔离原文到期、im-server 侧授权到期、签名时间戳窗口。每条与时间有关的 AC 都标明用哪个 |
| 同时设置 | 测试仓库提供 `tools/set-clocks.sh <nowMs>`：先设 [IM 钟]，再设 [C 钟] |
| 状态以 im-server 为准 | console-server 显示封禁、禁言、授权状态时**只用 im-server 返回的时间和状态**，不用自己的时钟计算 |
| 故障注入 | `CON_TEST_MODE=1` 时注册 `POST /console/test/faults {auditWrite: "fail"|"failResult"|"ok", imLatencyMs}`，用于审计写失败、结果未知类 AC |
| 生产保护 | `CON_ENV=production` 时开启 `CON_TEST_MODE` 拒绝启动（AC-CON-01-4） |
| 性能环境 | 性能 AC 在 Postgres 环境（im-server B6 之后）验证；B6 之前在 SQLite 上用缩小规模的目标（第 9 节） |
| 双节点 | 涉及失效通知的 AC 使用 PRD-social 5.6 节的双节点测试环境 |

---

## 3. 前置服务端改造：管理接口收紧（X-5）

> 这是**本 PRD 的前置条件**，也是 PRD-social 社交管理接口（S-11）的前置条件。改造只在 im-server 仓库，约 4.5 人日（第 14 节）。完成后 ARCH S8 从 P1 变为已实现（第 11.4 节）。

### F-KEY-01 管理接口迁到独立内网监听地址（第一批前置，P0）

**规则**
1. 新增配置 `IM_ADMIN_ADDR`（默认 `127.0.0.1:8081`，`off` 表示不开）。`/admin/*` 全部路由（包括测试接口 `/admin/test/*`）**只注册在这个地址上**；对外端口（`IM_ADDR`）只保留 `/ws`、`/healthz`、`/readyz`，以及测试 / 演示环境的 `/demo/login`。
2. 生产环境（`IM_ENV=production`）下 `IM_ADMIN_ADDR` 必须启用 TLS（`IM_ADMIN_TLS_CERT` / `IM_ADMIN_TLS_KEY`）；只有绑定回环地址（`127.0.0.1` / `::1`）时允许明文（同机部署的业务服务端）。不满足时拒绝启动。
3. 可选 `IM_ADMIN_ALLOWED_CIDRS`：来源 IP 不在列表内直接关闭连接前返回 403 / 4030；只看 TCP 对端地址，**不解析 `X-Forwarded-For`**（管理端口前面不放反向代理）。
4. `IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的地址两两不能相同（指标端口免鉴权；运维端口见 PRD-ops F-OPS-16）；相同时拒绝启动。
5. 集群下每个节点都开管理地址，业务方经内网负载均衡访问；签名的 nonce 去重需要共享存储（F-KEY-03 规则 5）。

**验收标准**
- AC-KEY-01-1【后端】：输入 默认配置启动；操作 在对外端口上请求 `POST /admin/users/token`（带正确密钥）和 `GET /admin/users/u1`；预期 HTTP 404（路由不存在），同样的请求发到 `IM_ADMIN_ADDR` 正常处理；错误码 —（404 为未注册路由）。
- AC-KEY-01-2【后端】：输入 `IM_ENV=production`、`IM_ADMIN_ADDR=10.0.0.5:8081`、未配置 TLS 证书；操作 启动 im-server；预期 进程以非 0 退出，日志指出缺少管理端口 TLS；错误码 —。
- AC-KEY-01-3【后端】：输入 `IM_ENV=production`、`IM_ADMIN_ADDR=127.0.0.1:8081`、无 TLS；操作 启动；预期 启动成功（回环例外）；错误码 —。
- AC-KEY-01-4【后端】：输入 `IM_ADMIN_ALLOWED_CIDRS=10.0.0.0/8`；操作 从 `192.168.1.10` 发正确签名的请求，并带 `X-Forwarded-For: 10.0.0.1`；预期 HTTP 403，业务未执行；错误码 4030。
- AC-KEY-01-5【后端】：输入 两组配置分别启动：① `IM_ADMIN_ADDR` 与 `IM_METRICS_ADDR` 都设为 `127.0.0.1:9090`；② `IM_ADMIN_ADDR` 与 `IM_OPS_ADDR` 都设为 `127.0.0.1:8081`；操作 启动；预期 两组都非 0 退出，日志指出冲突的两个地址；错误码 —。
- AC-KEY-01-6【后端】：输入 `IM_TEST_MODE=1`；操作 在对外端口请求 `POST /admin/test/clock`；预期 404；在管理地址上请求成功；错误码 —。

### F-KEY-02 调用方密钥与权限范围（第一批前置，P0）

**规则**
1. 调用方配置文件 `IM_ADMIN_CALLERS`（JSON 文件路径，权限 0600），每个调用方：`{id, alg: "hmac-sha256" | "ed25519", secret | publicKey, scopes: [...], allowedCIDRs?: [...], pushTypes?: [...], disabled?: bool}`（`pushTypes`：只允许这些 `type`；`IM_PUSH_ENABLED` 开启时，持有 `push.all` 的调用方必须配非空的 `pushTypes`，console 调用方也一样，[PRD-live](PRD-live-v1.0.md) F-PUSH-09 规则 3）。`id` 为 `[a-z0-9-]{3,32}`；HMAC 密钥 ≥ 32 字节随机值；不同调用方的密钥不能相同，也不能与 JWT 密钥、旧管理密钥相同（启动检查）。
2. **scope 目录**（每个管理接口在代码里声明所需 scope，未声明的接口启动失败）：

   | scope | 接口 | 批次 |
   |---|---|---|
   | `token.issue` | `POST /admin/users/token` | 已有 |
   | `user.read` | `GET /admin/users/{id}`、`GET /admin/users?q=` | 已有 / 第一批 |
   | `user.write` | `POST /admin/users/import` | 已有 |
   | `user.kick` | `POST /admin/users/{id}/force-logout` | 已有 |
   | `user.sessions` | `POST /admin/users/sessions/query`（在线设备，与运维后台共用同一查询函数） | 第一批 |
   | `user.ban` | `POST /admin/users/{id}/ban`、`/unban` | 第二批 |
   | `group.read` | `GET /admin/groups/{id}`、`GET /admin/groups?q=`、`GET /admin/groups/{id}/members` | 已有 / 第一批 |
   | `group.write` | `POST /admin/groups`、`PATCH /admin/groups/{id}`（不含 `owner`）、`POST` / `DELETE /admin/groups/{id}/members` | 已有 |
   | `group.dissolve` | `DELETE /admin/groups/{id}` | 第一批 |
   | `social.settings` | `GET` / `PATCH /admin/settings/social`、`PUT /admin/users/{id}/official`、`owner` 字段（PRD-social 9.3 节） | 第一批 |
   | `social.friends` | `POST /admin/friends/import`、`DELETE /admin/friends`、`GET /admin/users/{id}/friends` | 第一批 |
   | `msg.meta` | `POST /admin/convs/messages/query`（只返回元数据） | 第一批 |
   | `msg.body.group` / `msg.body.c2c` | `POST /admin/msg-grants`（按会话类型）、`POST /admin/convs/messages/body` | 第二批 |
   | `msg.quarantine` | 隔离原文的查看授权（F-ADM-32） | 第二批 |
   | `msg.delete` | `POST /admin/messages/delete` | 第二批 |
   | `mute.write` | `POST /admin/mute`、`/admin/unmute` | 第二批 |
   | `push.user` / `push.all` / `push.room` | `POST /admin/push`（按 `target`；`push.room` 随 L1 进目录，房间开关关闭时 `target=room` 返回 400 / 4000，PRD-live AC-PUSH-02-8） | 第二批（最后） |
   | `audit.read` | `GET /admin/audit`（im-server 侧审计，F-KEY-04） | 第一批 |
   | `test` | `/admin/test/*`（只在 `IM_TEST_MODE=1` 注册） | 已有 |

3. **正文类 scope 的额外限制**：`msg.body.*`、`msg.quarantine` 只能配置在 `bodyAccess: true` 的调用方上，且全部调用方中最多 1 个；该调用方**不能同时有其他任何 scope**。`msg.body.c2c` 另外受 `IM_C2C_BODY_ACCESS`（默认 `off`）控制：为 `off` 时，即使有 scope 也返回 403 / 4030（`reason: "c2cBodyDisabled"`）。打开它需要改 im-server 配置并重启，**不能通过任何管理接口打开**（X-7）。
4. console-server 使用两个调用方：**`console`**（主凭证，Ed25519）：`user.read`、`user.write`、`user.kick`、`user.sessions`、`group.read`、`group.write`、`group.dissolve`、`social.settings`、`social.friends`、`msg.meta`、`audit.read`；第二批追加 `user.ban`、`msg.delete`、`mute.write`、`push.user`、`push.all`。**永远不含 `token.issue`、`msg.body.*`、`msg.quarantine`、`test`。** **`console-msgread`**（第二批才配置，Ed25519，`bodyAccess: true`）：只有 `msg.body.group`、`msg.quarantine`（法务确认后才加 `msg.body.c2c`）。
5. 业务方调用方典型配置：`token.issue`、`user.read`、`user.write`、`user.kick`、`group.*`、`social.*`，第二批可加 `user.ban`、`msg.delete`、`mute.write`、`push.user`。业务方接入时可授予 `push.all`，三项必选：独立调用方（只有 `push.all`、配 `AllowedCIDRs`）、`pushTypes` 白名单、按 `type` 审计（L-17，Frank 2026-11-05 拍板）。
6. scope 不足返回 403 / 4030；调用方 `disabled` 或不存在返回 401 / 4017。配置文件变更需重启（一期不做热加载）。

**验收标准**
- AC-KEY-02-1【后端】：输入 调用方 `biz1` 只有 `token.issue`；操作 用 `biz1` 正确签名调用 `POST /admin/groups`；预期 HTTP 403，群未创建；错误码 4030。
- AC-KEY-02-2【后端】：输入 F-KEY-02 规则 2 的 scope 表；操作 参数化测试遍历全部管理路由 × "有该 scope / 没有该 scope"两种调用方，每个路由准备一个合法请求样本（`push.room` 的样本在房间开关关闭时按 PRD-live AC-PUSH-02-8 判定）；预期 有 scope 时返回该接口的成功状态码，没有时 403 / 4030；错误码 4030。
- AC-KEY-02-3【后端】：输入 在代码里新增一个没有声明 scope 的 `/admin/*` 路由；操作 运行 `go test ./...` 和启动服务；预期 启动阶段 panic / 测试失败（"route without scope"）；错误码 —。
- AC-KEY-02-4【后端】：输入 调用方配置中 `console` 同时带 `msg.meta` 和 `msg.body.group`；操作 启动；预期 非 0 退出，日志指出正文类 scope 只能在独立的 `bodyAccess` 调用方上；错误码 —。
- AC-KEY-02-5【后端】：输入 两个调用方使用相同 HMAC 密钥，或某调用方密钥等于 `IM_JWT_SECRET`；操作 启动；预期 非 0 退出；错误码 —。
- AC-KEY-02-6【后端】：输入 `console` 调用方按规则 4 配置；操作 用它调用 `POST /admin/users/token`；预期 403；错误码 4030。
- AC-KEY-02-7【后端】：输入 `IM_C2C_BODY_ACCESS=off`，`console-msgread` 带 `msg.body.c2c`；操作 为 `c2c:a:b` 申请查看授权；预期 403，`reason=c2cBodyDisabled`，未产生授权；错误码 4030。
- AC-KEY-02-8【后端】：输入 调用方 `biz1` 设置 `disabled: true`；操作 正确签名调用任意接口；预期 401；错误码 4017。

### F-KEY-03 请求签名与防重放（第一批前置，P0）

**规则**
1. 请求头：`X-IM-Caller`（调用方 id）、`X-IM-Timestamp`（毫秒）、`X-IM-Nonce`（16～64 个 base64url 字符）、`X-IM-Signature`（base64url）；可选 `X-IM-Operator`（后台操作人，格式 `{space}:{accountId}`，如 `admin:a_01J…`）、`X-Request-Id`（≤ 64 字符）、`X-IM-Target-Node`（目标节点 ID）。
2. 待签名串（UTF-8，`\n` 分隔）：`METHOD`、`PATH`（含按键名排序、RFC 3986 编码的查询串）、`TIMESTAMP`、`NONCE`、`hex(SHA-256(body))`（无 body 用空串的哈希）、`X-IM-Operator`（没有则空串）、`X-Request-Id`（没有则空串）、`X-IM-Target-Node`（没有则空串，共 8 行）。HMAC：`HMAC-SHA256(secret, 串)`；Ed25519：私钥对串签名，im-server 只保存公钥。**操作人和请求 ID 在签名范围内**，中间人不能改。节点运维接口（PRD-ops F-OPS-16）复用本规则：调用方 `console-ops`、Ed25519、每节点一把独立私钥（节点只存公钥），`X-IM-Target-Node` 必填且必须等于本节点 `IM_NODE_ID`，失败同样 401 / 4017。
3. 时间窗口：`|服务端时间 − X-IM-Timestamp| ≤ 300 秒`（[IM 钟]），超出拒绝。
4. nonce：同一调用方 10 分钟内重复的 nonce 拒绝；只有签名校验通过的请求才登记 nonce（防止用垃圾请求挤占）。
5. nonce 存储：单节点在内存（容量上限 100 万条，满时拒绝新请求并返回 503 / 5030，打告警日志）；集群（B6）存 Redis（`SET NX EX 600`）。重启后内存清空：重启后 5 分钟内的重放窗口靠时间戳窗口兜底，记为已接受风险（第 13.2 节）。
6. 失败（缺头、调用方不存在或停用、签名错误、超窗、nonce 重复）统一返回 401 / 4017；例外：请求不带 `X-IM-Caller` 时按旧密钥路径判定（F-KEY-05，失败返回 4013，与 PRD-core v1.3 AC-AUTH-01-2 一致），`msg` 区分原因（只用于排查）；失败计入指标 `im_admin_auth_failures_total{reason}`，不写审计（防刷爆，第 13.2 节）。
7. 接入文档提供 Go、Java、Node.js 三种语言的签名示例和测试向量 `protocol/vectors/admin-sign.json`（固定密钥、时间、nonce、body → 期望签名）。

**验收标准**
- AC-KEY-03-1【后端】：输入 `protocol/vectors/admin-sign.json` 中的每一组向量；操作 im-server 的签名校验函数计算；预期 结果与向量一致（HMAC 与 Ed25519 各至少 3 组，含查询串排序、空 body、中文 body）；错误码 —。
- AC-KEY-03-2【后端】：输入 合法签名的请求；操作 篡改 body 一个字节 / 改 `X-IM-Operator` / 改查询串顺序以外的参数值后发送；预期 三种都 401；错误码 4017。
- AC-KEY-03-3【后端】[IM 钟]：输入 时间戳 = 服务端时间 − 301 秒，和 + 301 秒；操作 发送；预期 都 401；时间戳 − 299 秒成功；错误码 4017。
- AC-KEY-03-4【后端】：输入 一个成功请求的完整报文；操作 5 分钟内原样重放；预期 401，业务只执行了一次（例如强制下线只踢一次、审计只有一条成功记录）；错误码 4017。
- AC-KEY-03-5【后端】：输入 1000 个签名错误、nonce 相同的请求；操作 发送后再用该 nonce 发一个正确签名的请求；预期 正确请求成功（错误请求不登记 nonce）；错误码 —。
- AC-KEY-03-6【后端】：输入 缺少 `X-IM-Signature` 的请求；操作 发送；预期 401，`im_admin_auth_failures_total{reason="missing"}` 加 1，`admin_audit` 无新增；错误码 4017。

### F-KEY-04 im-server 侧管理接口审计（第一批前置，P0）

**规则**
1. 每个**写**接口、`user.sessions`、`msg.meta`（单聊）、`msg-grants`、正文读取、`audit.read` 的每次调用，在执行**前**写一条 `admin_audit` 记录（intent），执行后补写结果记录（同一 `requestId`）；intent 写失败则不执行，返回 503 / 5030。只读的 `user.read` / `group.read` 列表不记录。
2. 字段：`id`、`ts`、`caller`、`operator`（只取签名过的 `X-IM-Operator`，未签名或旧密钥为空）、`requestId`、`route`、`targets`（userId / groupId / convId 列表，超过 50 个时只记数量和前 50 个）、`summary`（脱敏参数摘要：数量、旧值 → 新值；**不记正文、备注、推送 data 原文**，push 只记长度和 SHA-256）、`phase`（`intent` / `result`）、`code`。
3. 存储：im-server 主库的 `admin_audit` 表（SQLite 阶段只在应用层只追加：Store 接口只有 `AppendAdminAudit` 和查询方法，没有更新 / 删除；B6 迁 Postgres 后用数据库权限）。保留 1 年，由清理任务删除过期行（与 X-4 一致）。
4. 查询接口 `GET /admin/audit?from=&to=&caller=&operator=&requestId=&cursor=&limit=`（scope `audit.read`，时间范围必填且 ≤ 31 天），供 console-server 对账（F-CON-09 规则 4）和审计员查看"im-server 侧记录"。
5. PRD-social F-SOC-12 规则 9 的 `admin_audit` 即本表，字段以本节为准。

**验收标准**
- AC-KEY-04-1【后端】：输入 `console` 调用方带 `X-IM-Operator: admin:a1`、`X-Request-Id: r-1` 强制下线 u1；操作 查询 `admin_audit`；预期 有 intent、result 两条，`caller=console`、`operator=admin:a1`、`requestId=r-1`、`targets=[u1]`、result 的 `code=0`；错误码 —。
- AC-KEY-04-2【后端】：输入 测试注入 `admin_audit` 写失败；操作 调用强制下线；预期 503，u1 的连接未被踢；错误码 5030。
- AC-KEY-04-3【后端】：输入 好友导入请求中备注为特征串 `ZXQ-REMARK-789`，推送 data 含特征串 `ZXQ-PUSH-456`（第二批）；操作 调用后扫描 `admin_audit` 全部字段和服务端日志；预期 搜不到两个特征串；错误码 —。
- AC-KEY-04-4【后端】：输入 存储层；操作 检查 `store.Store` 接口；预期 对 `admin_audit` 只有追加和查询方法（契约测试断言接口方法列表）；错误码 —。
- AC-KEY-04-5【后端】：输入 `GET /admin/audit` 不带 `from` / `to`，或范围 32 天；操作 调用；预期 400；错误码 4000。

### F-KEY-05 旧 `X-Admin-Key` 的过渡规则（第一批前置，P0）

**规则**
1. 旧密钥 `IM_ADMIN_KEY` 视为调用方 `legacy`，**只能**访问 v1.2 已有的 9 个接口（`token.issue`、`user.write`、`user.read`、`user.kick`、`group.read`、`group.write` 中对应的已有路由）以及测试模式下的 `/admin/test/*`；**调用任何新增接口（第一批、第二批和 PRD-social 的全部新接口）一律 403 / 4030**。
2. 旧密钥只在管理地址上生效（F-KEY-01），不需要签名；调用照常写 `admin_audit`（`caller=legacy`，`operator` 为空）。
3. 生产环境默认关闭：`IM_ENV=production` 时只有显式设置 `IM_ADMIN_LEGACY=1` 才接受旧密钥，并在启动日志和 `/readyz` 详情中给出告警。目前还没有真实业务方接入（U-5），接入时直接使用分密钥 + 签名。
4. 测试和 e2e 继续可以使用旧密钥（现有用例不改）。
5. 旧密钥与新调用方的头同时出现时，以签名头为准，忽略 `X-Admin-Key`。

**验收标准**
- AC-KEY-05-1【后端】：输入 `IM_ADMIN_KEY=k`（非生产）；操作 用 `X-Admin-Key: k` 调用 v1.2 的 9 个接口；预期 行为与 v1.2 一致（现有 `server/e2e` 不改代码全部通过）；错误码 —。
- AC-KEY-05-2【后端】：输入 同上；操作 用旧密钥调用 `GET /admin/users?q=u`、`DELETE /admin/groups/g1`、`POST /admin/friends/import`、`POST /admin/convs/messages/query`；预期 都 403，群未解散、好友未导入；错误码 4030。
- AC-KEY-05-3【后端】：输入 `IM_ENV=production`、配置了 `IM_ADMIN_KEY`、未设 `IM_ADMIN_LEGACY`；操作 用旧密钥调用 `GET /admin/users/u1`；预期 401；错误码 4013。
- AC-KEY-05-4【后端】：输入 旧密钥强制下线 u1；操作 查 `admin_audit`；预期 有记录，`caller=legacy`、`operator` 为空；错误码 —。

---

## 4. console-server 公共基座（管理后台与运维后台的唯一规格）

> 本节是两个后台的**共同规格**。PRD-ops v0.2 引用本节，只定义运维空间的角色、权限点和运维业务；两份 PRD 冲突时以本节为准。本节规则的修改必须同时通知两份 PRD 的作者。环境变量统一前缀 `CON_`，命令行统一为 `console-server <子命令>`。

### 4.0 术语

| 术语 | 定义 |
|---|---|
| console-server | 后台服务（独立仓库，待建）。托管两个后台的前端静态资源和 `/console/api/*`，以调用方身份调用 im-server 管理接口和节点运维接口 |
| 账号空间（space） | `admin`（管理后台）或 `ops`（运维后台）。一个账号只属于一个空间；角色、会话、审计记录都带空间；**一个空间的任何账号或角色都不能读取、创建、修改另一个空间的账号** |
| 空间超管 | 每个空间里拥有账号管理权限的角色：`admin` 空间为 `super_admin`；`ops` 空间为 `ops_superadmin`（PRD-ops F-OPS-02） |
| 会话 | 服务端会话，浏览器只持有随机会话 ID；分两种状态：`setup`（首次设置中，只能调用设置接口）和 `full` |
| step-up | 重新验证：在已登录的会话里再次输入当前 TOTP |
| 审计 intent / result | 操作前写的记录 / 操作后写的结果记录，用同一个 `requestId` 关联 |

### F-CON-01 部署形态、网络边界与启动保护（第一批，P0）

**规则**
1. **一个二进制、两个监听地址**：`CON_ADMIN_ADDR`（管理后台）、`CON_OPS_ADDR`（运维后台），各自绑定一个空间和一个对外域名（`CON_ADMIN_ORIGIN`、`CON_OPS_ORIGIN`，如 `https://admin.console.example.internal`）。`CON_SPACES=admin,ops`（默认两个都开）；可以只开一个，从而把两个空间部署成两组实例（同一份代码），部署上的隔离不需要改需求。
2. 在一个监听地址上收到的会话 Cookie，只认属于该空间的会话；空间不符按无会话处理（401 / 4014）。
3. **只在内网 / VPN 后可达**：每个空间各自配置 `CON_ADMIN_ALLOWED_CIDRS` / `CON_OPS_ALLOWED_CIDRS`，不在范围内的请求（包括登录页和静态资源）返回 403 / 4030。来源 IP 只在 TCP 对端属于 `CON_TRUSTED_PROXIES` 时才从 `X-Forwarded-For` 取（从右往左取第一个不可信地址），否则用 TCP 对端地址。
4. 生产环境只允许 HTTPS（console-server 自己终止 TLS，或由可信代理终止），响应带 `Strict-Transport-Security: max-age=31536000`。由可信代理终止 TLS 时，请求必须带 `X-Forwarded-Proto: https`，否则 403 / 4030。
5. 存储：Postgres 数据库 `console`，schema `auth`（账号、角色、会话、失败计数、激活码、通知）、`audit`（审计记录、链头、锚定记录）、`biz`（导入任务、推送草稿与发送记录、正文授权）。数据库角色：`console_owner`（只用于 `console-server migrate up`，运行时不用）、`console_app`（运行时：`auth`、`biz` 读写；`audit` 只有 INSERT + SELECT，链头表 `audit.chain_head` 只有 SELECT + UPDATE）、`console_archiver`（只给归档任务：`audit` 的 SELECT 和对**已归档分区**的 DROP，见 F-CON-09）。
6. 备份：`console` 库纳入 ARCH 3.4 的备份体系（每日全量 + WAL 连续归档，备份文件加密，ARCH S10）；恢复演练每季度一次。
7. 多实例：console-server 无状态（会话、计数、授权都在库里），可以 ≥ 2 个实例。
8. 调用 im-server：主调用方 `console`（F-KEY-02 规则 4），Ed25519 私钥 `CON_IM_SIGNING_KEY`；第二批再配 `console-msgread` 私钥 `CON_IM_MSGREAD_KEY`。运维节点接口按 F-KEY-03 签名（Ed25519，每节点一把独立私钥，节点只存公钥），见 PRD-ops F-OPS-16。
9. **生产启动保护**（`CON_ENV=production`，任一不满足拒绝启动）：`CON_TEST_MODE` 未开启；已开启的空间配置了 `*_ALLOWED_CIDRS` 且不含 `0.0.0.0/0`、`::/0`；`*_ORIGIN` 为 `https://`；`CON_DATA_KEYS` 合格（F-CON-11）；`CON_IM_SIGNING_KEY` 已配置；`CON_AUDIT_ANCHOR` 已配置（F-CON-08）；数据库连接使用 `console_app` 且实际权限检查通过（启动时尝试对一张审计探针表执行 `UPDATE`，**必须失败**）。各空间 PRD 追加的生产检查（PRD-ops B-OPS-15）一并执行；不满足时**一次列出全部问题**后退出。
10. 测试模式（`CON_TEST_MODE=1`）注册 `/console/test/clock`、`/console/test/faults`（第 2.4 节）。

**验收标准**
- AC-CON-01-1【后端】：输入 `CON_ADMIN_ALLOWED_CIDRS=10.0.0.0/8`，`CON_TRUSTED_PROXIES` 为空；操作 从 `192.168.1.10` 请求登录页，并带 `X-Forwarded-For: 10.0.0.1`；预期 HTTP 403；错误码 4030。
- AC-CON-01-2【后端】：输入 `CON_TRUSTED_PROXIES=10.0.0.2`，代理 10.0.0.2 转发、`X-Forwarded-For: 192.168.1.10`；操作 请求登录页；预期 403（取到的真实来源不在范围内）；同一代理转发 `X-Forwarded-For: 10.1.2.3` 时 200；错误码 4030。
- AC-CON-01-3【后端】：输入 在 admin 空间登录拿到的 Cookie 值；操作 把它发到 `CON_OPS_ADDR` 的 `GET /console/api/session`；预期 401；错误码 4014。
- AC-CON-01-4【后端】：输入 `CON_ENV=production` 分别搭配：`CON_TEST_MODE=1`；`CON_ADMIN_ALLOWED_CIDRS` 未配置；含 `0.0.0.0/0`；未配置 `CON_AUDIT_ANCHOR`；数据库账号对审计表有 UPDATE 权限；操作 启动；预期 五种情况都非 0 退出，日志指出具体原因；错误码 —。
- AC-CON-01-5【后端】：输入 两个 console-server 实例连同一个库；操作 在实例 A 登录，请求发到实例 B；在 B 退出后请求 A；预期 B 返回 200；退出后 A 返回 401；错误码 4014。
- AC-CON-01-6【后端】：输入 `CON_SPACES=admin`；操作 启动并访问 `CON_OPS_ADDR`；预期 运维监听地址未开（连接被拒）；管理后台正常；错误码 —。

### F-CON-02 账号空间与账号生命周期（第一批，P0）

**规则**
1. 账号名：小写字母、数字、`.`、`_`、`-`，3～32 位，**在空间内唯一**（重复返回 409 / 4092）；一个账号对应一个真人，禁止共用。账号表预留 `idp_subject`、`idp_issuer`（SSO，P1）。
2. **引导**：每个空间的第一个空间超管只能用命令行 `console-server account bootstrap --space admin|ops --username <u>` 创建；该空间已有任何启用的空间超管时命令拒绝执行（非 0 退出）。命令写一条审计（`actor=cli:{操作系统用户}@{主机名}`，`action=account.bootstrap`），输出一次性激活码。网页上没有注册入口。
3. **创建**：空间超管在**本空间**内创建账号（账号名、显示名、角色），系统不生成初始密码，而是生成**一次性激活码**（20 位 base32，24 小时有效，只显示一次，库里只存哈希）。超管通过线下渠道交给本人。
4. **激活**：本人用"账号名 + 激活码 + 新密码"激活（`POST /console/api/auth/activate`），成功后得到 `setup` 会话，扫码绑定 TOTP 并提交一次正确的动态码后，发放 10 个恢复码（只显示一次）并把会话升为 `full`。激活码错误按登录失败计数（F-CON-04）。激活、绑定的 IP 和 UA 写审计；**每次激活或重置后的首次绑定，给本空间全部空间超管和审计员发安全通知**（F-CON-12），便于发现"超管截留激活码冒充他人"。
5. **停用 / 启用**：停用后该账号全部会话立即失效，登录按"账号或密码错误"处理（不暴露停用状态）。账号**不能删除**（保证审计中的操作人可查）。
6. **重置**：空间超管可以对本空间账号"重置密码与二次验证"（一并重置，不单独重置一项）：账号回到"待激活"，旧密码、TOTP、恢复码、全部会话立即作废，生成新激活码。
7. **跨空间隔离**：`admin` 空间的超管看不到也操作不了 `ops` 空间的账号；反之亦然。运维账号只能由运维超管创建和重置（X-2 ②）。对另一空间的账号 ID 调用任何账号接口一律返回 404 / 4040（不承认其存在），并写一条 `result=denied`、`code=4040` 的审计。
8. **自我保护**：不能修改自己的角色、不能停用或重置自己；不能让空间内启用的空间超管数降到 0（检查与修改在同一事务内、对该空间的超管集合加锁，防止两个超管并发互相停用）。违反时返回 409 / 4093，状态不变（应急授权见 F-CON-06 规则 6）。
9. 一个空间超管重置另一个空间超管时，需要第二个空间超管确认（双人复核）：**P1**；P0 只做"立即通知被重置者以外的全部空间超管和审计员"。
10. 账号创建、角色变更、停用、启用、重置、解锁、激活都写审计，并需要 step-up（F-CON-07）。

**验收标准**
- AC-CON-02-1【后端】：输入 admin 空间没有任何账号；操作 `console-server account bootstrap --space admin --username frank`；预期 退出码 0，输出激活码，审计新增 `account.bootstrap`；再执行一次同样命令（frank 已激活）退出码非 0；错误码 —。
- AC-CON-02-2：输入 超管新建客服账号 `kf1`（角色 `support`）；操作 kf1 用激活码 + 新密码激活、扫码绑定 TOTP、输入动态码；预期 进入首页，侧边栏只有客服权限内的菜单；全部空间超管和审计员收到"kf1 已激活"通知；错误码 —。
- AC-CON-02-3【后端】[C 钟]：输入 创建 kf2 后 24 小时零 1 分钟；操作 用激活码激活；预期 401，提示"账号或密码错误"；错误码 4015。
- AC-CON-02-4【后端】：输入 kf1 处于 `setup` 会话（已激活、未绑定 TOTP）；操作 调用 `GET /console/api/users?q=u`；预期 401；错误码 4016。
- AC-CON-02-5【前端】：输入 kf1 处于 `setup` 会话；操作 在地址栏直接打开 `/users`；预期 跳回"绑定二次验证"页；错误码 4016（前端据此跳转）。
- AC-CON-02-6【后端】：输入 admin 空间超管 a1 的会话；操作 调用 `GET /console/api/accounts?username=ops1`、`POST /console/api/accounts/{ops1 的 id}/reset`；预期 列表不含任何 ops 账号；重置返回 404，ops1 状态不变；审计新增一条 `space=admin`、`result=denied`、`code=4040` 的记录；错误码 4040。
- AC-CON-02-7【后端】：输入 admin 空间只有一个启用的超管 a1；操作 a1 修改自己的角色、停用自己、重置自己；再新建超管 a2 后，a1 修改自己的角色；预期 四次都 409，状态不变；错误码 4093。
- AC-CON-02-8【后端】：输入 admin 空间只有 a1、a2 两个启用的超管，都已 step-up；操作 a1 停用 a2 与 a2 停用 a1 两个请求并发发出（重复 20 轮，每轮恢复初始状态）；预期 每轮恰好一个成功，另一个返回 409 / 4093 或 401 / 4014（会话已随停用失效），任何一轮结束时启用的超管数都 ≥ 1；错误码 4093、4014。
- AC-CON-02-9【后端】：输入 超管重置 kf1；操作 用 kf1 原 Cookie 请求、用原密码 + 原 TOTP 登录、用原恢复码登录；预期 三种都失败（401 / 4014、401 / 4015、401 / 4015）；新激活码可以激活；错误码 4014、4015。
- AC-CON-02-10【后端】：输入 已存在账号 `kf1`；操作 新建同名账号；预期 409；在 ops 空间新建 `kf1` 成功（空间内唯一）；错误码 4092。
- AC-CON-02-11【后端】：输入 账号管理接口；操作 对 `/console/api/accounts/{id}` 发 `DELETE`；预期 405；错误码 —。

### F-CON-03 登录（第一批，P0）

**规则**
1. **一步提交**（与 PRD-ops 统一，评审 R-ADM-28）：`POST /console/api/auth/login {username, password, otp}` 或 `{username, password, recoveryCode}`；三项都正确才发放 `full` 会话。
2. 账号不存在、密码错、动态码错、恢复码错、账号已停用、账号待激活：**统一 401 / 4015**，`msg` 相同；对不存在的账号也执行一次假的 Argon2id 计算，使响应时间一致。
3. TOTP：RFC 6238，SHA-1，6 位，30 秒步长，允许前后各 1 个步长。**防重放**：记录每个账号最近一次成功使用的步长，步长 ≤ 该值的码一律拒绝；范围覆盖登录和 step-up（同一个码不能先登录再用于 step-up）。
4. 恢复码：10 个，每个只能用一次，库里只存加盐哈希；使用恢复码登录写审计并给本人发安全通知；剩余 ≤ 2 个时登录后提示重新生成；重新生成需要 step-up，旧码全部作废，写审计。
5. 登录成功：生成新的会话 ID（防会话固定），返回 `{account, roles, perms, csrfToken}`。
6. 登录成功、失败（带失败类别，仅审计内部可见）、锁定、恢复码使用都写审计（F-CON-08 规则 9 的"认证事件"），**审计里不出现密码、动态码、恢复码**。
7. SSO（P1）：账号有 `idp_subject` 时走 OIDC 登录，仍由 console-server 做角色、会话、审计；每个空间保留至少 1 个本地空间超管作应急。

**验收标准**
- AC-CON-03-1：输入 已激活账号 kf1、正确密码、当前动态码；操作 登录；预期 200，进入首页，`Set-Cookie` 为 `__Host-adm_sid`；错误码 —。
- AC-CON-03-2【后端】：输入 6 种失败情形（不存在、密码错、动态码错、恢复码错、已停用、待激活）；操作 各登录一次；预期 6 次响应的 HTTP 状态、`code`、`msg`、响应头集合完全相同；错误码 4015。
- AC-CON-03-3【后端】：输入 隔离的测试环境，"不存在账号"与"存在账号密码错误"各 50 次；操作 登录并记录耗时；前提 测试模式关闭账号锁定与 IP 限流（否则第 6 次起会返回 4291 / 4290）；预期 两组中位数相差 ≤ 30ms；另有单元测试断言不存在的账号也调用了一次 Argon2id；错误码 4015。
- AC-CON-03-4【后端】[C 钟]：输入 kf1 在步长 T 用码 X 登录成功；操作 在同一步长内用 X 再登录一次，再用 X 做 step-up；预期 两次都失败（401 / 4015），且计入失败次数；错误码 4015。
- AC-CON-03-5【后端】[C 钟]：输入 步长 T−1、T+1 的码，和 T−2 的码；操作 登录；预期 前两个成功（分别在两个独立用例中），T−2 失败；错误码 4015。
- AC-CON-03-6：输入 恢复码 R1；操作 用 R1 代替动态码登录，退出后再用 R1 登录；预期 第一次成功，审计有 `auth.recovery_code_used`，本人收到安全通知；第二次 401；错误码 4015。
- AC-CON-03-7【后端】：输入 登录前浏览器里被预先设置 `__Host-adm_sid=X`；操作 登录成功；预期 新 Cookie 值 ≠ X；用 X 请求 `GET /console/api/session` 返回 401；错误码 4014。
- AC-CON-03-8【后端】：输入 密码为特征串 `Pw-CANARY-7f3a91`、动态码 `123456`（错误）；操作 登录失败后扫描 console-server 日志、反向代理访问日志、`audit` 表全部字段；预期 找不到 `Pw-CANARY-7f3a91`，审计中没有动态码字段；错误码 4015。
- AC-CON-03-9【后端】：输入 剩余 2 个恢复码的账号；操作 登录后调用 `POST /console/api/me/recovery-codes`（未 step-up）；预期 403；step-up 后成功，返回 10 个新码，旧码全部失效，审计有 `auth.recovery_codes_regenerated`；错误码 4038。

### F-CON-04 密码策略、锁定与限流（第一批，P0）

**规则**
1. 密码 12～128 个字符；不能包含账号名（不区分大小写）；不能在弱密码表里（仓库内 `console-server/assets/weak-passwords.txt`，来源：SecLists `10-million-password-list-top-10000.txt`，固定版本并记录 SHA-256）；不强制字符种类（NIST SP 800-63B）；新密码不能与最近 5 次相同。不合规返回 400 / 4000，`reason` 为 `tooShort` / `tooLong` / `containsUsername` / `weak` / `reused`。
2. 修改密码需要当前密码 + 当前动态码；成功后删除该账号的其他会话，保留当前会话。
3. **锁定**（防枚举、防低频锁人）：失败计数按"规范化账号名 + 来源 IP"维护，**不管账号是否存在**；15 分钟内同一键连续失败 5 次，该键锁定 15 分钟。另外按账号名维护全局计数：1 小时内 50 次失败，该账号名对所有 IP 锁定 1 小时。锁定期间的登录**无论凭证是否正确**都返回 429 / 4291；对不存在的账号名同样会锁定、同样返回 4291，所以锁定提示不泄露账号是否存在。
4. 成功登录清零该"账号名 + IP"的计数。step-up 失败计入同一账号的计数（F-CON-07 规则 4）。
5. 按 IP 限流：每个 IP 每分钟最多 20 次 `login` + `activate` 请求，超出返回 429 / 4290（与锁定区分）。
6. 解锁：空间超管可以在页面解锁本空间账号（step-up、写审计）；命令行 `console-server account unlock --space --username` 可以解锁任何账号（应急，写审计 `actor=cli:{操作系统用户}@{主机名}`、`action=account.unlock`）。
7. 账号被锁定（键为已存在的账号）时，给本人和本空间空间超管发安全通知。
8. 失败计数、锁定状态都在 Postgres，多实例一致。

**验收标准**
- AC-CON-04-1【后端】：输入 账号 kf1 分别设置密码 `abc12345678`（11 位）、`password1234`（在弱密码表中）、`xkf1yzzzzzzzz`（含账号名）、与最近 5 次之一相同、16 位随机串；操作 修改密码；预期 前四个 400，`reason` 依次为 `tooShort`、`weak`、`containsUsername`、`reused`；第五个成功；错误码 4000。
- AC-CON-04-2【后端】：输入 两个账号设置相同密码；操作 读 `auth.accounts.password_hash`；预期 都以 `$argon2id$` 开头且值不同；错误码 —。
- AC-CON-04-3【后端】[C 钟]：输入 kf1 从 IP-A 连续 5 次密码错误；操作 第 6 次用正确密码 + 正确动态码从 IP-A 登录；预期 429；从 IP-B 用正确凭证登录成功；15 分钟后从 IP-A 登录成功；错误码 4291。
- AC-CON-04-4【后端】：输入 不存在的账号名 `ghost` 从 IP-A 连续 5 次失败；操作 第 6 次登录；预期 429，响应与 AC-CON-04-3 第 6 次完全相同；错误码 4291。
- AC-CON-04-5【后端】：输入 kf1 从 IP-A 失败 4 次后成功 1 次；操作 再失败 4 次；预期 第 4 次仍是 401 / 4015，没有锁定；错误码 4015。
- AC-CON-04-6【后端】[C 钟]：输入 kf1 在 1 小时内从 11 个 IP 各失败 5 次以内、合计 50 次；操作 第 51 次从新 IP 用正确凭证登录；预期 429；1 小时后成功；错误码 4291。
- AC-CON-04-7【后端】：输入 同一 IP；操作 1 分钟内发第 21 次 `login` 请求（凭证随意）；预期 429；错误码 4290。
- AC-CON-04-8【后端】：输入 两个实例 A、B；操作 kf1 从同一 IP 在 A 失败 3 次、在 B 失败 2 次；预期 第 6 次在任一实例都返回 429；错误码 4291。
- AC-CON-04-9【后端】：输入 kf1 被锁定；操作 超管未 step-up 调用解锁、step-up 后再调用；预期 第一次 403 / 4038；第二次成功，kf1 可立即登录，审计有 `account.unlock`，kf1 和超管收到锁定时的安全通知；错误码 4038。

### F-CON-05 会话（第一批，P0）

**规则**
1. 会话存在 Postgres（`auth.sessions`）；Cookie 只放随机会话 ID（≥ 128 位熵），**库里只存 SHA-256**。
2. Cookie：管理空间 `__Host-adm_sid`，运维空间 `__Host-ops_sid`；`HttpOnly`、`Secure`、`SameSite=Strict`、`Path=/`，不设 `Domain`。会话凭证不放 localStorage / sessionStorage。
3. 过期：空闲 30 分钟；绝对 12 小时。**带请求头 `X-Console-Background: 1` 的请求（自动刷新、轮询）不刷新空闲计时**；页面不可见时前端暂停自动刷新。由用户操作发起的请求（点击、手动刷新、提交表单）不得带 `X-Console-Background`。经 console-server 认证代理转发到观测组件的请求（运维空间的 Grafana，PRD-ops F-OPS-17），只有 `Sec-Fetch-Mode: navigate` 的请求刷新空闲计时；代理 Cookie 绑定原会话，原会话失效时立即失效。
4. 失效时机：退出登录；改密码（保留当前会话）；被停用、被重置；角色变化（该账号全部会话失效，重新登录后按新角色）；step-up 连续失败 5 次（F-CON-07）。
5. 每个账号最多 3 个并发会话，第 4 个登录成功时最早的会话失效；被挤掉的会话下一次请求返回 401 / 4014，`reason: "replaced"`，前端提示"你的账号在别处登录，本次登录已退出"。
6. 4014 的 `reason`：`none`（没带 Cookie）、`idle`、`absolute`、`loggedOut`、`revoked`（停用 / 重置 / 改角色 / 改密码）、`replaced`。前端只用它选择提示文案。
7. **CSRF**：所有非 GET / HEAD 请求必须同时满足：① 请求头 `X-CSRF-Token` 等于会话绑定的令牌；② `Origin` 等于该空间的 `CON_*_ORIGIN`。否则 403 / 4037，业务不执行。令牌在登录响应和 `GET /console/api/session` 中返回；前端只放内存，**刷新页面后重新调用 `GET /console/api/session` 取**。GET 请求不得有副作用。
8. 所有 `/console/api/*` 响应带 `Cache-Control: no-store`。

**验收标准**
- AC-CON-05-1【后端】：输入 登录成功的响应；操作 检查 `Set-Cookie`；预期 名称 `__Host-adm_sid`，含 `HttpOnly`、`Secure`、`SameSite=Strict`、`Path=/`，没有 `Domain`；错误码 —。
- AC-CON-05-2【前端】：输入 已登录页面；操作 执行 `document.cookie`、遍历 localStorage / sessionStorage；预期 都找不到会话 ID；错误码 —。
- AC-CON-05-3【后端】[C 钟]：输入 最后一次普通请求在 T；操作 T+29 分钟发一次普通请求，再过 31 分钟（T+60 分钟）发第二次；预期 第一次 200；第二次 401，`reason=idle`；错误码 4014。
- AC-CON-05-4【后端】[C 钟]：输入 登录后每 10 分钟发一次普通请求；操作 登录后 12 小时零 1 分钟请求；预期 401，`reason=absolute`；错误码 4014。
- AC-CON-05-5【后端】[C 钟]：输入 登录后只有带 `X-Console-Background: 1` 的请求，每 60 秒一次；操作 登录后第 31 分钟发一个普通请求；预期 401，`reason=idle`；错误码 4014。
- AC-CON-05-6【后端】：输入 退出前的 Cookie；操作 退出后重放 `GET /console/api/session`；预期 401，`reason=loggedOut`；错误码 4014。
- AC-CON-05-7【后端】：输入 已登录会话；操作 发 `POST /console/api/users/import`，不带 `X-CSRF-Token`；预期 403，导入未执行；错误码 4037。
- AC-CON-05-8【后端】：输入 同上；操作 带错误的 `X-CSRF-Token`；预期 403；错误码 4037。
- AC-CON-05-9【后端】：输入 同上；操作 带正确令牌、`Origin: https://evil.example`；预期 403；错误码 4037。
- AC-CON-05-10【前端】：输入 已登录页面；操作 刷新浏览器后立即提交一次强制下线；预期 前端先调用 `GET /console/api/session` 取得新令牌，强制下线成功，没有出现 4037 提示；错误码 —。
- AC-CON-05-11【后端】：输入 kf1 依次登录 4 次（4 个会话）；操作 用第 1 个会话请求；预期 401，`reason=replaced`；错误码 4014。
- AC-CON-05-12【前端】：输入 会话被挤掉；操作 在页面上点任意菜单；预期 跳到登录页并显示"你的账号在别处登录，本次登录已退出"；错误码 4014。
- AC-CON-05-13【后端】：输入 kf1 两个会话；操作 超管把 kf1 的角色从 `support` 改为 `operator`；预期 两个会话下一次请求都 401，`reason=revoked`；重新登录后 `perms` 为运营权限；错误码 4014。
- AC-CON-05-14【后端】：输入 任一 `/console/api/*` 接口；操作 检查响应头；预期 含 `Cache-Control: no-store`；错误码 —。
- AC-CON-05-15【后端】：输入 `auth.sessions` 表；操作 登录后对比 Cookie 值与库中字段；预期 库中只有 SHA-256 值，找不到 Cookie 原值；错误码 —。

### F-CON-06 角色与授权机制（第一批，P0）

**规则**
1. **可叠加角色**：一个账号可以有多个角色（同一空间内），权限取并集。每个空间的角色和权限点由各自的 PRD 定义（管理后台见第 5 节；运维后台见 PRD-ops）。
2. **白名单**：每个 `/console/api/*` 路由在代码里声明所需权限点（或声明为"只需登录"），未声明的路由启动失败。权限只在服务端判断，前端隐藏菜单只是体验。各空间的权限点由各自 PRD 定义（管理空间见第 5.2 节，运维空间见 PRD-ops F-OPS-02 规则 1）。
3. 无权限返回 403 / 4030。已认证的拒绝写审计（`result=denied`）；同一账号 1 分钟内超过 20 次拒绝时，之后的拒绝在该分钟结束时合并为一条（含次数和动作分布）。**未认证的拒绝**（无会话、CSRF 失败、CIDR 拒绝）不进审计表，只写日志和指标 `console_rejected_total{reason}`（防刷爆审计，R-ADM-29）。
4. **职责分离**：每个空间的 PRD 声明本空间的**互斥角色**，互斥角色不能与同空间任何其他角色叠加（授予时返回 409 / 4093）。管理空间：`auditor`；运维空间：`ops_auditor`、`ops_superadmin`（PRD-ops F-OPS-02）。空间超管默认**没有**正文查看、删除消息、全员推送、审计查看权限（第 5 节）。
5. 授予敏感角色（管理后台：`moderator`、`auditor`、`super_admin`；运维后台由 PRD-ops 指定）时，给本空间全部审计员发安全通知。
6. **应急授权**：**只在管理空间提供**（权限点 `admin.emergency`），运维空间不提供。空间超管可以给自己临时加业务角色（不能加 `auditor`、不能加空间超管），时长 15～240 分钟，需要 step-up + 理由；写审计并立即通知本空间全部审计员和其他空间超管；到期自动撤销（判断时按 [C 钟] 当前时间，不需要定时任务）。这是"不能修改自己角色"（F-CON-02 规则 8）的唯一例外。运维空间没有 `admin.emergency` 权限点，任何运维角色调用该接口都返回 403 / 4030（PRD-ops AC-OPS-02-9）。运维超管本人不能获得任何运维数据权限（互斥角色）；需要排障时，使用由运维超管另行创建的 `ops_engineer` 账号（第二批起需第二人批准，PRD-ops F-OPS-02）。

**验收标准**
- AC-CON-06-1【后端】：输入 在代码里新增一个没有声明权限点的 `/console/api/*` 路由；操作 运行测试和启动；预期 启动失败（"route without permission"）；错误码 —。
- AC-CON-06-2【后端】：输入 账号同时有 `support`、`operator`；操作 分别调用强制下线（support 才有）和用户导入（operator 才有）；预期 都成功；错误码 —。
- AC-CON-06-3【后端】：输入 只有 `support` 的账号；操作 调用用户导入；预期 403，审计新增 `result=denied`、`action=user.import`；错误码 4030。
- AC-CON-06-4【后端】：输入 同一账号 1 分钟内 30 次无权限请求；操作 查审计；预期 前 20 次各一条，其余合并为 1 条（`count=10`）；错误码 4030。
- AC-CON-06-5【后端】：输入 无会话；操作 发 100 次 `GET /console/api/users`；预期 都 401，审计表无新增，`console_rejected_total{reason="noSession"}` 增加 100；错误码 4014。
- AC-CON-06-6【后端】：输入 账号有 `support`；操作 超管再授予 `auditor`；预期 409，角色不变；错误码 4093。
- AC-CON-06-7【后端】[C 钟]：输入 超管 a1；操作 step-up 后为自己申请 30 分钟 `moderator` 应急授权，理由"处理举报 T-1"；31 分钟后调用 `moderator` 才有的接口；预期 申请成功，审计有 `account.emergency_grant`，审计员和其他超管收到通知；31 分钟后 403；错误码 4030。
- AC-CON-06-8【后端】：输入 超管 a1；操作 应急授权申请 `auditor`、申请 300 分钟、不带理由；预期 三次都 400；错误码 4000。
- AC-CON-06-9【后端】：输入 超管给 kf1 授予 `moderator`；操作 查审计员的通知；预期 有"kf1 被授予 moderator"通知；错误码 —。
- AC-CON-06-10【后端】：输入 ops 空间账号已有 `ops_engineer`；操作 运维超管再授予 `ops_superadmin` 或 `ops_auditor`；预期 都 409，角色不变；错误码 4093。

### F-CON-07 重新验证（step-up）（第一批，P0）

**规则**
1. `POST /console/api/auth/step-up {otp}`：成功后在**当前会话**上记录验证时间，之后 5 分钟内同一会话的高危操作不再要求输入（[C 钟]）。
2. 需要 step-up 的操作由各空间 PRD 列出（管理后台见第 5.3 节），基座本身要求：账号创建 / 角色变更 / 停用 / 启用 / 重置 / 解锁、应急授权、重新生成恢复码、审计导出（P1）。
3. **正文查看授权（F-ADM-08）、隔离原文查看（F-ADM-32）不享受 5 分钟免输**：每次申请都必须在请求里带当次动态码。
4. step-up 输错计入该账号的锁定计数；**同一会话连续 5 次 step-up 失败，注销该会话（401 / 4014，`reason=revoked`）并锁定该账号 15 分钟**（对所有 IP），通知本人和空间超管。
5. 需要 step-up 而未验证（或已超过 5 分钟）时返回 **403 / 4038**，业务不执行；前端弹框、验证通过后自动重放原请求一次。**不返回 401**。
6. 输错动态码返回 401 / 4015（前端在弹框内提示"动态码错误"，不跳登录页）。

**验收标准**
- AC-CON-07-1【后端】：输入 从未 step-up 的会话；操作 调用解散群；预期 403，群未解散；错误码 4038。
- AC-CON-07-2【后端】[C 钟]：输入 step-up 成功于 T；操作 T+4 分钟解散群 g1、T+6 分钟解散群 g2；预期 g1 成功不需要输入；g2 返回 403；错误码 4038。
- AC-CON-07-3【前端】：输入 未 step-up 的会话；操作 点击"解散群"并完成确认；预期 弹出 TOTP 框，不跳登录页；输入正确码后自动完成解散，不需要再点一次；错误码 4038（触发弹框）。
- AC-CON-07-4【后端】：输入 step-up 成功后 1 分钟；操作 申请群消息正文查看授权，请求里不带 `otp`；预期 403；错误码 4038。
- AC-CON-07-5【后端】：输入 有效会话；操作 连续 5 次 step-up 输错；预期 前 4 次 401 / 4015；第 5 次后该会话下一次请求 401，`reason=revoked`；用正确凭证从任意 IP 登录返回 429 / 4291；错误码 4015、4014、4291。
- AC-CON-07-6【前端】：输入 step-up 弹框；操作 输入错误动态码；预期 弹框内提示"动态码错误"，仍停留在当前页面；错误码 4015。

### F-CON-08 审计记录（第一批，P0）

**规则**
1. **记录范围**：所有写操作；所有敏感查看（正文、隔离原文、在线设备、单聊元数据、审计查询本身）；认证事件（登录成功 / 失败、退出、锁定、激活、绑定、恢复码使用与重新生成、step-up 失败）；已认证的权限拒绝（F-CON-06 规则 3）；命令行操作（`bootstrap`、`unlock`、`audit verify`、归档）。纯浏览（不含正文、不涉及单聊关系的列表与详情）不记录。
2. **字段**：`seq`（全局递增，链序）、`id`（UUIDv7）、`ts`、`space`、`phase`（`intent` / `result` / `event`）、`requestId`、操作人（`accountId`、`username`、当时的角色集合；命令行为 `cli:{操作系统用户}@{主机名}`）、来源 IP（按 F-CON-01 规则 3 解析，完整保存）、UA、`action`（枚举，如 `user.kick`）、`target`（类型 + ID 列表，超过 50 个时记数量和前 50 个）、`params`（脱敏摘要）、`reason`、`ticket`、`result`（`success` / `failure` / `denied` / `unknown`）、`code`、`prevHash`、`hash`。
3. **不记录**：密码、动态码、恢复码、激活码、会话 ID、CSRF 令牌、任何密钥、**消息正文**、好友备注；推送 `data` 只记长度和 SHA-256（全文在 `biz.push_records`，F-ADM-50）。
4. **两阶段**：写操作和敏感查看先写 `intent`（含全部参数摘要和理由）；**intent 写入失败则不执行操作、不返回数据，响应 503 / 5031**。执行后写 `result`（同 `requestId`）。result 写入失败时，操作已经生效：响应照常返回真实结果，同时打日志和指标 `console_audit_result_failures_total`（告警）；该 intent 在审计页显示为"结果未知"，由对账任务补齐（F-CON-09 规则 4）。纯查看类（无副作用）只写一条 `event` 记录，写失败则不返回数据（5031）。
5. **调用 im-server 超时**（默认 10 秒）：不重试写操作，响应 504 / 5040（"结果未知"），result 记为 `unknown`；对账任务按 `requestId` 从 im-server 侧审计（F-KEY-04）补写结论。例外：系统通知由对账任务按 `pushId` 重放（PRD-live 4.7 节对账表）。
6. **哈希链**：全局一条链。`hash = SHA-256(prevHash ‖ canonical_json(记录除 hash 外的全部字段))`。写入时在同一事务里 `SELECT … FOR UPDATE` 锁 `audit.chain_head` 单行，取 `seq`、`prevHash`，插入记录，更新链头，提交——多实例并发写严格串行。
7. **外部锚定**（P0）：每小时整点，以及每新增 1000 条记录，把 `(seq, hash, ts)` 写到 `CON_AUDIT_ANCHOR` 指定的外部只追加存储（对象存储的 WORM / Object Lock 桶，或公司日志平台；console-server 对它只有写入权限）。锚定记录同时写入 `audit.anchors`。锚定失败告警，并在 10 分钟内重试。
8. **校验命令** `console-server audit verify [--from-seq]`：逐条重算哈希、检查 seq 连续、把每个外部锚定点与库中同 seq 的 hash 比较、检查库中最大 seq ≥ 最近一次锚定的 seq；发现问题输出第一个异常的 seq 和类型（`hashMismatch` / `gap` / `anchorMismatch` / `truncated`）并以非 0 退出。该命令每天自动运行一次，失败告警。
9. **事件类记录**（认证事件、命令行操作）只写 `phase=event` 一条。
10. 数据库权限保证只追加：`console_app` 对 `audit.records` 没有 UPDATE / DELETE / TRUNCATE。

**验收标准**
- AC-CON-08-1：输入 客服 kf1 对 u1 强制下线，理由"工单 T-100 处理"；操作 查审计；预期 有 intent、result 两条，`requestId` 相同，`actor=kf1`、`action=user.kick`、`target=user:u1`、`reason` 一致、result `success`、IP 为测试机 IP；错误码 —。
- AC-CON-08-2【后端】：输入 im-server 返回 404 / 4042（用户不存在）；操作 强制下线 `nobody`；预期 result 记录 `result=failure`、`code=4042`；页面提示"用户不存在"；错误码 4042。
- AC-CON-08-3【后端】：输入 用 `console_app` 账号连接数据库；操作 对 `audit.records` 执行 `UPDATE`、`DELETE`、`TRUNCATE`；预期 三条都报权限错误；错误码 —。
- AC-CON-08-4【后端】：输入 测试注入 `auditWrite=fail`；操作 解散群 g1、查询 u1 在线设备；预期 都返回 503，g1 未解散（im-server 侧 `admin_audit` 无该请求），在线设备数据未返回；错误码 5031。
- AC-CON-08-5【后端】：输入 1000 条审计记录；操作 用库超级用户删除第 500 条后运行 `audit verify`；预期 非 0 退出，输出 `gap` 和 seq 500；错误码 —。
- AC-CON-08-6【后端】：输入 最近一次锚定在 seq 900；操作 用超级用户删除 seq 950～1000（截断尾部）后运行 verify；预期 通过（截断发生在锚定之后，属于已接受风险，第 13.2 节）；再删除 seq 850～1000 后运行 verify，非 0 退出，输出 `truncated`；错误码 —。
- AC-CON-08-7【后端】：输入 1000 条记录、锚定点 seq 500 和 1000；操作 用超级用户修改第 10 条的 `reason` 并从第 10 条开始整链重算 hash 后运行 verify；预期 非 0 退出，输出 `anchorMismatch` 和 seq 500；错误码 —。
- AC-CON-08-8【后端】：输入 两个实例各并发写 500 条审计；操作 运行 verify；预期 通过，seq 1～1000 连续无重复；错误码 —。
- AC-CON-08-9【后端】：输入 测试注入 `auditWrite=failResult`；操作 强制下线 u1；预期 u1 被踢下线，接口返回成功；审计只有 intent；审计页该条显示"结果未知"；对账任务运行后新增一条 `phase=result`、`result=success`、`params.reconciled=true` 的记录；错误码 —。
- AC-CON-08-10【后端】：输入 测试注入 `imLatencyMs=15000`；操作 强制下线 u1；预期 504，result 为 `unknown`；对账后补为 success；错误码 5040。
- AC-CON-08-11【后端】：输入 规则 1 的每一类动作（参数化，含管理后台第一批全部写操作和敏感查看）；操作 各执行一次；预期 每类在审计中都有对应记录，字段齐全；错误码 —。
- AC-CON-08-12【后端】[C 钟]：输入 `CON_AUDIT_ANCHOR` 指向测试 WORM 桶；操作 推进 1 小时；预期 桶中新增一条锚定对象，内容的 seq 和 hash 与库中一致；console-server 的凭证删除该对象返回权限错误；错误码 —。

### F-CON-09 审计查看、对账、保留与归档（第一批，P0）

**规则**
1. 只有审计查看权限的角色能看（管理后台为 `auditor`；运维空间由 PRD-ops 定义，运维空间的审计员可以选择"全部空间"，管理后台的审计员只能看 `admin` 空间）。
2. 查询：时间范围必填，单次 ≤ 31 天（两个后台统一）；可按操作人、动作、目标、结果、`requestId` 筛选；按 `seq` 倒序游标分页。查询本身写一条 `audit.query` 事件（含筛选条件）。
3. 详情：同一 `requestId` 的 intent / result 合并显示；可以跳转查看 im-server 侧同一 `requestId` 的记录（经 `GET /admin/audit`）。
4. **对账任务**：每 10 分钟扫描"intent 超过 5 分钟没有 result"的记录，按 `requestId` 查询 im-server 侧审计：有结论则补写一条 `result`（`params.reconciled=true`）；im-server 侧也没有该请求则补写 `result=failure`、`code=0`、`params.reconciled="notExecuted"`（console 侧在调用前失败）。
5. 只读：没有修改、删除审计的接口；对审计接口发 `PUT` / `PATCH` / `DELETE` 返回 405。
6. **保留与归档**（X-4）：`audit.records` 按月分区。在线保留 1 年；满 1 年的分区由归档任务导出为 JSONL，用 age 加密（公钥 `CON_ARCHIVE_RECIPIENT`），写入归档存储（与锚定存储不同的桶，开启 Object Lock），校验可解密且哈希链与锚定一致后，用 `console_archiver` 删除该分区；归档文件保存 3 年（自归档起，`CON_AUDIT_ARCHIVE_YEARS=3`）。归档操作本身写审计。归档后的链校验从归档文件的最后一条接续。
7. 导出 CSV（P1）：单次 ≤ 1 万条，需要 step-up，写审计。

**验收标准**
- AC-CON-09-1【前端】：输入 审计员按"操作人 = kf1、动作 = 强制下线、最近 7 天"筛选；操作 查询；预期 只出现符合条件的记录；时间范围选 32 天时"查询"按钮不可点并提示"最长 31 天"；错误码 —。
- AC-CON-09-2【后端】：输入 时间范围 32 天的查询请求；操作 调用审计查询接口；预期 400；错误码 4000。
- AC-CON-09-3【后端】：输入 审计员查询一次；操作 再查审计；预期 新增 `action=audit.query` 的记录，`params` 含筛选条件；错误码 —。
- AC-CON-09-4【后端】：输入 admin 空间的审计员；操作 查询时指定 `space=ops`；预期 403；错误码 4030。
- AC-CON-09-5【后端】：输入 审计接口；操作 发 `PATCH` 和 `DELETE`；预期 都 405；错误码 —。
- AC-CON-09-6【后端】[C 钟]：输入 一条 intent 没有 result（测试注入），im-server 侧也没有该 `requestId`；操作 推进 15 分钟让对账任务运行；预期 新增 `result=failure`、`reconciled=notExecuted` 的记录；错误码 —。
- AC-CON-09-7【后端】[C 钟]：输入 13 个月的测试审计数据；操作 运行归档任务；预期 最早 1 个月的分区被导出、加密、写入归档桶，解密后条数与哈希和库中一致，之后分区被删除；`audit verify` 仍通过；审计新增 `audit.archive`；错误码 —。
- AC-CON-09-8【后端】：输入 `console_app` 账号；操作 尝试 `DROP` 一个审计分区；预期 权限错误；错误码 —。
- AC-CON-09-9：输入 管理后台执行一次强制下线；操作 运维空间审计员选择"全部空间"查询；预期 能看到该记录（`space=admin`）；错误码 —。

### F-CON-10 浏览器侧安全（第一批，P0）

**规则**
1. IM 管理凭证、console 的签名私钥只在 console-server 的服务端配置里；前端代码、构建产物、接口响应、Cookie、日志中都不能出现。
2. 用户可控内容（昵称、群名、群公告、消息正文、推送内容、审计里的理由）一律按纯文本渲染，禁止 `dangerouslySetInnerHTML` / `innerHTML`（ESLint 规则强制）。
3. **头像**：后台**不加载业务方传入的外部头像 URL**，统一显示首字母占位（防止向第三方泄露后台人员的 IP 和 UA）；需要看原图时显示 URL 文本（P1：经 console-server 图片代理）。
4. 响应头：`Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'nonce-{每次响应随机}'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`；`X-Content-Type-Options: nosniff`；`Referrer-Policy: no-referrer`；`Strict-Transport-Security`（生产）；`Cache-Control: no-store`（接口）。**前端组件库选型条件：能在上述 CSP 下工作**；若确实做不到，只允许对 `style-src` 加 `'unsafe-inline'`，并记入 ARCH 实现记录，`script-src` 不能放宽。各空间可以在此基础上收紧，不能放宽：运维空间 `img-src 'self'`（不含 `data:`）、`style-src 'self'`，并加 `X-Frame-Options: DENY`（PRD-ops F-OPS-03 规则 4）。
5. 依赖安全：前端 `npm audit --audit-level=high` 阻断构建（与服务端 S11 一致）。
6. 每个空间的前端是独立入口（独立构建产物），各自的适配要求由各自 PRD 定义（管理后台：桌面，最小宽度 1280px）。

**验收标准**
- AC-CON-10-1【前端】：输入 构建产物 `dist/` 和一次完整操作流程（登录、搜索用户、强制下线、解散群、查审计）的 HAR；操作 搜索 IM 调用方 HMAC 密钥原文、`CON_IM_SIGNING_KEY` 私钥、`X-Admin-Key`、`X-IM-Signature`；预期 结果都为 0；错误码 —。
- AC-CON-10-2：输入 用户昵称、群名、消息元数据中的发送者昵称都为 `<img src=x onerror=alert(1)>`；操作 在用户列表、用户详情、群详情、消息元数据页、审计详情（理由也用该串）中查看；预期 原样显示字符串，不弹窗，浏览器没有对 `x` 的请求；错误码 —。
- AC-CON-10-3【后端】：输入 任一页面和任一接口；操作 检查响应头；预期 CSP 与规则 4 一致（含 `frame-ancestors 'none'`、`script-src 'self'` 不含 `unsafe-inline`），`nosniff`、`Referrer-Policy: no-referrer`、生产环境 HSTS `max-age=31536000` 都在；错误码 —。
- AC-CON-10-4【前端】：输入 在其他域名的页面中用 iframe 嵌入后台；操作 打开该页面；预期 浏览器拒绝加载后台；错误码 —。
- AC-CON-10-5【前端】：输入 用户 u1 的头像 URL 为 `https://tracker.example/a.png`；操作 打开用户列表和详情；预期 显示首字母占位，浏览器网络面板中没有对 `tracker.example` 的请求；错误码 —。
- AC-CON-10-6【前端】：输入 在任一组件中使用 `dangerouslySetInnerHTML`；操作 运行 `npm run lint`；预期 失败；错误码 —。

### F-CON-11 密钥材料与密码哈希参数（第一批，P0）

**规则**
1. `CON_DATA_KEYS`：数据加密密钥列表（每把 ≥ 32 字节随机，带 kid），`CON_DATA_ACTIVE_KID` 指定当前使用的一把。TOTP 种子用 AES-256-GCM 加密存储，记录 kid；轮换时新写入用新 kid，命令 `console-server keys rotate` 把旧记录重新加密，完成后才能从配置中删除旧 key。
2. 各类密钥（数据密钥、`CON_IM_SIGNING_KEY`、`CON_IM_MSGREAD_KEY`、数据库密码、归档公钥对应的私钥）互不相同；私钥类配置不能出现在日志、`/readyz` 和任何接口响应中。归档私钥**不放在 console-server 上**（只有恢复归档时由安全负责人线下使用）。
3. 恢复码、激活码、会话 ID 在库里只存哈希（恢复码、激活码用带服务端 pepper 的 HMAC-SHA256；会话 ID 用 SHA-256）。
4. Argon2id 参数可配置，默认 `m=64MiB, t=3, p=1`；全局并发信号量默认 4（同时最多 4 个哈希计算），排队超过 2 秒返回 503 / 5030（防内存型拒绝服务）。
5. 生产启动保护见 F-CON-01 规则 9：`CON_DATA_KEYS` 中任一把 < 32 字节、与其他密钥相同、`CON_DATA_ACTIVE_KID` 不存在时拒绝启动。

**验收标准**
- AC-CON-11-1【后端】：输入 账号绑定 TOTP 后；操作 读 `auth.totp` 表；预期 只有密文和 kid，用 Base32 种子原文搜索全库为 0；错误码 —。
- AC-CON-11-2【后端】：输入 新增 kid=k2 并设为当前，运行 `keys rotate`；操作 删除 k1 后重启并登录全部账号；预期 都能登录（全部已用 k2 重新加密）；未运行 rotate 就删除 k1 时启动失败并指出仍有 k1 加密的记录；错误码 —。
- AC-CON-11-3【后端】：输入 `CON_ENV=production`，`CON_DATA_KEYS` 中一把为 16 字节，或与 `CON_IM_SIGNING_KEY` 相同；操作 启动；预期 非 0 退出；错误码 —。
- AC-CON-11-4【后端】：输入 默认参数，并发 50 个登录请求；操作 观察进程内存与响应；预期 同时进行的 Argon2id 计算不超过 4 个，RSS 增量 ≤ 300MB，排队超过 2 秒的请求返回 503；错误码 5030。

### F-CON-12 安全通知（站内，第一批，P0）

**规则**
1. 顶栏通知中心（铃铛 + 未读数），通知按账号存储，保留 90 天；邮件 / IM 通知为 P1。
2. 触发：账号激活或重新绑定（通知本空间空间超管、审计员）；被重置（通知被重置者以外的空间超管、审计员）；被锁定（本人、空间超管）；恢复码被使用、剩余 ≤ 2 个（本人）；授予敏感角色（审计员）；应急授权（审计员、其他空间超管）；审计校验失败、锚定失败（空间超管、审计员）。各空间 PRD 可以追加触发条件（如 PRD-ops F-OPS-02 规则 5、F-OPS-08 规则 9），接收人按"空间 + 角色"指定；通知接口支持发给某空间某角色的全部启用账号。
3. 通知只说明"谁、什么事、何时"，不含密码、激活码、动态码等任何凭证。

**验收标准**
- AC-CON-12-1：输入 kf1 被锁定；操作 kf1 解锁后登录、超管登录；预期 两人的通知中心都有"kf1 于 {时间} 因多次登录失败被锁定"；错误码 —。
- AC-CON-12-2【后端】：输入 规则 2 的每一类触发（参数化）；操作 各触发一次；预期 对应接收人各收到 1 条通知，非接收人没有；通知内容中搜不到激活码、动态码；错误码 —。
- AC-CON-12-3【前端】：输入 有 3 条未读通知；操作 打开通知中心；预期 铃铛显示 3，打开后变为 0；错误码 —。

---

## 5. 管理后台的角色与权限（`admin` 空间）

### 5.1 角色

| 角色 | 典型用户 | 职责 | 敏感角色（授予时通知审计员） |
|---|---|---|---|
| `super_admin` 超级管理员 | 负责人（Frank）、技术负责人 | 本空间账号与角色管理；全员通知的批准；只读查看用户和群。**默认没有**正文、删除、审计权限，需要时用应急授权（F-CON-06 规则 6） | 是 |
| `operator` 运营 | 运营同学 | 用户导入、官方账号、好友导入与社交配置、建群与群管理、系统通知 | 否 |
| `support` 客服 | 客服 | 查用户、看在线设备（填理由）、强制下线、查消息元数据（单聊填理由）、禁言（第二批） | 否 |
| `moderator` 内容审核 | 审核专员 | 强制下线、解散违规群、元数据查询；第二批：群消息正文查看、删除消息、隔离原文查看、封禁、禁言 | 是 |
| `auditor` 审计员 | 安全 / 合规 | 只读：查看 `admin` 空间审计和 im-server 侧审计；不能与其他角色叠加 | 是 |

角色可以叠加（F-CON-06），例如一个人同时是 `support` + `operator`。自定义角色为 P1。

### 5.2 权限点矩阵

| 权限点 | 说明 | 批次 | 超管 | 运营 | 客服 | 审核 | 审计员 |
|---|---|---|---|---|---|---|---|
| `user.read` | 搜索用户、查看资料、官方标记、是否在线、封禁 / 禁言状态 | 一 | ✅ | ✅ | ✅ | ✅ | |
| `user.import` | 导入 / 更新用户 | 一 | | ✅ | | | |
| `user.kick` | 强制下线 | 一 | | | ✅ | ✅ | |
| `user.sessions` | 查看在线设备明细（填理由） | 一 | | | ✅ | ✅ | |
| `user.official` | 标记 / 取消官方账号 | 一 | | ✅ | | | |
| `social.read` | 查看某用户的好友列表（不含备注）、查看社交配置 | 一 | ✅ | ✅ | ✅ | ✅ | |
| `social.write` | 导入 / 删除好友关系 | 一 | | ✅ | | | |
| `social.settings` | 修改好友模式与社交限额 | 一 | | ✅ | | | |
| `group.read` | 群列表、详情、成员 | 一 | ✅ | ✅ | ✅ | ✅ | |
| `group.write` | 建群、加 / 移成员、改群名 / `historyVisible` / 公告 | 一 | | ✅ | | | |
| `group.owner` | 指定 / 收回群主（M4） | 一 | | ✅ | | | |
| `group.dissolve` | 解散群 | 一 | | ✅ | | ✅ | |
| `msg.meta` | 查看会话消息元数据（时间、发送者、seq、类型、长度），**不含正文**；单聊需填理由 | 一 | | | ✅ | ✅ | |
| `msg.read.group` | 群消息正文查看（F-ADM-08） | 二 | | | | ✅ | |
| `msg.read.c2c` | 单聊正文查看；**im-server 开关关闭期间无效** | 二 | | | | ✅ | |
| `msg.delete` | 删除违规消息 | 二 | | | | ✅ | |
| `msg.quarantine.read` | 查看被删消息的隔离原文 | 二 | | | | ✅ | |
| `user.ban` | 封禁 / 解封 | 二 | | | | ✅ | |
| `mute.write` | 用户级 / 群成员禁言与解除 | 二 | | | ✅ | ✅ | |
| `push.user` | 按用户发系统通知 | 二 | | ✅ | | | |
| `push.all.request` | 发起全员通知（草稿） | 二 | | ✅ | | | |
| `push.all.approve` | 批准全员通知（不能批准自己发起的） | 二 | ✅ | | | | |
| `push.records.read` | 查看系统通知发送记录全文（F-ADM-50 规则 7） | 二 | ✅ | | | | ✅ |
| `audit.read` | 查看审计日志（`admin` 空间 + im-server 侧） | 一 | | | | | ✅ |
| `admin.manage` | 本空间账号与角色管理、解锁 | 一 | ✅ | | | | |
| `admin.emergency` | 应急授权（只能给自己，F-CON-06 规则 6） | 一 | ✅ | | | | |
| `stats.read` | 数据概览 | P1 | ✅ | ✅ | | | |
| `word.write` | 敏感词维护 | P2 | | | | ✅ | |

### 5.3 需要理由、step-up、确认文字的操作

| 操作 | 理由 | step-up（5 分钟窗口） | 每次输入动态码 | 手动输入确认文字 |
|---|---|---|---|---|
| 强制下线、在线设备查看、单聊元数据查询、官方账号标记 / 取消、群主交接、移出群成员 | ✅ | | | |
| 好友导入 / 删除、社交配置修改 | ✅ | ✅ | | 切换好友模式时输入新模式名 `friends` / `open` |
| 解散群 | ✅ | ✅ | | 群 ID |
| 封禁 / 解封、删除消息 | ✅ | ✅ | | |
| 禁言 / 解除禁言 | ✅ | | | |
| 群消息正文查看、隔离原文查看 | ✅（+ 工单号） | | ✅ | |
| 发起全员通知 | ✅ | | | |
| 批准全员通知 | | ✅ | | "全员发送" |
| 账号管理、应急授权、重新生成恢复码 | 应急授权需要 | ✅ | | |

所有写操作都先弹确认框，写清对象和影响（例如"将强制 u1 的 3 个在线设备下线"）。理由、step-up 的要求在 console-server 校验，前端只负责弹框。

### F-ADM-05 管理后台角色与权限矩阵（第一批，P0）

**规则**：按 5.2、5.3 节执行；机制见 F-CON-06、F-CON-07。

**验收标准**
- AC-ADM-05-1【后端】：输入 5.2 节矩阵（第一批的行）× 5 个单角色账号，每个路由一个合法请求样本；操作 参数化测试逐格调用；预期 ✅ 的格返回该接口的成功状态码，空格返回 403；错误码 4030。
- AC-ADM-05-2【后端】：输入 5.3 节表中每一类"需要理由"的操作；操作 分别不带理由、理由为 3 个字符、理由为"aaaa"调用；预期 都 400，操作未执行；错误码 4000。
- AC-ADM-05-3【前端】：输入 只有 `support` 的账号；操作 登录后查看侧边栏，并在地址栏输入 `/accounts`、`/audit`、`/social/settings`；预期 侧边栏没有"账号管理""审计日志""社交配置"；直接访问显示"无权限"页；错误码 —。
- AC-ADM-05-4【后端】：输入 `super_admin`（无应急授权）；操作 调用元数据查询、审计查询；预期 都 403；错误码 4030。
- AC-ADM-05-5【前端】：输入 账号同时有 `support`、`operator`；操作 登录；预期 侧边栏显示两种角色菜单的并集，顶栏显示两个角色名；错误码 —。

---

## 6. 功能需求：管理后台业务

> 【后端】指 console-server 或 im-server；【前端】指管理后台 H5；未标注的为端到端。接口中 `POST /console/api/...` 均受 CSRF 保护（F-CON-05 规则 7）。im-server 侧接口见第 8 节。

### 6.1 用户

#### F-ADM-10 用户搜索与详情（第一批，P0）

**规则**
1. 搜索：按 userId 精确 / 前缀、昵称前缀（规则见 2.3 节）；可筛选"只看官方账号"。列表显示 userId、昵称、头像占位、注册时间、是否在线（是 / 否，不含设备明细）、官方标记；第二批增加"已封禁""禁言中"标记和到期时间。
2. 详情：资料、官方标记、是否在线；好友列表入口（F-ADM-15）；"查看在线设备"按钮（F-ADM-14）；第二批增加封禁状态（到期时间、理由、操作人）、用户级禁言状态；所在群（P1，G-ADM-07）。
3. 是否在线来自 im-server 的在线路由（单节点为本机 hub，集群为 Presence），console-server 不缓存。
4. 浏览列表和详情不写审计（不含设备明细和关系数据）。

**验收标准**
- AC-ADM-10-1：输入 存在 `u100`、`u101`、`u2`、`U100`；操作 搜索 `u10`；预期 返回 `u100`、`u101`，不返回 `u2`、`U100`（userId 区分大小写）；错误码 —。
- AC-ADM-10-2：输入 昵称为"小明""小明明""Tom""tommy"的用户；操作 搜索"小明"、"明"、"tom"；预期 "小明"返回前两个；"明"不返回任何人（前缀匹配）；"tom"返回 Tom 和 tommy（ASCII 不区分大小写）；错误码 —。
- AC-ADM-10-3【后端】：输入 userId `u_1` 和 `uX1`；操作 搜索 `u_`；预期 只返回 `u_1`（`_` 已转义，不作通配符）；错误码 —。
- AC-ADM-10-4：输入 u1 用 web、pc 各连一次；操作 打开 u1 详情；预期 显示"在线"；两个连接都断开后刷新显示"离线"；错误码 —。
- AC-ADM-10-5【前端】：输入 搜索框内容为空白；操作 点击搜索；预期 不发请求，提示"请输入关键字"；搜索不存在的 userId 显示"没有找到用户"；错误码 —。
- AC-ADM-10-6【后端】：输入 1 个官方账号 kefu 和 10 个普通用户；操作 勾选"只看官方账号"搜索；预期 只返回 kefu；错误码 —。
- AC-ADM-10-7【后端】：输入 1000 个 userId 以 `p` 开头的用户；操作 `limit=100` 连续翻 10 页；预期 共 1000 个，无重复、无遗漏；错误码 —。

#### F-ADM-11 用户导入（第一批，P0）

**规则**
1. 两种方式：表单单个添加；上传 CSV（列 `userId,nickname,avatar`，UTF-8，有表头），单文件 ≤ 5000 行。
2. **由 console-server 整体校验**（规则同 F-USER-01：userId 格式、昵称 ≤ 32、头像 ≤ 512；文件内重复以最后一条为准），任一行不合法则整个文件不提交，列出出错行号和原因（最多前 100 条）。
3. 校验通过后预览前 20 行和总行数；确认后由 console-server 按每批 500 调用 `POST /admin/users/import`；审计：开始前一条 intent（总行数、文件 SHA-256），结束后一条 result（成功批数 / 总批数、失败错误码）；im-server 侧每批一条（`X-Request-Id` 为 `{requestId}.{批号}`）。
4. 中途某批失败：停止后续批次，显示"已成功 N 批 / 共 M 批"；导入是 upsert，可以整文件重新提交。
5. 已存在的 userId 会被更新昵称和头像；预览中标明"将更新 K 个已有用户"为 P1（依赖批量查询）。

**验收标准**
- AC-ADM-11-1：输入 1200 行合法 CSV；操作 上传、确认；预期 显示成功 1200；im-server 收到 3 次导入调用（500 + 500 + 200），审计有 1 条 intent、1 条 result；错误码 —。
- AC-ADM-11-2：输入 第 7 行 userId 含空格、第 30 行昵称 33 字；操作 上传；预期 列出两条错误（行号 + 原因），"确认"按钮不可点，im-server 未收到调用；错误码 4004、4000（行内原因对应的码）。
- AC-ADM-11-3：输入 5001 行的 CSV；操作 上传；预期 提示"单个文件最多 5000 行"，不提交；错误码 4131。
- AC-ADM-11-4【后端】：输入 1200 行，测试让 im-server 第 2 批返回 500 / 5000；操作 确认导入；预期 页面显示"已成功 1 批 / 共 3 批"，第 3 批未调用；审计 result 记录成功批数 1、失败码 5000；错误码 5000。
- AC-ADM-11-5【后端】：输入 `support` 账号；操作 调用导入接口；预期 403；错误码 4030。

#### F-ADM-12 强制下线（第一批，P0）

**规则**：调用现有 `POST /admin/users/{userId}/force-logout`，语义不变（踢掉全部连接、此前签发的 token 全部作废，业务方重新签发即可恢复）。必须填理由；确认框显示"是否在线"。页面提示"业务方重新签发凭证后用户即可再次登录；要阻止登录请使用封禁（第二批）"。

**验收标准**
- AC-ADM-12-1：输入 u1 有 2 个在线连接；操作 客服填理由后强制下线；预期 两个连接都收到 `kicked {reason:"banned"}` 并以 4012 关闭；详情页变为"离线"；错误码 —。
- AC-ADM-12-2：输入 强制下线后的 u1；操作 业务方重新签发 token 并连接；预期 连接成功；错误码 —。
- AC-ADM-12-3【后端】：输入 不存在的 userId `nobody`；操作 强制下线；预期 页面提示"用户不存在"，审计 result `failure`、`code=4042`；错误码 4042。
- AC-ADM-12-4【后端】：输入 不带理由的强制下线请求；操作 调用；预期 400，u1 未被踢；错误码 4000。

#### F-ADM-14 在线设备查看（第一批，P0）

**规则**
1. 需要 `user.sessions` + 理由；请求 `POST /console/api/users/sessions/query {userId, reason, ticket?}`（userId、理由不进 URL）。
2. 返回每个连接：连接 ID、平台、建立时间、节点、脱敏 IP、下行队列状态（管理后台可以不显示队列状态）。脱敏：IPv4 保留前 2 段（`a.b.*.*`），IPv6 保留前 3 组（48 位），其余为 `*`；**im-server 的连接元数据和断开缓冲只保存脱敏后的值**，查询函数直接返回脱敏值，两个后台都拿不到完整 IP。字段与脱敏规则和运维后台的"当前连接"一致：im-server 只有一个查询函数，管理接口 `POST /admin/users/sessions/query` 和运维节点接口复用它（评审 X-05）。
3. 每次查询写审计（`user.sessions.query`，含理由、结果条数）。
4. 每个账号每分钟最多 30 次（4290），每天（Asia/Shanghai 自然日，2.3 节）最多查询 50 个不同 userId（429 / 4292），防止长期跟踪某人在线规律。
5. B6 之前（没有 Presence）管理接口只返回接收请求的那个节点上的连接，响应标 `partial: true`、`scope: "localNode"`；B6 之后由 im-server 经 Presence 汇总。集群下某个节点未响应时，不显示"不在线"，显示"在已响应的 N 个节点上找到 K 个连接；节点 X 未响应，结果可能不完整"。

**验收标准**
- AC-ADM-14-1：输入 u1 用 web、pc 各连一次，来源 IP 为 `10.1.2.3`；操作 客服填理由查询；预期 返回 2 条，平台分别为 web、pc，IP 显示为 `10.1.*.*`；审计有 `user.sessions.query`，理由一致，`params.count=2`；错误码 —。
- AC-ADM-14-2【后端】：输入 不带理由的查询；操作 调用；预期 400，没有返回任何连接数据；错误码 4000。
- AC-ADM-14-3【后端】：输入 `operator` 账号；操作 查询；预期 403；错误码 4030。
- AC-ADM-14-4【后端】：输入 同一客服账号当天已查询 50 个不同 userId；操作 查询第 51 个；预期 429，无数据返回；查询已查过的 userId 仍成功；错误码 4292。
- AC-ADM-14-5【后端】：输入 查询请求；操作 检查 console-server 访问日志和浏览器历史；预期 URL 中没有 userId 和理由；错误码 —。
- AC-ADM-14-6【后端】[依赖 B6]：输入 u1 只连在节点 B，节点 B 暂停响应；操作 查询 u1；预期 响应标记 `partial=true` 和未响应节点，页面不出现"当前不在线"；错误码 —。

#### F-ADM-13 封禁 / 解封（第二批，P0，PRD-core v1.3 变更）

**描述**：与强制下线的区别：封禁期间**无法再签发 token**，业务方也绕不过去（X-8：IM 和业务方都可以封禁）。

**规则**
1. 需要 `user.ban` + 理由 + step-up。时长：1 小时、1 天、7 天、30 天、永久。
2. 立即生效：踢掉全部在线连接，下发 `kicked {reason: "suspended", until}`（`until` 为毫秒，永久为 `null`），关闭码 4012；此前签发的 token 作废。老版本 SDK 不认识 `suspended` 时按关闭码 4012 处理（不重连，"账号已被下线"）。
3. 封禁期间 `POST /admin/users/token` 和 `/demo/login` 返回 **403 / 4034 USER_BANNED**，`details.bannedUntil`（永久为 `null`）。业务方据此提示用户。
4. 到期自动解封（签发时按 [IM 钟] 当前时间判断，不需要定时任务）；手动解封立即生效；解封后需要业务方重新签发 token。
5. 不改变用户资料、好友关系、群成员关系、历史消息。重复封禁以最后一次为准，审计里每次都有。
6. 管理接口同时开放给业务方：`POST /admin/users/{userId}/ban {durationSeconds | null, reason}`、`POST /admin/users/{userId}/unban {reason}`（scope `user.ban`）；`GET /admin/users/{userId}` 增加 `ban: {until, reason, by, at} | null`。`by` 为 `{caller}:{operator}`。
7. 对不存在的用户封禁返回 404 / 4042。

**验收标准**
- AC-ADM-13-1：输入 u1 有 2 个在线连接；操作 审核员填理由、step-up 后封禁 1 天；预期 两个连接收到 `kicked {reason:"suspended", until: 封禁时间+1 天}` 并以 4012 关闭；错误码 —。
- AC-ADM-13-2【后端】[IM 钟]：输入 u1 封禁 1 天；操作 业务方签发 u1 的 token；预期 403，`details.bannedUntil` = 封禁时间 + 1 天（±5 秒）；永久封禁时为 `null`；错误码 4034。
- AC-ADM-13-3【后端】：输入 封禁前签发、未过期的 token；操作 用它连接；预期 收到 `kicked` 并以 4012 关闭；错误码 —。
- AC-ADM-13-4【后端】[IM 钟]：输入 封禁 1 小时；操作 推进 61 分钟后签发 token 并连接；预期 都成功；错误码 —。
- AC-ADM-13-5：输入 封禁中的 u1；操作 解封；预期 立即可以签发 token；详情页封禁状态消失；审计有 `user.unban`；错误码 —。
- AC-ADM-13-6【后端】：输入 u1 封禁前在 g1、与 u2 是好友、有历史消息；操作 封禁后解封并重新登录；预期 仍在 g1 成员列表中、仍与 u2 互为好友、能看到封禁前的历史；错误码 —。
- AC-ADM-13-7【后端】：输入 `IM_DEMO_LOGIN=1`，u1 被封禁；操作 `POST /demo/login {userId:"u1"}`；预期 403，`details.bannedUntil` 正确；错误码 4034。
- AC-ADM-13-8【后端】：输入 不存在的用户 `nobody`；操作 封禁；预期 404；错误码 4042。
- AC-ADM-13-9【前端】：输入 `support` 账号打开用户详情；操作 查看操作按钮；预期 没有"封禁"按钮；用户列表中被封禁的用户显示"已封禁"和到期时间；错误码 —。
- AC-ADM-13-10【后端】：输入 未 step-up 的审核员会话；操作 封禁；预期 403，u1 未被封禁；错误码 4038。
- AC-ADM-13-11【前端】（sdk-ts）：输入 H5 已登录的 u1；操作 后台封禁 u1；预期 H5 显示"账号已被封禁，解封时间：{时间}"（永久显示"账号已被封禁"）后回到登录页，不自动重连；错误码 —。

### 6.2 社交关系管理（第一批；im-server 接口由 PRD-social M3 实现，S-11 ②：F-KEY 上线后才注册）

#### F-ADM-15 好友关系管理（第一批，P0）

**规则**
1. 查询：用户详情 → 好友列表（`GET /admin/users/{userId}/friends`，分页 100），显示对方 userId、来源（`app` / `admin`）、建立时间；**不显示备注**。需要 `social.read`。浏览不写审计。
2. 导入：表单（单对）或 CSV（列 `a,b,remarkA,remarkB`，后两列可空，≤ 5000 行）；console-server 先整体校验（userId 格式、`a ≠ b`、文件内重复以最后一条为准），再按每批 500 对调用 `POST /admin/friends/import`；**im-server 每批整批原子**（PRD-social F-SOC-12 规则 3），批之间不原子：中途失败停止后续批次，显示"已成功 N 批 / 共 M 批"；导入幂等，可以整文件重新提交。
3. 删除：粘贴或 CSV（列 `a,b`，≤ 5000 行），按每批 500 对调用 `DELETE /admin/friends`。
4. 导入、删除需要 `social.write` + 理由 + step-up；确认框写明"将为 N 对用户建立双向好友关系，不受拉黑影响，也不解除拉黑"。
5. 审计：一条 intent（对数、文件 SHA-256）+ 一条 result；**审计和日志中不出现备注原文**。错误码原样透传（4042 带 userId、4132 带 userId、4131）。

**验收标准**
- AC-ADM-15-1：输入 CSV 两行 `alice,bob,B,` 与 `alice,carol,,`；操作 运营填理由、step-up 后导入；预期 显示成功 2 对；alice 的好友列表出现 bob、carol（来源 `admin`），列表中没有备注列；错误码 —。
- AC-ADM-15-2：输入 1200 对的 CSV，其中第 2 批含不存在的用户 `ghost`；操作 导入；预期 第 1 批成功，第 2 批整批失败，页面显示"已成功 1 批 / 共 3 批"和"用户不存在：ghost"，第 3 批未调用；错误码 4042。
- AC-ADM-15-3【后端】：输入 备注列为特征串 `ZXQ-REMARK-789`；操作 导入后扫描 console-server 日志和审计；预期 搜不到特征串；错误码 —。
- AC-ADM-15-4【后端】：输入 alice 已有 1999 个好友；操作 导入含 alice 的 2 对；预期 页面提示"alice 的好友数已达上限"，这一批都没导入；错误码 4132。
- AC-ADM-15-5：输入 alice、bob 互为好友，好友模式为 `friends`；操作 运营删除这一对；预期 两侧都没有对方；alice 给 bob 发私聊返回 4035；错误码 —。
- AC-ADM-15-6【后端】：输入 CSV 中有一行 `a == b`；操作 上传；预期 校验阶段列出该行错误，不发任何调用；错误码 4000。
- AC-ADM-15-7【后端】：输入 F-KEY 未上线、`IM_TEST_MODE` 关闭的 im-server；操作 console-server 调用好友导入；预期 im-server 返回 404，console 页面提示"该功能尚未在服务端开放"；错误码 —（404 未注册）。

#### F-ADM-16 好友模式与社交配置（第一批，P0）

**规则**
1. 页面显示 PRD-social 5.1 节的全部参数和当前值（`GET /admin/settings/social`，需要 `social.read`）。
2. 修改：需要 `social.settings` + 理由 + step-up；提交前显示"旧值 → 新值"差异；**切换好友模式时额外要求手动输入新模式名**，并显示影响说明："切到好友制后，非好友之间不能再发私聊（历史会话保留）；官方账号不受影响"。
3. 调用 `PATCH /admin/settings/social`，只传修改了的字段；非法值由 im-server 返回 400 / 4000，整次不生效。
4. 审计：旧值 → 新值（im-server 侧 `admin_audit` 同样记录）。

**验收标准**
- AC-ADM-16-1：输入 当前 `mode=open`；操作 运营把模式改为 `friends`（输入确认文字、理由、step-up）；预期 保存成功，页面显示 `friends`；1 秒内非好友私聊返回 4035；审计 `params` 为 `mode: open → friends`；错误码 —。
- AC-ADM-16-2【前端】：输入 切换模式的弹窗；操作 确认文字输入 `friend`；预期 "确认"按钮不可点；错误码 —。
- AC-ADM-16-3【后端】：输入 `strangerMsgLimit=-1`；操作 保存；预期 400，配置不变；错误码 4000。
- AC-ADM-16-4【后端】：输入 `support` 账号；操作 读取配置、修改配置；预期 读取成功；修改 403；错误码 4030。
- AC-ADM-16-5【后端】：输入 只修改 `maxFriends`；操作 抓取 console-server 发给 im-server 的请求；预期 PATCH 体中只有 `maxFriends`；错误码 —。

#### F-ADM-17 官方账号标记（第一批，P0）

**规则**
1. 用户详情页"设为官方账号 / 取消官方账号"，需要 `user.official` + 理由；调用 `PUT /admin/users/{userId}/official`。
2. 确认框说明："官方账号不受好友制和陌生人限额限制，用户仍可以拉黑它"（PRD-social F-SOC-13 规则 2、3）。
3. 用户搜索可筛选"只看官方账号"（F-ADM-10 规则 1）。

**验收标准**
- AC-ADM-17-1：输入 好友模式 `friends`，kefu 与 alice 不是好友；操作 运营把 kefu 设为官方账号；预期 kefu 详情显示"官方"；kefu 给 alice 发私聊成功；审计有 `user.official.set`，理由一致；错误码 —。
- AC-ADM-17-2：输入 官方账号 kefu；操作 取消官方标记；预期 1 秒内 kefu 给非好友 alice 发私聊返回 4035；错误码 —。
- AC-ADM-17-3【后端】：输入 不存在的用户；操作 设为官方账号；预期 页面提示"用户不存在"；错误码 4042。
- AC-ADM-17-4【后端】：输入 `support` 账号；操作 调用设置接口；预期 403；错误码 4030。

### 6.3 群

#### F-ADM-20 群列表与搜索（第一批，P0）

**规则**：按 groupId 精确 / 前缀、群名前缀搜索；列表显示 groupId、群名、类型（自建 / 托管，M4 之前全部为托管）、群主（M4）、成员数、`historyVisible`、创建时间；默认按创建时间倒序；默认不含已解散的群，勾选"包含已解散"后显示并标记"已解散"和解散时间。

**验收标准**
- AC-ADM-20-1：输入 存在 `g-match-1`、`g-match-2`、`g-other`；操作 搜索 `g-match`；预期 返回前两个；错误码 —。
- AC-ADM-20-2：输入 群 g1 有 37 个成员；操作 查看列表；预期 成员数列为 37，与群详情一致；错误码 —。
- AC-ADM-20-3：输入 已解散的群 g9；操作 默认列表和勾选"包含已解散"各查一次；预期 默认不出现；勾选后出现并标记"已解散"；错误码 —。
- AC-ADM-20-4【后端】（M4）：输入 自建群 `ug-…`（群主 alice）和托管群 g1；操作 查看列表；预期 自建群类型为"自建"、群主 alice；g1 类型为"托管"、群主为空；错误码 —。

#### F-ADM-21 建群（第一批，P0）

**规则**：调用 `POST /admin/groups`，规则同 ADMIN.md（群名 1～50、初始成员 ≤ 2000、成员不存在整批不生效）；群 ID 不能以 `ug-` 开头（4004，PRD-social V-09）。初始成员可以手填（逗号 / 换行分隔）或上传 CSV；`historyVisible` 默认关，表单上说明含义。M4 起可选填群主（F-ADM-25）。

**验收标准**
- AC-ADM-21-1：输入 群 ID `g-event-1`、群名"活动群"、3 个已存在成员；操作 创建；预期 成功；3 个在线成员收到 `convAdded`；错误码 —。
- AC-ADM-21-2：输入 已存在（或已解散）的群 ID；操作 创建；预期 提示"群 ID 已存在"；错误码 4090。
- AC-ADM-21-3：输入 成员中有不存在的用户 `ghost`；操作 创建；预期 提示"用户不存在：ghost"，群未创建；错误码 4042。
- AC-ADM-21-4【前端】：输入 粘贴 2001 个成员；操作 提交；预期 提交前提示"群成员最多 2000 人"，不发请求；错误码 —。
- AC-ADM-21-5【后端】：输入 群 ID `ug-abc`；操作 创建；预期 提示"群 ID 格式不正确"；错误码 4004。

#### F-ADM-22 群详情与成员管理（第一批，P0）

**规则**
1. 详情：资料、设置、成员列表（分页，每页 50，按 userId 前缀筛选；G-ADM-10，与客户端 `groupMembers` 共用存储查询）、每个成员的入群时间；M4 增加角色列（群主 / 管理员 / 成员）和群公告；第二批增加群内禁言状态。
2. 加成员、移除成员：手填或粘贴，**由 console-server 拆批**（每批 500）：先用当前成员数一次性检查"加完是否超过 2000"，超过直接返回 4130、一个都不加；再逐批调用；一条 intent + 一条 result 汇总审计；移除需要理由。
3. 移除成员时，第二批的群内禁言记录一并清除（F-ADM-41 规则 4）。
4. 管理接口加减成员不产生群提示消息（PRD-social V-08）。

**验收标准**
- AC-ADM-22-1：输入 1500 人的群；操作 成员列表逐页翻到底；预期 每页 50 人，共 30 页、1500 人，无重复；错误码 —。
- AC-ADM-22-2：输入 群 g1；操作 加入 3 人；预期 在线的新成员收到 `convAdded`；成员数 +3；g1 的 maxSeq 不变；错误码 —。
- AC-ADM-22-3：输入 g1 成员 u5；操作 填理由移除 u5；预期 u5 收到 `convRemoved {reason:"removed"}`，之后 u5 在 g1 发言返回 4033；错误码 —。
- AC-ADM-22-4【后端】：输入 g1 有 500 人；操作 一次粘贴 1200 人加入；预期 console-server 分 3 批调用并全部成功，成员数 1700；审计只有 1 条 intent、1 条 result；错误码 —。
- AC-ADM-22-5【后端】：输入 g1 已有 1999 人；操作 加入 2 人；预期 提示"群成员最多 2000 人"，一个都没加，im-server 未收到加成员调用；错误码 4130。
- AC-ADM-22-6：输入 g1 成员 `u10`、`u11`、`u2`；操作 成员筛选输入 `u1`；预期 只显示 `u10`、`u11`；错误码 —。

#### F-ADM-23 群设置（第一批，P0）

**规则**：修改群名、`historyVisible`（调用现有 `PATCH`）；M4 起可改群公告（`notice`）。修改 `historyVisible` 的确认框说明"只影响之后的查询：打开后，所有成员（包括已入群的）都能看到入群前的历史"。

**验收标准**
- AC-ADM-23-1：输入 群 g1；操作 群名改为"新名字"；预期 成员下次拉取 `convs` 显示新名字；错误码 —。
- AC-ADM-23-2：输入 `historyVisible=false` 的 g1，成员 d 入群前有 10 条消息；操作 改为 true；预期 d 的 `sync` 能拉到入群前的 seq，d 的未读数不变（C-02）；错误码 —。
- AC-ADM-23-3【前端】：输入 群名清空或 51 个字；操作 编辑；预期 保存按钮不可点并提示；错误码 —。

#### F-ADM-24 解散群（第一批，P0）

**规则**
1. 需要 `group.dissolve` + 理由 + step-up + 手动输入群 ID 确认。调用 `DELETE /admin/groups/{groupId}`（scope `group.dissolve`）。
2. 与 PRD-social F-SOC-26 **共用存储方法 `DissolveGroup`**：一个事务内删除该群全部 `group_members` 行、`chat_groups` 写 `dissolved_at`，同一流程中发 `TopicGroupMembers` 失效；消息数据保留。管理接口可以解散任何群（托管群、自建群）。
3. 效果：在线成员收到 `convRemoved {convId, reason: "dissolved"}`；群从所有成员的 `convs`（以及 M3 的 `myGroups`）中消失；对该群的 `send` / `sync` / `read` 返回 4043；管理接口查询、加成员、改设置返回 404 / 4043。
4. **群 ID 不能复用**：用已解散群的 ID 建群返回 409 / 4090。
5. 不生成提示消息；不提供"恢复群"（P2 再评估）。
6. 解散后，后台仍能按会话查询该群的**元数据**（F-ADM-30，`msg.meta`）；正文按 F-ADM-08（第二批）。消息保留期（ARCH S13）未实现前为永久保存，列入第 13.2 节风险。
7. **客户端**（第一批协议变更）：`convRemoved` 增加可选 `reason`（`removed` / `dissolved`，M4 由 PRD-social 增加 `left`）；SDK 收到 `dissolved` 时从会话列表移除该群并提示"群聊已解散"；收到不认识的 `reason` 按 `removed` 处理。sdk-ts 只在内存；sdk-kotlin / sdk-swift（M5 / M6 首版）同时删除本地库中的该会话。

**验收标准**
- AC-ADM-24-1：输入 群 g1 有 3 个在线成员；操作 运营填理由、step-up、输入 `g1` 确认后解散；预期 3 人 1 秒内收到 `convRemoved {reason:"dissolved"}`；H5 刷新后会话列表中没有 g1；错误码 —。
- AC-ADM-24-2【后端】：输入 已解散的 g1；操作 成员 `send`、`sync`、`read`，管理接口 `GET /admin/groups/g1`；预期 前三个返回 4043；查询返回 404；错误码 4043。
- AC-ADM-24-3【后端】：输入 已解散的 g1；操作 `POST /admin/groups {groupId:"g1"}`；预期 409；错误码 4090。
- AC-ADM-24-4【后端】：输入 已解散的 g1 有 20 条消息；操作 客服按会话 `group:g1` 查询元数据；预期 返回 20 条元数据；错误码 —。
- AC-ADM-24-5：输入 解散时离线的成员 u3；操作 u3 上线（H5）；预期 `convs` 中没有 g1，H5 会话列表没有 g1；错误码 —。
- AC-ADM-24-6【前端】：输入 解散弹窗；操作 输入的群 ID 与目标不一致；预期 "确认"按钮不可点；错误码 —。
- AC-ADM-24-7【后端】：输入 不带理由，或未 step-up；操作 解散 g1；预期 分别返回 400 / 4000、403 / 4038，g1 未解散；错误码 4000、4038。
- AC-ADM-24-8【后端】：输入 存储契约测试；操作 解散后检查 PRD-social F-SOC-26 规则 3 列出的每个查询；预期 每个查询各一条断言通过（`GroupDissolveNoReuse`）；错误码 —。
- AC-ADM-24-9【前端】（sdk-ts）：输入 H5 收到 `convRemoved {reason:"dissolved"}`；操作 观察界面；预期 会话从列表移除，若正打开该群聊天页，提示"群聊已解散"并禁用输入框；收到 `reason:"left"`（未知值）时按"你已不在该群"处理；错误码 —。

#### F-ADM-25 指定 / 收回群主（第一批，随 M4 交付，P0）

**规则**：群详情"设置群主 / 收回群主"，需要 `group.owner` + 理由；调用 `PATCH /admin/groups/{groupId} {owner}`（scope `social.settings`，PRD-social F-SOC-30 规则 2）；新群主必须是成员，否则 404 / 4046；收回后群变为无群主的托管群；会生成 `from=""` 的 `ownerChanged` 提示消息（PRD-social 规则）。设置管理员为 P1。

**验收标准**
- AC-ADM-25-1：输入 托管群 g3，成员 bob；操作 运营设 bob 为群主；预期 群详情群主为 bob；bob 在客户端可以修改群名；审计有 `group.owner.set`；错误码 —。
- AC-ADM-25-2【后端】：输入 g3 群主 bob；操作 收回群主；预期 群详情群主为空、类型"托管"；bob 改群名返回 4030；错误码 —。
- AC-ADM-25-3【后端】：输入 非成员 eve；操作 设为群主；预期 页面提示"该用户不是群成员"；错误码 4046。

### 6.4 消息

#### F-ADM-30 会话消息元数据查询（第一批，P0）

**规则**
1. 入口：① 输入会话 ID（`c2c:{a}:{b}` 或 `group:{gid}`）；② 从群详情进入；③ 从用户详情 → 该用户的会话列表（P1，G-ADM-13）。
2. 只返回元数据：seq、msgId、发送者、类型、正文长度（字符数）、时间；第二批增加"已删除"标记、删除人、删除时间。**接口不返回正文字段**（不是前端隐藏）。
3. **单聊会话需要理由**（单聊双方和聊天时间属于社交关系信息）：`POST /console/api/messages/meta/query {convId, reason, ...}`，每次进入一个单聊会话写一条 `msg.meta.query` 审计；翻页不再要求理由（同一会话 15 分钟内）。每个账号每天最多查询 50 个不同的单聊会话（4292）。群会话不要求理由，但每次进入写一条审计（不含理由）。
4. 按 seq 倒序分页（每页 50），可按时间范围、发送者筛选；不受 `historyVisible` 和成员关系限制；包括已解散的群。
5. 会话 ID 格式错误返回 400 / 4007（4007 扩展 `admin` / `console` 通道，`http: 400`）；会话不存在返回空列表（与"存在但没有消息"相同，不暴露两个人是否聊过以外的信息）。
6. 一期不支持跨会话全文搜索（P2）。

**验收标准**
- AC-ADM-30-1：输入 群 g1 有 120 条消息；操作 客服查询 `group:g1`；预期 第 1 页 seq 120～71，第 3 页 20～1，之后显示"没有更多了"；每条只有元数据；审计有 1 条 `msg.meta.query`；错误码 —。
- AC-ADM-30-2：输入 g1 中 u2 发了 7 条；操作 按发送者 u2 筛选；预期 只出现这 7 条；错误码 —。
- AC-ADM-30-3【后端】：输入 会话 ID `c2c:a`（格式错误）；操作 查询；预期 400；错误码 4007。
- AC-ADM-30-4【后端】：输入 从未聊过的 `c2c:x:y`；操作 带理由查询；预期 200，空列表；错误码 —。
- AC-ADM-30-5【后端】：输入 单聊 `c2c:a:b`；操作 不带理由查询；预期 400，无数据；带理由后成功，审计含理由；错误码 4000。
- AC-ADM-30-6【后端】：输入 客服账号当天已查 50 个不同单聊；操作 查第 51 个；预期 429；错误码 4292。
- AC-ADM-30-7【后端】：输入 含特征串 `ZXQ-BODY-123` 的消息；操作 以客服身份查询元数据，并抓取 console-server ← im-server 的响应；预期 两层响应中都找不到特征串，也没有 `body` 字段；错误码 —。
- AC-ADM-30-8【后端】：输入 `operator` 账号；操作 查询元数据；预期 403；错误码 4030。

#### F-ADM-08 群消息正文查看（第二批，P0）

**规则**
1. **单聊正文**：法务书面确认前不可查看（X-7 默认 ①）。im-server 开关 `IM_C2C_BODY_ACCESS=off`（默认），任何凭证都读不到（403 / 4030，`reason: c2cBodyDisabled`）；后台页面显示"单聊正文查看未开启"。打开开关的前提见第 12 节。
2. **群消息正文**（X-7 ②）：需要 `msg.read.group`（`moderator`），每次申请一个会话的查看授权都要：**工单号 + 理由 + 当次动态码**（不享受 step-up 5 分钟窗口，F-CON-07 规则 3）。
3. 授权流程：console-server 校验动态码 → 写 intent → 用 `console-msgread` 凭证调用 im-server `POST /admin/msg-grants {convId, operator, reason, ticket}` → im-server 检查配额、写 `admin_audit`、返回 `grantId`（有效 15 分钟，[IM 钟]）→ console-server 把授权绑定到**当前会话 + 该会话 ID**。
4. 读取：`POST /console/api/messages/body {convId, beforeSeq, limit≤50}`，console-server 带 `grantId` 调用 im-server `POST /admin/convs/messages/body`。没有该会话的有效授权（未申请、已过期、换了会话、换了浏览器会话）统一返回 **403 / 4039**。每加载一页写一条 `msg.read.page` 审计（seq 范围、条数，不含正文）。
5. **配额（im-server 侧强制，console 被攻破也绕不过）**：每个操作人每天最多 20 个授权，`console-msgread` 调用方每天最多 200 个；超出返回 429 / 4292。
6. **水印**：正文区域覆盖平铺水印（账号名 + 当前时间到分钟 + 授权 ID 前 8 位）。水印用于追溯，**不作为防泄露手段**。
7. 不提供导出、复制按钮（P1 再评估导出，需审批）；正文按纯文本渲染（F-CON-10）。
8. 正文响应 `Cache-Control: no-store`。

**验收标准**
- AC-ADM-08-1【后端】：输入 `support` 账号（有 `msg.meta`，无 `msg.read.group`）；操作 申请 g1 的正文授权；预期 403；错误码 4030。
- AC-ADM-08-2【后端】：输入 审核员；操作 申请 g1 授权，分别缺工单号、缺理由、缺动态码、动态码错误；预期 依次 400 / 4000、400 / 4000、403 / 4038、401 / 4015，均未产生授权，im-server 侧无授权记录；错误码 4000、4038、4015。
- AC-ADM-08-3：输入 审核员带工单号、理由、动态码申请 g1 授权；操作 加载正文；预期 能看到 g1 正文；审计有 1 条 `msg.read.grant`（含工单号和理由）；im-server `admin_audit` 有对应授权记录；错误码 —。
- AC-ADM-08-4【后端】[IM 钟][C 钟]：输入 g1 的有效授权；操作 用它请求 g2 的正文；推进两边时钟 16 分钟后再请求 g1；预期 两次都 403；错误码 4039。
- AC-ADM-08-5【前端】：输入 正文页收到 4039；操作 观察界面；预期 弹出"填写工单号与理由 + 动态码"框，不跳登录页；错误码 4039。
- AC-ADM-08-6【后端】：输入 授权一次后翻 3 页；操作 查审计；预期 1 条 `msg.read.grant` 和 3 条 `msg.read.page`（含 seq 范围），全部字段中搜不到正文特征串；错误码 —。
- AC-ADM-08-7【后端】：输入 `IM_C2C_BODY_ACCESS=off`；操作 审核员申请 `c2c:a:b` 的授权；预期 403，`reason=c2cBodyDisabled`；错误码 4030。
- AC-ADM-08-8【后端】：输入 审核员当天已申请 20 个授权；操作 申请第 21 个；预期 429，未产生授权；错误码 4292。
- AC-ADM-08-9【后端】：输入 用 `console` 主凭证（不是 `console-msgread`）直接调用 `POST /admin/msg-grants`；操作 调用；预期 403；错误码 4030。
- AC-ADM-08-10【前端】：输入 正文页；操作 检查 DOM；预期 存在覆盖整个消息区域的水印层，文本含当前账号名、到分钟的时间、授权 ID 前 8 位；页面上没有导出、复制按钮；错误码 —。

#### F-ADM-31 删除违规消息（第二批，P0，PRD-core v1.3 变更）

**规则**
1. 需要 `msg.delete` + 理由 + step-up；可以在正文页单条或多选（单次 ≤ 50 条，同一会话）。群提示消息（`type=tip`）不能删除（400 / 4000）。
2. 服务端**软删除**：保留 seq、msgId、发送者、时间、类型，清空正文，标记 `deleted=true`、`deletedBy`（`{caller}:{operator}`）、`deletedAt`。**seq 不留空洞**。原文同时进入隔离存储（F-ADM-32，同一事务）。
3. 若被删的是会话最后一条，同一事务内把 `conv_seq` 的摘要改为占位（`lastMsg.deleted=true`，没有 `text`）。
4. **未读数**：已删除的消息仍计入未读（C-02 公式不变，按 seq 计算）。
5. 在线的会话参与者（单聊双方、群全体成员，含发送者的其他端）收到事件 `msgDeleted {convId, msgIds, seqs}`；离线的在下次 `sync` 时拿到 `deleted=true`、没有 `body` 的消息。
6. 客户端：把该消息替换为"该消息已被删除"，并**清除本地保存的原文**（sdk-ts：内存中的消息列表；sdk-kotlin / sdk-swift：本地数据库）；会话列表摘要显示"该消息已被删除"。**已经展示的系统推送通知（APNs / FCM）无法撤回**，接入文档写明。
7. 已删除的消息再次删除：幂等成功，不再下发事件；msgId 不存在 404 / 4044；跨会话的 msgId 400 / 4000；超过 50 条 400 / 4131。
8. 管理接口同时开放给业务方：`POST /admin/messages/delete {convId, msgIds, reason}`（scope `msg.delete`）。
9. 对用户的文案用"已删除，不再显示"，不承诺"立即彻底删除"（每小时备份保留 48 份，ARCH 3.4；隔离原文保存 30 天）。

**验收标准**
- AC-ADM-31-1：输入 群 g1 中 u1 发的消息 m1，3 个成员和 u1 的另一端在线；操作 审核员删除 m1；预期 4 个连接都收到 `msgDeleted {convId:"group:g1", msgIds:[m1], seqs:[…]}`，H5 显示"该消息已被删除"；错误码 —。
- AC-ADM-31-2【后端】：输入 删除 seq 5 的消息；操作 离线成员上线后 `sync`；预期 拿到 seq 5，`deleted=true`、没有 `body`，seq 4、5、6 连续；错误码 —。
- AC-ADM-31-3【前端】（sdk-ts）：输入 H5 已加载含特征串 `ZXQ-DEL-555` 的消息；操作 后台删除该消息后，在页面执行 SDK 调试接口导出内存中的消息列表；预期 搜不到特征串；错误码 —。
- AC-ADM-31-4【前端】（sdk-kotlin / sdk-swift，M5 / M6 首版验收）：输入 原生 SDK 本地库中有含特征串的消息；操作 收到 `msgDeleted` 后检查本地数据库文件；预期 搜不到特征串；错误码 —。
- AC-ADM-31-5：输入 被删的是 g1 最后一条消息；操作 成员拉取 `convs`；预期 g1 的摘要显示"该消息已被删除"；未读数不变；错误码 —。
- AC-ADM-31-6【后端】：输入 一次 51 条；跨会话的 msgId；不存在的 msgId；已删除的 msgId；操作 分别删除；预期 依次 400 / 4131、400 / 4000、404 / 4044、200（不再下发事件）；错误码 4131、4000、4044。
- AC-ADM-31-7【后端】：输入 删除操作；操作 查 console 审计和 im-server `admin_audit`；预期 都有 msgId 列表和理由，搜不到原文特征串；错误码 —。
- AC-ADM-31-8【后端】：输入 `type=tip` 的消息；操作 删除；预期 400；错误码 4000。
- AC-ADM-31-9【后端】：输入 业务方调用方有 `msg.delete`；操作 调用 `POST /admin/messages/delete`；预期 与后台删除效果相同，`deletedBy` 为 `{业务方 id}:`；错误码 —。

#### F-ADM-32 被删原文的加密隔离与取证查看（第二批，P0，X-9）

**规则**
1. 删除时在同一事务里把原文写入 `message_quarantine`：AES-256-GCM，密钥 `IM_QUARANTINE_KEY`（≥ 32 字节，纳入 im-server 生产启动保护，不能与其他密钥相同），每条随机 nonce；记录 msgId、convId、删除时间、到期时间。
2. 保存 30 天（`IM_QUARANTINE_DAYS`，默认 30，范围 1～365；法务要求更长时改配置，不需要改需求）。im-server 新增进程内定时任务（每小时一次，集群下只在一个节点运行），**物理删除**到期的记录。
3. 查看：需要 `msg.quarantine.read`（`moderator`），流程与 F-ADM-08 相同（工单号 + 理由 + 当次动态码 → im-server 授权 `quarantine=true`，配额与正文授权合并计算）；单聊消息的隔离原文同样受 `IM_C2C_BODY_ACCESS` 控制。
4. 到期后查看返回 404 / 4044。

**验收标准**
- AC-ADM-32-1【后端】：输入 删除含特征串 `ZXQ-QUAR-321` 的群消息；操作 检查 `messages` 表和 `message_quarantine` 表的原始字节；预期 `messages` 中没有特征串；`message_quarantine` 中也搜不到明文（已加密）；错误码 —。
- AC-ADM-32-2：输入 已删除的群消息；操作 审核员按 F-ADM-08 流程申请隔离原文授权并查看；预期 看到原文；审计有 `msg.quarantine.grant`；错误码 —。
- AC-ADM-32-3【后端】[IM 钟]：输入 删除后推进 30 天零 1 小时，清理任务运行；操作 查看隔离原文、检查表；预期 404；表中已没有该记录；错误码 4044。
- AC-ADM-32-4【后端】：输入 `IM_ENV=production`，未配置 `IM_QUARANTINE_KEY` 或与 JWT 密钥相同；操作 启动 im-server；预期 非 0 退出；错误码 —。
- AC-ADM-32-5【后端】：输入 `IM_C2C_BODY_ACCESS=off`，已删除的单聊消息；操作 申请隔离原文授权；预期 403，`reason=c2cBodyDisabled`；错误码 4030。

### 6.5 禁言

#### F-ADM-40 用户级禁言（第二批，P0，PRD-core v1.3 变更）

**规则**
1. 禁言后该用户在**所有会话**（单聊、群聊；二期聊天室）都不能发消息；可以登录、接收、拉历史、标记已读、处理好友申请。
2. 需要 `mute.write` + 理由。时长：10 分钟、1 小时、1 天、7 天、30 天、永久，或自定义 1 分钟～30 天。
3. 被禁言用户 `send` 返回 ack 错误 **4031 MUTED**，`error.details = {scope: "user", mutedUntil}`（永久为 `null`）。禁言检查在发送限流之前，4031 不扣发送令牌（C-29 措辞同 PRD-social V-06）。
4. 在线连接收到 `muted {scope: "user", mutedUntil}` / `unmuted {scope: "user"}`；**`auth` 的 ack 增加 `mutedUntil`**（未禁言时不出现），离线期间被禁言的用户上线即可禁用输入框（R-ADM-17）。
5. 到期自动解除（发送时按 [IM 钟] 判断）；重复禁言以最后一次为准；禁言状态在发送路径上读缓存，经 `cache.Invalidator`（主题 `mutes`）失效 + TTL 60 秒兜底（ARCH 2.2 约束 2）。
6. 管理接口：`POST /admin/mute {scope: "user", userId, durationSeconds | null, reason}`、`POST /admin/unmute {scope: "user", userId, reason}`（scope `mute.write`，与 PLAN 2.3 的单接口对齐）；`GET /admin/users/{id}` 增加 `mutedUntil`。不存在的用户 404 / 4042。
7. 官方账号同样可以被禁言（PRD-social F-SOC-13 只豁免好友制和限额）。

**验收标准**
- AC-ADM-40-1【后端】[IM 钟]：输入 u1 禁言 10 分钟；操作 u1 发单聊和群聊；预期 都返回 4031，`details.scope=user`、`mutedUntil` = 禁言时间 + 10 分钟（±5 秒）；错误码 4031。
- AC-ADM-40-2：输入 禁言中的 u1；操作 别人给 u1 发消息，u1 `sync`、`read`；预期 都正常；错误码 —。
- AC-ADM-40-3【后端】[IM 钟]：输入 u1 禁言 10 分钟；操作 推进 11 分钟后发送；预期 成功；错误码 —。
- AC-ADM-40-4：输入 禁言中的 u1 在线；操作 客服解除禁言；预期 u1 在线端收到 `unmuted`，输入框恢复，发送成功；错误码 —。
- AC-ADM-40-5【前端】（sdk-ts）：输入 u1 收到 `muted`；操作 观察聊天页；预期 输入框禁用，显示"你已被禁言，解除时间：{时间}"；永久禁言显示"你已被禁言"；错误码 —。
- AC-ADM-40-6：输入 u1 离线时被禁言 1 天；操作 u1 上线并打开任一会话；预期 `auth` ack 带 `mutedUntil`，输入框一开始就是禁用状态；错误码 —。
- AC-ADM-40-7【后端】：输入 不存在的用户；操作 禁言；预期 404；错误码 4042。
- AC-ADM-40-8【后端】：输入 发送限流 20 条 / 秒；操作 禁言中的 u1 1 秒内发 30 条，然后解除禁言立即发 1 条；预期 30 条都返回 4031（没有 4290）；解除后的那条成功；错误码 4031。
- AC-ADM-40-9【后端】（双节点）：输入 u1 连在节点 2；操作 在节点 1 禁言 u1；预期 1 秒内 u1 在节点 2 发送返回 4031；错误码 4031。

#### F-ADM-41 群成员禁言（第二批，P0，PRD-core v1.3 变更）

**规则**
1. 只在指定群内不能发言；时长、理由同 F-ADM-40；`send` 返回 4031，`details = {scope: "group", groupId, mutedUntil}`。
2. 用户级禁言与群内禁言同时存在时，返回 `scope: "user"`（范围大的优先），`mutedUntil` 取用户级记录。
3. 被禁言者不是该群成员：404 / 4046（`TARGET_NOT_GROUP_MEMBER`，PRD-social 已定义，扩展用于本场景）。群主、管理员（M4）也可以被后台禁言。
4. 被移出群（包括退群、解散）时清除该群的禁言记录；再次入群不继承。
5. 只有被禁言者本人收到 `muted {scope: "group", groupId, mutedUntil}`；`convs` 中该群会话增加 `mutedUntil`（离线上线后即可禁用输入框）。不公开通知全群（P1 再考虑群提示消息）。
6. 接口：`POST /admin/mute {scope: "group", groupId, userId, ...}`、`POST /admin/unmute {scope: "group", ...}`；群成员列表返回每个成员的 `mutedUntil`。

**验收标准**
- AC-ADM-41-1【后端】：输入 u1 在 g1 禁言 1 小时；操作 u1 在 g1、g2 和单聊发言；预期 g1 返回 4031（`scope=group`、`groupId=g1`）；g2 和单聊成功；错误码 4031。
- AC-ADM-41-2【后端】：输入 u1 同时被用户级禁言和 g1 禁言；操作 在 g1 发言；预期 4031，`scope=user`；错误码 4031。
- AC-ADM-41-3【后端】：输入 u1 在 g1 被禁言；操作 把 u1 移出 g1 再加入，然后在 g1 发言；预期 成功；错误码 —。
- AC-ADM-41-4：输入 u1 在 g1 被禁言；操作 打开群详情成员列表，再解除；预期 u1 显示"禁言中"和到期时间；解除后标记消失；错误码 —。
- AC-ADM-41-5【后端】：输入 u9 不是 g1 成员；操作 在 g1 禁言 u9；预期 404；错误码 4046。
- AC-ADM-41-6：输入 u1 离线时在 g1 被禁言；操作 u1 上线拉 `convs` 并打开 g1；预期 g1 会话带 `mutedUntil`，输入框禁用；打开 g2 输入框可用；错误码 —。

### 6.6 系统通知

#### F-ADM-50 系统通知（第二批最后，P0，阻塞于二期 push 协议）

**描述**：对应腾讯 IM 的系统消息 / 业务推送（现有 App 借道群消息实现，曾因监听错回调收不到）。客户端 SDK 通过 `pushes` 流交给业务代码按 `type` 分发。

**规则**
1. 目标：① 按用户（手填或 CSV，单次 ≤ 1000 个 userId）；② 全员。按房间推送随二期聊天室。
2. 内容：`type`（从 console-server 配置的模板中选择，如 `notice`、`activity`，每个模板定义 `data` 的 JSON Schema）+ `data`（JSON，≤ 4KB，按模板校验）。
3. **投递语义见 [PRD-live v1.0](PRD-live-v1.0.md) 第 4 节**（已冻结）：按用户推送持久化 7 天、离线用户上线补收；全员推送只投递给当前在线连接；多端在线时每个连接各收一次。`pushId` **全局唯一**，由调用方生成；"相同由 SDK 去重"明确为**同一 SDK 实例、同一账号内**，不跨设备。AC-ADM-50-1～8 的预期不变（对齐表见 PRD-live 4.7 节）。
4. 按用户推送：`push.user`，需要理由（选填工单号）。
5. **全员推送双人复核**：`operator` 发起草稿（`push.all.request` + 理由），另一个有 `push.all.approve` 的账号（`super_admin`）step-up + 输入"全员发送"后批准并发送；发起人不能批准自己的草稿；草稿 24 小时未批准自动作废。**全局**（所有账号合计）两次全员推送间隔 ≥ 5 分钟，否则 429 / 4290。同号重放不受这个间隔约束，也不写入"最近一次全员发送时间"。
6. 发送前必须预览（目标人数 / "全员"、type、格式化后的 data）。
7. **发送记录保存 `data` 全文**（`biz.push_records`，保留期与审计一致；审计表只存长度和 SHA-256，两者可对上）；审计员和超管可以查看发送记录全文。
8. 管理接口 `POST /admin/push {target: "user" | "all" | "room", userIds?, roomId?, type, data, pushId, ttlSeconds?}`（scope `push.user` / `push.all` / `push.room`；`target=room` 的接口 L2 开放；`ttlSeconds` 见 PRD-live F-PUSH-02 规则 4）。userId 列表中有不存在的用户：404 / 4042 并在 `details.userIds` 列出，整批不发送；> 1000：400 / 4131。

**验收标准**（push 协议定稿前为草案）
- AC-ADM-50-1：输入 u1、u2 在线；操作 运营向两人发送 type=`notice` 的通知；预期 两人 3 秒内收到 `push` 帧，`type`、`data` 与发送内容一致；错误码 —。
- AC-ADM-50-2【后端】[IM 钟]：输入 u3 离线；操作 按用户推送后，u3 在第 6 天上线；另一条推送后推进 8 天 u4 上线；预期 u3 收到；u4 收不到；错误码 —。
- AC-ADM-50-3：输入 5 个用户在线、u9 离线；操作 全员推送（批准后）；预期 5 人都收到；u9 上线后收不到；错误码 —。
- AC-ADM-50-4【后端】：输入 `data` 不符合模板 Schema，或超过 4KB；操作 发送；预期 400，不投递；错误码 4000。
- AC-ADM-50-5【后端】：输入 全员推送在 T 成功；操作 另一位运营发起、另一位超管在 T+4 分钟批准；预期 429，提示"全员通知间隔不少于 5 分钟"；T+5 分钟后批准成功；错误码 4290。
- AC-ADM-50-6【后端】：输入 userId 列表含不存在的 `ghost1`、`ghost2`；1001 个 userId；操作 分别发送；预期 404，`details.userIds=[ghost1, ghost2]`，整批未发送；400；错误码 4042、4131。
- AC-ADM-50-7【后端】：输入 运营 op1 发起的全员草稿；操作 op1（同时有超管角色的测试账号）批准自己的草稿；预期 403；错误码 4030。
- AC-ADM-50-8【后端】：输入 一次全员推送；操作 审计员查看发送记录与审计；预期 发送记录有 `data` 全文；审计中 `data` 只有长度和 SHA-256，且与全文计算结果一致；错误码 —。

### 6.7 P1 / P2（不在两批内）

#### F-ADM-42 全群禁言（P1）

**规则**：开启后**群主和管理员仍可发言**（M4 之后有群角色，按 PRD-social 9.5 节修订），其他成员发言返回 4031（`details.scope="groupAll"`）；全群在线成员收到 `muted {scope: "groupAll", groupId}`；`convs` 中该群带 `allMuted=true`。

**验收标准**
- AC-ADM-42-1【后端】：输入 自建群 g1（群主 alice、管理员 carol、成员 bob）；操作 开启全群禁言后三人各发一条，再关闭后 bob 发言；预期 alice、carol 成功，bob 返回 4031（`scope=groupAll`）；关闭后 bob 成功；错误码 4031。
- AC-ADM-42-2：输入 g1 成员在线；操作 开启全群禁言；预期 全部在线成员收到 `muted {scope:"groupAll"}`；错误码 —。

#### F-ADM-70 数据概览（P1）

**规则**：首页卡片：在线连接数（按平台）、今日新增用户、今日消息数、群总数；来自 `GET /admin/stats`（G-ADM-17）；每 60 秒刷新，**刷新请求带 `X-Console-Background: 1`**（不续期会话）；不读运维用的 `/metrics`。

**验收标准**
- AC-ADM-70-1：输入 3 个 web、2 个 pc 连接在线；操作 打开首页；预期 显示在线 5（web 3、pc 2），60 秒内刷新；错误码 —。
- AC-ADM-70-2【后端】[C 钟]：输入 首页打开、无人操作；操作 推进 31 分钟后查看下一次刷新请求的结果；预期 401；错误码 4014。

#### F-ADM-60 敏感词与内容审核（P2）

**规则**：维护敏感词库（拦截 / 替换为 `*`）；im-server 发送前检查（之后接 ARCH S14 的审核回调）；词库变更经 `cache.Invalidator` 通知所有节点。拦截时的错误码**届时分配**：403x 已用满，建议使用 HTTP 451 语义的 4510 CONTENT_BLOCKED（本版不写入 errors.json）。

**验收标准**
- AC-ADM-60-1【后端】：输入 拦截词"违规词"；操作 添加 10 秒后发送含该词的消息；预期 被拒绝，消息不落库；错误码 届时分配（建议 4510）。
- AC-ADM-60-2【后端】：输入 替换词"坏词"；操作 发送"这是坏词"；预期 接收方收到"这是**"；错误码 —。

---

## 7. 页面

| 页面 | 内容 | 关联 | 批次 |
|---|---|---|---|
| 登录 / 激活 / 绑定二次验证 | 一步登录（账号、密码、动态码或恢复码）；激活码 + 新密码；扫码绑定、显示恢复码 | F-CON-02、03 | 一 |
| 首页 | 快捷搜索；数据概览（P1） | F-ADM-70 | 一 |
| 用户 | 搜索列表 → 详情（资料、官方标记、在线、好友列表、在线设备、操作按钮）；导入 | F-ADM-10～12、14、15、17 | 一（封禁、禁言为二） |
| 社交配置 | 参数表、差异预览、模式切换确认 | F-ADM-16 | 一 |
| 好友导入 / 删除 | CSV 上传、校验结果、进度 | F-ADM-15 | 一 |
| 群 | 列表 → 详情（设置、成员分页、角色、成员禁言）；建群；解散 | F-ADM-20～25、41 | 一（禁言为二） |
| 消息 | 按会话查元数据；正文授权、水印、删除；隔离原文 | F-ADM-30、08、31、32 | 一（正文、删除为二） |
| 系统通知 | 新建（目标、模板、预览）；全员草稿与批准；发送记录 | F-ADM-50 | 二 |
| 审计日志 | 筛选、详情（intent / result 合并、im-server 侧记录） | F-CON-09 | 一 |
| 账号管理 | 账号列表、新建（激活码）、角色、停用、重置、解锁；应急授权 | F-CON-02、06 | 一 |
| 通知中心 | 安全通知列表 | F-CON-12 | 一 |
| 个人设置 | 修改密码、恢复码剩余数与重新生成、退出 | F-CON-03、04 | 一 |

桌面布局：左侧菜单（按权限显示）+ 顶栏（当前账号、角色、通知、退出）+ 内容区。最小宽度 1280px；Chrome、Edge、Safari、Firefox 最新两个大版本；不做移动端。前端是 console-server 仓库里的独立入口 `web/admin`（运维后台为 `web/ops`）。

---

## 8. 服务端缺口与实现顺序

### 8.1 im-server

标记：**接口** = 新管理接口；**迁移** = 新数据库迁移（0004 起，与 PRD-social 的 0004 / 0005 合并编排）；**协议** = 客户端协议变更；**设施** = 新基础设施。

| # | 能力 | 标记 | 批次 | 依赖 / 说明 |
|---|---|---|---|---|
| G-ADM-03 / 04 / 21 | 管理接口内网监听、调用方密钥与 scope、签名防重放、`admin_audit` 与查询接口、旧密钥过渡 | 接口、迁移、设施 | 第一批（前置） | 第 3 节 F-KEY-01～05；nonce 集群下进 Redis（B6） |
| G-ADM-05 | 用户列表 / 搜索 `GET /admin/users?q=&official=&cursor=&limit=` | 接口、迁移 | 第一批 | 昵称索引（ASCII 不区分大小写）；`LIKE` 转义 `%`、`_`、`\` |
| G-ADM-06 | 在线设备 `POST /admin/users/sessions/query {userId}` | 接口、设施 | 第一批 | 代码中**没有** `delivery.Presence`：`gateway.conn` 要保存平台、建立时间、来源 IP（**只保存脱敏后的值**，F-ADM-14 规则 2），`hub` 增加按用户列出连接；集群下 Presence（B6）。与运维节点接口**共用一个查询函数**（评审 X-05） |
| G-ADM-07 | 用户所在群 `GET /admin/users/{id}/groups` | 接口 | P1 | `group_members(user_id)` 索引由 PRD-social 0004 补上 |
| G-ADM-09 | 群列表 / 搜索 `GET /admin/groups?q=&includeDissolved=&cursor=&limit=` | 接口、迁移 | 第一批 | 成员数用 PRD-social F-SOC-11 的计数；群名索引；`dissolved_at` |
| G-ADM-10 | 群成员分页 `GET /admin/groups/{id}/members?cursor=&limit=&prefix=` | 接口 | 第一批 | 与客户端 `groupMembers`（M4）共用存储查询；本 PRD 先实现 |
| G-ADM-11 | 解散群 `DELETE /admin/groups/{id}`；`DissolveGroup`；群 ID 不复用 | 接口、迁移、协议 | 第一批 | PRD-social F-SOC-26 规则 3 列出的全部读路径（`memberCache`、`GetGroup`、`ListConversations`、`GetConversation`、`GroupConversations`、`myGroups`、`VisibleUsers`、`CreateGroup`）+ 契约测试 `GroupDissolveNoReuse`；`convRemoved.reason` |
| G-ADM-12 | 元数据查询 `POST /admin/convs/messages/query {convId, beforeSeq, limit, from?, sinceTs?, untilTs?}` | 接口、迁移 | 第一批 | 新索引 `messages(conv_id, from_user, seq)`、`messages(conv_id, ts)`；响应不含正文字段；不受 `historyVisible` 限制 |
| G-ADM-12b | 正文授权 `POST /admin/msg-grants`、读取 `POST /admin/convs/messages/body` | 接口、迁移 | 第二批 | `msg_grants` 表、每操作人 / 每调用方每日配额；`IM_C2C_BODY_ACCESS` |
| G-ADM-13 | 用户的会话列表（元数据） | 接口 | P1 | — |
| G-ADM-08 | 封禁 `POST /admin/users/{id}/ban`、`/unban`；签发 token 与 `/demo/login` 检查；`kicked.reason=suspended` | 接口、迁移、协议 | 第二批 | `user_bans` 表；与强制下线共用 token 版本号机制 |
| G-ADM-14 | 删除消息（软删除）`POST /admin/messages/delete`；`msgDeleted`；`sync` / `convs` 的删除标记；`conv_seq` 摘要同事务更新 | 接口、迁移、协议 | 第二批 | `messages.deleted_at / deleted_by`；Schema 兼容（11.1 节） |
| G-ADM-19 | 被删原文加密隔离、30 天到期物理删除 | 迁移、设施 | 第二批 | `message_quarantine`；`IM_QUARANTINE_KEY` |
| G-ADM-22 | im-server 进程内定时任务机制（每小时；集群下单节点运行） | 设施 | 第二批 | 目前服务端没有任何定时任务；与 PRD-social 的 `PurgeSocial`、`admin_audit` 过期清理共用 |
| G-ADM-15 | 禁言（用户级 / 群成员级）`POST /admin/mute`、`/unmute`；4031 `details`；`muted` / `unmuted`；`auth` ack 与 `convs` 带 `mutedUntil` | 接口、迁移、协议 | 第二批 | `mutes` 表；发送路径缓存 + `Invalidator` 主题 `mutes` + TTL 60 秒 |
| G-ADM-16 | 系统推送 `POST /admin/push`；按用户的通知队列与 7 天清理 | 接口、迁移、协议、设施 | 第二批最后 | 二期 push 协议；集群下经消息总线广播；全员推送限速分批下发 |
| G-ADM-17 | 统计 `GET /admin/stats` | 接口 | P1 | Presence |
| G-ADM-18 | 敏感词库 + 发送前检查 | 接口、迁移 | P2 | ARCH S14 |
| G-ADM-25 | `errors.json`、`ERRORS.md`、`frame.schema.json`、`vectors/` 变更 | 协议 | 第一批（错误码一次写全）/ 第二批（Schema） | 第 10、11 节 |

### 8.2 console-server

console-server 的全部能力即第 4 节 F-CON-01～12 与第 6 节的 console 侧部分；仓库待建（`console-server`），含 `web/admin`、`web/ops` 两个前端入口、`migrations/`（Postgres）、`assets/weak-passwords.txt`、`deploy/`（systemd 单元、归档与校验的定时任务）。

### 8.3 实现顺序

1. **F-KEY-01～05**（im-server）——同时解锁 PRD-social 的社交管理接口（S-11）。
2. Postgres 就绪（X-3：本机安装另行授权）→ console-server 骨架、迁移、启动保护（F-CON-01、11）。
3. 账号、登录、会话、角色、step-up、通知（F-CON-02～07、12）→ 审计（F-CON-08、09）→ 浏览器安全（F-CON-10）。
4. im-server 第一批只读接口（G-ADM-05、06、09、10、12）→ 写接口（G-ADM-11）。
5. 管理后台第一批页面（F-ADM-05、10～12、14～17、20～24、30）；F-ADM-25 随 PRD-social M4。
6. PRD-core v1.3 第二批条目与 Schema 定稿（11.1、11.2 节）→ im-server + sdk-ts → PRD-ui v0.2 文案 → 第二批（F-ADM-13、08、31、32、40、41）。
7. 二期 push 协议定稿 → F-ADM-50。

---

## 9. 非功能需求

| 类别 | 要求 |
|---|---|
| 安全 | 第 3、4 节全部；console-server ↔ im-server 走内网 + TLS（管理端口绑定回环时例外）；上线前做一次针对两个后台的渗透测试（ARCH S15） |
| 审计 | F-CON-08、09；在线 1 年 + 加密归档 3 年；锚定每小时 |
| 性能（Postgres 环境） | 用户前缀搜索、群列表 P95 ≤ 1 秒（100 万用户、10 万群）；元数据查询任意一页 P95 ≤ 500ms（单会话 10 万条）；审计查询（31 天、1000 万条）P95 ≤ 2 秒；审计写入（含链头锁）P95 ≤ 20ms、吞吐 ≥ 200 条 / 秒；登录 P95 ≤ 800ms（含 Argon2id）。统计口径：同一场景跑 3 次，取 3 次 P95 的中位数 |
| 性能（SQLite 环境，B6 之前） | 用户搜索、群列表 P95 ≤ 1 秒（10 万用户、1 万群）；元数据查询 P95 ≤ 500ms（单会话 1 万条） |
| 可用性 | 后台不可用不影响 IM 收发；console-server 支持 ≥ 2 个实例；Postgres 不可用时后台只读页面也不可用（返回 503 / 5030），IM 不受影响 |
| 兼容 | 桌面浏览器 Chrome、Edge、Safari、Firefox 最新两个大版本；最小宽度 1280px；不做移动端 |
| 可观测 | console-server 请求日志（不含 Cookie、令牌、理由、正文）；指标：登录失败率、锁定次数、`console_rejected_total`、`console_audit_result_failures_total`、锚定延迟、im-server 调用耗时与错误码分布；im-server：`im_admin_auth_failures_total{reason}`、管理接口按调用方的请求数 |
| 语言 | 一期只有中文 |

**性能 AC**（第一批，`TC-PERF-ADM`）：
- AC-ADM-90-1【后端】：输入 Postgres 环境，100 万用户；操作 并发 10 个会话做随机前缀搜索各 1000 次，跑 3 次；预期 3 次 P95 的中位数 ≤ 1 秒，第 2 页与第 1 页无重复；错误码 —。
- AC-ADM-90-2【后端】：输入 Postgres 环境，单会话 10 万条消息；操作 随机 `beforeSeq` 查询元数据 1000 次，跑 3 次；预期 P95 中位数 ≤ 500ms；错误码 —。
- AC-ADM-90-3【后端】：输入 1000 万条审计记录；操作 31 天范围按操作人筛选 200 次，跑 3 次；预期 P95 中位数 ≤ 2 秒；错误码 —。
- AC-ADM-90-4【后端】：输入 两个 console-server 实例；操作 并发写审计 60 秒；预期 吞吐 ≥ 200 条 / 秒，写入 P95 ≤ 20ms，之后 `audit verify` 通过；错误码 —。

---

## 10. 错误码

### 10.1 新增错误码（14 个，写入 errors.json）

号段规则（ERRORS.md 第 1 节）：新码前三位与 HTTP 状态一致；HTTP 状态以 errors.json 的 `http` 字段为准（既有例外：413x 使用 HTTP 400，沿用 4130、4131）。新增 `scope: "console"`：只出现在 console-server 对浏览器的接口上；**SDK 生成器跳过只含 `console` 的码**，不会混进终端 SDK 的枚举。

| code | 名称 | HTTP | 通道（scope） | 可重试 | 含义 | 处理 | 批次 |
|---|---|---|---|---|---|---|---|
| 4014 | CONSOLE_AUTH_REQUIRED | 401 | console | 否 | 没有有效的后台会话（未登录、过期、退出、被注销、被挤掉），`reason` 见 F-CON-05 规则 6 | 跳登录页（**前端只在这个码时跳登录页**） | 一 |
| 4015 | CONSOLE_CREDENTIALS_INVALID | 401 | console | 否 | 账号、密码、动态码、恢复码或激活码错误（登录、激活、step-up、改密时的当前密码） | 原地提示"账号、密码或动态码错误"，不跳转 | 一 |
| 4016 | CONSOLE_SETUP_REQUIRED | 401 | console | 否 | 会话处于首次设置状态（未绑定 TOTP），只能调用设置接口 | 跳"绑定二次验证"页 | 一 |
| 4017 | ADMIN_SIGNATURE_INVALID | 401 | admin | 否 | 管理接口（含 PRD-ops F-OPS-16 节点运维接口）签名无效：缺签名头、调用方不存在或停用、签名错误、时间戳超出 5 分钟、nonce 重复、目标节点不符 | 业务服务端检查时钟、密钥和签名实现 | 一（前置） |
| 4034 | USER_BANNED | 403 | admin | 否 | 用户被封禁，不能签发 token / 演示登录；`details.bannedUntil`（永久为 `null`）。也用于 `/demo/login`（HTTP，与管理接口同一响应格式）；scope 保持 `admin`，SDK 生成器不跳过，sdk-ts 能拿到 | 业务方提示用户"账号已被封禁" | 二 |
| 4037 | CSRF_INVALID | 403 | console | 否 | CSRF 令牌缺失或错误，或 `Origin` 不匹配 | 重新取会话信息后重放一次，仍失败提示刷新 | 一 |
| 4038 | STEP_UP_REQUIRED | 403 | console | 否 | 该操作需要重新验证（未验证、超过 5 分钟，或正文授权未带当次动态码） | 弹 TOTP 框，通过后自动重放 | 一 |
| 4039 | MSG_READ_GRANT_REQUIRED | 403 | console | 否 | 没有该会话有效的正文 / 隔离原文查看授权（未申请、已过期、换了会话） | 弹"工单号 + 理由 + 动态码"框 | 二 |
| 4044 | MESSAGE_NOT_FOUND | 404 | admin / console | 否 | 消息不存在（或隔离原文已过期删除） | 提示"消息不存在或已过期" | 二 |
| 4092 | CONSOLE_ACCOUNT_EXISTS | 409 | console | 否 | 同一账号空间内账号名已存在 | 换一个账号名 | 一 |
| 4093 | CONSOLE_ACCOUNT_STATE_CONFLICT | 409 | console | 否 | 操作会让自己失去权限、使空间内没有启用的空间超管，或违反职责分离（互斥角色与其他角色叠加：管理空间 `auditor`，运维空间 `ops_superadmin`、`ops_auditor`） | 提示具体原因（按 `reason`） | 一 |
| 4291 | CONSOLE_LOGIN_LOCKED | 429 | console | **否**（例外） | 登录失败过多，该账号名（或账号名 + IP）暂时锁定 | 提示"尝试次数过多，请 N 分钟后再试"，不自动重试 | 一 |
| 5031 | AUDIT_UNAVAILABLE | 503 | console | 是 | 审计写入失败，操作**未执行** | 提示"审计服务不可用，操作未执行，请稍后重试" | 一 |
| 5040 | UPSTREAM_RESULT_UNKNOWN | 504 | console | **否** | 调用 im-server 超时，操作结果未知 | 提示"操作结果未知，请刷新确认，不要重复提交"；系统通知提示"结果未知，系统会自动确认，请稍后查看发送记录"，由对账任务按 `pushId` 重放 | 一 |

### 10.2 已有错误码的扩展（只扩 `scope` / `http` / 说明，不改含义）

| code | 变更 | 来源 |
|---|---|---|
| 4000、4004、4030、4040、4130、4131、4290、5000、5030 | `scope` 增加 `console`（console-server 在自己的接口上也会产生这些码，例如 F-ADM-22 规则 2 人数超限由 console-server 自己返回 4130、AC-ADM-11-3 的 CSV 超行返回 4131；按下方透传规则，只透传、不自己产生的码如 4042、4043、4090、4046、4132 不加 `console`） | 2.3 节 |
| 4007 INVALID_CONV_ID | `scope` 增加 `admin`、`console`，`http` 由 `null` 改为 400（原来只在 ws 上使用，`http` 为空，补齐不改含义） | F-ADM-30（评审 R-ADM-03 ④） |
| 4031 MUTED | 去掉"二期"，`action` 改为"提示禁言及解除时间（`details.mutedUntil`）"；ack 的 `error` 增加 `details: {scope, groupId?, mutedUntil}` | F-ADM-40、41 |
| 4292 QUOTA_EXCEEDED（PRD-social 新增） | `scope` 增加 `admin`、`console`（正文授权配额、在线设备 / 单聊元数据的每日上限） | F-ADM-08、14、30 |
| 4046 TARGET_NOT_GROUP_MEMBER（PRD-social 新增） | 增加用于"群成员禁言的对象不是群成员" | F-ADM-41 |
| `kickReasons` | 增加 `{reason: "suspended", closeCode: 4012, note: "封禁，带 until"}` | F-ADM-13 |
| `closeCodes` | 增加 `{code: 1011, name: "INTERNAL_ERROR", reconnect: true, meaning: "服务端内部错误（如握手阶段查询吊销状态失败）"}`（代码已使用，PRD-ops F-OPS-09 规则 3 计入断开记录） | 复审 E-06 |
| **透传规则** | console 通道可以透传任何 `admin` scope 的码，透传码**不需要**逐个加 `console` scope；只有 console-server 自己产生的码才写 `scope: console`。写进 ERRORS.md 第 2 节 | 复审 E-04 |

### 10.3 与已占用错误码的核对

| 号段 | 已有（errors.json） | PRD-social v0.2 已占用 | 本 PRD 新增 | 剩余空位 |
|---|---|---|---|---|
| 400x | 4000～4009 | — | — | **无**（之后参数类细分用 4000 + `reason`，如密码策略） |
| 401x | 4010～4013 | — | 4014、4015、4016、4017 | 4018、4019 |
| 403x | 4030～4033 | 4035、4036（并把 4037～4039 留给本 PRD） | 4034、4037、4038、4039 | **无**（之后无权限类细分用 4030 + `reason`，如 `c2cBodyDisabled`） |
| 404x | 4040～4043 | 4045、4046 | 4044 | 4047～4049 |
| 409x | 4090 | 4091 | 4092、4093 | 4094～4099 |
| 413x | 4130、4131 | 4132～4137 | — | 4138、4139 |
| 429x | 4290 | 4292（并把 4291 留给登录锁定） | 4291 | 4293～4299 |
| 500x / 503x | 5000、5030 | — | 5031 | 5001～5009、5032～5039 |
| 504x（新区段） | — | — | 5040 | 5041～5049 |

结论：14 个新码与 errors.json 的 28 个已有码、PRD-social v0.2 的 12 个新码**没有重复**；PRD-social 预留给本 PRD 的 4037～4039、4291 按预留用途使用。PRD-ops v0.2 共用本节的基座错误码（4014～4016、4037、4038、4092、4093、4291、5031、5040），节点运维接口签名失败用 4017，每日排查上限用 4292；PRD-ops 声明不新增错误码（已核对）。之后若需要运维专用的新码，从上表"剩余空位"中取，并与本 PRD 一起写入 errors.json（errors.json 只增不改，写入前几份 PRD 一起核对）。

### 10.4 ERRORS.md 需要同步修改

1. 第 1 节区段表：401x 改为"4010～4012 用户凭证（回登录页）；4013、4017 管理接口凭证；4014～4016 后台会话（只出现在 console 通道）"；429x 注明 4291、4292 不可自动重试（例外）；新增 504x"结果未知，不自动重试"；注明 400x、403x 已用满，之后用 `reason` 细分。
2. 第 2 节：增加"后台接口（console）"一行：HTTP 状态取 `http`，响应体 `{code, msg, reason?, details?}`；`details` 为可选扩展对象。
3. 第 3 节表格补 14 个新码和 10.2 节的扩展；第 4 节 `kicked` 原因补 `suspended`。
4. 第 5 节"与 M1 实现的差异"补："管理接口在对外端口 → 迁到 `IM_ADMIN_ADDR`""旧密钥可调用全部接口 → 只能调用 v1.2 接口"。
5. **对服务端的变更**：im-server 的错误码一致性测试（`server/internal/protocol/errors_test.go` 的 `TestCodesMatchErrorsJSON`）跳过只含 `console` scope 的码（4014～4016、4037～4039、4092、4093、4291、5031、5040），im-server 不为它们定义 Go 常量；console-server 仓库有自己的一致性测试，只核对 `console` 码。
6. **对 protocol 的变更**：errors.json `closeCodes` 增加 1011 `INTERNAL_ERROR`（`reconnect: true`），ERRORS.md 第 4 节同步（10.2 节）。
7. 第 1 节注明 413x 使用 HTTP 400（沿用 4130、4131）；ADMIN.md 开头"HTTP 状态码由错误码前三位决定"改为"取 errors.json 的 `http` 字段"。

---

## 11. 协议与文档变更

### 11.1 协议 Schema（`frame.schema.json`）与兼容规则

| 变更 | 改法 | 批次 |
|---|---|---|
| 通用：SDK 必须忽略不认识的事件类型、字段和枚举值 | 写入 `protocol/README.md`；三套 SDK 加一致性测试（`vectors/frames.json` 增加"未知事件 / 未知字段 / 未知 reason"用例）。与 PRD-social V-18 是同一条 | 一 |
| `convRemoved` 增加可选 `reason` | 枚举 `removed` / `dissolved`（PRD-social M4 增加 `left`）；`additionalProperties: false` 下显式加字段 | 一 |
| `Error` 增加可选 `details`（对象） | 不放开 `additionalProperties`，只增加一个对象字段承载 `mutedUntil`、`scope`、`groupId`、`bannedUntil`、`userIds` | 一（先发，供第二批使用） |
| `Message.body` 改为条件必填：`deleted=true` 时不出现；增加 `deleted`、`deletedAt`；`LastMsg` 增加 `deleted` | `if/then` 条件 | 二 |
| 新增事件 `msgDeleted {convId, msgIds, seqs}`、`muted {scope, groupId?, mutedUntil}`、`unmuted {scope, groupId?}` | 加入 `EventFrame` 的 `oneOf` | 二 |
| `kicked` 增加 `reason: "suspended"` 和可选 `until` | 枚举扩展 | 二 |
| `auth` ack 增加可选 `mutedUntil`；`convs` 的群会话项增加可选 `mutedUntil`（P1：`allMuted`） | 可选字段 | 二 |
| 下行 `push` 帧 | 二期 push 协议定义 | 二（最后） |

**发布顺序**：每个变更先发"能容忍新字段 / 新事件"的 SDK（第一批随 PRD-social M3 一起发），服务端后开启。第二批的服务端开关 `IM_V13_ADMIN_FEATURES` 默认关闭，sdk-ts 对应版本发布后再打开；原生 SDK 首版（M5 / M6）直接包含。

### 11.2 对 PRD-core v1.3 的变更条目

> 编号 VA-xx（与 PRD-social 的 V-xx 并列合入 v1.3）。

| # | PRD-core 位置 | 变更 | 对现有 AC / 用例的影响 | 批次 |
|---|---|---|---|---|
| VA-01 | 第 7 节；D1 | "禁言、删消息、系统推送：二期"改为：禁言、删消息、封禁、群消息正文查看进入第二批（v1.3）；系统推送为第二批最后一项、**仍以二期 push 协议定稿为前提**（X-6 已拍板） | 无 | 二 |
| VA-02 | 第 3 节 | 新增后台相关功能点（引用本 PRD 第 2.1 节）；"管理后台页面：待定"改为本 PRD | — | 一 / 二 |
| VA-03 | F-AUTH-01；AC-AUTH-01-1 | 签发 token 时用户被封禁返回 403 / 4034（`details.bannedUntil`）；AC-AUTH-01-1 前置条件补"用户未被封禁" | 现有用例不封禁，结果不变 | 二 |
| VA-04 | F-AUTH-04（演示登录） | 演示登录同样检查封禁，返回 4034 | 同上 | 二 |
| VA-05 | F-GRP-01 | 新增解散群（管理接口 `DELETE /admin/groups/{id}`）；**已解散群的 ID 不能复用**（建群返回 4090）；对已解散群的读写返回 4043 | 现有用例不解散，结果不变；新增 TC-GRP 解散用例 | 一 |
| VA-06 | F-GRP-03 | `convRemoved` 增加 `reason`（`removed` / `dissolved`）；与 PRD-social V-11 合并为一条 | 可选字段 | 一 |
| VA-07 | 第 8 节（协议） | SDK 忽略未知事件 / 字段 / 枚举值；`Error.details`（11.1 节） | 无 | 一 |
| VA-08 | F-MSG-01；C-29 | 发送可能返回 4031（`details`）；禁言检查在发送限流之前，4031 不扣发送令牌 | 默认无禁言，现有用例不受影响 | 二 |
| VA-09 | F-AUTH-02（`auth`）；F-CONV-01 | `auth` ack 增加 `mutedUntil`；群会话增加 `mutedUntil` | 可选字段 | 二 |
| VA-10 | 第 8 节事件 | 新增 `muted`、`unmuted`、`msgDeleted`；`kicked.reason` 增加 `suspended`（带 `until`） | 无 | 二 |
| VA-11 | F-SYNC-01、F-CONV-01 | 消息增加 `deleted`；删除后不含 `body`；`lastMsg.deleted`；**未读公式（C-02）不变**，已删除的消息仍计入未读 | 无 | 二 |
| VA-12 | 第 5 节文案表 | 新增："群聊已解散"（一）；"该消息已被删除""你已被禁言，解除时间：{时间}""你已被禁言""账号已被封禁，解封时间：{时间}"（二） | 现有文案不变 | 一 / 二 |
| VA-13 | 6.1 节测试环境 | im-server 测试时钟驱动封禁 / 禁言 / 隔离原文到期；`tools/set-clocks.sh`；console-server 测试模式 | — | 一 |
| VA-14 | F-SYS-01 规则 4、C-10 | 错误码一致性测试覆盖 4031 的 `details`；`scope: console` 的码不进 SDK 枚举；im-server 的 `TestCodesMatchErrorsJSON` 同样跳过只含 `console` scope 的码（10.4 节第 5 条） | — | 一 |
| VA-15 | 下行 `push` | 系统通知（二期 push 协议） | — | 二（最后） |

### 11.3 对 `protocol/ADMIN.md` 的变更条目

| # | 变更 | 批次 |
|---|---|---|
| AD-01 | 开头"所有接口都需要请求头 `X-Admin-Key`"改为：管理接口只在 `IM_ADMIN_ADDR` 上提供；鉴权为调用方 + 签名（F-KEY-02、03，签名示例与 `vectors/admin-sign.json`）；旧 `X-Admin-Key` 为过渡方案（F-KEY-05） | 一（前置） |
| AD-02 | 每个接口增加"所需 scope"一列（F-KEY-02 规则 2 的表） | 一（前置） |
| AD-03 | 新增"审计"小节：`admin_audit` 记录范围、`GET /admin/audit` | 一（前置） |
| AD-04 | 新增接口：`GET /admin/users?q=`、`POST /admin/users/sessions/query`、`GET /admin/groups?q=`、`GET /admin/groups/{id}/members`、`DELETE /admin/groups/{id}`、`POST /admin/convs/messages/query` | 一 |
| AD-05 | 群规则补："已解散的群：查询、加成员、改设置返回 404 / 4043；用同一 ID 建群返回 409 / 4090"；错误优先级中"群不存在 4043"注明含已解散 | 一 |
| AD-06 | 社交管理接口（PRD-social 9.3 节）注明"F-KEY 上线后才在生产注册"，scope 为 `social.settings` / `social.friends` | 一 |
| AD-07 | 新增接口：`POST /admin/users/{id}/ban`、`/unban`、`POST /admin/mute`、`/unmute`、`POST /admin/messages/delete`、`POST /admin/msg-grants`、`POST /admin/convs/messages/body`；`GET /admin/users/{id}` 增加 `ban`、`mutedUntil`；`POST /admin/users/token` 增加 4034 | 二 |
| AD-08 | 新增 `POST /admin/push` | 二（最后） |
| AD-09 | "运维"小节补 `IM_ADMIN_ADDR`、`IM_ADMIN_TLS_*`、`IM_ADMIN_ALLOWED_CIDRS`、`IM_ADMIN_CALLERS`、`IM_ADMIN_LEGACY`、`IM_C2C_BODY_ACCESS`、`IM_QUARANTINE_KEY` / `_DAYS`；"仅测试"小节注明 `/admin/test/*` 也只在管理地址上 | 一 / 二 |

### 11.4 对其他文档的影响

| 文档 | 变更 |
|---|---|
| ARCH-v1 | 第 1.2 节组件增加 console-server（独立仓库、Postgres）；4.2 节 S8 由 P1 改为"已按 F-KEY 实现"，签名算法增加 Ed25519（只存公钥）；2.2 节注明 `delivery.Presence` 尚未实现，G-ADM-06 先补连接元数据；S13 未实现列入合规风险；第 7 节实现记录补"管理端口与指标端口分离" |
| PRD-social v0.2 | 9.3 节最后一行"后台服务默认只有 `social.friends` 的查询"改为"console 主凭证持有 `social.friends`、`social.settings`（X-6 第一批包含好友导入、模式配置、官方标记）"；F-SOC-26 / 9.5 节"是否提前到第一批"改为"已提前（X-6 + S-3）"；`admin_audit` 字段以本 PRD F-KEY-04 为准；4046 增加禁言用途 |
| PRD-ops v0.2 | 引用第 4 节作为账号、登录、会话、角色、step-up、审计、部署的唯一规格；定义 `ops` 空间角色（含运维超管、审计员是否可看全部空间）；节点运维接口按 F-KEY-03 同一实现签名（调用方 `console-ops`、Ed25519、每节点一把独立私钥、节点只存公钥，待签名串第 8 行 `X-IM-Target-Node` 必填）；"当前连接"与 G-ADM-06 共用查询函数和脱敏规则（IPv4 前 2 段、IPv6 前 48 位，在 im-server 内完成）；运维空间互斥角色为 `ops_superadmin`、`ops_auditor`，不提供应急授权；运维专用错误码从 10.3 节空位中取 |
| PRD-ui v0.2 | 第二批的客户端提示（被禁言输入框、"该消息已被删除"、"群聊已解散"、"账号已被封禁"）补 F-UI 条目和文案；第一批先补"群聊已解散" |
| PLAN | 第 2.3 节管理接口与本 PRD 对齐（`/admin/mute` 区分 `scope`；push 的 `target` 先做 `user` / `all`） |
| QUEUE | 需求分发时 X-5（F-KEY）为第一项；console-server 仓库建立为第二项（需 Postgres 授权） |
| tools/trace.py | 目前只读 `PRD-core-v1.2.md`；需要同时读本 PRD（AC 前缀 `AC-KEY-`、`AC-CON-`、`AC-ADM-` 都符合 `AC-[A-Z]+-\d+-\d+` 和行首 `^- (AC-…)`，验证层标记紧跟编号） |

---

## 12. 合规前提与上线门槛

| 能力 | 上线门槛（全部满足才能开启） |
|---|---|
| 单聊正文查看、单聊隔离原文查看（`IM_C2C_BODY_ACCESS=on`） | ① 法务书面确认（回答下列 4 个问题）；② 宿主 App（fx-sport / ab77 / android）隐私政策和用户协议写明"为处理举报、配合监管，平台可能查看消息内容"并已发布；③ 完成个人信息保护影响评估（《个人信息保护法》第 55、56 条，报告保存 ≥ 3 年）；④ 后台操作规程和人员培训记录。由 Frank 书面通知后，运维修改 im-server 配置并重启；该变更本身记入变更记录 |
| 群消息正文查看、删除消息与隔离原文（第二批） | 上面 ②、③、④ 完成（X-7 已拍板按 ② 开放，不需要单独的法务批复，但需要告知与评估） |
| 管理后台第一批 | 渗透测试通过；元数据查询、在线设备查看的操作规程（理由怎么填、多久复查一次审计）发给客服和审核 |

**需要法务回答的问题**（Frank 转交，不阻塞第一批）：
1. 单聊、群聊正文是否允许由内部人员查看，前提条件是什么（举报、监管要求、工单）。
2. 宿主 App 的隐私政策和用户协议需要怎么写。
3. 是否必须做个人信息保护影响评估，由谁做。
4. IM 运营方和宿主 App 运营方之间是否构成委托处理关系（《个人信息保护法》第 21 条），是否需要签协议。

另：被删原文隔离保存 30 天（X-9）、审计保留期（X-4）都已拍板；法务如要求更长，修改 `IM_QUARANTINE_DAYS`、`CON_AUDIT_ARCHIVE_YEARS` 即可，不需要改需求。

---

## 13. 不在本期范围与已接受的风险

### 13.1 不在本期范围

| 功能 | 计划 |
|---|---|
| 运维能力（节点、指标、配置检查、备份记录等） | PRD-ops v0.2（共用第 4 节基座） |
| 直播聊天室管理（房间列表、房间禁言、房间推送） | 随二期聊天室 |
| 自定义角色、按群 / 数据范围授权 | P1 |
| 公司 SSO 登录（账号表已预留 `idp_subject`） | P1 |
| 双人复核：超管重置另一超管、批量封禁 | P1（全员通知的双人复核已在第二批） |
| 消息、用户、审计导出 | P1（需 step-up、水印和审批） |
| 头像图片代理 | P1（一期显示首字母占位） |
| 群管理员的后台设置 | P1 |
| 跨会话全文搜索 | P2 |
| 举报工单流（X-7 收紧为 ③ 的前提） | P2 |
| 多应用 / 多租户 | 不做（S-4 ①：一个部署 = 一个业务方；一个 console-server 对应一个 im-server 部署） |
| 移动端适配 | 不做 |
| 用户注销与数据删除、消息保留期（ARCH S13） | 单独 PRD |

### 13.2 已接受的风险

| 风险 | 缓解 | 残余 |
|---|---|---|
| 空间超管截留新账号的激活码，冒充他人激活 | 激活的 IP / UA 写审计；激活即通知全部空间超管和审计员；账号对应真人、定期核对 | SSO（P1）后消除 |
| console-server 被攻破 | 主凭证不能签 token、不能读正文；正文凭证受 im-server 侧每日配额限制（200 个会话 / 天）且 im-server 侧有审计；单聊正文由 im-server 开关兜底 | 攻击者可以在 console scope 内执行写操作、读元数据，直到被发现 |
| 审计尾部截断（发生在最近一次锚定之后，最多 1 小时或 1000 条） | 每小时 + 每 1000 条锚定；im-server 侧 `admin_audit` 可交叉核对 | 窗口内的截断只能靠交叉核对发现 |
| im-server 重启后 5 分钟内的签名请求重放 | 时间戳窗口 5 分钟；管理端口在内网 + TLS | B6 后 nonce 进 Redis 消除 |
| 水印可以被开发者工具去掉 | 水印仅用于追溯；授权 ID 可定位到具体一次查看 | — |
| 已删除消息的原文仍在备份中（最多 48 小时） | 文案用"不再显示"；备份加密（ARCH S10） | — |
| 已展示的系统推送通知无法撤回 | 接入文档写明 | — |
| 消息保留期未实现，所有消息（含已解散群）永久保存 | 列入合规风险清单；S13 单独 PRD | 与最小必要原则存在差距 |
| 未认证的拒绝不进审计 | 写日志 + 指标 + 告警阈值 | 需要从日志平台追查 |

---

## 14. 工作量重估（人日）

| 部分 | 第一批（含前置） | 第二批 | 合计 | 说明 |
|---|---|---|---|---|
| 服务端（im-server） | 15（F-KEY 4.5 + 搜索 1.5 + 在线设备与连接元数据 2 + 群列表 1.5 + 成员分页 1 + 解散 2 + 元数据查询 1.5 + 契约测试与迁移 1） | 19（封禁 2 + 禁言 4 + 删除 3 + 隔离与定时任务 2 + 正文授权与配额 2 + 推送 5 + Schema / errors.json 1） | **34** | 社交管理接口已计入 PRD-social；push 协议设计不含 |
| console-server | 28.5（基座 21：骨架 / 迁移 / 部署 2.5、账号空间 2、登录与 TOTP 2.5、锁定限流 1.5、会话与 CSRF 1.5、角色与应急授权 1.5、step-up 0.5、审计两阶段 / 哈希链 / 锚定 / 校验 / 对账 3.5、审计查询与归档 1.5、安全头 0.5、密钥材料 1、通知 1、im-server 签名客户端与测试模式 1；业务 7.5：用户 2.5、社交 1.5、群 2.5、元数据 1） | 8（封禁 1、正文授权 1.5、删除与隔离 1.5、禁言 1、推送草稿 / 批准 / 记录 3） | **36.5** | 基座 21 人日由运维后台复用，PRD-ops 不再重复计算 |
| 前端（管理后台） | 18（基座页面 8：框架、登录 / 激活 / 绑定、账号管理、审计、通知、个人设置、错误码处理；业务 10：用户 3、社交 2、群 3.5、消息 1.5） | 8.5（封禁 1、正文与水印 2、删除与隔离 1.5、禁言 1.5、推送 2.5） | **26.5** | 基座页面的组件运维后台可复用一部分 |
| SDK 与客户端 | 1（sdk-ts + H5：`convRemoved.reason`、忽略未知） | 10（sdk-ts + H5 4、sdk-kotlin 3、sdk-swift 3） | **11** | 原生部分随 M5 / M6 首版 |
| 测试 | 13（F-KEY 安全用例 1.5、基座安全用例 5、第一批 e2e 4、性能 1、渗透测试配合 1.5） | 6（第二批 e2e 5、性能 1） | **19** | 外部渗透测试费用另计 |
| **合计** | **75.5** | **51.5** | **127** | v0.1 未给出；评审粗估 47～60 人日（未含推送、测试、审计与签名的完整实现） |

依赖（不计入）：Postgres 本机安装授权（X-3）；法务回复（X-7，不阻塞第一批）；二期 push 协议（阻塞 F-ADM-50）。

---

## 15. 待拍板与确认事项

**新增待拍板：无。** A～D 节 14 项、2.5 节、E 节都已按建议写入。

| # | 确认项 | 本 PRD 的处理 | 何时需要 |
|---|---|---|---|
| Q-1 | X-5 说"后台服务的凭证不能读消息正文"，X-7 又允许按 ② 查看群消息。本 PRD 的理解：**console 主凭证**永远不能读正文；群消息查看用**单独的 `console-msgread` 凭证**（只有正文类 scope），每次查看都要 im-server 侧授权、受 im-server 侧每日配额限制并留 im-server 侧审计（F-KEY-02 规则 3、4，F-ADM-08） | 若 Frank 认为 X-5 是绝对的（console-server 不得持有任何读正文的凭证），群消息查看要改为另一个独立部署的小服务持有凭证，第二批多约 3 人日 | 第二批开工前；不阻塞第一批 |

仍在进行中的既有事项（不是新拍板）：X-3 Postgres 本机安装授权（console-server 开工前）；X-7 法务确认（第 12 节问题清单）。

---

## 16. 变更记录

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v0.1 | 2026-11-03 | 初稿：角色与 RBAC、后台登录与安全（F-ADM-01～09）、用户 / 群 / 消息 / 禁言 / 系统通知 / 敏感词 / 概览功能点、服务端缺口 G-ADM-01～20、部署方案对比、待拍板 D-ADM-01～08 | 产品（AI） |
| v0.2 | 2026-11-03 | 按 REVIEW-admin-v0.1（阻塞 7 / 重要 20 / 建议 8）和 DECISIONS（X-1～X-9、S-3、S-9、S-11）修订：新增第 3 节 X-5 前置改造（F-KEY-01～05）；新增第 4 节 console-server 公共基座（F-CON-01～12，两个后台的唯一规格，取代 v0.1 的 F-ADM-01～04、06、07、09）；角色可叠加、超管最小权限、应急授权；审计两阶段 + 哈希链 + 外部锚定；step-up 改为 403 / 4038；错误码按号段重排（14 个新码）；分两批并标注每个功能点；新增在线设备查看（F-ADM-14）、社交管理（F-ADM-15～17）、群主交接（F-ADM-25）、隔离原文（F-ADM-32）；单聊正文 im-server 开关默认关闭；PRD-core v1.3 / ADMIN.md 变更条目；工作量重估 127 人日 | 产品（AI） |
| v0.2（复审后修订） | 2026-11-03 | 按 [一致性复审](../review/RECHECK-v0.2-2026-11-03.md) 第 6.1 节 A-1～A-15 修订（不升版本号），见 17.6 节 | 产品（AI） |
| v1.1 | 2026-11-05 | 落实 [PRD-live v1.0](PRD-live-v1.0.md) 第 11 节（L-17，Frank 2026-11-05 拍板）：F-ADM-50 规则 3 指向已冻结的 push 协议，`pushId` 全局唯一，去重范围明确；规则 5 同号重放不受 5 分钟间隔约束；规则 8 增加 `target=room` + `roomId`（接口 L2 开放）和可选 `ttlSeconds`；5040 增加系统通知的例外和前端文案；F-KEY-02 规则 2 增加 `push.room`，AC-KEY-02-2 增加对应样本；规则 5 业务方可持有 `push.all` 的三项必选条件；调用方配置增加 `pushTypes`；5.2 节补 `push.records.read`。AC-ADM-50-1～8 的预期不变。F-ADM-50 标题和第 139 行的“阻塞于 push 协议”、验收标准的“草案”字样不改原文，以本条记录为准：协议已定稿 | 产品（AI） |

---

## 17. v0.1 → v0.2 修订记录

### 17.1 评审意见逐条处理（REVIEW-admin-v0.1 第 2 节）

| 编号 | 严重度 | 处理 | 处理方式 / 理由 | 位置 |
|---|---|---|---|---|
| R-ADM-01 | 阻塞 | **已改** | X-3 已拍板 Postgres：`auth` / `audit` / `biz` 三个 schema；`console_app` 对审计只有 INSERT + SELECT，启动时探测 UPDATE 必须失败；会话与失败计数在库里，多实例一致；纳入 ARCH 3.4 备份 | F-CON-01 规则 5～7、9；AC-CON-01-4、01-5、04-8、08-3 |
| R-ADM-02 | 阻塞 | **已改** | step-up 改为 403 / 4038；正文授权缺失或过期统一 403 / 4039；前端只在 4014 时跳登录页，4038 弹框后自动重放；AC 改为"状态码 + 错误码 + 前端行为" | 2.3 节；F-CON-07；F-ADM-08；10.1 节 |
| R-ADM-03 | 阻塞 | **已改** | 按号段重排：401x 只放 401 语义（4014～4017），锁定 4291（429），CSRF / step-up / 授权 4037～4039（403）；新增 `scope: console`，SDK 生成器跳过；4050 取消（P2 届时建议 4510）；4007 补 `http: 400` 和 admin / console 通道；与 errors.json、PRD-social 逐码核对 | 第 10 节 |
| R-ADM-04 | 阻塞 | **已改** | X-5 写成前置改造 F-KEY-01～05：`/admin/*` 迁到内网监听地址；按调用方分密钥 + scope；console 主凭证不含 `token.issue` 和任何正文 scope；正文类 scope 只能在独立调用方上；单聊正文受 im-server 开关控制；旧密钥只能调 v1.2 接口、生产默认关闭；im-server 侧审计覆盖所有调用方（含旧密钥）；"被攻破的后果"如实写进 13.2 节 | 第 3 节；13.2 节 |
| R-ADM-05 | 阻塞 | **已改** | X-7：单聊正文法务确认前不可查看，im-server 开关默认关闭且不能经接口打开；第 12 节列上线门槛（法务、宿主隐私政策、影响评估、操作规程）和 4 个法务问题；群消息查看放第二批 | F-KEY-02 规则 3；F-ADM-08；第 12 节 |
| R-ADM-06 | 阻塞 | **已改** | X-6 已拍板分两批；第 11.2 节逐条列出对 PRD-core 的变更（VA-01～15，含 D1 / 第 7 节）、11.3 节 ADMIN.md、11.4 节 ARCH S8；系统通知放第二批最后并注明阻塞于 push 协议，AC 标为草案 | 2.1、2.2 节；第 11 节；F-ADM-50 |
| R-ADM-07 | 阻塞 | **已改** | 新设第 4 节"console-server 公共基座"作为两个后台的唯一规格（部署、账号空间、登录、锁定、会话、角色、step-up、审计、保留、浏览器安全、密钥、通知），PRD-ops v0.2 引用；位置按 Frank 的安排放在本 PRD 而不是 ARCH 第 8 节 | 第 4 节；11.4 节 |
| R-ADM-08 | 重要 | **已改** | 两阶段审计（intent 失败不执行；result 失败标"结果未知"并对账）；链头 `SELECT … FOR UPDATE` 串行化；外部锚定提前到 P0（每小时 + 每 1000 条）；AC 增加截断尾部、整链重算、并发写 | F-CON-08、09；AC-CON-08-4～10 |
| R-ADM-09 | 重要 | **已改** | 失败计数按"账号名 + IP"，不管账号是否存在，锁定提示对不存在的账号相同（4291）；账号名全局软上限 50 次 / 小时；命令行应急解锁；生产强制 CIDR | F-CON-04；AC-CON-04-3、04-4、04-6 |
| R-ADM-10 | 重要 | **部分** | 已改：新建和重置改为一次性激活码，不再由超管设初始密码；激活与重新绑定通知全部空间超管和审计员；超管默认没有正文、删除、全员推送、审计权限，改为有时限的应急授权；去掉超管的 `audit.read`。**未完全解决**：超管仍能截留激活码（13.2 节已接受风险，SSO 后消除）；"超管重置另一超管需第二人确认"放 P1 | F-CON-02、06；5.1、5.2 节 |
| R-ADM-11 | 重要 | **已改** | step-up 失败计入锁定，同一会话连续 5 次失败注销会话并锁定账号；正文授权每次都要当次动态码、不享受 5 分钟窗口；TOTP 防重放覆盖登录和 step-up | F-CON-03 规则 3；F-CON-07 |
| R-ADM-12 | 重要 | **已改** | `X-Console-Background: 1` 的请求不续期；页面不可见暂停刷新；AC 覆盖 | F-CON-05 规则 3；AC-CON-05-5、AC-ADM-70-2 |
| R-ADM-13 | 重要 | **部分** | 已改：单聊元数据查询要理由 + 审计 + 每日 50 个上限；群元数据查询写审计；在线设备独立为 F-ADM-14，要理由 + 审计 + IP 脱敏 + 每日上限，与运维后台共用查询函数和脱敏规则。**未采纳**"客服只能从用户详情进入"：客服处理投诉常需按会话 ID 直接查，用理由 + 审计 + 上限代替入口限制 | F-ADM-14、30 |
| R-ADM-14 | 重要 | **已改** | 推送 `data` 全文存 `biz.push_records`，审计存长度 + SHA-256 可对上；审计员和超管可查看 | F-ADM-50 规则 7；AC-ADM-50-8 |
| R-ADM-15 | 重要 | **已改** | 不加载外部头像，首字母占位（代理为 P1）；CSP 写全（`style-src` nonce，组件库选型条件，`script-src` 不放宽）；接口 `Cache-Control: no-store`；全部响应头有 AC | F-CON-10；AC-CON-05-14、10-3、10-5 |
| R-ADM-16 | 重要 | **已改** | `CON_DATA_KEYS` 信封加密 + kid 轮换命令；恢复码 / 激活码 / 会话 ID 只存哈希；Argon2id 参数与并发信号量；纳入启动保护；恢复码剩余提示与重新生成 | F-CON-11；F-CON-03 规则 4 |
| R-ADM-17 | 重要 | **已改** | `auth` ack 带用户级 `mutedUntil`，`convs` 群会话带 `mutedUntil`；AC 覆盖离线被禁言 | F-ADM-40 规则 4、F-ADM-41 规则 5；AC-ADM-40-6、41-6 |
| R-ADM-18 | 重要 | **已改** | 11.1 节逐处写 Schema 改法（`Error.details`、`body` 条件必填、事件加入 `oneOf`）；SDK 忽略未知事件 / 字段 / 枚举值并加一致性测试；发布顺序与服务端开关 | 11.1 节 |
| R-ADM-19 | 重要 | **部分**（期限不采纳） | 隔离原文 G-ADM-19 与删除消息同在第二批，同一事务写入，不存在衔接缺口；文案"已删除，不再显示"。保留期按 X-9 已拍板的 30 天，**不采纳**"不少于 6 个月"的建议；改为可配置，法务要求更长时改 `IM_QUARANTINE_DAYS` | F-ADM-31 规则 9；F-ADM-32 |
| R-ADM-20 | 重要 | **已改** | 生产强制 `*_ALLOWED_CIDRS` 且不能是全网段；只信任可信代理的 `X-Forwarded-For`；AC 覆盖伪造 | F-CON-01 规则 3、9；AC-CON-01-1、01-2、01-4 |
| R-ADM-21 | 重要 | **已改** | 8.1 节逐项标"接口 / 迁移 / 协议 / 设施"：Presence 不存在要补连接元数据；索引与 `LIKE` 转义；解散的读路径清单；禁言缓存走 `Invalidator`；定时任务机制（G-ADM-22）；`admin_audit` 在 SQLite 阶段的只追加方式；nonce 集群进 Redis；封禁覆盖 `/demo/login`；8.3 节给出实现顺序 | 第 8 节 |
| R-ADM-22 | 重要 | **已改** | S-3 之后第一批本来就改协议；解散群留在第一批并带 `reason: dissolved`，SDK 对未知 `reason` 按 `removed` 处理，老客户端显示"你已不在该群" | F-ADM-24 规则 7；2.2 节规则 1 |
| R-ADM-23 | 重要 | **已改** | 已删除的消息仍计入未读（C-02 不变）；`conv_seq` 摘要同事务更新；AC 分 sdk-ts 内存和 sdk-kotlin / sdk-swift 本地库（2.5 节不用 KMP）；写明已展示的推送通知无法撤回 | F-ADM-31 规则 3、4、6；AC-ADM-31-3、31-4 |
| R-ADM-24 | 重要 | **已改** | 全部 AC 改为"输入 / 操作 / 预期 / 错误码"；补齐评审列出的缺失 AC：bootstrap 拒绝（AC-CON-02-1）、锁定通知（AC-CON-12-1）、恢复码重新生成（AC-CON-03-9）、超管互相操作（AC-CON-02-7、02-8）、刷新后取 CSRF 令牌（AC-CON-05-10）、禁言 / 封禁不存在的用户（AC-ADM-40-7、13-8）、给非成员群内禁言（AC-ADM-41-5）、全员推送多端语义（F-ADM-50 规则 3）；矩阵和"需要理由"做成参数化 AC（AC-ADM-05-1、05-2） | 第 3～6 节 |
| R-ADM-25 | 重要 | **已改** | 每条时间相关 AC 标 [C 钟] / [IM 钟]；`tools/set-clocks.sh`；console 显示封禁 / 禁言状态只用 im-server 的时间 | 2.4 节 |
| R-ADM-26 | 重要 | **已改** | 保留期按 X-4（在线 1 年 + 加密归档 3 年），两个后台统一；法务要求不同时改配置；S13 未实现导致消息永久保存列入 13.2 节风险 | F-CON-09 规则 6；13.2 节 |
| R-ADM-27 | 重要 | **已改** | 全员推送改为"运营发起 + 超管批准"双人复核（P0 最小实现）；间隔改为全局 5 分钟并有 AC | F-ADM-50 规则 5；AC-ADM-50-5、50-7 |
| R-ADM-28 | 建议 | **已改** | 一步提交账号 + 密码 + 动态码，统一报错，与 PRD-ops 一致 | F-CON-03 规则 1、2 |
| R-ADM-29 | 建议 | **已改** | 未认证的拒绝只写日志和指标；已认证的拒绝按账号每分钟 20 条后合并 | F-CON-06 规则 3；AC-CON-06-4、06-5 |
| R-ADM-30 | 建议 | **已改** | console 调用方用 Ed25519（im-server 只存公钥），业务方可选 HMAC；签名覆盖 `X-IM-Operator` 和 `X-Request-Id` | F-KEY-03 规则 2 |
| R-ADM-31 | 建议 | **已改** | 新增 `kicked.reason = "suspended"`（带 `until`），关闭码仍为 4012，老 SDK 兼容；业务方签发失败时还有 4034 | F-ADM-13 规则 2；10.2 节 |
| R-ADM-32 | 建议 | **已改** | 性能 AC 写明 Postgres 环境，SQLite 环境另给缩小规模的目标；统计口径为 3 次 P95 的中位数 | 第 9 节；AC-ADM-90-1～4 |
| R-ADM-33 | 建议 | **已改** | 用户导入、加减群成员、好友导入都由 console-server 拆批，先查总量、一条汇总审计 | F-ADM-11、15、22 |
| R-ADM-34 | 建议 | **已改** | userId 区分大小写；昵称 / 群名前缀匹配 ASCII 不区分大小写；写进 AC | 2.3 节；AC-ADM-10-1、10-2 |
| R-ADM-35 | 建议 | **已改** | 注明水印只用于追溯；AC 改为检查 DOM；水印含授权 ID 前 8 位 | F-ADM-08 规则 6；AC-ADM-08-10 |

### 17.2 评审对待拍板项的意见（第 3 节）与跨 PRD 一致性（第 4 节）

| 项 | 处理 | 说明 |
|---|---|---|
| D-ADM-01（评审建议两个进程分开部署） | **不采纳**（以拍板为准） | X-1 ② 已拍板"一个服务同时服务两个后台"。缓解：两个监听地址、两个域名、两个账号空间，`CON_SPACES` 允许把同一份代码部署成两组实例（F-CON-01 规则 1、2） |
| D-ADM-02 | 已改 | X-2：自建 + 强制 TOTP，预留 `idp_subject`，每个空间保留本地应急超管 |
| D-ADM-03 | 已改 | X-6：分两批（因 S-3 社交管理与解散群进第一批），系统通知第二批最后 |
| D-ADM-04 | 已改 | X-7：单聊默认 ①（im-server 开关），群消息 ②；法务问题清单见第 12 节 |
| D-ADM-05 | 已改（期限不采纳） | X-9：加密隔离 30 天；可配置延长 |
| D-ADM-06 | 已改 | X-8：两边都可以封禁；4034 写入接入文档 |
| D-ADM-07 | 部分 | 按评审意见：投递语义交给二期 push 协议评审定，本 PRD 只给建议输入（F-ADM-50 规则 3） |
| D-ADM-08 | 已改 | X-4：在线 1 年 + 加密归档 3 年，两个后台统一 |
| D-ADM-09～14（评审新增） | 已改 | 分别由 X-3、X-5、R-ADM-10 的处理、X-2 + 第 4 节、X-6、X-7 落实，不再单列拍板项 |
| X-01 部署形态 | 已改 | 统一为 console-server（进程名、`CON_` 前缀、`console-server` 命令） |
| X-02 角色模型 | 已改 | 可叠加角色、按空间隔离；`auditor` 不能与其他角色叠加；审计员跨空间查看规则（F-CON-09 规则 1） |
| X-03 审计规格 | 已改 | 一份规格（F-CON-08、09）：哈希链 P0、写失败策略、字段、31 天、游标分页、保留期 |
| X-04 登录与 CSRF | 已改 | 一步登录、统一文案、`X-CSRF-Token` + Origin、数字错误码、密码历史 5 次、`console-server account bootstrap` |
| X-05 在线连接两套实现 | 已改 | im-server 一个查询函数、两个入口；隐私口径统一为理由 + 审计 + 脱敏 + 上限 |
| X-06 PRD-ui 缺口 | 已改 | 11.4 节列为 PRD-ui v0.2 待补项 |
| X-07 PRD-core 约束变更 | 已改 | 11.2 节 VA-01～15 |
| X-08 ARCH | 已改 | 11.4 节 |
| X-09 错误码 | 已改 | 第 10 节 |
| X-10 PLAN 接口对齐 | 已改 | `/admin/mute` 用 `scope` 区分用户级 / 群成员级；push 的 `target` 先做 `user` / `all`，`room` 随聊天室 |

### 17.3 v0.1 功能点与新编号的对应

| v0.1 | v0.2 |
|---|---|
| F-ADM-01 登录 | F-CON-02（激活）、F-CON-03（登录） |
| F-ADM-02 会话 | F-CON-05 |
| F-ADM-03 密码与锁定 | F-CON-04 |
| F-ADM-04 账号与角色管理 | F-CON-02、F-CON-06 |
| F-ADM-05 RBAC | F-CON-06（机制）+ F-ADM-05（管理后台矩阵） |
| F-ADM-06 审计 | F-CON-08、F-CON-09 |
| F-ADM-07 二次确认 | F-CON-07 + 5.3 节 |
| F-ADM-08 脱敏与敏感查看 | F-ADM-08（群消息正文，第二批）、F-ADM-14（在线设备）、F-ADM-30 规则 3（单聊元数据） |
| F-ADM-09 浏览器安全 | F-CON-10、F-CON-01 规则 3 |
| G-ADM-03 / 04 / 20 | F-KEY-01～05 / F-KEY-04 / F-CON-01 规则 10 |
| 新增 | F-KEY-01～05、F-CON-01、11、12、F-ADM-14～17、25、32 |

### 17.4 AC 抽查中标出问题的条目（评审 2.4 节）

| v0.1 AC | 处理 | v0.2 |
|---|---|---|
| AC-ADM-01-2 | 已改 | AC-CON-02-4（`setup` 会话调业务接口 401 / 4016） |
| AC-ADM-01-3 | 已改 | AC-CON-03-2（响应完全一致）+ AC-CON-03-3（各 50 次比中位数、隔离环境、单元测试断言假哈希） |
| AC-ADM-01-4 | 已改 | AC-CON-03-4（错误码、计入失败次数、覆盖 step-up） |
| AC-ADM-01-5 | 已改 | AC-CON-03-5（标 [C 钟]） |
| AC-ADM-01-7 | 已改 | AC-CON-02-4【后端】+ AC-CON-02-5【前端】 |
| AC-ADM-01-8 | 已改 | AC-CON-03-6（审计 + 通知本人） |
| AC-ADM-02-3 | 已改 | AC-CON-05-3、05-5（写明哪些请求续期） |
| AC-ADM-02-5 | 已改 | AC-CON-03-7（预设 Cookie X） |
| AC-ADM-02-7 | 已改 | 拆为 AC-CON-05-7、05-8、05-9 |
| AC-ADM-02-9 | 已改 | AC-CON-05-11 + 05-12（前端提示） |
| AC-ADM-02-10 | 已改 | 2.3 节前端处理规则 + AC-CON-07-3（4038 不跳登录页） |
| AC-ADM-03-1 | 已改 | AC-CON-04-1（弱密码表来源与版本固定） |
| AC-ADM-03-3 | 已改 | AC-CON-04-3、04-4（不泄露存在性） |
| AC-ADM-03-5 | 已改 | AC-CON-04-7（`login` + `activate` 都计入，一步登录没有单独的 TOTP 请求） |
| AC-ADM-03-8 | 已改 | AC-CON-03-8（扫描范围含反向代理日志） |
| AC-ADM-04-5 | 已改 | AC-CON-02-7（含有两个超管时）、02-8（并发），错误码 4093 |
| AC-ADM-05-1 | 已改 | AC-ADM-05-1（每个路由一个合法样本，判"成功状态码"） |
| AC-ADM-05-2 | 已改 | AC-CON-06-1（定为启动失败） |
| AC-ADM-06-4 | 已改 | AC-CON-08-5～7 |
| AC-ADM-06-5 | 已改 | AC-CON-08-4（两阶段方案确定后） |
| AC-ADM-07-1 | 已改 | AC-CON-07-1（403 / 4038） |
| AC-ADM-07-4 | 已改 | AC-ADM-05-2（参数化覆盖 5.3 节全部需要理由的操作） |
| AC-ADM-08-2 | 已改 | AC-ADM-08-2（逐情形对应错误码） |
| AC-ADM-08-3 | 已改 | AC-ADM-08-4（统一 4039） |
| AC-ADM-08-5 | 已改 | AC-ADM-08-10（检查 DOM） |
| AC-ADM-09-1 | 已改 | AC-CON-10-1（同时搜 IM 凭证和 console 私钥） |
| AC-ADM-09-4 | 已改 | AC-CON-01-1、01-2（伪造 `X-Forwarded-For`） |
| AC-ADM-13-4 | 已改 | 标 [IM 钟] |
| AC-ADM-24-5 | 已改 | AC-ADM-24-5（H5）+ 规则 7（原生 SDK 删本地库，M5 / M6 验收） |
| AC-ADM-30-3 | 已改 | AC-ADM-30-3（4007，admin / console 通道 http 400）+ 30-4（不存在返回空列表） |
| AC-ADM-31-3 | 已改 | AC-ADM-31-3（sdk-ts 内存）+ 31-4（原生本地库） |
| AC-ADM-50-1～3 | 部分 | 保留为草案，随 push 协议修改（R-ADM-06） |
| AC-ADM-50-5 | 已改 | AC-ADM-50-5（全局间隔、错误码 4290） |

### 17.5 统计

| 项 | v0.1 | v0.2 |
|---|---|---|
| 评审意见 | — | 第 2 节 35 条：已改 32、部分 3（R-ADM-10、13、19）、不采纳 0；阻塞 7 条全部已改。第 3 节对拍板项的意见中不采纳 1 条（D-ADM-01 拆成两个进程，以 X-1 拍板为准） |
| 功能点 | 26 | 42（第一批 32 = 前置 5 + 基座 12 + 业务 15；第二批 7；P1 / P2 3） |
| AC | 129 | 278（第一批 214，含性能 4；第二批 58；P1 / P2 6） |
| 新错误码 | 9（编号与号段规则冲突） | 14（逐码核对无冲突） |
| 待拍板 | 8 | 0（1 个确认项 Q-1） |

### 17.6 一致性复审后修订（2026-11-03）

依据 [RECHECK-v0.2-2026-11-03](../review/RECHECK-v0.2-2026-11-03.md) 第 6.1 节，直接在 v0.2 上修改，不新建版本。15 条全部照做。

| 清单编号 | 对应复审问题 | 怎么改的 |
|---|---|---|
| A-1 | C-01 | F-CON-06 规则 4 改为"各空间声明互斥角色，互斥角色不能与同空间任何其他角色叠加（409 / 4093）；管理空间 `auditor`，运维空间 `ops_auditor`、`ops_superadmin`"；AC-CON-06-6 不变；新增 AC-CON-06-10（ops 账号已有 `ops_engineer` 时再授予 `ops_superadmin` / `ops_auditor` 均 409 / 4093）；10.1 节 4093 的含义同步为"互斥角色与其他角色叠加" |
| A-2 | N-1 | F-CON-06 规则 6 开头加"只在管理空间提供（权限点 `admin.emergency`），运维空间不提供"；末尾加"运维空间的应急授权接口不注册（调用返回 403 / 4030）；运维超管要获得排障权限只能由另一名运维超管授予 `ops_engineer`" |
| A-3 | C-01 | F-CON-06 规则 2 末尾加"各空间的权限点由各自 PRD 定义（管理空间第 5.2 节，运维空间 PRD-ops F-OPS-02 规则 1）" |
| A-4 | C-05 | F-CON-05 规则 2 Cookie 名改为"管理空间 `__Host-adm_sid`，运维空间 `__Host-ops_sid`"；AC-CON-03-1、03-7、05-1 中的 `__Host-con_sid` 改为 `__Host-adm_sid` |
| A-5 | C-06 | F-CON-05 规则 3 末尾补：用户操作发起的请求不得带 `X-Console-Background`；经认证代理转发到观测组件（Grafana）的请求只有 `Sec-Fetch-Mode: navigate` 续期；代理 Cookie 绑定原会话 |
| A-6 | C-07 | F-CON-02 规则 7 末尾补"对另一空间账号 ID 调用任何账号接口一律 404 / 4040，并写 `result=denied`、`code=4040` 审计"；AC-CON-02-6 预期补审计断言 |
| A-7 | C-10 | F-CON-02 规则 2、F-CON-04 规则 6、F-CON-08 规则 2 的命令行操作人统一为 `cli:{操作系统用户}@{主机名}`；bootstrap 的 action 写明 `account.bootstrap`，命令行解锁写明 `account.unlock` |
| A-8 | C-11 ① | F-CON-12 规则 2 末尾补"各空间 PRD 可追加触发条件，接收人按'空间 + 角色'指定；通知接口支持发给某空间某角色的全部启用账号" |
| A-9 | C-11 ②③ | F-CON-01 规则 4 补"可信代理终止 TLS 时必须带 `X-Forwarded-Proto: https`，否则 403 / 4030"；规则 9 补"各空间 PRD 追加的生产检查（PRD-ops B-OPS-15）一并执行，一次列出全部问题后退出" |
| A-10 | C-11 ④ | F-CON-10 规则 4 末尾补"各空间可收紧不可放宽；运维空间 `img-src 'self'`、`style-src 'self'`，加 `X-Frame-Options: DENY`" |
| A-11 | D-03、D-04 | 2.3 节"理由"行补"运维排查接口的说明字段同样叫 `reason`、规则相同"，新增"每日上限"行（Asia/Shanghai 自然日，429 / 4292）；F-ADM-14 规则 2 脱敏改为 IPv4 前 2 段、IPv6 前 3 组（48 位），im-server 的连接元数据和断开缓冲只存脱敏值，字段取并集（连接 ID、平台、建立时间、节点、脱敏 IP、下行队列状态）；规则 4 每日上限注明自然日与 429 / 4292；G-ADM-06 同步"只保存脱敏后的值" |
| A-12 | D-06 | F-KEY-03 规则 1 加可选 `X-IM-Target-Node`；规则 2 待签名串加第 8 行 `X-IM-Target-Node`（没有则空串），并写明节点运维接口复用本规则（`console-ops`、Ed25519、每节点一把私钥、`X-IM-Target-Node` 必填且等于 `IM_NODE_ID`、失败 401 / 4017）；F-CON-01 规则 8 第二句改写；11.4 节 PRD-ops 行同步（另补 IP 脱敏口径、互斥角色、无应急授权）；10.1 节 4017 含义补"含节点运维接口、目标节点不符" |
| A-13 | D-10 | F-KEY-01 规则 4 改为 `IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的两两不同；AC-KEY-01-5 增加"`IM_ADMIN_ADDR` 与 `IM_OPS_ADDR` 相同"一组 |
| A-14 | E-04～E-08 | 10.1 节 4034 说明补"也用于 `/demo/login`，scope 保持 `admin`"；号段说明改为"HTTP 以 `http` 字段为准，413x 用 400 为既有例外"。10.2 节选定**透传规则**一种写法：表尾加"透传规则"行，首行改为只给 console-server 自己也会产生的码（4000、4004、4030、4040、4290、5000、5030）加 `console`，只透传的码不加；表中加 `closeCodes` 1011 行。10.4 节加第 5～7 条（对服务端的变更：`TestCodesMatchErrorsJSON` 跳过只含 console scope 的码、console-server 仓库自测；对 protocol 的变更：`closeCodes` 增加 1011 `INTERNAL_ERROR`；413x 用 400、ADMIN.md 改为"取 `http` 字段"）；VA-14 同步 im-server 测试跳过 console 码。只写变更条目，未改代码和 errors.json |
| A-15 | D-05、D-09 | 文首"背景材料"中 PRD-ops v0.1 改为 v0.2（并加复审链接），版本行注明已按复审修订；4.0 节"空间超管"行改为 `ops_superadmin`（PRD-ops F-OPS-02）；F-ADM-14 规则 5 前加"B6 之前只返回接收请求的节点上的连接，标 `partial: true`、`scope: "localNode"`；B6 之后经 Presence 汇总"；AC-ADM-14-6 的"（双节点）"改为"[依赖 B6]" |
| 自查补充 | 自查 | 10.3 节结论中 PRD-ops 使用的码补 4017（节点接口签名）、4292（每日排查上限），并注明 PRD-ops 不新增错误码 |

**统计变化**：AC 新增 1 条（AC-CON-06-10），共 278 条（第一批 214）；新错误码仍为 14 个，无编号变化。

#### 冻结前对齐（2026-11-03）

- 2.3 节理由长度明确按 Unicode 码点计数（PRD-ops 一致性自查提出）。

#### 冻结前核对修正（2026-11-03）

按冻结前逐条核对发现的问题修正：F-CON-06 规则 6 运维超管不能被授予 `ops_engineer`（互斥角色），改为另建排障账号；应急授权接口在运维空间返回 403 / 4030 的写法；10.2 节 4130、4131 改为 console-server 自己产生的码；AC 统计 278 / 214；10.4 节测试文件路径；AC-CON-03-3 补测试前提。
