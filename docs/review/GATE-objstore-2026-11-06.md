# 对象存储技术验证门：MinIO 结果

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-06 |
| 对象 | MinIO `RELEASE.2025-04-22T22-12-26Z`、mc `RELEASE.2025-04-16T18-13-26Z`（版本和 `deploy/testenv-ci.sh` 相同），开启 SSE-S3 |
| 环境 | Frank 的开发机，从源码构建到临时目录，只监听 `127.0.0.1`。这是"不在本机安装软件"的一次例外，Frank 同意。跑完已删除 |
| 工具 | [tools/objstore-contract](../../tools/objstore-contract/README.md) |
| 结论 | **MinIO 必需项全部通过。云厂商没有验证**，上线前按 AC-FILE-90-2 跑同一套工具 |

## 发现

1. 预签名 PUT 能写入未签名的 `Content-Encoding`、`Content-Disposition`；不替换元数据的 Copy 会把它们和标签一起带到目标。**所以 F-FILE-01 规则 3 的元数据和标签替换是必要的**，替换后最终对象只剩服务端写入的值。
2. 未签名的 `x-amz-meta-*` 被 MinIO 拒绝（400）；`x-amz-acl`、`x-amz-website-redirect-location`、`x-amz-tagging` 返回 200，但没有存下（或查不到）。这是 MinIO 的行为，其他厂商可能不同。
3. 这次 SSE-S3 下 Copy 前后 ETag 相同。PRD 不依赖这一点，按 Copy 响应的 ETag 核对。

## 没有验证

- 云厂商的任何行为（CH-05 还没选）。
- 生命周期规则的实际清除：只验证了配置能写入和读回。
- CORS 按来源限制：MinIO 这次把请求的来源原样返回，说明没有限定来源，生产要在目标厂商上配置并确认。
- 第一次运行时，带恶意头的 PUT 被拒后脚本改写干净对象，元数据和标签两项是空测，已改成直接造带恶意元数据的源对象并加前提检查，下面是修正后的结果。

## 结果

| 级别 | 检查 | 结果 | 详情 |
|---|---|---|---|
| 必需 | 预签名 PUT 到临时键 | 通过 | HTTP 200 |
| 必需 | PUT 超过签入的 Content-Length 被拒绝 | 通过 | HTTP 403 |
| 必需 | PUT 小于签入的 Content-Length 被拒绝 | 通过 | HTTP 403 |
| 必需 | PUT 改 Content-Type 被拒绝 | 通过 | HTTP 403 |
| 必需 | 过期的预签名 PUT 被拒绝 | 通过 | HTTP 403 |
| 记录 | 预签名 PUT 带未签名的 Content-Encoding | 记录 | HTTP 200，存下的值="gzip" |
| 记录 | 预签名 PUT 带未签名的 Content-Disposition | 记录 | HTTP 200，存下的值="inline; filename=x.html" |
| 记录 | 预签名 PUT 带未签名的 x-amz-meta-evil | 记录 | HTTP 400，存下的值="" |
| 记录 | 预签名 PUT 带未签名的 x-amz-acl | 记录 | HTTP 200，存下的值="" |
| 记录 | 预签名 PUT 带未签名的 x-amz-website-redirect-location | 记录 | HTTP 200，存下的值="" |
| 记录 | 预签名 PUT 带未签名的 x-amz-tagging | 记录 | HTTP 200，存下的值="" |
| 必需 | 测试前提：源对象确实带恶意元数据和标签 | 通过 | PUT HTTP 200 Content-Encoding="gzip" meta="1" redirect="" |
| 记录 | 对照：不替换的 Copy 保留什么 | 记录 | HTTP 200 Content-Type="text/html" Content-Encoding="gzip" meta="1" 标签=true |
| 必需 | HEAD 返回大小和 ETag | 通过 | HTTP 200 |
| 记录 | 源对象服务端加密 | 记录 | x-amz-server-side-encryption=AES256 |
| 必需 | Range GET + If-Match 正确 | 通过 | HTTP 206 |
| 必需 | Range GET + If-Match 错误返回 412 | 通过 | HTTP 412 |
| 必需 | 条件 Copy：ETag 不符返回 412 | 通过 | HTTP 412 |
| 必需 | 条件 Copy 失败时目标不存在 | 通过 | HTTP 404 |
| 必需 | HEAD 后替换对象，条件 Copy 返回 412 | 通过 | HTTP 412 |
| 必需 | 条件 Copy + 元数据替换成功 | 通过 | HTTP 200 |
| 必需 | 最终对象只有服务端写入的元数据 | 通过 | HTTP 200 Content-Type="image/png" Content-Encoding="" Content-Disposition="" meta="" redirect="" |
| 必需 | Copy 响应的 ETag 和 HEAD 最终键一致 | 通过 |  |
| 记录 | Copy 前后 ETag 是否相同（PRD 不依赖） | 记录 | 相同=true |
| 记录 | 最终对象服务端加密 | 记录 | x-amz-server-side-encryption=AES256 |
| 必需 | 最终对象没有标签 | 通过 | HTTP 200 |
| 必需 | 覆盖临时键不影响最终键 | 通过 | HTTP 200 |
| 必需 | 预签名 GET 的 response-* 头生效 | 通过 | HTTP 200 Content-Disposition="attachment; filename=\"file\"; filename*=UTF-8''%E6%96%87%E4%BB%B6.txt" |
| 优先 | 响应带 nosniff | 通过 | X-Content-Type-Options=nosniff |
| 必需 | 改 response-content-type 后签名失效 | 通过 | HTTP 403 |
| 必需 | 过期的预签名 GET 被拒绝 | 通过 | HTTP 403 |
| 必需 | 无签名不能读私有对象 | 通过 | HTTP 403 |
| 必需 | 私有桶匿名列举被拒绝 | 通过 | HTTP 403 |
| 必需 | 服务端凭证按前缀列举 | 通过 | HTTP 200，1 个键 |
| 必需 | 服务端凭证全桶列举被拒绝 | 通过 | HTTP 403 |
| 必需 | 跨桶 Copy 到公开桶 | 通过 | HTTP 200 |
| 必需 | 公开对象匿名可读，类型是服务端写入的 | 通过 | HTTP 200 Content-Type="image/png" |
| 优先 | 公开对象带 nosniff | 通过 | X-Content-Type-Options=nosniff |
| 必需 | 公开桶匿名列举被拒绝 | 通过 | HTTP 403 |
| 必需 | 临时前缀生命周期规则可以配置并读回（没有等实际清除） | 通过 | HTTP 200 / 读回 HTTP 200 |
| 必需 | CORS 预检允许 PUT | 通过 | HTTP 204 Allow-Origin="https://app.example.com" |
| 记录 | CORS 能否限定来源 | 记录 | Allow-Origin=https://app.example.com（* 表示没有限定，生产要配成 App 的来源） |
| 必需 | 按前缀列举并逐个删除 | 通过 | 删 1 个，剩 0 个 |
| 必需 | 服务端凭证能删公开桶对象 | 通过 | HTTP 204 |
| 必需 | 重复删除幂等 | 通过 | HTTP 204 |
| 记录 | 服务端凭证读公开桶（PRD 不需要） | 记录 | HTTP 403 |

必需项失败 0 个，共 46 项
