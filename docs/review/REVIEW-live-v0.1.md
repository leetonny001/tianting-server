# 需求评审记录：PRD-live-v0.1

| 项 | 内容 |
|---|---|
| 评审对象 | [docs/prd/PRD-live-v0.1.md](../prd/PRD-live-v0.1.md)（直播聊天室、系统推送、迁移） |
| 日期 | 2026-11-05 |
| 评审人 | 三个独立 AI 子代理（未参与 PRD 编写），只读，没有改文件、没有跑压测 |
| 视角 | 后端（`R-LB-nn`）、前端 / 客户端 / SDK（`R-LF-nn`）、测试（`R-LQ-nn`，原 QR-nn） |
| 对照材料 | 后端：`server/internal`、`protocol/`、ARCH-v1、PRD-core v1.3、PRD-admin v1.0、任务单、perf-2026-11-03。前端：三套 SDK、三个天听 App、fx-sport、ab77、android。测试：全部 105 条 AC、PRD-social 5.6 节、PENDING P-12 |
| 回应 | 产品（AI），见 [PRD-live v0.2](../prd/PRD-live-v0.2.md) 修订记录 |
| 拍板 | Frank |

编号说明：三份报告原来都有 `R-LIVE-nn` 或 `QR-nn`，有冲突，这里统一加前缀，各自内部序号不变。文件路径的公共前缀省略方式沿用各份报告。

## 1. 结论汇总

| 视角 | 阻塞 | 严重 | 一般 | 建议 | 合计 | 结论 |
|---|---|---|---|---|---|---|
| 后端 R-LB | 4 | 13 | 12 | 3 | 32 | 修改后再提交拍板 |
| 前端 R-LF | 4 | 12 | 7 | 2 | 25 | 暂不建议整份进入拍板 |
| 测试 R-LQ（自述） | 2 | 13 | 15 | 2 | 32 | 修改后通过 |
| **合计（自述）** | **10** | **38** | **34** | **7** | **89** | **先出 v0.2 再拍板** |

**核对说明**：加总与三份自述一致，共 89 条。测试报告自述“一般 15、建议 2”，但逐条看，R-LQ-16～30 是 15 条，R-LQ-31 明确标“一般”，R-LQ-32 是唯一的“建议”，实际为一般 16、建议 1，总数不变。按逐条级别合计是阻塞 10、严重 38、一般 35、建议 6。PRD-live v0.2 修订记录的处理计数按逐条级别统计。

**三份一致的判断**：

- push 主线（收件箱 + 客户端游标 + SDK 去重）能实现，调研事实基本可靠（引用抽查未发现错行）。
- 最大的问题集中在：`pushId` 键范围矛盾；全员推送幂等和崩溃；游标规则和存储；`pushes` 的交付语义；`replayed` 让在线用户静默漏提醒；L1 过关标准过不了；聊天室发送队列和帧分类；未注册用户在迁移中失败。

## 2. 合并对照（多方提出的同一问题）

| 问题 | 编号 |
|---|---|
| `pushId` 键范围矛盾，AC-PUSH-06-3 无法通过 | R-LB-01、R-LF-03、R-LQ-01 |
| 全员 / 房间幂等的崩溃语义 | R-LB-02、R-LQ-04 |
| L1 过关标准和里程碑归属 | R-LB-18、R-LF-18、R-LQ-02 |
| `replayed` 定义 | R-LB-07、R-LF-09、R-LQ-12 |
| 游标规则（混合 TTL、空结果、回退、缺口等待） | R-LB-06、R-LF-08、R-LQ-13 |
| 下行丢弃、队列、帧分类 | R-LB-03、R-LQ-06、R-LQ-07、R-LQ-24 |
| 回房后已删内容留在屏幕 | R-LB-12、R-LF-06 |
| 官方消息 `from` 为空与 Schema 冲突、下发接口语义 | R-LB-11、R-LQ-10 |
| `ttlSeconds` 默认值 | R-LB-19、R-LQ-11、R-LF-23 |
| 未注册用户、昵称、用户导入 | R-LB-15、R-LF-04 |
| 灰度期双写清单 | R-LB-29、R-LF-05、R-LF-16 |
| 敏感词依赖 F-ADM-60 | R-LB-04、R-LQ-29 |
| 5040 重放与现有规定冲突 | R-LB-08、R-LQ-19 |
| 性能目标、环境和样本 | R-LB-13、R-LB-21、R-LQ-08、R-LQ-09 |
| L-17 与 AC-MIG-02-2 | R-LB-17、R-LQ-15 |
| 行为用例（vectors 只验帧格式） | R-LF-13、R-LQ-28 |
| 房内文本 5000 码点过大 | R-LB-30、R-LF-24 |
| 令牌桶顺序和 AC-ROOM-08-1 | R-LB-25、R-LQ-23 |
| 房间生命周期缺 AC | R-LB-24、R-LQ-31 |
| 第 10 节 `custom` 写法和协议遗漏 | R-LB-26、R-LF-10、R-LQ-32 |
| F-MIG 主体不清 | R-LF-15、R-LQ-14 |

