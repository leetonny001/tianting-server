# 测试用例：console-server 公共基座（CON）

> 依据：[PRD-admin v1.0](../prd/PRD-admin-v1.0.md) 第 4 节 F-CON-01～12（两个后台的唯一规格，冻结）、第 2.3、2.4 节、第 10 节错误码；[PRD-ops v1.0](../prd/PRD-ops-v1.1.md) 第 4 节 B-OPS（运维侧验收另见 TC-OPS）。任务：[console.md](../tasks/console.md) CS-001～CS-017、[frontend-admin.md](../tasks/frontend-admin.md) FE-400～FE-405。
>
> 通用约定：
> - 环境：console-server 连真实 Postgres（每个用例独立库，`console-server migrate up` 后运行时用 `console_app`）；`CON_TEST_MODE=1`、`CON_ENV=test`（生产保护类用例除外）；im-server 测试模式（`IM_TEST_MODE=1`），console 用 `console` 调用方签名调用它。
> - 两个时钟：**[C 钟]** = console-server `POST /console/test/clock {nowMs}`，驱动会话过期、锁定、激活码、step-up 窗口、TOTP 步长、锚定与对账任务；**[IM 钟]** = im-server `POST /admin/test/clock`。需要两边一致时用 `tools/set-clocks.sh <nowMs>`（先 IM 后 C）。
> - 故障注入：`POST /console/test/faults {auditWrite: "fail"|"failResult"|"ok", imLatencyMs}`。
> - 账号夹具（admin 空间，均已激活并绑定 TOTP，测试持有种子可计算动态码）：`a1`、`a2`（`super_admin`）、`kf1`（`support`）、`op1`（`operator`）、`mod1`（`moderator`）、`aud1`（`auditor`）；ops 空间：`ops1`（`ops_engineer`）、`osa1`（`ops_superadmin`）、`oaud1`（`ops_auditor`）。"已 step-up"指当前会话 5 分钟内成功调用过 `POST /console/api/auth/step-up`。
> - 请求默认带正确的 `X-CSRF-Token` 与 `Origin: CON_ADMIN_ORIGIN`（CSRF 用例除外）。
> - 层级：集成 = Go 测试连 Postgres 调 HTTP 接口；端到端 = console-server + im-server；UI 自动化 = Playwright 对接上述环境；进程级 = 子进程启动看退出码与日志。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-CON-01-01 | 来源不在白名单时伪造 X-Forwarded-For 无效（负向） | AC-CON-01-1 | 集成 | P0 |
| TC-CON-01-02 | 可信代理转发时按真实来源判断 | AC-CON-01-2 | 集成 | P0 |
| TC-CON-01-03 | 可信代理终止 TLS 时缺 X-Forwarded-Proto 被拒（负向） | AC-CON-01-2 | 集成 | P1 |
| TC-CON-01-04 | 管理空间的 Cookie 在运维监听地址上无效（跨空间，负向） | AC-CON-01-3 | 集成 | P0 |
| TC-CON-01-05 | 生产启动保护：五种不合格配置都拒绝启动 | AC-CON-01-4 | 进程级 | P0 |
| TC-CON-01-06 | 审计只追加的启动自检：UPDATE 探针必须失败 | AC-CON-01-4 | 进程级 | P0 |
| TC-CON-01-07 | 多项问题一次全部列出 | AC-CON-01-4 | 进程级 | P1 |
| TC-CON-01-08 | 两个实例共享会话：登录与退出跨实例生效 | AC-CON-01-5 | 集成 | P0 |
| TC-CON-01-09 | 只开启 admin 空间时运维监听地址不存在 | AC-CON-01-6 | 进程级 | P1 |
| TC-CON-02-01 | 命令行引导第一个超管，已有超管时拒绝 | AC-CON-02-1 | 进程级 | P0 |
| TC-CON-02-02 | 引导命令不能跨空间绕过：ops 空间独立判断 | AC-CON-02-1 | 进程级 | P1 |
| TC-CON-02-03 | 新建客服账号：激活、绑定、只见客服菜单、通知超管与审计员 | AC-CON-02-2 | UI 自动化 | P0 |
| TC-CON-02-04 | 激活码 24 小时后失效（[C 钟]） | AC-CON-02-3 | 集成 | P0 |
| TC-CON-02-05 | 激活码错误按登录失败处理，且库中只存哈希 | AC-CON-02-3 | 集成 | P1 |
| TC-CON-02-06 | setup 会话不能调用业务接口（负向） | AC-CON-02-4 | 集成 | P0 |
| TC-CON-02-07 | setup 状态直接打开业务页面跳回绑定页 | AC-CON-02-5 | UI 自动化 | P0 |
| TC-CON-02-08 | 管理空间超管看不到、改不了运维账号（跨空间 404，负向） | AC-CON-02-6 | 集成 | P0 |
| TC-CON-02-09 | 运维超管同样不能操作管理空间账号（反向，负向） | AC-CON-02-6 | 集成 | P0 |
| TC-CON-02-10 | 超管自我保护：不能改自己角色、停用或重置自己 | AC-CON-02-7 | 集成 | P0 |
| TC-CON-02-11 | 两个超管并发互相停用，始终保留至少一个 | AC-CON-02-8 | 集成 | P0 |
| TC-CON-02-12 | 重置后旧会话、旧密码 + TOTP、旧恢复码全部作废 | AC-CON-02-9 | 集成 | P0 |
| TC-CON-02-13 | 账号名在空间内唯一 | AC-CON-02-10 | 集成 | P0 |
| TC-CON-02-14 | 账号名格式与停用后的登录提示 | AC-CON-02-10 | 集成 | P1 |
| TC-CON-02-15 | 账号不能删除 | AC-CON-02-11 | 集成 | P0 |
| TC-CON-03-01 | 正确凭证一步登录进入首页 | AC-CON-03-1 | UI 自动化 | P0 |
| TC-CON-03-02 | 6 种失败响应完全相同（防枚举） | AC-CON-03-2 | 集成 | P0 |
| TC-CON-03-03 | 不存在账号与密码错误的响应时间一致 | AC-CON-03-3 | 集成 | P1 |
| TC-CON-03-04 | 同一动态码不能重复使用（登录与 step-up，[C 钟]，负向） | AC-CON-03-4 | 集成 | P0 |
| TC-CON-03-05 | TOTP 允许前后各 1 个步长（[C 钟]） | AC-CON-03-5 | 集成 | P0 |
| TC-CON-03-06 | 恢复码只能用一次，使用写审计并通知本人 | AC-CON-03-6 | UI 自动化 | P0 |
| TC-CON-03-07 | 登录更换会话 ID，防会话固定（负向） | AC-CON-03-7 | 集成 | P0 |
| TC-CON-03-08 | 日志、代理日志、审计中没有密码与动态码 | AC-CON-03-8 | 安全扫描 | P0 |
| TC-CON-03-09 | 重新生成恢复码需要 step-up | AC-CON-03-9 | 集成 | P0 |
| TC-CON-04-01 | 密码策略的 4 种拒绝原因与合规密码 | AC-CON-04-1 | 集成 | P0 |
| TC-CON-04-02 | 弱密码表固定版本且哈希一致 | AC-CON-04-1 | 后端单测 | P2 |
| TC-CON-04-03 | 密码用 Argon2id 加盐哈希 | AC-CON-04-2 | 集成 | P0 |
| TC-CON-04-04 | 账号名 + IP 连续失败 5 次后锁定，正确凭证也被拒（[C 钟]） | AC-CON-04-3 | 集成 | P0 |
| TC-CON-04-05 | 不存在的账号同样锁定、响应相同（防枚举，负向） | AC-CON-04-4 | 集成 | P0 |
| TC-CON-04-06 | 成功登录清零计数 | AC-CON-04-5 | 集成 | P1 |
| TC-CON-04-07 | 账号名全局 1 小时 50 次锁定（[C 钟]） | AC-CON-04-6 | 集成 | P0 |
| TC-CON-04-08 | IP 限流：每分钟 20 次 login + activate | AC-CON-04-7 | 集成 | P0 |
| TC-CON-04-09 | 失败计数在多实例间一致 | AC-CON-04-8 | 集成 | P0 |
| TC-CON-04-10 | 解锁需要 step-up，写审计，被锁者可立即登录 | AC-CON-04-9 | 集成 | P0 |
| TC-CON-04-11 | 命令行应急解锁写审计 | AC-CON-04-9 | 进程级 | P1 |
| TC-CON-05-01 | 会话 Cookie 属性 | AC-CON-05-1 | 集成 | P0 |
| TC-CON-05-02 | 页面脚本和 Storage 中找不到会话 ID | AC-CON-05-2 | UI 自动化 | P0 |
| TC-CON-05-03 | 空闲 30 分钟过期（[C 钟]） | AC-CON-05-3 | 集成 | P0 |
| TC-CON-05-04 | 绝对 12 小时过期（[C 钟]） | AC-CON-05-4 | 集成 | P0 |
| TC-CON-05-05 | 后台请求不续期（[C 钟]，负向） | AC-CON-05-5 | 集成 | P0 |
| TC-CON-05-06 | 退出后重放 Cookie 无效 | AC-CON-05-6 | 集成 | P0 |
| TC-CON-05-07 | CSRF：缺令牌时写操作不执行（负向） | AC-CON-05-7 | 集成 | P0 |
| TC-CON-05-08 | CSRF：错误令牌（负向） | AC-CON-05-8 | 集成 | P0 |
| TC-CON-05-09 | CSRF：Origin 不匹配（负向） | AC-CON-05-9 | 集成 | P0 |
| TC-CON-05-10 | GET 请求没有副作用（负向） | AC-CON-05-9 | 后端单测 | P1 |
| TC-CON-05-11 | 刷新页面后先取 CSRF 令牌再提交 | AC-CON-05-10 | UI 自动化 | P0 |
| TC-CON-05-12 | 每账号最多 3 个会话，第 4 个挤掉最早的 | AC-CON-05-11 | 集成 | P0 |
| TC-CON-05-13 | 被挤掉后页面提示并跳登录 | AC-CON-05-12 | UI 自动化 | P0 |
| TC-CON-05-14 | 角色变化使该账号全部会话失效 | AC-CON-05-13 | 集成 | P0 |
| TC-CON-05-15 | 接口响应带 Cache-Control: no-store | AC-CON-05-14 | 集成 | P0 |
| TC-CON-05-16 | 库里只存会话 ID 的 SHA-256 | AC-CON-05-15 | 集成 | P0 |
| TC-CON-06-01 | 未声明权限点的路由启动失败（负向） | AC-CON-06-1 | 后端单测 | P0 |
| TC-CON-06-02 | 叠加角色取权限并集 | AC-CON-06-2 | 端到端 | P0 |
| TC-CON-06-03 | 无权限返回 403 并写 denied 审计 | AC-CON-06-3 | 集成 | P0 |
| TC-CON-06-04 | 1 分钟内超过 20 次拒绝合并为一条 | AC-CON-06-4 | 集成 | P1 |
| TC-CON-06-05 | 未认证的拒绝不进审计（防刷爆） | AC-CON-06-5 | 集成 | P0 |
| TC-CON-06-06 | 互斥角色 auditor 不能与其他角色叠加（负向） | AC-CON-06-6 | 集成 | P0 |
| TC-CON-06-07 | 应急授权：申请、生效、到期自动撤销（[C 钟]） | AC-CON-06-7 | 集成 | P0 |
| TC-CON-06-08 | 应急授权的非法申请（负向） | AC-CON-06-8 | 集成 | P0 |
| TC-CON-06-09 | 应急授权只在管理空间提供（负向） | AC-CON-06-7 | 集成 | P0 |
| TC-CON-06-10 | 应急授权只能给自己，未 step-up 被拒（负向） | AC-CON-06-8 | 集成 | P1 |
| TC-CON-06-11 | 授予敏感角色通知审计员 | AC-CON-06-9 | 集成 | P0 |
| TC-CON-06-12 | 运维空间互斥角色 | AC-CON-06-10 | 集成 | P0 |
| TC-CON-07-01 | 未 step-up 调用高危操作返回 403 / 4038（不是 401） | AC-CON-07-1 | 端到端 | P0 |
| TC-CON-07-02 | step-up 5 分钟窗口（[C 钟]） | AC-CON-07-2 | 端到端 | P0 |
| TC-CON-07-03 | step-up 窗口绑定会话，不跨会话继承（负向） | AC-CON-07-2 | 集成 | P1 |
| TC-CON-07-04 | 未 step-up 点解散：弹 TOTP 框，通过后自动完成 | AC-CON-07-3 | UI 自动化 | P0 |
| TC-CON-07-05 | 正文授权不享受 5 分钟窗口 | AC-CON-07-4 | 集成 | P0 |
| TC-CON-07-06 | 连续 5 次 step-up 失败注销会话并锁定账号 | AC-CON-07-5 | 集成 | P0 |
| TC-CON-07-07 | step-up 输错只在弹框内提示 | AC-CON-07-6 | UI 自动化 | P0 |
| TC-CON-08-01 | 强制下线的审计 intent / result | AC-CON-08-1 | 端到端 | P0 |
| TC-CON-08-02 | im-server 业务错误记为 failure 并透传 | AC-CON-08-2 | 端到端 | P0 |
| TC-CON-08-03 | 运行账号不能修改或删除审计记录（负向） | AC-CON-08-3 | 集成 | P0 |
| TC-CON-08-04 | intent 写失败时操作不执行、数据不返回（负向） | AC-CON-08-4 | 端到端 | P0 |
| TC-CON-08-05 | 删除中间记录被校验发现 | AC-CON-08-5 | 集成 | P0 |
| TC-CON-08-06 | 截断尾部：锚定之后通过、锚定之前被发现 | AC-CON-08-6 | 集成 | P0 |
| TC-CON-08-07 | 改记录并整链重算被锚定发现 | AC-CON-08-7 | 集成 | P0 |
| TC-CON-08-08 | 只改一条不重算被发现 | AC-CON-08-7 | 集成 | P1 |
| TC-CON-08-09 | 两实例并发写审计链不分叉 | AC-CON-08-8 | 集成 | P0 |
| TC-CON-08-10 | result 写失败：操作生效、显示结果未知、对账补齐 | AC-CON-08-9 | 端到端 | P0 |
| TC-CON-08-11 | 调用 im-server 超时：504 / 5040、unknown、对账补齐 | AC-CON-08-10 | 端到端 | P0 |
| TC-CON-08-12 | 每类动作都有字段齐全的审计（参数化） | AC-CON-08-11 | 端到端 | P0 |
| TC-CON-08-13 | 整点锚定写入 WORM 桶且不可删除 | AC-CON-08-12 | 集成 | P0 |
| TC-CON-08-14 | 每 1000 条触发一次锚定；锚定失败重试并告警 | AC-CON-08-12 | 集成 | P1 |
| TC-CON-09-01 | 审计员按条件筛选，32 天不可查询 | AC-CON-09-1 | UI 自动化 | P0 |
| TC-CON-09-02 | 审计查询接口拒绝 32 天范围（负向） | AC-CON-09-2 | 集成 | P0 |
| TC-CON-09-03 | 审计查询本身被审计 | AC-CON-09-3 | 集成 | P0 |
| TC-CON-09-04 | 管理空间审计员不能看运维空间（负向） | AC-CON-09-4 | 集成 | P0 |
| TC-CON-09-05 | 审计接口只读 | AC-CON-09-5 | 集成 | P0 |
| TC-CON-09-06 | 对账：im-server 也没有该请求时补写 notExecuted（[C 钟]） | AC-CON-09-6 | 集成 | P0 |
| TC-CON-09-07 | 对账只在一个实例运行 | AC-CON-09-6 | 集成 | P2 |
| TC-CON-09-08 | 满 1 年的分区加密归档后删除，链校验仍通过（[C 钟]） | AC-CON-09-7 | 集成 | P0 |
| TC-CON-09-09 | 运行账号不能删除审计分区（负向） | AC-CON-09-8 | 集成 | P0 |
| TC-CON-09-10 | 运维审计员可查看管理空间记录 | AC-CON-09-9 | 端到端 | P0 |
| TC-CON-10-01 | 构建产物与 HAR 中没有任何 IM 凭证或签名私钥 | AC-CON-10-1 | 安全扫描 | P0 |
| TC-CON-10-02 | console 接口响应和错误信息不泄露签名头 | AC-CON-10-1 | 端到端 | P1 |
| TC-CON-10-03 | XSS：用户可控内容一律按纯文本显示（负向） | AC-CON-10-2 | UI 自动化 | P0 |
| TC-CON-10-04 | 安全响应头 | AC-CON-10-3 | 集成 | P0 |
| TC-CON-10-05 | 被其他域名 iframe 嵌入时浏览器拒绝加载 | AC-CON-10-4 | UI 自动化 | P0 |
| TC-CON-10-06 | 不加载外部头像 | AC-CON-10-5 | UI 自动化 | P0 |
| TC-CON-10-07 | lint 阻止 dangerouslySetInnerHTML | AC-CON-10-6 | 后端单测 | P0 |
| TC-CON-11-01 | TOTP 种子只以密文存储 | AC-CON-11-1 | 集成 | P0 |
| TC-CON-11-02 | 数据密钥轮换 | AC-CON-11-2 | 集成 | P0 |
| TC-CON-11-03 | 生产环境不合格数据密钥拒绝启动（负向） | AC-CON-11-3 | 进程级 | P0 |
| TC-CON-11-04 | Argon2id 并发信号量防内存型拒绝服务 | AC-CON-11-4 | 性能 | P1 |
| TC-CON-11-05 | 私钥类配置不出现在日志和 /readyz | AC-CON-11-3 | 安全扫描 | P1 |
| TC-CON-12-01 | 锁定通知同时到达本人和超管 | AC-CON-12-1 | 端到端 | P0 |
| TC-CON-12-02 | 各类触发的接收人参数化 | AC-CON-12-2 | 集成 | P0 |
| TC-CON-12-03 | 通知只发给启用账号、按空间隔离 | AC-CON-12-2 | 集成 | P1 |
| TC-CON-12-04 | 未读数与已读 | AC-CON-12-3 | UI 自动化 | P0 |

