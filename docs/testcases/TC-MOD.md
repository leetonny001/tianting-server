# 测试用例：敏感词和举报（MOD，C6）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4.1、4.3、4.5 节，第 10 节 F-MOD-01～04，第 11～15 节的 C6 部分；[PRD-admin v1.4](../prd/PRD-admin-v1.4.md) 第 5 节、F-KEY-02、F-ADM-08、F-ADM-30、第 12 节。任务单：[backend-chat](../tasks/backend-chat.md) BE-491～BE-499，[console-chat](../tasks/console-chat.md) CS-206～CS-208，[frontend-chat](../tasks/frontend-chat.md) FE-690～FE-698。
> 范围：标了【C6】的 AC-MOD（Frank 2026-11-11"开始 C6"，QUEUE Q30）。同属 C6 的 AC-ACC-01-9、AC-ACC-10-5 在 [TC-ACC](TC-ACC.md)（TC-ACC-01-22、TC-ACC-10-05），AC-CHAT-90-4 和【全部】的 C6 部分在 [TC-CHAT](TC-CHAT.md)（TC-CHAT-90-40～47）。**C6 没有依赖 C3 的用例**（backend-chat 第 94 条）。
>
> 通用约定：
> - **不放真实词库**（F-MOD-01 规则 5）：词只用特征词 `违规词`、`badword`、`ZXQWORD` 和 `ZXQ-*` 特征串；需要大词库的用例在测试里现场随机生成，不落盘、不提交。截图里也只出现特征词。
> - 环境：**E1**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库，`IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=1`、测试里现场生成的 `IM_REPORT_KEY`；调用方 `console` 带 `moderation.words`、`report.read`、`report.handle`、`msg.meta` 等，`console-msgread`（`bodyAccess`）带 `msg.body.group`、`msg.quarantine`、`report.evidence`；`IM_C2C_BODY_ACCESS` 默认 `off`）；**E1 + 假 S3**（再加 `objstoretest`，`IM_FILES_ENABLED=1`）；**E1 + C4**（再加 `IM_MSG_ACTIONS_ENABLED=1` 和现场生成的 `IM_QUARANTINE_KEY`）；**E1 + console-server**（`CON_TEST_MODE=1`，Postgres，后台账号 `sup`（`support`）、`op`（`operator`）、`mod`（`moderator`）、`sa`（`super_admin`））；**E1 双节点**（现有 `twoNodes`，共享 SQLite，测试模式 Invalidator 互发）；**启动参数单独构造**（只验启动检查）。
> - **CI 现在不运行（PENDING P-21）**，结果都是本地结果，记录写明环境和提交号。真机在 P-13，性能机在 P-12。
> - 层级：**后端 e2e**（Go e2e，对 `/ws`、`/v1/*`、`/admin/*` 断言）、**console e2e**（console-server 对真实 im-server）、**SDK 单测**、**Playwright**（H5 或管理后台）、**原生 UI 测试**（Compose UI Test、XCUITest）、**端到端**（真实 im-server + 客户端或后台）。标了 Playwright（H5）和原生 UI 测试的用例 H5、Android、iOS 都要跑。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算。限流用例之间用 `/admin/test/limits/reset` 清计数。"10 秒后"用真实等待（规则 2 的生效时间，Invalidator 广播）。
> - 特征串：`ZXQWORD`（词库审计）、`ZXQ-RPT-1`（群消息证据）、`ZXQ-RPT-2`（单聊证据）、`ZXQ-MOD-NOTE`（举报补充说明）、`ZXQ-MOD-FILE`（文件名）。"搜不到"指对原始字节做子串搜索（SQLite 先 `PRAGMA wal_checkpoint(TRUNCATE)`；日志、im-server 审计 `admin_audit`、console 审计都搜）。
> - 记号：`u1`、`u2` 单聊双方；群 `g1`：群主 `o`、成员 `u1`、`u2`、`u3`；`u9` 和谁都没有关系；负向的"收不到任何帧"用哨兵法（之后给同一用户发一条普通消息，收到它之后断言前面没有别的帧）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-MOD-01-01 | 后台添加"违规词"，10 秒后发送被拦截、不落库 | AC-MOD-01-1 | 后端 e2e | P0 |
| TC-MOD-01-02 | 停用后 10 秒发送成功 | AC-MOD-01-2 | 后端 e2e | P0 |
| TC-MOD-01-03 | 双节点：节点 1 添加，10 秒后节点 2 发送被拦截 | AC-MOD-01-3 | 后端 e2e | P0 |
| TC-MOD-01-04 | 导入 5001 条 400 / 4131；含空行和重复的 100 条去重后入库 | AC-MOD-01-4 | 后端 e2e | P0 |
| TC-MOD-01-05 | support 添加词：403 / 4030 | AC-MOD-01-5 | console e2e | P0 |
| TC-MOD-01-06 | 添加 ZXQWORD：两边审计有记录、搜不到词 | AC-MOD-01-6 | 后端 e2e + console e2e | P0 |
| TC-MOD-01-07 | 词库页：搜索、导入结果汇总、停用；纯文本 | AC-MOD-01-7 | Playwright（管理后台） | P0 |
| TC-MOD-01-08 | 重新加载失败沿用旧词库；启动加载失败退出；词库为空告警 | AC-MOD-01-8 | 后端 e2e | P0 |
| TC-MOD-01-09 | 导入 65 个码点的词：跳过、计入无效 | AC-MOD-01-9 | 后端 e2e | P0 |
| TC-MOD-01-10 | 生产 + IM_MOD_ENABLED=1 + V13 关：拒绝启动 | AC-MOD-01-10 | 后端单测 + e2e | P0 |
| TC-MOD-02-01 | 单聊、群聊发送含拦截词：4510，别人收不到，不落库 | AC-MOD-02-1 | 后端 e2e | P0 |
| TC-MOD-02-02 | 全角、变音、大小写、零宽、空格、标点的 badword 都被拦截 | AC-MOD-02-2 | 后端 e2e | P0 |
| TC-MOD-02-03 | 带全角字符的普通文本：u2 收到原文 | AC-MOD-02-3 | 后端 e2e | P0 |
| TC-MOD-02-04 | 文件名含拦截词：图片、文件都 4510 | AC-MOD-02-4 | 后端 e2e | P0 |
| TC-MOD-02-05 | 昵称、用户名、群名、群公告、好友附言含拦截词：4510，未写入，不泄露存在性 | AC-MOD-02-5 | 后端 e2e | P0 |
| TC-MOD-02-06 | IM_CLIENT_CUSTOM_MSG=0：客户端 custom 4000 customDisabled | AC-MOD-02-6 | 后端 e2e | P0 |
| TC-MOD-02-07 | 被拉黑、被禁言的发送拦截词：4036、4031，不是 4510 | AC-MOD-02-7 | 后端 e2e | P0 |
| TC-MOD-02-08 | 命中一次：moderation_hits 有词 ID 和场景，搜不到全文 | AC-MOD-02-8 | 后端 e2e | P0 |
| TC-MOD-02-09 | 客户端收到 4510：标记失败、输入框清空、提示、不重发、不含词 | AC-MOD-02-9 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-MOD-02-10 | IM_MOD_ENABLED=0：含拦截词的文本发送成功 | AC-MOD-02-10 | 后端 e2e | P0 |
| TC-MOD-02-11 | IM_CLIENT_CUSTOM_MSG=1：custom 发送成功 | AC-MOD-02-11 | 后端 e2e | P0 |
| TC-MOD-02-12 | 新对象单聊命中：4510，发送令牌减 1，newPeerReject 退还 | AC-MOD-02-12 | 后端 e2e | P0 |
| TC-MOD-02-13 | 同场景 4048、4096、4040：newPeerReject 都退还 | AC-MOD-02-13 | 后端 e2e | P0 |
| TC-MOD-02-14 | 已保存的消息重发成功；后台按用户查次数；91 天后清除 | AC-MOD-02-14 | 后端 e2e | P0 |
| TC-MOD-02-15 | 生产 + IM_CLIENT_CUSTOM_MSG=1：配置检查有告警 | AC-MOD-02-15 | 后端 e2e | P0 |
| TC-MOD-02-16 | @ 所有人含拦截词：4510，次数不变，令牌减 1 | AC-MOD-02-16 | 后端 e2e | P0 |
| TC-MOD-03-01 | u2 举报 u1 的消息：成功，后台队列出现，u1 收不到帧 | AC-MOD-03-1 | 端到端 + 后端 e2e | P0 |
| TC-MOD-03-02 | 非参与者、其他会话、入群前、21 条：4040 相同响应；4131 | AC-MOD-03-2 | 后端 e2e | P0 |
| TC-MOD-03-03 | 同一消息举报 3 次：都成功，只有 1 条记录 | AC-MOD-03-3 | 后端 e2e | P0 |
| TC-MOD-03-04 | 当天第 21 次：4292 | AC-MOD-03-4 | 后端 e2e | P0 |
| TC-MOD-03-05 | 群消息举报后撤回：原始字节搜不到 ZXQ-RPT-1，审核员读到原文 | AC-MOD-03-5 | 后端 e2e + console e2e | P0 |
| TC-MOD-03-06 | 推进 181 天：report_purge 物理删除证据 | AC-MOD-03-6 | 后端 e2e | P0 |
| TC-MOD-03-07 | 举报没关联的、不存在的用户：响应相同 4040 | AC-MOD-03-7 | 后端 e2e | P0 |
| TC-MOD-03-08 | note 含特征串：日志和审计搜不到 | AC-MOD-03-8 | 后端 e2e + console e2e | P0 |
| TC-MOD-03-09 | 客户端长按举报：提交前提示、成功轻提示、单聊询问拉黑 | AC-MOD-03-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-MOD-03-10 | 单聊 off 时举报：只有元数据；打开开关后仍读不到正文 | AC-MOD-03-10 | 后端 e2e | P0 |
| TC-MOD-03-11 | 撤回提示行举报：成功，审核员读到隔离原文（群） | AC-MOD-03-11 | Playwright（H5）+ 原生 UI 测试 + Playwright（管理后台） | P0 |
| TC-MOD-03-12 | 没读到的已撤回消息 4040；隔离原文过期的成功、只有元数据 | AC-MOD-03-12 | 后端 e2e | P0 |
| TC-MOD-03-13 | 举报群：非成员 4040、成员成功、reason 和 note 不合法 400 | AC-MOD-03-13 | 后端 e2e | P0 |
| TC-MOD-03-14 | 证据写失败 500 无记录；开关关 403 featureDisabled | AC-MOD-03-14 | 后端 e2e | P0 |
| TC-MOD-03-15 | 被举报的文件：保留期内照常发放；过期后普通用户 4048，证据仍可看 | AC-MOD-03-15 | 后端 e2e | P0 |
| TC-MOD-04-01 | 审核员：读证据、删除消息、结案，审计顺序 | AC-MOD-04-1 | 端到端（管理后台 Playwright）+ console e2e | P0 |
| TC-MOD-04-02 | support、operator 读队列、结案：403 | AC-MOD-04-2 | console e2e | P0 |
| TC-MOD-04-03 | 单聊 off：申请证据授权 403 c2cBodyDisabled，元数据照常 | AC-MOD-04-3 | 后端 e2e + console e2e | P0 |
| TC-MOD-04-04 | console 主凭证调用 report.evidence 接口：403 | AC-MOD-04-4 | 后端 e2e | P0 |
| TC-MOD-04-05 | 两个审核员同时结案：一个成功，一个 4098 | AC-MOD-04-5 | 后端 e2e + console e2e | P0 |
| TC-MOD-04-06 | 同一用户被 3 人举报：列表聚合"3 次"，纯文本 | AC-MOD-04-6 | Playwright（管理后台） | P0 |
| TC-MOD-04-07 | 结案：举报人和被举报人收不到帧 | AC-MOD-04-7 | 后端 e2e | P0 |
| TC-MOD-04-08 | console 同时带 report.evidence 和 msg.meta：拒绝启动 | AC-MOD-04-8 | 后端单测 + e2e | P0 |
| TC-MOD-04-09 | super_admin 没有应急授权读证据：403 | AC-MOD-04-9 | console e2e | P0 |
| TC-MOD-04-10 | 单聊举报：不填理由 400、第 51 个 429、没有授权时没有 note 和文件名 | AC-MOD-04-10 | console e2e + 后端 e2e | P0 |
| TC-MOD-04-11 | 客服 msg.meta 查文件消息：两层响应搜不到文件名 | AC-MOD-04-11 | console e2e + 后端 e2e | P0 |
| TC-MOD-04-12 | 被举报的图片同源加载、html 只能下载，生产 CSP 没有违规 | AC-MOD-04-12 | Playwright（管理后台）+ console e2e | P0 |
| TC-MOD-04-13 | 单聊文件证据 off：申请文件 403 c2cBodyDisabled | AC-MOD-04-13 | 后端 e2e + console e2e | P0 |
| TC-MOD-04-14 | 密钥不匹配显示无法解密、可以结案；审计写失败 503 未结案 | AC-MOD-04-14 | 后端 e2e + console e2e | P0 |
| TC-MOD-04-15 | 文件名含 `<script>` 的证据：纯文本 | AC-MOD-04-15 | Playwright（管理后台） | P0 |
| TC-MOD-04-16 | 群文件 fetch：没有授权 403 / 4039，有授权成功 | AC-MOD-04-16 | 后端 e2e | P0 |
| TC-MOD-04-17 | 181 天前的 account_events：清除 | AC-MOD-04-17 | 后端 e2e | P0 |

