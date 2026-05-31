#!/bin/bash
#============================================================
# Terraria 专用服务器一键部署脚本
# 适用于: Ubuntu 22.04+ / Debian 11+
# SteamCMD AppID: 105600
#============================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ==================== 可配置变量 ====================
TS_USER="terraria"
TS_DIR="/opt/terraria"
TS_SERVER_DIR="${TS_DIR}/server"
TS_WORLD_DIR="${TS_DIR}/world"
SERVICE_NAME="terraria-server"
MANAGER_SCRIPT="/usr/local/bin/terraria-manager"

# 服务器配置
TS_PORT=7777
TS_MAX_PLAYERS=8
TS_SERVER_NAME="Terraria Server"
TS_SERVER_PASSWORD=""
TS_WORLD_NAME="world"
TS_DIFFICULTY=1          # 0=普通 1=专家 2=大师 3=旅途
TS_SEED=""
TS_LANGUAGE="en-US"
TS_SECURE=false          # 安装作弊防护

# 内存
TS_MEMORY_MAX="4G"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# ==================== 系统检查 ====================
check_root() { [[ $EUID -ne 0 ]] && { error "请使用 root 运行"; exit 1; }; }

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
}

# ==================== 用户配置 ====================
user_config() {
    echo -e "\n${CYAN}${BOLD}========== Terraria 服务器配置 ==========${NC}\n"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) 服务器名称:  ${TS_SERVER_NAME}"
    echo -e "  │  2) 游戏端口:    ${TS_PORT}"
    echo -e "  │  3) 最大玩家数:  ${TS_MAX_PLAYERS}"
    echo -e "  │  4) 服务器密码:  (无密码)"
    echo -e "  │  5) 难度:        1=专家"
    echo -e "  │  6) 世界名称:    ${TS_WORLD_NAME}"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    read -rp "  回车使用默认 / 输入 c 自定义: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        read -rp "  服务器名称 [${TS_SERVER_NAME}]: " input
        TS_SERVER_NAME="${input:-$TS_SERVER_NAME}"
        read -rp "  游戏端口 [${TS_PORT}]: " input
        TS_PORT="${input:-$TS_PORT}"
        read -rp "  最大玩家数 [${TS_MAX_PLAYERS}]: " input
        TS_MAX_PLAYERS="${input:-$TS_MAX_PLAYERS}"
        read -rp "  服务器密码 (留空无密码): " input
        TS_SERVER_PASSWORD="${input:-$TS_SERVER_PASSWORD}"
        read -rp "  难度 (0=普通 1=专家 2=大师 3=旅途) [${TS_DIFFICULTY}]: " input
        TS_DIFFICULTY="${input:-$TS_DIFFICULTY}"
        read -rp "  世界名称 [${TS_WORLD_NAME}]: " input
        TS_WORLD_NAME="${input:-$TS_WORLD_NAME}"
        read -rp "  世界种子 (留空随机): " input
        TS_SEED="${input:-$TS_SEED}"
    fi

    echo ""
    info "配置:"
    echo -e "    服务器名称:  ${CYAN}${TS_SERVER_NAME}${NC}"
    echo -e "    游戏端口:    ${CYAN}${TS_PORT}${NC}"
    echo -e "    最大玩家数:  ${CYAN}${TS_MAX_PLAYERS}${NC}"
    echo -e "    世界名称:    ${CYAN}${TS_WORLD_NAME}${NC}"
    echo ""
}

# ==================== 安装依赖 ====================
install_deps() {
    info "安装依赖..."
    apt-get update -y
    apt-get install -y curl wget lib32gcc-s1 screen
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
    if ! id "$TS_USER" &>/dev/null; then
        useradd -m -r -s /bin/bash "$TS_USER"
    fi
    mkdir -p "$TS_DIR" "$TS_SERVER_DIR" "$TS_WORLD_DIR" "${TS_DIR}/backups"
    chown -R "${TS_USER}:${TS_USER}" "$TS_DIR"
}

