# C4 后端验收报告（BE-480，2026-11-10）

| 项 | 内容 |
|---|---|
| 范围 | PRD-chat v1.6 里标了【C4】、验证层是【后端】或端到端（挂了 BE / CS 任务）的 AC，加上【全部】里挂在 C4 任务（BE-470～BE-481、CS-202、CS-204）上的【后端】AC，共 **61 条**（`tools/trace.py` 的 AC 表和 backend-chat.md 的 `- AC：` 行交叉得到）。【全部】只验 C4 增加的部分（第 15 节累计表）。**前端和手工 AC 不在本报告**（FE-672 另做）；端到端 AC 只记后端部分 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**。im-server：E1（SQLite、`IM_TEST_MODE=1`、`IM_MSG_ACTIONS_ENABLED=1`、测试隔离密钥）、需要时加进程内假 S3；console-server：真实 Postgres 17.11（临时实例，见第 1 节）+ 真实 im-server 进程 + 假 S3 |
| 基线 | tianting-server（im-mvp）@0bd02a1、tianting-console-server@4b6b9e2，加上本报告所在的提交和 tianting-console-server@b11828d（只有用例和文档，没有改实现） |
| 结论 | **通过 49、部分通过 8、未执行 4（其中等 C3 的 2 条）、失败 0**。性能没有判定（P-12）；真机、云厂商、部署 e2e 没有做 |
| 没有做的 | 真机（P-13）；性能判定（P-12，开发机只记录，第 4 节）；推送衔接（C3 暂停，BE-481）；云厂商对象存储（CH-05）；部署 e2e（TC-CHAT-90-29 第二层） |

## 1. 运行

| 命令 | 结果（本地） |
|---|---|
| `server/`：`go vet ./...`、`gofmt -l .` | 无问题 |
| `server/`：`go test -count=1 ./...` | 全部包通过（e2e 124 秒）。默认配置下 `IM_MSG_ACTIONS_ENABLED` 等新开关都关，这一次同时是 TC-CHAT-90-28 第 1 步（现有 e2e 不改代码全过） |
| `server/`：`go test -race -count=1 ./e2e/ ./internal/logic/ ./internal/store/... ./internal/config/ ./internal/quarantine/ ./internal/protocol/ ./internal/api/ ./internal/metrics/ ./internal/admin/ ./internal/files/ ./internal/scheduler/` | 全部通过（e2e 433 秒） |
| `server/`：`go test -race -count=1 -run 'TestTC_RCL_01_01\|TestTC_CHAT_90_29' ./e2e/` | 通过（全量开始后又改了 TC-RCL-01-01 的断言，单独再跑一次） |
| `server/`：`IM_PERF_DEV=1 go test -count=3 -run 'TestTC_CHAT_90_27\|TestTC_MEN_02_03' ./e2e/` | 3 次都通过，数字见第 4 节 |
| tianting-console-server：`go test -race -count=1 ./...`（`CON_TEST_REQUIRE_DB=1`，临时 Postgres） | 全部包通过（e2e 247 秒），Postgres 不可用时不会跳过 |
| `python3 -I tools/trace.py --check --summary` | gaps 0 |

**偶发失败**：本次没有出现，没有重跑过任何失败的用例。

**临时 Postgres**：`brew` 已装的 postgresql@17（P-5），`LC_ALL=C initdb --locale=C -U postgres --auth=trust` 到会话 scratchpad，只监听 `127.0.0.1:55491`、不开 Unix socket（路径超过 103 字节）、`lc_messages=C`；跑完 `pg_ctl stop` 并删除数据目录。

## 2. 本次新增和修改的用例

任务单第 1 条要的 `rcl_*`、`quo_*`、`men_*` 用例在 BE-472～477 时已经写了（`rcl_recall_test.go`、`rcl_admin_test.go`、`rcl_files_test.go`、`quo_quote_test.go`、`men_mention_test.go`、`c4_switch_test.go`），这次逐个核对 TC 里层级含"后端 e2e"的用例都有对应函数（有几个合在一个函数里，函数注释写了编号，例如 TC-RCL-02-04 在 `TestTC_RCL_03_05_LastMsgRecalled`、TC-QUO-01-03 和 02-07 在 `TestTC_QUO_01_07_Summaries`、TC-MEN-01-02 在 `TestTC_MEN_01_01_MentionMember`），没有重复写。补的：

