# 测试用例：客户端 UI（UI，M3）

> 依据：[PRD-ui v1.0](../prd/PRD-ui-v1.0.md)（冻结），本文件覆盖【M3】AC；数据与规则见 [PRD-core v1.3](../prd/PRD-core-v1.3.md)、[PRD-social v1.0](../prd/PRD-social-v1.2.md)、[PRD-admin v1.0](../prd/PRD-admin-v1.0.md)（第一批解散群）；SDK 行为见 [SDK 设计说明](../sdk/README.md)；任务见 [frontend-ui.md](../tasks/frontend-ui.md)。
>
> 通用约定：
> - 测试用户同 PRD-ui 2.1 节：alice、bob、carol、dave、eve 已导入，昵称与 userId 相同（另有说明除外）；`zhangsan` 昵称"张三"；`nobody` 不存在；frank 已导入但与 alice 没有任何关系。"开放模式 / 好友制"同 PRD-social 第 6 节，默认开放模式。
> - 视口：窄屏 375×812（未注明时的默认）、最小 320×568、桌面 1280×800、宽屏 1280×800 且 `(hover: hover) and (pointer: fine)`。
> - 层级：**前端单测** = Vitest（tianting-sdk-ts 或 tianting-web 组件测试，SDK 用 `FakeTransport` / `FakeClock` 注入）；**Playwright** = tianting-web 页面测试，Chromium + WebKit，连真实 im-server（测试模式），需要改写应答时用 `page.routeWebSocket`（文中记作 `routeWs`）；**端到端** = 真实 im-server + 多个浏览器上下文，前置条件写明依赖哪份 PRD 的服务端功能；**手工** = 人工执行并留截图 / 录屏。
> - 每个用例使用独立数据库；服务端状态用 PRD-social 5.6 节测试接口（`seed`、`limits/reset`、测试时钟）准备。负向断言"不出现"指操作完成后 2 秒内不出现（PRD-ui 2.1 节）。
> - 原生端（M5 / M6）：未标【Web】的 AC 用 Compose UI Test / XCUITest 按同样的输入和预期执行本文件的用例；【Web】AC 按 PRD-ui 3.6 节对照表执行（任务 FE-380～FE-387）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-UI-01-01 | 已登录进入 /chats 显示底部三项导航，切换到"我" | AC-UI-01-1 | Playwright | P0 |
| TC-UI-01-02 | 未登录打开单聊深链接，登录后回到原会话 | AC-UI-01-3 | Playwright | P0 |
| TC-UI-01-03 | 已登录访问 /login 跳转到 /chats | AC-UI-01-4 | Playwright | P0 |
| TC-UI-01-04 | 从聊天页后退恢复会话列表滚动位置 | AC-UI-01-5 | Playwright | P0 |
| TC-UI-01-05 | 二级页面隐藏底部标签栏 | AC-UI-01-6 | Playwright | P0 |
| TC-UI-01-06 | 回跳地址校验：四种恶意 redirect 都回到 /chats | AC-UI-01-7 | Playwright | P0 |
| TC-UI-01-07 | 回跳地址校验函数单元测试 | AC-UI-01-7 | 前端单测 | P0 |
| TC-UI-01-08 | 点击已选中的"消息"标签回到顶部 | AC-UI-01-8 | Playwright | P0 |
| TC-UI-02-01 | 320×568 下一级页面和主要二级页面无横向滚动 | AC-UI-02-5 | Playwright | P0 |
| TC-UI-02-02 | 桌面视口容器居中 480px | AC-UI-02-6 | Playwright | P0 |
| TC-UI-02-03 | iPhone 15 Safari 安全区 | AC-UI-02-8 | 手工 | P0 |
| TC-UI-03-01 | 用户名为空或全空格时登录按钮禁用 | AC-UI-03-1 | 前端单测 | P0 |
| TC-UI-03-02 | 回车登录只发一次请求并进入 /chats | AC-UI-03-2 | Playwright | P0 |
| TC-UI-03-03 | 请求未返回时连点三次只发一次 | AC-UI-03-3 | Playwright | P0 |
| TC-UI-03-04 | 演示登录未开启提示"登录服务未开启" | AC-UI-03-4 | Playwright | P0 |
| TC-UI-03-05 | 登录请求 10 秒超时提示网络异常 | AC-UI-03-5 | Playwright | P0 |
| TC-UI-03-06 | 登录页输入框计算字号 ≥ 16px | AC-UI-03-6 | Playwright | P1 |
| TC-UI-03-07 | 以昵称"张三"登录后各处显示张三 | AC-UI-03-7 | 端到端 | P0 |
| TC-UI-03-08 | 昵称超过 32 个字时提示并禁用 | AC-UI-03-8 | 前端单测 | P0 |
| TC-UI-03-10 | 登录页标题与品牌名 | AC-UI-03-10 | Playwright | P0 |
| TC-UI-04-01 | 群聊摘要带发送者前缀，自己发的不带 | AC-UI-04-1 | 前端单测 | P0 |
| TC-UI-04-02 | 单聊摘要不带发送者前缀 | AC-UI-04-2 | 前端单测 | P0 |
| TC-UI-04-03 | 没有消息的群不显示摘要和时间 | AC-UI-04-3 | 前端单测 | P0 |
| TC-UI-04-04 | custom 消息摘要显示"[暂不支持的消息]" | AC-UI-04-4 | 前端单测 | P0 |
| TC-UI-04-05 | 超长摘要单行省略且项高 72 | AC-UI-04-5 | Playwright | P1 |
| TC-UI-04-06 | 未读角标位置与 99+ | AC-UI-04-6 | Playwright | P0 |
| TC-UI-04-07 | truncated 时显示"仅显示最近 500 个会话" | AC-UI-04-7 | 前端单测 | P0 |
| TC-UI-04-08 | 摘要前缀按备注 > 昵称显示 | AC-UI-04-9 | 前端单测 | P0 |
| TC-UI-05-01 | + 菜单和空状态打开同一个发起单聊弹窗 | AC-UI-05-1 | Playwright | P0 |
| TC-UI-05-02 | 开放模式下回车进入单聊 | AC-UI-05-2 | Playwright | P0 |
| TC-UI-05-03 | 输入自己的 userId 进入自聊，好友制也不提示 | AC-UI-05-3 | Playwright | P0 |
| TC-UI-05-04 | + 菜单两项及顺序 | AC-UI-05-5 | Playwright | P0 |
| TC-UI-05-05 | 好友制下发起非好友单聊的提示 | AC-UI-05-6 | 前端单测 | P0 |
| TC-UI-05-06 | 点遮罩关闭弹窗，路由不变 | AC-UI-05-7 | Playwright | P0 |
| TC-UI-06-01 | 群名过长时省略，成员数完整 | AC-UI-06-1 | Playwright | P0 |
| TC-UI-06-02 | memberCount 缺失时只显示群名 | AC-UI-06-2 | Playwright | P0 |
| TC-UI-06-03 | 100 条消息时进入聊天页贴底 | AC-UI-06-3 | Playwright | P0 |
| TC-UI-06-04 | 停在聊天页时未读不增加，离开后增加 | AC-UI-06-4 | Playwright | P0 |
| TC-UI-06-05 | 再次进入群聊天页 3 秒内刷新成员数 | AC-UI-06-6 | 端到端 | P0 |
| TC-UI-06-06 | refreshConversations 30 秒节流 | AC-UI-06-6 | 前端单测 | P0 |
| TC-UI-06-07 | 进入聊天信息页时 setActiveConversation(null) | AC-UI-06-7 | 前端单测 | P0 |
| TC-UI-07-01 | 左右气泡与列表边距 60px | AC-UI-07-1 | Playwright | P0 |
| TC-UI-07-02 | 群聊显示发送者显示名，单聊和自己不显示 | AC-UI-07-2 | 前端单测 | P0 |
| TC-UI-07-03 | 超长消息气泡宽度 ≤ 70% | AC-UI-07-3 | Playwright | P0 |
| TC-UI-07-04 | custom 消息显示占位且无 console error | AC-UI-07-4 | Playwright | P0 |
| TC-UI-07-05 | URL 与 script 原样显示为文本 | AC-UI-07-5 | Playwright | P0 |
| TC-UI-07-06 | 自己气泡文字对比度 ≥ 4.5 | AC-UI-07-6 | Playwright | P0 |
| TC-UI-07-07 | 点击群聊中 bob 的头像进入资料卡 | AC-UI-07-7 | Playwright | P0 |
| TC-UI-08-01 | ack 延迟期间显示转圈，到达后消失 | AC-UI-08-1 | Playwright | P0 |
| TC-UI-08-02 | 点"!"确认后重发沿用 clientMsgId | AC-UI-08-2 | Playwright | P0 |
| TC-UI-08-03 | 4290 提示恰好一次，超时不提示 | AC-UI-08-3 | Playwright | P0 |
| TC-UI-08-04 | sendFailed 事件每次失败恰好一次 | AC-UI-08-3、AC-UI-08-5 | 前端单测 | P0 |
| TC-UI-08-05 | 失败图标点击区域 ≥ 44×44 | AC-UI-08-4 | Playwright | P0 |
| TC-UI-08-06 | 两次 4290 失败共提示两次，切页不额外提示 | AC-UI-08-5 | Playwright | P0 |
| TC-UI-09-01 | 长按 500ms 弹出菜单，短按和移动不弹出 | AC-UI-09-1 | Playwright | P0 |
| TC-UI-09-02 | 长按状态机单元测试 | AC-UI-09-1 | 前端单测 | P0 |
| TC-UI-09-03 | 复制多行消息写入剪贴板 | AC-UI-09-3 | Playwright | P0 |
| TC-UI-09-04 | 失败消息菜单直接重发 | AC-UI-09-4 | Playwright | P0 |
| TC-UI-09-05 | 顶部消息菜单出现在下方且在视口内 | AC-UI-09-6 | Playwright | P1 |
| TC-UI-09-06 | custom 消息长按无菜单 | AC-UI-09-7 | Playwright | P0 |
| TC-UI-09-07 | 剪贴板写入被拒绝时提示手动复制 | AC-UI-09-8 | 前端单测 | P0 |
| TC-UI-10-01 | 输入法组字期间 Enter 不发送 | AC-UI-10-2 | 前端单测 | P0 |
| TC-UI-10-02 | 输入框最多增长到 5 行 | AC-UI-10-3 | Playwright | P1 |
| TC-UI-10-03 | 超过 5000 码点显示已超出并禁用 | AC-UI-10-4 | Playwright | P0 |
| TC-UI-10-04 | 被移出群后输入框不存在 | AC-UI-10-5 | 前端单测 | P0 |
| TC-UI-10-05 | 输入框计算字号 ≥ 16px | AC-UI-10-6 | Playwright | P1 |
| TC-UI-10-06 | Safari 拼音输入回车选词不发送 | AC-UI-10-7 | 手工 | P0 |
| TC-UI-10-07 | send 返回 ok:false 时不清空输入框 | AC-UI-10-8 | 前端单测 | P0 |
| TC-UI-12-01 | 群聊信息页显示群名和群成员数 | AC-UI-12-1 | Playwright | P0 |
| TC-UI-12-02 | 单聊信息页显示账号并可进入资料卡 | AC-UI-12-2 | Playwright | P0 |
| TC-UI-12-03 | M3 群聊信息页没有任何操作入口 | AC-UI-12-3 | 前端单测 | P0 |
| TC-UI-12-04 | 被移出后打开聊天信息页 | AC-UI-12-5 | 前端单测 | P0 |
| TC-UI-13-01 | 通讯录按字母分组并显示人数和群聊入口 | AC-UI-13-1 | 前端单测 | P0 |
| TC-UI-13-02 | 陌生人会话和自己不出现在通讯录 | AC-UI-13-2 | Playwright | P0 |
| TC-UI-13-03 | 数字和 emoji 开头归入"#"组并排最后 | AC-UI-13-3 | 前端单测 | P0 |
| TC-UI-13-04 | 停在通讯录时对方同意申请，2 秒内出现 | AC-UI-13-4 | 端到端 | P0 |
| TC-UI-13-05 | 没有好友时的空状态 | AC-UI-13-5 | 前端单测 | P0 |
| TC-UI-13-06 | 未读申请数显示在入口和标签角标 | AC-UI-13-6 | Playwright | P0 |
| TC-UI-13-07 | 设置备注后移组 | AC-UI-13-7 | 前端单测 | P0 |
| TC-UI-13-08 | 拉黑的好友仍在列表并显示标签 | AC-UI-13-8 | 前端单测 | P0 |
| TC-UI-13-09 | pinyin-groups.json 在 Chromium 和 WebKit 一致 | AC-UI-13-9 | Playwright | P0 |
| TC-UI-13-10 | 不支持拼音排序时降级到"#" | AC-UI-13-10 | Playwright | P0 |
| TC-UI-13-11 | 加入分组代码后首屏 JS 不超过 200KB 且无拼音库 | AC-UI-13-11 | 前端单测 | P0 |
| TC-UI-14-01 | 从单聊头像进入好友资料卡并返回聊天 | AC-UI-14-1 | Playwright | P0 |
| TC-UI-14-02 | 自己的资料卡没有操作按钮 | AC-UI-14-2 | Playwright | P0 |
| TC-UI-14-03 | 备注、昵称、账号三行 | AC-UI-14-3 | 前端单测 | P0 |
| TC-UI-14-04 | 资料卡加入黑名单全流程 | AC-UI-14-4 | 端到端 | P0 |
| TC-UI-14-05 | 删除朋友后停留资料卡，会话保留 | AC-UI-14-5 | 端到端 | P0 |
| TC-UI-14-06 | 删除确认框正文随模式变化 | AC-UI-14-6 | 前端单测 | P0 |
| TC-UI-14-07 | relation=none 的按钮随模式变化 | AC-UI-14-7 | 前端单测 | P0 |
| TC-UI-14-08 | 拉黑的非好友不显示添加到通讯录 | AC-UI-14-8 | 前端单测 | P0 |
| TC-UI-14-09 | relation 按钮矩阵 | AC-UI-14-2、AC-UI-14-7、AC-UI-14-8 | 前端单测 | P1 |
| TC-UI-14-10 | 通过验证后变为好友 | AC-UI-14-9 | 端到端 | P0 |
| TC-UI-14-11 | 不可见用户与不存在用户的资料卡 DOM 相同 | AC-UI-14-10 | Playwright | P0 |
| TC-UI-14-12 | 从群聊进入发申请带 source=group | AC-UI-14-11 | Playwright | P0 |
| TC-UI-15-01 | "我"页账号与复制 | AC-UI-15-1 | Playwright | P0 |
| TC-UI-15-02 | "我"页显示名来自演示登录昵称 | AC-UI-15-2 | 端到端 | P0 |
| TC-UI-15-03 | 演示会话"我"页没有编辑入口，账号会话有 | AC-UI-15-3 | 前端单测 | P0 |
| TC-UI-16-01 | 退出确认框点取消不断开 | AC-UI-16-1 | Playwright | P0 |
| TC-UI-16-02 | 有失败消息时退出提示条数 | AC-UI-16-2 | Playwright | P0 |
| TC-UI-16-03 | pendingCount 统计与事件 | AC-UI-16-2 | 前端单测 | P0 |
| TC-UI-16-04 | 确认退出后 sessionStorage 无 token | AC-UI-16-3 | Playwright | P0 |
| TC-UI-16-05 | 关于天听版本号与 package.json 一致 | AC-UI-16-4 | Playwright | P1 |
| TC-UI-16-06 | 设置页顺序：演示会话没有"账号安全"，账号会话有 | AC-UI-16-6 | 前端单测 | P0 |
| TC-UI-17-01 | 离线时状态条显示且不遮挡消息 | AC-UI-17-1 | Playwright | P0 |
| TC-UI-18-01 | 被踢弹窗不能用 Esc、遮罩、后退关闭 | AC-UI-18-1 | Playwright | P0 |
| TC-UI-18-02 | tooManyConnections 文案 | AC-UI-18-2 | 前端单测 | P0 |
| TC-UI-18-03 | 被踢弹窗文案表与关闭码兜底 | AC-UI-18-2 | 前端单测 | P1 |
| TC-UI-18-04 | 点确定后社交状态清空 | AC-UI-18-3 | Playwright | P0 |
| TC-UI-19-01 | 同时只有一条轻提示，2 秒后消失 | AC-UI-19-1 | 前端单测 | P0 |
| TC-UI-19-02 | 不展示服务端 msg | AC-UI-19-2 | Playwright | P0 |
| TC-UI-19-03 | errors.json 每个 ws 码都有文案，命令+码表一致 | AC-UI-19-3 | 前端单测 | P0 |
| TC-UI-19-04 | friendApply 与 send 的 4292 文案不同 | AC-UI-19-4 | 前端单测 | P0 |
| TC-UI-19-05 | 4001 与 4008 文案 | AC-UI-19-5 | 前端单测 | P0 |
| TC-UI-19-06 | 非 send 命令 4290 与未定义码兜底 | AC-UI-19-4、AC-UI-19-5 | 前端单测 | P1 |
| TC-UI-20-01 | convs 延迟期间骨架屏，空列表后显示空状态 | AC-UI-20-1 | Playwright | P0 |
| TC-UI-20-02 | 首轮 socialSync 期间通讯录骨架屏 | AC-UI-20-2 | Playwright | P0 |
| TC-UI-20-03 | 三个列表的空状态文案 | AC-UI-20-3 | 前端单测 | P0 |
| TC-UI-21-01 | 占位头像首字 | AC-UI-21-1 | 前端单测 | P0 |
| TC-UI-21-02 | avatar.json 颜色下标与跨页面一致 | AC-UI-21-2 | 前端单测 | P0 |
| TC-UI-21-03 | 用户头像圆形、群头像圆角 10 | AC-UI-21-3 | Playwright | P0 |
| TC-UI-21-04 | 非 https、空串、404 回落占位且无错误 | AC-UI-21-4 | Playwright | P0 |
| TC-UI-21-05 | https 头像属性与 http 不发请求 | AC-UI-21-5 | Playwright | P0 |
| TC-UI-22-01 | 组件代码无颜色字面量 | AC-UI-22-3 | 前端单测 | P0 |
| TC-UI-22-02 | M3 构建在系统深色下仍为浅色 | AC-UI-22-4 | Playwright | P0 |
| TC-UI-23-01 | 总未读角标与页面标题 | AC-UI-23-1 | Playwright | P0 |
| TC-UI-23-02 | 99+ 与 0 时的标题 | AC-UI-23-2 | Playwright | P0 |
| TC-UI-23-03 | totalUnread 计算与事件 | AC-UI-23-1、AC-UI-23-2、AC-UI-23-3 | 前端单测 | P0 |
| TC-UI-23-04 | 打开会话后角标 2 秒内减少 | AC-UI-23-3 | Playwright | P0 |
| TC-UI-23-05 | 另一端进入新的朋友后红点消失 | AC-UI-23-4 | 端到端 | P0 |
| TC-UI-25-01 | 新的朋友列表顺序与第二行 | AC-UI-25-1 | 前端单测 | P0 |
| TC-UI-25-02 | 点同意后变为已添加，双方通讯录出现 | AC-UI-25-2 | 端到端 | P0 |
| TC-UI-25-03 | 申请详情来源与拒绝 | AC-UI-25-3 | 端到端 | P0 |
| TC-UI-25-04 | 发出的申请显示资料缓存或 userId | AC-UI-25-4 | Playwright | P0 |
| TC-UI-25-05 | 进入页面红点立即消失 | AC-UI-25-5 | Playwright | P0 |
| TC-UI-25-06 | 过期申请不显示同意且不计红点 | AC-UI-25-6 | 前端单测 | P0 |
| TC-UI-25-07 | friendRespond 4045 提示并变为已过期 | AC-UI-25-7 | Playwright | P0 |
| TC-UI-25-08 | 停在本页时新申请不产生角标 | AC-UI-25-8 | Playwright | P0 |
| TC-UI-26-01 | 搜索无关系用户显示添加到通讯录 | AC-UI-26-1 | 端到端 | P0 |
| TC-UI-26-02 | 关闭可被搜索与不存在的结果 DOM 相同 | AC-UI-26-2 | 端到端 | P0 |
| TC-UI-26-03 | 非法账号本地提示且不发请求 | AC-UI-26-3 | Playwright | P0 |
| TC-UI-26-04 | 四种 relation 的结果卡片 | AC-UI-26-4 | 前端单测 | P0 |
| TC-UI-26-05 | userSearch 4292 文案 | AC-UI-26-5 | Playwright | P0 |
| TC-UI-26-06 | 官方账号显示官方标识 | AC-UI-26-6 | 前端单测 | P0 |
| TC-UI-26-07 | 请求未返回时连按三次回车只发一次 | AC-UI-26-7 | Playwright | P0 |
| TC-UI-27-01 | 附言默认值与计数 | AC-UI-27-1 | Playwright | P0 |
| TC-UI-27-02 | 附言和备注发送后双方可见 | AC-UI-27-2 | 端到端 | P0 |
| TC-UI-27-03 | 附言 51、备注 33 码点禁用发送 | AC-UI-27-3 | 前端单测 | P0 |
| TC-UI-27-04 | 对方已申请过我时直接成为好友 | AC-UI-27-4 | 端到端 | P0 |
| TC-UI-27-05 | 不存在与拉黑了我的用户表现相同 | AC-UI-27-5 | 端到端 | P0 |
| TC-UI-27-06 | 直接打开为 other，从搜索进入为 search | AC-UI-27-6 | Playwright | P0 |
| TC-UI-27-07 | 请求未返回时连点三次只发一次 | AC-UI-27-7 | Playwright | P0 |
| TC-UI-27-08 | 申请自己时提示并返回 | AC-UI-27-8 | Playwright | P0 |
| TC-UI-28-01 | 备注多端同步 | AC-UI-28-1 | 端到端 | P0 |
| TC-UI-28-02 | 只有空格的备注清除备注 | AC-UI-28-2 | Playwright | P0 |
| TC-UI-28-03 | 备注超过 32 码点禁用完成 | AC-UI-28-3 | 前端单测 | P0 |
| TC-UI-28-04 | 未改动直接完成不发请求 | AC-UI-28-4 | Playwright | P0 |
| TC-UI-28-05 | friendRemark 4040 提示并返回 | AC-UI-28-5 | Playwright | P0 |
| TC-UI-29-01 | 黑名单倒序与设置页人数 | AC-UI-29-1 | 端到端 | P0 |
| TC-UI-29-02 | 移出黑名单多端同步后可私聊 | AC-UI-29-2 | 端到端 | P0 |
| TC-UI-29-03 | profile 为空时显示 userId 和占位头像 | AC-UI-29-3 | 前端单测 | P0 |
| TC-UI-30-01 | 关闭可被搜索后搜不到，打开后能搜到 | AC-UI-30-1 | 端到端 | P0 |
| TC-UI-30-02 | setPrivacy 4290 回滚 | AC-UI-30-2 | Playwright | P0 |
| TC-UI-30-03 | 请求期间开关禁用 | AC-UI-30-3 | Playwright | P0 |
| TC-UI-31-01 | 好友制下非好友发消息出现申请入口 | AC-UI-31-1 | 端到端 | P0 |
| TC-UI-31-02 | 被拉黑后发消息显示拒收提示 | AC-UI-31-2 | 端到端 | P0 |
| TC-UI-31-03 | 陌生人限额提示 | AC-UI-31-3 | 端到端 | P0 |
| TC-UI-31-04 | 连续 4035 只保留一条提示条 | AC-UI-31-4 | 前端单测 | P0 |
| TC-UI-31-05 | send 4292 只弹轻提示 | AC-UI-31-5 | Playwright | P0 |
| TC-UI-31-06 | 刷新后提示条和失败消息消失 | AC-UI-31-6 | Playwright | P0 |
| TC-UI-32-01 | 群聊列表排序、人数和总数 | AC-UI-32-1 | 前端单测 | P0 |
| TC-UI-32-02 | 600 个群全部显示 | AC-UI-32-2 | 端到端 | P1 |
| TC-UI-32-03 | myGroups 失败重试 | AC-UI-32-3 | Playwright | P0 |
| TC-UI-32-04 | 进入不在 convs 前 500 的群可收发 | AC-UI-32-4 | 端到端 | P1 |
| TC-UI-33-01 | 格式非法的 convId 跳回 /chats | AC-UI-33-1 | Playwright | P0 |
| TC-UI-33-02 | 非参与者的单聊本地拦截 | AC-UI-33-2 | Playwright | P0 |
| TC-UI-33-03 | 未聊过的单聊显示空聊天页并可发送 | AC-UI-33-3 | 端到端 | P0 |
| TC-UI-33-04 | 停在群聊天页时被管理接口移出 | AC-UI-33-4 | 端到端 | P0 |
| TC-UI-33-05 | 移出后从历史进入与刷新后打开 | AC-UI-33-5 | 端到端 | P0 |
| TC-UI-33-06 | 不存在的群显示群聊不存在或已解散 | AC-UI-33-6 | 端到端 | P0 |
| TC-UI-33-07 | 离线期间被移出，重连后生成快照 | AC-UI-33-9 | 端到端 | P0 |
| TC-UI-33-08 | 重连比对生成快照（SDK） | AC-UI-33-9 | 前端单测 | P0 |
| TC-UI-33-09 | truncated 时不推断移出 | AC-UI-33-9 | 前端单测 | P0 |
| TC-UI-33-10 | 刷新后打开已移出的群：sync 返回前不显示提示 | AC-UI-33-10 | 前端单测 | P0 |
| TC-UI-33-11 | loadHistory 对未知会话请求最新一页（SDK） | AC-UI-33-10 | 前端单测 | P0 |
| TC-UI-33-12 | 重新被拉进群后恢复 | AC-UI-33-11 | 端到端 | P0 |
| TC-UI-33-13 | 三种 reason 的文案 | AC-UI-33-12 | 前端单测 | P0 |
| TC-UI-33-14 | M3 构建收到 reason=left 按不认识处理 | AC-UI-33-12 | 前端单测 | P1 |
| TC-UI-33-15 | parseConvIdStrict 共享用例 | AC-UI-33-13 | 前端单测 | P0 |
| TC-UI-33-16 | 管理接口解散托管群 | AC-UI-33-14 | 端到端 | P0 |
| TC-UI-33-17 | 快照随退出登录清除，convAdded 清除快照（SDK） | AC-UI-33-11、AC-UI-33-4 | 前端单测 | P0 |
| TC-UI-33-18 | 被移出时已输入内容不保留 | AC-UI-33-4 | 前端单测 | P1 |
| TC-UI-90-01 | 昵称、群名、备注、附言中的 HTML 原样显示 | AC-UI-90-1 | Playwright | P0 |
| TC-UI-90-02 | 存储与 URL 中没有敏感数据 | AC-UI-90-2 | Playwright | P0 |
| TC-UI-90-03 | 对比度 CI 复算 6.2 节全部组合 | AC-UI-90-3 | 前端单测 | P0 |
| TC-UI-90-04 | 对比度 CI 发现低于要求的组合即失败 | AC-UI-90-3 | 前端单测 | P0 |
| TC-UI-90-05 | 清单中 token 名不存在时失败 | AC-UI-90-3 | 前端单测 | P0 |
| TC-UI-90-06 | axe 扫描无对比度和按钮名称违规 | AC-UI-90-4 | Playwright | P0 |
| TC-UI-90-07 | 品牌色与自己气泡色相检查 | AC-UI-90-5 | 前端单测 | P0 |
| TC-UI-90-08 | 500 个会话渲染性能 | AC-UI-90-6 | Playwright | P1 |
| TC-UI-90-09 | 一条新消息只重渲染受影响的会话项 | AC-UI-90-7 | Playwright | P1 |
| TC-UI-90-10 | 引用稳定（SDK） | AC-UI-90-7 | 前端单测 | P0 |
| TC-UI-90-11 | 首屏 JS 体积 CI | AC-UI-90-8 | 前端单测 | P0 |
| TC-UI-90-12 | aria-live 播报合并 | AC-UI-90-11 | Playwright | P0 |
| TC-UI-90-13 | 生产构建控制台不打印消息、附言和 token | AC-UI-90-12 | Playwright | P0 |
| TC-UI-90-14 | 生产响应头有 CSP 且无违规 | AC-UI-90-13 | Playwright | P0 |
| TC-UI-90-15 | 减少动态效果时过渡时长为 0 | AC-UI-90-14 | Playwright | P1 |
| TC-UI-90-16 | 不出现微信品牌词 | AC-UI-90-15 | 前端单测 | P0 |

