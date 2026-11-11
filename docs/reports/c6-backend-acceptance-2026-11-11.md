# C6 后端验收报告（BE-499，2026-11-11）

| 项 | 内容 |
|---|---|
| 范围 | PRD-chat v1.7 第 10 节里标了【C6】、验证层是【后端】或端到端（挂了 BE / CS 任务）的 AC，加上其他章节标【C6】的 AC-ACC-01-9、AC-ACC-10-5、AC-CHAT-90-4 和【全部】的 C6 部分（第 15 节累计表 C6 列：AC-CHAT-90-1、90-2、90-5、90-10、90-17；90-3 在 C6 不变，90-15 是"—"，按 BE-499 第 2 条不另跑），共 **59 条**：F-MOD-01 的【后端】9 条、F-MOD-02 的【后端】15 条、F-MOD-03 的【后端】12 条和端到端 1 条（AC-MOD-03-1）、F-MOD-04 的【后端】13 条和端到端 1 条（AC-MOD-04-1），其余 8 条。其他 PRD 没有标【C6】的 AC（BE-497 的 PRD-admin、social、ops、ui 和本次的 PRD-core v1.8 都只改规则说明，AC 不变；PRD-admin 的 AC-ADM-60-1、60-2 已作废）。**前端和手工 AC 不在本报告**（AC-MOD-01-7、02-9、03-9、03-11、04-6、04-12、04-15，FE-698 另做）；端到端 AC 只记服务端部分 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**。im-server：E1（SQLite、`IM_TEST_MODE=1`、`IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=1`、测试里一看就是假的 `IM_REPORT_KEY`，需要时加 `IM_MSG_ACTIONS_ENABLED=1`、进程内假 S3、双节点）；console-server：真实 Postgres 17（临时实例，见第 1 节）+ 真实 im-server 进程（从本工作区构建）+ 假 S3 |
| 基线 | tianting-server（im-mvp）@241bd72、tianting-console-server@9f00aba、tianting-admin@29d24f9（本报告不涉及前端），加上本报告所在的提交（第 3 节的实现修正、用例、PRD-core v1.8 和文档） |
| 结论 | **通过 53、部分通过 4、未执行 2、失败 0**。修了 1 处实现（问题 137：`groupCreate` 的敏感词检查顺序）。性能没有判定（P-12）；真机、部署 e2e 没有做 |
| 没有做的 | 性能判定（P-12：AC-CHAT-90-1 的 TC-CHAT-90-40 没有跑，AC-CHAT-90-4 开发机只记录）；部署 e2e（AC-CHAT-90-5 第二层）；真机（P-13）；客户端和后台页面（FE-698） |

## 1. 运行

| 命令 | 结果（本地） |
|---|---|
| `server/`：`go vet ./...`、`gofmt -l .` | 无问题 |
| `server/`：`go test -count=1 -timeout 30m ./...` | 全部包通过（e2e 170 秒）。默认配置下 `IM_MOD_ENABLED` 等新开关都关，这一次同时是 TC-CHAT-90-41 第 1 步（现有 e2e 不改代码全过） |
| `server/`：`go test -race -count=1 -timeout 50m ./...` | 全部包通过（e2e 672 秒），没有 DATA RACE。举报限额的并发用例、并发结案、`moderation` 的并发匹配和重新加载都在里面 |
| `server/`：`go test -count=1 -run TestTC_CHAT_90_42_MatchP99 -v ./internal/moderation` | 开发机记录见第 4 节，不判定 |
| `server/`：`go test -count=1 -run TestCorpusCompactFalsePositives -v ./internal/moderation` | 语料误判记录见第 5 节，不判定 |
| tianting-console-server：`go vet ./...` | 无问题 |
| tianting-console-server：`go test -race -count=1 -timeout 40m ./...`（`CON_TEST_REQUIRE_DB=1`，临时 Postgres） | 第 1 次：e2e 1 个失败（`TestCON03Timing`，见下面"偶发失败"），其余全部通过；第 2 次（不改任何东西，整个 console 全量重跑）：全部包通过（e2e 394 秒），没有 DATA RACE、没有 SKIP。console e2e 里的 im-server 是从本工作区（含第 3.1 节的修正）构建的真实进程 |
| `python3 -I tools/trace.py --check` | AC 1694，gaps 0（PRD-core 换成 v1.8，128 条不变） |

**偶发失败**：1 个，和 C6 无关。console-server 第 1 次全量的 `TestCON03Timing`（AC-CON-03 的登录耗时一致性：不存在的账号和真实账号密码错误时，50 次交替登录的中位数差不能超过 5%）报 ghost 1.502 秒、real 1.590 秒、差 5.50%，超过 5% 一点。当时机器上同时有 Android 模拟器（qemu 约 7 个核）和 iOS 的 `xcodebuild test` 在跑；这个用例只测 console 的 Argon2 登录路径，C6 没有改 console-server，也没有动登录。**断言没有放宽**，不改任何东西整个 console 全量重跑一次，全部通过。im-server 的 `go test ./...` 和 `-race` 全量各一次，都没有失败。

