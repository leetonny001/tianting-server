# 测试用例：账号与鉴权（AUTH）

> 依据：[PRD-core v1.3](../prd/PRD-core-v1.3.md) F-AUTH-01～F-AUTH-05、§5 文案表、§6.1 测试环境要求、§9 C-08/C-11～C-15/C-21/C-44；[ERRORS.md](../../protocol/ERRORS.md) 第 4 节关闭码。
> 管理接口（PRD-core v1.3 F-AUTH-01 规则 3、C-15；VC-01）：`/admin/*`（含测试接口 `/admin/test/*`）只在内网管理地址 `IM_ADMIN_ADDR` 上提供，下文所有管理接口请求都发到该地址；e2e 继续用旧密钥 `X-Admin-Key: test-admin-key`（过渡调用方 `legacy`，非生产环境，不带 `X-IM-Caller`）；`/ws`、`/demo/login` 仍在对外端口。
> 约定：管理密钥通过 `X-Admin-Key` 头传递，测试值 `test-admin-key`；连接地址 `ws://{host}/ws?platform={platform}`，连接后第一帧发送 `auth` 携带 token（PRD v1.2 C-52；下文“以 token=X 连接”均指这一过程）；"测试模式"指 §6.1 的测试环境（token 最短 1 秒，独立数据库）。
> 接口路径（C-15）：强制下线接口 `POST /admin/users/{userId}/force-logout`；演示登录接口 `POST /demo/login`，body `{"username":...}`（测试模式下可带 `ttlSeconds`，C-21）。被踢后的界面行为按 C-01：停留在当前页面弹窗，点"确定"后回到登录页并清空数据。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-AUTH-01-01 | 密钥正确、userId 合法时签发 token 和毫秒过期时间 | AC-AUTH-01-1 | 后端 e2e | P0 |
| TC-AUTH-01-02 | 签发的 token 可以直接用于连接 | AC-AUTH-01-1 | 后端 e2e | P0 |
| TC-AUTH-01-03 | 缺少管理密钥返回 401/4013 | AC-AUTH-01-2 | 后端 e2e | P0 |
| TC-AUTH-01-04 | 管理密钥错误返回 401/4013 | AC-AUTH-01-2 | 后端 e2e | P0 |
| TC-AUTH-01-05 | 密钥错误且 userId 非法时优先返回 4013 | AC-AUTH-01-2 | 后端 e2e | P0 |
| TC-AUTH-01-06 | userId 长度 1 和 64 签发成功 | AC-AUTH-01-3 | 后端 e2e | P0 |
| TC-AUTH-01-07 | userId 长度 0、65 或含非法字符返回 400/4004 | AC-AUTH-01-3 | 后端 e2e | P0 |
| TC-AUTH-01-08 | 不传 / 0 / -1 有效期按 7 天签发 | AC-AUTH-01-4 | 后端 e2e | P0 |
| TC-AUTH-01-09 | 有效期 30 天按 30 天，31 天截为 30 天 | AC-AUTH-01-5 | 后端 e2e | P0 |
| TC-AUTH-01-10 | 非测试模式下 ttlSeconds 小于 60 按 60 秒签发 | AC-AUTH-01-4 | 后端 e2e | P0 |
| TC-AUTH-01-11 | 测试模式下 ttlSeconds=1 按 1 秒签发 | AC-AUTH-01-4 | 后端 e2e | P0 |
| TC-AUTH-02-01 | token 有效时连接成功并可收发消息 | AC-AUTH-02-1 | 后端 e2e | P0 |
| TC-AUTH-02-02 | 四种合法平台均可连接 | AC-AUTH-02-1 | 后端 e2e | P0 |
| TC-AUTH-02-03 | token 无效时先升级、下发 kicked/tokenInvalid、以 4010 关闭 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-04 | token 缺失时下发 kicked/tokenInvalid、以 4010 关闭 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-05 | 篡改签名的 token 被判无效 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-06 | token 已过期时下发 kicked/tokenExpired、以 4011 关闭 | AC-AUTH-02-3 | 后端 e2e | P0 |
| TC-AUTH-02-07 | SDK 收到 4010/4011 后不再重连 | AC-AUTH-02-4 | SDK 单测 | P0 |
| TC-AUTH-02-08 | 4010 时弹窗提示"登录已失效"，点确定回到登录页 | AC-AUTH-02-4 | UI 自动化 | P0 |
| TC-AUTH-02-09 | 4011 时弹窗提示"登录已过期"，点确定回到登录页 | AC-AUTH-02-4 | UI 自动化 | P0 |
| TC-AUTH-02-10 | 同一账号 web + pc 同时在线 | AC-AUTH-02-5 | 后端 e2e | P0 |
| TC-AUTH-02-11 | 同一账号同一平台 web 连两次都在线 | AC-AUTH-02-5 | 后端 e2e | P0 |
| TC-AUTH-02-12 | platform 为空、缺失或为 tv 时以 4009 关闭 | AC-AUTH-02-6 | 后端 e2e | P0 |
| TC-AUTH-02-13 | platform 只接受小写 | AC-AUTH-02-6 | 后端 e2e | P0 |
| TC-AUTH-02-14 | 连接错误优先级：4009 先于 4010/4011 | AC-AUTH-02-6 | 后端 e2e | P0 |
| TC-AUTH-02-15 | 连接后 5 秒内不发 auth，以 4401 关闭 | AC-AUTH-02-7 | 后端 e2e | P0 |
| TC-AUTH-02-16 | auth 之前发送其他命令，以 4401 关闭 | AC-AUTH-02-7 | 后端 e2e | P0 |
| TC-AUTH-02-17 | 只在 URL 携带 token 不会上线 | AC-AUTH-02-8 | 后端 e2e | P0 |
| TC-AUTH-02-18 | auth 成功返回 userId 和 serverTime；重复 auth 返回 4000 | AC-AUTH-02-9 | 后端 e2e | P0 |
| TC-AUTH-02-19 | 第 11 个连接上线时最早的连接被踢（4429） | AC-AUTH-02-10 | 后端 e2e | P0 |
| TC-AUTH-02-20 | SDK 收到 4429 不重连并提示；收到 serverShutdown 退避重连 | AC-AUTH-02-10 | SDK 单测 | P0 |
| TC-AUTH-02-21 | Origin 白名单：非白名单 403，白名单和无 Origin 可连 | AC-AUTH-02-11 | 后端 e2e | P0 |
| TC-AUTH-03-01 | 在线期间 token 到期收到 kicked/tokenExpired、以 4011 关闭 | AC-AUTH-03-1 | 后端 e2e | P0 |
| TC-AUTH-03-02 | 到期前连接正常可用 | AC-AUTH-03-1 | 后端 e2e | P0 |
| TC-AUTH-03-03 | 强制下线后该用户所有连接收到 kicked/banned、以 4012 关闭 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-04 | 强制下线不影响其他用户 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-05 | kicked 帧完整发送后才关闭连接 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-06 | 强制下线后旧 token 连接收到 kicked/banned、以 4012 关闭 | AC-AUTH-03-3 | 后端 e2e | P0 |
| TC-AUTH-03-07 | 强制下线前签发但从未使用的 token 也作废 | AC-AUTH-03-3 | 后端 e2e | P0 |
| TC-AUTH-03-08 | 强制下线后重新签发的 token 可正常连接 | AC-AUTH-03-4 | 后端 e2e | P0 |
| TC-AUTH-03-09 | 强制下线后 1 秒内立即重签的 token 也可用 | AC-AUTH-03-4 | 后端 e2e | P0 |
| TC-AUTH-03-10 | 被踢（过期）后停留原页弹窗"登录已过期"、不重连，确定后回登录页 | AC-AUTH-03-5 | UI 自动化 | P0 |
| TC-AUTH-03-11 | 被踢（强制下线）后停留原页弹窗"账号已被下线"、不重连，确定后回登录页 | AC-AUTH-03-5 | UI 自动化 | P0 |
| TC-AUTH-03-12 | SDK 收到 kicked 后把发送中的消息标记失败 | AC-AUTH-03-6 | SDK 单测 | P0 |
| TC-AUTH-03-13 | 被踢弹窗期间发送中的消息在当前页显示失败 | AC-AUTH-03-6 | UI 自动化 | P0 |
| TC-AUTH-03-14 | 同一秒内：下线前签发的作废、下线后签发的可用（iatMs） | AC-AUTH-03-4 | 后端 e2e | P0 |
| TC-AUTH-03-15 | 强制下线不存在的用户返回 404/4042 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-16 | 已被强制下线且已过期的 token 报 banned/4012 | AC-AUTH-03-3 | 后端 e2e | P0 |
| TC-AUTH-03-17 | 强制下线接口缺少或错误密钥返回 401/4013 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-04-01 | 开启演示登录后输入用户名可登录并连接 | AC-AUTH-04-1 | UI 自动化 | P0 |
| TC-AUTH-04-02 | 演示登录接口返回的 token 可连接（接口层） | AC-AUTH-04-1 | 后端 e2e | P0 |
| TC-AUTH-04-03 | 演示登录即自动注册 | AC-AUTH-04-1 | 后端 e2e | P0 |
| TC-AUTH-04-04 | 前端构建产物中搜不到管理密钥 | AC-AUTH-04-2 | UI 自动化 | P0 |
| TC-AUTH-04-05 | 登录过程 HAR 中搜不到管理密钥 | AC-AUTH-04-2 | UI 自动化 | P0 |
| TC-AUTH-04-06 | 默认配置下演示登录接口返回 404 | AC-AUTH-04-3 | 后端 e2e | P0 |
| TC-AUTH-04-07 | 用户名含空格时提示"账号格式不正确" | AC-AUTH-04-4 | UI 自动化 | P0 |
| TC-AUTH-04-08 | 其他非法用户名均提示"账号格式不正确" | AC-AUTH-04-4 | UI 自动化 | P0 |
| TC-AUTH-04-09 | 演示登录用户名去首尾空白、空白返回 4004（接口层） | AC-AUTH-04-4 | 后端 e2e | P0 |
| TC-AUTH-04-10 | 登录页输入首尾带空格的用户名可登录、纯空白提示格式不正确 | AC-AUTH-04-4 | UI 自动化 | P0 |
| TC-AUTH-04-11 | 演示登录 token 有效期 1 天，测试模式可传 ttlSeconds | AC-AUTH-04-1 | 后端 e2e | P0 |
| TC-AUTH-05-01 | 退出登录回到登录页、刷新仍停留、服务端记录 1000 | AC-AUTH-05-1 | UI 自动化 | P0 |
| TC-AUTH-05-02 | 主动关闭（1000）后 SDK 不重连 | AC-AUTH-05-1 | SDK 单测 | P0 |
| TC-AUTH-05-03 | 退出后清空内存数据，换账号登录看不到上个账号的数据 | AC-AUTH-05-1 | UI 自动化 | P0 |
| TC-AUTH-05-04 | 一个标签页退出不影响另一个标签页 | AC-AUTH-05-1 | UI 自动化 | P0 |

