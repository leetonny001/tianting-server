# 测试用例：图片和文件消息（FILE，C2）

> 依据：[PRD-chat v1.0](../prd/PRD-chat-v1.0.md)（冻结）第 4、6 节、第 15 节 AC-FILE-90；[GATE-objstore](../review/GATE-objstore-2026-11-06.md)；[tools/objstore-contract](../../tools/objstore-contract/README.md)。任务单：[backend-chat](../tasks/backend-chat.md)、[console-chat](../tasks/console-chat.md)、[frontend-chat](../tasks/frontend-chat.md)。
> 范围：标了【C2】的 AC-FILE（CH-27）。依赖撤回、清空、举报的 AC 随 C4～C6。
>
> 通用约定：
> - 环境：**E1 单节点**（1 个 im-server，development，`IM_TEST_MODE=1`，SQLite 独立库；`IM_TRUSTED_PROXIES` 配成测试客户端，换 IP 用 `X-Forwarded-For`，PRD 第 4.5 节）；**E1 + 假 S3**（再加 `objstoretest` 独立进程，`IM_FILES_ENABLED=1`，公开桶和私有桶都在假 S3 上）；**E1 + console-server + tianting-admin**（`CON_TEST_MODE=1`，Postgres）；**真实 MinIO**（`deploy/testenv-ci.sh` 的固定版本）；**EP 性能**（目标机，P-12）；**Postgres**（B6 之后）；**模拟器**（Android 模拟器、iOS Simulator）；**真机**（P-13）。开发机不安装软件。
> - **CI 现在不运行（PENDING P-21）**，所有结果都是本地结果，记录时写明环境和提交号。性能用例在开发机的结果只记录，**不算通过**，发布判定等 P-12。真机用例在 P-13 到位前记未执行。
> - 层级：**后端 e2e**（Go e2e，对 `/v1/*`、`/ws`、`/admin/*` 断言）、**单测**、**SDK 单测**（三套 SDK，假 HTTP + `FakeTransport` + 假时钟，向量用例按 `protocol/vectors`）、**Playwright**（H5，Chromium + WebKit，连真实 im-server 测试模式）、**原生 UI 测试**（Compose UI Test、XCUITest，模拟器）、**端到端**（真实 im-server + 客户端）、**合同测试**（`tools/objstore-contract`）、**性能**、**手工**（留截图或录屏）。标了 Playwright 和原生 UI 测试的前端用例，H5、Android、iOS 都要跑，某端不适用的按 AC 的括号。
> - 时钟：**[IM 钟]** `POST /admin/test/clock`，从 Asia/Shanghai 当天 00:00 起算，避免跨日（R-CQ-41）；**[SDK 钟]** 各 SDK 的 `Clock` 注入；**[真实钟]** 依赖对象存储的真实时间。**预签名一律用真实时钟**，推进 [IM 钟] 不影响签名（第 4.4 节）。
> - 负向断言"假通道、接口无调用"用哨兵法（第 4.4 节）。每个用例使用独立数据库；用例之间用 `/admin/test/limits/reset {scope:"auth"}` 清计数。
> - 批次字段：C1、C2 按 AC 的标记；【全部】的 AC 只验证到 C2 为止的部分（PRD 第 15 节累计表）。
> - 记号：`F1`、`F2` 是 `fileId`；`M1`、`M2` 是内容。`u1` 是上传者，攻击序列里也是持 PUT 地址的恶意上传者。
> - 假 S3 的注入：不可用、删除失败、Copy 失败、HEAD 之后替换对象；暂停点（校验之后 Copy 之前、Copy 之后提交之前、公开桶 Copy 之后）；直接写对象、推进临时前缀生命周期（见 BE-404）。**假 S3 只验调度逻辑，不证明厂商行为**。

