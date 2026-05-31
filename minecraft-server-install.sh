#!/bin/bash
#============================================================
# Minecraft Java Edition 专用服务器一键部署脚本
# 支持: 官方原版 / Paper (高性能分支)
# 适用于: Ubuntu 22.04+ / Debian 11+
#============================================================

set -euo pipefail

# ==================== 颜色定义 ====================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ==================== 可配置变量 ====================
MC_USER="minecraft"
MC_DIR="/opt/minecraft"
MC_WORLD_DIR="${MC_DIR}/world"
SERVICE_NAME="mc-server"
MANAGER_SCRIPT="/usr/local/bin/mc-manager"

# 服务器类型: vanilla (官方原版) 或 paper (高性能)
SERVER_TYPE="paper"

# 内存配置 (根据玩家数量调整)
MC_MEMORY="4G"         # JVM 最大内存
MC_MEMORY_MIN="1G"     # JVM 最小内存

# 服务器配置
MC_PORT=25565
MC_MAX_PLAYERS=20
MC_MOTD="\\u00a7aMinecraft Server \\u00a77- Powered by Auto Installer"
MC_DIFFICULTY="normal"
MC_GAMEMODE="survival"
MC_VIEW_DISTANCE=10
MC_SIMULATION_DISTANCE=4
MC_ONLINE_MODE="true"
MC_PVP="true"
MC_SEED=""
MC_LEVEL_NAME="world"
MC_RCON_PORT=25575
MC_RCON_PASSWORD=""

# JVM 优化参数 (Aikar's Flags - 推荐)
# 参考: https://aikar.co/2018/07/02/tuning-the-jvm-g1gc-garbage-collector-flags-for-minecraft/
JVM_FLAGS=(
    "-XX:+UseG1GC"
    "-XX:+ParallelRefProcEnabled"
    "-XX:MaxGCPauseMillis=200"
    "-XX:+UnlockExperimentalVMOptions"
    "-XX:+DisableExplicitGC"
    "-XX:+AlwaysPreTouch"
    "-XX:G1NewSizePercent=30"
    "-XX:G1MaxNewSizePercent=40"
    "-XX:G1HeapRegionSize=8M"
    "-XX:G1ReservePercent=20"
    "-XX:G1HeapWastePercent=5"
    "-XX:G1MixedGCCountTarget=4"
    "-XX:InitiatingHeapOccupancyPercent=15"
    "-XX:G1MixedGCLiveThresholdPercent=90"
    "-XX:G1RSetUpdatingPauseTimePercent=5"
    "-XX:SurvivorRatio=32"
    "-XX:+PerfDisableSharedMem"
    "-XX:MaxTenuringThreshold=1"
    "-Dusing.aikars.flags=https://mcflags.emc.gs"
    "-Daikars.new.flags=true"
)

# 日志
info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# ==================== 系统检查 ====================
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 用户运行此脚本: sudo bash $0"
        exit 1
    fi
}

check_system() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
    else
        error "无法检测操作系统"
        exit 1
    fi
    info "检测到系统: $PRETTY_NAME"
}

check_resources() {
    local cpu_cores mem_total disk_free
    cpu_cores=$(nproc)
    mem_total=$(awk '/MemTotal/ {printf "%.0f", $2/1024/1024}' /proc/meminfo)
    disk_free=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')

    info "CPU: ${cpu_cores}核 | 内存: ${mem_total}GB | 磁盘: ${disk_free}GB"

    if [[ $mem_total -lt 4 ]]; then
        warn "内存不足 4GB，建议至少 4GB"
    fi
}