---

### TC-AUTH-01-01 密钥正确、userId 合法时签发 token 和毫秒过期时间
- 关联 AC：AC-AUTH-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动，独立数据库；alice 未被封禁（AC-AUTH-01-1 v1.3 前置，默认即未封禁；封禁场景见 PRD-admin F-ADM-13）。
- 步骤：
  1. 记录当前时间 T（毫秒）。
  2. `POST /admin/users/token`，头 `X-Admin-Key: test-admin-key`，body `{"userId":"alice"}`。
- 预期结果：
  1. HTTP 200，body 含非空字符串 `token` 和整数 `expiresAt`。
  2. `expiresAt` 是毫秒时间戳（13 位，且 > T）。
- 变更：v1.3（VA-03）

### TC-AUTH-01-02 签发的 token 可以直接用于连接
- 关联 AC：AC-AUTH-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同上。
- 步骤：
  1. 为 alice 签发 token。
  2. 用该 token、platform=web 连接 `/ws`，发送 `{"id":"p1","cmd":"ping"}`。
- 预期结果：
  1. 连接建立，未收到 `kicked`。
  2. 收到 `pong`。

### TC-AUTH-01-03 缺少管理密钥返回 401/4013
- 关联 AC：AC-AUTH-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. `POST /admin/users/token`，不带 `X-Admin-Key`，body `{"userId":"alice"}`。
  2. 带空值 `X-Admin-Key: `（空串）再调一次。
- 预期结果：
  1. 两次均 HTTP 401，body.code = 4013（不是 4010）。
  2. 响应体不含 `token` 字段。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-AUTH-01-04 管理密钥错误返回 401/4013
