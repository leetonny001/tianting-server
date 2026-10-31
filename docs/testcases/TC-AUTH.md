# 测试用例：账号与鉴权（AUTH）

> 依据：[PRD v1.0](../prd/PRD-core-v1.0.md) F-AUTH-01～F-AUTH-05、§5 文案表、§6.1 测试环境要求；[ERRORS.md](../../protocol/ERRORS.md) 第 4 节关闭码。
> 约定：管理密钥通过 `X-Admin-Key` 头传递，测试值 `test-admin-key`；连接地址 `ws://{host}/ws?token={token}&platform={platform}`；"测试模式"指 §6.1 的测试环境（token 最短 1 秒，独立数据库）。
> 强制下线接口、演示登录接口在 PRD §8 中为新增接口，路径未定，下文分别记为"强制下线接口"（示例 `POST /admin/users/{userId}/kick`）和"演示登录接口"（示例 `POST /demo/login`，body `{"username":...}`），以 BE 实现为准。

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
| TC-AUTH-02-01 | token 有效时连接成功并可收发消息 | AC-AUTH-02-1 | 后端 e2e | P0 |
| TC-AUTH-02-02 | 四种合法平台均可连接 | AC-AUTH-02-1 | 后端 e2e | P0 |
| TC-AUTH-02-03 | token 无效时先升级、下发 kicked/tokenInvalid、以 4010 关闭 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-04 | token 缺失时下发 kicked/tokenInvalid、以 4010 关闭 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-05 | 篡改签名的 token 被判无效 | AC-AUTH-02-2 | 后端 e2e | P0 |
| TC-AUTH-02-06 | token 已过期时下发 kicked/tokenExpired、以 4011 关闭 | AC-AUTH-02-3 | 后端 e2e | P0 |
| TC-AUTH-02-07 | SDK 收到 4010/4011 后不再重连 | AC-AUTH-02-4 | SDK 单测 | P0 |
| TC-AUTH-02-08 | 4010 时界面提示"登录已失效"并回到登录页 | AC-AUTH-02-4 | UI 自动化 | P0 |
| TC-AUTH-02-09 | 4011 时界面提示"登录已过期"并回到登录页 | AC-AUTH-02-4 | UI 自动化 | P0 |
| TC-AUTH-02-10 | 同一账号 web + pc 同时在线 | AC-AUTH-02-5 | 后端 e2e | P0 |
| TC-AUTH-02-11 | 同一账号同一平台 web 连两次都在线 | AC-AUTH-02-5 | 后端 e2e | P0 |
| TC-AUTH-02-12 | platform 为空、缺失或为 tv 时以 4009 关闭 | AC-AUTH-02-6 | 后端 e2e | P0 |
| TC-AUTH-03-01 | 在线期间 token 到期收到 kicked/tokenExpired、以 4011 关闭 | AC-AUTH-03-1 | 后端 e2e | P0 |
| TC-AUTH-03-02 | 到期前连接正常可用 | AC-AUTH-03-1 | 后端 e2e | P0 |
| TC-AUTH-03-03 | 强制下线后该用户所有连接收到 kicked/banned、以 4012 关闭 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-04 | 强制下线不影响其他用户 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-05 | kicked 帧完整发送后才关闭连接 | AC-AUTH-03-2 | 后端 e2e | P0 |
| TC-AUTH-03-06 | 强制下线后旧 token 连接收到 kicked/banned、以 4012 关闭 | AC-AUTH-03-3 | 后端 e2e | P0 |
| TC-AUTH-03-07 | 强制下线前签发但从未使用的 token 也作废 | AC-AUTH-03-3 | 后端 e2e | P0 |
| TC-AUTH-03-08 | 强制下线后重新签发的 token 可正常连接 | AC-AUTH-03-4 | 后端 e2e | P0 |
| TC-AUTH-03-09 | 强制下线后 1 秒内立即重签的 token 也可用 | AC-AUTH-03-4 | 后端 e2e | P0 |
| TC-AUTH-03-10 | 被踢（过期）后提示"登录已过期"、回登录页、不重连 | AC-AUTH-03-5 | UI 自动化 | P0 |
| TC-AUTH-03-11 | 被踢（强制下线）后提示"账号已被下线"、回登录页、不重连 | AC-AUTH-03-5 | UI 自动化 | P0 |
| TC-AUTH-03-12 | SDK 收到 kicked 后把发送中的消息标记失败 | AC-AUTH-03-6 | SDK 单测 | P0 |
| TC-AUTH-03-13 | 被踢时发送中的消息在界面显示失败 | AC-AUTH-03-6 | UI 自动化 | P0 |
| TC-AUTH-04-01 | 开启演示登录后输入用户名可登录并连接 | AC-AUTH-04-1 | UI 自动化 | P0 |
| TC-AUTH-04-02 | 演示登录接口返回的 token 可连接（接口层） | AC-AUTH-04-1 | 后端 e2e | P0 |
| TC-AUTH-04-03 | 演示登录即自动注册 | AC-AUTH-04-1 | 后端 e2e | P0 |
| TC-AUTH-04-04 | 前端构建产物中搜不到管理密钥 | AC-AUTH-04-2 | UI 自动化 | P0 |
| TC-AUTH-04-05 | 登录过程 HAR 中搜不到管理密钥 | AC-AUTH-04-2 | UI 自动化 | P0 |
| TC-AUTH-04-06 | 默认配置下演示登录接口返回 404 | AC-AUTH-04-3 | 后端 e2e | P0 |
| TC-AUTH-04-07 | 用户名含空格时提示"账号格式不正确" | AC-AUTH-04-4 | UI 自动化 | P0 |
| TC-AUTH-04-08 | 其他非法用户名均提示"账号格式不正确" | AC-AUTH-04-4 | UI 自动化 | P0 |
| TC-AUTH-05-01 | 退出登录回到登录页、刷新仍停留、服务端记录 1000 | AC-AUTH-05-1 | UI 自动化 | P0 |
| TC-AUTH-05-02 | 主动关闭（1000）后 SDK 不重连 | AC-AUTH-05-1 | SDK 单测 | P0 |
| TC-AUTH-05-03 | 退出后清空内存数据，换账号登录看不到上个账号的数据 | AC-AUTH-05-1 | UI 自动化 | P0 |
| TC-AUTH-05-04 | 一个标签页退出不影响另一个标签页 | AC-AUTH-05-1 | UI 自动化 | P0 |