**临时 Postgres**：`brew` 已装的 postgresql@17（P-5），`LC_ALL=C initdb -E UTF8 --locale=C -U postgres --auth=trust` 到会话 scratchpad，只监听 `127.0.0.1:55491`、不开 Unix socket、`lc_messages=C`，环境里 `LC_ALL=C`；跑完 `pg_ctl stop` 并删除数据目录。

## 2. 本次新增和修改的用例

任务单第 1 条要的 `mod_*_test.go` 在 BE-492～498 时已经写了（`mod_switch_test.go`、`mod_words_test.go`、`mod_send_test.go`、`mod_report_test.go`、`mod_handle_test.go`、`mod_deploy_test.go`），这次逐个核对 TC-MOD 里层级含"后端 e2e"的用例都有对应函数（有几个合在一个函数里，函数注释写了编号：TC-MOD-01-02 在 `TestTC_MOD_01_01_AddDisableEnable`，01-09 在 `TestTC_MOD_01_04_Import`，02-03 在 `TestTC_MOD_02_02_VariantsAndOriginalText`，02-11 在 `TestTC_CHAT_90_41_ModSwitchOff`，03-03 在 `TestTC_MOD_03_01_ReportAndIdempotency`，03-08 在 `TestTC_MOD_03_05_GroupEvidenceEncrypted`，04-02 的 im-server 一侧在 `TestTC_MOD_04_04_Scopes`，04-10 / 04-11 的 im-server 一侧在 `TestTC_MOD_04_16_EvidenceFiles`），没有重复写。补的：

- `server/e2e/mod_markers_test.go`（**特征串检查单独一个文件**，BE-499 第 1 条）：`TestTC_CHAT_90_43_ModMarkers`（TC-CHAT-90-43 的 im-server 一侧，E1 + 假 S3 + C4，DEBUG 日志）。加词 `ZXQWORD`、"违规词"；好友附言被拦截一次（`ZXQ-MOD-APPLY-1`，发给存在和不存在的人）、成功一次（`ZXQ-MOD-APPLY-2`）；单聊、群聊发被拦截的 `ZXQ-MOD-BODY`；举报群文本 `ZXQ-RPT-1`、群文件 `ZXQ-MOD-FILE.pdf`、单聊 `ZXQ-RPT-2`（`off`）和用户，`note` 都带 `ZXQ-MOD-NOTE`；审核员看详情、授权、读证据（读到原文、`note`、文件名是对照）、`fetch` 文件、结案、查命中。断言：
  - 日志和 `admin_audit` 搜不到 8 个特征串，`deploy/ops/scan-logs --marker=ZXQWORD --marker=ZXQ-MOD --marker=ZXQ-RPT` 再扫一遍；对照：日志有 `content blocked` 和 `reportId`，审计有 `reportId`、`fileId`、授权前 8 位，扫描工具对植入的特征串会报错；
  - 检查点后主库和 WAL 的原始字节搜不到 `ZXQ-MOD-BODY`、`ZXQ-MOD-APPLY-1`、`ZXQ-MOD-NOTE`（对照：群消息原文在）；
  - 逐表：`ZXQWORD` 只在 `blocked_words`，`ZXQ-RPT-1`、`ZXQ-RPT-2` 只在 `messages`、`conv_seq`（原消息和会话最后一条），`ZXQ-MOD-FILE` 只在 `files`，`ZXQ-MOD-APPLY-2` 只在 `friend_requests`；`reports`、`report_evidence`、`moderation_hits`、`admin_audit` 都没有；
  - 反证：让举报日志带上 `note` 的内容，用例报"logs contain ZXQ-MOD-NOTE"、`scan-logs` 命中 4 处；改回后通过。
  - console-server 一侧（console 日志、console 全库含 console 审计、im-server 审计）在 console 的 `TestTC_MOD_01_05_06_WordsViaConsole`、`TestTC_MOD_04_10_11_ReportMeta`、`TestTC_MOD_04_01_ReviewFlow`、`TestTC_MOD_04_12_16_EvidenceFiles` 的 `noneContains`，本次 console 全量里跑过，没有另写。
