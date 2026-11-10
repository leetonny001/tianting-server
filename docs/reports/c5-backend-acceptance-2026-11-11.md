# C5 后端验收报告（BE-489，2026-11-11）

| 项 | 内容 |
|---|---|
| 范围 | PRD-chat v1.7 里标了【C5】、验证层是【后端】或端到端（挂了 BE / CS 任务）的 AC，加上【全部】里挂在 C5 任务（BE-482～BE-490、CS-205）上的【后端】AC，共 **30 条**：第 9 节 F-CVS-01～04 的【后端】20 条、端到端 4 条（AC-CVS-03-2、04-1、04-2、04-5），其他章节标【C5】的 AC-APN-04-4、AC-MEN-03-2、AC-MEN-03-3，【全部】的 AC-CHAT-90-2、AC-CHAT-90-10（第 15 节累计表 C5 列；90-1、90-3 在 C5 不变，90-5、90-17 是"—"，按 BE-489 第 2 条不另跑）。其他 PRD 没有标【C5】的 AC（PRD-core、PRD-ui 只加注，AC 不变，第 88 条）。**前端和手工 AC 不在本报告**（FE-686 另做）；端到端 AC 只记后端部分 |
| 环境 | 本地开发机（macOS，Apple Silicon）。**CI 不运行（P-21），所有结果都是本地结果**。im-server：E1（SQLite、`IM_TEST_MODE=1`、`IM_CONV_SETTINGS_ENABLED=1`，需要时加 `IM_MSG_ACTIONS_ENABLED=1`、进程内假 S3）；console-server：真实 Postgres 17.11（临时实例，见第 1 节）+ 真实 im-server 进程 + 假 S3 |
| 基线 | tianting-server（im-mvp）@2ea7cbe、tianting-console-server@57da60f，加上本报告所在的提交（只有用例、共享向量的说明文字和文档，没有改实现） |
| 结论 | **通过 19、部分通过 6、未执行 5（全部等 C3）、失败 0**。性能没有判定（P-12）；真机、部署 e2e 没有做 |
| 没有做的 | 推送衔接（C3 暂停，BE-490，按 P-46 记"未执行（等 C3）"）；性能判定（P-12）；真机（P-13）；部署 e2e |

## 1. 运行

| 命令 | 结果（本地） |
|---|---|
| `server/`：`go vet ./...`、`gofmt -l .` | 无问题 |
| `server/`：`go test -count=1 -timeout 30m ./...` | 全部包通过（e2e 126 秒）。默认配置下 `IM_CONV_SETTINGS_ENABLED` 等新开关都关，这一次同时是 TC-CHAT-90-35 第 1 步（现有 e2e 不改代码全过） |
| `server/`：`go test -race -count=1 -timeout 40m ./...` | 全部包通过（e2e 526 秒），没有 DATA RACE。TC-CVS-03-14 的并发用例在里面 |
| `server/`：`go test -count=1 ./e2e/chatmodel -run TestChatSyncVectors -update`，再 `go test -count=1 ./e2e/chatmodel` | 通过；`cases` 逐字节不变（第 3 节） |
| tianting-console-server：`go vet ./...` | 无问题 |
| tianting-console-server：`go test -race -count=1 -timeout 40m ./...`（`CON_TEST_REQUIRE_DB=1`，临时 Postgres） | 全部包通过（e2e 249 秒），Postgres 不可用时不会跳过，没有 SKIP |
| `python3 -I tools/trace.py` | gaps 0 |

**偶发失败**：没有。console-server 第一次全量有 1 个失败（`TestOpsRoleExpire_AC02_12`，账号列表 5000），**原因是这次临时库建错了**：`initdb --locale=C` 没有指定编码，库是 SQL_ASCII，`auth.accounts` 前缀查询里的 `chr(1114111)` 报 `requested character too large for encoding`（Postgres 日志），单独重跑 5 次都失败，跟代码无关。按 `-E UTF8` 重建临时库后整个 console-server 全量重跑（不是只重跑这个用例），全部通过。用例和断言都没改。

**临时 Postgres**：`brew` 已装的 postgresql@17（P-5），`LC_ALL=C initdb --locale=C -E UTF8 -U postgres --auth=trust` 到会话 scratchpad（**必须写 `-E UTF8`**，不写时是 SQL_ASCII，见上），只监听 `127.0.0.1:55491`、不开 Unix socket、`lc_messages=C`；跑完 `pg_ctl stop` 并删除数据目录。

## 2. 本次新增和修改的用例

