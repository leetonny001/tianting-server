# 后端任务单：运维后台的 im-server 改造与部署配置（PRD-ops v1.0）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-ops v1.0](../prd/PRD-ops-v1.0.md)（冻结，167 条 AC）第 15 节 S-OPS-01～30、第 7 节；[PRD-admin v1.0](../prd/PRD-admin-v1.0.md) F-KEY-01（独立内网监听）、F-KEY-03（签名）；[ARCH-v1](../arch/ARCH-v1.md) 第 2、7 节；[压测报告](../reports/perf-2026-11-03.md)；[内存分析](../reports/mem-2026-11-03.md)；[错误码](../../protocol/ERRORS.md) |
| 负责 | 后端（AI） |
| 仓库 | `tianting-server`（本仓库 `server/`，部署配置在 `deploy/`，目前尚无 `deploy/` 目录，由本任务单新建） |
| 起点 | B5 已完成：`internal/metrics`（Prometheus 指标）、`IM_METRICS_ADDR` 上挂 `/metrics` + pprof、`conn closed` 日志、`internal/config` 的 `productionProblems`（返回 `[]string`，`IM_JWT_KEYS` 下漏查与 `IM_ADMIN_KEY` 相同）、`migrate status`（无执行时间和校验和）、`im-server backup`（无清单）、`im-loadtest` |
| 环境依赖 | 不在本机安装任何软件。E1 / E2a / EP 所需的 Prometheus、Alertmanager、Grafana、node_exporter、MinIO、Postgres（console-server 用）、`promtool`、`amtool`、`age` 由测试环境提供（容器或测试机），本任务单只交付配置文件、脚本和用例；CI 中 `promtool test rules` 用官方镜像运行 |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（Go 单测 / e2e、promtool、部署 e2e）或验证记录；`go test -race ./...` 全过；新增指标通过基数测试（AC-OPS-90-4）；协议 / 错误码 / ARCH 第 7 节实现记录同步更新；用例编号见 [TC-OPS](../testcases/TC-OPS.md) |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。同一条 AC 可以由 im-server（本文件）和 console-server（[console-ops.md](console-ops.md)）、前端（[frontend-ops.md](frontend-ops.md)）的任务共同负责。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-300 ～ BE-306 im-server 改造 | ✅ 2026-11-04 | 指标（`phase`、`origin_rejected`、`im_build_info`、`im_ready`、`im_draining`、Go 运行时、`im_ops_*`、基数检查）；`internal/opsapi` 5 个只读接口（来源网段 → 并发闸门 → Ed25519 签名 → 2 秒超时）；`internal/disconnects` 环形缓冲；`productionChecks` 启动保护与配置摘要共用；`MigrationReport` CLI 与接口共用、只读连接；pprof 移到 `IM_PPROF_ADDR`；`backup -manifest`；向量 `protocol/vectors/ops-sign.json`；e2e `TestOPS*` |
| BE-307 压测与对账工具 | 🟡 工具完成，验收待 console-server | `im-loadtest -scenarios opsimpact`、`-metrics-addr` / `-ops-addr` / `-pprof-addr`；`deploy/ops/reconcile`、`deploy/ops/scan-logs`（`deploy/ops/test_tools.py`）。**AC-OPS-90-1、14-6、08-13 还没有验收**：要 1 万连接 + console-server 驱动（BE-321）。目前只做过 100 连接、每段 15 秒的冒烟，运维流量直接打节点接口 |
| BE-310 ～ BE-314 部署配置 | 🟡 配置完成，部分验收待测试环境 | `deploy/observability`、`deploy/backup`、`deploy/release.sh`、`deploy/pki`。本机：`deploy/test_deploy.py` 27 个测试（假的 age、mc）；告警和面板用到的指标对真实 `/metrics` 和指标定义核对过。CI 的 `ops-deploy` 用真实工具：`promtool test rules`（AC-OPS-07-1）、`amtool`、MinIO 策略和真实 age（备份、解密、完整性、加密失败、AC-OPS-12-5）、Grafana 导入 14 个面板、Viewer 不能保存和建数据源、不能匿名。MinIO 不再发布公开镜像和二进制，CI 从固定版本的源码构建。**没有验证**：通知真的发到 webhook（07-2～07-6）、面板数据和读数（06-2～06-10）、防火墙规则（17-1、17-5、17-6、17-9，nftables 没有实际加载过）、console-server 对真实 Grafana 的代理、`backup-console.sh` 对真实 Postgres（只有语法检查）、`release.sh` 对真实新迁移（用的假新二进制）。告警接收人和地址是占位，等 P-7 的 F-1 |
| BE-320 测试环境编排 | 🟡 不含 console-server 的部分已真实跑通，console-server 部分待 P-9 | `deploy/testenv/`：`compose.yml`（E1）、`compose.e2a.yml`（E2a）、`tt`（时钟、暂停、kill、备份时间、停观测组件）、`webhook.py`、`verify.sh`。CI 工作流 `testenv`（run 41815506272，容器全真实）：AC-OPS-07-2（通知 101 秒，恢复 41 秒）、07-3（118 秒，恢复 59 秒；全程没有 console-server）、07-6（6 条通知都有 node_id，正则 0 命中）、90-5（Grafana、Prometheus 停止时打 n1：P50 1.6ms，P99 4.5ms）；04-6、08-11 只验了节点侧（两个版本号、暂停的 n2 不响应且 n1 不受影响）。console-server 镜像和 profile `console` 只做过 `compose config` 校验；生成的 `console.env`、`n1.env` 在本机用真实二进制和 Postgres 跑过（迁移、监听、签名探活、set-clocks）。**未验证**：console 的容器构建和联调、邮件 receiver、AC-OPS-07-4（要跑 2 小时 5 分钟）、07-5。验收时查出并修了三个配置问题：`group_interval` 5m 会让恢复通知超过 3 分钟（改 1m）；webhook 原始标签带 `instance`（`alert_relabel_configs` 去掉，生产还要配 `--web.external-url`）；汇总类告警没有 `node_id`（标 `all`）。 |
| BE-321 EP 性能验收 | ⬜ 未开始 | |
| BE-340 B6 部分 | ⏸ 随 B6 | Postgres 位置脱敏、库大小估算 |