- 关联 AC：AC-AUTH-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. `X-Admin-Key: wrong`，body `{"userId":"alice"}`。
  2. `X-Admin-Key: test-admin-key-x`（正确密钥加后缀）。
  3. `X-Admin-Key: TEST-ADMIN-KEY`（大小写不同）。
- 预期结果：
  1. 三次均 HTTP 401，body.code = 4013，无 `token`。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-AUTH-01-05 密钥错误且 userId 非法时优先返回 4013
- 关联 AC：AC-AUTH-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. `X-Admin-Key: wrong`，body `{"userId":"bad id"}`。
- 预期结果：
  1. HTTP 401，body.code = 4013（先校验密钥，不泄露参数校验结果）。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-AUTH-01-06 userId 长度 1 和 64 签发成功
- 关联 AC：AC-AUTH-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 分别为 userId = `a`、`a`×64、`A_z-09` 签发 token。
- 预期结果：
  1. 三次均 HTTP 200，返回 token；用 `a`×64 的 token 连接成功。

### TC-AUTH-01-07 userId 长度 0、65 或含非法字符返回 400/4004
- 关联 AC：AC-AUTH-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 分别以 userId = `""`、`a`×65、`a b`、`张三`、`a.b`、`a@b` 签发 token。
- 预期结果：
  1. 每次均 HTTP 400，body.code = 4004（不是 4000），无 `token`。

### TC-AUTH-01-08 不传 / 0 / -1 有效期按 7 天签发
- 关联 AC：AC-AUTH-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 记录 T，签发 `{"userId":"alice"}`。
  2. 记录 T，签发 `{"userId":"alice","ttlSeconds":0}`。
  3. 记录 T，签发 `{"userId":"alice","ttlSeconds":-1}`。
- 预期结果：
  1. 每次 `expiresAt` − T 在 604800000 ± 5000 毫秒内（7 天 ± 5 秒）。

### TC-AUTH-01-09 有效期 30 天按 30 天，31 天截为 30 天
- 关联 AC：AC-AUTH-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 记录 T，签发 `ttlSeconds`=2592000（30 天）。
  2. 记录 T，签发 `ttlSeconds`=2678400（31 天）。
  3. 记录 T，签发 `ttlSeconds`=2592001（30 天 + 1 秒）。
- 预期结果：
  1. 三次 `expiresAt` − T 均在 2592000000 ± 5000 毫秒内。

### TC-AUTH-01-10 非测试模式下 ttlSeconds 小于 60 按 60 秒签发
- 关联 AC：AC-AUTH-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端以**非测试模式**启动。
- 步骤：
  1. 记录 T，分别签发 `ttlSeconds` = 1、59、60、61。
  2. 记录 T，签发 `ttlSeconds` = 0。
- 预期结果：
  1. 1、59、60 → `expiresAt` − T 在 60000 ± 5000 毫秒内；61 → 61000 ± 5000 毫秒（C-13）。
  2. 0 → 仍按 7 天（604800000 ± 5000 毫秒）。

### TC-AUTH-01-11 测试模式下 ttlSeconds=1 按 1 秒签发
- 关联 AC：AC-AUTH-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端测试模式。
- 步骤：
  1. 记录 T，签发 `ttlSeconds`=1。
- 预期结果：
  1. `expiresAt` − T 在 1000 ± 500 毫秒内（不被抬到 60 秒）。

### TC-AUTH-02-01 token 有效时连接成功并可收发消息
- 关联 AC：AC-AUTH-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 各签发有效 token。
- 步骤：
  1. alice 以 platform=web、bob 以 platform=android 连接。
  2. alice 给 bob 发单聊 "hi bob"（`clientMsgId`=`cid-a2-0001`）。
  3. bob 回复 "hi alice"（`clientMsgId`=`cid-a2-0002`）。
- 预期结果：
  1. 两个连接都未收到 `kicked`。
  2. alice 收到成功 ack（`seq`=1，`convId`=`c2c:alice:bob`）；bob 收到 `msg`，`from`=alice、文本 "hi bob"。
  3. bob 的 ack `seq`=2；alice 收到对应 `msg`。

### TC-AUTH-02-02 四种合法平台均可连接
- 关联 AC：AC-AUTH-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有效 token。
- 步骤：
  1. 分别以 platform = `android`、`ios`、`web`、`pc` 连接并发送 `ping`。
- 预期结果：
  1. 4 个连接均收到 `pong`，未被关闭。

### TC-AUTH-02-03 token 无效时先升级、下发 kicked/tokenInvalid、以 4010 关闭
- 关联 AC：AC-AUTH-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 以 token=`bad-token`、platform=web 连接，读取帧直到连接关闭。
- 预期结果：
  1. 握手返回 HTTP 101（不再是 HTTP 401）。
  2. 收到 `{"cmd":"event","data":{"type":"kicked","reason":"tokenInvalid"}}`。
  3. 随后关闭码 4010。
- 备注：现有 `server/e2e/e2e_test.go` 的 `TestRejectsInvalidToken` 断言 HTTP 401，需按本用例改写。

### TC-AUTH-02-04 token 缺失时下发 kicked/tokenInvalid、以 4010 关闭
- 关联 AC：AC-AUTH-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 连接 `/ws?platform=web`（无 token 参数）。
  2. 连接 `/ws?token=&platform=web`（空 token）。
- 预期结果：
  1. 两次均升级成功，收到 `kicked`/`tokenInvalid`，关闭码 4010。

### TC-AUTH-02-05 篡改签名的 token 被判无效
- 关联 AC：AC-AUTH-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：为 alice 签发有效 token T。
- 步骤：
  1. 修改 T 签名段最后一个字符后连接。
  2. 把 T 的载荷段中 userId 改为 `bob`（保留原签名）后连接。
  3. 用另一个密钥自行签出的 alice token 连接。
- 预期结果：
  1. 三次均收到 `kicked`/`tokenInvalid`，关闭码 4010；步骤 2 不会以 bob 身份上线。

### TC-AUTH-02-06 token 已过期时下发 kicked/tokenExpired、以 4011 关闭
- 关联 AC：AC-AUTH-02-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；为 alice 签发 `ttlSeconds`=1 的 token。
- 步骤：
  1. 等待 2 秒。
  2. 用该 token、platform=web 连接。
- 预期结果：
  1. 升级成功，收到 `kicked`/`tokenExpired`（不是 `tokenInvalid`）。
  2. 关闭码 4011。

