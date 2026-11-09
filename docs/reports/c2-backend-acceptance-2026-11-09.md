# C2 后端验收报告（BE-440，2026-11-09）

| 项 | 内容 |
|---|---|
| 范围 | backend-chat.md 里 BE-430～BE-440 的 `- AC：` 行，共 80 条。**前端和三端的 AC 不在本报告**（FE-640 另做）；端到端 AC 只记后端部分 |
| 环境 | 本地开发机（macOS，Apple Silicon 10 核）。**CI 不运行（P-21），所有结果都是本地结果**。对象存储是进程内假 S3（`internal/objstore/objstoretest`，独立端口，校验 SigV4）；**假 S3 只验调度逻辑，不证明厂商行为** |
| 基线 | im-mvp@892fd62 + 本报告所在的提交（代码、用例、部署样例同一个提交） |
| 结论 | 通过 69、部分通过 7、未执行 4、失败 0。性能没有判定（P-12）；真实存储没有运行（第 3 节） |

## 1. 运行

| 命令（`server/` 下） | 结果 |
|---|---|
| `go vet ./...`、`gofmt -l .` | 无问题 |
| `go test -count=1 ./...` | 全部包通过（e2e 约 117 秒） |
| `go test -race -count=1 ./e2e/ ./internal/objstore/... ./internal/files/ ./internal/config/ ./internal/metrics/` | 全部通过（e2e 约 360 秒） |
| `IM_PERF_DEV=1 go test -count=3 -run …`（`TestTC_CHAT_90_03_ObjStoreDownText`、`TestTC_CHAT_90_11_UploadRequestLatency`、`TestPerfDevRecord`） | 3 次都通过，数字见第 4 节 |
| `python3 -I -m unittest discover -s deploy/objstore -p 'test_*.py'`（仓库根） | 11 项通过 |
| `python3 -I tools/trace.py --check --summary` | gaps 0 |

`TestTC_FILE_90_01_RealStore` 在没有 `IM_E2E_S3_*` 时跳过（设计如此）。`deploy/test_deploy.py` 需要 pyyaml，本机没有，没有运行（本次没改它覆盖的文件）。

## 2. 本次新增和修改的用例

- `e2e/file_acceptance_test.go`：
  - **F-FILE-01 攻击序列对应表**（22 条：RC2-07～09、RS4-16 甲乙丙、RS4-17/RS5-21、RS4-18～21、RS4-23 甲乙、RS5-06、RS5-07、RS5-16 甲乙、RS6-04、RS7-03 和 F-FILE-02/05/06 的伪造、多语言文件、绑定竞争），`TestFileAttackSequencesHaveTests` 断言每条都有测试函数；`fileAttackVendorOnly` 列出假 S3 证明不了的 4 项。序列本身的用例在 `file_complete_test.go`、`file_upload_test.go`、`file_avatar_test.go` 等（BE-433～437 时已写）。
  - `TestTC_CHAT_90_03_ObjStoreDownText`（AC-CHAT-90-3）：不用假 S3 的注入，直接用本地端口——拒绝连接是已关闭的端口，黑洞是只接受连接不响应的 listener。
  - `TestTC_CHAT_90_05_NoSecretsInLogsC2`（AC-CHAT-90-5 的 C2 部分）：用 `deploy/ops/scan-logs` 扫日志和除 `files` 外全部表（含 `admin_audit`）。变异确认：把一个确实写进库的串（`clientMsgId`、删除理由）加进特征串，扫描报出 3 处、用例失败。
  - `TestTC_CHAT_90_16_FileMetricsAndChecks`（AC-CHAT-90-17 的 C2 部分）。
- `e2e/file_realstore_test.go`：`TestTC_FILE_90_01_RealStore`（AC-FILE-90-1 的实现部分，默认跳过）和 `TestRealStoreChecksOnFake`（同一套检查在假 S3 上跑，证明用例本身可用，不算 AC 通过）。
- `config.TestObjstoreHTTPSCheck`、`objstore.TestObserveAndProbe`、`metrics` 标签用例补了 `kind`、`op`。
- `TestTC_CHAT_90_12_UploadRequestLatency` 改名 `TestTC_CHAT_90_11_UploadRequestLatency`：TC-CHAT 里申请上传 P99 的编号是 90-11（90-12 是错误码一致性），原名字用了 AC 编号。只改名，断言不变。

