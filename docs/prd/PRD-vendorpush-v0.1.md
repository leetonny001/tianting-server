# PRD：天听国内厂商推送通道（华为、荣耀、小米、OPPO、vivo 直连）

| 项 | 内容 |
|---|---|
| 版本 | **v0.1（草稿）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-10 |
| 状态 | **草稿，待评审**。没有独立评审，没有拍板；流程见 [process/README](../process/README.md)。**排期由 Frank 在 C3 之后定**（[QUEUE](../process/QUEUE.md) Q27） |
| 依据 | PENDING P-42：Frank 2026-11-10 决定"C3 后加厂商通道"，目标用户主要在中国大陆，直连五家，不用第三方聚合 |
| 对照 | [PRD-chat v1.5](PRD-chat-v1.5.md) 第 7 节（F-APN-01～06）、第 18、20、21 节；[backend-chat](../tasks/backend-chat.md) BE-406、BE-450～460 和文末第 34～46 条；[frontend-chat](../tasks/frontend-chat.md) FE-650～656 |
| 阅读提示 | **厂商资料的可信度分三级，逐条标注**：「已读」= 直接读到官方页面原文；「摘要」= 官方页面是前端渲染读不到，结论来自搜索引擎对该官方页面的索引摘要，**算半核实**；「未核实」= 没查到或页面读不到。**没有一条是凭记忆写成事实的**。标"未校准"的数字没有测过。AC 写法沿用 PRD-chat 第 4.4 节，批次标【V1】（`trace.py` 现有的是 C1～C6、M3、M4、SDK、P1、C，不冲突）；**本草稿不改 `trace.py`**，冻结并决定排期后再加 |

---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 做什么 | C3 之后加一个批次【V1】：服务端直连五家厂商推送，Android 端按品牌只注册一个通道。复用 C3 的令牌表、在线判断、合并、隐私、队列、重试和假通道的做法，**只新增 5 个 `push.Provider` 和客户端接入** |
| 最大的风险不在代码 | ① **OPPO 和 vivo 的正式推送要求 App 在它们的应用商店上架**（OPPO「已读」、vivo「摘要」），上架一般要软著和 App 备案，**这和 CH-23"内部小范围、邀请制"冲突**（VP-4）；② **OPPO 要求企业开发者**（「已读」）；③ IM 类消息要先向各家申请分类权益或私信通道（华为 5 个工作日、小米 5 个工作日、vivo 约 5 个工作日），**没批下来之前按营销类算，每台设备每天只有 1～2 条**，等于不可用 |
| 隐私 | 默认隐藏模式不变（F-APN-03）：通知只有"天听 / 你收到一条新消息"，请求里没有 `convId`、`userId`、`msgId`。**详情模式的内容会经过国内厂商的服务器**，而且小米的私信要套审核过的模板（VP-6）。五个 SDK 都会收集设备信息，隐私政策要逐个写明（第 9 节，给法务） |
| 客户端新依赖 | 五个厂商 SDK，**引入要 Frank 同意**（第 8.2 节）。小米、vivo、OPPO 官方只给 AAR 文件下载，没有找到官方 Maven（VP-5） |
| 服务端新依赖 | 没有。鉴权是 OAuth 客户端模式或 MD5 / SHA-256 签名，Go 标准库就够 |
| 工作量 | 约 **45.5 人日，低置信**（第 14 节）；不含 Frank 的账号、审核等待、上架、法务、真机 |
| Frank 要做的事 | 第 16 节：五家开发者账号和应用、IM 分类和通道申请、密钥放服务器、各品牌真机；**账号和审核现在就可以并行开始**，审核周期是 V1 最长的等待 |
| 拍板 | 8 项（第 17 节），最关键的是 VP-2（做哪几家、什么顺序）、VP-3（账号主体）、VP-4（OPPO / vivo 上架） |

---

## 1. 背景与目标

### 1.1 背景
- PRD-chat C3 只做 APNs 和 FCM。中国大陆的 Android 手机基本没有 Google Play 服务，**C3 之后这些手机仍然收不到离线通知**（F-APN-06，CH-13）。
- Frank 2026-11-10：目标用户主要在中国大陆（回答了 PRD-chat 第 20.0 节第 3 问），决定 C3 后加厂商通道，直连华为、荣耀、小米、OPPO、vivo，不用第三方聚合（P-42）。
- C3 的服务端已经用 `push.Provider` 接口隔开通道（F-APN-05 规则 5），设计时就预留了厂商通道。

### 1.2 目标
1. 五个品牌的大陆版 Android 手机，App 退到后台或被杀后，能在系统通知栏收到离线通知。
2. 隐私口径、合并、在线判断、重试、令牌失效清理和 C3 完全一致，不另起一套规则。
3. 每家都有 HTTP 层假通道，开发和后端验收不依赖 Frank 的账号。

### 1.3 成功标准
- 五个品牌各一台真机（大陆版系统），App 被杀后对方发消息，10 秒内收到通知（未校准），锁屏只显示固定文案（AC-VND-09-1）。
- 通知按"即时通讯"分类展示（有声音、横幅，不是静默），说明 IM 分类权益已经生效（AC-VND-09-4）。
- 后端 AC 全部用假通道通过；`send` 的 ack 不受厂商通道影响（AC-VND-03-5）。

---

## 2. 范围

### 2.1 做

| 项 | 内容 |
|---|---|
| 服务端 | 5 个 `push.Provider`：华为（HMS Push Kit）、荣耀（Honor Push）、小米（MiPush）、OPPO（OPPO PUSH）、vivo（vivo PUSH）。鉴权、发送、错误映射、配额熔断、配置检查 |
| 协议 | `deviceNotificationRegister.provider` 增加 `hms`、`honor`、`mi`、`oppo`、`vivo`；`GET /v1/config` 告诉客户端服务端启用了哪些通道 |
| sdk-kotlin | 通道选择接口、provider 字段 |
| Android | 按品牌选通道、五个厂商 SDK 接入、隐私同意后才初始化、设置页状态 |
| 测试替身 | 每家一个 HTTP 层假通道（第 12 节） |
| 部署 | 出站主机、密钥文件、运维手册增补 |
| 文档 | 隐私政策的 SDK 清单（给法务的草稿）、厂商审核材料清单 |

### 2.2 不做（和理由）

| 不做 | 理由 |
|---|---|
| 第三方聚合推送（个推、极光等） | Frank 决定直连（P-42）。聚合要花钱，通知内容和设备标识多经过一家第三方（CH-13 备选 ②） |
| 后台常驻长连接 | 耗电，大陆厂商系统多数会杀后台（F-APN-06 规则 1 已定）；小米官方也说非 MIUI 系统上应用要驻留后台才能收到，说明系统通道才是正路（小米 D1545「已读」） |
| 其他小厂商（魅族、三星大陆版、联想、中兴、努比亚等） | 每家要单独账号、审核和 SDK，成本和五家之一相当；先看 V1 之后的实际机型分布再决定。**这些手机继续走 F-APN-06 的"不支持离线通知"文案**（有 Play 服务的走 FCM） |
| HarmonyOS NEXT（纯血鸿蒙）设备 | 华为官方迁移文档说升级到 NEXT 后原 APK 应用要换成鸿蒙应用（H12「已读」），天听没有鸿蒙版，**不在本批**。推送 NEXT 要用 Push Kit v3 接口（H1「已读」） |
| 透传（数据）消息、VoIP 消息 | 只用"通知栏消息"，由系统代为显示，App 被杀也能显示；透传要 App 进程活着 |
| 海外版系统的厂商通道 | 小米海外要按设备地区换域名（D1696「已读」），其他几家海外端点未查；海外版手机多数有 Play 服务，走 FCM（VP-1） |
| iOS、H5、PC | 不受影响 |
| 免打扰、撤回替换通知 | 跟随 C4、C5 的口径（PRD-chat F-MEN-03、F-APN-03 规则 4），厂商通道只是多几个出口 |

### 2.3 和 PRD-chat C3 的关系

**复用，不改规则**（只在 provider 枚举和通道细节上扩展）：

| C3 的部分 | V1 的做法 |
|---|---|
| F-APN-01 令牌表 `device_push`、绑定 `sid`、同一令牌只属一个用户、`sid` 吊销同事务删令牌、发送前复查、日志只记哈希前缀、每小时 10 次注册 | 原样复用。`provider` 列多 5 个取值；唯一键改成 `(provider, token_hash)`（不同厂商的令牌可能碰巧同串，F-VND-02 规则 3） |
| F-APN-02 在线判断（presence、45 秒窗口、断线补推）、5 秒合并、队列和 worker、批量查询、重试次数和过期时间 | 原样复用。重试间隔和"可重试"的判断按各家错误码（第 6 节） |
| F-APN-03 隐藏 / 详情模式、文案、请求里不含 ID、详情只在开启的设备 | 原样复用；小米私信要套模板（F-VND-03 规则 4） |
| F-APN-04 点击、角标 | 隐藏模式点开进会话列表，详情模式 deep link；Android 角标不承诺（F-APN-04 规则 3） |
| F-APN-05 `push.Provider`、密钥文件 0600、生产缺密钥拒绝启动、配置检查 | 原样复用，每家一组配置（第 11 节） |
| F-APN-06 没有 FCM 的 Android | **改**：五个品牌改走厂商通道；其余品牌保留原文案。冻结时走 PRD-chat 变更（第 15 节） |
| 第 4.5 节假 APNs、假 FCM（BE-406） | 同样的写法，每家一个（第 12 节） |

---

## 3. 用户故事

| # | 角色 | 故事 |
|---|---|---|
| US-1 | 小米手机用户 | App 被系统清掉后，朋友给我发消息，我在通知栏看到"天听 / 你收到一条新消息"，有声音，点开进入会话列表 |
| US-2 | 华为手机用户 | 我在设置里打开"显示发送者和内容"，确认内容会经过华为的服务器后，通知显示发送者和摘要，点开直接进会话 |
| US-3 | 魅族手机用户 | 设置页明确告诉我"此设备暂不支持离线通知"，打开 App 才能收到消息 |
| US-4 | Frank | 某家通道的密钥失效或配额用完，消息收发不受影响，配置检查和指标能看出是哪一家、什么原因 |

---

## 4. 五家厂商资料（2026-11-10 核对）

### 4.0 来源编号

只用各家官方域名的页面。**没有下载任何文件**（SDK、样例工程、PDF 都没有下载；只能下载才能读的标"未核实"）。

