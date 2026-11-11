# 后端任务单：天听聊天产品第一批（C1 账号、C2 图片和文件、C3 离线推送、C4 撤回引用 @、C5 会话设置、C6 敏感词和举报、公共部分）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4～10、11、12、14、15、16、17、18、20.3 节；[ARCH-v1](../arch/ARCH-v1.md)；[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[管理接口](../../protocol/ADMIN.md) |
| 范围 | **C1、C2、C3、C4 和公共部分**（PRD 决定 CH-27）。C3 是 Frank 2026-11-10 决定按顺序继续的，**同一天又决定"推送后续做，先做功能"，C3 已暂停**（任务和用例保留）；C4 是同一天决定接着做的；**C5 是 Frank 2026-11-10 在聊天里说"开始 C5"后分发的**（第 17 节 C5 行，QUEUE Q29）；**C6 是 Frank 2026-11-11 在聊天里说"开始 C6"后分发的**（第 17 节 C6 行，PENDING P-47，QUEUE Q30）。【全部】的 AC 验证到 C6 为止的部分（PRD 第 15 节累计表，C3 那一列等 C3 恢复） |
| 负责 | 后端（AI） |
| 仓库 | im-server（GitHub 仓库 `tianting-server`，本地 `im-mvp/server/`）；协议在同仓库 `protocol/`，部署在 `deploy/`，工具在 `tools/` |
| 编号 | **BE-400 起**。公共 BE-400～BE-408，C1 BE-410～BE-424，C2 BE-430～BE-440，C3 BE-450～BE-460，C4 BE-470～BE-481，C5 BE-482～BE-490，C6 BE-491～BE-499 |
| 起点 | 现有迁移到 `0014_admin_stats.sql`；只有 SQLite；`IM_V13_ADMIN_FEATURES` 默认关闭；`tools/objstore-contract` 已有合同工具 |
| 完成定义 | 任务内的 AC 都有通过的自动化测试，用例名带 TC 编号（[TC-ACC](../testcases/TC-ACC.md)、[TC-FILE](../testcases/TC-FILE.md)、[TC-APN](../testcases/TC-APN.md)、[TC-RCL](../testcases/TC-RCL.md)、[TC-QUO](../testcases/TC-QUO.md)、[TC-MEN](../testcases/TC-MEN.md)、[TC-CVS](../testcases/TC-CVS.md)、[TC-MOD](../testcases/TC-MOD.md)、[TC-CHAT](../testcases/TC-CHAT.md)）；`go test -race ./...` 全过；默认配置下 `server/e2e` 现有文件不改代码全过（AC-ACC-07-1）；Schema、向量、`ERRORS.md`、`ADMIN.md` 同步；`python3 -I tools/trace.py --check` 没有缺口 |
| 环境限制 | **CI 现在不运行**（[PENDING](../reports/PENDING.md) P-21，Actions 预算是 $0），所有结果都是本地结果，要标明。开发机不装软件；真实 MinIO 要 Frank 同意才能临时构建。压测机 P-12、真机和渗透 P-13 未到位。**C3 的真实 APNs、FCM 要 Frank 先做 PRD 第 20.3 节的账号和网络步骤**（PENDING P-40），之前只用假通道 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。一条 AC 可以出现在多个任务里（功能任务 + 验收任务）。C6 的 AC 从 2026-11-11 分发起进 `- AC：` 行。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-400 | ✅ 完成（服务端部分） | errors.json、ERRORS.md、kickReasons、`kicked.recoverBefore`、`securityNotice`、`recoveryCodeUnacked`、`http-client.schema.json` 账号部分、ADMIN.md、README.md。AC-CHAT-90-14 服务端一致性（TC-CHAT-90-12）通过；**三套 SDK 枚举未做（FE-600～602）**。ADMIN.md 的账号接口随 BE-421 实现 |
| BE-402 | 🟡 部分完成 | 已做：`frames.json` 的 `kicked.recoverBefore`、`securityNotice` 用例；`account-session.json` 和 Go 参考客户端（`server/e2e/accountmodel`）覆盖规则 5 单飞、规则 6 和 6a 失败分类、规则 7 的 4011；规则 4 判定表 `serverCases`（参考模型 `server.go` 产出，真实服务端跑 HTTP 和 WS 两遍，`e2e/account_vectors_test.go`）。**未完成**：三套 SDK 跑向量（FE-600～602），所以 AC-CHAT-90-10、90-15 未完成 |
| BE-405 | 🟡 部分完成 | 已做：`testhook`（`-tags imhooks`，nm 断言生产构建没有符号）、`IM_TEST_DISABLE_AUTH_LOCK`、`limits/reset scope=auth`、Argon2 阻塞、注册事务失败、上海 00:00 时钟助手、生产拒绝启动（四个开关和带钩子的二进制）。AC-CHAT-90-18、AC-ACC-02-3 通过。AC-ACC-04-18、04-19、05-11 通过（BE-417、418）。吊销写失败、入表前吊销、入表后和周期复查失败、丢弃吊销事件、入表前暂停都已接上（BE-416、417）；**留到后续**：`IM_TEST_DISABLE_IDLE` 等三个开关现在只拦生产，功能随 BE-406 |
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
| BE-422 | 🟡 后端用例完成，AC-ACC-04-29 未执行，性能未判定 | 报告 `docs/reports/account-c1-acceptance-2026-11-06.md`。攻击序列对应表 `e2e/account_acceptance_test.go`；变异确认 `tools/account-mutation/run.py`，26 条全部被发现。AC-CHAT-90-5（im-server 部分）、90-17 通过；AC-CHAT-90-2 现有 e2e 全过，TC-PERF 等 P-12。**AC-ACC-04-29 未执行不算通过；AC-CHAT-90-1、90-11 开发机只记录；CS-200 还没做，依赖它的部分没验** |
| BE-401 | ✅ 完成（服务端部分） | `7878e4b`。`errors.json`、`ERRORS.md`（415x 区段、4096、4139、4150、5032，4131 加 `client`，文件的 reason）、`frame.schema.json`（`image`、`file`、`BodyByType` 互斥、`modVer`）、`http-client.schema.json` 文件部分、`frames.go` 的 `Validate`。**本地结果，CI 不运行**：`internal/protocol` 的 Schema 和 Validate 用例通过（含 TC-FILE-02-15 的协议部分）。**AC-CHAT-90-10 只有服务端部分**：`frames.json` 的 image、file 用例随 BE-403，三套 SDK 随 FE-600～602 |
| BE-403 | ✅ 完成 | `6c0829f`。**本地结果，CI 不运行**。① `protocol/vectors/chat-sync.json` 6 个用例（`expired` 只能从假变真、迟到的旧 `sync` 不回写、只重拉 `modVer` 变了的已加载会话、老服务端全部重拉、重拉失败下次再拉、没加载过的会话以 `convs` 为基线），期望值由 Go 参考客户端 `server/e2e/chatmodel` 生成（`-update`），`TestChatModelRules` 另外断言关键结论。"迟到的旧应答"用后一次重拉的应答带旧内容来表达（不依赖各端请求并发的细节）。② `account-session.json` 加 `attachmentFormat`、`attachmentCases` 11 个（F-FILE-07 规则 1、3、4：处理中→上传中→发送中、离线待发送、上传失败重发、复用 fileId、4096 有源/无源、自己的 4048 有源/无源、别人的 4048、2 个冷启动），模型 `server/e2e/accountmodel/attach.go`。③ `frames.json` 加 11 个合法、18 个非法（`image`、`file`、`MsgBody` 互斥、`modVer`）；服务端 `logic.TestFrameVectorsServerParse` 用真实帧解析跑全部 `send` 用例（严格解码 + `SendReq.Validate`），image、file 下行消息用服务端模型解码再编码。三套 SDK 同步副本（`protocol/SOURCE`=6c0829f）并加执行器，全部通过，SDK 没有发现 bug：sdk-ts `chat-sync-vectors.test.ts`、`attachment-vectors.test.ts`（冷启动 2 个跳过，H5 没有持久化）；sdk-kotlin `ChatSyncVectorsTest`、`AttachmentVectorsTest`；sdk-swift `ChatSyncVectorTests`、`AttachmentVectorTests`。AC-CHAT-90-10、90-15 通过（C2 范围）。新问题 32、33，**第 32 条要 Frank 定** |
| BE-404 | 🟡 部分完成（im-server 部分） | `7878e4b`。`internal/objstore/objstoretest`：校验 SigV4（请求头和预签名）、真实时钟过期、签入的大小和类型、未签入的 `x-amz-*`、ETag 条件、Copy 的元数据替换、`response-*`、匿名和列举权限、按前缀列举权限；注入不可用（拒绝连接、黑洞、503）、删除失败、Copy 失败、请求之后的回调（HEAD 之后替换）、直接写入、临时前缀生命周期。暂停点在 `files.Hooks`（测试模式）。**和第 4.5 节的差异：假 S3 在测试进程里、监听独立端口，不是独立 OS 进程**（AC-FILE-01-8 验的是字节只到对象存储的端口）。CORS 没有模拟（随 BE-438）。AC-FILE-01-8、01-15 通过（见 BE-433） |
| BE-430 | ✅ 完成 | `7878e4b`。`internal/objstore`：自写 SigV4（AWS 公开预签名向量通过），预签名 PUT / GET、HEAD、Range GET（If-Match）、条件 Copy（REPLACE）、DELETE、List、按前缀删除；签名用真实时钟；错误不带地址和签名。`IM_OBJSTORE_*` 配置（PRD 没有定义名字，按任务单第 3 条加）。**本地结果，CI 不运行**：AC-FILE-04-4（TC-FILE-04-04，含真实 6 秒过期）、AC-FILE-04-6（TC-FILE-04-06，假 S3）通过。**真实存储上的行为没有验证**（AC-FILE-90-1 随 BE-439，未执行） |
| BE-431 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。`0017_files.sql`（`files` 第 12.1 节全部字段 + `declared_*`、`put_expires_at`、`objects_deleted` 等，`upload_quota`，`conv_seq.mod_ver`），`store.FileStore` 和契约用例 5 个；第 4.1 节文件配置；`IM_FILES_ENABLED` 关：`/v1/files/*` 404、`send` 的 image / file 4000；`GET /v1/config` 的 `features.files`、文件 `limits`；`IM_UPLOAD_PUT_TTL`。**本地结果，CI 不运行**：AC-FILE-01-14、05-6 通过；AC-CHAT-90-2 的 e2e 部分（默认配置现有 e2e 不改代码全过）通过，TC-PERF 等 P-12 |
| BE-432 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。`POST /v1/files/uploads`、`DELETE /v1/files/uploads/{id}`；额度、声明合计、未完成数在一个事务里；频率 60 次 / 分钟；文件名清洗（NFC、`::`、分隔符、控制、Bidi / Default_Ignorable、首尾点）；禁止名单（内置 + `IM_FILE_BLOCKLIST` 只能加）；图片白名单；大小。**本地结果，CI 不运行**：AC-ACC-01-20、06-4，AC-FILE-01-4、01-6、01-7、01-11、01-16、01-18、01-23、01-28、01-29、02-16、03-2、05-1、05-2 通过。**AC-CHAT-90-12 只有开发机参考**（1000 次顺序申请 P99 约 0.4 ms），不算通过 |
| BE-433 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。complete：租约（[IM 钟]，按 attempt 释放）、Copy 前查额度、HEAD → If-Match Range 64 KB 嗅探（png / jpeg / gif / webp 头部宽高，可执行、脚本、HTML、SVG 禁止）→ 条件 Copy 到 `final/{fileId}/{attemptId}/{variant}`（REPLACE，只写服务端类型）→ 再 HEAD 核对 Copy 返回的 ETag；412 重试 1 次；提交 `WHERE state='pending'` 并复查额度；败者只删自己的键；提交后删临时键和其他尝试；HEAD 404 / 提交失败先重读。**本地结果，CI 不运行**：AC-FILE-01-1、01-2、01-3、01-5、01-8、01-9、01-10、01-13、01-15、01-18、01-19、01-20、01-21、01-22、01-26、01-27、02-11、05-1、05-4 通过（攻击序列在 `e2e/file_complete_test.go`） |
| BE-434 | ✅ 完成（本任务 AC 都有用例，端到端 AC 只验后端） | `a71a634`。判定顺序第 1 步（开关 4000、互斥）、第 6 步附件预检（4048、`fileKindMismatch`、4096）、第 10 步 `SaveMessage` 事务内绑定（4096，文件保持 ready）；第 6～10 步失败退还新对象配额（R-CB-20，**4137 也退还了，原来不退**）；正文只存 `fileId`，下发时按文件记录填写（过期 `expired=true`、文件名清空）；`lastMsg.text=""`；admin 正文接口带文件名，元数据接口没有。**本地结果，CI 不运行**：AC-FILE-02-2、02-3、02-11、02-15、06-4、06-7、06-8、07-8 通过；**AC-FILE-02-1、03-1 是端到端 AC，只验了后端部分**（客户端显示随 FE） |
| BE-435 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。`POST /v1/files/urls`：≤ 50（4131）、120 批 / 分钟（4290）、逐项 `code=4048`（和不存在完全相同）；单聊参与者、群成员 + `seq ≥ minSeq`、消息没删、文件没过期、上传者绑定前；签入 `response-content-type` 和 RFC 5987 / 6266 的 `response-content-disposition`（图片 inline、文件 attachment）；`im_file_url_issued_total`（无用户标签）。**本地结果，CI 不运行**：AC-FILE-01-25（后端部分）、04-1、04-2、04-3、04-4、04-5、04-7、04-8、04-9、05-5 通过。清空位置、撤回随 C4、C5 |
| BE-436 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。`file_purge`（每小时一批）：24 小时未绑定 → purging（租约条件）；换头像 7 天 → purging；保留期到 → expired（清空文件名、递增 `mod_ver`）后按前缀删对象；后台删除 → revoked（同一事务，文件名随原文进隔离加密区），30 天后删对象；purging 先删公开对象再删前缀和行；删除失败保留、下次重试。**本地结果，CI 不运行**：AC-FILE-01-24、01-26、06-1、06-2、06-5、06-7、06-9、06-10、06-11、06-13 通过；AC-FILE-06-12 只验了服务端（`modVer` 递增），SDK 重拉随 FE |
| BE-437 | ✅ 完成（本任务 AC 都有用例） | `a71a634`。`PATCH profile` 的 `avatarFileId`：先写 `public_key` 再 Copy 到公开桶，重试复用；绑定事务写 `users.avatar`、旧头像 `avatar_replaced_at`，绑回清零；注销清理把头像改 purging；先改昵称再绑头像（存储不可用时昵称成功、头像 5032）。**本地结果，CI 不运行**：AC-ACC-06-3、06-7（`nosniff` 是厂商能力没有验）、06-8、06-9、06-10、06-11、06-12、06-13、08-7 通过 |
| BE-438 | ✅ 完成（部署样例和检查；厂商生效未验证） | `deploy/FILES-RUNBOOK.md`（桶、凭证、生命周期、CORS、Android 开发例外、访问日志、备份不含文件、恢复后手动对账、配置检查、指标、告警）；`deploy/objstore/` 样例只有占位符（`policy-im-server.json` 只给 `final/` 前缀列举、`policy-public-read.json` 没有列举、`lifecycle-private.json` `tmp/` 1 天、`cors-private.json` 限定来源、`im-files.example.env` 凭证留空），`test_samples.py` 11 项静态检查。服务端：配置检查 `objstore_https`（生产 http 本来就拒绝启动，补了单测 `TestObjstoreHTTPSCheck`）、`objstore_reachable`（按 `final/` 列举 1 个键）；指标 `im_file_uploads_total{kind,result}`、`im_objstore_errors_total{op}`、`im_objstore_requests_total{op}`，**`im_file_uploaded_bytes_total` 改名为第 14.2 节的 `im_file_bytes_total`**。告警只写在部署说明（第一批不发页面告警，`alerts.yml` 是 F-OPS-07 的 11 条）。**本地结果，CI 不运行**：AC-CHAT-90-17（C2 部分，TC-CHAT-90-16）通过；AC-CHAT-90-3 见 BE-440 |
| BE-439 | 🟡 第 1、2 条完成；第 3 条等云厂商 | 第 2 条：生命周期实际清除、CORS 限定来源写进 `FILES-RUNBOOK.md` 第 7 节，另记了"无列举权限时不存在的键返回 403 还是 404"。第 1 条：`server/e2e/file_realstore_test.go`（`TestTC_FILE_90_01_RealStore`，设 `IM_E2E_S3_*` 才运行，默认跳过；同一套检查在假 S3 上跑通 `TestRealStoreChecksOnFake`，不算 AC 通过），`run-local-minio.sh` 加 `RUN_IM_E2E=1`。**2026-11-09 Frank 同意（P-38 ④）后由主会话在真实 MinIO 上运行：合同工具 46 项必需全过，`TestTC_FILE_90_01_RealStore` 6 个子用例全过，AC-FILE-90-1 通过；跑完已删除 MinIO**。AC-FILE-90-2 等 CH-05 |
| BE-440 | 🟡 后端用例完成，AC-FILE-90-1、90-2 未执行，性能未判定 | 报告 [c2-backend-acceptance-2026-11-09.md](../reports/c2-backend-acceptance-2026-11-09.md)。`e2e/file_acceptance_test.go`：F-FILE-01 攻击序列对应表（22 条，`TestFileAttackSequencesHaveTests`）、TC-CHAT-90-03（拒绝连接和黑洞各 1000 条文本）、TC-CHAT-90-05（C2 流程日志和除 `files` 外全部表含 `admin_audit`，用 `scan-logs`）、TC-CHAT-90-16。`TestTC_CHAT_90_12_UploadRequestLatency` 按 TC 编号改名为 `TestTC_CHAT_90_11_…`。**本地结果，CI 不运行**。性能（90-1、90-2 的 TC-PERF、90-3、90-12）开发机只记录，不算通过，等 P-12 |
| BE-450 | ✅ 协议部分完成 | `c940ce8`。**本地结果，CI 不运行**。`frame.schema.json` 加 `DeviceNotificationRegisterReq`、`DeviceNotificationUnregisterReq`、`DeviceNotificationSettingsReq`、`PresenceReq`（令牌上限 4096），ack 加 `showDetail`；`frames.json` 加 11 个合法、11 个非法；`server/internal/protocol/device.go`（命令名、请求结构、`Validate`）；`logic.TestTC_CHAT_90_24_DeviceFrameVectorsServerParse` 用服务端真实解析跑全部新用例（AC-CHAT-90-10 的 C3 服务端部分，TC-CHAT-90-24）；`README.md` 命令表、`ERRORS.md` 的 4030 `featureDisabled` 和注册的 4290 / 4000 文案（不新增错误码，`errors.json` 不变）。变异 4 条（令牌上限、空令牌、`env` 取值、`state` 取值）都被发现。**AC-APN-01-13、02-17 的行为（429 / 400 / 4030）要等 BE-451、BE-452 的服务端处理，没有验**；三套 SDK 副本随 FE-650、651 |
| BE-406、BE-451～BE-460 | ⏸ 暂停（Frank 2026-11-10：推送后续做） | C3 第一波开工后 Frank 决定先做 C4。这几项**没有提交任何代码**（做到一半的改动已从工作区撤掉，见文末"C3 暂停时的状态"）。P-40 ①～④ 已定（PRD v1.5），恢复时 BE-457 可以一起做 |
| BE-470 | ✅ 完成（协议和服务端解析） | `62a9c8e`。**本地结果，CI 不运行**。`frame.schema.json`：`RecallReq`、`QuoteRef`（上行）/ `QuoteView`（下行，四种 `state`，后三种没有 `text`）、`Mentions`（`userIds` 个数不在 Schema 里拦）、`BodyByType` 的 text 分支、`SendReq` 单聊不能带 `mentions`、`Message.recalled*`（撤回时 `deleted=true`）、`LastMsg.recalled*`、`Conversation.mentionUnread`、`firstMentionSeq`、`msgDeleted.recalled`、`groupMembers.keyword`（第 51 条）；`frames.json` C4 用例 25 个合法、29 个非法；`errors.json`、`ERRORS.md`（4097、4293、4044 / 4131 加 `ws`、`mentionAllForbidden`、`quoteUnavailable`、429x 例外加 4293、命令文案）；`README.md` 命令表；`ADMIN.md`（删除已撤回的幂等、后台版序列化、元数据的 `recalled*`、`POST /admin/files/{fileId}/fetch` 形状，第 62 条）。服务端用例 TC-CHAT-90-32、TC-RCL-03-07（`logic` 真实解析 + 下行模型解码再编码）通过。三套 SDK 的协议副本随 FE-660 起 |
| BE-471 | ✅ 完成（本任务 AC 都有用例） | `2678e5b`。**本地结果，CI 不运行**。`0018_msg_actions.sql`（`messages.recalled_at/recalled_by/quote_msg_id` + 索引、`conv_seq.last_recalled*`、`conv_mentions`、`group_mention_all`（加 `from_uid`，第 60 条）、`mention_all_quota`（列名用 `win`，第 64 条））；`store.MsgActionStore` 和 `storetest` 契约用例 5 个；`logic.MsgActionsConfig`（第 4.1 节 6 个变量 + 第 53 条的两个频率）；开关关 `recall`、带 `quote` / `mentions` 的 `send` 4030 `featureDisabled`；`features.msgActions`（`http-client.schema.json` 同步）；开关开缺 `IM_QUARANTINE_KEY` 任何环境启动失败（第 54 条），生产缺 `IM_PRIVACY_NOTICE_VERSION` 拒绝启动；配置检查 `msg_actions`；隔离加密器 V13 或 C4 任一开启就初始化；`/admin/test/faults` 的 `quarantineWrite`、`mentionWrite`、`mentionAllCounter`；`im_recalls_total{by}`；定时任务 `mention_purge`。AC-ACC-10-4（TC-ACC-10-04）、AC-RCL-01-10（TC-RCL-01-10 ①②③）、AC-CHAT-90-2 的 C4 部分（TC-CHAT-90-28：默认关闭下现有 e2e 不改代码全过）、AC-CHAT-90-17 的 C4 部分（TC-CHAT-90-34）通过 |
| BE-472 | ✅ 完成（本任务 AC 都有用例） | `2678e5b`。**本地结果**。`recall`：格式 → 频率（每分钟 20，重试计数，第 55 条）→ 会话权限 → 消息存在（其他会话、入群前看不到的都是 4044）→ `tip` 4000（第 56 条）→ 已撤回幂等 → 后台删除 4044 → 权限和时限（4030、4097 `windowSeconds`）；角色每次查库；撤回自己的取更宽时限（管理员同样，第 57 条）；托管群无群主谁都不能撤回他人的；一个事务里清空正文、写 `recalled_*`、隔离加密原文和文件名、文件 `revoked`、摘要、删提及、`mod_ver`；隔离写失败整个回滚；`msgDeleted.recalled` 发给全部参与者（发起的连接只收 ack）。AC-RCL-01-1（服务端部分）、01-2、01-3、01-4、01-5、01-7、01-9、01-11、02-1、02-2、02-3、02-6、02-7、04-2 通过（`e2e/rcl_recall_test.go`）。AC-RCL-02-1 第 5 步按规则 3 的顺序处理（第 63 条） |
| BE-473 | ✅ 完成（本任务 AC 都有用例） | `2678e5b`。**本地结果**。下行版 `toProto`：撤回的 `deleted=true`、`recalled*`、没有 `body`；`lastMsg` 同样；`FileAccessInfo` 把撤回当作删除；撤回的文件 30 天后 `file_purge` 删对象。AC-RCL-01-6（SQLite 检查点后主库和 WAL 搜不到 `ZXQ-RCL-1`、`ZXQ-RCL-3`，隔离表只有密文、解密对得上）、02-4 和 03-3 的服务端部分、03-5、03-7（真实帧过 Schema）、04-6、04-8、AC-FILE-06-3（TC-FILE-06-13）通过（`e2e/rcl_recall_test.go`、`rcl_files_test.go`） |
| BE-474 | ✅ 完成（im-server 一侧，本任务 AC 都有用例） | `2678e5b`。**本地结果**。后台删除已撤回的幂等（不写 `deleted_at`、不发事件、审计照写）；后台版序列化 `logic.ToAdminProto`（`deleted` 只表示后台删除）用在正文和隔离读取；元数据 `recalled*`、撤回的 `bodyLength=0`、没有 `fileSize`；`POST /admin/files/{fileId}/fetch`（`msg.quarantine` + 这个会话有效的隔离授权，按调用方和操作人查，第 66 条；`attachment`、`nosniff`、`no-store`，审计不含文件名；`objstore.Client.Get`）。AC-RCL-04-1、04-3、04-4、04-5、04-7、04-9 的 im-server 部分通过（`e2e/rcl_admin_test.go`、`rcl_files_test.go`）；console 一侧随 CS-204、CS-202 |
| BE-475 | ✅ 完成（本任务 AC 都有用例） | `2678e5b`（代码）、`feeea0b`（用例）。**本地结果，CI 不运行**。第 4.3 节第 7 步：只有 `text` 能带（其他 4000）；同一会话、`seq ≥ minSeq`、没撤回没删除、不是 `tip`，否则 4040 `quoteUnavailable`（六种响应逐字节相同，消息不保存）；去重在前；只存 `quote_msg_id`；`EnrichQuotes` 一页批量组装（文本前 100 个码点、"[图片]"、"[文件] {文件名}"、过期 `expired` 没有文字、原消息不存在按 `deleted`、不展开引用链），`sync`、`msg` 帧和后台正文读取都用。AC-QUO-01-1（服务端部分）、01-2、01-3、01-4、01-6、01-7、02-1、02-4、02-7 通过（`e2e/quo_quote_test.go`）；日志不记引用文字（`e2e/c4_logs_test.go`，BE-480 改名为 `c4_markers_test.go`；TC-CHAT-90-29 的 im-server 部分） |
| BE-476 | ✅ 完成（本任务 AC 都有用例，AC-MEN-02-3 只记录） | `2678e5b`（代码）、`feeea0b`（用例）。**本地结果**。只在群聊、只有 `text`；`userIds` 去重后超过 `IM_MENTION_MAX` 4131；静默过滤非成员，全部被过滤按没有 `mentions` 保存；不解析正文；`all=true` 只有群主、管理员（角色查库），托管群没有群主时谁都不能（4030 `mentionAllForbidden`）；全群禁言不影响；次数在 `SaveMessage` 事务里扣（滚动 60 分钟 + Asia/Shanghai 自然日，4293 `retryAfterSeconds`；计数不可用 5030；撤回不退）；@ 所有人只写一行；`groupMembers.keyword`（`userId` 前缀区分大小写、昵称前缀 ASCII 不区分，每分钟 30 次，开关关也能用）。AC-MEN-01-1、01-2、01-3、01-4、01-11、02-1、02-2、02-6、02-7、02-8、02-9 通过（`e2e/men_mention_test.go`）。**AC-MEN-02-3 是性能用例，开发机只记录、判定等 P-12（BE-480），这一波没有跑** |
| BE-477 | ✅ 完成（本任务 AC 都有用例） | `2678e5b`（代码）、`feeea0b`（用例）。**本地结果**。`conv_mentions` 和消息同一事务（自己不写，写失败整条不保存 5000）；`convs` 一次查询算 `mentionUnread`（@ 个人 + 别人的 @ 所有人，按 seq 去重，`seq > max(已读, 入群)`，只算当前所在的群）和 `firstMentionSeq`，开关开时每个会话都带 `mentionUnread`；撤回、后台删除时删行；`mention_purge` 清理已读之前和已不在群里的行。AC-MEN-01-1、01-4、03-1、03-4、03-8 通过（`e2e/men_mention_test.go`） |
| BE-478 | ✅ 完成（服务端部分） | `a8244f5`。**本地结果，CI 不运行**。`server/e2e/chatmodel` 扩展：`text` 消息、`quote`、`deleted` / `recalled` 单调合并（变真时清除原文、文件名、引用）、`msgDeleted` 清除引用它的原文（撤回 `recalled`、后台删除 `deleted`）、提及计数的节流刷新（5 秒，窗口内的事件在窗口结束时补拉一次，本地为 0 不触发）；`chat-sync.json` 新增 6 个 C4 用例（步骤 `event`、`advance`，期望 `convsFetches`、`mentionUnread`），C2 的 7 个用例输出不变，`format` 说明同步。`TestChatModelRules` 断言关键结论（变异检查：去掉引用清除、允许回写、去掉节流、去掉补拉、为 0 也刷新都会失败）。AC-CHAT-90-15 的 C4 服务端部分、AC-RCL-03-13、AC-QUO-02-6、AC-MEN-03-7 的向量完成；三套 SDK 的执行器随 FE-660、FE-661、FE-663 |
| BE-479 | ✅ 完成（第 3 条随 FE-660～664） | `a8244f5`、`40b9a85`。第 4 条：`docs/reports/privacy-notice-checklist-draft.md` 增加 C4 的 6 个告知项（措辞"不再显示"，只是清单草稿，不代替法律文本）。第 1、2 条（`40b9a85`，2026-11-10 协调方确认按 PRD-chat 第 16 节落实，第 67 条）：PRD-core v1.5→v1.6、PRD-social v1.1→v1.2、PRD-ui v1.3→v1.4、PRD-admin v1.3→v1.4、PRD-ops v1.4→v1.5；**改写 AC-UI-09-1**（同步 TC-UI-09-01）、**AC-ADM-31-6**（幂等扩展到已撤回的，TC-ADM-31-06 改关联说明），AC-UI-09-7 不变；其余是规则说明，AC 总数不变（1598）。第 5 条：`tools/trace.py` 的 `PRDS` 和各任务单、用例的"依据"链接改为新文件名，`traceability.md` 重新生成，gaps 0。口径问题见第 68～71 条。第 3 条随 FE-660～664 |
| BE-480 | 🟡 后端用例完成，等 C3 的 2 条未执行，性能未判定 | 报告 [c4-backend-acceptance-2026-11-10.md](../reports/c4-backend-acceptance-2026-11-10.md)。**本地结果，CI 不运行**。61 条 AC：通过 49、部分通过 8、未执行 4、失败 0。AC-APN-03-3、AC-RCL-03-9 **未执行（等 C3）**（BE-481，PENDING P-43 ②）；AC-MEN-02-3、AC-CHAT-90-1 是性能，开发机只记录（`IM_PERF_DEV=1`：`e2e/men_perf_test.go`、`e2e/c4_perf_test.go`，最差 P99 2.41 ms / 2.72 ms），判定等 P-12。新增特征串检查文件 `e2e/c4_markers_test.go`（原 `c4_logs_test.go` 改名并入，加 `TestTC_CHAT_90_29_MarkersInLogsAndAudit`，`scan-logs` 扫日志和 `admin_audit`）；console-server `e2e/c4_markers_test.go`（tianting-console-server@b11828d，TC-CHAT-90-29 的 console 一侧）；TC-RCL-01-01 补"2 秒内"的真实时间断言。没有改实现，没有新问题。全量 `go test ./...`、C4 相关包 `-race`、console-server 全量 `-race`（临时 Postgres）都通过 |
| BE-481 | ⬜ 未开始 | 依赖 C3（AC-APN-03-3、AC-RCL-03-9），C3 恢复后再做 |
| BE-482 | ✅ 完成（协议和服务端解析） | `05826d5`。**本地结果，CI 不运行**。`frame.schema.json`：`ConvSetReq`（`anyOf` 要求 `pinned`、`notifyOff` 至少一项）→ `{conv}`、`ConvDeleteReq`（`upToSeq` 非负整数）→ `{clearedSeq}`，两者 `convId` 不能是聊天室；`Conversation.pinned`、`pinnedAt`（只在 `pinned=true` 时出现，Schema 用 `if/then/else` 约束）、`notifyOff`；`convs` ack 的 `deleted [{convId, clearedSeq}]`、`deletedTruncated`；事件 `convSettingsChanged`、`convDeleted`；`http-client.schema.json` 的 `features.convSettings`（Go 字段已加，BE-483 接开关之前一直是 false，第 75 条）；`errors.json`、`ERRORS.md`：4220 `PIN_LIMIT_EXCEEDED`（ws，不带 `details`，第 80 条）、`422x` 区段、4040 / 4290 的 `convSet`、`convDelete` 动作和映射、`featureDisabled` 覆盖两个命令；`README.md` 命令表和事件。`frames.json` C5 用例 18 个合法、33 个非法。服务端用例 TC-CHAT-90-38（`logic` 严格解码 + `Validate`，下行模型解码再编码）通过；全量 `go vet ./...`、`go test -count=1 ./...` 通过，trace gaps 0。命令的分发和实现随 BE-483、BE-484、BE-485（现在发 `convSet`、`convDelete` 仍是 4001）。三套 SDK 的协议副本由 SDK 任务用 `scripts/sync-protocol.sh` 同步，执行器随 FE-680～682 |
| BE-483 | ✅ 完成 | `3774fbf`。**本地结果，CI 不运行**。`0019_conv_settings.sql`；`store.ConvSettingsStore`（一次取齐、置顶在同一事务数名额 4220、免打扰、删除 `cleared_seq` 只增不减、当前隐藏的 `deleted` 列表、`ConvClearedSeq`、给 C3 的 `NotifyOffUsers`、总未读聚合 `TotalUnread`），`storetest` 契约用例 6 个；会话查询 `LEFT JOIN conv_settings`。退群、群主移出、后台移除、客户端解散、后台解散在同一事务删行（`account_cleanup` 走 `LeaveGroup`），提交后发布 `convSettings` 失效（C5 没有订阅者）。`IM_CONV_SETTINGS_ENABLED`（默认关）、`IM_CONV_PIN_MAX`、`IM_CONV_SETTINGS_RATE_PER_SEC`、`IM_CONV_DELETED_MAX`；开关关 4030（格式校验之后），`features.convSettings`。用例：e2e `cvs_switch_test.go`（TC-CVS-02-07、TC-CVS-01-05 的清理部分、TC-CHAT-90-35），logic、config 单测；反证 12 处都能抓到 |
| BE-484 | ✅ 完成 | `385e9bd`。**本地结果，CI 不运行**。`convSet` 校验顺序 格式 4000 → 开关 4030 → 频率 4290（和 `convDelete` 共用，查库之前，不计入发送限流）→ `authorize` 的 4032 / 4043 / 4033 → 没有消息的单聊 4040 → 写入（同一事务数名额，4220 不带 `details`；再次置顶 `pinnedAt` 不变）；ack 的 `conv` 和 `convs` 这一项相同（同一个 `toConversation` 加禁言、提及计数）。`convs` 置顶在前（`pinnedAt` 倒序），其余 `updatedAt` 倒序、`convId`，同一条 SQL，500 上限里置顶的一定在。开关关闭后设置照常输出（P-46 ②）。`/admin/test/limits/reset` 能清。总未读聚合 `TotalUnread` 在 BE-483 的 Store 里（AC-APN-04-4 的数据得 5，契约用例）。用例：e2e `cvs_pin_test.go`（TC-CVS-01-01～01-05、TC-CVS-02-01 的 `unread` 部分记**部分通过**、TC-CVS-04-04；和 `convDelete` 混合的部分随 BE-485），排序契约用例，logic 单测；反证 9 处都能抓到（第一轮有 1 处没抓到：两个置顶的 `pinnedAt` 和 `updatedAt` 同向，补了"重新置顶更旧的会话"一步） |
| BE-485 | ✅ 完成 | `52e38f2`。**本地结果，CI 不运行**。`convDelete` 校验顺序 格式 → 开关 → 频率（和 `convSet` 共用）→ 4032 / 4033 / 4043 → 没有消息的单聊幂等成功（`clearedSeq=0`，不写）→ 一个事务（先读 `conv_seq`；`cleared = min(upToSeq, maxSeq)`，不回退；已读推进；清置顶、留免打扰）；日志只有 `convId`、`clearedSeq`。`convs` 在同一条 SQL 里隐藏 `deleted_at > 0 AND max_seq <= cleared_seq`（只设免打扰的空群照常显示，有新消息含 tip 自然重新出现），带 `deleted`、`deletedTruncated`（只列当前隐藏的，按删除时间倒序，`IM_CONV_DELETED_MAX`）。`minSeq = max(原值, clearedSeq + 1)` 四处一起改：`toConversation`、`authorize`（`sync`、`read`、撤回 4044、引用 4040 `quoteUnavailable`）、`files/urls.go` 的 `canAccess`（只对请求者 4048）、提及计数随已读推进。开关关闭后照常生效（P-46 ②）。后台查询不经过 `authorize`，不受影响。用例：e2e `cvs_delete_test.go`（TC-CVS-03-01～03-04、03-06、03-07、03-09、03-11～03-15，TC-CVS-04-04 的混合部分，清除范围内的撤回和引用，开关关闭后已有设置照常生效，`deleted` 截断），隐藏条件契约用例；TC-CVS-03-14 另用 `go test -race -count=3` 跑过；反证 21 处都能抓到（第一轮有 3 处只是编译失败，改成能编译的变异后重跑） |
| BE-486 | ✅ 完成 | `eddee5a`。**本地结果，CI 不运行**。提交后 `convSet` 发 `convSettingsChanged {convId, pinned, pinnedAt?, notifyOff}`、`convDelete` 发 `convDeleted {convId, clearedSeq}` 给同一用户的其他在线连接（`DeliverToUser` 带 `exceptConn`，同 `readUpdated`），操作的连接只拿 ack；幂等的重复操作也发当前状态；删除不另发 `readUpdated`、`convSettingsChanged`（第 84 条）。没有消息的单聊 `convDelete` 不写、不发事件。离线设备靠 `convs` 补齐。事件走 `Router`，跨节点随 B6。用例：e2e `cvs_sync_test.go`（TC-CVS-04-01、04-02、04-05 的服务端部分，事件帧按 `frame.schema.json` 校验；TC-CVS-03-02 的服务端部分在 BE-485 的 `TestTC_CVS_03_01_DeleteC2C`）；反证 6 处都能抓到（第一轮有 2 处是语法错误，改成发给不存在的用户后重跑） |
| BE-487 | ✅ 完成 | `609d7a2`，修正 `ec5f78a`（sdk-ts@4f2f4cc 发现两个 C5 用例 `maxSeq=0` 却加载了消息，违反 format 的“maxSeq 等于已加载的最大 seq”；改为 g1 2、bob 5，生成器加自检，C2、C4 用例不变）。**本地结果，CI 不运行**。`server/e2e/chatmodel` 加 C5（`convs.go`）：会话列表置顶在前（`pinnedAt` 倒序），其余 `updatedAt` 倒序、`convId`；`convs.deleted` 判断（消失的群在 `deleted` 里的移除、不生成 `removed` 快照，不在里面的推断"你已不在该群"，`truncated` 或 `deletedTruncated` 时不推断）；按 `minSeq` 物理清除本地（`convs` 和 `convDeleted` 两条路径，清除的文件进 `filesExpired`，迟到的应答不写回，保留下来的消息里的引用摘要照常显示，第 91 条）；`convSettingsChanged`、`convDeleted` 之后的状态（已读到 `clearedSeq`、不再置顶、仍免打扰，没有更新的消息时移出列表）；总未读。`chat-sync.json` 加 C5 用例 6 个（`expect` 多 `conversations`、`convState`、`removed`、`totalUnread`），原有 C2、C4 的 13 个逐字节不变。`TestChatModelRules` 断言关键结论；反证 15 处都能抓到（第一轮 1 处没抓到：两个置顶的 `pinnedAt` 和 `updatedAt` 同向，改了用例数据；1 处编译失败，换写法重跑）。三套 SDK 的执行器随 FE-680～682（TC-CHAT-90-39） |
| BE-488 | ✅ 完成 | `43911a8`。**本地结果，CI 不运行**。PRD-core **v1.6 → v1.7**：F-CONV-01 规则 4 作废（删除线保留原文），规则 1～3 加注 `pinned*`、`notifyOff`、`deleted`、隐藏条件、置顶排序；F-CONV-02 规则 5 总未读口径；F-GRP-03 规则 4 先查 `deleted`；128 条 AC 不变。PRD-ui **v1.4 → v1.5**：第 126 行、第 11 节、G-UI-07、08 去掉"不做"（"仅本地置顶"仍不做）；F-UI-04 规则 4、5、7、9；F-UI-23 加注，AC-UI-23-1、23-2 只加注、断言不变，TC-UI-23-01、02 同步；6.1 节 `color.surface.pinned`（浅色 `#F3F4F8`、深色 `#23262E`，取值按协调方转来的 tianting-web FE-683），6.2 节 L27～L31、D27～D31（另加 31 号小红点 3:1：浅色 4.40、深色 3.13）；6.6 节 `bell-off`，并按 tianting-web `src/components/Icon.tsx` 补列 C2 起已在用的图标（第 92 条）；279 条 AC 不变。旧版标"已被取代"，`trace.py`、frontend-chat 链接同步。`docs/sdk/README.md` 加 C5 接口和规则；隐私清单草稿加第 19～23 项（不是法律文本）。P-46 ③：`account_cleanup` 第 ② 步在 `RemoveUserSocial` 同一事务删除这个用户的全部 `conv_settings`，对方的不动（e2e `cvs_account_test.go`，反证 2 处都能抓到）。PRD-admin、PRD-social、PRD-ops 不改（第 3 条） |
| BE-489 | 🟡 后端用例完成，等 C3 的 5 条未执行 | 报告 [c5-backend-acceptance-2026-11-11.md](../reports/c5-backend-acceptance-2026-11-11.md)，`5b38668`。**本地结果，CI 不运行**。30 条 AC：通过 19、部分通过 6、未执行 5、失败 0。AC-CVS-02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4 **未执行（等 C3）**，AC-CVS-02-1 只验了 `unread` 那一半记部分通过（BE-490，P-46 按 P-43 ② 不算失败）；端到端 4 条的客户端部分随 FE-686；AC-CHAT-90-2 的 TC-PERF 等 P-12。新增特征串检查文件 `e2e/cvs_markers_test.go`（`TestTC_CVS_03_01_MarkersOnlyAffectSelf`：ZXQ-CVS-1、ZXQ-CVS-2 在 u1 的 `convs`、两个方向的 `sync`、`convDeleted`、文件地址都搜不到，u2、后台、库照常，`scan-logs` 扫日志和 `admin_audit`）。`chat-sync.json` 的 format 说明改为 `readSeq = maxSeq − unread`（frontend-chat 问题 69，用例数据逐字节不变）。没有改实现，没有新问题。全量 `go vet`、`go test ./...`、`go test -race ./...`、console-server 全量 `-race`（临时 Postgres）都通过 |
| BE-490 | ⬜ 未开始 | 依赖 C3（AC-CVS-02-1、02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4），C3 恢复后再做 |
| BE-491 | ✅ 完成（协议和服务端解析） | `002082a`。**本地结果，CI 不运行**。`frame.schema.json`：`ReportReq {targetType, convId?, msgIds?, userId?, groupId?, reason, note?}` → `{reportId}`，按 `targetType` 用 `if/then` 互斥（`message` 要 `convId`（不能是聊天室）和非空 `msgIds`，不能带 `userId`、`groupId`；`user` 要 `userId`、`group` 要 `groupId`，都不能带别的对象字段），`reason` 六个枚举；`msgIds` 20 条上限和 `note` 200 码点只由服务端判（4131、4000）；ack `reportId`；`Error` 的 4510 不能带 `details`。`http-client.schema.json`：`features.moderation`（Go 字段已加，BE-492 接开关之前一直是 false，第 96 条），`ErrorResponse` 的 4510 同样不带 `details`。`errors.json`、`ERRORS.md`：4510 `CONTENT_BLOCKED`（451，`ws`、`client`）、4098 `REPORT_ALREADY_HANDLED`（409，`admin`、`console`）、`451x` 区段；reason `4040 reportTargetInvalid`、`4000 customDisabled`；4040、4131、4292、4030 的 `report` 动作和映射；4510 文案按 P-48 ⑥ 分 `send`「消息包含不当内容，无法发送」和非消息「内容包含不当信息，请修改后重试」；`README.md` 命令表。`ADMIN.md`：4 个 C6 scope（`report.evidence` 是正文类）和词库、命中、举报队列、详情、结案、证据授权、证据读取、`files/fetch` 的 `report.evidence` 路径的请求和响应（第 97、109 条，P-48 ①③④⑤ 已按建议写入：用户举报和单聊消息举报的证据授权按单聊开关，`off` 时 403 `c2cBodyDisabled`；队列里单聊消息举报不带 `convId`、`msgIds`）。`frames.json` C6 用例 13 个合法、27 个非法。服务端用例 TC-CHAT-90-46（`logic` 严格解码 + `Validate`，下行 ack 和错误 ack 解码再编码）和 `protocol/report_test.go`（服务端独有的两个上限）通过；全量 `go vet ./...`、`go test -count=1 ./...` 通过，trace gaps 0。`report` 的分发和实现随 BE-495（现在发 `report` 仍是 4001）。三套 SDK 的协议副本还没同步，由 SDK 任务用 `scripts/sync-protocol.sh` 同步，执行器随 FE-690～692 |
| BE-492 | ✅ 完成 | `6ef4217`。**本地结果，CI 不运行**。`0020_moderation.sql`：`blocked_words`（`norm` 唯一）、`moderation_hits`、`reports`、`report_evidence`，另加 `report_quota`（每人每天的次数，第 124 条）；`store.WordStore`、`ReportStore`（这一步只有计数）。配置 `IM_MOD_ENABLED`、`IM_REPORT_DAILY`、`IM_REPORT_EVIDENCE_DAYS`、`IM_MOD_HITS_DAYS`（第 98 条）、`IM_CLIENT_CUSTOM_MSG`（生产默认 0、其他默认 1，不跟开关走）、`IM_REPORT_KEY`（32 字节 base64；开关开时缺失、长度不对、和 `IM_QUARANTINE_KEY` 或 JWT 密钥相同，任何环境都拒绝启动，错误信息不含密钥；开关关时设置了也校验格式，第 127 条）、`IM_PUBLIC_OPEN_ACK`、`IM_TEST_WORDS_STARTUP_FAIL`（只在测试模式，生产出现拒绝启动，第 123 条）。生产 + 开关开：要 `IM_V13_ADMIN_FEATURES=1`（AC-MOD-01-10）和 `IM_PRIVACY_NOTICE_VERSION`（AC-ACC-10-5）；生产 `open`：`checks.go` 改成逐项检查开关、V13、`IM_PUBLIC_OPEN_ACK`，词库非空在 `app.New` 迁移之后读库（AC-ACC-01-9，TC-ACC-01-19 断言不变）。配置检查 `moderation` 项（开关、`IM_REPORT_KEY` 已设置 / 未设置、生产打开 `custom` 的告警，AC-MOD-02-15）和查库的 `moderation_words`（词库为空告警，只有数量）。`report.evidence` 进 `IsBodyScope`，4 个 scope 进目录（AC-MOD-04-8 的单测和真实二进制用例）。指标 `im_blocked_total{scene}`、`im_mod_reload_errors_total`、`im_reports_pending`（抓取时现算，查询失败是 -1）、`im_report_undecryptable_total`（第 125 条）；`deploy/` 加告警"词库加载失败"和 promtool 用例（本机没有 promtool、PyYAML，`test_deploy.py` 未在本地运行）。故障注入 `wordsReload`、`evidenceWrite`。`report` 已分发：开关关时格式之后 4030 `featureDisabled`（TC-CHAT-90-41、TC-MOD-03-14 后半）；`features.moderation` 跟开关。**BE-491 遗留**：`ReportReq` 记下出现过的字段，`user`、`group` 带空串或 `null` 的 `convId`（以及其他对象字段、`note: null`）服务端也 4000，`frames.json` 加 5 个非法向量。用例：config 单测 6 个、adminauth 单测、e2e `mod_switch_test.go` 10 个（含 3 个真实二进制）；14 处反证（去掉实现后用例失败）。全量 `go vet ./...`、`go test -count=1 ./...` 通过，trace gaps 0 |
| BE-493 | ✅ 完成（语料误判随 BE-499） | `05acee9`。**本地结果，CI 不运行**。新包 `moderation`：`Normalize`（NFKC → NFD 去 `Mn` → 小写 → 去 Default_Ignorable 得到 `norm`，再去空白和标点得到 `compact`；词和文本同一个函数，第 101 条；Default_Ignorable 按 DerivedCoreProperties 的定义用标准库的组成部分算，不加依赖）；按码点的 Aho-Corasick（转移压缩成排序数组二分查找），`norm`、`compact` 各一个，放进不可变快照用 `atomic.Pointer` 整体替换，匹配不加锁，只返回第一个命中的 `word_id`。引擎：启动同步加载，开关开时失败就启动失败（`IM_TEST_WORDS_STARTUP_FAIL` 只在测试模式，另有表读不了的真实用例）；订阅 `blockedWords` 重建；[IM 钟] 超过 60 秒的下一次匹配触发重建（TTL 兜底）；重新加载失败沿用旧快照、`im_mod_reload_errors_total` 加 1、日志不含词，2 秒后自动重试（AC-MOD-01-8"解除注入后 10 秒内"）。Store：`AddWord`、`ImportWords` 按 `norm` 去重（已有的含停用的计入重复、状态不变，第 100 条）、`SetWordState`、`DeleteWord`（不级联命中）、`ListWords`（`norm` 子串、状态、倒序游标）、`ActiveWords`，契约用例 3 个。管理接口 `GET/POST /admin/moderation/words`、`PATCH/DELETE /{wordId}`、`POST .../import`（> 5000 整批 4131；空、> 64 码点、标准化后为空的跳过计入 `invalid`，单个添加 400 / 4000，第 102、128 条），scope `moderation.words`，不跟两个开关注册；审计只记操作、`wordId`、数量。`send` 第 9 步先接 `text`（4510 不带 `details`）。TC-CHAT-90-42 开发机记录：10 万词、5000 码点、10000 次单线程，P50 0.87 ms、P99 1.87 ms、最大 3.94 ms，构建 208 ms、自动机约 9 MiB（darwin/arm64 10 核，**只记录不判定**，P-12）。语料误判（R-CS-17）仓库里没有可用的真实语料，随 BE-499 用特征语料测并写进报告。用例：`moderation` 单测 4 个（含随机对拍、`-race` 并发）、e2e `mod_words_test.go` 9 个（TC-MOD-01-01～04、06、08 两个、TTL 兜底、列表和 scope）；`adminSamples` 加 5 个词库路由（AC-KEY-02-2）；19 处反证。全量 `go vet`、`go test -count=1 ./...` 通过，`moderation`、`admin`、`logic` 和 C6 e2e 加跑 `-race` 通过 |
| BE-494 | ✅ 完成 | `9318334`。**本地结果，CI 不运行**。`send`：第 1 步 `IM_CLIENT_CUSTOM_MSG=0` 时客户端 `custom` 400 / 4000 `customDisabled`（AC-MOD-02-6，只影响客户端 `send`）；第 9 步（引用、@ 所有人之后，保存事务之前）查 `text` 和 `image` / `file` 的**服务端记录的文件名**（第 111 条，第 6 步预检时取 `files.name`），命中 4510（不带 `details`），已扣发送令牌，`onReject` 退还新对象配额（AC-MOD-02-12），@ 所有人次数不扣（AC-MOD-02-16）；被拉黑、被禁言先 4036、4031 且没有命中记录（AC-MOD-02-7）；重发走第 2 步去重（AC-MOD-02-14 前半）。其他场景：注册（邀请码之后、保留名和用户名占用之前，已占用的用户名也是同样的 4510，不写账号、不扣邀请码，第 112 条；`im_registers_total{result="blocked"}`）、改昵称（`PATCH /v1/account/profile`，451 / 4510，昵称不变）、`groupCreate` 群名（开关和人数之后、配额之前）、`groupUpdate` 群名和公告（`UpdateGroupProfile` 加 `check` 回调，在存储层同一事务的群权限判定之后；非群主先 4030）、`friendApply` 附言（限流和配额之后、存在性判断之前，对存在和不存在的对象响应逐字节相同）；管理接口写入的不查（规则 3）。命中记录：`moderation_hits(user_id, scene, word_id, at)`，场景名放在 `moderation` 包（和 `metrics.BlockScenes` 有单测核对），注册时只计指标不写行（第 113 条），日志只有场景和 `wordId`（"content blocked"），写失败只记日志；`GET /admin/moderation/hits?userId=`（scope `report.read`，P-48 ③；写审计，`targets` 是 userId：`report.read` 加进审计的只读 scope，审计目标加 `reportId` 路径参数和 `userId` 查询参数）返回 `{userId, days, counts, recent}`（最近 50 条，没有内容，第 114 条）；`moderation_hits_purge` 每小时，清 `IM_MOD_HITS_DAYS` 之前的行，不跟开关。用例：e2e `mod_send_test.go` 12 个（TC-MOD-02-01～08、10、12～14、16；02-11 在 BE-492 的开关关用例里），`adminSamples` 加命中路由，`moderation` 单测加场景核对；13 处反证（另 1 处变异编译不过，换成存储层的变异）。全量 `go vet`、`go test -count=1 ./...` 通过，C6 发送用例加跑 `-race` 通过。TC-MOD-02-06 第 2 步"管理接口用 `custom` 发系统消息"im-server 没有这个路径，不涉及 |
| BE-495 | ✅ 完成（审核员读取随 BE-496） | `06853d8`。**本地结果，CI 不运行**。`report` 校验顺序：格式（`Validate`）→ 开关 → 每天限额预检（4292，在对象校验之前）→ 对象校验 → 幂等 → 写入。对象校验一律 404 / 4040 `reportTargetInvalid`（消息固定，响应逐字节相同）：`message` 经 `authorize`（参与者、成员、`minSeq`，含 C5 删除会话），消息都在这个会话、不是 `tip`，已撤回或被后台删除的要 `readSeq ≥ seq`；`user` 不能是自己，要存在且是好友（任一方向）、单聊 `conv_seq.max_seq > 0` 或有共同群（第 116 条）；`group` 要是当前成员。4040 也计次数（第 115 条）。写入一个事务：幂等（同一举报人 + `dedup_key` 24 小时内返回第一次的 `reportId`，不计次数，第 117 条的键）→ `report_quota` 条件加 1（并发也不超，第 124 条）→ `reports` 行 → `report_evidence`（每条 AES-256-GCM，`IM_REPORT_KEY`，复用 `quarantine.Cipher`，AAD 是 `reportId|idx`；idx 0 是 `note` 和"按单聊对待"的标记，1.. 是消息快照：元数据一直有，正文只在群消息或单聊 `on` 时有，已撤回、被删除的用隔离原文，隔离原文过期的只有元数据；用户、群举报只有元数据，P-48 ①）→ 证据引用的文件 `evidence_until` 取较大值。故障注入 `evidenceWrite` 整体回滚 5000、不计次数。`file_purge` 的删对象条件加 `evidence_until < now`（普通用户取地址仍只看 `retain_until` 和状态，AC-MOD-03-15）。`report_purge` 每小时：证据过期的删除，`reports` 行在 `IM_REPORT_EVIDENCE_DAYS` 之前的整条删除（P-48 ②），次数表清两天前的；不跟开关。不发任何帧；日志只有 `reportId`、`targetType`、`reason`。`IM_C2C_BODY_ACCESS` 经 `ModConfig.C2CBodyAccess` 在存证据时读一次。用例：契约用例 2 个（写入、清理），e2e `mod_report_test.go` 12 个（TC-MOD-03-01～07、10、12～15 和并发限额）；18 处反证（限额的预检和事务内检查各有一个用例能单独发现，并发用例连跑 3 次都发现）。全量 `go vet`、`go test -count=1 ./...` 通过，举报用例和 `store`、`logic` 加跑 `-race` 通过 |
| BE-496 | ✅ 完成（console 一侧随 CS-206～208） | `4f0dfc4`。**本地结果，CI 不运行**。管理接口不跟开关注册（P-48 ④）：`GET /admin/reports`（`report.read`；按幂等键聚合、`latestAt` 倒序，`targetKey` 是键的哈希，游标是偏移量；单聊消息举报只有 `convType`，群消息带 `convId`、`msgIds`；没有举报人、`note`、正文；`limit` 只收 20 / 50 / 100）、`GET /admin/reports/{id}`（元数据从证据里解出来只取类型、长度、大小，被举报人和保留期内被举报总次数；没有 `note`、正文、文件名；解不开时 `messages` 只有 `msgId`）、`POST .../resolve`（`report.handle`，条件更新 `state='pending'`，第二个 409 / 4098、状态不变；不执行任何处置、不发帧）、`POST /admin/report-grants`（`report.evidence`，要签名的操作人，工单号用 `reportId`，15 分钟，和 `msg-grants` 同一张表所以配额合并，第 132 条；单聊消息举报和用户举报在 `off` 时 403 / 4030 `c2cBodyDisabled`、不产生授权）、`POST .../evidence`（授权要是这个调用方 + 操作人 + 这条举报、没过期，否则 403 / 4039；读的时候再查单聊开关；只含被举报的消息和 `note`；密钥不匹配 `{messages: [], undecryptable: true}`、`im_report_undecryptable_total` 加 1、可以结案；`Cache-Control: no-store`）。`POST /admin/files/{fileId}/fetch` 加 `report.evidence` 路径：`grantId` 是举报授权时要 `report.evidence`、单聊 `off` 403 `c2cBodyDisabled`、授权属于调用方 + 操作人且没过期、这条举报的证据引用了 `fileId` + `msgId` + `convId`（否则 403 / 4039），对象按 `evidence_until` 保留，原消息状态不影响；隔离授权或不带 `grantId` 仍走 `msg.quarantine` 路径；第二批关闭时只有举报路径。`report.read` 进审计的只读 scope；举报接口的审计只记 `reportId`、`targetType`、理由枚举和授权前 8 位。`im_reports_pending` 是超过 24 小时未处理的数。用例：e2e `mod_handle_test.go` 11 个（TC-MOD-04-01、03、04（含 04-02 的 im-server 一侧）、05、07、10/11/16、13、14、17，队列，`im_reports_pending`），`adminSamples` 加 5 个路由；14 处反证（1 处变异是空操作不算，另补了让文件举报详情的变异能被发现的断言）。全量 `go vet`、`go test -count=1 ./...` 通过，`admin` 和举报处理用例加跑 `-race` 通过 |
| BE-497 | ✅ 完成 | `d4cd522`。**本地结果，CI 不运行**。PRD-admin v1.4 → **v1.5**：F-ADM-60 由 PRD-chat F-MOD 取代、改到第一批（C6），替换模式去掉；**AC-ADM-60-1 作废并入 AC-MOD-01-1**（TC-ADM-60-01 改关联），**AC-ADM-60-2 作废**（TC-ADM-60-02 删除）；backend-admin BE-242 标"由 PRD-chat C6 取代"、`- AC：` 行不再列这两条；F-KEY-02 规则 2 加 4 个 scope 和路由、规则 3 正文类加 `report.evidence`、规则 4 `console` 加前三个、`console-msgread` 加 `report.evidence`；5.2 节矩阵 `word.write` 换成 `moderation.words`（运营、审核），新增 `report.read`、`report.handle`、`report.evidence`（审核，超管默认没有）；5.1 节 `moderator` 加词库维护和举报处理；5.3 节加举报证据、单聊举报元数据两行；第 12 节加举报证据一行。AC 280 → 278，其余 AC 含义不变（AC-ADM-05-1 的参数化表随矩阵扩展）。PRD-social v1.2 → **v1.3**（F-SOC-02 附言在存在性判断之前过敏感词，AC 不变）；PRD-ops v1.5 → **v1.6**（F-OPS-06 指标、F-OPS-07 词库加载失败告警、F-OPS-10 `moderation` 检查项，AC 不变）；PRD-ui v1.5 → **v1.6**（F-UI-09 规则 3、4 加"举报"，F-UI-14 规则 10 资料卡"更多"菜单、F-UI-36 群设置加"举报"，都只在 `features.moderation=true` 时；6.6 节加 `flag`，按协调方转来的 FE-693 tianting-web@dd82a82 的写法；不新增颜色 token，AC 不变）。旧版本都标"已被取代"，`trace.py` 的 `PRDS` 和任务单、用例的链接目标同步。PRD-core 没有改（第 136 条）。ARCH-v1 S13、S14 改为内置敏感词和举报（不做业务方回调）。`docs/sdk/README.md` 加"举报和内容拦截"一节；隐私清单草稿加 C6 第 24～29 项（第 118、119 条按 P-48 写入，只是清单草稿，不代替法律文本）。trace：AC 1694，gaps 0 |
| BE-498～BE-499 | ⬜ 未开始 | 2026-11-11 分发（C6，QUEUE Q30）。不依赖 C3，没有"C3 恢复后做"的任务（文末第 94 条）；开发只用特征词，上线门槛见 BE-498 第 4 条 |
| 全部 | 🟡 C1、C2 已校准 | 见 [c1-acceptance-2026-11-07.md](../reports/c1-acceptance-2026-11-07.md) 第 9 节、[c2-calibration-2026-11-10.md](../reports/c2-calibration-2026-11-10.md)：人日不能直接换算成 AI 用时，瓶颈是本机资源和待决定项。C3 已分发（暂停）；C4 2026-11-10 已分发；C5 2026-11-10 已分发；C6 2026-11-11 已分发 |

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| 公共 | BE-400～BE-408 | 14 | 协议、向量、测试设施（假 S3、假通道、账号并发钩子）、测试环境、冻结文档变更 |
| C1 账号 | BE-410～BE-424 | 27.5 | 迁移、`/v1` 基座、闸门和限流、注册、登录、会话续期、改密、恢复码、注销、邀请码、验收、部署和运维手册 |
| C2 文件 | BE-430～BE-440 | 24.5 | 对象存储客户端、上传、`complete`、发送、访问控制、生命周期、头像、部署、合同测试、验收 |
| C3 推送 | BE-450～BE-460 | 19.5 | 协议、迁移和开关、令牌注册、吊销删令牌、在线判断和补推、队列和 worker、内容和隐私、APNs 和 FCM 通道、部署、验收、文档（假通道 BE-406 的 1 人日在公共里） |
| C4 撤回引用 @ | BE-470～BE-481 | 14.5 | 协议、迁移和开关、撤回命令、撤回的下发和隔离、后台接口和文件代理接口、引用、@ 和 @ 所有人、提及计数、共享向量、冻结文档、验收；撤回和推送的衔接（BE-481，等 C3） |
| C5 会话设置 | BE-482～BE-490 | 8 | 协议、迁移和开关、置顶和免打扰、删除会话、多端同步、共享向量、冻结文档、验收；免打扰和推送的衔接（BE-490，等 C3） |
| C6 敏感词和举报 | BE-491～BE-499 | 11 | 协议、迁移和开关、词库、发送检查、举报提交、举报处理接口、冻结文档、部署和上线门槛、验收；没有依赖 C3 的任务 |

