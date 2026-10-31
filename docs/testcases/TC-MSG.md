# 测试用例：消息（MSG）

> 依据：[PRD v1.1](../prd/PRD-core-v1.1.md) F-MSG-01～F-MSG-07 及 §9 补充规则（C-26～C-33）、[需求澄清](../review/CLARIFY-core-v1.0.md)、[ERRORS.md](../../protocol/ERRORS.md)、[protocol/README.md](../../protocol/README.md)（PRD §8 的协议变更以 PRD 为准）。
>
> 通用约定（适用于本文件全部用例）：
> - 每个用例使用独立数据库；服务端以测试模式启动（PRD §6.1）。
> - 用户 `alice`、`bob`、`carol` 已通过管理接口导入（存在）；`ghost` 从未导入、也从未签发过凭证（不存在）。
> - 单聊会话 ID：`c2c:alice:bob`；群会话 ID：`group:g1`。
> - 故障注入：`POST /admin/test/drop-ack {clientMsgId}`（保存但不回 ack）、`POST /admin/test/skip-delivery {clientMsgId}`（保存但不实时投递）。
> - "UI 自动化"指 Playwright，两个独立浏览器上下文分别登录 alice / bob（token 存 `sessionStorage`，互不影响）。
> - "SDK 单测"指 TS SDK 单测，注入假 WebSocket（假传输层）和假时钟。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-MSG-01-01 | 发送"你好"：先显示发送中，收到 ack 后变为已发送 | AC-MSG-01-1 | UI 自动化 | P0 |
| TC-MSG-01-02 | 空白输入（空、空格、换行、制表符、全角空格）发送按钮不可点 | AC-MSG-01-2 | UI 自动化 | P0 |
| TC-MSG-01-03 | 前端长度边界：1 / 5000 可发，5001 显示"已超出 1 字"并置灰按钮 | AC-MSG-01-3 | UI 自动化 | P0 |
| TC-MSG-01-04 | 前端 emoji 计数：5000 个 emoji 可发，5001 个显示"已超出 1 字" | AC-MSG-01-3 | UI 自动化 | P0 |
| TC-MSG-01-05 | 前端先去首尾空白再计数：首尾空白 + 5000 字可发 | AC-MSG-01-3 | UI 自动化 | P0 |
| TC-MSG-01-06 | SDK 码点计数函数：代理对、ZWJ 序列、组合字符、国旗 | AC-MSG-01-3 | SDK 单测 | P0 |
| TC-MSG-01-07 | 服务端：超过 5000 码点返回 4006，去空白后为空返回 4005 | AC-MSG-01-4 | 后端 e2e | P0 |
| TC-MSG-01-08 | 服务端长度边界：1 / 5000 / 5000 emoji / 5001 emoji / 首尾空白 | AC-MSG-01-4 | 后端 e2e | P0 |
| TC-MSG-01-09 | "  你好  "存储和下发为"你好"，中间空白保留 | AC-MSG-01-5 | 后端 e2e | P0 |
| TC-MSG-01-10 | 同一会话 seq 严格递增，双方 ack 的 convId 一致 | AC-MSG-01-6 | 后端 e2e | P0 |
| TC-MSG-01-11 | 单聊会话 ID 按 ASCII 字节序（大小写、数字） | AC-MSG-01-6 | 后端 e2e | P0 |
| TC-MSG-01-12 | 给自己发消息，会话 ID 为 c2c:alice:alice | AC-MSG-01-7 | 后端 e2e | P0 |
| TC-MSG-01-13 | 给不存在的用户发消息返回 4042，且不保存 | AC-MSG-01-8 | 后端 e2e | P0 |
| TC-MSG-01-14 | 给格式不合法的 userId 发消息返回 4004 | AC-MSG-01-8 | 后端 e2e | P0 |
| TC-MSG-01-15 | 收到 4042：消息标记失败并提示"用户不存在" | AC-MSG-01-9 | UI 自动化 | P0 |
| TC-MSG-01-16 | 前端空白按 Unicode White_Space 判定：\r、U+00A0 等算空白，U+200B / U+FEFF 不算 | AC-MSG-01-2 | UI 自动化 | P0 |
| TC-MSG-01-17 | 服务端空白按 Unicode White_Space 判定：去除并返回 4005；U+200B / U+FEFF 保留 | AC-MSG-01-4 | 后端 e2e | P0 |
| TC-MSG-01-18 | SDK 与服务端共用 25 个 White_Space 字符的测试数据，判定一致 | AC-MSG-01-2 | SDK 单测 | P0 |
| TC-MSG-02-01 | 双方在线：alice 发、bob 实时收到 | AC-MSG-02-1 | UI 自动化 | P0 |
| TC-MSG-02-02 | 服务端下发的 msg 帧字段完整 | AC-MSG-02-1 | 后端 e2e | P0 |
| TC-MSG-02-03 | bob 正在看会话：新消息追加，未读不增加，并上报已读 | AC-MSG-02-2 | UI 自动化 | P0 |
| TC-MSG-02-04 | bob 在会话列表页：会话移到顶部、摘要更新、未读 +1 | AC-MSG-02-3 | UI 自动化 | P0 |
| TC-MSG-02-05 | 先到 seq=3 后到 seq=2，按 2、3 展示 | AC-MSG-02-4 | SDK 单测 | P0 |
| TC-MSG-02-06 | 未确认消息排在已有 seq 之后，收到 ack 后移到 seq 位置 | AC-MSG-02-4 | SDK 单测 | P0 |
| TC-MSG-03-01 | 服务端不回 ack，10 秒（±1 秒）后显示发送失败 | AC-MSG-03-1 | UI 自动化 | P0 |
| TC-MSG-03-02 | SDK 发送超时边界：9.9 秒仍发送中、10 秒失败；错误 ack 立即失败 | AC-MSG-03-1 | SDK 单测 | P0 |
| TC-MSG-03-03 | 离线发送显示发送中，恢复网络后自动发出，对方只收到一条 | AC-MSG-03-2 | UI 自动化 | P0 |
| TC-MSG-03-04 | 离线队列重连后只自动发 1 次，离线期间不计时 | AC-MSG-03-2 | SDK 单测 | P0 |
| TC-MSG-03-05 | 失败消息（4290）手动重发后变为已发送，对方只收到一条 | AC-MSG-03-3 | UI 自动化 | P0 |
| TC-MSG-03-06 | 手动重发使用原 clientMsgId | AC-MSG-03-3 | SDK 单测 | P0 |
| TC-MSG-03-07 | ack 被丢弃后手动重发：对方只收到一条，msgId 不变（界面） | AC-MSG-03-4 | UI 自动化 | P0 |
| TC-MSG-03-08 | ack 被丢弃后重发：服务端返回第一次的 msgId / seq | AC-MSG-03-4 | 后端 e2e | P0 |
| TC-MSG-03-09 | 本地待发消息与 sync 回来的同 clientMsgId 消息合并，不重复 | AC-MSG-03-4 | SDK 单测 | P0 |
| TC-MSG-03-10 | 有发送失败的消息时刷新，弹出离开确认 | AC-MSG-03-5 | UI 自动化 | P0 |
| TC-MSG-03-11 | 有待发消息时刷新弹确认；没有待发 / 失败消息时不弹 | AC-MSG-03-5 | UI 自动化 | P0 |
| TC-MSG-03-12 | 超时标记失败后 ack 才到达：以 ack 为准变为已发送 | AC-MSG-03-1 | UI 自动化 | P0 |
| TC-MSG-04-01 | 同一 clientMsgId 重复提交 5 次只保存 1 条 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-04-02 | 同一 clientMsgId 内容不同：以第一次为准 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-04-03 | 同一 clientMsgId 并发提交 5 次只保存 1 条 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-04-04 | 不同发送者使用相同 clientMsgId 互不影响 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-04-05 | 同一用户两个连接用相同 clientMsgId 发到同一会话，只保存 1 条 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-04-06 | 同一 clientMsgId 发到另一个单聊 / 群返回 4008 | AC-MSG-04-2 | 后端 e2e | P0 |
| TC-MSG-04-07 | 收到 1、3，500ms 内无 2：发起 sync，最终展示 1、2、3 | AC-MSG-04-3 | SDK 单测 | P0 |
| TC-MSG-04-08 | 跳过 seq=2 的实时投递（故障注入）：界面最终展示 1、2、3 | AC-MSG-04-3 | UI 自动化 | P0 |
| TC-MSG-04-09 | 本地 maxSeq=0 时先收到 seq=3：补拉 1、2 | AC-MSG-04-3 | SDK 单测 | P0 |
| TC-MSG-04-10 | sync 结果与迟到的 msg 帧重叠时按 msgId 去重 | AC-MSG-04-3 | SDK 单测 | P0 |
| TC-MSG-04-11 | 收到 1、3，200ms 后 2 到达，不发起 sync | AC-MSG-04-4 | SDK 单测 | P0 |
| TC-MSG-04-12 | ack 带回的 seq 计入本地 maxSeq，不误判缺口 | AC-MSG-04-4 | SDK 单测 | P0 |
| TC-MSG-04-13 | 5 人群内各并发 20 条：所有成员 10 秒内看到 seq 1～100 | AC-MSG-04-5 | 后端 e2e | P0 |
| TC-MSG-04-14 | 返回 ack 后 kill -9，重启后 sync 能拉到该消息 | AC-MSG-04-6 | 后端 e2e | P0 |
| TC-MSG-04-15 | msg 帧和 sync 结果中每条消息都带 clientMsgId | AC-MSG-04-7 | 后端 e2e | P0 |
| TC-MSG-04-16 | clientMsgId 长度 7 / 65 返回 4000，8 / 64 成功 | AC-MSG-04-1 | 后端 e2e | P0 |
| TC-MSG-05-01 | bob 离线期间收到 5 条，上线打开会话看到全部 5 条 | AC-MSG-05-1 | UI 自动化 | P0 |
| TC-MSG-05-02 | 100 条消息：打开显示 81～100，翻页到 1 后显示"没有更多了" | AC-MSG-05-2 | UI 自动化 | P0 |
| TC-MSG-05-03 | 历史加载失败显示"加载失败，点击重试"，重试成功 | AC-MSG-05-2 | UI 自动化 | P0 |
| TC-MSG-05-04 | sync afterSeq / beforeSeq 结果升序、hasMore 正确 | AC-MSG-05-3 | 后端 e2e | P0 |
| TC-MSG-05-05 | limit 默认 50；limit=200 上限；limit=201 返回 4000 | AC-MSG-05-4 | 后端 e2e | P0 |
| TC-MSG-05-06 | limit=0 / 负数、afterSeq 与 beforeSeq 同时传返回 4000 | AC-MSG-05-4 | 后端 e2e | P0 |
| TC-MSG-05-07 | 拉取别人的单聊 4032、非成员拉群 4033、不存在的群 4043 | AC-MSG-05-5 | 后端 e2e | P0 |
| TC-MSG-05-08 | 会话 ID 不规范（顺序颠倒、格式错误）返回 4007 | AC-MSG-05-5 | 后端 e2e | P0 |
| TC-MSG-05-09 | 加载历史后当前可视位置不跳动 | AC-MSG-05-6 | UI 自动化 | P0 |
| TC-MSG-05-10 | sync 200 条 × 5000 字（约 3MB 下行帧）正常返回，连接不断开 | AC-MSG-05-4 | 后端 e2e | P0 |
| TC-MSG-05-11 | SDK 不限制下行帧大小，能解析约 3MB 的 sync 应答 | AC-MSG-05-4 | SDK 单测 | P0 |
| TC-MSG-06-01 | HTML / 脚本内容原样显示为文本，不执行 | AC-MSG-06-1 | UI 自动化 | P0 |
| TC-MSG-06-02 | 3 行消息按 3 行显示；300 字符长串断行，无横向滚动条 | AC-MSG-06-2 | UI 自动化 | P0 |
| TC-MSG-06-03 | PC 上 Enter 发送、Shift+Enter 换行 | AC-MSG-06-2 | UI 自动化 | P0 |
| TC-MSG-06-04 | 时间分隔：间隔 4 分钟不显示、6 分钟显示；昨天显示"昨天 HH:mm" | AC-MSG-06-3 | UI 自动化 | P0 |
| TC-MSG-06-05 | 时间格式边界：正好 5 分钟、今年、跨年、会话列表 | AC-MSG-06-3 | UI 自动化 | P0 |
| TC-MSG-06-06 | 查看历史时收到 2 条新消息：显示"↓ 2 条新消息"，不自动滚动 | AC-MSG-06-4 | UI 自动化 | P0 |
| TC-MSG-06-07 | 距底部 ≤100px 时新消息自动滚到底 | AC-MSG-06-4 | UI 自动化 | P0 |
| TC-MSG-06-08 | iOS Safari 键盘弹起时输入框不被遮挡 | AC-MSG-06-5 | 手工 | P0 |
| TC-MSG-06-09 | 跨天总显示时间；翻页后新的第一条显示时间，原第一条按间隔规则 | AC-MSG-06-3 | UI 自动化 | P0 |
| TC-MSG-07-01 | 1 秒内发 25 条：前 20 条成功，其余 4290；1 秒后可继续 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-02 | 限流按用户所有连接合计 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-03 | 限流按用户隔离：两个用户各 20 条都成功 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-04 | 单聊和群消息共享同一个限流额度 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-05 | 持续以 20 条/秒匀速发送 5 秒，全部成功 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-06 | 收到 4290：标记失败并提示"发送太频繁，请稍后再试" | AC-MSG-07-2 | UI 自动化 | P0 |
| TC-MSG-07-07 | 被 4290 拒绝的请求不消耗令牌 | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-08 | 去重命中的重发不消耗令牌、桶空时也返回原 ack | AC-MSG-07-1 | 后端 e2e | P0 |
| TC-MSG-07-09 | 令牌桶容量 20：空闲 3 秒后突发 25 条只过 20 条，0.5 秒后补充 10 个 | AC-MSG-07-1 | 后端 e2e | P0 |

