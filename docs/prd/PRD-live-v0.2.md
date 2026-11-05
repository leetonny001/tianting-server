# PRD：直播聊天室与系统推送（二期）

| 项 | 内容 |
|---|---|
| 版本 | **v0.2（草稿，2026-11-05）** |
| 作者 | 产品（AI） |
| 拍板人 | Frank |
| 日期 | 2026-11-05 |
| 状态 | 草稿，已按三视角评审（[REVIEW-live-v0.1](../review/REVIEW-live-v0.1.md)）修改，**未复审、未拍板**。第 13 节待拍板项确认后才能冻结。定稿前任务 BE-228、BE-229、CS-044、CS-045、FE-353、FE-424、FE-434 继续等待 |
| 对应队列 | [QUEUE](../process/QUEUE.md) Q19 |
| 依据 | [PLAN](../PLAN.md) 第 1 节、2.1～2.3 节；[PRD-core v1.3](PRD-core-v1.3.md) 第 7 节；[PRD-admin v1.0](PRD-admin-v1.0.md) 2.2 节规则 4、F-ADM-50；[PRD-ui v1.0](PRD-ui-v1.0.md) F-UI-44；[任务单](../tasks/backend-admin.md)；三个业务仓库的只读调研（第 2 节） |
| 关联 | 协议 [protocol/README.md](../../protocol/README.md)、Schema `protocol/schema/frame.schema.json`、用例 `protocol/vectors/frames.json`、错误码 `protocol/errors.json`、架构 [ARCH-v1](../arch/ARCH-v1.md) |
| 冻结文档 | 本文不修改任何已冻结 PRD。需要它们配合的地方列在第 11 节，定稿后走变更流程 |
| 阅读提示 | 第 2 节只写事实，推论单独标"推论"，没查到的写"未查到"。标"（L-n）"的规则按第 13 节的建议方案起草，拍板结果不同就随之修改。标"未校准"的数字没有现网依据，只是可配置的初始默认值，等 N-2、N-3、N-8 或压测结果校准 |

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
| R-LF-09 | 严重 | 转待拍板 | `replayed` 规则已定；展示转 L-4，H5 存储转 L-18。没采用报告的三态字段：`pushSeq` 加业务比较已能区分，不必多一个协议概念 |
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
| 离线补收（建议） | 按用户推送写入收件箱，按用户分配递增的 `pushSeq`；客户端每次连接成功后用 `pushSync {afterSeq}` 补拉；默认保留 7 天，到期不再补收。全员、按房间推送只投递在线连接（L-1）。游标规则见 4.3 节，独立于本地库存储，交付契约见 4.8 节 |
| `pushId` 去重（已定） | `pushId` **全局唯一**，调用方生成（建议 UUID）；服务端按 `pushId` 幂等，重复提交不重复投递，同号但调用方或内容不同返回 409 / 4094；SDK 再按 `pushId` 去重，消除实时投递和补拉的重叠 |
| 多端语义（建议） | 同一用户的每个在线连接各收一次；每台设备自己保存游标、自己补收；"收到"和"已读"不跨端同步（L-3） |
| 调研最重要的三点 | ① 三端都把"全局推送"做成加入一个共用的群，监听 `onReceiveRESTCustomData`，没有 `pushId` 和补收。② 客户端代码里没有发礼物、官方消息的调用，也没有调用在线人数接口。③ `AVChatRoom` 字样在三个仓库 0 命中，建群和 REST 下发都在业务服务端，**本次未调研** |
| 必须拍板 | 分两组（第 13 节）。**解除第二批阻塞**：L-1、L-2、L-3、L-4、L-16、L-17、L-18。**整体迁移**：影响最大的是 L-12（fx-sport 是 Flutter）、L-9（敏感词）、L-11（过渡期）、L-14（容量和压测环境） |
| 规模 | 30 个功能点（聊天室 15、推送 11、迁移 4），AC 共 211 条（聊天室 103、推送 85、迁移 23）；待拍板 21 项 |
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
| 推送不丢 | 保留期内的按用户推送：服务端可查询到；SDK **至少一次**交付给已订阅的业务代码（4.8 节）；同一 SDK 实例按 `pushId` 去重。业务代码自身崩溃或丢弃不在此指标内 | F-PUSH-03、F-PUSH-06、F-PUSH-08 的 AC |
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
| **L1 push 协议** | 第 4 节评审定稿并冻结（冻结即解除 BE-228、BE-229、CS-044、CS-045、FE-353、FE-424、FE-434 的阻塞）；im-server、sdk-ts、H5 通知 | 下表中归属 L1 的 AC 全部通过；行为用例 `vectors/push-sync.json` 在 sdk-ts 跑通。AC-ADM-50 系列随 CS-044、CS-045、FE-424 完成时验收，不作为协议冻结的前提 |
| **L2 聊天室** | im-server + sdk-ts；单节点；定义 `RoomRouter` 和房间状态接口 | 下表中归属 L2 的 P0 AC 通过；开发前先做扇出原型压测；性能 AC 按 8.1 节判定 |
| **L3 其他 SDK** | sdk-kotlin、sdk-swift，以及 Flutter 接入（L-12）；集群实现随 B6 | 各 SDK 跑通 `frames.json` 和行为用例 `push-sync.json`、`room-session.json`（`frames.json` 只验帧格式，验不到行为）；在业务工程的空分支上能引入并编译通过（L-13） |
| **L4 迁移** | 一端试点 → 三端灰度 → 下线腾讯（L-11、L-13） | F-MIG 的 AC 通过；执行方见 7 节；下线门槛由 Frank 确认 |

**AC 里程碑归属**（未列出的按所在功能点的里程碑）：

| AC | 归属 | 说明 |
|---|---|---|
| F-PUSH-01～04、06、07、09～11 的 AC | L1 | 帧中 `target=room` 的格式在 L1 冻结，接口要到 L2 才可用 |
| AC-PUSH-01-2 | L1 | `room` 帧只校验 Schema |
| AC-PUSH-02-8 | L1 | `target=room` 在 L1 返回错误 |
| AC-PUSH-02-7、F-PUSH-05 的 AC | L2 | 依赖房间 |
| AC-PUSH-07-2 | L1 | web 用 sdk-ts，"android"用 `platform=android` 的协议客户端模拟；真实 sdk-kotlin、sdk-swift 由 L3 的行为用例覆盖 |
| F-PUSH-08 的 AC | L1（sdk-ts） | 其他 SDK 在 L3 跑同一批行为用例 |
| AC-ROOM-12-3 | L3 | 原生 SDK 才有本地库；L2 用 AC-ROOM-12-4 断言 store 不写房间消息 |

依赖：L1 依赖第二批协议兼容规则（PRD-core 第 8 节"三套 SDK 必须忽略未知"）。L2 依赖 L1 的按房间目标定义，以及用户级禁言（F-ADM-40）。L1 与 L2 建议分开冻结（L-16）。

---

## 4. push 协议（v0.2，待拍板后冻结）

