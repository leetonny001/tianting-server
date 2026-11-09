# 后端任务单：天听聊天产品第一批（C1 账号、C2 图片和文件、C3 离线推送、公共部分）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-chat v1.5](../prd/PRD-chat-v1.5.md)（冻结）第 4～7、11、12、14、15、16、18、20.3 节；[ARCH-v1](../arch/ARCH-v1.md)；[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[管理接口](../../protocol/ADMIN.md) |
| 范围 | **C1、C2、C3 和公共部分**（PRD 决定 CH-27；C3 是 Frank 2026-11-10 决定按顺序继续的）。C4～C6 的 AC 不进任务；【全部】的 AC 验证到 C3 为止的部分（PRD 第 15 节累计表） |
| 负责 | 后端（AI） |
| 仓库 | im-server（GitHub 仓库 `tianting-server`，本地 `im-mvp/server/`）；协议在同仓库 `protocol/`，部署在 `deploy/`，工具在 `tools/` |
| 编号 | **BE-400 起**。公共 BE-400～BE-408，C1 BE-410～BE-424，C2 BE-430～BE-440，C3 BE-450～BE-460 |
| 起点 | 现有迁移到 `0014_admin_stats.sql`；只有 SQLite；`IM_V13_ADMIN_FEATURES` 默认关闭；`tools/objstore-contract` 已有合同工具 |
| 完成定义 | 任务内的 AC 都有通过的自动化测试，用例名带 TC 编号（[TC-ACC](../testcases/TC-ACC.md)、[TC-FILE](../testcases/TC-FILE.md)、[TC-APN](../testcases/TC-APN.md)、[TC-CHAT](../testcases/TC-CHAT.md)）；`go test -race ./...` 全过；默认配置下 `server/e2e` 现有文件不改代码全过（AC-ACC-07-1）；Schema、向量、`ERRORS.md`、`ADMIN.md` 同步；`python3 -I tools/trace.py --check` 没有缺口 |
| 环境限制 | **CI 现在不运行**（[PENDING](../reports/PENDING.md) P-21，Actions 预算是 $0），所有结果都是本地结果，要标明。开发机不装软件；真实 MinIO 要 Frank 同意才能临时构建。压测机 P-12、真机和渗透 P-13 未到位。**C3 的真实 APNs、FCM 要 Frank 先做 PRD 第 20.3 节的账号和网络步骤**（PENDING P-40），之前只用假通道 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。一条 AC 可以出现在多个任务里（功能任务 + 验收任务）。C4～C6 只写在任务内容里说明，不进 `- AC：` 行。

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
| BE-406、BE-450～BE-460 | ⬜ 未开始 | 2026-11-10 分发 C3（任务、TC-APN、`trace.py` 加【C3】）。BE-406 在 C2 期间没有做，随 C3 第一波做。官方文档核对见文末第 34～46 条，PENDING P-40 |
| 全部 | 🟡 C1、C2 已校准 | 见 [c1-acceptance-2026-11-07.md](../reports/c1-acceptance-2026-11-07.md) 第 9 节、[c2-calibration-2026-11-10.md](../reports/c2-calibration-2026-11-10.md)：人日不能直接换算成 AI 用时，瓶颈是本机资源和待决定项。C3 已分发；C4 起的部分等 Frank 决定 |

## 总览

| 阶段 | 任务 | 人日 | 说明 |
|---|---|---|---|
| 公共 | BE-400～BE-408 | 14 | 协议、向量、测试设施（假 S3、假通道、账号并发钩子）、测试环境、冻结文档变更 |
| C1 账号 | BE-410～BE-424 | 27.5 | 迁移、`/v1` 基座、闸门和限流、注册、登录、会话续期、改密、恢复码、注销、邀请码、验收、部署和运维手册 |
| C2 文件 | BE-430～BE-440 | 24.5 | 对象存储客户端、上传、`complete`、发送、访问控制、生命周期、头像、部署、合同测试、验收 |
| C3 推送 | BE-450～BE-460 | 19.5 | 协议、迁移和开关、令牌注册、吊销删令牌、在线判断和补推、队列和 worker、内容和隐私、APNs 和 FCM 通道、部署、验收、文档（假通道 BE-406 的 1 人日在公共里） |

**人日对照（PRD-chat 第 18 节）**