---

## F-MSG-01 单聊发送文本消息

### TC-MSG-01-01 发送"你好"：先显示发送中，收到 ack 后变为已发送
- 关联 AC：AC-MSG-01-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 各自在一个浏览器上下文登录，均在线；alice 打开 `/chat/c2c:alice:bob`；会话内已有 2 条历史消息（seq 1、2）。
- 步骤：
  1. alice 在输入框输入"你好"，点击发送。
  2. 立即（ack 返回前）检查消息列表；为保证能观察到"发送中"，可对该消息的 clientMsgId 注入 drop-ack 后 1 秒内检查，然后另用一条不注入的消息验证 ack 后状态。
  3. 等待 ack 返回。
- 预期结果：
  1. 点击后输入框清空，"你好"立即出现在列表最底部，位于 seq=2 之后，状态为"发送中"。
  2. 收到 ack 后状态变为"已发送"，该消息 seq=3。
  3. bob 端收到一条"你好"。

### TC-MSG-01-02 空白输入（空、空格、换行、制表符、全角空格）发送按钮不可点
- 关联 AC：AC-MSG-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录，打开与 bob 的聊天页。
- 步骤：依次把输入框内容设置为以下值，每次检查发送按钮状态，并在 PC 上按 Enter：
  1. 空字符串 `""`
  2. `"   "`（3 个半角空格）
  3. `"\n\n"`（2 个换行，用 Shift+Enter 输入）
  4. `"\t"`
  5. `"　"`（U+3000 全角空格）
  6. 混合：`" \n\t　 "`
  7. 对照组：`" a "`
- 预期结果：
  1. 第 1～6 种输入，发送按钮为禁用状态（`disabled`），按 Enter 不发出任何 `send` 帧，消息列表无新增。
  2. 第 7 种输入，发送按钮可点；发送后列表中显示的内容为"a"。

### TC-MSG-01-03 前端长度边界：1 / 5000 可发，5001 提示"消息过长"
- 关联 AC：AC-MSG-01-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 在线；alice 打开与 bob 的聊天页。
- 步骤：
  1. 输入"a"（1 个字符），发送。
  2. 输入 5000 个"中"，发送。
  3. 输入 5001 个"中"，尝试点击发送、按 Enter。
  4. 继续输入到 5003 个"中"；再删到 5000 个。
- 预期结果：
  1. 第 1、2 条均变为"已发送"，seq 分别为 1、2；bob 收到的第 2 条文本长度为 5000 个码点。
  2. 第 3 步输入框下方实时显示"已超出 1 字"，发送按钮置灰；点击和 Enter 都不发出 `send` 帧，列表不新增消息，bob 无新消息；输入框内容保留（C-28）。
  3. 第 4 步提示依次变为"已超出 3 字"；删到 5000 个时提示消失，按钮恢复可点。

### TC-MSG-01-04 前端 emoji 计数：5000 个 emoji 可发，5001 个提示"消息过长"
- 关联 AC：AC-MSG-01-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同 TC-MSG-01-03。
- 步骤：
  1. 输入 5000 个"😀"（U+1F600，JS 中 `length` 为 10000），发送。
  2. 输入 5001 个"😀"，点击发送。
- 预期结果：
  1. 第 1 步成功发送，变为"已发送"；bob 收到的文本为 5000 个码点（`[...text].length === 5000`）。
  2. 第 2 步显示"已超出 1 字"，发送按钮置灰，不发送。
  3. 前端不能用 UTF-16 `length` 计数（否则 2501 个 emoji 就会被误判为超长）。

### TC-MSG-01-05 前端先去首尾空白再计数：首尾空白 + 5000 字可发
- 关联 AC：AC-MSG-01-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同 TC-MSG-01-03。
- 步骤：
  1. 输入 `"  " + "中"×5000 + "\n　"`（去空白前 5004 个码点），发送。
  2. 输入 `"中"×2500 + " " + "中"×2500 + " "`（去首尾空白后 5001 个码点，中间空格计入），发送。
- 预期结果：
  1. 第 1 步可以发送，变为"已发送"；bob 收到的内容为 5000 个"中"（首尾空白已去掉）。
  2. 第 2 步显示"已超出 1 字"，发送按钮置灰，不发送。

