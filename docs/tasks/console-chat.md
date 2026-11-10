# console-server 任务单：天听聊天产品第一批（后台配合，C1、C2、C3、C4、C5）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 11.6、14 节、F-ACC-09、F-APN-01 规则 4、F-RCL-04 和 F-CVS-03 规则 2；[PRD-admin v1.2](../prd/PRD-admin-v1.4.md) 基座（权限点、两阶段审计、错误码）；[errors.json](../../protocol/errors.json) |
| 范围 | **C1、C2、C3、C4、C5**（CH-27）。C3 是 Frank 2026-11-10 决定按顺序继续的，同一天又决定"推送后续做，先做功能"，**C3 已暂停**（CS-203 保留）；C4 同一天决定接着做（CS-202 从 C2 挪来，CS-204 新增）；C5 是 Frank 2026-11-10 说"开始 C5"后分发的（CS-205，QUEUE Q29）。词库、举报（`moderation.words`、`report.*`、`console-msgread` 增加 `report.evidence`）属于 C6，这里不做 |
| 负责 | 后端（AI） |
| 仓库 | tianting-console-server（Go）。管理后台页面在 tianting-admin，见 [frontend-chat.md](frontend-chat.md)；im-server 接口见 [backend-chat.md](backend-chat.md) |
| 编号 | **CS-200 起** |
| 完成定义 | 任务内的 AC 有通过的测试（集成测试连真实 Postgres、对真实 im-server 的端到端）；`go test -race ./...` 全过；权限矩阵（AC-ADM-05-1 的参数化测试）覆盖新路由；所有写操作走现有两阶段审计。**CI 现在不运行（P-21）**，结果都是本地结果 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| CS-200 | ✅ 完成（接口部分，页面随 FE-619） | console-server `6820da8`（**本地结果，CI 不运行**：真实 Postgres 17、真实 im-server，`go test -race -count=1 ./...` 全过）。AC-ACC-05-9（console 路由、源码、scope）、09-2、09-5、09-6、09-7、09-8、09-13 通过（`e2e/invites_test.go`、`internal/app/route_test.go`），矩阵（AC-ADM-05-1）覆盖 3 个新路由。AC-ACC-09-1 只验了后端部分，三端注册和"后台显示已用 2 次"的页面随 FE-610～619；AC-ACC-09-11 只验了接口，页面随 FE-619。AC-CHAT-90-5 的 C1 后端 e2e 通过，部署 e2e 没有执行。变异确认 20 条全部发现 |
| CS-201 | ✅ 完成 | console-server@ec25a15（**本地结果，CI 不运行**：真实 Postgres 17、真实 im-server `IM_FILES_ENABLED=1` + 测试里的假 S3 `e2e/fakes3_test.go`）。代码没有改：元数据是白名单结构（`messageMeta`），正文接口原样透传 `body`，后台版序列化本来就能处理 `image`、`file`。新增 `TestTC_FILE_91_01_MetaHasNoFileName`（TC-FILE-91-01，AC-ADM-30-7 的 C2 扩展）：客服查元数据，console 响应和它从 im-server 拿到的响应都没有文件名、没有 `body`，只有类型和正文长度；正文授权后文件消息带文件名、大小、类型，图片带宽高、没有文件名；console 审计（`msg.meta.query`、`msg.read.grant`、`msg.read.page`）、console 全库、im-server 审计、两边日志都搜不到文件名。变异 1 项（元数据结构加回 `body`）被发现。后台 CSP 没有改（AC-CON-10-3 原有用例）。页面见 FE-639。`go test -race -count=1 ./...` 全过（临时 Postgres 17 集群，本地）。**没有验收的部分**：第 14.1 节元数据"显示大小"——im-server 元数据接口只有正文长度（见问题 12）；单聊正文开关没有另测（沿用 CS-200 之前的用例） |
| CS-203 | ⬜ 未开始 | 2026-11-10 分发（C3） |
| CS-204 | ✅ 完成（console 一侧，页面随 FE-671） | console-server@427131e（**本地结果，CI 不运行**：真实 Postgres 17、真实 im-server `IM_MSG_ACTIONS_ENABLED=1`、`IM_FILES_ENABLED=1` + 假 S3，`go test -race -count=1 ./...` 全过）。元数据白名单 `messageMeta`、正文白名单 `bodyMessage` 加 `recalled`、`recalledAt`、`recalledBy`（`omitempty`）；`body` 原样透传（`quote`、`mentions` 在里面）。新增 `e2e/recall_test.go` 3 个用例：`TestTC_RCL_04_07_MetaRecalled`（AC-RCL-04-7：撤回的文本、文件两层响应都有 `recalled*`，没有 `body`、文件名、`fileSize`，字段只在白名单内）、`TestTC_RCL_04_03_BodyRecalled`（AC-RCL-04-3：普通授权撤回的 `deleted` 没有、`recalled=true`、没有 `body`；隔离授权有原文；`quote`、`mentions` 透传；引用文字不进元数据、console 审计和日志）、`TestTC_RCL_04_01_DeleteRecalled`（AC-RCL-04-1 经 console 的一次：200、元数据仍是撤回、没有 `msgDeleted`（哨兵法）、两边审计都有；响应丢失 504 / 5040 后对账补 `success`、不是 `localOnly` 或不一致）。单聊 `IM_C2C_BODY_ACCESS` 没有在 console 另测（im-server TC-RCL-04-05 覆盖，console 透传）。变异见 CS-202 一行 |
| CS-202 | ✅ 完成（console 一侧，页面随 FE-671；批次 C4） | console-server@427131e（**本地结果**，同上）。新路由 `POST /console/api/files/{fileId}/fetch`（`msg.quarantine.read`、POST、请求体 `{convId, msgId, variant?}`），用 `console-msgread` 调 im-server 的 fetch；`imclient.Stream` 流式透传（不落盘、不缓存，超时只算到响应头）；固定 `Content-Disposition: attachment`（不带文件名）、`nosniff`、`no-store`，CSP 不变；两阶段审计 `msg.file.fetch`（intent 记 `fileId`、`msgId`、`variant`、授权前 8 位，result 记字节数；不记文件名）。授权按这个浏览器会话的隔离授权查、并比到期时间（问题 14）。新增 `TestTC_RCL_04_09_FetchViaConsole`（AC-RCL-04-9 console 一侧：没有权限点 4030、没有授权 / 只有普通授权 / 换浏览器会话 / 过期 4039 且不调用 im-server；有授权原图、缩略图 SHA-256 一致；没撤回、`fileId` 不属于 `msgId` 404 / 4048；主凭证直接调 im-server 4030；intent 写失败 5031 不调用；审计、全库、日志、请求都没有文件名）、`imclient.TestStream`；矩阵（AC-ADM-05-1）加了这个路由的样本（矩阵世界改成打开撤回和文件）。**变异 13 项全部被发现**（到期判断、`attachment`、隔离标志、路由权限点、元数据和正文的 `recalled`、审计 `fileId`、intent 失败、Stream 超时、Content-Type、对账按 localOnly、`nosniff`、本地过期不解绑）。单聊 `IM_C2C_BODY_ACCESS=off` 没有在 console 另测（im-server TC-RCL-04-09 单聊部分覆盖，console 透传 4030） |
| CS-205 | ✅ 完成 | console-server@6f99519（**本地结果，CI 不运行**：真实 Postgres 17、真实 im-server@383bc0a 开 `IM_CONV_SETTINGS_ENABLED`、`IM_MSG_ACTIONS_ENABLED`、`IM_FILES_ENABLED`、`IM_C2C_BODY_ACCESS=on` + 假 S3，`go test -race -count=1 ./...` 全过）。代码没有改。新增 `e2e/cvs_test.go` 的 `TestTC_CVS_03_01_AdminQueriesUnaffected`（TC-CVS-03-01 第 4 步，AC-CVS-03-1，问题 19）：u1 置顶、免打扰后删除和 u2 的单聊（10 条，含特征串和撤回的图片）和群 g1；先确认 u1 的 `convs.deleted` 有这两个会话、`sync` 为空，再验后台的客服元数据、审核员正文、隔离原文、经 console 取撤回文件（原图、缩略图哈希）、用户会话列表都和删除前逐条一致，审计照写；u1 的 `pinned`、`notifyOff`、`clearedSeq` 不出现在任何后台响应里。反证 6 处（在 im-server 副本上改：元数据、正文、隔离原文、文件代理分别套用删除者的 clearedSeq，用户会话列表套用隐藏条件，用户详情多返回会话设置）都能抓到。见问题 20、21；**问题 20、22 补齐**（console 是白名单结构、不透传）：console-server@25b9922（用户详情：只放 `UserRow` 实际读的字段和 console 自己的 `account`、`accountUnavailable`，按原始 JSON 挑键，保留"键在不在"和 `null` 的含义）、@8988242（用户列表：外层 `users`、`nextCursor`，行内 `userId`、`nickname`、`createdAt`、`online`、`official`、`ban {until}`、`mutedUntil`；`TestBatch2` 列表断言改成只验 `ban.until`，封禁理由只在详情，依据 AC-ADM-13-9）、@57da60f（群列表、群成员、用户所在群、数据概览：`imRead` 换成 `imList`，分页外层只放列表和 `nextCursor`，行内按各页面实际读的字段；已有 e2e 断言没有改）。新增 `TestUserDetailWhitelist`、`TestUserListWhitelist` 和 `e2e/list_whitelist_test.go` 4 个用例：键集合和页面字段完全一致，翻页照常，im-server 多返回的字段（含 `convSettings`、未知字段）都不透传；反证 13 + 10 + 34 处都能抓到。console 里已经没有原样透传 im-server 响应的读接口。`go test -race -count=1 ./...` 全过（本地） |