| 编号 | 厂商 | URL | 读取情况 |
|---|---|---|---|
| H1 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/android-server-dev-0000001050040110 | 已读 |
| H2 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/oauth2-0000001212610981 | 已读 |
| H3 | 华为 | https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/push-jwt-token | 已读 |
| H4 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/android-client-dev-0000001050042041 | 已读 |
| H5 | 华为 | https://developer.huawei.com/consumer/cn/doc/hmscore-guides/faq-0000001050042183 | 已读 |
| H6 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/message-classification-0000001149358835 | 已读 |
| H7 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/message-restriction-description-0000001361648361 | 已读 |
| H8 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-References/https-send-api-0000001050986197 | 已读 |
| H9 | 华为 | https://developer.huawei.com/consumer/cn/doc/HMSCore-Guides/android-integrating-sdk-0000001050040084 | 已读 |
| H10 | 华为 | https://developer.huawei.com/consumer/en/doc/HMSCore-Guides/personal-data-0000001050166427 | 已读 |
| H11 | 华为 | https://developer.huawei.com/consumer/en/doc/HMSCore-Guides/push-sdk-data-security-0000002293109909 | 已读（中文版 503） |
| H12 | 华为 | https://developer.huawei.com/consumer/en/doc/harmonyos-guides/app-data-migration-overview | 已读 |
| R1 | 荣耀 | https://developer.honor.com/cn/docs/11002/guides/cloud-base-api | 摘要 |
| R2 | 荣耀 | https://developer.honor.com/cn/forum/topicdetail/topicid-3634628190076928 | 摘要（官方论坛助手回复） |
| R3 | 荣耀 | https://agreement.itsec.honor.com/asm/agrFile/getHtmlFile?agrNo=1114&country=cn&branchId=0&langCode=zh-cn | 已读（推送系统服务的用户声明，2026-05-01 版） |
| R4 | 荣耀 | https://developer.honor.com/cn/kitdoc?category=base&docId=app-registration.md&kitId=11002&navigation=guides | 摘要 |
| D1559 等 | 小米 | `https://dev.mi.com/xiaomihyperos/documentation/detail?pId=` 加编号：1559（国内发送）、1696（海外）、1542（开通）、1544（客户端）、1545（FAQ）、2321（消息分类新规）、1656、2315、2314（分类和 channel）、1560（错误码）、1535、1534（隐私）、1731（应用商店账号） | 已读 |
| O1 等 | OPPO | 见下方 OPPO 来源说明 | 论坛官方帖已读（多为 2020 年旧版）；文档中心页面只有摘要 |
| V362 等 | vivo | `https://dev.vivo.com.cn/documentCenter/doc/` 加编号：362（服务端接口）、232（客户端）、174（regId，wap 版）、359、735、c7facbddba34455aa7bd0dd99767801e（分类）、368（错误码）、365（FAQ）、541、652（隐私）、281（wap 版，开通流程）、12（上架） | **全部是摘要**：文档页前端渲染，读不到原文 |

OPPO 来源说明（「O tid=N」指 `https://open.oppomobile.com/bbs/forum.php?mod=viewthread&tid=N`，OPPO 开放平台论坛的官方帖）：
- 「已读」：tid=1925（鉴权）、1927（单推）、1924（消息体字段）、6343（新消息分类 FAQ）、1906（开通推送服务）；1918（客户端 SDK 2.1.0，读的是 `https://open.oppomobile.com/bbs/forum.php?mod=viewthread&action=printable&tid=1918`）。多为 2020 年的旧版说明。
- 「摘要」：tid=1914（通知渠道适配，没有打开过）；文档中心 `https://open.oppomobile.com/new/developmentDoc/info?id=` 11256（鉴权）、11229（合规）等，`https://open.oppomobile.com/documentation/page/info?id=` 11239（regId）、13190、13216（配额）等，页面前端渲染只拿到空壳。

### 4.1 服务端接口和出站主机

| 厂商 | 鉴权 | 发送 | 出站主机 | 可信度 |
|---|---|---|---|---|
| 华为 | OAuth 客户端模式：`POST https://oauth-login.cloud.huawei.com/oauth2/v3/token`，`grant_type=client_credentials`、`client_id`、`client_secret`，`expires_in` 3600 秒，有效期内复用；申请频控 5 分钟 1000 个（H2）。另有 JWT 服务账号方式（PS256，`aud` 是同一 token 地址，H3） | `POST https://push-api.cloud.huawei.com/v1/{clientid}/messages:send`，`Authorization: Bearer`；`message.android.notification{title, body, click_action}`、`message.token[]`（H1、H8） | `oauth-login.cloud.huawei.com`、`push-api.cloud.huawei.com` | 已读。v1、v2 页面原文说"仅支持给 HarmonyOS 3.x/4.x 推送"、NEXT 用 v3；**这句没提 Android，Android 用 v1 是按 Android 服务端开发指南（H1）推断** |
| 荣耀 | `POST https://iam.developer.honor.com/auth/token`，form，`grant_type=client_credentials`、`client_id`、`client_secret`（R1）；有效期**未核实** | `POST https://push-api.cloud.honor.com/api/v1/{APPID}/sendMessage`，`Authorization: Bearer`；通知放 `android.notification`（title、body、clickAction）（R1） | `iam.developer.honor.com`、`push-api.cloud.honor.com` | 摘要。完整字段表**未核实** |
| 小米 | 请求头 `Authorization: key=<AppSecret>`（`key` 小写）（D1559、D1542） | `POST https://api.xmpush.xiaomi.com/v3/message/regid`（也有 v2）；`registration_id`（逗号分隔，最多 1000）、`restricted_package_name`、`pass_through=0`、`title`（<50 字）、`description`（<128 字）、`notify_type`、`notify_id`、`time_to_live`、`extra.channel_id`、`payload`（urlencode）（D1559）。**没有单独的鉴权接口** | `api.xmpush.xiaomi.com`（国内）；海外按地区 `*-api.xmpush.global.xiaomi.com`（D1696，不在本批） | 已读 |
| OPPO | `POST /server/v1/auth`：`app_key`、`timestamp`（毫秒，允许 10 分钟误差）、`sign = SHA256(appkey + timestamp + mastersecret)`；返回 `auth_token`，有效期 24 小时（O tid=1925）。过期后 10 分钟新旧都能用（摘要 11256）。**sign 用十六进制还是 Base64 未核实** | `POST /server/v1/message/notification/unicast`，form-urlencoded，`message` 是 JSON 字符串：`target_type=2`（registration_id）、`target_value`、`notification{title, content, channel_id, …}`（O tid=1927）。新分类字段 `category`、`notify_level`（摘要，传 `notify_level` 必须同时传 `category`，要求 OS13 以上） | **主机未核实**：论坛帖没写，官方旧版 PDF 的搜索摘要是 `api.push.oppomobile.com`（PDF 没有下载） | 接口路径已读（旧版）；主机未核实 |
| vivo | `POST /message/auth`：`sign = md5(appId + appKey + timestamp + appSecret)` 小写，`timestamp` 13 位毫秒，误差约 10 分钟；`authToken` 默认 1 天，可同时多个，官方建议集中缓存、1～2 小时刷新；每 app 每秒 10 次（V362） | `POST /message/send`，请求头 `authToken`，JSON：必填 `appId`、`regId`、`title`、`content`、`skipType`、`category`；`requestId`（≤64 字符）、`notifyId`、`pushMode`（V362） | `api-push.vivo.com.cn` | 摘要 |

### 4.2 令牌（regId / token）

| 厂商 | 获取 | 变化和失效 | 可信度 |
|---|---|---|---|
| 华为 | `HmsInstanceId.getInstance(ctx).getToken(appId, "HCM")`，子线程调用，**返回值可能为空**，必须同时实现 `HmsMessageService.onNewToken`（H4） | "Token 没有有效期，但在某些场景中会变化"：清数据、卸载重装、恢复出厂、`deleteToken` 后再申请；换华为帐号不变；禁止频繁申请；官方不建议直接调 `deleteToken`（H4、H5） | 已读 |
| 荣耀 | **未核实**（客户端集成页读不到） | 官方只说"每个设备上的每个应用的 token 唯一"（R1） | 未核实 |
| 小米 | `MiPushClient.registerPush`，在 `PushMessageReceiver.onReceiveRegisterResult` / `onCommandResult` 取 regId；也可 `getRegId()`（D1544） | 不卸载重装、不清数据就不变；`disablePush`/`enablePush` 不换；失效：`unregisterPush`、MIUI 上卸载、**超过 30 天没有和推送服务器建立长连接**（D1545） | 已读 |
| OPPO | `HeytapPushManager.register(context, appKey, appSecret, callback)`，`onRegister(responseCode, registerID)`，0 成功；`isSupportPush()` 判断支持（O tid=1918，旧版 2.1.0） | 卸载、应用注销、刷机、**30 天没联网**失效，回执事件 `regid_invalid`（摘要 11239） | 获取已读（旧版）；失效是摘要 |
| vivo | `PushClient.getInstance(ctx).turnOnPush`，回调 state=0 后 `getRegId`，否则拿到 null；首次订阅或变化时 `onReceiveRegId`；**官方建议每次冷启动调 `turnOnPush` 并把 regId 同步到服务端**；`turnOnPush`/`turnOffPush` 每天 100 次（V232） | `turnOffPush`、卸载、清数据、**超过 14 天未建立长连接**、重新订阅后仍用旧 regId（V174） | 摘要 |

结论：C3 的"距上次注册超过 30 天重新注册一次"（F-APN-01 规则 5a）对 vivo 不够（14 天），见 F-VND-06 规则 2。

### 4.3 消息分类、频控和配额（"即时通讯 / 私信"类）

