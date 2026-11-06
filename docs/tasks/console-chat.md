# console-server 任务单：天听聊天产品第一批（后台配合，C1、C2）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.1](../prd/PRD-chat-v1.1.md)（冻结）第 11.6、14 节和 F-ACC-09；[PRD-admin v1.2](../prd/PRD-admin-v1.3.md) 基座（权限点、两阶段审计、错误码）；[errors.json](../../protocol/errors.json) |
| 范围 | **只有 C1、C2**（CH-27）。词库、举报（`moderation.words`、`report.*`、`console-msgread` 增加 `report.evidence`）属于 C6，这里不做 |
| 负责 | 后端（AI） |
| 仓库 | tianting-console-server（Go）。管理后台页面在 tianting-admin，见 [frontend-chat.md](frontend-chat.md)；im-server 接口见 [backend-chat.md](backend-chat.md) |
| 编号 | **CS-200 起** |
| 完成定义 | 任务内的 AC 有通过的测试（集成测试连真实 Postgres、对真实 im-server 的端到端）；`go test -race ./...` 全过；权限矩阵（AC-ADM-05-1 的参数化测试）覆盖新路由；所有写操作走现有两阶段审计。**CI 现在不运行（P-21）**，结果都是本地结果 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| CS-200 | ✅ 完成（接口部分，页面随 FE-619） | console-server `6820da8`（**本地结果，CI 不运行**：真实 Postgres 17、真实 im-server，`go test -race -count=1 ./...` 全过）。AC-ACC-05-9（console 路由、源码、scope）、09-2、09-5、09-6、09-7、09-8、09-13 通过（`e2e/invites_test.go`、`internal/app/route_test.go`），矩阵（AC-ADM-05-1）覆盖 3 个新路由。AC-ACC-09-1 只验了后端部分，三端注册和"后台显示已用 2 次"的页面随 FE-610～619；AC-ACC-09-11 只验了接口，页面随 FE-619。AC-CHAT-90-5 的 C1 后端 e2e 通过，部署 e2e 没有执行。变异确认 20 条全部发现 |
| CS-201、CS-202 | ⬜ 未开始 | C2 |

## 总览

| 批次 | 任务 | 人日 | 第 18 节"console + 后台页面" | 差异说明 |
|---|---|---|---|---|
| C1 | CS-200 | 0.5 | 1.5（这里 0.5，页面 FE-619 1） | 合计一致 |
| C2 | CS-201、CS-202 | 1.5 | 2.5（这里 1.5，页面 FE-639 1） | 合计一致 |
| 公共 | — | 0 | 0 | 一致 |

**依赖顺序**：BE-421 → CS-200；BE-434 → CS-201 → CS-202。

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
- 批次：C2
- 仓库：tianting-console-server
- 依赖：CS-201
- AC：无（C2 没有专属 AC）
- 内容：
  1. 第 14.1 节"文件内容凭授权看，文件经 console-server 代理，后台 CSP 不放宽（R-CS-24）"；im-server 侧接口是 `POST /admin/files/{fileId}/fetch`（第 11.6 节）。
  2. **Frank 2026-11-06 决定不放开**（PRD v1.1 第 14.1 节）：正文页只显示文件名和"不可预览"，文件内容只走 `msg.quarantine`（隔离原文已有），举报证据的 `report.evidence` 随 C6。法务回复后再决定是否放开 `msg.read.group`。
  3. 代理响应固定 `Content-Disposition: attachment`，不内联；写审计。
- 工作量：0.5 人日（console + 后台页面列）

---

运维后台（第 14.2 节）的配置检查和指标都由 im-server 提供（BE-423、BE-438），`tianting-ops` 的配置检查页按接口数据显示，本批没有页面改动。推送、撤回、治理的项随 C3～C6。

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