### TC-AUTH-02-07 SDK 收到 4010/4011 后不再重连
- 关联 AC：AC-AUTH-02-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；**演示会话（`mode=demo`，PRD-core v1.5）**。账号会话收到 4011 先刷新，见 TC-ACC。
- 步骤：
  1. SDK 连接；假传输层下发 `kicked`/`tokenInvalid` 后以 4010 关闭。
  2. 假时钟推进 120 秒（超过退避上限 30 秒的 4 倍）。
  3. 重新初始化场景，改为 `kicked`/`tokenExpired` + 4011，重复步骤 2。
- 预期结果：
  1. 两种情况下，推进期间假传输层的连接尝试次数为 0。
  2. SDK 对上层抛出"被踢"事件，原因分别为 tokenInvalid、tokenExpired；连接状态为未连接。

### TC-AUTH-02-08 4010 时弹窗提示"登录已失效"，点确定回到登录页
- 关联 AC：AC-AUTH-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：开启演示登录；alice 登录 H5 进入会话列表；Playwright 记录 WebSocket。
- 步骤：
  1. 在 `sessionStorage` 中把 token 改为 `bad-token` 后刷新页面。
  2. 不点击弹窗，等待 60 秒，统计新建的 WebSocket 数量。
  3. 点击弹窗的"确定"。
- 预期结果：
  1. 页面停留在会话列表（URL 不变），弹出提示框，内容"登录已失效，请重新登录"。
  2. 收到 4010 关闭后 60 秒内没有再发起任何 `/ws` 连接，弹窗一直显示。
  3. 点"确定"后跳转登录页，`sessionStorage` 中无 token。

### TC-AUTH-02-09 4011 时弹窗提示"登录已过期"，点确定回到登录页
- 关联 AC：AC-AUTH-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：测试模式；**演示会话（PRD-core v1.5）**；用管理接口为 alice 签发 `ttlSeconds`=1 的 token，等待 2 秒；H5 已打开。
- 步骤：
  1. 把该过期 token 写入 `sessionStorage` 后刷新页面。
  2. 不点击弹窗，等待 60 秒，统计 `/ws` 连接数。
  3. 点击弹窗的"确定"。
- 预期结果：
  1. 页面停留在原页面，弹出提示框，内容"登录已过期，请重新登录"。
  2. 60 秒内只出现 1 次 `/ws` 连接（即被 4011 关闭的那次）。
  3. 点"确定"后跳转登录页。

### TC-AUTH-02-10 同一账号 web + pc 同时在线
- 关联 AC：AC-AUTH-02-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 有效 token。
- 步骤：
  1. alice 以 platform=web 连接为 A1，再以 platform=pc 连接为 A2。
  2. 等待 1 秒后，bob 给 alice 发 "multi"。
  3. A1、A2 各发一次 `ping`。
- 预期结果：
  1. A1 未被关闭、未收到 `kicked`（不会因 A2 上线而互踢）。
  2. A1、A2 都收到 "multi" 的 `msg`。
  3. 两端都收到 `pong`。

### TC-AUTH-02-11 同一账号同一平台 web 连两次都在线
- 关联 AC：AC-AUTH-02-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同上。
- 步骤：
  1. alice 以 platform=web 连接两次（W1、W2，可用同一个 token）。
  2. bob 给 alice 发 "twin"。
- 预期结果：
  1. W1、W2 都在线，都收到 "twin"；无 `kicked`/`replaced`。

### TC-AUTH-02-12 platform 为空、缺失或为 tv 时以 4009 关闭
- 关联 AC：AC-AUTH-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有效 token。
- 步骤：
  1. 连接 `/ws?token={t}&platform=`。
  2. 连接 `/ws?token={t}&platform=tv`。
  3. 连接 `/ws?token={t}`（无 platform 参数）。
- 预期结果：
  1. 三次连接均以关闭码 4009 关闭。
  2. 关闭前不收到任何数据帧（不发 `kicked`，C-05）。

### TC-AUTH-02-13 platform 只接受小写
- 关联 AC：AC-AUTH-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有效 token。
- 步骤：
  1. 分别以 platform = `Web`、`PC`、`IOS`、`Android` 连接。
- 预期结果：
  1. 四次均不收到任何数据帧，直接以关闭码 4009 关闭（C-14、C-05）。

### TC-AUTH-02-14 连接错误优先级：4009 先于 4010/4011
- 关联 AC：AC-AUTH-02-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；alice 已过期 token（`ttlSeconds`=1，等待 2 秒）。
- 步骤：
  1. 以 `token=bad-token&platform=tv` 连接。
  2. 以 `token=<过期 token>&platform=tv` 连接。
  3. 以 `token=bad-token&platform=web` 连接。
- 预期结果：
  1. 步骤 1、2：不发 kicked，以 4009 关闭（C-08：平台最先检查）。
  2. 步骤 3：`kicked`/`tokenInvalid` + 4010。

### TC-AUTH-02-15 连接后 5 秒内不发 auth，以 4401 关闭
- 关联 AC：AC-AUTH-02-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 以 `platform=web` 建立连接，不发送任何帧。
  2. 等待 6 秒。
- 预期结果：
  1. 约 5 秒时连接以 4401 关闭；关闭前没有收到任何帧（不发 `kicked`）。

### TC-AUTH-02-16 auth 之前发送其他命令，以 4401 关闭
- 关联 AC：AC-AUTH-02-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：bob 已注册。
- 步骤：
  1. 以 `platform=web` 建立连接，第一帧发送 `send`（发给 bob）。
  2. 另建一个连接，第一帧发送 `ping`。
- 预期结果：
  1. 两个连接都以 4401 关闭，没有收到 ack 或 `pong`。
  2. bob 没有收到消息，消息没有落库。

### TC-AUTH-02-17 只在 URL 携带 token 不会上线
- 关联 AC：AC-AUTH-02-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 的有效 token。
- 步骤：
  1. 以 `ws://{host}/ws?token=<有效 token>&platform=web` 建立连接，不发送 `auth`。
- 预期结果：
  1. 5 秒后以 4401 关闭；期间 alice 不在线（给 alice 发的消息不会推送到这个连接）。

### TC-AUTH-02-18 auth 成功返回 userId 和 serverTime；重复 auth 返回 4000
- 关联 AC：AC-AUTH-02-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式，服务端时钟冻结在 T。
- 步骤：
  1. 建立连接，发送 `{"id":"a1","cmd":"auth","data":{"token":<alice 的 token>}}`。
  2. 再发送一次 `auth`（id 为 a2）。
  3. 发送 `ping`。
