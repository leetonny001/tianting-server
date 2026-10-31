# 测试用例：统一错误码（SYS）

> 依据：[PRD v1.1](../prd/PRD-core-v1.1.md) F-SYS-01、§9 C-05/C-09/C-10、[ERRORS.md](../../protocol/ERRORS.md)、`protocol/errors.json`。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-SYS-01-01 | 服务端错误码常量与 errors.json 完全一致 | AC-SYS-01-1 | 后端 e2e | P0 |
| TC-SYS-01-02 | 服务端常量的名称与 HTTP 映射与 errors.json 一致 | AC-SYS-01-1 | 后端 e2e | P0 |
| TC-SYS-01-03 | 管理接口错误的 HTTP 状态码与 errors.json 的 http 字段一致 | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-04 | 管理接口错误响应体格式为 {code,msg} | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-05 | WS 请求错误通过 ack.error 返回，不带 data | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-06 | 连接级致命错误先下发 kicked、再用同号关闭码关闭 | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-07 | 第 4 节带错误码 AC 的错误码回归汇总 | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-08 | TS SDK 错误枚举与 errors.json 一致 | AC-SYS-01-3 | SDK 单测 | P0 |
| TC-SYS-01-09 | 未定义错误码 4999 按 4xx 区段兜底 | AC-SYS-01-4 | SDK 单测 | P0 |
| TC-SYS-01-10 | 未定义错误码 4999 在界面提示"请求出错" | AC-SYS-01-4 | UI 自动化 | P0 |
| TC-SYS-01-11 | 界面文案按 code 映射，不展示服务端 msg | AC-SYS-01-4 | UI 自动化 | P0 |
| TC-SYS-01-12 | 未定义码按前三位兜底：4039/4049/4299 | AC-SYS-01-4 | SDK 单测 | P0 |
| TC-SYS-01-13 | 未定义码 5999 视为可重试并提示服务异常 | AC-SYS-01-4 | SDK 单测 | P0 |
| TC-SYS-01-14 | 平台不合法不发 kicked，直接以 4009 关闭 | AC-SYS-01-2 | 后端 e2e | P0 |
| TC-SYS-01-15 | 未定义子区段码在界面上的兜底文案 | AC-SYS-01-4 | UI 自动化 | P0 |

---

### TC-SYS-01-01 服务端错误码常量与 errors.json 完全一致
- 关联 AC：AC-SYS-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：仓库内存在 `protocol/errors.json`；服务端以 Go 常量形式定义全部错误码（`server/internal/protocol`）。
- 步骤：
  1. 一致性测试读取 `protocol/errors.json` 的 `errors[].code`，得到集合 J。
  2. 通过反射或服务端导出的错误码列表得到集合 G。
  3. 比较 J 与 G。
- 预期结果：
  1. J 与 G 完全相等（无多、无少）；当前 J 共 28 个码：4000～4013、4030～4033、4040～4043、4090、4130、4131、4290、5000、5030。
  2. 在 errors.json 中临时加入一个码（如 4099）后测试失败，证明测试有效（变异验证，执行后还原）。

### TC-SYS-01-02 服务端常量的名称与 HTTP 映射与 errors.json 一致
- 关联 AC：AC-SYS-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-SYS-01-01。
- 步骤：
  1. 对 errors.json 中每个条目，取 `code`、`name`、`http`。
  2. 与服务端常量名、服务端内部 code→HTTP 映射表逐条比较。
- 预期结果：
  1. 每个 code 的名称一致（如 4013 ↔ ADMIN_KEY_INVALID，4131 ↔ BATCH_LIMIT_EXCEEDED）。
  2. `http` 非 null 的条目，服务端映射的 HTTP 状态码相同（如 4013→401、4042→404、4090→409、4130→400、4290→429、5030→503）；`http` 为 null 的码不会被用于管理接口。
  3. errors.json 的 `closeCodes`（1000、1001、1009、4009～4012、4400、4408）和 `kickReasons`（tokenInvalid→4010、tokenExpired→4011、banned→4012）与服务端常量一致。

### TC-SYS-01-03 管理接口错误的 HTTP 状态码与 errors.json 的 http 字段一致
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端测试模式启动，独立数据库；管理密钥为 `test-admin-key`，通过 `X-Admin-Key` 头传递。
- 步骤：
  1. 不带密钥调用 `POST /admin/users/token`，body `{"userId":"alice"}`。
  2. 带密钥调用 `POST /admin/users/token`，body `{"userId":"bad id"}`。
  3. `POST /admin/users/import` 一次导入 501 个用户。
  4. 建群 `g1` 两次（`POST /admin/groups`）。
  5. 向不存在的群 `g-none` 加成员（`POST /admin/groups/g-none/members`）。
  6. 向 `g1` 加成员 `["ghost-user"]`（从未注册）。
