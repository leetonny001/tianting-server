# 后端任务单：天听聊天产品第一批（C1 账号、C2 图片和文件、公共部分）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.1](../prd/PRD-chat-v1.1.md)（冻结）第 4～6、11、12、14、15、16、18 节；[ARCH-v1](../arch/ARCH-v1.md)；[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[管理接口](../../protocol/ADMIN.md) |
| 范围 | **只有 C1、C2 和公共部分**（PRD 决定 CH-27）。C3～C6 的 AC 不进任务；【全部】的 AC 只验证到 C2 为止的部分（PRD 第 15 节的累计表） |
| 负责 | 后端（AI） |
| 仓库 | im-server（GitHub 仓库 `tianting-server`，本地 `im-mvp/server/`）；协议在同仓库 `protocol/`，部署在 `deploy/`，工具在 `tools/` |
| 编号 | **BE-400 起**。公共 BE-400～BE-408，C1 BE-410～BE-424，C2 BE-430～BE-440 |
| 起点 | 现有迁移到 `0014_admin_stats.sql`；只有 SQLite；`IM_V13_ADMIN_FEATURES` 默认关闭；`tools/objstore-contract` 已有合同工具 |
| 完成定义 | 任务内的 AC 都有通过的自动化测试，用例名带 TC 编号（[TC-ACC](../testcases/TC-ACC.md)、[TC-FILE](../testcases/TC-FILE.md)、[TC-CHAT](../testcases/TC-CHAT.md)）；`go test -race ./...` 全过；默认配置下 `server/e2e` 现有文件不改代码全过（AC-ACC-07-1）；Schema、向量、`ERRORS.md`、`ADMIN.md` 同步；`python3 -I tools/trace.py --check` 没有缺口 |
| 环境限制 | **CI 现在不运行**（[PENDING](../reports/PENDING.md) P-21，Actions 预算是 $0），所有结果都是本地结果，要标明。开发机不装软件；真实 MinIO 要 Frank 同意才能临时构建。压测机 P-12、真机和渗透 P-13 未到位 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。一条 AC 可以出现在多个任务里（功能任务 + 验收任务）。C3～C6 只写在任务内容里说明，不进 `- AC：` 行。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-400 | ✅ 完成（服务端部分） | errors.json、ERRORS.md、kickReasons、`kicked.recoverBefore`、`securityNotice`、`recoveryCodeUnacked`、`http-client.schema.json` 账号部分、ADMIN.md、README.md。AC-CHAT-90-14 服务端一致性（TC-CHAT-90-12）通过；**三套 SDK 枚举未做（FE-600～602）**。ADMIN.md 的账号接口随 BE-421 实现 |
| BE-402 | 🟡 部分完成 | 已做：`frames.json` 的 `kicked.recoverBefore`、`securityNotice` 用例；`account-session.json` 和 Go 参考客户端（`server/e2e/accountmodel`）覆盖规则 5 单飞、规则 6 和 6a 失败分类、规则 7 的 4011；规则 4 判定表 `serverCases`（参考模型 `server.go` 产出，真实服务端跑 HTTP 和 WS 两遍，`e2e/account_vectors_test.go`）。**未完成**：三套 SDK 跑向量（FE-600～602），所以 AC-CHAT-90-10、90-15 未完成 |
| BE-405 | 🟡 部分完成 | 已做：`testhook`（`-tags imhooks`，nm 断言生产构建没有符号）、`IM_TEST_DISABLE_AUTH_LOCK`、`limits/reset scope=auth`、Argon2 阻塞、注册事务失败、上海 00:00 时钟助手、生产拒绝启动（四个开关和带钩子的二进制）。AC-CHAT-90-18、AC-ACC-02-3 通过。**AC-ACC-04-18、04-19、05-11 要 recover 和改密接口，随 BE-417、418**，现在只验了登录路径的钩子序列。吊销写失败、入表前吊销、入表后和周期复查失败、丢弃吊销事件、入表前暂停都已接上（BE-416、417）；**留到后续**：`IM_TEST_DISABLE_IDLE` 等三个开关现在只拦生产，功能随 BE-406 |
| BE-408 | ✅ 完成 | PRD-core v1.5、PRD-social v1.1、PRD-ui v1.3、PRD-admin v1.3、PRD-ops v1.4、ARCH 更新；作废 AC-UI-03-9，改写 AC-AUTH-02-4、AC-UI-15-3、AC-UI-16-6、AC-ADM-30-7；同步 TC 和 `trace.py`（PRDS；PRD-live 没有升版，它的 name 判断不变）。AC 总数 1463 → 1462。`docs/sdk/README.md` 按任务单由 FE-620 更新；前端代码（`nginx.conf`、`vite.config.ts`）随 FE-613 |
| BE-410 | ✅ 完成 | `0015_accounts.sql`、`store.AccountStore`、`storetest` 11 个契约用例；AC-CHAT-90-6（TC-CHAT-90-06，存储层）通过。`users.deleted_at` 的入口都改了，注销流程随 BE-420。Postgres 随 B6 |
| BE-411 | ✅ 完成 | `clientip` 共用、`auth.Check` 共用（WS 和 Bearer）、`/v1` 基座、`GET /v1/config`。AC-ACC-07-4（TC-ACC-07-04，`features.files` 和 `IM_FILES_ENABLED` 随 C2）、AC-CHAT-90-16（TC-CHAT-90-14，含真实 10 秒慢请求）通过 |
| BE-412 | ✅ 完成 | `x/crypto`（v0.58.0，和 console-server 同版本）、闸门、`Guard`、接口 e2e 通过：AC-ACC-01-16、02-4、02-5、02-10、02-11、02-12、02-14、04-22、04-24、05-3、05-18 |
| BE-413 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-01-1～8、10、11、14、15、17、19、21、AC-ACC-10-2 通过（TC-ACC-01-*、TC-ACC-10-02，本地 e2e）。**AC-ACC-01-5 的例子 `bob` 只有 3 位，走不到 4095**，用例改成 `bobby`，PRD 问题待处理。TC-ACC-01-01 第 3 步 `recovery-code/ack` 随 BE-418，用存储层代替。AC-ACC-01-6 的已注销前置用 `MarkUserDeleted`，`account_cleanup` 随 BE-420 |
| BE-414 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-02-1、2、3、6、7、13（服务端部分）和 02-4、5、10、11、12、14 通过（TC-ACC-02-*，本地 e2e）。AC-ACC-02-3 不带 `-race` 用生产参数，带 `-race` 用 8 MiB。`account_events`、`GET /v1/account/events`、`account_events_purge`、`auth_throttle_purge`、两个指标。AC-CHAT-90-16（TC-CHAT-90-14）通过。提前做了 AC-ACC-07-2（演示登录拒绝有账号的用户）：账号从这段开始存在。AC-ACC-02-13 的界面随 FE-610～612 |
| BE-415 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-03-1、2、3、4、15、19、21、22、24、25、27、31、32、33、35、37（服务端部分）、38、40、44（logout 部分；credentialsChanged 部分随 BE-417）、45、46、48、49、50 通过（TC-ACC-03-*，本地 e2e）。`0016_account_sessions.sql`：签发频率存在会话行上；`refresh_key_ledger` 台账，`im-server refresh-keys forget` 是泄露应急；配置检查项 `refresh_key_ledger`，生产配置拒绝启动。`im-server sessions revoke-all`。AC-ACC-03-19、03-27 的推送令牌部分 C3 上线时验。AC-ACC-03-33 的后台响应部分随 BE-421 |
| BE-416 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-03-5、6、17、28、29、30、36（服务端部分，SDK 重连随 FE）、41、47 通过；AC-ACC-03-16、03-34 要改密接口，用例在 BE-417 的提交里。`auth.Check` 共用判定：账号状态 → 版本号 → 无 sid 的 iat → 会话吊销 → 过期 → 代号；`auth.Recheck` 入表后和周期复查（1011 失败关闭，周期连续 3 次）；踢线都带 `recoverBefore`。总线重连后复查：`Gateway.RecheckAll` 留好入口，第一批没有总线，随 B6 接上 |
| BE-417 | ✅ 完成，AC-ACC-04-29 未执行 | 和 BE-418 同一个提交（接口互相依赖）。AC-ACC-04-1、2（服务端部分）、3、4、7、8、10（服务端部分）、11、12、14、15、16（服务端部分）、18、19、25 通过；连带 AC-ACC-03-16、03-34、03-44。用密码的提交事务第一条语句都锁 `accounts` 行（`store.CommitCredentials`）。**AC-ACC-04-29 只有 Postgres 下可达，用例标未执行，不算通过**。显示内容设置和令牌重新注册随 C3 |
| BE-418 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-04-13、20、23、24、30、31，AC-ACC-05-1、2、3、6、8、11、12、13、14、17、18、19（服务端部分）通过。`store.CommitRecover`：第一条语句不带版本条件锁行并递增；旧码优先；上限只约束最新一代。`recoverSched` 按 `(userId, gen)` 调度，单个实例生效，集群随 B6。TC-ACC-01-01 第 3 步改用 `recovery-code/ack` 接口 |
| BE-419 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-06-1、06-2、07-1（`server/e2e` 里演示和管理接口的用例不改代码全过，见汇报里的例外说明）、07-2、07-3 通过。`4048` 提前进 `errors.json`（C1 的 `avatarFileId` 要用，BE-401 不用再加） |
| BE-420 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-04-21、04-28、05-16、08-1、08-2、08-3、08-4、08-5（恢复和不重复；ack P99 是开发机参考，不算通过，见 BE-422 报告）通过；连带 AC-ACC-04-22、04-25 序列乙。`account_cleanup`：第 ① 步条件带"申请没被撤销"，第 ② 步在 `logic`，幂等。TC-ACC-01-07 改走真实注销。头像对象随 BE-437。**保留口径 CH-23 还没确认（规则 6），按 PRD 第 5 条表实现** |
| BE-421 | ✅ 完成（本任务 AC 都有用例） | AC-ACC-05-9（服务端路由和 scope；console 路由随 CS-200）、09-1（后端部分）、09-2、09-5、09-7、09-9、09-10、09-12、09-13、09-14 通过；AC-ACC-03-33 的后台响应部分通过。配置检查项 `invite_total` 查库现算。AC-ACC-09-6 的 console 角色随 CS-200，这里验了 scope 和旧密钥拒绝 |
| BE-424 | ✅ 完成（文档） | `docs/reports/privacy-notice-checklist-draft.md`：规则 3 的告知项、C1 和 C2 相关的事实和法务问题。**只是清单草稿，不代替法律文本** |
| BE-423 | ✅ 完成 | `deploy/ACCOUNT-RUNBOOK.md`：配置、`GOMEMLIMIT`、配置检查和指标、密钥台账和泄露应急、数据库恢复、NAT。`IM_TRUSTED_EGRESS_CIDRS`、`IM_TRUSTED_EGRESS_FACTOR`（PRD 没有定义名字，任务单要求实现时加）。AC-CHAT-90-17（TC-CHAT-90-15）的用例在 BE-422 的提交里 |
| 全部 | ⬜ 未开始 | C1 结束后按实际用时校准工作量（CH-27），再由 Frank 决定 C3 起的部分 |

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| 公共 | BE-400～BE-408 | 14 | 协议、向量、测试设施（假 S3、假通道、账号并发钩子）、测试环境、冻结文档变更 |
| C1 账号 | BE-410～BE-424 | 27.5 | 迁移、`/v1` 基座、闸门和限流、注册、登录、会话续期、改密、恢复码、注销、邀请码、验收、部署和运维手册 |
| C2 文件 | BE-430～BE-440 | 24.5 | 对象存储客户端、上传、`complete`、发送、访问控制、生命周期、头像、部署、合同测试、验收 |