**人日对照（PRD-chat 第 18 节）**

| 第 18 节的列 | 公共 | C1 | C2 | C3 | C4 | C5 | C6 | 落在 |
|---|---|---|---|---|---|---|---|---|
| 服务端 | 3 | 18.5 | 14.5 | 13 | 8.5 | 5 | 6.5 | 本单：BE-400～402、405；BE-410～421；BE-430～437；BE-450 0.5、BE-451 1.5、BE-452 2、BE-453 1.5、BE-454 2、BE-455 3、BE-456 1、BE-457 1.5；C4：BE-470 0.5、BE-471 0.5、BE-472 1.5、BE-473 0.5、BE-474 0.5、BE-475 1.5、BE-476 2、BE-477 1、BE-481 0.5；C5：BE-482 0.5、BE-483 1、BE-484 1、BE-485 1.5、BE-486 0.5、BE-490 0.5；C6：BE-491 0.5、BE-492 1、BE-493 1.5、BE-494 1.5、BE-495 1、BE-496 1 |
| console + 后台页面 | 0 | 1.5 | 2.5 | 0.5 | 1.5 | 0.5 | 6 | [console-chat](console-chat.md) 和 [frontend-chat](frontend-chat.md)；C3：CS-203；C4：CS-204 0.5、FE-671 1（CS-202 的 0.5 挪到 C4 做，人日仍记在 C2）；C5：CS-205 0.5（页面没有改动）；C6：CS-206 1、CS-207 1、CS-208 1，FE-696 1、FE-697 2 |
| sdk-ts / sdk-kotlin / sdk-swift | 1 / 1 / 1 | 3.5 / 4 / 4 | 3.5 / 5.5 / 5.5 | 0 / 3 / 3 | 3 / 4.5 / 4.5 | 1.5 / 2.5 / 2.5 | 1 / 1 / 1 | [frontend-chat](frontend-chat.md)；C3：FE-650、FE-651（sdk-ts 只同步协议副本，随 BE-450）；C4：FE-660；FE-661、662；FE-663、664；C5：FE-680；FE-681；FE-682；C6：FE-690；FE-691；FE-692 |
| H5 / Android / iOS | 0 | 5 / 5.5 / 5.5 | 6 / 8 / 8 | 0 / 5 / 5 | 5 / 6 / 6 | 2 / 3 / 3 | 1.5 / 2 / 2 | [frontend-chat](frontend-chat.md)；C3：FE-652、653；FE-654、655；C4：FE-665、666；FE-667、668；FE-669、670；C5：FE-683；FE-684；FE-685；C6：FE-693；FE-694；FE-695 |
| 测试 | 6 | 8.5 | 9.5 | 6 | 8 | 4 | 5 | 公共 6：BE-402～406。C1：BE-422 5 + FE-620 3.5。C2：BE-440 5.5 + FE-640 4。C3：BE-459 3.5 + FE-656 2.5。C4：BE-478 1 + BE-480 3.5 + FE-672 3.5。C5：BE-487 0.5 + BE-489 2 + FE-686 1.5。C6：BE-499 2.5 + FE-698 2.5 |
| 部署 | 1 | 1.5 | 3 | 2 | 0 | 0 | 0.5 | BE-407；BE-423；BE-438 2 + BE-439 1；BE-458 2；C4 没有；C5 没有；C6：BE-498 0.5 |
| 文档 | 4 | 3 | 1.5 | 1 | 1.5 | 0.5 | 1.5 | 公共：BE-400 1、BE-401 0.5、BE-408 2.5。C1：BE-423 1.5、BE-424 1、FE-620 0.5。C2：BE-438 1、BE-439 0.5。C3：BE-450 0.5、BE-460 0.5。C4：BE-470 0.5、BE-479 1。C5：BE-488 0.5。C6：BE-491 0.5、BE-497 1 |
| **合计（三份任务单）** | **17** | **60.5** | **67.5** | **38.5** | **48.5** | **24.5** | **28** | **284.5，和第 18 节一致**（C3 本单 19.5、console-chat 0.5、frontend-chat 18.5；C4 本单 14.5、console-chat 0.5、frontend-chat 33.5；C5 本单 8、console-chat 0.5、frontend-chat 16；C6 本单 11、console-chat 3、frontend-chat 14） |