- `server/e2e/mod_send_test.go`：
  - `TestTC_MOD_02_05_GroupCreateOrder`（问题 137，第 3 节）：重试已经成功的建群（群名后来进了词库）返回第一次的 `groupId`、不记命中；邀请非好友 + 群名含词是 4035（带 `userId`，和群名不含词时逐字节相同），不是 4510、不记命中；都是好友时 4510、记一条 `groupName` 命中、建群配额退还；配额用完是 4292。3 处反证：改回旧顺序（判重之前查）、检查放到被邀请人判定之前、4510 不退配额，各自让用例失败。
  - `TestTC_CHAT_90_47_BlockedScenes`（TC-CHAT-90-47 第 1 步，原来只验了 `text`）：7 个场景（第 110 条）各命中一次，每个 `im_blocked_total{scene}` 正好加 1，注册另计 `im_registers_total{result="blocked"}`；`im_blocked_total` 只有 `scene` 一个标签、正好 7 个序列。
- `server/internal/moderation/corpus_test.go`：`TestCorpusCompactFalsePositives`（R-CS-17 的语料误判，第 5 节），只记录，`-short` 跳过。
- `docs/testcases/TC-MOD.md` TC-MOD-02-05：第 3 步去掉"`groupCreate` 带含词的公告"（问题 129：`groupCreate` 没有公告字段，按 `groupUpdate` 改公告覆盖），加第 3a 步建群的检查顺序（问题 137）；第 4 步字段名 `note` 改为协议的 `message`。

## 3. 修了什么

### 3.1 问题 137：`groupCreate` 的敏感词检查顺序（已修）

**现象**（FE-690 实测）：`groupCreate` 在开关和人数之后、进存储层之前就查群名，所以邀请了不是好友的人时，群名含词照样返回 4510；改群名、公告（`groupUpdate`）是在存储层同一事务的群权限判定之后查。另外判重也在存储层，旧顺序下"重试已经成功的建群、群名后来进了词库"会得到 4510，而不是第一次的结果。

**PRD 怎么写的**：F-MOD-02 规则 5"判定位置见第 4.3 节第 9 步：**在已有的所有权限判定之后**，避免让被拒绝的用户探测词库"；backend-chat 第 112 条（P-48"都按建议"已定）写明"改资料、**建群**、改群：各自的权限判定之后"；第 4.3 节发送的顺序里，限流、对象、权限都在第 9 步之前，判重（第 2 步）也在之前。建群的被邀请人校验（只能拉好友，4035 带 `userId`）就是建群的权限判定。所以 PRD 已经写定，**不是新的产品决定**，按 PRD 统一。

**两个关注点**：
- **会不会借此探测好友关系**：不会多透露。被邀请人不是好友时 4035 本来就带 `userId`（F-SOC-21 已有的行为，和群名无关）；新顺序下群名含不含词，这个 4035 逐字节相同（用例断言）。4510 只在全部被邀请人都通过之后才出现，不提供 4035 以外的关系信息。旧顺序下 4510 反而盖住了 4035，但这不是在保护什么：同一个人不带词再发一次就能拿到 4035。
- **会不会泄露词库**：旧顺序会。被权限拒绝的请求（不是好友、加群上限、配额用完）也能拿到 4510，等于让被拒绝的人试探词库，违反规则 5。新顺序下只有本来就能建群的人才看得到 4510，和发消息、改群名一样。

**改法**：`store.CreateClientGroupParams` 加 `Check func() error`，`CreateClientGroup` 在判重、配额、自己的加群上限、全部被邀请人判定之后、写入之前调用（和 `UpdateGroupProfile` 的 `check` 同一个做法）；`logic.groupCreate` 传进去的回调只做匹配（事务里只计算），命中时事务回滚，事务之后退还建群配额、写命中记录（SQLite 只有一个写者）、返回 4510。只查客户端写的群名，没写名字时服务端生成的不查（规则 3）。没有改 PRD、AC 和协议。涉及文件：`server/internal/store/group.go`、`server/internal/store/sqlite_group.go`、`server/internal/logic/group.go`、`server/internal/logic/content.go`（注释）。

### 3.2 问题 136：PRD-core 升 v1.8（已做）

PRD-core v1.7 的 F-MSG-01 规则 6 只写到 PRD-chat 第 4.3 节第 7、8 步，第 7 节还写着"敏感词与内容审核 P2"。新建 `docs/prd/PRD-core-v1.8.md`：F-MSG-01 规则 6 加"PRD-chat v1.7 变更（C6）"说明（第 9 步敏感词 4510、放在所有权限判定之后、已扣令牌并退还 `newPeerReject`、去重优先；格式一步 `customDisabled`；群聊发送同一位置），第 7 节那一行改为指向 PRD-chat C6，末尾加"v1.8 变更记录"；v1.7 标"已被 v1.8 取代"；`tools/trace.py` 的 `PRDS` 和说明改为 v1.8，`traceability.md` 重新生成（只有 `core-v1.7` → `core-v1.8`）。**128 条 AC 全部不变**。全仓库没有别的文件链接 `PRD-core-v1.7.md`（v1.6 的"已被 v1.7 取代"是历史记录，不改）。

