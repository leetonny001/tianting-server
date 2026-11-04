# console-server 任务单：公共基座与管理后台业务（PRD-admin）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-admin v1.0](../prd/PRD-admin-v1.0.md)（冻结）第 2.3、2.4 节、第 4 节 F-CON-01～12（**两个后台的唯一规格**）、第 5 节、第 6 节 console 侧部分、第 9 节；[PRD-ops v1.0](../prd/PRD-ops-v1.0.md) 第 4 节 B-OPS-01～15（运维侧对基座的需求）；[ARCH-v1](../arch/ARCH-v1.md)；[errors.json](../../protocol/errors.json) |
| 负责 | 后端（AI） |
| 仓库 | tianting-console-server（待建，Go；PRD 中称 `console-server`）。管理后台前端在 tianting-admin（见 [frontend-admin.md](frontend-admin.md)），由 console-server 托管其构建产物 |
| 编号 | **CS-001～CS-099 本单**（基座 CS-001～CS-019、管理后台业务 CS-020～CS-069）；**CS-100～CS-199 运维模块**（另一份任务单，复用本单基座，不重复实现） |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（Go 单测 / 集成测试连真实 Postgres、进程级测试、与 im-server 的 e2e）；`go test -race ./...` 全过；迁移可 `migrate up` 到空库；console 错误码一致性测试通过；对应 TC-CON / TC-ADM 用例通过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。

**前置条件（所有任务）**：**本地开发环境需要 Postgres**（P-5 已于 2026-11-04 授权并安装，17.11，库 `tianting_console_dev` / `tianting_console_test`）。在授权前只能做不连库的部分（配置解析、签名客户端、密码策略等纯函数），集成测试一律以"Postgres 可用"为依赖。CI 使用容器内 Postgres（GitHub Actions service container），不依赖本机安装。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| CS-001 ～ CS-004 | ✅ 2026-11-04（只在本地） | 代码在本机 tianting-console-server，**GitHub 仓库待 Frank 创建**（P-9），还没有 CI。空间与监听、网段与 HTTPS 边界、响应头、迁移与数据库角色、生产启动保护（一次列全）、数据密钥与轮换、Argon2id 限流、im-server 签名客户端；对真实 Postgres 和真实 im-server 的进程级测试。**等后续任务才能验收**：AC-CON-01-3、01-5（登录，CS-007、CS-009）；AC-CON-11-1 全库搜索、11-4 的 RSS（登录，CS-007）；10-1、10-4（前端，FE-400）。deploy/ 只有 systemd 单元和配置示例，定时器和运行手册随 CS-013、CS-015 |
| CS-005 ～ CS-011 | ✅ 2026-11-04（只在本地） | `285957b`。引导、激活、TOTP 防重放、恢复码、密码策略、锁定与 IP 限流（都在 Postgres）、会话与 CSRF、权限注册表与路由强制、step-up、应急授权、账号管理与自我保护；AC-CON-01-3、01-5、11-1、11-4 也已验收。**例外**：弱密码表是手写的暂行版（406 条），不是任务单要求的 SecLists 文件，见 P-10；`assets/weak-passwords.sha256` 随正式文件一起加。 |
| CS-012 审计写入 | 🟡 机制完成 | 两阶段、哈希链、只追加、故障注入完成（AC-CON-08-3、08-8 的小规模）。等业务路由（CS-020 起）才能验收 08-1、08-2、08-4、08-9、08-10、08-11；AC-ADM-90-4 性能压测未做 |
| CS-016 站内通知 | 🟡 基座完成 | 接口和激活、重置、锁定、恢复码、敏感角色、应急授权的触发完成；审计校验失败、锚定失败的触发随 CS-013 |
| CS-013 ～ CS-015、CS-017、CS-020 ～ CS-062 | ⬜ 未开始 | 依赖顺序见下 |

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| 基座（第一批） | CS-001 ～ CS-017 | 23 | 骨架与网络边界、迁移与启动保护、密钥材料、im-server 签名客户端、账号、登录、锁定、会话与 CSRF、角色与 step-up、审计（写入 / 锚定校验 / 查询对账 / 归档）、通知。**运维后台复用** |
| 管理后台业务（第一批） | CS-020 ～ CS-027 | 8.5 | 业务公共层与权限矩阵、用户、在线设备、导入、社交、群、元数据 |
| 管理后台业务（第二批） | CS-040 ～ CS-045 | 8 | 封禁、正文授权、删除与隔离原文、禁言、系统通知（最后） |
| P1 | CS-060 ～ CS-062 | （不计入两批） | 数据概览、审计导出、SSO |

**人日对照（console-server 部分）**

| 批次 | 本任务单 | PRD-admin 第 14 节 | 差异说明 |
|---|---|---|---|
| 第一批 | 31.5（基座 23 + 业务 8.5） | 28.5（基座 21 + 业务 7.5） | +3。① 基座 +2：PRD 列出的基座分项加起来其实是 20.5 而不是 21（见文末"发现的 PRD 问题" 1）；本单把审计拆成 4 个任务（写入 2、锚定与校验 1.5、查询与对账 1、分区与归档 1，合计 5.5，PRD 为 3.5 + 1.5 = 5，+0.5），登录与锁定 3.5（PRD 2.5 + 1.5 = 4，−0.5），账号 2.5（PRD 2，+0.5，跨空间与并发互斥单列 CS-006），新增 CS-017 密钥隔离扫描与 XSS 端到端 1（PRD 摊在"测试"里），其余小项 +0.5；② 业务 +1：新增 CS-020"业务公共层与管理空间权限矩阵"（理由 / 工单号 / 每日上限 / 错误透传 / 5.2 节矩阵参数化测试），PRD 把它摊在各模块里 |
| 第二批 | 8 | 8 | 一致 |
| 不在本单 | — | — | 社交管理接口的 im-server 实现在 PRD-social 任务单；SDK 与客户端、测试工作量（PRD 第 14 节"测试 13 / 6"）不在本单 |

依赖顺序：（Postgres 可用）→ CS-001 → CS-002 → CS-003、CS-004 → CS-005 → CS-006 → CS-007 → CS-008 → CS-009 → CS-010 → CS-011 → CS-012 → CS-013 → CS-014、CS-015 → CS-016、CS-017 →（im-server X-5：BE-200～BE-203）→ CS-020 → CS-021～CS-026 →（第二批 im-server：BE-222～BE-229）→ CS-040～CS-045。

### 运维侧扩展点（CS-100～CS-199 使用，基座只实现一次）

| 扩展点 | 基座提供 | 运维侧怎么用（PRD-ops） | 所在任务 |
|---|---|---|---|
| 账号空间 | `Space` 注册表：`admin`、`ops` 各自的监听地址、Origin、CIDR、Cookie 名、CSP 覆盖、生产检查函数 | 注册 `ops` 空间：Cookie `__Host-ops_sid`、更窄 CIDR、CSP 收紧（`img-src 'self'`、`style-src 'self'`、`X-Frame-Options: DENY`）（B-OPS-01、11） | CS-001 |
| 生产启动检查 | `StartupCheck` 接口，全部检查跑完后**一次列出全部问题**再退出 | 追加节点凭证表、Grafana 代理配置、运维网段检查（B-OPS-15） | CS-002 |
| im-server 调用 | `imclient` 签名客户端（Ed25519，8 行待签名串，`X-IM-Target-Node`） | 节点运维接口：调用方 `console-ops`、每节点一把私钥、`X-IM-Target-Node` 必填（F-OPS-16） | CS-004 |
| 权限点与角色 | `PermissionRegistry`（每空间声明权限点、角色 → 权限点、互斥角色、敏感角色、是否提供应急授权）；路由声明权限点，未声明启动失败 | 注册运维角色与权限点；互斥角色 `ops_superadmin`、`ops_auditor`；**不提供应急授权**（`admin.emergency` 不注册，调用返回 403 / 4030）（B-OPS-08） | CS-010、CS-011 |
| step-up 清单 | 路由声明 `stepUp`（`window` 或 `everyTime`） | 告警静默（第二批）、审计导出（P1）（B-OPS-07） | CS-011 |
| 会话续期 | 中间件 `RenewPolicy`：`X-Console-Background: 1` 不续期；可为代理路由设置"只有 `Sec-Fetch-Mode: navigate` 续期" | Grafana 认证代理（F-OPS-17）；代理 Cookie 绑定原会话 | CS-009 |
| 审计 | `audit.Writer`（intent / result / event、两阶段、哈希链全局一条）；`action` 枚举按空间扩展；查询接口支持 `space` 过滤，可见空间由权限决定 | 运维 action（F-OPS-14）；`ops_auditor` 可选"全部空间"（B-OPS-09、10） | CS-012、CS-014 |
| 站内通知 | `notify.ToSpaceRole(space, role, event)`、`notify.ToAccount` | 运维追加触发条件（F-OPS-02 规则 5、F-OPS-08 规则 9）（B-OPS-14） | CS-016 |
| 业务公共层 | 理由校验、工单号、每日上限（Asia/Shanghai，4292）、游标分页参数 | 运维排查接口的 `reason` 规则相同、每日排查上限 4292 | CS-020 |
| 前端入口 | 每个空间一个静态资源目录（`CON_ADMIN_WEB_DIR`、`CON_OPS_WEB_DIR`） | tianting-ops 构建产物 | CS-001 |

