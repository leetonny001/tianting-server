#!/usr/bin/env bash
# 发布（S-OPS-29）：有待执行的迁移时先备份，备份失败就停下，不执行迁移、不切换二进制。
# 用法：release.sh <新二进制> <版本>。
# 环境变量：IM_BIN（当前二进制路径，默认 /usr/local/bin/im-server）、IM_DB、RESTART_CMD（默认 systemctl restart im-server）。备份的配置见 backup/lib.sh。
set -euo pipefail

NEW="${1:?usage: release.sh <new binary> <version>}"
VERSION="${2:?usage: release.sh <new binary> <version>}"
HERE="$(cd "$(dirname "$0")" && pwd)"
IM_BIN="${IM_BIN:-/usr/local/bin/im-server}"
RESTART_CMD="${RESTART_CMD:-systemctl restart im-server}"
: "${IM_DB:?IM_DB is required}"

# 用新二进制判断：它带着新迁移。
STATUS="$("$NEW" migrate status)"
PENDING="$(printf '%s\n' "$STATUS" | grep -c ' pending' || true)"

if [ "$PENDING" -gt 0 ]; then
  echo "有 $PENDING 个待执行的迁移，先备份"
  # 和定时备份完全相同的脚本。用旧二进制做快照：新二进制的校验可能认不出还没迁移的库。
  IM_SERVER_BIN="$IM_BIN" "$HERE/backup/backup.sh" --trigger pre-migrate --release "$VERSION"
  "$NEW" migrate up
else
  echo "无待执行迁移，跳过发布前备份"
fi

# 原子替换，再重启。
cp "$NEW" "$IM_BIN.new"
chmod 755 "$IM_BIN.new"
mv "$IM_BIN.new" "$IM_BIN"
$RESTART_CMD
echo "发布完成：$VERSION"