---

## F-MOD-01 词库

### TC-MOD-01-01 后台添加"违规词"，10 秒后发送被拦截、不落库
- 关联 AC：AC-MOD-01-1
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1、u2 是好友，单聊有消息
- 步骤：
  1. `console` 调 `POST /admin/moderation/words {word:"违规词"}`
  2. 等 10 秒，u1 `send {type:text, text:"这是违规词"}`
  3. u1 `sync` 这个会话；管理接口查元数据
- 预期结果：
  1. 第 1 步成功，返回 `wordId`
  2. 第 2 步 ack 错误 4510，没有 `details`，错误信息不含"违规词"
  3. 消息不在 `sync` 和元数据里（不落库），`conv_seq` 没有增加

### TC-MOD-01-02 停用后 10 秒发送成功
- 关联 AC：AC-MOD-01-2
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：同 TC-MOD-01-01，"违规词"已生效
- 步骤：
  1. `PATCH /admin/moderation/words/{id} {state:"disabled"}`
  2. 等 10 秒，u1 发"这是违规词"
- 预期结果：
  1. 发送成功，u2 收到原文
  2. 再启用、等 10 秒后发送：4510

### TC-MOD-01-03 双节点：节点 1 添加，10 秒后节点 2 发送被拦截
- 关联 AC：AC-MOD-01-3
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 双节点
- 前置条件：u1 连节点 2；两个节点都已加载（空）词库
- 步骤：
  1. 经节点 1 的管理接口添加"违规词"
  2. 等 10 秒，u1 经节点 2 发"这是违规词"
