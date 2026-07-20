#!/bin/bash
#============================================================
# Minecraft Java Edition 专用服务器一键部署脚本
# 支持: Paper / Vanilla / Fabric / Forge
# 适用于: Ubuntu 22.04+ / Debian 11+
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
MC_USER="minecraft"
MC_DIR="/opt/minecraft"
MC_WORLD_DIR="${MC_DIR}/world"
SERVICE_NAME="mc-server"
MANAGER_SCRIPT="/usr/local/bin/mc-manager"

# 服务器类型: paper / vanilla / fabric / forge
SERVER_TYPE="paper"

# 内存配置 (根据玩家数量调整)
MC_MEMORY="4G"         # JVM 最大内存
MC_MEMORY_MIN="1G"     # JVM 最小内存

# 版本 (留空=最新版)
MC_VERSION=""

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
MC_RCON_PASSWORD="${MC_RCON_PASSWORD:-}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"
CREDENTIALS_FILE="/etc/minecraft/credentials.env"

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

    if [[ $mem_total -lt 2 ]]; then
        error "内存不足 2GB，无法运行 Minecraft 服务器"
        exit 1
    fi

    # 自动根据系统内存调整 JVM 内存 (系统内存 - 1GB，最低 512M)
    local jvm_mem=$((mem_total - 1))
    [[ $jvm_mem -lt 1 ]] && jvm_mem=1
    MC_MEMORY="${jvm_mem}G"
    MC_MEMORY_MIN="$((jvm_mem / 2))G"
    [[ "$MC_MEMORY_MIN" == "0G" ]] && MC_MEMORY_MIN="512M"

    if [[ $mem_total -lt 4 ]]; then
        warn "内存不足 4GB，已自动调整 JVM 内存为 ${MC_MEMORY}，建议升级到 4GB+"
    fi
}

# ==================== 用户配置 ====================
user_config() {
    if [[ "$NONINTERACTIVE" == "1" ]]; then
        info "非交互模式：服务器类型 ${SERVER_TYPE}，使用默认值/环境变量"
        if [[ -z "$MC_RCON_PASSWORD" ]]; then
            MC_RCON_PASSWORD=$(set +o pipefail; head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 20)
            MC_RCON_PASSWORD="${MC_RCON_PASSWORD:-mc$(date +%s)}"
        fi
        return
    fi

    echo -e "\n${CYAN}${BOLD}========== 服务器配置 ==========${NC}\n"

    echo -e "  ${CYAN}选择服务器类型:${NC}"
    echo -e "  ┌─────────────────────────────────────────────────────────┐"
    echo -e "  │  1) Paper (推荐) - 高性能分支，支持 Bukkit/Spigot 插件  │"
    echo -e "  │  2) Vanilla  - 官方原版，纯净体验                       │"
    echo -e "  │  3) Fabric   - 轻量 Mod 加载器，适合客户端 Mod 联机     │"
    echo -e "  │  4) Forge    - 经典 Mod 加载器，Mod 数量最多             │"
    echo -e "  └─────────────────────────────────────────────────────────┘"
    echo ""
    read -rp "  请选择 [1/2/3/4, 默认1]: " type_choice

    case "${type_choice:-1}" in
        2) SERVER_TYPE="vanilla" ;;
        3) SERVER_TYPE="fabric" ;;
        4) SERVER_TYPE="forge" ;;
        *) SERVER_TYPE="paper" ;;
    esac

    # 获取可用版本列表
    local available_versions=""
    local latest_version=""
    info "获取可用版本列表..."
    available_versions=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://api.papermc.io/v2/projects/paper" 2>/dev/null | \
        python3 -c "import sys,json; d=json.load(sys.stdin); print(' '.join(d['versions'][-10:]))" 2>/dev/null || true)
    latest_version=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://api.papermc.io/v2/projects/paper" 2>/dev/null | \
        python3 -c "import sys,json; d=json.load(sys.stdin); print(d['versions'][-1])" 2>/dev/null || true)

    echo -e "\n  ${CYAN}当前默认配置:${NC}"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) 服务器类型:  ${SERVER_TYPE}"
    echo -e "  │  2) 游戏版本:    ${latest_version:-最新版} (最新)"
    echo -e "  │  3) 游戏端口:    ${MC_PORT}"
    echo -e "  │  4) 最大玩家数:  ${MC_MAX_PLAYERS}"
    echo -e "  │  5) JVM 内存:    ${MC_MEMORY}"
    echo -e "  │  6) 游戏模式:    ${MC_GAMEMODE}"
    echo -e "  │  7) 难度:        ${MC_DIFFICULTY}"
    echo -e "  │  8) 视距:        ${MC_VIEW_DISTANCE}"
    echo -e "  │  9) MOTD:        (服务器列表显示名称)"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    echo -e "  直接回车使用默认值，或输入 ${YELLOW}c${NC} 自定义"
    echo ""
    read -rp "  请选择 [回车=默认 / c=自定义]: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        echo ""
        if [[ -n "$available_versions" ]]; then
            echo -e "  ${CYAN}可用版本:${NC} ${available_versions}"
            echo ""
        fi
        read -rp "  游戏版本 [${latest_version:-最新}，留空=最新]: " input
        MC_VERSION="${input:-}"

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
    if [[ -z "$MC_RCON_PASSWORD" ]]; then
        MC_RCON_PASSWORD=$(set +o pipefail; head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 20)
        MC_RCON_PASSWORD="${MC_RCON_PASSWORD:-mc$(date +%s)}"
    fi

    echo ""
    info "最终配置:"
    echo -e "    服务器类型:  ${CYAN}${SERVER_TYPE}${NC}"
    echo -e "    游戏版本:    ${CYAN}${MC_VERSION:-最新}${NC}"
    echo -e "    游戏端口:    ${CYAN}${MC_PORT}${NC}"
    echo -e "    最大玩家数:  ${CYAN}${MC_MAX_PLAYERS}${NC}"
    echo -e "    JVM 内存:    ${CYAN}${MC_MEMORY}${NC}"
    echo -e "    游戏模式:    ${CYAN}${MC_GAMEMODE}${NC}"
    echo -e "    难度:        ${CYAN}${MC_DIFFICULTY}${NC}"
    echo -e "    RCON 端口:   ${CYAN}${MC_RCON_PORT}${NC}"
    echo ""
}

# ==================== 安装依赖 ====================
install_deps() {
    info "安装依赖..."
    apt-get update -y
    apt-get install -y curl wget jq
}