---

## F-CON-01 部署形态、网络边界与启动保护

### TC-CON-01-01 来源不在白名单时伪造 X-Forwarded-For 无效（负向）
- 关联 AC：AC-CON-01-1
- 优先级：P0
- 层级：集成
- 前置条件：`CON_ADMIN_ALLOWED_CIDRS=10.0.0.0/8`，`CON_TRUSTED_PROXIES` 为空；TCP 对端 `192.168.1.10`。
- 步骤：
  1. 请求登录页 `/login`，带 `X-Forwarded-For: 10.0.0.1`。
  2. 请求静态资源 `/assets/index.js` 与 `POST /console/api/auth/login`。
- 预期结果：
  1. 都返回 403 / 4030。
  2. 审计表无新增；`console_rejected_total{reason="cidr"}` 增加。

### TC-CON-01-02 可信代理转发时按真实来源判断
- 关联 AC：AC-CON-01-2
- 优先级：P0
- 层级：集成
- 前置条件：`CON_TRUSTED_PROXIES=10.0.0.2`；TCP 对端为 10.0.0.2。
- 步骤：
  1. 带 `X-Forwarded-For: 192.168.1.10` 请求登录页。
  2. 带 `X-Forwarded-For: 10.1.2.3` 请求登录页。
  3. 带 `X-Forwarded-For: 10.1.2.3, 192.168.1.10`（最右为不可信外网地址）请求。