### 3.3 其他

核对中没有发现别的实现和 PRD 不符的地方。

## 4. 性能（开发机，只记录，不判定，P-12）

TC-CHAT-90-42（AC-CHAT-90-4）：10 万个现场随机生成的词（不落盘）、5000 码点不命中的文本、单线程 10000 次（norm 和 compact 两遍都算），darwin/arm64 10 核，本次跑了两次：

| 次 | P50 | P99 | 最大 | 构建 | 自动机内存 |
|---|---|---|---|---|---|
| 1 | 0.84 ms | 7.49 ms | 37.5 ms | 434 ms | 约 9 MiB |
| 2 | 0.83 ms | 7.71 ms | 39.8 ms | 236 ms | 约 9 MiB |

**P99 超过 2 ms 的目标（未校准）**，而 BE-493 在同一台机器上记录的是 P99 1.87 ms。两次测量时机器上同时有别的任务在跑（Android 模拟器 qemu 占用约 7 个核、iOS 的 `xcodebuild test`），P50 和 BE-493 几乎一样（0.84 / 0.87 ms），尾部变差和负载吻合。开发机结果本来就不作判定（P-12）；在目标机上判定时要保证没有别的负载。没有改实现。

TC-CHAT-90-40（AC-CHAT-90-1 的 C6 部分：开词库后的 ack P99）没有跑，等 P-12 的目标机。

## 5. 标准化的语料误判（R-CS-17，BE-493 第 3 条）

仓库里没有真实语料和真实词库（F-MOD-01 规则 5），用**特征语料**测：语料是现行版本 PRD 的正文（PRD-chat v1.7、core v1.8、social v1.3、ui v1.6、admin v1.5、ops v1.6，非空行 7988 行，中文为主、夹英文和代码），先去掉原样出现的特征词；第二遍（compact，去掉空白和标点再匹配）多出来的命中就是它引起的误判。结果（`TestCorpusCompactFalsePositives`）：

| 词库 | norm 命中的行 | 只有 compact 命中的行 | 占 norm 不命中的行 |
|---|---|---|---|
| 特征词 3 个（违规词、badword、ZXQWORD） | 1 | 1 | 0.01% |
| 随机 2 个汉字 × 50 | 4139 | 3 | 0.08% |
| 随机 3 个汉字 × 200 | 2823 | 5 | 0.10% |
| 随机英文单词（4～8 个字母）× 200 | 4531 | 0 | 0.00% |

特征词那 1 行是 PRD 里举的例子"违 规 词"本身（本来就该拦）。随机词的几行是词跨过标点或空格（如表格的 `|`、括号、顿号两边）拼出来的。**结论只说明量级**：compact 多出来的误判在千分之一左右，2 个字的中文词最容易误判；真实词库来了以后（CH-19）应该用真实语料再测一次，词库负责人维护时尽量不收 2 个字的常用词。

## 6. 逐条

