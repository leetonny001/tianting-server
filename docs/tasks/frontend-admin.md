# 前端任务单：管理后台 H5（tianting-admin）与客户端配合（PRD-admin）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-admin v1.0](../prd/PRD-admin-v1.0.md)（冻结）第 2.3 节（前端对错误码的处理）、第 4 节 F-CON-02～12 的前端部分、第 5 节、第 6 节、第 7 节页面；[PRD-ui v1.0](../prd/PRD-ui-v1.0.md)（客户端文案）；[errors.json](../../protocol/errors.json) |
| 负责 | 前端（AI） |
| 仓库 | **tianting-admin**（React，`../tianting-admin`，已建空仓库）：FE-400～FE-424、FE-440；**客户端配合**：tianting-sdk-ts、tianting-web（FE-430～FE-434），tianting-sdk-kotlin（FE-435），tianting-sdk-swift（FE-436） |
| 技术栈 | React + Vite + TypeScript；组件库选型条件：能在 F-CON-10 规则 4 的 CSP 下工作（`style-src 'self' 'nonce-…'`，不允许 `unsafe-inline` 脚本）；测试：Vitest（组件与 API 客户端）、Playwright（对接 console-server 测试模式 + im-server 测试模式的端到端） |
| 后端 | 只调用 console-server 的 `/console/api/*`（同源，Cookie 会话）；**浏览器里永远不放 IM 管理密钥或签名私钥**。console-server 任务见 [console.md](console.md) |
| 开工条件 | CS-001（骨架、错误响应格式、测试模式）可用；在此之前用 MSW 模拟接口开发页面 |
| 完成定义 | 任务内列出的 AC 都有通过的测试（Vitest 或 Playwright）；`npm run lint`（含禁止 `dangerouslySetInnerHTML` / `innerHTML` 规则）、`npm audit --audit-level=high`、`npm test`、`npm run e2e` 全过；最小宽度 1280px，Chrome / Edge / Safari / Firefox 最新两个大版本 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| FE-400 ～ FE-405、FE-410 ～ FE-416 | ✅ 2026-11-04 | tianting-admin `320af4a`。30 个 Playwright 用例对真实 console-server + im-server + Postgres（生产 CSP 响应头）、17 个单元测试、lint、`npm audit`（只有 ESLint 依赖链的 moderate，没有 high）、构建产物扫描全过。**只在 Chrome 上跑过**：Edge、Safari、Firefox 未测。**没有端到端覆盖的**：AC-ADM-10-4（断线后刷新显示离线）、22-1（1500 人翻页，接口层已测）、15-7 的页面提示（接口和文案表已测）、23-2、16-1 的 1 秒生效（im-server 行为）；AC-ADM-14-6 依赖 B6，现在的文案没有列出未响应节点。CI 只跑 lint、单元测试、构建和扫描，端到端要等 P-9 |
| FE-420 ～ FE-423 | ✅ 2026-11-05 | 封禁、正文查看和水印、删除、隔离原文、禁言。`e2e/moderation.spec.ts` 4 个用例对真实后端通过（含两边时钟推进 16 分钟后 4039 再弹框），全量 34 通过、1 跳过。只在 Chromium 上验证；tianting-admin 的端到端测试还没进 CI |
| FE-424 | ⏸ 等二期 push 协议定稿 | |
| FE-431 ～ FE-433 | ✅ 2026-11-05 | tianting-sdk-ts `38ed458`、tianting-web `191094f`，CI 通过（含真实服务端的集成和 Playwright）。重连后复查已加载范围，断线期间被删的原文也会清掉。未验证：真机；断线期间被删、时钟偏差只有单元测试；页面走真实后台的全链路（测试直接调签名接口） |
| FE-434 | ⏸ 等 push 协议 | |
| FE-435、FE-436 | ✅ 2026-11-05 | tianting-sdk-kotlin `4fe4934`、tianting-sdk-swift `4815019`，各自的集成测试本机通过；CI 里的集成测试等 P-15 |
| FE-440 | ⬜ 未开始 | P1 / P2 |