# ==================== 用户配置 ====================
user_config() {
    echo -e "\n${CYAN}${BOLD}========== 服务器配置 ==========${NC}\n"

    echo -e "  ${CYAN}选择服务器类型:${NC}"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) Paper (推荐) - 高性能分支，插件支持      │"
    echo -e "  │  2) Vanilla  - 官方原版，纯净体验            │"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    read -rp "  请选择 [1/2, 默认1]: " type_choice

    case "${type_choice:-1}" in
        2) SERVER_TYPE="vanilla" ;;
        *) SERVER_TYPE="paper" ;;
    esac

    echo -e "\n  ${CYAN}当前默认配置:${NC}"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) 服务器类型:  ${SERVER_TYPE}"
    echo -e "  │  2) 游戏端口:    ${MC_PORT}"
    echo -e "  │  3) 最大玩家数:  ${MC_MAX_PLAYERS}"
    echo -e "  │  4) JVM 内存:    ${MC_MEMORY}"
    echo -e "  │  5) 游戏模式:    ${MC_GAMEMODE}"
    echo -e "  │  6) 难度:        ${MC_DIFFICULTY}"
    echo -e "  │  7) 视距:        ${MC_VIEW_DISTANCE}"
    echo -e "  │  8) MOTD:        (服务器列表显示名称)"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    echo -e "  直接回车使用默认值，或输入 ${YELLOW}c${NC} 自定义"
    echo ""
    read -rp "  请选择 [回车=默认 / c=自定义]: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        echo ""
        read -rp "  游戏端口 [${MC_PORT}]: " input
        MC_PORT="${input:-$MC_PORT}"

        read -rp "  最大玩家数 [${MC_MAX_PLAYERS}]: " input
        MC_MAX_PLAYERS="${input:-$MC_MAX_PLAYERS}"

        read -rp "  JVM 内存 (如 4G, 8G) [${MC_MEMORY}]: " input
        MC_MEMORY="${input:-$MC_MEMORY}"

        read -rp "  游戏模式 (survival/creative/adventure/spectator) [${MC_GAMEMODE}]: " input
        MC_GAMEMODE="${input:-$MC_GAMEMODE}"

        read -rp "  难度 (peaceful/easy/normal/hard) [${MC_DIFFICULTY}]: " input
        MC_DIFFICULTY="${input:-$MC_DIFFICULTY}"

        read -rp "  视距 [${MC_VIEW_DISTANCE}]: " input
        MC_VIEW_DISTANCE="${input:-$MC_VIEW_DISTANCE}"

        read -rp "  服务器种子 (留空随机): " input
        MC_SEED="${input:-$MC_SEED}"

        read -rp "  正版验证 (true/false) [${MC_ONLINE_MODE}]: " input
        MC_ONLINE_MODE="${input:-$MC_ONLINE_MODE}"
    fi

    # 自动生成 RCON 密码
    MC_RCON_PASSWORD=$(head -c 16 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 16)
    MC_RCON_PASSWORD="${MC_RCON_PASSWORD:-mc$(date +%s)}"

    echo ""
    info "最终配置:"
    echo -e "    服务器类型:  ${CYAN}${SERVER_TYPE}${NC}"
    echo -e "    游戏端口:    ${CYAN}${MC_PORT}${NC}"
    echo -e "    最大玩家数:  ${CYAN}${MC_MAX_PLAYERS}${NC}"
    echo -e "    JVM 内存:    ${CYAN}${MC_MEMORY}${NC}"
    echo -e "    游戏模式:    ${CYAN}${MC_GAMEMODE}${NC}"
    echo -e "    难度:        ${CYAN}${MC_DIFFICULTY}${NC}"
    echo -e "    RCON 端口:   ${CYAN}${MC_RCON_PORT}${NC}"
    echo ""
}

