# 测试用例：群聊（GRP）

> 依据：[PRD v1.0](../prd/PRD-core-v1.0.md) F-GRP-01～F-GRP-03、[ERRORS.md](../../protocol/ERRORS.md)。
>
> 通用约定：
> - 每个用例使用独立数据库，服务端测试模式。
> - 用户 `alice`、`bob`、`carol`、`dave` 已导入；`ghost` 不存在。批量用户 `u0001`～`u2001` 在需要时预先分批（每批 500）导入。
> - 群会话 ID 为 `group:{groupId}`。管理接口均携带正确的管理密钥。
> - "建群 / 加成员 / 移除成员 / 修改设置 / 查询群"指 PRD §8 新增的管理接口（具体路径以 `protocol/` 落地为准）。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-GRP-01-01 | 建群后初始成员都能发群消息 | AC-GRP-01-1 | 后端 e2e | P0 |
| TC-GRP-01-02 | 建空群成功，之后加入的成员可发消息 | AC-GRP-01-1 | 后端 e2e | P0 |
| TC-GRP-01-03 | 重复建同一个群 ID 返回 409 / 4090，原群不变 | AC-GRP-01-2 | 后端 e2e | P0 |
| TC-GRP-01-04 | 建群 2000 人成功 | AC-GRP-01-3 | 后端 e2e | P0 |
| TC-GRP-01-05 | 建群 2001 人返回 400 / 4130，群未创建 | AC-GRP-01-3 | 后端 e2e | P0 |
| TC-GRP-01-06 | 已有 1999 人时加 2 人返回 4130，一人都不加 | AC-GRP-01-3 | 后端 e2e | P0 |
| TC-GRP-01-07 | 已有 1999 人加 1 人成功（2000），再加 1 人返回 4130 | AC-GRP-01-3 | 后端 e2e | P0 |
| TC-GRP-01-08 | 单次加成员 501 人返回 400 / 4131，500 人成功 | AC-GRP-01-3 | 后端 e2e | P0 |
| TC-GRP-01-09 | 被移除的成员发消息 4033、拉取 4033、收不到新消息 | AC-GRP-01-4 | 后端 e2e | P0 |
| TC-GRP-01-10 | 查询群名、historyVisible、成员列表 | AC-GRP-01-5 | 后端 e2e | P0 |
| TC-GRP-01-11 | 对不存在的群做查询 / 加 / 删 / 改设置返回 404 / 4043 | AC-GRP-01-5 | 后端 e2e | P0 |
| TC-GRP-01-12 | historyVisible=false：新成员 D 只能拉到入群后的消息，minSeq=11、未读 0 | AC-GRP-01-6 | 后端 e2e | P0 |
| TC-GRP-01-13 | historyVisible=false：D 向上翻页到 minSeq 时 hasMore=false | AC-GRP-01-6 | 后端 e2e | P0 |
| TC-GRP-01-14 | historyVisible=false 且群里没有消息时入群，minSeq=1 | AC-GRP-01-6 | 后端 e2e | P0 |
| TC-GRP-01-15 | historyVisible=true：D 能拉到全部 10 条，minSeq=1 | AC-GRP-01-7 | 后端 e2e | P0 |
| TC-GRP-01-16 | false 改为 true 后，D 下次拉取能看到入群前的消息 | AC-GRP-01-8 | 后端 e2e | P0 |
| TC-GRP-01-17 | true 改回 false 后，D 立即看不到入群前的消息 | AC-GRP-01-8 | 后端 e2e | P0 |
| TC-GRP-01-18 | 设置切换不影响建群时的初始成员（joinSeq=0） | AC-GRP-01-8 | 后端 e2e | P0 |
| TC-GRP-01-19 | 移除后再加入（false）：只能看到重新加入之后的消息 | AC-GRP-01-9 | 后端 e2e | P0 |
| TC-GRP-01-20 | 移除后再加入（true）：能看到全部历史，包括被移除期间的消息 | AC-GRP-01-9 | 后端 e2e | P0 |
| TC-GRP-01-21 | 加成员包含不存在的 userId：404 / 4042，一人都不加 | AC-GRP-01-10 | 后端 e2e | P0 |
| TC-GRP-01-22 | 建群初始成员包含不存在的 userId：404 / 4042，群未创建 | AC-GRP-01-10 | 后端 e2e | P0 |
| TC-GRP-01-23 | 群名边界：空 / 纯空白 / 51 字返回 400 / 4000；1 / 50 字成功 | AC-GRP-01-11 | 后端 e2e | P0 |
| TC-GRP-01-24 | 修改群设置时群名不合法返回 400 / 4000 | AC-GRP-01-11 | 后端 e2e | P0 |
| TC-GRP-01-25 | 群 ID 不合法返回 400 / 4004；长度 64 成功 | AC-GRP-01-11 | 后端 e2e | P0 |
| TC-GRP-02-01 | 成员发群消息：其他在线成员都收到，发送设备不重复收到（界面） | AC-GRP-02-1 | UI 自动化 | P0 |
| TC-GRP-02-02 | 群消息投递：其他成员和发送者其他端收到，发送连接只收 ack | AC-GRP-02-1 | 后端 e2e | P0 |
| TC-GRP-02-03 | 群消息同一 clientMsgId 重发，成员只收到一条 | AC-GRP-02-1 | 后端 e2e | P0 |
| TC-GRP-02-04 | 非成员发群消息返回 4033，不保存 | AC-GRP-02-2 | 后端 e2e | P0 |
| TC-GRP-02-05 | 给不存在的群发消息 4043；空群非成员发消息 4033 | AC-GRP-02-3 | 后端 e2e | P0 |
| TC-GRP-02-06 | 群消息显示发送者 | AC-GRP-02-4 | UI 自动化 | P0 |
| TC-GRP-02-07 | 发群消息收到 4033：提示"你已不在该群"并禁用输入框 | AC-GRP-02-5 | UI 自动化 | P0 |
| TC-GRP-03-01 | 在线用户被拉进群，所有连接 1 秒内收到 convAdded | AC-GRP-03-1 | 后端 e2e | P0 |
| TC-GRP-03-02 | 建群时在线的初始成员收到 convAdded | AC-GRP-03-1 | 后端 e2e | P0 |
| TC-GRP-03-03 | 在线用户被移出群，所有连接收到 convRemoved，其他成员不收到 | AC-GRP-03-2 | 后端 e2e | P0 |
| TC-GRP-03-04 | 收到 convAdded 后会话列表出现该群 | AC-GRP-03-3 | UI 自动化 | P0 |
| TC-GRP-03-05 | 收到 convRemoved 后打开该群显示"你已不在该群"，输入框禁用 | AC-GRP-03-3 | UI 自动化 | P0 |