# ==================== 安装 Java ====================
install_java() {
    info "检查 Java 环境..."

    # Minecraft 1.20.5+ 需要 Java 21
    local required_java=21

    if command -v java &>/dev/null; then
        local java_version
        java_version=$(set +o pipefail; java -version 2>&1 | head -1 | grep -oP '\d+' | head -1 || true)
        if [[ $java_version -ge $required_java ]]; then
            info "Java ${java_version} 已安装，满足要求"
            return
        else
            warn "Java ${java_version} 版本过低，需要 ${required_java}+"
        fi
    fi

    info "安装 Java ${required_java}..."

    # 方式1: 尝试从系统包管理器安装
    local installed=false
    case $OS in
        ubuntu|debian)
            apt-get update -y
            apt-get install -y openjdk-${required_java}-jre-headless 2>/dev/null && installed=true
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y java-${required_java}-openjdk 2>/dev/null && installed=true
            ;;
    esac

    # 方式2: 包管理器没有 Java 21，从网上下载安装
    if [[ "$installed" != "true" ]] || ! command -v java &>/dev/null; then
        warn "包管理器未找到 Java ${required_java}，准备下载..."

        local arch arch_azul
        arch=$(uname -m)
        case "$arch" in
            x86_64)  arch="x64"; arch_azul="linux_x64" ;;
            aarch64) arch="a64";  arch_azul="linux_aarch64" ;;
            *)       error "不支持的架构: $arch"; exit 1 ;;
        esac

        # 多源下载: Azul Zulu (全球CDN) → 清华镜像 → 官方 Adoptium
        local java_urls=(
            "https://cdn.azul.com/zulu/bin/zulu21.38.21-ca-jre21.0.5-${arch_azul}.tar.gz"
        )
        # 检测国内网络，加入清华镜像
        if curl -sL --connect-timeout 3 --max-time 5 "https://mirrors.tuna.tsinghua.edu.cn" &>/dev/null; then
            info "检测到国内网络，加入清华镜像源"
            java_urls+=("https://mirrors.tuna.tsinghua.edu.cn/Adoptium/${required_java}/jre/${arch}_linux/latest")
        fi
        java_urls+=("https://api.adoptium.net/v3/binary/latest/${required_java}/ga/linux/${arch}/jre/hotspot/normal/eclipse?project=jdk")

        local tmp_java="/tmp/java-jre.tar.gz"
        local download_ok=false

        for url in "${java_urls[@]}"; do
            info "尝试下载: ${url}"
            if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 300 -o "$tmp_java" "$url" && [[ $(stat -c%s "$tmp_java" 2>/dev/null || echo 0) -gt 1000000 ]]; then
                download_ok=true
                break
            fi
            warn "下载失败，尝试下一个源..."
            rm -f "$tmp_java"
        done

        if [[ "$download_ok" != "true" ]]; then
            error "所有下载源均失败，请手动安装 Java ${required_java}"
            exit 1
        fi

        local java_dir="/opt/java/java-${required_java}"
        mkdir -p "$java_dir"
        tar -xzf "$tmp_java" -C "$java_dir" --strip-components=1
        rm -f "$tmp_java"

        # 创建软链接
        ln -sf "${java_dir}/bin/java" /usr/local/bin/java
        export PATH="${java_dir}/bin:$PATH"

        info "Java ${required_java} 安装完成: ${java_dir}"
    fi

    if command -v java &>/dev/null; then
        info "Java 安装完成: $(set +o pipefail; java -version 2>&1 | head -1 || echo "unknown")"
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

    mkdir -p "$MC_DIR" "$MC_WORLD_DIR" "${MC_DIR}/logs" "${MC_DIR}/plugins" "${MC_DIR}/mods" "${MC_DIR}/backups"
    chown -R "${MC_USER}:${MC_USER}" "$MC_DIR"
}

# ==================== 下载服务器 ====================
download_server() {
    info "下载 ${SERVER_TYPE} 服务器..."

    local jar_file=""
    local use_bmcl=false

    # 检测国内网络，优先使用 BMCL 镜像
    if curl -sL --connect-timeout 3 --max-time 5 "https://bmclapidoc.bangbang93.com" &>/dev/null; then
        info "检测到国内网络，使用 BMCL 镜像加速"
        use_bmcl=true
    fi

    case $SERVER_TYPE in
        paper)
            local paper_version paper_url paper_build
            local download_ok=false

            # 默认版本 (已知可用的稳定版)
            local default_version="1.21.4"
            local default_build="232"

            if [[ "$use_bmcl" == "true" ]]; then
                # 国内: BMCL 镜像
                if [[ -n "$MC_VERSION" ]]; then
                    paper_version="$MC_VERSION"
                else
                    paper_version=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://bmclapidoc.bangbang93.com/mc/game/version_manifest.json" 2>/dev/null | \
                        python3 -c "import sys,json; print(json.load(sys.stdin)['latest']['release'])" 2>/dev/null || true)
                fi

                if [[ -n "$paper_version" ]]; then
                    # BMCL 有时返回不存在的版本号，先验证文件大小
                    paper_url="https://bmclapidoc.bangbang93.com/paper/${paper_version}/latest/download"
                    info "BMCL Paper 版本: ${paper_version}"
                    jar_file="${MC_DIR}/paper.jar"
                    if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 180 -o "$jar_file" "$paper_url" && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 1000000 ]]; then
                        download_ok=true
                    else
                        warn "BMCL 下载失败或文件异常，尝试官方源..."
                        rm -f "$jar_file"
                    fi
                fi
            fi

            # 官方 PaperMC API (国内失败或国外)
            if [[ "$download_ok" != "true" ]]; then
                if [[ -n "$MC_VERSION" ]]; then
                    paper_version="$MC_VERSION"
                else
                    paper_version=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://api.papermc.io/v2/projects/paper" 2>/dev/null | \
                        python3 -c "import sys,json; d=json.load(sys.stdin); print(d['versions'][-1])" 2>/dev/null || true)
                fi

                if [[ -z "$paper_version" ]]; then
                    paper_version="$default_version"
                fi

                paper_build=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds" 2>/dev/null | \
                    python3 -c "import sys,json; d=json.load(sys.stdin); builds=[b for b in d['builds'] if b['channel']=='default']; print(builds[-1]['build'])" 2>/dev/null || true)

                if [[ -z "$paper_build" ]]; then
                    # API 也失败，使用默认版本的已知构建
                    paper_version="$default_version"
                    paper_build="$default_build"
                fi

                paper_url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/${paper_build}/downloads/paper-${paper_version}-${paper_build}.jar"
                info "PaperMC 版本: ${paper_version}, 构建: ${paper_build}"
                jar_file="${MC_DIR}/paper.jar"
                if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 180 -o "$jar_file" "$paper_url" && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 1000000 ]]; then
                    download_ok=true
                fi
            fi

            # 最终兜底仍通过构建 API 获取真实构建号，避免伪 latest URL。
            if [[ "$download_ok" != "true" ]]; then
                warn "PaperMC 下载失败，重试默认稳定版本..."
                paper_version="$default_version"
                paper_build=$(set +o pipefail; curl -fsSL --max-time 15 \
                    "https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds" 2>/dev/null | \
                    python3 -c "import sys,json; d=json.load(sys.stdin); b=[x for x in d['builds'] if x['channel']=='default']; print(b[-1]['build'])" 2>/dev/null || true)
                [[ -z "$paper_build" ]] && paper_build="$default_build"
                paper_url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/${paper_build}/downloads/paper-${paper_version}-${paper_build}.jar"
                jar_file="${MC_DIR}/paper.jar"
                if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 300 -o "$jar_file" "$paper_url" && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 1000000 ]]; then
                    download_ok=true
                fi
            fi

            if [[ "$download_ok" != "true" ]]; then
                error "Paper 下载失败，请检查网络或手动下载"
                error "手动下载地址: https://papermc.io/downloads/paper"
                exit 1
            fi
            ;;

        vanilla)
            local server_jar_url=""
            local download_ok=false

            if [[ "$use_bmcl" == "true" ]]; then
                # 国内: BMCL 镜像
                local mc_version
                if [[ -n "$MC_VERSION" ]]; then
                    mc_version="$MC_VERSION"
                else
                    mc_version=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://bmclapidoc.bangbang93.com/mc/game/version_manifest.json" 2>/dev/null | \
                        python3 -c "import sys,json; print(json.load(sys.stdin)['latest']['release'])" 2>/dev/null || true)
                fi

                if [[ -n "$mc_version" ]]; then
                    server_jar_url="https://bmclapidoc.bangbang93.com/version/${mc_version}/server"
                    info "BMCL 原版版本: ${mc_version}"
                fi
            fi

            # 国外或 BMCL 失败: 官方 Mojang API
            if [[ -z "$server_jar_url" ]]; then
                local version_json_url
                version_json_url=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://launchermeta.mojang.com/mc/game/version_manifest.json" 2>/dev/null | \
                    python3 -c "
