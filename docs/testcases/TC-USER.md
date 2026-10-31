# 测试用例：用户（USER）

> 依据：[PRD v1.0](../prd/PRD-core-v1.0.md) F-USER-01、F-USER-02，§3.1 通用约定。
> 导入用户接口在 PRD §8 中列为新增管理接口，路径尚未落到 protocol/。下文记为"导入用户接口"，示例路径 `POST /admin/users/import`，body `{"users":[{"userId":...,"nickname":...,"avatar":...}]}`，以 BE 实现为准。管理密钥通过 `X-Admin-Key` 头传递，测试值 `test-admin-key`。

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
| TC-USER-02-01 | 有昵称的用户在会话列表和聊天页显示昵称 | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-02 | 无昵称的用户显示 userId | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-03 | 群消息发送者显示昵称或 userId | AC-USER-02-1 | UI 自动化 | P1 |
| TC-USER-02-04 | 头像为空时显示首字母占位 | AC-USER-02-1 | UI 自动化 | P1 |

---

### TC-USER-01-01 导入 3 个合法用户后单聊不再报 4042
- 关联 AC：AC-USER-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端测试模式，独立数据库；alice 已签发 token 并以 platform=web 连接；`u-imp-1`、`u-imp-2`、`u-imp-3` 从未注册。
- 步骤：
  1. alice 给 `u-imp-2` 发单聊（`clientMsgId`=`cid-u-0001`，文本 "before"）。
  2. 调用导入用户接口，users = `[{"userId":"u-imp-1","nickname":"一号"},{"userId":"u-imp-2","nickname":"二号","avatar":"https://example.com/a2.png"},{"userId":"u-imp-3"}]`。
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
  2. alice 发 `convs`，读取 `c2c:alice:bob` 的 `name`（或通过 BE 提供的用户查询途径读取 bob 的昵称）。
- 预期结果：
  1. 步骤 1：HTTP 200，不报错（不返回 4090 之类冲突码）。
  2. 步骤 2：bob 的昵称为"红红"，不再是"小红"。
  3. 用户表中 bob 只有 1 条记录。

### TC-USER-01-04 昵称 32 字（中文码点）、头像 512 字符边界可导入
- 关联 AC：AC-USER-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端已启动。
- 步骤：
  1. 导入 `[{"userId":"u-edge","nickname":"<32 个汉字，如'测'×32>","avatar":"https://example.com/<补齐到总长 512 字符>"}]`。
  2. 读取 `u-edge` 的昵称和头像。
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
  3. 读取 `u-exist` 的昵称。
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
  2. 读取 `erin` 的昵称。
- 预期结果：
  1. 签发成功。
  2. 昵称仍为"艾琳"（签发凭证只在用户不存在时注册，不清空已有资料）。

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
  2. carol 的头像位置显示文字占位"C"或"c"（见疑问 3）。
  3. dave 显示图片头像，`src` 为 `https://example.com/d.png`。

---

## 覆盖检查

- AC-USER-01-1：TC-USER-01-01、TC-USER-01-02、TC-USER-01-09
- AC-USER-01-2：TC-USER-01-03、TC-USER-01-04
- AC-USER-01-3：TC-USER-01-05、TC-USER-01-06
- AC-USER-01-4：TC-USER-01-07、TC-USER-01-08
- AC-USER-01-5：TC-USER-01-10、TC-USER-01-11
- AC-USER-02-1：TC-USER-02-01、TC-USER-02-02、TC-USER-02-03、TC-USER-02-04

## 疑问

1. 导入用户接口的路径、请求/响应格式未定义（PRD §8 只说"新增"，protocol/ 尚未落实）；也没有"查询用户"接口，AC-USER-01-2 的"昵称被更新"靠什么读回？本文暂用 `convs` 的 `name` 字段，需 BE 确认。
2. 昵称超过 32 字、为空串，或头像超过 512 字符、不是 URL 时返回什么错误码（4000？）？是否同样整批不写入？
3. "首字母占位"对中文昵称取第一个汉字，对英文 userId 是否转大写？有昵称时取昵称首字还是 userId 首字母？
4. 导入空数组（0 个用户）、同一批次内 userId 重复时的行为未定义（成功 / 4000 / 以后一条为准？）。
5. 昵称更新后，已在线的客户端是否需要实时刷新（无对应事件），还是刷新页面 / 重新拉 `convs` 后才更新？
6. `userId` 比较是否区分大小写（`Bob` 与 `bob` 是否同一用户）？
