# PRD：直播聊天室与系统推送（二期）

| 项 | 内容 |
|---|---|
| 版本 | **v0.5（草稿，2026-11-05）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-05 |
| 状态 | 草稿。L1 部分：13.1 节 7 项已拍板，[RECHECK-live-v0.3](../review/RECHECK-live-v0.3.md) 的 13 条、[RECHECK-live-v0.4](../review/RECHECK-live-v0.4.md) 的 15 条已处理，`protocol/vectors/push-sync.json` 已扩充并用两份实现跑通，**按 3.3 节冻结**。L2、L3、L4 部分未拍板（13.2 节）。冻结前任务 BE-228、BE-229、CS-044、CS-045、FE-208、FE-353、FE-424、FE-434 继续等待 |
| 对应队列 | [QUEUE](../process/QUEUE.md) Q19 |
| 依据 | [PLAN](../PLAN.md) 第 1 节、2.1～2.3 节；[PRD-core v1.3](PRD-core-v1.3.md) 第 7 节；[PRD-admin v1.0](PRD-admin-v1.0.md) 2.2 节规则 4、F-ADM-50；[PRD-ui v1.0](PRD-ui-v1.0.md) F-UI-44；[任务单](../tasks/backend-admin.md)；三个业务仓库的只读调研（第 2 节） |
| 关联 | 协议 [protocol/README.md](../../protocol/README.md)、Schema `protocol/schema/frame.schema.json`、用例 `protocol/vectors/frames.json`、错误码 `protocol/errors.json`、架构 [ARCH-v1](../arch/ARCH-v1.md) |
| 冻结文档 | 本文不修改任何已冻结 PRD。需要它们配合的地方列在第 11 节，定稿后走变更流程 |
| 阅读提示 | 第 2 节只写事实，推论单独标"推论"，没查到的写"未查到"。标"（L-n）"的规则：L-1～L-4、L-16～L-18 是 Frank 已定的结论；其余按 13.2 节的建议方案起草，拍板结果不同就随之修改。标"未校准"的数字没有现网依据，只是可配置的初始默认值，等 N-2、N-3、N-8 或压测结果校准 |

---

## 修订记录（v0.4 → v0.5）

依据 [RECHECK-live-v0.4](../review/RECHECK-live-v0.4.md) 共 15 条。这轮复核的做法是只凭文字独立实现状态机再跑向量，主线成立（盲写 35/36），问题是规则留白和向量判定力不足。本轮把留白写死，并把向量从 36 个用例扩到 71 个。

| 编号 | 级别 | 处理 | 落点 |
|---|---|---|---|
| RCB-01 | 严重 | 已改：C9 分错误处理。4001 这次连接内不再发，不退避；其他错误和请求超时都按退避重试 | 4.3 节 C9；AC-PUSH-01-4、03-30 |
| RCB-02 | 严重 | 已改：补用例覆盖 10 个未判定的行为，修正用例 8、18 的断言；存活变异全部处理 | `push-sync.json`、8.3 节 |
| RCB-03 | 一般 | 已改：C6 先 C11 再取下一页；C8 不走 C11 | C6、C8；AC-PUSH-03-15 |
| RCB-04 | 一般 | 已改：5 秒是一个定时器，从首次失败或暂存区在失败状态下变为非空时起算，恰好到期就交付当时全部暂存帧 | C9；AC-PUSH-03-33 |
| RCB-05 | 一般 | 已改：C2 在请求进行中或退避时不启动定时器 | C2 |
| RCB-06 | 一般 | 已改：C8 清空暂存，全部能从收件箱补回 | C8；AC-PUSH-03-31 |
| RCB-07 | 一般 | 已改：回调内退订最后一个订阅者时立即停止，游标不动 | C12；AC-PUSH-03-32 |
| RCB-08 | 一般 | 已改：room、`ttlSeconds=0` 的帧补 SDK 行为用例 | AC-PUSH-03-11、`push-sync.json` |
| RCB-09 | 一般 | 已改：停留在系统通知页时新通知不计未查看 | F-PUSH-12 规则 3；AC-PUSH-12-11 |
| RCB-10 | 一般 | 已改：请求超时算失败，迟到的 ack 按请求 id 忽略 | C5、C9；AC-PUSH-03-30 |
| RCB-11 | 建议 | 已改：AC-PUSH-03-25 和用例 0 改成正好 1000 条、10 页 | AC-PUSH-03-25 |
| RCB-12 | 建议 | 已改：列表上限 100，1000 只是收件箱上限的结果，不是另一条规则 | F-PUSH-12 规则 1 |
| RCB-13 | 建议 | 已改：多标签页回退竞态写入已接受风险 ⑧，最多重复提醒一次 | 4.6 节 |
| RCB-14 | 建议 | 已改：向量的 `acs` 不再留空 | `push-sync.json` |
| RCB-15 | 建议 | 已改：退避重试不算新一轮；`fromLatest` 应答忽略 `pushes`、`hasMore`；去重集合按交付先后淘汰、命中不刷新；队列入队不去重；调度点先于网络事件和定时器 | 4.3、4.4、4.8 节；AC-PUSH-06-18 |
| 3.3 缺口 1 | — | 已改：C8 落盘进向量；Kotlin 未注入、Swift `pushes` 未迭代补 AC（L3） | AC-PUSH-03-16、03-34、08-9 |
| 3.3 缺口 2 | — | 已改：AC-PUSH-07-3、07-6 的持久化分支和 08-2 的 Kotlin 分支写明归 L3 | 3.2 节 |
| 3.3 缺口 3 | — | 同 RCB-08 | — |
| 3.3 缺口 4 | — | 已改：AC-PUSH-01-1 写明跨服务，BE-228 和 FE-208 各管一半 | 第 11 节 |

**AC 变化**：修改 AC-PUSH-01-1、01-4、03-11、03-15、03-16、03-20、03-25、08-9；新增 AC-PUSH-03-30～35、06-18、12-11（共 8 条）；没有删除。总数 252 条（聊天室 103、推送 126、迁移 23）。

---

## 修订记录（v0.3 → v0.4）

依据 [RECHECK-live-v0.3](../review/RECHECK-live-v0.3.md) 共 13 条。**根因**：v0.3 把游标拆成两个变量后，无订阅者的路径走不通（RC2-01～03）。本轮不再打补丁，换成更简单的设计：**只有一个持久化游标；没有订阅者时不发 `pushSync`，也不保留带 `pushSeq` 的实时帧**（都能从收件箱补回）。13.1 节 7 项的方向不变。

| 编号 | 级别 | 处理 | 落点 |
|---|---|---|---|
| RC2-01 | 阻塞 | 已改：去掉 `fetchAfter`、交付队列和缺口判断的混用；无订阅者不触发补拉；暂存区有上限 | 4.3 节 C1、C2、C12；AC-PUSH-03-25、08-4、08-5、08-9、08-10 |
| RC2-02 | 严重 | 已改：带 `pushSeq` 的帧不再溢出，溢出丢弃记录整个去掉；只有无 `pushSeq` 的帧有队列 | C4、C6、C7；4.8 节 |
| RC2-03 | 严重 | 已改：不再有带 `pushSeq` 的交付队列，不会重复入队；`pushId` 交付前查、交付时记 | 4.3 节；4.4 节；4.8 节 |
| RC2-04 | 严重 | 已改：AC-PUSH-06-8 只留 4042，4047 部分拆成 AC-PUSH-06-17（L2）；4.4 节和 F-PUSH-02 里的 4047 随 L2 生效；AC-PUSH-02-11 写明调用方持有 `push.room` | 3.2、3.3、4.4 节；F-PUSH-02 规则 7 |
| RC2-05 | 严重 | 已改：`protocol/vectors/push-sync.json` 已写，36 个用例，覆盖复核第三节 7 个场景 | 8.3 节 |
| RC2-06 | 一般 | 已改：C8 把游标直接重置为 0，后面走普通流程，C7 的 `max` 自然得到 `latestSeq`，不需要"重拉模式" | C7、C8 |
| RC2-07 | 一般 | 已改：`pushSynced` 带 `max(latestSeq, 本轮已交付的最大 pushSeq)`，在 C11 之后发；恢复后回放的漏计写入已接受风险 ⑤；回退规则优先于多标签页取 `max` | 4.8 节；4.6 节；F-PUSH-12 规则 4、5 |
| RC2-08 | 一般 | 已改：没有持久化能力时，换账号把游标清零，同实例 u1→u2→u1 从 0 重新回放，靠 `replayed` 和 `lastSeenSeq` 处理 | 4.8 节账号隔离；4.9 节；AC-PUSH-07-6 |
| RC2-09 | 一般 | 已改：断开时取消 500ms、5 秒和退避定时器，暂存保留到重连后合并 | C9；AC-PUSH-03-27 |
| RC2-10 | 一般 | 已改：注册后到下一个调度点才交付积压，同一调度点注册的都能收到 | 4.8 节；AC-PUSH-08-11 |
| RC2-11 | 一般 | 已改：FE-208 增加 F-PUSH-01 的【SDK】AC 并列入解除阻塞清单；FE-424 增加 5040 文案；`im-loadtest` 全员场景由 BE-229 承接 | 第 11 节；3.2 节 |
| RC2-12 | 一般 | 已改：`pushTypes` 校验只在 `IM_PUSH_ENABLED` 开启时做，放在 `app` 启动校验，现有 `x5_test.go`、`moderation_test.go` 默认关闭开关，不受影响 | F-PUSH-09 规则 3；AC-PUSH-09-7、09-9 |
| RC2-13 | 建议 | 已改：Swift 用 `IMSubscription`、`@MainActor`；`AsyncStream` 的交付定义；第 10 节 `L1 / L2` 拆开并补两个开关；8.2 节标里程碑；AC-PUSH-03-26 改 42～141；CS-014 的说明；`push_inbox.push_id` 索引；原生 `lastSeenSeq` 的键名；L-16 的残留措辞 | 对应章节 |

**AC 变化**：修改 AC-PUSH-02-11、03-7、03-17、03-25、03-26、03-27、06-8、07-3、08-4、08-5、08-9、08-10、09-7；新增 AC-PUSH-03-28、03-29、06-17、07-6、08-11、09-9，共 6 条，没有删除。总数 244 条（聊天室 103、推送 118、迁移 23）。

**参考模型发现并修正的矛盾**（模型只用于验证，不进仓库）：① `pushSynced` 如果在 C11 之前发，带的值会小于刚交付的暂存帧，已规定在 C11 之后发。② "已交付的最大 `pushSeq`" 如果不限定"本轮"，恢复后它一直是旧值，序号回退规则永远不触发，已限定本轮、C8 时重新开始。③ "重拉模式"是多余的状态，C8 重置游标为 0 即可。④ `fromLatest` 也遵守"无订阅者不发"，所以从连接到首个订阅之间的推送不会补收，写入已接受风险 ⑦。

---

## 修订记录（v0.2 → v0.3）

依据 [RECHECK-live-v0.2](../review/RECHECK-live-v0.2.md) 共 21 条。13.1 节 7 项是 Frank 的决定，本轮只落实，方向不变。**21 条都已写入本文，没有不改的。**新增功能点 F-PUSH-12；修改 13 条 AC（AC-PUSH-02-8、03-10、03-15、04-2、04-5、06-6、06-7、07-1、07-3、08-5、09-1、11-5，AC-MIG-02-2），新增 27 条 AC（AC-PUSH-02-14、03-23～27、06-13～16、07-5、08-8～10、09-6～8、12-1～10），没有删除。下面"技术选择"一列是本轮直接定下的，理由写在对应章节。

| 编号 | 级别 | 处理 | 落点 | 技术选择 |
|---|---|---|---|---|
| RC-01 | 阻塞 | 已改 | 3.2 节 AC 归属表；AC-PUSH-03-16、08-7 归 L3；AC-PUSH-06-6、06-13、06-15 随 CS-044；AC-PUSH-11-5 改 SQLite 断言，Postgres 版等 im-server 有实现；AC-PUSH-04-2 的 L1 只看开发机 | 发布结论放发布门槛，不放 L1 |
| RC-02 | 严重 | 已改 | C8、C7、AC-PUSH-03-10；4.6 节风险 ④ | 用 `afterSeq:0` 重拉；不加恢复纪元（epoch），长过旧游标的情况记为已接受风险 |
| RC-03 | 严重 | 已改 | 4.3 节两个变量，C1、C6、C7；4.8 节；AC-PUSH-03-25、08-5 | `fetchAfter` 和 `cursor` 分开；`pushId` 交付时才记入"见过"；暂存放不下一页时停止翻页 |
| RC-04 | 严重 | 已改 | 4.5 节规则 4；4.4 节；4.8 节账号隔离；AC-PUSH-07-3、07-5 | `T0`、去重集合、暂存帧按 userId 隔离，退出或换号时清除 |
| RC-05 | 严重 | 已改 | 4.5 节规则 3；新增 F-PUSH-12；AC-PUSH-12-1～10；第 11 节 F-UI-44；R-LF-09 改为已改 | 新增 SDK 事件 `pushSynced`，用来建立 `lastSeenSeq` 基线和处理序号回退 |
| RC-06 | 严重 | 已改 | F-PUSH-09 规则 3；F-PUSH-02 规则 7；第 10 节；AC-PUSH-09-6～8；AC-MIG-02-2 只留一支 | 配置字段 `pushTypes`；持有 `push.all` 必须配，不提供通配符 |
| RC-07 | 严重 | 已改 | 4.7 节对账表；8.2 节新注入 `imDropResponse`；AC-PUSH-06-6、06-13、06-15 | 对账任务自动重放；超过 1 小时（未校准）不再自动重放，转人工；全员重放不重走复核 |
| RC-08 | 严重 | 已改 | 第 11 节重写为确定内容 | `localStorage` 键用 `ui.noticeSeen.{userId}`，保留 `ui.` 前缀，AC-UI-90-2 不用改；错误码 `action` 文案不算改码 |
| RC-09 | 一般 | 已改 | 4.8 节三端签名；AC-PUSH-08-9、08-10 | Kotlin 有界队列 + 单独协程；Swift 增加专用入口 `onPush`，通用 `on` 不算订阅者 |
| RC-10 | 一般 | 已改 | C5、C6、C7、C9、C11；AC-PUSH-03-26、03-27 | 断开时取消进行中的请求，清单飞和退避 |
| RC-11 | 一般 | 已改 | 第 10 节 `push.room` 改 L1；AC-PUSH-02-8 | scope 随 L1 加入目录；优先级不变，AC 写明调用方持有 scope |
| RC-12 | 一般 | 已改 | 4.2 节 `ts`；4.4 节；AC-PUSH-06-7 | 重播沿用首次受理时间，幂等表存 `first_ts`；8 天内允许重播 |
| RC-13 | 一般 | 已改 | 4.4 节保留行；AC-PUSH-06-14 | 幂等清理以"无对应收件箱行"为条件；写明 `pushId` 过期后也不应复用 |
| RC-14 | 一般 | 已改 | 4.3 节回放总量；F-PUSH-09 规则 2；AC-PUSH-04-5 | SDK 固定 `limit=100`；`pushSync` 默认 120 次/分钟（未校准，10 连接 × 10 页） |
| RC-15 | 一般 | 已改 | 文首、第 0 节、第 4 节标题、4.7 节、第 11 节、13.1 节 | — |
| RC-16 | 一般 | 已改 | 新增 3.3 节冻结清单；8.3 节；4.3 节并行规则；AC-PUSH-03-24 | 冻结时出 `PRD-live-v1.0`，只冻结 3.3 节条目；`push-sync.json` 冻结前写好 |
| RC-17 | 一般 | 已改 | F-PUSH-01；第 8 节兼容；AC-PUSH-02-14 | 开关关闭时路由不注册；和 `IM_V13_ADMIN_FEATURES` 互相独立 |
| RC-18 | 建议 | 已改 | 4.2 节；F-PUSH-03；AC-PUSH-03-23 | 都不传、同传、`fromLatest:false` 都是 4000 |
| RC-19 | 建议 | 已改 | 1.3 节 | — |
| RC-20 | 建议 | 已改 | 4.4 节内容比对；AC-PUSH-06-16；AC-PUSH-07-1 | 哈希保留数字原文 |
| RC-21 | 建议 | 已改 | 4.9 节 | 按批落盘；Kotlin 注入 `PushCursorStore` 接口 |

---

## 修订记录（v0.1 → v0.2）

依据 [REVIEW-live-v0.1](../review/REVIEW-live-v0.1.md) 共 89 条问题。处理分四种：**已改**（规则和 AC 已写入本文）、**转待拍板**（技术部分已写，取舍在第 13 节）、**转 L2 前**（L2 开发或冻结前必须完成，本文写了要求但没法现在完成）、**不改**（附理由）。AC-PUSH-06-3 已删除。

| 编号 | 级别 | 处理 | 落点和说明 |
|---|---|---|---|
| R-LB-01 | 阻塞 | 已改 | 4.4 节 `pushId` 全局唯一；AC-PUSH-06-11 |
| R-LB-02 | 阻塞 | 已改 | 4.4 节幂等表、`pending` / `done`；AC-PUSH-06-4、06-7、06-8 |
| R-LB-03 | 阻塞 | 已改 | F-ROOM-08 帧分类表、两条队列、跨队列顺序；工作量重估第 14 节 |
| R-LB-04 | 阻塞 | 已改 | F-ROOM-11 最小范围；词库内容和负责人转 L-9 |
| R-LB-05 | 严重 | 已改 | 4.3 节 `push_seq` 表；AC-PUSH-03-13 |
| R-LB-06 | 严重 | 已改 | 4.3 节游标规则 C1～C10；8.3 节行为用例 |
| R-LB-07 | 严重 | 已改 | 4.5 节 `replayed` 按 `T0`；AC-PUSH-08-1；展示转 L-4 |
| R-LB-08 | 严重 | 已改 | 4.4 节重放；AC-PUSH-06-6、06-10；PRD-admin 变更列第 11 节 |
| R-LB-09 | 严重 | 已改 | F-PUSH-09 规则 2、4；AC-PUSH-09-3～5。数字未校准 |
| R-LB-10 | 严重 | 已改 | F-PUSH-01、第 8 节兼容；AC-PUSH-01-4、01-5、AC-ROOM-09-4 |
| R-LB-11 | 严重 | 已改 | F-ROOM-07 规则 3、4、4a；第 10 节；AC-ROOM-07-8～11 |
| R-LB-12 | 严重 | 已改 | F-ROOM-10 规则 4；AC-ROOM-10-5、10-6 |
| R-LB-13 | 严重 | 转 L2 前 | 上限规则和判定已写（F-ROOM-02 规则 8、8.1 节），扇出原型压测没做，默认值转 L-14 |
| R-LB-14 | 严重 | 转 L2 前 | 第 8 节集群原则已写；`RoomRouter` 接口设计和 B6 工作量要在 L2 前给出 |
| R-LB-15 | 严重 | 已改 | F-MIG-02 规则 6、F-ROOM-06 规则 2；部分成功转 L-19 |
| R-LB-16 | 严重 | 转待拍板 | L-20 |
| R-LB-17 | 严重 | 转待拍板 | L-17 条件改必选；20 次/分钟标未校准，等 N-3 |
| R-LB-18 | 一般 | 已改 | 3.2 节 AC 归属；AC-PUSH-02-1、02-7、02-8 |
| R-LB-19 | 一般 | 已改 | F-PUSH-02 规则 4；AC-PUSH-02-12 |
| R-LB-20 | 一般 | 已改 | 4.4 节内容比对；AC-PUSH-06-9 |
| R-LB-21 | 一般 | 已改 | F-PUSH-11 规则 1；AC-PUSH-04-4、11-5 |
| R-LB-22 | 一般 | 已改 | 1.3 节、4.8 节至少一次；AC-PUSH-08-7 |
| R-LB-23 | 一般 | 已改 | F-ROOM-02 规则 1；AC-ROOM-12-8；`room-session.json` |
| R-LB-24 | 一般 | 已改 | F-ROOM-01 规则 1、3、5、6；AC-ROOM-01-6～9；回收时长在 L-8 |
| R-LB-25 | 一般 | 已改 | F-ROOM-08 规则 1、2；AC-ROOM-08-1 |
| R-LB-26 | 一般 | 已改 | 第 10 节；F-ROOM-06 规则 5、6；AC-ROOM-06-9 |
| R-LB-27 | 一般 | 已改 | F-PUSH-09 规则 1；路径参数统一 `{roomId}` |
| R-LB-28 | 一般 | 已改 | F-PUSH-04 规则 2；AC-PUSH-04-6 |
| R-LB-29 | 一般 | 已改 | F-MIG-03 规则 2；AC-MIG-03-5 |
| R-LB-30 | 建议 | 转待拍板 | L-21 |
| R-LB-31 | 建议 | 已改 | F-ROOM-10 规则 1 统一 50；AC-ROOM-10-3 |
| R-LB-32 | 建议 | 已改 | 第 8 节隐私；注销路径目前不存在，只记录 |
| R-LF-01 | 阻塞 | 已改 | 4.9 节游标独立存储；AC-PUSH-03-16 |
| R-LF-02 | 阻塞 | 已改 | 4.8 节交付契约；AC-PUSH-08-4～7；1.3 节 |
| R-LF-03 | 阻塞 | 已改 | 同 R-LB-01。报告推荐前缀方案，这里选全局唯一，理由见 4.4 节：不改帧，调用方用 UUID 即可 |
| R-LF-04 | 阻塞 | 已改 | F-MIG-02 规则 6；AC-MIG-02-4；`fromNick` 回退见 F-ROOM-06；部分成功转 L-19 |
| R-LF-05 | 严重 | 已改 | `fromLatest`、F-MIG-03 规则 3；AC-PUSH-03-17、03-22、AC-MIG-03-4 |
| R-LF-06 | 严重 | 已改 | 同 R-LB-12 |
| R-LF-07 | 严重 | 已改 | F-ROOM-03 规则 4、5；AC-ROOM-03-7～9 |
| R-LF-08 | 严重 | 已改 | 4.3 节 C1～C10；AC-PUSH-03-18～20 |
| R-LF-09 | 严重 | 已改（v0.3） | v0.2 时转待拍板；13.1 节 L-4、L-18 已定，v0.3 写入 F-PUSH-12。没采用报告的三态字段：`pushSeq` 加业务比较已能区分，不必多一个协议概念 |
| R-LF-10 | 严重 | 已改 | F-ROOM-12 规则 2；第 10 节；AC-ROOM-12-4 |
| R-LF-11 | 严重 | 已改 | F-ROOM-12 规则 1、6；AC-ROOM-11-3、12-9 |
| R-LF-12 | 严重 | 已改 | F-ROOM-12 规则 4、5；AC-ROOM-05-4、12-5～8 |
| R-LF-13 | 严重 | 已改 | 8.3 节；3.2 节 L3 过关标准 |
| R-LF-14 | 严重 | 转待拍板 | 仅直播模式写入 F-ROOM-12 规则 7、AC-ROOM-12-11；形态和工期转 L-12 |
| R-LF-15 | 严重 | 转待拍板 | L3 过关标准已加；分发责任转 L-13 |
| R-LF-16 | 严重 | 已改 | F-MIG-03 规则 1、2；AC-MIG-03-5、03-6；方案取舍仍在 L-11 |
| R-LF-17 | 一般 | 已改 | F-PUSH-08 规则 2；AC-PUSH-08-2 |
| R-LF-18 | 一般 | 已改 | 3.2 节；第 11 节补两端 App 任务 |
| R-LF-19 | 一般 | 已改 | F-ROOM-14 状态表；AC-ROOM-14-3 |
| R-LF-20 | 一般 | 已改 | F-MIG-01 分开映射；F-ROOM-03 规则 6；AC-ROOM-03-6 |
| R-LF-21 | 一般 | 已改 | F-ROOM-03 规则 7；AC-ROOM-03-10（真机，未实测） |
| R-LF-22 | 一般 | 已改 | F-MIG-01 `liveStatus` 带 `event`、顶层字段去向 |
| R-LF-23 | 一般 | 已改 | F-MIG-01 提现 `ttlSeconds=0`；L-2 |
| R-LF-24 | 建议 | 转待拍板 | L-21；标准化写入 F-ROOM-11 规则 2；主播端列入 N-2。主播优先通道不做，主播发言可走服务端下发 |
| R-LF-25 | 建议 | 已改 | 2.8 节、N-7 |
| R-LQ-01 | 阻塞 | 已改 | 同 R-LB-01 |
| R-LQ-02 | 阻塞 | 已改 | 3.2 节过关标准和 AC 归属表 |
| R-LQ-03 | 严重 | 已改 | F-PUSH-11；AC-PUSH-11-1、11-2 |
| R-LQ-04 | 严重 | 已改 | 同 R-LB-02；4.6 节已接受风险 |
| R-LQ-05 | 严重 | 已改 | F-PUSH-11、F-ROOM-15 |
| R-LQ-06 | 严重 | 已改 | F-ROOM-08 规则 3、5；AC-ROOM-08-6、08-7 |
| R-LQ-07 | 严重 | 已改 | AC-ROOM-08-2、08-5 下限和统计口径 |
| R-LQ-08 | 严重 | 已改 | 8.1 节环境、样本、判定；目标机本身在 P-12、L-14 |
| R-LQ-09 | 严重 | 已改 | AC-PUSH-04-4、04-5、AC-ROOM-15-4 |
| R-LQ-10 | 严重 | 已改 | 同 R-LB-11 |
| R-LQ-11 | 严重 | 已改 | 同 R-LB-19；4.7 节 |
| R-LQ-12 | 严重 | 已改 | 同 R-LB-07；C2 等 500ms；AC-PUSH-06-5 两种顺序 |
| R-LQ-13 | 严重 | 已改 | C7；AC-PUSH-03-8、03-9、03-11、03-12 |
| R-LQ-14 | 严重 | 已改 | 第 7 节 AC 重写，标【业务方】；执行范围在 L-13 |
| R-LQ-15 | 严重 | 已改 | AC-MIG-02-2 按 L-17 写死两支 |
| R-LQ-16 | 一般 | 已改 | 第 5 节开头 |
| R-LQ-17 | 一般 | 已改 | F-ROOM-05 规则 2；8.2 节窗口推进；AC-ROOM-05-2、05-3 |
| R-LQ-18 | 一般 | 已改 | 8.2 节 |
| R-LQ-19 | 一般 | 已改 | AC-PUSH-06-6 改跨服务；4.7 节 |
| R-LQ-20 | 一般 | 已改 | AC-PUSH-03-5 |
| R-LQ-21 | 一般 | 已改 | AC-PUSH-03-6 |
| R-LQ-22 | 一般 | 已改 | AC-PUSH-07-4、03-21 |
| R-LQ-23 | 一般 | 已改 | AC-ROOM-08-1 |
| R-LQ-24 | 一般 | 已改 | AC-ROOM-08-3、08-4、08-8；AC-ROOM-07-3、06-8 |
| R-LQ-25 | 一般 | 已改 | AC-ROOM-14-1 |
| R-LQ-26 | 一般 | 已改 | AC-ROOM-12-3 改 L3，新增 12-4 |
| R-LQ-27 | 一般 | 已改 | AC-ROOM-02-3、02-4、01-3；`reason` 枚举 |
| R-LQ-28 | 一般 | 已改 | 同 R-LF-13 |
| R-LQ-29 | 一般 | 已改 | F-ROOM-11 规则 1、3、4；AC-ROOM-11-4～6 |
| R-LQ-30 | 一般 | 已改 | F-PUSH-02 规则 7；F-ROOM-15 规则 1 |
| R-LQ-31 | 一般 | 已改 | 逐项补 AC。F-MIG-03 的下线门槛数值留给 Frank，F-UI-44 的 AC 随 L-4 |
| R-LQ-32 | 建议 | 已改 | 第 10 节 |