| AC | 层 | 结论 | 用例编号 | 依据（测试函数；console 的标 console） | 说明 |
|---|---|---|---|---|---|
| AC-MOD-01-1 | 后端 | 通过 | TC-MOD-01-01 | `TestTC_MOD_01_01_AddDisableEnable` | 10 秒内 4510，`sync` 没有、`conv_seq` 没增加 |
| AC-MOD-01-2 | 后端 | 通过 | TC-MOD-01-02 | `TestTC_MOD_01_01_AddDisableEnable` | 停用后 10 秒内成功，u2 收到原文 |
| AC-MOD-01-3 | 后端 | 通过 | TC-MOD-01-03 | `TestTC_MOD_01_03_TwoNodes`、`TestModWordsTTLFallback` | 时钟冻结，靠 `blockedWords` 广播；TTL 兜底另测 |
| AC-MOD-01-4 | 后端 | 通过 | TC-MOD-01-04 | `TestTC_MOD_01_04_Import`、console `TestTC_MOD_01_05_06_WordsViaConsole` | 5001 条 400 / 4131、词库不变（console 不调 im-server）；100 条 `{added:80, duplicate:10, invalid:10}`，按 `norm` 去重 |
| AC-MOD-01-5 | 后端 | 通过 | TC-MOD-01-05 | console `TestTC_MOD_01_05_06_WordsViaConsole`、`TestModWordsListAndScope` | support 403 / 4030、不调用 im-server；im-server 没有 scope 的调用方 403 |
| AC-MOD-01-6 | 后端 | 通过 | TC-MOD-01-06 | `TestTC_MOD_01_06_AuditHasNoWords`、`TestTC_CHAT_90_43_ModMarkers`、console `TestTC_MOD_01_05_06_WordsViaConsole` | 两边审计有操作记录（wordId、数量），console 全库、两边日志、`admin_audit` 搜不到 `ZXQWORD` |
| AC-MOD-01-8 | 后端 | 通过 | TC-MOD-01-08 | `TestTC_MOD_01_08_ReloadFailure`、`TestTC_MOD_01_08_StartupLoadFailure`、`TestModWordsDynamicCheck` | 三种都验：旧词库生效、`im_mod_reload_errors_total` 增加、解除后 10 秒内生效；启动失败（注入和真的读不了两种）；空词库告警 |
| AC-MOD-01-9 | 后端 | 通过 | TC-MOD-01-09 | `TestTC_MOD_01_04_Import` | 65 个码点计入 `invalid`；单个添加 400 / 4000（第 102 条） |
| AC-MOD-01-10 | 后端 | 通过 | TC-MOD-01-10 | config `TestTC_MOD_01_10_ProductionNeedsV13AndNotice`、`TestTC_MOD_01_10_ReportKeyChecks`、`TestTC_MOD_01_10_ProductionBinary`、`TestTC_MOD_01_10_AppNeedsReportKey` | 真实二进制非 0 退出，指出缺 `IM_V13_ADMIN_FEATURES`，不打印密钥 |
| AC-MOD-02-1 | 后端 | 通过 | TC-MOD-02-01 | `TestTC_MOD_02_01_BlockedNotStored` | 哨兵法；`sync` 和元数据都没有 |
| AC-MOD-02-2 | 后端 | 通过 | TC-MOD-02-02 | `TestTC_MOD_02_02_VariantsAndOriginalText`、`moderation.TestNormalizeVariants` | 6 种写法都 4510 |
| AC-MOD-02-3 | 后端 | 通过 | TC-MOD-02-03 | `TestTC_MOD_02_02_VariantsAndOriginalText` | 下发和 `sync` 逐字节是原文 |
| AC-MOD-02-4 | 后端 | 通过 | TC-MOD-02-04 | `TestTC_MOD_02_04_FileNames` | 查服务端记录的文件名（第 111 条），文件仍是 `ready` |
| AC-MOD-02-5 | 后端 | 通过 | TC-MOD-02-05 | `TestTC_MOD_02_05_OtherScenes`、`TestTC_MOD_02_05_GroupCreateOrder` | 注册（含已占用的用户名同样 4510）、改昵称、建群、改群名和公告、附言；附言响应和对方是否存在无关；建群顺序见第 3.1 节；公告按 `groupUpdate`（第 129 条） |
| AC-MOD-02-6 | 后端 | 通过 | TC-MOD-02-06 | `TestTC_MOD_02_06_CustomDisabled` |  |
| AC-MOD-02-7 | 后端 | 通过 | TC-MOD-02-07 | `TestTC_MOD_02_07_PermissionFirst` | 4036、4031，没有命中记录 |
| AC-MOD-02-8 | 后端 | 通过 | TC-MOD-02-08 | `TestTC_MOD_02_08_HitRecord`、`TestTC_CHAT_90_43_ModMarkers` |  |
| AC-MOD-02-10 | 后端 | 通过 | TC-MOD-02-10 | `TestTC_MOD_02_10_SwitchOffNoCheck` |  |
| AC-MOD-02-11 | 后端 | 通过 | TC-MOD-02-11 | `TestTC_CHAT_90_41_ModSwitchOff` |  |
| AC-MOD-02-12 | 后端 | 通过 | TC-MOD-02-12 | `TestTC_MOD_02_12_TokensAndNewPeerRefund` |  |
| AC-MOD-02-13 | 后端 | 通过 | TC-MOD-02-13 | `TestTC_MOD_02_13_OtherRefunds` |  |
| AC-MOD-02-14 | 后端 | 通过 | TC-MOD-02-14 | `TestTC_MOD_02_14_ResendHitsPurge`、console `TestTC_MOD_02_14_HitsViaConsole` | 去重优先；按用户查次数（没有内容）；91 天后清除 |
| AC-MOD-02-15 | 后端 | 通过 | TC-MOD-02-15 | config `TestTC_MOD_02_15_CustomWarning` | 配置检查 `moderation` 项告警（`GET /internal/v1/config` 用的同一个 `Summary`） |
| AC-MOD-02-16 | 后端 | 通过 | TC-MOD-02-16 | `TestTC_MOD_02_16_MentionAll` |  |
| AC-MOD-03-1 | 端到端 | 部分通过 | TC-MOD-03-01 | `TestTC_MOD_03_01_ReportAndIdempotency`、`TestModReportQueue`、console `TestTC_MOD_03_01_QueueViaConsole` | 服务端部分通过：成功、队列出现（im-server 和经 console 两层）、u1 2 秒内只收到哨兵。客户端举报操作随 FE-698 |
| AC-MOD-03-2 | 后端 | 通过 | TC-MOD-03-02 | `TestTC_MOD_03_02_TargetInvalid` | 三种 4040 响应逐字节相同；21 条 4131 |
| AC-MOD-03-3 | 后端 | 通过 | TC-MOD-03-03 | `TestTC_MOD_03_01_ReportAndIdempotency` | 3 次同一个 `reportId`，1 条记录 |
| AC-MOD-03-4 | 后端 | 通过 | TC-MOD-03-04 | `TestTC_MOD_03_04_DailyLimit`、`TestModReportQuotaConcurrent` | 并发也不超（`-race` 全量里也跑） |
| AC-MOD-03-5 | 后端 | 通过 | TC-MOD-03-05 | `TestTC_MOD_03_05_GroupEvidenceEncrypted`、`TestTC_MOD_04_01_ReviewFlow`、console `TestTC_MOD_04_01_ReviewFlow` | 主库和 WAL 原始字节搜不到；审核员（im-server 和经 console）读到原文 |
| AC-MOD-03-6 | 后端 | 通过 | TC-MOD-03-06 | `TestTC_MOD_03_06_Purge` | 179 天还在，181 天证据和 `reports` 行一起删除（P-48 ②） |
| AC-MOD-03-7 | 后端 | 通过 | TC-MOD-03-07 | `TestTC_MOD_03_07_UserReports` | 没有关联、不存在、自己：响应逐字节相同 |
| AC-MOD-03-8 | 后端 | 通过 | TC-MOD-03-08 | `TestTC_MOD_03_05_GroupEvidenceEncrypted`、`TestTC_CHAT_90_43_ModMarkers`、console `TestTC_MOD_04_01_ReviewFlow`、`TestTC_MOD_04_10_11_ReportMeta` | 两边日志、两边审计、库搜不到 `note` |
| AC-MOD-03-10 | 后端 | 通过 | TC-MOD-03-10 | `TestTC_MOD_03_10_C2COffMetadataOnly`、`TestTC_MOD_04_03_C2COff` | 只有元数据；同一个库用 `on` 重启之后仍没有正文 |
| AC-MOD-03-12 | 后端 | 通过 | TC-MOD-03-12 | `TestTC_MOD_03_12_Recalled` |  |
| AC-MOD-03-13 | 后端 | 通过 | TC-MOD-03-13 | `TestTC_MOD_03_13_GroupReports` |  |
| AC-MOD-03-14 | 后端 | 通过 | TC-MOD-03-14 | `TestTC_MOD_03_14_EvidenceWriteFails`、`TestTC_CHAT_90_41_ModSwitchOff` | 500 / 5000 没有记录、不计次数；开关关 403 `featureDisabled` |
| AC-MOD-03-15 | 后端 | 通过 | TC-MOD-03-15 | `TestTC_MOD_03_15_FileEvidenceRetention`、`TestTC_MOD_04_16_EvidenceFiles` | 保留期内照常；过期后普通用户 4048，证据文件仍可取 |
| AC-MOD-04-1 | 端到端 | 部分通过 | TC-MOD-04-01 | `TestTC_MOD_04_01_ReviewFlow`、console `TestTC_MOD_04_01_ReviewFlow` | 服务端部分通过：经 console 读到原文和水印用的授权前 8 位、消息被删除、状态 `handled`，console 审计依次 `report.view`、`report.evidence.grant`、`msg.delete`、`report.handle`，im-server 审计路由顺序对。管理后台页面随 FE-698 |
| AC-MOD-04-2 | 后端 | 通过 | TC-MOD-04-02 | console `TestTC_MOD_04_02_ReportRoles`、`TestTC_MOD_04_04_Scopes` | 都不调用 im-server |
| AC-MOD-04-3 | 后端 | 通过 | TC-MOD-04-03 | `TestTC_MOD_04_03_C2COff`、console `TestTC_MOD_04_03_C2COff` | 403 / 4030 `c2cBodyDisabled`，不产生授权；元数据照常 |
| AC-MOD-04-4 | 后端 | 通过 | TC-MOD-04-04 | `TestTC_MOD_04_04_Scopes`、console `TestTC_MOD_04_04_09_EvidenceCallers` |  |
| AC-MOD-04-5 | 后端 | 通过 | TC-MOD-04-05 | `TestTC_MOD_04_05_ConcurrentResolve`、console `TestTC_MOD_04_05_14_Resolve` | 并发 20 次 |
| AC-MOD-04-7 | 后端 | 通过 | TC-MOD-04-07 | `TestTC_MOD_04_07_NoFrames` | 哨兵法 |
| AC-MOD-04-8 | 后端 | 通过 | TC-MOD-04-08 | adminauth `TestTC_MOD_04_08_ReportEvidenceIsBodyScope`、`TestTC_MOD_04_08_CallerConfigBinary` | 真实二进制非 0 退出，指出正文类 scope 的限制 |
| AC-MOD-04-9 | 后端 | 通过 | TC-MOD-04-09 | console `TestTC_MOD_04_04_09_EvidenceCallers` | 超管没有应急授权 403，不调用 im-server |
| AC-MOD-04-10 | 后端 | 通过 | TC-MOD-04-10 | console `TestTC_MOD_04_10_11_ReportMeta`、`TestTC_MOD_04_16_EvidenceFiles` | 不填理由 400、第 51 个 429（和单聊元数据配额合并）；没有授权时没有 `note` 和文件名 |
| AC-MOD-04-11 | 后端 | 通过 | TC-MOD-04-11 | console `TestTC_MOD_04_10_11_ReportMeta`、`TestTC_MOD_04_16_EvidenceFiles` | 两层响应都搜不到文件名 |
| AC-MOD-04-13 | 后端 | 通过 | TC-MOD-04-13 | `TestTC_MOD_04_13_C2CFileOff`、console `TestTC_MOD_04_13_C2CFileOff` |  |
| AC-MOD-04-14 | 后端 | 通过 | TC-MOD-04-14 | `TestTC_MOD_04_14_WrongKeyAndAuditFailure`、console `TestTC_MOD_04_14_Undecryptable`、`TestTC_MOD_04_05_14_Resolve` | 无法解密时可以结案、指标加 1；console 审计写失败 503 / 5031 未结案（im-server 自己的是 5030，第 133 条） |
| AC-MOD-04-16 | 后端 | 通过 | TC-MOD-04-16 | `TestTC_MOD_04_16_EvidenceFiles`、console `TestTC_MOD_04_12_16_EvidenceFiles` | 没有授权、别的举报、别的操作人、过期都 403 / 4039 |
| AC-MOD-04-17 | 后端 | 通过 | TC-MOD-04-17 | `TestTC_MOD_04_17_AccountEventsPurge` |  |
| AC-ACC-01-9 | 后端 | 通过 | TC-ACC-01-22 | config `TestTC_ACC_01_22_ProductionOpenChecks`、`TestTC_ACC_01_22_OpenNeedsWords`、`TestTC_ACC_01_22_ProductionOpenBinary` | 四种缺项都非 0 退出并指出缺哪一条 |
| AC-ACC-10-5 | 后端 | 通过 | TC-ACC-10-05 | config `TestTC_MOD_01_10_ProductionNeedsV13AndNotice`、`TestTC_MOD_01_10_ProductionBinary` |  |
| AC-CHAT-90-1 | 后端（全部） | 未执行 | TC-CHAT-90-40 | — | 性能。C6 部分（开词库后的 ack P99）没有跑，等 P-12 |
| AC-CHAT-90-2 | 后端（全部） | 部分通过 | TC-CHAT-90-41 | `TestTC_CHAT_90_41_ModSwitchOff`、本次 `go test ./...` | C6 部分：默认配置（`IM_MOD_ENABLED` 等新开关都关）下现有 e2e 不改代码全过；`report` 403 / 4030 `featureDisabled`、`features.moderation=false`、`custom` 照常。TC-PERF 判定等 P-12 |
| AC-CHAT-90-4 | 后端 | 未执行 | TC-CHAT-90-42 | `moderation.TestTC_CHAT_90_42_MatchP99` | 性能。开发机只记录（第 4 节：本次有别的负载，P99 7.5～7.7 ms，超过 2 ms 的目标；BE-493 时是 1.87 ms），不算通过也不算失败，等 P-12 |
| AC-CHAT-90-5 | 后端（全部） | 部分通过 | TC-CHAT-90-43 | `TestTC_CHAT_90_43_ModMarkers`、console 的 `noneContains`（见第 2 节） | C6 部分的后端 e2e 通过（举报备注、好友附言、被拦截的正文、文件名、词）。部署 e2e（真实部署的日志）未执行，同 C1～C5 |
| AC-CHAT-90-10 | 后端（全部） | 通过 | TC-CHAT-90-46 | logic `TestTC_CHAT_90_46_C6FrameVectorsServerParse`、protocol `TestReportReqServerOnlyLimits` | 服务端用真实解析跑 `frames.json` 的 C6 用例通过。三套 SDK：FE-690（sdk-ts，含 Schema 校验）、FE-691（sdk-kotlin）、FE-692（sdk-swift）的记录写明 C6 用例 13 个合法、32 个非法通过；三套 SDK 的 `frames.json` 副本和服务端逐字节相同（本次 `cmp`）。SDK 没有在本次重跑。本地结果 |
| AC-CHAT-90-17 | 后端（全部） | 通过 | TC-CHAT-90-47 | `TestTC_CHAT_90_47_BlockedScenes`、`TestModMetricsExposed`、`TestModReportsPendingMetric`、`TestTC_MOD_01_08_ReloadFailure`、`TestModWordsDynamicCheck`、config `TestTC_MOD_01_10_ReportKeyChecks`、`TestTC_MOD_02_15_CustomWarning`、metrics `TestLabelsAreBounded`、moderation `TestScenesMatchMetrics` | 7 个场景各加 1；`im_mod_reload_errors_total` 增加；25 小时前的待处理举报 `im_reports_pending=1`；配置检查有开关状态、词库是否为空（只有数量）、`IM_REPORT_KEY` 已设置（不含值）、`custom` 告警；标签没有 userId、wordId、reportId |

