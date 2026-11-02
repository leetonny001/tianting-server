# 后端任务单（一期）

| 项 | 内容 |
|---|---|
| 依据 | [PRD v1.1](../prd/PRD-core-v1.1.md)、[错误码](../../protocol/ERRORS.md) |
| 负责 | 后端（AI） |
| 起点 | M1 代码（`server/`）：鉴权、WS 网关、单聊 / 群聊、seq、去重、sync、已读位置 |
| 完成定义 | 任务内列出的 AC 都有通过的自动化测试（后端 e2e 或单测）；`go test -race ./...` 全过；协议文档 / Schema / 用例同步更新 |

格式说明：每个任务的 `- AC：` 一行列出它负责满足的验收标准，追溯矩阵由脚本从这一行生成。

## 进度

| 任务 | 状态 | 说明 |
|---|---|---|
| BE-001 协议定稿 | ✅ 完成 | Schema v1、28 条合法 / 17 条非法用例、README v1；Go 结构体编码结果按 Schema 校验 |
| BE-002 错误码落地 | ✅ 完成 | Go 常量与 errors.json 的一致性测试；M1 的 7 处差异已修正（token 无效时的"先升级"放在 BE-012） |
| BE-003 测试模式与故障注入 | ✅ 完成 | `IM_TEST_MODE`、drop-ack / skip-delivery / clock 接口（非测试模式下 404）、可注入时钟、有效期下限 1 秒 / 60 秒 |
| BE-004 文本校验 | ✅ 完成 | Unicode White_Space 去空白、按码点计数、4005 / 4006 |
| BE-005 持久化可靠性 | ✅ 完成 | `synchronous=FULL`；kill -9 子进程测试通过。注意：kill -9 只能验证进程崩溃，整机断电场景依赖 FULL 设置，自动化测试无法覆盖 |
| BE-006 M1 能力回归 | ✅ 完成 | e2e 对齐新错误码；补充同平台两个连接、重复提交 5 次 |
| BE-015 关闭码 | ✅ 提前完成 | 空闲 4408、慢连接 4400。修复了一个 bug：读超时会让库直接断开连接，4408 发不出去，改为空闲计时器 |
| BE-020 单聊对方存在校验 | ✅ 完成 | 对方不存在返回 4042，不落库、不占用 seq；给自己发消息不校验 |
| BE-021 去重校验会话一致性 | ✅ 完成 | **改为发送前先查重**（IMP-3）：命中且会话一致返回原 ack，不一致返回 4008 |
| BE-022 sync 翻页与权限 | 🟡 基本完成 | `beforeSeq`、limit、4032 / 4033 / 4043 已完成；群历史可见性（minSeq）在 BE-032 |
| BE-023 发送限流 | ✅ 完成 | 按用户的令牌桶（容量 20、每秒补充 20），多连接共享；先查重再扣令牌；被拒绝的请求不扣；变异测试验证了查重与限流的先后顺序 |
| BE-024 消息字段 | ✅ 完成 | B0 已完成 |
| BE-010 用户表与导入 | ✅ 完成 | 导入（upsert、单次 500 上限、整批校验、同批重复以最后一条为准）、`GET /admin/users/{userId}`、签发凭证时自动注册 |
| BE-011 凭证有效期规则 | ✅ 完成 | B0 已实现，B1 补齐边界 e2e（长度 0 / 1 / 64 / 65，空格、中文、`.`、`@`） |
| BE-012 先升级再踢 | ✅ 完成 | 检查顺序 4009 → 4010 → 4012 → 4011（C-08）；平台不合法不发 kicked；kicked 同步写完再关闭；M1 的 HTTP 401 测试已改写 |
| BE-013 到期踢下线与强制下线 | ✅ 完成 | 连接上挂到期定时器；`POST /admin/users/{userId}/force-logout`；**改用令牌版本号作废旧 token**（见 IMP-8） |
| BE-014 演示登录 | ✅ 完成 | `IM_DEMO_LOGIN=1` 才注册；去首尾空白；有效期 1 天；CORS |
| 其余 | ⬜ 未开始 | |

## 总览

