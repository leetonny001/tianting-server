# PRD：直播聊天室与系统推送（二期）

| 项 | 内容 |
|---|---|
| 版本 | **v0.1（草稿，2026-11-05）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-05 |
| 状态 | 草稿，未评审。第 13 节待拍板项确认、独立评审通过后才能冻结。第 4 节的 push 协议是**建议方案**，定稿前任务 BE-228、BE-229、CS-044、CS-045、FE-353、FE-424、FE-434 继续等待 |
| 对应队列 | [QUEUE](../process/QUEUE.md) Q19 |
| 依据 | [PLAN](../PLAN.md) 第 1 节、2.1～2.3 节；[PRD-core v1.3](PRD-core-v1.3.md) 第 7 节；[PRD-admin v1.0](PRD-admin-v1.0.md) 2.2 节规则 4、F-ADM-50；[PRD-ui v1.0](PRD-ui-v1.0.md) F-UI-44；[任务单](../tasks/backend-admin.md)；三个业务仓库的只读调研（第 2 节） |
| 关联 | 协议 [protocol/README.md](../../protocol/README.md)、Schema `protocol/schema/frame.schema.json`、用例 `protocol/vectors/frames.json`、错误码 `protocol/errors.json`、架构 [ARCH-v1](../arch/ARCH-v1.md) |
| 冻结文档 | 本文不修改任何已冻结 PRD。需要它们配合的地方列在第 11 节，定稿后走变更流程 |
| 阅读提示 | 第 2 节只写事实，推论单独标"推论"，没查到的写"未查到"。标"（L-n）"的规则按第 13 节的建议方案起草，拍板结果不同就随之修改 |

---

## 0. 结论先行

| 问题 | 结论 |
|---|---|
| 这份文档解决什么 | 定义替换腾讯 IM 所需的两块能力：直播聊天室、系统推送；回答冻结 PRD 留下的三个 push 问题；给出三端迁移的需求依据 |
| 离线补收（建议） | 按用户推送写入收件箱，按用户分配递增的 `pushSeq`；客户端每次连接成功后用 `pushSync {afterSeq}` 补拉；默认保留 7 天，到期不再补收。全员、按房间推送只投递在线连接（L-1） |
| `pushId` 去重（建议） | 调用方生成 `pushId`；服务端按"调用方 + pushId"幂等，重复提交不重复投递，同号不同内容返回 409；SDK 再按 `pushId` 去重，消除实时投递和补拉的重叠 |
| 多端语义（建议） | 同一用户的每个在线连接各收一次；每台设备自己保存游标、自己补收；"收到"和"已读"不跨端同步（L-3） |
| 调研最重要的三点 | ① 三端都把"全局推送"做成加入一个共用的群，监听 `onReceiveRESTCustomData`，没有 `pushId` 和补收。② 客户端代码里没有发礼物、官方消息的调用，也没有调用在线人数接口。③ `AVChatRoom` 字样在三个仓库 0 命中，建群和 REST 下发都在业务服务端，**本次未调研** |
| 必须尽快拍板 | **L-12**（fx-sport 是 Flutter，而 PLAN 只规划了 TS、Kotlin、Swift 三套 SDK）、**L-9**（现在靠腾讯的敏感词返回 80001，天听的敏感词还是 P2）、**L-17**（业务方默认没有 `push.all`，现有进球等推送是广播）、**L-11**（新旧版本过渡期怎么切） |
| 规模 | 28 个功能点（聊天室 14、推送 10、迁移 4），AC 共 105 条（聊天室 55、推送 38、迁移 12）；待拍板 17 项 |
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
| 推送不丢 | 保留期内的按用户推送，离线用户上线后 100% 补收；同一设备不重复展示 | F-PUSH-03、F-PUSH-06 的 AC |
| 推送实时 | 在线用户 3 秒内收到（与 AC-ADM-50-1 一致） | AC-PUSH-01-1 |
| 聊天室可用 | 单房 5000 人在线、消息高峰时，服务端不因聊天室断开连接，实时消息 P99 ≤ 1 秒 | 压测，AC-ROOM-08-5 |
| 能力覆盖 | 第 2 节列出的消息类型和推送类型全部有对应方案 | F-MIG-01 对照表评审 |
| 不回归 | 现有 IM 不受影响（按 PRD-social 1.3 节"不回归"口径） | 回归压测 |

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

**设备离线推送（APNs 等）不走腾讯**：ab77 使用友盟 `UMPush`（`AppDelegate+SDK.swift:42-43`、`:160-205`）；发消息时 `offlinePushInfo` 默认空（`FNChatManager.swift:312`、`:396`）。fx-sport、android 的设备推送集成未查到（只在 pubspec 和 `app`、`module-common` 源码里搜索了 `firebase_messaging`、`umeng`、`jpush`、`PushAgent`、`UMConfigure`、`FirebaseMessaging`）。

### 2.9 未查到的部分

以下**不能当作事实使用**，评审前需要找业务服务端或腾讯控制台确认：

| # | 未查到 | 已做的搜索 | 影响 |
|---|---|---|---|
| N-1 | 群类型是否就是 AVChatRoom，谁建群 | `AVChatRoom` 三个仓库 0 命中；客户端只见 `joinGroup`。ab77 注释提到"直播群"（`FNChatManager.swift:266`），不构成证明 | "AVChatRoom"的说法来自 PLAN 第 1 节，未被代码证实；不影响协议设计，影响 F-MIG-02 |
| N-2 | 业务服务端如何发礼物、官方消息和各类推送；禁言推送如何定向 | 业务服务端代码不在范围内 | F-MIG-02、L-15 |
| N-3 | 现网单房在线、全局在线、消息速率的峰值 | 三个仓库里没有 | L-14 容量目标 |
| N-4 | 腾讯侧"进群补发历史"的条数；腾讯第三方回调和敏感词的配置 | 控制台配置不在代码里 | F-ROOM-04 的 50 条是否够用；L-9、L-11 |
| N-5 | 游客（未登录）能否进聊天室 | ab77 的 IM 登录挂在平台登录之后（`FNIMSDKManager.swift:22-25`）；fx-sport、android 进房未见登录判断，但 `genUserSig` 对游客的行为未查到 | L-6 |
| N-6 | 系统消息（type=1）是否真的由 IM 下发 | 三端都有解析代码，ab77、android 又都本地构造 | F-MIG-01 |
| N-7 | android 的 IM SDK 版本 | 未找到版本目录文件 | 低 |
| N-8 | 客户端文本长度、发言频率限制 | 未专门查到 | F-ROOM-06、F-ROOM-08 的默认值 |
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
| F-MIG-01 | 能力对照表与接入指南 | L3 | P0 | 文档 |
| F-MIG-02 | 业务服务端切换需求 | L4 | P0 | 业务方 |
| F-MIG-03 | 灰度与回退 | L4 | P0 | 业务方 / 三端 |
| F-MIG-04 | 三端行为不回归 | L4 | P0 | 三端 |

### 3.2 里程碑

| 里程碑 | 内容 | 过关标准 |
|---|---|---|
| **L1 push 协议** | 第 4 节评审定稿；im-server、sdk-ts、H5 通知；同时解除 BE-228、BE-229、CS-044、CS-045、FE-353、FE-424、FE-434 的阻塞 | F-PUSH 的 AC 和 AC-ADM-50 系列通过 |
| **L2 聊天室** | im-server + sdk-ts；单节点 | F-ROOM 的 P0 AC 通过，压测达标 |
| **L3 其他 SDK** | sdk-kotlin、sdk-swift，以及 Flutter 接入（L-12）；集群实现随 B6 | 共享 `vectors/frames.json` 全部通过 |
| **L4 迁移** | 一端试点 → 三端灰度 → 下线腾讯（L-11、L-13） | F-MIG 的 AC 通过；下线门槛由 Frank 确认 |

依赖：L1 依赖第二批协议兼容规则（PRD-core 第 8 节"三套 SDK 必须忽略未知"）。L2 依赖 L1 的按房间目标定义，以及用户级禁言（F-ADM-40）。L1 与 L2 建议分开冻结（L-16）。

