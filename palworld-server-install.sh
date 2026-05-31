#!/bin/bash
#============================================================
# 幻兽帕鲁 (Palworld) 专用服务器一键部署脚本 v2.0
# 适用于 Ubuntu 22.04+ / Debian 11+
# 参考来源:
#   - https://imotao.com/8011.html
#   - https://tech.palworldgame.com/settings-and-operation/configuration
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
STEAM_USER="steam"
PAL_SERVER_DIR="/home/${STEAM_USER}/Steam/steamapps/common/PalServer"
PAL_CONFIG_DIR="${PAL_SERVER_DIR}/Pal/Saved/Config/LinuxServer"
PAL_SETTINGS_FILE="${PAL_CONFIG_DIR}/PalWorldSettings.ini"
PAL_SAVE_DIR="${PAL_SERVER_DIR}/Pal/Saved"
SERVICE_NAME="pal-server"
MANAGER_SCRIPT="/usr/local/bin/pal-manager"
UPDATE_SCRIPT="/usr/local/bin/pal-update"

# 端口配置
DEFAULT_PORT=8211
QUERY_PORT=27015
RCON_PORT=25575

# 性能配置
SWAP_SIZE="16G"
MAX_PLAYERS=32
SERVER_NAME="Palworld Server"
SERVER_PASSWORD=""
ADMIN_PASSWORD="admin123"

# 内存限制 (systemd cgroup, 防止内存泄漏拖垮系统)
MEMORY_MAX="14G"
MEMORY_HIGH="12G"

# 日志
info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
step()  { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}\n"; }

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
        OS_VERSION=$VERSION_ID
    else
        error "无法检测操作系统，请使用 Ubuntu 或 Debian"
        exit 1
    fi

    case $OS in
        ubuntu|debian)
            info "检测到系统: $PRETTY_NAME"
            ;;
        centos|rhel|rocky|almalinux)
            info "检测到系统: $PRETTY_NAME (RHEL系)"
            ;;
        *)
            warn "当前系统为 $PRETTY_NAME，脚本针对 Ubuntu/Debian 优化，其他系统可能需要手动调整"
            ;;
    esac
}

check_resources() {
    local cpu_cores mem_total disk_free
    cpu_cores=$(nproc)
    mem_total=$(awk '/MemTotal/ {printf "%.0f", $2/1024/1024}' /proc/meminfo)
    disk_free=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')

    info "CPU 核心数: ${cpu_cores}"
    info "总内存: ${mem_total}GB"
    info "可用磁盘: ${disk_free}GB"

    if [[ $cpu_cores -lt 4 ]]; then
        warn "CPU 核心数少于 4 核 (当前 ${cpu_cores}核)，建议至少 4 核以获得流畅体验"
    fi

    if [[ $mem_total -lt 16 ]]; then
        warn "内存不足 16GB (当前 ${mem_total}GB)，建议 32GB 以上以稳定运行"
    fi

    if [[ $disk_free -lt 20 ]]; then
        warn "磁盘剩余空间不足 20GB (当前 ${disk_free}GB)，建议至少 40GB"
    fi
}

# ==================== 用户交互配置 ====================
user_config() {

    echo -e "  ${CYAN}当前默认配置:${NC}"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) 服务器名称:  ${SERVER_NAME}"
    echo -e "  │  2) 服务器密码:  (无密码)"
    echo -e "  │  3) 管理员密码:  ${ADMIN_PASSWORD}"
    echo -e "  │  4) 最大玩家数:  ${MAX_PLAYERS}"
    echo -e "  │  5) 游戏端口:    ${DEFAULT_PORT}"
    echo -e "  │  6) RCON端口:    ${RCON_PORT}"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    echo -e "  直接回车使用以上默认值，或输入 ${YELLOW}c${NC} 自定义配置"
    echo ""
    read -rp "  请选择 [直接回车=使用默认 / c=自定义]: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        echo ""
        read -rp "  服务器名称 [${SERVER_NAME}]: " input
        SERVER_NAME="${input:-$SERVER_NAME}"

        read -rp "  服务器密码 (留空为无密码): " input
        SERVER_PASSWORD="${input:-$SERVER_PASSWORD}"

        read -rp "  管理员密码 [${ADMIN_PASSWORD}]: " input
        ADMIN_PASSWORD="${input:-$ADMIN_PASSWORD}"

        read -rp "  最大玩家数 [${MAX_PLAYERS}]: " input
        MAX_PLAYERS="${input:-$MAX_PLAYERS}"

        read -rp "  游戏端口 [${DEFAULT_PORT}]: " input
        DEFAULT_PORT="${input:-$DEFAULT_PORT}"

        read -rp "  RCON端口 [${RCON_PORT}]: " input
        RCON_PORT="${input:-$RCON_PORT}"
    fi

    echo ""
    info "最终配置:"
    echo -e "    服务器名称:  ${CYAN}${SERVER_NAME}${NC}"
    echo -e "    最大玩家数:  ${CYAN}${MAX_PLAYERS}${NC}"
    echo -e "    游戏端口:    ${CYAN}${DEFAULT_PORT}${NC}"
    echo -e "    RCON端口:    ${CYAN}${RCON_PORT}${NC}"
    [[ -n "$SERVER_PASSWORD" ]] && echo -e "    服务器密码:  ${CYAN}已设置${NC}" || echo -e "    服务器密码:  ${CYAN}无密码${NC}"
    echo ""
}