| 阶段 | 任务 | 说明 |
|---|---|---|
| B0 基础对齐 | BE-001 ～ BE-006 | 协议、错误码、测试模式、文本校验、持久化、M1 能力回归 |
| B1 用户与鉴权 | BE-010 ～ BE-015 | 用户表、凭证规则、先升级再踢、强制下线、演示登录、关闭码 |
| B2 消息 | BE-020 ～ BE-024 | 用户存在校验、去重校验、sync 翻页、限流、消息字段 |
| **B2.5 架构与安全基座** | BE-060 ～ BE-069 | 版本化迁移、存储契约测试、投递 / 限流 / 缓存接口、备份恢复、安全 P0（见 `docs/arch/ARCH-v1.md`） |
| B3 群 | BE-030 ～ BE-034 | 群模型、群管理接口、历史可见性、成员缓存、成员变动事件 |
| B4 会话 | BE-040 ～ BE-043 | 会话列表、未读、已读多端同步（P1）、昵称（P1） |
| B5 非功能 | BE-050 ～ BE-051 | 压测工具、可观测 |
| B6 集群化（二期） | BE-070 ～ BE-075 | Postgres 实现、数据迁移工具、NATS 投递、Redis 在线路由与限流、多节点 e2e |

依赖顺序：B0 → B1 → B2 → B2.5 → B3 → B4 → B5（B3 中的 BE-030 依赖 BE-010 的用户表）。前端可以在 BE-001 完成（协议定稿）后开始并行开发 SDK。

---

## B0 基础对齐

### BE-001 协议定稿：Schema、用例、协议文档
- 优先级：P0
- 依赖：无
- AC：AC-MSG-04-7、AC-MSG-01-6
- 内容：
  1. 按 PRD 第 8 节更新 `protocol/schema/frame.schema.json` 和 `protocol/vectors/frames.json`：新增 `convs` 请求 / 应答；`sync` 增加 `beforeSeq`（与 `afterSeq` 二选一）；`kicked` 原因新增 `tokenInvalid`；新增 `convAdded` / `convRemoved` 事件；`Message.clientMsgId` 改为必填；send 的 ack 增加 `convId`。
  2. 更新 `protocol/README.md`：命令说明、连接流程（先升级再踢）、重连流程（先 `convs` 再 `sync`）。
  3. Go 结构体同步修改，`TestOutFramesMatchSchema` 覆盖新结构。
- 完成标准：协议测试通过；前端评审确认协议可以开工。

### BE-002 错误码落地
- 优先级：P0
- 依赖：BE-001
- AC：AC-SYS-01-1、AC-SYS-01-2
- 内容：
  1. 从 `protocol/errors.json` 生成（或手写后加测试校验）Go 错误码常量，包括关闭码。
  2. 一致性测试：Go 常量集合与 `errors.json` 完全一致。
  3. 按 ERRORS.md 第 5 节修正 M1 的 7 处差异（4033 / 4032 / 4043 / 4090 / 4002～4008 / 4013 / 关闭码）。
  4. 管理接口的 HTTP 状态码取自 `errors.json`。

### BE-003 测试模式与故障注入
- 优先级：P0
- 依赖：无
- AC：AC-AUTH-03-1、AC-CONN-01-1、AC-MSG-03-4
- 内容：
  1. 配置项 `IM_TEST_MODE`（默认关闭，开启时启动日志打印醒目警告）。
  2. 测试模式下：`ttlSeconds` 最小可以是 1；空闲超时、写超时可以配置（已有 `gateway.Config`，补充环境变量）。
  3. 故障注入管理接口（仅测试模式注册路由）：`POST /admin/test/drop-ack {clientMsgId}`（保存消息但不回 ack）、`POST /admin/test/skip-delivery {clientMsgId}`（保存消息但不做实时投递，用来制造 seq 缺口）、`POST /admin/test/clock {nowMs}`（注入服务端时钟，C-43）。
  4. `auth.Signer` 暴露可注入的时钟。
- 完成标准：非测试模式下这些路由返回 404（e2e 断言）。

### BE-004 文本校验：去空白、按码点计数
- 优先级：P0
- 依赖：BE-002
- AC：AC-MSG-01-4、AC-MSG-01-5
- 内容：去掉首尾空白（含 `\n`、`\t`、全角空格 U+3000）后按 Unicode 码点计数；为空返回 4005，超过 5000 返回 4006；存储去空白后的内容。

