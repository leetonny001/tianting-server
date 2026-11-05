# 后端任务单：管理接口收紧与后台所需的 im-server 能力（PRD-admin）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-admin v1.0](../prd/PRD-admin-v1.0.md)（冻结）第 3 节 F-KEY-01～05、第 6 节、第 8.1 节 G-ADM-xx、第 10、11 节；[PRD-core v1.3](../prd/PRD-core-v1.3.md)；[PRD-social v1.0](../prd/PRD-social-v1.0.md)（社交管理接口、`DissolveGroup`）；[ARCH-v1](../arch/ARCH-v1.md)；[ADMIN.md](../../protocol/ADMIN.md)、[ERRORS.md](../../protocol/ERRORS.md)、[errors.json](../../protocol/errors.json) |
| 负责 | 后端（AI） |
| 仓库 | tianting-server（本仓库 `server/`、`protocol/`） |
| 起点 | 一期代码：`server/internal/admin`（静态密钥 `X-Admin-Key`，与 `/ws` 共用一个 mux，见 `server/internal/app/app.go`）、`store` 迁移 0001～0003、`hub`（单节点在线表）、BE-003 测试模式与 `POST /admin/test/clock`（[IM 钟]） |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（后端 e2e、单测或进程级测试）；`go test -race ./...` 全过；`protocol/`（ADMIN.md、ERRORS.md、errors.json、Schema、vectors）同步更新；存储新方法进入 `store/storetest` 契约测试；对应 TC-KEY / TC-ADM 用例通过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。console-server 侧任务见 [console.md](console.md)，管理后台前端见 [frontend-admin.md](frontend-admin.md)。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-200 管理接口独立监听 | ✅ 完成 | `IM_ADMIN_ADDR`（默认 127.0.0.1:8081）+ TLS + 来源网段（只看对端地址）；地址冲突与生产 TLS 启动保护（单测 + 进程级）；对外端口上 `/admin/*` 404；e2e / 压测工具 / SDK 集成测试改走管理端口 |
| BE-201 调用方、scope、旧密钥过渡 | ✅ 完成 | `IM_ADMIN_CALLERS`（0600、id 格式、密钥两两不同且不等于 JWT / 旧密钥、正文类 scope 只能在唯一的 bodyAccess 调用方上）；路由注册必须声明 scope（否则 panic）；参数化测试遍历全部路由；生产环境旧密钥需 `IM_ADMIN_LEGACY=1`，`/readyz` 告警。"旧密钥调用新接口 403"待 BE-203 有了第一个新接口后补测试 |
| BE-202 请求签名与防重放 | ✅ 完成 | `internal/adminauth`：8 行待签名串、HMAC / Ed25519、±300 秒（[IM 钟]）、nonce 签名通过后登记（内存 100 万，满时 503）；4017 + `im_admin_auth_failures_total{reason}`；向量 `protocol/vectors/admin-sign.json` 由 Go / Node.js / Java 三种实现在 CI 校验；7 个变异全部被测试发现 |
| BE-203 im-server 侧审计 `admin_audit` | ✅ 完成 | 迁移 0004（PRD-social 的迁移顺延到 0005 起）；intent 写失败 503 不执行；summary 只记字段名；`GET /admin/audit`；Store 只有追加和查询（契约测试反射断言）；`/admin/test/faults`；6 个变异全部被测试发现。AC-KEY-04-3 的好友备注 / 推送 data 部分随 PRD-social 好友导入和 BE-229 补测；AC-KEY-05-2 其余接口随 BE-205、BE-209、BE-210 和 PRD-social 补测 |
| BE-205 ～ BE-210 第一批查询接口 | ✅ 2026-11-04 | 用户搜索、在线设备（`internal/sessions.Query` 唯一入口，连接只存脱敏 IP）、群列表、成员分页、解散收口、消息元数据（响应类型没有正文字段）；迁移 0008（虚拟生成列 `nickname_fold` / `name_fold`，写路径不用改）；存储契约 5 组、e2e `TestTCADM*`；SQLite 规模压测见 [perf-admin-2026-11-04](../reports/perf-admin-2026-11-04.md)。**未验收**：AC-ADM-90-1、90-2 的 Postgres 规模（随 BE-070）、AC-ADM-14-6（依赖 B6） |
| BE-211 第一批协议 | ✅ 2026-11-04 | errors.json 新增 13 个码（4017 已在 X-5 加入）、10.2 节扩展、`closeCodes` 1011、`kickReasons` suspended；im-server 跳过只含 console 的码，SDK 生成器同样跳过；ERRORS.md、ADMIN.md、Schema `Error.details` 同步 |
| BE-220 进程内定时任务 | ✅ 完成 | `internal/scheduler`：注入时钟、失败不影响下次、`Locker` 接口给 B6 预留；社交清理也挂上来；`POST /admin/test/jobs/{name}/run`、`im_job_runs_total`、`im_job_duration_seconds` |
| BE-221 第二批协议 | ✅ 完成 | Schema、向量、ADMIN.md、ERRORS.md、开关 `IM_V13_ADMIN_FEATURES`。旧的二期 `msgDeleted`、`muted` 形状按 PRD 11.1 改掉。4039 的 scope 增加 `admin`：BE-225 要求 im-server 同码，PRD 10.4 第 5 条写的只含 console 与此冲突，以任务单为准。SDK 侧在 FE-431～436 |
| BE-222 封禁 / 解封 | ✅ 完成 | 13-1～13-8 用例通过。版本号和封禁一条查询读出（否则并发时能签发绕过封禁的 token）；握手区分 banned / suspended；进入在线表后再查一次版本号，封禁和强制下线都不留漏网连接（这一行没有自动化用例：时序窗口测不稳） |
| BE-223 删除消息 | ✅ 完成 | 31-1、31-2、31-5～31-9 用例通过 |
| BE-224 加密隔离 | ✅ 完成 | 32-1（文件原始字节，含 `secure_delete`，变异验证过）、32-3、32-4 通过。WAL 旧帧要等检查点才被覆盖，写入 ADMIN.md |
| BE-225 正文授权 | ✅ 完成 | 08-4、08-7、08-8、08-9、32-2、32-5、KEY-02-7 通过。im-server 这边 Q-1 两种方案都一样；Q-1 只决定 console-server 能否持有 `console-msgread` 凭证，影响 CS-041 |
| BE-226 用户级禁言 | ✅ 完成 | 40-1～40-4、40-6～40-9 通过（40-9 双节点） |
| BE-227 群成员禁言 | ✅ 完成 | 41-1～41-6 通过 |
| BE-228、BE-229 系统推送 | 🟡 2026-11-05 服务端完成 | 开关 `IM_PUSH_ENABLED` 默认关闭。AC-PUSH-04-5、04-2 开发机通过；AC-PUSH-04-4 按 v1.1 改为单聊 ack P99 ≤ 200ms（P-20 已定），上一轮实测 60.9ms，新判定待重跑（[压测报告](../reports/perf-push-2026-11-05.md)）。改坏验证 32 处全部发现。sdk-ts 和 console 的联调 AC（01-1、07-1、11-2、ADM-50-1、50-3）未验证 |
| BE-240 ～ BE-243 | ⬜ 未开始 | P1 / P2 |

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| **X-5 前置（第一批）** | BE-200 ～ BE-203 | 4.5 | 管理端口独立监听、调用方 + scope + 旧密钥过渡、请求签名防重放、`admin_audit`。**PRD-social 的社交管理接口（S-11）也以它为前置** |
| 第一批业务接口 | BE-205 ～ BE-211 | 9（另有 BE-122 1.5 在 backend-social.md） | 用户搜索、在线设备与连接元数据、群列表、成员分页、解散群（管理侧收口；`DissolveGroup` 本体是 BE-122）、元数据查询、第一批协议与错误码 |
| 第二批 | BE-220 ～ BE-229 | 19 | 定时任务、第二批 Schema、封禁、删除消息、隔离原文、正文授权、用户 / 群成员禁言、系统推送（最后） |
| P1 / P2 | BE-240 ～ BE-243 | （不计入两批） | 全群禁言、统计、敏感词、用户所在群 / 会话列表 |

