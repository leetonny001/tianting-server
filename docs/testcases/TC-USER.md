# 测试用例：用户（USER）

> 依据：[PRD-core v1.3](../prd/PRD-core-v1.3.md) F-USER-01、F-USER-02，§3.1 通用约定，§9 C-15～C-20。
> 接口路径（C-15）：导入用户 `POST /admin/users/import`，body `{"users":[{"userId":...,"nickname":...,"avatar":...}]}`；查询用户 `GET /admin/users/{userId}`，用于核对导入结果（返回 `userId`、`nickname`、`avatar`）。管理密钥通过 `X-Admin-Key` 头传递，测试值 `test-admin-key`。
> 管理接口（PRD-core v1.3 F-AUTH-01 规则 3、C-15；VC-01）：`/admin/*`（含测试接口 `/admin/test/*`）只在内网管理地址 `IM_ADMIN_ADDR` 上提供，下文所有管理接口请求都发到该地址；e2e 继续用旧密钥 `X-Admin-Key: test-admin-key`（过渡调用方 `legacy`，非生产环境，不带 `X-IM-Caller`）；`/ws`、`/demo/login` 仍在对外端口。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-USER-01-01 | 导入 3 个合法用户后单聊不再报 4042 | AC-USER-01-1 | 后端 e2e | P0 |
| TC-USER-01-02 | 只带 userId（无昵称、头像）的用户也能导入 | AC-USER-01-1 | 后端 e2e | P0 |
| TC-USER-01-03 | 重复导入同一 userId 更新昵称（upsert） | AC-USER-01-2 | 后端 e2e | P0 |
| TC-USER-01-04 | 昵称 32 字（中文码点）、头像 512 字符边界可导入 | AC-USER-01-2 | 后端 e2e | P0 |
| TC-USER-01-05 | 一次导入 500 个用户成功 | AC-USER-01-3 | 后端 e2e | P0 |
| TC-USER-01-06 | 一次导入 501 个用户返回 400/4131 且不写入 | AC-USER-01-3 | 后端 e2e | P0 |
| TC-USER-01-07 | 批次中含非法 userId 返回 400/4004 且整批不写入 | AC-USER-01-4 | 后端 e2e | P0 |
| TC-USER-01-08 | 各类非法 userId 逐一校验 | AC-USER-01-4 | 后端 e2e | P0 |
| TC-USER-01-09 | 导入接口缺少或错误密钥返回 401/4013 | AC-USER-01-1 | 后端 e2e | P0 |
| TC-USER-01-10 | 签发凭证自动注册用户 | AC-USER-01-5 | 后端 e2e | P0 |
| TC-USER-01-11 | 自动注册不覆盖已导入的昵称 | AC-USER-01-5 | 后端 e2e | P0 |
| TC-USER-01-12 | 昵称 33 字返回 400/4000 且整批不写入 | AC-USER-01-4 | 后端 e2e | P0 |
| TC-USER-01-13 | 头像 513 字符返回 400/4000 且整批不写入 | AC-USER-01-4 | 后端 e2e | P0 |
| TC-USER-01-14 | 导入空数组返回 400/4000 | AC-USER-01-3 | 后端 e2e | P0 |
| TC-USER-01-15 | 同一批内 userId 重复以最后一条为准 | AC-USER-01-2 | 后端 e2e | P0 |
| TC-USER-01-16 | userId 区分大小写 | AC-USER-01-1 | 后端 e2e | P0 |
| TC-USER-01-17 | 查询用户接口返回导入结果 | AC-USER-01-1 | 后端 e2e | P0 |
| TC-USER-02-01 | 有昵称的用户在会话列表和聊天页显示昵称 | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-02 | 无昵称的用户显示 userId | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-03 | 群消息发送者显示昵称或 userId | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-04 | 头像为空时显示首字母占位 | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-05 | 昵称修改后下次拉 convs 时生效 | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-06 | 占位取第一个码点、背景色下标 = FNV-1a-32(userId) mod 8 | AC-USER-02-1 | UI 自动化 | P1 |