**规格冲突的处理**：AC-ADM-05-3 要求客服的菜单里没有"社交配置"、直接访问显示无权限，而 5.2 节矩阵和 AC-ADM-16-4 让客服能读配置。处理：页面按 `social.settings` 守卫，`GET /console/api/settings/social` 仍然对 `social.read` 开放。

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| 基座页面（第一批） | FE-400 ～ FE-405 | 9 | 工程与 API 客户端（错误码处理、CSRF、step-up 重放）、布局与权限菜单、登录 / 激活 / 绑定、账号管理、审计日志、通知与个人设置 |
| 业务页面（第一批） | FE-410 ～ FE-416 | 10 | 用户、强制下线与在线设备、导入、社交、群列表与建群、群详情与解散、消息元数据 |
| 业务页面（第二批） | FE-420 ～ FE-424 | 8.5 | 封禁、正文与水印、删除与隔离原文、禁言、系统通知（最后） |
| 客户端配合 | FE-430 ～ FE-436 | 第一批 1、第二批 10 | sdk-ts + H5：`convRemoved.reason` 与忽略未知（第一批）；封禁、删除、禁言、推送提示（第二批）；sdk-kotlin / sdk-swift 首版（M5 / M6） |
| P1 | FE-440 | （不计入两批） | 数据概览 |

**人日对照（前端部分）**

| 批次 | 本任务单 | PRD-admin 第 14 节 | 差异说明 |
|---|---|---|---|
| 第一批 管理后台 | 19（基座 9 + 业务 10） | 18（基座 8 + 业务 10） | +1：FE-400 把 API 客户端的错误码处理、CSRF 令牌、step-up 弹框重放、构建产物密钥扫描放在一起估 2.5（PRD 的"错误码处理"分项没有单列人日） |
| 第二批 管理后台 | 8.5 | 8.5 | 一致 |
| SDK 与客户端 | 第一批 1、第二批 10 | 第一批 1、第二批 10 | 一致。**注意**：`convRemoved.reason`、"忽略未知事件 / 字段 / 枚举值"与 PRD-social V-11、V-18 是同一条，若 PRD-social 的前端任务单已经包含，FE-430 只做"群聊已解散"文案与验收，不重复实现 |

**两批合计对照（本任务分发涉及的三份任务单）**

| 批次 | im-server（BE） | console-server（CS） | 管理后台前端（FE-400～424） | SDK / 客户端（FE-430～436） | 合计 | PRD-admin 第 14 节 | 差异 |
|---|---|---|---|---|---|---|---|
| 第一批 | 15 | 31.5 | 19 | 1 | 66.5 | 75.5（其中测试 13） | 不含测试 13 时 PRD 为 62.5，本单 +4（console +3、前端 +1）；测试工作量（安全用例、e2e、性能、渗透测试配合）由用例 TC-KEY / TC-CON / TC-ADM 承载，不列任务 |
| 第二批 | 19 | 8 | 8.5 | 10 | 45.5 | 51.5（其中测试 6） | 不含测试 6 时一致（45.5） |

社交管理接口（F-SOC-01、12、13、30）的 im-server 实现已算在 PRD-social，PRD-admin 的 127 人日也不含它，两边口径一致。

依赖顺序：FE-400 → FE-401 → FE-402 → FE-403、FE-404、FE-405 → FE-410 ～ FE-416（各自依赖对应 CS 任务）→ FE-420 ～ FE-424（第二批，依赖 PRD-core v1.3 第二批与 sdk-ts 发布）。FE-430 随 PRD-social M3 的 sdk-ts 版本一起发；FE-431 ～ FE-434 在 im-server 打开 `IM_V13_ADMIN_FEATURES` **之前**发布。

---

## 基座页面（第一批）

