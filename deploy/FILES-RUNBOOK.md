# 图片和文件部署说明（PRD-chat C2，BE-438、BE-439 第 2 条）

只写 C2 文件功能多出来的部分，账号部分见 [ACCOUNT-RUNBOOK.md](ACCOUNT-RUNBOOK.md)。数字都未校准。样例在 [`deploy/objstore/`](objstore/)，**只有占位符**：桶名、域名要换成实际的，凭证由部署机的密钥管理注入，不进仓库、不进聊天。

样例自检（只证明样例符合 PRD，不证明厂商接受）：

```sh
python3 -I -m unittest discover -s deploy/objstore -p 'test_*.py'
```

## 1. 桶（F-FILE-01 规则 9，第 21 节必需能力）

| 桶 | 样例名 | 要求 |
|---|---|---|
| 私有桶 | `tianting-chat-private` | 私有；匿名和预签名都不能列举；**开启服务端加密**（密钥归属随厂商定，CH-05）；**不开版本控制**（F-FILE-06 规则 8）；临时前缀 `tmp/` 1 天生命周期；CORS 限定来源 |
| 公开读桶 | `tianting-chat-public` | 只放已绑定的头像（`avatars/` 前缀）；匿名只能读对象，**不能列举**；独立域名（`IM_OBJSTORE_PUBLIC_BASE_URL`，https）；优先配 `X-Content-Type-Options: nosniff` |

键结构（服务端生成，不含文件名和用户 ID，F-FILE-01 规则 5）：`tmp/{随机}`（预签名 PUT 的目标）、`final/{fileId}/{attemptId}/{original|thumb}`、公开桶 `avatars/{随机}`。

**版本控制**：不开。如果厂商强制开启，删除只写删除标记，要另配"非当前版本 1 天后删除"并写进本文件，否则 `file_purge` 删不掉内容。

**访问日志**：关闭，或脱敏到不含查询串（预签名的签名在查询串里，R-CS-22）。网关、CDN、负载均衡的访问日志同样处理。

**备份**：数据库备份（`deploy/backup/`）**不含文件内容**，文件的持久性靠对象存储自身（多副本）。这一点要写进备份记录（F-OPS-12，第 14.2 节）。

## 2. 凭证（第 11.3 节）

im-server 用一份最小权限凭证，策略样例 [`policy-im-server.json`](objstore/policy-im-server.json)：

| 前缀 | 权限 | 用途 |
|---|---|---|
| 私有桶 `tmp/*` | 读、写、删 | 预签名 PUT（用这份凭证签名）、HEAD、Range 读、Copy 的源、删除临时键 |
| 私有桶 `final/*` | 读、写、删 | Copy 的目标、预签名 GET、`file_purge` |
| 私有桶列举 | **只允许 `prefix` 以 `final/` 开头** | 提交后清理其他尝试、`file_purge` 按 `final/{fileId}/` 前缀删除 |
| 公开桶 `avatars/*` | 写、删 | 头像 Copy 和清理 |

**全桶列举不给 im-server**，只给第 6 节的对账任务（单独的凭证，用完收回）。公开桶匿名读的策略样例 [`policy-public-read.json`](objstore/policy-public-read.json)（只有 `s3:GetObject`，不要用"公开下载"之类的预设，它们通常会放开列举）。

配置检查 `objstore_reachable` 用这份凭证按 `final/` 前缀列举 1 个键：不可达时显示"不可达"，能连通但 4xx 时显示"凭证或权限"。

## 3. 生命周期（F-FILE-01 规则 3a）

样例 [`lifecycle-private.json`](objstore/lifecycle-private.json)（S3 `PutBucketLifecycleConfiguration` 的 JSON 形式）：`tmp/` 前缀 1 天过期，未完成的分片上传 1 天清除。`complete` 之后再 PUT 的对象、临时键删除失败、放弃的上传都靠它清除，**不靠 `file_purge`**。`final/` 不能配生命周期：它由 `file_purge` 按保留期删除（`IM_FILE_RETENTION_DAYS`）。

MinIO 上的等价命令：`mc ilm rule add --prefix tmp/ --expire-days 1 ALIAS/tianting-chat-private`。