---

### TC-USER-01-01 导入 3 个合法用户后单聊不再报 4042
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端测试模式，独立数据库；alice 已签发 token 并以 platform=web 连接；`u-imp-1`、`u-imp-2`、`u-imp-3` 从未注册。
- 步骤：
  1. alice 给 `u-imp-2` 发单聊（`clientMsgId`=`cid-u-0001`，文本 "before"）。
  2. 调用 `POST /admin/users/import`，users = `[{"userId":"u-imp-1","nickname":"一号"},{"userId":"u-imp-2","nickname":"二号","avatar":"https://example.com/a2.png"},{"userId":"u-imp-3"}]`。
  3. alice 分别给 `u-imp-1`、`u-imp-2`、`u-imp-3` 发单聊（`clientMsgId` 依次为 `cid-u-0002`～`cid-u-0004`）。
- 预期结果：
  1. 步骤 1：ack.error.code = 4042。
  2. 步骤 2：HTTP 200，响应体无 `code` 错误字段。
  3. 步骤 3：3 次 send 都返回成功 ack（含 `msgId`、`seq`=1、`convId`，如 `c2c:alice:u-imp-1`）。

### TC-USER-01-02 只带 userId（无昵称、头像）的用户也能导入
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-USER-01-01；`u-min` 从未注册。
- 步骤：
  1. 导入 `[{"userId":"u-min"}]`。
  2. alice 给 `u-min` 发单聊。
- 预期结果：
  1. 导入返回 HTTP 200。
  2. send 成功（无 4042）。

### TC-USER-01-03 重复导入同一 userId 更新昵称（upsert）
- 关联 AC：AC-USER-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：已导入 `[{"userId":"bob","nickname":"小红"}]`；alice 与 bob 已有一条单聊消息。
- 步骤：
  1. 再次导入 `[{"userId":"bob","nickname":"红红"}]`。
  2. `GET /admin/users/bob`。
  3. alice 重新发 `convs`，读取 `c2c:alice:bob` 的 `name`。
- 预期结果：
  1. 步骤 1：HTTP 200，不报错（不返回 4090 之类冲突码）。
  2. 步骤 2：HTTP 200，`nickname`="红红"。
  3. 步骤 3：`name` 为"红红"（C-19：下次拉 `convs` 生效）。

### TC-USER-01-04 昵称 32 字（中文码点）、头像 512 字符边界可导入
- 关联 AC：AC-USER-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 导入 `[{"userId":"u-edge","nickname":"<32 个汉字，如'测'×32>","avatar":"https://example.com/<补齐到总长 512 字符>"}]`。
  2. `GET /admin/users/u-edge`。
- 预期结果：
  1. HTTP 200。
  2. 昵称为完整 32 个汉字（按码点计数，未被截断），头像 URL 为完整 512 字符。

### TC-USER-01-05 一次导入 500 个用户成功
- 关联 AC：AC-USER-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：独立数据库；`u-b-001`～`u-b-500` 从未注册。
- 步骤：
  1. 一次导入 500 个用户 `u-b-001`～`u-b-500`。
  2. alice 给 `u-b-001` 和 `u-b-500` 各发一条单聊。
- 预期结果：
  1. HTTP 200。
  2. 两条 send 均成功（无 4042）。

### TC-USER-01-06 一次导入 501 个用户返回 400/4131 且不写入
- 关联 AC：AC-USER-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：独立数据库；`u-c-001`～`u-c-501` 从未注册。
- 步骤：
  1. 一次导入 501 个用户 `u-c-001`～`u-c-501`。
  2. alice 给 `u-c-001` 发单聊。
- 预期结果：
  1. HTTP 400，body.code = 4131。
  2. 步骤 2 返回 4042（整批未写入）。