- 预期结果：
  1. 4510，消息不落库
  2. 节点 2 的重建是 Invalidator 主题 `blockedWords` 触发的（不是等 60 秒 TTL）：断言发生在 10 秒内

### TC-MOD-01-04 导入 5001 条 400 / 4131；含空行和重复的 100 条去重后入库
- 关联 AC：AC-MOD-01-4
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：词库为空；测试现场生成特征词 `ZXQ-W-0001`～
- 步骤：
  1. `POST /admin/moderation/words/import {words:[5001 个]}`
  2. 导入 100 条：80 个不同的词、10 个空行、10 个和前面重复的（其中 2 个只是大小写或全角不同）
  3. 查词库
- 预期结果：
  1. 第 1 步 400 / 4131，词库没有变化
  2. 第 2 步成功，结果 `{added:80, duplicate:10, invalid:10}`（全角、大小写不同的按标准化后去重，backend-chat 第 100 条）
  3. 词库正好 80 个

### TC-MOD-01-05 support 添加词：403 / 4030
- 关联 AC：AC-MOD-01-5
- 优先级：P0
- 批次：C6
- 层级：console e2e
- 环境：E1 + console-server
- 前置条件：后台账号 `sup`（只有 `support`）、`op`、`mod`
- 步骤：
  1. `sup` 调 `POST /console/api/moderation/words`、`.../import`、`PATCH`、`DELETE`
  2. `op`、`mod` 各添加一个词
- 预期结果：
  1. `sup` 全部 403 / 4030，im-server 没有收到调用，词库没有变化
  2. `op`、`mod` 成功（`moderation.words` 给运营和审核，PRD-chat 第 14.1 节）

### TC-MOD-01-06 添加 ZXQWORD：两边审计有记录、搜不到词
- 关联 AC：AC-MOD-01-6
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + console-server
- 前置条件：—
- 步骤：
  1. `mod` 在后台添加 `ZXQWORD`，再导入 3 个含 `ZXQWORD` 前缀的词，再停用、删除其中一个
  2. 查 console 审计（`GET /console/api/audit`）和 im-server 的 `admin_audit`、两边的日志
- 预期结果：
  1. 每个操作都有审计记录（动作、操作人、`wordId` 或数量）
  2. 审计、日志、console 库里都搜不到 `ZXQWORD`

### TC-MOD-01-07 词库页：搜索、导入结果汇总、停用；纯文本
- 关联 AC：AC-MOD-01-7
- 优先级：P0
- 批次：C6
- 层级：Playwright（管理后台）
- 环境：E1 + console-server + tianting-admin（生产 CSP 响应头）
- 前置条件：`op` 登录；词库已有 `<b>ZXQ-W-1</b>` 这样带标签的特征词
- 步骤：
  1. 打开"词库"，搜索 `ZXQ-W`
  2. 选择一个 txt（20 行：15 个新词、3 个重复、2 个空行）导入
  3. 停用一个词
- 预期结果：
  1. 搜索结果正确；`<b>…</b>` 按原样显示为文字，不是粗体
  2. 结果汇总"新增 15、重复 3、无效 2"
  3. 停用后状态更新；选 5001 行的文件时不提交，提示"一次最多 5000 条"

### TC-MOD-01-08 重新加载失败沿用旧词库；启动加载失败退出；词库为空告警
- 关联 AC：AC-MOD-01-8
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1；第 2 种用启动参数单独构造
- 前置条件："违规词"已生效
- 步骤：
  1. 故障注入 `wordsReload`，再添加 `badword`，等 10 秒；分别发"违规词"和 `badword`；抓 `/metrics`
  2. `IM_MOD_ENABLED=1` 加故障注入 `wordsStartupLoad` 启动
  3. 清空词库，查 `GET /internal/v1/config` 的配置检查，发"违规词"
- 预期结果：
  1. "违规词"仍 4510（旧词库生效），`badword` 成功（新词没加载上）；`im_mod_reload_errors_total` 增加；解除注入后 10 秒内 `badword` 也被拦截
  2. 非 0 退出
  3. 配置检查有"词库为空"告警；发送成功（空词库不拦截）

### TC-MOD-01-09 导入 65 个码点的词：跳过、计入无效
- 关联 AC：AC-MOD-01-9
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：—
- 步骤：
  1. 导入 `[64 个码点的词, 65 个码点的词]`（含中文和 emoji，按码点数）
  2. 单个添加 65 个码点的词
- 预期结果：
  1. 结果 `{added:1, invalid:1}`，词库只有 64 码点的那个
  2. 400 / 4000（backend-chat 第 102 条）