| 用例 | 标题 | 关联 AC | 层级 | 优先级 |
|---|---|---|---|---|
| TC-FILE-01-01 | 正常流程：png + 缩略图，最终键和临时键不同，临时键已删 | AC-FILE-01-1 | 后端 e2e | P0 |
| TC-FILE-01-02 | 实际大小和声明不符：uploadMismatch，对象被删，不扣额度 | AC-FILE-01-2 | 后端 e2e | P0 |
| TC-FILE-01-03 | 声明 png，内容是 HTML：415，对象被删 | AC-FILE-01-3 | 后端 e2e | P0 |
| TC-FILE-01-04 | 当天额度用完 429，次日恢复 | AC-FILE-01-4 | 后端 e2e | P0 |
| TC-FILE-01-05 | 对象存储不可用：complete 503，申请成功，文本不受影响 | AC-FILE-01-5 | 后端 e2e | P0 |
| TC-FILE-01-06 | 未登录、别人的 fileId | AC-FILE-01-6 | 后端 e2e | P0 |
| TC-FILE-01-07 | 文件名清洗：路径、Bidi、控制字符、全清空、超长 | AC-FILE-01-7 | 后端 e2e | P0 |
| TC-FILE-01-08 | 100 MiB 上传不经过 im-server | AC-FILE-01-8 | 后端 e2e | P0 |
| TC-FILE-01-09 | complete 后用原地址覆盖：接收方下载到的还是 complete 时的内容 | AC-FILE-01-9 | 后端 e2e | P0 |
| TC-FILE-01-10 | complete 幂等；原图成功缩略图失败后重试整体重来 | AC-FILE-01-10 | 后端 e2e | P0 |
| TC-FILE-01-11 | 放弃的上传不扣额度；频率第 61 次；未完成第 21 个 | AC-FILE-01-11 | 后端 e2e | P0 |
| TC-FILE-01-12 | 4292 的提示：标记失败，不自动重试 | AC-FILE-01-12 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-01-13 | 缩略图的三种错：没带、没传、是 HTML | AC-FILE-01-13 | 后端 e2e | P0 |
| TC-FILE-01-14 | 开关关闭：404、4000，已有文件消息可 sync | AC-FILE-01-14 | 后端 e2e | P0 |
| TC-FILE-01-15 | 校验和 Copy 之间替换对象：失败，最终键没有被替换的内容 | AC-FILE-01-15 | 后端 e2e | P0 |
| TC-FILE-01-16 | PUT 地址过期或取消后不占未完成名额 | AC-FILE-01-16 | 后端 e2e | P0 |
| TC-FILE-01-17 | uploadPendingLimit：提示，不重试；用户取消时发 DELETE | AC-FILE-01-17 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-01-18 | 额度复查：同日最多超额一个文件 | AC-FILE-01-18 | 后端 e2e | P0 |
| TC-FILE-01-19 | 两个并发 complete：租约挡住第二个，元数据和最终对象一致 | AC-FILE-01-19 | 后端 e2e | P0 |
| TC-FILE-01-20 | 只传原图得到 4096 后换内容：最终都来自第二次 | AC-FILE-01-20 | 后端 e2e | P0 |
| TC-FILE-01-21 | PUT 带恶意头：最终对象只有服务端写入的 Content-Type | AC-FILE-01-21 | 后端 e2e | P0 |
| TC-FILE-01-22 | 尺寸校验：缩略图、头像超限，解析不到尺寸 | AC-FILE-01-22 | 后端 e2e | P0 |
| TC-FILE-01-23 | DELETE 不暴露存在性，只取消本人 pending | AC-FILE-01-23 | 后端 e2e | P0 |
| TC-FILE-01-24 | complete 提交前被 file_purge 抢先：提交失败，随机键被删 | AC-FILE-01-24 | 后端 e2e | P0 |
| TC-FILE-01-25 | 文件名进 Content-Disposition 按 RFC 5987 编码 | AC-FILE-01-25 | 后端 e2e | P0 |
| TC-FILE-01-26 | 崩溃残留：重试成功后清除；不重试 25 小时后清除 | AC-FILE-01-26 | 后端 e2e | P0 |
| TC-FILE-01-27 | 租约释放、已 ready 重读、两个提交竞争 | AC-FILE-01-27 | 后端 e2e | P0 |
| TC-FILE-01-28 | 取消和 complete：取消后 404；并发只有一个生效 | AC-FILE-01-28 | 后端 e2e | P0 |
| TC-FILE-01-29 | 反复申请再取消：声明字节超过每日额度 4 倍后 429 | AC-FILE-01-29 | 后端 e2e | P0 |
| TC-FILE-02-01 | 发 jpeg：对方先看缩略图，点开看原图，两边一致 | AC-FILE-02-1 | 端到端 | P0 |
| TC-FILE-02-02 | 客户端伪造 size、mime 无效 | AC-FILE-02-2 | 后端 e2e | P0 |
| TC-FILE-02-03 | 五种不能发的 fileId，消息都没保存 | AC-FILE-02-3 | 后端 e2e | P0 |
| TC-FILE-02-04 | EXIF 和 GPS 被清除，方向正确 | AC-FILE-02-4 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-05 | 上传中取消：消息消失，不发 send | AC-FILE-02-5 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-06 | 预留占位：图片加载前后下一条消息不跳动 | AC-FILE-02-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-07 | 恶意文件名按纯文本显示 | AC-FILE-02-7 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-08 | 会话列表摘要"[图片]" | AC-FILE-02-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-09 | 懒加载：40 张图只请求视口内缩略图 | AC-FILE-02-9 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-10 | 真机：HEIC、实况、各格式矩阵 | AC-FILE-02-10 | 手工 | P0 |
| TC-FILE-02-11 | 真实宽高：伪造无效；头部超限 415 | AC-FILE-02-11 | 后端 e2e | P0 |
| TC-FILE-02-12 | 解码炸弹缩略图：显示占位，不崩溃 | AC-FILE-02-12 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-13 | 不能解码的文件失败；超 20 MiB 压缩后 3 MiB 成功 | AC-FILE-02-13 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-14 | GIF：原生显示首帧和标记，H5 播放 | AC-FILE-02-14 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-02-15 | body 按 type 互斥 | AC-FILE-02-15 | 后端 e2e | P0 |
| TC-FILE-02-16 | svg、bmp、tiff 申请时 415 | AC-FILE-02-16 | 后端 e2e | P0 |
| TC-FILE-03-01 | 群里发 pdf，成员下载 SHA-256 相同 | AC-FILE-03-1 | 端到端 | P0 |
| TC-FILE-03-02 | 超过大小限制 413，边界通过 | AC-FILE-03-2 | 后端 e2e | P0 |
| TC-FILE-03-03 | 后面藏 HTML 的 .bin：浏览器作为附件保存 | AC-FILE-03-3 | Playwright（H5） | P0 |
| TC-FILE-03-04 | 200 字符文件名：中间省略，不横向滚动 | AC-FILE-03-4 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-03-05 | 下载中断网：失败可重试，重试从头、重新申请地址 | AC-FILE-03-5 | 原生 UI 测试（Android、iOS） | P0 |
| TC-FILE-03-06 | 用"文件"方式选 jpeg：发送前提示 | AC-FILE-03-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-03-07 | H5 下载中文文件名 | AC-FILE-03-7 | Playwright（H5） | P0 |
| TC-FILE-03-08 | 文件摘要不含文件名，刷新后也是 | AC-FILE-03-8 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-03-09 | 打开白名单：pdf 能打开，.xyz 只有保存 | AC-FILE-03-9 | 原生 UI 测试（Android、iOS） | P0 |
| TC-FILE-03-10 | 生产 CSP：上传、预览、下载没有违规 | AC-FILE-03-10 | Playwright（H5） | P0 |
| TC-FILE-04-01 | 第三人申请地址：和不存在的 fileId 完全相同 | AC-FILE-04-1 | 后端 e2e | P0 |
| TC-FILE-04-02 | 移出群后不能再取 | AC-FILE-04-2 | 后端 e2e | P0 |
| TC-FILE-04-03 | historyVisible=false：入群前的不能取，改 true 后可以 | AC-FILE-04-3 | 后端 e2e | P0 |
| TC-FILE-04-04 | 预签名过期：真实钟 | AC-FILE-04-4 | 后端 e2e | P0 |
| TC-FILE-04-05 | 后台删除后不能取 | AC-FILE-04-5 | 后端 e2e | P0 |
| TC-FILE-04-06 | 无签名访问、列举桶都被拒绝 | AC-FILE-04-6 | 后端 e2e | P0 |
| TC-FILE-04-07 | 申请和下载不留预签名和文件名在日志 | AC-FILE-04-7 | 后端 e2e | P0 |
| TC-FILE-04-08 | 未绑定：上传者能取，别人不能 | AC-FILE-04-8 | 后端 e2e | P0 |
| TC-FILE-04-09 | 批量：50 个里 1 个无权；51 个 400 | AC-FILE-04-9 | 后端 e2e | P0 |
| TC-FILE-04-10 | 连续滚动不触发 4290；同一张图命中缓存 | AC-FILE-04-10 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-04-11 | 打开调试日志：搜不到签名 | AC-FILE-04-11 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-05-01 | 可执行文件：申请时或 complete 时 415 | AC-FILE-05-1 | 后端 e2e | P0 |
| TC-FILE-05-02 | 配置新增类型；先规范化再判断 | AC-FILE-05-2 | 后端 e2e | P0 |
| TC-FILE-05-03 | 下载前来源提示：每个会话首次 | AC-FILE-05-3 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-05-04 | 无法识别的类型：文件通过，图片拒绝 | AC-FILE-05-4 | 后端 e2e | P0 |
| TC-FILE-05-05 | png 后面藏 HTML：下载响应是 image/png | AC-FILE-05-5 | 后端 e2e | P0 |
| TC-FILE-05-06 | 文件表 scanState 默认 skipped | AC-FILE-05-6 | 后端 e2e | P0 |
| TC-FILE-06-01 | 未绑定和 pending 25 小时后被清除 | AC-FILE-06-1 | 后端 e2e | P0 |
| TC-FILE-06-02 | 绑定后 181 天：对象删除，消息保留 expired，地址 404 | AC-FILE-06-2 | 后端 e2e | P0 |
| TC-FILE-06-03 | 一个文件只绑一条消息；原 clientMsgId 重发得原 ack | AC-FILE-06-4 | 后端 e2e | P0 |
| TC-FILE-06-04 | 对象删除失败：记录保留，下次重试 | AC-FILE-06-5 | 后端 e2e | P0 |
| TC-FILE-06-05 | 过期消息的显示，不发申请地址 | AC-FILE-06-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-06-06 | purging 的文件不能绑定 | AC-FILE-06-7 | 后端 e2e | P0 |
| TC-FILE-06-07 | 同一文件两个 clientMsgId 并发 | AC-FILE-06-8 | 后端 e2e | P0 |
| TC-FILE-06-08 | 保留期 0 不过期 | AC-FILE-06-9 | 后端 e2e | P0 |
| TC-FILE-06-09 | 过期后文件名清除 | AC-FILE-06-10 | 后端 e2e | P0 |
| TC-FILE-06-10 | 退群、解散：退群时其他人照常；解散后 404；对象都还在 | AC-FILE-06-11 | 后端 e2e | P0 |
| TC-FILE-06-11 | 过期递增 modVer：只重拉这个会话，变"已过期" | AC-FILE-06-12 | SDK 单测（三套） | P0 |
| TC-FILE-07-01 | 上传中杀 App：显示失败，点重发从头传，只收到一条 | AC-FILE-07-1 | 原生 UI 测试（Android、iOS） | P0 |
| TC-FILE-07-02 | complete 成功但 send 超时：手动重发不再上传 | AC-FILE-07-2 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-07-03 | H5：上传失败后刷新丢失；不刷新能重发 | AC-FILE-07-3 | Playwright（H5） | P0 |
| TC-FILE-07-04 | 文字越过上传中的图片；3 张图按选择顺序 | AC-FILE-07-4 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-07-05 | 系统相册删了原图：沙盒副本能重发；成功后副本被删 | AC-FILE-07-5 | 原生 UI 测试（Android、iOS） | P0 |
| TC-FILE-07-06 | 断网选图：失败可重发，只收到一条 | AC-FILE-07-6 | Playwright（H5）+ 原生 UI 测试 | P0 |
| TC-FILE-07-07 | 弱网上传失败 5 次：不扣额度 | AC-FILE-07-7 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-07-08 | send 被拒后文件还是 ready，取消拉黑后可以绑定 | AC-FILE-07-8 | 后端 e2e | P0 |
| TC-FILE-07-09 | 退出登录清除沙盒副本 | AC-FILE-07-9 | 原生 UI 测试（Android、iOS） | P0 |
| TC-FILE-07-10 | 文件 24 小时被清理：原生重传，H5 提示 | AC-FILE-07-10 | Playwright（H5）+ 原生 UI 测试 + SDK 单测 | P0 |
| TC-FILE-90-01 | 真实 MinIO 合同测试 | AC-FILE-90-1 | 合同测试 | P0 |
| TC-FILE-90-02 | 云厂商合同测试 | AC-FILE-90-2 | 手工 | P0 |
| TC-FILE-91-01 | 元数据响应搜不到文件名（AC-ADM-30-7 的扩展，C2 起生效） | AC-ADM-30-7 | 后端 e2e | P0 |

---

## F-FILE-01 上传流程

### TC-FILE-01-01 正常流程：png + 缩略图，最终键和临时键不同，临时键已删
- 关联 AC：AC-FILE-01-1
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 已登录
- 步骤：
  1. `POST /v1/files/uploads {kind:"image", name:"a.png", size:1048576, mime:"image/png", thumb:{size:20480, mime:"image/jpeg"}}`
  2. 按响应 `put`、`thumbPut` PUT 原图和缩略图到假 S3
  3. `POST /v1/files/{fileId}/complete`
  4. 查 `files` 和假 S3 对象列表