---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 这份文档解决什么 | 定义替换腾讯 IM 所需的两块能力：直播聊天室、系统推送；回答冻结 PRD 留下的三个 push 问题；给出三端迁移的需求依据 |
| 离线补收（已定） | 按用户推送写入收件箱，按用户分配递增的 `pushSeq`；客户端每次连接成功后用 `pushSync {afterSeq}` 补拉；默认保留 7 天，到期不再补收。全员、按房间推送只投递在线连接（L-1）。游标规则见 4.3 节，独立于本地库存储，交付契约见 4.8 节 |
| `pushId` 去重（已定） | `pushId` **全局唯一**，调用方生成（建议 UUID）；服务端按 `pushId` 幂等，重复提交不重复投递，同号但调用方或内容不同返回 409 / 4094；SDK 再按 `pushId` 去重，消除实时投递和补拉的重叠 |
| 多端语义（已定） | 同一用户的每个在线连接各收一次；每台设备自己保存游标、自己补收；"收到"和"已读"不跨端同步（L-3） |
| 调研最重要的三点 | ① 三端都把"全局推送"做成加入一个共用的群，监听 `onReceiveRESTCustomData`，没有 `pushId` 和补收。② 客户端代码里没有发礼物、官方消息的调用，也没有调用在线人数接口。③ `AVChatRoom` 字样在三个仓库 0 命中，建群和 REST 下发都在业务服务端，**本次未调研** |
| 拍板状态 | **解除第二批阻塞的 7 项（L-1、L-2、L-3、L-4、L-16、L-17、L-18）Frank 已全部按建议定下**（13.1 节）。**整体迁移的 14 项未定**（13.2 节），影响最大的是 L-12（fx-sport 是 Flutter）、L-9（敏感词）、L-11（过渡期）、L-14（容量和压测环境） |
| 规模 | 31 个功能点（聊天室 15、推送 12、迁移 4），AC 共 252 条（聊天室 103、推送 126、迁移 23）；拍板项 21 项（7 项已定） |
| 与现有规划的差异 | PLAN 把礼物 / 点赞消息列为 P1；调研显示三端都依赖礼物消息，本文建议把通用自定义消息提到 P0（L-10）。点赞未在聊天室代码里查到，不纳入 |

---

## 1. 背景与目标

### 1.1 背景

PRD-core v1.3 第 7 节把"直播聊天室"列为二期 P0，把 push 协议（离线补收、`pushId` 去重、多端语义）列为二期定稿项。PRD-admin 2.2 节规则 4 规定 F-ADM-50 依赖这份协议；目前第二批的系统通知相关任务都停在这一步（QUEUE Q14、Q15）。

服务端现状：`joinRoom`、`leaveRoom` 和 room 类型的 `send` 明确返回"尚不可用"（`server/internal/logic/logic.go:83-84`、`:154`），端到端测试也在断言这一点（`server/e2e/e2e_test.go:226`）。Schema 和 `frames.json` 已有帧的雏形（`frames.json:12-13`、`:26`、`:28`、`:45`）。

### 1.2 目标

1. 三个业务 App 现有的聊天室和站内推送用法，能在天听上找到对应能力（第 2 节 → 第 5～7 节）。
2. 定稿 push 协议，解除第二批系统通知的阻塞。
3. 迁移过程可灰度、可回退，用户侧发言门槛、提示等行为不回归。

### 1.3 成功标准

| 指标 | 目标（草案，容量以 L-14 为准） | 度量方法 |
|---|---|---|
| 推送不丢 | 保留期内的按用户推送：服务端可查询到；SDK **至少一次**交付给已订阅的业务代码（4.8 节）；同一 SDK 实例按 `pushId` 去重。业务代码自身崩溃或丢弃不在此指标内。**例外**：单用户收件箱超过上限（1000 条，未校准）时最旧的被淘汰（AC-PUSH-09-5）；从备份恢复时见 4.6 节风险 ④ | F-PUSH-03、F-PUSH-06、F-PUSH-08 的 AC |
| 推送实时 | 在线用户 3 秒内收到（与 AC-ADM-50-1 一致） | AC-PUSH-01-1 |
| 聊天室可用 | 单房 5000 人在线（未校准，L-14）、消息高峰时，服务端不因聊天室断开正常读取的连接，实时消息 P99 ≤ 1 秒 | 压测，AC-ROOM-08-5 |
| 能力覆盖 | 第 2 节列出的消息类型和推送类型全部有对应方案 | F-MIG-01 对照表评审 |
| 不回归 | 现有 IM 不受影响（按 PRD-social 1.3 节"不回归"口径） | AC-PUSH-04-4、AC-ROOM-08-4、AC-ROOM-15-4 |

---

## 2. 现状调研：三端如何使用腾讯 IM

> **调研方式**：只读源码，没有构建和运行。范围是 fx-sport、ab77、android 的工作目录，排除了 `build`、`Pods`、`.git`、`.dart_tool`、`.gradle` 和 ab77 的 `.claude/worktrees`。业务服务端不在这三个仓库，因此凡是"服务端如何做"的问题，这里都只有客户端侧的证据。账号、密钥、AppID、群 ID 的具体值没有抄入本文。

### 2.1 文件索引

下文用"文件名:行号"引用：

| 文件名 | 完整路径 |
|---|---|
| `im_manager.dart`、`im_chat_manager.dart`、`url_manage.dart` | `fx-sport/modules/common/lib/manager/` |
| `im_push_manager.dart` | `fx-sport/modules/common/lib/site/push/service/` |
| `push_notifier.dart`、`goal_push_notifier.dart`、`withdraw_push_notifier.dart` | `fx-sport/modules/common/lib/site/push/provider/` |
| `ban_push_model.dart`、`delete_push_model.dart` | `fx-sport/modules/common/lib/site/push/model/` |
| `live_api.dart` | `fx-sport/modules/common/lib/repository/live/` |
| `chat_room_notifier.dart`、`comment_allow_notifier.dart` | `fx-sport/modules/sport/lib/sport_detail/pages/chat_room/providers/` |
| `chat_message_item.dart`、`message_type.dart`、`msg_bean.dart` | `fx-sport/modules/sport/lib/sport_detail/pages/chat_room/models/` |
| `chat_room_page.dart` | `fx-sport/modules/sport/lib/sport_detail/pages/chat_room/` |
| `live_item_card.dart` | `fx-sport/modules/live/lib/live/widgets/` |
| `FNIMSDKManager.swift`、`FNChatManager.swift`、`FNIMPushManager.swift`、`FNMessageModel.swift`、`FNChatApi.swift`、`FNLiveRoomService.swift` | `ab77/Local/ABChatModule/Module/` |
| `ABPushMessage.swift` | `ab77/Local/ABProjectCommon/Data/Models/ABPushModels/` |
| `ABServerConfig.swift` | `ab77/Local/ABProjectCommon/Data/Config/` |
| `AppDelegate+SDK.swift` | `ab77/Fenix/` |
| `YIChatViewModel.swift` | `ab77/Local/Skins/YIUI/Classes/SportDetail/ABChatView/` |
| `YIMatchDetailViewModel.swift` | `ab77/Local/Skins/YIUI/Classes/SportDetail/ViewModel/` |
| `ChatRoomHelper.kt`、`MsgBean.kt`、`ChatViewModel.kt`、`ChatFragment.kt` | `android/module-sports-detail/sports-detail-internal/src/main/java/cam/android/hp/ab/sports/detail/internal/sports/chat/` |
| `GlobalPushManager.kt`、`GlobalPushHandle.kt` | `android/app/src/main/java/cam/android/hp/ab/push/` |
| `GlobalPushModel.kt` | `android/app/src/main/java/cam/android/hp/ab/push/model/` |
| `App.kt`、`AppHelper.kt` | `android/app/src/main/java/cam/android/hp/ab/` |
| `MainActivity.kt` | `android/app/src/main/java/cam/android/hp/ab/main/` |
| `LiveApi.kt` | `android/module-live/live-internal/src/main/java/cam/android/hp/ab/live/internal/source/` |

### 2.2 SDK 与登录

| 项 | fx-sport（Flutter） | ab77（iOS） | android |
|---|---|---|---|
| SDK | `tencent_cloud_chat_sdk` 8.8.7357（`fx-sport/pubspec.yaml:89`） | `TXIMSDK_Plus_iOS_XCFramework` 9.0.7654（`ab77/Podfile.lock:359`） | `libs.tencent.im.sdk`（`android/module-common/build.gradle:108`）；版本号未查到 |
| 初始化 | `im_manager.dart:182-187` | `FNIMSDKManager.swift:39-47` | `ChatRoomHelper.kt:32-42`，启动时调用（`App.kt:71`） |
| 取登录凭证 | 业务接口取 `userId`、`userSig`（`im_manager.dart:100`、`:110`） | `/live/basic/genUserSig`（`FNIMSDKManager.swift:56-69`、`FNChatApi.swift:113-114`） | `getChatRoomUserSig`（`ChatRoomHelper.kt:76-78`） |
| 登录时机 | 懒登录，聊天室和推送共用 `ensureLoggedIn`（`im_manager.dart:63-96`） | 平台登录通知触发（`FNIMSDKManager.swift:22-25`） | 进聊天室或初始化推送时（`ChatViewModel.kt:125`、`GlobalPushManager.kt:58`） |
| 凭证过期 | 登出后重新取凭证登录（`im_manager.dart:46-54`） | 同左（`FNIMSDKManager.swift:162-174`） | **只写日志**（`ChatRoomHelper.kt:62-65`） |
| 被踢下线 | 只重置登录标记（`im_manager.dart:39-41`） | 只写日志（`FNIMSDKManager.swift:155-159`） | 只写日志（`ChatRoomHelper.kt:57-60`） |
| 登出 | 重置登录标记（`im_manager.dart:149-166`），另外退聊天室（`im_chat_manager.dart:68-70`） | 登出、退推送群、移除监听（`FNIMSDKManager.swift:27-35`） | 断开推送、退出 IM（`AppHelper.kt:105-106`） |

### 2.3 进出房间

| 项 | fx-sport | ab77 | android |
|---|---|---|---|
| 群 ID 来源 | 业务接口 `/live/basic/event/im/groupId`（`live_api.dart:129`、`chat_room_notifier.dart:126-131`）；主播群由外部注入（`chat_room_notifier.dart:142-145`、`:160-165`） | 同接口（`FNChatApi.swift:115-116`、`FNChatManager.swift:81-86`） | 同接口（`LiveApi.kt:76`） |
| 进房 | `joinGroup`（`im_chat_manager.dart:123-126`） | `joinGroup`（`FNChatManager.swift:154-191`） | `joinGroup`（`ChatRoomHelper.kt:127-144`，调用见 `ChatViewModel.kt:125-128`） |
| 已在群内 | 错误码 10013 视为成功（`im_chat_manager.dart:127-128`） | 未单独处理 10013 | 10013 视为成功（`ChatRoomHelper.kt:135-138`） |
| 同时在几个业务房 | 一个（`im_chat_manager.dart:44-45`），换房先退旧（`:114-120`） | 一个；进新群前退其他群，过时的加群结果会被退掉（`FNChatManager.swift:31-36`、`:158-159`、`:166-170`） | 一个，进新房先退旧房（`ChatViewModel.kt:99`） |
| 赛事群 / 主播群切换 | 有，按主播和视频开关切换（`chat_room_notifier.dart:49-50`、`:183-192`），串行锁（`:206-210`），切换时清空消息（`:212`） | 有配置和直播间判断（`YIChatViewModel.swift:376-390`） | 未查到同等的双群切换逻辑（只查了聊天室目录） |
| 进下级页面 | 退群，返回后重连（`chat_room_notifier.dart:291-301`） | 有 `clearChatConnect` / `resumeChatConnect`（`YIMatchDetailViewModel.swift:169-175`） | 未查到 |
| 退房 | `quitGroup`，幂等（`im_chat_manager.dart:210-225`） | 退群失败保留记录，下次再退（`FNChatManager.swift:207-227`） | `quitGroup`（`ChatRoomHelper.kt:234-246`、`ChatViewModel.kt:383-386`） |
| 进房后的业务动作 | `/live/basic/live/init` 取禁言、已删消息、角色，同时隐含"观众进入"上报（`im_chat_manager.dart:140-142`、`:148-158`） | 同接口（`FNChatManager.swift:126-141`、`:178-184`） | 同接口（`ChatViewModel.kt:145-152`、`LiveApi.kt:95`） |
| 观看时长 | 30 秒上报一次（`im_chat_manager.dart:179-186`、`:197`） | 接口 `/live/basic/audience/duration`（`FNLiveRoomService.swift:26-28`） | 定时上报（`ChatViewModel.kt:519-536`） |
| 历史消息 | 未查到拉取调用；依赖进群后的下发 | `getHistoryMessageList` 存在但调用被注释，注释说"服务器配置后历史消息会从接收消息推过来"（`FNChatManager.swift:172-174`、`:234-255`） | 未查到拉取调用 |
| 群免打扰 | 进房后设置，离线不推不计未读（`im_chat_manager.dart:139`、`:161-176`） | 未查到；用 APNS 监听把角标数返回 0（`FNIMPushManager.swift:166-172`） | 未查到 |

### 2.4 消息类型与自定义消息

消息来自两种元素：文本元素和自定义元素。自定义元素的结构三端一致：`{"type": 整数, "data": 对象}`。发送者 VIP 等级放在 `cloudCustomData`（`{"vipLevel": n}`）。

| `type` | 含义 | fx-sport | ab77 | android | 谁发出（客户端可见范围） |
|---|---|---|---|---|---|
| （文本） | 会员 / 主播文本，按"发送者 == 当前主播"区分 | `chat_room_notifier.dart:365-383`、`:341` | `FNChatManager.swift:523-534` | `MsgBean.kt:191-215` | 客户端（`chat_room_notifier.dart:577-592`、`FNChatManager.swift:307-373`、`ChatRoomHelper.kt:149-153`） |
| 1 | 系统消息 `{text}` | 按文本去重（`chat_room_notifier.dart:333-338`），渲染为空（`chat_room_page.dart:433`），配置文案走跑马灯（`chat_room_page.dart:349`） | **本地构造，不经 SDK 发送**（`FNChatManager.swift:257-279`） | 本地构造（`ChatViewModel.kt:110-119`），也能解析 IM 下发的（`MsgBean.kt:223-236`） | IM 下发是否真实存在：未查到 |
| 2 | 官方消息 `{text, jumpType, eventId, liveRoom, url, anchorId}` | `chat_room_notifier.dart:387-396` | `FNMessageModel.swift:185-199` | `MsgBean.kt:238-252`、`:324-336` | 客户端没有发送调用；推论：业务服务端下发 |
| 3 | 晒单 `{orderType, betName, betItemName, odds, amount, winAmount, oddType, settlementStautus, marketId, ty}` | `chat_room_notifier.dart:616-659`；字段名 `settlementStautus` 带拼写错误（`msg_bean.dart:59`） | `FNMessageModel.swift:112-123` | `MsgBean.kt:93-107`、`:253-269`、`ChatRoomHelper.kt:155-166` | 客户端 |
| 5 | 礼物 `{orderId, orderPrice, id, type(0/1/2), message, name, price, count, smallPhotoUrl, bigPhotoUrl, bigGifUrl}` | `chat_room_notifier.dart:407-416`、`:429-458` | `FNMessageModel.swift:126-163` | `MsgBean.kt:53-65`、`:271-292` | 客户端没有发送调用，走 HTTP `/live/gift/send`（`live_api.dart:154`、`FNLiveRoomService.swift:21-23`、`ChatViewModel.kt:490-496`）；推论：业务服务端下发 |

**未知类型的处理三端不同**：fx-sport 回退成系统消息后渲染为空（`message_type.dart:12-15`、`chat_room_notifier.dart:421-425`）；ab77 映射为 `.unknown`（`FNMessageModel.swift:13-16`）；android 返回 `null` 丢弃（`MsgBean.kt:297`）。

**消息用到的字段**：`msgID`、`sender`、`nickName`、`timestamp`、`cloudCustomData`、`groupID`（如 `chat_room_notifier.dart:362-383`、`FNChatManager.swift:525-533`、`MsgBean.kt:196-204`）。昵称由客户端打码后显示（`chat_room_notifier.dart:460-488`、`FNMessageModel.swift:86-97`），三端规则不完全一致。**昵称来自消息本身**，这决定了天听的聊天室消息必须带发送者昵称（F-ROOM-06）。

**发送结果处理**：fx-sport 和 ab77 在发送成功后把消息加进本地列表（`chat_room_notifier.dart:595-608`、`FNChatManager.swift:353-354`）。android 发送时 `onlineUserOnly = true`（`ChatRoomHelper.kt:175`）；ab77 默认 `false`（`FNChatManager.swift:311`）。

### 2.5 在线人数、礼物、点赞

| 能力 | 事实 |
|---|---|
| 聊天室在线人数 | 三端**未查到**相关 SDK 调用或事件（搜索 `getGroupOnlineMemberCount`、`onlineMemberCount`、`onMemberEnter`、`onMemberLeave` 均 0 命中）。fx-sport 列表卡片的"观看人数"来自 HTTP 列表字段 `chatRoomNum`（`live_item_card.dart:101`、`:111`） |
| 礼物 | 列表 `/live/basic/gift/onlineList`、送礼 `/live/gift/send` 都走业务 HTTP（`live_api.dart:150`、`:154`；`FNChatApi.swift:123-126`；`LiveApi.kt:107`、`:110`）。送礼按钮只在主播群显示（`chat_room_notifier.dart:284-287`） |
| 点赞 | 聊天室代码中**未查到**。三端的点赞是主播动态接口 `/live/anchor/news/like`（`live_api.dart:123`、`LiveApi.kt:142-143`、`ab77/Local/ABSportCommon/LiveListManager/LiveListV2/Data/LiveHome/ABLiveHomeAPI.swift:47`），与 IM 无关 |

### 2.6 回调监听

| 回调 | fx-sport | ab77 | android |
|---|---|---|---|
| 新消息 `onRecvNewMessage` | 过滤推送群后转发（`im_chat_manager.dart:56-61`） | 只收当前目标群（`FNChatManager.swift:462-475`） | 只收当前群（`ChatRoomHelper.kt:276-282`） |
| `onRecvMessageRevoked` | 未查到 | 只写调试日志（`FNChatManager.swift:484-491`） | 未查到 |
| `onRecvMessageModified` | 未查到 | 当作新消息转发（`FNChatManager.swift:494-503`） | 未查到 |
| `onReceiveRESTCustomData`（推送唯一通道） | `im_push_manager.dart:39-45`、`:61-65` | `FNIMPushManager.swift:92-99` | `ChatRoomHelper.kt:287-296` |
| 成员进出、群解散、群资料变更 | 未查到 | 未查到 | 未查到；`addGroupListener` 名字像监听，实际只拉一次群主（`ChatRoomHelper.kt:109-125`） |
| 连接状态 | 回调为空（`im_manager.dart:42-44`） | 调试日志（`FNIMSDKManager.swift:136-153`） | 只写日志（`ChatRoomHelper.kt:45-55`） |

### 2.7 禁言、删消息、角色、发言门槛

全部经业务服务端，客户端不直接调用 IM 的管理能力：

| 能力 | 事实 |
|---|---|
| 禁言 | 接口 `/live/basic/setPermanentBan`（`FNChatApi.swift:121-122`）；调用见 `chat_room_notifier.dart:244-255`、`ChatViewModel.kt:391-393`。进房时取 `muteUntil`（`chat_room_notifier.dart:358`、`ChatViewModel.kt:152`）；实时变化靠 contentType=1 推送。fx-sport 注释称其为"全局实时禁言/解禁"（`chat_room_notifier.dart:89`） |
| 删消息 | 接口 `/live/basic/delete/message`（`FNChatApi.swift:119-120`）；删除后本地立即过滤（`chat_room_notifier.dart:258-275`）；其他人靠 contentType=3 推送和进房时的已删 ID 列表（`chat_room_notifier.dart:345-357`、`FNChatManager.swift:140`、`ChatViewModel.kt:151`） |
| 角色 | 来自业务接口 `live/init` 的 `groupInfoMember`，取值 Owner / Admin / Member / NotMember（`FNChatApi.swift:222-228`）。fx-sport 查自己是否 Admin（`chat_room_notifier.dart:233-241`）；android 判断列表中有 Admin 或 Owner（`ChatViewModel.kt:153-155`）。**两端判断不同** |
| 发言门槛 | 业务配置 `vipLevel`、`rechargeAmount`、`rechargeDays`、`userRechargeAmount`、`minOrderAmount`、`matchChatRoomEnabled`（`FNChatApi.swift:184-190`），客户端判断（`comment_allow_notifier.dart:38-42`、`ChatViewModel.kt:64-72`、`ChatFragment.kt:357-359`） |
| 消息上报 | 发送成功后调 `/live/basic/report/msg`（`im_chat_manager.dart:252-260`、`FNChatManager.swift:348`、`ChatViewModel.kt:229-239`）。ab77 自定义消息固定按晒单类型上报（`FNChatManager.swift:431`） |
| 敏感词 | **依赖腾讯返回 80001**（`im_chat_manager.dart:246-248`、`FNChatManager.swift:361-364`、`ChatRoomHelper.kt:184-185`）。发送前做 NFKC + 去变音符（`chat_room_notifier.dart:581`、`FNChatManager.swift:284-293`）；android 文本是否标准化未查到 |

### 2.8 系统推送

**通道**：登录后加入一个配置下发的全局推送群（群 ID 来自配置：`push_notifier.dart:73`、`ABServerConfig.swift:129`、`GlobalPushManager.kt:48`），监听 `onReceiveRESTCustomData`。fx-sport 的注释记录了服务端使用 REST 接口 `send_group_msg_with_attr`，也记录了此前因为监听错回调收不到推送（`im_push_manager.dart:18-24`）；此前还因没加群而收不到（`push_notifier.dart:18-21`）。

| 项 | fx-sport | ab77 | android |
|---|---|---|---|
| 加群 | 先查是否已加入，再 `joinGroup`（`push_notifier.dart:77-131`） | 同样流程（`FNIMSDKManager.swift:104-124`） | 直接 `joinGroup`（`GlobalPushManager.kt:60`），由 `MainActivity.kt:208` 启动 |
| 要求登录 | 是（`push_notifier.dart:44-50`） | 是（`FNIMSDKManager.swift:22-25`、`:81-91`） | 是（`GlobalPushManager.kt:38-41`） |
| 压制未读 / 角标 | 免打扰 + 清未读 + 清角标（`push_notifier.dart:182-197`），未读一变就再清（`:148-151`） | APNS 回调返回 0（`FNIMPushManager.swift:166-172`） | 未查到 |
| 推送全文写日志 | 写入阿里云日志（`im_push_manager.dart:69-72`） | 写 `HPLog`（`FNIMPushManager.swift:109` 等） | 写 `LogUtils`（`GlobalPushManager.kt:110`） |

**推送结构**：`{contentType, type?, desc?, content?, muteUntil?}`（`GlobalPushModel.kt:5-12`）。

| contentType | 含义 | 负载 | 依据 | 定向方式（客户端可见） |
|---|---|---|---|---|
| 0 | 进球 | `content`：`matchId`、比分、时间等 | `im_push_manager.dart:86-90`、`ABPushMessage.swift:17`、`GlobalPushHandle.kt:41` | 广播，客户端按投注 / 收藏过滤（`goal_push_notifier.dart:152-154`），按 `tag` 去重（`:164-166`） |
| 1 | 系统禁言 | 顶层 `muteUntil`（0 = 解禁），**没有 userId** | `ban_push_model.dart:7`、`:14`；`ABPushMessage.swift:19` | 未查到；负载里没有目标用户，推论：服务端另有定向手段，或该值对所有接收者生效，需查业务服务端 |
| 3 | 删消息 | `content {chatRoomId, messageId, deleteType, deleteBy, timestamp}` | `delete_push_model.dart:25-29`、`ABPushMessage.swift:21` | 广播，客户端比对 `chatRoomId`（`chat_room_notifier.dart:83-86`） |
| 4 | C2C 提现待确认 | `content {accounts, status, titleMap, contentMap, …}` | `ABPushMessage.swift:23`、`:166`；`GlobalPushHandle.kt:44`；`GlobalPushModel.kt` | **全局广播 + 客户端比对账号**（`FNIMPushManager.swift:137-142`，注释"后端是全局广播"；`withdraw_push_notifier.dart:56-60`）。账号列表因此会下发给所有在线客户端 |
| 6、7、8、9 | 主播上线 / 下线 / 开播 / 下播 | `content`（含 `onlineStatus`、`status`） | `im_push_manager.dart:117-121`、`ABPushMessage.swift:25-31`、`GlobalPushHandle.kt:45-48`、`:135-136` | 广播 |

**补收与去重**：客户端**没有** `pushId`、序号或补收逻辑（三端解析代码都不读这类字段）。除进球按 `tag` 去重外未查到其他去重。fx-sport 的提现推送还要求 App 在前台（`withdraw_push_notifier.dart:44-50`）。

**设备离线推送（APNs 等）不走腾讯**：ab77 使用友盟 `UMPush`（`AppDelegate+SDK.swift:42-43`、`:160-205`）；发消息时 `offlinePushInfo` 默认空（`FNChatManager.swift:312`、`:396`）。android 已声明友盟 push 依赖（`android/module-common/build.gradle:123`、`:128`），但 `app`、`module-common` 源码里未见 `PushAgent` 调用，是否启用未确认。fx-sport 的设备推送集成未查到（只在 pubspec 和源码里搜索了 `firebase_messaging`、`umeng`、`jpush`、`UMConfigure`、`FirebaseMessaging`）。

### 2.9 未查到的部分

以下**不能当作事实使用**，评审前需要找业务服务端或腾讯控制台确认：

| # | 未查到 | 已做的搜索 | 影响 |
|---|---|---|---|
| N-1 | 群类型是否就是 AVChatRoom，谁建群 | `AVChatRoom` 三个仓库 0 命中；客户端只见 `joinGroup`。ab77 注释提到"直播群"（`FNChatManager.swift:266`），不构成证明 | "AVChatRoom"的说法来自 PLAN 第 1 节，未被代码证实；不影响协议设计，影响 F-MIG-02 |
| N-2 | 业务服务端如何发礼物、官方消息和各类推送；禁言推送如何定向；主播端在哪里、如何发言 | 业务服务端代码和主播端不在范围内 | F-MIG-02、L-15、L-17、L-20 |
| N-3 | 现网单房在线、全局在线、消息速率的峰值 | 三个仓库里没有 | L-14 容量目标 |
| N-4 | 腾讯侧"进群补发历史"的条数；腾讯第三方回调和敏感词的配置 | 控制台配置不在代码里 | F-ROOM-04 的 50 条是否够用；L-9、L-11 |
| N-5 | 游客（未登录）能否进聊天室 | ab77 的 IM 登录挂在平台登录之后（`FNIMSDKManager.swift:22-25`）；fx-sport、android 进房未见登录判断，但 `genUserSig` 对游客的行为未查到 | L-6 |
| N-6 | 系统消息（type=1）是否真的由 IM 下发 | 三端都有解析代码，ab77、android 又都本地构造 | F-MIG-01 |
| N-7 | android 的 IM SDK 版本，以及 Kotlin、OkHttp 版本和 minSdk | 版本目录在远程 `hp-catalog`（`android/settings.gradle:28-31`），本地没有 | 影响 sdk-kotlin 能否直接引入（L-13） |
| N-8 | 客户端文本长度、发言频率限制；业务 userId 的实际字符集 | 未专门查到；`im_manager.dart:110` 只显示 userId 被转成字符串 | F-ROOM-06、F-ROOM-08 的默认值（L-21）、F-MIG-02 |
| N-9 | PRD-core 1.1 节"合计约 2400 行"的核实 | 未逐一统计 | 无 |

### 2.10 对需求的推论

以下是从上面事实得出的判断，不是事实本身：

