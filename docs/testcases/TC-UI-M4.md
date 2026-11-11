# 测试用例：客户端 UI（UI，M4：宽屏、深色、PC、客户端群管理）

> 依据：[PRD-ui v1.0](../prd/PRD-ui-v1.0.md)（冻结），本文件覆盖【M4】AC（M3 已有用例在 [TC-UI](TC-UI.md)）；数据与规则见 [PRD-core v1.3](../prd/PRD-core-v1.3.md)、[PRD-social v1.0](../prd/PRD-social-v1.3.md)、[PRD-admin v1.0](../prd/PRD-admin-v1.0.md)；SDK 行为见 [SDK 设计说明](../sdk/README.md)；任务见 [frontend-ui.md](../tasks/frontend-ui.md)。
>
> 通用约定：
> - 测试用户同 PRD-ui 2.1 节：alice、bob、carol、dave、eve 已导入，昵称与 userId 相同（另有说明除外）；`zhangsan` 昵称"张三"；`nobody` 不存在；frank 已导入但与 alice 没有任何关系。"开放模式 / 好友制"同 PRD-social 第 6 节，默认开放模式。
> - 视口：窄屏 375×812（未注明时的默认）、最小 320×568、桌面 1280×800、宽屏 1280×800 且 `(hover: hover) and (pointer: fine)`。
> - 层级：**前端单测** = Vitest（tianting-sdk-ts 或 tianting-web 组件测试，SDK 用 `FakeTransport` / `FakeClock` 注入）；**Playwright** = tianting-web 页面测试，Chromium + WebKit，连真实 im-server（测试模式），需要改写应答时用 `page.routeWebSocket`（文中记作 `routeWs`）；**端到端** = 真实 im-server + 多个浏览器上下文，前置条件写明依赖哪份 PRD 的服务端功能；**手工** = 人工执行并留截图 / 录屏。
> - 每个用例使用独立数据库；服务端状态用 PRD-social 5.6 节测试接口（`seed`、`limits/reset`、测试时钟）准备。负向断言"不出现"指操作完成后 2 秒内不出现（PRD-ui 2.1 节）。
> - 原生端（M5 / M6）：未标【Web】的 AC 用 Compose UI Test / XCUITest 按同样的输入和预期执行本文件的用例；【Web】AC 按 PRD-ui 3.6 节对照表执行（任务 FE-380～FE-387）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-UI-01-09 | 宽屏显示左侧导航栏而非底部标签栏 | AC-UI-01-2 | Playwright | P0 |
| TC-UI-02-04 | 宽屏聊天页三栏可见且无返回按钮 | AC-UI-02-1 | Playwright | P0 |
| TC-UI-02-05 | 宽屏未选会话的主区空态 | AC-UI-02-2 | Playwright | P0 |
| TC-UI-02-06 | 布局切换不改路由、不重连、不丢输入 | AC-UI-02-3 | Playwright | P0 |
| TC-UI-02-07 | 宽屏条件边界 | AC-UI-02-4 | Playwright | P0 |
| TC-UI-02-08 | 手机横屏保持窄屏 | AC-UI-02-7 | Playwright | P0 |
| TC-UI-02-09 | M4 起用触屏平板视口验证居中 | AC-UI-02-6 | Playwright | P1 |
| TC-UI-04-09 | 宽屏切换会话高亮 | AC-UI-04-8 | Playwright | P0 |
| TC-UI-05-07 | 宽屏 Esc 关闭弹窗并还原焦点 | AC-UI-05-4 | Playwright | P0 |
| TC-UI-05-08 | M4 "+"菜单三项顺序 | AC-UI-05-8 | 前端单测 | P0 |
| TC-UI-06-08 | 宽屏打开聊天页输入框获得焦点 | AC-UI-06-5 | Playwright | P0 |
| TC-UI-06-09 | 停在群聊天页时成员数实时更新 | AC-UI-06-8 | 端到端 | P0 |
| TC-UI-09-08 | 宽屏右键菜单且阻止浏览器菜单 | AC-UI-09-2 | Playwright | P0 |
| TC-UI-09-09 | 删除失败消息 | AC-UI-09-5 | Playwright | P0 |
| TC-UI-09-10 | removeFailed（SDK） | AC-UI-09-5 | 前端单测 | P0 |
| TC-UI-10-08 | 宽屏 Enter 发送、Shift+Enter 换行 | AC-UI-10-1 | Playwright | P0 |
| TC-UI-11-01 | 表情插入光标位置 | AC-UI-11-1 | 前端单测 | P0 |
| TC-UI-11-02 | 发送 emoji 对方收到一个码点 | AC-UI-11-2 | 端到端 | P0 |
| TC-UI-11-03 | 构建产物无大位图、svg 在白名单 | AC-UI-11-3 | 前端单测 | P0 |
| TC-UI-11-04 | 表情面板字符数与码点范围 | AC-UI-11-4 | 前端单测 | P0 |
| TC-UI-16-07 | 退出登录后深色偏好保留 | AC-UI-16-5 | Playwright | P0 |
| TC-UI-17-02 | 宽屏只有一条横跨状态条 | AC-UI-17-2 | Playwright | P0 |
| TC-UI-22-03 | 跟随系统实时切换 | AC-UI-22-1 | Playwright | P0 |
| TC-UI-22-04 | 刷新后首帧即为深色且无 CSP 违规 | AC-UI-22-2 | Playwright | P0 |
| TC-UI-22-05 | 选浅色后刷新仍为浅色 | AC-UI-22-5 | Playwright | P0 |
| TC-UI-22-06 | 深色主题 axe 扫描 | AC-UI-22-1 | Playwright | P1 |
| TC-UI-24-01 | 草稿保存与恢复 | AC-UI-24-1 | 前端单测 | P0 |
| TC-UI-24-02 | 草稿不改变排序 | AC-UI-24-2 | 前端单测 | P0 |
| TC-UI-24-03 | 清空后草稿删除 | AC-UI-24-3 | 前端单测 | P0 |
| TC-UI-24-04 | 退出登录清空草稿 | AC-UI-24-4 | Playwright | P0 |
| TC-UI-33-19 | 群主解散自建群，成员看到群聊已解散 | AC-UI-33-7 | 端到端 | P0 |
| TC-UI-33-20 | 另一端退群显示你已退出该群聊 | AC-UI-33-8 | 端到端 | P0 |
| TC-UI-33-21 | left 文案（组件） | AC-UI-33-8 | 前端单测 | P0 |
| TC-UI-34-01 | 生产包空白处右键无菜单，气泡右键有菜单 | AC-UI-34-1 | Playwright | P0 |
| TC-UI-34-02 | 生产包开发者工具快捷键无效 | AC-UI-34-2 | 手工 | P0 |
| TC-UI-34-03 | capabilities 白名单 CI | AC-UI-34-3 | 前端单测 | P0 |
| TC-UI-34-04 | 窗口最小尺寸 | AC-UI-34-4 | 手工 | P0 |
| TC-UI-34-05 | 双平台安装包与 H5 互发消息 | AC-UI-34-5 | 端到端 | P0 |
| TC-UI-34-06 | Tauri 中拼音输入回车选词不发送 | AC-UI-34-6 | 手工 | P0 |
| TC-UI-34-07 | 重启应用回到登录页 | AC-UI-34-7 | Playwright | P0 |
| TC-UI-35-01 | 选两位好友建群进入新群 | AC-UI-35-1 | 端到端 | P0 |
| TC-UI-35-02 | 只选一人时完成禁用 | AC-UI-35-2 | 前端单测 | P0 |
| TC-UI-35-03 | 本地过滤不区分大小写 | AC-UI-35-3 | 前端单测 | P0 |
| TC-UI-35-04 | 提交前被拉黑的好友标红 | AC-UI-35-4 | 端到端 | P0 |
| TC-UI-35-05 | groupCreate 4030 文案 | AC-UI-35-5 | Playwright | P0 |
| TC-UI-36-01 | 普通成员的群设置入口 | AC-UI-36-1 | 前端单测 | P0 |
| TC-UI-36-02 | 管理员的群设置入口 | AC-UI-36-2 | 前端单测 | P0 |
| TC-UI-36-03 | 群主与托管群群主 | AC-UI-36-3 | 前端单测 | P0 |
| TC-UI-36-04 | 托管群无群主时成员的入口 | AC-UI-36-4 | 前端单测 | P0 |
| TC-UI-36-05 | 改群名后其他成员实时看到 | AC-UI-36-5 | 端到端 | P0 |
| TC-UI-36-06 | 自建群群主退群自动转让 | AC-UI-36-6 | 端到端 | P0 |
| TC-UI-36-07 | 群主解散群 | AC-UI-36-7 | 端到端 | P0 |
| TC-UI-36-08 | groupUpdate 4030 提示并刷新 | AC-UI-36-8 | Playwright | P0 |
| TC-UI-36-09 | 入口矩阵 | AC-UI-36-1、AC-UI-36-2、AC-UI-36-3、AC-UI-36-4 | 前端单测 | P1 |
| TC-UI-37-01 | 发布公告后成员可见并出现提示消息 | AC-UI-37-1 | 端到端 | P0 |
| TC-UI-37-02 | 普通成员无编辑 | AC-UI-37-2 | 前端单测 | P0 |
| TC-UI-37-03 | 超过 1000 码点禁用完成 | AC-UI-37-3 | 前端单测 | P0 |
| TC-UI-37-04 | 公告 HTML 原样显示 | AC-UI-37-4 | Playwright | P0 |
| TC-UI-38-01 | 200 人群滚动加载无重复 | AC-UI-38-1 | 端到端 | P0 |
| TC-UI-38-02 | 群主移出两人 | AC-UI-38-2 | 端到端 | P0 |
| TC-UI-38-03 | 管理员移出模式不可选群主和管理员 | AC-UI-38-3 | 前端单测 | P0 |
| TC-UI-38-04 | 设置管理员 | AC-UI-38-4 | 端到端 | P0 |
| TC-UI-38-05 | 转让群主 | AC-UI-38-5 | 端到端 | P0 |
| TC-UI-38-06 | groupSetAdmin 4136 文案 | AC-UI-38-6 | Playwright | P0 |
| TC-UI-38-07 | 没有搜索或过滤输入框 | AC-UI-38-7 | 前端单测 | P0 |
| TC-UI-38-08 | 4046 提示并刷新列表 | AC-UI-38-5 | 前端单测 | P1 |
| TC-UI-39-01 | 邀请两位好友 | AC-UI-39-1 | 端到端 | P0 |
| TC-UI-39-02 | 部分失败的提示 | AC-UI-39-2 | 端到端 | P0 |
| TC-UI-39-03 | 已加载的成员不可选 | AC-UI-39-3 | 前端单测 | P0 |
| TC-UI-39-04 | 一次最多 50 人 | AC-UI-39-4 | 前端单测 | P0 |
| TC-UI-40-01 | joined 提示文案与备注 | AC-UI-40-1 | 前端单测 | P0 |
| TC-UI-40-02 | ownerChanged 与 created 文案 | AC-UI-40-2 | 前端单测 | P0 |
| TC-UI-40-03 | targets 超过 3 个 | AC-UI-40-3 | 前端单测 | P0 |
| TC-UI-40-04 | 不认识的 kind | AC-UI-40-4 | Playwright | P0 |
| TC-UI-40-05 | 提示消息长按无菜单 | AC-UI-40-5 | Playwright | P0 |
| TC-UI-40-06 | tip 摘要用 lastMsg.text | AC-UI-40-6 | 前端单测 | P0 |
| TC-UI-40-07 | F-UI-40 文案表逐行 | AC-UI-40-1、AC-UI-40-2、AC-UI-40-3、AC-UI-40-4 | 前端单测 | P1 |
| TC-UI-90-17 | 只用键盘完成发送与侧板操作 | AC-UI-90-9 | Playwright | P0 |
| TC-UI-90-18 | 640×400 无横向滚动且可发送 | AC-UI-90-10 | Playwright | P0 |

