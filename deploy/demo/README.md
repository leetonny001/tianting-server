# 天听本机演示环境

给分享会现场用：H5、管理后台、运维后台、Android、iOS 连同一套服务端，打开就有好友、群和历史消息可以演示，运维后台能看到节点、版本、在线连接、配置检查和数据库迁移。

- **全部在这台 Mac 上原生运行**：im-server、console-server（管理空间 + 运维空间）、Postgres 17、假 S3、H5、管理后台、运维后台。不用 docker，不用任何云服务，不注册账号，不花钱。
- **所有端口只绑 127.0.0.1**，外网和局域网都访问不到。
- 状态（随机密钥、数据库、日志、构建产物、进程号）都在 `deploy/demo/.run/`，不进仓库；`down` 时整个删掉。密钥每次 `up` 重新随机生成。
- 下面的账号密码是**演示用的测试值**，只在这套本机环境里有效。

## 需要什么

这台 Mac 上已经有的：Go、Node 22+、Postgres 17（`brew` 的 `postgresql@17`，只用它的 `initdb`、`pg_ctl`、`psql`）、python3、Android SDK 和 AVD `Pixel_10_Pro_XL`、Xcode 和 iPhone 16 Pro 模拟器。

同级目录要有 `tianting-console-server`、`tianting-web`、`tianting-admin`、`tianting-ops`、`tianting-sdk-ts`、`tianting-android`、`tianting-ios`，三个前端仓库（H5、管理后台、运维后台）和 SDK 要已经 `npm ci` 过（脚本不装任何依赖）。

## 一条命令

```bash
cd deploy/demo
./demo.sh up            # 构建 + 启动 + 灌演示数据，第一次约 2 分钟（Go、前端构建），之后约 1 分钟
./demo.sh android       # 启动 Android 模拟器（5556，-read-only），装好 App 并打开，约 3～5 分钟
./demo.sh ios           # 启动 iPhone 16 Pro 模拟器，构建、安装、打开 App，第一次约 5 分钟
./demo.sh status        # 各服务地址和健康状态

./demo.sh android-down  # 关 Android 模拟器（emu kill；-read-only，关掉后模拟器里的改动都不保留）
./demo.sh ios-down      # 结束 App，关掉 iPhone 16 Pro 模拟器
./demo.sh down          # 停掉全部（含上面两个模拟器），删除 .run/
```

其他：

```bash
./demo.sh reset         # 清空数据（IM 库、后台库、假 S3 里的图片和文件），重新灌一遍；服务不用重新构建
./demo.sh seed          # 只灌数据（已经灌过就跳过）
./demo.sh otp [账号]    # 后台账号（管理后台、运维后台）当前的动态码，例如 ./demo.sh otp shenhe、./demo.sh otp yunwei
./demo.sh ops-attach    # 只用于这次改动之前 up 起来、还没有运维后台的环境：补上运维后台，不清数据
```

`ops-attach` 会重启 im-server 和 console-server 各一次（几秒）：H5 自动重连，刷新也不用重新登录；管理后台的会话在 Postgres 里，也不用重新登录。新 `up` 的环境不需要它。

端口被占用时可以整体换一段：`DEMO_PORT_BASE=18480 ./demo.sh up`（之后的 `android`、`ios` 等命令也要带同样的变量）。

## 地址

