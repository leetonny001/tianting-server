# 需求评审记录：PRD-chat-v0.1

| 项 | 内容 |
|---|---|
| 评审对象 | [docs/prd/PRD-chat-v0.1.md](../prd/PRD-chat-v0.1.md) |
| 日期 | 2026-11-06 |
| 评审人 | 四个独立 AI 子代理（未参与 PRD 编写），只读，没有改文件、没有跑测试 |
| 视角 | 后端（`R-CB-nn`）、客户端（`R-CF-nn`）、测试（`R-CQ-nn`）、安全（`R-CS-nn`） |
| 回应 | 见 PRD-chat v0.2 修订记录 |
| 拍板 | Frank |

四份结论都是不建议进入拍板，先出 v0.2。下面是原文。

---

## 后端（R-CB）

**结论：v0.1 不宜整体进入拍板。** 3 个阻塞项是协议或存储设计错误，先修完出 v0.2 再拍板。CH-23、CH-13 可以现在就答，不受这些问题影响。

只读核实了 `im-mvp/server/internal`、`protocol/`、ARCH、PRD-core、PRD-admin、PRD-social，以及 `tianting-sdk-ts`、`tianting-sdk-kotlin`、`tianting-sdk-swift` 的 store 和 client 部分（Swift 只看了 `loadedRanges`）。没有改文件，没有跑任何测试或压测。下文的 `store.ts` 指 `tianting-sdk-ts/src/store.ts`，`Store.kt` 指 Kotlin SDK 的 `Store.kt`。

共 28 条：阻塞 3、严重 13、一般 11、建议 1。

## 阻塞

**R-CB-01 `users.lower_id` 唯一索引破坏现有行为**
- 位置：第 12.1 节 L1106；F-ACC-01 规则 4；R-15；AC-ACC-07-1
- 问题：
  - 冻结规则是 userId 区分大小写。索引建上后，`Alice` 和 `alice` 不能再并存。
  - 库里已有这类数据时，建索引必然失败，迁移会整体回滚。PRD 写的"只报告不处理"做不到。
  - 运行期有两种坏结果：`UpsertUsers` 只声明了 `ON CONFLICT (user_id)`，撞到新索引会报错；`EnsureUser` 的 `ON CONFLICT DO NOTHING` 会静默不插入，`issueFor` 仍然给一个没有用户行的 userId 签 token。
  - AC-ACC-07-1 只跑 e2e，挡不住：e2e 里没搜到 TC-USER-01-16 对应的代码。
- 证据：`PRD-core-v1.4.md:781`（C-18）；`TC-USER.md:224`（TC-USER-01-16）；`sqlite.go:497-498,507-508`；`sqlite_moderation.go:97-98`；`admin.go`（`issueFor`）
- 修改：
  - `users` 不加唯一约束。
  - 用 `accounts.username_lower` 唯一，加 `lower(user_id)` 的非唯一表达式索引。
  - 注册事务内同时查两者。
  - 演示和导入路径不变。
  - 写明"只有 `/v1/account/register` 不区分大小写"。

**R-CB-02 删除会话后，离线设备的本地数据不一致**
- 位置：F-CVS-03 规则 2、6（L884、L888）；AC-CVS-04-2；第 13.1 节 L1175
- 问题：
  - `deleted` 列表"只含群，单聊不需要"这个判断是错的。
  - D1 删除单聊时 D2 离线。D2 重连后，会话从列表消失（`convs` 覆盖列表），但本地库的原文还在，也拿不到 `clearedSeq`。
  - 有新消息后会话重新出现，`minSeq=11`，三套 SDK 都不按 `minSeq` 清本地消息，旧消息继续显示。
  - 这与"我这边这段历史不再可见"和 AC-CVS-04-2 矛盾。
- 证据：`store.ts` 中 `minSeq` 只出现在 33、432、465 行（会话视图）；`Store.kt:72,389`；`client.ts`（`catchUp` 只用 `minSeq` 算补拉起点）；`LocalDatabase.kt:23,288`（本地持久化）
- 修改：
  - `deleted` 覆盖单聊和群，都带 `clearedSeq`。
  - SDK 处理 `convs` 时，对任何会话物理清除 `seq < minSeq` 的消息、引用和缓存文件。
  - 增加 AC：D2 离线，单聊删除后对方再发一条，D2 本地库搜不到旧特征串。
  - 补充：老 SDK 不认识 `deleted`，仍会显示"你已不在该群"。可以评估"群保留在 `convs` 里带 `hidden=true`"的方案，但它要占 500 上限。

**R-CB-03 缩略图的协议和存储模型自相矛盾**
- 位置：F-FILE-01 规则 1、3（L408、L410）；F-FILE-02 规则 1（L431）；F-FILE-04 规则 1（L477）；L1029；L1107
- 问题：有三种互不兼容的写法。
  - 上传响应只有 `fileId` 和 `thumbPut`，没有 `thumbFileId`，但 `send` 必须带 `thumbFileId`。
  - 下载用 `{fileId, variant: thumb}`。
  - `files` 表是单行 `object_key` 加 `thumb_key`。
  - 另外 `complete` 只核对原图：缩略图不做 HEAD、不嗅探类型，可以缺失，也可以是 HTML。第 4.3 节的附件校验同样不查缩略图。
- 修改：
  - 选定单 `fileId` 加 `variant`，从 Schema 和 AC 里删掉 `thumbFileId`。
  - 图片的 `complete` 同时校验两个对象。
  - 图片消息必须有 `thumb`。
  - `thumb` 走同样的嗅探规则。

## 严重

**R-CB-04 离线同步合并不了可变字段，引用原文会留在本地**
- 位置：F-QUO-02 规则 4（L762）；AC-QUO-02-1；F-FILE-06 规则 3；第 13.1 节 L1175
- 问题：
  - SDK 合并规则是"已有的消息只会变成已删除"。
  - 重拉即使返回新的 `quote.state` 或 `expired=true`，已在本地的消息也不会更新。
  - m1 不在已加载范围、m2 在范围内时，SDK 收不到 m1 的撤回事实，m2 的 `quote.text` 永远留在本地库。
  - 第 12.2 节承诺的"撤回内容不从引用泄露"在这条路径上不成立。AC 只验了服务端和在线事件。
- 证据：`store.ts:96-99`；`Store.kt:150-157`
- 修改：第 13.1 节写明合并规则：对已有消息覆盖 `recalled`、`deleted`、`quote`、`expired`。增加 AC：m1 不在已加载范围，离线期间被撤回，重连后 m2 的本地库搜不到特征串。

**R-CB-05 F-RCL-03 规则 7 对老 SDK 的描述不符合事实**
- 位置：L698；AC-CHAT-90-7
- 问题：
  - 这项 PRD 标了"未核实"，我核实了实际行为。
  - 老 SDK 只看 `deleted`，不看 `recalled`。已加载的消息收到"无 body、`recalled=true`"时返回 null，**原文继续显示并留在本地库**。
  - 没加载过的消息按 `body ?? {}` 入库，`type=text` 变成空内容，不会显示"[暂不支持的消息]"。
  - AC-CHAT-90-7 只要求"不崩溃"，所以测不出这个问题。
- 证据：`store.ts:96-105`；`Store.kt:154-170`
- 修改：撤回消息下行时同时带 `deleted=true` 和 `recalled=true`，语义定为"`deleted` 是无正文，`recalled` 是原因"。老 SDK 会清除原文并显示"该消息已被删除"。AC-CHAT-90-7 增加原文清除断言。`lastMsg` 同样处理。

**R-CB-06 撤回依赖一个默认关闭的开关，PRD 没写**
- 位置：F-RCL-01 规则 5；F-RCL-04 异常（L730）；第 4.1 节；AC-RCL-04-1；AC-CHAT-90-2
- 问题：
  - `IM_V13_ADMIN_FEATURES` 默认 false。关闭时隔离加密器 `cipher` 为 nil，禁言不生效，`/admin/messages/delete` 不注册。
  - 按"隔离写失败就回滚"，撤回在默认配置下要么永远失败，要么要新开一条路径。
  - F-MEN-02 规则 4（全群禁言例外）也依赖这个开关。全群禁言的规则本身与代码一致（`logic/moderation.go:205-223`）。
- 证据：`config.go:171`；`app.go:204-211,267-269`
- 修改：
  - 决定 C4 是否和 `IM_V13_ADMIN_FEATURES` 绑定。
  - 或者把隔离加密从开关里拆出来单独启用。
  - 写入第 4.1 节和 AC-CHAT-90-2。
  - C4、C5 也要明确有无上线开关（现在没有），这与 V13 的"SDK 发布后再开"先例不一致。

**R-CB-07 第 4.3 节和 F-FILE-06 存在先查后写的竞态**
- 位置：L191-197；F-MEN-02 规则 2；F-FILE-06 规则 1、2（L513-514）；AC-FILE-06-5
- 问题：
  - "文件未绑定"和"@所有人次数"都在第 10 步事务之外检查。两条不同 `clientMsgId` 的并发 `send` 可以绑定同一个文件。
  - @所有人次数在第 8 步判定，第 9 步 4510 拒绝时扣减语义未定义；并发时会超限。
  - 如果次数由 `group_mention_all` 统计，而撤回又删行，"发一次撤回一次"就绕过限额。
  - `file_purge` 和发送绑定之间也有竞态：23:59:59 绑定与 24 小时清理同时发生。状态机没有中间态，AC-FILE-06-5 要求的"记录保留、下次重试"也无处表达。
  - SQLite 单连接掩盖了这些问题，Postgres 下才会暴露。现有代码已多处标注 Postgres 要加锁。
- 证据：`sqlite.go:134,748`；`sqlite_social.go:25`（全局加锁顺序）；`sqlite.go:51`
- 修改：
  - 文件绑定用 `UPDATE files SET state='attached', msg_id=? WHERE state='ready' AND owner=?` 放进 `SaveMessage` 事务，影响 0 行则 4096。
  - @所有人计数放进同一事务，并且不因撤回而回退。
  - 增加 `purging` 状态：先在事务内把状态从 `ready` 改成 `purging`，再删对象，最后删行。
  - 第 12 节列出新事务的加锁顺序。

**R-CB-08 预签名的安全性建立在未验证的假设上，回退方案自相矛盾**
- 位置：F-FILE-01 规则 1、3；AC-FILE-01-2；AC-FILE-01-8；第 21 节 #4；CH-05、CH-06
- 问题：
  - 配额（按声明大小扣）和内容校验都依赖"对象存储强制校验签名里的大小和 SHA-256"。这一点未验证。
  - 如果厂商不支持，存在三个后果：
    - 第 21 节 #4 的回退是"complete 时读取校验"，100 MiB 过 im-server，与规则 2 和 AC-FILE-01-8 矛盾。
    - PUT 地址有效 15 分钟，`complete` 通过嗅探后可以再次覆盖对象（TOCTOU）。
    - 申报 1 字节、实传超大文件。
  - 厂商实际支持情况**未核实**，第 4 条的 fake 存储也证明不了厂商行为。
- 修改：
  - 把技术验证从"C2 第一步"提到 CH-05、CH-06 拍板之前。
  - 把必需能力写成选型条件：签名大小或 POST policy 的长度范围、校验和、CopyObject、`response-content-disposition`、CORS。
  - 设计上改为 PUT 到临时键，`complete` 校验后由服务端 Copy 到最终键再删临时键。这样无论厂商是否强校验，都能消除 TOCTOU，也不读全文。

**R-CB-09 头像公开前缀可以被当成免费托管**
- 位置：F-ACC-06 规则 4（L331）；AC-FILE-04-6；AC-ACC-06-4
- 问题：`kind=avatar` 的对象键在公开前缀下。对象在 `complete` 之前就公开可读，且没有经过嗅探。攻击者申请一次就能在你们的存储域名上托管任意内容，放弃 `complete` 也能保留 24 小时。
- 修改：一律先上传到私有前缀，`complete` 校验通过后再 Copy 到公开前缀，`users.avatar` 只用最终地址。公开前缀强制 `nosniff` 和图片 `Content-Type`。

**R-CB-10 refresh token"返回相同新 token"按现有设计无法实现**
- 位置：F-ACC-03 规则 3、4（L267-268）；AC-ACC-03-3（L279）；L1104
- 问题：
  - 服务端只存哈希（`refresh_hash`、`prev_refresh_hash`），拿不到明文，无法重发"同样的新 token"。
  - access token 重签后 `iat` 和 `exp` 也不同。
  - `device_sessions` 只保留一代前的哈希，更早的重放按"找不到"处理，不会触发吊销，查找也没有 `sid`。
- 修改：
  - token 格式定为 `sid.secret`。
  - 宽限期语义改为"再次轮换，并让上一次发出的新 token 作废"，或用 `HMAC(key, sid, generation)` 派生以便重算。
  - AC-ACC-03-3 改为"会话不被吊销，新 token 可用"。
  - 任何代际的旧 token（`sid` 匹配、secret 不匹配）都按重放处理。

**R-CB-11 断线补推有误报，而且会被批量触发**
- 位置：F-APN-02 规则 4（L562）；AC-APN-02-6
- 问题：
  - 条件是"关闭码不是 1000"，覆盖了 1001、4400、4408、4011、4429 和 1006。
  - 滚动重启时 `KickAll` 全部走 1001：每个设备都要算一次未读（单连接 SQLite 上跑会话查询），再发一条通知。
  - 前台用户网络抖动 1006、2 秒内重连，也会收到"你有新消息"，规则里没有延迟。
- 证据：`app.go:399`；`errors.json:81`；`gateway.go:472-476`；`sqlite.go:51`
- 修改：只对 1006 和 4408 触发。延迟 30 秒左右（未校准），期间同 `sid` 重连则取消。全局限速。AC 增加"断开 5 秒内重连无调用""1001 无调用"。

**R-CB-12 推送在群聊场景下的性能隐患**
- 位置：F-APN-02 规则 7（L565）；F-APN-04 规则 2；AC-APN-04-4；AC-APN-02-8
- 问题：
  - 规则只保证入队不阻塞发送，没有管 worker 的数据库负载。
  - 每个离线设备都要查令牌、免打扰、拉黑；角标还要算全量总未读（查 500 个会话的 UNION 列表）。
  - 2000 人群一条消息，可能产生上千次这种查询，全部排在消息写入共用的唯一连接上。"不影响发送路径"实际不成立，而 AC-APN-02-8 只测 1 对 1。
  - "队列 10 万条"如果按设备计，50 条 2000 人群的消息就满了。
- 证据：`sqlite.go:17-18,51,666-668`；`store/migrations/sqlite/0007_group_roles.sql`（`max_members` 默认 2000）
- 修改：
  - 队列单位定为消息，worker 用批量查询一次取齐收件人的令牌和设置。
  - 角标改用聚合查询或短 TTL 缓存，超过阈值的群消息可以不带 `badge`。
  - 增加 AC：2000 人群、半数离线时测 send P99。

**R-CB-13 `cleared_seq` 无法区分"删除过"和"没删除"**
- 位置：F-CVS-03 规则 3（L885）；L1112；AC-CVS-01-5
- 问题：
  - 没有消息的群 `max_seq=0`，且允许存在（托管群 AC-SOC-29-3 的 maxSeq 为 0；C-39）。
  - 这种群只设了免打扰，`conv_settings` 就有一行 `cleared_seq=0`。按"seq > clearedSeq 才显示"会被隐藏。
  - 删除空群（`upToSeq=0`）和这种情况无法区分。
  - 退群时规则 6 连带删除这行，重新入群且 `historyVisible=true` 时，删除过的历史又可见。这个行为没有规定。
- 证据：`sqlite.go:639-652`；`logic.go:423-430`；`PRD-core-v1.4.md:802`
- 修改：`deleted_at`（或可空 `cleared_seq`）单独表示已删除。隐藏条件写成 `deleted_at>0 AND max_seq<=cleared_seq`。明确重新入群后的语义，并加 AC。

**R-CB-14 缩略图地址没有批量接口，会撞限流**
- 位置：F-FILE-04 规则 1、6（L477、L482）；L1046；AC-FILE-02-9
- 问题：规则写"批量申请由 SDK 合并"，但接口只有单个 `/v1/files/{id}/url`，没有东西可合并。一屏 40 张图加上翻页就接近 120 次每分钟上限，图片加载会报 4290。每次请求都查文件、消息、会话权限，单连接串行。
- 修改：增加 `POST /v1/files/urls {fileIds≤50, variant}`，批量查权限，限流按批次计，逐项返回结果。写明客户端图片缓存键用 `fileId`，不用 URL。

