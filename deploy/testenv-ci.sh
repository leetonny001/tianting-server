#!/usr/bin/env bash
# 真实工具的验证（CI 的 ops-deploy 任务）：promtool、amtool、MinIO + 真实 age 的备份、Grafana 的认证代理和只读面板。需要 docker。
# 开发机不装这些软件（backend-ops.md 的环境依赖）。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
OBS="$ROOT/observability"
WORK="$(mktemp -d)"

echo "== promtool"
docker run --rm -v "$OBS/prometheus:/rules:ro" -w /rules --entrypoint promtool prom/prometheus:v3.5.0 check rules alerts.yml
docker run --rm -v "$OBS/prometheus:/rules:ro" -w /rules --entrypoint promtool prom/prometheus:v3.5.0 test rules alerts_test.yml
mkdir -p "$WORK/prom/targets"
cp "$OBS/prometheus/prometheus.yml" "$OBS/prometheus/alerts.yml" "$WORK/prom/"
echo '[]' >"$WORK/prom/targets/im.json"
docker run --rm -v "$WORK/prom:/cfg:ro" -w /cfg --entrypoint promtool prom/prometheus:v3.5.0 check config prometheus.yml

echo "== amtool"
mkdir -p "$WORK/am"
cp -R "$OBS/alertmanager/." "$WORK/am/"
docker run --rm -v "$WORK/am:/etc/alertmanager:ro" --entrypoint amtool prom/alertmanager:v0.28.1 check-config /etc/alertmanager/alertmanager.yml

echo "== MinIO + age: backup"
root_pw="$(openssl rand -hex 16)"
writer_pw="$(openssl rand -hex 16)"
reader_pw="$(openssl rand -hex 16)"
# MinIO 不再发布公开的镜像和二进制：从固定版本的源码构建。MINIO_BIN_DIR 由 CI 缓存。
BIN="${MINIO_BIN_DIR:-$WORK/minio-bin}"
mkdir -p "$BIN"
build_from_source() { # name repo tag
  [ -x "$BIN/$1" ] && return 0
  git clone -q --depth 1 --branch "$3" "$2" "$WORK/src-$1"
  (cd "$WORK/src-$1" && CGO_ENABLED=0 go build -o "$BIN/$1" .)
}
build_from_source minio https://github.com/minio/minio RELEASE.2025-04-22T22-12-26Z
build_from_source mc https://github.com/minio/mc RELEASE.2025-04-16T18-13-26Z
MINIO_ROOT_USER=root MINIO_ROOT_PASSWORD="$root_pw" "$BIN/minio" server --quiet --address 127.0.0.1:19100 "$WORK/minio-data" >/dev/null 2>&1 &
MINIO_PID=$!
trap 'kill "$MINIO_PID" 2>/dev/null || true; docker rm -f tt-grafana >/dev/null 2>&1 || true' EXIT
for _ in $(seq 1 60); do curl -fs http://127.0.0.1:19100/minio/health/live >/dev/null && break; sleep 1; done
MC="$BIN/mc"
export MC_CONFIG_DIR="$WORK/mcconfig"
"$MC" alias set --quiet root http://127.0.0.1:19100 root "$root_pw" >/dev/null
"$MC" mb --quiet root/tianting-backups >/dev/null
"$MC" admin policy create root writer "$ROOT/backup/policy-backup-writer.json" >/dev/null
"$MC" admin policy create root reader "$ROOT/backup/policy-console-reader.json" >/dev/null
"$MC" admin user add root bwriter "$writer_pw" >/dev/null
"$MC" admin user add root creader "$reader_pw" >/dev/null
"$MC" admin policy attach root writer --user bwriter >/dev/null
"$MC" admin policy attach root reader --user creader >/dev/null
"$MC" alias set --quiet backup http://127.0.0.1:19100 bwriter "$writer_pw" >/dev/null
"$MC" alias set --quiet console http://127.0.0.1:19100 creader "$reader_pw" >/dev/null

age-keygen -o "$WORK/age.key" 2>/dev/null
RECIPIENT="$(age-keygen -y "$WORK/age.key")"
(cd "$ROOT/../server" && go build -o "$WORK/im-server" ./cmd/im-server)
IM_DB="$WORK/im.db" "$WORK/im-server" migrate up >/dev/null
mkdir -p "$WORK/textfile"
export MC IM_DB="$WORK/im.db" IM_SERVER_BIN="$WORK/im-server" AGE_RECIPIENT="$RECIPIENT" BACKUP_REMOTE=backup/tianting-backups TEXTFILE_DIR="$WORK/textfile" BACKUP_WORK_DIR="$WORK/backup-work"
"$ROOT/backup/backup.sh"
KEY="$("$MC" ls --recursive --quiet root/tianting-backups/manifests/im/ | awk '{print $NF}' | head -1)"
"$MC" cat "root/tianting-backups/manifests/im/$KEY" >"$WORK/manifest.json"
jq -e '.result == "success" and .verified and .encrypted and .uploaded and .source == "im"' "$WORK/manifest.json" >/dev/null
FILE="$(jq -r .file "$WORK/manifest.json")"
"$MC" cp --quiet "root/tianting-backups/backups/im/$FILE" "$WORK/obj" >/dev/null
[ "$(sha256sum "$WORK/obj" | cut -d' ' -f1)" = "$(jq -r .sha256 "$WORK/manifest.json")" ] || { echo "sha256 mismatch"; exit 1; }
# 真实 age 能解开，解开后是能打开的 SQLite，而且对象本身不是明文。
if head -c 16 "$WORK/obj" | grep -q "SQLite format"; then echo "object is not encrypted"; exit 1; fi
age -d -i "$WORK/age.key" -o "$WORK/restored.db" "$WORK/obj"
[ "$(sqlite3 "$WORK/restored.db" 'PRAGMA integrity_check;')" = ok ] || { echo "restored database is broken"; exit 1; }
echo "backup: object, hash, encryption ok"

