#!/bin/bash
#============================================================
# Valheim 专用服务器一键部署脚本
# 适用于: Ubuntu 22.04+ / Debian 11+
# SteamCMD AppID: 896660
#============================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ==================== 可配置变量 ====================
VH_USER="valheim"
VH_DIR="/opt/valheim"
VH_SERVER_DIR="${VH_DIR}/server"
VH_WORLD_DIR="${VH_DIR}/world"
SERVICE_NAME="valheim-server"
MANAGER_SCRIPT="/usr/local/bin/valheim-manager"

# 服务器配置
VH_SERVER_NAME="Valheim Server"
VH_SERVER_PORT=2456
VH_QUERY_PORT=2457
VH_WORLD_NAME="Dedicated"
VH_SERVER_PASSWORD="valheim"
VH_PUBLIC=1
VH_CROSSPLAY=false

# 性能
VH_MEMORY_MAX="14G"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# ==================== 系统检查 ====================
check_root() {
    [[ $EUID -ne 0 ]] && { error "请使用 root 运行"; exit 1; }
}

check_system() {
    [[ -f /etc/os-release ]] && . /etc/os-release || { error "无法检测系统"; exit 1; }
    info "系统: $PRETTY_NAME"
}

check_resources() {
    local cpu mem disk
    cpu=$(nproc)
    mem=$(awk '/MemTotal/ {printf "%.0f", $2/1024/1024}' /proc/meminfo)
    disk=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
    info "CPU: ${cpu}核 | 内存: ${mem}GB | 磁盘: ${disk}GB"
    [[ $mem -lt 4 ]] && warn "内存不足 4GB，建议至少 4GB"
}