---

## 基座（第一批，F-CON-01～12）

### CS-001 仓库骨架、空间与监听地址、网络边界、安全响应头（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：建仓库（tianting-console-server，私有，GitHub）；本地开发环境需要 Postgres（P-5，集成测试用）
- AC：AC-CON-01-1、AC-CON-01-2、AC-CON-01-3、AC-CON-01-6、AC-CON-10-3、AC-CON-10-4、AC-CON-05-14
- 内容：
  1. 目录：`cmd/console-server`（子命令 `serve`、`migrate`、`account`、`audit`、`keys`）、`internal/{config,space,httpx,auth,audit,notify,imclient,admin}`、`migrations/`、`assets/`、`deploy/`；CI：`go test -race`、govulncheck、gosec、Postgres service container。
  2. 配置：`CON_ENV`、`CON_SPACES=admin,ops`（默认两个）、`CON_ADMIN_ADDR` / `CON_OPS_ADDR`、`CON_ADMIN_ORIGIN` / `CON_OPS_ORIGIN`、`CON_ADMIN_ALLOWED_CIDRS` / `CON_OPS_ALLOWED_CIDRS`、`CON_TRUSTED_PROXIES`、`CON_TEST_MODE`、`CON_TICKET_PATTERN`（默认 `^[A-Za-z0-9_-]{3,32}$`）。只开启的空间才监听（AC-CON-01-6）。
  3. 空间绑定：每个监听地址绑定一个 `Space`；会话查找带 `space` 条件，Cookie 落到另一空间的监听地址按无会话处理 401 / 4014（AC-CON-01-3）。
  4. 来源 IP：TCP 对端属于 `CON_TRUSTED_PROXIES` 时才从 `X-Forwarded-For` 自右向左取第一个不可信地址，否则用对端地址；CIDR 外（含登录页、静态资源）403 / 4030；由可信代理终止 TLS 时必须带 `X-Forwarded-Proto: https`，否则 403 / 4030。未认证的拒绝只写日志和 `console_rejected_total{reason}`（`cidr`），不进审计。
  5. 响应头中间件：CSP（PRD F-CON-10 规则 4 全文，`style-src` 每次响应随机 nonce，`script-src 'self'` 不含 `unsafe-inline`，`frame-ancestors 'none'`）、`X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`、生产 HSTS `max-age=31536000`；所有 `/console/api/*` 带 `Cache-Control: no-store`。空间可收紧不可放宽（单测断言 ops 覆盖只能更严）。
  6. 静态资源：托管 tianting-admin 构建产物（`CON_ADMIN_WEB_DIR`），SPA 回退到 `index.html` 时注入 CSP nonce。
  7. 错误响应格式 `{code, msg, reason?, details?}`，HTTP 状态取 errors.json 的 `http`；console 码一致性测试（只核对 `scope` 含 `console` 的码，errors.json 从 tianting-server 的 `protocol/` 同步副本，`scripts/sync-protocol.sh`）。
  8. 测试模式：`CON_TEST_MODE=1` 注册 `POST /console/test/clock {nowMs}`（[C 钟]，冻结语义同 im-server IMP-7）、`POST /console/test/faults {auditWrite, imLatencyMs}`；`tools/set-clocks.sh <nowMs>`（先设 [IM 钟] 再设 [C 钟]）放在本仓库并在 tianting-server 的测试文档中引用。
  9. 测试：`httptest` 注入对端地址覆盖 01-1、01-2；两空间监听覆盖 01-3、01-6；响应头快照测试覆盖 10-3、05-14；10-4 用 Playwright 在其他源的页面 iframe 嵌入（与 FE-400 共用）。
  10. 运维侧扩展点：`Space` 注册表、CSP 覆盖、静态目录（见上表）。

### CS-002 Postgres 迁移、数据库角色、生产启动保护、部署与多实例（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-001；**本地开发环境需要 Postgres（P-5 授权后安装 `postgresql@17`；本任务不自行安装）**
- AC：AC-CON-01-4、AC-CON-01-5
- 内容：
  1. `console-server migrate up|status`（embed + checksum，与 im-server BE-060 同一模式），只用 `console_owner` 连接；schema `auth`、`audit`、`biz`；角色 `console_owner`、`console_app`（运行时：`auth` / `biz` 读写；`audit.records` 只有 INSERT + SELECT；`audit.chain_head` 只有 SELECT + UPDATE）、`console_archiver`（`audit` 的 SELECT + 对已归档分区的 DROP）。迁移内 `REVOKE` / `GRANT` 显式写出，并有审计探针表 `audit.probe`。
  2. 生产启动保护（`CON_ENV=production`），全部检查跑完后**一次列出全部问题**再非 0 退出：`CON_TEST_MODE` 未开、已开空间的 `*_ALLOWED_CIDRS` 已配置且不含 `0.0.0.0/0` / `::/0`、`*_ORIGIN` 为 `https://`、`CON_DATA_KEYS` 合格（CS-003）、`CON_IM_SIGNING_KEY` 已配置、`CON_AUDIT_ANCHOR` 已配置、数据库用户为 `console_app` 且对 `audit.probe` 执行 `UPDATE` **必须失败**。
  3. 多实例：会话、失败计数、授权全部在库里，进程无状态；集成测试起两个实例连同一个库（AC-CON-01-5）。
  4. `deploy/`：systemd 单元（`console-server serve`）、定时器（每日 `audit verify`、每月归档）、示例环境文件；备份纳入 ARCH 3.4（每日全量 + WAL，加密），恢复演练季度一次写进运行手册。
  5. 可用性：Postgres 不可用时所有 `/console/api/*` 返回 503 / 5030（只读页面也不可用），`/readyz` 503。
  6. 测试：进程级测试五种生产配置错误各自非 0 退出且日志指出原因，另有一条"多个错误同时出现时一次全部列出"。
  7. 运维侧扩展点：`StartupCheck` 接口（B-OPS-15 追加节点凭证、Grafana 代理、运维网段检查）。