---

## F-GRP-01 群管理（管理接口）

### TC-GRP-01-01 建群后初始成员都能发群消息
- 关联 AC：AC-GRP-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无群 g1。
- 步骤：
  1. 建群 `{groupId:"g1", name:"活动群", members:["alice","bob","carol"]}`。
  2. alice、bob、carol 各连接并依次给 g1 发 1 条消息。
- 预期结果：
  1. 建群返回 HTTP 200。
  2. 3 条消息都成功，ack.convId = `group:g1`，seq 依次为 1、2、3。
  3. 每个成员都收到另外两人的消息。

### TC-GRP-01-02 建空群成功，之后加入的成员可发消息
- 关联 AC：AC-GRP-01-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：
  1. 建群 `{groupId:"gEmpty", name:"空群", members:[]}`。
  2. 查询 gEmpty；把 alice 加入；alice 发消息。
- 预期结果：建群 200；查询成员列表为空、`historyVisible=false`（默认值）；alice 发消息成功，seq=1。

### TC-GRP-01-03 重复建同一个群 ID 返回 409 / 4090，原群不变
- 关联 AC：AC-GRP-01-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：已建 g1（name="活动群"，成员 alice、bob）。
- 步骤：
  1. 再次建群 `{groupId:"g1", name:"新名字", members:["carol"]}`。
  2. 查询 g1。
- 预期结果：
  1. HTTP 409，响应体 `{"code":4090, ...}`。
  2. g1 群名仍为"活动群"，成员仍为 alice、bob（carol 未加入）。

