# 测试用例：客户端 UI（UI，第二批与 P1）

> 2026-11-05：依据升为 PRD-ui v1.1。AC-UI-44-1～3、AC-UI-90-2 不变；F-UI-44 规则 2、3 的回放展示和未查看由 TC-PUSH-12-* 覆盖，TC-UI-44-04 的“列表上限 100”按 `ts` 取最新的 100 条。

> 依据：[PRD-ui v1.0](../prd/PRD-ui-v1.0.md)（冻结），本文件覆盖【第二批】和【P1】AC；数据与规则见 [PRD-core v1.3](../prd/PRD-core-v1.3.md)、[PRD-social v1.0](../prd/PRD-social-v1.2.md)、[PRD-admin v1.0](../prd/PRD-admin-v1.0.md)；SDK 行为见 [SDK 设计说明](../sdk/README.md)；任务见 [frontend-ui.md](../tasks/frontend-ui.md)。
>
> 通用约定：
> - 测试用户同 PRD-ui 2.1 节：alice、bob、carol、dave、eve 已导入，昵称与 userId 相同（另有说明除外）；`zhangsan` 昵称"张三"；`nobody` 不存在；frank 已导入但与 alice 没有任何关系。"开放模式 / 好友制"同 PRD-social 第 6 节，默认开放模式。
> - 视口：窄屏 375×812（未注明时的默认）、最小 320×568、桌面 1280×800、宽屏 1280×800 且 `(hover: hover) and (pointer: fine)`。
> - 层级：**前端单测** = Vitest（tianting-sdk-ts 或 tianting-web 组件测试，SDK 用 `FakeTransport` / `FakeClock` 注入）；**Playwright** = tianting-web 页面测试，Chromium + WebKit，连真实 im-server（测试模式），需要改写应答时用 `page.routeWebSocket`（文中记作 `routeWs`）；**端到端** = 真实 im-server + 多个浏览器上下文，前置条件写明依赖哪份 PRD 的服务端功能；**手工** = 人工执行并留截图 / 录屏。
> - 每个用例使用独立数据库；服务端状态用 PRD-social 5.6 节测试接口（`seed`、`limits/reset`、测试时钟）准备。负向断言"不出现"指操作完成后 2 秒内不出现（PRD-ui 2.1 节）。
> - 原生端（M5 / M6）：未标【Web】的 AC 用 Compose UI Test / XCUITest 按同样的输入和预期执行本文件的用例；【Web】AC 按 PRD-ui 3.6 节对照表执行（任务 FE-380～FE-387）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-UI-41-01 | 用户级禁言全部会话显示禁用条 | AC-UI-41-1 | 端到端 | P0 |
| TC-UI-41-02 | 群级禁言只影响该群 | AC-UI-41-2 | 端到端 | P0 |
| TC-UI-41-03 | 手动解除禁言后恢复 | AC-UI-41-3 | 端到端 | P0 |
| TC-UI-41-04 | 到期后本地恢复 | AC-UI-41-4 | 前端单测 | P0 |
| TC-UI-41-05 | 4031 后到期仍被拒绝时重新显示 | AC-UI-41-4 | 前端单测 | P1 |
| TC-UI-41-06 | 永久禁言文案 | AC-UI-41-5 | 前端单测 | P0 |
| TC-UI-41-07 | 全群禁言文案（P1） | AC-UI-41-6 | 前端单测 | P1 |
| TC-UI-42-01 | 在线成员看到消息被删除且 DOM 无原文 | AC-UI-42-1 | 端到端 | P0 |
| TC-UI-42-02 | 被删的是最后一条时摘要更新 | AC-UI-42-2 | 端到端 | P0 |
| TC-UI-42-03 | 离线成员上线后看到已删除占位 | AC-UI-42-3 | 端到端 | P0 |
| TC-UI-42-04 | 已删除占位长按无菜单 | AC-UI-42-4 | Playwright | P0 |
| TC-UI-42-05 | msgDeleted 清除本地原文（SDK） | AC-UI-42-1、AC-UI-42-3 | 前端单测 | P0 |
| TC-UI-43-01 | 登录时被封禁提示解封时间 | AC-UI-43-1 | 端到端 | P0 |
| TC-UI-43-02 | 永久封禁文案 | AC-UI-43-2 | Playwright | P0 |
| TC-UI-43-03 | 在线被封禁弹窗且不重连 | AC-UI-43-3 | 端到端 | P0 |
| TC-UI-43-04 | suspended 与不认识 reason 的弹窗文案 | AC-UI-43-4 | 前端单测 | P0 |
| TC-UI-44-01 | notice 推送显示横幅并进入列表 | AC-UI-44-1 | 端到端 | P0 |
| TC-UI-44-02 | 非 notice 类型不显示 | AC-UI-44-2 | 前端单测 | P0 |
| TC-UI-44-03 | 通知文本 XSS | AC-UI-44-3 | Playwright | P0 |
| TC-UI-44-04 | 横幅截断 50 码点、列表上限 100 | AC-UI-44-1 | 前端单测 | P1 |