| 厂商 | IM 怎么分类 | 申请 | IM 的配额 | 没分类时 | 可信度 |
|---|---|---|---|---|---|
| 华为 | 服务与通讯类，`android.category = "IM"`：点对点聊天、私信、群聊；**不包括**未关注人的私信、官方号或商家的批量私信和广告（H6） | **自分类权益**：AGC"增长 > 推送服务 > 配置"申请，审核 5 个工作日（H6） | 服务与通讯类不限量，锁屏、铃声、振动（H6、H7） | 资讯营销类是静默通知，只在通知中心；单设备每天新闻类 5 条（要许可证）、其他 2 条；超限仍返回 80000000，要靠回执 256 识别；测试消息 `target_user_type=1` 每天 500 条；只适用于中国境内（H7）。**没申请权益时消息默认归哪类，未核实** | 已读 |
| 荣耀 | **未核实**（官方《消息分类标准》《推送数量管理细则》页读不到） | 未核实 | 未核实 | 未核实 | 未核实 |
| 小米 | 私信 → "聊天消息"组（好友聊天、群组聊天、音视频通话等），共 75 个类目（D2321） | 在运营平台"消息分类管理 - channel 列表"新建 channel，5 个工作日审核，每应用最多 30 个（D2321、D2314）。**2026-07-01 起私信必须带 `extra.channel_id`、`extra.template_id`、`extra.template_param`**（JSON 字符串，变量值都是字符串，最多 9 个变量），已接入的应用 2026-12-31 前完成模板改造（D2321） | 私信每天不限量；QPS 按开启通知设备数分 500～3000 档（D2321、D1656）。用私信通道发营销会处罚，第二次违规停私信 3 天 | **默认通道每台设备每天 1 条**；公信每天总量 = 开启通知设备数（不足 1 万按 1 万）× 2、单设备 5 条（D2321） | 已读 |
| OPPO | 通讯与服务类（原私信，含 IM）不限量；社交通讯类可申请锁屏、横幅、铃声、振动强提醒（O tid=6343） | **不限量权益怎么申请：未核实**（13216 正文读不到） | 不限量（O tid=6343）；单用户每天最多接收 2000 条，超出回执 `user_daily_limit`（摘要 13190） | 内容与营销类每设备每天新闻 5 条、其他 2 条，只能 7:00～23:30 发（摘要 13190）；2024-07-29 起不是从 OPPO 软件商店下载的应用不再提供公信（摘要） | 分类已读；配额是摘要 |
| vivo | 系统消息，`category = "IM"`（必须大写）；官方建议只传 `category` 不传 `classification`（V359、Vc7fa…） | IM 直接传 `category=IM`（其余系统消息要接消息模板）；可走工单申请"即时消息不限量"，先接入二级分类，约 5 个工作日，推送权限为"正式"后生效（V735） | 系统消息配额 = 通知开启有效用户数 × 20（不足 1 万按 1 万）；不限量要申请（V735） | 按运营消息：单设备每天 2 条，只能 7:00～23:00；测试推送 `pushMode=1` 每应用每天 1000 条（V735） | 摘要 |

### 4.4 折叠 / 替换、通知渠道、有效期

| 厂商 | 替换同一条通知 | 通知渠道 | 有效期 | 可信度 |
|---|---|---|---|---|
| 华为 | `notification.tag` 同 tag 覆盖；`notification.notify_id` 同 id 替换（-1 或不填系统生成）；`android.collapse_key` 管离线缓存（0 只留最新，-1 全缓存，1～100 分组）（H8） | `notification.channel_id`（Android O 以上）；`importance` LOW 静默、NORMAL 有声有振，实际提醒受分类影响（H8） | `ttl` 默认 86400 秒，最长 15 天（H8）；**字段格式（带不带 `s`）未核实** | 已读 |
| 荣耀 | `tag`（官方推荐）、`group`、`notifyId` 三选一，同时设置时 group > tag > notifyId（R1） | 未核实 | 未核实 | 摘要 |
| 小米 | 相同 `notify_id` 覆盖，0～2147483647 整数（D1559、D1545） | `extra.channel_id` 必填；channel 要先在后台申请或调 `/v1/channel/add`，发过消息的 channel 不能改不能删；channel 必须和模板同属一个分类（D1696、D2321、D2314） | `time_to_live`；离线缓存最长私信 10 天、公信 1 天（D1559）；**单位未核实** | 已读 |
| OPPO | **未核实**：官方帖只有 `app_message_id`（同 ID 的单推只发一次，是去重不是替换）（O tid=1927） | `channel_id` Android 9 起必填（O tid=1924）；设置后不能改不能删（摘要 tid=1914）；旧版私信 channel 要先在 App 建好再到管理台登记（摘要） | **未核实** | 部分已读 |
| vivo | `notifyId` 1～2147483647，相同的覆盖，不传系统生成；`isPushSupport` 判断设备是否支持覆盖（V735、V362） | 只查到 Android 8 以上由通知渠道控制提醒方式，`notifyType` 不生效；系统是否按分类自动建渠道、能否自定义，**未核实** | **未核实** | 摘要 |

### 4.5 错误码

| 厂商 | 令牌失效（删令牌） | 鉴权 | 可重试 | 配额 / 频控 | 其他不重试 | 可信度 |
|---|---|---|---|---|---|---|
| 华为 | `80300007`（全部令牌无效）；`80100000` 部分成功，失败的在 `illegal_tokens` 里 | HTTP 401；`80200001` 鉴权错误；`80200003` access token 过期；`80200005` JWT 过期；`80600003` OAuth 失败（ID / Secret 错）；`80300002` 无权限（未开通或令牌和应用不匹配） | HTTP 500、502、503（流控）；`81000001` 内部错误 | 资讯营销超限不报错（见 4.3） | `80300008` 超 4096 字节；`80300010` 令牌数超限；`80100016` 内容含敏感信息 | 已读（H8） |
| 荣耀 | 未核实（《下行消息响应码》页读不到） | 未核实 | 未核实 | 未核实 | 未核实 | 未核实 |
| 小米 | `66007`（regId 格式非法）、`10029`（不合法的设备）。**`65003` 是设备不在线，不删**。官方建议用回执 `extra.callback.type=16` 取无效设备，feedback 接口已下线 | `21301`、`21302`、`22007`、`22000` | 官方没有标注；`1`、`10001`、`10002`、`10003`、`65029` 描述是服务端内部错误 | `200001` 超配额；`200002`、`10027` 超 QPS | — | 已读（D1560、D1656） |
| OPPO | `10000` = Invalid Registration_id（O tid=1927）；回执 `regid_invalid`、Feedback 接口（一次最多 500 个）（摘要） | **未核实** | **未核实** | `user_daily_limit` 回执（摘要）；返回码**未核实** | — | 部分已读 |
| vivo | `10302` regId 不合法，`invalidUser.status` 区分不存在还是卸载 / 解订阅 | `10000`（token 缺失、错误或过期，重新鉴权）、`10206` sign 错、`10205` appId 不存在、`10093` 鉴权接口超频 | `10072` 推送过快，可延迟重试 | `10070` 运营消息超配额、`10073` 系统消息超配额（当天别再发）；`10071` 超出发送时段 | `10096` category 无效、`10097` category 和 classification 不对应 | 摘要（V368） |

### 4.6 客户端 SDK

| 厂商 | SDK | 获取方式 | 配置文件 | 设备要求 | 可信度 |
|---|---|---|---|---|---|
| 华为 | `com.huawei.hms:push`（页面示例 6.11.0.300） | 华为 Maven `https://developer.huawei.com/repo/` | `agconnect-services.json` + Gradle 插件 `com.huawei.agconnect:agcp` | 依赖 HMS Core（华为机预装；非华为机要另装，"可能存在兼容性问题"） | 已读（H9、H5） |
| 荣耀 | **未核实**（坐标、仓库、配置文件都没读到） | 未核实 | 未核实 | Magic 6.0 以上、Magic 4.0 / 5.0 最新版支持，Android 版本无要求；**2023-11-30 以后上市的荣耀设备、荣耀 100 及以后只支持荣耀推送**；`HonorPushClient` 判断支持（R2） | 摘要 |
| 小米 | MiPush Android SDK（5.0.1 起 AAR） | **官网下载 AAR 放 `libs/`，用 flatDir**；官方页面没有 Maven 坐标 | 无单独文件，`registerPush` 传 AppId、AppKey | 最低 Android SDK 19；HyperOS 要求未核实 | 已读（D1544、D1545）；下载页没有访问 |
| OPPO | heytap push SDK（旧版 `com.heytap.msp-push-2.1.0.aar`） | 旧版只能手动下载放 `libs/`；3.x 的 Maven 坐标**未核实** | `register` 传 appKey、appSecret | minSdk 19；ColorOS 3.1+ 的 OPPO、一加 5/5T 以后、全部 realme（旧版说明） | 旧版已读；新版未核实 |
| vivo | vpush clientSDK（示例 `vpush_clientSDK_v4.1.5.0_515.aar`） | 官方示例是本地 AAR 放 `libs/`；官方 Maven **未核实** | 未核实（appId、appKey 放 Manifest，未核实） | 最低 Android 6.0；Funtouch OS / OriginOS 具体要求、是否覆盖 iQOO **未核实** | 摘要（V232、V365） |

---

## 5. 全局规则和默认值

### 5.1 provider 取值

| 值 | 厂商 | 品牌判定（`Build.MANUFACTURER` / `Build.BRAND`，不区分大小写） |
|---|---|---|
| `hms` | 华为 | `HUAWEI` |
| `honor` | 荣耀 | `HONOR` |
| `mi` | 小米 | `Xiaomi`、`Redmi`、`POCO`（大陆版） |
| `oppo` | OPPO | `OPPO`、`OnePlus`、`realme`（OPPO 旧版文档说支持一加 5 以后和全部 realme） |
| `vivo` | vivo | `vivo`、`iQOO`（是否覆盖 iQOO **未核实**） |

品牌字符串只是初选，**最终以厂商 SDK 的"是否支持"判断为准**（华为 HMS Core 可用性、`HonorPushClient`、OPPO `isSupportPush`、vivo `isPushSupport` 等；小米的判断方法未核实）。

### 5.2 默认值（都可配置，都未校准）

| 配置 | 默认 | 说明 |
|---|---|---|
| `IM_VENDOR_PUSH_ENABLED` | 0 | 总开关，和 `IM_OFFLINE_PUSH_ENABLED` 同时为 1 才生效 |
| `IM_PUSH_<厂商>_ENABLED` | 0 | 每家单独开关（`HMS`、`HONOR`、`MI`、`OPPO`、`VIVO`） |
| `IM_PUSH_VENDOR_REREGISTER_DAYS` | 7 | 客户端距上次注册超过这么多天就重新注册（vivo 14 天失效，取一半） |
| 配额熔断恢复时刻 | Asia/Shanghai 次日 00:00 | 收到超配额错误后该厂商该分类暂停到这个时刻 |

### 5.3 AC 标记

沿用 PRD-chat 第 4.4 节（【后端】【前端】、手工、[IM 钟]、哨兵法）。批次【V1】。前端 AC 只针对 Android。

---

## 6. 功能点

### F-VND-01 客户端选择通道（P0）

**规则**
1. **一台设备只注册一个通道**。App 启动、登录、回到前台时按下面的顺序选，选中第一个可用的：
   1. 品牌属于五家之一，**且**该厂商 SDK 判断可用，**且**服务端启用了这家（`GET /v1/config` 的 `features.offlinePushProviders` 含该值）→ 用厂商通道；
   2. Google Play 服务可用，且服务端启用 FCM → 用 `fcm`；
   3. 都不可用 → 不注册，"设置 → 通知"显示 F-APN-06 的"此设备暂不支持离线通知，打开 App 才能收到消息"。
   
   **FCM 可用时谁优先是拍板项 VP-1**，上面写的是建议（厂商优先）。
