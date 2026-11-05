# 运维测试环境（BE-320）

PRD-ops 3.3 节的 E1、E2a。**只在测试机上运行，开发机不装 docker。**
测试机需要：docker compose v2、Go（生成密钥、构建 `im-loadtest`）、jq、curl、openssl、python3（带 pyyaml，通知内容扫描要用）。

```bash
deploy/testenv/tt up e1          # E1 单节点，不含 console-server
deploy/testenv/tt up e2a         # E2a：再加 n2（版本 0.3.1、自己的 SQLite、同一份 JWT 密钥）
deploy/testenv/tt down           # 删除容器、数据卷和 .run/
deploy/testenv/verify.sh         # 环境验收，约 15 分钟
```

带 console-server：

```bash
export TIANTING_CONSOLE_DIR=/path/to/tianting-console-server   # 默认在 tianting-server 旁边
export TIANTING_OPS_DIST=/path/to/tianting-ops/dist            # 先在 tianting-ops 里 npm run build
deploy/testenv/tt up e2a --console
deploy/testenv/tt bootstrap eng1
```

运维后台是 `http://ops.localhost:18290`，Grafana 是 `http://grafana.ops.localhost:18290`。

## 里面有什么

| 服务 | 说明 |
|---|---|
| n1（n2） | `IM_TEST_MODE=1`，版本号构建时注入。WS 18080（18180），管理 18081（18181），只绑回环 |
| Prometheus、Alertmanager、Grafana、node_exporter | **直接挂载 `deploy/observability/` 的生产配置，一个字不改**。它们和 console-server 共用一个网络命名空间，所以配置里的 `127.0.0.1` 原样成立 |
| webhook | 办公 IM 机器人的替身，记录每条通知。`tt notifications` 查看 |
| MinIO | 从源码构建；桶和两份最小权限凭证由 `minio-init` 创建，策略文件和生产相同 |
| Postgres、console-server | profile `console`，`CON_TEST_MODE=1`，只开运维空间 |

抓取清单由生产的 `gen-targets` 从节点表（`.run/nodes/nodes.json`）生成。节点运维接口走 TLS，证书由生产的 `deploy/pki/issue-node-cert.sh` 签发（console-server 不接受非回环地址走明文）；CA 私钥留在 `.run/pki/`，不挂载给任何容器。密钥都是 `tt` 随机生成的，放在 `.run/`（不进仓库）。

## 系统通知（push）的调用方配置

测试环境现在**没有**配置 `IM_ADMIN_CALLERS` 和 `IM_PUSH_ENABLED`：`tt` 只用旧管理密钥调测试接口，console-server 只开运维空间，不调用 `POST /admin/push`，所以这里没有 `pushTypes` 要配。

要在这里联调管理后台的系统通知时，配置以 `deploy/im-admin-callers.example.json` 为准：`IM_PUSH_ENABLED=1` 时持有 `push.all` 的调用方必须配非空的 `pushTypes`，否则 im-server 启动失败；名单要和 console-server 的 `CON_PUSH_TEMPLATES`（示例在 tianting-console-server 的 `deploy/push-templates.example.json`）的 type 一致，否则发送返回 403 / 4030。

## 注意

- `verify.sh` 的 TC-PERF-00-01 要求 seq 从 1 开始，只能在新环境里跑一次（`verify.sh` 自己会先 `tt down`）。
- `tt set-clocks` 冻结时钟：令牌桶不再补充（发消息被限流），发给 MinIO 的签名时间也不走。用完运行 `tt release-clocks`。
- console-server 只在启动时读节点表。`tt up` 从 e1 升到 e2a 时会重启它；生产上增减节点也要重启。
- n1、n2 的进程不做健康检查：`docker` 的健康检查碰到 `kill -STOP` 会把容器标成不健康，和这里要测的东西混在一起。

## 辅助命令

`tt` 不带参数会列出全部命令。和验收用例的对应：

| 用例 | 命令 |
|---|---|
| AC-OPS-07-2 节点不可达 | `tt kill n1`，`tt start n1`，`tt notifications` |
| AC-OPS-07-3 备份过期 | `tt backup-age im 7260`，`tt backup-age im 0` |
| AC-OPS-08-11 节点无响应 | `tt pause n2`，`tt resume n2` |
| AC-OPS-04-6 版本不一致 | `tt up e2a`（30 分钟用 `tt set-clocks`，先 [IM 钟] 后 [C 钟]） |
| AC-OPS-90-5 观测组件全停 | `tt stop-observability`，`tt start-observability` |

## console-server 联调

`verify.sh --console` 最后跑 console-server 仓库的 `tools/ops-accept`：真实账号和 TOTP 登录，读真实组件，失败不中断，一次列全。

| 验收项 | 内容 |
|---|---|
| AC-OPS-90-3 | 双节点用户 300 次排查（3 个账号），P99 ≤ 800ms |
| AC-OPS-14-6 | 单节点和双节点用户排查后 `deploy/ops/reconcile`，差异 0 条 |
| AC-OPS-08-11 | n2 暂停时查只在 n2 上的用户：2.5 秒内返回部分结果，不判为不在线 |
| AC-OPS-04-2、04-5 | 真实 Prometheus 上的消息量；Prometheus 停止时卡片不可用 |
| AC-OPS-04-6、05-1 | 节点和版本来自探活；时钟推进后显示版本不一致超过 30 分钟 |
| AC-OPS-07-5 | 备份过期经真实 Alertmanager 进告警卡片，恢复后消失 |
| AC-OPS-12-3 | 只读凭证读真实 MinIO 的清单，超过 2 小时标红 |
| Grafana | 票据进入，经认证代理读真实面板和数据，写接口被拦 |

这里测的是接口。页面文案和交互由 tianting-ops 的 Playwright 覆盖（后端用假服务）。

## 没有覆盖

- **AC-OPS-90-2**：要在 1 万连接的 EP 环境里和 BE-321 同轮测。

- **邮件 receiver**：配置里的 SMTP 是占位地址，这里不发信，Alertmanager 日志里会有发信失败。邮件要等 F-1 定下接收人和 SMTP。
- **防火墙**（`firewall.nft`）：容器网络里不适用。