- 预期结果：
  1. 步骤 1 的 ack：`code`=0，`data.userId`=alice，`data.serverTime`=T。
  2. 步骤 2 返回 4000，连接不断开。
  3. 收到 `pong`。

### TC-AUTH-02-19 第 11 个连接上线时最早的连接被踢（4429）
- 关联 AC：AC-AUTH-02-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 的有效 token；bob 已注册。
- 步骤：
  1. alice 依次建立 C1～C10 共 10 个连接。
  2. 建立第 11 个连接 C11。
  3. bob 给 alice 发一条消息。
- 预期结果：
  1. 步骤 1 后 10 个连接都在线，没有收到 `kicked`。
  2. 步骤 2 后只有 C1 收到 `kicked/tooManyConnections` 并以 4429 关闭。
  3. C2～C11 都收到 bob 的消息，C1 没有收到。

### TC-AUTH-02-20 SDK 收到 4429 不重连并提示；收到 serverShutdown 退避重连
- 关联 AC：AC-AUTH-02-10
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层。
- 步骤：
  1. 假传输层下发 `kicked/tooManyConnections` 后以 4429 关闭。
  2. 重新初始化，下发 `kicked/serverShutdown` 后以 1001 关闭。
- 预期结果：
  1. 步骤 1：不发起重连，向上层抛出 `tooManyConnections`。
  2. 步骤 2：按退避策略重连（1 秒左右发起第一次重连），不向上层抛出被踢事件。

### TC-AUTH-02-21 Origin 白名单：非白名单 403，白名单和无 Origin 可连
- 关联 AC：AC-AUTH-02-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端配置 `IM_ALLOWED_ORIGINS=https://im.example.com`。
- 步骤：
  1. 以 `Origin: https://evil.example.com` 发起握手。
  2. 以 `Origin: https://im.example.com` 发起握手并完成 auth。
  3. 不带 Origin 头发起握手并完成 auth（模拟原生端）。
- 预期结果：
  1. 步骤 1：HTTP 403，握手失败。
  2. 步骤 2、3：连接成功，auth 返回 ack。

### TC-AUTH-03-01 在线期间 token 到期收到 kicked/tokenExpired、以 4011 关闭
- 关联 AC：AC-AUTH-03-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式。
- 步骤：
  1. 为 alice 签发 `ttlSeconds`=2 的 token，立即以 platform=web 连接，记录连接时刻 T0。
  2. 持续读取帧，最多等 3 秒。
- 预期结果：
  1. 在 T0 + 3 秒内收到 `kicked`/`tokenExpired`。
  2. 随后连接以关闭码 4011 关闭。

### TC-AUTH-03-02 到期前连接正常可用
- 关联 AC：AC-AUTH-03-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；bob 在线。
- 步骤：
  1. 为 alice 签发 `ttlSeconds`=2 的 token 并连接。
  2. 第 0.5 秒时 alice 给 bob 发 "before-expire"，并发 `ping`。
- 预期结果：
  1. send 成功、收到 `pong`；bob 收到消息。
  2. 在约 2 秒处才收到 `kicked`/`tokenExpired`（不早于 1.5 秒）。

### TC-AUTH-03-03 强制下线后该用户所有连接收到 kicked/banned、以 4012 关闭
- 关联 AC：AC-AUTH-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 platform=web、pc、ios 共 3 个连接在线。
- 步骤：
  1. 调用 `POST /admin/users/alice/force-logout`。
- 预期结果：
  1. 接口返回 HTTP 200。
  2. 3 个连接都收到 `kicked`/`banned`，随后都以关闭码 4012 关闭（1 秒内完成）。

### TC-AUTH-03-04 强制下线不影响其他用户
- 关联 AC：AC-AUTH-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob、carol 均在线。
- 步骤：
  1. 强制下线 alice。
  2. 1 秒后 bob 给 carol 发 "still-here"，bob 发 `ping`。
- 预期结果：
  1. bob、carol 未收到 `kicked`，连接未关闭。
  2. send 成功，carol 收到 "still-here"；bob 收到 `pong`。

### TC-AUTH-03-05 kicked 帧完整发送后才关闭连接
- 关联 AC：AC-AUTH-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线；连接上有持续的下行流量（bob 以 20 条/秒给 alice 发消息，持续进行中）。
- 步骤：
  1. 在下行流量中途强制下线 alice，循环 20 次。
  2. 每次读取 alice 连接上的全部帧直到关闭。
- 预期结果：
  1. 20 次中每次最后一个数据帧都是可完整解析的 `kicked`/`banned` JSON，紧接着关闭码 4012。
  2. 没有一次出现"收到关闭码但没收到 kicked"或 kicked 帧被截断（JSON 解析失败）。

### TC-AUTH-03-06 强制下线后旧 token 连接收到 kicked/banned、以 4012 关闭
- 关联 AC：AC-AUTH-03-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：为 alice 签发 token T1（7 天有效），用 T1 连接后断开。
- 步骤：
  1. 强制下线 alice。
  2. 用 T1 以 platform=web 重新连接。
- 预期结果：
  1. 收到 `kicked`/`banned`（不是 `tokenInvalid`）。
  2. 关闭码 4012。

### TC-AUTH-03-07 强制下线前签发但从未使用的 token 也作废
- 关联 AC：AC-AUTH-03-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：为 alice 先后签发 T1、T2，均未连接过。
- 步骤：
  1. 强制下线 alice（此时 alice 无在线连接）。
  2. 分别用 T1、T2 连接。
- 预期结果：
  1. `POST /admin/users/alice/force-logout` 返回 HTTP 200（无在线连接也成功）。
  2. T1、T2 均收到 `kicked`/`banned`，关闭码 4012。

### TC-AUTH-03-08 强制下线后重新签发的 token 可正常连接
- 关联 AC：AC-AUTH-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 已被强制下线；bob 在线。
- 步骤：
  1. 等待 2 秒后为 alice 签发新 token T3。
  2. 用 T3 连接，给 bob 发 "back"。
  3. 再用旧 token T1 连接一次。
- 预期结果：
  1. T3 连接成功，send 成功，bob 收到 "back"。
  2. T1 仍收到 `kicked`/`banned` + 4012（重签不会让旧 token 复活）。