---

## F-UI-01 信息架构与导航（M3；宽屏导航栏 M4）

### TC-UI-01-09 宽屏显示左侧导航栏而非底部标签栏
- 关联 AC：AC-UI-01-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：打开 `/chats`
- 预期结果：左侧导航栏有三个导航项（宽 64），页面中没有底部标签栏

---

## F-UI-02 布局：窄屏与桌面居中（M3）、宽屏三栏（M4）

### TC-UI-02-04 宽屏聊天页三栏可见且无返回按钮
- 关联 AC：AC-UI-02-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：打开 `/chat/c2c%3Aalice%3Abob`
- 预期结果：导航栏、会话列表（该会话高亮）、聊天主区同时可见；顶栏没有返回按钮

### TC-UI-02-05 宽屏未选会话的主区空态
- 关联 AC：AC-UI-02-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：打开 `/chats`
- 预期结果：主区显示天听 Logo 和"选择一个会话开始聊天"

### TC-UI-02-06 布局切换不改路由、不重连、不丢输入
- 关联 AC：AC-UI-02-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`page.on('websocket')` 计数。
- 步骤：
  1. 在聊天页输入"草稿"
  2. 视口改为 375×812（`hover: none`）
- 预期结果：
  1. 路由不变；输入框内容仍为"草稿"
  2. 全程只记录到 1 个 WebSocket 连接

