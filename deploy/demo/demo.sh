#!/usr/bin/env bash
# 天听本机演示环境：所有服务都在这台 Mac 上原生运行，只绑 127.0.0.1，不用 docker、不用任何云服务。说明见 README.md。
#   demo.sh up            构建并启动 im-server、console-server（含临时 Postgres 17）、假 S3、H5、管理后台，灌演示数据
#   demo.sh down          停掉 up 起的全部进程（含模拟器），删除 .run/
#   demo.sh status        各服务的地址和健康状态
#   demo.sh reset         清空数据（IM 库、后台库、假 S3），重新灌演示数据
#   demo.sh seed          只灌演示数据（已经灌过就跳过）
#   demo.sh otp [账号]    后台账号当前的动态码
#   demo.sh android       启动 AVD Pixel_10_Pro_XL（-port 5556 -read-only），装好指向演示服务端的 App
#   demo.sh android-down  关掉这个模拟器（emu kill）
#   demo.sh ios           在 iPhone 16 Pro 模拟器上构建、安装、启动指向演示服务端的 App
#   demo.sh ios-down      结束 App，关掉这个模拟器
# 状态（密钥、数据库、日志、构建产物、进程号）都在 deploy/demo/.run/，不进仓库。密钥每次 up 随机生成。
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
WS="$(cd "$REPO/.." && pwd)"
RUN="$HERE/.run"

CONSOLE_DIR="${TIANTING_CONSOLE_DIR:-$WS/tianting-console-server}"
WEB_DIR="${TIANTING_WEB_DIR:-$WS/tianting-web}"
ADMIN_DIR="${TIANTING_ADMIN_DIR:-$WS/tianting-admin}"
SDK_DIR="${TIANTING_SDK_TS_DIR:-$WS/tianting-sdk-ts}"
ANDROID_DIR="${TIANTING_ANDROID_DIR:-$WS/tianting-android}"
IOS_DIR="${TIANTING_IOS_DIR:-$WS/tianting-ios}"

BASE="${DEMO_PORT_BASE:-18380}"
P_IM=$BASE                 # im-server 对外端口（/ws、/v1/*）
P_IM_ADMIN=$((BASE + 1))   # im-server 管理端口
P_S3=$((BASE + 2))         # 假 S3
P_CONSOLE=$((BASE + 3))    # console-server 管理空间，同时托管管理后台页面
P_WEB=$((BASE + 4))        # H5（生产构建 + vite preview）
P_PG=$((BASE + 5))         # Postgres 17

H="127.0.0.1"
IM_HTTP="http://$H:$P_IM"
S3_URL="http://$H:$P_S3"
CONSOLE_URL="http://$H:$P_CONSOLE"
WEB_URL="http://$H:$P_WEB"

AVD="Pixel_10_Pro_XL"
EMU_PORT=5556
EMU_SERIAL="emulator-$EMU_PORT"
ANDROID_APP="com.tianting.app"
IOS_UDID="46E6F7B9-9E80-4E9C-9C04-9C3FF0A94E9F" # iPhone 16 Pro
IOS_APP="com.tianting.im"
ANDROID_SDK="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}"
ADB="$ANDROID_SDK/platform-tools/adb"

export LC_ALL=C

say() { printf '%s\n' "$*"; }
die() { printf '错误：%s\n' "$*" >&2; exit 1; }
need() { command -v "$1" > /dev/null 2>&1 || die "找不到 $1"; }

# ---- 进程：只按自己记下的 PID 管理。pids/<名字>.pid 是进程号，pids/<名字>.cmd 是它命令行里必须有的一段（防止 PID 被系统复用后误杀）。
pidfile() { echo "$RUN/pids/$1.pid"; }
alive() {
  local f; f="$(pidfile "$1")"
  [[ -f "$f" ]] || return 1
  local pid; pid="$(cat "$f")"
  [[ -n "$pid" ]] && kill -0 "$pid" 2> /dev/null || return 1
  local want; want="$(cat "$RUN/pids/$1.cmd" 2> /dev/null || true)"
  [[ -z "$want" ]] || ps -p "$pid" -o command= 2> /dev/null | grep -qF -- "$want"
}
remember() { # 名字 PID 命令行特征
  echo "$2" > "$(pidfile "$1")"
  printf '%s' "$3" > "$RUN/pids/$1.cmd"
}
stop_proc() { # 名字
  local f; f="$(pidfile "$1")"
  [[ -f "$f" ]] || return 0
  if alive "$1"; then
    local pid; pid="$(cat "$f")"
    kill -TERM "$pid" 2> /dev/null || true
    for _ in $(seq 1 150); do kill -0 "$pid" 2> /dev/null || break; sleep 0.1; done
    if kill -0 "$pid" 2> /dev/null; then
      say "  $1（PID $pid）15 秒没有退出，强制结束"
      kill -KILL "$pid" 2> /dev/null || true
    fi
    say "  已停止 $1（PID $pid）"
  fi
  rm -f "$f" "$RUN/pids/$1.cmd"
}

