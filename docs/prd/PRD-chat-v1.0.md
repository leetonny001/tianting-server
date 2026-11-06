# PRD：天听聊天产品第一批（账号、图片文件、离线推送、撤回引用 @、会话设置、内容治理）

| 项 | 内容 |
|---|---|
| 版本 | **v1.0（冻结，2026-11-06）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-06 |
| 状态 | **已冻结，可以分发任务**。之后要改走变更流程（[process/README](../process/README.md)）。评审和回应：[REVIEW-chat-v0.1](../review/REVIEW-chat-v0.1.md)（第 23 节）、[RECHECK-chat-v0.2](../review/RECHECK-chat-v0.2.md)（第 24 节）、[RS4](../review/RECHECK-chat-v0.4-security.md)（第 25 节）、[RS5](../review/RECHECK-chat-v0.5-security.md)（第 26 节）、[RS6](../review/RECHECK-chat-v0.6-security.md)（第 27 节）、[RS7](../review/RECHECK-chat-v0.7-security.md)（第 28 节）；技术验证见 [GATE-objstore](../review/GATE-objstore-2026-11-06.md)。**排期只有 C1、C2 和公共部分**（CH-27） |
| 取代 | [PRD-chat-v0.7](PRD-chat-v0.7.md) |
| 依据 | Frank 2026-11-06 的方向调整（[PLAN](../PLAN.md) 开头）和第一批范围（本文第 2.2 节） |
| 对照 | [PRD-core v1.4](PRD-core-v1.4.md)、[PRD-social v1.0](PRD-social-v1.0.md)、[PRD-ui v1.2](PRD-ui-v1.2.md)、[PRD-admin v1.2](PRD-admin-v1.2.md)、[PRD-ops v1.3](PRD-ops-v1.3.md)、[ARCH-v1](../arch/ARCH-v1.md)、[protocol/](../../protocol/README.md) |
| 阅读提示 | Frank 2026-11-06 回复"全部按建议"，AC 不用因拍板改动。标"未校准"的数字没有测过；标"未核实"的事情见第 21 节。AC 编号的规则：同一验证目的修正预期的同号改写，末尾标"（v0.2 改：来源）"；验证目的变了就废弃旧号、新增编号，废弃清单见第 16.3 节。新增 AC 接着原有编号 |

---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 这份文档做什么 | 把天听从"业务方接入的 IM"改成"用户自己注册使用的独立 IM"。第一批 6 个方向，共 40 个功能点（第 5～10 节）和 1 个非功能点（第 15 节） |
| Frank 还要做什么 | **第 20.0 节的 3 个事实问题和第 20.3 节的事**。13 项拍板已完成（第 20.1 节） |
| 账号 | **用户名 + 密码 + 恢复码，邀请码注册，第一批没有客服重置**（R-CS-01：客服能重置就等于能读任何人的聊天）。**代价：恢复码丢了找不回；密码被盗后只有 7 天能用旧恢复码找回**（CH-01）。`userId` 就是用户名，**这个决定不可逆**（CH-02）。刷新令牌用"新一代首次使用才生效"，加 2 分钟重叠窗口，常见的弱网情况不会被踢（RS7 修订没有再审，没有实现和运行） |
| 文件 | 只依赖 S3 接口。上传到临时位置，服务端检查后带 ETag 条件复制到**每次尝试新的随机键**，元数据由服务端替换。**MinIO 已通过技术验证；云厂商要在上线前按 AC-FILE-90-2 验证** |
| 推送 | 默认不显示发送者和内容。**中国大陆 Android 第一批收不到离线通知**（CH-13）。APNs、FCM 账号要 Frank 开 |
| 撤回 | 发送者 2 分钟，群主、管理员 7 天。线上帧同时带 `deleted` 和 `recalled`，老客户端也会清除原文 |
| 内容治理 | 只拦截不替换。查文本、文件名、昵称、用户名、群名、公告、好友申请附言，不查图片和文件内容。词库和处理人要 Frank 指定 |
| 合规 | **内部小范围，邀请码注册**（CH-23，已定）。面向公众开放前要问法务，本文不是法律意见。**即使内部，隔离原文和群消息正文也有 PRD-admin 第 12 节的门槛** |
| 发现的既有问题 | ① 客户端可以发 `custom`，`text` 也可以夹带 `body.data`（`frames.go` 第 122～126、177～191 行）；② SDK 把 `convs` 里消失的群当作"已不在该群"；③ SDK 对已有消息只接受"变成已删除"，其他字段不更新（`store.ts` 第 96～99 行等）；④ 隔离加密依赖默认关闭的 `IM_V13_ADMIN_FEATURES`（`app.go` 第 205 行） |
| 对冻结文档的影响 | PRD-core、PRD-social、PRD-ui、PRD-admin、PRD-ops、ARCH、protocol 都要走变更；受影响的 AC 按编号列在第 16.2 节 |
| 规模 | AC 数见第 24 节（v0.1 是 249）。新增的主要是评审指出的没有 AC 的高风险规则。输入含多个场景、前端三端都要验，实际验收量更大 |
| 工作量 | 完整约 **284.5 人日，低置信**（第 18 节，RS4 修订加了 9）。**先做 C1 + C2 + 公共，约 145 人日（Frank 拍板时是 136），C1 结束后校准再决定后面**（CH-27，已定） |
| 拍板 | **13 项全部按建议**（第 20.1 节）。**3 个事实问题还没回答**，先按第 20.0 节的工作假设走 |

---

## 1. 背景与目标

### 1.1 背景

2026-11-06 Frank 决定：天听不再替换 fx-sport、ab77、android 的腾讯 IM，按正常的独立 IM 产品设计；聊天室和迁移取消（[PLAN](../PLAN.md) 开头、[QUEUE](../process/QUEUE.md) Q19、阶段五）。

现状（出处都是代码或文档，2026-11-06 核对）：

| 项 | 现状 | 出处 |
|---|---|---|
| 消息类型 | `text`、`custom`、`tip`；`tip` 只由服务端生成 | `protocol/schema/frame.schema.json` `MsgType`；`server/internal/protocol/frames.go` 第 42～43、177～191 行 |
| 客户端发 `custom` | 允许，只校验 `data` 非空，内容不检查 | `frames.go` 第 186～189 行 |
| 登录 | 只有 `POST /demo/login` 和 `POST /admin/users/token`；IM 不维护密码 | `server/internal/app/app.go` 第 278 行；PRD-core F-AUTH-01 规则 1、F-AUTH-04 |
| token | HS256 JWT，带 `kid` 和令牌版本号 `tv`，每次连接查版本号；默认 7 天、最长 30 天；没有刷新 | `server/internal/auth/token.go`；PRD-core F-AUTH-01 |
| 多端 | 同一账号最多 10 个连接同时在线，`replaced` 只保留不实现 | PRD-core F-AUTH-02 规则 2、5，F-AUTH-03 规则 5；`errors.json` `kickReasons` |
| token 存储 | H5 `sessionStorage`；Android Keystore 加密文件；iOS Keychain；都只存 `{userId, token}` | `tianting-web/src/lib/session.ts`；`tianting-android/.../core/SessionStore.kt`；`tianting-ios/.../Core/SessionStore.swift` |
| 离线推送 | 服务端、Android、iOS 都没有 APNs / FCM 代码 | 在 `im-mvp/server`、`tianting-android/app`、`tianting-ios`、`tianting-sdk-*`、`tianting-web/src` 里搜索 `apns`、`fcm`、`firebase`、`UNUserNotification`、`PushKit`，只命中构建缓存和 `package-lock.json` |
| 对象存储 | im-server 没有；`tianting-console-server/internal/objstore` 有 S3 客户端，只用于审计锚定和归档 | `objstore.go` 包注释 |
| 被删除的消息 | 软删除，原文进隔离区 30 天；三套 SDK 重连后重拉已加载范围（`recheckDeleted`），`convs` 的 `lastMsg.deleted` 也作为线索 | PRD-admin F-ADM-31、32；`IMClient.kt` 第 916～932、1186～1192 行；`client.ts` 第 467 行；`Client.swift` 第 509 行 |
| 未知消息类型 | 三端都显示"[暂不支持的消息]" | `ChatPage.tsx` 第 551 行、`ChatPage.kt` 第 590 行、`ChatPage.swift` 第 557 行 |
| 单帧上限 | 上行 64 KB | `gateway.go` 第 73 行；`protocol/README.md` 第 1 节 |
| PC | Tauri 未开工，等 Rust 授权 | [QUEUE](../process/QUEUE.md) Q16、PENDING P-14 |

### 1.2 目标

1. 用户不依赖任何业务方，能自己注册、登录、找回账号、改资料。
2. 聊天内容覆盖日常：图片、文件、撤回、引用、@。
3. App 不在前台时能收到通知。
4. 用户能管理自己的会话：置顶、免打扰、删除。
5. 有最基本的内容治理：敏感词拦截、用户举报、后台处理。
6. 不破坏已冻结的规则：不丢不重、多端一致、安全优先（ARCH 第 4 节）。

### 1.3 成功标准

| 指标 | 目标 | 度量方法 |
|---|---|---|
| 功能 | 第 5～10、15 节的 AC 全部通过 | TC 用例，追溯矩阵 |
| 不回归 | 演示模式下 `server/e2e` 现有用例不改代码全部通过 | CI（AC-ACC-07-1） |
| 端到端演示 | 新用户注册 → 加好友 → 发图片和文件 → 对方锁屏收到通知 → 撤回 → 引用 → 群里 @ → 置顶和免打扰 → 触发敏感词 → 举报并在后台处理 | 彩排，Android、iOS 真机各一次 |
| 隐私 | 密码、恢复码、token、推送令牌、预签名地址、文件名、消息正文都不进日志 | AC-CHAT-90-5 |

---

## 2. 范围

### 2.1 不变的约定

沿用 PRD-core 3.1 节的 ID、文本计数、时间、错误映射、客户端兼容规则。产品名天听；界面遵守 PRD-ui B-1～B-8，只描述交互，不使用其他产品的截图和品牌。

### 2.2 第一批（Frank 已确认，顺序也按这个）

> **CH-27（2026-11-06 已定）**：现在只排 C1、C2 和公共部分。C3～C6 的规格和 AC 保留，顺序不变，C1 结束后校准工作量，再由 Frank 决定要不要继续、什么时候继续。

| 顺序 | 里程碑 | 方向 | 功能点 |
|---|---|---|---|
| 1 | C1 | 账号注册和登录（头像部分依赖 C2） | F-ACC-01～09 |
| 2 | C2 | 图片和文件消息（需要对象存储） | F-FILE-01～07 |
| 3 | C3 | 离线推送（APNs、FCM） | F-APN-01～06 |
| 4 | C4 | 撤回、引用、@ | F-RCL-01～04、F-QUO-01～02、F-MEN-01～03 |
| 5 | C5 | 会话置顶、免打扰、删除 | F-CVS-01～04 |
| 6 | C6 | 敏感词和举报 | F-MOD-01～04 |

顺序带来的依赖（R-CQ-18、RC2-13）：头像依赖 C2；撤回的文件处理依赖 C2，撤回的替换通知依赖 C3；F-MEN-03 的推送依赖 C3 和 C5；F-APN-04 的角标依赖 C5；F-MOD-02 的文件名检查依赖 C2；**`mod_ver` 和 SDK 合并规则从 C2 开始做**（文件过期要用，C4 复用）。每条 AC 标在它依赖的最后一个里程碑上。**单独过关还有三个前提**（RC2-15、RECHECK 第六部分）：C3 的手工 AC 依赖 P-13 真机；性能结论依赖 P-12；C4、C6 的后台 AC 要打开 `IM_V13_ADMIN_FEATURES`。**C2 上线到 C6 上线之间没有内容治理手段（只有已有的后台删除、禁言、封禁，它们要 `IM_V13_ADMIN_FEATURES=1`，默认关闭），这段时间不能开放注册（`open` 模式）**，见 F-ACC-01 规则 2。`invite` 模式下公开传播一个大额邀请码等于开放注册，靠配置检查告警提醒（F-ACC-09 规则 1，RC2-18）。

### 2.3 后续批（只列，不写规格）

语音消息；转发和多选；聊天记录搜索；多端同时在线；扫码登录。

"多端同时在线"的边界：现有的多连接和消息同步（F-AUTH-02、F-SYNC-01）**已实现且保持不变**；后续批定义的是设备列表、互踢策略和跨端体验。

### 2.4 不做

朋友圈；音视频通话；小程序；支付。

第一批范围内也明确不做：客服重置密码（CH-01）；国内厂商推送通道（CH-13）；Web Push 和浏览器通知；系统通知（`push` 帧）和好友申请的离线推送；消息编辑、已读回执、正在输入；撤回后"重新编辑"；图片内容识别；文件秒传和去重；断点续传；"原图"选项；手机号登录（CH-01）；消息多选（举报只能单条，R-CF-21）；iOS 和 Android 里播放 GIF 动画（显示首帧，F-FILE-02）；设备列表页（只有"退出其他设备"）；PC 端验收（等 P-14）。

### 2.5 端的范围

| 端 | 第一批 |
|---|---|
| H5（`tianting-web`） | 全部功能，离线推送除外 |
| Android、iOS | 全部功能 |
| PC（Tauri） | **排除**。`tianting-web` 的宽屏界面照常实现，但不验收打包形态 |
| 管理后台、运维后台 | 配合项见第 14 节 |

---

## 3. 用户故事

| # | 角色 | 故事 | 功能点 |
|---|---|---|---|
| U1 | 新用户 | 我拿到邀请码，自己设用户名和密码注册，马上能用 | F-ACC-01 |
| U2 | 用户 | 我忘了密码，用注册时保存的恢复码重设；恢复码也丢了，账号就找不回，注册时我已被明确告知 | F-ACC-05 |
| U3 | 用户 | 手机丢了，我改密码后别的设备全部下线 | F-ACC-04 |
| U4 | 用户 | 我把照片发给朋友，发出前自动去掉拍摄位置；对方先看到缩略图，点开看大图 | F-FILE-02 |
| U5 | 用户 | 我把合同文件发到群里，退群的人再也下载不了 | F-FILE-03、04 |
| U6 | 用户 | App 在后台时有人找我，手机提醒我，但锁屏不显示内容 | F-APN-02、03 |
| U7 | 用户 | 我发错了，2 分钟内撤回，对方看不到内容 | F-RCL-01 |
| U8 | 群主 | 有人在群里发广告，我撤回他的消息 | F-RCL-02 |
| U9 | 用户 | 我引用某条消息回复，大家知道我在回什么 | F-QUO-01 |
| U10 | 用户 | 我设了免打扰的群，只有有人 @ 我才提醒 | F-MEN-03 |
| U11 | 用户 | 我把重要的人置顶，把没用的会话删掉，几台设备都一样 | F-CVS-01～04 |
| U12 | 用户 | 有人骚扰我，我举报他 | F-MOD-03 |
| U13 | 审核员 | 我在后台处理举报：看证据、删除消息、禁言或封禁 | F-MOD-04 |
| U14 | 运营 | 我维护敏感词，改完很快生效 | F-MOD-01 |
| U15 | Frank | 我用一个配置决定是否开放注册 | F-ACC-01、F-ACC-09 |

---

## 4. 全局默认值与通用规则

### 4.1 默认值（都可以用环境变量配置，全部未校准）

| 配置 | 默认 | 说明 |
|---|---|---|
| `IM_REGISTER_MODE` | `invite` | `closed` / `invite` / `open` |
| `IM_ACCESS_TTL` | 24 小时 | 账号登录的 access token（R-CS-19）；剩余不足 6 小时时 SDK 刷新。管理接口和演示 token 的 7 天不变 |
| `IM_REFRESH_TTL` / `IM_REFRESH_TTL_WEB` | 90 天 / 24 小时 | 滑动续期；H5 放在 `sessionStorage`，所以更短 |
| `IM_REFRESH_ABSOLUTE` | 365 天 | 从登录算的绝对上限 |
| `IM_REFRESH_OVERLAP` | 2 分钟 | 转正后的软拒绝窗口（F-ACC-03 规则 4）；**启动时检查必须大于 SDK 请求超时 30 秒 + 网关超时** |
| `IM_REFRESH_MAX_PENDING` / `IM_REFRESH_ISSUE_RATE` | 4 个 / 每小时 20 次 | 待确认代上限（超出淘汰最旧）；每条会话的签发次数，包括重发和转正后的轮换 |
| `IM_REFRESH_IP_RATE` | 每 IP 每分钟 60 次 | `token/refresh` |
| `IM_SESSION_RECHECK` | 10 分钟，连续失败 3 次断开 | 在线连接的周期复查（F-ACC-03 规则 2） |
| `IM_RECOVERY_OLD_DAYS` | 7 天 | 改密、重新生成后旧恢复码的保留期（F-ACC-05 规则 5） |
| `IM_INVITE_WARN_TOTAL` | 500 | 有效邀请码剩余可用次数合计超过它时配置检查告警 |
| `IM_LOGIN_FAIL_LIMIT` | 5 次 / 15 分钟 | 按"用户名 + IP"（IPv6 按 /64）；超出锁 15 分钟。另有按用户名的渐进延迟 |
| `IM_ARGON_SLOTS` | 4 | 同时计算 Argon2 的数量，约占 256 MiB（m=64 MiB、t=3，和 console-server 一样）。部署要把它算进 `GOMEMLIMIT` |
| `IM_ARGON_RECOVER_SLOTS` / `IM_RECOVER_QUEUE` | 1 个 / 5 秒 | `recover` 专用位和排队时限（F-ACC-05 规则 2） |
| `IM_FILES_ENABLED` | 关 | C2 开关；关时 `/v1/files/*` 返回 404，`send` 的 `image` / `file` 返回 4000 |
| `IM_IMAGE_MAX_BYTES` | 20 MiB | 图片原图 |
| `IM_THUMB_MAX_BYTES` | 512 KiB | 缩略图 |
| `IM_FILE_MAX_BYTES` | 100 MiB | 文件 |
| `IM_AVATAR_MAX_BYTES` | 2 MiB | 头像 |
| `IM_UPLOAD_DAILY_BYTES` | 2 GiB / 人 / 天 | Asia/Shanghai 自然日，`complete` 成功时按实际大小扣；`open` 模式下注册 7 天内的账号 200 MiB |
| `IM_UPLOAD_RATE` / `IM_UPLOAD_PENDING_MAX` | 60 次 / 分钟、20 个 | 申请上传的频率、未完成上传数（R-CS-21） |
| `IM_UPLOAD_COMPLETE_LEASE` | 60 秒 | `complete` 的租约（F-FILE-01 规则 3） |
| `IM_UPLOAD_DAILY_DECLARED` | 每日额度的 4 倍 | 每人每天申请上传的声明字节数合计（F-FILE-01 规则 6a） |
| `IM_FILE_URL_TTL_IMAGE` / `IM_FILE_URL_TTL_FILE` | 120 秒 / 300 秒 | 预签名地址有效期（R-CS-22） |
| `IM_FILE_RETENTION_DAYS` | 180 | 消息文件保留期；0 表示不过期 |
| `IM_UPLOAD_ORPHAN_HOURS` | 24 | 未绑定消息的上传的清理时限 |
| `IM_OFFLINE_PUSH_ENABLED` | 关 | C3 开关。和已有的系统通知开关 `IM_PUSH_ENABLED` 无关 |
| `IM_PUSH_LIVE_WINDOW` | 45 秒 | 超过这个时长没收到帧的连接按不在线（心跳 30 秒） |
| `IM_PUSH_GRACE` / `IM_PUSH_GRACE_DELAY` | 120 秒 / 30 秒 | 断线补推的回看时长和延迟 |
| `IM_PUSH_QUEUE_MAX` | 10000 条消息 | 队列单位是消息，不是设备（R-CB-12） |
| `IM_PUSH_MERGE_SECONDS` | 5 秒 | 同一设备同一会话的合并窗口 |
| `IM_RECALL_WINDOW_SEC` | 120 | 发送者撤回时限 |
| `IM_GROUP_RECALL_WINDOW_SEC` | 604800（7 天） | 群主、管理员撤回时限 |
| `IM_MSG_ACTIONS_ENABLED` | 关 | C4 开关：撤回、引用、@。开启要求 `IM_QUARANTINE_KEY`（R-CB-06） |
| `IM_CONV_SETTINGS_ENABLED` | 关 | C5 开关 |
| `IM_MENTION_MAX` | 20 | 单条消息最多 @ 的人数 |
| `IM_MENTION_ALL_HOURLY` / `IM_MENTION_ALL_DAILY` | 3 / 10 | 每群 @ 所有人的次数 |
| `IM_CONV_PIN_MAX` | 20 | 每人置顶会话数 |
| `IM_MOD_ENABLED` | 关 | C6 开关；关时不检查敏感词，`report` 返回 4030（`reason: featureDisabled`，R-CQ-39） |
| `IM_REPORT_DAILY` | 20 | 每人每天举报次数 |
| `IM_REPORT_EVIDENCE_DAYS` | 180 | 举报证据保留期 |
| `IM_CLIENT_CUSTOM_MSG` | 生产关、测试开 | F-MOD-02 规则 4；生产打开时配置检查显示告警 |

开关的共同规则：新命令在开关关闭时返回 4030（`reason: featureDisabled`）；`send` 带 `quote`、`mentions` 同样；`GET /v1/config` 的 `features` 告诉客户端隐藏入口。开关关闭不影响已有消息的显示和读取。**`IM_V13_ADMIN_FEATURES` 的依赖**（RC2-15，已核实 `app.go` 第 205、267 行）：撤回本身不依赖 V13（隔离加密器只要二者之一开启就初始化）。但后台删除、隔离读取、禁言、封禁都要 V13。所以：C4 的后台 AC 要打开 V13；**`IM_ENV=production` 且 `IM_MOD_ENABLED=1` 时必须 `IM_V13_ADMIN_FEATURES=1`，否则拒绝启动**（举报了也没有手段处理）。

### 4.2 新的 HTTP 接口约定

1. 第一批新增客户端 HTTP 接口，路径前缀 `/v1/`，在对外端口（`IM_ADDR`）上提供；管理接口仍只在管理端口。
2. 请求和响应 JSON；错误 `{code, msg, reason?, details?}`，HTTP 状态取 `errors.json`。错误码新增 `scope: "client"`。
3. 需要登录的接口用 `Authorization: Bearer <token>`，不用 Cookie，token 不进 URL（ARCH S1）。校验和 WS 的 `auth` 相同（4010、4011、4012）。
4. 请求体上限 64 KB。文件内容不经过 im-server。
5. 现有服务端不设 `ReadTimeout`（ARCH 第 7 节，避免断开 WebSocket），`/v1/*` 要在路由层自己设超时（读请求 10 秒，未校准）。
5a. 按 IP 的限制一律用 `gateway/guard.go` 的 `clientIP` 可信代理逻辑（要从 gateway 提出来共用，R-CS-35），IPv6 按 /64 聚合（R-CS-06）。
6. H5 同源访问，沿用 PRD-ui F-UI-03 规则 7。`tianting-web/deploy/nginx.conf` 和 `vite.config.ts` 现在只转发 `/ws`、`/demo/login`，要增加 `/v1/`（R-CF-12）。
7. 生产环境只接受 HTTPS（ARCH S2）。

### 4.3 单聊、群聊发送的判定顺序（在 PRD-core F-MSG-01 规则 6 上扩展）

1. 格式（`image` / `file` / `quote` / `mentions` 的结构，4000）。
2. 去重命中 → 返回第一次的 ack（已保存的消息重发一律成功）。
3. 禁言 4031（不扣令牌）。
4. 发送限流 4290。
5. 单聊：新对象配额、对方存在、好友制、拉黑（F-MSG-01 规则 6 第 5～9 步）；群聊：群存在、成员（4043、4033）。
6. **附件预检**：存在、属于发送者、类型相符（4048、4000 `fileKindMismatch`）。
7. **引用校验**（4040 `reason`）。
8. **@ 所有人的权限**（4030 `reason`）；过滤 `mentions.userIds`。
9. **敏感词**（4510）。
10. （`SaveMessage` 事务内）陌生人限额 4137；**绑定文件**：`UPDATE files SET state='attached', msg_id=? WHERE file_id=? AND owner=? AND state='ready'`，影响 0 行则 4096；**@ 所有人计数**，超出 4293；保存；提交后投递。

第 5～10 步失败都已扣发送令牌（C-29 的"被哪一项拒绝就不扣哪一项"不变），都不保存消息。**新对象配额（R-CB-20）**：第 6～10 步失败时退还第 5 步预扣的 `newPeerReject`，因为对方的存在性已经过了判定、这些失败和探测无关；`newPeerHour` 同样不扣。@ 所有人次数只在消息保存时扣，**撤回不退**（否则发一次撤回一次就绕过限额，R-CB-07）。

新事务的加锁顺序（Postgres，延续 PRD-social 8.3 节）：`conv_seq` 行 → 文件行 → @ 所有人计数行。**账号事务**：`accounts` 行 → `recovery_codes` 行 → `device_sessions` 行（RS6-01）。

### 4.4 AC 标记

沿用已有 PRD：`AC-模块-序号-序号`，【后端】【前端】（三端都要验，只针对某端的写明），未标注的是端到端，（手工）要真机或人工，【C1】等是所属里程碑。[IM 钟] 要注入服务端时钟；[SDK 钟] 要注入 SDK 时钟（sdk-ts 已有 `Clock`，其他两套要有同样的注入点）；[真实钟] 依赖对象存储的真实时间。**预签名一律用真实时钟**（SigV4 和 console 的 objstore 同样的做法），推进 [IM 钟] 不影响签名。负向断言"假通道无调用"一律用哨兵法：之后给另一个离线用户发一条，它被调用后再断言目标无调用（R-CQ-29）。

### 4.5 测试设施（只在 `IM_TEST_MODE=1` 下生效，生产拒绝启动，R-CQ-01）

| 设施 | 要求 |
|---|---|
| 假 S3 | 独立进程，放在 `server/internal/objstore/objstoretest`（console 的是另一模块的 `internal` 包，用不了）。校验预签名、大小、过期、`response-*` 头、CORS、Copy、ETag 条件；可注入：删除失败、不可用、Copy 失败、**HEAD 之后替换对象**（RC2-07）。**假 S3 只验调度逻辑，不证明厂商行为**，厂商能力由 AC-FILE-90-1、90-2 验 |
| 账号并发钩子 | 测试模式可以让请求在 Argon2 计算之后、提交事务之前暂停，用来确定性地构造 F-ACC-04 规则 8 的交错（RS4-06）。生产构建不含这个钩子 |
| 假 APNs、假 FCM | HTTP 层，`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT` 只在测试模式生效。校验 ES256 JWT、必需请求头、OAuth；可注入 410、`UNREGISTERED`、429、503、超时；保存完整请求（含请求头）供断言 |
| 故障注入 | `POST /admin/test/faults` 增加：提及表写失败、隔离写失败、证据写失败、词库加载失败、@ 所有人计数不可用、审计写失败、注册事务失败、吊销写失败、入表前吊销、设置读取失败；已有的不回 ack（`drop-ack`）用于 AC-FILE-07-2；Argon2 阻塞钩子（让闸门占满，不依赖真实耗时） |
| 账号锁定 | `IM_TEST_DISABLE_AUTH_LOCK=1` 关闭账号锁定和 IP 限流（AC-ACC-02-3、05-11） |
| 限流重置 | `/admin/test/limits/reset` 增加 `scope: "auth"`，清"用户名 + IP"、IP、邀请码失败计数 |
| 时钟 | 第 4.4 节；AC 从 Asia/Shanghai 当天 00:00 起算，避免跨日（R-CQ-41） |
| 换 IP | `IM_TRUSTED_PROXIES` + `X-Forwarded-For`（R-CQ-40） |
| 空闲断开 | `IM_TEST_DISABLE_IDLE=1`，只关闭服务端 4408（AC-APN-02-5） |
| SDK | 假 HTTP（现有 FakeTransport 只有 WS）；Play 服务可用性替身；系统通知授权替身 |
| Token | **推送 AC 用账号登录的 token（有 `sid`）**；e2e 其他 AC 继续用演示或管理接口 token（R-CQ-10） |
| 真实存储 | CI 用固定版本 MinIO（`deploy/testenv-ci.sh` 的做法）。开发机不装 MinIO；要不要构建到临时目录按 `backend-ops.md` 的约定，未定前只在 CI 跑 |

---

## 5. C1 账号注册和登录

### F-ACC-01 注册（P0）

**规则**
1. `POST /v1/account/register {username, password, nickname?, inviteCode?, platform, deviceName?}`。`platform` 不合法 400 / 4000；`deviceName` 0～64 个码点，按纯文本显示，不进日志（R-CS-33）。按 `IM_REGISTER_MODE`：`closed` 一律 403 / 4030（`reason: registerClosed`）；`invite` 必须带有效邀请码，否则 403 / 4030（`reason: inviteInvalid`，缺失、错误、过期、用完的应答相同）；`open` 不需要邀请码。
2. **`IM_ENV=production` 且 `IM_REGISTER_MODE=open` 时，必须同时满足 `IM_MOD_ENABLED=1`（C6 上线前这个开关没有实现，所以一律拒绝）、`IM_V13_ADMIN_FEATURES=1`、词库非空、`IM_PUBLIC_OPEN_ACK=1`（表示已完成 CH-23 的确认），否则拒绝启动**（R-CS-09）。人机验证第一批不做，面向公众开放前要补。
3. 用户名：去首尾空白并转小写后 4～32 位，`[a-z0-9_-]`，字母开头（是现有 `userId` 规则的子集）。不合法 400 / 4004。
4. **`userId` 就是规范化后的用户名**，不可修改（CH-02）。**只有这个接口不区分大小写**（R-CB-01）：`users` 表不加唯一约束，C-18（userId 区分大小写）和演示、导入路径完全不变。唯一性靠 `accounts.username_lower` 唯一索引，加 `users` 上 `lower(user_id)` 的非唯一索引，注册事务内同时查两者，已存在 409 / 4095。保留名（`admin`、`root`、`system`、`official`、`tianting`、`support`、`kefu` 等，配置文件维护）也返回 4095。已注销的用户名不能复用。
5. 密码 8～128 个码点；不能等于用户名；不能全是同一个字符；不能在内置弱口令表里（400 / 4000，`reason: passwordWeak`）。用 Argon2id 存储，参数和 console-server 相同（m=64 MiB、t=3，`internal/secret`；那是另一个模块，不能直接引用，im-server 现在也没有 `x/crypto` 依赖）。
6. 昵称 1～32 个码点，不传时等于用户名；C6 上线后要过敏感词（F-MOD-02 规则 2）。
7. **防枚举**（R-CB-19、R-CS-08）：4095 必然暴露用户名是否存在。`invite` 模式先校验邀请码再查用户名，没有有效邀请码的人得不到这个信号。**所有模式**都按 IP 限制注册请求（每 IP 每小时 20 次，未校准，429 / 4290）。同一邀请码 **24 小时内** 4095 达到 20 次（未校准）自动停用并告警，所以一个邀请码每天最多探测 20 个用户名；按窗口计，`maxUses=1000` 的合法邀请码不会因为用户正常撞名累计失效（RC2-31）。这条路径写进 PRD-social 11.2 节的探测预算（第 16 节）。
8. 成功：同一事务写用户和账号、扣减邀请码；响应 `{userId, accessToken, accessExpiresAt, refreshToken, refreshExpiresAt, recoveryCode}`。恢复码的确认和丢失处理见 F-ACC-05 规则 1。
9. 请求参数之外不收集手机号、邮箱、实名信息（CH-23 的默认假设）。

**异常**
- 同一用户名并发注册：只有一个成功，其他 4095。
- 事务失败：邀请码不扣减，用户不产生。
- Argon2 闸门（R-CB-25、R-CS-07）：只用于要算哈希的路径（注册、登录、找回、改密、注销）；`token/refresh`、`logout` 不进。**先过 IP 和用户名限流，再进闸门**，排队最多 1 秒（未校准），超时 503 / 5030。已登录用户的改密留 1 个专用位，匿名请求打满时仍能改密。**`recover` 有自己的专用位，不和改密共用，排队时间更长（未校准）；恢复码命中才用，未命中不进闸门**（RS5-02）。

**验收标准**
- AC-ACC-01-1【后端】【C1】：输入 `IM_REGISTER_MODE=invite`、有效邀请码、用户名 `Alice_1`、合法密码；操作 注册；预期 成功，`userId=alice_1`，返回 token 和恢复码；token 能通过 WS `auth`；错误码 —。
- AC-ACC-01-2【后端】【C1】[IM 钟]：输入 `invite` 模式；缺邀请码、错误邀请码、过期邀请码、用完的邀请码；操作 分别用一个已存在的用户名注册；预期 全部 403，响应体完全相同（不暴露用户名是否存在）；错误码 4030。
- AC-ACC-01-3【后端】【C1】：输入 `IM_REGISTER_MODE=closed`；操作 注册；预期 403，`reason=registerClosed`，用户未创建；错误码 4030。
- AC-ACC-01-4【后端】【C1】：输入 `open` 模式，用户名 `abc`、33 位、`1abc`、`a b c`、含中文；操作 分别注册；预期 都是 400；错误码 4004。
- AC-ACC-01-5【后端】【C1】：输入 `users` 里已有演示用户 `Bob`；操作 注册 `bob`；再用管理接口导入 `BOB`；预期 注册 409；导入成功，`Bob`、`BOB` 并存（C-18、TC-USER-01-16 不变）；错误码 4095。（v0.2 改：R-CB-01、R-CQ-14）
- AC-ACC-01-6【后端】【C1】：输入 保留名 `admin`、已注销的用户名；操作 分别注册；预期 都是 409；错误码 4095。
- AC-ACC-01-7【后端】【C1】：输入 密码 7 位、129 个码点、等于用户名、全相同字符、弱口令表中的一条；操作 分别注册；预期 都是 400，`reason=passwordWeak`（长度问题也用这个 reason）；错误码 4000。
- AC-ACC-01-8【后端】【C1】：输入 同一用户名，20 个并发请求；操作 注册；预期 恰好 1 个成功，其余是 4095 或闸门的 5030；邀请码只扣 1 次；错误码 4095、5030。（v0.2 改：R-CQ-03）
- AC-ACC-01-9【后端】【C6】：输入 `IM_ENV=production`、`IM_REGISTER_MODE=open`，分别缺 `IM_MOD_ENABLED`、缺 `IM_V13_ADMIN_FEATURES`、词库为空、缺 `IM_PUBLIC_OPEN_ACK`；操作 启动 im-server；预期 都非 0 退出，错误信息指出缺哪一条；错误码 —。（v0.3 改：RC2-13、RC2-15）
- AC-ACC-01-10【后端】【C1】：输入 `open` 和 `invite` 两种模式，同一 IP（`X-Forwarded-For`，经可信代理）1 小时内第 21 次注册请求；操作 注册；预期 429；换 IP 不受影响；错误码 4290。（v0.2 改：R-CB-19、R-CQ-40）
- AC-ACC-01-11【后端】【C1】：输入 注册成功；操作 查数据库账号表和服务端日志；预期 表里只有 Argon2id 哈希，没有明文密码和明文恢复码；日志搜不到密码和恢复码特征串；错误码 —。
- AC-ACC-01-12【前端】【C1】：输入 注册页；操作 提交合法信息；预期 进入"保存恢复码"页，显示恢复码、"复制"按钮和"恢复码丢失后账号无法找回"的提示；只有一个"我已保存"按钮，点它之前不能进入会话列表；错误码 —。（v0.2 改：R-CF-10、R-CQ-37）
- AC-ACC-01-13【前端】【C1】：输入 服务端返回 4095；操作 注册；预期 用户名输入框下方提示"这个用户名不能使用"，其他输入保留；错误码 4095。
- AC-ACC-01-14【后端】【C1】：输入 持有一个有效邀请码（maxUses=1），每次换 IP（第 4.5 节）；操作 24 小时内用已存在的用户名注册 20 次，再用未占用的用户名注册；预期 前 20 次 4095；之后邀请码已停用，403；错误码 4095、4030。（v0.3 改：RC2-12）
- AC-ACC-01-15【后端】【C1】：输入 故障注入让第 1 次事务失败；操作 注册；预期 500，用户未创建，邀请码未扣；错误码 5000。
- AC-ACC-01-16【后端】【C1】：输入 Argon2 阻塞钩子让匿名注册占满 4 个闸门位；操作 同时 `token/refresh`、`logout`，以及一次已登录改密；再发一个匿名注册；预期 前三者不受影响；匿名注册等 1 秒后 503；错误码 5030。（v0.3 改：RC2-12）
- AC-ACC-01-17【后端】【C1】：输入 `platform=tv`；`deviceName` 65 个码点；操作 分别注册；预期 都是 400；错误码 4000。
- AC-ACC-01-18【前端】【C1】：输入 服务端依次返回 4004、4000（`passwordWeak`）、4030（`inviteInvalid`）、4290、5030；网络错误；操作 注册；预期 每种显示对应文案（"用户名格式不正确""密码太简单""邀请码无效""操作太频繁，请稍后再试""服务暂不可用""网络异常，请稍后重试"），已填内容保留；错误码 4004、4000、4030、4290、5030。
- AC-ACC-01-19【后端】【C1】：输入 `IM_ENV=production`、`IM_REGISTER_MODE=open`（C6 未上线）；操作 启动；预期 非 0 退出；错误码 —。（RC2-13）
- AC-ACC-01-20【后端】【C2】：输入 `open` 模式（测试环境）注册 3 天的账号；8 天的账号；操作 申请超过 200 MiB 的当日上传；预期 前者 429；后者成功；错误码 4292。（RC2-18）
- AC-ACC-01-21【后端】【C1】[IM 钟]：输入 同一邀请码 24 小时内 19 次 4095，25 小时后再 19 次；操作 注册；预期 邀请码仍有效；错误码 4095。（RC2-31）

### F-ACC-02 登录（P0）

**规则**
1. `POST /v1/account/login {username, password, platform, deviceName?}`，用户名按 F-ACC-01 规则 3 规范化。响应同注册。`recoveryCode`、`recoveryCodeGen` 只在最新一代恢复码还没确认时出现（F-ACC-05 规则 1）。注销冷静期内密码正确：403 / 4030（`reason: accountDeletionPending`，`details.deleteAt`），没有 token（F-ACC-08）。
2. 用户不存在、没有账号记录（演示和导入用户）、密码错误、已注销：**响应相同**，401 / 4018；不存在时也做一次哈希计算，让响应时间不可区分。
3. 失败限制：按"用户名 + IP"累计，15 分钟内 5 次，之后 429 / 4294（`details.retryAfterSeconds`），不自动重试。**不存在的用户名同样累计**，否则第 6 次的差别暴露存在性（R-CS-20）。另外每 IP 每分钟 20 次（4290），防止撞库。不按用户名单独全局锁，否则任何人都能把别人锁死；改为按用户名的**渐进延迟**：累计失败越多，响应越慢，最多 3 秒（未校准，R-CS-06）。计数存数据库（`auth_throttle`），重启不清零，多节点一致（R-CB-16）。**所有校验密码的接口（登录、`password`、`logout-others`、`recovery-code`、`delete`）共用这个计数，都走 Argon2 闸门**（RS4-11）。延迟放在闸门之外；同一用户名同时只算 1 个哈希，其余直接按失败限制返回，防止一个用户名占满闸门（RS4-12）。
3a. 登录失败的日志和事件不记录原始用户名，只记哈希（用户可能把密码误输进用户名框，R-CS-28）。
3b. 新设备登录、改密、找回、**重放吊销、任何一次刷新或转正来自这条会话没见过的 IP 前缀**（F-ACC-03 规则 4，RS4-08）：写一条 `account_events`，在线的其他设备收到 `securityNotice {kind, at, platform}`；"账号安全"页显示最近 20 条（R-CS-06、R-CS-28）。
4. 校验密码通过后才判断封禁：封禁中 403 / 4034（`details.bannedUntil`），文案沿用 F-UI-43。
5. 登录成功创建一条设备会话（F-ACC-03）。不踢其他设备（第 20.2 节）。

**异常**：数据库不可用 503 / 5030。密码哈希参数升级后，旧哈希登录成功时重算（不改变应答）。

**验收标准**
- AC-ACC-02-1【后端】【C1】：输入 正确的用户名和密码（大小写不同的用户名）；操作 登录；预期 成功，token 能连接；错误码 —。
- AC-ACC-02-2【后端】【C1】：输入 不存在的用户名；演示用户（无账号记录）；已注销用户；密码错误；操作 分别登录；预期 都是 401，响应体完全相同；错误码 4018。
- AC-ACC-02-3【后端】【C1】：输入 隔离的测试环境，测试模式关闭账号锁定和 IP 限流，"不存在账号"与"存在账号密码错误"各 50 次；操作 两组交替登录并记录耗时；预期 两组中位数相差 ≤ 5%（和 AC-CON-03-3 同口径）；错误码 4018。（v0.2 改：R-CQ-04、R-CB-27）
- AC-ACC-02-4【后端】【C1】[IM 钟]：输入 同一用户名 + IP 连续 5 次密码错误；操作 第 6 次用正确密码；15 分钟后再试；预期 第 6 次 429，`details.retryAfterSeconds` 存在；15 分钟后成功；错误码 4294。
- AC-ACC-02-5【后端】【C1】：输入 u1 被 IP-A 锁定；操作 从 IP-B 用正确密码登录 u1；预期 成功；错误码 —。
- AC-ACC-02-6【后端】【C1】：输入 封禁中的用户；操作 用错误密码、正确密码分别登录；预期 依次 401 / 4018、403 / 4034 带 `bannedUntil`；错误码 4018、4034。
- AC-ACC-02-7【后端】【C1】：输入 u1 已在 Android 登录；操作 再从 iOS 登录；预期 两边都在线（AC-AUTH-02-5 不变）；错误码 —。
- AC-ACC-02-8【前端】【C1】：输入 服务端返回 4018；操作 登录；预期 提示"用户名或密码错误"，密码框清空，用户名保留；错误码 4018。
- AC-ACC-02-9【前端】【C1】：输入 服务端返回 4294，`retryAfterSeconds=600`；操作 登录；预期 提示"尝试次数过多，请 10 分钟后再试"，不自动重试；错误码 4294。
- AC-ACC-02-10【后端】【C1】：输入 同一 IP 1 分钟内第 21 次登录（不同用户名）；同一 /64 内不同 IPv6 地址；操作 登录；预期 第 21 次 429；/64 内合并计数；错误码 4290。
- AC-ACC-02-11【后端】【C1】：输入 不存在的用户名 + 同一 IP 连续 6 次；操作 登录；预期 第 6 次 429，和存在的用户名行为相同；错误码 4294。
- AC-ACC-02-12【后端】【C1】：输入 用户名框输入特征串 `ZXQ-USER-1`；操作 登录失败后扫描日志、`account_events`；预期 搜不到特征串；错误码 —。
- AC-ACC-02-13【C1】：输入 u1 在 D1 在线；操作 u1 在新设备 D2 登录；预期 D1 2 秒内收到 `securityNotice`，提示"你的账号在新设备登录"；"账号安全"页有这条记录；错误码 —。
- AC-ACC-02-14【后端】【C1】[IM 钟]：输入 u1 被 5 次失败锁定；操作 重启 im-server 后立即用正确密码登录；预期 仍然 429；错误码 4294。

### F-ACC-03 会话续期与设备会话（P0）

**规则**
1. 每次账号登录产生一条**设备会话**。`sid` 是不少于 128 位的随机值，**不进日志、指标标签和后台响应**（`account_events` 存它的哈希，RS4-05）。会话字段：`{sid, userId, platform, deviceName, createdAt, lastSeenAt, currentHash, currentGen, nextGen, pendings（最多 4 个）, overlapUntil, overlapMinGen, rowVersion, absoluteExpiresAt, revokedAt, revokeReason}`。**代号由 `nextGen` 分配，严格单调，永不复用**，否则已作废的 access token 会在代号复用时复活（RS5-05）。access token 增加 `sid` 和代号 `gen` 声明；`POST /admin/users/token` 和 `/demo/login` 签发的 token 没有 `sid`，连接和收发行为和现在一样。
2. `auth` 时除现有的令牌版本号外，有 `sid` 的 token 还要检查：① 会话没有被吊销，已吊销就按 `revokeReason` 映射踢线原因（`credentialsChanged` → `kicked/passwordChanged`，账号注销 → `kicked/accountDeleted`，其余 → `kicked/sessionRevoked`，都是关闭码 4012），**都带 `recoverBefore`**（规则 6a，RS4-07、RS5-03）；② 规则 4 的代号判定。**连接入表后再查一次**，防止吊销和登记之间的竞态（R-CB-17、R-CS-35）。**复查不能照搬 `gateway.go` 第 166 行的 `err == nil &&`：入表后复查出错就用 1011 断开连接（失败关闭，可以重连，不用 4012，否则数据库抖动会把用户踢回登录页，RS5-09）；复查同时覆盖 `sid`、代号（`delivery.Session` 增加 `gen`，RS5-08）、版本号和 `credentials_changed_at`**。踢线放在事务提交之后。在线连接每 10 分钟（`IM_SESSION_RECHECK`）和总线重连后复查一次，防止一次踢线丢失后连接活到 access 到期（RS4-09）；**周期复查连续失败 3 次才用 1011 断开**，复查结果不符就按对应原因踢线。
3. refresh token 格式 `kid.sid.gen.nonce.mac`，`mac` 用**独立的 refresh 密钥**对 `kid`、`sid`、`gen`、`nonce` 计算，不和 JWT 共用密钥（RS5-04）。服务端存整个 token 的哈希。**密钥退役条件是 `IM_REFRESH_ABSOLUTE`（365 天），不是 JWT 的 30 天**，用持久化的密钥台账（`kid`、`retiredAt`）让配置检查拦截提前删除，否则一次运维操作会让全员下线。密钥泄露时的应急操作（主动提前删除并接受全员重新登录）写进运维手册。**先验 `mac`：不对就 401 / 4019（`reason: invalid`），不吊销，不计重放**，所以只知道 `sid` 的人吊销不了会话（RS4-05）。`/v1/account/token/refresh` 每 IP 每分钟 60 次（未校准）。**只有转正才把到期时间推到现在 + `IM_REFRESH_TTL`（H5 用 `_WEB`）**，重发不滑动；到期时间和新 access 的到期时间都不超过 `IM_REFRESH_ABSOLUTE`（RS4-10）。响应带 `absoluteExpiresAt`。
4. **新一代首次使用才生效**（RC2-01，RS4-02、04、08 修正，取代 v0.2 的 30 秒宽限期）。一条会话有一个当前代（`current`）和最多 4 个待确认代（`pending`，未校准）。
   - **用 `current` 刷新**：生成新的一对加入 `pending`，`current` 继续有效。**待确认代只按数量淘汰**：超过 `IM_REFRESH_MAX_PENDING` 时淘汰最旧的，不按年龄丢弃（RS5-14、RS5-21：攻击者不转正本来就用不了待确认代，按年龄丢弃没有安全收益，反而会误吊销已保存新代的客户端）。旧值重试**不限时长**，所以响应丢失、请求发出后被挂起、进程在响应到达前被杀，几分钟后用旧值重试都不会被踢。
   - **首次使用才转正**：用某个 `pending` 的 refresh token 刷新，或者验签通过且未过期的 access token（`gen` 声明）首次 `auth` 或访问 `/v1/*`：它变成 `current`，旧 `current` 和其他 `pending` 全部作废，`overlapUntil` = 现在 + 窗口，`overlapMinGen` = 旧 `current` 的代号。**同时断开这条 `sid` 上代号不是新 `current` 的连接**（`kicked/tokenExpired`，4011，SDK 走规则 7 刷新），这样被攻击者转正时，在线的受害者会马上发现。过期或验签失败的 token 不触发转正。
   - **access token 的代号判定**（`auth` 和 `/v1/*`，RS4-02）：

     | 代号 | 结果 |
     |---|---|
     | 等于 `current` | 放行 |
     | 属于 `pending` | 转正，放行 |
     | 其他（被替换的代、转正前的旧代） | 拒绝，不转正，不吊销。WS：`kicked/tokenExpired`（4011）；HTTP：401 / 4011（`reason: tokenSuperseded`）。SDK 走规则 7 |

     所以攻击者不转正就用不了偷来的令牌，一转正，受害者的旧值就失效。
   - **refresh token 的判定顺序**（RS5-18）：① `mac`（错：`invalid`）；② 会话存在；③ 已吊销：4019，`reason` 按吊销原因，不算重放；④ 已过期：4019；⑤ 代号；⑥ **签发频率限制只在将要签发 token 的分支检查**，`mac` 错误、已吊销、已过期、`superseded`、`replayDetected` 都不受它影响，否则攻击者用满次数就能让受害者到不了代号判定（RS6-05）。`mac` 正确但既不是 `current` 也不是 `pending` 时：**只有代号 ≥ `overlapMinGen` 且在 `overlapUntil` 之前**，才 401 / 4019（`reason: superseded`，`retryAfterSeconds`），不吊销，不发 token；**其他情况一律按重放处理**：吊销这条设备会话，401 / 4019（`reason: replayDetected`）。
   - **连续转正不能续开窗口**（RS5-01）：攻击者每次转正，`overlapMinGen` 都前移，受害者手里的旧值马上低于它，立即触发重放吊销。`gen` 受 `mac` 保护，改不了。
   - 服务端用条件更新（`UPDATE … WHERE sid=? AND row_version=? AND revoked_at IS NULL`，影响 0 行就重新读一次再判断），并发时不会出现两条链（RC2-04）。同一会话每小时签发超过 20 次（重发和转正后的轮换都算，未校准）：429 / 4290，不吊销；**写 `account_events`，连续 6 小时（未校准）触发时给其他在线设备 `securityNotice`**，防止持旧值的人静默压制受害者（RS5-14）。
   - **不用确定性派生 token**（重试返回同一个值）：攻击者和受害者会落在同一条链上，检测失效（RS4-04）。
   - **理由**：v0.2 的"30 秒宽限期"在弱网和切后台下会误吊销，每台设备约 18 小时刷新一次，这是常见路径；吊销还会同时删除推送令牌，用户不打开 App 就不再收到通知，也不知道原因。
   - **代价**：① 旧 refresh token 的有效期延长到新一代首次使用；② 窗口内迟到或重放的请求不触发吊销（攻击者拿不到新凭证）；③ 转正后晚于窗口才到达的重复请求仍然会误吊销；窗口内连续转正 2 次时，更早的重复请求也会误吊销。频率未测（第 21 节 4a），从 `im_refresh_replay_total` 看。
   - **检测盲区（如实写，RS4-08）**：**受害者卸载或换手机且没有退出时，旧值再也不会被使用，攻击者转正后可以滑动续期到绝对上限（365 天），不会触发重放检测**。受害者只有一台设备时，`securityNotice` 没有接收方。缓解："账号安全"页显示其他会话的数量和最近活跃时间（不做设备列表，只给"退出其他设备"提供依据）；每次刷新和转正都比较 IP 前缀，新前缀写 `account_events`（弱网切换也会触发，所以只提醒不吊销）。
   - **检测和提示**：`im_refresh_reissue_total`（重发次数）、`im_refresh_replay_total`（重放吊销次数）；受害者还在用这台设备时，攻击者转正后受害者下次上线（在线时立刻）就触发吊销；**受害者设备弹窗"检测到登录凭证异常，这台设备已下线。如果不是你本人操作，请尽快改密码"**，并写 `account_events`、给其他在线设备 `securityNotice`。误吊销也能从指标看到。
5. SDK 行为：access token 剩余不足 6 小时（`IM_ACCESS_TTL` 的 1/4）时刷新。触发时机：启动和回到前台时检查；已连接时用定时器；收到 `kicked/tokenExpired`。**这四个入口合并成一次（单飞），同一时间只有一个刷新请求；请求超时 30 秒（未校准，要小于重叠窗口）；超时放弃的请求，迟到的响应直接丢弃**（RC2-04）。**改密、找回、退出其他设备和刷新放进同一个串行队列**（RS4-03）。启动时已过期 → **先刷新，再发 `auth`**。连接中换 token：用 1000 断开后立即重连（持续不足 1 秒不显示连接提示，F-CONN-02 不变）。**新 token 先落盘再使用**（R-CF-09）。**队列内的改密如果要先刷新 access，直接执行，不重新排队**（RS5-15）。access 到期时间等于 `absoluteExpiresAt` 时，SDK 不再主动刷新（RS5-11）。**HTTP 和 WS 共用同一个 token 来源**（RS5-10）。
6. 刷新失败的处理（R-CF-09、RS4-07、RS4-10、RS5-01）：4019 按 `reason`：`superseded` 留在当前页面，重读本地最新 token，按 `retryAfterSeconds` 重试，**最多 3 次，仍然 `superseded` 就回登录页**；`replayDetected` 按规则 4 弹窗；其他 reason 按规则 6a 提示后回登录页。403 / 4034 显示封禁文案（F-UI-43）；**429 / 4290、网络错误和 5xx 留在当前页面**（原生端显示本地缓存）、退避重试，不回登录页。
6a. **找回截止日期 `recoverBefore`**（RS5-03）：**不跟吊销原因走，每次按账号状态算**：账号存在未过期、没用过、没作废的已确认旧码时，取其中最早的 `retire_until`；没有就不带，所以找回成功后不再提示。受害者持有的码比这个更新时，显示的日期会偏早，偏早是安全的方向。`kicked`（`passwordChanged`、`sessionRevoked`、`accountDeleted`）和 4019（`mac` 正确时）都带。客户端统一按这个字段提示：有日期时"你的账号凭证有变化。如果不是你本人操作，请在 {日期} 前用恢复码找回"，没有日期时按原因显示现有文案。**所有路径（在线、离线、先退出其他设备再改密、改密后注销）都走这一条**。多次改密不会把日期推后，所以不会让受害者以为还有时间。
7. `kicked/tokenExpired`（4011）：账号模式下 SDK 先尝试刷新一次，成功就重连，不弹窗；4019 才按 AC-AUTH-02-4 处理。演示模式没有 refresh token，行为不变。**收到 4011 先比较本地 token 的代号：本地比被拒绝的那个新，就直接用本地值重试或重连，不刷新**，否则换 token 瞬间的在途请求会不断触发刷新（RS5-10）。三套 SDK 现在收到 4011 都不重连，要改；`errors.json` 的 4011 动作同步改（第 11.5 节）。
8. 存储：Android 用现有 Keystore 加密文件、iOS 用现有 Keychain，Session 增加 `refreshToken` 和 `mode`（`account` / `demo`）；H5 沿用 D4，只放 `sessionStorage`，**关闭标签页后要重新登录（CH-26）**。天听不主动持久化 token；浏览器崩溃恢复是否会恢复 `sessionStorage` 并写入磁盘未核实，实现时实测（第 21 节，RS4-13）。老版本留下的 Session 没有 `mode`，按 `demo` 处理，token 到期后回登录页（R-CF-11）。
9. H5 多标签页：`sessionStorage` 被复制到新窗口时（复制标签页的表现未核实），两个页面共用 refresh token 会互相触发重放。启动时用 `navigator.locks` 按 `sid` 取互斥锁（不依赖对方页面响应，原页面被浏览器冻结也有效；兼容性未核实，没有这个接口时退回 `BroadcastChannel`），**拿不到锁的页面清除 token，回登录页**；刷新前再确认一次（R-CF-09、RS4-13）。
10. 封禁和强制下线已经吊销全部会话，所以 refresh 一律返回 4019，不返回 4034；用户下一次登录才看到封禁提示（R-CQ-16）。第 6 条的 4034 只是 SDK 对将来可能返回的兜底，第一批服务端不返回。
11. **没有 `sid` 的 token**（R-CS-02、R-CB-17）：账号表增加 `credentials_changed_at`；`auth` 和 `/v1/*` 拒绝 `iat` 早于它的无 `sid` token（`kicked/passwordChanged`，4012）。不递增 `token_version`，所以当前设备不会被一起踢。无 `sid` 的 token 调用 `/v1/account/` 下的 `password`、`logout`、`logout-others`、`recovery-code`、`delete`、`profile`：403 / 4030（`reason: accountSessionRequired`）。
12. 第一批不提供设备列表页面（后续批"多端同时在线"），提供"退出其他设备"（F-ACC-04 规则 6）。
13. H5 会话短 TTL，不主动退出也会到期；`session_purge` 每天清除过期 30 天以上的设备会话（R-CF-09、R-CB-23）。

**异常**：刷新时用户已注销：4019。数据库不可用：503 / 5030，按规则 6 不回登录页。

**验收标准**
- AC-ACC-03-1【后端】【C1】：输入 有效 refresh token；操作 刷新；预期 返回新 access、新 refresh；新 access 能连接；错误码 —。
- AC-ACC-03-2【后端】【C1】[IM 钟]：输入 R1 已刷新得到 R2，R2 已使用（转正），推进 3 分钟；操作 再用 R1 刷新，然后用 R2 这一代的 token 刷新；预期 R1 请求 401，`reason=replayDetected`；这条设备会话被吊销，R2 也 401，它的 access token `auth` 收到 `kicked/sessionRevoked`；错误码 4019。（RS4-04 改）
- AC-ACC-03-3【后端】【C1】：输入 R1 已刷新得到 R2（R2 没有用过）；操作 用 R1 再重试 4 次，R2 被淘汰；再用最新一代；用 R2 刷新；预期 R1 重试都成功，会话不被吊销；最新一代转正；R2 窗口内 `superseded`，窗口外按重放吊销；错误码 4019。（RS5-14 改）
- AC-ACC-03-4【后端】【C1】[IM 钟]：输入 refresh token 91 天没有使用；操作 刷新；预期 401；错误码 4019。
- AC-ACC-03-5【后端】【C1】：输入 `POST /admin/users/token` 签发的 token；操作 连接、发消息；查声明；预期 没有 `sid`，和现在行为一致；错误码 —。
- AC-ACC-03-6【后端】【C1】：输入 用户被强制下线；被封禁；操作 分别用之前的 refresh token 刷新；预期 都是 401（不是 4034）；错误码 4019。（v0.2 改：R-CQ-16）
- AC-ACC-03-7【前端】【C1】（Android、iOS）[SDK 钟]：输入 access token 已过期、refresh 有效，App 冷启动；操作 观察帧和页面；预期 没发 `auth` 之前先发了刷新请求；不出现登录页和弹窗，进入会话列表并连接成功；错误码 —。（v0.2 改：R-CQ-17）
- AC-ACC-03-8【前端】【C1】（Android、iOS）：输入 refresh token 已被吊销；操作 冷启动；预期 回登录页，提示"登录已失效，请重新登录"；错误码 4019。
- AC-ACC-03-9【前端】【C1】（H5）：输入 账号登录的标签页；操作 刷新页面；关闭后新开标签页；检查 `localStorage`；预期 刷新后仍登录；新标签页回登录页；`localStorage` 里没有 token；错误码 —。
- AC-ACC-03-10【前端】【C1】[SDK 钟]：输入 连接中的账号会话，access token 还有 6 小时 1 分钟；操作 把 SDK 钟推进 2 分钟；预期 自动刷新、以 1000 断开并重连，不弹窗，不出现连接提示；到期后消息照常收发；错误码 —。（v0.2 改：R-CQ-17）
- AC-ACC-03-11【前端】【C1】（Android、iOS）[SDK 钟]：输入 飞行模式，access token 已过期，本地有缓存；操作 冷启动；预期 进入会话列表显示缓存和"网络不可用"，不回登录页；恢复网络后自动刷新并连接；错误码 —。
- AC-ACC-03-12【前端】【C1】：输入 刷新接口返回 503；连接中收到 `kicked/tokenExpired` 且刷新成功；操作 观察；预期 前者退避重试，不回登录页；后者不弹窗，自动重连；错误码 5030。
- AC-ACC-03-13【前端】【C1】（Android、iOS）：输入 刷新响应已到达，新 token 落盘后、使用前杀掉 App；操作 重新启动；预期 用新 token，能正常登录，会话没有被吊销；错误码 —。
- AC-ACC-03-14【前端】【C1】（H5）：输入 已登录的页面，分别用脚本 `window.open` 打开同源新窗口（`sessionStorage` 被复制）、浏览器"复制标签页"、原页面被冻结时复制；操作 新页面启动；预期 新页面回登录页；原页面不受影响，没有 401；错误码 —。（RS4-13 改）
- AC-ACC-03-15【后端】【C1】：输入 refresh token 来自 `platform=web` 的登录；操作 查到期时间；连续刷新到超过绝对上限（[IM 钟]）；预期 到期时间是现在 + 24 小时；超过 365 天后 401；错误码 4019。
- AC-ACC-03-16【后端】【C1】[IM 钟]：输入 账号用户 u1 另外持有管理接口签发的 token T（无 `sid`）；操作 u1 改密后用 T `auth`；改密之后签发的无 `sid` token 连接；用它调用 `/v1/account/password`；预期 T 收到 `kicked/passwordChanged` 并以 4012 关闭；新 token 可以连接；调用 password 返回 403，`reason=accountSessionRequired`；错误码 4012、4030。
- AC-ACC-03-17【后端】【C1】：输入 故障注入在连接入表前吊销 `sid`；操作 连接完成；预期 连接随后收到 `kicked/sessionRevoked`，没有留在线；错误码 4012。
- AC-ACC-03-18【前端】【C1】（Android、iOS）：输入 App 里有老版本留下的 `{userId, token}`；操作 升级后启动；预期 按演示会话处理，能连接；"账号安全"入口隐藏；错误码 —。
- AC-ACC-03-19【后端】【C1】[IM 钟]：输入 R1 刷新后响应丢失；操作 推进 5 分钟、1 小时、3 天后依次用 R1 重试；预期 都成功，会话不被吊销，推送令牌还在，`im_refresh_reissue_total` 增加；错误码 —。（RC2-01）
- AC-ACC-03-20【前端】【C1】（Android、iOS）：输入 刷新请求发出后、响应到达前杀掉 App；操作 5 分钟后重新启动；预期 用旧 refresh token 重试成功，不回登录页；错误码 —。（RC2-01）
- AC-ACC-03-21【后端】【C1】：输入 同一 R1 的 2 个并发请求；操作 刷新，然后用其中一个结果；预期 都不吊销；两个结果都是 `pending`；先使用的转正，另一个随即作废；数据库里只有一条链；错误码 —。（RS4-04 改）
- AC-ACC-03-22【后端】【C1】[IM 钟]：输入 R1 在 1 分钟内连续重试 3 次，得到 R2、R3、R4；操作 用 R4 的 access token `auth`；推进 3 分钟后再用 R1；预期 转正后 R1 按重放处理；重试过程会话不被吊销；错误码 4019。（RS4-04 改）
- AC-ACC-03-23【前端】【C1】[SDK 钟]：输入 启动、回前台、定时器、`kicked/tokenExpired` 同时触发；第一个请求超时放弃后重试，迟到响应到达；操作 观察；预期 同一时间只有 1 个刷新请求；迟到响应被丢弃，本地保存的是最后一次的值；错误码 —。（RC2-04）
- AC-ACC-03-24【后端】【C1】[IM 钟]：输入 R1 刷新得到 R2 和新 access A2；操作 用 A2 `auth`；推进 3 分钟后再用 R1；预期 `auth` 成功且 `current` 转为新一代；R1 按重放处理；错误码 4019。（RS4-04 改）
- AC-ACC-03-25【后端】【C1】[IM 钟]：输入 被盗场景：攻击者用 R1 重发得到 R3 并使用，推进 3 分钟；操作 受害者上线用 R1 或 R2；预期 401，`reason=replayDetected`；`im_refresh_replay_total` 增加；`account_events` 有记录；受害者其他在线设备收到 `securityNotice`；错误码 4019。（RS4-04 改）
- AC-ACC-03-26【前端】【C1】：输入 刷新返回 4019，`reason=replayDetected`；操作 观察；预期 回登录页前弹窗提示"检测到登录凭证异常…"；错误码 4019。（RC2-01）
- AC-ACC-03-27【后端】【C1】[IM 钟]：输入 过期 31 天的设备会话；操作 运行 `session_purge`；预期 行被清除，绑定的令牌被清除；错误码 —。（RC2-24）
- AC-ACC-03-28【后端】【C1】：输入 R1 连续刷新 5 次，第一次的 (A2, R2) 被淘汰；操作 用 A2 `auth`、访问 `/v1/*`；预期 分别收到 `kicked/tokenExpired`、401 `reason=tokenSuperseded`；没有转正，R1 仍有效；错误码 4011。（RS5-14 改）
- AC-ACC-03-29【后端】【C1】：输入 `pending` 代的 access token，已过期；签名错误；操作 `auth`；预期 分别 4011、4010，`current` 都不变；错误码 4011、4010。（RS5-20 改）
- AC-ACC-03-30【后端】【C1】：输入 受害者用 A1 在线；攻击者用 R1 刷新并用新 access 转正；操作 观察受害者连接，受害者重连并刷新；预期 受害者连接收到 `kicked/tokenExpired`；A1 再 `auth` 被拒绝；用 R1 刷新：窗口内 `reason=superseded`，窗口后 `replayDetected` 并吊销；错误码 4011、4019。（RS4-02、RS4-08）
- AC-ACC-03-31【后端】【C1】[IM 钟]：输入 Q2(R1) 先到得到 R3，Q1(R1) 1 分钟后到得到 R4；操作 用 R3 的 access 转正；窗口内用 R1、R4 刷新；推进 3 分钟后再用 R4；预期 转正成功，会话不被吊销；窗口内 401 `reason=superseded`，不发 token；窗口后按重放吊销；错误码 4019。（RS4-04）
- AC-ACC-03-32【后端】【C1】：输入 只知道 `sid`，构造 `kid.sid.gen.nonce.错误mac`；操作 请求 `token/refresh`、`logout`；预期 都 401 `reason=invalid`；会话没有被吊销，`im_refresh_replay_total` 不变，没有异常弹窗事件；错误码 4019。（RS4-05）
- AC-ACC-03-33【后端】【C1】：输入 跑完登录、刷新、退出、重放的日志和 `/admin/accounts/{userId}` 响应；操作 搜索 `sid` 原值；预期 搜不到；错误码 —。（RS4-05）
- AC-ACC-03-34【C1】[IM 钟]：输入 u1 离线（access 未过期、已过期两种），攻击者改密；操作 u1 上线；预期 前者收到 `kicked/passwordChanged`，后者刷新得到 4019 `reason=credentialsChanged` 和 `recoverBefore`；两者都弹窗显示找回截止日期；错误码 4012、4019。（RS4-07）
- AC-ACC-03-35【后端】【C1】：输入 第一次刷新和转正来自新的 IP 前缀；操作 刷新，查 `account_events`、"账号安全"响应；预期 有事件；响应带其他会话数量和最近活跃时间；错误码 —。（RS4-08）
- AC-ACC-03-36【后端】【C1】：输入 入表后复查时数据库报错；操作 观察连接；预期 以 1011 断开，SDK 重连，不回登录页；错误码 —。（RS5-09 改）
- AC-ACC-03-47【后端】【C1】[IM 钟]：输入 吊销事件被故障注入丢弃；另一条连接周期复查时数据库报错 1 次、3 次；操作 推进时钟；预期 前者下一次复查后被 4012 踢；后者 1 次不断开，连续 3 次后 1011 断开；错误码 4012。（RS4-09、RS5-09）
- AC-ACC-03-48【后端】【C1】[IM 钟]：输入 攻击者转正后用自己的 `current` 把这个小时的 20 次签发用完，推进 3 分钟；操作 受害者用 R1 刷新；预期 立即 `reason=replayDetected`，不是 4290；会话被吊销；错误码 4019。（RS6-05）
- AC-ACC-03-37【C1】[IM 钟]：输入 同一会话第 21 次重发；连续重发 100 天不转正；刷新时距绝对上限不足 24 小时；操作 观察；预期 SDK 收到 4290 退避，不回登录页；100 天后到期（重发不滑动）；新 access 到期不超过绝对上限；错误码 4290、4019。（RS4-10）
- AC-ACC-03-38【后端】【C1】[IM 钟]：输入 攻击者用 R1 转正 gen2，1 分 50 秒后转正 gen3；操作 受害者在 gen3 转正后用 R1 刷新；预期 立即 `reason=replayDetected`，会话吊销，攻击者的 token 也失效；错误码 4019。（RS5-01）
- AC-ACC-03-39【前端】【C1】[SDK 钟]：输入 刷新返回 `superseded`，`retryAfterSeconds=120`；连续 3 次都是；操作 观察；预期 留在当前页面，按时间重试；第 3 次后回登录页；错误码 4019。（RS5-01）
- AC-ACC-03-40【后端】【C1】：输入 攻击者持 R1 得到 A2、A3；受害者转正 gen2（A3 作废），再轮换一代并转正；操作 用 A3 `auth`；查新一代的代号；预期 A3 一直被拒绝；新代号大于 A3；错误码 4011。（RS5-05）
- AC-ACC-03-41【后端】【C1】：输入 连接校验 gen=1 后暂停，另一请求转正 gen=2；操作 连接入表；预期 入表后复查发现代号不符，收到 `kicked/tokenExpired`；错误码 4011。（RS5-08）
- AC-ACC-03-42【前端】【C1】：输入 换 token 瞬间有 3 个在途 `/v1/files/urls`，带旧 token 得到 4011；操作 观察；预期 用本地新 token 重试成功，没有多发刷新请求；错误码 —。（RS5-10）
- AC-ACC-03-43【前端】【C1】[SDK 钟]：输入 距绝对上限 5 小时刷新，响应的 access 到期等于 `absoluteExpiresAt`；操作 推进 1 小时；预期 不再刷新，不重连；到期后回登录页；错误码 —。（RS5-11）
- AC-ACC-03-44【后端】【C1】：输入 旧代值遇到 `credentialsChanged` 吊销的会话；`logout` 带被替换的 Bearer 和有效 refresh；操作 分别请求；预期 前者 `reason=credentialsChanged`，`im_refresh_replay_total` 不变；后者退出成功；错误码 4019。（RS5-18）
- AC-ACC-03-45【后端】【C1】[IM 钟]：输入 持 R1 的人连续 6 小时每小时刷满 20 次；操作 查 `account_events`，观察 u1 其他在线设备；预期 有限流事件；其他设备收到 `securityNotice`；错误码 4290。（RS5-14）
- AC-ACC-03-46【后端】【C1】：输入 refresh 密钥轮换后的旧 token；配置检查里把未满 365 天的旧 refresh 密钥删除；操作 刷新；运行配置检查；预期 旧 token 刷新成功；配置检查报错；错误码 —。（RS5-04）

### F-ACC-04 退出、改密与下线（P0）

**规则**
1. 退出登录（F-AUTH-05、F-UI-16 不变）增加：先调用 `POST /v1/account/logout {refreshToken?}`（Bearer **或** refresh token，幂等，RC2-06：access 可能已过期），吊销当前 `sid` 和它的推送令牌，再用 1000 断开。Bearer 和 `refreshToken` **有一个有效就执行**，Bearer 过期或被替换不影响（RS5-18）；`refreshToken` 的 `mac` 必须正确（RS4-05），是任何一代的值都按退出处理，不算重放。**网络不通时**：仍然本地退出，并且 ① 调用系统接口注销令牌（FCM `deleteToken`、`unregisterForRemoteNotifications`）；② 把 `sid` 和 refresh token 存为"待退出"，下次联网时补发（只保留这两项，不算登录状态，R-CB-23、R-CF-08、R-CS-03）。
2. 改密 `POST /v1/account/password {oldPassword, newPassword}`：旧密码错误 401 / 4018（计入 F-ACC-02 规则 3 的失败次数）；新密码规则同注册。
3. 改密成功（RS4-03）：**吊销这个账号的全部设备会话（包括当前 `sid`），给当前设备发一个新 `sid`**，响应返回新 token 和新一代恢复码，所以和当前设备共用 `sid` 的攻击者也被清除。旧会话 `revokeReason=credentialsChanged`，旧 refresh 返回 4019（`reason: credentialsChanged`，`recoverBefore`），**不算重放，不弹凭证异常**；连接收到 `kicked/passwordChanged`（4012），令牌全部删除；写 `credentials_changed_at`（F-ACC-03 规则 11）。**当前设备**：SDK 在请求进行中把这个踢线缓存起来，等响应（超时 30 秒，RS5-15）：成功就先换新 token，再忽略旧 `sid` 的踢线，重连，1 秒内不显示连接提示；失败或超时才按踢线处理。响应丢失时，用新密码重新登录。**换 `sid` 后 SDK 重新注册推送令牌**，"显示发送者和内容"的设置随当前设备复制到新 `sid`（这是用户自己的选择；找回没有旧设备可复制，默认关，RS5-12）。**新 token 和新恢复码在确认保存前只放内存**（F-ACC-05 规则 1）。恢复码处理见 F-ACC-05 规则 5：**旧码保留 7 天，不立即作废**（RC2-02）。进入"保存恢复码"页。其他设备的弹窗按 F-ACC-03 规则 6a。
4. 现有强制下线（F-AUTH-03 规则 2）和封禁（F-ADM-13）同时吊销该用户全部设备会话和推送令牌。
5. 不认识 `passwordChanged`、`sessionRevoked` 的老 SDK 按关闭码 4012 处理（"账号已被下线"）。
6. "账号安全 → 退出其他设备"：`POST /v1/account/logout-others`，要当前密码，效果同规则 3 的吊销和换 `sid`（`revokeReason=loggedOutOthers`），不改密码（R-CS CH-04、RS4-03）。其他设备收到 `kicked/sessionRevoked`。
7. **不变量**：`sid` 吊销或过期时，同一事务删除绑定的推送令牌（F-APN-01 规则 4）。
8. **凭证版本**（RS4-06）：`accounts` 增加 `credentials_version`，改密、找回时递增。所有用密码的操作（登录、改密、退出其他设备、重新生成恢复码、申请注销、撤销注销），Argon2 计算在事务外，**提交事务的第一条语句都锁 `accounts` 行（`SELECT … FOR UPDATE`，或不改值的条件 UPDATE），持锁后再校验 `credentials_version`、调用方的 `sid` 没有吊销、账号状态没有变化**，不只是写恢复码的事务（RS7-01，否则 `logout-others` 或登录能在 `recover` 提交时留下看不见的新会话）；不匹配返回 401 / 4018，**不计入失败次数**。**`recover` 不做版本条件判断**（RS5-02，否则攻击者循环改密就能让受害者的找回一直失败）：**提交事务的第一条语句先锁 `accounts` 行并递增版本（不带版本条件）**，再原子检查"这一行恢复码没用过、没作废、没过期"（`UPDATE … WHERE used_at IS NULL AND revoked_at IS NULL AND 未过期`），然后作废更新的代。同一个码并发只有一个成功；在途的改密要么在它之前提交（新代随后被作废），要么随后失败，不会留下攻击者手里的有效新码（RS6-01，否则 Postgres READ COMMITTED 下改密能插进恢复码检查和版本递增之间）。所有写恢复码的事务都按第 4.3 节的顺序加锁。`account_cleanup` 第 ① 步的条件要包含"注销申请没有被撤销"。这样在途的请求不会覆盖受害者的找回。

**异常**：令牌删除或吊销写库失败：改密整体回滚，返回 5000，旧密码仍有效。D1 改密时 D2 正在刷新：刷新在吊销之后的返回 4019。

**验收标准**
- AC-ACC-04-1【后端】【C1】：输入 u1 在设备 D1、D2 登录；操作 D1 退出；预期 D1 的 refresh 不可用（4019），D1 的 access 再 `auth` 收到 `kicked/sessionRevoked`；D2 不受影响；错误码 4019。
- AC-ACC-04-2【后端】【C1】：输入 u1 在 D1、D2 在线；操作 D1 改密；预期 D1 的旧 `sid` 被吊销，响应的新 token 带新 `sid` 且可用，D1 1 秒内重连且不显示连接提示；D2 收到 `kicked/passwordChanged` 并以 4012 关闭；D1、D2 的旧 refresh 都 401，`reason=credentialsChanged`；错误码 4019。（RS4-03 改）
- AC-ACC-04-3【后端】【C1】：输入 旧密码错误；操作 改密；预期 401，密码不变，其他设备不受影响；错误码 4018。
- AC-ACC-04-4【后端】【C1】：输入 u1 有两条设备会话；操作 管理接口强制下线；预期 两条都吊销，两个 refresh 都 401；错误码 4019。（v0.2 改：R-CQ-18，推送令牌部分移到 AC-APN-01-3）
- AC-ACC-04-5【前端】【C1】：输入 D2 收到 `kicked/passwordChanged`，带 `recoverBefore`；操作 观察；预期 不重连，弹窗显示这个日期，确定后回登录页并清空本地数据；错误码 —。（RS5-03 改）
- AC-ACC-04-6【前端】【C1】（Android、iOS）：输入 网络断开时点退出；操作 确认退出，再恢复网络；预期 本地立即退出并回登录页，token 被清除，调用了系统的令牌注销；恢复网络后服务端收到 logout，`sid` 被吊销；错误码 —。（v0.2 改：R-CF-08、R-CS-03）
- AC-ACC-04-7【后端】【C1】：输入 故障注入让吊销写库失败；操作 改密；预期 500，旧密码仍有效，其他设备不受影响；错误码 5000。
- AC-ACC-04-8【后端】【C1】：输入 D2 刷新请求和 D1 改密并发；操作 观察；预期 刷新要么在吊销前成功然后新 token 被吊销，要么 401；不存在改密后仍有效的 D2 会话；错误码 4019。
- AC-ACC-04-10【C1】：输入 u1 在 D1、D2 在线；操作 D1 "退出其他设备"，密码错误 / 正确；预期 错误时 401，无变化；正确时 D2 收到 `kicked/sessionRevoked`，D1 换新 `sid` 后 1 秒内重连，不显示连接提示，密码不变；错误码 4018。（RS4-03 改）
- AC-ACC-04-11【后端】【C1】：输入 logout 请求重复 2 次；操作 请求；预期 都成功；错误码 —。
- AC-ACC-04-12【后端】【C1】：输入 新密码不合规；操作 改密；预期 400，密码和恢复码都不变；错误码 4000。
- AC-ACC-04-13【后端】【C1】[IM 钟]：输入 u1 恢复码 C1，改密后得到 C2；操作 第 6 天用 C1 找回；另一账号第 8 天用 C1 找回；用 C2 找回；预期 第 6 天成功，C2 随即作废（旧码优先）；第 8 天 401；没有用过 C1 时 C2 可用；错误码 4018。（RS4-01 改）
- AC-ACC-04-14【后端】【C1】：输入 待退出：access 已过期，只有 refresh token；操作 `POST /v1/account/logout {refreshToken}`；用已作废的旧代值（`mac` 正确）；预期 都成功，`sid` 被吊销，没有按重放处理；错误码 —。（RS4-05 改）
- AC-ACC-04-15【后端】【C1】：输入 攻击者偷到 D1 的 refresh 并转正，和 D1 共用 `sid1`，保持连接；操作 D1 改密（access 未过期）；预期 `sid1` 被吊销；攻击者的连接被踢，refresh 和 access 都不可用；D1 拿到新 `sid`；错误码 4012、4019。（RS4-03）
- AC-ACC-04-16【C1】：输入 D1 改密已提交，响应丢失；操作 D1 用旧 refresh 刷新；预期 4019 `reason=credentialsChanged`，不弹"凭证异常"，`im_refresh_replay_total` 不变；按规则 6a 提示后回登录页；错误码 4019。（RS4-03，RS5-20 改）
- AC-ACC-04-17【前端】【C1】：输入 定时刷新和改密同时触发；操作 观察请求顺序；预期 SDK 串行发送，没有误吊销；错误码 —。（RS4-03）
- AC-ACC-04-18【后端】【C1】：输入 攻击者登录请求在 Argon2 之后暂停（第 4.5 节钩子），受害者 `recover` 提交；操作 放行登录；预期 登录 401，`recover` 之后这个账号只有受害者的会话；错误码 4018。（RS4-06）
- AC-ACC-04-19【后端】【C1】：输入 攻击者改密请求在 Argon2 之后暂停，受害者 `recover` 提交；操作 放行改密；预期 改密 401；受害者的新密码和新会话不变；攻击者没有拿到恢复码；错误码 4018。（RS4-06）
- AC-ACC-04-20【后端】【C1】：输入 同一个恢复码 2 个并发 `recover`；操作 请求；预期 恰好一个成功，另一个 401；错误码 4018。（RS4-06）
- AC-ACC-04-21【后端】【C1】：输入 冷静期内 `recover` 撤销注销，同时一个 `delete` 在途；操作 放行 `delete`；运行 `account_cleanup`；预期 `delete` 401，账号不被清理；错误码 4018。（RS4-06）
- AC-ACC-04-22【后端】【C1】：输入 持有 access 的人连续 5 次用错误密码调 `logout-others`、`recovery-code`、`delete`；操作 第 6 次用正确密码；预期 429；和登录共用计数；错误码 4294。（RS4-11）
- AC-ACC-04-23【后端】【C1】：输入 攻击者有密码，脚本循环调 `password` 和登录；受害者持有效旧码；操作 受害者 `recover`；预期 第一次就成功，攻击者的会话吊销，在途的改密 401；错误码 —。（RS5-02）
- AC-ACC-04-24【后端】【C1】：输入 同一用户名 + IP 已锁定；同一用户名正在算哈希；其他用户循环改密占住改密专用位；操作 用正确恢复码 `recover`；预期 成功；错误恢复码仍受失败限制；错误码 —。（RS5-02）
- AC-ACC-04-25【C1】[IM 钟]：输入 C1 第 0 天换代。序列甲：攻击者先 `logout-others` 再改密；乙：改密后 `delete`；丙：第 5 天再改一次密；丁：受害者在线时 `logout-others`；操作 受害者上线或在线观察；预期 `kicked` 或 4019 都带 `recoverBefore`，都是第 7 天；客户端都显示这个日期；错误码 4012、4019。（RS5-03）
- AC-ACC-04-26【前端】【C3】（Android、iOS）：输入 打开了"显示内容"的设备改密；操作 观察令牌注册，发一条离线消息；预期 换 `sid` 后重新注册，设置保留，能收到通知；错误码 —。（RS5-12）
- AC-ACC-04-27【前端】【C1】[SDK 钟]：输入 改密请求：响应一直不来；响应晚于 30 秒；失败；响应先到踢线后到；操作 观察；预期 前三种提示失败，缓存的踢线随后生效，队列不堵；最后一种不弹窗，正常重连；错误码 —。（RS5-15）
- AC-ACC-04-28【后端】【C1】：输入 `delete/cancel` 在 Argon2 之后暂停，`recover` 提交后重新申请注销；操作 放行；预期 撤销 401，新申请不被撤销；错误码 4018。（RS5-21）
- AC-ACC-04-29【后端】【C1】（Postgres 实现后才能跑，仓库现在只有 SQLite，事务串行，这条交错不可达；**C1 验收时记为未执行，不算通过**）：输入 `recover` 事务暂停在第一条语句之后；攻击者的改密、`logout-others`、登录都已过 Argon2；操作 分别放行，再放行 `recover`；预期 都等待后 401，攻击者没有拿到恢复码；`recover` 提交后这个账号只有受害者的会话；错误码 4018。（RS6-01、RS7-01）
- AC-ACC-04-30【后端】【C1】：输入 攻击者持 Cn，循环并发 `recover{Cn}`；另一账号循环 `recover` 自己的码；操作 受害者 `recover{C1}`（比 Cn 老）；其他账号正常找回；预期 都在排队时限内成功；攻击者的码同一时间只算 1 个；错误码 —。（RS6-02）
- AC-ACC-04-31【后端】【C1】[IM 钟]：输入 C1 第 0 天换代，受害者持 C2；第 1 天攻击者改密得到 C3，循环 `recover` 用满上限；操作 受害者第 2 天用 C2；攻击者继续用最新的码；预期 受害者成功；攻击者第 7 次 429，`reason=recoverLimited`；错误码 4290。（RS7-02）

### F-ACC-05 恢复码与找回密码（P0，CH-01）

**规则**
1. 注册时生成 1 个恢复码（128 位随机，易读编码），服务端只存 `SHA-256(userId, code)`（`recovery_codes`，每代一行）。码有 128 位随机，不怕离线猜，所以不用 Argon2，也不用带密钥的 HMAC，不引入密钥生命周期（RS4-12、RS5-04：HMAC 密钥一丢，所有恢复码静默失效，也没有客服兜底）。**每次换码都是新的一代，有代号 `gen`，有自己的确认状态**（RC2-03）。**确认流程**（R-CF-10）：响应带 `recoveryCode`、`recoveryCodeGen`；客户端点"我已保存"后调用 `POST /v1/account/recovery-code/ack {gen}`。`gen` 不是最新一代：400 / 4000（`reason: recoveryCodeStale`），客户端提示"恢复码已更新，请重新登录查看"，不算确认。**最新一代未确认时登录（注册、改密、找回、重新生成的响应丢失，或在保存页被杀）**：作废这个没人确认的代（它没有 7 天保留），生成新的一代在登录响应里返回，客户端进入保存页；**确认前 token 只放内存，是 F-ACC-03 规则 5"先落盘"的例外**（这些 token 是 `current`，丢了不会造成吊销，RS5-13）。**已登录但最新一代未确认**（`recovery-code` 响应丢失，或在保存页被杀）：`auth` ack 和账号接口带 `recoveryCodeUnacked`，App 强制进保存页，输入密码重新生成，否则旧码第 7 天失效后用户手里就没有可用的码了。
2. `POST /v1/account/recover {username, recoveryCode, newPassword, platform}`：用户名不存在、恢复码错误、已用过：**相同响应** 401 / 4018，未命中计入失败限制。按 `(userId, code)` 查哈希，**未命中不占 Argon2 闸门**，匿名洪水压制不了；**命中的请求不受"用户名 + IP"锁定和单哈希规则限制**（128 位码不怕猜，否则持有密码的攻击者能压制受害者找回），用**独立专用位**计算新密码的哈希（RS4-12、RS5-02）。**专用位按 `(userId, gen)` 公平调度**（RS6-02）：同一个码同时只算 1 个；队列按账号轮转；同一账号里**代号更老的码优先**，否则持有新码的攻击者能压住受害者的旧码；进入专用位前重查一次码是否还可用。**成功次数上限只约束账号当前最新的一代**：用最新一代的码，每账号每小时成功 `recover` 不超过 6 次（未校准），超出 429 / 4290（`reason: recoverLimited`，只有持有效码才走到这一步，可以和 4018 区分）；其余有效的旧代都不受限（RS7-02，否则攻击者用满次数后，受害者手里不是最老的码也用不了）。专用位的调度按单个实例生效，集群时随 B6 重新看。每 IP 每分钟 20 次（未校准）的总量限流在查库之前，不看是否命中；办公室 NAT 下同网的人可以短时压制找回，换网络可以绕开，部署说明写明（第 12.2 节）。恢复码比较前规范化（大小写、去分隔符），哈希输入是 `userId` 和规范化后的码，用长度前缀拼接。
3. 成功：改密码；吊销全部设备会话和推送令牌（`revokeReason=credentialsChanged`，当前设备拿新 `sid`）；写 `credentials_changed_at`；这个恢复码作废，并生成新的一代，随响应返回；响应同时带新 token。其他在线设备收到 `kicked/passwordChanged`，文案同 AC-ACC-04-5。
4. **恢复码丢失 = 账号无法找回。第一批不提供客服重置**（R-CS-01）。原因：能重置密码就能以用户身份登录、读全部聊天，等于绕过 F-KEY-02（`console` 主凭证不含 `token.issue`）和 X-7（单聊正文不可看），内部小范围可以接受这个代价。注册页和保存页要明确告知。没有 `account.reset`，后台没有任何账号能改用户密码。备选见 CH-01。
5. **旧码保留**（RC2-02）：改密和"重新生成恢复码"（要输入当前密码）都产生新的一代，**已确认的旧码继续有效 7 天（`IM_RECOVERY_OLD_DAYS`，未校准）**，然后作废。原因：只作废旧码的话，撞库拿到密码的人改一次密，受害者就永久失去账号（没有客服）。
   - **旧码优先**（RS4-01）：找回成功后，**只作废比所用代更新的代**，吊销全部会话。用新代找回时，保留期内的旧代继续有效，到原定期限为止；多次改密或找回不延长它。否则攻击者改密拿到新码，马上用新码找回，受害者的旧码就作废了，7 天保护等于没有。
   - 可信设备提醒：改密、重新生成时，其他在线设备收到 `securityNotice`；离线设备下次上线时，`auth` 和刷新都带上原因和截止日期（F-ACC-03 规则 2、6，RS4-07）。登录失败页常驻"忘记密码"入口。
   - **代价**：丢手机且恢复码存在手机里的场景（R-CS-18）：拿到手机的人 7 天内仍能用旧码。两种风险取其轻，密码被盗比恢复码同时被拿到常见（判断，未校准）。
   - **7 天内没有发现就真的丢了**：攻击者改密后，受害者 7 天内没有用旧码找回，账号永久丢失（R-1、CH-01）。
5a. **冷静期内 `recover` 可以用**：成功后撤销注销申请（F-ACC-08），新密码生效。否则"攻击者先改密再申请注销"受害者既登录不了也撤销不了（RC2-02）。注销已到期执行后 `recover` 返回 4018。
6. 不提供客服联系入口的配置（因为客服也帮不了）；登录页"忘记密码"只进入恢复码流程。

**异常**：新密码不合规：400 / 4000，恢复码不作废。恢复码正确但账号封禁：403 / 4034，不改密码。保存页被杀：见规则 1。

**验收标准**
- AC-ACC-05-1【后端】【C1】：输入 正确的恢复码和新密码；操作 找回；预期 新密码可登录，旧密码 401；全部旧设备会话吊销；响应带新恢复码；旧恢复码再用 401；错误码 4018。
- AC-ACC-05-2【后端】【C1】：输入 不存在的用户名；错误的恢复码；已用过的恢复码；操作 分别找回；预期 都是 401，响应体相同；错误码 4018。
- AC-ACC-05-3【后端】【C1】：输入 同一用户名 + IP 连续 5 次恢复码错误；操作 第 6 次用错误恢复码；再用正确恢复码；预期 错误的 429；正确的成功；错误码 4294。（RS5-02 改）
- AC-ACC-05-5【前端】【C1】：输入 找回成功；操作 观察；预期 显示新恢复码，确认保存后进入会话列表；错误码 —。
- AC-ACC-05-6【后端】【C1】：输入 注册响应丢失（客户端没见过恢复码，最新一代 `acked_at` 为空）；操作 用密码登录；确认；再登录；预期 第一次登录响应带新恢复码，注册时的那个失效；确认后再登录不带；错误码 —。
- AC-ACC-05-7【前端】【C1】：输入 在保存页杀掉 App（H5 刷新）；操作 重新打开并登录；预期 再次进入保存页，显示新恢复码；确认前 token 没有持久化；错误码 —。
- AC-ACC-05-8【后端】【C1】：输入 新密码不合规；封禁中的账号；操作 分别用正确恢复码找回；预期 400；403；恢复码和密码都不变；错误码 4000、4034。
- AC-ACC-05-9【后端】【C1】：输入 全部 `/admin/*` 路由和 `console` 主凭证的 scope；操作 枚举；预期 没有任何接口能设置、重置用户密码或恢复码；错误码 —。
- AC-ACC-05-10【前端】【C1】：输入 注册页、保存页、"忘记密码"页；操作 查看；预期 都有"恢复码丢失后账号无法找回"的文字；没有联系客服的入口；错误码 —。
- AC-ACC-05-11【后端】【C1】：输入 存在账号恢复码错误、不存在账号各 50 次，关闭锁定；操作 交替请求；预期 中位数相差 ≤ 5%；错误码 4018。
- AC-ACC-05-12【后端】【C1】[IM 钟]：输入 登录后重新生成恢复码，当前密码错误 / 正确；操作 分别请求；第 6 天用旧码找回；另一账号第 8 天用旧码找回；预期 错误时 401，码不变；正确时返回新一代；第 6 天成功；第 8 天 401；错误码 4018。（RC2-02，取代 AC-ACC-05-4）
- AC-ACC-05-13【后端】【C1】：输入 改密响应丢失（新一代没人见过）；操作 用新密码登录；确认；预期 登录响应带再新一代的恢复码；旧的已确认码仍在 7 天内有效；确认后再登录不带；错误码 —。（RC2-03）
- AC-ACC-05-14【后端】【C1】：输入 D1 停在保存页显示 C1（`gen=1`），D2 登录生成 C2（`gen=2`）；操作 D1 ack `gen=1`；D2 ack `gen=2`；预期 D1 400，`reason=recoveryCodeStale`，状态仍未确认；D2 确认成功；错误码 4000。（RC2-03）
- AC-ACC-05-15【前端】【C1】：输入 服务端返回 `recoveryCodeStale`；操作 点"我已保存"；预期 提示"恢复码已更新，请重新登录查看"，回登录页；错误码 4000。（RC2-03）
- AC-ACC-05-16【后端】【C1】[IM 钟]：输入 攻击者改密后申请注销；操作 受害者第 3 天用旧恢复码找回；预期 成功，注销申请被撤销，第 8 天清理不处理这个账号，攻击者的会话都被吊销；错误码 —。（RC2-02）
- AC-ACC-05-17【后端】【C1】[IM 钟]：输入 C1 已确认；攻击者改密得到 C2，确认后立即用 C2 找回得到 C3，再次改密；操作 受害者第 6 天用 C1 找回；再用 C2、C3；另一账号第 8 天用 C1；预期 第 6 天成功，攻击者的会话全部吊销，C2、C3 和之后的代都作废；第 8 天 401（多次改密不延长）；错误码 4018。（RS4-01）
- AC-ACC-05-18【后端】【C1】：输入 匿名请求占满 Argon2 闸门；操作 同时用错误恢复码和正确恢复码 `recover`；预期 错误的快速 401，不等闸门；正确的用 `recover` 专用位成功；错误码 4018。（RS4-12）
- AC-ACC-05-19【C1】：输入 已登录，`recovery-code` 响应丢失；改密响应到达后在保存页杀 App；操作 重新打开；预期 前者收到 `recoveryCodeUnacked`，强制进保存页，输入密码后拿到新码；后者 token 没有落盘，回登录页，用新密码登录后拿到新码；错误码 —。（RS5-13）

### F-ACC-06 资料与头像（P0）

**规则**
1. 替代 PRD-ui G-UI-05（客户端改资料"不做"），因为不再有业务方。`PATCH /v1/account/profile {nickname?, avatarFileId?}`，至少一项。
2. 昵称规则同注册；变更后其他人按 F-SOC-10 的缓存规则更新（10 分钟 + 服务端应答覆盖），不做实时推送。
3. 头像走 F-FILE-01，`kind=avatar`：jpeg、png、webp，≤ 2 MiB；客户端裁剪为正方形并缩放到 512×512。
4. 头像先上传到私有临时键（F-FILE-01），`complete` 校验通过后 Copy 到**私有最终键**；**`PATCH profile` 绑定时才 Copy 到独立的公开读桶，用独立域名**（禁止列举，RC2-19 恢复 R-CS-10 的要求，RS4-22），地址含随机 ID，**公开键由服务端生成，先写进 `files.public_key` 再 Copy**，重试复用同一个键，崩溃不会留下无记录的公开对象（RS6-04）；写入 `users.avatar`（https，≤ 512，AC-UI-21-5 不变）。相同 `avatarFileId` 重复绑定按成功处理（响应丢失后重试）；绑定失败不立即删除公开对象（可能和并发重试撞键），由 `file_purge` 按 `public_key` 清除；头像 `retain_until` 为空，不受 180 天保留期影响。换头像时写 `files.avatar_replaced_at`，7 天内绑回这个文件就清空它。**换头像 7 天后或注销清理时，把头像文件改成 `purging`，走 `file_purge` 同一条路径，同时删除 `public_key` 和 `final/{fileId}/`**（RS5-17、RS6-04）。公开对象的 `Content-Type` 由服务端写入（元数据替换，F-FILE-01 规则 3，RS4-17）；`nosniff` 是优先能力，厂商或 CDN 支持就配（R-CB-09、R-CS-10）。**头像对持有地址的人公开**，这是有意的：列表里要大量加载，不适合每张都换预签名。未绑定的头像和其他上传一样走 F-FILE-06 的 24 小时清理；`PATCH profile` 绑定后状态 `attached`，不再走这条（R-CQ-39、RS4-22）。
5. 换头像后旧对象 7 天后删除（已有客户端缓存可以继续显示）。
6. 演示模式下 `POST /demo/login` 改昵称的行为不变。

**异常**：`avatarFileId` 未完成上传或类型不对：404 / 4048。对象存储不可用时昵称仍可以改，头像 503 / 5032。

**验收标准**
- AC-ACC-06-1【后端】【C1】：输入 u1 改昵称为"小明"；操作 u2 用 `users` 查 u1；预期 返回新昵称；错误码 —。
- AC-ACC-06-2【后端】【C1】：输入 昵称 33 个码点；空白；操作 分别修改；预期 都是 400；错误码 4000。
- AC-ACC-06-3【后端】【C2】：输入 `kind=avatar` 的已完成上传；`kind=file` 的上传；别人的上传；操作 分别作为 `avatarFileId`；预期 第一个成功，`users.avatar` 是 https 地址；后两个 404；错误码 4048。
- AC-ACC-06-4【后端】【C2】：输入 `kind=avatar` 声明 `image/svg+xml` 或 3 MiB；操作 申请上传；预期 依次 415、413；错误码 4150、4139。
- AC-ACC-06-5【前端】【C1】[SDK 钟]：输入 "我"页；操作 改昵称；预期 "我"页和聊天页立即显示新昵称；其他端把 SDK 钟推进 10 分钟后更新；错误码 —。（v0.2 改：R-CQ-18，头像部分拆到 AC-ACC-06-6）
- AC-ACC-06-6【前端】【C2】：输入 "我"页；操作 选图、裁剪、保存；预期 上传的是 512×512 图片，显示新头像；错误码 —。
- AC-ACC-06-7【后端】【C2】：输入 `kind=avatar` 的上传未 `complete`；已 `complete` 未绑定；操作 不带签名访问它的临时键；绑定前访问公开桶；`PATCH profile` 后访问最终地址；对公开桶列举；预期 临时键被拒绝；绑定前公开桶没有这个对象；最终地址可读，`Content-Type` 是图片（目标存储支持 `nosniff` 时响应带它）；域名和消息文件不同；列举被拒绝；错误码 —。（RS4-22 改）
- AC-ACC-06-8【后端】【C2】[IM 钟]：输入 已绑定的头像，25 小时后；换头像后 8 天；操作 运行清理；预期 25 小时不被清理；换头像 8 天后旧对象被删；错误码 —。
- AC-ACC-06-10【后端】【C2】[IM 钟]：输入 `complete` 后没有 `PATCH profile` 的头像；操作 推进 25 小时并运行清理；预期 对象和记录被删除；错误码 —。（RS4-22）
- AC-ACC-06-11【后端】【C2】[IM 钟]：输入 `PATCH profile` 响应丢失后用相同 `avatarFileId` 重试；已绑定的头像过 181 天；F1 → F2 → 5 天内绑回 F1；操作 分别观察，推进到第 8 天运行清理；预期 重试成功；头像还在；F1 不被删除；错误码 —。（RS5-17、RS7 改）
- AC-ACC-06-12【后端】【C2】[IM 钟]：输入 `PATCH profile` Copy 到公开桶后杀进程，重试成功；再换头像后 8 天；另一用户注销到期；操作 运行清理，查公开桶、`final/{fileId}/` 前缀和 `files` 行；预期 重试后公开桶只有 1 个对象；换头像和注销的旧文件三处都被清除；错误码 —。（RS6-04）
- AC-ACC-06-13【后端】【C2】[IM 钟]：输入 Copy 到公开桶后杀进程，不重试；操作 推进 25 小时运行 `file_purge`；预期 公开桶、私有前缀和 `files` 行都已清除；错误码 —。（RS7-03）
- AC-ACC-06-9【后端】【C2】：输入 对象存储不可用；操作 改昵称；改头像；预期 昵称成功；头像 503；错误码 5032。

### F-ACC-07 与演示登录、管理接口并存（P0）

**规则**
1. **三种入口并存，共用同一张 `users` 表和同一种 token**：账号登录（`/v1/account/*`）、演示登录（`/demo/login`，`IM_DEMO_LOGIN=1`，生产拒绝启动，ARCH S4 不变）、管理接口（`POST /admin/users/token`，scope `token.issue`，只在管理端口）。
2. 演示和导入的用户没有账号记录，不能密码登录（4018），也没有办法变成账号（第一批没有重置，CH-01）；要用账号就注册一个新用户名。**`accounts` 里有行的用户，`/demo/login` 一律拒绝（403 / 4030，`reason: accountExists`）**，否则测试和预发环境输入用户名就能接管账号（R-CB-18）。
3. **管理接口仍能为任何用户签发 token，包括有密码的用户**，这是最高权限，签发的 token 没有 `sid`，用户侧看不到。不再有业务方后，**生产环境调用方带 `token.issue` 时拒绝启动，除非显式设置 `IM_ALLOW_TOKEN_ISSUE=1`**（R-CS-26），放行时配置检查告警，每次调用照常写 `admin_audit`。
4. `GET /v1/config`（不需要登录）返回 `{registerMode, demoLogin, features, limits}`，客户端据此显示入口。**三端都有构建开关**（H5 `VITE_DEMO_LOGIN`、Android `build.gradle.kts`、iOS `Config.swift`），**构建开关是上限：两者同时为真才显示演示入口**（R-CF-11）。已登录用户拉配置失败时用上次缓存的值；从未成功过时隐藏新功能入口。
5. 登录页：
   - 演示开启：显示现有演示表单（F-UI-03 不变）和"使用账号登录"入口；
   - 演示关闭：只显示账号登录，AC-UI-03-9 的"请从业务方入口进入天听"作废；
   - 注册入口按 `registerMode` 显示，`closed` 时隐藏。
   - 演示会话（Session `mode=demo`）隐藏"账号安全"。
   - `features.files=false` 时隐藏"+"，已有的图片、文件消息照常显示，打开时按 4048 或开关关闭提示。
6. e2e 和性能测试继续用演示 / 管理接口签发 token，不依赖注册流程。

**异常**：`GET /v1/config` 请求失败：客户端显示"网络异常，请稍后重试"和重试按钮，不猜测模式。

**验收标准**
- AC-ACC-07-1【后端】【C1】：输入 `IM_DEMO_LOGIN=1`，默认配置；操作 跑 `server/e2e` 现有用例；预期 不改代码全部通过；错误码 —。
- AC-ACC-07-2【后端】【C1】：输入 `/demo/login` 产生的用户 demo1；已注册账号 acc1；操作 密码登录 demo1；用演示登录 acc1；预期 第一个 401；第二个 403，`reason=accountExists`，昵称不变；错误码 4018、4030。（v0.2 改：R-CS-01、R-CB-18）
- AC-ACC-07-3【后端】【C1】：输入 `IM_ENV=production`、调用方配置含 `token.issue`；操作 不带确认项启动；带 `IM_ALLOW_TOKEN_ISSUE=1` 启动并查 `GET /internal/v1/config`；预期 第一次非 0 退出；第二次启动成功，`productionChecks` 有 `token_issue_caller` 告警；错误码 —。（v0.2 改：R-CS-26）
- AC-ACC-07-4【后端】【C1】：输入 三种模式；操作 请求 `GET /v1/config`；预期 响应和配置一致，不包含任何密钥；错误码 —。
- AC-ACC-07-5【前端】【C1】：输入 `demoLogin=false`、`registerMode=invite`；操作 打开登录页；预期 有用户名、密码、"注册"入口（注册页有邀请码输入框），没有演示表单；页面不出现"业务方"字样；错误码 —。
- AC-ACC-07-6【前端】【C1】：输入 `demoLogin=true`；操作 打开登录页；预期 现有演示表单照常，AC-UI-03-1～8 通过；错误码 —。
- AC-ACC-07-7【前端】【C1】：输入 构建开关关、`demoLogin=true`；构建开关开、`demoLogin=false`；操作 打开登录页；预期 都不显示演示表单；错误码 —。
- AC-ACC-07-8【前端】【C1】：输入 演示会话；操作 打开"我 → 设置"；预期 没有"账号安全"；错误码 —。
- AC-ACC-07-9【前端】【C1】：输入 `registerMode=closed`；`GET /v1/config` 失败；操作 打开登录页；预期 前者没有注册入口；后者显示"网络异常，请稍后重试"和重试，不猜测模式；错误码 —。

### F-ACC-08 注销账号（P1，技术决定见 20.2）

**规则**
1. `POST /v1/account/delete {password}`。先二次确认，明确告知"7 天后不可恢复，用户名不能再使用，已发的消息会保留"。
2. **冷静期 7 天**（未校准，R-CS-27）：申请后立即吊销全部设备会话和令牌，在线连接 `kicked/accountDeleted`（4012）；这 7 天内用密码登录：403 / 4030（`reason: accountDeletionPending`，`details.deleteAt`），没有 token，客户端提示"账号正在注销，{日期}后不可恢复，是否撤销？"。撤销：`POST /v1/account/delete/cancel {username, password}`（**匿名接口**，和登录共用限流和相同响应，RC2-05），成功后再正常登录。密码也丢了：用恢复码找回（F-ACC-05 规则 5a）。防止密码被盗后账号被立即毁掉。
3. 到期后分两步（R-CB-15）：① 事务内在 **`users.deleted_at`** 标记（存在性判断都看 `users`），递增 `token_version`，删除密码和恢复码哈希；② 幂等后台任务 `account_cleanup` 删除好友、黑名单，逐群退出（群主转让按 PRD-social F-SOC-23 规则 4；托管群回到无群主，ADMIN.md"群"规则），删除头像对象，昵称改为"已注销用户"。第 ② 步最终一致，可以重试。
4. `users.deleted_at` 要改到的入口：`Relation`、`UserExists`、`MissingUsers`、`issueFor`、演示登录、`UpsertUsers`、管理加人。给已注销用户发消息 4042；签发 token 4042。
5. 数据处理（如实告知，隐私说明要写）：

   | 数据 | 处理 |
   |---|---|
   | 密码、恢复码、设备会话、令牌、好友、黑名单、头像 | 删除 |
   | 已发的消息和文件 | 保留（对方的记录不受影响），文件按原保留期 |
   | `userId` | **留在消息、会话 ID 和索引里，不能抹除**（CH-02）。界面显示"已注销用户"，协议层仍是原名 |
   | `account_events`、审计 | 按保留期 |

6. 法务可能要求不同的保留口径（CH-23），实现前确认。

**异常**：密码错误 401 / 4018，不做任何改动。第 ① 步失败整体回滚。第 ② 步中途失败：下次任务继续。**冷静期内和注销后持有旧 token：一律 `kicked/accountDeleted`（4012）**（`auth` 先看账号状态，RC2-05）；老 SDK 按 4012 显示"账号已被下线"。

**验收标准**
- AC-ACC-08-1【后端】【C1】[IM 钟]：输入 u1 密码错误 / 正确；操作 注销；第 3 天登录但不撤销；第 3 天用旧 token `auth`；第 8 天运行清理后再登录、再注册同名；预期 错误 401；正确时全部连接收到 `kicked/accountDeleted` 并以 4012 关闭；第 3 天登录 403，`reason=accountDeletionPending`，有 `deleteAt`，没有 token；旧 token 收到 `kicked/accountDeleted`；第 8 天登录 401，注册 409；错误码 4018、4030、4095、4012。（v0.3 改：RC2-05）
- AC-ACC-08-2【后端】【C1】[IM 钟]：输入 u2 和 u1 有聊天记录，g1 里有 u1 的消息，u1 注销到期；操作 u2 `sync`、u2 给 u1 发消息、`POST /admin/users/token` 给 u1 签发；预期 历史还在，`users` 查到昵称"已注销用户"；发消息 4042；签发 404；错误码 4042。（v0.2 改：R-CB-15）
- AC-ACC-08-3【后端】【C1】[IM 钟]：输入 u1 是自建群 g1 的群主；托管群 g2 的群主；操作 注销到期并运行 `account_cleanup`；预期 g1 群主转给最早的管理员或最早入群的成员，有 `ownerChanged`；g2 回到无群主；错误码 —。（v0.2 改：R-CQ-39）
- AC-ACC-08-4【后端】【C1】[IM 钟]：输入 u1 申请注销；操作 第 3 天匿名调用 `delete/cancel`，密码错误 / 正确；再登录；第 8 天运行清理；预期 错误 401；正确后可以正常登录，密码不变；清理不处理 u1；错误码 4018。（v0.3 改：RC2-05）
- AC-ACC-08-5【后端】【C1】[IM 钟]：输入 u1 在 600 个群里，第 ② 步中途让任务失败；操作 再次运行；同时 ≥ 1000 条其他用户的消息；预期 最终全部退出，没有重复提示消息；处理期间 ack P99 ≤ 200 ms（未校准，开发机参考，目标机重测）；错误码 —。（v0.3 改：RC2-26）
- AC-ACC-08-6【前端】【C1】：输入 "账号安全 → 注销账号"；操作 查看确认框；输入错误密码；预期 确认框有规则 1 的文字；密码错误原地提示；错误码 4018。
- AC-ACC-08-7【后端】【C2】[IM 钟]：输入 u1 有头像；操作 到期清理后访问旧头像地址；预期 对象不存在；错误码 —。

### F-ACC-09 邀请码与后台账号信息（P0）

**规则**
1. 邀请码由后台生成：`{code, maxUses, usedCount, expiresAt, note, createdBy}`，**128 位随机**（R-CS-08），只保存确定性哈希和后 4 位，明文只显示一次；单次最多生成 100 个；`maxUses` 1～1000。**全部有效邀请码的剩余可用次数合计超过 `IM_INVITE_WARN_TOTAL`（500，未校准）时配置检查告警**（公开传播一个大额码等于开放注册，所有 `open` 的启动保护都不生效，RC2-18）。
2. 权限点 `account.invite`（`operator`），写审计，审计不记明文。
3. 可以停用未用完的邀请码，立即生效。规则 F-ACC-01 第 7 条的自动停用同样显示在这里。过期或用完 90 天后 `invite_purge` 清除（未校准）。
4. ~~重置码~~：v0.2 取消，第一批没有 `account.reset`（CH-01、R-CS-01）。
5. 用户详情页增加账号信息：是否有账号、注册时间、最近登录时间、设备会话数（不含令牌）；权限沿用查看用户详情。

**异常**：邀请码并发超用：扣减在事务内，超过 `maxUses` 的按无效处理。写审计失败：操作不执行（5031）。

**验收标准**
- AC-ACC-09-1【C1】：输入 运营生成 maxUses=2 的邀请码；操作 三个用户用它注册；预期 前两个成功，第三个 403；后台显示已用 2 次；错误码 4030。
- AC-ACC-09-2【后端】【C1】：输入 生成邀请码；操作 查数据库、审计、日志；预期 搜不到明文；错误码 —。
- AC-ACC-09-5【后端】【C1】：输入 已停用的邀请码；操作 注册；预期 403；错误码 4030。
- AC-ACC-09-6【后端】【C1】：输入 `support` 账号；`auditor` 账号；`operator` 账号；操作 生成邀请码；预期 前两个 403；第三个成功；错误码 4030。
- AC-ACC-09-7【后端】【C1】：输入 一次 101 个；`maxUses=1001`；操作 生成；预期 依次 400 / 4131、400 / 4000；错误码 4131、4000。
- AC-ACC-09-8【后端】【C1】：输入 故障注入审计写失败；操作 生成邀请码；预期 503，邀请码未产生；错误码 5031。
- AC-ACC-09-9【后端】【C1】：输入 maxUses=1，10 个并发注册（用户名不同）；操作 注册；预期 恰好 1 个成功；错误码 4030。
- AC-ACC-09-10【后端】【C1】：输入 邀请码生成结果；操作 统计长度；预期 编码前至少 128 位随机；错误码 —。
- AC-ACC-09-11【前端】【C1】（管理后台）：输入 客服打开用户详情；操作 查看；预期 显示账号信息，**没有**"重置密码"类操作；错误码 —。
- AC-ACC-09-12【后端】【C1】[IM 钟]：输入 用完 91 天的邀请码；操作 运行 `invite_purge`；预期 记录被清除；错误码 —。
- AC-ACC-09-13【后端】【C1】：输入 账号用户、演示用户；操作 `GET /admin/accounts/{userId}`；预期 前者有账号信息，响应没有任何哈希和令牌；后者显示没有账号；错误码 —。
- AC-ACC-09-14【后端】【C1】：输入 生成 maxUses=600 的邀请码；操作 查配置检查；预期 有告警；错误码 —。（RC2-18）

### F-ACC-10 隐私告知入口（P0，RC2-16）

**规则**
1. 登录页底部、注册页、"我 → 设置 → 关于天听"都有"隐私说明"入口。内容来自部署配置（`IM_PRIVACY_NOTICE_URL` 或打包页面），版本号 `IM_PRIVACY_NOTICE_VERSION`，`GET /v1/config` 返回。**文字由 Frank 或法务提供（第 20.3 节），AI 只提供清单草稿，不代替法律文本。**
2. 注册页展示"注册即表示你已阅读隐私说明"和入口，账号记录同意时的版本（`accounts.notice_version`）。版本更新后，下次登录提示一次，确认后记录；第一批不做"不同意就不能用"的流程（内部小范围）。
3. 说明至少告知（PRD-admin 第 12 节 ②）：撤回原文保留 30 天（CH-17）；举报证据保留 180 天；账号事件保留 180 天；`moderation_hits` 保留 90 天；打开"显示内容"后消息内容经过苹果、谷歌；文件存在第三方对象存储；管理员在什么条件下能查看群消息正文和隔离原文；注销后 `userId` 仍留在消息里。
4. **启动保护**：`IM_ENV=production` 且 `IM_MSG_ACTIONS_ENABLED=1`（有隔离原文）或 `IM_MOD_ENABLED=1`（有证据）时，`IM_PRIVACY_NOTICE_VERSION` 为空：拒绝启动。

**异常**：说明页加载失败：显示"暂时打不开，请稍后重试"，不阻止登录。配置为空的非生产环境：入口隐藏。

**验收标准**
- AC-ACC-10-1【前端】【C1】：输入 配置了版本和地址；操作 打开登录页、注册页、关于天听；预期 都有"隐私说明"入口，能打开；错误码 —。
- AC-ACC-10-2【后端】【C1】：输入 版本 v1；操作 注册后查账号；预期 `notice_version=v1`；错误码 —。
- AC-ACC-10-3【前端】【C1】：输入 账号同意过 v1，配置更新为 v2；操作 登录；预期 提示一次，确认后不再提示；错误码 —。
- AC-ACC-10-4【后端】【C4】：输入 `IM_ENV=production`、`IM_MSG_ACTIONS_ENABLED=1`、版本为空；操作 启动；预期 非 0 退出，指出缺隐私说明；错误码 —。
- AC-ACC-10-5【后端】【C6】：输入 `IM_ENV=production`、`IM_MOD_ENABLED=1`、版本为空；操作 启动；预期 非 0 退出；错误码 —。

---

## 6. C2 图片和文件消息

### F-FILE-01 上传流程（P0）

**规则**
1. `POST /v1/files/uploads {kind: image|file|avatar, name, size, mime, thumb?: {size, mime}}` → `{fileId, put: {url, headers, expiresAt}, thumbPut?}`。**一个 `fileId`，原图和缩略图是它的两个变体**（`original`、`thumb`），没有单独的缩略图 ID（R-CF-02、R-CB-03、R-CQ-02）。`kind=image` 必须带 `thumb`。预签名 PUT 指向**临时键**，有效 15 分钟（未校准），**签入固定的 Content-Length 和 Content-Type**（RC2-08、RS4-17）。
2. 客户端直接 PUT 到对象存储，**文件内容不经过 im-server**。第一批不做分片和断点续传，失败从头重传。
3. `POST /v1/files/{fileId}/complete`（幂等，已 `ready` 就返回已有记录）。**先取租约**：`UPDATE files SET completing_until=?, completing_attempt=? WHERE file_id=? AND owner=? AND state='pending' AND (completing_until IS NULL OR completing_until<now)`（租约 60 秒，未校准），拿不到：503 / 5032（`reason: completeInProgress`，可重试）。**每次尝试结束时（包括失败）按 `completing_attempt` 条件释放租约**，否则补传缩略图后要等 60 秒（RS5-07）。Copy 前先查一次额度，超额的不做（RS5-06）。然后对每个变体：① HEAD 核对大小，记下 **ETag**，大小不符：删除临时对象，400 / 4000（`reason: uploadMismatch`）；② **带 `If-Match: ETag`** 的 Range 读取前 64 KB（未校准）嗅探真实类型，图片解析头部得到真实宽高（不解码像素，R-CS-12），解析不到尺寸就拒绝（RS4-21）；与声明矛盾或属于禁止类型：删除，415 / 4150；③ **条件 Copy（`x-amz-copy-source-if-match: ETag`）到本次尝试的键 `final/{fileId}/{attemptId}/{variant}`**（`attemptId` 随机），`x-amz-metadata-directive: REPLACE`、`x-amz-tagging-directive: REPLACE`，**只写服务端确定的 `Content-Type`，不保留 `Content-Encoding`、`Content-Disposition`、用户元数据和标签**（RS4-17、RS5-21）；再 HEAD 核对大小和 **Copy 响应返回的 ETag**（不假设和源相同，加密下可能不同，RS4-18）。条件失败（412）内部重试 1 次，仍失败 400 / 4000（`uploadMismatch`），临时对象不删。三次请求之间持续 PUT 换对象，条件不满足就失败，没有检查过的内容进不了最终键（RC2-07）。④ **全部变体成功后，提交事务 `WHERE state='pending'`，一次写入 `object_key`、`thumb_key` 和这次读到的元数据，状态 `ready`，再删临时键**。**不跳过**已成功的变体：失败或输掉竞争的尝试只删除自己的键，重试整体重来（RS4-16），所以最终内容和记录一定来自同一次尝试，`ready` 后内容不变。**崩溃留下的键靠 `{fileId}` 前缀定位**：提交后按前缀删除早于租约的其他尝试；`file_purge` 处理这个文件时按前缀全部删除，所以残留不会超过文件的保留期，头像按换头像或注销清理的时间（RS5-06、RS6-04）。清理不用请求上下文。**HEAD 得到 404 或提交失败时先重读状态，已 `ready` 就返回记录**（RS5-07）。客户端永远不知道最终键。响应 `{fileId, size, mime, width?, height?}`。图片缺缩略图：4096，状态不变。临时对象已不存在（`kind` 任何值）：同样 4096（RS4-23）。
3b. **按变体校验尺寸**（RS4-21）：缩略图长边 ≤ 512（未校准，规格 480 留余量），头像长边 ≤ 1024（未校准），超出 415 / 4150。GIF 在前 64 KB 内能看到的帧数超过上限（未校准）也拒绝。
3a. **临时前缀配生命周期规则**（1 天，必需能力）：`complete` 后再 PUT、临时键删除失败、放弃的上传留下的对象都靠它清除，不靠 `file_purge`（RC2-08）。
4. **不依赖厂商对内容的强校验**。大小靠签名的 Content-Length 限制（第 21 节必需能力），`complete` 再核对一次。**不做 SHA-256**：校验不能证明任何安全性质（服务端不读全文），也避免 H5 把 100 MiB 读入内存（R-CF-22）；完整性靠 TLS。
5. 对象键由服务端生成（随机），不包含文件名和用户 ID。文件名只存数据库。
6. 配额：申请时检查"已用 + 声明大小"是否超额（超出 429 / 4292，SDK 不重试）；**`complete` 成功时才按实际大小扣**，失败和放弃都不扣（R-CF-01）。**`complete` 提交时在同一事务复查：已用量已达额度就 429 / 4292，临时对象留给生命周期清除**，所以当日最多超额一个文件（100 MiB）（RS4-19，v0.3"不复查、最多超额 2 GiB"不成立，只统计 15 分钟内的未完成数，累计可达 187.5 GiB）。滥用靠申请频率 60 次 / 分钟（4290）和未完成上传数 20 个限制（R-CS-21）。全局用量告警**包含临时前缀**。
6a. **未完成上传数只统计 PUT 地址还没过期（15 分钟内）的 `pending`**，否则弱网下失败 3 轮就被锁 24 小时（RC2-09）。超限 429 / 4290（`reason: uploadPendingLimit`），SDK 不自动重试，提示"上传任务太多，请稍后再试"。`DELETE /v1/files/uploads/{fileId}` 取消（SDK `cancelUpload` 调用，幂等）：**只匹配本人且 `pending` 的上传**，其他情况都返回同样的成功（不暴露存在性），立刻释放名额，尽力删除临时对象（RS4-19、RS4-23）。**取消就是 `pending` → `purging` 的条件更新，不看租约，进行中的 `complete` 提交随之失败并删除自己的键，之后的 `complete` 按不存在处理**（RS5-16、RS7）。取消后 PUT 地址仍然有效，最多 15 分钟，对象靠生命周期清除。**每人每天申请的声明字节数合计不超过每日额度的 4 倍（`IM_UPLOAD_DAILY_DECLARED`，未校准），超出 429 / 4292**，否则反复申请加取消可以产生大量不计费的临时对象（RS5-16）。
7. 文件名：先 Unicode NFC；去掉路径分隔符、控制字符、**Bidi 控制和不可见字符（Default_Ignorable）**、开头的点、结尾的点和空格、`::` 之后的内容；再判断扩展名（R-CS-13、RC2-25）。清洗后为空用 `file`。清洗后超过 255 字节 400 / 4000（`reason: fileNameTooLong`）。显示时按纯文本（ARCH S12）。
8. 对象存储不可用：`complete`、Copy 失败 503 / 5032，可重试。`/uploads` 只签名，不访问存储，存储故障时也能成功（R-CQ-08）。
9. 桶要求：私有、**匿名和预签名都不能列举**（服务端凭证可以按前缀列举，第 11.3 节）、开启服务端加密（R-CS-23，密钥归属随厂商定）；关闭访问日志或脱敏（R-CS-22）。
10. 开发环境：http 的 MinIO 用 https 或 debug 例外（Android 禁止明文，R-CF-12）。

**异常**：断网后重传用新的 `fileId`；未 `complete` 或未绑定消息的上传由定时任务清理（F-FILE-06）。

**验收标准**
- AC-FILE-01-1【后端】【C2】：输入 1 MiB 的 png 和缩略图，声明正确；操作 申请、PUT、complete；预期 状态 `ready`，最终键不含文件名和 userId，和 PUT 地址的键不同，临时键已不存在；错误码 —。（v0.2 改：R-CB-08）
- AC-FILE-01-2【后端】【C2】：输入 申请时声明 1 MiB，实际 PUT 2 MiB；操作 complete；预期 400，`reason=uploadMismatch`，对象被删，没有扣额度；错误码 4000。（v0.2 改：R-CQ-07，不再依赖厂商校验）
- AC-FILE-01-3【后端】【C2】：输入 声明 `image/png`，内容是 HTML；操作 complete；预期 415，对象被删除；错误码 4150。
- AC-FILE-01-4【后端】【C2】[IM 钟]：输入 当天已用 2 GiB；操作 申请 1 字节；次日再申请；预期 第一次 429；第二次成功；错误码 4292。（v0.2 改：R-CQ-32、R-CQ-36，SDK 部分拆到 AC-FILE-01-12）
- AC-FILE-01-5【后端】【C2】：输入 对象存储不可用（假 S3 注入）；操作 complete；申请上传；WS 收发文本；预期 complete 503；申请成功；文本不受影响；错误码 5032。（v0.2 改：R-CQ-08）
- AC-FILE-01-6【后端】【C2】：输入 未登录；用别人的 `fileId` complete；操作 分别请求；预期 依次 401（4010）、404；错误码 4010、4048。
- AC-FILE-01-7【后端】【C2】：输入 文件名 `../../etc/pass\u202Ewd.`、`a\nb`、全是控制字符、清洗后 256 字节；操作 申请；查数据库；预期 依次存 `etcpasswd`、`ab`、`file`；最后一个 400；错误码 4000。（v0.2 改：R-CQ-26、R-CS-13）
- AC-FILE-01-8【后端】【C2】：输入 100 MiB 文件上传，假 S3 是独立进程；操作 抓 SDK 的 PUT 目标主机和假 S3 收到的字节数；预期 目标是对象存储，假 S3 收到全部字节，im-server 对外端口收到的请求体都 ≤ 64 KB；错误码 —。（v0.2 改：R-CB-27）
- AC-FILE-01-9【后端】【C2】：输入 `complete` 成功后；操作 用原 PUT 地址再次 PUT 恶意内容，然后接收方下载；1 天后查临时前缀；预期 下载到的是 `complete` 时的内容；临时对象被生命周期规则清除（假 S3 模拟，真实行为见 AC-FILE-90-1）；错误码 —。（v0.3 改：RC2-08）
- AC-FILE-01-10【后端】【C2】：输入 `complete` 顺序重复 2 次；原图 Copy 成功、缩略图 Copy 失败后重试；操作 请求；预期 都成功，额度只扣 1 次；重试时原图和缩略图都重新 Copy，上次的随机键被删除；全部成功后临时键才被删除；错误码 —。（RS4-16 改）
- AC-FILE-01-11【后端】【C2】[真实钟]：输入 放弃的上传 3 个；申请频率第 61 次；15 分钟内未完成的第 21 个；操作 请求；预期 放弃的不扣额度；后两个 429，第二个 `reason=uploadPendingLimit`；错误码 4290。（v0.3 改：RC2-09）
- AC-FILE-01-12【前端】【C2】：输入 服务端返回 4292；操作 发图片；预期 消息标记失败，提示"今天的上传额度已用完"，不自动重试；错误码 4292。
- AC-FILE-01-13【后端】【C2】：输入 `kind=image` 不带 `thumb`；只 PUT 了原图；缩略图是 HTML；操作 申请；complete；complete；预期 依次 400 / 4000、4096、415；错误码 4000、4096、4150。
- AC-FILE-01-14【后端】【C2】：输入 `IM_FILES_ENABLED=0`；操作 请求 `/v1/files/uploads`；`send` 的 `image`；预期 404；4000；已有文件消息可 `sync`；错误码 4000。
- AC-FILE-01-15【后端】【C2】：输入 假 S3 在 HEAD 之后、嗅探之前替换对象；在嗅探之后、Copy 之前替换对象；操作 complete；预期 都失败，内部重试 1 次，最终键没有被替换的内容，临时对象没有被删；错误码 4000。（RS4-18 改）
- AC-FILE-01-16【后端】【C2】[真实钟]：输入 20 个未完成上传，等 PUT 地址过期（测试时调短）；或取消其中 1 个；操作 再申请；预期 都成功；错误码 —。（RC2-09）
- AC-FILE-01-17【前端】【C2】：输入 服务端返回 `uploadPendingLimit`；操作 发文件；预期 标记失败，提示"上传任务太多，请稍后再试"，不自动重试；用户取消上传时发 `DELETE`；错误码 4290。（RC2-09）
- AC-FILE-01-18【后端】【C2】：输入 剩余额度 1 MiB，3 个已申请的 1 MiB 上传；每 15 分钟申请 20 个 100 MiB、24 小时后集中 `complete`；操作 依次 complete；预期 第 1 个成功，当日用量到达额度，后面都 429；集中 complete 时超额不超过一个文件；错误码 4292。（RS4-19 改）
- AC-FILE-01-19【后端】【C2】：输入 两个并发 `complete`，第一个校验后攻击者替换对象；操作 请求；预期 第二个 503 `reason=completeInProgress`；记录的元数据和最终对象一致；没有 `ready` 但对象缺失的状态；错误码 5032。（RS4-16）
- AC-FILE-01-20【后端】【C2】：输入 图片只传原图，`complete` 得到 4096；替换临时原图（类型不同），再传缩略图，再 `complete`；操作 下载；预期 最终内容、`mime`、宽高都来自第二次；错误码 —。（RS4-16）
- AC-FILE-01-21【后端】【C2】：输入 PUT 时带 `Content-Type: text/html`、`Content-Encoding: gzip`、`Content-Disposition`、`x-amz-meta-*`、`x-amz-acl`、`x-amz-website-redirect-location`、`x-amz-tagging`；操作 PUT；若成功再 complete，查最终对象和头像公开响应；预期 未签入的头被拒绝或被丢弃；最终对象和公开响应只有服务端写入的 `Content-Type`；错误码 —。（RS4-17）
- AC-FILE-01-22【后端】【C2】：输入 缩略图 6000×6000；头像 6000×6000；头部解析不到尺寸的 JPEG；操作 complete；预期 都 415；错误码 4150。（RS4-21）
- AC-FILE-01-23【后端】【C2】：输入 `ready`、`attached` 的文件；别人的 `pending` 上传；不存在的 `fileId`；操作 `DELETE /v1/files/uploads/{id}`；预期 响应都相同；文件状态和对象不变；错误码 —。（RS4-23）
- AC-FILE-01-24【后端】【C2】[IM 钟]：输入 `complete` 进行中（Copy 之后、提交之前），`file_purge` 把这条 `pending` 改成 `purging`；操作 放行；预期 `complete` 提交失败，它写的随机键被删除，没有永久孤儿对象；错误码 —。（RS4-23）
- AC-FILE-01-25【后端】【C2】：输入 文件名 `evil.exe"; x=".txt`；操作 上传，申请下载地址，看 `response-content-disposition`；预期 用 RFC 5987 / 6266 的 `filename*` 百分号编码，ASCII 回退名不含 `"` `;` `%`；H5 保存的文件名不是 `.exe`；错误码 —。（RS4-20）
- AC-FILE-01-26【后端】【C2】[IM 钟]：输入 Copy 之后、提交之前杀进程，留下 `final/{fileId}/{attemptId}`；操作 重试 `complete` 成功；另一个文件杀进程后不再重试，推进 25 小时运行 `file_purge`；预期 前者提交后残留被删除，下载到的是新一次的内容；后者对象和记录都被删除；错误码 —。（RS5-06）
- AC-FILE-01-27【后端】【C2】[IM 钟]：输入 缺缩略图 `complete` 得到 4096 后立即补传再 `complete`；Q1 租约过期，Q2 成功并删除临时键，Q1 继续；两个尝试都已 Copy，同时提交；操作 观察；预期 第二次 `complete` 立即执行，不是 503；Q1 重读状态后返回记录，不是 4096；恰好一个提交成功，败者只删除自己的键；错误码 —。（RS5-07）
- AC-FILE-01-28【后端】【C2】：输入 取消后再 `complete`；取消和 `complete` 并发；操作 请求；预期 前者 404；后者只有一个生效，状态和对象一致；错误码 4048。（RS5-16）
- AC-FILE-01-29【后端】【C2】：输入 反复申请 100 MiB 再取消，声明合计超过每日额度的 4 倍；操作 再申请；预期 429；错误码 4292。（RS5-16）

### F-FILE-02 图片消息（P0）

**规则**
1. `send {type: "image", body: {image: {fileId}}}`。服务端用自己的记录（`complete` 时读到的真实值）填 `size`、`mime`、`width`、`height` 下发（R-CS-12）。真实宽高超过 10000 或像素超过 4000 万（未校准）：`complete` 时 415 / 4150。
2. 允许：jpeg、png、webp、gif。SVG、BMP、TIFF 不允许。格式矩阵（R-CF-14，大小限制按**处理后**的文件算）：

   | 输入 | H5 | Android | iOS | 输出 |
   |---|---|---|---|---|
   | jpeg、png（不透明） | canvas | 系统 | ImageIO | jpeg |
   | png（透明）、webp | canvas | 系统 | ImageIO | png / webp（保留透明） |
   | HEIC | 浏览器能解就转，不能解提示"请用文件方式发送" | API 28+ 转 jpeg，否则同左（系统解码能力未核实） | 转 jpeg | jpeg |
   | gif | 不重新编码 | 不重新编码 | 不重新编码 | gif |
   | 动图 webp、apng | 按静态图处理（丢动画） | 同左 | 同左 | 静态 |

   解码失败或内存不足：消息标记失败，提示"无法处理这张图片，请用文件方式发送"。
3. **缩略图由发送端客户端生成**（长边 ≤ 480，≤ 512 KiB），和原图一起上传；服务端不解码像素。**缩略图可以和原图不同，这是接收方和审核员要面对的风险，不只影响发送者**（R-CS-12）：审核员和举报证据同时看缩略图和原图。**三端解码前检查像素上限（4000 万，未校准），超限显示占位，防止解码炸弹。**
4. **发送前重新编码，去掉 EXIF 和 GPS**：H5 用 canvas，Android、iOS 用系统图片接口；**先按 EXIF 方向旋转**，再去掉；长边超过 4096 时缩放。图片处理放在 App 或平台适配层，SDK 只接收处理好的结果（`sdk-core` 是纯 JVM、sdk-ts 测试环境没有 canvas，R-CF-24）。GIF 不重新编码。接收端：H5 播放 GIF；**Android、iOS 第一批显示首帧加"GIF"标记**（现有 Coil、`AsyncImage` 都不播放，要加依赖，R-CF-14）。
5. `lastMsg`：`type=image`、`text=""`；客户端显示"[图片]"。
6. 聊天页：按 `width`/`height` 预留占位，先显示缩略图；点击全屏查看原图，支持缩放和保存。加载失败显示占位和"点击重试"；过期显示"图片已过期"。
7. 发送状态机见 F-FILE-07。
8. 老版本客户端显示"[暂不支持的消息]"（`ChatPage.tsx` 第 551 行等，已核实）。
9. 图片全屏：缩放、保存。保存被系统权限拒绝时提示"没有相册权限"和前往设置的入口（API 29 以下的权限要求未核实）。

**异常**：缩略图上传失败：整条消息按上传失败处理，不发 `send`。原图过期但缩略图还在：两者同时过期，不单独保留。解码失败：显示占位，可以当文件下载。

**验收标准**
- AC-FILE-02-1【C2】：输入 u1 给 u2 发 2 MiB 的 jpeg；操作 发送；预期 u2 收到 `type=image`，先显示缩略图，点开看到原图；两边 `size`、`mime` 一致；错误码 —。
- AC-FILE-02-2【后端】【C2】：输入 `send` 带客户端伪造的 `size=1`、`mime=text/html`；操作 发送后 `sync`；预期 下发的是服务端记录的值；错误码 —。
- AC-FILE-02-3【后端】【C2】：输入 `fileId` 是别人的；是 `kind=file`；未 complete；已绑定到另一条消息；缩略图未上传（`complete` 未通过）；操作 分别发图片；预期 依次 404 / 4048、400 / 4000（`reason=fileKindMismatch`）、4096、4096、4096；消息未保存；错误码 4048、4000、4096。（v0.2 改：R-CQ-02）
- AC-FILE-02-4【前端】【C2】：输入 带 GPS、相机型号、Orientation=6 的 jpeg；操作 发送后从对象存储下载原图和缩略图并查看；预期 用 EXIF 解析工具读不到任何 EXIF 字段；显示方向和原图一致；错误码 —。（v0.2 改：R-CQ-39）
- AC-FILE-02-5【前端】【C2】：输入 上传进行到一半；操作 点取消；预期 消息从列表消失，不发 `send`；错误码 —。（v0.2 改：R-CQ-36，其余场景拆到 F-FILE-07）
- AC-FILE-02-6【前端】【C2】：输入 尺寸 4000×3000 的图片消息；操作 打开聊天页，比较图片加载前后下一条消息的位置；预期 `offsetTop`（原生端用控件坐标）变化 ≤ 1 px；错误码 —。（v0.2 改：R-CQ-25）
- AC-FILE-02-7【前端】【C2】：输入 文件名含 `<img src=x onerror=alert(1)>` 的**文件消息**；操作 在三端气泡里查看；预期 按纯文本显示，不执行脚本；错误码 —。（v0.2 改：R-CQ-27）
- AC-FILE-02-8【前端】【C2】：输入 最后一条是图片消息；操作 看会话列表；预期 摘要显示"[图片]"；错误码 —。
- AC-FILE-02-9【前端】【C2】：输入 40 张图片的历史消息；操作 打开聊天页；预期 只请求视口内的缩略图（懒加载），不请求原图；错误码 —。（v0.2 改：R-CF AC 表，三端都验）
- AC-FILE-02-10（手工）【C2】：输入 Android、iOS 真机相册里的 HEIC、实况照片；H5 拖入的 jpeg、png、gif 和 HEIC；操作 发送；预期 按规则 2 的矩阵：成功的三端都能显示；H5 解不了的 HEIC 提示用文件方式发送；错误码 —。（v0.2 改：R-CF-14）
- AC-FILE-02-11【后端】【C2】：输入 客户端 `send` 带伪造的 `width=1`；png 头部写 20000×20000；操作 发送；complete；预期 下发的是真实宽高；complete 415；错误码 4150。
- AC-FILE-02-12【前端】【C2】：输入 用测试替身让下载到的缩略图头部超过像素上限（解码炸弹）；操作 接收方打开聊天页；预期 显示占位，不崩溃，其他消息正常；错误码 —。
- AC-FILE-02-13【前端】【C2】：输入 不能解码的文件伪装成图片；超过 20 MiB 但压缩后 3 MiB 的照片；操作 分别发送；预期 前者失败提示；后者成功（按压缩后计算）；错误码 —。
- AC-FILE-02-14【前端】【C2】（Android、iOS）：输入 gif 消息；操作 查看；预期 显示首帧和"GIF"标记；H5 播放；错误码 —。
- AC-FILE-02-15【后端】【C2】：输入 `type=image` 的请求同时带 `body.text`、`body.data`；`type=text` 带 `body.data`、`body.image`；操作 发送；预期 都是 400（R-CB-21）；错误码 4000。
- AC-FILE-02-16【后端】【C2】：输入 `kind=image` 的 svg、bmp、tiff；操作 申请上传；预期 都是 415；错误码 4150。（R-CQ-23）

### F-FILE-03 文件消息（P0）

**规则**
1. `send {type: "file", body: {file: {fileId}}}`；服务端下发 `{fileId, name, size, mime}`。
2. 文件内容不重新编码，**EXIF 等元数据原样保留**；用"文件"方式发图片时，发送前提示"文件会保留原始信息（可能包含位置）"。
3. 聊天页：显示类型图标（按扩展名）、文件名（中间省略）、大小。下载（R-CF-13）：**Android、iOS 用 App 内 HTTP 下载到私有缓存**，有进度、可取消、撤回时能中止，完成后由 SAF / 分享面板交出，不用 `DownloadManager`（地址会被系统持久化，未核实）。**H5 走浏览器下载，没有进度和失败重试**。Android、iOS 打开文件：图片、pdf、txt 白名单类型可以"打开"，其他只提供"保存"（R-CS-13）。
4. **所有下载地址都签入 `response-content-type`（服务端记录的值）和 `response-content-disposition`**（R-CS-10）。文件名只用 RFC 5987 / 6266 的 `filename*=UTF-8''…` 百分号编码，加一个 ASCII 安全的回退名，不拼接原始字符串（RS4-20）。非图片一律 `attachment` 并带清洗后的文件名（H5 跨域时 `download` 属性无效，R-CF-13）；图片 `inline`。对象存储用独立域名，和 App 不同源；`nosniff` 是优先能力（第 21 节），目标存储支持就配，不支持时靠固定的响应头和独立域名（RC2-19）。H5 页面加 `Referrer-Policy: no-referrer`（R-CS-22）。
5. `lastMsg`：`type=file`、`text=""`；**客户端只显示"[文件]"，不显示文件名**（R-CB-24、R-CQ-13：`LastMsg` 没有文件名，H5 刷新后也拿不到；不存进摘要，因为文件名按正文对待）。
6. 禁止类型见 F-FILE-05。

**异常**：下载时地址过期：SDK 重新申请一次；再失败提示"下载失败"。文件名全部被清洗为空：用 `file` 显示。

**验收标准**
- AC-FILE-03-1【C2】：输入 u1 在群 g1 发 5 MiB 的 pdf；操作 成员 u2 下载；预期 显示文件名和大小；下载内容的 SHA-256 与原文件相同；错误码 —。
- AC-FILE-03-2【后端】【C2】：输入 101 MiB 文件；图片 21 MiB；操作 分别申请上传；预期 都是 413；错误码 4139。
- AC-FILE-03-3【前端】【C2】（H5）：输入 声明 `application/octet-stream` 的 `.bin` 文件，前 64 KB 是二进制，之后才是 HTML（嗅探识别不出，`complete` 通过）；操作 下载；预期 浏览器作为附件保存，不渲染；响应 `Content-Disposition` 是 `attachment`，`Content-Type` 是服务端记录的值；错误码 —。（v0.3 改：RC2-11）
- AC-FILE-03-4【前端】【C2】：输入 文件名 200 个字符；操作 看气泡；预期 中间省略，没有横向滚动；错误码 —。
- AC-FILE-03-5【前端】【C2】（Android、iOS）：输入 下载到一半断网；操作 观察，点重试；预期 显示失败和"重试"，重试从头下载，重新申请地址；错误码 —。（v0.2 改：R-CF-13）
- AC-FILE-03-6【前端】【C2】：输入 用"文件"方式选中 jpeg；操作 发送前；预期 出现保留原始信息的提示；确认后才发送；错误码 —。
- AC-FILE-03-7【前端】【C2】（H5）：输入 文件名含中文的文件；操作 点下载；预期 浏览器开始下载，保存的文件名正确；页面不显示进度（不验收）；错误码 —。
- AC-FILE-03-8【前端】【C2】：输入 最后一条是文件消息，H5 刷新页面；操作 看会话列表；预期 摘要"[文件]"，不含文件名；错误码 —。
- AC-FILE-03-9【前端】【C2】（Android、iOS）：输入 pdf 文件；`.xyz` 文件；操作 下载后查看操作；预期 pdf 有"打开"和"保存"；`.xyz` 只有"保存"；错误码 —。
- AC-FILE-03-10【前端】【C2】（H5）：输入 生产 nginx 配置的 CSP（`connect-src` 只加存储域名、`img-src` 加 `blob:`）；操作 上传、预览、下载；预期 控制台没有 CSP 违规；`connect-src` 不是 `https:`；错误码 —。

### F-FILE-04 访问控制（P0）

**规则**
1. `POST /v1/files/urls {items: [{fileId, variant: "original"|"thumb"}]}`，单次 ≤ 50（超出 4131），逐项返回 `{fileId, variant, url?, expiresAt?, code?}`（R-CB-14、R-CF-15）。预签名 GET：图片和缩略图 120 秒，文件 300 秒。规则 3 的不可访问项 `code=4048`，不影响同一批的其他项。
2. 只有同时满足才发地址：请求者**当前**能访问这条消息——单聊参与者；或当前群成员且消息 seq ≥ 他的 `minSeq`（包括 F-CVS-03 的清空位置）；消息没有被撤回、删除；文件没有过期。发送者在绑定消息前可以访问自己的上传。
3. 其他情况一律 **404 / 4048**，响应相同，不暴露文件是否存在。
4. **已知限制**：地址发出后到过期前，持有地址的人仍能下载；被移出前也可以批量预取；撤回、退群不能收回已发出的地址和已下载到本地的文件。选预签名的原因见第 20.2 节。
5. 客户端不把预签名地址持久化，不写日志（含 OkHttp、URLSession 调试日志）；按需申请，过期重新申请。**图片缓存的键是 `fileId + variant`，不是 URL**（地址每次都变，R-CF-15）。
6. 每人每分钟 120 **批**（未校准，4290）；SDK 把一屏的缩略图合并成一批。按用户统计取地址量进指标，异常时可以查（R-CS-22）。

**异常**：对象存储不可用时地址仍然可以生成（只是签名），下载由客户端按失败处理。消息状态和文件状态不一致时一律按不可访问（4048）。

**验收标准**
- AC-FILE-04-1【后端】【C2】：输入 单聊 u1、u2 的图片；操作 u3 申请地址；预期 404，响应与"不存在的 fileId"完全相同；错误码 4048。
- AC-FILE-04-2【后端】【C2】：输入 群 g1 的文件；操作 成员申请成功；把他移出群后再申请；预期 移出后 404；错误码 4048。
- AC-FILE-04-3【后端】【C2】：输入 `historyVisible=false`，D 入群前的图片；操作 D 申请地址；预期 404；改为 `true` 后成功；错误码 4048。
- AC-FILE-04-4【后端】【C2】[真实钟]：输入 `IM_FILE_URL_TTL_IMAGE=5`；默认配置；操作 取地址后等 6 秒 GET；查默认配置的 `expiresAt`；预期 第一个被对象存储拒绝；默认图片 `expiresAt` = 现在 + 120 秒，文件 + 300 秒；错误码 —。（v0.2 改：R-CQ-32）
- AC-FILE-04-5【后端】【C2】：输入 图片消息被后台删除；操作 参与者申请地址；预期 404；错误码 4048。（v0.2 改：R-CQ-18，撤回部分拆到 AC-RCL-04-6）
- AC-FILE-04-6【后端】【C2】：输入 不带 Authorization 直接访问对象键（无签名）；操作 GET；列举桶；预期 都被对象存储拒绝；错误码 —。
- AC-FILE-04-7【后端】【C2】：输入 申请地址和下载；操作 扫描 im-server 日志；预期 搜不到预签名查询串和文件名特征串；错误码 —。
- AC-FILE-04-8【后端】【C2】：输入 上传 `ready` 未绑定；操作 上传者申请地址；其他人申请；预期 上传者成功；其他人 4048；错误码 4048。
- AC-FILE-04-9【后端】【C2】：输入 一批 50 个，其中 1 个无权访问；51 个；操作 请求；预期 49 个有地址，1 个 `code=4048`，HTTP 200；51 个 400；错误码 4048、4131。
- AC-FILE-04-10【前端】【C2】：输入 200 张图片的会话，同一张图第二次出现；操作 连续滚动；预期 没有 4290；第二次命中缓存（不再下载）；错误码 —。
- AC-FILE-04-11【前端】【C2】：输入 打开调试日志的 SDK 和 App；操作 上传下载后扫描日志；预期 搜不到 `X-Amz-Signature`、预签名查询串；错误码 —。

### F-FILE-05 类型与安全（P0）

**规则**
1. 文件禁止的扩展名和真实类型（配置文件维护）：可执行和脚本类，如 `exe`、`msi`、`bat`、`cmd`、`com`、`scr`、`ps1`、`vbs`、`js`、`jar`、`apk`、`ipa`、`dmg`、`pkg`、`deb`、`rpm`、`sh`、`hta`、`lnk`、`chm`、`iso`；**会被浏览器或系统执行的文档类**：`html`、`htm`、`xhtml`、`svg`、`docm`、`xlsm`、`pptm`（R-CS-13）；RS4-25 补充：`xlsb`、`dotm`、`xlam`、`ppsm`、`pif`、`cpl`、`wsf`、`vbe`、`jse`、`reg`、`msp`、`mht`、`mhtml`、`url`、`desktop`、`command`、`svgz`、`xht`。老格式 `doc`、`xls`、`ppt` 可能含宏，但常用，第一批放行并有下载提示（规则 3），公众开放前随 CH-23 重新决定。415 / 4150。文件消息只有禁止名单，图片消息才是白名单（修正 v0.1 第 12.2 节）。**这不能替代病毒扫描**；脚本类没有魔数，只能靠扩展名。
2. 第一批不做病毒扫描。文件表预留 `scanState`（默认 `skipped`）。内部 / 小范围可以接受，**面向公众开放前要重新决定**（CH-23）。图片的前 64 KB 嗅探通不过"开头是合法图片、后面藏 HTML"的文件，所以靠规则 F-FILE-03 第 4 条固定响应头防御，不靠嗅探（R-CS-10）。
3. 下载前提示："文件来自其他用户，请确认来源可信"（只在下载非图片时，每个会话首次）。
4. 客户端渲染文件名按纯文本；图片只用系统解码器。

**异常**：扩展名和真实类型矛盾：按更严格的处理，拒绝。无法识别的真实类型：按 `application/octet-stream` 放行（文件消息）或拒绝（图片消息）。

**验收标准**
- AC-FILE-05-1【后端】【C2】：输入 `a.exe`；`a.EXE`；`a.pdf.exe`；扩展名 `.pdf` 但内容是 PE 可执行文件；操作 分别申请并 complete；预期 前三个申请时 415；第四个 complete 时 415 且对象被删；错误码 4150。
- AC-FILE-05-2【后端】【C2】：输入 配置新增 `xyz`；文件名 `a.html`、`b.svg`、`photo\u202Egpj.html`、`a.exe.`、`a.exe::$DATA`；操作 重启后分别上传；预期 `a.xyz` 415；其余 415（先规范化再判断）；错误码 4150。（v0.2 改：R-CS-13）
- AC-FILE-05-3【前端】【C2】：输入 群里的 zip 文件；操作 首次点下载；预期 出现来源提示；确认后下载；同一会话第二次不再提示；错误码 —。
- AC-FILE-05-4【后端】【C2】：输入 无法识别真实类型的文件（`kind=file`）；同样内容 `kind=image`；操作 complete；预期 前者通过，`mime=application/octet-stream`；后者 415；错误码 4150。
- AC-FILE-05-5【后端】【C2】：输入 开头是合法 png、后面是 HTML 的文件，`kind=image`；操作 接收方取原图地址并下载；预期 `Content-Type: image/png`，不是 `text/html`；错误码 —。
- AC-FILE-05-6【后端】【C2】：输入 上传完成；操作 查文件表；预期 `scanState=skipped`；错误码 —。

### F-FILE-06 生命周期（P0）

**规则**
1. 一个文件只能绑定一条消息（不去重、不秒传，避免成为存在性探测手段）。后续批转发时再定义引用方式。
2. 状态：`pending` → `ready` → `attached` → `revoked` → `purging` → 物理删除；或 `attached` → `expired` → `purging`；或 `pending` / `ready` 超过 24 小时未绑定 → `purging`（头像绑定后不走这条）；或头像 `attached` → 换头像 7 天后、注销清理时 → `purging`（RS6-04）。`pending` → `purging` 同样用条件更新，只匹配租约已过期或没有租约的行（`completing_until IS NULL OR completing_until < now`），`complete` 提交只匹配 `pending`，败者删除自己的随机键（RS4-23、RS6）。**`purging` 是清理和发送绑定之间的中间态**（R-CB-07）：事务内把 `ready` 改成 `purging`（`send` 绑定只匹配 `ready`，所以两边只有一个成功），再删对象，最后删行；删除失败保留 `purging` 下次重试。
3. **保留期**：绑定后 180 天（`IM_FILE_RETENTION_DAYS`，CH-10）到期，对象删除，消息保留，显示"已过期"；**清除 `files.name`**（R-CS-23）。`0` 表示不过期。过期时递增会话 `mod_ver`；**`mod_ver` 和 SDK 合并规则（F-RCL-03 规则 2、2a）从 C2 开始实现**（RC2-13）。
4. **撤回、后台删除**：立即停止发放地址（`revoked`）；**文件名移到隔离加密区，`files.name` 清空**（R-CS-23）；对象保留到隔离期结束（默认 30 天，同 F-ADM-32 和 CH-17）后物理删除。
5. 举报证据引用的文件：保留到证据到期（F-MOD-03 规则 6，`files.evidence_until` 和 `files.retain_until` 分开存，RC2-32）。**原消息正常时普通用户照常访问；只有过了原来的保留期或被撤回、删除后才不发放**（R-CS-25：否则举报一下就能让全群下载不了）。
6. 解散群、退群、删除会话：**不**删除文件（其他人可能还能访问）；保留期到了再删。
7. 定时任务 `file_purge` 每小时运行，一批 1000 个（未校准），和现有 `quarantine_purge` 同样的方式。对象存储删除失败下次重试，不影响消息。物理删除按 `final/{fileId}/` 前缀，不只删记录里的键（RS5-06）。**任何路径进入 `purging` 的文件，`public_key` 不为空就先删公开对象，成功后才能删行**（RS7-03）。
8. 备份：数据库备份（ARCH 3.4）不含文件内容；文件的持久性靠对象存储自身（多副本、版本）。恢复数据库到较早时间点后，文件表和对象可能不一致：缺对象的显示"已过期"。桶不开版本控制；开了就要写明同时清除旧版本，否则删除只写删除标记。多余对象按前缀对账：只在恢复数据库之后手动运行，宽限期要大于租约，并且要在恢复后的状态稳定后再运行，否则会误删（第 11.3 节的 List，RS4-14、RS5-06）。

**异常**：清理任务中途退出：下次继续。消息被撤回时文件已过期：只改状态，不报错。

**验收标准**
- AC-FILE-06-1【后端】【C2】[IM 钟]：输入 `ready` 但未绑定的上传；`pending` 的上传；操作 推进 25 小时并运行 `file_purge`；预期 对象和记录都被删除；错误码 —。（v0.2 改：R-CQ 清单）
- AC-FILE-06-2【后端】【C2】[IM 钟]：输入 已绑定的图片；操作 推进 181 天并运行清理；预期 对象不存在；`sync` 消息还在，带 `expired=true`；申请地址 404；错误码 4048。
- AC-FILE-06-3【后端】【C4】[IM 钟]：输入 被撤回的图片；操作 第 29 天检查、推进到第 31 天运行清理；预期 第 29 天对象还在且用户申请 404；第 31 天对象被删除；错误码 4048。
- AC-FILE-06-4【后端】【C2】：输入 同一 `fileId`；操作 再发一条消息；预期 4096；用原 `clientMsgId` 重发得到原 ack；错误码 4096。
- AC-FILE-06-5【后端】【C2】：输入 对象存储删除接口返回错误；操作 运行清理；预期 记录保留，指标 `im_job_runs_total{job="file_purge",result="error"}` 增加，下次重试成功；错误码 —。
- AC-FILE-06-6【前端】【C2】：输入 `expired=true` 的图片和文件消息；操作 查看；预期 分别显示"图片已过期""文件已过期"，不发申请地址的请求；错误码 —。
- AC-FILE-06-7【后端】【C2】：输入 `ready` 的文件，`file_purge` 已把它置为 `purging`；操作 `send` 绑定；预期 4096；错误码 4096。
- AC-FILE-06-8【后端】【C2】：输入 同一 `fileId`，两个不同 `clientMsgId` 并发发送；操作 发送；预期 恰好一个成功，另一个 4096，消息未保存；错误码 4096。
- AC-FILE-06-9【后端】【C2】[IM 钟]：输入 `IM_FILE_RETENTION_DAYS=0`；操作 推进 400 天运行清理；预期 文件不被删除；错误码 —。
- AC-FILE-06-10【后端】【C2】[IM 钟]：输入 已过期的文件；操作 查 `files.name`；预期 是空；错误码 —。（v0.3 改：RC2-13，撤回部分拆到 AC-RCL-04-8）
- AC-FILE-06-11【后端】【C2】：输入 u1 退群、解散群；操作 其他有权限的成员申请地址；预期 退群时其他人照常；解散后 404；对象都还在；错误码 4048。（v0.3 改：RC2-13，删除会话部分拆到 AC-CVS-03-15）
- AC-FILE-06-12【前端】【C2】（三套 SDK）：输入 已加载的图片消息过期（`modVer` 增加）；操作 重连；预期 只重拉这个会话，消息变成"已过期"，本地缓存被清除；错误码 —。（RC2-13）

### F-FILE-07 附件发送状态机（P0，R-CF-01）

**规则**
1. 状态：**处理中**（压缩、生成缩略图）→ **上传中** → **待发送**（已 `complete`）→ 发送中 → 已发送；任何一步可以转失败。SDK 新增处理中、上传中两种（现有 `Store.kt` 第 17 行只有 QUEUED、SENDING、SENT、FAILED），本地库要升级。
2. **发送前把源文件复制到 App 沙盒**（Android、iOS），上传成功且消息已发送后删除；失败的保留，供手动重发，退出登录时清除。H5 的 File 对象只在内存。
3. **冷启动**：处理中、上传中一律转为失败，不自动重发（现有 QUEUED 消息重连后自动发，这两种不能走这条路）。待发送的按现有规则自动发 1 次。
4. 手动重发：已有 `fileId` 且状态 `ready` 的，**复用同一 `fileId`，不重传**；`send` 返回 4096（文件超过 24 小时被清理）：有沙盒副本的回到"上传中"重新上传（不算自动重发次数，RC2-23），没有的转失败；上传没完成的从头上传；源文件没了（H5 刷新后）只能删除这条消息，提示"原文件已不可用，请重新选择"。`clientMsgId` 始终不变。
5. **顺序**：同一会话的附件按选择顺序串行上传和 `send`；**文字可以越过上传中的图片**，先拿到 seq（消息按 seq 显示，F-MSG-02）。未确认的消息排在所有有 seq 的消息之后，规则不变（`docs/sdk/README.md` 第 3 节）。
6. 一次最多选 9 个（未校准）；并发上传 1 个。
7. App 退到后台：第一批不保证继续上传（iOS `URLSession` 后台任务、Android 前台服务都不做），回到前台时已失败的显示失败。
8. 上传中的消息不占 10 秒发送超时（F-MSG-03 规则 1 从 `send` 发出时才开始计时）。

**异常**：磁盘空间不足导致复制失败：标记失败，提示"存储空间不足"。`complete` 返回 5032：SDK 退避重试 3 次，再转失败。

**验收标准**
- AC-FILE-07-1【前端】【C2】（Android、iOS）：输入 上传中杀掉 App；操作 重新启动；预期 这条消息显示失败，没有自动上传；点重发后从头上传，成功后对方只收到一条，`clientMsgId` 不变；错误码 —。
- AC-FILE-07-2【前端】【C2】：输入 `complete` 成功但 `send` 超时（故障注入不回 ack）；操作 手动重发；预期 没有新的 PUT 请求，复用同一 `fileId`；对方只收到一条；错误码 —。
- AC-FILE-07-3【前端】【C2】（H5）：输入 上传失败后刷新页面；另一条：上传失败不刷新；操作 查看；预期 刷新后失败消息不再出现（D4，刷新前有离开确认）；不刷新时手动重发成功；错误码 —。
- AC-FILE-07-4【前端】【C2】：输入 依次选 3 张图，第 1 张上传中时输入文字发送；操作 观察；预期 文字先显示已发送；3 张图按选择顺序 `send`，seq 递增；错误码 —。
- AC-FILE-07-5【前端】【C2】（Android、iOS）：输入 从相册选图后，系统相册里删除原图，再重发失败的消息；操作 重发；预期 成功（用沙盒副本）；发送成功后沙盒副本被删除；错误码 —。
- AC-FILE-07-6【前端】【C2】：输入 断网时选图；操作 观察，恢复网络；预期 标记失败，可手动重发；重发成功对方只收到一条；错误码 —。
- AC-FILE-07-7【前端】【C2】：输入 弱网下同一文件上传失败 5 次；操作 查当天额度；预期 额度没有被扣；错误码 —。
- AC-FILE-07-8【后端】【C2】：输入 一个 `kind=image` 的 `ready` 文件，`send` 失败（4036，被对方拉黑）；操作 取消拉黑后重发同 `clientMsgId`；预期 文件仍是 `ready`，可以绑定；错误码 4036。（v0.3 改：RC2-13）
- AC-FILE-07-9【前端】【C2】（Android、iOS）：输入 9 个文件上传失败未处理；操作 退出登录；预期 沙盒副本被清除；错误码 —。
- AC-FILE-07-10【前端】【C2】（Android、iOS）[IM 钟]：输入 待发送的消息，文件 `ready` 超过 24 小时已被清理，沙盒副本还在；H5 同样场景；操作 重发；预期 原生重新上传后发送成功；H5 提示"原文件已不可用，请重新选择"；错误码 4096。（RC2-23）

---

## 7. C3 离线推送

### F-APN-01 设备令牌（P0）

**规则**
1. WS 命令 `deviceNotificationRegister {provider: "apns"|"fcm", token, env?: "sandbox"|"production", appId}`；`deviceNotificationUnregister {}`（和现有系统通知的 `push`、`pushSync` 区分，R-CF-26）。每账号每小时 10 次（未校准，4290，R-CS-31）。令牌绑定当前 `sid`。**没有 `sid` 的连接（演示、管理接口 token）**：令牌只绑定 `userId`，按"该用户没有任何活跃连接"判断是否推送，退出时由客户端注销（测试环境用）。
2. 同一个令牌只属于一个用户：另一个账号在同一设备登录时，把令牌从旧账号移走，**"显示发送者和内容"重置为关**（R-CS-03）。这个设置存在 `device_sessions`（无 `sid` 时存令牌行），令牌刷新不影响（R-CB-23）。第一批不做令牌持有证明（要先拿到别人的令牌才能利用，风险低）。
3. 通道返回令牌失效（APNs 410 / `BadDeviceToken`、FCM `UNREGISTERED`）：删除这条令牌。
4. **不变量：`sid` 吊销或过期，同一事务删除绑定的令牌**。覆盖退出、改密、退出其他设备、找回、refresh 重放、refresh 过期（`session_purge`）、强制下线、封禁、注销（R-CS-03）。**发送前按当前 `sid` 状态复查**，已入队的通知不发（R-CB-23）。
5. 权限：Android 13 及以上（API 33，Manifest 要加 `POST_NOTIFICATIONS`，现在没有）和 iOS 都要系统授权（含 `.badge`）。用户拒绝时不注册令牌，"设置 → 通知"显示"系统通知已关闭"和前往系统设置的入口。不在首次启动就弹授权，在首次登录进入会话列表后弹。
5a. **每次回到前台对比系统授权状态和令牌**：开了就补注册，关了就注销（Android 13 以下用户也能在系统里关，R-CF-08）。令牌刷新回调可能发生在没有连接时，先缓存，连上后注册。
5b. iOS 工程现在没有 `.entitlements`，要加 `aps-environment`。Android：**CI 和开发用占位 `google-services.json`，真实配置只在发布时注入**，不进仓库（R-CQ-21）。
6. 令牌不进日志和审计（只记 provider 和令牌的 SHA-256 前 8 位）。

**异常**：`deviceNotificationRegister` 令牌为空或超长：400 / 4000。App 拿不到令牌（系统错误）：不报错，下次启动重试，设置页显示状态。令牌刷新：客户端重新注册，覆盖旧值。

**验收标准**
- AC-APN-01-1【后端】【C3】：输入 u1 注册令牌 T；操作 u2 用同一 T 注册；给 u1、u2 各发消息；预期 T 只归 u2，只收到 u2 的通知；错误码 —。
- AC-APN-01-2【后端】【C3】：输入 假通道返回 APNs 410；FCM `UNREGISTERED`；操作 发一次推送；预期 令牌被删除，下一条消息不再调用通道；错误码 —。
- AC-APN-01-3【后端】【C3】：输入 u1 有令牌；操作 分别退出、改密（其他设备）、退出其他设备、找回、refresh 重放、强制下线、封禁；预期 相应令牌被删除，之后的消息不调用通道；错误码 —。（v0.2 改：R-CS-03）
- AC-APN-01-4【后端】【C3】：输入 注册令牌；操作 扫描日志和审计；预期 搜不到令牌原文；错误码 —。
- AC-APN-01-5【前端】【C3】（Android 13+、iOS）：输入 用户拒绝通知授权；操作 打开"设置 → 通知"；预期 显示"系统通知已关闭"和入口；没有发注册命令；错误码 —。（v0.2 改：R-CF AC 表）
- AC-APN-01-6（手工）【C3】：输入 Android 13+、iOS 真机；操作 首次登录；预期 进入会话列表后才出现系统授权弹窗；同意后设备收到测试通知；错误码 —。
- AC-APN-01-7【后端】【C3】：输入 无 `sid` 的 token（演示）注册令牌；另有活跃连接 / 没有连接；操作 发消息；`deviceNotificationUnregister`；预期 有连接时不推；没有连接时推；注销后不推；错误码 —。（R-CQ-10）
- AC-APN-01-8【后端】【C3】：输入 A 开了"显示发送者和内容"后退出，B 用同一令牌注册；操作 给 B 发消息；预期 通知不含内容；错误码 —。
- AC-APN-01-9【后端】【C3】：输入 消息入队后、通道调用前吊销 `sid`；操作 观察假通道；预期 没有调用；错误码 —。
- AC-APN-01-10【前端】【C3】（Android、iOS）：输入 已注册令牌；操作 在系统设置关闭通知后回到前台；再打开回到前台；预期 依次发注销、注册；错误码 —。
- AC-APN-01-11【前端】【C3】（Android）：输入 API 30 模拟器，系统设置里关闭通知；操作 回到前台；预期 发注销，设置页显示已关闭；错误码 —。
- AC-APN-01-12【前端】【C3】（Android）：输入 占位 `google-services.json`；操作 CI 构建 debug 和测试；预期 构建通过；错误码 —。
- AC-APN-01-13【后端】【C3】：输入 同一账号 1 小时第 11 次注册；令牌为空；操作 注册；预期 429；400；错误码 4290、4000。

### F-APN-02 什么时候推（P0）

**规则**
1. 推送的消息：`text`、`image`、`file`。**不推**：`tip`、`custom`、自己发的、撤回和删除事件（详情模式的替换通知除外，F-APN-03 规则 4）、系统通知 `push` 帧、好友申请（第 2.4 节）。
2. 按**设备**判断，不按用户：设备有活跃连接就不推；手机后台 + PC 在线，手机照样推（第一批不做"PC 在线手机静音"）。
3. 活跃连接：最近 `IM_PUSH_LIVE_WINDOW`（45 秒）内收到过帧，且状态是前台。原生 SDK 用 `presence {state: "foreground"|"background"}` 上报（普通带 `id` 的命令，ack `{}`，不用改 gateway 的特判，R-CB-26）。**`auth` 成功后第一帧必须上报当前状态；`platform` 是 android、ios 的连接在收到之前按后台处理**（重连时才不会漏通知，R-CF-06）。`web`、`pc` 默认前台，不上报。
4. **断线补推**（R-CB-11）：只对关闭码 1006、4408 触发（滚动重启的 1001 不触发）。**延迟 30 秒**，期间同一设备重连则取消；该设备没有其他连接，断开前 120 秒内下发过消息且仍有未读 → 补发一条聚合通知"你有新消息"。全局限速每秒 100 条（未校准）。这是为了覆盖"连接已死但服务端还以为在线"这段时间。
5. 免打扰按 F-MEN-03。被踢下线的设备没有令牌，不推。
6. 合并和折叠（R-CF-07、R-CS-04、R-CB-22）：同一设备同一会话 5 秒内最多 1 条，后续合并。**隐藏模式**：每设备一个固定的不透明 id，所有会话折叠成一条（不泄露"哪些通知属于同一会话"）。**详情模式**：id = `HMAC(服务端密钥, sid + convId)` 截断。APNs 用 `apns-collapse-id` 请求头；**FCM 不用 `collapse_key`**（官方文档：每设备只保留 4 个，通知消息还会忽略它，评审 R-CF-07 已查）；Android 用通知 `tag`（未核实）。
7. **推送不影响发送路径**（R-CB-12）：在 `send` 的 ack 之后异步入队。**队列单位是消息**（`IM_PUSH_QUEUE_MAX`，默认 1 万，未校准），满了丢弃并计数。worker **批量查询**：一条消息一次取齐全部收件人的令牌、免打扰和拉黑，不逐人查。角标用聚合查询加 60 秒缓存；离线设备超过 200 的群消息不带 `badge`（未校准）。进程崩溃丢失的只是通知，消息靠 `sync` 不丢。SQLite 只有一条连接，worker 占用要控制（AC-APN-02-12）。
8. 重试：可重试的错误（429、5xx）首次调用外最多重试 2 次（总计 3 次调用）；通知过期时间隐藏模式 24 小时，详情模式 1 小时（R-CS-03）。

**异常**：通道长时间不可用：重试用尽后丢弃并计数，不积压，不影响消息。用户在通知到达前已读：通知仍可能显示（规则见 F-APN-04 规则 2）。

**验收标准**
- AC-APN-02-1【后端】【C3】：输入 u2 无连接、有令牌，u1 发 `text`、`image`、`file`；操作 观察假通道；预期 各 1 次调用（超过合并窗口发送）；错误码 —。
- AC-APN-02-2【后端】【C3】：输入 u2 在线（前台活跃）；操作 u1 发消息，再给离线的哨兵用户发一条；预期 哨兵被调用后，u2 的令牌没有调用；错误码 —。（v0.2 改：R-CQ-29）
- AC-APN-02-3【后端】【C3】：输入 u2 手机连接上报 `background`；PC 连接活跃；操作 u1 发消息；预期 手机令牌有调用；错误码 —。
- AC-APN-02-4【后端】【C3】：输入 隐藏模式；群提示消息；发送者自己的其他离线设备；操作 观察假通道；预期 都没有调用；错误码 —。（v0.2 改：R-CQ-09、R-CQ-18，撤回部分拆到 AC-RCL-03-9）
- AC-APN-02-5【后端】【C3】[IM 钟]：输入 u2 连接 46 秒没有帧（测试模式下关闭服务端空闲断开）；操作 u1 发消息；预期 假通道有调用；错误码 —。
- AC-APN-02-6【后端】【C3】[IM 钟]：输入 u2 前台在线时收到消息未读；操作 连接以 1006 断开，30 秒内重连；以 1006 断开不重连；以 4408 断开不重连；以 1000、1001 断开；预期 第 1 种无调用；第 2、3 种在 30 秒后收到 1 条聚合通知；第 4 种无调用；错误码 —。（v0.2 改：R-CB-11）
- AC-APN-02-7【后端】【C3】：输入 u1 在 5 秒内给离线的 u2 发 10 条；操作 观察假通道；预期 调用不超过 2 次，collapse id 相同；错误码 —。
- AC-APN-02-8【后端】【C3】：输入 假通道一直超时；同一场景通道正常；每种 ≥ 1000 条消息，生产配置，3 次取最差；操作 单聊发送；预期 全部 ack 成功，通道超时时 ack P99 ≤ 200 ms（未校准，开发机参考，目标机重测，增幅只记录）；`sync` 不缺；错误码 —。（v0.2 改：R-CB-27、R-CQ-22）
- AC-APN-02-9【后端】【C3】：输入 假通道一直 503；返回 503 一次后成功；返回 400；操作 发消息；预期 依次总调用 3 次、2 次、1 次，指标按结果计数；断言 `apns-expiration` 和 FCM `ttl`：隐藏 24 小时、详情 1 小时；错误码 —。（v0.2 改：R-CQ-30）
- AC-APN-02-10【后端】【C3】：输入 `IM_PUSH_QUEUE_MAX=10`，假通道阻塞；操作 继续发消息；预期 `send` 不受影响，指标 `im_offline_push_dropped_total` 增加；错误码 —。（v0.2 改：R-CQ-01）
- AC-APN-02-12【后端】【C3】：输入 2000 人群、1000 人离线有令牌，生产配置，≥ 1000 条，3 次取最差；操作 成员发消息，同时观察其他单聊的 ack；预期 群发送和单聊 ack P99 都 ≤ 200 ms（未校准，开发机参考，目标机重测）；数据库查询次数不随离线人数线性增长（每条消息 ≤ 5 次）；错误码 —。（R-CB-12）
- AC-APN-02-13【后端】【C3】：输入 android 账号连接已鉴权，还没有上报 `presence`；重连后上报 `background`；操作 对方发消息；预期 都有调用；错误码 —。（R-CF-06）
- AC-APN-02-14【后端】【C3】：输入 上报 `foreground` 后；操作 对方发消息；预期 无调用（哨兵法）；错误码 —。
- AC-APN-02-15【前端】【C3】（Android、iOS）：输入 App 在后台切换网络导致重连；操作 观察上行帧；预期 `auth` ack 后第一帧是 `presence {state:"background"}`；错误码 —。
- AC-APN-02-16【后端】【C3】：输入 `custom` 消息；系统通知 `push` 帧；好友申请；操作 目标离线；预期 无调用（哨兵法）；错误码 —。
- AC-APN-02-17【后端】【C3】：输入 `IM_OFFLINE_PUSH_ENABLED=0`；操作 注册令牌；发消息；预期 注册 403，`reason=featureDisabled`；无调用；错误码 4030。

### F-APN-03 内容与隐私（P0，CH-11）

**规则**
1. **默认**：标题"天听"，正文"你收到一条新消息"。**不含发送者、会话名、内容**。整个请求（**含请求头**）不含 `convId`、`userId`、`msgId` 及它们的普通哈希。原因：内容会经过苹果和谷歌的服务器。角标数字保留，会暴露未读数量，CH-11 里说明（R-CS-04）。
2. 用户可在"设置 → 通知"打开"显示发送者和内容"。打开前提示内容会经过第三方通道。服务端保存这个设置（每台设备独立），`deviceNotificationSettings {showDetail}`，ack 返回当前值，没有令牌时也可以设置（存 `device_sessions`，R-CF-22）。这是接收方的决定，发送方的内容因此经过第三方，隐私说明要写（R-CS CH-11）。
3. 打开时：标题为显示名（**服务端只有昵称，不用备注**，PRD-social 5.5 节）或群名，正文是消息摘要（最多 50 个码点）；图片"[图片]"，文件"[文件]"，不带文件名。
4. 撤回或删除后（尽力而为，F-ADM-31 规则 6 仍然有效，R-CB-22）：详情模式下，**只有这条消息是这台设备这个会话最后推送的一条时**，才发一条同 collapse id 的通知，文字替换为"一条消息已被撤回"，否则会盖掉更新的通知。记录放内存 LRU（24 小时），不落库，重启丢失。隐藏模式不发。
5. 系统通知栏里的内容属于操作系统，App 控制不了；发送时这里说清楚。

**异常**：详情模式下昵称为空：标题用 `userId`。摘要含控制字符：先去掉。替换通知发送失败：不重试，不告警。

**验收标准**
- AC-APN-03-1【后端】【C3】：输入 默认设置，消息含特征串 `ZXQ-PUSH-1`，发送者昵称 `ZXQ-NICK`；操作 抓假通道收到的完整请求（**请求头和请求体**）；预期 搜不到特征串、昵称、任何 userId、`convId`、`msgId`；collapse id 对两个不同会话相同（每设备固定）；错误码 —。（v0.2 改：R-CQ-28、R-CS-04）
- AC-APN-03-2【后端】【C3】：输入 用户打开详情，发送者设了备注；操作 推送；预期 标题是昵称，正文是摘要（超长时 50 个码点）；图片为"[图片]"；文件为"[文件]"不带文件名；错误码 —。
- AC-APN-03-3【后端】【C4】：输入 详情模式，m1 已推送后被撤回；m1、m2 都推送后撤回 m1；隐藏模式同样操作；操作 观察假通道；预期 第一种收到同 collapse id 的替换通知；第二种不发（否则盖掉 m2）；隐藏模式没有；错误码 —。（v0.2 改：R-CB-22）
- AC-APN-03-4【前端】【C3】（Android、iOS）：输入 "设置 → 通知"；操作 打开"显示发送者和内容"；预期 先有第三方通道提示，确认后才生效；重启后保留；错误码 —。
- AC-APN-03-5（手工）【C3】：输入 Android、iOS 真机锁屏，默认设置；操作 收到消息；预期 锁屏上只显示"天听 / 你收到一条新消息"；错误码 —。
- AC-APN-03-6【后端】【C3】：输入 详情模式，两个不同会话、两台设备；操作 抓 `apns-collapse-id`；预期 都不等于也不包含 `convId`；不同设备同一会话的值不同；FCM 请求没有 `collapse_key`；错误码 —。
- AC-APN-03-7【后端】【C3】：输入 同一账号两台设备，一台开详情；群消息详情模式；操作 推送；预期 只有开启的设备带内容；标题是群名；错误码 —。
- AC-APN-03-8【前端】【C3】（Android、iOS）：输入 令牌还没拿到；操作 打开详情开关；重新读取；预期 设置保存成功，显示当前值；错误码 —。

### F-APN-04 点击与角标（P0）

**规则**
1. 隐藏模式：点击进入会话列表（数据字段里没有会话）。详情模式：通知带 `convId`，走现有 deep link（`tianting://chat/{convId}`，PRD-ui 3.6 节，只接受已知路由），未登录先去登录页，登录账号不同就进会话列表。
2. iOS 角标 = 发送时的总未读（口径见 F-CVS-02 规则 4；大群不带，F-APN-02 规则 7），App 回到前台时刷新。第一批不保证跨设备已读后清除已送达的通知和角标。
3. Android 使用通知渠道"消息"；角标由系统决定，不承诺。
4. 进入前台：清除本 App 已显示的通知。

**异常**：点通知时登录已失效：先按 F-ACC-03 刷新，失败回登录页。详情模式通知里的会话已删除：进会话列表。

**验收标准**
- AC-APN-04-1（手工）【C3】：输入 隐藏模式，App 被系统杀死；操作 点通知；预期 启动进入会话列表，未读正确；错误码 —。
- AC-APN-04-2（手工）【C3】：输入 详情模式；操作 点通知；预期 直接进入该会话；错误码 —。
- AC-APN-04-3【前端】【C3】（Android、iOS）：输入 deep link 的 `convId` 不属于当前账号；操作 打开；预期 进会话列表，不显示任何内容；错误码 —。
- AC-APN-04-4【后端】【C5】：输入 u2 有 3 条普通未读、免打扰会话 2 条未读（其中 1 条 @ 他）；操作 再来一条普通消息，推送到假 APNs；预期 `badge=5`（3 + 新 1 + 提及 1，见 F-CVS-02 规则 4）；错误码 —。（v0.2 改：R-CF-20）
- AC-APN-04-5【后端】【C3】：输入 群里 300 个离线设备；操作 推送；预期 请求不带 `badge`；错误码 —。（R-CB-12）
- AC-APN-04-6【前端】【C3】（Android、iOS）：输入 通知栏有 2 条本 App 通知；操作 回到前台；预期 被清除；错误码 —。

### F-APN-05 通道配置（P0，CH-12）

**规则**
1. APNs：token 认证（`.p8`），配置 `IM_APNS_KEY_FILE`、`IM_APNS_KEY_ID`、`IM_APNS_TEAM_ID`、`IM_APNS_TOPIC`（bundle id，现在是 `com.tianting.im`）。sandbox 和 production 按令牌的 `env` 选端点。
2. FCM：HTTP v1 接口，`IM_FCM_CREDENTIALS_FILE`（服务账号）、项目 ID；App 里放 `google-services.json`（包名现在是 `com.tianting.app`）。
3. 密钥文件权限必须 0600，不进仓库；配了开关但缺密钥：生产拒绝启动，非生产告警并关闭该通道。
4. **im-server 要能访问外网**：`api.push.apple.com`、`fcm.googleapis.com`、`oauth2.googleapis.com`。服务器部署在中国大陆时到谷歌的网络通常不通，FCM 要走出境代理或换部署地（未核实具体路径，CH-13）。
5. 服务端通道用接口隔离（`push.Provider`），测试用假通道，以后可加厂商通道。

**异常**：凭证过期或被吊销（通道返回 401 / 403）：该通道的调用全部计失败，触发 F-OPS-07 指标，不影响消息。Key 轮换：改配置文件后重启。

**验收标准**
- AC-APN-05-1【后端】【C3】：输入 `IM_ENV=production`、开关打开、缺 APNs 密钥；密钥文件权限 0644；操作 启动；预期 都是非 0 退出，错误信息不含密钥内容；错误码 —。
- AC-APN-05-2【后端】【C3】：输入 配置检查接口；操作 `GET /internal/v1/config`；预期 显示通道是否启用、key id、文件存在、权限正确、最近一次调用结果，不含密钥；错误码 —。（v0.2 改：R-CQ-31）
- AC-APN-05-3（手工）【C3】：输入 Frank 提供的 APNs 和 FCM 凭证，一个真实有效令牌和一个已失效令牌；操作 向真机发送；预期 Android（有谷歌服务）和 iOS 都在 10 秒内收到（未校准）；失效令牌被删除；错误码 —。（v0.2 改：R-CQ-20）
- AC-APN-05-4【后端】【C3】：输入 假 APNs 校验 ES256 JWT、`apns-topic`、`apns-push-type`；假 FCM 校验 OAuth；`env=sandbox`、`env=production` 两种令牌；操作 发送；预期 签名和头都通过；sandbox 走 sandbox 端点；错误码 —。（R-CQ-20）
- AC-APN-05-5【后端】【C3】：输入 非生产环境缺 FCM 密钥；假通道返回 403；操作 启动；发消息；预期 启动告警，该通道关闭；403 计失败，消息不受影响；错误码 —。

### F-APN-06 没有 FCM 的 Android（P0，CH-13）

**规则**
1. 第一批**不做**国内厂商通道，也不做后台常驻（耗电，且多数厂商会杀）。
2. App 启动时用 Google Play 服务可用性判断：不可用就不注册令牌，"设置 → 通知"显示"此设备暂不支持离线通知，打开 App 才能收到消息"。
3. 消息本身不丢，打开 App 时 `sync` 补齐。
4. 面向中国大陆 Android 用户开放前必须重新决定（CH-13）。

**异常**：Google Play 服务可用但被用户禁用：按不可用处理。服务后来变为可用：下次启动注册。

**验收标准**
- AC-APN-06-1【前端】【C3】（Android）：输入 Google Play 服务不可用（测试替身）；操作 登录后打开"设置 → 通知"；预期 没有发 `deviceNotificationRegister`；显示上述文案；错误码 —。
- AC-APN-06-2（手工）【C3】：输入 没有谷歌服务的国内 Android 真机，App 退到后台被杀；操作 对方发 3 条消息后打开 App；预期 没有通知；打开后 3 条都在；错误码 —。
- AC-APN-06-3【后端】【C3】：输入 没有令牌的用户；操作 给他发消息；预期 不报错，无调用；消息在 `sync` 里；错误码 —。

---

## 8. C4 撤回、引用、@

### F-RCL-01 发送者撤回（P0）

**规则**
1. WS 命令 `recall {convId, msgId}`。发送者可以在消息 `ts` 后 120 秒内撤回（按服务端时间，[IM 钟]）。超时 4097。
2. 适用于 `text`、`image`、`file`。`custom` 不提供客户端入口（AC-UI-09-7 不变），服务端按发送者规则照常处理。`tip` 400 / 4000。自己给自己的消息也可以。
3. 校验顺序：格式 → 会话权限（4032、4033、4043）→ 消息存在（4044）→ 已撤回（幂等成功，不再发事件）→ 已被后台删除（4044）→ 权限和时限。
4. **禁言和被拉黑不影响撤回**（撤回是收回内容，不是发言）。不计入发送限流；单独限制每人每分钟 20 次（未校准，4290）。
5. 服务端：同一事务里清空正文，写 `recalled_at`、`recalled_by`；seq 不变不留空洞；原文和文件名进隔离区（CH-17）；如果是最后一条，更新 `conv_seq` 摘要；递增 `conv_seq.mod_ver`。依赖 `IM_MSG_ACTIONS_ENABLED`（R-CB-06）。
5a. **线上表示**（R-CB-05、R-CF-04，判断见第 23 节）：下行的消息和 `lastMsg` **同时带 `deleted=true` 和 `recalled=true`**，语义是"`deleted` 表示没有正文，`recalled` 表示原因"。事件也复用 `msgDeleted`，增加可选字段 `recalled: {by, at}`，不新增事件名。这样老版本 SDK 会清除原文并显示"该消息已被删除"；新 SDK 看到 `recalled` 显示撤回文案。存储层 `deleted_at` 和 `recalled_at` 分开，后台接口的 `deleted` 只表示后台删除。
6. 未读数公式不变（PRD-core F-CONV-02 规则 1），撤回不减未读；`updatedAt` 不变，会话不重新排序。

**异常**：和发送并发：消息未保存前撤回 4044。网络超时后客户端重试 `recall`：幂等。服务端事务失败：5000，消息不变。

**验收标准**
- AC-RCL-01-1【C4】[IM 钟]：输入 u1 发文本后 60 秒；操作 u1 撤回；预期 ack 成功；u2 和 u1 的另一端 2 秒内收到 `msgDeleted`，带 `recalled: {by:u1, at}`；错误码 —。
- AC-RCL-01-2【后端】【C4】[IM 钟]：输入 消息 `ts` 后 119 秒、恰好 120 秒、121 秒；操作 分别撤回；预期 前两个成功；第三个错误，消息不变；错误码 4097。（v0.2 改：R-CQ-41）
- AC-RCL-01-3【后端】【C4】：输入 u2 撤回 u1 的单聊消息；非参与方撤回；`msgId` 属于其他会话；`tip` 消息；操作 分别请求；预期 依次 4030、4032、4044、4000；错误码 4030、4032、4044、4000。
- AC-RCL-01-4【后端】【C4】：输入 已撤回的消息；操作 再撤回；预期 成功，不再发事件；错误码 —。
- AC-RCL-01-5【后端】【C4】[IM 钟]：输入 被禁言的 u1；被对方拉黑的 u1；操作 撤回 60 秒前自己的消息；预期 都成功；错误码 —。
- AC-RCL-01-6【后端】【C4】：输入 消息含特征串 `ZXQ-RCL-1`（文件消息的文件名含 `ZXQ-RCL-3`）；操作 撤回并做 SQLite 检查点后查 `messages`、`conv_seq`、`files` 原始字节；预期 都搜不到；隔离表里也搜不到明文；错误码 —。（v0.2 改：R-CB-27、R-CS-23。只验 SQLite；Postgres 的死元组要等 VACUUM，备份里的原文按备份保留期留存，所以对用户的措辞是"不再显示"，R-CS-34）
- AC-RCL-01-7【后端】【C4】：输入 u2 有 3 条未读，其中 1 条被撤回；操作 查 `convs`；预期 未读仍是 3，`updatedAt` 不变；错误码 —。
- AC-RCL-01-8【前端】【C4】[IM 钟]：输入 自己发的消息 60 秒内、121 秒后（按 `ts` 和校正后的服务端时间，不用设备时钟）；操作 长按；预期 前者有"撤回"，后者没有；菜单打开期间过了时限，点击后提示"发送时间超过 2 分钟，无法撤回"；错误码 4097。
- AC-RCL-01-9【后端】【C4】：输入 给自己发的消息；`custom` 消息（测试环境打开）；1 分钟内第 21 次撤回；操作 撤回；预期 前两个成功；第三个 429；错误码 4290。
- AC-RCL-01-10【后端】【C4】：输入 `IM_MSG_ACTIONS_ENABLED=0`；开启但缺 `IM_QUARANTINE_KEY`；`IM_V13_ADMIN_FEATURES=0`；操作 撤回；启动；撤回；预期 依次 403（`reason=featureDisabled`）、启动失败、开关开时不依赖 V13 也能撤回；错误码 4030。
- AC-RCL-01-11【后端】【C4】：输入 故障注入隔离写失败；重复 `recall`；操作 请求；预期 500，消息不变；恢复后重试成功且只发一次事件；错误码 5000。

### F-RCL-02 群主、管理员撤回（P0）

**规则**
1. 群主可以撤回群里任何成员（含管理员）的消息；管理员可以撤回普通成员的消息，不能撤回群主和其他管理员的。操作者撤回自己的消息时，取发送者规则和角色规则中更宽的时限。
1a. 文案（R-CQ-12、RC2-22，所有情况都在这张表，聊天页和列表摘要相同）：

   | 场景 | 操作者看 | 被撤回者看 | 其他人看 |
   |---|---|---|---|
   | 单聊，自己撤回 | 你撤回了一条消息 | — | 对方撤回了一条消息 |
   | 群聊，自己撤回 | 你撤回了一条消息 | — | {操作者显示名}撤回了一条消息 |
   | 群主、管理员撤回别人的 | 你撤回了 {被撤回者显示名} 的一条消息 | {操作者显示名}撤回了你的一条消息 | {操作者显示名}撤回了一条成员消息 |

1b. 客户端判断菜单（R-CF AC 表）：消息不带发送者角色，客户端只知道自己的 `myRole`。群主显示"撤回"（7 天内）；管理员对别人的消息也显示"撤回"，**服务端返回 4030 时提示"你没有权限撤回这条消息"**（对方可能是管理员），不额外下发角色。
2. 时限 7 天（`IM_GROUP_RECALL_WINDOW_SEC`）。超时 4097。更早的违规内容走举报和后台删除。
3. 托管群没有群主时，任何成员都不能撤回他人消息；业务方或后台指定了群主后按规则 1（和 PRD-social F-SOC-20 矩阵、F-SOC-30 一致）。
4. 角色不缓存，每次查库（PRD-social 第 12 节"群角色"）。
5. 群内自治操作不写后台审计；消息行保留 `recalledBy`，后台元数据可以查到是谁撤回的。

**异常**：角色在操作时刚被撤销：按查库结果 4030。被撤回者已退群：仍可以撤回。解散的群：4043。

**验收标准**
- AC-RCL-02-1【后端】【C4】：输入 自建群：群主 o、管理员 a、a2、成员 m；操作 o 撤回 a 的消息；a 撤回 m 的；a 撤回 a2 的；a 撤回 o 的；m 撤回 a 的；预期 前两个成功；后三个错误；错误码 4030。
- AC-RCL-02-2【后端】【C4】[IM 钟]：输入 m 的消息 6 天、8 天前；操作 o 分别撤回；预期 前者成功；后者错误；错误码 4097。
- AC-RCL-02-3【后端】【C4】：输入 托管群里被后台指定了群主的情况、没有群主的情况；操作 成员撤回他人消息；预期 有群主时按规则 1；没有群主时错误；错误码 4030。
- AC-RCL-02-4【C4】：输入 a 撤回 m 的消息；操作 a、m、其他成员 n 查看聊天页和会话列表（最后一条）；预期 按规则 1a 第 3 行逐项一致；错误码 —。（v0.3 改：RC2-22）
- AC-RCL-02-5【前端】【C4】：输入 管理员长按其他管理员的消息；操作 点"撤回"；预期 提示"你没有权限撤回这条消息"，消息不变；错误码 4030。
- AC-RCL-02-6【后端】【C4】：输入 m 已退群，他的消息；解散的群；角色刚被取消的 a；操作 o 撤回；对解散群撤回；a 撤回；预期 成功；4043；4030；错误码 4043、4030。
- AC-RCL-02-7【后端】【C4】[IM 钟]：输入 群主自己 1 天前的消息；操作 撤回；预期 成功（取更宽时限）；错误码 —。

### F-RCL-03 撤回的展示与同步（P0）

**规则**
1. 在线：`event msgDeleted {convId, msgIds, seqs, recalled: {by, at}}`（规则 F-RCL-01 第 5a 条），发给全部在线参与者（含操作者其他端）。
2. 离线和错过事件：`sync` 返回 `deleted=true`、`recalled=true`、没有 `body` 的消息。**`conv_seq` 增加 `mod_ver`**，撤回、后台删除、文件过期时递增，`convs` 返回 `modVer`；**SDK 只重拉 `modVer` 变化的会话**的已加载范围，代替现有每次全部重拉（R-CB-29；Kotlin 现在每次重连都重拉，`IMClient.kt` 第 919 行起）。已核实：原生端已加载范围跨冷启动保留（Kotlin `restore`，每会话最多 200 条；Swift `restoredUnverified`）。
2a. **SDK 合并规则改变**（R-CB-04、R-CF-05）：现有 Store 对已有消息只接受"变成已删除"（`store.ts` 第 96～99 行、`Store.kt` 第 154～157 行、`Store.swift` 第 120～123 行），改为**单调合并**（RC2-10）：`deleted`、`recalled`、`expired` 只能从假变真；`quote.state` 只能从 `normal` 往后走，不会变回 `normal`，`quote.text` 清除后不恢复；`deletedIds` 继续优先。这样迟到的旧 `sync` 应答（读库之后、应答之前发生了撤回）不会把原文写回。现有 `markDeleted`、`deletedLast`、Kotlin `DeletedRefs` 也要带上 `recalled`。
3. 客户端：文案按 F-RCL-02 规则 1a 的表（所有情况在一张表里，RC2-22）；**清除本地内存、本地数据库、已缓存的缩略图和文件**。
4. `lastMsg`：`deleted=true`、`recalled=true`、没有 `text`、带 `recalledBy`；会话列表显示同一张表的"列表摘要"。
5. 已下载到系统相册或文件夹的不能收回；已显示的系统通知按 F-APN-03 规则 4。
6. 撤回和删除的消息没有长按菜单（F-UI-09 规则 5 扩展）。
7. 老版本客户端（已按代码核实）：不认识 `recalled`，但认识 `deleted`，按规则 F-RCL-01 第 5a 条会清除原文并显示"该消息已被删除"。不做最低版本限制（没有证据表明有已发布的老版本在用，第 21 节）。
8. 撤回提示行提供"举报"入口（F-MOD-03 规则 2a）。
9. 下载中的文件被撤回：Android、iOS 中止下载并提示。本地文件删除失败：下次启动再清。

**异常**：事件丢失：靠规则 2 补齐。正在播放或下载被撤回的文件：取消下载并提示。本地文件删除失败：下次启动再清。

**验收标准**
- AC-RCL-03-1【前端】【C4】（sdk-ts）：输入 已加载含特征串 `ZXQ-RCL-2` 的消息；操作 收到带 `recalled` 的 `msgDeleted` 后导出内存消息；预期 搜不到；错误码 —。
- AC-RCL-03-2【前端】【C4】（sdk-kotlin、sdk-swift）：输入 本地库有含特征串的消息和缓存的缩略图；操作 收到带 `recalled` 的 `msgDeleted` 后检查数据库文件和缓存目录；预期 搜不到，缓存文件不存在；错误码 —。
- AC-RCL-03-3【C4】：输入 u2 已加载消息后断网，这时 u1 撤回；操作 u2 恢复网络；预期 重连后消息变为撤回文案；错误码 —。
- AC-RCL-03-4【前端】【C4】（sdk-kotlin、sdk-swift）：输入 u2 本地库有消息，App 被杀死，这时 u1 撤回；操作 u2 冷启动并连接；预期 消息变为撤回文案，本地库搜不到原文；错误码 —。
- AC-RCL-03-5【后端】【C4】：输入 最后一条被撤回；操作 查 `convs`；预期 `lastMsg.deleted=true`、`recalled=true`，没有 `text`，`modVer` 增加；错误码 —。
- AC-RCL-03-6【前端】【C4】：输入 群里 u1 撤回自己的最后一条；操作 u1 和其他成员看会话列表；预期 按 F-RCL-02 规则 1a 第 2 行：u1 看到"你撤回了一条消息"，其他成员看到"{u1 显示名}撤回了一条消息"；错误码 —。（v0.3 改：RC2-22）
- AC-RCL-03-7【后端】【C4】：输入 撤回的消息；操作 用 `protocol/vectors` 校验 `msg`、`sync`、事件帧；预期 符合 Schema，没有 `body`，`deleted`、`recalled` 都是 true；错误码 —。
- AC-RCL-03-8【前端】【C4】：输入 正在全屏看的图片被撤回；操作 观察；预期 退出全屏，提示"该消息已被撤回"；错误码 —。
- AC-RCL-03-9【后端】【C4】：输入 隐藏模式，消息被撤回；操作 观察假通道（哨兵法）；预期 没有调用；错误码 —。（从 AC-APN-02-4 拆出）
- AC-RCL-03-10【前端】【C4】（三套 SDK）：输入 单聊撤回；操作 查看；预期 对方看到"对方撤回了一条消息"，自己看到"你撤回了一条消息"；长按提示行不出现菜单；错误码 —。
- AC-RCL-03-11【前端】【C4】（三套 SDK）：输入 u2 重连时只有 c1 的 `modVer` 变了；操作 观察请求；预期 只对 c1 发 `sync` 重拉；错误码 —。
- AC-RCL-03-12【前端】【C4】（Android、iOS）：输入 文件正在下载；操作 对方撤回；预期 下载中止，提示"该消息已被撤回"，不留部分文件；错误码 —。
- AC-RCL-03-13【前端】【C4】（三套 SDK）：输入 m1 已被撤回（本地已清除），迟到的旧 `sync` 应答带 `deleted=false` 和原文；引用 m1 的 m2 迟到应答带 `quote.state=normal`；操作 合并；预期 m1、m2 都不恢复原文，本地库搜不到特征串；错误码 —。（RC2-10；同时进 `chat-sync.json`）

### F-RCL-04 和后台删除的关系（P0，CH-17）

**规则**

| | 撤回 | 后台删除（F-ADM-31） |
|---|---|---|
| 谁 | 发送者；群主、管理员 | 后台 `moderator` 或有 `msg.delete` 的调用方 |
| 时限 | 有（F-RCL-01、02） | 无 |
| 理由、审计 | 不要理由；不进后台审计；行内保留 `recalledBy` | 必填理由；审计 |
| 事件 / 字段（下行） | `msgDeleted` 带 `recalled`；消息 `deleted` + `recalled` | `msgDeleted`；消息 `deleted` |
| 文案 | "…撤回了一条消息" | "该消息已被删除" |
| 原文 | 默认进隔离区（CH-17） | 进隔离区（F-ADM-32） |

1. 后台删除已撤回的消息：幂等成功（F-ADM-31 规则 7 扩展），状态和文案保持撤回，审计照写，不发事件。
2. 撤回已被后台删除的消息：4044。
3. 后台元数据查询增加 `recalled`、`recalledAt`、`recalledBy`；普通正文授权对撤回消息只返回 `recalled=true`；隔离授权返回原文（单聊仍受 `IM_C2C_BODY_ACCESS` 限制）。隔离接口"只返回被删的"扩展到撤回的。**后台正文接口不能直接用下行的 `logic.ToProto`（`admin/moderation.go` 第 623、647 行）**，否则撤回的消息在后台显示成"已删除"，要分下行版和后台版（RC2-21）。
4. **面向用户的说明要写清楚**：撤回后对方看不到，但平台会按保留期留存用于处理投诉（隐私政策要写，CH-23）。

**异常**：隔离写入失败：整个撤回事务回滚（5000），消息不变，和 F-ADM-32 同一事务的要求一致。

**验收标准**
- AC-RCL-04-1【后端】【C4】：输入 `IM_V13_ADMIN_FEATURES=1`，已撤回的消息；操作 `POST /admin/messages/delete`；查后台元数据；预期 200，后台 `recalled` 仍为 true、`deleted` 为 false，没有事件，审计有记录；错误码 —。
- AC-RCL-04-2【后端】【C4】：输入 已被后台删除的消息；操作 发送者在时限内撤回；预期 错误，状态不变；错误码 4044。
- AC-RCL-04-3【后端】【C4】：输入 群消息被撤回（`IM_V13_ADMIN_FEATURES=1`）；操作 普通正文授权读取；隔离授权读取；预期 前者没有 `body`，`recalled=true`，**`deleted=false`**；后者返回原文；错误码 —。（v0.3 改：RC2-21）
- AC-RCL-04-4【后端】【C4】[IM 钟]：输入 被撤回的消息；操作 推进 31 天运行 `quarantine_purge`；预期 隔离记录被物理删除，隔离授权读取 404；错误码 4044。
- AC-RCL-04-5【后端】【C4】：输入 `IM_V13_ADMIN_FEATURES=1`，单聊被撤回的消息，`IM_C2C_BODY_ACCESS=off`；操作 申请隔离授权；预期 403，`reason=c2cBodyDisabled`；错误码 4030。
- AC-RCL-04-6【后端】【C4】：输入 图片消息被撤回；操作 参与者申请地址；预期 404；错误码 4048。（从 AC-FILE-04-5 拆出）
- AC-RCL-04-7【后端】【C4】：输入 `IM_V13_ADMIN_FEATURES=1`，撤回的消息；操作 `POST /admin/convs/messages/query`；预期 有 `recalled`、`recalledAt`、`recalledBy`，没有 `body`，文件消息没有文件名；错误码 —。
- AC-RCL-04-8【后端】【C4】[IM 钟]：输入 被撤回的文件；操作 查 `files.name`；预期 是空，文件名在隔离区（密文）；错误码 —。（从 AC-FILE-06-10 拆出）
- AC-RCL-04-9【后端】【C4】：输入 `IM_V13_ADMIN_FEATURES=1`，撤回的图片；操作 `POST /admin/files/{fileId}/fetch` 没有授权；隔离授权（`msg.quarantine`）；普通调用方；预期 依次 403 / 4039、成功、403 / 4030；错误码 4039、4030。（RC2-24）

### F-QUO-01 引用发送（P0）

**规则**
1. `send` 的 `body.quote = {msgId}`。只有 `text` 消息可以带引用（图片、文件不能，第一批）。
2. 被引用的消息必须在同一会话、发送者能看到（seq ≥ 他的 `minSeq`）、没有撤回或删除、不是 `tip`。否则 4040，`reason: quoteUnavailable`（不区分原因）。
3. 服务端保存 `quote_msg_id`，**不把被引用的内容复制进这条消息**，下发时才组装：`quote = {msgId, seq, from, type, state, text?}`（规则见 F-QUO-02）。
4. 不允许引用链展开：引用一条"带引用的消息"时只显示那条消息本身的文字。
5. 客户端可以引用图片和文件消息（摘要规则见 F-QUO-02 规则 2）。

**异常**：原消息在发送时刚被撤回：按当时状态，要么 4040，要么发送后 `state=recalled`，不会泄露原文。`quote.msgId` 格式错误：400 / 4000。

**验收标准**
- AC-QUO-01-1【C4】：输入 u1 引用 u2 的消息回复；操作 发送；预期 u2 收到的 `msg` 带 `quote`，`state=normal`，`text` 是原文摘要；错误码 —。
- AC-QUO-01-2【后端】【C4】：输入 其他会话的 msgId；不存在的；已撤回的；已删除的；`tip`；发送者入群前的；操作 分别引用；预期 都是 4040，`reason=quoteUnavailable`，响应相同，消息未保存；错误码 4040。
- AC-QUO-01-3【后端】【C4】：输入 `type=image` 带 `quote`；操作 发送；预期 错误；错误码 4000。
- AC-QUO-01-4【后端】【C4】：输入 同一 `clientMsgId` 重发（原消息已保存，被引用的消息已被撤回）；操作 重发；预期 返回第一次的 ack（去重优先）；错误码 —。
- AC-QUO-01-5【前端】【C4】：输入 长按消息选"引用"；操作 输入回复后发送；预期 输入框上方显示引用条，可以取消；发送后气泡顶部显示引用内容；错误码 —。
- AC-QUO-01-6【后端】【C4】：输入 m2 引用 m1（特征串 `ZXQ-QUO-2`）；操作 查 m2 的 `messages.body` 原始值；预期 只有 `quote_msg_id`，搜不到特征串；错误码 —。
- AC-QUO-01-7【后端】【C4】：输入 m3 引用 m2，m2 引用 m1；引用图片；引用文件；`quote.msgId` 格式错误；操作 分别发送后查下发；预期 m3 的 `quote.text` 是 m2 自身文字；"[图片]"；"[文件] {文件名}"；4000；错误码 4000。

### F-QUO-02 引用的展示（P0）

**规则**
1. `state`：`normal`（带 `text`）、`recalled`、`deleted`、`expired`（文件过期，仍显示"[文件]"）。后三种**没有原文**。
2. `text`：文本取前 100 个码点；图片"[图片]"；文件"[文件] {文件名}"。
3. 显示：`recalled` →"引用内容已被撤回"；`deleted` →"引用内容已被删除"；`expired` →"[图片]"或"[文件]"加"已过期"；不认识的 `state` →"引用内容不可用"。
4. **被引用的消息撤回或删除时**：客户端收到 `msgDeleted`，把本地所有引用这条消息的 `quote` 清除原文；`sync` 组装时也返回新状态，SDK 按 F-RCL-03 规则 2a 覆盖。撤回时被引用消息和引用它的消息所在会话的 `mod_ver` 同时递增（同一会话），所以 m1 不在已加载范围、m2 在范围内时，重拉 m2 也能更新。
5. 点击引用条跳到原消息：**距离已加载范围 200 条以内的向上翻页加载再跳转；更远的提示"消息太早，请翻页查看"**（现有 Store 假设已加载范围连续，第一批不做跳转到任意位置，R-CF-17）。拉不到提示"消息不存在"。
6. **已知限制**：`historyVisible=false` 的新成员，会通过别人的引用看到入群前消息的摘要（最多 100 个码点）。视为发送者的主动转述，接受。
7. 敏感词只查新消息的文本，不重新查引用摘要。

**异常**：`sync` 组装引用时原消息已不存在（物理删除）：`state=deleted`。不认识的 `state`：显示"引用内容不可用"。

**验收标准**
- AC-QUO-02-1【后端】【C4】：输入 m2 引用 m1（含特征串 `ZXQ-QUO-1`）；操作 撤回 m1 后 `sync` m2；后台删除另一条被引用的 m3 后 `sync`；预期 m2 的 `quote.state=recalled`，响应搜不到特征串；m3 的引用 `state=deleted`；错误码 —。
- AC-QUO-02-2【前端】【C4】（sdk-kotlin、sdk-swift、sdk-ts）：输入 本地有 m1 和引用它的 m2；操作 收到 m1 的 `msgDeleted`（带 `recalled`）后查本地（内存和数据库）；预期 m2 的引用里搜不到特征串，显示"引用内容已被撤回"；错误码 —。
- AC-QUO-02-3【前端】【C4】：输入 引用的消息距已加载范围 100 条；500 条；操作 点击引用条；预期 前者翻页后滚动到原消息并高亮；后者提示"消息太早，请翻页查看"，已加载范围保持连续；错误码 —。（v0.2 改：R-CF-17）
- AC-QUO-02-4【后端】【C4】：输入 `historyVisible=false`，D 入群后 u1 引用入群前的消息；操作 D 收到；预期 带 `quote.text`（规则 6）；D 直接 `sync` 那条原消息仍拉不到；错误码 —。
- AC-QUO-02-5【前端】【C4】：输入 引用摘要含 HTML；操作 显示；预期 按纯文本显示，不执行；错误码 —。
- AC-QUO-02-6【前端】【C4】（三套 SDK）：输入 u2 本地有 m2（引用 m1），m1 不在已加载范围；操作 u2 离线期间 m1 被撤回，重连；预期 内存和本地库搜不到 m2 引用里的原文特征串；错误码 —。（R-CB-04、R-CF-05）
- AC-QUO-02-7【后端】【C4】：输入 m1 是 150 个码点的文本；图片；文件过期后；操作 查引用 m1 的消息；预期 `text` 是前 100 个码点；"[图片]"；`state=expired` 没有文件名；错误码 —。

### F-MEN-01 @ 成员（P0）

**规则**
1. 只在群聊。`send` 的 `body.mentions = {userIds: [...], all?: bool}`。单聊带 `mentions` 400 / 4000。
2. 只有 `text` 可以带。`userIds` 最多 20 个（超出 400 / 4131），去重。
3. 服务端过滤：不是当前成员的 userId 静默去掉，不报错，保存过滤后的结果（选人和发送之间可能有人退群）。
4. **文字和 `mentions` 互相独立**：客户端在文本里插入 `@{公开昵称} `，**没有昵称用 `userId`，绝不用备注**（R-CF-03：备注只有自己可见，PRD-social 5.5 节）。选择列表里可以显示备注。真正的提醒只看 `mentions`，服务端不解析文本，纯文本里写 `@alice` 没有效果。第一批不做文字高亮的精确定位，消息对被 @ 的人整体用样式提示。
5. 输入模型（R-CF-16）：客户端内部用"文本 + `[{start, len, userId}]`"。行首或空白后输入 `@` 或 `＠` 才打开成员选择（`a@b.com` 不触发）；**组字中不改模型**；光标不能落进 mention 内部，退格一次删除整段，对应 `mentions` 一起移除；粘贴的 `@xxx` 只是普通文字；同名成员在列表里用头像和 `userId` 区分。**草稿改为结构化**，恢复后 `mentions` 和引用条都在。
6. 成员选择：`groupMembers` 增加可选 `keyword`（`userId` 或昵称前缀，每人每分钟 30 次，未校准）；有加载中、无结果状态。

**异常**：选人时成员列表加载失败：可以继续发普通文本。全部 `userIds` 都被过滤：按没有 `mentions` 保存，不报错。

**验收标准**
- AC-MEN-01-1【C4】：输入 群 g1，u1 @ u2；操作 发送；预期 u2 收到的消息带 `mentions.userIds=[u2]`，会话出现"[有人@我]"；u3 不提示；错误码 —。
- AC-MEN-01-2【后端】【C4】：输入 `mentions` 含非成员 u9 和 u2；操作 发送后 `sync`；预期 成功，只保留 u2，不报错；错误码 —。
- AC-MEN-01-3【后端】【C4】：输入 21 个 userId；单聊带 `mentions`；`type=image` 带 `mentions`；操作 分别发送；预期 依次 4131、4000、4000；错误码 4131、4000。
- AC-MEN-01-4【后端】【C4】：输入 文本含 `@u2`，没有 `mentions`；操作 发送；预期 u2 不被标记提及；错误码 —。
- AC-MEN-01-5【前端】【C4】：输入 群聊输入框；操作 输入 `@`，选 u2，再删除这段文字后发送；预期 发出的 `mentions` 不含 u2；错误码 —。
- AC-MEN-01-6【前端】【C4】：输入 单聊输入框；操作 输入 `@`；预期 不打开成员选择；错误码 —。
- AC-MEN-01-7【前端】【C4】：输入 给成员 u2 设备注"老板"，昵称"小王"；操作 @ u2 发送，抓上行帧和服务端存储；预期 正文含"@小王"，搜不到"老板"；选择列表显示备注；错误码 —。（R-CF-03）
- AC-MEN-01-8【前端】【C4】：输入 `a@b.com`；行首 `＠`；粘贴"@u2"；操作 观察；预期 第一个不打开选择；第二个打开；第三个不产生 `mentions`；错误码 —。
- AC-MEN-01-9（手工）【C4】：输入 三端系统中文输入法；操作 组字过程中输入 @、回车上屏（写法同 AC-UI-10-2）；预期 组字时不打开选择，不错位；错误码 —。
- AC-MEN-01-10【前端】【C4】：输入 @ 了 u2 并引用了一条消息的草稿；操作 离开会话再回来；预期 文字、`mentions`、引用条都恢复；发送后 u2 被提及；错误码 —。
- AC-MEN-01-11【后端】【C4】：输入 2000 人群，`groupMembers {keyword:"zh"}`；同一 userId 重复 3 次的 `mentions`；操作 请求；发送；预期 只返回前缀匹配的成员；`mentions` 去重；错误码 —。
- AC-MEN-01-12【前端】【C4】：输入 退格光标在 mention 后；操作 按一次退格；预期 整段删除；错误码 —。

### F-MEN-02 @ 所有人（P0）

**规则**
1. 只有群主和管理员可以。普通成员 `mentions.all=true`：4030，`reason: mentionAllForbidden`。托管群没有群主时，任何成员都不能。
2. 次数：每群每小时 3 次、每天 10 次（Asia/Shanghai 自然日）。**计数在保存消息的事务内，被 4510 等拒绝的不扣，撤回不退**（第 4.3 节，R-CB-07）。超出 4293，不自动重试，`details.retryAfterSeconds`。**不用 4292**，避免和"今天发起新聊天的次数已用完"的文案混淆。计数不可用时 503 / 5030，不放行。
3. 普通成员的客户端不显示"所有人"选项；服务端照样校验。
4. 全群禁言（F-ADM-42）时群主、管理员仍可发言和 @ 所有人。

**异常**：操作者在发送时刚被取消管理员：按查库结果 4030。次数计数器不可用：按拒绝处理（5030），不放行。

**验收标准**
- AC-MEN-02-1【后端】【C4】：输入 成员 m、管理员 a、群主 o；操作 分别 `mentions.all=true`；预期 m 错误，`reason=mentionAllForbidden`，消息未保存；a、o 成功；错误码 4030。
- AC-MEN-02-2【后端】【C4】[IM 钟]：输入 时钟从 Asia/Shanghai 当天 00:00 起，同一群 1 小时内第 4 次；操作 发送；1 小时后再发；继续到当天第 11 次；预期 第 4 次错误；1 小时后成功；第 11 次错误；错误码 4293。（v0.2 改：R-CQ-41）
- AC-MEN-02-3【后端】【C4】：输入 `IM_MENTION_ALL_HOURLY`、`IM_MENTION_ALL_DAILY` 都调高，2000 人、在线 200 人（未校准），发 1000 次，生产配置，3 次取最差；操作 发送；预期 ack P99 ≤ 200 ms（未校准，开发机参考，目标机重测）；错误码 —。（v0.3 改：RC2-12）
- AC-MEN-02-4【前端】【C4】：输入 普通成员和管理员的群聊；操作 打开 @ 选择；预期 只有管理员看到"所有人"；错误码 —。
- AC-MEN-02-5【前端】【C4】：输入 服务端返回 4293；操作 发送；预期 消息标记失败，提示"@所有人次数已用完"，不自动重发；错误码 4293。
- AC-MEN-02-6【后端】【C4】：输入 @ 所有人一次；操作 查 `conv_mentions`；预期 没有逐成员行，`group_mention_all` 一行；错误码 —。
- AC-MEN-02-7【后端】【C4】：输入 `IM_V13_ADMIN_FEATURES=1`，托管群无群主，成员 @ 所有人；全群禁言中群主 @ 所有人；操作 发送；预期 403；成功；错误码 4030。
- AC-MEN-02-8【后端】【C4】：输入 同一群发 3 次 @ 所有人后撤回 3 次；计数器故障注入；操作 发送；预期 第 1 种之后第 4 次仍 4293；第 2 种 503；错误码 4293、5030。（C6 部分拆到 AC-MOD-02-16）
- AC-MEN-02-9【后端】【C4】：输入 同一群并发 10 条 @ 所有人；操作 发送；预期 恰好 3 条成功，其余 4293；错误码 4293。

### F-MEN-03 提醒与免打扰（P0）

**规则**
1. `convs` 增加 `mentionUnread`（未读中提及我的数量，含 @ 所有人）、`firstMentionSeq`。按已读位置计算，已读前进后减少。被撤回或删除的提及不计入。**本地推算不了时刷新**：收到 `msgDeleted`、`readUpdated` 且这个会话本地 `mentionUnread > 0` 时，SDK 节流（每 5 秒最多一次，未校准）重新拉 `convs`（R-CF-18）。
2. 会话列表摘要前缀优先级（R-CF-20）："[有人@我]" > "[草稿]" > "[N 条]"（免打扰）。
3. 打开会话时如果 `firstMentionSeq` 不在视口，显示"有人@我"的跳转按钮。
4. **优先级**（第 20.2 节）：

   | 会话 | 普通消息 | @ 所有人 | @ 我 |
   |---|---|---|---|
   | 普通 | 推送 | 推送 | 推送 |
   | 免打扰 | 不推送 | **不推送**，会话内仍显示提及 | **推送** |

5. 被我拉黑的人在群里 @ 我：群里拉黑不生效（F-SOC-07），消息和"[有人@我]"照常，但**不打断免打扰**。
6. 自己 @ 自己不算提及。

**异常**：提及表写入失败：和消息同一事务，消息也不保存（5000）。已退群的成员不再计算。清除提及和已读并发：以已读位置为准。

**验收标准**
- AC-MEN-03-1【后端】【C4】：输入 u2 未读里 seq 5、7 @ 他，seq 9 @ 所有人；操作 查 `convs`；已读到 5 后再查；预期 `mentionUnread=3`、`firstMentionSeq=5`；之后 `mentionUnread=2`、`firstMentionSeq=7`；错误码 —。（v0.2 改：R-CQ-24）
- AC-MEN-03-2【后端】【C5】：输入 u2 免打扰群 g1、离线；操作 分别发普通消息、@ 所有人、@ u2，再给哨兵发一条；预期 假通道只收到 @ u2 那条；错误码 —。
- AC-MEN-03-3【后端】【C5】：输入 u2 拉黑了 u1，g1 免打扰；操作 u1 在 g1 @ u2，再给哨兵发一条；预期 `mentionUnread` 增加，u2 的令牌无调用；错误码 —。
- AC-MEN-03-4【后端】【C4】：输入 @ u2 的消息被撤回；操作 查 `convs`；预期 `mentionUnread` 减 1；错误码 —。
- AC-MEN-03-5【前端】【C4】：输入 有提及的会话同时有草稿；操作 看列表；预期 显示"[有人@我]"；错误码 —。
- AC-MEN-03-6【前端】【C4】：输入 打开未读 100 条的群，提及在第 10 条；操作 观察；预期 出现跳转按钮，点击滚动到该消息并高亮，之后按钮消失；错误码 —。
- AC-MEN-03-7【前端】【C4】（三套 SDK）：输入 本地有 `mentionUnread=1`；操作 另一端已读过提及位置，收到 `readUpdated`；对方撤回提及消息；预期 5 秒内重新拉取后"[有人@我]"消失；错误码 —。（R-CF-18）
- AC-MEN-03-8【后端】【C4】：输入 自己 @ 自己；已退群的成员被 @；故障注入提及表写失败；操作 发送；预期 第一种不计入；第二种被过滤；第三种 500，消息未保存；错误码 5000。

---

## 9. C5 会话置顶、免打扰、删除

### F-CVS-01 置顶（P0）

**规则**
1. WS 命令 `convSet {convId, pinned?, notifyOff?}`，至少一项。幂等。字段用 `notifyOff`，不用 `muted`：同一结构已有 `mutedUntil`、`allMuted`，事件 `muted` 和 4031 都表示禁言（R-CB-26）。依赖 `IM_CONV_SETTINGS_ENABLED`。
2. 每人最多置顶 20 个（超出 4220，文案"最多置顶 20 个会话"）。
3. 排序：置顶的在最前（`pinnedAt` 倒序），其余按 `updatedAt` 倒序，时间相同按 `convId`（F-CONV-01 规则 3 扩展）。
4. `convs` 的 500 上限：**置顶的一定在结果里**，其余取最近更新的；`truncated` 含义不变。
5. 只能操作自己参与的会话：单聊非参与方 4032；非群成员 4033；群不存在 4043；没有消息的单聊 4040。
6. 退群、被移出、解散：置顶和免打扰记录一并删除；重新加入不继承。

**异常**：`convSet` 没有任何字段：400 / 4000。解散的群：4043。写入失败：5000，客户端回滚乐观显示。

**验收标准**
- AC-CVS-01-1【后端】【C5】：输入 3 个会话，置顶最旧的 c1；再置顶 c2；操作 `convs`；预期 顺序 c2、c1、其余按时间；错误码 —。
- AC-CVS-01-2【后端】【C5】：输入 已置顶 20 个；操作 第 21 个；已置顶的再次置顶；预期 前者错误；后者成功且 `pinnedAt` 不变；错误码 4220。
- AC-CVS-01-3【后端】【C5】：输入 501 个会话，最旧的一个置顶；操作 `convs`；预期 返回 500 个，含置顶的那个，`truncated=true`；错误码 —。
- AC-CVS-01-4【后端】【C5】：输入 非参与方；非成员；没有消息的单聊；已解散的群；没有任何字段；操作 分别 `convSet`；预期 依次 4032、4033、4040、4043、4000；错误码 4032、4033、4040、4043、4000。
- AC-CVS-01-5【后端】【C5】：输入 g1 已置顶和免打扰；操作 把 u1 移出后再加入；预期 都没有了；错误码 —。
- AC-CVS-01-6【前端】【C5】：输入 会话列表；操作 Android、iOS 和窄屏 H5 长按，宽屏 H5 右键，H5 键盘菜单键；预期 菜单有"置顶 / 取消置顶""免打扰 / 取消免打扰""删除"；置顶项有明显的背景区分；错误码 —。（v0.2 改：R-CF AC 表）

### F-CVS-02 免打扰（P0）

**规则**
1. 按会话，单聊和群聊都可以。不做全局免打扰和时段。
2. 不推送，例外见 F-MEN-03。
3. 未读照常计算，`convs` 的 `unread` 不变。
4. **总未读 = 普通会话的未读之和 + 免打扰会话的 `mentionUnread` 之和**（PRD-ui F-UI-23 变更：标签角标、`document.title`、iOS 角标同一口径）。@ 我计入，否则出现"有通知、无角标"（R-CF-20，技术上定掉）。
5. 列表显示：免打扰会话的未读显示为小红点（无数字），摘要前显示"[N 条]"，右侧有免打扰图标。
6. 在线时收到免打扰会话的消息：列表更新，不响铃。

**异常**：设置读取失败时按没有免打扰处理（宁可多推一条，不漏）。

**验收标准**
- AC-CVS-02-1【后端】【C5】：输入 u2 免打扰 c2c 会话，离线；操作 u1 发消息，再给哨兵发一条；预期 u2 令牌无调用；`unread` 增加；错误码 —。
- AC-CVS-02-2【前端】【C5】：输入 普通会话未读 2、3，免打扰会话未读 5；操作 看标签角标和 `document.title`（H5）；预期 显示 5；错误码 —。
- AC-CVS-02-3【前端】【C5】：输入 免打扰会话未读 5；操作 看列表；预期 小红点、"[5 条]"、免打扰图标；错误码 —。
- AC-CVS-02-4【前端】【C5】：输入 总未读全在免打扰会话；操作 看 H5 标题；预期 `document.title` 为"天听"；错误码 —。
- AC-CVS-02-5【前端】【C5】：输入 免打扰会话未读 5，其中 1 条 @ 我；普通会话未读 2；操作 看角标；预期 3；错误码 —。
- AC-CVS-02-6【前端】【C5】（Android、iOS）：输入 App 前台，收到免打扰会话的消息；操作 观察；预期 列表更新，不响铃不弹通知；错误码 —。
- AC-CVS-02-7【后端】【C5】：输入 `IM_CONV_SETTINGS_ENABLED=0`；操作 `convSet`；预期 403，`reason=featureDisabled`；错误码 4030。
- AC-CVS-02-8【后端】【C5】：输入 设置读取故障注入；操作 给设了免打扰的离线用户发消息；预期 按没有免打扰推送（宁可多推），消息正常；错误码 —。

### F-CVS-03 删除会话（P0）

**规则**
1. WS 命令 `convDelete {convId, upToSeq}`，`upToSeq` 是客户端看到的最大 seq，服务端取 `min(upToSeq, maxSeq)`。只影响自己。
2. 效果：从自己的会话列表消失；`conv_settings` 写 `deleted_at` 和 `cleared_seq = upToSeq`；已读推进到 `upToSeq`；**我这边这段历史也不再可见**（`minSeq = max(原值, clearedSeq + 1)`，`sync` 和文件地址都按这个）。对方和其他成员不受影响；后台查询不受影响。**隐藏条件是 `deleted_at > 0 AND max_seq <= cleared_seq`**，所以只设了免打扰的空群（`cleared_seq` 默认 0、`max_seq=0`）不会被隐藏（R-CB-13）。
3. 有新消息（seq > `clearedSeq`）时重新出现，只显示新消息。清除置顶，**保留免打扰**（否则删掉一个免打扰的群，下一条消息又开始响）。
4. 群：删除会话不是退群。还在成员列表，"我的群聊"里还能看到，从那里进入时显示空的聊天页；只有 `tip` 也算新消息，会重新出现。
5. 并发：`upToSeq` 之后到达的消息不会被清除，会话马上重新出现。
6. **`convs` 增加 `deleted: [{convId, clearedSeq}]`，单聊和群都包含**（R-CB-02、R-CF-19），最多最近 2000 条（未校准），超出带 `deletedTruncated=true`。两个作用：① 三套 SDK 把 `convs` 里消失的群当作"你已不在该群"（PRD-core F-GRP-03 规则 4），必须先查这个列表，不生成 `removed` 快照；`truncated` 时不推断的规则不变；② **SDK 处理 `convs` 时，对任何会话物理清除 `seq < minSeq` 的消息、引用和缓存文件；列表里的会话同样清除**（现有 SDK 只用 `minSeq` 算补拉起点，不清本地）。老 SDK 不认识 `deleted`，群会显示"你已不在该群"，接受（没有已发布版本的证据；"群留在 `convs` 带 `hidden`"要占 500 上限，不采用）。
6a. 退群、被移出、解散：这一行连同清除记录一起删除。**重新入群按新成员算**：`minSeq` 只看 `joinSeq` 和 `historyVisible`，`historyVisible=true` 时能看到全部历史，包括之前删除过的（R-CB-13）。
7. 客户端确认框"删除后聊天记录不再显示，确定删除？"；清除本地消息、草稿、缓存文件。
8. 不存在的或已删除的再删：幂等成功。

**异常**：`upToSeq` 小于已有的 `clearedSeq`：不回退，幂等成功。写入失败：5000，客户端恢复显示。本地清除失败：下次启动再清，服务端状态不受影响。

**验收标准**
- AC-CVS-03-1【后端】【C5】：输入 u1、u2 单聊 10 条；操作 u1 删除（`upToSeq=10`）后 `convs`、`sync`；u2 查询；预期 u1 列表没有；`sync` 空；u2 不受影响；错误码 —。
- AC-CVS-03-2【C5】：输入 同上；操作 u2 再发 1 条；预期 u1 列表重新出现，未读 1，只能看到这 1 条；`minSeq=11`；错误码 —。
- AC-CVS-03-3【后端】【C5】：输入 `upToSeq=999`，`maxSeq=10`；`upToSeq=8`，`maxSeq=10`；操作 删除；预期 前者清到 10；后者清到 8，未读 2，会话不消失；错误码 —。
- AC-CVS-03-4【后端】【C5】：输入 g1 的会话被 u1 删除；操作 查成员、`myGroups`、`send`；预期 仍是成员，能发消息；`convs` 的 `deleted` 含 g1；错误码 —。
- AC-CVS-03-5【前端】【C5】（三套 SDK）：输入 u1 在 D1 删除 g1 的会话，D2 这时离线；操作 D2 重连后拉 `convs`；预期 g1 从列表消失，**不出现**"你已不在该群"，从通讯录进入可以正常发言；错误码 —。
- AC-CVS-03-6【后端】【C5】：输入 免打扰且置顶的 g1 被删除，然后有新消息；操作 查 `convs`；预期 不再置顶，仍免打扰；错误码 —。
- AC-CVS-03-7【后端】【C5】：输入 被清除范围内的图片；操作 u1 申请地址；预期 404；u2 仍可以；错误码 4048。
- AC-CVS-03-8【前端】【C5】：输入 有草稿、失败消息和本地缓存的会话；操作 确认删除；预期 都被清除；取消时不变；错误码 —。
- AC-CVS-03-9【后端】【C5】：输入 单聊非参与方；非群成员；已解散的群；操作 删除；预期 依次 4032、4033、4043；错误码 4032、4033、4043。（v0.2 改：R-CQ-33）
- AC-CVS-03-10【前端】【C5】（sdk-kotlin、sdk-swift）：输入 u1 的 D2 离线，本地库有和 u2 的单聊，含特征串；操作 D1 删除该单聊，u2 再发一条，D2 上线；预期 D2 本地库和缓存目录搜不到旧特征串，只显示新消息；错误码 —。（R-CB-02）
- AC-CVS-03-11【后端】【C5】：输入 没有消息的托管群，只设了免打扰；删除空群（`upToSeq=0`）；操作 `convs`；预期 第一种在列表里；第二种不在，`deleted` 含它；之后有消息重新出现；错误码 —。（R-CB-13）
- AC-CVS-03-12【后端】【C5】：输入 u1 删除 g1 会话（`historyVisible=true`）；操作 把 u1 移出再加入，`sync`；预期 看到全部历史，`deleted` 不再含 g1；错误码 —。
- AC-CVS-03-13【后端】【C5】：输入 g1 被删除后只产生一条 `tip`；`upToSeq` 小于已有 `clearedSeq`；重复删除；操作 查询；预期 tip 让会话重新出现；`clearedSeq` 不回退；幂等成功；错误码 —。
- AC-CVS-03-14【后端】【C5】：输入 `upToSeq=10` 的删除和 seq 11 的发送并发；操作 查 `convs`；预期 会话最终可见，seq 11 能拉到；错误码 —。
- AC-CVS-03-15【后端】【C5】：输入 u1 删除会话；操作 其他成员申请这个会话里的文件地址；预期 照常，对象还在；错误码 —。（从 AC-FILE-06-11 拆出）

### F-CVS-04 多端同步（P0）

**规则**
1. `convSet`、`convDelete` 的 ack 返回新状态；**同一用户的其他在线连接**收到 `convSettingsChanged {convId, pinned, pinnedAt?, notifyOff}` 或 `convDeleted {convId, clearedSeq}`。
2. 离线设备：重连时 `convs` 带全部设置，以服务端为准。不做"仅本地置顶"（PRD-ui G-UI-07）。
3. 推送判断读同一份设置（改完立即生效，缓存走 `cache.Invalidator`，TTL 60 秒兜底，ARCH 2.2 约束 2）。
4. 限流：每人每秒 5 次（未校准，4290）。

**异常**：事件丢失：下次 `convs` 补齐。两端同时改同一会话：后到的覆盖，`convs` 为准。

**验收标准**
- AC-CVS-04-1【C5】：输入 u1 在 D1、D2 在线；操作 D1 置顶、免打扰；预期 D2 在 2 秒内更新；错误码 —。
- AC-CVS-04-2【C5】：输入 D2 离线时 D1 置顶并删除另一个会话；操作 D2 重连；预期 和 D1 一致；错误码 —。
- AC-CVS-04-3【后端】【C5】（双节点）：输入 u1 在节点 1 把 c2c 会话设为免打扰后断开连接；操作 1 秒后 u2 在节点 2 给 u1 发消息；预期 节点 2 的假通道无调用（节点 2 读到新设置）；错误码 —。
- AC-CVS-04-4【后端】【C5】：输入 1 秒内 6 次 `convSet`；操作 发送；预期 第 6 次错误；错误码 4290。
- AC-CVS-04-5【C5】：输入 u1 在 D1、D2 在线；操作 D1 删除会话；预期 D2 2 秒内收到 `convDeleted`，会话消失，本地消息被清除；错误码 —。

---

## 10. C6 敏感词和举报

### F-MOD-01 词库（P0，CH-19）

**规则**
1. 词库存数据库，后台维护：单个添加、批量导入（每次最多 5000 条）、停用、删除。权限点 `moderation.words`（`operator`、`moderator`），写审计（只记数量和操作，审计不记词内容）。
2. 变更经 `cache.Invalidator`（新主题 `blockedWords`）在 10 秒内生效；TTL 60 秒兜底。
3. 匹配前做标准化：NFKC + 去变音符 + 英文转小写，去掉 Unicode Default_Ignorable（零宽、变体选择符、Bidi 等），**只用于匹配，存储和下发的仍是原文**（沿用 PRD-live F-ROOM-11 规则 2 的思路）。另外生成一份去掉空白和标点的文本再匹配一次，防"违 规 词"（可能误判，用语料测一次，R-CS-17）。不做拼音、形近、谐音。
4. 用多模式匹配（如 AC 自动机），词库 10 万条时单次匹配耗时目标见 AC-CHAT-90-4。
5. **仓库和代码里不放真实词库**，测试只用特征词。词库来源、版权、更新责任人由 Frank 指定（CH-19）。
6. 词库为空时不拦截，配置检查显示"词库为空"告警；`open` 模式下拒绝启动（F-ACC-01 规则 2）。

**异常**：导入中有不合法行（空、超过 64 个码点）：跳过并计入结果。重新加载失败：**沿用内存里的旧词库并告警**。`IM_MOD_ENABLED=1` 时启动加载失败：**启动失败**（词库和消息同库，库读不了本来也起不来；不做"放行但没人知道"，R-CS-30）。

**验收标准**
- AC-MOD-01-1【后端】【C6】：输入 后台添加"违规词"；操作 10 秒后发送含该词的消息；预期 被拦截，消息不落库；错误码 4510。（吸收废弃的 AC-ADM-60-1，R-CQ-15）
- AC-MOD-01-2【后端】【C6】：输入 停用该词；操作 10 秒后发送；预期 成功；错误码 —。
- AC-MOD-01-3【后端】【C6】（双节点）：输入 节点 1 添加；操作 10 秒后从节点 2 发送；预期 被拦截；错误码 4510。
- AC-MOD-01-4【后端】【C6】：输入 一次导入 5001 条；含空行和重复的 100 条；操作 分别导入；预期 前者 400；后者成功，去重后入库；错误码 4131。
- AC-MOD-01-5【后端】【C6】：输入 `support` 账号；操作 添加词；预期 403；错误码 4030。
- AC-MOD-01-6【后端】【C6】：输入 添加特征词 `ZXQWORD`；操作 查 console 审计和 `admin_audit`；预期 有操作记录，搜不到词内容；错误码 —。
- AC-MOD-01-7【前端】【C6】（管理后台）：输入 词库页；操作 搜索、批量导入、停用；预期 内容按纯文本显示，导入有结果汇总（新增、重复、无效）；错误码 —。
- AC-MOD-01-8【后端】【C6】：输入 故障注入词库重新加载失败；`IM_MOD_ENABLED=1` 时启动加载失败；词库为空；操作 观察；预期 旧词库继续生效，告警指标增加；启动失败；配置检查显示告警；错误码 —。
- AC-MOD-01-9【后端】【C6】：输入 导入 1 条 65 个码点的词；操作 导入；预期 跳过并计入结果中的无效；错误码 —。
- AC-MOD-01-10【后端】【C6】：输入 `IM_ENV=production`、`IM_MOD_ENABLED=1`、`IM_V13_ADMIN_FEATURES=0`；操作 启动；预期 非 0 退出；错误码 —。（RC2-15）

### F-MOD-02 发送检查（P0）

**规则**
1. **只拦截，不替换、不做"发送者可见但别人看不到"。** 命中：ack 错误 4510，消息不保存、不投递。替换模式（F-ADM-60 规则）不在第一批，AC-ADM-60-2 作废。
2. 检查范围（**第一批**）：`text`；`image`、`file` 的文件名；注册和修改的昵称；**注册的用户名**；客户端写入的群名、群公告（`groupCreate`、`groupUpdate`）；**好友申请附言**（`friendApply`，可以发给陌生人；在 F-SOC-02 的存在性判断之前检查，不泄露存在性，R-CS-17）。C6 上线前已注册的违规用户名不能改名，由后台封禁处理。
3. **不查**：图片内容；文件内容；引用摘要（已经查过）；服务端生成的 `tip`；管理接口写入的内容；后台系统通知。
4. **`custom` 在生产环境禁止客户端发送**，否则可以绕过检查：`IM_CLIENT_CUSTOM_MSG=0` 时 `send` 的 `type=custom` 返回 4000，`reason: customDisabled`。只影响客户端 `send`；`protocol/vectors` 里的 custom 帧是格式用例，不受影响。
4a. **`Validate` 按 `type` 严格互斥 `body` 字段**：`text` 只能带 `text`、`quote`、`mentions`；`image` 只能带 `image`；`file` 只能带 `file`；`custom` 只能带 `data`。多余字段 400 / 4000。Schema 用条件约束，加向量（R-CB-21：现在 `type=text` 可以夹带 `body.data` 原样落库下发，`frames.go` 第 177～185 行）。这条不受开关影响，从 C2 起生效。
4b. `clientMsgId` 对接收方可见，是改过的客户端之间的隐蔽通道，不处理（R-CS-32，见第 23 节）。
5. 判定位置见第 4.3 节第 9 步：在已有的所有权限判定之后，避免让被拒绝的用户探测词库。4510 已扣发送令牌。
6. 响应不告诉用户命中的是哪个词（防止试探词库）。客户端文案"消息包含不当内容，无法发送"。消息标记失败，不自动重发；输入框不保留（和其他发送失败一致，可以从气泡复制）。
7. 命中记录：写 `moderation_hits`（userId、场景、词 ID、时间），不存消息全文，保留 90 天（未校准）。后台按用户查次数，用于发现反复试探。
8. 去重：已保存的消息重发不重新检查（判定顺序第 2 步）。词库后来新增的词不影响已发消息。

**异常**：v0.1 的"20 ms 超时放行"**取消**：Go 里打断不了同步匹配，AC-CHAT-90-4 的耗时目标已经足够（R-CB-27、R-CS-30）。`hits` 写入失败：不影响拦截结果。

**验收标准**
- AC-MOD-02-1【后端】【C6】：输入 拦截词"违规词"；操作 单聊和群聊发送；预期 都错误；其他人 2 秒内收不到；不落库；错误码 4510。
- AC-MOD-02-2【后端】【C6】：输入 拉丁特征词 `badword`：`ＢＡＤＷＯＲＤ`（全角）、`bádwörd`（变音）、`BadWord`（大小写）、`bad\u200bword`（零宽）、`bad word`（空格）、`bad-word`（标点）；操作 发送；预期 都被拦截；错误码 4510。（v0.2 改：R-CQ-42、R-CS-17）
- AC-MOD-02-3【后端】【C6】：输入 带全角字符的普通文本；操作 发送后 u2 接收；预期 u2 收到原文，未被标准化；错误码 —。
- AC-MOD-02-4【后端】【C6】：输入 文件名含拦截词；操作 发图片、发文件；预期 都错误；错误码 4510。
- AC-MOD-02-5【后端】【C6】：输入 昵称含拦截词；用户名含；群名含；群公告含；好友申请附言含；操作 分别注册或修改、注册、建群、改公告、发申请；预期 都错误，内容未写入；申请的响应和对方是否存在无关；错误码 4510。（v0.2 改：R-CS-17）
- AC-MOD-02-6【后端】【C6】：输入 `IM_CLIENT_CUSTOM_MSG=0`；操作 客户端发 `custom`；预期 错误，`reason=customDisabled`，消息未保存；错误码 4000。
- AC-MOD-02-7【后端】【C6】：输入 `IM_V13_ADMIN_FEATURES=1`，拦截词；操作 被拉黑的用户、被禁言的用户分别发送；预期 依次 4036、4031（都不是 4510），不暴露词库命中；错误码 4036、4031。
- AC-MOD-02-8【后端】【C6】：输入 命中一次；操作 查 `moderation_hits` 和日志；预期 有词 ID 和场景，搜不到消息全文；错误码 —。
- AC-MOD-02-9【前端】【C6】：输入 服务端返回 4510；操作 发送；预期 消息在列表标记失败，**输入框清空**，提示"消息包含不当内容，无法发送"，不自动重发，错误信息不含具体的词；错误码 4510。（v0.2 改：R-CQ-35）
- AC-MOD-02-10【后端】【C6】：输入 `IM_MOD_ENABLED=0`；操作 发送含拦截词的文本；预期 成功；错误码 —。
- AC-MOD-02-11【后端】【C6】：输入 `IM_CLIENT_CUSTOM_MSG=1`（测试环境）；操作 客户端发 `custom`；预期 成功，行为和现在一样；错误码 —。
- AC-MOD-02-12【后端】【C6】：输入 拦截词，用户令牌已知；操作 单聊给新对象发送（配额开启）；查发送令牌和 `newPeerReject`；预期 4510；发送令牌减 1；`newPeerReject` 退还；错误码 4510。（R-CQ-34、R-CB-20）
- AC-MOD-02-13【后端】【C6】：输入 同样场景把失败原因换成 4048、4096、4040；操作 发送；预期 `newPeerReject` 都退还；错误码 4048、4096、4040。
- AC-MOD-02-14【后端】【C6】[IM 钟]：输入 消息已保存后词库新增该词；同一 `clientMsgId` 重发；操作 重发；后台查 hits；推进 91 天运行 `moderation_hits_purge`；预期 重发成功（去重优先）；`/admin/moderation/hits` 能按用户查次数；91 天后被清除；错误码 —。（v0.3 改：RC2-24）
- AC-MOD-02-15【后端】【C6】：输入 `IM_ENV=production`、`IM_CLIENT_CUSTOM_MSG=1`；操作 查配置检查；预期 有告警项；错误码 —。
- AC-MOD-02-16【后端】【C6】：输入 @ 所有人的消息含拦截词；操作 群主发送；查次数和发送令牌；预期 4510，次数不变，令牌减 1；错误码 4510。（从 AC-MEN-02-8 拆出）

### F-MOD-03 举报提交（P0）

**规则**
1. WS 命令 `report {targetType: "message"|"user"|"group", convId?, msgIds?, userId?, groupId?, reason, note?}`。`reason`：`spam`、`abuse`、`porn`、`fraud`、`illegal`、`other`。`note` 0～200 个码点。
2. `message`：协议单次 1～20 条，同一会话，举报人必须是参与者且能看到这些消息（seq ≥ 他的 `minSeq`）。否则 4040（`reason: reportTargetInvalid`，响应相同）。**客户端第一批只举报单条**（多选在后续批，R-CF-21）。
2a. **已撤回的消息**（R-CS-16、R-CS-25）：撤回提示行有"举报"入口，隔离原文还在就可以举报。服务端校验举报人 `readSeq ≥ seq`，防止没看过的人只凭 `msgId` 把保留期延长；已知限制：撤回后才打开会话也会满足这个条件，影响只是多留一份证据。
3. `user`：对象必须是好友、或有共同会话或共同群，否则同样 4040，防止探测。`group`：必须是成员或最近退出的（第一批只要求当前成员）。
4. 限额：每人每天 20 次（4292）；同一举报人对同一对象 24 小时内重复提交：幂等成功，不产生新记录。`reason` 不合法、`note` 超长 400 / 4000。
5. **保存证据快照**：在同一事务写入 `report_evidence`（AES-256-GCM，独立密钥 `IM_REPORT_KEY`），不依赖消息后续是否被撤回。文件只记引用，并延长文件保留到证据到期（F-FILE-06 规则 5）。**群消息存元数据和正文；单聊在 `IM_C2C_BODY_ACCESS=off` 时只存元数据**（R-CS-15、CH-22：否则将来开关一开，当时告知口径下收集的内容全部可读；代价是之后撤回的单聊消息没有证据）。
6. 证据保留 180 天，到期物理删除（`report_purge`）。
7. **被举报人不会收到任何通知**，举报人的身份只有后台审核员可见。
8. 客户端：长按消息"举报"（单条）；撤回提示行"举报"；资料卡和群信息页"举报"；提交前提示"所选内容将提交给平台审核"。提交成功后询问是否拉黑（单聊）。

**异常**：证据写入失败：举报失败（5000），不产生记录。消息已物理删除或隔离原文已过期：证据只有元数据，举报仍成功。

**验收标准**
- AC-MOD-03-1【C6】：输入 u2 举报 u1 的消息，理由 `abuse`；操作 提交；预期 成功；后台队列出现；u1 2 秒内收不到任何帧；错误码 —。
- AC-MOD-03-2【后端】【C6】：输入 不是参与者；其他会话的 msgId；入群前的消息；21 条；操作 分别举报；预期 前三个 4040，响应相同；第四个 4131；错误码 4040、4131。
- AC-MOD-03-3【后端】【C6】：输入 同一消息重复举报；操作 提交 3 次；预期 都成功，只有 1 条记录；错误码 —。
- AC-MOD-03-4【后端】【C6】：输入 当天第 21 次；操作 提交；预期 错误；错误码 4292。
- AC-MOD-03-5【后端】【C6】：输入 举报含特征串 `ZXQ-RPT-1` 的**群**消息后，发送者撤回；操作 查 `report_evidence` 原始字节；审核员读取证据；预期 原始字节搜不到明文；审核员读到原文；错误码 —。
- AC-MOD-03-6【后端】【C6】[IM 钟]：输入 证据；操作 推进 181 天运行 `report_purge`；预期 物理删除；错误码 —。
- AC-MOD-03-7【后端】【C6】：输入 举报用户：完全没有关联的 userId；不存在的 userId；操作 举报；预期 响应相同；错误码 4040。
- AC-MOD-03-8【后端】【C6】：输入 `note` 含特征串；操作 扫日志和审计；预期 搜不到；错误码 —。
- AC-MOD-03-9【前端】【C6】：输入 消息长按"举报"；操作 填理由、提交；预期 提交前有提示；提交后轻提示"已提交，感谢你的反馈"；单聊询问是否拉黑；错误码 —。（v0.2 改：R-CF-21）
- AC-MOD-03-10【后端】【C6】：输入 `IM_C2C_BODY_ACCESS=off`，单聊消息含特征串 `ZXQ-RPT-2`；操作 举报后查 `report_evidence`；把开关打开后申请证据；预期 只有元数据，密文解开也没有特征串；打开后仍读不到正文；错误码 —。（R-CS-15）
- AC-MOD-03-11【前端】【C6】：输入 看过后被撤回的消息；操作 点撤回提示行的"举报"；预期 成功；审核员能读到隔离原文（群）；错误码 —。（R-CS-16）
- AC-MOD-03-12【后端】【C6】：输入 u2 未读到这条（`readSeq < seq`）的已撤回消息；隔离原文已过期；操作 举报；预期 前者 4040；后者成功，证据只有元数据；错误码 4040。
- AC-MOD-03-13【后端】【C6】：输入 举报群：非成员；当前成员；`reason=xxx`；`note` 201 个码点；操作 举报；预期 4040（和不存在的群响应相同）；成功；400；400；错误码 4040、4000。
- AC-MOD-03-14【后端】【C6】：输入 故障注入证据写失败；`IM_MOD_ENABLED=0`；操作 举报；预期 500，没有记录；403，`reason=featureDisabled`；错误码 5000、4030。
- AC-MOD-03-15【后端】【C6】：输入 文件消息被举报，原消息正常；操作 其他成员取地址；180 天保留期过后；预期 正常发放（R-CS-25）；过期后普通用户 4048，证据仍可看；错误码 4048。

### F-MOD-04 举报处理（P0，CH-22）

**规则**
1. 后台"举报"队列：按状态（`pending`、`handled`、`rejected`）、理由、时间筛选；相同对象的多条举报聚合显示次数。权限点 `report.read`、`report.handle`（`moderator`）。
2. 查看元数据（举报人、被举报人、时间、理由、消息类型和长度）不需要工单，但**每次打开举报单写审计**；**单聊举报的元数据要填理由，和 F-ADM-30 规则 3 对齐，配额同样每天 50 个**（R-CS-25）。`note` 和文件名**按正文对待**，走规则 3 的授权（R-CS-14、R-CS-25）。证据只包含被举报的消息，不带上下文（不扩大收集范围）。
3. **查看证据正文**：群消息证据走 F-ADM-08 同样的流程（理由 + 当次动态码，工单号自动用举报 ID；配额合并；水印）。im-server 新 scope `report.evidence`，属于正文类 scope（要加进 `adminauth.IsBodyScope`），只能配在 `bodyAccess` 调用方（`console-msgread`）。**审核员同时看到缩略图和原图**（R-CS-12）。文件内容**经 console-server 代理**，固定响应头，图片校验后内联，其他只提供附件，不放宽后台 CSP（F-CON-10 规则 4 不变，R-CS-24）。
4. **单聊证据**：关闭时只存了元数据（F-MOD-03 规则 5），403 / 4030 `reason: c2cBodyDisabled`，页面显示"单聊正文查看未开启"，审核员只能看元数据和该用户被举报的次数。**这让单聊举报基本无法判定内容**，是否放开要法务和 Frank 定（CH-22）。
5. 处理：填结论（`handled` / `rejected`）和理由，可以跳转已有操作（删除消息、禁言、封禁、强制下线、解散群），**不自动执行**；这些操作沿用各自的权限和审计。
6. 第一批不给举报人发处理结果（避免依赖系统通知开关）。不设自动封禁。
7. 队列超过 24 小时未处理的数量进指标，告警默认不开（没有指定处理人之前没有意义）。

**异常**：读证据时密钥不匹配：显示"证据无法解密"并告警，不影响结案。写审计失败：操作不执行（5031）。

**验收标准**
- AC-MOD-04-1【C6】：输入 `IM_V13_ADMIN_FEATURES=1`，审核员打开队列；操作 查看群消息举报，填理由和动态码读证据，删除消息，结案；预期 看到原文和水印；消息被删除；状态 `handled`；审计依次有 `report.view`、`report.evidence.grant`、`msg.delete`、`report.handle`；错误码 —。
- AC-MOD-04-2【后端】【C6】：输入 `support` 账号；`operator` 账号；操作 读队列、结案；预期 都是 403；错误码 4030。
- AC-MOD-04-3【后端】【C6】：输入 `IM_C2C_BODY_ACCESS=off`，单聊消息举报；操作 审核员申请证据授权；预期 403，`reason=c2cBodyDisabled`；元数据照常可看；错误码 4030。
- AC-MOD-04-4【后端】【C6】：输入 调用方 `console` 主凭证；操作 调用 `report.evidence` 接口；预期 403；错误码 4030。（v0.2 改：R-CQ-36，配置校验拆到 AC-MOD-04-8）
- AC-MOD-04-5【后端】【C6】：输入 同一条举报；操作 两个审核员同时结案；预期 第一个成功；第二个 409，状态不变；错误码 4098。
- AC-MOD-04-6【前端】【C6】（管理后台）：输入 同一用户被 3 人举报；操作 看列表；预期 聚合显示"3 次"，点开看到每条；被举报内容按纯文本渲染；错误码 —。
- AC-MOD-04-7【后端】【C6】：输入 结案；操作 观察举报人和被举报人的连接；预期 2 秒内都收不到任何帧；错误码 —。
- AC-MOD-04-8【后端】【C6】：输入 配置 `console` 同时带 `report.evidence` 和 `msg.meta`；操作 启动；预期 非 0 退出，指出正文类 scope 的限制；错误码 —。
- AC-MOD-04-9【后端】【C6】：输入 `super_admin` 账号（没有应急授权）；操作 读证据；预期 403；错误码 4030。
- AC-MOD-04-10【后端】【C6】：输入 单聊举报；操作 不填理由查元数据；当天第 51 个；查 `note`；预期 400；429；没有授权时响应没有 `note` 和文件名；错误码 4000、4292。
- AC-MOD-04-11【后端】【C6】：输入 含特征串文件名的文件消息；操作 客服用 `msg.meta` 查询；预期 两层响应都搜不到文件名（同 AC-ADM-30-7 的做法，R-CS-14）；错误码 —。
- AC-MOD-04-12【前端】【C6】（管理后台）：输入 被举报的图片、html 文件；操作 查看；生产 CSP；预期 图片经 console-server 同源加载；html 只能下载；没有 CSP 违规；错误码 —。（R-CS-24）
- AC-MOD-04-13【后端】【C6】：输入 单聊文件证据，`IM_C2C_BODY_ACCESS=off`；操作 申请文件；预期 403，`reason=c2cBodyDisabled`；错误码 4030。
- AC-MOD-04-14【后端】【C6】：输入 证据密钥不匹配；审计写失败；操作 读证据；结案；预期 显示"证据无法解密"，可以结案；503，未结案；错误码 5031。
- AC-MOD-04-15【前端】【C6】（管理后台）：输入 文件名含 `<script>` 的证据；操作 查看；预期 按纯文本显示；错误码 —。（R-CQ-27）
- AC-MOD-04-16【后端】【C6】：输入 被举报的群文件；操作 `POST /admin/files/{fileId}/fetch` 没有授权；有授权；预期 403 / 4039；成功；错误码 4039。（RC2-24）
- AC-MOD-04-17【后端】【C6】[IM 钟]：输入 181 天前的 `account_events`；操作 运行 `account_events_purge`；预期 被清除；错误码 —。（RC2-24）

---

## 11. 协议变更清单（实现前落实到 protocol/，本文不改 protocol/）

### 11.1 WS 帧

| 变更 | 内容 | 功能点 |
|---|---|---|
| `MsgType` 增加 | `image`、`file` | F-FILE-02、03 |
| `MsgBody` 收紧 | 按 `type` 互斥字段，Schema 用条件约束（`additionalProperties:false` 分支），`Validate` 同步 | F-MOD-02 规则 4a |
| `MsgBody` 增加 | `image {fileId, width, height, size, mime, expired?}`（上行只有 `fileId`）；`file {fileId, name, size, mime, expired?}`（上行只有 `fileId`）；`quote {msgId}`（上行）/ `{msgId, seq, from, type, state, text?}`（下行）；`mentions {userIds, all?}` | F-FILE、F-QUO、F-MEN |
| `Message` 增加 | `recalled`、`recalledAt`、`recalledBy`；撤回的消息同时 `deleted=true`，所以现有"`deleted` 时没有 `body`"的条件不用改 | F-RCL-01 规则 5a |
| `LastMsg` 增加 | `recalled`、`recalledBy`；同样 `deleted=true`，没有 `text` | F-RCL-03 |
| `Conversation` 增加 | `pinned`、`pinnedAt`、`notifyOff`、`mentionUnread`、`firstMentionSeq`、`modVer` | F-CVS、F-MEN-03、F-RCL-03 |
| `GroupMembersReq` 增加 | 可选 `keyword` | F-MEN-01 规则 6 |
| `convs` ack 增加 | `deleted: [{convId, clearedSeq}]`（单聊和群）、`deletedTruncated` | F-CVS-03 |
| 新上行命令 | `recall {convId, msgId}` → `{}`；`convSet {convId, pinned?, notifyOff?}` → `{conv}`；`convDelete {convId, upToSeq}` → `{clearedSeq}`；`deviceNotificationRegister`、`deviceNotificationUnregister` → `{}`；`deviceNotificationSettings {showDetail?}` → `{showDetail}`；`presence {state}` → `{}`（普通带 `id` 的命令）；`report {…}` → `{reportId}` | F-RCL、F-CVS、F-APN、F-MOD |
| 事件变更 | `msgDeleted` 增加可选 `recalled {by, at}`；新事件 `convSettingsChanged {convId, pinned, pinnedAt?, notifyOff}`、`convDeleted {convId, clearedSeq}`、`securityNotice {kind, at, platform}` | F-RCL-03、F-CVS-04、F-ACC-02 |
| `kickReasons` 增加 | `passwordChanged`、`sessionRevoked`、`accountDeleted`（都是关闭码 4012；后者用于注销申请和之后）。`replaced` 仍保留不实现。**`kicked` 帧（`frames.go` 第 334～339 行）增加可选 `recoverBefore`**（RS5-03） | F-ACC-03、04、08 |
| 向后兼容 | 新增字段或枚举值，依赖 PRD-core 3.1 节"忽略不认识的字段和枚举值"。`MsgBody` 收紧只影响夹带多余字段的非法请求。向量见第 13.1 节 | — |

### 11.2 客户端 HTTP（新）

| 接口 | 鉴权 | 功能点 |
|---|---|---|
| `GET /v1/config` | 无 | F-ACC-07 |
| `POST /v1/account/register`、`login`、`recover`、`token/refresh`、`delete/cancel` | 无（限流） | F-ACC-01、02、05、03、08 |
| `POST /v1/account/logout`（Bearer 或 refresh token）；`logout-others`、`password`、`recovery-code`、`recovery-code/ack`、`delete`；`GET /v1/account/events`；`PATCH /v1/account/profile` | Bearer（要有 `sid`） | F-ACC-04、05、08、06 |
| `POST /v1/files/uploads`、`/v1/files/{id}/complete`、`/v1/files/urls`；`DELETE /v1/files/uploads/{id}` | Bearer | F-FILE-01、04 |

HTTP 接口要有 Schema（新增 `protocol/schema/http-client.schema.json`）。

### 11.3 对象存储客户端

`tianting-console-server/internal/objstore` **不能直接复用**：① 它是另一个 Go 模块的 `internal` 包，并依赖 `tianting/console/internal/secret`；② `Put` 强制 `COMPLIANCE` Object Lock（聊天文件必须能删除）；③ 没有预签名。可以复用的是 SigV4 代码和 `objstoretest` 假存储的思路，im-server 里新写一个，需要预签名 PUT、预签名 GET（带 `response-*`）、HEAD、Range GET、CopyObject（带条件和元数据替换）、DELETE、List。服务端凭证按前缀最小授权：临时前缀读、写、删；最终前缀读、写、删和**按 prefix 列举**（`file_purge` 和提交后清理要用，S3 风格的接口没有按前缀删除，要先列举再逐个删，凭记忆，验证门确认）；公开桶写、删（RS4-24、RS5-19、RS6-03）。全桶 List 只给恢复数据库后的对账任务。按它的先例不引入 SDK（包注释的理由：依赖越少供应链风险越小）。

### 11.4 服务端内部

`delivery.Router` 增加按 `sid` 踢线和按"`sid` + 代号"踢线（集群下走总线，RS4-02、RS4-03：改密时当前 `sid` 也要踢，不再需要排除）。**新增 `delivery.Presence` 接口**，由 hub 做本地实现，放"设备是否有活跃连接"，不扩 Router（Router 的约定是只投递不查询，`delivery.go` 第 15～23 行，R-CB-16）。`delivery.Session` 增加 `sid`、`gen`、最近收帧时间、前后台状态。`cache.Invalidator` 增加主题 `blockedWords`、`convSettings`。新定时任务（都只在一个节点运行，和 F-ADM-32 相同）：`file_purge`、`report_purge`、`invite_purge`、`session_purge`、`moderation_hits_purge`、`account_events_purge`、`account_cleanup`。登录失败、注册、邀请码失败的计数存数据库 `auth_throttle`，不用内存 `Limiter`（只有令牌桶 `Allow`，多节点翻倍，ARCH G2）。遵守 ARCH 第 2.2 节约束（`logic` 不依赖 `hub`）。

### 11.5 错误码（已和 `errors.json`、`ERRORS.md` 逐个核对未占用；避开了已取消的 PRD-live 里写过的 4047、4138）

| 码 | 名称 | HTTP | 通道 | 重试 | 含义 | 文案 / 动作 |
|---|---|---|---|---|---|---|
| 4018 | ACCOUNT_CREDENTIALS_INVALID | 401 | client | 否 | 用户名、密码或恢复码错误（不区分） | "用户名或密码错误" |
| 4019 | REFRESH_TOKEN_INVALID | 401 | client | 否 | refresh token 无效、过期、吊销或被重放 | 按 `reason`：`superseded` 留在当前页重试，其他回登录页（F-ACC-03 规则 6） |
| 4048 | FILE_NOT_FOUND | 404 | client、ws、admin | 否 | 文件不存在、无权访问、已过期或已撤回（不区分） | "文件不存在或已过期" |
| 4095 | USERNAME_TAKEN | 409 | client | 否 | 用户名已存在或保留 | "这个用户名不能使用" |
| 4096 | FILE_NOT_READY | 409 | ws、client | 否 | 文件未完成上传，或已绑定其他消息 | 重新上传 |
| 4097 | RECALL_WINDOW_EXPIRED | — | ws | 否 | 超过撤回时限（`details.windowSeconds`） | 按时限显示："发送时间超过 2 分钟，无法撤回"；群主、管理员"发送时间超过 7 天，无法撤回" |
| 4098 | REPORT_ALREADY_HANDLED | 409 | admin、console | 否 | 举报已被处理 | 刷新列表 |
| 4139 | FILE_TOO_LARGE | 413 | client | 否 | 超过大小限制（`details.maxBytes`） | "文件过大，最大 {N}" |
| 4150 | FILE_TYPE_NOT_ALLOWED | 415 | client | 否 | 禁止的类型或内容与声明不符 | "不支持发送这种文件" |
| 4220 | PIN_LIMIT_EXCEEDED | — | ws | 否 | 置顶数量已满 | "最多置顶 20 个会话" |
| 4293 | MENTION_ALL_LIMITED | — | ws | 否 | @ 所有人次数用完 | "@所有人次数已用完" |
| 4294 | ACCOUNT_LOGIN_LOCKED | 429 | client | 否 | 尝试过多，暂时锁定（`details.retryAfterSeconds`） | "尝试次数过多，请 N 分钟后再试" |
| 4510 | CONTENT_BLOCKED | 451 | ws、client | 否 | 内容被拦截（编号沿用 PRD-admin F-ADM-60 的建议） | "消息包含不当内容，无法发送" |
| 5032 | OBJECT_STORE_UNAVAILABLE | 503 | client | 是 | 对象存储暂不可用 | 退避重试 |

已有码的扩展：`4000`、`4004`、`4030`、`4034`、`4290`、`4292`、`5000`、`5030` 的 scope 增加 `client`；`4010`、`4011`、`4012` 增加 `client`，HTTP 状态 401（现在是 `null`，只影响新通道，WS 不变）；**`4011` 的动作改成：账号模式先比较本地代号并刷新或重连，4019 才回登录页；演示模式不变**（`errors.json` 第 16、68 行，RS5-20）；`4044` 增加 `ws`（撤回）；`4131` 增加 `ws`（举报、@ 超限）和 `client`（`/v1/files/urls`）。**`4292` 的文案按"命令 + 码"增加**：上传"今天的上传额度已用完"，举报"今天的举报次数已用完"（RC2-20）。`4290` 的 `uploadPendingLimit` 不自动重试（SDK 按 `reason` 判断）。`4030` 新 `reason`：`registerClosed`、`inviteInvalid`、`mentionAllForbidden`、`featureDisabled`、`accountSessionRequired`、`accountExists`、`accountDeletionPending`；`4019` 新 `reason`：`invalid`、`superseded`、`replayDetected`、`credentialsChanged`、`loggedOutOthers`、`accountDeleted`，`mac` 正确时都可以带 `recoverBefore`；**只有 `mac` 正确才返回 `invalid` 以外的 reason**（判定顺序见 F-ACC-03 规则 4）；`4011` 新 `reason`：`tokenSuperseded`；`5032` 新 `reason`：`completeInProgress`；`4290` 新 `reason`：`uploadPendingLimit`；`4040` 新 `reason`：`quoteUnavailable`、`reportTargetInvalid`；`4000` 新 `reason`：`passwordWeak`、`fileKindMismatch`、`customDisabled`、`uploadMismatch`、`fileNameTooLong`、`recoveryCodeStale`。

`ERRORS.md` 增加区段：`415x` 不支持的类型；`422x` 业务状态不允许；`451x` 内容治理；429x 的"不自动重试"例外增加 4293、4294。**F-SYS-01 的一致性测试要把 `client` scope 纳入**（服务端常量、三套 SDK 枚举）。

### 11.6 管理接口（新，只在管理端口；旧密钥都不能调用）

| 接口 | scope | 说明 |
|---|---|---|
| `POST /admin/invites`、`GET /admin/invites`、`POST /admin/invites/{id}/disable` | `account.invite` | F-ACC-09 |
| `GET /admin/accounts/{userId}` | `user.read` | F-ACC-09 规则 5。**没有 `account.reset`，没有改密码的管理接口** |
| `GET` / `POST` / `PATCH` / `DELETE /admin/moderation/words`、`POST .../import` | `moderation.words` | F-MOD-01 |
| `GET /admin/moderation/hits?userId=` | `report.read` | F-MOD-02 规则 7 |
| `GET /admin/reports`、`GET /admin/reports/{id}`、`POST /admin/reports/{id}/resolve` | `report.read`、`report.handle` | F-MOD-04 |
| `POST /admin/report-grants`、`POST /admin/reports/{id}/evidence` | `report.evidence`（正文类，只能配在 `bodyAccess` 调用方） | F-MOD-04 规则 3 |
| `POST /admin/files/{fileId}/fetch` | `report.evidence` 或 `msg.quarantine`（正文类调用方） | 审核员看被举报或隔离的文件，要有效授权；console-server 代理，单聊受 `IM_C2C_BODY_ACCESS` 限制 |
| `POST /admin/messages/delete` | `msg.delete`（已有） | F-RCL-04 规则 1 的幂等扩展 |
| `POST /admin/convs/messages/body`（已有） | 正文类 | 用后台版序列化：`deleted` 只表示后台删除，`recalled*` 单独（RC2-21） |
| `POST /admin/convs/messages/query`（已有） | `msg.meta` | 响应增加 `recalled*`；**文件消息只有类型和大小，文件名按正文对待**（R-CS-14） |

---

## 12. 数据、存储、安全与隐私

### 12.1 数据（新迁移从 `0015` 起；现有最新 `0014_admin_stats.sql`）

遵守 ARCH 3.1、3.3 节：只加可空或有默认值的列；时间用 `BIGINT` 毫秒；不用触发器；SQL 同时兼容 SQLite 和 Postgres；Store 新方法走契约测试。

| 表 | 主要字段 | 说明 |
|---|---|---|
| `accounts` | `user_id` PK、`username_lower` 唯一、`password_hash`、`credentials_version`、`credentials_changed_at`、`delete_requested_at`、`notice_version`、`created_at`、`last_login_at` | 演示和导入用户没有行 |
| `recovery_codes` | `user_id`、`gen`、`hash`、`acked_at`、`retire_until`、`used_at`、`revoked_at` | 每代一行；`retire_until` 换代时写一次，之后不延长；`revoked_at` 记录被旧码找回作废 |
| `users` 加 | `deleted_at`；`lower(user_id)` 上的**非唯一**索引 | `users` 不加唯一约束（R-CB-01） |
| `auth_throttle` | 键（用户名哈希 + IP 前缀等）、计数、窗口 | 重启不清零，定时清理 |
| `account_events` | `user_id`、类型、时间、IP 前缀、`sid` 的哈希 | 保留 180 天（未校准），隐私说明要写 |
| `device_sessions` | `sid` PK、`user_id`、`platform`、`device_name`、`current_hash`、`current_gen`、`next_gen`、`pendings`（最多 4 个，单独子表也可以）、`overlap_until`、`overlap_min_gen`、`row_version`、`seen_ip_prefixes`、`expires_at`、`absolute_expires_at`、`revoked_at`、`revoke_reason`、`last_seen_at`、`show_detail` | F-ACC-03 |
| `invites` | 只存哈希和后 4 位、`mismatch_count` | F-ACC-09 |
| `files` | `file_id` PK、`owner`、`kind`、`completing_until`、`completing_attempt`、`public_key`、`avatar_replaced_at`、`object_key`、`thumb_key`、`tmp_key`、`tmp_thumb_key`、`name`、`size`、`mime`、`thumb_mime`、`width`、`height`、`state`、`msg_id`、`retain_until`、`evidence_until`、`scan_state` | 一个 `fileId` 两个变体；没有 `sha256` |
| `upload_quota` | `user_id`、`day`、`bytes` | F-FILE-01 |
| `device_push` | `token_hash`、`token`（加密）、`provider`、`env`、`user_id`、`sid`、`show_detail`（只用于无 `sid`） | 令牌字段级加密 |
| `messages` 加 | `recalled_at`、`recalled_by`、`quote_msg_id`（带索引） | F-RCL、F-QUO |
| `conv_seq` 加 | `last_recalled`、`last_recalled_by`、`mod_ver` | F-RCL-03 |
| `conv_settings` | `user_id`、`conv_id`、`pinned_at`、`notify_off`、`deleted_at`、`cleared_seq` | F-CVS；只有设置过的会话才有行 |
| `conv_mentions` | `user_id`、`conv_id`、`seq` | 只记 @ 个人；清理到已读之后 |
| `group_mention_all` | `group_id`、`seq`、`ts` | @ 所有人只写一行，**不按成员展开**（AC-MEN-02-6）。次数单独计数，不靠这张表 |
| `blocked_words`、`moderation_hits` | | F-MOD |
| `reports`、`report_evidence` | 证据加密 | F-MOD-03、04 |

存储分层：消息正文和元数据继续在 SQLite / Postgres；文件内容只在对象存储。

### 12.2 安全

| 风险 | 对策 | 落在 |
|---|---|---|
| 撞库、暴力破解 | Argon2id；用户名 + IP 锁定；IP 限流（IPv6 /64）；按用户名渐进延迟；相同响应和时间；新设备登录提醒 | F-ACC-02 |
| 客服或后台接管账号 | **第一批没有任何重置接口** | F-ACC-05 规则 4、AC-ACC-05-9 |
| 匿名请求打满 Argon2 | 闸门只管算哈希的路径，先限流再进闸门；改密、`recover` 各有专用位；恢复码未命中不进闸门 | F-ACC-01 异常、F-ACC-05 规则 2 |
| 无 `sid` 的旧 token | `credentials_changed_at` | F-ACC-03 规则 11 |
| 演示登录接管账号 | 有账号的用户 `/demo/login` 拒绝 | F-ACC-07 规则 2 |
| 账号枚举 | 第 5 节各规则；PRD-social 11.2 增加注册路径 | F-ACC-01、02、05 |
| refresh token 被盗 | 轮换 + 重放检测；服务端只存哈希 | F-ACC-03 |
| 管理接口签发任意用户 token | 生产带 `token.issue` 默认拒绝启动 | F-ACC-07 |
| 刷新令牌被盗 | 新一代首次使用才生效 + 单链 + 单调代号 + access 代号判定 + 重放吊销（连续转正不能续开窗口）+ 指标和提示；盲区见 F-ACC-03 规则 4 | F-ACC-03 规则 4 |
| 只知道 `sid` 就吊销会话 | refresh token 带独立密钥的 MAC，密钥按 365 天退役 | F-ACC-03 规则 3 |
| 数据库恢复（RS4-14） | 恢复到旧时间点后：① 轮换过的客户端的值比库里新，会被当成重放批量吊销；② 已吊销的会话和已改的密码回到旧状态。运维手册要写：设置 `IM_RESTORE_EPOCH`，一段时间内把代号不匹配按软拒绝（`reason: superseded`）；同时吊销恢复点之后可能有变化的会话，提示用户重新确认 | 运维手册（分发时建任务） |
| 内部同一出口 IP（RS4-15） | 全公司共用注册 20 次 / 小时、登录 20 次 / 分钟，同办公室的人可以用"用户名 + 同一 IP"锁住同事。部署说明写明，可信出口网段可配更高额度；邀请码自动停用要告警到人，并能重新启用 | 部署说明 |
| `recover` 的 IP 限流（RS7） | 每 IP 每分钟 20 次在办公室 NAT 下可以短时压制同网用户找回，换移动网络可以绕开；部署说明写明，可信出口网段可配更高额度 | 部署说明 |
| 文件被未授权下载 | 随机键；只有鉴权后发 120～300 秒的地址；桶私有、禁止列举 | F-FILE-04 |
| 校验后覆盖对象 | PUT 只指向临时键，`complete` 取租约后条件 Copy 到每次新的随机键，元数据替换 | F-FILE-01 规则 3 |
| 压制受害者找回 | `recover` 不做版本判断，命中的请求不受锁定和单哈希限制，有独立专用位 | F-ACC-04 规则 8、F-ACC-05 规则 2 |
| 文件名进后台元数据 | 文件名按正文对待 | AC-MOD-04-11 |
| 伪造缩略图、解码炸弹 | 服务端读真实宽高；审核员看两份；客户端解码上限 | F-FILE-02 规则 3 |
| 上传恶意内容 | 图片白名单、文件禁止名单；嗅探；响应头由服务端固定；附件方式下载；独立域名；服务端不解码像素 | F-FILE-01、03、05 |
| 对象存储被用作免费网盘 | 每人每天额度；申请频率和未完成数；未绑定 24 小时清除；头像先私有后复制 | F-FILE-01、06，F-ACC-06 |
| 推送泄露内容和社交关系 | 默认隐藏；整个请求（含请求头）不含 ID；collapse id 不透明；角标数字保留 | F-APN-02、03 |
| 撤回内容从引用、缓存泄露 | 下发时组装；客户端清除；AC 用特征串检查 | F-QUO-02、F-RCL-03 |
| 探测词库 | 不告诉命中词；放在权限判定之后；记录命中 | F-MOD-02 |
| 举报被滥用 | 限额；幂等；不自动处罚；已撤回的要 `readSeq ≥ seq`；举报不影响文件访问 | F-MOD-03、04 |
| 日志泄露 | 扩展 ARCH S9 的特征串测试 | AC-CHAT-90-5 |

ARCH 第 4 节"明确不做 E2EE"不变。

### 12.3 合规（**CH-23 已定内部小范围；需要问法务，本节不是法律意见**）

**默认假设：内部 / 小范围**（邀请码注册、不上架公开应用商店、用户是认识的人）。**中间情形**：有真实用户、邀请制，但用 App Store 或 TestFlight 分发，"内部告知"可能不够，要一起问法务（R-CS CH-23）。

**PRD-admin 第 12 节的门槛仍然有效**（已冻结）：开放群消息正文查看、删除消息与隔离原文，要先完成隐私政策发布、个人信息影响评估、操作规程和培训；其中"宿主 App"现在就是天听本身（第 16.1 节）。第一批的撤回隔离和举报证据适用同一门槛。

| 项 | 内部 / 演示（默认） | 面向公众开放（通常涉及，具体要法务确认） |
|---|---|---|
| 注册 | 邀请码 + 用户名 | 通常要求手机号等真实身份信息认证，要接短信、改注册流程 |
| 内容安全 | 敏感词 + 举报 + 人工处理 | 通常要求更完整的审核能力、专人值守、处置和上报流程，包括图片内容 |
| 备案和许可 | 不涉及对外服务时一般不需要（未确认） | ICP 备案、App 备案、可能的增值电信许可、安全评估，**都要问法务** |
| 个人信息 | 按 PRD-admin 第 12 节至少告知：撤回原文保留 30 天、举报证据保留 180 天、账号事件保留、详情模式内容经过第三方 | 隐私政策、用户协议、同意流程、第三方（苹果、谷歌、对象存储）的披露、未成年人条款 |
| 日志留存 | 现有审计 1 年 | 法定留存要求（要法务给口径） |
| 消息和文件留存 | 第 4.1 节默认值 | 由法务定，配置可调 |
| 单聊正文 | `IM_C2C_BODY_ACCESS` 默认关（X-7） | 法务书面确认后再决定 |
| 账号注销 | F-ACC-08 | 应用商店通常要求 App 内可以注销（苹果的现行条款**未联网核实**） |
| 跨境 | FCM、APNs 的数据出境 | 要法务评估 |
| 病毒扫描、图片审核 | 不做 | 建议做，第一批之外另行排期 |
| 开放注册的技术条件 | 不能开 `open` | `open` 要求 `IM_MOD_ENABLED=1`、词库非空、`IM_PUBLIC_OPEN_ACK=1`（AC-ACC-01-9）；人机验证第一批没有；C6 上线前不能开 |

**结论**：第一批的实现按内部 / 小范围验收。面向公众开放不是改配置就能做到的，至少要补手机号、审核和法务手续。

---

## 13. SDK 和各端

### 13.1 SDK 新接口（TS 写法；Kotlin、Swift 语义相同）

| 能力 | 接口 |
|---|---|
| 账号（HTTP，独立于 `IMClient`） | `AccountClient.register/login/refresh/logout/logoutOthers/changePassword/recover/regenerateRecoveryCode/ackRecoveryCode/updateProfile/getConfig/listEvents/requestDelete/cancelDelete`；错误枚举含 `client` scope |
| token 提供者 | `IMClient.connect(tokenProvider)`：SDK 在需要时调用；保留现有 `connect(token)`（演示模式） |
| 文件 | `sendImage(conv, processed, {onProgress, signal})`、`sendFile(...)`、`getFileUrls(items)`、`cancelUpload`（调 `DELETE /v1/files/uploads/{id}`）。**图片压缩、缩略图、EXIF 清理在 App 或平台适配层，SDK 只接收处理好的结果**（`sdk-core` 是纯 JVM，sdk-ts 测试环境没有 canvas，R-CF-24） |
| 撤回 | `recall(convId, msgId)`；复用 `msgDeleted` 回调，消息带 `recalled`；清除本地原文、引用和缓存 |
| 引用、@ | 现有 `send(convId, text)` 保留，增加可选参数 `{quote?, mentions?}`（R-CF-26） |
| 会话 | `setConvPinned`、`setConvNotifyOff`、`deleteConv`；`conversations` 按新排序；`totalUnread()` 按 F-CVS-02 规则 4 |
| 设备通知（只有原生） | `deviceNotificationRegister`、`deviceNotificationUnregister`、`setDeviceNotificationDetail`、`setAppState`。名字避开现有系统通知的 `push`（R-CF-26） |
| 举报 | `report(...)` |
| 重连 | `recheckDeleted` 改成只重拉 `modVer` 变化的会话；Store 单调合并规则见 F-RCL-03 规则 2a；刷新单飞见 F-ACC-03 规则 5；`convs` 的 `deleted` 和 `minSeq` 用于清除本地（F-CVS-03 规则 6） |

**共享用例**（R-CF-23）：现有 `frames.json` 只验语法（sdk-ts 只验下行不抛异常），行为差异测不出来。新增 `protocol/vectors/chat-sync.json`（撤回合并、引用清除、提及计数、置顶排序、`deleted` 判断、总未读）和 `account-session.json`（续期、重放、失败分类、附件状态机），步骤式，写法同 `push-sync.json`，四方都跑（Go 参考客户端产出期望值，同 `social.json`）。新增 `protocol/schema/http-client.schema.json`。三套 SDK 都要有假 HTTP。同步更新过期的 `docs/sdk/README.md`（还写着 Kotlin、Swift"待实现"）。

SDK 日志不打印 token、refresh token、恢复码、推送令牌、预签名地址、文件名。

### 13.2 各端差异

| 项 | H5 | Android | iOS |
|---|---|---|---|
| token 存储 | `sessionStorage`（CH-26） | Keystore 加密文件 | Keychain（`AfterFirstUnlockThisDeviceOnly`，现有） |
| refresh TTL | 24 小时 | 90 天 | 90 天 |
| 离线通知 | 无 | FCM；没有 Google 服务时无（F-APN-06） | APNs |
| 选图 | `<input type=file>`，拖放、粘贴 | 系统照片选择器，不要存储权限 | PHPicker，不要相册完整权限 |
| 选文件 | `<input type=file>` | SAF | `UIDocumentPicker` |
| 拍照 | 不做 | 不做（第一批） | 不做（第一批） |
| 图片处理 | canvas（HEIC 看浏览器） | 系统图片接口（API 28+ 才尝试 HEIC） | ImageIO |
| 源文件 | 内存中的 File，刷新丢失 | 复制到 App 沙盒 | 复制到 App 沙盒 |
| 下载 | 浏览器下载，无进度 | App 内 OkHttp 到私有缓存，SAF 交出 | App 内 URLSession，分享面板 |
| 上传进度 | `XMLHttpRequest` | OkHttp | URLSession |
| 切后台时的上传 | 不保证 | 不保证 | 不保证 |
| GIF | 播放 | 首帧 + 标记 | 首帧 + 标记 |
| 本地缓存 | 内存（D4） | SDK 本地库 + 文件缓存（键 `fileId + variant`） | 同左 |
| 长按菜单 | 窄屏长按、宽屏右键、键盘菜单键 | 长按 | 长按 |
| 前后台上报 | 不上报（标签页隐藏不算后台） | 上报 | 上报（进入后台那一帧可能发不出去，服务端靠连接断开兜底，未实测） |
| 工程配置 | nginx、vite 转发 `/v1/`；CSP 增加具体存储域名和 `blob:`（构建变量注入，不能放宽成 `https:`）；存储 CORS 允许上传头 | Manifest 加 `POST_NOTIFICATIONS`；Firebase 占位配置 | `.entitlements` 加 `aps-environment` |
| 最低版本 | 沿用 | minSdk 24 | iOS 17 |
| AC 写法 | 按 PRD-ui 3.6 节的对应规则 | 同左 | 同左 |

### 13.3 界面变更（只描述交互）

| 页面 | 变更 |
|---|---|
| 登录页 | 账号登录、注册、忘记密码（只有恢复码流程）、保存恢复码、撤销注销、隐私说明入口；演示表单按配置显示 |
| 我 / 设置 | 新增"账号安全"（改密、重新生成恢复码、退出其他设备、最近登录记录、注销，演示会话隐藏）、"通知"（系统授权状态、显示发送者和内容）；"我"页可以编辑昵称和头像 |
| 会话列表 | 长按或右键菜单；置顶区分；免打扰图标和小红点；摘要前缀优先级 |
| 聊天页 | 输入区增加"+"（图片、文件，`features.files` 关闭时隐藏）；引用条；@ 成员选择；图片全屏；文件下载；撤回文案和"举报"；"有人@我"按钮 |
| 消息菜单 | "复制""引用""撤回""举报"按状态显示（F-UI-09 扩展） |
| 资料卡、群信息 | "举报" |

视觉遵守 PRD-ui 第 6 节，新图标加入 6.6 节清单，新增的提示色、占位色要有深色 token。不使用其他产品的图标、提示音。

### 13.4 页面状态和无障碍（R-CF-22）

| 页面 | 要覆盖的状态 |
|---|---|
| 注册、找回 | 提交中、4004、4000、4030、4095、4290、4294、5030、网络错误 |
| 保存恢复码 | 复制成功、复制失败、被杀后恢复 |
| @ 选择 | 加载中、无结果、加载失败（可继续发普通文本） |
| 通知设置 | 系统授权关闭、令牌获取中、令牌获取失败、设备不支持（F-APN-06）、保存失败 |
| 图片和文件气泡 | 处理中、上传中（进度）、失败、下载中、已过期、解码失败、保存权限被拒、存储空间不足 |
| 引用条 | 正常、已撤回、已删除、已过期 |

无障碍：新增的只有图标的按钮要有可读名称（NF-UI-7）；图片的替代文本"图片"，文件读文件名和大小；新消息播报（NF-UI-11）的内容：图片读"收到一张图片"，撤回读"一条消息已被撤回"；长按菜单要有辅助操作，H5 要有键盘入口（现在 `ChatPage.tsx` 只有 pointer 和 contextmenu）；管理员自己撤回别人消息时的文案见 F-RCL-02 规则 1a。

---

## 14. 后台配合

### 14.1 管理后台（`tianting-admin`、console-server）

| 项 | 内容 | 新权限点 |
|---|---|---|
| 用户详情 | 账号信息（F-ACC-09 规则 5），**没有重置密码** | — |
| 邀请码 | 新页面 | `account.invite`（`operator`） |
| 词库 | 新页面（F-MOD-01）；命中统计 | `moderation.words`（`operator`、`moderator`） |
| 举报 | 新页面（F-MOD-04） | `report.read`、`report.handle`、`report.evidence`（`moderator`，敏感） |
| 消息元数据页 | 显示撤回状态、类型和大小；**不显示文件名**（按正文对待，R-CS-14） | 沿用 `msg.meta` |
| 正文页 | 文件名、文件内容凭授权看，文件经 console-server 代理，后台 CSP 不放宽（R-CS-24） | 沿用 `msg.read.group` |
| 超管 | 默认**没有**举报证据权限（沿用 F-CON-06 规则 4 的职责分离），应急授权照旧 | — |

console-server 主凭证 `console` 增加 scope：`account.invite`、`moderation.words`、`report.read`、`report.handle`；`console-msgread` 增加 `report.evidence`。所有写操作走现有两阶段审计。

### 14.2 运维后台（`tianting-ops`）

| 项 | 内容 |
|---|---|
| 配置检查（F-OPS-10） | 增加：注册模式、邀请码总量告警、隐私说明版本、`token.issue` 放行告警、`custom` 打开告警、对象存储连通和 HTTPS、APNs / FCM 启用和密钥状态、词库是否为空、`IM_REPORT_KEY` |
| 指标（F-OPS-06） | `im_account_logins_total{result}`、`im_registers_total{result}`、`im_refresh_reissue_total`、`im_refresh_replay_total`；`im_file_uploads_total{kind,result}`、`im_file_bytes_total`、`im_file_url_issued_total`、`im_objstore_errors_total`；`im_offline_push_total{provider,result}`、`im_offline_push_dropped_total`、`im_offline_push_queue_len`；`im_recalls_total{by}`；`im_blocked_total{scene}`；`im_mod_reload_errors_total`；`im_reports_pending`。标签不含 userId 等无界字段（AC-OPS-90-4） |
| 告警（F-OPS-07） | 重放吊销突增、对象存储错误率、推送失败率、队列满、登录失败突增、词库加载失败、当天全局上传量（阈值都未校准，第一批默认不发页面告警，先观察；词库加载失败除外） |
| 备份（F-OPS-12） | 明确说明不含文件内容（F-FILE-06 规则 8） |

---

## 15. 非功能（F-CHAT-90）

**所有数字都未校准**。第一批没有 EP 测试机（PENDING P-12）；开发机结果只作参考，不能作为发布结论。**判定方法（R-CQ-22，沿用 P-20）**：主判据用绝对值，增幅只记录；每条写明负载和样本数（≥ 1000），生产配置，**3 次取最差**（和 TC-PERF 一致），报告写基线提交号。

| 类别 | 要求 |
|---|---|
| 不回归 | 新开关全关时，现有 e2e 全过，TC-PERF 判定不变 |
| 邀请总量 | 配置检查告警，不做硬限制 |
| HTTP | 登录 P99 ≤ 500 ms（含 Argon2）；申请上传 P99 ≤ 200 ms |
| Argon2 内存 | 同时 4 个约 256 MiB，部署要算进 `GOMEMLIMIT`（ARCH 第 7 节） |
| 容量和费用假设 | 1000 注册用户、200 同时在线、人均每天 5 张图片（约 1.5 MB）、1 个文件（约 5 MB）→ 约 12.5 GB / 天；180 天约 2.3 TB（公式：用户数 × 单人每日字节 × 保留天数）。**出口流量另算**：每人每天 ≈ 上传量 × 平均接收人数，群聊放大明显（R-CS-21）。Frank 选存储时看这两项 |
| 推送 | 入队到调用通道 P99 ≤ 2 秒 |
| 词库 | 10 万词，5000 码点文本单次匹配 P99 ≤ 2 ms |
| 集群（R-CB-16） | **推送模块的在线判断、合并、队列、补推定时器在 B6 需要重写**（都是节点本地状态）。能复用：`push.Provider`、令牌和设置存储、推送规则判断、`Invalidator` 主题、数据库配额、Store 方法。第一批推送只支持单节点部署。其他新功能走 `Router`、`Presence`、`Invalidator`、`Store` 接口 |
| 可用性 | 对象存储、APNs、FCM 不可用**不影响文本消息收发**，也不是 `/readyz` 的条件 |

**性能结论的判定**（RC2-14）：开发机结果只记录，**不算通过**；发布判定等目标机（P-12）。里程碑可以在"性能未判定"的状态下内部使用。

**【全部】的含义**（RC2-14）：每个里程碑发布前跑一次，**范围是到这个里程碑为止已上线的部分**，下表累计：

| AC | C1 | C2 增加 | C3 增加 | C4 增加 | C5 增加 | C6 增加 |
|---|---|---|---|---|---|---|
| AC-CHAT-90-1 | 单聊文本（账号 token） | 不变 | 不变 | 带引用的单聊、带 @ 的群聊 | 不变 | 开词库 |
| AC-CHAT-90-2 | C1 之外的开关本来就不存在 | `IM_FILES_ENABLED` 关 | 推送开关关 | C4 开关关 | C5 开关关 | C6 开关关 |
| AC-CHAT-90-3 | 不适用 | 对象存储不可用 | APNs、FCM 不可用 | 不变 | 不变 | 不变 |
| AC-CHAT-90-5 | 注册、登录、刷新、改密、找回 | 上传、下载 | 推送 | 撤回、引用、@ | — | 举报、附言 |
| AC-CHAT-90-8、90-9 | 账号页面 | 文件页面 | 通知设置 | 菜单和引用条 | 列表菜单 | 举报入口 |
| AC-CHAT-90-10 | `kickReasons`、`securityNotice` | `image`、`file`、`MsgBody` 收紧、`modVer` | 通知命令、`presence` | `recall`、`quote`、`mentions`、`msgDeleted.recalled` | `convSet`、`convDelete`、新事件 | `report` |
| AC-CHAT-90-15 | `account-session.json` | `chat-sync.json` 的合并和过期 | — | 撤回、引用、提及 | 置顶、删除、总未读 | — |
| AC-CHAT-90-17 | 账号指标 | 文件指标 | 推送指标 | 撤回指标 | — | 治理指标 |

**验收标准**
- AC-CHAT-90-1【后端】【全部】：输入 文本（范围见上表），500 条/秒，≥ 1000 条，生产配置，3 次取最差；操作 压测；预期 ack P99 ≤ 200 ms（未校准），错误率 0；增幅只记录；开发机结果不算通过；错误码 —。（v0.3 改：RC2-12、RC2-14）
- AC-CHAT-90-2【后端】【全部】：输入 `IM_FILES_ENABLED`、`IM_OFFLINE_PUSH_ENABLED`、`IM_MSG_ACTIONS_ENABLED`、`IM_CONV_SETTINGS_ENABLED`、`IM_MOD_ENABLED` 全关；操作 跑现有 e2e 和 TC-PERF；预期 e2e 全过，TC-PERF 判定通过；错误码 —。（v0.2 改：R-CB-06、R-CQ-22）
- AC-CHAT-90-3【后端】【全部】：输入 依赖（范围见上表）分别按拒绝连接和黑洞两种方式不可用；操作 文本消息收发 5 分钟，≥ 1000 条；预期 全部成功，ack P99 ≤ 200 ms（未校准，开发机结果不算通过）；`/readyz` 仍就绪；错误码 —。（v0.3 改：RC2-14）
- AC-CHAT-90-4【后端】【C6】：输入 10 万词的词库（随机中英文，长度 2～8），5000 码点文本（中文为主，不命中），10000 次；操作 单线程测量每次耗时；预期 P99 ≤ 2 ms（未校准）；错误码 —。（v0.2 改：R-CQ-22）
- AC-CHAT-90-5【后端】【全部】：输入 带特征串的密码、恢复码、邀请码、token、refresh token、推送令牌、预签名查询串、文件名、`deviceName`、引用文字、举报备注、好友附言、消息正文；操作 走注册、登录、刷新、改密、找回、上传、下载、推送、撤回、举报这些流程后扫描服务端日志和审计；预期 都搜不到；错误码 —。（v0.2 改：R-CQ-36、R-CS-28）
- AC-CHAT-90-6【后端】【C1】：输入 全新数据库和 `0014` 版本的数据库（含 `Alice`、`alice` 两个用户）；操作 `im-server migrate up`；预期 都成功，已有数据不变；升级后旧版本二进制按现有规则拒绝启动；错误码 —。（v0.2 改：R-CB-01）
- AC-CHAT-90-7【前端】【C4】（老版本三套 SDK）：输入 已加载含特征串的消息；操作 收到 `image`、`file`、带 `recalled` 的 `msgDeleted` 和带 `deleted`+`recalled` 的 `sync`、新事件和新字段；预期 不崩溃；图片、文件显示"[暂不支持的消息]"；撤回的消息显示"该消息已被删除"，**内存和本地库搜不到原文特征串**；错误码 —。（v0.2 改：R-CB-05、R-CF-04）
- AC-CHAT-90-8【前端】【全部】：输入 新增界面、文案、构建产物；操作 扫描；预期 符合 PRD-ui B-1（AC-UI-90-15）和 B-3（AC-UI-11-3），新增图标都在清单里；错误码 —。（v0.2 改：R-CQ-38）
- AC-CHAT-90-9【前端】【全部】：输入 新增页面（第 13.4 节）；操作 跑 axe-core；深色模式截图；键盘走查 H5 消息菜单；预期 没有 `button-name` 违规；新增颜色都来自 token；键盘能打开菜单；错误码 —。（R-CF-22）
- AC-CHAT-90-10【后端】【全部】：输入 第 11.1 节全部新帧、新事件、新字段；操作 `protocol/vectors` 校验（服务端和三套 SDK 都跑）；预期 全部通过；sdk-ts 也做 Schema 校验；错误码 —。（R-CQ-44、R-CF-23）
- AC-CHAT-90-11【后端】【C1】：输入 串行和 4 并发登录，≥ 1000 次，生产配置，3 次取最差；操作 测量；预期 P99 ≤ 500 ms（未校准），峰值存活内存增加 ≤ 300 MiB（未校准）；错误码 —。（R-CQ-22）
- AC-CHAT-90-12【后端】【C2】：输入 ≥ 1000 次申请上传；操作 测量；预期 P99 ≤ 200 ms（未校准）；错误码 —。（R-CQ-22）
- AC-CHAT-90-13【后端】【C3】：输入 ≥ 1000 条发给离线用户的消息；操作 测量入队到假通道收到；预期 P99 ≤ 2 秒（未校准）；错误码 —。（R-CQ-22）
- AC-CHAT-90-14【后端】【C1】：输入 `errors.json` 新增的 `client` scope 码；操作 跑 F-SYS-01 的一致性测试（服务端常量、三套 SDK 枚举）；预期 一致；错误码 —。（R-CQ-44）
- AC-CHAT-90-15【前端】【全部】（三套 SDK）：输入 `chat-sync.json`、`account-session.json`；操作 跑共享用例；预期 全部通过；错误码 —。（R-CF-23）
- AC-CHAT-90-16【后端】【C1】：输入 `/v1/*` 请求体 65 KB；慢速请求超过 10 秒；生产环境 http 请求；过期 token、被下线的 token；操作 分别请求；预期 400 / 4000；断开；拒绝；401（4011、4012）；错误码 4000、4011、4012。（R-CQ-23）
- AC-CHAT-90-17【后端】【全部】：输入 第 14.2 节的指标和配置检查项；操作 走对应流程后抓 `/metrics` 和 `GET /internal/v1/config`；预期 都出现，标签没有无界字段；错误码 —。（R-CQ-23）
- AC-FILE-90-1【后端】【C2】：输入 CI 固定版本的真实 MinIO，开启服务端加密（或试用桶）；操作 跑对象存储合同测试：预签名 PUT 到临时键、超大 PUT、过期 PUT、**未签名的头**、`complete` 后覆盖、Copy、**条件 Copy 成功和失败（412）**、**`If-Match` 的 Range GET 成功和失败**、**元数据替换（含恶意头，AC-FILE-01-21）**、**跨桶 Copy（头像）**、**加密下 Copy 前后 ETag 的关系**、`response-*` 头、过期 GET、无签名访问、匿名列举被拒绝、**用真实权限的凭证按前缀列举并删除，全桶列举被拒绝**、CORS 预检、删除、**临时前缀生命周期规则（检查配置生效；实际清除按厂商周期记录）**；预期 全部符合 F-FILE-01、03、04，或把不符合的写进部署说明；错误码 —。（R-CQ-19、R-CB-27、RS4-18）
- AC-FILE-90-2（手工）【C2】：输入 Frank 选定的云厂商；操作 跑同一套合同测试（上线前）；预期 全部通过，不通过的能力写进部署说明；错误码 —。（R-CQ-19）

---

## 16. 对已冻结文档和任务的影响（冻结后走变更流程，不在这里改）

### 16.1 文档和规则

| 文档 | 位置 | 影响 |
|---|---|---|
| PRD-core v1.4 | 第 704 行（图片、文件"二期 P1"） | 图片、文件提前到第一批；语音到后续批；表情包不在范围 |
| | 第 705 行（撤回、编辑、引用、@…"三期"） | 撤回、引用、@ 到第一批；编辑、正在输入、已读回执仍不做 |
| | 第 709 行（离线推送"随原生端"） | 移到本文 C3 |
| | 第 512 行（F-CONV-01 规则 4 一期不做置顶等） | **作废**，见 F-CVS |
| | F-CONV-01 规则 1、2、3 | `convs` 增加字段和 `deleted`；排序增加置顶；删除的会话不出现 |
| | F-CONV-02 规则 5、F-UI-23 | 总未读口径（F-CVS-02 规则 4） |
| | F-GRP-03 规则 4 | 快照要排除 `deleted` 列表 |
| | F-AUTH-01 规则 1、F-AUTH-02、F-AUTH-03、F-AUTH-04、F-AUTH-05 | IM 开始维护账号；`auth` 检查 `sid`；F-AUTH-03 规则 5 不变（`replaced` 不做） |
| | F-USER-01 规则 4、F-USER-02 | 注销；资料可自己改 |
| | F-MSG-01 规则 6、F-MSG-05 规则 7、F-MSG-06 规则 6、3.1 节消息类型 | 第 4.3、11.1 节 |
| | C-18（userId 区分大小写） | **不变**（R-CB-01） |
| | 第 1、2 节背景和角色 | "业务方"表述更新 |
| PRD-social v1.0 | 1.3 节防枚举指标、11.2 节 | 增加注册路径：每 IP 每小时 20 次、每邀请码 20 次（F-ACC-01 规则 7） |
| | F-SOC-02 | 附言过敏感词（C6，在存在性判断之前） |
| | F-SOC-09、F-SOC-10 | `userId` 是用户名，行为不变；昵称可变 |
| | `groupMembers` | 增加 `keyword` |
| PRD-ui v1.2 | 第 126 行、第 11 节、G-UI-05～08、10、11、13 | "不做"相应改掉 |
| | 第 245 行（App 图标角标"后续"） | iOS 角标进第一批 |
| | F-UI-03 | 登录页重写 |
| | F-UI-04 规则 5、9，F-UI-09 规则 4、5，F-UI-16，F-UI-23 | 第 13.3 节、F-MEN-03 规则 2 |
| | F-UI-42 | 增加撤回的并列规则 |
| | 3.4 节 token 存储、D-UI-10 | 原生端增加 refresh token；H5 不变（CH-26） |
| | NF-UI-14（第 1528 行）、`tianting-web/deploy/nginx.conf`、`vite.config.ts` | `connect-src` 加具体存储域名，`img-src` 加 `blob:`，`/v1/` 转发 |
| PRD-core、PRD-ui | F-AUTH-02 规则、F-UI-15 | "关于天听"增加隐私说明（F-ACC-10） |
| PRD-admin v1.2 | F-ADM-60 | P2 → 第一批；替换模式去掉；错误码 4510 写入 `errors.json` |
| | F-ADM-31 规则 6、7、F-ADM-32 | 撤回消息的幂等；隔离区包含撤回原文和文件名；文件处理 |
| | F-KEY-02 规则 2、4、5，第 5 节权限点矩阵 | 第 11.6、14.1 节；业务方典型配置去掉 `token.issue`；`report.evidence` 加进 `IsBodyScope` |
| | 第 12 节 | "宿主 App"改为天听；门槛适用撤回隔离和举报证据 |
| PRD-ops v1.3 | F-OPS-06、07、10、12 | 第 14.2 节 |
| PRD-live v1.2 | F-ROOM-11 | 聊天室已取消，只参考；4510 在本文落地；4047、4138 不复用 |
| ARCH-v1 | S13、S14、3.4 节、第 2 节图、G1、G2 | 内容安全从"业务方回调"改为内置（回调不做）；新增对象存储和推送通道依赖；备份不含文件；`Presence` 现在就定义；S4 增加 `open` 和 `token.issue` 的启动保护 |
| protocol/ | `README.md`（"token 由业务服务端"）、`frame.schema.json`、新 HTTP Schema、`errors.json`、`ERRORS.md`、`ADMIN.md`、`vectors/` | 第 11 节 |
| docs/sdk/README.md | 全文 | 已过期，随 C1 更新 |
| tools/trace.py | `PRDS` 文件名写死 | 冻结时加入本文，且各 PRD 升版要改文件名；端到端 AC 有一个任务就算覆盖、三端不分端这两个局限不在本次改，分发时人工核对（R-CQ-43） |
| docs/tasks、docs/testcases | | 新 BE、CS、FE、TC；下表 AC 的 TC 要改 |
| QUEUE、PENDING | Q22、P-26、P-28 | 已更新 |
| 现有任务 | BE-340、CS-140～142（B6）；P-12、P-13、P-15、P-21 | 不受影响但仍阻塞：CI、真机、压测机 |

### 16.2 冻结 AC 按编号（已逐条在 PRD 里核对原文，R-CQ-15）

| AC | 现在写的 | 影响 |
|---|---|---|
| AC-UI-03-9 | 生产构建显示"请从业务方入口进入天听" | **作废**，由 AC-ACC-07-5 取代；要处理对应 TC，否则 `trace.py` 报 UNKNOWN |
| AC-UI-09-1 | 菜单"只有复制" | 改写：C4 开关关闭时只有复制；开启时按消息状态 |
| AC-UI-09-7 | `custom` 不出现菜单 | **不变**；F-RCL-01 规则 2 已改成 `custom` 没有客户端入口 |
| AC-UI-15-3 | "我"页没有编辑入口 | 改写：账号会话有；演示会话没有 |
| AC-UI-16-6 | 设置页依次 4 项 | 改写：增加"账号安全""通知"（演示会话不显示账号安全） |
| AC-UI-23-1、23-2 | 总未读是全部之和 | 没有免打扰时结果不变；口径加注 F-CVS-02 规则 4 |
| AC-UI-90-13 | CSP 含 `script-src 'self'` 等 | 断言不变；NF-UI-14 的 `connect-src`、`img-src` 改写 |
| AC-AUTH-02-4 | 收到 4010 / 4011 后不重连，回登录页 | 账号模式下 4011 先刷新一次（F-ACC-03 规则 7）；演示模式不变 |
| AC-CONV-01-1 | `convs` 字段完整 | 增加可选字段，断言不变 |
| AC-GRP-03-3 | `convRemoved` 显示"你已不在该群" | 不变；增加 AC-CVS-03-5 区分删除会话 |
| AC-ADM-30-7 | 元数据响应没有正文 | 扩展到文件名（AC-MOD-04-11） |
| AC-ADM-31-6 | 已删除的幂等 | 扩展到已撤回的（AC-RCL-04-1） |
| AC-ADM-60-1 | 拦截词被拒绝 | **作废**，并入 AC-MOD-01-1；TC-ADM-60-01 改关联 |
| AC-ADM-60-2 | 替换为星号 | **作废**（只拦截）；TC-ADM-60-02 删除 |
| AC-KEY-02-2 | 参数化遍历 scope 表 | 表增加新 scope |
| AC-SOC-02-5 | 附言校验 | 不变；另加 AC-MOD-02-5 |
| AC-CON-10-3 | 后台 CSP | **不变，不放宽**（AC-MOD-04-12） |
| TC-USER-01-16 | userId 区分大小写（P0） | 不变；`server/e2e` 里搜不到对应代码，由 AC-ACC-01-5 补上自动化 |

### 16.3 本文废弃的 AC 编号（不复用）

| 编号 | 原内容 | 原因 |
|---|---|---|
| AC-ACC-09-3 | 重置码过期和只能用一次 | 第一批没有重置（R-CS-01） |
| AC-ACC-09-4 | 生成重置码的权限和 step-up | 同上 |
| AC-ACC-04-9 | 改密后旧恢复码立即失效 | v0.3 相反（旧码保留 7 天，RC2-02），由 AC-ACC-04-13 取代 |
| AC-ACC-05-4 | 重新生成后旧码立即失效 | 同上，由 AC-ACC-05-12 取代 |
| AC-APN-02-11 | 双节点启动告警 | v0.1 定稿时已删除，这个编号不再使用 |

v0.1 的其他编号都保留。标"（v0.2 改）"的是同一验证目的修正预期。

---

## 17. 里程碑和依赖

| 里程碑 | 开工前必须有 | 交付后可以演示 |
|---|---|---|
| 冻结（已完成） | 拍板、四轮安全复审、MinIO 技术验证。**第 20.0 节的 3 个事实问题 Frank 没有回答，按工作假设冻结**；假设不成立就走变更流程 | — |
| C1 | 拍板已完成 | 邀请码注册、登录、找回 |
| C2 | 拍板已完成；一个对象存储（开发期可以用 MinIO，仓库的 `deploy/testenv-ci.sh` 已有固定版本的构建）；**生产上线前 Frank 选定云厂商**（CH-05） | 发图片和文件 |
| C3 | **CH-27 校准后 Frank 决定继续**；**Frank 完成第 20.3 节的账号步骤**；真机 | 后台收通知 |
| C4 | **CH-27 校准后 Frank 决定继续**；**PRD-admin 第 12 节 ②③④ 完成后才能在生产打开**（CH-17 定了留 30 天，开发不受影响）；隐私说明版本已配置 | 撤回、引用、@ |
| C5 | **CH-27 校准后 Frank 决定继续** | 置顶、免打扰、删除 |
| C6 | **CH-27 校准后 Frank 决定继续**；**Frank 指定词库负责人和举报处理人**（CH-19、CH-22）；PRD-admin 第 12 节的门槛（证据查看适用） | 拦截、举报、后台处理 |

每个里程碑有开关（第 4.1 节），SDK 发布后再开，和 `IM_V13_ADMIN_FEATURES` 的做法一致。CH-27 已定：C1 结束后校准，再决定 C3 起的部分。

---

## 18. 工作量粗估（人日，**低置信**，没有校准）

v0.1 的 138.5 明显偏低（R-CB-28、R-CF-24）。v0.2 重估为 268；v0.3 加了账号状态机（RC2-01～06）、隐私告知（RC2-16）、条件 Copy 和取消接口（RC2-07～09），并把 `mod_ver` 和合并规则从 C4 提到 C2（RC2-13），合计 **275.5**。对照 PRD-social、PRD-admin 的同口径写法，**和 AI 实际开发的对应关系没有验证过**，C1 结束后回看校准。

| 功能 | 服务端 | console + 后台页面 | sdk-ts | sdk-kotlin | sdk-swift | H5 | Android | iOS | 测试 | 部署 | 文档 | 小计 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C1 账号 | 18.5 | 1.5 | 3.5 | 4 | 4 | 5 | 5.5 | 5.5 | 8.5 | 1.5 | 3 | 60.5 |
| C2 图片和文件 | 14.5 | 2.5 | 3.5 | 5.5 | 5.5 | 6 | 8 | 8 | 9.5 | 3 | 1.5 | 67.5 |
| C3 离线推送 | 13 | 0.5 | 0 | 3 | 3 | 0 | 5 | 5 | 6 | 2 | 1 | 38.5 |
| C4 撤回 | 3.5 | 1 | 1 | 1.5 | 1.5 | 1.5 | 2 | 2 | 3.5 | 0 | 0.5 | 18 |
| C4 引用 | 1.5 | 0 | 1 | 1.5 | 1.5 | 1.5 | 1.5 | 1.5 | 1.5 | 0 | 0.5 | 12 |
| C4 @ | 3.5 | 0.5 | 1 | 1.5 | 1.5 | 2 | 2.5 | 2.5 | 3 | 0 | 0.5 | 18.5 |
| C5 会话设置 | 5 | 0.5 | 1.5 | 2.5 | 2.5 | 2 | 3 | 3 | 4 | 0 | 0.5 | 24.5 |
| C6 敏感词和举报 | 6.5 | 6 | 1 | 1 | 1 | 1.5 | 2 | 2 | 5 | 0.5 | 1.5 | 28 |
| 公共：共享向量、HTTP Schema、假 S3 和假通道、冻结文档变更 | 3 | 0 | 1 | 1 | 1 | 0 | 0 | 0 | 6 | 1 | 4 | 17 |
| **合计** | **69** | **12.5** | **13.5** | **21.5** | **21.5** | **19.5** | **29.5** | **29.5** | **47** | **8** | **13** | **284.5** |

SDK 和客户端合计 135 人日（v0.1 是 60）。**里程碑小计**：C1 60.5、C2 67.5、C3 38.5、C4 48.5、C5 24.5、C6 28、公共 17。RS4 修订加了 9 人日（C1 6.5、C2 2.5，低置信）。RS5 修订没有单独重估，C1 结束后一起校准。

**切分**（CH-27）：RECHECK 第六部分基于 v0.2 的数字核对无误（126.5 / 104.5 / 135 / 151）；按 v0.3 的表重算：

| 方案 | v0.2 基础 | v0.3 | 占比 |
|---|---|---|---|
| 完整第一批 | 268 | 275.5 | 100% |
| C1 + C2 + 公共 | 126.5 | 136 | 49% |
| 同上，先不做 iOS 和 sdk-swift | 104.5 | 112.5 | 41% |
| 再加 Android 单端 C3 | 135 | 143 | 52% |
| 再加撤回（不含 iOS） | 151 | 157.5 | 57% |

**v0.5 重算**（RS4 修订后）：完整 284.5；C1 + C2 + 公共 **145**（Frank 按 136 拍板）；先不做 iOS 121；再加 Android C3 151.5；再加撤回 166。

不含：评审和复审、Frank 的操作、法务；**PRD-admin 第 12 节材料的法务部分**（文档列只算清单草稿和隐私告知页）、P-21 恢复的成本、真机等待；国内厂商通道（每家单独算，CH-13）；病毒扫描、图片审核、手机号登录（短信）、客服重置（CH-01 的备选）。

---

## 19. 风险

| # | 风险 | 影响 | 对策 |
|---|---|---|---|
| R-1 | **三条丢号路径**：① 恢复码丢失后又忘记密码；② **密码被盗（用户名公开，撞库目标明显）：攻击者改密，受害者要在 7 天内用旧恢复码找回，过了就永久丢失，也没有客服**（RC2-02；RS4-01 修正后攻击者不能抢先作废旧码）；③ 恢复码被人拿到：7 天内能抢走账号（丢手机时旧码也还有效，而且旧码优先） | 用户永久丢号；被盗后没有人工救济 | 注册时明确告知；旧码保留 7 天且不能被新代作废；可信设备提醒和离线上线提示（RS4-07）；冷静期可以找回；内部小范围接受，面向公众前重新决定（CH-01）。**客服重置的风险不只是被社工：客服本人或 console-server 被攻破，就能以用户身份读全部单聊，所以第一批不做**（R-CS-01） |
| R-2 | 中国大陆 Android 第一批没有离线通知 | 若用户主要在大陆，第一批推送对他们无效 | 明确告知（F-APN-06）；CH-13 拍板 |
| R-3 | Apple Developer、Firebase 账号未到位 | C3 无法真机验收 | C3 开工前 Frank 操作（第 20.3 节） |
| R-4 | im-server 到 FCM、APNs 的网络受部署地影响 | 推送不通 | 部署地和推送一起定；未核实 |
| R-5 | MinIO 的公开分发方式已变化（仓库 `testenv-ci.sh` 注释写明"不再发布公开的镜像和二进制"，只能从源码构建） | 长期维护成本 | 代码只依赖 S3 接口；生产建议云厂商（CH-05） |
| R-6 | 预签名地址在 TTL 内无法收回；被移出前可以批量预取（每分钟 120 批，每批 50 个） | 撤回、退群后 2～5 分钟内仍可下载；TTL 通常只约束请求开始（未核实），已开始的慢速下载可以持续到完成（RS4-24） | 接受并写明；地址量进指标；可以改短 TTL；**私有文件域名不做 CDN 缓存** |
| R-7 | `modVer` 变化的会话仍要重拉已加载范围；老 SDK 仍是全部重拉 | 重连流量 | 只影响变化的会话；已加载范围每会话最多 200 条（原生恢复） |
| R-8 | **客户端生成缩略图：缩略图可以和原图完全不同；解码炸弹影响接收方** | 接收者看到违规内容而审核员只看原图；大群客户端卡死 | v0.1"只影响发送者自己"的判断是错的（R-CS-12）。服务端读真实宽高；审核员同时看两份；三端解码上限。仍接受：缩略图内容不校验 |
| R-9 | 第一批不扫病毒、不审图片 | 恶意文件、违规图片只能靠举报 | 内部 / 小范围接受；公众开放前重新决定 |
| R-10 | 单聊举报在 X-7 下只有元数据 | 单聊骚扰只能按次数判断 | CH-22 |
| R-11 | C2 上线到 C6 上线之间没有内容治理 | 这期间只有后台删除、禁言 | 不开放注册；AC-ACC-01-9 |
| R-12 | 撤回原文保留 30 天和用户的"撤回"预期不一致 | 隐私投诉 | 写进说明；CH-17；对用户的措辞用"不再显示"；Postgres 死元组和备份也要等到期（R-CS-34） |
| R-13 | 新增面大，三套 SDK 和三端都要改，CI 现在没有实际运行（PENDING P-21） | 回归靠本地 | 先恢复 CI |
| R-14 | 真机（P-13）、压测机（P-12）都没有；Android CI 没有模拟器、后台 CI 没有 Playwright（R-CQ 跑不了的 AC） | 推送和性能结论不能发布 | 明确标记只有开发机结果 |
| R-15 | 老 SDK 不认识 `convs.deleted`，删除的群显示"你已不在该群" | 只影响老版本 | 没有已发布版本的证据（第 21 节第 7 项），接受 |
| R-16 | 推送模块在 B6 要重写在线判断和合并 | 集群化返工 | 提前写明（第 15 节）；接口已隔开 |
| R-17 | 登录名就是公开 `userId`，群成员列表就是撞库目标 | 撞库 | F-ACC-02 的防护和新设备提醒；CH-02 |
| R-18 | 技术验证发现目标厂商缺必需能力（尤其 ETag 条件 Copy） | 换厂商或改设计 | 冻结前验证（第 21 节第 4 项） |
| R-19 | 新一代首次使用才生效：旧 refresh token 有效期变长；**受害者不再上线（卸载、换机没退出）时，攻击者转正后可以滑动到 365 天，不触发检测**（RS4-08） | 被盗后可能长期不被发现 | 指标、新 IP 前缀事件、"账号安全"的其他会话数量；用户可以"退出其他设备"或改密 |

---

## 20. 拍板结果

> **2026-11-06 Frank 回复"全部按建议"：20.1 的 13 项都按"建议"列定下，备选和影响保留作记录。** 这包括不可逆的 CH-02（`userId` 就是用户名）。"指定人"和"选厂商"这几项定的是做法，具体人选和厂商还没给，见 20.3。编号不复用，取消的列在 20.2。

### 20.0 3 个事实问题（不是取舍，只有 Frank 知道，**还没回答**）

"全部按建议"不能回答这 3 个问题。在 Frank 回答前，按下表最后一列的工作假设继续；假设不成立就按"影响"改。

| 问题 | 为什么要问 | 答"有"的影响 | 工作假设（不是事实） |
|---|---|---|---|
| 有没有已经发给真实用户的 App 版本（商店、TestFlight、安装包）？ | 文档只看到 Android `versionCode = 1`，没有直接证据 | "不设最低版本"和 R-15 要重新看：老版本的群会被误显示"你已不在该群" | 没有 |
| 有没有生产环境和真实用户数据？ | 决定迁移 | F-ACC-07 规则 2 写的"演示和导入用户不能变成账号"要加迁移方案；已有 `Alice`、`alice` 这样的用户要处理 | 没有 |
| 目标用户主要在哪里，用什么手机？ | CH-13 | 大陆 Android 用户多，CH-13 的建议就不合适 | 未知。C3 开工前必须回答，C1、C2 不受影响 |

### 20.1 Frank 的决定（2026-11-06，全部按"建议"）

| # | 问题 | 建议 | 备选 | 影响 | 是否可逆 |
|---|---|---|---|---|---|
| **CH-27** | 第一批全部做（拍板时约 275.5 人日，v0.5 重算 284.5），还是先做一部分？ | **先做 C1 + C2 + 公共（拍板时约 136 人日，49%；v0.5 重算约 145），C1 结束后按实际用时校准，再决定后面**。C3→C4→C5→C6 的顺序不变 | ① 完整第一批（275.5）；② 同建议，先不做 iOS（112.5，iOS 成本只是推迟）；③ 再加 Android 推送（143）；④ 再加撤回（157.5，不含 iOS） | **不做的部分**：离线通知（App 不打开就不知道有消息）、撤回引用 @、置顶免打扰删除、敏感词和举报。**会丢失**：第 1.3 节的端到端演示暂时做不全；没有敏感词和举报，只能邀请制；撤回推迟也推迟了第 12 节门槛。不丢数据，后补没有迁移成本。数字都是低置信 | **可逆**，随时加回。这改变 Frank 已确认的范围，所以要他定 |
| **CH-23** | 这一批给内部小范围用，还是面向公众？（也决定注册是否开放，原 CH-03 并入） | **内部 / 小范围，邀请码注册**。公众开放单独决定，要先问法务（第 12.3 节、下面的清单）。用 App Store 或 TestFlight 发给真实用户算中间情形，也要问 | 直接面向公众：要加手机号实名、审核、备案（**工作量未估**） | 决定 CH-01、CH-13、CH-22 能不能按建议走。**没有回复就按内部小范围做**，不能开放注册。**即使内部，PRD-admin 第 12 节也适用**：隐私政策和用户协议写明并发布、个人信息保护影响评估（报告留 3 年以上）、后台操作规程和培训记录，完成后才能在生产打开隔离原文和群消息正文。这些材料的 owner 见 20.3，不在工作量里 | 内部→公众可以以后再开；公众开放后要收回很难 |
| **CH-13** | 中国大陆 Android 手机没有谷歌推送，要做小米、华为等厂商通道吗？ | **第一批不做**，只做谷歌推送。这些手机只有打开 App 才收到消息，App 里明确写。真要做时按实际用户机型挑 1～2 家 | ① 第一批做主要厂商：每家要单独企业开发者账号和审核，**工作量和时间未校准**；② 第三方聚合推送：要花钱，内容经过第三方 | **如果主要用户在大陆 Android，第一批的离线通知对他们没有用** | **可逆**，以后可以加，接口已隔开 |
| **CH-01** | 账号怎么注册、丢了怎么办？ | **用户名 + 密码 + 恢复码，没有客服重置**。内部小范围可以接受，也最安全：客服能重置就等于能看任何人的聊天 | ① **带双人审批和冷静期的客服重置**：不进主后台凭证，独立调用方，每天限额，客服发起、另一人批准，等 72 小时并通知已有设备（可以取消），重置后新设备看不到旧历史；约多 8 人日（未校准），还要定谁批准。② 手机号 + 验证码：找回简单，要短信服务和费用，涉及个人信息，约多 5 人日（未校准）；公众开放通常也要它 | **会有用户永久丢号**：① 恢复码和密码都丢了；② **密码被盗：攻击者改密后，受害者只有 7 天能用旧恢复码找回，过了就永久丢失，也没有人能帮**（第 19 节 R-1）。选 ①、② 都要改 F-ACC-05 | **可逆但救不回**：以后能加客服重置或手机号；已经丢的号救不回 |
| **CH-02** | **（不可逆）** 用户 ID 就用注册时的用户名吗？ | **用户名就是 `userId`**，小写，不能改 | 服务端生成随机 `userId`，用户名单独存：登录名不公开、以后能改名。**影响清单**：服务端注册和登录查找、`userSearch` 改按用户名（F-SOC-09 规则 1）、`users` 等响应带用户名、后台搜索（约 4）；三套 SDK 模型和显示名回退（约 1.5）；三端显示名回退、发起单聊、资料卡、@ 文本（F-USER-02、F-MEN-01，约 3）；后台页面（约 1）；测试（约 2）；文档（约 0.5）。**合计约 12 人日（8～16，低置信，未校准），不是 v0.1 写的 3 人日** | **选了之后基本改不回来**：`userId` 已写进会话 ID（`c2c:a:b`）、每条消息的发送者、好友和群表、日志，以后要改就是全库迁移。选建议的代价：用户名永远不能改；登录名公开（群成员列表就是撞库目标）；注销后用户名仍留在协议数据里，可能是真名 | **不可逆**。现在选备选约 12 人日，上线后再改是全库迁移 |
| **CH-26** | 网页版关掉标签页后要重新输密码，可以吗？ | **可以，第一批沿用**（D-UI-10）：token 不写硬盘，一次网页漏洞不会长期冒用 | 加"记住我"：体验好，token 要长期存在浏览器里，要改存储和 XSS 防护 | 决定网页版是"临时使用"还是"常驻"。Android、iOS 不受影响，登录一次可以保持 90 天 | **可逆** |
| **CH-05** | 文件存在哪？要花钱 | **生产用云厂商对象存储，厂商由 Frank 定**；开发测试用 MinIO | 生产也自建 MinIO：不付云费用，要自己负责磁盘、备份、升级（R-5）；小规模内部可以 | **用量**（1000 注册用户，都未校准）：假设每天约 12.5 GB，180 天约 2.3 TB；假设每个文件平均下载 3 次，出口约 37.5 GB/天、1.1 TB/月。**配额上限**：1000 人 × 2 GiB = 每天 2 TiB，是假设用量的约 160 倍，所以有全局用量告警。**金额未核实**（没有查价目）。厂商要满足第 21 节的必需能力，冻结前验证 | **可逆，有成本**：代码只依赖 S3 接口不用改，存量文件要复制 |
| **CH-10** | 聊天文件留多久？影响费用 | **180 天**，到期显示"已过期"，消息文字还在 | ① 不过期：存储一直涨；② 30 天：省钱但常常看不到旧文件 | 决定长期存储费用；法务可能有要求 | **部分**：配置随时改，改长只对还没删的有效，**已删除的文件救不回** |
| **CH-11** | 手机通知默认显示内容吗？ | **默认不显示发送者和内容**，只有"你收到一条新消息"，用户可以自己打开。角标数字保留 | ① 默认显示：体验好，内容默认经过苹果和谷歌；② 通知到手机后 App 自己取内容再显示（内容不经过第三方）：要多一个 iOS 扩展和接口，成功率不保证，**第一批不建议** | 选建议：默认通知信息少。用户打开详情后，发送方的内容也经过第三方，隐私说明要写 | **可逆**；已经发给第三方的内容收不回 |
| **CH-12** | 苹果、谷歌推送的账号和证书归谁？ | **归 Frank 的账号**，按 20.3 的步骤操作；AI 不接触账号，只拿服务器上的文件路径 | 公司主体账号：长期更合适，要主体资料 | C3 不做这些就没法真机验收。Apple 开发者账号要付费（**费用未核实**） | **可逆，迁移主体的成本未核实** |
| **CH-17** | 用户撤回的消息，平台还留吗？ | **加密留 30 天**，只能走审核流程看，到期删除；受害者可以在撤回提示上举报 | ① 立即删除：隐私最好，骚扰者发完撤回就没证据；**撤回这一项就不触发隔离门槛**，但后台删除和 C6 证据仍然要；② 留 7 天：折中，门槛同建议 | 用户以为撤回就没了，实际留 30 天。**选建议：PRD-admin 第 12 节 ②③④（隐私政策发布、影响评估、操作规程和培训）完成前，C4 不能在生产打开**（开发不受影响）；隐私说明入口见 F-ACC-10。材料的 owner 见 20.3 | **部分**：可以随时改；已留的和备份里的按原期限；选立即删除后，已删的证据救不回 |
| **CH-19** | 敏感词词库谁提供、谁维护？ | **Frank 指定一个人**负责内容来源、版权、更新。第一批在后台手工维护，仓库不放真实词 | 买第三方词库或内容审核服务：要花钱，内容可能发到第三方 | **没有词库，C6 上线也拦不了任何东西**。AI 不应该编造词库 | **可逆** |
| **CH-22** | 举报谁来处理？单聊举报能看内容吗？ | **`moderator` 在后台人工处理，Frank 指定人**，不自动处罚。**单聊举报法务确认前只留元数据，不留内容**（沿用 X-7）。群消息证据（含正文）**保留 180 天**，要先满足 PRD-admin 第 12 节 | 放开"举报人主动提交的那几条单聊"：举报才真正有用，要法务确认，改 X-7 口径，要有举报人知情提示和生效时间 | 没有处理人，举报队列没人看。选建议：单聊骚扰基本无法判断内容（R-10），之后撤回的单聊没有证据。**建议尽快问法务** | **部分**：只留元数据后，以前的内容补不回来；放开后，存过的内容收不回 |

### 20.1a 给法务的问题清单（CH-23，Frank 转交，RC2-27）

PRD-admin 第 12 节的 4 个问题（"宿主 App"现在就是天听本身，第 4 个委托处理关系要按天听重新看）：
1. 单聊、群聊正文是否允许内部人员查看，前提条件是什么。
2. 隐私政策和用户协议怎么写。
3. 是否必须做个人信息保护影响评估，由谁做。
4. 是否存在委托处理关系（对象存储、推送通道），要不要签协议。

第一批新增的默认值（都已写进规则，法务不同意就改配置）：
5. 注销后消息保留、`userId` 不能抹除，可以吗（F-ACC-08 规则 5）。
6. 撤回原文保留 30 天（CH-17）；举报证据含正文保留 180 天，比撤回长（CH-22）。
7. `account_events` 保留 180 天（含 IP 前缀）；`moderation_hits` 保留 90 天；文件 180 天（CH-10）。
8. 用户名是公开的、不可改，可能是真名（CH-02）。
9. 详情模式内容经过苹果、谷歌，数据出境怎么看（CH-11）。
10. 什么情况算"公众"：邀请制 + TestFlight 算不算；要不要实名、备案、增值电信许可。
11. 日志和审计的法定留存期。

### 20.2 不用 Frank 决定（已定，想改可以说）

| 原编号 | 问题 | 决定 | 理由 |
|---|---|---|---|
| CH-03 | 注册开放方式 | 并入 CH-23 | 和合规绑定 |
| CH-04 | token 和多设备 | 加 `sid`、refresh；不互踢；不做设备列表，提供"退出其他设备" | 不改 F-SYNC-01、AC-AUTH-02-5；被盗时有手段 |
| CH-06 | 访问方式 | 预签名：图片 120 秒、文件 300 秒，批量取地址 | 流量不走 IM 进程；代价是 R-6，还要配 CSP、CORS（R-CF-12） |
| CH-07 | 大小和类型 | 图片 20 MiB（压缩后算）、文件 100 MiB、头像 2 MiB、每人每天 2 GiB；配置可改（未校准） | 超过 100 MiB 要分片上传 |
| CH-08 | 缩略图 | 发送端生成，服务端读真实宽高不解码 | R-8 |
| CH-09 | EXIF、病毒 | 客户端清 EXIF，"文件"方式保留并提示；第一批不扫病毒 | 公众开放前随 CH-23 重新决定 |
| CH-14 | 免打扰和 @ | @ 我打断，@ 所有人不打断；**@ 我计入总未读和角标** | 产品默认值，后果可逆；否则出现"有通知、无角标" |
| CH-15 | 撤回时限 | 发送者 2 分钟（配置可改） | 常见产品的功能思路，只作参考（常见时长未核实）；后果可逆 |
| CH-16 | 群主、管理员撤回 | 可以，7 天内，显示是谁撤回的 | 后果可逆 |
| CH-18 | @ 所有人 | 只有群主、管理员；每群每小时 3 次、每天 10 次（未校准） | 配置可改 |
| CH-20 | 拦截还是替换 | 只拦截 | 替换让发送方以为发了原文，和幂等、撤回交互复杂 |
| CH-21 | 检查范围 | 查文本、文件名、昵称、用户名、群名、公告、附言；不查图片和文件内容 | 没有识别能力；公众开放前随 CH-23 重新决定 |
| CH-24 | 客户端 `custom` | 生产禁止，且按 `type` 收紧 `body` | 否则绕过敏感词；已核实 sdk-ts、`server/e2e` 不依赖（Kotlin、Swift 和 App 见第 21 节） |
| CH-25 | 注销账号 | 进第一批 P1，带 7 天冷静期 | 上架通常要求（未核实）；保留口径实现前随 CH-23 确认 |
| — | 老版本兼容 | 不设最低版本；线上帧 `deleted` + `recalled` 同时带，老客户端清除原文 | 取决于 20.0 的第 1 问 |
| — | 刷新令牌怎么轮换 | 新一代首次使用才生效（F-ACC-03 规则 4），不会因为弱网被踢；代价是旧令牌有效期变长，要安全复审 | 用户感知不到，代价写在 R-19 |
| — | 恢复码换代 | 旧码保留 7 天（F-ACC-05 规则 5） | 影响写在 CH-01 |
| — | 缩略图模型、预签名 TTL、并发闸门、collapse id、错误码 | 见对应规则 | 技术问题 |

### 20.3 Frank 需要亲自做的事（AI 做不了）

| 事 | 步骤 | 什么时候 |
|---|---|---|
| 事实问题 | 回复 20.0 的 3 个问题（拍板已完成） | 现在 |
| **技术验证门（冻结前必须过）** | 验证第 21 节的必需能力（含 ETag 条件 Copy、临时前缀生命周期）。**开发机不装软件，MinIO 只能在 CI 跑，而 CI 现在被 P-21 卡住，所以 Frank 要二选一：解决 GitHub 账单（P-21），或者开一个云厂商试用桶把凭证放在服务器文件里**。不做这一步就不能冻结 | 拍板后立刻 |
| 法务 | 把 20.1a 的清单转给法务，拿到书面回复 | 现在开始；公众开放或打开单聊正文前必须有 |
| **PRD-admin 第 12 节材料（owner：Frank，或他指定的人；AI 可以起草清单，不能代替）** | ② 隐私政策和用户协议（发布后配 `IM_PRIVACY_NOTICE_URL`、`IM_PRIVACY_NOTICE_VERSION`）；③ 个人信息保护影响评估；④ 后台操作规程和培训记录 | C4（选留存时）、C6 上生产前 |
| 对象存储（CH-05） | 选厂商，开通，建私有桶，开 CORS，配 HTTPS 域名，开服务端加密；把访问密钥放在服务器文件里（不发到聊天里） | 技术验证要用（可以先用试用账号）；C2 上线前 |
| APNs | ① 有 Apple Developer 账号（付费）；② 在开发者后台给 App ID `com.tianting.im` 开启 Push Notifications；③ 建 APNs Key，下载 `.p8`（只能下载一次）；④ 记下 Key ID、Team ID；⑤ 把 `.p8` 放到服务器，权限 0600 | C3 开工前 |
| FCM | ① 建 Firebase 项目；② 添加 Android 应用，包名 `com.tianting.app`；③ 下载 `google-services.json`（只在发布时注入，不进仓库）；④ 生成服务账号 JSON，放到服务器，权限 0600 | C3 开工前 |
| 服务器网络 | 确认部署地能访问 Apple 和 Google 的推送接口 | C3 开工前 |
| 词库（CH-19） | 指定负责人和来源 | C6 上线前 |
| 举报处理人（CH-22） | 指定，并在后台授予 `moderator` | C6 上线前 |
| 其他已有阻塞 | P-12（压测机）、P-13（真机和渗透测试）、P-15（CI 部署密钥）、P-21（GitHub 账单） | 越早越好，尤其 P-21 |

---

## 21. 没有核实的事情，和冻结前要做的事

**对象存储选型的必需能力（CH-05、R-CB-08）**：① 预签名 PUT、GET；② **限制上传大小**（PUT 签入 Content-Length，或 POST policy 的长度范围，至少一种）；③ CopyObject，**带 ETag 条件（`x-amz-copy-source-if-match`）**，Copy 后 ETag 可比较；Range GET 带 `If-Match`；④ GET 预签名支持 `response-content-type`、`response-content-disposition`；⑤ CORS；⑥ 禁止列举、私有桶（匿名和预签名禁止列举，服务端凭证可以按前缀列举）；**临时前缀的生命周期规则（1 天）**；⑦ 服务端加密；⑧ 第二个公开读桶和独立域名（头像）。**优先有**：配置 `nosniff`、关闭访问日志或脱敏。设计上**不依赖**厂商校验内容。

| # | 事情 | 现状 | 什么时候核实 |
|---|---|---|---|
| 1 | 中国的实名、内容安全、备案、数据出境的具体要求 | 不是法律专业判断 | 问法务（CH-23） |
| 2 | Apple Developer 费用、App 内注销的现行条款；APNs 请求头和错误码；FCM HTTP v1 细节；Android 通知 `tag` 替换 | 按记忆写。只有 FCM `collapse_key` 的 4 个上限是评审 R-CF-07 查过官方文档的，本人没有复查 | C3 开工时查文档 |
| 3 | MinIO 的许可和当前分发状况 | 只有仓库 `testenv-ci.sh` 的注释 | CH-05 |
| 4 | **技术验证门** | **MinIO 已通过**（[GATE-objstore](../review/GATE-objstore-2026-11-06.md)，本机临时构建，Frank 同意作为一次例外）。**云厂商没有验证，CH-05 选定后上线前跑 AC-FILE-90-2，不通过可能要改上传设计**（R-18）。生命周期的实际清除没有验证 | MinIO 已完成；云厂商上线前 |
| 4a | 真实环境下刷新响应丢失的频率；Argon2 的实际耗时 | 没有测 | C1 |
| 5 | 中国大陆服务器到 FCM 的连通性 | 没有测试 | 部署前 |
| 6 | iOS 进入后台时 `presence` 那一帧能否发出；H5 复制标签页是否复制 `sessionStorage`；浏览器和 Android 低版本的 HEIC 解码；API 29 以下保存图片的权限；`DownloadManager` 是否持久化地址 | 没有运行，评审也标未核实 | C2、C3 实现时 |
| 7 | **是否有已发布的老版本和生产用户** | 只看到 Android `versionCode = 1` 和测试环境，没有直接证据 | **已移到第 20.0 节，Frank 还没回答，先按工作假设** |
| 8 | Kotlin、Swift SDK 和各 App 是否依赖客户端发 `custom` | 复审：Android `src` 只有 androidTest 引用，`tianting-ios` 没有命中。本次搜到 Kotlin `Protocol.kt` 第 55 行、Swift `Protocol.swift` 第 17 行只是类型常量，**没有逐行确认没有发送路径** | C2 实现 F-MOD-02 规则 4a 时 |
| 9 | Swift Store 的合并 | 本次读了 `Store.swift` 第 116～130 行，和 TS、Kotlin 同样只接受"变成已删除"；其余没有逐行读 | C4 实现时 |
| 10 | 原生端本地库和文件缓存是否加密；下载到公共目录后其他 App 能否读取 | 没有查（Android `allowBackup=false` 已确认）。第一批新增文件缓存，建议复审专门看 | 复审 |
| 11 | 第 4.1、15 节和各 AC 里所有标"未校准"的数字 | 没有测试；第一批也没有 EP 测试机 | 实现后压测，P-12 |
| 12 | 工作量（第 18 节）；CH-02 备选的 12 人日；客服重置的 8 人日、手机号的 5 人日 | 没有基于实际开发校准 | C1 结束后回看 |
| 13 | Apple 账号主体迁移成本；对象存储价目 | 没有查 | CH-12、CH-05 |
| 14 | "新一代首次使用才生效"方案的安全性 | RS4～RS7 审过，修订见第 25～28 节，后两轮没有阻塞项。**RS7 的修订没有再审，整套方案都没有实现和运行** | C1 实现和测试 |
| 15 | 对象存储：云厂商的条件 Copy、加密下的 ETag、List 按前缀授权和分页、CORS 限定来源；MinIO 和厂商的生命周期实际清除；预签名 TTL 对慢速下载的约束 | MinIO 的其他项已实测；这些没有 | AC-FILE-90-2 |
| 16 | 浏览器：冻结页面的 `navigator.locks`、`sessionStorage` 的恢复和落盘（可能和 CH-26 的"关闭后要重新登录"不完全相符）；OkHttp、URLSession、网关的隐式重试；SQL 在 SQLite 和 Postgres 的兼容性 | 没有运行 | C1 实现时；和 CH-26 不符就报告 Frank |
| 17 | Postgres 下的账号事务（RS6-01）；云厂商 List 按 prefix 授权、删除和列举的一致性 | 仓库现在只有 SQLite，事务串行，推演不可达；AC-ACC-04-29 现在跑不了 | Postgres 实现时（B6）；验证门 |

**v0.1 未核实项中已关闭的**：老 SDK 收到撤回时的实际表现（原文留着、空气泡；已按代码核实 `store.ts`、`Store.kt`、`Store.swift`，用 F-RCL-01 规则 5a 解决）；已加载范围跨冷启动保留（Kotlin `restore`）。

---

## 22. 变更记录

| 版本 | 日期 | 内容 |
|---|---|---|
| v0.1 | 2026-11-06 | 初稿。按 Frank 确认的第一批范围起草，待独立评审 |
| v0.2 | 2026-11-06 | 按 REVIEW-chat-v0.1 四份独立评审修订（134 条，第 23 节）。关闭全部阻塞。账号改为不提供客服重置；上传改为临时键 + Copy；单 `fileId` 两个变体；待拍板收敛到 12 项；工作量重估为 268 人日；性能 AC 改绝对值；AC 增加里程碑标记 |
| v0.3 | 2026-11-06 | 按 RECHECK-chat-v0.2 修订（32 条，第 24 节）。刷新令牌改为新一代首次使用才生效；恢复码旧码保留 7 天；条件 Copy；单调合并；隐私告知（F-ACC-10）；第 20 节可以单独拍板，增加 20.0、20.1a、CH-27；修订记录改回真实状态；工作量 275.5 人日 |
| v0.4 | 2026-11-06 | 写入 Frank 的拍板：13 项全部按建议。AC 和规则不变（本来就按建议写）；只改状态、结论、里程碑前置条件和第 20 节。3 个事实问题没有回答，记工作假设 |
| v0.5 | 2026-11-06 | 按定向安全复审 RS4（25 条，第 25 节）修订：access 代号判定和重叠窗口；改密和退出其他设备换 `sid`；refresh token 带 MAC；凭证版本；找回只作废更新的代；`complete` 用租约和每次新随机键，元数据替换；`complete` 复查额度；合同测试补齐。删除"没有检测盲区"，写明盲区。工作量 284.5 人日 |
| v0.6 | 2026-11-06 | 按 RS5（21 条，第 26 节）修订：软拒绝只对最近一次转正失效的代生效；`recover` 不做版本判断，命中不受锁定限制；`recoverBefore` 按账号状态算，所有路径都带；refresh 独立密钥和 `kid`，恢复码改 SHA-256；代号严格单调；`final/{fileId}/{attemptId}` 让残留可以按前缀清理；租约释放；待确认代只按数量淘汰 |
| v0.7 | 2026-11-06 | 按 RS6（5 个严重项和若干一般项，第 27 节）修订：`recover` 事务先锁账号行；专用位按 `(userId, gen)` 公平调度，老码优先；最终前缀允许按 prefix 列举；头像走 `purging` 路径，公开键先写库；签发频率限制只在签发分支检查 |
| v1.0 | 2026-11-06 | 按 RS7（3 个严重项，第 28 节）修订：所有用密码的提交事务先锁账号行；成功次数上限只约束最新一代；任何路径进入 `purging` 都清公开对象。写入 MinIO 技术验证结果。**冻结**。排期按 CH-27 只有 C1、C2 和公共部分 |

---

## 23. v0.1 → v0.2 修订记录（134 条，REVIEW-chat-v0.1）

**v0.3 改回真实状态（RC2-17）**：已改 111、部分 22、不采纳 1。v0.2 曾写"已改 125、部分 8、严重项全部改完"，**这不成立**。阻塞 9 条：已改 6、部分 3（正文都改了，遗留在 v0.3 处理）。严重 64 条：已改 55、部分 9。"部分"都写明了遗留。现状断言都重新核对过代码（`store.ts`、`Store.kt`、`Store.swift`、`gateway.go`、`logic.go`、`config.go`、`app.go`）和冻结文档。

### 23.1 评审之间不一致时的判断

**refresh 宽限期（R-CS-05、R-CB-10、R-CQ-05）**
- 分歧：R-CS-05 要"签发新的一对，同一会话只有一条有效链，宽限期调短"；R-CB-10 给两种，"再轮换并让上次的新 token 作废"或 HMAC 派生以便重算；R-CQ-05 要"第一次返回的 R2 也不失效"。
- v0.2 的判断：再轮换，R2 作废，宽限期 30 秒。**这个判断 v0.3 被 RC2-01 推翻**：响应丢失超过 30 秒仍会误吊销，v0.2 写的"新 token 先落盘已经解决了进程被杀的情况"**是错的**（只覆盖响应到达之后），已删除。现方案见 F-ACC-03 规则 4 和第 24 节。单链的要求（R-CS-05）保留。

**老 SDK 收到撤回（R-CB-05、R-CF-04）**
- 分歧：R-CB-05 只要帧上同时带两个标志；R-CF-04 还要同时发 `msgDeleted`，并列为待拍板，或者确认没有已发布版本后删掉 AC。
- 判断：**不让 Frank 拍板，技术上定掉**。下行帧同时带两个标志，事件只有一个（`msgDeleted` 加可选 `recalled`），不新增 `msgRecalled`。这样不用发两个事件、处理两次，隐私上最稳：老客户端清除原文。成本只有一个字段，所以不需要先确认有没有老版本。`minClientVersion` 不做。
- 已按代码核实：三套 SDK 的 `addServer` 只在 `deleted` 时更新已有消息。

**配额的扣减时机（R-CF-01、R-CS-21）**
- 分歧：R-CF-01 要 `complete` 才扣；R-CS-21 担心申请上传被滥用。
- 判断：两个都做。扣在 `complete`，防滥用靠频率和未完成数。v0.2 写的"互不冲突"不成立（RC2-09：弱网用户被锁），v0.3 改为只统计未过期的并加取消接口。

**20 ms 超时放行（R-CB-27、R-CS-30）**
- 分歧：一个建议删除，一个建议加告警、`open` 模式失败即拒绝。
- 判断：删除。Go 里打断不了同步匹配，要实现只能另起协程，匹配本身目标 2 ms。

**缩略图 ID（R-CB-03、R-CF-02、R-CQ-02、R-CS-29）**：一致，单 `fileId` + variant。

### 23.2 评审对 CH 项的意见

| 意见来源 | 处理 |
|---|---|
| CH-01（R-CS）遗漏"不提供客服重置" | 已改，作为建议方案 |
| CH-02（R-CB、R-CS）遗漏不可逆 | 已改，写在问题和影响里 |
| CH-03（R-CS）`open` 条件不够 | 已改（F-ACC-01 规则 2），CH-03 并入 CH-23 |
| CH-04（R-CS）遗漏"退出其他设备" | 已改（F-ACC-04 规则 6），转 20.2 |
| CH-05、CH-06（R-CB、R-CF、R-CS）必需能力、CSP、预取 | 已改（第 21 节、R-6、第 13.2 节）；CH-06 转 20.2 |
| CH-08、CH-09（R-CS）| 已改（R-8、F-FILE-05）；转 20.2 |
| CH-11（R-CF、R-CS）隐藏推送 + 端上取内容；角标 | 已改，列为备选，不进第一批 |
| CH-14（R-CF）@ 是否计入角标 | 已改，技术上定掉，转 20.2 |
| CH-17（R-CS）短保留选项；要先修 R-CS-16 | 已改 |
| CH-22（R-CB、R-CS）只存元数据；知情提示 | 已改，只存元数据作为默认 |
| CH-23（R-CS）中间情形；PRD-admin 第 12 节 | 已改 |
| CH-24（R-CB）方案不完整 | 已改（F-MOD-02 规则 4a），转 20.2 |
| CH-25（R-CS）冷静期 | 已改，转 20.2 |
| 遗漏项：老版本兼容（R-CF） | 技术上定掉（20.2）。是否有老版本在用，第 21 节第 7 项问 Frank |
| 遗漏项：H5 关标签页（R-CF-25） | CH-26 |

### 23.3 逐条

| 编号 | 级别 | 处理 | 落实或理由 |
|---|---|---|---|
| R-CB-01 | 阻塞 | 已改 | `users` 不加唯一约束；`accounts.username_lower` 唯一；只有注册接口不区分大小写。F-ACC-01 规则 4、AC-ACC-01-5、AC-CHAT-90-6。已核实 C-18、TC-USER-01-16、`sqlite.go` 第 498、508 行 |
| R-CB-02 | 阻塞 | 已改 | `deleted` 覆盖单聊和群；SDK 物理清除 `seq < minSeq`。F-CVS-03 规则 6、AC-CVS-03-10。"群带 `hidden`"不采用（占 500 上限） |
| R-CB-03 | 阻塞 | 已改 | 单 `fileId` + variant，`complete` 校验两个变体，图片必须有缩略图。F-FILE-01 规则 1、3 |
| R-CB-04 | 严重 | 已改 | Store 合并规则改为覆盖。F-RCL-03 规则 2a、AC-QUO-02-6 |
| R-CB-05 | 严重 | 已改 | 线上同时带 `deleted`、`recalled`。见 23.1。AC-CHAT-90-7 |
| R-CB-06 | 严重 | 已改 | 新开关 `IM_MSG_ACTIONS_ENABLED`、`IM_CONV_SETTINGS_ENABLED`；隔离加密器从 V13 拆出。第 4.1 节、AC-RCL-01-10、AC-CHAT-90-2 |
| R-CB-07 | 严重 | 已改 | 绑定和 @ 计数进 `SaveMessage` 事务，撤回不退；`purging` 状态；加锁顺序。第 4.3 节、F-FILE-06 规则 2、AC-FILE-06-7、06-8、AC-MEN-02-9 |
| R-CB-08 | 严重 | 已改 | PUT 到临时键，`complete` 校验后 Copy。F-FILE-01 规则 3、4，第 21 节必需能力，技术验证提到冻结前 |
| R-CB-09 | 严重 | 已改 | 头像先私有后 Copy 到公开前缀。F-ACC-06 规则 4、AC-ACC-06-7 |
| R-CB-10 | 严重 | 已改 | `sid.secret`；宽限期再轮换。见 23.1。F-ACC-03 规则 3、4 |
| R-CB-11 | 严重 | 已改 | 只对 1006、4408；延迟 30 秒；全局限速。F-APN-02 规则 4、AC-APN-02-6 |
| R-CB-12 | 严重 | 已改 | 队列单位是消息；批量查询；大群不带角标。F-APN-02 规则 7、AC-APN-02-12、AC-APN-04-5 |
| R-CB-13 | 严重 | 已改 | `deleted_at` 单独表示；重新入群语义。F-CVS-03 规则 2、6a，AC-CVS-03-11、03-12 |
| R-CB-14 | 严重 | 已改 | `POST /v1/files/urls`。F-FILE-04 规则 1、AC-FILE-04-9 |
| R-CB-15 | 严重 | 已改 | `users.deleted_at`；分两步；列入口。F-ACC-08 规则 3、4，AC-ACC-08-5 |
| R-CB-16 | 严重 | 已改 | `delivery.Presence`；计数存库；任务单节点；第 15 节如实写 B6 要重写。第 11.4、15 节 |
| R-CB-17 | 一般 | 已改 | 和 R-CS-02 合并用 `credentials_changed_at`（不递增 `tv`，否则当前设备也被踢）；连接入表后复查 `sid`。F-ACC-03 规则 2、11 |
| R-CB-18 | 一般 | 已改 | 有账号的用户 `/demo/login` 拒绝。F-ACC-07 规则 2、AC-ACC-07-2 |
| R-CB-19 | 一般 | 已改 | IP 限流覆盖全部模式；邀请码失败上限。F-ACC-01 规则 7、AC-ACC-01-10、01-14 |
| R-CB-20 | 一般 | 已改 | 第 6～10 步失败退还预扣。第 4.3 节、AC-MOD-02-12、02-13 |
| R-CB-21 | 一般 | 已改 | `Validate` 和 Schema 按 `type` 互斥。F-MOD-02 规则 4a、AC-FILE-02-15 |
| R-CB-22 | 一般 | 已改 | collapse id 不透明；替换只在最后一条时发；内存 LRU。F-APN-02 规则 6、F-APN-03 规则 4。FCM 细节仍未核实（第 21 节） |
| R-CB-23 | 一般 | 已改 | 待退出补发；发送前复查；新定时任务；设置移到 `device_sessions`。F-ACC-04 规则 1、F-APN-01 规则 2、4、第 11.4 节 |
| R-CB-24 | 一般 | 已改 | 只显示"[文件]"。F-FILE-03 规则 5 |
| R-CB-25 | 一般 | 已改 | 闸门只用于算哈希的路径，排队有超时，第 15 节写内存预算。F-ACC-01 异常 |
| R-CB-26 | 一般 | 已改 | `presence` 改成普通带 `id` 的命令（比改 gateway 特判简单，和评审建议的做法不同）；字段改名 `notifyOff` |
| R-CB-27 | 一般 | 已改 | 逐条：AC-APN-02-8、MEN-02-3、CHAT-90-1 标开发机参考；ACC-02-3 交替采样；FILE-01-8 改断言；FILE-90 真实存储；RCL-01-6 写检查点；真实钟规则（第 4.4 节）；20 ms 超时取消 |
| R-CB-28 | 一般 | 已改 | 第 18 节加文档列，C1、C3 服务端和 C2、C4 SDK 上调 |
| R-CB-29 | 建议 | 已改 | `mod_ver`。F-RCL-03 规则 2、AC-RCL-03-11。已核实跨冷启动保留 |
| R-CF-01 | 阻塞 | 部分 | 新增 F-FILE-07。额度改为 `complete` 时扣（F-FILE-01 规则 6）。**遗留：弱网被未完成数锁住（RC2-09）、文件被清理后的路径（RC2-23），v0.3 已修** |
| R-CF-02 | 阻塞 | 已改 | 删除 `thumbFileId`，同 R-CB-03 |
| R-CF-03 | 阻塞 | 已改 | 文本插入公开昵称，绝不用备注。F-MEN-01 规则 4、AC-MEN-01-7 |
| R-CF-04 | 严重 | 已改 | 选方案 A 并简化：只发带 `recalled` 的 `msgDeleted`，不再发两个事件。`minClientVersion` 不做。见 23.1 |
| R-CF-05 | 严重 | 已改 | 同 R-CB-04。AC-QUO-02-6 |
| R-CF-06 | 严重 | 已改 | `auth` 后第一帧上报，收到前原生按后台。F-APN-02 规则 3、AC-APN-02-13、02-15。iOS 那一帧能否发出未实测（第 21 节） |
| R-CF-07 | 严重 | 已改 | 分通道；不用 FCM `collapse_key`；AC 到请求头；替换降级为尽力而为。F-APN-02 规则 6、AC-APN-03-6 |
| R-CF-08 | 严重 | 已改 | 退出时注销令牌并补发 logout；回前台对比授权；令牌刷新缓存。F-ACC-04 规则 1、F-APN-01 规则 5a、AC-APN-01-10、01-11 |
| R-CF-09 | 严重 | 部分 | 只有 4019 回登录；先落盘；`BroadcastChannel`；H5 短 TTL。F-ACC-03 规则 5、6、9，AC-ACC-03-11～14。响应丢失超过 30 秒后重试仍按重放处理，见 23.1。**遗留：第 2 点没有解决，理由是错的（RC2-01），v0.3 换方案** |
| R-CF-10 | 严重 | 已改 | `recovery_acked_at`；确认前不持久化；一个按钮；找回后的 kicked 原因。F-ACC-05 规则 1、3。客服入口取消（第一批没有客服重置） |
| R-CF-11 | 严重 | 已改 | 构建开关是上限；Session `mode`；老 Session 迁移；配置失败。F-ACC-07 规则 4、5，AC-ACC-03-18、07-7～9 |
| R-CF-12 | 严重 | 已改 | 第 4.2 节、第 13.2 节工程配置、第 16.1 节、AC-FILE-03-10、F-FILE-01 规则 10 |
| R-CF-13 | 严重 | 已改 | 原生 App 内下载；H5 走浏览器下载且不验进度；签入文件名。F-FILE-03 规则 3、4，AC-FILE-03-5、03-7 |
| R-CF-14 | 严重 | 已改 | 有矩阵、压缩后算大小、方向、失败处理。GIF 第一批只有 H5 播放，原生显示首帧：加依赖不值得进第一批；动图 webp、apng 按静态处理 |
| R-CF-15 | 严重 | 已改 | 批量接口；缓存键 `fileId + variant`。AC-FILE-04-10 |
| R-CF-16 | 一般 | 已改 | 中间模型、触发条件、结构化草稿、`groupMembers.keyword`。F-MEN-01 规则 5、6，AC-MEN-01-8～12 |
| R-CF-17 | 一般 | 部分 | 选评审给的第二种：200 条以内翻页跳转，更远提示翻页。不做 `loadAround`，第一批不值得改 Store 的连续假设 |
| R-CF-18 | 一般 | 已改 | 收到事件后节流刷新 `convs`（`msgDeleted` 是同一帧发给全群，带不了每人的值）。F-MEN-03 规则 1、AC-MEN-03-7 |
| R-CF-19 | 一般 | 已改 | 同 R-CB-02 |
| R-CF-20 | 一般 | 已改 | @ 我计入角标（技术上定掉）；前缀优先级；第 245 行进第 16 节；`.badge` 授权。F-CVS-02 规则 4、F-MEN-03 规则 2 |
| R-CF-21 | 一般 | 已改 | 客户端第一批只举报单条，协议保留数组。F-MOD-03 规则 2 |
| R-CF-22 | 一般 | 已改 | 第 13.4 节、AC-CHAT-90-9、AC-ACC-01-18。SHA-256 取消，顺带解决 WebCrypto 问题。表述问题表的 8 条：FILE-03-5、02-10、02-9、CVS-01-6、APN-01-5、ACC-01-12、FILE-04-4、RCL-01-8 都已改，管理员菜单见 F-RCL-02 规则 1b |
| R-CF-23 | 严重 | 已改 | `chat-sync.json`、`account-session.json`、HTTP Schema、假 HTTP、README。第 13.1 节、AC-CHAT-90-10、90-15 |
| R-CF-24 | 严重 | 已改 | 第 18 节重估，SDK + 客户端 130 人日；图片处理放 App 层 |
| R-CF-25 | 建议 | 已改 | CH-26 |
| R-CF-26 | 建议 | 已改 | `send` 保留加参数；离线推送统一 `deviceNotification*`。第 13.1 节 |
| R-CQ-01 | 阻塞 | 部分 | 第 4.5 节。**遗留：故障注入清单不全（RC2-24），v0.3 已补** |
| R-CQ-02 | 阻塞 | 已改 | 同 R-CB-03；AC-FILE-02-3、01-13 |
| R-CQ-03 | 严重 | 已改 | AC-ACC-01-8、01-16 |
| R-CQ-04 | 严重 | 已改 | 按 AC-CON-03-3 对齐，交替采样 ≤ 5%（协调人要求，不用评审的 20% / 5 ms） |
| R-CQ-05 | 严重 | 部分 | 不是一个 token 改写的问题，见 23.1：**第一次的 R2 作废**，不采用"R2 也不失效"（会有两条有效链）。AC-ACC-03-3。**实际否决了"R2 也不失效"，AC 当时仍有误吊销（RC2-01），v0.3 换方案** |
| R-CQ-06 | 严重 | 部分 | AC-FILE-03-3 用 `.bin`；`html` 进禁止名单。**v0.2 改错，和 F-FILE-05 矛盾（RC2-11），v0.3 已修** |
| R-CQ-07 | 严重 | 已改 | AC-FILE-01-2 改成服务端兜底；真实存储见 AC-FILE-90-1 |
| R-CQ-08 | 严重 | 已改 | AC-FILE-01-5 改 `complete`；`/uploads` 不探活 |
| R-CQ-09 | 严重 | 已改 | AC-APN-02-4、F-APN-02 规则 1 |
| R-CQ-10 | 严重 | 已改 | 第 4.5 节统一账号 token；AC-APN-01-7 |
| R-CQ-11 | 严重 | 部分 | AC-MEN-02-3、02-6。**v0.2 改错，每天 10 次照样挡住（RC2-12），v0.3 已修** |
| R-CQ-12 | 严重 | 已改 | F-RCL-02 规则 1a、AC-RCL-02-4、03-6 |
| R-CQ-13 | 严重 | 已改 | 同 R-CB-24；AC-FILE-03-8 |
| R-CQ-14 | 严重 | 已改 | 同 R-CB-01 |
| R-CQ-15 | 严重 | 已改 | 第 16.2 节按 AC 列出（已逐条核对原文）；AC-UI-09-7 的冲突通过改 F-RCL-01 规则 2 解决 |
| R-CQ-16 | 严重 | 已改 | refresh 一律 4019。F-ACC-03 规则 10、AC-ACC-03-6 |
| R-CQ-17 | 严重 | 已改 | [SDK 钟]；阈值 6 小时；AC-ACC-03-7、03-10 |
| R-CQ-18 | 严重 | 部分 | 全部 AC 加【C】标记；AC-ACC-04-4、06-5、FILE-04-5、APN-02-4 拆开；第 2.2 节补依赖。**v0.2 还有 3 条标错，且依赖 V13（RC2-13、RC2-15），v0.3 已修** |
| R-CQ-19 | 严重 | 已改 | AC-FILE-90-1、90-2 |
| R-CQ-20 | 严重 | 已改 | 假通道在 HTTP 层（第 4.5 节）；AC-APN-05-3、05-4 |
| R-CQ-21 | 严重 | 已改 | 占位配置；F-APN-01 规则 5b、AC-APN-01-12 |
| R-CQ-22 | 严重 | 已改 | 第 15 节判定方法；三个数字补 AC-CHAT-90-11～13 |
| R-CQ-23 | 严重 | 部分 | ★ 项全部补 AC。非 ★ 的约 87 条规则和 66 句异常没有逐条补：不少价值低或不可测，分发时在 TC 里覆盖，不写成 AC。**遗留：`/admin/files/{id}/fetch` 无授权和隔离分支没有 AC（RC2-24，v0.3 已补）；非 ★ 约 87 条规则和 66 句异常仍没有 AC** |
| R-CQ-24 | 一般 | 已改 | AC-MEN-03-1 |
| R-CQ-25 | 一般 | 已改 | AC-FILE-02-6 |
| R-CQ-26 | 一般 | 部分 | AC-FILE-01-7。**v0.2 期望值和规则对不上（RC2-25），v0.3 已修** |
| R-CQ-27 | 一般 | 已改 | AC-FILE-02-7、AC-MOD-04-15 |
| R-CQ-28 | 一般 | 已改 | AC-APN-03-1、03-6 |
| R-CQ-29 | 一般 | 已改 | 第 4.4 节哨兵法，相关 AC 已改 |
| R-CQ-30 | 一般 | 已改 | AC-APN-02-9 |
| R-CQ-31 | 一般 | 已改 | AC-APN-05-2 |
| R-CQ-32 | 一般 | 已改 | 时钟标记逐条修正；AC-FILE-04-4 用 `TTL=5` |
| R-CQ-33 | 一般 | 已改 | AC-FILE-01-7、AC-ACC-01-17、AC-MOD-03-13、AC-CVS-03-9、F-FILE-01 规则 3 |
| R-CQ-34 | 一般 | 已改 | 第 4.3 节第 6 步；AC-MOD-02-12、AC-MEN-02-8 |
| R-CQ-35 | 一般 | 已改 | 输入框不保留。F-MOD-02 规则 6、AC-MOD-02-9 |
| R-CQ-36 | 一般 | 已改 | AC-FILE-01-4、AC-CHAT-90-5、AC-FILE-02-5、AC-MOD-04-4 拆开或列明 |
| R-CQ-37 | 一般 | 已改 | 同 R-CF-10 |
| R-CQ-38 | 一般 | 已改 | AC-CHAT-90-8 |
| R-CQ-39 | 一般 | 已改 | ① 头像分支；② AC-FILE-02-4；③ 改引用 F-SOC-23（核实：ADMIN.md"群"规则确实写了管理接口移除群主时的转让，不是不存在，但注销应引 F-SOC-23）；④ 改 4030 + reason |
| R-CQ-40 | 建议 | 已改 | 第 4.5 节 |
| R-CQ-41 | 建议 | 已改 | AC-MEN-02-2、AC-RCL-01-2 |
| R-CQ-42 | 建议 | 已改 | AC-MOD-02-2 |
| R-CQ-43 | 建议 | 部分 | 写进第 16.1 节；`trace.py` 的两个局限不在 PRD 阶段改工具，分发时人工核对 |
| R-CQ-44 | 建议 | 已改 | AC-CHAT-90-10、90-14 |
| R-CS-01 | 阻塞 | 部分 | 按协调人要求：**第一批不提供客服重置**，`account.reset` 不进第一批。评审建议的双人审批和冷静期写进 CH-01 备选。F-ACC-05 规则 4、AC-ACC-05-9、R-1。**遗留：修法带来"密码被盗等于丢号"（RC2-02），v0.3 已写明并缓解，**风险仍在**** |
| R-CS-02 | 严重 | 已改 | `credentials_changed_at`。F-ACC-03 规则 11、AC-ACC-03-16 |
| R-CS-03 | 严重 | 已改 | 不变量；发送前复查；换主重置；详情 1 小时过期。F-APN-01 规则 2、4，AC-APN-01-3、01-8、01-9 |
| R-CS-04 | 严重 | 已改 | 隐藏模式每设备固定 id；AC 到请求头；角标写进 CH-11 |
| R-CS-05 | 严重 | 部分 | 见 23.1。**实际否决了"调短宽限期"；单链保留（RC2-01），v0.3 取消宽限期** |
| R-CS-06 | 严重 | 已改 | 渐进延迟；/64；新设备提醒；账号事件。F-ACC-02 规则 3、3b，AC-ACC-02-10、02-13。没有 MFA：第一批不做 |
| R-CS-07 | 严重 | 已改 | 同 R-CB-25；改密专用位；AC-ACC-01-16 |
| R-CS-08 | 严重 | 已改 | 全模式限流；失败上限；128 位；`maxUses` 上限。F-ACC-01 规则 7、F-ACC-09 规则 1 |
| R-CS-09 | 严重 | 部分 | 词库非空、确认项、新账号低配额都加了。人机验证不做：第一批默认内部小范围，公众开放前再补（写进第 12.3 节）。"举报处理人已配置"不进启动检查：技术上无法验证。**遗留：人机验证、处理人配置不做；低配额没有 AC（v0.3 已补）；邀请码可绕过，只有告警（RC2-18）** |
| R-CS-10 | 严重 | 部分 | 响应头由服务端固定、缩略图校验、头像隔离都已改。`nosniff` 能否配置未核实，列入优先能力。**遗留：`nosniff` 规则和 AC 打架、头像被放宽（RC2-19，v0.3 已统一）；`nosniff` 支持情况未核实** |
| R-CS-11 | 严重 | 已改 | 同 R-CB-08；AC-FILE-01-9 |
| R-CS-12 | 严重 | 已改 | 改写 R-8；真实宽高；两份；解码上限。AC-FILE-02-11、02-12、AC-MOD 规则 3 |
| R-CS-13 | 严重 | 已改 | 名单补全；文件名规范化；只提供保存。F-FILE-01 规则 7、F-FILE-05 规则 1、AC-FILE-05-2、03-9 |
| R-CS-14 | 严重 | 已改 | 文件名按正文。AC-MOD-04-11 |
| R-CS-15 | 严重 | 已改 | PRD-admin 第 12 节门槛写进第 12.3、17 节；单聊证据只存元数据；AC-MOD-03-10 |
| R-CS-16 | 严重 | 已改 | 撤回提示行可举报。F-MOD-03 规则 2a、AC-MOD-03-11 |
| R-CS-17 | 严重 | 已改 | 附言、用户名进范围；Default_Ignorable；去空白标点。F-MOD-01 规则 3、F-MOD-02 规则 2 |
| R-CS-18 | 一般 | 部分 | 改密时恢复码换新。重置码已取消，不适用。**v0.2 的修法造成 RC2-02，v0.3 改为旧码保留 7 天，丢手机的场景反而有 7 天风险，遗留写在 F-ACC-05 规则 5** |
| R-CS-19 | 一般 | 部分 | H5 refresh 24 小时、绝对上限 365 天、access 24 小时都加了。access 不到小时级：每次到期都要断开重连，24 小时是折中 |
| R-CS-20 | 一般 | 已改 | 不存在的用户名也累计；找回等时（AC-ACC-05-11）；计数存库。重置码已取消，不适用 |
| R-CS-21 | 一般 | 部分 | 申请频率、未完成数、全局用量告警、费用估算都加了。下载次数和流量上限不做：取地址已限制，第一批内部小范围，看实际流量再定 |
| R-CS-22 | 一般 | 已改 | 按用途分 TTL；`Referrer-Policy`；存储日志；SDK 日志 AC；取地址指标 |
| R-CS-23 | 一般 | 已改 | 服务端加密；文件名进隔离区；过期清除；AC-RCL-01-6、AC-FILE-06-10。`sha256` 不再存 |
| R-CS-24 | 一般 | 已改 | console-server 代理；CSP 不放宽。AC-MOD-04-12、04-13 |
| R-CS-25 | 一般 | 部分 | `readSeq` 校验、文件照常发放、审计、理由、`note` 按正文都已改。上下文不保存：扩大收集范围，和 X-7 的最小必要相反 |
| R-CS-26 | 一般 | 已改 | 生产默认拒绝启动。F-ACC-07 规则 3、AC-ACC-07-3 |
| R-CS-27 | 一般 | 部分 | 头像删除、数据表、冷静期都加了。`from` 用别名替换不做：`userId` 在会话 ID 和全部消息里，改写就是全库迁移，如实告知（CH-02） |
| R-CS-28 | 一般 | 已改 | `account_events`；特征串补全；失败只记哈希 |
| R-CS-29 | 一般 | 已改 | 同 R-CB-03 |
| R-CS-30 | 一般 | 已改 | 取消超时放行；启动加载失败就退出；打开 `custom` 进配置检查；词库加载失败要告警 |
| R-CS-31 | 建议 | 部分 | 加注册频率限制。持有证明和客户端校验归属不做：要先拿到令牌才能利用，风险低 |
| R-CS-32 | 建议 | 不采纳 | `clientMsgId` 下发给接收方是冻结规则（F-MSG-04 规则 4）；改了要动 AC-MSG-04-7 和三套 SDK 的合并；只有两边都改客户端才能利用，不是敏感词要防的场景。记入 F-MOD-02 规则 4b |
| R-CS-33 | 建议 | 已改 | F-ACC-01 规则 1 |
| R-CS-34 | 建议 | 部分 | AC 写明只验 SQLite，措辞沿用"不再显示"。Postgres 的 VACUUM 随 B6 处理 |
| R-CS-35 | 建议 | 已改 | F-ACC-03 规则 2；第 4.2 节 5a |

---

## 24. v0.2 → v0.3 修订记录（RECHECK-chat-v0.2，32 条）

RC2 共 32 条：已改 25、部分 6、不采纳 1。另外 4 行是 RECHECK 第七部分的冻结前事项。"部分"不是没改，是规则改了但有不能在文档里关闭的遗留。

**数字**：40 个功能点（新增 F-ACC-10）+ F-CHAT-90。AC 460 条。废弃编号见第 16.3 节（v0.3 新增废弃 AC-ACC-04-9、AC-ACC-05-4，都有取代编号）。修订记录第 23 节计数 111 / 22 / 1。待拍板 13 项。工作量 275.5 人日。

**切分数字**：RECHECK 第六部分的 126.5、104.5、135、151 在 v0.2 表上核对正确；v0.3 表重算是 136、112.5、143、157.5（第 18 节）。

**没有逐条复核的**：RECHECK 说明它只抽查了严重项，本次也只处理它指出的问题，没有重新逐条核对全部 AC 和第 16.2 节。

| 编号 | 级别 | 处理 | 落实或遗留 |
|---|---|---|---|
| RC2-01 | 阻塞 | 部分 | 采用"新一代首次使用才生效"（F-ACC-03 规则 4），理由和代价写在规则里；不再因响应丢失吊销；单链；重放吊销加指标、事件和用户提示（AC-ACC-03-19～26）；删除 v0.2 的错误理由（23.1）。**遗留：新方案没人做过安全复审，已列入冻结前（第 17 节、R-19）；真实环境响应丢失的频率未测** |
| RC2-02 | 阻塞 | 部分 | 旧码保留 7 天；找回优先于改密；可信设备提醒；冷静期内可以 `recover`（F-ACC-05 规则 5、5a，AC-ACC-04-13、05-12、05-16）；写进 R-1 和 CH-01。**遗留：风险没有消除，7 天内没发现就永久丢号；丢手机时旧码 7 天内有效；7 天未校准** |
| RC2-03 | 严重 | 已改 | 每代一行、每代自己的确认；ack 带 `gen`；修正登录响应。F-ACC-05 规则 1、AC-ACC-05-13～15 |
| RC2-04 | 严重 | 已改 | SDK 单飞、丢弃迟到响应；服务端条件更新。F-ACC-03 规则 4、5，AC-ACC-03-21～23 |
| RC2-05 | 严重 | 已改 | `delete/cancel` 改匿名；登录响应定义为 4030 `accountDeletionPending`；踢线原因统一 `accountDeleted`。F-ACC-08 规则 2、异常，AC-ACC-08-1、08-4 |
| RC2-06 | 严重 | 已改 | `logout` 接受 refresh token，任何一代都不算重放。F-ACC-04 规则 1、AC-ACC-04-14 |
| RC2-07 | 严重 | 部分 | Range 和 Copy 绑定 ETag，Copy 后再核对；进第 21 节必需能力；AC-FILE-01-15。**遗留：厂商是否支持未核实，假 S3 证明不了，等技术验证门** |
| RC2-08 | 严重 | 已改 | 全部变体成功后才删临时键；临时前缀生命周期规则改为必需；PUT 签入 Content-Length；写明允许超额 2 GiB。F-FILE-01 规则 1、3、3a、6，AC-FILE-01-10、01-18 |
| RC2-09 | 严重 | 已改 | 只统计 15 分钟内的 `pending`；`DELETE /v1/files/uploads/{id}`；单独 `reason`，SDK 不重试。23.1 的"互不冲突"已更正。AC-FILE-01-16、01-17 |
| RC2-10 | 严重 | 已改 | 改为单调合并。F-RCL-03 规则 2a、AC-RCL-03-13，进 `chat-sync.json` |
| RC2-11 | 严重 | 已改 | AC-FILE-03-3 改成前 64 KB 之后才有 HTML |
| RC2-12 | 严重 | 已改 | AC-MEN-02-3、AC-CHAT-90-1、AC-ACC-01-14、01-16 都改；闸门用阻塞钩子 |
| RC2-13 | 严重 | 已改 | AC-ACC-01-9 拆出 AC-ACC-01-19；AC-FILE-07-8 改用 4036；AC-FILE-06-10、06-11 拆出 AC-RCL-04-8、AC-CVS-03-15；`mod_ver` 和合并规则提前到 C2（AC-FILE-06-12）。第 2.2 节写明单独过关的前提 |
| RC2-14 | 严重 | 部分 | 【全部】改成累计范围表；开发机结果明确不算通过。**遗留：P-12 没有机器，性能 AC 现在都给不出发布结论** |
| RC2-15 | 严重 | 已改 | 第 4.1 节写明依赖；生产 `IM_MOD_ENABLED=1` 要求 V13；相关 AC 写明输入；`open` 条件增加；AC-MOD-01-10 |
| RC2-16 | 严重 | 部分 | CH-17、CH-23 写明门槛；新增 F-ACC-10 告知入口和 AC；owner 写进 20.3；隔离生产开启前要有隐私说明版本。**遗留：材料本身不存在，法务部分不在工作量里，Frank 要指定人** |
| RC2-17 | 一般 | 已改 | 第 23 节 14 条改回"部分"并写明遗留，计数改为 111 / 22 / 1，删除"严重项全部改完" |
| RC2-18 | 一般 | 部分 | 低配额补 AC-ACC-01-20；邀请总量进配置检查（AC-ACC-09-14）。**遗留：只告警不硬限制，公开传播大额邀请码仍等于开放注册** |
| RC2-19 | 一般 | 已改 | `nosniff` 统一为优先能力，AC 按厂商支持判断；头像恢复独立桶和独立域名，进必需能力 |
| RC2-20 | 一般 | 已改 | 4096 增加 client 和 409；4131 增加 client；4097 文案按时限；4292 按命令加文案 |
| RC2-21 | 一般 | 已改 | 后台序列化和下行分开（已核实 `admin/moderation.go` 第 623、647 行）；11.6 增加正文接口；AC-RCL-04-3 断言 `deleted=false` |
| RC2-22 | 一般 | 已改 | F-RCL-02 规则 1a 合成一张表，AC-RCL-02-4、03-6 对齐。v0.2 的"有人撤回了一条消息"取消 |
| RC2-23 | 一般 | 已改 | 4096 时有副本就重新上传。F-FILE-07 规则 4、AC-FILE-07-10 |
| RC2-24 | 一般 | 已改 | 第 4.5 节补故障注入和锁定开关；AC-ACC-03-27、AC-MOD-02-14、AC-MOD-04-16、04-17、AC-RCL-04-9 |
| RC2-25 | 一般 | 已改 | 规则增加去掉开头的点 |
| RC2-26 | 一般 | 已改 | AC-ACC-08-5 改绝对值 |
| RC2-27 | 一般 | 已改 | 20.1a 法务清单；CH-22 写明 180 天 |
| RC2-28 | 一般 | 已改 | 第 20.0 节 |
| RC2-29 | 一般 | 已改 | CH-02 列出影响清单，重估约 12 人日（8～16，低置信），不再用 3 人日 |
| RC2-30 | 建议 | 不采纳 | 不合并：合并要废号（协调人要求已有编号不改义），这几组 TC 可以共用一个用例，不增加实际工作 |
| RC2-31 | 建议 | 已改 | 邀请码失败按 24 小时窗口。F-ACC-01 规则 7、AC-ACC-01-21 |
| RC2-32 | 建议 | 已改 | `files` 增加 `thumb_mime`、`retain_until`、`evidence_until` |
| 七-3 | 冻结前 | 已改 | 必需能力和技术验证加 ETag 条件 Copy、生命周期规则（第 21 节） |
| 七-4 | 冻结前 | 已改 | 第 20.3 节写明依赖 P-21 或试用桶 |
| 七-5 | 冻结前 | 部分 | owner 写为 Frank 或他指定的人。**时间没有定，要 Frank 给** |
| 七-6 | 冻结前 | 未做 | 定向安全复审列入第 17 节，**还没做** |

---

## 25. v0.4 → v0.5 修订记录（RECHECK-chat-v0.4-security，25 条）

RS4 共 25 条。AI 对照 PRD 和 `gateway.go` 核实了 3 个阻塞项和吊销复查的代码引用；对象存储、浏览器行为都没有实测。"部分"是规则改了，但有不能在文档里关闭的遗留。**这些修订方案本身还没有人审过，冻结前要复核。**

| 条目 | 严重度 | 处理 | 落在 |
|---|---|---|---|
| RS4-01 | 阻塞 | 已改 | 找回只作废更新的代。F-ACC-05 规则 5，AC-ACC-04-13、05-17 |
| RS4-02 | 阻塞 | 已改 | access 代号判定表；转正时断开旧代连接。F-ACC-03 规则 4，AC-ACC-03-28～30 |
| RS4-03 | 阻塞 | 已改 | 改密、找回、退出其他设备换 `sid`；串行队列。F-ACC-04 规则 3、6，AC-ACC-04-2、10、15～17 |
| RS4-04 | 严重 | 部分 | 2 分钟重叠窗口，保留窗口内的 `pending`，软拒绝。F-ACC-03 规则 4，AC-ACC-03-31。**遗留：晚于窗口的重复请求仍会误吊销，频率未测** |
| RS4-05 | 严重 | 已改 | refresh token 带 MAC，`sid` 不进日志，刷新接口限流。F-ACC-03 规则 1、3，AC-ACC-03-32、33 |
| RS4-06 | 严重 | 已改 | `credentials_version` 条件提交。F-ACC-04 规则 8，AC-ACC-04-18～21 |
| RS4-07 | 严重 | 已改 | 踢线和 4019 按原因映射，带找回截止日期。F-ACC-03 规则 2、6，AC-ACC-03-34 |
| RS4-08 | 严重 | 部分 | 删除"没有检测盲区"，写明盲区；每次刷新比较 IP 前缀；其他会话数量。旧代 access 直接拒绝。AC-ACC-03-30、35。**遗留：受害者不再上线时不触发检测（R-19）** |
| RS4-09 | 一般 | 已改 | 失败关闭、周期复查。F-ACC-03 规则 2，AC-ACC-03-36。集群部分随 B6 |
| RS4-10 | 一般 | 已改 | F-ACC-03 规则 3、6，AC-ACC-03-37 |
| RS4-11 | 一般 | 已改 | F-ACC-02 规则 3，AC-ACC-04-22 |
| RS4-12 | 一般 | 已改 | 恢复码用 HMAC，未命中不占闸门。F-ACC-05 规则 1、2，AC-ACC-05-18。**AC-ACC-05-11 的等时只比较未命中的路径** |
| RS4-13 | 一般 | 部分 | `navigator.locks`，AC-ACC-03-14。**兼容性和 `sessionStorage` 落盘未核实（第 21 节第 16 项）** |
| RS4-14 | 一般 | 部分 | 12.2 写明，运维手册分发时建任务。还没有手册和 AC |
| RS4-15 | 建议 | 已改 | 12.2 写明，进部署说明 |
| RS4-16 | 严重 | 已改 | 租约 + 每次新随机键 + 不跳过变体。F-FILE-01 规则 3，AC-FILE-01-10、19、20 |
| RS4-17 | 严重 | 已改 | 签入 Content-Type，Copy 元数据替换。F-FILE-01 规则 1、3，AC-FILE-01-21。**厂商行为待验证门确认** |
| RS4-18 | 严重 | 部分 | AC-FILE-90-1 补齐，Copy 后按响应 ETag 核对，定义 412。**验证门本身还没跑** |
| RS4-19 | 一般 | 已改 | `complete` 复查额度，取消时删对象，告警含临时前缀。F-FILE-01 规则 6、6a，AC-FILE-01-18 |
| RS4-20 | 一般 | 已改 | F-FILE-03 规则 4，AC-FILE-01-25 |
| RS4-21 | 一般 | 已改 | F-FILE-01 规则 3b，AC-FILE-01-22 |
| RS4-22 | 一般 | 已改 | F-ACC-06 规则 4，AC-ACC-06-7、10 |
| RS4-23 | 一般 | 已改 | F-FILE-01 规则 3、6a，F-FILE-06 规则 2，第 11.3 节，AC-FILE-01-23、24 |
| RS4-24 | 建议 | 已改 | R-6，第 11.3 节 |
| RS4-25 | 建议 | 部分 | 补齐禁止名单；`doc`、`xls`、`ppt` 第一批放行并写明理由 |

---

## 26. v0.5 → v0.6 修订记录（RECHECK-chat-v0.5-security，21 条）

RS5 共 21 条，复核了 RS4 的修法。AI 核对了 `frames.go`、`token.go`、`errors.json` 的引用。RS4 状态更新：RS4-07 和 RS4-04 的路径随 RS5-03、RS5-21 ③ 修订。**这些修法还没有人审过，也没有实现和运行。**

| 条目 | 严重度 | 处理 | 落在 |
|---|---|---|---|
| RS5-01 | 阻塞 | 已改 | `overlapMinGen`；`retryAfterSeconds`；最多 3 次。F-ACC-03 规则 4、6，AC-ACC-03-38、39 |
| RS5-02 | 阻塞 | 已改 | `recover` 不做版本判断，命中不受限，独立专用位。F-ACC-04 规则 8、F-ACC-05 规则 2、F-ACC-01 异常，AC-ACC-05-3、04-23、24 |
| RS5-03 | 严重 | 已改 | `recoverBefore` 按账号状态算，`kicked` 增加字段。F-ACC-03 规则 2、6a，第 11.1 节，AC-ACC-04-5、25 |
| RS5-04 | 严重 | 已改 | refresh 独立密钥、`kid`、365 天退役；恢复码改 SHA-256。F-ACC-03 规则 3、F-ACC-05 规则 1，AC-ACC-03-46 |
| RS5-05 | 严重 | 已改 | `nextGen` 严格单调。F-ACC-03 规则 1，AC-ACC-03-40 |
| RS5-06 | 严重 | 已改 | `final/{fileId}/{attemptId}`，按前缀清理。F-FILE-01 规则 3、F-FILE-06 规则 7、8，AC-FILE-01-26。**遗留：残留最长留到文件保留期结束** |
| RS5-07 | 一般 | 已改 | 租约带 attempt，结束时释放；404 重读状态。AC-FILE-01-27 |
| RS5-08 | 一般 | 已改 | 复查比较代号。F-ACC-03 规则 2，AC-ACC-03-41 |
| RS5-09 | 一般 | 已改 | 出错用 1011，周期复查连续 3 次才断。AC-ACC-03-36、47 |
| RS5-10 | 一般 | 已改 | 先比较本地代号。F-ACC-03 规则 5、7，AC-ACC-03-42 |
| RS5-11 | 一般 | 已改 | `absoluteExpiresAt`。AC-ACC-03-43 |
| RS5-12 | 一般 | 已改 | 换 `sid` 后重新注册，设置随设备复制。F-ACC-04 规则 3，AC-ACC-04-26（C3） |
| RS5-13 | 一般 | 已改 | 确认前只放内存；`recoveryCodeUnacked`。F-ACC-05 规则 1，AC-ACC-05-19 |
| RS5-14 | 一般 | 已改 | 淘汰最旧；限流事件和提示。AC-ACC-03-3、28、45。**遗留：持旧值的人仍能让受害者退避，只能提醒，不能阻止** |
| RS5-15 | 一般 | 已改 | 30 秒超时，队列内不重新排队。AC-ACC-04-27 |
| RS5-16 | 一般 | 已改 | 取消的状态；声明字节上限。AC-FILE-01-28、29 |
| RS5-17 | 一般 | 已改 | `public_key`；重复绑定成功。F-ACC-06 规则 4，AC-ACC-06-11 |
| RS5-18 | 一般 | 已改 | 判定顺序；`logout` 两者之一有效。AC-ACC-03-44 |
| RS5-19 | 一般 | 已改 | 第 11.3 节 |
| RS5-20 | 建议 | 已改 | ① AC-ACC-03-29；② AC-ACC-05-6；③ 第 11.5 节；④ 第 12.2 节；⑤ `revoked_at`；⑥ 规则 3、4；⑦ 第 4.1 节；⑧ AC-ACC-04-16 |
| RS5-21 | 建议 | 部分 | ① AC-ACC-04-28；② 标签替换；③ 取消按年龄丢弃。**④ GIF 只能看前 64 KB，仍依赖客户端防护** |

---

## 27. v0.6 → v0.7 修订记录（RECHECK-chat-v0.6-security）

RS6 只审了 RS5-01～06 的修法，只展开阻塞和严重：没有阻塞项，5 个严重项。三轮安全复审的阻塞项是 3、2、0。**v0.7 的修法还没有人审过，也没有实现和运行**，放在对象存储技术验证门之后一起复核，因为验证结果可能还会改上传规则。

| 条目 | 严重度 | 处理 | 落在 |
|---|---|---|---|
| RS6-01 | 严重 | 已改（未运行） | `recover` 第一条语句锁账号行，加锁顺序。F-ACC-04 规则 8、第 4.3 节、AC-ACC-04-29。**只在 Postgres 下可达，这条 AC 现在跑不了** |
| RS6-02 | 严重 | 已改 | 专用位公平调度，老码优先，成功次数上限，查库前总量限流。F-ACC-05 规则 2，AC-ACC-04-30 |
| RS6-03 | 严重 | 已改 | 最终前缀允许按 prefix 列举，"禁止列举"限定为匿名和预签名。第 11.3 节、F-FILE-01 规则 9、第 21 节、AC-FILE-90-1。**厂商是否支持待验证门** |
| RS6-04 | 严重 | 已改 | 头像走 `purging` 路径；公开键先写库。F-ACC-06 规则 4、F-FILE-06 规则 2，AC-ACC-06-12 |
| RS6-05 | 严重 | 已改 | 判定顺序第 ⑥ 步。F-ACC-03 规则 4，AC-ACC-03-48 |
| 一般：`recoverBefore` | 一般 | 已改 | 排除已用和已作废的码。F-ACC-03 规则 6a |
| 一般：密钥台账 | 一般 | 部分 | F-ACC-03 规则 3。应急操作写进运维手册，分发时建任务 |
| 一般：恢复码规范化 | 一般 | 已改 | F-ACC-05 规则 2 |
| 一般：未命中要查库 | 一般 | 已改 | 查库前的每 IP 总量限流。F-ACC-05 规则 2 |
| 一般：`file_purge` 和进行中的 `complete` | 一般 | 已改 | 只匹配租约已过期的行。F-FILE-06 规则 2 |
| 一般：桶版本控制 | 一般 | 已改 | F-FILE-06 规则 8 |
| 一般：保存页被杀后看到"如果不是本人操作" | 一般 | 不改 | 磁盘上留的是已吊销的旧 token，提示措辞偏重但方向安全，不影响账号 |

---

## 28. v0.7 → v1.0 修订记录（RECHECK-chat-v0.7-security）

RS7 审了 RS6-01～05 的修法，没有阻塞项，3 个严重项。安全复审四轮的阻塞项是 3、2、0、0。RS7 认为改完这 3 项可以冻结，不需要再审一轮完整的；Frank 2026-11-06 同意"验证通过、复核符合预期就冻结"。**这些修法没有再审，也没有实现和运行**。

| 条目 | 严重度 | 处理 | 落在 |
|---|---|---|---|
| RS7-01 | 严重 | 已改（未运行） | F-ACC-04 规则 8，AC-ACC-04-29。**只在 Postgres 下可达，这条 AC 现在跑不了** |
| RS7-02 | 严重 | 已改 | F-ACC-05 规则 2，AC-ACC-04-31 |
| RS7-03 | 严重 | 已改 | F-FILE-06 规则 7，AC-ACC-06-13 |
| 第 21 节口径 | 一般 | 已改 | 云厂商验证放到上线前（AC-FILE-90-2），和 AC 一致 |
| `file_purge` 的 NULL 条件 | 一般 | 已改 | F-FILE-06 规则 2 |
| 取消和进行中的 `complete` | 一般 | 已改 | F-FILE-01 规则 6a |
| 换头像计时、绑回旧文件 | 一般 | 已改 | `avatar_replaced_at`，AC-ACC-06-11 |
| 公开键撞键 | 一般 | 已改 | 绑定失败不立即删除。F-ACC-06 规则 4 |
| `recover` 专用位没有数值 | 一般 | 已改 | 第 4.1 节，未校准 |
| 多实例公平性、NAT | 一般 | 部分 | 写明单实例和部署说明；集群随 B6 |
| 工具的 `tmp/` 列举权限 | 建议 | 已改 | 工具 README 写明只用于清理，第 11.3 节不给 im-server |