**实际清除没有验证**，见第 7 节。

## 4. CORS（第 21 节必需能力 ⑤）

H5 直接 PUT 到对象存储、用 `fetch` 下载，私有桶要配 CORS，样例 [`cors-private.json`](objstore/cors-private.json)：

- `AllowedOrigins` 只写 H5 的正式来源（https），**不能是 `*`**；测试环境的来源单独配在测试桶上。
- 方法 `PUT`、`GET`、`HEAD`；请求头只需要 `content-type`（`Content-Length` 由浏览器设置，不进预检）。
- 公开桶的头像用 `<img>` 显示，不需要 CORS。

**按来源限定没有验证**：MinIO 把请求的来源原样返回（GATE-objstore），说明它没有按桶限定。生产要在选定的厂商上确认，见第 7 节。

## 5. im-server 配置

样例 [`im-files.example.env`](objstore/im-files.example.env)。

| 配置 | 说明 |
|---|---|
| `IM_FILES_ENABLED` | 默认关。关时 `/v1/files/*` 404，`send` 的 image / file 4000，已有文件消息照常 `sync` |
| `IM_OBJSTORE_ENDPOINT` | **生产必须 https，否则拒绝启动**；只支持路径风格地址 |
| `IM_OBJSTORE_PUBLIC_BASE_URL` | 头像独立域名，**生产必填且必须 https**，否则拒绝启动（`users.avatar` 只接受 https） |
| `IM_OBJSTORE_BUCKET`、`IM_OBJSTORE_PUBLIC_BUCKET` | 两个桶必须不同，否则拒绝启动 |
| `IM_OBJSTORE_ACCESS_KEY`、`IM_OBJSTORE_SECRET_KEY` | 第 2 节的凭证。报错、日志、配置检查都不输出它们和地址 |
| `IM_FILE_BLOCKLIST` | 禁止名单文件，只能在内置名单上增加 |
| 额度、大小、有效期 | 见 PRD 第 4.1 节（`IM_UPLOAD_DAILY_BYTES`、`IM_FILE_MAX_BYTES`、`IM_UPLOAD_PUT_TTL`、`IM_FILE_URL_TTL_*`、`IM_FILE_RETENTION_DAYS` 等） |

对象存储单次请求的超时是 30 秒（代码默认值，没有配置项）。对象存储"黑洞"时，`complete` 最长等 30 秒后 503 / 5032；文本消息不受影响（AC-CHAT-90-3）。对象存储**不是** `/readyz` 的条件。

**开发环境（F-FILE-01 规则 10）**：开发环境允许 http 的 MinIO（配置检查 `objstore_https` 显示"不是 https"）。Android 禁止明文：要么给 MinIO 配 https，要么只在 debug 构建的 `network_security_config` 里给 MinIO 的主机名开明文例外，release 构建不能带这个例外。iOS 同理只在 Debug 配 ATS 例外。

## 6. 恢复数据库之后：多余对象的手动对账（F-FILE-06 规则 8）

恢复到较早时间点后，文件表和对象可能不一致：

- **缺对象**（库里有、对象没了）：消息显示"已过期"，不用处理。
- **多余对象**（对象在、库里没有这个 `fileId`）：要手动对账。第一批**没有自动工具**，按下面的步骤：

1. 等恢复后的服务稳定运行一段时间（至少大于 `complete` 租约 60 秒和 `IM_UPLOAD_PUT_TTL`，建议 1 小时），期间没有新的数据库恢复。
2. 用**单独的全桶列举凭证**（只在这次使用，用完收回）列出 `final/` 下的一级目录，得到 `fileId` 集合。
3. 和 `files` 表比对：不在表里的 `fileId`，且对象的最后修改时间早于"恢复时间点 − 1 小时"（宽限期要大于租约），才是多余对象。
4. 先输出清单人工核对，再按 `final/{fileId}/` 前缀删除。公开桶 `avatars/` 同样按 `files.public_key` 比对。
5. 不要在恢复后的状态稳定之前运行，否则会误删刚上传、还没提交的对象。

## 7. 没有验证的事情（BE-439 第 2 条，AC-FILE-90-1、90-2）