# ==================== 内核参数优化 ====================
optimize_kernel() {

    local sysctl_changed=false

    # vm.max_map_count: 增加内存映射区域数量限制，帕鲁服务器需要大量内存映射
    local current_max_map_count
    current_max_map_count=$(sysctl -n vm.max_map_count 2>/dev/null || echo "65530")
    if [[ $current_max_map_count -lt 1048576 ]]; then
        sysctl -w vm.max_map_count=1048576 >/dev/null
        if ! grep -q "vm.max_map_count" /etc/sysctl.conf; then
            echo "vm.max_map_count=1048576" >> /etc/sysctl.conf
        else
            sed -i 's/^vm.max_map_count=.*/vm.max_map_count=1048576/' /etc/sysctl.conf
        fi
        sysctl_changed=true
        info "vm.max_map_count 设置为 1048576"
    else
        info "vm.max_map_count 已足够 (${current_max_map_count})"
    fi

    # vm.swappiness: 降低 Swap 使用倾向，优先使用物理内存
    local current_swappiness
    current_swappiness=$(sysctl -n vm.swappiness 2>/dev/null || echo "60")
    if [[ $current_swappiness -gt 10 ]]; then
        sysctl -w vm.swappiness=10 >/dev/null
        if ! grep -q "vm.swappiness" /etc/sysctl.conf; then
            echo "vm.swappiness=10" >> /etc/sysctl.conf
        else
            sed -i 's/^vm.swappiness=.*/vm.swappiness=10/' /etc/sysctl.conf
        fi
        sysctl_changed=true
        info "vm.swappiness 设置为 10"
    fi

    # fs.file-max: 增加系统最大文件描述符数量
    local current_file_max
    current_file_max=$(sysctl -n fs.file-max 2>/dev/null || echo "0")
    if [[ $current_file_max -lt 1048576 ]]; then
        sysctl -w fs.file-max=1048576 >/dev/null
        if ! grep -q "fs.file-max" /etc/sysctl.conf; then
            echo "fs.file-max=1048576" >> /etc/sysctl.conf
        else
            sed -i 's/^fs.file-max=.*/fs.file-max=1048576/' /etc/sysctl.conf
        fi
        sysctl_changed=true
        info "fs.file-max 设置为 1048576"
    fi

    # net.core 网络优化
    for param in "net.core.rmem_max=16777216" "net.core.wmem_max=16777216" "net.core.somaxconn=4096"; do
        local key="${param%%=*}"
        local val="${param#*=}"
        local current
        current=$(sysctl -n "$key" 2>/dev/null || echo "0")
        if [[ $current -lt $val ]]; then
            sysctl -w "$key=$val" >/dev/null
            if ! grep -q "^$key" /etc/sysctl.conf; then
                echo "$key=$val" >> /etc/sysctl.conf
            else
                sed -i "s|^${key}=.*|${key}=${val}|" /etc/sysctl.conf
            fi
            sysctl_changed=true
            info "$key 设置为 $val"
        fi
    done

    # 用户文件描述符限制
    if ! grep -q "soft nofile" /etc/security/limits.conf 2>/dev/null; then
        cat >> /etc/security/limits.conf << 'EOF'

# Palworld server optimization
* soft nofile 1048576
* hard nofile 1048576
root soft nofile 1048576
root hard nofile 1048576
EOF
        sysctl_changed=true
        info "文件描述符限制已设置"
    fi

    if $sysctl_changed; then
        sysctl -p >/dev/null 2>&1
        info "内核参数优化完成"
    else
        info "内核参数已是最佳状态"
    fi
}