### FE-400 工程脚手架、API 客户端与错误码处理（2.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：CS-001（错误响应格式、测试模式）；CS-009（会话与 CSRF）、CS-011（step-up）接口
- AC：AC-CON-05-2、AC-CON-05-10、AC-CON-05-12、AC-CON-07-3、AC-CON-07-6、AC-CON-10-1、AC-CON-10-4、AC-CON-10-6
- 内容：
  1. 脚手架：Vite + React + TS、ESLint（`react/no-danger`、禁止 `innerHTML` / `outerHTML` / `insertAdjacentHTML` 的自定义规则，AC-CON-10-6 用一个故意违规的夹具文件验证 lint 失败）、Prettier、Vitest、Playwright、CI（`npm audit --audit-level=high` 阻断）；组件库按 CSP 条件选型并在 README 记录（若必须 `style-src 'unsafe-inline'`，记入 ARCH 实现记录）。
  2. API 客户端（`src/api/client.ts`）：同源 `fetch`、`credentials: 'same-origin'`；**CSRF 令牌只放内存**，应用启动和页面刷新后先 `GET /console/api/session` 取令牌（AC-CON-05-10）；会话凭证不进 localStorage / sessionStorage（AC-CON-05-2）。
  3. 错误码处理（PRD 2.3 节）：4014 → 登录页（按 `reason` 选文案，`replaced` 显示"你的账号在别处登录，本次登录已退出"，AC-CON-05-12）；4016 → 绑定二次验证页；4038 → TOTP 弹框，通过后**自动重放原请求一次**（AC-CON-07-3）；step-up 输错 4015 在弹框内提示"动态码错误"、不跳转（AC-CON-07-6）；4039 → "工单号 + 理由 + 动态码"弹框（FE-421 使用）；4037 → 重新取 session 后重放一次，仍失败提示刷新；5031 → "审计服务不可用，操作未执行"；5040 → "操作结果未知，请刷新确认，不要重复提交"；其他按 `code`（+ `reason`）查文案表，不直接显示 `msg`。
  4. 后台请求：自动刷新、轮询请求带 `X-Console-Background: 1`；`document.visibilityState !== 'visible'` 时暂停轮询；用户操作发起的请求不带该头（单测断言）。
  5. 安全检查：CI 中对 `dist/` 搜索测试环境的 IM 密钥、签名私钥、`X-Admin-Key`、`X-IM-Signature`（AC-CON-10-1 前端部分，HAR 部分见 CS-017）；Playwright 在另一源的页面 iframe 嵌入后台，断言被浏览器拒绝（AC-CON-10-4，依赖 CS-001 的 `frame-ancestors 'none'`）。

### FE-401 布局、权限菜单与路由守卫、纯文本渲染与头像占位（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-400；CS-010（`perms`）
- AC：AC-ADM-05-3、AC-ADM-05-5、AC-CON-02-5、AC-CON-10-2、AC-CON-10-5
- 内容：
  1. 布局：左侧菜单（按 `perms` 显示）+ 顶栏（当前账号、全部角色名、通知铃铛、退出）+ 内容区；最小宽度 1280px。
  2. 路由守卫：每个路由声明所需权限点，无权限显示"无权限"页（AC-ADM-05-3：`support` 看不到"账号管理""审计日志""社交配置"，直接访问显示无权限）；多角色显示并集（AC-ADM-05-5）；`setup` 状态直接打开任何业务路由都跳回"绑定二次验证"页（AC-CON-02-5）。
  3. 文本组件：昵称、群名、公告、正文、推送内容、审计理由一律走 `<Text>`（纯文本），XSS 夹具 `<img src=x onerror=alert(1)>` 的 Playwright 用例覆盖用户列表 / 详情 / 群详情 / 元数据 / 审计详情（AC-CON-10-2，与 CS-017 共用）。
  4. 头像：**不加载外部头像 URL**，统一首字母占位，"查看原图地址"只显示 URL 文本；Playwright 断言没有对 `tracker.example` 的请求（AC-CON-10-5）。

### FE-402 登录、激活、绑定二次验证与恢复码（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-400；CS-005、CS-007
- AC：AC-CON-02-2、AC-CON-03-1、AC-CON-03-6
- 内容：
  1. 登录页：账号、密码、动态码（可切换为恢复码）一步提交；4015 统一提示"账号、密码或动态码错误"；4291 提示"尝试次数过多，请 N 分钟后再试"、4290 提示稍后再试，不自动重试。
  2. 激活页：账号名 + 激活码 + 新密码（显示密码策略，`reason` 对应文案：`tooShort` / `tooLong` / `containsUsername` / `weak` / `reused`）。
  3. 绑定页：显示二维码（otpauth URI 在前端生成二维码，不请求第三方）、输入动态码；成功后显示 10 个恢复码（只显示一次，提供"我已保存"确认），然后进入首页。
  4. 登录后若提示恢复码剩余 ≤ 2，顶栏提醒去个人设置重新生成。
  5. Playwright：新账号激活 → 绑定 → 首页菜单只有客服权限（AC-CON-02-2）；登录后 Cookie 名 `__Host-adm_sid`（AC-CON-03-1）；恢复码登录一次成功、再次失败（AC-CON-03-6）。