2. 荣耀设备：荣耀推送可用就用 `honor`；不可用而 HMS Core 可用（老机型）是否退到 `hms`，见 VP-7（建议退到 `hms`）。
3. 选中的通道变了（比如用户装上了 Play 服务、厂商服务被禁用），用新通道重新注册，服务端用新令牌覆盖同一 `sid` 的旧令牌（F-VND-02 规则 2），客户端不需要先注销。
4. **厂商 SDK 在用户同意隐私政策之前不初始化、不注册**（小米 D1535 要求延迟初始化，OPPO 摘要 11229 要求同意后才能 init，vivo `agreePrivacyStatement` 要传真实同意状态 V652）。天听现在的隐私告知是 F-ACC-10；通知授权本来就在首次登录进入会话列表后才请求（F-APN-01 规则 5），厂商 SDK 在这之后初始化。
5. 系统通知授权关闭时不注册（F-APN-01 规则 5、5a 不变）。
6. 选择逻辑放在 Android 平台层，通过 FE-650 的 `PushTokenSource` 接口交给 sdk-kotlin；sdk-kotlin 只多一个"provider 由平台层给"的字段，不认识具体厂商。

**异常**：厂商 SDK 拿令牌失败或返回空（华为 `getToken` 可能返回空，H4）：等 `onNewToken` / 注册回调，不退到 FCM；下次启动重试，设置页显示"令牌获取中 / 获取失败"。厂商 SDK 初始化抛异常：按"该厂商不可用"处理，继续往下选。

**验收标准**
- AC-VND-01-1【前端】【V1】（Android）：输入 品牌替身依次为 HUAWEI、HONOR、Xiaomi、OPPO、vivo、Meizu，厂商可用性替身都返回可用，Play 服务替身不可用，服务端五家都启用；操作 登录进入会话列表并同意通知授权；预期 前五种各只发一次 `deviceNotificationRegister`，`provider` 依次为 `hms`、`honor`、`mi`、`oppo`、`vivo`；Meizu 不发，设置页显示"此设备暂不支持离线通知"；错误码 —。
- AC-VND-01-2【前端】【V1】（Android）：输入 品牌 Xiaomi，厂商可用性替身返回不可用；Play 服务可用 / 不可用；操作 登录；预期 依次注册 `fcm`；不注册并显示不支持文案；错误码 —。
- AC-VND-01-3【前端】【V1】（Android）：输入 品牌 HUAWEI，厂商和 Play 服务都可用；操作 登录；预期 按 VP-1 的结果只注册一个通道（建议口径下是 `hms`），不注册 `fcm`；错误码 —。
- AC-VND-01-4【前端】【V1】（Android）：输入 品牌 vivo，厂商可用，`features.offlinePushProviders` 不含 `vivo`；Play 服务不可用；操作 登录；预期 不注册，显示不支持文案；错误码 —。
- AC-VND-01-5【前端】【V1】（Android）：输入 首次安装，还没有同意隐私告知；操作 启动 App 停在隐私告知页；预期 厂商 SDK 替身的初始化和注册方法都没有被调用；同意并登录后才调用；错误码 —。
- AC-VND-01-6【前端】【V1】（Android）：输入 已用 `fcm` 注册；操作 Play 服务替身改为不可用、厂商可用后回到前台；预期 发一次 `provider=<厂商>` 的注册，没有先发注销；错误码 —。
- AC-VND-01-7【前端】【V1】（Android）：输入 品牌 HONOR，荣耀推送不可用、HMS Core 可用；操作 登录；预期 按 VP-7 的结果（建议口径下注册 `hms`）；错误码 —。

### F-VND-02 服务端注册（P0）

**规则**
1. `deviceNotificationRegister.provider` 增加 `hms`、`honor`、`mi`、`oppo`、`vivo`。厂商令牌没有 `env`（`env` 只对 `apns` 有意义，厂商令牌带了忽略）。令牌长度上限沿用 BE-450 的 4096 字节。
2. 同一 `sid` 只有一条令牌：再次注册（不论 provider）覆盖旧行，"显示发送者和内容"不变（F-APN-01 规则 2 和 R-CB-23 的口径）。
3. 令牌唯一键是 `(provider, token_hash)`：同一个串在不同厂商下是不同的令牌。同一厂商同一令牌换账号注册，按 F-APN-01 规则 2 从旧账号移走并把"显示内容"重置为关。
4. 服务端没启用的 provider：403 / 4030，`reason=providerDisabled`（新 reason，实现前和 `errors.json` 核对是否占用）；未知的 provider 值：400 / 4000（Schema 拒绝）。
5. `GET /v1/config` 的 `features` 增加 `offlinePushProviders: [...]`，只列服务端已启用且凭证检查通过的通道（含 `apns`、`fcm`）。
6. 注册频率、日志只记哈希前缀、`sid` 吊销同事务删令牌、发送前复查：F-APN-01 规则 1、4、6 不变。

**异常**：注册的 provider 在注册后被关掉：已有令牌保留，不推送，配置检查显示数量；重新打开后恢复。

**验收标准**
- AC-VND-02-1【后端】【V1】：输入 五家都启用；操作 依次用五个 provider 注册；用 `provider:"meizu"` 注册；关掉 `IM_PUSH_VIVO_ENABLED` 后用 `vivo` 注册；预期 前五次 `{}`；第六次 400；第七次 403，`reason=providerDisabled`；错误码 4000、4030。
- AC-VND-02-2【后端】【V1】：输入 u2 同一 `sid` 先注册 `fcm` 令牌 T1，再注册 `vivo` 令牌 T2；操作 u2 离线，u1 发消息，再给哨兵发一条；预期 只有假 vivo 收到 u2 的调用，假 FCM 在哨兵被调用后仍没有 u2 的调用；`device_push` 里 u2 只有一行；错误码 —。
- AC-VND-02-3【后端】【V1】：输入 u1 注册 `mi` 令牌 X，u2 注册 `oppo` 令牌 X（同一个串）；操作 分别给 u1、u2 发消息；预期 两条令牌都在，各自走自己的假通道；错误码 —。
- AC-VND-02-4【后端】【V1】：输入 五家各注册一个令牌，并让每家通道各发一次；操作 扫描日志、审计和指标标签；预期 搜不到任何令牌原文，只有 provider 和 SHA-256 前 8 位；错误码 —。
- AC-VND-02-5【后端】【V1】：输入 启用 `hms`、`mi`，`oppo` 开关打开但缺密钥（非生产）；操作 `GET /v1/config`；预期 `offlinePushProviders` 含 `hms`、`mi`，不含 `oppo`；错误码 —。

### F-VND-03 通知内容、分类和折叠（P0）

**规则**
1. 一律发**通知栏消息**（华为、荣耀 `android.notification`；小米 `pass_through=0`；OPPO `notification`；vivo 默认通知），不发透传。
2. 文案、隐藏 / 详情模式、整个请求不含 `convId`、`userId`、`msgId` 及普通哈希：F-APN-03 规则 1～3 不变。**厂商请求里要多带的字段（`requestId`、`app_message_id` 等）用每次随机生成的值**，不用消息 ID 推出来。
3. **IM 分类**：

   | 厂商 | 请求里怎么写 |
   |---|---|
   | 华为 | `android.category = "IM"`；`notification.importance = "NORMAL"` |
   | 荣耀 | 未核实（分类字段待查，VN-2） |
   | 小米 | `extra.channel_id` = 审核通过的"聊天消息"channel；`extra.template_id`、`extra.template_param`（规则 4） |
   | OPPO | `notification.channel_id` = 管理台登记的通讯类 channel；`category = "IM"`（摘要，要求 OS13 以上，VN-3） |
   | vivo | `category = "IM"`，不传 `classification`；`skipType` 隐藏模式 1（首页），详情模式见 F-VND-03 规则 6 |
4. **小米模板**：申请两个"聊天消息"模板：隐藏模式模板没有变量（文案固定"天听 / 你收到一条新消息"）；详情模式模板两个变量（标题、摘要）。**变量能否放任意聊天内容、审核能否通过，未核实**；不通过时小米的详情模式退回隐藏模式文案，设置页提示"小米手机暂只显示通用通知"（VP-6）。没有变量时 `template_param` 传什么（空 JSON 对象还是不传）未核实。
5. **折叠 id**：C3 的 collapse id 规则（隐藏模式每设备一个固定 id，详情模式 `HMAC(服务端密钥, sid + convId)`，F-APN-02 规则 6）不变，按厂商字段换算：
   - 华为、荣耀：`notification.tag` = 32 个十六进制字符（同 C3）。华为的 `collapse_key` 不设（默认 -1）。**tag 长度上限未核实**。
   - 小米 `notify_id`、vivo `notifyId`：取同一 HMAC 的前 31 位作为整数；vivo 取到 0 时用 1。整数由服务端密钥派生，不能反推会话。
   - OPPO：替换机制**未核实**；查到之前只靠 5 秒合并（F-APN-02 规则 6），可能出现多条通知并存。
6. 点击：隐藏模式打开 App 首页（进会话列表）；详情模式带 deep link `tianting://chat/{convId}`（F-APN-04 规则 1）。各家放 intent / 链接的字段：华为 `click_action`（H8）、vivo `skipType` + `skipContent`（`skipContent` 字段名未核实）、小米、OPPO、荣耀的字段未逐条核实（VN-4）。
7. 有效期：隐藏模式 24 小时，详情模式 1 小时（F-APN-02 规则 8），写到各家的有效期字段；字段名和单位见第 4.4 节，未核实的实现前查。
8. 通知渠道：Android 端仍然创建渠道"消息"（F-APN-04 规则 3）；小米、OPPO 的 channel_id 要和后台登记的一致，渠道 id 实现前定下来写进 Frank 的申请材料（第 16 节）。
9. 推送不影响发送路径、队列单位、批量查询：F-APN-02 规则 7 不变。

**异常**：华为 `80100016`（内容含敏感信息）：不重试，计 `bad_request`；只可能在详情模式出现，**隐藏模式出现说明固定文案被判违规，告警**。小米模板变量超长：截断到模板允许的长度（上限未核实）。

**验收标准**
- AC-VND-03-1【后端】【V1】：输入 默认（隐藏）设置，消息含特征串 `ZXQ-VND-1`，发送者昵称 `ZXQ-NICK`，五家各一个离线设备；操作 抓五个假通道的完整请求（请求头和请求体，含鉴权请求）；预期 搜不到特征串、昵称、任何 userId、`convId`、`msgId`；标题"天听"、正文"你收到一条新消息"（小米为隐藏模板 id）；分类字段按规则 3 的表；错误码 —。
- AC-VND-03-2【后端】【V1】：输入 详情模式；操作 五家各推一条文本、一条图片；预期 标题是昵称、正文是摘要（≤ 50 码点），图片为"[图片]"；小米走详情模板，`template_param` 的变量值都是字符串；错误码 —。
- AC-VND-03-3【后端】【V1】：输入 隐藏模式，同一设备两个会话；详情模式，两个会话、两台设备；操作 抓折叠字段；预期 隐藏模式两个会话的 `tag` / `notify_id` / `notifyId` 相同；详情模式不同会话不同、同一会话不同设备不同；整数在 1～2147483647 内；都不等于也不包含 `convId`；华为请求没有 `collapse_key`；错误码 —。
- AC-VND-03-4【后端】【V1】：输入 隐藏、详情两种模式；操作 五家各推一条；预期 有效期字段分别对应 24 小时、1 小时；错误码 —。
- AC-VND-03-5【后端】【V1】：输入 五个假通道都一直超时，≥ 1000 条消息，生产配置，3 次取最差；操作 单聊发送；预期 全部 ack 成功，ack P99 ≤ 200 ms（未校准，开发机参考）；`sync` 不缺；错误码 —。
- AC-VND-03-6【后端】【V1】：输入 vivo 设备的消息；操作 抓请求；预期 有 `category:"IM"`，没有 `classification`；`requestId` 两次请求不同，且不等于 `msgId` 及其哈希；错误码 —。

