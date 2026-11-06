# 测试用例：社交关系与群聊补缺（SOC）

> 依据：[PRD-social v1.0](../prd/PRD-social-v1.1.md)（已冻结）F-SOC-01～13（M3）、F-SOC-20～30（M4），5.4 节"操作 × ver"、5.6 节测试设施、8.3 节加锁规则、8.7 节并发测试、11.2 节存在性探测预算；[PRD-core v1.3](../prd/PRD-core-v1.3.md) 第 11～13 节；[ERRORS.md](../../protocol/ERRORS.md)。任务见 [backend-social.md](../tasks/backend-social.md)、[frontend-social.md](../tasks/frontend-social.md)。
>
> 通用约定：
> - 服务端测试模式（`IM_TEST_MODE=1`）；每个用例使用独立数据库，用例开始时对涉及的用户调用 `POST /admin/test/limits/reset`。社交管理接口在 X-5 落地前只在测试模式注册，用例经旧管理密钥调用；X-5 之后改发到 `IM_ADMIN_ADDR`。
> - 用户 alice、bob、carol、dave、eve、frank、kefu、op1 已导入，昵称与 userId 相同（另有说明除外）；`nobody`、`n001`… 等不存在。批量用户用 `seed` 或分批导入。
> - 没有特别说明时为默认配置：`mode=open`、`strangerMsgLimit=0`、`newPeerHourlyLimit=0`、`newPeerRejectDailyLimit=100`、其余见 5.1 节。CI 中后端用例在 `mode=open`、`mode=friends`（预置好友关系）各跑一遍（PRD 1.3 节），只对模式敏感的用例写明模式。
> - "收不到任何帧"指：操作 ack 之后 **2 秒内**，该连接收不到任何帧（pong 除外）（5.6 节）。"应答相同"指：JSON 去掉 `id`（以及写明的 `ver`、时间戳字段）后逐字段 / 逐字节相同。
> - "测试时钟"指 `POST /admin/test/clock`（冻结时间，IMP-7）；`seed`、`purge`、`drop-events`、`limits/reset` 是 5.6 节的测试接口（BE-103）。
> - 层级：**后端 e2e**（`server/e2e`，真实 WS / HTTP）；**后端 e2e（安全）**（11.2 节防枚举、隐私、日志扫描）；**存储契约**（`store/storetest`，SQLite 每次跑，`IM_TEST_PG_DSN` 存在时跑 Postgres）；**Postgres 并发（B6 前置）**（8.7 节，测试代码随功能写好、在 SQLite 上也运行，B6 上线前必须在 Postgres 上通过）；**双节点 e2e**（5.6 节）；**迁移测试**；**协议测试**；**模型测试**（16 节同步模型）；**压测**（同一基准机器 3 次取中位数）；**SDK 单测**（FakeTransport + FakeClock，含 `vectors/social.json`，三套 SDK 都跑）；**SDK 集成**（sdk-ts 对真实 im-server）；**UI 自动化**（PRD-ui 页面，Playwright；页面由 PRD-ui 任务实现）。
>
> **11.2 节逐路径的防枚举用例**：`send` 被拒 → TC-SOC-08-12、08-13、08-17、13-08；`userSearch` → TC-SOC-09-04、09-10；`friendApply` → TC-SOC-02-07、02-09；`block` / `unblock` → TC-SOC-07-11；`users` → TC-SOC-10-01、10-06、10-07；`friendRemark` / `friendDelete` / `friendRespond` → TC-SOC-05-06、05-04、03-08；`groupCreate` / `groupInvite` → TC-SOC-21-07、22-02。
>
> **默认配置不影响现有用例**：TC-SOC-08-15（现有 `server/e2e` 不改代码全过）、TC-SOC-08-16（TC-PERF-00-01～07 不回归）、TC-SOC-29-03（管理接口群操作不产生提示消息，TC-GRP 不改）。
>
> **标"B6 前置"的 9 条 AC** → TC-SOC-02-16、03-10、03-11、08-14、11-08、20-09、22-07、23-09、23-10。