| 第 18 节的列 | 公共 | C1 | C2 | C3 | 落在 |
|---|---|---|---|---|---|
| 服务端 | 3 | 18.5 | 14.5 | 13 | 本单：BE-400～402、405；BE-410～421；BE-430～437；BE-450 0.5、BE-451 1.5、BE-452 2、BE-453 1.5、BE-454 2、BE-455 3、BE-456 1、BE-457 1.5 |
| console + 后台页面 | 0 | 1.5 | 2.5 | 0.5 | [console-chat](console-chat.md) 和 [frontend-chat](frontend-chat.md)；C3：CS-203 |
| sdk-ts / sdk-kotlin / sdk-swift | 1 / 1 / 1 | 3.5 / 4 / 4 | 3.5 / 5.5 / 5.5 | 0 / 3 / 3 | [frontend-chat](frontend-chat.md)；C3：FE-650、FE-651（sdk-ts 只同步协议副本，随 BE-450） |
| H5 / Android / iOS | 0 | 5 / 5.5 / 5.5 | 6 / 8 / 8 | 0 / 5 / 5 | [frontend-chat](frontend-chat.md)；C3：FE-652、653；FE-654、655 |
| 测试 | 6 | 8.5 | 9.5 | 6 | 公共 6：BE-402～406。C1：BE-422 5 + FE-620 3.5。C2：BE-440 5.5 + FE-640 4。C3：BE-459 3.5 + FE-656 2.5 |
| 部署 | 1 | 1.5 | 3 | 2 | BE-407；BE-423；BE-438 2 + BE-439 1；BE-458 2 |
| 文档 | 4 | 3 | 1.5 | 1 | 公共：BE-400 1、BE-401 0.5、BE-408 2.5。C1：BE-423 1.5、BE-424 1、FE-620 0.5。C2：BE-438 1、BE-439 0.5。C3：BE-450 0.5、BE-460 0.5 |
| **合计（三份任务单）** | **17** | **60.5** | **67.5** | **38.5** | **183.5，和第 18 节一致**（C3 本单 19.5、console-chat 0.5、frontend-chat 18.5） |

**差异说明**：合计和第 18 节没有差异，因为这里是按第 18 节的列拆分，不是独立重估，**都是低置信**。下面这些第 18 节没有单独算，也没有另加人日，如果 C1 校准时超出就从这里看：① 第 4.5 节没有列的暂停点和故障注入（BE-404、BE-405）；② `sessions revoke-all` 命令路径（BE-415）；③ 邀请码重新启用和可信出口网段的配置（BE-421、BE-423）。**C3 同样是按列拆分**，第 18 节没有单独算、这里也没有另加人日的：④ 官方文档核对后补的行为（JWT 定时重签、`ExpiredProviderToken` 重签重试、延迟重试队列、410 的 `timestamp` 比较，文末第 34～46 条）；⑤ 假通道按官方错误码补的注入（BE-406）；⑥ 真机等待和 Frank 的账号步骤（第 18 节本来就不含）。

**依赖顺序**：BE-400、BE-401 → BE-408、BE-402、BE-403 → BE-405、BE-407 → BE-410 → BE-411 → BE-412 → BE-413、BE-414 → BE-415 → BE-416 → BE-417 → BE-418 → BE-419、BE-420、BE-421 → BE-424、BE-423 → BE-422 →（C1 结束，校准）→ BE-404、BE-406 → BE-430 → BE-431 → BE-432 → BE-433 → BE-434 → BE-435 → BE-436 → BE-437 → BE-438、BE-439 → BE-440 →（C2 校准，Frank 决定 C3）→ BE-450 → BE-406、BE-451、BE-460 → BE-452、BE-454 → BE-453、BE-455 → BE-456、BE-457 → BE-458 → CS-203 → BE-459。console 和前端的依赖写在各自任务里。

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
| 32 | F-RCL-03 规则 2 | 重连时有几个已加载的会话都要重拉，其中一个 `sync` 失败，其余会话这一次还拉不拉没写 | 三套 SDK 一致：按顺序重拉，第一个失败就停，这一轮其余的不拉；失败的会话不记新的 `modVer`，下次重连再拉（`chat-sync.json` 只断言"失败的下次再拉"，出错的用例里只有一个已加载的会话）。**要 Frank 定**：保持现状，或改成失败的跳过、其余照拉（三套 SDK 和向量一起改） |
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