TC-FILE 里层级是"后端 e2e"的用例都有对应测试；有几个和别的用例合在一个函数里（例如 TC-FILE-02-02、02-11 在 `TestTC_FILE_02_01_SendImage`，TC-FILE-06-10 在 `TestTC_FILE_04_02_GroupMembership`），函数注释写了编号，下表按注释对应。

## 3. AC-FILE-90-1（真实 MinIO）为什么没有执行

协调方转达 Frank 对 P-38 ④ 的答复是"同意按上次方式临时跑一次 MinIO"。跑 MinIO 要从网上下载 MinIO 和 mc 的源码（和 Go 模块）再构建。下载和运行外部软件需要 Frank 在会话里直接确认，**经过其他代理转述的同意，本任务不能当作确认**，所以没有下载、没有构建、没有运行，磁盘没有变化。准备好的东西：

- `tools/objstore-contract/run-local-minio.sh` 加了 `RUN_IM_E2E=1`：合同工具跑完后，用同一个 MinIO 和最小权限凭证跑 `TestTC_FILE_90_01_RealStore`（预签名 PUT、超大 PUT、改类型 PUT、过期 PUT、未签名的头、complete 后覆盖、HEAD + If-Match Range + 条件 Copy + 元数据替换、`response-*` 被签入、过期 GET、无签名访问和匿名列举被拒绝、跨桶 Copy 到公开桶、按前缀列举并删除、全桶列举被拒绝、连通检查）。
- Frank 直接确认后：按 GATE-objstore 的方式构建固定版本到临时目录，`MINIO_BIN_DIR=… RUN_IM_E2E=1 tools/objstore-contract/run-local-minio.sh`，跑完删除。生命周期实际清除、CORS 限定来源按 AC 只查配置（见 `deploy/FILES-RUNBOOK.md` 第 7 节）。

## 4. 开发机性能记录（**不算通过**，发布判定等 P-12）

顺序发送，不是 500 条/秒；开发机配置不是生产配置。3 次取最差。

| AC | 测量 | 3 次最差 P99 | 未校准目标 |
|---|---|---|---|
| AC-CHAT-90-1 | 单聊文本，账号 token，1000 条 | 0.47 ms | ≤ 200 ms |
| AC-CHAT-90-3 | 对象存储拒绝连接时文本 1000 条（期间不停 complete 和 /readyz） | 0.58 ms | ≤ 200 ms |
| AC-CHAT-90-3 | 对象存储黑洞时文本 1000 条 | 0.48 ms（单条最慢 4.7 ms） | ≤ 200 ms |
| AC-CHAT-90-12 | 申请上传 1000 次 | 0.50 ms | ≤ 200 ms |
| （参考）AC-CHAT-90-11 | 登录串行 / 4 并发，200 次 | 109 ms / 151 ms | ≤ 500 ms |

## 5. 逐条

