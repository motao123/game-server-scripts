#!/bin/bash
#============================================================
# Valheim vanilla dedicated server installer (Debian 13/Ubuntu)
#============================================================
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

VH_USER="${VH_USER:-valheim}"
VH_DIR="${VH_DIR:-/opt/valheim}"
VH_SERVER_DIR="${VH_SERVER_DIR:-${VH_DIR}/server}"
VH_WORLD_DIR="${VH_WORLD_DIR:-${VH_DIR}/world}"
SERVICE_NAME="${SERVICE_NAME:-valheim-server}"
MANAGER_SCRIPT="${MANAGER_SCRIPT:-/usr/local/bin/valheim-manager}"
CREDS_FILE="${CREDS_FILE:-/etc/valheim/credentials.env}"
VH_SERVER_NAME="${VH_SERVER_NAME:-Valheim Server}"
VH_SERVER_PORT="${VH_SERVER_PORT:-2456}"
VH_QUERY_PORT="${VH_QUERY_PORT:-}"
VH_WORLD_NAME="${VH_WORLD_NAME:-Dedicated}"
VH_SERVER_PASSWORD="${VH_SERVER_PASSWORD:-}"
VH_PUBLIC="${VH_PUBLIC:-1}"
VH_CROSSPLAY="${VH_CROSSPLAY:-false}"
VH_MEMORY_MAX="${VH_MEMORY_MAX:-14G}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"
FORCE_CONFIG_REWRITE="${FORCE_CONFIG_REWRITE:-0}"