### TC-MSG-01-06 SDK 码点计数函数：代理对、ZWJ 序列、组合字符、国旗
- 关联 AC：AC-MSG-01-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：TS SDK 暴露文本校验 / 计数函数（如 `validateText(text)` 返回去空白后的码点数或错误）。
- 步骤：对以下输入调用计数函数：
  1. `"a"`、`"你"`、`"😀"`
  2. `"👨‍👩‍👧"`（U+1F468 U+200D U+1F469 U+200D U+1F467）
  3. `"🇨🇳"`（两个区域指示符）
  4. `"é"` 预组合（U+00E9）与分解形式（U+0065 U+0301）
  5. `"👨‍👩‍👧"×1000`、`"👨‍👩‍👧"×1001`
  6. `"　你好　"`（首尾全角空格）、`"\t\n"`
- 预期结果：
  1. 分别为 1、1、1。
  2. 为 5（按码点，不按字形簇；C-27 确认为预期行为）。
  3. 为 2。
  4. 预组合为 1，分解形式为 2（不做 Unicode 归一化）。
  5. 1000 个家庭 emoji = 5000 码点，校验通过；1001 个 = 5005 码点，返回 TEXT_TOO_LONG（4006）。
  6. 去空白后分别为 2 和 0；后者返回 TEXT_EMPTY（4005）。
  7. 结果与服务端（TC-MSG-01-08）对同一输入的判定完全一致。

### TC-MSG-01-07 服务端：超过 5000 码点返回 4006，去空白后为空返回 4005
- 关联 AC：AC-MSG-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 通过测试 WS 客户端直连（绕过前端校验）。
- 步骤：alice 以 `send`（convType=c2c, to=bob）依次发送，每条用不同的 clientMsgId：
  1. text = `"中"×5001`
  2. text = `""`
  3. text = `"   "`
  4. text = `"\n\t"`
  5. text = `"　　"`（全角空格）
  6. 完成后 bob 执行 `sync {convId:"c2c:alice:bob", afterSeq:0}`。
- 预期结果：
  1. 第 1 条 ack.error.code = 4006。
  2. 第 2～5 条 ack.error.code = 4005。
  3. bob 未收到任何 `msg` 帧；sync 返回 0 条消息。

### TC-MSG-01-08 服务端长度边界：1 / 5000 / 5000 emoji / 5001 emoji / 首尾空白
- 关联 AC：AC-MSG-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-MSG-01-07。
- 步骤：alice 依次发送：
  1. `"a"`
  2. `"中"×5000`
  3. `"😀"×5000`（UTF-8 20000 字节，单帧 < 64KB）
  4. `"😀"×5001`
  5. `"  " + "中"×5000 + "\n"`
  6. `"👨‍👩‍👧"×1001`（5005 码点）
- 预期结果：
  1. 第 1、2、3、5 条成功，seq 依次为 1、2、3、4。
  2. 第 4、6 条返回 4006，不分配 seq。
  3. sync 拉到 4 条；seq=4 的文本为 5000 个"中"（无首尾空白）。

### TC-MSG-01-09 "  你好  "存储和下发为"你好"，中间空白保留
- 关联 AC：AC-MSG-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线；alice 另有一个 pc 连接在线。
- 步骤：
  1. alice(web) 发送 `"  你好  "`。
  2. alice(web) 发送 `"　\t你好\n"`。
  3. alice(web) 发送 `" 你 好\n吗 "`。
  4. bob 执行 sync afterSeq=0。
- 预期结果：
  1. bob 收到的 `msg` 帧 body.text 依次为 `"你好"`、`"你好"`、`"你 好\n吗"`。
  2. alice(pc) 收到的 `msg` 帧内容相同。
  3. sync 结果中 3 条消息的 body.text 与上面一致（存储的是去空白后的内容）。

### TC-MSG-01-10 同一会话 seq 严格递增，双方 ack 的 convId 一致
- 关联 AC：AC-MSG-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线，会话 `c2c:alice:bob` 无消息。
- 步骤：
  1. alice 连续发送 3 条（m1、m2、m3）。
  2. bob 发送 1 条（m4）给 alice。
  3. alice 再发 1 条（m5）。
- 预期结果：
  1. ack.seq 依次为 1、2、3、4、5，严格递增且连续；每个 ack 带非空 `msgId`（互不相同）和 `ts`（毫秒，单调不减）。
  2. 5 个 ack 的 `convId` 都等于 `"c2c:alice:bob"`（bob 发送的 ack 也是这个值，而不是 `c2c:bob:alice`）。

### TC-MSG-01-11 单聊会话 ID 按 ASCII 字节序（大小写、数字）
- 关联 AC：AC-MSG-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：导入用户 `Bob`、`alice`、`user9`、`user10`、`a-1`、`a_1`。
- 步骤：
  1. `alice` 给 `Bob` 发消息。
  2. `user9` 给 `user10` 发消息。
  3. `a_1` 给 `a-1` 发消息。
- 预期结果：
  1. ack.convId = `c2c:Bob:alice`（'B'=0x42 < 'a'=0x61）。
  2. ack.convId = `c2c:user10:user9`（'1'=0x31 < '9'=0x39，按字节序而非数值）。
  3. ack.convId = `c2c:a-1:a_1`（'-'=0x2D < '_'=0x5F）。
  4. `Bob` 与 `bob` 是两个不同用户，分属不同会话。

### TC-MSG-01-12 给自己发消息，会话 ID 为 c2c:alice:alice
- 关联 AC：AC-MSG-01-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 web 和 pc 两个连接在线。
- 步骤：
  1. alice(web) 发送 `{convType:"c2c", to:"alice", text:"备忘"}`。
  2. 观察 alice(pc) 和 alice(web) 收到的帧 500ms。
  3. alice 执行 `convs`。
- 预期结果：
  1. ack 成功，seq=1，convId = `c2c:alice:alice`。
  2. alice(pc) 恰好收到 1 个 `msg` 帧（不会因"既是接收方又是发送方的其他端"收到 2 次）；alice(web) 只收到 ack，没有 `msg` 帧。
  3. convs 中包含 `c2c:alice:alice`，unread=0（发言即已读）。

### TC-MSG-01-13 给不存在的用户发消息返回 4042，且不保存
- 关联 AC：AC-MSG-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线；`ghost` 不存在。
- 步骤：
  1. alice 发送 `{to:"ghost", clientMsgId:"cid-ghost-0001", text:"在吗"}`。
  2. alice 执行 `convs`。
  3. 导入用户 `ghost`，ghost 连接后执行 `sync {convId:"c2c:alice:ghost", afterSeq:0}` 和 `convs`。
  4. alice 用同一个 `cid-ghost-0001` 再发一次给 ghost。
- 预期结果：
  1. 第 1 步 ack.error.code = 4042。
  2. alice 的 convs 中没有 `c2c:alice:ghost`。
  3. ghost 的 sync 返回 0 条，convs 为空。
  4. 第 4 步成功，seq=1（第一次失败没有占用 seq，也没有占用 clientMsgId）。

### TC-MSG-01-14 给格式不合法的 userId 发消息返回 4004
- 关联 AC：AC-MSG-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线。
- 步骤：alice 分别给 `to` = `"bad id"`、`"张三"`、`"a.b"`、`"a@b"`、65 个 `a` 发消息。
- 预期结果：每条 ack.error.code = 4004，不保存消息。

### TC-MSG-01-15 收到 4042：消息标记失败并提示"用户不存在"
- 关联 AC：AC-MSG-01-9
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 已登录；`ghost` 不存在。
- 步骤：
  1. alice 通过"发起单聊"输入 `ghost`，进入聊天页。
  2. 发送"你好"。
- 预期结果：
  1. 消息状态变为"发送失败"（显示失败图标 / 重发入口）。
  2. 出现提示"用户不存在"（按错误码映射，不展示服务端 msg）。

### TC-MSG-01-16 前端空白按 Unicode White_Space 判定：\r、U+00A0 等算空白，U+200B / U+FEFF 不算
- 关联 AC：AC-MSG-01-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 在线；alice 打开与 bob 的聊天页（输入内容通过 `fill` 写入）。
- 步骤：依次把输入框设为以下值，检查发送按钮：
  1. `"\r"`、`" "`（不换行空格）、`" "`（全角宽空格 EM SPACE）、`" "`（行分隔符）、`"\u0085"`、`"\r\n 　"`
  2. `"​"`（零宽空格），点击发送
  3. `"﻿"`，点击发送
- 预期结果：
  1. 第 1 步每种输入发送按钮都禁用（C-26：以 Unicode `White_Space` 为准）。
  2. 第 2、3 步按钮可点，发送成功，状态变为"已发送"；bob 收到的 body.text 分别为 `"​"`、`"﻿"`（不被当作空白去掉）。

### TC-MSG-01-17 服务端空白按 Unicode White_Space 判定：去除并返回 4005；U+200B / U+FEFF 保留
- 关联 AC：AC-MSG-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 通过测试 WS 客户端直连。
- 步骤：alice 依次发送：
  1. `" \r 　 "`
  2. `" 你好 "`
  3. `"​"`
  4. `"﻿你好​"`
