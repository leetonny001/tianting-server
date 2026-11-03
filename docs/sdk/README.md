# 统一 SDK 设计说明

> 适用于 `sdk-ts`（H5、PC，已实现）和 `sdk-kmp`（Android、iOS，待实现）。**两套 SDK 必须遵守同样的行为规则**，这份文档就是对照基准（Q7）。
> 依据：[PRD v1.2](../prd/PRD-core-v1.2.md)、[协议](../../protocol/README.md)、[错误码](../../protocol/ERRORS.md)、[前端任务单](../tasks/frontend.md)。

## 1. 分层

```
界面（React / Compose / SwiftUI）
   │  订阅事件、读取快照、调用方法
IMClient ─────────── 对外的唯一入口（PLAN 3.2）
   ├─ 发送队列        发送状态机、离线排队、自动补发一次、手动重发
   ├─ 同步引擎        convs、补拉、缺口检测、历史翻页、会话事件
   ├─ 已读上报        可见 + 停在会话才报，1 秒节流
   ├─ Store           会话和消息（只在内存，D4），按 msgId 去重、按 clientMsgId 合并
   └─ Connection      首帧 auth、心跳、退避重连、被踢、请求 / ack 匹配
        └─ Transport  WebSocket；测试时换成 FakeTransport
Clock                 所有定时逻辑都走它；测试时换成 FakeClock
```

## 2. 对外接口（sdk-ts）

```ts
const im = new IMClient({ url: 'wss://im.example.com/ws', platform: 'web' });
im.on('conversations', (list) => …);        // 会话列表（已排序）
im.on('messages', (convId, list) => …);     // 某个会话的消息
im.on('state', (s) => …);                   // disconnected | connecting | connected
im.on('banner', (show) => …);               // 是否显示"连接中…/网络不可用"（断开超过 1 秒才为 true）
im.on('convRemoved', (convId) => …);        // 被移出群
im.on('fatal', (reason) => …);              // 被踢 / 不可重连：回到登录页
im.connect(token);

im.send(convId, text);                      // 返回 { ok, message } 或 { ok:false, reason:'empty'|'tooLong', over }
im.retry(convId, clientMsgId);              // 手动重发失败的消息
await im.loadHistory(convId);               // 向前翻页，返回是否还有更早的
im.setActiveConversation(convId | null);    // 进入 / 离开聊天页（已读上报依据）
im.disconnect();                            // 退出登录
```

工具：`c2cConvId(a, b)`、`groupConvId(gid)`、`checkText`、`trimText`、`countCodePoints`、`describeError(code)`、`ErrorCode`。
测试替身：`@im-mvp/sdk/testing` 导出 `FakeTransport`、`FakeClock`，界面的测试也可以用。

## 3. 行为规则（两套 SDK 必须一致）

### 连接（FE-003）

| 规则 | 依据 |
|---|---|
| URL 只带 `platform`；连接后第一帧发 `auth`（含 token），收到它的 ack 才进入"已连接"。token 不进 URL、不进日志 | C-52 |
| 每 30 秒 ping；发出后 10 秒没收到 pong 判定断线，按断线处理 | F-CONN-01 |
| 重连退避 1、2、4、8、16、30 秒，上限 30 秒是加抖动**之前**的值，再加 ±20% 抖动；鉴权成功后重置 | F-CONN-01、C-22 |
| 网络恢复（浏览器 `online`）时立即重连 | F-CONN-01 规则 3 |
| 收到 `kicked` 后不重连，例外 `serverShutdown` 按退避重连 | F-AUTH-03、C-55 |
| 关闭码 1000、4009～4012、4429 不重连；其他（含 1006、4401、4408、4400、1001）按退避重连 | ERRORS.md 第 4 节 |
| 断开持续超过 1 秒才显示连接提示，恢复后立即消失 | F-CONN-02 |
| **WebSocket 客户端不能限制下行帧大小**（很多库默认 32KB；500 个会话的 convs 约 125KB） | C-07 |

### 同步（FE-004）