**人日对照（PRD-chat 第 18 节）**

| 第 18 节的列 | 公共 | C1 | C2 | 落在 |
|---|---|---|---|---|
| 服务端 | 3 | 18.5 | 14.5 | 本单：BE-400～402、405；BE-410～421；BE-430～437 |
| console + 后台页面 | 0 | 1.5 | 2.5 | [console-chat](console-chat.md) 和 [frontend-chat](frontend-chat.md) |
| sdk-ts / sdk-kotlin / sdk-swift | 1 / 1 / 1 | 3.5 / 4 / 4 | 3.5 / 5.5 / 5.5 | [frontend-chat](frontend-chat.md) |
| H5 / Android / iOS | 0 | 5 / 5.5 / 5.5 | 6 / 8 / 8 | [frontend-chat](frontend-chat.md) |
| 测试 | 6 | 8.5 | 9.5 | 公共 6：BE-402～406。C1：BE-422 5 + FE-620 3.5。C2：BE-440 5.5 + FE-640 4 |
| 部署 | 1 | 1.5 | 3 | BE-407；BE-423；BE-438 2 + BE-439 1 |
| 文档 | 4 | 3 | 1.5 | 公共：BE-400 1、BE-401 0.5、BE-408 2.5。C1：BE-423 1.5、BE-424 1、FE-620 0.5。C2：BE-438 1、BE-439 0.5 |
| **合计（三份任务单）** | **17** | **60.5** | **67.5** | **145，和第 18 节一致** |

**差异说明**：合计和第 18 节没有差异，因为这里是按第 18 节的列拆分，不是独立重估，**都是低置信**。下面这些第 18 节没有单独算，也没有另加人日，如果 C1 校准时超出就从这里看：① 第 4.5 节没有列的暂停点和故障注入（BE-404、BE-405）；② `sessions revoke-all` 命令路径（BE-415）；③ 邀请码重新启用和可信出口网段的配置（BE-421、BE-423）。

**依赖顺序**：BE-400、BE-401 → BE-408、BE-402、BE-403 → BE-405、BE-407 → BE-410 → BE-411 → BE-412 → BE-413、BE-414 → BE-415 → BE-416 → BE-417 → BE-418 → BE-419、BE-420、BE-421 → BE-424、BE-423 → BE-422 →（C1 结束，校准）→ BE-404、BE-406 → BE-430 → BE-431 → BE-432 → BE-433 → BE-434 → BE-435 → BE-436 → BE-437 → BE-438、BE-439 → BE-440。console 和前端的依赖写在各自任务里。

---

## 公共部分

### BE-400 协议变更（C1 部分）：错误码、kickReasons、账号 HTTP Schema
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`protocol/`）
- 依赖：无（最先做）
- AC：AC-CHAT-90-10、AC-CHAT-90-14
- 内容：
  1. `protocol/errors.json`、`ERRORS.md`（第 11.5 节）：新增 4018、4019、4095、4294；`4000`、`4004`、`4030`、`4034`、`4290`、`5000`、`5030` 的 scope 增加 `client`；`4010`、`4011`、`4012` 增加 `client`，HTTP 401；**`4011` 的动作改写**（账号模式先比较本地代号并刷新或重连，4019 才回登录页；演示模式不变）；`4292` 按"命令 + 码"的文案只加上传。429x 的"不自动重试"例外增加 4294。
  2. 新 `reason`：`4030` 的 `registerClosed`、`inviteInvalid`、`accountSessionRequired`、`accountExists`、`accountDeletionPending`，`featureDisabled`（C2 起用）；`4019` 的 `invalid`、`superseded`、`replayDetected`、`credentialsChanged`、`loggedOutOthers`、`accountDeleted`；`4011` 的 `tokenSuperseded`；`4290` 的 `recoverLimited`；`4000` 的 `passwordWeak`、`recoveryCodeStale`。**第 11.5 节没有列 `recoverLimited`，这里按 F-ACC-05 规则 2 补上**。
  3. `kickReasons`：`passwordChanged`、`sessionRevoked`、`accountDeleted`（关闭码 4012）；`kicked` 帧增加可选 `recoverBefore`（`frames.go` 第 334～339 行，RS5-03）；`replaced` 仍不实现。
  4. 新事件 `securityNotice {kind, at, platform}`；`auth` ack 增加 `recoveryCodeUnacked`（F-ACC-05 规则 1，第 11.1 节没有列，这里补）。
  5. 新增 `protocol/schema/http-client.schema.json` 的账号部分：`GET /v1/config`、`/v1/account/*`（第 11.2 节）。`ADMIN.md` 增加 `POST /admin/invites`、`GET /admin/invites`、`POST /admin/invites/{id}/disable`、`GET /admin/accounts/{userId}`（第 11.6 节，没有 `account.reset`）。`README.md` 里"token 由业务服务端"改写。
  6. F-SYS-01 的一致性测试把 `client` scope 纳入（服务端常量、三套 SDK 枚举，SDK 部分见 FE-600～602）。
  7. **本任务不做（留到对应里程碑）**：4097、4098、4220、4293、4510；`recall`、`convSet`、`convDelete`、`deviceNotification*`、`presence`、`report` 命令；`msgDeleted.recalled`、`convSettingsChanged`、`convDeleted` 事件；`Message.recalled*`、`LastMsg.recalled*`、`Conversation` 的 `pinned` 等；`GroupMembersReq.keyword`；`4044`、`4131` 的 ws scope；`4040`、`4030`、`4290` 里属于撤回、引用、@、举报的 reason；词库和举报的管理接口。