## 总览

| 批次 | 任务 | 人日 | 第 18 节"console + 后台页面" | 差异说明 |
|---|---|---|---|---|
| C1 | CS-200 | 0.5 | 1.5（这里 0.5，页面 FE-619 1） | 合计一致 |
| C2 | CS-201、CS-202 | 1.5 | 2.5（这里 1.5，页面 FE-639 1） | 合计一致 |
| C3 | CS-203 | 0.5 | 0.5 | 一致 |
| C4 | CS-204（另做从 C2 挪来的 CS-202） | 0.5 | 1.5（这里 0.5，页面 FE-671 1） | 合计一致；CS-202 的 0.5 仍算在 C2 行（第 18 节按 C2 估的），C4 实际要做 1 |
| C5 | CS-205 | 0.5 | 0.5（管理后台页面没有改动） | 一致 |
| 公共 | — | 0 | 0 | 一致 |

**依赖顺序**：BE-421 → CS-200；BE-434 → CS-201；BE-453、BE-457 → CS-203 → BE-459（C3 暂停）；BE-474 → CS-204、CS-202 → FE-671 → BE-480；BE-485 → CS-205 → BE-489（C5）。

---

### CS-200 邀请码接口和用户详情的账号信息
- 优先级：P0
- 批次：C1
- 仓库：tianting-console-server
- 依赖：BE-421
- AC：AC-ACC-05-9、AC-ACC-09-1、AC-ACC-09-2、AC-ACC-09-5、AC-ACC-09-6、AC-ACC-09-7、AC-ACC-09-8、AC-ACC-09-11、AC-ACC-09-13
- 内容：
  1. 权限点 `account.invite`（`operator`，F-ACC-09 规则 2，第 14.1 节）。主凭证 `console` 增加 scope `account.invite`；其他 scope 属于 C6。权限注册表和矩阵增加样本，`support`、`auditor` 403 / 4030。
  2. 代理 `POST /admin/invites`、`GET /admin/invites`、`POST /admin/invites/{id}/disable`：写两阶段审计，**审计不记明文**（规则 2）；审计写失败 503 / 5031，邀请码不产生（AC-ACC-09-8）。明文只在生成响应里出现一次。错误透传 4131、4000。
  3. 用户详情（规则 5）：调 `GET /admin/accounts/{userId}`，合入现有详情响应：是否有账号、注册时间、最近登录时间、设备会话数；不含令牌；权限沿用查看用户详情。
  4. **没有 `account.reset`**：`AC-ACC-05-9` 枚举 console 路由和 `console` 主凭证 scope，断言没有改密码、改恢复码的接口。
  5. 超管默认权限按 F-CON-06 规则 4 不变。