---

## 4. push 协议（建议方案，待评审）

> 本节回答 PRD-admin 2.2 节规则 4 的三个问题。这里所有结论都是建议，协议评审通过后才生效。

### 4.1 语义总表

| 目标 | 进收件箱 | `pushSeq` | 离线补收 | 多端 | scope |
|---|---|---|---|---|---|
| `user`，`ttlSeconds` > 0（默认 7 天） | 是 | 按用户递增 | 有效期内可补收 | 每个连接各一次；每台设备各自补收 | `push.user` |
| `user`，`ttlSeconds` = 0 | 否 | 无 | 无 | 当前在线的每个连接各一次 | `push.user` |
| `all` | 否 | 无 | 无（L-1） | 当前在线的每个连接各一次 | `push.all` |
| `room` | 否 | 无 | 无 | 当前在该房的每个连接各一次 | `push.room` |

### 4.2 帧与接口

下行（在现有 `PushFrame` 上扩展；现有帧的 `additionalProperties: false` 需要显式加字段，`pushId` 改为必填）：

```jsonc
{ "cmd": "push", "data": {
    "pushId": "p1",            // 必填，1～64 位 [A-Za-z0-9_-]，调用方生成
    "type": "notice",          // 必填，业务类型，服务端不解释
    "data": { "text": "…" },   // 必填，业务负载，序列化后 ≤ 4KB
    "ts": 1730000000000,       // 必填，服务端受理时间
    "target": "user",          // 必填：user | all | room
    "pushSeq": 42,             // 只有进收件箱的推送才有
    "roomId": "match-123"      // 只有 target=room 才有
} }
```

上行（新增命令）：

```jsonc
// 请求
{ "id": "r9", "cmd": "pushSync", "data": { "afterSeq": 41, "limit": 50 } }
// 应答：按 pushSeq 升序，只含未过期的
{ "id": "r9", "cmd": "ack", "data": { "pushes": [ /* 同 push.data */ ], "hasMore": false, "latestSeq": 42 } }
```

管理接口（scope、错误码沿用 F-ADM-50 规则 8，增加 `roomId`、`ttlSeconds`）：

```jsonc
POST /admin/push
{ "target": "user|all|room", "userIds": ["u1"], "roomId": "match-123",
  "type": "notice", "data": { }, "pushId": "p1", "ttlSeconds": 604800 }
// 应答：accepted 只在 target=user 时出现，等于目标用户数；不返回在线人数
{ "pushId": "p1", "created": true, "accepted": 1 }
```

### 4.3 问题一：离线补收

| 环节 | 规则 |
|---|---|
| 写入 | 收件箱 `push_inbox(user_id, push_seq, push_id, type, data, created_at, expires_at)`，`(user_id, push_seq)` 与 `(user_id, push_id)` 都唯一。一次提交的所有用户在**同一事务**写入 |
| 序号 | 每个用户独立递增，分配方式与会话 `seq` 一致；连续、不留空洞。只有进收件箱的推送才占序号 |
| 投递 | 事务提交后向在线连接实时下发。提交后、下发前崩溃时，由下一次补拉兜底（4.6 节已接受风险） |
| 补拉时机 | SDK **每次连接成功**（`auth` 之后）调一次 `pushSync`；`hasMore=true` 时继续；实时帧发现 `pushSeq` 大于"游标 + 1"时也补拉 |
| 游标 | 由客户端保存，按 userId 区分。原生端持久化；H5 没有持久化（PRD-ui D4），每次新会话从 0 开始（L-4） |
| 到期 | **读取时按 `expires_at` 过滤**，不依赖定时清理；BE-220 的定时任务只负责回收空间。`afterSeq` 早于保留期不报错，返回还在有效期内的部分 |
| 被封禁 | 封禁期间连不上，收不到；解封后在有效期内可补收 |

### 4.4 问题二：`pushId` 去重

两层，解决不同的问题：

| 层 | 解决 | 规则 |
|---|---|---|
| 服务端幂等 | 调用方重试（包括 console-server 的 5040 结果未知） | 键为"调用方 + `pushId`"。同键同内容（`target`、目标、`type`、`data` 的 SHA-256 相同）：返回首次结果，`created=false`，不重复投递。同键不同内容：409 / 4094。幂等记录至少保留 7 天 |
| SDK 去重 | 实时帧和 `pushSync` 重叠、重连重发 | 同一 SDK 实例按 `pushId` 去重，近 2000 条 LRU；原生端随游标持久化。**不跨设备去重** |

### 4.5 问题三：多端语义

1. 同一用户的每个在线连接各收一次，包括同一台电脑的多个标签页。
2. 每台设备独立补收：A 设备在线收到，不影响离线的 B 设备上线后补收（L-3）。
3. 服务端不记录"谁收到了"，没有回执，也不跨端同步已读。天听"未查看条数"只统计当前会话（F-UI-44 规则 3 不变）。
4. 补拉到的推送，SDK 标记 `replayed=true`；实时帧为 `false`。是否弹提示由业务决定（L-4）。

### 4.6 已考虑的备选与已接受的风险

| 方案 | 不选的原因 |
|---|---|
| 服务端按用户记"已投递"标记 | 第一台设备收到后，第二台设备就补不到，违反多端语义 |
| 服务端按设备记录游标 | 需要设备身份和回执，复杂度高；现有消息模型也是客户端游标（PLAN 2.1 节） |
| 把推送当系统账号的会话消息 | 复用 `seq` 和 `sync`，但会污染会话列表和未读，正是现有 App 要压制的问题（`push_notifier.dart:182-197`） |
| 全员推送写扩散到收件箱 | 单次写入量等于用户总数，代价高；需要时走 L-1 的读扩散备选 |

已接受的风险：① 全员和按房间推送在连接断开的短暂窗口内会丢（L-1）。② 提交后、下发前进程崩溃时，一直在线的用户要等到下一次推送或重连才补到。

### 4.7 F-ADM-50 草案 AC 对齐

按本节建议方案，PRD-admin 的 AC 无需改预期，只明确实现路径。定稿时在 PRD-admin 变更记录里标注：

| 草案 AC | 对齐结果 | 对应 |
|---|---|---|
| AC-ADM-50-1 在线 3 秒内收到 | 保留。帧按 4.2 节，补 `pushId`、`ts`、`target` | AC-PUSH-01-1 |
| AC-ADM-50-2 第 6 天补收、8 天后收不到 | 保留，前提是默认 `ttlSeconds` = 7 天（L-2）。"收到"指 `pushSync` 返回；到期按读取时过滤，不等定时清理 | AC-PUSH-03-1、03-2 |
| AC-ADM-50-3 全员：离线上线后收不到 | 保留，前提是 L-1 选建议方案；选备选时这条要改 | AC-PUSH-04-1 |
| AC-ADM-50-4 模板 Schema、4KB | 保留。Schema 校验在 console-server；im-server 另校验 4KB | AC-PUSH-02-2 |
| AC-ADM-50-5 全员间隔 5 分钟 | 保留，**只约束 console-server**；im-server 对业务调用方另有限速（F-PUSH-09、L-17） | AC-PUSH-09-2 |
| AC-ADM-50-6 ghost、1001 | 保留 | AC-PUSH-02-3、02-4 |
| AC-ADM-50-7 不能自批 | 不受影响 | — |
| AC-ADM-50-8 发送记录与审计 | 不受影响；`biz.push_records.im_push_id` 就是本协议的 `pushId` | AC-PUSH-09-1 |
| 规则 3"`pushId` 相同由 SDK 去重" | 明确为**同一 SDK 实例内**，不跨设备 | AC-PUSH-07-1 |
| 规则 8 | 增加 `target=room` + `roomId`、`ttlSeconds` | AC-PUSH-05-1 |

---

## 5. 功能需求：聊天室

> 【后端】指 im-server；【SDK】指 sdk-ts（其他 SDK 跑同一批 vectors）；未标注的为端到端。负向断言沿用 PRD-core 6.1 节"2 秒内收不到任何帧"。[IM 钟] 指测试时钟。

#### F-ROOM-01 房间管理接口（L2，P0）

