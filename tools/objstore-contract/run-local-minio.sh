#!/usr/bin/env bash
# 用已经构建好的 MinIO 跑合同测试。不安装任何东西；数据和配置都在临时目录，结束时删除。
# 用法：MINIO_BIN_DIR=/path/with/minio-and-mc tools/objstore-contract/run-local-minio.sh
set -euo pipefail
BIN="${MINIO_BIN_DIR:?需要 MINIO_BIN_DIR，里面有 minio 和 mc}"
PORT="${MINIO_PORT:-19110}"
HERE="$(cd "$(dirname "$0")" && pwd)"
if lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "端口 $PORT 已被占用" >&2
  exit 2
fi
WORK="$(mktemp -d)"
ROOT_USER="root$(openssl rand -hex 4)"
ROOT_PW="$(openssl rand -hex 16)"
SCOPED_USER="svc$(openssl rand -hex 4)"
SCOPED_PW="$(openssl rand -hex 16)"
KMS_KEY="gate-key:$(openssl rand -base64 32)"
cleanup() {
  [ -n "${MINIO_PID:-}" ] && kill "$MINIO_PID" 2>/dev/null || true
  [ -n "${MINIO_PID:-}" ] && wait "$MINIO_PID" 2>/dev/null || true
  find "${WORK:?}" -delete 2>/dev/null || true
}
trap cleanup EXIT

MINIO_ROOT_USER="$ROOT_USER" MINIO_ROOT_PASSWORD="$ROOT_PW" MINIO_KMS_SECRET_KEY="$KMS_KEY" \
  "$BIN/minio" server --quiet --address "127.0.0.1:$PORT" --console-address "127.0.0.1:$((PORT + 1))" "$WORK/data" >"$WORK/minio.log" 2>&1 &
MINIO_PID=$!
for _ in $(seq 1 60); do
  curl -fs "http://127.0.0.1:$PORT/minio/health/live" >/dev/null && break
  sleep 1
done

export MC_CONFIG_DIR="$WORK/mc"
export MC_HOST_gate="http://$ROOT_USER:$ROOT_PW@127.0.0.1:$PORT"
mc() { "$BIN/mc" --config-dir "$WORK/mc" --quiet "$@"; }

mc mb gate/chat-private >/dev/null
mc mb gate/chat-public >/dev/null
mc encrypt set sse-s3 gate/chat-private >/dev/null

# 公开桶：匿名只能读对象，不能列举（不用 mc anonymous set download，它会放开列举）
cat >"$WORK/public.json" <<'EOF'
{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::chat-public/*"]}]}
EOF
mc anonymous set-json "$WORK/public.json" gate/chat-public >/dev/null

# 服务端凭证：PRD 第 11.3 节的最小权限
cat >"$WORK/scoped.json" <<'EOF'
{"Version":"2012-10-17","Statement":[
{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject","s3:DeleteObject","s3:GetObjectTagging"],"Resource":["arn:aws:s3:::chat-private/tmp/*","arn:aws:s3:::chat-private/final/*"]},
{"Effect":"Allow","Action":["s3:ListBucket"],"Resource":["arn:aws:s3:::chat-private"],"Condition":{"StringLike":{"s3:prefix":["tmp/*","final/*"]}}},
{"Effect":"Allow","Action":["s3:PutObject","s3:DeleteObject"],"Resource":["arn:aws:s3:::chat-public/*"]}
]}
EOF
mc admin user add gate "$SCOPED_USER" "$SCOPED_PW" >/dev/null
mc admin policy create gate im-server "$WORK/scoped.json" >/dev/null
mc admin policy attach gate im-server --user "$SCOPED_USER" >/dev/null

cd "$HERE"
OC_ENDPOINT="http://127.0.0.1:$PORT" OC_REGION=us-east-1 \
OC_ACCESS_KEY="$ROOT_USER" OC_SECRET_KEY="$ROOT_PW" \
OC_SCOPED_ACCESS_KEY="$SCOPED_USER" OC_SCOPED_SECRET_KEY="$SCOPED_PW" \
OC_BUCKET=chat-private OC_PUBLIC_BUCKET=chat-public \
  go run .

# 可选：把 im-server 的实现（objstore 客户端和 files 流程）也跑到这个 MinIO 上（AC-FILE-90-1 的实现部分，BE-439）。
# 用最小权限的服务端凭证；凭证只在环境变量里，不打印。
if [ "${RUN_IM_E2E:-0}" = 1 ]; then
  cd "$HERE/../../server"
  IM_E2E_S3_ENDPOINT="http://127.0.0.1:$PORT" IM_E2E_S3_REGION=us-east-1 \
  IM_E2E_S3_BUCKET=chat-private IM_E2E_S3_PUBLIC_BUCKET=chat-public \
  IM_E2E_S3_ACCESS_KEY="$SCOPED_USER" IM_E2E_S3_SECRET_KEY="$SCOPED_PW" \
    go test -count=1 -v -run 'TestTC_FILE_90_01_RealStore' ./e2e/
fi