**差异说明**：合计和第 18 节没有差异，因为这里是按第 18 节的列拆分，不是独立重估，**都是低置信**。下面这些第 18 节没有单独算，也没有另加人日，如果 C1 校准时超出就从这里看：① 第 4.5 节没有列的暂停点和故障注入（BE-404、BE-405）；② `sessions revoke-all` 命令路径（BE-415）；③ 邀请码重新启用和可信出口网段的配置（BE-421、BE-423）。**C3 同样是按列拆分**，第 18 节没有单独算、这里也没有另加人日的：④ 官方文档核对后补的行为（JWT 定时重签、`ExpiredProviderToken` 重签重试、延迟重试队列、410 的 `timestamp` 比较，文末第 34～46 条）；⑤ 假通道按官方错误码补的注入（BE-406）；⑥ 真机等待和 Frank 的账号步骤（第 18 节本来就不含）。**C4 同样是按列拆分**：⑦ BE-481（撤回和推送的衔接，等 C3）的 0.5 从撤回行的服务端 3.5 里拆出，没有另加；⑧ CS-202（后台看文件内容）从 C2 挪到 C4 做，它的 0.5 人日仍算在 C2（第 18 节 C2 行），C4 实际要做的任务合计 49；⑨ 第 18 节没有单独算、这里也没有另加的：后台版和下行版序列化分开（BE-474）、`fetch` 接口（BE-474）、成员搜索的 `keyword`（BE-476）。**C5 同样是按列拆分**：⑩ BE-490（免打扰和推送的衔接，等 C3）的 0.5 从 C5 行服务端 5 里拆出，没有另加；⑪ 第 18 节没有单独算、这里也没有另加的：`files/urls.go` 和 `authorize` 的 `minSeq` 改造（BE-485）、角标聚合函数先做（BE-484）、`convSettings` 失效广播的发布端（BE-483）、后台查询的回归（CS-205）。**C6 同样是按列拆分**：⑫ 没有依赖 C3 的 AC，没有从哪一列拆出任务；⑬ 第 18 节没有单独算、这里也没有另加的：`open` 模式的逐项启动检查（BE-492）、`fetch` 的举报授权路径（BE-496）、证据图片的同源内联（CS-208）、上线门槛清单（BE-498）；AC-MOD-02-13、AC-MOD-04-17 的功能 C1、C2 已有，只补用例。

**依赖顺序**：BE-400、BE-401 → BE-408、BE-402、BE-403 → BE-405、BE-407 → BE-410 → BE-411 → BE-412 → BE-413、BE-414 → BE-415 → BE-416 → BE-417 → BE-418 → BE-419、BE-420、BE-421 → BE-424、BE-423 → BE-422 →（C1 结束，校准）→ BE-404、BE-406 → BE-430 → BE-431 → BE-432 → BE-433 → BE-434 → BE-435 → BE-436 → BE-437 → BE-438、BE-439 → BE-440 →（C2 校准，Frank 决定 C3）→ BE-450 → BE-406、BE-451、BE-460 → BE-452、BE-454 → BE-453、BE-455 → BE-456、BE-457 → BE-458 → CS-203 → BE-459。**C3 暂停后先做 C4**：BE-470 → BE-471、BE-478、BE-479 → BE-472、BE-475、BE-476（可以并行）→ BE-473、BE-477 → BE-474 → CS-204、CS-202 → BE-480；BE-481 等 C3 恢复后的 BE-455、BE-456。**C5**：BE-482 → BE-483、BE-487、BE-488 → BE-484 → BE-485 → BE-486 → CS-205 → BE-489；BE-490 等 C3 恢复后的 BE-406、BE-455、BE-456。**C6**：BE-491 → BE-492、BE-497 → BE-493、BE-495（可以并行）→ BE-494、BE-496 → BE-498、CS-206～CS-208 → BE-499；没有等 C3 的任务。console 和前端的依赖写在各自任务里。

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
- 优先级：P0（2026-11-10 起：Frank 决定先做 C3；人日仍算在公共部分，第 18 节这样算）
- 批次：C3（原写 C2，C2 期间没有做）
- 仓库：tianting-server（`server/internal/push/pushtest`，写法同 `objstoretest`：在测试进程里、监听独立端口）
- 依赖：BE-405、BE-450
- AC：AC-APN-05-4、AC-APN-02-5
- 内容：
  1. 第 4.5 节：HTTP 层假通道，`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT` 只在测试模式生效（生产拒绝启动已在 BE-405 做了）；校验 ES256 JWT、必需请求头、OAuth；可注入 410、`UNREGISTERED`、429、503、超时；保存完整请求（含请求头）供断言。
  2. **按官方文档补的校验**（文末第 34～46 条）：假 APNs 要求 HTTP/2、`:path=/3/device/<token>`、`apns-topic`、`apns-push-type`；JWT 头 `alg=ES256`、10 字符 `kid`，声明 `iss`（10 字符 Team ID）、`iat`；`iat` 超过 1 小时返回 403 `ExpiredProviderToken`，同一连接 20 分钟内换 JWT 返回 429 `TooManyProviderTokenUpdates`；`apns-collapse-id` 超过 64 字节返回 400 `BadCollapseId`；sandbox 令牌打到 production 端点返回 400 `BadDeviceToken`；410 响应体带 `reason`、`timestamp`。假 FCM 校验 `Authorization: Bearer`、路径里的项目 ID，带 `collapse_key` 的请求照常接受并记录（AC-APN-03-6 靠记录断言没有它）；可注入 `UNREGISTERED`（404）、`INVALID_ARGUMENT`（400）、`SENDER_ID_MISMATCH`（403）、`QUOTA_EXCEEDED`（429 带 `Retry-After`）、`UNAVAILABLE`（503）、超时。假 OAuth token 端点签发短期 access token。
  3. 测试接口：读取已记录请求（按令牌、按时间）、清空、设定下一次或持续的响应、阻塞（AC-APN-02-10）、"暂停在调用前"（AC-APN-01-9）。
  4. `IM_TEST_DISABLE_IDLE=1`，只关服务端 4408（AC-APN-02-5）。
  5. 测试用的 `.p8`、服务账号 JSON 由测试在临时目录现场生成，**仓库里没有任何密钥文件**（仓库公开）。
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
  2. **改写 AC-UI-15-3**（"我"页没有编辑入口 → 账号会话有，演示会话没有）：改 TC-UI-15-03；**改写 AC-UI-16-6**（设置页增加"账号安全"，演示会话不显示；"通知"这一项随 C3，见 BE-460）：改 TC-UI-16-06。
  3. **改写 AC-AUTH-02-4**：账号模式下 4011 先刷新一次（F-ACC-03 规则 7），演示模式不变。改 TC-AUTH-02-07、02-09（只在演示模式下断言），TC-AUTH-02-08 不变。
  4. **AC-UI-90-13**：断言不变；NF-UI-14 的 `connect-src` 加具体存储域名，`img-src` 加 `blob:`（C2）；改 TC-UI-90-14 的前置配置。
  5. **AC-KEY-02-2**：参数化表增加 `account.invite`；改 TC-KEY-02-02。`report.*`、`moderation.words` 随 C6。
  6. **AC-ADM-30-7**：元数据响应没有正文，扩展到文件名。C2 就有文件消息，所以这条随 C2 生效，见 CS-201 和 TC-FILE-91-01。
  7. **不变、只核对**：AC-CONV-01-1（`modVer` 是可选字段，断言不变）；AC-CON-10-3（后台 CSP 不放宽）；TC-USER-01-16（由 AC-ACC-01-5 补上自动化）。
  8. **第 16.1 节 C1 的部分**：PRD-core F-AUTH-01 规则 1、F-AUTH-02、F-AUTH-03、F-AUTH-04、F-AUTH-05、F-USER-01 规则 4、F-USER-02、第 1、2 节"业务方"表述；PRD-social 1.3、11.2 增加注册路径，F-SOC-09、F-SOC-10 的说明；PRD-ui F-UI-03、F-UI-15、F-UI-16、第 126 行、第 11 节和 G-UI-05、3.4 节 token 存储、D-UI-10，`tianting-web/deploy/nginx.conf`、`vite.config.ts` 转发 `/v1/`（代码随 FE-613）；PRD-admin F-KEY-02 规则 2、4、5 和第 5 节矩阵（`account.invite`、业务方典型配置去掉 `token.issue`）；PRD-ops F-OPS-06、07、10 的账号部分；ARCH S4（`open` 和 `token.issue` 的启动保护）。`docs/sdk/README.md` 由 FE-620 更新。
  9. **第 16.1 节 C2 的部分**：PRD-core 第 704 行、F-MSG-01 规则 6、F-MSG-05 规则 7、F-MSG-06 规则 6、3.1 节消息类型、F-CONV-01 的 `modVer`；PRD-ui G-UI-06、G-UI-11、F-UI-04 规则 5 的摘要、NF-UI-14；PRD-admin F-ADM-31 规则 6、7 和 F-ADM-32 里**文件名进隔离区和文件处理**的部分；PRD-ops F-OPS-06、07、10 的文件部分、F-OPS-12；ARCH 3.4 节、第 2 节图（对象存储）。
  10. **留到 C3～C6，这个任务不动**（C3 的部分见 BE-460）：AC-UI-09-1、AC-UI-23-1、23-2、AC-GRP-03-3、AC-ADM-31-6、AC-ADM-60-1（TC-ADM-60-01 改关联）、AC-ADM-60-2（TC-ADM-60-02 删除）；AC-UI-09-7、AC-SOC-02-5 不变。第 16.1 节里 PRD-core 第 705、709、512 行，F-CONV-01 其余、F-CONV-02 规则 5，F-GRP-03 规则 4；PRD-social F-SOC-02、`groupMembers`；PRD-ui G-UI-07、08、10、13，第 245 行，F-UI-04 规则 9，F-UI-09，F-UI-23，F-UI-42；PRD-admin F-ADM-60、F-KEY-02 的 `report.evidence`、第 12 节；PRD-ops 的推送、撤回、治理部分；PRD-live F-ROOM-11；ARCH S13、S14、G1、G2、推送通道和 `Presence`。
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
  7. `session_purge` 每天清除过期 30 天以上的会话（规则 13）。**绑定令牌的删除随 C3**（`device_push` 还没有），见 BE-453。
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
  6. 无 `sid` 的 token（规则 11）：`auth` 和 `/v1/*` 拒绝 `iat` 早于 `credentials_changed_at` 的；调账号接口 403 / 4030。`delivery.Presence` 和 `Session` 的前后台字段随 C3，见 BE-454。
- 工作量：1.5 人日（服务端）

### BE-417 退出、改密、退出其他设备、凭证版本
- 优先级：P0
- 批次：C1
- 仓库：tianting-server
- 依赖：BE-416
- AC：AC-ACC-04-1、AC-ACC-04-2、AC-ACC-04-3、AC-ACC-04-4、AC-ACC-04-7、AC-ACC-04-8、AC-ACC-04-10、AC-ACC-04-11、AC-ACC-04-12、AC-ACC-04-14、AC-ACC-04-15、AC-ACC-04-16、AC-ACC-04-18、AC-ACC-04-19、AC-ACC-04-25、AC-ACC-04-29
- 内容：
  1. `logout`（规则 1）：Bearer 或 refresh 有一个有效就执行，幂等；`mac` 必须正确；任何一代的值都按退出处理，不算重放。
  2. `password`（规则 2、3）：吊销全部会话，给当前设备新 `sid`，响应带新 token 和新恢复码；写 `credentials_changed_at`；旧码按 F-ACC-05 规则 5。写库失败整体回滚。**"显示内容"设置的复制和令牌重新注册随 C3**，见 BE-453 第 3 条和 FE-650～655。
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

## C3 离线推送

> 2026-11-10 分发（Frank 决定按顺序先做 C3，CH-27，QUEUE Q26）。依据 PRD 第 7 节 F-APN-01～06、第 4.1、4.5、11、12.1、13～15 节的 C3 部分、第 18 节 C3 行、第 20.3 节；官方文档核对见文末第 34～46 条（PENDING P-40）。
> **开发和后端验收都用假 APNs、假 FCM（BE-406）**，不需要 Frank 的账号。真实通道只有 AC-APN-05-3（手工）要用，挡在第 20.3 节和 P-13，见 FE-656。
> 标了 C4、C5 的 AC-APN-03-3（撤回替换通知）、AC-APN-04-4（免打扰角标口径）不在本批，代码只留调用点。F-MEN-03 的免打扰过滤随 C4、C5。

### BE-450 协议变更（C3 部分）：设备通知命令、presence
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`protocol/`、`server/internal/protocol`）
- 依赖：BE-401
- AC：AC-CHAT-90-10、AC-APN-01-13、AC-APN-02-17
- 内容：
  1. `frame.schema.json`（第 11.1 节）：上行 `deviceNotificationRegister {provider:"apns"|"fcm", token, env?:"sandbox"|"production", appId}` → `{}`；`deviceNotificationUnregister {}` → `{}`；`deviceNotificationSettings {showDetail?}` → `{showDetail}`；`presence {state:"foreground"|"background"}` → `{}`（普通带 `id` 的命令，不改 gateway 的特判，R-CB-26）。令牌长度上限写进 Schema（PRD 只说"超长"，取 APNs 十六进制令牌和 FCM 令牌都放得下的 4096 字节，按 4000 拒绝）。
  2. `errors.json`、`ERRORS.md`：`4290` 用于注册频率（每账号每小时 10 次）；`4030 featureDisabled` 覆盖新命令；`4000` 令牌为空或超长。不新增错误码。
  3. `frames.json` 增加这 4 个命令的合法和非法用例（`provider` 取值、`env` 缺省、空令牌、超长令牌、`state` 取值）。三套 SDK 同步 `protocol/` 副本：sdk-ts 只同步副本和跑 Schema 校验（第 18 节 sdk-ts 列为 0，不加接口）；sdk-kotlin、sdk-swift 的执行器随 FE-650、FE-651。
  4. 名字避开现有系统通知的 `push`、`pushSync`（R-CF-26）。
- 工作量：1 人日（服务端 0.5 + 文档 0.5）

### BE-451 迁移、存储、配置和开关
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`store`、`config`）
- 依赖：BE-450
- AC：AC-APN-02-17、AC-APN-01-4、AC-CHAT-90-2
- 内容：
  1. `0018_device_push.sql`（第 12.1 节）：`device_push`（`token_hash` 唯一、`token` 字段级加密、`provider`、`env`、`user_id`、`sid` 可空、`show_detail` 只用于无 `sid`、`registered_at`）；`device_sessions` 增加 `show_detail`。`store.PushStore` 和契约用例（storetest），Postgres 随 B6。
  2. 令牌加密复用现有字段级加密的做法，密钥缺失时生产拒绝启动；日志和审计只记 provider 和令牌 SHA-256 前 8 位（规则 6）。
  3. 第 4.1 节配置：`IM_OFFLINE_PUSH_ENABLED`（默认关，和 `IM_PUSH_ENABLED` 无关）、`IM_PUSH_LIVE_WINDOW`、`IM_PUSH_GRACE`、`IM_PUSH_GRACE_DELAY`、`IM_PUSH_QUEUE_MAX`、`IM_PUSH_MERGE_SECONDS`；F-APN-05 规则 1、2 的通道配置。开关关：新命令 403 / 4030 `featureDisabled`，不入队；`GET /v1/config` 的 `features` 增加 `offlinePush`。
  4. AC-CHAT-90-2 的 C3 部分：推送开关关时现有 e2e 不改代码全过（第 15 节累计表）。
- 工作量：1.5 人日（服务端）

### BE-452 令牌注册、注销、通知设置
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`logic`、`gateway`）
- 依赖：BE-451
- AC：AC-APN-01-1、AC-APN-01-4、AC-APN-01-7、AC-APN-01-8、AC-APN-01-13、AC-APN-06-3
- 内容：
  1. F-APN-01 规则 1：令牌绑定当前 `sid`；**没有 `sid` 的连接（演示、管理接口 token）只绑定 `userId`**，按"该用户没有任何活跃连接"判断是否推送（R-CQ-10）。频率每账号每小时 10 次（4290），空或超长 4000。
  2. 规则 2：同一令牌只属于一个用户。另一个账号注册同一令牌时把令牌从旧账号移走，**"显示发送者和内容"重置为关**（R-CS-03）；令牌刷新（同一 `sid` 换令牌）覆盖旧值，不影响设置（R-CB-23）。
  3. F-APN-03 规则 2：`deviceNotificationSettings {showDetail}` 存 `device_sessions.show_detail`（无 `sid` 时存令牌行），没有令牌也能设，ack 返回当前值（R-CF-22）。
  4. AC-APN-06-3：没有令牌的用户不报错、不调用通道，消息照常进 `sync`。
  5. 规则 6 和 AC-CHAT-90-5 的 C3 部分：令牌、`showDetail` 变更的日志和审计只有 provider 和哈希前缀。
- 工作量：2 人日（服务端）

### BE-453 sid 吊销同事务删令牌（不变量）
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`account`、`sessions`、`logic`）
- 依赖：BE-452
- AC：AC-APN-01-3、AC-APN-01-9、AC-ACC-03-19、AC-ACC-03-27
- 内容：
  1. F-APN-01 规则 4：**`sid` 吊销或过期，同一事务删除绑定的令牌**。覆盖退出、改密、退出其他设备、找回、refresh 重放、`session_purge`（BE-415 第 7 条的遗留）、强制下线、封禁、注销、`sessions revoke-all`。加锁顺序沿用第 4.3 节"账号事务"。
  2. **发送前按当前 `sid` 状态复查**：worker 调通道前再查一次，已吊销的不发（R-CB-23，AC-APN-01-9 用 `/admin/test/faults` 的暂停点在入队后、调用前吊销）。
  3. 改密（BE-417 第 2 条的遗留）：当前设备换新 `sid` 时**复制 `show_detail`**，旧令牌随旧 `sid` 删除，客户端用新 `sid` 重新注册（AC-ACC-04-26 的服务端部分；客户端见 FE-650～655）。
  4. C1 AC 里"C3 上线时验"的部分：AC-ACC-03-19（refresh 响应丢失重试后推送令牌还在）、AC-ACC-03-27（`session_purge` 同时清除令牌），见 TC-APN-91-01、91-02。
- 工作量：1.5 人日（服务端）

### BE-454 在线判断：Presence、前后台、断线补推
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`delivery`、`hub`、`gateway`）
- 依赖：BE-451
- AC：AC-APN-02-2、AC-APN-02-3、AC-APN-02-5、AC-APN-02-6、AC-APN-02-13、AC-APN-02-14
- 内容：
  1. 第 11.4 节：**新增 `delivery.Presence` 接口**（hub 本地实现，不扩 Router，R-CB-16）；`delivery.Session` 增加最近收帧时间、前后台状态（BE-416 第 6 条的遗留）。`logic` 不依赖 `hub`（ARCH 2.2 节）。
  2. F-APN-02 规则 2、3：按**设备**判断；活跃 = `IM_PUSH_LIVE_WINDOW`（45 秒）内收到过帧且前台。`platform` 是 android、ios 的连接在收到 `presence` 之前按后台（R-CF-06）；`web`、`pc` 默认前台。
  3. 规则 4 断线补推（R-CB-11）：只对关闭码 1006、4408；延迟 `IM_PUSH_GRACE_DELAY`（30 秒），期间同一设备重连就取消；该设备没有其他连接、`IM_PUSH_GRACE`（120 秒）内下发过消息且仍有未读 → 一条聚合通知"你有新消息"。全局每秒 100 条。滚动重启的 1001 不触发。
  4. `IM_TEST_DISABLE_IDLE=1`（BE-406）让 AC-APN-02-5 能构造"46 秒没有帧但连接还在"。
  5. 第 15 节"集群"：第一批只支持单节点，节点本地状态在 B6 重写，接口留好。
- 工作量：2 人日（服务端）

### BE-455 推送队列和 worker：入队、合并、批量查询、重试
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（新包 `push`、`logic`）
- 依赖：BE-452、BE-454
- AC：AC-APN-02-1、AC-APN-02-4、AC-APN-02-7、AC-APN-02-8、AC-APN-02-9、AC-APN-02-10、AC-APN-02-12、AC-APN-02-16、AC-APN-04-5、AC-APN-01-9、AC-APN-06-3、AC-CHAT-90-13
- 内容：
  1. F-APN-02 规则 1：只推 `text`、`image`、`file`；不推 `tip`、`custom`、自己发的（含发送者自己的其他设备）、撤回和删除事件、系统通知 `push` 帧、好友申请。
  2. 规则 7（R-CB-12）：**`send` 的 ack 之后异步入队**，队列单位是消息（`IM_PUSH_QUEUE_MAX`），满了丢弃并计 `im_offline_push_dropped_total`。worker 批量查询：一条消息一次取齐全部收件人的令牌、拉黑（免打扰随 C5），每条消息数据库查询 ≤ 5 次。SQLite 只有一条连接，worker 的查询要短。
  3. 角标：聚合查询 + 60 秒缓存；离线设备超过 200 的群消息不带 `badge`（AC-APN-04-5）。口径按 F-CVS-02 规则 4 的部分随 C5（AC-APN-04-4），本批是"总未读"。
  4. 规则 6：同一设备同一会话 `IM_PUSH_MERGE_SECONDS`（5 秒）内最多 1 条；collapse id 的算法见 BE-456。
  5. 规则 8 重试：429、5xx 首次调用外最多重试 2 次；**重试间隔按官方文档（文末第 36 条，等 Frank 定）**：实现成可配置的延迟队列，不占 worker，测试用 [IM 钟] 推进；通知过期时间隐藏 24 小时、详情 1 小时，超过过期时间不再重试。重试用尽丢弃并计数，不积压。
  6. 指标 `im_offline_push_total{provider,result}`、`im_offline_push_queue_len`；`result` 是有界取值（ok、retry_exhausted、token_deleted、auth_error、bad_request、timeout、dropped），不带 userId 和令牌。
  7. 第 15 节性能：入队到调用通道 P99 ≤ 2 秒（AC-CHAT-90-13）；通道超时、2000 人群时 ack P99 ≤ 200 ms（AC-APN-02-8、02-12）。开发机只记录，判定等 P-12。
- 工作量：3 人日（服务端）

### BE-456 通知内容、隐私和 collapse id
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`push`）
- 依赖：BE-455
- AC：AC-APN-03-1、AC-APN-03-2、AC-APN-03-6、AC-APN-03-7、AC-APN-01-8
- 内容：
  1. F-APN-03 规则 1（CH-11）：默认标题"天听"、正文"你收到一条新消息"；**整个请求（含请求头）不含 `convId`、`userId`、`msgId` 及它们的普通哈希**；角标数字保留。
  2. 规则 3：打开详情时标题是昵称（**不用备注**）或群名，昵称为空用 `userId`；正文摘要最多 50 个码点，先去控制字符；图片"[图片]"，文件"[文件]"不带文件名。只有开了详情的那台设备带内容（AC-APN-03-7）。
  3. F-APN-02 规则 6：**隐藏模式每设备一个固定的不透明 id**（所有会话折叠成一条）；**详情模式 id = `HMAC(服务端密钥, sid + convId)` 截断**。APNs 放 `apns-collapse-id`（**上限 64 字节**，文末第 42 条）；FCM **不用 `collapse_key`**，放 `android.notification.tag`（官方已核实，文末第 40 条）。
  4. F-APN-04 规则 1：详情模式数据字段带 `convId`（deep link 用），隐藏模式没有。iOS `badge`；Android 渠道"消息"（`channel_id`）。
  5. F-APN-03 规则 4（撤回、删除后的替换通知）属于 C4（AC-APN-03-3）：只留"这台设备这个会话最后推送的一条"的内存 LRU 接口，本批不发。
- 工作量：1 人日（服务端）

### BE-457 通道：push.Provider、APNs、FCM 客户端和启动检查
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`push/apns`、`push/fcm`、`config`、`opsapi`）
- 依赖：BE-406、BE-455
- AC：AC-APN-01-2、AC-APN-05-1、AC-APN-05-2、AC-APN-05-4、AC-APN-05-5
- 内容：
  1. F-APN-05 规则 5：`push.Provider` 接口隔离，测试用假通道，以后可加厂商通道。**不新增依赖**：HTTP/2 用标准库，ES256 用标准库 `crypto/ecdsa`，OAuth 的服务账号 JWT 也用标准库签（RS256）。
  2. APNs（规则 1）：`.p8` token 认证，`IM_APNS_KEY_FILE`、`IM_APNS_KEY_ID`、`IM_APNS_TEAM_ID`、`IM_APNS_TOPIC`（`com.tianting.im`）；按令牌的 `env` 选 `api.sandbox.push.apple.com` 或 `api.push.apple.com`（443）；请求头 `authorization: bearer <JWT>`、`apns-topic`、`apns-push-type: alert`、`apns-priority`、`apns-expiration`（epoch 秒）、`apns-collapse-id`；**JWT 缓存，20～60 分钟之间重签**（文末第 38 条）。
  3. FCM（规则 2）：HTTP v1 `POST https://fcm.googleapis.com/v1/projects/{project_id}/messages:send`，`IM_FCM_CREDENTIALS_FILE`（服务账号）、项目 ID；OAuth scope 用 `https://www.googleapis.com/auth/firebase.messaging`（最小权限，文末第 44 条），token 端点取凭证文件的 `token_uri`；`android.ttl`（如 `"3600s"`）、`android.priority: HIGH`、`android.notification.tag`、`channel_id`；不带 `collapse_key`。
  4. 错误映射（F-APN-01 规则 3、F-APN-05 异常，**按文末第 34、35、37、39 条的建议实现，Frank 定后再调**）：APNs 410（`Unregistered`、`ExpiredToken`）和 400 `BadDeviceToken` → 删除令牌；FCM 404 `UNREGISTERED` → 删除；APNs 403 `ExpiredProviderToken` → 重签 JWT 后重试 1 次；其余 401 / 403 → 计凭证失败（F-OPS-07 指标），不影响消息；400 其他 → 不重试。
  5. 规则 3：密钥文件权限必须 0600，不进仓库；`IM_ENV=production` 开关打开但缺密钥或权限不对 → 非 0 退出，错误信息不含密钥内容（AC-APN-05-1）；非生产告警并关闭该通道（AC-APN-05-5）。
  6. 配置检查 `GET /internal/v1/config`（第 14.2 节，AC-APN-05-2）：通道是否启用、key id、文件存在、权限正确、最近一次调用结果，不含密钥。
  7. 第 15 节可用性：APNs、FCM 不可用不影响文本收发，不是 `/readyz` 的条件。
- 工作量：1.5 人日（服务端）

### BE-458 C3 部署：推送运维手册和网络
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`deploy/`）
- 依赖：BE-457
- AC：AC-APN-05-1、AC-APN-05-2、AC-CHAT-90-17
- 内容：
  1. `deploy/PUSH-RUNBOOK.md`：第 20.3 节 Frank 的步骤（APNs Key、Firebase 项目、服务账号）对应的服务器配置；密钥文件 0600、不进仓库、不发到聊天里；Key 轮换 = 改配置文件后重启；配置检查和指标；告警（推送失败率、队列满，第一批不发页面告警）。
  2. **网络**（F-APN-05 规则 4，CH-13）：出站白名单 `api.push.apple.com`、`api.sandbox.push.apple.com`（443，可选 2197）、`fcm.googleapis.com`、凭证文件里的 OAuth `token_uri` 的主机；中国大陆部署到谷歌不通时的出境代理（未核实，写成"部署前 Frank 确认"）。
  3. `deploy/push/im-push.example.env`：变量名和占位符，**没有任何真实值**（仓库公开）；`test_deploy.py` 加静态检查（占位符、没有私钥头、权限说明）。
  4. AC-CHAT-90-17 的 C3 部分：推送指标和配置检查项出现，标签没有无界字段。
- 工作量：2 人日（部署）