---

## F-UI-41 禁言提示（第二批，细化 PRD-admin F-ADM-40、41；全群禁言随 F-ADM-42 为 P1）

### TC-UI-41-01 用户级禁言全部会话显示禁用条
- 关联 AC：AC-UI-41-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端用户禁言与 `muted` 事件（F-ADM-40）；u1 有单聊和群聊。
- 步骤：
  1. 后台对 u1 用户级禁言 10 分钟
  2. u1 分别打开单聊和群聊聊天页
- 预期结果：3 秒内两个聊天页都显示"你已被禁言，解除时间：{禁言时间 + 10 分钟，MM-DD HH:mm}"，输入框不存在

### TC-UI-41-02 群级禁言只影响该群
- 关联 AC：AC-UI-41-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端群内禁言（F-ADM-41）。
- 步骤：
  1. 后台只在 g1 禁言 u1
  2. u1 依次打开 g1、g2、单聊并发送
- 预期结果：g1 显示禁用条；g2 和单聊可正常发送

### TC-UI-41-03 手动解除禁言后恢复
- 关联 AC：AC-UI-41-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端解除禁言与 `unmuted` 事件。
- 步骤：
  1. u1 停在显示禁用条的聊天页
  2. 后台解除禁言
- 预期结果：3 秒内输入框恢复

### TC-UI-41-04 到期后本地恢复
- 关联 AC：AC-UI-41-4
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts + 聊天页组件，`FakeClock`；`muted {scope:'user', mutedUntil: now+60s}`。
- 步骤：
  1. 推进 59 秒
  2. 推进到 61 秒（不推送 `unmuted`）
- 预期结果：
  1. 仍显示禁用条
  2. 输入框恢复

### TC-UI-41-05 4031 后到期仍被拒绝时重新显示
- 关联 AC：AC-UI-41-4
- 优先级：P1
- 层级：前端单测
- 前置条件：同上，到期恢复后 `send` 仍返回 4031（带新的 `mutedUntil`）。
- 步骤：发送一条
- 预期结果：消息标记失败，没有轻提示；禁用条重新出现，时间为新的 `mutedUntil`

### TC-UI-41-06 永久禁言文案
- 关联 AC：AC-UI-41-5
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 推送 `muted {scope:'user', mutedUntil:null}`。
- 步骤：渲染聊天页
- 预期结果：禁用条为"你已被禁言"

### TC-UI-41-07 全群禁言文案（P1）
- 关联 AC：AC-UI-41-6
- 优先级：P1
- 层级：前端单测
- 前置条件：P1，随 PRD-admin F-ADM-42 排期后执行；`muted {scope:'groupAll', groupId:'g1'}`。
- 步骤：渲染 g1 聊天页
- 预期结果：禁用条为"群主已开启全员禁言"

---

## F-UI-42 消息被删除（第二批，细化 PRD-admin F-ADM-31）

### TC-UI-42-01 在线成员看到消息被删除且 DOM 无原文
- 关联 AC：AC-UI-42-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端删除消息与 `msgDeleted` 事件（F-ADM-31）。
- 步骤：
  1. u1 在 g1 发"特征串ZXQ-DEL-1"，u2 停在 g1
  2. 审核员删除该消息
- 预期结果：3 秒内 u2 该位置显示"该消息已被删除"；`page.content()` 中搜不到"ZXQ-DEL-1"

### TC-UI-42-02 被删的是最后一条时摘要更新
- 关联 AC：AC-UI-42-2
- 优先级：P0
- 层级：端到端
- 前置条件：同上。
- 步骤：删除 g1 最后一条消息，u2 打开会话列表
- 预期结果：g1 摘要为"该消息已被删除"，没有发送者前缀

### TC-UI-42-03 离线成员上线后看到已删除占位
- 关联 AC：AC-UI-42-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端 `sync` 返回 `deleted=true`。
- 步骤：
  1. u3 离线期间消息被删除
  2. u3 上线打开 g1
- 预期结果：该消息显示"该消息已被删除"

### TC-UI-42-04 已删除占位长按无菜单
- 关联 AC：AC-UI-42-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录，窄屏；`routeWs` 推送 `msgDeleted`。
- 步骤：长按"该消息已被删除" 600ms
- 预期结果：不出现菜单