### F-VND-04 鉴权和凭证（P0）

**规则**
1. 华为：OAuth 客户端模式（H2），access token 缓存，到期前 5 分钟换新；收到 HTTP 401、`80200003` 先换 token 重试 1 次，不计凭证失败；`80600003`、`80300002`、其余 401 计凭证失败。JWT 服务账号方式本批不用（VP 不需要，第 18.2 节）。
2. 荣耀：OAuth 客户端模式（R1），缓存方式同华为；有效期和过期错误码未核实，实现前查（VN-2）。
3. 小米：每个请求带 `Authorization: key=<AppSecret>`，无 token 缓存；`21301` 等鉴权错误计凭证失败。
4. OPPO：`auth_token` 缓存 24 小时，到期前 1 小时换新（官方说过期后有 10 分钟过渡期，摘要）；auth_token 无效的返回码未核实，实现前查。
5. vivo：`authToken` 集中缓存，每 1 小时刷新（官方建议 1～2 小时，V362）；`10000` 先重新鉴权重试 1 次；`10206`、`10205` 计凭证失败；鉴权接口每秒不超过 10 次（`10093`）。
6. 时钟：OPPO、vivo 的签名带毫秒时间戳，服务器时间偏差超过 10 分钟会失败；运维手册写明要开 NTP。
7. 密钥文件 0600、不进仓库、生产缺密钥拒绝启动、非生产告警并关闭、配置检查不含密钥：F-APN-05 规则 3 和 AC-APN-05-1、05-2 的口径不变，扩到五家。
8. 所有鉴权和发送用 Go 标准库（`net/http`、`crypto/sha256`、`crypto/md5`），**不引入厂商服务端 SDK**。

**异常**：凭证失效不影响消息和其他通道（F-APN-05 异常）。同时多个 worker 发现 token 过期：只换一次（单飞）。

**验收标准**
- AC-VND-04-1【后端】【V1】[IM 钟]：输入 假华为、假荣耀的 token 端点；操作 连续推 100 条；把 [IM 钟] 推过有效期再推 1 条；预期 第一段只请求 1 次 token；推过后再请求 1 次；错误码 —。
- AC-VND-04-2【后端】【V1】：输入 假华为对下一次发送返回 `80200003`；假 vivo 返回 `10000`；操作 各推一条；预期 各自换 token 后重试 1 次成功，总调用 2 次，凭证失败计数不增加；错误码 —。
- AC-VND-04-3【后端】【V1】：输入 假 OPPO 校验 `sign = SHA256(app_key + timestamp + master_secret)`；假 vivo 校验 `sign = md5(appId + appKey + timestamp + appSecret)` 小写；假小米校验 `Authorization: key=`；操作 各推一条；预期 签名和头都通过；时间戳偏 11 分钟时假通道拒绝，计凭证失败；错误码 —。
- AC-VND-04-4【后端】【V1】：输入 `IM_ENV=production`，五家开关之一打开但缺密钥；密钥文件权限 0644；操作 启动；预期 都是非 0 退出，错误信息不含密钥内容；错误码 —。
- AC-VND-04-5【后端】【V1】：输入 配置检查接口；操作 `GET /internal/v1/config`；预期 每家显示是否启用、应用 id、文件存在、权限正确、最近一次调用结果、当天是否处于配额熔断，不含密钥；错误码 —。
- AC-VND-04-6【后端】【V1】：输入 假华为 token 端点返回 `80600003`；操作 推一条；预期 计凭证失败，不重试，其他四家和 FCM 照常；错误码 —。

### F-VND-05 错误处理、重试和配额（P0）

**规则**
1. **删令牌**（只删这一条，按原因计数）：华为 `80300007`、`80100000` 的 `illegal_tokens`；小米 `66007`、`10029`、回执 type=16 的无效设备；OPPO `10000`、回执 `regid_invalid`；vivo `10302`。荣耀未核实，查到前不删（只计失败）。
2. **不删令牌**：小米 `65003`（设备不在线）；华为 `80300002`（无权限，多半是配置错，同 C3 对 `DeviceTokenNotForTopic` 的处理，计 `bad_request` 并在配置检查显示）；所有鉴权错误。
3. **重试**：次数沿用 F-APN-02 规则 8（首次外最多 2 次，超过有效期就放弃）。可重试的：华为 HTTP 500、502、503、`81000001`；小米 `200002`、`10027`（QPS）；vivo `10072`；各家 HTTP 5xx 和超时。间隔：有 `Retry-After` 按它，没有时 60 秒起翻倍（和 C3 FCM 一致）。小米 `1`、`10001`～`10003`、`65029` 官方没说可重试，**按不重试处理**（VN-5）。
4. **配额熔断**：小米 `200001`、vivo `10070`、`10073`：不重试，该厂商该分类暂停到 Asia/Shanghai 次日 00:00，期间的通知直接丢弃并计 `quota_exceeded`，配置检查显示；**IM 分类权益生效后这几个码正常不应出现，出现就告警**。vivo `10071`（超出时段）只出现在运营消息，出现说明分类没生效，同样告警。
5. 指标沿用 `im_offline_push_total{provider,result}`，`provider` 多 5 个取值，`result` 增加 `quota_exceeded`（有界）。
6. 请求体由服务端固定模板生成；`80300008`、`10096`、`10097` 等"请求不合法"的码不重试，计 `bad_request`，批量出现就是模板错了。
7. 不使用小米 feedback 接口（已下线，D1560）；OPPO Feedback 接口和各家回执本批不接（只靠发送响应删令牌），见 VP-8。

**验收标准**
- AC-VND-05-1【后端】【V1】：输入 五家假通道分别返回规则 1 的失效码（华为 `80300007`、小米 `66007`、OPPO `10000`、vivo `10302`）；操作 推一次，再推下一条；预期 令牌被删除，下一条不再调用；荣耀假通道返回任意 4xx 时令牌不删；错误码 —。
- AC-VND-05-2【后端】【V1】：输入 华为一次请求带 3 个令牌（同一消息的 3 个设备），假通道返回 `80100000`，`illegal_tokens` 含其中 1 个；操作 推送；预期 只删这 1 个；错误码 —。
- AC-VND-05-3【后端】【V1】[IM 钟]：输入 假华为一直 503；假小米返回 `200002` 一次后成功；假 vivo 返回 `10072` 一次后成功；操作 推送并推进 [IM 钟]；预期 总调用依次 3、2、2 次，间隔不小于 60 秒；错误码 —。
- AC-VND-05-4【后端】【V1】[IM 钟]：输入 假小米返回 `200001`；操作 再推 3 条，推进 [IM 钟] 到次日 00:00 后再推 1 条；预期 熔断期间 3 条都没有调用，`quota_exceeded` 增加 3，配置检查显示熔断；次日那条有调用；其他厂商不受影响；错误码 —。
- AC-VND-05-5【后端】【V1】：输入 假小米返回 `65003`；假华为返回 `80300002`；操作 推送，再推下一条；预期 令牌都没删，下一条照常调用；错误码 —。
- AC-VND-05-6【后端】【V1】：输入 五家各一个令牌绑定 `sid`；操作 退出、改密、强制下线；预期 相应令牌同事务删除，之后不调用（AC-APN-01-3 的口径扩到五家）；错误码 —。
- AC-VND-05-7【后端】【V1】：输入 消息入队后、调用假 vivo 前吊销 `sid`；操作 观察假通道；预期 没有调用；错误码 —。

### F-VND-06 客户端令牌生命周期（P0）

**规则**
1. 令牌回调（华为 `onNewToken`、小米注册回调、OPPO `onRegister`、vivo `onReceiveRegId`、荣耀待查）交给 SDK 缓存，连上后注册（F-APN-01 规则 5a）。
2. 回到前台时距上次注册超过 `IM_PUSH_VENDOR_REREGISTER_DAYS`（7 天）就重新注册一次，令牌没变也注册（在每小时 10 次的限额内）。原因：vivo 14 天没长连接就失效，小米、OPPO 30 天。
3. vivo：每次冷启动调用 `turnOnPush`，成功后取 regId，变了就注册（官方建议，V232）；`turnOnPush` 每天 100 次，冷启动次数远低于这个数。
4. **退出登录时客户端不调用厂商的注销接口**（华为不建议直接调 `deleteToken`，H4；小米 `unregisterPush` 会让 regId 失效）。服务端在 `sid` 吊销时删令牌（F-APN-01 规则 4）已经够了；下次登录用同一令牌重新注册。这和 C3 对 FCM 的 `SystemTokenRevoker` 不同，FE-611 的调用点对厂商通道是空实现。
5. 华为 `getToken` 在子线程调用，返回空时只等回调（H4）。

**验收标准**
- AC-VND-06-1【前端】【V1】（Android）：输入 厂商令牌回调发生在没有连接时；操作 之后连上；预期 连上后发一次注册；错误码 —。
- AC-VND-06-2【前端】【V1】（Android）[SDK 钟]：输入 上次注册在 8 天前，令牌没变；上次注册在 6 天前；操作 回到前台；预期 前者重新注册一次，后者不注册；错误码 —。
- AC-VND-06-3【前端】【V1】（Android）：输入 vivo 替身，冷启动时 `turnOnPush` 回调后 `getRegId` 返回新值；返回旧值；操作 冷启动；预期 新值时注册一次；旧值时不额外注册（规则 2 的周期除外）；错误码 —。
- AC-VND-06-4【前端】【V1】（Android）：输入 五个品牌替身，已注册；操作 退出登录；预期 发送退出请求，厂商替身的注销 / `deleteToken` / `unregisterPush` / `turnOffPush` 都没有被调用；错误码 —。

### F-VND-07 设置页和用户可见状态（P0）

**规则**
1. "设置 → 通知"的状态沿用 FE-653：系统授权关闭、令牌获取中、令牌获取失败、设备不支持、保存失败。
2. 打开"显示发送者和内容"前的提示改为写明实际通道："内容会经过 {华为 / 荣耀 / 小米 / OPPO / vivo / 谷歌} 的推送服务器"（按本机选中的通道）。
3. 小米详情模板没批下来时（F-VND-03 规则 4），开关仍可打开，下方提示"小米手机暂只显示通用通知"。
4. 不显示通道名以外的技术细节（令牌、错误码）。