> 本节回答 PRD-admin 2.2 节规则 4 的三个问题。纯技术规则已按评审定下；涉及产品的部分标（L-n）。冻结后才生效。

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
    "ts": 1730000000000,       // 必填，服务端受理时间
    "target": "user",          // 必填：user | all | room。room 的帧格式在 L1 冻结，接口 L2 才开放
    "pushSeq": 42,             // 只有进收件箱的推送才有
    "roomId": "match-123"      // 只有 target=room 才有
} }
```

上行（新增命令）：

```jsonc
// 请求：二选一。fromLatest 只取最新序号建立游标，不回放
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
| 写入 | 收件箱 `push_inbox(user_id, push_seq, push_id, type, data, created_at, expires_at)`，`(user_id, push_seq)` 与 `(user_id, push_id)` 都唯一，`expires_at` 建索引。一次提交的所有用户在**同一事务**写入；Postgres 下按 userId 字节序加锁（F-PUSH-11） |
| 序号 | 计数器表 `push_seq(user_id, max_seq)`，每个用户独立递增，**不随清理删除**，序号不回退；`latestSeq` 从这张表读。不复用 `conv_seq`（它带会话语义）。分配时连续；但读取时因为条级 TTL 和收件箱上限，可能看到空洞。只有进收件箱的推送才占序号 |
| 投递 | 事务提交后向在线连接实时下发。提交后、下发前崩溃时，由下一次补拉兜底（4.6 节已接受风险） |
| 补拉时机 | SDK **每次连接成功**（`auth` 之后）调一次 `pushSync`，与 `joinRoom`、`convs` 并行互不阻塞（F-ROOM-03 规则 4）；`hasMore=true` 时继续；实时帧发现缺口时按下面的算法补拉 |
| 到期 | **读取时按 `expires_at` 过滤**（`now ≥ expires_at` 不返回），不依赖定时清理；BE-220 的定时任务只负责回收空间。`afterSeq` 早于保留期不报错，返回还在有效期内的部分 |
| 被封禁 | 封禁期间连不上，收不到；解封后在有效期内可补收 |

**游标规则**（三套 SDK 必须一致，由共享行为用例 `vectors/push-sync.json` 验证，输入是帧序列，期望是交付序列和游标）：

| # | 情况 | 处理 |
|---|---|---|
| C1 | 实时帧 `pushSeq = 游标 + 1` | 交付，交付后游标前进（4.8 节） |
| C2 | 实时帧 `pushSeq > 游标 + 1` | 暂存，等 500ms（与消息的缺口等待一致，协议 README 第 3 节规则 7）；缺口还在就 `pushSync {afterSeq: 游标}` |
| C3 | 实时帧 `pushSeq ≤ 游标` | **是否展示只看 `pushId`**：见过就丢弃，没见过就交付，不动游标 |
| C4 | 没有 `pushSeq` 的帧（`all`、`room`、`ttlSeconds=0`） | 按 `pushId` 去重后交付，**不动游标** |
| C5 | `pushSync` 单飞 | 同一时间只有一个请求；进行中发现新缺口，结束后再判断 |
| C6 | 应答 `hasMore=true` | 与暂存帧按 `pushSeq` 合并、按 `pushId` 去重后交付；游标 = 本页最大 `pushSeq`；继续拉 |
| C7 | 应答 `hasMore=false` | 交付后游标 = `max(游标, latestSeq)`。结果为空时也一样，尾部过期不会让游标停住 |
| C8 | `latestSeq < afterSeq`（服务端从备份恢复、切环境） | 游标重置为 `latestSeq`，靠 `pushId` 去重，否则之后的新推送都被游标挡住 |
| C9 | `pushSync` 失败 | 按 1、2、4…最长 30 秒退避重试；暂存帧等待超过 5 秒（未校准）先按 `pushSeq` 升序交付，**游标不前进**，之后的补拉靠 `pushId` 去重 |
| C10 | 没有游标（首次、无持久化） | 默认 `afterSeq: 0`；启动参数 `pushStart=latest` 时用 `fromLatest` 建立游标、不回放（灰度用，F-MIG-03 规则 3） |

**回放总量**：一次连接最多拉到收件箱全部内容，受单用户上限 1000 条（未校准，F-PUSH-09）约束，最多 10 页。

### 4.4 问题二：`pushId` 去重

两层，解决不同的问题：

| 层 | 解决 | 规则 |
|---|---|---|
| 服务端幂等 | 调用方重试（包括 console-server 的 5040 结果未知） | **`pushId` 全局唯一**。理由：收件箱唯一键和 SDK 去重都只看 `pushId`，帧里也没有调用方；全局唯一不需要改帧、不需要合成 ID，调用方用 UUID 即可避免撞号。同一调用方同内容：返回首次结果，`created=false`，按下面的状态决定是否再投递。调用方不同或内容不同：409 / 4094，不投递。v0.1 里"两个调用方同号各自投递"的 AC 已删除 |
| SDK 去重 | 实时帧和 `pushSync` 重叠、重连重发 | 同一 SDK 实例按 `pushId` 去重，近 2000 条 LRU（未校准）；原生端随游标持久化（4.9 节）。**不跨设备去重** |

**幂等表** `push_idempotency(push_id 主键, caller, content_hash, target, accepted, state, created_at)`：

| 环节 | 规则 |
|---|---|
| 校验失败 | 4000、4131、4042、4047、4030、4290 都**不占 `pushId`**，修正后可以用同号再提交 |
| 内容比对 | `content_hash` = SHA-256（`target`、去重排序后的 `userIds`、`roomId`、`type`、`data` 解析后按键排序重新序列化的字节、生效的 `ttlSeconds`） |
| `target=user` 且进收件箱 | 幂等记录、`push_seq`、`push_inbox` 在同一事务写入，`state=done`。投递靠实时帧 + 补拉，重试不再投递 |
| `all`、`room`、`ttlSeconds=0` | 先写记录 `state=pending`，再广播，广播发出后改 `done`。重试时：`done` 不投递；`pending` **重新广播**（至少一次，靠 SDK 去重），完成后改 `done`，都返回 `created=false`。调用方不重试时，崩溃会造成这条未投递（4.6 节已接受风险） |
| 重放 | 幂等检查先于限速，重放不受 F-PUSH-09 限速约束；console-server 的 5 分钟间隔也不约束同号重放（PRD-admin 变更，第 11 节） |
| 保留 | 幂等记录保留 8 天，**晚于收件箱到期**，否则同号重提交会撞 `(user_id, push_id)` 唯一键 |

### 4.5 问题三：多端语义

1. 同一用户的每个在线连接各收一次，包括同一台电脑的多个标签页。
2. 每台设备独立补收：A 设备在线收到，不影响离线的 B 设备上线后补收（L-3）。
3. 服务端不记录"谁收到了"，没有回执，也不跨端同步已读。天听"未查看条数"只统计当前会话（F-UI-44 规则 3 不变）。
4. `replayed` 是确定规则，只看时间，不看通道：`T0` 是**本 SDK 实例第一次 `auth` 成功时 ack 的 `serverTime`**，同一实例内重连不变；`replayed = (ts < T0)`。`ts` 和 `T0` 都是服务端时钟，没有客户端时钟偏差。所以网络闪断期间到达的推送、触发缺口补拉的实时帧、在线时错过的推送都是 `false`，App 重启或 H5 刷新后补到的旧推送是 `true`。是否提醒由业务决定（L-4）。

### 4.6 已考虑的备选与已接受的风险

| 方案 | 不选的原因 |
|---|---|
| 服务端按用户记"已投递"标记 | 第一台设备收到后，第二台设备就补不到，违反多端语义 |
| 服务端按设备记录游标 | 需要设备身份和回执，复杂度高；现有消息模型也是客户端游标（PLAN 2.1 节） |
| 把推送当系统账号的会话消息 | 复用 `seq` 和 `sync`，但会污染会话列表和未读，正是现有 App 要压制的问题（`push_notifier.dart:182-197`） |
| 全员推送写扩散到收件箱 | 单次写入量等于用户总数，代价高；需要时走 L-1 的读扩散备选 |