**R-CB-15 注销的数据模型和事务范围有问题**
- 位置：F-ACC-08 规则 2、4（L371、L373）；L1103；AC-ACC-08-2
- 问题：
  - `deleted_at` 放在 `accounts` 表，但存在性判断全看 `users`，`users` 行又保留。发给已注销用户不会返回 4042，签发和演示登录也能继续给他发 token。
  - "整体回滚"意味着一次注销要在同一事务里退出全部群，并逐群转让群主、写提示消息。用户可以在 600 个群里（AC-SOC-11-2），这段时间独占唯一连接。
- 证据：`sqlite_social.go:975-978`（`Relation` 的 `PeerExists`）；`sqlite_group.go:371-394`；`sqlite.go:51`；`admin.go`（`issueFor`）
- 修改：
  - `deleted_at` 放到 `users`，同事务递增 `token_version`。
  - 列出全部入口：`Relation`、`UserExists`、`MissingUsers`、`issueFor`、演示登录、`UpsertUsers`、管理加人。
  - 分两步：事务内先标记注销并失效凭证，再由幂等后台任务逐群清理。
  - AC 改为"最终一致"。

**R-CB-16 第 15 节"不增加 B6 返工"不成立**
- 位置：L1249；L1054；F-APN-02 规则 2、3、6、7；F-ACC-02 规则 3
- 问题：
  - 第 11.4 节把"查询设备是否有活跃连接"加到 `delivery.Router`。Router 的约定是不阻塞、只投递，返回值只用于指标；ARCH 规定这类查询属于 `Presence`，而代码里还没有这个接口。
  - 集群下，最近收帧时间和后台状态都在连接所在节点的内存里，需要新设计。
  - 合并窗口和内存队列是节点本地状态。
  - 登录失败计数、IP 限流的存储第 12.1 节没有定义；现有 `Limiter` 只有令牌桶 `Allow`，且 ARCH G2 已指出多节点下配额会倍增。
  - 定时任务 `file_purge` 等在多节点上会重复运行（F-ADM-32 已要求只在一个节点运行）。
- 证据：`delivery.go:15-23`；`ARCH-v1.md:28-29,69`；`ratelimit.go:17-20`
- 能复用（不用重做）：`push.Provider`、令牌和设置存储、推送规则判断；`Invalidator` 主题；DB 方案的配额；Store 方法（前提是补上加锁规则）。
- 要重写：在线判断、合并、队列、补推定时器。
- 修改：
  - 现在就定义 `delivery.Presence`，由 hub 做本地实现，把设备活跃判断放进去，不扩 Router。
  - `KickUser` 增加按 `sid` 排除的语义时，也要说明走总线。
  - 明确写出登录失败和注册限流的存储位置。
  - 第 15 节改成"推送模块的在线判断和合并在 B6 需要重写"。

## 一般

**R-CB-17 没有 `sid` 的 token 和吊销路径有缺口**
- 位置：F-ACC-03 规则 2；F-ACC-04；F-ACC-05 规则 3；F-ACC-07 规则 2
- 问题：
  - 改密和找回只吊销设备会话。无 `sid` 的 token（演示转账号、`token.issue`）最长 30 天内继续有效。
  - 无 `sid` 的 token 调用 `password`、`logout`、`recovery-code`、`delete` 时，"当前设备"的语义未定义。
  - 连接入表后复查只看版本号（`gateway.go:165-168`），要补 `sid` 复查，否则吊销和登记之间的连接会漏掉。
  - 如果靠递增 `token_version` 兜底，离线 D2 重连时的原因会是 `banned`，而不是 `passwordChanged`（`gateway.go:312-314`）。
- 修改：改密和找回同时递增 `token_version`。给 `revokedEvent` 补原因判断。明确 `/v1/account/*` 对无 `sid` token 的行为。

**R-CB-18 演示登录可以接管已注册账号**
- 位置：F-ACC-07 规则 1；F-ACC-06 规则 6
- 问题：只要 `IM_DEMO_LOGIN=1`，输入任意用户名就能拿 token，还能改昵称，包括有密码的账号。生产环境被 S4 挡住，但预发和测试环境会串号。
- 证据：`admin.go:362-410`
- 修改：`accounts` 里有行的用户，`/demo/login` 一律拒绝（4030）。AC-ACC-07-2 的流程同步补断言。

**R-CB-19 防枚举限流只覆盖 `open` 模式**
- 位置：F-ACC-01 规则 7（L216）；PRD-social 1.3 节
- 问题：`invite` 模式下，持有一个有效邀请码的人可以用 4095 无限探测用户名，因为成功才扣码，且 IP 限流只写在 `open` 模式。这条路径也不扣 `newPeerReject`。
- 修改：IP 限流对全部模式生效，并增加按邀请码计数的失败上限。第 16 节的 PRD-social 指标同步修改。

**R-CB-20 C-29 的修改漏了新对象配额**
- 位置：第 4.3 节 L197
- 问题：
  - 第 5 步预扣 `newPeerReject`，并可能消耗 `newPeerHour`，只有保存成功才退还 `newPeerReject`。
  - 新增第 6～9 步（4048、4096、4040、4510）是非静默失败，却不退。用户对新对象发几次被拦截的内容，配额就被耗尽，收到 4292。
  - PRD-social 只定义了 4042、4035、4036 不退。
- 证据：`social.go:741-751`；`logic.go:182`；`PRD-social-v1.0.md:491`
- 修改：第 6～9 步失败时退还第 4 步的预扣，第 5 步是否退还也写明，并加 AC。

**R-CB-21 CH-24 没有堵住 `custom` 绕过**
- 位置：F-MOD-02 规则 4；第 0 节发现 ①
- 问题：`type=text` 的请求可以同时带 `body.data`。`Validate` 不拒绝，整个 body 原样落库下发；Schema 的 `MsgBody` 也没有 `additionalProperties:false`。关掉 `custom` 后这条路仍然可以携带不过敏感词的内容。新增的 `image`、`file`、`quote`、`mentions` 共用同一个结构，也有同样风险。
- 证据：`frames.go:122-126,177-185`；`logic.go:160`；`frame.schema.json:54-69`
- 修改：`Validate` 按 `type` 严格互斥字段，Schema 用 `oneOf` 或条件约束，加向量用例。
- 核实：`server/e2e` 和 sdk-ts 确实不依赖客户端发 `custom`。Kotlin 和 Swift SDK、各 App 没有查（未核实）。

**R-CB-22 通知的 collapse id 和替换通知设计不完整**
- 位置：F-APN-02 规则 6；F-APN-03 规则 1、4（L585、L588）；AC-APN-03-1
- 问题：
  - "同一会话同一 collapse id"如果直接用 `convId`，就泄露给通道，AC-APN-03-1 不通过。规则没有说明要不透明化。
  - 按会话折叠时，撤回 m1 的替换通知会覆盖更新的 m2。
  - "24 小时内推送过这条消息的设备"需要记录，第 12.1 节没有对应表，队列又在内存里。
  - FCM 的 `collapse_key` 是否能替换已展示的通知、每设备可用的 key 数量上限：**未核实**（未联网，凭记忆可能要用 `notification.tag`，C3 开工时查文档）。
- 修改：collapse id 用 `HMAC(服务端密钥, convId)`。替换通知只在"该消息是这台设备这个会话最后推送的一条"时才发。记录放内存 LRU，尽力而为并写明。

**R-CB-23 令牌和会话的生命周期有缺口**
- 位置：F-ACC-04 规则 1（L291）；F-APN-01 规则 3、4；L1054；F-MOD-02 规则 7
- 问题：
  - 断网退出后，服务端的令牌还在，这台手机会继续收通知（详情模式会显示内容），最长持续到会话过期（90 天滑动）。
  - 已入队的通知没有要求发送前复查令牌，吊销后仍可能发出。
  - 第 11.4 节只有 `file_purge`、`report_purge`、`invite_purge`。没有清理 `device_sessions`、`device_push`、`reset_codes`、`moderation_hits`（后者明确保留 90 天）的任务。
  - `show_detail` 存在令牌行上（L1109），令牌刷新后设置归属不清。
- 修改：SDK 持久化"待退出"并在下次联网时补发。worker 发送前按 `sid` 校验。补 `session_purge` 等任务。设置移到 `device_sessions`，无 `sid` 时再放令牌行。

**R-CB-24 文件摘要的来源矛盾**
- 位置：F-FILE-03 规则 5（L461）
- 问题：规则要求列表显示"[文件] 文件名"，又说 `lastMsg.text=""`、摘要不走服务端。`LastMsg` 里没有文件名，没加载过消息的会话无法显示。
- 证据：`frames.go:272-280`
- 修改：二选一：服务端把清洗后的文件名写入 `last_text`（撤回和删除时已有清空逻辑），或者只显示"[文件]"。

**R-CB-25 Argon 并发闸门可能被用来拒绝服务**
- 位置：F-ACC-01 异常（L223）；第 15 节
- 问题：
  - console-server 参数是 64 MiB、t=3（`secret.go:149-150`），同时 8 个就是 512 MiB，运行在 WS 同一进程里，`GOMEMLIMIT` 规划没有考虑。
  - 闸门写在 `/v1/account/*` 全部路径上，`token/refresh` 不需要 Argon 也会被拦，满了直接 503。
  - 匿名请求可以把正常登录和续期一起挡住。
  - im-server 的 `go.mod` 还没有直接依赖 `x/crypto`，`internal/secret` 也不能跨模块复用。
- 修改：闸门只用于计算 Argon 的路径，refresh 排除，等待队列有短超时。第 15 节写明内存预算。

**R-CB-26 协议实现上的两个细节**
- 位置：L561、L1034（`presence`）；L1032（`muted`）
- 问题：
  - `presence` 没有 `id`、没有 ack，当前 `Handle` 对无 `id` 的帧直接回 4003（`logic.go:64`），只有 `ping` 在 gateway 特判（`gateway.go:493-496`）。必须改 gateway。
  - `Conversation.muted` 表示免打扰，而同一结构已有 `mutedUntil`、`allMuted`，事件 `muted` 和错误码 4031 都表示禁言（`frames.go:321-324`）。
- 修改：写明 gateway 的改动；字段改名 `notifyOff` 或 `dnd`。

**R-CB-27 无法或难以在服务端验证的 AC**
- AC-APN-02-8：100 条样本算 P99 没有统计意义，而且没有压测机（P-12）。AC-MEN-02-3、AC-CHAT-90-1 同样要标"只作参考"。
- AC-ACC-02-3：差值 20% 的中位数比较，PRD-admin 已改成交替采样 ≤5%（`PRD-admin-v1.2.md:1352`），应统一。
- AC-FILE-01-8：用内存增长证明"不经过服务端"是弱证据，应断言 PUT 目标主机是对象存储，且假存储收到全部字节。
- AC-FILE-01-2：假存储证明不了真实厂商，必须在 MinIO 和目标厂商上各跑一次。
- AC-RCL-01-6：查原始字节时要规定是否先做检查点，WAL 帧在检查点前保留（`sqlite.go:45`）。
- [IM 钟] 系列（AC-FILE-06-*）：SigV4 必须用真实时钟，否则推进时钟后签名失效。PRD 只在 AC-FILE-04-4 里提到真实时钟。
- F-MOD-02 的"检查超时 20 ms 放行"没有任何 AC；Go 里也无法打断正在跑的匹配，要么另起协程。AC-CHAT-90-4 要求 ≤2 ms，建议删掉这条超时。

## 工作量

**R-CB-28 工作量有失真（一般，判断，未校准）**
- 位置：第 18 节
- 问题：
  - 138.5 人日没有文档列，而第 16 节要改 7 份冻结文档和 `protocol/`。PRD-social 同规模算了文档 3.5 人日（`PRD-social-v1.0.md:1426`）。
  - C1 服务端 8 人日要包含新 HTTP 层、注册、登录、轮换、找回、邀请码、重置码、资料和注销。PRD-admin 里只算登录加锁定加会话就约 5.5 人日（`PRD-admin-v1.2.md:1322`）。
  - C3 服务端 8 人日要手写 APNs、FCM 鉴权和队列（不引 SDK），外加在线判断、补推、合并和角标。
  - SDK 的 C2、C4 平均每套 2 人日，要改本地库、缓存清除和合并规则（R-CB-04）。
  - 测试 20 人日按 249 条 AC 算，比 PRD-social 的比例不低（M3 约 0.05 人日每条），不算失真。
- 修改：加文档列。C1、C3 服务端和 C2、C4 SDK 上调。比例不给假精确，C1 结束后校准。

## 建议

**R-CB-29 用一个版本号替代全量重拉**
- 位置：F-RCL-03 规则 2；R-7（L1341）
- 核实结果：
  - 跨冷启动的范围是保留的：Kotlin `restore` 恢复每会话最多 200 条（`IMClient.kt:825-843`，`LocalDatabase.kt:23`），`loadedRanges` 不过滤（`Store.kt:476-477`）；Swift 用 `touched` 和 `restoredUnverified` 过滤（`Store.swift:474-477`）。
  - 所以"冷启动漏撤回"不是问题，AC-RCL-03-4 可行。代价是：Kotlin 每次重连都按已加载范围逐会话重拉。
  - 引用组装和文件过期计算还会让每次 `sync` 更重。
- 修改：`conv_seq` 加 `mod_ver`，撤回、删除、文件过期时递增，`convs` 返回。SDK 只重拉变化的会话。这个改动小，现在做比出问题后再加命令便宜。

## 对 CH 项的意见（只列有错或有遗漏的）

- **CH-02（遗漏）**：没写这是不可逆决定。userId 已写进 `convId`（`c2c:a:b`，`frames.go:414-419`）、`from_uid`、好友和群表，还会进日志（`gateway.go:171`）。以后想改名或拆分登录名，就是全库迁移，不是备选方案说的"多约 3 人日"。建议在"影响"栏写明，让 Frank 按不可逆决定来选。
- **CH-05、CH-06（遗漏）**：要补 R-CB-08 的选型必需能力，并且先做验证再拍板。
- **CH-22（遗漏选项）**：F-MOD-03 规则 5 在 `IM_C2C_BODY_ACCESS` 关闭时仍然存单聊正文快照 180 天。开关一旦打开，开关之前的举报也变得可读。缺一个选项："开关关闭时单聊证据只存元数据"，代价是之后撤回会丢证据。这个要和法务一起问。
- **CH-24（方案不完整）**：见 R-CB-21。

其余 CH 项没有意见。

## 已核实没有问题（不用重复查）

- 错误码新号与 `errors.json` 不冲突。
- 全群禁言的群主、管理员例外与代码一致。
- 第 4.3 节的去重、禁言、限流顺序与 `logic.go` 一致。
- 群主自动转让规则与 `afterOwnerGone` 一致。
- 双节点 AC（AC-CVS-04-3、AC-MOD-01-3）可以执行：测试模式有 `HTTPLoopback` 和 `TestPeers`（`app.go:171-174`）。
- `report.evidence` 按正文类 scope 配置可行，但要加进 `IsBodyScope`（`scope.go:48-50`）。

## 未核实

- 各云厂商和 MinIO 对预签名大小、校验和的支持。
- FCM 和 APNs 官方细节。
- Kotlin、Swift SDK 和各 App 对 `custom` 的依赖。
- 是否有存量生产数据和老版本客户端在用。这决定 R-CB-01 和 R-CB-05 的实际影响，但不影响修改方向。

---

## 客户端（R-CF）

# PRD-chat v0.1 独立评审：客户端、SDK、交互

**结论：不建议现在进入拍板，也不能冻结。** 先出 v0.2，修 3 条阻塞、补 CH-06 和 CH-11 的选项、重估第 18 节工作量。这些都是文档层面的修改，产品方向不用重做。CH-13、CH-23 不受影响，Frank 现在就可以回答。

- **统计：** 阻塞 3、严重 12、一般 7、建议 2。
- **范围：** 只读，没有改任何文件，没有启动模拟器。
- **证据路径：** 相对于 `/Users/frank/Desktop/workspace/`。
- **没有运行：** 所有结论来自读代码和文档，没有跑过程序。
- **联网核实：** 只查了 FCM 官方文档和 MDN。其他平台行为标"未核实"。

## 阻塞