- 预期结果：
  1. 第 1 条返回 4005。
  2. 第 2 条成功，存储和下发为 `"你好"`。
  3. 第 3 条成功，内容为 `"​"`（1 个码点）。
  4. 第 4 条成功，内容原样为 `"﻿你好​"`（4 个码点）。

### TC-MSG-01-18 SDK 与服务端共用 25 个 White_Space 字符的测试数据，判定一致
- 关联 AC：AC-MSG-01-2
- 优先级：P0
- 层级：SDK 单测
- 前置条件：仓库中存在服务端与 SDK 共用的空白字符表 / 测试数据文件（C-26）。
- 步骤：
  1. 读取共用测试数据，断言其中恰好有 25 个 `White_Space` 码点：U+0009～U+000D、U+0020、U+0085、U+00A0、U+1680、U+2000～U+200A、U+2028、U+2029、U+202F、U+205F、U+3000。
  2. 对每个字符 c 调用 SDK 校验：`c`、`c+"a"+c`。
  3. 对 U+200B、U+FEFF、U+180E 调用同样的校验。
  4. 服务端 Go 单测跑同一份数据。
- 预期结果：
  1. 每个 `c` 单独输入返回 TEXT_EMPTY（4005）；`c+"a"+c` 去空白后为 `"a"`。
  2. U+200B、U+FEFF、U+180E 不算空白，计 1 个码点。
  3. SDK 与服务端对每条数据的结果完全一致。

## F-MSG-02 接收消息

### TC-MSG-02-01 双方在线：alice 发、bob 实时收到
- 关联 AC：AC-MSG-02-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 两个浏览器上下文均已登录在线；bob 打开与 alice 的聊天页。
- 步骤：
  1. alice 发送"hello bob"。
- 预期结果：
  1. bob 页面 1 秒内出现"hello bob"，显示为对方消息（左侧）。
  2. bob 端未刷新、未手动拉取。（延迟 P99 由 TC-PERF-00-01 度量。）

### TC-MSG-02-02 服务端下发的 msg 帧字段完整
- 关联 AC：AC-MSG-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：alice 以 clientMsgId=`cid-00000001` 发送"hi bob"。
- 预期结果：bob 收到 `msg` 帧，满足：`msgId` = ack.msgId、`seq`=1、`from`="alice"、`convId`="c2c:alice:bob"、`body.text`="hi bob"、`ts` = ack.ts、`clientMsgId`="cid-00000001"。

### TC-MSG-02-03 bob 正在看会话：新消息追加，未读不增加，并上报已读
- 关联 AC：AC-MSG-02-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 打开 `/chat/c2c:alice:bob`，页面可见；会话已有 seq 1～2，bob readSeq=2。
- 步骤：
  1. alice 发送 1 条消息（seq=3）。
  2. 等待 1.5 秒（超过已读节流 1 秒）。
  3. bob 返回会话列表；另以 bob 的测试 WS 连接执行 `convs`。
- 预期结果：
  1. 新消息追加在列表底部。
  2. bob 发出 `read {convId:"c2c:alice:bob", seq:3}`。
  3. 会话列表中该会话未读数不显示（0）；convs 返回 readSeq=3、unread=0。

### TC-MSG-02-04 bob 在会话列表页：会话移到顶部、摘要更新、未读 +1
- 关联 AC：AC-MSG-02-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 在会话列表页；列表顺序为 `c2c:bob:carol`（最新）、`c2c:alice:bob`；与 alice 的会话未读为 0。
- 步骤：alice 发送"新消息 A"。
- 预期结果：
  1. `c2c:alice:bob` 移到列表第 1 位。
  2. 摘要显示"新消息 A"，时间显示为该消息服务端时间的 `HH:mm`。
  3. 未读角标显示"1"；bob 未上报已读。

### TC-MSG-02-05 先到 seq=3 后到 seq=2，按 2、3 展示
- 关联 AC：AC-MSG-02-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：SDK 注入假传输层和假时钟；会话 `c2c:alice:bob` 本地已有 seq=1。
- 步骤：
  1. 假传输层推送 seq=3 的 `msg`。
  2. 假时钟前进 100ms，推送 seq=2 的 `msg`。
  3. 读取 SDK 对外暴露的该会话消息列表。
- 预期结果：
  1. 列表顺序为 seq 1、2、3（按 seq，不按到达顺序）。
  2. 无重复项；没有发出 `sync`（缺口在 500ms 内补上）。

### TC-MSG-02-06 未确认消息排在已有 seq 之后，收到 ack 后移到 seq 位置
- 关联 AC：AC-MSG-02-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层；本地已有 seq 1～4。
- 步骤：
  1. 本地发送消息 P（未 ack）。
  2. 假传输层推送对方的 seq=6。
  3. 假传输层回 P 的 ack，seq=5。
- 预期结果：
  1. 第 2 步后列表顺序：1、2、3、4、6、P（P 在所有已有 seq 的消息之后）。
  2. 第 3 步后：1、2、3、4、5(P)、6，P 状态为"已发送"；不发起 sync（5 由 ack 补上）。

## F-MSG-03 发送状态与失败重发

### TC-MSG-03-01 服务端不回 ack，10 秒（±1 秒）后显示发送失败
- 关联 AC：AC-MSG-03-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在线，打开与 bob 的聊天页；测试钩子固定下一条消息的 clientMsgId 为 `cid-drop-0001`，并调用 `POST /admin/test/drop-ack {clientMsgId:"cid-drop-0001"}`。
- 步骤：
  1. alice 发送"测试超时"，记录时间 T0。
  2. 观察状态变化，记录变为"发送失败"的时间 T1。
- 预期结果：
  1. T0～T0+9s 期间状态为"发送中"。
  2. T1 − T0 在 9～11 秒之间，状态变为"发送失败"，显示重发入口。

### TC-MSG-03-02 SDK 发送超时边界：9.9 秒仍发送中、10 秒失败；错误 ack 立即失败
- 关联 AC：AC-MSG-03-1
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层不回 ack；假时钟；发送超时为默认 10 秒。
- 步骤：
  1. 发送消息 M1；假时钟前进 9900ms，检查状态；再前进 100ms，检查状态。
  2. 发送消息 M2；假传输层立即回 `ack.error {code:5000}`。
  3. M1 失败后，假传输层迟到地回 M1 的成功 ack（seq=7）。
- 预期结果：
  1. 9900ms 时 M1 为"发送中"；10000ms 时为"发送失败"。
  2. M2 立即为"发送失败"，不等 10 秒。
  3. 迟到的 ack 到达后 M1 变为"已发送"，按 seq=7 归位，重发入口消失（C-30）。

### TC-MSG-03-03 离线发送显示发送中，恢复网络后自动发出，对方只收到一条
- 关联 AC：AC-MSG-03-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 在线；alice 打开与 bob 的聊天页；bob 打开与 alice 的聊天页。
- 步骤：
  1. alice 上下文 `setOffline(true)`，等待顶部出现"网络不可用"。
  2. alice 发送"离线消息"。
  3. 保持离线 15 秒（超过 10 秒发送超时）。
  4. `setOffline(false)`，等待重连。
- 预期结果：
  1. 离线期间"离线消息"一直为"发送中"，15 秒后仍不是"发送失败"（离线期间不计时）。
  2. 恢复后 5 秒内变为"已发送"。
  3. bob 只收到 1 条"离线消息"；bob 执行 sync 也只有 1 条。

### TC-MSG-03-04 离线队列重连后只自动发 1 次，离线期间不计时
- 关联 AC：AC-MSG-03-2
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层（初始断开）、假时钟。
- 步骤：
  1. 断开状态下发送 M。假时钟前进 60 秒。
  2. 假传输层连接成功；记录发出的 `send` 帧。假传输层不回 ack。
  3. 假时钟前进 10 秒；再断开并重连一次；再前进 30 秒。
- 预期结果：
  1. 第 1 步期间无 `send` 帧发出，M 为"发送中"。
  2. 连接成功后恰好发出 1 个 M 的 `send` 帧。
  3. 发出后 10 秒 M 变为"发送失败"；之后的重连不再自动发送 M（`send` 帧总数仍为 1）。

### TC-MSG-03-05 失败消息（4290）手动重发后变为已发送，对方只收到一条
- 关联 AC：AC-MSG-03-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 两个上下文在线；alice 另有一个测试 WS 连接（platform=pc）。
- 步骤：
  1. alice(pc 测试连接) 在 200ms 内发出 20 条消息给 carol，全部成功。
  2. 同一秒内 alice 在页面给 bob 发送"重发测试"。
  3. 等待 1.5 秒，点击该失败消息的重发。
- 预期结果：
  1. 第 2 步消息状态为"发送失败"，提示"发送太频繁，请稍后再试"。
  2. 重发后变为"已发送"。
  3. bob 只收到 1 条"重发测试"；sync 中该内容只出现 1 次。

### TC-MSG-03-06 手动重发使用原 clientMsgId
- 关联 AC：AC-MSG-03-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟。
- 步骤：
  1. 发送 M，记录 `send` 帧中的 clientMsgId = X；假传输层不回 ack，假时钟前进 10 秒，M 失败。
  2. 调用重发 M。