**规则**
1. 房间显式创建（L-8）：`POST /admin/rooms {roomId, name?}`，scope `room.write`。`roomId` 格式与 groupId 相同；已存在且未关闭时幂等返回。
2. `GET /admin/rooms/{id}` 返回 `{roomId, name, status, online, createdAt}`，scope `room.read`。
3. 关闭：`DELETE /admin/rooms/{id}`，scope `room.write`。全部在房连接收到 `roomClosed {roomId, reason}`，之后不能进房。关闭后 `roomId` 可以再次创建（比赛重开、主播再开播）。
4. 房间元数据持久化；成员和消息不持久化（PLAN 2.1 节，L-5）。

**验收标准**
- AC-ROOM-01-1【后端】：输入 不存在的 `match-1`；操作 创建两次；预期 两次都成功，只有一个房间；错误码 —。
- AC-ROOM-01-2【后端】：输入 非法 `roomId`（含空格）；操作 创建；预期 400；错误码 4004。
- AC-ROOM-01-3：输入 u1、u2 在 `match-1`；操作 关闭房间；预期 两人都收到 `roomClosed`；u1 再次 `joinRoom` 失败；错误码 4047。
- AC-ROOM-01-4【后端】：输入 已关闭的 `match-1`；操作 再次创建，u1 进房；预期 成功，`recent` 为空；错误码 —。
- AC-ROOM-01-5【后端】：输入 只有 `room.read` 的调用方；操作 创建房间；预期 403；错误码 4030。

#### F-ROOM-02 进出房间（L2，P0）

**规则**
1. `joinRoom {roomId}` 的 ack 为 `{online, recent}`（与 `frames.json:26` 一致）。
2. **一个连接同一时间只在一个房间**：已在其他房间时，服务端先退旧房再进新房，不报错。依据：三端都是一次一个业务房（2.3 节）。
3. 重复进入同一房间幂等，重新返回当前的 `online` 和 `recent`。
4. `leaveRoom {roomId}` 幂等：不在该房也返回成功。连接断开等同退房。
5. 同一用户的不同连接可以在不同房间。进出房间**不向其他成员广播**（三端都没有监听成员进出，2.6 节）。
6. 被禁言的用户可以进房、收消息。进房不写会话列表，不计未读（PLAN 2.1 节）。
7. 只有登录用户可以进房（L-6）。

**验收标准**
- AC-ROOM-02-1：输入 `match-1` 已有 u1；操作 u2 进房；预期 ack 带 `online=2` 和 `recent`；u1 收不到"进房"帧（2 秒窗口）；错误码 —。
- AC-ROOM-02-2【后端】：输入 不存在的房间；操作 `joinRoom`；预期 ack 错误；错误码 4047。
- AC-ROOM-02-3【后端】：输入 同一连接在 `room-a`；操作 进 `room-b`，然后 u9 在 `room-a` 发言；预期 进房成功，该连接收不到 u9 的消息；`room-a` 在线数减 1；错误码 —。
- AC-ROOM-02-4【后端】：输入 同一连接；操作 连续两次进 `match-1`，再对 `match-2` 执行 `leaveRoom`；预期 都成功，`online` 不重复计数；错误码 —。
- AC-ROOM-02-5【后端】：输入 u1 的 web 在 `room-a`、pc 在 `room-b`；操作 分别有人发言；预期 两个连接各收各房的消息；错误码 —。
- AC-ROOM-02-6【后端】：输入 u1 在房内；操作 直接断开连接；预期 其他人下一次 `roomOnline` 不再计入 u1；错误码 —。
- AC-ROOM-02-7：输入 u1 在房内；操作 查 `convs`；预期 不出现 `room:` 会话，未读无变化；错误码 —。

#### F-ROOM-03 断线重连后自动回房（L2，P0）

**规则**
1. SDK 记住当前房间；重连并 `auth` 成功后自动重新 `joinRoom`。重复的 `recent` 按 `msgId` 去重。
2. 回房失败（4047）时通知业务代码"房间已关闭"，不再重试。
3. 调用 `leaveRoom` 或退出登录后不再自动回房。依据：三端都需要自己处理重连和切房的时序（`FNChatManager.swift:166-170`、`chat_room_notifier.dart:206-222`），放进 SDK 后业务不必重复。

**验收标准**
- AC-ROOM-03-1【SDK】：输入 已在 `match-1`，`FakeTransport`；操作 断开重连；预期 自动发 `joinRoom`；业务收到的 `recent` 不含已展示过的 `msgId`；错误码 —。
- AC-ROOM-03-2【SDK】：输入 断线期间房间被关闭；操作 重连；预期 业务收到房间关闭回调，不再重试；错误码 4047。
- AC-ROOM-03-3【SDK】：输入 已调用 `leaveRoom`；操作 断开重连；预期 不发 `joinRoom`；错误码 —。

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
2. `roomOnline {roomId, online}`（`frames.json:45`）向全房广播，最多每 5 秒一次，人数没变不发。
3. 三端现在都没有使用房内人数（2.5 节），这项能力是 PLAN 的 P0；列表页人数继续由业务接口提供，也可以调 `GET /admin/rooms/{id}`。

**验收标准**
- AC-ROOM-05-1【后端】：输入 u1 用 web、pc 两个连接在房内，u2 一个连接；操作 u3 进房看 ack；预期 `online=3`；错误码 —。
- AC-ROOM-05-2【后端】[IM 钟]：输入 u1 在房内；操作 5 秒内 10 个用户进房；预期 这 5 秒内 u1 最多收到 1 条 `roomOnline`，下一条人数正确；错误码 —。
- AC-ROOM-05-3【后端】[IM 钟]：输入 人数不变；操作 推进 60 秒；预期 没有 `roomOnline`；错误码 —。

#### F-ROOM-06 文本消息收发（L2，P0）

**规则**
1. `send {convType: "room", to: roomId, clientMsgId, type: "text", body: {text}, ext?}`；文本规则同 F-MSG-01（1～5000 码点）。必须在房内，否则 4041。
2. ack 为 `{msgId, convId, ts}`，**没有 `seq`**。下行 `msg` 增加 `fromNick`（发送时的昵称，服务端写入）和可选 `ext`。
3. `ext` 是 JSON 对象，序列化后 ≤ 1KB，服务端原样转发，用来承载 `vipLevel` 等（对应 `cloudCustomData`，2.4 节）。发言门槛由业务方决定，天听不判断。
4. 发送方的**发送连接不回显**（靠 ack）；同一用户在同房的其他连接会收到。依据：三端都在发送成功后本地加入列表（2.4 节）。
5. 去重：`(from, clientMsgId)` 在内存保留 5 分钟，重发返回原 ack。

**验收标准**
- AC-ROOM-06-1：输入 u1、u2 在房内；操作 u1 发文本，`ext={vipLevel:3}`；预期 u2 1 秒内收到，带 `from`、`fromNick`、`ext`、`msgId`、`ts`；u1 只收到 ack，没有 `msg`；错误码 —。
- AC-ROOM-06-2【后端】：输入 u1 不在房内；操作 发送；预期 ack 错误；错误码 4041。
- AC-ROOM-06-3【后端】：输入 同一 `clientMsgId`；操作 u1 发两次；预期 ack 相同，u2 只收到一条；错误码 —。
- AC-ROOM-06-4【后端】：输入 `ext` 超过 1KB、或 5001 码点文本；操作 发送；预期 都被拒绝；错误码 4000、4006。
- AC-ROOM-06-5【后端】：输入 u1 的 web、pc 同在房内；操作 web 发言；预期 pc 收到，web 不收到；错误码 —。

#### F-ROOM-07 自定义消息与服务端下发（L2，P0，L-10）