### FE-403 账号管理与应急授权（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-006、CS-008（解锁）、CS-010、CS-011
- AC：AC-CON-02-2、AC-CON-04-9、AC-CON-06-7
- 内容：
  1. 账号列表（本空间）、新建（账号名、显示名、角色多选）→ 显示一次性激活码（只显示一次，复制按钮）；改角色（互斥角色 `auditor` 选中时禁用其他选项，服务端 4093 仍要处理）、停用 / 启用、重置（显示新激活码）、解锁；全部操作先弹确认框，经 4038 弹框完成 step-up。
  2. 自我保护：对自己的账号隐藏"改角色 / 停用 / 重置"；服务端 4093 按 `reason` 显示原因。
  3. 应急授权（只有 `admin.emergency`）：角色（不含 `auditor`、`super_admin`）、时长 15～240 分钟、理由；显示到期时间（服务端返回）。

### FE-404 审计日志页（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-014
- AC：AC-CON-09-1、AC-CON-09-9、AC-CON-08-1、AC-CON-08-9
- 内容：
  1. 筛选：时间范围（必填，超过 31 天时"查询"按钮不可点并提示"最长 31 天"）、操作人、动作、目标、结果、`requestId`；游标分页"加载更多"。
  2. 详情：同一 `requestId` 的 intent / result 合并显示；只有 intent 的显示"结果未知"（AC-CON-08-9）；"查看 im-server 侧记录"按钮。
  3. 理由、参数摘要按纯文本显示。运维空间"全部空间"选项由 tianting-ops 实现，AC-CON-09-9 在本页只验证 `space=admin` 记录字段完整，跨空间部分在运维前端验收。

### FE-405 通知中心与个人设置（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-016、CS-007、CS-008
- AC：AC-CON-12-1、AC-CON-12-3、AC-CON-03-9
- 内容：
  1. 铃铛未读数（轮询带 `X-Console-Background: 1`，页面不可见时暂停），打开列表后标记已读（AC-CON-12-3）。
  2. 个人设置：修改密码（当前密码 + 动态码 + 新密码）、恢复码剩余数与重新生成（step-up）、退出登录。

---

## 业务页面（第一批）

### FE-410 用户搜索与详情（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-021
- AC：AC-ADM-10-1、AC-ADM-10-2、AC-ADM-10-4、AC-ADM-10-5、AC-ADM-17-1
- 内容：搜索框（空白不发请求，提示"请输入关键字"；无结果"没有找到用户"）、"只看官方账号"筛选、列表（userId、昵称、首字母头像、注册时间、是否在线、官方标记；第二批加"已封禁""禁言中"）；详情（资料、官方标记、在线状态、好友列表入口、"查看在线设备"、操作按钮按权限显示）。游标分页"加载更多"。

### FE-411 强制下线与在线设备（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-410；CS-021
- AC：AC-ADM-12-1、AC-ADM-12-3、AC-ADM-14-1
- 内容：强制下线确认框显示"将强制 u1 的 N 个在线设备下线"（N 来自是否在线 / 设备数）、理由输入（4～200 字符前端预校验）、提示"业务方重新签发凭证后用户即可再次登录；要阻止登录请使用封禁"；4042 显示"用户不存在"。在线设备：理由弹框 → `POST` 查询（userId、理由不进 URL 和浏览器历史）→ 表格（连接 ID、平台、建立时间、节点、脱敏 IP）；`partial` 时显示"在已响应的 N 个节点上找到 K 个连接；节点 X 未响应，结果可能不完整"，不显示"不在线"；4292 显示每日上限提示。