wait_http() { # 地址 秒数 名字
  local url=$1 secs=$2 name=$3
  for _ in $(seq 1 $((secs * 10))); do
    curl -fsS -o /dev/null --max-time 2 "$url" 2> /dev/null && return 0
    sleep 0.1
  done
  die "$name 没有在 ${secs} 秒内就绪（$url），日志在 $RUN/logs/"
}

port_busy() { lsof -nP -iTCP:"$1" -sTCP:LISTEN > /dev/null 2>&1; }

pg_env() { # 给 psql 用：密码走环境变量，不出现在命令行
  PGPASSWORD="$PG_SUPER" PGHOST="$H" PGPORT="$P_PG" PGUSER=postgres PGCONNECT_TIMEOUT=5 PGOPTIONS="-c client_min_messages=warning" "$@"
}

load_secrets() {
  # shellcheck disable=SC1091
  set -a; . "$RUN/secrets.env"; set +a
}

# ---- 密钥和配置：每次 up 随机生成，只在 .run/ 里 ----
gen_secrets() {
  umask 077
  local con msg
  con="$(node "$HERE/lib/keys.mjs")"
  msg="$(node "$HERE/lib/keys.mjs")"
  {
    echo "JWT_SECRET=$(openssl rand -hex 32)"
    echo "ADMIN_KEY=$(openssl rand -hex 32)"
    echo "QUARANTINE_KEY=$(openssl rand -hex 32)"
    echo "REPORT_KEY=$(openssl rand -base64 32)"
    echo "REFRESH_KEY=$(openssl rand -hex 32)"
    echo "SEED_SECRET=$(openssl rand -base64 32)"
    echo "CON_SEED=${con% *}"
    echo "CON_PUB=${con#* }"
    echo "MSGREAD_SEED=${msg% *}"
    echo "MSGREAD_PUB=${msg#* }"
    echo "DATA_KEY=$(openssl rand -base64 32)"
    echo "PEPPER=$(openssl rand -base64 32)"
    echo "PG_SUPER=$(openssl rand -hex 24)"
    echo "PG_APP=$(openssl rand -hex 24)"
  } > "$RUN/secrets.env"
}