"通过"一栏没有写说明的，是用例按 AC 原文逐项断言、本次全量（含 `-race`）通过。AC-KEY-02-2、PRD-admin AC-ADM-05-1 的参数化表随 C6 的 scope 和路由扩展（`TestEveryAdminRouteChecksScope`、console 的路由矩阵），本次全量里复跑通过（不计入上表）。

## 7. 汇总

| 结论 | 数量 | AC |
|---|---|---|
| 通过 | 53 | AC-MOD-01-1～01-6、01-8～01-10；AC-MOD-02-1～02-8、02-10～02-16；AC-MOD-03-2～03-8、03-10、03-12～03-15；AC-MOD-04-2～04-5、04-7～04-11、04-13、04-14、04-16、04-17；AC-ACC-01-9、AC-ACC-10-5；AC-CHAT-90-10、90-17 |
| 部分通过 | 4 | AC-MOD-03-1、AC-MOD-04-1（端到端，客户端和后台页面部分随 FE-698）；AC-CHAT-90-2（TC-PERF 等 P-12）；AC-CHAT-90-5（部署 e2e 未执行） |
| 未执行 | 2 | AC-CHAT-90-1、AC-CHAT-90-4（性能，等 P-12；90-4 开发机有记录） |
| 失败 | 0 | — |