**人日对照（im-server 部分）**

| 批次 | 本任务单 | PRD-admin 第 14 节"服务端" | 差异说明 |
|---|---|---|---|
| 第一批（含前置） | 13.5（4.5 + 9）+ BE-122 1.5 = 15 | 15 | 一致。PRD 的"解散 2"= BE-122（1.5，PRD-social 任务单，注明计入 PRD-admin）+ BE-209（0.5）；"契约测试与迁移 1"在本单为 BE-211（协议、错误码、迁移编排），各任务内的契约测试计入各自任务 |
| 第二批 | 19 | 19 | 一致。"推送 5"拆为 BE-228、BE-229 各 2.5；"隔离与定时任务 2"拆为 BE-220、BE-224 各 1 |
| 不在本单 | — | — | **社交管理接口（F-SOC-01 配置、F-SOC-12 好友导入 / 删除 / 查询、F-SOC-13 官方账号、F-SOC-30 群主交接）的 im-server 实现属于 PRD-social 任务单**，已计入 PRD-social 工作量；本单只要求它们挂在 BE-201 的 scope 机制和 BE-203 的审计上 |

依赖顺序：BE-200 → BE-201 → BE-202 → BE-203（X-5 完成）→ BE-205～BE-210 可并行（BE-209 等 BE-122）→ BE-211 收口第一批协议 →（PRD-core v1.3 第二批条目与 Schema 定稿）→ BE-220、BE-221 → BE-222～BE-227 →（二期 push 协议定稿）→ BE-228、BE-229。

环境约定：时间相关测试用 [IM 钟]（`POST /admin/test/clock`，BE-003 / IMP-7 冻结语义）；需要和 console-server 同时设钟的端到端用例用 `tools/set-clocks.sh`（console 任务单 CS-001 提供）。

---

## X-5 前置改造（第一批，F-KEY-01～05）