### TC-AUTH-03-09 强制下线后 1 秒内立即重签的 token 也可用
- 关联 AC：AC-AUTH-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有一个旧 token。
- 步骤：
  1. 强制下线 alice，接口返回后立即（< 100 毫秒，同一秒内）签发新 token T4。
  2. 用 T4 连接并发 `ping`；循环 10 次。
- 预期结果：
  1. 10 次 T4 都连接成功并收到 `pong`，无 `kicked`/`banned`（C-11：按 `iatMs` 毫秒比较）。

### TC-AUTH-03-10 被踢（过期）后停留原页弹窗"登录已过期"、不重连，确定后回登录页
- 关联 AC：AC-AUTH-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：测试模式；登录时 `POST /demo/login` 带 `ttlSeconds`=5（C-21，可由 Playwright 改写登录请求实现）；alice 登录后打开与 bob 的聊天页。
- 步骤：
  1. 等待 token 到期（约 5 秒）。
  2. 不点击弹窗，再等待 60 秒，统计 `/ws` 连接数。
  3. 点击"确定"。
- 预期结果：
  1. 页面停留在聊天页（URL 仍为 `/chat/c2c:alice:bob`），弹窗提示"登录已过期，请重新登录"。
  2. 到期后 60 秒内没有新的 `/ws` 连接。
  3. 点"确定"后跳转登录页，`sessionStorage` 中无 token。

### TC-AUTH-03-11 被踢（强制下线）后停留原页弹窗"账号已被下线"、不重连，确定后回登录页
- 关联 AC：AC-AUTH-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在两个浏览器上下文都已登录：上下文 1 打开 `/`（platform=web），上下文 2 打开 `/?platform=pc`（C-44）；均停在会话列表。
- 步骤：
  1. 调用 `POST /admin/users/alice/force-logout`。
  2. 不点击弹窗，等待 60 秒，统计两个上下文的 `/ws` 连接数。
  3. 两个上下文分别点"确定"，然后刷新页面。
- 预期结果：
  1. 两个上下文都停留在会话列表，弹窗提示"账号已被下线"。
  2. 60 秒内两个上下文都没有新的 `/ws` 连接。
  3. 点"确定"后跳转登录页；刷新后仍停留登录页（旧 token 已清空，不会自动登录）。

### TC-AUTH-03-12 SDK 收到 kicked 后把发送中的消息标记失败
- 关联 AC：AC-AUTH-03-6
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；已连接。
- 步骤：
  1. SDK 依次发送 3 条消息（`cid-k-1`～`cid-k-3`）；假传输层只回 `cid-k-1` 的成功 ack，另两条不回。
  2. 假传输层下发 `kicked`/`banned` 并以 4012 关闭。
- 预期结果：
  1. `cid-k-1` 状态为已发送。
  2. `cid-k-2`、`cid-k-3` 立即变为发送失败（不等发送超时）。
  3. SDK 不再重连，也不自动重发这两条。

### TC-AUTH-03-13 被踢弹窗期间发送中的消息在当前页显示失败
- 关联 AC：AC-AUTH-03-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：服务端测试模式开启故障注入；alice、bob 两个上下文登录，alice 打开与 bob 的聊天页。
- 步骤：
  1. 配置故障注入：丢弃 alice 下一条消息的 ack（按 `clientMsgId`，可通过拦截 WS 获取）。
  2. alice 发送 "pending-msg"，在显示"发送中"时调用 `POST /admin/users/alice/force-logout`。
  3. 弹窗显示期间（不点确定）检查聊天页；然后点"确定"。
- 预期结果：
  1. 页面停留在聊天页并弹窗"账号已被下线"；弹窗背后 "pending-msg" 显示为发送失败（C-01）。
  2. 点"确定"后跳转登录页。

### TC-AUTH-03-14 同一秒内：下线前签发的作废、下线后签发的可用（iatMs）
- 关联 AC：AC-AUTH-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式；可解码 token 载荷。
- 步骤：
  1. 在同一秒内依次执行：为 alice 签发 Ta → 调用 `POST /admin/users/alice/force-logout` → 签发 Tb（若跨秒则重试，最多 20 次）。
  2. 解码 Ta、Tb 的载荷。
  3. 分别用 Ta、Tb 以 platform=web 连接。
- 预期结果：
  1. 载荷含整数字段 `iatMs`（13 位毫秒时间戳），且 `iatMs(Ta)` < `iatMs(Tb)`，两者同属一秒（C-11）。
  2. Ta：`kicked`/`banned` + 4012。
  3. Tb：连接成功，`ping` 收到 `pong`。

### TC-AUTH-03-15 强制下线不存在的用户返回 404/4042
- 关联 AC：AC-AUTH-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`ghost-user` 从未注册；bob 在线。
- 步骤：
  1. `POST /admin/users/ghost-user/force-logout`。
  2. `POST /admin/users/bad%20id/force-logout`（userId 含空格）。
- 预期结果：
  1. 步骤 1：HTTP 404，body.code = 4042（C-12）；bob 不受影响。
  2. 步骤 2：HTTP 400，body.code = 4004。

### TC-AUTH-03-16 已被强制下线且已过期的 token 报 banned/4012
- 关联 AC：AC-AUTH-03-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：测试模式。
- 步骤：
  1. 为 alice 签发 `ttlSeconds`=1 的 token T。
  2. 强制下线 alice，等待 2 秒（T 已过期）。
  3. 用 T 以 platform=web 连接。
- 预期结果：
  1. 收到 `kicked`/`banned`（不是 `tokenExpired`），关闭码 4012（C-08：4012 先于 4011）。

### TC-AUTH-03-17 强制下线接口缺少或错误密钥返回 401/4013
- 关联 AC：AC-AUTH-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 platform=web 在线。
- 步骤：
  1. 不带 `X-Admin-Key` 调用 `POST /admin/users/alice/force-logout`。
  2. 带 `X-Admin-Key: wrong` 再调一次。
  3. alice 发 `ping`。
- 预期结果：
  1. 两次均 HTTP 401，body.code = 4013。
  2. alice 未收到 `kicked`，`ping` 收到 `pong`。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-AUTH-04-01 开启演示登录后输入用户名可登录并连接
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：服务端显式配置开启演示登录服务；H5 已部署；Playwright 记录网络。
- 步骤：
  1. 打开登录页，输入 `alice`，点击登录。