已接受的风险：① 全员和按房间推送在连接断开的短暂窗口内会丢（L-1）。② 全员和按房间推送是**至少一次语义下的尽力投递**：广播中途崩溃且调用方不重试时，部分用户收不到；调用方重试时重新广播，SDK 按 `pushId` 去重（4.4 节）。③ 单节点上，提交后、下发前进程崩溃时所有连接都会断开，重连即补拉，不存在长期漏收。**集群下这个风险才真实存在**（节点 A 崩溃或消息总线丢帧，节点 B 上一直在线的用户可能几天补不到），所以"低频定时 `pushSync`，或 pong 带 `latestSeq`"是 B6 的前提，届时补 AC。

### 4.7 F-ADM-50 草案 AC 对齐

AC 的预期大多不用改，但下面几处要走 PRD-admin 变更（清单在第 11 节，本文不改）：

| 草案 AC | 对齐结果 | 对应 |
|---|---|---|
| AC-ADM-50-1 在线 3 秒内收到 | 保留。帧按 4.2 节，补 `pushId`、`ts`、`target` | AC-PUSH-01-1 |
| AC-ADM-50-2 第 6 天补收、8 天后收不到 | 保留，前提是默认 `ttlSeconds` = 7 天（L-2）。"收到"指 `pushSync` 返回；到期按读取时过滤，不等定时清理 | AC-PUSH-03-1、03-2 |
| AC-ADM-50-3 全员：离线上线后收不到 | 保留，前提是 L-1 选建议方案；选备选时这条要改。全员不传 `ttlSeconds` 即可，默认 0，不会 400 | AC-PUSH-04-1、AC-PUSH-02-12 |
| AC-ADM-50-4 模板 Schema、4KB | 保留。Schema 校验在 console-server；im-server 另校验 4KB | AC-PUSH-02-2 |
| AC-ADM-50-5 全员间隔 5 分钟 | 保留，**只约束 console-server**；im-server 对业务调用方另有限速（F-PUSH-09、L-17） | AC-PUSH-09-2 |
| AC-ADM-50-6 ghost、1001 | 保留 | AC-PUSH-02-3、02-4 |
| AC-ADM-50-7 不能自批 | 不受影响 | — |
| AC-ADM-50-8 发送记录与审计 | 不受影响；`biz.push_records.im_push_id` 就是本协议的 `pushId` | AC-PUSH-09-1 |
| 规则 3"`pushId` 相同由 SDK 去重" | 明确为**同一 SDK 实例内**，不跨设备 | AC-PUSH-07-1 |
| 规则 8 | 增加 `target=room` + `roomId`、`ttlSeconds` | AC-PUSH-05-1 |
| 5040"不要重复提交" | **要改**：同一 `pushId` 的重放是安全的；只有审计对账结论为"未确认投递"时才重放，已确认成功的不重放；重放不受 5 分钟间隔约束 | AC-PUSH-06-6、06-10 |

### 4.8 SDK 交付契约（L1 冻结）

| 项 | 规则 |
|---|---|
| 语义 | **至少一次**。业务要按 `pushId` 幂等处理（进程在回调后、持久化前崩溃时会重复一次） |
| 接口形态 | TS：`on('push', (p: Push) => void)`；Kotlin：`pushes: Flow<Push>`；Swift：`on(.push)` 和 `events()`；Flutter 随 L-12。多订阅者每个都收到一次 |
| 没有订阅者 | SDK 暂存，上限 500 条（未校准），首个订阅者接上后按序交付。现有三端的默认行为都会丢（`emitter.ts:18-26`、`Client.swift:215-236`、`IMClient.kt:178` 的 DROP_OLDEST），要专门实现 |
| 暂存溢出 | 丢最旧的，**游标不前进**，下次连接由 `pushSync` 补回；没有 `pushSeq` 的帧丢了就丢了 |
| 游标前进时机 | **交付给订阅者之后**才前进并持久化。"交付"指回调已调用（TS、Swift）或 `emit` 已完成（Kotlin，挂起等到有订阅者，不用 DROP_OLDEST） |
| 业务崩溃或丢弃 | 不在 SDK 责任内，回调返回即算交付 |

### 4.9 游标存储

现有两套原生 SDK 的本地库是可丢的缓存：退出、换号、被踢、库损坏、Schema 升级都会清（`LocalDatabase.kt:69-75`、`:157-160`，`IMClient.kt:1175-1178`，`Client.swift:810-814`）。游标放进去就会回到 0，保留期内的推送全部回放，而 token 默认 7 天，这就是常态。所以：

| 项 | 规则 |
|---|---|
| 位置 | 独立的键值文件（Kotlin、Swift）或 `shared_preferences`（Flutter，L-12），不进 `TABLES`，不随库重建和 `wipeAll` 删除 |
| 内容 | 按 userId 分键，只存游标数字和最近 2000 个 `pushId`（未校准），不含通知内容，不受 FE-131 的清除要求约束 |
| 什么时候回放 | 只有卸载、清应用数据、换设备 |
| 没有持久化能力（H5、Kotlin `database` 为 null 的配置） | 游标只在内存，每个新实例从 `afterSeq: 0` 开始；H5 刷新就会回放，展示见 L-4、L-18 |
| 换号 | 各用各的键（AC-PUSH-07-3） |

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

**规则**：字段见 4.2 节。服务端不解释 `type` 和 `data`。三套 SDK 忽略不认识的字段；兼容规则补上**不认识的顶层 `cmd` 也忽略**（PRD-core 第 8 节只写了事件类型、字段、枚举值，变更见第 11 节）。`IM_PUSH_ENABLED` 关闭时服务端对 `pushSync` 返回 4001，SDK **静默忽略**这一次的 4001（不提示升级，下次连接再试）。

**验收标准**
- AC-PUSH-01-1：输入 u1 在线；操作 按用户推送 `type=notice`；预期 3 秒内收到，`pushId`、`type`、`data`、`ts`、`target=user`、`pushSeq` 都正确；错误码 —。
- AC-PUSH-01-2【后端】：输入 `frames.json` 新增的 user、all、room 三种帧；操作 用 Schema 校验；预期 全部通过；缺 `pushId` 的帧校验失败；错误码 —。
- AC-PUSH-01-3【SDK】：输入 帧中有未知字段 `foo`；操作 收到；预期 正常交给业务；错误码 —。
- AC-PUSH-01-4【SDK】：输入 服务端对 `pushSync` 返回 4001；操作 连接成功后；预期 不抛错、不提示升级，其他功能正常，下次连接再发一次；错误码 4001。
- AC-PUSH-01-5【SDK】：输入 顶层 `cmd=futureThing` 的帧；操作 收到；预期 忽略，连接保持；错误码 —。

#### F-PUSH-02 推送管理接口（L1，P0）