- 工作量：0.5 人日（console + 后台页面列）

### CS-201 元数据和正文接口配合文件消息
- 优先级：P0
- 批次：C2
- 仓库：tianting-console-server
- 依赖：BE-434
- AC：AC-ADM-30-7
- 内容：
  1. 第 14.1 节：消息元数据页的接口只透传类型和大小；**文件名按正文对待，不出现在 `msg.meta` 两层响应里**（R-CS-14，第 16.2 节 AC-ADM-30-7 扩展到文件名，C2 就有文件消息，所以这条随 C2 生效）。
  2. 正文接口（`msg.read.group`）的后台版序列化能处理 `image`、`file`：带文件名；单聊受 `IM_C2C_BODY_ACCESS` 限制不变。
  3. 后台 CSP 不放宽（AC-CON-10-3 不变）。
  4. C2 没有专属 AC，第 18 节算了 console 部分的人日；这里只挂已有的 AC-ADM-30-7。元数据里的撤回状态随 C4。
- 工作量：1 人日（console + 后台页面列）

### CS-202 审核员查看文件的代理
- 优先级：P1
- 批次：C4（原 C2；P-37 ④，Frank 2026-11-09 挪到 C4）
- 仓库：tianting-console-server
- 依赖：CS-201、BE-474（im-server `POST /admin/files/{fileId}/fetch`，形状见 BE-470 第 4 条）
- AC：AC-RCL-04-9
- 内容：
  1. 第 14.1 节"文件内容凭授权看，文件经 console-server 代理，后台 CSP 不放宽（R-CS-24）"；im-server 侧接口是 `POST /admin/files/{fileId}/fetch`（第 11.6 节）。
  2. **Frank 2026-11-06 决定不放开**（PRD v1.1 第 14.1 节）：正文页只显示文件名和"不可预览"，文件内容只走 `msg.quarantine`（隔离原文已有），举报证据的 `report.evidence` 随 C6。法务回复后再决定是否放开 `msg.read.group`。
  3. 代理响应固定 `Content-Disposition: attachment`，不内联；写审计。
  4. C4 范围（AC-RCL-04-9）：只开放 `msg.quarantine` 路径——审核员对**已撤回或已被后台删除**的文件消息，在这个会话有有效隔离授权时，经 console 代理取文件（原图或缩略图）；没有授权 403 / 4039，没有权限点 403 / 4030（console 和 im-server 两层都拦）；单聊受 `IM_C2C_BODY_ACCESS`。console 用正文类调用方 `console-msgread` 调 im-server，响应字节流式透传、不落盘、不缓存；console 审计记 `fileId`、`msgId`、授权单号，不记文件名。`report.evidence` 的路径随 C6。
  5. 后台 CSP 不放宽（AC-CON-10-3）；页面的"查看文件"按钮见 FE-671。