---

### TC-AUTH-01-01 密钥正确、userId 合法时签发 token 和毫秒过期时间
- 关联 AC：AC-AUTH-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动，独立数据库。
- 步骤：
  1. 记录当前时间 T（毫秒）。
  2. `POST /admin/users/token`，头 `X-Admin-Key: test-admin-key`，body `{"userId":"alice"}`。
- 预期结果：
  1. HTTP 200，body 含非空字符串 `token` 和整数 `expiresAt`。
  2. `expiresAt` 是毫秒时间戳（13 位，且 > T）。

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

### TC-AUTH-01-05 密钥错误且 userId 非法时优先返回 4013
- 关联 AC：AC-AUTH-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. `X-Admin-Key: wrong`，body `{"userId":"bad id"}`。
- 预期结果：
  1. HTTP 401，body.code = 4013（先校验密钥，不泄露参数校验结果）。

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
- 前置条件：SDK 注入假传输层和假时钟。
- 步骤：
  1. SDK 连接；假传输层下发 `kicked`/`tokenInvalid` 后以 4010 关闭。
  2. 假时钟推进 120 秒（超过退避上限 30 秒的 4 倍）。
  3. 重新初始化场景，改为 `kicked`/`tokenExpired` + 4011，重复步骤 2。
- 预期结果：
  1. 两种情况下，推进期间假传输层的连接尝试次数为 0。
  2. SDK 对上层抛出"被踢"事件，原因分别为 tokenInvalid、tokenExpired；连接状态为未连接。

### TC-AUTH-02-08 4010 时界面提示"登录已失效"并回到登录页
- 关联 AC：AC-AUTH-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：开启演示登录；alice 登录 H5 进入会话列表；Playwright 记录 WebSocket。
- 步骤：
  1. 在 `sessionStorage` 中把 token 改为 `bad-token` 后刷新页面。
  2. 等待 60 秒，统计新建的 WebSocket 数量。
- 预期结果：
  1. 页面提示"登录已失效，请重新登录"。
  2. 跳转到登录页。
  3. 收到 4010 关闭后，60 秒内没有再发起任何 `/ws` 连接。

### TC-AUTH-02-09 4011 时界面提示"登录已过期"并回到登录页
- 关联 AC：AC-AUTH-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：测试模式；用管理接口为 alice 签发 `ttlSeconds`=1 的 token，等待 2 秒；H5 已打开。
- 步骤：
  1. 把该过期 token 写入 `sessionStorage` 后刷新页面。
  2. 等待 60 秒，统计 `/ws` 连接数。
