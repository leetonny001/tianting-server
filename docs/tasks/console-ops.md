# console-server 任务单：运维模块（PRD-ops v1.0）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-ops v1.0](../prd/PRD-ops-v1.1.md)（冻结，167 条 AC）第 2～7 节；[PRD-admin v1.0](../prd/PRD-admin-v1.0.md) 第 4 节「console-server 公共基座」（F-CON-01～12）、F-KEY-03（签名）、2.3 / 2.4 节（通用约定、[C 钟] / [IM 钟]、故障注入）、第 10 节错误码；[ARCH-v1](../arch/ARCH-v1.md) |
| 负责 | 后端（AI） |
| 仓库 | `tianting-console-server`（**尚未建**，由基座任务 [console.md](console.md) 建仓库与工程骨架；本任务单只做运维模块） |
| 与基座的分工 | 账号空间、登录、TOTP、激活码、恢复码、锁定与限流、会话（Cookie、空闲 / 绝对过期、后台请求不续期、并发上限）、CSRF、角色与授权机制（可叠加、互斥、权限点声明、应急授权）、重新验证、审计（哈希链、写失败即失败、校验命令、查看、导出）、站内通知、测试模式（[C 钟]、审计写入失败注入）、启动保护框架**全部由基座 CS 任务（console.md，CS-001～099）实现**。本任务单只做：① 运维空间在基座上的配置与运维专属增量（B-OPS-01～15 中基座之外的部分）；② 运维业务模块（节点、总览、排查、配置、数据库与备份、Grafana 代理、告警读取与静默）；③ 运维侧对基座行为的验收测试 |
| 环境依赖 | Postgres、Prometheus、Alertmanager、Grafana、MinIO 由测试环境提供（见 [backend-ops.md](backend-ops.md) BE-320），不在开发机安装；im-server 运维接口见 BE-301～305 |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（Go 单测 / e2e）；接口清单 YAML 与第 6 节一致并驱动参数化权限测试；`go test -race ./...` 全过；依赖扫描与 im-server 一致（S11）；用例编号见 [TC-OPS](../testcases/TC-OPS.md) |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。"依赖：基座 CS 任务（console.md）"表示该能力的实现在基座里，本任务只做运维空间的配置、接入和验收。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| CS-100 ～ CS-103 运维空间与基座增量 | ✅ 2026-11-04（只在本地） | console-server `1c6f6d6`。CS-102 的基座行为由基座用例在运维空间上参数化覆盖了一部分，没有逐条补 AC-OPS-01-x 的运维专用用例 |
| CS-110 ～ CS-117 运维业务模块 | ✅ 2026-11-04（只在本地） | 接口清单 YAML 启动时比对；节点真实（两个 im-server，含 kill、SIGSTOP、TLS 身份校验）；Prometheus、Alertmanager、备份桶、Grafana 是假服务（Go 里假桶校验签名和权限）。排查按基座两阶段写审计：一次查询一条 intent + 一条 result，条数、partial、节点在 result 上（`deploy/ops/reconcile` 的导出要取 result）。私钥轮换：节点先配两把公钥，再换 keyFile 重启 console-server，没有做同一节点两把私钥热切换。**未验证**：真实 Grafana / Prometheus / MinIO 策略（BE-310、311、313）、AC-OPS-90-x 性能（CS-120）。备份清单对象键约定 `manifests/{source}/{finishedAt 补零 13 位}.json`，BE-313 要按这个写 |
| CS-120 性能与对账验收 | 🟡 联调和对账完成；AC-OPS-90-2 只完成开发机部分；AC-OPS-04-2 待决定（P-11） | `tools/ops-accept`，由 tianting-server 的 `deploy/testenv/verify.sh --console` 调用（testenv run 41835233061，全部真实容器）。AC-OPS-90-3：双节点用户 300 次排查 P50 9ms、P99 261ms、最大 374ms；AC-OPS-14-6：审计 321 条、节点请求 321 个、差异 0 条；AC-OPS-08-11（接口）：1.6 秒返回部分结果，不判为不在线。同一轮读真实组件也通过：Prometheus（04-5，消息速率 10.0/秒）、Alertmanager（07-5）、MinIO（12-3）、Grafana（票据、认证代理、14 个面板、数据查询、写接口被拒）、版本不一致超过 30 分钟（04-6）。**未过**：AC-OPS-04-2 的今日累计 1237.2（v1.1 要求 1200 ±3%），`increase()` 的外推造成，见 P-11，testenv 因此仍是失败。AC-OPS-90-2 开发机部分通过（总览 P99 8ms，单节点排查 P99 19ms，见 perf-ops-2026-11-04.md），但总览没有真实 Prometheus 查询，偏乐观；Grafana 24 小时面板首次加载没有实现（P-12）。页面文案只在 tianting-ops 的 Playwright 里验过，后端用假服务。联调查出并修复：`Node.Call` 名额满时立刻返回 ErrBusy，几个工程师同时排查就把正常节点报成未响应，改为最多排队 500ms。 |
| CS-130 ～ CS-132 第二批 / P1 | 🟡 2026-11-06 | tianting-console-server `fdc59f3`；Alertmanager 用假服务验证，真实 Alertmanager 和 AC-OPS-18-3 的 kill -9 未验证（`tools/ops-accept` 的 silence 步骤只编译过）；CI 未执行（P-21） |
| CS-133 | 🟡 2026-11-06 | tianting-console-server `945992d`，迁移 0011；`tools/ops-accept` 和 `tt bootstrap` 第二超管只编译过，没有真实跑；CI 未执行（P-21） |
| CS-140 ～ CS-142 B6 | ⏸ 随 B6 | |