## 总览

| 阶段 | 任务 | 人日 | 对照 PRD-ops 10.2 节 |
|---|---|---|---|
| 第一批：im-server 改造（S-OPS-01～26） | BE-300 ～ BE-307 | 10 | tianting-server 改造 8～10（第 15 节逐项合计 9.7） |
| 第一批：部署配置（S-OPS-27～30） | BE-310 ～ BE-314 | 6 | 部署 4～6 |
| 第一批：测试环境与 EP 验收 | BE-320 ～ BE-321 | 2.5 | 测试 3～5 的一部分（其余在 console-ops CS-120、frontend-ops FE-507） |
| B6：集群相关 | BE-340 | 1（随 B6 计入，不在 28～40 内） | 依赖 B6 的 AC 随 B6 计入 |
| **第一批合计** | 15 个任务 | **18.5** | |

全部运维工作量对照（三份任务单合计，第一批）：

| PRD-ops 10.2 分项 | PRD 估算 | 任务单 | 人日 |
|---|---|---|---|
| tianting-server 改造 | 8～10 | BE-300～307 | 10 |
| console-server 运维模块 | 6～9 | CS-110～117 | 10 |
| 公共基座中运维侧的增量 | 2～3 | CS-100、CS-101、CS-103 | 3 |
| 前端 tianting-ops | 5～7 | FE-500～506 | 7 |
| 部署 | 4～6 | BE-310～314 | 6 |
| 测试 | 3～5 | BE-320、BE-321、CS-102、CS-120、FE-507 | 6 |
| **合计** | **28～40** | | **42** |

> 超出上限约 2 人日，主要来自 console-server 运维模块（扇出框架、Grafana 认证代理各按 1.5 人日计）和把"运维侧对基座的验收测试"（CS-102）单列。若基座任务（console.md）已经覆盖 CS-102 的验收用例，或 X-5 签名客户端可直接复用，则回到 40 以内。第二批（CS-130、CS-131、FE-520）约 4.5 人日、P1（CS-132、FE-521）约 1.5 人日，B6（BE-340、CS-140～142）约 3.5 人日，均不在上表内。

依赖顺序：BE-300 → BE-303（断开指标）；X-5 签名库 + F-KEY-01 → BE-301 → BE-302 / BE-303 / BE-304 / BE-305（运维接口挂在 BE-301 的监听上）；BE-306 → BE-313 → BE-314；BE-300 → BE-310 / BE-311 / BE-312；全部 → BE-320 → BE-321。console-server 的节点客户端（CS-111）可以在 BE-301 的签名测试向量确定后并行开发。