### CS-003 密钥材料与密码哈希（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-002
- AC：AC-CON-11-1、AC-CON-11-2、AC-CON-11-3、AC-CON-11-4、AC-CON-04-2
- 内容：
  1. `CON_DATA_KEYS`（kid → ≥ 32 字节随机）、`CON_DATA_ACTIVE_KID`：TOTP 种子 AES-256-GCM 加密存 `auth.totp(account_id, kid, nonce, ciphertext)`；`console-server keys rotate` 用当前 kid 重新加密全部旧记录；启动时若配置中缺少某条记录使用的 kid → 拒绝启动并指出（AC-CON-11-2）。
  2. 各类密钥互不相同（数据密钥、`CON_IM_SIGNING_KEY`、`CON_IM_MSGREAD_KEY`、数据库密码）；私钥类配置不进日志、`/readyz`、任何响应（日志脱敏测试 + 配置结构体 `String()` 遮蔽）。归档私钥不在服务器上。
  3. 哈希：恢复码、激活码用带服务端 pepper（`CON_HASH_PEPPER`，纳入密钥互斥检查）的 HMAC-SHA256；会话 ID 用 SHA-256；密码 Argon2id（默认 `m=64MiB, t=3, p=1`，可配置），全局信号量 4、排队超过 2 秒 503 / 5030。
  4. 测试：全库搜索 TOTP Base32 种子为 0；轮换后删除旧 kid 重启并登录；生产 16 字节 key / 与签名私钥相同时非 0 退出；50 并发登录时同时进行的 Argon2id ≤ 4、RSS 增量 ≤ 300MB、超时请求 503（进程级测试读 `/proc` 或 `ps`，macOS 用 `ps -o rss`）。

### CS-004 im-server 签名客户端（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-001；tianting-server BE-202（签名规则与 `protocol/vectors/admin-sign.json`）
- AC：AC-CON-10-1
- 内容：
  1. `imclient`：Ed25519 私钥 `CON_IM_SIGNING_KEY`（主调用方 `console`），第二批 `CON_IM_MSGREAD_KEY`（`console-msgread`，只给 CS-041、CS-042 注入，类型上与主客户端分开，主客户端没有调用正文接口的方法）；8 行待签名串；每个请求带 `X-IM-Operator: admin:{accountId}`、`X-Request-Id`（= console 审计 `requestId`，拆批时为 `{requestId}.{批号}`）、随机 nonce。
  2. 超时默认 10 秒（`CON_IM_TIMEOUT`）；**写操作不重试**；超时返回内部错误类型 `ErrResultUnknown`（由 CS-012 转成 504 / 5040）。im-server 返回的错误码原样透传（4042、4043、4090、4046、4132 等）。
  3. 测试模式 `imLatencyMs` 故障注入在客户端层生效。
  4. 测试：用 `admin-sign.json` 向量校验签名；对真实 im-server（测试模式）跑一次签名调用；断言私钥不出现在任何响应、日志（AC-CON-10-1 的服务端部分，构建产物与 HAR 检查见 FE-400）。
  5. 运维侧扩展点：节点运维接口复用本客户端（`console-ops`、每节点私钥、`X-IM-Target-Node` 必填）。

### CS-005 账号空间、引导与激活（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-003
- AC：AC-CON-02-1、AC-CON-02-2、AC-CON-02-3、AC-CON-02-4、AC-CON-02-10、AC-CON-02-11
- 内容：
  1. 表 `auth.accounts(id UUIDv7, space, username, display_name, status: pending|active|disabled, password_hash, password_history, idp_subject, idp_issuer, created_at, …)`，唯一键 `(space, username)`；账号名 `[a-z0-9._-]{3,32}`，重复 409 / 4092（空间内唯一）。
  2. `console-server account bootstrap --space admin|ops --username <u>`：该空间已有启用的空间超管 → 非 0 退出；成功输出一次性激活码；写审计 `account.bootstrap`（`actor=cli:{OS 用户}@{主机名}`）。
  3. 激活码：20 位 base32、24 小时（[C 钟]）、只存 pepper HMAC；`POST /console/api/auth/activate {username, code, newPassword}`：错误或过期 401 / 4015，计入登录失败计数（CS-008）；成功得到 `setup` 会话。
  4. TOTP 绑定：`POST /console/api/auth/totp/setup`（返回 otpauth URI）、`POST /console/api/auth/totp/confirm {otp}` → 发放 10 个恢复码（只显示一次）、会话升为 `full`；激活与绑定的 IP / UA 写审计；**通知本空间全部空间超管和审计员**（CS-016）。
  5. `setup` 会话调用设置接口以外的任何接口 401 / 4016。
  6. 账号接口不注册 `DELETE`（405，AC-CON-02-11）。
  7. 测试：集成测试覆盖 bootstrap 二次执行、激活过期（[C 钟] +24h1m）、setup 会话 4016、同名 4092 与跨空间同名成功。

### CS-006 账号管理：创建、停用、启用、重置、跨空间隔离、自我保护（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-005、CS-009（会话失效）、CS-010（权限点 `admin.manage`）、CS-011（step-up）——接口骨架先做，权限与 step-up 校验在 CS-010、CS-011 完成后接入
- AC：AC-CON-02-6、AC-CON-02-7、AC-CON-02-8、AC-CON-02-9
- 内容：
  1. 接口（需要 `admin.manage` + step-up，写审计）：`GET /console/api/accounts`、`POST /console/api/accounts`（生成激活码）、`POST /console/api/accounts/{id}/roles`、`/disable`、`/enable`、`/reset`、`/unlock`（CS-008）。
  2. 跨空间：所有账号查询带 `space = 当前会话空间`；另一空间的账号 ID 返回 404 / 4040，并写 `result=denied`、`code=4040` 审计（AC-CON-02-6）。
  3. 停用：全部会话立即失效，登录按 4015；重置：账号回到 `pending`，旧密码 / TOTP / 恢复码 / 全部会话作废，生成新激活码；通知被重置者以外的空间超管和审计员。
  4. 自我保护：不能改自己的角色、不能停用 / 重置自己；不能让空间内启用的空间超管数降到 0——检查与修改在同一事务内，对 `(space, 超管集合)` 加 `SELECT … FOR UPDATE`（或空间行级锁）；违反 409 / 4093。
  5. 测试：并发互相停用 20 轮（每轮恢复初始状态），每轮恰好一个成功、启用超管数 ≥ 1（AC-CON-02-8）；重置后原 Cookie / 原密码 + TOTP / 原恢复码都失败（02-9）。
  6. 运维侧扩展点：空间超管角色名由空间注册（`super_admin` / `ops_superadmin`），本任务只按"该空间的超管角色"判断。

### CS-007 登录、TOTP 防重放与恢复码（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-005
- AC：AC-CON-03-1、AC-CON-03-2、AC-CON-03-3、AC-CON-03-4、AC-CON-03-5、AC-CON-03-6、AC-CON-03-7、AC-CON-03-8、AC-CON-03-9
- 内容：
  1. `POST /console/api/auth/login {username, password, otp | recoveryCode}` 一步提交；6 种失败（不存在、密码错、动态码错、恢复码错、已停用、待激活）统一 401 / 4015、相同 `msg` 与响应头集合；不存在的账号执行一次假 Argon2id（单测断言调用次数）。成功：新会话 ID（防会话固定，预置的 Cookie 值作废）、返回 `{account, roles, perms, csrfToken}`、Cookie 见 CS-009。
  2. TOTP：RFC 6238 / SHA-1 / 6 位 / 30 秒 / ±1 步长；`auth.totp.last_step` 记录最近成功步长，`step ≤ last_step` 拒绝（覆盖登录和 step-up），被拒的重放计入失败次数（CS-008）；时钟取 [C 钟]。
  3. 恢复码：10 个、单次使用、pepper HMAC；使用写审计 `auth.recovery_code_used` 并通知本人；剩余 ≤ 2 时登录响应带提示；`POST /console/api/me/recovery-codes` 需要 step-up（未验证 403 / 4038），返回 10 个新码、旧码全部作废，审计 `auth.recovery_codes_regenerated`。
  4. 审计（认证事件，`phase=event`）：登录成功 / 失败（带失败类别，仅审计内部可见）、退出、恢复码使用；**不记录密码、动态码、恢复码**。HTTP 访问日志不记录请求体。
  5. 测试模式开关 `CON_TEST_DISABLE_LOCKOUT=1`（只在 `CON_TEST_MODE=1` 下有效，关闭账号锁定与 IP 限流）供 AC-CON-03-3 的时序测试。
  6. 测试：AC-CON-03-3 在隔离环境"不存在账号"与"存在账号密码错"各 50 次比较中位数（≤ 30ms）；03-4、03-5 用 [C 钟] 固定步长；03-8 扫描 console 日志、反向代理日志（测试用 nginx 容器）、`audit` 全表找不到 `Pw-CANARY-7f3a91`、没有动态码字段。