write_config() {
  umask 077
  local web_origins="$WEB_URL,http://localhost:$P_WEB"
  # 调用方：demo-seed 只给灌数据用；console、console-msgread 的 scope 和 tianting-admin/e2e/stack.mjs 相同。
  cat > "$RUN/callers.json" << JSON
[
  {"id":"demo-seed","alg":"hmac-sha256","secret":"$SEED_SECRET",
   "scopes":["account.invite","social.friends","social.settings","group.read","group.write","moderation.words","user.read"]},
  {"id":"console","alg":"ed25519","publicKey":"$CON_PUB",
   "scopes":["user.read","user.write","user.kick","user.sessions","token.issue","group.read","group.write","group.dissolve",
             "msg.meta","audit.read","social.settings","social.friends","user.ban","msg.delete","mute.write","push.user",
             "push.all","stats.read","account.invite","moderation.words","report.read","report.handle"],
   "pushTypes":["notice","activity"]},
  {"id":"console-msgread","alg":"ed25519","publicKey":"$MSGREAD_PUB","bodyAccess":true,
   "scopes":["msg.body.group","msg.body.c2c","msg.quarantine","report.evidence"]}
]
JSON
  chmod 600 "$RUN/callers.json"
  # im-server：C1 账号（邀请码注册）、C2 图片和文件、C4 撤回引用 @、C5 置顶免打扰、C6 敏感词和举报都打开；
  # C3 离线推送（APNs、FCM）不打开。IM_PUSH_ENABLED 是后台的系统通知（PRD-live），不是 C3。
  # 不开 IM_TEST_MODE、IM_DEMO_LOGIN：登录都走真实账号和密码。假 S3 的凭证是占位值，它不校验签名。
  cat > "$RUN/im.env" << ENV
IM_ADDR=$H:$P_IM
IM_ADMIN_ADDR=$H:$P_IM_ADMIN
IM_METRICS_ADDR=off
IM_PPROF_ADDR=off
IM_OPS_ADDR=off
IM_DB=$RUN/data/im.db
IM_JWT_SECRET=$JWT_SECRET
IM_ADMIN_KEY=$ADMIN_KEY
IM_QUARANTINE_KEY=$QUARANTINE_KEY
IM_REFRESH_KEYS=demo:$REFRESH_KEY
IM_ADMIN_CALLERS=$RUN/callers.json
IM_ALLOWED_ORIGINS=$web_origins
IM_REGISTER_MODE=invite
IM_V13_ADMIN_FEATURES=1
IM_PUSH_ENABLED=1
IM_FILES_ENABLED=1
IM_OBJSTORE_ENDPOINT=$S3_URL
IM_OBJSTORE_REGION=us-east-1
IM_OBJSTORE_BUCKET=chat
IM_OBJSTORE_PUBLIC_BUCKET=avatars
IM_OBJSTORE_PUBLIC_BASE_URL=$S3_URL/avatars
IM_OBJSTORE_ACCESS_KEY=demo-fake-s3
IM_OBJSTORE_SECRET_KEY=demo-fake-s3-not-a-secret
IM_MSG_ACTIONS_ENABLED=1
IM_CONV_SETTINGS_ENABLED=1
IM_MOD_ENABLED=1
IM_REPORT_KEY=$REPORT_KEY
ENV
  cat > "$RUN/console.env" << ENV
CON_SPACES=admin
CON_ADMIN_ADDR=$H:$P_CONSOLE
CON_ADMIN_ORIGIN=$CONSOLE_URL
CON_ADMIN_WEB_DIR=$RUN/admin-dist
CON_METRICS_ADDR=off
CON_OWNER_DATABASE_URL=postgres://postgres:$PG_SUPER@$H:$P_PG/console?sslmode=disable
CON_DATABASE_URL=postgres://console_app:$PG_APP@$H:$P_PG/console?sslmode=disable
CON_DATA_KEYS=k1:$DATA_KEY
CON_HASH_PEPPER=$PEPPER
CON_IM_ADMIN_URL=http://$H:$P_IM_ADMIN
CON_IM_SIGNING_KEY=$CON_SEED
CON_IM_MSGREAD_KEY=$MSGREAD_SEED
CON_PUSH_TEMPLATES=$CONSOLE_DIR/deploy/push-templates.example.json
ENV
}