### TC-MOD-01-10 生产 + IM_MOD_ENABLED=1 + V13 关：拒绝启动
- 关联 AC：AC-MOD-01-10
- 优先级：P0
- 批次：C6
- 层级：后端单测 + e2e
- 环境：启动参数单独构造
- 前置条件：`IM_ENV=production`，其余生产必需项（含 `IM_REPORT_KEY`、`IM_PRIVACY_NOTICE_VERSION`）都合法
- 步骤：
  1. `IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=0` 启动
  2. 改成 `IM_V13_ADMIN_FEATURES=1` 启动
  3. `IM_MOD_ENABLED=1` 但不配 `IM_REPORT_KEY`，或它等于 `IM_QUARANTINE_KEY`（development 环境）
- 预期结果：
  1. 非 0 退出，错误信息指出缺 `IM_V13_ADMIN_FEATURES`
  2. 正常启动
  3. 非 0 退出，不打印任何密钥（backend-chat 第 99 条）

---

## F-MOD-02 发送检查

### TC-MOD-02-01 单聊、群聊发送含拦截词：4510，别人收不到，不落库
- 关联 AC：AC-MOD-02-1
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件："违规词"已生效；u1、u2 单聊；g1
- 步骤：
  1. u1 给 u2 发"违规词"；u1 在 g1 发"违规词"
  2. 2 秒后 u1 给 u2、在 g1 各发一条普通消息（哨兵）
- 预期结果：
  1. 两次都 4510
  2. u2、u3、o 只收到哨兵，之前没有任何 `msg` 帧
  3. 两个会话的 `sync` 和元数据都没有被拦截的消息

### TC-MOD-02-02 全角、变音、大小写、零宽、空格、标点的 badword 都被拦截
- 关联 AC：AC-MOD-02-2
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：词库有 `badword`
- 步骤：
  1. 分别发 `ＢＡＤＷＯＲＤ`、`bádwörd`、`BadWord`、`bad​word`、`bad word`、`bad-word`（各自放在一句普通话中间）
- 预期结果：
  1. 6 条都是 4510
  2. 另发 `bad` 和 `word` 分在两条消息里：都成功（只在单条消息内匹配）

### TC-MOD-02-03 带全角字符的普通文本：u2 收到原文
- 关联 AC：AC-MOD-02-3
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：词库有 `badword`
- 步骤：
  1. u1 发 `ＡＢＣ　１２３，ｇｏｏｄ​！`（全角字母、全角空格、零宽字符）
  2. u2 接收；u2 `sync`
- 预期结果：
  1. 发送成功
  2. u2 收到的 `text` 和发送的逐字节相同（没有被标准化），`sync` 也一样

### TC-MOD-02-04 文件名含拦截词：图片、文件都 4510
- 关联 AC：AC-MOD-02-4
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：词库有"违规词"；u1 已上传并 `complete` 一张图片和一个文件，名字分别是 `违规词.png`、`ＢＡＤＷＯＲＤ.pdf`（词库另有 `badword`）
- 步骤：
  1. u1 `send {type:image}`、`send {type:file}` 引用这两个 `fileId`
  2. 再发一个文件，上行 `body.file.name` 写成普通名字，但服务端记录的名字含拦截词
- 预期结果：
  1. 都是 4510，消息不保存，文件仍是 `ready`（没有绑定）
  2. 第 2 步也是 4510（查服务端记录的文件名，backend-chat 第 111 条）

### TC-MOD-02-05 昵称、用户名、群名、群公告、好友附言含拦截词：4510，未写入，不泄露存在性
- 关联 AC：AC-MOD-02-5
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（`invite` 模式，有邀请码）
- 前置条件：词库有"违规词"、`badword`；已注册用户名 `takenuser`
- 步骤：
  1. 注册：昵称"违规词"；用户名 `badwordx`；用户名 `takenuser` + 昵称"违规词"
  2. u1 `PATCH /v1/account/profile {nickname:"违规词"}`
  3. u1 `groupCreate {name:"违规词群"}`；o 对 g1 `groupUpdate {name:"违规词"}`、`groupUpdate {notice:"违规词"}`；`groupCreate` 带含词的公告
  4. u1 对 u2 发 `friendApply {note:"违规词"}`；对不存在的 `nouser` 发同样的申请
- 预期结果：
  1. 注册都是 451 / 4510，没有账号写入，邀请码次数不变；`takenuser` 那一次也是 4510（不泄露用户名已存在，backend-chat 第 112 条）
  2. 4510，昵称不变
  3. 都是 4510，没有建群，g1 名字和公告不变
  4. 两次申请的响应逐字节相同（4510），u2 没有申请记录
  5. 管理接口改昵称、改群名含"违规词"：成功（规则 3 不查管理接口）

### TC-MOD-02-06 IM_CLIENT_CUSTOM_MSG=0：客户端 custom 4000 customDisabled
- 关联 AC：AC-MOD-02-6
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（`IM_CLIENT_CUSTOM_MSG=0`）
- 前置条件：—
- 步骤：
  1. u1 `send {type:custom, data:{…}}`
  2. 管理接口用 `custom` 发系统消息（如果有这个路径）
- 预期结果：
  1. 400 / 4000，`reason=customDisabled`，消息未保存
  2. 管理接口不受影响

### TC-MOD-02-07 被拉黑、被禁言的发送拦截词：4036、4031，不是 4510
- 关联 AC：AC-MOD-02-7
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u2 拉黑了 u1；u3 在 g1 被禁言；词库有"违规词"
- 步骤：
  1. u1 给 u2 发"违规词"
  2. u3 在 g1 发"违规词"
  3. 查 `moderation_hits`
- 预期结果：
  1. 依次 4036、4031
  2. 没有命中记录（检查在权限判定之后，不暴露词库）

### TC-MOD-02-08 命中一次：moderation_hits 有词 ID 和场景，搜不到全文
- 关联 AC：AC-MOD-02-8
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：词库有"违规词"
- 步骤：
  1. u1 发"ZXQ-MOD-HIT 违规词"
  2. 查 `moderation_hits` 表和日志
- 预期结果：
  1. 有一行：`user_id=u1`、`scene=text`、`word_id`、时间
  2. 表、日志、审计都搜不到 `ZXQ-MOD-HIT`；`im_blocked_total{scene="text"}` 加 1

### TC-MOD-02-09 客户端收到 4510：标记失败、输入框清空、提示、不重发、不含词
- 关联 AC：AC-MOD-02-9
- 优先级：P0
- 批次：C6
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 三端；SDK 单测用 `FakeTransport`
- 前置条件：词库有"违规词"
- 步骤：
  1. 在聊天页输入"违规词"，发送
  2. 等 30 秒；冷启动（原生）后再看这条消息