### CS-008 密码策略、锁定、IP 限流与解锁（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-007
- AC：AC-CON-04-1、AC-CON-04-3、AC-CON-04-4、AC-CON-04-5、AC-CON-04-6、AC-CON-04-7、AC-CON-04-8、AC-CON-04-9
- 内容：
  1. 密码策略：12～128 字符、不含账号名（不区分大小写）、不在 `assets/weak-passwords.txt`（SecLists `10-million-password-list-top-10000.txt`，固定版本，`assets/weak-passwords.sha256` 由测试校验）、不与最近 5 次相同 → 400 / 4000，`reason` 为 `tooShort` / `tooLong` / `containsUsername` / `weak` / `reused`；不强制字符种类。改密码 `POST /console/api/me/password {currentPassword, otp, newPassword}`，成功后删除该账号其他会话、保留当前会话。
  2. 锁定：表 `auth.login_failures`（键 = 规范化账号名 + 来源 IP；另有按账号名的全局计数），**不管账号是否存在**；15 分钟内同一键连续失败 5 次 → 该键锁 15 分钟；1 小时内账号名累计 50 次 → 该账号名对所有 IP 锁 1 小时；锁定期间不论凭证 429 / 4291，对不存在的账号名响应完全相同；成功登录清零该"账号名 + IP"计数；激活码错误、step-up 失败计入同一账号（[C 钟]）。
  3. IP 限流：`login` + `activate` 每 IP 每分钟 20 次，超出 429 / 4290（表 `auth.ip_rate`，多实例一致，与锁定区分）。
  4. 解锁：页面 `POST /console/api/accounts/{id}/unlock`（`admin.manage` + step-up，审计 `account.unlock`）；命令行 `console-server account unlock --space --username`（`actor=cli:{OS 用户}@{主机名}`）。被锁定（键对应已存在账号）时通知本人和本空间空间超管（CS-016）。
  5. 全部计数在 Postgres，两个实例共享（AC-CON-04-8）。
  6. 测试：[C 钟] 覆盖 04-3（IP-A 锁、IP-B 可登、15 分钟后恢复）、04-6（11 个 IP 合计 50 次）；04-4 与 04-3 第 6 次响应逐字节比较（除 `Date`）；04-7 第 21 次 4290；04-9 未 step-up 4038、step-up 后解锁成功且通知已发。

### CS-009 会话、Cookie、过期、并发上限与 CSRF（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-007
- AC：AC-CON-05-1、AC-CON-05-3、AC-CON-05-4、AC-CON-05-5、AC-CON-05-6、AC-CON-05-7、AC-CON-05-8、AC-CON-05-9、AC-CON-05-11、AC-CON-05-13、AC-CON-05-15
- 内容：
  1. 表 `auth.sessions(id_hash SHA-256, account_id, space, state: setup|full, csrf_token_hash, created_at, last_active_at, step_up_at, revoked_reason)`；会话 ID ≥ 128 位熵，库里只存 SHA-256。
  2. Cookie：`__Host-adm_sid`（admin）/ `__Host-ops_sid`（ops，空间注册）；`HttpOnly; Secure; SameSite=Strict; Path=/`，无 `Domain`。
  3. 过期（[C 钟]）：空闲 30 分钟、绝对 12 小时；**带 `X-Console-Background: 1` 的请求校验会话但不刷新 `last_active_at`**；4014 `reason`：`none` / `idle` / `absolute` / `loggedOut` / `revoked` / `replaced`。
  4. 失效：退出（`POST /console/api/auth/logout`）、改密码（保留当前）、停用 / 重置、角色变化（全部会话 `revoked`）、step-up 连续 5 次失败（CS-011）。每账号最多 3 个会话，第 4 个登录时最早的标记 `replaced`。
  5. `GET /console/api/session`：返回账号、角色、权限点、`csrfToken`（每次调用不轮换，页面刷新后由前端重新取）。
  6. CSRF：所有非 GET / HEAD 同时要求 `X-CSRF-Token` 等于会话令牌、`Origin` 等于该空间 Origin；否则 403 / 4037，业务不执行、不进审计（只写 `console_rejected_total{reason="csrf"}`）。路由注册时 GET 路由不得有写操作（代码评审 + 白名单单测：GET handler 只能注入只读仓储接口）。
  7. 测试：[C 钟] 覆盖 05-3、05-4、05-5；Cookie 属性、退出重放、CSRF 三种情形、第 4 个会话、改角色两个会话都 `revoked`；库中搜不到 Cookie 原值。
  8. 运维侧扩展点：`RenewPolicy`（Grafana 代理只有 `Sec-Fetch-Mode: navigate` 续期，代理 Cookie 绑定原会话；B-OPS-05）。

### CS-010 角色、权限点白名单、职责分离与授权拒绝审计（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-009
- AC：AC-CON-06-1、AC-CON-06-2、AC-CON-06-3、AC-CON-06-4、AC-CON-06-5、AC-CON-06-6、AC-CON-06-9、AC-CON-06-10
- 内容：
  1. 表 `auth.account_roles(account_id, role, granted_by, granted_at, expires_at NULL)`；权限 = 未过期角色的权限点并集（`expires_at` 用于应急授权，CS-011；判断按 [C 钟] 当前时间，无定时任务）。
  2. `PermissionRegistry`：每空间声明权限点、角色 → 权限点、互斥角色、敏感角色。管理空间：5 个角色、5.2 节第一批权限点（`user.read` … `admin.emergency`），互斥角色 `auditor`，敏感角色 `moderator`、`auditor`、`super_admin`。
  3. 路由注册 `route(method, path, perm | LoginOnly, opts)`；**未声明权限点的 `/console/api/*` 路由启动失败（"route without permission"）**，并有单测遍历路由表。
  4. 拒绝：无权限 403 / 4030；已认证的拒绝写审计 `result=denied`，同一账号 1 分钟内超过 20 次时，其余在该分钟结束时合并为 1 条（`params.count`、动作分布）——用表 `audit.denied_buffer`（`auth` schema 内，非审计表）在下一次请求或后台任务中冲刷；未认证的拒绝（无会话、CSRF、CIDR）只写日志和 `console_rejected_total{reason}`。
  5. 授予角色：互斥角色与任何其他角色叠加 → 409 / 4093；授予敏感角色时通知本空间全部审计员（CS-016）；角色变化使该账号全部会话失效（CS-009）。
  6. 测试：叠加角色并集、`support` 调导入 403 + denied 审计、30 次拒绝合并、无会话 100 次只进指标、`support`+`auditor` 409；AC-CON-06-10 在 ops 空间注册运维角色的测试桩下验证（CS-100 起的运维任务会复用同一条测试）。
  7. 运维侧扩展点：`PermissionRegistry` 注册 ops 空间角色与权限点（B-OPS-08）。

### CS-011 重新验证（step-up）与应急授权（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-010
- AC：AC-CON-07-1、AC-CON-07-2、AC-CON-07-4、AC-CON-07-5、AC-CON-06-7、AC-CON-06-8
- 内容：
  1. `POST /console/api/auth/step-up {otp}`：成功写 `sessions.step_up_at`（[C 钟]），5 分钟内同一会话的 `stepUp: window` 路由放行；输错 401 / 4015（前端弹框内提示），计入账号锁定计数；同一会话连续 5 次失败 → 会话 `revoked`（下一次请求 401 / 4014）+ 账号对所有 IP 锁定 15 分钟 + 通知本人和空间超管。TOTP 防重放与登录共用 `last_step`。
  2. 路由选项 `stepUp: window | everyTime`：未验证或超过 5 分钟 → **403 / 4038**（不是 401），业务不执行；`everyTime`（正文授权、隔离原文授权）要求请求体自带 `otp`，缺失 403 / 4038、错误 401 / 4015，不看 5 分钟窗口。基座要求 step-up 的路由：账号创建 / 改角色 / 停用 / 启用 / 重置 / 解锁、应急授权、重新生成恢复码、审计导出（P1）。
  3. 应急授权（**只在管理空间注册**，权限点 `admin.emergency`）：`POST /console/api/me/emergency-grants {role, minutes 15～240, reason}`，只能给自己、不能是 `auditor` 或空间超管角色，否则 400 / 4000；需要 step-up；写 `account_roles.expires_at`；审计 `account.emergency_grant`；通知本空间全部审计员和其他空间超管。到期判断见 CS-010。
  4. 测试：[C 钟] T+4 / T+6 分钟；正文授权不带 otp 403；连续 5 次失败后会话 revoked 且任意 IP 登录 4291；应急授权 31 分钟后 403；三种非法申请 400。
  5. 运维侧扩展点：ops 空间不注册 `admin.emergency`（任何运维角色调用 403 / 4030，PRD-ops AC-OPS-02-9）；运维 step-up 清单通过路由选项声明。