**规则**
1. 接口见 4.2 节，只在管理地址提供，走 F-KEY 的签名和 scope。
2. `data` 序列化后 ≤ 4KB，否则 400 / 4000。`type` 1～64 字符。`pushId` 格式不符 400 / 4000。
3. `userIds` 去重后 ≤ 1000，超出 400 / 4131；`target=user` 时不能为空；`accepted` 是去重后的人数。有不存在的用户：404 / 4042，`details.userIds` 列出，**整批不发**（业务调用方是否允许部分成功见 L-19）。
4. `ttlSeconds` 是可选字段，**不传和传 0 含义不同**：`target=user` 不传时为 604800，传 0 表示只投在线；其他 `target` 不传时为 0，显式传非 0 时 400 / 4000。取值 0～604800。
5. 模板 Schema 校验只在 console-server（F-ADM-50 规则 2）；业务方直接调用时，im-server 不校验模板。
6. `target=room` 必须带 `roomId`，其他 `target` 不能带；L1 未开启房间时（`IM_ROOM_ENABLED` 关闭）`target=room` 返回 400 / 4000。
7. 错误优先级：签名和 scope → 4000 参数 → 4094 / 幂等命中 → 4131 → 4042 / 4047 → 4290。**幂等检查先于限速**，所以重放不会被限速挡住。

**验收标准**
- AC-PUSH-02-1【后端】：输入 合法的 `target=user`、`target=all`；操作 分别调用；预期 200，`created=true`；`target=user` 时 `accepted` 等于目标用户数，`all` 没有该字段；错误码 —。
- AC-PUSH-02-2【后端】：输入 `data` 为 4097 字节；操作 调用；预期 400，不投递；错误码 4000。
- AC-PUSH-02-3【后端】：输入 `userIds=[u1, ghost1, ghost2]`；操作 调用；预期 404，`details.userIds=[ghost1, ghost2]`，u1 收不到；错误码 4042。
- AC-PUSH-02-4【后端】：输入 1001 个 userId；操作 调用；预期 400；错误码 4131。
- AC-PUSH-02-5【后端】：输入 `target=all, ttlSeconds=3600`；`ttlSeconds=604801`；操作 分别调用；预期 都是 400；错误码 4000。
- AC-PUSH-02-6【后端】：输入 只有 `push.user` 的调用方；操作 `target=all`；预期 403；错误码 4030。
- AC-PUSH-02-7【后端】（L2）：输入 合法的 `target=room`；操作 调用；预期 200，`created=true`，没有 `accepted`；错误码 —。
- AC-PUSH-02-8【后端】（L1）：输入 `IM_ROOM_ENABLED` 关闭；操作 `target=room`；预期 400；错误码 4000。
- AC-PUSH-02-9【后端】：输入 `type` 为空、65 字符；`pushId` 含空格、65 字符；操作 分别调用；预期 都是 400；错误码 4000。
- AC-PUSH-02-10【后端】：输入 `data` 恰好 4096 字节；恰好 1000 个 userId；操作 分别调用；预期 都是 200；错误码 —。
- AC-PUSH-02-11【后端】：输入 `target=user` 且 `userIds` 为空；`target=room` 缺 `roomId`；`target=all` 带 `roomId`；`userIds=[u1,u1]`；操作 分别调用；预期 前三个 400；最后一个 200、`accepted=1`，u1 只收到一条；错误码 4000。
- AC-PUSH-02-12【后端】：输入 `target=all` 不传 `ttlSeconds`；`target=user` 不传；`target=user, ttlSeconds=0`；操作 分别调用；预期 都是 200；第二个进收件箱，第三个不进；错误码 —。
- AC-PUSH-02-13【后端】：输入 不符合 console 模板的 `data`，业务调用方；操作 调用；预期 200，im-server 不校验模板；错误码 —。

#### F-PUSH-03 按用户推送与离线补收（L1，P0）

**规则**：见 4.3 节。`pushSync` 的 `limit` 1～100，默认 50，超范围 400 / 4000。只能拉自己的收件箱。

**验收标准**
- AC-PUSH-03-1【后端】[IM 钟]：输入 u3 离线；操作 推送后第 6 天上线并 `pushSync {afterSeq:0}`；预期 收到；错误码 —。
- AC-PUSH-03-2【后端】[IM 钟]：输入 u4 离线，定时清理被关闭；操作 推送后推进 8 天，`pushSync`；预期 收不到，证明靠读取时过滤；错误码 —。
- AC-PUSH-03-3【后端】：输入 u1 有 120 条未过期推送；操作 `limit=50` 连续翻页；预期 共 120 条，升序，无重复；`hasMore` 最后一页为 false；错误码 —。
- AC-PUSH-03-4【SDK】：输入 游标为 41，`FakeTransport`；操作 收到 `pushSeq=44` 的实时帧，等 500ms；预期 触发 `pushSync {afterSeq:41}`；业务按 42、43、44 收到，不重复；三条都是 `replayed=false`（`ts` 都不早于 `T0`）；错误码 —。
- AC-PUSH-03-5【后端】：输入 `ttlSeconds=0`；操作 向在线的 u1 推送；向离线的 u2 推送，u2 上线 `pushSync`；预期 u1 收到的帧没有 `pushSeq`；u2 结果为空；错误码 —。
- AC-PUSH-03-6【后端】：输入 `limit` 为 0、1、100、101；操作 分别发 `pushSync`；预期 1、100 成功；0、101 的 ack 错误；错误码 4000。
- AC-PUSH-03-7【SDK】：输入 连接建立；操作 `auth` 成功；预期 SDK 自动发一次 `pushSync`；错误码 —。
- AC-PUSH-03-8【后端】[IM 钟]：输入 u1 有 41（ttl 1 小时）、42（ttl 7 天）、43（ttl 1 小时）；操作 推进 2 小时后 `pushSync {afterSeq:40, limit:1}`；预期 只返回 42，`hasMore=false`，`latestSeq=43`；错误码 —。
- AC-PUSH-03-9【SDK】：输入 游标 41，`pushSync` 返回空、`hasMore=false`、`latestSeq=43`；操作 处理；预期 游标变为 43，不交付；错误码 —。
- AC-PUSH-03-10【SDK】：输入 游标 100，`latestSeq=5`（服务端数据恢复）；操作 处理；预期 游标重置为 5，之后 `pushSeq=6` 的实时帧按序交付；错误码 —。
- AC-PUSH-03-11【SDK】：输入 游标 41；操作 收到 `target=all` 和 `ttlSeconds=0` 的帧（无 `pushSeq`）；预期 交付给业务，游标仍是 41；错误码 —。
- AC-PUSH-03-12【后端】[IM 钟]：输入 `ttlSeconds=3600` 的推送；操作 时钟恰好等于 `expires_at`、早 1 毫秒，分别 `pushSync`；预期 等于时不返回，早 1 毫秒时返回；错误码 —。
- AC-PUSH-03-13【后端】[IM 钟]：输入 u1 的全部推送已过期并被清理任务删除；操作 再推一条；预期 `pushSeq` 继续递增，不回退，`latestSeq` 正确；错误码 —。
- AC-PUSH-03-14【后端】[IM 钟]：输入 u1 被封禁期间有推送；操作 解封后在有效期内连接并 `pushSync`；预期 补收到；错误码 —。
- AC-PUSH-03-15【SDK】：输入 `hasMore=true` 的第一页；操作 处理；预期 用本页最大 `pushSeq` 继续拉取，直到 `hasMore=false`；错误码 —。
- AC-PUSH-03-16【SDK】（原生）：输入 游标 41，分别走退出登录再登录、被踢后重登、库损坏重建、Schema 升级；操作 同一账号连接；预期 `pushSync {afterSeq:41}`，不回放；错误码 —。
- AC-PUSH-03-17【SDK】：输入 启动参数 `pushStart=latest`，本机没有游标；操作 连接；预期 发 `pushSync {fromLatest:true}`，不交付任何历史，游标设为 `latestSeq`；已有游标时不走这条；错误码 —。
- AC-PUSH-03-18【SDK】：输入 游标 41；操作 收到 43，400ms 内收到 42；预期 不发 `pushSync`，按 42、43 交付；错误码 —。
- AC-PUSH-03-19【SDK】：输入 一次 `pushSync` 进行中；操作 又发现缺口；预期 不并发第二个，当前结束后如仍有缺口再拉；错误码 —。
- AC-PUSH-03-20【SDK】：输入 `pushSync` 持续失败，暂存了 44；操作 用假时钟推进 5 秒；预期 44 交付给业务，游标不前进；后续按退避重试；错误码 —。
- AC-PUSH-03-21【后端】：输入 `pushSync` 带多余字段 `userId`；操作 发送；预期 ack 错误；错误码 4000。
- AC-PUSH-03-22【后端】：输入 `{fromLatest:true}` 且 u1 有推送；操作 `pushSync`；预期 `pushes` 为空，`latestSeq` 正确，`hasMore=false`；错误码 —。