- 预期结果：
  1. 消息在列表标记失败；输入框清空；提示"消息包含不当内容，无法发送"；提示和错误里没有"违规词"
  2. 没有自动重发（`FakeTransport` 没有第二次 `send`）；失败气泡没有"重发"，有"复制""删除"（frontend-chat 第 81 条）

### TC-MOD-02-10 IM_MOD_ENABLED=0：含拦截词的文本发送成功
- 关联 AC：AC-MOD-02-10
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（`IM_MOD_ENABLED=0`）
- 前置条件：词库表里有"违规词"（开关开时加的）
- 步骤：
  1. u1 发"违规词"
  2. `GET /v1/config`
- 预期结果：
  1. 成功，u2 收到
  2. `features.moderation=false`

### TC-MOD-02-11 IM_CLIENT_CUSTOM_MSG=1：custom 发送成功
- 关联 AC：AC-MOD-02-11
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（`IM_CLIENT_CUSTOM_MSG=1`，开发环境默认）
- 前置条件：—
- 步骤：
  1. u1 `send {type:custom, data:{k:"v"}}`
- 预期结果：
  1. 成功，u2 收到，行为和 C6 之前相同（现有 e2e 不改代码全过）

### TC-MOD-02-12 新对象单聊命中：4510，发送令牌减 1，newPeerReject 退还
- 关联 AC：AC-MOD-02-12
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（社交配置开启新对象配额）
- 前置条件：u1 和 u5 没有单聊；记下 u1 的发送令牌和 `newPeerReject` 余量（测试接口）
- 步骤：
  1. u1 给 u5 发"违规词"
  2. 再查令牌和余量
- 预期结果：
  1. 4510
  2. 发送令牌减 1；`newPeerReject` 和 `newPeerHour` 都退还（不变）

### TC-MOD-02-13 同场景 4048、4096、4040：newPeerReject 都退还
- 关联 AC：AC-MOD-02-13
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + 假 S3 + C4
- 前置条件：同 TC-MOD-02-12
- 步骤：
  1. u1 给 u5 发引用不存在 `fileId` 的图片（4048）
  2. 发已经绑定过别的消息的 `fileId`（4096）
  3. 发引用看不到的消息的 `quote`（4040）
- 预期结果：
  1. 依次 4048、4096、4040
  2. 每次之后 `newPeerReject` 都退还

### TC-MOD-02-14 已保存的消息重发成功；后台按用户查次数；91 天后清除
- 关联 AC：AC-MOD-02-14
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 已发出 `clientMsgId=c1` 的"今天 ZXQ-NEW"并保存
- 步骤：
  1. 后台添加 `ZXQ-NEW`，等 10 秒；u1 用同一 `clientMsgId=c1` 重发
  2. u1 再发 3 条含 `ZXQ-NEW` 的新消息（不同 `clientMsgId`）
  3. `GET /admin/moderation/hits?userId=u1`
  4. [IM 钟] 推进 91 天，运行 `moderation_hits_purge`，再查
- 预期结果：
  1. 重发成功，返回第一次的 ack（去重优先）
  2. 3 条都是 4510
  3. 按场景 `text: 3`，最近记录 3 条，没有任何内容（backend-chat 第 114 条）
  4. 清除后次数为 0

### TC-MOD-02-15 生产 + IM_CLIENT_CUSTOM_MSG=1：配置检查有告警
- 关联 AC：AC-MOD-02-15
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：启动参数单独构造（`IM_ENV=production`，其余合法）
- 前置条件：—
- 步骤：
  1. `IM_CLIENT_CUSTOM_MSG=1` 启动，查 `GET /internal/v1/config`
  2. 不设 `IM_CLIENT_CUSTOM_MSG` 启动，再查
- 预期结果：
  1. 能启动；配置检查的 `moderation` 项有"`custom` 已打开"告警
  2. 默认是关，没有这条告警

### TC-MOD-02-16 @ 所有人含拦截词：4510，次数不变，令牌减 1
- 关联 AC：AC-MOD-02-16
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + C4
- 前置条件：o 是 g1 群主；记下 g1 的 @ 所有人次数和 o 的发送令牌
- 步骤：
  1. o 发 `text:"@所有人 违规词", mentions:{all:true}`
- 预期结果：
  1. 4510
  2. @ 所有人次数不变（在保存事务里才扣）；发送令牌减 1
  3. 成员没有 `mentionUnread` 变化

---

## F-MOD-03 举报提交

### TC-MOD-03-01 u2 举报 u1 的消息：成功，后台队列出现，u1 收不到帧
- 关联 AC：AC-MOD-03-1
- 优先级：P0
- 批次：C6
- 层级：端到端 + 后端 e2e
- 环境：E1 + console-server + 三端（端到端）；后端 e2e 用 E1
- 前置条件：u1 给 u2 发了一条单聊消息 m1；u1 在线
- 步骤：
  1. u2 `report {targetType:"message", convId, msgIds:[m1], reason:"abuse"}`（端到端：H5、Android、iOS 各从消息菜单提交一次）
  2. 2 秒后给 u1 发哨兵消息
  3. `mod` 打开后台举报队列
- 预期结果：
  1. ack `{reportId}`
  2. u1 只收到哨兵，之前没有任何帧（被举报人不收通知，规则 7）
  3. 队列里有这条，状态 `pending`，理由"辱骂骚扰"

### TC-MOD-03-02 非参与者、其他会话、入群前、21 条：4040 相同响应；4131
- 关联 AC：AC-MOD-03-2
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：g1 `historyVisible=false`，u4 在 seq 20 之后入群；u1、u2 单聊消息 m1；g1 消息 g1m5（seq 5）
- 步骤：
  1. u9 举报 u1、u2 单聊的 m1
  2. u2 用 `convId=g1` 举报 m1
  3. u4 举报 g1m5
  4. u2 一次举报 21 条
- 预期结果：
  1. 前三个都是 404 / 4040 `reason=reportTargetInvalid`，响应逐字节相同
  2. 第四个 400 / 4131
  3. 都没有产生记录，也都计入每天的次数（backend-chat 第 115 条，第 4 个在格式阶段被拒，不计）

### TC-MOD-03-03 同一消息举报 3 次：都成功，只有 1 条记录
- 关联 AC：AC-MOD-03-3
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 给 u2 的消息 m1
- 步骤：
  1. u2 举报 m1 三次（理由分别 `abuse`、`spam`、`abuse`）
  2. 查后台队列；查当天次数
  3. [IM 钟] 推进 25 小时后再举报一次
- 预期结果：
  1. 三次都成功，返回同一个 `reportId`
  2. 只有 1 条记录（理由是第一次的）；当天次数只算 1 次
  3. 第 3 步产生新记录