---

## F-UI-01 信息架构与导航（M3；宽屏导航栏 M4）

### TC-UI-01-01 已登录进入 /chats 显示底部三项导航，切换到"我"
- 关联 AC：AC-UI-01-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 打开 `/chats`
  2. 点击标签栏"我"
- 预期结果：
  1. 标签栏依次为"消息""通讯录""我"，"消息"为选中态（`color.brand`）
  2. 路由为 `/me`，"我"为选中态，"消息"变为 `color.text.secondary`

### TC-UI-01-02 未登录打开单聊深链接，登录后回到原会话
- 关联 AC：AC-UI-01-3
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录；alice、bob 已导入。
- 步骤：
  1. 打开 `/chat/c2c%3Aalice%3Abob`
  2. 在登录页以 alice 登录
- 预期结果：
  1. 跳转到 `/login?redirect=…`
  2. 登录成功后路由为 `/chat/c2c%3Aalice%3Abob`，显示与 bob 的聊天页

### TC-UI-01-03 已登录访问 /login 跳转到 /chats
- 关联 AC：AC-UI-01-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：地址栏打开 `/login`
- 预期结果：路由变为 `/chats`，不显示登录表单

### TC-UI-01-04 从聊天页后退恢复会话列表滚动位置
- 关联 AC：AC-UI-01-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。用 `seed` 造 30 个会话。
- 步骤：
  1. 在会话列表滚动到第 20 个会话
  2. 记录 `scrollTop`，点击第 20 个进入聊天页
  3. 执行 `page.goBack()`
- 预期结果：
  1. 回到 `/chats`
  2. 列表 `scrollTop` 与进入前之差 ≤ 72px

### TC-UI-01-05 二级页面隐藏底部标签栏
- 关联 AC：AC-UI-01-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：依次进入聊天页、聊天信息页、设置页、新的朋友、资料卡
- 预期结果：每个页面 `data-testid=tabbar` 都不存在或不可见；返回一级页后标签栏重新出现

### TC-UI-01-06 回跳地址校验：四种恶意 redirect 都回到 /chats
- 关联 AC：AC-UI-01-7
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录。
- 步骤：
  1. 分别打开 `/login?redirect=//evil.com`、`/login?redirect=https://evil.com`、`/login?redirect=/%5Cevil.com`、`/login?redirect=/not-a-route`
  2. 每次以 alice 登录
  3. `page.on('request')` 记录全部请求
- 预期结果：
  1. 四次登录后路由都是 `/chats`
  2. 请求记录中没有任何主机为 `evil.com` 的请求

### TC-UI-01-07 回跳地址校验函数单元测试
- 关联 AC：AC-UI-01-7
- 优先级：P0
- 层级：前端单测
- 前置条件：无。
- 步骤：对 `isSafeRedirect` 输入 `/chats`、`/chat/c2c%3Aa%3Ab`、`//evil.com`、`/\\evil.com`、`https://evil.com`、`javascript:alert(1)`、`/not-a-route`、空串
- 预期结果：前两个返回 true，其余返回 false

### TC-UI-01-08 点击已选中的"消息"标签回到顶部
- 关联 AC：AC-UI-01-8
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。30 个会话。
- 步骤：
  1. 滚动到第 20 个会话
  2. 点击"消息"标签
- 预期结果：1 秒内第一个会话项完全位于视口内（`getBoundingClientRect().top ≥ 列表顶`）