### TC-UI-42-05 msgDeleted 清除本地原文（SDK）
- 关联 AC：AC-UI-42-1、AC-UI-42-3
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`：会话有 seq 1～3，未读 2。
- 步骤：
  1. 推送 `msgDeleted {convId, msgIds:[第 2 条], seqs:[2]}`
  2. `sync` 返回 seq 3 带 `deleted=true`
- 预期结果：
  1. 第 2 条 `text` 被清空、标记 `deleted`；seq 不变，未读仍为 2
  2. 第 3 条同样处理

---

## F-UI-43 封禁提示（4034，第二批，细化 PRD-admin F-ADM-13）

### TC-UI-43-01 登录时被封禁提示解封时间
- 关联 AC：AC-UI-43-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端封禁与 `/demo/login` 返回 4034（F-ADM-13）。
- 步骤：
  1. 后台封禁 u1 1 天
  2. u1 在登录页登录
- 预期结果：提示"账号已被封禁，解封时间：{封禁时间 + 1 天，YYYY-MM-DD HH:mm}"，停留在登录页

### TC-UI-43-02 永久封禁文案
- 关联 AC：AC-UI-43-2
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录；`page.route('/demo/login')` 返回 HTTP 403 `{code:4034, details:{bannedUntil:null}}`。
- 步骤：输入"u1"登录
- 预期结果：提示"账号已被封禁"

### TC-UI-43-03 在线被封禁弹窗且不重连
- 关联 AC：AC-UI-43-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批服务端在线封禁下发 `kicked {reason:"suspended", until}` 并以 4012 关闭。
- 步骤：
  1. u1 在线时后台封禁 1 天
  2. 点"确定"，观察 10 秒内的 WebSocket
- 预期结果：
  1. 3 秒内弹出"账号已被封禁，解封时间：{封禁时间 + 1 天}"
  2. 回到登录页；没有新的 WebSocket 连接

### TC-UI-43-04 suspended 与不认识 reason 的弹窗文案
- 关联 AC：AC-UI-43-4
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport`。
- 步骤：
  1. 下发 `kicked {reason:"suspended", until:null}` 后以 4012 关闭
  2. 重新开始，下发 `kicked {reason:"xyz"}` 后以 4012 关闭
- 预期结果：
  1. 弹窗"账号已被封禁"
  2. 弹窗"账号已被下线"；两次都不自动重连

---

## F-UI-44 系统通知（第二批最后，细化 PRD-admin F-ADM-50）

### TC-UI-44-01 notice 推送显示横幅并进入列表
- 关联 AC：AC-UI-44-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第二批推送（F-ADM-50）与二期 `push` 协议。
- 步骤：
  1. 后台向在线的 u1 发送 `type=notice, data={text:"今晚 22 点维护"}`
  2. 点击横幅
- 预期结果：
  1. 3 秒内出现横幅"系统通知：今晚 22 点维护"，5 秒后消失
  2. 进入 `/me/notices`，列表中有这一条；"我"页入口未查看数先为 1、查看后为 0

### TC-UI-44-02 非 notice 类型不显示
- 关联 AC：AC-UI-44-2
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 推送 `push {type:'promo', data:{}}`。
- 步骤：渲染应用
- 预期结果：没有横幅，系统通知列表不增加；写 1 条日志

### TC-UI-44-03 通知文本 XSS
- 关联 AC：AC-UI-44-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录；`routeWs` 推送 `push {type:'notice', data:{text:'<img src=x onerror=alert(1)>'}}`。
- 步骤：等待横幅，打开系统通知列表，监听 `dialog` 和请求
- 预期结果：原样显示文本；没有对话框；没有发往 `x` 的请求

### TC-UI-44-04 横幅截断 50 码点、列表上限 100
- 关联 AC：AC-UI-44-1
- 优先级：P1
- 层级：前端单测
- 前置条件：组件测试。
- 步骤：
  1. 推送一条 80 码点的 notice
  2. 再推送 100 条
- 预期结果：
  1. 横幅显示"系统通知："加前 50 码点
  2. 列表只保留最新 100 条，按到达时间倒序

---

## AC 覆盖

| AC | 用例 |
|---|---|
| AC-UI-41-1 | TC-UI-41-01 |
| AC-UI-41-2 | TC-UI-41-02 |
| AC-UI-41-3 | TC-UI-41-03 |
| AC-UI-41-4 | TC-UI-41-04、TC-UI-41-05 |
| AC-UI-41-5 | TC-UI-41-06 |
| AC-UI-41-6 | TC-UI-41-07 |
| AC-UI-42-1 | TC-UI-42-01、TC-UI-42-05 |
| AC-UI-42-2 | TC-UI-42-02 |
| AC-UI-42-3 | TC-UI-42-03、TC-UI-42-05 |
| AC-UI-42-4 | TC-UI-42-04 |
| AC-UI-43-1 | TC-UI-43-01 |
| AC-UI-43-2 | TC-UI-43-02 |
| AC-UI-43-3 | TC-UI-43-03 |
| AC-UI-43-4 | TC-UI-43-04 |
| AC-UI-44-1 | TC-UI-44-01、TC-UI-44-04 |
| AC-UI-44-2 | TC-UI-44-02 |
| AC-UI-44-3 | TC-UI-44-03 |

共 17 条 AC、20 个用例。