### BE-459 C3 后端验收
- 优先级：P0
- 批次：C3
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-453、BE-456、BE-457、BE-458、CS-203
- AC：AC-CHAT-90-1、AC-CHAT-90-2、AC-CHAT-90-3、AC-CHAT-90-5、AC-CHAT-90-13、AC-CHAT-90-17、AC-APN-01-3、AC-APN-02-8、AC-APN-02-12
- 内容：
  1. 按 [TC-APN](../testcases/TC-APN.md) 落地后端 e2e，`push_*_test.go`，用例名带 TC 编号，全部用假 APNs、假 FCM 和账号登录的 token（有 `sid`，第 4.5 节）。
  2. 隐私序列单独一个文件：AC-APN-03-1、03-6 抓完整请求（头和体）搜特征串；AC-APN-01-3 的 9 条吊销路径一张表（含 `sessions revoke-all`、注销）。
  3. 【全部】的 C3 部分：AC-CHAT-90-2（推送开关关）、90-3（APNs、FCM 拒绝连接和黑洞）、90-5（推送流程的日志和审计）、90-17（推送指标），见 TC-CHAT-90-19～25。
  4. 性能（AC-APN-02-8、02-12，AC-CHAT-90-1、90-3、90-13）开发机只记录，不算通过，等 P-12。
  5. 写验收报告 `docs/reports/c3-backend-acceptance-<日期>.md`：哪些是假通道结果、哪些等真实通道（AC-APN-05-3，FE-656）。
- 工作量：3.5 人日（测试）

### BE-460 冻结文档变更（C3 涉及的部分）
- 优先级：P1
- 批次：C3
- 仓库：tianting-server（`docs/`）
- 依赖：BE-450
- AC：无（文档任务）
- 内容（第 16.1、16.2 节 C3 的部分，走变更流程）：
  1. PRD-core 第 709 行"离线推送随原生端" → 移到 PRD-chat C3；PRD-ui 第 245 行 iOS 角标进第一批；AC-UI-16-6 设置页的"通知"一项（BE-408 第 2 条留到 C3 的部分，**H5 没有离线通知，不显示"通知"**，第 13.2 节）；PRD-ops F-OPS-06、07、10 的推送部分；ARCH 的推送通道依赖和 `Presence`。
  2. 文末第 34～46 条 Frank 定了之后，需要改 PRD 的写进下一个小版本（不在本任务里先改）。
  3. `docs/sdk/README.md` 的设备通知接口（随 FE-650、FE-651）。
- 工作量：0.5 人日（文档）

## C4 撤回、引用、@

> 2026-11-10 分发（Frank 2026-11-10"推送后续做，先做功能"，CH-27，QUEUE Q28；C3 暂停）。依据 PRD 第 8 节 F-RCL-01～04、F-QUO-01～02、F-MEN-01～03，第 4.1、4.3、4.5、11、12.1、13～15 节的 C4 部分，第 18 节 C4 三行（撤回 18、引用 12、@ 18.5），第 17 节 C4 行。
> **C4 不依赖 C3 的代码**。依赖推送的两条 AC（AC-APN-03-3 撤回替换通知、AC-RCL-03-9 隐藏模式撤回不调用通道）单独放在 BE-481，**C3 恢复、BE-455、BE-456 完成后再做**，C4 其余任务不等它。F-MEN-03 规则 4、5 的免打扰推送口径（AC-MEN-03-2、03-3）标的是【C5】，不在本批。
> **生产打开 `IM_MSG_ACTIONS_ENABLED` 之前**要完成 PRD-admin 第 12 节 ②③④（CH-17，第 17 节，owner Frank），开发和测试不受影响；见 PENDING P-43 ①。
> 迁移号：C3 的 `0018_device_push.sql` 没有提交（C3 暂停），C4 用 `0018_msg_actions.sql`，C3 恢复时 BE-451 改用下一个空号（文末第 52 条）。

### BE-470 协议变更（C4 部分）：recall、quote、mentions、msgDeleted.recalled
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`protocol/`、`server/internal/protocol`）
- 依赖：BE-401
- AC：AC-CHAT-90-10、AC-RCL-03-7
- 内容：
  1. `frame.schema.json`（第 11.1 节）：上行 `recall {convId, msgId}` → `{}`（普通带 `id` 的命令）；`MsgBody` 的 `text` 分支增加 `quote {msgId}`（上行）/ `{msgId, seq, from, type, state, text?}`（下行，`state` 取 `normal`、`recalled`、`deleted`、`expired`，后三种没有 `text`）和 `mentions {userIds, all?}`（BE-401 第 3 条留的结构）；`image`、`file`、`custom` 分支不能带这两个（4000）。`Message` 增加 `recalled`、`recalledAt`、`recalledBy`（撤回的同时 `deleted=true`，"`deleted` 时没有 `body`"的条件不变）；`LastMsg` 增加 `recalled`、`recalledBy`；`Conversation` 增加 `mentionUnread`、`firstMentionSeq`；事件 `msgDeleted` 增加可选 `recalled {by, at}`。
  2. **`groupMembers` 的 `keyword`**：第 11.1 节写的 `GroupMembersReq` 在现有协议里是加成员的请求（`protocol/social.go` 第 298 行），分页查成员的是 `GroupMembersPageReq`（`logic/group.go` 第 500 行），`keyword` 加在后者（文末第 51 条）。
  3. `errors.json`、`ERRORS.md`（第 11.5 节）：新增 4097（`details.windowSeconds`）、4293（`details.retryAfterSeconds`，429x"不自动重试"的例外加 4293）；`4044` 的 scope 增加 `ws`（现在只有 admin、console）；`4131` 增加 `ws`（@ 超限）；`4040` 的 `quoteUnavailable`；`4030` 的 `mentionAllForbidden`；`4290` 用于撤回频率和成员搜索频率。
  4. `ADMIN.md`：`POST /admin/messages/delete` 对已撤回的幂等（F-RCL-04 规则 1）；`/admin/convs/messages/query` 和 `body` 的 `recalled`、`recalledAt`、`recalledBy`（后台版序列化，`deleted` 只表示后台删除，RC2-21）；**新增 `POST /admin/files/{fileId}/fetch`**（第 11.6 节；请求 `{convId, msgId, variant?:"original"|"thumb"}`，响应是文件字节，`Content-Type` 按服务端记录、`Content-Disposition: attachment`、`X-Content-Type-Options: nosniff`；scope `msg.quarantine`（C4）或 `report.evidence`（C6）；没有授权 403 / 4039，普通调用方 403 / 4030，单聊受 `IM_C2C_BODY_ACCESS`；console 问题 13 要的接口形状在这里定）。
  5. `frames.json` 增加合法、非法用例：`recall`、带 `quote` 和 `mentions` 的 `send`（`image` 带 `quote` 非法、`userIds` 21 个非法由服务端判断 4131 不在 Schema 里拦）、下行 `quote` 四种 `state`、撤回的 `msg` 和 `lastMsg`、`msgDeleted.recalled`、`groupMembers.keyword`。服务端 `logic` 用真实帧解析跑全部新用例（AC-CHAT-90-10 的 C4 服务端部分，TC-CHAT-90-32；AC-RCL-03-7，TC-RCL-03-07）。三套 SDK 同步 `protocol/` 副本，执行器随 FE-660、FE-661、FE-663。
  6. 本任务不做：`convSet`、`convDelete`、`convSettingsChanged`、`convDeleted`、`pinned*`、`notifyOff`（C5）；`report`、4510、4098（C6）。
- 工作量：1 人日（服务端 0.5，@ 行；文档 0.5，撤回行）

### BE-471 迁移、配置、开关和故障注入（C4）
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`store`、`config`、`app`、`testhook`）
- 依赖：BE-470
- AC：AC-ACC-10-4、AC-RCL-01-10、AC-CHAT-90-2、AC-CHAT-90-17
- 内容：
  1. `0018_msg_actions.sql`（第 12.1 节，只加可空或有默认值的列，SQLite / Postgres 兼容）：`messages` 加 `recalled_at`、`recalled_by`、`quote_msg_id`（带索引）；`conv_seq` 加 `last_recalled`、`last_recalled_by`（BE-431 第 1 条留的）；新表 `conv_mentions(user_id, conv_id, seq)`、`group_mention_all(group_id, seq, ts)`、@ 所有人次数计数（第 12.1 节"次数单独计数"，没有给表名，文末第 53 条：`mention_all_quota(group_id, window, bucket, count)`）。`store` 新方法走 `storetest` 契约用例，Postgres 随 B6。
  2. 第 4.1 节配置：`IM_MSG_ACTIONS_ENABLED`（默认关）、`IM_RECALL_WINDOW_SEC`（120）、`IM_GROUP_RECALL_WINDOW_SEC`（604800）、`IM_MENTION_MAX`（20）、`IM_MENTION_ALL_HOURLY` / `IM_MENTION_ALL_DAILY`（3 / 10）；撤回每人每分钟 20 次、成员搜索每人每分钟 30 次 PRD 没有变量名，用 `IM_RECALL_RATE_PER_MIN`、`IM_MEMBER_SEARCH_RATE_PER_MIN`（第 53 条）。开关关：`recall` 403 / 4030 `featureDisabled`，`send` 带 `quote` 或 `mentions` 同样（第 4.1 节共同规则）；`GET /v1/config` 的 `features` 增加 `msgActions`（`http-client.schema.json` 同步）。
  3. **启动检查**：开关开但没有设置 `IM_QUARANTINE_KEY` → 启动失败（AC-RCL-01-10；现在非生产环境会回退到开发默认值 `config.go` 第 249～252 行，开关打开时不回退，文末第 54 条）；`IM_ENV=production`、开关开、`IM_PRIVACY_NOTICE_VERSION` 为空 → 非 0 退出，错误信息指出缺隐私说明（AC-ACC-10-4，BE-424 第 3 条留的）。撤回不依赖 `IM_V13_ADMIN_FEATURES`（第 4.1 节 RC2-15）。
  4. 测试设施（第 4.5 节，BE-405 第 2 条留到 C4 的）：`POST /admin/test/faults` 增加 `quarantineWrite`（隔离写失败）、`mentionWrite`（提及表写失败）、`mentionAllCounter`（@ 所有人计数不可用）；只在测试模式生效（AC-CHAT-90-18 的生产拒绝不变）。
  5. AC-CHAT-90-2 的 C4 部分：开关关时现有 e2e 不改代码全过（TC-CHAT-90-28）。AC-CHAT-90-17 的 C4 部分：指标 `im_recalls_total{by}`（`by` 取 `sender`、`owner`、`admin`，有界）和配置检查项 `msg_actions`（开关状态、隔离密钥已设置、隐私说明版本），不含密钥（TC-CHAT-90-34）。
- 工作量：0.5 人日（服务端，撤回行）

### BE-472 撤回命令：发送者、群主和管理员
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`logic`、`store`、`quarantine`）
- 依赖：BE-471
- AC：AC-RCL-01-1、AC-RCL-01-2、AC-RCL-01-3、AC-RCL-01-4、AC-RCL-01-5、AC-RCL-01-7、AC-RCL-01-9、AC-RCL-01-11、AC-RCL-02-1、AC-RCL-02-2、AC-RCL-02-3、AC-RCL-02-6、AC-RCL-02-7、AC-RCL-04-2
- 内容：
  1. F-RCL-01 规则 3 的校验顺序：格式 → 会话权限（4032、4033、4043）→ 消息存在（4044，`msgId` 属于其他会话也是 4044）→ `tip` 4000 → 已撤回（幂等成功，不发事件）→ 已被后台删除（4044）→ 权限和时限（4030、4097）。**频率限制（4290）放在格式之后、查库之前**，幂等重试也计数（文末第 55 条）。不计入发送限流；禁言、被拉黑不影响（规则 4）。
  2. 时限按服务端时间 [IM 钟]：发送者 `IM_RECALL_WINDOW_SEC`；群主、管理员 `IM_GROUP_RECALL_WINDOW_SEC`；操作者撤回自己的取更宽（F-RCL-02 规则 1，**管理员撤回自己的消息也是 7 天**，AC 只测了群主，文末第 57 条）。4097 带 `details.windowSeconds`。
  3. F-RCL-02 角色矩阵：群主撤回任何人（含管理员）；管理员只能撤回普通成员；托管群没有群主时任何成员都不能撤回他人消息（规则 3）。**角色每次查库，不用 `members` 缓存**（规则 4）。被撤回者已退群仍可撤回；解散的群 4043。
  4. 规则 5 同一事务：清空正文，写 `recalled_at`、`recalled_by`，seq 不变；原文和文件名加密进隔离区（复用 `quarantine` 的加密器，`IM_QUARANTINE_KEY`，CH-17 留 30 天）；如果是最后一条，更新 `conv_seq` 的摘要和 `last_recalled*`；递增 `conv_seq.mod_ver`。**隔离写失败整个事务回滚**，500 / 5000，消息不变（AC-RCL-01-11 用 BE-471 的 `quarantineWrite` 注入）。文件消息的 `files` 行同一事务转 `revoked`、清空 `files.name`（规则 F-FILE-06 第 4 条的撤回路径，物理删除见 BE-473）。被撤回的提及从 `conv_mentions` 删除（见 BE-477）。
  5. 提交后投递 `msgDeleted {convId, msgIds, seqs, recalled:{by, at}}` 给全部在线参与者（含操作者其他端），事件只发一次（重试幂等）。未读公式、`updatedAt` 不变（规则 6，AC-RCL-01-7）。
  6. 群内自治不写后台审计（F-RCL-02 规则 5）；`im_recalls_total{by}` 计数。日志不记原文、文件名（AC-CHAT-90-5）。
  7. **不依赖 C3**：F-APN-02 规则 1"不推撤回事件"是推送 worker 的规则，C3 恢复后由 BE-455 处理；撤回替换通知见 BE-481。
- 工作量：1.5 人日（服务端，撤回行）

### BE-473 撤回的下发、同步、隔离和文件
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`logic`、`store`、`files`）
- 依赖：BE-472
- AC：AC-RCL-01-6、AC-RCL-02-4、AC-RCL-03-3、AC-RCL-03-5、AC-RCL-03-7、AC-RCL-04-6、AC-RCL-04-8、AC-FILE-06-3
- 内容：
  1. F-RCL-01 规则 5a、F-RCL-03 规则 2、4：`sync` 和 `msg` 下发 `deleted=true`、`recalled=true`、`recalledAt`、`recalledBy`，没有 `body`；`convs` 的 `lastMsg` 同样带 `deleted`、`recalled`、`recalledBy`，没有 `text`；`modVer` 增加（AC-RCL-03-5）。下行版 `logic.ToProto` 输出这一套，后台版见 BE-474。
  2. AC-RCL-01-6 的 SQLite 检查：撤回后做检查点（`PRAGMA wal_checkpoint(TRUNCATE)`），`messages`、`conv_seq`、`files` 原始字节和隔离表都搜不到特征串明文（隔离表只有密文）；Postgres 的死元组和备份不在这条（R-CS-34）。
  3. 访问控制（BE-435 第 2 条留的）：撤回的图片、文件参与者申请地址 404 / 4048（AC-RCL-04-6），和不存在完全相同。
  4. 生命周期（BE-436 第 2 条和问题 10 留的）：撤回的文件 `revoked`，对象保留 30 天后由 `file_purge` 物理删除（AC-FILE-06-3：第 29 天对象还在、用户申请 404，第 31 天对象被删除）；文件名在隔离区（密文），`files.name` 为空（AC-RCL-04-8）。
  5. 端到端 AC 的服务端部分：AC-RCL-02-4 的 `lastMsg.recalledBy` 和事件里的 `by`（三方文案由客户端按 F-RCL-02 规则 1a 的表生成，见 FE-665、667、669）；AC-RCL-03-3 断网期间撤回，重连后 `convs` 的 `modVer` 变化、`sync` 返回撤回状态。
- 工作量：0.5 人日（服务端，撤回行）

### BE-474 撤回和后台：管理接口、隔离读取、文件代理接口
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`admin`）
- 依赖：BE-473
- AC：AC-RCL-04-1、AC-RCL-04-3、AC-RCL-04-4、AC-RCL-04-5、AC-RCL-04-7、AC-RCL-04-9
- 内容：
  1. F-RCL-04 规则 1：`POST /admin/messages/delete` 对已撤回的幂等成功，状态和文案保持撤回（不写 `deleted_at`），审计照写，不发事件（F-ADM-31 规则 7 扩展，AC-ADM-31-6 的扩展见 BE-479）。
  2. 规则 3：**后台版序列化和下行版分开**（`admin/moderation.go` 第 623 行起 `readBodies` 现在直接用 `logic.ToProto`，RC2-21）：后台的 `deleted` 只表示后台删除，撤回的消息 `deleted=false`、`recalled=true`、`recalledAt`、`recalledBy`；普通正文授权对撤回的消息没有 `body`（AC-RCL-04-3）；隔离授权返回原文，"只返回被删的"扩展到撤回的；单聊仍受 `IM_C2C_BODY_ACCESS`（AC-RCL-04-5，403 `reason=c2cBodyDisabled`）。
  3. `/admin/convs/messages/query`（`msg.meta`）增加 `recalled`、`recalledAt`、`recalledBy`，没有 `body`，文件消息没有文件名（AC-RCL-04-7，R-CS-14；C2 加的 `fileSize` 撤回后不出现，和后台删除一致）。
  4. `quarantine_purge`（已有）覆盖撤回的隔离记录：31 天后物理删除，隔离授权读取 404 / 4044（AC-RCL-04-4）。
  5. **`POST /admin/files/{fileId}/fetch`**（BE-470 第 4 条定的形状，AC-RCL-04-9）：要 `msg.quarantine`（C4）且有这个会话的有效隔离授权，`fileId` 必须属于请求里的 `msgId`、消息已撤回或已被后台删除、对象还在保留期；im-server 按 `object_key` 从对象存储读出返回（不发预签名地址给 console），响应头固定 `attachment`、`nosniff`；没有授权 403 / 4039，普通调用方 403 / 4030；写 im-server 审计（不含文件名）。`report.evidence` 的路径随 C6。console 代理见 CS-202。
- 工作量：0.5 人日（服务端，撤回行）

### BE-475 引用：发送、组装、状态
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`logic`、`store`）
- 依赖：BE-471
- AC：AC-QUO-01-1、AC-QUO-01-2、AC-QUO-01-3、AC-QUO-01-4、AC-QUO-01-6、AC-QUO-01-7、AC-QUO-02-1、AC-QUO-02-4、AC-QUO-02-7
- 内容：
  1. 第 4.3 节第 7 步（BE-434 第 1 条留的）：只有 `text` 能带 `quote`（其他 4000）；被引用的消息同一会话、`seq ≥` 发送者的 `minSeq`、没有撤回或删除、不是 `tip`，否则 4040 `reason=quoteUnavailable`，**几种原因的响应完全相同**，消息不保存。`clientMsgId` 去重在第 7 步之前（AC-QUO-01-4：原消息已保存时返回第一次的 ack）。
  2. 只保存 `quote_msg_id`，**不把被引用的内容写进 `messages.body`**（AC-QUO-01-6）。下发时组装 `quote = {msgId, seq, from, type, state, text?}`：`text` 取前 100 个码点；图片"[图片]"；文件"[文件] {文件名}"；带引用的消息被引用时只取它自己的文字（不展开链，AC-QUO-01-7）；`recalled`、`deleted`、`expired` 没有 `text`，过期文件没有文件名；原消息已物理删除按 `deleted`（F-QUO-02 规则 1、2、异常）。`sync` 一页里批量组装，避免逐条查库。
  3. F-QUO-02 规则 4：撤回或后台删除被引用的消息时，同一会话的 `mod_ver` 已经递增（BE-472、后台删除），重拉范围内的引用拿到新状态（AC-QUO-02-1）。
  4. 规则 6：`historyVisible=false` 的新成员通过引用看得到入群前消息的摘要，但直接 `sync` 拉不到原消息（AC-QUO-02-4），按 PRD 接受。规则 7：敏感词不查引用摘要（C6）。
  5. 日志不记引用文字（AC-CHAT-90-5）。
- 工作量：1.5 人日（服务端，引用行）

### BE-476 @ 成员和 @ 所有人：校验、过滤、次数
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`logic`、`store`）
- 依赖：BE-471
- AC：AC-MEN-01-1、AC-MEN-01-2、AC-MEN-01-3、AC-MEN-01-4、AC-MEN-01-11、AC-MEN-02-1、AC-MEN-02-2、AC-MEN-02-3、AC-MEN-02-6、AC-MEN-02-7、AC-MEN-02-8、AC-MEN-02-9
- 内容：
  1. F-MEN-01 规则 1～3：只在群聊（单聊 4000）；只有 `text`（其他 4000）；`userIds` 超过 `IM_MENTION_MAX` 4131，去重；第 4.3 节第 8 步过滤不是当前成员的 userId（静默，不报错），全部被过滤按没有 `mentions` 保存；`mentions` 保存在消息 `body` 里（下发要带，撤回时随正文清除），**不解析正文里的 `@xxx`**（AC-MEN-01-4）。
  2. F-MEN-02：`all=true` 只有群主、管理员，普通成员 4030 `reason=mentionAllForbidden`、消息不保存；托管群没有群主时任何人都不能；角色每次查库；全群禁言时群主、管理员仍可以（AC-MEN-02-7）。
  3. 次数（规则 2，R-CB-07）：每群每小时 `IM_MENTION_ALL_HOURLY`、每天 `IM_MENTION_ALL_DAILY`（Asia/Shanghai 自然日，[IM 钟]）；**在 `SaveMessage` 事务里扣**（第 4.3 节第 10 步，加锁顺序 `conv_seq` → 文件行 → 计数行），被 4510 等拒绝的不扣，撤回不退；超出 4293 带 `details.retryAfterSeconds`；计数不可用 503 / 5030，不放行（AC-MEN-02-8 用 BE-471 的 `mentionAllCounter` 注入）；并发 10 条恰好 3 条成功（AC-MEN-02-9，SQLite 单连接下靠事务串行，Postgres 随 B6 用行锁）。小时窗口按滚动 60 分钟（文末第 58 条）。
  4. @ 所有人只写 `group_mention_all` 一行，不按成员展开（AC-MEN-02-6）。
  5. 规则 6 的成员搜索：`groupMembers` 的 `keyword`（`userId` 或昵称前缀，大小写按现有 `userId` 规则），每人每分钟 30 次（4290）；2000 人群只返回前缀匹配的（AC-MEN-01-11）。开关关时 `keyword` 照常可用（只读，文末第 59 条）。
  6. 性能：AC-MEN-02-3（2000 人、在线 200、1000 次 @ 所有人，ack P99 ≤ 200 ms）开发机只记录，判定等 P-12（BE-480）。
- 工作量：2 人日（服务端，@ 行）

### BE-477 提及计数：mentionUnread、firstMentionSeq
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`logic`、`store`）
- 依赖：BE-476、BE-472
- AC：AC-MEN-01-1、AC-MEN-01-4、AC-MEN-03-1、AC-MEN-03-4、AC-MEN-03-8
- 内容：
  1. F-MEN-03 规则 1：`SaveMessage` 同一事务写 `conv_mentions`（只记 @ 个人，自己 @ 自己不写，规则 6）；**提及表写失败整条消息不保存**（500 / 5000，AC-MEN-03-8 用 `mentionWrite` 注入）。
  2. `convs` 返回 `mentionUnread`（未读中提及我的数量：`conv_mentions` 里 `seq > readSeq` 的行数 + `group_mention_all` 里 `seq > readSeq` 且不是自己发的行数）和 `firstMentionSeq`；被撤回、删除的提及不计入（撤回时删行，后台删除同样）；已退群的不计算；已读前进后按已读位置重算（AC-MEN-03-1）。`conv_mentions` 定时清理到已读之后（第 12.1 节），复用现有定时任务调度，只在一个节点运行。
  3. 撤回提及消息后 `mentionUnread` 减 1（AC-MEN-03-4）；`convs` 的计算要批量，不按会话逐条查。
  4. 免打扰下的推送口径（规则 4、5）是 C5，C3 恢复后由推送 worker 读这里的提及表，本批不做。
- 工作量：1 人日（服务端，@ 行）

### BE-478 共享向量 chat-sync.json（撤回、引用、提及）
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`protocol/vectors`、`server/e2e/chatmodel`）
- 依赖：BE-470
- AC：AC-CHAT-90-15、AC-RCL-03-13、AC-QUO-02-6、AC-MEN-03-7
- 内容：
  1. BE-403 第 1 条留的：`chat-sync.json` 增加撤回合并（`recalled` 只能从假变真、迟到的旧 `sync` 带 `deleted=false` 和原文不回写，AC-RCL-03-13）、引用清除（`quote.state` 只能从 `normal` 往后走、`quote.text` 清除后不恢复；m1 不在已加载范围、m2 在范围内，离线期间 m1 被撤回，重连后 m2 的引用原文被清，AC-QUO-02-6）、提及计数（收到 `msgDeleted`、`readUpdated` 且本地 `mentionUnread > 0` 时节流 5 秒重新拉 `convs`，AC-MEN-03-7）。
  2. 期望值由 Go 参考客户端 `server/e2e/chatmodel` 生成（`-update`），`TestChatModelRules` 断言关键结论。三套 SDK 的执行器随 FE-660、FE-661、FE-663（AC-CHAT-90-15 的 C4 部分，TC-CHAT-90-33）。
  3. 置顶、删除会话、总未读的用例随 C5。
- 工作量：1 人日（测试：撤回 0.5、引用 0.5）

### BE-479 冻结文档变更（C4 涉及的部分）
- 优先级：P1
- 批次：C4
- 仓库：tianting-server（`docs/`）
- 依赖：BE-470
- AC：无（文档任务；要改写的 AC 列在内容里，不进这一行）
- 内容（第 16.1、16.2 节 C4 的部分，BE-408 第 10 条留的，走变更流程：PRD 升小版本，文末变更记录写原因，同步任务单和用例）：
  1. **改写 AC-UI-09-1**（菜单"只有复制"→ C4 开关关闭时只有复制，开启时按消息状态）：PRD-ui 升版，改 TC-UI-09-01；**AC-ADM-31-6** 的幂等扩展到已撤回的（AC-RCL-04-1），改 TC-ADM-31-06 的关联说明。AC-UI-09-7 不变（`custom` 没有菜单）。
  2. 第 16.1 节 C4 的部分：PRD-core 第 705 行（撤回、引用、@ 到第一批，编辑、正在输入、已读回执仍不做）、F-CONV-01 规则 1 的 `mentionUnread`、`firstMentionSeq`、F-MSG-01 规则 6 的第 7、8 步；PRD-social `groupMembers` 增加 `keyword`；PRD-ui 第 11 节、G-UI-05～13 里撤回、引用、@ 对应的"不做"，F-UI-04 规则 9（摘要前缀优先级）、F-UI-09 规则 4、5（菜单）、F-UI-42（撤回的并列规则）；PRD-admin F-ADM-31 规则 6、7 和 F-ADM-32（隔离区包含撤回原文和文件名）、第 12 节（门槛适用撤回隔离）；PRD-ops F-OPS-06、10 的撤回部分（`im_recalls_total{by}`、配置检查 `msg_actions`）。
  3. `docs/sdk/README.md`：`recall`、`send` 的 `{quote?, mentions?}`、`msgDeleted` 的 `recalled`、`mentionUnread`、`groupMembers` 的 `keyword`（随 FE-660～664）。
  4. `docs/reports/privacy-notice-checklist-draft.md` 增加 C4 的告知项（F-RCL-04 规则 4：撤回后对方看不到，平台加密留存 30 天用于处理投诉，措辞用"不再显示"）。**只是清单草稿，不代替法律文本**。
  5. `tools/trace.py` 已加 C4 里程碑（本次分发），PRD 升版时改 `PRDS` 文件名。
- 工作量：1 人日（文档：引用 0.5、@ 0.5）

### BE-480 C4 后端验收
- 优先级：P0
- 批次：C4
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-473、BE-474、BE-475、BE-477、BE-478、CS-202、CS-204
- AC：AC-CHAT-90-1、AC-CHAT-90-2、AC-CHAT-90-5、AC-CHAT-90-17、AC-RCL-01-1、AC-QUO-01-1、AC-MEN-01-1、AC-MEN-02-3
- 内容：
  1. 按 [TC-RCL](../testcases/TC-RCL.md)、[TC-QUO](../testcases/TC-QUO.md)、[TC-MEN](../testcases/TC-MEN.md) 落地后端 e2e，`rcl_*_test.go`、`quo_*_test.go`、`men_*_test.go`，用例名带 TC 编号；特征串检查（`ZXQ-RCL-*`、`ZXQ-QUO-*`）单独一个文件。
  2. 【全部】的 C4 部分（第 15 节累计表）：AC-CHAT-90-1 带引用的单聊、带 @ 的群聊（TC-CHAT-90-27）；90-2 C4 开关关（TC-CHAT-90-28）；90-5 撤回、引用、@ 流程的日志和审计（TC-CHAT-90-29）；90-17 撤回指标（TC-CHAT-90-34）。AC-CHAT-90-3 在 C4 没有新增依赖，不另跑。
  3. 性能（AC-CHAT-90-1、AC-MEN-02-3）开发机只记录，不算通过，等 P-12。
  4. 写验收报告 `docs/reports/c4-backend-acceptance-<日期>.md`：写明 **AC-APN-03-3、AC-RCL-03-9 挡在 C3（BE-481）**，状态"未执行（等 C3）"，按 PENDING P-43 ② 的结论处理。
- 工作量：3.5 人日（测试：撤回 1.5、引用 0.5、@ 1.5）

### BE-481 撤回和推送的衔接（依赖 C3，C3 恢复后做）
- 优先级：P0
- 批次：C4（**C3 恢复、BE-455、BE-456 完成后才能做**；C4 其余任务不等它）
- 仓库：tianting-server（`push`）
- 依赖：BE-472、BE-455、BE-456、BE-406
- AC：AC-APN-03-3、AC-RCL-03-9
- 内容：
  1. F-APN-03 规则 4（BE-456 第 5 条留的内存 LRU）：详情模式下 m1 已推送后被撤回，且 m1 是这台设备这个会话最后推送的一条 → 发同 collapse id 的替换通知（"该消息已被撤回"类的中性文案，不含原文）；m1 之后又推了 m2 → 不发（否则盖掉 m2）；隐藏模式不发（AC-APN-03-3）。替换通知失败不重试、不告警。
  2. F-APN-02 规则 1：撤回事件本身不入推送队列；隐藏模式消息被撤回，假通道没有调用（哨兵法，AC-RCL-03-9）。
  3. 撤回在入队后、调用通道前发生：worker 发送前复查消息状态，已撤回的不发（和 BE-453 第 2 条的 `sid` 复查放在一起）。
  4. 用例 TC-APN-03-03、TC-RCL-03-09，环境 E1 + 假通道。C3 恢复前状态写"未执行（等 C3）"。
- 工作量：0.5 人日（服务端，撤回行；从第 18 节撤回行服务端 3.5 里拆出）

---

## C5 会话置顶、免打扰、删除