**R-CF-01　F-FILE-02 规则 7（第 437 行）、F-FILE-01 异常和规则 5：上传没有和发送队列组成完整的状态机**
- **问题：** PRD 只写了"沿用 F-MSG-03，增加上传中"。以下情况都没有定义：
  1. 冷启动。未发送的消息会落库并在重连后自动发出，正在上传的消息还没有 `fileId`，恢复后走哪条路径未定义。
  2. 源文件失效。App 被杀后，content URI、临时文件或 H5 的 File 对象可能已经不可用，手动重发没有数据来源。
  3. 上传成功但 `send` 失败时，要复用 `fileId`，还是重传。规则只写了上传失败要重传。
  4. 顺序。先发图片、后发文字，文字会先拿到 seq 并排到图片前面。多张图的并发和顺序也没写。
  5. 配额。额度按声明大小扣，放弃的不退，又没有断点续传。弱网下重试 100 MiB 文件 20 次，当天额度就用完（4292）。
- **证据：**
  - `tianting-sdk-kotlin/sdk-core/.../LocalDatabase.kt:288` 附近：无 seq 的消息全部保留。
  - `IMClient.kt:1325-1328`：QUEUED 消息自动 dispatch。
  - `Store.kt:17`：只有 QUEUED、SENDING、SENT、FAILED 四种状态。
  - `docs/sdk/README.md` 第 3 节：未确认的消息排在所有有 seq 的消息之后。
- **建议：**
  - 增加一节"附件发送状态机"：处理中 → 上传中 → 待 send → 发送中 → 已发送/失败。
  - 冷启动时，处理中和上传中一律转为失败，且不自动重发。
  - 发送前把源文件复制到 App 沙盒；H5 刷新后源文件丢失时，只能删除该消息。
  - 上传完成后 `send` 失败的，复用同一 `fileId`。
  - 同一会话按选择顺序串行 `send`，并写明文字可以越过上传中的图片。
  - 额度改为 `complete` 时才正式扣减，或失败退回。
  - 补三端 AC。

**R-CF-02　F-FILE-01 规则 1（第 408 行）与 F-FILE-02 规则 1（第 431 行）、11.1、12.1：缩略图的协议互相矛盾**
- **问题：** 上传接口只返回一个 `fileId` 加 `thumbPut`，`files` 表是一行带 `thumb_key`，取地址用 `variant: thumb`。但 `send` 和下发的 `image` 都要求 `thumbFileId`，AC-FILE-02-3 也按单文件校验。`thumbFileId` 没有来源，缩略图的 `complete`、嗅探也没有定义。
- **证据：** PRD 上述行。
- **建议：** 删除 `thumbFileId`，统一成"一个 fileId 两个 variant"。同时写明 `complete` 要校验缩略图，下发的 `image` 也不再带这个字段。

**R-CF-03　F-MEN-01 规则 4（第 782 行）："插入 `@显示名 `"会泄露私人备注**
- **问题：** 三端现有的显示名优先取好友备注。按规则直接插入文本，备注（如"老板"）会发给全群并保存到服务端。这违反已冻结的"备注只有自己可见"，也可能触发敏感词。现有 AC 都测不出来。
- **证据：**
  - `tianting-sdk-ts/src/client.ts` `displayName()`：备注 → 昵称 → userId。
  - PRD-social 第 260、395 行。
  - PRD-ui F-UI-04 规则 5。
- **建议：** 明确文本里插入对方的公开昵称，没有昵称时用 userId，绝不用备注。增加 AC：给成员设备注后 @ 他，服务端正文搜不到备注。选择列表里可以显示备注，只影响本机。

## 严重

**R-CF-04　F-RCL-03 规则 7（第 698 行）、AC-CHAT-90-7（第 1259 行）：PRD 对老版本 SDK 的假设不成立（已核实，可以关闭第 21 节第 6 项的后半部分）**
- **问题：** 老客户端不会把撤回消息显示成"[暂不支持的消息]"。
  - 已加载的消息：`addServer` 只在 `deleted` 时才更新，**原文一直留着**。
  - 新拉到的撤回消息：`type=text`、`body` 为空，显示**空气泡**。
  - 会话摘要：显示空字符串。
  - 所以 AC-CHAT-90-7 对撤回消息过不了。
- **证据：**
  - `Store.kt:155-158`、`store.ts:96-98`（Swift 的 Store 未逐行读）。
  - `ChatPage.tsx:551`、`ChatPage.kt:590`、`ChatPage.swift:557`。
  - `ChatsPage.tsx:165`、`ChatsPage.kt:184`、`ChatLogic.swift:83`。
- **建议：** 增加一个待拍板项，二选一：
  - A：线上帧同时带 `deleted=true` 和 `recalled=true`，并同时发 `msgDeleted`。老客户端显示"该消息已被删除"并清除原文，新 SDK 以 `recalled` 为准。
  - B：确认没有要兼容的已发布版本（Android `versionCode=1`，是否有生产用户未核实），删除这部分 AC。
  - 也可以在 `/v1/config` 增加 `minClientVersion`。

**R-CF-05　F-QUO-02 规则 4（第 762 行）、F-RCL-03 规则 2：离线错过撤回时，原文会从引用里漏出来**
- **问题：** PRD 说复用 `recheckDeleted`，并靠 `sync` 返回新状态。但 Store 对已知 `msgId` 除 `deleted` 外一律忽略，所以 m2 的 `quote.state`、图片的 `expired`、`recalled` 这些变化都写不进去。AC-QUO-02-2 只测了在线收到事件的情况。这违反第 12.2 节"撤回内容从引用泄露"的对策。
- **证据：** `Store.kt:155-158`；`IMClient.kt` 中 `recheckDeleted`（第 919 行附近）。
- **建议：**
  - 规定 SDK 的 Store 对已知消息按服务端字段更新 `recalled`、`deleted`、`expired`、`quote`。
  - 用同一份向量验证。
  - 增加 AC：u2 离线时 m1 被撤回，u2 重连后内存和本地库搜不到 m2 引用里的原文。

**R-CF-06　F-APN-02 规则 3（第 561 行）：重连后的新连接默认算"活跃"，后台通知会丢**
- **问题：** `presence` 只在前后台切换时上报。原生端进入后台不断开，网络恢复时还会立即重连。Android 在后台切一次网络，新连接没有 `background` 状态，靠 ping 一直保持活跃，服务端就一直不推，而这正是 C3 的核心场景。另外 iOS SDK 没有 `beginBackgroundTask`，进入后台时那一帧是否发得出去没有保证（未实测）。
- **证据：**
  - `AppModel.kt:158-161`：`setForeground` 只调用 `visibilityChanged`。
  - `SystemEvents.swift:48-53`。
  - `client.ts:406` 附近：`visibilityChanged` 只处理已读和禁言，`networkOnline` 立即重连。
- **建议：** 规定 `auth` 成功后的第一帧必须上报当前状态，服务端在收到之前按 background 处理。增加 AC：后台重连后对方发消息，假通道有调用。

**R-CF-07　F-APN-02 规则 6（第 564 行）、F-APN-03 规则 4、AC-APN-02-7、AC-APN-03-3：按会话用 FCM `collapse_key` 的方案不成立**
- **问题：**
  - FCM 官方文档写明每台设备同时只保存 4 个不同的 collapse key，通知消息还会忽略 `collapse_key`。通知栏替换要另找机制（Android 的通知 tag，官方页面未提及，未核实）。
  - 撤回的替换通知用的是会话级 collapse id，会把该会话**更新的另一条**通知也盖掉。
  - collapse id 怎么生成没写。明文 `convId` 会把会话关系交给苹果、谷歌。AC-APN-03-1 只抓请求体，也测不到 APNs 的请求头。
  - 替换通知要知道"24 小时内推过哪些设备"，第 12.1 节没有对应的表。
- **证据：** firebase.google.com/docs/cloud-messaging/customize-messages/collapsible-message-types；PRD 第 12.1 节 `device_push`。
- **建议：**
  - 分通道写：APNs 用 `apns-collapse-id`，FCM 用 tag。
  - id 用每设备密钥做的 HMAC。
  - AC 扩展到请求头。
  - 替换通知要么降级为可选，要么补存储设计。

**R-CF-08　F-ACC-04 规则 1（第 291 行）、F-APN-01 规则 4：断网退出后，设备还会继续收到通知**
- **问题：** 网络不通时只做本地退出，服务端的 `sid` 和推送令牌要等到期（90 天）。这台设备会继续收到已退出账号的通知，详情模式下还包含内容。另外，用户后来在系统设置里开或关通知，第 F-APN-01 节规则 5 只写了首次授权，没有定义何时补注册，也没有 AC。Android 13 以下没有运行时权限，但用户同样可能关闭通知。`pushRegister` 只能走 WS，后台收到令牌刷新时发不出去。
- **证据：** PRD 上述行；`tianting-android/app/src/main/AndroidManifest.xml` 目前没有 `POST_NOTIFICATIONS`，只有两条网络权限。
- **建议：**
  - 退出时在本地删除或注销令牌，并把 logout 排队到下次联网。
  - 每次回到前台对比系统授权状态和令牌，变化就补注册或注销。
  - 令牌刷新先缓存，连上后再注册。
  - 补 AC，并覆盖 API 33 以下的设备。

**R-CF-09　F-ACC-03 规则 4、5、7（第 268～271 行）：续期的失败路径把网络问题当成登录失效，轮换还会误吊销**
- **问题：**
  1. "刷新失败才回登录页"没有区分类型。离线冷启动且 access token 已过期时，用户会被踢回登录页。AC-ACC-03-8 只测了已吊销。
  2. 响应丢失或进程被杀后超过 30 秒才重试，会按重放吊销会话。
  3. H5 从当前页打开的窗口会复制 `sessionStorage`（MDN 的 opener 规则），复制标签页很可能也一样（未核实）。两个标签页共用一个 refresh token，一边轮换后另一边就可能触发重放，两边都被踢。
  4. H5 关闭标签页不等于退出，`device_sessions` 会不断累积，用户详情页的"设备会话数"失真。
- **证据：** `tianting-web/src/lib/session.ts`；`SessionStore.kt`、`SessionStore.swift` 都只存 `{userId, token}`；MDN `Window.sessionStorage`。
- **建议：**
  - 只有 4019 才回登录页；403/4034 走封禁文案；网络错误或 5xx 留在本地缓存页面并退避重试。
  - 补离线冷启动 AC。
  - 新 token 必须先落盘再使用。
  - H5 用 `BroadcastChannel` 或启动时检测副本，副本重新登录。
  - 定义 H5 会话的清理方式。

**R-CF-10　F-ACC-01 规则 8、AC-ACC-01-12（第 217、237 行）、F-ACC-05：恢复码流程会丢码**
- **问题：**
  - 注册成功但响应丢失，重试得到 4095，用户能用密码登录，但永远没见过恢复码，也没有任何提示。
  - 在"保存恢复码"页杀掉 App 后，恢复码丢失，是否已登录也没定义。
  - AC 里同时写了"按钮"和"勾选"。
  - 复制、截屏策略没写。
  - 恢复码丢失时"走客服"，但客户端没有入口，`/v1/config` 也没有联系方式。
  - 找回成功后，其他设备收到哪个 `kicked` 原因、显示什么文案，也没写。
- **建议：**
  - `accounts` 增加 `recovery_acked_at`，用户确认后才写入。未确认时登录，强制重新生成并进入保存页。
  - 确认前不持久化 token。
  - 写清楚交互，并增加客服入口配置。

**R-CF-11　F-ACC-07 规则 4、5（第 350～354 行）：从演示模式到账号的改造，只考虑了 H5**
- **问题：**
  1. 三端都有构建开关，而 PRD 只提到 H5 的 `VITE_DEMO_LOGIN`。构建开关和 `/v1/config.demoLogin` 不一致时以谁为准，没写。
  2. 客户端区分不了演示会话和账号会话，因为 Session 只有 `{userId, token}`。演示用户打开"账号安全"、改密、注销时的表现未定义，也没有 AC。
  3. 老版本升级后，已存的 session 没有 refresh token，迁移行为没写。
  4. `filesEnabled=false` 时"+"按钮和已有图片消息怎么显示，已登录用户拉配置失败时怎么办，都没写。
- **证据：**
  - `tianting-android/app/build.gradle.kts:35,42`、`LoginPage.kt:92`。
  - `tianting-ios/.../Config.swift:9`、`LoginPage.swift:38-41`。
  - `tianting-web/src/lib/config.ts`、`LoginPage.tsx:21-25`。
- **建议：**
  - 构建开关作为上限，两者同时为真才显示演示入口。
  - Session 增加 `mode`，演示会话隐藏账号安全。
  - 补上述 AC。

**R-CF-12　第 4.2 节规则 6、第 13.2 节、第 16 节：H5 现有部署会拦住 C1 和 C2**
- **问题：**
  - 同源转发只有 `/ws` 和 `/demo/login`，`/v1/*` 到不了服务端（SPA 回退）。
  - CSP 的 `connect-src 'self'` 会拦截直传对象存储的 PUT。
  - `img-src` 不含 `blob:`，上传前的本地预览和下载到 blob 都会受影响。
  - 第 16 节没有列 NF-UI-14 和部署配置。
  - 开发用的 http MinIO 还会被 Android 的明文禁用拦住。
- **证据：**
  - `tianting-web/deploy/nginx.conf:10,22,31`；`vite.config.ts:15-16,39-49`。
  - PRD-ui 第 1528 行。
  - `network_security_config.xml`。
- **建议：**
  - 第 16 节增加 NF-UI-14、nginx、vite、CORS（要允许校验头）。
  - 明确对象存储域名从哪里配置进 CSP。
  - 增加 AC：生产 CSP 下上传下载没有违规。
  - 定义开发环境的 MinIO 走 https 或 debug 例外。

**R-CF-13　F-FILE-03 规则 3（第 459 行）、F-FILE-04 规则 5（第 481 行）、第 13.2 节"下载"行（第 1188 行）：下载方案和规则、AC 冲突**
- **问题：**
  - H5 用"浏览器下载"，页面拿不到进度和断网状态，做不到规则 3 的进度、AC-FILE-03-5 的失败重试、F-RCL-03 的"取消下载"，地址也会留在浏览器下载记录里。
  - Android 的 `DownloadManager` 由系统下载，地址过期后 SDK 无法重新申请，也没法按撤回取消，地址还会被系统持久化（未核实，与"不持久化预签名地址"冲突）。
  - H5 跨域时 `download` 属性无效，文件名要靠签名里的 `response-content-disposition`，PRD 没写。
- **建议：**
  - Android 改为 App 内用 OkHttp 下载到私有缓存，再用 SAF 或 FileProvider 交出。
  - H5 把这些 AC 改成 Android、iOS 专属，并单独写 H5 的降级验收。
  - 写明签名里要带文件名。

**R-CF-14　F-FILE-02 规则 2、4（第 432、434 行）、AC-FILE-02-10：图片处理只想到了 iOS**
- **问题：**
  - HEIC 只写了 iOS 转 JPEG。Android 用户相册也可能有 HEIC，而 minSdk 是 24（API 28 以下系统解码能力未核实）；桌面浏览器的 canvas 通常也解不了（未核实）。但 AC 要求三端都能正常。
  - 规则 4 只豁免 GIF，动图 WebP 和 APNG 重编码后会丢动画。
  - PNG 透明通道的输出格式、EXIF 方向、`width`/`height` 的取值时机都没写。
  - 发送端解码失败或内存不足的状态没写。
  - 20 MiB 按原文件还是重编码后的大小算，也没写。
  - 接收端看 GIF：Android 只有 `coil-compose` 没有 GIF 解码，iOS 现在只有 `AsyncImage`，都不能播放动图（未实测）。
- **证据：** `tianting-android/gradle/libs.versions.toml:22`；`app/build.gradle.kts:24`；`tianting-ios/.../Avatar.swift:23`。
- **建议：**
  - 增加一张"输入格式 → 输出格式 → 失败处理"的三端矩阵。
  - 写明压缩后计算大小。
  - 决定 GIF 播放是否进第一批，并把依赖计入工作量。

**R-CF-15　F-FILE-04 规则 1、6（第 477、482 行）：缩略图逐张申请地址，会触发限流，也用不上缓存**
- **问题：** 接口只能一次申请一个文件，规则却要求"批量申请由 SDK 合并"，每人每分钟只有 120 次。图片多的群滚动几屏就可能 4290。地址每次都变，按 URL 做 key 的缓存（Coil、URLCache、浏览器）都命不中，H5 只有内存缓存，刷新后全部重来。
- **建议：**
  - 增加批量接口 `POST /v1/files/urls`（如每次 50 个）。
  - SDK 的缓存 key 必须是 `fileId + variant`，不能是 URL。
  - 增加 AC：连续滚动 200 张图没有 4290。