---

## F-UI-02 布局：窄屏与桌面居中（M3）、宽屏三栏（M4）

### TC-UI-02-01 320×568 下一级页面和主要二级页面无横向滚动
- 关联 AC：AC-UI-02-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录，视口 320×568。
- 步骤：
  1. 依次打开 `/chats`、`/contacts`、`/me`、聊天页、`/contacts/requests`、`/user/bob`、`/contacts/add`
  2. 每页读取 `document.documentElement.scrollWidth`
- 预期结果：每页 `scrollWidth ≤ 320`

### TC-UI-02-02 桌面视口容器居中 480px
- 关联 AC：AC-UI-02-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录，视口 1280×800（M3 构建）。
- 步骤：
  1. 打开 `/chats`
  2. 读取应用容器、底部标签栏的 `getBoundingClientRect` 和 `body` 背景色
- 预期结果：
  1. 容器宽 480（±1）、左边缘 x = 400（±1）
  2. 标签栏宽度等于容器宽
  3. 容器外背景为 `color.bg` `#F2F3F7`，容器两侧有 1px `color.divider` 边线

### TC-UI-02-03 iPhone 15 Safari 安全区
- 关联 AC：AC-UI-02-8
- 优先级：P0
- 层级：手工
- 前置条件：iPhone 15 真机或模拟器，iOS Safari，已登录。
- 步骤：
  1. 打开会话列表截图
  2. 打开聊天页、弹出键盘前后各截图
- 预期结果：
  1. 标签栏图标和文字、输入区按钮都在底部横条之上
  2. 顶栏不被刘海遮挡
  3. 截图附在测试报告

---

## F-UI-03 登录页（M3，细化 F-AUTH-04）

### TC-UI-03-01 用户名为空或全空格时登录按钮禁用
- 关联 AC：AC-UI-03-1
- 优先级：P0
- 层级：前端单测
- 前置条件：登录页组件，`VITE_DEMO_LOGIN=true`。
- 步骤：
  1. 用户名为空
  2. 输入"   "
  3. 输入"alice"
- 预期结果：
  1. 前两种情况"登录"按钮 `disabled`
  2. 输入 alice 后按钮可点

### TC-UI-03-02 回车登录只发一次请求并进入 /chats
- 关联 AC：AC-UI-03-2
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录。
- 步骤：
  1. 输入"alice"按 Enter
  2. 统计 `/demo/login` 请求次数
- 预期结果：
  1. 请求恰好 1 次
  2. 成功后路由为 `/chats`

### TC-UI-03-03 请求未返回时连点三次只发一次
- 关联 AC：AC-UI-03-3
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录；`page.route('/demo/login')` 挂起不返回。
- 步骤：输入"alice"，连续点击"登录" 3 次
- 预期结果：只发出 1 次 `/demo/login` 请求；按钮处于加载态且禁用

### TC-UI-03-04 演示登录未开启提示"登录服务未开启"
- 关联 AC：AC-UI-03-4
- 优先级：P0
- 层级：Playwright
- 前置条件：同源部署（Vite proxy）；im-server 未开启演示登录（`IM_DEMO_LOGIN=0`，AC-AUTH-04-3）。
- 步骤：输入"alice"点"登录"
- 预期结果：
  1. 提示"登录服务未开启"
  2. 停留在 `/login`

### TC-UI-03-05 登录请求 10 秒超时提示网络异常
- 关联 AC：AC-UI-03-5
- 优先级：P0
- 层级：Playwright
- 前置条件：`page.route('/demo/login')` 不返回；`page.clock` 控制时间。
- 步骤：
  1. 输入"alice"点"登录"
  2. 推进 9 秒
  3. 再推进 2 秒
- 预期结果：
  1. 9 秒时无提示、按钮仍为加载态
  2. 10 秒（±1）时提示"网络异常，请稍后重试"，按钮恢复可点

### TC-UI-03-06 登录页输入框计算字号 ≥ 16px
- 关联 AC：AC-UI-03-6
- 优先级：P1
- 层级：Playwright
- 前置条件：未登录。
- 步骤：读取用户名、昵称两个输入框的 `getComputedStyle().fontSize`
- 预期结果：两个值都 ≥ 16px

### TC-UI-03-07 以昵称"张三"登录后各处显示张三
- 关联 AC：AC-UI-03-7
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-ui G-UI-23（演示登录接受 `nickname`）的服务端实现；bob 已导入。
- 步骤：
  1. 以 `zhangsan`、昵称"张三"登录
  2. 打开"我"页
  3. zhangsan 给 bob 发 1 条单聊
  4. bob 登录查看会话列表
- 预期结果：
  1. zhangsan 的"我"页显示名为"张三"
  2. bob 的会话列表中该会话名为"张三"

### TC-UI-03-08 昵称超过 32 个字时提示并禁用
- 关联 AC：AC-UI-03-8
- 优先级：P0
- 层级：前端单测
- 前置条件：登录页组件。
- 步骤：
  1. 用户名填 alice，昵称输入 33 个汉字
  2. 删到 32 个
- 预期结果：
  1. 33 个时显示"昵称最多 32 个字"，"登录"禁用
  2. 32 个时提示消失、按钮可点

### TC-UI-03-10 登录页标题与品牌名
- 关联 AC：AC-UI-03-10
- 优先级：P0
- 层级：Playwright
- 前置条件：未登录。
- 步骤：打开 `/login`
- 预期结果：`document.title` 为"天听"；页面可见文字"天听"

---

## F-UI-04 会话列表（M3，细化 F-CONV-01 / F-CONV-02）

### TC-UI-04-01 群聊摘要带发送者前缀，自己发的不带
- 关联 AC：AC-UI-04-1
- 优先级：P0
- 层级：前端单测
- 前置条件：会话列表组件，`FakeTransport` 回放 `convs`：g1 的 `lastMsg` 由 bob 发出"晚上好"。
- 步骤：
  1. 渲染会话列表
  2. 推送自己在 g1 发出的"收到"的 ack
- 预期结果：
  1. g1 摘要为"bob: 晚上好"
  2. 之后摘要为"收到"

### TC-UI-04-02 单聊摘要不带发送者前缀
- 关联 AC：AC-UI-04-2
- 优先级：P0
- 层级：前端单测
- 前置条件：单聊 c2c:alice:bob 最后一条由 bob 发出"你好"。
- 步骤：渲染会话列表
- 预期结果：摘要为"你好"，没有"bob: "前缀

### TC-UI-04-03 没有消息的群不显示摘要和时间
- 关联 AC：AC-UI-04-3
- 优先级：P0
- 层级：前端单测
- 前置条件：g2 的 `lastMsg = null`。
- 步骤：渲染会话列表
- 预期结果：g2 项显示群名；摘要和时间元素不存在

### TC-UI-04-04 custom 消息摘要显示"[暂不支持的消息]"
- 关联 AC：AC-UI-04-4
- 优先级：P0
- 层级：前端单测
- 前置条件：最后一条消息 `type = custom`。
- 步骤：渲染会话列表
- 预期结果：摘要为"[暂不支持的消息]"

### TC-UI-04-05 超长摘要单行省略且项高 72
- 关联 AC：AC-UI-04-5
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 把某会话 `lastMsg.text` 改为 100 码点。
- 步骤：打开 `/chats`，测量该会话项
- 预期结果：
  1. 摘要元素 `scrollWidth > clientWidth` 且 `text-overflow: ellipsis`
  2. 会话项高度 72（±1）

### TC-UI-04-06 未读角标位置与 99+
- 关联 AC：AC-UI-04-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 改写 `convs`：会话 A unread=5，会话 B unread=120。
- 步骤：
  1. 打开 `/chats`
  2. 读取 A 的角标和头像矩形
  3. 读取 B 的角标文字
- 预期结果：
  1. A 角标文字"5"，中心点落在头像外接矩形右上四分之一区域内
  2. B 角标为"99+"

### TC-UI-04-07 truncated 时显示"仅显示最近 500 个会话"
- 关联 AC：AC-UI-04-7
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 回放 `convs` 两次：`truncated=true`、`truncated=false`。
- 步骤：分别渲染会话列表并滚动到底部
- 预期结果：
  1. `true` 时底部有"仅显示最近 500 个会话"
  2. `false` 时没有

### TC-UI-04-08 摘要前缀按备注 > 昵称显示
- 关联 AC：AC-UI-04-9
- 优先级：P0
- 层级：前端单测
- 前置条件：g1 最后一条消息由 carol 发出；`users` 缓存中 carol 昵称"小卡"；`friends` 中 carol 备注"卡姐"。
- 步骤：
  1. 渲染会话列表
  2. 推送 `friends` 回调：carol 备注清空
- 预期结果：
  1. g1 摘要前缀为"卡姐: "
  2. 之后变为"小卡: "

---

## F-UI-05 "+"菜单与发起单聊（M3，细化 F-CONV-03）

### TC-UI-05-01 + 菜单和空状态打开同一个发起单聊弹窗
- 关联 AC：AC-UI-05-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。会话列表为空。
- 步骤：
  1. 点空状态的"发起单聊"，记录弹窗 `data-testid`，关闭
  2. 点"+" → "发起单聊"
- 预期结果：两次打开的弹窗 `data-testid` 相同（`start-chat-dialog`），页面中同时只有 1 个

### TC-UI-05-02 开放模式下回车进入单聊
- 关联 AC：AC-UI-05-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。开放模式。
- 步骤：
  1. 打开发起单聊弹窗，输入为空时检查按钮
  2. 输入"bob"按 Enter
- 预期结果：
  1. 输入为空时"开始聊天"禁用
  2. 进入 `/chat/c2c%3Aalice%3Abob`，弹窗关闭

### TC-UI-05-03 输入自己的 userId 进入自聊，好友制也不提示
- 关联 AC：AC-UI-05-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。好友制（`routeWs` 改写 `socialSync` 应答头 `mode=friends`）。
- 步骤：弹窗输入"alice"点"开始聊天"
- 预期结果：不出现好友提示，直接进入 `/chat/c2c%3Aalice%3Aalice`

### TC-UI-05-04 + 菜单两项及顺序
- 关联 AC：AC-UI-05-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 点"+"
  2. 点"添加朋友"
- 预期结果：
  1. 菜单依次为"添加朋友""发起单聊"两项
  2. 路由为 `/contacts/add`

### TC-UI-05-05 好友制下发起非好友单聊的提示
- 关联 AC：AC-UI-05-6
- 优先级：P0
- 层级：前端单测
- 前置条件：组件测试：`socialSettings.mode=friends`；`relationOf('carol')='none'`、`relationOf('bob')='friend'`。
- 步骤：
  1. 输入"carol"点"开始聊天"
  2. 点"仍然发起"
  3. 重新打开弹窗输入"bob"点"开始聊天"
- 预期结果：
  1. 弹窗显示"对方不是你的好友，可能无法发送消息"和"添加朋友""仍然发起"两个按钮
  2. 导航到与 carol 的聊天页
  3. bob 直接进入聊天页，没有提示

### TC-UI-05-06 点遮罩关闭弹窗，路由不变
- 关联 AC：AC-UI-05-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 打开弹窗
  2. 点击遮罩
- 预期结果：弹窗关闭；路由仍为 `/chats`

---

## F-UI-06 聊天页结构（M3，细化 F-MSG-05 / F-MSG-06）

### TC-UI-06-01 群名过长时省略，成员数完整
- 关联 AC：AC-UI-06-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`seed`：群 g50 名为 50 个汉字、12 人；群 g2 名为"周末球局"、12 人。
- 步骤：
  1. 打开 g50 聊天页，读取标题
  2. 打开 g2 聊天页
- 预期结果：
  1. g50 群名元素有省略号（`scrollWidth > clientWidth`），"（12）"元素完整可见
  2. g2 标题文本为"周末球局（12）"

### TC-UI-06-02 memberCount 缺失时只显示群名
- 关联 AC：AC-UI-06-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 从 `convs` 应答中删除 g1 的 `memberCount` 字段。
- 步骤：打开 g1 聊天页
- 预期结果：标题只有群名，不出现"（）"或"（0）"

### TC-UI-06-03 100 条消息时进入聊天页贴底
- 关联 AC：AC-UI-06-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`seed` 会话有 100 条消息。
- 步骤：打开该会话
- 预期结果：最后一条消息底边 ≤ 输入区上边缘

### TC-UI-06-04 停在聊天页时未读不增加，离开后增加
- 关联 AC：AC-UI-06-4
- 优先级：P0
- 层级：Playwright
- 前置条件：alice、bob 两个浏览器上下文已登录。
- 步骤：
  1. alice 打开与 bob 的聊天页
  2. bob 发 1 条
  3. alice 返回会话列表
  4. bob 再发 1 条
- 预期结果：
  1. 第一条到达后 alice 该会话未读为 0
  2. 第二条到达后未读为 1

### TC-UI-06-05 再次进入群聊天页 3 秒内刷新成员数
- 关联 AC：AC-UI-06-6
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social 服务端 `memberCount` 维护（G-UI-02）和管理接口加成员；g1 有 5 人。
- 步骤：
  1. alice 打开 g1（标题"（5）"）后返回会话列表
  2. 管理接口给 g1 加 2 人
  3. 等 30 秒后 alice 再次进入 g1
- 预期结果：3 秒内标题变为"（7）"

### TC-UI-06-06 refreshConversations 30 秒节流
- 关联 AC：AC-UI-06-6
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeClock`。
- 步骤：
  1. 调用 `refreshConversations()`
  2. 10 秒后再调
  3. 推进到 30 秒后再调
- 预期结果：
  1. 第一次发 `convs`
  2. 第二次不发帧
  3. 第三次发 `convs`

### TC-UI-06-07 进入聊天信息页时 setActiveConversation(null)
- 关联 AC：AC-UI-06-7
- 优先级：P0
- 层级：前端单测
- 前置条件：聊天页组件，注入记录调用的 SDK 桩。
- 步骤：
  1. 进入聊天页 A
  2. 进入聊天信息页
  3. `FakeTransport` 推 A 的新消息
  4. 返回聊天页
- 预期结果：
  1. 调用顺序为 `setActiveConversation(A)`、`(null)`、`(A)`
  2. 在信息页期间 A 的未读加 1

---

## F-UI-07 消息气泡与时间分隔（M3，细化 F-MSG-06 / F-GRP-02）

### TC-UI-07-01 左右气泡与列表边距 60px
- 关联 AC：AC-UI-07-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。单聊里双方各有 1 条消息。
- 步骤：测量自己气泡右边缘与列表右边缘、对方气泡左边缘与列表左边缘的距离
- 预期结果：两个距离都为 60px（±1）

### TC-UI-07-02 群聊显示发送者显示名，单聊和自己不显示
- 关联 AC：AC-UI-07-2
- 优先级：P0
- 层级：前端单测
- 前置条件：g1 中 bob 与我各 1 条；`users` 缓存中 bob 无昵称；之后更新为"阿博"；另有单聊。
- 步骤：
  1. 渲染 g1 消息列表
  2. 更新 `users` 缓存 bob 昵称为"阿博"
  3. 渲染单聊
- 预期结果：
  1. bob 消息上方为"bob"，自己的消息上方无名字
  2. 变为"阿博"
  3. 单聊双方都无名字

### TC-UI-07-03 超长消息气泡宽度 ≤ 70%
- 关联 AC：AC-UI-07-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一条 2000 个汉字的消息。
- 步骤：测量气泡宽度与消息列表宽度
- 预期结果：气泡宽 ≤ 列表宽 × 0.7 + 1px

### TC-UI-07-04 custom 消息显示占位且无 console error
- 关联 AC：AC-UI-07-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 推送一条 `type=custom` 的消息。
- 步骤：打开该会话，收集 `console` 事件
- 预期结果：显示"[暂不支持的消息]"（`color.text.secondary`）；没有 error 级日志

### TC-UI-07-05 URL 与 script 原样显示为文本
- 关联 AC：AC-UI-07-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：收到内容 `https://example.com/a?b=<script>` 的消息
- 预期结果：气泡文本与原文相同；`page.locator('.message a, .message script')` 数量为 0

