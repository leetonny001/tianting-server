# 测试用例：引用（QUO，C4）

> 依据：[PRD-chat v1.5](../prd/PRD-chat-v1.5.md)（冻结）第 4.3 节第 7 步、第 8 节 F-QUO-01～02、第 11、13 节的 C4 部分。任务单：[backend-chat](../tasks/backend-chat.md) BE-470、BE-475、BE-478、BE-480，[frontend-chat](../tasks/frontend-chat.md) FE-660～FE-672。
> 范围：标了【C4】的 AC-QUO。
>
> 通用约定：环境、层级、时钟、哨兵法同 [TC-RCL](TC-RCL.md)（**E1** 是 `IM_TEST_MODE=1`、SQLite、`IM_MSG_ACTIONS_ENABLED=1`）。**CI 现在不运行（PENDING P-21）**，结果都是本地结果。特征串：`ZXQ-QUO-1`、`ZXQ-QUO-2`。"搜不到"指对原始字节或完整响应做子串搜索。


| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-QUO-01-01 | 引用回复：对方收到带 quote 的 msg，state=normal | AC-QUO-01-1 | 端到端 + 后端 e2e | P0 |
| TC-QUO-01-02 | 不可引用的 6 种：都是 4040 quoteUnavailable，响应相同，消息未保存 | AC-QUO-01-2 | 后端 e2e | P0 |
| TC-QUO-01-03 | image 带 quote：4000 | AC-QUO-01-3 | 后端 e2e | P0 |
| TC-QUO-01-04 | 同一 clientMsgId 重发：返回第一次的 ack（被引用的消息已撤回） | AC-QUO-01-4 | 后端 e2e | P0 |
| TC-QUO-01-05 | 选"引用"：输入框上方引用条，可以取消；发送后气泡顶部显示 | AC-QUO-01-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-QUO-01-06 | messages.body 只有 quote_msg_id，搜不到被引用的原文 | AC-QUO-01-6 | 后端 e2e | P0 |
| TC-QUO-01-07 | 引用链不展开；图片、文件摘要；msgId 格式错 4000 | AC-QUO-01-7 | 后端 e2e | P0 |
| TC-QUO-02-01 | 被引用的撤回、后台删除后：sync 里 state 变了，没有原文 | AC-QUO-02-1 | 后端 e2e | P0 |
| TC-QUO-02-02 | 收到被引用消息的 msgDeleted：本地引用里搜不到原文（三套 SDK） | AC-QUO-02-2 | SDK 单测（三套） | P0 |
| TC-QUO-02-03 | 点击引用跳转：100 条以内翻页高亮，500 条提示太早 | AC-QUO-02-3 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-QUO-02-04 | historyVisible=false 的新成员：能看到引用摘要，拉不到原消息 | AC-QUO-02-4 | 后端 e2e | P0 |
| TC-QUO-02-05 | 引用摘要含 HTML：按纯文本显示 | AC-QUO-02-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-QUO-02-06 | m1 不在已加载范围、离线期间被撤回：重连后 m2 的引用原文被清（三套 SDK） | AC-QUO-02-6 | SDK 单测（三套）+ 共享向量 | P0 |
| TC-QUO-02-07 | 摘要截断 100 码点；图片；过期文件没有文件名 | AC-QUO-02-7 | 后端 e2e | P0 |

---

## F-QUO-01 引用发送

### TC-QUO-01-01 引用回复：对方收到带 quote 的 msg，state=normal
- 关联 AC：AC-QUO-01-1
- 优先级：P0
- 批次：C4
- 层级：端到端 + 后端 e2e
- 环境：E1 + 三端客户端
- 前置条件：u2 给 u1 发 m1（文本）
- 步骤：
  1. u1 发 `send {type:text, text, quote:{msgId:m1}}`（三端轮换发送方）
  2. u2 收
- 预期结果：
  1. u2 收到的 `msg.body.quote = {msgId:m1, seq, from:u2, type:text, state:normal, text:<m1 前 100 个码点>}`
  2. 三端气泡顶部都显示引用内容

### TC-QUO-01-02 不可引用的 6 种：都是 4040 quoteUnavailable，响应相同，消息未保存
- 关联 AC：AC-QUO-01-2
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：群 g1：其他会话的 msgId、不存在的、已撤回的、后台删除的、`tip`、D 入群前的（`historyVisible=false`）
- 步骤：
  1. 分别作为 `quote.msgId` 发送
  2. 比较 6 个错误响应
  3. 查 `messages`
- 预期结果：
  1. 都是 4040，`reason=quoteUnavailable`
  2. 6 个响应除 `id` 外逐字节相同
  3. 没有新消息行，seq 没有前进

### TC-QUO-01-03 image 带 quote：4000
- 关联 AC：AC-QUO-01-3
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 有一个 `ready` 的图片 `fileId`
- 步骤：
  1. 发 `type=image` 带 `quote`
- 预期结果：
  1. 4000，文件仍是 `ready`

### TC-QUO-01-04 同一 clientMsgId 重发：返回第一次的 ack（被引用的消息已撤回）
- 关联 AC：AC-QUO-01-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：u1 引用 m1 发出 m2（`clientMsgId=c`），之后 m1 被撤回
- 步骤：
  1. 用 `clientMsgId=c` 原样重发
- 预期结果：
  1. ack 和第一次相同（`msgId`、`seq`），不是 4040

### TC-QUO-01-05 选"引用"：输入框上方引用条，可以取消；发送后气泡顶部显示
- 关联 AC：AC-QUO-01-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：会话里有 u2 的文本、图片、文件各一条
- 步骤：
  1. 长按（H5 宽屏右键、键盘菜单键）文本选"引用"，再点取消
  2. 再次引用，输入回复后发送
  3. 分别引用图片和文件发送