1. 全局推送加群是腾讯下的变通做法。天听不需要"推送群"，登录连接后即可收推送，三端的加群、清未读、压角标代码都可以删除。
2. 进球、主播状态是有时效的广播，离线补收反而有害；提现、通知类才需要补收。所以补收应当按推送指定，不能一刀切（L-2）。
3. 提现推送把账号列表广播给所有客户端，迁移时应当改成按用户推送（L-15）。
4. 角色、发言门槛、禁言入口、删除入口都在业务服务端，天听不必在二期建"房间角色"，只提供执行能力。
5. 礼物的钱和订单在业务服务端，天听只需要一条可信的"服务端下发自定义消息"通道。

---

## 3. 范围与分批

### 3.1 功能点

| 编号 | 功能点 | 里程碑 | 优先级 | 涉及 |
|---|---|---|---|---|
| F-PUSH-01 | 下行 `push` 帧 | L1 | P0 | im-server / 协议 / SDK |
| F-PUSH-02 | 推送管理接口 `POST /admin/push` | L1 | P0 | im-server |
| F-PUSH-03 | 按用户推送与离线补收 | L1 | P0 | im-server / SDK |
| F-PUSH-04 | 全员推送 | L1 | P0 | im-server |
| F-PUSH-05 | 按房间推送 | L2 | P0 | im-server |
| F-PUSH-06 | `pushId` 幂等与去重 | L1 | P0 | im-server / SDK |
| F-PUSH-07 | 多端语义 | L1 | P0 | im-server / SDK |
| F-PUSH-08 | SDK `pushes` 流 | L1（sdk-ts）、L3（其他） | P0 | SDK |
| F-PUSH-09 | 推送审计与限速 | L1 | P0 | im-server |
| F-PUSH-10 | 推送可观测 | L1 | P1 | im-server |
| F-PUSH-11 | 推送并发与崩溃恢复 | L1 | P0 | im-server |
| F-PUSH-12 | 通知的回放展示和未查看 | L1（H5）、L3（原生 App） | P0 | H5 / 原生 App |
| F-ROOM-01 | 房间管理接口 | L2 | P0 | im-server |
| F-ROOM-02 | 进出房间 | L2 | P0 | im-server / SDK |
| F-ROOM-03 | 断线重连后自动回房 | L2 | P0 | SDK |
| F-ROOM-04 | 最近消息 | L2 | P0 | im-server |
| F-ROOM-05 | 在线人数 | L2 | P0 | im-server / SDK |
| F-ROOM-06 | 文本消息收发 | L2 | P0 | im-server / SDK |
| F-ROOM-07 | 自定义消息与服务端下发 | L2 | P0（L-10） | im-server / SDK |
| F-ROOM-08 | 限流与下行保护 | L2 | P0 | im-server |
| F-ROOM-09 | 禁言在聊天室生效 | L2 | P0 | im-server / SDK |
| F-ROOM-10 | 删除聊天室消息 | L2 | P0 | im-server / SDK |
| F-ROOM-11 | 内容拦截 | L2 | P0（L-9） | im-server / SDK |
| F-ROOM-12 | SDK 聊天室接口 | L2（sdk-ts）、L3（其他） | P0 | SDK |
| F-ROOM-13 | 聊天室可观测 | L2 | P1 | im-server |
| F-ROOM-14 | H5 聊天室演示页 | L2 | P1 | H5 |
| F-ROOM-15 | 聊天室并发与错误优先级 | L2 | P0 | im-server |
| F-MIG-01 | 能力对照表与接入指南 | L3 | P0 | 文档 |
| F-MIG-02 | 业务服务端切换需求 | L4 | P0 | 业务方 |
| F-MIG-03 | 灰度与回退 | L4 | P0 | 业务方 / 三端 |
| F-MIG-04 | 三端行为不回归 | L4 | P0 | 三端 |

### 3.2 里程碑

| 里程碑 | 内容 | 过关标准 |
|---|---|---|
| **L1 push 协议** | 3.3 节条目冻结（冻结即解除 BE-228、BE-229、CS-044、CS-045、FE-208、FE-353、FE-424、FE-434 的阻塞）；交付物是 im-server、sdk-ts、H5 通知 | 下表中归属 L1 的 AC 全部通过，只在 L1 交付物上验收；行为用例 `vectors/push-sync.json` 在 sdk-ts 跑通。AC-ADM-50 系列和"随 CS-044"的 AC 随 CS-044、CS-045、FE-424 完成时验收，不计入 L1 门槛 |
| **L2 聊天室** | im-server + sdk-ts；单节点；定义 `RoomRouter` 和房间状态接口 | 下表中归属 L2 的 P0 AC 通过；开发前先做扇出原型压测；性能 AC 按 8.1 节判定 |
| **L3 其他 SDK** | sdk-kotlin、sdk-swift，以及 Flutter 接入（L-12）；集群实现随 B6 | 各 SDK 跑通 `frames.json` 和行为用例 `push-sync.json`、`room-session.json`（`frames.json` 只验帧格式，验不到行为）；在业务工程的空分支上能引入并编译通过（L-13） |
| **L4 迁移** | 一端试点 → 三端灰度 → 下线腾讯（L-11、L-13） | F-MIG 的 AC 通过；执行方见 7 节；下线门槛由 Frank 确认 |

**AC 里程碑归属**（未列出的按所在功能点的里程碑）：

| AC | 归属 | 说明 |
|---|---|---|
| F-PUSH-01～04、06、07、09～12 的 AC | L1 | 帧中 `target=room` 的格式在 L1 冻结，接口要到 L2 才可用。下面单独列出的除外 |
| AC-PUSH-01-2 | L1 | `room` 帧只校验 Schema |
| AC-PUSH-02-8 | L1 | `target=room` 在房间开关关闭时返回错误 |
| AC-PUSH-06-17 | L2 | 依赖房间；AC-PUSH-06-8 留在 L1 |
| AC-PUSH-02-7、F-PUSH-05 的 AC | L2 | 依赖房间 |
| AC-PUSH-07-2 | L1 | web 用 sdk-ts，"android"用 `platform=android` 的协议客户端模拟；真实 sdk-kotlin、sdk-swift 由 L3 的行为用例覆盖 |
| F-PUSH-08 的 AC | L1（sdk-ts） | 其他 SDK 在 L3 跑同一批行为用例 |
| AC-PUSH-03-16、AC-PUSH-03-34、AC-PUSH-08-7 | L3 | 只有原生 SDK 才持久化游标，sdk-ts 没有持久化，这几条在 L1 无法验证 |
| AC-PUSH-07-3、AC-PUSH-07-6 的"有持久化"分支，AC-PUSH-08-2 的 Kotlin 分支 | L3 | 同一条 AC 的其余部分归 L1 |
| AC-PUSH-08-9、AC-PUSH-08-10 | L3 | 只针对 Swift、Kotlin。`push-sync.json` 的 `persistence=true` 用例同样归 L3，sdk-ts 跳过 |
| AC-PUSH-06-6、AC-PUSH-06-13、AC-PUSH-06-15 | 随 CS-044 | 需要 console-server，不计入 L1 门槛 |
| AC-PUSH-04-2 | L1（开发机结果）；发布门槛（目标机结果） | L1 只要 [P-开发机] 结果；[P-目标机] 结果归发布门槛，等 P-12 |
| AC-PUSH-11-5 | L1（SQLite 断言）；Postgres 版随 im-server 的 Postgres 实现 | 现在 im-server 只有 SQLite（`store/migrations/` 下只有 `sqlite`） |
| AC-PUSH-12-9 | L3 | 原生 App 的页面 |
| AC-ROOM-12-3 | L3 | 原生 SDK 才有本地库；L2 用 AC-ROOM-12-4 断言 store 不写房间消息 |

依赖：L1 依赖第二批协议兼容规则（PRD-core 第 8 节"三套 SDK 必须忽略未知"）。L2 依赖 L1 的按房间目标定义，以及用户级禁言（F-ADM-40）。L1 与 L2 分开冻结（L-16，已定）。

### 3.3 L1 冻结条目

**版本号**：复核通过后出 `PRD-live-v1.0.md`，文首写明"只冻结本节列出的条目，其余仍是草稿"；聊天室、迁移定稿后升 v1.1、v1.2。冻结后的修改走 `docs/process/README.md` 的变更流程。

| 章节 | 冻结范围 | 不冻结 |
|---|---|---|
| 第 4 节 | 全部，包括 `target=room` 的帧格式、三端 `pushes` 签名（4.8 节）、游标存储（4.9 节） | — |
| 第 6 节 | F-PUSH-01～04、06～12 | F-PUSH-05 和 AC-PUSH-02-7（依赖第 5 节未冻结的 AC-ROOM-08-2） |
| 第 8 节 | 与推送有关的兼容规则、8.1 节 `AC-PUSH-*` 行、8.2 节标 L1 的设施、8.3 节 `push-sync.json` | 其余 |
| 第 9 节 | 4094；4001、5040 的 `action` | 4041、4047、4138、4510。**4.4 节和 F-PUSH-02 规则 7 引用的 4047 随 L2 生效**：L1 里 `target=room` 由开关挡在 4000（规则 6） |
| 第 10 节 | 标 L1 的行（`L1 / L2` 的行已拆开） | 标 L2 的行 |
| 第 11 节 | "L1 冻结时"一组 | "L2 及以后"一组 |
| 第 13 节 | 13.1 节 | 13.2 节 |

**`push-sync.json` 已写好**（`protocol/vectors/`，71 个用例，已用两份实现跑通），和规则一起冻结，不是只冻结格式：它是三端一致的唯一判据，L-16 已列入冻结范围。写用例不需要实现，还能在冻结前发现规则的矛盾。sdk-ts 跑通是 L1 过关标准。

---

## 4. push 协议（按 3.3 节冻结）

> 本节回答 PRD-admin 2.2 节规则 4 的三个问题。纯技术规则已按评审定下；涉及产品的部分标（L-n），都是 Frank 已定的结论（13.1 节）。冻结后才生效。

### 4.1 语义总表

| 目标 | 进收件箱 | `pushSeq` | 离线补收 | 多端 | scope |
|---|---|---|---|---|---|
| `user`，`ttlSeconds` > 0 或不传（不传时为 7 天） | 是 | 按用户递增 | 有效期内可补收 | 每个连接各一次；每台设备各自补收 | `push.user` |
| `user`，`ttlSeconds` = 0 | 否 | 无 | 无 | 当前在线的每个连接各一次 | `push.user` |
| `all` | 否 | 无 | 无（L-1） | 当前在线的每个连接各一次 | `push.all` |
| `room` | 否 | 无 | 无 | 当前在该房的每个连接各一次 | `push.room` |

### 4.2 帧与接口

下行（在现有 `PushFrame` 上扩展；现有帧的 `additionalProperties: false` 需要显式加字段，`pushId` 改为必填）：

```jsonc
{ "cmd": "push", "data": {
    "pushId": "p1",            // 必填，1～64 位 [A-Za-z0-9_-]，调用方生成，全局唯一（建议 UUID）
    "type": "notice",          // 必填，业务类型，服务端不解释
    "data": { "text": "…" },   // 必填，业务负载，序列化后 ≤ 4KB
    "ts": 1730000000000,       // 必填，首次受理时间；pending 重播时沿用，不取重播时刻（4.4 节）
    "target": "user",          // 必填：user | all | room。room 的帧格式在 L1 冻结，接口 L2 才开放
    "pushSeq": 42,             // 只有进收件箱的推送才有
    "roomId": "match-123"      // 只有 target=room 才有
} }
```

上行（新增命令）：

```jsonc
// 请求：二选一。fromLatest 只取最新序号建立游标，不回放
// 两个都不传、同时传、fromLatest:false 都是 ack 错误 4000。SDK 固定用 limit=100
{ "id": "r9", "cmd": "pushSync", "data": { "afterSeq": 41, "limit": 50 } }
{ "id": "r9", "cmd": "pushSync", "data": { "fromLatest": true } }
// 应答：按 pushSeq 升序，只含未过期的。pushes、hasMore、latestSeq 在同一读事务内取得
{ "id": "r9", "cmd": "ack", "data": { "pushes": [ /* 同 push.data */ ], "hasMore": false, "latestSeq": 42 } }
```

管理接口（scope、错误码沿用 F-ADM-50 规则 8，增加 `roomId`、`ttlSeconds`）：

```jsonc
POST /admin/push
{ "target": "user|all|room", "userIds": ["u1"], "roomId": "match-123",
  "type": "notice", "data": { }, "pushId": "p1", "ttlSeconds": 604800 }
// ttlSeconds 可选：target=user 不传为 604800，传 0 只投在线；其他 target 不传为 0，传非 0 报错
// 应答：accepted 只在 target=user 时出现，等于去重后的目标用户数；不返回在线人数
{ "pushId": "p1", "created": true, "accepted": 1 }
```

### 4.3 问题一：离线补收

| 环节 | 规则 |
|---|---|
| 写入 | 收件箱 `push_inbox(user_id, push_seq, push_id, type, data, created_at, expires_at)`，`(user_id, push_seq)` 与 `(user_id, push_id)` 都唯一，`expires_at`、`push_id` 建索引（`push_id` 用于 4.4 节的幂等清理）。一次提交的所有用户在**同一事务**写入；Postgres 下按 userId 字节序加锁（F-PUSH-11） |
| 序号 | 计数器表 `push_seq(user_id, max_seq)`，每个用户独立递增，**不随清理删除**，序号不回退；`latestSeq` 从这张表读。不复用 `conv_seq`（它带会话语义）。分配时连续；但读取时因为条级 TTL 和收件箱上限，可能看到空洞。只有进收件箱的推送才占序号 |
| 投递 | 事务提交后向在线连接实时下发。提交后、下发前崩溃时，由下一次补拉兜底（4.6 节已接受风险） |
| 补拉时机 | **有订阅者时**（C12），SDK 每次连接成功（`auth` 之后）调一次 `pushSync`，**不等 `convs`、`socialSync`、`joinRoom`，并行且互不阻塞，任一失败不影响其他**（AC-PUSH-03-24）。现状 `client.ts` 的连接流程是先等 `convs`，要改。`hasMore=true` 时按下面的规则继续；实时帧发现缺口时也按下面的规则补拉 |
| 到期 | **读取时按 `expires_at` 过滤**（`now ≥ expires_at` 不返回），不依赖定时清理；BE-220 的定时任务只负责回收空间。`afterSeq` 早于保留期不报错，返回还在有效期内的部分 |
| 被封禁 | 封禁期间连不上，收不到；解封后在有效期内可补收 |

**只有一个游标**（三套 SDK 必须一致）：`cursor` 是**已交付的最大连续序号**，持久化（4.9 节），也是下一次 `pushSync` 的 `afterSeq`。"一页交付完 → 游标前进 → 才取下一页"，所以翻页不需要第二个变量。

**带 `pushSeq` 的帧都能从收件箱补回，所以 SDK 不需要替它们保存什么**：没有订阅者时直接丢弃，有订阅者时暂存区满了也可以清空。这是这套规则最简单的地方。只有不带 `pushSeq` 的帧补不回（C4）。

"见过"的 `pushId`：**交付前查，交付时记**，收到时不记。

**规则**（由共享行为用例 `vectors/push-sync.json` 验证，输入是事件序列，期望是发出的请求、交付序列和游标）：

| # | 情况 | 处理 |
|---|---|---|
| C1 | 有订阅者，实时帧 `pushSeq = 游标 + 1` | 没见过就交付；游标前进；然后 C11 |
| C2 | 有订阅者，实时帧 `pushSeq > 游标 + 1` | 进暂存区（上限 200 条，未校准），启动 500ms 定时器（已在计时就不重启；**有请求进行中或正在退避时不启动**，由它们结束后按 C11 判断；与消息的缺口等待一致，协议 README 第 3 节规则 7）。到期时缺口还在就 `pushSync {afterSeq: 游标}`。**暂存区满时清空，只留当前这一帧**，清掉的都能补回 |
| C3 | 有订阅者，实时帧 `pushSeq ≤ 游标` | **是否展示只看 `pushId`**：见过就丢弃，没见过就交付，不动游标 |
| C4 | 没有 `pushSeq` 的帧（`all`、`room`、`ttlSeconds=0`） | 按 `pushId` 去重后交付，**不动游标**。**没有订阅者时进一个有界队列（上限 500，未校准），溢出丢最旧的；这些帧补不回，是至多一次语义**（4.6 节风险 ⑥）。入队时不去重，交付时查。首个订阅者到调度点时按到达顺序先交付，再开始补拉 |
| C5 | `pushSync` 单飞 | 同一时间只有一个请求；进行中发现新缺口不并发，结束后按 C11 再判断。ack 按请求 id 匹配，没有进行中的请求或 id 不匹配的 ack 一律忽略 |
| C6 | 应答 `hasMore=true`（服务端保证此时 `pushes` 非空） | 本页与暂存帧中 `pushSeq ≤ 本页最大值` 的帧合并，按序交付，见过的跳过；比本页大的帧继续暂存。交付完后游标 = `max(游标, 本页最大 pushSeq)`（页内的序号空洞是服务端确认已过期的），**先按 C11 接上暂存帧（不启动定时器），再取下一页**，`afterSeq` 是接上之后的游标。`hasMore=true` 但 `pushes` 为空违反服务端保证，按 C9 的失败处理，避免原地死循环 |
| C7 | 应答 `hasMore=false`（且不符合 C8） | 同 C6 合并 `pushSeq ≤ latestSeq` 的帧。交付完后游标 = `max(游标, latestSeq)`，结果为空时也一样，尾部过期不会让游标停住。然后 C11，再发 `pushSynced`（4.8 节） |
| C8 | `latestSeq < afterSeq`（服务端从备份恢复、切环境）。**优先于 C7** | 不能把游标设成 `latestSeq`：恢复后新写入的序号都不大于它，会永远补不到。处理：**游标重置为 0，清空暂存（恢复前的大序号帧会让 C11 一直判定有缺口，它们都能从收件箱补回），取消 500ms 和 5 秒定时器，本轮重新开始**，落盘，发 `pushSync {afterSeq:0}`，不走 C11。之后走普通的 C6、C7，靠 `pushId` 去重（2000 条大于收件箱上限 1000），最后游标自然等于 `latestSeq`。长过旧游标的情况客户端察觉不了，见 4.6 节风险 ④ |
| C9 | `pushSync` 失败 | **4001**（服务端未开启）：不抛错、不提示升级、不退避，**这次连接内不再发 `pushSync`**（缺口定时器到期、退订再订阅都不发），实时帧照常按 C1～C4 处理，下次连接成功再发。**其他错误码（4000、4290、5000 等）和请求超时（SDK 现有的请求超时）都算失败**：按 1、2、4、8、16、30、30…秒退避重试（`defaults.backoffMs`），重试的 `afterSeq` 取当时的游标，成功后退避清零，重试不算新一轮（4.8 节）。**5 秒兜底**：失败状态下暂存区非空时启动**一个** 5 秒定时器（未校准；已在计时就不重启；首次失败时暂存已有帧就从那时起算，失败状态下暂存从空变非空就从那时起算），恰好到期就把**当时暂存区的全部帧**按 `pushSeq` 升序交付，**游标不前进**，之后的补拉靠 `pushId` 去重。任何一次成功应答都结束失败状态并取消这个定时器。没有失败时，暂存帧只等补拉，不抢先。**连接断开时**：进行中的请求算失败，取消 500ms、5 秒和退避定时器，单飞和退避状态清零，**暂存保留**，连接成功后立即补拉，按 C6、C7 合并 |
| C10 | 没有游标（首次、无持久化） | 默认 `afterSeq: 0`；启动参数 `pushStart=latest` 时用 `fromLatest` 建立游标、不回放（灰度用，F-MIG-03 规则 3），应答前到达的帧先暂存（用同一个暂存区和上限，不启动 500ms 定时器），应答后按 C11 处理。`fromLatest` 的应答**忽略 `pushes` 和 `hasMore`**（AC-PUSH-03-22 保证为空和 false），游标 = `latestSeq`。失败时按 C9 退避，重试仍是 `fromLatest`。已有持久化游标时不走这条 |
| C11 | 每个分支结束后 | 按序取出暂存帧：`pushSeq ≤ 游标` 的按 C3，等于 `游标 + 1` 的按 C1，能接上就继续；还有缺口的启动 500ms 定时器（有请求进行中或正在退避时不启动，由它们结束后再判断）；暂存空了就取消定时器 |
| C12 | 没有订阅者（4.8 节"算不算订阅者"） | **不发 `pushSync`，带 `pushSeq` 的实时帧直接丢弃**，游标不动。首个订阅者注册后，到**下一个调度点**开始：先交付 C4 的队列，已连接就发 `pushSync {afterSeq: 游标}`。**订阅者全部退出后同样停止**：清暂存，取消 500ms、5 秒和退避定时器，退避清零，迟到的 ack 直接丢弃（不交付、不动游标）。在进行中的请求返回前又有了订阅者时，不发第二个请求，应答按正常处理。**回调内退订最后一个订阅者**：当前这条算已交付并记入见过，本页其余的立刻停止交付，**游标不前进**（包括实时帧的 C1），不取下一页，不发 `pushSynced`；再订阅后从原游标补拉，已交付的靠见过跳过。只退订其中一个时，其余订阅者照常 |

**回放总量**：一次连接最多拉到收件箱全部内容，受单用户上限 1000 条（未校准，F-PUSH-09）约束；SDK 固定 `limit=100`，最多 10 页（翻页期间又有新推送时可能多一页）。

### 4.4 问题二：`pushId` 去重

两层，解决不同的问题：

| 层 | 解决 | 规则 |
|---|---|---|
| 服务端幂等 | 调用方重试（包括 console-server 的 5040 结果未知） | **`pushId` 全局唯一**。理由：收件箱唯一键和 SDK 去重都只看 `pushId`，帧里也没有调用方；全局唯一不需要改帧、不需要合成 ID，调用方用 UUID 即可避免撞号。同一调用方同内容：返回首次结果，`created=false`，按下面的状态决定是否再投递。调用方不同或内容不同：409 / 4094，不投递。v0.1 里"两个调用方同号各自投递"的 AC 已删除 |
| SDK 去重 | 实时帧和 `pushSync` 重叠、重连重发 | 按 `pushId` 去重，保留最近 2000 个（未校准，**按交付先后淘汰，命中不刷新**，包括没有 `pushSeq` 的帧），**按 userId 隔离**，退出或换号时清除内存部分；同一批推送发给 u1、u2 时，同一实例上两个账号各交付一次（4.8 节）。`pushId` 交付前查、交付时记（4.3 节）。原生端随游标持久化（4.9 节）。**不跨设备去重** |

**幂等表** `push_idempotency(push_id 主键, caller, content_hash, target, accepted, state, first_ts, created_at)`：

| 环节 | 规则 |
|---|---|
| 校验失败 | 4000、4131、4042、4047（L2）、4030、4290 都**不占 `pushId`**，修正后可以用同号再提交 |
| 内容比对 | `content_hash` = SHA-256（`target`、去重排序后的 `userIds`、`roomId`、`type`、`data` 解析后按键排序重新序列化的字节、生效的 `ttlSeconds`）。序列化**保留数字原文**（Go 用 `json.Number`），否则超过 2^53 的两个不同整数会被判成同内容 |
| `target=user` 且进收件箱 | 幂等记录、`push_seq`、`push_inbox` 在同一事务写入，`state=done`。投递靠实时帧 + 补拉，重试不再投递 |
| `all`、`room`、`ttlSeconds=0` | 先写记录 `state=pending`，再广播，广播发出后改 `done`。重试时：`done` 不投递；`pending` **重新广播**（至少一次，靠 SDK 去重），完成后改 `done`，都返回 `created=false`。重播帧的 `ts` 沿用 `first_ts`：这条通知本来就是那时发的，对重播期间新启动的实例算旧推送（`replayed=true`）。保留期 8 天内都允许重播，时效由调用方判断（console 的规则见 4.7 节）。调用方不重试时，崩溃会造成这条未投递（4.6 节已接受风险） |
| 重放 | 幂等检查先于限速，重放不受 F-PUSH-09 限速约束；console-server 的 5 分钟间隔也不约束同号重放（PRD-admin 变更，第 11 节） |
| 保留 | 幂等记录保留 8 天，**晚于收件箱到期**。清理任务先清收件箱，幂等记录**只有在没有对应收件箱行时才删**，清理落后或被关闭时不会撞 `(user_id, push_id)` 唯一键。`pushId` 过期后也**不应复用**：服务端会接受（`created=true`），但设备持久化的去重集合可能把它当重复丢掉 |

### 4.5 问题三：多端语义

1. 同一用户的每个在线连接各收一次，包括同一台电脑的多个标签页。
2. 每台设备独立补收：A 设备在线收到，不影响离线的 B 设备上线后补收（L-3）。
3. 服务端不记录"谁收到了"，没有回执，也不跨端同步已读。"未查看条数"的规则见 F-PUSH-12，依据是本机的 `lastSeenSeq`，**可以跨会话**（L-4，已定），不再是 F-UI-44 规则 3 的"本次登录期间"（变更见第 11 节）。
4. `replayed` 是确定规则，只看时间，不看通道：`T0` 是**本 SDK 实例上该 userId 第一次 `auth` 成功时 ack 的 `serverTime`**，同一账号重连不变，退出或换号时清除；`replayed = (ts < T0)`。`ts` 和 `T0` 都是服务端时钟，没有客户端时钟偏差。所以网络闪断期间到达的推送、触发缺口补拉的实时帧、在线时错过的推送都是 `false`，App 重启或 H5 刷新后补到的旧推送是 `true`。展示规则见 F-PUSH-12（L-4）。

### 4.6 已考虑的备选与已接受的风险

| 方案 | 不选的原因 |
|---|---|
| 服务端按用户记"已投递"标记 | 第一台设备收到后，第二台设备就补不到，违反多端语义 |
| 服务端按设备记录游标 | 需要设备身份和回执，复杂度高；现有消息模型也是客户端游标（PLAN 2.1 节） |
| 把推送当系统账号的会话消息 | 复用 `seq` 和 `sync`，但会污染会话列表和未读，正是现有 App 要压制的问题（`push_notifier.dart:182-197`） |
| 全员推送写扩散到收件箱 | 单次写入量等于用户总数，代价高；需要时走 L-1 的读扩散备选 |

已接受的风险：① 全员和按房间推送在连接断开的短暂窗口内会丢（L-1）。② 全员和按房间推送是**至少一次语义下的尽力投递**：广播中途崩溃且调用方不重试时，部分用户收不到；调用方重试时重新广播，SDK 按 `pushId` 去重（4.4 节）。③ 单节点上，提交后、下发前进程崩溃时所有连接都会断开，重连即补拉，不存在长期漏收。**集群下这个风险才真实存在**（节点 A 崩溃或消息总线丢帧，节点 B 上一直在线的用户可能几天补不到），所以"低频定时 `pushSync`，或 pong 带 `latestSeq`"是 B6 的前提，届时补 AC。④ **从备份恢复后序号长过设备游标时，客户端察觉不了**：恢复到 50 后又长到 110，设备游标是 100，`latestSeq ≥ afterSeq`，C8 不触发，51～100 这些新推送这台设备补不到（同序号的内容也可能已经不同）。备份恢复本身少见（RPO 1 小时，`ARCH-v1.md:122`），要彻底解决需要在 ack 带恢复纪元（epoch），代价是协议多一个概念，首版不做。恢复时运维可以提前告知，受影响的推送在业务上重发。 ⑤ **恢复后回放的未查看漏计**：`lastSeenSeq=100` 时服务端恢复，回放的新推送 51～55 因为不大于 100 不计未查看，之后 `pushSynced {55}` 才回退到 55。它们仍进列表，只是少了提醒；恢复本身少见，不值得为此改计数规则。⑥ **无 `pushSeq` 的帧在没有订阅者时是至多一次**：队列溢出就丢（C4），它们本来也不补收（L-1）。⑦ **`pushStart=latest` 从连接到首个订阅之间的推送不补收**：`fromLatest` 也遵守 C12，游标建立在首次订阅时。这个参数只用于灰度首次启动，那段时间腾讯通道还在。⑧ **多标签页的序号回退竞态**：A 标签页已把 `lastSeenSeq` 写成 50，B 标签页一轮补拉的快照是 49，`pushSynced {49}` 会写回 49，刷新后 50 重复提醒一次。只影响未查看数字，不重复交付，不值得为此给回退加条件。