### TC-MOD-03-04 当天第 21 次：4292
- 关联 AC：AC-MOD-03-4
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（`IM_REPORT_DAILY` 默认 20）
- 前置条件：u2 能看到 21 条不同的消息
- 步骤：
  1. u2 分别举报 20 条
  2. 举报第 21 条
  3. [IM 钟] 推进到次日 00:00 后再举报
- 预期结果：
  1. 前 20 次成功
  2. 第 21 次 429 / 4292，没有记录
  3. 次日成功

### TC-MOD-03-05 群消息举报后撤回：原始字节搜不到 ZXQ-RPT-1，审核员读到原文
- 关联 AC：AC-MOD-03-5
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + C4 + console-server
- 前置条件：u1 在 g1 发"ZXQ-RPT-1 内容"（m1）
- 步骤：
  1. u2 举报 m1；u1 撤回 m1
  2. SQLite 检查点后搜主库和 WAL 的原始字节
  3. `mod` 在后台申请证据授权（理由 + 动态码），读证据
- 预期结果：
  1. 举报成功，撤回成功
  2. 原始字节搜不到 `ZXQ-RPT-1`（`report_evidence`、隔离表都只有密文）
  3. 审核员读到"ZXQ-RPT-1 内容"

### TC-MOD-03-06 推进 181 天：report_purge 物理删除证据
- 关联 AC：AC-MOD-03-6
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：一条群消息举报，有证据
- 步骤：
  1. [IM 钟] 推进 179 天，运行 `report_purge`
  2. 推进到 181 天，再运行
- 预期结果：
  1. 179 天时证据还在
  2. 181 天后 `report_evidence` 没有这条的行（物理删除），读证据返回证据不存在；`reports` 行按 backend-chat 第 119 条的实现一起删除（Frank 定了以后按决定改）

### TC-MOD-03-07 举报没关联的、不存在的用户：响应相同 4040
- 关联 AC：AC-MOD-03-7
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u2 和 u9 没有好友、单聊、共同群；`nouser` 不存在；u2 和 u1 是好友；u2 和 u3 只有共同群 g1；u2 和 u6 只有单聊消息
- 步骤：
  1. u2 `report {targetType:"user", userId:"u9"}`、`{userId:"nouser"}`
  2. 举报 u1、u3、u6
- 预期结果：
  1. 两个都是 404 / 4040 `reportTargetInvalid`，响应逐字节相同
  2. 三个都成功（好友、共同群、共同会话，backend-chat 第 116 条）

### TC-MOD-03-08 note 含特征串：日志和审计搜不到
- 关联 AC：AC-MOD-03-8
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + console-server
- 前置条件：—
- 步骤：
  1. u2 举报 u1 的群消息，`note:"ZXQ-MOD-NOTE"`
  2. `mod` 打开详情、申请授权、读证据、结案
  3. 搜 im-server 日志、`admin_audit`、console 日志和审计、`reports` 表
- 预期结果：
  1. 都搜不到 `ZXQ-MOD-NOTE`（`note` 只在加密的证据里，backend-chat 第 122 条）
  2. 读证据的响应里有 `note`

### TC-MOD-03-09 客户端长按举报：提交前提示、成功轻提示、单聊询问拉黑
- 关联 AC：AC-MOD-03-9
- 优先级：P0
- 批次：C6
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 三端
- 前置条件：u1 给 u2 发了单聊消息、在 g1 发了群消息
- 步骤：
  1. u2 长按（H5 窄屏长按、宽屏右键、键盘菜单键）u1 的单聊消息 → "举报"
  2. 选理由"辱骂骚扰"，填补充说明，提交
  3. 对群消息再做一次
- 预期结果：
  1. 举报单有 6 个理由、补充说明（计数 0/200）、提示"所选内容将提交给平台审核"；不选理由不能提交
  2. 成功后轻提示"已提交，感谢你的反馈"；单聊询问是否拉黑（frontend-chat 第 79 条）
  3. 群消息成功后不询问拉黑
  4. 自己的消息、`tip` 的菜单里没有"举报"（frontend-chat 第 77 条）；`features.moderation=false` 时没有入口

### TC-MOD-03-10 单聊 off 时举报：只有元数据；打开开关后仍读不到正文
- 关联 AC：AC-MOD-03-10
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1（先 `IM_C2C_BODY_ACCESS=off`）
- 前置条件：u1 给 u2 发"ZXQ-RPT-2"
- 步骤：
  1. u2 举报这条
  2. 查 `report_evidence` 原始字节；用测试密钥解开密文
  3. 用 `IM_C2C_BODY_ACCESS=on` 重启（同一个库、同一把 `IM_REPORT_KEY`），`console-msgread` 申请证据授权并读证据
- 预期结果：
  1. 举报成功
  2. 原始字节和解开的明文都没有 `ZXQ-RPT-2`，只有元数据（类型、长度等）
  3. 授权和读取不报错，但只有元数据，没有正文

### TC-MOD-03-11 撤回提示行举报：成功，审核员读到隔离原文（群）
- 关联 AC：AC-MOD-03-11
- 优先级：P0
- 批次：C6
- 层级：Playwright（H5）+ 原生 UI 测试 + Playwright（管理后台）
- 环境：E1 + C4 + console-server + 三端 + tianting-admin
- 前置条件：u1 在 g1 发"ZXQ-RPT-3"，u2 打开会话看到了它，之后 u1 撤回
- 步骤：
  1. u2 点撤回提示行的"举报"，提交
  2. `mod` 在后台打开这条举报，申请授权，看证据
- 预期结果：
  1. 举报成功
  2. 后台证据显示"ZXQ-RPT-3"（隔离原文），有水印
  3. u1 自己撤回的提示行上没有"举报"

### TC-MOD-03-12 没读到的已撤回消息 4040；隔离原文过期的成功、只有元数据
- 关联 AC：AC-MOD-03-12
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + C4
- 前置条件：g1 里 u1 的消息 m1 已撤回，u3 的 `readSeq < m1.seq`；另一条已撤回的 m2，u2 看过，[IM 钟] 推进超过 30 天、隔离原文已清除
- 步骤：
  1. u3 举报 m1
  2. u2 举报 m2
- 预期结果：
  1. 404 / 4040 `reportTargetInvalid`
  2. 成功，证据只有元数据

### TC-MOD-03-13 举报群：非成员 4040、成员成功、reason 和 note 不合法 400
- 关联 AC：AC-MOD-03-13
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u9 不是 g1 成员；`gnone` 不存在
- 步骤：
  1. u9 `report {targetType:"group", groupId:"g1", reason:"spam"}`；u9 举报 `gnone`
  2. u2 举报 g1
  3. u2 用 `reason:"xxx"`；用 201 个码点的 `note`（含 emoji）