任务单第 1 条要的 `cvs_*_test.go` 在 BE-483～486、BE-488 时已经写了（`cvs_switch_test.go`、`cvs_pin_test.go`、`cvs_delete_test.go`、`cvs_sync_test.go`、`cvs_account_test.go`），这次逐个核对 TC-CVS 里层级含"后端 e2e"的用例都有对应函数（有几个合在一个函数里，函数注释写了编号：TC-CVS-03-02 在 `TestTC_CVS_03_01_DeleteC2C`，TC-CVS-03-15 在 `TestTC_CVS_03_07_FilesAfterDelete`，TC-CHAT-90-35 在 `TestTC_CVS_02_07_SwitchOff`），没有重复写。补的：

- tianting-server `server/e2e/cvs_markers_test.go`（**"只影响自己"和清除的特征串检查单独一个文件**，BE-489 第 1 条）：`TestTC_CVS_03_01_MarkersOnlyAffectSelf`。u2 给 u1 发正文含 `ZXQ-CVS-1` 的文本、一张图片、文件名含 `ZXQ-CVS-2` 的文件和一条普通文本，u1 在 D1 删除到 seq 4：
  - u1 的 D2 收到的 `convDeleted` 没有特征串；u1 两个连接的 `convs`、`sync`（`afterSeq` 和 `beforeSeq` 两个方向）都搜不到，图片原图、缩略图和文件地址 4048；
  - u2 再发一条后 u1 的会话重新出现（`lastMsg` 是新消息），两个方向的 `sync` 仍然搜不到；
  - 对照（只影响自己）：删除前 u1 能看到两个特征串、能拿地址；删除后 u2 的 `sync` 两个特征串都在、图片和文件地址照常；后台授权读正文有原文；库里原文还在（删除会话不删消息）；
  - 群里同样（u1 删除后搜不到，u2 照常）；
  - DEBUG 日志和 `admin_audit` 用 `deploy/ops/scan-logs --marker=ZXQ-CVS` 扫，搜不到；对照：删除的日志行（`conversation deleted`）在，扫描工具对植入的特征串会报错。
  - 反证：把删除位置改成 `upToSeq=2`（文件消息留在范围外）时用例报 4 处"has a marker"，改回后通过。
- `protocol/vectors/chat-sync.json` 的 `format."会话"`：改正问题 69（见第 3 节），用例数据不变。

## 3. 修了什么

没有改实现。核对中没有发现实现和 PRD 不符的地方，也没有新的口径问题（backend-chat.md 问题表没有新增，最大号仍是 92）。

- **frontend-chat 问题 69（已修）**：`chat-sync.json` 的 format 说明 `"会话"` 写的是 `readSeq=maxSeq`，C5 用例带了 `unread`，三套 SDK 实际按 `readSeq = maxSeq − unread` 展开。改的是生成器 `server/e2e/chatmodel/vectors_test.go` 里的说明文字（"readSeq=maxSeq−unread（C2、C4 的用例没有 unread，即 readSeq=maxSeq）"），再用 `go test ./e2e/chatmodel -run TestChatSyncVectors -update` 生成。核对：改前改后 `cases` 一段**逐字节相同**（19 个用例，C2、C4 的 13 个和 C5 的 6 个都不变），整份文件只有 `format."会话"` 一个键不同。三套 SDK 的协议副本（`frames.json` 现在和服务端逐字节相同，`chat-sync.json` 只差这一句说明）由 SDK 任务用 `scripts/sync-protocol.sh` 同步，不影响执行器。

## 4. 逐条