### 4.7 F-ADM-50 草案 AC 对齐

AC 的预期大多不用改，但下面几处要走 PRD-admin 变更（清单在第 11 节，本文不改）：

| 草案 AC | 对齐结果 | 对应 |
|---|---|---|
| AC-ADM-50-1 在线 3 秒内收到 | 保留。帧按 4.2 节，补 `pushId`、`ts`、`target` | AC-PUSH-01-1 |
| AC-ADM-50-2 第 6 天补收、8 天后收不到 | 保留，默认 `ttlSeconds` = 7 天（L-2）。"收到"指 `pushSync` 返回；到期按读取时过滤，不等定时清理 | AC-PUSH-03-1、03-2 |
| AC-ADM-50-3 全员：离线上线后收不到 | 保留（L-1）。全员不传 `ttlSeconds` 即可，默认 0，不会 400 | AC-PUSH-04-1、AC-PUSH-02-12 |
| AC-ADM-50-4 模板 Schema、4KB | 保留。Schema 校验在 console-server；im-server 另校验 4KB | AC-PUSH-02-2 |
| AC-ADM-50-5 全员间隔 5 分钟 | 保留，**只约束 console-server**；im-server 对业务调用方另有限速（F-PUSH-09、L-17） | AC-PUSH-09-2 |
| AC-ADM-50-6 ghost、1001 | 保留 | AC-PUSH-02-3、02-4 |
| AC-ADM-50-7 不能自批 | 不受影响 | — |
| AC-ADM-50-8 发送记录与审计 | 不受影响；`biz.push_records.im_push_id` 就是本协议的 `pushId` | AC-PUSH-09-1 |
| 规则 3"`pushId` 相同由 SDK 去重" | 明确为**同一 SDK 实例内**，不跨设备 | AC-PUSH-07-1 |
| 规则 8 | 增加 `target=room` + `roomId`、`ttlSeconds` | AC-PUSH-05-1 |
| 5040"不要重复提交" | **要改**：只有下面的对账结论才用同一 `pushId` 重放，其他写操作的 5040 规定不变；重放不受 5 分钟间隔约束 | AC-PUSH-06-6、06-10、06-13、06-15 |

**5040 对账与重放**（console-server，CS-014 的对账任务按 `requestId` 查 im-server 审计）：

| im-server 审计 | 含义 | console 的结论 | 重放 |
|---|---|---|---|
| result 为 2xx | 已受理 | 补写 `success` | **不重放** |
| result 为 4xx、5xx | 明确失败 | 补写 `failed` | 不重放，运营修正后新发 |
| 只有 intent，没有 result | im-server 处理中崩溃，可能是 `pending` | `unknown` | 用同一 `pushId` 重放 |
| 没有 intent | 请求没到 im-server（CS-014 的 `notExecuted`） | `notExecuted` | 用同一 `pushId` 重放 |

- **发起方**：对账任务自动重放，只重放一次，审计 `push.replay`（含 `pushId`、原 `requestId`）。运营不需要操作。
- **全员推送不重走双人复核**：批准的就是这份内容，`pushId` 和内容哈希都没变。重放不写入"最近一次全员发送时间"，也不受 5 分钟间隔约束。
- **过期处理**：距首次发送超过 1 小时（未校准）不再自动重放，发送记录标为 `unknown`，由运营判断是否新发。通知有时效，晚到比不到更容易误导。幂等记录的 8 天保留期只是上限。
- 重放结果按重放应答更新发送记录。

### 4.8 SDK 交付契约（L1 冻结）

| 项 | 规则 |
|---|---|
| 语义 | 带 `pushSeq` 的推送**至少一次**：业务要按 `pushId` 幂等处理（进程在回调后、持久化前崩溃时会重复一次）。没有 `pushSeq` 的推送**至多一次**（C4） |
| 接口形态 | 见下面的签名。Flutter 随 L-12。多订阅者每个都收到一次。**注册后到下一个调度点才交付积压，同一调度点注册的都能收到**（TS 是微任务，其他端是等价的调度）；已经有订阅者之后再注册的只收之后的。**调度点先于任何网络事件和定时器**：注册之后到达的帧，先过调度点，再按有订阅者处理。TS 的微任务天然如此，其他端要把注册和帧处理放进同一条串行队列 |
| 暂存和队列 | **带 `pushSeq` 的帧没有交付队列**：有订阅者时按 C1～C3 处理，暂存区上限 200（未校准），满时清空；没有订阅者时丢弃（C12）。**没有 `pushSeq` 的帧**没有订阅者时进上限 500（未校准）的队列，溢出丢最旧（C4）。现有三端的默认行为都不是这样（`emitter.ts:18-26`、`Client.swift:215-236`、`IMClient.kt:178` 的 DROP_OLDEST），要专门实现 |
| 游标前进时机 | **交付给订阅者之后**才前进并持久化。"交付"指回调已调用（TS、Swift `onPush`）、已发给收集端（Kotlin）或已写入流（Swift `pushes`：`AsyncStream` 的 continuation 已 `yield`，流创建后没有迭代也算；所以只有迭代中的流才算订阅者） |
| 业务崩溃或丢弃 | 不在 SDK 责任内，回调返回即算交付 |
| 账号隔离 | `T0`、去重集合、无 `pushSeq` 队列、暂存帧、定时器都按 userId 隔离，退出或换号时清除，不把 u1 的帧交给 u2（三套 SDK 都有同实例换号的分支：`client.ts:409`、`IMClient.kt:845-849`、`Client.swift:466`）。**游标**：有持久化时按 userId 分键保留；**没有持久化时换账号清零**，所以同实例 u1→u2→u1 时 u1 从 `afterSeq:0` 重新回放，`T0` 也是新的，靠 `replayed` 和 `lastSeenSeq` 处理（AC-PUSH-07-6）。订阅者属于实例，换号后继续有效 |
| 一轮补拉结束 | `hasMore=false` 且 C11 处理完后发事件 `pushSynced {latestSeq: max(latestSeq, 本轮已交付的最大 pushSeq)}`，F-PUSH-12 用它建立基线和判断序号回退。"本轮"从不属于翻页、也不属于退避重试的那次 `pushSync` 开始，包括这期间交付的实时帧；C8 时重新开始，否则恢复后这个值一直是旧值。订阅是可选的，不影响交付。没有订阅者时不补拉，也就不发 |

**三端签名**（L1 冻结；sdk-kotlin、sdk-swift 在 L3 实现）：

| 项 | TS | Kotlin | Swift |
|---|---|---|---|
| 启动参数 | `IMClientOptions.pushStart?: 'beginning' \| 'latest'` | `IMClientOptions(pushStart = PushStart.BEGINNING)` | `IMClientOptions.pushStart: PushStart = .beginning` |
| 订阅 | `on('push', (p: Push) => void): () => void` | `val pushes: Flow<Push>` | `onPush(_ handler: @MainActor @escaping (Push) -> Void) -> IMSubscription`（类型沿用 `Client.swift:104`、`:215`）；`pushes: AsyncStream<Push>` |
| 算不算订阅者 | 只算 `'push'` 的监听器 | 只算 `pushes` 的收集者（`subscriptionCount`） | 只算 `onPush` 和正在迭代的 `pushes`；通用的 `on`、`events()` **不算**，否则只有界面状态监听器时推送会被当成已交付 |
| 一轮补拉结束 | `on('pushSynced', (e: { latestSeq: number }) => void)` | `val pushSynced: Flow<Long>` | `onPushSynced(_:)` |

Kotlin：`onFrame` 是同步函数（`IMClient.kt:240`、`:942`），不能在里面挂起。交付由一条单独的协程按序发给收集端，不用 `emit` 挂起等订阅者，也不用 DROP_OLDEST；页交付完才前进游标、取下一页。

类型 `Push = { pushId, type, data, ts, target, roomId?, pushSeq?, replayed }`。

### 4.9 游标存储

现有两套原生 SDK 的本地库是可丢的缓存：退出、换号、被踢、库损坏、Schema 升级都会清（`LocalDatabase.kt:69-75`、`:157-160`，`IMClient.kt:1175-1178`，`Client.swift:810-814`）。游标放进去就会回到 0，保留期内的推送全部回放，而 token 默认 7 天，这就是常态。所以：

| 项 | 规则 |
|---|---|
| 位置 | 独立的键值文件（Kotlin、Swift）或 `shared_preferences`（Flutter，L-12），不进 `TABLES`，不随库重建和 `wipeAll` 删除 |
| 内容 | 按 userId 分键，只存游标数字和最近 2000 个 `pushId`（未校准，包括没有 `pushSeq` 的帧），不含通知内容，不受 FE-131 的清除要求约束 |
| 什么时候回放 | 只有卸载、清应用数据、换设备 |
| 没有持久化能力（H5、Kotlin 没有注入 `PushCursorStore` 的配置） | 游标只在内存，每个新实例从 `afterSeq: 0` 开始，换账号时清零；H5 刷新就会回放，展示见 F-PUSH-12。Kotlin 没有注入时行为和 H5 完全相同，不报错（AC-PUSH-03-34） |
| 换号 | 各用各的键（AC-PUSH-07-3） |
| 落盘频率 | **按批**：**一个事件（一个 ack 或一次收到的实时帧）处理完落盘一次**，不逐条重写 2000 个 `pushId`。落盘在交付之后，进程在中间退出时最多重复一批，符合至少一次（AC-PUSH-08-7）。C8 把游标重置为 0 时也落盘，中途退出后从头重拉，靠见过去重 |
| 存储接口 | Kotlin 核心模块保持纯 JVM，注入 `PushCursorStore` 接口（`load(userId)`、`save(userId, cursor, recentIds)`），Android 模块提供文件实现；没有注入时按"没有持久化能力"处理。Swift 直接用 Application Support 下的文件 |

---

## 5. 功能需求：聊天室

> 【后端】指 im-server；【SDK】指 sdk-ts（其他 SDK 跑同一批行为用例）；未标注的为端到端。负向断言沿用 PRD-core 6.1 节"2 秒内收不到任何帧"，**房间用例排除 `roomOnline` 和 pong**。[IM 钟] 指测试时钟，它只有 `Now`、`Freeze`、`Unfreeze`，没有定时器，所以依赖时间窗口的用例按 8.2 节的测试接口驱动。

#### F-ROOM-01 房间管理接口（L2，P0）

**规则**
1. 房间显式创建（L-8）：`POST /admin/rooms {roomId, name?}`，scope `room.write`。`roomId` 格式与 groupId 相同；已存在且未关闭时幂等返回。
2. `GET /admin/rooms/{id}` 返回 `{roomId, name, status, online, createdAt}`，scope `room.read`。
3. 关闭：`DELETE /admin/rooms/{id}`，scope `room.write`。全部在房连接收到 `roomClosed {roomId, reason}`，之后不能进房。`reason` 枚举：`closed`（管理接口关闭）、`idle`（自动回收）；SDK 把不认识的值原样交给业务。关闭后 `roomId` 可以再次创建（比赛重开、主播再开播）。重复 `DELETE` 返回 200。
4. 房间元数据持久化；成员和消息不持久化（PLAN 2.1 节，L-5）。启动时把未关闭的房间加载进内存，进房不查库，避免重连风暴压住唯一的数据库连接。
5. 重复创建：未关闭时幂等返回原房间，`name` 不同也不更新。已关闭房间 `GET` 返回 200、`status=closed`；不存在返回 404 / 4047。不提供列表接口（第 10 节）。
6. 自动回收（L-8）：房内没有连接超过 `IM_ROOM_IDLE_TTL`（默认 24 小时，未校准）时自动关闭，防止业务方忘记关房。

**验收标准**
- AC-ROOM-01-1【后端】：输入 不存在的 `match-1`；操作 创建两次；预期 两次都成功，只有一个房间；错误码 —。
- AC-ROOM-01-2【后端】：输入 非法 `roomId`（含空格）；操作 创建；预期 400；错误码 4004。
- AC-ROOM-01-3：输入 u1、u2 在 `match-1`；操作 关闭房间；预期 两人都收到 `roomClosed`，`reason=closed`；u1 再次 `joinRoom` 失败；错误码 4047。
- AC-ROOM-01-4【后端】：输入 已关闭的 `match-1`；操作 再次创建，u1 进房；预期 成功，`recent` 为空；错误码 —。
- AC-ROOM-01-5【后端】：输入 只有 `room.read` 的调用方；操作 创建房间；预期 403；错误码 4030。
- AC-ROOM-01-6【后端】：输入 `match-1` 有 2 人在线；操作 `GET`；预期 返回 `roomId`、`name`、`status=open`、`online=2`、`createdAt`；错误码 —。
- AC-ROOM-01-7【后端】：输入 已关闭的 `match-1`；操作 再次 `DELETE`，然后 `GET`；预期 都是 200，`status=closed`；错误码 —。
- AC-ROOM-01-8【后端】[IM 钟]：输入 空房，`IM_ROOM_IDLE_TTL=1h`；操作 推进 2 小时并触发回收任务（8.2 节）；预期 房间关闭，`reason=idle`；错误码 —。
- AC-ROOM-01-9【后端】：输入 有房间的服务端；操作 重启后 u1 进房；预期 进房成功，未关闭的房间仍存在；错误码 —。

#### F-ROOM-02 进出房间（L2，P0）

**规则**
1. `joinRoom {roomId}` 的 ack 为 `{online, recent, deletedMsgIds}`（`online`、`recent` 与 `frames.json:26` 一致，`deletedMsgIds` 见 F-ROOM-10）。服务端在房间锁内同时完成"取快照、订阅、ack 入队"，所以 ack 之前不会有该房的实时帧。
2. **一个连接同一时间只在一个房间**：已在其他房间时，服务端先退旧房再进新房，不报错。依据：三端都是一次一个业务房（2.3 节）。
3. 重复进入同一房间幂等，重新返回当前的 `online` 和 `recent`。
4. `leaveRoom {roomId}` 幂等：不在该房也返回成功。连接断开等同退房。
5. 同一用户的不同连接可以在不同房间。进出房间**不向其他成员广播**（三端都没有监听成员进出，2.6 节）。
6. 被禁言的用户可以进房、收消息。进房不写会话列表，不计未读（PLAN 2.1 节）。
7. 只有登录用户可以进房（L-6）。
8. 房内人数达到 `IM_ROOM_MAX_ONLINE`，或节点连接数达到 `IM_NODE_MAX_CONNS` 时拒绝新进入（4138）。默认值由 L-14 和压测确定，未校准。

**验收标准**
- AC-ROOM-02-1：输入 `match-1` 已有 u1；操作 u2 进房；预期 ack 带 `online=2`、`recent`、`deletedMsgIds`；u1 收不到"进房"帧（2 秒窗口，排除 `roomOnline`）；错误码 —。
- AC-ROOM-02-2【后端】：输入 不存在的房间；操作 `joinRoom`；预期 ack 错误；错误码 4047。
- AC-ROOM-02-3【后端】：输入 同一连接在 `room-a`；操作 进 `room-b`，然后 u9 在 `room-a` 发言；预期 进房成功，该连接收不到 u9 的消息；`GET /admin/rooms/room-a` 的 `online` 减 1；错误码 —。
- AC-ROOM-02-4【后端】：输入 同一连接；操作 连续两次进 `match-1`，再对 `match-2` 执行 `leaveRoom`；预期 都成功，`online` 不重复计数，该连接仍在 `match-1`，能收到其中的新消息；错误码 —。
- AC-ROOM-02-5【后端】：输入 u1 的 web 在 `room-a`、pc 在 `room-b`；操作 分别有人发言；预期 两个连接各收各房的消息；错误码 —。
- AC-ROOM-02-6【后端】：输入 u1 在房内；操作 直接断开连接；预期 其他人下一次 `roomOnline` 不再计入 u1；错误码 —。
- AC-ROOM-02-7：输入 u1 在房内；操作 查 `convs`；预期 不出现 `room:` 会话，未读无变化；错误码 —。
- AC-ROOM-02-8【后端】：输入 没有完成 `auth` 的连接；操作 发 `joinRoom`；预期 按首帧 `auth` 规则关闭连接，不进房；错误码 —。
- AC-ROOM-02-9【后端】：输入 `IM_ROOM_MAX_ONLINE=2`，房内已有 u1、u2；操作 u3 进房；u1 用第二个连接进房；预期 u3 被拒；u1 的第二个连接成功（不增加不同 userId 数）；错误码 4138。

#### F-ROOM-03 断线重连后自动回房（L2，P0）

**规则**
1. SDK 记住当前房间；重连并 `auth` 成功后自动重新 `joinRoom`。重复的 `recent` 按 `msgId` 去重，规则见 F-ROOM-12。
2. 回房失败（4047）时通知业务代码"房间已关闭"，不再重试。其他失败按退避重试（1、2、4…最长 30 秒）。
3. 调用 `leaveRoom`、退出登录或收到 `roomClosed` 后不再自动回房。依据：三端都需要自己处理重连和切房的时序（`FNChatManager.swift:166-170`、`chat_room_notifier.dart:206-222`），放进 SDK 后业务不必重复。
4. **连接成功后的顺序**：`auth` 成功后，`joinRoom`、`pushSync`、`convs` / `socialSync`（非仅直播模式）**并行且互不阻塞**，任一失败不影响其他。发送队列的 flush 只针对单聊和群聊，房间消息不进发送队列。
5. `joinRoom` 是"设置目标房间"语义：离线时调用不失败，连上后执行。
6. 被踢或凭证失效（fatal）后，业务取新 token 再 `connect` 时，自动回到原房间，除非已经调用 `leaveRoom` 或退出登录。
7. App 回前台时立即发一次短超时 ping（5 秒，未校准），超时就重连，不等下一次心跳（`connection.ts:35-36`，最长要等 40 秒）。

**验收标准**
- AC-ROOM-03-1【SDK】：输入 已在 `match-1`，`FakeTransport`；操作 断开重连；预期 自动发 `joinRoom`；业务收到的 `recent` 不含已展示过的 `msgId`；错误码 —。
- AC-ROOM-03-2【SDK】：输入 断线期间房间被关闭；操作 重连；预期 业务收到房间关闭回调，不再重试；错误码 4047。
- AC-ROOM-03-3【SDK】：输入 已调用 `leaveRoom`；操作 断开重连；预期 不发 `joinRoom`；错误码 —。
- AC-ROOM-03-4【SDK】：输入 已在房内，退出登录；操作 同一账号重新登录并连接；预期 不发 `joinRoom`；错误码 —。
- AC-ROOM-03-5：输入 u1 在房内；操作 重启服务端；预期 SDK 重连后自动回房，能继续收发；错误码 —。
- AC-ROOM-03-6【SDK】：输入 已在 `match-1`，token 过期触发 fatal；操作 `connect(新 token)`；预期 自动发 `joinRoom match-1`；错误码 —。
- AC-ROOM-03-7【SDK】：输入 回房返回 5000 或 4290；操作 用假时钟推进；预期 按 1、2、4 秒退避重试，成功后交付 `recent`；错误码 —。
- AC-ROOM-03-8【SDK】：输入 `FakeTransport`，`convs` 不返回；操作 重连；预期 `joinRoom`、`pushSync` 都已发出且结果正常交付；错误码 —。
- AC-ROOM-03-9【SDK】：输入 未连接；操作 `joinRoom(match-1)` 后再连接成功；预期 调用不报错，连上后发 `joinRoom`；错误码 —。
- AC-ROOM-03-10：输入 真机，在房内，App 切后台直到连接失效；操作 回前台；预期 5 秒内发现并重连回房；错误码 —。

#### F-ROOM-04 最近消息（L2，P0）

**规则**
1. 每个房间在内存保留最近 50 条消息（PLAN 2.1 节；是否够用见 N-4），进房时通过 `recent` 按时间升序返回。
2. 被删除的消息从缓冲移除。`push`（F-PUSH-05）不进缓冲。
3. 服务端重启后缓冲为空，这是可接受的（L-5）。聊天室不支持 `sync`，现有拒绝行为保留（`logic.go:413`）。

**验收标准**
- AC-ROOM-04-1【后端】：输入 房间已有 60 条消息；操作 u1 进房；预期 `recent` 为最后 50 条，升序；错误码 —。
- AC-ROOM-04-2【后端】：输入 一条消息已被删除；操作 u1 进房；预期 `recent` 不含该条；错误码 —。
- AC-ROOM-04-3【后端】：输入 有消息的房间；操作 重启服务端，u1 进房；预期 成功，`recent` 为空；错误码 —。
- AC-ROOM-04-4【后端】：输入 u1 在房内；操作 `sync {convId: "room:match-1"}`；预期 返回错误；错误码 4000。

#### F-ROOM-05 在线人数（L2，P0）

**规则**
1. 口径：当前在该房的**不同 userId 数**（L-7）。
2. `roomOnline {roomId, online}`（`frames.json:45`）向全房广播，最多每 5 秒一次（未校准），人数没变不发。时序：人数变化时，距上次广播 ≥ 5 秒就立即发；否则等窗口结束发最新值。是可覆盖帧（F-ROOM-08），积压时只保留最新一条。回房 ack 的 `online` 也由 SDK 发一次 `roomOnline` 事件给业务。
3. 三端现在都没有使用房内人数（2.5 节），这项能力是 PLAN 的 P0；列表页人数继续由业务接口提供，也可以调 `GET /admin/rooms/{id}`。

**验收标准**
- AC-ROOM-05-1【后端】：输入 u1 用 web、pc 两个连接在房内，u2 一个连接；操作 u3 进房看 ack；预期 `online=3`；错误码 —。
- AC-ROOM-05-2【后端】[IM 钟]：输入 u1 在房内，刚收到一条 `roomOnline`；操作 5 秒内 10 个用户进房，再调 8.2 节的窗口推进接口；预期 窗口结束前 u1 收不到新的 `roomOnline`，窗口结束后收到 1 条，人数是最新值；错误码 —。
- AC-ROOM-05-3【后端】[IM 钟]：输入 人数不变；操作 连续调 12 次窗口推进接口；预期 没有 `roomOnline`；错误码 —。
- AC-ROOM-05-4【SDK】：输入 回房 ack 带 `online=7`；操作 处理；预期 业务收到一次 `roomOnline`，`online=7`；错误码 —。

#### F-ROOM-06 文本消息收发（L2，P0）

**规则**
1. `send {convType: "room", to: roomId, clientMsgId, type: "text", body: {text}, ext?}`；文本规则同 F-MSG-01（1～5000 码点；房内是否单设更小上限见 L-21）。必须在房内，否则 4041。
2. ack 为 `{msgId, convId, ts}`，**没有 `seq`**（`SendAck.seq` 改条件必填）。下行 `msg` 增加 `fromNick`（发送时的昵称，服务端写入）和可选 `ext`。昵称从缓存取，缓存走 Invalidator，导入和改昵称都触发失效；用户没有昵称时 `fromNick` 为 userId。
3. `ext` 是 JSON 对象，序列化后 ≤ 1KB，服务端原样转发，用来承载 `vipLevel` 等（对应 `cloudCustomData`，2.4 节）。发言门槛由业务方决定，天听不判断。
4. 发送方的**发送连接不回显**（靠 ack）；同一用户在同房的其他连接会收到。依据：三端都在发送成功后本地加入列表（2.4 节）。
5. 去重：`(from, clientMsgId)` 在内存保留 5 分钟（未校准），重发返回原 ack；同一 `clientMsgId` 换房返回 4008。房间消息在 `send` 第一步查库判重之前分支（`logic.go:111`），不走持久化路径。
6. `customType`、`ext` 只对 `room` 开放；单聊和群聊传这两个字段时按现有严格解析返回 4000。

**验收标准**
- AC-ROOM-06-1：输入 u1、u2 在房内；操作 u1 发文本，`ext={vipLevel:3}`；预期 u2 1 秒内收到，带 `from`、`fromNick`、`ext`、`msgId`、`ts`；u1 只收到 ack，没有 `msg`；错误码 —。
- AC-ROOM-06-2【后端】：输入 u1 不在房内；操作 发送；预期 ack 错误；错误码 4041。
- AC-ROOM-06-3【后端】：输入 同一 `clientMsgId`；操作 u1 发两次；预期 ack 相同，u2 只收到一条；错误码 —。
- AC-ROOM-06-4【后端】：输入 `ext` 超过 1KB、或 5001 码点文本；操作 发送；预期 都被拒绝；错误码 4000、4006。
- AC-ROOM-06-5【后端】：输入 u1 的 web、pc 同在房内；操作 web 发言；预期 pc 收到，web 不收到；错误码 —。
- AC-ROOM-06-6【后端】：输入 同一 `clientMsgId`；操作 先在 `room-a` 发，再在 `room-b` 发；预期 第二次被拒；错误码 4008。
- AC-ROOM-06-7【后端】[IM 钟]：输入 u1 发过 `c1`；操作 推进 6 分钟再发 `c1`；预期 当作新消息，`msgId` 不同；错误码 —。
- AC-ROOM-06-8【后端】：输入 u1 没有昵称；u2 改昵称后；操作 分别发言；预期 u1 的 `fromNick=u1`；u2 的 `fromNick` 是新昵称；错误码 —。
- AC-ROOM-06-9【后端】：输入 单聊 `send` 带 `ext`；操作 发送；预期 被拒；错误码 4000。

#### F-ROOM-07 自定义消息与服务端下发（L2，P0，L-10）

**规则**
1. `type: "custom"`，`body: {customType: 整数, data: 对象}`，序列化后 ≤ 4KB。天听不解释内容。
2. 客户端可以发自定义消息，用于晒单（`customType=3`）；服务端通过 `IM_ROOM_CLIENT_CUSTOM_TYPES` 限定客户端允许的 `customType`，默认只允许 3（礼物、官方消息不能由客户端伪造）。
3. 服务端下发：`POST /admin/rooms/{roomId}/messages {type, body, ext?, fromUserId?, clientMsgId}`，scope `room.message`，用于礼物（5）、官方（2）、系统（1）。响应 `{msgId, ts, created}`。`fromUserId` 存在时，`from`、`fromNick` 取该用户（送礼人）；否则 `from`、`fromNick` 为空字符串（Schema 对 `room` 消息放开，见第 10 节）。**不要求发送者在房内**；`fromUserId` 被禁言或封禁时仍然下发（送礼是业务后端判断过的）。
4. 服务端下发的消息进入 `recent`，属于不可丢帧（F-ROOM-08），不受每连接 20 条/秒限制。去重键 `(调用方, roomId, clientMsgId)`，保留 5 分钟（未校准），重试返回原响应，`created=false`。
4a. 下发按"调用方 + 房间"限速，默认 50 条/秒（未校准，`IM_ROOM_PUSH_RATE`），超出 429 / 4290，否则单个调用方就能灌满所有连接的不可丢队列。下发内容不做敏感词检查（调用方可信，见 F-ROOM-11）。审计方式见 L-20。
5. **SDK 必须把不认识的 `customType` 原样交给业务**，不丢弃、不报错。业务层统一处理三端现在的差异（2.4 节）。