- 预期结果：
  1. 第 1 步 URL 指向 `tmp/` 下的键，签入 `Content-Length`、`Content-Type`
  2. 第 3 步 200，`{fileId, size, mime, width, height}` 是服务端读到的真实值
  3. `state=ready`；`object_key` 形如 `final/{fileId}/{attemptId}/original`，不含文件名和 userId，和 PUT 的键不同；临时键已不存在

### TC-FILE-01-02 实际大小和声明不符：uploadMismatch，对象被删，不扣额度
- 关联 AC：AC-FILE-01-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：声明 `size=1 MiB`。**PUT 地址签入了 Content-Length，真实存储和假 S3 都会拒绝 2 MiB 的 PUT**（GATE-objstore：HTTP 403），走不到 `complete`，所以用假 S3 的测试接口把 2 MiB 对象直接写到临时键（见文件末尾的说明）
- 步骤：
  1. 申请上传；调假 S3 测试接口写入 2 MiB 到临时键
  2. `complete`；查假 S3 和当日额度
  3. 对照：用预签名 PUT 真的发 2 MiB
- 预期结果：
  1. `complete` 400 / 4000，`reason=uploadMismatch`；临时对象被删；`upload_quota` 不变
  2. 第 3 步被假 S3 拒绝（403），之后 `complete` 4096

### TC-FILE-01-03 声明 png，内容是 HTML：415，对象被删
- 关联 AC：AC-FILE-01-3
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 声明 `kind=image, mime=image/png`，缩略图合法，原图内容 `<html>…`（长度和声明一致）
  2. `complete`；查假 S3
- 预期结果：
  1. 415 / 4150；原图临时对象被删；`state` 仍是 `pending`

### TC-FILE-01-04 当天额度用完 429，次日恢复
- 关联 AC：AC-FILE-01-4
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟] 从 Asia/Shanghai 当天 00:00 起算）
- 前置条件：把 u1 当日 `upload_quota` 设为 2 GiB（`seed`）
- 步骤：
  1. 申请 `kind=file, size=1`
  2. 把 [IM 钟] 推进到次日 00:00:01，再申请
- 预期结果：
  1. 第 1 步 429 / 4292；第 2 步成功

### TC-FILE-01-05 对象存储不可用：complete 503，申请成功，文本不受影响
- 关联 AC：AC-FILE-01-5
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1、u2 在线；u1 已 PUT 完一个文件；假 S3 注入不可用（拒绝连接，再换黑洞各跑一次）
- 步骤：
  1. `complete`
  2. 申请一个新上传
  3. u1 给 u2 发文本
- 预期结果：
  1. `complete` 503 / 5032；申请 200；文本收发正常，ack 正常

### TC-FILE-01-06 未登录、别人的 fileId
- 关联 AC：AC-FILE-01-6
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 有 `pending` 的 F1
- 步骤：
  1. 不带 Authorization 调 `complete` F1
  2. u2 带 token 调 `complete` F1；u2 调 `complete` 不存在的 fileId
- 预期结果：
  1. 第 1 步 401 / 4010
  2. 第 2 步都是 404 / 4048，响应相同

### TC-FILE-01-07 文件名清洗：路径、Bidi、控制字符、全清空、超长
- 关联 AC：AC-FILE-01-7
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 依次申请 `kind=file`，name：`../../etc/pass‮wd.`、`a\nb`、全是控制字符、清洗后 256 字节（中文 86 个 = 258 字节）
  2. 边界：清洗后正好 255 字节
  3. 查 `files.name`
- 预期结果：
  1. 前三个成功，依次存 `etcpasswd`、`ab`、`file`
  2. 第四个 400 / 4000，`reason=fileNameTooLong`；255 字节成功

### TC-FILE-01-08 100 MiB 上传不经过 im-server
- 关联 AC：AC-FILE-01-8
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（假 S3 是独立进程）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. sdk-ts 上传 100 MiB 文件，抓 SDK PUT 的目标主机
  2. 读假 S3 收到的字节数；读 im-server 对外端口每个请求的请求体长度
- 预期结果：
  1. 目标是假 S3；假 S3 收到全部字节
  2. im-server 每个请求体 ≤ 64 KB

### TC-FILE-01-09 complete 后用原地址覆盖：接收方下载到的还是 complete 时的内容
- 关联 AC：AC-FILE-01-9
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 上传 `kind=file` 内容 M1（文件 F1），已 `complete`，发给 u2；同样大小的恶意内容 M2
- 步骤：
  1. 攻击者 u1：用原 PUT 地址（还没过期）PUT M2 到临时键（大小相同才能过签名）
  2. u2：`POST /v1/files/urls`，下载，算 SHA-256
  3. 在假 S3 上触发"临时前缀 1 天生命周期"（测试接口推进），查临时前缀
- 预期结果：
  1. 第 1 步假 S3 接受（临时键已删除后可以重新写入）
  2. 第 2 步下载到的是 M1
  3. 第 3 步 M2 被清除。真实清除周期见 TC-FILE-90-01

### TC-FILE-01-10 complete 幂等；原图成功缩略图失败后重试整体重来
- 关联 AC：AC-FILE-01-10
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 上传图片 F1；假 S3 注入：缩略图 Copy 失败一次
- 步骤：
  1. `complete` F1（原图 Copy 成功，缩略图失败）
  2. 查假 S3 `final/F1/`
  3. 再 `complete`
  4. 再 `complete`（顺序重复）
  5. 查额度和 `final/F1/`
- 预期结果：
  1. 第 1 步 503 / 5032，临时键都还在；第 2 步第一次尝试的键已删除或只属于该尝试
  2. 第 3 步成功，原图和缩略图都是新的 `attemptId`；全部成功后临时键才删除
  3. 第 4 步返回同一条记录；额度只扣 1 次，`final/F1/` 只有一个 `attemptId`

### TC-FILE-01-11 放弃的上传不扣额度；频率第 61 次；未完成第 21 个
- 关联 AC：AC-FILE-01-11
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[真实钟]）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 申请 3 个上传不 PUT；查额度
  2. 脚本：同一分钟内申请并立刻 `DELETE`（释放名额），到第 61 次
  3. 等 1 分钟后，连续申请 21 个不 `DELETE`（都在 15 分钟内）
- 预期结果：
  1. 第 1 步额度不变
  2. 第 2 步第 61 次 429 / 4290（没有 `uploadPendingLimit`）
  3. 第 3 步第 21 个 429 / 4290，`reason=uploadPendingLimit`
- 说明：不取消就先撞上未完成数 20，第 61 次到不了，所以第 2 步要边申请边取消

### TC-FILE-01-12 4292 的提示：标记失败，不自动重试
- 关联 AC：AC-FILE-01-12
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：假接口让申请上传返回 429 / 4292
- 步骤：
  1. 发一张图；等 5 秒
- 预期结果：
  1. 消息标记失败，提示"今天的上传额度已用完"；只有 1 个申请请求

### TC-FILE-01-13 缩略图的三种错：没带、没传、是 HTML
- 关联 AC：AC-FILE-01-13
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 申请 `kind=image` 不带 `thumb`
  2. 申请合法，只 PUT 原图，`complete`
  3. 再申请一个合法的：PUT 原图和 HTML 内容的缩略图，`complete`
- 预期结果：
  1. 依次 400 / 4000、409 / 4096（`state` 不变）、415 / 4150

### TC-FILE-01-14 开关关闭：404、4000，已有文件消息可 sync
- 关联 AC：AC-FILE-01-14
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：开关开时 u1 给 u2 发了一张图；然后 `IM_FILES_ENABLED=0` 重启
- 步骤：
  1. `POST /v1/files/uploads`
  2. `send {type:"image", body:{image:{fileId}}}`
  3. u2 `sync`
  4. `GET /v1/config`
- 预期结果：
  1. 404；400 / 4000；`sync` 返回这条图片消息
  2. `features.files=false`

### TC-FILE-01-15 校验和 Copy 之间替换对象：失败，最终键没有被替换的内容
- 关联 AC：AC-FILE-01-15
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：假 S3 可注入"HEAD 之后替换对象"；u1 `kind=file` 内容 M1；替换内容 M2（大小相同）
- 步骤：
  1. 注入 A：HEAD 之后、嗅探之前替换为 M2；`complete`
  2. 查假 S3 请求记录、`final/`、临时键
  3. 注入 B（新的上传）：嗅探之后、Copy 之前替换为 M2；`complete`
  4. 同样查一遍
- 预期结果：
  1. A、B 都是：带 `If-Match` 的读或条件 Copy 得到 412，内部重试 1 次
  2. 持续替换时最终 400 / 4000，`reason=uploadMismatch`；`final/` 没有 M2；临时对象没有被删

