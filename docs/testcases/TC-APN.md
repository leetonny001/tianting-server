# 测试用例：离线推送（APN，C3）

> 依据：[PRD-chat v1.7](../prd/PRD-chat-v1.7.md)（冻结）第 4.4、4.5、7 节，第 13、15 节的 C3 部分，第 20.3 节；官方文档核对见 [backend-chat](../tasks/backend-chat.md) 文末第 34～46 条（PENDING P-40）。任务单：[backend-chat](../tasks/backend-chat.md) BE-406、BE-450～BE-460，[console-chat](../tasks/console-chat.md) CS-203，[frontend-chat](../tasks/frontend-chat.md) FE-650～FE-656。
> 范围：标了【C3】的 AC-APN（CH-27，Frank 2026-11-10 决定继续 C3）。AC-APN-03-3（【C4】撤回替换通知）是 C4 分发时写的 TC-APN-03-03，**依赖 C3 的推送实现，C3 恢复后执行**（backend-chat BE-481）；AC-APN-04-4（【C5】角标口径）随 C5，编号 TC-APN-04-04 留给它。AC-APN-02-11 已废弃（第 16.3 节），TC-APN-02-11 不使用。AC-ACC-04-26 的用例在 [TC-ACC](TC-ACC.md)（TC-ACC-04-30），AC-CHAT-90-13 和【全部】的 C3 部分在 [TC-CHAT](TC-CHAT.md)（TC-CHAT-90-18～25）。
>
> 通用约定：
> - 环境：**E1 + 假通道**（E1 单节点：1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库；再加 `IM_OFFLINE_PUSH_ENABLED=1` 和 BE-406 的假 APNs、假 FCM，`IM_APNS_ENDPOINT`、`IM_FCM_ENDPOINT` 指向它们；`.p8` 和服务账号 JSON 由测试在临时目录现场生成，权限 0600）；**E1 + 假通道 + console-server**（`CON_TEST_MODE=1`，Postgres）；**SDK 单测**（sdk-kotlin、sdk-swift：`FakeTransport` + 假时钟 + 授权、Play 服务、令牌三个替身）；**模拟器 + 替身**（Android 模拟器、iOS Simulator，App 里注入同样的替身，连真实 im-server 测试模式 + 假通道）；**真机 + 真实通道**（P-13 真机 + 第 20.3 节 Frank 的 APNs Key、Firebase 项目和服务账号 + 服务器能访问 Apple、Google）；**EP 性能**（目标机，P-12）。开发机不安装软件。
> - **假通道只验调度和请求格式，不证明 Apple、Google 的行为**；真实投递只有标了"真机 + 真实通道"的用例能证明，未到位记未执行，不算通过。
> - **CI 现在不运行（PENDING P-21）**，所有结果都是本地结果，记录时写明环境和提交号。性能用例在开发机只记录，不算通过。
> - 层级：**后端 e2e**（Go e2e，对 `/ws`、`/admin/*`、假通道记录断言）、**单测**、**SDK 单测**、**原生 UI 测试**（Compose UI Test、XCUITest，模拟器）、**手工**（留截图或录屏）、**性能**。
> - **推送用例一律用账号登录的 token（有 `sid`）**（第 4.5 节，R-CQ-10）；只有 TC-APN-01-07 用演示 token。
> - 负向断言"假通道无调用"用**哨兵法**（第 4.4 节，R-CQ-29）：之后给另一个离线、有令牌的哨兵用户 `s` 发一条，等假通道记录到 `s` 的调用后，再断言目标令牌没有调用。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`。合并窗口 5 秒、补推延迟 30 秒、在线窗口 45 秒都用 [IM 钟] 推进。
> - 记号：`u1` 发送者，`u2` 接收者，`s` 哨兵；`D1`、`D2` 同一账号的设备；`TA1`、`TA2` 是 APNs 令牌（64 个十六进制字符），`TF1`、`TF2` 是 FCM 令牌；"注册" = `deviceNotificationRegister`。"离线" = 该设备没有连接；"后台" = 有连接但上报了 `background`。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-APN-01-01 | 同一令牌换账号注册：只归新账号 | AC-APN-01-1 | 后端 e2e | P0 |
| TC-APN-01-02 | 通道说令牌失效：删除，下一条不再调用 | AC-APN-01-2 | 后端 e2e | P0 |
| TC-APN-01-03 | 9 条吊销路径都删令牌 | AC-APN-01-3 | 后端 e2e | P0 |
| TC-APN-01-04 | 日志和审计搜不到令牌原文 | AC-APN-01-4 | 后端 e2e | P0 |
| TC-APN-01-05 | 拒绝授权：设置页显示已关闭，没有注册 | AC-APN-01-5 | 原生 UI 测试 + SDK 单测 | P0 |
| TC-APN-01-06 | 真机：进入会话列表后才弹授权，同意后收到通知 | AC-APN-01-6 | 手工 | P0 |
| TC-APN-01-07 | 无 sid 的 token：按用户有无连接判断，注销后不推 | AC-APN-01-7 | 后端 e2e | P0 |
| TC-APN-01-08 | 令牌换主人后"显示内容"重置为关 | AC-APN-01-8 | 后端 e2e | P0 |
| TC-APN-01-09 | 入队后、调用前吊销 sid：不调用 | AC-APN-01-9 | 后端 e2e | P0 |
| TC-APN-01-10 | 系统里关、开通知后回前台：依次注销、注册 | AC-APN-01-10 | 原生 UI 测试 + SDK 单测 | P0 |
| TC-APN-01-11 | API 30：系统里关闭通知后回前台注销 | AC-APN-01-11 | 原生 UI 测试（Android） | P0 |
| TC-APN-01-12 | 占位 google-services.json：debug 构建和测试通过 | AC-APN-01-12 | 构建（Android） | P0 |
| TC-APN-01-13 | 注册频率第 11 次 429；空令牌 400 | AC-APN-01-13 | 后端 e2e | P0 |
| TC-APN-02-01 | text、image、file 各推 1 次 | AC-APN-02-1 | 后端 e2e | P0 |
| TC-APN-02-02 | 前台在线不推（哨兵法） | AC-APN-02-2 | 后端 e2e | P0 |
| TC-APN-02-03 | 手机后台 + PC 在线：手机照样推 | AC-APN-02-3 | 后端 e2e | P0 |
| TC-APN-02-04 | 群提示、自己的其他离线设备都不推 | AC-APN-02-4 | 后端 e2e | P0 |
| TC-APN-02-05 | 46 秒没有帧：按不在线推送 | AC-APN-02-5 | 后端 e2e | P0 |
| TC-APN-02-06 | 断线补推：1006、4408 推聚合通知，重连取消，1000、1001 不推 | AC-APN-02-6 | 后端 e2e | P0 |
| TC-APN-02-07 | 5 秒内 10 条：不超过 2 次调用，collapse id 相同 | AC-APN-02-7 | 后端 e2e | P0 |
| TC-APN-02-08 | 通道一直超时：send 的 ack 不受影响 | AC-APN-02-8 | 性能 | P0 |
| TC-APN-02-09 | 503 重试次数、400 不重试、过期时间 | AC-APN-02-9 | 后端 e2e | P0 |
| TC-APN-02-10 | 队列满：丢弃计数，send 不受影响 | AC-APN-02-10 | 后端 e2e | P0 |
| TC-APN-02-12 | 2000 人群 1000 离线：ack P99 和查询次数 | AC-APN-02-12 | 性能 + 后端 e2e | P0 |
| TC-APN-02-13 | 原生连接未上报 presence 按后台；重连上报 background | AC-APN-02-13 | 后端 e2e | P0 |
| TC-APN-02-14 | 上报 foreground 后不推（哨兵法） | AC-APN-02-14 | 后端 e2e | P0 |
| TC-APN-02-15 | 后台切网重连：auth 之后第一帧是 presence background | AC-APN-02-15 | SDK 单测 + 原生 UI 测试 | P0 |
| TC-APN-02-16 | custom、系统通知、好友申请都不推 | AC-APN-02-16 | 后端 e2e | P0 |
| TC-APN-02-17 | 开关关闭：注册 4030，不推 | AC-APN-02-17 | 后端 e2e | P0 |
| TC-APN-03-01 | 默认隐藏：请求头和请求体都搜不到内容和 ID | AC-APN-03-1 | 后端 e2e | P0 |
| TC-APN-03-02 | 详情模式：昵称不用备注，摘要 50 码点，图片文件占位 | AC-APN-03-2 | 后端 e2e | P0 |
| TC-APN-03-03 | 撤回替换通知：最后一条才替换，隐藏模式不发（等 C3） | AC-APN-03-3 | 后端 e2e | P0 |
| TC-APN-03-04 | 打开"显示内容"：先提示第三方通道，重启保留 | AC-APN-03-4 | 原生 UI 测试 | P0 |
| TC-APN-03-05 | 真机锁屏：只显示"天听 / 你收到一条新消息" | AC-APN-03-5 | 手工 | P0 |
| TC-APN-03-06 | 详情模式 collapse id 不含 convId，按设备不同；FCM 没有 collapse_key | AC-APN-03-6 | 后端 e2e | P0 |
| TC-APN-03-07 | 两台设备一台开详情；群消息标题是群名 | AC-APN-03-7 | 后端 e2e | P0 |
| TC-APN-03-08 | 还没有令牌也能保存详情设置 | AC-APN-03-8 | SDK 单测 + 原生 UI 测试 | P0 |
| TC-APN-04-01 | 真机：隐藏模式点通知冷启动进会话列表 | AC-APN-04-1 | 手工 | P0 |
| TC-APN-04-02 | 真机：详情模式点通知进会话 | AC-APN-04-2 | 手工 | P0 |
| TC-APN-04-03 | deep link 的 convId 不属于当前账号：进会话列表 | AC-APN-04-3 | 原生 UI 测试 | P0 |
| TC-APN-04-05 | 300 个离线设备的群：不带 badge | AC-APN-04-5 | 后端 e2e | P0 |
| TC-APN-04-06 | 回到前台清除本 App 的通知 | AC-APN-04-6 | 原生 UI 测试 | P0 |
| TC-APN-05-01 | 生产缺密钥、密钥 0644：拒绝启动，不泄露密钥 | AC-APN-05-1 | 后端单测 + e2e | P0 |
| TC-APN-05-02 | 配置检查显示通道状态，不含密钥 | AC-APN-05-2 | 后端 e2e | P0 |
| TC-APN-05-03 | 真机 + 真实通道：10 秒内收到，失效令牌被删 | AC-APN-05-3 | 手工 | P0 |
| TC-APN-05-04 | 假通道校验 JWT、请求头、OAuth；sandbox 走 sandbox | AC-APN-05-4 | 后端 e2e | P0 |
| TC-APN-05-05 | 非生产缺 FCM 密钥告警关闭；403 计失败 | AC-APN-05-5 | 后端 e2e | P0 |
| TC-APN-06-01 | Play 服务不可用：不注册，显示不支持 | AC-APN-06-1 | 原生 UI 测试（Android）+ SDK 单测 | P0 |
| TC-APN-06-02 | 真机：没有谷歌服务的 Android，打开后消息都在 | AC-APN-06-2 | 手工 | P0 |
| TC-APN-06-03 | 没有令牌的用户：不报错，不调用，sync 有消息 | AC-APN-06-3 | 后端 e2e | P0 |
| TC-APN-91-01 | AC-ACC-03-19 的 C3 部分：refresh 重试后令牌还在 | AC-ACC-03-19 | 后端 e2e | P0 |
| TC-APN-91-02 | AC-ACC-03-27 的 C3 部分：session_purge 同时清令牌 | AC-ACC-03-27 | 后端 e2e | P0 |
| TC-APN-91-03 | AC-ACC-04-6 的 C3 部分：断网退出调用系统令牌注销 | AC-ACC-04-6 | 原生 UI 测试 + SDK 单测 | P0 |

---

## F-APN-01 设备令牌

### TC-APN-01-01 同一令牌换账号注册：只归新账号
- 关联 AC：AC-APN-01-1
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u1、u2 都是账号登录（有 `sid`）；u3 给他们发消息；u1、u2 的连接随后断开
- 步骤：
  1. u1 注册 `{provider:"apns", token:TA1, env:"sandbox", appId:"com.tianting.im"}`
  2. u2 用同一个 `TA1` 注册
  3. 两个连接断开；u3 给 u1 发一条，再给 u2 发一条
  4. 查 `device_push`、假 APNs 记录
- 预期结果：
  1. 两次注册都是 ack `{}`
  2. `device_push` 只有一行 `TA1`，`user_id=u2`、`sid` 是 u2 的
  3. 假 APNs 对 `TA1` 只有 1 次调用（u2 的消息）；u1 的消息没有调用（哨兵法：再给 `s` 发一条，`s` 被调用后断言）

### TC-APN-01-02 通道说令牌失效：删除，下一条不再调用
- 关联 AC：AC-APN-01-2
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 离线，表驱动 4 行，每行一个新令牌：① APNs 410 `{reason:"Unregistered", timestamp:<注册之后>}`；② APNs 410 `ExpiredToken`；③ APNs 400 `BadDeviceToken`；④ FCM 404 `UNREGISTERED`。另加对照行 ⑤ APNs 410 `Unregistered`，`timestamp` **早于**这条令牌最近一次注册时间
- 步骤：
  1. 设定假通道对该令牌的响应；u1 给 u2 发一条
  2. 查 `device_push`；恢复假通道正常；超过合并窗口后 u1 再发一条（哨兵法）
- 预期结果：
  1. ①～④：第 1 条之后令牌行被删除；第 2 条没有调用；`im_offline_push_total{result="token_deleted"}` 增加
  2. ⑤：令牌不删，第 2 条照常调用（backend-chat 第 34 条的建议，Frank 改口径后同步改这一行）
  3. 每行只有 1 次失败调用，不重试

### TC-APN-01-03 9 条吊销路径都删令牌
- 关联 AC：AC-APN-01-3
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道；强制下线、封禁两行另在 E1 + 假通道 + console-server 跑一遍（CS-203）
- 前置条件：每行一个新账号 u，设备 `D1` 注册 `TA1`；改密、退出其他设备的行另有设备 `D2` 发起
- 步骤（表驱动，每行之后 u3 给 u 发一条，哨兵法断言）：
  1. `D1` 退出（`logout`）
  2. `D2` 改密
  3. `D2` 退出其他设备
  4. 找回（`recover`）
  5. refresh 重放（同一个旧 refresh 在转正后再用，触发重放吊销）
  6. 管理接口强制下线
  7. 封禁
  8. 注销（申请到期，`account_cleanup` 运行）
  9. `im-server sessions revoke-all`
- 预期结果：
  1. 每行吊销 `sid` 的那个事务提交后，`device_push` 里 `D1` 的令牌已不存在（同一事务：用 `/admin/test/faults` 让吊销写失败时，令牌也还在，二者一致）
  2. 之后的消息对 `TA1` 没有调用
  3. 改密行：`D2` 自己的令牌按 BE-453 第 3 条换到新 `sid` 后重新注册才有（见 TC-ACC-04-30）

### TC-APN-01-04 日志和审计搜不到令牌原文
- 关联 AC：AC-APN-01-4
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：令牌用特征串 `ZXQTOKEN` 开头的 64 字符十六进制（APNs）和 `ZXQ-FCM-TOKEN` 开头的 FCM 令牌
- 步骤：
  1. 注册、刷新令牌、注销、令牌被通道判失效删除、换账号转移、`deviceNotificationSettings`
  2. 用 `deploy/ops/scan-logs` 扫 im-server 日志、`admin_audit` 和全部表（`device_push.token` 是加密字段，单独断言）
- 预期结果：
  1. 日志和审计搜不到令牌原文，只有 provider 和 SHA-256 前 8 位
  2. `device_push.token` 列不是明文（搜不到特征串）

### TC-APN-01-05 拒绝授权：设置页显示已关闭，没有注册
- 关联 AC：AC-APN-01-5
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android 13+ 模拟器、iOS Simulator）+ SDK 单测
- 环境：模拟器 + 替身；SDK 单测
- 前置条件：授权替身返回"用户拒绝"
- 步骤：
  1. 首次登录进入会话列表，替身记录授权请求后返回拒绝
  2. 打开"设置 → 通知"
  3. 查 `FakeTransport` / 服务端收到的帧
- 预期结果：
  1. 显示"系统通知已关闭"和"前往系统设置"入口；入口打开系统设置（替身记录调用）
  2. 没有发过 `deviceNotificationRegister`

### TC-APN-01-06 真机：进入会话列表后才弹授权，同意后收到通知
- 关联 AC：AC-APN-01-6
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机 + 真实通道（**P-13 真机；第 20.3 节 APNs、FCM 和服务器网络**）
- 前置条件：**未到位前记未执行**。Android 13+（有谷歌服务）一台、iPhone 一台，都是全新安装
- 步骤：
  1. 冷启动，停在登录页，观察有没有授权弹窗
  2. 首次登录进入会话列表
  3. 同意授权，App 退到后台，另一个账号发一条消息；录屏
- 预期结果：
  1. 登录前没有弹窗；进入会话列表后才出现系统授权弹窗
  2. 同意后设备收到通知（默认隐藏模式："天听 / 你收到一条新消息"）

### TC-APN-01-07 无 sid 的 token：按用户有无连接判断，注销后不推
- 关联 AC：AC-APN-01-7
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 用**演示 token**（没有 `sid`）连接并注册 `TA1`
- 步骤：
  1. u2 另有一条活跃连接（前台）时，u1 发一条（哨兵法）
  2. u2 全部断开后，u1 再发一条
  3. u2 重新连接、发 `deviceNotificationUnregister`，断开；u1 再发一条（哨兵法）
- 预期结果：
  1. `device_push` 行 `sid` 为空、`user_id=u2`
  2. 第 1 步无调用；第 2 步 1 次调用；第 3 步无调用，令牌行已删

### TC-APN-01-08 令牌换主人后"显示内容"重置为关
- 关联 AC：AC-APN-01-8
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：A 在设备上注册 `TA1`，`deviceNotificationSettings {showDetail:true}`
- 步骤：
  1. A 退出；B 在同一设备登录并用 `TA1` 注册
  2. B 离线，u1 给 B 发一条含特征串 `ZXQ-DETAIL` 的消息
  3. 抓假 APNs 收到的完整请求
- 预期结果：
  1. B 的 `deviceNotificationSettings {}` 读到 `showDetail=false`
  2. 请求是隐藏模式："天听 / 你收到一条新消息"，搜不到 `ZXQ-DETAIL` 和 u1 的昵称

### TC-APN-01-09 入队后、调用前吊销 sid：不调用
- 关联 AC：AC-APN-01-9
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的 `D1` 注册 `TA1` 后离线；假通道设"调用前暂停"（BE-406 第 3 条）
- 步骤：
  1. u1 发一条，确认已入队、worker 停在调用前
  2. 管理接口强制下线 u2（吊销 `D1` 的 `sid`）
  3. 放开暂停；哨兵法
- 预期结果：
  1. 对 `TA1` 没有调用；`device_push` 已无 `TA1`

### TC-APN-01-10 系统里关、开通知后回前台：依次注销、注册
- 关联 AC：AC-APN-01-10
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android、iOS）+ SDK 单测
- 环境：模拟器 + 替身；SDK 单测
- 前置条件：已授权并注册令牌
- 步骤：
  1. App 退到后台；授权替身改成"已关闭"；回到前台
  2. 再退到后台；改回"已授权"；回到前台
  3. 另测：令牌替身在没有连接时回调新令牌，随后连接成功
- 预期结果：
  1. 第 1 步发 `deviceNotificationUnregister`，设置页显示已关闭
  2. 第 2 步发 `deviceNotificationRegister`（当前令牌）
  3. 第 3 步连接成功后才发注册，令牌是新值；没有连接时不报错

### TC-APN-01-11 API 30：系统里关闭通知后回前台注销
- 关联 AC：AC-APN-01-11
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android）
- 环境：Android 模拟器 API 30 + 替身
- 前置条件：已注册令牌（API 30 没有运行时权限，默认允许）
- 步骤：
  1. 在系统设置里关闭本 App 的通知（`areNotificationsEnabled()` 返回 false）
  2. 回到前台；打开"设置 → 通知"
- 预期结果：
  1. 发 `deviceNotificationUnregister`
  2. 设置页显示"系统通知已关闭"和入口

### TC-APN-01-12 占位 google-services.json：debug 构建和测试通过
- 关联 AC：AC-APN-01-12
- 优先级：P0
- 批次：C3
- 层级：构建（Android）
- 环境：本地 Gradle（**CI 现在不运行，P-21；原生 CI 还缺 P-15**，CI 那一半记未执行）
- 前置条件：仓库里只有占位 `google-services.json`（没有真实项目 ID 和密钥）
- 步骤：
  1. `assembleDebug`、单测、`connectedDebugAndroidTest` 的 C3 用例
  2. 检查仓库历史和构建产物
- 预期结果：
  1. 构建和测试通过
  2. 仓库里没有真实的 Firebase 配置；release 构建缺真实配置时明确失败，不静默用占位

### TC-APN-01-13 注册频率第 11 次 429；空令牌 400
- 关联 AC：AC-APN-01-13
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：同一账号，[IM 钟] 从整点开始
- 步骤：
  1. 1 小时内注册 10 次（可以是不同设备、不同令牌）
  2. 第 11 次注册
  3. 令牌为空；令牌超过 Schema 上限（BE-450 第 1 条）
  4. 推进 [IM 钟] 1 小时后再注册
- 预期结果：
  1. 前 10 次 ack；第 11 次 429 / 4290
  2. 空令牌、超长令牌 400 / 4000，不计入次数
  3. 第 4 步成功

## F-APN-02 什么时候推

### TC-APN-02-01 text、image、file 各推 1 次
- 关联 AC：AC-APN-02-1
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道 + 假 S3（`IM_FILES_ENABLED=1`）
- 前置条件：u2 无连接，有 APNs 和 FCM 两台设备的令牌
- 步骤：
  1. u1 发 `text`；推进 [IM 钟] 超过合并窗口；发 `image`；再推进；发 `file`
- 预期结果：
  1. 每台设备各 3 次调用，每种消息 1 次

### TC-APN-02-02 前台在线不推（哨兵法）
- 关联 AC：AC-APN-02-2
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的 ios 连接已上报 `presence {state:"foreground"}`，45 秒内有帧；u2 这台设备有令牌
- 步骤：
  1. u1 给 u2 发一条
  2. u1 给离线的哨兵 `s` 发一条，等 `s` 被调用
- 预期结果：
  1. `s` 被调用之后，u2 的令牌仍没有调用

### TC-APN-02-03 手机后台 + PC 在线：手机照样推
- 关联 AC：AC-APN-02-3
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的手机连接上报 `background`，手机有令牌；u2 的 `pc` 连接活跃（默认前台）
- 步骤：
  1. u1 发一条
- 预期结果：
  1. 手机令牌有 1 次调用；PC 连接照常收到消息

### TC-APN-02-04 群提示、自己的其他离线设备都不推
- 关联 AC：AC-APN-02-4
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：隐藏模式；群 g1 有 u1、u2，u2 离线有令牌；u1 另一台设备 `D2` 离线有令牌
- 步骤：
  1. 产生一条群提示消息（`tip`，例如拉人入群）
  2. u1 用 `D1` 在 g1 发一条普通消息（u1 的 `D2` 离线）
  3. 哨兵法
- 预期结果：
  1. 第 1 步对 u2 没有调用
  2. 第 2 步 u2 有调用；u1 的 `D2` 没有调用
- 说明：撤回的部分在 AC-RCL-03-9（C4）

### TC-APN-02-05 46 秒没有帧：按不在线推送
- 关联 AC：AC-APN-02-5
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道，`IM_TEST_DISABLE_IDLE=1`（只关服务端 4408）
- 前置条件：u2 的 ios 连接上报 `foreground` 后不再发任何帧
- 步骤：
  1. 推进 [IM 钟] 46 秒
  2. u1 发一条
- 预期结果：
  1. 连接仍在（没有被 4408 断开）；假通道对 u2 有 1 次调用

### TC-APN-02-06 断线补推：1006、4408 推聚合通知，重连取消，1000、1001 不推
- 关联 AC：AC-APN-02-6
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：每行新建 u2；u2 的 ios 连接前台在线，收到 u1 的一条消息后保持未读；u2 这台设备有令牌、没有其他连接
- 步骤（表驱动，断开后推进 [IM 钟]）：
  1. 以 1006 断开，30 秒内同一设备重连
  2. 以 1006 断开，不重连，推进 31 秒
  3. 以 4408 断开，不重连，推进 31 秒
  4. 以 1000 断开；以 1001 断开；各推进 31 秒
- 预期结果：
  1. 第 1 行没有调用（哨兵法）
  2. 第 2、3 行 30 秒后各 1 次调用，内容是聚合通知"你有新消息"（隐藏模式标题"天听"）
  3. 第 4 行都没有调用
  4. 30 秒内（第 29 秒）检查第 2、3 行都还没有调用

### TC-APN-02-07 5 秒内 10 条：不超过 2 次调用，collapse id 相同
- 关联 AC：AC-APN-02-7
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 离线，一台 APNs 设备；隐藏模式和详情模式各跑一遍
- 步骤：
  1. u1 在 5 秒内（[IM 钟]）给 u2 发 10 条
  2. 推进 [IM 钟] 到合并窗口之后
- 预期结果：
  1. 对 u2 的调用 ≤ 2 次
  2. 所有调用的 `apns-collapse-id` 相同，长度 ≤ 64 字节（backend-chat 第 42 条）

### TC-APN-02-08 通道一直超时：send 的 ack 不受影响
- 关联 AC：AC-APN-02-8
- 优先级：P0
- 批次：C3
- 层级：性能
- 环境：E1 + 假通道（开发机，只记录）；EP（P-12，判定）
- 前置条件：生产配置（`IM_TEST_MODE` 只用于假通道端点）；每种 ≥ 1000 条单聊；两种场景：假通道一直超时、假通道正常
- 步骤：
  1. 每种场景跑 3 次，测 `send` 的 ack 延迟
  2. 跑完后接收方 `sync`
- 预期结果：
  1. 全部 ack 成功；通道超时时 ack P99 ≤ 200 ms（未校准）；两种场景的增幅只记录
  2. `sync` 不缺消息
  3. 开发机结果只记录，不算通过

### TC-APN-02-09 503 重试次数、400 不重试、过期时间
- 关联 AC：AC-APN-02-9
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 离线；APNs 和 FCM 各跑一遍；重试间隔按 BE-455 第 5 条的配置，用 [IM 钟] 推进到每次重试
- 步骤（表驱动）：
  1. 假通道一直 503
  2. 503 一次后成功
  3. 返回 400（APNs `BadTopic` 之类非令牌错误；FCM `INVALID_ARGUMENT` 另看第 39 条口径）
  4. 隐藏模式、详情模式各发一条，抓请求
- 预期结果：
  1. 总调用次数依次 3、2、1
  2. 指标 `im_offline_push_total{provider,result}` 按结果计数（`retry_exhausted`、`ok`、`bad_request`）
  3. APNs `apns-expiration` = 发送时间 + 24 小时（隐藏）/ 1 小时（详情）的 epoch 秒；FCM `android.ttl` = `86400s` / `3600s`

### TC-APN-02-10 队列满：丢弃计数，send 不受影响
- 关联 AC：AC-APN-02-10
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道，`IM_PUSH_QUEUE_MAX=10`
- 前置条件：假通道设"阻塞"；u2 离线有令牌
- 步骤：
  1. u1 连续发 30 条
  2. 抓 `/metrics`
- 预期结果：
  1. 30 条全部 ack 成功，u2 `sync` 全有
  2. `im_offline_push_dropped_total` 增加（≥ 19）；`im_offline_push_queue_len` ≤ 10

### TC-APN-02-12 2000 人群 1000 离线：ack P99 和查询次数
- 关联 AC：AC-APN-02-12
- 优先级：P0
- 批次：C3
- 层级：性能 + 后端 e2e
- 环境：E1 + 假通道（开发机，只记录）；EP（P-12，判定）
- 前置条件：2000 人群，1000 人离线有令牌；生产配置；≥ 1000 条，3 次取最差；同时有其他单聊在发
- 步骤：
  1. 群成员持续发消息，同时测其他单聊的 ack
  2. 用测试模式的查询计数统计每条消息 worker 的数据库查询次数
- 预期结果：
  1. 群发送和单聊 ack P99 都 ≤ 200 ms（未校准，开发机不算通过）
  2. 每条消息查询 ≤ 5 次，不随离线人数增长（e2e 部分在开发机就判定）

### TC-APN-02-13 原生连接未上报 presence 按后台；重连上报 background
- 关联 AC：AC-APN-02-13
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的 `platform=android` 连接已 `auth`，还没有发 `presence`；设备有令牌
- 步骤：
  1. u1 发一条
  2. u2 断开重连，`auth` 后上报 `background`；u1 再发一条（超过合并窗口）
- 预期结果：
  1. 两条都有调用

### TC-APN-02-14 上报 foreground 后不推（哨兵法）
- 关联 AC：AC-APN-02-14
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的 android 连接上报 `foreground`
- 步骤：
  1. u1 发一条；哨兵法
- 预期结果：
  1. u2 的令牌没有调用

### TC-APN-02-15 后台切网重连：auth 之后第一帧是 presence background
- 关联 AC：AC-APN-02-15
- 优先级：P0
- 批次：C3
- 层级：SDK 单测（sdk-kotlin、sdk-swift）+ 原生 UI 测试（Android、iOS）
- 环境：SDK 单测；模拟器 + 替身
- 前置条件：App 在后台，连接在线
- 步骤：
  1. 模拟切换网络导致断开重连（SDK 单测：`FakeTransport` 断开后重连；模拟器：切飞行模式再恢复）
  2. 记录重连后的上行帧
- 预期结果：
  1. `auth` 的 ack 之后第一帧是 `presence {state:"background"}`
  2. 回到前台后发 `presence {state:"foreground"}`

### TC-APN-02-16 custom、系统通知、好友申请都不推
- 关联 AC：AC-APN-02-16
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道（`IM_CLIENT_CUSTOM_MSG` 测试默认开）
- 前置条件：u2 离线有令牌
- 步骤：
  1. u1 发 `custom` 消息
  2. 管理接口给 u2 发系统通知（`push` 帧）
  3. u1 给 u2 发好友申请
  4. 哨兵法
- 预期结果：
  1. 三种都没有调用

### TC-APN-02-17 开关关闭：注册 4030，不推
- 关联 AC：AC-APN-02-17
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1，`IM_OFFLINE_PUSH_ENABLED=0`（假通道照常启动，用来证明没有调用）
- 前置条件：u2 离线
- 步骤：
  1. u2 注册令牌；`deviceNotificationSettings`；`presence`
  2. u1 给 u2 发一条
  3. `GET /v1/config`
- 预期结果：
  1. 注册 403 / 4030 `reason=featureDisabled`；`deviceNotificationSettings` 同样（`presence` 的处理按 BE-451 实现时写明）
  2. 假通道没有任何调用
  3. `features.offlinePush=false`

## F-APN-03 内容与隐私

### TC-APN-03-01 默认隐藏：请求头和请求体都搜不到内容和 ID
- 关联 AC：AC-APN-03-1
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：默认设置；u1 昵称 `ZXQ-NICK`；u2 一台 APNs、一台 FCM 设备，离线
- 步骤：
  1. u1 在单聊 c1 发 `ZXQ-PUSH-1`；u1 在群 g1（群名 `ZXQ-GROUP`）发一条；超过合并窗口
  2. 抓假通道收到的完整请求（HTTP/2 伪头、全部请求头、请求体）
  3. 算出 u1、u2 的 userId、c1 和 g1 的 `convId`、两条 `msgId`，以及它们的 SHA-256、MD5 十六进制
- 预期结果：
  1. 标题"天听"，正文"你收到一条新消息"
  2. 搜不到特征串、昵称、群名、任何 userId、`convId`、`msgId` 及上面的普通哈希
  3. 两个会话的 `apns-collapse-id`（APNs）、`android.notification.tag`（FCM）相同（每设备固定）

### TC-APN-03-02 详情模式：昵称不用备注，摘要 50 码点，图片文件占位
- 关联 AC：AC-APN-03-2
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道 + 假 S3
- 前置条件：u2 `showDetail=true`；u2 给 u1 设了备注 `ZXQ-REMARK`；u1 昵称 `Nick`
- 步骤：
  1. u1 发 60 个码点的文本（含 emoji 和控制字符）；发图片；发文件 `ZXQ-FILE.pdf`
  2. 抓请求
- 预期结果：
  1. 标题是 `Nick`，搜不到 `ZXQ-REMARK`
  2. 文本摘要是去掉控制字符后的前 50 个码点（不截断代理对）
  3. 图片正文"[图片]"；文件正文"[文件]"，搜不到文件名
  4. 昵称为空时标题是 `userId`（另起一行数据）

### TC-APN-03-03 撤回替换通知：最后一条才替换，隐藏模式不发（等 C3）
- 关联 AC：AC-APN-03-3
- 优先级：P0
- 批次：C4（依赖 C3，C3 恢复后执行）
- 层级：后端 e2e
- 环境：E1 + 假通道（等 C3：BE-406、BE-455、BE-456；C4 开关打开）
- 前置条件：u2 离线，设备 `D1` 注册 `TA1`；三组：① 详情模式，m1 已推送；② 详情模式，m1、m2 都已推送（超过合并窗口）；③ 隐藏模式，m1 已推送
- 步骤：
  1. 每组 u1 撤回 m1
  2. 给哨兵 `s` 发一条，等 `s` 被调用
- 预期结果：
  1. ① 对 `TA1` 多 1 次调用，`apns-collapse-id` 和 m1 那次相同，内容不含 m1 原文
  2. ② 没有新的调用（不盖掉 m2）
  3. ③ 没有新的调用
  4. **C3 恢复前记"未执行（等 C3）"**（BE-481，PENDING P-43 ②）

### TC-APN-03-04 打开"显示内容"：先提示第三方通道，重启保留
- 关联 AC：AC-APN-03-4
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器 + 替身
- 前置条件：已登录，"设置 → 通知"
- 步骤：
  1. 打开"显示发送者和内容"
  2. 在提示里先点取消；再打开、点确认
  3. 杀掉 App 重启，进设置页
- 预期结果：
  1. 先出现"内容会经过第三方通道"的提示；取消后开关仍是关，没有发 `deviceNotificationSettings`
  2. 确认后发 `{showDetail:true}`，开关是开
  3. 重启后读服务端值，仍是开

### TC-APN-03-05 真机锁屏：只显示"天听 / 你收到一条新消息"
- 关联 AC：AC-APN-03-5
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机 + 真实通道（**P-13；第 20.3 节**）
- 前置条件：**未到位前记未执行**。Android（有谷歌服务）、iPhone 各一台，默认设置，锁屏
- 步骤：
  1. 另一个账号发一条含特征串的消息
  2. 锁屏截图
- 预期结果：
  1. 锁屏上只显示"天听 / 你收到一条新消息"，没有发送者和内容

### TC-APN-03-06 详情模式 collapse id 不含 convId，按设备不同；FCM 没有 collapse_key
- 关联 AC：AC-APN-03-6
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 两台设备（`D1` APNs、`D2` APNs），都开详情；另有一台 FCM 设备 `D3` 开详情；会话 c1、c2
- 步骤：
  1. 在 c1、c2 各发一条，超过合并窗口
  2. 抓 `apns-collapse-id` 和 FCM 请求体
- 预期结果：
  1. `apns-collapse-id` 不等于也不包含 c1、c2 的 `convId`；长度 ≤ 64 字节
  2. 同一会话在 `D1`、`D2` 的值不同；同一设备 c1、c2 的值不同
  3. FCM 请求里没有 `collapse_key`；`android.notification.tag` 不含 `convId`

### TC-APN-03-07 两台设备一台开详情；群消息标题是群名
- 关联 AC：AC-APN-03-7
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 的 `D1` 开详情、`D2` 没开；群 g1 群名 `G-Name`
- 步骤：
  1. u1 在 g1 发 `ZXQ-G`
- 预期结果：
  1. `D1` 的请求标题 `G-Name`、正文含摘要；`D2` 的请求是隐藏模式，搜不到 `ZXQ-G` 和群名

### TC-APN-03-08 还没有令牌也能保存详情设置
- 关联 AC：AC-APN-03-8
- 优先级：P0
- 批次：C3
- 层级：SDK 单测（sdk-kotlin、sdk-swift）+ 原生 UI 测试（Android、iOS）
- 环境：SDK 单测；模拟器 + 替身（令牌替身一直"获取中"）
- 前置条件：设备还没有令牌，没有发过注册
- 步骤：
  1. 打开详情开关并确认
  2. 重新进入设置页（重新读取）
- 预期结果：
  1. `deviceNotificationSettings {showDetail:true}` 的 ack 是 `{showDetail:true}`，保存成功
  2. 重新读取显示开；页面同时显示"令牌获取中"状态

## F-APN-04 点击与角标

### TC-APN-04-01 真机：隐藏模式点通知冷启动进会话列表
- 关联 AC：AC-APN-04-1
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机 + 真实通道（**P-13；第 20.3 节**）
- 前置条件：**未到位前记未执行**。隐藏模式；App 被系统杀死（不是用户划掉）
- 步骤：
  1. 另一个账号发 3 条消息
  2. 点通知
- 预期结果：
  1. App 启动进入会话列表，未读数正确（3）

### TC-APN-04-02 真机：详情模式点通知进会话
- 关联 AC：AC-APN-04-2
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机 + 真实通道（**P-13；第 20.3 节**）
- 前置条件：**未到位前记未执行**。详情模式
- 步骤：
  1. App 在后台和被杀死两种状态下，点通知
- 预期结果：
  1. 直接进入该会话
- 说明：deep link 的路由和异常（不属于当前账号、未登录）在模拟器上由 TC-APN-04-03 覆盖

### TC-APN-04-03 deep link 的 convId 不属于当前账号：进会话列表
- 关联 AC：AC-APN-04-3
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器 + 替身
- 前置条件：当前登录 u2；`convId` 是 u1 和 u3 的单聊
- 步骤：
  1. 用 `tianting://chat/{convId}` 打开 App（模拟点通知）
  2. 另测：未登录时打开；未知路由 `tianting://evil/x`