### TC-GRP-01-04 建群 2000 人成功
- 关联 AC：AC-GRP-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u0001～u2000 已导入。
- 步骤：建群 gBig，members = u0001～u2000（2000 人，一次请求）；查询 gBig；u2000 发一条消息。
- 预期结果：HTTP 200；成员数 = 2000；u2000 发送成功（seq=1）。（单次批量 500 的限制是否适用于建群，见"疑问"Q1。）

### TC-GRP-01-05 建群 2001 人返回 400 / 4130，群未创建
- 关联 AC：AC-GRP-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：u0001～u2001 已导入。
- 步骤：建群 gBig，members = u0001～u2001；然后查询 gBig。
- 预期结果：建群返回 HTTP 400 / 4130；查询返回 HTTP 404 / 4043（群未创建）。

### TC-GRP-01-06 已有 1999 人时加 2 人返回 4130，一人都不加
- 关联 AC：AC-GRP-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：gBig 已有 u0001～u1999（1999 人）。
- 步骤：加成员 `[u2000, u2001]`；查询 gBig；u2000 给 gBig 发消息。
- 预期结果：加成员返回 HTTP 400 / 4130；成员数仍为 1999，不含 u2000、u2001；u2000 发消息返回 4033。

### TC-GRP-01-07 已有 1999 人加 1 人成功（2000），再加 1 人返回 4130
- 关联 AC：AC-GRP-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：gBig 已有 1999 人。
- 步骤：加 `[u2000]`；再加 `[u2001]`；移除 u0001 后再加 `[u2001]`。
- 预期结果：第 1 次 200，成员 2000；第 2 次 400 / 4130，成员仍 2000；移除后再加成功，成员 2000。

### TC-GRP-01-08 单次加成员 501 人返回 400 / 4131，500 人成功
- 关联 AC：AC-GRP-01-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：空群 gBatch；u0001～u0501 已导入。
- 步骤：一次加 u0001～u0501；再一次加 u0001～u0500。
- 预期结果：第 1 次 HTTP 400 / 4131，成员数 0；第 2 次 200，成员数 500。

### TC-GRP-01-09 被移除的成员发消息 4033、拉取 4033、收不到新消息
- 关联 AC：AC-GRP-01-4
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob、carol，已有 3 条消息；三人在线。
- 步骤：
  1. 移除 carol。
  2. alice 发消息（seq=4）。
  3. carol 给 g1 发消息；carol sync `group:g1` afterSeq=0；carol 上报 `read {convId:"group:g1", seq:3}`。
- 预期结果：
  1. carol 在 500ms 内没有收到 seq=4 的 `msg`。
  2. carol 发消息返回 4033；sync 返回 4033（不返回原来能看到的 1～3）；read 返回 4033（见"疑问"Q6）。
  3. bob 正常收到 seq=4。

### TC-GRP-01-10 查询群名、historyVisible、成员列表
- 关联 AC：AC-GRP-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：建群 `{groupId:"g1", name:"活动群", members:["alice","bob"], historyVisible:true}`。
- 步骤：查询 g1；修改群名为"新群名"；加 carol；移除 bob；再查询。
- 预期结果：
  1. 第 1 次：name="活动群"、historyVisible=true、成员 = {alice, bob}。
  2. 第 2 次：name="新群名"、historyVisible=true、成员 = {alice, carol}。

### TC-GRP-01-11 对不存在的群做查询 / 加 / 删 / 改设置返回 404 / 4043
- 关联 AC：AC-GRP-01-5
- 优先级：P0
- 层级：后端 e2e
- 前置条件：群 g404 不存在。
- 步骤：对 g404 分别调用查询、加成员 `[alice]`、移除成员 `[alice]`、修改设置 `{historyVisible:true}`。
- 预期结果：均返回 HTTP 404 / 4043；之后 g404 仍不存在（加成员没有隐式建群）。

### TC-GRP-01-12 historyVisible=false：新成员 D 只能拉到入群后的消息，minSeq=11、未读 0
- 关联 AC：AC-GRP-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（historyVisible=false）成员 alice、bob，已有 10 条消息（seq 1～10）。
- 步骤：
  1. 加 dave；dave 连接，执行 `convs`、`sync {convId:"group:g1", afterSeq:0}`。
  2. alice 发 1 条（seq=11）；dave 再执行 `convs`、`sync afterSeq:0`。