# ==================== Swap 配置 ====================
setup_swap() {

    if swapon --show | grep -q "/swapfile"; then
        local current_swap
        current_swap=$(swapon --show --noheadings --bytes /swapfile | awk '{printf "%.0f", $3/1024/1024/1024}')
        info "Swap 已存在 (${current_swap}GB)，跳过创建"
        return
    fi

    info "创建 ${SWAP_SIZE} Swap 文件..."
    fallocate -l "${SWAP_SIZE}" /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile

    if ! grep -q "/swapfile" /etc/fstab; then
        echo "/swapfile none swap sw 0 0" >> /etc/fstab
    fi

    info "Swap 配置完成"
    swapon --show
}

# ==================== 安装 SteamCMD ====================
install_steamcmd() {

    if command -v steamcmd &>/dev/null; then
        info "SteamCMD 已安装，跳过"
        return
    fi

    info "更新软件源并安装依赖..."
    apt-get update -y

    # 安装通用依赖
    apt-get install -y curl wget screen jq

    case $OS in
        ubuntu)
            add-apt-repository multiverse -y
            dpkg --add-architecture i386
            apt-get update -y
            apt-get install -y steamcmd
            ;;
        debian)
            apt-get install -y software-properties-common
            apt-add-repository non-free -y
            dpkg --add-architecture i386
            apt-get update -y
            apt-get install -y steamcmd
            ;;
        centos|rhel|rocky|almalinux)
            # RHEL系需要手动下载 SteamCMD
            info "RHEL系系统，手动安装 SteamCMD..."
            local steamcmd_dir="/opt/steamcmd"
            mkdir -p "$steamcmd_dir"
            curl -sSL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | tar -xz -C "$steamcmd_dir"
            ln -sf "${steamcmd_dir}/steamcmd.sh" /usr/bin/steamcmd
            info "SteamCMD 安装到 ${steamcmd_dir}"
            ;;
        *)
            apt-get install -y lib32gcc-s1 steamcmd || {
                warn "包管理器安装失败，手动下载 SteamCMD..."
                local steamcmd_dir="/opt/steamcmd"
                mkdir -p "$steamcmd_dir"
                curl -sSL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | tar -xz -C "$steamcmd_dir"
                ln -sf "${steamcmd_dir}/steamcmd.sh" /usr/bin/steamcmd
            }
            ;;
    esac

    # 创建符号链接
    if [[ -f /usr/games/steamcmd ]] && [[ ! -f /usr/bin/steamcmd ]]; then
        ln -sf /usr/games/steamcmd /usr/bin/steamcmd
        info "已创建 steamcmd 符号链接"
    fi

    info "SteamCMD 安装完成"
}

# ==================== 创建 steam 用户 ====================
create_steam_user() {

    if id "$STEAM_USER" &>/dev/null; then
        info "用户 $STEAM_USER 已存在，跳过创建"
    else
        useradd -m -s /bin/bash "$STEAM_USER"
        info "用户 $STEAM_USER 创建完成"
    fi

    # 赋予 sudo 权限
    local sudoers_line="${STEAM_USER}   ALL=(ALL:ALL) ALL"
    if ! grep -qF "$sudoers_line" /etc/sudoers 2>/dev/null; then
        echo "$sudoers_line" >> /etc/sudoers
        info "已赋予 $STEAM_USER 用户 sudo 权限"
    fi

    # 设置用户资源限制
    local limits_file="/etc/security/limits.d/${STEAM_USER}.conf"
    cat > "$limits_file" << EOF
${STEAM_USER} soft nofile 1048576
${STEAM_USER} hard nofile 1048576
${STEAM_USER} soft nproc 65535
${STEAM_USER} hard nproc 65535
EOF
    info "已设置 $STEAM_USER 用户资源限制"
}

# ==================== 下载帕鲁服务器 ====================
download_palserver() {

    info "开始下载，这可能需要 10-30 分钟，取决于网络速度..."
    info "下载路径: ${PAL_SERVER_DIR}"

    # 确保安装目录存在
    mkdir -p "${PAL_SERVER_DIR}"
    chown "${STEAM_USER}:${STEAM_USER}" "${PAL_SERVER_DIR}"

    # 最多重试3次
    local retry=0
    local max_retry=3
    while ! sudo -u "$STEAM_USER" steamcmd +login anonymous +force_install_dir "${PAL_SERVER_DIR}" +app_update 2394010 validate +quit; do
        retry=$((retry + 1))
        if [[ $retry -ge $max_retry ]]; then
            error "SteamCMD 下载失败，已重试 ${max_retry} 次"
            error "请检查网络连接，或手动执行:"
            error "  sudo -u ${STEAM_USER} steamcmd +login anonymous +force_install_dir ${PAL_SERVER_DIR} +app_update 2394010 validate +quit"
            exit 1
        fi
        warn "下载失败，${retry}/${max_retry} 次重试..."
        sleep 5
    done

    if [[ -f "${PAL_SERVER_DIR}/PalServer.sh" ]]; then
        info "幻兽帕鲁服务器下载完成"
    else
        error "服务器文件缺失，请检查下载是否完整"
        exit 1
    fi
}

