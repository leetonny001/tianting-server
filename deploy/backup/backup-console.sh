#!/usr/bin/env bash
# 备份 console-server 的 Postgres（S-OPS-28）：pg_dump → 校验 → 加密 → 上传，清单格式和 im-server 相同，source=console。
# 配置见 lib.sh，另外：CONSOLE_PG_URL（只读备份账号）。
HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=deploy/backup/lib.sh
. "$HERE/lib.sh"
parse_args "$@"
: "${CONSOLE_PG_URL:?CONSOLE_PG_URL is required}"
PG_DUMP="${PG_DUMP:-pg_dump}"
PG_RESTORE="${PG_RESTORE:-pg_restore}"

start_job console
NAME="console-$(date -u +%Y%m%dT%H%M%SZ).dump"
SNAPSHOT="$WORK/$NAME"
"$PG_DUMP" --format=custom --file="$SNAPSHOT" "$CONSOLE_PG_URL" >/dev/null 2>&1 || fail_job "快照或校验失败"
"$PG_RESTORE" --list "$SNAPSHOT" >/dev/null 2>&1 || fail_job "快照或校验失败"
set_manifest '.verified = true'
finish_job "$SNAPSHOT" "$NAME"