### TC-UI-07-06 自己气泡文字对比度 ≥ 4.5
- 关联 AC：AC-UI-07-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：读取自己气泡的计算 `color` 和 `background-color`，按 WCAG 公式计算
- 预期结果：对比度 ≥ 4.5（期望 5.38，6.2 节 L16）

### TC-UI-07-07 点击群聊中 bob 的头像进入资料卡
- 关联 AC：AC-UI-07-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。g1 中有 bob 的消息。
- 步骤：点击 bob 消息左侧头像
- 预期结果：路由为 `/user/bob`

---

## F-UI-08 发送状态与失败重发（M3，细化 F-MSG-03）

### TC-UI-08-01 ack 延迟期间显示转圈，到达后消失
- 关联 AC：AC-UI-08-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让该条 `send` 的 ack 延迟 3 秒。
- 步骤：
  1. 发送"hi"
  2. 1 秒时检查
  3. ack 到达后检查
- 预期结果：
  1. 1 秒时消息 `data-status=sending`，气泡左侧有 `loader-circle` 图标
  2. 到达后 `data-status=sent`，图标不存在

### TC-UI-08-02 点"!"确认后重发沿用 clientMsgId
- 关联 AC：AC-UI-08-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让第一次 `send` 返回 5000。
- 步骤：
  1. 发送"hi"失败后点"!"
  2. 点"取消"，等待 2 秒
  3. 再点"!"并点"重发"
- 预期结果：
  1. 出现"重发该消息？"确认框
  2. 状态仍为 failed，2 秒内无 `send` 帧
  3. 发出的 `send` 帧 `clientMsgId` 与第一次相同

### TC-UI-08-03 4290 提示恰好一次，超时不提示
- 关联 AC：AC-UI-08-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. `routeWs` 让 `send` 返回 4290，发送 1 条
  2. `routeWs` 吞掉 ack，再发 1 条并等 11 秒
- 预期结果：
  1. 出现"发送太频繁，请稍后再试"恰好 1 次
  2. 超时后消息失败，期间没有轻提示

### TC-UI-08-04 sendFailed 事件每次失败恰好一次
- 关联 AC：AC-UI-08-3、AC-UI-08-5
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`、`FakeClock`。
- 步骤：
  1. 发送后回 4290
  2. `retry` 后再回 4290
  3. 再发一条不回 ack，推进 10 秒
- 预期结果：`sendFailed` 共 3 次：两次 `code=4290`、一次 `timeout`；期间 `conversations` 回调多次也不重复

### TC-UI-08-05 失败图标点击区域 ≥ 44×44
- 关联 AC：AC-UI-08-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一条失败消息。
- 步骤：读取"!"按钮的 `getBoundingClientRect`
- 预期结果：宽、高都 ≥ 44

### TC-UI-08-06 两次 4290 失败共提示两次，切页不额外提示
- 关联 AC：AC-UI-08-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `send` 两次都返回 4290。
- 步骤：
  1. 发送失败
  2. 重发再次失败
  3. 返回会话列表再进入聊天页
- 预期结果：共出现 2 次"发送太频繁，请稍后再试"，切页后没有第 3 次

---

## F-UI-09 消息操作菜单（M3；删除失败消息 M4）

### TC-UI-09-01 长按 500ms 弹出菜单，短按和移动不弹出
- 关联 AC：AC-UI-09-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一条已发送消息。第 1～3 步 C4 开关关闭（`IM_MSG_ACTIONS_ENABLED=0`，`features.msgActions=false`）；第 4 步开关开启（`IM_MSG_ACTIONS_ENABLED=1`，配好 `IM_QUARANTINE_KEY`），alice 1 分钟内刚发的一条文本消息。（PRD-ui v1.4 改写 AC-UI-09-1）
- 步骤：
  1. CDP 长按 500ms
  2. 关闭后按下 300ms 松开
  3. 按下后移动 15px 再保持 600ms
  4. 开关开启后重新登录，长按那条刚发的消息 500ms
- 预期结果：
  1. 第一次弹出菜单且只有"复制"
  2. 第二次不弹出
  3. 第三次不弹出
  4. 菜单按消息状态：有"复制""引用""撤回"（PRD-ui F-UI-09 规则 3、4；"举报"随 C6）

### TC-UI-09-02 长按状态机单元测试
- 关联 AC：AC-UI-09-1
- 优先级：P0
- 层级：前端单测
- 前置条件：消息组件，假定时器。
- 步骤：
  1. `pointerdown` → 推进 499ms → `pointerup`
  2. `pointerdown` → 推进 500ms
  3. `pointerdown` → `pointermove` 11px → 推进 600ms
- 预期结果：
  1. 不触发
  2. 触发
  3. 不触发

### TC-UI-09-03 复制多行消息写入剪贴板
- 关联 AC：AC-UI-09-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。消息内容"第一行\n第二行"；Chromium 授予 `clipboard-read`；WebKit 用桩替换 `navigator.clipboard.writeText`。
- 步骤：长按消息 → "复制"
- 预期结果：
  1. Chromium：读取剪贴板等于"第一行\n第二行"；WebKit：桩函数参数相同
  2. 出现"已复制"轻提示

### TC-UI-09-04 失败消息菜单直接重发
- 关联 AC：AC-UI-09-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一条失败消息。
- 步骤：
  1. 长按失败消息
  2. 点"重发"
- 预期结果：
  1. 菜单包含"复制""重发"
  2. 不弹确认框，直接发出 `send`，`clientMsgId` 不变

### TC-UI-09-05 顶部消息菜单出现在下方且在视口内
- 关联 AC：AC-UI-09-6
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。列表滚动使某条消息贴近消息列表顶边。
- 步骤：长按该消息
- 预期结果：菜单顶边 ≥ 气泡底边；菜单矩形完全在视口内

### TC-UI-09-06 custom 消息长按无菜单
- 关联 AC：AC-UI-09-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一条 custom 消息。
- 步骤：长按 600ms
- 预期结果：不出现菜单

### TC-UI-09-07 剪贴板写入被拒绝时提示手动复制
- 关联 AC：AC-UI-09-8
- 优先级：P0
- 层级：前端单测
- 前置条件：`navigator.clipboard.writeText` 桩抛错。
- 步骤：长按 → "复制"
- 预期结果：提示"复制失败，请长按文字手动复制"

---

## F-UI-10 输入区（M3，细化 F-MSG-01 / F-MSG-06）

### TC-UI-10-01 输入法组字期间 Enter 不发送
- 关联 AC：AC-UI-10-2
- 优先级：P0
- 层级：前端单测
- 前置条件：输入区组件，注入 SDK 桩记录 `send`。
- 步骤：
  1. `compositionstart` 后 `keydown Enter (isComposing=true)`
  2. `keydown Enter (keyCode=229)`
  3. `compositionend` 后 30ms 发 Enter
  4. `compositionend` 后 80ms 发 Enter
- 预期结果：
  1. 前三次 `send` 未调用
  2. 第四次调用 1 次

### TC-UI-10-02 输入框最多增长到 5 行
- 关联 AC：AC-UI-10-3
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：连续输入 8 行文本
- 预期结果：高度增长到 5 行后不再变化；输入框 `scrollHeight > clientHeight`，可滚动

### TC-UI-10-03 超过 5000 码点显示已超出并禁用
- 关联 AC：AC-UI-10-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 粘贴 5003 个非空白码点
  2. 按 Enter
  3. 删除 3 个字符
- 预期结果：
  1. 显示"已超出 3 字"（`color.danger`），"发送"禁用
  2. 没有 `send` 帧
  3. 提示消失，按钮可点

### TC-UI-10-04 被移出群后输入框不存在
- 关联 AC：AC-UI-10-5
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 推送 `convRemoved {convId:'group:g1'}`，停在 g1 聊天页。
- 步骤：渲染聊天页
- 预期结果：输入框不存在，显示"你已不在该群，无法发送消息"

### TC-UI-10-05 输入框计算字号 ≥ 16px
- 关联 AC：AC-UI-10-6
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：读取聊天输入框 `fontSize`
- 预期结果：≥ 16px

### TC-UI-10-06 Safari 拼音输入回车选词不发送
- 关联 AC：AC-UI-10-7
- 优先级：P0
- 层级：手工
- 前置条件：macOS Safari、iOS Safari 最新版，系统简体拼音输入法。
- 步骤：
  1. 在聊天输入框输入"nihao"
  2. 按回车选词
  3. 再按一次回车
- 预期结果：
  1. 只上屏"你好"，未发送
  2. 第二次回车发送"你好"
  3. 两个浏览器各录屏一次

### TC-UI-10-07 send 返回 ok:false 时不清空输入框
- 关联 AC：AC-UI-10-8
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport`，SDK `send` 返回 `{ok:false, reason:'tooLong'}`。
- 步骤：输入"abc"点"发送"
- 预期结果：输入框内容仍为"abc"

---

## F-UI-12 聊天信息页（M3 只读；M4 由 F-UI-36 扩展）

### TC-UI-12-01 群聊信息页显示群名和群成员数
- 关联 AC：AC-UI-12-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。g1 有 5 人。
- 步骤：打开 g1 聊天页，点顶栏"更多"
- 预期结果：进入 `/chat/group%3Ag1/info`，显示群名和"群成员（5）"，与顶栏"（5）"一致

### TC-UI-12-02 单聊信息页显示账号并可进入资料卡
- 关联 AC：AC-UI-12-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 打开与 bob 的聊天信息页
  2. 点击头部
- 预期结果：
  1. 显示 bob 的显示名和"账号：bob"
  2. 路由为 `/user/bob`

### TC-UI-12-03 M3 群聊信息页没有任何操作入口
- 关联 AC：AC-UI-12-3
- 优先级：P0
- 层级：前端单测
- 前置条件：M3 构建；聊天信息页组件，g1 普通群。
- 步骤：渲染 g1 聊天信息页
- 预期结果：页面中没有"修改群名""添加成员""退出群聊""解散"等按钮，也没有成员头像网格；SDK 桩没有收到任何 `group*` 调用

### TC-UI-12-04 被移出后打开聊天信息页
- 关联 AC：AC-UI-12-5
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 推送 `convRemoved {convId:'group:g1'}`（本次登录期间）。
- 步骤：打开 `/chat/group%3Ag1/info`
- 预期结果：显示快照中的群名和"你已不在该群"，不显示成员数

---

## F-UI-13 通讯录（M3，细化 PRD-social F-SOC-06 / F-SOC-11）

### TC-UI-13-01 通讯录按字母分组并显示人数和群聊入口
- 关联 AC：AC-UI-13-1
- 优先级：P0
- 层级：前端单测
- 前置条件：通讯录组件；`friends`：bob、alice、zhangsan（昵称"张三"）；`loadMyGroups` 返回 g1、g2。
- 步骤：
  1. 渲染 `/contacts`
  2. 点"群聊"
- 预期结果：
  1. 分组依次为 A（alice）、B（bob）、Z（张三），底部"3 位朋友"
  2. 群聊列表显示 2 个群

### TC-UI-13-02 陌生人会话和自己不出现在通讯录
- 关联 AC：AC-UI-13-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。开放模式；alice 与陌生人 carol 有单聊，给自己发过消息；alice 没有好友。
- 步骤：打开 `/contacts`
- 预期结果：好友列表中没有 carol，也没有 alice

### TC-UI-13-03 数字和 emoji 开头归入"#"组并排最后
- 关联 AC：AC-UI-13-3
- 优先级：P0
- 层级：前端单测
- 前置条件：好友显示名"123""😀abc""bob"。
- 步骤：渲染通讯录
- 预期结果："123""😀abc"在"#"组；"#"组排在 B 组之后

### TC-UI-13-04 停在通讯录时对方同意申请，2 秒内出现
- 关联 AC：AC-UI-13-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端（F-SOC-02、F-SOC-03、F-SOC-06 关系同步与 `socialChanged`）；alice 已向 carol 发出申请。
- 步骤：
  1. alice 停在 `/contacts`
  2. carol 登录后同意申请
- 预期结果：2 秒内 carol 出现在 alice 通讯录的 C 组

### TC-UI-13-05 没有好友时的空状态
- 关联 AC：AC-UI-13-5
- 优先级：P0
- 层级：前端单测
- 前置条件：`friends` 为空。
- 步骤：渲染通讯录
- 预期结果：好友区显示"还没有朋友"和"添加朋友"按钮；"新的朋友""群聊"入口仍在

### TC-UI-13-06 未读申请数显示在入口和标签角标
- 关联 AC：AC-UI-13-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 回放 `socialSync`：3 条 `in`、`pending`、未读申请。
- 步骤：打开 `/contacts`
- 预期结果："新的朋友"右侧显示"3"；"通讯录"标签角标显示"3"

### TC-UI-13-07 设置备注后移组
- 关联 AC：AC-UI-13-7
- 优先级：P0
- 层级：前端单测
- 前置条件：好友 bob（无昵称）。
- 步骤：推送 `friends` 回调：bob 备注"阿博"
- 预期结果：bob 显示为"阿博"并出现在 A 组，B 组中没有 bob

### TC-UI-13-08 拉黑的好友仍在列表并显示标签
- 关联 AC：AC-UI-13-8
- 优先级：P0
- 层级：前端单测
- 前置条件：好友 bob；`blocks` 含 bob。
- 步骤：渲染通讯录
- 预期结果：bob 仍在 B 组，名字旁有"已拉黑"

### TC-UI-13-09 pinyin-groups.json 在 Chromium 和 WebKit 一致
- 关联 AC：AC-UI-13-9
- 优先级：P0
- 层级：Playwright
- 前置条件：`protocol/vectors/pinyin-groups.json`（含张三→Z、李四→L、王五→W、阿强→A、欧阳→O、白→B、Alice→A、zoe→Z、123→#、😀→#）。
- 步骤：在 Chromium、WebKit 两个项目中各执行一次分组函数遍历全部用例
- 预期结果：两个浏览器中每条用例的分组字母都与期望一致

### TC-UI-13-10 不支持拼音排序时降级到"#"
- 关联 AC：AC-UI-13-10
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`addInitScript` 用桩替换 `Intl.Collator`，`resolvedOptions().collation` 返回 `default`；好友"张三""alice"。
- 步骤：打开 `/contacts`，收集 `console`
- 预期结果：通讯录正常显示；"张三"在"#"组，alice 在 A 组；没有 error

### TC-UI-13-11 加入分组代码后首屏 JS 不超过 200KB 且无拼音库
- 关联 AC：AC-UI-13-11
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 生产构建。
- 步骤：
  1. 运行包体积脚本（gzip 统计首屏 chunk）
  2. 检查 `package-lock.json` 与构建产物依赖
- 预期结果：
  1. 首屏 JS（gzip）≤ 200KB
  2. 依赖中没有 `pinyin`、`pinyin-pro` 等拼音库

---

## F-UI-14 资料卡（M3，细化 F-USER-02、PRD-social F-SOC-05 / 07 / 13）

### TC-UI-14-01 从单聊头像进入好友资料卡并返回聊天
- 关联 AC：AC-UI-14-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。bob 是好友。
- 步骤：
  1. 在与 bob 的聊天页点 bob 头像
  2. 点"发消息"
- 预期结果：
  1. 进入 `/user/bob`，主按钮为"发消息"
  2. 回到 `/chat/c2c%3Aalice%3Abob`

### TC-UI-14-02 自己的资料卡没有操作按钮
- 关联 AC：AC-UI-14-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。g1 中有自己的消息。
- 步骤：点击自己的头像
- 预期结果：进入 `/user/alice`，页面没有任何操作按钮和"更多"菜单

### TC-UI-14-03 备注、昵称、账号三行
- 关联 AC：AC-UI-14-3
- 优先级：P0
- 层级：前端单测
- 前置条件：`friends` 中 bob：`profile.nickname="阿博"`、`remark="老板"`。
- 步骤：渲染 `/user/bob`
- 预期结果：标题"老板"，第二行"昵称：阿博"，第三行"账号：bob"