- 工作量：2 人日（服务端 1 + 文档 1，公共）

### BE-401 协议变更（C2 部分）：image、file、MsgBody 收紧、modVer
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`protocol/`、`server/internal/protocol`）
- 依赖：BE-400
- AC：AC-CHAT-90-10
- 内容：
  1. 协议文件的改动可以在 C1 开工时和 BE-400 一起做（BE-408 要用），运行时校验随发送任务生效。
  2. `frame.schema.json`：`MsgType` 增加 `image`、`file`；`MsgBody` 增加 `image {fileId, width, height, size, mime, expired?}`、`file {fileId, name, size, mime, expired?}`（上行只有 `fileId`）。
  3. `MsgBody` 按 `type` 互斥，用条件约束（F-MOD-02 规则 4a，从 C2 起生效，不受开关影响）：`text` 只能带 `text`、`quote`、`mentions`（后两个的结构随 C4 加）；`image` 只能带 `image`；`file` 只能带 `file`；`custom` 只能带 `data`。`frames.go` 第 177～191 行的 `Validate` 同步。**AC-FILE-02-2、02-11 要求上行 `image` 里伪造的 `size`、`width` 不报错、被忽略，所以 `image` 内部不能用 `additionalProperties:false`**（见文末"发现的 PRD 问题" 9）。
  4. `Conversation` 增加 `modVer`；`convs` 下发。其他新字段不在这批。
  5. `errors.json`：4048、4096、4139、4150、5032；`4131` 的 scope 增加 `client`；`4292` 上传文案；`5032` 的 `completeInProgress`；`4290` 的 `uploadPendingLimit`；`4000` 的 `fileKindMismatch`、`uploadMismatch`、`fileNameTooLong`。`ERRORS.md` 增加 `415x` 区段。
  6. `http-client.schema.json` 的文件部分：`/v1/files/uploads`、`/v1/files/{id}/complete`、`/v1/files/urls`、`DELETE /v1/files/uploads/{id}`。
  7. `IM_CLIENT_CUSTOM_MSG` 和 `customDisabled`（F-MOD-02 规则 4）属于 C6，这里不做。
- 工作量：1 人日（服务端 0.5 + 文档 0.5，公共）

### BE-402 共享向量 account-session.json 和 Go 参考客户端
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`protocol/vectors`、参考客户端）
- 依赖：BE-400
- AC：AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 第 13.1 节：新增 `protocol/vectors/account-session.json`（续期、重放、失败分类），步骤式，写法同 `push-sync.json`；Go 参考客户端产出期望值，同 `social.json`。服务端和三套 SDK 四方都跑。
  2. 用例覆盖 F-ACC-03 规则 4 的判定表（`current`、`pending`、被替换、窗口内外）、规则 5 的单飞、规则 6 的失败分类、规则 7 的 4011。
  3. `frames.json` 增加 `kicked.recoverBefore`、`securityNotice` 的合法和非法用例。
- 工作量：1.5 人日（服务端 0.5 + 测试 1，公共）

### BE-403 共享向量 chat-sync.json（合并和过期）和附件状态机
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`protocol/vectors`）
- 依赖：BE-401、BE-402
- AC：AC-CHAT-90-10、AC-CHAT-90-15
- 内容：
  1. 新增 `chat-sync.json`，本批只写**单调合并和过期**：`expired` 只能从假变真、迟到的旧 `sync` 不回写、`modVer` 变化才重拉（F-RCL-03 规则 2、2a，第 15 节表）。撤回、引用、提及、置顶、`deleted`、总未读的用例留到 C4、C5。
  2. `account-session.json` 追加附件状态机（F-FILE-07 规则 1、3、4）。
  3. `frames.json` 增加 `image`、`file`、`MsgBody` 互斥的合法和非法用例。
- 工作量：0.5 人日（测试 0.5，公共）

### BE-404 测试设施：假 S3
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`server/internal/objstore/objstoretest`）
- 依赖：BE-401
- AC：AC-FILE-01-8、AC-FILE-01-15
- 内容：
  1. 第 4.5 节：**独立进程**（console 的是另一模块的 `internal` 包，用不了），只在 `IM_TEST_MODE=1` 的测试编排里启动。校验预签名、大小、过期、`response-*` 头、CORS、Copy、ETag 条件。
  2. 可注入：删除失败、不可用（拒绝连接、黑洞）、Copy 失败、**HEAD 之后替换对象**（RC2-07）。
  3. **第 4.5 节没有列、但 AC 要用的测试接口**（见文末问题 4）：暂停点（Copy 之后、提交之前；校验之后、Copy 之前；公开桶 Copy 之后），用于 AC-FILE-01-19、01-24、01-26、01-27、AC-ACC-06-12、06-13；直接写入对象和推进临时前缀生命周期，用于 AC-FILE-01-2、01-9、01-18。
  4. 写清楚：**假 S3 只验调度逻辑，不证明厂商行为**，厂商能力由 BE-439 验。
- 工作量：2 人日（测试 2，公共）

### BE-405 测试设施：账号并发钩子、故障注入、生产拒绝启动
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`server/internal/app`、`admin`、`config`）
- 依赖：BE-400
- AC：AC-ACC-02-3、AC-ACC-04-18、AC-ACC-04-19、AC-ACC-05-11、AC-CHAT-90-18
- 内容：
  1. 账号并发钩子（RS4-06）：请求在 Argon2 之后、提交事务之前暂停。**用构建标签隔离，生产构建不含这个钩子**，`go build` 不带标签时用 `archtest` 或符号检查断言。
  2. `POST /admin/test/faults` 增加：注册事务失败、吊销写失败、入表前吊销、审计写失败、Argon2 阻塞钩子（让闸门占满，不依赖真实耗时）。已有 `drop-ack` 复用。**第 4.5 节没有列、AC 要用的**：入表后复查失败（AC-ACC-03-36）、周期复查失败和丢弃吊销事件（AC-ACC-03-47）、连接校验之后入表之前的暂停（AC-ACC-03-41）。提及表、隔离写、证据写、词库、@ 计数、设置读取这些故障随 C4～C6。
  3. `IM_TEST_DISABLE_AUTH_LOCK=1`（AC-ACC-02-3、05-11）；`/admin/test/limits/reset` 增加 `scope:"auth"`；时钟从 Asia/Shanghai 当天 00:00 起算的助手（R-CQ-41）；换 IP 用 `IM_TRUSTED_PROXIES` + `X-Forwarded-For`。
  4. **生产拒绝启动**：`config/checks.go` 第 107 行的 `IM_TEST_MODE` 检查扩展到 `IM_TEST_DISABLE_AUTH_LOCK`、`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT`、`IM_TEST_DISABLE_IDLE`（R-CQ-01）。**这条没有 AC，用单测覆盖**（见文末问题 5）。
- 工作量：2.5 人日（服务端 1 + 测试 1.5，公共）

### BE-406 测试设施：假 APNs、假 FCM 和其余替身
- 优先级：P2（C3 才使用；放在公共部分是因为第 18 节这样算）
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-405
- AC：无（本批没有 AC 用到它）
- 内容：
  1. 第 4.5 节：HTTP 层假通道，`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT` 只在测试模式生效；校验 ES256 JWT、必需请求头、OAuth；可注入 410、`UNREGISTERED`、429、503、超时；保存完整请求（含请求头）供断言。
  2. `IM_TEST_DISABLE_IDLE=1`，只关服务端 4408。
  3. **C3 没有排期（CH-27）。如果校准后决定不做 C3，这 1 人日省下**；Frank 可以决定推迟（见汇报）。