### TC-USER-01-07 批次中含非法 userId 返回 400/4004 且整批不写入
- 关联 AC：AC-USER-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`u-ok-1`、`u-ok-2` 从未注册；已导入 `u-exist`（昵称"旧名"）。
- 步骤：
  1. 导入 `[{"userId":"u-ok-1"},{"userId":"bad id"},{"userId":"u-ok-2"},{"userId":"u-exist","nickname":"新名"}]`。
  2. alice 给 `u-ok-1`、`u-ok-2` 各发单聊。
  3. `GET /admin/users/u-exist`。
- 预期结果：
  1. HTTP 400，body.code = 4004。
  2. 两条 send 均返回 4042。
  3. `u-exist` 的昵称仍为"旧名"（整批回滚，包括 upsert 部分）。

### TC-USER-01-08 各类非法 userId 逐一校验
- 关联 AC：AC-USER-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 依次单独导入以下 userId（每次批次只含 1 个）：`""`（空串）、65 个字符 `a`×65、`a b`、`张三`、`a.b`、`a@b`。
  2. 对照：导入 `a`（长度 1）、`a`×64、`A_z-09`。
- 预期结果：
  1. 步骤 1 每次都返回 HTTP 400 / 4004。
  2. 步骤 2 每次都返回 HTTP 200。

### TC-USER-01-09 导入接口缺少或错误密钥返回 401/4013
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`u-key-1` 从未注册。
- 步骤：
  1. 不带 `X-Admin-Key` 导入 `[{"userId":"u-key-1"}]`。
  2. 带 `X-Admin-Key: wrong` 导入同样内容。
  3. alice 给 `u-key-1` 发单聊。
- 预期结果：
  1. 步骤 1、2：HTTP 401，body.code = 4013。
  2. 步骤 3：返回 4042（未写入）。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-USER-01-10 签发凭证自动注册用户
- 关联 AC：AC-USER-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 已连接；`dave` 从未导入、从未签发过 token。
- 步骤：
  1. alice 给 `dave` 发单聊（`clientMsgId`=`cid-u-0101`）。
  2. 调用 `POST /admin/users/token`，body `{"userId":"dave"}`（不连接）。
  3. alice 再给 `dave` 发单聊（`clientMsgId`=`cid-u-0102`）。
- 预期结果：
  1. 步骤 1：4042。
  2. 步骤 2：HTTP 200，返回 token 和 expiresAt。
  3. 步骤 3：send 成功，`convId`=`c2c:alice:dave`，`seq`=1。

### TC-USER-01-11 自动注册不覆盖已导入的昵称
- 关联 AC：AC-USER-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：已导入 `[{"userId":"erin","nickname":"艾琳"}]`。
- 步骤：
  1. 为 `erin` 签发 token。
  2. `GET /admin/users/erin`。
- 预期结果：
  1. 签发成功。
  2. 昵称仍为"艾琳"（签发凭证只在用户不存在时注册，不清空已有资料）。

### TC-USER-01-12 昵称 33 字返回 400/4000 且整批不写入
- 关联 AC：AC-USER-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`u-n-1`、`u-n-2` 从未注册。
- 步骤：
  1. 导入 `[{"userId":"u-n-1","nickname":"<'测'×32>"},{"userId":"u-n-2","nickname":"<'测'×33>"}]`。
  2. `GET /admin/users/u-n-1`。
- 预期结果：
  1. HTTP 400，body.code = 4000（C-16）。
  2. 步骤 2：HTTP 404 / 4042（`u-n-1` 也未写入）。

### TC-USER-01-13 头像 513 字符返回 400/4000 且整批不写入
- 关联 AC：AC-USER-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`u-v-1`、`u-v-2` 从未注册。
- 步骤：
  1. 导入 `[{"userId":"u-v-1"},{"userId":"u-v-2","avatar":"https://example.com/<补齐到 513 字符>"}]`。
  2. `GET /admin/users/u-v-1`。
- 预期结果：
  1. HTTP 400，body.code = 4000（C-16）。
  2. 步骤 2：HTTP 404 / 4042。

### TC-USER-01-14 导入空数组返回 400/4000
- 关联 AC：AC-USER-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 导入 `{"users":[]}`。
- 预期结果：
  1. HTTP 400，body.code = 4000（C-17）。

