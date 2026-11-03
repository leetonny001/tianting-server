# 快速复审：PRD-admin v0.2、PRD-ops v0.2，及四份 v0.2 PRD 的一致性核对

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-03 |
| 复审对象 | [PRD-admin-v0.2](../prd/PRD-admin-v0.2.md)、[PRD-ops-v0.2](../prd/PRD-ops-v0.2.md)；一致性核对另含 [PRD-social-v0.2](../prd/PRD-social-v0.2.md)、[PRD-ui-v0.2](../prd/PRD-ui-v0.2.md) |
| 依据 | [REVIEW-admin-v0.1](REVIEW-admin-v0.1.md)（阻塞 7）、[REVIEW-ops-v0.1](REVIEW-ops-v0.1.md)（阻塞 6）、[DECISIONS-2026-11-03](DECISIONS-2026-11-03.md)（全部已拍板）、`protocol/errors.json`、`protocol/ERRORS.md`、`protocol/ADMIN.md`、[PRD-core-v1.2](../prd/PRD-core-v1.2.md)（冻结）、`server/internal/`（`app/app.go`、`gateway/gateway.go`、`protocol/errors.go`、`protocol/errors_test.go`） |
| 范围 | 只看阻塞项是否关闭、公共基座与运维需求是否一致、错误码总表、跨文档冲突、新发现的阻塞级问题。**不是**重新做全面评审 |

**结论先行：需要再改，然后可以冻结。** 13 个原阻塞项中 10 个已关闭、3 个部分关闭（R-ADM-07、R-OPS-01、R-OPS-02），没有未关闭的。剩下的问题集中在一处：**PRD-ops v0.2 第 4 节和 F-OPS-01／02／08／16 有几处仍按自己的写法，没有跟随公共基座（PRD-admin 第 4 节）**，另有几处跨文档的批次和细节不一致。全部是文字修改，第 6 节共 32 条，约 1 人日；改完后对照第 6 节核对 diff 即可冻结，不需要再做一轮复审。

---

## 1. 阻塞项是否关闭

### 1.1 管理后台（REVIEW-admin-v0.1，R-ADM-01～07）

| 编号 | 问题（摘要） | 状态 | 依据 |
|---|---|---|---|
| R-ADM-01 | 后台数据库没定，AC 默认 Postgres | **已关闭** | X-3 落实为 Postgres：`auth`／`audit`／`biz` 三个 schema，`console_app` 对审计表只有 INSERT + SELECT，启动时探测 UPDATE 必须失败（F-CON-01 规则 5、7、9；AC-CON-01-4、01-5、08-3）；纳入 ARCH 3.4 备份（规则 6） |
| R-ADM-02 | 4017 STEP_UP 用 HTTP 401，和"401 跳登录页"冲突 | **已关闭** | step-up 改为 403／4038（F-CON-07 规则 5），正文授权缺失或过期统一 403／4039；前端只在 4014 时跳登录页（2.3 节）；AC 改为"状态码 + 错误码 + 前端行为"（AC-CON-07-1～6、AC-ADM-08-4、08-5） |
| R-ADM-03 | 错误码不符合号段规则；后台码混进 SDK | **已关闭**（遗留 4 处细节见第 3 节） | 按号段重排：401x 都是 401，403x 都是 403，锁定改 4291（429），4050 取消；新增 `scope: "console"`，SDK 生成器跳过；4007 补 `http: 400` 和 admin 通道；和 errors.json、PRD-social 逐码核对过（10.1～10.3 节）。遗留：im-server 自己的错误码一致性测试也要跳过 console 码（E-05）、console 透传规则（E-04）、关闭码 1011 缺失（E-06）、PRD-ops 用错码（E-01～E-03） |
| R-ADM-04 | 旧 `X-Admin-Key` 和 `/ws` 在同一个对外端口，后台管控可以整体绕开 | **已关闭** | F-KEY-01～05：`/admin/*` 只注册在 `IM_ADMIN_ADDR` 上（AC-KEY-01-1，已核对 `app.go` 第 95～102 行确实是同一个 mux，需要改）；按调用方分密钥 + scope；console 主凭证不含 `token.issue` 和任何正文 scope（规则 4，AC-KEY-02-6）；正文 scope 只能放在独立的调用方上（AC-KEY-02-4）；旧密钥只能调 v1.2 的 9 个接口，生产默认关（F-KEY-05）；im-server 侧审计覆盖所有调用方（F-KEY-04）；"被攻破的后果"如实写进 13.2 节。小遗漏：端口互斥检查没覆盖 PRD-ops 新增的 `IM_OPS_ADDR`／`IM_PPROF_ADDR`（D-10） |
| R-ADM-05 | 查看消息正文的合法性前提没写成上线门槛 | **已关闭** | `IM_C2C_BODY_ACCESS` 默认 off，不能经任何接口打开（F-KEY-02 规则 3，AC-KEY-02-7）；第 12 节列出上线门槛（法务、宿主隐私政策、个人信息保护影响评估、操作规程）和 4 个法务问题 |
| R-ADM-06 | 推翻已定约束却没列为变更 | **已关闭** | 11.2 节 VA-01～15 逐条列出对 PRD-core 的变更（含 D1／第 7 节），11.3 节 ADMIN.md，11.4 节 ARCH S8；系统通知放第二批最后，AC 只作为草案。跨文档的批次不一致（解散群）见 D-02 |
| R-ADM-07 | 和 PRD-ops 共用身份与审计，但关键设计互相冲突 | **部分** | 第 4 节"公共基座"已写成两个后台的唯一规格，部署、审计、保留期、CSRF、错误码风格都统一了。但 **PRD-ops v0.2 仍有 11 处与基座不一致**（第 2 节 C-01～C-11）：角色模型、登录步骤、初始密码、锁定提示、跨空间访问的响应、自我保护错误码、重置粒度、命令行审计格式等。评审原话"在此之前两份 PRD 都不能冻结"，所以按第 6 节改完才算关闭 |

### 1.2 运维后台（REVIEW-ops-v0.1，R-OPS-01～06）

| 编号 | 问题（摘要） | 状态 | 依据 |
|---|---|---|---|
| R-OPS-01 | 节点运维接口用一把共用的静态 token，明文传输 | **部分** | 实质已改：每节点独立凭证、2 把轮换、请求签名覆盖目标节点 ID／body／请求 ID／操作人、非回环地址必须 TLS 并校验服务名、节点侧 `IM_OPS_ALLOWED_CIDRS`、只向静态凭证表里的地址发请求、节点访问日志对账（F-OPS-16、F-OPS-14 规则 5）。**子项 (f)"与 PRD-admin 统一为同一套签名机制"没做到**：PRD-ops 写的是每节点 HMAC 共享密钥、自定义的待签名串，基座（F-CON-01 规则 8、11.4 节）写的是"引用 F-KEY-03，每节点独立公钥"，两者的算法、待签名串、请求头、错误码都不一样，PRD-ops 却声称"同一套实现"（D-06） |
| R-OPS-02 | 共用账号库时，管理超管可以接管运维账号 | **部分** | 接管路径已堵：X-2 独立账号空间，基座 F-CON-02 规则 7 + AC-CON-02-6，PRD-ops B-OPS-01／02 + AC-OPS-01-13／14。**子项"角色模型统一"反向处理了**：PRD-ops 改成单角色，并且误称"与公共基座一致"，实际基座是可叠加角色（C-01）；另外两份 AC 对跨空间访问的预期结果不同（C-07） |
| R-OPS-03 | 审计归属和规格冲突，SQLite 没有 GRANT | **已关闭** | B-OPS-09、F-OPS-14、F-OPS-15 全部引用基座 F-CON-08／09（Postgres、哈希链 P0、写失败则操作失败、在线 1 年 + 归档 3 年、31 天、游标分页），运维只扩展 `action` 枚举 |
| R-OPS-04 | 部署形态与 PRD-admin 冲突 | **已关闭**（遗留 1 处文字不一致） | 统一为 console-server 一个进程、两个空间两个域名；`CON_` 前缀、`console-server` 命令、CSRF、测试模式都以基座为准；会话、探测结果、最后可见时间存 Postgres。遗留：Cookie 名，基座写 `__Host-con_sid`，PRD-ops 写 `__Host-ops_sid`／`__Host-adm_sid`（C-05，改一处即可） |
| R-OPS-05 | "请求备份"和只读凭证自相矛盾 | **已关闭** | 按 O-2 ① + ③ 删除"请求立即备份"，改为发布流程 `migrate up` 前自动备份；console-server 凭证只读 `manifests/`（F-OPS-12、13，AC-OPS-12-5） |
| R-OPS-06 | 握手阶段被拒不写断开记录 | **已关闭** | F-OPS-09 规则 3 的表：4401／4009／4010～4012／1011 都计入 `im_disconnects_total{phase="handshake"}` 和日志；签名有效的 4011／4012／1011 按 userId 写入断开记录；不可归属的显示汇总；原 AC-OPS-09-3 删除，改为 09-3～09-6。已核对 `gateway.go` 第 124、148～157、205 行：目前只有鉴权成功的连接写 `conn closed`，握手阶段内部错误用 1011（`StatusInternalError`），PRD 的描述与代码现状一致。1011 不在 errors.json 关闭码表里（E-06） |