# ==================== 配置服务器 ====================
configure_server() {

    mkdir -p "${PAL_CONFIG_DIR}"
    mkdir -p "${PAL_SAVE_DIR}/SaveGames"
    mkdir -p "${PAL_SAVE_DIR}/Backup"

    # 复制默认配置文件
    local default_config="${PAL_SERVER_DIR}/DefaultPalWorldSettings.ini"
    if [[ -f "$default_config" ]] && [[ ! -f "${PAL_SETTINGS_FILE}" ]]; then
        cp "$default_config" "${PAL_SETTINGS_FILE}"
        info "已从默认配置创建 PalWorldSettings.ini"
    fi

    # 如果配置文件不存在，先启动一次服务器生成默认配置
    if [[ ! -f "${PAL_SETTINGS_FILE}" ]]; then
        info "首次启动以生成默认配置文件..."
        cd "${PAL_SERVER_DIR}"
        sudo -u "$STEAM_USER" timeout 90 ./PalServer.sh -useperfthreads -NoAsyncLoadingThread -UseMultithreadForDS || true

        local wait_count=0
        while [[ ! -f "${PAL_SETTINGS_FILE}" ]] && [[ $wait_count -lt 45 ]]; do
            sleep 2
            wait_count=$((wait_count + 1))
        done

        pkill -f PalServer-Linux 2>/dev/null || true
        pkill -f PalServer 2>/dev/null || true
        sleep 5
    fi

    if [[ -f "${PAL_SETTINGS_FILE}" ]]; then
        # 备份原始配置
        cp "${PAL_SETTINGS_FILE}" "${PAL_SETTINGS_FILE}.bak.$(date +%Y%m%d%H%M%S)"
        info "配置文件已备份"

        # 写入优化后的配置 (基于官方文档)
        # 参考: https://tech.palworldgame.com/settings-and-operation/configuration
        cat > "${PAL_SETTINGS_FILE}" << EOF
[/Script/Pal.PalGameWorldSettings]
OptionSettings=(
    ; ========== 服务器基础设置 ==========
    ServerName="${SERVER_NAME}",
    ServerDescription="Powered by Palworld Auto Installer",
    AdminPassword="${ADMIN_PASSWORD}",
    ServerPassword="${SERVER_PASSWORD}",
    ServerPlayerMaxNum=${MAX_PLAYERS},
    PublicPort=${DEFAULT_PORT},
    PublicIP="",
    RCONEnabled=True,
    RCONPort=${RCON_PORT},

    ; ========== 性能优化 (官方参数) ==========
    ; Pal同步距离(cm)，降低可减少网络负载，最小5000最大15000
    ServerReplicatePawnCullDistance=10000,

    ; 每个公会最大基地数(默认4,最大10)，降低可减少服务器负载
    BaseCampMaxNumInGuild=4,

    ; 每个基地最大帕鲁数(最大50)，降低可减少计算量
    BaseCampWorkerMaxNum=15,

    ; 帕鲁刷新率(影响性能)，降低可提升性能
    PalSpawnNumRate=1.000000,

    ; 每个玩家建筑上限(0=无限制)
    MaxBuildingLimitNum=0,

    ; ========== 存档与备份 ==========
    ; 启用自动备份(会增加磁盘负载)
    bIsUseBackupSaveData=True,

    ; ========== 游戏平衡设置 ==========
    ; 经验倍率
    ExpRate=1.000000,

    ; 帕鲁捕获率
    PalCaptureRate=1.000000,

    ; 帕鲁攻击力倍率
    PalDamageRateAttack=1.000000,

    ; 帕鲁防御力倍率
    PalDamageRateDefense=1.000000,

    ; 玩家攻击力倍率
    PlayerDamageRateAttack=1.000000,

    ; 玩家防御力倍率
    PlayerDamageRateDefense=1.000000,

    ; 死亡惩罚: None/Item/ItemAndEquipment/All
    DeathPenalty=Item,

    ; 白天时间流速
    DayTimeSpeedRate=1.000000,

    ; 夜晚时间流速
    NightTimeSpeedRate=1.000000,

    ; 聊天限制(每分钟最大消息数)
    ChatPostLimitPerMinute=10,

    ; 显示加入/离开消息
    bIsShowJoinLeftMessage=True,

    ; 显示玩家列表
    bShowPlayerList=True
)
EOF
        info "配置文件已写入优化参数"
        info "配置文件路径: ${PAL_SETTINGS_FILE}"
    else
        warn "配置文件未生成，首次启动后请手动检查"
    fi
}

