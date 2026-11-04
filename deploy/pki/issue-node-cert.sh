#!/usr/bin/env bash
# 签发节点运维证书（S-OPS-30）。服务名 <node_id>.ops.internal，只用于 IM_OPS_ADDR 的 TLS。
# 用法：
#   issue-node-cert.sh ca <目录>                 创建内部 CA（私钥 ca.key 要离线保存，不进仓库）
#   issue-node-cert.sh node <目录> <node_id>     用目录里的 CA 签发节点证书，有效期 1 年
set -euo pipefail
umask 077

cmd="${1:?usage: issue-node-cert.sh ca|node <dir> [node_id]}"
dir="${2:?usage: issue-node-cert.sh ca|node <dir> [node_id]}"
mkdir -p "$dir"

case "$cmd" in
  ca)
    if [ -e "$dir/ca.key" ]; then
      echo "$dir/ca.key already exists: refusing to overwrite" >&2
      exit 1
    fi
    openssl genpkey -algorithm ed25519 -out "$dir/ca.key"
    openssl req -x509 -new -key "$dir/ca.key" -days 3650 -subj "/CN=Tianting Internal Ops CA" \
      -addext "basicConstraints=critical,CA:TRUE,pathlen:0" -addext "keyUsage=critical,keyCertSign,cRLSign" -out "$dir/ca.pem"
    chmod 644 "$dir/ca.pem"
    echo "CA: $dir/ca.pem（发给 console-server 的 CON_OPS_CA_FILE）；$dir/ca.key 离线保存"
    ;;
  node)
    node="${3:?usage: issue-node-cert.sh node <dir> <node_id>}"
    if ! printf '%s' "$node" | grep -Eq '^[A-Za-z0-9_-]{1,64}$'; then
      echo "invalid node id" >&2
      exit 2
    fi
    name="$node.ops.internal"
    work="$(mktemp -d)"
    trap 'find "${work:?}" -mindepth 1 -delete; rmdir "${work:?}"' EXIT
    openssl genpkey -algorithm ed25519 -out "$dir/$node.key"
    openssl req -new -key "$dir/$node.key" -subj "/CN=$name" -out "$work/req.csr"
    printf 'subjectAltName=DNS:%s\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\n' "$name" >"$work/ext"
    openssl x509 -req -in "$work/req.csr" -CA "$dir/ca.pem" -CAkey "$dir/ca.key" -CAcreateserial -days 365 -extfile "$work/ext" -out "$dir/$node.pem" 2>/dev/null
    chmod 644 "$dir/$node.pem"
    echo "节点证书：$dir/$node.pem、$dir/$node.key（IM_OPS_TLS_CERT、IM_OPS_TLS_KEY），服务名 $name"
    ;;
  *)
    echo "usage: issue-node-cert.sh ca|node <dir> [node_id]" >&2
    exit 2
    ;;
esac