### FE-412 用户导入（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-022
- AC：AC-ADM-11-1、AC-ADM-11-2、AC-ADM-11-3、AC-ADM-11-4
- 内容：单个添加表单；CSV 上传 → 调 `validate` → 错误列表（行号 + 原因，最多 100 条，有错误时"确认"不可点）/ 预览前 20 行与总行数 → 确认 → 结果"已成功 N 批 / 共 M 批"（失败时显示失败码文案并说明可整文件重新提交）；5001 行提示"单个文件最多 5000 行"。

### FE-413 社交管理：好友列表、好友导入 / 删除、社交配置、官方账号（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-410；CS-023
- AC：AC-ADM-15-1、AC-ADM-15-2、AC-ADM-15-5、AC-ADM-15-7、AC-ADM-16-1、AC-ADM-16-2、AC-ADM-17-1、AC-ADM-17-2、AC-ADM-17-3
- 内容：
  1. 用户详情 → 好友列表（对方 userId、来源、建立时间，**没有备注列**）。
  2. 好友导入 / 删除页：CSV 上传、校验结果、确认框"将为 N 对用户建立双向好友关系，不受拉黑影响，也不解除拉黑"、理由、step-up、进度"已成功 N 批 / 共 M 批"、错误透传文案（"用户不存在：ghost"、"alice 的好友数已达上限"）；`reason: "notAvailable"` 显示"该功能尚未在服务端开放"。
  3. 社交配置页：参数表与当前值、编辑后显示"旧值 → 新值"差异；切换好友模式时影响说明 + 手动输入新模式名（不一致"确认"不可点，AC-ADM-16-2）。
  4. 官方账号：用户详情"设为 / 取消官方账号"，确认框说明"官方账号不受好友制和陌生人限额限制，用户仍可以拉黑它"、理由。

### FE-414 群列表与建群（1.5 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-024
- AC：AC-ADM-20-1、AC-ADM-20-2、AC-ADM-20-3、AC-ADM-21-1、AC-ADM-21-2、AC-ADM-21-3、AC-ADM-21-4
- 内容：群列表（groupId、群名、类型、群主、成员数、`historyVisible`、创建时间，默认按创建时间倒序，"包含已解散"勾选后标记"已解散"和解散时间）；建群表单（群 ID、群名 1～50、初始成员手填 / 粘贴 / CSV，提交前超过 2000 人提示"群成员最多 2000 人"且不发请求；`historyVisible` 默认关并说明含义；M4 起可选群主）；错误文案：4090"群 ID 已存在"、4042"用户不存在：{id}"、4004"群 ID 格式不正确"。

### FE-415 群详情、成员管理、群设置、群主与解散（2 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-414；CS-025
- AC：AC-ADM-22-1、AC-ADM-22-2、AC-ADM-22-3、AC-ADM-22-6、AC-ADM-23-1、AC-ADM-23-2、AC-ADM-23-3、AC-ADM-24-1、AC-ADM-24-6、AC-ADM-25-1、AC-ADM-30-1、AC-CON-07-3
- 内容：
  1. 详情：资料、设置、成员列表（每页 50、userId 前缀筛选、入群时间；M4 角色列与公告；第二批禁言状态）。
  2. 加 / 移成员（粘贴，移除需理由）；4130 显示"群成员最多 2000 人"。
  3. 群设置：群名（清空或 51 字时保存按钮不可点并提示，AC-ADM-23-3）、`historyVisible`（确认框说明"只影响之后的查询：打开后，所有成员（包括已入群的）都能看到入群前的历史"）、（M4）公告。
  4. 群主（M4）：设置 / 收回、理由；4046 显示"该用户不是群成员"。
  5. 解散：确认框写清影响、理由、手动输入群 ID（不一致"确认"不可点，AC-ADM-24-6）→ 4038 弹框 → 自动完成解散（AC-CON-07-3）。
  6. "查看消息元数据"入口跳 FE-416（群会话）。