- 预期结果：
  1. 第 1 步：convs 中 g1 的 maxSeq=10、minSeq=11、unread=0；sync 返回空数组、hasMore=false。
  2. 第 2 步：dave 实时收到 seq=11；convs 中 unread=1；sync 只返回 seq=11。

### TC-GRP-01-13 historyVisible=false：D 向上翻页到 minSeq 时 hasMore=false
- 关联 AC：AC-GRP-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：接 TC-GRP-01-12，群里再发 2 条，共 seq 1～13，dave 的 minSeq=11。
- 步骤：dave 执行 `sync {beforeSeq:14, limit:20}`、`sync {beforeSeq:11, limit:20}`、`sync {afterSeq:5}`。
- 预期结果：
  1. 第 1 个返回 seq 11～13，hasMore=false。
  2. 第 2 个返回空数组，hasMore=false（不报错，也不泄露 seq ≤ 10）。
  3. 第 3 个只返回 seq 11～13。

### TC-GRP-01-14 historyVisible=false 且群里没有消息时入群，minSeq=1
- 关联 AC：AC-GRP-01-6
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g2（historyVisible=false）成员 alice，无消息。
- 步骤：加 dave；dave 执行 convs；alice 发 1 条；dave sync afterSeq=0。
- 预期结果：入群时 convs 中 g2 maxSeq=0、minSeq=1、unread=0；之后 dave 能拉到 seq=1，unread=1。

### TC-GRP-01-15 historyVisible=true：D 能拉到全部 10 条，minSeq=1
- 关联 AC：AC-GRP-01-7
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（historyVisible=true）已有 10 条消息。
- 步骤：加 dave；dave 执行 convs 和 sync afterSeq=0。
- 预期结果：convs 中 g1 minSeq=1、maxSeq=10；sync 返回 seq 1～10；unread 的值记录下来，与产品确认（按公式应为 10，见"疑问"Q3）。

### TC-GRP-01-16 false 改为 true 后，D 下次拉取能看到入群前的消息
- 关联 AC：AC-GRP-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-GRP-01-12 第 1 步完成（dave minSeq=11，sync 为空）。
- 步骤：修改 g1 设置 `historyVisible=true`；dave 不重连，直接执行 sync afterSeq=0 和 convs。
- 预期结果：sync 返回 seq 1～10；convs 中 minSeq=1（修改在查询时立即生效，无需重新入群或重连）。

### TC-GRP-01-17 true 改回 false 后，D 立即看不到入群前的消息
- 关联 AC：AC-GRP-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：接 TC-GRP-01-16（historyVisible=true，dave 能看到 1～10）。
- 步骤：修改设置 `historyVisible=false`；dave 执行 sync afterSeq=0、`sync {beforeSeq:11}`、convs。
- 预期结果：sync afterSeq=0 返回空；beforeSeq=11 返回空、hasMore=false；convs 中 minSeq=11。

### TC-GRP-01-18 设置切换不影响建群时的初始成员（joinSeq=0）
- 关联 AC：AC-GRP-01-8
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 建群时成员 alice（joinSeq=0），historyVisible=true，已有 10 条消息。
- 步骤：修改为 false；alice sync afterSeq=0；alice 执行 convs。
- 预期结果：alice 仍能拉到 seq 1～10；minSeq=1。

### TC-GRP-01-19 移除后再加入（false）：只能看到重新加入之后的消息
- 关联 AC：AC-GRP-01-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（false）已有 10 条；dave 入群（joinSeq=10）；之后群里发 seq 11～15。
- 步骤：
  1. 移除 dave；群里发 seq 16～20。
  2. 再加 dave；dave 执行 convs、sync afterSeq=0。
  3. 群里发 seq=21；dave 再 sync afterSeq=0。
- 预期结果：
  1. 第 2 步 convs 中 minSeq=21；sync 为空（看不到 11～15，即使第一次入群时看过）。
  2. 第 3 步 sync 只返回 seq=21。