- 预期结果：
  1. 第 1 个 403 / 4030。
  2. 第 2 个 200。
  3. 第 3 个 403（自右向左取第一个不可信地址 192.168.1.10）。

### TC-CON-01-03 可信代理终止 TLS 时缺 X-Forwarded-Proto 被拒（负向）
- 关联 AC：AC-CON-01-2
- 优先级：P1
- 层级：集成
- 前置条件：同上，`CON_ENV=production` 的测试配置（证书由代理终止）。
- 步骤：
  1. 经代理请求登录页，不带 `X-Forwarded-Proto`。
  2. 带 `X-Forwarded-Proto: https`。
- 预期结果：
  1. 第一次 403 / 4030；第二次 200 且带 HSTS。

### TC-CON-01-04 管理空间的 Cookie 在运维监听地址上无效（跨空间，负向）
- 关联 AC：AC-CON-01-3
- 优先级：P0
- 层级：集成
- 前置条件：`CON_SPACES=admin,ops`；kf1 在 `CON_ADMIN_ADDR` 登录得到 Cookie 值 X。
- 步骤：
  1. 把 X 以 `__Host-adm_sid=X` 和 `__Host-ops_sid=X` 两种名称分别发到 `CON_OPS_ADDR` 的 `GET /console/api/session`。
- 预期结果：
  1. 都 401 / 4014，`reason=none`。

### TC-CON-01-05 生产启动保护：五种不合格配置都拒绝启动
- 关联 AC：AC-CON-01-4
- 优先级：P0
- 层级：进程级
- 前置条件：`CON_ENV=production`，其余配置合格；逐项破坏：① `CON_TEST_MODE=1`；② `CON_ADMIN_ALLOWED_CIDRS` 未配置；③ 含 `0.0.0.0/0`（另测 `::/0`）；④ 未配置 `CON_AUDIT_ANCHOR`；⑤ 数据库账号对审计表有 UPDATE 权限（用 `console_owner` 连接运行）。
- 步骤：
  1. 分别启动 `console-server serve`。
- 预期结果：
  1. 五种都非 0 退出，日志指出具体原因。

### TC-CON-01-06 审计只追加的启动自检：UPDATE 探针必须失败
- 关联 AC：AC-CON-01-4
- 优先级：P0
- 层级：进程级
- 前置条件：`CON_ENV=production`；准备一个被误授予 `UPDATE ON audit.probe` 的 `console_app` 库。
- 步骤：
  1. 启动。
  2. 撤销该权限后再启动。
- 预期结果：
  1. 第一次非 0 退出，日志指出"审计表可被更新"。
  2. 第二次启动成功（探针 UPDATE 失败被视为通过）。

### TC-CON-01-07 多项问题一次全部列出
- 关联 AC：AC-CON-01-4
- 优先级：P1
- 层级：进程级
- 前置条件：同时破坏 ①、②、④，并让 `CON_ADMIN_ORIGIN` 为 `http://`。
- 步骤：
  1. 启动。
- 预期结果：
  1. 非 0 退出，日志一次列出 4 条问题（不是只报第一条）。

### TC-CON-01-08 两个实例共享会话：登录与退出跨实例生效
- 关联 AC：AC-CON-01-5
- 优先级：P0
- 层级：集成
- 前置条件：实例 A、B 连同一个库。
- 步骤：
  1. kf1 在 A 登录。
  2. 用该 Cookie 请求 B 的 `GET /console/api/session`。
  3. 在 B 退出后用同一 Cookie 请求 A。
- 预期结果：
  1. B 返回 200。
  2. A 返回 401 / 4014（`reason=loggedOut`）。

### TC-CON-01-09 只开启 admin 空间时运维监听地址不存在
- 关联 AC：AC-CON-01-6
- 优先级：P1
- 层级：进程级
- 前置条件：`CON_SPACES=admin`，`CON_OPS_ADDR` 有值。
- 步骤：
  1. 启动后连接 `CON_OPS_ADDR`。
  2. 访问管理后台登录页。
- 预期结果：
  1. 运维地址连接被拒绝。
  2. 管理后台 200。

---

## F-CON-02 账号空间与账号生命周期

### TC-CON-02-01 命令行引导第一个超管，已有超管时拒绝
- 关联 AC：AC-CON-02-1
- 优先级：P0
- 层级：进程级
- 前置条件：admin 空间没有任何账号。
- 步骤：
  1. 运行 `console-server account bootstrap --space admin --username frank`。
  2. 用输出的激活码激活 frank 并绑定 TOTP。
  3. 再次运行同样命令。
- 预期结果：
  1. 第 1 次退出码 0，输出 20 位激活码；审计新增 `action=account.bootstrap`、`actor=cli:{OS 用户}@{主机名}`。
  2. 第 3 步退出码非 0，未创建账号。

### TC-CON-02-02 引导命令不能跨空间绕过：ops 空间独立判断
- 关联 AC：AC-CON-02-1
- 优先级：P1
- 层级：进程级
- 前置条件：admin 空间已有超管 frank；ops 空间无账号。
- 步骤：
  1. 运行 `bootstrap --space ops --username osa9`。
- 预期结果：
  1. 成功（空间独立）；网页上不存在注册入口（`POST /console/api/accounts` 未登录 401）。

### TC-CON-02-03 新建客服账号：激活、绑定、只见客服菜单、通知超管与审计员
- 关联 AC：AC-CON-02-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：a1 已登录并 step-up。
- 步骤：
  1. a1 在账号管理新建 `kf9`（角色 `support`），记下激活码。
  2. 新浏览器打开激活页，输入 kf9 + 激活码 + 合规新密码。
  3. 扫码绑定 TOTP 并输入动态码，确认恢复码已保存。
- 预期结果：
  1. 进入首页，侧边栏只有用户、消息元数据等客服权限内的菜单。
  2. a1、a2、aud1 的通知中心都有"kf9 已激活"。
  3. 激活码只在创建时显示一次，账号详情不再显示。

### TC-CON-02-04 激活码 24 小时后失效（[C 钟]）
- 关联 AC：AC-CON-02-3
- 优先级：P0
- 层级：集成
- 前置条件：[C 钟] T 时创建 kf2。
- 步骤：
  1. [C 钟] 设为 T + 24 小时 1 分钟。
  2. 用激活码激活。
- 预期结果：
  1. 401 / 4015，`msg` 与"账号或密码错误"相同；该失败计入 `kf2 + IP` 的失败计数。

### TC-CON-02-05 激活码错误按登录失败处理，且库中只存哈希
- 关联 AC：AC-CON-02-3
- 优先级：P1
- 层级：集成
- 前置条件：创建 kf3。
- 步骤：
  1. 用错误激活码激活一次。
  2. 全库搜索激活码原文。
- 预期结果：
  1. 401 / 4015。
  2. 搜不到原文（只有 pepper HMAC）。

### TC-CON-02-06 setup 会话不能调用业务接口（负向）
- 关联 AC：AC-CON-02-4
- 优先级：P0
- 层级：集成
- 前置条件：kf1 激活后尚未绑定 TOTP（`setup` 会话）。
- 步骤：
  1. 调用 `GET /console/api/users?q=u`、`GET /console/api/audit?…`、`POST /console/api/auth/step-up`。
  2. 调用 `POST /console/api/auth/totp/setup`。
- 预期结果：
  1. 前三个都 401 / 4016。
  2. 设置接口 200。

### TC-CON-02-07 setup 状态直接打开业务页面跳回绑定页
- 关联 AC：AC-CON-02-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同上，浏览器持有 setup 会话。
- 步骤：
  1. 地址栏打开 `/users`、`/groups`。
- 预期结果：
  1. 都跳到"绑定二次验证"页。

### TC-CON-02-08 管理空间超管看不到、改不了运维账号（跨空间 404，负向）
- 关联 AC：AC-CON-02-6
- 优先级：P0
- 层级：集成
- 前置条件：a1 已登录并 step-up；ops 空间有 ops1（id = O1）。
- 步骤：
  1. `GET /console/api/accounts?username=ops1`。
  2. `POST /console/api/accounts/O1/reset`。
  3. `POST /console/api/accounts/O1/disable`、`/roles`、`/unlock`。
  4. 查审计。
- 预期结果：
  1. 列表不含任何 ops 账号。
  2. 重置及其余接口都 404 / 4040，ops1 状态不变。
  3. 审计每次有一条 `space=admin`、`result=denied`、`code=4040`。

### TC-CON-02-09 运维超管同样不能操作管理空间账号（反向，负向）
- 关联 AC：AC-CON-02-6
- 优先级：P0
- 层级：集成
- 前置条件：osa1 已登录 ops 空间并 step-up；kf1 的 id = K1。
- 步骤：
  1. osa1 调用 `POST /console/api/accounts/K1/reset`（发往 `CON_OPS_ADDR`）。
