#!/usr/bin/env bash
# 环境验收（BE-320）：真实的 Prometheus、Alertmanager、Grafana、node_exporter、MinIO 和真实的 im-server 容器。约 15 分钟。
# 覆盖 AC-OPS-07-2、07-3、07-6（通知内容）、90-5（不停 IM、只停观测组件）、04-6 和 08-11 的节点侧部分。
# 加 --console 时也启动 console-server。CI 上没有 --console：console-server 仓库还没有远端（PENDING P-9）。
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
TT="$HERE/tt"
CONSOLE="${1:-}"
LIMIT=180 # 这几条 AC 的上限都是 3 分钟
WORK="$(mktemp -d)"

cleanup() {
  if [ -n "${KEEP:-}" ]; then return; fi
  "$TT" compose logs --tail 40 alertmanager prometheus >"$WORK/last.log" 2>&1 || true
  "$TT" down >/dev/null 2>&1 || true
}
trap cleanup EXIT
fail() {
  echo "FAIL: $*" >&2
  echo "--- notifications" >&2
  "$TT" notifications >&2 || true
  echo >&2
  "$TT" compose logs --tail 40 alertmanager >&2 || true
  exit 1
}
now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

# 等一条通知：alertname、status、标签过滤（jq 表达式）；since 之后收到。输出用时（秒）。
wait_notification() { # alertname status filter sinceMs limitSeconds
  local deadline=$(($(date +%s) + $5)) row
  while [ "$(date +%s)" -lt "$deadline" ]; do
    row="$("$TT" notifications | jq -c --arg a "$1" --arg s "$2" --argjson since "$4" \
      "[.[] | select(.receivedAtMs >= \$since) | .receivedAtMs as \$at | .payload.alerts[] | select(.labels.alertname == \$a and .status == \$s and ($3)) | . + {at: \$at}] | first // empty" 2>/dev/null || true)"
    if [ -n "$row" ]; then
      printf '%s' "$row"
      return 0
    fi
    sleep 2
  done
  return 1
}
wait_query() { # 描述 PromQL jq条件 秒
  local deadline=$(($(date +%s) + $4))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if "$TT" query "$2" 2>/dev/null | jq -e "$3" >/dev/null 2>&1; then
      echo "ok: $1"
      return 0
    fi
    sleep 3
  done
  fail "$1"
}

"$TT" down >/dev/null 2>&1 || true
echo "== 启动 E1"
"$TT" up e1 $CONSOLE

echo "== 真实配置已加载，抓取正常"
wait_query "n1 被抓取，node_id 来自节点表" 'up{job="im",node_id="n1"}' 'length == 1 and .[0].value[1] == "1"' 90
wait_query "n1 版本 0.3.0" 'im_build_info{job="im",node_id="n1",version="0.3.0"}' 'length == 1' 30
wait_query "textfile 指标经 node_exporter 到达" 'im_backup_last_success_timestamp_seconds{source="im"}' 'length == 1' 60
[ "$("$TT" compose exec -T alertmanager wget -qO- http://127.0.0.1:9093/-/ready >/dev/null 2>&1; echo $?)" = 0 ] || fail "alertmanager not ready"

echo "== Grafana：只听回环，认证代理和数据源连通"
G() { "$TT" compose exec -T grafana wget -qO- --header 'X-WEBAUTH-USER: oncall' "http://127.0.0.1:3000$1"; }
for _ in $(seq 1 60); do G /api/dashboards/uid/tianting-overview >/dev/null 2>&1 && break; sleep 2; done
G /api/dashboards/uid/tianting-overview | jq -e '.dashboard.panels | length == 14' >/dev/null || fail "dashboard"
G '/api/datasources/proxy/uid/prometheus/api/v1/query?query=up%7Bjob%3D%22im%22%7D' | jq -e '.data.result | length == 1' >/dev/null || fail "grafana datasource"
echo "ok: 14 个面板，Viewer 能经数据源查到 n1"

echo "== AC-OPS-07-2：kill -9，3 分钟内告警；重启后 3 分钟内恢复"
t0="$(now_ms)"
"$TT" kill n1
row="$(wait_notification 节点不可达 firing '.labels.node_id == "n1"' "$t0" "$LIMIT")" || fail "节点不可达 not notified in ${LIMIT}s"
echo "$row" | jq -e '.labels.severity == "critical" and (.startsAt | length > 0) and .annotations.page == "/nodes"' >/dev/null || fail "节点不可达 content: $row"
echo "ok: 节点不可达 用时 $(( ($(echo "$row" | jq .at) - t0) / 1000 )) 秒"
sleep $((180 - ($(now_ms) - t0) / 1000 > 0 ? 180 - ($(now_ms) - t0) / 1000 : 0))
"$TT" start n1
t1="$(now_ms)"
row="$(wait_notification 节点不可达 resolved '.labels.node_id == "n1"' "$t1" "$LIMIT")" || fail "节点不可达 not resolved in ${LIMIT}s"
echo "ok: 已恢复 用时 $(( ($(echo "$row" | jq .at) - t1) / 1000 )) 秒"