# ==================== 安装 Java ====================
install_java() {
    info "检查 Java 环境..."

    # Minecraft 1.20.5+ 需要 Java 21
    local required_java=21

    if command -v java &>/dev/null; then
        local java_version
        java_version=$(java -version 2>&1 | head -1 | grep -oP '\d+' | head -1)
        if [[ $java_version -ge $required_java ]]; then
            info "Java ${java_version} 已安装，满足要求"
            return
        else
            warn "Java ${java_version} 版本过低，需要 ${required_java}+"
        fi
    fi

    info "安装 Java ${required_java}..."

    case $OS in
        ubuntu|debian)
            apt-get update -y
            apt-get install -y openjdk-${required_java}-jre-headless
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y java-${required_java}-openjdk
            ;;
        *)
            apt-get update -y
            apt-get install -y openjdk-${required_java}-jre-headless || \
            apt-get install -y default-jre-headless
            ;;
    esac

    if command -v java &>/dev/null; then
        info "Java 安装完成: $(java -version 2>&1 | head -1)"
    else
        error "Java 安装失败"
        exit 1
    fi
}

# ==================== 创建用户和目录 ====================
setup_user_and_dir() {
    info "创建用户和目录..."

    if ! id "$MC_USER" &>/dev/null; then
        useradd -m -r -s /bin/bash "$MC_USER"
        info "用户 $MC_USER 创建完成"
    fi

    mkdir -p "$MC_DIR" "$MC_WORLD_DIR" "${MC_DIR}/logs" "${MC_DIR}/plugins" "${MC_DIR}/backups"
    chown -R "${MC_USER}:${MC_USER}" "$MC_DIR"
}

# ==================== 下载服务器 ====================
download_server() {
    info "下载 ${SERVER_TYPE} 服务器..."

    local jar_file=""

    case $SERVER_TYPE in
        paper)
            # Paper API: https://api.papermc.io/v2/projects/paper
            # 获取最新版本和构建号
            local paper_version paper_build paper_url

            # 获取 Paper 支持的最新 MC 版本
            paper_version=$(curl -sL --max-time 15 "https://api.papermc.io/v2/projects/paper" 2>/dev/null | \
                python3 -c "import sys,json; d=json.load(sys.stdin); print(d['versions'][-1])" 2>/dev/null)

            if [[ -z "$paper_version" ]]; then
                warn "无法通过 API 获取 Paper 版本，使用备用方式..."
                # 备用: 直接下载最新构建
                paper_version="1.21.4"
            fi

            # 获取该版本最新构建号
            paper_build=$(curl -sL --max-time 15 "https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds" 2>/dev/null | \
                python3 -c "import sys,json; d=json.load(sys.stdin); builds=[b for b in d['builds'] if b['channel']=='default']; print(builds[-1]['build'])" 2>/dev/null)

            if [[ -z "$paper_build" ]]; then
                warn "无法获取构建号，尝试使用 latest..."
                paper_url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/latest/downloads/paper-${paper_version}-latest.jar"
            else
                paper_url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/${paper_build}/downloads/paper-${paper_version}-${paper_build}.jar"
            fi

            info "Paper 版本: ${paper_version}, 构建: ${paper_build:-latest}"
            info "下载地址: ${paper_url}"

            jar_file="${MC_DIR}/paper.jar"
            curl -sL --max-time 120 -o "$jar_file" "$paper_url" || {
                error "Paper 下载失败，请检查网络"
                error "手动下载: ${paper_url}"
                exit 1
            }
            ;;

        vanilla)
            # 获取最新版本信息
            local version_json_url server_jar_url
            version_json_url=$(curl -sL --max-time 15 "https://launchermeta.mojang.com/mc/game/version_manifest.json" 2>/dev/null | \
                python3 -c "
import sys,json
d=json.load(sys.stdin)
latest=d['latest']['release']
for v in d['versions']:
    if v['id']==latest:
        print(v['url'])
        break
" 2>/dev/null)

            if [[ -n "$version_json_url" ]]; then
                server_jar_url=$(curl -sL --max-time 15 "$version_json_url" 2>/dev/null | \
                    python3 -c "import sys,json; d=json.load(sys.stdin); print(d['downloads']['server']['url'])" 2>/dev/null)
            fi

            if [[ -z "$server_jar_url" ]]; then
                error "无法获取原版服务器下载地址"
                error "请手动从 https://www.minecraft.net/en-us/download/server 下载"
                exit 1
            fi

            info "原版服务器下载地址: ${server_jar_url}"
            jar_file="${MC_DIR}/server.jar"
            curl -sL --max-time 120 -o "$jar_file" "$server_jar_url" || {
                error "下载失败"
                exit 1
            }
            ;;
    esac

    if [[ -f "$jar_file" ]] && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 1000000 ]]; then
        info "服务器下载完成: $jar_file ($(du -h "$jar_file" | cut -f1))"
    else
        error "服务器文件下载失败或文件异常"
        exit 1
    fi

    chown "${MC_USER}:${MC_USER}" "$jar_file"
}