- 预期结果：
  1. 重发的 `send` 帧 clientMsgId 仍为 X，text 相同。
  2. 请求 `id` 可以不同。X 长度在 8～64 之间。

### TC-MSG-03-07 ack 被丢弃后手动重发：对方只收到一条，msgId 不变（界面）
- 关联 AC：AC-MSG-03-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 在线；alice 的下一条 clientMsgId 固定为 `cid-dropack-01`，并注入 drop-ack。
- 步骤：
  1. alice 发送"ack 丢了"；bob 端 1 秒内收到该消息，记录其 msgId = M1（从 WS 帧中读取）。
  2. 等待 alice 端变为"发送失败"（约 10 秒）。
  3. 取消 drop-ack 注入（或注入只生效一次），alice 点击重发。
- 预期结果：
  1. alice 端变为"已发送"，ack.msgId = M1、seq 与 bob 收到的相同。
  2. bob 端界面只有 1 条"ack 丢了"，没有收到第二个 `msg` 帧。

### TC-MSG-03-08 ack 被丢弃后重发：服务端返回第一次的 msgId / seq
- 关联 AC：AC-MSG-03-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线；注入 `drop-ack {clientMsgId:"cid-dropack-02"}`（只丢第一次）。
- 步骤：
  1. alice 发送 cid-dropack-02，等待 2 秒确认无 ack。
  2. bob 读取收到的 `msg`，记录 msgId=M1、seq=S1。
  3. alice 用 cid-dropack-02 再发一次。
  4. bob 执行 sync afterSeq=0。
- 预期结果：
  1. 第 3 步 ack.msgId = M1、ack.seq = S1。
  2. bob 只收到 1 个 `msg` 帧（200ms 内无第二帧）；sync 返回 1 条。

### TC-MSG-03-09 本地待发消息与 sync 回来的同 clientMsgId 消息合并，不重复
- 关联 AC：AC-MSG-03-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层；alice 本地发送 M（clientMsgId=X），未收到 ack。
- 步骤：
  1. 假传输层断开并重连；SDK 发起 sync，返回包含 clientMsgId=X、msgId=M1、seq=3 的消息。
  2. 随后假传输层回 X 的 ack（msgId=M1, seq=3）。
- 预期结果：
  1. 列表中只有 1 条 M，状态为"已发送"、seq=3。
  2. 第 2 步不新增条目。

### TC-MSG-03-10 有发送失败的消息时刷新，弹出离开确认
- 关联 AC：AC-MSG-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：按 TC-MSG-03-01 制造一条"发送失败"的消息。
- 步骤：
  1. 注册 `page.on('dialog')` 监听；执行 `page.reload()`。
  2. 选择"取消"。
- 预期结果：
  1. 触发 `beforeunload` 类型的对话框。
  2. 取消后页面不刷新，失败消息仍在。

### TC-MSG-03-11 有待发消息时刷新弹确认；没有待发 / 失败消息时不弹
- 关联 AC：AC-MSG-03-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在聊天页。
- 步骤：
  1. 离线状态下发送一条消息（发送中、在待发队列），执行 reload。
  2. 恢复网络，消息变为已发送后，执行 reload。
- 预期结果：
  1. 第 1 步弹出 `beforeunload` 确认。
  2. 第 2 步不弹窗，直接刷新，刷新后停留在原聊天页并重新加载最新 20 条。

### TC-MSG-03-12 超时标记失败后 ack 才到达：以 ack 为准变为已发送
- 关联 AC：AC-MSG-03-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 在线；alice 页面用 Playwright `routeWebSocket` 代理连接，把下一条 `send` 的 ack 延迟 12 秒再转发给页面（服务端正常保存并投递）。
- 步骤：
  1. alice 发送"迟到的 ack"；等待约 10 秒，确认变为"发送失败"。
  2. 不点重发，等待 ack 到达（第 12 秒）。
- 预期结果：
  1. 第 10 秒（±1 秒）显示"发送失败"。
  2. ack 到达后变为"已发送"，重发入口消失，按 ack 的 seq 排在正确位置（C-30）。
  3. bob 只收到 1 条；刷新前不再弹出 `beforeunload` 确认（已无失败消息）。

## F-MSG-04 消息不丢、不重、有序

### TC-MSG-04-01 同一 clientMsgId 重复提交 5 次只保存 1 条
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：alice 用 `cid-dup-0001` 顺序提交同一条消息 5 次；bob sync afterSeq=0。
- 预期结果：
  1. 5 次 ack 的 msgId、seq、ts、convId 完全相同，seq=1。
  2. bob 只收到 1 个 `msg` 帧；sync 返回 1 条。

### TC-MSG-04-02 同一 clientMsgId 内容不同：以第一次为准
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：
  1. alice 用 `cid-dup-0002` 发送"第一次"。
  2. alice 用 `cid-dup-0002` 发送"第二次"。
  3. bob sync afterSeq=0。
- 预期结果：
  1. 两次 ack 的 msgId、seq 相同。
  2. sync 只有 1 条，body.text = "第一次"；bob 没有收到内容为"第二次"的 `msg`。

### TC-MSG-04-03 同一 clientMsgId 并发提交 5 次只保存 1 条
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线（单连接），bob 在线。
- 步骤：alice 不等 ack，连续写出 5 个 `send` 帧（id r1～r5，clientMsgId 都是 `cid-race-0001`）。
- 预期结果：
  1. 收到 5 个 ack，msgId、seq 全部相同。
  2. bob 只收到 1 个 `msg`；sync 返回 1 条。

### TC-MSG-04-04 不同发送者使用相同 clientMsgId 互不影响
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：
  1. alice 用 `cid-shared-01` 给 bob 发"A"。
  2. bob 用 `cid-shared-01` 给 alice 发"B"。
- 预期结果：
  1. 两条都成功，seq 分别为 1、2，msgId 不同（去重键是"发送者 + clientMsgId"）。
  2. 不返回 4008。

### TC-MSG-04-05 同一用户两个连接用相同 clientMsgId 发到同一会话，只保存 1 条
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 web、pc 两个连接在线；bob 在线。
- 步骤：
  1. alice(web) 用 `cid-multi-01` 发"x"。
  2. alice(pc) 用 `cid-multi-01` 发"x"。
- 预期结果：
  1. 两个 ack 的 msgId、seq 相同。
  2. bob 只收到 1 个 `msg`；alice(pc) 收到 alice(web) 那条的 `msg` 1 次（之后不会再收到重复帧）。

### TC-MSG-04-06 同一 clientMsgId 发到另一个单聊 / 群返回 4008
- 关联 AC：AC-MSG-04-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：群 g1 成员为 alice、bob；alice、carol 在线。
- 步骤：
  1. alice 用 `cid-conflict-1` 给 bob 发消息，成功。
  2. alice 用 `cid-conflict-1` 给 carol 发消息。
  3. alice 用 `cid-conflict-1` 给群 g1 发消息。
  4. carol sync `c2c:alice:carol` afterSeq=0；bob sync `group:g1` afterSeq=0。
- 预期结果：
  1. 第 2、3 步 ack.error.code = 4008。
  2. carol 未收到 `msg`；两个 sync 都返回 0 条。

### TC-MSG-04-07 收到 1、3，500ms 内无 2：发起 sync，最终展示 1、2、3
- 关联 AC：AC-MSG-04-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟；会话本地 maxSeq=0。
- 步骤：
  1. 推送 seq=1，再推送 seq=3。
  2. 假时钟前进 499ms，检查是否发出 sync。
  3. 再前进 1ms（共 500ms），检查发出的帧。
  4. 假传输层对 sync 回应 `{messages:[seq=2], hasMore:false}`。
- 预期结果：
  1. 499ms 时未发出 sync。
  2. 500ms 时发出 `sync {convId, afterSeq:1}`（只补缺口，limit 不小于 1）。
  3. 最终列表为 1、2、3，无重复。

### TC-MSG-04-08 跳过 seq=2 的实时投递（故障注入）：界面最终展示 1、2、3
- 关联 AC：AC-MSG-04-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 在页面打开与 alice 的聊天页；alice 用测试 WS 客户端发送；注入 `skip-delivery {clientMsgId:"cid-gap-0002"}`。
- 步骤：alice 依次发送 cid-gap-0001（"一"）、cid-gap-0002（"二"）、cid-gap-0003（"三"）。
- 预期结果：
  1. bob 实时只收到"一""三"；约 500ms 后 bob 发出 `sync {afterSeq:1}`。
  2. 2 秒内 bob 页面按"一""二""三"顺序显示，没有重复。

### TC-MSG-04-09 本地 maxSeq=0 时先收到 seq=3：补拉 1、2
- 关联 AC：AC-MSG-04-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟；会话本地无消息（maxSeq=0）。
- 步骤：推送 seq=3；假时钟前进 500ms；对 sync 回应 seq 1、2。
- 预期结果：发出 `sync {afterSeq:0}`；最终列表为 1、2、3。