- 工作量：0.5 人日（console + 后台页面列；人日算在 C2）

---

### CS-203 后台下线、封禁删除推送令牌；运维配置检查显示推送项
- 优先级：P0
- 批次：C3
- 仓库：tianting-console-server（端到端测试）、tianting-ops（配置检查页，只在需要时改）
- 依赖：BE-453、BE-457
- AC：AC-APN-01-3、AC-CHAT-90-17
- 内容：
  1. F-APN-01 规则 4：后台的**强制下线、封禁**（console 调 im-server 管理接口）之后，被吊销 `sid` 绑定的推送令牌要在同一事务删除。逻辑在 im-server（BE-453），这里补 console 路径的端到端：经 console 下线、封禁后，给该用户发消息，假通道没有调用（哨兵法）；console 审计不含令牌（只有 im-server 记的哈希前缀也不进 console 审计）。
  2. 第 14.2 节：`tianting-ops` 的配置检查页按 `GET /internal/v1/config` 显示新增的 APNs / FCM 启用和密钥状态、最近一次调用结果（BE-457）。页面如果是按接口数据通用渲染就不改代码，只补一条走查记录；如果是写死的项，补上这两项，不显示密钥。
  3. 用户详情的"设备会话数"不变，不显示推送令牌（F-ACC-09 规则 5 没有这一项）。