- 预期结果：
  1. 两个都是 404 / 4040，响应逐字节相同
  2. 成功
  3. 都是 400 / 4000；200 个码点的 `note` 成功

### TC-MOD-03-14 证据写失败 500 无记录；开关关 403 featureDisabled
- 关联 AC：AC-MOD-03-14
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1；第 2 步 `IM_MOD_ENABLED=0`
- 前置条件：u2 能看到 u1 的群消息
- 步骤：
  1. 故障注入 `evidenceWrite`，u2 举报
  2. `IM_MOD_ENABLED=0` 时 u2 举报
- 预期结果：
  1. 500 / 5000，`reports`、`report_evidence` 都没有记录，不计入当天次数
  2. 403 / 4030，`reason=featureDisabled`

### TC-MOD-03-15 被举报的文件：保留期内照常发放；过期后普通用户 4048，证据仍可看
- 关联 AC：AC-MOD-03-15
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + 假 S3（`IM_FILE_RETENTION_DAYS` 默认 180）
- 前置条件：u1 在 g1 发文件 f1（消息正常，没撤回）；第 10 天 u2 举报它
- 步骤：
  1. u3 `POST /v1/files/urls` 取 f1 的地址
  2. [IM 钟] 推进到发送后 181 天，运行 `file_purge`；u3 再取地址
  3. 审核员申请举报授权后 `fetch` f1
- 预期结果：
  1. 正常发放（举报不影响普通用户访问）
  2. 404 / 4048；对象没有被删除（`evidence_until` = 举报时间 + 180 天，还没到）
  3. 成功取到文件；推进到举报后 181 天再运行 `file_purge`、`report_purge`，对象和证据都被删除

---

## F-MOD-04 举报处理

### TC-MOD-04-01 审核员：读证据、删除消息、结案，审计顺序
- 关联 AC：AC-MOD-04-1
- 优先级：P0
- 批次：C6
- 层级：端到端（管理后台 Playwright）+ console e2e
- 环境：E1 + console-server + tianting-admin（`IM_V13_ADMIN_FEATURES=1`）；客户端用三端之一轮换
- 前置条件：u1 在 g1 发"ZXQ-RPT-4"，u2 举报了它；`mod` 已登录
- 步骤：
  1. `mod` 打开举报队列，进入这条
  2. 点"查看证据"，填理由和当次动态码
  3. 从详情跳到删除消息，删除 m1（理由、step-up）
  4. 回到举报，填结论"已处理"和理由，结案
  5. 查 console 审计
- 预期结果：
  1. 证据显示"ZXQ-RPT-4"和水印（账号名 + 时间到分钟 + 授权前 8 位）
  2. m1 被删除，客户端显示"该消息已被删除"
  3. 状态 `handled`
  4. 审计依次有 `report.view`、`report.evidence.grant`、`msg.delete`、`report.handle`

### TC-MOD-04-02 support、operator 读队列、结案：403
- 关联 AC：AC-MOD-04-2
- 优先级：P0
- 批次：C6
- 层级：console e2e
- 环境：E1 + console-server
- 前置条件：有一条 `pending` 举报
- 步骤：
  1. `sup`、`op` 分别 `GET /console/api/reports`、`GET /console/api/reports/{id}`、`POST .../resolve`
- 预期结果：
  1. 都是 403 / 4030，im-server 没有收到调用，状态仍 `pending`
  2. 两人的侧边栏没有"举报"菜单（AC-ADM-05-3 的做法）

### TC-MOD-04-03 单聊 off：申请证据授权 403 c2cBodyDisabled，元数据照常
- 关联 AC：AC-MOD-04-3
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + console-server（`IM_C2C_BODY_ACCESS=off`）
- 前置条件：u2 举报了 u1 的单聊消息
- 步骤：
  1. `mod` 打开详情（填理由）
  2. 申请证据授权
- 预期结果：
  1. 元数据照常：举报人、被举报人、时间、理由、类型、长度
  2. 403 / 4030，`reason=c2cBodyDisabled`，没有产生授权，不占授权配额；页面显示"单聊正文查看未开启"

### TC-MOD-04-04 console 主凭证调用 report.evidence 接口：403
- 关联 AC：AC-MOD-04-4
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：一条群消息举报
- 步骤：
  1. 用 `console` 主凭证签名调 `POST /admin/report-grants`、`POST /admin/reports/{id}/evidence`
  2. 用 `console-msgread` 调同样的接口
- 预期结果：
  1. 都是 403 / 4030
  2. `console-msgread` 成功

### TC-MOD-04-05 两个审核员同时结案：一个成功，一个 4098
- 关联 AC：AC-MOD-04-5
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + console-server
- 前置条件：一条 `pending` 举报；两个审核员 `mod`、`mod2`
- 步骤：
  1. 两人并发 `resolve`（一个 `handled`、一个 `rejected`），重复 20 次构造交错
- 预期结果：
  1. 每次只有一个成功，另一个 409 / 4098
  2. 状态是成功那个的结论，之后不再改变

### TC-MOD-04-06 同一用户被 3 人举报：列表聚合"3 次"，纯文本
- 关联 AC：AC-MOD-04-6
- 优先级：P0
- 批次：C6
- 层级：Playwright（管理后台）
- 环境：E1 + console-server + tianting-admin
- 前置条件：u1 昵称是 `<img src=x onerror=alert(1)>ZXQ`；u2、u3、u6 分别举报 u1（用户举报，backend-chat 第 117 条）
- 步骤：
  1. `mod` 打开举报列表
  2. 点开这一项
- 预期结果：
  1. 一行，显示"3 次"
  2. 展开看到 3 条（举报人、时间、理由）
  3. 昵称按原样显示为文字，没有图片、没有执行脚本

### TC-MOD-04-07 结案：举报人和被举报人收不到帧
- 关联 AC：AC-MOD-04-7
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u2 举报 u1 的消息；u1、u2 都在线
- 步骤：
  1. 管理接口结案（`handled`、`rejected` 各一次，两条举报）
  2. 2 秒后分别给 u1、u2 发哨兵
- 预期结果：
  1. 两人都只收到哨兵，之前没有任何帧（第一批不发处理结果，规则 6）

### TC-MOD-04-08 console 同时带 report.evidence 和 msg.meta：拒绝启动
- 关联 AC：AC-MOD-04-8
- 优先级：P0
- 批次：C6
- 层级：后端单测 + e2e
- 环境：启动参数单独构造
- 前置条件：调用方配置文件
- 步骤：
  1. `console` 同时配 `report.evidence` 和 `msg.meta` 启动
  2. `report.evidence` 只在 `bodyAccess` 的 `console-msgread` 上，且它没有别的 scope