- 预期结果：
  1. 页面提示"登录已过期，请重新登录"。
  2. 跳转到登录页。
  3. 60 秒内只出现 1 次 `/ws` 连接（即被 4011 关闭的那次）。

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
  2. 关闭前收到的帧不含 `msg`、`pong` 等业务帧（是否先收到 `kicked` 见疑问 1）。

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
  1. 调用强制下线接口，目标 `alice`。
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
  1. 强制下线接口返回 HTTP 200（无在线连接也成功）。
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
  1. 10 次 T4 都连接成功并收到 `pong`，无 `kicked`/`banned`（防止按秒精度的签发时间与下线时间比较导致误伤）。

### TC-AUTH-03-10 被踢（过期）后提示"登录已过期"、回登录页、不重连
- 关联 AC：AC-AUTH-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：测试模式；演示登录签发的 token 有效期配置为 5 秒（或 UI 注入 `ttlSeconds`=5 的 token）；alice 登录进入会话列表。
- 步骤：
  1. 等待 token 到期（约 5 秒）。
  2. 之后再等待 60 秒，统计 `/ws` 连接数。
- 预期结果：
  1. 页面提示"登录已过期，请重新登录"，跳转登录页。
  2. 到期后 60 秒内没有新的 `/ws` 连接。

### TC-AUTH-03-11 被踢（强制下线）后提示"账号已被下线"、回登录页、不重连
- 关联 AC：AC-AUTH-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在两个浏览器上下文（模拟 web、pc 两端）都已登录。
- 步骤：
  1. 通过管理接口强制下线 alice。
  2. 等待 60 秒，统计两个上下文的 `/ws` 连接数。
- 预期结果：
  1. 两个上下文都提示"账号已被下线"，并跳转登录页。
  2. 60 秒内两个上下文都没有新的 `/ws` 连接。
  3. 刷新页面后停留在登录页（旧 token 不会自动登录成功）。

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

### TC-AUTH-03-13 被踢时发送中的消息在界面显示失败
- 关联 AC：AC-AUTH-03-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：服务端测试模式开启故障注入；alice、bob 两个上下文登录，alice 打开与 bob 的聊天页。
- 步骤：
  1. 配置故障注入：丢弃 alice 下一条消息的 ack（按 `clientMsgId`，可通过拦截 WS 获取）。
  2. alice 发送 "pending-msg"，在显示"发送中"时调用强制下线接口。
  3. 在跳转登录页前截取聊天页状态（或重新登录后检查同一标签页内存状态，见疑问 6）。
- 预期结果：
  1. "pending-msg" 显示为发送失败。
  2. 页面提示"账号已被下线"并回到登录页。

### TC-AUTH-04-01 开启演示登录后输入用户名可登录并连接
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：服务端显式配置开启演示登录服务；H5 已部署；Playwright 记录网络。
- 步骤：
  1. 打开登录页，输入 `alice`，点击登录。
- 预期结果：
  1. 演示登录接口返回 HTTP 200，含 token。
  2. 随后发起 `/ws?token=...&platform=web`，未收到 `kicked`，连接保持。
  3. 页面进入会话列表，顶部无"连接中…"/"网络不可用"提示（连接后 3 秒检查）。
  4. `sessionStorage` 中保存了 token。

### TC-AUTH-04-02 演示登录接口返回的 token 可连接（接口层）
- 关联 AC：AC-AUTH-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端以开启演示登录的配置启动。
- 步骤：
  1. 调用演示登录接口 body `{"username":"alice"}`（不带任何管理密钥头）。
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
  1. 调用演示登录接口 body `{"username":"alice"}`。
  2. 用 GET 方法访问同一路径。
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

### TC-AUTH-05-01 退出登录回到登录页、刷新仍停留、服务端记录 1000
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 登录进入会话列表；可读取服务端日志。
- 步骤：
  1. 点击右上角菜单 → 退出登录。
  2. 检查 `sessionStorage`。
  3. 刷新页面。
  4. 查看服务端日志中 alice 该连接的断开记录。
- 预期结果：
  1. 跳转登录页。
  2. `sessionStorage` 中不再有 token。
  3. 刷新后仍停留登录页，不发起 `/ws` 连接。
  4. 服务端日志记录该连接以关闭码 1000 断开。

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
  1. alice 在标签页中退出登录。
  2. 同一标签页以 `zoe` 登录。
- 预期结果：
  1. zoe 的会话列表显示空状态，不出现 `c2c:alice:bob` 会话或 "secret-a"。
  2. 直接访问 `/chat/c2c:alice:bob` 不显示 "secret-a"。