**合计**：已关闭 10（R-ADM-01～06、R-OPS-03～06），部分 3（R-ADM-07、R-OPS-01、R-OPS-02），未关闭 0。

---

## 2. 公共基座一致性：B-OPS-01～15 逐条核对

核对对象：PRD-admin v0.2 第 4 节（F-CON-01～12，下称"基座"）对 PRD-ops v0.2 第 4 节（B-OPS-01～15）以及 F-OPS-01／02 的相关 AC。统一写法的原则：取安全性更好、并且与拍板一致的一方；PRD-ops 第 4 节自己也写了"以修改基座为准"，所以基座缺少的就补进基座，PRD-ops 与基座冲突的改 PRD-ops。

### 2.1 逐条结论

| 编号 | 运维侧需求 | 结论 | 说明 |
|---|---|---|---|
| B-OPS-01 | 独立账号空间、跨空间无权限 | **满足；AC 预期冲突** | 基座 F-CON-02 规则 7、F-CON-01 规则 2 满足。但跨空间操作账号时，AC-CON-02-6 期望 404／4040，AC-OPS-01-13 期望 403／4030 + 拒绝审计 → C-07 |
| B-OPS-02 | 生命周期只归运维超管；命令行写审计；自我保护 | **部分冲突** | 归属、命令行写审计、不能停用最后一个超管都满足（F-CON-02 规则 2、7、8）。冲突：命令行操作人格式和 action 名（C-10）；重置粒度（C-09）；自我保护的错误码（C-08） |
| B-OPS-03 | 登录 | **冲突** | 半登录两步提交 vs 基座一步提交（C-02）；初始密码 vs 激活码（C-03）；锁定与其他失败响应相同 vs 基座 429／4291（C-04）。TOTP 防重放、恢复码、按 IP 每分钟 20 次一致；响应时间基座要求更严（中位数差 ≤ 30ms，PRD-ops ≤ 100ms），以基座为准 |
| B-OPS-04 | 会话 | **满足；Cookie 名冲突** | 存储、属性、30 分钟／12 小时、换 ID、3 个并发都一致。Cookie 名不同（C-05） |
| B-OPS-05 | 后台请求不续期（`X-Console-Background`） | **部分** | 请求头规则一致（F-CON-05 规则 3、AC-CON-05-5）。基座缺 Grafana 认证代理的续期规则（只有页面导航续期）和"手动操作不带该头"（C-06） |
| B-OPS-06 | CSRF | **满足** | F-CON-05 规则 7、AC-CON-05-7～10 |
| B-OPS-07 | 重新验证 | **满足；重置粒度冲突** | 403／4038、5 分钟窗口、基座清单覆盖运维第一批的操作（F-CON-07 规则 2、5）。"重置他人密码／TOTP"分开写，与基座"一并重置"冲突（C-09） |
| B-OPS-08 | 角色模型：单角色 | **冲突** | 基座是可叠加角色（F-CON-06 规则 1）。PRD-ops 第 2 节、B-OPS-08、14.4 节都写"单角色，与基座一致"，与事实不符（C-01） |
| B-OPS-09 | 审计存储与规格 | **满足** | F-CON-01 规则 5、F-CON-08、F-CON-09 |
| B-OPS-10 | 跨空间审计可见性 | **满足** | F-CON-09 规则 1、AC-CON-09-4、09-9 与 B-OPS-10、AC-OPS-15-2／15-3 一致（见 2.3 节 C-12 的说明，不需要改） |
| B-OPS-11 | 网络、`X-Forwarded-*`、未认证拒绝不写审计 | **部分** | 网段按空间配置、只信任可信代理、未认证拒绝不写审计都满足（F-CON-01 规则 3、F-CON-06 规则 3）。基座缺"经代理时要求 `X-Forwarded-Proto=https`"；PRD-ops 的"越权请求按账号限流"，基座的做法是按账号每分钟 20 条后合并审计，等价于防刷爆，PRD-ops 改用基座说法即可（C-11） |
| B-OPS-12 | 测试模式 | **满足** | 2.4 节、F-CON-01 规则 9、10 |
| B-OPS-13 | 错误码由基座统一，运维不新增 | **满足；运维 AC 用错码** | 基座第 10 节已分配。PRD-ops 的 AC 用了 4000（应为 4093）、4290（每日配额应为 4292），节点接口 401 没写码（第 3 节 E-01～E-03） |
| B-OPS-14 | 站内通知可以发给某空间某角色的全部账号 | **部分** | F-CON-12 的触发清单是封闭的，没有"各空间追加触发、按空间 + 角色指定接收人"的通用能力；PRD-ops F-OPS-02 规则 5、F-OPS-08 规则 9 依赖它（C-11） |
| B-OPS-15 | 运维部分的生产启动保护 | **部分** | 基座 F-CON-01 规则 9 没有引用空间追加检查，也没有要求"一次列出全部问题"（AC-OPS-03-3 要求）（C-11） |

### 2.2 冲突与统一写法