#### F-PUSH-04 全员推送（L1，P0）

**规则**
1. 只投递当前在线连接（L-1）；不进收件箱。
2. 集群下经消息总线广播（B6）；单节点遍历连接异步下发，不占用发送路径，不另设分批参数，以 AC-PUSH-04-2 的结果为准。推送路径上**缺少广播能力时启动失败**，不能像现有 `Broadcaster` 类型断言那样静默跳过（`logic/social.go:801-807`）。
3. console-server 的双人复核和 5 分钟间隔不变（F-ADM-50 规则 5）。

**验收标准**
- AC-PUSH-04-1：输入 5 个用户在线、u9 离线；操作 全员推送；预期 5 人收到；u9 上线 `pushSync` 收不到；错误码 —。
- AC-PUSH-04-2【后端】[P-开发机 / P-目标机]（8.1 节）：输入 1 万连接在线，后台 600 条/秒单聊；操作 每 30 秒全员推送一条，共 20 次，统计所有连接的到达耗时；预期 合并后 P99 ≤ 5 秒全部送达；同窗口单聊 ack P99 ≤ 1 秒；错误码 —。
- AC-PUSH-04-3【后端】：输入 全员推送；操作 查收件箱（8.2 节测试接口）；预期 没有新增记录；错误码 —。
- AC-PUSH-04-4【后端】[P-开发机]：输入 后台 600 条/秒单聊；操作 按用户推送 1000 人 / 次，每秒 1 次，持续 5 分钟，与不推送的基线同一轮交替对照；预期 接口耗时 P99 ≤ 2 秒；单聊 ack P99 增幅 ≤ 20%（未校准）；错误码 —。
- AC-PUSH-04-5【后端】[P-开发机]：输入 u1 收件箱 1000 条；操作 `pushSync {limit:100}` 执行 1000 次；预期 P99 ≤ 200ms；错误码 —。
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
- AC-PUSH-06-6（跨服务，L1 随 CS-044 验收）：输入 注入 console-server 到 im-server 的延迟（`imLatencyMs`），调用超时 5040，实际已投递；操作 审计对账结论为"未确认投递"，用同一 `im_push_id` 重放；预期 重放 `created=false`，用户只收到一次；对账确认成功时不重放；错误码 —。
- AC-PUSH-06-7【后端】：输入 全员推送，注入"幂等记录写入后、广播前进程退出"；操作 重启后用同一 `pushId` 重试；预期 `created=false`，**重新广播**，在线用户收到，`state` 变为 `done`；错误码 —。
- AC-PUSH-06-8【后端】：输入 `userIds` 含不存在用户（4042）；`target=room` 房间不存在（4047）；操作 失败后修正内容，用同一 `pushId` 重新提交；预期 第一次不留记录，第二次 `created=true`；错误码 4042、4047。
- AC-PUSH-06-9【后端】：输入 `userIds=[u1,u2]` 和 `[u2,u1,u1]`，`data` 键顺序不同；操作 同一 `pushId` 提交两次；预期 判定为同内容，`created=false`；`ttlSeconds` 不同时判定为不同内容，4094；错误码 4094。
- AC-PUSH-06-10【后端】：输入 调用方已触发 4290；操作 用已成功的 `pushId` 重放；预期 200，`created=false`，不扣限速；错误码 —。
- AC-PUSH-06-11【后端】：输入 调用方 A 用 `p1` 提交成功；操作 调用方 B 用 `p1` 提交，内容相同、内容不同各一次；预期 都是 409，不投递；错误码 4094。
- AC-PUSH-06-12【后端】[IM 钟]：输入 `p1` 已提交；操作 推进 6 天重试；推进 9 天并跑清理后重试；预期 前者 `created=false`；后者 `created=true`，不与收件箱唯一键冲突；错误码 —。

#### F-PUSH-07 多端语义（L1，P0）

**规则**：见 4.5 节。

**验收标准**
- AC-PUSH-07-1：输入 u1 的 web、pc 都在线；操作 推送；预期 两个连接都收到，`pushId` 相同，两端的业务回调各触发一次；错误码 —。
- AC-PUSH-07-2（L1 用 `platform=android` 的协议客户端）：输入 u1 的 web 在线、android 离线；操作 推送，web 收到后 android 上线并 `pushSync`；预期 android 补收到；错误码 —。
- AC-PUSH-07-3【SDK】：输入 u1 退出登录，u2 在同一设备登录；操作 `pushSync`；预期 从 u2 自己的游标开始，不沿用 u1 的；错误码 —。
- AC-PUSH-07-4【后端】：输入 u1、u2 都有推送；操作 u1 发 `pushSync`；预期 结果不含 u2 的任何一条；错误码 —。

#### F-PUSH-08 SDK `pushes` 流（L1 sdk-ts，L3 其他，P0）

**规则**
1. 流中每项为 `{pushId, type, data, ts, target, roomId?, pushSeq?, replayed}`。按 `type` 分发由业务代码负责。接口形态和交付契约见 4.8 节。
2. 业务代码抛异常不影响 SDK 状态。各端表现不同：TS 捕获后继续；Kotlin 的异常在业务自己的收集协程里，SDK 接不到，重新 `collect` 可以继续收；Swift 回调不抛错，不适用。现有 App 的解析失败处理各自实现（2.8 节）。
3. **SDK 不把 `data` 写入日志**。现有 App 把全文写进日志（2.8 节），其中可能含账号。

**验收标准**
- AC-PUSH-08-1【SDK】：输入 `T0` 之后的实时帧；`T0` 之前的 `pushSync` 结果；闪断重连后 `ts ≥ T0` 的 `pushSync` 结果；操作 处理；预期 依次 `replayed=false`、`true`、`false`；错误码 —。
- AC-PUSH-08-2【SDK】（TS）：输入 业务回调抛异常；操作 再收一条；预期 第二条正常交付；错误码 —。Kotlin 改为：收集器抛错后 SDK 状态不受影响，其他订阅者继续，重新 `collect` 可收到后续推送。
- AC-PUSH-08-3【SDK】：输入 `data` 含特征串；操作 开启调试日志，处理推送；预期 日志里搜不到特征串；错误码 —。
- AC-PUSH-08-4【SDK】：输入 连接后才注册订阅者，期间到达 3 条；操作 注册；预期 按 `pushSeq` 顺序交付 3 条；错误码 —。
- AC-PUSH-08-5【SDK】：输入 没有订阅者，暂存已满（上限 500，未校准）；操作 再到达 1 条；预期 丢最旧的一条且游标不前进；之后注册订阅者并重连，丢掉的那条经 `pushSync` 补回；错误码 —。
- AC-PUSH-08-6【SDK】：输入 两个订阅者；操作 收到一条；预期 两个都收到一次；错误码 —。
- AC-PUSH-08-7【SDK】（原生）：输入 回调已执行、游标持久化之前进程退出；操作 重启并 `pushSync`；预期 这条再交付一次（至少一次语义），`replayed` 按规则计算；错误码 —。
- （引用）PRD-ui AC-UI-44-1～3 继续有效；回放展示见 L-4、L-18，确定后给 F-UI-44 补 AC。