import sys,json
d=json.load(sys.stdin)
target='${MC_VERSION}' if '${MC_VERSION}' else d['latest']['release']
for v in d['versions']:
    if v['id']==target:
        print(v['url'])
        break
" 2>/dev/null || true)

                if [[ -n "$version_json_url" ]]; then
                    server_jar_url=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "$version_json_url" 2>/dev/null | \
                        python3 -c "import sys,json; d=json.load(sys.stdin); print(d['downloads']['server']['url'])" 2>/dev/null || true)
                fi
            fi

            if [[ -z "$server_jar_url" ]]; then
                error "无法获取原版服务器下载地址"
                exit 1
            fi

            info "下载地址: ${server_jar_url}"
            jar_file="${MC_DIR}/server.jar"
            if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 300 -o "$jar_file" "$server_jar_url" && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 1000000 ]]; then
                download_ok=true
            fi

            if [[ "$download_ok" != "true" ]]; then
                error "原版服务器下载失败"
                exit 1
            fi
            ;;

        fabric)
            local fabric_version="0.16.14"
            local loader_version="1.0.1"
            local mc_version="1.21.4"
            local download_ok=false

            if [[ -n "$MC_VERSION" ]]; then
                mc_version="$MC_VERSION"
            else
                # 获取最新 MC 版本
                local latest_mc
                latest_mc=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://meta.fabricmc.net/v2/versions/game" 2>/dev/null | \
                    python3 -c "import sys,json; d=json.load(sys.stdin); print([v['version'] for v in d if v['stable']][0])" 2>/dev/null || true)
                [[ -n "$latest_mc" ]] && mc_version="$latest_mc"
            fi

            info "Fabric 版本: ${fabric_version}, MC: ${mc_version}"

            # 方式1: 通过 Fabric API 下载服务端 jar
            local fabric_url="https://meta.fabricmc.net/v2/versions/loader/${mc_version}/${fabric_version}/${loader_version}/server/jar"
            jar_file="${MC_DIR}/fabric-server.jar"

            if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 180 -o "$jar_file" "$fabric_url" && [[ $(stat -c%s "$jar_file" 2>/dev/null || echo 0) -gt 500000 ]]; then
                download_ok=true
            fi

            # 方式2: 使用安装器
            if [[ "$download_ok" != "true" ]]; then
                warn "直接下载失败，尝试使用安装器..."
                local installer_jar="/tmp/fabric-installer.jar"
                curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 60 -o "$installer_jar" "https://maven.fabricmc.net/net/fabricmc/fabric-installer/1.0.1/fabric-installer-1.0.1.jar"
                if [[ -f "$installer_jar" ]] && [[ $(stat -c%s "$installer_jar" 2>/dev/null || echo 0) -gt 10000 ]]; then
                    sudo -u "$MC_USER" java -jar "$installer_jar" server -dir "$MC_DIR" -mcversion "$mc_version" -loader "$fabric_version" -downloadMinecraft 2>/dev/null
                    if [[ -f "${MC_DIR}/fabric-server.jar" ]]; then
                        jar_file="${MC_DIR}/fabric-server.jar"
                        download_ok=true
                    fi
                    rm -f "$installer_jar"
                fi
            fi

            if [[ "$download_ok" != "true" ]]; then
                error "Fabric 服务器下载失败"
                error "手动下载: https://fabricmc.net/use/server/"
                exit 1
            fi
            ;;

        forge)
            local mc_version="1.21.4"
            local forge_version="54.0.16"
            local download_ok=false

            if [[ -n "$MC_VERSION" ]]; then
                mc_version="$MC_VERSION"
            else
                # 获取最新 MC 版本
                local latest_mc
                latest_mc=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://files.minecraftforge.net/net/minecraftforge/forge/maven-metadata.json" 2>/dev/null | \
                    python3 -c "import sys,json; d=json.load(sys.stdin); versions=sorted(d.keys(), reverse=True); print(versions[0])" 2>/dev/null || true)
                [[ -n "$latest_mc" ]] && mc_version="$latest_mc"
            fi

            # 获取该 MC 版本的最新 Forge 版本
            local forge_versions_json
            forge_versions_json=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "https://files.minecraftforge.net/net/minecraftforge/forge/maven-metadata.json" 2>/dev/null | \
                python3 -c "
import sys,json
d=json.load(sys.stdin)
v='${mc_version}'
if v in d:
    builds=d[v]
    print(builds[-1] if isinstance(builds, list) else builds)