**验收标准**
- AC-VND-07-1【前端】【V1】（Android）：输入 品牌替身 vivo，已选中 `vivo`；操作 打开"显示发送者和内容"；预期 提示里写"vivo"，确认后才生效；错误码 —。
- AC-VND-07-2【前端】【V1】（Android）：输入 品牌 Xiaomi，服务端配置标记小米详情模板不可用；操作 打开详情开关；预期 开关打开，显示"小米手机暂只显示通用通知"；错误码 —。

### F-VND-08 隐私合规（P0，给法务）

**规则**
1. 隐私政策（PRD-admin 第 12 节 ②）增加"第三方 SDK"清单，每个 SDK 写：名称、提供方、收集的信息、用途（消息推送）、调用的权限、提供方隐私政策链接（小米 D1535 明确要求这些项；OPPO 摘要 11229 要求同样的项）。第 9 节是草稿。
2. 厂商 SDK 只在用户同意隐私政策后初始化（F-VND-01 规则 4）。隐私告知版本变化需要重新同意时（F-ACC-10），新版本同意前不初始化新加入的 SDK。
3. 详情模式的内容经过选中的厂商服务器，提示按 F-VND-07 规则 2。
4. 本节不是法律意见，问题清单见第 9.2 节。

**验收标准**
- AC-VND-08-1（手工）【V1】：输入 发布构建的隐私政策页；操作 对照第 9.1 节；预期 五个 SDK 都在清单里，每项都有名称、提供方、收集的信息、用途、链接；错误码 —。

### F-VND-09 真机验收（手工）

**验收标准**
- AC-VND-09-1（手工）【V1】：输入 五个品牌各一台大陆版真机，隐藏模式，App 被系统杀死；操作 对方发消息；预期 10 秒内收到（未校准），锁屏只显示"天听 / 你收到一条新消息"，点开进会话列表；错误码 —。
- AC-VND-09-2（手工）【V1】：输入 同上，详情模式；操作 点通知；预期 直接进该会话（小米模板不可用时按 AC-VND-07-2 显示通用通知，点开进会话列表）；错误码 —。
- AC-VND-09-3（手工）【V1】：输入 每家一个真实有效令牌和一个卸载后失效的令牌；操作 发送；预期 有效的收到；失效的被删除（荣耀待错误码核实后验）；错误码 —。
- AC-VND-09-4（手工）【V1】：输入 IM 分类权益已批的五个品牌真机；操作 一天内连续收 10 条通知（间隔超过合并窗口）；预期 每条都有横幅和提示音，没有被归为静默或营销通知，没有当天被限量；错误码 —。
- AC-VND-09-5（手工）【V1】：输入 魅族或其他品牌、没有 Play 服务的真机；操作 打开"设置 → 通知"；预期 显示"此设备暂不支持离线通知"（AC-APN-06-2 的口径不变）；错误码 —。

---

## 7. 协议变更清单（实现前落实到 `protocol/`，本文不改）

| 位置 | 变更 |
|---|---|
| `frame.schema.json` | `deviceNotificationRegister.provider` 枚举加 `hms`、`honor`、`mi`、`oppo`、`vivo` |
| `GET /v1/config` | `features.offlinePushProviders: string[]`；小米详情模板是否可用的标记（字段名实现时定） |
| `errors.json`、`ERRORS.md` | 4030 增加 `reason=providerDisabled`（先核对未占用），不新增错误码 |
| `frames.json` | 五个 provider 的合法用例、未知 provider 的非法用例 |

---

## 8. 客户端

### 8.1 Android 改动
1. 通道选择器（F-VND-01），实现 FE-650 的 `PushTokenSource`。
2. 五个厂商 SDK 接入：初始化、令牌回调、"是否支持"判断；**每个厂商的接入放在独立模块或独立源集**，没有该厂商配置时编译为空实现（和 C3 占位 `google-services.json` 的思路一样，R-CQ-21）。
3. 华为 `agconnect-services.json` 等真实配置只在发布时注入，CI 和开发用占位（AC-APN-01-12 的口径扩到华为）。客户端用到的 AppId、AppKey、AppSecret（小米、OPPO 的 `register` 要传，H/O/D 页面）**也在发布时注入，不进仓库**；它们能从 APK 里提取，不算服务端密钥，但仓库公开，不放更稳妥。
4. 设置页文案（F-VND-07）。

### 8.2 新依赖（**引入要 Frank 同意**）

| SDK | 获取方式 | 备注 |
|---|---|---|
| 华为 `com.huawei.hms:push` + Gradle 插件 `com.huawei.agconnect:agcp` | 华为 Maven `https://developer.huawei.com/repo/`（H9） | 要在 Gradle 加一个新仓库 |
| 荣耀推送 SDK | **未核实**（坐标和仓库都没读到） | 实现前查官方集成页 |
| 小米 MiPush SDK | 官网下载 AAR（D1544），无 Maven | 要定 AAR 放哪里（VP-5）；**本次没有下载** |
| OPPO heytap push SDK | 旧版只给 AAR（O tid=1918）；新版 Maven 未核实 | 同上 |
| vivo vpush clientSDK | 官方示例是本地 AAR（V232）；Maven 未核实 | 同上 |

AAR 文件能不能放进公开仓库（再分发许可）**未核实**，见 VP-5。

---

## 9. 隐私合规材料（给法务的草稿，不是法律意见）

### 9.1 SDK 收集的数据（按官方页面）

| SDK | 提供方 | 收集 / 处理的信息 | 来源 |
|---|---|---|---|
| 华为推送 | 华为（主体名称未核实） | AAID、Push Token、AppID、包名、应用版本、设备类型和型号、系统版本、国家码、消息内容等；投递数据留 15 天，统计数据留 5 年。SDK 只在应用打开时运行 | H10、H11「已读」 |
| 荣耀推送 | 荣耀（主体名称未核实） | 系统服务声明：设备语言和地区、UDID、型号、系统版本、网络方式、MCC/MNC、IP；处理 OAID、应用安装 / 卸载 / 运行状态，用于内容风控和分类；内嵌听云 SDK。**这是系统服务的用户声明，不是给开发者的 SDK 说明**（后者没找到） | R3「已读」 |
| 小米推送 | 北京小米移动软件有限公司 | 包名、版本、运行状态，设备厂商、型号、内存、系统版本、归属地，WiFi 状态，通知栏设置，消息内容和送达、点击时间；设备标识 SHA256 后映射成自定义 ID。要求延迟初始化 | D1535、D1534「已读」 |
| OPPO 推送 | 广东欢太科技有限公司 | RegisterId、appKey、appSecret、包名、应用版本号、SDK 版本号；数据存在境内。要求同意后再 init | 11229「摘要」 |
| vivo 推送 | 主体名称未核实 | 应用基本信息、应用内设备标识符、设备硬件信息、系统基本信息；`agreePrivacyStatement` 要传真实同意状态 | V541、V652「摘要」 |

### 9.2 给法务的问题（接 PRD-chat 第 20.1a 节）
1. 五家都是数据处理方还是独立的个人信息处理者；荣耀开通时要同意《数据处理附录》（R4 摘要），其他几家要不要签。
2. 隐私政策里 SDK 清单的写法（第 9.1 节）是否够；是否要单独的"第三方 SDK 共享清单"页面。
3. 详情模式下聊天摘要经过厂商服务器（境内），和 CH-11 的谷歌、苹果（境外）比，告知要怎么写。
4. 上架 OPPO、vivo 商店算不算"面向公众"（CH-23 第 10 问），要不要实名、备案（VP-4）。
5. 荣耀声明里内嵌的第三方 SDK（听云）要不要在天听的清单里写。

---

## 10. 服务端

| 项 | 内容 |
|---|---|
| 包 | `server/internal/push/{hms,honor,mipush,oppo,vivo}`，各自实现 `push.Provider` |
| 共用 | token 缓存和单飞、配额熔断、错误码到 `result` 的映射表、整数折叠 id 派生 |
| 出站主机 | `oauth-login.cloud.huawei.com`、`push-api.cloud.huawei.com`、`iam.developer.honor.com`、`push-api.cloud.honor.com`、`api.xmpush.xiaomi.com`、OPPO（**未核实**，疑似 `api.push.oppomobile.com`）、`api-push.vivo.com.cn`；均 443（端口未逐家核实） |
| 部署地 | 五家都在境内，服务器在大陆时网络问题比 FCM 小；**服务器在境外时的连通性未核实**（R-4 的反向问题） |
| 依赖 | 只用标准库，不加服务端依赖 |
| 数据库 | 不需要新迁移：`device_push.provider` 是字符串；唯一键从 `token_hash` 改成 `(provider, token_hash)` 要一个迁移（编号接 C3 之后） |

---

## 11. 配置项

| 变量 | 说明 |
|---|---|
| `IM_VENDOR_PUSH_ENABLED`、`IM_PUSH_{HMS,HONOR,MI,OPPO,VIVO}_ENABLED` | 第 5.2 节 |
| `IM_HMS_CLIENT_ID`、`IM_HMS_CLIENT_SECRET_FILE` | 华为 AGC 的 Client ID 和 Client Secret（文件，0600） |
| `IM_HONOR_APP_ID`、`IM_HONOR_CLIENT_ID`、`IM_HONOR_CLIENT_SECRET_FILE` | 荣耀推送服务页的 APPID、Client ID、Secret |
| `IM_MI_APP_SECRET_FILE`、`IM_MI_PACKAGE`、`IM_MI_CHANNEL_ID`、`IM_MI_TEMPLATE_HIDDEN`、`IM_MI_TEMPLATE_DETAIL` | 小米 AppSecret、包名 `com.tianting.app`、channel、两个模板 id（详情模板为空 = 不可用） |
| `IM_OPPO_APP_KEY`、`IM_OPPO_MASTER_SECRET_FILE`、`IM_OPPO_CHANNEL_ID` | OPPO |
| `IM_VIVO_APP_ID`、`IM_VIVO_APP_KEY`、`IM_VIVO_APP_SECRET_FILE` | vivo |
| `IM_{HMS,HONOR,MI,OPPO,VIVO}_ENDPOINT`、`..._AUTH_ENDPOINT` | **只在测试模式生效**，指向假通道；生产配了拒绝启动（同 `IM_APNS_ENDPOINT`） |
| `IM_PUSH_VENDOR_REREGISTER_DAYS` | 客户端读取的重新注册周期，经 `GET /v1/config` 下发 |

`deploy/push/im-push.example.env` 补这些变量的占位符，**没有任何真实值**（仓库公开）。

---

