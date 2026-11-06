# 账号功能部署和运维手册（PRD-chat 第 12.2 节，BE-423）

只写账号功能（C1）多出来的部分。数字都未校准。

## 1. 部署配置

| 配置 | 说明 |
|---|---|
| `IM_REGISTER_MODE` | `closed` / `invite` / `open`，默认 `invite`。生产环境 `open` 拒绝启动（C6 上线前一律拒绝） |
| `IM_REFRESH_KEYS`、`IM_REFRESH_ACTIVE_KID` | refresh 密钥，独立于 JWT 密钥。没配时账号接口 503 / 5030，其他功能不受影响 |
| `IM_REFRESH_OVERLAP` | 默认 2 分钟，必须大于 60 秒（SDK 超时 30 秒 + 网关超时 30 秒），否则配置错误 |
| `IM_ALLOW_TOKEN_ISSUE` | 生产环境有调用方带 `token.issue` 时必须设为 1 才能启动，配置检查一直显示 `token_issue_caller` 告警。这个权限能以任何用户身份登录，没有业务方后应该去掉 |
| `IM_TRUSTED_EGRESS_CIDRS`、`IM_TRUSTED_EGRESS_FACTOR` | 见第 5 节 |
| `IM_INVITE_WARN_TOTAL` | 默认 500，有效邀请码剩余次数合计超过它时配置检查告警 `invite_total` |
| `IM_SESSION_RECHECK` | 在线连接的周期复查，默认 10 分钟 |

**`GOMEMLIMIT` 要把 Argon2 算进去**：每次计算 64 MiB。`IM_ARGON_SLOTS` 默认 4 个，加改密、`recover` 各 1 个专用位，同时最多 6 个，约 384 MiB。

**网关不能给 `/v1/` 的响应加 `WWW-Authenticate`。** im-server 不发这个头，要确认的是网关和它前面的代理（比如统一鉴权插件）。原因：

- URLSession 看到带这个头的 401，会把同一个 POST 再发一次（macOS、回环、HTTP/1.1 上核实过，其他组合没有测）。refresh 的 4019、改密的 4018 都是 401，refresh 被重发可能就是重放。
- 所以 sdk-swift 遇到 HTTP 认证质询直接取消请求，按网络错误处理，保证只发 1 次。代价是读不到响应体：有这个头时，iOS 上会话失效收不到 4019，一直退避，回不到登录页；改密输错旧密码也只显示网络错误。

部署后检查（假的 refresh，不用真实凭证），应该看到 401 和 JSON 响应体，没有这个头：

```sh
curl -si -X POST https://网关地址/v1/account/token/refresh -H 'Content-Type: application/json' -d '{"refreshToken":"x"}' | grep -i '^www-authenticate' && echo 不合格 || echo 合格
```

## 2. 配置检查和指标

`GET /internal/v1/config` 的 `productionChecks`：

| 检查项 | 含义 |
|---|---|
| `register_mode` | 注册模式 |
| `refresh_keys` | refresh 密钥和其他密钥不同 |
| `refresh_key_ledger` | 第 3 节的台账 |
| `invite_total` | 邀请码总量告警（只告警） |
| `token_issue_caller` | `token.issue` 放行告警（只告警） |

隐私说明版本在配置摘要里（`IM_PRIVACY_NOTICE_VERSION`）；C4、C6 打开后为空时拒绝启动。

指标：`im_account_logins_total{result}`、`im_registers_total{result}`、`im_refresh_reissue_total`、`im_refresh_replay_total`、`im_invites_auto_disabled_total`。标签没有无界字段。

建议的告警规则（第一批默认不发页面告警，先看值）：

- `rate(im_refresh_replay_total[10m])` 突增：可能是令牌被盗，也可能是弱网下迟到的重复请求（PRD F-ACC-03 规则 4 的代价 ③，频率未测）。
- `rate(im_account_logins_total{result="invalid"}[10m])` 突增：撞库。
- `im_invites_auto_disabled_total` 增加：要通知到人，见第 5 节。