### TC-UI-14-04 资料卡加入黑名单全流程
- 关联 AC：AC-UI-14-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端拉黑（F-SOC-07）；alice 与 bob 是好友。
- 步骤：
  1. 在 bob 资料卡点"更多 → 加入黑名单"
  2. 确认
  3. 打开 `/me/settings/blocks`
- 预期结果：
  1. 确认框正文为"加入黑名单后，你将不再收到对方的私聊消息和好友申请；在共同的群聊里仍能看到对方的发言"
  2. 资料卡显示"已加入黑名单"，"更多"菜单变为"移出黑名单"
  3. 黑名单中有 bob

### TC-UI-14-05 删除朋友后停留资料卡，会话保留
- 关联 AC：AC-UI-14-5
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端删除好友（F-SOC-05）；alice 与 bob 是好友且有单聊。
- 步骤：
  1. 在 bob 资料卡点"更多 → 删除朋友"并确认
  2. 打开通讯录和会话列表
- 预期结果：
  1. 停留在 `/user/bob`，按钮变为"添加到通讯录"
  2. 通讯录中没有 bob；会话列表中与 bob 的会话仍在

### TC-UI-14-06 删除确认框正文随模式变化
- 关联 AC：AC-UI-14-6
- 优先级：P0
- 层级：前端单测
- 前置条件：好友 bob；分别设置 `socialSettings.mode=friends` 和 `open`。
- 步骤：点"删除朋友"打开确认框
- 预期结果：好友制下正文包含"删除后你们将无法继续私聊"；开放模式下不包含

### TC-UI-14-07 relation=none 的按钮随模式变化
- 关联 AC：AC-UI-14-7
- 优先级：P0
- 层级：前端单测
- 前置条件：`relationOf('carol')='none'`。
- 步骤：
  1. 开放模式渲染 `/user/carol`
  2. 好友制渲染
- 预期结果：
  1. 开放模式显示"添加到通讯录"和"发消息"
  2. 好友制只显示"添加到通讯录"

### TC-UI-14-08 拉黑的非好友不显示添加到通讯录
- 关联 AC：AC-UI-14-8
- 优先级：P0
- 层级：前端单测
- 前置条件：`relationOf('eve')='blocked'`，eve 不是好友。
- 步骤：渲染 `/user/eve`
- 预期结果：显示"已加入黑名单"；没有"添加到通讯录"

### TC-UI-14-09 relation 按钮矩阵
- 关联 AC：AC-UI-14-2、AC-UI-14-7、AC-UI-14-8
- 优先级：P1
- 层级：前端单测
- 前置条件：资料卡组件。
- 步骤：遍历 relation（self / friend / blocked 且好友 / blocked 非好友 / pendingIn / pendingOut / none）× 模式（open / friends）× 官方账号（是 / 否）
- 预期结果：每个组合的主要按钮、其他按钮和"更多"菜单项与 F-UI-14 规则 3 的表一致（快照断言）

### TC-UI-14-10 通过验证后变为好友
- 关联 AC：AC-UI-14-9
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端（F-SOC-03）；dave 已给 alice 发申请。
- 步骤：打开 `/user/dave`，点"通过验证"
- 预期结果：2 秒内按钮变为"发消息"；通讯录中出现 dave

### TC-UI-14-11 不可见用户与不存在用户的资料卡 DOM 相同
- 关联 AC：AC-UI-14-10
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。frank 存在但与 alice 无任何关系（`users` 不返回）；nobody 不存在。
- 步骤：
  1. 打开 `/user/frank`，保存资料卡区域 `innerHTML`
  2. 打开 `/user/nobody`
- 预期结果：
  1. frank 页显示"frank"、占位头像和"添加到通讯录"，没有"用户不存在"
  2. 两页 `innerHTML` 去掉 `data-*` 属性并把用户 ID 文字替换为占位符后相等

### TC-UI-14-12 从群聊进入发申请带 source=group
- 关联 AC：AC-UI-14-11
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。g1 中有 carol 的消息，carol 与 alice 无关系。
- 步骤：在 g1 点 carol 头像 → "添加到通讯录" → "发送"
- 预期结果：发出的 `friendApply` 帧 `source="group"`、`sourceRef="g1"`

---

## F-UI-15 我（M3）

### TC-UI-15-01 "我"页账号与复制
- 关联 AC：AC-UI-15-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：打开 `/me`，点账号旁"复制"
- 预期结果：显示"账号：alice"；剪贴板内容为"alice"（Chromium 读取 / WebKit 桩）；出现"已复制"

### TC-UI-15-02 "我"页显示名来自演示登录昵称
- 关联 AC：AC-UI-15-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 G-UI-23 演示登录 `nickname` 与 PRD-social `users`（F-SOC-10）服务端。
- 步骤：
  1. 以 `zhangsan`、昵称"张三"登录，打开"我"
  2. 退出后以 alice、不填昵称登录（alice 没有昵称），打开"我"
- 预期结果：
  1. 显示名为"张三"
  2. 显示名为"alice"

### TC-UI-15-03 演示会话"我"页没有编辑入口，账号会话有
- 关联 AC：AC-UI-15-3
- 优先级：P0
- 层级：前端单测
- 前置条件："我"页组件；分别用 `mode=demo`、`mode=account` 的会话。
- 步骤：分别渲染 `/me`
- 预期结果：演示会话卡片没有"编辑"箭头，页面中没有昵称或头像的编辑控件；账号会话有编辑入口

---

## F-UI-16 设置（M3；外观 M4）

### TC-UI-16-01 退出确认框点取消不断开
- 关联 AC：AC-UI-16-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 打开 `/me/settings` 点"退出登录"
  2. 点"取消"，监听 WebSocket `close` 2 秒
- 预期结果：
  1. 出现"确定退出登录？"确认框
  2. 停留在设置页，2 秒内没有 WebSocket close

### TC-UI-16-02 有失败消息时退出提示条数
- 关联 AC：AC-UI-16-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让两次 `send` 返回 5000，产生 2 条失败消息。
- 步骤：点"退出登录"
- 预期结果：确认框正文为"有 2 条消息未发送，退出后将丢失"

### TC-UI-16-03 pendingCount 统计与事件
- 关联 AC：AC-UI-16-2
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`。
- 步骤：
  1. 离线发送 1 条（queued）；在线发 1 条不回 ack（sending）；再发 1 条回 5000（failed）
  2. 让 sending 那条收到 ack
- 预期结果：
  1. 依次发出 `pendingCount` 事件 1、2、3；`pendingCount()` 为 3
  2. 之后为 2

### TC-UI-16-04 确认退出后 sessionStorage 无 token
- 关联 AC：AC-UI-16-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：退出登录并确认
- 预期结果：回到 `/login`；`sessionStorage` 中没有 token 键；WebSocket 以 1000 关闭

### TC-UI-16-05 关于天听版本号与 package.json 一致
- 关联 AC：AC-UI-16-4
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：打开"关于天听"
- 预期结果：显示的版本号等于 tianting-web `package.json` 的 `version`；有构建号和开源许可入口

### TC-UI-16-06 设置页顺序：演示会话没有"账号安全"，账号会话有
- 关联 AC：AC-UI-16-6
- 优先级：P0
- 层级：前端单测
- 前置条件：M3 构建；分别用 `mode=demo`、`mode=account` 的会话。
- 步骤：分别渲染 `/me/settings`
- 预期结果：演示会话依次为"隐私""黑名单""关于天听""退出登录"，没有"外观"和"账号安全"；账号会话在"黑名单"之后多一项"账号安全"；"退出登录"为 `color.danger`。"通知"这一项随 C3

---

## F-UI-17 连接状态条（M3，引用 F-CONN-02）

### TC-UI-17-01 离线时状态条显示且不遮挡消息
- 关联 AC：AC-UI-17-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 在会话列表执行 `context.setOffline(true)`，等待状态条
  2. 进入聊天页
- 预期结果：
  1. 会话列表状态条文字为"网络不可用"
  2. 聊天页状态条同样显示；消息列表最上方一条消息的上边缘 ≥ 状态条下边缘

---

## F-UI-18 被踢与登录失效弹窗（M3，引用 F-AUTH-02 / F-AUTH-03）

### TC-UI-18-01 被踢弹窗不能用 Esc、遮罩、后退关闭
- 关联 AC：AC-UI-18-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 下发 `kicked {reason:'tokenInvalid'}` 并以 4010 关闭。
- 步骤：
  1. 弹窗出现后按 Esc
  2. 点击遮罩
  3. 执行 `page.goBack()`
- 预期结果：每一步后弹窗仍在，路由不变

### TC-UI-18-02 tooManyConnections 文案
- 关联 AC：AC-UI-18-2
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 下发 `kicked {reason:'tooManyConnections'}` 并以 4429 关闭。
- 步骤：渲染应用
- 预期结果：弹窗文案为"登录设备过多，当前设备已下线"，只有"确定"按钮

### TC-UI-18-03 被踢弹窗文案表与关闭码兜底
- 关联 AC：AC-UI-18-2
- 优先级：P1
- 层级：前端单测
- 前置条件：`FakeTransport`。
- 步骤：依次模拟 4010/tokenInvalid、4011/tokenExpired、4012/banned、4012 无 kicked、1001/serverShutdown
- 预期结果：文案依次为"登录已失效，请重新登录""登录已过期，请重新登录""账号已被下线""账号已被下线"；1001 不弹窗、按断线处理

### TC-UI-18-04 点确定后社交状态清空
- 关联 AC：AC-UI-18-3
- 优先级：P0
- 层级：Playwright
- 前置条件：alice 有好友 bob；dave 无好友。
- 步骤：
  1. alice 登录后被踢，点"确定"
  2. 以 dave 登录打开通讯录
- 预期结果：
  1. 回到登录页
  2. dave 的通讯录没有 bob，显示"还没有朋友"

---

## F-UI-19 轻提示与错误文案（M3，细化 F-SYS-01）

### TC-UI-19-01 同时只有一条轻提示，2 秒后消失
- 关联 AC：AC-UI-19-1
- 优先级：P0
- 层级：前端单测
- 前置条件：组件测试，假定时器；`FakeTransport` 依次回两个错误 ack：4290、4042。
- 步骤：
  1. 连续触发两个请求
  2. 推进 1.7 秒、2.3 秒
- 预期结果：
  1. 屏幕上只有 1 条轻提示，内容"用户不存在"
  2. 1.7 秒仍在、2.3 秒已消失

### TC-UI-19-02 不展示服务端 msg
- 关联 AC：AC-UI-19-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `send` 返回 `{code:4042, msg:"user not found xyz"}`。
- 步骤：给 nobody 发消息
- 预期结果：显示"用户不存在"；`page.content()` 中搜不到"user not found xyz"

### TC-UI-19-03 errors.json 每个 ws 码都有文案，命令+码表一致
- 关联 AC：AC-UI-19-3
- 优先级：P0
- 层级：前端单测
- 前置条件：tianting-web 文案模块；`protocol/errors.json`；`protocol/vectors/social.json` 文案用例。
- 步骤：
  1. 遍历 errors.json 中 `scope` 为 ws 的全部码求文案
  2. 遍历 social.json 文案用例（命令 + 码 → 文案）
- 预期结果：
  1. 每个码文案非空
  2. 每条用例与期望完全相等；errors.json 新增 ws 码未补文案时该测试失败（X-10）

### TC-UI-19-04 friendApply 与 send 的 4292 文案不同
- 关联 AC：AC-UI-19-4
- 优先级：P0
- 层级：前端单测
- 前置条件：文案模块。
- 步骤：求 `('friendApply',4292)`、`('send',4292)`、`('userSearch',4292)`
- 预期结果：分别为"今天的次数已用完，明天再试""今天发起新聊天的次数已用完，请稍后再试""今天的次数已用完，明天再试"

### TC-UI-19-05 4001 与 4008 文案
- 关联 AC：AC-UI-19-5
- 优先级：P0
- 层级：前端单测
- 前置条件：文案模块；logger 桩。
- 步骤：求 `('friendApply',4001)`、`('send',4008)`、`('send',4002)`
- 预期结果：4001 →"当前版本过低，请刷新页面"；4008、4002 →"请求出错"，并各写 1 条日志

### TC-UI-19-06 非 send 命令 4290 与未定义码兜底
- 关联 AC：AC-UI-19-4、AC-UI-19-5
- 优先级：P1
- 层级：前端单测
- 前置条件：文案模块。
- 步骤：求 `('userSearch',4290)`、`('send',4290)`、`('send',4099)`、`('send',5099)`
- 预期结果："操作太频繁，请稍后再试""发送太频繁，请稍后再试""请求出错""服务异常，请稍后重试"

---

## F-UI-20 空状态与加载状态（M3）

### TC-UI-20-01 convs 延迟期间骨架屏，空列表后显示空状态
- 关联 AC：AC-UI-20-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 把 `convs` 的 ack 延迟 3 秒并返回空列表。
- 步骤：
  1. 打开 `/chats`，1 秒时检查
  2. ack 到达后检查，点"发起单聊"
- 预期结果：
  1. 1 秒时有 6 条骨架屏，没有"还没有聊天"
  2. 显示"还没有聊天"和"发起单聊"；点击后打开发起单聊弹窗

### TC-UI-20-02 首轮 socialSync 期间通讯录骨架屏
- 关联 AC：AC-UI-20-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 把第一页 `socialSync` 的 ack 延迟 3 秒。
- 步骤：打开 `/contacts`，1 秒时检查
- 预期结果：好友区显示 6 条骨架屏，没有"还没有朋友"

### TC-UI-20-03 三个列表的空状态文案
- 关联 AC：AC-UI-20-3
- 优先级：P0
- 层级：前端单测
- 前置条件：申请、黑名单、群聊均为空。
- 步骤：分别渲染"新的朋友"、黑名单、群聊列表
- 预期结果：分别显示"暂无新的朋友""黑名单为空""还没有加入群聊"

---

## F-UI-21 头像（M3，细化 F-USER-02 / C-20）

### TC-UI-21-01 占位头像首字
- 关联 AC：AC-UI-21-1
- 优先级：P0
- 层级：前端单测
- 前置条件：`<Avatar>` 组件。
- 步骤：分别传入显示名"bob""张三""😀abc"
- 预期结果：文字分别为"B""张""😀"

### TC-UI-21-02 avatar.json 颜色下标与跨页面一致
- 关联 AC：AC-UI-21-2
- 优先级：P0
- 层级：前端单测
- 前置条件：`protocol/vectors/avatar.json`。
- 步骤：
  1. 遍历用例调用 `avatarColorIndex`、`avatarInitial`
  2. 在会话列表、聊天页、"我"页、通讯录渲染同一 userId 的占位头像
- 预期结果：
  1. 全部用例下标与首字一致
  2. 四处背景色相同

### TC-UI-21-03 用户头像圆形、群头像圆角 10
- 关联 AC：AC-UI-21-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。有单聊和群会话。
- 步骤：读取单聊和群会话项头像的宽高与 `border-radius`
- 预期结果：单聊宽高相等且圆角 ≥ 宽度 50%；群头像圆角 10px（±0.5）

### TC-UI-21-04 非 https、空串、404 回落占位且无错误
- 关联 AC：AC-UI-21-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 改写 `users` 应答：三个用户 avatar 分别为 `http://…`、空串、返回 404 的 https 地址。
- 步骤：打开展示这三人的页面，收集 `pageerror` 与 `console`
- 预期结果：三人都显示占位头像；没有未处理错误

### TC-UI-21-05 https 头像属性与 http 不发请求
- 关联 AC：AC-UI-21-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。一个用户 avatar 为 `https://img.test/a.png`（`page.route` 返回图片），另一个为 `http://img.test/b.png`。
- 步骤：打开会话列表，记录请求
- 预期结果：https 的 `<img>` 有 `referrerpolicy="no-referrer"`、`loading="lazy"`；没有发往 `http://img.test/b.png` 的请求

---

## F-UI-22 主题：浅色（M3）/ 深色（M4）