- 预期结果：
  1. 步骤 1：HTTP 401，body.code = 4013。
  2. 步骤 2：HTTP 400，body.code = 4004。
  3. 步骤 3：HTTP 400，body.code = 4131。
  4. 步骤 4 第二次：HTTP 409，body.code = 4090。
  5. 步骤 5：HTTP 404，body.code = 4043。
  6. 步骤 6：HTTP 404，body.code = 4042。
  7. 每一步的 HTTP 状态码都等于 errors.json 中该 code 的 `http` 字段（测试代码从 errors.json 查表断言，而不是写死）。

### TC-SYS-01-04 管理接口错误响应体格式为 {code,msg}
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-SYS-01-03。
- 步骤：
  1. 用错误密钥 `wrong` 调用 `POST /admin/users/token`。
  2. 用非 JSON 的 body（`not-json`）带正确密钥调用 `POST /admin/users/token`。
- 预期结果：
  1. 步骤 1：HTTP 401，响应体是 JSON 对象，含整数字段 `code`=4013 和字符串字段 `msg`（非空），不含 `token`。
  2. 步骤 2：HTTP 400，`code`=4000（参数不合法），响应体同样为 `{code,msg}`。

### TC-SYS-01-05 WS 请求错误通过 ack.error 返回，不带 data
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 platform=web 连接成功；`ghost-user` 从未注册。
- 步骤：
  1. alice 发送 `{"id":"r1","cmd":"send",...}`，单聊目标 `ghost-user`，`clientMsgId`=`cid-sys-0001`，文本 "hi"。
  2. alice 发送 `{"id":"r2","cmd":"nope"}`。
- 预期结果：
  1. 收到 `{"id":"r1","cmd":"ack","error":{"code":4042,...}}`，不含 `data` 字段。
  2. 收到 `id`=`r2` 的 ack，`error.code`=4001，不含 `data`。
  3. 连接保持打开（请求级错误不关闭连接），随后 `ping` 能收到 `pong`。

### TC-SYS-01-06 连接级致命错误先下发 kicked、再用同号关闭码关闭
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端测试模式启动。
- 步骤：
  1. 用 token=`bad-token`、platform=web 连接 `/ws`，读取所有帧直到连接关闭。
  2. 签发 alice 的 token（ttlSeconds=1），等待 2 秒后连接，读取所有帧直到关闭。
- 预期结果：
  1. 步骤 1：WebSocket 升级成功（HTTP 101，而非 HTTP 401）；第一个且唯一的数据帧为 `{"cmd":"event","data":{"type":"kicked","reason":"tokenInvalid"}}`，可完整解析为 JSON；随后关闭码为 4010。
  2. 步骤 2：收到 `kicked`/`tokenExpired`，随后关闭码为 4011。
  3. 两种情况下 kicked 帧均在关闭帧之前到达（客户端读到 kicked 后再读到 close）。
- 备注：现有 `TestRejectsInvalidToken` 断言 HTTP 401，与 PRD v1.1 冲突，需按本用例改写。4009 不发 kicked（C-05），见 TC-SYS-01-14。

### TC-SYS-01-07 第 4 节带错误码 AC 的错误码回归汇总
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：其余模块用例已实现（本模块范围内见 TC-USER、TC-AUTH、TC-CONN；MSG/GRP/CONV/SYNC 由另一组用例覆盖）。
- 步骤：
  1. 在 CI 中汇总所有断言错误码的后端 e2e 用例，生成"AC → 断言的 code / 关闭码 / HTTP 状态"清单。
  2. 与 PRD 第 4 节每条带错误码的 AC 逐条对照。
- 预期结果：
  1. PRD 第 4 节中每条写明错误码的 AC 都至少有一个后端 e2e 断言该码，且断言值与 AC 一致（如 AC-USER-01-3 → 400/4131，AC-AUTH-01-2 → 401/4013，AC-AUTH-02-6 → 关闭码 4009，AC-CONN-01-2 → 关闭码 4408）。
  2. 清单中无断言旧码（如 4030 代替 4033/4032、4040 代替 4043、4010 代替 4013）的用例。

### TC-SYS-01-08 TS SDK 错误枚举与 errors.json 一致
- 关联 AC：AC-SYS-01-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：`sdk-ts` 中定义了错误码枚举。
- 步骤：
  1. 单测读取 `protocol/errors.json`。
  2. 遍历 TS SDK 错误枚举，比较 code 集合与名称。
- 预期结果：
  1. code 集合完全相等（28 个），名称一一对应（如 `TOKEN_REVOKED = 4012`）。
  2. 若 SDK 中带有 `retryable` 属性，与 errors.json 的 `retryable` 一致（4290、5000、5030 为 true，其余为 false）。
  3. SDK 的关闭码表与 `closeCodes` 一致，包括 `reconnect` 标志（1000、4009～4012 为 false，1001、1009、4400、4408 为 true）。

### TC-SYS-01-09 未定义错误码 4999 按 4xx 区段兜底
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；已鉴权连接。
- 步骤：
  1. 调用 SDK 发送一条单聊消息（`clientMsgId`=`cid-4999`）。
  2. 假传输层对该请求回 `{"id":"<同 id>","cmd":"ack","error":{"code":4999,"msg":"xxx"}}`。
  3. 假时钟推进 60 秒。