### CS-012 审计写入：两阶段、哈希链、脱敏、写失败与结果未知（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-002、CS-004、CS-010
- AC：AC-CON-08-1、AC-CON-08-2、AC-CON-08-3、AC-CON-08-4、AC-CON-08-8、AC-CON-08-9、AC-CON-08-10、AC-CON-08-11、AC-ADM-90-4
- 内容：
  1. 表 `audit.records`（按月分区，见 CS-015）字段：`seq`、`id`（UUIDv7）、`ts`、`space`、`phase`（`intent` / `result` / `event`）、`request_id`、`account_id`、`username`、`roles`、`actor`（命令行 `cli:…`）、`ip`、`ua`、`action`、`target`（类型 + ID 列表，> 50 只记数量和前 50）、`params`（脱敏摘要）、`reason`、`ticket`、`result`（`success` / `failure` / `denied` / `unknown`）、`code`、`prev_hash`、`hash`；`audit.chain_head` 单行。
  2. 写入：同一事务 `SELECT … FROM audit.chain_head FOR UPDATE` → 取 `seq`、`prev_hash` → 插入 → 更新链头 → 提交；`hash = SHA-256(prevHash ‖ canonical_json(除 hash 外全部字段))`（canonical JSON：键排序、无空白、UTF-8，规则写进 `docs/audit-hash.md` 并有固定向量测试）。
  3. 中间件 `audited(action, kind: write|sensitiveView|event)`：写操作与敏感查看先写 intent（含参数摘要与理由），**失败则不执行、不返回数据，503 / 5031**；执行后写 result；result 写失败时照常返回真实结果，打日志并 `console_audit_result_failures_total` +1，该 intent 显示"结果未知"（由 CS-014 对账补齐）。纯查看只写一条 `event`，写失败 5031。im-server 超时（CS-004 `ErrResultUnknown`）→ 504 / 5040、result 记 `unknown`。
  4. 脱敏：`params` 用每个 action 的白名单构造；**不记录**密码、动态码、恢复码、激活码、会话 ID、CSRF 令牌、密钥、消息正文、好友备注；推送 `data` 只记长度与 SHA-256。
  5. 数据库权限保证只追加（CS-002 的 GRANT）；集成测试用 `console_app` 执行 UPDATE / DELETE / TRUNCATE 均报权限错误。
  6. 参数化测试（AC-CON-08-11）：管理后台第一批全部写操作和敏感查看的 action 表（与 CS-020 的路由表同一份数据），每类执行一次后断言审计记录字段齐全。
  7. 性能：两实例并发写 60 秒，吞吐 ≥ 200 条 / 秒、P95 ≤ 20ms，之后 `audit verify` 通过（AC-ADM-90-4，Postgres 环境）。
  8. 运维侧扩展点：`action` 枚举按空间注册（运维 action 见 F-OPS-14）；同一条全局链。

### CS-013 外部锚定与 `audit verify` 校验命令（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-012；测试用 WORM 桶（MinIO 容器开启 Object Lock，CI 内运行）
- AC：AC-CON-08-5、AC-CON-08-6、AC-CON-08-7、AC-CON-08-12
- 内容：
  1. `CON_AUDIT_ANCHOR`（S3 兼容 Object Lock 桶或日志平台 HTTP 端点，抽象为 `AnchorSink`）：每小时整点（[C 钟]）和每新增 1000 条，把 `(seq, hash, ts)` 写入外部存储并写 `audit.anchors`；失败告警、10 分钟内重试；console 凭证对桶只有写入权限（集成测试断言删除返回权限错误）。
  2. `console-server audit verify [--from-seq]`：逐条重算哈希、检查 seq 连续、逐个外部锚定点比对、检查最大 seq ≥ 最近锚定 seq；输出第一个异常的 seq 和类型（`hashMismatch` / `gap` / `anchorMismatch` / `truncated`），非 0 退出；命令本身写 `event` 审计。归档后的校验从归档文件最后一条接续（CS-015）。
  3. 每日自动运行（`deploy/` 定时器）；校验失败、锚定失败通知空间超管和审计员（CS-016）。
  4. 测试：以超级用户删除第 500 条 → `gap 500`；锚定在 900 时删除 950～1000 通过、删除 850～1000 → `truncated`；改第 10 条并整链重算 → `anchorMismatch 500`；[C 钟] 推进 1 小时后桶中新增对象且 seq / hash 与库一致。

### CS-014 审计查询、详情、对账任务与只读接口（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-012；tianting-server BE-203（`GET /admin/audit`）
- AC：AC-CON-09-2、AC-CON-09-3、AC-CON-09-4、AC-CON-09-5、AC-CON-09-6、AC-CON-09-9、AC-ADM-90-3
- 内容：
  1. `GET /console/api/audit?from=&to=&actor=&action=&target=&result=&requestId=&space=&cursor=&limit=`（权限点 `audit.read`）：时间范围必填、≤ 31 天，否则 400 / 4000；按 `seq` 倒序游标分页；查询本身写 `audit.query` 事件（含筛选条件）。管理空间审计员只能看 `space=admin`，指定 `space=ops` → 403 / 4030；运维审计员可选"全部空间"（权限由 ops 空间注册）。
  2. 详情 `GET /console/api/audit/requests/{requestId}`：合并同一 `requestId` 的 intent / result；`GET /console/api/audit/im?requestId=`：经 `imclient` 调 im-server `GET /admin/audit`（写 `audit.query` 事件）。
  3. 只读：审计路由对 `PUT` / `PATCH` / `DELETE` 返回 405（显式注册 405 handler，不依赖默认 404）。
  4. 对账任务（每 10 分钟，[C 钟]，多实例下用 `pg_try_advisory_lock` 只跑一个）：扫描 intent 超过 5 分钟无 result 的记录 → 按 `requestId` 查 im-server 审计：有结论补写 `result`（`params.reconciled=true`）；im-server 也没有 → 补写 `result=failure`、`code=0`、`params.reconciled="notExecuted"`。
  5. 索引：`(space, ts)`、`(account_id, ts)`、`(request_id)`、`(action, ts)`。性能：1000 万条、31 天按操作人筛选 P95 ≤ 2 秒（AC-ADM-90-3）。
  6. 测试：32 天 400；查询后新增 `audit.query`；admin 审计员 `space=ops` 403；PATCH / DELETE 405；[C 钟] 推进 15 分钟对账 `notExecuted`；AC-CON-09-9 在 ops 空间审计员测试桩下验证（运维任务复用）。

### CS-015 审计分区、保留与加密归档（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-013、CS-014
- AC：AC-CON-09-7、AC-CON-09-8
- 内容：
  1. `audit.records` 按月分区（迁移预建 + 每月任务预建下月分区）。
  2. `console-server audit archive`（`console_archiver` 连接）：把满 1 年（[C 钟]）的分区导出 JSONL → age 加密（公钥 `CON_ARCHIVE_RECIPIENT`）→ 写归档桶（与锚定桶不同，开启 Object Lock，保存 `CON_AUDIT_ARCHIVE_YEARS=3` 年）→ 校验可解密（测试环境用测试私钥；生产只校验对象写入与密文哈希，私钥线下保存）、条数与哈希链和锚定一致 → DROP 分区；归档操作写审计 `audit.archive`；记录归档末条 `(seq, hash)` 供 `verify` 接续。
  3. `console_app` 尝试 DROP 分区报权限错误（AC-CON-09-8）。
  4. 测试：生成 13 个月测试数据，运行归档后最早分区被导出、解密核对、删除，`audit verify` 仍通过。

