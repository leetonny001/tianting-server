# 内部 CA 和节点运维证书（S-OPS-30）

节点运维接口（`IM_OPS_ADDR`）绑定非回环地址时必须用 TLS。console-server 只信任这个内部 CA，并校验服务名和节点凭证表一致，校验失败时不发送请求（PRD-ops F-OPS-16）。

## 第一次

```bash
deploy/pki/issue-node-cert.sh ca /安全目录/ops-ca
deploy/pki/issue-node-cert.sh node /安全目录/ops-ca n1
```

- `ca.pem` 配给 console-server 的 `CON_OPS_CA_FILE`。
- `n1.pem`、`n1.key` 配给 n1 的 `IM_OPS_TLS_CERT`、`IM_OPS_TLS_KEY`。
- 节点凭证表里 n1 的 `tlsServerName` 写 `n1.ops.internal`。
- **`ca.key` 离线保存，不进仓库，不放在节点上。**

## 轮换节点证书

证书有效期 1 年。到期前重新签发同名证书，替换文件，重启该节点。console-server 不用动。

## 轮换 CA

1. 生成新 CA，把新旧两个 `ca.pem` 拼成一个文件配给 `CON_OPS_CA_FILE`，重启 console-server。
2. 逐个节点换成新 CA 签发的证书。
3. 从 `CON_OPS_CA_FILE` 去掉旧 CA，再重启 console-server。

## 节点签名私钥（和 TLS 证书无关）

每个节点一把 Ed25519，console-server 持有私钥，节点只配公钥。轮换：节点 `IM_OPS_PUBKEYS` 先配新旧两把并重启 → console-server 的节点凭证表换新 `keyFile` 并重启 → 节点去掉旧公钥并重启。