- 预期结果：
  1. 进会话列表，不显示任何该会话的内容，没有请求它的消息
  2. 未登录先到登录页，登录后进会话列表（账号不同）；未知路由忽略

### TC-APN-04-05 300 个离线设备的群：不带 badge
- 关联 AC：AC-APN-04-5
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：群里 300 个离线设备有 APNs 令牌；对照组：10 个离线设备的群
- 步骤：
  1. 两个群各发一条
- 预期结果：
  1. 300 设备的群的请求都没有 `aps.badge`
  2. 10 设备的群的请求带 `badge`（总未读）

### TC-APN-04-06 回到前台清除本 App 的通知
- 关联 AC：AC-APN-04-6
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android、iOS）
- 环境：模拟器 + 替身
- 前置条件：用本地发通知的方式在通知栏放 2 条本 App 通知（模拟器收不到真实推送）
- 步骤：
  1. 回到前台
  2. 查通知栏（Android `NotificationManager.getActiveNotifications()`；iOS `getDeliveredNotifications`）
- 预期结果：
  1. 2 条都被清除；iOS 角标按本地总未读刷新

## F-APN-05 通道配置

### TC-APN-05-01 生产缺密钥、密钥 0644：拒绝启动，不泄露密钥
- 关联 AC：AC-APN-05-1
- 优先级：P0
- 批次：C3
- 层级：后端单测 + e2e
- 环境：E1（启动检查，不需要假通道）
- 前置条件：`IM_ENV=production`、`IM_OFFLINE_PUSH_ENABLED=1`；测试生成的 `.p8`（内容含特征串注释）
- 步骤：
  1. 不配 `IM_APNS_KEY_FILE` 启动
  2. 配了，但文件权限 0644 启动
  3. FCM 同样两种
