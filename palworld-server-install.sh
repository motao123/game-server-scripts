#!/bin/bash
#============================================================
# 幻兽帕鲁 (Palworld) 专用服务器一键部署脚本 v2.0
# 适用于 Ubuntu 22.04+ / Debian 11+
# 参考来源:
#   - https://imotao.com/8011.html
#   - https://tech.palworldgame.com/settings-and-operation/configuration
#============================================================

set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

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
CREDENTIALS_FILE="/etc/palworld/credentials.env"

# SteamCMD / 离线包下载配置
STEAMCMD_URL="${STEAMCMD_URL:-https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz}"
STEAMCMD_PROXY="${STEAMCMD_PROXY:-}"
PALSERVER_ARCHIVE_URL="${PALSERVER_ARCHIVE_URL:-}"
PALSERVER_ARCHIVE_SHA256="${PALSERVER_ARCHIVE_SHA256:-}"

# 端口配置
DEFAULT_PORT="${DEFAULT_PORT:-8211}"
QUERY_PORT="${QUERY_PORT:-27015}"
RCON_PORT="${RCON_PORT:-25575}"
REST_API_PORT="${REST_API_PORT:-8212}"  # v1.0 REST API，仅本地访问（官方建议不暴露公网）

# 性能配置
SWAP_SIZE="${SWAP_SIZE:-16G}"
MAX_PLAYERS="${MAX_PLAYERS:-32}"
SERVER_NAME="${SERVER_NAME:-Palworld Server}"
SERVER_PASSWORD="${SERVER_PASSWORD:-}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"

# 内存限制 (systemd cgroup, 防止内存泄漏拖垮系统)
# 由 compute_memory_limits() 根据系统内存动态计算
MEMORY_MAX=""
MEMORY_HIGH=""

# 日志
info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
step()  { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}\n"; }

generate_password() {
    local password
    password=$(set +o pipefail; head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 20)
    printf '%s' "${password:-pal$(date +%s)}"
}

write_credentials_file() {
    mkdir -p "$(dirname "$CREDENTIALS_FILE")"
    umask 077
    {
        printf 'RCON_PORT=%q\n' "$RCON_PORT"
        printf 'REST_API_PORT=%q\n' "$REST_API_PORT"
        printf 'ADMIN_PASSWORD=%q\n' "$ADMIN_PASSWORD"
    } > "$CREDENTIALS_FILE"
    chown root:"${STEAM_USER}" "$CREDENTIALS_FILE"
    chmod 640 "$CREDENTIALS_FILE"
    info "管理员凭证已保存到 ${CREDENTIALS_FILE} (root:${STEAM_USER} 640)"
}

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

# ==================== 动态计算内存限制 ====================
compute_memory_limits() {
    local mem_total
    mem_total=$(awk '/MemTotal/ {printf "%.0f", $2/1024/1024}' /proc/meminfo)

    # 给系统至少留 2G；低内存机器不设置超过物理内存的限制
    local mem_max=$((mem_total - 2))
    [[ $mem_max -lt 2 ]] && mem_max=2
    local mem_high=$((mem_max - 2))
    [[ $mem_high -lt 1 ]] && mem_high=1

    MEMORY_MAX="${mem_max}G"
    MEMORY_HIGH="${mem_high}G"
    info "内存限制: MemoryMax=${MEMORY_MAX}, MemoryHigh=${MEMORY_HIGH} (系统总内存 ${mem_total}G)"
}

