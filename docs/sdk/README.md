# 统一 SDK 设计说明

> 适用于 [tianting-sdk-ts](https://github.com/leetonny001/tianting-sdk-ts)（H5、PC）、[tianting-sdk-kotlin](https://github.com/leetonny001/tianting-sdk-kotlin)（Android）和 [tianting-sdk-swift](https://github.com/leetonny001/tianting-sdk-swift)（iOS），各自一个仓库。账号（C1）的 SDK 部分三套都做完了（FE-600～602、FE-610～612，第 6 节的语义已经统一，结果都是本地的，CI 不运行）；界面（FE-613～618）还没开工，SDK 部分里没有验收的项见[任务单](../tasks/frontend-chat.md)的进度表。**三套 SDK 必须遵守同样的行为规则**，这份文档就是对照基准（Q7）。
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
测试替身：`@tianting/sdk/testing` 导出 `FakeTransport`、`FakeClock`，界面的测试也可以用。

## 3. 行为规则（三套 SDK 必须一致）

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

三套 SDK 和服务端跑同一批数据，保证规则一致：

| 文件 | 内容 | 服务端 | sdk-ts |
|---|---|---|---|
| `protocol/errors.json` | 错误码、关闭码、被踢原因 | 一致性测试 | 生成 `errors.gen.ts` + 一致性测试 |
| `protocol/vectors/text.json` | 去空白和计数（25 个空白字符、U+200B/U+FEFF、emoji、ZWJ、5000/5001） | `TestTextVectors` | `text.test.ts` |
| `protocol/vectors/frames.json` | 帧格式的合法 / 非法用例 | Schema 校验 | `frames-schema.test.ts`（Schema 校验，合法和非法都跑） |
| `protocol/schema/http-client.schema.json` | 账号接口的请求和响应 | `validateHTTP` | `account-http-schema.test.ts`（请求体）、`test/support/account.ts`（假响应） |
| `protocol/vectors/account-session.json` | `cases`：客户端判断。`serverCases`：规则 4 的判定表 | `server/e2e/accountmodel`、`account_vectors_test.go` | `account-vectors.test.ts`：`cases` 跑真实的 `AccountClient` + 假 HTTP + 假时钟；`serverCases` 约束假服务端 |

## 5. 测试

- 单元测试（`npm test`）：只用 `FakeTransport` + `FakeHttp` + `FakeClock`，毫秒级、结果确定；覆盖 FE-002～008、FE-600、FE-610 的 AC。
- 集成测试（tianting-sdk-ts 仓库的 `npm run test:integration`）：编译并启动真实的 im-server（测试模式），用标准 WebSocket 验证收发、未读已读、丢 ack 后重发、convAdded、强制下线、被服务端断开后的重连补拉。

## 6. 账号会话语义（FE-610～612，三套 SDK 一致）

依据 [PRD-chat](../prd/PRD-chat-v1.1.md) F-ACC-03 规则 5～7、F-ACC-04 规则 1、3、6，F-ACC-05 规则 1。代码入口：tianting-sdk-ts 的 `src/account-policy.ts`（纯判断，和 `server/e2e/accountmodel/model.go` 逐条对应）、`src/account.ts`（单飞、队列、定时器）、`src/connection.ts`（`onClosedWithProvider`）。**向量只覆盖 `model.go` 有的判断，下面标了"向量没有"的要靠各 SDK 自己的测试**，测试用例可以照 `test/account-session.test.ts` 翻译，用例名带 TC 编号。S1、K7、S3、K5（编号见任务单的问题表）在三套 SDK 里是同一组用例：sdk-ts `test/account-unified.test.ts`、sdk-kotlin `AccountUnifiedTest`、sdk-swift `AccountUnifiedTests`。

对外：`AccountClient`（HTTP，独立于 `IMClient`），`im.connect(account.tokenProvider)` 进入账号模式；`im.connect(token)` 是演示模式，行为不变。

### 刷新

| 规则 | 要点 |
|---|---|
| 什么时候刷新 | `accessExpiresAt - now < 6 小时`。恰好 6 小时不刷新，所以定时器要定在"到期前 6 小时 + 1 毫秒"（向量没有这个边界） |
| 四个入口 | `start`、`foreground`、`timer`、`kicked`，合并成一次。只有已连接才有定时器。HTTP 4011 也走同一个单飞 |
| 单飞 | 同一时间最多一个刷新请求。每个请求有序号，30 秒超时放弃后序号作废，这个序号的成功和失败响应都丢弃 |
| 落盘 | 响应到达 → 写存储 → 再比一次序号 → 才换 token。写存储失败按退避处理，继续用旧值（旧值还有效）。存储回调可以挂起，所以调用要按顺序执行，结束时的清除不能被更早的保存超过（S3） |
| 换 token | 已连接：断开前先让旧连接的回调失效，`close(1000)`，立即重连，状态不足 1 秒不提示。已断开在等退避：立即拨号 |
| 启动已过期 | 先刷新，`auth` 不发。拨号在等 token，状态是连接中 |
| 绝对上限 | `accessExpiresAt == absoluteExpiresAt`：不主动刷新，定时器定在到期时刻，到期回登录页。`kicked` 和 4011 仍然可以刷新（`model.go` 如此） |
| 本地时钟 | 过期判断用本地时钟。偏慢：服务端回 4011，走规则 7（向量没有） |
| 刷新回来已不足 6 小时（S1） | 时钟偏快或 TTL 配得短时，**刷新刚返回的** token 按本地时钟已经不足 6 小时：这个 token 不再主动刷新（`start`、`foreground`、`timer` 都不刷新，定时器不定），也不按本地时间当成过期（拨号和 HTTP 直接用它），等服务端的 4011。否则每次拨号和连接后的定时器都再刷新一次，换 token 又重连，一直循环到服务端限流。4011、`refresh()` 还是刷新，每个 4011 最多一次；下一个时间正常的 token 恢复。只看刷新的响应：登录、改密的响应不足 6 小时可以再刷新一次。这个标记不落盘，重启后从头判断，每次启动最多多一次刷新。启动已过期的主路径不变。到绝对上限的 token 不适用（向量没有） |

### 失败（`classifyRefreshFailure`）

| 结果 | 动作 |
|---|---|
| 网络错误、4290、5xx | 留在当前页面，退避 1、2、4…30 秒，±20% 抖动。PRD 没写参数，和重连相同 |
| HTTP 错误但响应体没有 `code`（代理的页面等）、200 但字段不全 | 按网络错误处理，**不回登录页**（向量没有） |
| 4019 `superseded` | 按 `details.retryAfterSeconds` 重试（上限 600 秒，防异常值），重试前重读本地存储，连续 3 次回登录页，刷新成功清零 |
| 4019 `replayDetected` | 弹窗 `credentialAnomaly`，回登录页 |
| 4019 其他、不认识的 reason、不认识的错误码 | 回登录页。有 `details.recoverBefore` 就提示日期 |
| 4034 | `banNotice`（兜底，第一批服务端不返回） |

**等重试时只有到点的定时器能发请求**：`start`、`foreground`、`kicked` 都不提前（向量没有：向量里的 `trigger timer` 就是"到点"）。否则切一次前台就破坏 `retryAfterSeconds`。`recoverBefore` 和 `retryAfterSeconds` 在响应的 `details` 里，向量的 `failure` 把它们写平了。

### 4011（规则 7）

比较的是**身份**：`sid` 不同就算本地更新，`sid` 相同才比 `gen`。**新 `sid` 的 `gen` 从 1 开始**（`service.go`），只比 `gen` 的话，改密之后旧连接的 4011 会被当成"本地不比它新"，多发一次刷新（向量没有，PRD 只说"代号"）。

- 本地更新：`useLocal`，直接用本地值重连或重试，不刷新。
- 否则：`mustRefresh` 置位，不看剩余时间刷新一次，等 token 变了再重连。这个标记要带到退避重试里，否则剩余时间充足时重试不会发生（向量没有）。
- 连接：关闭码 4011 没有 `kicked` 帧也一样。连续被拒绝时第一次立即重连，之后按退避（鉴权成功清零）（向量没有）。
- HTTP：同一个请求最多因为 4011 重试 2 次；等刷新最多 30 秒，超时把错误交给调用方，后台继续重试。

### 串行队列

刷新、改密、找回、退出其他设备、登录、注册、注销申请排在同一个队列，同一时间一个。PRD 只列了前四个，其他也会换 token 或吊销会话，一起排更保守。

- 刷新占队列的时候改密要等；改密占队列的时候定时器到期只做记号，改密结束再判断（成功换了 token 就不用刷新了）。
- **队列内的请求要先刷新时直接在自己的位置刷新，不重新排队**，否则就是等自己。收到 4011 同样如此。这个刷新失败就让这个操作失败。
- 任何结果（成功、失败、超时）都要释放队列。

### 改密期间的踢线（F-ACC-04 规则 3）

**真实服务端是先踢线后回响应**（`credentials.go` 提交后 `KickUserExcept`），所以缓存踢线是主路径，不是边角情况。

| 情况 | 动作 |
|---|---|
| 改密、退出其他设备、找回、注销申请的请求进行中（在队列里排着不算），收到 `passwordChanged`、`sessionRevoked`、`accountDeleted`，或者不认识的原因 + 4012 | 缓存，连接停着不重连，等响应（30 秒） |
| 成功 | 先换 token（新 `sid`），再忽略这个踢线，立即重连 |
| 失败、超时 | 踢线生效。超时之后到达的响应丢弃，即使它是成功的（"响应丢失时用新密码重新登录"） |
| 响应先到踢线后到 | 换 token 时旧连接的回调已失效，踢线到不了。还不够：连接在通知到达时可能还没开，之后才用旧 token 连上，这条连接的踢线不能结束新会话。见下面的 K5 |
| `banned`、`suspended` 等其他原因 | 不缓存，立即生效 |
| 退出登录进行中 | 服务端也会踢这条连接，缓存，退出后忽略 |

**旧 `sid` 的踢线（K5）**：连接用的 token 的 `sid` 和本地 `sid` 不同，这个踢线作废，用新 token 重连。

| SDK | 竞争 | 做法 |
|---|---|---|
| sdk-kotlin | 连接和账号在两条线程上，踢线可能比"token 换了"的通知晚一步（第一次跑真实服务端就遇到了） | `KickInfo.sid` |
| sdk-ts | 只有一条事件循环，但有一个微任务宽的窗口：拨号读到旧 token，`then` 回调运行之前换了 `sid`，这时没有连接可断，随后用旧 token 连上。逐拍错开能复现 | `KickInfo.sid` |
| sdk-swift | 没有。连接和账号都在主 actor 上，"token 换了"的通知同步断开旧连接；拨号读 token 到开连接之间没有挂起点，所以旧 token 不会比换 `sid` 活得久。`testK5_dialAroundTheSidChangeNeverEndsTheSession` 逐拍验证，在拨号里加一次让出就会失败 | 不需要，`KickInfo` 没有 `sid` |

### 其他

- **恢复码**：响应带 `recoveryCode` 时清掉存储里的旧值（改密后旧值已被吊销，杀 App 后应该回登录页，而不是拿旧值刷新再弹"凭证有变化"），新 token 只放内存；确认后才写。这段时间的刷新和退出其他设备也不写。收到 `recoveryCodeStale` 结束会话。确认时写到"存的就是在用的"才清只放内存的标记：写的时候刷新还是只放内存，写完发现 token 变了就再写一次；写失败抛给调用方，再调一次能重写（S3）。确认只对当前等确认的那一代生效：确认进行中改密、找回或重新生成换了代，旧一代的响应不写盘、不清标记，确认新一代才写（S4，RS5-13）。
- **落盘期间会话结束**：登录、改密等响应写存储之后比一次 `epoch`，会话已经结束就清存储并抛 `SessionEndedError`，不把结束的会话救回来（S3）。退出开始时 `epoch` 就加一，等 logout 响应时回来的响应也不采用。
- **会话已结束时连接（K7）**：`connect(tokenProvider)` 时会话已经结束（没登录过，或者 `start()` 的刷新收到 4019 之后 App 才连接），发 `fatal`（session 类），状态回到断开，不开连接，不再停在连接中。`SessionEndedError` 多了可选的 `end`，`accessToken()` 抛出时带上最近一次结束的原因，`fatal` 带同样的 `dialog`、`reason`、`code`，但没有 `origin`（这个 `fatal` 不是连接上的踢线发的）；没登录过时只有 `action: login`，退出之后是 `action: logout`。等 token 时会话才结束的，走原来的 `onEnded`，只有一次 `fatal`。App 之后再登录，再调一次 `connect` 就能连上。
- **退出**：先发 logout（Bearer 和 refresh 都带），再 1000 断开；连不上也本地退出。进行中的刷新作废，等响应的改密回来时不再采用（否则退出后会话又活了）。H5 没有"待退出"，原生要有（FE-611）。
- **退出进行中登录（问题表 A1、I6）**：logout 的收尾只作用于它开始时的那个会话，三套 SDK 相同，用例在 `account-unified.test.ts`、`AccountUnifiedTest`、`AccountUnifiedTests` 的 A1 组。
  - 登录、注册、找回、`restore` 算新会话，序号 `sessionSeq` 加一（**在写存储之前**）。logout 开始时记下序号，收尾时序号变了就不结束会话，也不发 `ended`，所以旧响应晚到（成功或失败）都不影响新会话和存储，App 不用等它。
  - logout 只在序号没变时清存储：写待退出期间登录成功，存储里的就是新 token（sdk-ts 没有待退出，logout 开始时也不清存储）。
  - 旧 sid 的待退出不受影响：确认了就删，网络错误、4290、5xx 留着。这时**不撤销系统推送令牌**（新会话正在用），改成立刻开始补发，旧 sid 靠补发解绑，所以这段时间旧 sid 的推送还可能到这台设备。
  - 新会话建立时放掉所有 logout 占的踢线缓存：旧连接已随"token 换了"断开，缓存的踢线作废；新会话的踢线立即生效，不等旧 logout。
  - logout **之前**发出、之后才回来的登录响应不采用（`SessionEndedError`），退出优先。服务端已经建的这条设备会话留到过期，界面走不到这条路（退出前没有登录页）。
  - App 不再等退出结束才能登录。旧 `IMClient` 要在新会话开始前关掉：它还绑着 `tokenProvider`，会用新 token 重连；它的 `disconnect` 清的本地库和新会话共用。H5 每次退出换一个 `AccountClient` 实例，本来就不等。
- **踢线的会话结束**：关闭码 4010、4012 的踢线清掉本地 token；`tooManyConnections` 不清（账号本身没问题）。`serverShutdown` 按退避重连。
- **不认识的踢线原因**：关闭码说可以重连就忽略；否则按关闭码结束，原因原样告诉界面（`moderation.test.ts` 已有用例）。
- **securityNotice**：放行服务端实际发的七种（`server/internal/account` 的 `notify` 调用）：`newDeviceLogin`、`passwordChanged`、`passwordRecovered`、`recoveryCodeRegenerated`、`refreshReplay`、`refreshRateLimited`、`newIpPrefix`。不认识的 `kind` 不发给 App。`loggedOutOthers`、注销等只写 `account_events`，不发这个帧。发起方所在的 `sid` 不提醒。改密、找回时其他设备先收到提醒，再收到 `kicked`。真实服务端的集成测试：`recoveryCodeRegenerated` 三套都有，`passwordChanged` 三套都有；`refreshReplay`、`refreshRateLimited`、`newIpPrefix`、`passwordRecovered` 只有 sdk-ts（`test/integration/security-notice.test.ts`）。
- **日志**：只有方法、路径、状态码、错误码、动作。token、refresh、恢复码、密码不进 URL、日志、事件和错误信息；传输层的原始错误不带进异常（错误信息可能有 URL）。

### HTTP 客户端不能隐式重发（S2、K4，第 21 节 16）

账号的 POST 不能重发：refresh 重发一次可能就是重放。下面都用真实 socket 数到达的请求，只在回环、HTTP/1.1 上测过，HTTP/2、HTTP/3、真实网关没有测。

| 客户端 | 已核实 | 做法 |
|---|---|---|
| URLSession（macOS） | 带 `WWW-Authenticate` 的 401 会让它再发一次 POST，默认处理、拒绝保护空间、空凭证都一样。连接断开、超时、取消、`503`/`429` 带 `Retry-After: 0`、`408`、`421` 时 POST 只到达 1 次，同样场景下 GET 会重发 | HTTP 认证质询直接取消请求，按网络错误处理。代价是带这个头时 4019、4018 读不到，只会一直退避，所以**网关不能给 `/v1/` 加这个头**（[部署手册](../../deploy/ACCOUNT-RUNBOOK.md)第 1 节）。`URLSessionRetryTests` |
| OkHttp | 默认配置下，复用的连接断在半路时 POST 会被重发；只关 `retryOnConnectionFailure` 不够，`503 + Retry-After: 0` 还是会重发 | 关 `retryOnConnectionFailure` 和重定向，请求体标记 one-shot。`OkHttpRetryTest` |

### 文件位置

| 内容 | 位置 |
|---|---|
| 向量 | `protocol/vectors/account-session.json`（各 SDK 用 `scripts/sync-protocol.sh` 同步副本，`protocol/SOURCE` 记提交号） |
| Schema | `protocol/schema/http-client.schema.json`、`frame.schema.json` |
| 判断的基准 | `server/e2e/accountmodel/model.go`（客户端）、`server.go`（服务端判定表） |
| sdk-ts 的执行器 | `test/account-vectors.test.ts`、`test/support/fake-account-server.ts` |