共 243 个用例：P0 220 个、P1 23 个。M3（F-SOC-01～13）156 个，M4（F-SOC-20～30）87 个（F-SOC-26 的 M3 部分用例 TC-SOC-26-03、26-05～26-07 先在 M3 用管理接口解散跑一遍）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-SOC-01-01 | 新部署返回全部默认配置 | AC-SOC-01-1 | 后端 e2e | P0 |
| TC-SOC-01-02 | 切到好友制 1 秒内生效，无需重启 | AC-SOC-01-2 | 后端 e2e | P0 |
| TC-SOC-01-03 | 非法配置返回 400 / 4000，整次不生效 | AC-SOC-01-3 | 后端 e2e | P0 |
| TC-SOC-01-04 | 切到好友制后历史会话可见可拉、不能发；切回恢复 | AC-SOC-01-4 | 后端 e2e | P0 |
| TC-SOC-01-05 | 待处理申请跨模式切换仍有效 | AC-SOC-01-4 | 后端 e2e | P1 |
| TC-SOC-01-06 | 双节点：节点 1 改模式，节点 2 1 秒内按新模式判定 | AC-SOC-01-5 | 双节点 e2e | P0 |
| TC-SOC-01-07 | 双节点关闭广播：TTL 60 秒兜底 | AC-SOC-01-5 | 双节点 e2e | P0 |
| TC-SOC-01-08 | 改陌生人限额：两个在线连接收到 socialSettingsChanged，SDK 回调新值 | AC-SOC-01-6 | SDK 集成 | P0 |
| TC-SOC-01-09 | 离线期间改配置，重连后从同步应答头拿到新值 | AC-SOC-01-6 | SDK 集成 | P1 |
| TC-SOC-01-10 | 陌生人条数按累计口径：0 改为 3 后立即受限 | AC-SOC-01-7 | 后端 e2e | P0 |
| TC-SOC-01-11 | 调低好友上限：保留已有，拦截新增，降到上限以下恢复 | AC-SOC-01-8 | 后端 e2e | P0 |
| TC-SOC-01-12 | 启动配置只在首次写入，之后以数据库为准 | AC-SOC-01-9 | 后端 e2e（子进程） | P0 |
| TC-SOC-02-01 | 发送申请：两端收到事件，同步后看到附言、来源、昵称 | AC-SOC-02-1 | 后端 e2e | P0 |
| TC-SOC-02-02 | SDK：收到事件后自动同步，"新的朋友"出现申请 | AC-SOC-02-1 | SDK 集成 | P0 |
| TC-SOC-02-03 | 同一天重复申请合并，第 4 次 4292 | AC-SOC-02-2 | 后端 e2e | P0 |
| TC-SOC-02-04 | 被拒绝、过期后再申请复用同一对申请行 | AC-SOC-02-2 | 后端 e2e | P1 |
| TC-SOC-02-05 | 互相申请自动成为好友，双方 ver 各加 1 | AC-SOC-02-3 | 后端 e2e | P0 |
| TC-SOC-02-06 | 已互为好友时再申请：accepted、不改数据、无事件 | AC-SOC-02-3 | 后端 e2e | P1 |
| TC-SOC-02-07 | 防枚举：申请拉黑了我的人与正常申请的应答同构 | AC-SOC-02-4 | 后端 e2e（安全） | P0 |
| TC-SOC-02-08 | 格式错误：给自己、附言超长、to 不合法 | AC-SOC-02-5 | 后端 e2e | P0 |
| TC-SOC-02-09 | 申请不存在的用户：同构 pending，之后导入也看不到 | AC-SOC-02-6 | 后端 e2e（安全） | P0 |
| TC-SOC-02-10 | 好友已满：申请任何人 4132；对方已满时申请仍为待处理 | AC-SOC-02-7 | 后端 e2e | P0 |
| TC-SOC-02-11 | 来源为群：服务端校验，申请人一侧原样记录 | AC-SOC-02-8 | 后端 e2e | P0 |
| TC-SOC-02-12 | 单向删除后的两种再申请 | AC-SOC-02-9 | 后端 e2e | P0 |
| TC-SOC-02-13 | 申请限流 4290 与每日配额 4292 | AC-SOC-02-10 | 后端 e2e | P0 |
| TC-SOC-02-14 | 日志不出现附言和预设备注原文 | AC-SOC-02-11 | 后端 e2e（安全） | P0 |
| TC-SOC-02-15 | 申请自己拉黑的人返回 4000，取消拉黑后可以申请 | AC-SOC-02-12 | 后端 e2e | P0 |
| TC-SOC-02-16 | 互相申请并发：100 组都成为好友、无死锁（B6 前置） | AC-SOC-02-13 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-02-17 | 判定表 9 行逐行（存储契约） | AC-SOC-02-3、AC-SOC-02-4、AC-SOC-02-6、AC-SOC-02-7、AC-SOC-02-9 | 存储契约 | P0 |
| TC-SOC-03-01 | 同意申请：多端事件、备注、双方好友列表 | AC-SOC-03-1 | 后端 e2e | P0 |
| TC-SOC-03-02 | SDK：同意后两端好友列表更新 | AC-SOC-03-1 | SDK 集成 | P0 |
| TC-SOC-03-03 | 拉黑对方时仍可同意，黑名单不变 | AC-SOC-03-1 | 后端 e2e | P1 |
| TC-SOC-03-04 | 申请时的预设备注在同意后生效 | AC-SOC-03-2 | 后端 e2e | P0 |
| TC-SOC-03-05 | 拒绝：申请人仍显示等待验证且无感知 | AC-SOC-03-3 | 后端 e2e | P0 |
| TC-SOC-03-06 | 过期边界：7 天减 1 秒可同意，7 天加 1 秒 4045 | AC-SOC-03-4 | 后端 e2e | P0 |
| TC-SOC-03-07 | 拒绝后改同意；已同意再拒绝 4091 | AC-SOC-03-5 | 后端 e2e | P0 |
| TC-SOC-03-08 | 防枚举：不存在、已过期、申请时被拉黑的 4045 响应体相同 | AC-SOC-03-6 | 后端 e2e（安全） | P0 |
| TC-SOC-03-09 | 重复同意幂等，ver 只加 1 | AC-SOC-03-7 | 后端 e2e | P0 |
| TC-SOC-03-10 | 好友上限并发：每轮恰好一个成功（B6 前置） | AC-SOC-03-8 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-03-11 | 互相同意并发：100 轮无死锁（B6 前置） | AC-SOC-03-9 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-04-01 | SDK 红点数与列表排序 | AC-SOC-04-1 | SDK 集成 | P0 |
| TC-SOC-04-02 | 一端标记已读，另一端 2 秒内红点清零 | AC-SOC-04-2 | SDK 集成 | P0 |
| TC-SOC-04-03 | 标记已读后对方再次申请（合并）重新计入红点 | AC-SOC-04-3 | 后端 e2e | P0 |
| TC-SOC-04-04 | requestSeenVer 只进不退，截断到当前 ver | AC-SOC-04-4 | 后端 e2e | P0 |
| TC-SOC-04-05 | 已过期申请显示"已过期"、无"同意"按钮、不计红点 | AC-SOC-04-5 | UI 自动化 | P0 |
| TC-SOC-04-06 | SDK：过期判断用服务端时间，不用本地时钟 | AC-SOC-04-5 | SDK 单测 | P0 |
| TC-SOC-04-07 | 30 天查询过滤（不执行清理） | AC-SOC-04-6 | 后端 e2e | P0 |
| TC-SOC-04-08 | 清理任务删除超过 30 天的申请并推进 purgedVer | AC-SOC-04-7 | 后端 e2e | P0 |
| TC-SOC-05-01 | 单向删除：对方列表不变、无感知 | AC-SOC-05-1 | 后端 e2e | P0 |
| TC-SOC-05-02 | SDK：删除后本端立即移除，另一端同步移除 | AC-SOC-05-1 | SDK 集成 | P1 |
| TC-SOC-05-03 | 好友制下删除后双方都不能私聊，历史可拉 | AC-SOC-05-2 | 后端 e2e | P0 |
| TC-SOC-05-04 | 删除不是好友的人、不存在的人：幂等成功 | AC-SOC-05-3 | 后端 e2e（安全） | P0 |
| TC-SOC-05-05 | 备注多端同步，且在对方和管理接口里都看不到 | AC-SOC-05-4 | SDK 集成 + 后端 e2e | P0 |
| TC-SOC-05-06 | 备注边界、非好友 4040 同构、空白清除 | AC-SOC-05-5 | 后端 e2e（安全） | P0 |
| TC-SOC-05-07 | 备注值未变：changed=false、ver 不变 | AC-SOC-05-6 | 后端 e2e | P0 |
| TC-SOC-06-01 | 离线补拉：一页 4 项、3 个 ver | AC-SOC-06-1 | 后端 e2e | P0 |
| TC-SOC-06-02 | 多端：pc 端同意，web 端 2 秒内出现好友 | AC-SOC-06-2 | SDK 集成 | P0 |
| TC-SOC-06-03 | 断网期间的变化重连后 5 秒内补齐 | AC-SOC-06-2 | SDK 集成 | P0 |
| TC-SOC-06-04 | 事件被丢弃：重连后的同步轮包含变化 | AC-SOC-06-3 | 后端 e2e | P0 |
| TC-SOC-06-05 | 事件被丢弃：下一次事件触发的同步轮一并补齐 | AC-SOC-06-3 | SDK 集成 | P0 |
| TC-SOC-06-06 | 同一个 ver 跨页：不丢不重 | AC-SOC-06-4 | 后端 e2e | P0 |
| TC-SOC-06-07 | 全量分页：2500 个好友 5 页 | AC-SOC-06-5 | 后端 e2e | P0 |
| TC-SOC-06-08 | 同步轮进行中删除、新增：进入下一轮 | AC-SOC-06-6 | 后端 e2e | P0 |
| TC-SOC-06-09 | 同步轮进行中删除第 1 页已返回的好友 | AC-SOC-06-6 | 后端 e2e | P0 |
| TC-SOC-06-10 | 墓碑、清理与全量判定 | AC-SOC-06-7 | 后端 e2e | P0 |
| TC-SOC-06-11 | 清理发生在增量轮中途：restart | AC-SOC-06-8 | 后端 e2e | P0 |
| TC-SOC-06-12 | SDK：restart、purgedVer 全量与 30 天丢弃（共享用例） | AC-SOC-06-8、AC-SOC-06-7 | SDK 单测（vectors/social.json） | P0 |
| TC-SOC-06-13 | 并发同意 10 人：ver 恰好 +10、20 项分布在 10 个 ver | AC-SOC-06-9 | 后端 e2e | P0 |
| TC-SOC-06-14 | 版本号每操作一个（5.4 节逐行，存储契约） | AC-SOC-06-9、AC-SOC-03-7 | 存储契约 | P0 |
| TC-SOC-06-15 | 参数校验 | AC-SOC-06-10 | 后端 e2e | P0 |
| TC-SOC-06-16 | 修改类命令的 ack | AC-SOC-06-11 | 后端 e2e | P0 |
| TC-SOC-06-17 | SDK：ack 的 ver 不连续时先应用 items 再补一轮 | AC-SOC-06-11 | SDK 单测（vectors/social.json） | P0 |
| TC-SOC-06-18 | 全量同步性能 | AC-SOC-06-12 | 压测 | P1 |
| TC-SOC-06-19 | 全量轮限流：1 分钟内第 11 轮 4290，增量不受限 | AC-SOC-06-13 | 后端 e2e | P0 |
| TC-SOC-06-20 | H5 刷新后 2000 好友 2 秒内显示 | AC-SOC-06-14 | UI 自动化（性能） | P1 |
| TC-SOC-06-21 | 同步模型测试 1000 个随机序列 | AC-SOC-06-15 | 模型测试 | P0 |
| TC-SOC-07-01 | 被拉黑后私聊 4036，对方收不到、不保存 | AC-SOC-07-1 | 后端 e2e | P0 |
| TC-SOC-07-02 | SDK：4036 标记失败、提示"被对方拒收"、不自动重发 | AC-SOC-07-1 | SDK 集成 | P0 |
| TC-SOC-07-03 | 拉黑者自己给对方发私聊照常成功 | AC-SOC-07-2 | 后端 e2e | P0 |
| TC-SOC-07-04 | 群里拉黑不生效 | AC-SOC-07-3 | 后端 e2e | P0 |
| TC-SOC-07-05 | 拉黑把对方的待处理申请置为 rejected | AC-SOC-07-4 | 后端 e2e | P0 |
| TC-SOC-07-06 | 取消拉黑不恢复申请 | AC-SOC-07-4 | 后端 e2e | P1 |
| TC-SOC-07-07 | 黑名单多端同步；取消后对方可以私聊 | AC-SOC-07-5 | SDK 集成 | P0 |
| TC-SOC-07-08 | 拉黑边界：自己、黑名单满、取消不在名单的人 | AC-SOC-07-6 | 后端 e2e | P0 |
| TC-SOC-07-09 | 拉黑不改变好友关系 | AC-SOC-07-7 | 后端 e2e | P0 |
| TC-SOC-07-10 | 拉黑静默；对方的搜索和资料结果不变 | AC-SOC-07-8 | 后端 e2e（安全） | P0 |
| TC-SOC-07-11 | 防枚举：拉黑不存在的人与拉黑无关系、不可被搜索的人同构 | AC-SOC-07-9 | 后端 e2e（安全） | P0 |
| TC-SOC-07-12 | 拉黑每日配额 4292 | AC-SOC-07-10 | 后端 e2e | P0 |
| TC-SOC-07-13 | 拉黑 ack 之后立即发送一定被拒（100 轮） | AC-SOC-07-11 | 后端 e2e | P0 |
| TC-SOC-08-01 | 好友制：互为好友可发，单向边不可发 | AC-SOC-08-1 | 后端 e2e | P0 |
| TC-SOC-08-02 | 开放模式陌生人限额：第 4 条 4137，对方回复后解除 | AC-SOC-08-2 | 后端 e2e | P0 |
| TC-SOC-08-03 | 回复解除是永久的 | AC-SOC-08-2 | 后端 e2e | P1 |
| TC-SOC-08-04 | 默认配置：陌生人 10 条、150 个新对象都成功 | AC-SOC-08-3 | 后端 e2e | P0 |
| TC-SOC-08-05 | 去重优先：已保存的消息被拉黑后重发仍返回原 ack | AC-SOC-08-4 | 后端 e2e | P0 |
| TC-SOC-08-06 | 判定顺序：4042 > 4035 > 4036 | AC-SOC-08-5 | 后端 e2e | P0 |
| TC-SOC-08-07 | 每小时新对象配额 | AC-SOC-08-6 | 后端 e2e | P0 |
| TC-SOC-08-08 | 给自己发消息：好友制下也成功、不扣新对象配额 | AC-SOC-08-7 | 后端 e2e | P0 |
| TC-SOC-08-09 | 聊天页提示：4035 好友验证提示与申请入口；4137 中的 N | AC-SOC-08-8 | UI 自动化 | P0 |
| TC-SOC-08-10 | SDK：权限类错误码透传、标记失败、不自动重发 | AC-SOC-08-8 | SDK 单测 | P0 |
| TC-SOC-08-11 | 被 4036 拒绝的消息不计入陌生人条数 | AC-SOC-08-9 | 后端 e2e | P0 |
| TC-SOC-08-12 | 新对象被拒配额：100 次静默失败后对任何新对象 4292 | AC-SOC-08-10 | 后端 e2e（安全） | P0 |
| TC-SOC-08-13 | 好友制下给 100 个非好友发：之后 4292 | AC-SOC-08-11 | 后端 e2e（安全） | P0 |
| TC-SOC-08-14 | 陌生人限额并发：恰好 3 条成功、seq 连续（B6 前置） | AC-SOC-08-12 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-08-15 | 默认配置下现有 e2e 不改代码全部通过 | AC-SOC-08-13 | 后端 e2e（回归） | P0 |
| TC-SOC-08-16 | 默认配置下 TC-PERF 错误码与 P99 不回归 | AC-SOC-08-13 | 压测 | P0 |
| TC-SOC-08-17 | 新对象被拒配额并发：10 连接同时探测不突破 | AC-SOC-08-14 | 后端 e2e（安全） | P0 |
| TC-SOC-08-18 | 预扣退还：成功发送后 newPeerReject 不变，发送令牌少 1 | AC-SOC-08-15 | 后端 e2e | P0 |
| TC-SOC-08-19 | 被发送限流 4290 拒绝时不扣新对象配额 | AC-SOC-08-15 | 后端 e2e | P1 |
| TC-SOC-09-01 | 搜索返回资料与关系，申请后变为 pendingOut | AC-SOC-09-1 | SDK 集成 | P0 |
| TC-SOC-09-02 | relation 枚举按优先级取值 | AC-SOC-09-1 | 后端 e2e | P1 |
| TC-SOC-09-03 | 只做精确匹配，前缀返回 null | AC-SOC-09-2 | 后端 e2e | P0 |
| TC-SOC-09-04 | 防枚举：不可被搜索与不存在逐字节相同 | AC-SOC-09-3 | 后端 e2e（安全） | P0 |
| TC-SOC-09-05 | 搜索限流 4290 与每日配额 4292 | AC-SOC-09-4 | 后端 e2e | P0 |
| TC-SOC-09-06 | 被对方拉黑后搜索结果不变 | AC-SOC-09-5 | 后端 e2e（安全） | P0 |
| TC-SOC-09-07 | 关键字格式、搜索自己、搜索被我拉黑的人 | AC-SOC-09-6 | 后端 e2e | P0 |
| TC-SOC-09-08 | 隐私开关多端同步 | AC-SOC-09-7 | SDK 集成 | P0 |
| TC-SOC-09-09 | 日志不出现搜索关键字 | AC-SOC-09-8 | 后端 e2e（安全） | P0 |
| TC-SOC-09-10 | 响应时间不可区分 | AC-SOC-09-9 | 后端基准测试 | P1 |
| TC-SOC-10-01 | 可见范围：同群、有单聊消息的人可见；无关和不存在的不返回 | AC-SOC-10-1 | 后端 e2e | P0 |
| TC-SOC-10-02 | 好友、自己、30 天内收到的申请可见 | AC-SOC-10-1 | 后端 e2e | P1 |
| TC-SOC-10-03 | 被移出群后只在群里认识的人不可见 | AC-SOC-10-2 | 后端 e2e | P0 |
| TC-SOC-10-04 | 参数与限流 | AC-SOC-10-3 | 后端 e2e | P0 |
| TC-SOC-10-05 | 收到的申请 30 天后不再可见（不执行清理） | AC-SOC-10-4 | 后端 e2e | P0 |
| TC-SOC-10-06 | 防枚举：申请不可被搜索的人后仍拿不到资料 | AC-SOC-10-5 | 后端 e2e（安全） | P0 |
| TC-SOC-10-07 | 拉黑不授予可见 | AC-SOC-10-6 | 后端 e2e（安全） | P0 |
| TC-SOC-10-08 | 群消息显示发送者昵称（来自 users） | AC-SOC-10-7 | UI 自动化 | P0 |
| TC-SOC-10-09 | SDK：getUsers 合并请求、每批 100、命中缓存不请求 | AC-SOC-10-7 | SDK 单测 | P0 |
| TC-SOC-10-10 | SDK 资料缓存 10 分钟过期 | AC-SOC-10-8 | SDK 单测 | P0 |
| TC-SOC-10-11 | 同步应答带新资料时立即覆盖缓存 | AC-SOC-10-9 | SDK 集成 | P0 |
| TC-SOC-11-01 | memberCount 随管理接口加减成员变化 | AC-SOC-11-1 | 后端 e2e | P0 |
| TC-SOC-11-02 | 600 个群：convs 截断，myGroups 翻 3 页拿全 | AC-SOC-11-2 | 后端 e2e | P0 |
| TC-SOC-11-03 | myGroups 参数与限流 | AC-SOC-11-2 | 后端 e2e | P1 |
| TC-SOC-11-04 | 通讯录显示好友、群聊、新的朋友红点 | AC-SOC-11-3 | UI 自动化 | P0 |
| TC-SOC-11-05 | 开放模式下陌生人不进通讯录 | AC-SOC-11-4 | UI 自动化 | P0 |
| TC-SOC-11-06 | 0004 迁移回填成员数 | AC-SOC-11-5 | 迁移测试 | P0 |
| TC-SOC-11-07 | repair counters 修复漂移且幂等 | AC-SOC-11-5 | 存储契约 | P1 |
| TC-SOC-11-08 | 并发加减成员后 memberCount 准确（B6 前置） | AC-SOC-11-6 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-12-01 | 导入一对好友 | AC-SOC-12-1 | 后端 e2e | P0 |
| TC-SOC-12-02 | 导入不受拉黑影响，也不解除拉黑 | AC-SOC-12-1 | 后端 e2e | P1 |
| TC-SOC-12-03 | 重复导入幂等；备注变化才算变化 | AC-SOC-12-2 | 后端 e2e | P0 |
| TC-SOC-12-04 | 数量上限、用户不存在、a==b | AC-SOC-12-3 | 后端 e2e | P0 |
| TC-SOC-12-05 | 导入后超过 maxFriends 整批不生效 | AC-SOC-12-4 | 后端 e2e | P0 |
| TC-SOC-12-06 | 管理接口删除好友：双向删除 | AC-SOC-12-5 | 后端 e2e | P0 |
| TC-SOC-12-07 | 查询好友分页、不返回备注 | AC-SOC-12-6 | 后端 e2e | P0 |
| TC-SOC-12-08 | 一批导入中同一用户出现 3 次，ver 只加 1 | AC-SOC-12-7 | 后端 e2e | P0 |
| TC-SOC-12-09 | 鉴权与 scope | AC-SOC-12-8 | 后端 e2e | P0 |
| TC-SOC-12-10 | 审计记录与备注特征串 | AC-SOC-12-9 | 后端 e2e（安全） | P0 |
| TC-SOC-12-11 | 生产配置下社交管理接口未注册 | AC-SOC-12-10 | 后端 e2e（子进程） | P0 |
| TC-SOC-13-01 | 好友制下官方账号与非好友互发 | AC-SOC-13-1 | 后端 e2e | P0 |
| TC-SOC-13-02 | 官方账号豁免陌生人限额和每小时新对象数 | AC-SOC-13-2 | 后端 e2e | P0 |
| TC-SOC-13-03 | 官方账号被拉黑后 4036 | AC-SOC-13-3 | 后端 e2e | P0 |
| TC-SOC-13-04 | 取消官方标记 1 秒内按普通规则判定 | AC-SOC-13-4 | 后端 e2e | P0 |
| TC-SOC-13-05 | 资料与应答头带 official | AC-SOC-13-5 | 后端 e2e | P0 |
| TC-SOC-13-06 | 标记不存在的用户；标记与取消都审计 | AC-SOC-13-6 | 后端 e2e | P0 |
| TC-SOC-13-07 | 聊天页标题和搜索结果显示"官方"标识 | AC-SOC-13-7 | UI 自动化 | P0 |
| TC-SOC-13-08 | 官方账号不豁免新对象被拒配额 | AC-SOC-13-8 | 后端 e2e（安全） | P0 |
| TC-SOC-20-01 | 自建群 groupInfo 与 convs 的角色 | AC-SOC-20-1 | 后端 e2e | P0 |
| TC-SOC-20-02 | 托管群 groupInfo | AC-SOC-20-2 | 后端 e2e | P0 |
| TC-SOC-20-03 | 非成员 4033、已解散 4043 | AC-SOC-20-3 | 后端 e2e | P0 |
| TC-SOC-20-04 | 成员修改群名 4030 | AC-SOC-20-4 | 后端 e2e | P0 |
| TC-SOC-20-05 | 管理员移出管理员 4030 | AC-SOC-20-5 | 后端 e2e | P0 |
| TC-SOC-20-06 | 管理员转让群主 4030 | AC-SOC-20-6 | 后端 e2e | P0 |
| TC-SOC-20-07 | 托管群（无群主）成员邀请 4030 | AC-SOC-20-7 | 后端 e2e | P0 |
| TC-SOC-20-08 | 权限矩阵全量遍历（5 × 7） | AC-SOC-20-8 | 后端 e2e（表驱动） | P0 |
| TC-SOC-20-09 | 并发邀请与移出后 memberCount 准确（B6 前置） | AC-SOC-20-9 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-20-10 | groupChanged 1 秒合并 | AC-SOC-20-10 | 后端 e2e | P0 |
| TC-SOC-20-11 | SDK：收到 groupChanged 后用 groupInfo 更新本地会话 | AC-SOC-20-10 | SDK 单测 | P0 |
| TC-SOC-21-01 | 建群：被邀请人和本人其他端收到 convAdded，第一条为 created 提示 | AC-SOC-21-1 | 后端 e2e | P0 |
| TC-SOC-21-02 | SDK：createGroup 后界面得到会话 | AC-SOC-21-1 | SDK 集成 | P1 |
| TC-SOC-21-03 | 同一 clientReqId 重复提交只建一个群 | AC-SOC-21-2 | 后端 e2e | P0 |
| TC-SOC-21-04 | SDK：超时重试使用同一个 clientReqId | AC-SOC-21-2 | SDK 单测 | P0 |
| TC-SOC-21-05 | 成员数不足、包含自己 | AC-SOC-21-3 | 后端 e2e | P0 |
| TC-SOC-21-06 | 包含非好友：4035 + userId，群未创建，无事件 | AC-SOC-21-4 | 后端 e2e | P0 |
| TC-SOC-21-07 | 防枚举：拉黑了我、不存在、非好友同构 | AC-SOC-21-5 | 后端 e2e（安全） | P0 |
| TC-SOC-21-08 | 自建群人数上限 | AC-SOC-21-6 | 后端 e2e | P0 |
| TC-SOC-21-09 | 默认群名用昵称，不用备注 | AC-SOC-21-7 | 后端 e2e | P0 |
| TC-SOC-21-10 | 默认群名超过 50 码点截断 | AC-SOC-21-7 | 后端 e2e | P1 |
| TC-SOC-21-11 | 关闭客户端建群；每日建群配额 | AC-SOC-21-8 | 后端 e2e | P0 |
| TC-SOC-21-12 | 被邀请人加入的群数已满：4135 + userId | AC-SOC-21-9 | 后端 e2e | P0 |
| TC-SOC-21-13 | 业务方不能使用 ug- 前缀 | AC-SOC-21-10 | 后端 e2e | P0 |
| TC-SOC-21-14 | 失败不记录，同一 clientReqId 重试重新判定 | AC-SOC-21-11 | 后端 e2e | P0 |
| TC-SOC-21-15 | 0005 迁移预检：已有 ug- 群 ID 时中止 | AC-SOC-21-10 | 迁移测试 | P1 |
| TC-SOC-22-01 | 成员邀请好友：convAdded 与 joined 提示 | AC-SOC-22-1 | 后端 e2e | P0 |
| TC-SOC-22-02 | 部分成功：非好友、拉黑了我、不存在都是 4035 | AC-SOC-22-2 | 后端 e2e（安全） | P0 |
| TC-SOC-22-03 | 邀请已在群里的人：忽略、无事件、无提示 | AC-SOC-22-3 | 后端 e2e | P0 |
| TC-SOC-22-04 | 加完超过上限整次 4130 | AC-SOC-22-4 | 后端 e2e | P0 |
| TC-SOC-22-05 | 客户端邀请：新成员 minSeq 处看到入群提示、未读 1 | AC-SOC-22-5 | 后端 e2e | P0 |
| TC-SOC-22-06 | 参数、非成员、托管群 | AC-SOC-22-6 | 后端 e2e | P0 |
| TC-SOC-22-07 | 邀请与解散并发（B6 前置） | AC-SOC-22-7 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-23-01 | 退群：本人其他端 convRemoved left，之后 4033 | AC-SOC-23-1 | 后端 e2e | P0 |
| TC-SOC-23-02 | SDK：left 原因保留会话展示信息 | AC-SOC-23-1 | SDK 单测 | P1 |
| TC-SOC-23-03 | 群主退群：转给最早的管理员 | AC-SOC-23-2 | 后端 e2e | P0 |
| TC-SOC-23-04 | 群主退群且没有管理员：转给最早入群的成员 | AC-SOC-23-3 | 后端 e2e | P0 |
| TC-SOC-23-05 | 自建群最后一人退群：解散，ID 不能被业务方使用 | AC-SOC-23-4 | 后端 e2e | P0 |
| TC-SOC-23-06 | 托管群退群与最后一人退群后保留空群 | AC-SOC-23-5 | 后端 e2e | P0 |
| TC-SOC-23-07 | 非成员退群 4033 | AC-SOC-23-6 | 后端 e2e | P0 |
| TC-SOC-23-08 | 托管群群主退群：回到无群主 | AC-SOC-23-7 | 后端 e2e | P0 |
| TC-SOC-23-09 | 最后两人同时退群（B6 前置） | AC-SOC-23-8 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-23-10 | 群主退群与转让并发（B6 前置） | AC-SOC-23-9 | Postgres 并发（B6 前置） | P0 |
| TC-SOC-24-01 | 群主移出成员 | AC-SOC-24-1 | 后端 e2e | P0 |
| TC-SOC-24-02 | 管理员的移出权限 | AC-SOC-24-2 | 后端 e2e | P0 |
| TC-SOC-24-03 | 移出自己 4000；非成员不进 removed | AC-SOC-24-3 | 后端 e2e | P0 |
| TC-SOC-24-04 | 被移出的管理员再入群为普通成员 | AC-SOC-24-4 | 后端 e2e | P0 |
| TC-SOC-25-01 | 转让群主 | AC-SOC-25-1 | 后端 e2e | P0 |
| TC-SOC-25-02 | 转让错误：非成员、管理员调用、转给自己 | AC-SOC-25-2 | 后端 e2e | P0 |
| TC-SOC-25-03 | 管理员上限与幂等 | AC-SOC-25-3 | 后端 e2e | P0 |
| TC-SOC-25-04 | 设置 / 取消管理员的提示与错误 | AC-SOC-25-3 | 后端 e2e | P1 |
| TC-SOC-25-05 | 转让给管理员：管理员数减 1 | AC-SOC-25-4 | 后端 e2e | P0 |
| TC-SOC-26-01 | 客户端解散：在线成员 convRemoved dissolved，离线成员上线后不再有该群 | AC-SOC-26-1 | 后端 e2e | P0 |
| TC-SOC-26-02 | SDK：dissolved 原因区分显示 | AC-SOC-26-1 | SDK 单测 | P0 |
| TC-SOC-26-03 | 解散后发、拉、读都 4043，管理接口 404 | AC-SOC-26-2 | 后端 e2e | P0 |
| TC-SOC-26-04 | 解散权限 | AC-SOC-26-3 | 后端 e2e | P0 |
| TC-SOC-26-05 | 存储契约：数据保留与 F-SOC-26 规则 3 的每个查询 | AC-SOC-26-4 | 存储契约 | P0 |
| TC-SOC-26-06 | 解散后同群关系不再授予资料可见 | AC-SOC-26-5 | 后端 e2e | P0 |
| TC-SOC-26-07 | 管理接口解散托管群（M3）：效果相同，ID 不复用 | AC-SOC-26-6 | 后端 e2e | P0 |
| TC-SOC-26-08 | 客户端解散与管理接口解散两条路径结果一致 | AC-SOC-26-1、AC-SOC-26-6 | 存储契约 | P1 |
| TC-SOC-27-01 | 管理员改群名：renamed 提示，下一次 convs 生效 | AC-SOC-27-1 | 后端 e2e | P0 |
| TC-SOC-27-02 | 改群名的权限与参数 | AC-SOC-27-2 | 后端 e2e | P0 |
| TC-SOC-27-03 | 公告长度、清除、noticeBy 与 noticeAt | AC-SOC-27-3 | 后端 e2e | P0 |
| TC-SOC-27-04 | 公告提示显示摘要，点击看全文 | AC-SOC-27-4 | UI 自动化 | P0 |
| TC-SOC-27-05 | 日志不出现公告原文 | AC-SOC-27-5 | 后端 e2e（安全） | P0 |
| TC-SOC-28-01 | 200 人群翻 4 页，第一项是群主 | AC-SOC-28-1 | 后端 e2e | P0 |
| TC-SOC-28-02 | 翻页中已返回的成员退群、新成员入群 | AC-SOC-28-2 | 后端 e2e | P0 |
| TC-SOC-28-03 | 翻页中尚未返回的成员退群 | AC-SOC-28-3 | 后端 e2e | P0 |
| TC-SOC-28-04 | 非成员、limit、伪造游标 | AC-SOC-28-4 | 后端 e2e | P0 |
| TC-SOC-28-05 | 聊天信息页显示前 20 个成员，滚动加载全部 | AC-SOC-28-5 | UI 自动化 | P0 |
| TC-SOC-29-01 | 六种操作依次生成 6 条 tip，seq 连续 | AC-SOC-29-1 | 后端 e2e | P0 |
| TC-SOC-29-02 | 退群、移出不生成提示 | AC-SOC-29-2 | 后端 e2e | P0 |
| TC-SOC-29-03 | 管理接口操作不生成提示（PRD-core AC 不变） | AC-SOC-29-3 | 后端 e2e | P0 |
| TC-SOC-29-04 | 客户端不能发 tip、不能用 tip- 前缀 | AC-SOC-29-4 | 后端 e2e | P0 |
| TC-SOC-29-05 | 个性化渲染与服务端摘要 | AC-SOC-29-5 | UI 自动化 | P0 |
| TC-SOC-29-06 | SDK：未知 kind 显示"[群通知]"，tip 计入未读 | AC-SOC-29-5 | SDK 单测（vectors/social.json） | P0 |
| TC-SOC-29-07 | Schema：tip 允许 from 为空 | AC-SOC-29-6 | 协议测试 | P0 |
| TC-SOC-29-08 | 提示消息推进操作人已读并多端同步 | AC-SOC-29-7 | 后端 e2e | P0 |
| TC-SOC-29-09 | 存储契约：提示消息写入失败整体回滚 | AC-SOC-29-8 | 存储契约 | P0 |
| TC-SOC-29-10 | 提示消息文本只用昵称，不用备注 | AC-SOC-29-9 | 后端 e2e | P0 |
| TC-SOC-30-01 | 建托管群时指定群主 | AC-SOC-30-1 | 后端 e2e | P0 |
| TC-SOC-30-02 | 收回群主 | AC-SOC-30-2 | 后端 e2e | P0 |
| TC-SOC-30-03 | 指定非成员为群主 | AC-SOC-30-3 | 后端 e2e | P0 |
| TC-SOC-30-04 | 管理接口对自建群不受客户端限制、不产生提示 | AC-SOC-30-4 | 后端 e2e | P0 |
| TC-SOC-30-05 | 管理接口移除自建群群主：自动转让给最早的管理员 | AC-SOC-30-5 | 后端 e2e | P0 |
| TC-SOC-30-06 | 管理接口移除托管群群主：回到无群主 | AC-SOC-30-6 | 后端 e2e | P0 |
| TC-SOC-30-07 | 群主交接的审计 | AC-SOC-30-7 | 后端 e2e | P0 |