- tianting-server `server/e2e/c4_markers_test.go`（**特征串检查单独一个文件**；原 `c4_logs_test.go` 改名并入，`TestTC_CHAT_90_29_LogsHaveNoText` 不变）：
  - 新增 `TestTC_CHAT_90_29_MarkersInLogsAndAudit`：正文、文件名、引用文字、@ 正文带 `ZXQ-RCL-*`、`ZXQ-QUO-*`、`ZXQ-MEN-*`，走单聊撤回、管理员撤回成员、群主撤回文件、引用（含被引用的消息撤回后 `sync`）、@、@ 所有人、后台删除带引用的消息、隔离授权读正文、`fetch` 已撤回的文件；用 `deploy/ops/scan-logs` 扫 DEBUG 日志和 `admin_audit` 全部行。对照：隔离读取确实返回原文、`fetch` 确实拿到字节、审计里有 fileId 和 msgId、扫描工具在植入特征串的文件上会报出来。
- tianting-console-server `e2e/c4_markers_test.go`：`TestTC_CHAT_90_29_ConsoleLogsAndAudit`（TC-CHAT-90-29 的 console 一侧）：经 console 走元数据、普通正文、隔离正文、`fetch`、审计写失败的错误路径、删除已撤回的消息；console 日志、im-server 日志、console 全库、im-server 审计都搜不到三类特征串，再用 `scan-logs` 扫一次。
- tianting-server `server/e2e/men_perf_test.go`：`TestTC_MEN_02_03_MentionAllAckDevRecord`（2000 人、在线 200、1000 次 @ 所有人，`IM_PERF_DEV=1` 才跑；断言 1000 次都成功、200 个连接都收到 1000 条，延迟只打印）。
- tianting-server `server/e2e/c4_perf_test.go`：`TestTC_CHAT_90_27_QuoteAndMentionAckDevRecord`（带引用的单聊、带 @ 的群聊各 1000 条，同上只记录）。
- `TestTC_RCL_01_01_RecallWithin60s` 补了 AC-RCL-01-1 的"2 秒内"：原来只靠 3 秒的等待超时，现在按真实时间断言 `msgDeleted` 在 2 秒内到达。

## 3. 修了什么

没有改实现。核对中没有发现实现和 PRD 不符的地方，也没有新的口径问题（backend-chat.md 问题表没有新增，最大号仍是 71）。

## 4. 开发机性能记录（**不算通过**，判定等 P-12）

顺序发送（一条 ack 回来再发下一条），不是 500 条/秒；开发机不是生产配置；同时还有模拟器等进程在跑。3 次取最差。

| AC | 测量 | 3 次 P99 | 最差 P99 | 未校准目标 |
|---|---|---|---|---|
| AC-CHAT-90-1（TC-CHAT-90-27） | 带引用的单聊文本，1000 条，对方在线 | 1.00 / 2.72 / 0.57 ms | 2.72 ms（单条最慢 6.98 ms） | ≤ 200 ms |
| AC-CHAT-90-1（TC-CHAT-90-27） | 带 @ 成员的群聊文本，1000 条，3 人在线 | 0.68 / 0.55 / 1.19 ms | 1.19 ms | ≤ 200 ms |
| AC-MEN-02-3（TC-MEN-02-03） | 2000 人群、在线 200，群主 1000 次 @ 所有人 | 2.29 / 2.11 / 2.41 ms | 2.41 ms（单条最慢 17.8 ms） | ≤ 200 ms |

## 5. 逐条