#### F-PUSH-09 推送审计与限速（L1，P0）

**规则**
1. im-server `admin_audit`：只记目标数量、`target`、`type`、`pushId`、`data` 长度和 SHA-256（BE-228 第 3 条）。现有摘要白名单不含 `userIds`、`roomId`、`pushId`（`admin/audit.go:86-130`），需要用 `auditNote` 写入；房间路径参数统一叫 `{roomId}`。
2. 限速（都可配置，都未校准）：`target=all` 默认 20 次/分钟（按调用方）；`target=user` 默认 600 次/分钟（按调用方）；`pushSync` 默认每用户 30 次/分钟。超出 429 / 4290（`pushSync` 为 ack 错误 4290，SDK 按退避重试）。幂等命中的重放不计入。console-server 的 5 分钟间隔是另外一层（L-17）。
3. 业务调用方默认没有 `push.all`（PRD-admin F-KEY-02 规则 5）；授予由 L-17 决定。
4. 单用户收件箱上限 1000 条（未校准，`IM_PUSH_INBOX_MAX`），超出时淘汰最旧的，序号不变。清理任务按 `expires_at` 索引分批删除，每批 1000 行（未校准），不一次占住数据库连接。

**验收标准**
- AC-PUSH-09-1【后端】：输入 一次推送；操作 查 `admin_audit`；预期 有 `pushId`、目标数量、长度和 SHA-256，没有 `data` 明文；错误码 —。
- AC-PUSH-09-2【后端】：输入 持有 `push.all` 的调用方，冻结时钟；操作 1 分钟内第 21 次全员推送；预期 429；时钟推进 1 分钟后恢复；错误码 4290。
- AC-PUSH-09-3【后端】：输入 `target=user` 限速配置为 2，冻结时钟；操作 第 3 次；预期 429；错误码 4290。
- AC-PUSH-09-4【后端】：输入 `pushSync` 限速配置为 2，冻结时钟；操作 u1 第 3 次；预期 ack 错误；u2 不受影响；错误码 4290。
- AC-PUSH-09-5【后端】：输入 `IM_PUSH_INBOX_MAX=3`；操作 给 u1 推 5 条；预期 `pushSync {afterSeq:0}` 只返回最新 3 条，`pushSeq` 是 3、4、5；错误码 —。

#### F-PUSH-10 推送可观测（L1，P1）

**规则**：指标：推送数（按 `target`）、投递数、`pushSync` 次数和耗时、收件箱行数。`type` 可能由调用方任意填写，不作为指标标签。

**验收标准**
- AC-PUSH-10-1【后端】：输入 各 `target` 推送一次；操作 读 `/metrics`；预期 计数正确，标签没有 `type`、`userId`；错误码 —。

#### F-PUSH-11 推送并发与崩溃恢复（L1，P0）

**规则**
1. 一次提交的收件箱写入在同一事务内；Postgres 下按 userId 字节序加锁，避免交叠批次死锁（沿用 PRD-social 8.3 节的做法）。将来按用户分片后无法同事务写入，改为逐用户幂等写入（靠 `(user_id, push_id)` 唯一键），到时补 AC。
2. 这组用例 SQLite、Postgres 都跑。

**验收标准**
- AC-PUSH-11-1【后端】：输入 持续按用户推送并记录已 ack 的 `pushId`；操作 kill -9 后重启；预期 所有已 ack 的 `pushId` 都能被 `pushSync` 取到，`pushSeq` 不重复；错误码 —。
- AC-PUSH-11-2【后端】：输入 注入"提交后跳过下发"（8.2 节），u1 在线；操作 推送 p1，再推送 p2；预期 u1 没收到 p1 的实时帧，收到 p2 的帧（`pushSeq` 跳号）；`pushSync` 能取到 p1；sdk-ts 按 C2 补到 p1 后按序交付；错误码 —。
- AC-PUSH-11-3【后端】：输入 50 个 goroutine 并发给同一用户推送；操作 结束后 `pushSync`；预期 `pushSeq` 连续无重复，共 50 条；错误码 —。
- AC-PUSH-11-4【后端】：输入 同一 `pushId` 同内容并发提交 20 次；操作 结束后查询；预期 恰好一次 `created=true`，收件箱一条；错误码 —。
- AC-PUSH-11-5【后端】（Postgres）：输入 两组交叠的 `userIds`，顺序相反；操作 并发提交 100 轮；预期 全部成功，没有死锁错误；错误码 —。
- AC-PUSH-11-6【后端】：输入 1000 人的提交，注入第 500 人写入失败；操作 查询；预期 整批回滚，所有人收件箱无这条，没有下发，也没有幂等记录；错误码 —。

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
- AC-MIG-02-2【业务方】：输入 业务方调用方凭证；操作 按 L-17 的结果二选一写死：选 ① 时，用白名单外的 `type` 全员推送预期 403；选 ② 时，任何 `target=all` 预期 403；错误码 4030。
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
| 集群 | 首版单节点。L2 就定义 `RoomRouter`（按房投递、订阅、退订）和房间状态接口，单节点用内存实现，B6 只换实现（ARCH 第 7 行的原则）。`logic` 不能依赖 `hub`，现有 `Broadcaster` 类型断言失败时静默不发，不能沿用。要跨节点的有：广播、订阅表、在线人数、最近 50 条缓冲、`(from, clientMsgId)` 去重、房间关闭状态、已删 ID、单房限流、无 `seq` 时的顺序。**这是 B6 的新增范围，v0.2 没有工作量数字**。B6 前提：低频定时 `pushSync` 或 pong 带 `latestSeq`（4.6 节） |
| 安全 | 所有下发、推送接口走 F-KEY 签名和 scope；`ext`、`data` 按纯文本渲染；日志不写正文和 `data` |
| 隐私 | 账号类定向信息不进广播负载（L-15）；收件箱明文保存，负载禁止放账号明文；用户注销时清除收件箱，列入将来的注销范围（目前没有注销路径）；终端用户 IP 按现有脱敏规则 |
| 兼容 | 先发能容忍新字段和未知 `cmd` 的 SDK，再开服务端能力；服务端开关 `IM_PUSH_ENABLED`、`IM_ROOM_ENABLED`，默认关闭。开关关闭时 `pushSync` 返回 4001，SDK 静默忽略；`IM_ROOM_ENABLED=1` 要求 `IM_V13_ADMIN_FEATURES=1`，否则启动失败 |

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

