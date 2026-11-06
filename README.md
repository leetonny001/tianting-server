# 天听（tianting-server）

**天听**是自研 IM 平台：Go 服务端 + 统一 SDK + 移动端（Android / iOS）+ PC + H5 + 管理后台 / 运维后台，按类似微信的 IM 产品设计。
本仓库是服务端，同时存放协议和全部需求 / 架构 / 测试 / 流程文档；其他部分各自一个仓库：

| 仓库 | 内容 | 本地目录（同级） |
|---|---|---|
| [tianting-server](https://github.com/leetonny001/tianting-server) | Go 服务端、`protocol/`（协议唯一真相源）、需求 / 架构 / 测试 / 流程文档、压测与追溯工具 | `im-mvp/`（本仓库，本地目录沿用旧名） |
| [tianting-sdk-ts](https://github.com/leetonny001/tianting-sdk-ts) | SDK（TypeScript）：H5、PC | `tianting-sdk-ts/` |
| [tianting-sdk-kotlin](https://github.com/leetonny001/tianting-sdk-kotlin) | SDK（Kotlin）：Android | `tianting-sdk-kotlin/` |
| [tianting-sdk-swift](https://github.com/leetonny001/tianting-sdk-swift) | SDK（Swift Package）：iOS | `tianting-sdk-swift/` |
| [tianting-web](https://github.com/leetonny001/tianting-web) | H5 客户端（React），也是 PC 的界面 | `tianting-web/` |
| [tianting-desktop](https://github.com/leetonny001/tianting-desktop) | PC 客户端（Tauri 壳，加载 tianting-web） | `tianting-desktop/` |
| [tianting-android](https://github.com/leetonny001/tianting-android) | Android 客户端（Kotlin + Compose） | `tianting-android/` |
| [tianting-ios](https://github.com/leetonny001/tianting-ios) | iOS 客户端（Swift + SwiftUI） | `tianting-ios/` |
| [tianting-admin](https://github.com/leetonny001/tianting-admin) | 管理后台 H5（业务运营） | `tianting-admin/` |
| [tianting-ops](https://github.com/leetonny001/tianting-ops) | 运维后台 H5 | `tianting-ops/` |

全部私有。各 SDK 仓库里的 `protocol/` 是本仓库 `protocol/` 的副本，用各自的 `scripts/sync-protocol.sh` 同步，不在副本里改。后台服务（`console-server`，X-1）拍板后再建仓库。
整体方案见 [docs/PLAN.md](docs/PLAN.md)（第 0 节是整体布局），协议见 [protocol/README.md](protocol/README.md)，管理接口见 [protocol/ADMIN.md](protocol/ADMIN.md)。

## 当前进度

工作队列和状态见 [docs/process/QUEUE.md](docs/process/QUEUE.md)，每天的日报在 [docs/reports/daily/](docs/reports/daily/)。

- [x] 需求：PRD v1.2（128 条验收标准）、评审、任务单、310 个测试用例，追溯 128/128
- [x] 服务端核心（B0～B5）：鉴权、WebSocket 网关、单聊 / 群、会话列表、未读已读、多端同步；架构与安全基座；可观测；压测（7 条性能用例全部通过）
- [x] 统一 SDK（TypeScript，[tianting-sdk-ts](https://github.com/leetonny001/tianting-sdk-ts)）：连接管理、同步引擎、发送队列、已读；82 个单元测试 + 对真实服务端的集成测试
- [ ] H5 客户端（UI 参考微信）、PC、Android、iOS
- [ ] 管理后台 H5、运维后台 H5
- [x] 系统推送（站内通知）

## 服务端

需要 Go 1.27+，没有其他依赖（SQLite 为纯 Go 实现）。

```bash
cd server
go run ./cmd/im-server                 # 默认 :8080，数据写入 ./im.db（开发环境自动迁移）
go test -race ./...                    # 单测 + 契约测试 + 端到端测试（含进程级测试）
```

配置全部走环境变量，完整列表见 [server/cmd/im-server/main.go](server/cmd/im-server/main.go) 顶部注释。常用的：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `IM_ENV` | `development` | `production` 时，不安全的配置（默认密钥、测试模式等）拒绝启动 |
| `IM_ADDR` | `:8080` | 监听地址 |
| `IM_DB` | `im.db` | SQLite 文件 |
| `IM_JWT_SECRET` / `IM_ADMIN_KEY` | 开发密钥 | 生产必须设置，至少 32 字节 |
| `IM_QUARANTINE_KEY` | 开发密钥 | 被删消息原文的加密密钥。生产必须设置，至少 32 字节，不能与其他密钥相同。换密钥后，旧的隔离原文解不开 |
| `IM_QUARANTINE_DAYS` | `30` | 隔离原文保存天数，1～365 |
| `IM_V13_ADMIN_FEATURES` | 关闭 | 管理后台第二批（封禁、禁言、删消息、正文授权）。sdk-ts 对应版本发布后再打开 |
| `IM_ALLOWED_ORIGINS` | 只允许本机 | 浏览器 Origin 白名单 |
| `IM_ADMIN_ADDR` | `127.0.0.1:8081` | 管理接口 `/admin/*`，只给内网；对外端口上没有 |
| `IM_METRICS_ADDR` | `127.0.0.1:9090` | Prometheus 指标和 pprof，只给内网 |

运维命令：`im-server migrate up|status`、`im-server backup -o FILE`、`im-server restore -i FILE`。

### 手动体验

```bash
# 1. 业务服务端为用户签发 token，并建群（管理接口在单独的管理端口 IM_ADMIN_ADDR，默认 127.0.0.1:8081）
curl -s -X POST localhost:8081/admin/users/token -H 'X-Admin-Key: dev-admin-key' -d '{"userId":"alice"}'
curl -s -X POST localhost:8081/admin/groups -H 'X-Admin-Key: dev-admin-key' \
  -d '{"groupId":"g1","name":"测试群","members":["alice"]}'

# 2. 用任意 WebSocket 客户端（如 websocat）连接 ws://localhost:8080/ws?platform=web
#    第一帧发 auth（token 不放在 URL 里），之后就可以收发消息：
#    {"id":"a1","cmd":"auth","data":{"token":"<token>"}}
#    {"id":"r1","cmd":"send","data":{"convType":"group","to":"g1","clientMsgId":"cid-00000001","type":"text","body":{"text":"hi"}}}
```

### 代码结构

```
server/
├── cmd/im-server/      入口：配置、子命令（migrate / backup / restore）、优雅退出
├── cmd/im-loadtest/    压测工具（TC-PERF-00-01～07）
├── internal/
│   ├── app/            组装各模块（main 和 e2e 共用）
│   ├── config/         环境变量与生产启动保护
│   ├── api/            /healthz、/readyz
│   ├── admin/          /admin/*：用户、token、强制下线、群管理
│   ├── gateway/        /ws：Origin、首帧鉴权、心跳、慢连接、握手限流
│   ├── hub/            本节点的在线表（delivery.Router 的单节点实现）
│   ├── delivery/       投递接口（集群时换成消息总线实现）
│   ├── cache/          缓存失效广播接口
│   ├── logic/          send / sync / read / convs：权限、落库、投递、群成员缓存
│   ├── store/          存储接口、SQLite 实现、版本化迁移、备份恢复、契约测试
│   ├── metrics/        Prometheus 指标
│   ├── auth/           JWT（kid 密钥轮换）
│   ├── ratelimit/      令牌桶
│   └── protocol/       帧类型、错误码、校验；测试保证与 Schema、共享用例一致
└── e2e/                起完整服务，多客户端跑真实场景
```

## SDK 和客户端

在各自的仓库里，见上表。TypeScript SDK 的集成测试会编译并启动本仓库的服务端（默认找同级目录，或设置 `TIANTING_SERVER_DIR`）。