- 工作量：0.5 人日（console + 后台页面列）

### CS-204 后台元数据和正文接口配合撤回
- 优先级：P0
- 批次：C4
- 仓库：tianting-console-server
- 依赖：BE-474
- AC：AC-RCL-04-1、AC-RCL-04-3、AC-RCL-04-7
- 内容：
  1. 元数据接口（`msg.meta`，CS-201 第 4 条留的）：console 的 `messageMeta` 是白名单结构，增加 `recalled`、`recalledAt`、`recalledBy` 透传；仍然没有 `body`、文件名（AC-RCL-04-7）；撤回的文件消息没有 `fileSize`（im-server 不返回）。
  2. 正文接口（`msg.read.group`）：按 im-server 的后台版序列化透传，撤回的消息 `deleted=false`、`recalled=true`、没有 `body`（AC-RCL-04-3）；隔离接口（`msg.quarantine`）返回撤回的原文（单聊照旧受 `IM_C2C_BODY_ACCESS`）。正文里的 `quote`、`mentions` 原样透传（引用文字按正文对待，不出现在元数据和 console 日志里）。
  3. 后台删除已撤回的消息（AC-RCL-04-1）：console 两阶段审计照写，im-server 幂等返回 200；console 对账把它记成成功，不当成"状态不一致"。
  4. 页面见 FE-671。
- 工作量：0.5 人日（console + 后台页面列，撤回行）

### CS-205 后台查询不受删除会话、置顶、免打扰影响
- 优先级：P0
- 批次：C5
- 仓库：tianting-console-server
- 依赖：BE-485
- AC：AC-CVS-03-1
- 内容：
  1. F-CVS-03 规则 2"后台查询不受影响"（没有单独的 AC，用 TC-CVS-03-01 第 4 步覆盖，问题 19）：console-server e2e（真实 im-server，`IM_CONV_SETTINGS_ENABLED=1`）里 u1 删除和 u2 的单聊、删除 g1 的会话之后，客服的 `msg.meta` 查询、正文授权读取（`msg.read.group`）、隔离读取照常返回全部消息；u1 的置顶、免打扰不出现在用户详情和元数据（第 14.1 节没有列，console 是白名单结构，不透传）。
  2. 代码预计不用改（console 不读 `conv_settings`，im-server 的管理接口不经过 `authorize`）。im-server 改了 `minSeq`（BE-485 第 4 条）之后跑一遍现有 console e2e，确认没有回归。
  3. 管理后台页面没有 C5 的改动（第 14.1 节没有 C5 的行），运维后台也没有（AC-CHAT-90-17 的 C5 列是"—"）。
- 工作量：0.5 人日（console + 后台页面列）

---

运维后台（第 14.2 节）的配置检查和指标都由 im-server 提供（BE-423、BE-438、BE-457），`tianting-ops` 的配置检查页按接口数据显示。C1、C2 没有页面改动；C3 的推送项见 CS-203。C4 的撤回指标 `im_recalls_total{by}` 和配置检查项 `msg_actions` 由 im-server 提供（BE-471），运维后台按接口数据通用显示，C4 不改页面（如果配置检查页是写死的项，随 FE-672 走查时补）。治理的项随 C6。

---

## 实现时发现的问题

都没有改冻结 PRD 和 AC，也没有改 im-server。下面按最保守的理解实现，要改口径由 Frank 决定。