### TC-GRP-01-20 移除后再加入（true）：能看到全部历史，包括被移除期间的消息
- 关联 AC：AC-GRP-01-9
- 优先级：P0
- 层级：后端 e2e
- 前置条件：同 TC-GRP-01-19，但 historyVisible=true。
- 步骤：重新加入后 dave sync afterSeq=0 limit=200。
- 预期结果：返回 seq 1～20；minSeq=1。

### TC-GRP-01-21 加成员包含不存在的 userId：404 / 4042，一人都不加
- 关联 AC：AC-GRP-01-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice；carol、dave 在线。
- 步骤：加成员 `["carol","ghost","dave"]`；查询 g1。
- 预期结果：HTTP 404 / 4042；成员仍只有 alice；carol、dave 没有收到 `convAdded`。

### TC-GRP-01-22 建群初始成员包含不存在的 userId：404 / 4042，群未创建
- 关联 AC：AC-GRP-01-10
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无 g3。
- 步骤：建群 `{groupId:"g3", name:"x", members:["alice","ghost"]}`；查询 g3；再用合法成员建 g3。
- 预期结果：第 1 步 HTTP 404 / 4042；查询 404 / 4043；再建 g3 成功（没有残留半个群导致 4090）。

### TC-GRP-01-23 群名边界：空 / 纯空白 / 51 字返回 400 / 4000；1 / 50 字成功
- 关联 AC：AC-GRP-01-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：分别建群，name 为：`""`、`"   "`、`"群"×51`、不传 name、`"群"`、`"群"×50`、`"😀"×50`、`"  "+"群"×50+"  "`。
- 预期结果：
  1. 前 4 种返回 HTTP 400 / 4000，群未创建。
  2. 后 4 种成功；最后一种查询到的群名为 50 个"群"（按 §3.1 去首尾空白、按码点计数，见"疑问"Q7）。

### TC-GRP-01-24 修改群设置时群名不合法返回 400 / 4000
- 关联 AC：AC-GRP-01-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 name="活动群"。
- 步骤：修改群名为 `""`、`"群"×51`。
- 预期结果：均返回 HTTP 400 / 4000；查询 g1 仍为"活动群"。

### TC-GRP-01-25 群 ID 不合法返回 400 / 4004；长度 64 成功
- 关联 AC：AC-GRP-01-11
- 优先级：P0
- 层级：后端 e2e
- 前置条件：无。
- 步骤：groupId 分别为 `""`、`"g 1"`、`"群1"`、`"g.1"`、65 个 `g`、64 个 `g`、`"G_1-a"`。
- 预期结果：前 5 种 HTTP 400 / 4004；后 2 种成功。

## F-GRP-02 群消息收发

### TC-GRP-02-01 成员发群消息：其他在线成员都收到，发送设备不重复收到（界面）
- 关联 AC：AC-GRP-02-1
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g1 成员 alice、bob、carol；三个浏览器上下文分别登录并打开 `/chat/group:g1`。
- 步骤：alice 发送"大家好"。
- 预期结果：
  1. bob、carol 1 秒内看到"大家好"，发送者为 alice。
  2. alice 页面只有 1 条"大家好"（ack 后状态为已发送，没有因收到 `msg` 再多一条）。

### TC-GRP-02-02 群消息投递：其他成员和发送者其他端收到，发送连接只收 ack
- 关联 AC：AC-GRP-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob、carol；alice 以 web、pc 在线；bob、carol 在线；dave（非成员）在线。
- 步骤：alice(web) 发送 `cid-grp-0001`"hello group"。
- 预期结果：
  1. bob、carol、alice(pc) 各收到 1 个 `msg`：convId=`group:g1`、from=alice、seq=1、msgId=ack.msgId。
  2. alice(web) 200ms 内没有收到 `msg`；dave 没有收到 `msg`。

### TC-GRP-02-03 群消息同一 clientMsgId 重发，成员只收到一条
- 关联 AC：AC-GRP-02-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob。
- 步骤：alice 用 `cid-grp-dup1` 连发 3 次。
- 预期结果：3 个 ack 的 msgId、seq 相同；bob 只收到 1 个 `msg`；sync 只有 1 条。