## 3. refresh 密钥台账（F-ACC-03 规则 3，RS5-04）

台账在数据库 `refresh_key_ledger`，只有 `kid` 和时间，没有密钥。启动时自动同步。

**退役条件是 `IM_REFRESH_ABSOLUTE`（365 天），不是 JWT 的 30 天。** refresh token 活得比 JWT 长得多，提前删除密钥会让持它签发的 refresh token 全部变成 `invalid`，等于全员下线。

正常轮换：

1. `IM_REFRESH_KEYS` 增加 k2，`IM_REFRESH_ACTIVE_KID=k2`，重启。k1 进台账，`retired_at` 记下这个时间。
2. **满 365 天后**才能从配置删除 k1。提前删除：配置检查 `refresh_key_ledger` 报错，生产配置拒绝启动。

### 密钥泄露的应急操作

拿到 refresh 密钥的人能伪造任何一代的 refresh token（但还要知道 `sid`），这时要主动提前删除并接受全员重新登录：

1. 配置增加新密钥并设为当前，从配置删除泄露的密钥。
2. `im-server refresh-keys -db FILE forget KID`：在台账里明确放弃这把密钥，配置检查不再拦。
3. 启动。持这把密钥的 refresh token 全部返回 4019（`reason=invalid`）。
4. 再运行第 4 节的 `sessions revoke-all`，并排查泄露原因。

## 4. 数据库恢复（RS4-14）

恢复到旧时间点后：

- 轮换过的客户端手里的值比库里新，会被当成重放；
- 已吊销的会话和已改的密码回到旧状态，包括受害者刚做的 `recover`。

步骤：

1. 恢复数据库，**启动前**运行 `im-server sessions -db FILE revoke-all`：吊销全部设备会话（`reason=revoked`，不计重放），用户重新登录。
2. 启动。在线连接由周期复查踢掉（如果服务没停）。
3. **告知恢复点之后改过密码、找回过账号的用户重新确认**：他们的密码和恢复码回到了旧状态。

`revoke-all` 不改密码、不改恢复码。

## 5. NAT 和可信出口网段（RS4-15、RS7）

内部小范围常见同一出口 IP。按 IP 的限制全公司共用：

| 限制 | 默认 | 后果 |
|---|---|---|
| 注册 | 每 IP 每小时 20 次 | 同一办公室集中注册会 429 |
| 登录和校验密码 | 每 IP 每分钟 20 次 | 同上 |
| 用户名 + IP 失败 | 15 分钟 5 次，锁 15 分钟 | **同办公室的人可以用"用户名 + 同一 IP"锁住同事的登录** |
| `recover` | 每 IP 每分钟 20 次 | **同网的人可以短时压制找回，换移动网络可以绕开** |
| `token/refresh` | 每 IP 每分钟 60 次 | |

可信出口网段可以配更高额度：`IM_TRUSTED_EGRESS_CIDRS=203.0.113.0/24`（逗号分隔，PRD 没有定义这个名字，BE-423 加的），`IM_TRUSTED_EGRESS_FACTOR` 是额度倍数，默认 10。只放大按 IP 的总量限制，**不放大"用户名 + IP"的失败次数**。IP 取值走 `IM_TRUSTED_PROXIES`。

**邀请码自动停用**：同一邀请码 24 小时内撞 20 次已占用的用户名会自动停用（`im_invites_auto_disabled_total`，日志只有哈希）。持有大额邀请码的人可以故意触发。**第一批不做重新启用，用新建邀请码恢复。**

## 6. 没有的东西

- **没有任何后台接口能改用户密码或恢复码**。恢复码丢失就是账号无法找回（F-ACC-05 规则 4）。
- `recover` 专用位的公平调度按单个实例生效，集群时随 B6 重新看。
- 周期复查的"总线重连后复查"第一批没有总线，入口是 `Gateway.RecheckAll`，随 B6 接上。