### TC-UI-02-07 宽屏条件边界
- 关联 AC：AC-UI-02-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录；`hover: hover`、`pointer: fine`。
- 步骤：视口依次设为 767×800、768×800、768×499
- 预期结果：767×800 窄屏；768×800 宽屏；768×499 窄屏

### TC-UI-02-08 手机横屏保持窄屏
- 关联 AC：AC-UI-02-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录；视口 812×375，`hover: none`、`pointer: coarse`（`isMobile`、`hasTouch`）。
- 步骤：打开 `/chats`
- 预期结果：有底部标签栏，没有左侧导航栏

### TC-UI-02-09 M4 起用触屏平板视口验证居中
- 关联 AC：AC-UI-02-6
- 优先级：P1
- 层级：Playwright
- 前置条件：M4 构建；视口 768×1024，`hover: none`。
- 步骤：打开 `/chats`，测量容器
- 预期结果：容器宽 480（±1）居中，底部标签栏宽等于容器宽

---

## F-UI-04 会话列表（M3，细化 F-CONV-01 / F-CONV-02）

### TC-UI-04-09 宽屏切换会话高亮
- 关联 AC：AC-UI-04-8
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。两个会话 A、B。
- 步骤：
  1. 点 A
  2. 点 B
- 预期结果：
  1. A 背景为 `color.surface.selected`
  2. A 取消高亮，B 高亮

---

## F-UI-05 "+"菜单与发起单聊（M3，细化 F-CONV-03）

### TC-UI-05-07 宽屏 Esc 关闭弹窗并还原焦点
- 关联 AC：AC-UI-05-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：
  1. 点"+" → "发起单聊"
  2. 按 Esc
- 预期结果：弹窗关闭；`document.activeElement` 为"+"按钮

### TC-UI-05-08 M4 "+"菜单三项顺序
- 关联 AC：AC-UI-05-8
- 优先级：P0
- 层级：前端单测
- 前置条件：M4 构建。
- 步骤：打开"+"菜单
- 预期结果：依次为"发起群聊""添加朋友""发起单聊"

---

## F-UI-06 聊天页结构（M3，细化 F-MSG-05 / F-MSG-06）

### TC-UI-06-08 宽屏打开聊天页输入框获得焦点
- 关联 AC：AC-UI-06-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：点击会话打开聊天页
- 预期结果：输入框为 `document.activeElement`