| AC | 状态 | 用例编号 | 依据（测试函数） | 说明 |
|---|---|---|---|---|
| AC-FILE-04-4 | 通过 | TC-FILE-04-04 | `TestPresignGetExpiresAndResponseHeaders`, `TestTC_FILE_04_04_URLExpiry` | 含真实 6 秒过期 |
| AC-FILE-04-6 | 通过 | TC-FILE-04-06 | `TestAnonymousAccessDenied` | 单测层（objstore 包，对假 S3） |
| AC-CHAT-90-2 | 部分通过 | TC-CHAT-90-02 | — | 默认配置（`IM_FILES_ENABLED` 关）下现有 e2e 不改代码全过（本次 `go test ./...`）；TC-PERF 判定等 P-12 |
| AC-FILE-01-14 | 通过 | TC-FILE-01-14 | `TestTC_FILE_01_14_SwitchOff` |  |
| AC-FILE-05-6 | 通过 | TC-FILE-05-06 | `TestTC_FILE_05_06_ScanState` |  |
| AC-ACC-01-20 | 通过 | TC-ACC-01-20 | `TestTC_ACC_01_20_NewAccountQuota` |  |
| AC-ACC-06-4 | 通过 | TC-ACC-06-04 | `TestTC_FILE_02_16_ImageWhitelist` |  |
| AC-CHAT-90-12 | 未执行 | TC-CHAT-90-11 | `TestTC_CHAT_90_11_UploadRequestLatency` | 性能。开发机只记录（第 4 节），不算通过，发布判定等 P-12 |
| AC-FILE-01-4 | 通过 | TC-FILE-01-04 | `TestTC_FILE_01_04_DailyQuota` |  |
| AC-FILE-01-6 | 通过 | TC-FILE-01-06 | `TestTC_FILE_01_06_AuthAndOwner` |  |
| AC-FILE-01-7 | 通过 | TC-FILE-01-07 | `TestSanitizeName`, `TestTC_FILE_01_07_NameSanitize` |  |
| AC-FILE-01-11 | 通过 | TC-FILE-01-11 | `TestTC_FILE_01_11_RateAndPending` |  |
| AC-FILE-01-16 | 通过 | TC-FILE-01-16 | `TestTC_FILE_01_16_PendingFreed` |  |
| AC-FILE-01-18 | 通过 | TC-FILE-01-18 | `TestTC_FILE_01_18_QuotaRecheck` |  |
| AC-FILE-01-23 | 通过 | TC-FILE-01-23 | `TestTC_FILE_01_23_CancelHidesExistence` |  |
| AC-FILE-01-28 | 通过 | TC-FILE-01-28 | `TestTC_FILE_01_28_CancelAndComplete` |  |
| AC-FILE-01-29 | 通过 | TC-FILE-01-29 | `TestTC_FILE_01_29_DeclaredLimit` |  |
| AC-FILE-02-16 | 通过 | TC-FILE-02-16 | `TestTC_FILE_02_16_ImageWhitelist` |  |
| AC-FILE-03-2 | 通过 | TC-FILE-03-02 | `TestTC_FILE_03_02_SizeLimits` |  |
| AC-FILE-05-1 | 通过 | TC-FILE-05-01 | `TestTC_FILE_01_03_SniffRejects`, `TestTC_FILE_05_01_02_Blocklist` |  |
| AC-FILE-05-2 | 通过 | TC-FILE-05-02 | `TestSanitizeName`, `TestTC_FILE_05_01_02_Blocklist` |  |
| AC-FILE-01-1 | 通过 | TC-FILE-01-01 | `TestTC_FILE_01_01_HappyPath` |  |
| AC-FILE-01-2 | 通过 | TC-FILE-01-02 | `TestTC_FILE_01_02_SizeMismatch` |  |
| AC-FILE-01-3 | 通过 | TC-FILE-01-03 | `TestTC_FILE_01_03_SniffRejects` |  |
| AC-FILE-01-5 | 通过 | TC-FILE-01-05 | `TestTC_FILE_01_05_StoreDown` |  |
| AC-FILE-01-8 | 通过 | TC-FILE-01-08 | `TestTC_FILE_01_08_BytesBypassServer` | 假 S3 在测试进程里、监听独立端口（问题 12，Frank 已确认） |
| AC-FILE-01-9 | 通过 | TC-FILE-01-09 | `TestTC_FILE_01_09_OverwriteAfterComplete` | 临时前缀生命周期由假 S3 模拟（AC 原文如此），真实清除见 AC-FILE-90-1 |
| AC-FILE-01-10 | 通过 | TC-FILE-01-10 | `TestTC_FILE_01_10_RetryRedoesAll` |  |
| AC-FILE-01-13 | 通过 | TC-FILE-01-13 | `TestTC_FILE_01_13_ThumbErrors`, `TestTC_FILE_02_16_ImageWhitelist` |  |
| AC-FILE-01-15 | 通过 | TC-FILE-01-15 | `TestTC_FILE_01_15_ReplaceDuringVerify` |  |
| AC-FILE-01-19 | 通过 | TC-FILE-01-19 | `TestTC_FILE_01_19_ConcurrentComplete` |  |
| AC-FILE-01-20 | 通过 | TC-FILE-01-20 | `TestTC_FILE_01_20_ContentFromLastAttempt` |  |
| AC-FILE-01-21 | 通过 | TC-FILE-01-21 | `TestTC_FILE_01_21_MaliciousHeaders` |  |
| AC-FILE-01-22 | 通过 | TC-FILE-01-22 | `TestTC_FILE_01_22_Dimensions` |  |
| AC-FILE-01-26 | 通过 | TC-FILE-01-26 | `TestTC_FILE_01_26_CrashLeftovers` |  |
| AC-FILE-01-27 | 通过 | TC-FILE-01-27 | `TestTC_FILE_01_27_LeaseAndRace` |  |
| AC-FILE-02-11 | 通过 | TC-FILE-02-11 | `TestTC_FILE_01_22_Dimensions`, `TestTC_FILE_02_01_SendImage` |  |
| AC-FILE-05-4 | 通过 | TC-FILE-05-04 | `TestTC_FILE_01_03_SniffRejects` |  |
| AC-FILE-02-1 | 部分通过 | TC-FILE-02-01 | `TestTC_FILE_02_01_SendImage` | 端到端 AC，只验了后端部分（下发服务端记录的值、取地址）；客户端显示随 FE-640 |
| AC-FILE-02-2 | 通过 | TC-FILE-02-02 | `TestTC_FILE_02_01_SendImage` |  |
| AC-FILE-02-3 | 通过 | TC-FILE-02-03 | `TestTC_FILE_02_03_BadFileIDs` |  |
| AC-FILE-02-15 | 通过 | TC-FILE-02-15 | `TestMsgBodyExclusive`, `TestTC_FILE_02_15_BodyExclusive` |  |
| AC-FILE-03-1 | 部分通过 | TC-FILE-03-01 | `TestTC_FILE_03_01_GroupFile` | 端到端 AC，只验了后端部分（群成员取地址、下载内容一致）；客户端随 FE-640 |
| AC-FILE-06-4 | 通过 | TC-FILE-06-03 | `TestTC_FILE_06_03_OneMessagePerFile` |  |
| AC-FILE-06-7 | 通过 | TC-FILE-06-06 | `TestTC_FILE_06_06_PurgingCannotBind` |  |
| AC-FILE-06-8 | 通过 | TC-FILE-06-07 | `TestTC_FILE_06_07_ConcurrentBind` |  |
| AC-FILE-07-8 | 通过 | TC-FILE-07-08 | `TestTC_FILE_07_08_ReadyAfterReject` |  |
| AC-FILE-01-25 | 通过 | TC-FILE-01-25 | `TestDisposition`, `TestTC_FILE_01_25_Disposition` |  |
| AC-FILE-04-1 | 通过 | TC-FILE-04-01 | `TestTC_FILE_04_01_08_Access` |  |
| AC-FILE-04-2 | 通过 | TC-FILE-04-02 | `TestTC_FILE_04_02_GroupMembership` |  |
| AC-FILE-04-3 | 通过 | TC-FILE-04-03 | `TestTC_FILE_04_03_HistoryVisible` |  |
| AC-FILE-04-5 | 通过 | TC-FILE-04-05 | `TestTC_FILE_04_05_AdminDeleted` |  |
| AC-FILE-04-7 | 通过 | TC-FILE-04-07 | `TestTC_FILE_04_07_NoSecretsInLogs` |  |
| AC-FILE-04-8 | 通过 | TC-FILE-04-08 | `TestTC_FILE_04_01_08_Access` |  |
| AC-FILE-04-9 | 通过 | TC-FILE-04-09 | `TestTC_FILE_04_09_Batch` |  |
| AC-FILE-05-5 | 通过 | TC-FILE-05-05 | `TestTC_FILE_05_05_PolyglotServedAsImage` |  |
| AC-FILE-01-24 | 通过 | TC-FILE-01-24 | `TestTC_FILE_01_24_PurgeBeatsComplete` |  |
| AC-FILE-06-1 | 通过 | TC-FILE-06-01 | `TestTC_FILE_06_01_Orphans` |  |
| AC-FILE-06-2 | 通过 | TC-FILE-06-02 | `TestTC_FILE_06_02_Expiry` |  |
| AC-FILE-06-5 | 通过 | TC-FILE-06-04 | `TestTC_FILE_06_04_DeleteFailure` |  |
| AC-FILE-06-9 | 通过 | TC-FILE-06-08 | `TestTC_FILE_06_08_NoRetention` |  |
| AC-FILE-06-10 | 通过 | TC-FILE-06-09 | `TestTC_CHAT_90_05_NoSecretsInLogsC2`, `TestTC_FILE_06_02_Expiry` |  |
| AC-FILE-06-11 | 通过 | TC-FILE-06-10 | `TestTC_FILE_04_02_GroupMembership` |  |
| AC-FILE-06-12 | 部分通过 | TC-FILE-06-11 | `TestTC_FILE_06_02_Expiry` | 服务端部分（过期时 modVer 递增，其他会话不变）通过；三套 SDK 重拉随 FE |
| AC-FILE-06-13 | 通过 | TC-FILE-06-12 | `TestTC_FILE_06_12_AdminDeletedPurge` |  |
| AC-ACC-06-3 | 通过 | TC-ACC-06-03 | `TestTC_ACC_06_03_BindAvatar` |  |
| AC-ACC-06-7 | 部分通过 | TC-ACC-06-07 | `TestTC_ACC_06_07_AvatarExposure` | 假 S3 上公开响应只有服务端写的 Content-Type；`nosniff` 是厂商能力，假 S3 上没有，真实存储没验 |
| AC-ACC-06-8 | 通过 | TC-ACC-06-08 | `TestTC_ACC_06_08_11_Replace` |  |
| AC-ACC-06-9 | 通过 | TC-ACC-06-09 | `TestTC_ACC_06_09_StoreDown` |  |
| AC-ACC-06-10 | 通过 | TC-ACC-06-10 | `TestTC_ACC_06_10_UnboundAvatar` |  |
| AC-ACC-06-11 | 通过 | TC-ACC-06-11 | `TestTC_ACC_06_08_11_Replace` |  |
| AC-ACC-06-12 | 通过 | TC-ACC-06-12 | `TestTC_ACC_06_12_CrashAndCleanup` |  |
| AC-ACC-06-13 | 通过 | TC-ACC-06-13 | `TestTC_ACC_06_13_CrashNoRetry` |  |
| AC-ACC-08-7 | 通过 | TC-ACC-08-07 | `TestTC_ACC_06_12_CrashAndCleanup` |  |
| AC-CHAT-90-3 | 部分通过 | TC-CHAT-90-03 | `TestTC_CHAT_90_03_ObjStoreDownText` | 功能部分通过：拒绝连接（端口已关闭）和黑洞（接受连接不响应）各 1000 条文本全部成功、对方都收到，期间 `/readyz` 一直 200，申请 200，complete 503 / 5032 不阻塞发送。ack P99 只在开发机记录（第 4 节），不算通过；5 分钟时长随 P-12 |
| AC-CHAT-90-17 | 通过 | TC-CHAT-90-15,TC-CHAT-90-16 | `TestTC_CHAT_90_15_MetricsAndChecks`, `TestTC_CHAT_90_16_FileMetricsAndChecks` | C1 账号指标和检查项；C2 文件指标 4 个、`objstore_reachable`（e2e）、`objstore_https`（`config.TestObjstoreHTTPSCheck`）；标签取值都在枚举内（`metrics.TestLabelsAreBounded`） |
| AC-FILE-90-1 | 未执行 | TC-FILE-90-01 | `TestTC_FILE_90_01_RealStore` | 真实 MinIO 没有运行（见第 3 节）。用例和脚本已就绪：`TestTC_FILE_90_01_RealStore`（设 `IM_E2E_S3_*` 才运行）、`run-local-minio.sh` 的 `RUN_IM_E2E=1`；同一套检查在假 S3 上跑通（`TestRealStoreChecksOnFake`，不算通过） |
| AC-FILE-90-2 | 未执行 | TC-FILE-90-02 | — | 等云厂商（CH-05）。手工 AC，前端走查部分见 FE-640 |
| AC-CHAT-90-1 | 未执行 | TC-CHAT-90-01 | — | 性能。开发机只记录（第 4 节），不算通过，发布判定等 P-12。C2 的范围和 C1 相同（第 15 节表"不变"） |
| AC-CHAT-90-5 | 部分通过 | TC-CHAT-90-04,TC-CHAT-90-05 | `TestTC_CHAT_90_04_NoSecretsInLogs`, `TestTC_CHAT_90_05_NoSecretsInLogsC2` | im-server 部分通过：C1（TC-CHAT-90-04）和 C2（TC-CHAT-90-05：上传、下载、415、4292、存储不可用、后台删除、过期清理）的日志和数据库（除 `files` 外全部表，含 `admin_audit`）搜不到 token、预签名查询串和签名、文件名、存储密钥。console-server 的 C2 日志和审计随 CS-201 / FE-640 |

## 6. 需要 Frank 决定的事

1. **AC-FILE-90-1 的 MinIO 运行**：请在会话里直接确认是否现在下载并构建固定版本的 MinIO 和 mc（到 scratchpad 临时目录，只监听 127.0.0.1，跑完删除）。确认后按第 3 节运行。
2. **backend-chat.md 问题 28**：最小权限凭证下，不存在的键在云厂商上可能返回 403 而不是 404，会让"临时对象不存在"（4096）变成 500。选定厂商（CH-05）跑 AC-FILE-90-2 时确认；若是 403，二选一：服务端把 403 也按不存在处理，或给 `tmp/` 列举权限。

其余新问题（24～27、29～31）是记录，不需要决定，见 backend-chat.md 文末。