- 预期结果：
  1. 404 / 4040，kf1 不变，审计 `space=ops`、`result=denied`。

### TC-CON-02-10 超管自我保护：不能改自己角色、停用或重置自己
- 关联 AC：AC-CON-02-7
- 优先级：P0
- 层级：集成
- 前置条件：admin 空间只有 a1 一个启用的超管，已 step-up。
- 步骤：
  1. a1 修改自己的角色为 `super_admin`+`operator`。
  2. a1 停用自己。
  3. a1 重置自己。
  4. 新建超管 a2 后，a1 再修改自己的角色。
- 预期结果：
  1. 四次都 409 / 4093，状态不变。

### TC-CON-02-11 两个超管并发互相停用，始终保留至少一个
- 关联 AC：AC-CON-02-8
- 优先级：P0
- 层级：集成
- 前置条件：只有 a1、a2 两个启用的超管，都已 step-up。
- 步骤：
  1. 并发发出 a1 停用 a2、a2 停用 a1；每轮结束恢复初始状态，重复 20 轮。
- 预期结果：
  1. 每轮恰好一个成功，另一个 409 / 4093 或 401 / 4014。
  2. 任何一轮结束时启用超管数 ≥ 1。

### TC-CON-02-12 重置后旧会话、旧密码 + TOTP、旧恢复码全部作废
- 关联 AC：AC-CON-02-9
- 优先级：P0
- 层级：集成
- 前置条件：kf1 有一个会话（Cookie C）和未用过的恢复码 R；a1 已 step-up。
- 步骤：
  1. a1 重置 kf1。
  2. 用 C 请求 session。
  3. 用原密码 + 当前动态码登录。
  4. 用原密码 + R 登录。
  5. 用新激活码激活。
- 预期结果：
  1. C → 401 / 4014（`reason=revoked`）。
  2. 两次登录都 401 / 4015。
  3. 新激活码可激活；a2、aud1 收到"kf1 被重置"通知，a1 不收到。

### TC-CON-02-13 账号名在空间内唯一
- 关联 AC：AC-CON-02-10
- 优先级：P0
- 层级：集成
- 前置条件：admin 空间已有 kf1。
- 步骤：
  1. a1 新建 `kf1`。
  2. osa1 在 ops 空间新建 `kf1`。
- 预期结果：
  1. 第一次 409 / 4092。
  2. 第二次成功。

### TC-CON-02-14 账号名格式与停用后的登录提示
- 关联 AC：AC-CON-02-10
- 优先级：P1
- 层级：集成
- 前置条件：a1 已 step-up。
- 步骤：
  1. 新建 `Kf9`（大写）、`k`（1 位）、33 位账号名。
  2. 停用 kf1 后用正确凭证登录。
- 预期结果：
  1. 三次都 400 / 4000。
  2. 登录 401 / 4015（不暴露停用状态）。

### TC-CON-02-15 账号不能删除
- 关联 AC：AC-CON-02-11
- 优先级：P0
- 层级：集成
- 前置条件：a1 已登录并 step-up。
- 步骤：
  1. 对 `/console/api/accounts/{kf1}` 发 `DELETE`。
- 预期结果：
  1. 405；kf1 仍存在。

---

## F-CON-03 登录

### TC-CON-03-01 正确凭证一步登录进入首页
- 关联 AC：AC-CON-03-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：kf1 已激活。
- 步骤：
  1. 登录页输入账号、密码、当前动态码提交。
- 预期结果：
  1. 200，进入首页；`Set-Cookie` 名为 `__Host-adm_sid`。

### TC-CON-03-02 6 种失败响应完全相同（防枚举）
- 关联 AC：AC-CON-03-2
- 优先级：P0
- 层级：集成
- 前置条件：准备：不存在账号、密码错、动态码错、恢复码错、已停用账号、待激活账号。
- 步骤：
  1. 各登录一次，记录状态、`code`、`msg`、响应头名称集合。
- 预期结果：
  1. 6 次完全相同（401 / 4015）。

### TC-CON-03-03 不存在账号与密码错误的响应时间一致
- 关联 AC：AC-CON-03-3
- 优先级：P1
- 层级：集成
- 前置条件：隔离测试环境；`CON_TEST_DISABLE_LOCKOUT=1` 关闭锁定与 IP 限流。
- 步骤：
  1. "不存在账号"与"存在账号密码错"各登录 50 次，记录耗时。
  2. 运行单元测试：不存在账号的路径调用了一次 Argon2id。
- 预期结果：
  1. 两组中位数相差 ≤ 30ms。
  2. 单测通过。

### TC-CON-03-04 同一动态码不能重复使用（登录与 step-up，[C 钟]，负向）
- 关联 AC：AC-CON-03-4
- 优先级：P0
- 层级：集成
- 前置条件：[C 钟] 固定在步长 T 内；kf1 用码 X 登录成功。
- 步骤：
  1. 同一步长内用 X 再登录一次。
  2. 在第一次登录的会话中用 X 做 step-up。
- 预期结果：
  1. 两次都 401 / 4015。
  2. 两次都计入失败次数（再失败 3 次即锁定）。

### TC-CON-03-05 TOTP 允许前后各 1 个步长（[C 钟]）
- 关联 AC：AC-CON-03-5
- 优先级：P0
- 层级：集成
- 前置条件：kf1 未在当前时间附近成功使用过动态码。
- 步骤：
  1. 用例 A：用步长 T−1 的码登录。
  2. 用例 B（独立账号状态）：用 T+1 的码登录。
  3. 用例 C：用 T−2 的码登录。
- 预期结果：
  1. A、B 成功；C 401 / 4015。

### TC-CON-03-06 恢复码只能用一次，使用写审计并通知本人
- 关联 AC：AC-CON-03-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：kf1 有恢复码 R1。
- 步骤：
  1. 登录页切换为恢复码，用 R1 登录。
  2. 退出后再用 R1 登录。
- 预期结果：
  1. 第一次成功，审计有 `auth.recovery_code_used`，kf1 通知中心有"恢复码已使用"。
  2. 第二次 401 / 4015。

### TC-CON-03-07 登录更换会话 ID，防会话固定（负向）
- 关联 AC：AC-CON-03-7
- 优先级：P0
- 层级：集成
- 前置条件：浏览器预先设置 `__Host-adm_sid=X`。
- 步骤：
  1. kf1 登录成功。
  2. 用 X 请求 `GET /console/api/session`。
- 预期结果：
  1. 新 Cookie 值 ≠ X。
  2. X 请求 401 / 4014。

### TC-CON-03-08 日志、代理日志、审计中没有密码与动态码
- 关联 AC：AC-CON-03-8
- 优先级：P0
- 层级：安全扫描
- 前置条件：反向代理（测试 nginx）在 console-server 前；密码 `Pw-CANARY-7f3a91`、动态码 `123456`（错误）。
- 步骤：
  1. 登录失败一次。
  2. 扫描 console-server 日志、nginx 访问与错误日志、`audit` 全表全部字段。
- 预期结果：
  1. 找不到 `Pw-CANARY-7f3a91`；审计记录中没有动态码字段。

### TC-CON-03-09 重新生成恢复码需要 step-up
- 关联 AC：AC-CON-03-9
- 优先级：P0
- 层级：集成
- 前置条件：kf1 剩余 2 个恢复码，登录后未 step-up。
- 步骤：
  1. 登录响应检查提示字段。
  2. `POST /console/api/me/recovery-codes`。
  3. step-up 后再调用。
  4. 用旧恢复码登录。
- 预期结果：
  1. 登录响应提示恢复码不足。
  2. 未 step-up → 403 / 4038。
  3. step-up 后返回 10 个新码，审计 `auth.recovery_codes_regenerated`。
  4. 旧码 401 / 4015。

---

## F-CON-04 密码策略、锁定与限流

### TC-CON-04-01 密码策略的 4 种拒绝原因与合规密码
- 关联 AC：AC-CON-04-1
- 优先级：P0
- 层级：集成
- 前置条件：kf1 已登录；最近 5 次密码历史已知。
- 步骤：
  1. 依次修改为：`abc12345678`（11 位）、`password1234`、`xkf1yzzzzzzzz`、最近 5 次之一、16 位随机串；另测 129 位。
- 预期结果：
  1. 前四个 400 / 4000，`reason` 依次 `tooShort`、`weak`、`containsUsername`、`reused`；129 位 `tooLong`。
  2. 16 位随机串成功，其他会话被删除、当前会话保留。

### TC-CON-04-02 弱密码表固定版本且哈希一致
- 关联 AC：AC-CON-04-1
- 优先级：P2
- 层级：后端单测
- 前置条件：仓库 `assets/weak-passwords.txt` 与记录的 SHA-256。
- 步骤：
  1. 测试计算文件 SHA-256。
- 预期结果：
  1. 与记录值一致（防止被悄悄替换）。

### TC-CON-04-03 密码用 Argon2id 加盐哈希
- 关联 AC：AC-CON-04-2
- 优先级：P0
- 层级：集成
- 前置条件：两个账号设置相同密码。
- 步骤：
  1. 读 `auth.accounts.password_hash`。
- 预期结果：
  1. 都以 `$argon2id$` 开头，两个值不同。

### TC-CON-04-04 账号名 + IP 连续失败 5 次后锁定，正确凭证也被拒（[C 钟]）
- 关联 AC：AC-CON-04-3
- 优先级：P0
- 层级：集成
- 前置条件：kf1；IP-A、IP-B（测试替身设置对端地址）。
- 步骤：
  1. 从 IP-A 连续 5 次密码错误。
  2. 第 6 次用正确密码 + 正确动态码从 IP-A 登录。
  3. 从 IP-B 用正确凭证登录。
  4. [C 钟] +15 分钟后从 IP-A 登录。