---

## 第一批：im-server 改造

### BE-300 指标扩展：握手阶段、构建信息、就绪 / 下线、Go 运行时、运维接口指标
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：无
- AC：AC-OPS-04-8、AC-OPS-06-6、AC-OPS-06-9、AC-OPS-06-10、AC-OPS-09-5、AC-OPS-90-4
- 内容：
  1. S-OPS-01：`im_disconnects_total` 增加 `phase`（`handshake` / `session`）标签；握手阶段 4009、4401、4010、4011、4012、1011 全部计入（`by=server`）。在 `internal/gateway` 的握手拒绝路径统一调用一个 `recordHandshakeReject(code, reason)`。现有面板、告警改为 `sum without (phase)` 兼容。
  2. S-OPS-02：Origin 不在白名单时 `websocket.Accept` 失败的路径计 `im_handshakes_total{result="origin_rejected"}`。
  3. S-OPS-03 / 04：新增 `im_build_info{version, go_version, node_id} 1`、`im_ready`、`im_draining`（与 `/readyz`、优雅下线状态机同源，不另算）。
  4. S-OPS-05：`collectors.NewGoCollector` 开启 runtime/metrics 规则，确认导出 `go_gc_gomemlimit_bytes`、`go_gc_heap_live_bytes`、`go_cpu_classes_gc_total_cpu_seconds_total`、`go_cpu_classes_total_cpu_seconds_total`；client_golang 版本不支持的用自定义 gauge（读 `runtime/metrics`）。
  5. S-OPS-06：`im_ops_requests_total{endpoint, result}`（ok / unauthorized / forbidden_cidr / throttled / error）、`im_ops_inflight`。
  6. 基数测试（AC-OPS-90-4）：遍历注册表中全部 `im_*` 指标，断言没有 userId / uid / conv / conn 标签，`code`、`reason`、`cmd`、`phase`、`result`、`endpoint` 的取值在枚举内（未知值归 `unknown`）；变异测试：故意把 userId 放进标签，测试必须失败。
- 估算：1 人日（S-OPS-01～06 合计 1.4，标签归一已有基础）

### BE-301 运维监听：`IM_OPS_ADDR`、签名校验、TLS、来源网段、作用域隔离、访问日志
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：PRD-admin X-5 的签名库（F-KEY-03，Ed25519 + 8 行待签名串）与 `IM_ADMIN_ADDR`（F-KEY-01），由管理后台对应的 BE 任务交付；BE-300
- AC：AC-OPS-16-1、AC-OPS-16-2、AC-OPS-16-3、AC-OPS-16-6、AC-OPS-16-7、AC-OPS-16-8、AC-OPS-16-9、AC-OPS-08-7、AC-OPS-14-6
- 内容：
  1. S-OPS-11：配置 `IM_OPS_ADDR`（默认 `127.0.0.1:9091`，`off` 关闭）、`IM_OPS_TLS_CERT` / `IM_OPS_TLS_KEY`、`IM_OPS_ALLOWED_CIDRS`、`IM_OPS_PUBKEYS=kid:base64`（最多 2 把）。独立 `http.Server` + 独立 `ServeMux`，只注册 `/internal/v1/*`。
  2. 中间件顺序：来源网段（不在 `IM_OPS_ALLOWED_CIDRS` → 403 / 4030，不进入签名校验）→ 并发闸门（同时最多 4 个，超出 429 / 4290）→ 签名校验（复用 X-5 签名库：调用方必须是 `console-ops`；第 8 行 `X-IM-Target-Node` 必填且等于 `IM_NODE_ID`；时间窗 300 秒用 [IM 钟]；nonce 10 分钟本地去重，只登记验签通过的；失败一律 401 / 4017，不返回数据）→ 处理超时 2 秒。
  3. S-OPS-12：作用域隔离——`console-ops` 不在 `IM_ADMIN_CALLERS` 中，不能调用 `/admin/*`；管理接口调用方不能调用 `/internal/v1/*`；`IM_ADDR` 上不挂任何运维路径（404）。
  4. S-OPS-10：访问日志（独立 logger）字段：时间、密钥 ID、`X-IM-Operator`、`X-Request-Id`、接口、结果条数、耗时、调用方 IP；**不写 userId**；签名失败写原因枚举（缺少签名 / 时间戳过期 / nonce 重复 / 目标节点不符 / 签名错误 / 来源网段不允许），不写密钥和签名值。
  5. 测试：签名测试向量（与 console-server CS-111 共用 `protocol/vectors/ops-sign.json`）；5 种失败 + 篡改 body + 跨节点重放 + 密钥轮换（k1 → k2，去掉 k1 后 k1 失效）+ 来源网段 + 并发闸门（配合 BE-305 的延迟注入）。