- 预期结果：
  1. SDK 按前三位 499 兜底（C-09：不属于 403/404/429，归入"其余 4xxx"），归类为"请求出错，不可重试"，向上层抛出的错误码为 4999、分类为不可重试。
  2. 推进 60 秒内，假传输层没有收到第二次带 `cid-4999` 的 send（不自动重试）。
  3. 连接不被断开、不触发重连。

### TC-SYS-01-10 未定义错误码 4999 在界面提示"请求出错"
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录 H5，打开与 bob 的聊天页；Playwright 拦截 WebSocket（`routeWebSocket`），把 alice 下一条 send 的 ack 替换为 `error.code=4999`。
- 步骤：
  1. alice 输入 "hello" 并发送。
- 预期结果：
  1. 界面出现提示"请求出错"。
  2. 该消息显示为发送失败（不再自动重试，WS 上只出现 1 次该 `clientMsgId` 的 send）。
  3. 界面不展示服务端的 `msg` 原文 "xxx"。

### TC-SYS-01-11 界面文案按 code 映射，不展示服务端 msg
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录；Playwright 拦截 WS，把 ack 的 `msg` 改为 `SERVER_RAW_MSG_42`，code 保持服务端原值。
- 步骤：
  1. alice 通过"发起单聊"给 `ghost-user`（未注册）发 "hi"。
- 预期结果：
  1. 界面提示"用户不存在"（4042 的映射文案）。
  2. 页面任何位置都不出现 `SERVER_RAW_MSG_42`。

### TC-SYS-01-12 未定义码按前三位兜底：4039/4049/4299
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；已鉴权连接。
- 步骤：
  1. 分别发送 3 条消息，假传输层依次回 `error.code` = 4039、4049、4299。
  2. 每种情况推进假时钟 60 秒。
- 预期结果：
  1. 4039 → 归类"无权限"，提示键对应文案"无权限"，不重试。
  2. 4049 → 归类"不存在"，文案"内容不存在"，不重试。
  3. 4299 → 归类"频率限制"，文案"发送太频繁"（C-09）。
  4. 60 秒内 4039、4049 对应的 `clientMsgId` 都只发过 1 次。

### TC-SYS-01-13 未定义码 5999 视为可重试并提示服务异常
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：同 TC-SYS-01-12。
- 步骤：
  1. 发送消息 `cid-5999`，假传输层回 `error.code`=5999。
- 预期结果：
  1. SDK 把错误归类为可重试（与 5000/5030 同类），文案"服务异常，请稍后重试"（C-09）。
  2. 消息标记为发送失败且允许重发；连接不断开。

### TC-SYS-01-14 平台不合法不发 kicked，直接以 4009 关闭
- 关联 AC：AC-SYS-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 有效 token T。
- 步骤：
  1. 以 `token=T&platform=tv` 连接，读取所有帧直到关闭。
- 预期结果：
  1. 关闭前没有收到任何数据帧（无 `kicked`，C-05）。
  2. 关闭码 4009。

### TC-SYS-01-15 未定义子区段码在界面上的兜底文案
- 关联 AC：AC-SYS-01-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录并打开与 bob 的聊天页；Playwright `routeWebSocket` 按次把 send 的 ack 改为 `error.code` = 4049、5999。
- 步骤：
  1. alice 发 "x1"（ack 为 4049）。
  2. alice 发 "x2"（ack 为 5999）。
- 预期结果：
  1. 步骤 1 提示"内容不存在"，"x1"显示发送失败。
  2. 步骤 2 提示"服务异常，请稍后重试"，"x2"显示发送失败，可点击重发。

---

## 覆盖检查

- AC-SYS-01-1：TC-SYS-01-01、TC-SYS-01-02
- AC-SYS-01-2：TC-SYS-01-03、TC-SYS-01-04、TC-SYS-01-05、TC-SYS-01-06、TC-SYS-01-07、TC-SYS-01-14
- AC-SYS-01-3：TC-SYS-01-08
- AC-SYS-01-4：TC-SYS-01-09、TC-SYS-01-10、TC-SYS-01-11、TC-SYS-01-12、TC-SYS-01-13、TC-SYS-01-15

## 已澄清

1. KMP SDK 枚举一致性是否纳入一期 → C-10：不纳入，放到 M5；本文件不设 KMP 用例。
2. 未定义码落在已知子区段 / 未定义 5xxx 如何兜底 → C-09：按前三位兜底，未定义 5xxx 可重试；新增 TC-SYS-01-12、TC-SYS-01-13、TC-SYS-01-15，修订 TC-SYS-01-09。
3. 4009 是否先发 kicked → C-05（PRD v1.1 F-SYS-01 规则 2）：不发，直接以 4009 关闭；新增 TC-SYS-01-14。