- 工作量：1 人日（测试 1，公共）

### BE-407 测试环境和 CI 任务定义
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`deploy/testenv`、`.github`）
- 依赖：BE-400
- AC：无（设施任务）
- 内容：
  1. `deploy/testenv/` 增加假 S3 和固定版本 MinIO 的编排；`deploy/testenv-ci.sh` 增加 `tools/objstore-contract` 的一步。只交付编排文件，不在开发机安装。
  2. `.github` 增加 `chat-contract` 任务定义。**CI 现在不运行（P-21）**，写好但没运行过的要在任务状态里写明。
  3. PRD 第 4.5 节"要不要构建到临时目录按 `backend-ops.md` 的约定，未定前只在 CI 跑"。本批沿用：临时构建每次要 Frank 同意。
- 工作量：1 人日（部署 1，公共）

### BE-408 冻结文档变更（C1、C2 涉及的部分）
- 优先级：P0
- 批次：C1（**C1 开工时做，现在不改那些文档**）
- 仓库：tianting-server（`docs/`、`protocol/`）
- 依赖：BE-400、BE-401
- AC：无（文档任务；要作废和改写的 AC 列在内容里，不进这一行）
- 内容（第 16.1、16.2 节，走 [process](../process/README.md) 的变更流程：PRD 升小版本，文末变更记录写原因，同步任务单和用例）：
  1. **作废 AC-UI-03-9**（"请从业务方入口进入天听"）：PRD-ui 升版；AC-ACC-07-5 取代。删除 `TC-UI.md` 第 33、447 行起的 TC-UI-03-09 和第 2436 行的对照行；`frontend-ui.md` 第 236 行去掉这个编号；否则 `trace.py` 报 UNKNOWN。
  2. **改写 AC-UI-15-3**（"我"页没有编辑入口 → 账号会话有，演示会话没有）：改 TC-UI-15-03；**改写 AC-UI-16-6**（设置页增加"账号安全"，演示会话不显示；"通知"这一项随 C3）：改 TC-UI-16-06。
  3. **改写 AC-AUTH-02-4**：账号模式下 4011 先刷新一次（F-ACC-03 规则 7），演示模式不变。改 TC-AUTH-02-07、02-09（只在演示模式下断言），TC-AUTH-02-08 不变。
  4. **AC-UI-90-13**：断言不变；NF-UI-14 的 `connect-src` 加具体存储域名，`img-src` 加 `blob:`（C2）；改 TC-UI-90-14 的前置配置。
  5. **AC-KEY-02-2**：参数化表增加 `account.invite`；改 TC-KEY-02-02。`report.*`、`moderation.words` 随 C6。
  6. **AC-ADM-30-7**：元数据响应没有正文，扩展到文件名。C2 就有文件消息，所以这条随 C2 生效，见 CS-201 和 TC-FILE-91-01。
  7. **不变、只核对**：AC-CONV-01-1（`modVer` 是可选字段，断言不变）；AC-CON-10-3（后台 CSP 不放宽）；TC-USER-01-16（由 AC-ACC-01-5 补上自动化）。
  8. **第 16.1 节 C1 的部分**：PRD-core F-AUTH-01 规则 1、F-AUTH-02、F-AUTH-03、F-AUTH-04、F-AUTH-05、F-USER-01 规则 4、F-USER-02、第 1、2 节"业务方"表述；PRD-social 1.3、11.2 增加注册路径，F-SOC-09、F-SOC-10 的说明；PRD-ui F-UI-03、F-UI-15、F-UI-16、第 126 行、第 11 节和 G-UI-05、3.4 节 token 存储、D-UI-10，`tianting-web/deploy/nginx.conf`、`vite.config.ts` 转发 `/v1/`（代码随 FE-613）；PRD-admin F-KEY-02 规则 2、4、5 和第 5 节矩阵（`account.invite`、业务方典型配置去掉 `token.issue`）；PRD-ops F-OPS-06、07、10 的账号部分；ARCH S4（`open` 和 `token.issue` 的启动保护）。`docs/sdk/README.md` 由 FE-620 更新。
  9. **第 16.1 节 C2 的部分**：PRD-core 第 704 行、F-MSG-01 规则 6、F-MSG-05 规则 7、F-MSG-06 规则 6、3.1 节消息类型、F-CONV-01 的 `modVer`；PRD-ui G-UI-06、G-UI-11、F-UI-04 规则 5 的摘要、NF-UI-14；PRD-admin F-ADM-31 规则 6、7 和 F-ADM-32 里**文件名进隔离区和文件处理**的部分；PRD-ops F-OPS-06、07、10 的文件部分、F-OPS-12；ARCH 3.4 节、第 2 节图（对象存储）。
  10. **留到 C3～C6，这个任务不动**：AC-UI-09-1、AC-UI-23-1、23-2、AC-GRP-03-3、AC-ADM-31-6、AC-ADM-60-1（TC-ADM-60-01 改关联）、AC-ADM-60-2（TC-ADM-60-02 删除）；AC-UI-09-7、AC-SOC-02-5 不变。第 16.1 节里 PRD-core 第 705、709、512 行，F-CONV-01 其余、F-CONV-02 规则 5，F-GRP-03 规则 4；PRD-social F-SOC-02、`groupMembers`；PRD-ui G-UI-07、08、10、13，第 245 行，F-UI-04 规则 9，F-UI-09，F-UI-23，F-UI-42；PRD-admin F-ADM-60、F-KEY-02 的 `report.evidence`、第 12 节；PRD-ops 的推送、撤回、治理部分；PRD-live F-ROOM-11；ARCH S13、S14、G1、G2、推送通道和 `Presence`。
  11. **工具**：`tools/trace.py` 已由 Frank 改好，本任务不改它。各 PRD 升版后 `PRDS` 的文件名要跟着改，在这个任务执行时一起提交。
- 工作量：2.5 人日（文档 2.5，公共）

---

## C1 账号

### BE-410 迁移和 Store：账号相关的表
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`server/internal/store`）
- 依赖：BE-400、BE-405
- AC：AC-CHAT-90-6
- 内容：
  1. 第 12.1 节，新迁移从 `0015` 起：`accounts`、`recovery_codes`、`users.deleted_at` 和 `lower(user_id)` 上的**非唯一**索引、`auth_throttle`、`account_events`、`device_sessions`、`invites`。`users` 不加唯一约束（R-CB-01）。
  2. 遵守 ARCH 3.1、3.3 节：只加可空或有默认值的列；时间 `BIGINT` 毫秒；不用触发器；SQL 同时兼容 SQLite 和 Postgres；Store 新方法走 `storetest` 契约测试。
  3. 加锁顺序（第 4.3 节）：`accounts` → `recovery_codes` → `device_sessions`。
  4. `users.deleted_at` 要改到的入口：`Relation`、`UserExists`、`MissingUsers`、`issueFor`、演示登录、`UpsertUsers`、管理加人（F-ACC-08 规则 4）。
- 工作量：1.5 人日（服务端）

### BE-411 /v1 基座和 GET /v1/config
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`server/internal/api`、`gateway`）
- 依赖：BE-410
- AC：AC-CHAT-90-16、AC-ACC-07-4
- 内容：
  1. 第 4.2 节：路径 `/v1/`，对外端口；错误 `{code, msg, reason?, details?}`；`Authorization: Bearer`；请求体 ≤ 64 KB；路由层读超时 10 秒（现有服务端不设 `ReadTimeout`）；生产只接受 HTTPS。
  2. **把 `gateway/guard.go` 第 86 行的 `clientIP` 提出来共用**（R-CS-35），IPv6 按 /64 聚合（R-CS-06）。
  3. Bearer 校验和 WS `auth` 相同，共用一个函数，错误 4010、4011、4012。
  4. `GET /v1/config`（F-ACC-07 规则 4）：`{registerMode, demoLogin, features, limits}`，加隐私说明版本和地址（F-ACC-10 规则 1）；不含任何密钥。