- 预期结果：
  1. `POST /demo/login` 返回 HTTP 200，含 token。
  2. 随后发起 `/ws?token=...&platform=web`，未收到 `kicked`，连接保持。
  3. 页面进入会话列表，顶部无"连接中…"/"网络不可用"提示（连接后 3 秒检查）。
  4. `sessionStorage` 中保存了 token。

### TC-AUTH-04-02 演示登录接口返回的 token 可连接（接口层）
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端以开启演示登录的配置启动。
- 步骤：
  1. `POST /demo/login`，body `{"username":"alice"}`（不带任何管理密钥头）。
  2. 用返回的 token 以 platform=web 连接，发 `ping`。
- 预期结果：
  1. HTTP 200，返回 token（及 expiresAt）。
  2. 连接成功，收到 `pong`。

### TC-AUTH-04-03 演示登录即自动注册
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：演示登录开启；`newbie01` 从未注册；bob 已连接。
- 步骤：
  1. bob 给 `newbie01` 发单聊。
  2. 演示登录 `{"username":"newbie01"}`。
  3. bob 再给 `newbie01` 发单聊。
- 预期结果：
  1. 步骤 1：4042。
  2. 步骤 2：HTTP 200。
  3. 步骤 3：send 成功，`convId`=`c2c:bob:newbie01`。

### TC-AUTH-04-04 前端构建产物中搜不到管理密钥
- 关联 AC：AC-AUTH-04-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：服务端管理密钥配置为唯一可识别的值 `qa-admin-key-7f3c9e2b`（构建环境变量中也存在该值）。
- 步骤：
  1. 执行 H5 生产构建。
  2. 在构建产物目录（含 `.js`、`.map`、`.html`、`.json`）中搜索 `qa-admin-key-7f3c9e2b`，以及其 Base64 编码。
- 预期结果：
  1. 两种形式的匹配数都为 0。

### TC-AUTH-04-05 登录过程 HAR 中搜不到管理密钥
- 关联 AC：AC-AUTH-04-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同 TC-AUTH-04-04；演示登录开启；Playwright 开启 `recordHar`（含 WebSocket 帧）。
- 步骤：
  1. 打开登录页，以 `alice` 登录，进入会话列表后等待 3 秒。
  2. 保存 HAR，搜索 `qa-admin-key-7f3c9e2b` 和 `X-Admin-Key`。
- 预期结果：
  1. 匹配数都为 0（请求头、响应体、WS 帧中都没有）。

### TC-AUTH-04-06 默认配置下演示登录接口返回 404
- 关联 AC：AC-AUTH-04-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端以默认配置启动（不设置开启演示登录的配置项）。
- 步骤：
  1. `POST /demo/login`，body `{"username":"alice"}`。
  2. 用 GET 方法访问 `/demo/login`。
- 预期结果：
  1. 两次均 HTTP 404。
  2. 响应体不含 `token`；alice 未被自动注册（之后 bob 给 alice 发消息返回 4042，前提是 alice 此前未注册）。

### TC-AUTH-04-07 用户名含空格时提示"账号格式不正确"
- 关联 AC：AC-AUTH-04-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：演示登录开启；打开登录页。
- 步骤：
  1. 输入 `ali ce`，点击登录。
- 预期结果：
  1. 页面提示"账号格式不正确"。
  2. 停留在登录页，不发起 `/ws` 连接，`sessionStorage` 中无 token。

### TC-AUTH-04-08 其他非法用户名均提示"账号格式不正确"
- 关联 AC：AC-AUTH-04-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同上。
- 步骤：
  1. 依次输入 `张三`、`a.b`、`a@b`、65 个 `a`，各点击一次登录。
  2. 对照：输入 64 个 `a` 登录。
- 预期结果：
  1. 步骤 1 每次都提示"账号格式不正确"，停留登录页。
  2. 步骤 2 登录成功进入会话列表。

### TC-AUTH-04-09 演示登录用户名去首尾空白、空白返回 4004（接口层）
- 关联 AC：AC-AUTH-04-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：演示登录开启；bob 已连接。
- 步骤：
  1. `POST /demo/login`，body `{"username":"  carol\t"}`；用返回的 token 连接。
  2. `POST /demo/login`，body 分别为 `{"username":""}`、`{"username":"   "}`、`{"username":"a b"}`。
- 预期结果：
  1. 步骤 1：HTTP 200；连接后身份为 `carol`（bob 给 `carol` 发消息，该连接收到，`convId`=`c2c:bob:carol`）（C-21）。
  2. 步骤 2：三次均 HTTP 400 / 4004。

### TC-AUTH-04-10 登录页输入首尾带空格的用户名可登录、纯空白提示格式不正确
- 关联 AC：AC-AUTH-04-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：演示登录开启；打开登录页。
- 步骤：
  1. 输入 `   `（3 个空格），点击登录。
  2. 输入 ` alice `，点击登录。
- 预期结果：
  1. 步骤 1：提示"账号格式不正确"，停留登录页。
  2. 步骤 2：登录成功进入会话列表，"我"页显示当前账号 `alice`（无空格）。
- 变更：v1.3（V-UI-01）

### TC-AUTH-04-11 演示登录 token 有效期 1 天，测试模式可传 ttlSeconds
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：演示登录开启。
- 步骤：
  1. 非测试模式：记录 T，`POST /demo/login` `{"username":"alice"}`。
  2. 测试模式：`POST /demo/login` `{"username":"alice","ttlSeconds":2}`，立即连接，等待 3 秒。
- 预期结果：
  1. `expiresAt` − T 在 86400000 ± 5000 毫秒内（C-21）。
  2. 3 秒内收到 `kicked`/`tokenExpired`，关闭码 4011。

### TC-AUTH-05-01 退出登录回到登录页、刷新仍停留、服务端记录 1000
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 登录进入会话列表；可读取服务端日志。
- 步骤：
  1. 进入"我" → 设置 → 退出登录，在确认框中点"取消"。
  2. 再次点"退出登录"，在确认框中点"退出"。
  3. 检查 `sessionStorage`。
  4. 刷新页面。
  5. 查看服务端日志中 alice 该连接的断开记录。
- 预期结果：
  1. 步骤 1：确认框关闭，仍停留在设置页；WS 连接不断开（服务端日志无该连接的断开记录），token 仍在 `sessionStorage`。
  2. 步骤 2：跳转登录页。
  3. `sessionStorage` 中不再有 token。
  4. 刷新后仍停留登录页，不发起 `/ws` 连接。
  5. 服务端日志记录该连接以关闭码 1000 断开（只有一条断开记录）。