### TC-UI-06-09 停在群聊天页时成员数实时更新
- 关联 AC：AC-UI-06-8
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupChanged` 与 `groupInfo`（F-SOC-20 规则 5）。
- 步骤：
  1. alice 停在 g1 聊天页
  2. 管理接口给 g1 加 1 人
- 预期结果：3 秒内标题成员数加 1，不离开页面

---

## F-UI-09 消息操作菜单（M3；删除失败消息 M4）

### TC-UI-09-08 宽屏右键菜单且阻止浏览器菜单
- 关联 AC：AC-UI-09-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。一条已发送消息。
- 步骤：在气泡上右键（`click({button:'right'})`），监听 `contextmenu` 事件的 `defaultPrevented`
- 预期结果：出现 F-UI-09 菜单；`defaultPrevented = true`

### TC-UI-09-09 删除失败消息
- 关联 AC：AC-UI-09-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 让一次 `send` 返回 5000。
- 步骤：
  1. 右键失败消息点"删除"
  2. 读取 SDK `pendingCount()`（演示面板或测试钩子）
- 预期结果：
  1. 该消息从列表消失，在当前页面生命周期内不再出现
  2. `pendingCount()` 减 1

### TC-UI-09-10 removeFailed（SDK）
- 关联 AC：AC-UI-09-5
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，一条 failed 消息。
- 步骤：
  1. `removeFailed(convId, clientMsgId)`
  2. 对一条 sent 消息调用
- 预期结果：
  1. 消息从 `messages(convId)` 移除，`pendingCount` 事件减 1，不发任何帧
  2. 不移除（只允许 failed）

---

## F-UI-10 输入区（M3，细化 F-MSG-01 / F-MSG-06）

### TC-UI-10-08 宽屏 Enter 发送、Shift+Enter 换行
- 关联 AC：AC-UI-10-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：
  1. 输入"你好"按 Enter
  2. 输入"a"，Shift+Enter，输入"b"
- 预期结果：
  1. 发出 1 条消息；输入框清空且仍有焦点
  2. 输入框内容为"a\nb"，没有发送

---

## F-UI-11 表情（系统 emoji）（M4，PRD-core 第 7 节变更 V-UI-06）

### TC-UI-11-01 表情插入光标位置
- 关联 AC：AC-UI-11-1
- 优先级：P0
- 层级：前端单测
- 前置条件：输入区组件，内容"你好世界"，光标在第 2 个字后。
- 步骤：打开表情面板点 😀
- 预期结果：输入框内容为"你好😀世界"，光标位于 😀 之后

### TC-UI-11-02 发送 emoji 对方收到一个码点
- 关联 AC：AC-UI-11-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-core 单聊（服务端已有）；alice、bob 两个上下文。
- 步骤：alice 通过表情面板发送"😀"
- 预期结果：bob 收到的消息 `text` 为 U+1F600，`countCodePoints = 1`

### TC-UI-11-03 构建产物无大位图、svg 在白名单
- 关联 AC：AC-UI-11-3
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 脚本，生产构建 `dist/`。
- 步骤：扫描 png / jpg / webp / gif 大小与全部 svg
- 预期结果：没有大于 1KB 的位图；每个 svg 都在 6.6 节图标白名单中

### TC-UI-11-04 表情面板字符数与码点范围
- 关联 AC：AC-UI-11-4
- 优先级：P0
- 层级：前端单测
- 前置条件：表情列表常量。
- 步骤：遍历每个字符的码点
- 预期结果：字符数 ≥ 80；所有码点 ≤ U+1FAFF；不含 U+200D

---

## F-UI-16 设置（M3；外观 M4）

### TC-UI-16-07 退出登录后深色偏好保留
- 关联 AC：AC-UI-16-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录；外观选"深色"。
- 步骤：退出登录
- 预期结果：登录页为深色；`localStorage['ui.theme'] === 'dark'`

---

## F-UI-17 连接状态条（M3，引用 F-CONN-02）

### TC-UI-17-02 宽屏只有一条横跨状态条
- 关联 AC：AC-UI-17-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：`context.setOffline(true)`
- 预期结果：页面中状态条元素只有 1 个；宽度等于列表栏 + 主区宽度（±1px）

---

## F-UI-22 主题：浅色（M3）/ 深色（M4）

### TC-UI-22-03 跟随系统实时切换
- 关联 AC：AC-UI-22-1
- 优先级：P0
- 层级：Playwright
- 前置条件：M4 构建；外观"跟随系统"；`emulateMedia({colorScheme:'dark'})`。
- 步骤：
  1. 打开页面读取 `body` 背景色
  2. 切换为 `light`
- 预期结果：
  1. 为 `#111318`
  2. 1 秒内变为 `#F2F3F7`，没有刷新（`performance.navigation` 不变）

### TC-UI-22-04 刷新后首帧即为深色且无 CSP 违规
- 关联 AC：AC-UI-22-2
- 优先级：P0
- 层级：Playwright
- 前置条件：生产构建 + NF-UI-14 的 CSP 响应头；系统浅色；外观选"深色"。
- 步骤：刷新页面，在 `DOMContentLoaded` 时读取 `body` 计算背景色
- 预期结果：为 `#111318`；没有 `securitypolicyviolation`；`/theme-init.js` 为同源外部脚本

### TC-UI-22-05 选浅色后刷新仍为浅色
- 关联 AC：AC-UI-22-5
- 优先级：P0
- 层级：Playwright
- 前置条件：M4 构建；系统深色。
- 步骤：外观选"浅色"后刷新
- 预期结果：`localStorage['ui.theme'] === 'light'`；`body` 背景为 `#F2F3F7`

### TC-UI-22-06 深色主题 axe 扫描
- 关联 AC：AC-UI-22-1
- 优先级：P1
- 层级：Playwright
- 前置条件：M4 构建，深色。
- 步骤：用 axe 扫描登录、会话列表、聊天页、通讯录、新的朋友、资料卡、设置
- 预期结果：无 `color-contrast` 违规（AC-UI-90-4 的 M4 部分）

---

## F-UI-24 草稿（M4）

### TC-UI-24-01 草稿保存与恢复
- 关联 AC：AC-UI-24-1
- 优先级：P0
- 层级：前端单测
- 前置条件：会话列表 + 聊天页组件。
- 步骤：
  1. 在会话 A 输入"明天见"不发送，返回会话列表
  2. 再进入 A
- 预期结果：
  1. A 的摘要为"[草稿] 明天见"，"[草稿]"为 `color.danger`
  2. 输入框内容为"明天见"

### TC-UI-24-02 草稿不改变排序
- 关联 AC：AC-UI-24-2
- 优先级：P0
- 层级：前端单测
- 前置条件：A 排第 3，有草稿。
- 步骤：
  1. 检查顺序
  2. `FakeTransport` 推送 A 的新消息
- 预期结果：
  1. A 仍在第 3
  2. A 移到顶部，摘要仍为"[草稿] …"

### TC-UI-24-03 清空后草稿删除
- 关联 AC：AC-UI-24-3
- 优先级：P0
- 层级：前端单测
- 前置条件：A 有草稿。
- 步骤：进入 A 清空输入框后离开
- 预期结果：摘要恢复为最后一条消息

### TC-UI-24-04 退出登录清空草稿
- 关联 AC：AC-UI-24-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录 alice，会话 A 有草稿。
- 步骤：退出登录后以 alice 重新登录
- 预期结果：A 没有草稿，摘要为最后一条消息

---

## F-UI-33 未知会话、被移出 / 退出 / 解散的会话（M3；`reason = left` 为 M4）