" 2>/dev/null || true)
            [[ -n "$forge_versions_json" ]] && forge_version="$forge_versions_json"

            info "Forge 版本: ${mc_version}-${forge_version}"

            # 下载 Forge 安装器
            local installer_jar="/tmp/forge-installer.jar"
            local forge_url="https://maven.minecraftforge.net/net/minecraftforge/forge/${mc_version}-${forge_version}/forge-${mc_version}-${forge_version}-installer.jar"

            curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 120 -o "$installer_jar" "$forge_url"

            if [[ ! -f "$installer_jar" ]] || [[ $(stat -c%s "$installer_jar" 2>/dev/null || echo 0) -lt 100000 ]]; then
                error "Forge 安装器下载失败"
                error "手动下载: https://files.minecraftforge.net/"
                exit 1
            fi

            # 安装 Forge 服务端
            info "安装 Forge 服务端 (这可能需要几分钟)..."
            cd "$MC_DIR"
            sudo -u "$MC_USER" java -jar "$installer_jar" --installServer . 2>/dev/null
            rm -f "$installer_jar"

            # Forge installation is driven by its generated run.sh/argument files.
            if [[ -x "${MC_DIR}/run.sh" ]]; then
                jar_file="${MC_DIR}/run.sh"
                download_ok=true
            else
                local forge_jar
                forge_jar=$(find "$MC_DIR" -maxdepth 1 -type f -name 'forge-*.jar' -print -quit 2>/dev/null || true)
                if [[ -n "$forge_jar" ]]; then
                    jar_file="$forge_jar"
                    download_ok=true
                elif [[ -f "${MC_DIR}/libraries/net/minecraftforge/forge/${mc_version}-${forge_version}/forge-${mc_version}-${forge_version}-server.jar" ]]; then
                    jar_file="${MC_DIR}/libraries/net/minecraftforge/forge/${mc_version}-${forge_version}/forge-${mc_version}-${forge_version}-server.jar"
                    download_ok=true
                fi
            fi

            if [[ "$download_ok" != "true" ]]; then
                error "Forge 安装失败"
                error "手动下载: https://files.minecraftforge.net/"
                exit 1
            fi
            ;;
    esac

    info "服务器下载完成: $jar_file ($(du -h "$jar_file" | cut -f1))"
    chown "${MC_USER}:${MC_USER}" "$jar_file"
}

# ==================== 首次启动生成配置文件 ====================
generate_configs() {
    info "生成配置文件..."

    # 同意 EULA
    cat > "${MC_DIR}/eula.txt" << EOF
eula=true
EOF

    # 首次启动以生成配置文件 (用较低内存避免 OOM)
    info "首次启动以生成配置文件 (首次需要下载依赖，可能需要 1-3 分钟)..."
    cd "$MC_DIR"

    local jar_file
    case "$SERVER_TYPE" in
        paper)   jar_file="paper.jar" ;;
        fabric)  jar_file="fabric-server.jar" ;;
        forge)
            jar_file=$(ls forge-*.jar 2>/dev/null | head -1)
            jar_file="${jar_file:-server.jar}"
            ;;
        *)       jar_file="server.jar" ;;
    esac

    local java_pid
    if [[ "$SERVER_TYPE" == "forge" && -x "${MC_DIR}/run.sh" ]]; then
        sudo -u "$MC_USER" env JAVA_ARGS="-Xms${MC_MEMORY_MIN} -Xmx${MC_MEMORY_MIN}" bash "${MC_DIR}/run.sh" --nogui &
    else
        sudo -u "$MC_USER" java -Xms${MC_MEMORY_MIN} -Xmx${MC_MEMORY_MIN} -jar "$jar_file" --nogui &
    fi
    java_pid=$!

    # 等待 server.properties 生成 (最多 180 秒)
    local wait_count=0
    while [[ ! -f "${MC_DIR}/server.properties" ]] && [[ $wait_count -lt 90 ]]; do
        sleep 2
        wait_count=$((wait_count + 1))
        # 检查 java 进程是否还活着
        if ! kill -0 "$java_pid" 2>/dev/null; then
            break
        fi
    done

    # 停止服务器
    kill -TERM "$java_pid" 2>/dev/null || true
    # Only wait for the exact process started above; never kill other Java instances.
    local stop_wait=0
    while kill -0 "$java_pid" 2>/dev/null && [[ $stop_wait -lt 10 ]]; do
        sleep 1
        stop_wait=$((stop_wait + 1))
    done
    kill -KILL "$java_pid" 2>/dev/null || true
    wait "$java_pid" 2>/dev/null || true
    sleep 2

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
    local extra_args="--nogui"
    case "$SERVER_TYPE" in
        paper)   jar_file="paper.jar" ;;
        vanilla) jar_file="server.jar" ;;
        fabric)  jar_file="fabric-server.jar" ;;
        forge)
            if [[ -x "${MC_DIR}/run.sh" ]]; then
                jar_file="run.sh"
            else
                jar_file=$(find "${MC_DIR}" -maxdepth 1 -type f -name 'forge-*.jar' -printf '%f\n' -quit 2>/dev/null || true)
                [[ -z "$jar_file" ]] && { error "Forge 启动文件不存在"; exit 1; }
            fi
            ;;
    esac

    cat > "${MC_DIR}/start.sh" << STARTSCRIPT
#!/bin/bash
cd "${MC_DIR}"

if [[ "${SERVER_TYPE}" == "forge" && "${jar_file}" == "run.sh" ]]; then
    exec ./run.sh --nogui
else
    exec java \\
        -Xms${MC_MEMORY_MIN} \\
        -Xmx${MC_MEMORY} \\
        ${JVM_FLAGS[*]} \\
        -jar "${jar_file}" ${extra_args}
fi
STARTSCRIPT

    chmod +x "${MC_DIR}/start.sh"
    chown "${MC_USER}:${MC_USER}" "${MC_DIR}/start.sh"
    info "启动脚本: ${MC_DIR}/start.sh"
}

# ==================== 创建 systemd 服务 ====================
create_systemd_service() {
    info "创建 systemd 服务..."

    # systemd 支持 K/M/G/T 后缀，直接在用户配置上增加固定运行时余量。
    local mem_limit
    case "$MC_MEMORY" in
        *[Gg]) mem_limit="$(( ${MC_MEMORY%[Gg]} + 1 ))G" ;;
        *[Mm]) mem_limit="$(( ${MC_MEMORY%[Mm]} + 1024 ))M" ;;
        *[Kk]) mem_limit="$(( ${MC_MEMORY%[Kk]} + 1048576 ))K" ;;
        *)
            error "JVM 内存格式无效: ${MC_MEMORY}（示例: 4G 或 4096M）"
            exit 1
            ;;
    esac

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
ExecStop=/bin/kill -SIGINT \$MAINPID
TimeoutStopSec=120

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
CREDENTIALS_FILE="/etc/minecraft/credentials.env"
source "$CREDENTIALS_FILE"
RCON_PORT="$MC_RCON_PORT"
RCON_PASSWORD="$MC_RCON_PASSWORD"

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
    echo "服务器控制:"
    echo "  start       启动服务器"
    echo "  stop        停止服务器"
    echo "  restart     重启服务器"
    echo "  status      查看状态"
    echo "  logs        实时日志"
    echo "  console     显示控制台替代方式"
    echo "  cmd <命令>  执行服务器命令"
    echo "  players     查看在线玩家"
    echo "  say <消息>  广播消息"
    echo "  whitelist   白名单管理"
    echo ""
    echo "运维管理:"
    echo "  backup      立即备份"
    echo "  update      更新服务器"
    echo "  config      编辑配置"
    echo "  memory      查看内存"
    echo "  info        服务器信息"
    echo ""
    echo "内容管理:"
    echo "  plugin      插件管理 (搜索/安装/列表/删除)"
    echo "  datapack    数据包管理 (安装/列表/删除/重载)"
    echo "  resourcepack 资源包配置 (设置/移除)"
    echo "  packs       查看已安装内容总览"
    echo ""
}