### TC-UI-22-01 组件代码无颜色字面量
- 关联 AC：AC-UI-22-3
- 优先级：P0
- 层级：前端单测
- 前置条件：CI lint 脚本。
- 步骤：
  1. 对 `src/` 运行颜色字面量检查
  2. 在一个组件中故意加入 `#fff` 和 `<path fill="#000">` 再运行
- 预期结果：
  1. 当前代码通过（token 文件和许可证文件豁免）
  2. 故意加入后检查失败并指出文件和行号

### TC-UI-22-02 M3 构建在系统深色下仍为浅色
- 关联 AC：AC-UI-22-4
- 优先级：P0
- 层级：Playwright
- 前置条件：M3 构建；`page.emulateMedia({colorScheme:'dark'})`。
- 步骤：打开 `/login` 与 `/chats`
- 预期结果：`body` 背景色为 `#F2F3F7`

---

## F-UI-23 未读总数与"新的朋友"角标（M3）

### TC-UI-23-01 总未读角标与页面标题
- 关联 AC：AC-UI-23-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 改写 `convs`：三个会话未读 2、5、0。
- 步骤：打开 `/chats`
- 预期结果："消息"标签角标"7"；`document.title` 为"(7) 天听"
- 说明（PRD-ui v1.5 加注）：三个会话都没有免打扰（`notifyOff` 不出现），结果不变；有免打扰时总未读按 PRD-chat F-CVS-02 规则 4 计算，见 TC-CVS-02-02、02-04、02-05

### TC-UI-23-02 99+ 与 0 时的标题
- 关联 AC：AC-UI-23-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 改写 `convs`：总未读 120；之后改为 0。
- 步骤：
  1. 打开 `/chats`
  2. 重新加载使总未读为 0
- 预期结果：
  1. 角标"99+"，`document.title` 为"(120) 天听"
  2. `document.title` 为"天听"，角标不显示
- 说明（PRD-ui v1.5 加注）：同 TC-UI-23-01，`convs` 里没有免打扰会话

### TC-UI-23-03 totalUnread 计算与事件
- 关联 AC：AC-UI-23-1、AC-UI-23-2、AC-UI-23-3
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`。
- 步骤：
  1. 回放三个会话未读 2、5、0
  2. 对未读 5 的会话 `markRead`
- 预期结果：
  1. `totalUnread()` 为 7，事件 1 次
  2. 变为 2，事件再发 1 次

### TC-UI-23-04 打开会话后角标 2 秒内减少
- 关联 AC：AC-UI-23-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。某会话未读 5，总未读 7。
- 步骤：打开该会话
- 预期结果：2 秒内"消息"标签角标为"2"

### TC-UI-23-05 另一端进入新的朋友后红点消失
- 关联 AC：AC-UI-23-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `markFriendRequestsSeen` 多端同步（F-SOC-04、AC-SOC-04-2）；bob 有 1 条未读申请；两个标签页，第二个带 `?platform=pc`（C-44）。
- 步骤：bob 在 web 标签页进入"新的朋友"
- 预期结果：2 秒内 pc 标签页的"通讯录"角标消失

---

## F-UI-25 新的朋友（M3，细化 PRD-social F-SOC-03 / F-SOC-04）

### TC-UI-25-01 新的朋友列表顺序与第二行
- 关联 AC：AC-UI-25-1
- 优先级：P0
- 层级：前端单测
- 前置条件：`friendRequests.items`：carol（附言"我是 carol"）先、dave（无附言）后。
- 步骤：渲染 `/contacts/requests`
- 预期结果：dave 在上，第二行"请求添加你为朋友"；carol 第二行"我是 carol"；两项右侧都是"同意"

### TC-UI-25-02 点同意后变为已添加，双方通讯录出现
- 关联 AC：AC-UI-25-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端（F-SOC-03、F-SOC-06）；carol 已向 alice 申请。
- 步骤：alice 在新的朋友点 carol 的"同意"
- 预期结果：2 秒内该项变为"已添加"；alice 通讯录出现 carol；carol 通讯录出现 alice

### TC-UI-25-03 申请详情来源与拒绝
- 关联 AC：AC-UI-25-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端；dave 以 `source=search` 向 alice 申请。
- 步骤：
  1. alice 点 dave 所在行
  2. 点"拒绝"
  3. dave 查看自己的"新的朋友"
- 预期结果：
  1. 底部菜单显示"通过搜索添加"及"同意""拒绝""查看资料"
  2. 该项显示"已拒绝"，没有二次确认
  3. dave 一侧仍显示"等待验证"

### TC-UI-25-04 发出的申请显示资料缓存或 userId
- 关联 AC：AC-UI-25-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。eve 不可被搜索；`routeWs` 让 `userSearch eve` 返回 eve 资料（昵称"伊娃"）。
- 步骤：
  1. 通过搜索结果给 eve 发申请，打开新的朋友
  2. 刷新页面后再打开
- 预期结果：
  1. 该项显示"伊娃"，右侧"等待验证"
  2. 显示"eve"，第二行"我：{附言}"（附言为空时"等待对方验证"），右侧"等待验证"

### TC-UI-25-05 进入页面红点立即消失
- 关联 AC：AC-UI-25-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。2 条未读申请。
- 步骤：打开 `/contacts/requests`
- 预期结果："通讯录"标签角标和"新的朋友"红点立即消失；发出 `markFriendRequestsSeen` 对应的命令 1 次

### TC-UI-25-06 过期申请不显示同意且不计红点
- 关联 AC：AC-UI-25-6
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 回放一条 `in` 申请，`expireAt` 早于 auth ack 的 `serverTime`（等价于测试时钟推进）。
- 步骤：渲染新的朋友和通讯录
- 预期结果：该项显示"已过期"，没有"同意"；红点为 0

### TC-UI-25-07 friendRespond 4045 提示并变为已过期
- 关联 AC：AC-UI-25-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `friendRespond` 返回 4045，随后的同步页把该申请标为 `expired`。
- 步骤：点"同意"
- 预期结果：提示"该申请已过期"；2 秒内该项变为"已过期"

### TC-UI-25-08 停在本页时新申请不产生角标
- 关联 AC：AC-UI-25-8
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。alice 停在 `/contacts/requests`；`routeWs` 推送 `socialChanged` 与包含 frank 申请的同步页。
- 步骤：等待列表更新
- 预期结果：列表顶部出现 frank；"通讯录"标签角标不出现（200ms 去抖后再次 `markFriendRequestsSeen`）

---

## F-UI-26 添加朋友（搜索）（M3，细化 PRD-social F-SOC-09 / F-SOC-02）

### TC-UI-26-01 搜索无关系用户显示添加到通讯录
- 关联 AC：AC-UI-26-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `userSearch`（F-SOC-09）；bob 可被搜索、与 alice 无关系，昵称"阿博"。
- 步骤：
  1. alice 在 `/contacts/add` 搜索"bob"
  2. 点"添加到通讯录"
- 预期结果：
  1. 结果卡片显示"阿博""账号：bob"和"添加到通讯录"
  2. 进入 `/contacts/apply/bob`

### TC-UI-26-02 关闭可被搜索与不存在的结果 DOM 相同
- 关联 AC：AC-UI-26-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `setPrivacy` 与 `userSearch` 防枚举（F-SOC-09 规则 3）。
- 步骤：
  1. bob 关闭"允许通过账号搜索到我"
  2. alice 搜索"bob"，保存结果区域 `innerHTML`
  3. alice 搜索"nobody"
- 预期结果：
  1. 两次结果文字都是"未找到该用户"
  2. 去掉 `data-*` 属性后两次 `innerHTML` 相等

### TC-UI-26-03 非法账号本地提示且不发请求
- 关联 AC：AC-UI-26-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：输入"a b"按 Enter
- 预期结果：提示"账号格式不正确"；没有 `userSearch` 帧

### TC-UI-26-04 四种 relation 的结果卡片
- 关联 AC：AC-UI-26-4
- 优先级：P0
- 层级：前端单测
- 前置条件：搜索页组件；`searchUser` 桩依次返回自己、好友 bob、pendingOut 的 carol、blocked 的 eve。
- 步骤：依次搜索
- 预期结果：自己：文字"这是你自己"无按钮；bob："发消息"；carol："已发送申请"+"重新发送"；eve："已加入黑名单"+"查看资料"，没有"添加到通讯录"

### TC-UI-26-05 userSearch 4292 文案
- 关联 AC：AC-UI-26-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `userSearch` 返回 4292。
- 步骤：搜索"bob"
- 预期结果：提示"今天的次数已用完，明天再试"

### TC-UI-26-06 官方账号显示官方标识
- 关联 AC：AC-UI-26-6
- 优先级：P0
- 层级：前端单测
- 前置条件：`searchUser('kefu')` 返回 `official=true` 的用户。
- 步骤：搜索"kefu"
- 预期结果：结果卡片有"官方"标识（`color.brand` 字、`color.surface.selected` 底）

### TC-UI-26-07 请求未返回时连按三次回车只发一次
- 关联 AC：AC-UI-26-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 吞掉 `userSearch` 的 ack。
- 步骤：输入"bob"连续按 3 次 Enter
- 预期结果：只发出 1 次 `userSearch`；显示加载态

---

## F-UI-27 申请添加朋友（M3，细化 PRD-social F-SOC-02）

### TC-UI-27-01 附言默认值与计数
- 关联 AC：AC-UI-27-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。alice 无昵称（显示名 alice）。
- 步骤：打开 `/contacts/apply/bob`
- 预期结果：附言输入框默认"我是alice"，右下角"7 / 50"

### TC-UI-27-02 附言和备注发送后双方可见
- 关联 AC：AC-UI-27-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `friendApply` / `friendRespond`（F-SOC-02、F-SOC-03、F-SOC-05）。
- 步骤：
  1. alice 填附言"球友"、备注"B哥"点"发送"
  2. bob 打开新的朋友
  3. bob 同意，alice 打开通讯录
- 预期结果：
  1. 提示"已发送"并返回上一页
  2. bob 列表中有 alice，第二行"球友"
  3. alice 通讯录中 bob 显示为"B哥"

### TC-UI-27-03 附言 51、备注 33 码点禁用发送
- 关联 AC：AC-UI-27-3
- 优先级：P0
- 层级：前端单测
- 前置条件：申请页组件。
- 步骤：
  1. 附言输入 51 个码点
  2. 附言恢复为 10 个，备注输入 33 个码点
- 预期结果：
  1. 显示"最多 50 个字"，"发送"禁用
  2. 显示"最多 32 个字"，"发送"禁用

### TC-UI-27-04 对方已申请过我时直接成为好友
- 关联 AC：AC-UI-27-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端（F-SOC-02 判定表"对方已申请"行）；bob 已给 alice 发申请未处理。
- 步骤：alice 打开 `/contacts/apply/bob` 点"发送"
- 预期结果：提示"已添加到通讯录"，页面替换为 bob 的资料卡，按钮为"发消息"

### TC-UI-27-05 不存在与拉黑了我的用户表现相同
- 关联 AC：AC-UI-27-5
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端防枚举（F-SOC-02）；carol 已拉黑 alice。
- 步骤：
  1. alice 给 nobody 发申请
  2. alice 给 carol 发申请
- 预期结果：两次都提示"已发送"并返回上一页，界面表现相同

### TC-UI-27-06 直接打开为 other，从搜索进入为 search
- 关联 AC：AC-UI-27-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 直接打开 `/contacts/apply/bob` 点"发送"
  2. 从搜索结果进入 bob 的申请页点"发送"
- 预期结果：
  1. 第一次 `friendApply.source="other"`
  2. 第二次 `source="search"`

### TC-UI-27-07 请求未返回时连点三次只发一次
- 关联 AC：AC-UI-27-7
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 吞掉 `friendApply` 的 ack。
- 步骤：连续点击"发送" 3 次
- 预期结果：只发出 1 次 `friendApply`

### TC-UI-27-08 申请自己时提示并返回
- 关联 AC：AC-UI-27-8
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。先打开 `/contacts` 再打开 `/contacts/apply/alice`。
- 步骤：等待页面处理
- 预期结果：提示"不能添加自己"并返回上一页（`/contacts`）；没有 `friendApply`

---

## F-UI-28 设置备注（M3，细化 PRD-social F-SOC-05）

### TC-UI-28-01 备注多端同步
- 关联 AC：AC-UI-28-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端备注与多端同步（F-SOC-05、F-SOC-06）；alice 两个标签页，第二个 `?platform=pc`。
- 步骤：web 标签页把 bob 的备注改为"老板"点"完成"
- 预期结果：资料卡标题变为"老板"；2 秒内 pc 标签页通讯录中 bob 显示为"老板"

### TC-UI-28-02 只有空格的备注清除备注
- 关联 AC：AC-UI-28-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。bob 有昵称"阿博"、备注"老板"。
- 步骤：输入"  "点"完成"
- 预期结果：发出 `friendRemark` 的 `remark` 为空串；资料卡标题恢复为"阿博"

### TC-UI-28-03 备注超过 32 码点禁用完成
- 关联 AC：AC-UI-28-3
- 优先级：P0
- 层级：前端单测
- 前置条件：备注页组件。
- 步骤：输入 33 个码点
- 预期结果：显示"最多 32 个字"，"完成"禁用

### TC-UI-28-04 未改动直接完成不发请求
- 关联 AC：AC-UI-28-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。bob 备注"老板"。
- 步骤：打开备注页，直接点"完成"
- 预期结果：返回资料卡；没有 `friendRemark` 帧

### TC-UI-28-05 friendRemark 4040 提示并返回
- 关联 AC：AC-UI-28-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `friendRemark` 返回 4040。
- 步骤：修改备注点"完成"
- 预期结果：提示"对方已不在你的通讯录中"并返回资料卡

---

## F-UI-29 黑名单（M3，细化 PRD-social F-SOC-07）

### TC-UI-29-01 黑名单倒序与设置页人数
- 关联 AC：AC-UI-29-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端拉黑（F-SOC-07）。
- 步骤：
  1. alice 在资料卡先后拉黑 bob、eve
  2. 打开设置页和 `/me/settings/blocks`
- 预期结果：
  1. 设置页"黑名单"右侧显示"2"
  2. 列表 eve 在上、bob 在下

### TC-UI-29-02 移出黑名单多端同步后可私聊
- 关联 AC：AC-UI-29-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端拉黑、多端同步与私聊权限（F-SOC-07、F-SOC-08）；alice 两个标签页。
- 步骤：
  1. alice 在黑名单点 bob 的"移出"
  2. bob 给 alice 发私聊
- 预期结果：
  1. bob 从列表消失并提示"已移出黑名单"；2 秒内另一个标签页黑名单中也没有 bob
  2. 发送成功，alice 收到

### TC-UI-29-03 profile 为空时显示 userId 和占位头像
- 关联 AC：AC-UI-29-3
- 优先级：P0
- 层级：前端单测
- 前置条件：`blocks`：frank，`profile=null`。
- 步骤：渲染黑名单
- 预期结果：显示"frank"和占位头像（首字"F"）

---

## F-UI-30 隐私设置（M3，细化 PRD-social F-SOC-09）

### TC-UI-30-01 关闭可被搜索后搜不到，打开后能搜到
- 关联 AC：AC-UI-30-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `setPrivacy` / `userSearch`（F-SOC-09）；alice 与 bob 无关系。
- 步骤：
  1. alice 关闭开关，bob 搜索"alice"
  2. alice 打开开关，bob 再搜索
- 预期结果：
  1. 第一次显示"未找到该用户"
  2. 第二次显示 alice

### TC-UI-30-02 setPrivacy 4290 回滚
- 关联 AC：AC-UI-30-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `setPrivacy` 返回 4290。
- 步骤：点击开关
- 预期结果：开关回到原值；提示"操作太频繁，请稍后再试"

### TC-UI-30-03 请求期间开关禁用
- 关联 AC：AC-UI-30-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 吞掉 `setPrivacy` 的 ack。
- 步骤：点击开关
- 预期结果：开关为禁用态（`aria-disabled=true`），再次点击不发新请求

---

## F-UI-31 私聊权限提示（M3，细化 PRD-social F-SOC-08）

### TC-UI-31-01 好友制下非好友发消息出现申请入口
- 关联 AC：AC-UI-31-1
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端私聊权限（F-SOC-08）且配置为好友制；carol 不是 alice 的好友。
- 步骤：
  1. alice 给 carol 发"你好"
  2. 点"发送好友申请"并发送
- 预期结果：
  1. 消息显示"!"，下方出现"对方开启了好友验证，你还不是他的好友"和"发送好友申请"
  2. 进入 `/contacts/apply/carol`；`friendApply.source="conv"`

### TC-UI-31-02 被拉黑后发消息显示拒收提示
- 关联 AC：AC-UI-31-2
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端拉黑与私聊权限（F-SOC-07、F-SOC-08）；bob 已拉黑 alice。
- 步骤：alice 给 bob 发消息
- 预期结果：消息失败，下方提示条"消息已发出，但被对方拒收了"；没有轻提示

### TC-UI-31-03 陌生人限额提示
- 关联 AC：AC-UI-31-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端陌生人限额（F-SOC-08）；开放模式、`strangerMsgLimit=3`；bob 与 alice 是陌生人且 bob 未回复。
- 步骤：alice 给 bob 连发 4 条
- 预期结果：第 4 条失败，下方出现"对方回复之前，最多只能发送 3 条消息"

### TC-UI-31-04 连续 4035 只保留一条提示条
- 关联 AC：AC-UI-31-4
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport`：给 carol 的 3 次 `send` 都返回 4035。
- 步骤：连发 3 条
- 预期结果：消息列表只有 1 条"对方开启了好友验证……"提示条，位于第 3 条失败消息下方