## 一般

**R-CF-16　F-MEN-01 规则 5（第 783 行）：@ 输入交互没写到能实现的程度**
- **问题：**
  - 规则只写了"删除这段文字时一起移除"，没写是一次退格删除整段，还是只要改动一个字就失效。
  - 组字状态：组字中输入 `@` 是否打开选择，全角 `＠`，`a@b.com` 中间的 `@` 是否触发，都没写。
  - 同名成员，粘贴带 `@` 的文字，都没写。
  - 草稿只存字符串，恢复后 `mentions` 和引用条都会丢，用户看着有 `@` 却不会提醒。
  - "可搜索"做不到：`groupMembers` 只接受 `groupId`、`cursor`、`limit`，且禁止其他字段；第 11.1 节也没有新增。2000 人群只能翻完全部成员再本地搜。
- **证据：**
  - `Composer.tsx:97-110`：textarea，无法放原子节点。
  - `ChatPage.kt:666`：`TextFieldValue`。
  - `Composer.swift:39`：`UITextView`，这一端最好做。
  - `ImSession.kt:95`：草稿是 String。
  - `frame.schema.json` `GroupMembersReq`。
- **建议：**
  - 规定用"文本 + `[{start,len,userId}]`"的中间模型。
  - 组字结束前不改动模型；光标不能落进 mention 内；退格一次删除整段。
  - 触发条件为行首或空白后的 `@` 或 `＠`。
  - 草稿改成结构化。
  - 增加 `groupMembers.keyword` 或明确"只搜已加载"。
  - 补（手工）输入法 AC，并引用 AC-UI-10-2 的写法。

**R-CF-17　F-QUO-02 规则 5、F-MEN-03 规则 3：跳转到不在已加载范围的消息，SDK 没有接口**
- **问题：** Store 假设已加载范围是连续的，靠 `oldestSeq` 翻页，`sync` 每次最多 200 条，第 13.1 节也没有"跳转到 seq"。如果留下空洞，翻页和 `loadedRanges` 都会出错。
- **证据：** `IMClient.kt` `recheckDeleted`；`Store.kt:476-477`；`maxSyncLimit=200`（`frames.go:62`）。
- **建议：** 增加 `loadAround(convId, seq)` 并定义不连续区间的处理，或者第一批只支持距离已加载范围 N 条以内的跳转，超出提示"请翻页查看"。

**R-CF-18　F-MEN-03 规则 1（第 815 行）、F-RCL-03 规则 1：`mentionUnread` 的本地状态不够推算**
- **问题：** `msgRecalled` 不说明是否提及了我，未加载的消息也无法判断。另一端把已读推进到中间位置时，本地只有总数和 `firstMentionSeq`，也算不出来。"[有人@我]"可能一直留着，要等下次 `convs`。相关 AC 全是【后端】。
- **建议：** 事件带 `mentionUnread`、`firstMentionSeq` 的新值，或规定收到这些事件后节流刷新 `convs`。增加三套 SDK 的 AC。

**R-CF-19　F-CVS-03 规则 6（第 888 行）、AC-CVS-04-2：其他设备离线时删除会话，本地不一定清除**
- **问题：** `deleted` 只含群。单聊消失后本地消息怎么清除没写。会话重新出现时 `minSeq` 变大，但现有 SDK 只在补拉起点和翻页判断时用 `minSeq`，不会清掉更小的本地消息，所以 D2 还能看到已删除的历史。
- **证据：** `Store.kt:320-341`；`minSeq` 只出现在 `Store.kt:72,389`、`IMClient.kt:394,941`。
- **建议：** 规定 SDK 在 `convs` 里发现 `minSeq` 增大时，清除内存、本地库、缓存文件中更小的消息；单聊从列表消失时同样清除。增加 AC。

**R-CF-20　F-CVS-02 规则 4、5（第 868、869 行）、F-APN-04 规则 2：免打扰的角标和未读有缺口**
- **问题：**
  - 免打扰会话里 @ 我会推送，但总未读和 iOS 角标都不算，出现"有通知、无角标"。
  - "[N 条]"和"[有人@我]"、"[草稿]"同时存在时的优先级没写全。
  - PRD-ui 第 245 行写着 App 图标角标"后续"，第 16 节没有列这一处。
  - 回到前台刷新角标要有 `.badge` 授权，F-APN-01 没提。
- **建议：**
  - 在 CH-14 的影响里补上选择：`mentionUnread` 是否计入总未读（我建议计入）。
  - 增加摘要优先级表。
  - 把第 245 行加入第 16 节。

**R-CF-21　F-MOD-03 规则 8（第 983 行）与第 2.3 节（第 98 行）：范围矛盾**
- **问题：** 举报要"多选到 20 条"，而"多选"被列为后续批。现有界面没有多选模式，C6 的客户端工作量每端只有 1 人日。
- **建议：** 第一批改为只举报单条，协议保留数组；或者把多选明确放进范围并加工作量。

**R-CF-22　第 4 至 10 节：缺失的状态、无障碍和文案**
- **缺失状态：**
  - 注册、找回页的 4004、4000、4030、4290、5030 没有前端 AC，只有 4095。
  - @ 选择的加载中、无结果、空。
  - 图片保存权限被拒（API 29 以下要存储权限，未核实）。
  - 通知设置页"令牌获取中/失败"；没有令牌时 `pushSettings` 写到哪里；客户端如何读回 `showDetail`（命令 ack 是 `{}`）。
  - `filesEnabled` 关闭。
  - 第 13.2 节没有"选文件"行，后台上传只写了 iOS。Android 和 H5 切到后台时的行为、整文件 SHA-256 的"处理中"状态也都没写。浏览器 WebCrypto 对 100 MiB 只能一次性读入内存（未实测）。
- **无障碍和深色：** PRD-chat 没有出现这些词。NF-UI-7 的按钮清单、NF-UI-11 的播报内容（图片读什么）、长按菜单的辅助操作、图片替代文本、新增红色提示和占位色的深色 token 都要补。H5 的菜单只有 pointer 和 contextmenu（`ChatPage.tsx:535-549`），没有键盘入口。
- **文案：** 管理员自己撤回别人消息时看到什么、`quote.state=expired` 显示什么（图片或文件），都没写。
- **建议：** 增加一张"页面 × 状态"表和一条 AC-CHAT-90-9（无障碍、深色）。

## AC 表述问题（并入 R-CF-22 修改）

| AC | 问题 |
|---|---|
| AC-FILE-03-5 | H5 做不到（见 R-CF-13），应标 Android、iOS |
| AC-FILE-02-10 | H5 的 HEIC 路径没有规则（见 R-CF-14） |
| AC-FILE-02-9 | 只验 H5，原生端懒加载没有验收 |
| AC-CVS-01-6 | "宽屏右键"只适用 H5，应拆开写 |
| AC-APN-01-5 | 要写明 Android 13+；13 以下用系统开关验证 |
| AC-ACC-01-12 | "按钮"和"勾选"矛盾 |
| AC-FILE-04-4 | 标了 [IM 钟]，内容却是真实时间 |
| AC-RCL-01-8 | 没说按设备时钟还是服务端时间；管理员撤回他人消息的菜单完全没有前端 AC。消息不带发送者角色，现有 `myRole` 只知道自己的角色，要规定显示规则 |

## 严重（工作量和向量）

**R-CF-23　第 13.1 节（第 1163 行）、第 11.1 节（第 1037 行）：三套 SDK"跑同一批向量"目前只覆盖语法**
- **问题：**
  - `frames.json` 只有帧格式。sdk-ts 甚至只验证"下行帧不抛异常"，不做 Schema 校验；Kotlin、Swift 才校验。
  - 撤回合并、引用清除、提及计数、置顶排序、`deleted` 判断、总未读、token 续期、上传状态机，都没有共享的行为用例。三套 SDK 最容易在这里出现差异。
  - `/v1/*` 没有 Schema，`protocol/schema` 只有 `frame.schema.json`。FakeTransport 也只有 WS。
  - `docs/sdk/README.md` 已过期：写着 Kotlin、Swift"待实现"，Store"只在内存"，frames 用例"待补"。
  - 现状还有差异：Swift 注释说恢复的会话每次启动只复查一次，Kotlin 代码每次重连都复查已加载范围（第 19 节 R-7）。
- **证据：**
  - `tianting-sdk-ts/test/moderation.test.ts:632-649`。
  - `VectorsTest.kt:107-154`；`VectorTests.swift:86-122`。
  - `Client.swift:506-509`。
- **已有先例：** `push-sync.json` 有 71 个步骤式用例，声明四套 SDK 都跑；`social.json` 的期望值由 Go 参考客户端生成。
- **建议：**
  - 新增 `chat-sync.json`、`account-session.json`，再加 HTTP Schema。
  - 补 SDK 用的假 HTTP。
  - 把覆盖要求写进 AC，并计入工作量。
  - 同步更新 README。
  - 第 21 节第 6 项的前半部分可以关闭：原生端恢复的消息算已加载范围（`Store.kt:476-477`、`restore()`），每个会话最多 200 条。

**R-CF-24　第 18 节（第 1317 行）：SDK 和客户端工作量明显偏低，会误导拍板**
- **问题：**
  - SDK 合计 24 人日，等于每套 8 人日，要覆盖 HTTP 客户端、token 轮换、上传管线、撤回、引用、提及、会话设置、举报。
  - 对照：PRD-social 记原生 SDK 各 6 人日；PRD-admin 第二批 Kotlin、Swift 各 3 人日，而那只是删除、禁言和通知。
  - C2 每套 SDK 只有 2 人日、每端只有 3 人日，要做选择、重编码、缩略图、进度、取消、全屏查看、下载、缓存。
  - C1 每端 2 人日，要做登录、注册、恢复码、找回、改密、重新生成、注销、资料、头像裁剪。
  - C6 每套 SDK 0.5 人日、每端 1 人日。
  - 全文 249 条 AC，其中 78 条不是纯后端，测试只有 20 人日；PRD-social 的 M3 125 条 AC 记 9 人日，且这里还要三端各验一遍。
  - 没有计入：共享向量、本地库升级、部署和 CSP、Firebase 和 entitlement 接入（工程里还没有 `.entitlements`）、图片依赖。
  - `sendImage` 的图片处理放在哪里也没定：`sdk-core` 是纯 JVM，不能用 Bitmap；sdk-ts 的测试跑在没有 canvas 的 Node 环境。这也会影响估算。
- **建议：** 按功能和端重新拆分。我的判断是 SDK 和客户端合计（现为 60 人日）很可能要接近翻倍（未校准）。同时明确图片处理由 App 层或平台适配层负责，SDK 只接收处理好的结果。

## 建议

**R-CF-25　第 13.2 节第 1183 行、F-ACC-03 规则 7：H5"关标签页就要重新登录"没有作为产品决定列出**
- **问题：** 这条规则来自演示时期的 D4。现在是独立 IM，用户每次都要输密码，这会影响 H5 的定位。
- **建议：** 增加一个待拍板项，或在 CH-04 的影响里写明。

**R-CF-26　第 13.1 节：SDK 接口命名**
- **问题：** 新增的 `sendText` 和现有的 `send(convId, text)`（README 第 2 节）关系没写。`registerPush` 和现有系统通知的 `push` 模块（`push.ts`、`Push.kt`）容易混淆。
- **建议：** 写明旧接口的去留；离线推送统一用 `deviceNotification` 或类似前缀。

## 待拍板项

只提建议方案有问题或漏选项的：

- **CH-06：** 影响栏要补 CSP、CORS 和批量取地址（R-CF-12、R-CF-15）。否则"预签名"看起来只有 5 分钟收不回这一个代价。
- **CH-11：** 漏了一个选项："隐藏推送 + 端上取内容"。
  - 做法：APNs `mutable-content` 配 iOS Notification Service Extension，Android 用 data message 由 App 取内容再显示。这样既能显示内容，正文又不经过苹果、谷歌。
  - 代价：多一个 iOS target、App Group，Keychain 要加 access group（现有 `KeychainSessionStore` 没有，工程也没有 entitlements），还要新增一个 HTTP 读消息接口（现在只有 WS），且 NSE 的成功率不保证。
  - 我不建议放进第一批，但应该列出来。第一批的现有方案不依赖 NSE，这一点也要写明。
- **CH-14：** 建议方案本身没问题，但要补"@ 我是否计入总未读和角标"（R-CF-20）。
- **遗漏的待拍板项：** 老版本兼容口径（R-CF-04）。

## 核对中确认无误的内容

PRD 引用的以下事实和代码相符：
- `ChatPage` 三处行号
- `recheckDeleted` 的位置
- `convs` 里消失的群被当作被移出
- minSdk 24、iOS 17、两个包名
- 没有推送代码
- 第 18 节的加总

## 未核实

- 除 FCM 外的各平台行为：HEIC 解码、复制标签页、`DownloadManager` 持久化、WebCrypto 内存、APNs 细节、Android 通知 tag。
- Swift `Store` 的合并逻辑：只读了 Kotlin 和 TS，Swift 依据模型和 README 的一致性规则推断。
- 是否有已发布的老版本和生产用户。

---

## 测试（R-CQ）

# PRD-chat-v0.1 测试与验收评审（R-CQ）

**结论：不建议现在进入拍板。** 共 44 条：阻塞 2、严重 21、一般 15、建议 6。多数是一句话能改的 AC，出一版 v0.2 关掉阻塞和主要严重项，再冻结。
第 20 节的 CH 业务问题（尤其 CH-23、CH-13）不受这些问题影响，可以先问 Frank。

范围：249 条 AC 全部读过，没有运行测试，没有改文件。APNs、FCM、Gradle 插件的行为凭常识判断，没有联网核实；这几处在条目里注明"未核实"。

## 核对通过的部分

- 38 个功能点和 F-CHAT-90 都有 AC。249 条的格式都有"输入 / 操作 / 预期 / 错误码"。
- AC 引用的错误码都在 `errors.json` 或第 11.5 节里。编号和 `trace.py` 的正则兼容。
- 已有设施可以直接用：`/admin/test/clock`、`/admin/test/jobs/{name}/run`、双节点环境 `IM_TEST_PEERS`、`IM_TRUSTED_PROXIES`。
- 我原本怀疑 AC-CVS-04-3 和 AC-MOD-01-3（双节点）要等 B6。核实后现有环境能跑，不算问题。

## 阻塞（2）

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-01 | 第 4.4 节（L201）；F-APN-05；第 4.1 节 | 没有测试设施章节，而 PRD-social 5.6、PRD-admin 第 2 节都有。具体缺口：① 16 条 AC 依赖"假通道"，但没定义它在哪一层，也没有 APNs、FCM 端点覆盖配置；② 假 S3 未定义。`objstoretest` 没有预签名，所有删除都被拒，还是另一模块的 internal 包，用不了；③ AC-APN-02-5 的"关闭服务端空闲断开"和 AC-APN-02-10 的队列上限没有配置项（第 4.1 节没有，10 万条很难填满）；④ `limits/reset` 只按 userId，清不了"用户名 + IP"计数 | 新增第 4.5 节，列出：假 S3（独立进程，校验预签名、大小、SHA、过期、Content-Disposition、CORS、删除错误注入）；假 APNs / 假 FCM（HTTP 层，`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT` 只在测试模式生效）；`IM_PUSH_QUEUE_MAX`；登录限流重置接口；SDK 假时钟；Play 服务替身 |
| R-CQ-02 | L408、L431、L1029、L1107；AC-FILE-02-1、02-3 | 缩略图的模型自相矛盾。F-FILE-01 一次申请只返回一个 `fileId` 和 `thumbPut`，`files` 表用 `thumb_key`，F-FILE-04 用 `variant=thumb`；但 `send` 要带独立的 `thumbFileId`。图片消息的 TC 输入写不出来 | 二选一。建议删除 `thumbFileId`，一个 `fileId` 带 `thumb`，并写明 `complete` 同时校验两个对象。AC-FILE-02-3 增加"缩略图未上传 → 4096" |

## 严重（21）