# ==================== 创建启动脚本 ====================
create_start_script() {

    local start_script="${PAL_SERVER_DIR}/start-palserver.sh"
    cat > "$start_script" << 'STARTSCRIPT'
#!/bin/bash
# 幻兽帕鲁服务器优化启动脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 环境变量优化
export EOS_ENABLED=false              # 禁用 Epic Online Services，减少内存占用
export MALLOC_ARENA_MAX=2             # 限制 glibc 内存分配器 arena 数量，减少内存碎片
export LC_ALL=en_US.UTF-8

# 启动服务器
# -useperfthreads: 使用性能优化线程
# -NoAsyncLoadingThread: 禁用异步加载线程
# -UseMultithreadForDS: 使用多线程
exec ./PalServer.sh \
    -useperfthreads \
    -NoAsyncLoadingThread \
    -UseMultithreadForDS
STARTSCRIPT

    chmod +x "$start_script"
    chown "${STEAM_USER}:${STEAM_USER}" "$start_script"
    info "启动脚本已创建: ${start_script}"
}

# ==================== 创建 systemd 服务 ====================
create_systemd_service() {

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Palworld Dedicated Server
Documentation=https://tech.palworldgame.com/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${STEAM_USER}
Group=${STEAM_USER}
WorkingDirectory=${PAL_SERVER_DIR}
ExecStart=/bin/bash ${PAL_SERVER_DIR}/start-palserver.sh
ExecStop=/bin/kill -SIGINT \$MAINPID

; 重启策略: 失败后10秒重启，10分钟内最多重启5次
Restart=on-failure
RestartSec=10
StartLimitIntervalSec=600
StartLimitBurst=5

; ===== 内存限制 (防止内存泄漏拖垮系统) =====
MemoryMax=${MEMORY_MAX}
MemoryHigh=${MEMORY_HIGH}

; ===== 安全加固 =====
ProtectSystem=strict
ReadWritePaths=${PAL_SERVER_DIR}/Pal/Saved
ReadWritePaths=${PAL_SERVER_DIR}/Pal/Content/Paks
ReadWritePaths=${PAL_SERVER_DIR}/Engine
ReadWritePaths=${PAL_SERVER_DIR}
PrivateTmp=true
NoNewPrivileges=true

; ===== 日志 =====
StandardOutput=journal
StandardError=journal
SyslogIdentifier=palworld

; ===== OOM 保护 =====
OOMScoreAdjust=-500

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    info "systemd 服务创建完成 (MemoryMax=${MEMORY_MAX}, MemoryHigh=${MEMORY_HIGH})"
}

# ==================== 创建每日自动重启定时器 ====================
create_restart_timer() {

    # 每天凌晨4点自动重启，防止内存泄漏
    cat > "/etc/systemd/system/${SERVICE_NAME}-restart.service" << EOF
[Unit]
Description=Restart Palworld Server (prevent memory leak)
After=${SERVICE_NAME}.service

[Service]
Type=oneshot
ExecStart=/bin/systemctl restart ${SERVICE_NAME}
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-restart.timer" << EOF
[Unit]
Description=Daily restart for Palworld Server

[Timer]
OnCalendar=*-*-* 04:00:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}-restart.timer"
    systemctl start "${SERVICE_NAME}-restart.timer"
    info "每日凌晨4:00自动重启已启用 (防止内存泄漏)"
}

# ==================== 创建存档备份脚本 ====================
create_backup_script() {

    local backup_dir="/home/${STEAM_USER}/pal-backups"
    local backup_script="/usr/local/bin/pal-backup"

    mkdir -p "$backup_dir"

    cat > "$backup_script" << BACKUPSCRIPT
#!/bin/bash
# 幻兽帕鲁存档备份脚本

BACKUP_DIR="${backup_dir}"
SAVE_DIR="${PAL_SAVE_DIR}"
TIMESTAMP=\$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="\${BACKUP_DIR}/pal_backup_\${TIMESTAMP}.tar.gz"

mkdir -p "\$BACKUP_DIR"

# 备份前先通知服务器保存
echo "正在备份存档..."

# 压缩存档目录
tar -czf "\$BACKUP_FILE" -C "\$SAVE_DIR" SaveGames 2>/dev/null

if [[ \$? -eq 0 ]]; then
    echo "[\$(date)] 备份成功: \$BACKUP_FILE"
    # 保留最近30个备份
    ls -t "\$BACKUP_DIR"/pal_backup_*.tar.gz 2>/dev/null | tail -n +31 | xargs -r rm -f
    echo "[\$(date)] 清理旧备份完成"
else
    echo "[\$(date)] 备份失败!" >&2
    exit 1
fi
BACKUPSCRIPT

    chmod +x "$backup_script"

    # 每6小时自动备份一次
    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.service" << EOF
[Unit]
Description=Palworld Server Save Backup

[Service]
Type=oneshot
ExecStart=${backup_script}
User=${STEAM_USER}
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-backup.timer" << EOF
[Unit]
Description=Backup Palworld saves every 6 hours

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
    info "每6小时自动备份已启用 (存档路径: ${backup_dir})"
}