# ---- 构建 ----
build_all() {
  say "构建（第一次几分钟，之后快很多）："
  for d in "$CONSOLE_DIR" "$WEB_DIR" "$ADMIN_DIR" "$SDK_DIR"; do [[ -d "$d" ]] || die "找不到 $d"; done
  for d in "$WEB_DIR" "$ADMIN_DIR" "$SDK_DIR"; do
    [[ -d "$d/node_modules" ]] || die "$d 没有 node_modules（演示脚本不装依赖，请先在那里 npm ci）"
  done
  mkdir -p "$RUN/bin"
  say "  im-server"
  (cd "$REPO/server" && timeout 600 go build -o "$RUN/bin/im-server" ./cmd/im-server) > "$RUN/logs/build-im.log" 2>&1 \
    || { tail -20 "$RUN/logs/build-im.log" >&2; die "im-server 构建失败"; }
  say "  console-server"
  (cd "$CONSOLE_DIR" && timeout 600 go build -o "$RUN/bin/console-server" ./cmd/console-server) > "$RUN/logs/build-console.log" 2>&1 \
    || { tail -20 "$RUN/logs/build-console.log" >&2; die "console-server 构建失败"; }
  say "  SDK（tianting-sdk-ts/dist，H5 和灌数据脚本都用它）"
  (cd "$SDK_DIR" && timeout 300 node node_modules/typescript/bin/tsc -p tsconfig.build.json) > "$RUN/logs/build-sdk.log" 2>&1 \
    || { tail -20 "$RUN/logs/build-sdk.log" >&2; die "SDK 构建失败"; }
  say "  H5（生产构建，带 CSP；对象存储源 $S3_URL，不显示演示登录）"
  (cd "$WEB_DIR" && VITE_STORAGE_ORIGINS="$S3_URL" VITE_DEMO_LOGIN=false TIANTING_BUILD_ID=demo \
    timeout 300 node node_modules/vite/bin/vite.js build --logLevel warn --outDir "$RUN/web-dist" --emptyOutDir) \
    > "$RUN/logs/build-web.log" 2>&1 || { tail -20 "$RUN/logs/build-web.log" >&2; die "H5 构建失败"; }
  say "  管理后台（生产构建，由 console-server 托管，CSP 是 console-server 的响应头）"
  (cd "$ADMIN_DIR" && timeout 300 node node_modules/vite/bin/vite.js build --logLevel warn --outDir "$RUN/admin-dist" --emptyOutDir) \
    > "$RUN/logs/build-admin.log" 2>&1 || { tail -20 "$RUN/logs/build-admin.log" >&2; die "管理后台构建失败"; }
}

# ---- Postgres 17（临时实例，数据在 .run/pg） ----
pg_bin() {
  local b
  if [[ -n "${PG_BIN:-}" ]]; then echo "$PG_BIN"; return; fi
  b="$(brew --prefix postgresql@17 2> /dev/null || true)/bin"
  [[ -x "$b/initdb" ]] && { echo "$b"; return; }
  dirname "$(command -v initdb)"
}
pg_start() {
  local bin; bin="$(pg_bin)"
  "$bin/postgres" --version | grep -q ' 17\.' || die "需要 Postgres 17（找到的是 $("$bin/postgres" --version)），可以用 PG_BIN 指定"
  if [[ ! -d "$RUN/pg" ]]; then
    printf '%s\n' "$PG_SUPER" > "$RUN/pg-pw"
    LC_ALL=C timeout 120 "$bin/initdb" -D "$RUN/pg" -U postgres -E UTF8 --locale=C -A scram-sha-256 --pwfile="$RUN/pg-pw" \
      > "$RUN/logs/initdb.log" 2>&1 || { tail -20 "$RUN/logs/initdb.log" >&2; die "initdb 失败"; }
    rm -f "$RUN/pg-pw"
  fi
  mkdir -p "$RUN/pgsock"
  LC_ALL=C timeout 90 "$bin/pg_ctl" -D "$RUN/pg" -l "$RUN/logs/postgres.log" -w -t 60 \
    -o "-c port=$P_PG -c listen_addresses=$H -c unix_socket_directories=$RUN/pgsock -c lc_messages=C" start > /dev/null \
    || { tail -20 "$RUN/logs/postgres.log" >&2; die "Postgres 没有起来"; }
}
pg_stop() {
  [[ -f "$RUN/pg/postmaster.pid" ]] || return 0
  local bin pid; bin="$(pg_bin)"; pid="$(head -1 "$RUN/pg/postmaster.pid")"
  LC_ALL=C timeout 60 "$bin/pg_ctl" -D "$RUN/pg" -m fast -w -t 45 stop > /dev/null 2>&1 || true
  say "  已停止 Postgres（PID $pid）"
}
pg_create_db() {
  local bin; bin="$(pg_bin)"
  pg_env timeout 30 "$bin/psql" -d postgres -v ON_ERROR_STOP=1 -qc "DROP DATABASE IF EXISTS console WITH (FORCE)" > /dev/null
  pg_env timeout 30 "$bin/psql" -d postgres -v ON_ERROR_STOP=1 -qc "CREATE DATABASE console" > /dev/null
}
pg_grant_app() { # 迁移建的 console_app 没有 LOGIN 和密码（0001_schemas_roles.sql）：这里给它
  local bin; bin="$(pg_bin)"
  printf "ALTER ROLE console_app LOGIN PASSWORD '%s';\n" "$PG_APP" \
    | pg_env timeout 30 "$bin/psql" -d console -v ON_ERROR_STOP=1 -q -f - > /dev/null
}