# ==================== 用户配置 ====================
user_config() {
    echo -e "\n${CYAN}${BOLD}========== Valheim 服务器配置 ==========${NC}\n"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) 服务器名称:  ${VH_SERVER_NAME}"
    echo -e "  │  2) 服务器密码:  ${VH_SERVER_PASSWORD}"
    echo -e "  │  3) 世界名称:    ${VH_WORLD_NAME}"
    echo -e "  │  4) 游戏端口:    ${VH_SERVER_PORT}"
    echo -e "  │  5) 公开服务器:  ${VH_PUBLIC}"
    echo -e "  │  6) 跨平台:      ${VH_CROSSPLAY}"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    read -rp "  回车使用默认 / 输入 c 自定义: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        read -rp "  服务器名称 [${VH_SERVER_NAME}]: " input
        VH_SERVER_NAME="${input:-$VH_SERVER_NAME}"
        read -rp "  服务器密码 (至少5位) [${VH_SERVER_PASSWORD}]: " input
        VH_SERVER_PASSWORD="${input:-$VH_SERVER_PASSWORD}"
        read -rp "  世界名称 [${VH_WORLD_NAME}]: " input
        VH_WORLD_NAME="${input:-$VH_WORLD_NAME}"
        read -rp "  游戏端口 [${VH_SERVER_PORT}]: " input
        VH_SERVER_PORT="${input:-$VH_SERVER_PORT}"
        read -rp "  公开服务器 (0/1) [${VH_PUBLIC}]: " input
        VH_PUBLIC="${input:-$VH_PUBLIC}"
        read -rp "  跨平台 (true/false) [${VH_CROSSPLAY}]: " input
        VH_CROSSPLAY="${input:-$VH_CROSSPLAY}"
    fi

    # Valheim 密码至少5位
    if [[ ${#VH_SERVER_PASSWORD} -lt 5 ]]; then
        error "Valheim 服务器密码至少需要 5 个字符"
        exit 1
    fi

    VH_QUERY_PORT=$((VH_SERVER_PORT + 1))

    echo ""
    info "配置:"
    echo -e "    服务器名称:  ${CYAN}${VH_SERVER_NAME}${NC}"
    echo -e "    世界名称:    ${CYAN}${VH_WORLD_NAME}${NC}"
    echo -e "    游戏端口:    ${CYAN}${VH_SERVER_PORT}${NC} (查询: ${VH_QUERY_PORT})"
    echo ""
}

# ==================== 安装依赖 ====================
install_deps() {
    info "安装依赖..."
    apt-get update -y
    apt-get install -y curl wget lib32gcc-s1

    # Valheim 专用服务器需要额外的 32 位库
    dpkg --add-architecture i386 2>/dev/null || true
    apt-get update -y
    apt-get install -y libc6:i386 libstdc++6:i386 2>/dev/null || true
}

# ==================== 安装 SteamCMD ====================
install_steamcmd() {
    if command -v steamcmd &>/dev/null; then
        info "SteamCMD 已安装"
        return
    fi
    info "安装 SteamCMD..."
    apt-get install -y steamcmd 2>/dev/null || {
        local dir="/opt/steamcmd"
        mkdir -p "$dir"
        curl -sSL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | tar -xz -C "$dir"
        ln -sf "${dir}/steamcmd.sh" /usr/bin/steamcmd
    }
    [[ -f /usr/games/steamcmd ]] && ln -sf /usr/games/steamcmd /usr/bin/steamcmd
    info "SteamCMD 安装完成"
}

# ==================== 创建用户和目录 ====================
setup_user() {
    if ! id "$VH_USER" &>/dev/null; then
        useradd -m -r -s /bin/bash "$VH_USER"
    fi
    mkdir -p "$VH_DIR" "$VH_SERVER_DIR" "$VH_WORLD_DIR" "${VH_DIR}/backups"
    chown -R "${VH_USER}:${VH_USER}" "$VH_DIR"
}

# ==================== 下载服务器 ====================
download_server() {
    info "下载 Valheim 专用服务器 (AppID: 896660)..."
    info "这可能需要 10-20 分钟..."

    mkdir -p "$VH_SERVER_DIR"
    chown "${VH_USER}:${VH_USER}" "$VH_SERVER_DIR"

    local retry=0
    while ! sudo -u "$VH_USER" steamcmd +login anonymous +force_install_dir "$VH_SERVER_DIR" +app_update 896660 validate +quit; do
        retry=$((retry + 1))
        [[ $retry -ge 3 ]] && { error "下载失败"; exit 1; }
        warn "重试 ${retry}/3..."
        sleep 5
    done

    if [[ -f "${VH_SERVER_DIR}/valheim_server.x86_64" ]]; then
        info "下载完成"
    else
        error "服务器文件缺失"
        exit 1
    fi
}

# ==================== 创建启动脚本 ====================
create_start_script() {
    info "创建启动脚本..."

    cat > "${VH_DIR}/start.sh" << 'STARTSCRIPT'
#!/bin/bash
export templdpath=$LD_LIBRARY_PATH
export LD_LIBRARY_PATH=./linux64:$LD_LIBRARY_PATH
export SteamAppId=892970

SERVER_DIR="SERVER_DIR_PLACEHOLDER"
WORLD_DIR="WORLD_DIR_PLACEHOLDER"

cd "${SERVER_DIR}"

./valheim_server.x86_64 \
    -name "SERVER_NAME_PLACEHOLDER" \
    -port SERVER_PORT_PLACEHOLDER \
    -world "WORLD_NAME_PLACEHOLDER" \
    -password "SERVER_PASSWORD_PLACEHOLDER" \
    -public PUBLIC_PLACEHOLDER \
    CROSSPLAY_PLACEHOLDER \
    -savedir "${WORLD_DIR}" \
    -logFile "${SERVER_DIR}/logs/valheim-$(date +%Y%m%d).log"

export LD_LIBRARY_PATH=$templdpath
STARTSCRIPT

    sed -i "s|SERVER_DIR_PLACEHOLDER|${VH_SERVER_DIR}|g" "${VH_DIR}/start.sh"
    sed -i "s|WORLD_DIR_PLACEHOLDER|${VH_WORLD_DIR}|g" "${VH_DIR}/start.sh"
    sed -i "s|SERVER_NAME_PLACEHOLDER|${VH_SERVER_NAME}|g" "${VH_DIR}/start.sh"
    sed -i "s|SERVER_PORT_PLACEHOLDER|${VH_SERVER_PORT}|g" "${VH_DIR}/start.sh"
    sed -i "s|WORLD_NAME_PLACEHOLDER|${VH_WORLD_NAME}|g" "${VH_DIR}/start.sh"
    sed -i "s|SERVER_PASSWORD_PLACEHOLDER|${VH_SERVER_PASSWORD}|g" "${VH_DIR}/start.sh"
    sed -i "s|PUBLIC_PLACEHOLDER|${VH_PUBLIC}|g" "${VH_DIR}/start.sh"

    if [[ "$VH_CROSSPLAY" == "true" ]]; then
        sed -i "s|CROSSPLAY_PLACEHOLDER|-crossplay|g" "${VH_DIR}/start.sh"
    else
        sed -i "s|CROSSPLAY_PLACEHOLDER||g" "${VH_DIR}/start.sh"
    fi

    chmod +x "${VH_DIR}/start.sh"
    chown "${VH_USER}:${VH_USER}" "${VH_DIR}/start.sh"
    mkdir -p "${VH_SERVER_DIR}/logs"
    chown -R "${VH_USER}:${VH_USER}" "${VH_SERVER_DIR}/logs"
}

# ==================== 创建 systemd 服务 ====================
create_service() {
    info "创建 systemd 服务..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Valheim Dedicated Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${VH_USER}
Group=${VH_USER}
WorkingDirectory=${VH_SERVER_DIR}
ExecStart=${VH_DIR}/start.sh
ExecStop=/bin/kill -SIGINT \$MAINPID
Restart=on-failure
RestartSec=10
StartLimitIntervalSec=600
StartLimitBurst=5

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

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
}

# ==================== 创建管理脚本 ====================
create_manager() {
    info "创建管理脚本..."

    cat > "${MANAGER_SCRIPT}" << 'MANAGEREOF'
#!/bin/bash
SERVICE="valheim-server"
VH_DIR="/opt/valheim"
VH_SERVER_DIR="/opt/valheim/server"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

show_help() {
    echo -e "${CYAN}Valheim 服务器管理工具${NC}"
    echo ""
    echo "用法: valheim-manager <命令>"
    echo ""
    echo "命令:"
    echo "  start       启动服务器"
    echo "  stop        停止服务器"
    echo "  restart     重启服务器"
    echo "  status      查看状态"
    echo "  logs        实时日志"
    echo "  backup      立即备份"
    echo "  update      更新服务器"
    echo "  config      编辑启动脚本"
    echo "  memory      查看内存"
    echo "  info        服务器信息"
    echo ""
}

cmd_start()   { systemctl start "$SERVICE" && echo -e "${GREEN}已启动${NC}"; }
cmd_stop()    { systemctl stop "$SERVICE" && echo -e "${YELLOW}已停止${NC}"; }
cmd_restart() { systemctl restart "$SERVICE" && echo -e "${GREEN}已重启${NC}"; }
cmd_status()  { systemctl status "$SERVICE" --no-pager; }
cmd_logs()    { journalctl -u "$SERVICE" -f --no-pager; }

cmd_backup() {
    local backup_dir="${VH_DIR}/backups"
    local ts=$(date +%Y%m%d_%H%M%S)
    local file="${backup_dir}/valheim_backup_${ts}.tar.gz"
    mkdir -p "$backup_dir"
    systemctl stop "$SERVICE" 2>/dev/null
    sleep 3
    tar -czf "$file" -C "${VH_DIR}" world 2>/dev/null
    systemctl start "$SERVICE" 2>/dev/null
    [[ -f "$file" ]] && echo -e "${GREEN}备份: ${file}${NC}" || echo -e "${RED}备份失败${NC}"
    ls -t "${backup_dir}"/valheim_backup_*.tar.gz 2>/dev/null | tail -n +21 | xargs -r rm -f
}

cmd_update() {
    echo -e "${CYAN}更新中...${NC}"
    systemctl stop "$SERVICE"
    sudo -u valheim steamcmd +login anonymous +force_install_dir "$VH_SERVER_DIR" +app_update 896660 validate +quit
    systemctl start "$SERVICE"
    echo -e "${GREEN}更新完成${NC}"
}

cmd_config() { ${EDITOR:-nano} "${VH_DIR}/start.sh"; echo -e "${YELLOW}重启生效: valheim-manager restart${NC}"; }

cmd_memory() {
    echo -e "${CYAN}=== 内存 ===${NC}"
    systemctl show "$SERVICE" --property=MemoryCurrent --property=MemoryPeak 2>/dev/null || true
    ps aux | grep "[v]alheim_server" | awk '{printf "PID: %s  CPU: %s%%  RSS: %sMB\n", $2, $3, $6/1024}'
}

cmd_info() {
    local ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo -e "${CYAN}=== Valheim 服务器 ===${NC}"
    echo "地址:   ${ip}:2456"
    echo "状态:   $(systemctl is-active "$SERVICE")"
}

case "${1:-help}" in
    start) cmd_start ;; stop) cmd_stop ;; restart) cmd_restart ;;
    status) cmd_status ;; logs) cmd_logs ;; backup) cmd_backup ;;
    update) cmd_update ;; config) cmd_config ;; memory) cmd_memory ;;
    info) cmd_info ;; *) show_help ;;
