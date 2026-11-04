# 备份任务的公共部分（S-OPS-28）。im-server 和 console-server 的备份写同格式清单，发布前备份走同一段代码。
# 配置来自环境变量：
#   AGE_RECIPIENT       age 公钥（必填）
#   BACKUP_REMOTE       对象存储位置，mc 的写法，如 backup/tianting-backups（必填）
#   TEXTFILE_DIR        node_exporter 的 textfile 目录（必填）
#   BACKUP_WORK_DIR     临时目录，默认 /var/lib/tianting/backup-work
#   BACKUP_RETAIN       对象存储保留份数，默认 48
#   MC                  mc 命令，默认 mc

set -euo pipefail

MC="${MC:-mc}"
BACKUP_RETAIN="${BACKUP_RETAIN:-48}"
WORK_ROOT="${BACKUP_WORK_DIR:-/var/lib/tianting/backup-work}"

TRIGGER=schedule
RELEASE=""
SOURCE=""
MANIFEST=""
WORK=""

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --trigger) TRIGGER="${2:?}"; shift 2 ;;
      --release) RELEASE="${2:?}"; shift 2 ;;
      *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
  done
  case "$TRIGGER" in
    schedule | pre-migrate) ;;
    *) echo "--trigger must be schedule or pre-migrate" >&2; exit 2 ;;
  esac
}

now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

cleanup_work() {
  if [ -n "${WORK:-}" ] && [ -d "$WORK" ]; then
    find "$WORK" -mindepth 1 -delete
    rmdir "$WORK"
  fi
}

start_job() {
  SOURCE="$1"
  : "${AGE_RECIPIENT:=}"
  : "${BACKUP_REMOTE:?BACKUP_REMOTE is required}"
  : "${TEXTFILE_DIR:?TEXTFILE_DIR is required}"
  mkdir -p "$WORK_ROOT"
  WORK="$(mktemp -d "$WORK_ROOT/$SOURCE.XXXXXX")"
  chmod 700 "$WORK"
  # 明文快照只在这个目录里，退出时一定删掉。
  trap cleanup_work EXIT
  STARTED_AT="$(now_ms)"
  MANIFEST="$WORK/manifest.json"
  jq -n --arg source "$SOURCE" --arg trigger "$TRIGGER" --arg release "$RELEASE" --argjson startedAt "$STARTED_AT" --arg host "$(hostname)" \
    '{source: $source, trigger: $trigger, startedAt: $startedAt, host: $host, file: "", size: 0, sha256: "", verified: false, encrypted: false, uploaded: false, migrationVersion: 0, result: "failure"}
     + (if $release != "" then {release: $release} else {} end)' >"$MANIFEST"
  metric_attempt
}

# 改清单的字段。
set_manifest() {
  local tmp="$MANIFEST.tmp"
  jq "$@" "$MANIFEST" >"$tmp"
  mv "$tmp" "$MANIFEST"
}

# 对象键约定：manifests/{source}/{finishedAt 毫秒，补零到 13 位}.json。console-server 按这个取最近 48 条。
upload_manifest() {
  local finished key
  finished="$(jq -r .finishedAt "$MANIFEST")"
  key="$(printf 'manifests/%s/%013d.json' "$SOURCE" "$finished")"
  "$MC" cp --quiet "$MANIFEST" "$BACKUP_REMOTE/$key" >/dev/null
}

# 失败：写 result=failure 清单，不更新成功时间戳，非 0 退出。原因是固定文字：不含路径和凭证。
fail_job() {
  local reason="$1"
  set_manifest --arg error "$reason" --argjson finishedAt "$(now_ms)" '.result = "failure" | .error = $error | .finishedAt = $finishedAt'
  upload_manifest || echo "manifest upload failed too" >&2
  echo "backup failed: $reason" >&2
  exit 1
}