**验收标准**
- AC-ROOM-07-1：输入 u1、u2 在房内；操作 u1 发 `customType=3` 的晒单；预期 u2 收到 `body` 原样；错误码 —。
- AC-ROOM-07-2【后端】：输入 默认配置；操作 u1 发 `customType=5`；预期 拒绝；错误码 4030。
- AC-ROOM-07-3【后端】：输入 不在房内的 u5 送礼，昵称 `小五`；操作 业务方调下发接口，`fromUserId=u5`；预期 房内收到 `customType=5`，`from=u5`、`fromNick=小五`；新进房的人在 `recent` 看到；错误码 —。
- AC-ROOM-07-4【后端】：输入 `fromUserId` 不存在；房间不存在；操作 分别下发；预期 都是 404；错误码 4042、4047。
- AC-ROOM-07-5【SDK】：输入 `customType=99`；操作 收到；预期 业务回调拿到原始 `customType` 和 `data`；错误码 —。
- AC-ROOM-07-6【后端】：输入 `body` 超过 4KB；操作 发送、下发；预期 都拒绝；错误码 4000。
- AC-ROOM-07-7【后端】：输入 `IM_ROOM_CLIENT_CUSTOM_TYPES=3,9`；操作 u1 发 `customType=9`；预期 成功；错误码 —。
- AC-ROOM-07-8【后端】：输入 u1 在房内；操作 下发官方消息，不带 `fromUserId`；预期 200，响应带 `msgId`、`ts`、`created=true`；u1 收到 `from=""`、`fromNick=""`，通过 Schema 校验；错误码 —。
- AC-ROOM-07-9【后端】：输入 同一 `clientMsgId`；操作 下发两次，再在另一房间用同号下发；预期 第二次 `created=false`，房内只有一条；另一房间是新消息；错误码 —。
- AC-ROOM-07-10【后端】：输入 `IM_ROOM_PUSH_RATE=1`，冻结时钟；操作 同一调用方向同一房一次发 3 条；预期 超出的返回 429；错误码 4290。
- AC-ROOM-07-11【后端】：输入 `fromUserId=u5`，u5 已被禁言；操作 下发礼物；预期 成功，`from=u5`；错误码 —。

#### F-ROOM-08 限流与下行保护（L2，P0）

**规则**
1. 上行：检查顺序是禁言 → 单用户单房桶 → 用户全局桶（F-MSG-07 的 20 条/秒）。单房桶默认 2 条/秒（未校准，`IM_ROOM_USER_RATE`，容量 2），是独立的桶实例；被房间桶拒绝时不扣全局令牌（C-29）。超出 4290。
2. 下行：每个房间按 200ms 窗口（未校准）汇集，**窗口内最多选最新的 4 条**可丢消息广播，等效每连接 20 条/秒（未校准）；其余丢弃。按房选取，不按连接单独计算，不复用 `TokenBucket`（10 万次/秒走同一把锁不可行，`ratelimit.go:49-71`）。不增加批量帧（PLAN 提到的"合并"指合并发送窗口，帧格式不变）。帧字节按房共享，不为每个连接重新序列化。
3. **帧分类**，服务端和 SDK 都按这张表：

| 类别 | 帧 | 处理 |
|---|---|---|
| 可丢 | 房内客户端文本、客户端自定义消息 | 进窗口选取；进每连接的可丢队列，满了丢最旧的 |
| 可覆盖 | `roomOnline` | 每连接每房只留最新一条 |
| 不可丢 | 服务端下发的消息（礼物、官方、系统）、`msgDeleted`、`roomClosed`、`push`、ack、单聊和群聊的 `msg`、其他事件 | 进现有队列（满 256 帧按现有规则 4400），不受 20 条/秒限制 |

4. **两条队列**：`hub.Client` 增加按类别发送的接口；每个连接有不可丢队列（现有，上限 256 帧）和可丢队列（上限 128 帧，未校准）。写协程先发不可丢。可丢队列满不会导致断开。
5. **跨队列顺序**：写入 `msgDeleted` 时，从该连接的可丢队列和房间窗口里移除对应 `msgId`，所以不会出现"先收到删除、后收到消息"。写入 `roomClosed` 时清空该连接该房的可丢队列，`roomClosed` 之后不再有该房消息。不可丢和可丢之间的其他相对顺序不保证。
6. **不读数据的连接**：可丢部分被丢弃；不可丢队列满后 4400；写超时后仍按现有规则断开（`gateway.go:414-416`）。这种连接最终会被断开，不影响同房其他连接。
7. 丢弃不补发，也不通知客户端。聊天室消息没有 `seq`，这是预期行为。

**验收标准**
- AC-ROOM-08-1【后端】：输入 u1 在房内，冻结时钟；操作 一次性发 5 条；预期 前 2 条成功，其余 4290；被房间桶拒绝的不扣全局令牌（之后在另一房间仍能发）；错误码 4290。
- AC-ROOM-08-2【后端】：输入 100 个用户在房内，每人 1 条/秒，消息 200 字节；操作 持续 10 秒；预期 每个连接每秒收到可丢消息 ≥ 18 条且 ≤ 20 条，每个窗口收到的是该窗口内最新的；连接都不断开；错误码 —。
- AC-ROOM-08-3【后端】：输入 与 08-2 相同的高峰，所有连接持续读取；操作 同时下发 5 条礼物；预期 所有连接都收到这 5 条；错误码 —。
- AC-ROOM-08-4【后端】：输入 同一用户的一个连接不读 socket 的应用层（用不读取的测试客户端），另一连接在群聊；同房另有正常连接；操作 房内高峰持续 30 秒，并发一条群消息；预期 另一连接的群消息 1 秒内到达；正常连接不受影响；不读的连接最终被断开（4400 或写超时）；错误码 —。
- AC-ROOM-08-5【后端】[P-目标机]（8.1 节）：输入 单房 5000 用户在线（未校准，L-14），100 个发送者各 1 条/秒，消息 200 字节；操作 持续 10 分钟，统计**实际被下发的消息**从服务端受理到客户端收到的耗时，被窗口丢弃和被 4290 拒绝的不计入；预期 P99 ≤ 1 秒；正常读取的连接意外断开 0；每连接可丢消息 ≥ 18 条/秒；错误码 —。
- AC-ROOM-08-6【后端】：输入 与 08-2 相同的高峰，所有连接持续读取；操作 删除一条最近的消息；关闭房间；预期 所有连接都收到 `msgDeleted`；所有连接都收到 `roomClosed`，其后没有该房消息；错误码 —。
- AC-ROOM-08-7【后端】：输入 消息 m1 刚进入 200ms 窗口；操作 窗口结束前删除 m1；预期 m1 不再下发；没有连接先收到 `msgDeleted` 后收到 m1；错误码 —。
- AC-ROOM-08-8【后端】：输入 一个连接只读得很慢（读取速度低于下行），可丢队列已满；操作 向该连接发单聊和 `push`；预期 单聊和 `push` 都到达，同连接上不被聊天室拖累；错误码 —。
- AC-ROOM-08-9【后端】：输入 大量人数变化造成 `roomOnline` 积压；操作 恢复读取；预期 只收到最新一条；错误码 —。

#### F-ROOM-09 禁言在聊天室生效（L2，P0）

**规则**
1. 沿用用户级禁言（F-ADM-40 规则 1 已包含"二期聊天室"）：房内发送返回 4031，`details.scope=user`；检查在限流之前，不扣令牌。`IM_ROOM_ENABLED=1` 时必须 `IM_V13_ADMIN_FEATURES=1`，否则启动失败（V13 关闭时禁言不生效，`logic/moderation.go:127-129`）。
2. 依据：现有禁言是全局生效（`chat_room_notifier.dart:89`），不是按房间，所以**二期不增加房间级禁言**；需要时作为 P1。
3. 客户端配合沿用 `muted` / `unmuted` 事件和 `auth` ack 的 `mutedUntil`，不再需要 contentType=1 推送。
4. 谁能禁言由业务服务端判断，天听不建房间角色（2.7 节）。

**验收标准**
- AC-ROOM-09-1：输入 u1 在房内，被用户级禁言；操作 发送文本、晒单；预期 都被拒绝，`details.scope=user`；u1 仍能收到其他人的消息；错误码 4031。
- AC-ROOM-09-2【后端】：输入 被禁言的 u1；操作 进房；预期 成功；错误码 —。
- AC-ROOM-09-3【SDK】：输入 u1 在房内；操作 解除禁言后再发送；预期 收到 `unmuted`，发送成功；错误码 —。
- AC-ROOM-09-4【后端】：输入 `IM_ROOM_ENABLED=1`、`IM_V13_ADMIN_FEATURES=0`；操作 启动服务端；预期 启动失败，日志指出配置项；错误码 —。
- AC-ROOM-09-5【后端】：输入 被禁言的 u1，冻结时钟；操作 连发 5 条被拒，解禁后立即发 2 条；预期 前 5 条 4031；后 2 条成功，证明禁言不扣令牌；错误码 4031。
- AC-ROOM-09-6：输入 u1 在房内；操作 管理接口禁言 u1；预期 u1 收到 `muted` 事件；错误码 —。

#### F-ROOM-10 删除聊天室消息（L2，P0）

**规则**
1. `POST /admin/rooms/{roomId}/messages/delete {msgIds, reason}`，scope `msg.delete`，单次 ≤ 50，与现有删除接口一致（`admin/moderation.go:29`）。
2. 从缓冲移除，向全房广播 `msgDeleted {convId, msgIds}`，**没有 `seqs`**（现有 Schema 要求必填，需要改为条件必填）。`msgDeleted` 不可丢（F-ROOM-08）。
3. 消息已不在缓冲时**也广播**，并返回成功：客户端列表里可能还有。
4. **回房兜底**：`recent` 排除已删消息，只能保证新进的人看不到；断线期间被删的消息还留在业务列表里。所以每个房间在内存保留最近 200 条（未校准）被删的 `msgId`，通过 `joinRoom` ack 的 `deletedMsgIds` 返回；SDK 回房后对其中已交付过的 `msgId` 发一次 `msgDeleted`。这样现有 App 的"进房取已删 ID 列表"和 contentType=3 推送都可以去掉。服务端重启后列表为空，此时重启前已交付、断线期间被删的消息可能留在屏幕上，作为已接受风险。
5. 聊天室消息不持久化，PRD-admin F-ADM-32 的原文隔离不适用（L-5）。

**验收标准**
- AC-ROOM-10-1：输入 u1、u2 在房内，一条消息 m1；操作 业务方删除 m1；预期 两人 3 秒内收到 `msgDeleted`，业务回调拿到 m1；错误码 —。
- AC-ROOM-10-2【后端】：输入 已被挤出缓冲的 m0；操作 删除；预期 成功并广播；错误码 —。
- AC-ROOM-10-3【后端】：输入 51 个 `msgIds`；操作 删除；预期 400；错误码 4131。
- AC-ROOM-10-4【后端】：输入 不存在的房间；操作 删除；预期 404；错误码 4047。
- AC-ROOM-10-5：输入 u1 收到过 m1 后断线；操作 断线期间删除 m1，u1 重连回房；预期 ack 的 `deletedMsgIds` 含 m1，业务回调拿到 m1 被删，列表不再显示；错误码 —。
- AC-ROOM-10-6【SDK】：输入 `deletedMsgIds` 含本实例没交付过的 m9；操作 回房；预期 不通知业务；错误码 —。

#### F-ROOM-11 内容拦截（L2，P0，L-9）

**规则**
1. 房内**客户端文本**发送前检查敏感词。命中时返回 4510，消息不广播、不进缓冲。**本期只做拦截**，不做 F-ADM-60 的替换模式。
2. 校验前做 NFKC + 去变音符，防止绕过；**只用于匹配，广播和存入缓冲的仍是原文**。依据：现有客户端自己做（2.7 节）；放到服务端后三端行为一致。注意 fx-sport 现在发送的是标准化后的文本（`chat_room_notifier.dart:581-608`），迁移时由业务决定是否继续。
3. 检查范围：客户端的 `text`。**不查**晒单的 `data`、`ext`、服务端下发的消息（业务后端可信；是否扩大由 L-9 决定）。
4. 词库最小实现：`IM_ROOM_BLOCKED_WORDS_FILE` 指向的文本文件，每行一词，启动时加载，变更走现有 Invalidator（新主题 `blockedWords`）并由管理接口 `POST /admin/rooms/blocked-words/reload`（scope `room.write`）触发。不在 im-server 建词库管理页面。词库内容和负责人见 L-9。
5. `golang.org/x/text` 从间接依赖改为直接依赖（`server/go.mod:26`）。
6. 客户端提示沿用现有"消息包含敏感词，发送失败"（`chat_room_notifier.dart:610`）。

**验收标准**
- AC-ROOM-11-1【后端】：输入 拦截词"违规词"；操作 房内发送含该词的文本；预期 被拒绝，其他人 2 秒内收不到（排除 `roomOnline`），`recent` 没有；错误码 4510。
- AC-ROOM-11-2【后端】：输入 同上；操作 发送全角或带变音符的变体；预期 同样被拦截；错误码 4510。
- AC-ROOM-11-3【SDK】：输入 服务端返回 4510；操作 发送；预期 `sendRoomText` 返回错误码 4510，SDK 不做本地回显；错误码 4510。
- AC-ROOM-11-4【后端】：输入 拦截词在晒单 `data` 和 `ext` 中；业务方下发的官方消息含拦截词；操作 分别发送；预期 都成功；错误码 —。
- AC-ROOM-11-5【后端】：输入 带全角的普通文本（不含拦截词）；操作 发送后 u2 接收；预期 u2 收到原文，未被标准化；错误码 —。
- AC-ROOM-11-6【后端】：输入 词库文件新增一词；操作 调 reload，再发含该词的文本；预期 被拦截；错误码 4510。

#### F-ROOM-12 SDK 聊天室接口（L2 sdk-ts，L3 其他，P0）

**规则**
1. 接口（对齐 PLAN 3.2 节，四套 SDK 语义一致）：

| 接口 | TS | Kotlin | Swift | 说明 |
|---|---|---|---|---|
| 进房 | `joinRoom(roomId): Promise<RoomJoined>` | `suspend joinRoom(roomId): RoomJoined` | `joinRoom(roomId) async throws -> RoomJoined` | 设置目标房间；离线时连上后执行 |
| 退房 | `leaveRoom(): Promise<void>` | `suspend leaveRoom()` | `leaveRoom() async throws` | 无参，SDK 用当前 `roomId` 发线上的 `leaveRoom {roomId}` |
| 发文本 | `sendRoomText(text, ext?): Promise<RoomSendAck>` | `suspend sendRoomText(text, ext): RoomSendAck` | `sendRoomText(text, ext:) async throws -> RoomSendAck` | SDK 生成 `clientMsgId`；不做本地回显；业务在成功后加入列表（2.4 节） |
| 发自定义 | `sendRoomCustom(customType, data, ext?)` | 同左 | 同左 | 返回同上 |
| 事件 | `on('roomMessage' \| 'roomOnline' \| 'roomDeleted' \| 'roomClosed', …)` | `roomEvents: Flow<RoomEvent>` | `on(.room…)` | 只交付当前房间 |

Flutter 的签名随 L-12 确定。`RoomJoined = {online, recent}`，`RoomSendAck = {msgId, ts}`。
2. **独立模型**：新建 `RoomMessage`（无 `seq`，有 `fromNick`、`ext`、`customType`、`data`）和 `RoomSendAck`。房间帧在 SDK 入口分流，**不进 Store、不写本地库、不进会话列表**。现有模型 `seq` 必填，Kotlin、Swift 会解码失败丢弃（`Protocol.kt:148`、`Protocol.swift:91`），TS 会污染会话（`store.ts:351-355`）。`msgDeleted` 按 `room:` 前缀分流。`convid.json` 里 `room:` 仍判非法，因为房间不走现有 `send()`。
3. 进房结果把 `recent` 交给业务。
4. 换房串行：连续调用时只保留最后一个目标，过时的结果不通知（三端现在各自实现，2.3 节）。入口过滤非当前房间的帧。
5. **已见集合**：按当前房间保留最近 1000 个 `msgId`（未校准），包含自己发送 ack 里的 `msgId`；**`leaveRoom`、切房时清空**，所以显式退房再进房时 `recent` 全部重新交付（fx-sport 进下级页面再返回靠这个重填列表，`chat_room_notifier.dart:291-301`）。只有 SDK 自动回房时才用已见集合过滤 `recent`。
6. 超时：发送超时后用同一 `clientMsgId` 重试一次，仍失败返回可识别错误。
7. **仅直播模式**（`LiveClient` 或初始化参数，由 L-12 定形态）：关闭 `convs`、社交和消息库，只保留连接、房间、推送、游标。业务 App 只要这些，否则每次重连都多跑 `convs`、`socialSync`（`client.ts:415-426`、`IMClient.kt:866-876`、`Client.swift:477-487`）。

**验收标准**
- AC-ROOM-12-1【SDK】：输入 `FakeTransport`；操作 跑 `frames.json` 的房间用例和行为用例 `room-session.json`；预期 全部通过；错误码 —。
- AC-ROOM-12-2【SDK】：输入 连续调用 `joinRoom(a)`、`joinRoom(b)`、`joinRoom(c)`；操作 ack 乱序返回；预期 最终状态是 c，a、b 的结果不通知业务；错误码 —。
- AC-ROOM-12-3【SDK】（L3，sdk-kotlin、sdk-swift）：输入 进房并收到消息；操作 查本地数据库；预期 没有房间消息；错误码 —。
- AC-ROOM-12-4【SDK】：输入 进房并收到消息和 `msgDeleted`；操作 查 store 和会话列表；预期 没有房间消息，不出现 `room:` 会话；错误码 —。
- AC-ROOM-12-5【SDK】：输入 切到 `room-b` 后；操作 收到 `room-a` 的迟到帧；预期 不交付；错误码 —。
- AC-ROOM-12-6【SDK】：输入 在 `match-1` 收到过 m1；操作 `leaveRoom` 后再 `joinRoom(match-1)`，`recent` 含 m1；预期 m1 重新交付，界面不是空的；错误码 —。
- AC-ROOM-12-7【SDK】：输入 自己发的消息 ack 带 m2；操作 重连，`recent` 含 m2；预期 m2 不交付给业务；错误码 —。
- AC-ROOM-12-8【SDK】：输入 `recent` 含 m1，实时帧 m3 在 ack 之前到达（服务端保证不发生，SDK 仍要兼容）；操作 处理；预期 交付顺序 m1、m3，不重复；错误码 —。
- AC-ROOM-12-9【SDK】：输入 超时未收到 ack；操作 `sendRoomText`；预期 用同一 `clientMsgId` 重试一次；仍失败时返回可识别错误；错误码 —。
- AC-ROOM-12-10【SDK】：输入 收到 `roomClosed`；操作 重连；预期 不发 `joinRoom`；错误码 —。
- AC-ROOM-12-11【SDK】：输入 仅直播模式；操作 连接和重连；预期 只发 `auth`、`joinRoom`、`pushSync`，不发 `convs`、`socialSync`，不打开消息库；错误码 —。

#### F-ROOM-13 聊天室可观测（L2，P1）

**规则**：指标：房间数、房内连接数、消息数、丢弃数（按原因）。**房间 ID 不作为指标标签**，防止基数爆炸；日志不写正文，只写 `userId` 和 `roomId`（PRD-core 第 6 节）。

**验收标准**
- AC-ROOM-13-1【后端】：输入 触发下行丢弃；操作 读 `/metrics`；预期 丢弃数增加，标签中没有 `roomId`；错误码 —。
- AC-ROOM-13-2【后端】：输入 发送含特征串的文本；操作 扫描日志；预期 搜不到特征串；错误码 —。

#### F-ROOM-14 H5 聊天室演示页（L2，P1）

**规则**：PLAN 3.3 节的模拟比赛直播间：在线人数 + 消息流 + 推送横幅。文本一律按纯文本渲染（F-MSG-06）。它和天听正式导航的关系待 PRD-ui 变更时确定。

下表是聊天室状态的默认表现，H5 演示页遵守；业务 App 可以覆盖，但必须处理每一种状态：

| 状态 | 输入框 | 提示 |
|---|---|---|
| 进房中（等 ack） | 禁用 | 加载态，不显示空列表文案 |
| 断线、回房中 | 禁用 | "连接中…" |
| 回房失败，退避中 | 禁用 | "连接中…" |
| 4047 / `roomClosed` | 消失 | "直播间已关闭" |
| 4031 被禁言 | 禁用 | 沿用禁言文案（`mutedText`） |
| 4290 发言太快 | 可用，内容保留 | "发言太快，请稍后再试" |
| 4510 被拦截 | 可用，内容保留 | "消息包含敏感词，发送失败" |
| 4138 房间已满 | 消失 | "直播间人数已满" |
| 人数 | — | 收到第一个 `online` 后才显示 |

**验收标准**
- AC-ROOM-14-1：输入 u1、u2 两个账号各开一个标签页进同一房间；操作 互发消息；预期 互相 3 秒内可见，人数 5 秒内显示 2（同一账号两个标签页显示 1）；错误码 —。
- AC-ROOM-14-2【前端】：输入 文本 `<img src=x onerror=alert(1)>`；操作 显示；预期 原样显示为文本，不弹窗；错误码 —。
- AC-ROOM-14-3【前端】：输入 `FakeTransport`；操作 依次模拟状态表的每一行；预期 输入框和提示与表一致；错误码 —。

#### F-ROOM-15 聊天室并发与错误优先级（L2，P0）

**规则**
1. 房内 `send` 的错误优先级：未完成 `auth` → 4000（参数）→ 4041（不在房内，含房间已关闭）→ 4031（禁言）→ 4030（`customType` 不允许）→ 4510（敏感词）→ 4290（限流）。禁言和敏感词都先于限流，命中时不扣令牌。
2. 关闭房间与进房、发言用同一把房间锁：关闭之后的进房一律 4047；关闭之前已通过检查的发言可以广播，但排在 `roomClosed` 之前。
3. 在线人数以订阅表为准，进出都在房间锁内更新。

**验收标准**
- AC-ROOM-15-1【后端】：输入 同时命中不在房内、参数非法；被禁言且超限；操作 分别发送；预期 分别是 4000、4031；错误码 4000、4031。
- AC-ROOM-15-2【后端】：输入 200 个连接并发进出同一房间，结束时留下 50 个不同 userId；操作 `GET` 房间；预期 `online=50`；开 race 检测无报告；错误码 —。
- AC-ROOM-15-3【后端】：输入 房内持续发言，同时并发进房；操作 关闭房间；预期 所有连接最后一帧是 `roomClosed`，其后没有该房消息；关闭后的进房都是 4047；错误码 4047。
- AC-ROOM-15-4【后端】：输入 8.1 节 [P-开发机] 环境，礼物下发 50 条/秒；操作 同时跑单聊压测 5 分钟，与不下发的基线同一轮交替对照；预期 单聊 ack P99 的增幅 ≤ 20%（未校准）；错误码 —。

---

## 6. 功能需求：系统推送

> 协议细节见第 4 节。

#### F-PUSH-01 下行 `push` 帧（L1，P0）

**规则**：字段见 4.2 节。服务端不解释 `type` 和 `data`。三套 SDK 忽略不认识的字段；兼容规则补上**不认识的顶层 `cmd` 也忽略**（PRD-core 第 8 节只写了事件类型、字段、枚举值，变更见第 11 节）。`IM_PUSH_ENABLED` 关闭时：服务端对 `pushSync` 返回 4001，SDK **静默忽略**这一次的 4001（不提示升级，下次连接再试）；`POST /admin/push` 路由不注册，返回 404（和 `IM_V13_ADMIN_FEATURES` 关闭时第二批路由的做法一致）。这两个开关互相独立。

**验收标准**
- AC-PUSH-01-1（跨服务，真实服务端加 sdk-ts；BE-228、FE-208 各管下发和接收）：输入 u1 在线；操作 按用户推送 `type=notice`；预期 3 秒内收到，`pushId`、`type`、`data`、`ts`、`target=user`、`pushSeq` 都正确；错误码 —。
- AC-PUSH-01-2【后端】：输入 `frames.json` 新增的 user、all、room 三种帧；操作 用 Schema 校验；预期 全部通过；缺 `pushId` 的帧校验失败；错误码 —。
- AC-PUSH-01-3【SDK】：输入 帧中有未知字段 `foo`；操作 收到；预期 正常交给业务；错误码 —。
- AC-PUSH-01-4【SDK】：输入 服务端对 `pushSync` 返回 4001；操作 连接成功后；预期 不抛错、不提示升级、不退避，其他功能正常；这次连接内缺口定时器到期、退订再订阅都不发 `pushSync`，实时帧照常交付；下次连接再发一次；错误码 4001。
- AC-PUSH-01-5【SDK】：输入 顶层 `cmd=futureThing` 的帧；操作 收到；预期 忽略，连接保持；错误码 —。

#### F-PUSH-02 推送管理接口（L1，P0）

**规则**
1. 接口见 4.2 节，只在管理地址提供，走 F-KEY 的签名和 scope。
2. `data` 序列化后 ≤ 4KB，否则 400 / 4000。`type` 1～64 字符。`pushId` 格式不符 400 / 4000。
3. `userIds` 去重后 ≤ 1000，超出 400 / 4131；`target=user` 时不能为空；`accepted` 是去重后的人数。有不存在的用户：404 / 4042，`details.userIds` 列出，**整批不发**（业务调用方是否允许部分成功见 L-19）。
4. `ttlSeconds` 是可选字段，**不传和传 0 含义不同**：`target=user` 不传时为 604800，传 0 表示只投在线；其他 `target` 不传时为 0，显式传非 0 时 400 / 4000。取值 0～604800。
5. 模板 Schema 校验只在 console-server（F-ADM-50 规则 2）；业务方直接调用时，im-server 不校验模板。
6. `target=room` 必须带 `roomId`，其他 `target` 不能带；L1 未开启房间时（`IM_ROOM_ENABLED` 关闭）`target=room` 返回 400 / 4000。
7. 错误优先级：签名和 scope → `type` 白名单（F-PUSH-09 规则 3，403 / 4030）→ 4000 参数 → 4094 / 幂等命中 → 4131 → 4042 / 4047（4047 随 L2 生效） → 4290。**幂等检查先于限速**，所以重放不会被限速挡住。

**验收标准**
- AC-PUSH-02-1【后端】：输入 合法的 `target=user`、`target=all`；操作 分别调用；预期 200，`created=true`；`target=user` 时 `accepted` 等于目标用户数，`all` 没有该字段；错误码 —。
- AC-PUSH-02-2【后端】：输入 `data` 为 4097 字节；操作 调用；预期 400，不投递；错误码 4000。
- AC-PUSH-02-3【后端】：输入 `userIds=[u1, ghost1, ghost2]`；操作 调用；预期 404，`details.userIds=[ghost1, ghost2]`，u1 收不到；错误码 4042。
- AC-PUSH-02-4【后端】：输入 1001 个 userId；操作 调用；预期 400；错误码 4131。
- AC-PUSH-02-5【后端】：输入 `target=all, ttlSeconds=3600`；`ttlSeconds=604801`；操作 分别调用；预期 都是 400；错误码 4000。
- AC-PUSH-02-6【后端】：输入 只有 `push.user` 的调用方；操作 `target=all`；预期 403；错误码 4030。
- AC-PUSH-02-7【后端】（L2）：输入 合法的 `target=room`；操作 调用；预期 200，`created=true`，没有 `accepted`；错误码 —。
- AC-PUSH-02-8【后端】（L1）：输入 `IM_ROOM_ENABLED` 关闭，调用方持有 `push.room`；操作 `target=room`；预期 400（没有 `push.room` 的调用方按优先级是 403 / 4030，见 AC-PUSH-05-5）；错误码 4000。
- AC-PUSH-02-9【后端】：输入 `type` 为空、65 字符；`pushId` 含空格、65 字符；操作 分别调用；预期 都是 400；错误码 4000。
- AC-PUSH-02-10【后端】：输入 `data` 恰好 4096 字节；恰好 1000 个 userId；操作 分别调用；预期 都是 200；错误码 —。
- AC-PUSH-02-11【后端】：输入 调用方持有 `push.user`、`push.all`、`push.room`，`IM_ROOM_ENABLED` 关闭；`target=user` 且 `userIds` 为空；`target=room` 缺 `roomId`；`target=all` 带 `roomId`；`userIds=[u1,u1]`；操作 分别调用；预期 前三个 400；最后一个 200、`accepted=1`，u1 只收到一条；错误码 4000。
- AC-PUSH-02-12【后端】：输入 `target=all` 不传 `ttlSeconds`；`target=user` 不传；`target=user, ttlSeconds=0`；操作 分别调用；预期 都是 200；第二个进收件箱，第三个不进；错误码 —。
- AC-PUSH-02-13【后端】：输入 不符合 console 模板的 `data`，业务调用方；操作 调用；预期 200，im-server 不校验模板；错误码 —。
- AC-PUSH-02-14【后端】：输入 `IM_PUSH_ENABLED` 关闭，`IM_V13_ADMIN_FEATURES` 分别开、关；操作 调用 `POST /admin/push`，WS 发 `pushSync`；预期 前者都是 404（路由未注册）；后者 ack 错误 4001；错误码 4001。