# ==================== 首次启动生成配置文件 ====================
generate_configs() {
    info "生成配置文件..."

    # 同意 EULA
    cat > "${MC_DIR}/eula.txt" << EOF
eula=true
EOF

    # 首次启动以生成配置文件
    info "首次启动以生成默认配置..."
    cd "$MC_DIR"

    local jar_file
    if [[ "$SERVER_TYPE" == "paper" ]]; then
        jar_file="paper.jar"
    else
        jar_file="server.jar"
    fi

    sudo -u "$MC_USER" timeout 60 java -Xms${MC_MEMORY_MIN} -Xmx${MC_MEMORY} -jar "$jar_file" --nogui || true

    # 等待配置文件生成
    local wait_count=0
    while [[ ! -f "${MC_DIR}/server.properties" ]] && [[ $wait_count -lt 30 ]]; do
        sleep 2
        wait_count=$((wait_count + 1))
    done

    # 停止服务器
    pkill -f "java.*${jar_file}" 2>/dev/null || true
    sleep 3

    chown -R "${MC_USER}:${MC_USER}" "$MC_DIR"
}

# ==================== 配置服务器 ====================
configure_server() {
    info "写入优化配置..."

    local props_file="${MC_DIR}/server.properties"

    # 备份原始配置
    if [[ -f "$props_file" ]]; then
        cp "$props_file" "${props_file}.bak.$(date +%Y%m%d%H%M%S)"
    fi

    # 写入 server.properties
    cat > "$props_file" << EOF
# Minecraft Server Configuration
# Generated by Auto Installer

# ========== 网络 ==========
server-port=${MC_PORT}
server-ip=
query.port=${MC_PORT}
enable-rcon=true
rcon.port=${MC_RCON_PORT}
rcon.password=${MC_RCON_PASSWORD}
enable-query=false

# ========== 服务器 ==========
motd=${MC_MOTD}
max-players=${MC_MAX_PLAYERS}
online-mode=${MC_ONLINE_MODE}
white-list=false
enforce-whitelist=false
enable-command-block=false
allow-flight=false
max-tick-time=60000
max-world-size=29999984
network-compression-threshold=256
rate-limit=0

# ========== 游戏 ==========
gamemode=${MC_GAMEMODE}
difficulty=${MC_DIFFICULTY}
hardcore=false
pvp=${MC_PVP}
spawn-protection=16
allow-nether=true
generate-structures=true
spawn-npcs=true
spawn-animals=true
spawn-monsters=true

# ========== 世界 ==========
level-name=${MC_LEVEL_NAME}
level-seed=${MC_SEED}
level-type=minecraft\:normal

# ========== 性能优化 ==========
view-distance=${MC_VIEW_DISTANCE}
simulation-distance=${MC_SIMULATION_DISTANCE}
sync-chunk-writes=true
entity-broadcast-range-percentage=100
max-chained-neighbor-updates=10000

# ========== 日志 ==========
debug=false
broadcast-console-to-ops=true
broadcast-rcon-to-ops=true
log-ips=true
function-permission-level=2
EOF

    info "server.properties 配置完成"

    # Paper 专用优化配置
    if [[ "$SERVER_TYPE" == "paper" ]]; then
        # paper-global.yml 优化
        local paper_global="${MC_DIR}/config/paper-global.yml"
        if [[ -f "$paper_global" ]]; then
            info "检测到 Paper 配置文件，建议手动优化: ${paper_global}"
        fi

        # bukkit.yml 优化
        local bukkit_yml="${MC_DIR}/bukkit.yml"
        if [[ -f "$bukkit_yml" ]]; then
            # 降低 chunk 加载距离以节省内存
            sed -i "s/chunk-load-range:.*/chunk-load-range: ${MC_VIEW_DISTANCE}/" "$bukkit_yml" 2>/dev/null || true
        fi
    fi

    chown -R "${MC_USER}:${MC_USER}" "$MC_DIR"
}