- 预期结果：
  1. 第 6 次 429 / 4291。
  2. IP-B 成功。
  3. 15 分钟后 IP-A 成功。
  4. kf1 和 a1、a2 收到锁定通知。

### TC-CON-04-05 不存在的账号同样锁定、响应相同（防枚举，负向）
- 关联 AC：AC-CON-04-4
- 优先级：P0
- 层级：集成
- 前置条件：`ghost` 不存在。
- 步骤：
  1. 从 IP-A 对 ghost 连续 5 次失败。
  2. 第 6 次登录，与 TC-CON-04-04 第 6 次响应逐字节比较（除 `Date`）。
- 预期结果：
  1. 429 / 4291，响应完全相同；没有人收到通知。

### TC-CON-04-06 成功登录清零计数
- 关联 AC：AC-CON-04-5
- 优先级：P1
- 层级：集成
- 前置条件：kf1、IP-A。
- 步骤：
  1. 失败 4 次，成功 1 次，再失败 4 次。
- 预期结果：
  1. 第二轮第 4 次仍是 401 / 4015，没有锁定。

### TC-CON-04-07 账号名全局 1 小时 50 次锁定（[C 钟]）
- 关联 AC：AC-CON-04-6
- 优先级：P0
- 层级：集成
- 前置条件：kf1；11 个不同 IP。
- 步骤：
  1. 1 小时内从 11 个 IP 各失败不超过 5 次，合计 50 次。
  2. 第 51 次从新 IP 用正确凭证登录。
  3. [C 钟] +1 小时后再登录。
- 预期结果：
  1. 第 51 次 429 / 4291。
  2. 1 小时后成功。

### TC-CON-04-08 IP 限流：每分钟 20 次 login + activate
- 关联 AC：AC-CON-04-7
- 优先级：P0
- 层级：集成
- 前置条件：同一 IP；不同账号名避免触发锁定。
- 步骤：
  1. 1 分钟内发 20 次 `login`/`activate`（混合），再发第 21 次。
- 预期结果：
  1. 第 21 次 429 / 4290（与锁定的 4291 区分）。

### TC-CON-04-09 失败计数在多实例间一致
- 关联 AC：AC-CON-04-8
- 优先级：P0
- 层级：集成
- 前置条件：实例 A、B 连同一个库。
- 步骤：
  1. kf1 从同一 IP 在 A 失败 3 次、在 B 失败 2 次。
  2. 第 6 次分别发到 A 和 B。
- 预期结果：
  1. 两个实例都 429 / 4291。

### TC-CON-04-10 解锁需要 step-up，写审计，被锁者可立即登录
- 关联 AC：AC-CON-04-9
- 优先级：P0
- 层级：集成
- 前置条件：kf1 被锁定；a1 已登录未 step-up。
- 步骤：
  1. a1 调用 `POST /console/api/accounts/{kf1}/unlock`。
  2. a1 step-up 后再调用。
  3. kf1 用正确凭证登录。
- 预期结果：
  1. 第一次 403 / 4038。
  2. 第二次成功，审计 `account.unlock`。
  3. kf1 登录成功；kf1 与超管的通知中心都有锁定时的通知。

### TC-CON-04-11 命令行应急解锁写审计
- 关联 AC：AC-CON-04-9
- 优先级：P1
- 层级：进程级
- 前置条件：osa1 被锁定。
- 步骤：
  1. 运行 `console-server account unlock --space ops --username osa1`。
- 预期结果：
  1. 退出码 0；osa1 可登录；审计 `action=account.unlock`、`actor=cli:{OS 用户}@{主机名}`。

---

## F-CON-05 会话与 CSRF

### TC-CON-05-01 会话 Cookie 属性
- 关联 AC：AC-CON-05-1
- 优先级：P0
- 层级：集成
- 前置条件：kf1。
- 步骤：
  1. 登录并检查 `Set-Cookie`。
- 预期结果：
  1. 名称 `__Host-adm_sid`，含 `HttpOnly`、`Secure`、`SameSite=Strict`、`Path=/`，无 `Domain`。

### TC-CON-05-02 页面脚本和 Storage 中找不到会话 ID
- 关联 AC：AC-CON-05-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：已登录页面，浏览器拿到 Cookie 值 X（测试从 CDP 读取）。
- 步骤：
  1. 执行 `document.cookie`；遍历 localStorage、sessionStorage、IndexedDB。
- 预期结果：
  1. 都找不到 X。

### TC-CON-05-03 空闲 30 分钟过期（[C 钟]）
- 关联 AC：AC-CON-05-3
- 优先级：P0
- 层级：集成
- 前置条件：最后一次普通请求在 T。
- 步骤：
  1. T+29 分钟发普通请求。
  2. 再过 31 分钟（T+60）发普通请求。
- 预期结果：
  1. 第一次 200。
  2. 第二次 401 / 4014，`reason=idle`。

### TC-CON-05-04 绝对 12 小时过期（[C 钟]）
- 关联 AC：AC-CON-05-4
- 优先级：P0
- 层级：集成
- 前置条件：登录于 T，之后每 10 分钟一次普通请求。
- 步骤：
  1. T + 12 小时 1 分钟请求。
- 预期结果：
  1. 401 / 4014，`reason=absolute`。

### TC-CON-05-05 后台请求不续期（[C 钟]，负向）
- 关联 AC：AC-CON-05-5
- 优先级：P0
- 层级：集成
- 前置条件：登录于 T，之后只有带 `X-Console-Background: 1` 的请求，每 60 秒一次。
- 步骤：
  1. T + 31 分钟发普通请求。
- 预期结果：
  1. 后台请求在 30 分钟内都 200；第 31 分钟普通请求 401 / 4014，`reason=idle`。

### TC-CON-05-06 退出后重放 Cookie 无效
- 关联 AC：AC-CON-05-6
- 优先级：P0
- 层级：集成
- 前置条件：kf1 已登录，Cookie C。
- 步骤：
  1. 退出。
  2. 用 C 请求 session。
- 预期结果：
  1. 401 / 4014，`reason=loggedOut`。

### TC-CON-05-07 CSRF：缺令牌时写操作不执行（负向）
- 关联 AC：AC-CON-05-7
- 优先级：P0
- 层级：集成
- 前置条件：op1 已登录。
- 步骤：
  1. `POST /console/api/users/import`（合法请求体）不带 `X-CSRF-Token`。
- 预期结果：
  1. 403 / 4037；im-server 未收到导入调用；审计表无新增（只增加 `console_rejected_total{reason="csrf"}`）。

### TC-CON-05-08 CSRF：错误令牌（负向）
- 关联 AC：AC-CON-05-8
- 优先级：P0
- 层级：集成
- 前置条件：同上。
- 步骤：
  1. 带另一会话的令牌、带随机串各发一次。
- 预期结果：
  1. 都 403 / 4037，导入未执行。

### TC-CON-05-09 CSRF：Origin 不匹配（负向）
- 关联 AC：AC-CON-05-9
- 优先级：P0
- 层级：集成
- 前置条件：同上，令牌正确。
- 步骤：
  1. `Origin: https://evil.example`。
  2. 不带 `Origin`。
  3. `Origin` 为运维空间的 Origin。
- 预期结果：
  1. 三次都 403 / 4037。

### TC-CON-05-10 GET 请求没有副作用（负向）
- 关联 AC：AC-CON-05-9
- 优先级：P1
- 层级：后端单测
- 前置条件：路由表。
- 步骤：
  1. 单测遍历所有 GET 路由，断言 handler 只注入只读仓储接口；对每个 GET 路由发请求后比较 `auth` / `biz` 表快照（`audit.query` 事件除外）。
- 预期结果：
  1. 除审计事件外无数据变化。

### TC-CON-05-11 刷新页面后先取 CSRF 令牌再提交
- 关联 AC：AC-CON-05-10
- 优先级：P0
- 层级：UI 自动化
- 前置条件：kf1 已登录，u1 在线。
- 步骤：
  1. 刷新浏览器后立即在用户详情提交一次强制下线。
- 预期结果：
  1. 网络请求顺序为先 `GET /console/api/session`、再强制下线；强制下线成功；页面没有出现 4037 提示。

### TC-CON-05-12 每账号最多 3 个会话，第 4 个挤掉最早的
- 关联 AC：AC-CON-05-11
- 优先级：P0
- 层级：集成
- 前置条件：kf1。
- 步骤：
  1. 依次登录 4 次得到 S1～S4。
  2. 用 S1 请求；用 S2～S4 请求。
- 预期结果：
  1. S1 → 401 / 4014，`reason=replaced`。
  2. S2～S4 都 200。

### TC-CON-05-13 被挤掉后页面提示并跳登录
- 关联 AC：AC-CON-05-12
- 优先级：P0
- 层级：UI 自动化
- 前置条件：浏览器 1 持有 kf1 的最早会话，另外登录 3 次。
- 步骤：
  1. 在浏览器 1 点任意菜单。
- 预期结果：
  1. 跳到登录页并显示"你的账号在别处登录，本次登录已退出"。

### TC-CON-05-14 角色变化使该账号全部会话失效
- 关联 AC：AC-CON-05-13
- 优先级：P0
- 层级：集成
- 前置条件：kf1 有两个会话；a1 已 step-up。
- 步骤：
  1. a1 把 kf1 的角色从 `support` 改为 `operator`。
  2. 两个会话各请求一次。
  3. kf1 重新登录。