- 工作量：1.5 人日（服务端）

### BE-412 Argon2 闸门和失败限制
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-411
- AC：AC-ACC-01-16、AC-ACC-02-4、AC-ACC-02-5、AC-ACC-02-10、AC-ACC-02-11、AC-ACC-02-12、AC-ACC-02-14、AC-ACC-04-22、AC-ACC-04-24、AC-ACC-05-3、AC-ACC-05-18
- 内容：
  1. 引入 `x/crypto/argon2`（im-server 现在没有这个依赖，console 的 `internal/secret` 不能引用），m=64 MiB、t=3（F-ACC-01 规则 5）。
  2. 闸门（F-ACC-01 异常，R-CB-25）：`IM_ARGON_SLOTS`=4；注册、登录、找回、改密、注销才进；`token/refresh`、`logout` 不进；**先过限流，再进闸门**；排队最多 1 秒，超时 503 / 5030；改密 1 个专用位；`recover` 自己的专用位（`IM_ARGON_RECOVER_SLOTS`、`IM_RECOVER_QUEUE`），恢复码命中才用。专用位在 4 个之外，最多 6 个（PRD v1.1 第 4.1 节）。
  3. `auth_throttle`（F-ACC-02 规则 3）：按"用户名哈希 + IP 前缀"15 分钟 5 次，429 / 4294，`details.retryAfterSeconds`；不存在的用户名同样累计；每 IP 每分钟 20 次 4290；按用户名的渐进延迟，最多 3 秒，放在闸门之外；同一用户名同时只算 1 个哈希。所有校验密码的接口共用（RS4-11、RS4-12）。
  4. 登录失败的日志和事件只记用户名哈希（规则 3a）。
- 工作量：1.5 人日（服务端）

### BE-413 注册（F-ACC-01）和隐私版本
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-412
- AC：AC-ACC-01-1、AC-ACC-01-2、AC-ACC-01-3、AC-ACC-01-4、AC-ACC-01-5、AC-ACC-01-6、AC-ACC-01-7、AC-ACC-01-8、AC-ACC-01-10、AC-ACC-01-11、AC-ACC-01-14、AC-ACC-01-15、AC-ACC-01-17、AC-ACC-01-19、AC-ACC-01-21、AC-ACC-10-2
- 内容：
  1. `POST /v1/account/register`，规则 1～9。`closed`、`invite`、`open` 三种模式；`invite` 模式先校验邀请码再查用户名（规则 7）。
  2. 用户名规范化和 `userId`（规则 3、4，CH-02）：`accounts.username_lower` 唯一 + `users.lower(user_id)` 非唯一，事务内都查；C-18 和演示、导入路径不变。保留名配置文件。
  3. 密码规则和内置弱口令表（规则 5）；响应带 `recoveryCode`、`recoveryCodeGen`。
  4. 同一事务写 `users`、`accounts`、`recovery_codes`、`device_sessions`，扣邀请码；`IM_PRIVACY_NOTICE_VERSION` 写入 `accounts.notice_version`（F-ACC-10 规则 2）。
  5. 每 IP 每小时 20 次；同一邀请码 24 小时内 4095 达到 20 次自动停用并告警（规则 7，`invites.mismatch_count` 按窗口计，RC2-31）。
  6. 启动保护：`IM_ENV=production` 且 `open` 拒绝启动（规则 2，C6 上线前一律拒绝，AC-ACC-01-19）。（缺开关时的逐项报错属于 C6。）
- 工作量：1.5 人日（服务端）

### BE-414 登录（F-ACC-02）和账号事件
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-412
- AC：AC-ACC-02-1、AC-ACC-02-2、AC-ACC-02-3、AC-ACC-02-6、AC-ACC-02-7、AC-ACC-02-13
- 内容：
  1. `POST /v1/account/login`（规则 1、2、4、5）：四种失败响应相同，401 / 4018；不存在时也算一次哈希；校验密码通过后才判断封禁；不踢其他设备。冷静期内 403 / 4030（`accountDeletionPending`）。
  2. 登录创建设备会话；`sid` ≥ 128 位随机（F-ACC-03 规则 1）；旧哈希参数升级后重算。
  3. `account_events` 和 `GET /v1/account/events`；`securityNotice` 下发（规则 3b）。保留 180 天，`account_events_purge`。
  4. 指标 `im_account_logins_total{result}`、`im_registers_total{result}`（第 14.2 节）。
- 工作量：1.5 人日（服务端）

### BE-415 会话续期：新一代首次使用才生效
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-413、BE-414
- AC：AC-ACC-03-1、AC-ACC-03-2、AC-ACC-03-3、AC-ACC-03-4、AC-ACC-03-15、AC-ACC-03-19、AC-ACC-03-21、AC-ACC-03-22、AC-ACC-03-24、AC-ACC-03-25、AC-ACC-03-27、AC-ACC-03-31、AC-ACC-03-32、AC-ACC-03-33、AC-ACC-03-35、AC-ACC-03-37、AC-ACC-03-38、AC-ACC-03-40、AC-ACC-03-44、AC-ACC-03-45、AC-ACC-03-46、AC-ACC-03-48、AC-ACC-03-49、AC-ACC-03-50
- 内容：
  1. refresh token 格式 `kid.sid.gen.nonce.mac`（规则 3）：独立的 refresh 密钥；**持久化密钥台账（`kid`、`retiredAt`），退役条件 365 天**；配置检查拦截提前删除。`/v1/account/token/refresh` 每 IP 每分钟 60 次。
  2. 规则 4 全部：判定顺序（`mac` → 会话存在 → 已吊销 → 已过期 → 代号 → 签发频率）；`pending` 只按数量淘汰（`IM_REFRESH_MAX_PENDING`）；`pending` 转正、`overlapUntil`、`overlapMinGen`；代号由 `nextGen` 分配，永不复用（RS5-05）；**不用确定性派生 token**。
  3. 条件更新 `UPDATE … WHERE sid=? AND row_version=? AND revoked_at IS NULL`（RC2-04）。签发频率每小时 20 次，只在要签发 token 的分支检查（RS6-05）；连续 6 小时触发给其他设备 `securityNotice`（RS5-14）。
  4. 转正才滑动到期时间（`IM_REFRESH_TTL`、`_WEB`），不超过 `IM_REFRESH_ABSOLUTE`（RS4-10）；响应带 `absoluteExpiresAt`。
  5. 每次刷新和转正比较 IP 前缀（`seen_ip_prefixes`），新前缀写事件；响应带其他会话数量和最近活跃时间（RS4-08）。指标 `im_refresh_reissue_total`、`im_refresh_replay_total`。
  6. `recoverBefore`（规则 6a）：按账号状态每次计算；`mac` 正确时 4019 都带。
  7. `session_purge` 每天清除过期 30 天以上的会话（规则 13）。**绑定令牌的删除随 C3**（`device_push` 还没有）。
  8. `im-server sessions revoke-all`（第 12.2 节，RS4-14，AC-ACC-03-50）：吊销全部设备会话，`reason=revoked`，不计重放。
- 工作量：3.5 人日（服务端）

### BE-416 auth 校验、踢线和周期复查
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`gateway`、`delivery`、`hub`）
- 依赖：BE-415
- AC：AC-ACC-03-5、AC-ACC-03-6、AC-ACC-03-16、AC-ACC-03-17、AC-ACC-03-28、AC-ACC-03-29、AC-ACC-03-30、AC-ACC-03-34、AC-ACC-03-36、AC-ACC-03-41、AC-ACC-03-47
- 内容：
  1. `auth` 和 `/v1/*` 的代号判定表（规则 4）；被替换的代：WS `kicked/tokenExpired`，HTTP 401 / 4011（`tokenSuperseded`）。过期或验签失败不触发转正。
  2. 吊销原因映射（规则 2）：`credentialsChanged` → `kicked/passwordChanged`，注销 → `kicked/accountDeleted`，其余 → `kicked/sessionRevoked`，都带 `recoverBefore`。踢线放在事务提交之后。
  3. **入表后复查**（规则 2）：不能照搬 `gateway.go` 第 166 行的 `err == nil &&`，出错用 1011 断开；同时覆盖 `sid`、代号、版本号、`credentials_changed_at`。`delivery.Session` 增加 `sid`、`gen`（第 11.4 节，RS5-08）。
  4. `delivery.Router` 增加按 `sid` 踢线和按"`sid` + 代号"踢线；转正时断开这条 `sid` 上代号不是新 `current` 的连接。
  5. 周期复查（`IM_SESSION_RECHECK`，10 分钟、总线重连后），连续失败 3 次才 1011（RS4-09、RS5-09）。
  6. 无 `sid` 的 token（规则 11）：`auth` 和 `/v1/*` 拒绝 `iat` 早于 `credentials_changed_at` 的；调账号接口 403 / 4030。`delivery.Presence` 和 `Session` 的前后台字段随 C3。