### BE-005 持久化可靠性
- 优先级：P0
- 依赖：无
- AC：AC-MSG-04-6
- 内容：SQLite 设置 `synchronous=FULL`；新增 e2e：以子进程启动服务，发消息收到 ack 后 `kill -9`，重启后 `sync` 能拉到这条消息。

### BE-006 M1 已实现能力回归
- 优先级：P0
- 依赖：BE-002
- AC：AC-AUTH-02-1、AC-AUTH-02-5、AC-MSG-02-1、AC-MSG-05-1、AC-GRP-02-1、AC-SYNC-01-1、AC-SYNC-01-2、AC-MSG-04-1、AC-MSG-04-5、AC-CONV-02-1、AC-CONV-02-2
- 内容：把 M1 已有的 e2e 用例对齐新错误码和 TC 编号（用例名带 TC 编号，便于追溯）；补充同平台两个连接（web + web）的多端用例；补充"重复提交 5 次"的去重用例。

---

## B1 用户与鉴权

### BE-010 用户表与导入接口
- 优先级：P0
- 依赖：BE-002
- AC：AC-USER-01-1、AC-USER-01-2、AC-USER-01-3、AC-USER-01-4、AC-USER-01-5
- 内容：
  1. 新表 `users(user_id PK, nickname, avatar, created_at, tokens_valid_after)`。
  2. `POST /admin/users/import {users:[{userId, nickname?, avatar?}]}`：upsert；单次 ≤ 500，超出返回 4131；任意 `userId` 不合法则整批不写入，返回 4004。
  3. 签发凭证时，用户不存在则自动注册。
  4. Store 接口新增 `UserExists`、`UpsertUsers` 等方法。

### BE-011 凭证有效期规则
- 优先级：P0
- 依赖：BE-003、BE-010
- AC：AC-AUTH-01-1、AC-AUTH-01-2、AC-AUTH-01-3、AC-AUTH-01-4、AC-AUTH-01-5
- 内容：`ttlSeconds` 不传、0 或负数按 7 天，上限 30 天，测试模式下最小 1 秒；密钥错误返回 401 / 4013；`userId` 不合法返回 400 / 4004；补充边界 e2e（长度 0 / 1 / 64 / 65，含空格、中文、`.`、`@`）。

### BE-012 凭证错误改为"先升级、再踢"，校验平台标识
- 优先级：P0
- 依赖：BE-002
- AC：AC-AUTH-02-2、AC-AUTH-02-3、AC-AUTH-02-6
- 内容：
  1. token 无效 / 缺失：完成 WS 升级 → 下发 `kicked/tokenInvalid` → 以 4010 关闭；已过期：`kicked/tokenExpired` → 4011。
  2. platform 缺失或不在 android / ios / web / pc 中：以 4009 关闭。
  3. `kicked` 帧**写完后**再关闭（同步写，不走异步队列）。
  4. `auth.Verify` 区分"无效"和"过期"两种错误，并返回过期时间。

### BE-013 连接期间到期踢下线与强制下线
- 优先级：P0
- 依赖：BE-010、BE-012
- AC：AC-AUTH-03-1、AC-AUTH-03-2、AC-AUTH-03-3、AC-AUTH-03-4
- 内容：
  1. 连接上按 token 过期时间挂定时器，到期时下发 `kicked/tokenExpired` 并以 4011 关闭。
  2. `POST /admin/users/{userId}/force-logout`：把 `users.tokens_valid_after` 更新为当前时间；该用户所有在线连接下发 `kicked/banned` 并以 4012 关闭。
  3. 鉴权时：token 的签发时间早于 `tokens_valid_after` → `kicked/banned`，以 4012 关闭。
  4. hub 增加"按用户关闭所有连接"的能力。

### BE-014 演示登录服务
- 优先级：P0
- 依赖：BE-010
- AC：AC-AUTH-04-1、AC-AUTH-04-3、AC-AUTH-04-4
- 内容：`IM_DEMO_LOGIN=1` 时注册 `POST /demo/login {username}` → 校验格式（不合法返回 400 / 4004）→ 自动注册 → 返回 token；默认不注册该路由（返回 404）；允许 H5 开发服务器跨域访问（仅限演示模式）。

