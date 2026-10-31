# im-mvp

自研 IM：Go 服务端 + 移动端（Android / iOS）+ PC + H5。整体方案见 [docs/PLAN.md](docs/PLAN.md)，协议见 [protocol/README.md](protocol/README.md)。

## 当前进度

- [x] M0 规划与骨架：服务端骨架、`/healthz`、协议 Schema 草案 + 用例
- [ ] M1 服务端核心

## 服务端

需要 Go 1.27+。

```bash
cd server
go run ./cmd/im-server            # 默认监听 :8080，可用 IM_ADDR 修改
curl localhost:8080/healthz
go test ./...                     # 包含协议 Schema 与用例的一致性测试
```