- 工作量：1.5 人日（服务端）

### BE-417 退出、改密、退出其他设备、凭证版本
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-416
- AC：AC-ACC-04-1、AC-ACC-04-2、AC-ACC-04-3、AC-ACC-04-4、AC-ACC-04-7、AC-ACC-04-8、AC-ACC-04-10、AC-ACC-04-11、AC-ACC-04-12、AC-ACC-04-14、AC-ACC-04-15、AC-ACC-04-16、AC-ACC-04-18、AC-ACC-04-19、AC-ACC-04-25、AC-ACC-04-29
- 内容：
  1. `logout`（规则 1）：Bearer 或 refresh 有一个有效就执行，幂等；`mac` 必须正确；任何一代的值都按退出处理，不算重放。
  2. `password`（规则 2、3）：吊销全部会话，给当前设备新 `sid`，响应带新 token 和新恢复码；写 `credentials_changed_at`；旧码按 F-ACC-05 规则 5。写库失败整体回滚。**"显示内容"设置的复制和令牌重新注册随 C3**。
  3. `logout-others`（规则 6），`revokeReason=loggedOutOthers`。强制下线和封禁同时吊销（规则 4）。
  4. **凭证版本**（规则 8，RS4-06、RS7-01）：所有用密码的操作，Argon2 在事务外，**提交事务第一条语句锁 `accounts` 行**，持锁后校验 `credentials_version`、调用方 `sid`、账号状态；不匹配 401 / 4018，不计失败。
  5. **AC-ACC-04-29 只有 Postgres 实现后才能跑**：测试代码随功能写好，SQLite 下跳过并标"未执行"，C1 验收不算通过（见 BE-422）。
- 工作量：2 人日（服务端）

### BE-418 恢复码和找回（F-ACC-05）
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-417
- AC：AC-ACC-04-13、AC-ACC-04-20、AC-ACC-04-23、AC-ACC-04-24、AC-ACC-04-30、AC-ACC-04-31、AC-ACC-05-1、AC-ACC-05-2、AC-ACC-05-3、AC-ACC-05-6、AC-ACC-05-8、AC-ACC-05-11、AC-ACC-05-12、AC-ACC-05-13、AC-ACC-05-14、AC-ACC-05-17、AC-ACC-05-18、AC-ACC-05-19
- 内容：
  1. 规则 1：`SHA-256(userId, code)`（长度前缀拼接，128 位随机，不用 Argon2 和 HMAC）；每代一行；`recovery-code/ack {gen}`；`recoveryCodeStale`；最新一代未确认时登录作废并重新生成；`recoveryCodeUnacked`。
  2. 规则 2：未命中不占闸门；命中不受"用户名 + IP"锁定和单哈希限制；专用位按 `(userId, gen)` 公平调度，代号更老的优先，进入前重查（RS6-02）。**成功次数上限只约束最新一代**（RS7-02），429 / 4290 `recoverLimited`。每 IP 每分钟 20 次在查库之前。
  3. 规则 3、5：旧码保留 7 天，`retire_until` 只写一次；旧码优先：只作废比所用代更新的代。`recovery-code`（要当前密码）。
  4. `recover` 事务（F-ACC-04 规则 8，RS5-02、RS6-01）：第一条语句锁行并递增版本（不带版本条件），原子检查这一行码可用，再作废更新的代。
- 工作量：1.5 人日（服务端）

### BE-419 昵称和演示、管理接口并存
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-418
- AC：AC-ACC-06-1、AC-ACC-06-2、AC-ACC-07-1、AC-ACC-07-2、AC-ACC-07-3
- 内容：
  1. `PATCH /v1/account/profile {nickname?}`（F-ACC-06 规则 1、2）；头像字段随 BE-437，C1 时带 `avatarFileId` 返回 404 / 4048。
  2. F-ACC-07 规则 2：有账号的用户 `/demo/login` 403 / 4030（`accountExists`）。
  3. 规则 3：生产带 `token.issue` 拒绝启动，除非 `IM_ALLOW_TOKEN_ISSUE=1`，配置检查 `token_issue_caller` 告警（R-CS-26）。
  4. 规则 6：`server/e2e` 现有文件不改，继续用演示和管理接口。
- 工作量：0.5 人日（服务端）

### BE-420 注销账号（F-ACC-08）
- 优先级：P1
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-418
- AC：AC-ACC-04-21、AC-ACC-04-28、AC-ACC-05-16、AC-ACC-08-1、AC-ACC-08-2、AC-ACC-08-3、AC-ACC-08-4、AC-ACC-08-5
- 内容：
  1. `POST /v1/account/delete`、匿名的 `delete/cancel`（规则 1、2）；冷静期内 `auth` 一律 `kicked/accountDeleted`（异常段，RC2-05）。
  2. 到期两步（规则 3）：事务内标记 `users.deleted_at`、递增 `token_version`、删密码和恢复码；幂等后台任务 `account_cleanup`（只在一个节点运行）删好友、黑名单，逐群退出，昵称改"已注销用户"。群主转让按 F-SOC-23 规则 4。
  3. `recover` 撤销注销（F-ACC-05 规则 5a）；`account_cleanup` 第 ① 步条件包含"注销申请没有被撤销"。
  4. 头像对象删除随 BE-437（AC-ACC-08-7）。保留口径实现前随 CH-23 确认（规则 6）。
- 工作量：1 人日（服务端）

### BE-421 邀请码和后台账号信息（管理接口）
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`server/internal/admin`）
- 依赖：BE-410、BE-413
- AC：AC-ACC-05-9、AC-ACC-09-1、AC-ACC-09-2、AC-ACC-09-5、AC-ACC-09-7、AC-ACC-09-9、AC-ACC-09-10、AC-ACC-09-12、AC-ACC-09-13、AC-ACC-09-14
- 内容：
  1. `POST /admin/invites`、`GET /admin/invites`、`POST /admin/invites/{id}/disable`（scope `account.invite`）：128 位随机，只存哈希和后 4 位；单次 ≤ 100（4131）；`maxUses` 1～1000（4000）；`invite_purge`（过期或用完 90 天）。
  2. `GET /admin/accounts/{userId}`（`user.read`）：账号信息，没有哈希和令牌。
  3. 配置检查：有效邀请码剩余次数合计超过 `IM_INVITE_WARN_TOTAL` 告警（RC2-18）。`admin` 路由和 `console` scope 的枚举测试：没有任何改密码的接口（AC-ACC-05-9）。
  4. `AC-KEY-02-2` 的参数化表加 `account.invite`。
  5. 邀请码自动停用要告警到人；**第一批不做重新启用，用新建邀请码恢复**（PRD v1.1 第 12.2 节）。
- 工作量：1 人日（服务端）

### BE-422 C1 后端验收：攻击序列和全量用例
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-419、BE-420、BE-421、BE-423、CS-200
- AC：AC-ACC-04-29、AC-CHAT-90-1、AC-CHAT-90-2、AC-CHAT-90-5、AC-CHAT-90-11、AC-CHAT-90-17
- 内容：
  1. 按 TC-ACC 的编号落地后端 e2e，文件名 `account_*_test.go`，用例名带 TC 编号；攻击序列（F-ACC-03～05）单独放一个文件，每条写清谁、什么顺序、时钟怎么推。
  2. AC-CHAT-90-5：`deploy/ops/scan-logs` 扫 C1 流程。
  3. 性能（AC-CHAT-90-1、90-2、90-11）：开发机只记录，**不算通过**，发布判定等 P-12；报告写基线提交号，进 `docs/reports/`。
  4. **AC-ACC-04-29 验收记未执行**。AC-ACC-08-5 的 P99 也只是开发机参考。