- 预期结果：
  1. 非 0 退出，日志指出正文类 scope 只能在独立的 `bodyAccess` 调用方上
  2. 正常启动

### TC-MOD-04-09 super_admin 没有应急授权读证据：403
- 关联 AC：AC-MOD-04-9
- 优先级：P0
- 批次：C6
- 层级：console e2e
- 环境：E1 + console-server
- 前置条件：一条群消息举报；`sa` 只有 `super_admin`
- 步骤：
  1. `sa` 申请证据授权、读证据、取证据文件
  2. `sa` 给自己申请 30 分钟 `moderator` 应急授权后再读
- 预期结果：
  1. 都是 403 / 4030
  2. 应急授权期间可以读（第 14.1 节"应急授权照旧"），审计有 `account.emergency_grant`

### TC-MOD-04-10 单聊举报：不填理由 400、第 51 个 429、没有授权时没有 note 和文件名
- 关联 AC：AC-MOD-04-10
- 优先级：P0
- 批次：C6
- 层级：console e2e + 后端 e2e
- 环境：E1 + console-server
- 前置条件：51 条单聊举报（各自不同的会话，含 `note:"ZXQ-MOD-NOTE"` 和文件名 `ZXQ-MOD-FILE.pdf` 的文件消息）；`mod` 当天已查过 0 次单聊元数据
- 步骤：
  1. 不填理由打开一条单聊举报
  2. 填理由依次打开 51 条（中间穿插 1 次单聊元数据查询，验证配额合并）
  3. 看一条带 `note` 和文件的详情响应
- 预期结果：
  1. 400 / 4000
  2. 第 50 个之后 429 / 4292（和单聊元数据查询合并，每天 50 个）
  3. console 和 im-server 的响应都没有 `note`、文件名（搜不到两个特征串）

### TC-MOD-04-11 客服 msg.meta 查文件消息：两层响应搜不到文件名
- 关联 AC：AC-MOD-04-11
- 优先级：P0
- 批次：C6
- 层级：console e2e + 后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：g1 里有文件名 `ZXQ-MOD-FILE.pdf` 的文件消息，被举报
- 步骤：
  1. `sup` 用 `msg.meta` 查 g1 的元数据
  2. `mod` 打开这条举报的详情（不申请授权）
- 预期结果：
  1. console 响应和它从 im-server 拿到的响应都搜不到 `ZXQ-MOD-FILE`（同 AC-ADM-30-7 的做法）
  2. 举报详情同样没有文件名

### TC-MOD-04-12 被举报的图片同源加载、html 只能下载，生产 CSP 没有违规
- 关联 AC：AC-MOD-04-12
- 优先级：P0
- 批次：C6
- 层级：Playwright（管理后台）+ console e2e
- 环境：E1 + 假 S3 + console-server + tianting-admin（生产 CSP 响应头）
- 前置条件：g1 里一张 PNG 和一个 `x.html`（内容含 `<script>`）被举报
- 步骤：
  1. `mod` 申请证据授权后打开证据
  2. 看图片（缩略图和原图）；点 html 的"下载"
  3. 监听 `securitypolicyviolation`；记录网络请求
- 预期结果：
  1. 图片缩略图和原图都显示，请求都是 console 同源地址（不是对象存储地址）
  2. html 只能下载（`Content-Disposition: attachment`、`nosniff`），没有在页面里渲染
  3. 没有 CSP 违规；后台 CSP 和 C5 相同（AC-CON-10-3 不变）

### TC-MOD-04-13 单聊文件证据 off：申请文件 403 c2cBodyDisabled
- 关联 AC：AC-MOD-04-13
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + 假 S3 + console-server（`IM_C2C_BODY_ACCESS=off`）
- 前置条件：u2 举报了 u1 发的单聊文件
- 步骤：
  1. `console-msgread` 直接调 `POST /admin/files/{fileId}/fetch`（带任意举报授权）
  2. `mod` 经后台取这个文件
- 预期结果：
  1. 403 / 4030，`reason=c2cBodyDisabled`
  2. 后台同样 403，页面显示"单聊正文查看未开启"；文件的 `evidence_until` 没有被延长（backend-chat 第 95 条）

### TC-MOD-04-14 密钥不匹配显示无法解密、可以结案；审计写失败 503 未结案
- 关联 AC：AC-MOD-04-14
- 优先级：P0
- 批次：C6
- 层级：后端 e2e + console e2e
- 环境：E1 + console-server
- 前置条件：一条群消息举报，证据用密钥 K1 写入
- 步骤：
  1. 换成密钥 K2 重启 im-server；`mod` 申请授权、读证据
  2. 结案
  3. 另一条举报：console 故障注入审计写失败后结案
- 预期结果：
  1. 读证据成功返回 `undecryptable`，页面显示"证据无法解密"；告警指标增加
  2. 结案成功
  3. 503 / 5031，im-server 没有收到结案调用，状态仍 `pending`

### TC-MOD-04-15 文件名含 `<script>` 的证据：纯文本
- 关联 AC：AC-MOD-04-15
- 优先级：P0
- 批次：C6
- 层级：Playwright（管理后台）
- 环境：E1 + 假 S3 + console-server + tianting-admin
- 前置条件：g1 里文件名 `<script>alert(1)</script>.txt` 的文件消息被举报
- 步骤：
  1. `mod` 申请授权后看证据
- 预期结果：
  1. 文件名按原样显示为文字；没有弹窗、没有执行脚本；DOM 里没有新增的 `script` 元素

### TC-MOD-04-16 群文件 fetch：没有授权 403 / 4039，有授权成功
- 关联 AC：AC-MOD-04-16
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：g1 里的文件 f1 被举报（举报 R1）；另一条举报 R2 不含 f1
- 步骤：
  1. `console-msgread` 调 `POST /admin/files/f1/fetch`：不带 `grantId`；带 R2 的授权；带别的操作人的 R1 授权；带过期的 R1 授权
  2. 带这个操作人有效的 R1 授权，`variant` 分别是缩略图和原图
- 预期结果：
  1. 都是 403 / 4039（backend-chat 第 109 条）
  2. 两个都成功，响应头 `attachment`、`nosniff`

### TC-MOD-04-17 181 天前的 account_events：清除
- 关联 AC：AC-MOD-04-17
- 优先级：P0
- 批次：C6
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 有登录事件（`account_events`）
- 步骤：
  1. [IM 钟] 推进 179 天，运行 `account_events_purge`
  2. 推进到 181 天，再运行
- 预期结果：
  1. 179 天时事件还在（`GET /v1/account/events` 能看到）
  2. 181 天后被清除（C1 已有这个定时任务，C6 补用例）
