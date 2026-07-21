#!/bin/bash
#============================================================
# Terraria vanilla dedicated server installer (Debian 13/Ubuntu)
#============================================================
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

TS_USER="${TS_USER:-terraria}"
TS_DIR="${TS_DIR:-/opt/terraria}"
TS_SERVER_DIR="${TS_SERVER_DIR:-${TS_DIR}/server}"
TS_WORLD_DIR="${TS_WORLD_DIR:-${TS_DIR}/world}"
SERVICE_NAME="${SERVICE_NAME:-terraria-server}"
MANAGER_SCRIPT="${MANAGER_SCRIPT:-/usr/local/bin/terraria-manager}"
CREDS_FILE="${CREDS_FILE:-/etc/terraria/credentials.env}"
TS_PORT="${TS_PORT:-7777}"
TS_MAX_PLAYERS="${TS_MAX_PLAYERS:-8}"
TS_SERVER_NAME="${TS_SERVER_NAME:-Terraria Server}"
TS_SERVER_PASSWORD="${TS_SERVER_PASSWORD:-}"
TS_WORLD_NAME="${TS_WORLD_NAME:-world}"
TS_DIFFICULTY="${TS_DIFFICULTY:-1}"
TS_SEED="${TS_SEED:-}"
TS_LANGUAGE="${TS_LANGUAGE:-en-US}"
TS_SECURE="${TS_SECURE:-0}"
TS_MEMORY_MAX="${TS_MEMORY_MAX:-4G}"
TS_VERSION="${TS_VERSION:-}"
TS_STABLE_FALLBACK="${TS_STABLE_FALLBACK:-1455}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"
FORCE_CONFIG_REWRITE="${FORCE_CONFIG_REWRITE:-0}"

show_help() {
    cat <<'EOF'
用法: sudo bash terraria-server-install.sh [--help]

安装或更新 Terraria 原版专用服务器。重复运行默认保留已有凭证、配置和世界。
环境变量:
  FORCE_CONFIG_REWRITE=1  重新生成 credentials.env 和 serverconfig.txt
  NONINTERACTIVE=1        使用环境变量/默认值，不显示交互配置
  TS_VERSION=<编号>       固定官方服务器版本；留空自动探测当前版本
  TS_STABLE_FALLBACK=1455 自动探测失败时使用的稳定版本
EOF
}