# 加密失败：有失败清单，成功时间不变（AC-OPS-12-2）。
BEFORE="$(grep success "$WORK/textfile/im_backup_im.prom")"
sleep 1
if AGE_RECIPIENT= "$ROOT/backup/backup.sh" 2>/dev/null; then echo "empty recipient should fail"; exit 1; fi
[ "$(grep success "$WORK/textfile/im_backup_im.prom")" = "$BEFORE" ] || { echo "success time changed"; exit 1; }
"$MC" ls --recursive --quiet root/tianting-backups/manifests/im/ | wc -l | grep -q 2 || { echo "no failure manifest"; exit 1; }

# console-server 的凭证（AC-OPS-12-5）：能读清单；不能读 backups/，不能写，不能删。
"$MC" cat "console/tianting-backups/manifests/im/$KEY" >/dev/null
if "$MC" cat "console/tianting-backups/backups/im/$FILE" >/dev/null 2>&1; then echo "reader read backups/"; exit 1; fi
echo x >"$WORK/x"
if "$MC" cp --quiet "$WORK/x" console/tianting-backups/manifests/x >/dev/null 2>&1; then echo "reader wrote"; exit 1; fi
if "$MC" rm --quiet "console/tianting-backups/manifests/im/$KEY" >/dev/null 2>&1; then echo "reader deleted"; exit 1; fi
# 备份凭证读不了备份（加密后的也不需要读）。
if "$MC" cat "backup/tianting-backups/backups/im/$FILE" >/dev/null 2>&1; then echo "writer read backups/"; exit 1; fi
echo "policies ok"

echo "== Grafana"
mkdir -p "$WORK/grafana"
sed 's/^http_addr = .*/http_addr = 0.0.0.0/; s/^whitelist = .*/whitelist = 172.16.0.0\/12/; s/^cookie_secure = .*/cookie_secure = false/' "$OBS/grafana/grafana.ini" >"$WORK/grafana/grafana.ini"
docker run -d --name tt-grafana -p 13000:3000 \
  -v "$WORK/grafana/grafana.ini:/etc/grafana/grafana.ini:ro" \
  -v "$OBS/grafana/provisioning:/etc/grafana/provisioning:ro" \
  -v "$OBS/grafana/dashboards:/etc/grafana/dashboards:ro" grafana/grafana:11.6.0 >/dev/null
G=http://127.0.0.1:13000
ok=""
for _ in $(seq 1 90); do
  if curl -fs -H 'X-WEBAUTH-USER: oncall' "$G/api/dashboards/uid/tianting-overview" -o "$WORK/exported.json"; then ok=1; break; fi
  sleep 1
done
[ -n "$ok" ] || { docker logs tt-grafana | tail -30; echo "dashboard was not imported"; exit 1; }
# 导入后规整比对（AC-OPS-06-1）：面板 14 个，标题一致。
jq -e '.dashboard.panels | length == 14' "$WORK/exported.json" >/dev/null
diff <(jq -S '[.dashboard.panels[].title]' "$WORK/exported.json") <(jq -S '[.panels[].title]' "$OBS/grafana/dashboards/tianting.json")
# 代理用户是 Viewer，保存面板和数据源管理被拒（AC-OPS-17-1）。
ROLE="$(curl -fs -H 'X-WEBAUTH-USER: oncall' "$G/api/user/orgs" | jq -r '.[0].role')"
[ "$ROLE" = Viewer ] || { echo "role is $ROLE"; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
[ "$(code -X POST -H 'X-WEBAUTH-USER: oncall' -H 'Content-Type: application/json' -d '{"dashboard":{"title":"x"}}' "$G/api/dashboards/db")" = 403 ] || { echo "viewer saved a dashboard"; exit 1; }
[ "$(code -X POST -H 'X-WEBAUTH-USER: oncall' -H 'Content-Type: application/json' -d '{"name":"x","type":"prometheus","url":"http://x","access":"proxy"}' "$G/api/datasources")" = 403 ] || { echo "viewer created a datasource"; exit 1; }
# 没有用户头：不能匿名进。
[ "$(code "$G/api/dashboards/uid/tianting-overview")" = 401 ] || { echo "anonymous access"; exit 1; }
echo "grafana ok"
echo "== all deploy checks passed"