### AC 按现在写法无法通过

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-03 | AC-ACC-01-8 ↔ L223 | 20 个并发，预期"其余 409"；但闸门同时 8 个，满了返回 503 / 5030 | 预期改为"恰好 1 个成功，其余为 4095 或 5030，邀请码只扣 1 次"。另加一条闸门 AC |
| R-CQ-04 | AC-ACC-02-3 ↔ F-ACC-02 规则 3 | 同一账号错 100 次，第 6 次起就是 4294；每 IP 每分钟超 20 次就是 4290。测不到 100 个 4018。20% 的阈值也没有固定环境 | 输入改为"100 个不同的存在账号 vs 100 个不存在账号，用不同测试 IP，串行，单机 3 轮取最差"。判定改为"差值 ≤ 20% 或 ≤ 5 ms，取较宽者" |
| R-CQ-05 | AC-ACC-03-3 ↔ L267～268、L1104 | 要求"返回相同的新 token"，但服务端只存哈希，`device_sessions` 只有 `refresh_hash`、`prev_refresh_hash`，取不回明文 | 改为"宽限期内重试返回**一对可用 token**，第一次返回的 R2 也不失效，会话不吊销"。或者加密保存新 token 并改规则 |
| R-CQ-06 | AC-FILE-03-3 ↔ L410、L503 | `.txt` 内容是 HTML，按"扩展名和真实类型矛盾 → 拒绝"和 complete 嗅探，这里就 415 了，走不到下载 | 输入改为扩展名、类型都允许但浏览器会渲染的文件（如声明 `application/octet-stream` 的 `.bin`，内容是 HTML），并明确 `text/html` 是否在禁止名单 |
| R-CQ-07 | AC-FILE-01-2 ↔ L408、第 21 节第 4 项 | 依赖"大小和 SHA 写进签名"，MinIO 和云厂商是否支持未核实；假 S3 只能证明自己的假设。规则也没定义 HEAD 大小不符时返回什么 | 分两条：真实 S3 合同测试（见 R-CQ-19）和服务端兜底（complete 时大小不符 → 删除对象、4150 或新 reason）。C2 技术验证放到冻结之前 |
| R-CQ-08 | AC-FILE-01-5 ↔ L414、L484 | "申请上传"只是签名，不访问存储；F-FILE-04 也写了"只是签名"，所以 `/uploads` 不会自然返回 5032 | 操作改为 `complete`（HEAD 失败 → 5032）。或者明确 `/uploads` 要探活，并写探活方式和超时 |
| R-CQ-09 | AC-APN-02-4 ↔ AC-APN-03-3、L559、L588 | 前者"撤回事件无调用"，后者详情模式"收到替换通知"，前者没写模式 | AC-APN-02-4 输入加"隐藏模式"；规则 1 注明"详情模式的替换通知除外" |
| R-CQ-10 | AC-APN-02-1～10、APN-03、MEN-03 ↔ L355、L539 | e2e 约定用演示 token，而无 `sid` 的连接走"按用户判断"的另一条路径。AC-APN-02-3（手机后台 + PC 活跃）在这条路径上必然失败，生产路径也测不到 | 第 7 节开头统一写"推送 AC 用账号登录的 token（有 `sid`）"，另加一条 AC 覆盖无 `sid` 分支 |
| R-CQ-11 | AC-MEN-02-3 ↔ L167 | 要算 P99，但同一群每小时只能 @ 所有人 3 次，单群采不到样本。也没写样本数、在线人数、机器 | 输入写明"`IM_MENTION_ALL_HOURLY` 调高，2000 人、在线 N 人，发 1000 次，单机、生产配置；3 轮最差"。"不写逐成员行"单独成条 |
| R-CQ-12 | AC-RCL-02-4 ↔ L694、AC-RCL-03-6 | 文案"撤回了一条成员消息"在规则里没有。"全部成员查看"包含操作者 a 自己，和"你撤回了一条消息"冲突；会话列表摘要也不一致 | F-RCL-02 增加文案表（操作者、被撤回者、其他人、列表摘要 4 行），AC 逐行断言 |
| R-CQ-13 | L461 ↔ L1031、schema `LastMsg` | "[文件] 文件名"要从消息取，但 `lastMsg.text=""`，也没有 name 字段。H5 只有内存，刷新后拿不到。这条规则没有 AC | `lastMsg` 增加 `fileName`，或改为只显示"[文件]"。补 AC：刷新后的摘要 |

### 和冻结文档的冲突

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-14 | L1106 ↔ PRD-core C-18、TC-USER-01-16（P0）；第 16 节未列 | `users.lower_id` 唯一索引会让 `alice` 和 `Alice` 不能同时导入，而这条用例要求都成功。`server/e2e` 搜不到 `Alice`，这条用例可能没有自动化，所以 AC-ACC-07-1 未必能发现 | 唯一性只放在注册路径（查询 + 账号表唯一），或者走变更改 C-18 并改 TC，二选一 |
| R-CQ-15 | 第 16 节 | 只写了 F-UI-09 规则等，以下冻结 AC 会失败但没点名：AC-UI-09-1（菜单"只有复制"）、AC-UI-15-3（"我"页没有编辑入口）、AC-UI-16-6（设置页固定 4 项）、AC-UI-09-7（`custom` 无菜单，和 F-RCL-01 规则 2 "`custom` 可撤回"冲突）。另外 AC-ADM-60-1、60-2 删除后，TC-ADM-60-01、02 会变成 `trace.py` 的 UNKNOWN，退出码 1 | 第 16 节按 AC 编号列出，写明作废或改写的内容；同步处理 TC。AC-ADM-60-1 和 AC-MOD-01-1 合并 |

### 规则矛盾、时钟、里程碑

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-16 | L274 ↔ L294 | refresh 时"已封禁 4034"，但封禁已吊销全部会话，按规则 3 应是 4019。两条都没有 AC | 定一个（建议 4019，下一次登录才得到 4034），加 AC |
| R-CQ-17 | AC-ACC-03-7、03-10；F-ACC-03 规则 5 | [IM 钟] 驱动不了客户端判断过期。AC-ACC-03-7 实际走的可能是"连接后 4011 再刷新"，规则的"先刷新再连接"验不到。AC-ACC-03-10 没标钟；"连接中到期前主动刷新"没有阈值；"1 小时内到期"的输入在 24 小时规则下启动时就刷新了 | 增加标记 [SDK 钟]（sdk-ts 已有 `Clock` 注入）。规则写明阈值。分别断言"未发 auth 就先 refresh"和"连接中到期前 N 分钟刷新" |
| R-CQ-18 | L94；AC-ACC-04-4、06-3～06-5、FILE-04-5、FILE-06-3、APN-02-4、APN-03-3、APN-04-4 | C1 的 AC 依赖 C2（头像）和 C3（令牌表）；C2 依赖 C4（撤回）；C3 依赖 C4、C5。第 2.2 节只列了三项依赖，AC 也没有里程碑标记（PRD-ui 有【M3】）。按第 5 阶段"P0 100%"，C1、C2、C3 单独都过不了 | 每条 AC 加【C1】等标记；跨里程碑的拆开（如令牌部分归 C3）；第 2.2 节补依赖 |

### 替身和覆盖

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-19 | AC-FILE-04-4、04-6、03-3；CH-05 | 假 S3 验不了桶策略、真实过期、CORS 和厂商差异，也没有 AC 要求跑真实存储 | 新增 AC-FILE-90-1：真实 MinIO 跑合同测试（CI，`minio-test.sh` 的模式）；AC-FILE-90-2（手工）：目标云厂商跑同一套，上线前执行 |
| R-CQ-20 | AC-APN-01-2、02-9、05-1、05-3；第 21 节第 2 项 | 如果假通道接在 `push.Provider` 这层，JWT 签名、HTTP/2、必需的请求头、sandbox / production 选端点、FCM OAuth、错误码映射都没被测。第 21 节承认这些错误码"按记忆写"；真实验证只有 AC-APN-05-3 的成功路径 | 假通道放到 HTTP 层并校验 ES256 JWT。新增 AC：sandbox 令牌走 sandbox 端点。AC-APN-05-3 增加一条"真实失效令牌 → 令牌被删除" |
| R-CQ-21 | L1392；Android 工程 | Android 工程现在没有 firebase 和 google-services 依赖，文件是否入库"实现时定"。常见做法下 Gradle 插件缺文件会构建失败（未核实），C3 的 Android 构建和 CI 就依赖 Frank 的 Firebase | 写明"CI 和开发用占位配置，真实配置只在发布时注入"，并加一条构建 AC |
| R-CQ-22 | 第 15 节 L1244～1248；AC-CHAT-90-1、90-2、90-3、90-4；APN-02-8；FILE-01-8 | 详见下面"性能数字"一节。三个数字没有 AC；基线和样本未定义；"增幅"判定已被 P-20 否定过一次；口径和 TC-PERF 矛盾 | 见下面"性能数字"一节 |
| R-CQ-23 | 下面"规则无 AC 清单"标★的项 | 高风险规则没有 AC，有两组。安全类：SVG 等图片类型拒绝、超管无证据权限、邀请权限、`/admin/files/{id}/url`、每 IP 20 次 / 分、`group` 举报探测、头像不可枚举。一致性类：改密回滚、隔离写失败回滚、提及表失败、词库加载失败、20 ms 超时放行、证据写失败、@ 所有人计数器不可用；还有三个 `IM_*_ENABLED` 开关关闭时的行为 | 每项至少补 1 条【后端】AC；故障注入用 R-CQ-01 的设施 |

## 一般（15）

| 编号 | 位置 | 问题 | 建议改写 |
|---|---|---|---|
| R-CQ-24 | AC-MEN-03-1 | "减少相应数量"不可判定；`firstMentionSeq` 没有断言 | 给定 seq 5、7、9，已读到 5 后 `mentionUnread=2`、`firstMentionSeq=7` |
| R-CQ-25 | AC-FILE-02-6 | "布局不跳动"不可观察 | 图片加载前后，后一条消息的 offsetTop 变化 ≤ 1 px；原生端用对应控件坐标 |
| R-CQ-26 | AC-FILE-01-7 | "去掉后的名字"没有确切值 | 写出期望字符串；补"清洗后为空 → `file`"和超 255 字节 |
| R-CQ-27 | AC-FILE-02-7 | 图片消息 `body` 不含文件名，这条必然通过。真正渲染文件名的是文件气泡和管理后台，都没有 XSS 用例 | 改为文件消息，加【前端】（管理后台）一条 |
| R-CQ-28 | AC-APN-03-1 | 只抓"请求体"，`apns-collapse-id` 在请求头（未核实），可能带 `convId` | 改为"请求头和请求体"，并断言 collapse id 不等于也不包含 `convId` |
| R-CQ-29 | AC-APN-02-2 ↔ L1247 | 负向窗口 2 秒，而入队到调用 P99 就是 2 秒，无调用证明不了什么。涉及 AC-CVS-02-1、MEN-03-2、03-3、APN-01-2 | 统一用哨兵：之后给另一个离线用户发一条，它被调用后再断言目标用户没有调用 |
| R-CQ-30 | AC-APN-02-9、L566 | "最多 3 次"是总次数还是重试次数？"两次失败后成功"分辨不出来；24 小时过期无 AC | 增加"一直 503：总调用 N 次"；断言 `apns-expiration`、FCM `ttl` |
| R-CQ-31 | AC-APN-05-2 | "到期状态"不可观察：`.p8` 通常没有到期时间（未核实） | 改为"文件存在、权限正确、最近一次调用结果" |
| R-CQ-32 | 时钟标记 | 依赖时钟未标：AC-ACC-01-2（过期邀请码）、FILE-01-4（次日）、ACC-06-5（10 分钟，要 SDK 钟）、RCL-01-1、01-5（60 秒）、RCL-01-8。标错：AC-FILE-04-4（对象存储用真实时钟）。多标：AC-ACC-05-3。另外 IM 钟推进后预签名的时间戳偏移，真实存储会拒绝（未核实） | 分别补标；AC-FILE-04-4 改为 `IM_FILE_URL_TTL=5`，等 6 秒，并断言默认 `expiresAt=now+300`；规定预签名用真实时钟 |
| R-CQ-33 | 缺错误码 | 文件名超长（L413）；`platform` 非法（L210、L243）；`note` 超 200、`reason` 非法（L976）；非群成员 `convDelete`（AC-CVS-03-9 只有单聊）；HEAD 大小不符（L410） | 逐项补码和 AC |
| R-CQ-34 | L186～197；AC-FILE-02-3 | `fileKindMismatch` 需要查记录，但 4000 只在第 1 步；@ 所有人被后面的 4510 拒绝时次数扣不扣，没写；第 4.3 节"已扣令牌"无 AC | 第 6 步加 4000；写明不保存就不扣次数；加顺序 AC（4510 后令牌减 1、次数不变） |
| R-CQ-35 | L954 ↔ AC-MOD-02-9 | "消息标记失败"和"保留在输入框"同时存在，气泡和输入框都有文字？AC 只验前者 | 定一种，AC 断言 |
| R-CQ-36 | 捆绑和层级 | AC-FILE-01-4 标【后端】但含 SDK 行为；AC-CHAT-90-5 标【后端】但扫 SDK 日志，`trace.py` 不要求 FE，且 1.3 节的"预签名地址"没进列表，"全部流程"未定义；AC-FILE-02-5 一条 3 个场景；AC-MOD-04-4 两种操作 | 拆分；AC-CHAT-90-5 列出具体流程 |
| R-CQ-37 | AC-ACC-01-12 | 又是"按钮"又是"勾选"，没有对应规则；恢复码页被杀或刷新时恢复码是否丢失未定义 | 补规则：未确认前退出时的行为；AC 覆盖 |
| R-CQ-38 | AC-CHAT-90-8 | "AC-UI-90-5 同口径"引用错了，那条查 B-2 色相。B-1 应是 AC-UI-90-15，B-3 是 AC-UI-11-3 | 改引用 |
| R-CQ-39 | L514 ↔ F-ACC-06；AC-FILE-02-4；L374；L799 | ① `kind=avatar` 不绑定消息，按状态机 24 小时后会被删，无 AC；② 去 EXIF 后竖拍照片的方向未规定，无 AC；③ F-ACC-08 的"ADMIN.md 规则"不存在，实际在 PRD-social F-SOC-23，托管群群主注销无 AC；④ `report` 在开关关闭时返回 4001，`errors.json` 的动作是"提示升级" | ① 状态机加头像分支；② AC 加 Orientation=6 的图显示方向正确；③ 改引用、补 AC；④ 换成 4030 + reason |

## 建议（6）

- **R-CQ-40**：AC-ACC-01-10、02-5 的"换 IP"写明手段，即 `IM_TRUSTED_PROXIES` + `X-Forwarded-For`。
- **R-CQ-41**：AC-MEN-02-2 写明测试时钟从 Asia/Shanghai 当天 00:00 开始，否则推 4 小时可能跨日。AC-RCL-01-2 补恰好 120 秒的边界。
- **R-CQ-42**：AC-MOD-02-2 列出具体变体。中文"违规词"没有大小写和变音，要用拉丁特征词。
- **R-CQ-43**：`trace.py` 三处局限：端到端 AC 有一个任务就算覆盖（AC-MOD-04-1 实际要 BE + CS + FE）；三端 AC 不分端；`PRDS` 的文件名写死，冻结升版时 5 个 PRD 都要改，第 16 节只提了加入本文。
- **R-CQ-44**：第 11.1 节要求 `vectors` 同步，但只有 AC-RCL-03-7 验向量。建议一条总 AC 覆盖全部新帧和新事件，三套 SDK 都跑。
- **R-CQ-44 附**：第 11.5 节"`client` scope 纳入 F-SYS-01"也应写成 AC。

## 覆盖统计

| 项 | 数字 |
|---|---|
| AC 总数 | 249（后端 171、前端 57、端到端 14、手工 7；其中 [IM 钟] 17） |
| 功能点无 AC | 0 / 39 |
| 编号规则中全部或部分没有 AC | 约 87 / 226（人工核对，误差几条） |
| 异常语句没有 AC | 约 66 / 82，约 8 成。部分价值低或不可测，不必每句都补 |

### 规则无 AC 清单（★ 高风险，建议先补）