# ==================== 用户交互配置 ====================
user_config() {

    if [[ -z "$ADMIN_PASSWORD" ]]; then
        ADMIN_PASSWORD=$(generate_password)
        info "已自动生成强管理员密码"
    fi

    if [[ "$NONINTERACTIVE" == "1" ]]; then
        info "非交互模式：使用环境变量与默认配置"
    else
        echo -e "  ${CYAN}当前默认配置:${NC}"
        echo -e "  ┌─────────────────────────────────────────────┐"
        echo -e "  │  1) 服务器名称:  ${SERVER_NAME}"
        echo -e "  │  2) 服务器密码:  $([[ -n "$SERVER_PASSWORD" ]] && echo 已设置 || echo 无密码)"
        echo -e "  │  3) 管理员密码:  (已设置，安装结束显示)"
        echo -e "  │  4) 最大玩家数:  ${MAX_PLAYERS}"
        echo -e "  │  5) 游戏端口:    ${DEFAULT_PORT}"
        echo -e "  │  6) RCON端口:    ${RCON_PORT}"
        echo -e "  │  7) REST API端口: ${REST_API_PORT} (仅本地)"
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

            read -rsp "  管理员密码 (至少12位，回车保留自动密码): " input
            echo ""
            ADMIN_PASSWORD="${input:-$ADMIN_PASSWORD}"

            read -rp "  最大玩家数 [${MAX_PLAYERS}]: " input
            MAX_PLAYERS="${input:-$MAX_PLAYERS}"

            read -rp "  游戏端口 [${DEFAULT_PORT}]: " input
            DEFAULT_PORT="${input:-$DEFAULT_PORT}"

            read -rp "  RCON端口 [${RCON_PORT}]: " input
            RCON_PORT="${input:-$RCON_PORT}"

            read -rp "  REST API端口(仅本地) [${REST_API_PORT}]: " input
            REST_API_PORT="${input:-$REST_API_PORT}"
        fi
    fi

    if [[ ${#ADMIN_PASSWORD} -lt 12 ]]; then
        error "管理员密码至少需要 12 个字符"
        exit 1
    fi

    echo ""
    info "最终配置:"
    echo -e "    服务器名称:  ${CYAN}${SERVER_NAME}${NC}"
    echo -e "    最大玩家数:  ${CYAN}${MAX_PLAYERS}${NC}"
    echo -e "    游戏端口:    ${CYAN}${DEFAULT_PORT}${NC}"
    echo -e "    RCON端口:    ${CYAN}${RCON_PORT}${NC}"
    echo -e "    REST API端口: ${CYAN}${REST_API_PORT}${NC} (仅本地，不开防火墙)"
    [[ -n "$SERVER_PASSWORD" ]] && echo -e "    服务器密码:  ${CYAN}已设置${NC}" || echo -e "    服务器密码:  ${CYAN}无密码${NC}"
    [[ "$STEAMCMD_URL" != "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" ]] && echo -e "    SteamCMD镜像: ${CYAN}${STEAMCMD_URL}${NC}"
    [[ -n "$STEAMCMD_PROXY" ]] && echo -e "    SteamCMD代理: ${CYAN}${STEAMCMD_PROXY}${NC}"
    [[ -n "$PALSERVER_ARCHIVE_URL" ]] && echo -e "    离线包安装:  ${CYAN}已启用${NC}"
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
    for param in "net.core.rmem_max=67108864" "net.core.wmem_max=67108864" "net.core.somaxconn=4096"; do
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

    local requested_gb available_gb swap_gb
    requested_gb="${SWAP_SIZE%G}"
    available_gb=$(df -BG / | awk 'NR==2 {gsub(/G/, "", $4); print $4}')
    swap_gb=$requested_gb
    # 始终为根文件系统保留至少 8G。
    if (( swap_gb > available_gb - 8 )); then
        swap_gb=$((available_gb - 8))
    fi
    if (( swap_gb < 1 )); then
        warn "磁盘空间不足，跳过 Swap 创建（可用 ${available_gb}GB）"
        return
    fi

    info "创建 ${swap_gb}G Swap 文件（磁盘可用 ${available_gb}GB）..."
    fallocate -l "${swap_gb}G" /swapfile
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
install_steamcmd_from_archive() {
    local steamcmd_dir="/opt/steamcmd"
    local archive="/tmp/steamcmd_linux.tar.gz"

    info "使用安装包安装 SteamCMD: ${STEAMCMD_URL}"
    mkdir -p "$steamcmd_dir"
    if [[ -f "$STEAMCMD_URL" ]]; then
        cp "$STEAMCMD_URL" "$archive"
    else
        curl -fL --connect-timeout 20 --retry 3 --retry-delay 5 "$STEAMCMD_URL" -o "$archive"
    fi
    tar -xzf "$archive" -C "$steamcmd_dir"
    ln -sf "${steamcmd_dir}/steamcmd.sh" /usr/bin/steamcmd
    info "SteamCMD 安装到 ${steamcmd_dir}"
}

install_steamcmd() {

    if command -v steamcmd &>/dev/null; then
        info "SteamCMD 已安装，跳过"
        return
    fi

    info "更新软件源并安装依赖..."
    apt-get update -y

    # 安装通用依赖
    apt-get install -y curl wget screen jq python3

    case $OS in
        ubuntu)
            add-apt-repository multiverse -y
            dpkg --add-architecture i386
            apt-get update -y
            apt-get install -y steamcmd || install_steamcmd_from_archive
            ;;
        debian)
            apt-get install -y software-properties-common
            apt-add-repository non-free -y
            dpkg --add-architecture i386
            apt-get update -y
            apt-get install -y steamcmd || install_steamcmd_from_archive
            ;;
        centos|rhel|rocky|almalinux)
            info "RHEL系系统，手动安装 SteamCMD..."
            install_steamcmd_from_archive
            ;;
        *)
            apt-get install -y lib32gcc-s1 steamcmd || {
                warn "包管理器安装失败，手动下载 SteamCMD..."
                install_steamcmd_from_archive
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

    # 游戏进程用户不得拥有 sudo 权限。
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
run_steamcmd_palworld_update() {
    local steamcmd_args=(
        +login anonymous
        +force_install_dir "${PAL_SERVER_DIR}"
        +app_update 2394010 validate
        +quit
    )

    if [[ -n "$STEAMCMD_PROXY" ]]; then
        info "SteamCMD 将通过代理下载: ${STEAMCMD_PROXY}"
        sudo -u "$STEAM_USER" env \
            HTTP_PROXY="$STEAMCMD_PROXY" \
            HTTPS_PROXY="$STEAMCMD_PROXY" \
            ALL_PROXY="$STEAMCMD_PROXY" \
            http_proxy="$STEAMCMD_PROXY" \
            https_proxy="$STEAMCMD_PROXY" \
            all_proxy="$STEAMCMD_PROXY" \
            steamcmd "${steamcmd_args[@]}"
    else
        sudo -u "$STEAM_USER" steamcmd "${steamcmd_args[@]}"
    fi
}

download_palserver_from_archive() {
    local archive="/tmp/PalServer.tar.gz"

    info "使用用户自备离线包安装 Palworld 服务端"
    info "离线包地址: ${PALSERVER_ARCHIVE_URL}"

    mkdir -p "$(dirname "${PAL_SERVER_DIR}")"
    if [[ -f "$PALSERVER_ARCHIVE_URL" ]]; then
        cp "$PALSERVER_ARCHIVE_URL" "$archive"
    else
        curl -fL --connect-timeout 20 --retry 3 --retry-delay 5 "$PALSERVER_ARCHIVE_URL" -o "$archive"
    fi

    if [[ -n "$PALSERVER_ARCHIVE_SHA256" ]]; then
        echo "${PALSERVER_ARCHIVE_SHA256}  ${archive}" | sha256sum -c -
    fi

    tar -xzf "$archive" -C "$(dirname "${PAL_SERVER_DIR}")"
    chown -R "${STEAM_USER}:${STEAM_USER}" "${PAL_SERVER_DIR}"

    if [[ ! -f "${PAL_SERVER_DIR}/PalServer.sh" ]]; then
        error "离线包解压后未找到 PalServer.sh"
        error "请确认压缩包内目录结构为 PalServer/PalServer.sh"
        exit 1
    fi

    info "Palworld 服务端离线包安装完成"
}

download_palserver() {

    info "开始下载，这可能需要 10-30 分钟，取决于网络速度..."
    info "下载路径: ${PAL_SERVER_DIR}"

    # 确保安装目录存在
    mkdir -p "${PAL_SERVER_DIR}"
    chown "${STEAM_USER}:${STEAM_USER}" "${PAL_SERVER_DIR}"

    if [[ -n "$PALSERVER_ARCHIVE_URL" ]]; then
        download_palserver_from_archive
        return
    fi

    # 最多重试3次
    local retry=0
    local max_retry=3
    while ! run_steamcmd_palworld_update; do
        retry=$((retry + 1))
        if [[ $retry -ge $max_retry ]]; then
            error "SteamCMD 下载失败，已重试 ${max_retry} 次"
            error "国内服务器连接 Steam CDN 失败较常见，可尝试:"
            error "  1) 使用代理: sudo env STEAMCMD_PROXY=socks5://127.0.0.1:7890 $0"
            error "  2) 使用 HTTP 代理: sudo env STEAMCMD_PROXY=http://127.0.0.1:7890 $0"
            error "  3) 使用自备离线包: sudo env PALSERVER_ARCHIVE_URL=https://your-private-url/PalServer.tar.gz $0"
            error "也可手动执行:"
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

    # 立即 chown：确保下方「首次启动生成配置」(若触发) 时 steam 用户能写入
    # 不然 steam 进程在 root 属主目录下跑，ServerUID/配置文件写不进
    chown -R "${STEAM_USER}:${STEAM_USER}" "${PAL_SAVE_DIR}"

    # 用官方默认配置作为基础（单行格式，含全部配置项）
    # 写入后校验关键配置，避免上游格式变化导致静默失败。
    local default_config="${PAL_SERVER_DIR}/DefaultPalWorldSettings.ini"
    if [[ -f "$default_config" ]]; then
        cp "$default_config" "${PAL_SETTINGS_FILE}"
        # sed 修改关键项；用户输入先转义 sed replacement 元字符
        local escaped_server_name escaped_admin_password escaped_server_password
        escaped_server_name=$(printf '%s' "$SERVER_NAME" | sed 's/[&|\\]/\\&/g')
        escaped_admin_password=$(printf '%s' "$ADMIN_PASSWORD" | sed 's/[&|\\]/\\&/g')
        escaped_server_password=$(printf '%s' "$SERVER_PASSWORD" | sed 's/[&|\\]/\\&/g')
        sed -i \
            -e "s|ServerName=\"[^\"]*\"|ServerName=\"${escaped_server_name}\"|" \
            -e "s|ServerDescription=\"[^\"]*\"|ServerDescription=\"Powered by Palworld Auto Installer\"|" \
            -e "s|AdminPassword=\"[^\"]*\"|AdminPassword=\"${escaped_admin_password}\"|" \
            -e "s|ServerPassword=\"[^\"]*\"|ServerPassword=\"${escaped_server_password}\"|" \
            -e "s/ServerPlayerMaxNum=[0-9]*/ServerPlayerMaxNum=${MAX_PLAYERS}/" \
            -e "s/PublicPort=[0-9]*/PublicPort=${DEFAULT_PORT}/" \
            -e "s/RCONEnabled=[A-Za-z]*/RCONEnabled=True/" \
            -e "s/RCONPort=[0-9]*/RCONPort=${RCON_PORT}/" \
            -e "s/RESTAPIEnabled=[A-Za-z]*/RESTAPIEnabled=True/" \
            -e "s/RESTAPIPort=[0-9]*/RESTAPIPort=${REST_API_PORT}/" \
            "${PAL_SETTINGS_FILE}"
        info "已从默认配置创建 PalWorldSettings.ini（单行格式 + sed 修改关键项）"
    fi

    local first_start_pid=""
    if [[ ! -f "${PAL_SETTINGS_FILE}" ]]; then
        info "首次启动以生成默认配置文件..."
        cd "${PAL_SERVER_DIR}"
        sudo -u "$STEAM_USER" ./PalServer.sh -useperfthreads -NoAsyncLoadingThread -UseMultithreadForDS &
        first_start_pid=$!

        local wait_count=0
        while [[ ! -f "${PAL_SETTINGS_FILE}" ]] && [[ $wait_count -lt 45 ]]; do
            sleep 2
            wait_count=$((wait_count + 1))
            kill -0 "$first_start_pid" 2>/dev/null || break
        done

        kill -SIGINT "$first_start_pid" 2>/dev/null || true
        sleep 5
        kill -KILL "$first_start_pid" 2>/dev/null || true
    fi

    if [[ -f "${PAL_SETTINGS_FILE}" ]]; then
        for expected in \
            "AdminPassword=\"${ADMIN_PASSWORD}\"" \
            "RCONEnabled=True" \
            "RCONPort=${RCON_PORT}" \
            "RESTAPIEnabled=True" \
            "RESTAPIPort=${REST_API_PORT}"; do
            if ! grep -qF "$expected" "${PAL_SETTINGS_FILE}"; then
                error "配置写入校验失败: ${expected%%=*}"
                exit 1
            fi
        done
        chown "${STEAM_USER}:${STEAM_USER}" "${PAL_SETTINGS_FILE}"
        chmod 640 "${PAL_SETTINGS_FILE}"
        info "PalWorldSettings.ini 已校验并设为 640"
        info "配置文件路径: ${PAL_SETTINGS_FILE}"
    else
        error "配置文件未生成，请检查服务器是否完整"
        exit 1
    fi

    chmod 640 "${PAL_SETTINGS_FILE}"
    info "存档与配置目录属主已修正为 ${STEAM_USER}"
}

# ==================== 创建启动脚本 ====================
create_start_script() {

    local start_script="${PAL_SERVER_DIR}/start-palserver.sh"
    cat > "$start_script" << 'STARTSCRIPT'
#!/bin/bash
# 幻兽帕鲁服务器优化启动脚本 (v1.0)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 环境变量优化
# MALLOC_ARENA_MAX: 限制 glibc 内存分配器 arena 数量，减少内存碎片
export MALLOC_ARENA_MAX=2
export LC_ALL=en_US.UTF-8

# v1.0 启动参数说明：
# 官方文档明确指出 -useperfthreads -NoAsyncLoadingThread -UseMultithreadForDS
# 在 v1.0 及以后「不设置可能提升性能」，故默认不传。
# 如需启用，设置环境变量 PAL_PERF_ARGS 后重启服务即可。
# 参考: https://docs.palworldgame.com/settings-and-operation/arguments
exec ./PalServer.sh ${PAL_PERF_ARGS:-}
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
; 重启频率限制: 10 分钟内最多重启 5 次（systemd 230+ 此项移到 [Unit]）
StartLimitIntervalSec=600
StartLimitBurst=5

[Service]
Type=simple
User=${STEAM_USER}
Group=${STEAM_USER}
WorkingDirectory=${PAL_SERVER_DIR}
ExecStart=/bin/bash ${PAL_SERVER_DIR}/start-palserver.sh
ExecStop=/usr/local/bin/pal-stop \$MAINPID

; 关闭: pal-stop 先 RCON Save 落盘，再发 SIGINT
KillSignal=SIGINT
TimeoutStopSec=300

; 重启策略: 失败后10秒重启（频率限制在 [Unit] 里）
Restart=on-failure
RestartSec=10

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

# ==================== 创建优雅重启脚本 ====================
create_graceful_restart_script() {

    # 含 RCON 密码，仅 root 可读写执行
    cat > /usr/local/bin/pal-graceful-restart << GRACEFULEOF
#!/bin/bash
# 幻兽帕鲁服务器优雅重启: 广播预警 -> 等待 -> RCON Save -> systemctl restart
set -e

CREDENTIALS_FILE="/etc/palworld/credentials.env"
source "\$CREDENTIALS_FILE"
RCON_PORT="\${RCON_PORT}"
RCON_PASS="\${ADMIN_PASSWORD}"

echo "[\$(date)] 开始优雅重启流程..."

# 1. 广播预警（60 秒倒计时）
/usr/local/bin/pal-rcon --port "\$RCON_PORT" --password "\$RCON_PASS" \\
    "Broadcast Server_restarting_in_60_seconds" || true
echo "[\$(date)] 已广播重启预警，等待 60 秒..."
sleep 60

# 2. RCON Save 落盘
/usr/local/bin/pal-rcon --port "\$RCON_PORT" --password "\$RCON_PASS" "Save" || true
echo "[\$(date)] 已发送 Save 命令，等待 5 秒落盘..."
sleep 5

# 3. systemctl restart (ExecStop 走 SIGINT 优雅退出，systemd 自动拉起)
systemctl restart ${SERVICE_NAME}
echo "[\$(date)] 重启完成"
GRACEFULEOF

    chmod 700 /usr/local/bin/pal-graceful-restart
    chown root:root /usr/local/bin/pal-graceful-restart
    info "优雅重启脚本已创建: /usr/local/bin/pal-graceful-restart"
}

# ==================== 创建每日自动重启定时器 ====================
create_restart_timer() {

    # 每天凌晨4点自动重启，防止内存泄漏
    cat > "/etc/systemd/system/${SERVICE_NAME}-restart.service" << EOF
[Unit]
Description=Graceful restart Palworld Server (save before restart)
After=${SERVICE_NAME}.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/pal-graceful-restart
EOF

    cat > "/etc/systemd/system/${SERVICE_NAME}-restart.timer" << EOF
[Unit]
Description=Daily graceful restart for Palworld Server

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
    info "每日凌晨4:00自动优雅重启已启用 (广播->Save->重启)"
}

# ==================== 创建存档备份脚本 ====================
create_backup_script() {

    local backup_dir="/home/${STEAM_USER}/pal-backups"
    local backup_script="/usr/local/bin/pal-backup"

    mkdir -p "$backup_dir"
    chown "${STEAM_USER}:${STEAM_USER}" "$backup_dir"

    cat > "$backup_script" << BACKUPSCRIPT
#!/bin/bash
# 幻兽帕鲁存档备份脚本

BACKUP_DIR="${backup_dir}"
SAVE_DIR="${PAL_SAVE_DIR}"
CREDENTIALS_FILE="/etc/palworld/credentials.env"
source "\$CREDENTIALS_FILE"
RCON_PORT="\${RCON_PORT}"
RCON_PASS="\${ADMIN_PASSWORD}"
TIMESTAMP=\$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="\${BACKUP_DIR}/pal_backup_\${TIMESTAMP}.tar.gz"

mkdir -p "\$BACKUP_DIR"

# 备份前先通知服务器保存，确保内存中的存档落盘
echo "正在保存存档..."
/usr/local/bin/pal-rcon --port "\$RCON_PORT" --password "\$RCON_PASS" "Save" 2>/dev/null || \\
    echo "[WARN] RCON 保存失败（服务器可能未启动），继续备份"
sleep 3

echo "正在压缩存档..."
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

    # 备份脚本含 RCON 密码，仅 steam 用户可读写执行
    chown "${STEAM_USER}:${STEAM_USER}" "$backup_script"
    chmod 700 "$backup_script"

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

# ==================== 安装 RCON 助手 ====================
install_rcon_helper() {

    cat > /usr/local/bin/pal-rcon << 'RCONEOF'
#!/usr/bin/env python3
"""Minimal RCON client for Palworld dedicated server.
Protocol: https://developer.valvesoftware.com/wiki/Source_RCON_Protocol
"""
import socket
import struct
import sys
import argparse


def _pack(pkt_id, pkt_type, body):
    payload = struct.pack('<ii', pkt_id, pkt_type) + body.encode('utf-8') + b'\x00\x00'
    return struct.pack('<i', len(payload)) + payload


def _recv(sock):
    size_data = b''
    while len(size_data) < 4:
        chunk = sock.recv(4 - len(size_data))
        if not chunk:
            return None
        size_data += chunk
    size = struct.unpack('<i', size_data)[0]
    data = b''
    while len(data) < size:
        chunk = sock.recv(size - len(data))
        if not chunk:
            break
        data += chunk
    pkt_id, pkt_type = struct.unpack('<ii', data[:8])
    body = data[8:-2].decode('utf-8', errors='replace')
    return pkt_id, pkt_type, body


def rcon(host, port, password, command, debug=False):
    try:
        sock = socket.create_connection((host, port), timeout=5)
    except (socket.timeout, ConnectionRefusedError) as e:
        print(f'RCON 连接失败: {e}', file=sys.stderr)
        return 1
    try:
        # 认证：发 AUTH 后循环读，直到 AUTH_RESPONSE(type 2) 或超时
        # 某些服务器认证后会先发一个空 RESPONSE 再发 AUTH_RESPONSE
        sock.sendall(_pack(1, 3, password))
        sock.settimeout(3)
        auth_ok = False
        auth_pkts = 0
        try:
            while True:
                pkt_id, pkt_type, _ = _recv(sock)
                auth_pkts += 1
                if pkt_id == -1:
                    print('RCON 认证失败，请检查管理员密码', file=sys.stderr)
                    return 1
                if pkt_type == 2:
                    auth_ok = True
                    break
                sock.settimeout(0.5)  # 后续包短超时，快速结束
        except socket.timeout:
            auth_ok = True  # 没收到 AUTH_RESPONSE 但也没收到 -1，假设认证成功
        if not auth_ok:
            print('RCON 认证超时', file=sys.stderr)
            return 1
        if debug:
            print(f'[DEBUG] 认证: 读取 {auth_pkts} 个包', file=sys.stderr)

        # 执行命令：循环读所有响应包，用超时结束
        # 关键：不在空 body 时 break——服务器可能先发空 ack 再发实际数据
        # ShowPlayers 等命令的响应可能被分片，或先发空 ack 再发实际数据
        sock.sendall(_pack(2, 2, command))
        bodies = []
        sock.settimeout(3)  # 第一个包用较长超时等响应
        try:
            while len(bodies) < 20:
                pkt_id, pkt_type, body = _recv(sock)
                bodies.append(body)
                if debug:
                    preview = body[:80].replace('\n', '\\n')
                    print(f'[DEBUG] 包 {len(bodies)}: type={pkt_type} len={len(body)} body={preview!r}', file=sys.stderr)
                sock.settimeout(0.5)  # 后续包短超时，无新数据快速结束
        except socket.timeout:
            pass
        if debug:
            non_empty = [b for b in bodies if b]
            print(f'[DEBUG] 命令响应: 共 {len(bodies)} 包, 非空 {len(non_empty)} 包', file=sys.stderr)
        output = '\n'.join(b for b in bodies if b)
        if output:
            print(output)
        return 0
    finally:
        sock.close()


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description='Palworld RCON client')
    ap.add_argument('--host', default='127.0.0.1')
    ap.add_argument('--port', type=int, default=25575)
    ap.add_argument('--password', required=True)
    ap.add_argument('--debug', action='store_true', help='输出调试信息到 stderr')
    ap.add_argument('command')
    args = ap.parse_args()
    sys.exit(rcon(args.host, args.port, args.password, args.command, args.debug))
RCONEOF

    chmod 755 /usr/local/bin/pal-rcon
    info "RCON 助手已安装: /usr/local/bin/pal-rcon"
}

# ==================== 安装优雅停止脚本 ====================
install_stop_script() {

    # systemd ExecStop 调用：RCON Save 落盘 -> sleep 3 -> SIGINT
    # 含 RCON 密码，与 pal-graceful-restart 同策略（heredoc 直接展开变量，避免 sed 特殊字符问题）
    cat > /usr/local/bin/pal-stop << STOPEOF
#!/bin/bash
# Palworld 优雅停止: RCON Save 落盘 -> 等待 -> SIGINT
# 由 systemd ExecStop 调用，\$1 = \$MAINPID
CREDENTIALS_FILE="/etc/palworld/credentials.env"
source "\$CREDENTIALS_FILE"
RCON_PORT="\${RCON_PORT}"
RCON_PASS="\${ADMIN_PASSWORD}"
PID="\$1"

# RCON Save（服务器可能未启动或 RCON 不可用，忽略错误）
/usr/local/bin/pal-rcon --port "\$RCON_PORT" --password "\$RCON_PASS" "Save" 2>/dev/null || true
sleep 3

# 发 SIGINT 让 Palworld 优雅退出（systemd 之后会按 TimeoutStopSec 兜底 SIGKILL）
kill -SIGINT "\$PID" 2>/dev/null || true
STOPEOF

    chmod 750 /usr/local/bin/pal-stop
    chown root:"${STEAM_USER}" /usr/local/bin/pal-stop
    info "优雅停止脚本已安装: /usr/local/bin/pal-stop (ExecStop 自动 Save)"
}

# ==================== 创建管理脚本 ====================
create_manager_script() {

    cat > "${MANAGER_SCRIPT}" << 'MANAGEREOF'
#!/bin/bash
# 幻兽帕鲁服务器管理脚本
# 用法: pal-manager [命令]

SERVICE="pal-server"
STEAM_USER="STEAM_USER_PLACEHOLDER"
PAL_SERVER_DIR="PAL_SERVER_DIR_PLACEHOLDER"
DEFAULT_PORT=DEFAULT_PORT_PLACEHOLDER
RCON_PORT=RCON_PORT_PLACEHOLDER
REST_API_PORT=REST_API_PORT_PLACEHOLDER
STEAMCMD_PROXY="${STEAMCMD_PROXY:-}"
CREDENTIALS_FILE="/etc/palworld/credentials.env"
source "$CREDENTIALS_FILE"
ADMIN_PASS="$ADMIN_PASSWORD"

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
    echo "  kick        踢出玩家 (需 SteamID)"
    echo "  ban         封禁玩家 (需 SteamID)"
    echo "  unban       解封玩家 (需 SteamID)"
    echo "  save        立即保存存档"
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

run_steamcmd_update() {
    local steamcmd_args=(
        +login anonymous
        +force_install_dir "$PAL_SERVER_DIR"
        +app_update 2394010 validate
        +quit
    )

    if [[ -n "$STEAMCMD_PROXY" ]]; then
        echo -e "${CYAN}SteamCMD 将通过代理下载: ${STEAMCMD_PROXY}${NC}"
        sudo -u "$STEAM_USER" env \
            HTTP_PROXY="$STEAMCMD_PROXY" \
            HTTPS_PROXY="$STEAMCMD_PROXY" \
            ALL_PROXY="$STEAMCMD_PROXY" \
            http_proxy="$STEAMCMD_PROXY" \
            https_proxy="$STEAMCMD_PROXY" \
            all_proxy="$STEAMCMD_PROXY" \
            steamcmd "${steamcmd_args[@]}"
    else
        sudo -u "$STEAM_USER" steamcmd "${steamcmd_args[@]}"
    fi
}

cmd_update() {
    echo -e "${CYAN}正在更新服务器...${NC}"
    systemctl stop "$SERVICE" 2>/dev/null

    local retry=0
    local max_retry=3
    while ! run_steamcmd_update; do
        retry=$((retry + 1))
        if [[ $retry -ge $max_retry ]]; then
            echo -e "${RED}SteamCMD 更新失败，已重试 ${max_retry} 次${NC}" >&2
            echo -e "${YELLOW}国内服务器可尝试: sudo env STEAMCMD_PROXY=socks5://127.0.0.1:7890 pal-manager update${NC}" >&2
            systemctl start "$SERVICE" 2>/dev/null
            return 1
        fi
        echo -e "${YELLOW}更新失败，${retry}/${max_retry} 次重试...${NC}"
        sleep 5
    done

    systemctl start "$SERVICE"
    echo -e "${GREEN}更新完成${NC}"
}

cmd_backup() {
    /usr/local/bin/pal-backup
}

cmd_config() {
    ${EDITOR:-nano} "$PAL_SERVER_DIR/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
    echo -e "${YELLOW}配置已修改，重启服务器生效: pal-manager restart${NC}"
}

cmd_rcon() {
    if [[ -z "$1" ]]; then
        echo "用法: pal-manager rcon <命令>"
        echo "v1.0 可用 RCON 命令:"
        echo "  Info                                    服务器信息"
        echo "  ShowPlayers                             在线玩家"
        echo "  Broadcast <msg>                         广播消息"
        echo "  Save                                    保存存档"
        echo "  Shutdown <秒> <msg>                     倒计时关闭"
        echo "  DoExit                                  强制关闭"
        echo "  KickPlayer <SteamID>                    踢人"
        echo "  BanPlayer <SteamID>                     封禁"
        echo "  UnBanPlayer <SteamID>                   解封"
        echo "  TeleportToPlayer <SteamID>              传送到玩家"
        echo "  TeleportToMe <SteamID>                  玩家传送到我"
        echo "  ToggleSpectate                          切换旁观模式"
        return 1
    fi
    /usr/local/bin/pal-rcon --port "$RCON_PORT" --password "$ADMIN_PASS" "$1"
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

cmd_kick() {
    if [[ -z "$1" ]]; then
        echo "用法: pal-manager kick <SteamID>"
        echo "先用 pal-manager players 查看在线玩家 SteamID"
        return 1
    fi
    cmd_rcon "KickPlayer $1"
}

cmd_ban() {
    if [[ -z "$1" ]]; then
        echo "用法: pal-manager ban <SteamID>"
        echo "先用 pal-manager players 查看在线玩家 SteamID"
        return 1
    fi
    cmd_rcon "BanPlayer $1"
}

cmd_unban() {
    if [[ -z "$1" ]]; then
        echo "用法: pal-manager unban <SteamID>"
        return 1
    fi
    cmd_rcon "UnBanPlayer $1"
}

cmd_save() {
    cmd_rcon "Save"
    echo -e "${GREEN}已发送 Save 命令${NC}"
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
    echo "服务器地址:   ${ip}:${DEFAULT_PORT}"
    echo "RCON端口:     ${RCON_PORT}"
    echo "REST API端口: ${REST_API_PORT} (仅本地)"
    echo "状态:         $(systemctl is-active "$SERVICE")"
    echo "运行时间:     $(systemctl show "$SERVICE" --property=ActiveEnterTimestamp --value 2>/dev/null)"
    echo "配置文件:     $PAL_SERVER_DIR/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
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
    kick)       shift; cmd_kick "$@" ;;
    ban)        shift; cmd_ban "$@" ;;
    unban)      shift; cmd_unban "$@" ;;
    save)       cmd_save ;;
    memory)     cmd_memory ;;
    info)       cmd_info ;;
    *)          show_help ;;
