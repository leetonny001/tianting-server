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
| 全部 | ⬜ 未开始 | |

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