esac
MANAGEREOF

    chmod +x "${MANAGER_SCRIPT}"
}

# ==================== 创建自动备份 ====================
create_backup_timer() {
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" << EOF
[Unit]
Description=Valheim Server Backup
[Service]
Type=oneshot
ExecStart=${MANAGER_SCRIPT} backup
User=${VH_USER}
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" << EOF
[Unit]
Description=Backup Valheim every 6 hours
[Timer]
OnCalendar=*-*-* 00,06,12,18:00:00
Persistent=true
RandomizedDelaySec=60
[Install]
WantedBy=timers.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}-backup.timer"
    systemctl start "${SERVICE_NAME}-backup.timer"
    info "每6小时自动备份已启用"
}

# ==================== 防火墙 ====================
setup_firewall() {
    if command -v ufw &>/dev/null; then
        ufw allow "${VH_SERVER_PORT}/udp" comment "Valheim Game"
        ufw allow "${VH_QUERY_PORT}/udp" comment "Valheim Query"
        info "已开放 UDP ${VH_SERVER_PORT}-${VH_QUERY_PORT}"
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${VH_SERVER_PORT}-${VH_QUERY_PORT}/udp"
        firewall-cmd --reload
    else
        warn "请手动开放 UDP ${VH_SERVER_PORT}-${VH_QUERY_PORT}"
    fi
}