- 预期结果：
  1. 两个会话都 401 / 4014，`reason=revoked`。
  2. 新登录的 `perms` 为运营权限（含 `user.import`，不含 `user.kick`）。

### TC-CON-05-15 接口响应带 Cache-Control: no-store
- 关联 AC：AC-CON-05-14
- 优先级：P0
- 层级：集成
- 前置条件：路由表。
- 步骤：
  1. 对每个 `/console/api/*` 路由（含错误响应 401 / 403 / 404）检查响应头。
- 预期结果：
  1. 都含 `Cache-Control: no-store`。

### TC-CON-05-16 库里只存会话 ID 的 SHA-256
- 关联 AC：AC-CON-05-15
- 优先级：P0
- 层级：集成
- 前置条件：kf1 登录，Cookie 值 X。
- 步骤：
  1. 查 `auth.sessions`，全库搜索 X。
- 预期结果：
  1. 只有 `SHA-256(X)`；搜不到 X。

---

## F-CON-06 角色与授权

### TC-CON-06-01 未声明权限点的路由启动失败（负向）
- 关联 AC：AC-CON-06-1
- 优先级：P0
- 层级：后端单测
- 前置条件：测试构建中注册一个未声明权限点的 `/console/api/debug`。
- 步骤：
  1. 运行测试并启动。
- 预期结果：
  1. 启动失败，信息含 "route without permission"。

### TC-CON-06-02 叠加角色取权限并集
- 关联 AC：AC-CON-06-2
- 优先级：P0
- 层级：端到端
- 前置条件：账号 so1 同时有 `support`、`operator`；u1 在线。
- 步骤：
  1. so1 强制下线 u1（support 权限）。
  2. so1 导入用户（operator 权限）。
- 预期结果：
  1. 都成功。

### TC-CON-06-03 无权限返回 403 并写 denied 审计
- 关联 AC：AC-CON-06-3
- 优先级：P0
- 层级：集成
- 前置条件：kf1（只有 `support`）。
- 步骤：
  1. 调用用户导入。
- 预期结果：
  1. 403 / 4030；审计新增 `result=denied`、`action=user.import`、`actor=kf1`。

### TC-CON-06-04 1 分钟内超过 20 次拒绝合并为一条
- 关联 AC：AC-CON-06-4
- 优先级：P1
- 层级：集成
- 前置条件：kf1；[C 钟] 固定在某分钟内。
- 步骤：
  1. 30 次无权限请求（用户导入 20 次、审计查询 10 次）。
  2. [C 钟] 推进到下一分钟并发一个请求。
  3. 查审计。
- 预期结果：
  1. 前 20 次各一条；其余合并为 1 条，`count=10`，含动作分布。

### TC-CON-06-05 未认证的拒绝不进审计（防刷爆）
- 关联 AC：AC-CON-06-5
- 优先级：P0
- 层级：集成
- 前置条件：无会话。
- 步骤：
  1. 发 100 次 `GET /console/api/users`。
- 预期结果：
  1. 都 401 / 4014；审计表无新增；`console_rejected_total{reason="noSession"}` 增加 100。

### TC-CON-06-06 互斥角色 auditor 不能与其他角色叠加（负向）
- 关联 AC：AC-CON-06-6
- 优先级：P0
- 层级：集成
- 前置条件：kf1 有 `support`；aud1 有 `auditor`；a1 已 step-up。
- 步骤：
  1. a1 给 kf1 再授予 `auditor`。
  2. a1 给 aud1 再授予 `support`。
  3. a1 新建账号时同时选 `auditor`、`operator`。
- 预期结果：
  1. 三次都 409 / 4093，角色不变。

### TC-CON-06-07 应急授权：申请、生效、到期自动撤销（[C 钟]）
- 关联 AC：AC-CON-06-7
- 优先级：P0
- 层级：集成
- 前置条件：a1 已 step-up；[C 钟] T。
- 步骤：
  1. a1 为自己申请 30 分钟 `moderator`，理由"处理举报 T-1"。
  2. T+10 分钟调用 moderator 才有的接口（解散群）。
  3. T+31 分钟再调用。
- 预期结果：
  1. 申请成功，审计 `account.emergency_grant`；aud1 和 a2 收到通知。
  2. T+10 成功。
  3. T+31 → 403 / 4030（无定时任务，按当前 [C 钟] 判断）。

### TC-CON-06-08 应急授权的非法申请（负向）
- 关联 AC：AC-CON-06-8
- 优先级：P0
- 层级：集成
- 前置条件：a1 已 step-up。
- 步骤：
  1. 申请 `auditor`。
  2. 申请 `super_admin`。
  3. 申请 300 分钟（另测 14 分钟）。
  4. 不带理由。
- 预期结果：
  1. 全部 400 / 4000，角色不变。

### TC-CON-06-09 应急授权只在管理空间提供（负向）
- 关联 AC：AC-CON-06-7
- 优先级：P0
- 层级：集成
- 前置条件：osa1 已登录 ops 空间并 step-up。
- 步骤：
  1. 向 `CON_OPS_ADDR` 调用应急授权接口申请 `ops_engineer`。
- 预期结果：
  1. 403 / 4030（ops 空间未注册 `admin.emergency`）；osa1 角色不变。

### TC-CON-06-10 应急授权只能给自己，未 step-up 被拒（负向）
- 关联 AC：AC-CON-06-8
- 优先级：P1
- 层级：集成
- 前置条件：a1 未 step-up；a1 已 step-up 两种状态。
- 步骤：
  1. 未 step-up 申请。
  2. step-up 后在请求体中指定 `accountId=kf1`。
- 预期结果：
  1. 第一次 403 / 4038。
  2. 第二次 400 / 4000（不接受为他人申请），kf1 角色不变。

### TC-CON-06-11 授予敏感角色通知审计员
- 关联 AC：AC-CON-06-9
- 优先级：P0
- 层级：集成
- 前置条件：a1 已 step-up。
- 步骤：
  1. a1 给 kf1 授予 `moderator`。
  2. a1 给 kf1 授予 `operator`（非敏感）。
- 预期结果：
  1. aud1 收到"kf1 被授予 moderator"；授予 operator 时没有通知。

### TC-CON-06-12 运维空间互斥角色
- 关联 AC：AC-CON-06-10
- 优先级：P0
- 层级：集成
- 前置条件：ops1 已有 `ops_engineer`；osa1 已 step-up。
- 步骤：
  1. osa1 给 ops1 再授予 `ops_superadmin`。
  2. 再授予 `ops_auditor`。
- 预期结果：
  1. 都 409 / 4093，角色不变。

---

## F-CON-07 重新验证（step-up）

### TC-CON-07-01 未 step-up 调用高危操作返回 403 / 4038（不是 401）
- 关联 AC：AC-CON-07-1
- 优先级：P0
- 层级：端到端
- 前置条件：op1 从未 step-up；群 g1 存在。
- 步骤：
  1. 解散 g1（带理由和确认群 ID）。
- 预期结果：
  1. 403 / 4038；g1 未解散（im-server 无调用）；会话仍有效（再请求 session 200）。

### TC-CON-07-02 step-up 5 分钟窗口（[C 钟]）
- 关联 AC：AC-CON-07-2
- 优先级：P0
- 层级：端到端
- 前置条件：op1 在 T step-up 成功；g1、g2 存在。
- 步骤：
  1. T+4 分钟解散 g1。
  2. T+6 分钟解散 g2。
- 预期结果：
  1. g1 成功且不需要输入动态码。
  2. g2 403 / 4038。

### TC-CON-07-03 step-up 窗口绑定会话，不跨会话继承（负向）
- 关联 AC：AC-CON-07-2
- 优先级：P1
- 层级：集成
- 前置条件：op1 有会话 S1（刚 step-up）和 S2（未 step-up）。
- 步骤：
  1. 用 S2 解散 g1。
- 预期结果：
  1. 403 / 4038。

### TC-CON-07-04 未 step-up 点解散：弹 TOTP 框，通过后自动完成
- 关联 AC：AC-CON-07-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：op1 未 step-up。
- 步骤：
  1. 群详情点"解散群"，填理由、输入群 ID 确认。
  2. 在弹出的 TOTP 框输入正确动态码。
- 预期结果：
  1. 弹框出现，不跳登录页。
  2. 通过后自动完成解散，不需要再点一次（网络面板中原请求被重放一次）。

### TC-CON-07-05 正文授权不享受 5 分钟窗口
- 关联 AC：AC-CON-07-4
- 优先级：P0
- 层级：集成
- 前置条件：mod1 已 step-up 1 分钟（第二批，`IM_V13_ADMIN_FEATURES=1`）。
- 步骤：
  1. 申请 g1 正文授权，请求体不带 `otp`。
- 预期结果：
  1. 403 / 4038；im-server 无授权记录。

### TC-CON-07-06 连续 5 次 step-up 失败注销会话并锁定账号
- 关联 AC：AC-CON-07-5
- 优先级：P0
- 层级：集成
- 前置条件：kf1 有效会话。
- 步骤：
  1. 连续 5 次 step-up 输错。
  2. 该会话再请求一次。
  3. 从另一个 IP 用正确凭证登录。
- 预期结果：
  1. 前 4 次 401 / 4015；第 5 次后该会话 401 / 4014，`reason=revoked`。
  2. 登录 429 / 4291（对所有 IP 锁 15 分钟）；kf1 与超管收到通知。

