# 对象存储合同测试

检查一个 S3 兼容存储是否满足 [PRD-chat](../../docs/prd/) 第 21 节的必需能力（AC-FILE-90-1、90-2）。只用 Go 标准库。

## 本机跑 MinIO

Frank 2026-11-06 同意，技术验证门作为"不在本机安装软件"的一次例外：把固定版本的 MinIO 和 mc（版本见 `deploy/testenv-ci.sh`）从源码构建到临时目录，只监听 `127.0.0.1`，跑完删除。

```bash
MINIO_BIN_DIR=/path/with/minio-and-mc tools/objstore-contract/run-local-minio.sh
```

凭证每次随机生成，数据目录结束时删除。

## 跑云厂商（AC-FILE-90-2，上线前）

先由 Frank 建好两个桶和一个最小权限的服务端凭证，凭证放在环境变量里，不要写进文件或聊天。然后在 `tools/objstore-contract` 下：

| 环境变量 | 含义 |
|---|---|
| `OC_ENDPOINT`、`OC_REGION` | 服务地址、区域（只支持路径风格地址） |
| `OC_ACCESS_KEY`、`OC_SECRET_KEY` | 管理凭证，只用于配置生命周期规则 |
| `OC_SCOPED_ACCESS_KEY`、`OC_SCOPED_SECRET_KEY` | im-server 用的最小权限凭证 |
| `OC_BUCKET`、`OC_PUBLIC_BUCKET` | 私有桶、公开桶 |
| `OC_PUBLIC_BASE` | 可选，公开桶的独立域名 |

```bash
go run .
```

## 结果怎么看

- **必需**失败时退出码是 1，要改设计或换厂商。
- **优先**失败写进部署说明。
- **记录**只给实现参考。

**没有验证**：生命周期规则的实际清除（只查配置能写入和读回）；CORS 限定来源（MinIO 这个版本未必支持按桶配置，生产要在目标厂商上确认）。

**权限说明**：`run-local-minio.sh` 给服务端凭证的 `tmp/` 列举权限只用于这个工具清理自己的对象，PRD 第 11.3 节不给 im-server。