# ==================== 创建启动脚本 ====================
create_start_script() {
    info "创建启动脚本..."

    local jar_file
    if [[ "$SERVER_TYPE" == "paper" ]]; then
        jar_file="paper.jar"
    else
        jar_file="server.jar"
    fi

    cat > "${MC_DIR}/start.sh" << STARTSCRIPT
#!/bin/bash
cd "${MC_DIR}"

exec java \\
    -Xms${MC_MEMORY_MIN} \\
    -Xmx${MC_MEMORY} \\
    ${JVM_FLAGS[*]} \\
    -jar ${jar_file} --nogui
STARTSCRIPT

    chmod +x "${MC_DIR}/start.sh"
    chown "${MC_USER}:${MC_USER}" "${MC_DIR}/start.sh"
    info "启动脚本: ${MC_DIR}/start.sh"
}

# ==================== 创建 systemd 服务 ====================
create_systemd_service() {
    info "创建 systemd 服务..."

    # 计算内存限制 (JVM 内存 + 1G 余量)
    local mem_limit
    mem_limit=$(echo "$MC_MEMORY" | sed 's/G//')
    mem_limit=$((mem_limit + 1))G

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Minecraft Java Edition Server (${SERVER_TYPE})
Documentation=https://www.minecraft.net/en-us/download/server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${MC_USER}
Group=${MC_USER}
WorkingDirectory=${MC_DIR}
ExecStart=${MC_DIR}/start.sh
ExecStop=/usr/bin/screen -S mc-server -X stuff "stop\n"

Restart=on-failure
RestartSec=10
StartLimitIntervalSec=600
StartLimitBurst=5

MemoryMax=${mem_limit}
MemoryHigh=${mem_limit}
OOMScoreAdjust=-500

ProtectSystem=strict
ReadWritePaths=${MC_DIR}
PrivateTmp=true
NoNewPrivileges=true

StandardOutput=journal
StandardError=journal
SyslogIdentifier=minecraft

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    info "systemd 服务创建完成 (MemoryMax=${mem_limit})"
}