- 预期结果：
  1. 都是非 0 退出，错误信息说明缺什么或权限不对
  2. 输出里搜不到密钥内容

### TC-APN-05-02 配置检查显示通道状态，不含密钥
- 关联 AC：AC-APN-05-2
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：APNs、FCM 都配置；先成功推送一次，再让假 FCM 返回 503 一次
- 步骤：
  1. `GET /internal/v1/config`
- 预期结果：
  1. 有 APNs、FCM 两项：是否启用、key id、文件存在、权限正确、最近一次调用结果（APNs 成功、FCM 失败）
  2. 搜不到 `.p8` 内容、服务账号私钥、access token

### TC-APN-05-03 真机 + 真实通道：10 秒内收到，失效令牌被删
- 关联 AC：AC-APN-05-3
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机 + 真实通道（**第 20.3 节：APNs Key、Key ID、Team ID，Firebase 项目、服务账号 JSON、真实 `google-services.json`，服务器能访问 Apple 和 Google；P-13 真机**）
- 前置条件：**未到位前记未执行**。Android（有谷歌服务）、iPhone 各一台，各有一个有效令牌；另各准备一个已失效令牌（卸载重装前的旧令牌）
- 步骤：
  1. 两台设备 App 退到后台，另一个账号各发一条，计时
  2. 给失效令牌所属的测试账号发一条
  3. 查 `device_push`、指标