## 3. 后端（R-LB）

未核实：SDK 仓库和 console-server 的实际实现；所有性能判断都是量级推算。路径相对 `im-mvp/`。

| 编号 | 级别 | PRD 位置 | 代码 / 文档位置 | 问题 | 建议 |
|---|---|---|---|---|---|
| R-LB-01 | 阻塞（L1） | 4.3、4.4、AC-PUSH-06-3、06-5 | `admin/auth.go:89-93` | `pushId` 键范围三种定义互相矛盾：幂等键是调用方 + pushId，收件箱唯一键是 `(user_id, push_id)`，SDK 只按 `pushId` 去重，帧里没有调用方。AC-06-3 无法通过 | ① `pushId` 全局唯一，跨调用方同号内容不同 4094，删 AC-06-3（推荐）；② 服务端合成下行 ID |
| R-LB-02 | 阻塞（L1） | 4.1、4.3、4.4、4.6、AC-PUSH-06-4、06-6、第 10 节 | `store/migrations/sqlite/`；`hub/hub.go:180-192` | `all`、`room`、`ttlSeconds=0` 的幂等记录和投递顺序没定义，崩溃时不是丢就是重。第 10 节也没有幂等表 | 增加 `push_idempotency`，`pending` 时重试重新广播（至少一次），`done` 时不投递；校验失败不占号；清理晚于收件箱 |
| R-LB-03 | 阻塞（L2） | F-ROOM-08 规则 2、3、AC-ROOM-08-2～4、F-PUSH-05 | `gateway/gateway.go:344-347`、`:397-417`、`:425-426`、`:433-439`；`hub/hub.go:13-21` | “先丢聊天室消息”在现有单 FIFO 上做不到，满 256 帧断开，实际积压约 512 帧，已取走的丢不掉；帧分类没定义 | 增加分类表，写明两条队列上限、跨队列顺序；`hub.Client` 按类别发送；写明不读连接写超时后断开；重估工作量 |
| R-LB-04 | 阻塞（L2，L-9 选 ① 时） | F-ROOM-11、L-9、第 14 节 | `PRD-admin-v1.0.md:1026-1032`；`adminauth/scope.go`；`server/go.mod:26` | F-ADM-60 只有两条 AC，词库接口、scope、表、缓存主题都不存在，AC-ROOM-11-1 的拦截词无法配置；替换模式和检查范围不明 | 写清最小范围：词库来源、scope、失效主题、匹配算法、检查范围、词库负责人；重估 |
| R-LB-05 | 严重（L1） | 4.3、第 10 节 | `store/sqlite.go:154-160`；`docs/tasks/backend-admin.md:333` | `pushSeq` 计数表没列出。`max(push_seq)+1` 在清理后回退，客户端游标大于新序号，新推送补不到 | 新增 `push_seq(user_id, max_seq)`，不随清理删除；`latestSeq` 从它读；加 AC |
| R-LB-06 | 严重（L1） | 4.2、4.3、AC-PUSH-03-4 | `ARCH-v1.md:122` | 游标规则缺失：TTL 空洞、尾部过期不前进、备份恢复后 `latestSeq < afterSeq`、`pushSeq ≤ 游标`的帧、补拉失败时暂存帧 | 写成规则和共享 vectors |
| R-LB-07 | 严重（L1） | L-4、4.5、AC-PUSH-08-1 | `protocol/frames.go:202-207` | `replayed` 不能等于“不用提醒”，网络闪断 3 秒的提现通知只能自己找，是回归 | 以 auth ack 的 `serverTime` 为界 |
| R-LB-08 | 严重（L1） | 4.4、4.7、AC-PUSH-06-6、F-PUSH-09 | `protocol/errors.json:58`；`docs/tasks/console.md:458`；`logic/logic.go:110-126` | 5040 规定“不要重复提交”，这里要求重放；5 分钟间隔和 20 次/分钟可能挡住重放 | 幂等检查先于限速；重放不受这两项约束；更新 5040 `action`；CS-044、045 增加对账 |
| R-LB-09 | 严重（L1） | F-PUSH-09、F-PUSH-02、第 8 节 | `store/sqlite.go:51`；`logic/logic.go:124`；`app/app.go:233` | `target=user` 和 `pushSync` 没有限速和容量上限，收件箱单用户无上限，清理需要索引、大 DELETE 占住唯一连接 | 调用方限速、单用户上限、`pushSync` 限流、`expires_at` 索引、分批清理 |
| R-LB-10 | 严重（通用） | 第 8 节、F-PUSH-01、AC-PUSH-03-7、F-ROOM-09 | `logic/logic.go:85-89`；`errors.json:6`；`app/app.go:63-64`；`logic/moderation.go:127-129`；`PRD-core-v1.3.md:118` | 开关组合未定义：开关关闭时 `pushSync` 得 4001（动作是提示升级）；兼容规则没覆盖未知 `cmd`；V13 关闭时禁言静默失效 | `pushSync` 的 4001 静默；兼容规则补未知 `cmd`；配置校验拒绝启动 |
| R-LB-11 | 严重（L2） | F-ROOM-07 规则 3、AC-ROOM-07-3、第 10 节 | `frame.schema.json` `Message`；`logic/logic.go:180-182` | “`from` 为空”违反 Schema；响应格式、去重键（跨房撞车）、敏感词、限速都缺 | 放开 `from` 或定义官方账号；写响应；去重键 `(调用方, roomId, clientMsgId)`；按调用方和房限速 |
| R-LB-12 | 严重（L2） | F-ROOM-10 规则 4、F-MIG-01、F-ROOM-03 | `gateway/gateway.go:404-408`；PRD 2.7 | 去掉已删 ID 列表后，断线期间被删的消息留在客户端屏幕上，内容合规回退 | `joinRoom` ack 增加 `deletedMsgIds`；加 AC |
| R-LB-13 | 严重（L2） | 1.3、第 8 节、AC-ROOM-08-5、L-14 | `perf-2026-11-03.md`；`gateway/gateway.go:414-416`；`config/config.go:240-245` | （推算）5000 人房目标比已验证的扇出高约 10 倍，没有准入保护，超载就是 OOM | L2 开发前做扇出原型压测；按房发送窗口、共享帧字节；可配置房间和节点上限，启用 4138 |
| R-LB-14 | 严重（L2） | 第 8 节、F-ROOM-04～06、F-PUSH-04 | `delivery/delivery.go:16-29`；`ARCH-v1.md:7`、`:68`、`:77-78`、`:191`；`ratelimit/ratelimit.go:15-20`；`logic/social.go:801` | `Router` 只有按用户投递，`Broadcaster` 断言失败时静默不发；缓冲、去重、关闭状态、已删 ID、限流、顺序都要跨节点；4.6 风险 ② 在单节点不存在、集群才真实存在 | L2 就定义 `RoomRouter` 和房间状态接口；同步 ARCH；低频 `pushSync` 作 B6 前提；给 B6 粗估 |
| R-LB-15 | 严重（L4） | F-ROOM-06 规则 2、F-PUSH-02 规则 3、F-MIG-02、L-11 | `admin/admin.go:224-228`、`:269-314`；`0001_init.sql` | 自动注册用户昵称为空；没有昵称缓存；未注册用户 404 整批不发，双发大面积失败 | 用户全量导入和昵称同步；昵称缓存走 Invalidator；可选跳过不存在的用户 |
| R-LB-16 | 严重（L4） | F-ROOM-07 规则 3、AC-MIG-02-1 | `admin/audit.go:33-38`、`:61-74`；`store/sqlite.go:45-51` | 高频下发接口每次写两条审计，保留 1 年，挤占单聊发送事务 | 拍板：采样或聚合审计，或批量接口；加不回归 AC |
| R-LB-17 | 严重（通用） | L-17、F-PUSH-09 规则 2 | `PRD-admin-v1.0.md:221`、`:993`；`adminauth/caller.go:29-39` | 20 次/分钟没有依据；持有 `push.all` 的密钥可绕开双人复核 | 先拿 N-3；独立调用方、`AllowedCIDRs`、`type` 白名单、审计都改为必选 |
| R-LB-18 | 一般（L1） | AC-PUSH-02-1、3.1、L-16 | — | AC-02-1 属 L1，却要求 `room` 成功 | L1 中 `target=room` 返回错误，AC 拆开，帧格式在 L1 冻结 |
| R-LB-19 | 一般（L1） | F-PUSH-02 规则 4、4.2 | `admin/admin.go:418-421`；`admin/moderation.go:77-94` | `ttlSeconds` 不传和传 0 要区分；默认值与非 user 冲突 | 可选字段；默认只对 `user` 生效；加 AC |
| R-LB-20 | 一般（L1） | 4.4 | — | 内容比对未规范化 | `userIds` 去重排序；`data` 规范化；哈希含 `ttlSeconds`、`roomId`；`accepted` 去重后计 |
| R-LB-21 | 一般（L1） | 4.3、第 8 节 | `store/sqlite.go:51`；`ARCH-v1.md:82`、`:136` | （未实测）千人事务占住唯一连接；Postgres 可能死锁；分片后无法同事务 | 加不回归 AC；按 userId 排序加锁；说明分片后改逐用户幂等写入 |
| R-LB-22 | 一般（L1） | 1.3、F-PUSH-08 规则 2 | — | 游标持久化和回调先后没定义；“100%”过满 | 定义至少一次或至多一次；改成功标准 |
| R-LB-23 | 一般（L2） | F-ROOM-02 规则 1、F-ROOM-03 规则 1 | `gateway/gateway.go:159-161`、`:502-511` | 实时消息可能先于 ack，也可能和 `recent` 重复 | 房间锁内取快照、订阅、ack 入队；或 SDK 暂存；共享 vector |
| R-LB-24 | 一般（L2） | F-ROOM-01、F-ROOM-04、L-8、第 10 节 | — | 生命周期不完整：忘关房、重复 DELETE、`name` 不同、关后 GET、列表接口不一致、重启恢复 | 自动回收；补定义；启动时加载 |
| R-LB-25 | 一般（L2） | F-ROOM-08 规则 1、2、AC-ROOM-08-1 | `ratelimit/ratelimit.go:49-71`、`:17-20`；`PRD-core-v1.3.md:792` | 桶顺序没写；下行不能复用 `TokenBucket` | 先房间桶；下行按房窗口选取；房间桶独立实例 |
| R-LB-26 | 一般（L2） | F-ROOM-06 规则 3、F-ROOM-07 规则 1、第 10 节 | `frames.json:7`；`protocol/frames.go:122-126`、`:152-158`、`:186-189`；`logic/logic.go:432-443`、`:111` | 协议清单遗漏：`frames.json:7` 无 `customType`；`MsgBody` 共用；配置名缺；`send` 第一步查库判重 | 补全；只对 `room` 开放；房间消息提前分支 |
| R-LB-27 | 一般（L4） | F-PUSH-09 规则 1、AC-MIG-02-1 | `admin/audit.go:86-130`、`:158-166` | 审计摘要白名单不含 `userIds`、`roomId`、`{id}`、`pushId` | 路径参数用 `{roomId}`；扩展白名单；`auditNote` |
| R-LB-28 | 一般（L1） | F-PUSH-04 规则 2、AC-PUSH-04-2 | `hub/hub.go:180-192`；`logic/social.go:801-807` | “分批限速”没有参数；没有 `Broadcaster` 时静默跳过仍返回 200 | 写参数或删掉要求；缺广播能力时启动失败 |
| R-LB-29 | 一般（L4） | L-11、F-MIG-03 规则 2 | PRD 2.7、2.8 | 禁言、删除也要双写，否则换通道就能发言；删除 ID 映射 | F-MIG-03 增加双写和 ID 映射 |
| R-LB-30 | 建议（L2） | F-ROOM-06 规则 1 | `protocol/frames.go:60` | 房内 5000 码点约 20KB，乘 10 万帧/秒不可接受 | 查明现状后单设上限 |
| R-LB-31 | 建议（L2） | F-ROOM-10 规则 1 | `admin/moderation.go:29` | 现有删除单次 50，房间 100，没说明原因 | 统一 |
| R-LB-32 | 建议（L1） | 第 8 节、4.3 | `store/store.go`；`ARCH-v1.md:176` | 收件箱明文保存通知，用户注销时如何清除未写 | 列入将来注销范围；负载禁放账号明文 |