### FE-416 会话消息元数据页（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-admin
- 依赖：FE-401；CS-026
- AC：AC-ADM-30-1、AC-ADM-30-2、AC-ADM-24-4
- 内容：输入会话 ID（`c2c:{a}:{b}` / `group:{gid}`，前端先做格式预校验）或从群详情进入；单聊先弹理由框；表格（seq、msgId、发送者、类型、正文长度、时间；第二批"已删除"标记、删除人、删除时间）；按发送者、时间范围筛选；seq 倒序"加载更多"，到底显示"没有更多了"；4007、4292 文案。**页面没有任何正文字段**。

---

## 业务页面（第二批）

### FE-420 封禁 / 解封（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-admin
- 依赖：FE-410；CS-040
- AC：AC-ADM-13-1、AC-ADM-13-5、AC-ADM-13-9
- 内容：用户详情"封禁"（时长 1 小时 / 1 天 / 7 天 / 30 天 / 永久、理由、step-up）与"解封"；按钮只对有 `user.ban` 的账号显示（`support` 看不到）；列表与详情显示"已封禁"和到期时间、理由、操作人（**只用 im-server 返回的时间**）。

### FE-421 群消息正文查看与水印（2 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-admin
- 依赖：FE-416；CS-041
- AC：AC-ADM-08-3、AC-ADM-08-5、AC-ADM-08-10
- 内容：元数据页"查看正文"→ 弹框（工单号、理由、当次动态码）→ 正文页（纯文本、seq 倒序加载）；任何请求收到 4039 都弹同一个框、不跳登录页（AC-ADM-08-5）；单聊显示"单聊正文查看未开启"；**覆盖整个消息区域的平铺水印层**（账号名 + 当前时间到分钟 + 授权 ID 前 8 位，每分钟更新）；不提供导出、复制按钮，消息区域 `user-select: none`（仅体验，不作为防泄露手段）。

### FE-422 删除消息与隔离原文查看（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-admin
- 依赖：FE-421；CS-042
- AC：AC-ADM-31-1、AC-ADM-32-2
- 内容：正文页单条 / 多选删除（≤ 50 条，同一会话；`type=tip` 不可选）、理由、step-up；删除后该条显示"该消息已被删除"和删除人、时间；元数据页"查看隔离原文"（流程同 FE-421，水印相同）；4044 显示"消息不存在或已过期"。

### FE-423 用户级与群成员禁言（1.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-admin
- 依赖：FE-410、FE-415；CS-043
- AC：AC-ADM-40-4、AC-ADM-41-4
- 内容：用户详情"禁言 / 解除禁言"（10 分钟、1 小时、1 天、7 天、30 天、永久、自定义 1 分钟～30 天；理由）；群成员列表行内"群内禁言 / 解除"，显示"禁言中"和到期时间；4042、4046 文案。

### FE-424 系统通知（2.5 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-admin
- 依赖：CS-044、CS-045；二期 push 协议定稿
- AC：AC-ADM-50-1、AC-ADM-50-3、AC-ADM-50-5、AC-ADM-50-8
- 内容：新建通知（目标：手填 / CSV 用户 ≤ 1000，或全员；模板选择后按 JSON Schema 渲染表单；预览目标人数 / "全员"、type、格式化 data）；全员草稿列表与批准（不能批准自己发起的，按钮隐藏，服务端 403 仍处理；输入"全员发送"+ step-up；4290 显示"全员通知间隔不少于 5 分钟"）；发送记录（`data` 全文、发送人、批准人、时间）。

---

## 客户端配合（SDK / H5）

> 这些 AC 的验证层是 sdk-ts / H5（或原生 SDK），不在 tianting-admin 仓库。编号放在本单是因为它们来自 PRD-admin；与 PRD-social、PRD-ui 的前端任务单有交集时以先开工的一方为准，另一方只做核对。

