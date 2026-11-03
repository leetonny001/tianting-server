# 后端任务单：社交关系与群聊补缺（M3 / M4）

| 项 | 内容 |
|---|---|
| 依据 | [PRD-social v1.0](../prd/PRD-social-v1.0.md)（已冻结，204 条 AC）、[PRD-core v1.3](../prd/PRD-core-v1.3.md) 第 11～13 节、[ARCH-v1](../arch/ARCH-v1.md)、[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[管理接口](../../protocol/ADMIN.md) |
| 负责 | 后端（AI） |
| 仓库 | im-server（GitHub 仓库 `tianting-server`，本地 `im-mvp/server/`）；协议文档在同仓库 `protocol/` |
| 起点 | M2 代码：B0～B5 全部完成（[backend.md](backend.md)），迁移到 0003，`store/storetest` 契约测试、`delivery.Router`、`cache.Invalidator`、`ratelimit.Limiter` 接口已就位 |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（后端 e2e、存储契约测试或 Postgres 并发测试），用例名带 TC-SOC 编号；CI 中 `mode=open`、`mode=friends` 各跑一遍；`go test -race ./...` 全过；默认配置下 `server/e2e` 现有用例不改代码全过（AC-SOC-08-13）；协议 Schema / vectors / ERRORS.md / ADMIN.md 同步更新；标"B6 前置"的测试代码随功能写好、在 SQLite 上运行，B6（BE-070）上线前在 Postgres 上全部通过 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由 `tools/trace.py` 从这一行生成。一条 AC 可以出现在多个任务里（功能任务 + 专项测试任务）。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-100、BE-102、BE-103、BE-105～BE-115（M3 功能） | ✅ 2026-11-03 | 迁移 `0006_social.sql`（勘误 E-26）与回填、`im-server repair counters`、发布手册 [release-m3.md](../process/release-m3.md)；社交配置（默认值 / 校验 / PATCH / `IM_SOCIAL_DEFAULTS` / 缓存失效 + 60 秒 TTL / `socialSettingsChanged`）；限流器 `Reconfigure` / 退还 / 按用户清空（突发值见勘误 E-24）；测试设施 drop-events / purge / seed / limits/reset；12 个社交命令、两个事件、12 个错误码、Schema 与 vectors；好友申请判定表 9 行、处理 / 删除 / 备注、同步轮（游标 HMAC、排序键归并、`restart`）、红点、清理任务（每小时）、拉黑、私聊判定顺序（预扣 + 退还）、陌生人限额（`SaveMessage` 事务内）、搜索 / 隐私、`users` 可见范围、`myGroups`；管理接口（配置、导入 / 删除 / 查询好友、官方账号，X-5 之后形态，勘误 E-25）。测试：e2e（`server/e2e/social_*_test.go`，用例名带 TC-SOC 编号），存储契约 `FriendApplyTable`、`SocialVerConcurrent`、`RepairCounters`、0006 迁移回填；`go test -race ./...` 全过，现有 e2e 不改断言全过（AC-SOC-08-13，只给 X-5 路由遍历测试补了新路由的样例） |
| BE-101（存储基座） | ✅ 2026-11-03 | 按字节序锁用户、每事务每用户只分配一个 ver；加锁顺序断言用测试注入的回调实现（`onLockOrderViolation`，等价于 `locktrace` 构建标签，不需要单独构建），`TestSocialLockOrder` 跑全部社交写操作并含变异测试。Postgres 的 `lock_timeout` / `store_deadlock_total` 随 B6（BE-070）实现 |
| BE-100 第 3 条（测试模式启动时抽样校验计数器） | ✅ 2026-11-03 | `CounterDrift`：每种计数器随机抽 100 行，不一致记 ERROR 日志并提示 `repair counters` |
| BE-104（双节点测试环境） | ✅ 2026-11-03 | 测试模式的 `cache.HTTPLoopback`（`IM_TEST_PEERS` / `App.SetTestPeers`、`/admin/test/invalidate`、`IM_TEST_INVALIDATION=off`，勘误 E-27）；e2e 两个进程共享一个 SQLite 文件：广播 1 秒内生效、关闭广播时 59 秒仍旧 / 61 秒后新（AC-SOC-01-5） |
| BE-116（e2e 与 CI 矩阵） | ✅ 2026-11-03 | CI 增加好友制一轮：`IM_E2E_SOCIAL_MODE=friends` 以 `mode=friends` 启动并给登录过的用户预置好友关系，现有 e2e 不改代码全过（社交用例自己设置模式，这一轮跳过） |
| BE-117（Postgres 并发测试代码，B6 前置） | ✅ 2026-11-03（SQLite） | `store/storetest` 并发组 7 项：`FriendMutualApplyRace`、`FriendLimitRace`、`FriendMutualAcceptNoDeadlock`、`StrangerLimitRace`、`MemberCountRace`、`AdminImportVsApplyNoDeadlock`、`SocialVerConcurrent`；BE-070 接入 Postgres 时这 7 项必须通过 |
| BE-118（5.4 节逐行、social.json） | ✅ 2026-11-03 | `TestTCSOC0614VerPerOperation` 24 行（5.4 节 19 行全覆盖）：谁的 ver 变化、同一操作的行共享一个 ver、谁收到事件、谁零帧；`protocol/vectors/social.json` 11 个用例 + 16 条"命令 + 错误码"文案，期望由参考客户端生成并校验 |
| BE-119（同步模型测试） | ✅ 2026-11-03 | `server/e2e/socialmodel` 参考客户端（10.2 节规则）+ 随机生成器（两人各两端，14 种操作含重连 / 刷新 / 丢事件 / 推进时钟 / 清理，limit 随机），失败时贪心收缩成最小序列并输出种子；CI 单独一步跑 1000 个序列（约 10 秒），普通 e2e 和 race 跑 100 个；变异测试确认能发现缺陷。搜索耗时对比 `TestTCSOC0910SearchTiming`（AC-SOC-09-9） |
| BE-120（防枚举、日志扫描） | ✅ 2026-11-03 | `social_enum_test.go`：11.2 节逐路径同构比较、每日确认上限 100 + 100、四个特征串扫描服务端日志和审计全部字段（含变异测试）；零帧断言在 BE-118 逐行表里 |
| BE-121（性能） | ✅ 2026-11-03 | `im-loadtest` 新场景 `social-full`、`c2c-friends`；默认配置下 TC-PERF-00-01～07 重跑，报告 [perf-social-2026-11-03.md](../reports/perf-social-2026-11-03.md) |
| BE-122（M3，解散群） | ✅ 2026-11-03 | `Store.DissolveGroup`（删除全部成员、写 `dissolved_at`、消息保留）、`DELETE /admin/groups/{groupId}`（scope `group.dissolve`，审计）、`convRemoved.reason`（管理接口移出 `removed`、解散 `dissolved`）；规则 3 的查询全部按"不存在"处理，群 ID 不复用（4090）；契约测试 `GroupDissolveNoReuse`、e2e（AC-SOC-26-2、26-4、26-5、26-6） |
| BE-130 ～ BE-138（M4） | ⬜ 未开始 | 开工条件：M3 后端全部完成 |
| BE-113 第 3 条（`member_count`） | ✅ 提前完成 | 2026-11-03 为 H5 群标题提前做：迁移 0005、加 / 减成员同事务维护、`convs` 群会话和 `GET /admin/groups/{id}` 带 `memberCount`（契约测试 `MemberCountMaintained`、e2e）。第 1、2 条（`users` 可见范围、`myGroups`）仍待社交关系表 |

## 总览

| 阶段 | 任务（人日） | 人日 | 说明 |
|---|---|---|---|
| S0 基座（M3） | BE-100（1.5）、BE-101（1.5）、BE-102（1）、BE-103（1）、BE-104（1.5）、BE-105（1） | 7.5 | 0004 迁移与回填、计数器修复与发布手册、存储基座（用户锁 / ver / 加锁顺序断言）、社交配置与限流器扩展、测试设施、双节点测试环境、协议与文档 |
| S1 好友与同步（M3） | BE-106（1.5）、BE-107（1）、BE-108（1.5）、BE-109（1） | 5 | 好友申请判定表、处理 / 删除 / 备注、`socialSync` 同步轮、红点与清理任务 |
| S2 拉黑与私聊权限（M3） | BE-110（1）、BE-111（1.5） | 2.5 | 拉黑、发送判定顺序、陌生人限额、两个新对象配额 |
| S3 搜索、资料、通讯录（M3） | BE-112（1）、BE-113（1） | 2 | `userSearch` / `setPrivacy`、`users` 可见范围、`myGroups`、`memberCount` |
| S4 管理接口（M3） | BE-114（1.5）、BE-115（1） | 2.5 | 配置 / 好友导入删除查询的路由与审计接入、官方账号 |
| S5 e2e 与并发测试代码（M3） | BE-116（1.5）、BE-117（1） | 2.5 | 两种模式的 e2e 矩阵、M3 部分的 Postgres 并发测试 |
| S6 专项测试（M3，计入 PRD 15.1 节"测试"） | BE-118（1）、BE-119（1.5）、BE-120（1）、BE-121（1） | 4.5 | 同步模型测试、防枚举安全用例与日志扫描、性能与不回归压测对比、5.4 节逐行 ver 断言 |
| 解散群（M3，计入 PRD-admin） | BE-122（1.5） | 1.5 | `DissolveGroup`、`DELETE /admin/groups/{id}`、`convRemoved.reason` |
| G0 群基座（M4） | BE-130（2） | 2 | 0005 迁移、`ug-` 预检、群锁与提示消息同事务、契约测试 |
| G1 群命令（M4） | BE-131（1）、BE-132（1.5）、BE-133（1）、BE-134（1.5）、BE-135（1）、BE-136（1） | 7 | 角色与 `groupInfo`、建群与提示消息协议、邀请 / 移出、退群 / 转让 / 管理员 / 解散、改资料 / 成员分页、托管群交接 |
| G2 e2e 与专项测试（M4） | BE-137（1.5）、BE-138（2） | 3.5 | M4 e2e 与 Postgres 并发测试代码、权限矩阵遍历与群主变化全场景 |

**人日合计与 PRD 第 15 节对照**

| 里程碑 | 本任务单 | PRD 第 15 节 | 差异说明 |
|---|---|---|---|
| M3 服务端 | 22（BE-100～BE-117） | 21.5 | +0.5：官方账号单列 1 人日（BE-115），PRD 计在"管理接口与审计 2"里 |
| M3 测试 | 4.5（BE-118～BE-121） | 6 | 其余 1.5 为 TC-SOC 用例编写（`docs/testcases/TC-SOC.md`，本次需求分发已完成） |
| M3 解散群 | 1.5（BE-122） | 0（计入 PRD-admin G-ADM-11） | PRD-social 15.1 节未单列；PRD-admin 任务单引用本任务，不重复建 |
| M4 服务端 | 10.5（BE-130～BE-137） | 10 | +0.5：托管群交接按最小粒度 1 人日（PRD 0.5） |
| M4 测试 | 2（BE-138） | 3 | 其余 1 为 TC-SOC M4 部分用例编写（已完成） |
| **后端合计** | **M3 26.5（不含 BE-122）/ M4 12.5** | — | sdk-ts 与原生 SDK 见 [frontend-social.md](frontend-social.md)；H5 页面、文档修订不在本任务单 |

**依赖顺序**：PRD-admin F-KEY-01～05（X-5）→ BE-100 → BE-101 → BE-102、BE-103 → BE-105（协议先定，SDK 可并行开工）→ BE-106 → BE-107 → BE-108 → BE-109 → BE-110 → BE-111 → BE-112、BE-113 → BE-114、BE-115 → BE-122 → BE-116、BE-117 → BE-118～BE-121；BE-104 只依赖 BE-102，可以随时插入。M4：BE-130 → BE-131 → BE-132 → BE-133 → BE-134 → BE-135 → BE-136 → BE-137 → BE-138。

**与其他 PRD 的交接（写成依赖，不在本任务单重复建任务）**

| 依赖 | 用在 | 说明 |
|---|---|---|
| PRD-admin F-KEY-01（X-5）独立内网管理端口 | BE-114、BE-115、BE-122、BE-136 | 社交管理接口只注册在 `IM_ADMIN_ADDR`；X-5 之前只在 `IM_TEST_MODE=1` 下注册（F-SOC-12 规则 8） |
| PRD-admin F-KEY-02（X-5）调用方密钥与 scope | BE-114、BE-115、BE-136 | 新增 scope `social.settings`、`social.friends`；scope 不足 403 / 4030 |
| PRD-admin F-KEY-04 im-server 侧审计 | BE-114、BE-115、BE-136 | `admin_audit` 表、`AppendAdminAudit` 由 F-KEY-04 建，字段以 F-KEY-04 为准；本任务单只调用 |
| PRD-admin G-ADM-11 / F-ADM-24 解散群 | BE-122 | 同一实现：im-server 部分由 BE-122 交付，PRD-admin 任务单引用 |
| PRD-admin F-ADM-25 指定 / 收回群主 | BE-136 | console 侧调用 `PATCH /admin/groups/{id} {owner}` |
| PRD-admin 第二批封禁（X-8） | BE-112 | F-SOC-09 规则 7：封禁用户搜索返回 null、其申请查询时过滤，随封禁功能一起实现，本任务单只预留查询过滤点 |
| BE-070 Postgres 实现（B6） | BE-117、BE-137 | 标"B6 前置"的并发测试在 Postgres 上通过是 BE-070 的验收条件 |

---

## S0 基座（M3）

### BE-100 0004 迁移、回填、计数器修复与发布手册
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-060（版本化迁移）、BE-063（备份恢复）、PRD-admin F-KEY-04（若 X-5 的迁移先合入，本迁移编号顺延；`admin_audit` 表由 F-KEY-04 的迁移建）
- AC：AC-SOC-11-5
- 内容：
  1. `store/migrations/sqlite/0004_social.sql`（若编号已被 X-5 占用则顺延，文件头注释写"PRD-social 0004"）：新表 `app_settings`、`user_social`、`friends`、`friend_requests`、`blocks`（字段、主键、索引按 PRD 8.1 节，`peer_id` 不加外键）；`user_conv` 加 `sent_count INTEGER NOT NULL DEFAULT 0`；`users` 加 `official SMALLINT NOT NULL DEFAULT 0`；`chat_groups` 加 `member_count INTEGER NOT NULL DEFAULT 0`、`dissolved_at BIGINT NOT NULL DEFAULT 0`（**从 0005 提前**：M3 的 BE-122 解散群需要，见 PRD 问题清单）；`group_members` 加索引 `(user_id)`。
  2. 同一迁移内回填：`sent_count` 按 `messages`（单聊，按 `from_uid` 分组计数）；`member_count` 按 `group_members` 计数。只用 ARCH 3.3 的通用 SQL（`ON CONFLICT`、`RETURNING`、行值比较），不用触发器、不用自增主键。
  3. `im-server repair counters`：按基础表重算 `user_social.friend_count / block_count`、`chat_groups.member_count`、`user_conv.sent_count`；幂等；逐行持锁、可在线执行；输出修复条数。测试模式启动时抽样 100 行校验，不一致记 ERROR 日志。
  4. 发布 / 回滚手册 `docs/process/release-m3.md`：停机升级（停节点 → 备份 → `migrate up` → 启动），回滚 = 恢复备份，写明数据损失范围；演示环境跑 24 小时再接真实业务方（PRD 8.5 节）。
  5. 测试：迁移测试（在含 0003 数据的库上执行 0004：3 个群成员数 2 / 10 / 2000，单聊 `sent_count` 正确）；`RepairCounters` 契约测试（人为改坏计数 → 修复 → 再执行一次无变化）。

### BE-101 社交存储基座：用户锁、ver 分配、加锁顺序断言
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-100、BE-061（存储契约测试）
- AC：AC-SOC-06-9
- 内容：
  1. Store 内部原语：`lockUsers(tx, ids...)`（按 userId 字节序；行不存在先 `INSERT … ON CONFLICT DO NOTHING` 再 `SELECT … FOR UPDATE`，SQLite 下 no-op；**不为不存在的用户建行**，由调用方传入"只锁存在的用户"）、`lockGroups(tx, ids...)`；`allocVer(tx, uid)`（`UPDATE user_social SET ver = ver + 1 … RETURNING ver`，同一事务对同一用户只允许调用一次，重复调用 panic）。
  2. 加锁顺序断言（测试构建 tag `locktrace`）：每个加锁点调用 `lock(kind, id)`，记录事务内序列，断言符合 8.3 节全局顺序（群 → 用户按字节序 → `conv_seq`）；变异测试验证（故意反序应失败）。
  3. Store 接口新增 M3 方法签名（PRD 8.6 节），实现分散在 BE-106～BE-115；本任务先给出 `SocialVerPerOp` 契约测试骨架和 `SocialVerConcurrent`（同一用户 8 个并发事务 × 1000 次，ver 连续无重复；SQLite 上运行，B6 前置）。
  4. Postgres 约束预留：`lock_timeout = 2s` 超时映射 5030；死锁检测记 `store_deadlock_total` 指标（8.3 节 L9）。

### BE-102 社交配置、缓存失效与限流器扩展
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-101、BE-062（`cache.Invalidator`）、BE-023（令牌桶）
- AC：AC-SOC-01-1、AC-SOC-01-3、AC-SOC-01-6、AC-SOC-01-9
- 内容：
  1. `internal/social/settings.go`：5.1 节全部参数（M4 参数先定义、M4 生效），默认值、范围校验（超范围 / 未知字段 / `mode` 非法 → 4000，整次不生效）；`SocialSettings / SetSocialSettings`（`app_settings` key=`social`，PATCH 只改传了的字段，在事务里读改写）。
  2. 启动配置 `IM_SOCIAL_DEFAULTS`（JSON）：只在没有 `social` 行时写入一次（`INSERT … ON CONFLICT DO NOTHING`）；格式非法拒绝启动。
  3. 进程内缓存 + `cache.Invalidator` 主题 `socialSettings`，TTL 60 秒兜底（用注入时钟）；修改成功后给全部在线连接下发 `event/socialSettingsChanged {mode, strangerMsgLimit}`（经 `delivery.Router` 广播，尽力投递）。
  4. `ratelimit.Limiter` 扩展：`Reconfigure(rate, burst)`（剩余令牌截断到新容量、不清零、调大不补满）；`Take(key) bool` / `Refund(key)`（预扣 + 退还，供 `newPeerReject`）；新增 5.2 节全部键的限流器实例，配置失效时调用 `Reconfigure`。单测覆盖截断、退还、并发 `Take` 不超发。
  5. `knownCmds`（`metrics.go`）加入全部新命令。

### BE-103 社交测试设施
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-101、BE-003（测试模式与测试时钟）
- AC：（测试设施：PRD 5.6 节，被 AC-SOC-02-7、04-4、06-3、06-5、06-7、06-8、06-12、07-6、11-2 等使用）
- 内容：只在 `IM_TEST_MODE=1` 下注册（非测试模式 404，e2e 断言）：
  1. `POST /admin/test/social/drop-events {userId, type, count}`：丢弃接下来发给该用户的 `count` 个指定类型事件（在 `delivery.Router` 外包一层测试装饰器）。
  2. `POST /admin/test/social/purge`：按当前测试时钟立即执行一次 `PurgeSocial`。
  3. `POST /admin/test/social/seed {friends?, blocks?, requests?, groupMembers?}`：批量直接写入，不经限流和配额，但**按正常规则**分配 ver、维护 `friend_count / block_count / member_count`；支持指定 `updatedAt`（造过期数据）。
  4. `POST /admin/test/limits/reset {userId}`：清空该用户全部令牌桶（含发送限流和 5.2 节全部键）。
  5. 负向断言辅助：e2e helper `expectNoFrame(conn, 2*time.Second)`（pong 除外）。

### BE-104 双节点测试环境
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-102
- AC：AC-SOC-01-5
- 内容：
  1. 测试模式下的 `cache.Invalidator` 实现 `httpLoopback`：本地失效后 POST 给 `IM_TEST_PEERS` 中的其他进程（`/admin/test/invalidate`）；`IM_TEST_INVALIDATION=off` 关掉广播（只靠 TTL）。
  2. e2e helper：两个 im-server 子进程共享同一个 SQLite 文件（WAL + `busy_timeout`），各自独立测试时钟。
  3. 只验证配置和缓存失效（跨节点事件投递留给 BE-072）。用例：节点 1 改 `mode`，节点 2 上 1 秒内按新模式判定；关闭广播时节点 2 推进 59 秒仍旧、61 秒后新。

### BE-105 社交协议与文档（M3）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-101
- AC：（协议：PRD 9.1～9.4 节 M3 部分）
- 内容：
  1. `protocol/schema/frame.schema.json`：M3 新命令请求 / ack（`friendApply`、`friendRespond`、`friendDelete`、`friendRemark`、`friendRequestsSeen`、`socialSync`、`block`、`unblock`、`userSearch`、`setPrivacy`、`users`、`myGroups`）、数据结构 `Profile / FriendItem / RequestItem / BlockItem / items`、事件 `socialChanged`、`socialSettingsChanged`；`Conversation.memberCount`；`convRemoved.reason`（`removed` / `dissolved`）。`vectors/frames.json` 补合法 / 非法用例；Go 结构体按 Schema 校验（`TestOutFramesMatchSchema`）。
  2. `protocol/errors.json`：9.4 节 12 个新码（与 PRD-admin 第 10 节**一次写入**，写入前核对）；4130 / 4030 / 4040 / 4000 扩大 scope；ERRORS.md 写明 4292 `retryable=false` 例外、文案改为"命令 + 错误码"映射表。
  3. `protocol/vectors/social.json` 初版（同步轮、ack、错误文案用例，供 FE-100 / FE-130 / FE-140 共用；用例由 BE-118 的参考客户端生成并校验）。
  4. `protocol/README.md` 第 3 节规则 9（单聊权限 4035 / 4036 / 4137 / 4292 与判定顺序、客户端必须忽略不认识的事件）；`ADMIN.md` 新增 9.3 节 M3 接口（标"X-5 之前只在测试模式注册"）。
  5. 协议定稿后通知 FE 开工（FE-100）。

---

## S1 好友与同步（M3）

### BE-106 好友申请 friendApply（判定表 9 行）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-102、BE-103、BE-105
- AC：AC-SOC-02-1、AC-SOC-02-2、AC-SOC-02-3、AC-SOC-02-4、AC-SOC-02-5、AC-SOC-02-6、AC-SOC-02-7、AC-SOC-02-8、AC-SOC-02-9、AC-SOC-02-10、AC-SOC-02-12
- 内容：
  1. `logic` 命令 `friendApply`：格式校验（4004 / 4000，附言 0～50、预设备注 0～32 码点，去首尾空白后计数；`source` 枚举；`source=group` 必须带 `sourceRef`）→ 限流 `friendApply`（1/s 突发 5，4290）→ 配额 `friendApplyDay:{uid}`、`friendApplyDay:{uid}:{peer}`（4292；被拒不扣，C-29）。
  2. `Store.ApplyFriend`：一个事务内按 L2 先锁双方（对方不存在只锁自己），再按 F-SOC-02 规则 2 判定表第 3～9 行匹配；第 6 行只写我的 out 行；第 7 行恢复我的边（检查 4132）；第 8 行自动成为好友（4133 不改数据，双方之间所有申请行置 `accepted`）；第 9 行新建 / 合并（`applyCount+1`、`expireAt` 重算、附言 / 来源 / 预设备注覆盖）。`source=group` 在锁内校验双方都在群里，不满足 in 行记 `other`，out 行原样。
  3. ver 与事件按 5.4 节第 1～5 行：每个有变化的用户 `allocVer` 一次；`socialChanged {ver}` 发给 ver 变化的用户（排除发起连接）；对方不存在 / 拉黑我时对方零写入、零帧。
  4. ack `{status, ver, changed, items}`；out 行在 `accepted` 前 `profile=null`，保证第 6 行与第 9 行应答同构。
  5. 日志只记附言 / 备注长度；`protocol.Errorf` 的 `msg` 不拼入用户输入。
  6. 契约测试 `FriendApplyTable`（判定表逐行）；e2e 覆盖列出的 AC。

### BE-107 处理申请、删除好友、备注名
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-106
- AC：AC-SOC-03-1、AC-SOC-03-2、AC-SOC-03-3、AC-SOC-03-4、AC-SOC-03-5、AC-SOC-03-6、AC-SOC-03-7、AC-SOC-05-1、AC-SOC-05-3、AC-SOC-05-4、AC-SOC-05-5、AC-SOC-05-6、AC-SOC-01-8
- 内容：
  1. `friendRespond {from, action, remark?}`：只看我这一侧来自 `from` 的 in 行；不存在 / 已过期 / 申请时被我拉黑（没有 in 行）统一 4045，响应体除 `id` 外逐字节相同；同意：锁双方（L2），4132 / 4133 读加锁后的 `friend_count`（L3），建立双向边（申请人一侧备注 = 预设备注，我这一侧 = 本次 `remark`），双方之间所有申请行置 `accepted`；拒绝：只改我的 in 行，申请人零写入零帧；幂等：同意再同意 / 拒绝再拒绝 `changed=false`、拒绝后可改同意、同意后再拒绝 4091；我拉黑对方时仍可同意。
  2. `friendDelete {userId}`：单向墓碑（`deleted=1`、新 ver、备注清空），`friend_count-1`；不是好友（含不存在）幂等 `changed=false` 不分配 ver；申请行保持原状。
  3. `friendRemark {userId, remark}`：0～32 码点（4000），去空白后为空表示清除；不是我这一侧的好友（含不存在）4040，响应体相同；值未变 `changed=false`。备注不出现在对方的任何接口、管理接口、服务端生成文本里。
  4. 限流 `social:{uid}`（5/s 突发 20）。`maxFriends` 调低后只拦截新增（4132），已有关系保留。
  5. 契约测试 `FriendRespond`、`FriendTombstoneSync`；e2e 覆盖列出的 AC。

### BE-108 关系同步 socialSync
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-107
- AC：AC-SOC-06-1、AC-SOC-06-2、AC-SOC-06-3、AC-SOC-06-4、AC-SOC-06-5、AC-SOC-06-6、AC-SOC-06-10、AC-SOC-06-11、AC-SOC-06-13
- 内容：
  1. `Store.SocialPage(uid, sinceVer | cursor, limit)`：第一页在一个读事务里读 V（当前 ver）与 P（`purgedVer`）；`0 < sinceVer ≤ V 且 sinceVer ≥ P` 为增量轮，否则全量轮（`full=true`）；`untilVer = V` 写进游标整轮不变。
  2. 排序键 `(ver, 类型, 对方 userId, 方向)`，类型顺序 好友 < 申请 < 黑名单；三张表各按 `(owner_id, ver, peer_id[, direction])` 索引取 `limit+1` 行后归并；下一页从排序键严格大于游标的项开始（行值比较 `(ver, peer_id) > (?, ?)`）。增量轮含墓碑；全量轮不含墓碑、申请只取 `updatedAt` 在 30 天内的。
  3. 游标：不透明（base64 JSON + HMAC，密钥启动时生成），编码 `{轮类型, sinceVer, untilVer, 最后排序键, uid}`；非法 / 他人游标 4000；`sinceVer` 与 `cursor` 同时传或都不传、`sinceVer` 为负或非整数、`limit` 不在 1～500 → 4000。
  4. 每页应答头：`full, untilVer, latestVer, hasMore, cursor, friends, requests, blocks, requestSeenVer, settings{searchable}, mode, strangerMsgLimit, official`；每项带最新 `profile`（规则 9：out 申请非 `accepted` 为 null；黑名单项按可见范围，依赖 BE-113 的 `VisibleUsers`，先实现为"好友 / 收到的申请 / 有消息的单聊 / 同群"四条）。`status=expired` 按 `expireAt` 与服务端时间计算。
  5. 修改类 ack 统一 `{ver, changed, items}`（BE-106、BE-107、BE-110 共用组装函数）。
  6. 全量轮限流 `socialFull:{uid}`（每分钟 10 轮、突发 5），在第一页计数，含服务端判定的全量；增量轮不限。
  7. 契约测试 `SocialPaging`（同 ver 跨页、全量分页、轮内变化进入下一轮）；e2e 覆盖列出的 AC（含 `drop-events` 两种补齐路径）。

### BE-109 "新的朋友"红点与清理任务
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-108
- AC：AC-SOC-04-1、AC-SOC-04-2、AC-SOC-04-3、AC-SOC-04-4、AC-SOC-04-6、AC-SOC-04-7、AC-SOC-06-7、AC-SOC-06-8
- 内容：
  1. `friendRequestsSeen {ver}`：`requestSeenVer = max(原值, min(ver, 当前 ver))`；有变化 `allocVer` 并给其他连接发 `socialChanged`（5.4 节第 16 行）；ack `{requestSeenVer, ver, changed}`。
  2. `PurgeSocial(before)`：每小时一次（进程内定时器，集群化时改单节点，记入 ARCH）；按用户在持锁事务里物理删除 `updatedAt < now−30 天` 的墓碑和申请行，`purgedVer = max(purgedVer, 被删行最大 ver)`；不分配 ver、不发事件。
  3. `SocialPage` 后续页检查：增量轮且当前 `purgedVer > sinceVer` → `restart=true`、不带数据。
  4. 查询时 30 天过滤与清理任务互相独立（不执行清理时全量轮也不返回 30 天前的申请）。
  5. 契约测试 `SocialPurge`；e2e 覆盖列出的 AC（测试时钟 + `social/purge`）。

---

## S2 拉黑与私聊权限（M3）

### BE-110 拉黑与黑名单
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-108
- AC：AC-SOC-07-2、AC-SOC-07-3、AC-SOC-07-4、AC-SOC-07-5、AC-SOC-07-6、AC-SOC-07-7、AC-SOC-07-8、AC-SOC-07-9、AC-SOC-07-10
- 内容：
  1. `block / unblock {userId}`：只锁自己（L4）；不能拉黑自己 4000；`maxBlocks` 满 4134；重复拉黑 / 取消不在名单的人幂等 `changed=false`；对不存在的用户照常写黑名单行（计入 `maxBlocks`、`blockDailyLimit`），应答同构。
  2. 拉黑同一事务里把我一侧来自对方的 `pending` in 行置 `rejected`（同一个 ver）；对方零写入零帧；取消拉黑不恢复任何东西；好友边不变。
  3. 限流 `social:{uid}` + 配额 `blockDay:{uid}`（4292）。
  4. 群里拉黑不生效（群发送路径不查黑名单，加回归断言）。
  5. 契约测试 `BlockList`；e2e 覆盖列出的 AC。

### BE-111 私聊权限：发送判定顺序、陌生人限额、新对象配额
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-110、BE-102
- AC：AC-SOC-01-2、AC-SOC-01-4、AC-SOC-01-7、AC-SOC-05-2、AC-SOC-07-1、AC-SOC-07-11、AC-SOC-08-1、AC-SOC-08-2、AC-SOC-08-3、AC-SOC-08-4、AC-SOC-08-5、AC-SOC-08-6、AC-SOC-08-7、AC-SOC-08-9、AC-SOC-08-10、AC-SOC-08-11、AC-SOC-08-14、AC-SOC-08-15
- 内容：
  1. `Store.Relation(a, b)`：一次查询返回 a→b 边、b→a 边、b 是否拉黑 a、双方 `official`、会话是否已有消息（是否新对象）；普通读不加锁（L7），每次 `send` 实时判定、不缓存。
  2. `logic.send` 单聊判定顺序（F-SOC-08 规则 7）：格式 → 去重命中直接返回原 ack → 发送限流 4290 → 新对象时 `newPeerReject.Take`（扣不到 4292）→ 新对象且 `newPeerHourlyLimit > 0` 且我不是官方 → `newPeerHour`（4292，通过即扣）→ 对方不存在 4042 → 好友制且非互为好友且双方都非官方 4035 → 对方拉黑我 4036 → `SaveMessage`。保存成功 `Refund` newPeerReject；第 6～8 步拒绝不退。给自己发永远允许、不碰任何新对象配额。
  3. `SaveMessage` 增加陌生人限额参数（L6）：持有 `conv_seq` 行锁后读双方 `sent_count`，满足 F-SOC-08 规则 4 拒绝条件则回滚（4137，不占 seq），否则写消息、`sent_count+1`；被 4036 / 4137 拒绝的不计数；对方回复过（`sent_count>0`）永久解除。
  4. 模式切换只影响发送：`sync`、`read`、`convs` 不变（加回归断言）。
  5. 契约测试 `StrangerSentCount`；e2e 覆盖列出的 AC（含 `drop-ack` 后被拉黑的重发、10 连接并发预扣）。

---

## S3 搜索、资料、通讯录（M3）

### BE-112 用户搜索与"可被搜索"开关
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-110
- AC：AC-SOC-09-1、AC-SOC-09-2、AC-SOC-09-3、AC-SOC-09-4、AC-SOC-09-5、AC-SOC-09-6、AC-SOC-09-7、AC-SOC-09-8、AC-SOC-09-9
- 内容：
  1. `userSearch {keyword}`：去空白后必须是合法 userId（4004），只精确匹配；返回 `{user: {userId, nickname, avatar, official, relation} | null}`；`relation` 优先级 `self > blocked > friend > pendingIn > pendingOut > none`（与 SDK `relationOf` 同一枚举，定义在 `protocol/vectors/social.json`）。
  2. 不存在 / 不可被搜索 / 封禁（预留过滤点，随 PRD-admin 第二批封禁实现）统一 null，**走同一查询路径**（每次都查用户、隐私、关系后统一判断）；搜索自己总能返回。
  3. `setPrivacy {searchable}`：只锁自己，值变化 `allocVer`（5.4 节第 15 行）并给其他连接发 `socialChanged`；`NULL` 表示跟随 `searchableDefault`。
  4. 限流 `search:{uid}`（每分钟 10、突发 10，4290）+ 配额 `searchDay:{uid}`（4292）。日志只记"搜索一次、是否命中"。
  5. e2e 覆盖列出的 AC；09-9 的耗时对比在 BE-119 中以基准测试运行（关闭限流）。

### BE-113 批量查资料 users、我的群聊 myGroups、会话成员数
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-108、BE-033（群成员缓存）
- AC：AC-SOC-10-1、AC-SOC-10-2、AC-SOC-10-3、AC-SOC-10-4、AC-SOC-10-5、AC-SOC-10-6、AC-SOC-11-1、AC-SOC-11-2、AC-SOC-11-6、AC-SOC-26-5
- 内容：
  1. `Store.VisibleUsers(uid, ids)`：自己 ∪ 我这一侧非墓碑好友 ∪ 单聊已有消息 ∪ 同在未解散群（`group_members(user_id)` 索引）∪ 30 天内的 in 申请；不在范围 / 不存在都不返回不报错。`users {userIds}`：1～100（4000）、含非法 ID 4004、去重；限流 `users:{uid}`（每分钟 30、突发 10）。
  2. `myGroups {cursor?, limit?}`：`limit` 1～200（默认 200），按 groupId 升序游标翻页，返回 `{groups:[{groupId, name, memberCount, joinedAt}], nextCursor}`；不受 `convs` 500 上限；限流 `groupRead:{uid}`（10/s 突发 30）。
  3. `member_count` 维护：管理接口加 / 减成员在持有群锁（L5）的同一事务里更新；`convs` 群会话、`GET /admin/groups/{id}` 返回增加 `memberCount`（ADMIN.md 同步）。
  4. 契约测试 `VisibleUsers`、`MyGroups`、`MemberCountMaintained`；e2e 覆盖列出的 AC；11-6 的并发版本见 BE-117。

---

## S4 管理接口（M3）

### BE-114 社交管理接口：配置、好友导入 / 删除 / 查询、审计
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-102、BE-107、PRD-admin F-KEY-01（X-5）、PRD-admin F-KEY-02（X-5）、PRD-admin F-KEY-04
- AC：AC-SOC-12-1、AC-SOC-12-2、AC-SOC-12-3、AC-SOC-12-4、AC-SOC-12-5、AC-SOC-12-6、AC-SOC-12-7、AC-SOC-12-8、AC-SOC-12-9、AC-SOC-12-10、AC-SOC-01-3
- 内容：
  1. 路由：`GET / PATCH /admin/settings/social`、`POST /admin/friends/import`、`DELETE /admin/friends`、`GET /admin/users/{userId}/friends`；X-5 落地前只在 `IM_TEST_MODE=1` 注册（生产 404）；X-5 之后注册在 `IM_ADMIN_ADDR`，scope `social.settings` / `social.friends`（不足 403 / 4030）。
  2. `ImportFriends`：≤ 500 对（4131）；错误优先级 4004 → 4000（`a==b`）→ 4131 → 4042 → 4132（`msg` 写出 userId）；整批原子：收集全部用户、去重、按字节序一次性加锁（L3，最多 1000 行）后逐对处理；幂等（备注值变化才算变化）、同批重复对以最后为准；每个有变化的用户 ver 只 +1；同时把这些对之间 pending / rejected 申请置 `accepted`；不受拉黑影响也不解除拉黑。
  3. `RemoveFriends`：双向墓碑，不是好友忽略；`ListFriends`：按对方 userId 升序游标翻页，`limit` 1～500（默认 100），每项 `{userId, source, createdAt}`，**不返回备注**。
  4. 审计：每次成功调用经 `AppendAdminAudit`（F-KEY-04）写一条：调用方（测试模式为 `test`）、接口、涉及 userId、变化摘要（配置旧值 → 新值、导入 / 删除对数）；不含备注原文。
  5. 契约测试 `FriendImportAtomic`；e2e 覆盖列出的 AC。

### BE-115 官方账号
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-111、BE-114
- AC：AC-SOC-13-1、AC-SOC-13-2、AC-SOC-13-3、AC-SOC-13-4、AC-SOC-13-5、AC-SOC-13-6、AC-SOC-13-8
- 内容：
  1. `PUT /admin/users/{userId}/official {official}`（scope `social.settings`，上线条件同 BE-114，用户不存在 404 / 4042，审计）；`GET /admin/users/{userId}` 返回 `official`。
  2. 豁免点：好友制（任一方官方）、陌生人限额（任一方官方）、`newPeerHour`（发送方官方）；不豁免：发送限流、拉黑、`newPeerReject`、申请 / 搜索 / 群规则。发送路径每次经 `Relation` 读取，不缓存（取消后 1 秒内生效）。
  3. 资料带 `official`（搜索、`users`、同步项）；`socialSync` 应答头 `official` 为本人标记。改标记不改 ver、不发事件（5.4 节第 19 行）。
  4. e2e 覆盖列出的 AC。

---

## S5 e2e 与并发测试代码（M3）

### BE-116 M3 后端 e2e：两种模式矩阵与 CI
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-106 ～ BE-115、BE-122
- AC：AC-SOC-08-13、AC-SOC-01-4、AC-SOC-05-2、AC-SOC-12-5
- 内容：
  1. `server/e2e/social_*_test.go`：TC-SOC M3 后端用例逐条实现（用例名带 TC 编号），每个用例独立数据库 + `limits/reset` 隔离。
  2. CI 矩阵：`IM_SOCIAL_DEFAULTS` 为空（默认 open）与 `{"mode":"friends"}` + 预置好友关系各跑一遍现有私聊 e2e；默认配置下 `server/e2e` 现有用例**不改代码**全过（AC-SOC-08-13，PRD-core V-21）。
  3. 通用断言 helper：应答"除 `id` / ver / 时间戳外逐字段相同"、"2 秒内收不到任何帧"。
  4. CI 加 `tools/trace.py --check`（已有）确认 social 204/204。

### BE-117 M3 Postgres 并发测试代码（B6 前置）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-116、BE-101
- AC：AC-SOC-02-13、AC-SOC-03-8、AC-SOC-03-9、AC-SOC-08-12、AC-SOC-11-6、AC-SOC-06-9
- 内容：
  1. `store/storetest` 并发组（`IM_TEST_PG_DSN` 存在时跑 Postgres，否则 SQLite）：`FriendMutualApplyRace`（100 组互相申请）、`FriendLimitRace`（1999 好友两连接同时同意 × 50 轮）、`FriendMutualAcceptNoDeadlock`（100 轮 ≤ 10 秒）、`StrangerLimitRace`（2 连接 × 10 条恰好 3 成功、seq 1～3）、`MemberCountRace`（管理接口 10 加 10 减）、`AdminImportVsApplyNoDeadlock`（导入与申请 / 同意交错 1000 次）、`SocialVerConcurrent`。
  2. 每个测试断言：结果正确、无死锁、无 5000；Postgres 上记录 `store_deadlock_total` 为 0。
  3. 在 BE-070 的验收清单里登记这 7 项为前置条件。

---

## S6 专项测试（M3）

### BE-118 5.4 节逐行 ver 与事件断言、共享 social.json 用例
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-116、BE-119（Go 参考客户端，两者并行开发，BE-119 先交付参考客户端）
- AC：AC-SOC-03-7、AC-SOC-06-9、AC-SOC-06-11
- 内容：
  1. 契约测试 `SocialVerPerOp`：5.4 节 19 行逐行——谁的 ver 变化、改了哪些行（同一 ver）、谁收到 `socialChanged`、谁 2 秒内收不到任何帧（e2e 版本同步跑一遍）。
  2. 用 Go 参考客户端（BE-119）跑 `protocol/vectors/social.json`，作为三套 SDK 的基准实现；用例必须包含 PRD 10.3 节列出的 8 类场景。

### BE-119 同步模型测试（随机操作序列）
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-116
- AC：AC-SOC-06-15
- 内容：
  1. `server/e2e/socialmodel`：Go 参考客户端实现 PRD 10.2 节全部同步规则（增量逐页应用、全量整体替换、`restart`、事件去抖、ack 规则、轮结束追加一轮）。
  2. 随机生成器：两个用户各两个连接，随机执行申请 / 同意 / 拒绝 / 删除 / 备注 / 拉黑 / 取消拉黑 / 隐私 / 标记已读 / 管理接口导入删除，随机断线重连、`drop-events`、推进测试时钟并 `purge`、`limit` 在 1～500 随机；静止后断言每个客户端本地状态与服务端全量逐项相同、本地 ver = 服务端 ver。
  3. 每次 CI ≥ 1000 个序列（固定种子 + 随机种子各一半）；失败时做序列收缩，输出最小复现序列与种子。
  4. 同一包内 `BenchmarkSearchTiming`：不可被搜索的 bob 与 `nobody` 各 1000 次（关闭限流），中位数差 ≤ 20%（AC-SOC-09-9）。

### BE-120 防枚举与隐私安全用例（11.2 节逐路径）、日志扫描
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-116、BE-066（日志脱敏测试框架）
- AC：AC-SOC-02-4、AC-SOC-02-6、AC-SOC-02-11、AC-SOC-03-6、AC-SOC-05-3、AC-SOC-05-5、AC-SOC-07-9、AC-SOC-08-10、AC-SOC-08-11、AC-SOC-09-3、AC-SOC-09-8、AC-SOC-10-5、AC-SOC-10-6、AC-SOC-12-9
- 内容：
  1. `server/e2e/social_enum_test.go`：11.2 节每一行一条用例（send 被拒、userSearch、friendApply、block / unblock、users、remark / delete / respond），统一用"除 `id` / ver / 时间戳外 JSON 逐字段比较"helper；1.3 节指标：一个账号一天最多确认 100 + 100 个 userId。
  2. 日志扫描：附言 `ZXQ-SOCIAL-123`、预设备注 `ZXQ-REMARK-456`、搜索 `ZXQ-SEARCH-9`、导入备注 `ZXQ-REMARK-789` 处理完后扫描整个测试进程日志和 `admin_audit` 全部字段，都搜不到（复用 BE-066 的捕获方式，变异测试验证）。
  3. 隐私负向断言：被单向删除、被拒绝、被拉黑、被静默的一方 2 秒内零帧、ver 不变（1.3 节"隐私"指标）。

### BE-121 性能：全量同步压测与不回归对比
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-116、BE-050（压测工具与基线）
- AC：AC-SOC-06-12、AC-SOC-08-13
- 内容：
  1. `im-loadtest` 新场景 `social-full`：`seed` 5000 好友 + 1000 黑名单 + 200 申请，全量轮 `limit=500`，单页 P99 ≤ 100ms、整轮 ≤ 1 秒；3 次取中位数（C-48 同一基准机器）。
  2. 新场景 `c2c-friends`：好友制下私聊发送 P99 与开放模式对比，增加 ≤ 2ms（1.3 节）。
  3. 默认配置下重跑 TC-PERF-00-01～07：错误率 0、P99 与 M3 前基线（`docs/reports/perf-2026-11-03.md`）相比 ±10%（3 次中位数）；TC-PERF-00-02 的错误码统计中没有 4292、4137、4035、4036。报告 `docs/reports/perf-social-*.md`。

---

## 解散群（M3，与 PRD-admin 第一批同一实现）

### BE-122 解散群：DissolveGroup、管理接口解散、convRemoved.reason
- 优先级：P0
- 里程碑：M3
- 仓库：tianting-server
- 依赖：BE-100、BE-113、PRD-admin G-ADM-11 / F-ADM-24（同一实现，工作量计入 PRD-admin）、PRD-admin F-KEY-01（X-5）
- AC：AC-SOC-26-2、AC-SOC-26-4、AC-SOC-26-5、AC-SOC-26-6
- 内容：
  1. `Store.DissolveGroup(gid)`：持有群锁（L5），一个事务内删除该群全部 `group_members` 行、写 `chat_groups.dissolved_at`、`member_count=0`；消息保留；同流程发 `TopicGroupMembers` 失效。客户端解散、最后一人退群（M4，BE-134）复用同一方法。
  2. `DELETE /admin/groups/{groupId}`（X-5 后注册在管理端口，审计由 PRD-admin F-KEY-04）；所有在线成员收到 `convRemoved {convId, reason:"dissolved"}`；管理接口移除成员的 `convRemoved` 补 `reason:"removed"`。
  3. F-SOC-26 规则 3 的全部查询加 `dissolved_at = 0`（或由成员行已删自然满足）：`GroupExists`（`memberCache`）、`GetGroup`、管理接口 `getGroup` / 成员查询 / 加成员 / 改设置（404 / 4043）、`ListConversations`、`GetConversation`、`GroupConversations`、`myGroups`、`VisibleUsers`、`CreateGroup`（ID 存在过 → 4090）；`send` / `sync` / `read` → 4043。
  4. 契约测试 `GroupDissolveNoReuse`：规则 3 每个查询一条断言 + 消息行仍在 + `group_members` 无该群 + `dissolved_at ≠ 0`；e2e 覆盖列出的 AC（M3 部分）。

---

## G0 群基座（M4）

### BE-130 0005 迁移、ug- 预检、群锁与提示消息同事务
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-117（M3 后端完成）、BE-122
- AC：AC-SOC-29-8、AC-SOC-26-4
- 内容：
  1. `0005_group_roles.sql`：`chat_groups` 加 `owner_id`、`created_by`、`notice`、`notice_by`、`notice_at`、`max_members DEFAULT 2000`（`dissolved_at` 已在 0004）；`group_members` 加 `role DEFAULT 'member'`、`role_at`；新表 `client_group_req`（PK `(user_id, client_req_id)`、索引 `created_at`），7 天清理并入 `PurgeSocial` 定时器。
  2. 迁移前预检：已有 `ug-` 开头的群 ID 时中止并列出（`migrate up` 返回非 0）。
  3. 群 Store 方法统一骨架：`withGroupLock(gid, fn)` → 权限 / 人数 / 群主 / `dissolved_at` 判断 → 修改 → 可选 `tip` 参数在同一事务分配 seq（顺序：群锁 → `conv_seq`）；任何一步失败整体回滚。故障注入点 `IM_TEST_MODE` 下"提示消息写入失败"。
  4. 契约测试 `TipSameTx`（注入失败后成员表、`member_count`、seq 不变）、`GroupDissolveNoReuse`（M4 新增的 `groupInfo` / `groupMembers` 查询补断言）；加锁顺序断言覆盖全部群方法。

---

## G1 群命令（M4）

### BE-131 群角色、groupInfo、groupChanged 合并
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-130
- AC：AC-SOC-20-1、AC-SOC-20-2、AC-SOC-20-3、AC-SOC-20-4、AC-SOC-20-5、AC-SOC-20-6、AC-SOC-20-7、AC-SOC-20-10
- 内容：
  1. 权限矩阵实现为一张表 `groupPerm[role][op]`（F-SOC-20 规则 2，含"托管群无群主"列），所有群命令统一调用；不缓存角色。
  2. `groupInfo {groupId}`：返回 `{groupId, name, notice, noticeBy, noticeAt, owner, managed, myRole, memberCount, maxMembers, historyVisible, createdAt}`；非成员 4033、不存在 / 已解散 4043；限流 `groupRead`。
  3. `convs`、`myGroups` 返回 `myRole`；Schema 加 `Conversation.myRole`。
  4. `groupChanged {groupId}`：进程内按群 1 秒合并（首个变化立即发、之后窗口内合并为窗口末一条），发给在线成员。
  5. 群写命令限流 `groupOp:{uid}`（5/s 突发 20）；未注册的 `group*` 在 M3 返回 4001，M4 起注册。

### BE-132 客户端建群 groupCreate 与提示消息协议
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-131
- AC：AC-SOC-21-1、AC-SOC-21-2、AC-SOC-21-3、AC-SOC-21-4、AC-SOC-21-5、AC-SOC-21-6、AC-SOC-21-7、AC-SOC-21-8、AC-SOC-21-9、AC-SOC-21-10、AC-SOC-21-11、AC-SOC-29-4、AC-SOC-29-6、AC-SOC-29-9
- 内容：
  1. 协议：`MsgType` 加 `tip`；`Message.from`、`LastMsg.from` 用 `if/then` 只在 `type=tip` 时允许空串；`Error` 加可选 `userId`（若 PRD-core 12.3 节 D-1 定为 `details.userId` 则按 `details`）；`body.tip {kind, actor, targets, count?, text}`；`vectors/frames.json` 补用例（tip + 空 from 通过、text + 空 from 失败、带 userId 的错误 ack）。客户端 `send` 的 `type=tip` 或 `clientMsgId` 以 `tip-` 开头 → 4000。
  2. `groupCreate {clientReqId, name?, members}`：`clientReqId` 8～64；`members` 去掉自己、去重后 ≥ 2（4000）；总人数 ≤ `clientGroupMaxMembers`（4130）；`clientGroupEnabled=false` 4030；配额 `groupCreateDay`（4292）；每个被邀请人存在 + 互为好友 + 未拉黑我，否则 4035 且 `error.userId`（三种情况同构）；被邀请人或我自己的群数 ≥ `maxJoinedGroups` 4135（`count(*)`，软上限）。
  3. `CreateClientGroup`：groupId = `ug-` + UUIDv7 去连字符；默认群名用前 3 人昵称（无昵称用 userId，绝不用备注）"、"连接、截断 50 码点；`historyVisible=false`、`max_members` 固定；群主 = 建群者；同一事务写 `client_group_req`（只记成功）和 `created` 提示消息（seq=1，`from`=建群者，推进其 readSeq）；同一 `clientReqId` 重复提交返回首次结果。
  4. 提示消息摘要生成器（5.5 节，只用昵称），所有 kind 共用；`POST /admin/groups` 拒绝 `ug-` 前缀（400 / 4004，ADMIN.md 同步）。
  5. ack `{groupId, conv}`；被邀请人和建群者其他连接收到 `convAdded`；成员缓存失效。

### BE-133 邀请入群与移出成员
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-132
- AC：AC-SOC-22-1、AC-SOC-22-2、AC-SOC-22-3、AC-SOC-22-4、AC-SOC-22-5、AC-SOC-22-6、AC-SOC-24-1、AC-SOC-24-2、AC-SOC-24-3、AC-SOC-24-4、AC-SOC-29-7
- 内容：
  1. `groupInvite {groupId, members}`：1～50 人（4000）；非成员 4033、群不存在 4043、权限矩阵（托管群无群主 4030）；逐人校验同建群（4035 / 4135），部分成功 `{added, failed:[{userId, code}]}`；已是成员忽略、不重置 joinSeq；加完超过上限（自建群 `max_members`、托管群 2000）整次 4130。
  2. `InviteMembers`：群锁内写成员（**先按当前 maxSeq 记 joinSeq，再分配 `joined` 提示消息 seq**，保证新成员 minSeq 处能看到提示、未读 1）、`member_count`；`targets` 最多 20 个，超出带 `count`；提示消息推进邀请人 readSeq 并给其他连接发 `readUpdated`；新成员 `convAdded`；`TopicGroupMembers` 失效；在线成员 `groupChanged`。
  3. `groupKick {groupId, members}`：1～50；不能移出自己（4000）、不能移出群主（4030）、管理员只能移出成员；非成员忽略，ack `{removed}`；被移出者 `convRemoved {reason:"removed"}`；被移出的管理员失去角色；不生成提示消息。
  4. e2e 覆盖列出的 AC。

### BE-134 退群、转让群主、设置管理员、客户端解散
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-133、BE-122
- AC：AC-SOC-23-1、AC-SOC-23-2、AC-SOC-23-3、AC-SOC-23-4、AC-SOC-23-5、AC-SOC-23-6、AC-SOC-23-7、AC-SOC-25-1、AC-SOC-25-2、AC-SOC-25-3、AC-SOC-25-4、AC-SOC-26-1、AC-SOC-26-2、AC-SOC-26-3、AC-SOC-29-2
- 内容：
  1. `groupLeave`：非成员 4033；同"被移除"处理，本人其他连接 `convRemoved {reason:"left"}`（Schema 加 `left`）；不生成提示消息；在线成员 `groupChanged`。群主退群：自建群转给最早的管理员（`role_at`、userId），无管理员给最早入群成员，`ownerChanged`（`from=""`）；托管群回到无群主（`targets=[]`）。最后一人退群：自建群调用 `DissolveGroup`（同一事务），托管群保留空群。
  2. `groupTransfer {groupId, to}`：只有群主（4030）；`to` 非成员 4046、为自己 4000；原群主变成员，转给管理员时管理员数 −1；`ownerChanged`（`from`=原群主）。`chat_groups.owner_id` 与成员行 `role` 同事务更新。
  3. `groupSetAdmin {groupId, userId, admin}`：只有群主；目标非成员 4046、自己 4000、满 10 人 4136；幂等不生成提示；`adminSet` / `adminUnset` 提示消息。
  4. `groupDissolve {groupId}`：只有自建群群主（托管群任何人 4030）；调用 `DissolveGroup`，效果同 BE-122。
  5. 契约测试 `GroupRoles`、`GroupOwnerAutoTransfer`、`ManagedOwnerRevert`；e2e 覆盖列出的 AC。

### BE-135 修改群名与公告、群成员分页
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-134
- AC：AC-SOC-27-1、AC-SOC-27-2、AC-SOC-27-3、AC-SOC-27-5、AC-SOC-28-1、AC-SOC-28-2、AC-SOC-28-3、AC-SOC-28-4、AC-SOC-29-1
- 内容：
  1. `groupUpdate {groupId, name?, notice?}`：至少一项（4000）；群主 / 管理员（4030）；群名 1～50 码点、公告 0～1000 码点（空串清除），记 `noticeBy` / `noticeAt`（服务端时间）；同事务生成 `renamed`（text = 新群名）/ `noticeUpdated`（text = 公告前 100 码点）；下一次 `convs` 生效；`groupChanged`。日志不记公告原文。
  2. `ListGroupMembers(gid, cursor, limit)`：排序 群主 → 管理员（`role_at`、userId）→ 成员（入群时间、userId）；游标编码排序键（不透明、HMAC），非法 4000；`limit` 1～100（默认 50）；非成员 4033、不存在 / 解散 4043；返回 `{members:[{userId, role, joinedAt, nickname, avatar, official}], nextCursor}`；与 PRD-admin G-ADM-10 共用存储查询。
  3. 契约测试 `GroupMembersPaging`（翻页期间入群 / 退群）；e2e 覆盖列出的 AC（29-1 的 6 种 kind 连续 seq 在本任务收尾验证）。

### BE-136 托管群与管理接口交接
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-134、PRD-admin F-KEY-02（X-5）、PRD-admin F-KEY-04、PRD-admin F-ADM-25
- AC：AC-SOC-30-1、AC-SOC-30-2、AC-SOC-30-3、AC-SOC-30-4、AC-SOC-30-5、AC-SOC-30-6、AC-SOC-30-7、AC-SOC-29-3
- 内容：
  1. `POST /admin/groups` 增加 `owner?`；`PATCH /admin/groups/{id}` 增加 `owner`（字符串：指定 / 更换，非成员 404 / 4046；`null`：收回）、`notice?`；`Store.SetGroupOwner` 生成 `ownerChanged`（`from=""`）；scope `social.settings`、审计（F-KEY-04）。
  2. `GET /admin/groups/{id}` 返回增加 `owner`、`admins`、`notice`、`createdBy`、`managed`（ADMIN.md 同步）。
  3. 管理接口对所有群保留全部能力，不受好友、`max_members`、`maxJoinedGroups` 限制（仍受 2000）；不产生提示消息（群主变化除外）；管理接口移除群主：自建群按 F-SOC-23 规则 4 自动转让，托管群回到无群主。
  4. e2e 覆盖列出的 AC（含"管理接口建群、加 500 人、改名、改 historyVisible 后 maxSeq 为 0"）。

---

## G2 e2e 与专项测试（M4）

### BE-137 M4 后端 e2e 与 Postgres 并发测试代码（B6 前置）
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-131 ～ BE-136
- AC：AC-SOC-20-9、AC-SOC-22-7、AC-SOC-23-8、AC-SOC-23-9
- 内容：
  1. `server/e2e/group_m4_test.go`：TC-SOC M4 后端用例逐条实现（用例名带 TC 编号）。
  2. 并发组：`GroupMemberCountRace`（20 邀请 + 10 移出后 `memberCount` = `groupMembers` 总数）、`GroupInviteVsDissolveRace`（50 轮）、`GroupLastTwoLeaveRace`（50 轮）、`GroupOwnerLeaveVsTransferRace`（50 轮）；在 BE-070 验收清单登记为前置条件。
  3. M4 回归：TC-GRP 现有用例（只用管理接口）不改代码全过（PRD-core V-08、V-10）。

### BE-138 群权限矩阵遍历与群主变化全场景
- 优先级：P0
- 里程碑：M4
- 仓库：tianting-server
- 依赖：BE-137
- AC：AC-SOC-20-8、AC-SOC-23-2、AC-SOC-23-7、AC-SOC-30-5、AC-SOC-30-6、AC-SOC-27-5
- 内容：
  1. 表驱动 e2e：5 类角色 / 群类型 × 7 类操作，每格一条断言，矩阵数据与服务端 `groupPerm` 表**分开维护**（从 PRD 表格手抄），防止实现与测试共用一份错误数据。
  2. 群主变化全场景：自建群有管理员 / 无管理员 / 只剩一人；托管群群主退群；管理接口移除群主（两类群）；每种情况断言新群主、提示消息 `from` / `targets`、无群主时没有任何成员获得群主权限。
  3. 日志扫描：公告特征串 `ZXQ-NOTICE-321`；XSS 字符串在群名、公告中原样存取。