- 预期结果：
  1. 两台设备都在 10 秒内收到（未校准）
  2. 失效令牌被删除（APNs 410 / 400 `BadDeviceToken`，FCM `UNREGISTERED`，记录实际收到的 reason）
  3. 记录 sandbox / production 端点和实际的响应头，回填 backend-chat 第 34～46 条的"未核实"项

### TC-APN-05-04 假通道校验 JWT、请求头、OAuth；sandbox 走 sandbox
- 关联 AC：AC-APN-05-4
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道（假 APNs 按 sandbox、production 两个主机名各起一个监听）
- 前置条件：u2 两台 iOS 设备，令牌 `env=sandbox`、`env=production`；一台 FCM 设备
- 步骤：
  1. 给 u2 发一条
  2. 推进 [IM 钟] 50 分钟再发一条；推进到 61 分钟内不重签的反例由单测覆盖（时钟注入）
  3. 检查假通道记录
- 预期结果：
  1. APNs：HTTP/2，`:path=/3/device/<token>`，`authorization: bearer <JWT>` 用测试公钥验签通过（`alg=ES256`、`kid`、`iss`、`iat`），`apns-topic=com.tianting.im`，`apns-push-type=alert`；sandbox 令牌打到 sandbox 监听，production 打到 production
  2. 第 2 条之前 JWT 已重签（`iat` 变了），两次签发间隔 ≥ 20 分钟（backend-chat 第 38 条）
  3. FCM：`POST /v1/projects/{project_id}/messages:send`，`Authorization: Bearer` 是假 OAuth 端点签发的 token，签发请求的 scope 是 `firebase.messaging`

