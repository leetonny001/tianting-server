#!/bin/sh
# 建桶和两份最小权限凭证（策略文件和生产一样）。可以重复运行。
set -eu
mc alias set --quiet root http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
mc mb --quiet --ignore-existing root/tianting-backups >/dev/null
mc admin policy create root writer /app/writer.json >/dev/null
mc admin policy create root reader /app/reader.json >/dev/null
mc admin user add root backup-writer "$BACKUP_WRITER_SECRET" >/dev/null
mc admin user add root console-reader "$CONSOLE_READER_SECRET" >/dev/null
mc admin policy attach root writer --user backup-writer >/dev/null 2>&1 || true
mc admin policy attach root reader --user console-reader >/dev/null 2>&1 || true

# 自检：备份凭证能写清单，console 凭证能读，但不能写。
mc alias set --quiet writer http://minio:9000 backup-writer "$BACKUP_WRITER_SECRET" >/dev/null
mc alias set --quiet reader http://minio:9000 console-reader "$CONSOLE_READER_SECRET" >/dev/null
key=manifests/im/0000000000000.json
echo '{"selfcheck":true}' | mc pipe --quiet "writer/tianting-backups/$key" >/dev/null
mc cat "reader/tianting-backups/$key" >/dev/null
if echo x | mc pipe --quiet reader/tianting-backups/manifests/x >/dev/null 2>&1; then
  echo "console reader can write" >&2
  exit 1
fi
mc rm --quiet "root/tianting-backups/$key" >/dev/null
echo "minio ready"