### BE-015 空闲超时与慢连接的关闭码
- 优先级：P0
- 依赖：BE-002
- AC：AC-CONN-01-1、AC-CONN-01-2
- 内容：空闲超时以 4408 关闭，慢连接以 4400 关闭；e2e 断言具体关闭码。

---

## B2 消息

### BE-020 单聊对方存在校验
- 优先级：P0
- 依赖：BE-010
- AC：AC-MSG-01-7、AC-MSG-01-8、AC-CONV-03-3
- 内容：发送单聊前校验对方用户存在，不存在返回 4042，不保存消息、不生成会话；给自己发消息照常成功。

### BE-021 去重校验会话一致性
- 优先级：P0
- 依赖：BE-002
- AC：AC-MSG-04-1、AC-MSG-04-2、AC-MSG-03-3、AC-MSG-03-4
- 内容：命中（发送者，clientMsgId）去重后，核对会话：不一致返回 4008；一致则返回第一次的 `msgId` / `seq` / `ts` / `convId`。

### BE-022 sync 往前翻页、limit 校验与权限错误码
- 优先级：P0
- 依赖：BE-001、BE-002
- AC：AC-MSG-05-2、AC-MSG-05-3、AC-MSG-05-4、AC-MSG-05-5、AC-CONN-01-4、AC-CONN-01-8
- 内容：
  1. `beforeSeq`：返回 seq < beforeSeq 的最近 limit 条，按升序排列，并返回 `hasMore`；与 `afterSeq` 同时出现或都不出现时返回 4000。
  2. `limit` 不在 1～200 之间时返回 4000。
  3. 权限：单聊非参与方返回 4032；群不存在返回 4043；非成员返回 4033。

### BE-023 发送限流
- 优先级：P0
- 依赖：BE-002
- AC：AC-MSG-07-1
- 内容：按用户维度（所有连接合计）做令牌桶限流，每秒 20 条，超出返回 4290；e2e 验证"25 条中 20 条成功、5 条返回 4290，1 秒后恢复"。

### BE-024 消息字段补齐
- 优先级：P0
- 依赖：BE-001
- AC：AC-MSG-01-6、AC-MSG-04-7
- 内容：send 的 ack 返回 `convId`；`msg` 帧和 `sync` 结果中的每条消息都带 `clientMsgId`。

---

## B2.5 架构与安全基座

设计依据：`docs/arch/ARCH-v1.md`。目标是让 B3 之后的代码一开始就按"可分布式、可换库、安全优先"的约束来写。

### BE-060 版本化数据库迁移
- 优先级：P0
- 依赖：—
- AC：（架构：ARCH 3.1）
- 内容：`store/migrations/sqlite/*.sql`（embed）+ `schema_migrations(version, applied_at, checksum)`；每个迁移一个事务；已执行文件的 checksum 变化时拒绝启动；`im-server migrate up|status`；`IM_AUTO_MIGRATE`（开发默认开，生产默认关）。现有建表语句改为 `0001_init.sql`，已有数据库可以无损升级。

### BE-061 存储契约测试
- 优先级：P0
- 依赖：BE-060
- AC：（架构：ARCH 3.2）
- 内容：`store/storetest` 共享测试集（seq 连续且并发不重复、去重、翻页边界、只前进的已读、事务原子性、令牌版本号）；SQLite 实现接入；预留 `IM_TEST_PG_DSN`。SQL 只使用 ARCH 3.3 列出的通用语法。

### BE-062 投递、在线路由、限流、缓存失效接口
- 优先级：P0
- 依赖：—
- AC：（架构：ARCH 2.2）
- 内容：新增 `delivery.Router`（`DeliverToUser`、`KickUser`）和 `cache.Invalidator`；`ratelimit.Limiter` 抽成接口；`logic`、`admin` 不再直接 import `hub`。一期只有本地实现，行为不变（现有 e2e 全部通过即为验收）。增加架构测试：`logic` 包的依赖里不能出现 `hub`。