---

## F-SOC-01 社交模式与业务方配置

### TC-SOC-01-01 新部署返回全部默认配置
- 关联 AC：AC-SOC-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：全新数据库；不设置 `IM_SOCIAL_DEFAULTS`。
- 步骤：`GET /admin/settings/social`。
- 预期结果：HTTP 200；`mode=open`、`strangerMsgLimit=0`、`newPeerHourlyLimit=0`、`newPeerRejectDailyLimit=100`、`maxFriends=2000`、`maxBlocks=1000`、`requestExpireDays=7`、`applyDailyLimit=30`、`applyPerTargetDailyLimit=3`、`blockDailyLimit=100`、`searchableDefault=true`、`searchDailyLimit=100`、`clientGroupEnabled=true`、`clientGroupMaxMembers=500`、`maxJoinedGroups=500`、`groupCreateDailyLimit=10`（5.1 节全部 16 项，不多不少）。

### TC-SOC-01-02 切到好友制 1 秒内生效，无需重启
- 关联 AC：AC-SOC-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式；alice、bob 不是好友；alice 已连接。
- 步骤：
  1. alice 给 bob 发一条私聊（确认开放模式可发）。
  2. `PATCH /admin/settings/social {"mode":"friends"}`。
  3. PATCH 返回后 1 秒内，alice 再给 bob 发一条。
- 预期结果：1 成功；2 返回 200；3 返回 4035，消息不保存（bob 的 `sync` 只有第 1 条），服务端未重启。

### TC-SOC-01-03 非法配置返回 400 / 4000，整次不生效
- 关联 AC：AC-SOC-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：默认配置。
- 步骤：依次 PATCH：①`{"mode":"friends","maxFriends":99}`；②`{"strangerMsgLimit":-1}`；③`{"foo":1}`；④`{"mode":"closed"}`；⑤`{"newPeerRejectDailyLimit":0}`；⑥`{"maxFriends":5001}`；每次之后 GET。
- 预期结果：6 次都 HTTP 400 / 4000；每次 GET 都与默认值完全相同（①中合法的 `mode=friends` 也没有生效，`mode` 仍为 `open`）。

### TC-SOC-01-04 切到好友制后历史会话可见可拉、不能发；切回恢复
- 关联 AC：AC-SOC-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式，alice 与 bob（非好友）互发 5 条（seq 1～5）。
- 步骤：
  1. PATCH `mode=friends`；alice 调 `convs`、`sync {convId: c2c:alice:bob, afterSeq:0}`、`read {seq:5}`；alice `send` 给 bob。
  2. PATCH `mode=open`；alice 再 `send` 给 bob。
- 预期结果：1 中 `convs` 含该会话、`sync` 返回 5 条、`read` 成功、`send` 返回 4035；2 中 `send` 成功，seq=6。

### TC-SOC-01-05 待处理申请跨模式切换仍有效
- 关联 AC：AC-SOC-01-4
- 优先级：P1
- 层级：后端 e2e
- 前置条件：开放模式；alice 已向 bob 发申请（pending）。
- 步骤：PATCH `mode=friends`；bob `friendRespond {from: alice, action: accept}`；alice 给 bob 发私聊。
- 预期结果：同意成功（不是 4045）；双方互为好友；私聊成功。

### TC-SOC-01-06 双节点：节点 1 改模式，节点 2 1 秒内按新模式判定
- 关联 AC：AC-SOC-01-5
- 优先级：P0
- 层级：双节点 e2e
- 前置条件：双节点测试环境（两进程共享 SQLite，`IM_TEST_PEERS` 互指，广播开启）；alice 连在节点 2；alice、bob 非好友。
- 步骤：向节点 1 `PATCH {"mode":"friends"}`；1 秒内 alice 经节点 2 给 bob 发私聊。
- 预期结果：返回 4035。

### TC-SOC-01-07 双节点关闭广播：TTL 60 秒兜底
- 关联 AC：AC-SOC-01-5
- 优先级：P0
- 层级：双节点 e2e
- 前置条件：同上，但节点 2 设置 `IM_TEST_INVALIDATION=off`；节点 2 已缓存 `mode=open`（alice 先发一条成功）。
- 步骤：
  1. 向节点 1 PATCH `mode=friends`。
  2. 节点 2 测试时钟推进 59 秒，alice 经节点 2 发私聊。
  3. 节点 2 测试时钟再推进 2 秒（累计 61 秒），alice 再发。
- 预期结果：2 成功（仍按旧模式）；3 返回 4035。

### TC-SOC-01-08 改陌生人限额：两个在线连接收到 socialSettingsChanged，SDK 回调新值
- 关联 AC：AC-SOC-01-6
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice 用 sdk-ts 开两个实例（platform web、pc）均已连接，注册 `socialSettings` 回调。
- 步骤：`PATCH {"strangerMsgLimit":3}`。
- 预期结果：两个连接 1 秒内都收到 `event/socialSettingsChanged {mode:"open", strangerMsgLimit:3}`；两个实例的 `socialSettings` 回调都给出 `strangerMsgLimit=3`、`mode=open`。

### TC-SOC-01-09 离线期间改配置，重连后从同步应答头拿到新值
- 关联 AC：AC-SOC-01-6
- 优先级：P1
- 层级：SDK 集成
- 前置条件：alice 的 SDK 已同步一轮后断线。
- 步骤：PATCH `{"mode":"friends","strangerMsgLimit":5}`；alice 重连。
- 预期结果：重连后第一轮 `socialSync` 每一页的应答头 `mode=friends`、`strangerMsgLimit=5`；SDK `socialSettings` 回调给出新值（未收到事件也能更新）。

### TC-SOC-01-10 陌生人条数按累计口径：0 改为 3 后立即受限
- 关联 AC：AC-SOC-01-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式、`strangerMsgLimit=0`；alice、bob 非好友，会话无消息。
- 步骤：alice 给 bob 连发 5 条；PATCH `strangerMsgLimit=3`；alice 再发 1 条。
- 预期结果：前 5 条成功（seq 1～5）；第 6 条返回 4137，会话 maxSeq 仍为 5。

### TC-SOC-01-11 调低好友上限：保留已有，拦截新增，降到上限以下恢复
- 关联 AC：AC-SOC-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 与 f001～f150 互为好友；eve 给 alice 发过申请（pending），zed 也给 alice 发过申请。
- 步骤：
  1. PATCH `maxFriends=100`；alice 给 f001、f150 各发私聊；alice 同步好友列表。
  2. alice 同意 eve 的申请。
  3. alice 删除 f001～f051（51 个，剩 99）；alice 同意 eve 的申请；alice 同意 zed 的申请。
- 预期结果：1 私聊成功、好友 150 个；2 返回 4132，无数据变化；3 同意 eve 成功（好友 100），同意 zed 返回 4132。

### TC-SOC-01-12 启动配置只在首次写入，之后以数据库为准
- 关联 AC：AC-SOC-01-9
- 优先级：P0
- 层级：后端 e2e（子进程）
- 前置条件：全新数据库。
- 步骤：
  1. 以 `IM_SOCIAL_DEFAULTS={"mode":"friends"}` 启动，GET。
  2. PATCH `mode=open`；用同样的 `IM_SOCIAL_DEFAULTS` 重启；GET。
- 预期结果：1 返回 `mode=friends`；2 返回 `mode=open`。

---

## F-SOC-02 发送好友申请

### TC-SOC-02-01 发送申请：两端收到事件，同步后看到附言、来源、昵称
- 关联 AC：AC-SOC-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 一个连接；bob 两个连接（web、pc），记下 bob 的 ver 为 v。
- 步骤：alice `friendApply {to: bob, message: "我是 alice", source: "search"}`；bob 任一连接 `socialSync {sinceVer: v}`。
- 预期结果：ack `status=pending`、`changed=true`；bob 两个连接 1 秒内都收到 `socialChanged {ver: v+1}`；同步返回 1 条 requests：`userId=alice`、`direction=in`、`status=pending`、`message="我是 alice"`、`source=search`、`applyCount=1`、`profile.nickname="alice"`。

### TC-SOC-02-02 SDK：收到事件后自动同步，"新的朋友"出现申请
- 关联 AC：AC-SOC-02-1
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice、bob 各一个 sdk-ts 实例在线。
- 步骤：alice 调 `applyFriend("bob", {message: "我是 alice", source: "search"})`。
- 预期结果：返回 `'pending'`；bob 的 `friendRequests` 回调在 1 秒内给出一条 `in` 申请（附言、来源、昵称同上），`unseen=1`。

### TC-SOC-02-03 同一天重复申请合并，第 4 次 4292
- 关联 AC：AC-SOC-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟固定。
- 步骤：alice 给 bob 申请 3 次，附言依次 a、b、c，每次之间测试时钟推进 1 分钟；第 4 次申请（附言 d）；bob 全量同步。
- 预期结果：前 3 次 `pending`；第 4 次 4292（命中 `friendApplyDay:alice:bob`）；bob 的申请列表里 alice 只有一条，`message="c"`、`applyCount=3`、`expireAt = 第 3 次申请时间 + 7 天`。

### TC-SOC-02-04 被拒绝、过期后再申请复用同一对申请行
- 关联 AC：AC-SOC-02-2
- 优先级：P1
- 层级：后端 e2e
- 前置条件：alice 给 bob 申请，bob 拒绝。
- 步骤：
  1. 测试时钟推进 1 天，alice 再申请；bob 同步。
  2. 测试时钟推进 8 天（过期），alice 再申请；bob 同步。
- 预期结果：两次 bob 一侧都只有一条来自 alice 的 in 行，状态回到 `pending`、`applyCount` 依次为 2、3、`expireAt` 重新计算；数据库 `friend_requests` 中 (bob, alice, in) 与 (alice, bob, out) 各一行。

### TC-SOC-02-05 互相申请自动成为好友，双方 ver 各加 1
- 关联 AC：AC-SOC-02-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 已给 bob 发申请（未处理）；记 alice、bob 的 ver 为 a0、b0（bob 的 b0 已含收到申请那次）。
- 步骤：bob `friendApply {to: alice}`；双方增量同步。
- 预期结果：bob 的 ack `status=accepted`、`ver=b0+1`；alice 当前 ver = a0+1；双方好友列表互有对方；alice→bob 这条申请在 alice 的 out 行、bob 的 in 行都是 `accepted`；数据库中没有 (bob, alice, out) 的新申请行；alice 的全部连接收到 `socialChanged {ver: a0+1}`。

### TC-SOC-02-06 已互为好友时再申请：accepted、不改数据、无事件
- 关联 AC：AC-SOC-02-3
- 优先级：P1
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友；alice 有第二个连接；记双方 ver。
- 步骤：alice `friendApply {to: bob}`。
- 预期结果：ack `status=accepted`、`changed=false`、`items` 为空、`ver` 等于当前 ver；双方 ver 不变；alice 的第二个连接和 bob 2 秒内收不到任何帧（5.4 节第 3 行）。

### TC-SOC-02-07 防枚举：申请拉黑了我的人与正常申请的应答同构
- 关联 AC：AC-SOC-02-4
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：bob 拉黑 alice；carol 从未拉黑 alice；bob 在线，记 bob 的 ver。
- 步骤：alice `friendApply {to: bob, message: "hi"}` 得 ack1；alice `friendApply {to: carol, message: "hi"}` 得 ack2；bob 全量同步。
- 预期结果：ack1、ack2 去掉 `ver` 和时间戳字段（`createdAt`、`updatedAt`、`expireAt`）后，除 `userId` 外 JSON 逐字段相同：`status=pending`、`changed=true`、`items.requests` 各 1 条 out 行、`profile=null`；bob 的 ver 不变；bob 的连接 2 秒内收不到任何帧；bob 的申请列表里没有 alice。

### TC-SOC-02-08 格式错误：给自己、附言超长、to 不合法
- 关联 AC：AC-SOC-02-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：alice 依次：`{to: alice}`；`{to: bob, message: 51 个码点（含 emoji 计 1）}`；`{to: "a b"}`；`{to: bob, message: 50 个码点前后各加 3 个空格}`；`{to: bob, remark: 33 个码点}`；`{to: bob, foo: 1}`。
- 预期结果：依次 4000、4000、4004、成功（去空白后 50）、4000、4000；失败的请求不写任何行、不扣 `friendApplyDay` 配额。

### TC-SOC-02-09 申请不存在的用户：同构 pending，之后导入也看不到
- 关联 AC：AC-SOC-02-6
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：`nobody` 不存在；carol 存在。
- 步骤：alice 申请 `nobody` 得 ack1、申请 carol 得 ack2；alice 增量同步；业务方导入 `nobody`；`nobody` 连接并全量同步。
- 预期结果：ack1 与 ack2 按 TC-SOC-02-07 的方式比较相同；alice 同步得到发给 `nobody` 的 out 行（`status=pending`、`profile=null`）；`nobody` 的申请列表为空、ver=0。

### TC-SOC-02-10 好友已满：申请任何人 4132；对方已满时申请仍为待处理
- 关联 AC：AC-SOC-02-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`maxFriends=2000`；`seed` alice 有 2000 个好友；`seed` bob 有 2000 个好友；dave 有 0 个好友。
- 步骤：
  1. alice 申请 dave；alice 申请 `nobody`。
  2. dave 申请 bob；bob 同意 dave；bob、dave 同步。
  3. 重置后 `seed` 一条 alice→dave 的待处理申请（alice 仍有 2000 个好友）；dave 同意 alice。
- 预期结果：
  1. 两次都返回 4132（判定表第 5 行在"对方是否存在"之前，两次应答除 `id` 外相同）。
  2. dave 得 `pending`；bob 同意返回**按 F-SOC-03 规则 2 为 4132**（bob 自己已满），申请仍为 `pending`、双方好友数不变。⚠ AC 原文此处写 4133，与规则 2 矛盾，已列入 PRD 问题清单 P-2，产品确认前按规则 2 断言。
  3. dave 同意返回 4133（对方 alice 已满），不改数据，申请仍为 `pending`。

### TC-SOC-02-11 来源为群：服务端校验，申请人一侧原样记录
- 关联 AC：AC-SOC-02-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob；g2 成员 alice、carol 之外的人（carol 不在 g2）。
- 步骤：alice `friendApply {to: bob, source: "group", sourceRef: "g1"}`；alice `friendApply {to: carol, source: "group", sourceRef: "g1"}`；alice、bob、carol 同步。另：`{source: "group"}` 不带 `sourceRef`。
- 预期结果：bob 一侧 in 行 `source=group`；carol 一侧 in 行 `source=other`，申请照常 `pending`；alice 的两条 out 行都是 `source=group`；不带 `sourceRef` 返回 4000。

### TC-SOC-02-12 单向删除后的两种再申请
- 关联 AC：AC-SOC-02-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友。
- 步骤：
  1. bob 删除 alice；alice 申请 bob；bob 同步。
  2. 重置数据：互为好友后 alice 删除 bob；记 bob 的 ver；alice 申请 bob；alice 同步。
- 预期结果：1 alice 得 `pending`，bob 收到 `socialChanged`，申请列表出现 alice；2 alice 得 `accepted`，alice 好友列表恢复 bob（备注为空）；bob 的 ver 不变，bob 的连接 2 秒内收不到任何帧。

### TC-SOC-02-13 申请限流 4290 与每日配额 4292
- 关联 AC：AC-SOC-02-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟固定；u01～u40 已导入。
- 步骤：
  1. 测试时钟不动，alice 连发 6 个申请（u01～u06）。
  2. 之后每次测试时钟推进 1 秒、申请一个新用户（u07 起），直到成功累计 30 个；再申请 1 个新用户。
- 预期结果：1 前 5 个 `pending`，第 6 个（u06）4290；2 u06 重新申请成功并计入；成功累计到第 30 个后，第 31 个返回 4292；被 4290 拒绝的那次不计入 30。

### TC-SOC-02-14 日志不出现附言和预设备注原文
- 关联 AC：AC-SOC-02-11
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：测试进程捕获全部服务端日志（含 debug 级别）。
- 步骤：alice `friendApply {to: bob, message: "ZXQ-SOCIAL-123", remark: "ZXQ-REMARK-456"}`；bob 同意；alice 再发一个附言超长的申请（触发 4000）。
- 预期结果：日志中搜不到 `ZXQ-SOCIAL-123`、`ZXQ-REMARK-456`；4000 错误应答的 `msg` 中也不含用户输入。

### TC-SOC-02-15 申请自己拉黑的人返回 4000，取消拉黑后可以申请
- 关联 AC：AC-SOC-02-12
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 拉黑 bob。
- 步骤：alice 申请 bob；alice 取消拉黑 bob；alice 再申请 bob。
- 预期结果：第 1 次 4000（不写任何行）；第 2 次 `pending`，bob 收到申请。

### TC-SOC-02-16 互相申请并发：100 组都成为好友、无死锁（B6 前置）
- 关联 AC：AC-SOC-02-13
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：`IM_TEST_PG_DSN` 指向空库（未设置时在 SQLite 上运行）；100 组新用户 (x_i, y_i)。
- 步骤：每组用两个连接同时发 `x→y`、`y→x` 申请。
- 预期结果：每组结束后 x、y 互为好友（各一条边）；双方之间没有 `pending` 的申请行；无死锁、无 5000；`store_deadlock_total=0`。

### TC-SOC-02-17 判定表 9 行逐行（存储契约）
- 关联 AC：AC-SOC-02-3、AC-SOC-02-4、AC-SOC-02-6、AC-SOC-02-7、AC-SOC-02-9
- 优先级：P0
- 层级：存储契约
- 前置条件：`FriendApplyTable`，SQLite / Postgres 各跑一遍。
- 步骤：对 F-SOC-02 规则 2 第 3～9 行各构造一个状态后调用 `ApplyFriend`；第 8 行再分"对方好友已满"和"未满"两种。
- 预期结果：每行结果与判定表一致：第 3 行 4000；第 4 行 `accepted`、无写入；第 5 行 4132；第 6 行只写申请人的 out 行（对方无 `user_social` 行被创建）；第 7 行恢复边、对方无写入；第 8 行 4133 无写入 / 双向边 + 全部申请行 `accepted`；第 9 行 out + in 两行。

---

## F-SOC-03 处理好友申请

### TC-SOC-03-01 同意申请：多端事件、备注、双方好友列表
- 关联 AC：AC-SOC-03-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发了申请；alice 两个连接在线；bob 两个连接在线（b1、b2）。
- 步骤：b1 `friendRespond {from: alice, action: accept, remark: "同学A"}`；双方同步。
- 预期结果：ack `changed=true`；alice 两个连接、b2 在 1 秒内收到 `socialChanged`，b1 不收（从 ack 拿结果）；bob 的好友列表中 alice 的 `remark="同学A"`；alice 的好友列表中有 bob。

### TC-SOC-03-02 SDK：同意后两端好友列表更新
- 关联 AC：AC-SOC-03-1
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice、bob 各一个 sdk-ts 实例；alice 已申请 bob。
- 步骤：bob 调 `respondFriend("alice", "accept", {remark: "同学A"})`。
- 预期结果：bob 的 `friends` 回调立即（ack 应用）出现 alice（备注"同学A"）；alice 的 `friends` 回调 1 秒内出现 bob；bob 的 `unseen` 变为 0。

### TC-SOC-03-03 拉黑对方时仍可同意，黑名单不变
- 关联 AC：AC-SOC-03-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：alice 给 bob 发了申请，之后 bob 收到申请前未拉黑；bob 再拉黑 alice（in 行变为 `rejected`）。
- 步骤：bob 同意 alice（未过期）；bob 同步。
- 预期结果：同意成功；双方互为好友；bob 的黑名单里仍有 alice。