> 2026-11-10 分发（依据 Frank 2026-11-10 在聊天里说"开始 C5"，满足第 17 节 C5 行"CH-27 校准后 Frank 决定继续"；QUEUE Q29）。依据 PRD 第 9 节 F-CVS-01～04，第 4.1、4.5、11、12.1、13～16 节的 C5 部分，其他章节标【C5】的 AC-APN-04-4、AC-MEN-03-2、AC-MEN-03-3，第 15 节累计表的 C5 列，第 17 节 C5 行，第 18 节 C5 行（24.5 人日）。开关 `IM_CONV_SETTINGS_ENABLED`。
> **C5 不依赖 C3 的代码**。依赖推送的 6 条 AC（AC-CVS-02-1、02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4）单独放在 BE-490，**C3 恢复、BE-406、BE-455、BE-456 完成后再做**，C5 其余任务不等它。判断依据（文末第 72 条）：这 6 条的预期都是"假通道 / 假 APNs 有没有调用、`badge` 是多少"，要推送 worker 和假通道才有调用可以断言（哨兵法的哨兵也要 worker 才会被调用）；main 上没有 `push` 包，也没有假 APNs、假 FCM（BE-406、BE-455、BE-456 都没有提交，只有不能直接用的本机草稿分支），C1、C2 的测试设施（假 S3、故障注入、双节点 `twoNodes`）里没有推送通道。能在 C5 先验的部分留在 C5：`unread`、`mentionUnread` 照常增加（BE-484 第 5 条），角标口径的聚合函数（BE-484 第 5 条，单测），`convSettings` 失效广播的发布端（BE-483 第 4 条）。**AC-CVS-02-6（App 前台不响铃）留在 C5**：预期只涉及客户端前台行为，不需要推送通道；三端现在没有任何系统通知和提示音代码，C3 恢复加通知后由 FE-653、FE-655 补跑 TC-CVS-02-06（frontend-chat 第 58 条）。
> 现状（2026-11-10 调查，tianting-server `61debab`）：没有 `conv_settings`、`convSet`、`convDelete`、4220；`convs`（`logic/logic.go` 第 393 行）按 `updatedAt` 取 500；`toConversation`（第 412 行）单聊 `minSeq` 固定 1、群按 `joinSeq`；`authorize`（第 489 行）单聊固定 1；`files/urls.go` 的 `canAccess`（第 137～141 行）自己算 `minSeq`，不经过 `authorize`。双节点测试设施 `e2e/social_twonode_test.go`（共享 SQLite、测试模式 Invalidator 互发）已有。
> 迁移号：C5 用 `0019_conv_settings.sql`，C3 恢复时 BE-451 用下一个空号（文末第 76 条）。

### BE-482 协议变更（C5 部分）：convSet、convDelete、新事件、4220
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`protocol/`、`server/internal/protocol`）
- 依赖：BE-470
- AC：AC-CHAT-90-10
- 内容：
  1. `frame.schema.json`（第 11.1 节）：上行 `convSet {convId, pinned?, notifyOff?}` → `{conv}`（完整的 `Conversation`；Schema 用 `anyOf` 要求至少一项，都没有时服务端 400 / 4000）；`convDelete {convId, upToSeq}` → `{clearedSeq}`（`upToSeq` 非负整数）；`Conversation` 增加 `pinned`、`pinnedAt`（只在置顶时出现）、`notifyOff`；`ConvsAck` 增加 `deleted: [{convId, clearedSeq}]`、`deletedTruncated`；新事件 `convSettingsChanged {convId, pinned, pinnedAt?, notifyOff}`、`convDeleted {convId, clearedSeq}`。字段叫 `notifyOff`，不用 `muted`（F-CVS-01 规则 1，R-CB-26）。
  2. `errors.json`、`ERRORS.md`（第 11.5 节）：新增 4220 `PIN_LIMIT_EXCEEDED`（ws，不重试，文案"最多置顶 20 个会话"，不带 `details`，文末第 80 条）和 `422x` 区段"业务状态不允许"；`4040` 的动作补"`convSet`：没有消息的单聊"；`4290` 的动作补会话设置（每人每秒 5 次）。`4030` 的 `featureDisabled` 已有。
  3. `http-client.schema.json`：`GET /v1/config` 的 `features` 增加 `convSettings`（文末第 75 条；同 C4 的 `msgActions`，老服务端没有这个字段按 false）。
  4. `frames.json` 增加合法、非法用例：`convSet` 只带 `pinned`、只带 `notifyOff`、都带、都不带（非法）；`convDelete` 的 `upToSeq` 为负（非法）；下行带 `pinned*`、`notifyOff` 的 `Conversation`，带 `deleted`、`deletedTruncated` 的 `convs` ack，两个新事件。服务端 `logic` 用真实帧解析跑全部新用例（AC-CHAT-90-10 的 C5 服务端部分，TC-CHAT-90-38）。三套 SDK 同步 `protocol/` 副本，执行器随 FE-680～682。
  5. 本任务不做：`report`、4510、4098（C6）；设备通知命令和 `presence`（C3，BE-450 已提交）。
- 工作量：0.5 人日（服务端）

### BE-483 迁移、Store、配置、开关和成员变动的清理
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`store`、`config`、`app`、`logic`、`cache`）
- 依赖：BE-482
- AC：AC-CVS-02-7、AC-CVS-01-5、AC-CVS-03-12、AC-CHAT-90-2
- 内容：
  1. `0019_conv_settings.sql`（第 12.1 节，只加新表，SQLite / Postgres 兼容）：`conv_settings(user_id, conv_id, pinned_at, notify_off, deleted_at, cleared_seq)`，主键 `(user_id, conv_id)`，数值列默认 0；索引 `(user_id, pinned_at)`、`(user_id, deleted_at)`。**只有设置过的会话才有行**。Store 接口（名字先查有没有被占用，第 50 条的教训）：按用户一次取齐、置顶（同一事务数当前置顶数，超过 4220）、免打扰、删除（`cleared_seq` 只增不减）、按成员删除、按群删除、按会话和一批 userId 批量取免打扰（给 C3 worker，F-APN-02 规则 7，BE-490 用）；走 `storetest` 契约用例，Postgres 随 B6。
  2. 第 4.1 节配置：`IM_CONV_SETTINGS_ENABLED`（默认关）、`IM_CONV_PIN_MAX`（20）；每人每秒 5 次的限流和 `deleted` 的 2000 上限 PRD 没有变量名，用 `IM_CONV_SETTINGS_RATE_PER_SEC`、`IM_CONV_DELETED_MAX`（文末第 73 条）。开关关：`convSet`、`convDelete` 403 / 4030 `featureDisabled`（AC-CVS-02-7）；`GET /v1/config` 的 `features.convSettings=false`。**开关关闭时已有的设置怎么处理要 Frank 定（文末第 81 条）**，定之前按建议实现：照常生效，只拦新命令。
  3. F-CVS-01 规则 6、F-CVS-03 规则 6a（AC-CVS-01-5、AC-CVS-03-12）：退群、被移出、解散在**同一事务**删除这个成员（解散时是整个群）的 `conv_settings` 行，入口和 C4 处理 `conv_mentions` 的地方一一核对（主动退群、群主或管理员移出、解散、后台移除、`account_cleanup` 的逐群退出）；重新加入不继承，`minSeq` 只看 `joinSeq` 和 `historyVisible`。单聊的行在注销时怎么处理见文末第 82 条（要 Frank 定）。
  4. 失效广播（第 11.4 节、F-CVS-04 规则 3）：`cache.Invalidator` 增加主题 `convSettings`（key 是 `userId`）；`convSet`、`convDelete`、第 3 条的清理提交后发布。C5 没有订阅者（`convs` 每次读库）；订阅者是 C3 的角标缓存（BE-490 第 3 条）。单测用记录型 Invalidator 断言发布了。
  5. AC-CHAT-90-2 的 C5 部分：开关关时现有 e2e 不改代码全过（TC-CHAT-90-35）；没有设置时 `convs` 不输出 `deleted`、`deletedTruncated`、`pinned`、`pinnedAt`、`notifyOff`（`omitempty`），AC-CONV-01-1 的断言不变。
  6. 第 15 节累计表 AC-CHAT-90-17 的 C5 列是"—"：不加指标和配置检查项。
- 工作量：1 人日（服务端）

### BE-484 置顶和免打扰：convSet、排序、500 上限、限流
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`logic`、`store`）
- 依赖：BE-483
- AC：AC-CVS-01-1、AC-CVS-01-2、AC-CVS-01-3、AC-CVS-01-4、AC-CVS-04-4
- 内容：
  1. `convSet` 的校验顺序：格式（没有任何字段 400 / 4000）→ 开关（4030）→ 频率（4290，格式之后、查库之前，和 `convDelete` 共用一个计数，文末第 73 条）→ 会话权限：单聊非参与方 4032；群不存在或已解散 4043；非成员 4033；单聊没有消息 4040（F-CVS-01 规则 5；**和 `convDelete` 对没有消息的单聊幂等成功不同**，两条都是原文，第 78 条）→ 写入（AC-CVS-01-4）。
  2. 置顶：原来没有置顶时 `pinned_at = 服务端时间`；**已置顶的再次置顶幂等成功、`pinnedAt` 不变**（AC-CVS-01-2）；置顶数在同一事务里判断，超过 `IM_CONV_PIN_MAX` 4220；`pinned=false` 清零。免打扰写 `notify_off`。写失败 500 / 5000。
  3. ack `{conv}`：和 `convs` 里这一项完全相同（同一个 `toConversation` 加设置）。
  4. `convs`（F-CVS-01 规则 3、4，F-CONV-01 规则 3 的扩展）：置顶的在最前（`pinnedAt` 倒序），其余 `updatedAt` 倒序，时间相同按 `convId`；500 上限里**置顶的一定在**，其余取最近更新的；`truncated` 含义不变（AC-CVS-01-1、01-3；AC-CONV-01-4、01-7 没有置顶时结果不变，第 88 条）。`ListConversations` 改成一条 SQL `LEFT JOIN conv_settings`，不按会话逐条查（同 C4 的 `applyMentionCounts`）。
  5. 免打扰下的未读（F-CVS-02 规则 3、4）：`unread` 照常计算（AC-CVS-02-1 的"`unread` 增加"在这里先验，TC-CVS-02-01 记部分通过）；总未读的服务端口径（普通会话 `unread` 之和 + 免打扰会话 `mentionUnread` 之和）写成 `store` 的一个聚合函数，C5 只有单测（用 AC-APN-04-4 的数据：3 条普通、免打扰 2 条其中 1 条 @，再来 1 条普通 → 5），C3 的角标直接用它（BE-490 第 3 条）。
  6. 限流（F-CVS-04 规则 4）：每人每秒 5 次，第 6 次 4290（AC-CVS-04-4）；不计入发送限流；`/admin/test/limits/reset` 能清。
- 工作量：1 人日（服务端）

### BE-485 删除会话：convDelete、clearedSeq、隐藏和重新出现
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`logic`、`store`、`files`）
- 依赖：BE-484
- AC：AC-CVS-03-1、AC-CVS-03-2、AC-CVS-03-3、AC-CVS-03-4、AC-CVS-03-6、AC-CVS-03-7、AC-CVS-03-9、AC-CVS-03-11、AC-CVS-03-12、AC-CVS-03-13、AC-CVS-03-14、AC-CVS-03-15
- 内容：
  1. 校验：格式 → 开关 → 频率 → 单聊非参与方 4032、非群成员 4033、已解散的群 4043（AC-CVS-03-9）；不存在的（没有消息的单聊）、已删除的再删：幂等成功（规则 8）。
  2. 同一事务（规则 1、2、3）：读 `conv_seq` 行（Postgres 加锁顺序接在第 4.3 节后面：`conv_seq` → `conv_settings`），`cleared = min(upToSeq, maxSeq)`；`cleared_seq = max(原值, cleared)`（**不回退**，AC-CVS-03-13）；`deleted_at = 服务端时间`；已读推进到 `max(readSeq, cleared)`（和 `read` 命令同样单调）；**清除置顶**（`pinned_at=0`，文末第 74 条），保留免打扰。ack `{clearedSeq}` 是生效后的值。
  3. **隐藏条件 `deleted_at > 0 AND max_seq <= cleared_seq`**（规则 2，R-CB-13）：`convs` 不返回这些会话。只设了免打扰的空群（`cleared_seq=0`、`deleted_at=0`）照常显示；删除空群（`upToSeq=0`）隐藏（AC-CVS-03-11）。有新消息（只有 `tip` 也算）时 `max_seq > cleared_seq`，自然重新出现，`unread` 只算新消息（AC-CVS-03-2、03-13），不另写状态。并发（规则 5）：`upToSeq` 之后到达的消息不会被清除，会话马上可见（AC-CVS-03-14，用 `go test -race` 和重复交错构造两种先后）。
  4. **`minSeq = max(原值, clearedSeq + 1)`**（规则 2）四处一起改：`toConversation`（单聊不再固定 1）；`authorize`（`sync`、`recall`、`quote` 都经过它：清除范围内的消息撤回 4044、引用 4040 `quoteUnavailable`，和"看不到的消息"一致，文末第 77 条）；`files/urls.go` 的 `canAccess`（清除范围内的图片、文件 404 / 4048，AC-CVS-03-7；其他成员照常，对象还在，AC-CVS-03-15）；C4 的 `mentionUnread` 只算 `seq > readSeq`，已读推进后自然减少。
  5. `convs` 的 `deleted: [{convId, clearedSeq}]`（规则 6）：单聊和群都有，**只列当前隐藏的**，按 `deleted_at` 倒序最多 `IM_CONV_DELETED_MAX`（2000），超出 `deletedTruncated=true`（AC-CVS-03-4、03-11，文末第 79 条）。
  6. 群（规则 4）：删除会话不是退群，成员、`myGroups`、`send` 不受影响（AC-CVS-03-4）。规则 6a：退群、被移出、解散删除这一行（BE-483 第 3 条），重新入群按新成员算，`historyVisible=true` 时看到全部历史，`deleted` 不再含它（AC-CVS-03-12）。
  7. 删除后有新消息（AC-CVS-03-6）：不再置顶，仍免打扰。
  8. 只影响自己：对方和其他成员的 `convs`、`sync`、文件地址不变（AC-CVS-03-1、03-15）；**后台查询不受影响**（管理接口不经过 `authorize`；没有单独的 AC，用例在 TC-CVS-03-01 第 4 步，console 一侧见 CS-205，文末第 87 条）。
  9. 写失败 500 / 5000，客户端恢复显示。日志只记 `convId`、`clearedSeq`。
- 工作量：1.5 人日（服务端）

### BE-486 多端同步：新事件和 ack
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`logic`）
- 依赖：BE-484、BE-485
- AC：AC-CVS-04-1、AC-CVS-04-2、AC-CVS-04-5、AC-CVS-03-2
- 内容：
  1. F-CVS-04 规则 1：提交后给**同一用户的其他在线连接**发 `convSettingsChanged {convId, pinned, pinnedAt?, notifyOff}`（`convSet`）或 `convDeleted {convId, clearedSeq}`（`convDelete`）；操作的连接只拿 ack（`DeliverToUser(uid, frame, exceptConn)`，同 `readUpdated`）。幂等的重复操作也发，内容是当前状态（客户端按服务端覆盖）。删除时**不另发** `readUpdated`、`convSettingsChanged`：`convDeleted` 已经意味着已读到 `clearedSeq`、置顶清除（文末第 84 条；SDK 的处理见 frontend-chat 第 55 条）。
  2. 规则 2：离线设备重连时 `convs` 带全部设置和 `deleted`，以服务端为准（AC-CVS-04-2）；事件丢失下次 `convs` 补齐；两端同时改同一会话后到的覆盖。
  3. 端到端 AC 的服务端部分：AC-CVS-04-1（D2 2 秒内收到 `convSettingsChanged`）、04-5（D2 2 秒内收到 `convDeleted`）、03-2（u2 再发后 u1 的 `convs` 重新出现，`unread=1`、`minSeq=11`）、04-2（D2 重连后 `convs` 和 D1 一致）；客户端部分随 FE-680～686。
  4. 集群（第 15 节）：事件走 `Router`，跨节点投递随 B6；第一批单节点。
- 工作量：0.5 人日（服务端）

### BE-487 共享向量 chat-sync.json（置顶排序、deleted 判断、总未读）
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`protocol/vectors`、`server/e2e/chatmodel`）
- 依赖：BE-482
- AC：AC-CHAT-90-15、AC-CVS-03-5、AC-CVS-02-5
- 内容：
  1. BE-478 第 3 条留的：`chat-sync.json` 增加置顶排序（`pinnedAt` 倒序在前，其余 `updatedAt`，同时间 `convId`；`convSettingsChanged` 改变排序）；**`deleted` 判断**（`convs` 里消失的群先查 `deleted`：在里面的是删除会话，从列表移除，**不生成 `removed` 快照**；不在里面的按 F-GRP-03 规则 4 推断"你已不在该群"；`truncated` 或 `deletedTruncated` 为 true 时不推断，AC-CVS-03-5，frontend-chat 第 54 条）；本地清除（`seq < minSeq` 的消息、文件缓存键都清，`convs` 和 `convDeleted` 两条路径；**保留下来的消息里引用被清除消息的摘要照常显示**，P-46 ④，第 91 条）；`convDeleted` 之后的状态（`readSeq ≥ clearedSeq`、不再置顶、仍免打扰）；总未读（普通会话 `unread` + 免打扰会话 `mentionUnread`：AC-CVS-02-5 的 3，AC-CVS-02-4 的 0）。
  2. 期望值由 Go 参考客户端 `server/e2e/chatmodel` 生成（`-update`），`TestChatModelRules` 断言关键结论。三套 SDK 的执行器随 FE-680～682（AC-CHAT-90-15 的 C5 部分，TC-CHAT-90-39）。
- 工作量：0.5 人日（测试）

### BE-488 冻结文档变更（C5 涉及的部分）
- 优先级：P1
- 批次：C5
- 仓库：tianting-server（`docs/`）
- 依赖：BE-482
- AC：无（文档任务；要加注的 AC 列在内容里，不进这一行）
- 内容（第 16.1、16.2 节 C5 的部分，走变更流程：PRD 升小版本，文末变更记录写原因，同步任务单和用例；做法同 BE-479 的 `40b9a85`）：
  1. PRD-core（升小版本）：F-CONV-01 规则 4"一期不做置顶、删除会话、免打扰"（现在第 538 行，第 16.1 节写的是旧版第 512 行）**作废**，见 PRD-chat F-CVS；F-CONV-01 规则 1、2、3：`convs` 增加 `pinned`、`pinnedAt`、`notifyOff`、`deleted`、`deletedTruncated`，排序增加置顶，删除的会话不出现；F-CONV-02 规则 5：总未读口径（F-CVS-02 规则 4）；F-GRP-03 规则 4：快照先排除 `deleted` 列表。AC-CONV-01-1（只加可选字段）、AC-CONV-01-2、01-4、01-7（没有置顶时结果不变）、AC-GRP-03-3（另有 AC-CVS-03-5 区分删除会话）断言不变，规则里加注（第 88 条）。
  2. PRD-ui（升小版本）：第 126 行、第 11 节、G-UI-07、G-UI-08 去掉置顶、免打扰、删除会话的"不做"（G-UI-07 的"不做仅本地置顶"保留，F-CVS-04 规则 2）；F-UI-04 规则 9 改成会话菜单（长按、右键、键盘，AC-CVS-01-6），规则 4 的免打扰小红点、规则 5 的"[N 条]"、规则 7 的置顶排序；F-UI-23：**AC-UI-23-1、23-2 加注 F-CVS-02 规则 4**（没有免打扰时结果不变，第 16.2 节），TC-UI-23-01、02 的说明同步；第 6.1 节新增置顶背景 token（浅色、深色，带对比度计算），第 6.6 节加免打扰图标（第 13.3 节，frontend-chat 第 57 条）。
  3. PRD-admin、PRD-social、PRD-ops：第 16.1 节没有 C5 的行，第 14.2 节 C5 没有新指标和配置检查项，不改。
  4. `docs/sdk/README.md`：`setConvPinned`、`setConvNotifyOff`、`deleteConv`，`conversations` 的新排序，`totalUnread()` 的口径，`convs.deleted` 和本地清除（随 FE-680～682）。
  5. `docs/reports/privacy-notice-checklist-draft.md` 增加 C5 的告知项：删除会话只是自己这边不再显示，对方和其他成员不受影响，平台仍然保存消息（后台查询不受影响，F-CVS-03 规则 2）；置顶、免打扰按账号保存、多端同步。**只是清单草稿，不代替法律文本**。第 82 条 Frank 定了之后补注销的处理。
  6. `tools/trace.py` 已加 C5 里程碑（本次分发）；PRD 升版时改 `PRDS` 文件名。
- 工作量：0.5 人日（文档）

### BE-489 C5 后端验收
- 优先级：P0
- 批次：C5
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-485、BE-486、BE-487、CS-205
- AC：AC-CHAT-90-2、AC-CHAT-90-10、AC-CVS-01-1、AC-CVS-03-1、AC-CVS-03-2、AC-CVS-04-1
- 内容：
  1. 按 [TC-CVS](../testcases/TC-CVS.md) 落地后端 e2e，`cvs_*_test.go`，用例名带 TC 编号；"只影响自己"和清除的特征串检查（`ZXQ-CVS-*`）单独一个文件。
  2. 【全部】的 C5 部分（第 15 节累计表）：AC-CHAT-90-2 C5 开关关（TC-CHAT-90-35）；90-10 新帧（TC-CHAT-90-38，服务端部分随 BE-482）。90-1、90-3 在 C5 不变，90-5、90-17 是"—"，不另跑。
  3. 写验收报告 `docs/reports/c5-backend-acceptance-<日期>.md`：写明 **AC-CVS-02-1、02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4 挡在 C3（BE-490）**，状态"未执行（等 C3）"；AC-CVS-02-1 的 `unread` 部分记"部分通过"。这 6 条怎么算要 Frank 定（文末第 72 条，建议同 P-43 ②）。
- 工作量：2 人日（测试）

### BE-490 免打扰和推送的衔接（依赖 C3，C3 恢复后做）
- 优先级：P0
- 批次：C5（**C3 恢复、BE-406、BE-455、BE-456 完成后才能做**；C5 其余任务不等它）
- 仓库：tianting-server（`push`、`testhook`）
- 依赖：BE-484、BE-406、BE-455、BE-456
- AC：AC-CVS-02-1、AC-CVS-02-8、AC-CVS-04-3、AC-MEN-03-2、AC-MEN-03-3、AC-APN-04-4
- 内容：
  1. F-CVS-02 规则 2，F-MEN-03 规则 4、5：worker 按消息批量取收件人的免打扰（BE-483 第 1 条的批量接口，F-APN-02 规则 7，和令牌、拉黑一起取齐，不逐人查）。免打扰会话：普通消息不推，@ 所有人不推，@ 我推（AC-MEN-03-2）；被我拉黑的人在群里 @ 我不打断免打扰（`mentionUnread` 照常增加，令牌无调用，AC-MEN-03-3）；单聊免打扰不推（AC-CVS-02-1）。判定写成纯函数，矩阵单测。
  2. F-CVS-02 异常：设置读取失败按没有免打扰推送（宁可多推），消息正常；`POST /admin/test/faults` 增加 `convSettingsRead`（第 4.5 节"设置读取失败"），只在测试模式生效（AC-CVS-02-8，AC-CHAT-90-18 的生产拒绝不变）。`convs` 读设置失败不降级（文末第 90 条）。
  3. 角标（F-APN-04 规则 2、F-CVS-02 规则 4）：用 BE-484 第 5 条的聚合，`badge` = 普通会话未读之和 + 免打扰会话 `mentionUnread` 之和（AC-APN-04-4：`badge=5`）；聚合的 60 秒缓存（F-APN-02 规则 7）订阅 `convSettings` 主题（BE-483 第 4 条），`convSet`、`convDelete` 之后立即失效。
  4. 改完立即生效（F-CVS-04 规则 3）：worker 每条消息读库，不缓存免打扰（第 89 条）。AC-CVS-04-3 用双节点 `twoNodes` 加每个节点一个假通道：节点 1 设免打扰后断开，1 秒后节点 2 发消息，节点 2 的假通道无调用（哨兵法）。第 15 节"第一批推送只支持单节点"不受影响，这条验的是节点 2 读到新设置（第 83 条）。
  5. 用例 TC-CVS-02-01、02-08、04-03，TC-MEN-03-02、03-03，TC-APN-04-04；环境 E1 + 假通道（AC-CVS-04-3 是双节点 + 假通道）。C3 恢复前状态写"未执行（等 C3）"。
- 工作量：0.5 人日（服务端；从第 18 节 C5 行服务端 5 里拆出）

---

## C6 敏感词和举报

> 2026-11-11 分发（依据 Frank 2026-11-11 在聊天里说"开始 C6"，PENDING P-47，满足第 17 节 C6 行"CH-27 校准后 Frank 决定继续"；QUEUE Q30）。依据 PRD 第 10 节 F-MOD-01～04，第 4.1、4.3、4.5、11、12、14、15、16 节的 C6 部分，其他章节标【C6】的 AC-ACC-01-9、AC-ACC-10-5、AC-CHAT-90-4，第 15 节累计表的 C6 列，第 17 节 C6 行，第 18 节 C6 行（28 人日）；后台部分另依据 PRD-admin v1.4 第 5 节（角色和权限点）、F-KEY-02、F-ADM-08、F-ADM-30 和第 12 节。开关 `IM_MOD_ENABLED`；证据密钥 `IM_REPORT_KEY`。
> **开发不阻塞、上线有门槛**：词库负责人（CH-19）和举报处理人（CH-22）由 Frank 在 C6 上线前指定（第 20.3 节，P-47）。**仓库、代码、用例、向量、部署示例里都不放真实词库**（F-MOD-01 规则 5），开发和测试只用特征词：AC 原文里的 `违规词`、`badword`、`ZXQWORD` 和 `ZXQ-*` 特征串；性能用例的 10 万词由测试现场随机生成，不落盘、不提交。上线门槛清单见 BE-498 第 4 条（文末第 93 条）。
> **C6 不依赖 C3**：逐条核对了 F-MOD-01～04 的 58 条和 AC-ACC-01-9、AC-ACC-10-5、AC-CHAT-90-4，没有一条的预期要推送 worker 或假通道（"2 秒内收不到任何帧"说的都是 WS 帧，用现有的哨兵法），所以**没有"C3 恢复后做"的任务**（文末第 94 条）。C3 恢复时 BE-459 补一条哨兵用例：4510 拦下的消息、举报、结案都不进推送队列。
> **`IM_C2C_BODY_ACCESS` 和 C6 的关系**（文末第 95 条，都是 PRD 原文，不是新决定）：① 存证据时读开关（F-MOD-03 规则 5）：`off` 时单聊只存元数据（不存正文、文件名和文件引用，也就不延长文件保留）；`on` 时和群一样。② 读证据时再读一次（F-MOD-04 规则 4）：`off` 时单聊证据和单聊文件一律 403 / 4030 `c2cBodyDisabled`，不管当时存了什么（AC-MOD-04-3、04-13）；`off` 期间的举报，之后打开开关也没有正文（AC-MOD-03-10）。③ 单聊举报的元数据照常可看，要理由、每天 50 个，和 F-ADM-30 规则 3 合并（F-MOD-04 规则 2）。④ 开关只能改配置重启（PRD-admin F-KEY-02 规则 3），打开要先满足 PRD-admin 第 12 节第一行的 ①～④。⑤ `on` 时单聊证据用哪个权限读，PRD 没写，见第 104 条（要 Frank 定，不阻塞：第一批 `off`）。
> 现状（2026-11-11 调查，tianting-server `a6a2a4f`）：没有 `blocked_words`、`moderation_hits`、`reports`、`report_evidence`、`report` 命令、4510、4098；没有 `IM_MOD_ENABLED`、`IM_REPORT_KEY`、`IM_CLIENT_CUSTOM_MSG`，客户端现在能发 `custom`；`config/checks.go` 第 133～136 行对生产 `open` 一律拒绝（注释"逐项报错随 C6"）；`adminauth.IsBodyScope`（`scope.go` 第 46 行）只有 `msg.body.*`、`msg.quarantine`；`admin/files_fetch.go` 只认 `msg.quarantine`（第 24 行注释"report.evidence 随 C6"）；`files.evidence_until` C2 已建列（`0017_files.sql` 第 32 行）；`account_events_purge` C1 已有（`app/app.go`、`account/service.go`）；第 4.3 节第 6～10 步失败退还 `newPeerReject` 的 `onReject` C2 已有（`logic/social.go` 第 731 行）；AES-GCM 加密器 `quarantine.Cipher` C4 已有，证据用同一实现、不同密钥；`golang.org/x/text` 已在 `go.mod`（标准化不加新依赖）。`friendApply`、`groupCreate`、`groupUpdate` 在 `logic/social.go` 第 184 行、`logic/group.go` 第 117、392 行；注册、改资料在 `account/register.go` 第 45 行、`account/profile.go` 第 24 行。console 的单聊元数据配额在 console-server `internal/app/bizmeta.go` 第 142 行（`dailyQuota("msg.meta.c2c", …)`）。
> 迁移号：C6 用 `0020_moderation.sql`；C3 恢复时 BE-451 用下一个空号（同第 52、76 条）。

### BE-491 协议变更（C6 部分）：report、4510、4098、管理接口
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`protocol/`、`server/internal/protocol`）
- 依赖：BE-482
- AC：AC-CHAT-90-10
- 内容：
  1. `frame.schema.json`（第 11.1 节）：上行 `report {targetType, convId?, msgIds?, userId?, groupId?, reason, note?}` → `{reportId}`。Schema 按 `targetType` 用条件约束：`message` 必须有 `convId`、`msgIds`（非空数组），不能带 `userId`、`groupId`；`user` 必须有 `userId`；`group` 必须有 `groupId`；`reason` 是 `spam`、`abuse`、`porn`、`fraud`、`illegal`、`other`；`note` 字符串。**`msgIds` 超过 20 不写进 Schema**（服务端返回 4131，AC-MOD-03-2）；`note` 的 200 个码点由服务端按码点判（Schema 的长度是 UTF-16 单位，只能近似）。`convId` 不能是聊天室。
  2. `errors.json`、`ERRORS.md`（第 11.5 节）：新增 4510 `CONTENT_BLOCKED`（451，`ws`、`client`，不重试，文案"消息包含不当内容，无法发送"，**不带 `details`**，F-MOD-02 规则 6）、4098 `REPORT_ALREADY_HANDLED`（409，`admin`、`console`，"举报已被处理"，动作"刷新列表"）。已有码的扩展：`4040` 加 `reason: reportTargetInvalid`；`4000` 加 `reason: customDisabled`；`4131` 的 scope 加 `ws`（举报超过 20 条，C4 已为 @ 加过的话核对即可）；`4292` 按"命令 + 码"加举报的文案"今天的举报次数已用完"（RC2-20）；`4030` 的 `featureDisabled`、`c2cBodyDisabled` 已有。
  3. `http-client.schema.json`：`GET /v1/config` 的 `features` 增加 `moderation`（文末第 96 条；同 C4 的 `msgActions`、C5 的 `convSettings`，老服务端没有这个字段按 false）。
  4. `ADMIN.md`（第 11.6 节）：scope 目录加 `moderation.words`、`report.read`、`report.handle`、`report.evidence`（正文类）；写清 `/admin/moderation/words`（含 `import`）、`/admin/moderation/hits`、`/admin/reports`、`/admin/reports/{id}`、`/admin/reports/{id}/resolve`、`/admin/report-grants`、`/admin/reports/{id}/evidence` 的请求和响应（PRD 没写，按第 97 条），以及 `POST /admin/files/{fileId}/fetch` 的 `report.evidence` 路径（第 109 条）。
  5. `frames.json` 增加合法、非法用例：三种 `targetType` 各一个合法的；`message` 缺 `msgIds`、`message` 带 `userId`、`reason=xxx`（非法）；ack `{reportId}`；4510 的错误 ack（没有 `details`）。服务端 `logic` 用真实帧解析跑全部新用例（AC-CHAT-90-10 的 C6 服务端部分，TC-CHAT-90-46）。三套 SDK 同步 `protocol/` 副本，执行器随 FE-690～692。
  6. 第 15 节累计表 AC-CHAT-90-15 的 C6 列是"—"：`chat-sync.json`、`account-session.json` 不加用例。
- 工作量：1 人日（服务端 0.5；文档 0.5：`ERRORS.md`、`ADMIN.md`）