### TC-FILE-01-16 PUT 地址过期或取消后不占未完成名额
- 关联 AC：AC-FILE-01-16
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[真实钟]，测试配置把 PUT 地址 TTL 调到 5 秒）
- 前置条件：u1 已有 20 个未完成上传
- 步骤：
  1. 申请第 21 个（确认 429）
  2. 等 6 秒（PUT 地址全部过期），再申请
  3. 另一用户同样 20 个，`DELETE` 其中 1 个，再申请
- 预期结果：
  1. 第 1 步 4290；第 2、3 步都成功
- 说明：PRD 没有给 PUT 有效期命名配置项，实现时要加一个（见文件末尾的说明）

### TC-FILE-01-17 uploadPendingLimit：提示，不重试；用户取消时发 DELETE
- 关联 AC：AC-FILE-01-17
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：假接口让申请返回 429 / 4290，`reason=uploadPendingLimit`
- 步骤：
  1. 发文件；等 5 秒
  2. 恢复接口，发文件，上传中点取消
- 预期结果：
  1. 第 1 步标记失败，提示"上传任务太多，请稍后再试"，只有 1 个请求
  2. 第 3 步发 `DELETE /v1/files/uploads/{id}`

### TC-FILE-01-18 额度复查：同日最多超额一个文件
- 关联 AC：AC-FILE-01-18
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]，测试配置 `IM_UPLOAD_DAILY_DECLARED` 调大）
- 前置条件：场景一：u1 剩余额度 1 MiB，3 个已申请的 1 MiB 上传，都已 PUT；场景二：u2 当日用量 0。[真实钟] 下每轮申请 20 个 `size=100 MiB` 并 PUT（对象用假 S3 测试接口写入，只记长度不占磁盘），每轮后推进 [IM 钟] 15 分钟，共 5 轮 100 个。**默认的声明上限 8 GiB 在第 5 轮就会 4292**（`IM_UPLOAD_DAILY_DECLARED`，RS5-16），未完成上传超过 24 小时又会被清理，所以场景二把声明上限调大、在 24 小时之前 `complete`，只验证提交时的复查（见文件末尾的说明）
- 步骤：
  1. 场景一：依次 `complete` 3 个
  2. 场景二：[IM 钟] 推进到首轮申请后 23 小时 50 分（清理之前），依次 `complete` 全部
  3. 查两个账号的 `upload_quota`
- 预期结果：
  1. 场景一：第 1 个成功，当日用量到达额度，后两个 429 / 4292，临时对象留给生命周期清除
  2. 场景二：前 21 个成功（第 21 个之前已用不足 2 GiB），之后都 429 / 4292
  3. 超额不超过一个文件（100 MiB）

### TC-FILE-01-19 两个并发 complete：租约挡住第二个，元数据和最终对象一致
- 关联 AC：AC-FILE-01-19
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 `kind=file` 内容 M1；暂停点停在"第一个 complete 校验通过、Copy 之前"；攻击者 u1 持原 PUT 地址
- 步骤：
  1. Q1：`complete`，到暂停点
  2. Q2：`complete`
  3. 攻击者：PUT M2（大小相同）到临时键
  4. 放行 Q1
  5. 查 `files`、最终对象；下载算 SHA-256
- 预期结果：
  1. Q2 503 / 5032，`reason=completeInProgress`
  2. Q1 条件 Copy 412，内部重试后：要么用新 ETag 重新校验并写入（记录和最终对象都是 M2）要么 `uploadMismatch`
  3. 记录的 `size`、`mime` 和最终对象一致；没有 `ready` 但对象缺失的状态

### TC-FILE-01-20 只传原图得到 4096 后换内容：最终都来自第二次
- 关联 AC：AC-FILE-01-20
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 图片上传：只 PUT 原图 P1（jpeg，800×600）
- 步骤：
  1. `complete`，得到 4096
  2. 用原地址替换临时原图为 P2（png，400×300，大小相同；`Content-Type` 签名限定时另申请一次）
  3. PUT 缩略图；`complete`
  4. 下载原图；查响应
- 预期结果：
  1. 最终内容是 P2；`mime`、`width`、`height` 都来自第二次的读取；和第一次无关

### TC-FILE-01-21 PUT 带恶意头：最终对象只有服务端写入的 Content-Type
- 关联 AC：AC-FILE-01-21
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES；MinIO 复跑
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 用预签名 PUT 分别带 `Content-Type: text/html`（与签名不同）、`Content-Encoding: gzip`、`Content-Disposition: inline`、`x-amz-meta-evil: 1`、`x-amz-acl: public-read`、`x-amz-website-redirect-location: /x`、`x-amz-tagging: a=b`
  2. PUT 成功的继续 `complete`；HEAD 最终对象；取标签；对头像再 `PATCH profile` 后匿名 GET 公开地址
- 预期结果：
  1. PUT 要么被拒绝要么头被丢弃（`Content-Type` 不符的被拒绝）
  2. 最终对象和公开响应：只有服务端写入的 `Content-Type`，没有 `Content-Encoding`、用户元数据、标签、重定向

### TC-FILE-01-22 尺寸校验：缩略图、头像超限，解析不到尺寸
- 关联 AC：AC-FILE-01-22
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 图片消息：缩略图头部 6000×6000，`complete`
  2. `kind=avatar`：头部 6000×6000，`complete`
  3. `kind=image`：头部解析不到尺寸的 JPEG（截断 SOF），`complete`
  4. 边界：缩略图长边 512，头像长边 1024
- 预期结果：
  1. 前三个 415 / 4150；边界成功

### TC-FILE-01-23 DELETE 不暴露存在性，只取消本人 pending
- 关联 AC：AC-FILE-01-23
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 有 `ready` 的 F1、`attached` 的 F2；u2 有 `pending` 的 F3
- 步骤：
  1. u1：`DELETE /v1/files/uploads/F1`；`F2`；`F3`；不存在的 id
  2. 查三个文件的状态和对象；u2 `complete` F3
- 预期结果：
  1. 4 次响应完全相同
  2. F1、F2、F3 状态和对象不变；u2 的 `complete` 正常

### TC-FILE-01-24 complete 提交前被 file_purge 抢先：提交失败，随机键被删
- 关联 AC：AC-FILE-01-24
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：u1 `pending` 的 F1 已 PUT；暂停点"Copy 之后、提交之前"；[IM 钟] 冻结
- 步骤：
  1. Q1：`complete` F1，到暂停点
  2. [IM 钟] 推进 25 小时（租约已过期），运行 `file_purge`，F1 进入 `purging`
  3. 放行 Q1
  4. 查 `final/F1/`、`files`
- 预期结果：
  1. `complete` 提交影响 0 行，失败（404 / 4048）；它写的随机键被删除
  2. 下次 `file_purge` 后 `final/F1/` 为空，`files` 行被删，没有永久孤儿

### TC-FILE-01-25 文件名进 Content-Disposition 按 RFC 5987 编码
- 关联 AC：AC-FILE-01-25
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 上传并发出文件，文件名 `evil.exe"; x=".txt`（申请时被清洗但判断扩展名是 `.txt`）
- 步骤：
  1. 接收方 `/v1/files/urls`；解码 URL 的 `response-content-disposition`
  2. Playwright 下载，查保存的文件名
- 预期结果：
  1. 带 `filename*=UTF-8''…` 百分号编码；ASCII 回退名不含 `"` `;` `%`
  2. H5 保存的文件名扩展名不是 `.exe`

### TC-FILE-01-26 崩溃残留：重试成功后清除；不重试 25 小时后清除
- 关联 AC：AC-FILE-01-26
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：暂停点"Copy 之后、提交之前"；F1、F2 都已 PUT
- 步骤：
  1. `complete` F1，到暂停点 `kill -9` im-server；重启；[IM 钟] 推进超过租约
  2. 再次 `complete` F1；下载；列 `final/F1/`
  3. `complete` F2，到暂停点 `kill -9`；重启，不再重试；[IM 钟] 推进 25 小时，运行 `file_purge`
  4. 查 F2 的 `final/F2/` 和 `files`
- 预期结果：
  1. F1 重试成功；提交后残留的旧 `attemptId` 被删除；下载到的是这一次的内容
  2. F2 对象和记录都被删除

### TC-FILE-01-27 租约释放、已 ready 重读、两个提交竞争
- 关联 AC：AC-FILE-01-27
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 场景一：图片缺缩略图 `complete` 得到 4096；立即 PUT 缩略图；立即 `complete`
  2. 场景二：Q1 `complete` 停在 Copy 之后；[IM 钟] 推进 61 秒（租约过期）；Q2 `complete` 成功并删除临时键；放行 Q1
  3. 场景三：暂停两个尝试都在 Copy 之后（第一个过期后第二个进入），同时放行提交
- 预期结果：
  1. 场景一第二次 `complete` 立即执行，不是 503
  2. 场景二 Q1 的 HEAD 得到 404，重读状态后返回同一条记录，不是 4096
  3. 场景三恰好一个提交成功，败者只删除自己的 `attemptId` 键；`final/` 里留下的和记录一致