## 12. 测试替身（照 BE-406 的写法）

放 `server/internal/push/pushtest`，在测试进程里、独立端口；只在 `IM_TEST_MODE=1` 生效。每家一个 HTTP 层假通道，共同能力：保存完整请求（含请求头）供断言；读取、清空；设定下一次或持续的响应；阻塞、超时、"调用前暂停"（AC-VND-05-7）。测试用的密钥在临时目录现场生成，**仓库里没有任何密钥文件**。

| 假通道 | 校验 | 可注入 |
|---|---|---|
| 假华为 | token 端点：`grant_type`、`client_id`、`client_secret`，签发带 `expires_in` 的 token；发送：`Bearer`、路径里的 client id、`message.android.notification`、`category`、`tag`；不应有 `collapse_key` | HTTP 401、`80200003`、`80600003`、`80300002`、`80300007`、`80100000` + `illegal_tokens`、`80300008`、`80100016`、500 / 502 / 503 |
| 假荣耀 | token 端点、`Bearer`、路径里的 APPID、`android.notification`、`tag` | 401、任意 4xx、5xx（错误码核实后补） |
| 假小米 | `Authorization: key=`、`restricted_package_name`、`pass_through=0`、`extra.channel_id`、`extra.template_id`、`template_param` 是 JSON 字符串且值都是字符串、`notify_id` 范围 | `66007`、`10029`、`65003`、`21301`、`200001`、`200002`、`10027`、5xx |
| 假 OPPO | 鉴权：`sign` 按 SHA-256 重算、时间戳 ±10 分钟；签发 24 小时 `auth_token`；单推：form 里 `message` 是 JSON、`target_type=2`、`channel_id` | `10000`、auth_token 失效（码核实后补）、5xx |
| 假 vivo | 鉴权：`sign` 按 md5 重算（小写）、时间戳 ±10 分钟、每秒 10 次；发送：`authToken` 头、必填字段、`category` 大写、没有 `classification`、`notifyId` 范围 | `10302`、`10000`、`10206`、`10070`、`10071`、`10072`、`10073`、`10093`、`10096`、5xx |

**假通道只验调度逻辑和请求格式，不证明厂商真实行为**（和假 S3 同样的限制）；真实行为靠第 F-VND-09 的手工 AC。

---

## 13. 对已冻结文档的影响（本草稿不改，V1 冻结时走变更）

| 文档 | 影响 |
|---|---|
| PRD-chat F-APN-06 | 规则 1"不做国内厂商通道"改为"五个品牌见 PRD-vendorpush"；AC-APN-06-1、06-2 的输入限定为"五个品牌以外" |
| PRD-chat F-APN-01 规则 1 | provider 枚举；规则 3 的删令牌错误码加厂商的 |
| PRD-chat CH-13、R-2、第 18 节"不含"、第 21 节第 5 条 | 记录 P-42 的决定 |
| `protocol/` | 第 7 节 |
| backend-chat、frontend-chat | 新任务 BE、FE 编号（V1 分发时定）；FE-611 `SystemTokenRevoker` 对厂商是空实现 |
| `tools/trace.py` | `CHAT_MILESTONES` 之外再纳入本 PRD 的【V1】（冻结后） |
| 隐私政策（PRD-admin 第 12 节 ②） | 第 9.1 节的 SDK 清单 |

---

## 14. 工作量粗估（人日，**低置信**，没有校准）

按 PRD-chat 第 18 节的列。每家的服务端 2 = 鉴权和缓存 0.5 + 发送和字段 0.5 + 错误映射和熔断 0.5 + 配置检查和启动检查 0.5；测试 2 = 假通道 0.5 + 后端 AC 1 + 真机 0.5；Android 1.5 = SDK 接入、回调、可用性判断、构建注入。

| 功能 | 服务端 | console + 后台页面 | sdk-ts | sdk-kotlin | sdk-swift | H5 | Android | iOS | 测试 | 部署 | 文档 | 小计 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 公共：协议、通道选择、`offlinePushProviders`、唯一键迁移、整数折叠 id、熔断、指标、设置页、隐私清单草稿 | 3 | 0.5 | 0 | 1.5 | 0 | 0 | 3 | 0 | 2 | 1 | 1.5 | 12.5 |
| 华为 | 2 | 0 | 0 | 0 | 0 | 0 | 1.5 | 0 | 2 | 0.5 | 0.5 | 6.5 |
| 荣耀 | 2 | 0 | 0 | 0 | 0 | 0 | 1.5 | 0 | 2 | 0.5 | 0.5 | 6.5 |
| 小米（含两个模板） | 2.5 | 0 | 0 | 0 | 0 | 0 | 1.5 | 0 | 2 | 0.5 | 0.5 | 7 |
| OPPO | 2 | 0 | 0 | 0 | 0 | 0 | 1.5 | 0 | 2 | 0.5 | 0.5 | 6.5 |
| vivo | 2 | 0 | 0 | 0 | 0 | 0 | 1.5 | 0 | 2 | 0.5 | 0.5 | 6.5 |
| **合计** | **13.5** | **0.5** | **0** | **1.5** | **0** | **0** | **10.5** | **0** | **12** | **3.5** | **4** | **45.5** |

对照：C3（APNs + FCM，含在线判断、队列、隐私等全部基础）是 38.5。V1 的每家成本主要是"多一套鉴权和错误码"，基础已在 C3。**荣耀和 OPPO 的未核实项最多**，实现时可能各多 1～2 人日；估计区间约 40～60。

**不含**：评审和复审；Frank 的账号、审核等待（每项 1～5 个工作日，可能多轮）；OPPO / vivo 上架（软著、App 备案，VP-4）；法务；真机采购（P-13）；HarmonyOS NEXT；其他小厂商；回执接口（VP-8）。

---

## 15. 里程碑和依赖

| 里程碑 | 开工前必须有 | 交付后可以演示 |
|---|---|---|
| V0 准备（**可以和 C3 并行**） | 本 PRD 评审、拍板（第 17 节）；Frank 开始第 16 节的账号、应用和审核 | — |
| V1 公共 | C3 后端完成（BE-450～457，令牌表、队列、Provider 接口）；VP-1、VP-5 已定 | 通道选择和假通道下的完整链路 |
| V1 第一波（建议华为、荣耀、小米，VP-2） | 该厂商的账号、应用、IM 分类权益或私信 channel 已批；该品牌真机 | 该品牌 App 被杀后收到通知 |
| V1 第二波（建议 OPPO、vivo） | 同上；**VP-4 定了上架方案，或接受测试权限的限制** | 同上 |
| V1 验收 | 五家后端 AC 通过；F-VND-09 每个品牌的手工 AC（真机，P-13） | 大陆五个品牌都有离线通知 |

每家有单独开关（第 5.2 节），审核先下来的先开，不必等五家都好。

---

## 16. Frank 要亲自做的事（AI 做不了）

| 事 | 步骤 | 挡住什么 | 资料可信度 |
|---|---|---|---|
| 定账号主体（VP-3） | 个人还是企业；企业要营业执照，各家实名认证 | 所有厂商账号；OPPO 只有企业账号（「已读」） | 见各行 |
| 华为 | ① 注册华为开发者账号并实名（个人或企业）；② AppGallery Connect 建项目和应用（包名 `com.tianting.app`）；③ "项目设置 > API 管理"打开推送服务，"增长 > 推送服务"开通；④ 申请**自分类权益**（IM，5 个工作日）；⑤ 下载 `agconnect-services.json`（发布时注入，不进仓库）；⑥ 把 Client ID 和 Client Secret 放服务器文件（0600） | ①～③⑥ 挡住华为真机验收；④ 挡住 AC-VND-09-4（没批之前分类可能按营销算，未核实） | 开通步骤已读（H5、H6）；FAQ 说**未上架华为应用市场也能用推送**；个人能否申请自分类权益、要不要软著、费用**未核实** |
| 荣耀 | ① 注册荣耀开发者账号并认证；② 管理中心 > 生态服务 > 应用服务 > 推送服务申请：应用类型"移动应用"，填包名和**全部 SHA256 证书指纹**（debug 和 release 签名都要，签名证书由 Frank 保管）；③ 同意《荣耀推送服务使用协议》和《数据处理附录》（给法务看）；④ 记下 APPID、Client ID、Secret 放服务器；⑤ 消息分类申请（规则未核实） | 荣耀真机验收 | 摘要（R4）；从流程看不要求先上架（推断）；个人能否开通、费用**未核实** |
| 小米 | ① 注册小米开发者账号（审核 1～3 个工作日）；② 登录 `admin.xmpush.xiaomi.com` 启用推送、同意协议、建应用（包名）、拿 AppId / AppKey / AppSecret；③ "消息分类管理 - channel 列表"申请"聊天消息" channel（5 个工作日）；④ 申请两个私信模板（隐藏、详情，F-VND-03 规则 4）；⑤ AppSecret 放服务器 | ②⑤ 挡住小米联调；③④ 挡住真实可用（**没有 channel 和模板每台设备每天只有 1 条**） | 已读（D1542、D2321）；推送基础服务免费（D1545）；**小米应用商店不接受个人账号**（D1731，指商店）；个人能否开推送、是否要上架**未核实** |
| OPPO | ① 注册 **OPPO 企业开发者**并实名；② 建应用、申请推送（审核 1 个工作日）；③ 通讯类 channel 登记、不限量权益申请（方式未核实）；④ AppKey、AppSecret（客户端）和 MasterSecret（服务端）分开放；⑤ **上架 OPPO 软件商店**才有正式推送权限，没上架只有测试权限（每天 1000 条），上架后要重新申请 | ①② 挡住联调（测试权限）；⑤ 挡住正式使用（VP-4） | 已读（O tid=1906，2020 年旧帖）；政府、企业内部办公类可"不上架申请推送"（摘要，天听是否适用要问）；软著、费用**未核实** |
| vivo | ① 注册 vivo 开放平台账号；② 建应用；③ 推送运营平台提交推送申请（自动审核），拿 APPID / APPKEY / APPSECRET；④ 加测试设备联调；⑤ 上传带推送的 APK 审核；⑥ 工单申请"即时消息不限量"（约 5 个工作日，推送权限"正式"后生效）；⑦ AppSecret 放服务器 | ①～④ 挡住联调（测试推送每天 1000 条）；⑤⑥ 挡住正式使用：**审核中的应用权限是"受限"，只能发测试消息；转正要同步开放平台的上架结果**，实际要上架（VP-4） | 摘要（V281、V12、V735）；**个人开发者上架需要本人的软著和 App 备案**（摘要）；个人能否开推送、费用**未核实** |
| 密钥放服务器 | 每家的服务端密钥写进服务器上的文件，权限 0600，路径配到第 11 节的变量；**不发到聊天里，不进仓库**；AI 只拿路径 | 所有真实通道 | — |
| 签名证书指纹 | 荣耀要 SHA256 指纹；华为是否要指纹未核实 | 荣耀开通 | — |
| 真机 | 五个品牌各一台大陆版（P-13）；荣耀最好一台新机（只支持荣耀推送）一台老机（VP-7） | F-VND-09 | — |
| 法务 | 第 9.2 节问题 | 隐私政策发布、上架 | — |
| 上架准备（如果 VP-4 选上架） | 软著、App 备案、各商店资质；和 CH-23 一起决定 | OPPO、vivo 正式推送 | 未核实 |