### TC-AUTH-05-04 一个标签页退出不影响另一个标签页
- 关联 AC：AC-AUTH-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同一浏览器上下文开两个标签页：标签页 1 登录 alice，标签页 2 登录 bob。
- 步骤：
  1. 标签页 1 退出登录。
  2. 在标签页 2 刷新页面；随后 carol 给 bob 发 "after-logout"。
- 预期结果：
  1. 标签页 2 仍为 bob 的登录态，刷新后自动重连停留原页面。
  2. 标签页 2 收到 "after-logout"。

---

## 覆盖检查

- AC-AUTH-01-1：TC-AUTH-01-01、TC-AUTH-01-02
- AC-AUTH-01-2：TC-AUTH-01-03、TC-AUTH-01-04、TC-AUTH-01-05
- AC-AUTH-01-3：TC-AUTH-01-06、TC-AUTH-01-07
- AC-AUTH-01-4：TC-AUTH-01-08
- AC-AUTH-01-5：TC-AUTH-01-09
- AC-AUTH-02-1：TC-AUTH-02-01、TC-AUTH-02-02
- AC-AUTH-02-2：TC-AUTH-02-03、TC-AUTH-02-04、TC-AUTH-02-05
- AC-AUTH-02-3：TC-AUTH-02-06
- AC-AUTH-02-4：TC-AUTH-02-07、TC-AUTH-02-08、TC-AUTH-02-09
- AC-AUTH-02-5：TC-AUTH-02-10、TC-AUTH-02-11
- AC-AUTH-02-6：TC-AUTH-02-12
- AC-AUTH-03-1：TC-AUTH-03-01、TC-AUTH-03-02
- AC-AUTH-03-2：TC-AUTH-03-03、TC-AUTH-03-04、TC-AUTH-03-05
- AC-AUTH-03-3：TC-AUTH-03-06、TC-AUTH-03-07
- AC-AUTH-03-4：TC-AUTH-03-08、TC-AUTH-03-09
- AC-AUTH-03-5：TC-AUTH-03-10、TC-AUTH-03-11
- AC-AUTH-03-6：TC-AUTH-03-12、TC-AUTH-03-13
- AC-AUTH-04-1：TC-AUTH-04-01、TC-AUTH-04-02、TC-AUTH-04-03
- AC-AUTH-04-2：TC-AUTH-04-04、TC-AUTH-04-05
- AC-AUTH-04-3：TC-AUTH-04-06
- AC-AUTH-04-4：TC-AUTH-04-07、TC-AUTH-04-08
- AC-AUTH-05-1：TC-AUTH-05-01、TC-AUTH-05-02、TC-AUTH-05-03、TC-AUTH-05-04

## 疑问

1. 平台不合法（4009）：F-SYS-01 规则 2 把它列为"连接级致命错误，先下发 kicked"，但 F-AUTH-02 规则 4 只说"用 4009 关闭"，`kickReasons` 也没有对应原因。4009 前是否下发 kicked？reason 是什么？
2. token 无效和 platform 不合法同时出现时，用 4009 还是 4010？token 既被强制下线作废、又已过期时，返回 banned/4012 还是 tokenExpired/4011？
3. platform 是否区分大小写（`Web`、`PC` 是否合法）？
4. 强制下线接口、演示登录接口的路径与响应未定义；对不存在的 userId 强制下线返回什么（200 还是 404/4042）？
5. 非测试模式下 `ttlSeconds` 传 1～59 等很小的正数是否有下限？AC-AUTH-01-4/5 只覆盖 ≤0 和 >30 天。`ttlSeconds` 传非整数 / 字符串时返回 4000 还是按默认 7 天？
6. AC-AUTH-03-6 要求"发送中的消息显示为发送失败"，但 F-AUTH-03 规则 4 同时要求回到登录页，且 F-AUTH-05 规则说清空内存数据。被踢回登录页后用户还能在哪里看到这条失败消息？是否只要求 SDK 状态正确即可？
7. "强制下线前签发的 token 全部失效"的判定精度：若签发时间按秒记录，下线后同一秒内重签的 token 可能被误判失效（TC-AUTH-03-09 按"必须可用"验证），需 BE 确认实现方式。
8. 演示登录的用户名是否先去首尾空白再校验（§3.1 的去空白规则只说文本）？输入 ` alice ` 是登录成功还是提示格式不正确？空用户名是前端禁用按钮还是提示"账号格式不正确"？
9. 演示登录开启时签发的 token 有效期是多少（沿用 7 天默认？），测试模式下能否配置成短有效期（TC-AUTH-03-10 依赖此能力）？