## 8. 待决定事项的实现状态和上线门槛

- 第 104、108、118、119、121 条：Frank 2026-11-11 已定（P-48，"都按建议"），都按建议实现：第 104 条 `on` 时单聊证据只要 `report.evidence` 的授权流程（用例覆盖 `off`，console 的 `TestTC_MOD_04_13_C2CFileOff` 也走了 `on`）；第 108 条开关关掉后只拦客户端的 `report`（`TestTC_CHAT_90_41_ModSwitchOff`）和敏感词检查（`TestTC_MOD_02_10_SwitchOffNoCheck`），后台的词库、命中、举报接口不跟开关注册（`TestEveryAdminRouteChecksScope` 在默认开关关的环境里遍历到这些路由），`report_purge`、`moderation_hits_purge` 不跟开关注册（`internal/app/app.go` 读代码核对，没有"开关关时跑清理"的单独用例）；第 118 条用户、群举报只存元数据，`note` 加密，用户举报和单聊消息举报的 `note` 按单聊对待（`TestTC_MOD_03_07_UserReports`、`TestTC_MOD_04_03_C2COff`）；第 119 条 180 天和证据一起删除整条举报（`TestTC_MOD_03_06_Purge`）；第 121 条命中统计只对 `report.read` 开放（`TestTC_MOD_02_14_ResendHitsPurge`、console `TestTC_MOD_02_14_HitsViaConsole`）。
- **上线门槛**（`deploy/MODERATION-RUNBOOK.md` 第 6 节 ①～⑪，BE-498 第 4 条）：**都未确认**。开发、测试只用特征词，生产打开 `IM_MOD_ENABLED=1` 前要逐条确认，其中 ①②（词库负责人、举报处理人）、③（PRD-admin 第 12 节 ②③④）、⑪（渗透测试）要 Frank 或外部完成。

## 9. 开放项

1. **P-12**：AC-CHAT-90-1（TC-CHAT-90-40）、AC-CHAT-90-4 的判定，AC-CHAT-90-2 的 TC-PERF。
2. **FE-698**：两条端到端 AC 的客户端和后台页面部分，以及本报告不含的前端 AC。
3. 部署 e2e（AC-CHAT-90-5 第二层）、真机（P-13）没有做。
4. 真实词库来了以后用真实语料再测一次误判（第 5 节）。
5. C3 恢复时 BE-459 补一条哨兵用例：4510 拦下的消息、举报、结案都不进推送队列（第 94 条）。
6. 本次没有新的待 Frank 决定的事（问题 137 按 PRD 规则 5 和第 112 条统一，不是新决定）。
