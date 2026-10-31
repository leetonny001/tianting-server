# im-mvp

自研 IM：Go 服务端 + 移动端（Android / iOS）+ PC + H5。整体方案见 [docs/PLAN.md](docs/PLAN.md)，协议见 [protocol/README.md](protocol/README.md)。

## 当前进度

- [x] M0 规划与骨架：服务端骨架、`/healthz`、协议 Schema 草案 + 用例
- [x] M1 服务端核心：鉴权、WebSocket 网关、心跳、单聊 / 群聊、ACK、seq、去重、sync、已读
- [ ] M2 聊天室 + 管理接口（推送、禁言、删消息）

## 服务端

需要 Go 1.27+，无其他依赖（SQLite 为纯 Go 实现，内嵌）。

```bash
cd server
go run ./cmd/im-server                 # 默认 :8080，数据写入 ./im.db
go test -race ./...                    # 单测 + 协议一致性 + 端到端测试
```

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `IM_ADDR` | `:8080` | 监听地址 |
| `IM_DB` | `im.db` | SQLite 文件 |
| `IM_JWT_SECRET` | 开发密钥 | 用户 token 签名密钥，≥ 16 字节，**生产必须设置** |
| `IM_ADMIN_KEY` | `dev-admin-key` | 管理接口密钥，**生产必须设置** |

### 手动体验

```bash
# 1. 业务服务端为用户签发 token
curl -s -X POST localhost:8080/admin/users/token \
  -H 'X-Admin-Key: dev-admin-key' -d '{"userId":"alice"}'

# 2. 建群
curl -s -X POST localhost:8080/admin/groups \
  -H 'X-Admin-Key: dev-admin-key' -d '{"groupId":"g1","name":"测试群","members":["alice","bob"]}'

# 3. 用任意 WebSocket 客户端连接（如 websocat），然后发 JSON 帧
#    ws://localhost:8080/ws?token=<token>&platform=web
#    {"id":"r1","cmd":"send","data":{"convType":"c2c","to":"bob","clientMsgId":"cid-00000001","type":"text","body":{"text":"hi"}}}
```

### 代码结构

```
server/
├── cmd/im-server/      入口：读配置、优雅退出
├── internal/
│   ├── app/            组装各模块（main 和 e2e 共用）
│   ├── api/            /healthz
│   ├── admin/          /admin/*：签发 token、建群、加成员
│   ├── gateway/        /ws：鉴权、读写循环、心跳超时、慢连接保护
│   ├── hub/            在线表：用户 → 多个连接
│   ├── logic/          send / sync / read：权限、落库、投递
│   ├── store/          存储接口 + SQLite 实现（seq 分配、去重）
│   ├── auth/           HS256 JWT
│   └── protocol/       帧类型、错误码、校验；测试保证与 Schema 一致
└── e2e/                起完整服务，多客户端跑真实场景
```