# ==================== 创建管理脚本 ====================
create_manager_script() {

    cat > "${MANAGER_SCRIPT}" << 'MANAGEREOF'
#!/bin/bash
# 幻兽帕鲁服务器管理脚本
# 用法: pal-manager [命令]

SERVICE="pal-server"
RCON_PORT=25575
ADMIN_PASS="ADMIN_PASSWORD_PLACEHOLDER"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

show_help() {
    echo -e "${CYAN}幻兽帕鲁服务器管理工具${NC}"
    echo ""
    echo "用法: pal-manager <命令>"
    echo ""
    echo "命令:"
    echo "  start       启动服务器"
    echo "  stop        停止服务器"
    echo "  restart     重启服务器"
    echo "  status      查看服务器状态"
    echo "  logs        查看实时日志 (Ctrl+C 退出)"
    echo "  logs-all    查看全部日志"
    echo "  update      更新服务器"
    echo "  backup      立即备份存档"
    echo "  config      编辑配置文件"
    echo "  rcon        发送 RCON 命令"
    echo "  players     查看在线玩家"
    echo "  broadcast   广播消息"
    echo "  memory      查看内存使用"
    echo "  info        显示服务器信息"
    echo ""
}

cmd_start()    { systemctl start "$SERVICE" && echo -e "${GREEN}服务器已启动${NC}"; }
cmd_stop()     { systemctl stop "$SERVICE" && echo -e "${YELLOW}服务器已停止${NC}"; }
cmd_restart()  { systemctl restart "$SERVICE" && echo -e "${GREEN}服务器已重启${NC}"; }
cmd_status()   { systemctl status "$SERVICE" --no-pager; }

cmd_logs()     { journalctl -u "$SERVICE" -f --no-pager; }
cmd_logs_all() { journalctl -u "$SERVICE" --no-pager -n 500; }

cmd_update() {
    echo -e "${CYAN}正在更新服务器...${NC}"
    systemctl stop "$SERVICE" 2>/dev/null
    sudo -u steam steamcmd +login anonymous +force_install_dir "/home/steam/Steam/steamapps/common/PalServer" +app_update 2394010 validate +quit
    systemctl start "$SERVICE"
    echo -e "${GREEN}更新完成${NC}"
}

cmd_backup() {
    /usr/local/bin/pal-backup
}

cmd_config() {
    ${EDITOR:-nano} "/home/steam/Steam/steamapps/common/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
    echo -e "${YELLOW}配置已修改，重启服务器生效: pal-manager restart${NC}"
}

cmd_rcon() {
    if [[ -z "$1" ]]; then
        echo "用法: pal-manager rcon <命令>"
        return 1
    fi
    echo "$1" | timeout 5 nc -u -w1 127.0.0.1 "$RCON_PORT" 2>/dev/null || \
        echo -e "${YELLOW}RCON 连接失败，请确认 RCON 已启用${NC}"
}

cmd_players() {
    cmd_rcon "ShowPlayers"
}

cmd_broadcast() {
    if [[ -z "$*" ]]; then
        echo "用法: pal-manager broadcast <消息>"
        return 1
    fi
    cmd_rcon "Broadcast $*"
}

cmd_memory() {
    echo -e "${CYAN}=== 内存使用情况 ===${NC}"
    systemctl show "$SERVICE" --property=MemoryCurrent --property=MemoryPeak 2>/dev/null || true
    echo ""
    ps aux | grep -i "[P]alServer" | awk '{printf "PID: %s  CPU: %s%%  MEM: %s%%  RSS: %sMB\n", $2, $3, $4, $6/1024}'
}

cmd_info() {
    local ip
    ip=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')
    echo -e "${CYAN}=== 服务器信息 ===${NC}"
    echo "服务器地址: ${ip}:8211"
    echo "RCON端口:   25575"
    echo "状态:       $(systemctl is-active "$SERVICE")"
    echo "运行时间:   $(systemctl show "$SERVICE" --property=ActiveEnterTimestamp --value 2>/dev/null)"
    echo "配置文件:   /home/steam/Steam/steamapps/common/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
}

case "${1:-help}" in
    start)      cmd_start ;;
    stop)       cmd_stop ;;
    restart)    cmd_restart ;;
    status)     cmd_status ;;
    logs)       cmd_logs ;;
    logs-all)   cmd_logs_all ;;
    update)     cmd_update ;;
    backup)     cmd_backup ;;
    config)     cmd_config ;;
    rcon)       shift; cmd_rcon "$@" ;;
    players)    cmd_players ;;
    broadcast)  shift; cmd_broadcast "$@" ;;
    memory)     cmd_memory ;;
    info)       cmd_info ;;
    *)          show_help ;;