### TC-UI-31-05 send 4292 只弹轻提示
- 关联 AC：AC-UI-31-5
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `send` 返回 4292。
- 步骤：给 bob 发消息
- 预期结果：出现轻提示"今天发起新聊天的次数已用完，请稍后再试"；消息列表中没有提示条

### TC-UI-31-06 刷新后提示条和失败消息消失
- 关联 AC：AC-UI-31-6
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让 `send` 返回 4035。
- 步骤：
  1. 发消息出现提示条
  2. 刷新页面重新进入该会话
- 预期结果：
  1. 提示条存在
  2. 提示条和失败消息都不再出现

---

## F-UI-32 群聊列表（M3，细化 PRD-social F-SOC-11）

### TC-UI-32-01 群聊列表排序、人数和总数
- 关联 AC：AC-UI-32-1
- 优先级：P0
- 层级：前端单测
- 前置条件：`loadMyGroups` 返回"周末球局"（12 人）、"Alpha"（3 人）。
- 步骤：渲染 `/contacts/groups`
- 预期结果：依次"Alpha""周末球局"，右侧"3 人""12 人"；顶部"2 个群聊"

### TC-UI-32-02 600 个群全部显示
- 关联 AC：AC-UI-32-2
- 优先级：P1
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `myGroups` 分页（F-SOC-11）；`seed` 让 alice 在 600 个群、100 个单聊会话中。
- 步骤：打开 `/contacts/groups`
- 预期结果：显示 600 个群，顶部"600 个群聊"

### TC-UI-32-03 myGroups 失败重试
- 关联 AC：AC-UI-32-3
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`routeWs` 让第一次 `myGroups` 返回 5000，之后正常。
- 步骤：
  1. 打开群聊列表
  2. 点"加载失败，点击重试"
- 预期结果：
  1. 显示"加载失败，点击重试"
  2. 重新请求成功并显示列表

### TC-UI-32-04 进入不在 convs 前 500 的群可收发
- 关联 AC：AC-UI-32-4
- 优先级：P1
- 层级：端到端
- 前置条件：需要 PRD-social M3 服务端 `myGroups`；`seed` 让 alice 有 600 个会话，群 g599 不在 `convs` 前 500。
- 步骤：
  1. 在群聊列表点 g599
  2. 发送 1 条消息
- 预期结果：
  1. 进入 g599 聊天页并加载出历史
  2. 发送成功，会话出现在会话列表

---

## F-UI-33 未知会话、被移出 / 退出 / 解散的会话（M3；`reason = left` 为 M4）

### TC-UI-33-01 格式非法的 convId 跳回 /chats
- 关联 AC：AC-UI-33-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：打开 `/chat/abc`
- 预期结果：路由变为 `/chats`；提示"会话不存在"；没有 `sync` 帧

### TC-UI-33-02 非参与者的单聊本地拦截
- 关联 AC：AC-UI-33-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：打开 `/chat/c2c%3Abob%3Acarol`
- 预期结果：路由变为 `/chats`；提示"会话不存在"；`routeWs` 记录中没有 `sync`

### TC-UI-33-03 未聊过的单聊显示空聊天页并可发送
- 关联 AC：AC-UI-33-3
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-core 单聊与 `sync`（服务端已有）；dave 已导入、与 alice 从未聊过。
- 步骤：
  1. 打开 `/chat/c2c%3Aalice%3Adave`
  2. 发送"你好"
- 预期结果：
  1. 显示与 dave 的空聊天页，输入框可用
  2. 发送成功后会话列表出现 dave

### TC-UI-33-04 停在群聊天页时被管理接口移出
- 关联 AC：AC-UI-33-4
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-core 管理接口移除成员与 `convRemoved`（BE-031、BE-034）；alice 在 g1。
- 步骤：
  1. alice 停在 g1 聊天页
  2. 管理接口把 alice 移出 g1
- 预期结果：3 秒内列表底部出现"你已不在该群"；输入框消失，显示"你已不在该群，无法发送消息"；标题仍为 g1 群名、不带成员数；会话列表中 g1 消失

### TC-UI-33-05 移出后从历史进入与刷新后打开
- 关联 AC：AC-UI-33-5
- 优先级：P0
- 层级：端到端
- 前置条件：接 TC-UI-33-04；需要 PRD-core `sync` 对非成员返回 4033（AC-GRP-01-4）。
- 步骤：
  1. 返回会话列表，`page.goBack()` / `goForward()` 回到 `/chat/group%3Ag1`
  2. 刷新页面再打开 `/chat/group%3Ag1`
- 预期结果：
  1. 标题仍为 g1 群名，显示"你已不在该群"
  2. 标题为"群聊"，显示"你已不在该群"

### TC-UI-33-06 不存在的群显示群聊不存在或已解散
- 关联 AC：AC-UI-33-6
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-core `sync` 对不存在的群返回 4043（AC-GRP-02-3）。
- 步骤：打开 `/chat/group%3Anotexist`
- 预期结果：标题"群聊"；列表居中"群聊不存在或已解散"；输入区为禁用条"群聊不存在或已解散"，没有输入框

### TC-UI-33-07 离线期间被移出，重连后生成快照
- 关联 AC：AC-UI-33-9
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-core 管理接口移除成员；alice 在 g1。
- 步骤：
  1. alice 停在 g1 聊天页，`context.setOffline(true)`
  2. 管理接口移出 alice
  3. `context.setOffline(false)` 等待重连
- 预期结果：重连后 3 秒内列表底部"你已不在该群"，输入框消失并显示"你已不在该群，无法发送消息"，标题仍为 g1 群名；会话列表中没有 g1

### TC-UI-33-08 重连比对生成快照（SDK）
- 关联 AC：AC-UI-33-9
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`：本地 `convs` 有 g1、g2、c2c:alice:bob。
- 步骤：断线后重连，新 `convs` 只有 g2（`truncated=false`）
- 预期结果：发出 `convRemoved {convId:'group:g1'}`（无 reason）；`conversationSnapshot('group:g1')` 为 `{removed:true, name:'g1 群名'}`；c2c:alice:bob 没有快照、没有事件

### TC-UI-33-09 truncated 时不推断移出
- 关联 AC：AC-UI-33-9
- 优先级：P0
- 层级：前端单测
- 前置条件：同上。
- 步骤：重连后新 `convs` 不含 g1 且 `truncated=true`
- 预期结果：不发 `convRemoved`，没有快照

### TC-UI-33-10 刷新后打开已移出的群：sync 返回前不显示提示
- 关联 AC：AC-UI-33-10
- 优先级：P0
- 层级：前端单测
- 前置条件：组件测试；`FakeTransport` 本地无 g1 会话、无快照；`sync` 的应答由测试手动放行。
- 步骤：
  1. 打开 `/chat/group%3Ag1`
  2. 检查 DOM
  3. 放行 4033 应答
- 预期结果：
  1. `FakeTransport` 记录到 `sync {convId:"group:g1", beforeSeq:9007199254740991, …}`
  2. 应答前显示加载状态，没有"你已不在该群"，也没有输入框
  3. 之后才显示"你已不在该群"，标题"群聊"

### TC-UI-33-11 loadHistory 对未知会话请求最新一页（SDK）
- 关联 AC：AC-UI-33-10
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`。
- 步骤：对本地不存在的 `group:g9` 调用 `loadHistory`，分别回 4033、4043、成功一页
- 预期结果：每次发出 `beforeSeq = Number.MAX_SAFE_INTEGER` 的 `sync`；4033 / 4043 以带 `code` 的错误抛出且不重试；成功时 `group:g9` 出现在 `conversations()`

### TC-UI-33-12 重新被拉进群后恢复
- 关联 AC：AC-UI-33-11
- 优先级：P0
- 层级：端到端
- 前置条件：接 TC-UI-33-04；需要 PRD-core 管理接口加成员与 `convAdded`。
- 步骤：
  1. 管理接口把 alice 重新加入 g1
  2. 发送 1 条消息
- 预期结果：
  1. 3 秒内会话列表重新出现 g1；停在 g1 聊天页的输入框恢复，禁用条消失
  2. 发送成功

### TC-UI-33-13 三种 reason 的文案
- 关联 AC：AC-UI-33-12
- 优先级：P0
- 层级：前端单测
- 前置条件：`FakeTransport` 依次推送 `convRemoved {convId:"group:g1", reason:"dissolved"}`、`{convId:"group:g2", reason:"xyz"}`、`{convId:"group:g3"}`。
- 步骤：分别渲染 g1、g2、g3 聊天页
- 预期结果：
  1. g1 列表底部和禁用条都为"群聊已解散"
  2. g2、g3 都为"你已不在该群"和"你已不在该群，无法发送消息"

### TC-UI-33-14 M3 构建收到 reason=left 按不认识处理
- 关联 AC：AC-UI-33-12
- 优先级：P1
- 层级：前端单测
- 前置条件：M3 构建；`FakeTransport` 推送 `convRemoved {convId:"group:g4", reason:"left"}`。
- 步骤：渲染 g4 聊天页
- 预期结果：显示"你已不在该群"和"你已不在该群，无法发送消息"

### TC-UI-33-15 parseConvIdStrict 共享用例
- 关联 AC：AC-UI-33-13
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts；`protocol/vectors/convid.json`；`self = alice`。
- 步骤：遍历用例
- 预期结果：`c2c:alice:bob` → `{convType:"c2c", to:"bob"}`；`group:g1` → `{convType:"group", to:"g1"}`；`c2c:bob:carol` 抛 `notParticipant`；`abc`、`c2c:abc`、`c2c:bob:alice`、`c2c:alice:alice`、`c2c:alice:bob:x`、`c2c:alice:b.b`、`group:`、`group:g1:x`、`room:r1` 都抛 `invalid`

### TC-UI-33-16 管理接口解散托管群
- 关联 AC：AC-UI-33-14
- 优先级：P0
- 层级：端到端
- 前置条件：需要 PRD-admin 第一批 `DELETE /admin/groups/{id}` 与 `convRemoved.reason=dissolved`（F-ADM-24、PRD-social F-SOC-26 M3 部分）。
- 步骤：
  1. alice 停在托管群 g1 聊天页
  2. 业务方调用 `DELETE /admin/groups/g1`
- 预期结果：3 秒内列表底部"群聊已解散"，输入框消失并显示"群聊已解散"；会话列表中 g1 消失

### TC-UI-33-17 快照随退出登录清除，convAdded 清除快照（SDK）
- 关联 AC：AC-UI-33-11、AC-UI-33-4
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，`FakeTransport`。
- 步骤：
  1. 推送 `convRemoved g1` 后推送 `convAdded g1`
  2. 再次推送 `convRemoved g1` 后调用 `disconnect()` 并重新登录
- 预期结果：
  1. `convAdded` 后快照为空、g1 回到列表
  2. 重新登录后没有 g1 的快照

### TC-UI-33-18 被移出时已输入内容不保留
- 关联 AC：AC-UI-33-4
- 优先级：P1
- 层级：前端单测
- 前置条件：聊天页组件，输入框中有"未发送"。
- 步骤：
  1. 推送 `convRemoved g1`
  2. 推送 `convAdded g1`
- 预期结果：
  1. 输入区变为禁用条
  2. 输入框恢复为空

---

## 非功能（PRD-ui 9.5 节）

### TC-UI-90-01 昵称、群名、备注、附言中的 HTML 原样显示
- 关联 AC：AC-UI-90-1
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`seed`：用户 `xss1` 昵称 `<img src=x onerror=alert(1)>`，他发来的消息与所在群名相同，alice 给他的备注 `<b>x</b>`，他发来的好友申请附言同为该字符串。
- 步骤：依次打开会话列表、群聊天页与单聊聊天页、通讯录、新的朋友、资料卡；监听 `dialog` 和请求
- 预期结果：各处原样显示文本；`document.title` 不受影响；没有对话框；没有发往 `x` 的请求；`page.locator('img[src="x"], b')` 数量为 0

### TC-UI-90-02 存储与 URL 中没有敏感数据
- 关联 AC：AC-UI-90-2
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。
- 步骤：
  1. 登录、收发消息、发附言为 `ZXQ-APPLY` 的申请、设置备注 `ZXQ-REMARK`
  2. 读取 `localStorage`、`sessionStorage`、`location.href` 和历史中的全部 URL
- 预期结果：`localStorage` 只有 `ui.` 开头的键；`sessionStorage` 除 token 外没有消息内容；URL 和历史中搜不到 `ZXQ-APPLY`、`ZXQ-REMARK`

### TC-UI-90-03 对比度 CI 复算 6.2 节全部组合
- 关联 AC：AC-UI-90-3
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 脚本、`src/theme/tokens.json`。
- 步骤：运行脚本，输出 L01～L26、D01～D26、头像调色板 8 色的对比度
- 预期结果：全部满足要求；每个值与 6.2 节表中的两位小数一致（如 L05 = 4.53、D15 = 4.58、L15 = 4.64）

### TC-UI-90-04 对比度 CI 发现低于要求的组合即失败
- 关联 AC：AC-UI-90-3
- 优先级：P0
- 层级：前端单测
- 前置条件：同上，临时副本中把 `color.text.tertiary` 浅色改为 `#737985`（v0.1 值）。
- 步骤：运行脚本
- 预期结果：脚本退出码非 0，并指出 L12（4.37）、L13、L14 不达标

### TC-UI-90-05 清单中 token 名不存在时失败
- 关联 AC：AC-UI-90-3
- 优先级：P0
- 层级：前端单测
- 前置条件：同上，临时副本中删除 `color.border.input`。
- 步骤：运行脚本
- 预期结果：退出码非 0，提示 L23 / L25 / D23 / D25 引用的 token 不存在

### TC-UI-90-06 axe 扫描无对比度和按钮名称违规
- 关联 AC：AC-UI-90-4
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。浅色主题（M3）。
- 步骤：用 `@axe-core/playwright` 依次扫描登录、会话列表、聊天页、通讯录、新的朋友、资料卡、设置
- 预期结果：每页 `color-contrast` 和 `button-name` 违规数为 0

### TC-UI-90-07 品牌色与自己气泡色相检查
- 关联 AC：AC-UI-90-5
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 脚本；临时副本中把 `color.brand` 改为 `#07C160`。
- 步骤：
  1. 对正式 token 运行
  2. 对临时副本运行
- 预期结果：
  1. 正式 token 通过（色相 232°～233°）
  2. 临时副本失败