| 端 | 地址 | 说明 |
|---|---|---|
| H5 | http://127.0.0.1:18384 | 生产构建（带 CSP），`vite preview` 做本地静态服务，`/v1/`、`/ws` 转发到 im-server |
| 管理后台 | http://127.0.0.1:18383 | 生产构建，由 console-server 托管（和线上一样同源、同样的响应头）。**要用 127.0.0.1，不要用 localhost**：后台校验 Origin |
| 运维后台 | http://127.0.0.1:18386 | `tianting-ops` 生产构建，由同一个 console-server 进程的运维空间托管（`CON_SPACES=admin,ops`，两个空间各自的端口、Cookie、账号和 CSP）。同样**要用 127.0.0.1** |
| Android | 模拟器 `emulator-5556` | debug 包，服务端 `http://10.0.2.2:18380`（模拟器看到的本机），图片和文件经 `adb reverse` 走 18382 |
| iOS | iPhone 16 Pro 模拟器 | Debug 包，启动时用环境变量 `TIANTING_SERVER=http://127.0.0.1:18380` 指定服务端 |
| im-server | 127.0.0.1:18380（客户端）、18381（管理接口）、18387（节点运维接口）、18388（指标） | 节点 `n1`，版本号构建时注入（`demo-<提交号>`）。运维接口按生产方式只走 TLS，见下面"运维后台" |
| 假 S3 | 127.0.0.1:18382 | 图片和文件，只在内存里 |
| Postgres 17 | 127.0.0.1:18385 | console-server 的库，临时实例，数据在 `.run/pg` |

im-server 打开的功能：C1 账号（邀请码注册、账号密码登录）、C2 图片和文件、C4 撤回 / 引用 / @、C5 置顶 / 免打扰 / 删除会话、C6 敏感词和举报，以及后台的第二批功能（封禁、禁言、删除消息、看正文）和系统通知。**C3 离线推送（APNs、FCM）不打开**。没有开测试模式和演示登录，登录都走真实的账号和密码。

## 账号

### 聊天账号（密码都是 `tt-demo-2026`）

| 昵称 | 账号 | 用在哪 | 关系 |
|---|---|---|---|
| 小明 | `xiaoming` | **H5** | 和小红、小刚、小丽、阿伟互为好友；爬山小队群主 |
| 小红 | `xiaohong` | **Android** | 好友 |
| 小刚 | `xiaogang` | **iOS** | 好友 |
| 小丽 | `xiaoli` | 备用（可以在另一个浏览器窗口登录 H5） | 好友 |
| 阿伟 | `awei` | 备用 | 好友 |
| 路人甲 | `lurenjia` | 陌生人：不是任何人的好友，在演示大群里发了广告 | 给小明发了好友申请（待处理） |

### 后台账号（密码都是 `Tianting-Demo-Only-2026`）

| 角色 | 账号 | 能做什么（节选） |
|---|---|---|
| 超管 | `chaoguan` | 后台账号管理、审批全员通知 |
| 运营 | `yunying` | 用户、群、邀请码、系统通知、**词库** |
| 审核 | `shenhe` | **举报处理**、封禁、禁言、删除消息、查看正文和举报证据 |
| 客服 | `kefu` | 查用户、踢下线、看会话元数据、禁言 |

### 后台怎么登录（动态码）

后台登录是"账号 + 密码 + 动态码（TOTP）"。console-server 的测试模式**没有**固定的测试动态码（只有拨时钟的测试接口），这套演示环境也不开测试模式，所以动态码是真的，按下面两种方式拿：

1. **命令行**：`./demo.sh otp shenhe`（或 `./demo.sh otp 审核`、不带参数列出全部），把 6 位数字填进"动态码"。
2. **手机验证器**（可选）：`.run/console-accounts.json` 里每个账号有 `otpauthUri`，可以导入自己手机上已有的验证器 App。

注意：同一个码只能用一次。灌数据脚本激活账号时用过当时的码，所以 `up` / `reset` 刚结束的 30 秒内登录可能提示动态码错误；刚登录完马上又要码（比如查看举报证据、删除消息要重新验证）会提示动态码错误，等 30 秒换下一个码再填。

### 运维后台账号（http://127.0.0.1:18386，密码也是 `Tianting-Demo-Only-2026`）

运维空间的账号和管理后台的是两套，互相登录不了。登录方式一样：账号 + 密码 + 动态码，动态码用 `./demo.sh otp yunwei`（或 `./demo.sh otp 运维值班`）。