| # | 冲突 | 基座（PRD-admin 第 4 节） | PRD-ops v0.2 | **统一后的写法** | 理由 |
|---|---|---|---|---|---|
| C-01 | 角色模型 | 可叠加角色；`auditor` 不能与其他角色叠加；空间超管可以给自己做应急授权（F-CON-06 规则 1、4、6） | 一个账号一个角色（第 2 节、B-OPS-08、F-OPS-02 规则 2、14.4 节） | **以基座为准，同一空间内可叠加，按空间补充互斥规则**：① 每个空间的 PRD 声明"互斥角色"，被声明的角色不能与同空间任何其他角色叠加（授予时 409／4093）。管理空间：`auditor`。运维空间：`ops_auditor`、`ops_superadmin`。② 每个路由声明所需**权限点**；运维空间的权限点由 PRD-ops 定义（如 `ops.overview.read`、`ops.troubleshoot`、`ops.audit.read`、`ops.account.manage`），角色 → 权限点矩阵即 F-OPS-02 规则 1 的表。③ **应急授权只在管理空间可用**；运维空间没有应急授权（见第 5 节 N-1） | 叠加 + 互斥既满足管理后台"客服 + 运营"一人兼任，又能强制"排障和审计分人""运维超管无数据权限"，安全性不低于单角色；评审 R-OPS-02、D-ADM-12 都建议可叠加；PRD-ops 选单角色的唯一理由是"与基座一致"，前提不成立 |
| C-02 | 登录步骤 | 一步提交 `{username, password, otp | recoveryCode}`，三项都对才发会话（F-CON-03 规则 1） | 密码通过后进入 5 分钟"半登录"，再单独校验 TOTP（B-OPS-03、AC-OPS-01-17） | **一步提交**（基座）。AC-OPS-01-17 改为"同一步长内用同一码第二次登录失败"，与 AC-CON-03-4 相同 | 半登录状态会单独暴露"密码正确"这一信号，可用于离线猜密码之后的验证；评审 R-ADM-28 已定一步提交，PRD-admin 17.1 节也写了"与 PRD-ops 一致" |
| C-03 | 新账号首次登录 | 不设初始密码，生成一次性激活码（24 小时、只存哈希），本人用激活码设密码，再绑 TOTP（F-CON-02 规则 3、4） | 系统随机生成初始密码、只显示一次，首次登录先改密码再绑 TOTP（B-OPS-03、AC-OPS-01-5、14.2 节 R-OPS-13） | **激活码**（基座）。AC-OPS-01-5 改为激活流程，未绑定 TOTP 时请求业务接口返回 401／4016 | 激活码有效期短、库里只存哈希、激活即通知超管和审计员，比"超管看过的初始密码"暴露面小 |
| C-04 | 锁定是否单独提示 | 锁定期间返回 429／4291，不管凭证对不对；**不存在的账号名同样会被锁、同样返回 4291**，所以不泄露账号是否存在（F-CON-04 规则 3，AC-CON-04-3、04-4） | 锁定和其他四种失败响应完全相同（B-OPS-03、AC-OPS-01-2、01-3） | **以基座为准**：失败（账号不存在、密码错、动态码错、恢复码错、已停用、待激活）统一 401／4015；锁定统一 429／4291，按"账号名 + IP"计数，与账号是否存在无关。PRD-ops 的 AC-OPS-01-2 去掉"已锁定"一项，AC-OPS-01-3 第 6 次的预期改为 429／4291，并补一句"对不存在的账号名连续失败 5 次后同样返回 429／4291" | PRD-ops 担心的是 v0.1"只有存在的账号才会锁"导致可以枚举，基座已经用"不存在的账号同样锁定"堵住；基座还多了按账号名的全局上限，防分布式猜密码。合法用户能看到"尝试次数过多，N 分钟后再试"，不会一直重试把自己锁更久 |
| C-05 | Cookie 名 | 两个空间都叫 `__Host-con_sid`（F-CON-05 规则 2） | `__Host-ops_sid`，管理空间 `__Host-adm_sid`（B-OPS-04、AC-OPS-01-6、01-14） | **按空间区分**：管理空间 `__Host-adm_sid`，运维空间 `__Host-ops_sid`；其余属性不变 | 两个空间本来就是不同主机名，换名不影响隔离，但能避免本地开发或误配成同一主机时互相覆盖，日志里也一眼可分；改基座 4 处即可 |
| C-06 | 后台请求不续期 | 只规定 `X-Console-Background: 1` 不续期（F-CON-05 规则 3） | 另外要求：经 Grafana 认证代理的请求只有页面导航（`Sec-Fetch-Mode: navigate`）续期；手动刷新不带该头（B-OPS-05、AC-OPS-01-16、17-8） | **补进基座** F-CON-05 规则 3：① 由用户操作发起的请求（点击、手动刷新、提交）不得带 `X-Console-Background`；② 经 console-server 认证代理转发到观测组件（运维空间的 Grafana）的请求，只有 `Sec-Fetch-Mode: navigate` 的请求续期空闲计时；代理 Cookie 绑定原会话，原会话失效即失效 | 不补的话 Grafana 面板的 30 秒自动刷新会让"空闲 30 分钟过期"失效，正是 R-OPS-11 的问题 |
| C-07 | 跨空间操作账号的响应 | 列表里看不到另一空间的账号；对另一空间账号 ID 调用重置返回 404／4040（AC-CON-02-6） | 返回 403／4030，并写 `space=admin`、`result=denied` 审计（AC-OPS-01-13） | **返回 404／4040**（不承认该账号 ID 存在），**同时写一条 `result=denied`、`code=4040` 的审计**；基座 F-CON-02 规则 7 和 AC-CON-02-6 补审计断言，AC-OPS-01-13 改为 404／4040 | 404 不泄露另一空间的账号是否存在；跨空间尝试本身是可疑行为，需要留痕 |
| C-08 | 自我保护的错误码 | 改自己角色、停用／重置自己、超管降到 0：409／4093（F-CON-02 规则 8） | 返回 4000（AC-OPS-01-10、01-11） | **409／4093**，审计 `result=failure`、`code=4093` | 4093 是基座为此专门分配的码 |
| C-09 | 重置的粒度 | "重置密码与二次验证"一并进行，账号回到待激活（F-CON-02 规则 6） | 重置密码、重置 TOTP 分开（B-OPS-02、B-OPS-07、AC-OPS-01-13） | **一并重置**（基座）。PRD-ops 把"重置密码、重置 TOTP"统一写成"重置（密码与二次验证一并重置）"，AC-OPS-01-13 的 6 个接口改为 5 个：重置、解锁、停用、修改角色、查询详情 | 只重置一项时，持有另一项的人仍可接管；一并重置 + 通知更安全 |
| C-10 | 命令行操作的审计格式 | `actor=cli:{操作系统用户}`，action `account.bootstrap`（F-CON-02 规则 2、F-CON-08 规则 2、AC-CON-02-1） | 操作人 `cli@主机名`，action `account.create`（B-OPS-02、AC-OPS-01-18） | **`actor=cli:{操作系统用户}@{主机名}`**，首个超管的 action 为 `account.bootstrap`，解锁为 `account.unlock` | 用户和主机都要能追溯（多实例部署时主机名必要），两份 AC 取并集 |
| C-11 | 站内通知、网络、启动保护、响应头（基座缺的通用能力） | F-CON-12 触发清单封闭；F-CON-01 规则 4 没写 `X-Forwarded-Proto`；规则 9 没有空间追加检查、没要求一次列出全部问题；F-CON-10 规则 4 只有一套 CSP（允许 `img-src data:`） | B-OPS-14、B-OPS-11、B-OPS-15、F-OPS-03 规则 4（更严的 CSP，不用 `data:`，加 `X-Frame-Options: DENY`） | **补进基座**：① F-CON-12 增加"各空间 PRD 可以追加触发条件，接收人按'空间 + 角色'指定"，通知接口支持发给某空间某角色的全部启用账号；② F-CON-01 规则 4 增加"由可信代理终止 TLS 时，要求 `X-Forwarded-Proto=https`，否则 403"；③ 规则 9 增加"各空间 PRD 追加的生产检查（PRD-ops B-OPS-15）一并执行；启动时一次列出全部不满足项"；④ F-CON-10 规则 4 增加"各空间可以在此基础上收紧、不能放宽；运维空间 `img-src 'self'`（不含 `data:`）、`style-src 'self'`，并加 `X-Frame-Options: DENY`" | 都是 PRD-ops 合理的安全要求，基座是唯一规格，应由基座承载 |

### 2.3 已核对、无需修改的点

| # | 核对点 | 结论 |
|---|---|---|
| C-12 | 运维审计员与管理审计员的可见范围 | **一致，保留现写法**：`ops_auditor` 可以看 `space=ops` 和 `space=admin`；管理空间 `auditor` 只能看 `space=admin`，指定 `space=ops` 返回 403／4030（F-CON-09 规则 1、AC-CON-09-4、09-9；B-OPS-10、AC-OPS-15-2、15-3）。理由：运维记录里有被排查用户和工单号，管理侧没有必要看；运维空间网段更窄、账号更少，由它做跨空间审计符合"运维账号不受管理后台控制"的拍板（X-2） |
| — | `X-Console-Background` 请求头本身 | 一致（F-CON-05 规则 3、AC-CON-05-5；3.2 节、AC-OPS-01-15、01-16）。缺的部分见 C-06 |
| — | 审计规格、测试模式、CSRF、会话时长、并发会话上限、按 IP 登录限流 | 一致 |

---

## 3. 错误码

### 3.1 总表（errors.json 现有 28 个 + 四份 PRD 新增 26 个）

"scope（定稿后）"列已经计入各 PRD 提出的扩展和本复审的建议（E-04）。HTTP 一列"—"表示 `null`。

**现有码（errors.json v1）**

| code | 名称 | 来源 | scope（定稿后） | HTTP | 变更 |
|---|---|---|---|---|---|
| 4000 | BAD_REQUEST | errors.json | ws / admin / console | 400 | admin 10.2 加 console |
| 4001 | UNKNOWN_CMD | errors.json | ws | — | UI V-UI-10 只改客户端文案 |
| 4002 | INVALID_FRAME | errors.json | ws | — | — |
| 4003 | MISSING_REQUEST_ID | errors.json | ws | — | — |
| 4004 | INVALID_ID | errors.json | ws / admin / console | 400 | admin 10.2 加 console；social 用于 `ug-` 前缀 |
| 4005 | TEXT_EMPTY | errors.json | ws | — | — |
| 4006 | TEXT_TOO_LONG | errors.json | ws | — | — |
| 4007 | INVALID_CONV_ID | errors.json | ws / admin / console | **400**（原为 —） | admin 10.2（R-ADM-03 ④） |
| 4008 | CLIENT_MSG_ID_CONFLICT | errors.json | ws | — | UI 只改文案 |
| 4009 | INVALID_PLATFORM | errors.json | ws | — | — |
| 4010～4012 | TOKEN_INVALID／EXPIRED／REVOKED | errors.json | ws | — | — |
| 4013 | ADMIN_KEY_INVALID | errors.json | admin | 401 | 只用于旧密钥（F-KEY-05） |
| 4030 | FORBIDDEN | errors.json | ws / admin / console | 403 | admin 加 console；`reason` 细分（如 `c2cBodyDisabled`） |
| 4031 | MUTED | errors.json | ws | — | admin 10.2：去掉"二期"、加 `details` |
| 4032 | NOT_CONV_PARTICIPANT | errors.json | ws | — | — |
| 4033 | NOT_GROUP_MEMBER | errors.json | ws | — | — |
| 4040 | NOT_FOUND | errors.json | ws / admin / console | 404 | admin 加 console |
| 4041 | NOT_IN_ROOM | errors.json | ws | — | — |
| 4042 | USER_NOT_FOUND | errors.json | ws / admin / console | 404 | admin 加 console |
| 4043 | GROUP_NOT_FOUND | errors.json | ws / admin / console | 404 | 含已解散（admin AD-05、social F-SOC-26） |
| 4090 | GROUP_EXISTS | errors.json | admin / console | 409 | 含已解散群 ID 不复用 |
| 4130 | GROUP_MEMBER_LIMIT_EXCEEDED | errors.json | ws / admin / console | 400 | social 加 ws、改通用说明；admin 加 console |
| 4131 | BATCH_LIMIT_EXCEEDED | errors.json | admin / console | 400 | admin 加 console |
| 4290 | RATE_LIMITED | errors.json | ws / admin / console | 429 | admin 加 console；PRD-ops 误用于每日配额（E-01） |
| 5000 | INTERNAL | errors.json | ws / admin / console | 500 | admin 加 console |
| 5030 | SERVICE_UNAVAILABLE | errors.json | ws / admin / console | 503 | admin 加 console |

**新增码**