#### F-PUSH-03 按用户推送与离线补收（L1，P0）

**规则**：见 4.3 节。`pushSync` 的 `limit` 1～100，默认 50（SDK 固定用 100），超范围是 ack 错误 4000（这是 WS 命令，不是 HTTP）。`afterSeq`、`fromLatest` 二选一（4.2 节）。只能拉自己的收件箱。

**验收标准**
- AC-PUSH-03-1【后端】[IM 钟]：输入 u3 离线；操作 推送后第 6 天上线并 `pushSync {afterSeq:0}`；预期 收到；错误码 —。
- AC-PUSH-03-2【后端】[IM 钟]：输入 u4 离线，定时清理被关闭；操作 推送后推进 8 天，`pushSync`；预期 收不到，证明靠读取时过滤；错误码 —。
- AC-PUSH-03-3【后端】：输入 u1 有 120 条未过期推送；操作 `limit=50` 连续翻页；预期 共 120 条，升序，无重复；`hasMore` 最后一页为 false；错误码 —。
- AC-PUSH-03-4【SDK】：输入 游标为 41，`FakeTransport`；操作 收到 `pushSeq=44` 的实时帧，等 500ms；预期 触发 `pushSync {afterSeq:41}`；业务按 42、43、44 收到，不重复；三条都是 `replayed=false`（`ts` 都不早于 `T0`）；错误码 —。
- AC-PUSH-03-5【后端】：输入 `ttlSeconds=0`；操作 向在线的 u1 推送；向离线的 u2 推送，u2 上线 `pushSync`；预期 u1 收到的帧没有 `pushSeq`；u2 结果为空；错误码 —。
- AC-PUSH-03-6【后端】：输入 `limit` 为 0、1、100、101；操作 分别发 `pushSync`；预期 1、100 成功；0、101 的 ack 错误；错误码 4000。
- AC-PUSH-03-7【SDK】：输入 已注册订阅者；操作 `auth` 成功；预期 SDK 自动发一次 `pushSync`；没有订阅者时不发（AC-PUSH-03-25）；错误码 —。
- AC-PUSH-03-8【后端】[IM 钟]：输入 u1 有 41（ttl 1 小时）、42（ttl 7 天）、43（ttl 1 小时）；操作 推进 2 小时后 `pushSync {afterSeq:40, limit:1}`；预期 只返回 42，`hasMore=false`，`latestSeq=43`；错误码 —。
- AC-PUSH-03-9【SDK】：输入 游标 41，`pushSync` 返回空、`hasMore=false`、`latestSeq=43`；操作 处理；预期 游标变为 43，不交付；错误码 —。
- AC-PUSH-03-10【SDK】：输入 游标 100，服务端恢复后 `pushSync {afterSeq:100}` 返回空、`latestSeq=55`，收件箱里 1～50 是见过的、51～55 是新的；操作 处理；预期 再发 `pushSync {afterSeq:0}`；只交付 51～55；游标变为 55；之后 `pushSeq=56` 的实时帧按序交付；错误码 —。
- AC-PUSH-03-11【SDK】：输入 游标 41；操作 收到 `target=all`、`target=room`（带 `roomId`）和 `ttlSeconds=0` 的帧（无 `pushSeq`）；预期 交付给业务，`target`、`roomId` 原样，游标仍是 41；错误码 —。
- AC-PUSH-03-12【后端】[IM 钟]：输入 `ttlSeconds=3600` 的推送；操作 时钟恰好等于 `expires_at`、早 1 毫秒，分别 `pushSync`；预期 等于时不返回，早 1 毫秒时返回；错误码 —。
- AC-PUSH-03-13【后端】[IM 钟]：输入 u1 的全部推送已过期并被清理任务删除；操作 再推一条；预期 `pushSeq` 继续递增，不回退，`latestSeq` 正确；错误码 —。
- AC-PUSH-03-14【后端】[IM 钟]：输入 u1 被封禁期间有推送；操作 解封后在有效期内连接并 `pushSync`；预期 补收到；错误码 —。
- AC-PUSH-03-15【SDK】：输入 `hasMore=true` 的第一页；操作 处理；预期 发 `limit=100`，用本页最大 `pushSeq` 继续拉取，直到 `hasMore=false`；暂存帧能接上本页时先接上，`afterSeq` 是接上之后的游标；错误码 —。
- AC-PUSH-03-16【SDK】（原生）：输入 游标 41，分别走退出登录再登录、被踢后重登、库损坏重建、Schema 升级；操作 同一账号连接；预期 `pushSync {afterSeq:41}`，不回放；C8 把游标重置为 0 后进程退出，重启后发 `pushSync {afterSeq:0}`，见过的不重复交付；错误码 —。
- AC-PUSH-03-17【SDK】：输入 启动参数 `pushStart=latest`，本机没有游标，已注册订阅者；操作 连接；预期 发 `pushSync {fromLatest:true}`，不交付任何历史，游标设为 `latestSeq`，应答前到达的 `latestSeq+1` 在应答后交付；已有游标时不走这条；错误码 —。
- AC-PUSH-03-18【SDK】：输入 游标 41；操作 收到 43，400ms 内收到 42；预期 不发 `pushSync`，按 42、43 交付；错误码 —。
- AC-PUSH-03-19【SDK】：输入 一次 `pushSync` 进行中；操作 又发现缺口；预期 不并发第二个，当前结束后如仍有缺口再拉；错误码 —。
- AC-PUSH-03-20【SDK】：输入 `pushSync` 持续失败，暂存了 44；操作 用假时钟从首次失败起推进 5 秒；预期 44 交付给业务，游标不前进；后续按退避重试；错误码 —。
- AC-PUSH-03-21【后端】：输入 `pushSync` 带多余字段 `userId`；操作 发送；预期 ack 错误；错误码 4000。
- AC-PUSH-03-22【后端】：输入 `{fromLatest:true}` 且 u1 有推送；操作 `pushSync`；预期 `pushes` 为空，`latestSeq` 正确，`hasMore=false`；错误码 —。
- AC-PUSH-03-23【后端】：输入 `{limit:10}`；`{afterSeq:1, fromLatest:true}`；`{fromLatest:false}`；操作 分别发 `pushSync`；预期 都是 ack 错误；错误码 4000。
- AC-PUSH-03-24【SDK】：输入 `FakeTransport` 不回 `convs`；操作 `auth` 成功；预期 `pushSync` 照常发出，推送正常交付；`pushSync` 失败时 `convs` 也不受影响；错误码 —。
- AC-PUSH-03-25【SDK】：输入 没有订阅者，收件箱 1000 条，其中 `pushSeq=1000` 是连接后到达的实时帧；操作 连接后等 60 秒；预期 **一次 `pushSync` 都不发**，帧被丢弃，游标不动；注册订阅者后到下一个调度点发 `pushSync {afterSeq:0}`，每页交付完才取下一页，`afterSeq` 依次为 0、100…900，共 10 页，1000 条按序只交付一次，游标到 1000；错误码 —。
- AC-PUSH-03-26【SDK】：输入 游标 41，暂存 `pushSeq=200`，第一页是 42～141、`hasMore=true`；第二种情况：应答 `latestSeq=43`，`pushSeq=44` 的帧先于 ack 到达；操作 处理；预期 第一种情况 200 不抢先，按序在 199 之后只交付一次；第二种情况 42、43 之后交付 44，游标到 44，`pushSynced {44}`；错误码 —。
- AC-PUSH-03-27【SDK】：输入 `pushSync` 失败，正在退避，暂存 `pushSeq=200`；操作 断线，假时钟推进 10 秒，再重连成功；预期 断线期间 200 不交付、不发请求；重连后立即发 `pushSync`，不等退避结束，200 按序在 199 之后交付；断开时进行中的请求算失败，它迟到的 ack 被忽略，不影响下一次单飞；错误码 —。
- AC-PUSH-03-28【SDK】：输入 一次 `pushSync` 进行中，暂存 `pushSeq=44`；操作 全部订阅者退订，应答到达，等 10 秒；预期 应答被丢弃，不交付，游标不动，不再发请求；重新订阅后从原游标补拉，内容只交付一次；错误码 —。
- AC-PUSH-03-29【SDK】：输入 暂存区上限配置为 3，已暂存 3 条；操作 再到达 1 条有缺口的实时帧，500ms 后补拉；预期 暂存被清空，只留这一帧，补拉后所有序号按序只交付一次；错误码 —。
- AC-PUSH-03-30【SDK】：输入 `pushSync` 依次返回 4000、4290、5000 和请求超时；操作 用假时钟推进；预期 都按退避重试，间隔 1、2、4、8、16、30、30 秒；成功后、断开重连后都从 1 秒重新开始；超时后迟到的 ack 被忽略；错误码 —。
- AC-PUSH-03-31【SDK】：输入 游标 100，暂存 `pushSeq=105`，`pushSync {afterSeq:100}` 返回空、`latestSeq=55`；操作 处理，重拉结束后等 60 秒；预期 暂存被清空，不再有 `pushSync`；`pushSynced` 是 55，不带恢复前交付的序号；错误码 —。
- AC-PUSH-03-32【SDK】：输入 只有一个订阅者，一页 42、43、44；操作 回调收到 43 时退订，60 秒后再订阅；预期 44 不交付，游标不动，不取下一页，不发 `pushSynced`；再订阅后发 `afterSeq` = 原游标，只交付 44；有两个订阅者时只退订其中一个不影响另一个；错误码 —。
- AC-PUSH-03-33【SDK】：输入 `pushSync` 持续失败，暂存 44、45，失败状态下又到达 47；操作 用假时钟推进；预期 从首次失败起恰好 5000ms 三条一起按序交付，4999ms 时都不交付；暂存区满清空后只交付留下的那一帧；期间有一次成功应答时计时取消，再次失败重新计 5 秒；断开重连后旧定时器不再触发；错误码 —。
- AC-PUSH-03-34【SDK】（Kotlin，L3）：输入 没有注入 `PushCursorStore`；操作 连接、收推送、重建实例；预期 不报错，新实例从 `afterSeq:0` 开始，行为和 H5 相同；错误码 —。
- AC-PUSH-03-35【SDK】：输入 `hasMore=true` 但 `pushes` 为空；操作 处理；预期 按失败退避，不立即重发，游标不动；错误码 —。

#### F-PUSH-04 全员推送（L1，P0）

**规则**
1. 只投递当前在线连接（L-1）；不进收件箱。
2. 集群下经消息总线广播（B6）；单节点遍历连接异步下发，不占用发送路径，不另设分批参数，以 AC-PUSH-04-2 的结果为准。推送路径上**缺少广播能力时启动失败**，不能像现有 `Broadcaster` 类型断言那样静默跳过（`logic/social.go:801-807`）。
3. console-server 的双人复核和 5 分钟间隔不变（F-ADM-50 规则 5）。

**验收标准**
- AC-PUSH-04-1：输入 5 个用户在线、u9 离线；操作 全员推送；预期 5 人收到；u9 上线 `pushSync` 收不到；错误码 —。
- AC-PUSH-04-2【后端】[P-开发机 / P-目标机]（8.1 节；L1 只要开发机结果，目标机结果归发布门槛）：输入 1 万连接在线，后台 600 条/秒单聊；操作 每 30 秒全员推送一条，共 20 次，统计所有连接的到达耗时；预期 合并后 P99 ≤ 5 秒全部送达；同窗口单聊 ack P99 ≤ 1 秒；错误码 —。
- AC-PUSH-04-3【后端】：输入 全员推送；操作 查收件箱（8.2 节测试接口）；预期 没有新增记录；错误码 —。
- AC-PUSH-04-4【后端】[P-开发机]：输入 后台 600 条/秒单聊；操作 按用户推送 1000 人 / 次，每秒 1 次，持续 5 分钟，与不推送的基线同一轮交替对照；预期 接口耗时 P99 ≤ 2 秒；单聊 ack P99 增幅 ≤ 20%（未校准）；错误码 —。
- AC-PUSH-04-5【后端】[P-开发机]：输入 u1 收件箱 1000 条，测试配置把 `IM_PUSH_SYNC_RATE` 调到不会触发限速；操作 `pushSync {limit:100}` 执行 1000 次；预期 P99 ≤ 200ms；错误码 —。
- AC-PUSH-04-6【后端】：输入 配置移除广播能力的构建（测试替身）；操作 启动；预期 启动失败；错误码 —。

#### F-PUSH-05 按房间推送（L2，P0）

**规则**：`target=room` + `roomId`，scope `push.room`；只投递当前在该房的连接；不进 `recent`，与聊天室消息的区别就在这里。属于不可丢帧（F-ROOM-08）。

**验收标准**
- AC-PUSH-05-1：输入 u1 在 `room-a`，u2 在 `room-b`；操作 向 `room-a` 推送；预期 u1 收到，`target=room`、`roomId=room-a`；u2 收不到；错误码 —。
- AC-PUSH-05-2【后端】：输入 不存在的房间；操作 推送；预期 404；错误码 4047。
- AC-PUSH-05-3【后端】：输入 向 `room-a` 推送后；操作 u3 进房看 `recent`；预期 没有这条；错误码 —。
- AC-PUSH-05-4【后端】：输入 与 AC-ROOM-08-2 相同的高峰；操作 向该房推送 5 条；预期 正常读取的连接都收到；错误码 —。
- AC-PUSH-05-5【后端】：输入 没有 `push.room` 的调用方；操作 `target=room`；预期 403；错误码 4030。

#### F-PUSH-06 `pushId` 幂等与去重（L1，P0）

**规则**：见 4.4 节。

**验收标准**
- AC-PUSH-06-1【后端】：输入 同一调用方、同一 `pushId`、同样内容；操作 提交两次；预期 第二次 200，`created=false`；u1 只收到一次，收件箱只有一条；错误码 —。
- AC-PUSH-06-2【后端】：输入 同一 `pushId`，不同 `data`；操作 提交；预期 409，不投递；错误码 4094。
- AC-PUSH-06-4【后端】：输入 全员推送已完成（`state=done`）；操作 用同一 `pushId` 重试；预期 `created=false`，不再次广播；错误码 —。
- AC-PUSH-06-5【SDK】：输入 同一 `pushId` 先经实时帧、又经 `pushSync` 到达；再反过来先 `pushSync`、后实时帧；操作 处理；预期 两种顺序下业务都只收到一次；错误码 —。
- AC-PUSH-06-6（跨服务，随 CS-044）：输入 注入 `imDropResponse`（im-server 处理完，console 收不到响应），调用得到 5040；操作 跑对账；预期 结论 `success`（im-server result=200），**不重放**，用户只收到一次；错误码 —。
- AC-PUSH-06-7【后端】：输入 全员推送，注入"幂等记录写入后、广播前进程退出"；操作 重启后用同一 `pushId` 重试；预期 `created=false`，**重新广播**，在线用户收到，帧的 `ts` 等于首次受理时间，`state` 变为 `done`；错误码 —。
- AC-PUSH-06-8【后端】：输入 `userIds` 含不存在用户（4042）；操作 失败后修正内容，用同一 `pushId` 重新提交；预期 第一次不留记录，第二次 `created=true`；错误码 4042。
- AC-PUSH-06-9【后端】：输入 `userIds=[u1,u2]` 和 `[u2,u1,u1]`，`data` 键顺序不同；操作 同一 `pushId` 提交两次；预期 判定为同内容，`created=false`；`ttlSeconds` 不同时判定为不同内容，4094；错误码 4094。
- AC-PUSH-06-10【后端】：输入 调用方已触发 4290；操作 用已成功的 `pushId` 重放；预期 200，`created=false`，不扣限速；错误码 —。
- AC-PUSH-06-11【后端】：输入 调用方 A 用 `p1` 提交成功；操作 调用方 B 用 `p1` 提交，内容相同、内容不同各一次；预期 都是 409，不投递；错误码 4094。
- AC-PUSH-06-12【后端】[IM 钟]：输入 `p1` 已提交；操作 推进 6 天重试；推进 9 天并跑清理后重试；预期 前者 `created=false`；后者 `created=true`，不与收件箱唯一键冲突；错误码 —。
- AC-PUSH-06-13（跨服务，随 CS-044）[C 钟]：输入 注入 `imLatencyMs` 超过调用超时（请求没发出），得到 5040；操作 推进时钟跑对账；预期 结论 `notExecuted`，自动用同一 `pushId` 重放，`created=true`，用户收到一次，审计有 `push.replay`；全员推送时不重走复核，也不受 5 分钟间隔约束；距首次发送超过 1 小时才跑对账时不重放，记录标为 `unknown`；错误码 —。
- AC-PUSH-06-14【后端】[IM 钟]：输入 `p1` 发给 u1、u2，推进 8 天多；`IM_PUSH_PURGE_BATCH=1`，`push_purge` 只跑一次，所以还剩一条收件箱行；操作 用 `p1` 再次提交；预期 幂等记录没有被删，返回 `created=false`，不是 5000；错误码 —。
- AC-PUSH-06-15（跨服务，随 CS-044）[C 钟]：输入 全员推送，注入 `push-exit-before-broadcast`，im-server 审计只有 intent，console 得到 5040；操作 重启后跑对账；预期 结论 `unknown`，自动用同一 `pushId` 重放，`created=false`，在线用户收到；错误码 —。
- AC-PUSH-06-16【后端】：输入 `data` 只差一个超过 2^53 的整数（9007199254740993 和 9007199254740992）；操作 同一 `pushId` 提交两次；预期 判定为不同内容；错误码 4094。
- AC-PUSH-06-17【后端】（L2）：输入 `target=room`，房间不存在；操作 失败后创建房间，用同一 `pushId` 重新提交；预期 第一次 404、不留记录，第二次 `created=true`；错误码 4047。
- AC-PUSH-06-18【SDK】：输入 去重集合上限配置为 2，先后交付 a、b，再收到 a（命中），再交付 c；操作 再收到 b、a；预期 b 不交付（还在集合里），a 再交付一次（已被挤出，命中不刷新）；默认上限下第 2001 条把最早的挤出去；有持久化时无 `pushSeq` 的帧也记入并落盘；错误码 —。

#### F-PUSH-07 多端语义（L1，P0）

**规则**：见 4.5 节。

**验收标准**
- AC-PUSH-07-1（`tianting-desktop` 还没有实现，L1 用 `platform=pc` 的 sdk-ts）：输入 u1 的 web、pc 都在线；操作 推送；预期 两个连接都收到，`pushId` 相同，两端的业务回调各触发一次；错误码 —。
- AC-PUSH-07-2（L1 用 `platform=android` 的协议客户端）：输入 u1 的 web 在线、android 离线；操作 推送，web 收到后 android 上线并 `pushSync`；预期 android 补收到；错误码 —。
- AC-PUSH-07-3【SDK】：输入 同一 SDK 实例，u1 在第 1 天登录，第 3 天退出，u2 登录，u2 有第 2 天的离线推送；操作 `pushSync`；预期 不沿用 u1 的游标（无持久化时从 0，有持久化时用 u2 自己的键）；这条推送 `replayed=true`（`T0` 是 u2 的）；u1 的暂存帧不会交给 u2；错误码 —。
- AC-PUSH-07-4【后端】：输入 u1、u2 都有推送；操作 u1 发 `pushSync`；预期 结果不含 u2 的任何一条；错误码 —。
- AC-PUSH-07-5【SDK】：输入 同一实例，`p1` 同时发给 u1、u2；操作 u1 收到后退出，u2 登录并 `pushSync`；预期 u2 也收到 `p1`，不被当作重复丢弃；错误码 —。
- AC-PUSH-07-6【SDK】：输入 没有持久化的实例，u1 收到 `p1` 后退出，u2 登录后退出，u1 再次登录；操作 连接；预期 发 `pushSync {afterSeq:0}`，`p1` 再交付一次，`replayed=true`；有持久化的实例按 u1 的键发 `afterSeq=原游标`，不重放；错误码 —。

#### F-PUSH-08 SDK `pushes` 流（L1 sdk-ts，L3 其他，P0）

**规则**
1. 流中每项为 `{pushId, type, data, ts, target, roomId?, pushSeq?, replayed}`。按 `type` 分发由业务代码负责。接口形态和交付契约见 4.8 节。
2. 业务代码抛异常不影响 SDK 状态。各端表现不同：TS 捕获后继续；Kotlin 的异常在业务自己的收集协程里，SDK 接不到，重新 `collect` 可以继续收；Swift 回调不抛错，不适用。现有 App 的解析失败处理各自实现（2.8 节）。
3. **SDK 不把 `data` 写入日志**。现有 App 把全文写进日志（2.8 节），其中可能含账号。

**验收标准**
- AC-PUSH-08-1【SDK】：输入 `T0` 之后的实时帧；`T0` 之前的 `pushSync` 结果；闪断重连后 `ts ≥ T0` 的 `pushSync` 结果；操作 处理；预期 依次 `replayed=false`、`true`、`false`；错误码 —。
- AC-PUSH-08-2【SDK】（TS）：输入 业务回调抛异常；操作 再收一条；预期 第二条正常交付；错误码 —。Kotlin 改为：收集器抛错后 SDK 状态不受影响，其他订阅者继续，重新 `collect` 可收到后续推送。
- AC-PUSH-08-3【SDK】：输入 `data` 含特征串；操作 开启调试日志，处理推送；预期 日志里搜不到特征串；错误码 —。
- AC-PUSH-08-4【SDK】：输入 连接后才注册订阅者，期间到达 3 条带 `pushSeq` 的实时帧；操作 注册；预期 这 3 条已被丢弃，下一个调度点发 `pushSync`，经补拉按 `pushSeq` 顺序交付 3 条，各一次；错误码 —。
- AC-PUSH-08-5【SDK】：输入 没有订阅者，无 `pushSeq` 队列上限配置为 3；操作 到达 4 条全员帧后注册订阅者；预期 丢掉最旧的一条，按到达顺序交付后 3 条，然后才发 `pushSync`；丢掉的那条补不回；游标不动；错误码 —。
- AC-PUSH-08-6【SDK】：输入 两个订阅者；操作 收到一条；预期 两个都收到一次；错误码 —。
- AC-PUSH-08-7【SDK】（原生）：输入 回调已执行、游标持久化之前进程退出；操作 重启并 `pushSync`；预期 这条再交付一次（至少一次语义），`replayed` 按规则计算；错误码 —。
- AC-PUSH-08-8【SDK】：输入 `hasMore=true`、`false` 两页；操作 处理；预期 第二页处理完发一次 `pushSynced {latestSeq}`，第一页之后不发；C8 重拉时只在重拉结束后发；错误码 —。
- AC-PUSH-08-9【SDK】（Swift）：输入 只注册了通用 `on` 监听器，没有 `onPush`；操作 连接，到达 3 条带 `pushSeq` 的帧；预期 不发 `pushSync`，游标不前进；注册 `onPush` 后经补拉按序交付；`pushes` 流创建后没有迭代时同样不算订阅者；错误码 —。
- AC-PUSH-08-10【SDK】（Kotlin）：输入 没有收集者；操作 到达 501 条实时帧（500 条全员帧、1 条带 `pushSeq`）；预期 读帧不被阻塞（同时到达的消息正常处理），带 `pushSeq` 的被丢弃，全员帧 500 条都在队列；开始收集后先按序收到 500 条，再经补拉收到带 `pushSeq` 的那条；错误码 —。
- AC-PUSH-08-11【SDK】：输入 没有订阅者，队列里有 1 条全员帧；操作 同一调度点注册 a、b，之后再注册 c；预期 a、b 都收到这条，c 不收；之后的新推送 a、b、c 都收到；错误码 —。
- （引用）PRD-ui AC-UI-44-1～3 继续有效；回放展示见 F-PUSH-12。

#### F-PUSH-09 推送审计与限速（L1，P0）

**规则**
1. im-server `admin_audit`：只记目标数量、`target`、`type`、`pushId`、`data` 长度和 SHA-256（BE-228 第 3 条）。现有摘要白名单不含 `userIds`、`roomId`、`pushId`（`admin/audit.go:86-130`），需要用 `auditNote` 写入；房间路径参数统一叫 `{roomId}`。
2. 限速（都可配置，都未校准）：`target=all` 默认 20 次/分钟（按调用方）；`target=user` 默认 600 次/分钟（按调用方）；`pushSync` 默认每用户 120 次/分钟（`IM_PUSH_SYNC_RATE`。每用户最多 10 个连接，每个连接最多 10 页）。超出 429 / 4290（`pushSync` 为 ack 错误 4290，SDK 按退避重试）。幂等命中的重放不计入。console-server 的 5 分钟间隔是另外一层（L-17）。
3. **`push.all` 和 `type` 白名单**（L-17，已定）：调用方配置新增可选字段 `pushTypes: string[]`。配了就只允许名单内的 `type`，所有 `target` 都适用，否则 403 / 4030，检查放在 scope 之后（F-PUSH-02 规则 7）。**`IM_PUSH_ENABLED` 开启时，持有 `push.all` 的调用方必须配非空的 `pushTypes`，否则启动失败**（在 `app` 的启动校验里做，开关关闭时不校验，升级不会因此启动失败；现有 `x5_test.go`、`moderation_test.go` 的调用方不受影响），不提供通配符；console-server 也一样，部署时按 `CON_PUSH_TEMPLATES` 配置。im-server 不区分调用方是谁，所以这条对所有调用方一视同仁。业务方接入时另外两项（独立调用方只有 `push.all`、配 `AllowedCIDRs`）是接入清单的必选项，按 AC-MIG-02-2 验收。
4. 单用户收件箱上限 1000 条（未校准，`IM_PUSH_INBOX_MAX`），超出时淘汰最旧的，序号不变。清理任务按 `expires_at` 索引分批删除，每批 1000 行（未校准），不一次占住数据库连接。

**验收标准**
- AC-PUSH-09-1【后端】：输入 一次推送；操作 查 `admin_audit`；预期 有 `pushId`、`type`、目标数量、长度和 SHA-256，没有 `data` 明文；错误码 —。
- AC-PUSH-09-2【后端】：输入 持有 `push.all` 的调用方，冻结时钟；操作 1 分钟内第 21 次全员推送；预期 429；时钟推进 1 分钟后恢复；错误码 4290。
- AC-PUSH-09-3【后端】：输入 `target=user` 限速配置为 2，冻结时钟；操作 第 3 次；预期 429；错误码 4290。
- AC-PUSH-09-4【后端】：输入 `pushSync` 限速配置为 2，冻结时钟；操作 u1 第 3 次；预期 ack 错误；u2 不受影响；错误码 4290。
- AC-PUSH-09-5【后端】：输入 `IM_PUSH_INBOX_MAX=3`；操作 给 u1 推 5 条；预期 `pushSync {afterSeq:0}` 只返回最新 3 条，`pushSeq` 是 3、4、5；错误码 —。
- AC-PUSH-09-6【后端】：输入 调用方 `pushTypes=[goal]`；操作 `target=all, type=goal`；`target=all, type=notice`；`target=user, type=notice`；预期 第一个 200；后两个 403，不投递，不占 `pushId`；错误码 4030。
- AC-PUSH-09-7【后端】：输入 `IM_PUSH_ENABLED` 开启，配置中持有 `push.all`、没有 `pushTypes`（或为空）的调用方；操作 启动；预期 启动失败，错误信息指出调用方 ID，不含密钥；错误码 —。
- AC-PUSH-09-8【后端】：输入 没有 `push.all`、也没有 `pushTypes` 的调用方；操作 `target=user`，任意 `type`；预期 200，不受限制；错误码 —。
- AC-PUSH-09-9【后端】：输入 `IM_PUSH_ENABLED` 关闭，同样的配置；操作 启动；预期 正常启动；错误码 —。