### TC-MSG-04-10 sync 结果与迟到的 msg 帧重叠时按 msgId 去重
- 关联 AC：AC-MSG-04-3
- 优先级：P0
- 层级：SDK 单测
- 前置条件：同 TC-MSG-04-07，已发出 sync。
- 步骤：
  1. sync 尚未返回时，seq=2 的 `msg` 帧迟到到达。
  2. sync 返回 seq=2（同一 msgId）。
- 预期结果：列表中 seq=2 只出现 1 次；列表为 1、2、3。

### TC-MSG-04-11 收到 1、3，200ms 后 2 到达，不发起 sync
- 关联 AC：AC-MSG-04-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟。
- 步骤：推送 seq=1、seq=3；前进 200ms 后推送 seq=2；再前进 1000ms。
- 预期结果：整个过程没有发出 `sync` 帧；列表为 1、2、3。

### TC-MSG-04-12 ack 带回的 seq 计入本地 maxSeq，不误判缺口
- 关联 AC：AC-MSG-04-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层、假时钟；本地已收到 bob 发的 seq=1。
- 步骤：
  1. alice 发送 M，假传输层回 ack seq=2。
  2. 推送 bob 的 seq=3。
  3. 前进 1000ms。
- 预期结果：本地 maxSeq 依次为 1→2→3；没有发出 `sync`。

### TC-MSG-04-13 5 人群内各并发 20 条：所有成员 10 秒内看到 seq 1～100
- 关联 AC：AC-MSG-04-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：群 busy 成员 s0～s4，全部在线（每人一个连接）。
- 步骤：
  1. 5 个成员并发，每人连续发出 20 条（clientMsgId `cid-{i}-{j}`）。
  2. 记录最后一个 ack 的时间 T。
  3. 每个成员把"自己收到的 ack + 收到的 msg 帧"按 seq 合并；在 T+10s 时检查；之后每人执行 sync afterSeq=0 limit=200 再检查一次。
- 预期结果：
  1. 100 个 ack 全部成功（每人每秒正好 20 条，不触发 4290；若发送超过 1 秒窗口导致 4290，用例视为环境问题重跑）。
  2. T+10s 时每个成员的合并结果都是 seq 1～100，连续、不重复，共 100 条。
  3. sync 结果为 100 条，seq 1～100 连续。

### TC-MSG-04-14 返回 ack 后 kill -9，重启后 sync 能拉到该消息
- 关联 AC：AC-MSG-04-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：服务端以独立进程启动（SQLite 文件库，`synchronous=FULL`）；alice 在线。
- 步骤：
  1. alice 发送 `cid-kill-0001`"持久化"，收到 ack（seq=S，msgId=M）后立刻对服务端进程发 SIGKILL。
  2. 用同一数据库文件重启服务端。
  3. bob 连接并 sync `c2c:alice:bob` afterSeq=0。
  4. alice 重连后用 `cid-kill-0001` 再发一次。
- 预期结果：
  1. sync 返回该消息，msgId=M、seq=S、text="持久化"。
  2. 第 4 步返回同一 msgId / seq（去重记录同样已落盘）。
  3. 重复 20 次，全部通过。

### TC-MSG-04-15 msg 帧和 sync 结果中每条消息都带 clientMsgId
- 关联 AC：AC-MSG-04-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：群 g1 成员 alice、bob；alice 以 web、pc 在线，bob 在线。
- 步骤：
  1. alice(web) 给 bob 发单聊（`cid-c2c-0001`），给 g1 发群消息（`cid-grp-0001`）。
  2. 收集 bob、alice(pc) 收到的全部 `msg` 帧。
  3. bob 对两个会话分别执行 afterSeq=0 和 beforeSeq=100 的 sync。
- 预期结果：所有 `msg` 帧和 sync 结果中的每条消息都包含非空 `clientMsgId`，且等于发送时的值。

### TC-MSG-04-16 clientMsgId 长度 7 / 65 返回 4000，8 / 64 成功
- 关联 AC：AC-MSG-04-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：alice 分别用长度为 7（`cid-007`）、8（`cid-0008`）、64、65 的 clientMsgId 发消息；以及不带 clientMsgId 发送。
- 预期结果：长度 7、65 和缺失时返回 4000，不保存（C-31）；长度 8、64 成功，seq 分别为 1、2。

## F-MSG-05 历史消息与补拉

### TC-MSG-05-01 bob 离线期间收到 5 条，上线打开会话看到全部 5 条
- 关联 AC：AC-MSG-05-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 未登录；alice 已登录。
- 步骤：
  1. alice 给 bob 发 m1～m5。
  2. bob 登录，打开与 alice 的会话。
- 预期结果：会话列表中该会话未读 5；打开后按顺序显示 m1～m5（seq 1～5），无重复。

### TC-MSG-05-02 100 条消息：打开显示 81～100，翻页到 1 后显示"没有更多了"
- 关联 AC：AC-MSG-05-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：`c2c:alice:bob` 已有 100 条消息（内容 m1～m100，分 5 秒发送以避开限流）。
- 步骤：
  1. bob 打开该会话；记录 WS 发出的 sync 帧。
  2. 滚动到顶部，等待加载；重复 4 次。
  3. 第 5 次滚动到顶部。
- 预期结果：
  1. 打开时发出 `sync {beforeSeq:101, limit:20}`，显示 m81～m100。
  2. 4 次翻页依次加载 m61～m80、m41～m60、m21～m40、m1～m20，每次 `beforeSeq` 为当前最小 seq。
  3. 加载到 m1 后顶部显示"没有更多了"；第 5 次滚动不再发出 sync。

### TC-MSG-05-03 历史加载失败显示"加载失败，点击重试"，重试成功
- 关联 AC：AC-MSG-05-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同 TC-MSG-05-02；使用 Playwright `routeWebSocket` 拦截，对第 1 次翻页的 sync 回 `ack.error {code:5000}`。
- 步骤：打开会话，滚动到顶部；看到失败提示后点击。
- 预期结果：顶部显示"加载失败，点击重试"；点击后重新发出同一个 beforeSeq 的 sync，加载出 m61～m80。

### TC-MSG-05-04 sync afterSeq / beforeSeq 结果升序、hasMore 正确
- 关联 AC：AC-MSG-05-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 100 条消息；bob 在线。
- 步骤：bob 依次执行：
  1. `{afterSeq:0, limit:3}`
  2. `{beforeSeq:101, limit:20}`
  3. `{beforeSeq:21, limit:20}`
  4. `{afterSeq:100}`
  5. `{beforeSeq:1, limit:20}`
- 预期结果：
  1. seq 1～3，hasMore=true。
  2. seq 81～100（升序），hasMore=true。
  3. seq 1～20，hasMore=false。
  4. 空数组，hasMore=false。
  5. 空数组，hasMore=false。

### TC-MSG-05-05 limit 默认 50；limit=200 上限；limit=201 返回 4000
- 关联 AC：AC-MSG-05-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：会话有 250 条消息。
- 步骤：
  1. `{afterSeq:0}`（不传 limit）
  2. `{afterSeq:0, limit:200}`
  3. `{afterSeq:200, limit:200}`
  4. `{afterSeq:0, limit:201}`
  5. `{afterSeq:0, limit:1}`
- 预期结果：
  1. 50 条（seq 1～50），hasMore=true。
  2. 200 条（seq 1～200），hasMore=true。
  3. 50 条（seq 201～250），hasMore=false。
  4. ack.error.code = 4000。
  5. 1 条（seq 1），hasMore=true。

### TC-MSG-05-06 limit=0 / 负数、afterSeq 与 beforeSeq 同时传返回 4000
- 关联 AC：AC-MSG-05-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：会话有 10 条消息。
- 步骤：
  1. `{afterSeq:0, limit:0}`
  2. `{afterSeq:0, limit:-1}`
  3. `{afterSeq:0, beforeSeq:5}`
  4. `{limit:10}`（afterSeq、beforeSeq 都不传）
- 预期结果：每个 ack.error.code = 4000（C-32）。

### TC-MSG-05-07 拉取别人的单聊 4032、非成员拉群 4033、不存在的群 4043
- 关联 AC：AC-MSG-05-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 1 条消息；群 g1 成员为 alice、bob；群 g404 不存在；carol 在线。
- 步骤：carol 分别 sync：
  1. `c2c:alice:bob` afterSeq=0
  2. `group:g1` afterSeq=0
  3. `group:g404` afterSeq=0
  4. `group:g1` beforeSeq=10
- 预期结果：依次返回 4032、4033、4043、4033；均不返回消息数据。

### TC-MSG-05-08 会话 ID 不规范（顺序颠倒、格式错误）返回 4007
- 关联 AC：AC-MSG-05-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线。
- 步骤：alice sync `c2c:bob:alice`、`c2c:alice`、`foo:alice:bob`、`group:`。
- 预期结果：每个返回 4007（INVALID_CONV_ID），而不是 4032 / 4033。

### TC-MSG-05-09 加载历史后当前可视位置不跳动
- 关联 AC：AC-MSG-05-6
- 优先级：P0
- 层级：UI 自动化
- 前置条件：会话有 100 条消息；bob 打开会话，视口 375×812。
- 步骤：
  1. 滚动到顶部，在触发加载前记录 seq=81 消息元素相对视口的 `top` 值 Y0。
  2. 等待 61～80 渲染完成后，再次记录 seq=81 元素的 `top` 值 Y1。