### TC-SOC-03-04 申请时的预设备注在同意后生效
- 关联 AC：AC-SOC-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：alice `friendApply {to: bob, remark: "B哥"}`；bob 同意（不带 remark）；双方同步。
- 预期结果：alice 一侧 bob 的 `remark="B哥"`；bob 一侧 alice 的 `remark=""`。

### TC-SOC-03-05 拒绝：申请人仍显示等待验证且无感知
- 关联 AC：AC-SOC-03-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发了申请；记 alice 的 ver 与 out 行 `expireAt`。
- 步骤：bob `friendRespond {from: alice, action: reject}`；alice 增量同步。
- 预期结果：bob 一侧 in 行 `rejected`；alice 的 ver 不变，增量同步 0 项；alice 一侧 out 行仍为 `pending`、`expireAt` 不变；alice 的连接 2 秒内收不到任何帧。

### TC-SOC-03-06 过期边界：7 天减 1 秒可同意，7 天加 1 秒 4045
- 关联 AC：AC-SOC-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟；T0 时 alice 申请 bob，carol 申请 bob。
- 步骤：时钟设为 T0+7 天−1 秒，bob 同意 alice；时钟设为 T0+7 天+1 秒，bob 同意 carol；bob、carol 全量同步。
- 预期结果：同意 alice 成功；同意 carol 返回 4045；bob 一侧 carol 的申请、carol 一侧发给 bob 的申请都显示 `status=expired`。

### TC-SOC-03-07 拒绝后改同意；已同意再拒绝 4091
- 关联 AC：AC-SOC-03-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发了申请。
- 步骤：bob 拒绝；bob 再拒绝；bob 同意；bob 再拒绝。
- 预期结果：第 1 次 `changed=true`；第 2 次 `changed=false`；第 3 次成功成为好友；第 4 次 4091，好友关系不变。

### TC-SOC-03-08 防枚举：不存在、已过期、申请时被拉黑的 4045 响应体相同
- 关联 AC：AC-SOC-03-6
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：dave 的申请已过期（测试时钟）；carol 从未申请；bob 先拉黑 eve，eve 再申请 bob（bob 无 in 行）。
- 步骤：bob 依次同意 carol、dave、eve、`nobody`。
- 预期结果：4 次都返回 4045；4 个错误应答去掉 `id` 后逐字节相同。

### TC-SOC-03-09 重复同意幂等，ver 只加 1
- 关联 AC：AC-SOC-03-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 给 bob 发了申请；记 alice、bob 的 ver。
- 步骤：bob 同意 alice 两次。
- 预期结果：第一次 `changed=true`、`ver=b+1`；第二次 `changed=false`、`ver=b+1`、`items` 为空；数据库 `friends` 中 (alice, bob)、(bob, alice) 各一条；alice 的 ver 只 +1，bob 的 ver 只 +1。

### TC-SOC-03-10 好友上限并发：每轮恰好一个成功（B6 前置）
- 关联 AC：AC-SOC-03-8
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：`maxFriends=2000`；`seed` bob 有 1999 个好友；carol、dave 各给 bob 发了申请。
- 步骤：bob 的两个连接同时分别同意 carol、dave；重复 50 轮，每轮重置数据。
- 预期结果：每轮恰好一个成功、一个 4132；bob 的 `friend_count` 与 `friends` 非墓碑行数都为 2000，从不超过。

### TC-SOC-03-11 互相同意并发：100 轮无死锁（B6 前置）
- 关联 AC：AC-SOC-03-9
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：每轮 `seed` alice→bob、bob→alice 两条待处理申请。
- 步骤：alice、bob 各用一个连接同时同意对方，重复 100 轮。
- 预期结果：每轮结束互为好友、各一条边、4 行申请都 `accepted`；无死锁、无 5000；100 轮总耗时 ≤ 10 秒。

---

## F-SOC-04 "新的朋友"列表与红点

### TC-SOC-04-01 SDK 红点数与列表排序
- 关联 AC：AC-SOC-04-1
- 优先级：P0
- 层级：SDK 集成
- 前置条件：bob 的 sdk-ts 在线，从未标记已读。
- 步骤：carol 申请 bob；测试时钟推进 1 秒；dave 申请 bob。
- 预期结果：bob 的 `friendRequests` 回调 `unseen=2`，`items` 中 dave 在 carol 之前（`updatedAt` 倒序）。

### TC-SOC-04-02 一端标记已读，另一端 2 秒内红点清零
- 关联 AC：AC-SOC-04-2
- 优先级：P0
- 层级：SDK 集成
- 前置条件：bob 的 web、pc 两个 SDK 实例在线，红点都为 2。
- 步骤：web 实例调 `markFriendRequestsSeen()`。
- 预期结果：web 实例 `unseen` 立即为 0；pc 实例 2 秒内收到 `socialChanged`、同步后 `unseen=0`。

### TC-SOC-04-03 标记已读后对方再次申请（合并）重新计入红点
- 关联 AC：AC-SOC-04-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：carol、dave 已申请 bob；bob `friendRequestsSeen {ver: 当前 ver}`。
- 步骤：carol 再次申请 bob；bob 同步，按规则计算红点。
- 预期结果：carol 的 in 行 ver > `requestSeenVer`，dave 的不大于；红点数 = 1。

### TC-SOC-04-04 requestSeenVer 只进不退，截断到当前 ver
- 关联 AC：AC-SOC-04-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` 使 bob 当前 ver = 7、`requestSeenVer=0`。
- 步骤：`friendRequestsSeen {ver:5}`；`{ver:3}`；`{ver:100}`。
- 预期结果：第 1 次 `requestSeenVer=5`、`ver=8`、`changed=true`；第 2 次 `requestSeenVer=5`、`ver=8`、`changed=false`；第 3 次 `requestSeenVer=8`、`ver=9`。另：参数 `ver` 为 -1 或非整数返回 4000。

### TC-SOC-04-05 已过期申请显示"已过期"、无"同意"按钮、不计红点
- 关联 AC：AC-SOC-04-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：carol 的申请已过期（测试时钟推进 8 天），dave 的申请未过期；bob 未标记已读。
- 步骤：bob 打开"新的朋友"。
- 预期结果：carol 一行显示"已过期"、没有"同意"按钮；dave 一行有"同意"按钮；入口红点为 1。

### TC-SOC-04-06 SDK：过期判断用服务端时间，不用本地时钟
- 关联 AC：AC-SOC-04-5
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport；auth ack `serverTime` = T；本地时钟 = T − 2 天。
- 步骤：注入一条 `expireAt = T − 1 秒` 的 pending in 申请。
- 预期结果：SDK 判定为 `expired`，`unseen=0`；本地时钟改为 T + 10 天但服务端偏移不变时，30 天丢弃按服务端时间计算。

### TC-SOC-04-07 30 天查询过滤（不执行清理）
- 关联 AC：AC-SOC-04-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` 两条发给 bob 的申请：updatedAt 分别为 now−(29 天 23 小时)、now−(30 天 + 1 秒)；不调用 purge。
- 步骤：bob `socialSync {sinceVer: 0}`。
- 预期结果：前者出现在 requests 中（`status=expired`）；后者不出现；数据库中两行都还在。

### TC-SOC-04-08 清理任务删除超过 30 天的申请并推进 purgedVer
- 关联 AC：AC-SOC-04-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同上，后者的 ver 记为 r。
- 步骤：`POST /admin/test/social/purge`；查数据库；bob `socialSync {sinceVer: r−1}`。
- 预期结果：该行被物理删除；bob 的 `purgedVer ≥ r`；同步返回 `full=true`。

---

## F-SOC-05 删除好友与备注名

### TC-SOC-05-01 单向删除：对方列表不变、无感知
- 关联 AC：AC-SOC-05-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友，alice 给 bob 设了备注"老板"；bob 在线，记 bob 的 ver。
- 步骤：alice `friendDelete {userId: bob}`；alice、bob 同步。
- 预期结果：ack `changed=true`，`items.friends` 中 bob 的墓碑 `deleted=true`、`remark=""`；alice 的好友列表没有 bob；bob 的好友列表仍有 alice；bob 的 ver 不变、2 秒内收不到任何帧。

### TC-SOC-05-02 SDK：删除后本端立即移除，另一端同步移除
- 关联 AC：AC-SOC-05-1
- 优先级：P1
- 层级：SDK 集成
- 前置条件：alice 两个 SDK 实例；alice、bob 互为好友。
- 步骤：实例 1 调 `deleteFriend("bob")`。
- 预期结果：实例 1 的 `friends` 回调立即不含 bob（应用 ack items）；实例 2 2 秒内不含 bob；`relationOf("bob")` 两端都为 `none`。

### TC-SOC-05-03 好友制下删除后双方都不能私聊，历史可拉
- 关联 AC：AC-SOC-05-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`；alice、bob 互为好友并互发 3 条。
- 步骤：alice 删除 bob；alice 给 bob 发；bob 给 alice 发；双方 `sync` 该会话、`convs`。
- 预期结果：两次发送都 4035、不保存；双方 `sync` 都拉到 3 条；双方 `convs` 仍有该会话；申请行保持 `accepted`。

### TC-SOC-05-04 删除不是好友的人、不存在的人：幂等成功
- 关联 AC：AC-SOC-05-3
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：carol 不是 alice 的好友；记 alice 的 ver。
- 步骤：alice 删除 carol 得 ack1；删除 `nobody` 得 ack2。
- 预期结果：两者都 `changed=false`、`items` 为空、`ver` 为当前 ver；alice 的 ver 不变；ack1、ack2 除 `id` 外相同。

### TC-SOC-05-05 备注多端同步，且在对方和管理接口里都看不到
- 关联 AC：AC-SOC-05-4
- 优先级：P0
- 层级：SDK 集成 + 后端 e2e
- 前置条件：alice（web、pc 两个 SDK 实例）、bob 互为好友，有单聊消息。
- 步骤：web 实例 `setFriendRemark("bob", "老板")`；bob 依次调 `socialSync {sinceVer:0}`、`users [alice]`、`userSearch alice`、`convs`；调 `GET /admin/users/alice/friends`。
- 预期结果：pc 实例 2 秒内 `friends` 中 bob 的备注为"老板"；bob 的 4 个应答和管理接口应答的 JSON 中都不含"老板"（字符串搜索），管理接口每项没有 `remark` 字段。

### TC-SOC-05-06 备注边界、非好友 4040 同构、空白清除
- 关联 AC：AC-SOC-05-5
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：alice、bob 互为好友；carol 不是 alice 的好友。
- 步骤：alice 依次 `friendRemark` bob 为 33 个码点；bob 为 32 个码点；carol 为"x"；`nobody` 为"x"；bob 为"  "（两个空格）。
- 预期结果：依次 4000、成功、4040、4040、成功且 bob 的备注变为 `""`；两个 4040 应答除 `id` 外相同。

### TC-SOC-05-07 备注值未变：changed=false、ver 不变
- 关联 AC：AC-SOC-05-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友；alice 有第二个连接。
- 步骤：alice 两次把 bob 的备注设为"老板"。
- 预期结果：第一次 `changed=true`、ver+1，第二个连接收到 `socialChanged`；第二次 `changed=false`、ver 不变，第二个连接 2 秒内收不到任何帧。

---

## F-SOC-06 关系同步

### TC-SOC-06-01 离线补拉：一页 4 项、3 个 ver
- 关联 AC：AC-SOC-06-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 已给 alice 发申请；bob 全部断开，记 bob 的 ver 为 v0。
- 步骤：carol 申请 bob；dave 申请 bob；alice 同意 bob；bob 上线 `socialSync {sinceVer: v0}`。
- 预期结果：`full=false`、`untilVer=v0+3`、`latestVer=v0+3`、`hasMore=false`，共 4 项，按排序键 `(ver, 类型, 对方, 方向)` 依次为：carol 的 in 申请（ver v0+1）、dave 的 in 申请（v0+2）、alice 的好友项（v0+3，带资料）、发给 alice 的 out 申请（`status=accepted`、带资料，v0+3）。

### TC-SOC-06-02 多端：pc 端同意，web 端 2 秒内出现好友
- 关联 AC：AC-SOC-06-2
- 优先级：P0
- 层级：SDK 集成
- 前置条件：bob 已申请 alice；alice 的 pc、web 两个 SDK 实例在线。
- 步骤：pc 实例 `respondFriend("bob", "accept")`。
- 预期结果：web 实例 `friends` 回调 2 秒内含 bob。

### TC-SOC-06-03 断网期间的变化重连后 5 秒内补齐
- 关联 AC：AC-SOC-06-2
- 优先级：P0
- 层级：SDK 集成
- 前置条件：bob、carol 已申请 alice；alice 的 web 实例断网（传输层断开，不退出登录），pc 实例在线。
- 步骤：pc 实例同意 bob 和 carol；web 实例恢复网络。
- 预期结果：web 实例重连后先 `convs` 再 `socialSync {sinceVer: 断网前 ver}`（增量）；5 秒内 `friends` 含 bob、carol，`unseen` 与 pc 一致。

### TC-SOC-06-04 事件被丢弃：重连后的同步轮包含变化
- 关联 AC：AC-SOC-06-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 已申请 bob；alice 在线；`POST /admin/test/social/drop-events {userId: alice, type: socialChanged, count: 100}`。
- 步骤：bob 同意 alice；等 2 秒；alice 断线重连并从旧 ver 同步。
- 预期结果：alice 2 秒内没有收到 `socialChanged`；重连后的同步轮包含 bob 的好友项和 `accepted` 的 out 申请。

### TC-SOC-06-05 事件被丢弃：下一次事件触发的同步轮一并补齐
- 关联 AC：AC-SOC-06-3
- 优先级：P0
- 层级：SDK 集成
- 前置条件：同上，但 `count: 1`；alice 使用 sdk-ts，不重连。
- 步骤：bob 同意 alice（事件被丢弃）；carol 申请 alice（事件正常）。
- 预期结果：alice 的 SDK 收到 carol 申请引起的 `socialChanged` 后开始一轮，`friends` 含 bob、`friendRequests` 含 carol 的 in 申请。

### TC-SOC-06-06 同一个 ver 跨页：不丢不重
- 关联 AC：AC-SOC-06-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：记 alice 的 ver 为 v。
- 步骤：`POST /admin/friends/import {pairs: [alice-bob, alice-carol, alice-dave]}`；alice `socialSync {sinceVer: v, limit: 2}`；用返回的 `cursor` 拉下一页。
- 预期结果：alice 的 ver = v+1；第一页 2 项、`hasMore=true`；第二页 1 项、`hasMore=false`；3 项 userId 互不相同，ver 都是 v+1 = `untilVer`。

### TC-SOC-06-07 全量分页：2500 个好友 5 页
- 关联 AC：AC-SOC-06-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 有 2500 个好友（`maxFriends=5000`）。
- 步骤：`socialSync {sinceVer: 0, limit: 500}` 并用 cursor 翻到底。
- 预期结果：共 5 页，每页 `full=true`、`untilVer` 相同，前 4 页 `hasMore=true`、每页 500 项，第 5 页 `hasMore=false`；合计 2500 项无重复。

### TC-SOC-06-08 同步轮进行中删除、新增：进入下一轮
- 关联 AC：AC-SOC-06-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 有 1200 个好友；x 是排在第 3 页的好友；y 已申请 alice。
- 步骤：全量轮拉完第 1 页（`limit=500`）；alice 删除 x、同意 y；继续拉完第 2、3 页；再从 `untilVer` 增量同步。
- 预期结果：第 2、3 页没有 x 和 y（x 的旧版本 ver ≤ untilVer 但已变为墓碑，全量轮不返回墓碑）；最后一页 `latestVer = untilVer + 2`；下一轮增量返回 x 的墓碑和 y 的好友项（及 y 的 in 申请 `accepted`）；按客户端规则应用后共 1200 个好友，不含 x、含 y。

### TC-SOC-06-09 同步轮进行中删除第 1 页已返回的好友
- 关联 AC：AC-SOC-06-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同上，x 改为第 1 页已返回的好友。
- 步骤：同上。
- 预期结果：本轮结束后客户端暂时含 x；下一轮增量返回 x 的墓碑，应用后不含 x、含 y，共 1200 个。

### TC-SOC-06-10 墓碑、清理与全量判定
- 关联 AC：AC-SOC-06-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友。
- 步骤：
  1. alice 删除 bob，ack 的 ver 记为 v；`socialSync {sinceVer: v−1}`。
  2. 测试时钟推进 31 天；`social/purge`；再次 `socialSync {sinceVer: v−1}`。
  3. `socialSync {sinceVer: 当前 ver}`。
- 预期结果：1 返回 bob 的墓碑（`deleted=true`、ver=v）；2 alice 的 `purgedVer ≥ v`，返回 `full=true`、结果中没有 bob；3 `full=false`、0 项、`hasMore=false`。

### TC-SOC-06-11 清理发生在增量轮中途：restart
- 关联 AC：AC-SOC-06-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 在 31 天前删除了 b1（墓碑 ver=s+1），之后有 3 个新好友（ver s+2～s+4）。
- 步骤：`socialSync {sinceVer: s, limit: 1}` 拉第 1 页；`social/purge`；用 cursor 拉第 2 页；再 `socialSync {sinceVer: s}`。
- 预期结果：第 2 页 `restart=true`、friends / requests / blocks 都为空；重新开始返回 `full=true`。

### TC-SOC-06-12 SDK：restart、purgedVer 全量与 30 天丢弃（共享用例）
- 关联 AC：AC-SOC-06-8、AC-SOC-06-7
- 优先级：P0
- 层级：SDK 单测（vectors/social.json）
- 前置条件：`protocol/vectors/social.json` 中 `restart`、`purgedFull`、`drop30Days` 三组用例。
- 步骤：sdk-ts、Go 参考客户端（后续 Kotlin、Swift）依次喂入本地状态和应答页序列。
- 预期结果：收到 `restart=true` 后用同一个 `sinceVer` 重发第一页、丢弃已应用的本轮页；全量轮收齐后整体替换；超过 30 天的申请从本地删除；最终状态、`socialVer` 与用例期望逐项相同。

### TC-SOC-06-13 并发同意 10 人：ver 恰好 +10、20 项分布在 10 个 ver
- 关联 AC：AC-SOC-06-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u1～u10 都已申请 alice；记 alice 的 ver 为 v。
- 步骤：alice 的两个连接并发各同意 5 人；`socialSync {sinceVer: v}` 翻完。
- 预期结果：alice 的 ver = v+10；共 20 项，ver 取值恰好为 v+1～v+10，每个 ver 恰好 2 项（好友项 + in 申请项）。

### TC-SOC-06-14 版本号每操作一个（5.4 节逐行，存储契约）
- 关联 AC：AC-SOC-06-9、AC-SOC-03-7
- 优先级：P0
- 层级：存储契约
- 前置条件：`SocialVerPerOp`、`SocialVerConcurrent`。
- 步骤：对 5.4 节 19 行逐行执行操作并检查 A、B 的 ver、改动行的 ver、事件接收者；另 8 个并发事务 × 1000 次修改同一用户。
- 预期结果：每行与表一致（无变化的操作不分配 ver）；并发后 ver 从起点连续递增 8000、无重复无空洞。

### TC-SOC-06-15 参数校验
- 关联 AC：AC-SOC-06-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 当前 ver = 5；bob 有一个合法游标。
- 步骤：`sinceVer:-1`；`sinceVer:1.5`；`sinceVer:"1"`；同时传 `sinceVer`、`cursor`；都不传；`limit:0`；`limit:501`；改动游标中的 1 个字节；alice 使用 bob 的游标；`sinceVer:6`。
- 预期结果：前 9 个都返回 4000；`sinceVer:6`（大于当前 ver）返回 `full=true`、`untilVer=5`。

### TC-SOC-06-16 修改类命令的 ack
- 关联 AC：AC-SOC-06-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 已申请 bob；记 bob 的 ver 为 b。
- 步骤：bob 同意 alice；再同意一次。
- 预期结果：第一次 `changed=true`、`ver=b+1`，`items.friends` 含 alice 的好友项、`items.requests` 含 alice 的 in 申请项，二者 ver 都为 b+1，`items.blocks` 为空数组；第二次 `changed=false`、`ver=b+1`、三个数组都为空。

### TC-SOC-06-17 SDK：ack 的 ver 不连续时先应用 items 再补一轮
- 关联 AC：AC-SOC-06-11
- 优先级：P0
- 层级：SDK 单测（vectors/social.json）
- 前置条件：本地 `socialVer=10`。
- 步骤：自己发起删除，ack `changed=true, ver=12, items={friends:[x 墓碑 ver 12]}`；接着模拟 `socialSync {sinceVer:10}` 返回 ver 11 的 y 好友项和 ver 12 的 x 墓碑。
- 预期结果：先移除 x，再发起 `sinceVer:10` 的一轮；同一键保留 ver 较大的一项；最终含 y、不含 x、`socialVer=12`。`changed=false` 的 ack 不改变本地状态。

### TC-SOC-06-18 全量同步性能
- 关联 AC：AC-SOC-06-12
- 优先级：P1
- 层级：压测
- 前置条件：基准机器（C-48）；`seed` alice 5000 个好友 + 1000 个黑名单 + 200 条申请（`maxFriends=5000`）。
- 步骤：`im-loadtest -scenario social-full`，全量轮 `limit=500`，重复 3 次。
- 预期结果：3 次的中位数：单页耗时 P99 ≤ 100ms、整轮 ≤ 1 秒；报告写入 `docs/reports/perf-social-*.md`。

### TC-SOC-06-19 全量轮限流：1 分钟内第 11 轮 4290，增量不受限
- 关联 AC：AC-SOC-06-13
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟；`socialFull` 为每分钟 10 轮、突发 5。
- 步骤：
  1. t=0 连续开始 5 个全量轮（`sinceVer:0`，只拉第一页），其中第 3 个用大于当前 ver 的 `sinceVer`（服务端判定为全量）。
  2. 之后每推进 6 秒开始 1 个全量轮，到 t=30s 累计 10 轮；t=31s 开始第 11 轮。
  3. 同一时刻连续开始 20 个增量轮（`sinceVer` = 当前 ver）。
- 预期结果：前 10 轮成功（服务端判定的全量也计数）；第 11 轮 4290；20 个增量轮全部成功；被 4290 拒绝的那轮不扣令牌。

### TC-SOC-06-20 H5 刷新后 2000 好友 2 秒内显示
- 关联 AC：AC-SOC-06-14
- 优先级：P1
- 层级：UI 自动化（性能）
- 前置条件：开发机；`seed` alice 有 2000 个好友；alice 已登录 H5。
- 步骤：刷新页面并打开通讯录，记录从刷新到好友列表渲染完成的时间；重复 3 次。
- 预期结果：中位数 ≤ 2 秒；SDK 侧全量轮 4 页、只触发一次 `friends` 回调。

### TC-SOC-06-21 同步模型测试 1000 个随机序列
- 关联 AC：AC-SOC-06-15
- 优先级：P0
- 层级：模型测试
- 前置条件：`server/e2e/socialmodel`；两个用户各两个连接（Go 参考客户端）。
- 步骤：生成 1000 个随机操作序列（申请 / 同意 / 拒绝 / 删除 / 备注 / 拉黑 / 取消拉黑 / 隐私 / 标记已读 / 管理接口导入删除，随机断线重连、`drop-events`、推进测试时钟并 purge、`limit` 1～500 随机）；每个序列结束后等待事件静止。
- 预期结果：每个参考客户端本地好友 / 申请 / 黑名单 / `requestSeenVer` / 设置与服务端 `sinceVer:0` 全量逐项相同，本地 ver = 服务端当前 ver；失败时输出种子与最小复现序列。

---

## F-SOC-07 拉黑与黑名单

### TC-SOC-07-01 被拉黑后私聊 4036，对方收不到、不保存
- 关联 AC：AC-SOC-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式；alice 两个连接在线；alice `block {userId: bob}`。
- 步骤：bob 给 alice 发私聊；alice `sync` 该会话。
- 预期结果：bob 收到 4036；alice 的两个连接 2 秒内收不到 msg；`sync` 中没有这条；会话 maxSeq 不变。

### TC-SOC-07-02 SDK：4036 标记失败、提示"被对方拒收"、不自动重发
- 关联 AC：AC-SOC-07-1
- 优先级：P0
- 层级：SDK 集成
- 前置条件：同上，bob 使用 sdk-ts。
- 步骤：bob `send` 给 alice；等待 15 秒；断线重连一次。
- 预期结果：消息状态为失败、错误码 4036；`messageFor("send", 4036)` 为"消息已发出，但被对方拒收了"；15 秒内和重连后都没有自动重发（服务端只收到 1 次该 clientMsgId）。

### TC-SOC-07-03 拉黑者自己给对方发私聊照常成功
- 关联 AC：AC-SOC-07-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式；alice 拉黑 bob。
- 步骤：alice 给 bob 发私聊。
- 预期结果：成功；bob 收到。

### TC-SOC-07-04 群里拉黑不生效
- 关联 AC：AC-SOC-07-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob；alice 拉黑 bob。
- 步骤：bob 在 g1 发消息。
- 预期结果：发送成功；alice 收到该群消息。

### TC-SOC-07-05 拉黑把对方的待处理申请置为 rejected
- 关联 AC：AC-SOC-07-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 已申请 alice（pending）、carol 已申请 alice；记 bob 的 ver。
- 步骤：alice 拉黑 bob；alice 同步并计算红点；bob 同步。
- 预期结果：alice 一侧 bob 的申请 `rejected`、ver 与黑名单行相同（同一个 ver）；红点从 2 变为 1；bob 一侧 out 行仍 `pending`；bob 的 ver 不变。

### TC-SOC-07-06 取消拉黑不恢复申请
- 关联 AC：AC-SOC-07-4
- 优先级：P1
- 层级：后端 e2e
- 前置条件：接上一用例。
- 步骤：alice 取消拉黑 bob；alice 同步。
- 预期结果：黑名单项为墓碑；bob 的 in 申请仍为 `rejected`（未过期前 alice 仍可同意）。

### TC-SOC-07-07 黑名单多端同步；取消后对方可以私聊
- 关联 AC：AC-SOC-07-5
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice 的 web、pc 两个 SDK 实例。
- 步骤：web 实例 `block("bob")`；等 2 秒；web 实例 `unblock("bob")`；bob 给 alice 发私聊。
- 预期结果：pc 实例 2 秒内 `blocks` 含 bob；取消后两端 2 秒内都不含 bob；bob 的私聊成功。

### TC-SOC-07-08 拉黑边界：自己、黑名单满、取消不在名单的人
- 关联 AC：AC-SOC-07-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`maxBlocks=1000`；`seed` alice 已拉黑 1000 人。
- 步骤：alice 拉黑自己；alice 拉黑第 1001 人；alice 取消拉黑一个不在名单里的 carol；alice 重复拉黑名单里已有的 b0001。
- 预期结果：依次 4000、4134、成功 `changed=false` 且 ver 不变、成功 `changed=false`。

### TC-SOC-07-09 拉黑不改变好友关系
- 关联 AC：AC-SOC-07-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 互为好友。
- 步骤：alice 拉黑 bob；alice 全量同步。
- 预期结果：`friends` 含 bob（非墓碑），`blocks` 含 bob；bob 的好友列表仍含 alice。

### TC-SOC-07-10 拉黑静默；对方的搜索和资料结果不变
- 关联 AC：AC-SOC-07-8
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：alice、bob 有单聊消息；bob 在线；拉黑前 bob 调 `userSearch alice`、`users [alice]` 记下应答。
- 步骤：alice 拉黑 bob；bob 再调同样两个命令。
- 预期结果：bob 的 ver 不变；bob 的连接 2 秒内收不到任何帧；两个应答去掉 `id` 后与拉黑前逐字节相同。

### TC-SOC-07-11 防枚举：拉黑不存在的人与拉黑无关系、不可被搜索的人同构
- 关联 AC：AC-SOC-07-9
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：eve 关闭"可被搜索"，与 alice 无任何关系。
- 步骤：alice 拉黑 `nobody` 得 ack1；alice 拉黑 eve 得 ack2；alice 同步。
- 预期结果：ack1、ack2 去掉 ver 和时间戳后除 `userId` 外相同（`changed=true`、各 1 条 blocks）；两个黑名单项 `profile=null`；`nobody` 计入 `block_count`。

### TC-SOC-07-12 拉黑每日配额 4292
- 关联 AC：AC-SOC-07-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`blockDailyLimit=100`；测试时钟。
- 步骤：alice 交替拉黑 / 取消拉黑（u01～u50），每次推进 1 秒，共 100 次；第 101 次拉黑 u51。
- 预期结果：前 100 次成功；第 101 次 4292；被拒绝的这次不扣令牌（推进 15 分钟后可以再拉黑 1 次）。

### TC-SOC-07-13 拉黑 ack 之后立即发送一定被拒（100 轮）
- 关联 AC：AC-SOC-07-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式。
- 步骤：每轮：alice 的连接 1 `block bob`，收到 ack 后 bob 立即发私聊；之后 alice `unblock` 并 `limits/reset`；重复 100 轮。
- 预期结果：100 次发送都返回 4036；alice 收到的消息为 0；会话 maxSeq 为 0。

---

## F-SOC-08 私聊权限

### TC-SOC-08-01 好友制：互为好友可发，单向边不可发
- 关联 AC：AC-SOC-08-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`；alice、bob 互为好友；alice、carol 互为好友后 carol 删除了 alice（alice 仍有 carol 的边）。
- 步骤：alice 给 bob 发；alice 给 carol 发；carol `sync` 该会话。
- 预期结果：给 bob 成功；给 carol 4035，消息不保存（carol 拉不到）。