# ==================== 创建管理脚本 ====================
create_manager_script() {
    info "创建管理脚本..."

    cat > "${MANAGER_SCRIPT}" << 'MANAGEREOF'
#!/bin/bash
# Minecraft 服务器管理脚本

SERVICE="mc-server"
MC_DIR="/opt/minecraft"
RCON_PORT=25575
RCON_PASSWORD="RCON_PASS_PLACEHOLDER"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

show_help() {
    echo -e "${CYAN}Minecraft 服务器管理工具${NC}"
    echo ""
    echo "用法: mc-manager <命令>"
    echo ""
    echo "命令:"
    echo "  start       启动服务器"
    echo "  stop        停止服务器"
    echo "  restart     重启服务器"
    echo "  status      查看状态"
    echo "  logs        实时日志"
    echo "  console     进入服务器控制台 (Ctrl+A D 退出)"
    echo "  cmd <命令>  执行服务器命令"
    echo "  players     查看在线玩家"
    echo "  say <消息>  广播消息"
    echo "  whitelist   白名单管理"
    echo "  backup      立即备份"
    echo "  update      更新服务器"
    echo "  config      编辑配置"
    echo "  memory      查看内存"
    echo "  info        服务器信息"
    echo ""
}

cmd_start()   { systemctl start "$SERVICE" && echo -e "${GREEN}服务器已启动${NC}"; }
cmd_stop()    { systemctl stop "$SERVICE" && echo -e "${YELLOW}服务器已停止${NC}"; }
cmd_restart() { systemctl restart "$SERVICE" && echo -e "${GREEN}服务器已重启${NC}"; }
cmd_status()  { systemctl status "$SERVICE" --no-pager; }
cmd_logs()    { journalctl -u "$SERVICE" -f --no-pager; }

cmd_console() {
    echo -e "${YELLOW}进入服务器控制台，按 Ctrl+A 然后按 D 退出${NC}"
    screen -r mc-server 2>/dev/null || echo "服务器未在 screen 中运行"
}

cmd_cmd() {
    if [[ -z "$*" ]]; then
        echo "用法: mc-manager cmd <服务器命令>"
        return 1
    fi
    # 通过 systemd 的 ExecStop 机制发送命令
    screen -S mc-server -X stuff "$*\n" 2>/dev/null || \
        echo -e "${YELLOW}无法发送命令，请使用 mc-manager console 进入控制台${NC}"
}

cmd_players() { cmd_cmd "list"; }

cmd_say() {
    if [[ -z "$*" ]]; then
        echo "用法: mc-manager say <消息>"
        return 1
    fi
    cmd_cmd "say $*"
}

cmd_whitelist() {
    echo "白名单管理:"
    echo "  mc-manager cmd whitelist add <玩家名>"
    echo "  mc-manager cmd whitelist remove <玩家名>"
    echo "  mc-manager cmd whitelist list"
    echo "  mc-manager cmd whitelist on"
    echo "  mc-manager cmd whitelist off"
}

cmd_backup() {
    local backup_dir="${MC_DIR}/backups"
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_file="${backup_dir}/world_backup_${timestamp}.tar.gz"

    mkdir -p "$backup_dir"
    echo "正在备份..."

    # 先保存世界
    screen -S mc-server -X stuff "save-off\n" 2>/dev/null
    screen -S mc-server -X stuff "save-all\n" 2>/dev/null
    sleep 3

    tar -czf "$backup_file" -C "${MC_DIR}" world 2>/dev/null

    screen -S mc-server -X stuff "save-on\n" 2>/dev/null

    if [[ -f "$backup_file" ]]; then
        echo -e "${GREEN}备份成功: ${backup_file}${NC}"
        # 保留最近20个备份
        ls -t "${backup_dir}"/world_backup_*.tar.gz 2>/dev/null | tail -n +21 | xargs -r rm -f
    else
        echo -e "${RED}备份失败${NC}"
    fi
}

cmd_update() {
    echo -e "${CYAN}更新服务器...${NC}"
    systemctl stop "$SERVICE"

    local jar_file="${MC_DIR}/paper.jar"
    local current_hash=$(md5sum "$jar_file" 2>/dev/null | awk '{print $1}')

    # 重新下载
    local paper_version=$(curl -sL "https://api.papermc.io/v2/projects/paper" 2>/dev/null | \
        python3 -c "import sys,json; d=json.load(sys.stdin); print(d['versions'][-1])" 2>/dev/null)
    local paper_build=$(curl -sL "https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds" 2>/dev/null | \
        python3 -c "import sys,json; d=json.load(sys.stdin); builds=[b for b in d['builds'] if b['channel']=='default']; print(builds[-1]['build'])" 2>/dev/null)
    local paper_url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/${paper_build}/downloads/paper-${paper_version}-${paper_build}.jar"

    cp "$jar_file" "${jar_file}.bak"
    curl -sL -o "$jar_file" "$paper_url"

    local new_hash=$(md5sum "$jar_file" 2>/dev/null | awk '{print $1}')

    systemctl start "$SERVICE"

    if [[ "$current_hash" != "$new_hash" ]]; then
        echo -e "${GREEN}更新完成 (已从 ${paper_build} 更新)${NC}"
    else
        echo -e "${GREEN}已是最新版本${NC}"
    fi
}

cmd_config() {
    ${EDITOR:-nano} "${MC_DIR}/server.properties"
    echo -e "${YELLOW}配置已修改，重启生效: mc-manager restart${NC}"
}

cmd_memory() {
    echo -e "${CYAN}=== 内存使用 ===${NC}"
    systemctl show "$SERVICE" --property=MemoryCurrent --property=MemoryPeak 2>/dev/null || true
    ps aux | grep "[j]ava.*minecraft\|[j]ava.*paper" | awk '{printf "PID: %s  CPU: %s%%  MEM: %s%%  RSS: %sMB\n", $2, $3, $4, $6/1024}'
}

cmd_info() {
    local ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo -e "${CYAN}=== 服务器信息 ===${NC}"
    echo "服务器地址:  ${ip}:${MC_PORT}"
    echo "RCON端口:    ${RCON_PORT}"
    echo "RCON密码:    ${RCON_PASSWORD}"
    echo "服务器类型:  ${SERVER_TYPE}"
    echo "状态:        $(systemctl is-active "$SERVICE")"
    echo "配置文件:    ${MC_DIR}/server.properties"
    echo "世界目录:    ${MC_DIR}/world"
}

case "${1:-help}" in
    start)    cmd_start ;;
    stop)     cmd_stop ;;
    restart)  cmd_restart ;;
    status)   cmd_status ;;
    logs)     cmd_logs ;;
    console)  cmd_console ;;
    cmd)      shift; cmd_cmd "$@" ;;
    players)  cmd_players ;;
    say)      shift; cmd_say "$@" ;;
    whitelist) cmd_whitelist ;;
    backup)   cmd_backup ;;
    update)   cmd_update ;;
    config)   cmd_config ;;
    memory)   cmd_memory ;;
    info)     cmd_info ;;
    *)        show_help ;;