### TC-UI-33-19 群主解散自建群，成员看到群聊已解散
- 关联 AC：AC-UI-33-7
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupDissolve` 与 `convRemoved.reason=dissolved`（F-SOC-26）。
- 步骤：
  1. bob 停在 alice 自建群 g5 的聊天页
  2. alice 在群设置解散 g5
- 预期结果：3 秒内 bob 看到"群聊已解散"，输入框消失

### TC-UI-33-20 另一端退群显示你已退出该群聊
- 关联 AC：AC-UI-33-8
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupLeave` 与 `convRemoved.reason=left` 多端下发（F-SOC-23）；alice 两个标签页，第二个 `?platform=pc`。
- 步骤：
  1. web 标签页停在 g2 聊天页
  2. pc 标签页退出 g2
- 预期结果：3 秒内 web 标签页显示"你已退出该群聊"（列表底部与禁用条）

### TC-UI-33-21 left 文案（组件）
- 关联 AC：AC-UI-33-8
- 优先级：P0
- 层级：前端单测
- 前置条件：M4 构建；`FakeTransport` 推送 `convRemoved {convId:'group:g2', reason:'left'}`。
- 步骤：渲染 g2 聊天页
- 预期结果：列表底部和禁用条都为"你已退出该群聊"

---

## F-UI-34 PC 客户端（Tauri）专项（M4，R-UI-17）

### TC-UI-34-01 生产包空白处右键无菜单，气泡右键有菜单
- 关联 AC：AC-UI-34-1
- 优先级：P0
- 层级：Playwright
- 前置条件：tianting-desktop 生产包（WebDriver / tauri-driver）。
- 步骤：
  1. 在会话列表空白处右键
  2. 在消息气泡上右键
- 预期结果：
  1. 不出现任何菜单
  2. 出现 F-UI-09 消息菜单

### TC-UI-34-02 生产包开发者工具快捷键无效
- 关联 AC：AC-UI-34-2
- 优先级：P0
- 层级：手工
- 前置条件：macOS 与 Windows 生产包。
- 步骤：分别按 Cmd+Option+I / Ctrl+Shift+I
- 预期结果：开发者工具不打开（截图记录）

### TC-UI-34-03 capabilities 白名单 CI
- 关联 AC：AC-UI-34-3
- 优先级：P0
- 层级：前端单测
- 前置条件：tianting-desktop CI 脚本。
- 步骤：
  1. 检查 `src-tauri/capabilities/*.json`
  2. 临时加入 `shell:allow-open` 再检查
- 预期结果：
  1. 没有 `shell:*`、`fs:*`、`http:*`、`process:*`，检查通过
  2. 加入后 CI 失败

### TC-UI-34-04 窗口最小尺寸
- 关联 AC：AC-UI-34-4
- 优先级：P0
- 层级：手工
- 前置条件：macOS 生产包。
- 步骤：
  1. 把窗口拖到 600×400
  2. 在 800×560 下打开聊天页，检查 `scrollWidth`
- 预期结果：
  1. 窗口停在 800×560
  2. 三栏完整显示，`scrollWidth ≤ 800`

### TC-UI-34-05 双平台安装包与 H5 互发消息
- 关联 AC：AC-UI-34-5
- 优先级：P0
- 层级：端到端
- 前置条件：需要已部署的 im-server（PRD-core 单聊）；macOS 与 Windows 安装包。
- 步骤：
  1. 各安装一次，以 alice 登录
  2. 与 H5 上的 bob 互发消息
- 预期结果：
  1. 窗口标题为"天听"
  2. 双方都收到

### TC-UI-34-06 Tauri 中拼音输入回车选词不发送
- 关联 AC：AC-UI-34-6
- 优先级：P0
- 层级：手工
- 前置条件：macOS（WKWebView）与 Windows（WebView2）生产包，系统拼音输入法。
- 步骤：输入"nihao"按回车选词
- 预期结果：只上屏"你好"，不发送（录屏记录）

### TC-UI-34-07 重启应用回到登录页
- 关联 AC：AC-UI-34-7
- 优先级：P0
- 层级：Playwright
- 前置条件：tianting-desktop 生产包（WebDriver）。
- 步骤：登录后关闭窗口，重新启动应用
- 预期结果：显示登录页（token 只在 `sessionStorage`）

---

## F-UI-35 发起群聊（M4，细化 PRD-social F-SOC-21）

### TC-UI-35-01 选两位好友建群进入新群
- 关联 AC：AC-UI-35-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupCreate` 与 `tip`（F-SOC-21、F-SOC-29）；bob、carol 是 alice 的好友。
- 步骤：alice "+" → "发起群聊"，选 bob、carol，点"完成（2）"，不填群名
- 预期结果：进入新群聊天页，标题为服务端生成的群名加"（3）"；第一条为"你创建了群聊"

### TC-UI-35-02 只选一人时完成禁用
- 关联 AC：AC-UI-35-2
- 优先级：P0
- 层级：前端单测
- 前置条件：发起群聊组件。
- 步骤：只勾选 bob
- 预期结果：按钮"完成（1）"为禁用态

### TC-UI-35-03 本地过滤不区分大小写
- 关联 AC：AC-UI-35-3
- 优先级：P0
- 层级：前端单测
- 前置条件：好友：bob、Bobby（userId `u_bo`）、carol。
- 步骤：搜索框输入"BO"
- 预期结果：只剩显示名或 userId 含"bo"的 bob、Bobby

### TC-UI-35-04 提交前被拉黑的好友标红
- 关联 AC：AC-UI-35-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupCreate` 的 4035（带 `userId`）。
- 步骤：
  1. alice 选 bob、carol
  2. carol 拉黑 alice
  3. alice 点"完成"
- 预期结果：提示"carol 不是你的好友，无法邀请"，carol 标红；没有创建群

### TC-UI-35-05 groupCreate 4030 文案
- 关联 AC：AC-UI-35-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 让 `groupCreate` 返回 4030。
- 步骤：选 2 人点"完成"
- 预期结果：提示"当前不允许创建群聊"

