# PRD：社交关系（好友、拉黑、通讯录、私聊与群聊补缺）

| 项 | 内容 |
|---|---|
| 版本 | **v0.1（草稿，待评审）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-03 |
| 状态 | 草稿。评审通过、待拍板项（第 14 节 S-4～S-7）拍板后冻结为 v1.0，并作为 **PRD-core v1.3** 的变更合入 |
| 依据 | [拍板清单](../review/DECISIONS-2026-11-03.md) 2.5 节 S-1～S-3、X-5、X-6、X-8、U-2、U-4；[PRD-core v1.2](PRD-core-v1.2.md)；[PRD-ui v0.1](PRD-ui-v0.1.md)；[PRD-admin v0.1](PRD-admin-v0.1.md)；[ARCH-v1](../arch/ARCH-v1.md)；[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[管理接口](../../protocol/ADMIN.md)；[SDK 设计说明](../sdk/README.md) |
| 关联 | 待拍板 S-4～S-7（第 14 节）；PRD-core v1.3 变更条目（第 13.1 节） |

---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 好友模式（S-1） | 服务端一个开关 `mode = open | friends`，**默认 `open`**（保证 PRD-core 现有 AC 和 e2e 不变）。开放模式：任何人可私聊，好友只用于整理通讯录，陌生人在对方回复前最多发 3 条。好友制：**互为好友**才能私聊，发送时实时判定 |
| "业务方"粒度 | 已核实 im-server 是**单租户**：只有一把 `IM_ADMIN_KEY`，`users` 表没有 `app_id`，userId 全局唯一，PRD-admin 也写明"一期一个后台对应一个 im-server 部署"。推荐**按部署配置**（一个部署 = 一个用户空间 = 一个业务方），配置存数据库、管理接口可改。是否要多租户是 **S-4** |
| 删除好友 | **单向删除**（同微信）：只删自己这一侧，对方不收通知、列表里仍有你。好友制下任一方删除后双方都不能再私聊 |
| 拉黑是否暴露 | 同微信：被拉黑方**发消息**返回 4036"消息已发出，但被对方拒收了"；**发好友申请**返回成功（假装待验证），对方收不到；搜索照常能搜到 |
| 群聊里拉黑 | 不生效（不影响收发群消息）；只影响"拉黑方被对方邀请进群" |
| 离线补拉 | 每个用户一个**关系版本号 `ver`**（单调递增，类似会话的 seq）。好友、申请、黑名单的每次变化都带新 ver；客户端每次连上后 `socialSync {sinceVer}` 增量拉取。实时事件 `socialChanged` 只是提醒，丢了不影响正确性（ARCH 2.2 约束 4） |
| 群角色 | 群主 / 管理员 / 成员。**客户端建的群（自建群）有群主；业务方建的群（托管群）默认没有群主**，客户端只能看、发言、退群；业务方可以通过管理接口指定群主，把托管群交给用户管理 |
| 邀请入群 | 任何成员都能邀请**互为好友**的人，直接入群、不需要对方同意（同微信 40 人以下的做法）；入群审批、"被拉进群需我同意"放 P1 |
| 群主退群 | 自动转让给最早入群的管理员，没有管理员时给最早入群的成员；最后一个人退群时群自动解散 |
| 新错误码 | 403x 3 个、404x 2 个、409x 1 个、413x 5 个、429x 1 个，共 12 个（第 9.4 节）。**403x 号段只剩 4038、4039**，需和 PRD-admin 一起定稿（S-5） |
| 工作量 | 服务端约 21 人日，sdk-ts 7，H5 10，sdk-kotlin / sdk-swift 各 6（在各自首版之上），测试 5，合计约 **55 人日**，明显高于 DECISIONS 2.5 节的估计（服务端 1.5～2 周），原因是群聊补缺（第 5 节）原本不在估计里。排期见 S-7 |

---

## 1. 背景与目标

### 1.1 背景

一期只做了"能聊"：任何登录用户可以给任何存在的 userId 发私聊（`logic.go` 第 117 行只校验对方存在）；群只能由业务方通过 `/admin/*` 建和管理；没有好友、拉黑、通讯录、用户搜索。PRD-core 第 7 节把"App 内建群、通讯录、好友"放在三期，PRD-ui 的通讯录因此只能"从单聊会话派生"（U-2）。

2026-11-03 Frank 拍板 S-1～S-3：好友模式按业务方配置、关系来源两者都支持、全部第一批做（M3，分享会前）。

### 1.2 目标

1. 让独立 IM App 成为一个完整的社交聊天软件：加好友、拉黑、通讯录、自己建群和管理群。
2. 让接入方（fx-sport 等）按自己的产品形态选择"开放私聊"或"好友制"，并能把自己系统里的关系（关注、好友）导入 IM。
3. 不破坏一期已冻结的规则：消息不丢不重、多端一致、分布式就绪（ARCH）、安全优先。

### 1.3 成功标准

| 指标 | 目标 | 度量方法 |
|---|---|---|
| 正确性 | 第 4、5 节全部 AC 通过；两种模式各跑一遍 PRD-core 的私聊 e2e | 测试用例 TC-SOC-* |
| 多端一致 | 一端同意好友申请，另一端 2 秒内列表更新；断线期间的关系变化重连后补齐 | AC-SOC-06-* |
| 隐私 | 搜索、批量查资料、好友申请三条路径都无法批量确认"某个 userId 是否存在" | 安全用例（第 10 节） |
| 性能 | 好友制下私聊发送 P99 增加 ≤ 2ms（多两次主键查询）；`socialSync` 全量 2000 好友 P99 ≤ 200ms | 压测（沿用 C-48 口径） |
| 演示 | 分享会现场：搜索加好友 → 对方同意 → 开聊；拉黑后消息被拒收；建群、邀请、转让群主 | 彩排 |

---

## 2. 角色与场景

| 角色 | 说明 |
|---|---|
| 终端用户 | 加好友、拉黑、建群、管理自己建的群 |
| 群主 / 群管理员 | 自建群的管理者（第 5 节） |
| 业务服务端 | 选择好友模式；导入、增删好友关系；建托管群、指定群主（第 8 节） |
| 管理后台运营 | 通过后台服务调用同一套管理接口（PRD-admin） |

核心场景：
1. 小明在独立 App 里搜索小红的账号，发送好友申请"我是小明"；小红在"新的朋友"里看到红点，点同意；小明的手机和电脑同时出现小红，可以开始聊天。
2. fx-sport 选择好友制，把用户的互关关系批量导入 IM；没有互关的用户之间不能私聊。
3. 小明被陌生人骚扰，拉黑对方；对方再发消息提示"被对方拒收"，加好友对方也收不到。
4. 小明建一个 5 人的群，拉进 3 个好友，改群名、发群公告；有事要离开时把群主转让给小红。
5. 业务方建了一个 2000 人的活动群（托管群），成员只能聊天和退群；业务方指定一个运营账号做群主后，该账号可以在 App 里踢人、发公告。

---

## 3. 范围

### 3.1 功能点总览

| 编号 | 功能点 | 优先级 | 本次第一批（M3） | 涉及 |
|---|---|---|---|---|
| F-SOC-01 | 社交模式与业务方配置 | P0 | ✅ | 服务端 / 管理接口 |
| F-SOC-02 | 发送好友申请 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-03 | 处理好友申请（同意、拒绝、忽略、过期） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-04 | "新的朋友"列表与红点 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-05 | 删除好友与备注名 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-06 | 关系同步：好友列表、多端同步、离线补拉 | P0 | ✅ | 服务端 / SDK |
| F-SOC-07 | 拉黑与黑名单 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-08 | 私聊权限（两种模式、陌生人限额、新会话限流） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-09 | 用户搜索与"可被搜索"开关 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-10 | 批量查用户资料（U-4）与访问范围 | P0 | ✅ | 服务端 / SDK |
| F-SOC-11 | 通讯录数据（好友 + 我的群聊 + 新的朋友入口） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-12 | 业务方管理好友关系（导入、删除、查询） | P0 | ✅ | 服务端 / 管理接口 |
| F-SOC-20 | 群角色与群资料（群主、管理员、成员数、公告） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-21 | 客户端建群 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-22 | 邀请入群 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-23 | 退群（含群主退群） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-24 | 移出成员 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-25 | 转让群主与设置管理员 | P0 | ✅（管理员可按 S-7 延后） | 服务端 / SDK / H5 |
| F-SOC-26 | 解散群 | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-27 | 修改群名与群公告 | P0 | ✅（公告可按 S-7 延后） | 服务端 / SDK / H5 |
| F-SOC-28 | 群成员列表（分页） | P0 | ✅ | 服务端 / SDK / H5 |
| F-SOC-29 | 群提示消息（tip） | P0 | ✅ | 协议 / 服务端 / SDK / H5 |
| F-SOC-30 | 托管群与管理接口交接 | P0 | ✅ | 服务端 / 管理接口 |

### 3.2 P1（不在第一批）

| 功能 | 说明 |
|---|---|
| 入群审批（群设置"邀请需群主或管理员确认"）、个人设置"被拉进群需要我同意" | 第一批靠"只能邀请互为好友的人"控制骚扰 |
| 业务方提供的搜索号（`searchId`，如"IM 号""手机号"） | 第一批只按 userId 精确搜索 |
| 陌生人消息收纳（单独的"陌生人消息"入口） | 第一批只做条数限额 |
| 群提示消息不计入未读 | 第一批计入（第 F-SOC-29 规则 5） |
| 从"新的朋友"里删除记录、好友分组 / 标签、星标朋友 | — |
| 管理接口：查询 / 导入黑名单、按用户设置"可被搜索" | 第一批业务方只能设默认值 |
| 群管理员禁言、全群禁言、群二维码、按链接入群 | 依赖 PRD-admin 第二批的禁言 |

### 3.3 不做

- 关注 / 粉丝（单向关系）：业务方如需，可以在自己系统里维护，按"互关"导入为好友。
- 附近的人、摇一摇、推荐好友、通讯录（手机联系人）匹配：隐私风险大，与 IM 公共库定位不符。
- 用户注销：沿用 PRD-core F-USER-01 规则 4（一期不删用户），注销单独立 PRD（ARCH S13）。

---

## 4. 术语

| 术语 | 定义 |
|---|---|
| 业务方 | 一个 im-server 部署对应的接入方（S-4 推荐：按部署）。同一部署内 userId 全局唯一，配置全局生效 |
| 好友边 | 有向关系"A 把 B 加为好友"，存在 A 这一侧（`friends(owner=A, peer=B)`）。备注名存在边上 |
| 互为好友 | A→B、B→A 两条边都存在（且未删除） |
| 陌生人 | 不互为好友的两个人 |
| 关系版本号 `ver` | 每个用户一个，从 0 开始，**该用户**的好友边、好友申请、黑名单、隐私设置每变化一次加 1。用于增量同步，语义类似会话的 seq |
| 好友申请 | 从申请人到被申请人的一条记录。同一对人、同一方向只有一条（重复申请合并） |
| 自建群 | 客户端用 `groupCreate` 建的群，groupId 以 `ug-` 开头，有群主 |
| 托管群 | 业务方用 `POST /admin/groups` 建的群，默认没有群主（`owner = null`） |
| 群提示消息（tip） | 服务端在群里生成的灰色提示（"A 邀请 B 加入了群聊"），是一条占用 seq 的消息，`type = "tip"` |
| 可见范围 | 一个用户能读取哪些人的资料（F-SOC-10） |

---

## 5. 通用约定

### 5.1 业务方可配置参数（F-SOC-01）

全部参数存在数据库 `app_settings`，管理接口可改（第 8 节），没配置时用默认值。

| 参数 | 含义 | 默认 | 范围 |
|---|---|---|---|
| `mode` | 好友模式 | `open` | `open` / `friends` |
| `strangerMsgLimit` | 开放模式下，对方回复前最多发几条 | 3 | 0～100，0 表示不限制 |
| `newPeerHourlyLimit` | 每人每小时最多向几个"新的单聊对象"发消息（两种模式都生效，F-SOC-08） | 30 | 1～1000 |
| `maxFriends` | 每人好友上限（自己这一侧的边数） | 2000 | 100～5000 |
| `maxBlocks` | 黑名单上限 | 1000 | 100～5000 |
| `requestExpireDays` | 好友申请有效期 | 7 | 1～30 |
| `applyDailyLimit` | 每人每天最多发几次好友申请（重复申请也算） | 30 | 1～500 |
| `applyPerTargetDailyLimit` | 每天对同一个人最多申请几次 | 3 | 1～10 |
| `searchableDefault` | 用户没设置过时是否可被搜索 | `true` | 布尔 |
| `searchDailyLimit` | 每人每天最多搜索几次 | 100 | 10～1000 |
| `clientGroupEnabled` | 是否允许客户端建群 | `true` | 布尔 |
| `clientGroupMaxMembers` | 自建群人数上限（含群主） | 500 | 3～2000 |
| `maxJoinedGroups` | 每人通过客户端操作最多加入几个群（建群、被邀请） | 500 | 10～2000 |
| `groupCreateDailyLimit` | 每人每天最多建几个群 | 10 | 1～100 |

固定值（不开放配置）：群管理员最多 10 人；好友申请附言 0～50 个码点；备注名 0～32 个码点（与昵称一致）；群公告 0～1000 个码点；群名 1～50 个码点（同 C-38）；建群时除自己外至少 2 人；单次邀请 / 移出最多 50 人；`users` 单次最多 100 个；`groupMembers` 单次最多 100 个。文本一律按 PRD-core 3.1 节先去首尾空白、再按码点计数。

### 5.2 限流（复用 `ratelimit.Limiter`）

| 键 | 速率 | 超出 |
|---|---|---|
| `friendApply:{uid}` | 每秒 1 个，突发 5 | 4290 |
| `friendApplyDay:{uid}`、`friendApplyDay:{uid}:{peer}` | `applyDailyLimit` / `applyPerTargetDailyLimit` 每天 | 4292 |
| `search:{uid}` | 每分钟 10 个，突发 10 | 4290 |
| `searchDay:{uid}` | `searchDailyLimit` 每天 | 4292 |
| `users:{uid}` | 每分钟 30 个，突发 10 | 4290 |
| `social:{uid}`（删除、备注、拉黑、取消拉黑、隐私设置、标记已读） | 每秒 5 个，突发 20 | 4290 |
| `groupOp:{uid}`（邀请、移出、转让、设管理员、改资料、退群、解散） | 每秒 5 个，突发 20 | 4290 |
| `groupCreateDay:{uid}` | `groupCreateDailyLimit` 每天 | 4292 |
| `newPeer:{uid}` | `newPeerHourlyLimit` 每小时（只在"发给一个没有单聊记录的人"时扣） | 4290 |

规则：被拒绝的请求不扣令牌（C-29）；"每天"按 `NewPer(24h/N, N)` 的令牌桶近似，不按自然日清零（避免零点集中）；一期为内存实现、重启清零，集群化时换 Redis（ARCH G2）。

### 5.3 多错误优先级（C-08 风格）

所有新命令统一：帧 / 参数格式（4002、4004、4000）→ 限流 4290 / 配额 4292 → 资源存在性（4042、4043、4045）→ 关系与权限（4033、4030、4035、4036、4046）→ 数量上限（413x）。例外：`send` 见 F-SOC-08 规则 7。

---

## 6. 功能需求：好友、拉黑、通讯录、私聊

> 写法同 PRD-core 第 4 节：规则 → 验收标准。【后端】【前端】表示主要验证层，未标注的为端到端。
> AC 中的 alice、bob、carol、dave 都是已导入的用户；"好友制下"指 `mode=friends`，"开放模式下"指 `mode=open`。

### F-SOC-01 社交模式与业务方配置（P0）

**规则**
1. 配置粒度：**一个 im-server 部署一套配置**（S-4）。参数见 5.1 节，由管理接口 `GET / PATCH /admin/settings/social` 读写。
2. 修改即时生效：服务端缓存配置，修改后通过 `cache.Invalidator`（主题 `socialSettings`）通知所有节点，TTL 兜底 60 秒。
3. 模式切换不改任何已有数据：`open → friends` 后，非好友之间的历史会话仍在会话列表里、可以拉历史，但不能再发（4035）；`friends → open` 后立即恢复。
4. 客户端在 `socialSync` 应答里拿到 `mode`、`strangerMsgLimit`（只读），用于界面提示；**判定一律以服务端为准**。
5. 非法配置（超出 5.1 节范围、未知字段、`mode` 不是两个值之一）返回 HTTP 400 / 4000，整次修改不生效。

**验收标准**
- AC-SOC-01-1【后端】：新部署 `GET /admin/settings/social` 返回 5.1 节的全部默认值，`mode=open`。
- AC-SOC-01-2【后端】：`PATCH {"mode":"friends"}` 成功后，alice 立即给非好友 bob 发私聊返回 4035（无需重启，1 秒内生效）。
- AC-SOC-01-3【后端】：`PATCH {"mode":"friends","maxFriends":99}` 返回 400 / 4000，再次 GET 时 `mode` 仍为 `open`。
- AC-SOC-01-4【后端】：双节点测试环境中，在节点 1 修改 `mode`，连在节点 2 的用户 1 秒内按新模式判定（失效通知生效）；关掉失效广播后 60 秒内也按新模式判定（TTL 兜底）。
- AC-SOC-01-5【后端】：alice 和 bob 在开放模式下聊过 5 条，切到好友制后，alice 拉 `convs` 仍有该会话，`sync` 能拉到 5 条，`send` 返回 4035；切回开放模式后 `send` 成功。

### F-SOC-02 发送好友申请（P0）

**规则**
1. 命令 `friendApply {to, message?, source?, sourceRef?, remark?}`：
   - `message`：附言 0～50 码点（去首尾空白后）；`remark`：同意后给对方的预设备注名 0～32 码点。
   - `source`：`search`（搜索）、`group`（群聊）、`card`（名片 / 资料卡）、`conv`（单聊会话）、`other`，默认 `other`。`source=group` 时必须带 `sourceRef=groupId`，服务端校验双方都在这个群里，不满足时按 `other` 记录（不报错）。
2. 结果（ack `{status, ver}`）：
   | 情况 | 结果 |
   |---|---|
   | 已互为好友 | `status=accepted`，不产生新记录 |
   | 对方这一侧仍有我（我之前单向删了对方） | 直接恢复我这一侧的好友边，`status=accepted`，对方不收申请（同微信） |
   | 对方有一条发给我的、未过期的待处理申请 | **自动成为好友**，两条申请都变为已同意，`status=accepted` |
   | 对方把我拉黑了 | 返回 `status=pending`（**与正常申请完全相同**），但对方收不到、也不会出现在对方的"新的朋友"里 |
   | 其他 | 创建或合并申请，`status=pending` |
3. **重复申请合并**：同一申请人对同一人只有一条申请。待处理期间再次申请：附言、来源、预设备注更新为最新一次，`expireAt` 重新计算，申请次数 +1，对方列表中这条移到最前并重新计入红点（F-SOC-04）。被拒绝或过期后再次申请，同样复用这一条，状态回到待处理。
4. 有效期：`expireAt = 最后一次申请时间 + requestExpireDays`。过期由读取时按服务端时间判定，不需要定时任务。
5. 不能申请自己（4000）；对方不存在 4042（计入每日配额，防止用申请探测账号）；自己好友已满 4132；对方好友已满**不在申请时报错**（避免暴露对方信息），在对方同意时报 4133。
6. 每次状态变化：申请人、被申请人各自的 `ver` 加 1，并给双方在线连接（申请人除发起的连接外）下发 `socialChanged`。
7. 日志不记录附言和备注原文，只记录长度。

**验收标准**
- AC-SOC-02-1：alice 给 bob 发申请（附言"我是 alice"，来源 `search`），ack `status=pending`；bob 在线的两个连接 1 秒内收到 `socialChanged`，`socialSync` 后 bob 的申请列表里有 alice，方向 `in`，附言"我是 alice"，来源 `search`。
- AC-SOC-02-2【后端】：alice 在 1 天内给 bob 申请 3 次（附言依次为 a、b、c），bob 的申请列表里 alice 只有一条，附言为 c，`applyCount=3`；第 4 次返回 4292。
- AC-SOC-02-3【后端】：alice 给 bob 发了申请（未处理），bob 再给 alice 发申请，bob 的 ack 为 `status=accepted`；双方 `socialSync` 后好友列表互相包含对方，两条申请状态都是 `accepted`。
- AC-SOC-02-4【后端】：bob 把 alice 拉黑后，alice 给 bob 发申请，ack 与 AC-SOC-02-1 字段完全相同（`status=pending`）；bob 没有收到 `socialChanged`，bob 的申请列表里没有 alice。
- AC-SOC-02-5【后端】：给自己发申请返回 4000；给不存在的 `nobody` 发申请返回 4042；附言 51 个码点返回 4000；`to` 为 `a b` 返回 4004。
- AC-SOC-02-6【后端】：alice 已有 2000 个好友（`maxFriends=2000`）时申请 bob，返回 4132；bob 已有 2000 个好友时，alice 申请成功（pending），bob 同意时返回 4133，申请仍为待处理。
- AC-SOC-02-7【后端】：`source=group, sourceRef=g1`，alice 和 bob 都在 g1 时记录来源 `group`；bob 不在 g1 时记录为 `other`，申请仍成功。
- AC-SOC-02-8【后端】：alice 和 bob 互为好友后，bob 单向删除 alice；alice 再申请 bob，返回 `pending`，bob 收到申请。反过来，alice 单向删除 bob 后再申请 bob（bob 仍有 alice），返回 `accepted`，bob 没有收到申请，alice 的好友列表恢复 bob。
- AC-SOC-02-9【后端】：alice 在 1 秒内连续发 6 个申请（给 6 个不同的人），前 5 个成功，第 6 个返回 4290；同一天内第 31 个申请返回 4292（`applyDailyLimit=30`）。
- AC-SOC-02-10【后端】：日志扫描：附言为特征串 `ZXQ-SOCIAL-123` 的申请处理完后，服务端日志中搜不到该特征串（同 ARCH S9 的测试方式）。

### F-SOC-03 处理好友申请（P0）

**规则**
1. 命令 `friendRespond {from, action: "accept" | "reject", remark?}`，只能处理发给自己的申请。
2. **同意**：在一个事务里创建双方的好友边（申请人一侧的备注 = 申请时的预设备注；被申请人一侧 = 本次的 `remark`），两条申请记录变为 `accepted`。双方 `ver` 加 1，双方在线连接收到 `socialChanged`。
3. **拒绝**：只改被申请人一侧的记录为 `rejected`；**申请人一侧仍显示"等待验证"直到过期**（不告诉申请人被拒绝，同微信）。申请人之后可以再次申请（受每日配额限制）。
4. **忽略**：不操作即可，过期后双方都显示"已过期"。
5. 幂等：已同意的再同意返回成功；已拒绝的可以改为同意（只要未过期）；已同意的再拒绝返回 4091（已经是好友，要用删除）。
6. 申请不存在、已过期、不是发给我的，统一返回 4045（不区分，防止探测）。
7. 同意时自己好友已满 4132，对方好友已满 4133；两种情况都不改任何数据。

**验收标准**
- AC-SOC-03-1：bob 同意 alice 的申请（`remark="同学A"`），双方在线连接 1 秒内收到 `socialChanged`；`socialSync` 后 bob 的好友列表中 alice 的备注为"同学A"，alice 的好友列表中有 bob。
- AC-SOC-03-2【后端】：alice 申请时带 `remark="B哥"`，bob 同意后 alice 一侧 bob 的备注为"B哥"。
- AC-SOC-03-3【后端】：bob 拒绝后，bob 一侧该申请为 `rejected`，alice 一侧仍为 `pending`、`expireAt` 不变；alice 没有收到 `socialChanged`。
- AC-SOC-03-4【后端】：申请发出 7 天零 1 秒后（测试时钟），bob 同意返回 4045；双方 `socialSync` 中该申请都显示为 `expired`。
- AC-SOC-03-5【后端】：bob 拒绝后、未过期前改为同意，成功成为好友；之后再拒绝返回 4091。
- AC-SOC-03-6【后端】：bob 对 carol 并没有发过的申请调用同意，返回 4045；对已过期的申请调用同意，同样返回 4045，两者响应体相同。
- AC-SOC-03-7【后端】：同一条申请重复同意 2 次，第二次返回成功，好友边只有一条，`ver` 只增加一次。

### F-SOC-04 "新的朋友"列表与红点（P0）

**规则**
1. 列表数据来自 `socialSync` 的 `requests`：包括我收到的（`in`）和我发出的（`out`），最近 30 天内有变化的，按 `updatedAt` 倒序。状态显示：`pending`（收到的显示"同意"按钮，发出的显示"等待验证"）、`accepted`（"已添加"）、`rejected`（只有收到的一侧会出现，显示"已拒绝"）、`expired`（"已过期"）。
2. 红点数 = 我收到的、状态为 `pending`、未过期、且 `ver > requestSeenVer` 的申请条数。
3. 进入"新的朋友"页面时调用 `friendRequestsSeen {ver}`，服务端把 `requestSeenVer` 推进到 `min(ver, 当前 ver)`，只进不退；给我的其他在线连接下发 `socialChanged`，其他端红点清零（多端同步）。
4. 合并后的重复申请（F-SOC-02 规则 3）会得到新的 `ver`，因此重新计入红点。
5. 30 天前的申请记录由后台清理任务物理删除（不影响好友关系）。

**验收标准**
- AC-SOC-04-1：carol、dave 先后给 bob 发申请，bob 的 SDK 红点数为 2，列表中 dave 在 carol 之前。
- AC-SOC-04-2：bob 在 web 端进入"新的朋友"后，bob 的 pc 端红点 2 秒内变为 0。
- AC-SOC-04-3【后端】：bob 标记已读后，carol 再次申请 bob（合并），bob 的红点数变为 1。
- AC-SOC-04-4【后端】：`friendRequestsSeen {ver: 5}` 之后再调用 `{ver: 3}`，`requestSeenVer` 仍为 5；传入比当前 ver 大的值时，按当前 ver 记录。
- AC-SOC-04-5【前端】：已过期的申请显示"已过期"，不显示"同意"按钮，不计入红点。
- AC-SOC-04-6【后端】：31 天前更新过的申请不再出现在 `socialSync` 的全量结果中。

### F-SOC-05 删除好友与备注名（P0）

**规则**
1. `friendDelete {userId}`：**单向删除**——只删除我这一侧的好友边（写墓碑，见 F-SOC-06），备注一并删除；对方不收任何通知，对方列表里我仍然在。
2. 删除不影响会话：会话仍在会话列表里、历史可拉；好友制下删除后**双方**都不能再私聊（不再互为好友，F-SOC-08）。
3. 删除不是好友的人：幂等成功，不改变 `ver`。
4. `friendRemark {userId, remark}`：0～32 码点，空字符串表示清除；对方不是我这一侧的好友时返回 4040。备注只有自己可见，多端同步（`ver` 加 1，`socialChanged`）。
5. 删除后，双方之间的申请记录保持原状（`accepted`），重新加好友走 F-SOC-02。

**验收标准**
- AC-SOC-05-1：alice、bob 互为好友，alice 删除 bob：alice 的好友列表没有 bob，bob 的好友列表仍有 alice，bob 没有收到 `socialChanged`。
- AC-SOC-05-2【后端】：好友制下 AC-SOC-05-1 之后，alice 给 bob 发私聊返回 4035，bob 给 alice 发私聊也返回 4035；双方 `sync` 该会话仍能拉到历史。
- AC-SOC-05-3【后端】：删除一个不是好友的人返回成功，`ver` 不变。
- AC-SOC-05-4：alice 在 web 端把 bob 的备注改为"老板"，alice 的 pc 端 2 秒内显示"老板"；bob 的任何接口都拿不到这个备注。
- AC-SOC-05-5【后端】：备注 33 个码点返回 4000；给不是好友的 carol 设置备注返回 4040；备注为"  "（去空白后为空）时清除备注。

### F-SOC-06 关系同步：好友列表、多端同步、离线补拉（P0）

**规则**
1. 版本号：每个用户一个 `ver`。以下变化让**该用户**的 `ver` 加 1，并把新 ver 写到变化的那一行上：我的好友边新增 / 删除 / 改备注；我收到或发出的申请新建 / 合并 / 状态变化；我的黑名单新增 / 删除；我的隐私设置变化。ver 的分配和数据修改在同一个事务里（同 seq 的分配方式）。
2. 删除用**墓碑**：好友边、黑名单删除时不物理删除，标记 `deleted` 并更新 ver。墓碑保留 30 天，清理后记录 `purgedVer`。
3. 命令 `socialSync {sinceVer}`：
   - 返回 `ver` 大于 `sinceVer` 的全部变化（好友、申请、黑名单，含墓碑），按 ver 升序，单次最多 1000 项，`hasMore=true` 时客户端用返回的 `ver` 继续拉。
   - `sinceVer = 0` 或 `sinceVer < purgedVer` 时返回全量（`full=true`，不含墓碑），客户端用它替换本地。`sinceVer` 大于当前 ver 时同样返回全量（客户端数据异常）。
   - 每次都返回：`requestSeenVer`、`settings {searchable}`、`mode`、`strangerMsgLimit`。
   - 每一项带 `profile {nickname, avatar}`（应答时的最新值）；昵称头像变化**不改变** ver，SDK 按 F-SOC-10 刷新。
4. 实时提醒：事件 `socialChanged {ver}`，只是提醒"有新变化"。投递尽力而为；SDK 收到后若 `ver > 本地 ver`，200ms 去抖后 `socialSync`。
5. 发起操作的连接从 ack 里拿到新 `ver` 和变更后的数据，直接更新本地，不需要再拉（与"自己发的消息不下发给发送连接"一致）。
6. SDK 每次连接成功（含重连）：先 `convs`，再 `socialSync {sinceVer: 本地 ver}`，直到 `hasMore=false`。
7. 好友列表的排序、按首字母分组由客户端完成（PRD-ui），服务端不排序。

**验收标准**
- AC-SOC-06-1【后端】：bob 一个连接都不在线时，carol、dave 给他发申请、alice 同意了 bob 之前的申请；bob 上线后 `socialSync {sinceVer: 断线前的 ver}` 一次拿到这 3 项变化，`ver` 比断线前大 3。
- AC-SOC-06-2：alice 在 pc 端同意 bob 的申请，alice 的 web 端 2 秒内好友列表出现 bob；web 端断网期间发生的同意，web 重连后 5 秒内补齐。
- AC-SOC-06-3【后端】：故障注入丢弃 `socialChanged` 事件（skip-delivery 同类注入），客户端重连后的 `socialSync` 仍能拿到全部变化（正确性不依赖事件）。
- AC-SOC-06-4【后端】：alice 有 2500 项变化时，`socialSync {sinceVer: 0}` 返回 `full=true`；增量拉取时第一次返回 1000 项、`hasMore=true`，连续 3 次拉完，项目不重复、不遗漏，ver 严格递增。
- AC-SOC-06-5【后端】：alice 删除 bob 后，`socialSync {sinceVer: 删除前的 ver}` 返回 bob 的墓碑（`deleted=true`）；31 天后（测试时钟，清理任务执行后）用同一个 sinceVer 拉取，返回 `full=true`，结果中没有 bob。
- AC-SOC-06-6【后端】：同一用户两个连接并发处理 10 个申请，最终 ver 恰好增加 10，`socialSync` 中每项的 ver 互不相同。
- AC-SOC-06-7【后端】：`sinceVer` 为负数或不是整数返回 4000；`sinceVer` 大于当前 ver 时返回 `full=true`。
- AC-SOC-06-8【后端】：2000 个好友的全量 `socialSync`，P99 ≤ 200ms（C-48 同一基准机器）。

### F-SOC-07 拉黑与黑名单（P0）

**规则**
1. `block {userId}` / `unblock {userId}`；黑名单在 `socialSync` 的 `blocks` 里，多端同步。
2. 拉黑**不改变好友关系**（同微信）：好友列表里对方仍在，界面标记"已拉黑"。
3. A 拉黑 B 之后：
   | B 的操作 | 结果 |
   |---|---|
   | 给 A 发私聊 | 4036（"消息已发出，但被对方拒收了"），消息不保存、A 收不到 |
   | 给 A 发好友申请 | 假装成功（F-SOC-02 规则 2） |
   | 邀请 A 进群 | A 进入 `failed`，错误码 4036 |
   | 搜索 A、查 A 的资料 | 照常（不暴露拉黑） |
   | 在共同的群里发言 | 照常，A 照常收到（**群里拉黑不生效**） |
   A 自己给 B 发私聊不受影响（开放模式下照常；好友制下仍要求互为好友）。
4. 拉黑时，B 发给 A 的待处理申请自动变为 `rejected`（A 一侧）。取消拉黑不恢复任何东西。
5. 不能拉黑自己（4000）；对方不存在 4042；黑名单满 4134；重复拉黑、取消一个不在黑名单的人都是幂等成功。

**验收标准**
- AC-SOC-07-1：alice 拉黑 bob 后，bob 给 alice 发私聊返回 4036，alice 的所有连接都没有收到消息，alice `sync` 该会话也没有这条；bob 的 SDK 把消息标记失败，提示"消息已发出，但被对方拒收了"。
- AC-SOC-07-2【后端】：alice 拉黑 bob 后，alice 给 bob 发私聊成功（开放模式）。
- AC-SOC-07-3【后端】：alice、bob 都在 g1，alice 拉黑 bob 后，bob 在 g1 发的消息 alice 照常收到。
- AC-SOC-07-4【后端】：bob 有一条发给 alice 的待处理申请，alice 拉黑 bob 后，alice 一侧该申请为 `rejected`，alice 的红点减 1。
- AC-SOC-07-5：alice 在 web 拉黑 bob，pc 端 2 秒内黑名单出现 bob；取消拉黑后两端都移除，bob 再发私聊成功。
- AC-SOC-07-6【后端】：拉黑自己返回 4000；拉黑不存在的用户返回 4042；`maxBlocks=1000` 时第 1001 个返回 4134；取消拉黑不在黑名单里的人返回成功、`ver` 不变。
- AC-SOC-07-7【后端】：alice、bob 互为好友，alice 拉黑 bob 后 `socialSync` 中 bob 仍在 alice 的好友列表里，同时出现在黑名单里。

### F-SOC-08 私聊权限（P0）

**规则**
1. 每次 `send`（单聊）时实时判定，不缓存判定结果（第 11 节）。给自己发消息永远允许。
2. **好友制**：双方必须互为好友，否则 4035。
3. **两种模式都生效的拉黑**：对方把我拉黑，4036。
4. **开放模式的陌生人限额**：不互为好友、且对方**从未在这个会话里发过消息**时，我在这个会话里最多发 `strangerMsgLimit` 条，超出返回 4037；对方回复过一条之后永久解除（即使之后又变回陌生人）。互为好友的不受限。
5. **新会话限流**（两种模式）：给一个"我和他之间还没有单聊会话"的人发第一条消息，扣一次 `newPeer:{uid}` 令牌，超出 4290。这一条同时防止用 4042 批量探测账号（D5 的副作用）。
6. 历史会话：权限只影响**发送**；`sync`、`read`、`convs` 的规则不变（PRD-core F-MSG-05 规则 4）。删除好友、被拉黑、模式切换之后，会话仍在列表里。
7. 判定顺序（变更 PRD-core F-MSG-01）：格式（4002～4007）→ **去重命中直接返回第一次的 ack**（已经保存的消息重发一律成功，不受之后关系变化影响）→ 发送限流 4290 → 对方不存在 4042 → 新会话限流 4290 → 好友制下非好友 4035 → 被拉黑 4036 → 开放模式陌生人限额 4037。被 4035～4037 拒绝的消息**扣发送令牌**（防止用拒绝结果高频探测）。
8. 对方不存在：沿用 4042（D5）。用户注销一期不存在（F-USER-01 规则 4）；被封禁的用户（X-8）照常可以接收消息。
9. 客户端：收到 4035 时消息标记失败，在聊天页插入提示"对方开启了好友验证，你还不是他的好友"，并给出"发送好友申请"入口；4036 提示"消息已发出，但被对方拒收了"；4037 提示"对方回复之前，最多只能发送 N 条消息"（N 取 `socialSync` 返回的 `strangerMsgLimit`）。

**验收标准**
- AC-SOC-08-1【后端】：好友制下，alice、bob 互为好友，alice 发私聊成功；alice 单向加了 carol（carol 未加 alice），alice 给 carol 发返回 4035，消息不保存。
- AC-SOC-08-2【后端】：开放模式、`strangerMsgLimit=3`，alice 给陌生人 bob 连发 4 条，前 3 条成功，第 4 条返回 4037；bob 回复 1 条后，alice 再发 10 条都成功。
- AC-SOC-08-3【后端】：开放模式、`strangerMsgLimit=0`，alice 给陌生人连发 10 条全部成功。
- AC-SOC-08-4【后端】：alice 的消息 m1 已保存但 ack 被丢弃（故障注入），之后 bob 拉黑 alice；alice 用同一个 clientMsgId 重发 m1，返回与第一次相同的 msgId 和 seq（不返回 4036）。
- AC-SOC-08-5【后端】：好友制下，alice 给不存在的 `nobody` 发返回 4042（而不是 4035）；给不是好友、但把 alice 拉黑了的 bob 发返回 4035（而不是 4036）。
- AC-SOC-08-6【后端】：`newPeerHourlyLimit=30`，alice 在 1 小时内给 30 个从未聊过的人各发 1 条成功，第 31 个新的人返回 4290；同一小时内给已经聊过的人发消息不受影响。
- AC-SOC-08-7【后端】：给自己发消息在好友制下成功；被自己拉黑的人不影响给自己发。
- AC-SOC-08-8【前端】：好友制下收到 4035，消息标记失败，聊天页出现"对方开启了好友验证，你还不是他的好友"和"发送好友申请"按钮；收到 4037 时提示中的 N 等于服务端配置的 `strangerMsgLimit`。
- AC-SOC-08-9【后端】：开放模式下，bob 被 alice 拉黑后发 1 条返回 4036；alice 解除拉黑后 bob 发送成功。（4036 的消息不计入陌生人条数。）

### F-SOC-09 用户搜索与"可被搜索"开关（P0）

**规则**
1. `userSearch {keyword}`：去首尾空白后必须是合法 userId（4004），**只做精确匹配**，不做前缀、模糊匹配（防枚举）。
2. 返回 `{user: {userId, nickname, avatar, relation} | null}`。`relation`：`self`、`friend`（我这一侧有边）、`pendingOut`、`pendingIn`、`none`。不暴露"对方是否拉黑了我""对方是否加了我"。
3. 返回 `null` 的情况：用户不存在；对方关闭了"可被搜索"。两种情况的应答**完全相同**。搜索自己总能返回。
4. `setPrivacy {searchable}`：个人开关，默认跟随 `searchableDefault`；多端同步（`ver` 加 1）。关闭后仍可以被好友、同群成员、会话对象通过资料卡找到，只是不能被陌生人搜到。
5. 限流见 5.2 节（每分钟 10 次、每天 100 次）；超出返回 4290 / 4292。
6. 日志只记录"搜索了一次、是否命中"，不记录关键字。

**验收标准**
- AC-SOC-09-1：alice 搜索 `bob`，返回 bob 的 userId、昵称、头像，`relation=none`；alice 申请 bob 后再搜，`relation=pendingOut`。
- AC-SOC-09-2【后端】：搜索 `bo`（bob 的前缀）返回 `null`。
- AC-SOC-09-3【后端】：bob 关闭"可被搜索"后，alice 搜索 `bob` 和搜索不存在的 `nobody`，两个应答的 JSON 除 `id` 外逐字节相同。
- AC-SOC-09-4【后端】：alice 在 1 分钟内搜索 11 次，第 11 次返回 4290；`searchDailyLimit=100` 时第 101 次返回 4292。
- AC-SOC-09-5【后端】：bob 拉黑 alice 后，alice 搜索 bob 的结果与拉黑前相同（`relation` 不变）。
- AC-SOC-09-6【后端】：关键字为 `a b` 或 65 个字符返回 4004；搜索自己返回 `relation=self`（即使自己关闭了可被搜索）。
- AC-SOC-09-7：alice 在 web 关闭"可被搜索"，pc 端设置页 2 秒内显示为关闭。
- AC-SOC-09-8【后端】：日志扫描：搜索特征串 `ZXQ-SEARCH-9` 后，服务端日志中搜不到它。

### F-SOC-10 批量查用户资料（U-4）与访问范围（P0）

**规则**
1. 命令 `users {userIds}`：1～100 个，去重后处理；返回 `{users: [{userId, nickname, avatar}]}`。
2. **可见范围**（R-UI-04）：自己；我这一侧的好友；和我有单聊会话的人；和我同在一个未解散群里的人；30 天内和我有好友申请往来的人（两个方向）；我拉黑的人。
3. 不在可见范围内的、不存在的，**都不返回、也不报错**（两者无法区分）。
4. 限流：每分钟 30 次（5.2 节）。
5. 群成员列表（F-SOC-28）、搜索结果、`socialSync` 都直接带资料，`users` 主要用于刷新缓存和显示群消息发送者。
6. SDK 缓存资料 10 分钟；`convs` 刷新时失效（PRD-ui 8.3 节）。

**验收标准**
- AC-SOC-10-1【后端】：alice 和 bob 同在 g1、和 carol 有单聊、和 dave 没有任何关系，`users [bob, carol, dave, nobody]` 只返回 bob、carol。
- AC-SOC-10-2【后端】：alice 被移出 g1 后，再查只在 g1 里认识的 bob，不返回 bob。
- AC-SOC-10-3【后端】：101 个 userId 返回 4000；含不合法 ID 返回 4004；1 分钟内第 31 次调用返回 4290。
- AC-SOC-10-4【后端】：dave 给 alice 发了好友申请（alice 未处理），alice 查 dave 能返回资料；31 天后（测试时钟）不再返回。
- AC-SOC-10-5【前端】：群聊中 bob 的消息显示 bob 的昵称；bob 昵称被业务方修改后，10 分钟内或下一次 `convs` 刷新后显示新昵称。

### F-SOC-11 通讯录数据（P0）

**规则**
1. 通讯录 = **好友列表**（`socialSync`）+ **我的群聊**（`convs` 里 `type=group` 的会话）+ "新的朋友"入口（带红点）。U-2 的"从单聊会话派生联系人"作废。
2. `convs` 的群会话增加 `memberCount`、`myRole`（`owner` / `admin` / `member`），解决 G-UI-02 的成员数来源。
3. 服务端只给数据，排序、分组、拼音规则在 PRD-ui 修订（第 13.2 节）。
4. 开放模式下也显示好友列表（好友只用来整理通讯录）；陌生人会话只在会话列表里。

**验收标准**
- AC-SOC-11-1【后端】：alice 在 g1（5 人）、g2（托管群），`convs` 中 g1 的 `memberCount=5`、`myRole=owner`，g2 的 `myRole=member`。
- AC-SOC-11-2【前端】：alice 有 3 个好友、在 2 个群里、有 1 条未读申请，通讯录显示 3 个好友、"群聊"入口里 2 个群、"新的朋友"红点为 1。
- AC-SOC-11-3【前端】：开放模式下，alice 和陌生人 carol 有单聊，通讯录好友列表里没有 carol。

### F-SOC-12 业务方管理好友关系（P0）

**规则**
1. 导入：`POST /admin/friends/import {pairs: [{a, b, remarkA?, remarkB?}]}`，单次 ≤ 500 对；每一对建立**双向**好友边，来源 `admin`。
2. 幂等：已经是好友的对不变（但传了备注时更新备注）；同一请求里重复的对按最后一条为准；`a == b` 返回 4000。
3. 整批原子：任意一对不合法（4004 / 4000）、任一用户不存在（4042）、任一用户导入后超过 `maxFriends`（4132，`msg` 中写出 userId）时，整批不生效。错误优先级：4004 → 4000 → 4131 → 4042 → 4132。
4. 删除：`DELETE /admin/friends {pairs: [{a, b}]}`，单次 ≤ 500 对，删除**双向**的边（墓碑）；不是好友的忽略。
5. 查询：`GET /admin/users/{userId}/friends?cursor=&limit=`（limit 1～500，默认 100），按 peer userId 升序，游标翻页。
6. 导入和删除都让涉及用户的 `ver` 加 1、给在线连接下发 `socialChanged`；导入同时把双方之间未处理的申请标记为 `accepted`。
7. 导入不受拉黑影响（业务方的决定优先），但不解除拉黑。

**验收标准**
- AC-SOC-12-1【后端】：导入 `[{a: alice, b: bob, remarkA: "B"}]` 返回 `{imported: 1}`；alice、bob 互为好友，alice 一侧备注为"B"，在线的 alice、bob 都收到 `socialChanged`。
- AC-SOC-12-2【后端】：同样的请求重复调用 3 次，好友边仍各一条，`ver` 只在第一次增加。
- AC-SOC-12-3【后端】：501 对返回 400 / 4131；包含不存在的用户返回 404 / 4042 且其他 500 对都没有导入；`a == b` 返回 400 / 4000。
- AC-SOC-12-4【后端】：alice 已有 1999 个好友，导入 2 对含 alice 的关系返回 400 / 4132，两对都不导入。
- AC-SOC-12-5【后端】：`DELETE /admin/friends [{a: alice, b: bob}]` 后两侧都没有对方；好友制下双方私聊返回 4035。
- AC-SOC-12-6【后端】：alice 有 250 个好友，`limit=100` 翻 3 页拿到全部 250 个，无重复。
- AC-SOC-12-7【后端】：缺少或错误的管理密钥返回 401 / 4013（X-5 落地后改为按调用方 scope 判定，见第 8 节）。

---

## 7. 功能需求：群聊补缺

### F-SOC-20 群角色与群资料（P0）

**规则**
1. 角色：`owner`（群主，最多 1 个）、`admin`（管理员，最多 10 个）、`member`。托管群默认没有群主（F-SOC-30）。
2. 权限矩阵：
   | 操作 | 群主 | 管理员 | 成员 | 托管群（无群主）里的任何成员 |
   |---|---|---|---|---|
   | 发言、看资料、看成员、退群 | ✅ | ✅ | ✅ | ✅ |
   | 邀请好友入群 | ✅ | ✅ | ✅ | ❌ 4030 |
   | 修改群名、群公告 | ✅ | ✅ | ❌ 4030 | ❌ 4030 |
   | 移出成员 | ✅（管理员和成员） | ✅（只能移出成员） | ❌ 4030 | ❌ 4030 |
   | 设置 / 取消管理员、转让群主、解散 | ✅ | ❌ 4030 | ❌ 4030 | ❌ 4030 |
3. `groupInfo {groupId}` 返回 `{groupId, name, notice, noticeBy, noticeAt, owner, managed, myRole, memberCount, maxMembers, historyVisible, createdAt}`；`managed=true` 表示托管群。非成员 4033，群不存在或已解散 4043。
4. 成员数由服务端在加减成员的事务里维护（`chat_groups.member_count`），`convs`、`groupInfo` 直接读取。
5. 群资料变化的实时通知：给在线成员下发 `groupChanged {groupId}`（提醒，SDK 收到后对已打开的群重新 `groupInfo`）；可靠的记录靠群提示消息（F-SOC-29）和下一次 `convs`。

**验收标准**
- AC-SOC-20-1【后端】：alice 建群后 `groupInfo` 返回 `owner=alice`、`myRole=owner`、`managed=false`、`memberCount=3`。
- AC-SOC-20-2【后端】：业务方建的群 g2，成员 bob 调 `groupInfo` 返回 `owner=null`、`managed=true`、`myRole=member`。
- AC-SOC-20-3【后端】：非成员 carol 查 g1 返回 4033；查已解散的群返回 4043。
- AC-SOC-20-4【后端】：按第 2 条矩阵，成员 bob 修改群名返回 4030、管理员 carol 移出管理员 dave 返回 4030、管理员 carol 转让群主返回 4030；托管群成员邀请好友返回 4030。
- AC-SOC-20-5【后端】：g1 有 3 人，邀请 2 人后 `convs` 中 g1 的 `memberCount` 为 5；移出 1 人后为 4（同一事务维护，不会出现中间值）。

### F-SOC-21 客户端建群（P0）

**规则**
1. `groupCreate {clientReqId, name?, members}`：`clientReqId` 8～64 位（同 clientMsgId 的规则），同一用户重复提交返回第一次的结果（重发不重复建群），去重记录保留 7 天。
2. `members` 不含自己（含自己时忽略），去重后**至少 2 人**，总人数（含自己）≤ `clientGroupMaxMembers`（超出 4130）。
3. 每个被邀请人必须和我**互为好友**（4035）、没有拉黑我（4036）、加入的群数未满（4135）；**任一人不满足时整个建群失败**，错误里带 `userId`。我自己加入的群数已满返回 4135。`clientGroupEnabled=false` 时返回 4030。
4. groupId 由服务端生成：`ug-` + UUIDv7 去掉连字符（35 个字符）。**`ug-` 前缀保留给自建群**：业务方 `POST /admin/groups` 使用这个前缀返回 4004（变更 ADMIN.md）。
5. `name` 可选：不传时用前 3 个成员（含自己）的显示名，以"、"连接，超过 50 码点截断，建群时生成、之后不自动变化。
6. 建群者为群主；`historyVisible=false`（客户端一期不能修改）。
7. 成功：ack 返回 `{groupId, conv}`（`conv` 与 `convs` 中的一项相同）；被邀请人和建群者的其他在线连接收到 `convAdded`；群里生成一条提示消息（F-SOC-29）。
8. 每天最多建 `groupCreateDailyLimit` 个群（4292）。

**验收标准**
- AC-SOC-21-1：alice 和 bob、carol 互为好友，`groupCreate {members: [bob, carol]}` 成功；bob、carol 在线连接 1 秒内收到 `convAdded`，alice 的另一个连接也收到；群里第一条消息是 seq=1 的提示消息"alice 邀请 bob、carol 加入了群聊"。
- AC-SOC-21-2【后端】：同一个 `clientReqId` 提交 3 次，只建 1 个群，3 次返回同一个 groupId。
- AC-SOC-21-3【后端】：`members` 只有 1 人（或为 `[alice 自己, bob]`）返回 4000。
- AC-SOC-21-4【后端】：members 中 dave 不是 alice 的好友，返回 4035 且 `error.userId=dave`，群未创建、任何人都没有收到事件。
- AC-SOC-21-5【后端】：`clientGroupMaxMembers=500`，邀请 499 人成功，邀请 500 人（总 501）返回 4130。
- AC-SOC-21-6【后端】：不传 `name`，alice、bob、carol 的昵称为"A""B""C"，群名为"A、B、C"。
- AC-SOC-21-7【后端】：业务方 `POST /admin/groups {groupId: "ug-123"}` 返回 400 / 4004。
- AC-SOC-21-8【后端】：`clientGroupEnabled=false` 时建群返回 4030；同一天第 11 次建群返回 4292。
- AC-SOC-21-9【后端】：bob 已加入 500 个群（`maxJoinedGroups=500`），alice 建群邀请 bob 返回 4135 且 `error.userId=bob`。

### F-SOC-22 邀请入群（P0）

**规则**
1. `groupInvite {groupId, members}`：单次 1～50 人；任何成员都可以邀请（托管群不行，4030）；邀请人不是成员 4033，群不存在 4043。
2. 被邀请人要求同建群（互为好友、没拉黑邀请人、群数未满）；**部分成功**：ack 返回 `{added, failed: [{userId, code}]}`。已经是成员的直接忽略（不进 `added`、不进 `failed`，不重置 joinSeq，C-36）。
3. 加完后总人数超过群上限（自建群 `maxMembers`、托管群 2000）时**整次失败** 4130，一人都不加（与管理接口一致）。
4. 直接入群，不需要被邀请人同意（入群审批是 P1）。
5. 新成员的 `joinSeq` 按入群时刻计算，`historyVisible` 规则不变（F-GRP-01 规则 6、7）；新成员收到 `convAdded`；群里生成一条提示消息；成员缓存失效（`TopicGroupMembers`）。

**验收标准**
- AC-SOC-22-1：g1 中的成员 bob 邀请好友 dave，dave 1 秒内收到 `convAdded`；g1 所有在线成员收到提示消息"bob 邀请 dave 加入了群聊"。
- AC-SOC-22-2【后端】：bob 一次邀请 dave（好友）、eve（不是好友）、frank（拉黑了 bob），返回 `added=[dave]`、`failed=[{eve, 4035}, {frank, 4036}]`。
- AC-SOC-22-3【后端】：邀请已经在群里的 carol，返回 `added=[]`、`failed=[]`，carol 的 joinSeq 不变、没有收到事件、没有提示消息。
- AC-SOC-22-4【后端】：群已有 499 人（上限 500），一次邀请 2 人返回 4130，一人都没加。
- AC-SOC-22-5【后端】：g1 已有 10 条消息、`historyVisible=false`，dave 被邀请后 `convs` 中 `minSeq=11`，能看到 seq=11 的入群提示，未读为 1（提示消息计入未读，F-SOC-29 规则 5）。
- AC-SOC-22-6【后端】：51 人返回 4000；非成员邀请返回 4033；托管群成员邀请返回 4030。

### F-SOC-23 退群（P0）

**规则**
1. `groupLeave {groupId}`：任何成员都可以退出，包括托管群。
2. 退群后同"被移除"：不能再发、拉、读（4033），`convs` 中不再出现；本人其他在线连接收到 `convRemoved {reason: "left"}`；成员缓存失效。
3. **不生成群提示消息**（同微信：退群不通知全体成员）；在线成员收到 `groupChanged`（成员数变化）。
4. **群主退群**：自动转让给最早入群的管理员；没有管理员时给最早入群的成员；生成提示消息"X 已成为新群主"。
5. **最后一个人退群**：群自动解散（按 F-SOC-26 处理，群 ID 不复用）。
6. 不是成员时返回 4033（不是幂等成功：避免客户端误以为退了一个从来不在的群）。

**验收标准**
- AC-SOC-23-1：bob 在 web 退出 g1，bob 的 pc 端收到 `convRemoved {reason: "left"}`；之后 bob `send` / `sync` g1 返回 4033；其他成员没有收到提示消息，`groupInfo` 的 `memberCount` 减 1。
- AC-SOC-23-2【后端】：群主 alice 退群，g1 有管理员 carol（入群晚于 bob）和成员 bob，新群主为 carol；群里出现提示消息"carol 已成为新群主"。
- AC-SOC-23-3【后端】：群主 alice 退群，没有管理员，成员 bob 比 dave 先入群，新群主为 bob。
- AC-SOC-23-4【后端】：群里只剩 alice 一人时 alice 退群，之后 `groupInfo` 返回 4043；业务方用这个 groupId 建群返回 4090。
- AC-SOC-23-5【后端】：托管群 g2 的成员 bob 退群成功；`GET /admin/groups/g2` 的成员列表中没有 bob。
- AC-SOC-23-6【后端】：不是成员的 carol 退出 g1 返回 4033。

### F-SOC-24 移出成员（P0）

**规则**
1. `groupKick {groupId, members}`：单次 1～50 人；权限见 F-SOC-20 矩阵。不能移出自己（4000，应使用退群），不能移出群主（4030）。
2. 不是成员的忽略；ack 返回 `{removed}`。
3. 被移出的人收到 `convRemoved {reason: "removed"}`（与管理接口移除相同）；不生成提示消息；在线成员收到 `groupChanged`。
4. 被移出的管理员同时失去管理员身份；再次入群时为普通成员。

**验收标准**
- AC-SOC-24-1：群主 alice 移出 bob，bob 1 秒内收到 `convRemoved {reason: "removed"}`，之后 `send` 返回 4033。
- AC-SOC-24-2【后端】：管理员 carol 移出成员 bob 成功；移出管理员 dave 返回 4030；移出群主返回 4030。
- AC-SOC-24-3【后端】：移出自己返回 4000；`members` 含非成员 eve 时，返回的 `removed` 不含 eve。
- AC-SOC-24-4【后端】：管理员 dave 被移出后再被邀请入群，`myRole=member`。

### F-SOC-25 转让群主与设置管理员（P0）

**规则**
1. `groupTransfer {groupId, to}`：只有群主可以；`to` 必须是群成员（否则 4046）；`to` 为自己返回 4000。原群主变为普通成员；生成提示消息"alice 将群主转让给 bob"。
2. `groupSetAdmin {groupId, userId, admin: true | false}`：只有群主可以；目标必须是成员（4046）；不能设置自己（4000）；管理员已满 10 人返回 4136；重复设置幂等。生成提示消息"alice 将 carol 设为管理员" / "取消了 carol 的管理员身份"。
3. 转让给管理员时，该管理员直接成为群主（管理员数减 1）。

**验收标准**
- AC-SOC-25-1：alice 把群主转让给 bob 后，`groupInfo` 中 `owner=bob`，alice 的 `myRole=member`；成员收到提示消息；alice 再移出成员返回 4030。
- AC-SOC-25-2【后端】：转让给非成员 eve 返回 4046；管理员调用转让返回 4030；转让给自己返回 4000。
- AC-SOC-25-3【后端】：设置第 11 个管理员返回 4136；把已经是管理员的 carol 再设一次返回成功、不生成提示消息。
- AC-SOC-25-4【后端】：把群主转让给管理员 carol 后，carol 的 `myRole=owner`，管理员数为原来减 1。

### F-SOC-26 解散群（P0）

**规则**
1. `groupDissolve {groupId}`：只有群主可以；托管群（无群主）客户端不能解散（4030），由业务方或后台解散（PRD-admin F-ADM-24）。
2. 解散后的效果与 PRD-admin F-ADM-24 完全一致（共用一个服务端实现）：所有在线成员收到 `convRemoved {reason: "dissolved"}`；对该群的 `send` / `sync` / `read` / `groupInfo` 返回 4043；群从所有成员的 `convs` 中消失；**群 ID 不能复用**（建群返回 4090）；消息数据保留，终端用户不可见。
3. 不生成提示消息（群已经不存在）。

**验收标准**
- AC-SOC-26-1：群主 alice 解散 g1，3 个在线成员 1 秒内收到 `convRemoved {reason: "dissolved"}`；离线成员上线后 `convs` 中没有 g1。
- AC-SOC-26-2【后端】：解散后成员 `send`、`sync`、`read` g1 都返回 4043；`GET /admin/groups/{g1}` 返回 404 / 4043。
- AC-SOC-26-3【后端】：管理员调用解散返回 4030；托管群里任何成员调用返回 4030。
- AC-SOC-26-4【后端】：解散后后台按会话 `group:{g1}` 仍能查询到历史消息（需要 `msg.read`，PRD-admin AC-ADM-24-4）。

### F-SOC-27 修改群名与群公告（P0）

**规则**
1. `groupUpdate {groupId, name?, notice?}`：至少一项（4000）；群主和管理员可以（4030）。
2. 群名 1～50 码点（C-38）；公告 0～1000 码点，空字符串表示清除；记录 `noticeBy`、`noticeAt`。
3. 生成提示消息："alice 修改群名为'xxx'"；公告更新时提示消息带公告前 100 码点（摘要），全文通过 `groupInfo` 获取。
4. 修改后新群名在下一次 `convs` 中生效（同 AC-ADM-23-1）；在线成员同时收到 `groupChanged`。
5. 日志不记录公告原文。

**验收标准**
- AC-SOC-27-1：管理员 carol 把群名改为"周末球局"，在线成员收到提示消息；之后 `convs` 中 g1 的 `name` 为"周末球局"。
- AC-SOC-27-2【后端】：成员 bob 修改群名返回 4030；群名为空或 51 码点返回 4000；`name`、`notice` 都不传返回 4000。
- AC-SOC-27-3【后端】：公告 1000 码点成功、1001 码点返回 4000；`groupInfo` 返回完整公告、`noticeBy=carol`、`noticeAt` 为服务端时间。
- AC-SOC-27-4【前端】：收到公告提示消息时，聊天页显示公告摘要，点击后显示全文。

### F-SOC-28 群成员列表（P0）

**规则**
1. `groupMembers {groupId, cursor?, limit?}`：`limit` 1～100（默认 50）；排序：群主 → 管理员（按设为管理员的时间）→ 成员（按入群时间、userId）；返回 `{members: [{userId, role, joinedAt, nickname, avatar}], nextCursor}`，`nextCursor` 为空表示没有更多。
2. 只有成员能查（4033）；群不存在或已解散 4043；游标不透明，非法游标返回 4000。
3. 翻页期间有人入群或退群：不重复；新入群的人可能出现在最后一页（按入群时间排在最后）。
4. 业务方的管理接口另有分页接口（PRD-admin G-ADM-10），两者共用存储查询。

**验收标准**
- AC-SOC-28-1【后端】：200 人的群，`limit=50` 翻 4 页拿到全部 200 人，无重复，第一项是群主。
- AC-SOC-28-2【后端】：翻到第 2 页时有 1 人退群、1 人入群，继续翻完，没有重复项，新入群的人出现在结果中。
- AC-SOC-28-3【后端】：非成员查询返回 4033；`limit=101` 返回 4000；伪造的 cursor 返回 4000。
- AC-SOC-28-4【前端】：聊天信息页显示前 20 个成员头像和"查看全部成员"；成员列表滚动加载到全部（PRD-ui AC-UI-12-4 由 P1 提为 P0）。

### F-SOC-29 群提示消息（P0）

**规则**
1. 新消息类型 `type = "tip"`：服务端生成，占用 seq，和普通消息一起 `sync`、补拉、去重（`clientMsgId` 由服务端生成，前缀 `tip-`）。客户端不能发送 `tip`（4000）。
2. `body = {tip: {kind, actor?, targets?, text?}}`；`kind`：`created`、`joined`（邀请入群，含管理接口加人）、`ownerChanged`、`adminSet`、`adminUnset`、`renamed`、`noticeUpdated`。`from` 为操作人 userId；业务方、后台、自动转让等系统操作 `from = ""`（Schema 只允许 `type=tip` 时为空）。
3. **生成提示消息的操作**：建群、邀请（含业务方加成员）、转让（含自动转让）、设置 / 取消管理员、改群名、改公告。**不生成**：退群、移出、解散、业务方移除成员（同微信，不通知全体成员）。
4. 业务方一次加很多人时只生成一条：`targets` 最多 20 个 userId，另带 `count`（总人数）。
5. 计入未读、出现在 `lastMsg`（摘要由服务端按操作时的显示名生成，如"bob 邀请 dave 加入了群聊"）。不计入未读放 P1（需要改未读公式，影响 C-02）。
6. 不认识的 `kind` 显示为"[群通知]"；不认识的 `type` 沿用 PRD-ui 的"[暂不支持的消息类型]"。
7. 客户端用 `actor`、`targets` 和本地资料渲染个性化文案（"你邀请了……"），服务端摘要只用于 `lastMsg` 和不认识的情况。

**验收标准**
- AC-SOC-29-1【后端】：建群、邀请、转让、设管理员、改群名、改公告各一次，`sync` 依次拿到 6 条 `type=tip`，`kind` 分别为 `created`、`joined`、`ownerChanged`、`adminSet`、`renamed`、`noticeUpdated`，seq 连续。
- AC-SOC-29-2【后端】：退群、移出各一次，群的 maxSeq 不变。
- AC-SOC-29-3【后端】：业务方一次加 500 人，只生成 1 条提示消息，`targets` 20 个、`count=500`。
- AC-SOC-29-4【后端】：客户端 `send {type: "tip"}` 返回 4000。
- AC-SOC-29-5【前端】：alice 自己看到"你邀请 dave 加入了群聊"，bob 看到"alice 邀请 dave 加入了群聊"；会话列表摘要显示服务端生成的文本。
- AC-SOC-29-6【后端】：提示消息的 `from` 为空时通过 Schema 校验；`type=text` 且 `from` 为空时 Schema 校验失败（`vectors/frames.json` 新增用例）。

### F-SOC-30 托管群与管理接口交接（P0）

**规则**
1. 业务方建的群（`POST /admin/groups`）默认 `owner = null`（托管群）：客户端只能发言、查看、退群（F-SOC-20 矩阵）。
2. 业务方可以把托管群交给用户：`POST /admin/groups` 增加可选 `owner`；`PATCH /admin/groups/{groupId}` 增加 `owner`（字符串：指定或更换群主，必须是成员，否则 404 / 4046；`null`：收回为托管群，原群主变为普通成员）。生成 `ownerChanged` 提示消息（`from=""`）。
3. 管理接口对**所有**群（包括自建群）保持全部能力：加减成员、改名、改 `historyVisible`、解散；不受客户端的好友、人数 500、群数 500 限制（仍受 2000 总数限制）。
4. 管理接口移除的人如果是群主：按群主退群规则自动转让（F-SOC-23 规则 4）。
5. 交接点清单见 9.5 节。

**验收标准**
- AC-SOC-30-1【后端】：`POST /admin/groups {groupId: g3, owner: bob, members: [bob, carol]}` 后，bob 的 `myRole=owner`，可以在客户端改群名。
- AC-SOC-30-2【后端】：`PATCH /admin/groups/g3 {owner: null}` 后 `managed=true`，bob 改群名返回 4030，群里出现 `from=""` 的 `ownerChanged` 提示消息。
- AC-SOC-30-3【后端】：`PATCH {owner: eve}`（eve 不是成员）返回 404 / 4046。
- AC-SOC-30-4【后端】：业务方给自建群 g1（上限 500）加成员到 600 人成功，给非好友加成员成功。
- AC-SOC-30-5【后端】：业务方移除自建群的群主 alice，新群主按 F-SOC-23 规则 4 产生。

---

## 8. 数据模型

迁移编号接在 0003 之后，放在 `server/internal/store/migrations/sqlite/`；Postgres 版本在 B6 写同编号文件（ARCH 3.1）。

### 8.1 新表与改动

**0004_social.sql**

| 表 | 字段 | 主键 / 索引 | 说明 |
|---|---|---|---|
| `app_settings` | `key TEXT`、`value TEXT`（JSON）、`updated_at BIGINT` | PK `key` | 业务方配置（5.1 节），key = `social` |
| `user_social` | `user_id TEXT`、`ver BIGINT`、`purged_ver BIGINT`、`request_seen_ver BIGINT`、`searchable SMALLINT NULL`（NULL = 跟随默认）、`friend_count INTEGER`、`block_count INTEGER`、`updated_at BIGINT` | PK `user_id` | 每用户关系版本号与计数；ver 和数据修改同一事务分配（同 `conv_seq`） |
| `friends` | `owner_id`、`peer_id`、`remark TEXT`、`source TEXT`、`created_at`、`updated_at`、`deleted SMALLINT`、`ver BIGINT` | PK `(owner_id, peer_id)`；索引 `(owner_id, ver)` | 有向好友边；删除写墓碑 |
| `friend_requests` | `owner_id`、`peer_id`、`direction TEXT`（`in`/`out`）、`status TEXT`（`pending`/`accepted`/`rejected`）、`message TEXT`、`source TEXT`、`source_ref TEXT`、`preset_remark TEXT`、`apply_count INTEGER`、`created_at`、`updated_at`、`expire_at`、`ver` | PK `(owner_id, peer_id, direction)`；索引 `(owner_id, ver)`、`(updated_at)`（清理） | 一条申请**按双方各存一行**（申请人 `out`、被申请人 `in`）。拒绝只改 `in` 行 |
| `blocks` | `owner_id`、`peer_id`、`created_at`、`deleted`、`ver` | PK `(owner_id, peer_id)`；索引 `(owner_id, ver)` | 黑名单；删除写墓碑 |
| `user_conv`（改） | 新增 `sent_count INTEGER NOT NULL DEFAULT 0` | — | 我在这个单聊里发过几条（封顶 1000），用于陌生人限额；`SaveMessage` 同一事务 +1；迁移时按 `messages` 回填 |
| `group_members`（改） | 新增索引 `(user_id)` | — | **现状缺失**：`ListConversations` 的 `gm.user_id = ?` 和"共同群"判断都在全表扫描 |

**0005_group_roles.sql**

| 表 | 字段 | 说明 |
|---|---|---|
| `chat_groups`（改） | `owner_id TEXT NOT NULL DEFAULT ''`（空 = 托管群）、`created_by TEXT NOT NULL DEFAULT ''`（空 = 业务方）、`notice TEXT DEFAULT ''`、`notice_by TEXT DEFAULT ''`、`notice_at BIGINT DEFAULT 0`、`max_members INTEGER DEFAULT 2000`、`member_count INTEGER DEFAULT 0`（迁移时回填）、`dissolved_at BIGINT DEFAULT 0` | 解散只打标记，`GroupExists` 对已解散的群返回 false，但建群时按"存在过"判 4090（与 F-ADM-24 共用） |
| `group_members`（改） | `role TEXT NOT NULL DEFAULT 'member'`、`role_at BIGINT DEFAULT 0` | `owner` 冗余在 `chat_groups.owner_id` 和成员行上，两处在同一事务更新 |
| `client_group_req` | `user_id`、`client_req_id`、`group_id`、`created_at`；PK `(user_id, client_req_id)` | 建群去重，7 天后清理 |

### 8.2 满足 ARCH 约束的方式

| 约束 | 做法 |
|---|---|
| 可迁移到 Postgres（ARCH 3.3） | 只用 `ON CONFLICT … DO UPDATE`、`RETURNING`、标准 `LIMIT`；时间全是 `BIGINT` 毫秒；不用自增主键（全部业务主键）；枚举用 `TEXT`，不用数据库 enum；不用触发器；`ver` 在应用层事务里 `UPDATE user_social SET ver = ver + 1 … RETURNING ver` 分配 |
| 向前兼容的迁移（ARCH 3.1） | 只加表、加列（有默认值）、加索引；回填在同一迁移里；旧版本二进制读新库不受影响（不读新列） |
| 可分片（按 userId） | `user_social`、`friends`、`friend_requests`、`blocks`、`user_conv`、`client_group_req` 的主键都以用户 ID 开头，可以按 `owner_id` / `user_id` 分片。跨用户的写入（同意申请写双方、导入写双方）一期在单库事务里完成；分片后改为"申请行为状态机 + 按用户各自写入 + 幂等重试"（申请行的 `status` 是唯一真相，任一侧写失败可重放），不需要跨分片事务。群表继续按 `group_id`；`group_members(user_id)` 索引分片后需要全局二级索引或按用户冗余一份"我的群"（ARCH 3.6 第 4 步时处理，记入 ARCH 差距） |
| 契约测试（ARCH 3.2） | `store/storetest` 新增：`SocialVerMonotonic`（并发变更 ver 连续不重复）、`FriendApplyMerge`、`FriendMutualAutoAccept`、`FriendTombstoneSync`（墓碑与 purgedVer）、`FriendLimitAtomic`（并发同意不突破上限）、`BlockList`、`FriendImportAtomic`、`StrangerSentCount`、`GroupRoles`、`GroupOwnerAutoTransfer`、`GroupDissolveNoReuse`、`GroupMemberCount`（与成员数一致）、`GroupMembersPaging`、`GroupCreateIdempotent`、`VisibleUsers`（F-SOC-10 可见范围）。Postgres 实现必须通过同一套 |
| 不依赖总线 | 关系变化靠 `ver` + `socialSync`、群变化靠 seq + `convs`，事件只影响实时性 |

### 8.3 Store 接口新增（摘要）

`SocialSettings / SetSocialSettings`；`ApplyFriend`（返回结果状态与双方新 ver，内部处理合并、互相申请、恢复单向边、拉黑静默）；`RespondFriend`；`DeleteFriend`；`SetRemark`；`Block / Unblock`；`SocialChanges(uid, sinceVer, limit)`；`SocialFull(uid)`；`SetRequestSeen`；`SetSearchable`；`Relation(a, b)`（一次查询返回 a→b 边、b→a 边、b 是否拉黑 a、双方 sent_count，供发送路径使用）；`VisibleUsers(uid, ids)`；`ImportFriends / RemoveFriends`；`CreateClientGroup`（含去重）；`SetGroupRole`；`TransferOwner`；`LeaveGroup`（含自动转让、最后一人解散）；`DissolveGroup`；`UpdateGroupProfile`；`ListGroupMembers(gid, cursor, limit)`；`PurgeSocial(before)`（清理墓碑和 30 天前的申请）。

---

## 9. 协议变更清单（由后端落实到 protocol/）

### 9.1 新增命令（上行，都有 ack）

| 命令 | 请求 `data` | ack `data` | 功能点 |
|---|---|---|---|
| `friendApply` | `{to, message?, source?, sourceRef?, remark?}` | `{status, ver}` | F-SOC-02 |
| `friendRespond` | `{from, action, remark?}` | `{ver, friend?}` | F-SOC-03 |
| `friendDelete` | `{userId}` | `{ver}` | F-SOC-05 |
| `friendRemark` | `{userId, remark}` | `{ver}` | F-SOC-05 |
| `friendRequestsSeen` | `{ver}` | `{requestSeenVer}` | F-SOC-04 |
| `socialSync` | `{sinceVer}` | `{ver, full, hasMore, friends, requests, blocks, requestSeenVer, settings, mode, strangerMsgLimit}` | F-SOC-06 |
| `block` / `unblock` | `{userId}` | `{ver}` | F-SOC-07 |
| `userSearch` | `{keyword}` | `{user}` | F-SOC-09 |
| `setPrivacy` | `{searchable}` | `{ver}` | F-SOC-09 |
| `users` | `{userIds}` | `{users}` | F-SOC-10（U-4） |
| `groupCreate` | `{clientReqId, name?, members}` | `{groupId, conv}` | F-SOC-21 |
| `groupInvite` | `{groupId, members}` | `{added, failed}` | F-SOC-22 |
| `groupLeave` | `{groupId}` | `{}` | F-SOC-23 |
| `groupKick` | `{groupId, members}` | `{removed}` | F-SOC-24 |
| `groupTransfer` | `{groupId, to}` | `{}` | F-SOC-25 |
| `groupSetAdmin` | `{groupId, userId, admin}` | `{}` | F-SOC-25 |
| `groupDissolve` | `{groupId}` | `{}` | F-SOC-26 |
| `groupUpdate` | `{groupId, name?, notice?}` | `{}` | F-SOC-27 |
| `groupInfo` | `{groupId}` | 见 F-SOC-20 规则 3 | F-SOC-20 |
| `groupMembers` | `{groupId, cursor?, limit?}` | `{members, nextCursor}` | F-SOC-28 |

数据结构：`FriendItem {userId, remark, source, createdAt, ver, deleted?, profile}`；`RequestItem {userId, direction, status, message, source, applyCount, createdAt, updatedAt, expireAt, ver, profile}`（`status` 由服务端按 `expireAt` 计算出 `expired`）；`BlockItem {userId, createdAt, ver, deleted?, profile}`。所有请求仍然严格解析（未知字段 4000，与现有 `decode` 一致）。

### 9.2 新增 / 修改事件与字段

| 变更 | 说明 |
|---|---|
| 新增事件 `socialChanged {ver}` | 提醒；发给本人除发起连接外的全部连接、以及关系另一方的全部连接 |
| 新增事件 `groupChanged {groupId}` | 群资料或成员数变化的提醒，发给在线成员 |
| `convRemoved` 增加 `reason`：`removed` / `dissolved` / `left` | `removed`、`dissolved` 与 PRD-admin 第 8 节一致，新增 `left`（自己在其他端退群） |
| `Conversation` 增加可选 `memberCount`、`myRole`（群） | F-SOC-11；向后兼容 |
| `MsgType` 增加 `tip`；`Message.from` 在 `type=tip` 时允许为空 | F-SOC-29；Schema 用 `if/then`；`vectors/frames.json` 补用例 |
| ack `error` 增加可选 `userId` | 建群失败时指出是哪个人（同 4031 带 `mutedUntil` 的做法） |
| 协议 README 第 3 节规则 9 补充 | 单聊发送权限 4035 / 4036 / 4037；群操作 4030 / 4046 |

### 9.3 管理接口（ADMIN.md）

| 接口 | 说明 | 功能点 |
|---|---|---|
| `GET /admin/settings/social` / `PATCH /admin/settings/social` | 读写 5.1 节参数；PATCH 只改传了的字段 | F-SOC-01 |
| `POST /admin/friends/import` | `{pairs: [{a, b, remarkA?, remarkB?}]}` → `{imported}`，≤ 500 对，整批原子 | F-SOC-12 |
| `DELETE /admin/friends` | `{pairs: [{a, b}]}` → `{removed}`，≤ 500 对 | F-SOC-12 |
| `GET /admin/users/{userId}/friends?cursor=&limit=` | 好友分页 | F-SOC-12 |
| `POST /admin/groups` 增加 `owner?`；groupId 不能以 `ug-` 开头 | — | F-SOC-21、30 |
| `PATCH /admin/groups/{groupId}` 增加 `owner?`（字符串或 `null`）、`notice?` | — | F-SOC-30 |
| `GET /admin/groups/{groupId}` 返回增加 `owner`、`admins`、`notice`、`memberCount`、`createdBy`、`managed` | — | F-SOC-20 |
| `DELETE /admin/groups/{groupId}` 解散 | 与 PRD-admin G-ADM-11 是同一个接口，**提前到第一批** | F-SOC-26 |
| X-5 权限范围（scope）新增 | `social.settings`（读写配置）、`social.friends`（导入 / 删除 / 查询好友）；后台服务（console）默认只有 `social.friends` 的查询 | X-5 |

### 9.4 新增错误码（核对 errors.json：以下编号都未使用；4034、4044 已被 PRD-admin 预定，避开）

| code | 名称 | HTTP | 通道 | 可重试 | 含义 | 客户端处理 |
|---|---|---|---|---|---|---|
| 4035 | NOT_FRIEND | 403 | ws | 否 | 好友制下不是互为好友；建群 / 邀请的对象不是好友 | 提示"对方开启了好友验证，你还不是他的好友"，给出"发送好友申请"入口 |
| 4036 | REJECTED_BY_PEER | 403 | ws | 否 | 被对方拉黑（私聊、被邀请入群） | 提示"消息已发出，但被对方拒收了" |
| 4037 | STRANGER_MSG_LIMIT | 403 | ws | 否 | 开放模式下对方回复前的条数用完 | 提示"对方回复之前，最多只能发送 N 条消息" |
| 4045 | FRIEND_REQUEST_NOT_FOUND | 404 | ws | 否 | 申请不存在、已过期、不是发给你的 | 提示"该申请已过期"，刷新列表 |
| 4046 | TARGET_NOT_GROUP_MEMBER | 404 | ws / admin | 否 | 操作对象不是群成员（转让、设管理员、指定群主） | 提示"对方已不在群里"，刷新成员列表 |
| 4091 | FRIEND_REQUEST_HANDLED | 409 | ws | 否 | 申请已同意，不能再拒绝 | 刷新列表 |
| 4132 | FRIEND_LIMIT_EXCEEDED | 400 | ws / admin | 否 | 自己的好友数已达上限 | 提示"好友数量已达上限" |
| 4133 | PEER_FRIEND_LIMIT_EXCEEDED | 400 | ws | 否 | 对方的好友数已达上限 | 提示"对方好友数量已达上限" |
| 4134 | BLOCK_LIMIT_EXCEEDED | 400 | ws | 否 | 黑名单已满 | 提示"黑名单已满" |
| 4135 | JOINED_GROUP_LIMIT_EXCEEDED | 400 | ws | 否 | 自己或对方加入的群数已达上限 | 按 `error.userId` 提示是谁 |
| 4136 | GROUP_ADMIN_LIMIT_EXCEEDED | 400 | ws | 否 | 管理员已满 10 人 | 提示"管理员最多 10 人" |
| 4292 | DAILY_QUOTA_EXCEEDED | 429 | ws | 否 | 每日次数用完（好友申请、搜索、建群） | 提示"今天的次数已用完，明天再试" |

已有错误码扩大使用范围（只扩 `scope`、不改含义）：4130 增加 ws 通道（自建群人数上限，`msg` 里的"2000"只用于日志，客户端文案改为"群成员已达上限"）；4030 用于群权限不足、托管群操作、`clientGroupEnabled=false`。

**号段说明（S-5）**：403x 只剩 4038、4039。PRD-admin 评审（R-ADM-03）建议的后台专用码 4035～4038 与本表冲突，且只是建议、未写入 errors.json。本 PRD 建议：社交用 4035～4037；后台的 CSRF、STEP_UP 用 4038、4039；登录锁定用 4291（评审备选）；4292 留给本 PRD 的每日配额；CONTENT_BLOCKED（P2）到时另议。errors.json 只增不改，必须两份 PRD 一起定稿后再写入。

### 9.5 与 PRD-admin 群管理的交接点

| 交接点 | 约定 |
|---|---|
| 解散群 | 客户端 `groupDissolve` 和 `DELETE /admin/groups/{id}`（G-ADM-11）共用实现与效果；**G-ADM-11 从第二批提前到第一批** |
| `convRemoved.reason` | 统一枚举 `removed` / `dissolved` / `left`，PRD-admin 第 8 节补 `left` |
| 群成员分页 | `groupMembers`（客户端）和 `GET /admin/groups/{id}/members`（G-ADM-10）共用存储查询；后台额外支持按 userId 前缀筛选 |
| 群列表 | 后台群列表（G-ADM-09）增加"群主""类型（自建 / 托管）"列 |
| 群主与管理员 | 后台群详情显示群主、管理员；后台可以指定群主（`PATCH owner`）；后台设置管理员放 P1 |
| 禁言（第二批） | PRD-admin F-ADM-42 写"一期没有群管理员概念"，本 PRD 之后**有了**：全群禁言时群主、管理员可以发言（F-ADM-42 规则需要改） |
| 人数上限 | 后台 / 管理接口按 2000 校验；自建群的 `maxMembers` 只约束客户端邀请 |
| 用户详情 | 后台用户详情增加好友数、黑名单数（P1，`GET /admin/users/{id}/friends` 已提供） |

---

## 10. SDK 变更（sdk-ts、sdk-kotlin、sdk-swift 语义一致）

### 10.1 对外接口（以 sdk-ts 为例，Kotlin / Swift 用各自的命名习惯，语义相同）

```ts
// 状态（快照 + 事件）
im.on('friends', (list: Friend[]) => …);              // 好友（含备注、资料）
im.on('friendRequests', ({ items, unseen }) => …);    // 新的朋友 + 红点数
im.on('blocks', (list: BlockedUser[]) => …);
im.on('socialSettings', ({ searchable, mode, strangerMsgLimit }) => …);
im.on('groupChanged', (groupId) => …);

// 好友
await im.applyFriend(userId, { message, source, sourceRef, remark });  // → 'pending' | 'accepted'
await im.respondFriend(userId, 'accept' | 'reject', { remark });
await im.deleteFriend(userId);
await im.setFriendRemark(userId, remark);
im.markFriendRequestsSeen();
im.relationOf(userId);   // 'self' | 'friend' | 'pendingOut' | 'pendingIn' | 'blocked' | 'none'（只看本地）

// 拉黑、搜索、资料
await im.block(userId); await im.unblock(userId);
await im.searchUser(keyword);           // → User | null
await im.setSearchable(bool);
await im.getUsers(userIds);             // U-4：带缓存，自动分批 100

// 群
await im.createGroup({ name, members });          // 自动生成 clientReqId，超时重试用同一个
await im.inviteToGroup(groupId, members);         // → { added, failed }
await im.leaveGroup(groupId);
await im.kickFromGroup(groupId, members);
await im.transferGroupOwner(groupId, to);
await im.setGroupAdmin(groupId, userId, admin);
await im.dissolveGroup(groupId);
await im.updateGroup(groupId, { name, notice });
await im.getGroupInfo(groupId);
await im.loadGroupMembers(groupId, cursor?);      // → { members, nextCursor }
```

所有异步方法失败时抛出 / 返回带错误码的结果，错误码透传给界面（与 `send` 一致）。

### 10.2 本地状态与行为规则（三套 SDK 必须一致）

| 规则 | 依据 |
|---|---|
| 本地保存 `socialVer`、好友、申请、黑名单、`requestSeenVer`、设置；sdk-ts 只在内存（D4），原生 SDK 是否持久化在 M5 / M6 定，但必须保存 `socialVer` 与数据的一致性（一起写） | F-SOC-06 |
| 每次连接成功：先 `convs`，再 `socialSync {sinceVer: socialVer}`，循环到 `hasMore=false`；`full=true` 时整体替换 | F-SOC-06 规则 6 |
| 收到 `socialChanged {ver}` 且 `ver > socialVer`：去抖 200ms 后 `socialSync`；`ver <= socialVer` 忽略 | F-SOC-06 规则 4 |
| 按 `ver` 升序应用变化；`deleted=true` 移除；同一 userId 只保留 ver 最大的一项 | F-SOC-06 |
| 自己发起的操作用 ack 里的 `ver` 和数据直接更新本地；若 ack 的 ver 不等于 `socialVer + 1`（中间有别的变化），再触发一次 `socialSync` | F-SOC-06 规则 5 |
| 申请是否过期：用 `expireAt` 和**服务端时间**（auth ack 的 `serverTime` 换算的偏移）判断，不用本地时钟 | PRD-core 3.1 节 |
| 红点 = 收到的、`pending`、未过期、`ver > requestSeenVer` 的条数；`markFriendRequestsSeen` 本地立即清零 | F-SOC-04 |
| `send` 收到 4035 / 4036 / 4037：消息标记失败，错误码透传；**不自动重发**（与 4042 相同） | F-SOC-08 |
| `convRemoved` 按 `reason` 区分，保留会话展示信息（DECISIONS 第 3 节"SDK 收到被移出群时保留会话的展示信息"） | F-SOC-23、24、26 |
| `type=tip` 的消息正常入库、计入未读；不认识的 `kind` 渲染为"[群通知]" | F-SOC-29 |
| 资料缓存 10 分钟；`convs` 刷新时失效；`getUsers` 合并同一事件循环内的请求、每批最多 100 | F-SOC-10 |
| `createGroup` 的 `clientReqId` 在一次调用的所有重试中不变 | F-SOC-21 规则 1 |
| 被踢、退出登录时清空全部社交状态 | F-AUTH-03 |

### 10.3 共享测试数据

新增 `protocol/vectors/social.json`：给定"本地状态 + 一串 `socialSync` 应答 / 事件"，断言最终的好友、申请、黑名单、红点数。三套 SDK 都跑（对应 SDK README 第 4 节）。错误码一致性测试自动覆盖第 9.4 节新码。

### 10.4 原生 SDK 的时间

`sdk-kotlin`、`sdk-swift` 目前**都还没有开始**（QUEUE Q17、Q18，M5 / M6），S-3 的"三套 SDK 第一批"无法在它们存在之前实现。处理方式见 S-6。

---

## 11. 安全与隐私

| 风险 | 对策 | 落在 |
|---|---|---|
| 账号枚举：搜索 | 只精确匹配；不存在与不可被搜索的应答相同；每分钟 10 次、每天 100 次；日志不记关键字 | F-SOC-09 |
| 账号枚举：批量查资料 | 只返回可见范围内的人，范围外与不存在无法区分；每分钟 30 次 | F-SOC-10 |
| 账号枚举：好友申请 / 私聊返回 4042 | 好友申请 4042 计入每日配额；私聊对新对象限流（每小时 30 个新对象），D5 的 4042 无法被高频利用 | F-SOC-02、08 |
| 拉黑状态泄露 | 申请静默、搜索与资料照常；只有私聊明确返回 4036（与微信一致，否则发送方会一直以为对方收到了） | F-SOC-07 |
| 陌生人骚扰（开放模式） | 回复前最多 3 条；新对象限流；拉黑；只能被互为好友的人拉进群 | F-SOC-08、22 |
| 好友申请轰炸 | 每秒 1 个、每天 30 个、同一人每天 3 次；重复申请合并，不刷屏 | F-SOC-02 |
| 拉群骚扰 | 只能邀请互为好友的人；每天建群 10 个；加入群数上限 | F-SOC-21、22 |
| 隐私数据进日志 | 附言、备注、搜索关键字、群公告原文禁止进日志，只记长度 / 是否命中；用特征串扫描日志的测试保证（ARCH S9 同款） | AC-SOC-02-10、09-8 |
| 越权 | 所有群操作服务端按角色判定，客户端隐藏按钮只是体验；托管群客户端不能管理 | F-SOC-20 |
| 管理接口滥用 | 好友导入、配置修改走 X-5 的分调用方密钥和 scope；后台服务默认不能改配置 | 9.3 节 |
| 绕过拉黑的竞态 | 拉黑和发送都以数据库为准，不缓存关系判定；拉黑提交后的下一条消息一定被拒 | 第 12 节 |
| 存储型 XSS | 附言、备注、群名、公告一律按纯文本渲染（ARCH S12） | PRD-ui |

---

## 12. 分布式影响

| 项 | 一期（单节点） | 集群 | 理由 |
|---|---|---|---|
| 发送路径的关系判定 | **不缓存**，每次 `Relation(a, b)` 一次主键查询（两条边 + 拉黑 + sent_count） | 同左；必要时加"按用户对"的 LRU，经 `cache.Invalidator` 主题 `relations`（key = userId，删除涉及该用户的全部项）失效，TTL 60 秒 | 按用户缓存整份好友表，1 万在线 × 2000 好友约 800MB，不划算；主键查询在 SQLite 上约 20µs。拉黑必须立即生效，缓存会引入竞态 |
| 社交配置 | 进程内缓存，`Invalidator` 主题 `socialSettings`，TTL 60 秒 | 失效经 NATS 广播 | ARCH 2.2 约束 2 |
| 群成员缓存 | 客户端群操作（建群、邀请、退群、移出、解散）同样发 `TopicGroupMembers` 失效 | 同左 | 现有 `memberCache` 只在 admin 包里失效，logic 新路径必须补上 |
| 群角色 | 不缓存，群管理操作直接查库 | 同左 | 频率低 |
| 关系事件投递 | `delivery.Router.DeliverToUser` | 经 Presence + NATS 投到对方所在节点 | 投递丢失由 `ver` + `socialSync` 补齐 |
| ver 分配 | 数据库事务内 `ver = ver + 1 RETURNING` | 按用户分片后仍在该用户所在分片的事务里 | 同 seq 的做法（ARCH 2.3） |
| 限流 | 内存令牌桶，重启清零 | Redis 令牌桶；每日配额需要持久化（Redis 带 TTL） | ARCH G2 |
| 清理任务 | 进程内每小时一次 `PurgeSocial` | 只在一个节点上跑（分布式锁或独立 cron） | 防止多节点重复清理 |

---

## 13. 对其他文档的影响

### 13.1 PRD-core v1.3 变更条目

| # | 位置 | 变更 |
|---|---|---|
| V-01 | 第 3 节 | 新增 F-SOC-01～12、F-SOC-20～30（引用本 PRD） |
| V-02 | 第 7 节 | 删除"App 内建群、通讯录、好友：三期" |
| V-03 | F-MSG-01 规则 5 | 改为"对方不存在返回 4042；发送权限见 F-SOC-08（4035 / 4036 / 4037），判定顺序见 F-SOC-08 规则 7" |
| V-04 | AC-USER-01-1、AC-MSG-01-*、AC-CONV-03-*、AC-SYNC-01-* 等私聊用例 | 注明"开放模式（默认）"；e2e 默认配置 `mode=open`、`strangerMsgLimit=0`，避免现有用例被陌生人限额打断 |
| V-05 | F-CONV-01 规则 1 | 群会话增加 `memberCount`、`myRole` |
| V-06 | F-GRP-01 | 增加群主、管理员、托管群（F-SOC-20、30）；`ug-` 前缀保留；群上限 2000 不变，自建群另有 `maxMembers` |
| V-07 | F-GRP-03 | `convRemoved` 增加 `reason`；客户端群操作同样触发 `convAdded` / `convRemoved` |
| V-08 | 3.1 节 | 新增消息类型 `tip`（F-SOC-29） |
| V-09 | F-CONV-03 | 发起单聊增加"从好友列表 / 资料卡发起"；好友制下给非好友发起时提示先加好友 |
| V-10 | F-SYS-01 规则 4、C-10 | "KMP SDK"改为"Kotlin、Swift SDK 各自一致"（DECISIONS 2.5 节已定） |
| V-11 | 第 5 节文案表 | 新增 4035、4036、4037、4045、4046、4091、4132～4136、4292 的文案（第 9.4 节） |
| V-12 | 第 8 节 | 并入本 PRD 第 9 节 |
| V-13 | F-USER-02 | 资料来源增加 `users` 命令（U-4），访问范围见 F-SOC-10 |

### 13.2 PRD-ui

- **U-2 作废**：F-UI-13 改为"好友列表 + 群聊 + 新的朋友"；删除"联系人来自你的聊天记录"；AC-UI-13-* 重写。
- G-UI-01（`users`）、G-UI-02（`memberCount`、`groupMembers`）、G-UI-04（好友、搜索）、G-UI-09（客户端群管理）由"后续 / 三期"改为本期；第 11 节"不在本期范围"相应删除。
- F-UI-12 聊天信息页从"只读"改为按角色显示操作；AC-UI-12-3 作废。
- 新增页面（只列清单，交互在 PRD-ui v0.2 写）：

| 页面 | 路由（建议） | 功能点 |
|---|---|---|
| 新的朋友 | `/contacts/requests` | F-SOC-03、04 |
| 添加朋友 / 搜索 | `/contacts/add` | F-SOC-09、02 |
| 申请添加朋友（填附言、备注） | `/contacts/apply/:userId` | F-SOC-02 |
| 好友资料卡（发消息、备注、拉黑、删除） | `/user/:userId`（扩展 F-UI-14） | F-SOC-05、07 |
| 设置备注 | `/user/:userId/remark` | F-SOC-05 |
| 黑名单 | `/me/settings/blocks` | F-SOC-07 |
| 隐私设置（可被搜索） | `/me/settings/privacy` | F-SOC-09 |
| 发起群聊（选择好友） | `/groups/new` | F-SOC-21 |
| 群设置（群名、公告、管理员、转让、退出、解散） | `/chat/:convId/info`（扩展 F-UI-12） | F-SOC-20、23～27 |
| 群公告 | `/chat/:convId/notice` | F-SOC-27 |
| 群成员列表（移出、设管理员） | `/chat/:convId/members` | F-SOC-24、25、28 |
| 邀请成员（选择好友） | `/chat/:convId/invite` | F-SOC-22 |
| 聊天页：4035 / 4036 / 4037 提示条、群提示消息样式 | — | F-SOC-08、29 |

### 13.3 PRD-admin

- G-ADM-11 解散群提前到第一批（9.5 节）；`convRemoved.reason` 补 `left`。
- F-ADM-42 全群禁言的"一期没有群管理员"前提改变。
- 群列表 / 详情增加群主、类型、管理员；新增"好友关系查询"（P1）。
- 第 8 节错误码与本 PRD 第 9.4 节合并定稿（S-5）。

### 13.4 DECISIONS

- **X-6**：第一批"不改客户端协议"的前提已在 2.5 节作废（S-3）；另外 **"解散群"从第二批移到第一批**（本 PRD 的客户端解散和群主退群自动解散都依赖它）。禁言、删消息、封禁、系统通知仍在第二批。
- **X-5**：好友导入、配置接口要纳入分调用方密钥的 scope（9.3 节）；X-5 若晚于社交上线，这两个接口先用现有 `X-Admin-Key`。
- **U-4**：访问范围由本 PRD F-SOC-10 定义（比"有共同会话的人"更宽：加上好友、申请往来、黑名单）。

### 13.5 其他

- `protocol/README.md` 第 2、3 节、`frame.schema.json`、`vectors/frames.json`、新增 `vectors/social.json`；`ERRORS.md`、`errors.json`；`ADMIN.md`。
- `docs/sdk/README.md` 第 2、3 节（第 10 节内容）。
- ARCH 第 1.2 节差距表补一条："`group_members` 缺少 `user_id` 索引；分片后需要'我的群'二级索引"。
- 测试用例新增 `TC-SOC.md`；`tools/trace.py` 目前只读 `PRD-core-v1.2.md`，需要改为同时读本 PRD（或 v1.3 合并后读 v1.3）。

---

## 14. 待拍板项

| # | 问题 | 选项 | 推荐 | 代价 |
|---|---|---|---|---|
| **S-4** | "业务方"的粒度：fx-sport、ab77、android（yd / yi / tb）是各用一个 im-server 部署，还是共用一个部署 | ① **按部署**：一个部署 = 一个用户空间 = 一套配置；X-5 的"分密钥"指同一业务方的多个调用方（业务后端、后台服务）。② **多租户 `appId`**：一个部署服务多个业务方，userId 在 appId 内唯一，配置按 appId | **①**。现状就是单租户（一把 `IM_ADMIN_KEY`、`users` 无 `app_id`），PRD-admin 也把多租户放在 P2。如果几个 App 的用户需要互相聊天（同一账号体系），共用一个部署即可；如果是不同账号体系，各自部署 | ①：零额外工作；以后改多租户要给全部表的主键加 `app_id`、token 带 appId、缓存键和限流键带 appId，约 8～10 人日并需要数据迁移。②：现在做约多 6～8 人日（服务端 + 管理接口 + 契约测试），M3 更紧 |
| **S-5** | 403x 号段只剩 4038、4039，社交和后台都要用 | ① 按第 9.4 节分配：社交 4035～4037，后台 CSRF / STEP_UP 用 4038 / 4039、锁定用 4291；② 后台专用码换一个号段（如 4031 以外全部改用 HTTP 403 + 通用码 4030 + `reason` 字段） | **①**，两份 PRD 一起定稿后一次写入 errors.json | errors.json 只增不改，选错不能回头；①之后 403x 用完，以后的无权限类只能用 4030 + `reason` |
| **S-6** | S-3 要求"三套 SDK 第一批"，但 sdk-kotlin、sdk-swift 还没开始（M5 / M6） | ① M3 完成协议、服务端、sdk-ts、H5 和共享测试数据 `social.json`；Kotlin / Swift SDK 在 M5 / M6 **首版就包含社交**，不单独排期。② M3 就开始写 Kotlin / Swift SDK（核心 + 社交） | **①** | ①：分享会前原生端没有社交功能（原生端本来也在 M5 / M6）；靠共享测试数据保证三套一致。②：M3 多约 20～25 人日（两套 SDK 的核心部分都要提前），分享会前风险大 |
| **S-7** | M3 工作量：本 PRD 约 55 人日，DECISIONS 2.5 节只估了服务端 1.5～2 周（不含群聊补缺） | ① 全部 23 个功能点进 M3；② M3 做好友、拉黑、通讯录、私聊权限、搜索、资料（F-SOC-01～12）+ 群的建群 / 邀请 / 退群 / 移出 / 解散 / 改名 / 成员列表 / 提示消息；**群管理员（F-SOC-25 后半）、群公告（F-SOC-27 的公告部分）**放 M4 | **②**：演示的关键路径都在；管理员和公告不影响协议的其他部分（字段先留好） | ①：M3 延后约 1.5 周；②：M3 少约 4 人日，M4 补；PRD-ui 的群设置页分两次做 |

已直接写进规则、不需要拍板的产品细节：单向删除（F-SOC-05）；拉黑的暴露方式（F-SOC-07）；群里拉黑不生效；开放模式陌生人 3 条；邀请不需要同意；群主退群自动转让；提示消息计入未读；默认可被搜索；只按 userId 精确搜索；默认开放模式。

---

## 15. 工作量估算（人日）

| 部分 | 内容 | 人日 |
|---|---|---|
| 服务端：存储 | 0004 / 0005 迁移与回填、Store 新方法、契约测试 15 个 | 4 |
| 服务端：好友与同步 | 申请（合并、互相申请、静默）、处理、删除、备注、ver / 墓碑 / socialSync、红点、清理任务 | 4 |
| 服务端：拉黑与私聊权限 | 拉黑、发送路径判定、陌生人限额、新会话限流 | 2 |
| 服务端：搜索与资料 | `userSearch`、`setPrivacy`、`users` 与可见范围 | 1.5 |
| 服务端：群 | 角色、建群（去重）、邀请、退群（自动转让 / 解散）、移出、转让、管理员、解散、改名公告、成员分页、提示消息 | 5 |
| 服务端：管理接口与配置 | 配置读写与失效、好友导入 / 删除 / 查询、群接口扩展 | 2 |
| 服务端：协议 | Schema、vectors、errors.json、ADMIN.md、协议 README | 1 |
| 服务端：e2e | 两种模式 × 第 6、7 节后端 AC | 1.5 |
| **服务端小计** | | **21** |
| sdk-ts | 社交状态与同步、好友 / 拉黑 / 搜索 / 资料 API、群 API、tip、单元测试 + 集成测试 | 7 |
| H5 | 13.2 节 13 个页面 / 改动、错误提示、红点 | 10 |
| sdk-kotlin | 在其首版之上增加社交（同 sdk-ts 的规则，跑 `social.json`） | 6 |
| sdk-swift | 同上 | 6 |
| 测试 | TC-SOC 用例编写、两种模式回归、安全用例（枚举、日志扫描）、双节点失效用例 | 5 |
| **合计** | | **55**（M3 范围内为 43：不含两套原生 SDK 的 12） |

按 S-7 ② 执行时，M3 减少约 4 人日（服务端 2、sdk-ts 0.5、H5 1.5）。

---

## 16. 测试要点（每个功能点另写 TC-SOC 用例，这里只列关键场景）

| 场景 | 要点 |
|---|---|
| 两种模式全量回归 | PRD-core 私聊 e2e 在 `open`（`strangerMsgLimit=0`）下全过；`friends` 下非好友全部 4035、好友全部通过 |
| 申请状态机 | 申请 → 合并 → 拒绝 → 再申请 → 同意；互相申请；单向删除后的两种再申请；过期边界（第 7 天 ±1 秒，测试时钟） |
| 多端与离线 | 两端同时在线 / 一端离线 / 事件被丢弃，三种情况下关系最终一致；红点多端清零 |
| 并发 | 双方同时同意 / 同时申请；接近好友上限时并发同意不突破；同一用户并发操作 ver 连续 |
| 拉黑竞态 | 拉黑 ack 之后立即发送的消息一定被拒；拉黑前已保存消息的重发仍成功 |
| 防枚举 | 搜索、资料、申请三条路径对"不存在 / 不可见 / 被拉黑"的应答逐字节比较；限流与配额边界 |
| 陌生人限额 | 第 N、N+1 条；对方回复后解除；4036 不计数；给自己发不计数 |
| 群权限矩阵 | 第 F-SOC-20 规则 2 的每一格（3 种角色 + 托管群 × 7 类操作） |
| 群主退群 | 有管理员 / 无管理员 / 只剩一人；管理接口移除群主 |
| 解散 | 客户端和管理接口两条路径效果一致；群 ID 不复用；离线成员上线后的状态 |
| 提示消息 | seq 连续、historyVisible=false 时新成员能看到自己的入群提示、计入未读、业务方批量加人只一条 |
| 分布式 | 双节点：配置修改、群成员缓存失效、跨节点 `socialChanged` 投递；关掉失效广播后 TTL 兜底 |
| 迁移 | 在已有 0003 数据的库上执行 0004 / 0005，`sent_count`、`member_count` 回填正确；契约测试 SQLite 全过 |
| 安全 | 日志扫描（附言、备注、关键字、公告特征串）；XSS 字符串在附言、备注、群名、公告中原样显示 |
| SDK 一致性 | `vectors/social.json` 三套 SDK 结果一致；错误码枚举一致性测试覆盖新码 |

---

## 17. 变更记录

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v0.1 | 2026-11-03 | 初稿：按 S-1～S-3 起草好友、拉黑、通讯录、搜索、资料、私聊权限、群聊补缺（F-SOC-01～12、20～30），数据模型 0004 / 0005，协议与错误码，SDK、安全、分布式、文档影响，待拍板 S-4～S-7 | 产品（AI） |