### TC-GRP-02-04 非成员发群消息返回 4033，不保存
- 关联 AC：AC-GRP-02-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob；dave 不是成员。
- 步骤：dave 给 g1 发消息；alice sync `group:g1` afterSeq=0。
- 预期结果：dave 收到 4033；alice、bob 没有收到 `msg`；sync 返回 0 条。

### TC-GRP-02-05 给不存在的群发消息 4043；空群非成员发消息 4033
- 关联 AC：AC-GRP-02-3
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g404 不存在；gEmpty 已存在且成员为空。
- 步骤：alice 分别给 g404、gEmpty 发消息。
- 预期结果：g404 返回 4043；gEmpty 返回 4033（不是 4043）。

### TC-GRP-02-06 群消息显示发送者
- 关联 AC：AC-GRP-02-4
- 优先级：P0
- 层级：UI 自动化
- 前置条件：g1 成员 alice、bob、carol；alice 设置了昵称"小A"，bob 没有昵称；carol 打开 `/chat/group:g1`。
- 步骤：alice、bob 各发一条消息。
- 预期结果：carol 页面中 alice 的消息显示发送者"小A"（若 F-USER-02 尚未交付则显示 `alice`），bob 的消息显示 `bob`；单聊页面不显示发送者名称不作要求。

### TC-GRP-02-07 发群消息收到 4033：提示"你已不在该群"并禁用输入框
- 关联 AC：AC-GRP-02-5
- 优先级：P0
- 层级：UI 自动化
- 前置条件：dave 已登录并打开 `/chat/group:g1`；使用 `routeWebSocket` 屏蔽下发给 dave 的 `convRemoved`，然后通过管理接口移除 dave（保证页面只能通过发送收到 4033）。
- 步骤：dave 发送"我还在吗"。
- 预期结果：该消息标记为发送失败；提示"你已不在该群"；输入框和发送按钮变为禁用。

## F-GRP-03 群成员变动通知

### TC-GRP-03-01 在线用户被拉进群，所有连接 1 秒内收到 convAdded
- 关联 AC：AC-GRP-03-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1（name="活动群"，historyVisible=false）已有 5 条消息；dave 以 web、pc 在线。
- 步骤：记录时间 T0，加 dave。
- 预期结果：
  1. dave 两个连接都在 T0+1s 内收到 `event {type:"convAdded"}`。
  2. 事件中的会话信息与 convs 单项结构一致：convId=`group:g1`、type=group、name="活动群"、maxSeq=5、minSeq=6、unread=0、updatedAt 有值。
  3. 已在群里的 alice、bob 不收到 `convAdded`。

### TC-GRP-03-02 建群时在线的初始成员收到 convAdded
- 关联 AC：AC-GRP-03-1
- 优先级：P0
- 层级：后端 e2e
- 前置条件：alice、bob 在线；无 g5。
- 步骤：建群 `{groupId:"g5", name:"新群", members:["alice","bob"]}`。
- 预期结果：alice、bob 1 秒内收到 `convAdded`（convId=`group:g5`、maxSeq=0、minSeq=1）。（PRD 只写"被加入群时"，建群是否下发见"疑问"Q4。）

### TC-GRP-03-03 在线用户被移出群，所有连接收到 convRemoved，其他成员不收到
- 关联 AC：AC-GRP-03-2
- 优先级：P0
- 层级：后端 e2e
- 前置条件：g1 成员 alice、bob、dave；dave 以 web、pc 在线；alice 在线。
- 步骤：移除 dave。
- 预期结果：dave 两个连接都在 1 秒内收到 `event {type:"convRemoved", convId:"group:g1"}`；alice 不收到该事件。

### TC-GRP-03-04 收到 convAdded 后会话列表出现该群
- 关联 AC：AC-GRP-03-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：dave 在会话列表页；g1 已有消息"hi"（最后一条）。
- 步骤：通过管理接口把 dave 加入 g1。
- 预期结果：1 秒内列表出现"活动群"，未读为 0（historyVisible=false），无需刷新页面。