- 变更：v1.3（V-UI-01）

### TC-AUTH-05-02 主动关闭（1000）后 SDK 不重连
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；已连接。
- 步骤：
  1. 调用 SDK 的退出 / 断开接口。
  2. 假时钟推进 120 秒；期间触发一次浏览器 `online` 事件。
- 预期结果：
  1. 假传输层收到客户端发出的关闭码 1000。
  2. 推进期间连接尝试次数为 0（`online` 事件也不触发重连）。

### TC-AUTH-05-03 退出后清空内存数据，换账号登录看不到上个账号的数据
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 与 bob 有单聊（含消息 "secret-a"）；`zoe` 没有任何会话。
- 步骤：
  1. alice 在标签页中退出登录（"我" → 设置 → 退出登录 → 确认框点"退出"）。
  2. 同一标签页以 `zoe` 登录。
- 预期结果：
  1. zoe 的会话列表显示空状态，不出现 `c2c:alice:bob` 会话或 "secret-a"。
  2. 直接访问 `/chat/c2c:alice:bob` 不显示 "secret-a"。
- 变更：v1.3（V-UI-01）

### TC-AUTH-05-04 一个标签页退出不影响另一个标签页
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同一浏览器上下文开两个标签页：标签页 1 登录 alice，标签页 2 登录 bob。
- 步骤：
  1. 标签页 1 退出登录（"我" → 设置 → 退出登录 → 确认框点"退出"）。
  2. 在标签页 2 刷新页面；随后 carol 给 bob 发 "after-logout"。
- 预期结果：
  1. 标签页 2 仍为 bob 的登录态，刷新后自动重连停留原页面。
  2. 标签页 2 收到 "after-logout"。
- 变更：v1.3（V-UI-01）

---

## 覆盖检查

- AC-AUTH-01-1：TC-AUTH-01-01、TC-AUTH-01-02
- AC-AUTH-01-2：TC-AUTH-01-03、TC-AUTH-01-04、TC-AUTH-01-05
- AC-AUTH-01-3：TC-AUTH-01-06、TC-AUTH-01-07
- AC-AUTH-01-4：TC-AUTH-01-08、TC-AUTH-01-10、TC-AUTH-01-11
- AC-AUTH-01-5：TC-AUTH-01-09
- AC-AUTH-02-1：TC-AUTH-02-01、TC-AUTH-02-02
- AC-AUTH-02-2：TC-AUTH-02-03、TC-AUTH-02-04、TC-AUTH-02-05
- AC-AUTH-02-3：TC-AUTH-02-06
- AC-AUTH-02-4：TC-AUTH-02-07、TC-AUTH-02-08、TC-AUTH-02-09
- AC-AUTH-02-5：TC-AUTH-02-10、TC-AUTH-02-11
- AC-AUTH-02-6：TC-AUTH-02-12、TC-AUTH-02-13、TC-AUTH-02-14
- AC-AUTH-03-1：TC-AUTH-03-01、TC-AUTH-03-02
- AC-AUTH-03-2：TC-AUTH-03-03、TC-AUTH-03-04、TC-AUTH-03-05、TC-AUTH-03-15、TC-AUTH-03-17
- AC-AUTH-03-3：TC-AUTH-03-06、TC-AUTH-03-07、TC-AUTH-03-16
- AC-AUTH-03-4：TC-AUTH-03-08、TC-AUTH-03-09、TC-AUTH-03-14
- AC-AUTH-03-5：TC-AUTH-03-10、TC-AUTH-03-11
- AC-AUTH-03-6：TC-AUTH-03-12、TC-AUTH-03-13
- AC-AUTH-04-1：TC-AUTH-04-01、TC-AUTH-04-02、TC-AUTH-04-03、TC-AUTH-04-11
- AC-AUTH-04-2：TC-AUTH-04-04、TC-AUTH-04-05
- AC-AUTH-04-3：TC-AUTH-04-06
- AC-AUTH-04-4：TC-AUTH-04-07、TC-AUTH-04-08、TC-AUTH-04-09、TC-AUTH-04-10
- AC-AUTH-05-1：TC-AUTH-05-01、TC-AUTH-05-02、TC-AUTH-05-03、TC-AUTH-05-04

## 已澄清

1. 4009 前是否下发 kicked → C-05：不发，直接以 4009 关闭；修订 TC-AUTH-02-12，新增 TC-AUTH-02-13、TC-AUTH-02-14 也按此断言。
2. 多错误同时出现的优先级 → C-08：4009 → 4010 → 4012 → 4011；新增 TC-AUTH-02-14、TC-AUTH-03-16。
3. platform 大小写 → C-14：只接受小写；新增 TC-AUTH-02-13。
4. 强制下线 / 演示登录接口路径，强制下线不存在用户 → C-15：`POST /admin/users/{userId}/force-logout`、`POST /demo/login`；C-12：404/4042；修订相关步骤，新增 TC-AUTH-03-15、TC-AUTH-03-17。
5. 非测试模式 ttlSeconds 下限 → C-13：最小 60 秒；新增 TC-AUTH-01-10、TC-AUTH-01-11。
6. 被踢后失败消息在哪里看到 → C-01（PRD v1.1 F-AUTH-03 规则 4、AC-AUTH-02-4/03-5/03-6 改写）：停留当前页弹窗，点"确定"后回登录页；修订 TC-AUTH-02-08、02-09、03-10、03-11、03-13。
7. 强制下线判定精度 → C-11：token 带 `iatMs`，按毫秒比较；修订 TC-AUTH-03-09，新增 TC-AUTH-03-14。
8. 演示登录用户名去空白、空用户名 → C-21：先去首尾空白，空返回 4004；新增 TC-AUTH-04-09、TC-AUTH-04-10。
9. 演示登录 token 有效期、测试模式短有效期 → C-21：1 天，测试模式可传 `ttlSeconds`；修订 TC-AUTH-03-10，新增 TC-AUTH-04-11。另据 C-44，TC-AUTH-03-11 用 `?platform=pc` 模拟 pc 端。

## 疑问

1. `ttlSeconds` 传非整数或字符串（如 `"abc"`、`1.5`）时返回 4000，还是按默认 7 天？C-13 只定了下限，暂不设用例。