| 设施 | 接口 / 手段 | 用到的 AC |
|---|---|---|
| 关闭定时清理 | 配置开关 `IM_TEST_DISABLE_JOBS=push_purge`（现有配置未查到同类开关，新增） | AC-PUSH-03-2、03-8 |
| 手动跑清理和回收 | 现有 `POST /admin/test/jobs/{name}/run`，新任务 `push_purge`、`room_idle_close` | AC-PUSH-03-13、06-12、AC-ROOM-01-8 |
| 查收件箱 | `GET /admin/test/push-inbox/{userId}`（没有正式管理接口） | AC-PUSH-04-3、06-1、11-6 |
| 提交后跳过下发 | `POST /admin/test/faults/push-skip-delivery {pushId}`（现有 skip-delivery 只按 `clientMsgId`，`logic/faults.go:22`） | AC-PUSH-11-2 |
| 幂等记录后退出 | `POST /admin/test/faults/push-exit-before-broadcast {pushId}` | AC-PUSH-06-7 |
| 写入中途失败 | `POST /admin/test/faults/push-fail-at {index}` | AC-PUSH-11-6 |
| 房间窗口推进 | `POST /admin/test/rooms/{roomId}/flush {what: online\|messages}`：立即结束当前窗口 | AC-ROOM-05-2、05-3 |
| 敏感词 | 测试启动时给 `IM_ROOM_BLOCKED_WORDS_FILE` 指向夹具文件 | AC-ROOM-11-1、11-2、11-6 |
| 限流清零 | 现有 `limits/reset` 要同时清房间桶和推送限速 | AC-ROOM-08-1、AC-PUSH-09-2～4 |
| console 到 im-server 的延迟 | 现有 `imLatencyMs`（`tianting-console-server/internal/imclient/client.go`，AC-CON-08-10） | AC-PUSH-06-6 |
| 下行不读 | 不需要服务端注入，用不读取的测试客户端 | AC-ROOM-08-4、08-8 |
| 进程崩溃 | 现有 `im-loadtest` kill -9 场景、`durability_test.go` | AC-PUSH-11-1 |
| [IM 钟] | 现有 `clock.Settable` | 标 [IM 钟] 的 AC |

### 8.3 共享行为用例

`frames.json` 只验证解码不崩溃（`VectorsTest.kt:141-152`、`VectorTests.swift:130`）。新增两份，四套 SDK 都跑：

| 文件 | 内容 | 落地时间 |
|---|---|---|
| `protocol/vectors/push-sync.json` | 游标规则 C1～C10、`pushId` 去重、`replayed`、交付顺序：输入帧序列和时钟，期望交付序列和游标 | L1，sdk-ts 先跑 |
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

新 scope：`room.read`、`room.write`、`room.message`、`push.room`。4047、4094、4138、4510 已与 `errors.json` 核对，当前未被占用。现有码的 `action` 要改：4001 增加"`pushSync` 由 SDK 静默忽略"；5040 增加"`pushId` 幂等的接口可以在对账后重放"（PRD-admin 变更，第 11 节）。

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
| 新增 scope `push.room`（L2）及 `room.*` | L1 / L2 |
| 打通 `joinRoom`、`leaveRoom` 和 room 类型的 `send`；更新 `e2e_test.go:226` 的断言 | L2 |
| `joinRoom` ack 增加 `deletedMsgIds` | L2 |
| `Message` 增加 `fromNick`、`ext`；`body` 增加 `{customType, data}`（`MsgType` 已有 `custom`）；room 消息无 `seq`；**room 消息放开 `from`、`fromNick` 可为空字符串** | L2 |
| `send` 请求增加 `ext`（`frame.schema.json:255-270` 现在 `additionalProperties: false`）；`SendAck.seq` 改条件必填；`customType`、`ext` 只对 `room` 开放；更新 `frames.json:7` 的房间 custom 用例 | L2 |
| `recent` 中的消息与 `msg` 同结构 | L2 |
| 事件 `roomClosed {roomId, reason}`，`reason` 枚举 `closed`、`idle`；`roomOnline` 不变 | L2 |
| `msgDeleted` 的 `seqs` 改为条件必填（`convId` 为 `room:` 时不出现） | L2 |
| 管理接口 `POST /admin/rooms`、`GET` / `DELETE /admin/rooms/{roomId}`、`/admin/rooms/{roomId}/messages`（响应 `{msgId, ts, created}`）、`/messages/delete`（单次 50）、`/admin/rooms/blocked-words/reload`。不提供 `GET /admin/rooms` 列表 | L2 |
| 配置：`IM_ROOM_USER_RATE`、`IM_ROOM_PUSH_RATE`、`IM_ROOM_CLIENT_CUSTOM_TYPES`、`IM_ROOM_IDLE_TTL`、`IM_ROOM_MAX_ONLINE`、`IM_NODE_MAX_CONNS`、`IM_ROOM_BLOCKED_WORDS_FILE`、`IM_PUSH_INBOX_MAX` | L1 / L2 |
| 新增行为用例 `vectors/room-session.json` | L2 |
| `errors.json`：第 9 节 | L1 / L2 |
| `protocol/README.md:53` 的"二期"行改为正式定义 | L1 / L2 |

---

## 11. 对已冻结文档和任务的影响（只列出，本文不改）

| 文档 / 任务 | 问题 | 建议 |
|---|---|---|
| PRD-admin F-ADM-50 规则 1、8 | 没有房间目标，也没有 `ttlSeconds` | 定稿后走变更，增加；AC 对齐见 4.7 节 |
| PRD-admin F-ADM-50 规则 3 | "`pushId` 相同由 SDK 去重"要明确范围；`pushId` 全局唯一 | 定稿后走变更 |
| PRD-admin 5040 规定（第 471、1143 行）、`errors.json:58` | 规定"不重试写操作""不要重复提交"，本文要求对账未确认时用同一 `pushId` 重放 | 定稿后走变更，限定为有 `pushId` 幂等的接口 |
| PRD-admin F-ADM-50 规则 5、CS-045 | 5 分钟间隔如果按最近一次发送判断，可能挡住重放（console 实现未核实） | 明确同号重放不受约束 |
| PRD-admin F-KEY-02 规则 5 | 业务方默认没有 `push.all`，但现有进球、主播状态是广播 | 由 L-17 决定，可能要改这一句 |
| PRD-admin F-ADM-60 | 敏感词是 P2，错误码"届时分配" | 由 L-9 决定；选建议方案时提前并启用 4510，只做拦截 |
| PRD-admin 5.2 节 | `console.md:499` 已指出缺 `push.records.read`，文档还没补 | 随 F-ADM-50 变更一起补 |
| PRD-ui F-UI-44 规则 2、3 | 没有考虑补收：H5 每次刷新都会回放；规则 2 按到达时间排序，回放项会排在更新的实时项前面；"最多 100 条"和回放的关系未定义 | 由 L-4、L-18 决定，排序改按 `ts`，补 AC |
| PRD-ui D4 | H5 只用内存 | 由 L-18 决定是否开例外 |
| PRD-ui 各端 F-UI-44 | 原生也强制，但第 11 节原来没有两端 App 的页面任务 | 分发时给 tianting-android、tianting-ios 补任务 |
| PRD-core 第 7、8 节 | 推送行需要指向本文；新增命令没有列入；兼容规则没覆盖未知顶层 `cmd`（`PRD-core-v1.3.md:118`） | 定稿后走变更 |
| PLAN 第 1 节、开头 | 礼物、点赞列 P1；开头写"不使用 Flutter"（`PLAN.md:8`），同一段又把替换 fx-sport 的腾讯 IM 列为目标（`PLAN.md:4`），而 fx-sport 是 Flutter，接入方式没有方案 | 随 L-10、L-12 更新 |
| ARCH-v1 2.2 节 | 投递接口没有房间和全员广播 | L2 定义 `RoomRouter` 时同步 |
| BE-228 | 增加 `ttlSeconds`、幂等表、4094 | 定稿后更新 |
| BE-229 | 表增加 `push_seq` 计数表；`pushSync`；读取时过滤；第 2 条"遍历 hub"保留 | 定稿后更新 |
| CS-044 | `pushId` 由 console 生成（UUID）并存 `im_push_id`；5040 对账，未确认时用同一号重放 | 定稿后更新 |
| CS-045、FE-424 | 协议本身不影响；CS-045 见上面的 5 分钟间隔 | 定稿后核对 |
| FE-208、FE-353、FE-434 | `pushes` 流增加 `replayed`、`pushSeq`，SDK 自动 `pushSync`、游标规则、交付契约 | 定稿后更新；FE-434 的估时会增加 |
| FE-435、FE-436 | 原生 SDK 第二批内容没有列 `pushes` 流（`frontend-admin.md` FE-435）；两套 SDK 已发首版，要补游标独立存储、房间模型 | 分发时补任务 |
| 测试用例 TC-ADM-50 | 随 AC 同步 | 定稿后更新 |
| P-12 | AC-ROOM-08-5、AC-PUSH-04-2 的发布结论需要目标机 | 在 P-12 中加入 |

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