# ==================== 下载服务器 ====================
download_server() {
    info "下载 Terraria 专用服务器 (AppID: 105600)..."

    mkdir -p "$TS_SERVER_DIR"
    chown "${TS_USER}:${TS_USER}" "$TS_SERVER_DIR"

    local retry=0
    while ! sudo -u "$TS_USER" steamcmd +login anonymous +force_install_dir "$TS_SERVER_DIR" +app_update 105600 validate +quit; do
        retry=$((retry + 1))
        [[ $retry -ge 3 ]] && { error "下载失败"; exit 1; }
        warn "重试 ${retry}/3..."
        sleep 5
    done

    # Terraria SteamCMD 安装的是 Windows 版本的服务器
    # Linux 需要从 terraria.org 下载原生版本
    if [[ ! -f "${TS_SERVER_DIR}/TerrariaServer" ]] && [[ ! -f "${TS_SERVER_DIR}/TerrariaServer.exe" ]]; then
        info "SteamCMD 安装的是 Windows 版，下载 Linux 原生版本..."

        # 获取最新版本号
        local latest_version
        latest_version=$(curl -sL --max-time 10 "https://terraria.org/api/get/download/1" 2>/dev/null | \
            python3 -c "import sys,json; print(json.load(sys.stdin).get('current',''))" 2>/dev/null)

        if [[ -z "$latest_version" ]]; then
            latest_version="1449"
        fi

        info "Terraria 服务器版本: ${latest_version}"

        # 下载 Linux 版本
        local dl_url="https://terraria.org/api/download/pc-dedicated-server/terraria-server-${latest_version}.zip"
        local tmp_zip="/tmp/terraria-server.zip"

        curl -sL --max-time 120 -o "$tmp_zip" "$dl_url" || {
            warn "从 terraria.org 下载失败，尝试使用 SteamCMD 安装的版本..."
        }

        if [[ -f "$tmp_zip" ]] && [[ $(stat -c%s "$tmp_zip" 2>/dev/null || echo 0) -gt 100000 ]]; then
            unzip -o "$tmp_zip" -d "$TS_SERVER_DIR" 2>/dev/null
            rm -f "$tmp_zip"
        fi
    fi

    # 查找服务器可执行文件
    local server_bin=""
    if [[ -f "${TS_SERVER_DIR}/TerrariaServer" ]]; then
        server_bin="${TS_SERVER_DIR}/TerrariaServer"
    elif [[ -f "${TS_SERVER_DIR}/TerrariaServer.bin.x86_64" ]]; then
        server_bin="${TS_SERVER_DIR}/TerrariaServer.bin.x86_64"
    elif [[ -f "${TS_SERVER_DIR}/Linux/TerrariaServer" ]]; then
        server_bin="${TS_SERVER_DIR}/Linux/TerrariaServer"
    elif [[ -f "${TS_SERVER_DIR}/TerrariaServer.exe" ]]; then
        # 需要 mono 来运行 Windows 版
        info "安装 Mono 运行时..."
        apt-get install -y mono-complete 2>/dev/null || apt-get install -y mono-runtime 2>/dev/null
        server_bin="${TS_SERVER_DIR}/TerrariaServer.exe"
    fi

    if [[ -z "$server_bin" ]]; then
        error "未找到 Terraria 服务器可执行文件"
        error "请检查 ${TS_SERVER_DIR} 目录"
        exit 1
    fi

    info "服务器可执行文件: ${server_bin}"
    chmod +x "$server_bin" 2>/dev/null
    chown -R "${TS_USER}:${TS_USER}" "$TS_DIR"
}

# ==================== 创建服务器配置文件 ====================
create_server_config() {
    info "创建服务器配置文件..."

    cat > "${TS_DIR}/serverconfig.txt" << EOF
# Terraria Server Configuration

# 世界设置
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

# 性能设置
upnp=1
npcstream=60

# MOTD
motd=Welcome to ${TS_SERVER_NAME}!
EOF

    chown "${TS_USER}:${TS_USER}" "${TS_DIR}/serverconfig.txt"
}