已核实无问题：连接补拉窗口安全（`gateway.go:160-174`）；按用户推送的崩溃语义成立；4047、4094、4138、4510 未被占用；`push.user`、`push.all` 已在目录；`routeAny` 可按 target 判 scope；F-ROOM-09 的 `checkMuted` 符合规则；引用行号正确；下行帧大小不受限（`PRD-core-v1.3.md:676`）。

## 4. 前端 / 客户端（R-LF）

路径省略：`Protocol.kt`、`IMClient.kt`、`LocalDatabase.kt` 在 `tianting-sdk-kotlin/sdk-core/src/main/kotlin/com/tianting/sdk/`；`Client.swift`、`Protocol.swift`、`Connection.swift`、`LocalDatabase.swift` 在 `tianting-sdk-swift/Sources/TiantingIM/`；`client.ts`、`connection.ts`、`protocol.ts`、`store.ts`、`emitter.ts` 在 `tianting-sdk-ts/src/`；业务仓库文件名沿用 PRD 2.1 节。

| 编号 | 级别 | PRD 位置 | 代码位置 | 问题 | 建议 |
|---|---|---|---|---|---|
| R-LF-01 | 阻塞 | 4.3、4.4、L-3、AC-PUSH-07-3 | `LocalDatabase.kt:69-75`、`:157-160`、`:347`；`IMClient.kt:85`、`:134`、`:264`、`:849`、`:1175-1178`；`Client.swift:986`、`:810-814`、`:1001`、`:1011-1013`；`LocalDatabase.swift:76-90` | 原生本地库是可丢缓存，退出、换号、被踢、库损坏、Schema 升级都清库，游标回 0，保留期内全部回放。Kotlin `database` 可为 null | 游标独立存放，按 userId 分键；明确无持久化和无游标时的行为；补 AC |
| R-LF-02 | 阻塞 | 1.3、F-PUSH-08、PLAN 3.2 | `IMClient.kt:178`、`:184`、`:225`；`Client.swift:215-236`；`emitter.ts:18-26`；`App.kt:71`、`MainActivity.kt:208` | `pushes` 没有交付语义：无订阅者时丢、慢订阅者丢最旧；游标已前进就补不回来 | L1 冻结交付契约：至少一次、无订阅者暂存、交付后才前进、三端签名 |
| R-LF-03 | 阻塞 | 4.3、4.4、AC-PUSH-06-3 | `frame.schema.json:2216-2230` | 规则矛盾，AC-06-3 写不出来 | 三选一（报告推荐前缀方案，也列了全局唯一） |
| R-LF-04 | 阻塞（L4） | F-PUSH-02 规则 3、AC-PUSH-02-3、AC-ROOM-07-4、F-MIG-02 | `protocol/ADMIN.md:53-54`；PRD-core F-USER-01 规则 3；`chat_room_notifier.dart:368` | 灰度期多数业务用户不存在：提现整批失败、礼物消失、`fromNick` 空；userId 字符集未确认（`im_manager.dart:110`） | F-MIG-02 增加全量导入、昵称同步、字符集核对；部分成功或自动建档二选一；定义 `fromNick` 为空 |
| R-LF-05 | 严重 | F-MIG-03 规则 2、AC-MIG-03-3、4.3 | `FNIMPushManager.swift:137-142` | 双发和收件箱叠加：灰度到时游标为 0，再来一遍 | `pushSync {fromLatest:true}`，或只向已灰度用户写；补 AC |
| R-LF-06 | 严重 | F-ROOM-10 规则 4、F-MIG-01 | `chat_room_notifier.dart:345-358`；`client.ts:429-431`；`FNChatManager.swift:140`；`ChatViewModel.kt:151` | 重连时已删消息还在列表，合规回退 | `deletedMsgIds`；补 AC |
| R-LF-07 | 严重 | 4.3、F-ROOM-03 规则 1、F-ROOM-12 | `client.ts:414-426`；`IMClient.kt:864-876`；`Client.swift:471-487`；`connection.ts:155`、`:318` | 重连时序未定义 | auth 后并行互不阻塞；房间消息不进发送队列；`joinRoom` 改设目标语义；退避 |
| R-LF-08 | 严重 | 4.3、AC-PUSH-03-4、AC-PUSH-08-1、4.5 | `protocol/README.md` 第 3 节规则 7；`client.ts:673`；`Client.swift:780-783` | `pushSync` 算法不完整：缺口等待、单飞、游标推进、回退、`replayed`、回放总量 | 写成状态机和共享行为用例 |
| R-LF-09 | 严重 | L-4、4.5、F-PUSH-08、F-UI-44 规则 2、3 | `tianting-web/src/lib/session.ts:12-26`；`ImProvider.tsx:26-31` | `replayed` 混了离线补收和回放；H5 每次 F5 全量回放；H5、原生列表不一致；回放项排序 | 拆三态；H5 `localStorage` 存 `lastSeenSeq`（D4 例外，要拍板）；按 `ts` 排序；原生列表二选一 |
| R-LF-10 | 严重 | F-ROOM-06、07、12、第 10 节、F-MIG-01 | `Protocol.kt:18`、`:71-75`、`:83`、`:148`；`Protocol.swift:65-69`、`:91`、`:160`；`protocol.ts:30`、`:138-151`；`convid.json:22`；`client.ts:486-488`、`:538-542`；`store.ts:351-355`；`IMClient.kt:948`；`frame.schema.json:255-270`；`server/e2e/e2e_test.go:223-225` | 现有 SDK 模型不能承载房间消息：无 seq 解码失败、TS 污染会话、`customType` 被丢、send `ext` 被 4000 | 新建 `RoomMessage`、`RoomSendAck`，入口分流；补第 10 节 |
| R-LF-11 | 严重 | F-ROOM-12、F-MIG-01、AC-ROOM-11-3 | `PLAN.md:146-157`；`client.ts:269-287`；`chat_room_notifier.dart:595-608` | 聊天室 SDK 接口没定义；AC-11-3 “标记失败”没有东西可标记 | 补接口表；AC 改“返回 4510” |
| R-LF-12 | 严重 | F-ROOM-03、04、AC-ROOM-03-1、F-MIG-04 | `chat_room_notifier.dart:291-301`、`:212` | `recent` 去重歧义：显式 leave 后恢复界面空；自发消息是否已见；ack 与实时帧顺序；迟到帧 | 写死规则，各补 AC |
| R-LF-13 | 严重 | 3.2 L3、AC-ROOM-12-1 | `VectorsTest.kt:141-152`；`VectorTests.swift:130` | `frames.json` 只验不崩溃，证明不了三端一致 | 新增行为用例，L1 落地 |
| R-LF-14 | 严重 | L-12、L-13、第 14 节、F-MIG-01 | `client.ts:415-426`；`IMClient.kt:866-876`；`Client.swift:477-487`；`pubspec.yaml:68`、`:77`；`websocket_client.dart:6`；`fx-sport/app/android/settings.gradle.kts:36` | 业务只要聊天室和推送，但每次重连都跑 `convs`、`socialSync`、本地库；L-12 估算偏大 | “仅直播模式”；sdk-dart 只做子集；粗估 8～12 人日（低置信） |
| R-LF-15 | 严重 | 3.2 L3、L-13、F-MIG-03 | `tianting-sdk-swift/Package.swift`；`ab77/Podfile:131`；`ABChatModule.podspec:26`；`android/settings.gradle:28-31` | SDK 不能被业务工程引入：无 podspec、无 maven-publish；android 版本目录核实不了 | L3 过关加“空分支能编译”；分发列入交付物 |
| R-LF-16 | 严重 | F-MIG-03 规则 1～3、L-11、AC-MIG-03-1～3 | `delete_push_model.dart:25-29`；`App.kt:71`；`FNIMSDKManager.swift:22-25`；`im_manager.dart:63-96`；`live_api.dart:154` | 灰度方案不完整：礼物、官方、禁言、删除、开关房、上报；配置时机 | 双写清单；默认走腾讯、配置未到不切、只在冷启动切；按 userId 哈希 |
| R-LF-17 | 一般 | AC-PUSH-08-2、F-PUSH-08 规则 2 | `emitter.ts:18-26`；`IMClient.kt:226`；`Client.swift:215` | 回调抛异常只对 TS 有意义 | AC 按端改写 |
| R-LF-18 | 一般 | 3.2 L1、AC-PUSH-07-2、第 11 节 | `tianting-android/app/src/main`、`tianting-ios/Kit/Sources`、`tianting-web/src`（grep `notices` 无） | L1 含 android；第 11 节没有两端 App 的 F-UI-44 | L1 限 web + pc；补任务 |
| R-LF-19 | 一般 | 第 5 节、F-ROOM-14、F-MIG-04 | `chat_room_notifier.dart:610`；`errors.json:52` | 交互状态几乎未定义 | 状态 → 界面 → 文案表 |
| R-LF-20 | 一般 | F-MIG-01、F-ROOM-03 规则 3 | `im_manager.dart:46-54`；`FNIMSDKManager.swift:162-174`；`ChatRoomHelper.kt:62-65`；`IMClient.kt:1172-1181`；`Client.swift:810-814` | 过期和被踢都映射 `kicked`；`connect(新 token)` 后是否回房未写 | 分开映射；自动回原房；补 AC |
| R-LF-21 | 一般 | AC-MIG-04-3、F-ROOM-03 | `Connection.swift:115-118`；`connection.ts:139-144`、`:35-36`；`SystemEvents.swift:50` | 回前台可能最长 40 秒才发现连接已死（推论，未实测） | 回前台立即 ping；真机验证 |
| R-LF-22 | 一般 | F-MIG-01 contentType 0 / 6～9 | `GlobalPushHandle.kt:129-138`；`ABPushMessage.swift:25-31`；`im_push_manager.dart:117-121`；`GlobalPushModel.kt:8-9` | 四种事件合成 `liveStatus` 可能还原不了；顶层 `desc`、`type` 无去向 | 拆 4 个 `type` 或 `data` 带 `event`；写去向 |
| R-LF-23 | 一般 | F-MIG-01 contentType 4、L-2 | `withdraw_push_notifier.dart:44-50`、`:56-60`；`FNIMPushManager.swift:137-142` | 对照表没给 `ttlSeconds`；后台丢弃；账号比对要删 | 逐类写 `ttlSeconds`；写后台处理 |
| R-LF-24 | 建议 | F-ROOM-06 规则 1、F-ROOM-08、N-8、F-ROOM-11 规则 2 | `frame.schema.json:58-62`；`Transport.swift:34`、`:73`；`chat_room_notifier.dart:581-608`；`ChatRoomHelper.kt:172-175` | 5000 码点过大；标准化差异；主播文本可能被丢；主播端未核实 | 单设上限；写清标准化；主播优先通道；N-2 列主播端 |
| R-LF-25 | 建议 | 2.8、第 12 节、N-7 | `android/module-common/build.gradle:123`、`:128`；`android/settings.gradle:28-31` | android 设备推送依赖已声明，只是未见调用 | 改写 2.8、N-7 |