### BE-063 备份与恢复
- 优先级：P0
- 依赖：BE-060
- AC：（架构：ARCH 3.4）
- 内容：`im-server backup -o <file>` 和 `POST /admin/backup`（`VACUUM INTO` 一致性快照，写入期间不阻塞收发消息）；`im-server restore`；自动化恢复演练测试：写入数据 → 备份 → 删除数据库 → 恢复 → 逐会话校验条数和最大 seq。备份加密和上传对象存储在部署脚本中完成（M7）。

### BE-064 生产启动保护、Origin 白名单、资源上限
- 优先级：P0
- 依赖：—
- AC：AC-AUTH-05-1（原 BE-051 的安全部分提前）
- 内容：S3、S4、S6——`IM_ENV=production` 时缺少密钥、密钥短于 32 字节、等于开发默认值、开启测试模式或演示登录都拒绝启动；`IM_ALLOWED_ORIGINS`（未配置时只允许本机；没有 Origin 头的原生端放行）；HTTP `ReadTimeout` / `WriteTimeout` / `IdleTimeout`；单用户最多 10 个连接，超出时踢掉最早的连接（新增踢线原因 `tooManyConnections`，需要更新 errors.json）。

### BE-065 WS 鉴权改为首帧 auth
- 优先级：P0
- 依赖：决策 A2、PRD v1.2 C-52
- AC：（新增，PRD v1.2）
- 内容：S1——握手 URL 只带 `platform`；第一帧必须是 `{"cmd":"auth","data":{"token":…}}`，5 秒内没有收到以 4010 关闭；auth 之前的其他命令返回 4010 并关闭；鉴权失败沿用"先升级再踢"的错误码与顺序（4009 → 4010 → 4012 → 4011）；auth 成功返回 ack（含 userId、服务端时间）。为平滑过渡，`IM_LEGACY_QUERY_TOKEN=1` 时仍接受 URL token（生产启动保护禁止开启）。同步修改 Schema、测试用例、现有 e2e。

### BE-066 握手限流与日志脱敏
- 优先级：P0
- 依赖：BE-062、BE-065
- AC：（架构：S7、S9）
- 内容：按 IP 的新建连接令牌桶和鉴权失败计数（连续失败临时封禁）；只信任 `IM_TRUSTED_PROXIES` 传来的 `X-Forwarded-For`；日志脱敏测试：发送带特征串的 token 和消息正文，断言整个测试过程的日志输出里都不出现。

### BE-067 JWT 密钥轮换
- 优先级：P0
- 依赖：—
- AC：（架构：S5）
- 内容：token 头部带 `kid`；`IM_JWT_KEYS=kid1:secret1,kid2:secret2`、`IM_JWT_ACTIVE_KID`；旧密钥只验证不签发；未知 `kid` 视为 token 无效。兼容 `IM_JWT_SECRET`（视为单把密钥）。

### BE-068 CI 安全扫描
- 优先级：P0
- 依赖：—
- AC：（架构：S11）
- 内容：GitHub Actions：`go test -race`、`go vet`、`govulncheck`、`gosec`、`go mod verify`、`tools/trace.py --check`；高危漏洞阻断合并。

### BE-069 就绪检查与优雅下线
- 优先级：P1
- 依赖：BE-062
- AC：（架构：ARCH 2.4）
- 内容：`/readyz`（数据库可用才就绪）；SIGTERM 时先把就绪状态置为失败，再给在线连接下发 `kicked/serverShutdown`，最后关闭。

---

## B3 群

### BE-030 群模型与建群 / 加成员规则
- 优先级：P0
- 依赖：BE-010、BE-002
- AC：AC-GRP-01-1、AC-GRP-01-2、AC-GRP-01-3、AC-GRP-01-10、AC-GRP-01-11、AC-GRP-02-2、AC-GRP-02-3
- 内容：
  1. `chat_groups` 增加 `history_visible`（默认 false）；`group_members` 增加 `join_seq`（入群时读取当前会话的 maxSeq）。
  2. 群名 1～50 字；成员可以为空；按加完后的总人数校验 ≤ 2000（超出返回 4130）；单次 ≤ 500（超出返回 4131）；成员不存在返回 4042，整批不生效；冲突返回 4090。
  3. 发送 / 拉取时先查群是否存在：不存在返回 4043，非成员返回 4033（修正 M1 中空群误报 4040 的问题）。