# ---- 各服务 ----
start_s3() {
  ( exec nohup python3 -I "$HERE/fake_s3.py" --port "$P_S3" --origins "$WEB_URL,http://localhost:$P_WEB" ) \
    > "$RUN/logs/s3.log" 2>&1 < /dev/null &
  remember s3 $! "$HERE/fake_s3.py"
  wait_http "$S3_URL/__health" 20 "假 S3"
}
start_im() {
  mkdir -p "$RUN/data"
  ( set -a; . "$RUN/im.env"; set +a; exec nohup "$RUN/bin/im-server" ) > "$RUN/logs/im-server.log" 2>&1 < /dev/null &
  remember im-server $! "$RUN/bin/im-server"
  wait_http "$IM_HTTP/readyz" 60 "im-server"
}
start_console() {
  ( set -a; . "$RUN/console.env"; set +a; timeout 120 "$RUN/bin/console-server" migrate up ) > "$RUN/logs/console-migrate.log" 2>&1 \
    || { tail -20 "$RUN/logs/console-migrate.log" >&2; die "console-server 迁移失败"; }
  pg_grant_app
  local out
  out="$( set -a; . "$RUN/console.env"; set +a
          timeout 60 "$RUN/bin/console-server" account bootstrap --space admin --username chaoguan 2> "$RUN/logs/console-bootstrap.log" )" \
    || { tail -20 "$RUN/logs/console-bootstrap.log" >&2; die "创建后台超管失败"; }
  ROOT_CODE="$(printf '%s' "$out" | tail -1 | awk -F': ' '{print $NF}' | tr -d '[:space:]')"
  [[ -n "$ROOT_CODE" ]] || die "没有拿到超管激活码"
  ( set -a; . "$RUN/console.env"; set +a; exec nohup "$RUN/bin/console-server" ) > "$RUN/logs/console-server.log" 2>&1 < /dev/null &
  remember console-server $! "$RUN/bin/console-server"
  wait_http "$CONSOLE_URL/readyz" 60 "console-server"
}
start_web() {
  ( cd "$WEB_DIR" && TIANTING_SERVER="$IM_HTTP" VITE_STORAGE_ORIGINS="$S3_URL" \
      exec nohup node node_modules/vite/bin/vite.js preview --outDir "$RUN/web-dist" --host "$H" --port "$P_WEB" --strictPort ) \
    > "$RUN/logs/web.log" 2>&1 < /dev/null &
  remember web $! "vite.js preview --outDir $RUN/web-dist"
  wait_http "$WEB_URL/" 60 "H5"
}

write_state() {
  umask 077
  cat > "$RUN/state.json" << JSON
{"imHttp":"$IM_HTTP","imWs":"ws://$H:$P_IM/ws","imAdmin":"http://$H:$P_IM_ADMIN","consoleOrigin":"$CONSOLE_URL",
 "seedCaller":"demo-seed","seedSecret":"$SEED_SECRET","rootUsername":"chaoguan","rootActivationCode":"$ROOT_CODE"}
JSON
}

seed() {
  [[ -f "$RUN/state.json" ]] || die "服务没有起来，先 demo.sh up"
  DEMO_RUN="$RUN" DEMO_SDK="$SDK_DIR" timeout 300 node "$HERE/lib/seed.mjs" 2>&1 | tee "$RUN/logs/seed.log"
  [[ "${PIPESTATUS[0]}" == 0 ]] || die "灌数据失败，看 $RUN/logs/seed.log"
}

services_up() { # 数据目录已经准备好之后启动 s3、im、console、web
  start_s3
  start_im
  start_console
  write_state
  alive web || start_web
}

cmd_up() {
  need go; need node; need python3; need openssl; need curl; need lsof; need timeout
  if [[ -f "$RUN/pids/im-server.pid" ]] && alive im-server; then
    say "演示环境已经在运行（先 demo.sh down 再 up，或者 demo.sh reset 重灌数据）"; cmd_status; return
  fi
  local p
  for p in "$P_IM" "$P_IM_ADMIN" "$P_S3" "$P_CONSOLE" "$P_WEB" "$P_PG"; do
    port_busy "$p" && die "端口 $p 已经被占用（演示环境用 $P_IM～$P_PG，可以用 DEMO_PORT_BASE 换一段）"
  done
  rm -rf "$RUN"
  mkdir -p "$RUN/pids" "$RUN/logs"
  chmod 700 "$RUN"
  gen_secrets
  load_secrets
  write_config
  build_all
  say "启动："
  pg_start
  pg_create_db
  services_up
  say "  全部就绪"
  seed
  cmd_status
}