### TC-USER-01-15 同一批内 userId 重复以最后一条为准
- 关联 AC：AC-USER-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`u-dup` 从未注册。
- 步骤：
  1. 导入 `[{"userId":"u-dup","nickname":"第一"},{"userId":"u-dup","nickname":"第二"}]`。
  2. `GET /admin/users/u-dup`。
- 预期结果：
  1. HTTP 200。
  2. `nickname`="第二"（C-17）。

### TC-USER-01-16 userId 区分大小写
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：已导入 `[{"userId":"alice","nickname":"小明"}]`；`Alice` 从未注册。
- 步骤：
  1. `GET /admin/users/Alice`。
  2. 导入 `[{"userId":"Alice","nickname":"大写"}]`。
  3. `GET /admin/users/alice`、`GET /admin/users/Alice`。
- 预期结果：
  1. 步骤 1：HTTP 404 / 4042。
  2. 步骤 2：HTTP 200。
  3. alice 的 `nickname` 仍为"小明"，Alice 为"大写"（两个不同用户，C-18）。

### TC-USER-01-17 查询用户接口返回导入结果
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：已导入 `[{"userId":"u-q","nickname":"查询","avatar":"https://example.com/q.png"}]`。
- 步骤：
  1. `GET /admin/users/u-q`，带正确密钥。
  2. 同一请求不带 `X-Admin-Key`。
- 预期结果：
  1. HTTP 200，`userId`="u-q"、`nickname`="查询"、`avatar`="https://example.com/q.png"。
  2. HTTP 401 / 4013。
- 说明：验证旧密钥（`legacy`）路径——请求发到管理地址 `IM_ADMIN_ADDR`、不带 `X-IM-Caller`，非生产环境；签名路径（带 `X-IM-Caller`）的鉴权失败 4017 由 PRD-admin AC-KEY-03-* 覆盖，不在本用例。
- 变更：v1.3（VC-01）

### TC-USER-02-01 有昵称的用户在会话列表和聊天页显示昵称
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：开启演示登录；已导入 `[{"userId":"bob","nickname":"小红"}]`；两个浏览器上下文分别以 alice、bob 登录。
- 步骤：
  1. bob 给 alice 发 "你好"。
  2. alice 查看会话列表。
  3. alice 点开该会话进入聊天页。
- 预期结果：
  1. 会话列表中该会话名称显示"小红"，不显示 `bob`。
  2. 聊天页顶栏会话名显示"小红"。

### TC-USER-02-02 无昵称的用户显示 userId
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：`carol` 只通过演示登录自动注册，没有昵称；alice、carol 两个上下文登录。
- 步骤：
  1. carol 给 alice 发 "hi"。
  2. alice 查看会话列表和聊天页。
- 预期结果：
  1. 会话列表名称显示 `carol`。
  2. 聊天页顶栏显示 `carol`；不出现空白、`undefined`、`null`。

### TC-USER-02-03 群消息发送者显示昵称或 userId
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：bob 昵称"小红"，carol 无昵称；管理接口建群 `g-nick`（成员 alice、bob、carol）；三个上下文登录。
- 步骤：
  1. bob 在群里发 "1"，carol 发 "2"。
  2. alice 打开群聊天页。
- 预期结果：
  1. 消息 "1" 的发送者显示"小红"。
  2. 消息 "2" 的发送者显示 `carol`。

### TC-USER-02-04 头像为空时显示首字母占位
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：bob 昵称"小红"且无头像；carol 无昵称无头像；`dave` 有头像 `https://example.com/d.png`；均与 alice 有单聊。
- 步骤：
  1. alice 查看会话列表中三个会话的头像位置。
- 预期结果：
  1. bob 的头像位置显示文字占位"小"。
  2. carol 的头像位置显示文字占位"C"（大写，C-20）。
  3. dave 显示图片头像，`src` 为 `https://example.com/d.png`。