echo "== AC-OPS-07-3：console-server 不在运行时，备份过期 3 分钟内告警，改回后 3 分钟内恢复"
if [ -n "$CONSOLE" ]; then "$TT" compose stop console; fi
t2="$(now_ms)"
"$TT" backup-age im 7260
row="$(wait_notification 备份过期 firing '.labels.source == "im"' "$t2" "$LIMIT")" || fail "备份过期 not notified in ${LIMIT}s"
echo "ok: 备份过期 用时 $(( ($(echo "$row" | jq .at) - t2) / 1000 )) 秒"
t3="$(now_ms)"
"$TT" backup-age im 0
row="$(wait_notification 备份过期 resolved '.labels.source == "im"' "$t3" "$LIMIT")" || fail "备份过期 not resolved in ${LIMIT}s"
echo "ok: 已恢复 用时 $(( ($(echo "$row" | jq .at) - t3) / 1000 )) 秒"
if [ -n "$CONSOLE" ]; then "$TT" compose start console; fi

echo "== AC-OPS-07-6：全部通知的内容"
"$TT" notifications >"$WORK/notifications.json"
jq -e 'length > 0' "$WORK/notifications.json" >/dev/null || fail "no notifications"
jq -e '[.[].payload.alerts[]] | all(.labels.node_id != null)' "$WORK/notifications.json" >/dev/null || fail "通知缺 node_id: $(jq -c '[.[].payload.alerts[] | select(.labels.node_id == null) | .labels]' "$WORK/notifications.json")"
python3 -I "$HERE/../observability/check-alertmanager" --scan "$WORK/notifications.json" || fail "通知内容含敏感信息"
echo "ok: $(jq 'length' "$WORK/notifications.json") 条通知，都有 node_id，正则 0 命中"

echo "== AC-OPS-90-5：同时停止 console-server、Grafana、Prometheus 后跑 TC-PERF-00-01（打到 n1 本身）"
"$TT" stop-observability
(cd "$ROOT/server" && go build -o "$WORK/im-loadtest" ./cmd/im-loadtest)
(umask 077; grep '^ADMIN_KEY=' "${TT_RUN_DIR:-$HERE/.run}/secrets" | cut -d= -f2- >"$WORK/admin.key")
"$WORK/im-loadtest" -scenarios latency -external-addr 127.0.0.1:18080 -external-admin-addr 127.0.0.1:18081 \
  -external-admin-key-file "$WORK/admin.key" -dir "$WORK" -out "$WORK/latency.md" >/dev/null || { cat "$WORK/latency.md" >&2 || true; fail "TC-PERF-00-01"; }
grep -E 'P50|P99|TC-PERF-00-01' "$WORK/latency.md" | head -6 || true
"$TT" start-observability

echo "== 升到 E2a：n2 版本 0.3.1，各自的 SQLite，同一份密钥"
"$TT" up e2a $CONSOLE
wait_query "两个节点都被抓取（file_sd 热加载）" 'up{job="im"}' 'length == 2 and all(.[]; .value[1] == "1")' 120
wait_query "AC-OPS-04-6（节点侧）：两个版本号" 'count by (version) (im_build_info{job="im"})' '[.[].metric.version] | sort == ["0.3.0","0.3.1"]' 60

echo "== AC-OPS-08-11（节点侧）：kill -STOP 暂停 n2"
"$TT" pause n2
if curl -fsS -m 2 http://127.0.0.1:18180/readyz >/dev/null 2>&1; then fail "paused n2 still answers"; fi
curl -fsS -m 2 http://127.0.0.1:18080/readyz >/dev/null || fail "n1 affected by pausing n2"
wait_query "暂停的 n2 在 Prometheus 里是 down" 'up{job="im",node_id="n2"}' '.[0].value[1] == "0"' 60
"$TT" set-clocks "$(now_ms)" # 跳过暂停的 n2，不挂住
"$TT" resume n2
curl -fsS -m 5 --retry 5 --retry-all-errors --retry-delay 1 http://127.0.0.1:18180/readyz >/dev/null || fail "n2 did not resume"
echo "ok: 暂停和恢复"

echo "== 环境验收通过"