# ==================== 创建启动脚本 ====================
create_start_script() {
    info "创建启动脚本..."

    local server_bin=""
    local use_mono=false

    if [[ -f "${TS_SERVER_DIR}/TerrariaServer" ]]; then
        server_bin="${TS_SERVER_DIR}/TerrariaServer"
    elif [[ -f "${TS_SERVER_DIR}/TerrariaServer.bin.x86_64" ]]; then
        server_bin="${TS_SERVER_DIR}/TerrariaServer.bin.x86_64"
    elif [[ -f "${TS_SERVER_DIR}/Linux/TerrariaServer" ]]; then
        server_bin="${TS_SERVER_DIR}/Linux/TerrariaServer"
    else
        server_bin="${TS_SERVER_DIR}/TerrariaServer.exe"
        use_mono=true
    fi

    cat > "${TS_DIR}/start.sh" << STARTSCRIPT
#!/bin/bash
cd "${TS_DIR}"

SERVER_BIN="${server_bin}"
CONFIG="${TS_DIR}/serverconfig.txt"
USE_MONO=${use_mono}

if \$USE_MONO; then
    exec mono "\$SERVER_BIN" -config "\$CONFIG"
else
    exec "\$SERVER_BIN" -config "\$CONFIG"
fi
STARTSCRIPT

    chmod +x "${TS_DIR}/start.sh"
    chown "${TS_USER}:${TS_USER}" "${TS_DIR}/start.sh"
}

# ==================== 创建 systemd 服务 ====================
create_service() {
    info "创建 systemd 服务..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Terraria Dedicated Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${TS_USER}
Group=${TS_USER}
WorkingDirectory=${TS_DIR}
ExecStart=${TS_DIR}/start.sh
ExecStop=/bin/kill -SIGINT \$MAINPID
Restart=on-failure
RestartSec=10
StartLimitIntervalSec=600
StartLimitBurst=5

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

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
}

# ==================== 创建管理脚本 ====================
create_manager() {
    info "创建管理脚本..."

    cat > "${MANAGER_SCRIPT}" << 'MANAGEREOF'
#!/bin/bash
SERVICE="terraria-server"
TS_DIR="/opt/terraria"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

show_help() {
    echo -e "${CYAN}Terraria 服务器管理工具${NC}"
    echo ""
    echo "用法: terraria-manager <命令>"
    echo ""
    echo "命令:"
    echo "  start       启动服务器"
    echo "  stop        停止服务器"
    echo "  restart     重启服务器"
    echo "  status      查看状态"
    echo "  logs        实时日志"
    echo "  console     进入控制台"
    echo "  backup      立即备份"
    echo "  config      编辑配置"
    echo "  world       世界管理"
    echo "  memory      查看内存"
    echo "  info        服务器信息"
    echo ""
}

cmd_start()   { systemctl start "$SERVICE" && echo -e "${GREEN}已启动${NC}"; }
cmd_stop()    { systemctl stop "$SERVICE" && echo -e "${YELLOW}已停止${NC}"; }
cmd_restart() { systemctl restart "$SERVICE" && echo -e "${GREEN}已重启${NC}"; }
cmd_status()  { systemctl status "$SERVICE" --no-pager; }
cmd_logs()    { journalctl -u "$SERVICE" -f --no-pager; }

cmd_console() {
    echo -e "${YELLOW}进入控制台 (Ctrl+A D 退出)${NC}"
    screen -r terraria 2>/dev/null || echo "未在 screen 中运行"
}

cmd_backup() {
    local backup_dir="${TS_DIR}/backups"
    local ts=$(date +%Y%m%d_%H%M%S)
    local file="${backup_dir}/terraria_backup_${ts}.tar.gz"
    mkdir -p "$backup_dir"
    systemctl stop "$SERVICE" 2>/dev/null
    sleep 3
    tar -czf "$file" -C "${TS_DIR}" world 2>/dev/null
    systemctl start "$SERVICE" 2>/dev/null
    [[ -f "$file" ]] && echo -e "${GREEN}备份: ${file}${NC}" || echo -e "${RED}备份失败${NC}"
    ls -t "${backup_dir}"/terraria_backup_*.tar.gz 2>/dev/null | tail -n +21 | xargs -r rm -f
}

cmd_config() { ${EDITOR:-nano} "${TS_DIR}/serverconfig.txt"; echo -e "${YELLOW}重启生效${NC}"; }

cmd_world() {
    echo "世界文件: ${TS_DIR}/world/"
    ls -lh "${TS_DIR}/world/" 2>/dev/null || echo "无世界文件"
    echo ""
    echo "修改世界: 编辑 serverconfig.txt 中的 world 参数"
}

cmd_memory() {
    echo -e "${CYAN}=== 内存 ===${NC}"
    systemctl show "$SERVICE" --property=MemoryCurrent --property=MemoryPeak 2>/dev/null || true
    ps aux | grep "[T]errariaServer" | awk '{printf "PID: %s  CPU: %s%%  RSS: %sMB\n", $2, $3, $6/1024}'
}

cmd_info() {
    local ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo -e "${CYAN}=== Terraria 服务器 ===${NC}"
    echo "地址:   ${ip}:7777"
    echo "状态:   $(systemctl is-active "$SERVICE")"
    echo "配置:   ${TS_DIR}/serverconfig.txt"
}

case "${1:-help}" in
    start) cmd_start ;; stop) cmd_stop ;; restart) cmd_restart ;;
    status) cmd_status ;; logs) cmd_logs ;; console) cmd_console ;;
    backup) cmd_backup ;; config) cmd_config ;; world) cmd_world ;;
    memory) cmd_memory ;; info) cmd_info ;; *) show_help ;;