| AC | 层 | 结论 | 用例编号 | 依据（测试函数；console 的标 console） | 说明 |
|---|---|---|---|---|---|
| AC-CVS-01-1 | 后端 | 通过 | TC-CVS-01-01 | `TestTC_CVS_01_01_PinOrder` | 顺序 c2、c1、其余按 `updatedAt`；ack 的 `conv` 和 `convs` 这一项相同 |
| AC-CVS-01-2 | 后端 | 通过 | TC-CVS-01-02 | `TestTC_CVS_01_02_PinLimit` | 第 21 个 4220（不带 `details`，没有写入）；再次置顶 `pinnedAt` 不变 |
| AC-CVS-01-3 | 后端 | 通过 | TC-CVS-01-03 | `TestTC_CVS_01_03_PinnedSurvivesTruncation` |  |
| AC-CVS-01-4 | 后端 | 通过 | TC-CVS-01-04 | `TestTC_CVS_01_04_Errors` | 4032、4033、4040、4043、4000，都没有写入、不发事件 |
| AC-CVS-01-5 | 后端 | 通过 | TC-CVS-01-05 | `TestTC_CVS_01_05_RejoinDoesNotInherit`、`TestTC_CVS_01_05_CleanupOnLeave` | 移出、退群、后台移除、两种解散都在同一事务删行 |
| AC-CVS-02-1 | 后端 | 部分通过 | TC-CVS-02-01 | `TestTC_CVS_02_01_UnreadUnderNotifyOff` | `unread` 增加那一半通过（`notifyOff=true`、离线后 `unread` +1）。"u2 令牌无调用"要推送 worker 和假通道，**未执行（等 C3）**（BE-490，第 72 条，P-46 按 P-43 ② 不算失败） |
| AC-CVS-02-7 | 后端 | 通过 | TC-CVS-02-07 | `TestTC_CVS_02_07_SwitchOff` | `convSet`、`convDelete` 403 / 4030 `featureDisabled`，格式错误先 4000，没有写入，`features.convSettings=false` |
| AC-CVS-02-8 | 后端 | 未执行（等 C3） | TC-CVS-02-08 | — | 预期是"按没有免打扰推送"，要推送 worker 和假通道（BE-490）。`convs` 读设置失败不降级（第 90 条）不是这条 AC |
| AC-CVS-03-1 | 后端 | 通过 | TC-CVS-03-01 | `TestTC_CVS_03_01_DeleteC2C`、`TestTC_CVS_03_01_MarkersOnlyAffectSelf`、console `TestTC_CVS_03_01_AdminQueriesUnaffected` | u1 列表没有、`deleted` 含 `{convId, 10}`、`sync` 空；u2 不受影响；后台元数据和正文 10 条（im-server 和 console 两层，第 87 条，CS-205）；特征串检查见第 2 节 |
| AC-CVS-03-2 | 端到端 | 部分通过 | TC-CVS-03-02 | `TestTC_CVS_03_01_DeleteC2C`、`TestTC_CVS_03_01_MarkersOnlyAffectSelf` | 服务端部分通过：重新出现，`unread=1`、`minSeq=11`，`deleted` 不再含它，`sync` 两个方向只有这 1 条、搜不到 `ZXQ-CVS-1`。客户端部分（三端显示、本地搜不到）随 FE-686 |
| AC-CVS-03-3 | 后端 | 通过 | TC-CVS-03-03 | `TestTC_CVS_03_03_UpToSeq` |  |
| AC-CVS-03-4 | 后端 | 通过 | TC-CVS-03-04 | `TestTC_CVS_03_04_DeleteGroup` |  |
| AC-CVS-03-6 | 后端 | 通过 | TC-CVS-03-06 | `TestTC_CVS_03_06_DeleteClearsPinKeepsNotifyOff` |  |
| AC-CVS-03-7 | 后端 | 通过 | TC-CVS-03-07 | `TestTC_CVS_03_07_FilesAfterDelete` | 原图、缩略图 4048，和不存在的 fileId 响应相同；u2 照常下载 |
| AC-CVS-03-9 | 后端 | 通过 | TC-CVS-03-09 | `TestTC_CVS_03_09_DeleteErrors` |  |
| AC-CVS-03-11 | 后端 | 通过 | TC-CVS-03-11 | `TestTC_CVS_03_11_EmptyGroups` |  |
| AC-CVS-03-12 | 后端 | 通过 | TC-CVS-03-12 | `TestTC_CVS_03_12_RejoinSeesHistory` |  |
| AC-CVS-03-13 | 后端 | 通过 | TC-CVS-03-13 | `TestTC_CVS_03_13_TipAndMonotonic` |  |
| AC-CVS-03-14 | 后端 | 通过 | TC-CVS-03-14 | `TestTC_CVS_03_14_DeleteRacesSend` | 并发 50 次加两种确定的先后；本次 `-race` 全量也跑 |
| AC-CVS-03-15 | 后端 | 通过 | TC-CVS-03-15 | `TestTC_CVS_03_07_FilesAfterDelete`（群的部分） | 对象还在，`files` 行状态不变 |
| AC-CVS-04-1 | 端到端 | 部分通过 | TC-CVS-04-01 | `TestTC_CVS_04_01_SettingsEvent` | 服务端部分通过：D2 2 秒内（真实时间）收到两条 `convSettingsChanged`，符合 Schema；D1 和对方不收（哨兵法）。D2 页面更新随 FE-686 |
| AC-CVS-04-2 | 端到端 | 部分通过 | TC-CVS-04-02 | `TestTC_CVS_04_02_OfflineDeviceCatchesUp` | 服务端部分通过：D2 重连后 `convs`（顺序、置顶、免打扰、`deleted`）和 D1 相同。客户端显示随 FE-686 |
| AC-CVS-04-3 | 后端 | 未执行（等 C3） | TC-CVS-04-03 | — | 双节点 + 每个节点一个假通道（第 83 条，BE-490） |
| AC-CVS-04-4 | 后端 | 通过 | TC-CVS-04-04 | `TestTC_CVS_04_04_RateLimit`、`TestTC_CVS_04_04_MixedWithDelete` | 第 6 次 4290；`convSet`、`convDelete` 共用计数（第 73 条） |
| AC-CVS-04-5 | 端到端 | 部分通过 | TC-CVS-04-05 | `TestTC_CVS_04_05_DeletedEvent`、`TestTC_CVS_03_01_MarkersOnlyAffectSelf` | 服务端部分通过：D2 2 秒内收到 `convDeleted {convId, clearedSeq}`（不另收 `readUpdated`、`convSettingsChanged`，第 84 条），事件里没有特征串。D2 会话消失、本地消息清除随 FE-686（共享向量见第 5 节） |
| AC-MEN-03-2 | 后端 | 未执行（等 C3） | TC-MEN-03-02 | — | 预期是"假通道只收到 @ u2 那条"（BE-490） |
| AC-MEN-03-3 | 后端 | 未执行（等 C3） | TC-MEN-03-03 | — | 同上（BE-490）。`mentionUnread` 增加的部分随 BE-490 一起验 |
| AC-APN-04-4 | 后端 | 未执行（等 C3） | TC-APN-04-04 | — | 要假 APNs 的 `badge`（BE-490）。角标口径的聚合函数 `TotalUnread` 已有 Store 契约用例（AC 的数据得 5，BE-484），不算这条 AC 通过 |
| AC-CHAT-90-2 | 后端（全部） | 部分通过 | TC-CHAT-90-35 | `TestTC_CVS_02_07_SwitchOff`、本次 `go test ./...` | C5 部分：默认配置（`IM_CONV_SETTINGS_ENABLED` 等新开关都关）下现有 e2e 不改代码全过；没有设置时 `convs` 不带 `pinned`、`pinnedAt`、`notifyOff`、`deleted`、`deletedTruncated`。TC-PERF 判定等 P-12 |
| AC-CHAT-90-10 | 后端（全部） | 通过 | TC-CHAT-90-38 | `TestTC_CHAT_90_38_C5FrameVectorsServerParse` | 服务端用真实解析跑 `frames.json` 的 C5 用例（18 个合法、33 个非法）通过。三套 SDK：sdk-ts（FE-680，含 Schema 校验）、sdk-swift（FE-682）的记录写明 C5 帧 51 个通过；sdk-kotlin（FE-681）的记录是 sdk-core 单测全部通过，`VectorsTest` 遍历 `frames.json` 全部合法、非法用例（本次读代码核对）。三套 SDK 的 `frames.json` 副本和服务端逐字节相同（本次 `cmp`）。SDK 没有在本次重跑。本地结果 |