### CS-016 站内安全通知（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-010
- AC：AC-CON-12-1、AC-CON-12-2
- 内容：
  1. 表 `auth.notifications(id, account_id, space, kind, payload, created_at, read_at)`，保留 90 天（每日清理）；接口 `GET /console/api/notifications`、`GET /console/api/notifications/unread-count`（前端轮询带 `X-Console-Background: 1`）、`POST /console/api/notifications/read`。
  2. `notify.ToSpaceRole(space, role, kind, payload)`、`notify.ToAccount(...)`；只发给启用账号。触发（基座）：激活 / 重新绑定（空间超管、审计员）；被重置（被重置者以外的空间超管、审计员）；被锁定（本人、空间超管）；恢复码使用、剩余 ≤ 2（本人）；授予敏感角色（审计员）；应急授权（审计员、其他空间超管）；审计校验失败、锚定失败（空间超管、审计员）。
  3. 内容只含"谁、什么事、何时"；参数化测试断言每类触发的接收人集合，并扫描通知内容找不到激活码、动态码。
  4. 运维侧扩展点：运维追加触发条件按"空间 + 角色"注册（B-OPS-14）。

### CS-017 浏览器侧密钥隔离检查与依赖安全（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-001、CS-004；tianting-admin FE-400（构建产物）
- AC：AC-CON-10-1、AC-CON-10-2
- 内容：
  1. CI 任务"密钥隔离扫描"：对 tianting-admin `dist/` 与一次完整流程（登录、搜索用户、强制下线、解散群、查审计）的 HAR，搜索 IM 调用方 HMAC 密钥原文、`CON_IM_SIGNING_KEY` 私钥、`X-Admin-Key`、`X-IM-Signature`，结果必须为 0（测试环境使用固定的可识别测试密钥）。
  2. 接口层面：任何 `/console/api/*` 响应都不回显 im-server 请求头；`imclient` 的错误不把签名头带进 `msg`。
  3. XSS 端到端（AC-CON-10-2）：用 `<img src=x onerror=alert(1)>` 作为昵称、群名、发送者昵称、审计理由，Playwright 走一遍用户列表 / 详情 / 群详情 / 元数据页 / 审计详情，断言无对话框、无对 `x` 的请求（前端纯文本渲染见 FE-401）。

---

## 管理后台业务（第一批）

### CS-020 业务公共层与管理空间权限矩阵（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-010、CS-011、CS-012
- AC：AC-ADM-05-1、AC-ADM-05-2、AC-ADM-05-4
- 内容：
  1. 理由校验：去首尾空白后 4～200 个码点、不能由同一字符重复组成，否则 400 / 4000（`reason: "reasonInvalid"`）；工单号按 `CON_TICKET_PATTERN`。
  2. 每日上限计数器（表 `biz.daily_quota(account_id, kind, day Asia/Shanghai, key)`，按"不同对象"计数，已查过的对象不重复计数），超出 429 / 4292；分钟级限流 4290。
  3. 列表与搜索参数：`limit` ∈ {20, 50, 100}、关键字去空白后 1～64 字符；带理由的敏感查询一律 `POST` + JSON，路由注册时禁止敏感查询使用 GET（单测）。
  4. im-server 错误透传表与前端文案码一致（4042、4043、4090、4046、4130、4131、4132、4007、4292）。
  5. **5.2 节矩阵参数化测试**（AC-ADM-05-1）：第一批每个路由一个合法请求样本 × 5 个单角色账号，✅ 格返回该接口成功状态码，空格 403 / 4030；5.3 节"需要理由"的每类操作不带理由 / 3 字符 / `aaaa` 都 400 且未执行（AC-ADM-05-2）；`super_admin` 无应急授权时元数据、审计查询 403（AC-ADM-05-4）。
  6. 5.3 节 step-up 要求通过路由选项声明：好友导入 / 删除、社交配置、解散群、（二）封禁 / 解封、删除消息、批准全员通知走 `window`；正文与隔离原文授权走 `everyTime`。

### CS-021 用户搜索、详情、强制下线与在线设备查看（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-020；tianting-server BE-205、BE-206、BE-013
- AC：AC-ADM-10-1、AC-ADM-10-2、AC-ADM-10-3、AC-ADM-10-4、AC-ADM-10-6、AC-ADM-10-7、AC-ADM-12-1、AC-ADM-12-2、AC-ADM-12-3、AC-ADM-12-4、AC-ADM-14-1、AC-ADM-14-2、AC-ADM-14-3、AC-ADM-14-4、AC-ADM-14-5、AC-ADM-14-6
- 内容：
  1. `GET /console/api/users?q=&official=&cursor=&limit=`（`user.read`，不写审计）、`GET /console/api/users/{id}`（资料、官方标记、是否在线；第二批追加封禁 / 禁言状态，**状态与到期时间只用 im-server 返回值**）。
  2. 强制下线 `POST /console/api/users/{id}/kick {reason, ticket?}`（`user.kick` + 理由，审计 `user.kick`）；im-server 4042 透传，审计 result `failure`、`code=4042`。
  3. 在线设备 `POST /console/api/users/sessions/query {userId, reason, ticket?}`（`user.sessions` + 理由，审计 `user.sessions.query`，`params.count`）：每账号每分钟 30 次（4290）、每天 50 个不同 userId（4292）；userId、理由不出现在 URL 和访问日志（访问日志只记路径模板）；B6 之前透传 `partial` / `scope: "localNode"`，B6 之后透传未响应节点（AC-ADM-14-6 依赖 B6）。
  4. 测试：与 im-server 的 e2e 覆盖搜索规则（转义、大小写、官方筛选、翻页 1000 无重复）、在线状态、踢线两连接、重新签发后可连、`nobody` 4042、无理由 400、`operator` 403、第 51 个 userId 429、访问日志无 userId。

### CS-022 用户导入（CSV 校验、拆批、汇总审计）（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-020；tianting-server BE-010（`POST /admin/users/import`）
- AC：AC-ADM-11-1、AC-ADM-11-2、AC-ADM-11-3、AC-ADM-11-4、AC-ADM-11-5
- 内容：
  1. `POST /console/api/users/import/validate`（multipart CSV 或单个表单）：UTF-8、表头 `userId,nickname,avatar`、≤ 5000 行（超出 400 / 4131）；整体校验（userId 格式 4004、昵称 ≤ 32 / 头像 ≤ 512 → 4000，文件内重复以最后一条为准），最多返回前 100 条错误（行号 + 码 + 原因）；通过则返回 `importToken`（存 `biz.import_jobs`，含文件 SHA-256、预览前 20 行、总行数）。
  2. `POST /console/api/users/import {importToken}`（`user.import`）：intent（总行数、文件 SHA-256）→ 按每批 500 调 im-server（`X-Request-Id={requestId}.{批号}`）→ 某批失败即停止 → result（成功批数 / 总批数、失败码）；响应 `{succeededBatches, totalBatches, failedCode?}`。
  3. 测试：1200 行 3 次调用；第 7 / 30 行错误不调用 im-server；5001 行 4131；第 2 批注入 5000 停止；`support` 403。