### BE-492 迁移、配置、开关、启动检查和运维项
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`store`、`config`、`app`、`adminauth`、`metrics`、`testhook`、`deploy/`）
- 依赖：BE-491
- AC：AC-ACC-01-9、AC-ACC-10-5、AC-MOD-01-10、AC-MOD-04-8、AC-MOD-02-10、AC-MOD-02-15、AC-MOD-03-14、AC-CHAT-90-2、AC-CHAT-90-17
- 内容：
  1. `0020_moderation.sql`（第 12.1 节，只加新表，SQLite / Postgres 兼容，时间 `BIGINT` 毫秒）：`blocked_words(word_id PK, word, norm, compact, state, created_at, updated_at)`，`norm` 唯一（第 100 条）；`moderation_hits(user_id, scene, word_id, at)`，索引 `(user_id, at)`、`(at)`；`reports(report_id PK, reporter, target_type, conv_id, target_user, group_id, msg_ids, reason, dedup_key, state, conclusion, handled_by, handled_at, created_at)`，索引 `(state, created_at)`、`(reporter, dedup_key, created_at)`、`(target_user)`；`report_evidence(report_id, idx, ciphertext, expires_at)`，`note`、文件名、正文都只在密文里（第 122 条）。Store 接口名先查有没有被占用（第 50 条的教训），走 `storetest` 契约用例，Postgres 随 B6。
  2. 第 4.1 节配置：`IM_MOD_ENABLED`（默认关）、`IM_REPORT_DAILY`（20）、`IM_REPORT_EVIDENCE_DAYS`（180）、`IM_CLIENT_CUSTOM_MSG`（`IM_ENV=production` 时默认 0，否则默认 1）、`IM_REPORT_KEY`（32 字节随机值，base64）；`moderation_hits` 的 90 天 PRD 没有变量名，用 `IM_MOD_HITS_DAYS`（第 98 条）。开关关：不检查敏感词（AC-MOD-02-10），`report` 403 / 4030 `featureDisabled`（AC-MOD-03-14 后半），`features.moderation=false`；管理接口和定时清理照常（第 108 条，要 Frank 确认）。
  3. 启动检查：① `IM_MOD_ENABLED=1` 时缺 `IM_REPORT_KEY`、长度不对、或和 `IM_QUARANTINE_KEY`、`IM_JWT_SECRET` 相同：**任何环境都启动失败**（第 99 条，同第 54 条的做法）。② 生产 + `IM_MOD_ENABLED=1`：要 `IM_V13_ADMIN_FEATURES=1`（AC-MOD-01-10，第 4.1 节）和 `IM_PRIVACY_NOTICE_VERSION`（AC-ACC-10-5，F-ACC-10 规则 4）。③ 生产 + `IM_REGISTER_MODE=open`：把 `checks.go` 第 133～136 行的一律拒绝改成逐项检查 `IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=1`、词库非空、`IM_PUBLIC_OPEN_ACK=1`，错误信息指出缺哪一条（AC-ACC-01-9）；词库非空要读库，放在迁移之后。AC-ACC-01-19 的输入不满足这些条件，结果仍是拒绝，用例不改（第 106 条）。④ 调用方配置：`report.evidence` 加进 `adminauth.IsBodyScope`，`console` 同时带 `report.evidence` 和 `msg.meta` 非 0 退出、指出正文类 scope 的限制（AC-MOD-04-8）；scope 目录加 4 个新 scope，未声明 scope 的新路由启动失败（AC-KEY-02-3 不变）。
  4. 配置检查（F-OPS-10，第 14.2 节）新增 `moderation` 项：开关状态、词库是否为空（告警，F-MOD-01 规则 6）、`IM_REPORT_KEY` 是否已设置（不含值）、生产打开 `IM_CLIENT_CUSTOM_MSG` 的告警（AC-MOD-02-15）。指标（F-OPS-06）：`im_blocked_total{scene}`（场景见第 110 条）、`im_mod_reload_errors_total`、`im_reports_pending`（第 107 条），标签没有无界字段（AC-OPS-90-4）。告警（F-OPS-07）：词库加载失败是第一批唯一默认开的页面告警，加进 `deploy/` 的告警规则；举报积压的告警默认不开（F-MOD-04 规则 7）。运维后台的配置检查页是按接口数据通用显示的（`tianting-ops/src/pages/Config.tsx`），不改页面。AC-CHAT-90-17 的 C6 部分（TC-CHAT-90-47）。
  5. 故障注入（第 4.5 节，只在测试模式生效，AC-CHAT-90-18 的生产拒绝不变）：`POST /admin/test/faults` 增加 `wordsReload`（重新加载失败）、`wordsStartupLoad`（启动加载失败）、`evidenceWrite`（证据写失败）。"证据密钥不匹配"用换一把 `IM_REPORT_KEY` 重启来造，不加注入。
  6. AC-CHAT-90-2 的 C6 部分：开关关时现有 e2e 不改代码全过（TC-CHAT-90-41）；`IM_CLIENT_CUSTOM_MSG` 不跟 `IM_MOD_ENABLED` 走（第 105 条），开发和测试环境默认 1，现有发 `custom` 的 e2e 不受影响。
- 工作量：1 人日（服务端）

### BE-493 词库：存储、标准化、匹配、生效和管理接口
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`moderation`（新包）、`store`、`admin`、`cache`）
- 依赖：BE-492
- AC：AC-MOD-01-1、AC-MOD-01-2、AC-MOD-01-3、AC-MOD-01-4、AC-MOD-01-6、AC-MOD-01-8、AC-MOD-01-9、AC-CHAT-90-4
- 内容：
  1. 管理接口（第 11.6 节，scope `moderation.words`）：`GET /admin/moderation/words?q=&state=&cursor=&limit=`；`POST`（单个；空、超过 64 个码点 400 / 4000，第 102 条）；`PATCH /{id}`（只改 `state`）；`DELETE /{id}`（物理删除，`moderation_hits` 的 `word_id` 保留、不级联）；`POST .../import`（`{words: [...]}`，一次最多 5000，超出 400 / 4131，AC-MOD-01-4；空行、超过 64 码点的跳过计入 `invalid`，AC-MOD-01-9；结果 `{added, duplicate, invalid}`，第 103 条）。**去重按 `norm`**（第 100 条）：已有的（含停用的）计入 `duplicate`，状态不变。
  2. im-server 侧审计（F-KEY-04，AC-MOD-01-6）：`admin/audit.go` 的字段白名单只记操作、`wordId` 和数量（`count`、`added`、`duplicate`、`invalid`），**不记词内容**；请求日志不打请求体。console 一侧见 CS-206。
  3. 标准化（规则 3）：NFKC → 去变音符（NFD 后去掉 `Mn`）→ 英文小写 → 去掉 Default_Ignorable（零宽、变体选择符、Bidi 控制等）得到 `norm`；再去掉空白和标点得到 `compact`。**词和待查文本用同一个函数**（第 101 条：词里本来有空格、标点的，`compact` 里也去掉）。只用于匹配，存储和下发的仍是原文（AC-MOD-02-3）。用已有的 `golang.org/x/text`，不加依赖。语料误判测一次（R-CS-17），结果写进 BE-499 的报告。
  4. 匹配（规则 4）：`norm` 和 `compact` 各一个 Aho-Corasick 自动机，构建成不可变快照，用 `atomic.Pointer` 整体替换，匹配不加锁；返回第一个命中的 `word_id`（只用于记录，不返回给客户端）。AC-CHAT-90-4：10 万词（随机中英文，长度 2～8，测试现场生成）、5000 码点不命中的中文文本、10000 次单线程，P99 ≤ 2 ms（未校准）；写成 `go test -bench` + TC-CHAT-90-42，开发机结果只记录。
  5. 生效（规则 2）：写操作提交后经 `cache.Invalidator` 发主题 `blockedWords`，各节点订阅后重建快照，10 秒内生效（AC-MOD-01-1、01-2）；另有 60 秒 TTL 兜底重载。双节点用现有 `twoNodes`（AC-MOD-01-3）。
  6. 失败（F-MOD-01 异常）：重新加载失败沿用内存里的旧快照，`im_mod_reload_errors_total` 加 1，日志不含词（AC-MOD-01-8 第 1 种）；`IM_MOD_ENABLED=1` 时启动加载失败直接非 0 退出（第 2 种）；词库为空时不拦截、配置检查告警（第 3 种）。
  7. 停用的词不进自动机。`IM_MOD_ENABLED=0` 时不加载、不建自动机，管理接口照常可用（第 108 条）。
- 工作量：1.5 人日（服务端）

### BE-494 发送检查：各场景、custom、命中记录
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`logic`、`account`、`store`、`admin`）
- 依赖：BE-493
- AC：AC-MOD-02-1、AC-MOD-02-2、AC-MOD-02-3、AC-MOD-02-4、AC-MOD-02-5、AC-MOD-02-6、AC-MOD-02-7、AC-MOD-02-8、AC-MOD-02-11、AC-MOD-02-12、AC-MOD-02-13、AC-MOD-02-14、AC-MOD-02-16
- 内容：
  1. 单聊、群聊发送（第 4.3 节第 9 步，F-MOD-02 规则 5）：在格式、去重、禁言、限流、单聊权限 / 群成员、附件预检、引用校验、@ 所有人权限**之后**检查；命中 4510，消息不保存、不投递（AC-MOD-02-1）；已扣发送令牌；单聊新对象的 `newPeerReject` 走已有的 `onReject` 退还（AC-MOD-02-12；AC-MOD-02-13 的 4048、4096、4040 C2、C4 已经退还，C6 补用例）；@ 所有人次数在保存事务里扣，4510 不扣（AC-MOD-02-16）。被拉黑、被禁言的先返回 4036、4031（AC-MOD-02-7）。已保存的消息重发走第 2 步去重，不重新检查（AC-MOD-02-14 前半，规则 8）。
  2. 检查内容（规则 2、3）：`text` 的 `text`；`image`、`file` 的**服务端记录的文件名**（`files.name`，不是客户端上行的字段，第 111 条；AC-MOD-02-4）；不查引用摘要、`tip`、管理接口写入的内容、系统通知、图片和文件内容；`custom` 不查（生产禁止客户端发送，第 3 条）。
  3. `custom`（规则 4）：`IM_CLIENT_CUSTOM_MSG=0` 时客户端 `send` 的 `type=custom` 返回 400 / 4000 `reason: customDisabled`，消息不保存（AC-MOD-02-6）；`=1` 时行为和现在一样（AC-MOD-02-11）；只影响客户端 `send`，管理接口和 `frames.json` 不受影响。放在第 1 步格式校验里。
  4. 其他场景（规则 2，AC-MOD-02-5）：注册的用户名和昵称（`/v1/account/register`，在邀请码校验之后、用户名唯一性判断之前，第 112 条）；改昵称（`PATCH /v1/account/profile`）；客户端的 `groupCreate` 群名和公告、`groupUpdate` 的群名和公告（在群权限判定之后）；**好友申请附言**（`friendApply`：格式、限流之后，**F-SOC-02 的存在性判断之前**，不存在的对象也返回 4510，响应和对方是否存在无关，第 112 条）。都是"内容未写入"。HTTP 接口返回 451 / 4510。C6 上线前已注册的违规用户名不能改名，后台封禁处理（规则 2，部署说明写明）。
  5. 命中记录（规则 7）：写 `moderation_hits(user_id, scene, word_id, at)`，**不存消息全文**，日志只记场景和 `word_id`（AC-MOD-02-8）；场景名见第 110 条；注册时还没有用户，只计指标不写行（第 113 条）。写入失败不影响拦截结果。`im_blocked_total{scene}` 加 1。
  6. 查询和清理：`GET /admin/moderation/hits?userId=`（scope `report.read`）返回这个用户在保留期内按场景的次数和最近 50 条（时间、场景、`word_id`），不含任何内容（第 114 条）；`moderation_hits_purge`（单节点定时任务，同 F-ADM-32）清除 `IM_MOD_HITS_DAYS` 之前的行，[IM 钟] 推进 91 天后被清除（AC-MOD-02-14 后半）。
  7. 响应不告诉用户命中了哪个词（规则 6）：4510 没有 `details`，日志和审计也不记命中的词内容。
- 工作量：1.5 人日（服务端）

### BE-495 举报提交：校验、限额、幂等、证据加密和清理
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`logic`、`store`、`files`、`scheduler`）
- 依赖：BE-492
- AC：AC-MOD-03-1、AC-MOD-03-2、AC-MOD-03-3、AC-MOD-03-4、AC-MOD-03-5、AC-MOD-03-6、AC-MOD-03-7、AC-MOD-03-8、AC-MOD-03-10、AC-MOD-03-11、AC-MOD-03-12、AC-MOD-03-13、AC-MOD-03-14、AC-MOD-03-15
- 内容：
  1. 校验顺序：格式（`reason` 不合法、`note` 超过 200 码点 400 / 4000，AC-MOD-03-13；`msgIds` 超过 20 400 / 4131，AC-MOD-03-2）→ 开关（4030 `featureDisabled`，AC-MOD-03-14）→ 每天限额（4292，AC-MOD-03-4，计数口径见第 115 条）→ 对象校验 → 幂等 → 写入。
  2. 对象校验（规则 2、2a、3），失败一律 404 / 4040 `reason: reportTargetInvalid`，**响应逐字节相同**：`message`：举报人是参与者，消息都在同一会话、`seq ≥` 举报人的 `minSeq`（经 `authorize`，C5 的删除会话也算在内，AC-MOD-03-2）；**已撤回的**要 `readSeq ≥ seq`（AC-MOD-03-12 前半）；`user`：好友、或有共同会话、或有共同群（"共同会话"按第 116 条），完全没关联的和不存在的响应相同（AC-MOD-03-7）；`group`：当前成员，非成员和不存在的群响应相同（AC-MOD-03-13）。
  3. 幂等（规则 4）：同一举报人对同一对象 24 小时内重复提交返回第一次的 `reportId`，不产生新记录（AC-MOD-03-3；"同一对象"按第 117 条）。
  4. **证据快照**（规则 5）：在同一事务写 `reports` 和 `report_evidence`（AES-256-GCM，`IM_REPORT_KEY`，复用 `quarantine.Cipher` 的实现，AAD 绑定 `report_id`）。群消息：元数据 + 正文（含已撤回的隔离原文，隔离原文已过期的只有元数据，AC-MOD-03-12 后半）+ 文件引用；单聊：`IM_C2C_BODY_ACCESS=off` 时只存元数据（AC-MOD-03-10），`on` 时同群。`note` 也只在密文里（第 122 条）。用户、群举报的证据内容见第 118 条（要 Frank 定）。证据写失败 500 / 5000，没有记录（AC-MOD-03-14 前半，故障注入 `evidenceWrite`）。
  5. 文件（F-FILE-06 规则 5，AC-MOD-03-15）：证据引用的文件写 `files.evidence_until = 举报时间 + IM_REPORT_EVIDENCE_DAYS`（取较大值），和 `retain_until` 分开；`file_purge` 在两者都过期后才删对象。普通用户取地址仍只看 `retain_until` 和消息状态：原消息正常时照常发放，过了原保留期 404 / 4048，证据仍可看。单聊在 `off` 时不存文件引用，所以不延长（第 95 条 ①）。
  6. `report_purge`（规则 6，单节点定时任务）：证据过期（180 天）物理删除；[IM 钟] 推进 181 天后被删除（AC-MOD-03-6）。`reports` 行本身的保留期见第 119 条（要 Frank 定，定之前和证据一起删）。
  7. 不通知（规则 7）：被举报人、举报人都不收任何帧（AC-MOD-03-1，哨兵法）；举报人身份只在管理接口出现。
  8. 日志和审计（AC-MOD-03-8、AC-CHAT-90-5）：只记 `reportId`、`targetType`、`reason`，不记 `note`、正文、文件名；`im_reports_pending` 由 BE-496 维护。
  9. AC-MOD-03-5、03-11 的审核员读取部分随 BE-496 和 CS-208；本任务验证原始字节搜不到 `ZXQ-RPT-1`（SQLite 检查点后主库和 WAL）。
- 工作量：1 人日（服务端）

### BE-496 举报处理：队列、证据授权、证据读取、结案
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`admin`、`store`、`adminauth`）
- 依赖：BE-495、BE-474
- AC：AC-MOD-03-5、AC-MOD-03-11、AC-MOD-04-1、AC-MOD-04-3、AC-MOD-04-4、AC-MOD-04-5、AC-MOD-04-7、AC-MOD-04-10、AC-MOD-04-11、AC-MOD-04-13、AC-MOD-04-14、AC-MOD-04-16、AC-MOD-04-17
- 内容：
  1. 队列（规则 1，scope `report.read`）：`GET /admin/reports?state=&reason=&from=&to=&cursor=`，按对象聚合（第 117 条）返回次数和最新时间；`GET /admin/reports/{id}` 返回元数据：举报人、被举报人、时间、理由、消息类型和长度、该用户被举报的总次数。**`note` 和文件名按正文对待，不在这两个响应里**（AC-MOD-04-10 第 3 种、AC-MOD-04-11）；单聊举报的元数据要带理由（传法同现有的单聊元数据查询，没有 400 / 4000），每天 50 个的配额在 console（CS-207，和 F-ADM-30 规则 3 合并）。每次 `GET /admin/reports/{id}` 写 im-server 审计。
  2. 证据授权（规则 3，scope `report.evidence`，只能在 `bodyAccess` 调用方 `console-msgread`）：`POST /admin/report-grants {reportId, reason}`，工单号自动用 `reportId`，配额和 `msg-grants` 合并（`grantsPerOperator`、`grantsPerCaller`，`admin/moderation.go` 第 32、33 行），授权绑定调用方 + `X-IM-Operator` + `reportId`；`console` 主凭证调用 403 / 4030（AC-MOD-04-4）。单聊举报在 `IM_C2C_BODY_ACCESS=off` 时 403 / 4030 `c2cBodyDisabled`，元数据照常（AC-MOD-04-3）。
  3. 证据读取：`POST /admin/reports/{id}/evidence {grantId}` 返回解密后的消息（群：正文、隔离原文、文件名、缩略图和原图的引用）和 `note`；只含被举报的消息，不带上下文（规则 2）。**密钥不匹配**（AES-GCM 验证失败）：返回 `{undecryptable: true}`，告警指标加 1，结案不受影响（AC-MOD-04-14 前半）。
  4. 文件（第 11.6 节，AC-MOD-04-13、04-16）：`POST /admin/files/{fileId}/fetch` 增加 `report.evidence` 路径：`grantId` 必须是这个调用方 + 操作人的、没过期的举报授权，且这条举报的证据引用了 `fileId`（`msgId`、`variant` 同 C4），否则 403 / 4039（第 109 条）；单聊 `off` 时 403 / 4030 `c2cBodyDisabled`；`variant` 支持缩略图和原图（审核员两份都看，R-CS-12）；响应头和 C4 相同（`attachment`、`nosniff`），对象按 `evidence_until` 保留。`msg.quarantine` 路径不变。
  5. 结案（规则 5，scope `report.handle`）：`POST /admin/reports/{id}/resolve {result: handled|rejected, reason}`；条件更新 `state='pending'`，第二个 409 / 4098、状态不变（AC-MOD-04-5）；**不自动执行**任何处置，删除、禁言、封禁、下线、解散走各自的接口和审计（AC-MOD-04-1）；举报人和被举报人都不收帧（AC-MOD-04-7，规则 6）。`im_reports_pending`：超过 24 小时未处理的数量（规则 7，第 107 条）。
  6. 元数据接口的文件名（AC-MOD-04-11，同 AC-ADM-30-7）：`msg.meta` 的两层响应都没有文件名，C2 已做，C6 补带特征串的用例。
  7. AC-MOD-04-17：`account_events_purge` C1 已有，C6 只补 [IM 钟] 推进 181 天的用例（TC-MOD-04-17）。
  8. 证据读取和授权的审计只记 `reportId`、授权前 8 位，不记正文、`note`、文件名（AC-CHAT-90-5）。
- 工作量：1 人日（服务端）

### BE-497 冻结文档变更（C6 涉及的部分）
- 优先级：P1
- 批次：C6
- 仓库：tianting-server（`docs/`）
- 依赖：BE-491
- AC：无（文档任务；要作废、加注的 AC 列在内容里，不进这一行）
- 内容（第 16.1、16.2 节 C6 的部分，走变更流程：PRD 升小版本，文末变更记录写原因，同步任务单和用例；做法同 BE-479 的 `40b9a85`、BE-488 的 `43911a8`）：
  1. PRD-admin（v1.4 → v1.5）：F-ADM-60 从 P2 改到第一批（PRD-chat F-MOD），替换模式去掉；**AC-ADM-60-1 作废，并入 AC-MOD-01-1**，TC-ADM-60-01 改关联 AC-MOD-01-1；**AC-ADM-60-2 作废**，TC-ADM-60-02 删除；backend-admin BE-242 标"由 PRD-chat C6 取代"，`- AC：` 行去掉这两条（否则 `trace.py` 报 UNKNOWN）。F-KEY-02 规则 2 scope 目录加 `moderation.words`、`report.read`、`report.handle`、`report.evidence`；规则 3 正文类 scope 加 `report.evidence`；规则 4 `console` 加前三个、`console-msgread` 加 `report.evidence`。第 5.2 节矩阵：**`word.write`（P2，只有审核）换成 `moderation.words`（运营、审核）**，新增 `report.read`、`report.handle`、`report.evidence`（审核，超管默认没有），`moderator` 的职责加"举报处理"（第 14.1 节，第 120 条）。第 12 节：举报证据（含单聊证据）适用门槛的一行（同撤回隔离那一行，第 70 条已改"宿主 App"）。AC-ADM-05-1 的参数化表随矩阵扩展，文字不变。
  2. PRD-social（升小版本）：F-SOC-02 附言在存在性判断之前过敏感词（C6），AC-SOC-02-5 不变，另有 AC-MOD-02-5。
  3. PRD-ops（升小版本）：F-OPS-06 加 `im_blocked_total{scene}`、`im_mod_reload_errors_total`、`im_reports_pending`；F-OPS-07 加词库加载失败告警；F-OPS-10 加 `moderation` 检查项（BE-492 第 4 条）。AC 不变。
  4. PRD-core、PRD-ui：第 16.1 节没有 C6 的 PRD-core 行（F-MSG-01 规则 6 的第 9 步已随 C4 写进说明的话核对即可）；PRD-ui F-UI-09 消息菜单加"举报"（第 13.3 节），资料卡、群信息页的"举报"，新增图标进 6.6 节清单（frontend-chat 第 80 条）。AC 不变。
  5. ARCH-v1：S13、S14 内容安全从"业务方回调"改为内置敏感词和举报（第 16.1 节 ARCH 一行的 C6 部分）。
  6. `docs/sdk/README.md`：`report(...)`、4510 的处理、`features.moderation`（随 FE-690～692）。
  7. `docs/reports/privacy-notice-checklist-draft.md` 增加 C6 的告知项：发送内容会过敏感词检查（只拦截，不告诉命中的词）；命中记录（用户、场景、词、时间，不含全文）保留 90 天；举报证据加密保留 180 天，群消息含正文，单聊在单聊正文查看关闭时只有元数据；被举报人不会收到通知、举报人身份只有审核员可见；第 118、119 条定了之后补。**只是清单草稿，不代替法律文本**。
  8. `tools/trace.py` 已加 C6 里程碑（本次分发）；PRD 升版时改 `PRDS` 文件名。
- 工作量：1 人日（文档）

### BE-498 C6 部署和上线门槛
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`deploy/`、`docs/`）
- 依赖：BE-492、BE-496
- AC：无（部署任务；AC-MOD-01-10、AC-ACC-10-5 的启动检查在 BE-492）
- 内容：
  1. 部署说明：`IM_REPORT_KEY` 的生成（32 字节随机，和 `IM_QUARANTINE_KEY`、JWT 密钥不同）、保管和备份；**第一批不支持轮换**，换密钥后旧证据显示"证据无法解密"（F-MOD-04 异常，第 99 条）；部署示例只放占位值。
  2. 调用方配置：`deploy/im-admin-callers.example.json` 的 `console` 加 `moderation.words`、`report.read`、`report.handle`，`console-msgread` 加 `report.evidence`；`deploy/test_deploy.py` 同步；**已部署环境要手动加**（同 console-chat 第 1 条的教训）。
  3. 词库运维（CH-19）：生产词库只经后台导入（每次 ≤ 5000 条），不进仓库、不进镜像、不进备份以外的地方；词库为空时配置检查告警；词库加载失败的告警接到谁（Frank 指定）。
  4. **上线门槛清单**（写进部署说明，生产打开 `IM_MOD_ENABLED=1` 前逐条确认，第 93 条）：① CH-19 词库负责人、来源、版权、更新已由 Frank 指定，生产词库已导入；② CH-22 举报处理人已指定，并在管理后台授予 `moderator`（敏感角色，授予时通知审计员）；③ PRD-admin 第 12 节 ②③④（隐私政策和用户协议发布、个人信息影响评估、操作规程和培训）完成，**举报证据查看适用**（PRD-chat 第 12.3 节、第 17 节）；④ `IM_PRIVACY_NOTICE_VERSION` 已配置，隐私说明含 BE-497 第 7 条的告知项（AC-ACC-10-5）；⑤ `IM_V13_ADMIN_FEATURES=1`（AC-MOD-01-10）；⑥ `IM_REPORT_KEY` 已生成并保管；⑦ 第 2 条的调用方 scope 已加；⑧ 生产 `IM_CLIENT_CUSTOM_MSG=0`（默认）；⑨ `IM_C2C_BODY_ACCESS` 保持 `off`，打开要 PRD-admin 第 12 节第一行 ①（法务书面确认），单聊举报因此基本只能按次数判断（CH-22、R-10）；⑩ 三端已发布带 `features.moderation` 的版本（第 17 节"SDK 发布后再开"）；⑪ 管理后台第一批的渗透测试（PRD-admin 第 12 节最后一行，P-13）。开放注册 `open` 不在 C6 上线范围：另要 `IM_PUBLIC_OPEN_ACK` 和第 12.3 节的法务手续。
  5. 运维手册：词库加载失败怎么处理（旧词库继续生效，查日志和库）；举报积压指标的含义；`report_purge`、`moderation_hits_purge` 的运行检查。
- 工作量：0.5 人日（部署）

### BE-499 C6 后端验收
- 优先级：P0
- 批次：C6
- 仓库：tianting-server（`server/e2e`）
- 依赖：BE-494、BE-496、CS-206、CS-207、CS-208
- AC：AC-CHAT-90-1、AC-CHAT-90-2、AC-CHAT-90-4、AC-CHAT-90-5、AC-CHAT-90-10、AC-CHAT-90-17、AC-MOD-03-1、AC-MOD-03-5、AC-MOD-04-1
- 内容：
  1. 按 [TC-MOD](../testcases/TC-MOD.md) 落地后端 e2e，`mod_*_test.go`，用例名带 TC 编号；特征串检查（`ZXQWORD`、`ZXQ-RPT-*`、`ZXQ-MOD-*`）单独一个文件，对日志、im-server 审计、console 审计、SQLite 主库和 WAL 做字节搜索。AC-ACC-01-9、AC-ACC-10-5 在 [TC-ACC](../testcases/TC-ACC.md)（TC-ACC-01-22、TC-ACC-10-05）。
  2. 【全部】的 C6 部分（第 15 节累计表）：AC-CHAT-90-1 开词库（TC-CHAT-90-40，用 10 万随机词，开发机只记录）；90-2 C6 开关关（TC-CHAT-90-41）；90-5 举报、附言（TC-CHAT-90-43）；90-10 `report`（TC-CHAT-90-46）；90-17 治理指标（TC-CHAT-90-47）；90-4（TC-CHAT-90-42）。90-3 在 C6 不变，90-15 是"—"，不另跑。AC-KEY-02-2 的参数化表随 scope 目录扩展复跑。
  3. 端到端 AC 的服务端部分：AC-MOD-03-1（举报后后台队列出现，u1 收不到帧）、AC-MOD-04-1（审核员走完查看、授权、读证据、删除、结案，审计顺序）。客户端和后台页面部分随 FE-698。
  4. 写验收报告 `docs/reports/c6-backend-acceptance-<日期>.md`：写明本地结果、CI 不运行；性能未判定（P-12）；标准化的语料误判结果（BE-493 第 3 条）；上线门槛清单（BE-498 第 4 条）的状态；第 104、118、119 条等待决定时按什么实现。
- 工作量：2.5 人日（测试）

---

## PRD 问题的处理

分发时发现的矛盾和缺口已在 [PRD-chat v1.1](../prd/PRD-chat-v1.1.md) 第 22 节修正。实现 C1 时发现的（PRD 冻结，没有改，都按最保守的理解实现，等 Frank 决定）：

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 1 | AC-ACC-01-5 | 例子 `bob` 只有 3 位，走不到 4095 | 用例改用 `bobby` |
| 2 | F-ACC-03 规则 4 第 ② 步 | `mac` 正确但会话行不存在（`session_purge` 或注销清理删了）没有写 reason | `reason=expired`，不带 `recoverBefore` |
| 3 | F-ACC-03 规则 4 | `pending` 的 access 未过期，但 refresh 到期时间已过（H5 24 小时），写的是"转正"，转正又会滑动到期时间，等于用 access 复活过期会话 | 不转正，按过期 4011 |
| 4 | F-ACC-04 规则 3、6，F-ACC-08 规则 2 | 会话已被吊销后再次吊销，原因是否覆盖没写。`logout-others` 后改密、注销后找回时，旧 refresh 的 `reason` 是第一次的（规则 6a 的日期不受影响） | 保留第一次的原因 |
| 5 | F-ACC-05 规则 2 | "每 IP 每分钟 20 次"和登录的是否同一个计数没写 | 分开计数，登录失败压不到找回 |
| 6 | F-ACC-08 规则 2 | `delete/cancel` 密码正确但没有注销申请时的响应没写 | 200，只有知道密码的人走得到 |
| 7 | F-ACC-03 规则 4 | `securityNotice` 写"其他在线设备"，同一 `sid` 被踢的连接可能先收到一条 | 没有排除，客户端忽略不了也无害 |
| 8 | 第 11.2 节 | `recovery-code` 响应、`PATCH profile` 响应没有定义 | `{{recoveryCode, recoveryCodeGen}}`（Schema 补 `RecoveryCodeResponse`）、`{{}}` |
| 9 | 第 11.5 节 | 4048 排在 C2 的协议任务，但 AC 要求 C1 的 `avatarFileId` 返回它 | 提前加进 `errors.json` |
| 10 | F-ACC-03 规则 2 | "最近活跃时间"只说显示，没说什么算活跃 | 登录、刷新、转正时更新，连接和收发不更新 |

