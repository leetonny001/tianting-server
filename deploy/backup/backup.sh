#!/usr/bin/env bash
# 备份 im-server（S-OPS-28）：快照并校验 → age 加密 → 上传 backups/ → 写 manifests/ → 更新 textfile 指标。
# systemd timer 每小时一次；发布前由 deploy/release.sh 调用（--trigger pre-migrate --release 版本）。
# 配置见 lib.sh，另外：IM_SERVER_BIN（默认 im-server）、IM_DB。
HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=deploy/backup/lib.sh
. "$HERE/lib.sh"
parse_args "$@"
IM_SERVER_BIN="${IM_SERVER_BIN:-im-server}"
: "${IM_DB:?IM_DB is required}"

start_job im
NAME="im-$(date -u +%Y%m%dT%H%M%SZ).db"
SNAPSHOT="$WORK/$NAME"
RAW="$WORK/raw-manifest.json"
"$IM_SERVER_BIN" backup -db "$IM_DB" -o "$SNAPSHOT" -manifest "$RAW" -trigger "$TRIGGER" >/dev/null 2>&1 || fail_job "快照或校验失败"
# im-server 校验过快照：把校验结果和迁移版本带过来。
set_manifest --slurpfile raw "$RAW" '.verified = $raw[0].verified | .migrationVersion = $raw[0].migrationVersion'
finish_job "$SNAPSHOT" "$NAME"