### TC-UI-90-08 500 个会话渲染性能
- 关联 AC：AC-UI-90-6
- 优先级：P1
- 层级：Playwright
- 前置条件：生产构建；`routeWs` 回放 500 个会话的 `convs`；Chrome CPU 4 倍降速；PRD-core C-50 基准机器。
- 步骤：打开 `/chats`，读取 `performance.measure('convs-render')` 与 `longtask` 观察器，重复 3 次
- 预期结果：3 次中位数 ≤ 500ms；期间没有超过 100ms 的长任务

### TC-UI-90-09 一条新消息只重渲染受影响的会话项
- 关联 AC：AC-UI-90-7
- 优先级：P1
- 层级：Playwright
- 前置条件：开发构建开启 React Profiler；500 个会话。
- 步骤：推送 1 条消息，读取 Profiler 记录
- 预期结果：重渲染的会话项组件数 ≤ 2

### TC-UI-90-10 引用稳定（SDK）
- 关联 AC：AC-UI-90-7
- 优先级：P0
- 层级：前端单测
- 前置条件：sdk-ts，500 个会话。
- 步骤：推送 1 条消息，比较前后两次 `conversations()` 的元素引用
- 预期结果：只有 1 个会话对象引用变化，其余 `Object.is` 为 true

### TC-UI-90-11 首屏 JS 体积 CI
- 关联 AC：AC-UI-90-8
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 生产构建。
- 步骤：运行体积脚本
- 预期结果：首屏 JS（gzip）≤ 200KB；超出时 CI 失败

### TC-UI-90-12 aria-live 播报合并
- 关联 AC：AC-UI-90-11
- 优先级：P0
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`page.clock`。
- 步骤：
  1. 停在 g1 聊天页，3 秒内收到 5 条群消息
  2. 停在与 bob 的单聊，收到"你好"
- 预期结果：
  1. `aria-live` 区域只更新 1 次，内容"收到 5 条新消息"
  2. 内容为"bob：你好"

### TC-UI-90-13 生产构建控制台不打印消息、附言和 token
- 关联 AC：AC-UI-90-12
- 优先级：P0
- 层级：Playwright
- 前置条件：生产构建；alice、bob 两个上下文。
- 步骤：收发内容为 `ZXQ-UI-LOG-1` 的消息，发送附言为 `ZXQ-UI-LOG-2` 的申请，收集全部 `console` 输出
- 预期结果：输出中搜不到两个特征串和 token

### TC-UI-90-14 生产响应头有 CSP 且无违规
- 关联 AC：AC-UI-90-13
- 优先级：P0
- 层级：Playwright
- 前置条件：生产构建（`vite preview` 加部署 CSP 配置）。**PRD-ui v1.3：CSP 配置按 NF-UI-14：`connect-src` 含账号接口，C2 起加具体存储域名；`img-src` C2 起加 `blob:`。**
- 步骤：
  1. 打开 `/login` 读取响应头
  2. 登录并完成收发消息，监听 `securitypolicyviolation`
- 预期结果：
  1. CSP 包含 `script-src 'self'` 和 `frame-ancestors 'none'`
  2. 没有违规报告

### TC-UI-90-15 减少动态效果时过渡时长为 0
- 关联 AC：AC-UI-90-14
- 优先级：P1
- 层级：Playwright
- 前置条件：已登录（alice），窄屏 375×812，测试模式 im-server（`IM_TEST_MODE=1 IM_DEMO_LOGIN=1`），Vite proxy 同源。`page.emulateMedia({reducedMotion:'reduce'})`。
- 步骤：读取弹窗、轻提示、菜单的 `transition-duration` / `animation-duration`，以及转圈图标
- 预期结果：前三者为 0s；转圈图标仍有动画

### TC-UI-90-16 不出现微信品牌词
- 关联 AC：AC-UI-90-15
- 优先级：P0
- 层级：前端单测
- 前置条件：CI 脚本。
- 步骤：
  1. 扫描 `src/`、`index.html`、`public/`、`dist/`（不区分大小写）
  2. 在临时文件中写入"WeChat"再扫描
- 预期结果：
  1. 当前代码没有"微信""WeChat""weixin"
  2. 写入后 CI 失败

---

## AC 覆盖

| AC | 用例 |
|---|---|
| AC-UI-01-1 | TC-UI-01-01 |
| AC-UI-01-3 | TC-UI-01-02 |
| AC-UI-01-4 | TC-UI-01-03 |
| AC-UI-01-5 | TC-UI-01-04 |
| AC-UI-01-6 | TC-UI-01-05 |
| AC-UI-01-7 | TC-UI-01-06、TC-UI-01-07 |
| AC-UI-01-8 | TC-UI-01-08 |
| AC-UI-02-5 | TC-UI-02-01 |
| AC-UI-02-6 | TC-UI-02-02 |
| AC-UI-02-8 | TC-UI-02-03 |
| AC-UI-03-1 | TC-UI-03-01 |
| AC-UI-03-2 | TC-UI-03-02 |
| AC-UI-03-3 | TC-UI-03-03 |
| AC-UI-03-4 | TC-UI-03-04 |
| AC-UI-03-5 | TC-UI-03-05 |
| AC-UI-03-6 | TC-UI-03-06 |
| AC-UI-03-7 | TC-UI-03-07 |
| AC-UI-03-8 | TC-UI-03-08 |
| AC-UI-03-10 | TC-UI-03-10 |
| AC-UI-04-1 | TC-UI-04-01 |
| AC-UI-04-2 | TC-UI-04-02 |
| AC-UI-04-3 | TC-UI-04-03 |
| AC-UI-04-4 | TC-UI-04-04 |
| AC-UI-04-5 | TC-UI-04-05 |
| AC-UI-04-6 | TC-UI-04-06 |
| AC-UI-04-7 | TC-UI-04-07 |
| AC-UI-04-9 | TC-UI-04-08 |
| AC-UI-05-1 | TC-UI-05-01 |
| AC-UI-05-2 | TC-UI-05-02 |
| AC-UI-05-3 | TC-UI-05-03 |
| AC-UI-05-5 | TC-UI-05-04 |
| AC-UI-05-6 | TC-UI-05-05 |
| AC-UI-05-7 | TC-UI-05-06 |
| AC-UI-06-1 | TC-UI-06-01 |
| AC-UI-06-2 | TC-UI-06-02 |
| AC-UI-06-3 | TC-UI-06-03 |
| AC-UI-06-4 | TC-UI-06-04 |
| AC-UI-06-6 | TC-UI-06-05、TC-UI-06-06 |
| AC-UI-06-7 | TC-UI-06-07 |
| AC-UI-07-1 | TC-UI-07-01 |
| AC-UI-07-2 | TC-UI-07-02 |
| AC-UI-07-3 | TC-UI-07-03 |
| AC-UI-07-4 | TC-UI-07-04 |
| AC-UI-07-5 | TC-UI-07-05 |
| AC-UI-07-6 | TC-UI-07-06 |
| AC-UI-07-7 | TC-UI-07-07 |
| AC-UI-08-1 | TC-UI-08-01 |
| AC-UI-08-2 | TC-UI-08-02 |
| AC-UI-08-3 | TC-UI-08-03、TC-UI-08-04 |
| AC-UI-08-4 | TC-UI-08-05 |
| AC-UI-08-5 | TC-UI-08-04、TC-UI-08-06 |
| AC-UI-09-1 | TC-UI-09-01、TC-UI-09-02 |
| AC-UI-09-3 | TC-UI-09-03 |
| AC-UI-09-4 | TC-UI-09-04 |
| AC-UI-09-6 | TC-UI-09-05 |
| AC-UI-09-7 | TC-UI-09-06 |
| AC-UI-09-8 | TC-UI-09-07 |
| AC-UI-10-2 | TC-UI-10-01 |
| AC-UI-10-3 | TC-UI-10-02 |
| AC-UI-10-4 | TC-UI-10-03 |
| AC-UI-10-5 | TC-UI-10-04 |
| AC-UI-10-6 | TC-UI-10-05 |
| AC-UI-10-7 | TC-UI-10-06 |
| AC-UI-10-8 | TC-UI-10-07 |
| AC-UI-12-1 | TC-UI-12-01 |
| AC-UI-12-2 | TC-UI-12-02 |
| AC-UI-12-3 | TC-UI-12-03 |
| AC-UI-12-5 | TC-UI-12-04 |
| AC-UI-13-1 | TC-UI-13-01 |
| AC-UI-13-2 | TC-UI-13-02 |
| AC-UI-13-3 | TC-UI-13-03 |
| AC-UI-13-4 | TC-UI-13-04 |
| AC-UI-13-5 | TC-UI-13-05 |
| AC-UI-13-6 | TC-UI-13-06 |
| AC-UI-13-7 | TC-UI-13-07 |
| AC-UI-13-8 | TC-UI-13-08 |
| AC-UI-13-9 | TC-UI-13-09 |
| AC-UI-13-10 | TC-UI-13-10 |
| AC-UI-13-11 | TC-UI-13-11 |
| AC-UI-14-1 | TC-UI-14-01 |
| AC-UI-14-2 | TC-UI-14-02、TC-UI-14-09 |
| AC-UI-14-3 | TC-UI-14-03 |
| AC-UI-14-4 | TC-UI-14-04 |
| AC-UI-14-5 | TC-UI-14-05 |
| AC-UI-14-6 | TC-UI-14-06 |
| AC-UI-14-7 | TC-UI-14-07、TC-UI-14-09 |
| AC-UI-14-8 | TC-UI-14-08、TC-UI-14-09 |
| AC-UI-14-9 | TC-UI-14-10 |
| AC-UI-14-10 | TC-UI-14-11 |
| AC-UI-14-11 | TC-UI-14-12 |
| AC-UI-15-1 | TC-UI-15-01 |
| AC-UI-15-2 | TC-UI-15-02 |
| AC-UI-15-3 | TC-UI-15-03 |
| AC-UI-16-1 | TC-UI-16-01 |
| AC-UI-16-2 | TC-UI-16-02、TC-UI-16-03 |
| AC-UI-16-3 | TC-UI-16-04 |
| AC-UI-16-4 | TC-UI-16-05 |
| AC-UI-16-6 | TC-UI-16-06 |
| AC-UI-17-1 | TC-UI-17-01 |
| AC-UI-18-1 | TC-UI-18-01 |
| AC-UI-18-2 | TC-UI-18-02、TC-UI-18-03 |
| AC-UI-18-3 | TC-UI-18-04 |
| AC-UI-19-1 | TC-UI-19-01 |
| AC-UI-19-2 | TC-UI-19-02 |
| AC-UI-19-3 | TC-UI-19-03 |
| AC-UI-19-4 | TC-UI-19-04、TC-UI-19-06 |
| AC-UI-19-5 | TC-UI-19-05、TC-UI-19-06 |
| AC-UI-20-1 | TC-UI-20-01 |
| AC-UI-20-2 | TC-UI-20-02 |
| AC-UI-20-3 | TC-UI-20-03 |
| AC-UI-21-1 | TC-UI-21-01 |
| AC-UI-21-2 | TC-UI-21-02 |
| AC-UI-21-3 | TC-UI-21-03 |
| AC-UI-21-4 | TC-UI-21-04 |
| AC-UI-21-5 | TC-UI-21-05 |
| AC-UI-22-3 | TC-UI-22-01 |
| AC-UI-22-4 | TC-UI-22-02 |
| AC-UI-23-1 | TC-UI-23-01、TC-UI-23-03 |
| AC-UI-23-2 | TC-UI-23-02、TC-UI-23-03 |
| AC-UI-23-3 | TC-UI-23-03、TC-UI-23-04 |
| AC-UI-23-4 | TC-UI-23-05 |
| AC-UI-25-1 | TC-UI-25-01 |
| AC-UI-25-2 | TC-UI-25-02 |
| AC-UI-25-3 | TC-UI-25-03 |
| AC-UI-25-4 | TC-UI-25-04 |
| AC-UI-25-5 | TC-UI-25-05 |
| AC-UI-25-6 | TC-UI-25-06 |
| AC-UI-25-7 | TC-UI-25-07 |
| AC-UI-25-8 | TC-UI-25-08 |
| AC-UI-26-1 | TC-UI-26-01 |
| AC-UI-26-2 | TC-UI-26-02 |
| AC-UI-26-3 | TC-UI-26-03 |
| AC-UI-26-4 | TC-UI-26-04 |
| AC-UI-26-5 | TC-UI-26-05 |
| AC-UI-26-6 | TC-UI-26-06 |
| AC-UI-26-7 | TC-UI-26-07 |
| AC-UI-27-1 | TC-UI-27-01 |
| AC-UI-27-2 | TC-UI-27-02 |
| AC-UI-27-3 | TC-UI-27-03 |
| AC-UI-27-4 | TC-UI-27-04 |
| AC-UI-27-5 | TC-UI-27-05 |
| AC-UI-27-6 | TC-UI-27-06 |
| AC-UI-27-7 | TC-UI-27-07 |
| AC-UI-27-8 | TC-UI-27-08 |
| AC-UI-28-1 | TC-UI-28-01 |
| AC-UI-28-2 | TC-UI-28-02 |
| AC-UI-28-3 | TC-UI-28-03 |
| AC-UI-28-4 | TC-UI-28-04 |
| AC-UI-28-5 | TC-UI-28-05 |
| AC-UI-29-1 | TC-UI-29-01 |
| AC-UI-29-2 | TC-UI-29-02 |
| AC-UI-29-3 | TC-UI-29-03 |
| AC-UI-30-1 | TC-UI-30-01 |
| AC-UI-30-2 | TC-UI-30-02 |
| AC-UI-30-3 | TC-UI-30-03 |
| AC-UI-31-1 | TC-UI-31-01 |
| AC-UI-31-2 | TC-UI-31-02 |
| AC-UI-31-3 | TC-UI-31-03 |
| AC-UI-31-4 | TC-UI-31-04 |
| AC-UI-31-5 | TC-UI-31-05 |
| AC-UI-31-6 | TC-UI-31-06 |
| AC-UI-32-1 | TC-UI-32-01 |
| AC-UI-32-2 | TC-UI-32-02 |
| AC-UI-32-3 | TC-UI-32-03 |
| AC-UI-32-4 | TC-UI-32-04 |
| AC-UI-33-1 | TC-UI-33-01 |
| AC-UI-33-2 | TC-UI-33-02 |
| AC-UI-33-3 | TC-UI-33-03 |
| AC-UI-33-4 | TC-UI-33-04、TC-UI-33-17、TC-UI-33-18 |
| AC-UI-33-5 | TC-UI-33-05 |
| AC-UI-33-6 | TC-UI-33-06 |
| AC-UI-33-9 | TC-UI-33-07、TC-UI-33-08、TC-UI-33-09 |
| AC-UI-33-10 | TC-UI-33-10、TC-UI-33-11 |
| AC-UI-33-11 | TC-UI-33-12、TC-UI-33-17 |
| AC-UI-33-12 | TC-UI-33-13、TC-UI-33-14 |
| AC-UI-33-13 | TC-UI-33-15 |
| AC-UI-33-14 | TC-UI-33-16 |
| AC-UI-90-1 | TC-UI-90-01 |
| AC-UI-90-2 | TC-UI-90-02 |
| AC-UI-90-3 | TC-UI-90-03、TC-UI-90-04、TC-UI-90-05 |
| AC-UI-90-4 | TC-UI-90-06 |
| AC-UI-90-5 | TC-UI-90-07 |
| AC-UI-90-6 | TC-UI-90-08 |
| AC-UI-90-7 | TC-UI-90-09、TC-UI-90-10 |
| AC-UI-90-8 | TC-UI-90-11 |
| AC-UI-90-11 | TC-UI-90-12 |
| AC-UI-90-12 | TC-UI-90-13 |
| AC-UI-90-13 | TC-UI-90-14 |
| AC-UI-90-14 | TC-UI-90-15 |
| AC-UI-90-15 | TC-UI-90-16 |

共 191 条 AC、209 个用例。