### BE-200 管理接口迁到独立内网监听地址（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-064（启动保护）、BE-051（`IM_METRICS_ADDR` 已独立）
- AC：AC-KEY-01-1、AC-KEY-01-2、AC-KEY-01-3、AC-KEY-01-4、AC-KEY-01-5、AC-KEY-01-6
- 内容：
  1. `config`：新增 `IM_ADMIN_ADDR`（默认 `127.0.0.1:8081`，`off` 关闭）、`IM_ADMIN_TLS_CERT` / `IM_ADMIN_TLS_KEY`、`IM_ADMIN_ALLOWED_CIDRS`；预留 `IM_OPS_ADDR`、`IM_PPROF_ADDR` 的解析（运维端口本身由 PRD-ops 任务实现）。
  2. `app.go` 拆成两个 mux：对外 mux 只有 `/ws`、`/healthz`、`/readyz`、（测试 / 演示）`/demo/login`；管理 mux 承载全部 `/admin/*` 和 `/admin/test/*`。`main` 分别 `ListenAndServe`，管理地址按配置走 TLS。对外端口上请求 `/admin/*` 返回 404（未注册路由）。
  3. 启动保护：`IM_ENV=production` 且管理地址非回环（`127.0.0.1` / `::1`）而没有 TLS 证书 → 非 0 退出并打印原因；`IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的两两相同 → 非 0 退出并列出冲突的两项。
  4. CIDR 中间件：只看 TCP 对端地址（`r.RemoteAddr`），**不解析 `X-Forwarded-For`**；不在范围内返回 403 / 4030 且不进入业务 handler。
  5. 测试：进程级测试（起子进程看退出码与日志）覆盖 01-2、01-3、01-5 两组；e2e 在两个端口上分别请求覆盖 01-1、01-6；01-4 用 `httptest` 注入对端地址 `192.168.1.10` + 伪造头。现有 `server/e2e` 改为向管理地址发管理请求（只改测试夹具中的地址，不改用例逻辑）。

### BE-201 调用方密钥、scope 目录与旧密钥过渡（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-200
- AC：AC-KEY-02-1、AC-KEY-02-2、AC-KEY-02-3、AC-KEY-02-4、AC-KEY-02-5、AC-KEY-02-6、AC-KEY-02-8、AC-KEY-05-1、AC-KEY-05-2、AC-KEY-05-3、AC-KEY-05-4
- 内容：
  1. `IM_ADMIN_CALLERS`（JSON 文件，启动时检查权限 0600）：`{id, alg: hmac-sha256|ed25519, secret|publicKey, scopes, allowedCIDRs?, disabled?, bodyAccess?}`；`id` 匹配 `[a-z0-9-]{3,32}`；HMAC 密钥 ≥ 32 字节；调用方之间、与 `IM_JWT_SECRET`、与 `IM_ADMIN_KEY` 两两不同，否则非 0 退出。
  2. scope 目录（PRD F-KEY-02 规则 2 全表）做成 Go 常量；管理路由注册改为 `adm.route(method, path, scope, handler)`，**未声明 scope 的 `/admin/*` 路由在注册阶段 panic（"route without scope"）**，并有一条单测遍历 mux 断言每个路由都有 scope。
  3. 正文类 scope（`msg.body.group`、`msg.body.c2c`、`msg.quarantine`）只能在 `bodyAccess: true` 的调用方上、全部调用方中最多 1 个、该调用方不能有其他 scope；违反 → 非 0 退出（AC-KEY-02-4）。`IM_C2C_BODY_ACCESS`（默认 `off`）只从环境变量读，**不提供任何管理接口修改它**；正文接口本身在 BE-225 实现。
  4. 鉴权结果：scope 不足 403 / 4030；调用方 `disabled` 或不存在 401 / 4017（签名路径，BE-202）。
  5. 旧密钥：`IM_ADMIN_KEY` 视为调用方 `legacy`，scope 白名单只含 v1.2 已有 9 个路由（按"路由 + 方法"列表，不按 scope 名，避免新路由复用旧 scope 时被旧密钥访问）+ 测试模式下 `/admin/test/*`；调用其他路由 403 / 4030。只在请求不带 `X-IM-Caller` 时走旧密钥路径，失败 401 / 4013；同时带签名头时以签名头为准。`IM_ENV=production` 且未设 `IM_ADMIN_LEGACY=1` 时不接受旧密钥（401 / 4013），设了则启动日志告警、`/readyz` 详情带 `legacyAdminKey: true`。
  6. 测试：参数化 e2e（AC-KEY-02-2）——测试表 = 全部管理路由 × {有该 scope 的调用方, 只有其他 scope 的调用方}，每个路由一个合法请求样本，表格放 `server/internal/admin/scope_test.go`，新增路由不加样本则测试失败；`console` 调用方按 PRD 规则 4 配置调用 `POST /admin/users/token` 返回 403（02-6）；现有 `server/e2e` 不改代码全过（05-1）。

### BE-202 请求签名与防重放（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201
- AC：AC-KEY-03-1、AC-KEY-03-2、AC-KEY-03-3、AC-KEY-03-4、AC-KEY-03-5、AC-KEY-03-6
- 内容：
  1. 头：`X-IM-Caller`、`X-IM-Timestamp`（毫秒）、`X-IM-Nonce`（16～64 个 base64url 字符）、`X-IM-Signature`（base64url），可选 `X-IM-Operator`（`{space}:{accountId}`）、`X-Request-Id`（≤ 64）、`X-IM-Target-Node`。
  2. 待签名串 8 行（`\n` 分隔）：METHOD、PATH（查询串按键名排序 + RFC 3986 编码）、TIMESTAMP、NONCE、`hex(SHA-256(body))`、Operator、Request-Id、Target-Node（缺省空串）。HMAC-SHA256 与 Ed25519（只存公钥）两种实现放 `internal/adminauth`，节点运维接口（PRD-ops F-OPS-16）复用同一个包。
  3. 窗口：`|now − ts| ≤ 300 秒`，`now` 取注入时钟（[IM 钟]）。
  4. nonce：签名校验**通过之后**才登记；同一调用方 10 分钟内重复拒绝。单节点内存实现（容量 100 万，满时 503 / 5030 + 告警日志）；接口 `NonceStore` 预留 Redis 实现（`SET NX EX 600`，随 B6 的 BE-073）。
  5. 失败统一 401 / 4017（缺头、调用方不存在 / 停用、签名错、超窗、nonce 重复、目标节点不符），`msg` 区分原因；指标 `im_admin_auth_failures_total{reason}`（`missing` / `caller` / `signature` / `window` / `nonce` / `node`），不写 `admin_audit`。
  6. `protocol/vectors/admin-sign.json`：HMAC、Ed25519 各 ≥ 3 组（含查询串排序、空 body、中文 body）；Go 单测按向量校验；`protocol/ADMIN.md` 附 Go / Java / Node.js 签名示例（示例代码同样跑向量，Java / Node 示例在 CI 中用容器执行一次）。
  7. 测试：篡改 body / Operator / 查询参数值（03-2）；[IM 钟] ±301 / −299 秒（03-3）；原样重放强制下线只踢一次、`admin_audit` 只有一条成功 result（03-4）；1000 个错签同 nonce 后正确签名成功（03-5）；缺签名头指标 +1 且审计无新增（03-6）。

### BE-203 im-server 侧管理接口审计 `admin_audit`（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-202、BE-060（迁移）、BE-061（契约测试）
- AC：AC-KEY-04-1、AC-KEY-04-2、AC-KEY-04-3、AC-KEY-04-4、AC-KEY-04-5
- 内容：
  1. 迁移（与 PRD-social 的 0004 / 0005 合并编排，见 BE-211）：表 `admin_audit(id, ts, caller, operator, request_id, route, targets_json, summary_json, phase, code)`，索引 `(ts)`、`(request_id)`、`(caller, ts)`。PRD-social F-SOC-12 规则 9 的审计即本表。
  2. `store.Store` 只增加 `AppendAdminAudit` 和 `QueryAdminAudit`，**没有更新 / 删除方法**；契约测试用反射断言接口中与 `AdminAudit` 相关的方法只有这两个（04-4）。过期清理（保留 1 年）由 BE-220 的定时任务调用专用的 `PurgeAdminAuditBefore`（单独的窄接口，只给清理任务注入）。
  3. 中间件：写接口、`user.sessions`、`msg.meta`（单聊）、`msg-grants`、正文读取、`audit.read` 执行前写 intent，失败返回 503 / 5030 且不执行；执行后写 result（同 `requestId`，`code` 为业务结果码）。`operator` 只取签名过的 `X-IM-Operator`，旧密钥为空、`caller=legacy`。`targets` 超过 50 个只记数量和前 50 个。
  4. 脱敏：`summary` 只放数量、旧值 → 新值；**不记正文、好友备注、推送 data 原文**（push 只记长度 + SHA-256）；handler 用白名单方式构造 summary，而不是序列化请求体。
  5. `GET /admin/audit?from=&to=&caller=&operator=&requestId=&cursor=&limit=`（scope `audit.read`）：`from` / `to` 必填、跨度 ≤ 31 天，否则 400 / 4000；游标分页按 `id` 倒序。
  6. 测试注入：测试模式 `POST /admin/test/faults {adminAuditWrite: "fail"}`（扩展 BE-003 的故障注入）用于 04-2；04-3 在好友导入（PRD-social 接口，未就绪时用测试模式路由）和第二批 push 上各跑一次特征串扫描（扫审计表全部列 + 服务端日志缓冲）。

---

## 第一批业务接口（G-ADM-05、06、09、10、11、12、25）

### BE-205 用户列表 / 搜索 `GET /admin/users?q=`（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201、BE-010（用户表）
- AC：AC-ADM-10-1、AC-ADM-10-2、AC-ADM-10-3、AC-ADM-10-6、AC-ADM-10-7、AC-ADM-90-1
- 内容：
  1. 接口 `GET /admin/users?q=&official=&cursor=&limit=`（scope `user.read`，不写审计）：`q` 去首尾空白后 1～64 字符；userId 精确 / 前缀（区分大小写）与昵称前缀（ASCII 不区分大小写，其他字符按码点精确）两路结果合并去重；`official=true` 只返回官方账号（`official` 列由 PRD-social 迁移提供，未就绪时该筛选返回空并有 TODO 测试）。`limit` 取 20 / 50 / 100，游标为 `(sortKey, userId)` 的不透明编码。
  2. 迁移：`users(nickname_fold)`（ASCII 小写化后的昵称）索引、`users(user_id)` 前缀可用的索引（SQLite 用 `>= q AND < q||x'FF'` 范围查询代替 `LIKE`）；必须用 `LIKE` 的地方先转义 `%`、`_`、`\` 并加 `ESCAPE '\'`（`_` 是合法 userId 字符，AC-ADM-10-3）。
  3. 返回项：`userId`、`nickname`、`avatar`（原样返回，由前端决定不加载）、`createdAt`、`online`（来自 `hub`，见 BE-206）、`official`；第二批由 BE-222 / BE-226 追加 `ban`、`mutedUntil`。
  4. 性能：`im-loadtest` 增加 `admin-search` 场景（SQLite：10 万用户、P95 ≤ 1 秒；Postgres（B6 之后）：100 万用户、3 次 P95 中位数 ≤ 1 秒），报告写 `docs/reports/perf-admin-*.md`（AC-ADM-90-1 在 BE-070 之后验收）。

### BE-206 连接元数据与在线设备查询 `POST /admin/users/sessions/query`（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201、BE-203
- AC：AC-ADM-14-1、AC-ADM-14-6、AC-ADM-10-4
- 内容：
  1. `gateway.conn` 在鉴权成功时记录 `connId`、`platform`、`connectedAt`、`node`（`IM_NODE_ID`）、**脱敏后的来源 IP**（IPv4 保留前 2 段 `a.b.*.*`，IPv6 保留前 3 组 / 48 位）——完整 IP 不进连接元数据和断开缓冲（PRD-ops F-OPS-09 共用）。来源 IP 按 BE-066 的可信代理规则解析后再脱敏。
  2. `hub` 增加 `ListUserConns(userId)`；新建 `internal/sessions.Query(userId)` 作为**唯一查询函数**，管理接口与 PRD-ops 节点运维接口（`IM_OPS_ADDR`）都调用它（评审 X-05）；返回字段：连接 ID、平台、建立时间、节点、脱敏 IP、下行队列长度。
  3. 接口 `POST /admin/users/sessions/query {userId}`（scope `user.sessions`，写 `admin_audit` intent / result，`summary.count`）；userId 只在请求体中。B6 之前响应带 `partial: true, scope: "localNode"`；B6 之后（BE-073 Presence）汇总各节点，未响应节点列入 `unresponsiveNodes`，**不返回"不在线"的结论**（AC-ADM-14-6，依赖 B6，在 BE-075 多节点 e2e 中验收）。
  4. `GET /admin/users/{id}` 增加 `online`（布尔），列表同（AC-ADM-10-4）。
  5. 测试：e2e 用 `X-Forwarded-For` 经可信代理模拟 `10.1.2.3`，断言返回 `10.1.*.*`；单测覆盖 IPv6 脱敏；断言 `gateway` 结构体中没有保存完整 IP 的字段（反射 + grep 测试）。

### BE-207 群列表 / 搜索 `GET /admin/groups?q=`（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201、BE-030
- AC：AC-ADM-20-1、AC-ADM-20-2、AC-ADM-20-3、AC-ADM-20-4
- 内容：
  1. 接口 `GET /admin/groups?q=&includeDissolved=&cursor=&limit=`（scope `group.read`，不写审计）：groupId 精确 / 前缀（区分大小写）、群名前缀（ASCII 不区分大小写）；默认按 `created_at` 倒序；默认排除 `dissolved_at IS NOT NULL`，`includeDissolved=true` 时返回并带 `dissolvedAt`。
  2. 返回项：`groupId`、`name`、`type`（`managed` / `user`：M4 之前全部 `managed`；`ug-` 前缀为自建群）、`owner`（M4，来自 PRD-social F-SOC-20 的角色表，未就绪时为 `null`）、`memberCount`（PRD-social F-SOC-11 的计数列；未就绪时 `COUNT(*)` 兜底，并与群详情一致）、`historyVisible`、`createdAt`。
  3. 迁移：`chat_groups(name_fold)`、`chat_groups(created_at)` 索引；`dissolved_at` 列由 BE-122（backend-social.md）添加（迁移编排见 BE-211）。
  4. 测试：e2e 覆盖前缀、成员数一致、解散群过滤；AC-ADM-20-4 在 PRD-social M4 自建群可用后补跑（任务内先用存储层直接插入 `ug-` 群验证类型与群主列）。

### BE-208 群成员分页 `GET /admin/groups/{id}/members`（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201、BE-030
- AC：AC-ADM-22-1、AC-ADM-22-6
- 内容：
  1. 接口 `GET /admin/groups/{id}/members?cursor=&limit=&prefix=`（scope `group.read`）：按 userId 升序游标分页，`limit` 默认 50（20 / 50 / 100），`prefix` 为 userId 前缀（区分大小写，转义同 BE-205）；返回 `userId`、`joinedAt`、（M4）`role`、（第二批，BE-227）`mutedUntil`。已解散或不存在的群 404 / 4043。
  2. 存储方法 `ListGroupMembers(groupId, after, limit, prefix)` 进 `storetest`，与客户端 `groupMembers`（PRD-social F-SOC-28，M4，BE-135）共用，本任务先实现；BE-135 在 M4 把排序改为"群主 → 管理员 → 成员"并加角色字段时，管理接口沿用同一方法（`prefix` 筛选时按 userId 排序）。
  3. 测试：1500 人群逐页 30 页无重复；前缀 `u1` 只命中 `u10`、`u11`。

### BE-209 解散群：管理侧收口（scope、审计、验收）（0.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：**BE-122**（[backend-social.md](backend-social.md)：`DissolveGroup`、`DELETE /admin/groups/{id}`、`convRemoved.reason`、F-SOC-26 规则 3 全部读路径、契约测试 `GroupDissolveNoReuse`；该任务 1.5 人日计入 PRD-admin 工作量，**本单不重复实现**）、BE-201、BE-203
- AC：AC-ADM-24-1、AC-ADM-24-2、AC-ADM-24-3、AC-ADM-24-5、AC-ADM-24-8
- 内容：
  1. `DELETE /admin/groups/{id}` 声明 scope `group.dissolve`；旧密钥调用 403 / 4030（属于新接口，AC-KEY-05-2）；写 `admin_audit` intent / result（`targets=[groupId]`）。
  2. 核对 BE-122 已覆盖：已解散群 `send` / `sync` / `read` → 4043；管理接口查询、加成员、改设置 → 404 / 4043；同 ID 建群 → 409 / 4090；`GroupDissolveNoReuse` 每个查询一条断言（AC-ADM-24-8）；在线成员 1 秒内收到 `convRemoved {reason:"dissolved"}`；离线成员上线 `convs` 无该群（AC-ADM-24-5 的服务端部分，H5 部分见 FE-430）。
  3. 本任务粒度小于 1 人日：它是 BE-122 与 X-5 的接缝，单列是为了让 PRD-admin 的 AC 有明确的验收归属。

### BE-210 会话消息元数据查询 `POST /admin/convs/messages/query`（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-201、BE-203、BE-209（已解散群仍可查）
- AC：AC-ADM-30-1、AC-ADM-30-2、AC-ADM-30-3、AC-ADM-30-4、AC-ADM-30-7、AC-ADM-24-4、AC-ADM-90-2
- 内容：
  1. 接口 `POST /admin/convs/messages/query {convId, beforeSeq?, limit≤50, from?, sinceTs?, untilTs?}`（scope `msg.meta`）：按 seq 倒序；返回 `seq`、`msgId`、`from`、`type`、`bodyLength`（码点数）、`ts`，第二批加 `deleted`、`deletedBy`、`deletedAt`；**响应结构体没有 `body` 字段**（用独立的 `MessageMeta` 类型，单测断言 JSON 中不出现 `body` 键）。
  2. 不受 `historyVisible` 和成员关系限制；已解散群照常返回；会话不存在返回空列表（与"存在但无消息"一致）；会话 ID 格式错误 400 / 4007（4007 扩展 `admin` 通道、`http: 400`，见 BE-211）。
  3. 审计：单聊会话写 intent / result（`targets=[convId]`）；群会话按 F-KEY-04 规则 1 不强制，但同样记录（console 侧另有审计）。
  4. 迁移：索引 `messages(conv_id, from_user, seq)`、`messages(conv_id, ts)`；`bodyLength` 用存储时已有的长度或查询时计算（SQLite `length()` 按字符计；Postgres `char_length`），在契约测试中断言中文与 emoji 的码点计数一致。
  5. 性能：`im-loadtest` 增加 `admin-meta` 场景（单会话 1 万条 / SQLite，10 万条 / Postgres，P95 ≤ 500ms）；AC-ADM-90-2 在 BE-070 之后验收。
  6. 测试：e2e 120 条三页、按发送者筛选、`c2c:a` → 4007、从未聊过的单聊返回空、抓 im-server 响应找不到特征串 `ZXQ-BODY-123`。

### BE-211 第一批协议、错误码与迁移编排（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-200 ～ BE-210 的接口确定后收口（错误码可先行）
- AC：AC-KEY-02-2、AC-ADM-30-3
- 内容：
  1. `errors.json`：一次写入 PRD 第 10.1 节 14 个新码（4014、4015、4016、4017、4034、4037、4038、4039、4044、4092、4093、4291、5031、5040），新增 `scope: "console"`；10.2 节扩展（4000、4004、4030、4040、4130、4131、4290、5000、5030 加 `console`；4007 加 `admin` / `console`、`http: 400`；4031 去掉"二期"、补 `details`；4292 加 `admin` / `console`；4046 补禁言用途；`kickReasons` 加 `suspended`；`closeCodes` 加 1011 `INTERNAL_ERROR`）。
  2. `server/internal/protocol/errors_test.go` 的 `TestCodesMatchErrorsJSON` 跳过只含 `console` scope 的码，im-server 只为 4017、4034、4044 定义 Go 常量。
  3. `ERRORS.md` 按 PRD 10.4 节 1～7 条修改（区段表、console 通道、透传规则、第 5 节差异、413x 用 HTTP 400）。
  4. `ADMIN.md`：AD-01～AD-06、AD-09（第一批部分）：鉴权改为调用方 + 签名、每个接口的 scope 列、审计小节、新接口、群解散规则、社交接口上线条件、运维配置项。
  5. Schema（`frame.schema.json`）：`convRemoved.reason`（`removed` / `dissolved`）、`Error.details`（对象，先发供第二批使用）；`protocol/README.md` 写明"SDK 忽略不认识的事件类型、字段和枚举值"；`vectors/frames.json` 增加未知事件 / 未知字段 / 未知 reason 用例（与 PRD-social V-18 同一条，若 PRD-social 任务已写则只核对）。
  6. 迁移编排：本 PRD 第一批迁移（`admin_audit`、`dissolved_at`、搜索 / 元数据索引）与 PRD-social 0004 / 0005 合并排号，迁移文件头注明来源 PRD 与任务号；`im-server migrate status` 输出不变。
  7. 测试：错误码一致性测试、Schema 合法 / 非法用例、迁移 up 后契约测试全过。AC-KEY-02-2 的参数化表在此任务结束时补齐全部第一批路由。

---

## 第二批（PRD-core v1.3 变更；服务端开关 `IM_V13_ADMIN_FEATURES` 默认关闭）

### BE-220 进程内定时任务机制（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-203
- AC：AC-ADM-32-3
- 内容：
  1. `internal/scheduler`：注册"名称 + 周期 + 函数"，使用注入时钟（[IM 钟] 推进后测试可手动触发：测试模式 `POST /admin/test/jobs/{name}/run`）；集群下只在一个节点运行（单节点直接运行；B6 用 Postgres advisory lock / Redis 锁，接口预留）。
  2. 首批任务：`admin_audit` 过期清理（保留 1 年）、隔离原文到期物理删除（BE-224）、按用户推送 7 天清理（BE-229）；PRD-social 的 `PurgeSocial` 也挂在这里。
  3. 每次运行写日志和指标 `im_job_runs_total{job,result}`、`im_job_duration_seconds`。
  4. 测试：单测（推进时钟触发、失败不影响下次）；与 BE-224 一起验收 AC-ADM-32-3。

### BE-221 第二批协议 Schema 与错误码说明（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：PRD-core v1.3 第二批条目与 Schema 定稿（PRD-admin 11.1、11.2 节）、BE-211
- AC：AC-ADM-31-2、AC-ADM-40-6
- 内容：
  1. `frame.schema.json`：`Message.body` 条件必填（`deleted=true` 时不出现），增加 `deleted`、`deletedAt`；`LastMsg.deleted`；新事件 `msgDeleted {convId, msgIds, seqs}`、`muted {scope, groupId?, mutedUntil}`、`unmuted {scope, groupId?}` 加入 `EventFrame.oneOf`；`kicked.reason` 加 `suspended` 和可选 `until`；`auth` ack 可选 `mutedUntil`；`convs` 群会话项可选 `mutedUntil`。
  2. `vectors/frames.json` 增加上述合法 / 非法用例，三套 SDK 同步（SDK 侧见 FE-431～FE-436）。
  3. `ADMIN.md` AD-07（封禁、禁言、删除、正文授权接口，`GET /admin/users/{id}` 的 `ban` / `mutedUntil`，签发 token 的 4034）；ERRORS.md 第 4 节补 `suspended`。
  4. 开关 `IM_V13_ADMIN_FEATURES`：关闭时第二批管理接口不注册、不下发新事件；sdk-ts 对应版本发布后再打开（发布顺序见 PRD 11.1 节）。
  5. 测试：Schema 用例；开关关闭时第二批路由 404。

### BE-222 封禁 / 解封（2 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-221、BE-013（令牌版本号）、BE-014（演示登录）
- AC：AC-ADM-13-1、AC-ADM-13-2、AC-ADM-13-3、AC-ADM-13-4、AC-ADM-13-5、AC-ADM-13-6、AC-ADM-13-7、AC-ADM-13-8
- 内容：
  1. 迁移：`user_bans(user_id PK, until NULL=永久, reason, by, at)`；重复封禁覆盖（审计每次都有）。
  2. 接口 `POST /admin/users/{id}/ban {durationSeconds|null, reason}`、`POST /admin/users/{id}/unban {reason}`（scope `user.ban`，审计）；`by = {caller}:{operator}`；用户不存在 404 / 4042。`GET /admin/users/{id}` 与列表增加 `ban: {until, reason, by, at} | null`。
  3. 生效：同一流程中 `token_version + 1`（复用 BE-013 机制，旧 token 作废）并踢掉全部连接：`kicked {reason: "suspended", until}` + 关闭码 4012。
  4. 签发检查：`POST /admin/users/token` 与 `/demo/login` 在签发前按 [IM 钟] 判断 `until IS NULL OR until > now` → 403 / 4034，`details.bannedUntil`；到期自动失效（不需要定时任务）。握手时 token 版本号已作废，所以封禁前签发的 token 连接即 `kicked` + 4012（13-3）。
  5. 不改资料、好友、群成员、历史消息（13-6 用 e2e 断言）。
  6. 测试：e2e 覆盖 13-1～13-8；[IM 钟] 推进 61 分钟验证到期。

### BE-223 删除违规消息（软删除）（3 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-221、BE-224（同一事务写隔离表，两任务一起合入）、BE-040（`conv_seq` 摘要）
- AC：AC-ADM-31-1、AC-ADM-31-2、AC-ADM-31-5、AC-ADM-31-6、AC-ADM-31-7、AC-ADM-31-8、AC-ADM-31-9
- 内容：
  1. 迁移：`messages.deleted_at`、`messages.deleted_by`；删除 = 清空 `body`、写删除标记，**保留 seq**（不留空洞）。
  2. 接口 `POST /admin/messages/delete {convId, msgIds, reason}`（scope `msg.delete`，审计记 msgId 列表和理由，不记原文）：单次 ≤ 50（超出 400 / 4131）；msgId 不在该会话 400 / 4000；不存在 404 / 4044；`type=tip` 400 / 4000；已删除的幂等成功、不再发事件。同时开放给业务方（`deletedBy = {业务方 id}:`）。
  3. 同一事务：写 `message_quarantine`（BE-224）、清空正文、若为会话最后一条则把 `conv_seq` 摘要改为 `lastMsg.deleted=true`（无 `text`）；未读公式不变（已删除的仍计入）。
  4. 事件：在线参与者（单聊双方、群全体成员，含发送者其他端）收到 `msgDeleted {convId, msgIds, seqs}`；`sync` 返回 `deleted=true`、无 `body`。
  5. 测试：e2e 4 个连接收到事件、离线成员 sync seq 连续、最后一条摘要、边界错误码、业务方调用方效果相同、审计特征串扫描。

### BE-224 被删原文加密隔离与到期物理删除（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-220、BE-223
- AC：AC-ADM-32-1、AC-ADM-32-3、AC-ADM-32-4
- 内容：
  1. 迁移：`message_quarantine(msg_id PK, conv_id, nonce, ciphertext, kid, deleted_at, expires_at)`；AES-256-GCM，密钥 `IM_QUARANTINE_KEY`（≥ 32 字节），每条随机 nonce，AAD = `msgId|convId`。
  2. 启动保护：`IM_ENV=production` 时 `IM_QUARANTINE_KEY` 未配置、< 32 字节、与 JWT 密钥 / 管理密钥 / 调用方 HMAC 密钥相同 → 非 0 退出。
  3. `IM_QUARANTINE_DAYS`（默认 30，范围 1～365）；BE-220 每小时任务物理删除 `expires_at <= now` 的行；到期后查看返回 404 / 4044（读取接口在 BE-225）。
  4. 测试：检查 SQLite 文件原始字节中 `messages`、`message_quarantine` 都搜不到特征串 `ZXQ-QUAR-321`；[IM 钟] 推进 30 天 1 小时后触发任务，行消失；进程级启动保护测试。

### BE-225 正文查看授权、读取与配额（2 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-201（`bodyAccess` 调用方）、BE-224、PRD-admin 15 节 Q-1 确认（不阻塞第一批）
- AC：AC-KEY-02-7、AC-ADM-08-4、AC-ADM-08-7、AC-ADM-08-8、AC-ADM-08-9、AC-ADM-32-2、AC-ADM-32-5
- 内容：
  1. 迁移：`msg_grants(grant_id PK, caller, operator, conv_id, quarantine bool, reason, ticket, created_at, expires_at)`；配额计数按 Asia/Shanghai 自然日：每操作人每天 20 个、`bodyAccess` 调用方每天 200 个（正文与隔离原文合并计算），超出 429 / 4292。
  2. `POST /admin/msg-grants {convId, operator, reason, ticket, quarantine?}`（scope 按会话类型 `msg.body.group` / `msg.body.c2c`；`quarantine=true` 需 `msg.quarantine`）：单聊在 `IM_C2C_BODY_ACCESS=off` 时 403 / 4030 `reason: "c2cBodyDisabled"`，不产生授权；返回 `grantId`、`expiresAt`（15 分钟，[IM 钟]）。`console` 主凭证没有该 scope → 403 / 4030（08-9）。
  3. `POST /admin/convs/messages/body {grantId, convId, beforeSeq, limit≤50}`：授权不存在、过期或 convId 不符 → 403 / 4039（im-server 侧同码，console 透传）；`quarantine` 授权读取 `message_quarantine` 解密结果，到期 404 / 4044。响应 `Cache-Control: no-store`。
  4. 审计：授权与每页读取写 `admin_audit`（seq 范围、条数，不含正文）。
  5. 测试：e2e 覆盖 c2c 开关、配额第 21 个、主凭证 403、[IM 钟] 推进 16 分钟后 4039、换会话 4039、隔离原文读取。

### BE-226 用户级禁言（2 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-221、BE-033（`cache.Invalidator`）、BE-023（发送限流）
- AC：AC-ADM-40-1、AC-ADM-40-2、AC-ADM-40-3、AC-ADM-40-4、AC-ADM-40-6、AC-ADM-40-7、AC-ADM-40-8、AC-ADM-40-9
- 内容：
  1. 迁移：`mutes(scope, user_id, group_id, until, reason, by, at)`，主键 `(scope, user_id, group_id)`；重复禁言覆盖。
  2. 接口 `POST /admin/mute {scope:"user", userId, durationSeconds|null, reason}`、`POST /admin/unmute {scope:"user", userId, reason}`（scope `mute.write`，审计）；用户不存在 404 / 4042；`GET /admin/users/{id}` 增加 `mutedUntil`；官方账号同样可禁言。
  3. 发送路径：**禁言检查在发送限流之前**（4031 不扣令牌，C-29），ack 错误 4031 `details {scope:"user", mutedUntil}`；按 [IM 钟] 判断到期；状态读缓存，经 `cache.Invalidator` 主题 `mutes` 失效 + TTL 60 秒兜底。
  4. 事件：在线连接收到 `muted {scope:"user", mutedUntil}` / `unmuted {scope:"user"}`；`auth` ack 增加 `mutedUntil`（未禁言不出现）。接收、sync、read、好友申请不受影响。
  5. 测试：e2e 覆盖 40-1～40-8；40-9 用 PRD-social 5.6 节双节点测试环境（`Invalidator` 跨节点，1 秒内生效）。

### BE-227 群成员禁言（2 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-226、BE-209（解散时清除）、BE-208（成员列表字段）
- AC：AC-ADM-41-1、AC-ADM-41-2、AC-ADM-41-3、AC-ADM-41-4、AC-ADM-41-5、AC-ADM-41-6
- 内容：
  1. `POST /admin/mute {scope:"group", groupId, userId, ...}` / `unmute`：被禁言者不是成员 404 / 4046；群主、管理员（M4）也可被禁言。
  2. 发送：同时存在用户级与群内禁言时返回 `scope:"user"`，`mutedUntil` 取用户级；群内为 `details {scope:"group", groupId, mutedUntil}`。
  3. 移出群、退群、解散时在同一事务删除该群禁言记录（`RemoveGroupMembers`、`DissolveGroup` 内），再入群不继承。
  4. 事件只发给本人 `muted {scope:"group", groupId, mutedUntil}`；`convs` 群会话项带 `mutedUntil`；`GET /admin/groups/{id}/members` 每个成员带 `mutedUntil`。
  5. 测试：e2e 覆盖 41-1～41-6。

### BE-228 系统推送接口 `POST /admin/push`（2.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：[PRD-live v1.1](../prd/PRD-live-v1.1.md)（push 协议，已冻结）、BE-221、BE-203。开关 `IM_PUSH_ENABLED`
- AC：AC-ADM-50-1、AC-ADM-50-6、AC-PUSH-01-1、AC-PUSH-01-2、AC-PUSH-02-1、AC-PUSH-02-2、AC-PUSH-02-3、AC-PUSH-02-4、AC-PUSH-02-5、AC-PUSH-02-6、AC-PUSH-02-8、AC-PUSH-02-9、AC-PUSH-02-10、AC-PUSH-02-11、AC-PUSH-02-12、AC-PUSH-02-13、AC-PUSH-02-14、AC-PUSH-06-1、AC-PUSH-06-2、AC-PUSH-06-4、AC-PUSH-06-7、AC-PUSH-06-8、AC-PUSH-06-9、AC-PUSH-06-10、AC-PUSH-06-11、AC-PUSH-06-12、AC-PUSH-06-14、AC-PUSH-06-16、AC-PUSH-09-1、AC-PUSH-09-2、AC-PUSH-09-3、AC-PUSH-09-4、AC-PUSH-09-5、AC-PUSH-09-6、AC-PUSH-09-7、AC-PUSH-09-8、AC-PUSH-09-9、AC-PUSH-10-1
- 内容：
  1. `POST /admin/push {target:"user"|"all", userIds?, type, data, pushId}`（scope `push.user` / `push.all`）：`userIds` > 1000 → 400 / 4131；含不存在用户 → 404 / 4042，`details.userIds` 列出，整批不发；`data` ≤ 4KB（模板 Schema 校验在 console 侧，CS-044）。
  2. 下行 `push` 帧按 PRD-live 4.2 节，`pushId` 幂等按 4.4 节：幂等表 `push_idempotency`、4094（写入 errors.json 和 Go）、`ttlSeconds` 可选字段、`pushTypes` 白名单（F-PUSH-09 规则 3）。协议、Schema、`frames.json`、`ADMIN.md` 按 PRD-live 第 10 节标 L1 的行落实。
  3. 审计：`summary` 只记目标数量、`target`、type、`pushId`、data 长度与 SHA-256，走 `auditNote`（现有白名单不含）。
  4. 测试：e2e 两个在线用户 3 秒内收到；不存在用户与超量错误码。用例见 TC-PUSH。

### BE-229 按用户推送的离线队列与全员在线投递（2.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-server
- 依赖：BE-228、BE-220
- AC：AC-ADM-50-2、AC-ADM-50-3、AC-PUSH-03-1、AC-PUSH-03-2、AC-PUSH-03-3、AC-PUSH-03-5、AC-PUSH-03-6、AC-PUSH-03-8、AC-PUSH-03-12、AC-PUSH-03-13、AC-PUSH-03-14、AC-PUSH-03-21、AC-PUSH-03-22、AC-PUSH-03-23、AC-PUSH-04-1、AC-PUSH-04-2、AC-PUSH-04-3、AC-PUSH-04-4、AC-PUSH-04-5、AC-PUSH-04-6、AC-PUSH-07-1、AC-PUSH-07-2、AC-PUSH-07-4、AC-PUSH-11-1、AC-PUSH-11-2、AC-PUSH-11-3、AC-PUSH-11-4、AC-PUSH-11-5、AC-PUSH-11-6
- 内容：
  1. 迁移：`push_inbox(user_id, push_seq, push_id, type, data, created_at, expires_at)`、`push_seq(user_id, max_seq)`（不随清理删除）；命令 `pushSync`、读取时按 `expires_at` 过滤、收件箱上限、清理顺序（先收件箱后幂等记录，PRD-live 4.3、4.4 节），BE-220 每小时分批清理。
  2. 全员推送只投递当前在线连接，异步下发，不另设分批参数（F-PUSH-04 规则 2），集群下经消息总线广播（B6）；多端在线时每个连接各收一次。`im-loadtest` 新增全员场景（AC-PUSH-04-2 要用，`scenarios.go` 现在没有）。
  3. 测试：[IM 钟] 第 6 天上线收到、第 8 天收不到；5 个在线收到、离线用户上线收不到。

---

## P1 / P2（不在两批内）

### BE-240 全群禁言（P1）（1.5 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-server
- 依赖：BE-227、PRD-social M4（群主 / 管理员角色）
- AC：AC-ADM-42-1、AC-ADM-42-2
- 内容：`POST /admin/mute {scope:"groupAll", groupId}` / `unmute`；群主、管理员仍可发言，其他成员 4031 `details.scope="groupAll"`；全群在线成员收到 `muted {scope:"groupAll", groupId}`；`convs` 带 `allMuted=true`。e2e 覆盖。

### BE-241 统计接口 `GET /admin/stats`（P1）（1 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-server
- 依赖：BE-206；集群下依赖 BE-073 Presence
- AC：AC-ADM-70-1
- 内容：返回在线连接数（按平台）、今日新增用户、今日消息数、群总数（"今日"按 Asia/Shanghai）；scope 新增 `stats.read`（加入 F-KEY-02 目录与 ADMIN.md）；不读 `/metrics`。

### BE-242 敏感词与发送前检查（P2）（3 人日）
- 优先级：P2
- 批次：P1（P2 功能，两批之外）
- 仓库：tianting-server
- 依赖：ARCH S14、错误码届时分配（建议 4510）
- AC：AC-ADM-60-1、AC-ADM-60-2
- 内容：词库表（拦截 / 替换）、管理接口、发送前检查（拦截不落库、替换为 `*`），词库变更经 `cache.Invalidator` 通知所有节点（10 秒内生效）。排期另定。

### BE-243 用户所在群与用户会话列表（P1）（1 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-server
- 依赖：PRD-social 0004 的 `group_members(user_id)` 索引
- AC：（无对应 AC：G-ADM-07、G-ADM-13 为 P1 接口）
- 内容：`GET /admin/users/{id}/groups`（scope `group.read`）、用户会话列表元数据（scope `msg.meta`，写审计）；供后台用户详情"所在群"、元数据入口 ③。