### TC-FILE-01-28 取消和 complete：取消后 404；并发只有一个生效
- 关联 AC：AC-FILE-01-28
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `DELETE` F1 后 `complete` F1
  2. F2：`DELETE` 和 `complete` 同时发，重复 20 轮
- 预期结果：
  1. 第 1 步 404 / 4048
  2. 第 2 步每轮：要么 `ready` 且对象存在，要么 `purging`、`complete` 失败且随机键被删；状态和对象一致

### TC-FILE-01-29 反复申请再取消：声明字节超过每日额度 4 倍后 429
- 关联 AC：AC-FILE-01-29
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：`IM_UPLOAD_DAILY_BYTES=2 GiB`，声明上限 8 GiB
- 步骤：
  1. 循环申请 `size=100 MiB` 再 `DELETE` 81 次（累计 8100 MiB，上限 8192 MiB）
  2. 第 82 次申请
  3. 次日 00:00:01（[IM 钟]）再申请
- 预期结果：
  1. 前 81 次成功；第 82 次 429 / 4292；次日成功

## F-FILE-02 图片消息

### TC-FILE-02-01 发 jpeg：对方先看缩略图，点开看原图，两边一致
- 关联 AC：AC-FILE-02-1
- 优先级：P0
- 批次：C2
- 层级：端到端
- 环境：E1 + 假 S3
- 前置条件：u1、u2 好友；三套 SDK 和三端（H5、Android、iOS）各发一次、各收一次
- 步骤：
  1. u1 发 2 MiB 的 jpeg
  2. u2 看消息；点开
  3. 比较两边 `body.image`
- 预期结果：
  1. u2 收到 `type=image`，先显示缩略图（网络记录先有 `variant=thumb`），点开后加载 `variant=original`
  2. `size`、`mime`、`width`、`height` 两边一致

### TC-FILE-02-02 客户端伪造 size、mime 无效
- 关联 AC：AC-FILE-02-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 `ready` 的图片 F1（真实 `size=102400`、`image/jpeg`）
- 步骤：
  1. `send {type:"image", body:{image:{fileId:F1, size:1, mime:"text/html"}}}`
  2. `sync`
- 预期结果：
  1. `send` 成功，伪造字段被忽略
  2. `sync` 下发 `size=102400`、`mime=image/jpeg`
- 说明：上行 `image` 按第 11.1 节只有 `fileId`，AC 要求伪造字段不报错，所以 Schema 不能在 `image` 内部用 `additionalProperties:false`（见文件末尾的说明）

### TC-FILE-02-03 五种不能发的 fileId，消息都没保存
- 关联 AC：AC-FILE-02-3
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u2 的 `ready` 图片 F-a；u1 的 `kind=file` 文件 F-b；u1 `pending` 的 F-c；u1 已绑定到另一条消息的 F-d；u1 只传了原图 `complete` 没通过的 F-e
- 步骤：
  1. u1 分别发图片，`fileId` 依次用 F-a～F-e
  2. u2 `sync`
- 预期结果：
  1. 依次 404 / 4048、400 / 4000（`reason=fileKindMismatch`）、409 / 4096、4096、4096
  2. 没有新消息

### TC-FILE-02-04 EXIF 和 GPS 被清除，方向正确
- 关联 AC：AC-FILE-02-4
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：ES（MinIO 复跑）
- 前置条件：夹具 jpeg 带 GPS、相机型号、Orientation=6
- 步骤：
  1. 三端分别发送；从对象存储下载原图和缩略图
  2. 用 exiftool（测试机）读两个文件；比较显示方向和夹具旋转后的参考图
- 预期结果：
  1. 读不到任何 EXIF 字段；显示方向和原图一致

### TC-FILE-02-05 上传中取消：消息消失，不发 send
- 关联 AC：AC-FILE-02-5
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：用慢速代理让 PUT 进行到一半
- 步骤：
  1. 点取消
- 预期结果：
  1. 消息从列表消失；没有 `send`；发了 `DELETE /v1/files/uploads/{id}`

### TC-FILE-02-06 预留占位：图片加载前后下一条消息不跳动
- 关联 AC：AC-FILE-02-6
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：4000×3000 图片消息后面有一条文本消息；用代理让图片延迟 2 秒
- 步骤：
  1. 打开聊天页，记录下一条消息的 `offsetTop`（原生用控件坐标）
  2. 图片加载完再记录一次
- 预期结果：
  1. 变化 ≤ 1 px

### TC-FILE-02-07 恶意文件名按纯文本显示
- 关联 AC：AC-FILE-02-7
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：文件消息，文件名 `<img src=x onerror=alert(1)>`
- 步骤：
  1. 三端打开聊天页
- 预期结果：
  1. 气泡显示原文字符；没有弹窗、没有 `img` 元素；H5 没有 CSP 违规

### TC-FILE-02-08 会话列表摘要"[图片]"
- 关联 AC：AC-FILE-02-8
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 最后一条是图片消息，打开会话列表
- 预期结果：
  1. 摘要"[图片]"；`lastMsg.text` 为空

### TC-FILE-02-09 懒加载：40 张图只请求视口内缩略图
- 关联 AC：AC-FILE-02-9
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：会话有 40 张图片的历史
- 步骤：
  1. 打开聊天页，记录 `/v1/files/urls` 和下载请求
- 预期结果：
  1. 只请求视口内的 `variant=thumb`，没有 `original`；滚动后才取下一批

### TC-FILE-02-10 真机：HEIC、实况、各格式矩阵
- 关联 AC：AC-FILE-02-10
- 优先级：P0
- 批次：C2
- 层级：手工
- 环境：真机（P-13）
- 前置条件：**真机依赖 P-13，未到位前记未执行**。Android（API 28+ 和 API 26 各一台，有就测）、iPhone 相册有 HEIC、实况照片；H5 用 Chrome、Safari
- 步骤：
  1. Android、iOS 从相册选 HEIC、实况照片发送
  2. H5 拖入 jpeg、png、gif、HEIC 发送
  3. 三端查看对方收到的结果；留截图
- 预期结果：
  1. 按规则 2 的矩阵：HEIC 在原生端转 jpeg；能发送成功的三端都能显示
  2. H5 解不了的 HEIC 提示"请用文件方式发送"；Android 低版本解不了时同样提示

### TC-FILE-02-11 真实宽高：伪造无效；头部超限 415
- 关联 AC：AC-FILE-02-11
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. u1 `complete` 800×600 的 png；`send` 带 `width=1`；`sync`
  2. png 头部写 20000×20000，`complete`
  3. 边界：头部 10000×4000（像素 4000 万）；10000×4001
- 预期结果：
  1. 下发 `width=800`；第 2 步 415 / 4150
  2. 边界：前者成功，后者 415 / 4150

### TC-FILE-02-12 解码炸弹缩略图：显示占位，不崩溃
- 关联 AC：AC-FILE-02-12
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：测试替身让缩略图下载内容头部是 20000×20000
- 步骤：
  1. 接收方打开聊天页；查内存（原生）和其他消息
- 预期结果：
  1. 显示占位；不崩溃；其他消息正常；没有解码调用（替身断言）

### TC-FILE-02-13 不能解码的文件失败；超 20 MiB 压缩后 3 MiB 成功
- 关联 AC：AC-FILE-02-13
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 选伪装成 jpg 的文本文件发图片
  2. 选 25 MiB 的照片（重编码后约 3 MiB）发图片
- 预期结果：
  1. 第 1 步标记失败，提示"无法处理这张图片，请用文件方式发送"
  2. 第 2 步成功；申请的 `size` 是压缩后的值

### TC-FILE-02-14 GIF：原生显示首帧和标记，H5 播放
- 关联 AC：AC-FILE-02-14
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. u1 发 gif（文件字节不变）；Android、iOS、H5 查看
- 预期结果：
  1. Android、iOS 首帧 + "GIF"标记；H5 播放；下载内容和原文件 SHA-256 相同

### TC-FILE-02-15 body 按 type 互斥
- 关联 AC：AC-FILE-02-15
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `send {type:"image", body:{image:{fileId}, text:"x"}}`
  2. `send {type:"image", body:{image:{fileId}, data:{}}}`
  3. `send {type:"text", body:{text:"x", data:{}}}`
  4. `send {type:"text", body:{text:"x", image:{fileId}}}`
  5. `send {type:"text", body:{text:"x"}}`
- 预期结果：
  1. 前 4 个 400 / 4000，消息未保存；第 5 个成功

### TC-FILE-02-16 svg、bmp、tiff 申请时 415
- 关联 AC：AC-FILE-02-16
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `kind=image` 分别申请 `image/svg+xml`、`image/bmp`、`image/tiff`
  2. jpeg、png、webp、gif 对照
- 预期结果：
  1. 前三个 415 / 4150；对照都成功

## F-FILE-03 文件消息