cmd_start()   { systemctl start "$SERVICE" && echo -e "${GREEN}服务器已启动${NC}"; }
cmd_stop()    { systemctl stop "$SERVICE" && echo -e "${YELLOW}服务器已停止${NC}"; }
cmd_restart() { systemctl restart "$SERVICE" && echo -e "${GREEN}服务器已重启${NC}"; }
cmd_status()  { systemctl status "$SERVICE" --no-pager; }
cmd_logs()    { journalctl -u "$SERVICE" -f --no-pager; }

cmd_console() {
    echo -e "${YELLOW}systemd 直接管理 Java 进程，不再使用 screen 控制台。${NC}"
    echo "请使用: mc-manager cmd <命令> 或 journalctl -u $SERVICE -f"
    return 1
}

cmd_cmd() {
    if [[ -z "$*" ]]; then
        echo "用法: mc-manager cmd <服务器命令>"
        return 1
    fi
    if command -v mcrcon &>/dev/null; then
        mcrcon -H 127.0.0.1 -P "$RCON_PORT" -p "$RCON_PASSWORD" "$*"
    else
        echo -e "${YELLOW}未安装 mcrcon，无法发送命令。可安装 mcrcon 后重试。${NC}"
        return 1
    fi
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

    # 先保存世界（若安装了 mcrcon）
    if command -v mcrcon &>/dev/null; then
        mcrcon -H 127.0.0.1 -P "$RCON_PORT" -p "$RCON_PASSWORD" "save-off" "save-all" 2>/dev/null || true
        sleep 3
    fi

    tar -czf "$backup_file" -C "${MC_DIR}" world 2>/dev/null

    if command -v mcrcon &>/dev/null; then
        mcrcon -H 127.0.0.1 -P "$RCON_PORT" -p "$RCON_PASSWORD" "save-on" 2>/dev/null || true
    fi

    if [[ -f "$backup_file" ]]; then
        echo -e "${GREEN}备份成功: ${backup_file}${NC}"
        # 保留最近20个备份
        ls -t "${backup_dir}"/world_backup_*.tar.gz 2>/dev/null | tail -n +21 | xargs -r rm -f
    else
        echo -e "${RED}备份失败${NC}"
    fi
}

cmd_update() {
    if [[ $EUID -ne 0 ]]; then
        echo -e "${RED}更新需要 root${NC}" >&2
        return 1
    fi
    echo -e "${CYAN}更新服务器...${NC}"
    local was_active=false
    systemctl is-active --quiet "$SERVICE" && was_active=true
    systemctl stop "$SERVICE" || true

    local server_type="paper"
    if [[ -f "${MC_DIR}/paper.jar" ]]; then
        server_type="paper"
    elif [[ -f "${MC_DIR}/fabric-server.jar" ]]; then
        server_type="fabric"
    elif [[ -x "${MC_DIR}/run.sh" ]] || find "$MC_DIR" -maxdepth 1 -name 'forge-*.jar' -print -quit | grep -q .; then
        server_type="forge"
    elif [[ -f "${MC_DIR}/server.jar" ]]; then
        server_type="vanilla"
    fi

    if [[ "$server_type" == "forge" ]]; then
        echo -e "${YELLOW}Forge 更新需要运行对应版本安装器，未修改现有文件。${NC}"
        $was_active && systemctl start "$SERVICE" || true
        return 1
    fi

    local jar_file url tmp_file old_hash new_hash
    case "$server_type" in
        paper)
            jar_file="${MC_DIR}/paper.jar"
            local paper_version paper_build
            paper_version=$(curl -fsSL --max-time 20 "https://api.papermc.io/v2/projects/paper" | python3 -c "import sys,json; print(json.load(sys.stdin)['versions'][-1])") || {
                $was_active && systemctl start "$SERVICE" || true; return 1;
            }
            paper_build=$(curl -fsSL --max-time 20 "https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds" | python3 -c "import sys,json; d=json.load(sys.stdin); print([b for b in d['builds'] if b['channel']=='default'][-1]['build'])") || {
                $was_active && systemctl start "$SERVICE" || true; return 1;
            }
            url="https://api.papermc.io/v2/projects/paper/versions/${paper_version}/builds/${paper_build}/downloads/paper-${paper_version}-${paper_build}.jar"
            ;;
        vanilla)
            jar_file="${MC_DIR}/server.jar"
            local version_json_url
            version_json_url=$(curl -fsSL --max-time 20 "https://launchermeta.mojang.com/mc/game/version_manifest.json" | python3 -c "import sys,json; d=json.load(sys.stdin); latest=d['latest']['release']; print(next(v['url'] for v in d['versions'] if v['id']==latest))") || {
                $was_active && systemctl start "$SERVICE" || true; return 1;
            }
            url=$(curl -fsSL --max-time 20 "$version_json_url" | python3 -c "import sys,json; print(json.load(sys.stdin)['downloads']['server']['url'])") || {
                $was_active && systemctl start "$SERVICE" || true; return 1;
            }
            ;;
        fabric)
            jar_file="${MC_DIR}/fabric-server.jar"
            local mc_version
            mc_version=$(curl -fsSL --max-time 20 "https://meta.fabricmc.net/v2/versions/game" | python3 -c "import sys,json; print(next(v['version'] for v in json.load(sys.stdin) if v['stable']))") || {
                $was_active && systemctl start "$SERVICE" || true; return 1;
            }
            url="https://meta.fabricmc.net/v2/versions/loader/${mc_version}/0.16.14/1.0.1/server/jar"
            ;;
    esac

    tmp_file="${jar_file}.new"
    rm -f "$tmp_file"
    if ! curl -fL --connect-timeout 20 --retry 3 --max-time 300 -o "$tmp_file" "$url" || \
       [[ $(stat -c%s "$tmp_file" 2>/dev/null || echo 0) -lt 500000 ]]; then
        rm -f "$tmp_file"
        echo -e "${RED}下载或文件校验失败，保留原版本${NC}" >&2
        $was_active && systemctl start "$SERVICE" || true
        return 1
    fi

    old_hash=$(sha256sum "$jar_file" | awk '{print $1}')
    new_hash=$(sha256sum "$tmp_file" | awk '{print $1}')
    if [[ "$old_hash" == "$new_hash" ]]; then
        rm -f "$tmp_file"
        echo -e "${GREEN}已是最新版本${NC}"
    else
        cp -p "$jar_file" "${jar_file}.bak"
        chown --reference="$jar_file" "$tmp_file"
        chmod --reference="$jar_file" "$tmp_file"
        mv -f "$tmp_file" "$jar_file"
        echo -e "${GREEN}更新完成；旧版本: ${jar_file}.bak${NC}"
    fi
    $was_active && systemctl start "$SERVICE" || true
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
    # 从日志获取版本信息
    local mc_version
    mc_version=$(journalctl -u "$SERVICE" --no-pager 2>/dev/null | grep -oP 'version \K[0-9]+\.[0-9]+\.[0-9]+' | tail -1)
    local server_jar
    if [[ -f "${MC_DIR}/paper.jar" ]]; then
        server_jar="Paper"
    elif [[ -f "${MC_DIR}/fabric-server.jar" ]]; then
        server_jar="Fabric"
    elif ls "${MC_DIR}"/forge-*.jar &>/dev/null; then
        server_jar="Forge"
    elif [[ -f "${MC_DIR}/server.jar" ]]; then
        server_jar="Vanilla"
    else
        server_jar="Unknown"
    fi
    echo -e "${CYAN}=== 服务器信息 ===${NC}"
    echo "服务器类型:  ${server_jar}"
    echo "游戏版本:    ${mc_version:-未知}"
    echo "服务器地址:  ${ip}:${MC_PORT}"
    echo "RCON端口:    ${RCON_PORT}"
    echo "RCON密码:    ${RCON_PASSWORD}"
    echo "状态:        $(systemctl is-active "$SERVICE")"
    echo "配置文件:    ${MC_DIR}/server.properties"
    echo "世界目录:    ${MC_DIR}/world"
}