### TC-GRP-03-05 收到 convRemoved 后打开该群显示"你已不在该群"，输入框禁用
- 关联 AC：AC-GRP-03-3
- 优先级：P0
- 层级：UI 自动化
- 前置条件：dave 是 g1 成员，在会话列表页；另一组：dave 正停留在 `/chat/group:g1` 页。
- 步骤：
  1. 移除 dave，然后 dave 点击会话列表中的 g1（若列表已移除该项，则直接访问 `/chat/group:g1`）。
  2. 另一组：停留在聊天页时移除 dave。
- 预期结果：两种情况下都显示"你已不在该群"，输入框和发送按钮禁用；不出现"加载失败，点击重试"等其他错误提示。

---

## 覆盖检查

| AC | 覆盖用例 |
|---|---|
| AC-GRP-01-1 | TC-GRP-01-01、TC-GRP-01-02 |
| AC-GRP-01-2 | TC-GRP-01-03 |
| AC-GRP-01-3 | TC-GRP-01-04、TC-GRP-01-05、TC-GRP-01-06、TC-GRP-01-07、TC-GRP-01-08 |
| AC-GRP-01-4 | TC-GRP-01-09 |
| AC-GRP-01-5 | TC-GRP-01-10、TC-GRP-01-11 |
| AC-GRP-01-6 | TC-GRP-01-12、TC-GRP-01-13、TC-GRP-01-14 |
| AC-GRP-01-7 | TC-GRP-01-15 |
| AC-GRP-01-8 | TC-GRP-01-16、TC-GRP-01-17、TC-GRP-01-18 |
| AC-GRP-01-9 | TC-GRP-01-19、TC-GRP-01-20 |
| AC-GRP-01-10 | TC-GRP-01-21、TC-GRP-01-22 |
| AC-GRP-01-11 | TC-GRP-01-23、TC-GRP-01-24、TC-GRP-01-25 |
| AC-GRP-02-1 | TC-GRP-02-01、TC-GRP-02-02、TC-GRP-02-03 |
| AC-GRP-02-2 | TC-GRP-02-04 |
| AC-GRP-02-3 | TC-GRP-02-05 |
| AC-GRP-02-4 | TC-GRP-02-06 |
| AC-GRP-02-5 | TC-GRP-02-07 |
| AC-GRP-03-1 | TC-GRP-03-01、TC-GRP-03-02 |
| AC-GRP-03-2 | TC-GRP-03-03 |
| AC-GRP-03-3 | TC-GRP-03-04、TC-GRP-03-05 |

共 19 条 AC，全部覆盖。

## 疑问

- Q1（批量上限与建群 2000 人冲突）：规则 2 说"单次批量最多 500 人"，但 AC-GRP-01-3 要求"建群时有 2000 个成员可以成功"。500 的上限是否只适用于加成员、不适用于建群？若适用，2000 人建群必然返回 4131；建群 2001 人时 4130 与 4131 哪个优先？
- Q2（错误优先级）：一次请求同时违反多条规则（如含不存在的 userId 且超出 2000 上限）时，返回 4042 还是 4130？
- Q3（historyVisible=true 新成员未读数）：AC-GRP-01-7 没写未读数。按公式 unread = maxSeq − max(readSeq, minSeq−1)，新成员 readSeq 若为 0，则入群即有 10 条未读；从 false 改为 true 后，老的新成员未读会从 0 突增到 10。是否预期？还是入群时把 readSeq 初始化为 joinSeq？
- Q4（建群是否推 convAdded）：F-GRP-03 只说"被加入群时"，建群时的在线初始成员是否收到 `convAdded`？（AC-CONN-01-7 的场景也依赖新群出现在会话列表。）
- Q5（重复加入 / 重复移除）：加一个已是成员的用户、移除一个不是成员的用户，是幂等成功还是报错？是否重新计算 joinSeq、是否再次推 `convAdded`？请求里同一个 userId 出现两次如何计数？
- Q6（被移除成员的其他命令）：被移除成员上报 `read`、或在会话列表中是否还保留该群（`convs` 是否还返回），PRD 未规定；TC-GRP-01-09 暂按 `read` 返回 4033 预期。AC-GRP-03-3 "打开该群显示你已不在该群"暗示本地可能保留该会话。
- Q7（群名计数）：群名"1～50 字"是否同样按 §3.1 去首尾空白、按码点计数？纯空白群名是否视为空？