"通过"一栏没有写说明的，是用例按 AC 原文逐项断言、本次全量（含 `-race`）通过。

## 5. 不计入本报告、但有后端产出的前端 AC

| AC | 后端部分 | 状态 |
|---|---|---|
| AC-CHAT-90-15（C5 部分）、AC-CVS-03-5、AC-CVS-02-5 | BE-487：`chat-sync.json` 的 C5 用例 6 个，`TestChatModelRules` 断言关键结论（本次全量通过）；本次只改了 format 说明（问题 69） | 三套 SDK 的执行器随 FE-680～682（记录都写明 C5 用例 6 个通过），结论在前端报告（FE-686） |

## 6. 汇总

| 结论 | 数量 | AC |
|---|---|---|
| 通过 | 19 | AC-CVS-01-1～01-5、02-7、03-1、03-3、03-4、03-6、03-7、03-9、03-11～03-15、04-4，AC-CHAT-90-10 |
| 部分通过 | 6 | AC-CVS-02-1（`unread` 一半通过，推送一半等 C3）；AC-CVS-03-2、04-1、04-2、04-5（端到端，客户端部分随 FE-686）；AC-CHAT-90-2（TC-PERF 等 P-12） |
| 未执行 | 5 | AC-CVS-02-8、04-3，AC-MEN-03-2、03-3，AC-APN-04-4：**未执行（等 C3）**，挡在 BE-490，按 P-46（同 P-43 ②）不算失败 |
| 失败 | 0 | — |

## 7. 开放项

1. **C3 恢复后**做 BE-490（上面 5 条和 AC-CVS-02-1 的推送一半），状态改为实际结论，然后关闭 C5 的后端验收。
2. **P-12**：AC-CHAT-90-2 的 TC-PERF。
3. **FE-686**：四条端到端 AC 的客户端部分；AC-CVS-02-6 等 C3 加通知后补跑（frontend-chat 第 58 条）。
4. 三套 SDK 下次同步协议时带上 `chat-sync.json` 的新说明（问题 69，用例数据没变，执行器不受影响）。
5. 部署 e2e、真机（P-13）没有做。
6. 本次没有新的待 Frank 决定的事。