### TC-SOC-08-02 开放模式陌生人限额：第 4 条 4137，对方回复后解除
- 关联 AC：AC-SOC-08-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式、`strangerMsgLimit=3`；alice、bob 非好友，无会话。
- 步骤：alice 给 bob 连发 4 条；bob 回复 1 条；alice 再连发 10 条（每 1 秒 1 条，避开发送限流）。
- 预期结果：前 3 条成功（seq 1～3），第 4 条 4137 且不占 seq；bob 回复 seq=4；之后 10 条都成功（seq 5～14）。

### TC-SOC-08-03 回复解除是永久的
- 关联 AC：AC-SOC-08-2
- 优先级：P1
- 层级：后端 e2e
- 前置条件：接上一用例；之后 alice、bob 曾成为好友又被 alice 删除（重新变回陌生人）。
- 步骤：alice 再给 bob 发 5 条。
- 预期结果：全部成功（bob 的 `sent_count > 0`）。

### TC-SOC-08-04 默认配置：陌生人 10 条、150 个新对象都成功
- 关联 AC：AC-SOC-08-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：默认配置；u001～u150 已导入、与 alice 无会话；测试时钟可推进。
- 步骤：alice 给陌生人 bob 连发 10 条；alice 在 1 小时内（测试时钟）给 u001～u150 各发 1 条；查 `newPeerReject` 剩余令牌（测试接口）。
- 预期结果：全部成功；`newPeerReject` 剩余令牌仍为 100（成功发送不扣）。

### TC-SOC-08-05 去重优先：已保存的消息被拉黑后重发仍返回原 ack
- 关联 AC：AC-SOC-08-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`POST /admin/test/drop-ack {clientMsgId: m1}`。
- 步骤：alice 用 m1 给 bob 发（保存但不回 ack）；bob 拉黑 alice；alice 用 m1 重发。
- 预期结果：重发返回与第一次相同的 msgId、seq、ts、convId（不是 4036）；bob 能 `sync` 到 m1。

### TC-SOC-08-06 判定顺序：4042 > 4035 > 4036
- 关联 AC：AC-SOC-08-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 拉黑 alice，二者不是好友。
- 步骤：`mode=friends` 下 alice 给 `nobody` 发、给 bob 发；切 `mode=open` 后 alice 给 bob 发。
- 预期结果：依次 4042、4035、4036。

### TC-SOC-08-07 每小时新对象配额
- 关联 AC：AC-SOC-08-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`newPeerHourlyLimit=30`；测试时钟不动；u01～u31 存在、与 alice 无会话；carol 与 alice 已有会话。
- 步骤：alice 给 u01～u30 各发 1 条；给 u31 发；给 `nobody` 发；给 carol 发；测试时钟推进 1 小时后给 u31 发。
- 预期结果：前 30 条成功；u31 返回 4292；`nobody` 返回 4292（不是 4042）；给 carol 成功；推进后给 u31 成功。