---

## F-UI-36 群设置（M4，扩展 F-UI-12，细化 PRD-social F-SOC-20、23～27）

### TC-UI-36-01 普通成员的群设置入口
- 关联 AC：AC-UI-36-1
- 优先级：P0
- 层级：前端单测
- 前置条件：`getGroupInfo` 返回 `myRole=member`、自建群。
- 步骤：渲染群设置
- 预期结果：可见成员网格、"+"、群聊名称（不可点击）、群公告、"退出群聊"；不可见"−""设置管理员""转让群主""解散群聊"

### TC-UI-36-02 管理员的群设置入口
- 关联 AC：AC-UI-36-2
- 优先级：P0
- 层级：前端单测
- 前置条件：`myRole=admin`、自建群。
- 步骤：渲染群设置
- 预期结果：可见"−"，群聊名称可点击修改；不可见"设置管理员""转让群主""解散群聊"

### TC-UI-36-03 群主与托管群群主
- 关联 AC：AC-UI-36-3
- 优先级：P0
- 层级：前端单测
- 前置条件：自建群 `myRole=owner`；托管群 `managed=true` 的 `owner`。
- 步骤：分别渲染
- 预期结果：自建群群主可见全部入口；托管群群主不可见"解散群聊"

### TC-UI-36-04 托管群无群主时成员的入口
- 关联 AC：AC-UI-36-4
- 优先级：P0
- 层级：前端单测
- 前置条件：`managed=true`、无群主、`myRole=member`。
- 步骤：渲染群设置
- 预期结果：只可见成员网格、群聊名称（只读）、群公告、"退出群聊"；网格末尾没有"+"

### TC-UI-36-05 改群名后其他成员实时看到
- 关联 AC：AC-UI-36-5
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupUpdate`、`groupChanged`、`renamed` 提示消息。
- 步骤：群主 alice 把群名改为"周六球局"
- 预期结果：3 秒内 bob 的聊天页标题为"周六球局（N）"，出现"alice 修改群名为"周六球局""

### TC-UI-36-06 自建群群主退群自动转让
- 关联 AC：AC-UI-36-6
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端退群与自动转让（F-SOC-23）；最早的管理员为 dave。
- 步骤：
  1. alice 在群设置点"退出群聊"
  2. 确认
  3. dave 打开群设置
- 预期结果：
  1. 确认框正文为"你是群主，退出后群主将自动转让给其他成员"
  2. alice 回到会话列表，该群不在列表中
  3. dave 的群设置出现"解散群聊"

### TC-UI-36-07 群主解散群
- 关联 AC：AC-UI-36-7
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupDissolve`。
- 步骤：alice 点"解散群聊"并确认
- 预期结果：alice 回到会话列表；在线成员看到"群聊已解散"

### TC-UI-36-08 groupUpdate 4030 提示并刷新
- 关联 AC：AC-UI-36-8
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 让 `groupUpdate` 返回 4030。
- 步骤：管理员修改群名
- 预期结果：提示"无权限"；随后重新发出 1 次 `groupInfo`

### TC-UI-36-09 入口矩阵
- 关联 AC：AC-UI-36-1、AC-UI-36-2、AC-UI-36-3、AC-UI-36-4
- 优先级：P1
- 层级：前端单测
- 前置条件：群设置组件。
- 步骤：遍历 `myRole`（owner / admin / member）× `managed`（是 / 否）× 托管群是否有群主
- 预期结果：每种组合的可见入口与 F-UI-36 规则 2 的表一致（快照断言）

---

## F-UI-37 群公告（M4，细化 PRD-social F-SOC-27）

### TC-UI-37-01 发布公告后成员可见并出现提示消息
- 关联 AC：AC-UI-37-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupUpdate {notice}` 与 `noticeUpdated` 提示消息（F-SOC-27、F-SOC-29）。
- 步骤：
  1. 群主发布"周六 9 点集合"
  2. bob 打开群公告页
- 预期结果：
  1. bob 聊天页出现"{群主} 更新了群公告"
  2. 显示全文、群主显示名和发布时间

### TC-UI-37-02 普通成员无编辑
- 关联 AC：AC-UI-37-2
- 优先级：P0
- 层级：前端单测
- 前置条件：`myRole=member`。
- 步骤：渲染群公告页
- 预期结果：没有"编辑"；显示"仅群主和管理员可编辑"

### TC-UI-37-03 超过 1000 码点禁用完成
- 关联 AC：AC-UI-37-3
- 优先级：P0
- 层级：前端单测
- 前置条件：`myRole=owner`，编辑态。
- 步骤：输入 1001 个码点
- 预期结果：显示"已输入 1001 / 1000"（`color.danger`），"完成"禁用

### TC-UI-37-04 公告 HTML 原样显示
- 关联 AC：AC-UI-37-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 让 `groupInfo` 返回公告 `<b>x</b>`。
- 步骤：打开群公告页
- 预期结果：显示文本 `<b>x</b>`；页面中没有 `<b>` 元素

---

## F-UI-38 群成员列表（M4，细化 PRD-social F-SOC-24、25、28）

### TC-UI-38-01 200 人群滚动加载无重复
- 关联 AC：AC-UI-38-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupMembers` 分页（F-SOC-28）；`seed` 200 人群，含 1 个群主、2 个管理员。
- 步骤：打开成员列表并滚动到底
- 预期结果：共 200 人，无重复 userId；第一项为带"群主"标签的群主，随后是管理员