| AC | 层 | 结论 | 用例编号 | 依据（测试函数；console 的标 console） | 说明 |
|---|---|---|---|---|---|
| AC-ACC-10-4 | 后端 | 通过 | TC-ACC-10-04 | `TestTC_ACC_10_04_PrivacyNoticeRequired` |  |
| AC-FILE-06-3 | 后端 | 通过 | TC-FILE-06-13 | `TestTC_FILE_06_13_RecalledFilePurge` |  |
| AC-APN-03-3 | 后端 | 未执行（等 C3） | TC-APN-03-03 | — | 依赖 C3 推送 worker 和假通道（BE-481、BE-455、BE-456）。按 PENDING P-43 ② 不算失败 |
| AC-RCL-01-1 | 端到端 | 部分通过 | TC-RCL-01-01 | `TestTC_RCL_01_01_RecallWithin60s` | 服务端部分通过：ack `{}`，对方和自己另一端 2 秒内（本次补了真实时间断言）收到带 `recalled{by,at}` 的 `msgDeleted`，发起连接只收 ack。客户端部分（三端显示）随 FE，前端报告 FE-672 |
| AC-RCL-01-2 | 后端 | 通过 | TC-RCL-01-02 | `TestTC_RCL_01_02_WindowBoundary` |  |
| AC-RCL-01-3 | 后端 | 通过 | TC-RCL-01-03 | `TestTC_RCL_01_03_Errors` |  |
| AC-RCL-01-4 | 后端 | 通过 | TC-RCL-01-04 | `TestTC_RCL_01_04_Idempotent` |  |
| AC-RCL-01-5 | 后端 | 通过 | TC-RCL-01-05 | `TestTC_RCL_01_05_MutedAndBlocked` |  |
| AC-RCL-01-6 | 后端 | 通过 | TC-RCL-01-06 | `TestTC_RCL_01_06_NoPlaintextAfterRecall` |  |
| AC-RCL-01-7 | 后端 | 通过 | TC-RCL-01-07 | `TestTC_RCL_01_07_UnreadUnchanged` |  |
| AC-RCL-01-9 | 后端 | 通过 | TC-RCL-01-09 | `TestTC_RCL_01_09_SelfCustomAndRate` |  |
| AC-RCL-01-10 | 后端 | 通过 | TC-RCL-01-10 | `TestTC_RCL_01_10_QuarantineKeyRequired`、`TestTC_RCL_01_10_SwitchOff`、`TestTC_RCL_01_10_StartupAndNoV13` | e2e（开关关 4030、不依赖 V13）+ config 单测（缺隔离密钥启动失败） |
| AC-RCL-01-11 | 后端 | 通过 | TC-RCL-01-11 | `TestTC_RCL_01_11_QuarantineWriteFault` |  |
| AC-RCL-02-1 | 后端 | 通过 | TC-RCL-02-01 | `TestTC_RCL_02_01_RoleMatrix` |  |
| AC-RCL-02-2 | 后端 | 通过 | TC-RCL-02-02 | `TestTC_RCL_02_02_OwnerWindow` |  |
| AC-RCL-02-3 | 后端 | 通过 | TC-RCL-02-03 | `TestTC_RCL_02_03_ManagedGroup` |  |
| AC-RCL-02-4 | 端到端 | 部分通过 | TC-RCL-02-04 | `TestTC_RCL_03_05_LastMsgRecalled` | 服务端部分通过：a 撤回 m 后三人 `sync`、`convs.lastMsg` 一致（`recalledBy=a`）。客户端部分（三端显示）随 FE，前端报告 FE-672 |
| AC-RCL-02-6 | 后端 | 通过 | TC-RCL-02-06 | `TestTC_RCL_02_06_LeftDissolvedDemoted` |  |
| AC-RCL-02-7 | 后端 | 通过 | TC-RCL-02-07 | `TestTC_RCL_02_07_OwnWiderWindow` |  |
| AC-RCL-03-3 | 端到端 | 部分通过 | TC-RCL-03-03 | `TestTC_RCL_03_03_OfflineRecall` | 服务端部分通过：u2 离线期间撤回，重连 `sync` 拿到 `deleted=recalled=true`、没有原文。重连后显示撤回文案随 FE（SDK 单测已有） |
| AC-RCL-03-5 | 后端 | 通过 | TC-RCL-03-05 | `TestTC_RCL_03_05_LastMsgRecalled` |  |
| AC-RCL-03-7 | 后端 | 通过 | TC-RCL-03-07 | `TestTC_CHAT_90_32_C4FrameVectorsServerParse`、`TestTC_RCL_01_01_RecallWithin60s` |  |
| AC-RCL-03-9 | 后端 | 未执行（等 C3） | TC-RCL-03-09 | — | 同上（BE-481）。撤回事件本身不入推送队列的部分随 BE-481 一起验 |
| AC-RCL-04-1 | 后端 | 通过 | TC-RCL-04-01 | `TestTC_RCL_04_01_AdminDeleteRecalled`、console `TestTC_RCL_04_01_DeleteRecalled` | im-server 一侧和 console 一侧（CS-204，含响应丢失后的对账）都通过 |
| AC-RCL-04-2 | 后端 | 通过 | TC-RCL-04-02 | `TestTC_RCL_04_02_RecallAdminDeleted` |  |
| AC-RCL-04-3 | 后端 | 通过 | TC-RCL-04-03 | `TestTC_RCL_04_03_AdminBodies`、console `TestTC_RCL_04_03_BodyRecalled` | im-server 一侧和 console 一侧（CS-204）都通过 |
| AC-RCL-04-4 | 后端 | 通过 | TC-RCL-04-04 | `TestTC_RCL_04_04_QuarantinePurge` |  |
| AC-RCL-04-5 | 后端 | 通过 | TC-RCL-04-05 | `TestTC_RCL_04_05_C2CBodyDisabled` |  |
| AC-RCL-04-6 | 后端 | 通过 | TC-RCL-04-06 | `TestTC_RCL_04_06_RecalledImageURL` |  |
| AC-RCL-04-7 | 后端 | 通过 | TC-RCL-04-07 | `TestTC_RCL_04_07_MetaRecalled`、`TestTC_RCL_04_07_MetaRecalledFile`、console `TestTC_RCL_04_07_MetaRecalled` | im-server（文本、文件）和 console 两层元数据都通过 |
| AC-RCL-04-8 | 后端 | 通过 | TC-RCL-04-08 | `TestTC_RCL_04_07_MetaRecalled`、`TestTC_RCL_01_06_NoPlaintextAfterRecall` |  |
| AC-RCL-04-9 | 后端 | 通过 | TC-RCL-04-09 | `TestTC_RCL_04_09_Fetch`、`TestTC_RCL_04_09_FetchGrantID`、`TestTC_RCL_04_09_FetchC2CDisabled`、console `TestTC_RCL_04_09_FetchViaConsole` | im-server（含 grantId，P-44 ④）和 console 代理（CS-202）都通过 |
| AC-QUO-01-1 | 端到端 | 部分通过 | TC-QUO-01-01 | `TestTC_QUO_01_01_QuoteReply` | 服务端部分通过：u2 收到的 `msg` 带 `quote{state:normal,text}`。客户端部分（三端显示）随 FE，前端报告 FE-672 |
| AC-QUO-01-2 | 后端 | 通过 | TC-QUO-01-02 | `TestTC_QUO_01_02_Unavailable` |  |
| AC-QUO-01-3 | 后端 | 通过 | TC-QUO-01-03 | `TestTC_QUO_01_07_Summaries` |  |
| AC-QUO-01-4 | 后端 | 通过 | TC-QUO-01-04 | `TestTC_QUO_01_04_DedupFirst` |  |
| AC-QUO-01-6 | 后端 | 通过 | TC-QUO-01-06 | `TestTC_QUO_01_06_NotCopied` |  |
| AC-QUO-01-7 | 后端 | 通过 | TC-QUO-01-07 | `TestTC_QUO_01_07_Summaries` |  |
| AC-QUO-02-1 | 后端 | 通过 | TC-QUO-02-01 | `TestTC_QUO_02_01_StateAfterRecallAndDelete` |  |
| AC-QUO-02-4 | 后端 | 通过 | TC-QUO-02-04 | `TestTC_QUO_02_04_HistoryHidden` |  |
| AC-QUO-02-7 | 后端 | 通过 | TC-QUO-02-07 | `TestTC_QUO_01_07_Summaries` |  |
| AC-MEN-01-1 | 端到端 | 部分通过 | TC-MEN-01-01 | `TestTC_MEN_01_01_MentionMember` | 服务端部分通过：u2 收到 `mentions.userIds=[u2]`、`mentionUnread=1`，u3 为 0。"[有人@我]"分（三端显示）随 FE，前端报告 FE-672 |
| AC-MEN-01-2 | 后端 | 通过 | TC-MEN-01-02 | `TestTC_MEN_01_01_MentionMember` |  |
| AC-MEN-01-3 | 后端 | 通过 | TC-MEN-01-03 | `TestTC_MEN_01_03_Limits` |  |
| AC-MEN-01-4 | 后端 | 通过 | TC-MEN-01-04 | `TestTC_MEN_01_04_TextIsNotParsed` |  |
| AC-MEN-01-11 | 后端 | 通过 | TC-MEN-01-11 | `TestTC_MEN_01_11_KeywordAndDedup` |  |
| AC-MEN-02-1 | 后端 | 通过 | TC-MEN-02-01 | `TestTC_MEN_02_01_MentionAllRoles` |  |
| AC-MEN-02-2 | 后端 | 通过 | TC-MEN-02-02 | `TestTC_MEN_02_02_MentionAllQuota` |  |
| AC-MEN-02-3 | 后端 | 未执行 | TC-MEN-02-03 | `TestTC_MEN_02_03_MentionAllAckDevRecord` | 性能 AC，判定等 P-12（EP）。开发机只记录（第 4 节）：功能部分（1000 次都成功、200 个在线连接都收到 1000 条）通过，ack P99 不算通过 |
| AC-MEN-02-6 | 后端 | 通过 | TC-MEN-02-06 | `TestTC_MEN_02_06_NotExpanded` |  |
| AC-MEN-02-7 | 后端 | 通过 | TC-MEN-02-07 | `TestTC_MEN_02_07_ManagedAndAllMuted` |  |
| AC-MEN-02-8 | 后端 | 通过 | TC-MEN-02-08 | `TestTC_MEN_02_08_RecallNoRefundAndCounterFault` |  |
| AC-MEN-02-9 | 后端 | 通过 | TC-MEN-02-09 | `TestTC_MEN_02_09_Concurrent` |  |
| AC-MEN-03-1 | 后端 | 通过 | TC-MEN-03-01 | `TestTC_MEN_03_01_Counts` |  |
| AC-MEN-03-4 | 后端 | 通过 | TC-MEN-03-04 | `TestTC_MEN_03_04_RecallMention` |  |
| AC-MEN-03-8 | 后端 | 通过 | TC-MEN-03-08 | `TestTC_MEN_03_08_SelfLeftAndWriteFault` |  |
| AC-CHAT-90-1 | 后端（全部） | 未执行 | TC-CHAT-90-27 | `TestTC_CHAT_90_27_QuoteAndMentionAckDevRecord` | 性能。C4 部分（TC-CHAT-90-27：带引用的单聊、带 @ 的群聊）开发机只记录（第 4 节），不算通过，等 P-12 |
| AC-CHAT-90-2 | 后端（全部） | 部分通过 | TC-CHAT-90-28 | `TestTC_RCL_01_10_SwitchOff` | C4 部分（TC-CHAT-90-28）：默认配置 `IM_MSG_ACTIONS_ENABLED` 关（其他开关也关）下现有 e2e 不改代码全过（本次 `go test ./...`），开关关的 4030 见 `TestTC_RCL_01_10_SwitchOff`；TC-PERF 判定等 P-12 |
| AC-CHAT-90-5 | 后端（全部） | 部分通过 | TC-CHAT-90-29 | `TestTC_CHAT_90_29_LogsHaveNoText`、`TestTC_CHAT_90_29_MarkersInLogsAndAudit`、console `TestTC_CHAT_90_29_ConsoleLogsAndAudit` | C4 部分的后端 e2e 通过：im-server（日志、`admin_audit`）和 console-server（两边日志、console 全库、im-server 审计）都用 `scan-logs` 扫，搜不到 ZXQ-RCL / ZXQ-QUO / ZXQ-MEN，审计里有 msgId、fileId。部署 e2e（TC-CHAT-90-29 第二层，真实部署的日志）未执行，同 C1、C2 |
| AC-CHAT-90-10 | 后端（全部） | 部分通过 | TC-CHAT-90-32 | `TestTC_CHAT_90_32_C4FrameVectorsServerParse` | C4 部分（TC-CHAT-90-32）：服务端用真实帧解析跑 `frames.json` 的 C4 用例（25 个合法、29 个非法）通过；sdk-ts（FE-660）、sdk-kotlin（FE-661）的记录写明 C4 帧 54 个通过。sdk-swift（FE-663）的进度记录没有写明 C4 帧向量的结果，随前端报告 FE-672 确认。本地结果 |
| AC-CHAT-90-17 | 后端（全部） | 通过 | TC-CHAT-90-34 | `TestTC_CHAT_90_34_MsgActionsCheck`、`TestTC_CHAT_90_34_RecallMetrics` | C4 部分（TC-CHAT-90-34）：`im_recalls_total{by}` 三个取值都增加，标签没有 userId、convId、msgId；配置检查 `msg_actions`（开关、隔离密钥已设置、隐私说明版本，不含密钥） |