# ==================== 插件管理 (Modrinth API) ====================
cmd_plugin() {
    local subcmd="${1:-help}"
    shift 2>/dev/null

    local plugins_dir="${MC_DIR}/plugins"
    local modrinth_api="https://api.modrinth.com/v2"
    # 通过版本清单获取当前 MC 版本
    local mc_version
    mc_version=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 10 "${modrinth_api}/search?query=placeholder&limit=1" 2>/dev/null | \
        python3 -c "import sys,json; print(json.load(sys.stdin)['hits'][0]['versions'][-1])" 2>/dev/null || echo "")

    case "$subcmd" in
        search)
            local query="$*"
            if [[ -z "$query" ]]; then
                echo "用法: mc-manager plugin search <关键词>"
                return 1
            fi
            echo -e "${CYAN}搜索插件: ${query}${NC}"
            local result
            result=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "${modrinth_api}/search?query=${query}&facets=%5B%5B%22project_type%3Aplugin%22%5D%5D&limit=10" 2>/dev/null || true)
            if [[ -z "$result" ]]; then
                echo -e "${YELLOW}无法连接 Modrinth API，请检查网络${NC}"
                return 1
            fi
            echo "$result" | python3 -c "
import sys, json
d = json.load(sys.stdin)
hits = d.get('hits', [])
if not hits:
    print('  未找到结果')
else:
    print(f'  {\"序号\":<4} {\"名称\":<25} {\"下载量\":<12} {\"简介\"}')
    print(f'  {\"-\"*4} {\"-\"*25} {\"-\"*12} {\"-\"*40}')
    for i, h in enumerate(hits, 1):
        desc = h.get('description', '')[:50]
        dl = h.get('downloads', 0)
        if dl >= 1000000:
            dl_str = f'{dl/1000000:.1f}M'
        elif dl >= 1000:
            dl_str = f'{dl/1000:.0f}K'
        else:
            dl_str = str(dl)
        print(f'  {i:<4} {h[\"title\"]:<25} {dl_str:<12} {desc}')
    print()
    print('  安装: mc-manager plugin install <插件名>')