#### F-PUSH-10 推送可观测（L1，P1）

**规则**：指标：推送数（按 `target`）、投递数、`pushSync` 次数和耗时、收件箱行数。`type` 可能由调用方任意填写，不作为指标标签。

**验收标准**
- AC-PUSH-10-1【后端】：输入 各 `target` 推送一次；操作 读 `/metrics`；预期 计数正确，标签没有 `type`、`userId`；错误码 —。

#### F-PUSH-11 推送并发与崩溃恢复（L1，P0）

**规则**
1. 一次提交的收件箱写入在同一事务内；Postgres 下按 userId 字节序加锁，避免交叠批次死锁（沿用 PRD-social 8.3 节的做法）。将来按用户分片后无法同事务写入，改为逐用户幂等写入（靠 `(user_id, push_id)` 唯一键），到时补 AC。
2. L1 只跑 SQLite：单连接下顺序写错不会死锁，所以按 `sqlite_social.go:38` 的做法注入加锁顺序断言。im-server 有 Postgres 实现后（现在没有），这组用例再跑 Postgres。

**验收标准**
- AC-PUSH-11-1【后端】：输入 持续按用户推送并记录已 ack 的 `pushId`；操作 kill -9 后重启；预期 所有已 ack 的 `pushId` 都能被 `pushSync` 取到，`pushSeq` 不重复；错误码 —。
- AC-PUSH-11-2【后端】：输入 注入"提交后跳过下发"（8.2 节），u1 在线；操作 推送 p1，再推送 p2；预期 u1 没收到 p1 的实时帧，收到 p2 的帧（`pushSeq` 跳号）；`pushSync` 能取到 p1；sdk-ts 按 C2 补到 p1 后按序交付；错误码 —。
- AC-PUSH-11-3【后端】：输入 50 个 goroutine 并发给同一用户推送；操作 结束后 `pushSync`；预期 `pushSeq` 连续无重复，共 50 条；错误码 —。
- AC-PUSH-11-4【后端】：输入 同一 `pushId` 同内容并发提交 20 次；操作 结束后查询；预期 恰好一次 `created=true`，收件箱一条；错误码 —。
- AC-PUSH-11-5【后端】：输入 两组交叠的 `userIds`，顺序相反；操作 并发提交 100 轮；预期 全部成功；SQLite 下加锁顺序断言不报警（按 userId 字节序）；Postgres 实现后同一用例没有死锁错误；错误码 —。
- AC-PUSH-11-6【后端】：输入 1000 人的提交，注入第 500 人写入失败；操作 查询；预期 整批回滚，所有人收件箱无这条，没有下发，也没有幂等记录；错误码 —。

#### F-PUSH-12 通知的回放展示和未查看（L1 H5，L3 原生 App，P0）

落实 L-4、L-18（已定）。PRD-ui F-UI-44 的变更见第 11 节。只针对 `type=notice`。

**规则**
1. **列表**：`replayed` 真假都进"我 → 系统通知"列表。按 `ts` 倒序，`ts` 相同时按 `pushSeq` 倒序；最多保留 `ts` 最新的 100 条，超出的不显示，但仍参与第 3 条的计数。（一次回放最多 1000 条是收件箱上限的结果，不是列表的另一条规则。）
2. **横幅**：只有 `replayed=false` 才弹，`replayed=true` 不弹。
3. **未查看条数**：
   - `replayed=false`：计入，有没有 `pushSeq` 都一样。
   - `replayed=true`：`lastSeenSeq` 存在且 `pushSeq > lastSeenSeq` 时计入；否则不计。没有 `pushSeq` 的不计（只有 `pending` 重播才会出现）。
   - 打开系统通知页时清零；**停留在该页时收到的新通知不计入**。
4. **`lastSeenSeq`**：
   - **更新**：打开系统通知页时，以及停留在该页收到新通知时，更新为 `max(当前值, 已收到的最大 pushSeq)`。"已收到"只算本实例收到的 `type=notice`。没有 `pushSeq` 的帧不更新。
   - **建立基线**：收到 `pushSynced {latestSeq}` 时，如果还没有 `lastSeenSeq`，写入 `latestSeq`。也就是第一次使用、清缓存或退出后重新登录时，这一轮回放不计未查看，这是 L-18 接受的退化。
   - **序号回退**：收到 `pushSynced {latestSeq}` 时，如果 `lastSeenSeq > latestSeq`（服务端恢复），改为这个值。这个值已经包含本轮交付的实时帧（4.8 节），所以不会把刚看过的通知误判成未查看。恢复后回放期间的漏计是已接受风险 ⑤（4.6 节）。
5. **H5 存储**（L-18）：`localStorage` 键 `ui.noticeSeen.{userId}`，值只有这一个数字，不存内容。**退出登录时删除**，被踢或凭证失效时不删（重新登录后不重复提醒）。读写失败按"没有 `lastSeenSeq`"处理。多标签页各自写，正常更新取 `max`；序号回退是直接覆盖，优先于取 `max`（别的标签页写回大值时，它自己下一次 `pushSynced` 也会回退；竞态见 4.6 节风险 ⑧）。
6. **原生 App**（L3）：规则相同，`lastSeenSeq` 存在各端的偏好存储，键名同样是 `ui.noticeSeen.{userId}`（PRD-ui 第 246 行要求原生偏好也只有 `ui.` 键）。列表只在内存，重启后为空，与 H5 靠回放看到 7 天不一致，**接受这个不一致**（L-4，已定），页面底部注明"只显示本次打开以来收到的通知"。

**验收标准**
- AC-PUSH-12-1【前端】：输入 `replayed=true` 的 `notice`；操作 收到；预期 不弹横幅，进列表；错误码 —。
- AC-PUSH-12-2【前端】：输入 先收到 `ts=200` 的实时通知，再收到 `ts=100`、`ts=300` 的回放；操作 打开列表；预期 顺序是 300、200、100；错误码 —。
- AC-PUSH-12-3【前端】：输入 `lastSeenSeq=40`，回放 `pushSeq` 为 39、41、42；操作 看"我"页入口；预期 显示 2；打开系统通知页后清零，`lastSeenSeq` 变为 42；错误码 —。
- AC-PUSH-12-4【前端】（Playwright，真实服务端）：输入 u1 打开过系统通知页后关闭标签页（不是退出登录，`ui.noticeSeen.u1` 还在），期间有 3 条按用户通知；操作 重新打开并登录，看入口，打开系统通知页，再刷新页面；预期 登录后未查看 3，不弹横幅；打开后清零；刷新后列表仍有 3 条（回放），不弹横幅，未查看 0；错误码 —。
- AC-PUSH-12-5【前端】：输入 u1 没有 `ui.noticeSeen.u1`，回放 3 条后 `pushSynced {latestSeq:3}`；操作 看入口，再收到 `pushSeq=4` 的回放；预期 前 3 条不计，`ui.noticeSeen.u1` 写入 3；第 4 条计 1；错误码 —。
- AC-PUSH-12-6【前端】：输入 u1 已有 `ui.noticeSeen.u1`；操作 退出登录；被踢后重新登录；预期 退出后键被删除；被踢时键还在，重新登录后不重复提醒；错误码 —。
- AC-PUSH-12-7【前端】：输入 `localStorage` 读写抛异常；操作 收到回放和实时通知；预期 页面不报错，回放不计，实时通知计入；错误码 —。
- AC-PUSH-12-8【前端】：输入 `lastSeenSeq=100`，`pushSynced {latestSeq:55}`；操作 处理，再收到 `pushSeq=56` 的回放；预期 `lastSeenSeq` 改为 55；56 计入未查看；错误码 —。
- AC-PUSH-12-9（原生 App，L3）：输入 App 重启后；操作 打开系统通知页；预期 只显示本次打开以来收到的通知，底部有说明；未查看按规则 3、4 计算；错误码 —。
- AC-PUSH-12-10【前端】：输入 登录、收到通知、打开系统通知页后；操作 读 `localStorage`；预期 只有 `ui.` 开头的键（AC-UI-90-2 不用改），`ui.noticeSeen.*` 的值是纯数字，搜不到通知文本；错误码 —。
- AC-PUSH-12-11【前端】：输入 停留在系统通知页；操作 收到 `replayed=false` 的通知，离开页面看入口；预期 通知进列表，入口未查看为 0，`lastSeenSeq` 已更新；错误码 —。

---

## 7. 功能需求：迁移

> 业务服务端和三个 App 的改造不在 im-mvp 仓库。本节定义天听要提供什么、业务方要改什么、怎样算迁移完成。责任边界见 L-13。AC 标【业务方】的由业务方执行、天听复核结果；按 L-13 建议方案，天听只对试点一端执行，其余由业务方签字。行为基线是迁移前录制的旧版录屏和清单（业务方在灰度前提供）。

#### F-MIG-01 能力对照表与接入指南（L3，P0）

**规则**：`docs/` 下提供对照文档；首版内容如下，评审时补全。

| 现在（腾讯 IM + 业务接口） | 天听 |
|---|---|
| `initSDK`、`login(userID, userSig)`（2.2 节） | SDK 初始化 + 连接；`userSig` → 业务后端用 `token.issue` 签发的 token（F-AUTH-01） |
| `onUserSigExpired` | 凭证失效的 fatal：业务取新 token 后 `connect`，SDK 自动回房（F-ROOM-03 规则 6）。fx-sport、ab77 现有的自动恢复行为要保留 |
| `onKickedOffline` | `kicked` 事件 + 关闭码（F-AUTH-03） |
| `joinGroup`、`quitGroup`（10013 特判） | `joinRoom`、`leaveRoom`（幂等，不需要特判） |
| `setGroupReceiveMessageOpt` 免打扰、清未读、压角标 | 删除：房间不写会话，不计未读 |
| 进群补发历史 | `joinRoom` ack 的 `recent` |
| `createTextMessage` + `cloudCustomData` | `sendRoomText(text, ext)` |
| `createCustomMessage({type, data})` | `sendRoomCustom(customType, data, ext)` |
| `msg.nickName`、`sender`、`timestamp`、`msgID` | `fromNick`、`from`、`ts`、`msgId` |
| `onRecvNewMessage` + 群 ID 过滤 | SDK 房间消息流，不需要过滤 |
| 错误码 80001 | 4510（F-ROOM-11） |
| 全局推送群 + `onReceiveRESTCustomData` | `pushes` 流，不需要加群 |
| contentType 0 | `type=goal`，`target=all`，不传 `ttlSeconds`（默认 0）（L-17） |
| contentType 6～9 | `type=liveStatus`，`target=all`，`data` 带 `event`（`online`、`offline`、`liveStart`、`liveEnd`），因为 android、ab77 现在靠 contentType 判事件（`GlobalPushHandle.kt:129-138`、`ABPushMessage.swift:25-31`）（L-17） |
| 推送顶层 `desc`、`type`（`GlobalPushModel.kt:8-9`） | 放入 `data`；使用情况未核实，业务方确认没用时删除 |
| contentType 1 | `muted` / `unmuted` 事件（F-ROOM-09） |
| contentType 3 | `msgDeleted` 事件（F-ROOM-10） |
| contentType 4 | `type=withdraw`，`target=user`，`ttlSeconds=0`（提现待确认过期后弹出就是错的，L-2），负载去掉账号列表，客户端删除账号比对（L-15）。App 在后台时如何处理由业务负责（`withdraw_push_notifier.dart:44-50`） |
| 业务接口：`genUserSig`、`groupId`、`live/init`、礼物、时长、上报 | **继续由业务后端提供**，只把返回内容换成 token 和 `roomId`；`live/init` 的已删列表由 `joinRoom` ack 的 `deletedMsgIds` 取代（F-ROOM-10 规则 4） |

**验收标准**
- AC-MIG-01-1：输入 第 2.4 节和 2.8 节的全部类型；操作 按对照表逐项核对；预期 每一项都有去向，或写明"删除"；错误码 —。
- AC-MIG-01-2：输入 对照表中的示例代码；操作 在 sdk-ts、sdk-kotlin、sdk-swift 和 Flutter 接入（L-12）的集成测试里实际运行；预期 通过；错误码 —。
- AC-MIG-01-3：输入 对照表标"删除"的项（加群、免打扰、清未读、压角标、账号比对）；操作 在试点端的改造分支上搜索调用；预期 搜不到；错误码 —。

#### F-MIG-02 业务服务端切换需求（L4，P0）

**规则**
1. `genUserSig` 增加返回天听 token（或新接口），过渡期两者并存。
2. 赛事、主播开房时调 `POST /admin/rooms`，结束时关闭；`groupId` 接口返回 `roomId`。
3. 礼物、官方消息改调 `POST /admin/rooms/{roomId}/messages`；推送改调 `POST /admin/push`；禁言、删除分别调 `/admin/mute`、`/admin/rooms/{roomId}/messages/delete`。
4. 调用方配置 scope 和密钥（F-KEY-02），密钥不下发客户端。
5. 业务服务端的现状（N-1、N-2）要先确认，本条可能随之调整。
6. **灰度前置条件**：用 `/admin/users/import`（每批 500）全量导入用户和昵称；之后昵称变更增量同步；核对业务 userId 符合 `^[A-Za-z0-9_-]{1,64}$`（N-8），不符合的要给映射方案。否则未注册用户的推送整批失败（L-19）、礼物因 `fromUserId` 不存在而丢失、`fromNick` 为空。

**验收标准**
- AC-MIG-02-1【业务方】：输入 业务方测试环境；操作 走完开房、送礼、删除、禁言、关房（发言是 WS 操作，不进管理审计，单独验证房内可见）；预期 每个管理操作在天听侧的 `admin_audit` 都有记录（高频下发的审计方式按 L-20）；错误码 —。
- AC-MIG-02-2【业务方】：输入 业务方持有 `push.all` 的调用方（L-17，已定）；操作 查配置；用白名单外的 `type` 全员推送；预期 该调用方只有 `push.all`、配了 `AllowedCIDRs` 和 `pushTypes`；推送返回 403；错误码 4030。
- AC-MIG-02-3【业务方】：输入 客户端安装包；操作 搜索管理密钥特征；预期 搜不到；错误码 —。
- AC-MIG-02-4【业务方】：输入 全量导入后的测试环境；操作 抽 100 个业务用户，按用户推送、送礼；预期 都不返回 4042，`fromNick` 与业务昵称一致；错误码 —。
- AC-MIG-02-5【业务方】：输入 token 过渡期；操作 接口返回两种凭证，`groupId` 接口返回 `roomId`；预期 新旧版本都能取到各自所需；错误码 —。

#### F-MIG-03 灰度与回退（L4，P0）

**规则**
1. 业务配置按 App 版本和 **userId 哈希**决定走腾讯还是天听（L-11），同一用户多端一致。App 里两套实现并存一个过渡期，配置可远程切回。**默认走腾讯；配置未到不切换；切换只在冷启动生效**（三端初始化时机不同：`App.kt:71`、`FNIMSDKManager.swift:22-25`、`im_manager.dart:63-96`）。
2. **过渡期业务后端双写清单**（按 L-11 建议方案）：推送、礼物、官方消息、禁言和解禁、删除、开关房、消息上报。单个客户端只启用一路，不会收到两次。禁言必须两边都生效，否则换通道就能发言。删除的消息 ID 分属两个通道，`/live/basic/delete/message` 要能区分来源，业务方维护对应关系。
3. **首次游标**：灰度用户第一次连天听时 SDK 用 `pushStart=latest`（AC-PUSH-03-17），不回放灰度前的推送，否则双发期写进收件箱的提现会再弹一遍。回退再切回时已有游标，也不会重放。
4. 聊天室过渡期的互通方式由 L-11 决定。
5. 下线腾讯的门槛由 Frank 确认，建议：旧版本活跃占比低于约定值，并且连续 2 周没有回退。具体数值待 Frank 定。

**验收标准**
- AC-MIG-03-1【业务方】：输入 灰度比例 10%，1 万个样本 userId；操作 跑配置判定函数；预期 落入天听的比例在 8%～12%（未校准）；同一 userId 多次、多端结果相同；错误码 —。
- AC-MIG-03-2【业务方】：输入 已灰度用户；操作 配置切回腾讯，冷启动后走查进房、发言、收推送；预期 这三项都成功，天听侧没有该用户的连接；错误码 —。
- AC-MIG-03-3【业务方】：输入 双发期间，已灰度用户；操作 触发一次进球；预期 用户只看到一次提示；错误码 —。
- AC-MIG-03-4【业务方】：输入 灰度前向某用户发过提现（写入天听收件箱）；操作 把该用户灰度到天听并冷启动；预期 不弹这条提现；错误码 —。
- AC-MIG-03-5【业务方】：输入 用户 A 在腾讯通道、B 在天听通道，同一场比赛；操作 业务后端禁言 A、B，并各删一条消息；送一次礼；预期 两边都生效，礼物两边都能看到；A、B 的文本发言互不可见（L-11 ①）；错误码 —。
- AC-MIG-03-6【业务方】：输入 配置接口超时；操作 冷启动；预期 走腾讯，功能正常；运行中配置变化不立即切换；错误码 —。

#### F-MIG-04 三端行为不回归（L4，P0）

**规则**：下面这些现有行为迁移后保持不变：赛事群 / 主播群切换规则（`chat_room_notifier.dart:183-192`）；发言门槛提示（2.7 节）；昵称打码；送礼按钮只在主播房显示；敏感词提示；进球按投注 / 收藏过滤；进下级页面退房、返回重连。`settlementStautus` 的拼写错误保留，修改需要业务方另行决定，因为新旧版本要互相解析。

**验收标准**
- AC-MIG-04-1【业务方】：输入 新版本和旧版录屏；操作 走查赛事群 / 主播群切换；预期 切换条件和结果与录屏相同；错误码 —。
- AC-MIG-04-2【业务方】：输入 新版本的晒单消息；操作 用字段 `settlementStautus` 解码；预期 三端显示一致；错误码 —。
- AC-MIG-04-3【业务方】：输入 真机，无网 → 恢复网络；操作 在聊天室内切换；再切后台等连接失效后回前台；预期 都自动回房，不产生重复消息（回前台见 AC-ROOM-03-10）；错误码 —。
- AC-MIG-04-4【业务方】：输入 发言门槛不满足的用户；操作 尝试发言；预期 提示文案与录屏相同，消息不发出；错误码 —。
- AC-MIG-04-5【业务方】：输入 同一用户昵称；操作 三端显示；预期 打码结果与旧版一致（三端规则不完全一致，2.4 节，各自对比各自的旧版）；错误码 —。
- AC-MIG-04-6【业务方】：输入 主播房、赛事房；操作 查看送礼按钮；预期 只在主播房显示；错误码 —。
- AC-MIG-04-7【业务方】：输入 含敏感词的文本；操作 发送；预期 提示"消息包含敏感词，发送失败"，消息不显示；错误码 4510。
- AC-MIG-04-8【业务方】：输入 有投注和收藏、无投注和收藏的比赛各一场；操作 进球；预期 只有前者提示；错误码 —。
- AC-MIG-04-9【业务方】：输入 在聊天室；操作 进下级页面再返回；预期 退房后回房，列表由 `recent` 重新填充，不是空的；错误码 —。

---

## 8. 非功能需求

| 类别 | 要求 |
|---|---|
| 性能（未校准，L-14） | 见 8.1 节。数字都是初始目标，没有现网依据（N-3） |
| 不回归 | 现有性能基线（`docs/reports/perf-2026-11-03.md`）不下降；聊天室高峰不影响单聊、群聊（AC-ROOM-08-4、AC-ROOM-15-4、AC-PUSH-04-4） |
| 可靠性 | 按用户推送在 ack 前已提交事务（AC-PUSH-11-1）。聊天室消息尽力而为，允许丢失，不提供补拉 |
| 集群 | 首版单节点。L2 就定义 `RoomRouter`（按房投递、订阅、退订）和房间状态接口，单节点用内存实现，B6 只换实现（ARCH 第 7 行的原则）。`logic` 不能依赖 `hub`，现有 `Broadcaster` 类型断言失败时静默不发，不能沿用。要跨节点的有：广播、订阅表、在线人数、最近 50 条缓冲、`(from, clientMsgId)` 去重、房间关闭状态、已删 ID、单房限流、无 `seq` 时的顺序。**这是 B6 的新增范围，目前没有工作量数字**。B6 前提：低频定时 `pushSync` 或 pong 带 `latestSeq`（4.6 节） |
| 安全 | 所有下发、推送接口走 F-KEY 签名和 scope；`ext`、`data` 按纯文本渲染；日志不写正文和 `data` |
| 隐私 | 账号类定向信息不进广播负载（L-15）；收件箱明文保存，负载禁止放账号明文；用户注销时清除收件箱，列入将来的注销范围（目前没有注销路径）；终端用户 IP 按现有脱敏规则 |
| 兼容 | 先发能容忍新字段和未知 `cmd` 的 SDK，再开服务端能力；服务端开关 `IM_PUSH_ENABLED`、`IM_ROOM_ENABLED`，默认关闭。`IM_PUSH_ENABLED` 关闭时 `pushSync` 返回 4001，SDK 静默忽略，`POST /admin/push` 路由不注册（404），它和 `IM_V13_ADMIN_FEATURES` 互相独立；`IM_ROOM_ENABLED=1` 要求 `IM_V13_ADMIN_FEATURES=1`，否则启动失败 |

### 8.1 性能验收

| 标记 | 环境 | 能得出什么结论 |
|---|---|---|
| [P-开发机] | M4 10 核 16GB，压测客户端同机（与 perf-2026-11-03 相同）。只做同一轮内交替对照 | 回归和趋势；**不能作为发布结论** |
| [P-目标机] | 目标服务器 + 独立压测机（P-12、C-50），同样交替对照 | 发布结论 |

| AC | 指标 | 样本和窗口 | 判定 | 环境 |
|---|---|---|---|---|
| AC-PUSH-04-2 | 1 万连接全员送达 P99 ≤ 5 秒 | 每 30 秒一次，共 20 次，合并所有连接 | 合并后的 P99；同窗口单聊 ack P99 ≤ 1 秒 | 开发机可部分验收（已测过 1 万连接、600 条/秒，P99 64ms），发布结论在目标机 |
| AC-PUSH-04-4 | 1000 人按用户推送 ≤ 2 秒，单聊增幅 ≤ 20% | 每秒 1 次，5 分钟，与基线交替 | 接口 P99；单聊 ack P99 对比基线 | 开发机 |
| AC-PUSH-04-5 | `pushSync` 100 条 ≤ 200ms | 1000 次 | P99 | 开发机 |
| AC-ROOM-15-4 | 礼物下发期间单聊增幅 ≤ 20% | 5 分钟，与基线交替 | 单聊 ack P99 对比基线 | 开发机 |
| AC-ROOM-08-5 | 5000 人房 P99 ≤ 1 秒 | 10 分钟，只统计实际下发的消息 | P99；断开数 | **目标机**。下行约 10 万帧/秒，已测过的最接近场景约 1 万次投递/秒，差约 10 倍（推算，未实测），开发机上超标时分不清是服务端还是压测客户端 |

**和 P-12 的关系**：P-12 卡的是 Prometheus、Grafana 同机验收，这里的 AC 不依赖它们，但 AC-ROOM-08-5 和 AC-PUSH-04-2 的发布结论同样需要目标机，建议在 P-12 中加入，和 C-50 目标机重测合并成一次。`im-loadtest` 目前没有房间和全员场景（`scenarios.go`），要新增。L2 开发前先在开发机做扇出原型压测，用来确定 L-14 的默认上限；原型结果不是验收结果。

### 8.2 测试设施（只在测试模式注册，PRD-core 6.1 节）

| 设施 | 接口 / 手段 | 用到的 AC | 里程碑 |
|---|---|---|---|
| 关闭定时清理 | 配置开关 `IM_TEST_DISABLE_JOBS=push_purge`（现有配置未查到同类开关，新增） | AC-PUSH-03-2、03-8 | L1 |
| 手动跑清理和回收 | 现有 `POST /admin/test/jobs/{name}/run`，新任务 `push_purge`、`room_idle_close`；`push_purge` 的批大小可配置（`IM_PUSH_PURGE_BATCH`） | AC-PUSH-03-13、06-12、06-14、AC-ROOM-01-8 | L1 / L2 |
| 查收件箱 | `GET /admin/test/push-inbox/{userId}`（没有正式管理接口） | AC-PUSH-04-3、06-1、11-6 | L1 |
| 提交后跳过下发 | `POST /admin/test/faults/push-skip-delivery {pushId}`（现有 skip-delivery 只按 `clientMsgId`，`logic/faults.go:22`） | AC-PUSH-11-2 | L1 |
| 幂等记录后退出 | `POST /admin/test/faults/push-exit-before-broadcast {pushId}` | AC-PUSH-06-7、06-15 | L1 |
| 写入中途失败 | `POST /admin/test/faults/push-fail-at {index}` | AC-PUSH-11-6 | L1 |
| 房间窗口推进 | `POST /admin/test/rooms/{roomId}/flush {what: online\|messages}`：立即结束当前窗口 | AC-ROOM-05-2、05-3 | L2 |
| 敏感词 | 测试启动时给 `IM_ROOM_BLOCKED_WORDS_FILE` 指向夹具文件 | AC-ROOM-11-1、11-2、11-6 | L2 |
| 限流清零 | 现有 `limits/reset` 要同时清房间桶和推送限速 | AC-ROOM-08-1、AC-PUSH-09-2～4 | L1 / L2 |
| console 到 im-server 的延迟 | 现有 `imLatencyMs`（`tianting-console-server/internal/imclient/client.go:103-110`，AC-CON-08-10）。**它在发请求之前等待，超时后请求根本没发出**，只能造"没到 im-server"的情况 | AC-PUSH-06-13 | 随 CS-044 |
| im-server 处理完、console 收不到响应 | 新增 console 测试配置 `imDropResponse`：请求正常发出、im-server 处理完后，丢弃响应并按连接中断处理（走 `client.go` 现有的 `ErrResultUnknown` 路径） | AC-PUSH-06-6 | 随 CS-044 |
| 下行不读 | 不需要服务端注入，用不读取的测试客户端 | AC-ROOM-08-4、08-8 | L2 |
| 进程崩溃 | 现有 `im-loadtest` kill -9 场景、`durability_test.go` | AC-PUSH-11-1 | L1 |
| [IM 钟] | 现有 `clock.Settable` | 标 [IM 钟] 的 AC | L1 |

### 8.3 共享行为用例

`frames.json` 只验证解码不崩溃（`VectorsTest.kt:141-152`、`VectorTests.swift:130`）。新增两份，四套 SDK 都跑：

| 文件 | 内容 | 落地时间 |
|---|---|---|
| `protocol/vectors/push-sync.json` | 规则 C1～C12、单游标、`pushId` 去重、暂存区和无 `pushSeq` 队列、账号隔离、`replayed`、`pushSynced`、交付顺序、订阅者进退：输入事件序列和假时钟，期望发出的请求、交付序列和游标。71 个用例，已用两份实现跑通（都不进仓库）；`persistence=true` 的 5 个用例只有带持久化的 SDK 跑 | **已写好**，和 L1 一起冻结（3.3 节），sdk-ts 跑通是 L1 过关标准 |
| `protocol/vectors/room-session.json` | 回房、已见集合、切房过滤、ack 前实时帧、`deletedMsgIds` | L2，sdk-ts 先跑 |