| code | 名称 | 来源 PRD | scope | HTTP | 可重试 | 备注 |
|---|---|---|---|---|---|---|
| 4014 | CONSOLE_AUTH_REQUIRED | admin（基座） | console | 401 | 否 | 前端只在这个码时跳登录页 |
| 4015 | CONSOLE_CREDENTIALS_INVALID | admin（基座） | console | 401 | 否 | 含 step-up 输错 |
| 4016 | CONSOLE_SETUP_REQUIRED | admin（基座） | console | 401 | 否 | |
| 4017 | ADMIN_SIGNATURE_INVALID | admin（F-KEY-03） | admin | 401 | 否 | 节点运维接口签名失败也用它（E-03） |
| 4034 | USER_BANNED | admin | admin（说明里注明也用于 `/demo/login`） | 403 | 否 | 第二批（E-08） |
| 4035 | NOT_FRIEND | social | ws | 403 | 否 | |
| 4036 | REJECTED_BY_PEER | social | ws | 403 | 否 | |
| 4037 | CSRF_INVALID | admin（基座） | console | 403 | 否 | social 预留给 admin，按预留使用 |
| 4038 | STEP_UP_REQUIRED | admin（基座） | console | 403 | 否 | 同上 |
| 4039 | MSG_READ_GRANT_REQUIRED | admin | console | 403 | 否 | 第二批 |
| 4044 | MESSAGE_NOT_FOUND | admin | admin / console | 404 | 否 | 第二批 |
| 4045 | FRIEND_REQUEST_NOT_FOUND | social | ws | 404 | 否 | social 写的 HTTP 是 404，见 E-07 |
| 4046 | TARGET_NOT_GROUP_MEMBER | social | ws / admin / console | 404 | 否 | admin 增加"禁言对象不是成员""指定群主不是成员"用途 |
| 4091 | FRIEND_REQUEST_HANDLED | social | ws | 409 | 否 | |
| 4092 | CONSOLE_ACCOUNT_EXISTS | admin（基座） | console | 409 | 否 | |
| 4093 | CONSOLE_ACCOUNT_STATE_CONFLICT | admin（基座） | console | 409 | 否 | PRD-ops 应使用而用了 4000（E-02） |
| 4132 | FRIEND_LIMIT_EXCEEDED | social | ws / admin / console | 400 | 否 | 413x 沿用 4130 的 HTTP 400（E-07） |
| 4133 | PEER_FRIEND_LIMIT_EXCEEDED | social | ws | 400 | 否 | 同上 |
| 4134 | BLOCK_LIMIT_EXCEEDED | social | ws | 400 | 否 | 同上 |
| 4135 | JOINED_GROUP_LIMIT_EXCEEDED | social | ws | 400 | 否 | 同上 |
| 4136 | GROUP_ADMIN_LIMIT_EXCEEDED | social | ws | 400 | 否 | 同上 |
| 4137 | STRANGER_MSG_LIMIT | social | ws | 400 | 否 | v0.1 的 4037 改号 |
| 4291 | CONSOLE_LOGIN_LOCKED | admin（基座） | console | 429 | **否**（例外） | social 预留，按预留使用 |
| 4292 | QUOTA_EXCEEDED | social | ws / admin / console | 429 | **否**（例外） | admin 扩到正文授权、在线设备、单聊元数据的每日上限；PRD-ops 每日上限也应使用它（E-01） |
| 5031 | AUDIT_UNAVAILABLE | admin（基座） | console | 503 | 是 | |
| 5040 | UPSTREAM_RESULT_UNKNOWN | admin（基座） | console | 504 | **否** | 新区段 504x |

**关闭码与被踢原因**

| 项 | 来源 | 结论 |
|---|---|---|
| `kickReasons` 增加 `suspended`（关闭码 4012，带 `until`） | admin F-ADM-13、10.2 节 | 合规；PRD-ui F-UI-18 文案表缺这一行（D-08） |
| 关闭码 1011（握手阶段内部错误，代码里是 `StatusInternalError`） | PRD-ops F-OPS-09 规则 3；`gateway.go` 第 205 行 | errors.json `closeCodes` 里没有（E-06） |
| PRD-ui | — | 不新增错误码；用到的码都在上表内。AC-UI-19-3 要求 errors.json 里每个 ws 码都有文案，写入新码时同一 PR 补文案 |
| PRD-ops | — | 声明不新增错误码，核对属实 |

### 3.2 重复、冲突、不合号段规则的地方

**重复**：无。admin 的 14 个新码和 social 的 12 个新码互不重叠，也不和 errors.json 的 28 个重复；social 预留的 4037～4039、4291 由 admin 按预留用途使用。PRD-admin 10.3 节的号段占用表核对属实（401x 剩 4018、4019；403x 用满；404x 剩 4047～4049；409x 剩 4094～4099；413x 剩 4138、4139；429x 剩 4293～4299）。

**号段规则**：新码都满足"前三位与 HTTP 状态一致"，有两类既有的例外需要写进 ERRORS.md：413x 用 HTTP 400（沿用 4130、4131 的先例，ADMIN.md"HTTP 状态码由前三位决定"的说法不准确）；4291、4292、5040 不可自动重试（admin 10.4 第 1 条和 social 9.4 节已要求写明）。

**冲突和遗漏**：

| # | 问题 | 位置 | 改法 |
|---|---|---|---|
| E-01 | PRD-ops 每日 50 个 userId 上限返回 4290，基座同一个限制（F-ADM-14 规则 4、AC-ADM-14-4）返回 4292；4290 可自动重试，会让前端反复重试 | PRD-ops 3.2 节"错误"行、F-OPS-08 规则 5、AC-OPS-08-9 | 每日上限改 429／4292；每分钟上限保留 4290 |
| E-02 | PRD-ops 自我保护返回 4000 | AC-OPS-01-10、01-11 | 改 409／4093（同 C-08） |
| E-03 | PRD-ops 节点运维接口签名失败只写"返回 401"，没有错误码；并发超限只写"429" | F-OPS-16 规则 2、7，AC-OPS-16-1、16-2、16-3、16-6、16-7、16-8 | 签名失败 401／4017（与 F-KEY-03 规则 6 相同）；来源网段不符 403／4030；并发超限 429／4290 |
| E-04 | console-server 把 im-server 的码"原样透传"（2.3 节），但 10.2 节只给一部分码加了 `console`；4046、4132、4044 等透传码没有统一处理 | PRD-admin 10.2 节 | 加一条规则："console 通道可以透传任何 `admin` scope 的码；透传码不需要逐个加 `console` scope，只有 console-server 自己产生的码才写 `console`"。按这条规则，上表中现有码的 `console` 都可以不写；两种写法选一种，写进 ERRORS.md 第 2 节 |
| E-05 | im-server 的 `protocol/errors_test.go`（`TestCodesMatchErrorsJSON`）要求 errors.json 里每个码在 Go 里都有常量。写入只给 console 用的 4014～4016、4037～4039、4092、4093、4291、5031、5040 后，要么 im-server 定义一批自己永远不用的常量，要么测试失败；PRD-admin 只写了"SDK 生成器跳过"，没写 im-server 这边 | PRD-admin 10.4 节、VA-14 | 补："im-server 的错误码一致性测试同样跳过只含 `console` scope 的码；console-server 仓库有自己的一致性测试，只核对 `console` 码" |
| E-06 | 关闭码 1011 已在代码里使用（握手阶段查询吊销状态失败），PRD-ops 也把它列进断开记录，但 errors.json `closeCodes` 和 ERRORS.md 第 4 节没有 | PRD-admin 10.4 节（errors.json 变更由 admin 汇总） | errors.json `closeCodes` 增加 `{code: 1011, name: "INTERNAL_ERROR", reconnect: true, meaning: "服务端内部错误（如握手阶段查询吊销状态失败）"}`，ERRORS.md 第 4 节同步 |
| E-07 | 只在 ws 上使用的码，errors.json 现有惯例是 `http: null`（4031～4033、4041）；PRD-social 9.4 节给 4035、4036、4045、4091、4133～4137 都写了 HTTP 值。只增不改，写入后就不能再统一 | PRD-social 9.4 节表格 HTTP 列 | scope 只有 ws 的码 HTTP 写 `—`（null）：4035、4036、4045、4091、4133、4134、4135、4136、4137；有 admin scope 的 4046、4132、4292 保留 404／400／429 |
| E-08 | 4034 的 scope 只有 `admin`，但第二批 `/demo/login` 也返回它（F-ADM-13 规则 3、AC-ADM-13-7；PRD-ui F-UI-43），sdk-ts 要读 `details.bannedUntil` | PRD-admin 10.1 节 4034 行 | 说明列补"也用于 `/demo/login`（HTTP，与管理接口同一响应格式）"；scope 保持 `admin`（SDK 生成器只跳过 console，sdk-ts 能拿到这个码） |

---

## 4. 其他跨文档冲突