esac
MANAGEREOF

    # 替换 RCON 密码
    sed -i "s/RCON_PASS_PLACEHOLDER/${MC_RCON_PASSWORD}/g" "${MANAGER_SCRIPT}"
    chmod +x "${MANAGER_SCRIPT}"
    info "管理脚本: ${MANAGER_SCRIPT}"
}

# ==================== 创建自动备份 ====================
create_backup_timer() {
    info "创建自动备份定时器..."

    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" << EOF
[Unit]
Description=Minecraft Server Backup

[Service]
Type=oneshot
ExecStart=${MANAGER_SCRIPT} backup
User=${MC_USER}
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" << EOF
[Unit]
Description=Backup Minecraft world every 6 hours

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

# ==================== 配置防火墙 ====================
setup_firewall() {
    info "配置防火墙..."

    if command -v ufw &>/dev/null; then
        ufw allow "${MC_PORT}/tcp" comment "Minecraft Server"
        ufw allow "${MC_RCON_PORT}/tcp" comment "Minecraft RCON"
        info "已开放 TCP ${MC_PORT}(游戏) / TCP ${MC_RCON_PORT}(RCON)"
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${MC_PORT}/tcp"
        firewall-cmd --permanent --add-port="${MC_RCON_PORT}/tcp"
        firewall-cmd --reload
        info "已开放端口 (firewalld)"
    else
        warn "请手动开放端口: TCP ${MC_PORT}"
    fi
}

# ==================== 启动服务器 ====================
start_server() {
    info "启动 Minecraft 服务器..."
    systemctl start "${SERVICE_NAME}"
    sleep 10

    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        info "服务器启动成功!"
    else
        warn "服务器可能还在初始化，等待30秒..."
        sleep 20
        if systemctl is-active --quiet "${SERVICE_NAME}"; then
            info "服务器启动成功!"
        else
            warn "启动可能失败，请检查日志: journalctl -u ${SERVICE_NAME} -f"
        fi
    fi
}

# ==================== 显示结果 ====================
show_result() {
    local ip_addr
    ip_addr=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo -e "${GREEN}${BOLD}       Minecraft 服务器部署完成!${NC}"
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo ""
    echo -e "  服务器类型:  ${CYAN}${SERVER_TYPE}${NC}"
    echo -e "  服务器地址:  ${CYAN}${ip_addr}:${MC_PORT}${NC}"
    echo -e "  RCON 端口:   ${CYAN}${MC_RCON_PORT}${NC}"
    echo -e "  RCON 密码:   ${CYAN}${MC_RCON_PASSWORD}${NC}"
    echo ""
    echo -e "  配置文件:    ${MC_DIR}/server.properties"
    echo -e "  世界目录:    ${MC_DIR}/world"
    echo ""
    echo -e "  ${YELLOW}${BOLD}管理命令:${NC}"
    echo -e "    mc-manager start       # 启动"
    echo -e "    mc-manager stop        # 停止"
    echo -e "    mc-manager restart     # 重启"
    echo -e "    mc-manager status      # 状态"
    echo -e "    mc-manager logs        # 日志"
    echo -e "    mc-manager console     # 进入控制台"
    echo -e "    mc-manager cmd <命令>  # 执行命令"
    echo -e "    mc-manager players     # 在线玩家"
    echo -e "    mc-manager say <消息>  # 广播"
    echo -e "    mc-manager backup      # 备份"
    echo -e "    mc-manager update      # 更新"
    echo -e "    mc-manager config      # 编辑配置"
    echo -e "    mc-manager info        # 服务器信息"
    echo ""
    echo -e "  ${RED}${BOLD}!!! 重要: 云服务器安全组配置 !!!${NC}"
    echo -e "  ${YELLOW}系统防火墙已自动放行，但云服务器还需在控制台配置安全组:${NC}"
    echo ""
    echo -e "  ┌──────────────┬──────────┬────────────────────────────┐"
    echo -e "  │    端口      │   协议   │         用途               │"
    echo -e "  ├──────────────┼──────────┼────────────────────────────┤"
    echo -e "  │    25565     │   TCP    │  游戏主端口 (必须)         │"
    echo -e "  │    25575     │   TCP    │  RCON 远程管理 (可选)      │"
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
    echo -e "${GREEN}${BOLD}"
    echo "  __  __ _                  _         _           "
    echo " |  \/  (_)_ __   ___ _ __ | |_   ___| |__   ___  "
    echo " | |\/| | | '_ \ / _ \ '_ \| __| | __| '_ \ / _ \ "
    echo " | |  | | | | | |  __/ | | | |_ _| |_| | | |  __/ "
    echo " |_|  |_|_|_| |_|\___|_| |_|\__(_)____|_| |_|\___| "
    echo -e "      Dedicated Server Installer${NC}"
    echo ""

    check_root
    check_system
    check_resources
    user_config

    echo -e "\n${CYAN}${BOLD}即将执行部署步骤:${NC}"
    echo "  [1] 安装 Java 21"
    echo "  [2] 创建用户和目录"
    echo "  [3] 下载 ${SERVER_TYPE} 服务器"
    echo "  [4] 生成配置文件"
    echo "  [5] 写入优化配置"
    echo "  [6] 创建启动脚本"
    echo "  [7] 创建 systemd 服务"
    echo "  [8] 创建管理脚本"
    echo "  [9] 创建自动备份"
    echo "  [10] 配置防火墙"
    echo "  [11] 启动服务器"
    echo ""
    echo ""
    read -rp "回车开始部署 / 输入 n 取消: " confirm
    if [[ "$confirm" == "n" || "$confirm" == "N" ]]; then
        echo "已取消"
        exit 0
    fi

    install_java
    setup_user_and_dir
    download_server
    generate_configs
    configure_server
    create_start_script
    create_systemd_service
    create_manager_script
    create_backup_timer
    setup_firewall
    start_server
    show_result
}

main "$@"