### TC-APN-05-05 非生产缺 FCM 密钥告警关闭；403 计失败
- 关联 AC：AC-APN-05-5
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：`IM_ENV=development`，开关打开，不配 FCM 凭证
- 步骤：
  1. 启动，看日志和配置检查
  2. 给 FCM 设备发消息；给 APNs 设备发消息，假 APNs 返回 403 `InvalidProviderToken`
  3. 抓 `/metrics`；检查消息收发
- 预期结果：
  1. 启动成功并告警；FCM 通道关闭，没有 FCM 调用
  2. APNs 403 计失败（`result="auth_error"`），不重试（`ExpiredProviderToken` 的重签重试见第 37 条，另起一行数据）
  3. 消息照常 ack、`sync`

## F-APN-06 没有 FCM 的 Android

### TC-APN-06-01 Play 服务不可用：不注册，显示不支持
- 关联 AC：AC-APN-06-1
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android）+ SDK 单测（sdk-kotlin）
- 环境：模拟器 + 替身（Play 服务可用性替身返回"不可用"；另一行返回"已禁用"）
- 前置条件：已授权通知
- 步骤：
  1. 登录后打开"设置 → 通知"
  2. 查上行帧
- 预期结果：
  1. 没有发 `deviceNotificationRegister`
  2. 显示"此设备暂不支持离线通知，打开 App 才能收到消息"
  3. 替身改成可用后重启，发注册