esac
MANAGEREOF

    # 替换占位符
    sed -i \
        -e "s/STEAM_USER_PLACEHOLDER/${STEAM_USER}/g" \
        -e "s|PAL_SERVER_DIR_PLACEHOLDER|${PAL_SERVER_DIR}|g" \
        -e "s/DEFAULT_PORT_PLACEHOLDER/${DEFAULT_PORT}/g" \
        -e "s/RCON_PORT_PLACEHOLDER/${RCON_PORT}/g" \
        -e "s/REST_API_PORT_PLACEHOLDER/${REST_API_PORT}/g" \
        "${MANAGER_SCRIPT}"
    chown root:root "${MANAGER_SCRIPT}"
    chmod 700 "${MANAGER_SCRIPT}"
    info "管理脚本已创建: ${MANAGER_SCRIPT} (root:root 700)"
}

# ==================== 配置防火墙 ====================
setup_firewall() {

    if command -v ufw &>/dev/null; then
        ufw allow "${DEFAULT_PORT}/udp"  comment "Palworld Game Port"
        ufw allow "${QUERY_PORT}/udp"   comment "Palworld Query Port"
        info "已添加游戏端口规则；RCON ${RCON_PORT}/tcp 默认不向公网开放"
        if ! ufw status 2>/dev/null | grep -qi "Status: active"; then
            warn "UFW 规则已添加，但 UFW 当前未启用"
        fi
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${DEFAULT_PORT}/udp"
        firewall-cmd --permanent --add-port="${QUERY_PORT}/udp"
        firewall-cmd --reload
        info "已开放游戏端口；RCON ${RCON_PORT}/tcp 默认不向公网开放"
    else
        warn "未检测到防火墙工具 (ufw/firewalld)，请手动确认以下端口已开放:"
        warn "  UDP ${DEFAULT_PORT} (游戏)"
        warn "  UDP ${QUERY_PORT} (查询)"
        warn "RCON TCP ${RCON_PORT} 请仅通过本机或 SSH 隧道使用"
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
    echo -e "  服务器地址:    ${CYAN}${ip_addr}:${DEFAULT_PORT}${NC}"
    echo -e "  RCON 端口:     ${CYAN}${RCON_PORT}${NC}"
    echo -e "  REST API 端口: ${CYAN}${REST_API_PORT}${NC} (仅本地，未开防火墙)"
    echo -e "  管理员密码:    ${CYAN}${ADMIN_PASSWORD}${NC}"
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
    echo -e "    pal-manager kick <id>   # 踢人"
    echo -e "    pal-manager ban <id>    # 封禁"
    echo -e "    pal-manager unban <id>  # 解封"
    echo -e "    pal-manager save        # 立即保存存档"
    echo -e "    pal-manager memory      # 查看内存使用"
    echo -e "    pal-manager info        # 显示服务器信息"
    echo ""
    echo -e "  ${YELLOW}${BOLD}自动任务:${NC}"
    echo -e "    每日凌晨4:00  自动优雅重启 (广播->Save->重启)"
    echo -e "    每6小时       自动备份存档 (含 RCON Save 落盘)"
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
    echo -e "  │    ${DEFAULT_PORT}      │   UDP    │  游戏主端口 (必须)         │"
    echo -e "  │    ${QUERY_PORT}     │   UDP    │  查询端口 (必须)           │"
    echo -e "  │    ${RCON_PORT}     │   TCP    │  RCON（仅本机/SSH隧道）      │"
    echo -e "  └──────────────┴──────────┴────────────────────────────┘"
    echo ""
    echo -e "  ${CYAN}RCON ${RCON_PORT} 与 REST API ${REST_API_PORT} 均不开放公网。${NC}"
    echo -e "  凭证文件: ${CREDENTIALS_FILE} (root:${STEAM_USER} 640)"
    echo ""
    echo -e "  ${CYAN}配置方式:${NC}"
    echo -e "    腾讯云:  控制台 → 云服务器 → 安全组 → 添加入站规则"
    echo -e "    阿里云:  控制台 → ECS → 安全组 → 配置规则 → 入方向"
    echo -e "    棉花云:  控制台 → 云服务器 → 防火墙 → 添加规则"
    echo ""
    echo -e "  ${YELLOW}${BOLD}手动开服注意事项 (非 systemd 管理):${NC}"
    echo -e "  若以非 root 用户手动运行 ./PalServer.sh，需将该用户加入 steam 组:"
    echo -e "    sudo usermod -aG steam <你的用户>"
    echo -e "  否则存档目录 (steam 属主) 无写权限，会导致存档丢失。"
    echo -e "  ${CYAN}推荐使用 pal-manager / systemctl 管理服务，避免此问题。${NC}"
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
    compute_memory_limits
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
    echo -e "  [ 8] 创建 systemd 服务 + 内存限制 (MemoryMax=${MEMORY_MAX})"
    echo -e "  [ 9] 安装 RCON 助手 (pal-rcon)"
    echo -e "  [10] 创建优雅停止脚本 (ExecStop 自动 Save)"
    echo -e "  [11] 创建优雅重启脚本 (广播->Save->重启)"
    echo -e "  [12] 创建每日自动重启定时器"
    echo -e "  [13] 创建存档自动备份"
    echo -e "  [14] 创建管理脚本 (pal-manager)"
    echo -e "  [15] 配置防火墙端口"
    echo -e "  [16] 配置日志轮转"
    echo -e "  [17] 启动服务器"
    echo ""
    echo ""
    if [[ "$NONINTERACTIVE" != "1" ]]; then
        read -rp "回车开始部署 / 输入 n 取消: " confirm
        if [[ "$confirm" == "n" || "$confirm" == "N" ]]; then
            echo "已取消部署"
            exit 0
        fi
    fi

    local total_steps=17
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
    write_credentials_file

    run_step "下载帕鲁服务器"
    download_palserver

    run_step "创建优化启动脚本"
    create_start_script

    run_step "配置服务器参数"
    configure_server

    run_step "创建 systemd 服务"
    create_systemd_service

    run_step "安装 RCON 助手"
    install_rcon_helper

    run_step "创建优雅停止脚本"
    install_stop_script

    run_step "创建优雅重启脚本"
    create_graceful_restart_script

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