### TC-SOC-08-08 给自己发消息：好友制下也成功、不扣新对象配额
- 关联 AC：AC-SOC-08-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`、`newPeerHourlyLimit=1`；alice 从未给自己发过。
- 步骤：alice 给自己发 3 条；再给一个存在的新对象 bob（好友）发 1 条。
- 预期结果：3 条都成功；`newPeerReject`、`newPeerHour` 剩余令牌不变；给 bob 的那条成功（`newPeerHour` 仍有 1 个令牌）。

### TC-SOC-08-09 聊天页提示：4035 好友验证提示与申请入口；4137 中的 N
- 关联 AC：AC-SOC-08-8
- 优先级：P0
- 层级：UI 自动化
- 前置条件：`mode=friends`；alice、bob 非好友；另一组开放模式 `strangerMsgLimit=3`、alice 已给 carol 发 3 条。
- 步骤：alice 在与 bob 的聊天页发消息；alice 在与 carol 的聊天页发消息。
- 预期结果：第一条标记失败，聊天页插入"对方开启了好友验证，你还不是他的好友"和"发送好友申请"按钮；第二条标记失败，提示"对方回复之前，最多只能发送 3 条消息"；都不自动重发。

### TC-SOC-08-10 SDK：权限类错误码透传、标记失败、不自动重发
- 关联 AC：AC-SOC-08-8
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport；本地 `strangerMsgLimit=3`。
- 步骤：`send` 四次，服务端分别回 4035、4036、4137、4292；推进假时钟 60 秒并模拟重连。
- 预期结果：4 条消息都为失败、`code` 依次为 4035 / 4036 / 4137 / 4292；`messageFor` 给出 9.4 节文案（4137 含"3 条"）；没有任何自动重发；4292 不按 429x 兜底重试。

### TC-SOC-08-11 被 4036 拒绝的消息不计入陌生人条数
- 关联 AC：AC-SOC-08-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式、`strangerMsgLimit=3`；alice 拉黑 bob。
- 步骤：bob 给 alice 发 1 条；alice 取消拉黑；bob 连发 4 条。
- 预期结果：第 1 条 4036；之后前 3 条成功、第 4 条 4137；bob 的 `sent_count=3`。

### TC-SOC-08-12 新对象被拒配额：100 次静默失败后对任何新对象 4292
- 关联 AC：AC-SOC-08-10
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：默认配置；测试时钟不动；n001～n101 不存在；bob 存在、与 alice 无会话；carol 与 alice 已有会话。
- 步骤：alice 给 n001～n100 各发 1 条；给 n101 发；给 bob 发；给 carol 发；测试时钟推进 15 分钟后给 bob 发。
- 预期结果：前 100 条都 4042；n101 返回 4292；bob 返回 4292，与 n101 的应答除 `id` 外相同；carol 成功；推进后给 bob 发成功（通过配额检查并保存，令牌退还）。

### TC-SOC-08-13 好友制下给 100 个非好友发：之后 4292
- 关联 AC：AC-SOC-08-11
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：`mode=friends`；u001～u101 存在、都不是 alice 的好友、与 alice 无会话。
- 步骤：alice 给 u001～u100 各发 1 条；给 u101 发。
- 预期结果：前 100 条 4035；第 101 条 4292。

### TC-SOC-08-14 陌生人限额并发：恰好 3 条成功、seq 连续（B6 前置）
- 关联 AC：AC-SOC-08-12
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：开放模式、`strangerMsgLimit=3`；alice、bob 无会话；发送限流在用例中 `limits/reset` 并调高。
- 步骤：alice 的两个连接同时各给 bob 发 10 条。
- 预期结果：恰好 3 条成功、17 条 4137；会话 seq 为 1、2、3；`sent_count=3`。

### TC-SOC-08-15 默认配置下现有 e2e 不改代码全部通过
- 关联 AC：AC-SOC-08-13
- 优先级：P0
- 层级：后端 e2e（回归）
- 前置条件：M3 服务端；不设置任何社交配置。
- 步骤：运行 `go test ./server/e2e/...`（不修改任何现有用例代码）；再以 `IM_SOCIAL_DEFAULTS={"mode":"friends"}` + 预置好友关系跑现有私聊用例（PRD-core V-21，作为本 PRD 回归，不作为 PRD-core AC 的通过条件）。
- 预期结果：默认配置下现有用例全部通过；friends 模式下预置了好友关系的用例全部通过。

### TC-SOC-08-16 默认配置下 TC-PERF 错误码与 P99 不回归
- 关联 AC：AC-SOC-08-13
- 优先级：P0
- 层级：压测
- 前置条件：基准机器；M3 前基线 `docs/reports/perf-2026-11-03.md`。
- 步骤：默认配置下运行 TC-PERF-00-01～07，各 3 次取中位数。
- 预期结果：错误率 0；TC-PERF-00-02 的错误码统计中没有 4292、4137、4035、4036；各场景 P99 与基线相比在 ±10% 以内；好友制下私聊发送 P99 比开放模式增加 ≤ 2ms。

### TC-SOC-08-17 新对象被拒配额并发：10 连接同时探测不突破
- 关联 AC：AC-SOC-08-14
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：`newPeerRejectDailyLimit=100`；测试时钟不动；alice 已给 99 个不存在的 userId 各发 1 条（都 4042）；alice 开 10 个连接。
- 步骤：10 个连接同时各给一个不同的不存在 userId 发 1 条；之后给存在的新对象 bob 发。
- 预期结果：恰好 1 条 4042、9 条 4292；给 bob 返回 4292。

### TC-SOC-08-18 预扣退还：成功发送后 newPeerReject 不变，发送令牌少 1
- 关联 AC：AC-SOC-08-15
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试接口可读取 alice 的令牌桶剩余量。
- 步骤：记录 `newPeerReject:alice`、发送限流的剩余令牌；alice 给存在的新对象 bob 发 1 条。
- 预期结果：发送成功；`newPeerReject` 剩余与发送前相同；发送令牌少 1。

### TC-SOC-08-19 被发送限流 4290 拒绝时不扣新对象配额
- 关联 AC：AC-SOC-08-15
- 优先级：P1
- 层级：后端 e2e
- 前置条件：测试时钟不动；alice 的发送令牌已用完。
- 步骤：alice 给不存在的 `nobody` 发 1 条。
- 预期结果：返回 4290；`newPeerReject` 剩余令牌不变（判定顺序第 3 步拒绝，第 4 步未执行）。

---

## F-SOC-09 用户搜索与"可被搜索"开关

### TC-SOC-09-01 搜索返回资料与关系，申请后变为 pendingOut
- 关联 AC：AC-SOC-09-1
- 优先级：P0
- 层级：SDK 集成
- 前置条件：bob 设置了昵称"小B"、头像 URL；alice 与 bob 无关系。
- 步骤：alice `searchUser("bob")`；alice `applyFriend("bob")`；再 `searchUser("bob")`。
- 预期结果：第一次返回 `{userId:"bob", nickname:"小B", avatar:…, official:false, relation:"none"}`；第二次 `relation:"pendingOut"`；SDK `relationOf("bob")` 也为 `pendingOut`。

### TC-SOC-09-02 relation 枚举按优先级取值
- 关联 AC：AC-SOC-09-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：carol 申请了 alice（alice 一侧 pendingIn）；dave 是 alice 的好友且被 alice 拉黑。
- 步骤：alice 搜索 carol、dave、alice。
- 预期结果：carol `pendingIn`；dave `blocked`（优先于 `friend`）；alice `self`。

### TC-SOC-09-03 只做精确匹配，前缀返回 null
- 关联 AC：AC-SOC-09-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 可被搜索。
- 步骤：alice 搜索 `bo`、`BOB`（大小写不同）、`  bob  `（前后空白）。
- 预期结果：`bo`、`BOB` 返回 `{user:null}`；`  bob  ` 去空白后命中 bob。

### TC-SOC-09-04 防枚举：不可被搜索与不存在逐字节相同
- 关联 AC：AC-SOC-09-3
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：bob `setPrivacy {searchable:false}`。
- 步骤：alice 搜索 `bob` 得 r1；搜索 `nobody` 得 r2。
- 预期结果：r1、r2 都是 `{user:null}`，JSON 去掉 `id` 后逐字节相同。

### TC-SOC-09-05 搜索限流 4290 与每日配额 4292
- 关联 AC：AC-SOC-09-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟；`searchDailyLimit=100`。
- 步骤：
  1. 测试时钟不动，alice 搜索 11 次。
  2. `limits/reset`；之后每次推进 6 秒搜索 1 次，共 101 次。
- 预期结果：1 前 10 次成功、第 11 次 4290；2 前 100 次成功、第 101 次 4292。

### TC-SOC-09-06 被对方拉黑后搜索结果不变
- 关联 AC：AC-SOC-09-5
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：alice 搜索 bob 记下 r1。
- 步骤：bob 拉黑 alice；alice 搜索 bob 得 r2。
- 预期结果：r1、r2 去掉 `id` 后逐字节相同（`relation` 仍为原值，不暴露拉黑）。

### TC-SOC-09-07 关键字格式、搜索自己、搜索被我拉黑的人
- 关联 AC：AC-SOC-09-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 关闭"可被搜索"；alice 拉黑 bob。
- 步骤：alice 搜索 `a b`；搜索 65 个字符的串；搜索 64 个字符的合法 ID；搜索 `alice`；搜索 `bob`。
- 预期结果：依次 4004、4004、`{user:null}`（不存在）、返回自己且 `relation=self`、`relation=blocked`。

### TC-SOC-09-08 隐私开关多端同步
- 关联 AC：AC-SOC-09-7
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice 的 web、pc 两个 SDK 实例。
- 步骤：web 实例 `setSearchable(false)`。
- 预期结果：web ack `changed=true`；pc 实例 2 秒内收到 `socialChanged`，同步后 `socialSettings.searchable=false`；再设一次 false 时 `changed=false`、pc 收不到帧。

### TC-SOC-09-09 日志不出现搜索关键字
- 关联 AC：AC-SOC-09-8
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：捕获服务端全部日志。
- 步骤：alice 搜索 `ZXQ-SEARCH-9`（合法 userId 格式，不存在）。
- 预期结果：日志中搜不到 `ZXQ-SEARCH-9`；日志只有"一次搜索、未命中"。

### TC-SOC-09-10 响应时间不可区分
- 关联 AC：AC-SOC-09-9
- 优先级：P1
- 层级：后端基准测试
- 前置条件：测试中关闭搜索限流；bob 不可被搜索；`nobody` 不存在。
- 步骤：交替对 bob、`nobody` 各搜索 1000 次，记录服务端处理耗时（从解码完成到写出应答）。
- 预期结果：两组中位数之差 ≤ 较小者的 20%；两条路径都执行了用户、隐私、关系三次查询（查询计数器断言）。

---

## F-SOC-10 批量查用户资料与可见范围

### TC-SOC-10-01 可见范围：同群、有单聊消息的人可见；无关和不存在的不返回
- 关联 AC：AC-SOC-10-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob；alice 与 carol 有单聊消息；dave 与 alice 无关系。
- 步骤：alice `users {userIds:[bob, carol, dave, nobody]}`。
- 预期结果：返回 bob、carol 两项（`userId, nickname, avatar, official`），没有 dave 和 `nobody`，不报错。

### TC-SOC-10-02 好友、自己、30 天内收到的申请可见
- 关联 AC：AC-SOC-10-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：eve 是 alice 的好友（无会话）；frank 给 alice 发了申请。
- 步骤：alice `users [alice, eve, frank, alice]`（含重复）。
- 预期结果：返回 alice、eve、frank 各一项（去重）。

### TC-SOC-10-03 被移出群后只在群里认识的人不可见
- 关联 AC：AC-SOC-10-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 只因同在 g1 而有关系；alice `users [bob]` 能返回 bob。
- 步骤：管理接口把 alice 移出 g1；alice 再 `users [bob]`。
- 预期结果：返回空列表。

### TC-SOC-10-04 参数与限流
- 关联 AC：AC-SOC-10-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟不动。
- 步骤：`users` 传 101 个 ID；传 100 个 ID；传 `[bob, "a b"]`；传空数组；`limits/reset` 后 1 分钟内调用 31 次（测试时钟不动时先 10 次突发，之后每 2 秒推进一次补令牌，累计 30 次成功后第 31 次）。
- 预期结果：101 个 4000；100 个成功；含不合法 ID 4004；空数组 4000；第 31 次 4290。

### TC-SOC-10-05 收到的申请 30 天后不再可见（不执行清理）
- 关联 AC：AC-SOC-10-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：dave 给 alice 发了申请，alice 未处理；二者无其他关系。
- 步骤：alice `users [dave]`；测试时钟推进到申请最后更新后 30 天零 1 秒（不 purge）；再查。
- 预期结果：第一次返回 dave；第二次返回空。

### TC-SOC-10-06 防枚举：申请不可被搜索的人后仍拿不到资料
- 关联 AC：AC-SOC-10-5
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：eve 不可被搜索、与 alice 无其他关系。
- 步骤：alice 申请 eve；alice `users [eve]` 得 r1；`users [nobody]` 得 r2；eve 同意；alice 再 `users [eve]`。
- 预期结果：r1、r2 都是 `{users:[]}`，去掉 `id` 后相同；alice 同步得到的 out 行 `profile=null`；eve 同意后返回 eve 的资料。

### TC-SOC-10-07 拉黑不授予可见
- 关联 AC：AC-SOC-10-6
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：frank 与 alice 无任何关系。
- 步骤：alice 拉黑 frank；alice `users [frank]`；alice 同步黑名单。
- 预期结果：`users` 返回空；黑名单项 frank 的 `profile=null`。

### TC-SOC-10-08 群消息显示发送者昵称（来自 users）
- 关联 AC：AC-SOC-10-7
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g1 成员 alice、bob；bob 昵称"小B"；alice 的 SDK 缓存为空。
- 步骤：bob 在 g1 发消息；alice 打开 g1。
- 预期结果：消息显示"小B"；SDK 发出 1 次 `users`（含 bob）。

### TC-SOC-10-09 SDK：getUsers 合并请求、每批 100、命中缓存不请求
- 关联 AC：AC-SOC-10-7
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport。
- 步骤：同一事件循环内调用 `getUsers` 3 次，共涉及 250 个不同 userId；之后再查其中 10 个。
- 预期结果：发出 3 个 `users` 请求（100 / 100 / 50），每个 userId 只出现一次；第二次查询不发请求。

### TC-SOC-10-10 SDK 资料缓存 10 分钟过期
- 关联 AC：AC-SOC-10-8
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeClock；bob 的资料已缓存（昵称"旧"）；服务端返回的新昵称为"新"。
- 步骤：假时钟推进 9 分 59 秒后显示 bob 的消息；再推进 2 秒后显示。
- 预期结果：第一次显示"旧"、不发请求；第二次发出 `users [bob]`，显示"新"。

### TC-SOC-10-11 同步应答带新资料时立即覆盖缓存
- 关联 AC：AC-SOC-10-9
- 优先级：P0
- 层级：SDK 集成
- 前置条件：alice、bob 互为好友；alice 的 SDK 已缓存 bob 昵称"旧"（1 分钟前）。
- 步骤：业务方 `POST /admin/users/import` 把 bob 昵称改为"新"；alice 的 SDK 开始一轮同步（重连触发全量）。
- 预期结果：同步应答中 bob 的 `profile.nickname="新"`；缓存立即更新、界面显示"新"，不等 10 分钟；`convs` 刷新不会清空缓存。

---

## F-SOC-11 通讯录数据

### TC-SOC-11-01 memberCount 随管理接口加减成员变化
- 关联 AC：AC-SOC-11-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：管理接口建 g1，5 人含 alice。
- 步骤：alice `convs`；加 2 人后 `convs`；移除 1 人后 `convs`、`myGroups`、`GET /admin/groups/g1`。
- 预期结果：`memberCount` 依次为 5、7、6；`myGroups` 与管理接口中 g1 的 `memberCount` 都为 6。

### TC-SOC-11-02 600 个群：convs 截断，myGroups 翻 3 页拿全
- 关联 AC：AC-SOC-11-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 在 600 个群里，并有 100 个单聊会话。
- 步骤：alice `convs`；`myGroups {limit:200}` 翻到 `nextCursor` 为空。
- 预期结果：`convs` 500 项、`truncated=true`；`myGroups` 3 页（200 / 200 / 200），按 groupId 升序，600 个无重复。

### TC-SOC-11-03 myGroups 参数与限流
- 关联 AC：AC-SOC-11-2
- 优先级：P1
- 层级：后端 e2e
- 前置条件：测试时钟不动。
- 步骤：`limit:0`；`limit:201`；伪造 cursor；连续调用 31 次。
- 预期结果：前三者 4000；`groupRead` 突发 30，第 31 次 4290。

### TC-SOC-11-04 通讯录显示好友、群聊、新的朋友红点
- 关联 AC：AC-SOC-11-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 有 3 个好友、在 2 个群里、有 1 条未读的待处理申请。
- 步骤：alice 打开通讯录；点"群聊"。
- 预期结果：好友列表 3 人；"群聊"里 2 个群；"新的朋友"红点为 1。SDK 侧 `friends` 3 项、`loadMyGroups()` 2 项、`unseen=1`。

### TC-SOC-11-05 开放模式下陌生人不进通讯录
- 关联 AC：AC-SOC-11-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：开放模式；alice 与陌生人 carol 有单聊。
- 步骤：alice 打开通讯录和会话列表。
- 预期结果：会话列表有 carol；通讯录好友列表没有 carol。

### TC-SOC-11-06 0004 迁移回填成员数
- 关联 AC：AC-SOC-11-5
- 优先级：P0
- 层级：迁移测试
- 前置条件：迁移到 0003 的数据库：3 个群，成员数 2、10、2000；若干单聊消息（alice→bob 7 条、bob→alice 2 条）。
- 步骤：`im-server migrate up` 执行 0004；查询 `convs` 和 `user_conv`。
- 预期结果：三个群 `memberCount` 分别为 2、10、2000；alice 在该单聊的 `sent_count=7`、bob 为 2；`group_members(user_id)` 索引存在。

### TC-SOC-11-07 repair counters 修复漂移且幂等
- 关联 AC：AC-SOC-11-5
- 优先级：P1
- 层级：存储契约
- 前置条件：人为把 g1 的 `member_count`、alice 的 `friend_count`、某会话 `sent_count` 改错。
- 步骤：`im-server repair counters`；再执行一次。
- 预期结果：第一次修复 3 条、值与基础表一致；第二次修复 0 条。

### TC-SOC-11-08 并发加减成员后 memberCount 准确（B6 前置）
- 关联 AC：AC-SOC-11-6
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：g1 有 100 人；另有 10 个不在群里的用户。
- 步骤：并发执行 10 个加成员（各 1 个新人）和 10 个移除成员（各 1 个老成员）的管理接口请求。
- 预期结果：`memberCount` 等于 `GET /admin/groups/g1` 返回的成员总数（100）。

---

## F-SOC-12 业务方管理好友关系

### TC-SOC-12-01 导入一对好友
- 关联 AC：AC-SOC-12-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；alice、bob 在线；bob 曾给 alice 发过待处理申请。
- 步骤：`POST /admin/friends/import {pairs:[{a:alice, b:bob, remarkA:"B"}]}`；双方同步。
- 预期结果：返回 `{imported:1}`；双方互为好友，`source=admin`，alice 一侧 bob 的备注为"B"；bob 的申请变为 `accepted`；alice、bob 都收到 `socialChanged`。

### TC-SOC-12-02 导入不受拉黑影响，也不解除拉黑
- 关联 AC：AC-SOC-12-1
- 优先级：P1
- 层级：后端 e2e
- 前置条件：alice 拉黑了 carol。
- 步骤：导入 alice-carol。
- 预期结果：导入成功、互为好友；alice 的黑名单仍有 carol；carol 给 alice 发私聊仍 4036。

### TC-SOC-12-03 重复导入幂等；备注变化才算变化
- 关联 AC：AC-SOC-12-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：同一请求 `{a:alice, b:bob, remarkA:"B"}` 调 3 次；第 4 次 `remarkA:"C"`。
- 预期结果：边各一条；alice、bob 的 ver 只在第 1 次 +1；第 4 次 alice 的 ver +1、bob 的不变。

### TC-SOC-12-04 数量上限、用户不存在、a==b
- 关联 AC：AC-SOC-12-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：导入 501 对；导入 `[alice-bob, carol-nobody]`；导入 `[{a:alice, b:alice}]`；导入 `[{a:"a b", b:bob}, {a:alice, b:alice}]`（同时有 4004 和 4000）。
- 预期结果：依次 400 / 4131；404 / 4042 且 alice-bob 没有导入；400 / 4000；400 / 4004（4004 优先）。

### TC-SOC-12-05 导入后超过 maxFriends 整批不生效
- 关联 AC：AC-SOC-12-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 有 1999 个好友。
- 步骤：导入 `[alice-bob, alice-carol]`。
- 预期结果：400 / 4132，`msg` 含 `alice`；bob、carol 都没有成为 alice 的好友。

### TC-SOC-12-06 管理接口删除好友：双向删除
- 关联 AC：AC-SOC-12-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`；alice、bob 互为好友。
- 步骤：`DELETE /admin/friends {pairs:[{a:alice, b:bob}]}`；双方互发私聊；删除不是好友的对 alice-carol。
- 预期结果：返回 `{removed:1}`；两侧都没有对方（墓碑）；双方私聊都 4035；删除 alice-carol 返回 `{removed:0}`。

### TC-SOC-12-07 查询好友分页、不返回备注
- 关联 AC：AC-SOC-12-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` alice 有 250 个好友且都有备注。
- 步骤：`GET /admin/users/alice/friends?limit=100` 翻到底；`limit=0`、`limit=501`。
- 预期结果：3 页（100 / 100 / 50），按对方 userId 升序，250 个无重复；每项只有 `userId, source, createdAt`，没有 `remark`；`limit` 越界 400 / 4000。

### TC-SOC-12-08 一批导入中同一用户出现 3 次，ver 只加 1
- 关联 AC：AC-SOC-12-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：记 alice 的 ver。
- 步骤：导入 `[alice-bob, alice-carol, alice-dave, alice-bob(remarkA:"X")]`（同批重复对以最后为准）。
- 预期结果：alice 的 ver +1；alice 一侧 bob 的备注为"X"；bob、carol、dave 的 ver 各 +1。

### TC-SOC-12-09 鉴权与 scope
- 关联 AC：AC-SOC-12-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；X-5 已落地时另准备一个只有 `social.settings` scope 的调用方密钥。
- 步骤：不带管理密钥导入；带错误密钥导入；（X-5 后）用只有 `social.settings` 的密钥导入。
- 预期结果：401 / 4013；401 / 4013；403 / 4030。

### TC-SOC-12-10 审计记录与备注特征串
- 关联 AC：AC-SOC-12-9
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：捕获服务端日志。
- 步骤：PATCH 配置 `strangerMsgLimit` 0→3；导入 `[{a:alice, b:bob, remarkA:"ZXQ-REMARK-789"}]`；删除 alice-bob；查询 `admin_audit`。
- 预期结果：各有一条记录，含调用方（测试模式 `test`）、接口、涉及 userId，配置记录含"0 → 3"，导入 / 删除记录含对数；`admin_audit` 全部字段和服务端日志中都搜不到 `ZXQ-REMARK-789`。

### TC-SOC-12-11 生产配置下社交管理接口未注册
- 关联 AC：AC-SOC-12-10
- 优先级：P0
- 层级：后端 e2e（子进程）
- 前置条件：`IM_TEST_MODE` 关闭、X-5 未落地（非生产 env 以允许启动）。
- 步骤：带正确管理密钥请求 `GET /admin/settings/social`、`POST /admin/friends/import`、`DELETE /admin/friends`、`GET /admin/users/alice/friends`、`PUT /admin/users/alice/official`、`POST /admin/test/social/seed`。
- 预期结果：全部 HTTP 404。

---

## F-SOC-13 官方账号

### TC-SOC-13-01 好友制下官方账号与非好友互发
- 关联 AC：AC-SOC-13-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`；kefu 与 alice 不是好友。
- 步骤：`PUT /admin/users/kefu/official {official:true}`；kefu 给 alice 发；alice 回复 kefu。
- 预期结果：两条都成功。

### TC-SOC-13-02 官方账号豁免陌生人限额和每小时新对象数
- 关联 AC：AC-SOC-13-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：开放模式、`strangerMsgLimit=3`、`newPeerHourlyLimit=30`；kefu 是官方账号；u01～u50 与 kefu 无会话。
- 步骤：kefu 给不回复的 alice 连发 10 条（每秒 1 条）；1 小时内（测试时钟）给 u01～u50 各发 1 条。
- 预期结果：全部成功。

### TC-SOC-13-03 官方账号被拉黑后 4036
- 关联 AC：AC-SOC-13-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：kefu 是官方账号。
- 步骤：alice 拉黑 kefu；kefu 给 alice 发。
- 预期结果：4036。

### TC-SOC-13-04 取消官方标记 1 秒内按普通规则判定
- 关联 AC：AC-SOC-13-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`mode=friends`；kefu 是官方账号，已与 alice 有会话。
- 步骤：`PUT /admin/users/kefu/official {official:false}`；1 秒内 kefu 给 alice 发。
- 预期结果：4035。

### TC-SOC-13-05 资料与应答头带 official
- 关联 AC：AC-SOC-13-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：kefu 是官方账号且可被搜索；kefu 已给 alice 发过消息。
- 步骤：alice `userSearch kefu`、`users [kefu]`；kefu `socialSync {sinceVer:0}`；`GET /admin/users/kefu`。
- 预期结果：搜索与 `users` 中 `official=true`；kefu 的应答头 `official=true`；管理接口返回 `official:true`；改标记前后 kefu 的 ver 不变。