show_help() {
    cat <<'EOF'
用法: sudo bash valheim-server-install.sh [--help]

安装或更新 Valheim 原版专用服务器。重复运行默认保留已有凭证、配置和世界。
环境变量:
  FORCE_CONFIG_REWRITE=1  重新生成 credentials.env（世界仍保留）
  NONINTERACTIVE=1        使用环境变量/默认值，不显示交互配置
  VH_USER=<用户>          服务账户；会持久化，管理更新不会硬编码账户
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
    VH_SERVER_DIR="${VH_SERVER_DIR:-${VH_DIR}/server}"
    VH_WORLD_DIR="${VH_WORLD_DIR:-${VH_DIR}/world}"
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
    if (( mem < 4 )); then warn "建议至少 4GB 内存"; fi
}
gen_password() {
    local p
    p=$(set +o pipefail; tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16)
    printf '%s\n' "${p:-vh$(date +%s)}"
}
user_config() {
    [[ -n "$VH_SERVER_PASSWORD" ]] || VH_SERVER_PASSWORD=$(gen_password)
    [[ "$FORCE_CONFIG_REWRITE" == "1" ]] && warn "FORCE_CONFIG_REWRITE=1：将重写凭证，世界仍会保留"
    if [[ "$NONINTERACTIVE" != "1" && ! -f "$CREDS_FILE" || "$NONINTERACTIVE" != "1" && "$FORCE_CONFIG_REWRITE" == "1" ]]; then
        echo -e "\n${CYAN}${BOLD}========== Valheim 服务器配置 ==========${NC}"
        printf '名称: %s\n世界: %s\n端口: %s\n公开: %s\n跨平台: %s\n账户: %s\n\n' "$VH_SERVER_NAME" "$VH_WORLD_NAME" "$VH_SERVER_PORT" "$VH_PUBLIC" "$VH_CROSSPLAY" "$VH_USER"
        local choice input
        read -rp "回车使用以上配置 / 输入 c 自定义: " choice
        if [[ "$choice" =~ ^[cC]$ ]]; then
            read -rp "服务器名称 [${VH_SERVER_NAME}]: " input; VH_SERVER_NAME="${input:-$VH_SERVER_NAME}"
            read -rp "服务器密码（至少 5 位，回车保留当前）: " input; VH_SERVER_PASSWORD="${input:-$VH_SERVER_PASSWORD}"
            read -rp "世界名称 [${VH_WORLD_NAME}]: " input; VH_WORLD_NAME="${input:-$VH_WORLD_NAME}"
            read -rp "游戏端口 [${VH_SERVER_PORT}]: " input; VH_SERVER_PORT="${input:-$VH_SERVER_PORT}"
            read -rp "公开服务器 0/1 [${VH_PUBLIC}]: " input; VH_PUBLIC="${input:-$VH_PUBLIC}"
            read -rp "跨平台 true/false [${VH_CROSSPLAY}]: " input; VH_CROSSPLAY="${input:-$VH_CROSSPLAY}"
        fi
    fi
    [[ "$VH_SERVER_PORT" =~ ^[0-9]+$ ]] && (( VH_SERVER_PORT >= 1 && VH_SERVER_PORT <= 65533 )) || { error "VH_SERVER_PORT 必须是 1-65533"; exit 1; }
    (( ${#VH_SERVER_PASSWORD} >= 5 )) || { error "Valheim 密码至少需要 5 个字符"; exit 1; }
    [[ "$VH_PUBLIC" =~ ^[01]$ ]] || { error "VH_PUBLIC 必须是 0 或 1"; exit 1; }
    [[ "$VH_CROSSPLAY" == true || "$VH_CROSSPLAY" == false ]] || { error "VH_CROSSPLAY 必须是 true 或 false"; exit 1; }
    VH_QUERY_PORT=$((VH_SERVER_PORT + 1))
}
install_deps() {
    info "安装 Debian 13/Ubuntu 运行依赖..."
    dpkg --add-architecture i386
    apt-get update -y
    apt-get install -y sudo curl ca-certificates tar gzip coreutils util-linux python3 nano lib32gcc-s1 libc6:i386 libstdc++6:i386 libatomic1 libpulse0 libpulse0:i386
}
install_steamcmd() {
    if ! command -v steamcmd &>/dev/null; then
        apt-get install -y steamcmd 2>/dev/null || {
            local dir=/opt/steamcmd archive
            archive=$(mktemp /tmp/steamcmd.XXXXXX.tar.gz); mkdir -p "$dir"
            curl -fL --connect-timeout 20 --retry 3 --max-time 300 -o "$archive" https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz
            tar -tzf "$archive" >/dev/null; tar -xzf "$archive" -C "$dir"; rm -f "$archive"
            chmod 755 "$dir/steamcmd.sh" "$dir/linux32/steamcmd" 2>/dev/null || true
            chown -R root:root "$dir"
            rm -f /usr/local/bin/steamcmd
            cat > /usr/local/bin/steamcmd <<'STEAMCMD_WRAPPER'
#!/bin/bash
exec /opt/steamcmd/steamcmd.sh "$@"
STEAMCMD_WRAPPER
            chmod 755 /usr/local/bin/steamcmd
        }
    fi
    [[ -x /usr/games/steamcmd ]] && ln -sf /usr/games/steamcmd /usr/local/bin/steamcmd
    command -v steamcmd &>/dev/null || { error "SteamCMD 安装失败"; exit 1; }
}
setup_user() {
    id "$VH_USER" &>/dev/null || useradd -m -r -s /bin/bash "$VH_USER"
    mkdir -p "$VH_DIR" "$VH_SERVER_DIR" "$VH_WORLD_DIR" "${VH_DIR}/backups" "${VH_SERVER_DIR}/logs"
    chown -R "${VH_USER}:${VH_USER}" "$VH_DIR"
}
write_credentials() {
    [[ -f "$CREDS_FILE" && "$FORCE_CONFIG_REWRITE" != "1" ]] && return
    mkdir -p "$(dirname "$CREDS_FILE")"; umask 077
    cat > "$CREDS_FILE" <<EOF
# Generated by valheim-server-install.sh
VH_USER=$(printf '%q' "$VH_USER")
VH_DIR=$(printf '%q' "$VH_DIR")
VH_SERVER_DIR=$(printf '%q' "$VH_SERVER_DIR")
VH_WORLD_DIR=$(printf '%q' "$VH_WORLD_DIR")
SERVICE_NAME=$(printf '%q' "$SERVICE_NAME")
VH_SERVER_NAME=$(printf '%q' "$VH_SERVER_NAME")
VH_SERVER_PORT=${VH_SERVER_PORT}
VH_QUERY_PORT=${VH_QUERY_PORT}
VH_WORLD_NAME=$(printf '%q' "$VH_WORLD_NAME")
VH_SERVER_PASSWORD=$(printf '%q' "$VH_SERVER_PASSWORD")
VH_PUBLIC=${VH_PUBLIC}
VH_CROSSPLAY=${VH_CROSSPLAY}
VH_MEMORY_MAX=$(printf '%q' "$VH_MEMORY_MAX")
EOF
    chown "root:${VH_USER}" "$CREDS_FILE"; chmod 640 "$CREDS_FILE"
}
download_server() {
    info "通过 SteamCMD 安装/验证 Valheim 原版服务器..."
    local retry=0
    while ! sudo -u "$VH_USER" steamcmd +force_install_dir "$VH_SERVER_DIR" +login anonymous +app_update 896660 validate +quit; do
        retry=$((retry + 1)); (( retry < 3 )) || { error "SteamCMD 下载失败"; exit 1; }; warn "下载失败，重试 ${retry}/3"
    done
    [[ -x "${VH_SERVER_DIR}/valheim_server.x86_64" ]] || { error "缺少 valheim_server.x86_64"; exit 1; }
}
create_start_script() {
    cat > "${VH_DIR}/start.sh" <<'EOF'
#!/bin/bash
set -euo pipefail
CREDS_FILE="/etc/valheim/credentials.env"
# shellcheck disable=SC1090
source "$CREDS_FILE"
cd "$VH_SERVER_DIR"
mkdir -p logs
export templdpath="${LD_LIBRARY_PATH:-}"
export LD_LIBRARY_PATH="./linux64:${LD_LIBRARY_PATH:-}"
export SteamAppId=892970
args=(-name "$VH_SERVER_NAME" -port "$VH_SERVER_PORT" -world "$VH_WORLD_NAME" -password "$VH_SERVER_PASSWORD" -public "$VH_PUBLIC" -savedir "$VH_WORLD_DIR" -logFile "$VH_SERVER_DIR/logs/valheim-$(date +%Y%m%d).log")
[[ "$VH_CROSSPLAY" == true ]] && args+=(-crossplay)
exec ./valheim_server.x86_64 "${args[@]}"
EOF
    chmod 750 "${VH_DIR}/start.sh"; chown "root:${VH_USER}" "${VH_DIR}/start.sh"
}
create_service() {
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Valheim Vanilla Dedicated Server
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=600
StartLimitBurst=5
[Service]
Type=simple
User=${VH_USER}
Group=${VH_USER}
WorkingDirectory=${VH_SERVER_DIR}
ExecStart=${VH_DIR}/start.sh
ExecStop=/bin/kill -SIGINT \$MAINPID
TimeoutStopSec=120
Restart=on-failure
RestartSec=10
MemoryMax=${VH_MEMORY_MAX}
OOMScoreAdjust=-500
ProtectSystem=strict
ReadWritePaths=${VH_DIR}
PrivateTmp=true
NoNewPrivileges=true
StandardOutput=journal
StandardError=journal
SyslogIdentifier=valheim
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
CREDS_FILE="/etc/valheim/credentials.env"
[[ -r "$CREDS_FILE" ]] || { echo "无法读取 $CREDS_FILE" >&2; exit 1; }
# shellcheck disable=SC1090
source "$CREDS_FILE"
SERVICE="$SERVICE_NAME"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
fail() { echo -e "${RED}$*${NC}" >&2; return 1; }
need_root() { [[ $EUID -eq 0 ]] || fail "此命令需要 root"; }
usage() {
 cat <<EOF
${CYAN}Valheim 原版服务器管理工具${NC}
用法: valheim-manager <命令> [参数]
  start | stop | restart | status | logs
  backup                   创建包含世界和列表文件的校验备份
  restore <latest|路径>     安全恢复备份
  update                   更新官方原版服务器并保留运行状态
  config                   编辑持久化凭证/启动配置
  world                    显示世界和 admin/banned/permitted 列表文件
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
  if p.startswith('/') or any(x=='..' for x in p.split('/')) or m.issym() or m.islnk() or m.isdev(): raise SystemExit(1)
PY
}
service_active() { systemctl is-active --quiet "$SERVICE"; }
backup_locked() {
    local tag="${1:-backup}" backup_dir="${VH_DIR}/backups" ts partial file payload was_active=false list
    mkdir -p "$backup_dir"; ts=$(date +%Y%m%d_%H%M%S); file="${backup_dir}/valheim_${tag}_${ts}.tar.gz"; partial="${file}.partial"; payload=$(mktemp -d)
    service_active && was_active=true; $was_active && systemctl stop "$SERVICE"
    trap '$was_active && systemctl start "$SERVICE" >/dev/null 2>&1 || true; rm -rf "$payload"; rm -f "$partial"' RETURN
    mkdir -p "$payload/data" "$payload/etc"
    [[ -d "$VH_WORLD_DIR" ]] && cp -a "$VH_WORLD_DIR" "$payload/data/world"
    for list in adminlist.txt bannedlist.txt permittedlist.txt; do [[ -f "${VH_WORLD_DIR}/${list}" ]] && cp -a "${VH_WORLD_DIR}/${list}" "$payload/data/"; done
    cp -a "$CREDS_FILE" "$payload/etc/credentials.env"
    tar -czf "$partial" -C "$payload" .; validate_archive "$partial" || fail "备份 tar 校验失败"
    mv "$partial" "$file"; sha256sum "$file" > "${file}.sha256"; chown "${VH_USER}:${VH_USER}" "$file" "${file}.sha256" 2>/dev/null || true
    rm -rf "$payload"; $was_active && systemctl start "$SERVICE"; trap - RETURN; printf '%s\n' "$file"
}
cmd_backup() {
    need_root; mkdir -p "${VH_DIR}/backups"; exec 9>"${VH_DIR}/backups/.backup.lock"; flock -n 9 || fail "另一个备份/恢复任务正在运行"
    local file; file=$(backup_locked backup) || return; echo -e "${GREEN}备份完成: ${file}${NC}"
}
resolve_restore_file() {
    if [[ "$1" == latest ]]; then python3 - "${VH_DIR}/backups" <<'PY'
import glob,os,sys
f=glob.glob(os.path.join(sys.argv[1],'valheim_*.tar.gz')); print(max(f,key=os.path.getmtime) if f else '')
PY
    else readlink -f -- "$1"; fi
}
cmd_restore() {
    need_root; [[ $# -eq 1 ]] || fail "用法: valheim-manager restore <latest|备份路径>"
    mkdir -p "${VH_DIR}/backups"; exec 9>"${VH_DIR}/backups/.backup.lock"; flock -n 9 || fail "另一个备份/恢复任务正在运行"
    local archive stage rollback pre was_active=false
    archive=$(resolve_restore_file "$1"); [[ -n "$archive" && -f "$archive" ]] || fail "备份不存在: $1"
    [[ -f "${archive}.sha256" ]] && (cd "$(dirname "$archive")" && sha256sum -c "$(basename "${archive}.sha256")") || [[ ! -f "${archive}.sha256" ]] || return 1
    validate_archive "$archive" || fail "备份不安全或已损坏"
    stage=$(mktemp -d "${VH_DIR}/restore.XXXXXX"); rollback=$(mktemp -d "${VH_DIR}/rollback.XXXXXX"); tar -xzf "$archive" --no-same-owner -C "$stage"
    [[ -d "$stage/data/world" && -f "$stage/etc/credentials.env" ]] || { rm -rf "$stage" "$rollback"; fail "备份缺少世界或凭证"; }
    service_active && was_active=true; pre=$(backup_locked pre_restore) || { rm -rf "$stage" "$rollback"; return 1; }; $was_active && systemctl stop "$SERVICE"
    trap 'rm -rf "$VH_WORLD_DIR"; [[ -d "$rollback/world" ]] && mv "$rollback/world" "$VH_WORLD_DIR"; [[ -f "$rollback/credentials.env" ]] && mv "$rollback/credentials.env" "$CREDS_FILE"; $was_active && systemctl start "$SERVICE" >/dev/null 2>&1 || true; rm -rf "$stage" "$rollback"' ERR
    [[ -d "$VH_WORLD_DIR" ]] && mv "$VH_WORLD_DIR" "$rollback/world"; cp -a "$CREDS_FILE" "$rollback/credentials.env"
    mv "$stage/data/world" "$VH_WORLD_DIR"; cp -a "$stage/etc/credentials.env" "$CREDS_FILE"
    chown -R "${VH_USER}:${VH_USER}" "$VH_WORLD_DIR"; chown "root:${VH_USER}" "$CREDS_FILE"; chmod 640 "$CREDS_FILE"
    if $was_active && ! systemctl start "$SERVICE"; then
        rm -rf "$VH_WORLD_DIR"; [[ -d "$rollback/world" ]] && mv "$rollback/world" "$VH_WORLD_DIR"
        [[ -f "$rollback/credentials.env" ]] && mv "$rollback/credentials.env" "$CREDS_FILE"
        systemctl start "$SERVICE" || true
        trap - ERR; rm -rf "$stage" "$rollback"; fail "恢复后的服务启动失败，已回滚"
    fi
    trap - ERR; rm -rf "$stage" "$rollback"
    echo -e "${GREEN}恢复完成；恢复前备份: ${pre}${NC}"
}
run_update() { sudo -u "$VH_USER" steamcmd +force_install_dir "$VH_SERVER_DIR" +login anonymous +app_update 896660 validate +quit; }
cmd_update() {
    need_root; local active=false retry=0; service_active && active=true; $active && systemctl stop "$SERVICE"
    while ! run_update; do retry=$((retry+1)); if (( retry >= 3 )); then $active && systemctl start "$SERVICE" || true; fail "SteamCMD 更新失败，原运行状态已恢复"; fi; echo "更新失败，重试 ${retry}/3"; done
    $active && systemctl start "$SERVICE"; echo -e "${GREEN}更新完成；服务状态已保持${NC}"
}
cmd_config() { [[ $# -eq 0 ]] || fail "config 不接受参数"; need_root; "${EDITOR:-nano}" "$CREDS_FILE"; echo "修改后请重启: valheim-manager restart"; }
cmd_world() {
    [[ $# -eq 0 ]] || fail "world 不接受参数"; echo -e "${CYAN}世界目录: ${VH_WORLD_DIR}${NC}"; du -sh "$VH_WORLD_DIR" 2>/dev/null || true
    python3 - "$VH_WORLD_DIR" <<'PY'
import glob,os,sys
patterns=('**/*.db','**/*.fwl','**/adminlist.txt','**/bannedlist.txt','**/permittedlist.txt')
seen=set()
for pat in patterns:
 for p in glob.glob(os.path.join(sys.argv[1],pat),recursive=True):
  if p not in seen: seen.add(p); print(f"{os.path.relpath(p,sys.argv[1])}\t{os.path.getsize(p)} bytes")
PY
}
cmd_memory() { systemctl show "$SERVICE" -p MemoryCurrent -p MemoryPeak -p MemoryMax --no-pager; ps -u "$VH_USER" -o pid,%cpu,%mem,rss,cmd --no-headers 2>/dev/null || true; }
cmd_info() { local ip; ip=$(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I | python3 -c 'import sys; print((sys.stdin.read().split() or ["unknown"])[0])'); printf '地址: %s:%s\n查询: %s:%s\n世界: %s\n账户: %s\n状态: %s\n配置: %s\n世界目录: %s\n' "$ip" "$VH_SERVER_PORT" "$ip" "$VH_QUERY_PORT" "$VH_WORLD_NAME" "$VH_USER" "$(systemctl is-active "$SERVICE" 2>/dev/null || true)" "$CREDS_FILE" "$VH_WORLD_DIR"; }
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
    chmod 750 "$MANAGER_SCRIPT"; chown "root:${VH_USER}" "$MANAGER_SCRIPT"
}
create_backup_timer() {
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" <<EOF
[Unit]
Description=Valheim validated backup
[Service]
Type=oneshot
User=root
ExecStart=${MANAGER_SCRIPT} backup
EOF
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" <<EOF
[Unit]
Description=Backup Valheim every 6 hours
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
    if command -v ufw &>/dev/null; then ufw allow "${VH_SERVER_PORT}:$((VH_SERVER_PORT+2))/udp" comment "Valheim Server"; else warn "请在云安全组/防火墙放行 UDP ${VH_SERVER_PORT}-$((VH_SERVER_PORT+2))"; fi
}
start_server() {
    local was_active=false; systemctl reset-failed "$SERVICE_NAME" 2>/dev/null || true; systemctl is-active --quiet "$SERVICE_NAME" && was_active=true
    if $was_active; then systemctl restart "$SERVICE_NAME"; else systemctl start "$SERVICE_NAME"; fi
    sleep 5; systemctl is-active --quiet "$SERVICE_NAME" || warn "服务尚未就绪，请查看: journalctl -u ${SERVICE_NAME} -n 100"
}
show_result() {
    echo -e "${GREEN}${BOLD}Valheim 原版服务器部署完成${NC}"
    echo "管理: valheim-manager start|stop|restart|status|logs|backup|restore|update|config|world|info|memory"
    echo "世界: ${VH_WORLD_DIR} | UDP: ${VH_SERVER_PORT}-$((VH_SERVER_PORT+2)) | 账户: ${VH_USER}"
}
main() {
    parse_args "$@"; check_root; load_existing_credentials; check_system; install_deps; check_resources; user_config
    if [[ "$NONINTERACTIVE" != "1" ]]; then local confirm; read -rp "回车开始部署 / 输入 n 取消: " confirm; [[ "$confirm" =~ ^[nN]$ ]] && exit 0; fi
    install_steamcmd; setup_user; write_credentials; download_server; create_start_script; create_service; create_manager; create_backup_timer; setup_firewall; start_server; show_result
}
main "$@"