| 角色 | 账号 | 能做什么 |
|---|---|---|
| 运维值班（`ops_engineer`） | `yunwei` | **演示用这个**：总览、节点、连接排查、告警静默、配置检查、数据库与备份 |
| 运维超管（`ops_superadmin`） | `ywchaoguan` | 只管运维账号（建账号、授角色），看不到总览和节点（PRD-ops 的职责分离） |
| 运维超管（第二个） | `ywfuhe` | 同上。授予 `ops_engineer` 要第二个超管批准：`yunwei` 是 `ywchaoguan` 申请、`ywfuhe` 批准的 |

两个运维超管只能用命令行创建（`console-server account bootstrap --space ops`，`demo.sh` 里做了），网页上不能再加超管。

## 演示数据

`./demo.sh up` 和 `./demo.sh reset` 会自动灌（`lib/seed.mjs`，用管理接口和 SDK，图片和 PDF 是脚本现场画出来的，没有版权问题）：

- **好友**：小明、小红、小刚、小丽、阿伟两两互为好友；路人甲 → 小明 的好友申请待处理。
- **群**：托管群"天听演示大群"（后台建的，没有群主，6 个人都在）；自建群"周末爬山小队"（小明建的，小红、小刚、小丽在）。
- **小明 ↔ 小红**：历史文字、一张图片（日落山景）、一个文件（周末行程安排.pdf）、一条引用回复、一条撤回的消息。
- **周末爬山小队**：图片（路线起点）、`@小红`、引用、`@所有人`。
- **其他单聊**：小明 ↔ 小刚、小红 ↔ 小刚、小明 ↔ 小丽各有几条。
- **会话设置**：小明置顶和小红的单聊，小红置顶爬山小队，小刚置顶和小明的单聊；三个人都把演示大群设成免打扰。
- **敏感词**：`演示违禁词`（无害的演示词，不是真实敏感词）。
- **举报**（都待处理）：路人甲在演示大群里的广告被小明、小丽、阿伟举报（3 次）；路人甲本人被举报 2 次；演示大群被举报 1 次。小红、小刚没有举报那条广告，留给现场演示。
- **运维后台账号**：`lib/seed-ops.mjs`，见上面的表。

## 运维后台

console-server 的运维空间（PRD-ops），和管理后台是同一个进程、同一个库，端口、Cookie（`__Host-ops_sid`）、账号和 CSP（更严：没有 `data:` 图片、没有样式 nonce、`X-Frame-Options: DENY`）都是分开的。

**节点运维接口按生产方式只走 TLS**：内部 CA 和节点证书用生产的 `deploy/pki/issue-node-cert.sh` 签发，放在 `.run/pki/`（CA 私钥也在这里，`down` 时随 `.run/` 删除）；节点表 `.run/ops/nodes.json` 里 `n1` 的 `tlsServerName` 是 `n1.ops.internal`，console-server 只信任这个 CA（`CON_OPS_CA_FILE`）。代码其实允许回环地址不带服务名走明文，这里没有用。节点签名私钥（Ed25519）只在 console-server 这边（`.run/ops/n1.key`，0600），im-server 只配公钥。im-server 的运维接口只接受 127.0.0.1 来源（`IM_OPS_ALLOWED_CIDRS`）。

能演示的（数据都是真的，来自 im-server 节点，console-server 每 5 秒经 TLS 探测一次）：

| 页面 | 看什么 |
|---|---|
| 总览 | 服务状态"正常"、版本 `demo-<提交号>`、节点就绪 1 / 1、在线连接数和平台分布（H5 登录后是 web 1，手机登上去会变） |
| 节点 | `n1`：地址、版本、就绪、连接数、启动时间和运行时长、内存估算、goroutine、断开缓冲 |
| 连接排查 | 填 userId（例如 `xiaoming`）、工单号（例如 `DEMO-001`，3～32 位字母数字）、理由（4～200 个字，例如 `分享会演示连接排查`）→ 看到他在各端的连接和最近断开记录。每次查询写审计，每人每天最多查 50 个不同用户 |
| 配置检查 | 每个节点的生产启动保护检查项（这套环境不是生产，大多显示"不适用"）、`GOMEMLIMIT` 提示 |
| 数据库与备份 | im-server 的 SQLite 位置、连通性、迁移版本和每个迁移的校验和 |