- 预期结果：
  1. 引用条出现、取消后消失、没有发出 `quote`
  2. 气泡顶部显示原文摘要
  3. 图片显示"[图片]"，文件显示"[文件] {文件名}"

### TC-QUO-01-06 messages.body 只有 quote_msg_id，搜不到被引用的原文
- 关联 AC：AC-QUO-01-6
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：m1 含 `ZXQ-QUO-2`
- 步骤：
  1. m2 引用 m1
  2. 查 m2 行的 `body` 原始值和 `quote_msg_id`；检查点后搜库文件
- 预期结果：
  1. `quote_msg_id=m1`，`body` 里没有被引用的内容
  2. 库文件里 `ZXQ-QUO-2` 只出现在 m1 的行（不在 m2）

### TC-QUO-01-07 引用链不展开；图片、文件摘要；msgId 格式错 4000
- 关联 AC：AC-QUO-01-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：m2 引用 m1，m3 引用 m2；图片消息 i、文件消息 f
- 步骤：
  1. 发 m3 后看下发
  2. 引用 i、引用 f 后看下发
  3. `quote.msgId` 填 `"../x"`
- 预期结果：
  1. m3 的 `quote.text` 是 m2 自己的文字
  2. "[图片]"；"[文件] {文件名}"
  3. 4000

## F-QUO-02 引用的展示

### TC-QUO-02-01 被引用的撤回、后台删除后：sync 里 state 变了，没有原文
- 关联 AC：AC-QUO-02-1
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：m2 引用 m1（含 `ZXQ-QUO-1`）；m4 引用 m3
- 步骤：
  1. 撤回 m1 后 `sync` m2
  2. 后台删除 m3 后 `sync` m4
- 预期结果：
  1. m2 的 `quote.state=recalled`，没有 `text`，整个响应搜不到 `ZXQ-QUO-1`
  2. m4 的 `quote.state=deleted`

### TC-QUO-02-02 收到被引用消息的 msgDeleted：本地引用里搜不到原文（三套 SDK）
- 关联 AC：AC-QUO-02-2
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）
- 环境：SDK 单测
- 前置条件：本地（内存，原生还有本地库）有 m1 和引用它的 m2
- 步骤：
  1. 推 m1 的 `msgDeleted`（带 `recalled`）
  2. 读内存和本地库
- 预期结果：
  1. m2 的 `quote.state=recalled`、没有 `text`，搜不到特征串
  2. 展示数据是"引用内容已被撤回"

### TC-QUO-02-03 点击引用跳转：100 条以内翻页高亮，500 条提示太早
- 关联 AC：AC-QUO-02-3
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：会话有 700 条；已加载最近 50 条；q1 引用距已加载范围 100 条的消息，q2 引用 500 条外的
- 步骤：
  1. 点 q1 的引用条
  2. 点 q2 的引用条
- 预期结果：
  1. 翻页加载后滚动到原消息并高亮
  2. 提示"消息太早，请翻页查看"，已加载范围仍然连续（中间没有空洞）

### TC-QUO-02-04 historyVisible=false 的新成员：能看到引用摘要，拉不到原消息
- 关联 AC：AC-QUO-02-4
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1
- 前置条件：群 `historyVisible=false`，m1 发在 D 入群之前
- 步骤：
  1. D 入群后 u1 引用 m1 发 m2
  2. D 收 m2
  3. D `sync` 到 m1 的 seq 范围
- 预期结果：
  1. m2 带 `quote.text`（F-QUO-02 规则 6 的已知限制）
  2. D 拉不到 m1

### TC-QUO-02-05 引用摘要含 HTML：按纯文本显示
- 关联 AC：AC-QUO-02-5
- 优先级：P0
- 批次：C4
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：Playwright（H5）+ 模拟器（Android、iOS）
- 前置条件：m1 的文字是 `<img src=x onerror=alert(1)><b>粗</b>`
- 步骤：
  1. 引用 m1 发送，两边查看
- 预期结果：
  1. 引用条和气泡按原样显示这串字符
  2. H5 没有生成 `img`、`b` 元素，没有弹窗、没有请求

### TC-QUO-02-06 m1 不在已加载范围、离线期间被撤回：重连后 m2 的引用原文被清（三套 SDK）
- 关联 AC：AC-QUO-02-6
- 优先级：P0
- 批次：C4
- 层级：SDK 单测（三套）+ 共享向量
- 环境：SDK 单测
- 前置条件：u2 本地有 m2（引用 m1，带特征串），m1 不在已加载范围
- 步骤：
  1. 离线期间 m1 被撤回（会话 `modVer` 增加）
  2. 重连，按 `modVer` 重拉已加载范围
- 预期结果：
  1. m2 的 `quote.state=recalled`，内存和本地库搜不到特征串
  2. `chat-sync.json` 的同名用例通过

### TC-QUO-02-07 摘要截断 100 码点；图片；过期文件没有文件名
- 关联 AC：AC-QUO-02-7
- 优先级：P0
- 批次：C4
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：m1 是 150 个码点的文本（含 4 字节字符）；图片 i；文件 f 已按 [IM 钟] 过期并 `file_purge`
- 步骤：
  1. 分别引用后看下发
- 预期结果：
  1. `text` 正好是前 100 个码点（不截断半个字符）
  2. "[图片]"
  3. `state=expired`，没有 `text`（没有文件名）

---

## 说明

- 用例编号按 AC 编号对齐（TC-QUO-0X-YY 对应 AC-QUO-0X-Y）。
- TC-QUO-02-03 的"200 条以内"按 seq 差判断（frontend-chat 第 20 条）。