### TC-APN-06-02 真机：没有谷歌服务的 Android，打开后消息都在
- 关联 AC：AC-APN-06-2
- 优先级：P0
- 批次：C3
- 层级：手工
- 环境：真机（**P-13：没有谷歌服务的国内 Android 真机**；不需要 FCM 账号）
- 前置条件：**未到位前记未执行**
- 步骤：
  1. App 退到后台，等系统杀掉（或在开发者选项里限制后台）
  2. 另一个账号发 3 条
  3. 打开 App
- 预期结果：
  1. 期间没有通知
  2. 打开后 3 条都在（`sync` 补齐）

### TC-APN-06-03 没有令牌的用户：不报错，不调用，sync 有消息
- 关联 AC：AC-APN-06-3
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：u2 从没注册过令牌，离线
- 步骤：
  1. u1 给 u2 发一条；哨兵法
  2. u2 上线 `sync`
- 预期结果：
  1. ack 成功；假通道对 u2 没有调用；服务端日志没有错误
  2. `sync` 里有这条

## 已有 AC 的 C3 部分

C1 的 3 条 AC 写了"推送令牌部分 C3 上线时验"（PRD v1.1 变更记录），主体用例在 TC-ACC，这里补 C3 那一半。

### TC-APN-91-01 AC-ACC-03-19 的 C3 部分：refresh 重试后令牌还在
- 关联 AC：AC-ACC-03-19
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：同 TC-ACC-03-19；`D1` 的 `sid` 注册了 `TA1`
- 步骤：
  1. 按 TC-ACC-03-19 的步骤：R1 刷新后响应丢失，推进 5 分钟、1 小时、3 天依次用 R1 重试
  2. 每次之后 `D1` 离线，u3 发一条