### TC-UI-38-02 群主移出两人
- 关联 AC：AC-UI-38-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupKick` 与 `convRemoved`。
- 步骤：群主进入移出模式选 bob、carol，确认"确定将 2 人移出群聊？"
- 预期结果：列表中没有两人；群设置人数减 2；bob 的聊天页显示"你已不在该群"

### TC-UI-38-03 管理员移出模式不可选群主和管理员
- 关联 AC：AC-UI-38-3
- 优先级：P0
- 层级：前端单测
- 前置条件：`myRole=admin`，成员列表含群主、另一管理员、普通成员。
- 步骤：进入移出模式
- 预期结果：群主和其他管理员的复选框禁用；普通成员可选

### TC-UI-38-04 设置管理员
- 关联 AC：AC-UI-38-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupSetAdmin`。
- 步骤：
  1. 群主在设置管理员模式点 dave
  2. dave 打开群设置
- 预期结果：
  1. dave 的标签变为"管理员"
  2. dave 的群设置出现"−"

### TC-UI-38-05 转让群主
- 关联 AC：AC-UI-38-5
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupTransfer`。
- 步骤：群主选 dave，确认"确定将群主转让给 dave？转让后你将成为普通成员"
- 预期结果：dave 带"群主"标签排在第一；原群主变为普通成员

### TC-UI-38-06 groupSetAdmin 4136 文案
- 关联 AC：AC-UI-38-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 让 `groupSetAdmin` 返回 4136。
- 步骤：设置一名管理员
- 预期结果：提示"管理员最多 10 人"

### TC-UI-38-07 没有搜索或过滤输入框
- 关联 AC：AC-UI-38-7
- 优先级：P0
- 层级：前端单测
- 前置条件：成员列表组件。
- 步骤：渲染
- 预期结果：页面中没有 `input` 元素

### TC-UI-38-08 4046 提示并刷新列表
- 关联 AC：AC-UI-38-5
- 优先级：P1
- 层级：前端单测
- 前置条件：`transferGroupOwner` 返回 4046。
- 步骤：转让给 dave
- 预期结果：提示"对方已不在群里"，重新加载成员列表第一页

---

## F-UI-39 邀请成员（M4，细化 PRD-social F-SOC-22）

### TC-UI-39-01 邀请两位好友
- 关联 AC：AC-UI-39-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupInvite` 与 `joined` 提示消息（F-SOC-22、F-SOC-29）。
- 步骤：在群设置点"+"，选 eve、frank 提交
- 预期结果：提示"已邀请 2 人"，返回群设置，人数加 2；聊天页出现"你邀请 eve、frank 加入了群聊"

### TC-UI-39-02 部分失败的提示
- 关联 AC：AC-UI-39-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M4 服务端 `groupInvite` 的 `failed[]`；eve 已拉黑 alice。
- 步骤：邀请 bob、carol、eve
- 预期结果：提示"已邀请 2 人"和"eve 不是你的好友，无法邀请"

### TC-UI-39-03 已加载的成员不可选
- 关联 AC：AC-UI-39-3
- 优先级：P0
- 层级：前端单测
- 前置条件：成员列表已加载到 bob。
- 步骤：打开邀请页
- 预期结果：bob 显示"已在群里"，复选框禁用

### TC-UI-39-04 一次最多 50 人
- 关联 AC：AC-UI-39-4
- 优先级：P0
- 层级：前端单测
- 前置条件：好友 60 人。
- 步骤：依次勾选 51 人
- 预期结果：第 51 人时提示"一次最多邀请 50 人"，该人未被勾选

---

## F-UI-40 群提示消息（M4，细化 PRD-social F-SOC-29）

### TC-UI-40-01 joined 提示文案与备注
- 关联 AC：AC-UI-40-1
- 优先级：P0
- 层级：前端单测
- 前置条件：`tip {kind:'joined', actor:'bob', targets:['carol','dave']}`。
- 步骤：
  1. 渲染
  2. 设置 carol 备注"卡姐"后再渲染
- 预期结果：
  1. "bob 邀请 carol、dave 加入了群聊"，没有头像和气泡
  2. "bob 邀请 卡姐、dave 加入了群聊"

### TC-UI-40-02 ownerChanged 与 created 文案
- 关联 AC：AC-UI-40-2
- 优先级：P0
- 层级：前端单测
- 前置条件：当前用户 alice。
- 步骤：
  1. 渲染 `{kind:'ownerChanged', actor:'', targets:[]}`
  2. 渲染 `{kind:'created', actor:'alice'}`
- 预期结果：
  1. "群主已退出群聊"
  2. "你创建了群聊"

### TC-UI-40-03 targets 超过 3 个
- 关联 AC：AC-UI-40-3
- 优先级：P0
- 层级：前端单测
- 前置条件：`{kind:'joined', actor:'bob', targets:[20 个], count:35}`。
- 步骤：渲染
- 预期结果：只列前 3 个显示名，加"等 35 人"

### TC-UI-40-04 不认识的 kind
- 关联 AC：AC-UI-40-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。`routeWs` 推送 `tip {kind:'futureKind'}`。
- 步骤：打开该群，收集 `console`
- 预期结果：显示"[群通知]"；没有 error

### TC-UI-40-05 提示消息长按无菜单
- 关联 AC：AC-UI-40-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录，窄屏；群中有一条 tip 消息。
- 步骤：长按 600ms
- 预期结果：不出现菜单

### TC-UI-40-06 tip 摘要用 lastMsg.text
- 关联 AC：AC-UI-40-6
- 优先级：P0
- 层级：前端单测
- 前置条件：群最后一条为 `joined` tip，`lastMsg.text="bob 邀请 carol 加入了群聊"`、`from='bob'`。
- 步骤：渲染会话列表
- 预期结果：摘要等于 `lastMsg.text`，没有"bob: "前缀

### TC-UI-40-07 F-UI-40 文案表逐行
- 关联 AC：AC-UI-40-1、AC-UI-40-2、AC-UI-40-3、AC-UI-40-4
- 优先级：P1
- 层级：前端单测
- 前置条件：文案函数。
- 步骤：遍历 created / joined / ownerChanged（三种）/ adminSet / adminUnset / renamed / noticeUpdated / 不认识 kind，actor / targets 分别为自己和他人
- 预期结果：每行与 F-UI-40 规则 2 的表一致