实现 C2 时发现的（PRD 冻结，没有改；按下面的理解实现）。**Frank 2026-11-09 回复"按现在的实现"，第 11～23 条都按"实现"一列确认**（PENDING P-36）；已写进 [PRD-chat v1.3](../prd/PRD-chat-v1.3.md)（第 11、12、15、20 条不需要改 PRD）：

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 11 | 任务单 BE-401、BE-404、BE-431、BE-436 | 引用的"文末问题 4、8、9、10"在这份任务单里不存在（文末只有 C1 的 10 条） | 按引用处的说明实现 |
| 12 | 第 4.5 节、AC-FILE-01-8 | 假 S3 要求"独立进程"，任务要求不在开发机安装和运行额外软件 | 假 S3 在测试进程里、监听独立端口；AC-FILE-01-8 验的是字节只进对象存储的端口、im-server 收到的请求体 ≤ 64 KB |
| 13 | AC-FILE-01-15 第二种 | "嗅探之后、Copy 之前替换"：如果替换后的内容重试时嗅探不通过（例如换成 HTML），按规则 3 ② 是 415 并删除，和 AC 写的"4000、临时对象不删"冲突 | 替换内容能通过嗅探时 4000 `uploadMismatch`、不删；通不过时 415、删除。用例用能通过嗅探的替换内容 |
| 14 | F-FILE-06 规则 2、3、4 | `expired`、`revoked` 之后写"purging → 物理删除"，但消息要继续显示"已过期"和占位尺寸 | 对象按前缀物理删除，`files` 行保留（状态 expired / revoked、文件名已清空、`objects_deleted=1`）；行一直不删 |
| 15 | AC-ACC-01-20 | "申请超过 200 MiB 的当日上传"，但单个文件上限 100 MiB，只能靠已用量越过 | 用例把 `IM_FILE_MAX_BYTES` 调到 300 MiB，一次申请 201 MiB |
| 16 | F-FILE-04 规则 6 | "按用户统计取地址量进指标"和"不进标签"（第 14.2 节、AC-OPS-90-4） | 只有不带标签的 `im_file_url_issued_total`，没有按用户的统计 |
| 17 | F-FILE-01 规则 6 | "按实际大小扣"没说缩略图算不算 | 原图 + 缩略图都算（申请时的声明也一样） |
| 18 | F-FILE-01 规则 3、规则 6a | 租约用哪个时钟没写；AC-FILE-01-27 是 [IM 钟]，AC-FILE-01-16 的 PUT 地址是 [真实钟] | 租约、24 小时清理、额度日期用 [IM 钟]；PUT 地址过期和未完成数用真实时钟 |
| 19 | F-ACC-06 规则 1、异常 | 一次 `PATCH` 同时带昵称和头像、头像失败时的响应没写 | 昵称先保存，响应是头像的错误（5032 / 4048） |
| 20 | 第 4.3 节第 10 步、R-CB-20 | 第 6～10 步失败退还 `newPeerReject`，第 10 步包括 4137，现有实现 4137 不退 | 按 PRD 退还（含 4137）；现有 e2e 全过 |
| 21 | 第 11.3 节、BE-430 第 3 条 | 对象存储的环境变量名、禁止名单配置文件的变量名 PRD 没有定义 | `IM_OBJSTORE_ENDPOINT/REGION/BUCKET/PUBLIC_BUCKET/PUBLIC_BASE_URL/ACCESS_KEY/SECRET_KEY`、`IM_FILE_BLOCKLIST`（只能加不能减）；生产要求 https 和公开桶独立域名 |
| 22 | F-FILE-01 规则 3b | GIF 帧数上限"未校准"没有数值 | 前 64 KB 内超过 300 帧拒绝 |
| 23 | F-FILE-01 规则 3 ② | "与声明矛盾"对图片的判断粒度没写 | 图片、缩略图、头像的嗅探类型必须和声明完全相同（`image/jpg` 按 `image/jpeg`）；文件只拒绝禁止的真实类型，认不出的按 octet-stream |

C2 部署和验收（BE-438～440）时发现的（PRD 冻结，没有改；按"实现"一列做）。**需要 Frank 定的是第 28 条**，其余是记录：

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 24 | 第 14.2 节告警、F-FILE-01 规则 6 | "当天全局上传量（含临时前缀）"：申请后不 `complete` 的对象只在 `tmp/`，im-server 看不到 | im-server 只有 `im_file_bytes_total`（complete 成功的字节）；含临时前缀的量要用对象存储自己的用量指标，部署说明写明，选定厂商后补 |
| 25 | 第 14.2 节 | 告警要"对象存储错误率"，指标只列了 `im_objstore_errors_total`，没有分母 | 加 `im_objstore_requests_total{op}`，两个都带有界的 `op` 标签（head / get / list / put / copy / delete）；404、412 不算错误 |
| 26 | 第 14.2 节 | `im_file_uploads_total{kind,result}` 的 `result` 取值没写 | 按错误码归类：ok / mismatch / type_denied / not_ready / quota / unavailable / gone / error；只统计拿到租约的 complete 尝试 |
| 27 | F-FILE-06 规则 8 | "多余对象按前缀对账，手动运行"没有要求工具 | 第一批只在部署说明写步骤（全桶列举用单独凭证，宽限 1 小时），没有工具 |
| 28 | 第 11.3 节、F-FILE-01 规则 3 | 服务端凭证没有全桶列举权限。AWS 风格的存储对这种凭证 HEAD 不存在的键可能返回 403 而不是 404；实现把 404 当"临时对象不存在"（4096），403 会变成 500 | 没有改；MinIO 上这份权限能列 `tmp/`，没有这个问题。**要 Frank 在 AC-FILE-90-2 选定厂商时确认**：若是 403，改判断（403 也按不存在）或给 `tmp/` 列举权限 |
| 29 | 第 4.1 节 | 对象存储单次请求超时没有配置项 | 代码默认 30 秒；黑洞时 `complete` 最长等 30 秒 503，文本不受影响（TC-CHAT-90-03） |
| 30 | AC-CHAT-90-3 | "文本收发 5 分钟" | 开发机每种方式顺序 1000 条（约 1 秒），5 分钟随 P-12 |
| 31 | TC-CHAT-90-05 | 环境写"ES + console-server"，C2 的 console 日志和审计（CS-201） | 只扫了 im-server 的日志和库；console-server 部分随 CS-201 / FE-640 |

BE-403 共享向量时发现的（PRD 冻结，没有改；按"实现"一列做）。**需要 Frank 定的是第 32 条**，第 33 条是记录。冷启动转失败的原因各端不同，见 frontend-chat.md D4，向量不比较原因：

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 32 | F-RCL-03 规则 2 | 重连时有几个已加载的会话都要重拉，其中一个 `sync` 失败，其余会话这一次还拉不拉没写 | 三套 SDK 一致：按顺序重拉，第一个失败就停，这一轮其余的不拉；失败的会话不记新的 `modVer`，下次重连再拉（`chat-sync.json` 只断言"失败的下次再拉"，出错的用例里只有一个已加载的会话）。~~要 Frank 定~~ **Frank 2026-11-10 已定（P-39 ②）：失败的跳过、其余照拉**——每个要重拉的会话独立处理（仍按顺序串行），失败的不记新 `modVer`、下次重连再拉，其余这一轮照常重拉并记新 `modVer`。已处理：chatmodel 和 `chat-sync.json`（新增"多个会话要重拉、中间一个失败"，`TestChatModelRules` 补断言）`e71a977`；sdk-ts `f417580`、sdk-kotlin `2819ce6`、sdk-swift `c8ec6ff`（协议副本同步到 `e71a977`，各补一个单测；本地结果） |
| 33 | F-FILE-07 规则 4 | "别人的 `fileId` 收到 4048 不重传"：SDK 现在没有转发入口，正常流程造不出带别人 `fileId` 的消息 | 向量的 `foreignFileId` 起点由各端测试直接写进本地仓库（sdk-ts、sdk-swift 写内存，sdk-kotlin 写本地库）；有转发后改走公开接口 |

C3 开工时按 PRD 第 21 节第 2 条核对官方文档（2026-11-10，**只看 Apple、Google 官方页面**；Apple 文档页面是前端渲染，读的是同一路径的官方数据接口 `developer.apple.com/tutorials/data/documentation/usernotifications/<页面>.json`）。PRD 冻结，没有改；**要 Frank 定的是第 34、36、37、39 条**（PENDING P-40），其余是缺口或确认，按"建议"一列实现，Frank 不同意再改。任务里写的实现都按"建议"一列。