**规则**
1. `type: "custom"`，`body: {customType: 整数, data: 对象}`，序列化后 ≤ 4KB。天听不解释内容。
2. 客户端可以发自定义消息，用于晒单（`customType=3`）；服务端可通过配置限定客户端允许的 `customType`，默认只允许 3（礼物、官方消息不能由客户端伪造）。
3. 服务端下发：`POST /admin/rooms/{id}/messages {type, body, ext?, fromUserId?, clientMsgId}`，scope `room.message`，用于礼物（5）、官方（2）、系统（1）。`fromUserId` 存在时，`from`、`fromNick` 取该用户（送礼人）；否则 `from` 为空。**不要求发送者在房内**。
4. 服务端下发的消息进入 `recent`，并享有 F-ROOM-08 的高优先级。
5. **SDK 必须把不认识的 `customType` 原样交给业务**，不丢弃、不报错。业务层统一处理三端现在的差异（2.4 节）。

**验收标准**
- AC-ROOM-07-1：输入 u1、u2 在房内；操作 u1 发 `customType=3` 的晒单；预期 u2 收到 `body` 原样；错误码 —。
- AC-ROOM-07-2【后端】：输入 默认配置；操作 u1 发 `customType=5`；预期 拒绝；错误码 4030。
- AC-ROOM-07-3【后端】：输入 不在房内的 u5 送礼；操作 业务方调下发接口，`fromUserId=u5`；预期 房内收到 `customType=5`，`from=u5`、`fromNick` 正确；新进房的人在 `recent` 看到；错误码 —。
- AC-ROOM-07-4【后端】：输入 `fromUserId` 不存在；房间不存在；操作 分别下发；预期 都是 404；错误码 4042、4047。
- AC-ROOM-07-5【SDK】：输入 `customType=99`；操作 收到；预期 业务回调拿到原始 `customType` 和 `data`；错误码 —。
- AC-ROOM-07-6【后端】：输入 `body` 超过 4KB；操作 发送、下发；预期 都拒绝；错误码 4000。

#### F-ROOM-08 限流与下行保护（L2，P0）

**规则**
1. 上行：沿用用户 20 条/秒（F-MSG-07），并增加单用户单房 2 条/秒（可配置，`IM_ROOM_USER_RATE`），超出 4290。
2. 下行：每个房间按 200ms 窗口汇集后广播；每个连接的聊天室消息上限 20 条/秒（可配置）。超出时**丢弃普通文本和客户端自定义消息**，只保留最新的；服务端下发的消息不因这条规则丢弃。不增加批量帧（PLAN 提到的"合并"指合并发送窗口，帧格式不变）。
3. 慢客户端：下行队列积压时**先丢聊天室消息**，再考虑现有 4400 断开规则；单聊、群聊、`push` 不被聊天室拖累。
4. 丢弃不补发，也不通知客户端。聊天室消息没有 `seq`，这是预期行为。

**验收标准**
- AC-ROOM-08-1【后端】：输入 u1 在房内；操作 1 秒内发 5 条；预期 前 2 条成功，其余 4290；错误码 4290。
- AC-ROOM-08-2【后端】：输入 100 个用户在房内；操作 合计发送 100 条/秒，持续 10 秒；预期 任一连接每秒收到 ≤ 20 条；连接都不断开；错误码 —。
- AC-ROOM-08-3【后端】：输入 与 08-2 相同的高峰；操作 同时下发 5 条礼物；预期 所有未读满的连接都收到这 5 条；错误码 —。
- AC-ROOM-08-4【后端】：输入 注入一个不读数据的连接，同一用户另一连接在群聊；操作 房内高峰持续 30 秒，并发一条群消息；预期 另一连接的群消息 1 秒内到达；聊天室消息被丢弃不影响群消息；错误码 —。
- AC-ROOM-08-5【后端】：输入 单房 5000 用户在线（L-14）；操作 持续 100 条/秒上行，10 分钟；预期 未被限流的消息 P99 ≤ 1 秒；意外断开 0；错误码 —。

#### F-ROOM-09 禁言在聊天室生效（L2，P0）

**规则**
1. 沿用用户级禁言（F-ADM-40 规则 1 已包含"二期聊天室"）：房内发送返回 4031，`details.scope=user`；检查在限流之前。
2. 依据：现有禁言是全局生效（`chat_room_notifier.dart:89`），不是按房间，所以**二期不增加房间级禁言**；需要时作为 P1。
3. 客户端配合沿用 `muted` / `unmuted` 事件和 `auth` ack 的 `mutedUntil`，不再需要 contentType=1 推送。
4. 谁能禁言由业务服务端判断，天听不建房间角色（2.7 节）。

**验收标准**
- AC-ROOM-09-1：输入 u1 在房内，被用户级禁言；操作 发送文本、晒单；预期 都被拒绝，`details.scope=user`；u1 仍能收到其他人的消息；错误码 4031。
- AC-ROOM-09-2【后端】：输入 被禁言的 u1；操作 进房；预期 成功；错误码 —。
- AC-ROOM-09-3【SDK】：输入 u1 在房内；操作 解除禁言后再发送；预期 收到 `unmuted`，发送成功；错误码 —。

#### F-ROOM-10 删除聊天室消息（L2，P0）

**规则**
1. `POST /admin/rooms/{id}/messages/delete {msgIds, reason}`，scope `msg.delete`，单次 ≤ 100。
2. 从缓冲移除，向全房广播 `msgDeleted {convId, msgIds}`，**没有 `seqs`**（现有 Schema 要求必填，需要改为条件必填）。
3. 消息已不在缓冲时**也广播**，并返回成功：客户端列表里可能还有。
4. 现有 App 的"进房取已删 ID 列表"不再需要，因为 `recent` 已经排除；contentType=3 推送也不再需要。
5. 聊天室消息不持久化，PRD-admin F-ADM-32 的原文隔离不适用（L-5）。

**验收标准**
- AC-ROOM-10-1：输入 u1、u2 在房内，一条消息 m1；操作 业务方删除 m1；预期 两人 3 秒内收到 `msgDeleted`，业务回调拿到 m1；错误码 —。
- AC-ROOM-10-2【后端】：输入 已被挤出缓冲的 m0；操作 删除；预期 成功并广播；错误码 —。
- AC-ROOM-10-3【后端】：输入 101 个 `msgIds`；操作 删除；预期 400；错误码 4131。
- AC-ROOM-10-4【后端】：输入 不存在的房间；操作 删除；预期 404；错误码 4047。

#### F-ROOM-11 内容拦截（L2，P0，L-9）

**规则**
1. 房内文本发送前检查敏感词。命中拦截时返回 4510，消息不广播、不进缓冲。词库和规则沿用 PRD-admin F-ADM-60（该功能点目前是 P2，本条要求提前，影响见 L-9）。
2. 校验前做 NFKC + 去变音符，防止绕过。依据：现有客户端自己做（2.7 节）；放到服务端后三端行为一致。
3. 客户端提示沿用现有"消息包含敏感词，发送失败"（`chat_room_notifier.dart:610`）。

**验收标准**
- AC-ROOM-11-1【后端】：输入 拦截词"违规词"；操作 房内发送含该词的文本；预期 被拒绝，其他人 2 秒内收不到，`recent` 没有；错误码 4510。
- AC-ROOM-11-2【后端】：输入 同上；操作 发送全角或带变音符的变体；预期 同样被拦截；错误码 4510。
- AC-ROOM-11-3【SDK】：输入 服务端返回 4510；操作 发送；预期 SDK 给业务一个可识别的错误，消息标记失败；错误码 4510。

#### F-ROOM-12 SDK 聊天室接口（L2 sdk-ts，L3 其他，P0）

**规则**
1. 接口对齐 PLAN 3.2 节：`joinRoom(roomId)`、`leaveRoom()`、`sendRoomText`、`sendRoomCustom`；事件流：消息、`roomOnline`、`msgDeleted`、`roomClosed`。
2. 进房结果把 `recent` 交给业务，不写本地数据库。
3. 换房串行：连续调用时只保留最后一个目标，过时的结果不通知（三端现在各自实现，2.3 节）。

**验收标准**
- AC-ROOM-12-1【SDK】：输入 `FakeTransport`；操作 跑 `frames.json` 的房间用例；预期 全部通过；错误码 —。
- AC-ROOM-12-2【SDK】：输入 连续调用 `joinRoom(a)`、`joinRoom(b)`、`joinRoom(c)`；操作 ack 乱序返回；预期 最终状态是 c，a、b 的结果不通知业务；错误码 —。
- AC-ROOM-12-3【SDK】：输入 进房成功；操作 查本地数据库；预期 没有房间消息；错误码 —。