parse_args() {
    [[ $# -eq 0 ]] && return
    case "$1" in
        -h|--help) [[ $# -eq 1 ]] || { error "--help 不接受其他参数"; exit 2; }; show_help; exit 0 ;;
        *) error "未知参数: $1"; show_help >&2; exit 2 ;;
    esac
}

load_existing_credentials() {
    if [[ -f "$CREDS_FILE" && "$FORCE_CONFIG_REWRITE" != "1" ]]; then
        # shellcheck disable=SC1090
        source "$CREDS_FILE"
        info "沿用已有凭证与安装配置: ${CREDS_FILE}"
    fi
    TS_SERVER_DIR="${TS_SERVER_DIR:-${TS_DIR}/server}"
    TS_WORLD_DIR="${TS_WORLD_DIR:-${TS_DIR}/world}"
}

check_root() { [[ $EUID -eq 0 ]] || { error "请使用 root 运行: sudo bash $0"; exit 1; }; }
check_system() {
    [[ -f /etc/os-release ]] || { error "无法检测系统"; exit 1; }
    # shellcheck disable=SC1091
    source /etc/os-release
    info "系统: ${PRETTY_NAME:-unknown}"
}
check_resources() {
    local cpu mem disk
    cpu=$(nproc); mem=$(python3 -c "print(round(int(open('/proc/meminfo').read().split('MemTotal:')[1].split()[0])/1048576))")
    disk=$(df -BG / | python3 -c "import sys; print(sys.stdin.readlines()[1].split()[3].rstrip('G'))")
    info "CPU: ${cpu}核 | 内存: ${mem}GB | 可用磁盘: ${disk}GB"
    if (( mem < 2 )); then warn "建议至少 2GB 内存"; fi
}

user_config() {
    [[ "$FORCE_CONFIG_REWRITE" == "1" ]] && warn "FORCE_CONFIG_REWRITE=1：将重写凭证与服务器配置，世界仍会保留"
    if [[ "$NONINTERACTIVE" != "1" && ! -f "$CREDS_FILE" || "$NONINTERACTIVE" != "1" && "$FORCE_CONFIG_REWRITE" == "1" ]]; then
        echo -e "\n${CYAN}${BOLD}========== Terraria 服务器配置 ==========${NC}"
        printf '名称: %s\n端口: %s\n玩家: %s\n世界: %s\n难度: %s\n\n' "$TS_SERVER_NAME" "$TS_PORT" "$TS_MAX_PLAYERS" "$TS_WORLD_NAME" "$TS_DIFFICULTY"
        local choice input
        read -rp "回车使用以上配置 / 输入 c 自定义: " choice
        if [[ "$choice" =~ ^[cC]$ ]]; then
            read -rp "服务器名称 [${TS_SERVER_NAME}]: " input; TS_SERVER_NAME="${input:-$TS_SERVER_NAME}"
            read -rp "游戏端口 [${TS_PORT}]: " input; TS_PORT="${input:-$TS_PORT}"
            read -rp "最大玩家数 [${TS_MAX_PLAYERS}]: " input; TS_MAX_PLAYERS="${input:-$TS_MAX_PLAYERS}"
            read -rp "服务器密码 [留空保持无密码]: " input; TS_SERVER_PASSWORD="${input:-$TS_SERVER_PASSWORD}"
            read -rp "难度 0-3 [${TS_DIFFICULTY}]: " input; TS_DIFFICULTY="${input:-$TS_DIFFICULTY}"
            read -rp "世界名称 [${TS_WORLD_NAME}]: " input; TS_WORLD_NAME="${input:-$TS_WORLD_NAME}"
            read -rp "世界种子 [留空随机]: " input; TS_SEED="${input:-$TS_SEED}"
        fi
    fi
    [[ "$TS_PORT" =~ ^[0-9]+$ ]] && (( TS_PORT >= 1 && TS_PORT <= 65535 )) || { error "TS_PORT 必须是 1-65535"; exit 1; }
    [[ "$TS_MAX_PLAYERS" =~ ^[0-9]+$ ]] && (( TS_MAX_PLAYERS >= 1 && TS_MAX_PLAYERS <= 255 )) || { error "TS_MAX_PLAYERS 必须是 1-255"; exit 1; }
    [[ "$TS_DIFFICULTY" =~ ^[0-3]$ ]] || { error "TS_DIFFICULTY 必须是 0-3"; exit 1; }
}

install_deps() {
    info "安装 Debian 13/Ubuntu 运行依赖..."
    apt-get update -y
    apt-get install -y sudo curl ca-certificates unzip tar gzip coreutils util-linux python3 nano libicu76 libssl3t64 zlib1g
}
setup_user() {
    id "$TS_USER" &>/dev/null || useradd -m -r -s /bin/bash "$TS_USER"
    mkdir -p "$TS_DIR" "$TS_SERVER_DIR" "$TS_WORLD_DIR" "${TS_DIR}/backups"
    chown -R "${TS_USER}:${TS_USER}" "$TS_DIR"
}
write_credentials() {
    [[ -f "$CREDS_FILE" && "$FORCE_CONFIG_REWRITE" != "1" ]] && return
    mkdir -p "$(dirname "$CREDS_FILE")"
    umask 077
    cat > "$CREDS_FILE" <<EOF
# Generated by terraria-server-install.sh
TS_USER=$(printf '%q' "$TS_USER")
TS_DIR=$(printf '%q' "$TS_DIR")
TS_SERVER_DIR=$(printf '%q' "$TS_SERVER_DIR")
TS_WORLD_DIR=$(printf '%q' "$TS_WORLD_DIR")
SERVICE_NAME=$(printf '%q' "$SERVICE_NAME")
TS_PORT=${TS_PORT}
TS_MAX_PLAYERS=${TS_MAX_PLAYERS}
TS_SERVER_NAME=$(printf '%q' "$TS_SERVER_NAME")
TS_SERVER_PASSWORD=$(printf '%q' "$TS_SERVER_PASSWORD")
TS_WORLD_NAME=$(printf '%q' "$TS_WORLD_NAME")
TS_DIFFICULTY=${TS_DIFFICULTY}
TS_SEED=$(printf '%q' "$TS_SEED")
TS_LANGUAGE=$(printf '%q' "$TS_LANGUAGE")
TS_SECURE=${TS_SECURE}
TS_MEMORY_MAX=$(printf '%q' "$TS_MEMORY_MAX")
TS_VERSION=$(printf '%q' "$TS_VERSION")
TS_STABLE_FALLBACK=$(printf '%q' "$TS_STABLE_FALLBACK")
EOF
    chown "root:${TS_USER}" "$CREDS_FILE"; chmod 640 "$CREDS_FILE"
}

resolve_version() {
    if [[ -n "$TS_VERSION" ]]; then printf '%s\n' "$TS_VERSION"; return; fi
    # Terraria currently serves the website from the former JSON discovery paths.
    # Use an explicitly maintained stable fallback until an official machine-readable endpoint exists.
    warn "使用稳定版本 ${TS_STABLE_FALLBACK}（可通过 TS_VERSION 覆盖）" >&2
    printf '%s\n' "$TS_STABLE_FALLBACK"
}

install_server_atomic() {
    local version url zip staging old bin
    version=$(resolve_version)
    url="https://terraria.org/api/download/pc-dedicated-server/terraria-server-${version}.zip"
    zip=$(mktemp "/tmp/terraria-${version}.XXXXXX.zip")
    staging=$(mktemp -d "${TS_DIR}/server.new.XXXXXX")
    old="${TS_DIR}/server.old.$$"
    trap 'rm -f "${zip:-}"; rm -rf "${staging:-}"' RETURN
    info "下载 Terraria 官方原版服务器 ${version}..."
    curl -fL --connect-timeout 20 --retry 3 --retry-delay 5 --max-time 600 -o "$zip" "$url"
    (( $(stat -c%s "$zip") > 100000 )) || { error "下载文件过小"; return 1; }
    unzip -tq "$zip" >/dev/null || { error "官方 zip 校验失败"; return 1; }
    unzip -q "$zip" -d "$staging"
    bin=$(python3 - "$staging" <<'PY'
import os,sys
names=('TerrariaServer.bin.x86_64','TerrariaServer','TerrariaServer.bin.x86','TerrariaServer.exe')
for root,dirs,files in os.walk(sys.argv[1]):
 for n in names:
  if n in files:
   print(os.path.join(root,n)); raise SystemExit
PY
)
    [[ -n "$bin" && -f "$bin" ]] || { error "暂存目录中缺少服务器可执行文件"; return 1; }
    chmod +x "$bin" 2>/dev/null || true
    [[ -d "$TS_SERVER_DIR" ]] && mv "$TS_SERVER_DIR" "$old"
    if ! mv "$staging" "$TS_SERVER_DIR"; then
        [[ -d "$old" ]] && mv "$old" "$TS_SERVER_DIR"
        return 1
    fi
    bin="${bin/$staging/$TS_SERVER_DIR}"
    printf '%s\n' "$bin" > "${TS_DIR}/.server_bin"
    chown -R "${TS_USER}:${TS_USER}" "$TS_SERVER_DIR" "${TS_DIR}/.server_bin"
    rm -rf "$old"; rm -f "$zip"; trap - RETURN
    info "服务器已原子更新到 ${version}"
}

create_server_config() {
    [[ -f "${TS_DIR}/serverconfig.txt" && "$FORCE_CONFIG_REWRITE" != "1" ]] && { info "保留已有配置: ${TS_DIR}/serverconfig.txt"; return; }
    cat > "${TS_DIR}/serverconfig.txt" <<EOF
world=${TS_WORLD_DIR}/${TS_WORLD_NAME}.wld
worldname=${TS_WORLD_NAME}
autocreate=3
seed=${TS_SEED}
difficulty=${TS_DIFFICULTY}
maxplayers=${TS_MAX_PLAYERS}
port=${TS_PORT}
password=${TS_SERVER_PASSWORD}
secure=${TS_SECURE}
lang=${TS_LANGUAGE}
upnp=0
npcstream=60
motd=Welcome to ${TS_SERVER_NAME}!
EOF
    chown "${TS_USER}:${TS_USER}" "${TS_DIR}/serverconfig.txt"; chmod 640 "${TS_DIR}/serverconfig.txt"
}
create_start_script() {
    cat > "${TS_DIR}/start.sh" <<'EOF'
#!/bin/bash
set -euo pipefail
CREDS_FILE="/etc/terraria/credentials.env"
# shellcheck disable=SC1090
source "$CREDS_FILE"
SERVER_BIN=$(<"${TS_DIR}/.server_bin")
cd "$TS_DIR"
if [[ "$SERVER_BIN" == *.exe ]]; then exec mono "$SERVER_BIN" -config "${TS_DIR}/serverconfig.txt"; fi
exec "$SERVER_BIN" -config "${TS_DIR}/serverconfig.txt"
EOF
    chmod 750 "${TS_DIR}/start.sh"; chown "root:${TS_USER}" "${TS_DIR}/start.sh"
}
create_service() {
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Terraria Vanilla Dedicated Server
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=600
StartLimitBurst=5
[Service]
Type=simple
User=${TS_USER}
Group=${TS_USER}
WorkingDirectory=${TS_DIR}
ExecStart=${TS_DIR}/start.sh
ExecStop=/bin/kill -SIGINT \$MAINPID
TimeoutStopSec=60
Restart=on-failure
RestartSec=10
MemoryMax=${TS_MEMORY_MAX}
OOMScoreAdjust=-500
ProtectSystem=strict
ReadWritePaths=${TS_DIR}
PrivateTmp=true
NoNewPrivileges=true
StandardOutput=journal
StandardError=journal
SyslogIdentifier=terraria
[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload; systemctl enable "$SERVICE_NAME"
    systemctl reset-failed "$SERVICE_NAME" 2>/dev/null || true
}

create_manager() {
    cat > "$MANAGER_SCRIPT" <<'MANAGER'
#!/bin/bash
set -euo pipefail
CREDS_FILE="/etc/terraria/credentials.env"
[[ -r "$CREDS_FILE" ]] || { echo "无法读取 $CREDS_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
source "$CREDS_FILE"
SERVICE="$SERVICE_NAME"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
fail() { echo -e "${RED}$*${NC}" >&2; return 1; }
need_root() { [[ $EUID -eq 0 ]] || fail "此命令需要 root"; }
usage() {
 cat <<EOF
${CYAN}Terraria 原版服务器管理工具${NC}
用法: terraria-manager <命令> [参数]
  start | stop | restart | status | logs
  backup                   创建校验备份
  restore <latest|路径>     安全恢复备份
  update                   更新官方原版服务器并保留运行状态
  config                   编辑 serverconfig.txt
  world                    显示世界文件
  info | memory            显示服务器或内存信息
EOF
}
validate_archive() {
    local archive="$1"
    tar -tzf "$archive" >/dev/null || return 1
    python3 - "$archive" <<'PY'
import sys,tarfile
with tarfile.open(sys.argv[1], 'r:gz') as t:
 for m in t.getmembers():
  p=m.name
  if p.startswith('/') or any(x=='..' for x in p.split('/')) or m.issym() or m.islnk() or m.isdev():
   raise SystemExit(1)
PY
}
service_active() { systemctl is-active --quiet "$SERVICE"; }
backup_locked() {
    local tag="${1:-backup}" backup_dir="${TS_DIR}/backups" ts partial file payload was_active=false
    mkdir -p "$backup_dir"; ts=$(date +%Y%m%d_%H%M%S)
    file="${backup_dir}/terraria_${tag}_${ts}.tar.gz"; partial="${file}.partial"; payload=$(mktemp -d)
    service_active && was_active=true
    $was_active && systemctl stop "$SERVICE"
    trap '$was_active && systemctl start "$SERVICE" >/dev/null 2>&1 || true; rm -rf "$payload"; rm -f "$partial"' RETURN
    mkdir -p "$payload/data" "$payload/etc"
    [[ -d "$TS_WORLD_DIR" ]] && cp -a "$TS_WORLD_DIR" "$payload/data/world"
    [[ -f "${TS_DIR}/serverconfig.txt" ]] && cp -a "${TS_DIR}/serverconfig.txt" "$payload/data/serverconfig.txt"
    [[ -f "$CREDS_FILE" ]] && cp -a "$CREDS_FILE" "$payload/etc/credentials.env"
    tar -czf "$partial" -C "$payload" .
    validate_archive "$partial" || fail "备份 tar 校验失败"
    mv "$partial" "$file"; sha256sum "$file" > "${file}.sha256"
    chown "${TS_USER}:${TS_USER}" "$file" "${file}.sha256" 2>/dev/null || true
    rm -rf "$payload"; $was_active && systemctl start "$SERVICE"; trap - RETURN
    printf '%s\n' "$file"
}
cmd_backup() {
    need_root; mkdir -p "${TS_DIR}/backups"
    exec 9>"${TS_DIR}/backups/.backup.lock"; flock -n 9 || fail "另一个备份/恢复任务正在运行"
    local file; file=$(backup_locked backup) || return
    echo -e "${GREEN}备份完成: ${file}${NC}"
}
resolve_restore_file() {
    local arg="$1"
    if [[ "$arg" == latest ]]; then
        python3 - "${TS_DIR}/backups" <<'PY'
import glob,os,sys
f=glob.glob(os.path.join(sys.argv[1],'terraria_*.tar.gz'))
print(max(f,key=os.path.getmtime) if f else '')
PY
    else readlink -f -- "$arg"; fi
}
cmd_restore() {
    need_root; [[ $# -eq 1 ]] || fail "用法: terraria-manager restore <latest|备份路径>"
    mkdir -p "${TS_DIR}/backups"; exec 9>"${TS_DIR}/backups/.backup.lock"; flock -n 9 || fail "另一个备份/恢复任务正在运行"
    local archive stage rollback pre was_active=false
    archive=$(resolve_restore_file "$1"); [[ -n "$archive" && -f "$archive" ]] || fail "备份不存在: $1"
    [[ -f "${archive}.sha256" ]] && (cd "$(dirname "$archive")" && sha256sum -c "$(basename "${archive}.sha256")") || [[ ! -f "${archive}.sha256" ]] || return 1
    validate_archive "$archive" || fail "备份不安全或已损坏"
    stage=$(mktemp -d "${TS_DIR}/restore.XXXXXX"); rollback=$(mktemp -d "${TS_DIR}/rollback.XXXXXX")
    tar -xzf "$archive" --no-same-owner -C "$stage"
    [[ -d "$stage/data/world" && -f "$stage/data/serverconfig.txt" ]] || { rm -rf "$stage" "$rollback"; fail "备份缺少 world 或 serverconfig.txt"; }
    service_active && was_active=true
    pre=$(backup_locked pre_restore) || { rm -rf "$stage" "$rollback"; return 1; }
    $was_active && systemctl stop "$SERVICE"
    trap 'rm -rf "$TS_WORLD_DIR" "${TS_DIR}/serverconfig.txt"; [[ -d "$rollback/world" ]] && mv "$rollback/world" "$TS_WORLD_DIR"; [[ -f "$rollback/serverconfig.txt" ]] && mv "$rollback/serverconfig.txt" "${TS_DIR}/serverconfig.txt"; [[ -f "$rollback/credentials.env" ]] && mv "$rollback/credentials.env" "$CREDS_FILE"; $was_active && systemctl start "$SERVICE" >/dev/null 2>&1 || true; rm -rf "$stage" "$rollback"' ERR
    [[ -d "$TS_WORLD_DIR" ]] && mv "$TS_WORLD_DIR" "$rollback/world"
    [[ -f "${TS_DIR}/serverconfig.txt" ]] && mv "${TS_DIR}/serverconfig.txt" "$rollback/serverconfig.txt"
    mv "$stage/data/world" "$TS_WORLD_DIR"; mv "$stage/data/serverconfig.txt" "${TS_DIR}/serverconfig.txt"
    if [[ -f "$stage/etc/credentials.env" ]]; then cp -a "$CREDS_FILE" "$rollback/credentials.env" 2>/dev/null || true; cp -a "$stage/etc/credentials.env" "$CREDS_FILE"; fi
    chown -R "${TS_USER}:${TS_USER}" "$TS_WORLD_DIR" "${TS_DIR}/serverconfig.txt"; chown "root:${TS_USER}" "$CREDS_FILE"; chmod 640 "$CREDS_FILE"
    if $was_active && ! systemctl start "$SERVICE"; then
        rm -rf "$TS_WORLD_DIR" "${TS_DIR}/serverconfig.txt"
        [[ -d "$rollback/world" ]] && mv "$rollback/world" "$TS_WORLD_DIR"
        [[ -f "$rollback/serverconfig.txt" ]] && mv "$rollback/serverconfig.txt" "${TS_DIR}/serverconfig.txt"
        [[ -f "$rollback/credentials.env" ]] && mv "$rollback/credentials.env" "$CREDS_FILE"
        systemctl start "$SERVICE" || true
        trap - ERR; rm -rf "$stage" "$rollback"; fail "恢复后的服务启动失败，已回滚"
    fi
    trap - ERR; rm -rf "$stage" "$rollback"
    echo -e "${GREEN}恢复完成；恢复前备份: ${pre}${NC}"
}
resolve_version() {
    [[ -n "${TS_VERSION:-}" ]] && { echo "$TS_VERSION"; return; }
    echo "${TS_STABLE_FALLBACK:-1455}"
}
cmd_update() {
    need_root; local active=false version zip stage old bin url
    service_active && active=true; version=$(resolve_version); zip=$(mktemp "/tmp/terraria-${version}.XXXX.zip"); stage=$(mktemp -d "${TS_DIR}/server.new.XXXXXX"); old="${TS_DIR}/server.old.$$"
    url="https://terraria.org/api/download/pc-dedicated-server/terraria-server-${version}.zip"
    trap 'if [[ -d "$old" ]]; then rm -rf "$TS_SERVER_DIR"; mv "$old" "$TS_SERVER_DIR"; fi; $active && systemctl start "$SERVICE" >/dev/null 2>&1 || true; rm -f "$zip"; rm -rf "$stage"' RETURN
    curl -fL --connect-timeout 20 --retry 3 --max-time 600 -o "$zip" "$url"; (( $(stat -c%s "$zip") > 100000 )); unzip -tq "$zip" >/dev/null; unzip -q "$zip" -d "$stage"
    bin=$(python3 - "$stage" <<'PY'
import os,sys
for r,d,f in os.walk(sys.argv[1]):
 for n in ('TerrariaServer.bin.x86_64','TerrariaServer','TerrariaServer.bin.x86','TerrariaServer.exe'):
  if n in f: print(os.path.join(r,n)); raise SystemExit
PY
)
    [[ -n "$bin" && -f "$bin" ]] || fail "更新包缺少服务器程序"
    $active && systemctl stop "$SERVICE"
    mv "$TS_SERVER_DIR" "$old"; mv "$stage" "$TS_SERVER_DIR"; bin="${bin/$stage/$TS_SERVER_DIR}"; chmod +x "$bin" 2>/dev/null || true; echo "$bin" > "${TS_DIR}/.server_bin"; chown -R "${TS_USER}:${TS_USER}" "$TS_SERVER_DIR" "${TS_DIR}/.server_bin"
    if $active; then
        systemctl reset-failed "$SERVICE" || true
        if ! systemctl start "$SERVICE"; then rm -rf "$TS_SERVER_DIR"; mv "$old" "$TS_SERVER_DIR"; systemctl reset-failed "$SERVICE" || true; systemctl start "$SERVICE" || true; fail "新版本启动失败，已回滚"; fi
    fi
    rm -rf "$old"; rm -f "$zip"; trap - RETURN
    echo -e "${GREEN}已更新到 ${version}；服务状态已保持${NC}"
}
cmd_config() { [[ $# -eq 0 ]] || fail "config 不接受参数"; "${EDITOR:-nano}" "${TS_DIR}/serverconfig.txt"; echo "重启后生效: terraria-manager restart"; }
cmd_world() { [[ $# -eq 0 ]] || fail "world 不接受参数"; echo -e "${CYAN}世界目录: ${TS_WORLD_DIR}${NC}"; du -sh "$TS_WORLD_DIR" 2>/dev/null || true; python3 - "$TS_WORLD_DIR" <<'PY'
import glob,os,sys
for p in sorted(glob.glob(os.path.join(sys.argv[1],'*.wld*'))): print(f"{os.path.basename(p)}\t{os.path.getsize(p)} bytes")
PY
}
cmd_memory() { systemctl show "$SERVICE" -p MemoryCurrent -p MemoryPeak -p MemoryMax --no-pager; ps -u "$TS_USER" -o pid,%cpu,%mem,rss,cmd --no-headers 2>/dev/null || true; }
cmd_info() { local ip; ip=$(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I | python3 -c 'import sys; print((sys.stdin.read().split() or ["unknown"])[0])'); printf '地址: %s:%s\n状态: %s\n配置: %s/serverconfig.txt\n世界: %s\n服务: %s\n' "$ip" "$TS_PORT" "$(systemctl is-active "$SERVICE" 2>/dev/null || true)" "$TS_DIR" "$TS_WORLD_DIR" "$SERVICE"; }
cmd="${1:-}"; [[ -n "$cmd" ]] || { usage >&2; exit 2; }; shift
case "$cmd" in
 help|-h|--help) [[ $# -eq 0 ]] || fail "help 不接受参数"; usage ;;
 start|stop|restart) [[ $# -eq 0 ]] || fail "$cmd 不接受参数"; systemctl reset-failed "$SERVICE" 2>/dev/null || true; systemctl "$cmd" "$SERVICE" ;;
 status) [[ $# -eq 0 ]] || fail "status 不接受参数"; systemctl status "$SERVICE" --no-pager ;;
 logs) [[ $# -eq 0 ]] || fail "logs 不接受参数"; journalctl -u "$SERVICE" -f --no-pager ;;
 backup) [[ $# -eq 0 ]] || fail "backup 不接受参数"; cmd_backup ;;
 restore) cmd_restore "$@" ;;
 update) [[ $# -eq 0 ]] || fail "update 不接受参数"; cmd_update ;;
 config) cmd_config "$@" ;; world) cmd_world "$@" ;;
 info) [[ $# -eq 0 ]] || fail "info 不接受参数"; cmd_info ;;
 memory) [[ $# -eq 0 ]] || fail "memory 不接受参数"; cmd_memory ;;
 *) fail "未知命令: $cmd"; usage >&2; exit 2 ;;
esac
MANAGER
    chmod 750 "$MANAGER_SCRIPT"; chown "root:${TS_USER}" "$MANAGER_SCRIPT"
}

create_backup_timer() {
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" <<EOF
[Unit]
Description=Terraria validated backup
[Service]
Type=oneshot
User=root
ExecStart=${MANAGER_SCRIPT} backup
EOF
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" <<EOF
[Unit]
Description=Backup Terraria every 6 hours
[Timer]
OnCalendar=*-*-* 00,06,12,18:00:00
Persistent=true
RandomizedDelaySec=60
[Install]
WantedBy=timers.target
EOF
    systemctl daemon-reload; systemctl enable --now "${SERVICE_NAME}-backup.timer"
}
setup_firewall() {
    if command -v ufw &>/dev/null; then ufw allow "${TS_PORT}/tcp" comment "Terraria Server"; else warn "请在云安全组/防火墙放行 TCP ${TS_PORT}"; fi
}
start_server() {
    local was_active=false
    systemctl reset-failed "$SERVICE_NAME" 2>/dev/null || true
    systemctl is-active --quiet "$SERVICE_NAME" && was_active=true
    if $was_active; then systemctl restart "$SERVICE_NAME"; else systemctl start "$SERVICE_NAME"; fi
    sleep 3; systemctl is-active --quiet "$SERVICE_NAME" || warn "服务尚未就绪，请查看: journalctl -u ${SERVICE_NAME} -n 100"
}
show_result() {
    echo -e "${GREEN}${BOLD}Terraria 原版服务器部署完成${NC}"
    echo "管理: terraria-manager start|stop|restart|status|logs|backup|restore|update|config|world|info|memory"
    echo "配置: ${TS_DIR}/serverconfig.txt | 世界: ${TS_WORLD_DIR} | TCP: ${TS_PORT}"
}
main() {
    parse_args "$@"; check_root; load_existing_credentials; check_system; install_deps; check_resources; user_config
    if [[ "$NONINTERACTIVE" != "1" ]]; then local confirm; read -rp "回车开始部署 / 输入 n 取消: " confirm; [[ "$confirm" =~ ^[nN]$ ]] && exit 0; fi
    setup_user; write_credentials; install_server_atomic
    if [[ $(<"${TS_DIR}/.server_bin") == *.exe ]]; then apt-get install -y mono-runtime; fi
    create_server_config; create_start_script; create_service; create_manager; create_backup_timer; setup_firewall; start_server; show_result
}
main "$@"