### BE-031 群管理接口补齐
- 优先级：P0
- 依赖：BE-030
- AC：AC-GRP-01-4、AC-GRP-01-5、AC-GRP-01-8
- 内容：
  - `DELETE /admin/groups/{groupId}/members`（body：`{members:[]}`）移除成员；
  - `PATCH /admin/groups/{groupId}` 修改 `name` / `historyVisible`；
  - `GET /admin/groups/{groupId}` 查询群名、设置和成员列表。
  - 被移除的成员不再收到新消息，发送和拉取都返回 4033。

### BE-032 群历史可见性
- 优先级：P0
- 依赖：BE-030、BE-022
- AC：AC-GRP-01-6、AC-GRP-01-7、AC-GRP-01-8、AC-GRP-01-9
- 内容：计算 `minSeq`：`historyVisible` 为 true 时为 1，否则为 `join_seq + 1`。`sync` 的两种方向都只返回 seq ≥ minSeq 的消息，翻页到 minSeq 时 `hasMore=false`；设置在查询时生效；被移除后再加入时重置 `join_seq`。

### BE-033 群成员内存缓存
- 优先级：P0
- 依赖：BE-031
- AC：AC-MSG-04-5
- 内容：群成员列表缓存在内存中，加成员、移除成员时失效；目的是避免每条群消息都查一次全部成员（性能需求见 BE-050）。

### BE-034 群成员变动事件
- 优先级：P0
- 依赖：BE-031、BE-040
- AC：AC-GRP-03-1、AC-GRP-03-2、AC-CONN-01-7
- 内容：加成员时，向被加入用户的在线连接下发 `convAdded`（会话信息结构与 `convs` 的单项相同）；移除时下发 `convRemoved {convId}`。

---

## B4 会话

### BE-040 会话列表 `convs`
- 优先级：P0
- 依赖：BE-001、BE-030
- AC：AC-CONV-01-1、AC-CONV-01-3、AC-CONV-01-4、AC-CONV-01-7、AC-CONN-01-7
- 内容：
  1. 新表 `user_conv(user_id, conv_id, join_seq, created_at)`：单聊在 send 事务里为双方 upsert；入群时写入，被移除时删除。
  2. 新表 `conv_last(conv_id, last_msg_id, last_from, last_text, last_ts)`：在 send 事务里更新（每个会话一行）。
  3. `convs` 命令返回 `{convId, type, name, maxSeq, readSeq, minSeq, unread, lastMsg, updatedAt}`，按 `updatedAt` 倒序、`convId` 升序排列，上限 500 个，超出时带 `truncated: true`（C-03）；`lastMsg` 摘要截断为 100 个码点，没有消息时为 `null`。
  4. 对已有数据做迁移：从 `messages` 和 `group_members` 回填这两张表。

### BE-041 未读数与已读规则
- 优先级：P0
- 依赖：BE-040、BE-032
- AC：AC-CONV-02-1、AC-CONV-02-2、AC-CONV-02-3、AC-CONV-02-5、AC-CONV-02-6、AC-GRP-01-12
- 内容：未读 = maxSeq − max(readSeq, joinSeq)（C-02，与 historyVisible 无关）；send 事务里把发送者的 readSeq 推进到新 seq（发言即已读）；`read` 上报的 seq 截断为 min(seq, maxSeq)，只进不退。

### BE-042 已读多端同步（P1）
- 优先级：P1
- 依赖：BE-041
- AC：AC-SYNC-02-1
- 内容：已读位置前进时，向该用户的其他在线连接下发 `event/readUpdated {convId, readSeq}`（需要同步补充协议）。

### BE-043 会话名使用昵称（P1）
- 优先级：P1
- 依赖：BE-040、BE-010
- AC：AC-USER-02-1
- 内容：单聊会话的 `name` 优先取对方昵称；群消息的发送者信息附带昵称（需要同步补充协议）。

---

## B5 非功能