- 工作量：5 人日（测试）

### BE-423 C1 部署和运维手册（第 12.2 节）
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`deploy/`、`docs/`）
- 依赖：BE-415、BE-421
- AC：AC-CHAT-90-17
- 内容：
  1. **运维手册：数据库恢复**（RS4-14）：恢复到旧时间点后，轮换过的客户端的值比库里新，已吊销的会话和已改的密码回到旧状态。步骤：运行 `im-server sessions revoke-all`，用户重新登录；告知恢复点之后改过密码的用户重新确认。
  2. **运维手册：refresh 密钥台账**（F-ACC-03 规则 3，RS5-04）：`kid`、`retiredAt`；退役条件 365 天，不是 JWT 的 30 天；配置检查拦截提前删除；**密钥泄露的应急操作：主动提前删除并接受全员重新登录**。
  3. **部署说明：NAT 和可信网段**（RS4-15、RS7）：全公司共用注册 20 次 / 小时、登录 20 次 / 分钟，同办公室的人能用"用户名 + 同一 IP"锁住同事；`recover` 每 IP 每分钟 20 次可以短时压制同网用户，换移动网络能绕开；可信出口网段可配更高额度（配置项名 `IM_TRUSTED_EGRESS_CIDRS`，PRD 没有定义，实现时加）。
  4. 部署说明：`GOMEMLIMIT` 把 Argon2 算进去（第 15 节）；`IM_ALLOW_TOKEN_ISSUE`；注册模式。
  5. 第 14.2 节：配置检查（注册模式、邀请码总量告警、隐私说明版本、`token.issue`）；账号指标；告警规则（重放吊销突增、登录失败突增，第一批默认不发页面告警）。标签没有无界字段（AC-OPS-90-4）。
- 工作量：3 人日（部署 1.5 + 文档 1.5）

### BE-424 隐私告知清单草稿
- 优先级：P0
- 批次：C1
- 仓库：tianting-server（`docs/`）
- 依赖：BE-413
- AC：无（文档任务）
- 内容：
  1. F-ACC-10 规则 3 的告知项清单。**只提供清单草稿，法律文本由 Frank 或法务提供（第 20.3 节）**，不代替法律文本。
  2. 法务问题清单（第 20.1a 节）里和 C1、C2 相关的项单独标出（注销后的保留、用户名公开、文件 180 天）。
  3. 启动保护（规则 4）依赖 `IM_MSG_ACTIONS_ENABLED`、`IM_MOD_ENABLED`，属于 C4、C6，不在这里。
- 工作量：1 人日（文档）

---

## C2 图片和文件

### BE-430 对象存储客户端
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`server/internal/objstore`）
- 依赖：BE-404、BE-422
- AC：AC-FILE-04-4、AC-FILE-04-6
- 内容：
  1. 第 11.3 节：`tianting-console-server/internal/objstore` 不能直接复用（另一个模块的 `internal`、强制 Object Lock、没有预签名）。新写，复用 SigV4 思路，不引入 SDK。
  2. 能力：预签名 PUT（签入 Content-Length、Content-Type）、预签名 GET（带 `response-*`）、HEAD、Range GET（`If-Match`）、CopyObject（条件 + 元数据和标签替换）、DELETE、按前缀 List。**预签名一律用真实时钟**（第 4.4 节）。
  3. 配置项和凭证按前缀最小授权（第 11.3 节）：全桶 List 不给 im-server。清理不用请求上下文。
- 工作量：1.5 人日（服务端）

### BE-431 文件表、配置和开关
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-430
- AC：AC-CHAT-90-2、AC-FILE-01-14、AC-FILE-05-6
- 内容：
  1. 迁移：`files`（第 12.1 节全部字段，含 `scan_state` 默认 `skipped`）、`upload_quota`；`conv_seq.mod_ver`。`last_recalled` 等随 C4。
  2. 第 4.1 节的文件配置项；`IM_FILES_ENABLED` 关时 `/v1/files/*` 404、`send` 的 `image`、`file` 4000（**不是共同规则的 4030**，按 F-FILE 的写法）；`GET /v1/config` 的 `features.files`、`limits`。
  3. PUT 地址 15 分钟要有配置项（AC-FILE-01-16 要调短，PRD 没有名字，问题 8），名字 `IM_UPLOAD_PUT_TTL`。
- 工作量：1 人日（服务端）

### BE-432 申请上传、配额和取消
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-431
- AC：AC-ACC-01-20、AC-ACC-06-4、AC-CHAT-90-12、AC-FILE-01-4、AC-FILE-01-6、AC-FILE-01-7、AC-FILE-01-11、AC-FILE-01-16、AC-FILE-01-18、AC-FILE-01-23、AC-FILE-01-28、AC-FILE-01-29、AC-FILE-02-16、AC-FILE-03-2、AC-FILE-05-1、AC-FILE-05-2
- 内容：
  1. `POST /v1/files/uploads`（F-FILE-01 规则 1）：只签名，不访问存储（规则 8）；对象键服务端随机。
  2. 规则 6、6a：申请时检查"已用 + 声明"；频率 60 次 / 分钟；未完成数只统计 PUT 地址没过期的；`IM_UPLOAD_DAILY_DECLARED`。`open` 模式注册 7 天内 200 MiB（第 4.1 节）。
  3. `DELETE /v1/files/uploads/{fileId}`：`pending` → `purging` 的条件更新，不看租约；响应不暴露存在性（RS4-23、RS5-16）。
  4. 规则 7 文件名清洗：NFC；去路径分隔符、控制字符、Bidi、Default_Ignorable、开头的点、结尾的点和空格、`::` 之后；再判断扩展名。255 字节。
  5. F-FILE-05 规则 1 的禁止名单配置文件；`kind=image` 白名单（SVG、BMP、TIFF 415）；大小限制（413 / 4139）。
- 工作量：2 人日（服务端）

### BE-433 complete：租约、嗅探、条件 Copy
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-432
- AC：AC-FILE-01-1、AC-FILE-01-2、AC-FILE-01-3、AC-FILE-01-5、AC-FILE-01-8、AC-FILE-01-9、AC-FILE-01-10、AC-FILE-01-13、AC-FILE-01-15、AC-FILE-01-18、AC-FILE-01-19、AC-FILE-01-20、AC-FILE-01-21、AC-FILE-01-22、AC-FILE-01-26、AC-FILE-01-27、AC-FILE-02-11、AC-FILE-05-1、AC-FILE-05-4
- 内容：
  1. F-FILE-01 规则 3：先取租约（60 秒，每次尝试结束按 `completing_attempt` 释放）；Copy 前查额度；每个变体 HEAD 记 ETag → `If-Match` Range 读 64 KB 嗅探、解析头部真实宽高 → 条件 Copy 到 `final/{fileId}/{attemptId}/{variant}`，元数据和标签都 REPLACE，只写服务端确定的 `Content-Type` → 再 HEAD 核对大小和 Copy 响应的 ETag。
  2. 412 内部重试 1 次，再失败 `uploadMismatch`，临时对象不删。全部变体成功后提交事务 `WHERE state='pending'`，再删临时键；**不跳过已成功的变体**；败者只删自己的键；提交后按前缀删除早于租约的其他尝试。HEAD 404 或提交失败先重读状态。
  3. 规则 3b：缩略图长边 ≤ 512，头像 ≤ 1024，GIF 帧数上限；F-FILE-02 规则 1：宽高 ≤ 10000、像素 ≤ 4000 万。
  4. 规则 6：提交时复查，已达额度 429 / 4292，**当日最多超额一个文件**。
  5. F-FILE-05 规则 1：真实类型矛盾 415 并删除；无法识别的文件 `application/octet-stream`，图片拒绝。
- 工作量：3.5 人日（服务端）