### TC-USER-02-05 昵称修改后下次拉 convs 时生效
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：bob 昵称"小红"；alice 已登录，会话列表显示与 bob 的会话"小红"。
- 步骤：
  1. 管理接口导入 `[{"userId":"bob","nickname":"红红"}]`。
  2. alice 不刷新，等待 3 秒。
  3. alice 刷新页面（重新拉 `convs`）。
- 预期结果：
  1. 步骤 2：仍显示"小红"属于预期（一期不做实时推送，C-19），页面不报错。
  2. 步骤 3：会话列表和聊天页顶栏显示"红红"。

### TC-USER-02-06 占位取第一个码点、背景色下标 = FNV-1a-32(userId) mod 8
- 关联 AC：AC-USER-02-1
- 优先级：P1
- 层级：UI 自动化
- 前置条件：`u-emoji` 昵称"😀abc"、无头像；`dave2` 无昵称、无头像；两人都与 alice 有单聊。另按 `protocol/vectors/avatar.json` 的每条用例导入对应用户（`userId` + 昵称、无头像），并都与 alice 有单聊。
- 步骤：
  1. alice 查看会话列表，记录两人头像占位的文字和背景色（调色板下标）。
  2. 刷新页面后再次记录；在另一个浏览器上下文以 alice 登录再次记录。
  3. 对 `avatar.json` 的每条用例，读取会话列表中该用户头像占位的首字和背景色下标，与用例的期望值逐条比对。
- 预期结果：
  1. `u-emoji` 占位为"😀"（完整一个码点，不是半个代理对）；`dave2` 占位为"D"。
  2. 背景色为 8 色调色板的第 `FNV-1a-32(UTF-8(userId)) mod 8` 个（C-20）；三次记录中，同一 userId 的背景色下标完全相同。
  3. `avatar.json` 每条用例的首字和颜色下标都与期望值一致。
  4. 群头像用 `groupId` 计算，另见 PRD-ui F-UI-21，不在本用例。
- 变更：v1.3（V-UI-12）

---

## 覆盖检查

- AC-USER-01-1：TC-USER-01-01、TC-USER-01-02、TC-USER-01-09、TC-USER-01-16、TC-USER-01-17
- AC-USER-01-2：TC-USER-01-03、TC-USER-01-04、TC-USER-01-15
- AC-USER-01-3：TC-USER-01-05、TC-USER-01-06、TC-USER-01-14
- AC-USER-01-4：TC-USER-01-07、TC-USER-01-08、TC-USER-01-12、TC-USER-01-13
- AC-USER-01-5：TC-USER-01-10、TC-USER-01-11
- AC-USER-02-1：TC-USER-02-01、TC-USER-02-02、TC-USER-02-03、TC-USER-02-04、TC-USER-02-05、TC-USER-02-06

## 已澄清

1. 导入 / 查询用户接口路径，昵称如何读回 → C-15：`POST /admin/users/import`、`GET /admin/users/{userId}`；TC-USER-01-03/04/07/11 改为用查询接口核对，新增 TC-USER-01-17。
2. 昵称超长、头像超长的错误码 → C-16：4000，整批不写入；新增 TC-USER-01-12、TC-USER-01-13。
3. 首字母占位规则 → C-20：显示名第一个码点，英文转大写，背景色按 userId 哈希；修订 TC-USER-02-04，新增 TC-USER-02-06。
4. 导入空数组、同批 userId 重复 → C-17：空数组 4000，重复以最后一条为准；新增 TC-USER-01-14、TC-USER-01-15。
5. 昵称修改后在线客户端如何刷新 → C-19：下次拉 `convs` 生效；修订 TC-USER-01-03，新增 TC-USER-02-05。
6. userId 是否区分大小写 → C-18：区分；新增 TC-USER-01-16。

## 疑问

1. `GET /admin/users/{userId}` 查询不存在的用户时，C-15 没写返回值。TC-USER-01-12/13/16 按 HTTP 404 / 4042 断言（与 C-12 强制下线不存在用户一致），需 BE 确认。