### TC-CON-07-07 step-up 输错只在弹框内提示
- 关联 AC：AC-CON-07-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：TOTP 弹框已出现。
- 步骤：
  1. 输入错误动态码。
- 预期结果：
  1. 弹框内提示"动态码错误"，仍停留在当前页面，原操作未执行。

---

## F-CON-08 审计记录

### TC-CON-08-01 强制下线的审计 intent / result
- 关联 AC：AC-CON-08-1
- 优先级：P0
- 层级：端到端
- 前置条件：kf1 已登录；u1 在线；测试机 IP 已知。
- 步骤：
  1. kf1 在页面对 u1 强制下线，理由"工单 T-100 处理"。
  2. aud1 查审计。
- 预期结果：
  1. intent、result 两条，`requestId` 相同；`actor=kf1`、`action=user.kick`、`target=user:u1`、`reason` 一致、result `success`、IP 为测试机 IP、`roles=[support]`。

### TC-CON-08-02 im-server 业务错误记为 failure 并透传
- 关联 AC：AC-CON-08-2
- 优先级：P0
- 层级：端到端
- 前置条件：kf1。
- 步骤：
  1. 强制下线 `nobody`。
- 预期结果：
  1. 响应 404 / 4042，页面提示"用户不存在"；result 记录 `result=failure`、`code=4042`。

### TC-CON-08-03 运行账号不能修改或删除审计记录（负向）
- 关联 AC：AC-CON-08-3
- 优先级：P0
- 层级：集成
- 前置条件：用 `console_app` 连接数据库。
- 步骤：
  1. 对 `audit.records` 执行 `UPDATE`、`DELETE`、`TRUNCATE`。
  2. 对 `audit.chain_head` 执行 `DELETE`。
- 预期结果：
  1. 四条都报权限错误。

### TC-CON-08-04 intent 写失败时操作不执行、数据不返回（负向）
- 关联 AC：AC-CON-08-4
- 优先级：P0
- 层级：端到端
- 前置条件：注入 `auditWrite=fail`；op1 已 step-up；kf1 有 `user.sessions`。
- 步骤：
  1. op1 解散 g1。
  2. kf1 查询 u1 在线设备。
- 预期结果：
  1. 都 503 / 5031。
  2. g1 未解散（im-server `admin_audit` 无该请求）；在线设备数据未返回。

### TC-CON-08-05 删除中间记录被校验发现
- 关联 AC：AC-CON-08-5
- 优先级：P0
- 层级：集成
- 前置条件：1000 条审计记录。
- 步骤：
  1. 用库超级用户删除 seq 500。
  2. 运行 `console-server audit verify`。
- 预期结果：
  1. 非 0 退出，输出 `gap` 和 seq 500。

### TC-CON-08-06 截断尾部：锚定之后通过、锚定之前被发现
- 关联 AC：AC-CON-08-6
- 优先级：P0
- 层级：集成
- 前置条件：1000 条；最近一次锚定在 seq 900。
- 步骤：
  1. 删除 seq 950～1000 后 verify。
  2. 再删除 seq 850～1000 后 verify。
- 预期结果：
  1. 第一次通过（已接受风险）。
  2. 第二次非 0 退出，输出 `truncated`。

### TC-CON-08-07 改记录并整链重算被锚定发现
- 关联 AC：AC-CON-08-7
- 优先级：P0
- 层级：集成
- 前置条件：1000 条；锚定点 seq 500、1000。
- 步骤：
  1. 超级用户修改第 10 条 `reason` 并从第 10 条起重算全部 hash。
  2. verify。
- 预期结果：
  1. 非 0 退出，输出 `anchorMismatch` 和 seq 500。

### TC-CON-08-08 只改一条不重算被发现
- 关联 AC：AC-CON-08-7
- 优先级：P1
- 层级：集成
- 前置条件：1000 条。
- 步骤：
  1. 修改第 10 条 `reason`，不重算。
  2. verify。
- 预期结果：
  1. 非 0 退出，输出 `hashMismatch` 和 seq 10。

### TC-CON-08-09 两实例并发写审计链不分叉
- 关联 AC：AC-CON-08-8
- 优先级：P0
- 层级：集成
- 前置条件：实例 A、B。
- 步骤：
  1. 各并发写 500 条。
  2. verify。
- 预期结果：
  1. 通过；seq 1～1000 连续无重复，每条 `prevHash` 等于上一条 `hash`。

### TC-CON-08-10 result 写失败：操作生效、显示结果未知、对账补齐
- 关联 AC：AC-CON-08-9
- 优先级：P0
- 层级：端到端
- 前置条件：注入 `auditWrite=failResult`；u1 在线。
- 步骤：
  1. kf1 强制下线 u1。
  2. 审计页查看该条。
  3. [C 钟] 推进 15 分钟让对账运行。
- 预期结果：
  1. u1 被踢，接口返回成功；`console_audit_result_failures_total` +1。
  2. 只有 intent，显示"结果未知"。
  3. 新增 `phase=result`、`result=success`、`params.reconciled=true`。

### TC-CON-08-11 调用 im-server 超时：504 / 5040、unknown、对账补齐
- 关联 AC：AC-CON-08-10
- 优先级：P0
- 层级：端到端
- 前置条件：注入 `imLatencyMs=15000`；u1 在线。
- 步骤：
  1. kf1 强制下线 u1。
  2. 对账运行后查审计。
- 预期结果：
  1. 响应 504 / 5040；console 没有重试（im-server 只收到 1 次请求）。
  2. result 先为 `unknown`，对账后补为 `success`。

### TC-CON-08-12 每类动作都有字段齐全的审计（参数化）
- 关联 AC：AC-CON-08-11
- 优先级：P0
- 层级：端到端
- 前置条件：action 表：管理后台第一批全部写操作和敏感查看、认证事件、命令行操作、已认证拒绝。
- 步骤：
  1. 逐类执行一次。
  2. 检查对应审计记录的 `space`、`phase`、`requestId`、操作人、IP、UA、`action`、`target`、`params`、`result`、`code`、`prevHash`、`hash`。
- 预期结果：
  1. 每类都有记录且字段齐全；`params` 中没有密码、动态码、恢复码、激活码、会话 ID、CSRF 令牌、正文、备注。

### TC-CON-08-13 整点锚定写入 WORM 桶且不可删除
- 关联 AC：AC-CON-08-12
- 优先级：P0
- 层级：集成
- 前置条件：`CON_AUDIT_ANCHOR` 指向测试 MinIO（Object Lock）。
- 步骤：
  1. [C 钟] 推进 1 小时。
  2. 读取桶中新增对象，比较库中同 seq 的 hash。
  3. 用 console-server 凭证删除该对象。
- 预期结果：
  1. 新增一条锚定对象，seq 与 hash 一致，`audit.anchors` 同步。
  2. 删除返回权限错误。

### TC-CON-08-14 每 1000 条触发一次锚定；锚定失败重试并告警
- 关联 AC：AC-CON-08-12
- 优先级：P1
- 层级：集成
- 前置条件：锚定存储可切换为不可用。
- 步骤：
  1. 连续写 1000 条。
  2. 让存储不可用后再写 1000 条，10 分钟内恢复。
- 预期结果：
  1. 写满 1000 条时有一次锚定。
  2. 失败时告警且 a1、aud1 收到"锚定失败"通知；恢复后 10 分钟内补写成功。

---

## F-CON-09 审计查看、对账、保留与归档

### TC-CON-09-01 审计员按条件筛选，32 天不可查询
- 关联 AC：AC-CON-09-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：aud1 已登录；有 kf1 的强制下线和其他记录。
- 步骤：
  1. 筛选"操作人 = kf1、动作 = 强制下线、最近 7 天"查询。
  2. 时间范围选 32 天。
- 预期结果：
  1. 只出现符合条件的记录。
  2. "查询"按钮不可点，提示"最长 31 天"。

### TC-CON-09-02 审计查询接口拒绝 32 天范围（负向）
- 关联 AC：AC-CON-09-2
- 优先级：P0
- 层级：集成
- 前置条件：aud1。
- 步骤：
  1. 时间范围 32 天调用。
  2. 不带时间范围调用。
- 预期结果：
  1. 都 400 / 4000。

### TC-CON-09-03 审计查询本身被审计
- 关联 AC：AC-CON-09-3
- 优先级：P0
- 层级：集成
- 前置条件：aud1。
- 步骤：
  1. 查询一次（带筛选条件）。
  2. 再查审计。
- 预期结果：
  1. 新增 `action=audit.query`，`params` 含筛选条件。

### TC-CON-09-04 管理空间审计员不能看运维空间（负向）
- 关联 AC：AC-CON-09-4
- 优先级：P0
- 层级：集成
- 前置条件：aud1。
- 步骤：
  1. 查询时指定 `space=ops`。
  2. 查询时指定 `space=all`。
- 预期结果：
  1. 都 403 / 4030。

### TC-CON-09-05 审计接口只读
- 关联 AC：AC-CON-09-5
- 优先级：P0
- 层级：集成
- 前置条件：aud1；某条审计 id。
- 步骤：
  1. 对审计接口发 `PATCH`、`DELETE`、`PUT`。
- 预期结果：
  1. 都 405；记录不变。

### TC-CON-09-06 对账：im-server 也没有该请求时补写 notExecuted（[C 钟]）
- 关联 AC：AC-CON-09-6
- 优先级：P0
- 层级：集成
- 前置条件：测试注入一条 intent 无 result，im-server 侧没有该 `requestId`。
- 步骤：
  1. [C 钟] 推进 15 分钟。