看不到的（这台 Mac 上没有，按约定不安装，页面会如实显示"数据不可用"，不会显示 0）：

- **Prometheus**：总览的消息量、错误率、握手与断开没有数据。im-server 的原始指标是开着的，可以 `curl http://127.0.0.1:18388/metrics` 看，但没有地方汇总。
- **Alertmanager**：告警卡片显示"告警数据不可用"，告警静默页能填表但提交不了。
- **Grafana**：没有配置，"指标"入口进不去。
- **备份存储**：没有配置备份桶，"备份（最近一次成功）"和备份记录显示数据不可用。
- **审计页、账号页**：`yunwei` 没有这两个权限（职责分离）；账号页用 `ywchaoguan` 登录看，审计要 `ops_auditor` 角色，演示数据里没有建。

## 建议演示路线（约 11 分钟）

开场前：`./demo.sh up`、`./demo.sh android`、`./demo.sh ios`；H5 用小明登录，Android 用小红登录，iOS 用小刚登录；另开一个浏览器窗口登录后台（审核 `shenhe`），再开一个标签页登录运维后台（运维值班 `yunwei`）。

| 时间 | 做什么 | 看什么 |
|---|---|---|
| 0:00 | 三个端的会话列表 | 置顶的会话在最前；演示大群有免打扰图标；小红的爬山小队有"有人@我"；历史里的图片、文件能打开 |
| 1:00 | H5 小明给小红发一句话、发一张图 | Android 上小红马上收到；小红回一句，H5 收到 |
| 2:30 | 小明在爬山小队发 `@小刚 …` | iOS 上小刚的会话出现"有人@我" |
| 3:30 | 小明发一条再撤回（2 分钟内）；小红长按一条消息"引用"回复 | 两边都变成"撤回了一条消息"；引用条显示原文 |
| 5:00 | 小刚在 iOS 会话列表里对一个会话置顶 / 取消置顶、开关免打扰（会话上的操作菜单） | 列表顺序和免打扰图标马上变化 |
| 6:00 | H5 小明发"这里有演示违禁词" | 发送失败，提示"消息包含不当内容，无法发送"；后台运营 `yunying` 的"词库"里能看到这个词，也可以现场加一个新词再试 |
| 7:00 | Android 小红在演示大群里长按路人甲的广告 → 举报 | 后台"举报"页这条群消息的次数从 3 变成 4 |
| 8:00 | 后台审核 `shenhe`：举报 → 这条群消息"查看每条" → 打开一条 → 查看证据（填理由 + 动态码）→ 结案"已处理"；可以再封禁路人甲或删除那条广告（这些操作也要动态码） | 证据里能看到广告原文；每条举报单独结案，结一条次数就少一 |
| 9:30 | 运维后台 `yunwei`：总览 → 节点 → 连接排查（userId `xiaohong`，工单号 `DEMO-001`，理由 `分享会演示连接排查`，理由要 4～200 个字）→ 配置检查 | 总览的在线连接按平台分（web、android、ios）；排查结果里能看到小红在 Android 上的连接；Prometheus、Alertmanager、Grafana 没有装，相关卡片如实显示"数据不可用" |
| 11:00 | 收尾 | `./demo.sh down` |

`@所有人` 每个群每小时只能 3 次（演示数据已经用了 1 次）。

## 常见故障