#### F-ROOM-13 聊天室可观测（L2，P1）

**规则**：指标：房间数、房内连接数、消息数、丢弃数（按原因）。**房间 ID 不作为指标标签**，防止基数爆炸；日志不写正文，只写 `userId` 和 `roomId`（PRD-core 第 6 节）。

**验收标准**
- AC-ROOM-13-1【后端】：输入 触发下行丢弃；操作 读 `/metrics`；预期 丢弃数增加，标签中没有 `roomId`；错误码 —。
- AC-ROOM-13-2【后端】：输入 发送含特征串的文本；操作 扫描日志；预期 搜不到特征串；错误码 —。

#### F-ROOM-14 H5 聊天室演示页（L2，P1）

**规则**：PLAN 3.3 节的模拟比赛直播间：在线人数 + 消息流 + 推送横幅。文本一律按纯文本渲染（F-MSG-06）。它和天听正式导航的关系待 PRD-ui 变更时确定。

**验收标准**
- AC-ROOM-14-1：输入 两个标签页进同一房间；操作 互发消息；预期 互相可见，人数显示 2；错误码 —。
- AC-ROOM-14-2【前端】：输入 文本 `<img src=x onerror=alert(1)>`；操作 显示；预期 原样显示为文本，不弹窗；错误码 —。

---

## 6. 功能需求：系统推送

> 协议细节见第 4 节。

#### F-PUSH-01 下行 `push` 帧（L1，P0）

**规则**：字段见 4.2 节。服务端不解释 `type` 和 `data`。三套 SDK 忽略不认识的字段（PRD-core 第 8 节兼容规则）。

**验收标准**
- AC-PUSH-01-1：输入 u1 在线；操作 按用户推送 `type=notice`；预期 3 秒内收到，`pushId`、`type`、`data`、`ts`、`target=user`、`pushSeq` 都正确；错误码 —。
- AC-PUSH-01-2【后端】：输入 `frames.json` 新增的 user、all、room 三种帧；操作 用 Schema 校验；预期 全部通过；缺 `pushId` 的帧校验失败；错误码 —。
- AC-PUSH-01-3【SDK】：输入 帧中有未知字段 `foo`；操作 收到；预期 正常交给业务；错误码 —。

#### F-PUSH-02 推送管理接口（L1，P0）

**规则**
1. 接口见 4.2 节，只在管理地址提供，走 F-KEY 的签名和 scope。
2. `data` 序列化后 ≤ 4KB，否则 400 / 4000。`type` 1～64 字符。`pushId` 格式不符 400 / 4000。
3. `userIds` ≤ 1000，超出 400 / 4131；有不存在的用户：404 / 4042，`details.userIds` 列出，**整批不发**。
4. `ttlSeconds` 取值 0～604800，默认 604800；`target` 不是 `user` 时不接受非 0 值，否则 400 / 4000。
5. 模板 Schema 校验只在 console-server（F-ADM-50 规则 2）；业务方直接调用时，im-server 不校验模板。

**验收标准**
- AC-PUSH-02-1【后端】：输入 合法的三种 `target`；操作 分别调用；预期 200，`created=true`；`target=user` 时 `accepted` 等于目标用户数，其他目标没有该字段；错误码 —。
- AC-PUSH-02-2【后端】：输入 `data` 为 4097 字节；操作 调用；预期 400，不投递；错误码 4000。
- AC-PUSH-02-3【后端】：输入 `userIds=[u1, ghost1, ghost2]`；操作 调用；预期 404，`details.userIds=[ghost1, ghost2]`，u1 收不到；错误码 4042。
- AC-PUSH-02-4【后端】：输入 1001 个 userId；操作 调用；预期 400；错误码 4131。
- AC-PUSH-02-5【后端】：输入 `target=all, ttlSeconds=3600`；`ttlSeconds=604801`；操作 分别调用；预期 都是 400；错误码 4000。
- AC-PUSH-02-6【后端】：输入 只有 `push.user` 的调用方；操作 `target=all`；预期 403；错误码 4030。

#### F-PUSH-03 按用户推送与离线补收（L1，P0）

**规则**：见 4.3 节。`pushSync` 的 `limit` 1～100，默认 50，超范围 400 / 4000。只能拉自己的收件箱。

**验收标准**
- AC-PUSH-03-1【后端】[IM 钟]：输入 u3 离线；操作 推送后第 6 天上线并 `pushSync {afterSeq:0}`；预期 收到；错误码 —。
- AC-PUSH-03-2【后端】[IM 钟]：输入 u4 离线，定时清理被关闭；操作 推送后推进 8 天，`pushSync`；预期 收不到，证明靠读取时过滤；错误码 —。
- AC-PUSH-03-3【后端】：输入 u1 有 120 条未过期推送；操作 `limit=50` 连续翻页；预期 共 120 条，升序，无重复；`hasMore` 最后一页为 false；错误码 —。
- AC-PUSH-03-4【SDK】：输入 游标为 41，`FakeTransport`；操作 收到 `pushSeq=44` 的实时帧；预期 触发 `pushSync {afterSeq:41}`；业务按 42、43、44 收到，不重复；错误码 —。
- AC-PUSH-03-5【后端】：输入 `ttlSeconds=0`；操作 向离线的 u1 推送，u1 上线 `pushSync`；预期 收不到；帧中没有 `pushSeq`；错误码 —。
- AC-PUSH-03-6【后端】：输入 `limit=101`；操作 `pushSync`；预期 400；错误码 4000。
- AC-PUSH-03-7【SDK】：输入 连接建立；操作 `auth` 成功；预期 SDK 自动发一次 `pushSync`；错误码 —。

#### F-PUSH-04 全员推送（L1，P0）

**规则**
1. 只投递当前在线连接（L-1）；不进收件箱。
2. 集群下经消息总线广播（B6）；单节点遍历连接，分批限速下发，不阻塞发送路径。
3. console-server 的双人复核和 5 分钟间隔不变（F-ADM-50 规则 5）。

**验收标准**
- AC-PUSH-04-1：输入 5 个用户在线、u9 离线；操作 全员推送；预期 5 人收到；u9 上线 `pushSync` 收不到；错误码 —。
- AC-PUSH-04-2【后端】：输入 1 万连接在线；操作 全员推送一条；预期 P99 ≤ 5 秒全部送达；发送中单聊消息 P99 仍 ≤ 1 秒；错误码 —。
- AC-PUSH-04-3【后端】：输入 全员推送；操作 查收件箱；预期 没有新增记录；错误码 —。

#### F-PUSH-05 按房间推送（L2，P0）

**规则**：`target=room` + `roomId`，scope `push.room`；只投递当前在该房的连接；不进 `recent`，与聊天室消息的区别就在这里。下行丢弃规则对它不生效。

**验收标准**
- AC-PUSH-05-1：输入 u1 在 `room-a`，u2 在 `room-b`；操作 向 `room-a` 推送；预期 u1 收到，`target=room`、`roomId=room-a`；u2 收不到；错误码 —。
- AC-PUSH-05-2【后端】：输入 不存在的房间；操作 推送；预期 404；错误码 4047。
- AC-PUSH-05-3【后端】：输入 向 `room-a` 推送后；操作 u3 进房看 `recent`；预期 没有这条；错误码 —。

#### F-PUSH-06 `pushId` 幂等与去重（L1，P0）

**规则**：见 4.4 节。

**验收标准**
- AC-PUSH-06-1【后端】：输入 同一调用方、同一 `pushId`、同样内容；操作 提交两次；预期 第二次 200，`created=false`；u1 只收到一次，收件箱只有一条；错误码 —。
- AC-PUSH-06-2【后端】：输入 同一 `pushId`，不同 `data`；操作 提交；预期 409，不投递；错误码 4094。
- AC-PUSH-06-3【后端】：输入 两个调用方用相同 `pushId`；操作 各提交一次；预期 两条都成功，各自投递；错误码 —。
- AC-PUSH-06-4【后端】：输入 全员推送；操作 用同一 `pushId` 重试；预期 不再次广播；错误码 —。
- AC-PUSH-06-5【SDK】：输入 同一 `pushId` 先经实时帧、又经 `pushSync` 到达；操作 处理；预期 业务只收到一次；错误码 —。
- AC-PUSH-06-6【后端】：输入 注入 im-server 延迟，console-server 调用超时（5040），实际已投递；操作 对账后用同一 `im_push_id` 重放；预期 用户只收到一次；错误码 —。