- 估算：2 人日（S-OPS-10、11、12 合计 2.0；复用签名库）

### BE-302 节点信息与当前连接接口：`GET /internal/v1/node`、`POST /internal/v1/connections:query`
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-301
- AC：AC-OPS-16-10、AC-OPS-05-1、AC-OPS-05-3、AC-OPS-04-1、AC-OPS-04-4、AC-OPS-08-1、AC-OPS-08-2、AC-OPS-08-6、AC-OPS-08-10
- 内容：
  1. S-OPS-13：`/internal/v1/node` 返回节点 ID、版本、启动时间、`ready`、`draining`、按平台连接数（hub 维护原子计数，O(1)）、存活内存估算（`/gc/heap/live:bytes` + 栈）、堆、栈、GOMEMLIMIT、goroutine 数、断开缓冲最早时间（BE-303）。**只用 `runtime/metrics`**，不调用 `runtime.ReadMemStats`；基准测试断言 `/sched/pauses/total/other:seconds` 增量为 0。
  2. S-OPS-14：连接对象新增建立时间、**脱敏后的** IP（IPv4 前 2 段、IPv6 前 3 组，脱敏函数放 `internal/gateway`，与 PRD-admin F-ADM-14 共用）、发送队列积压状态只读方法；hub 按 userId 索引查询，不遍历、不持全局锁。
  3. 查询函数 `hub.ConnectionsOf(userId)` 与管理后台在线设备接口（PRD-admin G-ADM-06）共用。用户不存在与不在线返回同样的空列表。
  4. 响应不含 token、订阅会话、消息内容（单测断言字段白名单）。
- 估算：1 人日（S-OPS-13、14 合计 1.1）

### BE-303 断开记录：环形缓冲、握手阶段记录、结构化日志
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-300、BE-301
- AC：AC-OPS-09-1、AC-OPS-09-2、AC-OPS-09-3、AC-OPS-09-4、AC-OPS-09-5、AC-OPS-09-6、AC-OPS-09-7、AC-OPS-09-8、AC-OPS-09-9、AC-OPS-09-10、AC-OPS-09-11、AC-OPS-09-12、AC-OPS-06-3、AC-OPS-06-4
- 内容：
  1. S-OPS-15：`internal/hub/disconnects`（或独立包）环形缓冲：每用户最多 50 条、全节点最多 2 万条（超出淘汰全节点最旧一条，同时从该用户桶移除）；只存 userId、连接 ID、阶段、平台、关闭码、原因、关闭方、时间、时长、脱敏 IP；读时复制、写入 O(1)、不长时间持锁。`POST /internal/v1/disconnects:query {userId, limit≤50}` 返回记录 + 缓冲最早时间。
  2. 写入点：会话中断开（`conn closed` 处）；握手阶段只写签名有效的 4011、4012、1011（userId 取自已验签 claims）；4010、4401、4009 只计指标、写日志，不写缓冲。
  3. S-OPS-07 / 08 / 09：`conn rejected` 日志统一字段 `code`、`phase=handshake`、`reason`、`platform`、脱敏 IP、`uid`（仅 4011 / 4012 / 1011）；`auth timeout` / `no auth frame` 补 `code=4401`；`conn closed` 加 `phase=session` 和脱敏 IP；握手限流日志的完整 IP 改为脱敏。4010 的日志不写 token 中自称的 userId。
  4. 测试：10 万条 / 1 万用户后强制 GC 比较存活堆（与 2 万条时相差 ≤ 5%）；u2 5 条 + u1 1000 条；1 万条 / 秒写 + 每秒 10 次读的基准（写 P99 ≤ 1ms）；日志扫描测试（4010 不含自称 userId）。
- 估算：1.5 人日（S-OPS-07～09、15 合计 1.4）