### CS-023 社交管理：好友列表、导入 / 删除、社交配置、官方账号（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-020；**tianting-server BE-114（社交配置、好友导入 / 删除 / 查询的管理接口与审计）、BE-115（官方账号）——见 [backend-social.md](backend-social.md)**；tianting-server BE-201（scope `social.settings` / `social.friends`）
- AC：AC-ADM-15-1、AC-ADM-15-2、AC-ADM-15-3、AC-ADM-15-4、AC-ADM-15-5、AC-ADM-15-6、AC-ADM-15-7、AC-ADM-16-1、AC-ADM-16-3、AC-ADM-16-4、AC-ADM-16-5、AC-ADM-17-1、AC-ADM-17-2、AC-ADM-17-3、AC-ADM-17-4
- 内容：
  1. 好友列表 `GET /console/api/users/{id}/friends?cursor=`（`social.read`，分页 100，**响应不含备注字段**，不写审计）。
  2. 好友导入 / 删除：`validate` + 提交两步（同 CS-022）；CSV `a,b,remarkA,remarkB` / `a,b`，≤ 5000 行，`a ≠ b`、userId 格式、文件内重复取最后一条；每批 500 对；im-server 每批原子、批之间不原子；`social.write` + 理由 + step-up；审计 intent（对数、文件 SHA-256）+ result；**备注只在内存中传给 im-server，不进日志、审计和 `biz.import_jobs`**：导入任务表只存文件 SHA-256、对数和预览（预览不显示备注列），确认提交时前端重新上传同一文件，console 校验哈希一致后再拆批调用。im-server 未注册该接口（F-KEY 未上线）返回 404 时，响应 `code=4040`、`reason: "notAvailable"`，前端提示"该功能尚未在服务端开放"（AC-ADM-15-7）。
  3. 社交配置：`GET /console/api/settings/social`（`social.read`）、`PATCH /console/api/settings/social`（`social.settings` + 理由 + step-up；**只把修改了的字段**传给 im-server；切换 `mode` 时请求必须带 `confirmMode` 等于新模式名，否则 400 / 4000）；审计 `params` 为"旧值 → 新值"（先读当前值）。
  4. 官方账号：`PUT /console/api/users/{id}/official {official, reason}`（`user.official` + 理由，审计 `user.official.set` / `user.official.unset`），4042 透传。
  5. 测试：与 im-server（PRD-social 接口就绪后）e2e 覆盖 15-1～15-7、16-1、16-3～16-5、17-1～17-4；特征串 `ZXQ-REMARK-789` 扫描 console 日志与审计。

### CS-024 群查询与建群（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-020；tianting-server BE-207、BE-030
- AC：AC-ADM-20-1、AC-ADM-20-2、AC-ADM-20-3、AC-ADM-20-4、AC-ADM-21-1、AC-ADM-21-2、AC-ADM-21-3、AC-ADM-21-5
- 内容：
  1. `GET /console/api/groups?q=&includeDissolved=&cursor=&limit=`、`GET /console/api/groups/{id}`（`group.read`，不写审计）。
  2. 建群 `POST /console/api/groups {groupId, name, members[], historyVisible, owner?}`（`group.write`，审计 `group.create`）：`ug-` 前缀 400 / 4004（console 自校验，与 im-server 一致）；成员 > 2000 400 / 4130；4090、4042 透传；M4 起可选 `owner`（调用 F-SOC-30 接口）。
  3. 测试：e2e 覆盖前缀搜索、成员数、包含已解散、（M4）类型与群主、建群成功 / 4090 / 4042 / 4004。

### CS-025 群成员、群设置、群主交接与解散（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-024；tianting-server BE-208、BE-209、BE-031；群主交接依赖 BE-136（backend-social.md，M4）
- AC：AC-ADM-22-1、AC-ADM-22-2、AC-ADM-22-3、AC-ADM-22-4、AC-ADM-22-5、AC-ADM-22-6、AC-ADM-23-1、AC-ADM-23-2、AC-ADM-24-1、AC-ADM-24-2、AC-ADM-24-7、AC-ADM-25-1、AC-ADM-25-2、AC-ADM-25-3
- 内容：
  1. 成员列表 `GET /console/api/groups/{id}/members?cursor=&prefix=`（每页 50）。
  2. 加成员 `POST /console/api/groups/{id}/members {userIds}`：先查当前成员数，`当前 + 新增（去重后） > 2000` → 400 / 4130、一个都不加、不调 im-server；否则每批 500 调用；一条 intent + 一条 result。移除 `DELETE /console/api/groups/{id}/members {userIds, reason}`（理由必填）。
  3. 群设置 `PATCH /console/api/groups/{id} {name?, historyVisible?, notice?(M4)}`（`group.write`，审计旧值 → 新值）。
  4. 群主 `PUT /console/api/groups/{id}/owner {userId|null, reason}`（`group.owner` + 理由，调用 `PATCH /admin/groups/{id} {owner}`，scope `social.settings`）；4046 透传；审计 `group.owner.set` / `group.owner.unset`。
  5. 解散 `POST /console/api/groups/{id}/dissolve {reason, confirmGroupId}`（`group.dissolve` + 理由 + step-up；`confirmGroupId` 必须等于目标，否则 400 / 4000）→ `DELETE /admin/groups/{id}`；审计 `group.dissolve`。（用 POST 而不是 DELETE：请求体带理由。）
  6. 测试：1500 人 30 页；加 3 人 maxSeq 不变；移除后 4033；1200 人分 3 批一条 intent；1999 + 2 → 4130 且 im-server 无调用；筛选 `u1`；改名与 historyVisible；解散事件、4043、无理由 400 / 未 step-up 4038；（M4）群主设置 / 收回 / 非成员 4046。

### CS-026 会话消息元数据查询（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server（待建）
- 依赖：CS-020；tianting-server BE-210
- AC：AC-ADM-30-1、AC-ADM-30-2、AC-ADM-30-3、AC-ADM-30-4、AC-ADM-30-5、AC-ADM-30-6、AC-ADM-30-7、AC-ADM-30-8、AC-ADM-24-4
- 内容：
  1. `POST /console/api/messages/meta/query {convId, beforeSeq?, from?, sinceTs?, untilTs?, reason?, ticket?}`（`msg.meta`，每页 50）：会话 ID 格式 console 先校验 → 400 / 4007；单聊 `c2c:` 必须带理由，否则 400 / 4000；每次"进入"一个单聊会话写 `msg.meta.query`（含理由），同一会话 15 分钟内翻页不再要求理由（表 `biz.meta_sessions(session_id, conv_id, until)`，[C 钟]）；单聊每账号每天 50 个不同会话（4292）；群会话不要求理由、每次进入写一条审计（不含理由）。
  2. 响应模型只有元数据字段（独立类型，单测断言序列化结果没有 `body` 键）；im-server 响应在 console 内反序列化到同一类型，未知字段丢弃。
  3. 测试：120 条 3 页 + 1 条审计；按发送者筛选；4007；未聊过空列表；无理由 400；第 51 个单聊 429；两层响应找不到 `ZXQ-BODY-123`；`operator` 403；已解散群 20 条元数据。

---

## 管理后台业务（第二批）

### CS-040 封禁 / 解封（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：CS-021；tianting-server BE-222
- AC：AC-ADM-13-1、AC-ADM-13-5、AC-ADM-13-10
- 内容：`POST /console/api/users/{id}/ban {duration: 1h|1d|7d|30d|permanent, reason}`、`POST /console/api/users/{id}/unban {reason}`（`user.ban` + 理由 + step-up，审计 `user.ban` / `user.unban`，每次都记）；用户详情与列表显示 im-server 返回的 `ban`（不自己计算到期）；未 step-up 403 / 4038 且 im-server 无调用。e2e 覆盖封禁踢线、解封可签发、未 step-up。