| 规则 | 依据 |
|---|---|
| 每次连接成功（含重连）：先 `convs`，再对本地**已经加载过消息**、且服务端 maxSeq 更大的会话补拉 | F-CONN-01 规则 4 |
| 补拉起点 = max(本地 maxSeq, minSeq − 1)；缺口超过 200 条时清掉本地已加载的服务端消息，只加载最新 20 条，更早的靠翻页 | C-24 |
| 首次登录不为每个会话拉历史；打开会话时 `loadHistory`（第一次 `beforeSeq = maxSeq + 1, limit = 20`） | 实现约定 |
| 实时消息出现 seq 缺口后等 500ms，仍未补上就 `sync` | F-MSG-04 规则 2 |
| 按 `msgId` 去重；用 `clientMsgId` 把服务端回来的消息与本地待发消息合并 | F-MSG-04 规则 1 |
| 会话按 `updatedAt` 倒序、`convId` 字节序升序 | F-CONV-01 |
| 收到本地没有的会话的消息（第一次和某人聊、群消息比 `convAdded` 先到）：先补临时会话，200ms 后重新拉 `convs` 用服务端数据替换 | 实现约定 |
| 处理 `convAdded`、`convRemoved`、`readUpdated` | F-GRP-03、F-SYNC-02 |

### 发送（FE-005）

| 规则 | 依据 |
|---|---|
| 在线：发送中 → 收到 ack 已发送；10 秒无 ack 或收到错误 → 失败（错误码透传给界面） | F-MSG-03 |
| 离线：进入待发队列，重连后自动发 1 次；发出后连接断开的也算 1 次自动补发；补发失败后只能手动重发 | F-MSG-03 规则 2 |
| 手动重发沿用原 `clientMsgId`（服务端去重，返回第一次的 msgId 和 seq） | F-MSG-03 规则 2 |
| 超时标记失败后 ack 才到：以 ack 为准，变为已发送 | C-30 |
| 被踢时发送中和排队的消息全部标记失败 | AC-AUTH-03-6 |
| 未确认的消息排在所有有 seq 的消息之后 | F-MSG-03 规则 3 |
| 文本：去首尾空白（Unicode White_Space，不能用 JS 的 `\s`）后按码点计数，1～5000 | 3.1 节、C-26 |

### 已读与未读（FE-006）

| 规则 | 依据 |
|---|---|
| 页面可见、并且停在该会话时才上报，节流 1 秒；上报当前最新 seq，本地立即清零 | F-CONV-02 规则 3 |
| 未读 = maxSeq − max(已读基线, readSeq)，已读基线 = 同步时服务端的 maxSeq − unread（即 max(readSeq, joinSeq)）。历史可见不影响未读 | C-02 |
| 自己在其他端发的消息不增加未读，并推进已读 | C-34 |
| 已读只进不退 | F-CONV-02 规则 4 |

## 4. 共享的测试数据

两套 SDK 和服务端跑同一批数据，保证规则一致：

| 文件 | 内容 | 服务端 | sdk-ts |
|---|---|---|---|
| `protocol/errors.json` | 错误码、关闭码、被踢原因 | 一致性测试 | 生成 `errors.gen.ts` + 一致性测试 |
| `protocol/vectors/text.json` | 去空白和计数（25 个空白字符、U+200B/U+FEFF、emoji、ZWJ、5000/5001） | `TestTextVectors` | `text.test.ts` |
| `protocol/vectors/frames.json` | 帧格式的合法 / 非法用例 | Schema 校验 | 待补（sdk-kmp 开始时一起做） |

## 5. 测试

- 单元测试（`npm test`）：只用 `FakeTransport` + `FakeClock`，82 个用例，毫秒级、结果确定；覆盖 FE-002～008 的 AC。
- 集成测试（`npm run test:integration -w sdk-ts`）：编译并启动真实的 im-server（测试模式），用标准 WebSocket 验证收发、未读已读、丢 ack 后重发、convAdded、强制下线、被服务端断开后的重连补拉。