- 预期结果：|Y1 − Y0| ≤ 2px；seq=81 仍在可视区域内，没有跳到底部或顶部。

### TC-MSG-05-10 sync 200 条 × 5000 字（约 3MB 下行帧）正常返回，连接不断开
- 关联 AC：AC-MSG-05-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：`c2c:alice:bob` 有 200 条消息，每条为 5000 个"中"（每条 UTF-8 15000 字节）；bob 在线。
- 步骤：bob 执行 `sync {afterSeq:0, limit:200}`；之后发 ping。
- 预期结果：
  1. 收到一个完整的 ack，200 条消息，每条 5000 码点，hasMore=false。
  2. 连接没有以 1009 或其他关闭码断开，ping 正常收到 pong（下行不设帧大小上限，C-07）。

### TC-MSG-05-11 SDK 不限制下行帧大小，能解析约 3MB 的 sync 应答
- 关联 AC：AC-MSG-05-4
- 优先级：P0
- 层级：SDK 单测
- 前置条件：假传输层。
- 步骤：SDK 发起 sync 后，假传输层回一个约 3MB 的 ack（200 条 × 5000 字）。
- 预期结果：SDK 正常解析并把 200 条写入会话，不报错、不主动断开（C-07）；SDK 只对上行帧校验 64KB 上限（发送 70KB 的帧会在本地拒绝）。

## F-MSG-06 文本展示

### TC-MSG-06-01 HTML / 脚本内容原样显示为文本，不执行
- 关联 AC：AC-MSG-06-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice、bob 两个上下文在线；两边都注册 `page.on('dialog')` 计数。
- 步骤：alice 依次发送 `<img src=x onerror=alert(1)>`、`<script>alert(2)</script>`、`<b>粗体</b>`、`&lt;`。
- 预期结果：
  1. 双方气泡中按原文逐字显示 4 条文本（`textContent` 与发送内容完全一致）。
  2. 两个页面都没有弹窗（dialog 计数为 0），DOM 中不存在由消息产生的 `img`、`script`、`b` 元素。
  3. 会话列表摘要同样按纯文本显示。

### TC-MSG-06-02 3 行消息按 3 行显示；300 字符长串断行，无横向滚动条
- 关联 AC：AC-MSG-06-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 打开聊天页；分别在 375×812 和 1280×800 两种视口下执行。
- 步骤：
  1. alice 发送 `"第一行\n第二行\n第三行"`。
  2. alice 发送 `"https://example.com/" + "a"×280`（共 300 字符，无空格）。
- 预期结果：
  1. 第 1 条在气泡内显示为 3 行。
  2. 第 2 条在气泡内断行，气泡宽度不超过消息列表宽度。
  3. `document.documentElement.scrollWidth <= clientWidth`（无横向滚动条）。

### TC-MSG-06-03 PC 上 Enter 发送、Shift+Enter 换行
- 关联 AC：AC-MSG-06-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：1280×800 视口，alice 在聊天页。
- 步骤：输入"行1"，按 Shift+Enter，输入"行2"，按 Enter。
- 预期结果：只发出 1 条消息，内容为 `"行1\n行2"`，显示为 2 行；输入框清空。

### TC-MSG-06-04 时间分隔：间隔 4 分钟不显示、6 分钟显示；昨天显示"昨天 HH:mm"
- 关联 AC：AC-MSG-06-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：用 `routeWebSocket` 返回构造的会话数据（服务端 ts 可控）；浏览器时钟用 `page.clock` 固定为 2026-10-31 15:00:00（本地时区）。消息 ts：m1=2026-10-30 21:05、m2=10-31 14:00、m3=14:04、m4=14:10。
- 步骤：打开该会话。
- 预期结果：
  1. m1 上方显示"昨天 21:05"。
  2. m2 上方显示"14:00"。
  3. m3 上方不显示时间（与 m2 间隔 4 分钟）。
  4. m4 上方显示"14:10"（与 m3 间隔 6 分钟）。

### TC-MSG-06-05 时间格式边界：正好 5 分钟、今年、跨年、会话列表
- 关联 AC：AC-MSG-06-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：同 TC-MSG-06-04，浏览器时钟固定为 2026-10-31 15:00。
- 步骤：构造消息 ts：a=2025-12-31 23:59、b=2026-01-01 00:00、c=2026-03-05 08:07、d=10-31 14:00:00、e=10-31 14:05:00、f=10-31 14:10:01；打开会话；再看会话列表中以 c、d 为最后一条的两个会话。
- 预期结果：
  1. a（当前已加载的第一条）显示"2025-12-31 23:59"；b 与 a 只间隔 1 分钟，但跨天，显示"01-01 00:00"（C-33）。
  2. c 显示"03-05 08:07"。
  3. d 显示"14:00"；e 不显示（正好 5 分钟，不算"超过"）；f 显示"14:10"（间隔 5 分 1 秒）。
  4. 会话列表：最后消息为 d 的会话显示"14:00"；最后消息为 c 的显示"03-05 08:07"。

### TC-MSG-06-06 查看历史时收到 2 条新消息：显示"↓ 2 条新消息"，不自动滚动
- 关联 AC：AC-MSG-06-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：会话有 60 条消息；bob 打开会话并向上滚动，使距底部 > 100px（如 500px）。
- 步骤：
  1. 记录 scrollTop；alice 发送 2 条消息。
  2. 点击"↓ 2 条新消息"。
- 预期结果：
  1. 显示"↓ 2 条新消息"；scrollTop 不变。
  2. 点击后滚动到底部（距底部 ≤ 1px），最后 2 条可见，提示消失。

### TC-MSG-06-07 距底部 ≤100px 时新消息自动滚到底
- 关联 AC：AC-MSG-06-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：bob 打开会话，滚动到距底部 80px。
- 步骤：alice 发送 1 条消息；再把 bob 滚动到距底部 101px，alice 再发 1 条。
- 预期结果：
  1. 第 1 条到达后自动滚到底部，不显示"↓ N 条新消息"。
  2. 第 2 条到达后不自动滚动，显示"↓ 1 条新消息"。

### TC-MSG-06-08 iOS Safari 键盘弹起时输入框不被遮挡
- 关联 AC：AC-MSG-06-5
- 优先级：P0
- 层级：手工
- 前置条件：iPhone（最新两个大版本的 iOS Safari 各一台），登录 alice，打开与 bob 的聊天页。
- 步骤：
  1. 点击输入框弹出键盘。
  2. 输入 3 行文字；收起键盘再弹起；横竖屏各一次。
  3. 在微信内置浏览器重复。
- 预期结果：输入框和发送键始终完整显示在键盘上方；最新消息可见；页面不出现大片空白或错位。

### TC-MSG-06-09 跨天总显示时间；翻页后新的第一条显示时间，原第一条按间隔规则
- 关联 AC：AC-MSG-06-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：用测试模式服务端时钟（`POST /admin/test/clock`）造数据，浏览器时钟固定为 2026-10-31 15:00。会话共 21 条消息：seq 1 = 10-30 23:58，seq 2 = 10-31 00:01，seq 3～21 = 10-31 14:00～14:18 每分钟一条。
- 步骤：
  1. 打开会话（加载最新 20 条，即 seq 2～21）。
  2. 向上滚动加载 seq 1。
- 预期结果：
  1. 第 1 步：seq 2 是当前已加载的第一条，显示"00:01"；seq 3 与 seq 2 间隔超过 5 分钟，显示"14:00"；seq 4～21 不显示时间。
  2. 第 2 步：seq 1 成为第一条，显示"昨天 23:58"；seq 2 与 seq 1 只间隔 3 分钟但跨天，仍显示"00:01"（C-33）。

## F-MSG-07 发送频率限制

### TC-MSG-07-01 1 秒内发 25 条：前 20 条成功，其余 4290；1 秒后可继续
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线；新会话。
- 步骤：
  1. alice 在 200ms 内顺序发出 25 条（不同 clientMsgId）。
  2. 等待 1.1 秒，再发 1 条。
  3. bob sync afterSeq=0。
- 预期结果：
  1. 第 1～20 条成功，seq 1～20；第 21～25 条返回 4290，不分配 seq。
  2. 第 2 步成功，seq=21。
  3. sync 返回 21 条，seq 1～21 连续；bob 收到 21 个 `msg` 帧。

### TC-MSG-07-02 限流按用户所有连接合计
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 以 web、pc 两个连接在线。
- 步骤：同一秒内 alice(web) 发 10 条、alice(pc) 发 15 条（交替发出）。
- 预期结果：两端成功总数 = 20，4290 总数 = 5。

### TC-MSG-07-03 限流按用户隔离：两个用户各 20 条都成功
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线。
- 步骤：同一秒内 alice 给 carol 发 20 条，bob 给 carol 发 20 条。
- 预期结果：40 条全部成功，无 4290。

### TC-MSG-07-04 单聊和群消息共享同一个限流额度
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：群 g1 成员 alice、bob；alice 在线。
- 步骤：同一秒内 alice 给 bob 发 10 条单聊，再给 g1 发 15 条群消息。
- 预期结果：前 20 条成功（10 单聊 + 10 群），最后 5 条群消息返回 4290。