> 共 21 项，**均未决定**；文中的规则暂按建议起草。纯技术选择（`pushId` 全局唯一、幂等表、游标规则等）已在第 4 节定下，不在这里。"优先级"分两组：只想解除第二批阻塞，定第一组即可；L-9、L-11、L-12 不挡 L1。

### 13.1 解除第二批阻塞需要的（L1）

| # | 问题 | 建议方案 | 备选 | 影响 |
|---|---|---|---|---|
| **L-1** | 全员推送要不要离线补收？ | **不补收**，只投在线连接，与 F-ADM-50 规则 3 的输入一致；现有 App 也没有补收（2.8 节） | ② 全员收件箱读扩散：一份数据，所有用户按全局序号补拉，保留 7 天 | 选 ① 最简单，公告类通知离线用户看不到，需要业务方确认够用（REVIEW-admin D-ADM-07 留的问题）；广播中途崩溃且不重试时部分用户收不到（4.6 节）。选 ② 多一条序号和一张表，AC-ADM-50-3 要改 |
| **L-2** | 按用户推送是否都补收？ | **调用方按条指定** `ttlSeconds`：`target=user` 不传为 7 天，0 表示仅在线；对照表逐类写明（提现为 0） | ② 一律 7 天 | ② 更简单，但有时效的通知（如提现待确认）上线后才弹就是错的；① 需要业务方按类型决定，多了混合 TTL 造成的序号空洞，已由游标规则 C7 处理 |
| **L-3** | 多端补收用谁的游标？ | **客户端游标**，每台设备各自补收（4.5 节）；游标独立于本地库存储（4.9 节），退出、被踢、Schema 升级都不回放；只有卸载、清数据、H5 才回放 | ② 服务端按设备记录；③ 用户只收一次 | ① 与消息模型一致，服务端无状态，但换设备或清数据会回放保留期内的内容。② 需要设备身份；③ 第二台设备会漏 |
| **L-4** | 旧推送（`replayed=true`）在"我 → 系统通知"怎么展示？ | **都进列表，按 `ts` 排序，不弹横幅**；`pushSeq` 高于本机已看过的序号时计入未查看（需要 L-18 的存储），其余不计。`replayed=false` 的正常弹横幅、计未查看 | ② `replayed=true` 一律不计未查看（v0.1 的方案）；③ 回放也弹横幅、计未查看 | ① 离线 6 天的用户能看到有未读，刷新页面不会重复提醒。② 最简单，但离线用户没有任何提醒，AC-ADM-50-2 的"收到"对用户几乎没有意义。③ 每次刷新一串横幅。还要决定：原生端列表是否持久化，不持久化时重启后列表为空，H5 靠回放能看到 7 天，两端不一致；建议先接受不一致并在页面注明。选定后给 F-UI-44 补 AC |
| **L-16** | 协议和聊天室是否一起冻结？ | **分开**：第 4、6 节先冻结（L1），第 5、7 节晚一步；`target=room` 帧格式在 L1 冻结但接口返回错误；L1 冻结范围包括三端 `pushes` 签名、交付契约和 `push-sync.json` | ② 整份一起冻结 | ① 冻结后解除第二批系统通知的阻塞，要先有这份 v0.2 的 AC 归属；缺点是两轮评审。② 一次评审，但第二批要继续等 |
| **L-17** | 业务调用方能否持有 `push.all`？现有进球、主播状态是广播，PRD-admin 规定默认不给（只影响业务方接入，不影响 console 的系统通知） | **接入时授予，三项必选**：独立调用方（只有 `push.all`，配 `AllowedCIDRs`）、`type` 白名单（调用方配置新增字段）、按 `type` 审计。im-server 限速 20 次/分钟是**未校准**的占位，等 N-3 再定；业务方自行处理 4290 | ② 不给；业务方把事件发到一个所有人都进的"主题房间"；③ 单独做主题订阅（P1） | ① 与现有用法最接近，但绕开人工复核，密钥能向所有人发白名单内的 `type`；多场比赛同时进球可能超限，`ttlSeconds=0` 的推送遇到 4290 就没了。② 每个连接只能进一个房，和观看聊天室冲突，要放宽 F-ROOM-02 规则 2。③ 最干净，工作量最大。结果决定 AC-MIG-02-2 取哪一支 |
| **L-18** | H5 能否用 `localStorage` 存一个数字？（新增）PRD-ui D4 规定 H5 只用内存 | **开一个小例外**：只存 `lastSeenSeq`，按 userId 分键，不存内容，退出登录时删除 | ② 不存，L-4 的未查看在 H5 上失效，刷新后不计；③ 存游标，H5 也不回放 | ① 改冻结的 D4，但没有内容和凭证落盘；清缓存后退回 ②。③ H5 刷新后列表就空了，和 H5 靠回放看历史矛盾 |

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
| L1 push 协议（含原 BE-228、229、FE-434 的部分；幂等表、游标规则、行为用例） | 8 | 5 | 4 | 17 |
| L2 聊天室（含双队列改造、扇出原型压测、`RoomRouter`、下发接口） | 14 | 6 | 6 | 26 |
| L3 sdk-kotlin、sdk-swift 补充（含行为用例、仅直播模式、分发配置） | — | 9 | 3 | 12 |
| L3 Flutter（L-12） | — | 前端评审粗估 8～12（含测试，只做仅直播子集，低置信） | 含左 | 8～12 |
| L4 迁移 | 业务方 | 业务方 | 业务方 | 待 L-13 |
| L-9 敏感词最小实现 | 3 | 0.5 | 1 | 4.5（BE-242 原估 3 人日加词库入口，词库内容不含） |
| B6 集群的房间和全员部分 | 待估 | — | 待估 | 待 B6 设计，v0.2 没有数字 |

估算依据是同类任务（BE-228、229 各 2.5 人日）的量级和评审意见，不是详细设计结果。

---

## 15. 变更记录

| 版本 | 日期 | 变更 | 作者 |
|---|---|---|---|
| v0.1 | 2026-11-05 | 初稿：现状调研、push 协议建议方案、聊天室、推送、迁移需求和待拍板清单 | 产品（AI） |
| v0.2 | 2026-11-05 | 按 REVIEW-live-v0.1 三视角 89 条问题修改，处理结果见文首修订记录 | 产品（AI） |