| 功能点 | 没有 AC 的内容 |
|---|---|
| F-ACC-01 | 规则 6 昵称；异常：事务失败不扣邀请码、★并发闸门 |
| F-ACC-02 | ★规则 3 每 IP 20 次 / 分；异常：5030、哈希升级 |
| F-ACC-03 | 规则 5（不足 1 秒不提示）、★规则 6（运行中 4011 刷新一次）、规则 7（原生端存 refresh）；异常 4034 |
| F-ACC-04 | 规则 1 幂等、规则 2（计入失败次数、新密码校验）、规则 5 老 SDK；★异常：回滚、并发刷新 |
| F-ACC-05 | 异常：新密码不合规时恢复码不作废、封禁 4034 |
| F-ACC-06 | 规则 3 裁剪、★规则 4 公开读和不可猜、规则 5 旧头像 7 天、规则 6；异常 5032 |
| F-ACC-07 | 规则 5（`closed` 隐藏入口）；异常：config 失败 |
| F-ACC-08 | 规则 1 确认框（整个功能点没有前端 AC）、规则 2（好友、黑名单、哈希、令牌）、规则 3；异常：回滚、旧 token 4012 |
| F-ACC-09 | 规则 1 上限 100、★规则 2 `account.invite` 权限、规则 5 详情页；异常：5031、并发超用 |
| F-FILE-01 | 规则 1 PUT 15 分钟过期、规则 5 不退额度、规则 6 长度 |
| F-FILE-02 | 规则 1 宽高范围、★规则 2 类型白名单、规则 3 缩略图上限、规则 4（4096 缩放、GIF 不重编码）、规则 6（失败重试、全屏保存）；异常：缩略图失败、解码失败 |
| F-FILE-03 | 规则 3 打开文件、规则 5 摘要；异常：文件名为空 |
| F-FILE-04 | 规则 2 绑定前自己可访问、规则 5、规则 6（120 次 / 分，而且没有批量接口）；异常 2 句 |
| F-FILE-05 | 规则 2 `scanState`；异常：无法识别的类型（文件放行、图片拒绝） |
| F-FILE-06 | 规则 5 证据保留、规则 6 退群不删、规则 8；`pending` 超时；`RETENTION_DAYS=0`；异常 2 句 |
| F-APN-01 | ★规则 1（无 `sid` 分支、`pushUnregister`、`env`）、规则 4 注销；异常 3 句 |
| F-APN-02 | 规则 1（`custom`、`push` 帧、好友申请）、规则 3 `foreground`、规则 4 超 120 秒的负例、规则 8；异常：重试用尽 |
| F-APN-03 | 规则 2 各设备独立、规则 3 群名、规则 4（后台删除、24 小时边界）；异常 3 句 |
| F-APN-04 | 规则 1 未登录和详情 `convId`、规则 3、规则 4；异常 2 句 |
| F-APN-05 | 规则 1 选端点、规则 3 非生产和 FCM 缺密钥；异常：401 / 403、轮换 |
| F-APN-06 | 异常 2 句 |
| F-RCL-01 | 规则 2（给自己、`custom`）、规则 4 限流；异常 2 句 |
| F-RCL-02 | 规则 1（自己的消息按更宽权限）、规则 5；异常：已退群、4043、角色刚撤销 |
| F-RCL-03 | 规则 3 单聊"对方撤回"、规则 6 无菜单；异常：下载中、本地删除失败 |
| F-RCL-04 | 规则 3 元数据；★异常：隔离失败回滚 |
| F-QUO-01 | 规则 3 不复制正文、规则 4 引用链、引用图片或文件；异常 4000 |
| F-QUO-02 | 规则 1 `expired`、规则 2（截断、图片、文件）、规则 7；异常 2 句 |
| F-MEN-01 | 规则 2 去重、规则 5 分页搜索；异常 2 句 |
| F-MEN-02 | 规则 1 托管群、规则 4 全群禁言；★异常 5030 |
| F-MEN-03 | 规则 1 `firstMentionSeq`、规则 6 自己 @ 自己；★异常：提及表失败回滚、已退群 |
| F-CVS-01 | 规则 5 的 4043、规则 6 解散；异常：无字段、5000 |
| F-CVS-02 | 规则 6 不响铃；异常：读取失败按未免打扰 |
| F-CVS-03 | 规则 4（只有 tip 也重现）、规则 5 并发、规则 8 幂等；异常 3 句 |
| F-CVS-04 | `convDeleted` 事件（AC-CVS-04-1 只验置顶和免打扰）；异常：并发 |
| F-MOD-01 | 规则 6 词库为空告警；★异常：加载失败沿用旧词库、启动时放行；超 64 码点 |
| F-MOD-02 | 规则 5 已扣令牌、规则 7（90 天、后台查询）、规则 8；★异常：20 ms 超时放行、hits 写失败 |
| F-MOD-03 | 规则 1 校验、规则 2 已撤回可举报、★规则 3 `group`、规则 4 24 小时后、规则 5 文件延长、规则 8 入口；★异常：证据失败、只有元数据 |
| F-MOD-04 | 规则 3 配额合并、规则 7 指标；异常：解密失败、5031 |
| 第 4 节 | ★`IM_FILES_ENABLED` 关（404、4000）、`IM_OFFLINE_PUSH_ENABLED` 关、`IM_MOD_ENABLED` 关时 `report`；`IM_CLIENT_CUSTOM_MSG` 默认值；`/v1` 的 64 KB、超时、HTTPS、HTTP 上的 4011 / 4012 |
| 第 11、14 节 | `invite_purge`（连规则都没有）；`/admin/moderation/hits`、★`/admin/files/{id}/url`、`GET /admin/accounts/{id}`；★超管无证据权限；新指标和配置检查项（除 AC-ACC-07-3、APN-05-2） |

## 跑不了的 AC

### 开发机（无 docker、不装软件）

- **需要真实存储才算数**：AC-FILE-01-2、03-3、04-4、04-6。`minio-test.sh` 从源码构建不要 docker，但 `backend-ops.md` 的约定是本机不装 MinIO；是否允许构建到临时目录，要 Frank 明确。
- **能跑但只作参考**（第 15 节自己说明，等 P-12）：AC-CHAT-90-1、90-3、90-4，MEN-02-3，APN-02-8，FILE-01-8。
- **模拟器可以跑**：原生 UI 类 AC。
- 其余【后端】AC 在测试设施补齐后都能跑。

### CI

- **现在全部跑不了**：P-21 账单问题，2026-11-06 起所有仓库的 CI 都没有实际运行。
- **恢复后仍跑不了或不完整**：
  - Android 端：`ci.yml` 跑 ubuntu、没有模拟器步骤，且等 P-15。涉及 AC-ACC-03-7、03-8，APN-01-5、03-4、04-3、06-1，以及 41 条通用【前端】AC 的 Android 部分。
  - iOS 端：只有模拟器单测，也等 P-15。上面同一批 AC 的 iOS 部分都受影响；AC-RCL-03-2、03-4，QUO-02-2，CVS-03-5 的 Kotlin 和 Swift 部分要看 SDK 集成测试能否落地。
  - 管理后台：`tianting-admin` 的 CI 没有 playwright。涉及 AC-MOD-01-7、04-6、04-1。
  - 性能结论：AC-CHAT-90-1、90-3、90-4，MEN-02-3，APN-02-8（标准 runner 配置不够，P-12）。
- **新增工作**：真实 MinIO 的合同测试要新建 CI 任务（现有任务只跑备份）。

### 需要 Frank 真机或账号

- AC-APN-01-6、03-5、04-1、04-2（真机，APNs + Firebase 账号）。
- AC-APN-05-3（他的凭证）。
- AC-APN-06-2（没有谷歌服务的国内真机）。
- AC-FILE-02-10（真机相册）。
- 第 1.3 节的端到端彩排。
- R-CQ-19 建议的云厂商合同测试（CH-05）。

### 替身是否够

| 替身 | 判断 |
|---|---|
| 假 APNs / FCM | 对调度逻辑够用（何时推、合并、免打扰、内容隐藏）。前提是放到 HTTP 层；放在 `Provider` 层不够 |
| 假 S3 | 对访问控制、状态机、清理够用；对签名强制、桶策略、CORS、厂商差异不够，必须有真实 MinIO + 目标厂商的合同测试 |

两者都不能替代 AC-APN-05-3 这样的真实验证，而这条现在只覆盖成功路径。

## 性能数字（第 15 节，R-CQ-22）

| 数字 | AC | 测量方法 |
|---|---|---|
| 登录 P99 ≤ 500 ms | **无** | 无。Argon2 参数和闸门都未校准 |
| 申请上传 P99 ≤ 200 ms | **无** | 无 |
| 入队到调用 P99 ≤ 2 秒 | **无** | 无 |
| 文本 ack 增幅 ≤ 5 ms | AC-CHAT-90-1 | 不完整：基线版本、负载、并发都没写。"3 次取中位数"和 TC-PERF 的"3 次最差"矛盾。AC-PUSH-04-4 就是增幅判定失败后，Frank 改为绝对值（P-20）；同一报告基线 P99 25.8～25.9 ms，5 ms 约等于噪声水平 |
| 通道超时增幅 ≤ 5 ms | AC-APN-02-8 | 不足：只有 100 条，P99 基本就是最大值 |
| 依赖不可用 ≤ 2 倍 | AC-CHAT-90-3 | 不完整：对照未定义，黑洞和拒绝连接的结果不同 |
| 词库 P99 ≤ 2 ms | AC-CHAT-90-4 | 不完整：写的是"基准测试"，普通 benchmark 不出 P99，文本和词分布未写 |
| @ 所有人 P99 ≤ 200 ms | AC-MEN-02-3 | 不可执行，见 R-CQ-11 |
| 内存增长 ≤ 10 MiB | AC-FILE-01-8 | 不完整：口径未写（建议 GC 后 heap inuse，假 S3 独立进程） |
| 不回归 | AC-CHAT-90-2 | 规则写"不变差"，AC 写"全过"，两者不同 |
| 登录时间差 ≤ 20% | AC-ACC-02-3 | 不可执行，见 R-CQ-04 |

建议改法：
- 每条写明基线 commit、负载、样本数（≥ 1000）、生产配置，按 TC-PERF 取 3 次最差。
- 主判据用绝对值（沿用 P-20 的做法），增幅只记录。
- 第 15 节三个无 AC 的数字补 AC。
- 所有结论标"开发机参考，目标机重测"。

## 过关判断

- **第 1 阶段过关标准"每个功能点有可验证的 AC"**：形式上满足，但 R-CQ-02～13 的 AC 现在无法判定或无法通过，实质上没过。
- **第 2 阶段"所有阻塞关闭"**：未满足，还有 2 条。

建议顺序：
1. 产品按 R-CQ-01、02 和严重项出 v0.2。
2. C2 的预签名技术验证（第 21 节第 4 项）放到冻结前做，否则 AC-FILE-01-2 的预期可能要改。
3. 独立复审后再请 Frank 拍板冻结。

---

## 安全（R-CS）

# PRD-chat v0.1 安全与隐私评审

评审范围：只读设计评审，没有改文件，也没有做任何探测。
对照材料：ARCH-v1、PRD-social 1.3 和 11.2 节、PRD-admin F-KEY-02、F-ADM-08/13/30/31/32 和第 12 节、PRD-ui NF-UI-14、`server/internal`（`auth/token.go`、`gateway/gateway.go`、`gateway/guard.go`、`admin/admin.go`、`quarantine`、`store/sqlite.go`、`protocol/*`）、`tianting-console-server/internal/secret`。

PLAN 第 9 节实际是"演示记录"，只在 B2.5 那条复述了"安全放第一位"。安全基线以 ARCH 第 4 节为准。

**结论：现在不建议进入拍板。** 共 1 项阻塞、16 项严重、13 项一般、5 项建议。详见文末。

---

## 阻塞

**R-CS-01 客服重置码等于"签发任意用户 token"，绕过 X-7 和 F-KEY-02**
- **PRD 位置**：F-ACC-05 规则 4、F-ACC-07 规则 2、F-ACC-09 规则 4、第 14.1 节（`console` 主凭证增加 `account.reset`）、CH-01。
- **问题**：
  - PRD-admin F-KEY-02 规则 4 规定 `console` 主凭证永远不含 `token.issue` 和 `msg.body.*`。
  - 但 `account.reset` 的效果相同：生成重置码，设新密码，再以该用户身份登录并 `sync`，就能读到全部单聊和群聊历史。
  - 持有这个权限的是 `support`，而 `support` 没有任何正文权限。
  - PRD 自己承认"核实身份第一批没有技术手段"，R-1 只把它当作用户被社工的风险，没有写成内部越权和凭证被攻破的风险。
- **攻击场景**：
  - 客服账号被盗，或客服本人作恶：给目标生成重置码后登录，读取单聊。`IM_C2C_BODY_ACCESS=off`、水印、配额全部不起作用。
  - console-server 被攻破：用 `console` 凭证批量生成重置码，PRD-admin"console 被攻破也绕不过"的设计失效。
  - 外部社工：攻击者冒充用户骗客服重置，同样拿到全部历史和后续消息。
- **建议的修改**：
  1. `account.reset` 按 `token.issue` 同级处理，不进 `console` 主凭证，放到独立调用方，并在 im-server 侧强制每日配额。
  2. 双人审批（`support` 发起，另一个角色批准）。
  3. 设冷静期（如 72 小时），期间向该账号所有在线设备显示"有人申请重置密码"，并允许用已有设备取消。
  4. 重置成功后，账号标记"经客服重置"，新会话看不到重置前的历史，等用户确认后才恢复；这一条的具体规则要 Frank 定。
  5. 重写 R-1，改写 CH-01。详见文末对 CH-01 的意见。

---

## 严重

**R-CS-02 没有 `sid` 的 token 不受改密、找回、注销影响**
- **PRD 位置**：F-ACC-03 规则 1、F-ACC-04 规则 3、F-ACC-05 规则 3、F-ACC-07 规则 1～3。
- **问题**：
  - 吊销都按 `sid` 做，改密和找回没有提令牌版本号。
  - 演示、导入用户和 `token.issue` 签发的 token 没有 `sid`，最长 30 天（`auth/token.go` 只校验版本号）。
  - 封禁和强制下线走 `tv`（F-ADM-13 规则 2），不受影响。
- **攻击场景**：导入用户的旧 token 泄露。用户设了密码或做了找回，旧 token 仍能连接和读消息，并且不计入设备会话数。
- **建议的修改**：增加 `credentials_changed_at`，在 `auth` 和 `/v1/*` 校验时拒绝 `iat` 早于它的无 `sid` token。这样不用提 `tv`，AC-ACC-04-2 的"D1 不断线"不受影响。补一条 AC。

**R-CS-03 吊销会话后设备仍可能收到推送，换账号后可能继承"显示内容"设置**
- **PRD 位置**：F-APN-01 规则 1、2、4，F-ACC-03 规则 3，F-ACC-04 规则 1，F-ACC-05 规则 3，F-APN-02 规则 7、8，第 12.1 节 `device_push.show_detail`。
- **问题**：
  - F-APN-01 规则 4 只列了退出、改密、强制下线、封禁、注销。**找回密码（全部吊销）、refresh 重放吊销、refresh 过期**都没有删令牌。
  - 断网退出时，服务端的 `sid` 和令牌一直留着（F-ACC-04 规则 1），refresh 最长 90 天，AC-ACC-04-6 只验证本地。
  - `show_detail` 存在令牌行上，令牌移到新账号时没有规定重置。
  - 内存队列和重试期间，没有规定发送时重新确认令牌归属。通知过期时间是 24 小时。
- **攻击场景**：
  - 手机被盗，机主用恢复码找回。如果开了详情模式，被盗手机继续在锁屏显示发送者和正文。
  - 共用设备上，A 开了详情后退出，B 登录，B 的消息默认带内容显示。
- **建议的修改**：
  1. 规定不变量："`sid` 吊销或过期，就在同一事务删除绑定令牌"；发送前按当前 `sid` 状态复查。
  2. 令牌换主时 `show_detail` 重置为关。
  3. 客户端本地退出时调用系统接口注销令牌（FCM `deleteToken`、APNs 注销），并保留待发的 logout 请求。
  4. 详情模式的通知过期时间缩短。
  5. 补 AC：找回、重放、断网退出之后的通道调用。

**R-CS-04 隐藏模式的 collapse id 和角标仍向苹果、谷歌泄露会话关系**
- **PRD 位置**：F-APN-02 规则 6、F-APN-03 规则 1、F-APN-04 规则 2、AC-APN-03-1、第 12.2 节（"数据字段不含任何 ID"）。
- **问题**：
  - "同一会话相同 collapse id"没有规定怎么生成。如果是 `convId` 或它的普通哈希，`c2c:a:b` 由用户名组成，可以用字典反推。
  - `apns-collapse-id` 是请求头，AC-APN-03-1 只查请求体，测不出来。
  - 角标也带出未读数量。