### BE-434 发送：image、file 和判定顺序
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`server/internal/logic`、`protocol`）
- 依赖：BE-433、BE-401
- AC：AC-FILE-02-1、AC-FILE-02-2、AC-FILE-02-3、AC-FILE-02-11、AC-FILE-02-15、AC-FILE-03-1、AC-FILE-06-4、AC-FILE-06-7、AC-FILE-06-8、AC-FILE-07-8
- 内容：
  1. 第 4.3 节判定顺序在 C2 的部分：第 1 步格式、第 6 步附件预检（4048、4000 `fileKindMismatch`）、第 10 步 `SaveMessage` 事务内 `UPDATE files SET state='attached', msg_id=? WHERE … AND state='ready'`（影响 0 行 4096）。第 7、8、9 步属于 C4、C6。第 6～10 步失败退还 `newPeerReject`（R-CB-20）。加锁顺序 `conv_seq` → 文件行。
  2. 下发用服务端记录的值（F-FILE-02 规则 1）；`lastMsg`：`type=image|file`，`text=""`（不含文件名）。
  3. `MsgBody` 互斥（BE-401）。`SaveMessage` 失败时文件保持 `ready`（AC-FILE-07-8）。
  4. `/admin/convs/messages/query` 响应文件消息只有类型和大小（R-CS-14）；`body` 接口带文件名，凭正文授权。见 CS-201。
- 工作量：2 人日（服务端）

### BE-435 访问控制：批量取地址
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-434
- AC：AC-FILE-01-25、AC-FILE-04-1、AC-FILE-04-2、AC-FILE-04-3、AC-FILE-04-4、AC-FILE-04-5、AC-FILE-04-7、AC-FILE-04-8、AC-FILE-04-9、AC-FILE-05-5
- 内容：
  1. `POST /v1/files/urls`（F-FILE-04 规则 1）：单次 ≤ 50（4131）；图片、缩略图 120 秒，文件 300 秒；逐项 `code`。
  2. 规则 2：单聊参与者；当前群成员且 `seq ≥ minSeq`；消息没被删除；文件没过期；发送者绑定前可以访问。**清空位置（F-CVS-03）和撤回的判定随 C4、C5 补**。
  3. F-FILE-03 规则 4：签入 `response-content-type` 和 `response-content-disposition`（RFC 5987 / 6266，ASCII 回退名不含 `"` `;` `%`）；非图片 `attachment`，图片 `inline`。
  4. 每人每分钟 120 批（4290）；`im_file_url_issued_total` 按用户统计不进标签。预签名查询串和文件名不进日志。
- 工作量：1.5 人日（服务端）

### BE-436 生命周期：file_purge 和 mod_ver
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-435
- AC：AC-FILE-01-24、AC-FILE-01-26、AC-FILE-06-1、AC-FILE-06-2、AC-FILE-06-5、AC-FILE-06-7、AC-FILE-06-9、AC-FILE-06-10、AC-FILE-06-11、AC-FILE-06-12、AC-FILE-06-13
- 内容：
  1. F-FILE-06 规则 2：状态机；`purging` 是中间态；`pending` → `purging` 只匹配租约过期或没有租约的行。
  2. 规则 3：180 天到期，对象删除，`expired=true`，清除 `files.name`，递增 `mod_ver`（F-RCL-03 规则 2 从 C2 开始实现）。规则 4 的**后台删除**路径：`revoked`、文件名进隔离加密区、对象保留 30 天。撤回路径随 C4。**这条路径的物理删除只有 C4 的 AC**（问题 10），本批用 TC-FILE-04-05 验到 `revoked` 为止。
  3. 规则 7：`file_purge` 每小时一批 1000，只在一个节点运行；按 `final/{fileId}/` 前缀删除；`public_key` 不空先删公开对象，成功后才删行。指标 `im_job_runs_total{job="file_purge"}`。
  4. 规则 5 的证据保留（`evidence_until`）随 C6，列先建好。
- 工作量：1.5 人日（服务端）

### BE-437 头像（F-ACC-06 的 C2 部分）
- 优先级：P0
- 批次：C2
- 仓库：tianting-server
- 依赖：BE-436、BE-419
- AC：AC-ACC-06-3、AC-ACC-06-7、AC-ACC-06-8、AC-ACC-06-9、AC-ACC-06-10、AC-ACC-06-11、AC-ACC-06-12、AC-ACC-06-13、AC-ACC-08-7
- 内容：
  1. 规则 3、4：`kind=avatar`；`complete` 到私有最终键；`PATCH profile` 才 Copy 到公开读桶。**公开键由服务端生成，先写 `files.public_key` 再 Copy**，重试复用（RS6-04）。相同 `avatarFileId` 重复绑定按成功处理。
  2. `retain_until` 为空；`avatar_replaced_at`，7 天内绑回清空；7 天后或注销清理时走 `purging`。
  3. 对象存储不可用时昵称成功、头像 503 / 5032。写 `users.avatar`（https，≤ 512）。
- 工作量：1.5 人日（服务端）

### BE-438 C2 部署：桶、生命周期、CORS、部署说明
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`deploy/`、`docs/`）
- 依赖：BE-437
- AC：AC-CHAT-90-3、AC-CHAT-90-17
- 内容：
  1. 桶策略（F-FILE-01 规则 9、第 21 节必需能力）：私有桶、公开读桶和独立域名；匿名和预签名禁止列举；服务端加密；临时前缀 1 天生命周期（规则 3a）；不开版本控制（规则 F-FILE-06 第 8 条）。凭证按前缀最小授权。
  2. 部署说明：CORS 限定来源；Android 开发环境 https 或 debug 例外（规则 10）；访问日志关闭或脱敏；备份不含文件内容（第 14.2 节）；恢复数据库后多余对象的手动对账任务（F-FILE-06 规则 8）。
  3. 配置检查：对象存储连通和 HTTPS；文件指标（第 14.2 节）；告警：对象存储错误率、当天全局上传量（含临时前缀，默认不发页面告警）。
- 工作量：3 人日（部署 2 + 文档 1）

### BE-439 对象存储合同测试接入
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`tools/objstore-contract`、`.github`）
- 依赖：BE-407、BE-430
- AC：AC-FILE-90-1、AC-FILE-90-2
- 内容：
  1. **AC-FILE-90-1**：`tools/objstore-contract` 已有合同工具，2026-11-06 在 MinIO 上 46 项必需全部通过（[GATE-objstore](../review/GATE-objstore-2026-11-06.md)）。这次要把实现（BE-430）也跑到 MinIO 上。**CI 现在不运行（PENDING P-21，Actions 预算是 $0）；开发机不装软件；真实 MinIO 这一项要 Frank 再次同意临时构建才能跑**（`tools/objstore-contract/run-local-minio.sh`，构建到临时目录、只监听 `127.0.0.1`、跑完删除）。没跑时状态写"未执行"，不算通过。
  2. 工具没有验证的两项如实写进部署说明：生命周期实际清除（只查配置）、CORS 限定来源。AC-FILE-90-1 要求"检查配置生效；实际清除按厂商周期记录"。
  3. **AC-FILE-90-2 等云厂商**（CH-05 还没选）：上线前跑同一套工具，凭证放环境变量，不写进文件和聊天；不通过可能要改上传设计（R-18）。前端走查部分见 FE-640。这条是手工 AC，`trace.py` 要求有 FE 任务。
- 工作量：1.5 人日（部署 1 + 文档 0.5）

### BE-440 C2 后端验收
- 优先级：P0
- 批次：C2
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-438、BE-439、CS-201、CS-202
- AC：AC-CHAT-90-1、AC-CHAT-90-2、AC-CHAT-90-3、AC-CHAT-90-5、AC-CHAT-90-12、AC-CHAT-90-17
- 内容：
  1. 按 TC-FILE 落地后端 e2e，`file_*_test.go`，用例名带 TC 编号；F-FILE-01 的攻击序列单独放一个文件。
  2. AC-CHAT-90-3：对象存储按拒绝连接和黑洞两种方式不可用。AC-CHAT-90-5：扫 C2 流程。
  3. 性能（90-1、90-2、90-3、90-12）开发机只记录，不算通过，发布判定等 P-12。
- 工作量：5.5 人日（测试）

---

## PRD 问题的处理

分发时发现的矛盾和缺口已在 [PRD-chat v1.1](../prd/PRD-chat-v1.1.md) 第 22 节修正，这里不再单独记录。