已核实无问题：第 2 节引用抽查约 25 处无错行；contentType 与 PRD 一致；计数一致；旧 SDK 忽略 `push`（`IMClient.kt:951`、`client.ts:490`）；未知命令只回 ack 错误（`logic.go:85-89`）；sdk-swift 与 ab77 都是 iOS 15；Swift 传输层已放开 1MB。未核实：android 的 Kotlin / OkHttp 版本和 minSdk、fx-sport 是否发布 Web、业务 userId 字符集、主播端、游客、回前台连接状态。

## 5. 测试（R-LQ）

AC 总体：105 条中 39 条 AC 本身有问题。“合理”“及时”没有出现，实际问题词是“未读满”“功能正常”“约 10%”“一致”。

| 编号 | 级别 | PRD 位置 | 问题 | 建议 |
|---|---|---|---|---|
| R-LQ-01 | 阻塞 | 4.3、4.4、AC-PUSH-06-3、06-5 | 幂等键、唯一键、SDK 去重键不一致，AC-06-3 过不了 | 协议问题，L1 冻结前统一 |
| R-LQ-02 | 阻塞 | 3.2、AC-PUSH-02-1、05-1～3、07-2、L-16 | L1 要求 F-PUSH AC 全部通过，但 F-PUSH-05 是 L2，F-PUSH-08 部分是 L3，AC-ADM-50 要 console 和 H5 就绪 | 每条 AC 标归属；拆 02-1；07-2 写明 L1 用协议客户端 |
| R-LQ-03 | 严重 | 4.3、4.6、第 8 节 | 崩溃恢复没有 AC（`im-loadtest` kill -9、`durability_test.go` 没用上；skip-delivery 只按 `clientMsgId`，见 `faults.go`） | kill -9 已 ack 可补拉；注入提交后跳过下发 |
| R-LQ-04 | 严重 | 4.4、AC-PUSH-06-4、4.6 | 全员幂等 + 崩溃丢整条；失败后同号重试未定义 | 明确顺序和语义 |
| R-LQ-05 | 严重 | 4.3、第 8 节；PRD-social 8.3、8.7 | 并发没有 AC 也没有加锁规则：`pushSeq` 连续、同 `pushId` 并发、Postgres 死锁、并发进出房、关房竞态、整批原子 | 增加“并发与契约测试”，SQLite、Postgres 都跑 |
| R-LQ-06 | 严重 | F-ROOM-08 规则 2、3、F-ROOM-10、AC-ROOM-08-3 | 事件帧是否可丢未定义；`msgDeleted` 被丢等于违规内容留在屏幕；可能先删后到 | `msgDeleted`、`roomClosed` 不可丢且不早于消息；加 AC |
| R-LQ-07 | 严重 | AC-ROOM-08-2、08-5 | 丢弃没有下限，一条不发也算过；P99 分母不清 | 每连接每秒 ≥ 18 且最新；写统计口径、人数、大小 |
| R-LQ-08 | 严重 | AC-ROOM-08-5、AC-PUSH-04-2、第 8 节、P-12 | 性能 AC 没写环境和样本；`im-loadtest` 没有房间和全员场景（`scenarios.go`） | 写环境、样本、窗口、判定；合并 P-12 和 C-50 |
| R-LQ-09 | 严重 | 第 8 节、1.3 | 1000 人推送 ≤ 2 秒、`pushSync` ≤ 200ms、不下降都没 AC | 补 AC，含千人推送期间单聊 P99 |
| R-LQ-10 | 严重 | F-ROOM-07 规则 3、Schema、第 10 节 | 无 `fromUserId` 的官方消息违反 Schema，主用法无 AC；响应、去重、禁言发送者未定义 | 改 Schema；加 AC |
| R-LQ-11 | 严重 | 4.2、F-PUSH-02 规则 4、AC-PUSH-02-1、02-5、AC-ADM-50-3 | `ttlSeconds` 默认值矛盾，console 发全员会 400 | `target=user` 默认 7 天，其他默认 0 |
| R-LQ-12 | 严重 | 4.3、4.5、AC-PUSH-03-4、08-1、L-4 | `replayed` 让在线用户静默漏提醒；无 500ms 等待 | 按 `ts` 早于本次连接建立的确定规则；加反向顺序 AC |
| R-LQ-13 | 严重 | 4.1、4.3、AC-PUSH-03 | 混合 TTL、`hasMore`、`latestSeq`、全过滤时游标未定义；边界无 AC；无 `pushSeq` 帧不动游标无 AC | 明确规则；补 AC |
| R-LQ-14 | 严重 | AC-MIG-01-2、02-1、03-1、03-2、04-1、04-2、04-4、L-13 | F-MIG 不可判定或主体不清：“约 10%”、“功能正常”、无基线；04-1 捆 7 项；02-1 发言不进审计；01-2 无 fx-sport | 逐项拆；给容差和基线；标执行方 |
| R-LQ-15 | 严重 | AC-MIG-02-2、F-PUSH-09 规则 3、L-17、F-MIG-01 | “L-17 选默认时”指向不清，两条 AC 不能同时适用 | 按 L-17 结果写死 |
| R-LQ-16 | 一般 | AC-ROOM-02-1、11-1 | 负向断言会被 `roomOnline` 打断 | 房间用例排除 `roomOnline` |
| R-LQ-17 | 一般 | AC-ROOM-05-2、05-3、02-6、第 8 节 | `clock.Settable` 没有定时器；5 秒节流时序未定义 | 写时序和测试驱动 |
| R-LQ-18 | 一般 | 第 8 节；PRD-social 5.6 | 注入只列名字且漏项：`imLatencyMs`、跳过下发、敏感词 seed、查收件箱、`limits/reset`、关闭清理开关 | 写成接口 + AC 表 |
| R-LQ-19 | 一般 | AC-PUSH-06-6；PRD-admin 第 471、1143 行；CS-044 | 与“不要重复提交”冲突；标【后端】但需要 console | 明确重放条件；改跨服务 AC |
| R-LQ-20 | 一般 | AC-PUSH-03-5 | 离线没有帧，无法观察 | 拆开 |
| R-LQ-21 | 一般 | AC-PUSH-03-6、F-PUSH-03 | `pushSync` 是 WS 命令，写 400 不适用；没测 0 | ack 4000；补 0、1、100 |
| R-LQ-22 | 一般 | AC-PUSH-07-4 | 用例几乎为空；多余字段返回 4000（`e2e_test.go`） | 改写 |
| R-LQ-23 | 一般 | AC-ROOM-08-1；`ratelimit.go` | 令牌桶持续补充，用例不稳定；扣减顺序未写 | 冻结时钟或一次发出；写顺序 |
| R-LQ-24 | 一般 | AC-ROOM-08-3、08-4、07-3 | “未读满”未定义；08-4 只查另一连接；昵称回退未定义 | 明确范围；增同连接场景；定义回退 |
| R-LQ-25 | 一般 | AC-ROOM-14-1 | 没写是否同账号和时限 | 写明 |
| R-LQ-26 | 一般 | AC-ROOM-12-3 | `tianting-sdk-ts/src` 未搜到 indexedDB、sqlite，L2 可能空转（仅关键词） | 改 L3 或断言 store 不写 |
| R-LQ-27 | 一般 | AC-ROOM-02-3、02-4、01-3 | 在线数读法、`match-1` 断言、`reason` 枚举缺 | 补 |
| R-LQ-28 | 一般 | 3.2 L3、第 5 节 | `frames.json` 验不到行为 | 共享行为用例 |
| R-LQ-29 | 一般 | F-ROOM-11、F-ADM-60、AC-ROOM-11-1、11-2 | 替换模式、词库入口、范围未定义 | 只做拦截；定义入口 |
| R-LQ-30 | 一般 | 第 5、6 节 | 多错误优先级未定义 | 优先级表 + AC |
| R-LQ-31 | 一般 | 多处（合并项） | 功能规则没有对应 AC：F-ROOM-01 规则 2、3；02 规则 7；03 规则 3；06 规则 5；07 规则 2、4；08 规则 1、2；09 规则 1；11 规则 2；12 规则 1；F-PUSH-02 规则 2、5；F-PUSH-03 / 4.3；04 规则 2；05；06 / 4.4；09 规则 2；L-4 的 F-UI-44；F-MIG-01～03 | 逐项补 AC |
| R-LQ-32 | 建议 | 第 10 节 | `MsgType` 已有 `custom`，应改为 `body` 增加 `customType` | 改 |

数字依据（并入 R-LQ-08、L-14、L-17）：2 条/秒、20 条/秒、200ms、全员 5 秒、2000 LRU、5 分钟去重、20 次/分钟、5000 人在文中都没有依据。能找到依据的只有 50 条（PLAN）和 4KB（F-ADM-50）。

性能可验收性（均未实测）：AC-PUSH-04-2 开发机可部分验收，发布结论等 C-50 和 P-12；AC-ROOM-08-5 开发机结论不确定，实际需要 P-12 的机器和独立压测机。

核对无误：AC 数 105（55/38/12）、功能点 28、待拍板 17；4047、4094、4138、4510 未被占用；多处引用属实。未核实：第 2 节业务引用（不在测试范围）；`imLatencyMs` 只确认存在。

## 6. 回应

处理结果逐条见 [PRD-live v0.2 修订记录](../prd/PRD-live-v0.2.md)。需要外部提供的数据：N-2（业务服务端如何发礼物和推送）、N-3（现网峰值）、N-8（现有文本长度和发言频率限制）。