#### F-PUSH-07 多端语义（L1，P0）

**规则**：见 4.5 节。

**验收标准**
- AC-PUSH-07-1：输入 u1 的 web、pc 都在线；操作 推送；预期 两个连接都收到，`pushId` 相同，两端的业务回调各触发一次；错误码 —。
- AC-PUSH-07-2：输入 u1 的 web 在线、android 离线；操作 推送，web 收到后 android 上线；预期 android 补收到；错误码 —。
- AC-PUSH-07-3【SDK】：输入 u1 退出登录，u2 在同一设备登录；操作 `pushSync`；预期 从 u2 自己的游标开始，不沿用 u1 的；错误码 —。
- AC-PUSH-07-4【后端】：输入 u1 拉取 `pushSync`；操作 尝试读 u2 的推送（接口没有目标用户参数）；预期 只返回 u1 的；错误码 —。

#### F-PUSH-08 SDK `pushes` 流（L1 sdk-ts，L3 其他，P0）

**规则**
1. 流中每项为 `{pushId, type, data, ts, target, roomId?, replayed}`。按 `type` 分发由业务代码负责。
2. 业务代码抛异常不影响后续推送。现有 App 的解析失败处理各自实现（2.8 节）。
3. **SDK 不把 `data` 写入日志**。现有 App 把全文写进日志（2.8 节），其中可能含账号。

**验收标准**
- AC-PUSH-08-1【SDK】：输入 实时帧和 `pushSync` 结果各一条；操作 处理；预期 实时项 `replayed=false`，补拉项 `replayed=true`；错误码 —。
- AC-PUSH-08-2【SDK】：输入 业务回调抛异常；操作 再收一条；预期 第二条正常交付；错误码 —。
- AC-PUSH-08-3【SDK】：输入 `data` 含特征串；操作 开启调试日志，处理推送；预期 日志里搜不到特征串；错误码 —。
- （引用）PRD-ui AC-UI-44-1～3 继续有效；回放展示见 L-4。

#### F-PUSH-09 推送审计与限速（L1，P0）

**规则**
1. im-server `admin_audit`：只记目标数量、`type`、`data` 长度和 SHA-256（BE-228 第 3 条）。
2. 限速：`target=all` 默认 20 次/分钟（按调用方，可配置），超出 429 / 4290。console-server 的 5 分钟间隔是另外一层（L-17）。
3. 业务调用方默认没有 `push.all`（PRD-admin F-KEY-02 规则 5）；授予由 L-17 决定。

**验收标准**
- AC-PUSH-09-1【后端】：输入 一次推送；操作 查 `admin_audit`；预期 有长度和 SHA-256，没有 `data` 明文；错误码 —。
- AC-PUSH-09-2【后端】：输入 持有 `push.all` 的调用方；操作 1 分钟内第 21 次全员推送；预期 429；错误码 4290。

#### F-PUSH-10 推送可观测（L1，P1）

**规则**：指标：推送数（按 `target`）、投递数、`pushSync` 次数和耗时、收件箱行数。`type` 可能由调用方任意填写，不作为指标标签。

**验收标准**
- AC-PUSH-10-1【后端】：输入 各 `target` 推送一次；操作 读 `/metrics`；预期 计数正确，标签没有 `type`、`userId`；错误码 —。

---

## 7. 功能需求：迁移

> 业务服务端和三个 App 的改造不在 im-mvp 仓库。本节定义天听要提供什么、业务方要改什么、怎样算迁移完成。责任边界见 L-13。

#### F-MIG-01 能力对照表与接入指南（L3，P0）

**规则**：`docs/` 下提供对照文档；首版内容如下，评审时补全。

| 现在（腾讯 IM + 业务接口） | 天听 |
|---|---|
| `initSDK`、`login(userID, userSig)`（2.2 节） | SDK 初始化 + 连接；`userSig` → 业务后端用 `token.issue` 签发的 token（F-AUTH-01） |
| `onUserSigExpired`、`onKickedOffline` | `kicked` 事件 + 关闭码（F-AUTH-03） |
| `joinGroup`、`quitGroup`（10013 特判） | `joinRoom`、`leaveRoom`（幂等，不需要特判） |
| `setGroupReceiveMessageOpt` 免打扰、清未读、压角标 | 删除：房间不写会话，不计未读 |
| 进群补发历史 | `joinRoom` ack 的 `recent` |
| `createTextMessage` + `cloudCustomData` | `sendRoomText(text, ext)` |
| `createCustomMessage({type, data})` | `sendRoomCustom(customType, data, ext)` |
| `msg.nickName`、`sender`、`timestamp`、`msgID` | `fromNick`、`from`、`ts`、`msgId` |
| `onRecvNewMessage` + 群 ID 过滤 | SDK 房间消息流，不需要过滤 |
| 错误码 80001 | 4510（F-ROOM-11） |
| 全局推送群 + `onReceiveRESTCustomData` | `pushes` 流，不需要加群 |
| contentType 0 / 6～9 | `type`：`goal`、`liveStatus`（Schema 已有这两个示例名），`target=all`，`ttlSeconds=0` 语义（L-2、L-17） |
| contentType 1 | `muted` / `unmuted` 事件（F-ROOM-09） |
| contentType 3 | `msgDeleted` 事件（F-ROOM-10） |
| contentType 4 | `type=withdraw`，`target=user`，负载去掉账号列表（L-15） |
| 业务接口：`genUserSig`、`groupId`、`live/init`、礼物、时长、上报 | **继续由业务后端提供**，只把返回内容换成 token 和 `roomId`；`live/init` 的已删列表可去掉 |

**验收标准**
- AC-MIG-01-1：输入 第 2.4 节和 2.8 节的全部类型；操作 按对照表逐项核对；预期 每一项都有去向，或写明"删除"；错误码 —。
- AC-MIG-01-2：输入 对照表中的示例代码；操作 三套 SDK 的集成测试实际运行；预期 通过；错误码 —。

#### F-MIG-02 业务服务端切换需求（L4，P0）

**规则**
1. `genUserSig` 增加返回天听 token（或新接口），过渡期两者并存。
2. 赛事、主播开房时调 `POST /admin/rooms`，结束时关闭；`groupId` 接口返回 `roomId`。
3. 礼物、官方消息改调 `POST /admin/rooms/{id}/messages`；推送改调 `POST /admin/push`；禁言、删除分别调 `/admin/mute`、`/admin/rooms/{id}/messages/delete`。
4. 调用方配置 scope 和密钥（F-KEY-02），密钥不下发客户端。
5. 业务服务端的现状（N-1、N-2）要先确认，本条可能随之调整。

**验收标准**
- AC-MIG-02-1【后端】：输入 业务方测试环境；操作 走完开房、发言、送礼、删除、禁言、关房；预期 每一步在天听侧的 `admin_audit` 都有记录；错误码 —。
- AC-MIG-02-2【后端】：输入 业务方的调用方凭证；操作 调用未授权的 `push.all`（L-17 选默认时）；预期 403；错误码 4030。
- AC-MIG-02-3：输入 客户端安装包；操作 搜索管理密钥特征；预期 搜不到；错误码 —。

#### F-MIG-03 灰度与回退（L4，P0）

**规则**
1. 业务配置按 App 版本和用户比例决定走腾讯还是天听（L-11）。App 里两套实现并存一个过渡期，配置可远程切回。
2. 过渡期**推送双发**：同一业务事件同时发到腾讯和天听；单个客户端只启用一路，不会收到两次。
3. 聊天室过渡期的互通方式由 L-11 决定。
4. 下线腾讯的门槛由 Frank 确认，建议：旧版本活跃占比低于约定值，并且连续 2 周没有回退。