### TC-SOC-13-06 标记不存在的用户；标记与取消都审计
- 关联 AC：AC-SOC-13-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：`PUT /admin/users/nobody/official {official:true}`；标记 kefu、取消 kefu；查询 `admin_audit`。
- 预期结果：第一次 404 / 4042；`admin_audit` 中有标记、取消各一条，含调用方、接口、`kefu`、旧值 → 新值。

### TC-SOC-13-07 聊天页标题和搜索结果显示"官方"标识
- 关联 AC：AC-SOC-13-7
- 优先级：P0
- 层级：UI 自动化
- 前置条件：kefu 是官方账号。
- 步骤：alice 搜索 kefu；打开与 kefu 的聊天页。
- 预期结果：搜索结果和聊天页标题都显示"官方"标识；普通用户 bob 不显示。

### TC-SOC-13-08 官方账号不豁免新对象被拒配额
- 关联 AC：AC-SOC-13-8
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：kefu 是官方账号；测试时钟不动。
- 步骤：kefu 给 100 个不存在的 userId 各发 1 条；再给存在的新对象 bob 发。
- 预期结果：前 100 条 4042；给 bob 4292。

---

## F-SOC-20 群角色与群资料（M4）

> 本节起为 M4。自建群 g1 默认：群主 alice，管理员 carol，成员 bob、dave；托管群 g2（管理接口建，无群主）成员 bob、carol；托管群 g3 由业务方指定 op1 为群主，成员 op1、bob、carol。

### TC-SOC-20-01 自建群 groupInfo 与 convs 的角色
- 关联 AC：AC-SOC-20-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 与 bob、carol 互为好友。
- 步骤：alice `groupCreate {members:[bob, carol]}`；alice `groupInfo`、`convs`、`myGroups`。
- 预期结果：`owner=alice`、`myRole=owner`、`managed=false`、`memberCount=3`、`maxMembers=500`、`historyVisible=false`；`convs` 和 `myGroups` 中该群 `myRole=owner`。

### TC-SOC-20-02 托管群 groupInfo
- 关联 AC：AC-SOC-20-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g2。
- 步骤：bob `groupInfo {groupId: g2}`、`convs`。
- 预期结果：`owner=""`、`managed=true`、`myRole=member`、`maxMembers=2000`；`convs` 中 g2 `myRole=member`。

### TC-SOC-20-03 非成员 4033、已解散 4043
- 关联 AC：AC-SOC-20-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：eve 不在 g1；g4 是已解散的自建群。
- 步骤：eve `groupInfo g1`；alice `groupInfo g4`；alice `groupInfo` 不存在的 `gx`。
- 预期结果：4033、4043、4043。

### TC-SOC-20-04 成员修改群名 4030
- 关联 AC：AC-SOC-20-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：bob `groupUpdate {groupId: g1, name: "x"}`。
- 预期结果：4030；群名不变、maxSeq 不变。

### TC-SOC-20-05 管理员移出管理员 4030
- 关联 AC：AC-SOC-20-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 中 dave 也被设为管理员。
- 步骤：carol `groupKick {groupId: g1, members:[dave]}`。
- 预期结果：4030；dave 仍在群里且仍是管理员。

### TC-SOC-20-06 管理员转让群主 4030
- 关联 AC：AC-SOC-20-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：carol `groupTransfer {groupId: g1, to: bob}`。
- 预期结果：4030；群主仍为 alice。

### TC-SOC-20-07 托管群（无群主）成员邀请 4030
- 关联 AC：AC-SOC-20-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g2；bob 与 eve 互为好友。
- 步骤：bob `groupInvite {groupId: g2, members:[eve]}`。
- 预期结果：4030；eve 不在 g2。

### TC-SOC-20-08 权限矩阵全量遍历（5 × 7）
- 关联 AC：AC-SOC-20-8
- 优先级：P0
- 层级：后端 e2e（表驱动）
- 前置条件：每格独立建数据：自建群群主 / 管理员 / 成员，托管群有群主时的群主（g3 的 op1），托管群无群主时的成员（g2 的 bob）。
- 步骤：每类角色执行 7 类操作：发言、邀请好友、改名、改公告、移出（目标为普通成员）、设管理员 / 转让、解散。
- 预期结果（✅ 成功 / ❌ 4030）：

  | 角色 | 发言 | 邀请 | 改名 | 改公告 | 移出成员 | 设管理员 / 转让 | 解散 |
  |---|---|---|---|---|---|---|---|
  | 自建群群主 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
  | 自建群管理员 | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
  | 自建群成员 | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
  | 托管群群主 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
  | 托管群（无群主）成员 | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

  35 格每格一条断言；测试数据从 PRD 表格手抄，不引用服务端权限表。

### TC-SOC-20-09 并发邀请与移出后 memberCount 准确（B6 前置）
- 关联 AC：AC-SOC-20-9
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：自建群 g1 有 50 人；群主 alice 与 20 个群外用户互为好友。
- 步骤：并发执行 20 个邀请（各 1 人）和 10 个移出（各 1 个老成员）。
- 预期结果：`groupInfo.memberCount` 等于 `groupMembers` 翻完得到的总数（60）。

### TC-SOC-20-10 groupChanged 1 秒合并
- 关联 AC：AC-SOC-20-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 有 15 人，其中 3 人在线（观察者）。
- 步骤：1 秒内 10 名成员先后 `groupLeave`；之后观察 2 秒；观察者调 `groupInfo`。
- 预期结果：每个观察者在这 3 秒内收到的 g1 `groupChanged` ≤ 2 条；`memberCount=5`。

### TC-SOC-20-11 SDK：收到 groupChanged 后用 groupInfo 更新本地会话
- 关联 AC：AC-SOC-20-10
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport；本地会话 g1 `myRole=member`、`memberCount=5`。
- 步骤：连续注入 3 个 `groupChanged {groupId: g1}`；`groupInfo` 应答 `{myRole: admin, memberCount: 6, name: "新名"}`。
- 预期结果：只发出 1 个在途 `groupInfo`（合并）；本地会话更新为 admin / 6 / 新名；没有重拉 `convs`。

---

## F-SOC-21 客户端建群（M4）

### TC-SOC-21-01 建群：被邀请人和本人其他端收到 convAdded，第一条为 created 提示
- 关联 AC：AC-SOC-21-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 与 bob、carol 互为好友；alice 两个连接、bob、carol 在线；昵称与 userId 相同。
- 步骤：alice 连接 1 `groupCreate {clientReqId: "req-0001", members:[bob, carol]}`；bob `sync` 该群；alice `convs`。
- 预期结果：ack 含 `groupId`（`ug-` + 32 位十六进制，共 35 个字符）和 `conv`；bob、carol、alice 连接 2 在 1 秒内收到 `convAdded`；第一条消息 seq=1、`type=tip`、`from=alice`、`body.tip.kind=created`、`actor=alice`、`targets=[bob, carol]`；`lastMsg.text` 为"alice 邀请 bob、carol 加入了群聊"；alice 的 `readSeq=1`。

### TC-SOC-21-02 SDK：createGroup 后界面得到会话
- 关联 AC：AC-SOC-21-1
- 优先级：P1
- 层级：SDK 集成
- 前置条件：同上，使用 sdk-ts。
- 步骤：alice `createGroup({members:["bob","carol"]})`。
- 预期结果：返回 `{groupId, conv}`；alice 本地会话列表出现该群（`myRole=owner`）；bob 的会话列表 1 秒内出现该群、未读 1。

### TC-SOC-21-03 同一 clientReqId 重复提交只建一个群
- 关联 AC：AC-SOC-21-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-SOC-21-01。
- 步骤：用同一个 `clientReqId` 提交 3 次（第 2 次改了 `name`）。
- 预期结果：3 次返回同一个 groupId；`myGroups` 中只多 1 个群；bob 只收到 1 次 `convAdded`；群名为第 1 次的结果。

### TC-SOC-21-04 SDK：超时重试使用同一个 clientReqId
- 关联 AC：AC-SOC-21-2
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport 丢弃第一次 ack。
- 步骤：`createGroup(...)`；假时钟推进到超时触发重试。
- 预期结果：两次请求的 `clientReqId` 相同；最终 resolve 为第二次的 ack。

### TC-SOC-21-05 成员数不足、包含自己
- 关联 AC：AC-SOC-21-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 与 bob、carol 互为好友。
- 步骤：`members:[bob]`；`members:[alice, bob]`；`members:[bob, bob]`；`members:[alice, bob, carol]`。
- 预期结果：前三个 4000（去掉自己、去重后不足 2 人）；第四个成功（忽略自己），群 3 人。

### TC-SOC-21-06 包含非好友：4035 + userId，群未创建，无事件
- 关联 AC：AC-SOC-21-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：dave 不是 alice 的好友；bob 是；bob、dave 在线。
- 步骤：alice `groupCreate {members:[bob, dave]}`。
- 预期结果：4035，`error.userId=dave`；没有新群（`myGroups` 不变）；任何人 2 秒内收不到事件；`client_group_req` 没有记录。

### TC-SOC-21-07 防枚举：拉黑了我、不存在、非好友同构
- 关联 AC：AC-SOC-21-5
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：frank 是 alice 的好友但拉黑了 alice；dave 不是好友；bob 是好友。
- 步骤：分别建群 `[bob, frank]`、`[bob, nobody]`、`[bob, dave]`。
- 预期结果：都返回 4035，`error.userId` 分别为 frank、nobody、dave；三个错误应答去掉 `id`、`userId` 后逐字节相同。

### TC-SOC-21-08 自建群人数上限
- 关联 AC：AC-SOC-21-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`clientGroupMaxMembers=500`；`seed` alice 与 u001～u500 互为好友。
- 步骤：建群邀请 u001～u499；再建群邀请 u001～u500。
- 预期结果：第一次成功（500 人）、`maxMembers=500`；第二次 4130。

### TC-SOC-21-09 默认群名用昵称，不用备注
- 关联 AC：AC-SOC-21-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob、carol 昵称分别为"A""B""C"，互为好友。
- 步骤：alice 不传 `name` 建群 `[bob, carol]`；alice 把 bob 的备注设为"老板"；再建一个群 `[bob, carol]`。
- 预期结果：两个群名都是"A、B、C"；提示消息和 `lastMsg.text` 中都不含"老板"。

### TC-SOC-21-10 默认群名超过 50 码点截断
- 关联 AC：AC-SOC-21-7
- 优先级：P1
- 层级：后端 e2e
- 前置条件：bob、carol 昵称各 30 个码点。
- 步骤：alice 不传 `name` 建群。
- 预期结果：群名为"A、{bob 昵称}、{carol 昵称}"的前 50 个码点。

### TC-SOC-21-11 关闭客户端建群；每日建群配额
- 关联 AC：AC-SOC-21-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试时钟不动。
- 步骤：PATCH `clientGroupEnabled=false`，alice 建群；改回 true；alice 建 11 个群（每次不同 clientReqId、间隔 1 秒避开 `groupOp`）。
- 预期结果：第一次 4030；之后前 10 个成功、第 11 个 4292。

### TC-SOC-21-12 被邀请人加入的群数已满：4135 + userId
- 关联 AC：AC-SOC-21-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`maxJoinedGroups=500`；`seed` bob 已在 500 个群里（其中 300 个是管理接口拉入的）；alice 与 bob、carol 互为好友。
- 步骤：alice 建群 `[bob, carol]`。
- 预期结果：4135，`error.userId=bob`；群未创建。另：`seed` alice 自己在 500 个群里时建群返回 4135。

### TC-SOC-21-13 业务方不能使用 ug- 前缀
- 关联 AC：AC-SOC-21-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：`POST /admin/groups {groupId: "ug-123", name: "x", members: []}`。
- 预期结果：400 / 4004。

### TC-SOC-21-14 失败不记录，同一 clientReqId 重试重新判定
- 关联 AC：AC-SOC-21-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：dave 不是 alice 的好友。
- 步骤：alice 用 `clientReqId=R` 建群 `[bob, dave]`；alice、dave 成为好友；alice 用同一个 R 再建。
- 预期结果：第一次 4035；第二次建群成功。

### TC-SOC-21-15 0005 迁移预检：已有 ug- 群 ID 时中止
- 关联 AC：AC-SOC-21-10
- 优先级：P1
- 层级：迁移测试
- 前置条件：0004 数据库中直接写入一个 `ug-legacy` 群。
- 步骤：`im-server migrate up`。
- 预期结果：迁移中止、退出码非 0，输出中列出 `ug-legacy`；`schema_migrations` 中没有 0005。

---

## F-SOC-22 邀请入群（M4）

### TC-SOC-22-01 成员邀请好友：convAdded 与 joined 提示
- 关联 AC：AC-SOC-22-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；bob 与 dave 互为好友（dave 不在群里）；dave、alice、carol 在线。
- 步骤：bob `groupInvite {groupId: g1, members:[dave]}`。
- 预期结果：ack `added=[dave]`、`failed=[]`；dave 1 秒内收到 `convAdded`；在线成员收到 `type=tip`、`kind=joined`、`actor=bob`、`targets=[dave]`、`from=bob` 的消息。

### TC-SOC-22-02 部分成功：非好友、拉黑了我、不存在都是 4035
- 关联 AC：AC-SOC-22-2
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：bob 与 dave、frank 互为好友，frank 拉黑了 bob；eve 不是 bob 的好友。
- 步骤：bob 邀请 `[dave, eve, frank, nobody]`。
- 预期结果：`added=[dave]`；`failed=[{userId:eve, code:4035}, {userId:frank, code:4035}, {userId:nobody, code:4035}]`。

### TC-SOC-22-03 邀请已在群里的人：忽略、无事件、无提示
- 关联 AC：AC-SOC-22-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；记 carol 的 joinSeq 和群 maxSeq；carol 在线。
- 步骤：bob 邀请 `[carol]`。
- 预期结果：`added=[]`、`failed=[]`；carol 的 joinSeq 不变；carol 2 秒内收不到任何帧；群 maxSeq 不变。

### TC-SOC-22-04 加完超过上限整次 4130
- 关联 AC：AC-SOC-22-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` 自建群 g5 有 499 人（`max_members=500`）；邀请人与 x、y 互为好友。
- 步骤：邀请 `[x, y]`；再邀请 `[x]`。
- 预期结果：第一次 4130，一人都没加；第二次成功，500 人。

### TC-SOC-22-05 客户端邀请：新成员 minSeq 处看到入群提示、未读 1
- 关联 AC：AC-SOC-22-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 已有 10 条消息、`historyVisible=false`；bob 与 dave 互为好友。
- 步骤：bob 邀请 dave；dave `convs`、`sync {afterSeq: 0}`。另一组对照：管理接口把 eve 加入 g1，eve `convs`。
- 预期结果：dave 的 `minSeq=11`、`unread=1`，`sync` 只返回 seq=11 的 `joined` 提示；eve（管理接口加入）`minSeq=12`、`unread=0`、群 maxSeq 不因加 eve 变化（AC-GRP-01-6 不变）。

### TC-SOC-22-06 参数、非成员、托管群
- 关联 AC：AC-SOC-22-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1、g2。
- 步骤：bob 一次邀请 51 人；非成员 eve 邀请；g2 成员 bob 邀请；邀请空数组。
- 预期结果：4000、4033、4030、4000。

### TC-SOC-22-07 邀请与解散并发（B6 前置）
- 关联 AC：AC-SOC-22-7
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：每轮新建自建群（群主 alice、成员 bob）；bob 与 x 互为好友。
- 步骤：bob 邀请 x 与 alice 解散同时发出，重复 50 轮。
- 预期结果：每轮要么邀请返回 4043，要么邀请成功后被解散（x 收到 `convAdded` 后收到 `convRemoved {reason:"dissolved"}`）；结束时不存在 `dissolved_at ≠ 0` 但还有 `group_members` 行的群。

---

## F-SOC-23 退群（M4）

### TC-SOC-23-01 退群：本人其他端 convRemoved left，之后 4033
- 关联 AC：AC-SOC-23-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 的 web、pc 在线；记 g1 的 maxSeq 和 memberCount。
- 步骤：bob web `groupLeave {groupId: g1}`；bob `send`、`sync` g1；alice `groupInfo g1`。
- 预期结果：pc 收到 `convRemoved {convId: group:g1, reason: "left"}`；`send`、`sync` 返回 4033；g1 maxSeq 不变（不生成提示）；`memberCount` 减 1；bob 的 `convs`、`myGroups` 不再有 g1。

### TC-SOC-23-02 SDK：left 原因保留会话展示信息
- 关联 AC：AC-SOC-23-1
- 优先级：P1
- 层级：SDK 单测
- 前置条件：FakeTransport。
- 步骤：注入 `convRemoved {reason: "left"}`、`{reason: "unknownX"}`。
- 预期结果：会话标记为已退出、保留名称和最后一条消息；未知 reason 按 `removed` 处理。

### TC-SOC-23-03 群主退群：转给最早的管理员
- 关联 AC：AC-SOC-23-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：自建群：群主 alice，成员 bob（先入群），管理员 carol（后入群）。
- 步骤：alice 退群；bob `groupInfo`；bob `sync`。
- 预期结果：`owner=carol`；carol 的 `myRole=owner`；群里新增 `ownerChanged` 提示，`from=""`、`targets=[carol]`；不推进任何人的 readSeq。

### TC-SOC-23-04 群主退群且没有管理员：转给最早入群的成员
- 关联 AC：AC-SOC-23-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：自建群：群主 alice，成员 bob 早于 dave 入群，无管理员。
- 步骤：alice 退群。
- 预期结果：新群主为 bob。另：bob 与 dave 同一毫秒入群时按 userId 取 bob。

### TC-SOC-23-05 自建群最后一人退群：解散，ID 不能被业务方使用
- 关联 AC：AC-SOC-23-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：自建群 `ug-…` 只剩 alice。
- 步骤：alice 退群；alice `groupInfo`；`GET /admin/groups/{该群}`；`POST /admin/groups {groupId: 该群 ID}`。
- 预期结果：4043；404 / 4043；400 / 4004；`chat_groups.dissolved_at ≠ 0`。

### TC-SOC-23-06 托管群退群与最后一人退群后保留空群
- 关联 AC：AC-SOC-23-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g2 成员 bob、carol。
- 步骤：bob 退群；`GET /admin/groups/g2`；carol 退群；`GET /admin/groups/g2`；eve 给 g2 发消息。
- 预期结果：第一次成员只有 carol；第二次 200、成员为空、`memberCount=0`；eve 发消息 4033（AC-GRP-02-3 不变）。

### TC-SOC-23-07 非成员退群 4033
- 关联 AC：AC-SOC-23-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：carol 不在 g6。
- 步骤：carol `groupLeave {groupId: g6}`；退出不存在的群 `gx`。
- 预期结果：4033；4043。

### TC-SOC-23-08 托管群群主退群：回到无群主
- 关联 AC：AC-SOC-23-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g3（群主 op1，成员 bob、carol）。
- 步骤：op1 退群；bob `groupInfo`；bob `groupUpdate {name: "x"}`；bob `sync`。
- 预期结果：`owner=""`、`managed=true`；改名 4030；群里出现 `ownerChanged`，`from=""`、`targets=[]`。

### TC-SOC-23-09 最后两人同时退群（B6 前置）
- 关联 AC：AC-SOC-23-8
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：每轮新建只剩 2 人的自建群。
- 步骤：两人同时 `groupLeave`，重复 50 轮。
- 预期结果：每轮结束群都已解散（`dissolved_at ≠ 0`、无成员行）；从不出现"0 个成员但未解散"。

### TC-SOC-23-10 群主退群与转让并发（B6 前置）
- 关联 AC：AC-SOC-23-9
- 优先级：P0
- 层级：Postgres 并发（B6 前置）
- 前置条件：每轮新建自建群：群主 alice，成员 bob、carol。
- 步骤：alice `groupLeave` 与 alice `groupTransfer {to: bob}` 同时发出，重复 50 轮。
- 预期结果：每轮结束恰好一个群主，且群主是群成员（`owner_id` 与成员行 `role=owner` 一致）。

---

## F-SOC-24 移出成员（M4）

### TC-SOC-24-01 群主移出成员
- 关联 AC：AC-SOC-24-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；bob 在线；记 maxSeq。
- 步骤：alice `groupKick {groupId: g1, members:[bob]}`；bob `send` g1。
- 预期结果：ack `removed=[bob]`；bob 1 秒内收到 `convRemoved {reason: "removed"}`；`send` 4033；maxSeq 不变；其他在线成员收到 `groupChanged`。

### TC-SOC-24-02 管理员的移出权限
- 关联 AC：AC-SOC-24-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；dave 被设为管理员。
- 步骤：carol 移出 bob；carol 移出 dave；carol 移出 alice。
- 预期结果：成功；4030；4030。

### TC-SOC-24-03 移出自己 4000；非成员不进 removed
- 关联 AC：AC-SOC-24-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；eve 不在群里。
- 步骤：alice 移出 `[alice]`；alice 移出 `[bob, eve]`；alice 一次移出 51 人。
- 预期结果：4000；`removed=[bob]`；4000。

### TC-SOC-24-04 被移出的管理员再入群为普通成员
- 关联 AC：AC-SOC-24-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；dave 是管理员，与 alice 互为好友。
- 步骤：alice 移出 dave；alice 邀请 dave；dave `groupInfo`。
- 预期结果：`myRole=member`；`groupInfo` 的管理员数比移出前少 1。

---

## F-SOC-25 转让群主与设置管理员（M4）

### TC-SOC-25-01 转让群主
- 关联 AC：AC-SOC-25-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：alice `groupTransfer {groupId: g1, to: bob}`；alice、bob `groupInfo`；成员 `sync`；alice 移出 dave。
- 预期结果：`owner=bob`；alice `myRole=member`；成员收到 `ownerChanged` 提示（`from=alice`、`targets=[bob]`），alice 的 readSeq 推进到该 seq；alice 移出 4030。

### TC-SOC-25-02 转让错误：非成员、管理员调用、转给自己
- 关联 AC：AC-SOC-25-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；eve 不在群里。
- 步骤：alice 转给 eve；carol 转给 bob；alice 转给 alice。
- 预期结果：4046；4030；4000。

### TC-SOC-25-03 管理员上限与幂等
- 关联 AC：AC-SOC-25-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` 自建群 g7：群主 alice，已有 10 个管理员（含 carol），成员 bob。
- 步骤：alice `groupSetAdmin {userId: bob, admin: true}`；记 maxSeq；alice 再设 carol 为管理员。
- 预期结果：4136；设 carol 返回成功，maxSeq 不变（不生成提示）。