- 预期结果：
  1. `device_push` 的 `TA1` 一直在，绑定的 `sid` 不变
  2. 每次都有调用

### TC-APN-91-02 AC-ACC-03-27 的 C3 部分：session_purge 同时清令牌
- 关联 AC：AC-ACC-03-27
- 优先级：P0
- 批次：C3
- 层级：后端 e2e
- 环境：E1 + 假通道
- 前置条件：同 TC-ACC-03-27；过期 31 天的设备会话绑定了 `TA1`；另一个过期 29 天的会话绑定 `TA2`
- 步骤：
  1. 运行 `session_purge`
  2. 给两个用户各发一条（哨兵法）
- 预期结果：
  1. `TA1` 的会话行和令牌行都被清除，对 `TA1` 没有调用
  2. 过期 29 天的会话行还在（未满 30 天不清）；`sid` 已过期，发送前复查不调用 `TA2`（F-APN-01 规则 4）
  3. 运行后 `device_push` 里没有 `sid` 指向不存在会话的行

### TC-APN-91-03 AC-ACC-04-6 的 C3 部分：断网退出调用系统令牌注销
- 关联 AC：AC-ACC-04-6
- 优先级：P0
- 批次：C3
- 层级：原生 UI 测试（Android、iOS）+ SDK 单测
- 环境：模拟器 + 替身；SDK 单测
- 前置条件：同 TC-ACC-04-06；已注册令牌
- 步骤：
  1. 断网时退出
  2. 恢复网络
- 预期结果：
  1. 本地立即退出时调用了系统令牌注销（Android `SystemTokenRevoker`，iOS `unregisterSystemPushToken`，替身记录）
  2. 恢复网络后服务端收到 logout，`sid` 吊销，`device_push` 里该令牌已删
  3. 同一台设备已有新会话时不撤销系统令牌（frontend-chat 问题 A15）

---

## 说明

- 用例编号按 AC 编号对齐（TC-APN-0X-YY 对应 AC-APN-0X-Y），所以有空号：TC-APN-02-11（AC 已废弃）、TC-APN-04-04（C5）。TC-APN-03-03 是 C4 的，等 C3 恢复后执行。
- TC-APN-01-02、02-09、05-04、05-05 里按 backend-chat 第 34～46 条"建议"一列写的断言（410 的 `timestamp`、重试间隔、JWT 重签、`ExpiredProviderToken`），Frank 改口径后同步改用例，不改 AC。
- 真机用例（01-06、03-05、04-01、04-02、05-03、06-02）共 6 条，挡在 P-13；其中除 06-02 外都还挡在第 20.3 节（PENDING P-40）。