### BE-304 配置摘要接口与生产启动保护：`GET /internal/v1/config`、`productionProblems` 重构、漏洞修复
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-301
- AC：AC-OPS-10-1、AC-OPS-10-4、AC-OPS-10-5、AC-OPS-10-8、AC-OPS-10-9、AC-OPS-16-4、AC-OPS-03-3
- 内容：
  1. S-OPS-16：`internal/config` 生成结构化脱敏摘要（PRD F-OPS-10 规则 1 的 7 组）；密钥类只输出 kid、数量、"已配置 / 长度达标"，**不输出值和哈希**；数据库位置 SQLite 输出文件路径。`productionProblems` 改为返回 `[]Check{ID, Passed, Applicable, Detail}`（稳定 ID，Detail 为"实际检查了什么"，如"IM_JWT_KEYS：2 把，最短 48 字节，均不同于管理密钥"），启动保护与 `/internal/v1/config` 共用；单测断言两边 ID 集合相同（AC-OPS-10-9）。
  2. S-OPS-18（漏洞修复）：使用 `IM_JWT_KEYS` 时逐把与 `IM_ADMIN_KEY` 比较，错误信息只报 kid；`IM_OPS_PUBKEYS` 每项必须是合法 Ed25519 公钥。
  3. S-OPS-19：production 新增检查——`IM_NODE_ID` 必须配置；`IM_OPS_ADDR` 非回环时必须 TLS 和 `IM_OPS_ALLOWED_CIDRS`；`IM_OPS_ADDR` 未关闭时 `IM_OPS_PUBKEYS` 必须已配置且合法；`IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的两两不同；`IM_PPROF_ADDR` 若设置必须为回环。一次列出全部问题。
  4. S-OPS-21：`IM_PLANNED_CONNS`（可选整数）。
  5. 密钥泄露扫描测试：特征串密钥写入配置，断言 `/internal/v1/config` 响应和日志都不含特征串及其 SHA-256 前 16 位十六进制。
  6. AC-OPS-03-3 是 console-server 的启动保护，本任务只提供"一次列出全部问题"的同款报错格式供 CS-100 对齐（列在此处是为了让两边的错误格式一致，主责在 CS-100）。
- 估算：1.5 人日（S-OPS-16、18、19、21 合计 1.3）

### BE-305 数据库接口与迁移状态：`GET /internal/v1/db`、`migrate status` 共用函数、测试模式故障注入
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-301
- AC：AC-OPS-11-1、AC-OPS-11-2、AC-OPS-11-3、AC-OPS-11-4、AC-OPS-11-5
- 内容：
  1. S-OPS-22：`store` 增加 `MigrationStatus(ctx) []MigrationRow{Version, AppliedAt, ChecksumOK, Pending}`；`im-server migrate status` 输出版本、执行时间、校验和是否一致、未执行列表；`/internal/v1/db` 调用**同一个函数**。
  2. S-OPS-17：`/internal/v1/db` 使用单独的只读连接（SQLite `file:...?mode=ro`），不与消息写入共用连接；返回类型、脱敏位置、连通性、迁移状态。
  3. S-OPS-24：`IM_TEST_MODE` 下增加故障注入 `store.Ping` 失败（`/readyz` 503、`/internal/v1/node` 的 `ready=false, draining=false`）、运维接口人为延迟（供 AC-OPS-16-8）；非测试模式路由 404。
  4. 单测：只读连接执行 INSERT 返回 `SQLITE_READONLY`，主连接写入同时成功；篡改 `0001` 校验和后接口与 CLI 都标"校验和不一致"。
- 估算：1 人日（S-OPS-17、22、24 合计 0.8）

### BE-306 pprof 分离、备份清单输出、只读路由守护
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-304
- AC：AC-OPS-17-7、AC-OPS-13-3、AC-OPS-05-4、AC-OPS-12-1
- 内容：
  1. S-OPS-20：pprof 从 `IM_METRICS_ADDR` 移到 `IM_PPROF_ADDR`（开发默认 `127.0.0.1:6060`，生产默认关闭；设置时必须是回环地址，否则拒绝启动，检查项在 BE-304）；指标端口只保留 `/metrics`；更新 `cmd/im-server/main.go` 顶部注释与 ARCH 第 7 节实现记录（X-09）。
  2. S-OPS-23：`im-server backup -manifest FILE` 输出清单 JSON：`source`、`trigger`（由参数传入）、开始 / 结束时间、文件名（不含目录）、大小、SHA-256、是否通过校验、迁移版本、主机、结果、失败原因摘要（不含路径和凭证）。加密、上传、textfile 指标由 BE-313 的脚本完成。
  3. 路由守护测试：枚举 `IM_ADDR`、`IM_ADMIN_ADDR`、`IM_METRICS_ADDR`、`IM_OPS_ADDR`、`IM_PPROF_ADDR` 全部注册路由——运维端口只有 GET 和两个只读 POST 查询（`connections:query`、`disconnects:query`），没有改变节点状态的路径；任何端口都不存在含 backup / restore 的路径（AC-OPS-13-3 的 im-server 半边，console-server 半边见 CS-110）。
- 估算：1 人日（S-OPS-20、23 合计 0.6 + 路由守护测试）

### BE-307 压测工具 `opsimpact` 场景与运维对账脚本
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-301、BE-306
- AC：AC-OPS-90-1、AC-OPS-90-2、AC-OPS-14-6、AC-OPS-08-13
- 内容：
  1. S-OPS-25：`im-loadtest` 新增 `-metrics-addr`、`-ops-addr`、`-pprof-addr` 固定端口参数；堆快照改从 `IM_PPROF_ADDR` 采集；新增场景 `opsimpact`：在同一轮压测中按"关 / 开 / 关 / 开 / 关 / 开"交替，每段 5 分钟，"开"段由外部运维流量驱动器（读取 console-server 地址和 20 个测试账号）施加负载；输出每段观测连接端到端 P99、吞吐、意外断开数，以及 3 个开段 / 3 个关段 P99 的中位数和差值，结论按 AC-OPS-90-1 判定（中位数差 ≤ 20ms、每段 P99 ≤ 1 秒、断开 0、吞吐 ≥ 490 条/秒）。
  2. S-OPS-26：`deploy/ops/reconcile`：输入 console-server 审计导出（或审计库只读查询结果）与各节点运维访问日志，按请求 ID 双向比对（审计 → 日志、日志中用户级接口 → 审计），核对结果条数，输出差异列表，有差异时非 0 退出。
  3. 日志特征串扫描工具（`deploy/ops/scan-logs`）：给定特征串，扫描 console-server 访问日志、im-server 全部日志、前置代理访问日志，供 AC-OPS-08-13、AC-OPS-10-1、AC-OPS-14-2 复用。
- 估算：1 人日（S-OPS-25、26 合计 1.1）

---

## 第一批：部署配置（`deploy/`）

### BE-310 Prometheus 抓取配置、节点清单生成、防火墙规则、Grafana 访问配置
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-300
- AC：AC-OPS-06-8、AC-OPS-17-1、AC-OPS-17-5、AC-OPS-17-6、AC-OPS-17-9
- 内容：
  1. S-OPS-27（抓取部分）：`deploy/observability/prometheus/prometheus.yml`，`job="im"` 用 file_sd；`deploy/observability/gen-targets`：由 console-server 的节点凭证表（只读其中 node_id、指标地址，不读私钥引用）生成 file_sd 清单，打 `node_id` 标签，与 `im_build_info{node_id}` 一致；抓取间隔 15 秒；保留 30 天。
  2. 防火墙规则（nftables 片段 + 说明）：`IM_METRICS_ADDR` 只对 Prometheus 主机开放；Prometheus（9090）、Alertmanager（9093）只监听回环或只对 console-server、Grafana 主机开放；Grafana 只对 console-server 开放。
  3. Grafana 访问配置（`grafana.ini` provisioning）：`auth.proxy` 启用、`whitelist` 只含 console-server 地址、`auto_sign_up` 映射为 Viewer、`disable_login_form`、匿名关闭、`allow_embedding=false`；任何代理用户不能获得 Editor / Admin。
  4. 部署 e2e：从 VPN 测试机直连 Grafana / Prometheus / Alertmanager / 指标端口被拒；伪造 `X-WEBAUTH-USER` 被拒；console-server 主机可读。
- 估算：1 人日

### BE-311 告警规则、promtool 用例与 Alertmanager 配置
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-300
- AC：AC-OPS-07-1、AC-OPS-07-2、AC-OPS-07-3、AC-OPS-07-4、AC-OPS-07-5、AC-OPS-07-6、AC-OPS-07-7、AC-OPS-07-8、AC-OPS-07-9
- 内容：
  1. `deploy/observability/prometheus/alerts.yml`：PRD F-OPS-07 规则 1 的 11 条（无就绪节点用 `or vector(0)`；5xxx 占比 > 1% 且 ≥ 10 条；耗时告警读 `le="0.5"` 桶 + send 速率 > 1；连接骤降带 `im_draining`、`changes(process_start_time_seconds[10m])` 抑制；内存告警用存活内存估算 > GOMEMLIMIT × 0.66 且 GOMEMLIMIT < 2^62；GC 风暴用 GC CPU 占比 > 25%；备份过期含 `absent`）。
  2. `alerts_test.yml`：每条规则覆盖触发 / 不触发 / 边界（AC-OPS-07-1 列出的全部边界）；CI 用 `promtool test rules`（官方镜像，不在开发机安装）。
  3. `deploy/observability/alertmanager/alertmanager.yml`：办公 IM 机器人 + 邮件 receiver（接收人按 F-1 默认值：Frank）；`repeat_interval: 1h`；inhibit：`im_draining == 1` 时抑制该节点除"节点不可达"外的警告级告警；模板输出告警名、级别、`node_id`、当前值、阈值、开始时间、运维后台链接，**不引用 `instance`**，恢复时发"已恢复"。
  4. `deploy/observability/check-alertmanager`：列出全部 receiver 目标地址，断言不指向天听自身服务地址、模板不引用 `instance`（AC-OPS-07-9）；通知内容正则扫描（userId、IPv4 / IPv6、`IP:端口`、JWT）。
  5. 静默第一批只在 Alertmanager 主机用 `amtool`，`comment` 必须写工单号（写进运维手册）。
- 估算：1.5 人日

### BE-312 Grafana 面板：14 个面板 JSON 与只读导入
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-300、BE-310
- AC：AC-OPS-06-1、AC-OPS-06-2、AC-OPS-06-3、AC-OPS-06-4、AC-OPS-06-5、AC-OPS-06-6、AC-OPS-06-7、AC-OPS-06-8、AC-OPS-06-9、AC-OPS-06-10
- 内容：
  1. `deploy/observability/grafana/dashboards/*.json`：PRD F-OPS-06 规则 1 的 14 个面板，`node_id` 变量（默认全部）；参考线（500 条/秒、1 秒、32 KB、25%）与说明文字；关闭码面板按 `phase` 分组、组内按关闭方；P99 超过 2.5 秒用值映射显示"> 2.5s"；GOMEMLIMIT ≥ 2^62 时不画线并显示"未设置 GOMEMLIMIT"；每连接存活内存只在连接数 ≥ 1000 时显示；数据保留说明文本面板。
  2. provisioning：`allowUiUpdates: false`、`disableDeletion: true`，部署流程自动导入。
  3. `deploy/observability/grafana/normalize.jq` + 比对脚本（AC-OPS-06-1）：导出面板 JSON 删除 `id`、`version`、`iteration` 等运行时字段后与仓库内容比对。
- 估算：1.5 人日

### BE-313 定时备份任务：快照、校验、加密、上传、清单、textfile 指标
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-306
- AC：AC-OPS-12-1、AC-OPS-12-2、AC-OPS-12-4、AC-OPS-12-5、AC-OPS-12-7、AC-OPS-12-8
- 内容：
  1. S-OPS-28：`deploy/backup/backup.sh`（systemd timer 每小时，保留 48 份）：`im-server backup -manifest` → `age` 加密（公钥来自配置，ARCH S10）→ 上传 `backups/` → 写清单到 `manifests/`（补充 `encrypted`、`uploaded`、`trigger`、发布版本字段）→ 成功后在同一步写 node_exporter textfile：`im_backup_last_success_timestamp_seconds{source,trigger}`、`im_backup_last_attempt_timestamp_seconds{source}`、`im_backup_last_size_bytes{source}`。任一步失败：写 `result=failure` 清单（失败原因摘要不含路径和凭证），不更新成功时间戳，非 0 退出。
  2. `deploy/backup/backup-console.sh`：console-server Postgres（`pg_dump`）同格式清单，`source=console`。
  3. 对象存储策略（MinIO / S3 policy JSON）：备份任务凭证可写 `backups/`、`manifests/`；console-server 凭证只能 List / Get `manifests/`，不能读 `backups/`、不能写或删任何对象。
  4. 部署 e2e：一次成功备份（大小 / SHA-256 与对象一致）、加密失败、删除最近 3 份清单、console-server 凭证四项权限检查、指标与清单结束时间相差 ≤ 1 秒。
- 估算：1 人日

### BE-314 发布脚本与内部 CA
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-313、BE-305
- AC：AC-OPS-13-1、AC-OPS-13-2、AC-OPS-13-5、AC-OPS-16-5
- 内容：
  1. S-OPS-29：`deploy/release.sh`：`im-server migrate status`（新二进制）判断有无待执行迁移 → 有则调用与定时备份完全相同的 `backup.sh --trigger pre-migrate --release <版本>`，任一步失败立即非 0 退出（不执行 `migrate up`、不切换二进制）→ `migrate up` → 切换二进制并重启；没有待执行迁移时写"无待执行迁移，跳过发布前备份"。
  2. S-OPS-30：`deploy/pki/README.md` + `deploy/pki/issue-node-cert.sh`：内部 CA、节点运维证书（服务名 `<node_id>.ops.internal`）签发与轮换步骤；测试环境用脚本生成 n1、n2、n9 三张证书（n9 用于伪造节点用例 AC-OPS-16-5）。CA 私钥不进仓库。
  3. 部署 e2e：有迁移时备份结束时间早于 `migrate up`；上传失败时旧二进制仍在运行、`0002` 未执行；无迁移时不产生新清单。
- 估算：1 人日

---

## 第一批：测试环境与性能验收

### BE-320 运维测试环境编排（E1 / E2a）与 IM 解耦验证
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-300 ～ BE-314；console-server 可运行（CS-110 起）
- AC：AC-OPS-90-5、AC-OPS-08-11、AC-OPS-04-6
- 内容：
  1. `deploy/testenv/`：E1（1 个 im-server `IM_TEST_MODE=1`、`IM_NODE_ID=n1` + console-server 基座测试模式 + Postgres + Prometheus + Alertmanager + Grafana + webhook 接收器 + MinIO + node_exporter textfile）与 E2a（再加 n2，同 JWT 密钥、各自 SQLite、静态节点列表）的编排文件和启动说明。**只交付编排文件，不在开发机安装**；运行依赖测试机上的容器运行时。
  2. webhook 接收器（记录每条通知，供 AC-OPS-07-x 断言）；`tools/set-clocks.sh` 的运维版本（先 [IM 钟] 后 [C 钟]）。
  3. 两节点不同版本号（构建时注入 `0.3.0` / `0.3.1`）、`kill -STOP` 暂停节点等辅助脚本。
  4. AC-OPS-90-5：同时停止 console-server、Grafana、Prometheus 后跑 TC-PERF-00-01。
- 估算：1.5 人日

### BE-321 EP 性能验收：交替对照与容量面板核对
- 优先级：P0
- 批次：第一批
- 仓库：tianting-server
- 依赖：BE-307、BE-312、BE-311、CS-114、CS-117
- AC：AC-OPS-90-1、AC-OPS-90-2、AC-OPS-06-2、AC-OPS-06-10、AC-OPS-07-8
- 内容：
  1. 按 AC-OPS-90-1 跑 `opsimpact`：1 万连接 + 500 条/秒；运维流量（20 个 `ops_engineer` 合计 10 次/秒排查 + 15 秒总览 + 60 秒配置 / 数据库页 + Grafana 24 小时面板 30 秒刷新）；关 / 开交替 3 次，每段 5 分钟，取 3 个开段 P99 中位数与 3 个关段 P99 中位数比较。
  2. 同轮采集 console-server 接口耗时（AC-OPS-90-2），以及 Grafana 吞吐 / P99 / 每连接存活内存面板读数（AC-OPS-06-2、06-10）；GOMEMLIMIT 300MiB / 800MiB 两段（AC-OPS-07-8）。
  3. 报告写入 `docs/reports/perf-ops-<日期>.md`，注明服务端与压测客户端同机、结论只在同一轮内比较（C-50）。
- 估算：1 人日

---

## B6：集群相关（不计入第一批验收）

### BE-340 Postgres 位置脱敏与库大小 / 行数估算
- 优先级：P1
- 批次：B6
- 仓库：tianting-server
- 依赖：B6 的 Postgres 实现（backend.md BE-070）；BE-304、BE-305
- AC：AC-OPS-10-10、AC-OPS-11-6
- 内容：
  1. `/internal/v1/config`、`/internal/v1/db` 的数据库位置在 Postgres 下只输出主机和库名，去掉用户名和密码（连接串解析后重建，不做字符串替换）；日志同样脱敏。
  2. 第二批（仅 Postgres）：`/internal/v1/db` 增加数据库大小（`pg_database_size`）和消息表行数估算（`pg_class.reltuples`），**不执行 `count(*)`**；只读账号。
- 估算：1 人日（随 B6 计入）