- **攻击场景**：通道方或拿到通道数据的人，能按 collapse id 还原"哪些通知属于同一会话"，甚至还原具体的两个用户。
- **建议的修改**：隐藏模式用每设备固定的 collapse id，或用服务端密钥 HMAC（设备 + 会话）。AC 改为检查整个请求（含请求头）。隐藏模式的角标是否保留，写入 CH-11 让 Frank 知情。

**R-CS-05 refresh 宽限期"返回相同 token"和"只存哈希"矛盾**
- **PRD 位置**：F-ACC-03 规则 3、4，AC-ACC-03-3，第 12.1 节 `device_sessions`。
- **问题**：
  - 要在 30 秒内返回同样的新 refresh，服务端必须留存它的明文。但表里只有 `refresh_hash`、`prev_refresh_hash`。
  - 实现时只能违反其中一条。如果偷偷存明文，就破坏了第 12.2 节的对策。
- **攻击场景**：数据库或备份泄露后，宽限期内的明文 token 可以直接使用。宽限期内的旧 token 还能换到受害者的同一个新 token，且不触发重放检测。
- **建议的修改**：宽限期内签发新的一对，不复用旧的；同一会话最多保留一条有效链。改 AC-ACC-03-3。宽限期调短，并记录次数。

**R-CS-06 撞库防护可以轻易绕过，也没有被盗提醒**
- **PRD 位置**：F-ACC-02 规则 3、第 12.2 节、CH-02、CH-04。
- **问题**：
  - 没有按用户名的总量限制。计数键是"用户名 + IP"，换 IP 就重新计数。
  - "按 IP"没有定义 IPv6 聚合。现有 `guard.clientIP` 返回完整地址，如果沿用，一个 /64 相当于无限个 IP。
  - CH-02 让登录名等于公开 `userId`，群成员列表就是攻击目标清单。
  - 没有 MFA，也没有新设备登录提醒。第一批没有设备列表，受害者无从发现被盗。
- **攻击场景**：用少量 IPv6 前缀或代理池，针对单个用户名持续试密码。或者用泄露的密码库，按群成员列表逐个撞。
- **建议的修改**：
  1. 增加按用户名的渐进延迟（不锁死）；已登录设备的正常操作不受影响。
  2. IPv6 按 /64 计。
  3. 新设备登录时写一条会话内安全提示（只发给该账号）。
  4. 改密、找回时也提示。

**R-CS-07 Argon2 并发闸门可被未登录者打满，连带挡住补救操作**
- **PRD 位置**：F-ACC-01 异常（`/v1/account/*` 共 8 个并发，满了 503）、第 4.2 节、F-ACC-03 规则 5。
- **问题**：
  - 闸门覆盖了 `/v1/account/*` 全部接口，包括不做密码哈希的 refresh 和 logout，也包括被盗后要用的改密。
  - console 的参数是 64 MiB、t=3（`secret.go`），8 个并发约占 512 MiB。ARCH 第 7 节要求 `GOMEMLIMIT` 按存活内存配置。
  - 规定了登录限流，但闸门和限流的先后没有写。
- **攻击场景**：少量 IP 持续发登录、注册、找回请求，所有用户无法登录、改密和注销。access token 过期的客户端刷新也失败。
- **建议的修改**：
  1. 闸门只管做哈希的路径，refresh 和 logout 不进。
  2. 先按 IP 和用户名限流，再进闸门。
  3. 已登录用户的改密单独留配额。
  4. 明确 refresh 收到 5030 时 SDK 不回登录页。
  5. Argon2 内存占用写进容量估算。

**R-CS-08 `invite` 模式没有注册限流，防枚举指标实际被破坏**
- **PRD 位置**：F-ACC-01 规则 1、7，AC-ACC-01-10，F-ACC-09 规则 1，PRD-social 1.3 和 11.2 节。
- **问题**：
  - 限流只写了 `open` 模式。
  - 邀请码成功注册才扣减，所以持有一个有效邀请码的人可以无限次得到 4095，不可被搜索的用户也能被确认存在。这和 PRD-social 每账号每天 100 + 100 的上限直接冲突。
  - 没有有效邀请码的人也不受限，可以持续猜码。邀请码的长度和熵都没有规定，但只存哈希和后 4 位，为了查找只能用确定性哈希，所以码必须足够长。
  - `maxUses` 没有上限。
- **攻击场景**：拿到任何一个邀请码后，遍历常见用户名，得到完整用户清单。或者暴力猜短邀请码。
- **建议的修改**：
  1. 所有模式都按 IP 限流。
  2. 单个邀请码的 4095 失败次数设上限，超过就停用并告警。
  3. 规定邀请码至少 128 位随机。
  4. `maxUses` 设上限。
  5. 第 16 节写明这条路径的预算，补 AC。

**R-CS-09 `open` 模式的启动保护形同虚设，也没有防批量注册**
- **PRD 位置**：F-ACC-01 规则 2、7，F-MOD-01 规则 6，第 12.3 节，CH-03。
- **问题**：
  - 规则 2 只检查 `IM_MOD_ENABLED=1`，而词库为空时"不拦截"，只给配置告警，所以保护能被空词库通过。
  - 只有每 IP 每小时 20 次。没有人机验证或工作量证明，新账号的上传和发送额度也没有分级。
  - 第 12.3 节说开放前要补手机号和法务手续，但技术上改一个环境变量就能开。
- **攻击场景**：一个 IP 每天约 480 个账号，每个账号每天 2 GiB，即约 960 GiB 的上传额度。同时批量注册用于骚扰。
- **建议的修改**：`open` 模式在生产环境额外要求：词库非空、举报处理人已配置、有防批量注册手段、显式确认项（如 `IM_PUBLIC_OPEN_ACK`）。新账号使用较低配额。

**R-CS-10 文件响应头和类型校验有缺口：多格式文件、缩略图、公开头像**
- **PRD 位置**：F-FILE-01 规则 3，F-FILE-02，F-FILE-03 规则 4，F-ACC-06 规则 3、4，AC-FILE-03-3。
- **问题**：
  - 附件头只写在文件消息上。图片、缩略图、头像的 `Content-Type` 没有规定由服务端固定，`X-Content-Type-Options: nosniff` 也没有写。
  - 只嗅探前 4 KB，开头是合法 PNG 或 JPEG、后面藏 HTML 的文件可以通过。
  - 规则 3 没有说缩略图也要嗅探，AC 也只测原文件。
  - 头像无签名、公开读，没有写响应头和桶策略隔离；只靠"客户端裁剪"，服务端不校验。
  - AC-FILE-03-3（内容是 HTML 的 `.txt` 能下载）和 AC-FILE-01-3（内容与声明矛盾就拒绝）可能互相冲突。
- **攻击场景**：上传头部合法的 HTML 头像，把公开地址发给别人点开；如果对象存储或 CDN 按内容嗅探，脚本就在存储域名下运行。独立域名能挡住对 App 的 XSS，挡不住钓鱼。缩略图可以直接是任意内容。
- **建议的修改**：
  1. 所有下载地址都签入 `response-content-type`（用服务端记录的值）和 `response-content-disposition`。
  2. 非图片一律 attachment。
  3. 通过桶或 CDN 配置 `nosniff`（目标存储能否配置，**未核实**）。
  4. 缩略图同样校验。
  5. 头像用独立桶和独立域名，并禁止列举。
  6. 补 AC。

**R-CS-11 校验后还能覆盖对象（TOCTOU），大小和哈希绑定未验证**
- **PRD 位置**：F-FILE-01 规则 1、3，第 21 节第 4 项，AC-FILE-01-2、01-8。
- **问题**：
  - 预签名 PUT 有效 15 分钟，`complete` 之后没有规定它失效。
  - 大小和 SHA-256 写进签名这件事是否可行，PRD 自己标了未验证。如果不支持，第 21 节的退路是"complete 时读取校验"，和"内容不经过 im-server"及 AC-FILE-01-8 冲突。
  - 同一缺口还会让按声明大小扣配额的做法失效。
- **攻击场景**：先传干净图片并 `complete`，再用同一地址覆盖成恶意内容。如果目标存储不校验大小，还能绕过单文件限制和配额。
- **建议的修改**：
  1. `complete` 时服务端把对象复制到最终键（客户端从不知道最终键），并校验大小和 ETag 或校验和。预签名只指向临时键。
  2. C2 第一步的技术验证通过前，不要冻结 F-FILE-01。
  3. HEAD 和 Range 读取的预算写清楚。

**R-CS-12 R-8 的判断不成立：伪造缩略图能影响接收方和审核员**
- **PRD 位置**：F-FILE-02 规则 1、3，第 19 节 R-8，CH-08。
- **问题**：R-8 说"只影响恶意发送者自己的信息"，这是错的。
  - 缩略图和原图可以完全不同。
  - `width`、`height` 由客户端声明，只做范围校验。
  - 缩略图和原图都可以是解码炸弹（512 KiB 的 PNG 可以解压出极大的位图）。
  - CH-08 说减少了解码器攻击面，实际是把它转移到了每个接收方和审核员的设备上。
- **攻击场景**：
  - 接收者看到的缩略图是违规内容，审核员只看原图（R-8 明确写了），原图干净，举报被驳回。
  - 在 2000 人群发解码炸弹，让客户端崩溃或卡死；如果是最新消息，还可能反复触发。
- **建议的修改**：
  1. 改写 R-8。
  2. 审核员和证据必须同时看到缩略图和原图。
  3. 服务端在嗅探时解析头部，读取真实尺寸（不做解码），不用客户端声明的值。
  4. 三端解码前检查像素上限，超限显示占位。
  5. CH-08 的"影响"一栏写明这些风险。

**R-CS-13 文件禁止名单不完整，文件名可伪装**
- **PRD 位置**：F-FILE-01 规则 6，F-FILE-05 规则 1 和异常，第 12.2 节（写的是"类型白名单和禁止名单"，文件其实只有禁止名单），CH-07、CH-09。
- **问题**：
  - 示例名单里没有 `html`、`htm`、`xhtml`、`svg`、`hta`、`lnk`、`chm`、`iso`、宏文档等。SVG 只在图片消息里被禁，`kind=file` 可以发。
  - 文件名清洗只提路径分隔符和控制字符，没有提 Bidi 控制字符（U+202E）。
  - 没有处理 Windows 的结尾点和空格（`a.exe.`）、`::$DATA`。
  - 脚本类没有魔数，"真实类型"校验对它们无效，无法识别的又一律放行。
- **攻击场景**：文件名 `photo<U+202E>gpj.html`，实际是 `.html`，界面显示类似 `.jpg` 结尾，下载后被系统浏览器打开。Android 和 iOS"用系统方式打开"放大了这个风险。
- **建议的修改**：
  1. 补全名单，加入 html 和 svg 类。
  2. 文件名去掉 Bidi 和不可见字符，去掉结尾的点和空格，先规范化再判断扩展名。
  3. 客户端打开文件时，非白名单类型只提供"保存"。
  4. 修正第 12.2 节的描述。
  5. 补 AC。

**R-CS-14 文件名进入后台元数据，构成单聊内容泄露**
- **PRD 位置**：第 14.1 节"消息元数据、正文页"（显示文件名和大小），第 11.6 节，对照 F-ADM-30 规则 2、AC-ADM-30-7。
- **问题**：文件名是用户写的内容（例如"离婚协议.pdf"）。PRD-admin 规定元数据不含正文，`console` 主凭证（带 `msg.meta`）不能读内容，单聊正文还受 X-7 限制。第 14.1 节这一行没有区分元数据页和正文页。
- **攻击场景**：客服填个理由查单聊元数据，就能看到双方传过什么文件。`IM_C2C_BODY_ACCESS=off` 也拦不住。
- **建议的修改**：明确文件名按正文对待，`msg.meta` 只返回类型和大小，文件名要有正文授权。补一条类似 AC-ADM-30-7 的特征串用例。

**R-CS-15 单聊证据和合规门槛的说明不完整**
- **PRD 位置**：F-MOD-03 规则 5、F-MOD-04 规则 3、4，第 12.3 节，第 16 节，第 17 节 C6，CH-22、CH-23；对照 PRD-admin 第 12 节。
- **问题**：
  - PRD-admin 第 12 节（已冻结）要求开放**群消息正文查看和隔离原文**前，必须完成隐私政策发布、个人信息影响评估、操作规程和培训。第 12.3 节默认写"内部告知"，第 17 节 C6 前提和第 16 节都没有提到它。第 12 节的主语还是"宿主 App"，而现在已经没有宿主 App。
  - 单聊举报在法务确认**前**就把正文写入证据，保留 180 天。将来开关一开，这些在当时告知口径下收集的内容就全部可读。
  - 证据保留期（180 天）比隔离期（30 天）长，用户侧没有说明。
- **攻击场景**：不是技术攻击，是合规风险：产品按"内部告知"上线了 C6，实际上没有满足自己冻结的门槛；法务后来否定时，已经存了一批单聊证据。
- **建议的修改**：
  1. 第 12.3 节和第 17 节写明 PRD-admin 第 12 节的门槛仍然有效，并改写其中的"宿主 App"。
  2. `IM_C2C_BODY_ACCESS=off` 时，单聊证据只存元数据；或者加上"开关打开前收集的证据永远不可读"的规则。
  3. 把证据保留期写进告知内容。

**R-CS-16 撤回的消息无法在界面上举报，CH-17 的理由落空**
- **PRD 位置**：F-RCL-03 规则 3、6，F-MOD-03 规则 2、8，CH-17，AC-MOD-03-5。
- **问题**：F-MOD-03 规则 2 允许举报已撤回的消息，但 F-RCL-03 规则 6 说撤回的消息没有菜单，举报入口又是长按消息。客户端也已经清除了内容。AC 只测了"先举报后撤回"。
- **攻击场景**：骚扰者发完 2 分钟内撤回。受害者只能举报用户，没有消息证据。CH-17 用"防止销毁骚扰证据"作理由，保留了 30 天原文，普通用户却用不上这份证据。隐私代价付了，好处没拿到。
- **建议的修改**：撤回提示行提供"举报"入口。服务端校验举报人当时能看到这条消息。补 AC。也要防反向滥用，见 R-CS-25。

**R-CS-17 敏感词检查范围有漏洞**
- **PRD 位置**：F-MOD-02 规则 2、3，F-MOD-01 规则 3，F-ACC-01 规则 3、6，F-APN-03 异常。
- **问题**：
  - **好友申请附言**不在检查范围内（`FriendApplyReq.Message`，0～50 码点，可以发给陌生人）。
  - **用户名**只检查保留名，不过敏感词。它不可修改，是公开 ID，还是昵称为空时的显示名。
  - 标准化没有处理空格和标点插入（"违 规 词"），也没有说明变体选择符、Bidi 等其他不可见字符。
- **攻击场景**：用附言向陌生人群发违规内容（对方不需要是好友）。注册一个本身就违规的用户名，且后台无法让他改名。
- **建议的修改**：范围加上附言和用户名（C6 上线前已注册的用户名要有后台处理办法）。标准化时可以额外生成一份去空白和标点的文本用于匹配（注意误判，要测）。不可见字符按 Unicode Default_Ignorable 处理。`custom` 的处理本身没问题。

---

## 一般

**R-CS-18 改密不影响恢复码和未用的重置码**
- **PRD 位置**：F-ACC-04 规则 3、F-ACC-05 规则 3～5。
- **问题和场景**：U3 的手机丢了，用户改密后，存在手机里的恢复码还有效，拿到手机的人能再次夺走账号。未使用的重置码也没有规定失效。
- **建议的修改**：改密后强提示重新生成恢复码（或在改密响应中直接换新）；改密和找回时作废全部未用的重置码；限制同时只有一个有效重置码。

**R-CS-19 access 和 refresh 的时长偏长，H5 没有区分**
- **PRD 位置**：第 4.1 节、F-ACC-03 规则 3、7。
- **问题和场景**：H5 关标签页就要重新登录，却持有 90 天滑动 refresh，放在 JS 可读的 `sessionStorage`，一次 XSS 就能长期冒用。滑动续期也没有绝对上限，access 还有 7 天。
- **建议的修改**：H5 使用短 refresh TTL（如 24 小时）；所有端设绝对上限；评估把 access 缩到小时级（现有 `auth` 每次都查库，缩短不影响吊销，只缩小被盗窗口）。