### TC-FILE-03-01 群里发 pdf，成员下载 SHA-256 相同
- 关联 AC：AC-FILE-03-1
- 优先级：P0
- 批次：C2
- 层级：端到端
- 环境：E1 + 假 S3
- 前置条件：群 g1 有 u1、u2；三套 SDK 和三端各发一次、各收一次
- 步骤：
  1. u1 发 5 MiB 的 pdf
  2. u2 查看气泡，下载
  3. 算下载文件的 SHA-256
- 预期结果：
  1. 气泡显示文件名和大小；SHA-256 和原文件相同

### TC-FILE-03-02 超过大小限制 413，边界通过
- 关联 AC：AC-FILE-03-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `kind=file, size=101 MiB`；`kind=image, size=21 MiB`
  2. `kind=file, size=100 MiB`；`kind=image, size=20 MiB`
- 预期结果：
  1. 前两个 413 / 4139，带 `details.maxBytes`；边界成功

### TC-FILE-03-03 后面藏 HTML 的 .bin：浏览器作为附件保存
- 关联 AC：AC-FILE-03-3
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）
- 环境：E1 + 假 S3
- 前置条件：文件 `.bin`，前 64 KB 二进制，之后是 `<script>`；声明 `application/octet-stream`
- 步骤：
  1. 上传并发出；接收方在 H5 点下载
  2. 抓下载响应头
- 预期结果：
  1. `Content-Disposition: attachment…`，`Content-Type: application/octet-stream`；浏览器保存，页面没有渲染

### TC-FILE-03-04 200 字符文件名：中间省略，不横向滚动
- 关联 AC：AC-FILE-03-4
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 发文件名 200 个字符的文件；375 宽查看
- 预期结果：
  1. 文件名中间省略，扩展名可见；没有横向滚动

### TC-FILE-03-05 下载中断网：失败可重试，重试从头、重新申请地址
- 关联 AC：AC-FILE-03-5
- 优先级：P0
- 批次：C2
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 假 S3 + 模拟器
- 前置条件：用代理限速
- 步骤：
  1. 下载到一半断网；观察
  2. 恢复网络，点"重试"
- 预期结果：
  1. 显示失败和"重试"
  2. 重试发了新的 `/v1/files/urls`；从 0 字节开始下载；SHA-256 正确

### TC-FILE-03-06 用"文件"方式选 jpeg：发送前提示
- 关联 AC：AC-FILE-03-6
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. "+ → 文件"，选 jpeg
  2. 取消一次；再选，确认
- 预期结果：
  1. 出现"文件会保留原始信息（可能包含位置）"；取消不发送，确认才发送

### TC-FILE-03-07 H5 下载中文文件名
- 关联 AC：AC-FILE-03-7
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 点下载名为"合同.pdf"的文件
- 预期结果：
  1. 浏览器开始下载，保存为"合同.pdf"；页面不显示进度

### TC-FILE-03-08 文件摘要不含文件名，刷新后也是
- 关联 AC：AC-FILE-03-8
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 最后一条是文件消息，文件名含特征串；H5 刷新页面；原生杀掉重开
- 预期结果：
  1. 摘要"[文件]"；页面搜不到特征串

### TC-FILE-03-09 打开白名单：pdf 能打开，.xyz 只有保存
- 关联 AC：AC-FILE-03-9
- 优先级：P0
- 批次：C2
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 假 S3 + 模拟器
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 分别下载 `a.pdf`、`a.xyz`；查看操作
- 预期结果：
  1. pdf 有"打开""保存"；`.xyz` 只有"保存"

### TC-FILE-03-10 生产 CSP：上传、预览、下载没有违规
- 关联 AC：AC-FILE-03-10
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）
- 环境：E1 + 假 S3
- 前置条件：生产 nginx 配置（`deploy/nginx.conf`），`connect-src` 只加存储域名，`img-src` 加 `blob:`
- 步骤：
  1. Playwright 上传、预览、下载，监听 `securitypolicyviolation` 和控制台
  2. 读响应头
- 预期结果：
  1. 没有 CSP 违规；`connect-src` 不含 `https:` 通配

## F-FILE-04 访问控制

### TC-FILE-04-01 第三人申请地址：和不存在的 fileId 完全相同
- 关联 AC：AC-FILE-04-1
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1、u2 单聊里的图片 F1
- 步骤：
  1. u3：`POST /v1/files/urls {items:[{fileId:F1, variant:"original"}]}`
  2. u3：同样请求，`fileId` 是不存在的值
- 预期结果：
  1. `items[0].code=4048`，没有 `url`；两次响应除 `fileId` 外相同（AC 的 404 在批量接口里是逐项 `code`，见文件末尾的说明）

### TC-FILE-04-02 移出群后不能再取
- 关联 AC：AC-FILE-04-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：群 g1 的文件 F1；成员 m
- 步骤：
  1. m 申请地址
  2. 把 m 移出 g1；m 再申请
- 预期结果：
  1. 第 1 步有 `url`；第 2 步 `code=4048`

### TC-FILE-04-03 historyVisible=false：入群前的不能取，改 true 后可以
- 关联 AC：AC-FILE-04-3
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：g1 `historyVisible=false`；F1 发出后 D 入群
- 步骤：
  1. D 申请 F1
  2. `groupUpdate historyVisible=true`；D 再申请
- 预期结果：
  1. 第 1 步 `code=4048`；第 2 步有 `url`

### TC-FILE-04-04 预签名过期：真实钟
- 关联 AC：AC-FILE-04-4
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[真实钟]；MinIO 复跑）
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `IM_FILE_URL_TTL_IMAGE=5` 启动；取图片地址，等 6 秒 GET
  2. 默认配置启动；取图片、文件地址，读 `expiresAt`；推进 [IM 钟] 不影响签名
- 预期结果：
  1. 第 1 步被存储拒绝（403）
  2. 第 2 步图片 `expiresAt` = 现在 + 120 秒，文件 = 现在 + 300 秒（误差 ≤ 2 秒）

### TC-FILE-04-05 后台删除后不能取
- 关联 AC：AC-FILE-04-5
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（`IM_V13_ADMIN_FEATURES=1`）
- 前置条件：u1、u2 单聊的图片消息 M1（F1）
- 步骤：
  1. u2 申请 F1，成功
  2. `POST /admin/messages/delete` 删除 M1
  3. u2 申请 F1
- 预期结果：
  1. 第 3 步 `code=4048`；`files.state=revoked`，`files.name` 为空；对象还在

### TC-FILE-04-06 无签名访问、列举桶都被拒绝
- 关联 AC：AC-FILE-04-6
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES；MinIO 复跑
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 不带 Authorization 和签名 GET 最终键
  2. 匿名 `GET 桶/?list-type=2`；用预签名地址改路径列举
- 预期结果：
  1. 都被存储拒绝（403）

### TC-FILE-04-07 申请和下载不留预签名和文件名在日志
- 关联 AC：AC-FILE-04-7
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：文件名特征串 `ZXQ-FILE-9`
- 步骤：
  1. 上传、发送、取地址、下载
  2. 扫描 im-server 日志
- 预期结果：
  1. 搜不到 `X-Amz-Signature`、`X-Amz-Credential`、`ZXQ-FILE-9`

### TC-FILE-04-08 未绑定：上传者能取，别人不能
- 关联 AC：AC-FILE-04-8
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 `ready` 未绑定的 F1
- 步骤：
  1. u1 申请；u2 申请
- 预期结果：
  1. u1 有 `url`；u2 `code=4048`

### TC-FILE-04-09 批量：50 个里 1 个无权；51 个 400
- 关联 AC：AC-FILE-04-9
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 一批 50 个：49 个有权、1 个别人的
  2. 一批 51 个
- 预期结果：
  1. HTTP 200，49 个有地址，1 个 `code=4048`
  2. 第 2 步 400 / 4131

### TC-FILE-04-10 连续滚动不触发 4290；同一张图命中缓存
- 关联 AC：AC-FILE-04-10
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：200 张图片的会话
- 步骤：
  1. 连续滚动到底，再滚回，同一张图第二次进入视口；记录 `/v1/files/urls` 批次和下载请求
- 预期结果：
  1. 没有 4290；一屏合并成一批
  2. 滚回时命中缓存（键 `fileId + variant`），不再下载

### TC-FILE-04-11 打开调试日志：搜不到签名
- 关联 AC：AC-FILE-04-11
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：三端打开 SDK、OkHttp、URLSession 调试日志
- 步骤：
  1. 上传、下载一次；抓全部日志
- 预期结果：
  1. 搜不到 `X-Amz-Signature`、预签名查询串、文件名特征串；本地库、缓存文件搜不到预签名地址

## F-FILE-05 类型与安全

### TC-FILE-05-01 可执行文件：申请时或 complete 时 415
- 关联 AC：AC-FILE-05-1
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. `kind=file` 分别申请 `a.exe`、`a.EXE`、`a.pdf.exe`
  2. 申请 `a.pdf`（`application/pdf`），PUT PE 文件内容，`complete`