### CS-041 群消息正文查看授权与读取（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：CS-011（`everyTime`）、CS-004（`console-msgread` 客户端）；tianting-server BE-225；PRD-admin Q-1 确认
- AC：AC-ADM-08-1、AC-ADM-08-2、AC-ADM-08-3、AC-ADM-08-4、AC-ADM-08-6、AC-ADM-08-7、AC-ADM-08-8、AC-ADM-08-9、AC-CON-07-4
- 内容：
  1. `POST /console/api/messages/grants {convId, ticket, reason, otp}`（`msg.read.group` / `msg.read.c2c`，按会话类型）：顺序 = 权限（4030）→ 工单号 / 理由（4000）→ 动态码缺失（4038）/ 错误（4015）→ 写 intent → 用 `console-msgread` 凭证调 `POST /admin/msg-grants`（4292、4030 `c2cBodyDisabled` 透传）→ 表 `biz.msg_grants(grant_id, session_id_hash, conv_id, expires_at 来自 im-server)` 绑定**当前会话 + convId**。
  2. `POST /console/api/messages/body {convId, beforeSeq, limit≤50}`：无该会话有效授权（未申请、过期（按 im-server 返回的到期时间与 [IM 钟] 结果）、换会话、换浏览器会话）→ 403 / 4039；每页写 `msg.read.page`（seq 范围、条数，不含正文）；响应 `Cache-Control: no-store`。
  3. 单聊：im-server 开关关闭时页面显示"单聊正文查看未开启"（透传 4030 `reason=c2cBodyDisabled`）。
  4. 主凭证调用 `msg-grants` 403 的验证放在 tianting-server BE-225；本任务的测试断言主 `imclient` 类型没有正文方法（编译期保证）。
  5. 测试：e2e 覆盖 08-1～08-4、08-6～08-8；正文特征串不出现在 console 审计和日志。

### CS-042 删除消息与隔离原文查看（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：CS-041；tianting-server BE-223、BE-224、BE-225
- AC：AC-ADM-31-1、AC-ADM-31-7、AC-ADM-32-2、AC-ADM-32-5
- 内容：`POST /console/api/messages/delete {convId, msgIds≤50, reason}`（`msg.delete` + 理由 + step-up，审计 msgId 列表与理由，不含原文）；隔离原文授权 `POST /console/api/messages/grants {…, quarantine: true}`（`msg.quarantine.read`，流程同 CS-041，审计 `msg.quarantine.grant`）与读取（4044 透传为"消息不存在或已过期"）；元数据页显示删除标记、删除人、删除时间（im-server 返回值）。e2e 覆盖。

### CS-043 用户级与群成员禁言（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：CS-021、CS-025；tianting-server BE-226、BE-227
- AC：AC-ADM-40-4、AC-ADM-40-7、AC-ADM-41-4、AC-ADM-41-5
- 内容：`POST /console/api/mutes {scope: user|group, userId, groupId?, duration: 10m|1h|1d|7d|30d|permanent|customSeconds(60～2592000), reason}`、`POST /console/api/mutes/release {scope, userId, groupId?, reason}`（`mute.write` + 理由，不需要 step-up，审计 `mute.set` / `mute.release`）；4042、4046 透传；用户详情、群成员列表显示 im-server 返回的 `mutedUntil`。e2e 覆盖解除后可发送、成员列表禁言标记。

### CS-044 系统通知：模板与按用户推送（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：二期 push 协议定稿；tianting-server BE-228、BE-229
- AC：AC-ADM-50-1、AC-ADM-50-2、AC-ADM-50-4、AC-ADM-50-6
- 内容：
  1. 模板配置 `CON_PUSH_TEMPLATES`（文件：`type` → `data` 的 JSON Schema），`GET /console/api/push/templates`。
  2. `POST /console/api/push/preview`（目标人数、type、格式化 data）→ `POST /console/api/push/user {userIds≤1000 | importToken, type, data, reason, ticket?}`（`push.user` + 理由）：`data` 按模板 Schema 校验、≤ 4KB，否则 400 / 4000；> 1000 → 400 / 4131；4042 + `details.userIds` 透传、整批不发送。
  3. 表 `biz.push_records(id, kind, type, data 全文, data_sha256, targets, created_by, approved_by, sent_at, im_push_id)`，保留期与审计一致；审计只记 `data` 长度与 SHA-256。
  4. 测试：e2e 两人 3 秒内收到；[IM 钟] 第 6 / 8 天补收；Schema 不符 / 超长 400；ghost 与 1001。

### CS-045 全员通知：草稿、双人批准、间隔与发送记录（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-console-server（待建）
- 依赖：CS-044
- AC：AC-ADM-50-3、AC-ADM-50-5、AC-ADM-50-7、AC-ADM-50-8
- 内容：
  1. 草稿 `POST /console/api/push/all/drafts {type, data, reason}`（`push.all.request`），24 小时未批准自动作废（读取时按 [C 钟] 判断）。
  2. 批准 `POST /console/api/push/all/drafts/{id}/approve {confirmText: "全员发送"}`（`push.all.approve` + step-up）：**发起人不能批准自己的草稿**（即使同时有超管角色）→ 403 / 4030；全局两次全员推送间隔 ≥ 5 分钟（表 `biz.push_records` 最近一次全员发送时间，事务内加锁）→ 429 / 4290。
  3. 发送记录 `GET /console/api/push/records`（审计员与超管可看全文；权限点在 5.2 节之外新增 `push.records.read` 给 `auditor`、`super_admin`——见文末 PRD 问题 4）。
  4. 测试：5 人在线收到、离线上线收不到；T+4 分钟 429、T+5 分钟成功；自批 403；记录全文与审计 SHA-256 一致。

---

## P1

### CS-060 数据概览（1 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-console-server（待建）
- 依赖：tianting-server BE-241
- AC：AC-ADM-70-1、AC-ADM-70-2
- 内容：`GET /console/api/stats`（`stats.read`，代理 `GET /admin/stats`）；前端每 60 秒刷新带 `X-Console-Background: 1`，[C 钟] 推进 31 分钟后刷新请求 401 / 4014（复用 CS-009 续期规则）。

### CS-061 审计导出 CSV（1 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-console-server（待建）
- 依赖：CS-014
- AC：（无对应 AC：F-CON-09 规则 7 为 P1）
- 内容：单次 ≤ 1 万条、step-up、写审计；导出文件不落盘（流式）。

### CS-062 SSO（OIDC）登录与双人复核（3 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-console-server（待建）
- 依赖：CS-007
- AC：（无对应 AC：F-CON-03 规则 7、F-CON-02 规则 9 为 P1）
- 内容：账号有 `idp_subject` 时走 OIDC；每空间保留至少 1 个本地空间超管；超管重置另一超管需第二人确认。

---

## 发现的 PRD 问题（不改 PRD，供变更流程参考）

| # | 位置 | 问题 | 本单的处理 |
|---|---|---|---|
| 1 | 第 14 节 console-server 第一批 | 基座分项相加为 20.5（2.5 + 2 + 2.5 + 1.5 + 1.5 + 1.5 + 0.5 + 3.5 + 1.5 + 0.5 + 1 + 1 + 1），表中写 21；第一批合计 28.5 也因此多 0.5（20.5 + 7.5 = 28） | 按本单自己的估算（基座 22）列出，差异见总览 |
| 2 | 第 7 节末段 vs PLAN 第 4 节 | PRD 写"前端是 console-server 仓库里的独立入口 `web/admin`（运维后台为 `web/ops`）"，PLAN 第 4 节已建独立仓库 tianting-admin / tianting-ops | 按分发要求：前端在 tianting-admin 仓库，console-server 只托管构建产物（CS-001 第 6 条） |
| 3 | F-ADM-24 / PRD-social F-SOC-26 | `DissolveGroup` 与 `DELETE /admin/groups/{id}` 在 PRD-admin（G-ADM-11，"解散 2 人日"）和 PRD-social（M3）都出现，容易重复实现 | PRD-social 任务单 BE-122 实现并注明计入 PRD-admin；本单 BE-209 只做 scope / 审计接入与验收 |
| 4 | F-ADM-50 规则 7 vs 5.2 节 | "审计员和超管可以查看发送记录全文"，但 5.2 节没有对应权限点（`audit.read` 只有审计员，超管没有） | CS-045 新增 `push.records.read`（审计员、超管），需 PRD 补一行 |
| 5 | 5.2 节 `user.official` vs F-ADM-25 / F-KEY-02 | console 侧群主交接用权限点 `group.owner`，im-server 侧 scope 是 `social.settings`；不是矛盾，但 ADMIN.md 需写明 `PATCH /admin/groups/{id}` 的 `owner` 字段走 `social.settings`、其他字段走 `group.write`（同一路由两个 scope） | BE-201 的 scope 声明支持"按字段追加 scope" |