### TC-SOC-25-04 设置 / 取消管理员的提示与错误
- 关联 AC：AC-SOC-25-3
- 优先级：P1
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：alice 设 bob 为管理员；取消 bob；对非成员 eve 设置；对自己设置；carol（管理员）设 dave。
- 预期结果：依次生成 `adminSet`、`adminUnset` 提示（`from=alice`、`targets=[bob]`）；4046；4000；4030。

### TC-SOC-25-05 转让给管理员：管理员数减 1
- 关联 AC：AC-SOC-25-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（管理员 carol，共 1 个管理员）。
- 步骤：alice 转让给 carol；`GET /admin/groups/g1`。
- 预期结果：carol `myRole=owner`；`admins` 为空（原来 1 个，减 1）；alice 为普通成员。

---

## F-SOC-26 解散群

### TC-SOC-26-01 客户端解散：在线成员 convRemoved dissolved，离线成员上线后不再有该群
- 关联 AC：AC-SOC-26-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：自建群 g1：群主 alice，bob、carol、dave 在线，eve 离线。
- 步骤：alice `groupDissolve {groupId: g1}`；eve 上线 `convs`。
- 预期结果：bob、carol、dave 1 秒内收到 `convRemoved {reason: "dissolved"}`；alice 其他连接也收到；eve 的 `convs` 中没有 g1；不生成提示消息。

### TC-SOC-26-02 SDK：dissolved 原因区分显示
- 关联 AC：AC-SOC-26-1
- 优先级：P0
- 层级：SDK 单测
- 前置条件：FakeTransport；本地有 g1 会话。
- 步骤：注入 `convRemoved {convId: group:g1, reason: "dissolved"}`。
- 预期结果：会话标记为"已解散"（与 `removed` 区分），保留名称和最后一条消息；对该群调用 `send` 前本地即可判定不可发。

### TC-SOC-26-03 解散后发、拉、读都 4043，管理接口 404
- 关联 AC：AC-SOC-26-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 已被解散（M3 用管理接口 `DELETE /admin/groups/g1`；M4 再用客户端解散各跑一遍）。
- 步骤：原成员 bob `send`、`sync`、`read` g1；`GET /admin/groups/g1`；bob `myGroups`。
- 预期结果：三个命令都 4043；管理接口 404 / 4043；`myGroups` 中没有 g1。

### TC-SOC-26-04 解散权限
- 关联 AC：AC-SOC-26-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1、g2、g3。
- 步骤：carol（g1 管理员）解散 g1；bob（g2 成员）解散 g2；op1（g3 群主）解散 g3。
- 预期结果：都 4030，三个群都还在。

### TC-SOC-26-05 存储契约：数据保留与 F-SOC-26 规则 3 的每个查询
- 关联 AC：AC-SOC-26-4
- 优先级：P0
- 层级：存储契约
- 前置条件：`GroupDissolveNoReuse`；g1 有 5 条消息、3 个成员。
- 步骤：`DissolveGroup(g1)`；逐个调用 `GroupExists`（`memberCache`）、`GetGroup`、管理接口 `getGroup` / 成员查询 / 加成员 / 改设置、`ListConversations`、`GetConversation`、`GroupConversations`、`MyGroups`、`VisibleUsers`、`CreateGroup(g1)`；M4 另加 `groupInfo`、`ListGroupMembers`。
- 预期结果：`messages` 中 5 条仍在；`group_members` 中没有 g1 的行；`dissolved_at ≠ 0`；每个查询各一条断言：存在性查询为"不存在 / 4043"，列表查询不含 g1，`VisibleUsers` 不因 g1 返回成员，`CreateGroup` 返回 4090。

### TC-SOC-26-06 解散后同群关系不再授予资料可见
- 关联 AC：AC-SOC-26-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 只因同在 g1 互相可见。
- 步骤：解散 g1；alice `users [bob]`。
- 预期结果：返回空列表。

### TC-SOC-26-07 管理接口解散托管群（M3）：效果相同，ID 不复用
- 关联 AC：AC-SOC-26-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g2 成员 bob（在线）、carol（离线）。
- 步骤：`DELETE /admin/groups/g2`；carol 上线 `convs`；bob `send`、`sync`、`read` g2；`POST /admin/groups {groupId: "g2", name: "x", members: []}`。
- 预期结果：bob 1 秒内收到 `convRemoved {reason: "dissolved"}`；carol 的 `convs` 没有 g2；三个命令 4043；重建返回 409 / 4090；管理接口移除成员时的 `convRemoved` 带 `reason: "removed"`（对照）。

### TC-SOC-26-08 客户端解散与管理接口解散两条路径结果一致
- 关联 AC：AC-SOC-26-1、AC-SOC-26-6
- 优先级：P1
- 层级：存储契约
- 前置条件：两个同构的群（成员、消息相同）。
- 步骤：一个用 `groupDissolve`、一个用 `DELETE /admin/groups/{id}`；比较两个群解散后的数据库状态和下发事件。
- 预期结果：`chat_groups`（除 ID 和时间外）、`group_members`、`messages`、`member_count` 状态相同；事件都是 `convRemoved {reason: "dissolved"}`；都触发了 `TopicGroupMembers` 失效。

---

## F-SOC-27 修改群名与群公告（M4）

### TC-SOC-27-01 管理员改群名：renamed 提示，下一次 convs 生效
- 关联 AC：AC-SOC-27-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；bob 在线。
- 步骤：carol `groupUpdate {groupId: g1, name: "周末球局"}`；bob `convs`。
- 预期结果：在线成员收到 `type=tip`、`kind=renamed`、`text="周末球局"`、`from=carol` 的消息和 `groupChanged`；`convs` 中 g1 `name="周末球局"`。

### TC-SOC-27-02 改群名的权限与参数
- 关联 AC：AC-SOC-27-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：bob 改名；alice 改名为 `""`；改名为 `"   "`；改名为 51 码点；`groupUpdate {groupId: g1}`（都不传）；改名为 50 码点。
- 预期结果：4030；4000；4000；4000；4000；成功。

### TC-SOC-27-03 公告长度、清除、noticeBy 与 noticeAt
- 关联 AC：AC-SOC-27-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；测试时钟设为 T。
- 步骤：carol 设公告 1000 码点；alice `groupInfo`；设 1001 码点；设 `""`。
- 预期结果：成功，`notice` 为完整 1000 码点、`noticeBy=carol`、`noticeAt=T`，`noticeUpdated` 提示的 `text` 为前 100 码点；1001 码点 4000；空串清除公告并生成 `noticeUpdated` 提示。

### TC-SOC-27-04 公告提示显示摘要，点击看全文
- 关联 AC：AC-SOC-27-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g1；carol 发了 300 码点的公告。
- 步骤：bob 打开 g1 聊天页；点击公告提示。
- 预期结果：聊天页显示公告摘要（前 100 码点）；点击后显示全文（SDK 调 `getGroupInfo`）。

### TC-SOC-27-05 日志不出现公告原文
- 关联 AC：AC-SOC-27-5
- 优先级：P0
- 层级：后端 e2e（安全）
- 前置条件：捕获服务端日志。
- 步骤：carol 设公告 `ZXQ-NOTICE-321`；再设一个 1001 码点的公告（触发 4000）。
- 预期结果：日志中搜不到 `ZXQ-NOTICE-321`；4000 应答的 `msg` 不含用户输入。

---

## F-SOC-28 群成员列表（M4）

### TC-SOC-28-01 200 人群翻 4 页，第一项是群主
- 关联 AC：AC-SOC-28-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`seed` 自建群 g8 共 200 人：群主 alice、管理员 3 人、成员 196 人。
- 步骤：成员 bob `groupMembers {groupId: g8, limit: 50}` 翻到 `nextCursor` 为空。
- 预期结果：4 页，每页 50；200 人无重复；第一项 `role=owner`（alice），接着 3 个管理员（按 `role_at`、userId），再成员（按入群时间、userId）；每项带 `nickname, avatar, official`。

### TC-SOC-28-02 翻页中已返回的成员退群、新成员入群
- 关联 AC：AC-SOC-28-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g8 同上；m10 在第 1 页。
- 步骤：翻完第 2 页；m10 退群；新成员 x 入群；继续翻完。
- 预期结果：m10 只出现 1 次（第 1 页）；x 出现在最后一页；没有重复项。

### TC-SOC-28-03 翻页中尚未返回的成员退群
- 关联 AC：AC-SOC-28-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g8；m180 排在第 4 页。
- 步骤：翻完第 2 页；m180 退群；继续翻完。
- 预期结果：结果中没有 m180；其余 199 人无重复、无遗漏。

### TC-SOC-28-04 非成员、limit、伪造游标
- 关联 AC：AC-SOC-28-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g8；eve 不在群里。
- 步骤：eve 查询；bob `limit:101`；bob `limit:0`；bob 用伪造 cursor；bob 用 g1 的 cursor 查 g8。
- 预期结果：4033；4000；4000；4000；4000。

### TC-SOC-28-05 聊天信息页显示前 20 个成员，滚动加载全部
- 关联 AC：AC-SOC-28-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g8（200 人）。
- 步骤：bob 打开聊天信息页；点"查看全部成员"并滚动到底。
- 预期结果：信息页显示 20 个头像和"查看全部成员"；成员列表滚动加载到 200 人（SDK `loadGroupMembers` 调用 4 次）。

---

## F-SOC-29 群提示消息（M4）

### TC-SOC-29-01 六种操作依次生成 6 条 tip，seq 连续
- 关联 AC：AC-SOC-29-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 与 bob、carol、dave 互为好友。
- 步骤：alice 建群 `[bob, carol]`；alice 邀请 dave；alice 转让给 bob；bob 设 carol 为管理员；carol 改群名；carol 改公告；bob `sync {afterSeq: 0}`。
- 预期结果：6 条 `type=tip`，seq 1～6，`kind` 依次 `created`、`joined`、`ownerChanged`、`adminSet`、`renamed`、`noticeUpdated`；`clientMsgId` 都以 `tip-` 开头。

### TC-SOC-29-02 退群、移出不生成提示
- 关联 AC：AC-SOC-29-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；记 maxSeq。
- 步骤：bob 退群；alice 移出 dave。
- 预期结果：maxSeq 不变。

### TC-SOC-29-03 管理接口操作不生成提示（PRD-core AC 不变）
- 关联 AC：AC-SOC-29-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u001～u500 已导入。
- 步骤：管理接口建托管群 g9；一次加 500 人；改名；改 `historyVisible=true`；移除 1 人。
- 预期结果：g9 的 maxSeq 为 0；没有任何 tip；TC-GRP 现有用例不改代码全过。

### TC-SOC-29-04 客户端不能发 tip、不能用 tip- 前缀
- 关联 AC：AC-SOC-29-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1。
- 步骤：alice `send {type: "tip", …}`；alice `send {clientMsgId: "tip-abcdefgh", type: "text", …}`；alice 用 `clientMsgId: "tipabcdefgh"` 发。
- 预期结果：4000；4000；成功（只拒绝 `tip-` 前缀）。

### TC-SOC-29-05 个性化渲染与服务端摘要
- 关联 AC：AC-SOC-29-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g1；alice 邀请 dave。
- 步骤：alice、bob 分别打开 g1 聊天页和会话列表。
- 预期结果：alice 看到"你邀请 dave 加入了群聊"，bob 看到"alice 邀请 dave 加入了群聊"；会话列表摘要显示服务端生成的 `lastMsg.text`。

### TC-SOC-29-06 SDK：未知 kind 显示"[群通知]"，tip 计入未读
- 关联 AC：AC-SOC-29-5
- 优先级：P0
- 层级：SDK 单测（vectors/social.json）
- 前置条件：FakeTransport。
- 步骤：注入 `kind=joined`、`kind=futureKind` 两条 tip；调用 `tipText`。
- 预期结果：第一条按 actor / targets 生成个性化文案（自己是 actor 时为"你邀请了…"）；第二条为"[群通知]"；两条都计入未读；SDK 生成的 `clientMsgId` 1000 次都不以 `tip-` 开头。

### TC-SOC-29-07 Schema：tip 允许 from 为空
- 关联 AC：AC-SOC-29-6
- 优先级：P0
- 层级：协议测试
- 前置条件：`protocol/vectors/frames.json` 新增用例。
- 步骤：用 `frame.schema.json` 校验：`type=tip` 且 `from=""` 的 `Message`、`LastMsg`；`type=text` 且 `from=""` 的 `Message`、`LastMsg`；带 `userId` 的错误 ack。
- 预期结果：tip 两条通过；text 两条失败；带 userId 的错误 ack 通过；Go 结构体编码结果按 Schema 校验通过。

### TC-SOC-29-08 提示消息推进操作人已读并多端同步
- 关联 AC：AC-SOC-29-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；bob 两个连接；carol 未读为 0。
- 步骤：bob 连接 1 邀请 dave；查看 bob 连接 2 的事件、bob 的 `convs`、carol 的 `convs`。
- 预期结果：bob 的 `readSeq` 等于该 tip 的 seq；bob 连接 2 收到 `readUpdated`；carol 的未读数 +1。

### TC-SOC-29-09 存储契约：提示消息写入失败整体回滚
- 关联 AC：AC-SOC-29-8
- 优先级：P0
- 层级：存储契约
- 前置条件：`TipSameTx`；测试构建注入"提示消息写入失败"。
- 步骤：调用 `InviteMembers(g1, [dave], tip)`。
- 预期结果：返回错误；`group_members`、`member_count`、`conv_seq` 都与调用前相同；dave 没有收到 `convAdded`。

### TC-SOC-29-10 提示消息文本只用昵称，不用备注
- 关联 AC：AC-SOC-29-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1；bob 把 dave 的备注设为"前任"；dave 昵称"D"。
- 步骤：bob 邀请 dave；成员 `sync`、`convs`。
- 预期结果：tip 的 `body.tip.text` 和 `lastMsg.text` 为"bob 邀请 D 加入了群聊"，任何应答中都不含"前任"。

---

## F-SOC-30 托管群与管理接口交接（M4）

### TC-SOC-30-01 建托管群时指定群主
- 关联 AC：AC-SOC-30-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：`POST /admin/groups {groupId: g3, name: "活动群", owner: bob, members: [bob, carol]}`；bob `groupInfo`；bob `groupUpdate {name: "新名"}`。
- 预期结果：bob `myRole=owner`、`managed=true`；改名成功；群里出现 `ownerChanged`（`from=""`、`targets=[bob]`）和 `renamed`。

### TC-SOC-30-02 收回群主
- 关联 AC：AC-SOC-30-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g3 群主 bob。
- 步骤：`PATCH /admin/groups/g3 {owner: null}`；bob `groupInfo`；bob 改名。
- 预期结果：`managed=true`、`owner=""`、bob `myRole=member`；改名 4030；群里出现 `from=""` 的 `ownerChanged`（`targets=[]`）。

### TC-SOC-30-03 指定非成员为群主
- 关联 AC：AC-SOC-30-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g3；eve 不是成员。
- 步骤：`PATCH /admin/groups/g3 {owner: eve}`；`POST /admin/groups {groupId: g10, owner: eve, members: [bob]}`。
- 预期结果：都返回 404 / 4046；g3 群主不变；g10 未创建。

### TC-SOC-30-04 管理接口对自建群不受客户端限制、不产生提示
- 关联 AC：AC-SOC-30-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：自建群 g1（`max_members=500`，现有 400 人）；u001～u200 与 alice 都不是好友；记 maxSeq。
- 步骤：管理接口给 g1 加 u001～u200。
- 预期结果：成功，600 人；g1 的 maxSeq 不变；再用客户端邀请 1 人返回 4130（自建群 `max_members` 只约束客户端）。

### TC-SOC-30-05 管理接口移除自建群群主：自动转让给最早的管理员
- 关联 AC：AC-SOC-30-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（管理员 carol、dave，carol 先成为管理员）。
- 步骤：`DELETE /admin/groups/g1/members {members: [alice]}`；`groupInfo`。
- 预期结果：`owner=carol`；生成 `ownerChanged`（`from=""`、`targets=[carol]`）；alice 收到 `convRemoved {reason: "removed"}`。

### TC-SOC-30-06 管理接口移除托管群群主：回到无群主
- 关联 AC：AC-SOC-30-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g3（群主 op1，成员 bob、carol）。
- 步骤：管理接口移除 op1；bob `groupInfo`；bob 移出 carol；bob 设 carol 为管理员。
- 预期结果：`owner=""`、`managed=true`；两个操作都 4030；没有任何成员 `myRole=owner`。

### TC-SOC-30-07 群主交接的审计
- 关联 AC：AC-SOC-30-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g3。
- 步骤：PATCH 指定 bob、更换为 carol、收回（`null`）；查询 `admin_audit`。
- 预期结果：3 条记录，含调用方、接口、`g3`、旧群主 → 新群主。