"通过"一栏没有写说明的，是用例按 AC 原文逐项断言、本次全量（含 `-race`）通过。

## 6. 不计入本报告、但有后端产出的前端 AC

| AC | 后端部分 | 状态 |
|---|---|---|
| AC-RCL-03-13、AC-QUO-02-6、AC-MEN-03-7、AC-CHAT-90-15（C4 部分） | BE-478：`chat-sync.json` 新增 6 个 C4 用例，`TestChatModelRules` 断言关键结论（本次全量通过） | 三套 SDK 的执行器随 FE-660、FE-661、FE-663，结论在前端报告（FE-672） |

## 7. 汇总

| 结论 | 数量 | AC |
|---|---|---|
| 通过 | 49 | 见第 5 节 |
| 部分通过 | 8 | AC-RCL-01-1、AC-RCL-02-4、AC-RCL-03-3、AC-QUO-01-1、AC-MEN-01-1（端到端，客户端部分随 FE-672）；AC-CHAT-90-2（TC-PERF 等 P-12）；AC-CHAT-90-5（部署 e2e 未执行）；AC-CHAT-90-10（sdk-swift 的 C4 帧向量结果待前端报告确认） |
| 未执行 | 4 | AC-APN-03-3、AC-RCL-03-9（**未执行（等 C3）**，挡在 C3 的 BE-481，按 PENDING P-43 ② 不算失败）；AC-MEN-02-3、AC-CHAT-90-1（性能，开发机只记录，等 P-12） |
| 失败 | 0 | — |

## 8. 开放项

1. **C3 恢复后**做 BE-481（AC-APN-03-3、AC-RCL-03-9），状态从"未执行（等 C3）"改为实际结论。
2. **P-12**：AC-CHAT-90-1（C4 部分）、AC-MEN-02-3 在 EP 上压测判定；AC-CHAT-90-2 的 TC-PERF。
3. **FE-672**：五条端到端 AC 的客户端部分；确认 sdk-swift 跑过 `frames.json` 的 C4 帧（AC-CHAT-90-10）。
4. 部署 e2e（TC-CHAT-90-29 第二层）、真机（P-13）、云厂商对象存储（CH-05）没有做。
5. 本次没有新的待 Frank 决定的事。