---

## 非功能（PRD-ui 9.5 节）

### TC-UI-90-17 只用键盘完成发送与侧板操作
- 关联 AC：AC-UI-90-9
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），M4 构建，宽屏视口 1280×800（`hover: hover`、`pointer: fine`），测试模式 im-server。
- 步骤：
  1. Tab 到会话列表，Enter 打开会话
  2. 在输入框输入并 Enter 发送
  3. Tab 到"更多"，Enter 打开聊天信息侧板，按 Esc
- 预期结果：
  1. 每一步焦点元素有可见焦点样式（`outline-width ≥ 2px`）
  2. 消息发出
  3. 侧板关闭，焦点回到"更多"

### TC-UI-90-18 640×400 无横向滚动且可发送
- 关联 AC：AC-UI-90-10
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录；视口 640×400。
- 步骤：
  1. 依次打开宽屏 AC 涉及的页面（会话列表、聊天页、聊天信息、资料卡、设置）
  2. 发送一条消息
- 预期结果：
  1. 每页 `scrollWidth ≤ 640`
  2. 发送成功

---

## AC 覆盖

| AC | 用例 |
|---|---|
| AC-UI-01-2 | TC-UI-01-09 |
| AC-UI-02-1 | TC-UI-02-04 |
| AC-UI-02-2 | TC-UI-02-05 |
| AC-UI-02-3 | TC-UI-02-06 |
| AC-UI-02-4 | TC-UI-02-07 |
| AC-UI-02-6 | TC-UI-02-09 |
| AC-UI-02-7 | TC-UI-02-08 |
| AC-UI-04-8 | TC-UI-04-09 |
| AC-UI-05-4 | TC-UI-05-07 |
| AC-UI-05-8 | TC-UI-05-08 |
| AC-UI-06-5 | TC-UI-06-08 |
| AC-UI-06-8 | TC-UI-06-09 |
| AC-UI-09-2 | TC-UI-09-08 |
| AC-UI-09-5 | TC-UI-09-09、TC-UI-09-10 |
| AC-UI-10-1 | TC-UI-10-08 |
| AC-UI-11-1 | TC-UI-11-01 |
| AC-UI-11-2 | TC-UI-11-02 |
| AC-UI-11-3 | TC-UI-11-03 |
| AC-UI-11-4 | TC-UI-11-04 |
| AC-UI-16-5 | TC-UI-16-07 |
| AC-UI-17-2 | TC-UI-17-02 |
| AC-UI-22-1 | TC-UI-22-03、TC-UI-22-06 |
| AC-UI-22-2 | TC-UI-22-04 |
| AC-UI-22-5 | TC-UI-22-05 |
| AC-UI-24-1 | TC-UI-24-01 |
| AC-UI-24-2 | TC-UI-24-02 |
| AC-UI-24-3 | TC-UI-24-03 |
| AC-UI-24-4 | TC-UI-24-04 |
| AC-UI-33-7 | TC-UI-33-19 |
| AC-UI-33-8 | TC-UI-33-20、TC-UI-33-21 |
| AC-UI-34-1 | TC-UI-34-01 |
| AC-UI-34-2 | TC-UI-34-02 |
| AC-UI-34-3 | TC-UI-34-03 |
| AC-UI-34-4 | TC-UI-34-04 |
| AC-UI-34-5 | TC-UI-34-05 |
| AC-UI-34-6 | TC-UI-34-06 |
| AC-UI-34-7 | TC-UI-34-07 |
| AC-UI-35-1 | TC-UI-35-01 |
| AC-UI-35-2 | TC-UI-35-02 |
| AC-UI-35-3 | TC-UI-35-03 |
| AC-UI-35-4 | TC-UI-35-04 |
| AC-UI-35-5 | TC-UI-35-05 |
| AC-UI-36-1 | TC-UI-36-01、TC-UI-36-09 |
| AC-UI-36-2 | TC-UI-36-02、TC-UI-36-09 |
| AC-UI-36-3 | TC-UI-36-03、TC-UI-36-09 |
| AC-UI-36-4 | TC-UI-36-04、TC-UI-36-09 |
| AC-UI-36-5 | TC-UI-36-05 |
| AC-UI-36-6 | TC-UI-36-06 |
| AC-UI-36-7 | TC-UI-36-07 |
| AC-UI-36-8 | TC-UI-36-08 |
| AC-UI-37-1 | TC-UI-37-01 |
| AC-UI-37-2 | TC-UI-37-02 |
| AC-UI-37-3 | TC-UI-37-03 |
| AC-UI-37-4 | TC-UI-37-04 |
| AC-UI-38-1 | TC-UI-38-01 |
| AC-UI-38-2 | TC-UI-38-02 |
| AC-UI-38-3 | TC-UI-38-03 |
| AC-UI-38-4 | TC-UI-38-04 |
| AC-UI-38-5 | TC-UI-38-05、TC-UI-38-08 |
| AC-UI-38-6 | TC-UI-38-06 |
| AC-UI-38-7 | TC-UI-38-07 |
| AC-UI-39-1 | TC-UI-39-01 |
| AC-UI-39-2 | TC-UI-39-02 |
| AC-UI-39-3 | TC-UI-39-03 |
| AC-UI-39-4 | TC-UI-39-04 |
| AC-UI-40-1 | TC-UI-40-01、TC-UI-40-07 |
| AC-UI-40-2 | TC-UI-40-02、TC-UI-40-07 |
| AC-UI-40-3 | TC-UI-40-03、TC-UI-40-07 |
| AC-UI-40-4 | TC-UI-40-04、TC-UI-40-07 |
| AC-UI-40-5 | TC-UI-40-05 |
| AC-UI-40-6 | TC-UI-40-06 |
| AC-UI-90-9 | TC-UI-90-17 |
| AC-UI-90-10 | TC-UI-90-18 |

共 73 条 AC、79 个用例。