**R-CS-20 找回和锁定的防枚举细节没有写全**
- **PRD 位置**：F-ACC-02 规则 3、F-ACC-05 规则 2，AC-ACC-02-4、05-2，F-ACC-09 规则 4，第 11.4 节。
- **问题和场景**：
  - 没有写不存在的用户名也累计失败次数。如果只对存在的用户计数，第 6 次 4294 和 4018 的差别就暴露了存在性。
  - `recover` 没有做等时要求，也没有时间 AC。
  - 重置码的长度和熵没有规定。
  - 失败计数存在哪里没写。沿用内存实现（ARCH G2）的话，重启就清零，多节点时额度翻倍。
- **建议的修改**：逐条补规则和 AC；计数器进 `Limiter` 接口并写明持久性要求；重置码规定最小熵。

**R-CS-21 上传接口没有频率限制**
- **PRD 位置**：F-FILE-01 规则 5，F-FILE-04 规则 6（只限制了取地址），第 12.2 节。
- **问题和场景**：一个账号每天 2 GiB 额度够申请上亿个 1 字节的上传，形成写库和 `complete` 回源 HEAD 的压力。多账号可叠加额度。2000 人群转发 100 MiB 文件会产生大量出口流量，没有下载配额和费用保护。
- **建议的修改**：加每分钟申请次数和待完成上传数上限；加全局日用量告警；下载次数和流量设上限；费用估算（第 15 节）加入出口流量。

**R-CS-22 预签名地址的泄露面只覆盖了 im-server 日志**
- **PRD 位置**：F-FILE-04 规则 4、5，AC-FILE-04-7，R-6，CH-06。
- **问题和场景**：
  - 对象存储和 CDN 的访问日志、浏览器历史、OkHttp/URLSession 的调试日志都可能带完整查询串。
  - 被踢前批量预取：每分钟 120 次，5 分钟最多约 600 个有效地址，退群后仍能下载。R-6 说"5 分钟内仍可下载"，没有说可以批量预取。
- **建议的修改**：
  1. 地址 TTL 按用途分：图片短（如 60～120 秒），大文件按下载需要。
  2. 响应加 `Referrer-Policy`（H5）。
  3. 关闭存储访问日志或脱敏，写进部署要求。
  4. SDK 的 HTTP 层加日志脱敏 AC。
  5. 按用户统计取地址量，异常时告警。

**R-CS-23 撤回和隔离期间，文件的保护弱于文本**
- **PRD 位置**：F-FILE-06 规则 4，F-RCL-01 规则 5，AC-RCL-01-6，第 12.1 节 `files`。
- **问题和场景**：
  - 文本原文是 AES-GCM 加密隔离，文件对象却原样留在桶里 30 天，也没有要求服务端加密。
  - 撤回时规定清除正文，但 `files.name`、`sha256` 仍是明文，AC 只查 `messages` 和 `conv_seq`。过期文件（"消息保留，显示已过期"）的文件名也一直留着。
- **建议的修改**：
  1. 要求桶启用服务端加密（可行性和密钥归属 Frank 选型时确认）。
  2. 撤回时把文件名和哈希也移到隔离加密区。
  3. 过期后清除文件名和哈希。
  4. 把 `files` 表加入 AC-RCL-01-6。

**R-CS-24 审核员看文件的方案和现有 CSP 冲突；H5 CSP 变更遗漏**
- **PRD 位置**：第 11.6 节 `POST /admin/files/{fileId}/url`、第 14.1 节、第 16 节；对照 F-CON-10 规则 4（`img-src 'self' data:`、`connect-src 'self'`，"不能放宽"）、NF-UI-14（`connect-src` 只允许同源）。
- **问题和场景**：
  - 后台拿到预签名地址后，无法在现有 CSP 下直接加载，开发时可能放宽 CSP。
  - 走预签名地址也绕开了水印、`no-store` 和"每页审计"。恶意文件直接出现在高权限审核员的浏览器里。
  - H5 直传对象存储也要改 `connect-src`，第 16 节没有列 NF-UI-14。
- **建议的修改**：后台文件经 console-server 代理，固定响应头（图片校验后内联，其他只提供 attachment）。NF-UI-14 进第 16 节，只允许具体的存储域名，不能放宽成 `https:`。单聊文件也要有 AC（`c2cBodyDisabled`）：第 11.6 节写的"要有效授权"逻辑上会被拒绝，但没有用例。

**R-CS-25 举报可被滥用：延长保留、影响文件可用性、元数据缺审计**
- **PRD 位置**：F-MOD-03 规则 2、5，F-FILE-06 规则 5，F-MOD-04 规则 2；对照 F-ADM-30 规则 3。
- **问题和场景**：
  - 群成员即使没看过原文，只要知道 `msgId`，就能举报已撤回的消息，把它的保留期从 30 天延到 180 天。这时用户请求路径上还要解密隔离原文。
  - F-FILE-06 规则 5 写"过程中不发放给普通用户"，如果照字面实现，举报一下就能让全群都下载不了这个文件。
  - F-MOD-04 规则 2 的举报元数据暴露单聊双方关系，却没有理由、审计和配额（F-ADM-30 规则 3 对同类信息要求理由，每天 50 个）。`note` 也可能包含单聊内容，规则没有说它算元数据还是正文。
  - 只保存举报人选中的消息，没有上下文，判定容易被断章取义。
- **建议的修改**：
  1. 限定举报撤回消息的人（如撤回前已投递或已加载）。
  2. 把规则 5 改为"原消息正常时照常发放；到期后才不发放"。
  3. 查看举报单写审计，单聊举报元数据的要求和 F-ADM-30 对齐。
  4. `note` 按正文对待。
  5. 评估是否保存前后若干条元数据作为上下文。

**R-CS-26 生产环境配置 `token.issue` 只告警**
- **PRD 位置**：F-ACC-07 规则 3、AC-ACC-07-3。
- **问题和场景**：已经没有业务方，这个权限没有正当用途，却能静默冒充任何用户；签发的 token 没有 `sid`，用户侧完全看不到。告警容易被忽略。`admin/admin.go` 里的 `issueFor` 也会自动创建用户。
- **建议的修改**：按 ARCH S4 的做法，生产默认拒绝启动，只有显式确认项才放行，并在 im-server 侧写审计。

**R-CS-27 注销后的数据处理不完整，也没有冷静期**
- **PRD 位置**：F-ACC-08 规则 2、3，F-ACC-06 规则 4、5，第 12.3 节；对照 ARCH S13。
- **问题和场景**：
  - 公开读的头像对象只写了"头像清空"，没有写删除；知道地址的人仍能访问。
  - `userId`（用户选的，可能是真名）继续留在 `from` 和 `convId` 里，也留在索引中防止复用。界面显示"已注销用户"，协议层还是原名。
  - 文本消息没有保留期（PRD-admin F-ADM-22 附近写明"未实现前永久保存"），只有文件有 180 天。"注销"对用户的实际意义需要写清楚。
  - 只需密码就立即不可恢复。密码被盗就能永久毁掉账号、好友关系和群主身份。
- **建议的修改**：
  1. 注销时删除头像对象。
  2. 协议层用不可逆的别名替换 `from`，或者如实告知。
  3. 列出每类数据的处理方式。
  4. 加几天冷静期，期间可取消（CH-25 可作为选项）。

**R-CS-28 用户账号事件没有审计记录，日志 AC 的特征串不全**
- **PRD 位置**：第 14.2 节（只有指标）、F-ACC-09 规则 5、AC-CHAT-90-5、第 1.3 节。
- **问题和场景**：
  - 用户的登录、改密、找回、重放吊销都没有记录，只有计数指标和"最近登录时间"。发生盗号时无法调查，也无法答复投诉。
  - 特征串清单没有邀请码、重置码、好友附言（已有其他 AC）、`deviceName`。"密码误输进用户名框"会把密码写进失败日志，失败日志不应记录原始用户名。
- **建议的修改**：增加 `account_events`（事件类型、时间、IP 前缀、`sid`，有保留期；隐私政策要写）。补全 AC-CHAT-90-5 的特征串。对失败登录只记用户名的哈希。

**R-CS-29 `thumbFileId` 的定义前后不一致**
- **PRD 位置**：F-FILE-01 规则 1（响应只有 `thumbPut`）、F-FILE-02 规则 1（`thumbFileId`）、F-FILE-04 规则 1（`variant`）、第 12.1 节（`thumb_key`）、AC-FILE-02-3。
- **问题和场景**：缩略图到底是不是一个独立 `fileId` 不清楚。如果独立，AC-FILE-02-3 只校验了 `fileId`，发送方可能引用别人的上传或已绑定的文件当缩略图，绕过"一个文件绑定一条消息"，造成越权读取。
- **建议的修改**：统一为一个 `fileId` 两个变体，去掉 `thumbFileId`；如果保留，补全同样的校验和 AC。

**R-CS-30 内容治理失败即放行，且无法被发现**
- **PRD 位置**：F-MOD-01 异常、F-MOD-02 异常（20 毫秒超时放行）、第 4.1 节 `IM_CLIENT_CUSTOM_MSG`、AC-MOD-02-6。
- **问题和场景**：放行是有理由的取舍，但启动失败时只有告警，默认"不发页面告警"（第 14.2 节），实际可能长期不生效。`IM_CLIENT_CUSTOM_MSG=1` 在生产环境也没有拒绝或告警。CPU 被打满时，超时放行还能被故意利用。
- **建议的修改**：`IM_MOD_ENABLED=1` 时启动加载失败，配置检查必须显示错误。放行比例设告警。生产环境打开 `custom` 时进配置检查。`open` 模式下改为失败即拒绝。

---

## 建议

- **R-CS-31 推送令牌绑定没有持有证明**（F-APN-01 规则 2）：知道令牌的人可以把它注册到自己账号，把受害设备的令牌抢走。实际利用要先拿到令牌，所以风险低。可加每账号注册频率限制，并在客户端校验通知归属。
- **R-CS-32 `clientMsgId` 是隐蔽通道**（`frames.go` 第 140 行）：8～64 字节的客户端自定义串会原样下发给接收方，改过的客户端之间可以绕过敏感词。可以只下发给发送方自己的其他端。这个问题已经存在，但关闭 `custom` 后它成了仅剩的旁路之一。
- **R-CS-33 `deviceName` 没有长度和字符限制**（F-ACC-01 规则 1、F-ACC-02 规则 1）：用户控制的文本，今后会进设备列表和后台页面。建议现在就规定长度，按纯文本渲染，不进日志。
- **R-CS-34 撤回原文的物理清除没有覆盖 Postgres**（AC-RCL-01-6）：SQLite 已开 `secure_delete`（`sqlite.go` 第 45 行，注释也说明 WAL 要等检查点）。生产目标是 Postgres（ARCH A1），死元组要等 VACUUM。"原始字节搜不到"不能作为生产结论。另外每小时备份保留 48 份（F-ADM-31 规则 9），对用户的撤回说明也要沿用"不再显示"的措辞。
- **R-CS-35 `sid` 吊销也要补连接竞态，并复用现有 IP 逻辑**：`gateway.go` 第 165～168 行对令牌版本有"校验后再查一次"的防竞态处理，第 11.4 节应规定 `sid` 也一样。`/v1/*` 必须复用 `guard.clientIP` 的可信代理逻辑（现在是 gateway 私有方法），不能各写一份，否则 `X-Forwarded-For` 可伪造，所有按 IP 的限制失效。

---

## 查过，没有发现问题

- **SSRF**：没有发现入口。头像只接受 `avatarFileId`，群没有客户端设置的头像字段（`GroupUpdateReq` 只有名称和公告），对象键由服务端生成，推送端点固定。
- **路径穿越**：对象键不含文件名。客户端保存时仍要再清洗一次（见 R-CS-13）。
- **token 进 URL**：`/v1/*` 用 Bearer，WS 首帧鉴权不变。
- **其他**：
  - 密码只存 Argon2id，登录响应统一和假哈希的设计合理。
  - 引用下发时组装、撤回后清除本地引用的设计合理，AC 也够。
  - `historyVisible=false` 经引用看到摘要，PRD 已作为已知限制列出，可以接受。
  - 深链只接受已知路由，账号不符时进会话列表。
  - X-Forwarded-For 现有实现（从右往左取第一个不可信地址）正确。

---

## 未核实

- 目标对象存储是否支持：签名里绑定大小和校验和（PRD 第 21 节也未核实）、条件写入、响应头配置 `nosniff`。
- 原生端本地库和文件缓存是否加密，Android 的 `allowBackup` 配置，下载到公共目录后其他 App 能否读取。这些我没有查客户端仓库。第一批新增了文件缓存，建议客户端评审专门看一遍。
- FCM 和 APNs 对 collapse id 的具体限制和日志留存。
- 合规：我不做法律判断。R-CS-15 只指出文档之间的矛盾和假设漏项。PRD 第 12.3 节标注"未确认""要问法务"的做法是诚实的；不诚实或不准确的地方是 R-8 的判断、第 12.2 节的"类型白名单"和"不含任何 ID"这几处说法。

---

## 对 CH 项的意见（只列有问题的）

| CH | 意见 |
|---|---|
| **CH-01** | 建议方案不安全（R-CS-01）。"后台重置码兜底"要加：独立凭证、双人审批、冷静期和设备通知。**遗漏的选项**：不提供客服重置，恢复码丢了账号就无法找回（内部小范围可以接受，也最安全）。 |
| **CH-02** | 建议方案有安全代价，且写得偏轻：登录名公开，注销后也留在协议数据里，不可修改。备选只需约 3 人日（未校准），之后改不了，建议 Frank 重新权衡。如果维持建议，必须同时修 R-CS-06 和 R-CS-17。 |
| **CH-03** | `open` 的条件不够（R-CS-09）。 |
| **CH-04** | 遗漏选项：第一批不做设备列表，但提供"退出其他所有设备"按钮。现在被盗后只能改密，而且改密有 R-CS-18 的问题。 |
| **CH-06** | 可以接受预签名，但"影响"要写明批量预取和存储日志（R-CS-22），并按用途分 TTL。 |
| **CH-08** | 可以接受客户端生成缩略图，但必须改写 R-8，并增加服务端读尺寸、审核员看两份、客户端解码上限（R-CS-12）。 |
| **CH-09** | "只拦可执行类型"不够（R-CS-13），名单要补全。 |
| **CH-11** | 建议方案安全，但要加上 R-CS-04。开启详情是接收方的决定，发送方的内容因此经过第三方，隐私政策要说明。 |
| **CH-17** | 遗漏选项：撤回原文只保留很短时间（如 7 天）；或者保留 30 天，但必须先修 R-CS-16，否则隐私代价换不到证据价值。 |
| **CH-22** | 建议方案（不可读）安全，但遗漏了"证据写入"的问题（R-CS-15）。备选"放开被举报的几条"要同时规定举报人知情提示和生效时间点。 |
| **CH-23** | 遗漏中间情形：有真实用户、邀请制、但会用 App Store 或 TestFlight 分发。这种情况下"内部告知"可能不够，需要问法务。PRD-admin 第 12 节的门槛也要纳入这项决定。 |
| **CH-25** | 遗漏选项：注销加冷静期（R-CS-27）。 |

---

## 结论：是否可以进入拍板

**现在不建议进入拍板。先出 v0.2，至少改下面几项后再让 Frank 拍板：**

1. **R-CS-01（阻塞）**：它决定 CH-01 的方案本身。
2. **会改变 CH 内容或影响判断的 5 条**：
   - R-CS-12（CH-08 依据的 R-8 判断错误）
   - R-CS-13（CH-09）
   - R-CS-15（CH-22、CH-23，文档之间有矛盾）
   - R-CS-16（CH-17）
   - R-CS-09（CH-03）
3. **CH 表格本身的修改**：更新上表列出的 CH-02、CH-04、CH-06、CH-11、CH-25 的备选和影响。

其余严重项（R-CS-02～08、10、11、14、17）不影响拍板方向，但必须在**冻结前**解决。R-CS-11 还依赖 C2 第一步的技术验证。一般项可以在 v0.2 一起改，也可以进入各里程碑的安全用例（TC-SEC），不能悄悄丢掉。

总体判断：PRD 对主要风险的结构设计是认真的（默认隐藏推送、防枚举、隔离、审计都有考虑），问题集中在三处：新增账号能力打穿了已冻结的正文隔离边界；吊销和文件校验的不变量没有写全；有几条风险判断写得过于乐观。这些都能在文档层面修改，不需要改变第一批的范围。