---

## 9. 错误码（建议，定稿后写入 errors.json）

| 码 | 名称 | HTTP | 含义 | 用于 |
|---|---|---|---|---|
| 4041 | NOT_IN_ROOM | — | 已有；`action` 去掉"二期" | F-ROOM-06 |
| 4047 | ROOM_NOT_FOUND | 404 | 房间不存在；进房、发言、下发时也包括已关闭 | F-ROOM-01、02、07、10，F-PUSH-05 |
| 4094 | PUSH_ID_CONFLICT | 409 | 同一 `pushId` 的调用方或内容不同 | F-PUSH-06 |
| 4138 | ROOM_FULL | —（ws） | 房间或节点达到上限（默认值见 L-14） | F-ROOM-02 |
| 4510 | CONTENT_BLOCKED | —（ws） | 内容被拦截；编号沿用 F-ADM-60 的建议 | F-ROOM-11 |

新 scope：`room.read`、`room.write`、`room.message`、`push.room`。4047、4094、4138、4510 已与 `errors.json` 核对，当前未被占用。现有码的 `action` 要改：4001 增加"`pushSync` 由 SDK 静默忽略"；5040 增加"`pushId` 幂等的接口由对账任务重放"（PRD-admin 变更，第 11 节）；4041 去掉"二期"（L2）。`protocol/README.md` 和 PRD-admin 的"错误码只增不改"约束的是码值和含义，`action` 文案的修改不算改码。

---

## 10. 协议变更清单（定稿后落实到 protocol/）

| 变更 | 里程碑 |
|---|---|
| `PushFrame`：`pushId` 改必填；增加 `target`、`pushSeq`、`roomId`；`ts` 改必填；更新 `frames.json:28` | L1 |
| 新增命令 `pushSync`（`afterSeq`、`limit` 或 `fromLatest`）及应答 | L1 |
| `POST /admin/push` 写入 `ADMIN.md`：增加可选 `ttlSeconds`、`roomId`、响应 `{pushId, created, accepted}` | L1 |
| 表 `push_inbox`、`push_seq`、`push_idempotency` 及迁移；`expires_at` 索引 | L1 |
| 新增行为用例 `vectors/push-sync.json` | L1 |
| 兼容规则：不认识的顶层 `cmd` 也忽略 | L1 |
| 新增 scope `push.room`（随 L1 进目录，L1 时 `target=room` 由开关挡住） | L1 |
| 新增 scope `room.read`、`room.write`、`room.message` | L2 |
| 调用方配置 `callerJSON` 新增 `pushTypes`；持有 `push.all` 必须配（F-PUSH-09 规则 3）；更新 `ADMIN.md` | L1 |
| 表 `push_idempotency` 增加 `first_ts` | L1 |
| 配置：`IM_PUSH_ENABLED`、`IM_PUSH_SYNC_RATE`、`IM_PUSH_PURGE_BATCH`、`IM_PUSH_INBOX_MAX`、`IM_PUSH_ALL_RATE`（`target=all`，AC-PUSH-09-2）、`IM_PUSH_USER_RATE`（`target=user`，AC-PUSH-09-3） | L1 |
| 打通 `joinRoom`、`leaveRoom` 和 room 类型的 `send`；更新 `e2e_test.go:226` 的断言 | L2 |
| `joinRoom` ack 增加 `deletedMsgIds` | L2 |
| `Message` 增加 `fromNick`、`ext`；`body` 增加 `{customType, data}`（`MsgType` 已有 `custom`）；room 消息无 `seq`；**room 消息放开 `from`、`fromNick` 可为空字符串** | L2 |
| `send` 请求增加 `ext`（`frame.schema.json:255-270` 现在 `additionalProperties: false`）；`SendAck.seq` 改条件必填；`customType`、`ext` 只对 `room` 开放；更新 `frames.json:7` 的房间 custom 用例 | L2 |
| `recent` 中的消息与 `msg` 同结构 | L2 |
| 事件 `roomClosed {roomId, reason}`，`reason` 枚举 `closed`、`idle`；`roomOnline` 不变 | L2 |
| `msgDeleted` 的 `seqs` 改为条件必填（`convId` 为 `room:` 时不出现） | L2 |
| 管理接口 `POST /admin/rooms`、`GET` / `DELETE /admin/rooms/{roomId}`、`/admin/rooms/{roomId}/messages`（响应 `{msgId, ts, created}`）、`/messages/delete`（单次 50）、`/admin/rooms/blocked-words/reload`。不提供 `GET /admin/rooms` 列表 | L2 |
| 配置：`IM_ROOM_ENABLED`、`IM_ROOM_USER_RATE`、`IM_ROOM_PUSH_RATE`、`IM_ROOM_CLIENT_CUSTOM_TYPES`、`IM_ROOM_IDLE_TTL`、`IM_ROOM_MAX_ONLINE`、`IM_NODE_MAX_CONNS`、`IM_ROOM_BLOCKED_WORDS_FILE` | L2 |
| 新增行为用例 `vectors/room-session.json` | L2 |
| `errors.json`：4094；4001、5040 的 `action` | L1 |
| `errors.json`：4047、4138、4510；4041 的 `action` | L2 |
| `protocol/README.md:53` 的"二期"行：推送部分改为正式定义 | L1 |
| `protocol/README.md:53` 的"二期"行：聊天室部分改为正式定义 | L2 |

---

## 11. 对已冻结文档和任务的影响（只列出，本文不改）

### L1 冻结时

冻结时按 `docs/process/README.md` 走变更流程：升小版本、写变更记录、同步任务和用例。

| 文档 / 任务 | 变更内容 |
|---|---|
| PRD-admin F-ADM-50 规则 1、8 | 增加 `target=room` + `roomId`（接口 L2 开放）、可选 `ttlSeconds`；AC 对齐见 4.7 节，AC-ADM-50-1～8 的预期不变 |
| PRD-admin F-ADM-50 规则 3 | `pushId` 全局唯一，由调用方生成；"相同由 SDK 去重"明确为**同一 SDK 实例、同一账号内**，不跨设备 |
| PRD-admin 第 154 行（前端错误处理）、第 471 行、第 1143 行（5040 文案）、`errors.json:58` | 其他写操作的"不重试、不要重复提交"不变；增加例外：系统通知由对账任务重放（4.7 节对账表）。前端文案对系统通知改为"结果未知，系统会自动确认，请稍后查看发送记录"。这是 `action` 文案，不算改码 |
| PRD-admin F-ADM-50 规则 5 | 同号重放不受 5 分钟间隔约束，不写入"最近一次全员发送时间" |
| PRD-admin F-KEY-02 规则 2（第 215 行）、AC-KEY-02-2 | scope 表增加 `push.room`；参数化测试增加对应样本（房间开关关闭时按 AC-PUSH-02-8 判定） |
| PRD-admin F-KEY-02 规则 5（第 221 行） | 改为：业务方接入时可授予 `push.all`，三项必选：独立调用方（只有 `push.all`、配 `AllowedCIDRs`）、`pushTypes` 白名单、按 `type` 审计（L-17） |
| PRD-admin 调用方配置格式 | 增加 `pushTypes`；持有 `push.all` 必须配，console 调用方也要配 |
| PRD-admin 5.2 节 | 补 `push.records.read`（`console.md:499` 已指出） |
| PRD-ui F-UI-44 规则 2 | 按 `ts` 倒序（原来是到达时间）；横幅只对 `replayed=false`；最多 100 条的取法见 F-PUSH-12 规则 1 |
| PRD-ui F-UI-44 规则 3 | 未查看条数改为 F-PUSH-12 规则 3、4，可以跨会话（原来是"本次登录期间"） |
| PRD-ui F-UI-44 AC | 增加 AC-PUSH-12-1～11 的引用；AC-UI-44-1～3 不变 |
| PRD-ui D4、NF-UI-13（第 1527 行） | 增加一个例外：`localStorage` 的 `ui.noticeSeen.{userId}`，值只有一个数字，退出登录时删除（L-18）。token 和内容的约束不变 |
| PRD-ui AC-UI-90-2（第 1536 行）、`TC-UI.md:2281-2282` | **不改**：键以 `ui.` 开头，继续满足。选这个键名就是为了不动这条冻结的 AC，也不用改 ESLint 规则 |
| `tianting-web/src/lib/uiPrefs.ts` | 注释写着"退出登录也保留"，增加 `removePref` 和这个键的例外说明（实现时改） |
| PRD-core 第 7、8 节 | 推送行指向本文；新增命令 `pushSync`；兼容规则补上未知顶层 `cmd`（`PRD-core-v1.3.md:118`） |
| `protocol/README.md` 第 4 节 | 说明"错误码只增不改"不含 `action` 文案；第 53 行的"二期"改为正式定义 |
| BE-228 | 依赖改为本文；AC 增加 AC-PUSH-01-1 的下发部分、F-PUSH-01 的【后端】AC 和 F-PUSH-02、06、09、10 的 L1 AC。第 1 条增加 `ttlSeconds`、`pushTypes`、幂等表、4094。第 3 条审计增加 `pushId`、`target`，走 `auditNote`（现有白名单不含）。开关 `IM_PUSH_ENABLED` |
| BE-229 | AC 增加 F-PUSH-03、04、07、11。第 1 条表结构改为 `push_inbox`（含 `push_seq`）、`push_seq` 计数表；增加 `pushSync`、读取时过滤、收件箱上限、清理顺序。第 2 条"按批限速"改为 F-PUSH-04 规则 2。`im-loadtest` 新增全员场景（`scenarios.go` 没有，AC-PUSH-04-2 要用） |
| CS-044 | `pushId` 由 console 生成（UUID）存 `im_push_id`。增加 4.7 节对账表和自动重放（第 2 条）。第 4 条测试增加 AC-PUSH-06-6、06-13、06-15 和 `imDropResponse`。工作量 +0.5 人日（粗估） |
| CS-045 | 第 2 条：判断"最近一次全员发送时间"时排除重放；全员重放不重走复核 |
| CS-014 | 对账结论增加"只有 intent"按 `unknown` 处理。任务单现在只写两种结论；代码 `tianting-console-server/internal/app/reconcile.go` 已有 `imIntentOnly`、`partial`，实际改动比任务单小 |
| FE-208 | SDK：单游标规则 C1～C12、暂存区、无 `pushSeq` 队列、账号隔离、`pushSynced`、`pushStart`、自动 `pushSync` 且不被 `convs` 阻塞，跑通 `push-sync.json`。AC 增加 AC-PUSH-01-1 的接收部分、F-PUSH-01 的【SDK】AC（01-3、01-4、01-5）和 F-PUSH-03、06、07、08 的【SDK】AC |
| FE-353 | AC 增加 AC-PUSH-12-1～8、12-10、12-11；内容改为 F-PUSH-12，"倒序"改按 `ts` |
| FE-424、FE-434 | FE-424：系统通知的 5040 文案改为"结果未知，系统会自动确认，请稍后查看发送记录"；FE-434 的范围由 FE-208 承接，估时重估 |
| TC-ADM-50、TC-UI | 随 AC 同步；新增 AC 分配用例并更新 `traceability.md` |
| P-12 | AC-PUSH-04-2 的发布结论需要目标机，加入 P-12 |

### L2 及以后

| 文档 / 任务 | 问题 | 处理 |
|---|---|---|
| PRD-admin F-ADM-60 | 敏感词是 P2，错误码"届时分配" | 由 L-9 决定；选建议方案时提前并启用 4510，只做拦截 |
| 原生 App 的 F-UI-44 | 原生也强制，但没有页面任务 | L3 分发时给 tianting-android、tianting-ios 补任务（AC-PUSH-12-9） |
| FE-435、FE-436 | 原生 SDK 第二批内容没有列 `pushes` 流（`frontend-admin.md` FE-435）；两套 SDK 已发首版，要补单游标规则、暂存区、游标独立存储、三端签名、房间模型 | L3 分发时补任务 |
| PLAN 第 1 节、开头 | 礼物、点赞列 P1；开头写"不使用 Flutter"（`PLAN.md:8`），同一段又把替换 fx-sport 的腾讯 IM 列为目标（`PLAN.md:4`），而 fx-sport 是 Flutter，接入方式没有方案 | 随 L-10、L-12 更新 |
| ARCH-v1 2.2 节 | 投递接口没有房间和全员广播 | L2 定义 `RoomRouter` 时同步 |
| P-12 | AC-ROOM-08-5 的发布结论需要目标机 | 在 P-12 中加入 |

---

## 12. 不在本期范围

| 功能 | 说明 |
|---|---|
| 设备离线推送（APNs / FCM / 厂商） | 随原生端（PRD-core 第 7 节）。ab77 现有 UMPush 不走腾讯（2.8 节），迁移时不受影响 |
| 定时推送、灰度推送 | PLAN 列为 P1 |
| 聊天室点赞 | 现有聊天室没有（2.5 节） |
| 房间级禁言、房间角色、全员禁言 | P1；现有角色和入口都在业务服务端（F-ROOM-09） |
| 礼物业务（订单、余额、动画） | 业务服务端和客户端负责；天听只承载消息 |
| 观看时长、消息上报 | 业务接口，保持不变 |
| 聊天室历史消息和 `sync` | 现有 App 不使用（2.3 节）；L-5 |
| 成员列表、进出广播 | 现有 App 不使用（2.6 节） |
| 管理后台房间页面 | P1，定稿后另列；首版用接口 |
| 视频流 | 与 IM 无关 |

---

## 13. 拍板清单

> 共 21 项：13.1 节 7 项**已定**，13.2 节 14 项**未定**，相关规则暂按建议起草。纯技术选择（`pushId` 全局唯一、幂等表、游标规则等）已在第 4 节定下，不在这里。"优先级"分两组：只想解除第二批阻塞，定第一组即可；L-9、L-11、L-12 不挡 L1。

### 13.1 解除第二批阻塞需要的（L1，已定）

> **2026-11-05 Frank 拍板：这 7 项全部按建议方案。** "备选"列只作记录。

| # | 问题 | 结论 | 没选的备选 | 影响 |
|---|---|---|---|---|
| **L-1** | 全员推送要不要离线补收？ | **不补收**，只投在线连接，与 F-ADM-50 规则 3 的输入一致；现有 App 也没有补收（2.8 节） | ② 全员收件箱读扩散：一份数据，所有用户按全局序号补拉，保留 7 天 | 选 ① 最简单，公告类通知离线用户看不到，需要业务方确认够用（REVIEW-admin D-ADM-07 留的问题）；广播中途崩溃且不重试时部分用户收不到（4.6 节）。选 ② 多一条序号和一张表，AC-ADM-50-3 要改 |
| **L-2** | 按用户推送是否都补收？ | **调用方按条指定** `ttlSeconds`：`target=user` 不传为 7 天，0 表示仅在线；对照表逐类写明（提现为 0） | ② 一律 7 天 | ② 更简单，但有时效的通知（如提现待确认）上线后才弹就是错的；① 需要业务方按类型决定，多了混合 TTL 造成的序号空洞，已由游标规则 C7 处理 |
| **L-3** | 多端补收用谁的游标？ | **客户端游标**，每台设备各自补收（4.5 节）；游标独立于本地库存储（4.9 节），退出、被踢、Schema 升级都不回放；只有卸载、清数据、H5 才回放 | ② 服务端按设备记录；③ 用户只收一次 | ① 与消息模型一致，服务端无状态，但换设备或清数据会回放保留期内的内容。② 需要设备身份；③ 第二台设备会漏 |
| **L-4** | 旧推送（`replayed=true`）在"我 → 系统通知"怎么展示？ | **都进列表，按 `ts` 排序，不弹横幅**；`pushSeq` 高于本机已看过的序号时计入未查看（需要 L-18 的存储），其余不计。`replayed=false` 的正常弹横幅、计未查看 | ② `replayed=true` 一律不计未查看（v0.1 的方案）；③ 回放也弹横幅、计未查看 | ① 离线 6 天的用户能看到有未读，刷新页面不会重复提醒。② 最简单，但离线用户没有任何提醒，AC-ADM-50-2 的"收到"对用户几乎没有意义。③ 每次刷新一串横幅。原生端列表不持久化，重启后为空，H5 靠回放能看到 7 天，两端不一致，接受这个不一致并在页面注明。规则和 AC 见 F-PUSH-12 |
| **L-16** | 协议和聊天室是否一起冻结？ | **分开**：第 4、6 节先冻结（L1），第 5、7 节晚一步；`target=room` 帧格式在 L1 冻结但接口返回错误；L1 冻结范围包括三端 `pushes` 签名、交付契约和 `push-sync.json`。具体条目见 3.3 节（F-PUSH-05 依赖第 5 节，随 L2） | ② 整份一起冻结 | ① 冻结后解除第二批系统通知的阻塞（AC 归属见 3.2 节）；缺点是两轮评审。② 一次评审，但第二批要继续等 |
| **L-17** | 业务调用方能否持有 `push.all`？现有进球、主播状态是广播，PRD-admin 规定默认不给（只影响业务方接入，不影响 console 的系统通知） | **接入时授予，三项必选**：独立调用方（只有 `push.all`，配 `AllowedCIDRs`）、`type` 白名单（调用方配置新增字段）、按 `type` 审计。im-server 限速 20 次/分钟是**未校准**的占位，等 N-3 再定；业务方自行处理 4290 | ② 不给；业务方把事件发到一个所有人都进的"主题房间"；③ 单独做主题订阅（P1） | ① 与现有用法最接近，但绕开人工复核，密钥能向所有人发白名单内的 `type`；多场比赛同时进球可能超限，`ttlSeconds=0` 的推送遇到 4290 就没了。② 每个连接只能进一个房，和观看聊天室冲突，要放宽 F-ROOM-02 规则 2。③ 最干净，工作量最大。规则见 F-PUSH-09 规则 3，业务方验收见 AC-MIG-02-2 |
| **L-18** | H5 能否用 `localStorage` 存一个数字？（新增）PRD-ui D4 规定 H5 只用内存 | **开一个小例外**：只存 `lastSeenSeq`，按 userId 分键，不存内容，退出登录时删除。键名和规则见 F-PUSH-12 规则 5 | ② 不存，L-4 的未查看在 H5 上失效，刷新后不计；③ 存游标，H5 也不回放 | ① 改冻结的 D4，但没有内容和凭证落盘；清缓存后退回 ②。③ H5 刷新后列表就空了，和 H5 靠回放看历史矛盾 |

### 13.2 整体迁移需要的

| # | 问题 | 建议方案 | 备选 | 影响 |
|---|---|---|---|---|
| **L-5** | 聊天室消息要持久化吗？ | **不持久**，只保留内存缓冲（PLAN 2.1 节）；**问法务后定**，建议列为上线门槛（是否必须由法务确认） | ② 按天保留到数据库，用于审核和取证 | ① 省资源，但删除无法取证。注意三者要一起问法务：不持久化、日志不写正文、下发消息进审计（L-20）。② 有合规价值，写入量大，还要决定是否走隔离原文 |
| **L-6** | 游客能进房只读吗？（N-5） | 二期**只允许登录用户**；业务方查明现状后再定 | ② 业务后端给游客签匿名 token，只读 | 如果现网游客能看聊天室，① 会造成回退；② 增加匿名用户、连接数和人数口径的问题 |
| **L-7** | 在线人数的口径？ | **不同 userId 数**，最多每 5 秒广播一次（未校准） | ② 连接数；③ 业务方可配置放大系数 | ① 跨节点去重有成本（B6）；② 简单但虚高。③ 是运营决定，不建议放进 IM |
| **L-8** | 房间怎么创建和回收？ | **管理接口显式创建，并自动回收**：没有连接超过 24 小时（未校准）自动关闭 | ② 首次 `joinRoom` 自动创建 | ① 防止任意 `roomId` 占内存，业务后端要在开赛、开播时调用；24 小时要和业务确认，比赛延期可能被回收，业务可再次创建。② 接入更省事，但要限流 |
| **L-9** | 敏感词什么时候做？现在靠腾讯 80001（2.7 节） | **把聊天室文本的拦截提到 L2、P0**（F-ROOM-11，只拦截、只查客户端文本）；约 4.5 人日（第 14 节，粗估）。**词库内容和负责人要业务方提供** | ② 业务后端做回调审核（ARCH S14）；③ 保持 P2，接受过渡期没有拦截 | 这是上线门槛。① 不含词库内容；晒单 `data`、下发消息不查，是否要扩大也请一起定。② 增加延迟和业务方工作量；③ 有内容合规风险，不建议 |
| **L-10** | 礼物等自定义消息是否提到 P0？（PLAN 列 P1） | **提到 P0**，只做通用自定义消息和服务端下发（F-ROOM-07） | ② 保持 P1，L2 只有文本 | 三端都依赖（2.4 节），不做就不能迁移；① 不含礼物业务逻辑 |
| **L-11** | 新旧版本过渡期聊天室怎么办？ | **按版本和 userId 哈希灰度，过渡期新旧用户在两个通道、互相看不到发言；业务后端双写**（F-MIG-03 规则 2 的清单）；灰度前全量导入用户；设门槛后下线腾讯 | ② 服务端双向桥接（需要腾讯回调，N-4 未查到）；③ 按房间整体切换，并强制升级 | ① **成本不再是"最低"**：要双写礼物、官方、禁言、删除、开关房、上报，维护删除 ID 对应关系，全量导入和昵称同步，短期内聊天室人气分流。② 体验最好，但工作量和消息循环风险都高；③ 切换干净，但强制升级对业务打扰大 |
| **L-12** | fx-sport 是 Flutter，PLAN 只有 TS、Kotlin、Swift 三套 SDK，怎么接？ | **纯 Dart 的 `sdk-dart`，只做"仅直播"子集**（连接、房间、推送、游标），跑同一套行为用例；同时给 Kotlin、Swift 增加仅直播模式 | ② Flutter 插件包一层 sdk-kotlin、sdk-swift；③ fx-sport 不迁移 | ① 前端评审粗估 8～12 人日含测试（低置信），要校验 vectors 之外的行为用例；覆盖 Flutter Web（`fx-sport/app/web` 存在，是否发布未核实）。② 少写协议代码，但要维护平台通道、两端事件对齐，还要先解决 SDK 分发（L-13），覆盖不到 Web。③ 无法完全下线腾讯。**PLAN 没有这一项，当前工期未包含** |
| **L-13** | 迁移的责任边界？ | 天听交付服务端、SDK（**含分发：podspec、Maven、pub**）、指南、用户导入工具，并**配合一端试点**（建议 android，它的封装最集中：`ChatRoomHelper.kt`；要先核实它的远程版本目录，N-7）；三个 App 和业务服务端的改造由各自负责人排期 | ② 本 PRD 负责到三端全部上线 | ① 范围清楚，但 F-MIG-02～04 的 AC 由业务方执行，L4 过关靠试点一端的 AC 加业务方签字；ab77 的 `ABChatModule` 是本地 Pod，需要 podspec。② 工作量大幅增加，且需要业务仓库的改动权限 |
| **L-14** | 容量目标和验收环境？（N-3 未查到现网数据） | **设可配置的房间和节点上限，满员 4138**，默认值由 L2 前的扇出原型压测确定；5000 人只是暂定值；发布结论放在 P-12 的目标机，和 C-50 合并 | ② 不设硬上限（v0.1 的方案）；③ 按大房间设计（超大房采样下发） | 不只是数字，还要定目标机和独立压测机（要花钱或要机器）。② 超载就是 OOM 或全员慢连接，后端评审不建议。腾讯现网如果有大型赛事，5000 可能不够，必须先拿数据 |
| **L-15** | 提现推送把账号列表广播给所有客户端（2.8 节），迁移时改吗？ | **必须改成按用户推送**，负载不含其他账号，客户端不再比对账号 | ② 保持广播，协议不管 | ① 业务后端要改（账号映射 userId）；② 把账号发给无关用户，是隐私问题，不建议 |
| **L-19** | 业务调用方推送时名单里有未注册用户怎么办？（新增） | **保持整批 4042**，把灰度前全量导入作为前置条件（F-MIG-02 规则 6） | ② 调用方可选 `skipUnknown=true`，部分成功并在 `details` 返回跳过名单；③ `target=user` 对未注册用户自动建档 | ① 严格，AC-ADM-50-6 不变，但导入有遗漏时双发会大面积失败。② 多一个参数，业务不感知遗漏的风险在调用方。③ 产生没有昵称的用户 |
| **L-20** | 高频下发接口的审计怎么做？（新增）每次写 2 条、保留 1 年，挤占单聊发送事务 | **`room.message` 聚合审计**：按调用方 + 房间每分钟一条（条数、字节数）；单条不留痕 | ② 采样；③ 全量（现状），接受性能影响；④ 批量接口，一次多条一条审计 | 改变"每个管理操作都有审计"的默认，需要合规确认（和 L-5 一起问法务）。③ 高峰可能影响单聊（真实速率未知，N-2、N-3） |
| **L-21** | 房内文本长度是否单设上限？（新增）现在沿用 5000 码点，20 条/秒约 300KB/s（估算） | **单设上限，默认 500 码点（未校准，可配）**，查明 N-8 后定 | ② 沿用 5000 | ① 如果现网允许更长的文本，会造成回退，所以要先查 N-8；② 带宽和 `recent` 大小不可控（`recent` 最坏约 1MB，估算） |

---

## 14. 工作量粗估（人日，评审后分发时重估）

| 里程碑 | 服务端 | SDK / 前端 | 测试 | 合计 |
|---|---|---|---|---|
| L1 push 协议（含原 BE-228、229、FE-434 的部分；幂等表、游标规则、行为用例、H5 回放展示；不含 CS-044、CS-045） | 8 | 7 | 4 | 19 |
| L2 聊天室（含双队列改造、扇出原型压测、`RoomRouter`、下发接口） | 14 | 6 | 6 | 26 |
| L3 sdk-kotlin、sdk-swift 补充（含行为用例、仅直播模式、分发配置） | — | 9 | 3 | 12 |
| L3 Flutter（L-12） | — | 前端评审粗估 8～12（含测试，只做仅直播子集，低置信） | 含左 | 8～12 |
| L4 迁移 | 业务方 | 业务方 | 业务方 | 待 L-13 |
| L-9 敏感词最小实现 | 3 | 0.5 | 1 | 4.5（BE-242 原估 3 人日加词库入口，词库内容不含） |
| B6 集群的房间和全员部分 | 待估 | — | 待估 | 待 B6 设计，目前没有数字 |

估算依据是同类任务（BE-228、229 各 2.5 人日）的量级和评审意见，不是详细设计结果。

---

## 15. 变更记录

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v0.1 | 2026-11-05 | 初稿：现状调研、push 协议建议方案、聊天室、推送、迁移需求和待拍板清单 | 产品（AI） |
| v0.2 | 2026-11-05 | 按 REVIEW-live-v0.1 三视角 89 条问题修改，处理结果见文首修订记录 | 产品（AI） |
| v0.3 | 2026-11-05 | 13.1 节 7 项已定，落实到正文；按 RECHECK-live-v0.2 的 21 条修改 L1 部分，新增 F-PUSH-12、3.3 节冻结条目，处理结果见文首修订记录 | 产品（AI） |
| v0.4 | 2026-11-05 | 按 RECHECK-live-v0.3 的 13 条修改 L1 部分：改成单游标、无订阅者不补拉的简化设计；写好 `protocol/vectors/push-sync.json` 并用参考模型跑通；处理结果见文首修订记录 | 产品（AI） |
| v0.5 | 2026-11-05 | 按 RECHECK-live-v0.4 的 15 条修改 L1 部分：C9 错误分类、5 秒计时、C8 清暂存、回调内退订等留白写死；`push-sync.json` 扩到 71 个用例；处理结果见文首修订记录 | 产品（AI） |
