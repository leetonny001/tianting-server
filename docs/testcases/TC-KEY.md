# 测试用例：管理接口收紧（KEY，X-5 前置改造）

> 依据：[PRD-admin v1.0](../prd/PRD-admin-v1.0.md) 第 3 节 F-KEY-01～05（冻结）、第 10 节错误码、[ADMIN.md](../../protocol/ADMIN.md)、[errors.json](../../protocol/errors.json)。任务：[backend-admin.md](../tasks/backend-admin.md) BE-200～BE-203、BE-211、BE-225。
>
> 通用约定：
> - 环境：每个用例独立数据库；im-server `IM_TEST_MODE=1`（除非用例写明关闭）；**[IM 钟]** = im-server 注入时钟 `POST /admin/test/clock {nowMs}`（冻结语义，IMP-7），只在管理地址上可用。本文件不涉及 console 钟。
> - 地址：对外端口 `IM_ADDR`（下称"对外端口"）；管理地址 `IM_ADMIN_ADDR=127.0.0.1:8081`（下称"管理地址"）。
> - 调用方夹具（`IM_ADMIN_CALLERS`，权限 0600）：`console`（Ed25519，scope 按 F-KEY-02 规则 4 第一批）、`console-msgread`（Ed25519，`bodyAccess: true`，`msg.body.group`、`msg.quarantine`、`msg.body.c2c`）、`biz1`（HMAC，只有 `token.issue`）、`biz-full`（HMAC，业务方典型 scope）、`biz-off`（`disabled: true`）。"正确签名"指按 F-KEY-03 规则 2 的 8 行待签名串、当前 [IM 钟] 时间戳、新随机 nonce。
> - 旧密钥：`IM_ADMIN_KEY=k`，非生产。
> - 进程级用例：以子进程启动 `im-server`，断言退出码与标准错误输出。
> - 用户 `u1`、`u2`，群 `g1` 已按需导入。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-KEY-01-01 | 对外端口上没有任何 /admin/* 路由（负向） | AC-KEY-01-1 | 后端 e2e | P0 |
| TC-KEY-01-02 | 同样的请求发到管理地址正常处理 | AC-KEY-01-1 | 后端 e2e | P0 |
| TC-KEY-01-03 | 生产环境非回环管理地址未配置 TLS 拒绝启动 | AC-KEY-01-2 | 进程级 | P0 |
| TC-KEY-01-04 | 生产环境管理地址绑定回环时允许明文 | AC-KEY-01-3 | 进程级 | P0 |
| TC-KEY-01-05 | 来源 IP 不在 CIDR 内时伪造 X-Forwarded-For 无效（负向） | AC-KEY-01-4 | 后端 e2e | P0 |
| TC-KEY-01-06 | 来源 IP 在 CIDR 内时正常处理 | AC-KEY-01-4 | 后端 e2e | P1 |
| TC-KEY-01-07 | 管理地址与指标地址相同拒绝启动 | AC-KEY-01-5 | 进程级 | P0 |
| TC-KEY-01-08 | 管理地址与运维地址相同拒绝启动 | AC-KEY-01-5 | 进程级 | P0 |
| TC-KEY-01-09 | 测试接口只在管理地址上注册 | AC-KEY-01-6 | 后端 e2e | P0 |
| TC-KEY-02-01 | scope 不足：只有 token.issue 的调用方建群被拒（负向） | AC-KEY-02-1 | 后端 e2e | P0 |
| TC-KEY-02-02 | 全部管理路由 × 有 / 无 scope 参数化 | AC-KEY-02-2 | 后端 e2e | P0 |
| TC-KEY-02-03 | 新增未声明 scope 的路由启动失败（负向） | AC-KEY-02-3 | 后端单测 | P0 |
| TC-KEY-02-04 | 主凭证同时带元数据与正文 scope 拒绝启动（负向） | AC-KEY-02-4 | 进程级 | P0 |
| TC-KEY-02-05 | 正文类 scope 的其他配置违规（负向） | AC-KEY-02-4 | 进程级 | P0 |
| TC-KEY-02-06 | 调用方密钥重复或与其他密钥相同拒绝启动（负向） | AC-KEY-02-5 | 进程级 | P0 |
| TC-KEY-02-07 | console 主凭证不能签发 token（负向） | AC-KEY-02-6 | 后端 e2e | P0 |
| TC-KEY-02-08 | 单聊正文开关关闭时即使有 scope 也拿不到授权（负向） | AC-KEY-02-7 | 后端 e2e | P0 |
| TC-KEY-02-09 | 任何管理接口都不能打开单聊正文开关（负向） | AC-KEY-02-7 | 后端 e2e | P0 |
| TC-KEY-02-10 | 停用的调用方返回 401 / 4017（负向） | AC-KEY-02-8 | 后端 e2e | P0 |
| TC-KEY-02-11 | 不存在的调用方返回 401 / 4017（负向） | AC-KEY-02-8 | 后端 e2e | P1 |
| TC-KEY-03-01 | 签名测试向量（HMAC 与 Ed25519） | AC-KEY-03-1 | 后端单测 | P0 |
| TC-KEY-03-02 | 篡改 body、操作人、参数值后签名失效（负向） | AC-KEY-03-2 | 后端 e2e | P0 |
| TC-KEY-03-03 | 只改变查询串参数顺序仍然有效（对照） | AC-KEY-03-2 | 后端 e2e | P1 |
| TC-KEY-03-04 | 时间戳窗口 ±300 秒 | AC-KEY-03-3 | 后端 e2e | P0 |
| TC-KEY-03-05 | 原样重放成功请求被拒绝，业务只执行一次（负向） | AC-KEY-03-4 | 后端 e2e | P0 |
| TC-KEY-03-06 | 换 nonce 不重算签名同样被拒（负向） | AC-KEY-03-4 | 后端 e2e | P1 |
| TC-KEY-03-07 | 签名错误的请求不登记 nonce | AC-KEY-03-5 | 后端 e2e | P0 |
| TC-KEY-03-08 | 缺签名头：指标增加、审计无新增 | AC-KEY-03-6 | 后端 e2e | P0 |
| TC-KEY-03-09 | 缺时间戳、nonce 长度不合法、目标节点不符（负向） | AC-KEY-03-6 | 后端 e2e | P1 |
| TC-KEY-03-10 | nonce 存储满时拒绝新请求 | AC-KEY-03-4 | 后端单测 | P2 |
| TC-KEY-04-01 | 强制下线的 intent / result 字段齐全 | AC-KEY-04-1 | 后端 e2e | P0 |
| TC-KEY-04-02 | targets 超过 50 个只记数量和前 50 个 | AC-KEY-04-1 | 后端 e2e | P2 |
| TC-KEY-04-03 | intent 写失败时操作不执行（负向） | AC-KEY-04-2 | 后端 e2e | P0 |
| TC-KEY-04-04 | 审计与日志不含备注和推送 data 原文 | AC-KEY-04-3 | 后端 e2e | P0 |
| TC-KEY-04-05 | 存储接口对 admin_audit 只有追加与查询（契约） | AC-KEY-04-4 | 后端单测 | P0 |
| TC-KEY-04-06 | 审计查询的时间范围必填且不超过 31 天（负向） | AC-KEY-04-5 | 后端 e2e | P0 |
| TC-KEY-05-01 | 旧密钥调用 v1.2 九个接口行为不变 | AC-KEY-05-1 | 后端 e2e | P0 |
| TC-KEY-05-02 | 旧密钥调用新增接口一律 403（负向） | AC-KEY-05-2 | 后端 e2e | P0 |
| TC-KEY-05-03 | 同时带签名头与旧密钥时以签名头为准 | AC-KEY-05-2 | 后端 e2e | P1 |
| TC-KEY-05-04 | 生产环境默认不接受旧密钥（负向） | AC-KEY-05-3 | 后端 e2e | P0 |
| TC-KEY-05-05 | 生产环境显式打开旧密钥时告警 | AC-KEY-05-3 | 进程级 | P1 |
| TC-KEY-05-06 | 旧密钥调用写审计且操作人为空 | AC-KEY-05-4 | 后端 e2e | P0 |

---

## F-KEY-01 管理接口独立内网监听地址

### TC-KEY-01-01 对外端口上没有任何 /admin/* 路由（负向）
- 关联 AC：AC-KEY-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：默认配置启动；`biz-full` 正确签名；旧密钥 `k`。
- 步骤：
  1. 在对外端口请求 `POST /admin/users/token {userId:"u1"}`（带正确签名）。
  2. 在对外端口请求 `GET /admin/users/u1`（带 `X-Admin-Key: k`）。
  3. 在对外端口请求 `GET /admin/audit?from=…&to=…`。
- 预期结果：
  1. 三次都返回 HTTP 404（未注册路由），响应体不是管理接口的错误格式。
  2. im-server 没有签发 token，`admin_audit` 无新增。

### TC-KEY-01-02 同样的请求发到管理地址正常处理
- 关联 AC：AC-KEY-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-KEY-01-01。
- 步骤：
  1. 把 TC-KEY-01-01 的前两个请求改发到管理地址。
- 预期结果：
  1. 签发 token 返回 200 且 token 可用于 `/ws` 鉴权；查询用户返回 200。

### TC-KEY-01-03 生产环境非回环管理地址未配置 TLS 拒绝启动
- 关联 AC：AC-KEY-01-2
- 优先级：P0
- 层级：进程级
- 前置条件：`IM_ENV=production`、`IM_ADMIN_ADDR=10.0.0.5:8081`，未设 `IM_ADMIN_TLS_CERT` / `IM_ADMIN_TLS_KEY`，其余生产必填项合格。
- 步骤：
  1. 启动 im-server。
- 预期结果：
  1. 进程以非 0 退出。
  2. 日志指出管理端口缺少 TLS 证书。

### TC-KEY-01-04 生产环境管理地址绑定回环时允许明文
- 关联 AC：AC-KEY-01-3
- 优先级：P0
- 层级：进程级
- 前置条件：`IM_ENV=production`、`IM_ADMIN_ADDR=127.0.0.1:8081`，无 TLS，其余生产必填项合格（`IM_ADMIN_LEGACY` 未设）。
- 步骤：
  1. 启动 im-server。
  2. 向 `127.0.0.1:8081` 发一个正确签名的 `GET /admin/users/u1`。
- 预期结果：
  1. 启动成功，`/readyz` 返回 200。
  2. 请求返回 200。另测 `IM_ADMIN_ADDR=[::1]:8081` 同样成功。

### TC-KEY-01-05 来源 IP 不在 CIDR 内时伪造 X-Forwarded-For 无效（负向）
- 关联 AC：AC-KEY-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_ADMIN_ALLOWED_CIDRS=10.0.0.0/8`；测试替身把 TCP 对端地址设为 `192.168.1.10`。
- 步骤：
  1. 发正确签名的强制下线 `POST /admin/users/u1/force-logout`，带 `X-Forwarded-For: 10.0.0.1`。
- 预期结果：
  1. HTTP 403 / 4030。
  2. u1 的连接未被踢；`admin_audit` 无新增（未进入业务 handler）。

### TC-KEY-01-06 来源 IP 在 CIDR 内时正常处理
- 关联 AC：AC-KEY-01-4
- 优先级：P1
- 层级：后端 e2e
- 前置条件：同 TC-KEY-01-05，TCP 对端地址为 `10.2.3.4`。
- 步骤：
  1. 发同一请求（不带 `X-Forwarded-For`）。
- 预期结果：
  1. 返回 200，u1 被强制下线。

### TC-KEY-01-07 管理地址与指标地址相同拒绝启动
- 关联 AC：AC-KEY-01-5
- 优先级：P0
- 层级：进程级
- 前置条件：`IM_ADMIN_ADDR=127.0.0.1:9090`、`IM_METRICS_ADDR=127.0.0.1:9090`。
- 步骤：
  1. 启动 im-server。
- 预期结果：
  1. 非 0 退出；日志同时列出 `IM_ADMIN_ADDR` 与 `IM_METRICS_ADDR`。

### TC-KEY-01-08 管理地址与运维地址相同拒绝启动
- 关联 AC：AC-KEY-01-5
- 优先级：P0
- 层级：进程级
- 前置条件：`IM_ADMIN_ADDR=127.0.0.1:8081`、`IM_OPS_ADDR=127.0.0.1:8081`。
- 步骤：
  1. 启动 im-server。
  2. 另测 `IM_ADMIN_ADDR` 与 `IM_PPROF_ADDR` 相同；以及 `IM_OPS_ADDR=off` 时与管理地址取值无关。
- 预期结果：
  1. 前两组非 0 退出，日志列出冲突的两个地址。
  2. `off` 的地址不参与比较，启动成功。

### TC-KEY-01-09 测试接口只在管理地址上注册
- 关联 AC：AC-KEY-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_TEST_MODE=1`。
- 步骤：
  1. 在对外端口请求 `POST /admin/test/clock {nowMs: 1700000000000}`。
  2. 在管理地址上请求同一接口（旧密钥或 `test` scope 调用方）。
- 预期结果：
  1. 对外端口 404，[IM 钟] 未改变。
  2. 管理地址 200，之后签发的 token `iatMs` 等于设置值。

---

## F-KEY-02 调用方密钥与 scope

### TC-KEY-02-01 scope 不足：只有 token.issue 的调用方建群被拒（负向）
- 关联 AC：AC-KEY-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：调用方 `biz1` 只有 `token.issue`；用户 u1、u2 存在。
- 步骤：
  1. `biz1` 正确签名调用 `POST /admin/groups {groupId:"g9", name:"x", members:["u1","u2"]}`。
  2. 查询 g9（`biz-full`）。
- 预期结果：
  1. HTTP 403 / 4030。
  2. g9 不存在（404 / 4043）。

### TC-KEY-02-02 全部管理路由 × 有 / 无 scope 参数化
- 关联 AC：AC-KEY-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试表 `scope_test.go`：每个 `/admin/*` 路由一个合法请求样本；每个路由准备两个调用方：只有该路由 scope 的、拥有其他全部 scope 但缺这一个的。
- 步骤：
  1. 逐个路由用两个调用方各发一次（每次前重置数据）。
  2. 统计测试表路由数与注册路由数。
- 预期结果：
  1. 有 scope 时返回该接口的成功状态码；没有时 403 / 4030 且无副作用。
  2. 两个数量相等（新增路由没有样本时测试失败）。

### TC-KEY-02-03 新增未声明 scope 的路由启动失败（负向）
- 关联 AC：AC-KEY-02-3
- 优先级：P0
- 层级：后端单测
- 前置条件：测试构建标签下注入一个 `adm.handleFunc("GET /admin/debug", h)`（未走带 scope 的注册函数）。
- 步骤：
  1. 运行 `go test ./internal/admin/...`。
  2. 以该构建启动服务。
- 预期结果：
  1. 测试失败，信息含 "route without scope"。
  2. 启动阶段 panic，非 0 退出。

### TC-KEY-02-04 主凭证同时带元数据与正文 scope 拒绝启动（负向）
- 关联 AC：AC-KEY-02-4
- 优先级：P0
- 层级：进程级
- 前置条件：`console` 的 scopes 为 `["msg.meta", "msg.body.group"]`。
- 步骤：
  1. 启动 im-server。
- 预期结果：
  1. 非 0 退出，日志指出正文类 scope 只能在独立的 `bodyAccess` 调用方上。

### TC-KEY-02-05 正文类 scope 的其他配置违规（负向）
- 关联 AC：AC-KEY-02-4
- 优先级：P0
- 层级：进程级
- 前置条件：分别配置：① 两个 `bodyAccess: true` 调用方；② `console-msgread` 同时带 `msg.body.group` 和 `user.read`；③ `msg.quarantine` 配在 `bodyAccess` 为 false 的调用方上。
- 步骤：
  1. 分别启动。
- 预期结果：
  1. 三种都非 0 退出，日志指出违规的调用方和 scope。

### TC-KEY-02-06 调用方密钥重复或与其他密钥相同拒绝启动（负向）
- 关联 AC：AC-KEY-02-5
- 优先级：P0
- 层级：进程级
- 前置条件：分别配置：① `biz1`、`biz-full` 使用相同 HMAC 密钥；② `biz1` 密钥等于 `IM_JWT_SECRET`；③ `biz1` 密钥等于 `IM_ADMIN_KEY`；④ `biz1` 密钥只有 31 字节；⑤ 调用方文件权限 0644。
- 步骤：
  1. 分别启动。
- 预期结果：
  1. 五种都非 0 退出，日志指出原因（不打印密钥本身）。

### TC-KEY-02-07 console 主凭证不能签发 token（负向）
- 关联 AC：AC-KEY-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`console` 按 F-KEY-02 规则 4 配置。
- 步骤：
  1. `console` 正确签名调用 `POST /admin/users/token {userId:"u1"}`。
  2. 再调用 `/admin/test/clock`、`POST /admin/msg-grants`。
- 预期结果：
  1. 签发 token 返回 403 / 4030，响应中没有 token。
  2. 后两个都 403 / 4030。

### TC-KEY-02-08 单聊正文开关关闭时即使有 scope 也拿不到授权（负向）
- 关联 AC：AC-KEY-02-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_C2C_BODY_ACCESS=off`（默认）；`console-msgread` 带 `msg.body.c2c`；`IM_V13_ADMIN_FEATURES=1`。
- 步骤：
  1. `console-msgread` 为 `c2c:u1:u2` 调用 `POST /admin/msg-grants`。
  2. 查询 `msg_grants` 表。
- 预期结果：
  1. HTTP 403 / 4030，`reason=c2cBodyDisabled`。
  2. 没有新授权记录；`admin_audit` 有 intent 和 `code=4030` 的 result。

### TC-KEY-02-09 任何管理接口都不能打开单聊正文开关（负向）
- 关联 AC：AC-KEY-02-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_C2C_BODY_ACCESS=off`；调用方拥有全部 scope 的测试夹具（含 `test`）。
- 步骤：
  1. 遍历全部管理路由（含 `PATCH /admin/settings/social`、`/admin/test/*`），在请求体中加入 `c2cBodyAccess: true`、`IM_C2C_BODY_ACCESS: "on"` 等字段发送。
  2. 再次为 `c2c:u1:u2` 申请授权。
- 预期结果：
  1. 没有任何接口改变开关（未知字段被忽略或返回 400）。
  2. 申请仍 403 / 4030 `c2cBodyDisabled`。代码检查：开关只在 `config` 中从环境变量读取一次（单测断言没有 setter）。

### TC-KEY-02-10 停用的调用方返回 401 / 4017（负向）
- 关联 AC：AC-KEY-02-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`biz-off`（`disabled: true`，scope 足够）。
- 步骤：
  1. `biz-off` 正确签名调用 `GET /admin/users/u1`。
- 预期结果：
  1. HTTP 401 / 4017；`im_admin_auth_failures_total{reason="caller"}` +1。

### TC-KEY-02-11 不存在的调用方返回 401 / 4017（负向）
- 关联 AC：AC-KEY-02-8
- 优先级：P1
- 层级：后端 e2e
- 前置条件：无。
- 步骤：
  1. 用 `X-IM-Caller: nobody` 和任意签名调用 `GET /admin/users/u1`。
- 预期结果：
  1. HTTP 401 / 4017；`msg` 与签名错误不同（只用于排查），HTTP 状态和 `code` 相同。

---

## F-KEY-03 请求签名与防重放

### TC-KEY-03-01 签名测试向量（HMAC 与 Ed25519）
- 关联 AC：AC-KEY-03-1
- 优先级：P0
- 层级：后端单测
- 前置条件：`protocol/vectors/admin-sign.json`：HMAC 与 Ed25519 各 ≥ 3 组，含查询串排序、空 body、中文 body。
- 步骤：
  1. im-server 签名校验函数逐组计算待签名串与签名。
  2. Go / Java / Node.js 示例代码各自跑同一份向量。
- 预期结果：
  1. 每组的待签名串与签名都和向量一致。
  2. 三种语言示例全部通过。

### TC-KEY-03-02 篡改 body、操作人、参数值后签名失效（负向）
- 关联 AC：AC-KEY-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`console` 对 `GET /admin/users?q=u&limit=20` 和 `POST /admin/users/u1/force-logout {}`（`X-IM-Operator: admin:a1`）生成合法签名。
- 步骤：
  1. 把 body 改一个字节后发送。
  2. 把 `X-IM-Operator` 改为 `admin:a2` 后发送。
  3. 把 `q=u` 改为 `q=v` 后发送。
  4. 把 `X-Request-Id` 改掉后发送。
- 预期结果：
  1. 四种都 401 / 4017，业务未执行（u1 未被踢）。

### TC-KEY-03-03 只改变查询串参数顺序仍然有效（对照）
- 关联 AC：AC-KEY-03-2
- 优先级：P1
- 层级：后端 e2e
- 前置条件：同上。
- 步骤：
  1. 发送 `GET /admin/users?limit=20&q=u`（与签名时顺序不同、值相同）。
- 预期结果：
  1. 200（服务端按键名排序重建待签名串）。

### TC-KEY-03-04 时间戳窗口 ±300 秒
- 关联 AC：AC-KEY-03-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：[IM 钟] 冻结在 T。
- 步骤：
  1. 分别以 `X-IM-Timestamp = T − 301 秒`、`T + 301 秒`、`T − 299 秒`、`T + 299 秒` 签名并发送。
- 预期结果：
  1. 前两个 401 / 4017（`reason=window`）；后两个成功。

### TC-KEY-03-05 原样重放成功请求被拒绝，业务只执行一次（负向）
- 关联 AC：AC-KEY-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u1 在线 1 个连接；抓取一次成功的强制下线完整报文（含 nonce）。
- 步骤：
  1. u1 重新签发 token 并连接。
  2. 5 分钟内（[IM 钟] +60 秒）原样重放该报文。
  3. 查 `admin_audit`。
- 预期结果：
  1. 重放返回 401 / 4017（`reason=nonce`），u1 新连接未被踢。
  2. `admin_audit` 只有一组 intent / result（成功）。

### TC-KEY-03-06 换 nonce 不重算签名同样被拒（负向）
- 关联 AC：AC-KEY-03-4
- 优先级：P1
- 层级：后端 e2e
- 前置条件：同上的成功报文。
- 步骤：
  1. 只替换 `X-IM-Nonce` 为新值后重放。
- 预期结果：
  1. 401 / 4017（签名错误，因为 nonce 在签名范围内）。

### TC-KEY-03-07 签名错误的请求不登记 nonce
- 关联 AC：AC-KEY-03-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：准备 1000 个签名错误、nonce 都为 N 的请求。
- 步骤：
  1. 依次发送 1000 个错误请求。
  2. 用 nonce N 发一个正确签名的请求。
- 预期结果：
  1. 前 1000 个都 401 / 4017。
  2. 最后一个成功。

### TC-KEY-03-08 缺签名头：指标增加、审计无新增
- 关联 AC：AC-KEY-03-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：记录 `im_admin_auth_failures_total{reason="missing"}` 与 `admin_audit` 行数。
- 步骤：
  1. 发带 `X-IM-Caller` 但缺 `X-IM-Signature` 的 `GET /admin/users/u1`。
- 预期结果：
  1. 401 / 4017。
  2. 指标 +1；`admin_audit` 行数不变。

### TC-KEY-03-09 缺时间戳、nonce 长度不合法、目标节点不符（负向）
- 关联 AC：AC-KEY-03-6
- 优先级：P1
- 层级：后端 e2e
- 前置条件：`IM_NODE_ID=n1`。
- 步骤：
  1. 分别发送：缺 `X-IM-Timestamp`；nonce 15 个字符；nonce 65 个字符；`X-IM-Target-Node: n2`（节点运维路由样本，若 PRD-ops 接口未就绪则用测试路由）。
- 预期结果：
  1. 都 401 / 4017，均不写 `admin_audit`。

### TC-KEY-03-10 nonce 存储满时拒绝新请求
- 关联 AC：AC-KEY-03-4
- 优先级：P2
- 层级：后端单测
- 前置条件：单测把内存 nonce 容量设为 10。
- 步骤：
  1. 连续发 11 个不同 nonce 的正确请求（10 分钟内）。
- 预期结果：
  1. 第 11 个返回 503 / 5030，并打告警日志；容量释放（推进 [IM 钟] 10 分钟）后恢复。

---

## F-KEY-04 im-server 侧管理接口审计

### TC-KEY-04-01 强制下线的 intent / result 字段齐全
- 关联 AC：AC-KEY-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u1 在线。
- 步骤：
  1. `console` 带 `X-IM-Operator: admin:a1`、`X-Request-Id: r-1` 强制下线 u1。
  2. `GET /admin/audit?requestId=r-1&from=…&to=…`（`audit.read`）。
- 预期结果：
  1. 两条记录：`phase` 分别为 `intent`、`result`；`caller=console`、`operator=admin:a1`、`requestId=r-1`、`targets=[u1]`、`route` 为强制下线；result `code=0`。

### TC-KEY-04-02 targets 超过 50 个只记数量和前 50 个
- 关联 AC：AC-KEY-04-1
- 优先级：P2
- 层级：后端 e2e
- 前置条件：群 g1 存在；120 个用户已导入。
- 步骤：
  1. `console` 一次加 120 人到 g1。
  2. 查该请求的审计。
- 预期结果：
  1. `targets` 只有前 50 个 userId，另有总数 120。

### TC-KEY-04-03 intent 写失败时操作不执行（负向）
- 关联 AC：AC-KEY-04-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u1 在线；测试注入 `adminAuditWrite=fail`。
- 步骤：
  1. `console` 强制下线 u1。
- 预期结果：
  1. 503 / 5030。
  2. u1 的连接仍在，token 未作废。

### TC-KEY-04-04 审计与日志不含备注和推送 data 原文
- 关联 AC：AC-KEY-04-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：好友导入接口可用（PRD-social BE-114；未就绪时用测试模式路由）；第二批推送接口可用时追加推送步骤。
- 步骤：
  1. 好友导入一对，`remarkA=ZXQ-REMARK-789`。
  2. （第二批）推送 `data` 含 `ZXQ-PUSH-456`。
  3. 扫描 `admin_audit` 全部列和服务端日志缓冲。
- 预期结果：
  1. 两个特征串都搜不到；推送记录只有 data 长度和 SHA-256。

### TC-KEY-04-05 存储接口对 admin_audit 只有追加与查询（契约）
- 关联 AC：AC-KEY-04-4
- 优先级：P0
- 层级：后端单测
- 前置条件：`store.Store` 接口。
- 步骤：
  1. 契约测试反射列出接口中名称含 `AdminAudit` 的方法。
- 预期结果：
  1. 只有 `AppendAdminAudit`、`QueryAdminAudit`；过期清理方法在单独的窄接口中，业务代码拿不到。

### TC-KEY-04-06 审计查询的时间范围必填且不超过 31 天（负向）
- 关联 AC：AC-KEY-04-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：调用方有 `audit.read`。
- 步骤：
  1. `GET /admin/audit` 不带 `from` / `to`。
  2. 带 32 天范围。
  3. 带 31 天范围。
- 预期结果：
  1. 前两个 400 / 4000。
  2. 第三个 200。

---

## F-KEY-05 旧 X-Admin-Key 过渡

### TC-KEY-05-01 旧密钥调用 v1.2 九个接口行为不变
- 关联 AC：AC-KEY-05-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_ADMIN_KEY=k`（非生产）；现有 `server/e2e` 只把管理请求地址改为管理地址。
- 步骤：
  1. 运行现有 `server/e2e` 全部用例（不改用例代码）。
- 预期结果：
  1. 全部通过。

### TC-KEY-05-02 旧密钥调用新增接口一律 403（负向）
- 关联 AC：AC-KEY-05-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同上；g1 存在；u1、u2 存在。
- 步骤：
  1. 旧密钥依次调用 `GET /admin/users?q=u`、`DELETE /admin/groups/g1`、`POST /admin/friends/import`、`POST /admin/convs/messages/query`、`POST /admin/users/sessions/query`、`GET /admin/audit`。
- 预期结果：
  1. 全部 403 / 4030。
  2. g1 未解散，好友未导入。

### TC-KEY-05-03 同时带签名头与旧密钥时以签名头为准
- 关联 AC：AC-KEY-05-2
- 优先级：P1
- 层级：后端 e2e
- 前置条件：`biz1` 只有 `token.issue`。
- 步骤：
  1. 请求 `GET /admin/users/u1`，同时带 `biz1` 的正确签名和 `X-Admin-Key: k`。
- 预期结果：
  1. 403 / 4030（按 `biz1` 的 scope 判断，忽略旧密钥）。

### TC-KEY-05-04 生产环境默认不接受旧密钥（负向）
- 关联 AC：AC-KEY-05-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`IM_ENV=production`（测试用自签名证书满足其他检查）、配置了 `IM_ADMIN_KEY`、未设 `IM_ADMIN_LEGACY`。
- 步骤：
  1. 旧密钥调用 `GET /admin/users/u1`。
- 预期结果：
  1. 401 / 4013。

### TC-KEY-05-05 生产环境显式打开旧密钥时告警
- 关联 AC：AC-KEY-05-3
- 优先级：P1
- 层级：进程级
- 前置条件：同上，另设 `IM_ADMIN_LEGACY=1`。
- 步骤：
  1. 启动并请求 `/readyz` 详情；旧密钥调用 `GET /admin/users/u1`。
- 预期结果：
  1. 启动日志有告警；`/readyz` 详情含 `legacyAdminKey: true`；调用成功。

### TC-KEY-05-06 旧密钥调用写审计且操作人为空
- 关联 AC：AC-KEY-05-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u1 在线。
- 步骤：
  1. 旧密钥强制下线 u1（附带 `X-IM-Operator: admin:a1`，未签名）。
  2. 查 `admin_audit`。
- 预期结果：
  1. 有 intent / result，`caller=legacy`、`operator` 为空（未签名的操作人头被忽略）。