- 预期结果：
  1. 前三个 415 / 4150，申请时就拒绝
  2. 第四个 `complete` 415 / 4150，临时对象被删

### TC-FILE-05-02 配置新增类型；先规范化再判断
- 关联 AC：AC-FILE-05-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：配置文件新增 `xyz`，重启
- 步骤：
  1. 分别申请 `a.xyz`、`a.html`、`b.svg`、`photo‮gpj.html`、`a.exe.`、`a.exe::$DATA`
- 预期结果：
  1. 都是 415 / 4150

### TC-FILE-05-03 下载前来源提示：每个会话首次
- 关联 AC：AC-FILE-05-3
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 群里点下载 zip；确认
  2. 同一会话再点另一个文件
  3. 另一会话点文件；点图片
- 预期结果：
  1. 第 1 次出现"文件来自其他用户，请确认来源可信"，确认后下载
  2. 同一会话第 2 次不提示；另一会话首次提示；图片不提示

### TC-FILE-05-04 无法识别的类型：文件通过，图片拒绝
- 关联 AC：AC-FILE-05-4
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：随机字节文件（`file(1)` 识别不出）
- 步骤：
  1. `kind=file` 上传，`complete`
  2. 同样内容 `kind=image`，声明 `image/png`，`complete`
- 预期结果：
  1. 前者 200，`mime=application/octet-stream`；后者 415 / 4150

### TC-FILE-05-05 png 后面藏 HTML：下载响应是 image/png
- 关联 AC：AC-FILE-05-5
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：合法 png 头，尾部 `<html><script>`
- 步骤：
  1. `kind=image` 上传，发送；接收方取原图地址
  2. 下载，读响应头
- 预期结果：
  1. `Content-Type: image/png`，不是 `text/html`；`Content-Disposition: inline`

### TC-FILE-05-06 文件表 scanState 默认 skipped
- 关联 AC：AC-FILE-05-6
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 上传完成；查 `files`
- 预期结果：
  1. `scan_state=skipped`

## F-FILE-06 生命周期

### TC-FILE-06-01 未绑定和 pending 25 小时后被清除
- 关联 AC：AC-FILE-06-1
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：F1 `ready` 未绑定；F2 `pending` 已 PUT
- 步骤：
  1. [IM 钟] 推进 23 小时，运行 `file_purge`
  2. 再推进 2 小时，运行 `file_purge`
  3. 查 `files`、假 S3
- 预期结果：
  1. 第 1 次都还在；第 2 次对象和记录都被删除

### TC-FILE-06-02 绑定后 181 天：对象删除，消息保留 expired，地址 404
- 关联 AC：AC-FILE-06-2
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：u1 给 u2 发图片 F1
- 步骤：
  1. 推进 179 天，运行清理，查对象
  2. 推进到 181 天，运行清理
  3. u2 `sync`；u2 申请 F1
- 预期结果：
  1. 第 1 步对象还在
  2. 第 2 步对象不存在；`sync` 消息还在，`body.image.expired=true`
  3. 地址 `code=4048`

### TC-FILE-06-03 一个文件只绑一条消息；原 clientMsgId 重发得原 ack
- 关联 AC：AC-FILE-06-4
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u1 用 `clientMsgId=c1` 发 F1，成功
- 步骤：
  1. u1 用 `clientMsgId=c2` 发 F1
  2. u1 用 `c1` 重发
- 预期结果：
  1. 第 1 步 409 / 4096，没有新消息
  2. 第 2 步返回第一次的 ack，`msgId`、`seq` 相同

### TC-FILE-06-04 对象删除失败：记录保留，下次重试
- 关联 AC：AC-FILE-06-5
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：F1 过期；假 S3 注入删除失败
- 步骤：
  1. 运行 `file_purge`；抓 `/metrics`
  2. 关闭注入；再运行
- 预期结果：
  1. 第 1 次 `files` 行还在（`purging`）；`im_job_runs_total{job="file_purge",result="error"}` 增加
  2. 第 2 次对象和行都处理完；`result="ok"` 增加

### TC-FILE-06-05 过期消息的显示，不发申请地址
- 关联 AC：AC-FILE-06-6
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：`expired=true` 的图片和文件消息
- 步骤：
  1. 打开聊天页；点击两条消息
- 预期结果：
  1. 分别显示"图片已过期""文件已过期"；没有 `/v1/files/urls` 请求

### TC-FILE-06-06 purging 的文件不能绑定
- 关联 AC：AC-FILE-06-7
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：F1 `ready`，超过 24 小时；暂停点停在 `file_purge` 改状态之后、删对象之前
- 步骤：
  1. 运行 `file_purge` 到暂停点
  2. u1 发 F1
- 预期结果：
  1. 409 / 4096，消息未保存；放行后对象和行被删

### TC-FILE-06-07 同一文件两个 clientMsgId 并发
- 关联 AC：AC-FILE-06-8
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 同时发 F1，`c1`、`c2`；重复 20 轮
- 预期结果：
  1. 每轮恰好一个成功，另一个 409 / 4096，只保存一条消息

### TC-FILE-06-08 保留期 0 不过期
- 关联 AC：AC-FILE-06-9
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：`IM_FILE_RETENTION_DAYS=0`；F1 已绑定
- 步骤：
  1. 推进 400 天，运行清理；申请地址，下载
- 预期结果：
  1. 文件不被删除；能下载；`retain_until` 为空

### TC-FILE-06-09 过期后文件名清除
- 关联 AC：AC-FILE-06-10
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：ES（[IM 钟]）
- 前置条件：F1 文件名含特征串
- 步骤：
  1. 推进 181 天，运行清理；查 `files.name`
- 预期结果：
  1. 为空

### TC-FILE-06-10 退群、解散：退群时其他人照常；解散后 404；对象都还在
- 关联 AC：AC-FILE-06-11
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：群 g1 文件 F1（u1 发），成员 u1、u2、u3
- 步骤：
  1. u1 退群；u2 申请 F1；查对象
  2. 解散 g1；u2、u3 申请 F1；查对象
- 预期结果：
  1. 退群后 u2 有 `url`，u1 `code=4048`
  2. 解散后都 `code=4048`；对象都还在，到保留期才删

### TC-FILE-06-11 过期递增 modVer：只重拉这个会话，变"已过期"
- 关联 AC：AC-FILE-06-12
- 优先级：P0
- 批次：C2
- 层级：SDK 单测（三套）
- 环境：E1 + 假 S3
- 前置条件：三套 SDK，假传输层；会话 c1、c2 已加载消息；c1 的 F1 已缓存（文件缓存有 `fileId + variant`，TS 只有内存）
- 步骤：
  1. 假服务端 `convs` 让 c1 的 `modVer` +1，c2 不变；重连
  2. 观察 `sync`；查 Store 和文件缓存
  3. 再发一个迟到的旧 `sync` 应答，F1 `expired=false`
- 预期结果：
  1. 只重拉 c1；消息变成已过期，本地缓存被清除
  2. 迟到的应答不会把 `expired` 改回假（单调合并）
- 说明：向量：`protocol/vectors/chat-sync.json` 的合并和过期用例

## F-FILE-07 附件发送状态机

### TC-FILE-07-01 上传中杀 App：显示失败，点重发从头传，只收到一条
- 关联 AC：AC-FILE-07-1
- 优先级：P0
- 批次：C2
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 假 S3 + 模拟器
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 发图片，上传中杀 App
  2. 重新启动，等 10 秒；看消息和请求
  3. 点重发；对方查看
- 预期结果：
  1. 消息显示失败，没有自动上传
  2. 重发从头上传（新 `fileId`）；`clientMsgId` 不变；对方只收到一条

### TC-FILE-07-02 complete 成功但 send 超时：手动重发不再上传
- 关联 AC：AC-FILE-07-2
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：ES（`drop-ack`）
- 前置条件：故障注入 `drop-ack` 一次
- 步骤：
  1. 发图片；`complete` 成功，`send` 没有 ack，10 秒后标记失败
  2. 手动重发；抓请求
  3. 对方查看
- 预期结果：
  1. 没有新的 PUT 和 `uploads`；复用同一 `fileId`、同一 `clientMsgId`
  2. 对方只收到一条

### TC-FILE-07-03 H5：上传失败后刷新丢失；不刷新能重发
- 关联 AC：AC-FILE-07-3
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）
- 环境：E1 + 假 S3
- 前置条件：假 S3 注入 PUT 失败一次
- 步骤：
  1. 发图片，失败；手动重发
  2. 再发一张，失败；刷新页面（接受离开确认）
- 预期结果：
  1. 第 1 步重发成功
  2. 第 2 步刷新前有离开确认；刷新后失败消息不再出现

### TC-FILE-07-04 文字越过上传中的图片；3 张图按选择顺序
- 关联 AC：AC-FILE-07-4
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：代理让第 1 张图 PUT 慢 5 秒
- 步骤：
  1. 依次选 3 张图；第 1 张上传中输入文字发送
  2. 看页面和 ack 的 seq