esac
MANAGEREOF

    chmod +x "${MANAGER_SCRIPT}"
}

# ==================== 自动备份 ====================
create_backup_timer() {
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" << EOF
[Unit]
Description=Terraria Server Backup
[Service]
Type=oneshot
ExecStart=${MANAGER_SCRIPT} backup
User=${TS_USER}
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" << EOF
[Unit]
Description=Backup Terraria every 6 hours
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
        ufw allow "${TS_PORT}/tcp" comment "Terraria Server"
        info "已开放 TCP ${TS_PORT}"
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${TS_PORT}/tcp"
        firewall-cmd --reload
    else
        warn "请手动开放 TCP ${TS_PORT}"
    fi
}

# ==================== 启动 ====================
start_server() {
    info "启动 Terraria 服务器..."
    systemctl start "${SERVICE_NAME}"
    sleep 8
    systemctl is-active --quiet "${SERVICE_NAME}" && info "启动成功!" || warn "请等待初始化..."
}

# ==================== 结果 ====================
show_result() {
    local ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo -e "${GREEN}${BOLD}       Terraria 服务器部署完成!${NC}"
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo ""
    echo -e "  服务器地址:  ${CYAN}${ip}:${TS_PORT}${NC}"
    echo -e "  配置文件:    ${TS_DIR}/serverconfig.txt"
    echo -e "  世界目录:    ${TS_DIR}/world/"
    echo ""
    echo -e "  管理命令:"
    echo -e "    terraria-manager start/stop/restart/status/logs/console/backup/config/info"
    echo ""
    echo -e "  ${RED}${BOLD}!!! 重要: 云服务器安全组配置 !!!${NC}"
    echo -e "  ${YELLOW}系统防火墙已自动放行，但云服务器还需在控制台配置安全组:${NC}"
    echo ""
    echo -e "  ┌──────────────┬──────────┬────────────────────────────┐"
    echo -e "  │    端口      │   协议   │         用途               │"
    echo -e "  ├──────────────┼──────────┼────────────────────────────┤"
    echo -e "  │    7777      │   TCP    │  游戏主端口 (必须)         │"
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
    echo "  ╔╦╗╔═╗╔═╗╔═╗╔═╗╦═╗╔╦╗"
    echo "   ║ ║╣ ╠═╣╚═╗║╣ ╠╦╝║║║"
    echo "   ╩ ╩ ╩╩ ╩╚═╝╚═╝╩╚═╩ ╩"
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
    echo "  [5] 创建配置文件"
    echo "  [6] 创建启动脚本"
    echo "  [7] 创建 systemd 服务"
    echo "  [8] 创建管理脚本"
    echo "  [9] 创建自动备份"
    echo "  [10] 配置防火墙"
    echo "  [11] 启动服务器"
    echo ""
    read -rp "确认部署? (y/N): " confirm
    [[ "$confirm" != "y" && "$confirm" != "Y" ]] && { echo "已取消"; exit 0; }

    install_deps
    install_steamcmd
    setup_user
    download_server
    create_server_config
    create_start_script
    create_service
    create_manager
    create_backup_timer
    setup_firewall
    start_server
    show_result
}

main "$@"