esac
MANAGEREOF

    # 替换管理员密码
    sed -i "s/ADMIN_PASSWORD_PLACEHOLDER/${ADMIN_PASSWORD}/g" "${MANAGER_SCRIPT}"
    chmod +x "${MANAGER_SCRIPT}"
    info "管理脚本已创建: ${MANAGER_SCRIPT}"
}

# ==================== 配置防火墙 ====================
setup_firewall() {

    if command -v ufw &>/dev/null; then
        ufw allow "${DEFAULT_PORT}/udp"  comment "Palworld Game Port"
        ufw allow "${QUERY_PORT}/udp"   comment "Palworld Query Port"
        ufw allow "${RCON_PORT}/tcp"    comment "Palworld RCON"
        info "已开放 UDP ${DEFAULT_PORT}(游戏) / UDP ${QUERY_PORT}(查询) / TCP ${RCON_PORT}(RCON)"
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${DEFAULT_PORT}/udp"
        firewall-cmd --permanent --add-port="${QUERY_PORT}/udp"
        firewall-cmd --permanent --add-port="${RCON_PORT}/tcp"
        firewall-cmd --reload
        info "已开放端口 (firewalld)"
    else
        warn "未检测到防火墙工具 (ufw/firewalld)，请手动确认以下端口已开放:"
        warn "  UDP ${DEFAULT_PORT} (游戏)"
        warn "  UDP ${QUERY_PORT} (查询)"
        warn "  TCP ${RCON_PORT} (RCON)"
    fi
}

# ==================== 配置日志轮转 ====================
setup_logrotate() {

    cat > "/etc/logrotate.d/palworld" << EOF
/var/log/palworld/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 0640 ${STEAM_USER} ${STEAM_USER}
}
EOF
    info "日志轮转已配置 (保留7天)"
}

# ==================== 启动服务器 ====================
start_server() {

    systemctl start "${SERVICE_NAME}"
    sleep 8

    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        info "服务器启动成功!"
    else
        warn "服务器可能还在初始化中，请等待 30 秒后检查状态"
        warn "查看日志: journalctl -u ${SERVICE_NAME} -f"
    fi
}