# ==================== 启动 ====================
start_server() {
    info "启动 Valheim 服务器..."
    systemctl start "${SERVICE_NAME}"
    sleep 10
    systemctl is-active --quiet "${SERVICE_NAME}" && info "启动成功!" || warn "请等待初始化..."
}

# ==================== 结果 ====================
show_result() {
    local ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo -e "${GREEN}${BOLD}       Valheim 服务器部署完成!${NC}"
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo ""
    echo -e "  服务器地址:  ${CYAN}${ip}:${VH_SERVER_PORT}${NC}"
    echo -e "  服务器密码:  ${CYAN}${VH_SERVER_PASSWORD}${NC}"
    echo -e "  世界名称:    ${CYAN}${VH_WORLD_NAME}${NC}"
    echo ""
    echo -e "  管理命令:"
    echo -e "    valheim-manager start/stop/restart/status/logs/backup/update/config/info"
    echo ""
    echo -e "  ${RED}${BOLD}!!! 重要: 云服务器安全组配置 !!!${NC}"
    echo -e "  ${YELLOW}系统防火墙已自动放行，但云服务器还需在控制台配置安全组:${NC}"
    echo ""
    echo -e "  ┌──────────────┬──────────┬────────────────────────────┐"
    echo -e "  │    端口      │   协议   │         用途               │"
    echo -e "  ├──────────────┼──────────┼────────────────────────────┤"
    echo -e "  │    2456      │   UDP    │  游戏主端口 (必须)         │"
    echo -e "  │    2457      │   UDP    │  查询端口 (必须)           │"
    echo -e "  └──────────────┴──────────┴────────────────────────────┘"
    echo ""
    echo -e "  ${CYAN}配置方式:${NC}"
    echo -e "    腾讯云:  控制台 → 云服务器 → 安全组 → 添加入站规则"
    echo -e "    阿里云:  控制台 → ECS → 安全组 → 配置规则 → 入方向"
    echo -e "    棉花云:  控制台 → 云服务器 → 防火墙 → 添加规则"
    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
}

# ==================== 主流程 ====================
main() {
    echo -e "${CYAN}${BOLD}"
    echo "  ╦  ╦╔═╗╔═╗╦  ╦╔╦╗╔═╗"
    echo "  ╚╗╔╝╠═╣╠═╝║  ║ ║ ║╣ "
    echo "   ╚╝ ╩ ╩╩  ╩══╩ ╩ ╚═╝"
    echo -e "    Dedicated Server Installer${NC}"
    echo ""

    check_root
    check_system
    check_resources
    user_config

    echo -e "\n${CYAN}${BOLD}部署步骤:${NC}"
    echo "  [1] 安装依赖"
    echo "  [2] 安装 SteamCMD"
    echo "  [3] 创建用户和目录"
    echo "  [4] 下载服务器"
    echo "  [5] 创建启动脚本"
    echo "  [6] 创建 systemd 服务"
    echo "  [7] 创建管理脚本"
    echo "  [8] 创建自动备份"
    echo "  [9] 配置防火墙"
    echo "  [10] 启动服务器"
    echo ""
    echo ""
    read -rp "回车开始部署 / 输入 n 取消: " confirm
    [[ "$confirm" == "n" || "$confirm" == "N" ]] && { echo "已取消"; exit 0; }

    install_deps
    install_steamcmd
    setup_user
    download_server
    create_start_script
    create_service
    create_manager
    create_backup_timer
    setup_firewall
    start_server
    show_result
}

main "$@"