### FE-430 sdk-ts + H5：`convRemoved.reason`、忽略未知、"群聊已解散"（1 人日）
- 优先级：P0
- 批次：第一批
- 仓库：tianting-sdk-ts、tianting-web
- 依赖：BE-122 / BE-209、BE-211（Schema 与 vectors）；**FE-100（frontend-social.md）已含 sdk-ts 的 `convRemoved.reason` 解析与忽略未知事件**，本任务的 SDK 部分只做核对，主要工作是 H5 文案与验收
- AC：AC-ADM-24-1、AC-ADM-24-5、AC-ADM-24-9
- 内容：sdk-ts 解析 `convRemoved.reason`（`removed` / `dissolved`，未知值按 `removed`），对不认识的事件类型、字段、枚举值静默忽略（跑 `vectors/frames.json` 新用例）；`dissolved` 时从会话列表移除（内存）。H5：正打开该群时提示"群聊已解散"并禁用输入框；未知 reason（如 `left`）显示"你已不在该群"；刷新 / 重新登录后会话列表没有该群。Playwright 覆盖。

### FE-431 sdk-ts + H5：封禁提示（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-ts、tianting-web
- 依赖：BE-221、BE-222
- AC：AC-ADM-13-11、AC-ADM-13-1
- 内容：`kicked {reason: "suspended", until}` → 不重连；H5 显示"账号已被封禁，解封时间：{时间}"（永久显示"账号已被封禁"）后回到登录页；`/demo/login` 返回 4034 时显示同一文案（`details.bannedUntil`）。老版本按 4012 处理"账号已被下线"。

### FE-432 sdk-ts + H5：消息删除（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-ts、tianting-web
- 依赖：BE-221、BE-223
- AC：AC-ADM-31-1、AC-ADM-31-3、AC-ADM-31-5
- 内容：处理 `msgDeleted {convId, msgIds, seqs}` 与 `sync` 中 `deleted=true` 的消息：替换为"该消息已被删除"，**清除内存中保存的原文**（SDK 调试接口导出消息列表搜不到特征串）；会话摘要 `lastMsg.deleted` 显示"该消息已被删除"；未读数不变。

### FE-433 sdk-ts + H5：禁言（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-ts、tianting-web
- 依赖：BE-221、BE-226、BE-227
- AC：AC-ADM-40-4、AC-ADM-40-5、AC-ADM-40-6、AC-ADM-41-6
- 内容：处理 `muted` / `unmuted`（`scope: user | group`）、`auth` ack 的 `mutedUntil`、`convs` 群会话项的 `mutedUntil`、发送 ack 4031 的 `details`；H5 输入框禁用并显示"你已被禁言，解除时间：{时间}"（永久"你已被禁言"），群内禁言只影响该群；到期按服务端时间自动恢复。

### FE-434 sdk-ts + H5：系统推送 `pushes` 流（1 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-ts、tianting-web
- 依赖：二期 push 协议定稿、BE-228、BE-229
- AC：AC-ADM-50-1、AC-ADM-50-3
- 内容：SDK 暴露 `pushes` 流（按 `type` 分发给业务代码）、按 `pushId` 去重；H5 演示页显示收到的通知。随 push 协议修改。

### FE-435 sdk-kotlin：第二批协议首版支持（M5）（3 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-kotlin
- 依赖：BE-221；M5 首版
- AC：AC-ADM-31-4
- 内容：首版直接包含 `convRemoved.reason`（删除本地库中该会话）、`kicked.suspended`、`msgDeleted`（**删除本地数据库中的原文**，验收时检查数据库文件搜不到特征串）、`muted` / `unmuted` / `mutedUntil`、4031 `details`、忽略未知；跑共享 `vectors/frames.json`。

### FE-436 sdk-swift：第二批协议首版支持（M6）（3 人日）
- 优先级：P0
- 批次：第二批
- 仓库：tianting-sdk-swift
- 依赖：BE-221；M6 首版
- AC：AC-ADM-31-4
- 内容：同 FE-435（Swift 实现，本地库为 SQLite / GRDB，删除后执行 `VACUUM` 或使用 `secure_delete` 保证数据库文件中搜不到原文）。

---

## P1

### FE-440 数据概览（1 人日）
- 优先级：P1
- 批次：P1
- 仓库：tianting-admin
- 依赖：CS-060
- AC：AC-ADM-70-1、AC-ADM-70-2
- 内容：首页卡片（在线连接数按平台、今日新增用户、今日消息数、群总数），每 60 秒刷新，刷新请求带 `X-Console-Background: 1`、页面不可见时暂停；会话过期（4014）时跳登录页。