**验收标准**
- AC-MIG-03-1：输入 灰度 10% 用户；操作 查看连接；预期 约 10% 用户连接天听；其余仍走腾讯；两路都能收到进球推送；错误码 —。
- AC-MIG-03-2：输入 已灰度用户；操作 配置切回腾讯，重启 App 后；预期 功能正常，不再连接天听；错误码 —。
- AC-MIG-03-3：输入 双发期间，已灰度用户；操作 触发一次进球；预期 用户只看到一次提示；错误码 —。

#### F-MIG-04 三端行为不回归（L4，P0）

**规则**：下面这些现有行为迁移后保持不变：赛事群 / 主播群切换规则（`chat_room_notifier.dart:183-192`）；发言门槛提示（2.7 节）；昵称打码；送礼按钮只在主播房显示；敏感词提示；进球按投注 / 收藏过滤；进下级页面退房、返回重连。`settlementStautus` 的拼写错误保留，修改需要业务方另行决定，因为新旧版本要互相解析。

**验收标准**
- AC-MIG-04-1：输入 三端新版本；操作 按上面的清单逐项走查；预期 与旧版本一致；错误码 —。
- AC-MIG-04-2：输入 新版本的晒单消息；操作 用字段 `settlementStautus` 解码；预期 三端显示一致；错误码 —。
- AC-MIG-04-3：输入 无网 → 恢复网络；操作 在聊天室内切换；预期 自动回房，不产生重复消息；错误码 —。
- AC-MIG-04-4：输入 发言门槛不满足的用户；操作 尝试发言；预期 提示与旧版本一致，消息不发出；错误码 —。

---

## 8. 非功能需求

| 类别 | 要求 |
|---|---|
| 性能（草案，L-14） | 单房 5000 在线；100 条/秒上行时未被限流的消息 P99 ≤ 1 秒；全员推送 1 万连接 P99 ≤ 5 秒；1000 人的按用户推送接口 ≤ 2 秒返回；`pushSync` 返回 100 条 ≤ 200ms |
| 不回归 | 现有性能基线（`docs/reports/perf-2026-11-03.md`）不下降；聊天室高峰不影响单聊、群聊（AC-ROOM-08-4） |
| 可靠性 | 按用户推送在 ack 前已提交事务。聊天室消息尽力而为，允许丢失，不提供补拉 |
| 集群 | 首版单节点。ARCH-v1 的 `delivery.Router` 目前只有按用户投递（ARCH 2.2 节），集群需要补"按房间广播"和"全员广播"，房间订阅表和在线人数也要跨节点；**这是 B6 的新增范围** |
| 安全 | 所有下发、推送接口走 F-KEY 签名和 scope；`ext`、`data` 按纯文本渲染；日志不写正文和 `data` |
| 隐私 | 账号类定向信息不进广播负载（L-15）；终端用户 IP 按现有脱敏规则 |
| 兼容 | 先发能容忍新字段的 SDK，再开服务端能力；服务端开关建议 `IM_PUSH_ENABLED`、`IM_ROOM_ENABLED`，默认关闭 |
| 测试支持 | 复用 [IM 钟]、双节点环境；增加"关闭定时清理""下行不读"两种故障注入，只在测试模式注册（PRD-core 6.1 节） |

---

## 9. 错误码（建议，定稿后写入 errors.json）

| 码 | 名称 | HTTP | 含义 | 用于 |
|---|---|---|---|---|
| 4041 | NOT_IN_ROOM | — | 已有；`action` 去掉"二期" | F-ROOM-06 |
| 4047 | ROOM_NOT_FOUND | 404 | 房间不存在或已关闭 | F-ROOM-01、02、07、10，F-PUSH-05 |
| 4094 | PUSH_ID_CONFLICT | 409 | 同一 `pushId` 内容不同 | F-PUSH-06 |
| 4138 | ROOM_FULL | —（ws） | 房间达到上限，只在 L-14 设上限时启用 | F-ROOM-02 |
| 4510 | CONTENT_BLOCKED | —（ws） | 内容被拦截；编号沿用 F-ADM-60 的建议 | F-ROOM-11 |

新 scope：`room.read`、`room.write`、`room.message`、`push.room`。4047、4094、4138 已与 `errors.json` 核对，当前未被占用。

---

## 10. 协议变更清单（定稿后落实到 protocol/）

| 变更 | 里程碑 |
|---|---|
| `PushFrame`：`pushId` 改必填；增加 `target`、`pushSeq`、`roomId`；`ts` 改必填；更新 `frames.json:28` | L1 |
| 新增命令 `pushSync` 及应答 | L1 |
| `POST /admin/push` 写入 `ADMIN.md`：增加 `ttlSeconds`、`roomId`、响应 `{pushId, created, accepted}` | L1 |
| 新增 scope `push.room`（L2）及 `room.*` | L1 / L2 |
| 打通 `joinRoom`、`leaveRoom` 和 room 类型的 `send`；更新 `e2e_test.go:226` 的断言 | L2 |
| `Message` 增加 `fromNick`、`ext`；`type` 增加 `custom`，`body` 增加 `{customType, data}`；room 消息无 `seq` | L2 |
| `recent` 中的消息与 `msg` 同结构 | L2 |
| 事件 `roomClosed {roomId, reason}`；`roomOnline` 不变 | L2 |
| `msgDeleted` 的 `seqs` 改为条件必填（`convId` 为 `room:` 时不出现） | L2 |
| 管理接口 `POST/GET/DELETE /admin/rooms`、`/admin/rooms/{id}/messages`、`/messages/delete` | L2 |
| `errors.json`：第 9 节 | L1 / L2 |
| `protocol/README.md:53` 的"二期"行改为正式定义 | L1 / L2 |

---

## 11. 对已冻结文档和任务的影响（只列出，本文不改）

| 文档 / 任务 | 问题 | 建议 |
|---|---|---|
| PRD-admin F-ADM-50 规则 1、8 | 没有房间目标，也没有 `ttlSeconds` | 定稿后走变更，增加；AC 对齐见 4.7 节 |
| PRD-admin F-KEY-02 规则 5 | 业务方默认没有 `push.all`，但现有进球、主播状态是广播 | 由 L-17 决定，可能要改这一句 |
| PRD-admin F-ADM-60 | 敏感词是 P2，错误码"届时分配" | 由 L-9 决定；选建议方案时提前并启用 4510 |
| PRD-admin 5.2 节 | `console.md:499` 已指出缺 `push.records.read`，文档还没补 | 随 F-ADM-50 变更一起补 |
| PRD-ui F-UI-44 规则 2、3 | 没有考虑补收：H5 每次登录都会回放 | 由 L-4 决定，补一条 AC |
| PRD-core 第 7、8 节 | 推送行需要指向本文；新增命令没有列入 | 定稿后走变更 |
| PLAN 第 1 节、开头 | 礼物、点赞列 P1；开头写"不使用 Flutter"（`PLAN.md:8`），同一段又把替换 fx-sport 的腾讯 IM 列为目标（`PLAN.md:4`），而 fx-sport 是 Flutter，接入方式没有方案 | 随 L-10、L-12 更新 |
| ARCH-v1 2.2 节 | 投递接口没有房间和全员广播 | B6 前补 |
| BE-228 | 增加 `ttlSeconds`、幂等表、4094 | 定稿后更新 |
| BE-229 | 表增加 `push_seq`；`pushSync`；读取时过滤；第 2 条"遍历 hub"保留 | 定稿后更新 |
| CS-044 | `pushId` 由 console 生成并存 `im_push_id`；5040 对账用同一号重放 | 定稿后更新 |
| CS-045、FE-424 | 不受协议影响 | 无 |
| FE-208、FE-353、FE-434 | `pushes` 流增加 `replayed`，SDK 自动 `pushSync`、保存游标 | 定稿后更新；FE-434 的估时可能增加 |
| FE-435、FE-436 | 原生 SDK 第二批内容没有列 `pushes` 流（`frontend-admin.md` FE-435） | 分发时补任务 |
| 测试用例 TC-ADM-50 | 随 AC 同步 | 定稿后更新 |

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