stop_services() {
  stop_proc web
  stop_proc console-server
  stop_proc im-server
  stop_proc s3
}

cmd_down() {
  [[ -d "$RUN" ]] || { say "没有在运行的演示环境"; return; }
  say "停止："
  if [[ -f "$RUN/pids/emulator.pid" ]]; then cmd_android_down || true; fi
  if [[ -f "$RUN/ios-booted" ]]; then cmd_ios_down || true; fi
  stop_services
  pg_stop
  rm -rf "$RUN"
  say "已删除 $RUN"
}

cmd_reset() {
  [[ -f "$RUN/secrets.env" ]] || die "演示环境没有在运行，先 demo.sh up"
  load_secrets
  say "清空数据："
  stop_proc console-server
  stop_proc im-server
  stop_proc s3
  rm -rf "$RUN/data" "$RUN/seeded" "$RUN/console-accounts.json"
  if ! pg_env "$(pg_bin)/pg_isready" -q 2> /dev/null; then pg_start; fi
  pg_create_db
  say "  IM 库、后台库、假 S3 里的文件都清掉了"
  say "启动："
  services_up
  seed
  say "提示：手机上的 App 本地还留着旧数据，重新运行 demo.sh android / demo.sh ios（会重装 App）"
}

health() { # 地址
  if curl -fsS -o /dev/null --max-time 2 "$1" 2> /dev/null; then echo "正常"; else echo "不可用"; fi
}

cmd_status() {
  [[ -d "$RUN" ]] || { say "没有在运行的演示环境（demo.sh up 启动）"; return; }
  local pg="不可用"
  [[ -f "$RUN/secrets.env" ]] && load_secrets && pg_env "$(pg_bin)/pg_isready" -q 2> /dev/null && pg="正常"
  say ""
  say "[$(health "$WEB_URL/")] H5（小明用）        $WEB_URL"
  say "[$(health "$CONSOLE_URL/readyz")] 管理后台            $CONSOLE_URL"
  say "[$(health "$IM_HTTP/readyz")] im-server           $IM_HTTP（管理接口 http://$H:$P_IM_ADMIN）"
  say "[$(health "$S3_URL/__health")] 假 S3               $S3_URL"
  say "[$pg] Postgres 17         $H:$P_PG"
  local a="没有启动" i="没有启动"
  if [[ -f "$RUN/pids/emulator.pid" ]] && alive emulator; then a="运行中（$EMU_SERIAL，App 连 http://10.0.2.2:$P_IM）"; fi
  if [[ -f "$RUN/ios-booted" ]]; then i="已启动（iPhone 16 Pro，App 连 $IM_HTTP）"; fi
  say "Android 模拟器：$a"
  say "iOS 模拟器：$i"
  if [[ -f "$RUN/seeded" ]]; then
    say ""
    say "演示账号（密码都是 tt-demo-2026）：小明 xiaoming（H5）、小红 xiaohong（Android）、小刚 xiaogang（iOS）、"
    say "  好友 小丽 xiaoli、阿伟 awei，陌生人 路人甲 lurenjia"
    say "后台账号（密码都是 Tianting-Demo-Only-2026，动态码用 demo.sh otp 看）：超管 chaoguan、运营 yunying、审核 shenhe、客服 kefu"
  else
    say ""
    say "还没有灌演示数据（demo.sh seed）"
  fi
}

cmd_otp() {
  [[ -f "$RUN/console-accounts.json" ]] || die "还没有后台账号（先 demo.sh up）"
  node "$HERE/lib/totp.mjs" "$RUN" "${1:-}"
}