## 总览

| 阶段 | 任务 | 人日 | 对照 PRD-ops 10.2 节 |
|---|---|---|---|
| 第一批：运维空间与基座增量 | CS-100、CS-101、CS-103 | 3 | 公共基座中运维侧的增量 2～3 |
| 第一批：运维侧对基座的验收 | CS-102 | 1.5 | 测试 3～5 的一部分 |
| 第一批：运维业务模块 | CS-110 ～ CS-117 | 10 | console-server 运维模块 6～9 |
| 第一批：性能与对账验收 | CS-120 | 1 | 测试 3～5 的一部分 |
| **第一批合计** | 13 个任务 | **15.5** | |
| 第二批 | CS-130、CS-131 | 3 | PRD：第二批约 3～4（含前端），不在 28～40 内 |
| P1 | CS-132 | 0.5 | 随基座审计导出（F-CON-09 规则 7） |
| B6 | CS-140 ～ CS-142 | 2.5 | 随 B6 计入 |

> 运维业务模块按 10 人日计，比 PRD 的 6～9 多约 1 人日：扇出框架（部分结果、并发与超时、TLS 服务名校验）和 Grafana 认证代理（票据、主机 Cookie 与原会话绑定、续期规则）各按 1.5 人日计。三份任务单合计对照见 [backend-ops.md 总览](backend-ops.md#总览)。

依赖顺序：基座 CS 任务（console.md）→ CS-100 → CS-101 → CS-102 / CS-103；CS-110 → CS-111（+ BE-301）→ CS-112 → CS-113；CS-111 + BE-302 / BE-303 → CS-114；CS-111 + BE-304 → CS-115；CS-111 + BE-305 + BE-313 → CS-116；CS-101 + BE-310 → CS-117。

---

## 第一批：运维空间与基座增量

### CS-100 运维空间接入：域名、Cookie、来源网段、可信代理、响应头、运维启动保护
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：账号空间、会话、网络边界与启动保护框架（F-CON-01、F-CON-05、F-CON-10）
- AC：AC-OPS-01-6、AC-OPS-01-14、AC-OPS-03-1、AC-OPS-03-2、AC-OPS-03-3、AC-OPS-03-4、AC-OPS-03-6、AC-OPS-14-7
- 内容：
  1. 注册 `ops` 空间：主机 `ops.<内网域名>`、Cookie 名 `__Host-ops_sid`（B-OPS-04）、只接受 `ops` 空间账号登录；带 `__Host-adm_sid` 的请求在运维主机上视为无会话（401 / 4014）。
  2. 运维空间来源网段 `CON_OPS_ALLOWED_CIDRS`（B-OPS-11）：网段外请求在登录前 403、不返回登录页内容、不写审计，只计 `console_rejected_total{reason="cidr"}`；未登录与 CSRF 失败同样只计指标；只信任可信代理的 `X-Forwarded-For`，生产经代理时要求 `X-Forwarded-Proto=https`（否则 403 / 4030）。
  3. 运维 CSP 与响应头（F-OPS-03 规则 4）：`default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; object-src 'none'; frame-ancestors 'none'`，不允许 `data:`；`X-Frame-Options: DENY`、`nosniff`、`Referrer-Policy: no-referrer`、HSTS；`/console/api/` 全部 `Cache-Control: no-store`。
  4. B-OPS-15 运维启动保护（挂在基座启动保护框架上，一次列出全部问题）：节点凭证表中每个节点有地址、TLS 服务名、可读取的合法 Ed25519 私钥；Grafana 代理共享配置已设置；运维网段未配置或含 `0.0.0.0/0`、`::/0` 时拒绝启动；与基座检查（如测试模式）合并输出。
- 估算：1 人日

### CS-101 运维角色与权限：权限点、互斥角色、无应急授权、生命周期约束、敏感角色通知
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：角色与授权机制（F-CON-06）、账号生命周期（F-CON-02）、站内通知（F-CON-12）；CS-100
- AC：AC-OPS-02-1、AC-OPS-02-3、AC-OPS-02-4、AC-OPS-02-5、AC-OPS-02-6、AC-OPS-02-7、AC-OPS-02-9、AC-OPS-01-10、AC-OPS-01-11、AC-OPS-01-13
- 内容：
  1. 运维空间角色定义（F-OPS-02 规则 1）：`ops_viewer`、`ops_engineer`、`ops_auditor`、`ops_superadmin`，权限点 `ops.overview.read`、`ops.config.read`、`ops.troubleshoot`、`ops.silence.write`、`ops.audit.read`、`ops.account.manage`；`ops_engineer` 包含 `ops_viewer` 全部权限点。
  2. 互斥角色 `ops_superadmin`、`ops_auditor`（授予时 409 / 4093）；运维空间**不注册**应急授权接口（调用返回 403 / 4030，基座 F-CON-06 规则 6）。
  3. 生命周期约束（B-OPS-02）：只有 `ops_superadmin` 有 `ops.account.manage`；不能改自己角色、不能停用 / 重置自己、不能停用最后一个启用的 `ops_superadmin`（409 / 4093）；管理空间账号对运维账号 ID 调用账号接口 404 / 4040 并写 `space=admin, result=denied` 审计（基座 F-CON-02 规则 7，运维侧只做配置与验收）。
  4. 敏感角色（`ops_superadmin`、`ops_engineer`、`ops_auditor`）授予或变更时，通过基座站内通知发给全部 `ops_auditor`（B-OPS-14）。
  5. 参数化权限测试：读取 CS-110 的接口清单 YAML，4 个角色 + 无会话逐个调用（有权限 ≠ 403、无权限 403 / 4030、无会话 401）；未声明权限点的路由启动失败（基座机制，运维路由同样覆盖）。
- 估算：1 人日

### CS-102 运维侧对基座行为的验收：登录、锁定、会话、CSRF、重新验证、后台请求不续期、命令行引导
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：F-CON-02～05、07、08；CS-100、CS-101
- AC：AC-OPS-01-1、AC-OPS-01-2、AC-OPS-01-3、AC-OPS-01-4、AC-OPS-01-5、AC-OPS-01-7、AC-OPS-01-8、AC-OPS-01-9、AC-OPS-01-12、AC-OPS-01-15、AC-OPS-01-17、AC-OPS-01-18、AC-OPS-17-8
- 内容：
  1. 不新增实现，只在运维空间上写 e2e（用 [C 钟]）：路由清单中无未登录注册接口、`ops` 空间无账号时登录页提示；六种失败响应完全相同（401 / 4015）；锁定 429 / 4291（含不存在账号 `ghost`）与超管解锁审计；同 IP 21 次 4290；激活后未绑 TOTP 访问 overview 401 / 4016；空闲 30 分钟 / 绝对 12 小时；改密删除其他会话；CSRF 三种失败 403 / 4037 且不调节点、不写审计；step-up 403 / 4038、4 分钟内免验、6 分钟后再验；TOTP 同步长重放 4015；`account bootstrap --space ops` 写 `account.bootstrap` 审计且进入哈希链。
  2. 运维专属续期规则（B-OPS-05，基座 F-CON-05 规则 3 已含）的验收：自动刷新（`X-Console-Background: 1`）不续期；Grafana 代理只有 `Sec-Fetch-Mode: navigate` 续期（与 CS-117 联调）。
  3. 如基座任务的用例已在运维空间上参数化覆盖上述 AC，本任务只补缺口并在用例中引用基座用例编号。
- 估算：1.5 人日

### CS-103 运维审计：action 扩展、跨空间可见性、写失败即失败、中文名称
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：审计记录与查看（F-CON-08、F-CON-09）；CS-101
- AC：AC-OPS-14-1、AC-OPS-14-2、AC-OPS-14-3、AC-OPS-14-4、AC-OPS-14-5、AC-OPS-14-7、AC-OPS-15-1、AC-OPS-15-2、AC-OPS-15-3、AC-OPS-15-4、AC-OPS-15-5、AC-OPS-15-6、AC-OPS-15-7
- 内容：
  1. 扩展 `action` 枚举：`ops.troubleshoot.query`（额外字段：工单号、理由、结果条数、`partial`、查询节点列表）、`ops.config.view`、`ops.grafana.enter`；第二批 `ops.silence.create` / `ops.silence.expire`；P1 `ops.audit.export`。运维事件 `space=ops`。
  2. 跨空间可见性（B-OPS-10）：`ops_auditor` 可查 `space=ops` 与 `space=admin`；管理空间 `auditor` 以 `space=ops` 筛选返回 403 / 4030 并写 `result=denied`，不带空间条件时结果过滤掉 `space=ops`。
  3. 写失败即失败（X-4）在运维 action 上的接线：排查不调用节点、配置检查不返回数据（返回 5031）；总览等不写审计的页面不受影响。
  4. 运维 action 中文名称（"连接排查""查看配置检查""进入指标"等）随接口返回或由前端映射（与 FE-502 约定一处来源）。
  5. 参数化测试：第一批每类事件各触发一次，检查基座字段与额外字段；特征串密码 / TOTP 不进审计和日志；审计账号 `UPDATE` / `DELETE` 被拒；超级用户篡改 `reason` 后校验命令非 0 并指出断点；网段外 / 未登录各 1000 次不写审计、指标 +2000。
- 估算：1 人日

---

## 第一批：运维业务模块

### CS-110 运维模块骨架：接口清单 YAML、部分结果响应、只读路由守护
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：工程骨架、路由权限点声明机制
- AC：AC-OPS-02-1、AC-OPS-02-6、AC-OPS-05-4、AC-OPS-10-7、AC-OPS-13-3、AC-OPS-15-7
- 内容：
  1. `internal/ops` 模块；`api/ops-routes.yaml`：PRD 第 6 节接口清单的机器可读版本（方法、路径、权限点、是否写审计、是否重新验证、批次），启动时与实际注册路由比对，不一致启动失败；作为 AC-OPS-02-1 参数化测试的数据源。
  2. 统一响应包装 `{data, partial, missingNodes, dataTime}`；错误按基座 `{code, msg, reason?, details?}`。
  3. 路由守护测试：节点相关接口只有 GET（`troubleshoot/query`、`grafana/ticket`、第二批静默除外）；不存在任何与备份、恢复相关的路径；配置检查、审计接口对 PUT / POST / PATCH / DELETE 返回 405。
- 估算：1 人日

### CS-111 节点客户端：节点凭证表、Ed25519 签名、TLS 服务名校验、扇出与部分结果
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-110；BE-301（签名测试向量 `protocol/vectors/ops-sign.json`）；BE-314（测试证书）
- AC：AC-OPS-16-2、AC-OPS-16-5、AC-OPS-16-6、AC-OPS-08-11、AC-OPS-90-3
- 内容：
  1. 节点凭证表（配置文件）：`node_id`、地址、TLS 服务名、kid、私钥文件引用；私钥只从文件读取，不进数据库、不进日志、不出现在任何响应。支持同一节点两把 kid（轮换：切换签名私钥时不中断探测）。
  2. 签名客户端：复用 X-5 签名实现（F-KEY-03 8 行待签名串，`X-IM-Caller: console-ops`、`X-IM-Operator: ops:{accountId}`、`X-Request-Id`、`X-IM-Target-Node`），用 BE-301 的测试向量做一致性单测。
  3. TLS：只信任内部 CA，`ServerName` 取凭证表中的 TLS 服务名；校验失败时**不发送请求**，节点状态记为"未通过身份校验"。只向凭证表中的地址发请求（不信任任何自报地址）。
  4. 扇出：每实例并发上限 8、单节点超时 1.5 秒、对单节点最多 2 个并发；部分失败返回 `partial: true` + `missingNodes`。
- 估算：1.5 人日

### CS-112 节点探测与节点列表：5 秒探测、Postgres 共享状态、状态判定
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-111；BE-302
- AC：AC-OPS-04-1、AC-OPS-04-4、AC-OPS-04-6、AC-OPS-05-1、AC-OPS-05-2、AC-OPS-05-3、AC-OPS-05-5、AC-OPS-11-3
- 内容：
  1. 后台探测：每 5 秒调用各节点 `GET /internal/v1/node`（单节点超时 1.5 秒），结果和"最后可见时间"写 Postgres（多实例共享；用 advisory lock 或"最近一次探测时间"避免多实例重复探测放大）。
  2. 状态判定：`draining=true` → 下线中；`ready=false` 且 `draining=false` → 未就绪；最后一次成功距今 > 15 秒 → 不可达；TLS 校验失败 → 未通过身份校验。静态节点不可达时一直保留。
  3. `GET /console/api/ops/v1/nodes`：节点 ID、地址、版本、启动时间与运行时长、状态、连接数、最后可见时间、存活内存估算、堆 + 栈、goroutine、断开缓冲最早时间；只读探测缓存。
  4. 版本不一致：记录首次发现不一致的时间，持续 ≥ 30 分钟（[C 钟]）时给出提示。
- 估算：1 人日

### CS-113 总览：探测缓存 + Prometheus + Alertmanager + 备份卡片
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-112；BE-300、BE-311、BE-313
- AC：AC-OPS-04-2、AC-OPS-04-3、AC-OPS-04-5、AC-OPS-04-8、AC-OPS-04-9、AC-OPS-07-5、AC-OPS-12-3
- 内容：
  1. `GET /console/api/ops/v1/overview`：服务状态、版本、节点数、在线连接（按平台，来自探测缓存，不依赖 Prometheus）；消息量（1 分钟速率、今日累计按 Asia/Shanghai 自然日用 `increase()`）；错误率（5 分钟 5xxx / 4xxx 占比分开、Top 3 错误码，分母为 `im_commands_total` 全部命令、不含 auth 和心跳）；握手与断开（`phase="handshake"` 按关闭码、会话中 Top 3）；备份（最近一次成功距今，> 2 小时标红）。**不在用户请求时扇出到节点。**
  2. `GET /console/api/ops/v1/alerts`：Alertmanager API 只读（内网），当前触发告警（名称、级别、节点、开始时间）。
  3. 数据源不可用：对应卡片 `{unavailable: true, reason}`，绝不返回 0 或空列表；告警不可用时显示"告警数据不可用"。
  4. Prometheus 查询带超时（1 秒）和每实例并发上限，查询语句集中在一个文件并有单测。
- 估算：1 人日

### CS-114 连接排查：参数校验、频率与每日配额、扇出、结果判定、审计
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-111、CS-103；BE-302、BE-303
- AC：AC-OPS-08-1、AC-OPS-08-2、AC-OPS-08-3、AC-OPS-08-4、AC-OPS-08-5、AC-OPS-08-6、AC-OPS-08-7、AC-OPS-08-8、AC-OPS-08-9、AC-OPS-08-10、AC-OPS-08-11、AC-OPS-08-13、AC-OPS-09-1、AC-OPS-09-2、AC-OPS-09-3、AC-OPS-09-4、AC-OPS-09-5、AC-OPS-09-6、AC-OPS-09-7、AC-OPS-09-10、AC-OPS-09-11、AC-OPS-14-4、AC-OPS-02-5
- 内容：
  1. `POST /console/api/ops/v1/troubleshoot/query {userId, ticket, reason}`：userId 格式（PRD-core 3.1）不合法 4004；`ticket` 按 `CON_TICKET_PATTERN`、`reason` 4～200 码点且不能由同一字符重复组成（复用基座校验函数）不合法 4000；**校验失败、权限失败、配额失败、审计写失败都不调用节点**。
  2. 频率：每账号每分钟 30 次（4290）；每日配额：每账号每 Asia/Shanghai 自然日 50 个不同 userId（429 / 4292），重查已查过的不计数（配额状态存 Postgres，[C 钟]）。
  3. 扇出全部静态节点并发查询 `connections:query` + `disconnects:query`；合并断开记录按时间倒序取 50；从 Prometheus 读最近 1 小时 4401 / 4010 / 4009 汇总（只有数字）；返回数据范围说明所需字段（各节点缓冲最早时间）。
  4. 结果判定：全部节点响应且无连接 → "当前不在线"（不区分不存在）；有节点未响应 → `partial: true`，**不得返回"当前不在线"结论**。
  5. 一次查询一条 `ops.troubleshoot.query` 审计（先写审计再调用节点；写失败返回 5031 不调用）；节点请求带同一 `X-Request-Id`。console-server 访问日志不记录请求体（userId、工单号、理由不进日志）。
  6. 汇总提示所需的计数（各关闭码次数、服务端关闭占比、握手 4011 / 4012 占比）由接口计算返回。
- 估算：2 人日

### CS-115 配置检查：扇出、差异视图、风险提示纯函数、审计
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-111、CS-103；BE-304
- AC：AC-OPS-10-1、AC-OPS-10-2、AC-OPS-10-3、AC-OPS-10-4、AC-OPS-10-5、AC-OPS-10-6、AC-OPS-10-7
- 内容：
  1. `GET /console/api/ops/v1/config`：扇出 `/internal/v1/config`，按节点返回 7 组摘要与生产检查项（原样透传 im-server 的 ID、结论、实际检查内容，不在 console-server 手抄）；写 `ops.config.view` 审计（写失败则不返回数据）。
  2. 风险提示纯函数（F-OPS-10 规则 2，单测）：Origin 含 `*`；production 无可信代理；GOMEMLIMIT 未设置；GOMEMLIMIT < 规划连接数 × 32 KB × 1.5（规划连接数取 `IM_PLANNED_CONNS`，未配置时取 Prometheus 近 7 天该节点峰值并注明来源）；`IM_NODE_ID` 未配置；pprof 非回环；非 production 开测试模式。
  3. 差异视图：同一项在各节点取值不同时高亮（版本、节点 ID、地址类除外）。
  4. 密钥特征串扫描：响应、日志不含特征串及其 SHA-256 前 16 位。
- 估算：1 人日

### CS-116 数据库与备份记录（只读）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-111；BE-305、BE-313
- AC：AC-OPS-11-1、AC-OPS-11-2、AC-OPS-11-5、AC-OPS-12-1、AC-OPS-12-2、AC-OPS-12-3、AC-OPS-12-4、AC-OPS-12-5、AC-OPS-12-6、AC-OPS-12-7、AC-OPS-12-8、AC-OPS-13-4
- 内容：
  1. `GET /console/api/ops/v1/db`：透传节点 `/internal/v1/db`（迁移版本、执行时间、校验和、未执行列表、连通性）。
  2. `GET /console/api/ops/v1/backups`：用**只读凭证**（只能 List / Get `manifests/`）读取清单，按 `source` 分组取最近 48 次；最近一次成功时间与距今时长（> 2 小时标红，[C 钟]）；`trigger=pre-migrate` 标"发布前"并带发布版本；只输出文件名，不输出路径、桶外地址和凭证；没有下载、恢复、触发备份接口。
  3. 健康只看"最近一次成功"：清单缺失时自然退化为更早的那次。
  4. 测试：console-server 凭证对 `backups/` 读、任一前缀写、删除均 AccessDenied（与 BE-313 的对象存储策略联调）。
- 估算：1 人日

### CS-117 Grafana 认证代理：一次性票据、主机 Cookie 绑定会话、Viewer 映射、续期规则
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-101、CS-103；BE-310（Grafana `auth.proxy` 与 whitelist）；基座 CS 任务（console.md）：会话
- AC：AC-OPS-17-1、AC-OPS-17-2、AC-OPS-17-3、AC-OPS-17-4、AC-OPS-17-5、AC-OPS-17-8、AC-OPS-14-1
- 内容：
  1. `POST /console/api/ops/v1/grafana/ticket`：校验会话和 `ops.overview.read`（`ops_auditor`、`ops_superadmin` 403 / 4030，不签发）；签发 30 秒有效、只能用一次的票据（Postgres 原子消费），写 `ops.grafana.enter` 审计；返回 `grafana.ops.<内网域名>` 跳转地址。
  2. Grafana 主机上的代理（同一 console-server 进程按 Host 分流）：用票据换主机专属 Cookie（`__Host-` 前缀、`SameSite=Strict`），Cookie 记录原运维会话 ID；每个请求校验原会话仍有效（退出、过期、改密、改角色后立即 401 并跳回运维后台登录页）。
  3. 向 Grafana 传 `X-WEBAUTH-USER`（运维账号名），**剥离客户端自带的同名头**；所有用户映射为 Viewer；拦截保存面板、数据源管理等写接口（Grafana 侧 Viewer 本身也会拒绝，代理侧双重保护）。
  4. 续期（B-OPS-05）：只有 `Sec-Fetch-Mode: navigate` 的请求续期原会话空闲计时，数据查询（`/api/ds/query` 等）不续期。
- 估算：1.5 人日

---

## 第一批：性能与对账验收

### CS-120 运维后台自身性能与节点对账验收
- 优先级：P0
- 批次：第一批
- 仓库：tianting-console-server
- 依赖：CS-113、CS-114、CS-117；BE-307、BE-320
- AC：AC-OPS-90-2、AC-OPS-90-3、AC-OPS-14-6
- 内容：
  1. EP：AC-OPS-90-1 的"开"段中统计 console-server 接口耗时（每类 ≥ 300 样本）：总览 P99 ≤ 1 秒、单节点排查 P99 ≤ 500ms；Grafana 24 小时面板首次加载 5 次中位数 ≤ 3 秒（与 BE-321 同轮）。
  2. E2a：双节点用户 300 次排查（分散到多个 `ops_engineer` 测试账号，避开每分钟 30 次和每日 50 个 userId 上限）P99 ≤ 800ms。
  3. E2a：20 次排查后运行 `deploy/ops/reconcile`，差异 0 条。
- 估算：1 人日

---

## 第二批 / P1

### CS-130 告警静默：经 console-server 创建 / 提前结束，重新验证与审计
- 优先级：P1
- 批次：第二批
- 仓库：tianting-console-server
- 依赖：CS-113、CS-103；基座 CS 任务（console.md）：重新验证
- AC：AC-OPS-18-1、AC-OPS-18-2、AC-OPS-18-3、AC-OPS-18-4
- 内容：
  1. `POST /console/api/ops/v1/silences`、`POST /silences/{id}:expire`：`ops.silence.write`（仅 `ops_engineer`），需要重新验证（403 / 4038），必填 `ticket`、`reason`（规则同排查）。
  2. 匹配条件只允许 `alertname`、`node_id` 的等值匹配（禁止正则、禁止空匹配），时长 15 分钟～24 小时，否则 4000，不调用 Alertmanager。
  3. 调用 Alertmanager API：`createdBy` = 运维账号名，`comment` = 工单号；写 `ops.silence.create` / `ops.silence.expire` 审计；`GET /alerts` 返回当前生效的静默（匹配条件、创建人、到期时间）。
- 估算：1.5 人日

### CS-131 双人批准授予排障角色与异常查询通知
- 优先级：P1
- 批次：第二批
- 仓库：tianting-console-server
- 依赖：CS-101、CS-114；基座 CS 任务（console.md）：站内通知
- AC：AC-OPS-02-8、AC-OPS-08-14
- 内容：
  1. 授予 `ops_engineer` 进入"待批准"状态，需另一名 `ops_superadmin` 批准后生效；本人批准 403 / 4030；三步写审计。
  2. 同一账号 1 小时内查询 ≥ 20 个不同 userId 时向全部 `ops_auditor` 发站内通知（含账号名和数量，不含 userId 列表），同一小时窗口只发一次。
- 估算：1.5 人日

### CS-133 双人批准扩大到超管、拒绝、过期（P-22）
- 优先级：P1
- 批次：第二批
- 仓库：tianting-console-server
- 依赖：CS-131
- AC：AC-OPS-01-19、AC-OPS-01-20、AC-OPS-02-10、AC-OPS-02-11、AC-OPS-02-12
- 内容：
  1. 授予 `ops_superadmin` 也进入待批准；新建账号在生效前只有"只需登录"的权限。
  2. 拒绝：任一启用的 `ops_superadmin`，重新验证，理由必填，写 `ops.role.reject`；发起人拒绝等于撤回。
  3. 过期：7 天（[C 钟]）后自动失效，写 `ops.role.expire`，之后批准或拒绝 409 / 4093；批准时重新检查互斥角色和账号状态。
  4. 命令行：`account bootstrap --space ops` 在启用的超管（含待激活）少于 2 个时可执行，写审计并通知全部 `ops_superadmin`、`ops_auditor`；admin 空间不变。
  5. 同步 `tools/ops-accept` 和测试辅助函数的超管创建方式。
- 估算：1 人日

### CS-132 审计导出（运维侧）
- 优先级：P1
- 批次：P1
- 仓库：tianting-console-server
- 依赖：基座 CS 任务（console.md）：审计导出（F-CON-09 规则 7）
- AC：AC-OPS-15-8
- 内容：在运维空间启用基座导出：需要重新验证、1 万条以内、写 `ops.audit.export`（筛选条件、条数）；验收 AC-OPS-15-8。
- 估算：0.5 人日（实现在基座）

---

## B6：集群相关（不计入第一批验收）

### CS-140 节点注册表发现："未登记"节点与过期清理
- 优先级：P1
- 批次：B6
- 仓库：tianting-console-server
- 依赖：B6 的节点注册表（backend.md BE-070～075）；CS-112
- AC：AC-OPS-05-6、AC-OPS-05-7
- 内容：读取注册表只用于发现；不在凭证表中的节点显示"未登记"，**不发送任何请求**（包括探测）；注册表过期且不在凭证表中的节点 10 分钟后从列表消失；静态节点不受影响。
- 估算：1 人日

### CS-141 排查走 Presence 路由与退化
- 优先级：P1
- 批次：B6
- 仓库：tianting-console-server
- 依赖：B6 的 Presence（Redis）；CS-114
- AC：AC-OPS-08-12
- 内容：先查 Presence 得到用户所在节点，只请求这些节点；Presence 不可用时退化为全部节点并返回提示"在线路由不可用，已查询全部节点"。
- 估算：1 人日

### CS-142 Postgres 数据库位置与库大小展示
- 优先级：P1
- 批次：B6
- 仓库：tianting-console-server
- 依赖：BE-340
- AC：AC-OPS-10-10、AC-OPS-11-6
- 内容：配置检查、数据库页透传 BE-340 的脱敏位置（主机、库名）与第二批的库大小、行数估算；console-server 日志同样不含连接串用户名和密码。
- 估算：0.5 人日