- 预期结果：
  1. 文字先显示已发送
  2. 3 张图按选择顺序 `send`，seq 递增；并发上传数 1

### TC-FILE-07-05 系统相册删了原图：沙盒副本能重发；成功后副本被删
- 关联 AC：AC-FILE-07-5
- 优先级：P0
- 批次：C2
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 假 S3 + 模拟器
- 前置条件：假 S3 注入第一次 PUT 失败
- 步骤：
  1. 从相册选图发送，失败
  2. 删除系统相册里的原图
  3. 点重发；成功后查 App 沙盒目录
- 预期结果：
  1. 重发成功；沙盒副本被删除

### TC-FILE-07-06 断网选图：失败可重发，只收到一条
- 关联 AC：AC-FILE-07-6
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试
- 环境：E1 + 假 S3
- 前置条件：无特殊要求（通用约定的账号和环境）
- 步骤：
  1. 断网，选图发送
  2. 恢复网络，点重发
- 预期结果：
  1. 标记失败；重发成功；对方只收到一条

### TC-FILE-07-07 弱网上传失败 5 次：不扣额度
- 关联 AC：AC-FILE-07-7
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：E1 + 假 S3
- 前置条件：代理让 PUT 连续失败 5 次
- 步骤：
  1. 同一文件手动重发 5 次
  2. 查 `upload_quota`
- 预期结果：
  1. 额度没有变化；每次都是消息失败状态

### TC-FILE-07-08 send 被拒后文件还是 ready，取消拉黑后可以绑定
- 关联 AC：AC-FILE-07-8
- 优先级：P0
- 批次：C2
- 层级：后端 e2e
- 环境：E1 + 假 S3
- 前置条件：u2 把 u1 拉黑；u1 `ready` 的图片 F1
- 步骤：
  1. u1：`send {clientMsgId:c1, image:{fileId:F1}}`
  2. 查 `files.state`；u2 取消拉黑
  3. u1 同 `c1` 重发
- 预期结果：
  1. 第 1 步错误码 4036；`state=ready`，发送令牌按判定顺序已扣
  2. 第 3 步成功，`state=attached`

### TC-FILE-07-09 退出登录清除沙盒副本
- 关联 AC：AC-FILE-07-9
- 优先级：P0
- 批次：C2
- 层级：原生 UI 测试（Android、iOS）
- 环境：E1 + 假 S3 + 模拟器
- 前置条件：9 个文件上传失败未处理
- 步骤：
  1. 退出登录；查沙盒目录
- 预期结果：
  1. 9 个副本都被清除

### TC-FILE-07-10 文件 24 小时被清理：原生重传，H5 提示
- 关联 AC：AC-FILE-07-10
- 优先级：P0
- 批次：C2
- 层级：Playwright（H5）+ 原生 UI 测试 + SDK 单测
- 环境：ES（[IM 钟]）
- 前置条件：原生端：待发送消息（文件已 `ready`），沙盒副本还在；H5 同样场景（页面未刷新）；[IM 钟] 推进 25 小时并运行 `file_purge`
- 步骤：
  1. 原生点重发
  2. H5 点重发（H5 刷新的场景单独跑一次）
- 预期结果：
  1. 原生：`send` 得到 4096，回到"上传中"，重新上传后发送成功；不算自动重发次数
  2. H5 刷新后没有源文件的场景：提示"原文件已不可用，请重新选择"，只能删除这条消息

## 对象存储合同（F-FILE-90）

### TC-FILE-90-01 真实 MinIO 合同测试
- 关联 AC：AC-FILE-90-1
- 优先级：P0
- 批次：C2
- 层级：合同测试
- 环境：真实 MinIO（CI，或 Frank 同意的临时构建）
- 前置条件：**现状：CI 不运行（PENDING P-21，Actions 预算是 $0）；开发机不装软件。这一项要 Frank 再次同意临时构建才能跑**（`MINIO_BIN_DIR=… tools/objstore-contract/run-local-minio.sh`，构建到临时目录、只监听 `127.0.0.1`、跑完删除）。2026-11-06 的结果见 GATE-objstore（46 项必需全部通过），是冻结前的技术验证，实现完成后要重跑
- 步骤：
  1. CI 恢复后：`deploy/testenv-ci.sh` 里的固定版本 MinIO，开启 SSE-S3；跑 `go run .`（环境变量见工具 README）
  2. 逐项核对工具输出：预签名 PUT 到临时键、超大 PUT、过期 PUT、未签名的头、`complete` 后覆盖、Copy、条件 Copy 成功和失败（412）、`If-Match` 的 Range GET 成功和失败、元数据替换（含恶意头）、跨桶 Copy、加密下 Copy 前后 ETag、`response-*` 头、过期 GET、无签名访问、匿名列举、前缀列举并删除、全桶列举被拒绝、CORS 预检、删除、生命周期配置
  3. 再用 im-server 对 MinIO 跑一遍 TC-FILE-01-01、01-09、01-15、01-21（在 `IM_TEST_MODE=1` 下）
- 预期结果：
  1. 退出码 0，必需项全部通过；优先项失败的写进部署说明
  2. "没有验证"的两项如实记录：生命周期实际清除（只查配置生效）、CORS 限定来源

### TC-FILE-90-02 云厂商合同测试
- 关联 AC：AC-FILE-90-2
- 优先级：P0
- 批次：C2
- 层级：手工
- 环境：Frank 选定的云厂商（CH-05）
- 前置条件：**等 Frank 选定云厂商（CH-05）**，未选定前记未执行。Frank 建好两个桶和最小权限凭证，凭证放环境变量，不写进文件和聊天
- 步骤：
  1. 在 `tools/objstore-contract` 下配 `OC_*`，`go run .`
  2. H5 用生产 CORS、CSP 配置上传、预览、下载一次；Android、iOS 各上传、下载一次
  3. 把结果写成 `docs/review/GATE-objstore-<厂商>-<日期>.md`
- 预期结果：
  1. 必需项全部通过，不通过的能力写进部署说明；必需项不通过时可能要改上传设计（R-18），报告 Frank
  2. 三端走查没有 CORS、CSP 问题；CORS 限定到 App 的来源

## 已有 AC 的扩展

### TC-FILE-91-01 元数据响应搜不到文件名（AC-ADM-30-7 的扩展，C2 起生效）
- 关联 AC：AC-ADM-30-7
- 优先级：P0
- 批次：已有 AC，随 C2
- 层级：后端 e2e
- 环境：E1 + 假 S3 + console-server
- 前置条件：群 g1 有文件消息，文件名特征串 `ZXQ-FILE-META`；客服账号有 `msg.meta`
- 步骤：
  1. 客服用元数据接口查询这条消息（console-server 响应和它调用 im-server 的响应都抓）
  2. 有正文授权的账号查正文接口
- 预期结果：
  1. 两层响应都搜不到特征串，只有类型和大小
  2. 正文接口有文件名；两边审计搜不到文件名
- 说明：第 16.2 节把这条扩展到文件名，C6 的新 AC 随 C6；C2 就有文件消息，所以这里先验

---

## 说明：写用例时发现的问题

不改 PRD，这里记录用例是怎么处理的，汇总在 [backend-chat](../tasks/backend-chat.md) 文末。

1. **AC-FILE-01-2**：PUT 签入了 Content-Length，声明 1 MiB 实际 2 MiB 的 PUT 被存储拒绝（GATE-objstore：403），`complete` 得到 4096，不是 `uploadMismatch`。用例用假 S3 测试接口直接写入，只验 `complete` 的核对逻辑。
2. **AC-FILE-01-18**：每 15 分钟 20 个 100 MiB 第 5 轮就撞上声明上限 8 GiB（RS5-16 后加的）；"24 小时后 `complete`"和未绑定 24 小时清理冲突。用例调大上限，把时钟推到 24 小时之前，少跑几轮。
3. **AC-FILE-01-11**：不取消就先撞上未完成数 20，第 61 次到不了。用例边申请边取消。
4. **AC-FILE-01-16**：要调短 PUT 地址有效期，第 4.1 节没有配置项。
5. **AC-FILE-02-2、02-11**：第 11.1 节说上行 `image` 只有 `fileId`，这两条 AC 要伪造字段不报错。用例按 AC。
6. **AC-FILE-04-1、04-2、04-3、04-5**：写的是"404"，而 F-FILE-04 规则 1 和 AC-FILE-04-9 是批量接口，单项不可访问 HTTP 200 + `code=4048`。用例断言 `code=4048`。规则 3 的"一律 404 / 4048"也应该按这个理解。
7. **AC-FILE-90-1**：工具现在不验证生命周期实际清除和 CORS 限定来源，AC 接受"检查配置生效"。
8. **后台删除文件的物理删除**：路径在 C2 就有，唯一验证 30 天后物理删除的 AC 标在 C4。本批只验到 `revoked`。