| # | 冲突 | 各文档的写法 | 统一后的写法 |
|---|---|---|---|
| D-01 | 后台对社交接口的权限 | PRD-social 9.3 节第 1045 行："后台服务（console）默认只有 `social.friends` 的查询"。PRD-admin F-KEY-02 规则 4：console 主凭证持有 `social.friends`（导入、删除、查询）和 `social.settings`（社交配置、官方账号、群主交接），供第一批 F-ADM-15～17、25 使用 | **以 PRD-admin 为准**（X-6 + S-3 已把社交管理放进第一批，S-11 要求等 X-5 并有审计，F-KEY 已满足）。PRD-social 9.3 节该行改写（第 6 节 S-1）。PRD-admin 11.4 节已经列了这一条，PRD-social 还没改 |
| D-02 | 解散群的批次 | PRD-social：F-SOC-26 整体标 M4（第 770 行），9.3 节 `DELETE /admin/groups/{id}` 标 M4，标题和第 12 行仍写"待 X-6 拍板"。PRD-admin：F-ADM-24、G-ADM-11、`convRemoved.reason` 都在**第一批**，VA-12 把"群聊已解散"文案定为第一批，11.4 节要求 PRD-ui"第一批先补'群聊已解散'"。PRD-ui：F-UI-33 规则 4 把 `dissolved` 的界面放 M4，M3 只处理 `reason` 缺失或 `removed` | **拆开交付**：① `DissolveGroup` 存储方法、`DELETE /admin/groups/{id}`、群 ID 不复用、`convRemoved.reason`（`removed`／`dissolved`）、sdk-ts 和 H5 的"群聊已解散"界面 → **M3**（与 PRD-admin 第一批 G-ADM-11 同一次交付）；② 客户端 `groupDissolve` 命令（只有自建群主能用，依赖 M4 的自建群）、最后一人退群自动解散、`reason: left` → **M4**。PRD-social、PRD-ui 按此修改（第 6 节 S-2、U-1） |
| D-03 | 在线连接的 IP 脱敏 | PRD-admin F-ADM-14 规则 2：IPv4 `a.b.*.*`，IPv6 保留前 4 组（64 位），并声称"与运维后台一致"。PRD-ops 3.2 节：IPv4 保留前 3 段（`10.8.1.*`），IPv6 保留前 48 位，"节点只保存脱敏后的值"。两份都说 im-server 只有一个查询函数 | **统一为更严的一档，并在 im-server 内完成脱敏**：IPv4 保留前 2 段（`a.b.*.*`），IPv6 保留前 3 组（48 位），其余为 `*`；`gateway.conn` 的连接元数据和断开缓冲**只保存脱敏后的值**，查询函数直接返回脱敏值，两个后台都拿不到完整 IP。查询函数的字段取并集：连接 ID、平台、建立时间、节点、脱敏 IP、下行队列状态；管理后台可以不显示队列状态 |
| D-04 | 用户级查询的理由、工单号和配额 | 基座 2.3 节：理由去首尾空白后 4～200 字符、不能由同一个字符重复组成（"两个后台统一"）；工单号 `CON_TICKET_PATTERN` 默认 `^[A-Za-z0-9_-]{3,32}$`；F-ADM-14 每日上限 4292。PRD-ops F-OPS-08 规则 1：工单号 1～32 位，说明（`note`）2～200 码点，并称"与 PRD-admin 一致"（14.2 节 R-OPS-44、14.5 节 X-07 也这么写）；每日上限 4290、按 Asia/Shanghai 自然日 | **以基座为准**：字段名统一为 `reason`（审计字段名就是 `reason`），4～200 码点、不能由同一字符重复组成；工单号用 `CON_TICKET_PATTERN`；每日上限 429／4292，"每天"按 Asia/Shanghai 自然日计算（写进基座 2.3 节，两个后台同用） |
| D-05 | 管理后台在线设备在多节点下怎么查 | PRD-admin：console-server 调 im-server 管理接口 `POST /admin/users/sessions/query`（经内网负载均衡到某一个节点），F-ADM-14 规则 5、AC-ADM-14-6 却要求"某节点未响应时标 `partial`"。PRD-ops：B6 之前由 console-server 扇出到全部静态节点。B6（Presence）之前，单个 im-server 节点看不到其他节点的连接，AC-ADM-14-6 无法执行 | 不阻塞冻结。PRD-admin F-ADM-14 规则 5 写明"B6 之前（没有 Presence）管理接口只返回接收请求的那个节点上的连接，并标 `partial: true`、`scope: "localNode"`；B6 之后由 im-server 经 Presence 汇总"；AC-ADM-14-6 标 `[依赖 B6]` |
| D-06 | 节点运维接口的签名方式 | PRD-admin F-CON-01 规则 8、11.4 节：节点接口"引用 F-KEY-03（每节点独立公钥）"。PRD-ops F-OPS-16 规则 1、2：每节点 HMAC 共享密钥（`IM_OPS_KEYS=kid:secret`），待签名串 = 方法 + 路径 + 目标节点 ID + 时间戳 + nonce + body 哈希 + 请求 ID + 操作人账号 ID，失败只写 401；同时声称"与 X-5 同一套签名算法和实现"。F-KEY-03 的待签名串没有目标节点 ID，字段顺序也不同 | **统一为 F-KEY-03 的实现 + 一行目标节点**：① 算法 Ed25519；console-server 为每个节点持有一把独立私钥（节点凭证表里是私钥文件引用），节点只配置自己那把的公钥 `IM_OPS_PUBKEYS=kid:base64`（最多 2 把，用于轮换）——满足拍板"每节点独立"，节点被攻破也拿不到可以签名的材料。② 请求头与 F-KEY-03 相同（`X-IM-Caller: console-ops`、`X-IM-Timestamp`、`X-IM-Nonce`、`X-IM-Signature`、`X-IM-Operator: ops:{accountId}`、`X-Request-Id`），另加 `X-IM-Target-Node`。③ 待签名串在 F-KEY-03 规则 2 的 7 行后加第 8 行 `X-IM-Target-Node`（管理接口为空串；运维接口必填，且必须等于本节点 `IM_NODE_ID`）。④ 签名失败 401／4017；来源网段不符 403／4030；并发超限 429／4290。⑤ 时间窗口 300 秒、nonce 10 分钟（节点本地内存即可） |
| D-07 | `console-msgread` 凭证与 X-5"后台凭证不能读正文"（PRD-admin Q-1） | X-5 原文："后台服务的凭证不能签发 token、不能读消息正文"。X-7：群消息按 ② 可以查看（专门角色 + 理由 + 重新验证 + 审计 + 水印）。PRD-admin：console 主凭证永远不能读正文；第二批另配 `console-msgread` 凭证（只有正文类 scope，不能有其他 scope），每次查看都要 im-server 侧授权（15 分钟、绑定会话 ID）、受 im-server 侧每日配额限制（每操作人 20、调用方 200）、写 im-server 侧审计；单聊受 `IM_C2C_BODY_ACCESS` 兜底 | **不算冲突，但需要 Frank 确认解释**。X-5 和 X-7 字面上有张力：console-server 进程仍持有一把能读正文的凭证。PRD-admin 的设计把"console 被攻破"的损失限定为"每天最多 200 个群会话的正文、且 im-server 侧全部留痕"，这是 X-5 想要的"有边界"的合理落实，比另起一个服务（多约 3 人日）性价比高。建议：Frank 在 DECISIONS 里把 Q-1 记为"按 PRD-admin 的理解通过"；第二批开工前确认即可，**不阻塞第一批冻结**。PRD-admin 现有写法（第 0 节、15 节、2.2 节）已足够，不需要改 |
| D-08 | 封禁后在线被踢的客户端提示 | PRD-admin F-ADM-13 规则 2：`kicked {reason: "suspended", until}`，关闭码 4012；AC-ADM-13-11 期望 H5 显示"账号已被封禁，解封时间：{时间}"。PRD-ui F-UI-18 文案表只有 `banned`→"账号已被下线"；F-UI-43 规则 2 写"原因按 PRD-admin 定稿"；AC-UI-43-3 只要求"弹出被踢弹窗" | PRD-ui F-UI-18 文案表增加一行 `4012 / suspended` → 有 `until` 时"账号已被封禁，解封时间：{YYYY-MM-DD HH:mm}"，`until = null` 时"账号已被封禁"（标"第二批"）；F-UI-43 规则 2 写死 `suspended`；AC-UI-43-3 补文案断言 |
| D-09 | 文档引用过期 | PRD-social 第 12 行"依赖（待拍板，默认按建议方案）"列出 X-5、X-6、U-4、X-8，实际都已拍板；F-SOC-26 标题"待 X-6 拍板"。PRD-ui 第 12 行背景文档引用 PRD-admin **v0.1**。PRD-admin 第 13 行引用 PRD-ops **v0.1**；4.0 节写运维超管"v0.1 为 `ops-admin`"，PRD-ops v0.2 已定为 `ops_superadmin` | 改为已拍板／v0.2／`ops_superadmin`（第 6 节） |
| D-10 | 内网端口互斥 | PRD-admin F-KEY-01 规则 4 只检查 `IM_ADMIN_ADDR ≠ IM_METRICS_ADDR`；PRD-ops 新增 `IM_OPS_ADDR`、`IM_PPROF_ADDR`，并要求"运维密钥不能调 `/admin/*`、管理密钥不能调运维接口" | F-KEY-01 规则 4 改为：`IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的地址两两不能相同，相同时拒绝启动 |

---

## 5. 新发现的问题

### 5.1 阻塞级

| # | 问题 | 位置 | 为什么是阻塞 | 改法 |
|---|---|---|---|---|
| **N-1** | **运维空间的应急授权会绕开"运维超管没有数据权限"和"排障与审计分人"** | 基座 F-CON-06 规则 6 × PRD-ops 第 2 节、F-OPS-02 | 基座允许空间超管"给自己临时加业务角色"（只排除审计员和空间超管），15～240 分钟。放到运维空间，`ops_superadmin` 可以随时给自己加 `ops_engineer`，查任意用户的在线连接和断开记录；而 PRD-ops 的前提是"运维超管本身没有任何运维数据权限"，第二批还要求授予 `ops_engineer` 需要第二个超管批准。两份 PRD 各自看都对，合在一起出现越权路径。这两份文档冻结后会直接按此实现 | 基座 F-CON-06 规则 6 改为"应急授权**只在管理空间**提供（权限点 `admin.emergency`）；运维空间不提供应急授权，运维超管要获得排障权限只能由另一名运维超管授予 `ops_engineer`（第二批起需第二人批准），且 `ops_superadmin` 与其他运维角色互斥（C-01）"；PRD-ops F-OPS-02 补一条 AC："`ops_superadmin` 调用应急授权接口返回 403／4030" |

### 5.2 重要（不阻塞冻结，建议一并改）

| # | 问题 | 改法 |
|---|---|---|
| N-2 | im-server 的错误码一致性测试会因 console 码失败（E-05） | 见第 6 节 A-12 |
| N-3 | 管理后台在线设备查询在 B6 之前不能跨节点（D-05） | 见第 6 节 A-14 |
| N-4 | PRD-ops 第 2 节、B-OPS-08、14.4 节都写"单角色与公共基座一致"，14.2 节 R-OPS-44 和 14.5 节 X-07 写"下限与 PRD-admin 一致为 2"——修订记录里对基座的描述与基座事实不符，说明 PRD-ops 修订时参照的是基座的草稿而不是定稿 | 第 6 节 O-1、O-7 一并改掉修订记录里的这些说法；以后基座变更按 PRD-admin 第 4 节开头的要求"同时通知两份 PRD 的作者" |

---

## 6. 冻结前必须做的修改清单

共 32 条：PRD-admin 15 条、PRD-ops 10 条、PRD-social 4 条、PRD-ui 3 条。每条写到可以直接照着改。

### 6.1 PRD-admin v0.2（`docs/prd/PRD-admin-v0.2.md`）

| # | 位置 | 改成什么 |
|---|---|---|
| A-1 | F-CON-06 规则 4（第 429 行） | 改为："**职责分离**：每个空间的 PRD 声明本空间的**互斥角色**，互斥角色不能与同空间任何其他角色叠加（授予时返回 409／4093）。管理空间：`auditor`；运维空间：`ops_auditor`、`ops_superadmin`（PRD-ops F-OPS-02）。空间超管默认**没有**正文查看、删除消息、全员推送、审计查看权限（第 5 节）。" AC-CON-06-6 不变；新增 AC-CON-06-10【后端】：输入 ops 空间账号已有 `ops_engineer`；操作 运维超管再授予 `ops_superadmin` 或 `ops_auditor`；预期 都 409，角色不变；错误码 4093 |
| A-2 | F-CON-06 规则 6（第 431 行）开头 | 在"**应急授权**："后插入"**只在管理空间提供**（权限点 `admin.emergency`），运维空间不提供。"；规则末尾加"运维空间的应急授权接口不注册（调用返回 403／4030）"。（N-1） |
| A-3 | F-CON-06 规则 2（第 427 行） | 末尾加："各空间的权限点由各自 PRD 定义（管理空间见第 5.2 节，运维空间见 PRD-ops F-OPS-02 规则 1）。" |
| A-4 | F-CON-05 规则 2（第 398 行）；AC-CON-03-1（第 361 行）、AC-CON-03-7（第 367 行）、AC-CON-05-1（第 407 行） | 规则 2 的 `__Host-con_sid` 改为"管理空间 `__Host-adm_sid`，运维空间 `__Host-ops_sid`"；三条 AC 中的 `__Host-con_sid` 改为 `__Host-adm_sid`（这三条在管理空间验证）。（C-05） |
| A-5 | F-CON-05 规则 3（第 399 行）末尾 | 加："由用户操作发起的请求（点击、手动刷新、提交表单）不得带 `X-Console-Background`。经 console-server 认证代理转发到观测组件的请求（运维空间的 Grafana，PRD-ops F-OPS-17），只有 `Sec-Fetch-Mode: navigate` 的请求刷新空闲计时；代理 Cookie 绑定原会话，原会话失效时立即失效。"（C-06） |
| A-6 | F-CON-02 规则 7（第 331 行）；AC-CON-02-6（第 342 行） | 规则 7 末尾加："对另一空间的账号 ID 调用任何账号接口一律返回 404／4040（不承认其存在），并写一条 `result=denied`、`code=4040` 的审计。" AC-CON-02-6 预期末尾加"审计新增一条 `space=admin`、`result=denied`、`code=4040` 的记录"。（C-07） |
| A-7 | F-CON-02 规则 2（第 326 行）、规则 6（第 379 行）、F-CON-08 规则 2（第 466 行） | `actor=cli:{操作系统用户}`、`actor=cli:…`、`命令行为 cli:{os 用户}` 统一改为 `cli:{操作系统用户}@{主机名}`。（C-10） |
| A-8 | F-CON-12 规则 2（第 549 行）末尾 | 加："各空间 PRD 可以追加触发条件（如 PRD-ops F-OPS-02 规则 5、F-OPS-08 规则 9），接收人按'空间 + 角色'指定；通知接口支持发给某空间某角色的全部启用账号。"（C-11 ①） |
| A-9 | F-CON-01 规则 4（第 306 行）末尾、规则 9（第 311 行）末尾 | 规则 4 加："由可信代理终止 TLS 时，请求必须带 `X-Forwarded-Proto: https`，否则 403／4030。" 规则 9 加："各空间 PRD 追加的生产检查（PRD-ops B-OPS-15）一并执行；不满足时**一次列出全部问题**后退出。"（C-11 ②③） |
| A-10 | F-CON-10 规则 4（第 518 行）末尾 | 加："各空间可以在此基础上收紧，不能放宽：运维空间 `img-src 'self'`（不含 `data:`）、`style-src 'self'`，并加 `X-Frame-Options: DENY`（PRD-ops F-OPS-03 规则 4）。"（C-11 ④） |
| A-11 | 2.3 节"理由"行（第 150 行）、F-ADM-14 规则 2、4（第 687、689 行） | 2.3 节"理由"行末尾加："运维后台排查接口的说明字段同样叫 `reason`、规则相同。"并新增一行"**每日上限**：'每天'按 Asia/Shanghai 自然日计算；超出返回 429／4292（两个后台统一）"。F-ADM-14 规则 2 的脱敏改为"IPv4 保留前 2 段（`a.b.*.*`），IPv6 保留前 3 组（48 位），其余为 `*`；**im-server 的连接元数据和断开缓冲只保存脱敏后的值**，查询函数返回字段：连接 ID、平台、建立时间、节点、脱敏 IP、下行队列状态"。（D-03、D-04） |
| A-12 | F-KEY-03 规则 1、2（第 236～237 行）；F-CON-01 规则 8（第 310 行）；11.4 节 PRD-ops 行 | 规则 1 请求头加"可选 `X-IM-Target-Node`（目标节点 ID）"；规则 2 待签名串末尾加第 8 行"`X-IM-Target-Node`（没有则空串）"，并加一句"节点运维接口（PRD-ops F-OPS-16）复用本规则：调用方 `console-ops`、Ed25519、每节点一把独立私钥，`X-IM-Target-Node` 必填且必须等于本节点 `IM_NODE_ID`，失败同样 401／4017"。F-CON-01 规则 8 第二句改为"运维节点接口按 F-KEY-03 签名（Ed25519，每节点一把独立私钥，节点只存公钥），见 PRD-ops F-OPS-16"。（D-06） |
| A-13 | F-KEY-01 规则 4（第 179 行） | 改为："`IM_ADMIN_ADDR`、`IM_OPS_ADDR`、`IM_METRICS_ADDR`、`IM_PPROF_ADDR` 中已启用的地址两两不能相同；相同时拒绝启动。"AC-KEY-01-5 增加"`IM_ADMIN_ADDR` 与 `IM_OPS_ADDR` 相同"一组。（D-10） |
| A-14 | 10.1 节 4034 行、10.2 节表尾、10.4 节 | 4034 说明列加"也用于 `/demo/login`"。10.2 节表尾加一行"**透传规则**：console 通道可以透传任何 `admin` scope 的码；只有 console-server 自己产生的码写 `scope: console`"（并相应删掉 10.2 第一行"scope 增加 console"的逐码写法，或保留但注明二者等价，二选一）。10.4 节加三条：⑤ "im-server 的 `TestCodesMatchErrorsJSON` 跳过只含 `console` scope 的码；console-server 仓库自己核对 console 码"；⑥ "`closeCodes` 增加 1011 `INTERNAL_ERROR`（重连），ERRORS.md 第 4 节同步"；⑦ "第 1 节注明 413x 使用 HTTP 400（沿用 4130、4131），ADMIN.md 开头'HTTP 状态码由错误码前三位决定'改为'取 errors.json 的 `http` 字段'"。（E-04～E-08） |
| A-15 | 第 13 行"背景材料"、4.0 节"空间超管"行（第 295 行）、F-ADM-14 规则 5 与 AC-ADM-14-6（第 690、698 行） | 背景材料中 `PRD-ops v0.1` 改为 `PRD-ops v0.2`；4.0 节"角色名由 PRD-ops 定义，v0.1 为 `ops-admin`"改为"`ops_superadmin`（PRD-ops F-OPS-02）"。F-ADM-14 规则 5 前加"B6 之前（没有 Presence）只返回接收请求的节点上的连接，响应标 `partial: true`；B6 之后经 Presence 汇总。"AC-ADM-14-6 的"（双节点）"改为"[依赖 B6]"。（D-05、D-09） |

### 6.2 PRD-ops v0.2（`docs/prd/PRD-ops-v0.2.md`）

| # | 位置 | 改成什么 |
|---|---|---|
| O-1 | 第 2 节第 72 行；B-OPS-08（第 162 行）；F-OPS-02 规则 2（第 222 行）；14.1 节 R-OPS-02 行、14.4 节 R-OPS-02 子项行 | 第 72 行改为："角色按公共基座**可以叠加**（同一空间内），但 `ops_superadmin`、`ops_auditor` 是**互斥角色**，不能与任何其他运维角色叠加（基座 F-CON-06 规则 4）：'排障'和'审计'必须是两个人，运维超管没有任何运维数据权限。`ops_engineer` 已包含 `ops_viewer` 的权限。运维空间**没有应急授权**。" B-OPS-08 改为："**角色模型**：同一空间内可叠加，按空间分域；运维空间的互斥角色为 `ops_superadmin`、`ops_auditor`；每个接口声明所需权限点，未声明的路由启动失败"，差异列改为"与基座一致（v0.1 可叠加、v0.2 草稿曾改单角色，复审后按基座）"。F-OPS-02 规则 1 的表头改为"权限点 × 角色"，每行补权限点名（`ops.overview.read`、`ops.config.read`、`ops.troubleshoot`、`ops.silence.write`、`ops.audit.read`、`ops.account.manage`、个人设置为"只需登录"）；规则 2 的"新建账号必须选择一个角色"改为"至少选择一个角色"，"声明所需角色"改为"声明所需权限点"。14.1、14.4 节 R-OPS-02 的"不采纳、改为单角色"改为"已改：按基座可叠加 + 互斥角色"。新增 AC-OPS-02-9【后端】：[E1] 操作：`ops_superadmin` 调用应急授权接口；给 `ops_engineer` 账号再授予 `ops_auditor`；预期：前者 403／4030；后者 409／4093。（C-01、N-1） |
| O-2 | B-OPS-03（第 157 行）；AC-OPS-01-2、01-3、01-5、01-17（第 189、190、192、204 行）；14.2 节 R-OPS-12、R-OPS-13 行 | B-OPS-03 改为："**登录**：以基座 F-CON-03、F-CON-04 为准——账号 + 密码 + 动态码（或恢复码）**一步提交**；TOTP ±1 步长、同一码不能重复使用；新账号用**一次性激活码**激活后绑定 TOTP、发放恢复码（不设初始密码）；账号不存在／密码错／动态码错／恢复码错／已停用／待激活统一 401／4015，响应时间中位数差 ≤ 30ms；锁定（按'账号名 + IP'15 分钟 5 次、按账号名 1 小时 50 次）统一 429／4291，**不存在的账号名同样锁定、同样返回 4291**；`ops_superadmin` 可解锁；同 IP 每分钟 20 次"。AC-OPS-01-2 删去"已锁定账号"，改为六种失败（同基座 AC-CON-03-2）、响应完全相同、错误码 4015。AC-OPS-01-3 第 6 次预期改为"返回 429／4291"，并加"对不存在的账号名 `ghost` 连续失败 5 次后第 6 次同样返回 429／4291，响应与 `eng1` 第 6 次完全相同"。AC-OPS-01-5 改为："`ops_superadmin` 新建账号 `view1`（得到一次性激活码）；`view1` 用激活码 + 新密码激活后、绑定 TOTP 之前请求 `GET /console/api/ops/v1/overview`；预期：返回 401／4016，不返回数据；页面跳到'绑定二次验证'页"。AC-OPS-01-17 改为："用某个 TOTP 码登录成功后，在同一 30 秒步长内用同一码再次登录；预期：401／4015"。14.2 节两行中的"半登录""首次改密""初始密码随机、只显示一次"改为"一步提交""激活码"。（C-02、C-03、C-04） |
| O-3 | B-OPS-02（第 156 行）；B-OPS-07（第 161 行）；AC-OPS-01-13（第 200 行）；AC-OPS-01-18（第 205 行） | B-OPS-02 中"重置密码、重置 TOTP"改为"重置（密码与二次验证一并重置，基座 F-CON-02 规则 6）"，"操作人记为 `cli@主机名`"改为"操作人记为 `cli:{操作系统用户}@{主机名}`"。B-OPS-07 中"重置他人密码／TOTP"改为"重置他人账号"。AC-OPS-01-13 改为："调用重置、解锁、停用、修改角色、查询账号详情 5 个接口；预期：全部返回 404／4040，`eng1` 状态不变，每次都产生一条 `space=admin`、`result=denied`、`code=4040` 的审计记录"。AC-OPS-01-18 的 `action=account.create`、`cli@<主机名>` 改为 `action=account.bootstrap`、`cli:<操作系统用户>@<主机名>`。（C-07、C-09、C-10） |
| O-4 | AC-OPS-01-10、01-11（第 197、198 行） | "返回 4000"全部改为"返回 409／4093"。（C-08、E-02） |
| O-5 | 3.2 节"个人信息"行（第 128 行）；AC-OPS-08-1（第 401 行） | "IPv4 保留前 3 段，如 `10.8.1.*`；IPv6 保留前 48 位"改为"IPv4 保留前 2 段（如 `10.8.*.*`），IPv6 保留前 3 组（48 位），与 PRD-admin F-ADM-14 相同，脱敏在 im-server 内完成"；AC-OPS-08-1 的 `127.0.0.*` 改为 `127.0.*.*`。（D-03） |
| O-6 | F-OPS-08 规则 1（第 387～391 行）；第 6 节 `/troubleshoot/query` 行；AC-OPS-01-9、AC-OPS-08-3、08-4、08-13；F-OPS-14 规则 2 表、规则 7；F-OPS-18 规则 1 | 请求体 `{userId, ticket, note}` 全文改为 `{userId, ticket, reason}`；`ticket` 规则改为"必填，格式由基座 `CON_TICKET_PATTERN` 决定（默认 `^[A-Za-z0-9_-]{3,32}$`）"；`note` 规则改为"`reason`：必填，去首尾空白后 4～200 个码点，不能由同一个字符重复组成（基座 2.3 节）"。AC-OPS-08-3 的输入加一组"`ticket` 为 `AB`（2 位）"，预期都 4000。AC-OPS-08-4 改为：输入 `reason` 为 `"  abc "`、`"aaaa"`、`"abcd"`、200 个汉字、201 个汉字、199 个汉字 + 1 个 emoji；预期依次 4000、4000、通过、通过、4000、通过。（D-04） |
| O-7 | 14.2 节 R-OPS-31、R-OPS-44 行；14.5 节 X-07 行 | "说明（2～200）""下限与 PRD-admin 一致为 2""2～200 码点"改为"理由 4～200 码点、不能由同一字符重复组成（以基座为准）"。（N-4） |
| O-8 | 3.2 节"错误"行（第 131 行）；F-OPS-08 规则 5（第 398 行）；AC-OPS-08-9（第 409 行） | "频率与配额 4290"改为"频率 4290；每日配额 4292（不可自动重试）"；规则 5 和 AC-OPS-08-9 中每日 50 个 userId 上限的"返回 4290"改为"返回 429／4292"（每分钟 30 次仍为 4290）。（E-01） |
| O-9 | F-OPS-16 规则 1、2、4、7（第 599～606 行）；7.3 节表格"密钥"列；AC-OPS-16-1、16-2、16-3、16-4、16-6、16-7、16-8、16-9；14.1 节 R-OPS-01 行；11.1 节 X-5 行 | 规则 1 改为："**每节点独立凭证**：console-server 为每个节点持有一把独立的 Ed25519 私钥（节点凭证表中为私钥文件引用）；节点配置自己那把的公钥 `IM_OPS_PUBKEYS=kid:base64`（最多 2 把同时有效，用于轮换）。一个节点的公钥只能验证发给它的请求。"规则 2 改为："**请求签名**：与 X-5 管理接口使用同一实现（PRD-admin F-KEY-03）：请求头 `X-IM-Caller: console-ops`、`X-IM-Timestamp`、`X-IM-Nonce`、`X-IM-Signature`、`X-IM-Operator: ops:{accountId}`、`X-Request-Id`、`X-IM-Target-Node`；待签名串为 F-KEY-03 规则 2 的 8 行（第 8 行 `X-IM-Target-Node` 必填，且必须等于本节点 `IM_NODE_ID`）。时间戳超出 300 秒、nonce 10 分钟内重复、目标节点不符、签名不符：**401／4017**，不返回任何数据。"规则 4 的"返回 403"改为"返回 403／4030"；规则 7 的"超出返回 429"改为"超出返回 429／4290"。7.3 节"密钥"列改为"私钥（文件引用）"。AC-OPS-16-1～16-3 的"返回 401"改为"返回 401／4017"；AC-OPS-16-4 的 ④⑤ 改为"④ `IM_OPS_PUBKEYS` 某项不是合法的 Ed25519 公钥；⑤ 生产环境未配置 `IM_NODE_ID`"；AC-OPS-16-6 中 `IM_OPS_KEYS=k1:…,k2:…` 改为 `IM_OPS_PUBKEYS=k1:…,k2:…`（console-server 从 k1 私钥切换到 k2 私钥）；AC-OPS-16-7 的"401（或 403）"改为"401／4017"；AC-OPS-16-8 的 429 写为 429／4290；AC-OPS-16-9 的 403 写为 403／4030。14.1、11.1 节中"HMAC""同一套 HMAC 签名"改为"F-KEY-03 同一实现（Ed25519，每节点一把私钥）"。（D-06、E-03） |
| O-10 | F-OPS-02 规则 5（第 225 行）；AC-OPS-15-7（第 591 行） | 规则 5 开头加"运维空间的敏感角色（基座 F-CON-06 规则 5）为 `ops_superadmin`、`ops_engineer`、`ops_auditor`："，并把"授予或变更为 `ops_engineer`、`ops_auditor` 时"改为"授予或变更为上述角色时"。AC-OPS-15-7 的"返回 404 或 405"改为"返回 405"（与基座 F-CON-09 规则 5 一致）。 |

### 6.3 PRD-social v0.2（`docs/prd/PRD-social-v0.2.md`）

| # | 位置 | 改成什么 |
|---|---|---|
| S-1 | 9.3 节"X-5 权限范围（scope）新增"行（第 1045 行） | "后台服务（console）默认只有 `social.friends` 的查询"改为"console-server 主凭证持有 `social.friends`（导入、删除、查询）和 `social.settings`（社交配置、官方账号、群主交接），供 PRD-admin 第一批 F-ADM-15～17、25 使用（PRD-admin F-KEY-02 规则 4）"。（D-01） |
| S-2 | F-SOC-26 标题和引言（第 770～772 行）；9.3 节 `DELETE /admin/groups/{groupId}` 行（第 1044 行）；9.5 节"解散群"行（第 1097 行）；3.1 节功能点总览中 F-SOC-26 的里程碑 | 标题改为"F-SOC-26 解散群（P0；存储方法、管理接口、`convRemoved.reason` 为 M3，客户端 `groupDissolve` 为 M4）"；引言改为"X-6 ②、S-10 已拍板：解散群提前。规则 2～6 中的 `DissolveGroup`、`DELETE /admin/groups/{id}`、群 ID 不复用、`convRemoved {reason: dissolved}` 在 **M3** 交付（与 PRD-admin 第一批 G-ADM-11 同一实现）；规则 1 的客户端 `groupDissolve`、最后一人退群自动解散在 M4"。AC-SOC-26-6 标【M3】，AC-SOC-26-1、26-3 标【M4】，26-2、26-4、26-5 两个里程碑都要验证。9.3 节该行里程碑改为 M3，说明去掉"是否提前……待 X-6 拍板"。9.5 节该行去掉"是否从第二批提前到第一批随 X-6（S-10），默认提前"，改为"已提前：服务端与 H5 提示在 M3，客户端命令在 M4"。（D-02） |
| S-3 | 第 12 行"依赖（待拍板，默认按建议方案）" | 行名改为"依赖（已拍板）"；内容改为"X-5 ②、X-6 ②（S-10：解散群提前）、U-4（做）、X-8 ②，2026-11-03 已按建议通过；错误码与 PRD-admin v0.2 第 10 节一起写入 errors.json"；全文搜索"待 X-5 拍板""待 X-6 拍板""待 U-4 拍板"，把"默认按建议方案"一并删去。（D-09） |
| S-4 | 9.4 节新增错误码表 | 只有 ws scope 的码 HTTP 列改为"—"：4035、4036、4045、4091、4133、4134、4135、4136、4137；4046 的通道改为"ws / admin"（不变）并注明"PRD-admin 增加禁言、指定群主用途"；4292 的通道改为"ws / admin"，注明"PRD-admin 用于正文授权、在线设备、单聊元数据和运维排查的每日上限"；表下"暂定编号"改为"已与 PRD-admin v0.2 第 10 节核对，无冲突，按本表写入 errors.json"。（E-07） |

### 6.4 PRD-ui v0.2（`docs/prd/PRD-ui-v0.2.md`）

| # | 位置 | 改成什么 |
|---|---|---|
| U-1 | F-UI-33 规则 2（第 966 行）、规则 4（第 970～976 行）；AC-UI-33-7（第 986 行）；第 0 节"第二批"行；V-UI-16 | 规则 2 的"`reason` 缺失或为 `removed`"改为"`reason` 缺失、为 `removed` 或为不认识的值"。规则 4 标题改为"按 `reason` 区分"，表中 `dissolved` 行标【M3】、`left` 行标【M4】；表下说明改为"'群聊已解散'在 M3 随管理接口解散（PRD-admin F-ADM-24 第一批、PRD-social F-SOC-26 M3 部分）交付，M4 起客户端解散同样适用"。新增 AC-UI-33-9【M3】："alice 停在托管群 g1 的聊天页时，业务方调用 `DELETE /admin/groups/g1`：3 秒内列表底部出现'群聊已解散'，输入框消失并显示'群聊已解散'；会话列表中 g1 消失"。AC-UI-33-7 保持 M4。第 0 节和 V-UI-16 中"'群聊已解散'按 S-10 随 M4"改为"随 M3"。（D-02） |
| U-2 | F-UI-18 文案表（第 650 行之后）；F-UI-43 规则 2（第 1176 行）；AC-UI-43-3（第 1182 行） | F-UI-18 表增加一行"`4012 / suspended`（第二批）｜有 `until` 时'账号已被封禁，解封时间：{YYYY-MM-DD HH:mm}'；`until = null` 时'账号已被封禁'"。F-UI-43 规则 2 改为"在线时被封禁：服务端下发 `kicked {reason: "suspended", until}` 并以 4012 关闭，按 F-UI-18 弹窗，文案见 F-UI-18 表 `suspended` 行"。AC-UI-43-3 改为"u1 在线时被封禁 1 天，3 秒内弹出'账号已被封禁，解封时间：{封禁时间 + 1 天}'；点'确定'后回到登录页，不自动重连"。（D-08） |
| U-3 | 第 12 行"背景文档" | `[PRD-admin v0.1](PRD-admin-v0.1.md)（第二批的客户端行为）` 改为 `[PRD-admin v0.2](PRD-admin-v0.2.md)（第一批的解散群提示、第二批的客户端行为）`；4.4 节开头"以 PRD-admin v0.2 与 errors.json 定稿为准"保留。（D-09） |

---

## 7. 结论

**需要再改。** 原 13 个阻塞项：已关闭 10、部分 3（R-ADM-07、R-OPS-01、R-OPS-02）、未关闭 0；新发现阻塞级问题 1 个（N-1，运维空间应急授权越权）。

- 公共基座 vs PRD-ops：B-OPS-01～15 中完全满足 4 条（06、09、10、12）；规则满足但 AC 或细节冲突 4 条（01、04、07、13）；基座缺能力、部分满足 4 条（05、11、14、15）；直接冲突 3 条（02、03、08）。归并为 **11 个冲突**（C-01～C-11）；审计员可见范围核对后一致（C-12），不需要改。
- 错误码：**无重复编号**；冲突和遗漏 **8 处**（E-01～E-08），新码全部符合号段规则，两类既有例外（413x 用 400、429x 不可重试）需要写进 ERRORS.md。
- 其他跨文档冲突 **10 处**（D-01～D-10），其中 D-07（Q-1）不需要改文档，只需 Frank 在第二批开工前确认。
- 修改清单 **32 条**（PRD-admin 15、PRD-ops 10、PRD-social 4、PRD-ui 3），全部是文字修改，约 1 人日。

改完第 6 节后，PRD-admin v0.2、PRD-ops v0.2 可以冻结为 v1.0，不需要再做一轮复审，只需按第 6 节逐条核对 diff。PRD-social、PRD-ui 的 7 条修改是跨文档对齐，建议与两个后台 PRD 同一批改完，再一起合入 PRD-core v1.3 和 errors.json（errors.json 只增不改，四份 PRD 的码一次写全）。