# ==================== 显示结果 ====================
show_result() {
    local ip_addr
    ip_addr=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo -e "${GREEN}${BOLD}         幻兽帕鲁服务器部署完成!${NC}"
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo ""
    echo -e "  服务器地址:  ${CYAN}${ip_addr}:${DEFAULT_PORT}${NC}"
    echo -e "  RCON 端口:   ${CYAN}${RCON_PORT}${NC}"
    echo -e "  管理员密码:  ${CYAN}${ADMIN_PASSWORD}${NC}"
    echo ""
    echo -e "  配置文件:    ${PAL_SETTINGS_FILE}"
    echo -e "  服务器目录:  ${PAL_SERVER_DIR}"
    echo -e "  存档目录:    ${PAL_SAVE_DIR}"
    echo -e "  服务名称:    ${SERVICE_NAME}"
    echo ""
    echo -e "  ${YELLOW}${BOLD}管理命令 (推荐使用 pal-manager):${NC}"
    echo -e "    pal-manager start       # 启动服务器"
    echo -e "    pal-manager stop        # 停止服务器"
    echo -e "    pal-manager restart     # 重启服务器"
    echo -e "    pal-manager status      # 查看状态"
    echo -e "    pal-manager logs        # 查看实时日志"
    echo -e "    pal-manager update      # 更新服务器"
    echo -e "    pal-manager backup      # 立即备份存档"
    echo -e "    pal-manager config      # 编辑配置"
    echo -e "    pal-manager players     # 查看在线玩家"
    echo -e "    pal-manager broadcast X # 广播消息"
    echo -e "    pal-manager memory      # 查看内存使用"
    echo -e "    pal-manager info        # 显示服务器信息"
    echo ""
    echo -e "  ${YELLOW}${BOLD}自动任务:${NC}"
    echo -e "    每日凌晨4:00  自动重启 (防止内存泄漏)"
    echo -e "    每6小时       自动备份存档"
    echo ""
    echo -e "  ${YELLOW}${BOLD}systemctl 命令:${NC}"
    echo -e "    sudo systemctl start ${SERVICE_NAME}"
    echo -e "    sudo systemctl stop ${SERVICE_NAME}"
    echo -e "    sudo systemctl restart ${SERVICE_NAME}"
    echo -e "    sudo systemctl status ${SERVICE_NAME}"
    echo -e "    sudo journalctl -u ${SERVICE_NAME} -f"
    echo ""
    echo -e "  ${RED}${BOLD}!!! 重要: 云服务器安全组配置 !!!${NC}"
    echo -e "  ${YELLOW}系统防火墙已自动放行，但云服务器还需在控制台配置安全组:${NC}"
    echo ""
    echo -e "  ┌──────────────┬──────────┬────────────────────────────┐"
    echo -e "  │    端口      │   协议   │         用途               │"
    echo -e "  ├──────────────┼──────────┼────────────────────────────┤"
    echo -e "  │    8211      │   UDP    │  游戏主端口 (必须)         │"
    echo -e "  │    27015     │   UDP    │  查询端口 (必须)           │"
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
    echo -e "${CYAN}${BOLD}"
    echo "  ____       _     _   ____  ____  _   _ ____  "
    echo " |  _ \ __ _| |___| |_/ ___||  _ \| | | / ___| "
    echo " | |_) / _\` | / __| __\___ \| |_) | | | \___ \ "
    echo " |  __/ (_| | \__ \ |_ ___) |  __/| |_| |___) |"
    echo " |_|   \__,_|_|___/\__|____/|_|    \___/|____/ "
    echo -e "      Dedicated Server Installer v2.0${NC}"
    echo -e "      https://tech.palworldgame.com/"
    echo ""

    check_root
    check_system
    check_resources
    user_config

    # 显示部署步骤概览
    echo -e "${CYAN}${BOLD}即将执行以下部署步骤:${NC}"
    echo -e "  [ 1] 优化内核参数 (vm.max_map_count, 文件描述符, 网络缓冲区)"
    echo -e "  [ 2] 配置 Swap 空间 (${SWAP_SIZE})"
    echo -e "  [ 3] 安装 SteamCMD"
    echo -e "  [ 4] 创建 steam 用户"
    echo -e "  [ 5] 下载帕鲁服务器 (AppID: 2394010)"
    echo -e "  [ 6] 创建优化启动脚本"
    echo -e "  [ 7] 配置服务器参数 (PalWorldSettings.ini)"
    echo -e "  [ 8] 创建 systemd 服务 + 内存限制"
    echo -e "  [ 9] 创建每日自动重启定时器"
    echo -e "  [10] 创建存档自动备份"
    echo -e "  [11] 创建管理脚本 (pal-manager)"
    echo -e "  [12] 配置防火墙端口"
    echo -e "  [13] 配置日志轮转"
    echo -e "  [14] 启动服务器"
    echo ""
    echo ""
    read -rp "回车开始部署 / 输入 n 取消: " confirm
    if [[ "$confirm" == "n" || "$confirm" == "N" ]]; then
        echo "已取消部署"
        exit 0
    fi

    local total_steps=14
    local current_step=0

    run_step() {
        current_step=$((current_step + 1))
        echo -e "\n${CYAN}${BOLD}[${current_step}/${total_steps}]${NC} $1"
        echo -e "${CYAN}────────────────────────────────────────${NC}"
    }

    run_step "优化内核参数"
    optimize_kernel

    run_step "配置 Swap 空间"
    setup_swap

    run_step "安装 SteamCMD"
    install_steamcmd

    run_step "创建 steam 用户"
    create_steam_user

    run_step "下载帕鲁服务器"
    download_palserver

    run_step "创建优化启动脚本"
    create_start_script

    run_step "配置服务器参数"
    configure_server

    run_step "创建 systemd 服务"
    create_systemd_service

    run_step "创建每日自动重启定时器"
    create_restart_timer

    run_step "创建存档备份脚本"
    create_backup_script

    run_step "创建管理脚本"
    create_manager_script

    run_step "配置防火墙"
    setup_firewall

    run_step "配置日志轮转"
    setup_logrotate

    run_step "启动服务器"
    start_server

    show_result
}

main "$@"