# 加密 → 上传 → 清单 → 指标。snapshot 是明文文件，调用方已经校验过。明文在退出时随工作目录删除。
finish_job() {
  local snapshot="$1" name="$2" encrypted size sha finished
  encrypted="$WORK/$name.age"
  [ -n "$AGE_RECIPIENT" ] || fail_job "加密失败"
  age -r "$AGE_RECIPIENT" -o "$encrypted" "$snapshot" 2>/dev/null || fail_job "加密失败"
  size="$(wc -c <"$encrypted" | tr -d ' ')"
  sha="$(shasum -a 256 "$encrypted" 2>/dev/null | cut -d' ' -f1 || sha256sum "$encrypted" | cut -d' ' -f1)"
  # 清单里的大小和哈希是对象存储里那个文件的（AC-OPS-12-1）。
  set_manifest --arg file "$name.age" --argjson size "$size" --arg sha "$sha" '.file = $file | .size = $size | .sha256 = $sha | .encrypted = true'
  "$MC" cp --quiet "$encrypted" "$BACKUP_REMOTE/backups/$SOURCE/$name.age" >/dev/null 2>&1 || fail_job "上传失败"
  finished="$(now_ms)"
  set_manifest --argjson finishedAt "$finished" '.uploaded = true | .result = "success" | .finishedAt = $finishedAt'
  upload_manifest || fail_job "上传失败"
  metric_success "$finished" "$size"
  prune
  echo "backup ok: $SOURCE $name.age"
}

# textfile 指标：原子替换，只有全部成功才更新成功时间。其他 trigger 的时间保留。
metric_file() { echo "$TEXTFILE_DIR/im_backup_$SOURCE.prom"; }

write_metrics() {
  local attempt="$1" size="$2" schedule="$3" premigrate="$4" tmp
  tmp="$(metric_file).tmp.$$"
  {
    echo "im_backup_last_attempt_timestamp_seconds{source=\"$SOURCE\"} $attempt"
    [ -z "$size" ] || echo "im_backup_last_size_bytes{source=\"$SOURCE\"} $size"
    [ -z "$schedule" ] || echo "im_backup_last_success_timestamp_seconds{source=\"$SOURCE\",trigger=\"schedule\"} $schedule"
    [ -z "$premigrate" ] || echo "im_backup_last_success_timestamp_seconds{source=\"$SOURCE\",trigger=\"pre-migrate\"} $premigrate"
  } >"$tmp"
  mv "$tmp" "$(metric_file)"
}

# 读旧值。键放进变量再传：macOS 自带的 bash 3.2 处理 $( ) 里嵌套的转义引号有问题，读出来是空的，旧的成功时间就丢了。
metric_value() {
  [ -f "$(metric_file)" ] || return 0
  grep -F "$1 " "$(metric_file)" | cut -d' ' -f2 || true
}

load_metrics() {
  local key
  key="im_backup_last_size_bytes{source=\"$SOURCE\"}"
  OLD_SIZE="$(metric_value "$key")"
  key="im_backup_last_success_timestamp_seconds{source=\"$SOURCE\",trigger=\"schedule\"}"
  OLD_SCHEDULE="$(metric_value "$key")"
  key="im_backup_last_success_timestamp_seconds{source=\"$SOURCE\",trigger=\"pre-migrate\"}"
  OLD_PREMIGRATE="$(metric_value "$key")"
}

metric_attempt() {
  load_metrics
  write_metrics "$(python3 -c 'import time; print(int(time.time()))')" "$OLD_SIZE" "$OLD_SCHEDULE" "$OLD_PREMIGRATE"
}

metric_success() {
  local finished_ms="$1" size="$2" seconds
  seconds="$(python3 -c "print($finished_ms/1000)")"
  load_metrics
  if [ "$TRIGGER" = schedule ]; then OLD_SCHEDULE="$seconds"; else OLD_PREMIGRATE="$seconds"; fi
  write_metrics "$seconds" "$size" "$OLD_SCHEDULE" "$OLD_PREMIGRATE"
}

# 只保留最近 N 份备份。清单留着：页面只读最近 48 条，旧清单由桶的生命周期规则清理。
# 失败不影响这次备份的结论。
prune() {
  local names count excess
  names="$("$MC" ls --quiet "$BACKUP_REMOTE/backups/$SOURCE/" 2>/dev/null | awk '{print $NF}' | sort || true)"
  [ -n "$names" ] || return 0
  count="$(printf '%s\n' "$names" | wc -l | tr -d ' ')"
  excess=$((count - BACKUP_RETAIN))
  [ "$excess" -gt 0 ] || return 0
  printf '%s\n' "$names" | head -n "$excess" | while read -r old; do
    "$MC" rm --quiet "$BACKUP_REMOTE/backups/$SOURCE/$old" >/dev/null 2>&1 || true
  done
}