| # | 位置 | PRD 原文 | 官方文档 | 来源 | 建议 |
|---|---|---|---|---|---|
| 34 | F-APN-01 规则 3、AC-APN-01-2 | "通道返回令牌失效（APNs 410 / `BadDeviceToken`…）：删除这条令牌" | 410 的 reason 是 `Unregistered`（令牌对这个 topic 不再有效）和 `ExpiredToken`（令牌已过期），410 的响应体带 `timestamp`（APNs 确认失效的毫秒时间）；Apple 说 `Unregistered` 之后不用再推，"除非 App 重新拿到同一个令牌"。**`BadDeviceToken` 不是 410，是 400**，含义是令牌无效**或和环境（sandbox / production）不匹配** | [handling-notification-responses-from-apns](https://developer.apple.com/documentation/usernotifications/handling-notification-responses-from-apns) | 410（两种 reason）删除，但 **`timestamp` 早于这条令牌最近一次注册时间时不删**（设备又注册了同一个令牌）；400 `BadDeviceToken` 也删除（Apple 列为不要重试），按 reason 计数，配置检查显示最近的 `BadDeviceToken` 数量，用来发现 `env` 配错。PRD 下一版把"410 / `BadDeviceToken`"改成"410（`Unregistered`、`ExpiredToken`）/ 400 `BadDeviceToken`"。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 35 | F-APN-01 规则 3、F-APN-02 规则 8 | 只列了令牌失效和"可重试的错误（429、5xx）" | 400 还有 `DeviceTokenNotForTopic`、`BadTopic`、`TopicDisallowed`、`BadCollapseId`、`PayloadTooLarge`（413）等；Apple 明确不要重试 `BadDeviceToken`、`DeviceTokenNotForTopic`、`Forbidden`、`ExpiredToken`、`Unregistered`、`PayloadTooLarge` | 同上 | 缺口，不矛盾。`DeviceTokenNotForTopic`、`BadTopic`、`TopicDisallowed` 多半是 `IM_APNS_TOPIC` 配错，**不删令牌**（否则一次配错删光全部令牌），计 `bad_request` 并在配置检查显示；其余 4xx 不重试。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 36 | F-APN-02 规则 8、AC-APN-02-9 | "可重试的错误（429、5xx）首次调用外最多重试 2 次"，没写间隔；AC 例子"503 一次后成功 → 共 2 次" | APNs：5xx "15 分钟后"可以重试，可以退避；`TooManyRequests`（429）可以延迟后重试；另一个 429 `TooManyProviderTokenUpdates` 是服务端 JWT 换得太勤，原样重试没用。FCM：429 按 `Retry-After` 等待（没有时默认 60 秒），`QUOTA_EXCEEDED` 指数退避且首次至少 1 分钟；503 按 `Retry-After`；500 指数退避；请求超时至少设 10 秒；400、401、403、404 不重试；连续失败 60 分钟放弃 | Apple 同上；[FCM ErrorCode](https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode)、[scale-fcm 的 Handling retries](https://firebase.google.com/docs/cloud-messaging/scale-fcm) | **要 Frank 定**。建议：次数不变（AC-APN-02-9 不变），间隔按官方：APNs 5xx 15 分钟、30 分钟；APNs 429 和 FCM 按 `Retry-After`，没有时 60 秒起翻倍；超过通知过期时间（隐藏 24 小时、详情 1 小时）就放弃。重试放延迟队列，不占 worker，测试用 [IM 钟]。代价：详情模式的第 2 次重试接近 1 小时过期。另一个选项是保持 PRD 的立即重试（几秒内），实现简单，但违背 Apple 对 5xx 的建议。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 37 | F-APN-05 异常、AC-APN-05-5 | "凭证过期或被吊销（通道返回 401 / 403）：该通道的调用全部计失败" | APNs 不返回 401；403 里 `ExpiredProviderToken` 是 **JWT 过期，要重新生成**（不是凭证被吊销），`InvalidProviderToken`、`MissingProviderToken`、`BadEnvironmentKeyIdInToken`、`BadCertificate` 才是凭证或配置问题，`UnrelatedKeyIdInToken` 要换新连接。FCM 401 是 `THIRD_PARTY_AUTH_ERROR`（APNs 证书或 Web Push 密钥无效，iOS 不走 FCM 时正常不出现）；403 是 `SENDER_ID_MISMATCH`（令牌属于另一个发送方，是单个令牌的问题）。OAuth access token 过期时 FCM 返回什么，ErrorCode 页没有写（**未核实**） | Apple 同上；[FCM ErrorCode](https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode) | **要 Frank 定**。建议：APNs 403 `ExpiredProviderToken` 先重签 JWT 重试 1 次，不计凭证失败；FCM 401 先换 access token 重试 1 次；其余 401、403 计凭证失败（F-OPS-07）；`SENDER_ID_MISMATCH` 计失败、不删令牌（Google 只说 `UNREGISTERED`、`INVALID_ARGUMENT` 可以删）。AC-APN-05-5 的"假通道返回 403 → 计失败"不变。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 38 | F-APN-05 规则 1、AC-APN-05-4 | 只写"token 认证（`.p8`）""校验 ES256 JWT" | JWT 头 `alg=ES256`、`kid`（10 字符 Key ID），声明 `iss`（10 字符 Team ID）、`iat`；**刷新不早于 20 分钟、不晚于 60 分钟**；`iat` 超过 1 小时被拒（403 `ExpiredProviderToken`）；同一连接上换得太勤返回 429 `TooManyProviderTokenUpdates` | [establishing-a-token-based-connection-to-apns](https://developer.apple.com/documentation/usernotifications/establishing-a-token-based-connection-to-apns)；错误码同第 34 条 | 缺口。JWT 缓存，每 40～50 分钟重签；假 APNs 校验这两条（BE-406）。Key 轮换仍是改配置后重启（F-APN-05 异常）。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 39 | F-APN-01 规则 3、AC-APN-01-2 | FCM 只写 `UNREGISTERED` 删除令牌 | `UNREGISTERED`（404）：从服务端删除这个令牌；**`INVALID_ARGUMENT`（400）在消息本身完全合法时也表示令牌无效，可以删除** | [manage-tokens](https://firebase.google.com/docs/cloud-messaging/manage-tokens)、[FCM ErrorCode](https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode) | **要 Frank 定**。建议：请求体由服务端固定模板生成、发送前自检，`INVALID_ARGUMENT` 时删除令牌并计数（批量出现就是模板错了，靠指标发现）。保守的选项是只计失败不删，代价是无效令牌一直占调用。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 40 | F-APN-02 规则 6 | "FCM 不用 `collapse_key`（官方文档：每设备只保留 4 个，通知消息还会忽略它）；Android 用通知 `tag`（未核实）" | 一致：每设备最多存 4 个不同 collapse key 的消息；通知消息"总是可折叠，会忽略 `collapse_key`"。`AndroidNotification.tag`：同 tag 的通知已显示时，新通知替换它；不设则每次新建一条。App 自己显示时，`NotificationManager.notify(tag, id, …)` 用 (tag, id) 这一对替换 | [collapsible-message-types](https://firebase.google.com/docs/cloud-messaging/customize-messages/collapsible-message-types)、[projects.messages](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages)、[NotificationManager](https://developer.android.com/reference/android/app/NotificationManager) | 确认，不用改规则。PRD 下一版去掉"未核实"。`tag` 只对 FCM 代为显示的通知消息生效（App 在后台）；App 在前台时设备本来不推（F-APN-02 规则 2）。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 41 | F-APN-05 规则 4 | "im-server 要能访问外网：`api.push.apple.com`、`fcm.googleapis.com`、`oauth2.googleapis.com`" | APNs 开发环境是 `api.sandbox.push.apple.com`，生产 `api.push.apple.com`，端口 443（也可用 2197），HTTP/2 + TLS 1.2 以上。FCM 发送端点 `fcm.googleapis.com`。OAuth token 端点的主机官方页面没有直接写（**未核实**，服务账号 JSON 里有 `token_uri`） | [sending-notification-requests-to-apns](https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns)、[projects.messages.send](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages/send) | 缺一个主机。出站白名单加 `api.sandbox.push.apple.com`（`env=sandbox` 的令牌要用）；OAuth 端点从凭证文件的 `token_uri` 读，不写死（BE-457、BE-458）。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 42 | F-APN-02 规则 6 | 详情模式 collapse id "`HMAC(服务端密钥, sid + convId)` 截断"，没写长度 | `apns-collapse-id` 最多 64 字节，超出返回 400 `BadCollapseId` | 同第 41 条 | 缺口。固定 32 个十六进制字符（HMAC-SHA256 取前 128 位）；隐藏模式的每设备 id 同样长度，外观相同。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 43 | F-APN-02、F-APN-04 规则 3 | 没写 Android 用通知消息还是数据消息，也没写优先级 | 通知消息默认 HIGH，数据消息默认 NORMAL，"NORMAL 不会在设备休眠时打开网络连接"；`ttl` 最长 4 周，格式是秒数（如 `"3600s"`）；消息 4096 字节以内 | [projects.messages](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages)、[setting-message-priority](https://firebase.google.com/docs/cloud-messaging/customize-messages/setting-message-priority)、[FCM ErrorCode](https://firebase.google.com/docs/reference/fcm/rest/v1/ErrorCode) | 缺口。Android 用**通知消息**（带 `notification.tag`、`channel_id`"消息"），显式 `priority: HIGH`，`ttl` 用隐藏 `86400s`、详情 `3600s`；摘要最多 50 个码点，远小于 4096 字节。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 44 | AC-APN-05-4 | "假 FCM 校验 OAuth" | 发送接口要 OAuth scope `https://www.googleapis.com/auth/firebase.messaging` 或 `cloud-platform` 二选一，`Authorization: Bearer <access_token>` | [projects.messages.send](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages/send)、[send/v1-api](https://firebase.google.com/docs/cloud-messaging/send/v1-api) | 缺口。只申请 `firebase.messaging`（最小权限）；假 FCM 校验 Bearer 和签发时的 scope。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 45 | F-APN-05 规则 1、AC-APN-05-4、AC-APN-02-9 | 断言 `apns-topic`、`apns-push-type`、`apns-expiration`，没写取值 | `apns-topic` 必需（bundle id）；`apns-push-type` 在 iOS 上是"建议"（watchOS 必需），可见通知用 `alert`；`apns-priority` 默认 10（立即），5 按功耗；`apns-expiration` 是 UTC epoch 秒，0 表示只投一次不存储，存储最多 30 天；正文 JSON 最大 4096 字节 | 同第 41 条 | 缺口。`apns-push-type: alert`、`apns-priority: 10`、`apns-expiration` = 发送时间 + 24 小时（隐藏）/ 1 小时（详情）的 epoch 秒；AC-APN-02-9 按这个口径断言。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |
| 46 | F-APN-01 规则 5、5a | Android 13 以上要 `POST_NOTIFICATIONS`；每次回到前台对比授权和令牌 | 一致：Android 13（API 33）起是运行时权限，新安装的 App 默认关闭，被拒后系统不会再自动弹。FCM 建议每月刷新一次令牌，安卓令牌 270 天不活跃会失效（iOS 走 APNs，没有这个规则） | [notification-permission](https://developer.android.com/develop/ui/views/notifications/notification-permission)、[manage-tokens](https://firebase.google.com/docs/cloud-messaging/manage-tokens) | 确认。小补充：回到前台时如果距上次注册超过 30 天，即使令牌没变也重新注册一次（在每小时 10 次的限额内），让服务端知道令牌还活着；不改 AC。**2026-11-10 已写进 PRD-chat v1.5**（P-40，Frank"都按建议"） |

没有查到官方说法、仍是"未核实"的：Apple Developer 费用和 App 内注销条款（第 21 节第 2 条前半，和推送实现无关，没有查）；FCM access token 的有效期；OAuth token 端点主机；中国大陆到 FCM 的连通性（第 21 节第 5 条，要部署地实测）；iOS 进入后台时 `presence` 那一帧能否发出（第 21 节第 6 条，FE-651、FE-654 实现时记录）。

C3 第一波实现时发现的（PRD 冻结，没有改；C3 已暂停，**都没有落地**，恢复时按"建议"一列做，Frank 不同意再改）。**需要 Frank 定的是第 47、48 条**：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 47 | 第 12.1 节、BE-451 第 2 条 | 令牌字段级加密和详情模式 collapse id 的"服务端密钥"没有定义变量名；"复用现有字段级加密的做法"没说复用哪把密钥。另外 F-APN-05 规则 2 的"项目 ID"也没有变量名 | 新增 `IM_PUSH_TOKEN_KEY`（≥ 32 字节，开关打开时生产必填，和其他密钥不同），令牌 AES-256-GCM（AAD = 令牌哈希）和 collapse id 的 HMAC 用它按用途派生，不复用 `IM_QUARANTINE_KEY`；项目 ID 默认取服务账号 JSON 的 `project_id`，`IM_FCM_PROJECT_ID` 可以覆盖。**要 Frank 定**变量名 |
| 48 | F-APN-01 规则 1、4 | 没有 `sid` 的令牌（演示、管理接口 token）在强制下线、封禁、改密、找回、注销时删不删没写（不变量只说 `sid`）；无 `sid` 又没有令牌时 `deviceNotificationSettings` 无处保存 | 这些操作同时让无 `sid` 的旧 token 失效（版本号、`credentials_changed_at`），所以在吊销全部会话的同一事务里把这个用户的令牌（含无 `sid` 的）全删；退出、refresh 重放只删那条 `sid` 的，`sessions revoke-all` 只删有 `sid` 的。无 `sid` 无令牌时 ack 返回 `false`、不保存。**要 Frank 定** |
| 49 | F-APN-04 规则 3、v1.5 规则 9 | `channel_id`"消息"：Android 渠道 ID 和显示名是两个东西，PRD 只写了名字 | 服务端发 `channel_id: "messages"`，App 建渠道时 ID 用 `messages`、显示名"消息"（和 FE-652 对齐，不需要 Frank 定） |
| 50 | 任务单 BE-451 第 1 条、BE-459 第 1 条 | `store.PushStore`、`push_*_test.go` 这两个名字已被系统推送（PRD-live）占用（`store.PushStore` 是收件箱，`e2e/push_test.go` 已存在） | 存储接口叫 `store.DevicePushStore`，e2e 文件用 `apn_*_test.go`（记录，不需要 Frank 定） |

C4 分发时发现的（2026-11-10，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。**要 Frank 定的是第 61 条**（PENDING P-43 ②），其余是缺口或记录：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 51 | 第 11.1 节 | "`GroupMembersReq` 增加可选 `keyword`"：现有协议里 `GroupMembersReq` 是加成员的请求（`protocol/social.go` 第 298 行），分页查成员的是 `GroupMembersPageReq`（`logic/group.go` 第 500 行） | `keyword` 加在 `GroupMembersPageReq`（BE-470 第 2 条）；PRD 下一版改名字 |
| 52 | 第 12.1 节、BE-451 | C3 计划的 `0018_device_push.sql` 没有提交（C3 暂停），C4 先做 | C4 用 `0018_msg_actions.sql`；C3 恢复时 BE-451 用下一个空号，BE-451 第 1 条和"C3 暂停时的状态"里的 `0018` 到时一起改 |
| 53 | 第 4.1 节、第 12.1 节 | 撤回每人每分钟 20 次、成员搜索每人每分钟 30 次没有变量名；@ 所有人"次数单独计数，不靠这张表"没有给存储 | `IM_RECALL_RATE_PER_MIN`、`IM_MEMBER_SEARCH_RATE_PER_MIN`；计数表 `mention_all_quota`，和消息同一事务（BE-471、BE-476） |
| 54 | AC-RCL-01-10、第 4.1 节 | "开启但缺 `IM_QUARANTINE_KEY` → 启动失败"，现有配置在非生产环境缺这把密钥时回退到开发默认值（`config.go` 第 249～252 行），按现状不会失败 | 开关打开时不回退，任何环境都启动失败；测试显式设置密钥（BE-471 第 3 条） |
| 55 | F-RCL-01 规则 3、4 | 校验顺序里没有写频率限制（4290）在哪一步，网络超时后的幂等重试算不算次数 | 格式之后、查库之前；重试也计数（每分钟 20 次够重试用） |
| 56 | F-RCL-01 规则 2、3 | `tip` 是 400 / 4000，但规则 3 的"格式（4000）"在最前面，判断 `tip` 要先查到消息 | `tip` 在"消息存在"之后判断；AC-RCL-01-3 每个输入单独请求，结果不变 |
| 57 | F-RCL-02 规则 1、AC-RCL-02-7 | "操作者撤回自己的消息时取更宽的时限"，AC 只测了群主，管理员撤回自己的消息没有单独写 | 按规则字面：群主、管理员撤回自己的都是 7 天；TC-RCL-02-07 加管理员一行 |
| 58 | F-MEN-02 规则 2 | "每群每小时 3 次"是整点小时还是滚动 60 分钟没写，AC-MEN-02-2 两种都满足 | 滚动 60 分钟（避免整点前后连发 6 次）；每天按 Asia/Shanghai 自然日 |
| 59 | F-MEN-01 规则 6、第 4.1 节共同规则 | `IM_MSG_ACTIONS_ENABLED` 关闭时 `groupMembers` 的 `keyword` 能不能用没写 | 能用（只读，不产生提及） |
| 60 | F-MEN-03 规则 1、第 12.1 节 | `conv_mentions`"清理到已读之后"，但 `sync` 还要下发 `mentions`；@ 所有人"含"在 `mentionUnread` 里，发的人自己算不算没写 | `mentions` 存在消息 `body` 里（撤回时随正文清除），`conv_mentions` 只用于计数；自己发的 @ 所有人不计入自己的 `mentionUnread`（规则 6 的延伸） |
| 61 | AC-APN-03-3、AC-RCL-03-9 | 两条标【C4】，但要 C3 的推送 worker 和假通道（BE-455、BE-456、BE-406），C3 已暂停 | 放进 BE-481，C3 恢复后做，C4 其余任务不依赖 C3 代码；C4 验收时记"未执行（等 C3）"。**这两条 P0 未执行时 C4 能不能验收，要 Frank 定**（P-43 ②） |
| 62 | 第 11.6 节、console 问题 13 | `POST /admin/files/{fileId}/fetch` 的请求、响应形状没写 | BE-470 第 4 条：请求 `{convId, msgId, variant?}`，响应文件字节、`attachment`、`nosniff`，不发预签名地址给 console |

C4 后端第一波实现时发现的（2026-11-10，PRD 冻结，没有改；按"实现"一列做，Frank 不同意再改）。**需要 Frank 定的是第 67 条**（已由协调方确认并完成），其余是缺口或记录；第 68～71 条是 BE-479 第 1、2 条落实时记的，第 70 条要协调方定：

| # | 位置 | 问题 | 现在的实现 |
|---|---|---|---|
| 63 | AC-RCL-02-1、TC-RCL-02-01 | 5 个操作用同一组消息：第 1 步 o 撤回了 a 的消息之后，第 5 步"m 撤回 a 的"按 F-RCL-01 规则 3 的顺序（已撤回在权限之前）是幂等成功，不是 AC 写的 4030 | 按规则 3 的顺序实现（撤回状态对所有参与者已经可见，幂等成功不泄露什么）；用例里 a 多发一条，第 5 步撤回 a 没被撤回的那条，另断言撤回已撤回的是幂等成功。不改 AC，TC-RCL-02-01 下次修订时写明"a 的另一条" |
| 64 | 第 12.1 节、文末第 53 条 | 计数表建议的列名 `window` 在 SQLite / Postgres 里是保留字（窗口函数） | 列名用 `win`（`mention_all_quota(group_id, win, bucket, count)`），其余同第 53 条 |
| 65 | F-RCL-01 规则 3 | `historyVisible=false` 的群里，后加入的群主、管理员能不能撤回自己入群前（看不到）的消息没写 | 按"消息不存在"返回 4044（和其他会话的 msgId 一样）：看不到的消息不能操作，也不透露它存在 |
| 66 | 第 11.6 节、BE-470 第 4 条 | `POST /admin/files/{fileId}/fetch` 的请求形状 `{convId, msgId, variant?}` 里没有 `grantId`，"有这个会话的有效隔离授权"要按什么找没写 | 按调用方 + 签名的 `X-IM-Operator` + `convId` + `quarantine=true` + 未过期查找（`store.HasActiveMsgGrant`），和正文读取的授权校验同一组条件；`fileId` 不属于 `msgId`、消息没撤回也没被后台删除、对象已删除都是 404 / 4048，不区分。2026-11-10 P-44 ④ 已落实（tianting-server@d17df86，console-server@4b6b9e2，本地结果）：fetch 请求体加可选 `grantId`，带上时只认这一个授权（调用方 + 操作人 + 会话 + 隔离 + 未过期），不带时不变；console 带这个浏览器会话的 `grantId`（PRD-chat v1.6 第 11.6 节） |
| 67 | BE-479 第 1、2 条 | BE-479 要改 PRD-ui（AC-UI-09-1 改写、升版）、PRD-core / social / admin / ops 的 C4 部分，但这一波的分发要求"不改 PRD 和 AC" | **没有做第 1、2 条**，只做了第 4 条（隐私清单草稿）和第 5 条的核对（trace.py 已有 C4）；第 3 条随 FE-660～664。**要 Frank 定**：BE-479 第 1、2 条（PRD 升小版本、改写 AC-UI-09-1、AC-ADM-31-6 的关联说明）什么时候做、由谁做。2026-11-10 协调方确认按 PRD-chat 第 16 节落实，已完成（`40b9a85`） |
| 68 | BE-479 第 2 条、PRD-ui F-UI-04 | BE-479 写"F-UI-04 规则 9（摘要前缀优先级）"，PRD-chat 第 16.1 节写"F-UI-04 规则 5、9 → 第 13.3 节、F-MEN-03 规则 2"。摘要前缀属于规则 5（摘要规则），规则 9 是会话长按菜单、置顶、免打扰（C5） | 写在 PRD-ui v1.4 F-UI-04 的"PRD-chat v1.5 变更（C4）"说明里，按规则 5 的扩展写（撤回摘要、"[有人@我]"和前缀优先级）；规则 9 留给 C5 |
| 69 | BE-479 第 2 条、PRD-ops F-OPS-10 | PRD-chat 第 14.2 节的配置检查一行没有列撤回的检查项；BE-479、BE-471 有 `msg_actions`（开关状态、隔离密钥已设置、隐私说明版本） | 按 BE-471 的实现写进 PRD-ops v1.5 F-OPS-10 的说明，依据写 PRD-chat 第 4.1 节（开启要求 `IM_QUARANTINE_KEY`）和 F-ACC-10 规则 4；PRD-chat 第 14.2 节已冻结，没有改 |
| 70 | PRD-chat 第 16.1 节、PRD-admin 第 12 节 | 第 16.1 节这一行有两件事："宿主 App"改为天听；门槛适用撤回隔离和举报证据。BE-479 只列"门槛适用撤回隔离"，"宿主 App 改为天听"不属于某个里程碑，BE-408 和 BE-479 都没列 | PRD-admin v1.4 第 12 节只加了"撤回原文的隔离"一行（门槛同第二批那一行，C4 在 ②③④ 完成前不能在生产打开，CH-17）；"宿主 App（fx-sport / ab77 / android）"的原文没改。**要协调方定**：随 C6（举报证据）一起改，还是单独改。**2026-11-10 协调方定**：第 16.1 节已写定，不是新决定，现在改进 PRD-admin v1.4 第 12 节（"宿主 App"改为天听，第 4 问加注按天听重新看）；举报证据仍随 C6 |
| 71 | BE-479 第 2 条、PRD-core | BE-479 列的 PRD-core 部分没有 F-MSG-05 规则 7 和 F-CONV-01 规则 1 的 `lastMsg.recalled`，但第 16.1 节的 PRD-core 行（"F-MSG-01 规则 6、F-MSG-05 规则 7…→ 第 4.3、11.1 节"、"F-CONV-01 规则 1、2、3 → `convs` 增加字段"）包括它们 | 按 PRD-chat F-RCL-01 规则 5a、F-RCL-03 规则 2、2a、4 和第 11.1 节的原文在 PRD-core v1.6 加了说明（撤回的消息同时 `deleted=true`、`recalled=true`；`lastMsg` 增加 `recalled`、`recalledBy`），AC 不变。F-MSG-01 规则 6 同时写了第 4.3 节第 1 步的结构校验和第 10 步的 @ 所有人计数（同是 C4 的部分） |

C5 分发时发现的（2026-11-10，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。**要 Frank 定的是第 72、81、82、86 条**；第 77、88 条是"PRD 别处已写明"，按原文实现，不算新决定；其余是缺口或记录：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 72 | AC-CVS-02-1、02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4 | 6 条标【C5】，预期都是"假通道 / 假 APNs 有没有调用、`badge` 是多少"，要 C3 的推送 worker 和假通道（BE-406、BE-455、BE-456），C3 已暂停。main 上没有 `push` 包和假通道（只有不能直接用的本机草稿分支），C1、C2 的设施（假 S3、故障注入、双节点）里没有推送通道，哨兵法的哨兵也调不到 | 放进 BE-490，C3 恢复后做，C5 其余任务不依赖 C3 代码；能先验的部分在 C5（`unread`、`mentionUnread` 增加，角标聚合函数单测，失效广播的发布端）。AC-CVS-02-6 留在 C5（只是客户端前台行为）。**这 6 条 P0 未执行时 C5 能不能验收，要 Frank 定**：建议同 P-43 ②（记"未执行（等 C3）"、不算失败，C5 先内部验收，C3 恢复后补跑再关闭）。**2026-11-10 Frank 定（P-46，"都按建议"）**：按建议执行 |
| 73 | F-CVS-04 规则 4、F-CVS-03 规则 6、第 4.1 节 | "每人每秒 5 次"没有变量名，没说 `convSet`、`convDelete` 是不是一个计数、在校验的哪一步；`deleted` 的 2000 上限没有变量名 | `IM_CONV_SETTINGS_RATE_PER_SEC`（5），两个命令共用一个计数，格式之后、查库之前，不计入发送限流；`IM_CONV_DELETED_MAX`（2000）（BE-483、BE-484） |
| 74 | F-CVS-03 规则 3 | "有新消息时重新出现……清除置顶"没说什么时候清：删除时还是重新出现时 | 删除时同一事务清（`pinned_at=0`），隐藏的会话不占置顶名额；AC-CVS-03-6 的结果相同（BE-485 第 2 条） |
| 75 | 第 4.1 节共同规则 | "`GET /v1/config` 的 `features` 告诉客户端隐藏入口"，C5 的字段名没写 | `features.convSettings`（同 C4 的 `msgActions`，老服务端没有按 false），`http-client.schema.json` 同步（BE-482 第 3 条） |
| 76 | 第 12.1 节 | C3 计划的迁移没有提交，C4 已用 `0018` | C5 用 `0019_conv_settings.sql`；C3 恢复时 BE-451 用下一个空号（同第 52 条） |
| 77 | F-CVS-03 规则 2 | "`sync` 和文件地址都按这个 `minSeq`"：现在单聊 `minSeq` 固定 1（`toConversation`、`authorize`），文件地址的 `canAccess` 自己算、不经过 `authorize`；撤回、引用清除范围内的消息报什么错没单独写 | **按 PRD 原文实现，不是新决定**：四处一起改（BE-485 第 4 条）。撤回按"看不到的消息"4044（同第 65 条），引用按 F-QUO-01 已写的"`seq ≥` 发送者的 `minSeq`"返回 4040 `quoteUnavailable` |
| 78 | F-CVS-01 规则 5、F-CVS-03 规则 8 | 没有消息的单聊：`convSet` 是 4040，`convDelete` 是幂等成功 | 两条都是原文，照写，不统一（记录） |
| 79 | F-CVS-03 规则 6 | `deleted`"最多最近 2000 条"：按什么算"最近"、已经重新出现的会话在不在里面没写 | 只列当前隐藏的，按 `deleted_at` 倒序；重新出现的不在里面（AC-CVS-03-2、03-12"`deleted` 不再含"也要求这样） |
| 80 | 第 11.5 节 4220 | 文案"最多置顶 20 个会话"写死 20，`IM_CONV_PIN_MAX` 可以配置 | 按 PRD：4220 不带 `details`，文案固定；改默认值时要同步三端文案（部署说明写明）。带 `details.max` 是改冻结接口，不在本批 |
| 81 | 第 4.1 节共同规则、F-CVS-03 规则 2 | `IM_CONV_SETTINGS_ENABLED` 打开过、又关掉以后，已有的置顶、免打扰、删除（隐藏和 `clearedSeq`）还生不生效没写；共同规则只说"新命令返回 4030""不影响已有消息的显示和读取" | **要 Frank 定**。建议：照常生效，只拦 `convSet`、`convDelete`（4030）和隐藏入口。理由：否则关开关会让用户删掉的历史重新可见（规则 2"我这边这段历史也不再可见"），置顶顺序也会突然变。另一个选项是关开关时全部忽略，代价就是这两点。**2026-11-10 Frank 定（P-46，"都按建议"）**：按建议执行 |
| 82 | F-ACC-08 规则 3、5 | 注销到期的 `account_cleanup` 逐群退出时会按 F-CVS-01 规则 6 删掉群的 `conv_settings` 行，但**单聊的行**（置顶、免打扰、删除位置）不在数据处理表里 | **要 Frank 定**（改"如实告知"的数据处理表）。建议：第 ② 步一并删除这个用户的全部 `conv_settings` 行（和好友、黑名单同属"删除"），隐私清单草稿加一行；不删的代价是留一份没有用处的个人设置。**2026-11-10 Frank 定（P-46，"都按建议"）**：按建议执行 |
| 83 | AC-CVS-04-3、第 15 节集群一行 | AC 是双节点，第 15 节写"第一批推送只支持单节点部署" | 不矛盾：这条验的是节点 2 读到新设置（共享库 + 失效广播），不承诺集群推送；用现有 `twoNodes` 加每个节点一个假通道（BE-490 第 4 条） |
| 84 | F-CVS-03 规则 2、3，F-CVS-04 规则 1 | 删除时"已读推进到 `upToSeq`"、"清除置顶"，其他设备要不要另收 `readUpdated`、`convSettingsChanged` 没写，只写了 `convDeleted` | 只发 `convDeleted`：它已经意味着已读到 `clearedSeq`、置顶清除、`minSeq = clearedSeq + 1`，SDK 按这个更新（frontend-chat 第 55 条，`chat-sync.json` 有用例） |
| 85 | F-CVS-01、F-CVS-03 | 对已隐藏（删除后还没有新消息）的会话 `convSet` 怎么办没写；客户端没有入口，只有直接调接口能做到 | 允许，照常写设置，隐藏条件不变（只看 `deleted_at`、`cleared_seq`）；置顶占名额，重新出现时保持置顶（删除时已经清过一次，这是删除之后新设的） |
| 86 | F-CVS-03 规则 2、F-QUO-02 规则 6 | u1 删除会话后，u2 的新消息引用了被清除范围内的旧消息：u1 收到的 `quote` 带旧消息的摘要（最多 100 码点），删掉的历史露出一小段；AC-CVS-03-10 只要求"本地搜不到旧特征串" | **要 Frank 定**（产品行为）。建议：接受，和 F-QUO-02 规则 6（看不到入群前消息的新成员也能看到引用摘要）一致，也是常见 IM 的做法，引用是对方新发的内容。另一个选项：按查看者的 `minSeq` 把 `quote.state` 改成 `deleted`、不带 `text`（引用组装要按查看者区分，C4 的批量组装要改，约 0.5 人日）。TC-CVS-03-10 里 u2 的新消息不引用旧消息。**2026-11-10 Frank 定（P-46，"都按建议"）**：按建议执行 |
| 87 | F-CVS-03 规则 2 | "后台查询不受影响"没有单独的 AC | 不加 AC：TC-CVS-03-01 第 4 步验后台元数据和正文，console 一侧 CS-205 回归（console-chat 问题 19） |
| 88 | AC-CONV-01-2、01-4、01-7，AC-UI-23-1、23-2 | 这几条写的是"移到列表顶部""最近更新的 500 个""总未读是全部之和"，有置顶、免打扰时字面不成立 | **按 PRD 处理，不是新决定**：AC 的输入里没有置顶和免打扰，结果不变（第 16.2 节已对 AC-UI-23-1、23-2 写明"没有免打扰时结果不变，口径加注"；AC-CONV-01-* 的依据是 F-CVS-01 规则 3、4 和第 16.1 节 F-CONV-01 一行）。BE-488 在 PRD-core、PRD-ui 加注，AC 不改 |
| 89 | F-CVS-04 规则 3、F-APN-02 规则 7 | 规则 3 写"缓存走 `cache.Invalidator`，TTL 60 秒"，F-APN-02 规则 7 写 worker 按消息批量查库 | 不矛盾：worker 每条消息批量读免打扰（不缓存，改完立即生效）；有缓存的是角标聚合（60 秒），它订阅 `convSettings` 主题（BE-483 第 4 条发布，BE-490 第 3 条订阅） |
| 90 | F-CVS-02 异常、第 4.5 节 | "设置读取失败按没有免打扰处理"只说推送；`convs` 读设置失败怎么办没写 | `convs` 读设置失败是普通的 500 / 5000（和读会话失败一样），不降级成"没有设置"（否则删掉的会话会重新出现）；推送路径的降级在 BE-490 第 2 条 |
| 91 | BE-487 第 1 条、P-46 ④（BE-487 实现时发现） | 第 1 条写本地清除时"引用这些消息的 `quote.text`"也清，和 P-46 ④"删除会话后，对方新消息里的引用摘要照常显示"冲突 | **已按 PRD 处理（实现 Frank 已定的 P-46 ④，同 frontend-chat 第 62 条，不是新决定）**：保留下来的消息（seq ≥ minSeq）里引用被清除消息的摘要照常显示；向量用例"本地清除（convs 路径）"里有一条这样的消息。第 1 条已改 |
| 92 | PRD-ui 6.6 节、AC-UI-11-3（BE-488 时 FE-683 发现） | 6.6 节原来没有列 C2 起代码里在用的图标（图片、文件各类型、下载、关闭、缩放，以及设置、前进）；"图标在清单里"目前没有自动检查：check-assets 只查构建产物里的 svg 文件，lucide 是内联组件，查不到 | **已按 PRD 处理（补清单，不改行为）**：BE-488 按 tianting-web `src/components/Icon.tsx` 补进 PRD-ui v1.5 的 6.6 节。缺的自动检查记录在这里：建议各端加一个用例，从图标注册表（H5 是 `Icon.tsx` 的 `icons`）取出 Lucide 名字，和 6.6 节表格逐个比对（属于前端任务，没有分发） |

C6 分发时发现的（2026-11-11，PRD 冻结，没有改；按"建议"一列实现，Frank 不同意再改）。**要 Frank 定的是第 104、108、118、119、121 条**（都不阻塞开发，定之前按建议实现）；第 95、105、106、107、120 条是"PRD 别处已写明"，按原文实现，**不算新决定**；其余是缺口或记录：

| # | 位置 | 问题 | 建议 |
|---|---|---|---|
| 93 | 第 17 节 C6 行、第 20.3 节、PRD-admin 第 12 节 | C6 的开工条件里有"Frank 指定词库负责人和举报处理人"，P-47 定了开发不等它；上线门槛分散在第 4.1、12.3、17、20.3 节和 PRD-admin 第 12 节 | 记录：开发只用特征词，不阻塞；门槛汇总成清单写进部署说明（BE-498 第 4 条），生产打开 `IM_MOD_ENABLED=1` 前逐条确认 |
| 94 | 第 10 节、C3 暂停（P-43） | C6 有没有依赖推送的 AC | 逐条核对没有：F-MOD 的"收不到任何帧"都是 WS 帧，哨兵法用普通消息就行，不要推送 worker。**没有"C3 恢复后做"的任务**。C3 恢复时 BE-459 补一条：4510 拦下的消息、举报、结案都不进推送队列（F-MOD-03 规则 7 的"不会收到任何通知"包括离线通知） |
| 95 | F-MOD-03 规则 5、F-MOD-04 规则 4、AC-MOD-03-10、04-3、04-13、第 11.6 节 | `IM_C2C_BODY_ACCESS` 和举报的关系分散在几处 | **按 PRD 原文实现，不是新决定**（C6 开头的说明 ①～④）：存证据和读证据各读一次开关；`off` 时单聊只存元数据，因此不存文件引用、不延长文件保留（规则 5 的"只存元数据"，元数据的范围见 F-MOD-04 规则 2：举报人、被举报人、时间、理由、消息类型和长度）；读证据 `off` 一律 403 `c2cBodyDisabled`；之后打开开关也读不到 `off` 期间的正文 |
| 96 | 第 4.1 节共同规则 | "`GET /v1/config` 的 `features` 告诉客户端隐藏入口"，C6 的字段名没写 | `features.moderation`（同第 75 条），`http-client.schema.json` 同步（BE-491 第 3 条）。只控制"举报"入口；4510 的提示不看它 |
| 97 | 第 11.6 节 | 词库、命中、举报、证据授权、证据读取、结案几个接口只有路径和 scope，没有请求和响应 | BE-491 第 4 条在 `ADMIN.md` 写定，形状按 BE-493、494、496 的内容：导入 `{words}` → `{added, duplicate, invalid}`；授权 `{reportId, reason}` → `{grantId, expiresAt}`；证据 `{grantId}` → `{messages, note?, undecryptable?}`；结案 `{result, reason}` → `{}` |
| 98 | F-MOD-02 规则 7、第 4.1 节 | `moderation_hits` 保留 90 天没有变量名 | `IM_MOD_HITS_DAYS`（90），定时任务 `moderation_hits_purge`（第 11.4 节已列） |
| 99 | 第 4.1 节、第 14.2 节、F-MOD-04 异常 | `IM_REPORT_KEY` 只在配置检查一行出现，没写缺失时怎么办、能不能和别的密钥相同、怎么轮换 | 同第 54 条：`IM_MOD_ENABLED=1` 时缺失或长度不对，任何环境都启动失败；不能和 `IM_QUARANTINE_KEY`、JWT 密钥相同（同 F-KEY-02 规则 1 的思路）；**第一批不做轮换**，换密钥后旧证据走"证据无法解密"（F-MOD-04 异常已写这种情况），部署说明写明（BE-498 第 1 条） |
| 100 | F-MOD-01 规则 1、AC-MOD-01-4 | "去重后入库"按原文还是标准化后去重没写 | 按标准化后的 `norm` 去重（`ＢＡＤＷＯＲＤ` 和 `badword` 是同一个词）；已有的（含停用的）算 `duplicate`，状态不变，要启用就单独启用 |
| 101 | F-MOD-01 规则 3 | 第二遍"去掉空白和标点再匹配"，词本身含空格、标点时怎么办没写 | 词和待查文本过同一个标准化函数，`compact` 自动机里的词也去掉空白和标点；误判用语料测一次（R-CS-17），结果写进验收报告 |
| 102 | F-MOD-01 异常 | "空、超过 64 个码点"只写了导入，单个添加没写 | 单个添加同样校验，400 / 4000（导入是跳过并计数） |
| 103 | F-MOD-01 规则 1、AC-MOD-01-7 | 批量导入的格式没写；结果汇总的三项（新增、重复、无效）没有字段名 | 接口收 JSON `{words: [...]}`；后台页面读 txt（一行一个，UTF-8）后拆成数组再提交，文件内容不上传到 console 以外的地方；结果 `{added, duplicate, invalid}` |
| 104 | F-MOD-04 规则 3、4 | 规则 4 只写了 `IM_C2C_BODY_ACCESS=off` 时单聊证据 403；`on` 时单聊证据用什么权限读没写：只要 `report.evidence`，还是另要 `msg.read.c2c`（PRD-admin 5.2 节，审核才有） | **要 Frank 定（隐私口径，不阻塞：第一批开关是 `off`，法务确认前不会打开）**。建议：`on` 时和群一样只要 `report.evidence` 的授权流程，im-server 照常检查开关；两个权限点都只在审核角色上，区别只在以后有自定义角色时。定之前 `on` 的路径按建议实现，用例只覆盖 `off`。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 105 | F-MOD-02 规则 4、第 4.1 节 | `IM_CLIENT_CUSTOM_MSG` 是不是只在 `IM_MOD_ENABLED=1` 时生效 | **按 PRD 原文，不是新决定**：第 4.1 节是独立的配置项，"生产关、测试开"，规则 4 也没有挂开关。所以 C6 部署后生产环境的客户端就不能发 `custom`（不管 `IM_MOD_ENABLED`）；开发、测试环境默认开，现有 e2e 不受影响。部署说明写明 |
| 106 | AC-ACC-01-19、AC-ACC-01-9 | C1 的 AC-ACC-01-19 写"open 模式拒绝启动（C6 未上线）"，C6 之后 open 满足四个条件就能启动 | **按 PRD 原文，不是新决定**：F-ACC-01 规则 2 写的就是四个条件；AC-ACC-01-19 的输入没有这些条件，结果仍是拒绝，断言不变，只是错误信息从"还没有实现"变成逐项指出缺什么（`checks.go` 第 133 行的注释已写"逐项报错随 C6"） |
| 107 | 第 14.2 节、F-MOD-04 规则 7 | 指标表只有名字 `im_reports_pending`，没说是全部待处理还是超过 24 小时的 | **按 PRD 原文**：规则 7 写的是"超过 24 小时未处理的数量进指标"，就是它；不加标签 |
| 108 | 第 4.1 节共同规则 | `IM_MOD_ENABLED` 打开过、又关掉以后：后台词库维护、举报队列、证据读取、结案还能不能用，`report_purge`、`moderation_hits_purge` 还跑不跑，没写 | **要 Frank 确认**（同 P-46 第 81 条的口径）。建议：只拦客户端的 `report` 和敏感词检查；后台接口照常可用（已有的举报要能处理完）；**清理任务照常跑**（保留期是对用户的承诺，不能因为关开关而延长）。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 109 | 第 11.6 节 `fetch` 一行（v1.6） | "带 `grantId` 时只认这一个授权……是隔离授权（`quarantine=true`）"只写了 `msg.quarantine` 路径；`report.evidence` 路径的授权怎么找没写 | 带 `report.evidence` 的调用方：`grantId` 必须是这个调用方 + 操作人的、没过期的**举报授权**，而且这条举报的证据引用了这个 `fileId`；任何一条不符合（含没有 `grantId`）403 / 4039。`msg.quarantine` 路径不变（BE-496 第 4 条） |
| 110 | F-MOD-02 规则 7、第 14.2 节 | `moderation_hits` 的"场景"和 `im_blocked_total{scene}` 的取值没写 | `text`、`fileName`、`nickname`、`username`、`groupName`、`groupNotice`、`friendApply`，两处一致；标签有界 |
| 111 | F-MOD-02 规则 2、AC-MOD-02-4 | "`image`、`file` 的文件名"查哪一份：客户端在 `send` 里带的，还是上传时服务端记录的 | 查服务端记录的 `files.name`（下发以服务端记录为准，第 11.1 节）；上传申请时不查（发出之前别人看不到） |
| 112 | F-MOD-02 规则 2、5，AC-MOD-02-5 | 规则 5 说敏感词放在所有权限判定之后，但附言要"在 F-SOC-02 的存在性判断之前"；注册用户名的检查放在哪一步没写 | 附言：格式、限流之后，存在性判断之前（原文）。注册：邀请码校验之后、用户名唯一性判断之前（同样不泄露"这个用户名已存在"）。改资料、建群、改群：各自的权限判定之后 |
| 113 | F-MOD-02 规则 7 | 注册时命中，还没有 userId，`moderation_hits` 写什么没写 | 不写行，只计 `im_blocked_total{scene="username"}` 和 `nickname`；不记尝试的用户名 |
| 114 | F-MOD-02 规则 7、AC-MOD-02-14 | `/admin/moderation/hits?userId=` 的响应没写 | 保留期内按场景的次数 + 最近 50 条（时间、场景、`word_id`），没有任何内容；看词要另有 `moderation.words` |
| 115 | F-MOD-03 规则 4 | 每天 20 次算哪些：幂等的重复、4040 的算不算 | 通过格式和开关之后的请求都算，包括 4040（防止用举报试探关系，响应本来就相同）；**幂等的重复不算**（没有新记录，AC-MOD-03-3 提交 3 次都成功）；按 Asia/Shanghai 自然日 |
| 116 | F-MOD-03 规则 3 | "有共同会话"怎么判断没写 | 两人的单聊有过消息（`conv_seq` 有行且 `max_seq > 0`），包括一方删除了会话（C5，后台查询不受影响，关系也还在） |
| 117 | F-MOD-03 规则 4、F-MOD-04 规则 1、AC-MOD-04-6 | 幂等的"同一对象"和队列聚合的"相同对象"没定义；"同一用户被 3 人举报"可能是 3 条用户举报，也可能是举报他的 3 条不同消息 | 幂等键：消息是 `convId` + 排序后的 `msgIds`，用户是 `userId`，群是 `groupId`（都加举报人）。队列按同样的对象聚合显示次数；详情另显示"该用户被举报的总次数"（所有类型，F-MOD-04 规则 4 也用它）。AC-MOD-04-6 用 3 条用户举报 |
| 118 | F-MOD-03 规则 5、F-MOD-04 规则 2、3 | **举报用户、举报群**的证据存什么没写（规则 5 只写了消息）；它们的 `note`"按正文对待、走规则 3 的授权"，但规则 3 是群消息证据的流程，用户举报不属于任何会话，单聊开关管不管它没写 | **要 Frank 定（收集范围和隐私口径）**。建议（按最保守）：用户、群举报只存元数据（对象 ID、举报时间），不快照昵称、群名、公告（规则 2"不扩大收集范围"）；`note` 都加密存；群举报和群消息举报的 `note` 走规则 3 的授权；**用户举报和单聊消息举报的 `note` 按单聊对待**，`IM_C2C_BODY_ACCESS=off` 时看不到（单聊骚扰多数走用户举报，`note` 里很可能是转述的单聊内容）。另一个选项：用户举报的 `note` 也走授权、不受单聊开关限制，代价是单聊内容可能经 `note` 绕过 X-7。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 119 | F-MOD-03 规则 6、第 12.1 节 | 证据 180 天物理删除；`reports` 行（举报人、对象、理由、结论、处理人）保留多久没写，隐私说明要写 | **要 Frank 定（保留期，隐私口径）**。建议：和证据一起在 180 天删除整条举报（最少保留；"被举报次数"只按 180 天算）。另一个选项：`reports` 行保留到审计保留期（1 年），代价是举报人和被举报人的关系多留半年。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 120 | PRD-admin 5.2 节、F-KEY-02、PRD-chat 第 14.1、16.1 节 | PRD-admin 现在是 `word.write`（P2，只有审核），PRD-chat 是 `moderation.words`（运营、审核）；PRD-admin 矩阵没有 `report.*` | **按 PRD 原文，不是新决定**：第 16.1 节已写 PRD-admin 的 F-ADM-60、F-KEY-02、第 5 节矩阵按第 11.6、14.1 节改，BE-497 第 1 条落实（超管默认没有 `report.evidence`，第 14.1 节） |
| 121 | 第 14.1 节词库一行、F-MOD-02 规则 7、第 11.6 节 | 第 14.1 节写词库页有"命中统计"、权限 `moderation.words`（运营也有）；第 11.6 节按用户查命中次数要 `report.read`。按用户查"谁反复试探"是个人行为数据，运营能不能看没写 | **要 Frank 定（权限口径）**。建议：命中统计（按用户查次数）只对有 `report.read` 的（审核）显示，词库页和举报详情都有入口；只有 `moderation.words` 的运营看不到这个入口（第 11.6 节没有按词统计的接口，不新增）。另一个选项：运营也能查，代价是运营能看到具体用户的试探记录。console 一侧见 console-chat 第 27 条。**2026-11-11 Frank 定（P-48，"都按建议"）**：按建议执行 |
| 122 | F-MOD-03 规则 1、5，AC-MOD-03-8 | `note` 存在 `reports` 行还是证据里没写；规则 2 说 `note` 和文件名按正文对待 | `note` 和文件名只在加密的 `report_evidence` 里，`reports` 行只有 ID、类型、理由、状态；日志和审计只记 `reportId` |
| 123 | 第 4.5 节故障注入、BE-492 第 5 条、AC-MOD-01-8 第 2 种 | "词库加载失败"要分重新加载和启动加载；启动加载失败发生在进程起来之前，`POST /admin/test/faults` 还不存在 | **已按 PRD 处理**：`wordsReload` 走 `/admin/test/faults`；启动加载失败用环境变量 `IM_TEST_WORDS_STARTUP_FAIL=1`（只在 `IM_TEST_MODE=1` 生效，生产配置里出现就拒绝启动，同 `IM_TEST_DISABLE_AUTH_LOCK` 的做法）；另有不靠注入的用例（库里的 `blocked_words` 读不了时启动失败） |
| 124 | F-MOD-03 规则 4、第 115 条、TC-MOD-03-02、03-14 | 第 115 条要 4040 也算次数，TC-MOD-03-14 要证据写失败不算；只靠 `reports` 行数做不到（4040 没有记录），任务单第 12.1 节的四张表里没有计数 | **已按 PRD 处理**：`0020` 另加 `report_quota(user_id, day, count)`（Asia/Shanghai 当天 00:00 的毫秒）。写入事务里条件加 1（并发也不超）；4040 单独加 1；幂等的重复、写失败（事务回滚）不加；`report_purge` 清两天前的行。只有计数，没有内容 |
| 125 | F-MOD-04 异常、第 14.2 节 | 读证据时密钥不匹配"告警"，第 14.2 节的指标表没有对应的名字 | **已按 PRD 处理**：`im_report_undecryptable_total`（计数器，没有标签）；告警规则不默认加（第一批只开词库加载失败） |
| 126 | F-MOD-04 规则 7、第 107 条 | `im_reports_pending` 抓取时查库，库读不了时是 0 还是别的值没写 | **已按 PRD 处理**：查询失败返回 -1，不当成"没有积压" |
| 127 | 第 99 条 | 只写了开关打开时 `IM_REPORT_KEY` 的检查；开关关闭但配了一个格式不对的值怎么办没写 | **已按 PRD 处理**：设置了就校验格式（不是 32 字节 base64 拒绝启动）；没设置时开关关闭照常启动。开过又关掉的部署要保留同一把密钥，已有证据才能读（P-48 ④） |
| 128 | F-MOD-01 异常、第 102 条 | 只写了空和超过 64 个码点不合法；只由零宽字符、变音符组成的词标准化后是空串，入库后匹配不到任何东西 | **已按 PRD 处理**：标准化后 `norm` 为空的同样不合法（单个添加 400 / 4000，导入计入 `invalid`）。只有标点的词 `compact` 为空，只按 `norm` 匹配，照常入库 |
| 129 | F-MOD-02 规则 2、AC-MOD-02-5、TC-MOD-02-05 第 3 步 | 规则 2 写"客户端写入的群名、群公告（`groupCreate`、`groupUpdate`）"，TC-MOD-02-05 有"`groupCreate` 带含词的公告"；但协议的 `groupCreate` 没有公告字段（`GroupCreateReq` 只有 `clientReqId`、`name`、`members`，Schema 也不允许多余字段） | **已按 PRD 处理**：`groupCreate` 只查群名，公告只能经 `groupUpdate` 写，在那里查；TC-MOD-02-05 的那一步按"`groupUpdate` 改公告"覆盖，用例文档随 BE-499 校对 |
| 130 | F-MOD-03 异常、规则 2a | "消息已物理删除……证据只有元数据，举报仍成功"：消息行都没了时，服务端无法核对它属于哪个会话、举报人能不能看到 | **已按 PRD 处理**：理解为举报之后才被删除；举报时消息行不存在按对象不存在 4040（和其他 4040 响应相同）。后台删除（`deleted_at`）的消息和撤回的一样处理：要 `readSeq ≥ seq`，有隔离原文就存原文 |
| 131 | F-MOD-03 规则 3 | 能不能举报自己没写 | **已按 PRD 处理**：举报自己（用户举报）按没有关联处理，4040；举报自己发的消息不拦（客户端菜单没有入口，frontend-chat 第 77 条），被举报人记为发送者 |
| 132 | F-MOD-04 规则 3、第 11.6 节 | 证据授权"配额和 `msg-grants` 合并"，`0020` 只加新表 | **已按 PRD 处理**：举报授权也存在 `msg_grants`，`conv_id` 是 `report:{reportId}`（不会和真的会话 ID 相同，正文读取、隔离读取、`fetch` 的隔离路径按会话匹配时永远对不上），`ticket` 是 `reportId`；配额、48 小时清理都和 `msg-grants` 一起 |
| 133 | AC-MOD-04-14、F-MOD-04 异常 | "写审计失败：操作不执行（5031）"：5031 是 console-server 的码；im-server 自己的审计写失败沿用 F-KEY-04 的 503 / 5030 | **已按 PRD 处理**：im-server 一侧 503 / 5030、状态不变（用例 TC-MOD-04-14 的 im-server 部分）；console 的 5031 随 CS-207 |
| 134 | F-MOD-04 规则 3、第 11.6 节证据读取一行 | 证据里"文件名"：协议的 `image` 没有文件名字段（只有 `file` 有） | **已按 PRD 处理**：图片证据只有 `fileId`、宽高、大小、类型，缩略图和原图经 `fetch` 看；文件证据带文件名 |
| 135 | F-MOD-03 规则 5、AC-MOD-03-5 | 举报之后被撤回的消息，证据里是什么 | **已按 PRD 处理**：证据是举报那一刻的快照（"不依赖消息后续是否被撤回"），所以是举报时的原文，`recalled` 是举报时的状态；撤回之后再举报（AC-MOD-03-11、03-12）的证据才是隔离原文、`recalled=true` |
| 136 | BE-497 第 4 条、PRD-core v1.7 第 310 行和第 740 行 | 任务单说"F-MOD-01 规则 6 的第 9 步已随 C4 写进说明的话核对即可"；核对结果是没有写：PRD-core 第 310 行的 C4 说明只到第 7、8 步，第 740 行还写着"敏感词与内容审核 P2" | **已按 PRD 处理**：PRD-core 不在 BE-497 的改动范围（第 16.1 节没有 C6 的 PRD-core 行），这次不升版；规则以 PRD-chat 第 4.3 节为准（PRD-core 第 16.1 节 F-MSG-01 规则 6 那一行已指向它）。下次 PRD-core 升版时补第 9 步（敏感词 4510）并改第 740 行 |

### C3 暂停时的状态（2026-11-10）

- **已提交**：BE-450 的协议部分（见进度表）。
- **没有提交、已从工作区撤掉**：BE-451（`0018_device_push.sql`、`store.DevicePushStore` 和契约用例 5 个、配置和开关、`features.offlinePush`）、BE-453 的存储部分（全部吊销入口同一事务删令牌、改密复制 `show_detail`、`session_purge` 清令牌）、BE-454（`delivery.Presence`、hub 实现、连接的收帧时间和前后台、`IM_TEST_DISABLE_IDLE`、关闭回调）、BE-455、456（`push` 包：队列、合并、延迟队列重试、断线补推、角标缓存、内容和 collapse id）、BE-406（`push/pushtest` 假 APNs、假 FCM、假 OAuth）、BE-457（`push/apns`、`push/fcm` 客户端）、app 和 admin 的接线。其中存储契约用例当时已经通过；`push` 包、假通道、客户端只编译过，**没有 e2e，也没有单测**。这份草稿只留在当时会话的临时目录里，不在仓库，**恢复时按任务单重新做，不要假设草稿存在**。
- **恢复时从这里接着做**：按依赖顺序 BE-451（迁移 `0018`，`DevicePushStore`，`IM_OFFLINE_PUSH_ENABLED` 等第 4.1 节配置，4030，`features.offlinePush` 同时改 `http-client.schema.json`）→ BE-406、BE-452、BE-454 → BE-453、BE-455 → BE-456、BE-457 → BE-458 → BE-459。第 47、48 条 Frank 定了再落地；BE-455 的重试按 v1.5 的间隔（延迟队列 + [IM 钟]）。
- **草稿参考**：暂停时的未测试草稿由协调方存成**本机本地分支** `wip/c3-push-2026-11-10`（c373fc4，基于 8dd5ab1，没有推送到远端，不要原样合并），恢复时可以对照，但按任务单重新做和测试。