# ---- Android ----
cmd_android() {
  [[ -f "$RUN/pids/im-server.pid" ]] && alive im-server || die "演示服务端没有在运行，先 demo.sh up"
  [[ -x "$ADB" ]] || die "找不到 adb（$ADB）"
  local emu="$ANDROID_SDK/emulator/emulator"
  [[ -x "$emu" ]] || die "找不到 emulator（$emu）"
  "$emu" -list-avds | grep -qx "$AVD" || die "没有 AVD $AVD"
  if [[ -f "$RUN/pids/emulator.pid" ]] && alive emulator; then
    say "模拟器已经在运行（$EMU_SERIAL），重新安装 App"
  else
    # 5556 被别的实例占着时不碰它，停下来。
    if timeout 20 "$ADB" devices | grep -q "^$EMU_SERIAL"; then die "$EMU_SERIAL 已经有别的模拟器在用，演示脚本不碰它"; fi
    if port_busy "$EMU_PORT" || port_busy $((EMU_PORT + 1)); then die "端口 $EMU_PORT/$((EMU_PORT + 1)) 已经被占用，演示脚本不碰它"; fi
    say "启动模拟器 $AVD（-port $EMU_PORT -read-only，退出后不保留任何改动）"
    ( exec nohup "$emu" -avd "$AVD" -port "$EMU_PORT" -read-only ) > "$RUN/logs/emulator.log" 2>&1 < /dev/null &
    remember emulator $! "-avd $AVD -port $EMU_PORT"
    timeout 180 "$ADB" -s "$EMU_SERIAL" wait-for-device || die "模拟器 3 分钟内没有出现，看 $RUN/logs/emulator.log"
    local ok=""
    for _ in $(seq 1 180); do
      [[ "$(timeout 10 "$ADB" -s "$EMU_SERIAL" shell getprop sys.boot_completed 2> /dev/null | tr -d '\r')" == 1 ]] && { ok=1; break; }
      alive emulator || die "模拟器进程退出了，看 $RUN/logs/emulator.log"
      sleep 2
    done
    [[ -n "$ok" ]] || die "模拟器 6 分钟内没有启动完"
  fi
  say "构建 App（debug，服务端 http://10.0.2.2:$P_IM）"
  (cd "$ANDROID_DIR" && timeout 1200 ./gradlew -q --no-daemon :app:assembleDebug "-Ptianting.devServer=http://10.0.2.2:$P_IM") \
    > "$RUN/logs/build-android.log" 2>&1 || { tail -30 "$RUN/logs/build-android.log" >&2; die "Android 构建失败"; }
  local apk="$ANDROID_DIR/app/build/outputs/apk/debug/app-debug.apk"
  [[ -f "$apk" ]] || die "找不到 $apk"
  timeout 30 "$ADB" -s "$EMU_SERIAL" uninstall "$ANDROID_APP" > /dev/null 2>&1 || true
  timeout 180 "$ADB" -s "$EMU_SERIAL" install -r "$apk" > /dev/null || die "安装 App 失败"
  # 图片和文件的预签名地址是 http://127.0.0.1:$P_S3：把模拟器里的这个端口转到 Mac 上（debug 构建允许明文连 127.0.0.1）。
  timeout 20 "$ADB" -s "$EMU_SERIAL" reverse "tcp:$P_S3" "tcp:$P_S3" > /dev/null
  timeout 20 "$ADB" -s "$EMU_SERIAL" shell monkey -p "$ANDROID_APP" -c android.intent.category.LAUNCHER 1 > /dev/null 2>&1 \
    || die "启动 App 失败"
  say "Android 就绪：用 小红 xiaohong / tt-demo-2026 登录（关闭：demo.sh android-down）"
}

cmd_android_down() {
  [[ -f "$RUN/pids/emulator.pid" ]] || { say "没有演示脚本启动的模拟器"; return 0; }
  if alive emulator; then
    local pid; pid="$(cat "$RUN/pids/emulator.pid")"
    timeout 30 "$ADB" -s "$EMU_SERIAL" emu kill > /dev/null 2>&1 || true
    for _ in $(seq 1 600); do kill -0 "$pid" 2> /dev/null || break; sleep 0.1; done
    if kill -0 "$pid" 2> /dev/null; then stop_proc emulator; else say "  已关闭模拟器 $EMU_SERIAL（PID $pid）"; fi
  fi
  rm -f "$RUN/pids/emulator.pid" "$RUN/pids/emulator.cmd"
}