| 现象 | 处理 |
|---|---|
| `up` 说端口被占用 | 先 `./demo.sh status` 看是不是上次没关；不是的话换一段端口 `DEMO_PORT_BASE=18480 ./demo.sh up` |
| `up` 中途失败 | 看 `.run/logs/` 里对应的日志（build-*.log、pki.log、im-server.log、console-server.log、postgres.log、seed.log、seed-ops.log），`./demo.sh down` 后再 `up` |
| 后台提示动态码错误 | 码只能用一次、30 秒换一次：等下一个码。还不行看 Mac 的时间是否准 |
| 后台登录后马上回到登录页 / 请求被拒 | 地址要用 `http://127.0.0.1:18383`，不要用 localhost |
| 运维后台登录后什么都看不到 / 只有账号页 | 用的是运维超管（`ywchaoguan`、`ywfuhe` 只管账号），换 `yunwei`；地址用 `http://127.0.0.1:18386` |
| 运维后台节点显示不可达 | `./demo.sh status` 看"节点运维接口"那一行（检查 TLS 握手和证书）；看 `.run/logs/im-server.log`、`console-server.log` |
| H5 图片、文件打不开 | 地址用 `http://127.0.0.1:18384`；假 S3 只在内存里，单独重启过就会丢图，`./demo.sh reset` |
| 手机提示登录失效 / 账号或密码错误 | 做过 `reset` 后 App 里还是旧数据：重新运行 `./demo.sh android` / `./demo.sh ios`（会重装 App） |
| iOS 从主屏幕重新打开后连不上 | 服务端地址是启动时的环境变量带进去的，从主屏幕打开不会带：再运行一次 `./demo.sh ios` |
| `android` 说 5556 被占用 | 有别的模拟器在用这个端口，脚本不会碰它：先自己关掉那个模拟器 |
| Android App 收不到图片 | `adb -s emulator-5556 reverse --list` 应该有 18382；重新运行 `./demo.sh android` |
| 灌数据失败 | 看 `.run/logs/seed.log`（运维后台账号看 `seed-ops.log`）；`./demo.sh reset` 会清空重灌 |
| 想看哪个进程是演示环境起的 | `.run/pids/*.pid`；`down` 只按这些 PID 停进程，不会动别的 |

## 已知限制

- **没有离线推送**（C3 关着）：App 在后台时收不到系统通知，演示时让 App 留在前台。
- **没有公网地址**：只能在这台 Mac 和它的模拟器上用，观众的手机连不上。
- **单聊消息的举报证据看不到正文**：单聊正文开关 `IM_C2C_BODY_ACCESS` 默认关（隐私设计），所以举报演示用群消息。
- 假 S3 只在内存里：`down` / `reset` 之后图片和文件就没了（`reset` 会重新生成演示用的那几张）。
- Android 模拟器用 `-read-only` 启动：关掉后装的 App 和登录状态都不保留，每次 `./demo.sh android` 都是全新安装。
- iOS 每次 `./demo.sh ios` 都会卸载重装 App（只动 iPhone 16 Pro 这一台模拟器）。这台 Mac 的 Xcode 27 没有 Simulator.app，脚本会打开 Xcode 自带的 DeviceHub 来显示模拟器；如果那里看不到画面，可以用 Claude 的 iOS 模拟器面板，或者 `xcrun simctl io 46E6F7B9-9E80-4E9C-9C04-9C3FF0A94E9F screenshot 文件名.png` 截图。
- H5 没有 favicon，浏览器控制台里有一条 `/favicon.ico` 404，不影响使用。
- 时间显示：后台按北京时间显示，H5 和手机按本机时区显示。
- **运维后台没有 Prometheus、Alertmanager、Grafana、备份存储**：这台 Mac 上没有，按约定不安装。消息量、错误率、告警、告警静默、指标面板、备份记录都显示"数据不可用"，详见上面"运维后台"。
- 运维后台只有一个节点 `n1`：演示不了多节点、版本不一致、节点不可达（这些在测试环境 `deploy/testenv` 里测）。

## 文件

| 文件 | 内容 |
|---|---|
| `demo.sh` | 全部命令 |
| `lib/seed.mjs` | 演示数据（用 im-server 管理接口、`tianting-sdk-ts` 和 console-server 接口） |
| `lib/media.mjs` | 现场画演示图片、生成演示 PDF |
| `lib/totp.mjs` | 动态码（`demo.sh otp`） |
| `lib/seed-ops.mjs` | 运维后台账号（运维空间的两个超管和运维值班，双人批准） |
| `lib/keys.mjs` | 生成 Ed25519 密钥：console-server 调 im-server 管理接口用的、运维接口的节点签名密钥 |
| `fake_s3.py` | 假 S3（复制自 tianting-web/e2e/fake_s3.py） |