- 预期结果：
  1. 新增 `result=failure`、`code=0`、`params.reconciled="notExecuted"`。

### TC-CON-09-07 对账只在一个实例运行
- 关联 AC：AC-CON-09-6
- 优先级：P2
- 层级：集成
- 前置条件：两个实例；同一条待对账 intent。
- 步骤：
  1. 两实例同时到达对账周期。
- 预期结果：
  1. 只补写一条 result。

### TC-CON-09-08 满 1 年的分区加密归档后删除，链校验仍通过（[C 钟]）
- 关联 AC：AC-CON-09-7
- 优先级：P0
- 层级：集成
- 前置条件：13 个月的测试审计数据；归档桶（Object Lock）与测试 age 密钥对。
- 步骤：
  1. 运行归档任务。
  2. 用测试私钥解密归档文件，比较条数与哈希。
  3. 运行 `audit verify`。
- 预期结果：
  1. 最早 1 个月分区被导出、加密、写入归档桶，之后被删除；审计新增 `audit.archive`。
  2. 条数和哈希与库中一致。
  3. verify 通过（从归档末条接续）。

### TC-CON-09-09 运行账号不能删除审计分区（负向）
- 关联 AC：AC-CON-09-8
- 优先级：P0
- 层级：集成
- 前置条件：`console_app` 连接。
- 步骤：
  1. `DROP TABLE` 一个审计分区。
- 预期结果：
  1. 权限错误。

### TC-CON-09-10 运维审计员可查看管理空间记录
- 关联 AC：AC-CON-09-9
- 优先级：P0
- 层级：端到端
- 前置条件：kf1 在管理后台执行一次强制下线；oaud1 已登录 ops 空间。
- 步骤：
  1. oaud1 选择"全部空间"查询。
  2. aud1 在管理空间查询同一时段。
- 预期结果：
  1. oaud1 能看到该记录（`space=admin`）。
  2. aud1 看不到任何 `space=ops` 的记录。

---

## F-CON-10 浏览器侧安全

### TC-CON-10-01 构建产物与 HAR 中没有任何 IM 凭证或签名私钥
- 关联 AC：AC-CON-10-1
- 优先级：P0
- 层级：安全扫描
- 前置条件：测试环境使用可识别的测试密钥；tianting-admin `dist/`；一次完整流程（登录、搜索用户、强制下线、解散群、查审计）的 HAR。
- 步骤：
  1. 搜索 IM 调用方 HMAC 密钥原文、`CON_IM_SIGNING_KEY` 私钥（原文与 base64）、`X-Admin-Key`、`X-IM-Signature`。
- 预期结果：
  1. 结果都为 0。

### TC-CON-10-02 console 接口响应和错误信息不泄露签名头
- 关联 AC：AC-CON-10-1
- 优先级：P1
- 层级：端到端
- 前置条件：注入 im-server 返回 401 / 4017（错误的签名私钥）。
- 步骤：
  1. kf1 查询用户。
- 预期结果：
  1. 响应体和 console 日志中没有 `X-IM-Signature`、私钥或待签名串。

### TC-CON-10-03 XSS：用户可控内容一律按纯文本显示（负向）
- 关联 AC：AC-CON-10-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：u1 昵称、群 g1 名称、g1 中发送者昵称都为 `<img src=x onerror=alert(1)>`；一条审计的理由也用该串。
- 步骤：
  1. 依次打开用户列表、用户详情、群详情、消息元数据页、审计详情。
- 预期结果：
  1. 原样显示字符串，没有对话框；浏览器网络面板中没有对 `x` 的请求。

### TC-CON-10-04 安全响应头
- 关联 AC：AC-CON-10-3
- 优先级：P0
- 层级：集成
- 前置条件：任一页面（`/`、`/login`）和任一接口；`CON_ENV=production` 测试配置。
- 步骤：
  1. 检查响应头。
- 预期结果：
  1. CSP 与 F-CON-10 规则 4 一致（`frame-ancestors 'none'`、`script-src 'self'` 不含 `unsafe-inline`、`style-src` 带每次不同的 nonce）；`X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`、HSTS `max-age=31536000` 都在。

### TC-CON-10-05 被其他域名 iframe 嵌入时浏览器拒绝加载
- 关联 AC：AC-CON-10-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：另一源的测试页 iframe 嵌入后台登录页。
- 步骤：
  1. 打开该测试页。
- 预期结果：
  1. iframe 内容被浏览器拒绝（控制台有 CSP `frame-ancestors` 报错）。

### TC-CON-10-06 不加载外部头像
- 关联 AC：AC-CON-10-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：u1 头像 URL 为 `https://tracker.example/a.png`。
- 步骤：
  1. 打开用户列表和 u1 详情。
- 预期结果：
  1. 显示首字母占位；网络面板中没有对 `tracker.example` 的请求。

### TC-CON-10-07 lint 阻止 dangerouslySetInnerHTML
- 关联 AC：AC-CON-10-6
- 优先级：P0
- 层级：后端单测
- 前置条件：在一个组件夹具中使用 `dangerouslySetInnerHTML`（另测 `el.innerHTML = …`）。
- 步骤：
  1. 运行 `npm run lint`。
- 预期结果：
  1. 失败并指出该行。

---

## F-CON-11 密钥材料与密码哈希

### TC-CON-11-01 TOTP 种子只以密文存储
- 关联 AC：AC-CON-11-1
- 优先级：P0
- 层级：集成
- 前置条件：kf1 绑定 TOTP 后，测试持有 Base32 种子原文。
- 步骤：
  1. 读 `auth.totp`；全库 dump 搜索种子原文。
- 预期结果：
  1. 只有密文、nonce 和 kid；搜不到原文。

### TC-CON-11-02 数据密钥轮换
- 关联 AC：AC-CON-11-2
- 优先级：P0
- 层级：集成
- 前置条件：全部账号用 k1 加密。
- 步骤：
  1. 新增 k2 并设为当前，运行 `console-server keys rotate`。
  2. 从配置删除 k1，重启，登录全部账号。
  3. 另一库：未运行 rotate 就删除 k1 并启动。
- 预期结果：
  1. 重启后全部能登录。
  2. 未 rotate 时启动失败并指出仍有 k1 加密的记录。

### TC-CON-11-03 生产环境不合格数据密钥拒绝启动（负向）
- 关联 AC：AC-CON-11-3
- 优先级：P0
- 层级：进程级
- 前置条件：`CON_ENV=production`。
- 步骤：
  1. 分别：`CON_DATA_KEYS` 中一把 16 字节；与 `CON_IM_SIGNING_KEY` 相同；`CON_DATA_ACTIVE_KID` 不存在。
- 预期结果：
  1. 都非 0 退出；日志不打印密钥内容。

### TC-CON-11-04 Argon2id 并发信号量防内存型拒绝服务
- 关联 AC：AC-CON-11-4
- 优先级：P1
- 层级：性能
- 前置条件：默认参数（m=64MiB, t=3, p=1，信号量 4）。
- 步骤：
  1. 并发 50 个登录请求，采样进程 RSS 和同时进行的哈希数（指标）。
- 预期结果：
  1. 同时进行的 Argon2id ≤ 4；RSS 增量 ≤ 300MB；排队超过 2 秒的请求返回 503 / 5030。

### TC-CON-11-05 私钥类配置不出现在日志和 /readyz
- 关联 AC：AC-CON-11-3
- 优先级：P1
- 层级：安全扫描
- 前置条件：测试环境的签名私钥、数据密钥、数据库密码。
- 步骤：
  1. 启动、访问 `/readyz` 详情、触发一次配置错误。
  2. 扫描日志和响应。
- 预期结果：
  1. 都搜不到私钥类配置的值。

---

## F-CON-12 站内安全通知

### TC-CON-12-01 锁定通知同时到达本人和超管
- 关联 AC：AC-CON-12-1
- 优先级：P0
- 层级：端到端
- 前置条件：kf1 被锁定（TC-CON-04-04 的步骤）。
- 步骤：
  1. a1 解锁 kf1 后 kf1 登录，打开通知中心。
  2. a1 打开通知中心。
- 预期结果：
  1. 两人都有"kf1 于 {时间} 因多次登录失败被锁定"。

### TC-CON-12-02 各类触发的接收人参数化
- 关联 AC：AC-CON-12-2
- 优先级：P0
- 层级：集成
- 前置条件：触发表：激活、重新绑定、被重置、被锁定、恢复码使用、剩余 ≤ 2、授予敏感角色、应急授权、校验失败、锚定失败。
- 步骤：
  1. 逐类触发一次，统计每个账号新增的通知。
  2. 扫描通知内容。
- 预期结果：
  1. 对应接收人各 1 条，非接收人没有（例如重置时被重置者以外的超管收到、执行者本人以外）；内容中搜不到激活码、动态码、恢复码。

### TC-CON-12-03 通知只发给启用账号、按空间隔离
- 关联 AC：AC-CON-12-2
- 优先级：P1
- 层级：集成
- 前置条件：a2 已停用；ops 空间有 oaud1。
- 步骤：
  1. 在 admin 空间授予敏感角色。
- 预期结果：
  1. a2 和 oaud1 都没有收到。

### TC-CON-12-04 未读数与已读
- 关联 AC：AC-CON-12-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：kf1 有 3 条未读通知。
- 步骤：
  1. 查看铃铛，打开通知中心。
- 预期结果：
  1. 铃铛显示 3，打开后变为 0；轮询请求带 `X-Console-Background: 1`。