# ---- iOS ----
ios_state() { xcrun simctl list devices | grep -F "($IOS_UDID)" | sed -E 's/.*\((Booted|Shutdown|[A-Za-z ]+)\) *$/\1/'; }

cmd_ios() {
  [[ -f "$RUN/pids/im-server.pid" ]] && alive im-server || die "演示服务端没有在运行，先 demo.sh up"
  need xcrun; need xcodebuild
  xcrun simctl list devices | grep -F "($IOS_UDID)" | grep -q 'iPhone 16 Pro (' || die "找不到 iPhone 16 Pro（$IOS_UDID）"
  if [[ "$(ios_state)" != Booted ]]; then
    say "启动模拟器 iPhone 16 Pro"
    timeout 120 xcrun simctl boot "$IOS_UDID"
  fi
  : > "$RUN/ios-booted"
  timeout 300 xcrun simctl bootstatus "$IOS_UDID" -b > /dev/null || die "iPhone 16 Pro 没有启动完"
  # 给观众看的窗口：有 Simulator.app 就用它；Xcode 27 里没有，换成 Xcode 自带的 DeviceHub（com.apple.dt.Devices）。
  if ! open -b com.apple.iphonesimulator --args -CurrentDeviceUDID "$IOS_UDID" 2> /dev/null; then
    open -b com.apple.dt.Devices 2> /dev/null || say "  （没有找到能显示模拟器画面的 App，模拟器照样在后台运行）"
  fi
  say "构建 App（Debug，第一次要几分钟；产物在 tianting-ios/build/demo，不进仓库）"
  local derived="$IOS_DIR/build/demo"
  timeout 1800 xcodebuild -project "$IOS_DIR/Tianting.xcodeproj" -scheme Tianting -configuration Debug \
    -destination "id=$IOS_UDID" -derivedDataPath "$derived" build > "$RUN/logs/build-ios.log" 2>&1 \
    || { grep -E 'error:' "$RUN/logs/build-ios.log" | head -20 >&2; die "iOS 构建失败，看 $RUN/logs/build-ios.log"; }
  local app="$derived/Build/Products/Debug-iphonesimulator/Tianting.app"
  [[ -d "$app" ]] || die "找不到 $app"
  timeout 60 xcrun simctl terminate "$IOS_UDID" "$IOS_APP" > /dev/null 2>&1 || true
  timeout 60 xcrun simctl uninstall "$IOS_UDID" "$IOS_APP" > /dev/null 2>&1 || true
  timeout 120 xcrun simctl install "$IOS_UDID" "$app" || die "安装 App 失败"
  # Debug 构建认启动环境变量 TIANTING_SERVER（Kit/Sources/TiantingAppKit/Core/Config.swift）：不用改 Info-Debug.plist。
  SIMCTL_CHILD_TIANTING_SERVER="$IM_HTTP" timeout 60 xcrun simctl launch "$IOS_UDID" "$IOS_APP" > /dev/null \
    || die "启动 App 失败"
  say "iOS 就绪：用 小刚 xiaogang / tt-demo-2026 登录（关闭：demo.sh ios-down）"
  say "注意：从主屏幕重新点开 App 不会带上服务端地址，要重开就再运行一次 demo.sh ios"
}

cmd_ios_down() {
  if [[ "$(ios_state)" == Booted ]]; then
    timeout 60 xcrun simctl terminate "$IOS_UDID" "$IOS_APP" > /dev/null 2>&1 || true
    timeout 120 xcrun simctl shutdown "$IOS_UDID" || true
    say "  已关闭 iPhone 16 Pro 模拟器"
  else
    say "  iPhone 16 Pro 没有在运行"
  fi
  rm -f "$RUN/ios-booted"
}

case "${1:-}" in
  up) cmd_up ;;
  down) cmd_down ;;
  status) cmd_status ;;
  reset) cmd_reset ;;
  seed) [[ -f "$RUN/secrets.env" ]] || die "先 demo.sh up"; seed ;;
  otp) cmd_otp "${2:-}" ;;
  android) mkdir -p "$RUN/logs" "$RUN/pids"; cmd_android ;;
  android-down) cmd_android_down ;;
  ios) mkdir -p "$RUN/logs"; cmd_ios ;;
  ios-down) cmd_ios_down ;;
  *) sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