1. **部署示例缺 `account.invite`**：PRD-admin F-KEY-02 规则 4（v1.3）和 PRD-chat 第 14.1 节要求主凭证有这个 scope，`deploy/im-admin-callers.example.json` 和 `deploy/test_deploy.py` 没跟上，已补。**已部署环境的调用方配置要手动加**，否则运营生成邀请码是 403 / 4030（`TestInviteScopeMissing`）。`deploy/test_deploy.py` 本机缺 pyyaml，没有运行；同样的比对由 console 的 `TestTC_ACC_05_08_ConsoleScopeAndCalls` 覆盖。
2. **审计动作名不用权限点的名字**：console 里 `account.` 开头的动作按"只改自己的库"处理：对账不查 im-server（`localOnlyAction`），404 记成 `denied`。邀请码是 im-server 的写操作，所以动作名是 `invite.create`、`invite.disable`，权限点仍是 `account.invite`。tianting-admin 的审计页如果有动作名映射要加这两个。
3. **结果未知时有一个没人知道明文的邀请码**：生成时 504 / 5040，im-server 可能已经生成，明文只在那次响应里，找不回。PRD 没有写怎么办。现在不重试、不补发，对账补审计结论；页面要提示到列表停用最新的几条再重新生成（FE-619）。
4. **`deleteRequestedAt` 不透传**：`GET /admin/accounts/{userId}` 多返回这个字段，F-ACC-09 规则 5 只列了 4 项，console 用独立类型只留这 4 项。客服如果要看"注销申请中"，要改规则 5。
5. **账号信息取不到时详情照常返回**：PRD 没有写。现在是 `account: null`、`accountUnavailable: true`，不写成 `hasAccount=false`，客服仍能看资料和强制下线。
6. **`note` 不进 console 审计**：F-CON-08 规则 4 说 intent 含参数摘要，备注是自由文本，可能被人粘贴邀请码，所以只记 `hasNote`。后 4 位也不记，追溯靠 `inviteId`。列表里备注和后 4 位照常显示。
7. **邀请码操作不要理由和 step-up**：PRD-admin 第 5.3 节的表没有列，按表实现，只要确认框。公开传播大额邀请码等于开放注册（RC2-18），如果要加 step-up 改第 5.3 节。
8. **im-server 列表没有 `nextCursor`**：console 按"满一页"算，正好整页时最后会多翻到一个空页。console 的 `limit` 沿用 20、50、100，默认 50。
9. **`count` 的边界按 im-server**：`count=0` 或省略是 1 个；负数是 4131，不是 4000。AC-ACC-09-7 没有覆盖负数，没有在 console 另外拦。
10. **枚举口径**：TC-ACC-05-08 按 `password`、`recovery`、`reset` 搜 console 路由，会命中后台账号自己的 3 个接口（`/me/password`、`/me/recovery-codes`、`/accounts/{id}/reset`）。这 3 个不改 IM 用户，用例写成显式白名单，并用源码解析确认它们的处理函数不调用 im-server。
11. **AC-CHAT-90-5 不在 CS-200 的 AC 行**：验收记录把 console 日志和审计部分记在 CS-200 下，追溯矩阵只挂 BE-422、BE-440。用例已写（`TestTC_CHAT_90_04_NoSecretsInLogsAndAudit`），没有改 AC 行，追溯矩阵不变。
12. **元数据页的"大小"拿不到文件大小**（CS-201、FE-639）：PRD-chat 第 14.1 节写元数据页"显示类型和大小"，im-server 的 `/admin/convs/messages/query` 对文件消息只有 `bodyLength`（`store/admin.go` 第 84 行：非文本消息是 body JSON 的码点数，正文只存 `fileId`，所以和文件名、文件大小都无关），没有文件大小。console 只能透传它有的字段，页面显示类型和"正文长度"。要显示文件大小需要 im-server 在元数据里加 `size`（文件大小不算正文），属于后端改动，没有做，由 Frank 定。**P-37 ③ 已定（2026-11-09）**：im-server 加 `fileSize`，console 透传，页面显示。**已落实（2026-11-09）**：im-server 已加（tianting-server@f50b46c）；console 的元数据是白名单结构，tianting-console-server@62941c3 加 `fileSize`（`omitempty`），TC-FILE-91-01 断言文件、图片的 `fileSize` 等于原文件字节数、文本没有、仍然没有文件名和 `body`（去掉字段时失败），`go test -race -count=1 ./...` 全过（临时 Postgres 17、真实 im-server，本地）；页面 tianting-admin@4b18297（"大小"一列，图片、文件显示"1.2 MB"这样的大小，其他照旧正文长度），Vitest 111 个、e2e 56 个过（3 个按配置跳过）。
13. **CS-202 等 im-server 接口**：`POST /admin/files/{fileId}/fetch` 在 im-server 里还没有（AC-RCL-04-9 是 C4），请求、响应的形状 `protocol/ADMIN.md` 也没有写。console 侧没有先写一个调不通的代理。**P-37 ④ 已定**：CS-202 挪到 C4。**C4 分发（2026-11-10）**：接口形状在 backend-chat BE-470 第 4 条定了，im-server 实现见 BE-474。
14. **fetch 的授权绑定和到期**（CS-202）：im-server 的 `fetch` 请求没有 `grantId`，按调用方 + 操作人 + 会话查有效隔离授权（backend-chat 第 66 条）。同一个后台账号在另一个浏览器会话申请了授权，这个会话的旧授权过期后也能被 im-server 放行。console 按"授权绑定当前浏览器会话"（F-ADM-08，和正文读取相同）实现：先查这个会话对这个会话 ID 的 `quarantine` 授权，再拿 im-server 给的 `expiresAt` 和 console 时钟比，过期就 4039、解除绑定、不调用 im-server。console 和 im-server 时钟有偏差时到期会差几秒；更彻底的做法是 im-server 的 `fetch` 接受 `grantId`，属于 im-server 改动，没有做。2026-11-10 P-44 ④ 已落实（tianting-server@d17df86，console-server@4b6b9e2，本地结果）：fetch 请求体加可选 `grantId`，带上时只认这一个授权（调用方 + 操作人 + 会话 + 隔离 + 未过期），不带时不变；console 带这个浏览器会话的 `grantId`（PRD-chat v1.6 第 11.6 节）。
15. **"授权单号"记前 8 位**（CS-202 第 4 条）：完整 `grantId` 是读正文的凭据，console 不交给浏览器、审计里也只记前 8 位（和 `msg.read.page` 一致）；工单号在申请授权那条审计（`msg.quarantine.grant`）里。审计页按 `grant` 前缀和申请记录对得上。
16. **代理响应的 Content-Type 和文件名**（CS-202）：`Content-Disposition` 固定 `attachment`、不带文件名（文件名按正文对待，R-CS-14）；`Content-Type` 用 im-server 给的类型（解析不了是 `application/octet-stream`），加 `nosniff`，浏览器不会按它渲染。页面（FE-671）下载时用正文接口（隔离授权）里的文件名命名。
17. **传到一半断了**（CS-202）：状态码 200 已经发出去，没法改成错误；审计 result 记 `failure`、`params.incomplete=true` 和已传字节数。PRD 没有写。
18. **取文件不另限次数**（CS-202）：PRD 没有写配额；每次取文件都写一条两阶段审计，授权本身的配额在 im-server（申请时）。如果要限次数，改第 14.1 节。
19. **"后台查询不受影响"没有 AC**（CS-205，C5 分发时记）：F-CVS-03 规则 2 写了，PRD 没有单独的 AC。CS-205 的 AC 行挂 AC-CVS-03-1，用例 TC-CVS-03-01 第 4 步验后台元数据和正文，不新增 AC（backend-chat 第 87 条）。
20. **用户详情接口不是白名单**（CS-205 实测）：`internal/app/bizusers.go` 的 `getUser` 把 im-server 的响应读进 `map[string]any` 后原样返回，和"console 是白名单结构、不透传"的设计不符；im-server 以后在用户详情里加字段（比如会话设置）会直接透到后台页面。**协调方定（按已有设计补齐，不改产品行为）**：改成按后台页面实际用到的字段白名单输出，加用例；CS-205 的用例同时兜住。
21. **后台用户会话列表不隐藏用户删除的会话**（CS-205 实测）：im-server `ListUserConvMeta` 不套用删除会话的隐藏条件。符合 F-CVS-03 规则 2"后台查询不受影响"，按此验收，不改。
22. **还有 5 处读接口原样透传 im-server 的响应**（CS-205 第 20 条之后实测）：用户列表、群列表、群成员、用户所在群、数据概览都用 `imRead` 原样返回。**协调方定（按已有设计补齐，不改产品行为）**：都改成按页面实际读取的字段白名单输出，已在 console-server@8988242、57da60f 完成。