" 2>/dev/null || echo -e "${YELLOW}解析结果失败${NC}"
            ;;

        install)
            local query="$*"
            if [[ -z "$query" ]]; then
                echo "用法: mc-manager plugin install <插件名>"
                return 1
            fi
            mkdir -p "$plugins_dir"

            echo -e "${CYAN}搜索插件: ${query}${NC}"
            local result
            result=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "${modrinth_api}/search?query=${query}&facets=%5B%5B%22project_type%3Aplugin%22%5D%5D&limit=5" 2>/dev/null || true)

            if [[ -z "$result" ]]; then
                echo -e "${YELLOW}无法连接 Modrinth API，尝试直接下载...${NC}"
                # 尝试用 slug 直接获取
                local project_id="$query"
                local versions_json
                versions_json=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "${modrinth_api}/project/${project_id}/version" 2>/dev/null || true)
                if [[ -n "$versions_json" ]]; then
                    local dl_url filename
                    dl_url=$(echo "$versions_json" | python3 -c "import sys,json; v=json.load(sys.stdin); print(v[0]['files'][0]['url'])" 2>/dev/null || true)
                    filename=$(echo "$versions_json" | python3 -c "import sys,json; v=json.load(sys.stdin); print(v[0]['files'][0]['filename'])" 2>/dev/null || true)
                    if [[ -n "$dl_url" ]]; then
                        echo "下载: ${filename}"
                        curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 120 -o "${plugins_dir}/${filename}" "$dl_url"
                        echo -e "${GREEN}安装成功: ${filename}${NC}"
                        echo -e "${YELLOW}重启服务器生效: mc-manager restart${NC}"
                        return 0
                    fi
                fi
                echo -e "${RED}下载失败，请手动将 .jar 文件放入 ${plugins_dir}/${NC}"
                return 1
            fi

            # 让用户选择或自动安装第一个
            local project_id project_name
            project_id=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['hits'][0]['slug'])" 2>/dev/null || true)
            project_name=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['hits'][0]['title'])" 2>/dev/null || true)

            if [[ -z "$project_id" ]]; then
                echo -e "${RED}未找到匹配的插件${NC}"
                return 1
            fi

            echo -e "找到: ${GREEN}${project_name}${NC} (${project_id})"

            # 获取版本信息
            local versions_json
            versions_json=$(set +o pipefail; curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 15 "${modrinth_api}/project/${project_id}/version" 2>/dev/null || true)

            local dl_url filename
            dl_url=$(echo "$versions_json" | python3 -c "import sys,json; v=json.load(sys.stdin); print(v[0]['files'][0]['url'])" 2>/dev/null || true)
            filename=$(echo "$versions_json" | python3 -c "import sys,json; v=json.load(sys.stdin); print(v[0]['files'][0]['filename'])" 2>/dev/null || true)

            if [[ -z "$dl_url" ]]; then
                echo -e "${RED}获取下载地址失败${NC}"
                return 1
            fi

            # 检查是否已安装
            if [[ -f "${plugins_dir}/${filename}" ]]; then
                echo -e "${YELLOW}已安装: ${filename}${NC}"
                return 0
            fi

            echo "下载: ${filename}..."
            if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 120 -o "${plugins_dir}/${filename}" "$dl_url" && [[ $(stat -c%s "${plugins_dir}/${filename}" 2>/dev/null || echo 0) -gt 10000 ]]; then
                echo -e "${GREEN}安装成功: ${filename}${NC}"
                echo -e "${YELLOW}重启服务器生效: mc-manager restart${NC}"
            else
                rm -f "${plugins_dir}/${filename}"
                echo -e "${RED}下载失败${NC}"
                return 1
            fi
            ;;

        list)
            echo -e "${CYAN}=== 已安装插件 ===${NC}"
            if [[ -d "$plugins_dir" ]] && ls "$plugins_dir"/*.jar &>/dev/null; then
                for f in "$plugins_dir"/*.jar; do
                    local size
                    size=$(du -h "$f" | cut -f1)
                    echo "  $(basename "$f") ($size)"
                done
                echo ""
                echo "共 $(ls "$plugins_dir"/*.jar 2>/dev/null | wc -l) 个插件"
            else
                echo "  暂无已安装插件"
                echo "  安装: mc-manager plugin install <插件名>"
            fi
            ;;

        remove)
            local name="$*"
            if [[ -z "$name" ]]; then
                echo "用法: mc-manager plugin remove <插件文件名>"
                echo "查看列表: mc-manager plugin list"
                return 1
            fi
            # 支持模糊匹配
            local found
            found=$(ls "$plugins_dir"/${name}*.jar 2>/dev/null | head -1)
            if [[ -n "$found" ]]; then
                rm -f "$found"
                echo -e "${GREEN}已删除: $(basename "$found")${NC}"
                echo -e "${YELLOW}重启服务器生效: mc-manager restart${NC}"
            else
                echo -e "${RED}未找到插件: ${name}${NC}"
                echo "已安装插件:"
                ls "$plugins_dir"/*.jar 2>/dev/null | xargs -I{} echo "  $(basename {})"
            fi
            ;;

        *)
            echo "插件管理:"
            echo "  mc-manager plugin search <关键词>  搜索插件 (Modrinth)"
            echo "  mc-manager plugin install <名称>   安装插件"
            echo "  mc-manager plugin list             列出已安装插件"
            echo "  mc-manager plugin remove <名称>    删除插件"
            ;;
    esac
}

# ==================== 数据包管理 ====================
cmd_datapack() {
    local subcmd="${1:-help}"
    shift 2>/dev/null

    local datapacks_dir="${MC_DIR}/world/datapacks"

    case "$subcmd" in
        install)
            local source="$*"
            if [[ -z "$source" ]]; then
                echo "用法: mc-manager datapack install <文件路径或URL>"
                return 1
            fi
            mkdir -p "$datapacks_dir"

            local filename=""
            if [[ "$source" =~ ^https?:// ]]; then
                filename=$(basename "$source" | sed 's/?.*//')
                echo "下载数据包: ${filename}..."
                if curl -fL --connect-timeout 20 --retry 3 --retry-delay 3 --max-time 120 -o "${datapacks_dir}/${filename}" "$source" && \
                   [[ $(stat -c%s "${datapacks_dir}/${filename}" 2>/dev/null || echo 0) -gt 100 ]]; then
                    echo -e "${GREEN}安装成功: ${filename}${NC}"
                else
                    rm -f "${datapacks_dir}/${filename}"
                    echo -e "${RED}下载失败${NC}"
                    return 1
                fi
            elif [[ -f "$source" ]]; then
                filename=$(basename "$source")
                cp "$source" "${datapacks_dir}/${filename}"
                echo -e "${GREEN}安装成功: ${filename}${NC}"
            else
                echo -e "${RED}文件不存在: ${source}${NC}"
                return 1
            fi
            echo -e "${YELLOW}重载数据包: mc-manager datapack reload${NC}"
            ;;

        list)
            echo -e "${CYAN}=== 已安装数据包 ===${NC}"
            if [[ -d "$datapacks_dir" ]] && ls "$datapacks_dir"/*.zip &>/dev/null; then
                for f in "$datapacks_dir"/*.zip; do
                    local size
                    size=$(du -h "$f" | cut -f1)
                    echo "  $(basename "$f") ($size)"
                done
                echo ""
                echo "共 $(ls "$datapacks_dir"/*.zip 2>/dev/null | wc -l) 个数据包"
            else
                echo "  暂无已安装数据包"
                echo "  安装: mc-manager datapack install <文件路径或URL>"
            fi
            ;;

        remove)
            local name="$*"
            if [[ -z "$name" ]]; then
                echo "用法: mc-manager datapack remove <数据包名>"
                return 1
            fi
            local found
            found=$(ls "$datapacks_dir"/${name}*.zip 2>/dev/null | head -1)
            if [[ -n "$found" ]]; then
                rm -f "$found"
                echo -e "${GREEN}已删除: $(basename "$found")${NC}"
                echo -e "${YELLOW}重载数据包: mc-manager datapack reload${NC}"
            else
                echo -e "${RED}未找到数据包: ${name}${NC}"
            fi
            ;;

        reload)
            echo "重载数据包..."
            cmd_cmd "reload"
            echo -e "${GREEN}已发送重载命令${NC}"
            ;;

        *)
            echo "数据包管理:"
            echo "  mc-manager datapack install <路径或URL>  安装数据包"
            echo "  mc-manager datapack list                 列出已安装"
            echo "  mc-manager datapack remove <名称>        删除数据包"
            echo "  mc-manager datapack reload               重载数据包"
            ;;
    esac
}

# ==================== 资源包配置 ====================
cmd_resourcepack() {
    local subcmd="${1:-help}"
    shift 2>/dev/null

    local props="${MC_DIR}/server.properties"

    case "$subcmd" in
        set)
            local url="$1"
            local sha1="$2"
            if [[ -z "$url" ]]; then
                echo "用法: mc-manager resourcepack set <URL> [SHA1]"
                echo "示例: mc-manager resourcepack set https://example.com/pack.zip abc123..."
                return 1
            fi
            # 计算 SHA1（如果是本地文件）
            if [[ -f "$url" ]]; then
                sha1=$(sha1sum "$url" | awk '{print $1}')
                echo -e "${YELLOW}注意: 本地文件需要上传到网络，客户端才能下载${NC}"
                echo -e "${YELLOW}SHA1: ${sha1}${NC}"
            fi
            # 修改 server.properties
            if [[ -f "$props" ]]; then
                sed -i "s|^resource-pack=.*|resource-pack=${url}|" "$props"
                if [[ -n "$sha1" ]]; then
                    sed -i "s|^resource-pack-sha1=.*|resource-pack-sha1=${sha1}|" "$props"
                fi
                sed -i "s|^require-resource-pack=.*|require-resource-pack=false|" "$props"
                echo -e "${GREEN}资源包已设置${NC}"
                echo "URL: ${url}"
                [[ -n "$sha1" ]] && echo "SHA1: ${sha1}"
                echo -e "${YELLOW}重启服务器生效: mc-manager restart${NC}"
            else
                echo -e "${RED}配置文件不存在: ${props}${NC}"
            fi
            ;;

        remove)
            if [[ -f "$props" ]]; then
                sed -i "s|^resource-pack=.*|resource-pack=|" "$props"
                sed -i "s|^resource-pack-sha1=.*|resource-pack-sha1=|" "$props"
                sed -i "s|^require-resource-pack=.*|require-resource-pack=false|" "$props"
                echo -e "${GREEN}资源包配置已移除${NC}"
                echo -e "${YELLOW}重启服务器生效: mc-manager restart${NC}"
            fi
            ;;

        *)
            echo "资源包管理:"
            echo "  mc-manager resourcepack set <URL> [SHA1]  设置服务器资源包"
            echo "  mc-manager resourcepack remove            移除资源包配置"
            echo ""
            echo "资源包需要托管在可公开下载的 URL 上"
            echo "推荐: 将 .zip 上传到对象存储 (腾讯云COS/阿里云OSS) 获取链接"
            ;;
    esac
}

# ==================== 内容总览 ====================
cmd_packs() {
    echo -e "${CYAN}${BOLD}=== Minecraft 服务器内容总览 ===${NC}"
    echo ""

    # 插件
    echo -e "${GREEN}[插件]${NC} ${MC_DIR}/plugins/"
    if ls "${MC_DIR}/plugins/"*.jar &>/dev/null; then
        for f in "${MC_DIR}/plugins/"*.jar; do
            echo "  - $(basename "$f")"
        done
    else
        echo "  (无)"
    fi
    echo ""

    # 数据包
    echo -e "${GREEN}[数据包]${NC} ${MC_DIR}/world/datapacks/"
    if ls "${MC_DIR}/world/datapacks/"*.zip &>/dev/null; then
        for f in "${MC_DIR}/world/datapacks/"*.zip; do
            echo "  - $(basename "$f")"
        done
    else
        echo "  (无)"
    fi
    echo ""

    # 资源包
    echo -e "${GREEN}[资源包]${NC}"
    local rp
    rp=$(grep "^resource-pack=" "${MC_DIR}/server.properties" 2>/dev/null | cut -d= -f2)
    if [[ -n "$rp" ]]; then
        echo "  URL: ${rp}"
    else
        echo "  (未配置)"
    fi
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
    version)  cmd_info ;;
    plugin)   shift; cmd_plugin "$@" ;;
    datapack) shift; cmd_datapack "$@" ;;
    resourcepack) shift; cmd_resourcepack "$@" ;;
    packs)    cmd_packs ;;
    *)        show_help ;;
esac
MANAGEREOF

    # 管理脚本含 RCON 密码，仅 root 可读执行。
    chown root:root "${MANAGER_SCRIPT}"
    chmod 700 "${MANAGER_SCRIPT}"
    info "管理脚本: ${MANAGER_SCRIPT} (root:root 700)"
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
User=root
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
        info "已添加游戏端口规则；RCON ${MC_RCON_PORT}/tcp 默认不向公网开放"
        if ! ufw status 2>/dev/null | grep -qi "Status: active"; then
            warn "UFW 规则已添加，但 UFW 当前未启用"
        fi
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${MC_PORT}/tcp"
        firewall-cmd --reload
        info "已开放游戏端口；RCON ${MC_RCON_PORT}/tcp 默认不向公网开放"
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
    echo -e "    mc-manager start        # 启动"
    echo -e "    mc-manager stop         # 停止"
    echo -e "    mc-manager restart      # 重启"
    echo -e "    mc-manager status       # 状态"
    echo -e "    mc-manager logs         # 日志"
    echo -e "    mc-manager console      # 进入控制台"
    echo -e "    mc-manager cmd <命令>   # 执行命令"
    echo -e "    mc-manager players      # 在线玩家"
    echo -e "    mc-manager say <消息>   # 广播"
    echo -e "    mc-manager backup       # 备份"
    echo -e "    mc-manager update       # 更新"
    echo -e "    mc-manager config       # 编辑配置"
    echo -e "    mc-manager info         # 服务器信息"
    echo ""
    echo -e "  ${YELLOW}${BOLD}内容管理:${NC}"
    echo -e "    mc-manager plugin search <关键词>   # 搜索插件"
    echo -e "    mc-manager plugin install <名称>    # 安装插件"
    echo -e "    mc-manager plugin list              # 已安装插件"
    echo -e "    mc-manager datapack install <URL>   # 安装数据包"
    echo -e "    mc-manager resourcepack set <URL>   # 设置资源包"
    echo -e "    mc-manager packs                    # 查看所有内容"
    echo ""
    echo -e "  ${RED}${BOLD}!!! 重要: 云服务器安全组配置 !!!${NC}"
    echo -e "  ${YELLOW}系统防火墙已自动放行，但云服务器还需在控制台配置安全组:${NC}"
    echo ""
    echo -e "  ┌──────────────┬──────────┬────────────────────────────┐"
    echo -e "  │    端口      │   协议   │         用途               │"
    echo -e "  ├──────────────┼──────────┼────────────────────────────┤"
    echo -e "  │    ${MC_PORT}     │   TCP    │  游戏主端口 (必须)         │"
    echo -e "  │    ${MC_RCON_PORT}     │   TCP    │  RCON（仅本机/SSH隧道）      │"
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
    mkdir -p "$(dirname "$CREDENTIALS_FILE")"
    umask 077
    {
        printf 'MC_RCON_PORT=%q\n' "$MC_RCON_PORT"
        printf 'MC_RCON_PASSWORD=%q\n' "$MC_RCON_PASSWORD"
    } > "$CREDENTIALS_FILE"
    chown root:root "$CREDENTIALS_FILE"
    chmod 600 "$CREDENTIALS_FILE"

    echo -e "\n${CYAN}${BOLD}即将执行部署步骤:${NC}"
    echo "  [1] 安装依赖"
    echo "  [2] 安装 Java 21"
    echo "  [3] 创建用户和目录"
    echo "  [4] 下载 ${SERVER_TYPE} 服务器"
    echo "  [5] 生成配置文件"
    echo "  [6] 写入优化配置"
    echo "  [7] 创建启动脚本"
    echo "  [8] 创建 systemd 服务"
    echo "  [9] 创建管理脚本"
    echo "  [10] 创建自动备份"
    echo "  [11] 配置防火墙"
    echo "  [12] 启动服务器"
    echo ""
    echo ""
    if [[ "$NONINTERACTIVE" != "1" ]]; then
        read -rp "回车开始部署 / 输入 n 取消: " confirm
        if [[ "$confirm" == "n" || "$confirm" == "N" ]]; then
            echo "已取消"
            exit 0
        fi
    fi

    install_deps
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