**建议现在就开始的**：VP-3 定主体 → 五家账号注册和应用创建（不依赖代码）→ 华为、小米、vivo 的 IM 分类申请（审核要真实的包名和应用，不依赖 V1 代码，但小米模板和 vivo 不限量要先接入分类，可能要等 V1 第一版能发测试消息）。

---

## 17. 拍板项

| # | 问题 | 选项 | 代价 | 建议 |
|---|---|---|---|---|
| **VP-1** | 五个品牌的手机同时有 Play 服务时，用厂商通道还是 FCM？ | ① **厂商优先，FCM 兜底**；② FCM 优先，厂商兜底；③ 按系统区域（大陆版用厂商，海外版用 FCM） | ① 海外版的这些品牌手机也走大陆通道，小米海外要按地区换域名（不在本批），可能收不到（未核实）；② 大陆装了 Play 服务的手机走 FCM，大陆服务器到 FCM 多半不通（R-4），而且大陆系统常限制 FCM 后台唤醒（未核实）；③ 区域判断没有官方统一方法，要每家查 | **①**：目标用户在大陆；海外用户少时接受，后续再做 ③ |
| **VP-2** | V1 做哪几家，什么顺序？ | ① 五家一起；② **分两波：华为、荣耀、小米先（不要求上架或未见上架要求），OPPO、vivo 后（要上架）**；③ 按实际用户机型分布挑 | ① 被 OPPO / vivo 上架卡住时整批拖住；② 第二波可能长期停在测试权限；③ 要先有用户机型数据（现在没有） | **②**；有机型数据后按 ③ 调顺序 |
| **VP-3** | 开发者账号用个人还是企业主体？ | ① **企业**；② 个人；③ 混合（能个人的个人，OPPO 企业） | ① 要营业执照和实名，准备时间长，但小米商店、OPPO 都只认企业，以后上架也要；② 个人在 OPPO 不行，其余几家的服务端接口和分类权益是否对个人开放都**未核实**；③ 主体分散，以后迁移成本未核实 | **①**，和 CH-12 的"公司主体账号长期更合适"一致；没有企业主体时先 ③，OPPO 等企业主体 |
| **VP-4** | OPPO、vivo 正式推送要上架它们的商店，上不上？ | ① 上架（要软著、App 备案、商店审核，公开可下载）；② **暂不上架，用测试权限**（每天 1000 条，测试设备）；③ 不做这两家 | ① 和 CH-23"内部小范围、邀请制"冲突，算不算面向公众要问法务；时间和成本未估；② 只够内部少量设备，**是否允许长期这样用未核实**，可能违反协议；③ 这两家用户收不到离线通知 | **先 ②，同时问法务**；CH-23 改为公众开放时一起走 ① |
| **VP-5** | 只给 AAR 文件的 SDK（小米、OPPO、vivo）怎么进构建？ | ① 提交到仓库 `libs/`；② **发布和 CI 时从 Frank 控制的私有位置注入**，开发和 CI 用空实现；③ 私有 Maven | ① 仓库公开，**再分发许可未核实**；二进制进仓库难审计；② 构建多一步，CI 构建的是空实现，真机包要 Frank 或发布流程注入；③ 要维护仓库 | **②**，和 `google-services.json` 同样的做法；查清许可后可以改 ① |
| **VP-6** | 详情模式在厂商通道开不开？ | ① **开，和 FCM 一样由用户自己打开**；② 厂商通道只有隐藏模式 | ① 摘要经过国内厂商服务器；小米要模板审核，变量能否放聊天内容未核实，不过就退回通用通知；华为可能因"敏感信息"拦截（`80100016`）；② 体验差，但隐私最好、审核最简单 | **①**，提示写明通道名（F-VND-07） |
| **VP-7** | 荣耀老机（不支持荣耀推送、有 HMS Core）怎么办？ | ① **退到 `hms`**；② 只用荣耀推送，不支持就"不支持离线通知" | ① 华为账号里要多管一批设备，HMS 在荣耀机上的长期支持未核实；② 老荣耀机收不到 | **①** |
| **VP-8** | 要不要接各家的回执 / 无效设备回调（小米 type=16、OPPO `regid_invalid`、华为回执 256）？ | ① **V1 不接，只靠发送响应删令牌**；② 接 | ① 失效令牌多占几次调用，华为营销超限只能靠回执识别（但 IM 不受限）；② 每家要一个公网回调地址、验签，约多 1 人日每家（未校准），回调地址要 Frank 配 | **①**，指标显示失效率高再做 |

### 17.1 不用 Frank 决定（已按下面做，想改可以说）

| 问题 | 决定 | 理由 |
|---|---|---|
| 退出时调不调厂商注销 | 不调（F-VND-06 规则 4） | 华为不建议调 `deleteToken`；服务端删令牌已够 |
| 华为鉴权方式 | OAuth 客户端模式，不用 JWT 服务账号 | 只要一个 Secret 文件，和荣耀同一套代码 |
| 折叠 id 的整数派生 | 同一个 HMAC 取前 31 位 | 不新增密钥，隐私口径和 C3 一致 |
| 配额熔断到次日 0 点 | 第 5.2 节 | vivo 官方说超配额当天别再发（摘要）；小米配额按天 |
| 重新注册周期 7 天 | 第 5.2 节 | vivo 14 天失效取一半 |
| 服务端不引入厂商 SDK | 标准库 | 和 BE-457 一致 |

---

## 18. 风险

| # | 风险 | 影响 | 对策 |
|---|---|---|---|
| VR-1 | OPPO、vivo 要上架才能正式推送 | 两家长期只有测试权限 | VP-4；分两波（VP-2） |
| VR-2 | IM 分类权益、私信 channel、模板审核不通过或周期长 | 按营销类算，每天 1～2 条，等于不可用 | V0 就开始申请；AC-VND-09-4 验证；指标里 `quota_exceeded` 告警 |
| VR-3 | 荣耀、OPPO 的文档大部分读不到，错误码、字段未核实 | 实现时返工；令牌失效不删 | 第 19 节；实现前由人在浏览器里核对（AI 只能读到摘要） |
| VR-4 | 小米私信模板新规（2026-07-01 起，存量 12-31 前改造）可能继续变 | 规则过期 | 实现时复查 D2321 |
| VR-5 | 厂商内容风控拦截详情模式（华为 `80100016`；荣耀声明里写了内容风控） | 部分详情通知收不到 | 计数；隐藏模式文案固定 |
| VR-6 | 五个厂商 SDK 收集设备信息 | 隐私投诉、上架审核 | 同意后才初始化；隐私政策清单；法务 |
| VR-7 | HarmonyOS NEXT 用户增多 | 华为新机装不了 APK | 不在本批，单独评估鸿蒙版 |
| VR-8 | 服务器在境外时到大陆厂商的连通性 | 推送失败 | 部署前实测（未核实） |
| VR-9 | 厂商 AAR 再分发许可 | 不能进公开仓库 | VP-5 |

---

## 19. 未核实事项清单（实现前要查；标"人工"的要人在浏览器或后台里看）

| # | 事项 | 现状 | 什么时候 |
|---|---|---|---|
| VN-1 | 华为：没有自分类权益时 IM 消息默认归哪类；个人开发者能否申请；`tag` 长度上限；`ttl` 字段格式；Push 专用的 SDK 合规指南和对开发者隐私政策的要求（中文页 503）；Android 用 v1 接口是推断 | 未读到 | V1 公共开工前 |
| VN-2 | 荣耀（人工）：客户端 SDK 坐标、仓库、配置文件；token 获取和刷新；access token 有效期；分类字段和配额（《消息分类标准》《推送数量管理细则》）；《下行消息响应码》；channel_id；个人能否开通；费用 | 页面读不到，只有摘要 | 荣耀开工前 |
| VN-3 | OPPO（人工）：发送主机；`sign` 编码；`notify_id` 是否存在、替换机制；`category`、`notify_level` 的确切要求；auth_token 失效和频控的返回码；不限量权益怎么申请；3.x SDK 的 Maven 坐标；软著、费用；"不上架申请推送"是否适用天听 | 论坛旧帖（2020 年）和摘要 | OPPO 开工前 |
| VN-4 | 各家点击跳转字段（小米、OPPO、荣耀；vivo `skipContent`）；有效期字段和单位（小米 `time_to_live`、OPPO、vivo、荣耀） | 未逐条查 | 各家开工前 |
| VN-5 | 小米：哪些错误码可重试（官方没标注）；无变量模板的 `template_param` 怎么传；模板变量能否放聊天内容、长度上限；判断设备是否支持的方法；个人能否开推送、是否要上架；HyperOS 要求 | 未读到 | 小米开工前 |
| VN-6 | vivo（人工）：全部条目都是摘要，接口字段、错误码要人工核对原文；官方 Maven；系统是否按分类自动建渠道；iQOO 覆盖；个人能否开推送；费用 | 摘要 | vivo 开工前 |
| VN-7 | 五家费用（只有小米写了"基础服务目前免费"）；各家账号主体要求；软著、App 备案要求 | 大部分未核实 | VP-3 前 |
| VN-8 | 五家 AAR / SDK 的再分发许可 | 没查 | VP-5 前 |
| VN-9 | 海外版系统上厂商通道的行为（VP-1 ①的代价） | 没查 | 有海外用户时 |
| VN-10 | 境外服务器到五家主机的连通性 | 没测 | 部署前 |
| VN-11 | OPPO / vivo 测试权限能否长期用于内部小范围（VP-4 ②） | 没查协议原文 | VP-4 前 |
| VN-12 | 工作量（第 14 节） | 没校准 | C3 结束后按 C3 实际用时回看 |

**本次没有下载任何文件**：OPPO 服务端 API PDF（`storepic.oppomobile.com/.../OPPO推送平台服务端API-V1.9.3.pdf`）、vivo 服务端 API PDF（`swsdl.vivo.com.cn`，PUSH-UPS-API 2.7.0）、小米 SDK 下载页都没有打开，相关内容标"未核实"。

---

## 20. 变更记录

| 版本 | 日期 | 内容 |
|---|---|---|
| v0.1 | 2026-11-10 | 初稿。按 P-42（Frank"C3 后加厂商通道"）起草；五家资料按官方页面核对，读不到的标摘要或未核实；待独立评审和拍板 |