### BE-050 压测工具与性能基线
- 优先级：P0
- 依赖：BE-033、BE-040
- AC：（非功能：PRD 1.3 节、第 6 节）
- 内容：`tools/loadtest`（Go）：1 万连接保持；单聊、群聊吞吐（目标 ≥ 500 条/秒）；端到端延迟 P99（目标 ≤ 1 秒）；`sync` 200 条耗时（目标 ≤ 200ms）。输出压测报告 `docs/reports/perf-*.md`。

### BE-051 可观测与安全配置
- 优先级：P0
- 依赖：BE-015
- AC：—（原 AC 已移到 BE-064）
- 内容：连接断开日志记录关闭码和原因；Prometheus 指标（在线连接数、收发消息速率、ack 延迟、错误码分布）。Origin 白名单和生产启动保护已提前到 BE-064。

---

---

## B6 集群化（二期，设计见 ARCH 第 2、3 节）

| 任务 | 内容 |
|---|---|
| BE-070 Postgres 存储实现 | 通过 BE-061 契约测试；`migrations/postgres`；连接池；`sync` 可走只读副本 |
| BE-071 数据迁移工具 | `im-server export` / `import`（JSON Lines），导入后逐会话校验条数、最大 seq、内容哈希抽样 |
| BE-072 NATS 投递 | `delivery.Router` 的集群实现；`cache.Invalidator` 经总线广播 |
| BE-073 Redis 在线路由与限流 | Presence（TTL 续期）；Lua 原子令牌桶 |
| BE-074 去重表拆分 | `client_msg_dedup` 按 `from_uid` 分片（ARCH 2.3） |
| BE-075 多节点 e2e | docker compose 起 3 个节点 + Postgres + NATS + Redis，跨节点收发、踢线、杀掉一个节点后客户端自动重连且不丢不重 |
| 安全 P1 | S8 管理接口签名与审计、S10 备份加密与字段加密、S13 保留期与注销、S14 内容审核回调 |

---

## 实现约定（回答用例编写阶段的遗留问题）

| # | 问题 | 约定 | 相关用例 |
|---|---|---|---|
| IMP-1 | `GET /admin/users/{userId}` 查询不存在的用户 | 返回 HTTP 404 / 4042 | TC-USER-01-12/13/16 |
| IMP-2 | `ttlSeconds` 传了非整数或字符串 | 返回 HTTP 400 / 4000 | — |
| IMP-3 | 去重命中的重发恰好遇上令牌桶为空 | **先查重，再扣令牌**：命中重复时直接返回原 ack，不扣令牌，也不会返回 4290（与 C-29 一致） | TC-MSG 中 C-29 相关用例 |
| IMP-4 | 新用户令牌桶的初始状态；新建连接是否影响 | 初始为满（20 个令牌）；令牌桶按用户维度统计，新建连接不重置也不增加 | TC-MSG-07-* |
| IMP-5 | 同一请求里重复出现同一个 userId | 先去重，计数按 1 个算（500 / 2000 两个上限都是） | TC-GRP-01-* |
| IMP-6 | groupId 和群名都不合法；4090 和 4042 的先后 | 先校验 ID 格式（4004），再校验其他参数（4000）；建群时先判断群是否已存在（4090），再校验成员是否存在（4042） | TC-GRP 中 C-08 相关用例 |
| IMP-7 | `POST /admin/test/clock` 是冻结时间还是设置偏移 | **冻结**：设置后服务端的"当前时间"固定为该值，直到再次设置；传 `{"nowMs": null}` 恢复真实时钟。对服务端所有用到时间的地方生效：消息时间戳、token 过期、限流、会话排序 | TC-CONV-01-07/10/11 |
| IMP-8 | C-11 要求用毫秒级签发时间判断"强制下线前签发的 token"，但 e2e 测试发现：签发和强制下线落在**同一毫秒**时无法区分。用 `<` 比较会漏掉同毫秒内签发的旧 token，用 `≤` 又会误伤强制下线后同毫秒内重新签发的新 token | **改用令牌版本号**：`users.token_version` 在强制下线时加 1，签发 token 时把当前版本写进 `tv` 字段，校验时 `tv < 当前版本` 即作废。效果满足 C-11 的意图（新 token 必须可用、旧 token 必须作废），且与时间精度无关；`iatMs` 保留，仅用于日志 | TC-AUTH-03-*（C-11 相关用例的断言不变） |