### TC-MSG-07-05 持续以 20 条/秒匀速发送 5 秒，全部成功
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线。
- 步骤：alice 每 50ms 发 1 条，共 100 条（5 秒）。
- 预期结果：100 条全部成功，seq 1～100；无 4290（令牌桶每秒补充 20 个，匀速 20 条/秒不会耗尽，C-29）。

### TC-MSG-07-06 收到 4290：标记失败并提示"发送太频繁，请稍后再试"
- 关联 AC：AC-MSG-07-2
- 优先级：P0
- 层级：UI 自动化
- 前置条件：alice 在页面打开与 bob 的聊天页；alice 另有一个测试 WS 连接。
- 步骤：测试连接在 200ms 内发 20 条，同一秒内在页面发送"太快了"。
- 预期结果：
  1. "太快了"标记为"发送失败"，提示"发送太频繁，请稍后再试"。
  2. SDK 不自动重试（1.5 秒内没有第二个 `send` 帧）；之后可手动重发成功（见 TC-MSG-03-05）。

### TC-MSG-07-07 被 4290 拒绝的请求不消耗令牌
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线，空闲 3 秒以上（桶满 20）。
- 步骤：
  1. alice 在 100ms 内发 20 条（耗尽令牌）。
  2. 紧接着以每 10ms 一条的速度持续发送 1 秒（约 100 次请求）。
- 预期结果：
  1. 第 1 步 20 条全部成功。
  2. 第 2 步的 1 秒内成功约 20 条（18～22 条，令牌按每秒 20 个补充），其余返回 4290；如果被拒绝的请求也消耗令牌，成功数会接近 0（C-29）。

### TC-MSG-07-08 去重命中的重发不消耗令牌、桶空时也返回原 ack
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线，桶满。
- 步骤：
  1. alice 在 100ms 内发 20 条（cid-rl-01～cid-rl-20），耗尽令牌。
  2. 立即用 cid-rl-01 重发 5 次。
  3. 等待 100ms（补充约 2 个令牌），发 2 条新消息。
- 预期结果：
  1. 第 2 步 5 次都返回 cid-rl-01 第一次的 msgId 和 seq=1，不返回 4290（去重命中不计数，C-29）。
  2. 第 3 步 2 条新消息成功，seq 为 21、22（重发没有吃掉补充的令牌）。

### TC-MSG-07-09 令牌桶容量 20：空闲 3 秒后突发 25 条只过 20 条，0.5 秒后补充 10 个
- 关联 AC：AC-MSG-07-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice 在线，空闲 3 秒（若按每秒 20 个补充且无上限，会累积 60 个）。
- 步骤：
  1. 100ms 内发 25 条。
  2. 等待 500ms，100ms 内发 15 条。
- 预期结果：
  1. 第 1 步成功 20 条、4290 共 5 条（容量上限 20）。
  2. 第 2 步成功约 10 条（9～11 条），其余返回 4290。

---

## 覆盖检查

| AC | 覆盖用例 |
|---|---|
| AC-MSG-01-1 | TC-MSG-01-01 |
| AC-MSG-01-2 | TC-MSG-01-02、TC-MSG-01-16、TC-MSG-01-18 |
| AC-MSG-01-3 | TC-MSG-01-03、TC-MSG-01-04、TC-MSG-01-05、TC-MSG-01-06 |
| AC-MSG-01-4 | TC-MSG-01-07、TC-MSG-01-08、TC-MSG-01-17 |
| AC-MSG-01-5 | TC-MSG-01-09 |
| AC-MSG-01-6 | TC-MSG-01-10、TC-MSG-01-11 |
| AC-MSG-01-7 | TC-MSG-01-12 |
| AC-MSG-01-8 | TC-MSG-01-13、TC-MSG-01-14 |
| AC-MSG-01-9 | TC-MSG-01-15 |
| AC-MSG-02-1 | TC-MSG-02-01、TC-MSG-02-02 |
| AC-MSG-02-2 | TC-MSG-02-03 |
| AC-MSG-02-3 | TC-MSG-02-04 |
| AC-MSG-02-4 | TC-MSG-02-05、TC-MSG-02-06 |
| AC-MSG-03-1 | TC-MSG-03-01、TC-MSG-03-02、TC-MSG-03-12 |
| AC-MSG-03-2 | TC-MSG-03-03、TC-MSG-03-04 |
| AC-MSG-03-3 | TC-MSG-03-05、TC-MSG-03-06 |
| AC-MSG-03-4 | TC-MSG-03-07、TC-MSG-03-08、TC-MSG-03-09 |
| AC-MSG-03-5 | TC-MSG-03-10、TC-MSG-03-11 |
| AC-MSG-04-1 | TC-MSG-04-01、TC-MSG-04-02、TC-MSG-04-03、TC-MSG-04-04、TC-MSG-04-05、TC-MSG-04-16 |
| AC-MSG-04-2 | TC-MSG-04-06 |
| AC-MSG-04-3 | TC-MSG-04-07、TC-MSG-04-08、TC-MSG-04-09、TC-MSG-04-10 |
| AC-MSG-04-4 | TC-MSG-04-11、TC-MSG-04-12 |
| AC-MSG-04-5 | TC-MSG-04-13 |
| AC-MSG-04-6 | TC-MSG-04-14 |
| AC-MSG-04-7 | TC-MSG-04-15 |
| AC-MSG-05-1 | TC-MSG-05-01 |
| AC-MSG-05-2 | TC-MSG-05-02、TC-MSG-05-03 |
| AC-MSG-05-3 | TC-MSG-05-04 |
| AC-MSG-05-4 | TC-MSG-05-05、TC-MSG-05-06、TC-MSG-05-10、TC-MSG-05-11 |
| AC-MSG-05-5 | TC-MSG-05-07、TC-MSG-05-08 |
| AC-MSG-05-6 | TC-MSG-05-09 |
| AC-MSG-06-1 | TC-MSG-06-01 |
| AC-MSG-06-2 | TC-MSG-06-02、TC-MSG-06-03 |
| AC-MSG-06-3 | TC-MSG-06-04、TC-MSG-06-05、TC-MSG-06-09 |
| AC-MSG-06-4 | TC-MSG-06-06、TC-MSG-06-07 |
| AC-MSG-06-5 | TC-MSG-06-08 |
| AC-MSG-07-1 | TC-MSG-07-01、TC-MSG-07-02、TC-MSG-07-03、TC-MSG-07-04、TC-MSG-07-05、TC-MSG-07-07、TC-MSG-07-08、TC-MSG-07-09 |
| AC-MSG-07-2 | TC-MSG-07-06 |

共 38 条 AC，全部覆盖。

## 已澄清

| 原问题 | 结论 | 已更新 / 新增用例 |
|---|---|---|
| Q1 空白的定义 | C-26：以 Unicode `White_Space`（25 个字符）为准，U+200B、U+FEFF 不算；服务端与 SDK 共用字符表和测试数据 | 新增 TC-MSG-01-16、TC-MSG-01-17、TC-MSG-01-18 |
| Q2 超过 5000 字的交互 | C-28：输入框下方实时显示"已超出 N 字"，发送按钮置灰 | 更新 TC-MSG-01-03、TC-MSG-01-04、TC-MSG-01-05 |
| Q3 限流窗口与计数 | C-29：令牌桶（容量 20、每秒补 20）；4290 拒绝的请求和去重命中的重发都不计数 | 更新 TC-MSG-07-05；新增 TC-MSG-07-07、TC-MSG-07-08、TC-MSG-07-09 |
| Q4 超时后 ack 才到达 | C-30：以 ack 为准，变为"已发送" | 更新 TC-MSG-03-02；新增 TC-MSG-03-12 |
| Q5 clientMsgId 长度越界 | C-31：返回 4000 | 新增 TC-MSG-04-16 |
| Q6 sync 参数边界 | C-32：limit ≤ 0、afterSeq / beforeSeq 同时出现或都不出现时返回 4000 | 更新 TC-MSG-05-06 |
| Q7 ZWJ / 国旗计数 | C-27：按码点计数是预期行为 | 更新 TC-MSG-01-06 |
| Q8 时间分隔的第一条与跨天 | C-33：当前已加载的第一条总是显示时间；跨天总是显示时间 | 更新 TC-MSG-06-05；新增 TC-MSG-06-09 |
| （v1.1 新增）下行帧大小 | C-07：64KB 只限上行，下行不设上限，SDK 不得限制下行 | 新增 TC-MSG-05-10、TC-MSG-05-11 |

## 疑问

- Q9（去重重发与空桶）：C-29 说去重命中的重发"不计数"。TC-MSG-07-08 按"桶空时也直接返回原 ack，不返回 4290"来判定，需要后端确认实现顺序是先查重、再扣令牌。
- Q10（令牌桶初始状态）：新连接 / 新用户的桶是满的（20 个）还是空的？TC-MSG-07-09 按"空闲后桶满 20"判定；同一用户新建连接不应重置桶（限流按用户合计）。