## 13. 待拍板清单

> 共 17 项。每项都写了建议、备选和影响，**均未决定**；文中的规则暂按建议起草。**优先级**：L-12、L-9、L-17、L-11 影响最大，建议先定。

| # | 问题 | 建议方案 | 备选 | 影响 |
|---|---|---|---|---|
| **L-1** | 全员推送要不要离线补收？ | **不补收**，只投在线连接，与 F-ADM-50 规则 3 的输入一致；现有 App 也没有补收（2.8 节） | ② 全员收件箱读扩散：一份数据，所有用户按全局序号补拉，保留 7 天 | 选 ① 最简单，公告类通知离线用户看不到，需要业务方确认够用（REVIEW-admin D-ADM-07 留的问题）。选 ② 多一条序号和一张表，AC-ADM-50-3 要改 |
| **L-2** | 按用户推送是否都补收？ | **调用方按条指定** `ttlSeconds`：默认 7 天，0 表示仅在线 | ② 一律 7 天 | ② 更简单，但有时效的通知（如提现待确认的浮窗）上线后才弹就是错的；① 需要业务方按类型决定 |
| **L-3** | 多端补收用谁的游标？ | **客户端游标**，每台设备各自补收（4.5 节） | ② 服务端按设备记录；③ 用户只收一次 | ① 与消息模型一致，服务端无状态；缺点是换设备或清数据会回放保留期内的内容。② 需要设备身份；③ 第二台设备会漏 |
| **L-4** | H5 没有持久化，每次新会话都会回放 7 天内的通知，怎么展示？ | 回放项 `replayed=true`：**进"我 → 系统通知"列表，不弹横幅，不计未查看** | ② 回放也弹横幅、计未查看；③ H5 把游标存 `localStorage` | ① 列表和 PRD-ui D4（内存）相容，需要给 F-UI-44 加 AC。② 每次登录一串横幅。③ 改动 D4，且清缓存后仍回放 |
| **L-5** | 聊天室消息要持久化吗？ | **不持久**，只保留内存缓冲（PLAN 2.1 节） | ② 按天保留到数据库，用于审核和取证 | ① 省资源，但删除无法取证，事后查不到；② 有合规价值，写入量大，还要决定是否走隔离原文，建议问法务后定 |
| **L-6** | 游客能进房只读吗？（N-5） | 二期**只允许登录用户**；业务方查明现状后再定 | ② 业务后端给游客签匿名 token，只读 | 如果现网游客能看聊天室，① 会造成回退；② 增加匿名用户、连接数和人数口径的问题 |
| **L-7** | 在线人数的口径？ | **不同 userId 数**，最多每 5 秒广播一次 | ② 连接数；③ 业务方可配置放大系数 | ① 跨节点去重有成本（B6）；② 简单但虚高。③ 是运营决定，不建议放进 IM |
| **L-8** | 房间怎么创建？ | **管理接口显式创建** | ② 首次 `joinRoom` 自动创建 | ① 防止任意 `roomId` 占内存，业务后端要在开赛、开播时调用；② 接入更省事，但要限流，还要有回收规则 |
| **L-9** | 敏感词什么时候做？现在靠腾讯 80001（2.7 节） | **把聊天室文本的拦截提到 L2、P0**（F-ROOM-11） | ② 业务后端做回调审核（ARCH S14）；③ 保持 P2，接受过渡期没有拦截 | 这是上线门槛。① 增加约 3～5 人日，还要准备词库；② 增加延迟和业务方工作量；③ 有内容合规风险，不建议 |
| **L-10** | 礼物等自定义消息是否提到 P0？（PLAN 列 P1） | **提到 P0**，只做通用自定义消息和服务端下发（F-ROOM-07） | ② 保持 P1，L2 只有文本 | 三端都依赖（2.4 节），不做就不能迁移；① 不含礼物业务逻辑，所以增量不大 |
| **L-11** | 新旧版本过渡期聊天室怎么办？ | **按版本灰度，过渡期新旧用户在两个通道、互相看不到发言；推送和礼物、官方消息由业务后端双发**；设门槛后下线腾讯 | ② 服务端双向桥接（需要腾讯回调，N-4 未查到）；③ 按房间整体切换，并强制升级 | ① 成本最低，短期内聊天室人气分流；② 体验最好，但工作量和消息循环风险都高；③ 切换干净，但强制升级对业务打扰大 |
| **L-12** | fx-sport 是 Flutter，PLAN 只有 TS、Kotlin、Swift 三套 SDK，怎么接？ | **新写纯 Dart 的 `sdk-dart`**，跑同一套 `vectors` | ② Flutter 插件包一层 sdk-kotlin、sdk-swift；③ fx-sport 不迁移 | ① 约为一套新 SDK 的工作量（参考 sdk-kotlin），但逻辑一致、易测试。② 少写协议代码，但要维护平台通道和两端事件对齐。③ 无法完全下线腾讯。**PLAN 没有这一项，当前工期未包含** |
| **L-13** | 迁移的责任边界？ | 天听交付服务端、SDK、指南，并**配合一端试点**（建议 android，它的封装最集中：`ChatRoomHelper.kt`）；三个 App 和业务服务端的改造由各自负责人排期 | ② 本 PRD 负责到三端全部上线 | ① 范围清楚，但 F-MIG-02～04 的验收依赖外部；② 工作量大幅增加，且需要业务仓库的改动权限 |
| **L-14** | 容量目标？（N-3 未查到现网数据） | 先按**单房 5000、单机 1 万连接**设计，**不设硬上限**；拿到业务方峰值后再校准 | ② 设上限，满员返回 4138；③ 按大房间设计（超大房采样下发） | 影响 B6 范围和压测环境。腾讯现网如果有大型赛事，5000 可能不够，必须先拿数据 |
| **L-15** | 提现推送把账号列表广播给所有客户端（2.8 节），迁移时改吗？ | **必须改成按用户推送**，负载不含其他账号，客户端不再比对账号 | ② 保持广播，协议不管 | ① 业务后端要改（账号映射 userId）；② 把账号发给无关用户，是隐私问题，不建议 |
| **L-16** | 协议和聊天室是否一起冻结？ | **分开**：第 4、6 节先评审冻结（L1），第 5、7 节晚一步 | ② 整份一起冻结 | ① 马上解除第二批系统通知的阻塞；缺点是有两轮评审，L2 可能回头改 `target=room`。② 一次评审，但第二批要继续等 |
| **L-17** | 业务调用方能否持有 `push.all`？现有进球、主播状态是广播，PRD-admin 规定默认不给 | **接入时授予**，im-server 限速 20 次/分钟（F-PUSH-09）；不受 console 的 5 分钟和双人复核约束 | ② 不给；业务方把这些事件发到一个所有人都进的"主题房间"；③ 单独做主题订阅（P1） | ① 与现有用法最接近，但绕开了人工复核，密钥风险更高；可以考虑按 `type` 白名单。② 不用新能力，但每个连接只能进一个房，和观看聊天室冲突，要放宽 F-ROOM-02 规则 2。③ 最干净，工作量最大 |

---

## 14. 工作量粗估（人日，评审后分发时重估）

| 里程碑 | 服务端 | SDK / 前端 | 测试 | 合计 |
|---|---|---|---|---|
| L1 push 协议（含原 BE-228、229、FE-434 的部分） | 6 | 3 | 3 | 12 |
| L2 聊天室 | 10 | 4 | 5 | 19 |
| L3 sdk-kotlin、sdk-swift 补充 | — | 6 | 2 | 8 |
| L3 sdk-dart（L-12 选 ① 时） | — | 待评估 | 待评估 | 待评估 |
| L4 迁移 | 业务方 | 业务方 | 业务方 | 待 L-13 |

估算依据是同类任务（BE-228、229 各 2.5 人日）的量级，不是详细设计结果。敏感词（L-9）、B6 集群部分不含在内。

---

## 15. 变更记录

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v0.1 | 2026-11-05 | 初稿：现状调研、push 协议建议方案、聊天室、推送、迁移需求和待拍板清单 | 产品（AI） |