| 项 | 状态 | 上线前要做的 |
|---|---|---|
| 临时前缀生命周期的**实际清除** | 合同工具只检查规则能写入并读回；实际清除按厂商周期（MinIO 和多数厂商按天批处理），没有等 | 在选定厂商上配好后，放一个测试对象到 `tmp/`，按厂商周期记录实际清除时间 |
| CORS **按来源限定** | MinIO 原样返回请求的来源，没有限定；云厂商没测 | 在选定厂商上用另一个来源发预检，确认被拒绝 |
| 不存在的键返回 404 还是 403 | im-server 的凭证没有全桶列举权限。AWS 风格的存储对没有列举权限的不存在键可能返回 403 而不是 404；`complete` 把 404 当成"临时对象不存在"（4096），403 会变成内部错误 | AC-FILE-90-2 时用最小权限凭证 HEAD 一个不存在的 `tmp/` 键，确认是 404；不是的话报 Frank（可能要改判断或给 `tmp/` 列举权限） |
| 云厂商的条件 Copy、加密下的 ETag、按前缀授权和分页、未签名头的处理、`nosniff` | CH-05 还没选，没测 | AC-FILE-90-2：用同一套合同工具（`tools/objstore-contract`）和 `server/e2e/file_realstore_test.go` |

真实存储上的测试：`tools/objstore-contract` 验存储能力；`server/e2e/file_realstore_test.go`（`TestTC_FILE_90_01_RealStore`）把 im-server 的实现跑到真实存储上，设置 `IM_E2E_S3_*` 环境变量才运行，凭证只放环境变量。

## 8. 配置检查、指标和告警（第 14.2 节，AC-CHAT-90-17）

`GET /internal/v1/config` 的检查项：

| 检查项 | 含义 |
|---|---|
| `objstore_https` | 对象存储地址和头像域名是否 https（生产不是 https 时已经拒绝启动）；文件功能关闭时显示"关闭" |
| `objstore_reachable` | 对象存储能否连通、凭证能否按最终前缀列举（每次查询现算，最多 5 秒） |

指标（标签没有 userId、fileId、文件名）：

| 指标 | 含义 |
|---|---|
| `im_file_uploads_total{kind,result}` | `complete` 尝试的结果。`kind`：image / file / avatar；`result`：ok / mismatch / type_denied / not_ready / quota / unavailable / gone / error |
| `im_file_bytes_total` | `complete` 成功的实际字节数（原图 + 缩略图） |
| `im_file_url_issued_total` | 发出的下载地址数（不按用户统计） |
| `im_objstore_errors_total{op}` | 对象存储请求失败数（网络错误和存储返回的错误；404、412 是正常结果，不算）。`op`：head / get / list / put / copy / delete |
| `im_objstore_requests_total{op}` | 对象存储请求数（不含预签名），算错误率用。PRD 没有列，为告警加的 |
| `im_job_runs_total{job="file_purge",result}` | 清理任务 |

建议的告警规则（**第一批默认不发页面告警，先看值**，阈值未校准；没有放进 `alerts.yml`，那里是 PRD-ops F-OPS-07 的 11 条）：

```promql
# 对象存储错误率：5 分钟内 > 5% 且请求数 ≥ 20
sum(increase(im_objstore_errors_total[5m])) / sum(increase(im_objstore_requests_total[5m])) > 0.05
  and sum(increase(im_objstore_requests_total[5m])) >= 20

# 当天全局上传量：im-server 只能看到 complete 成功的字节，看不到临时前缀
sum(increase(im_file_bytes_total[24h])) > 50e9
```

**当天全局上传量要含临时前缀**（F-FILE-01 规则 6）：申请后不 `complete` 的对象只在 `tmp/` 里，im-server 的指标看不到。要用对象存储自己的用量指标（按桶或按前缀的存储量、PUT 次数）配告警，阈值按 PRD 第 15 节的容量假设（约 12.5 GB / 天）的几倍设，选定厂商后补上。

`file_purge` 连续失败：`increase(im_job_runs_total{job="file_purge",result="error"}[3h]) >= 3`（对象存储删除失败会保留记录下次重试，不影响消息）。
