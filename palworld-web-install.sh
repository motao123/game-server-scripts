#!/bin/bash
#============================================================
# 幻兽帕鲁 Web 管理面板 - 安装脚本
# 依赖: palworld-server-install.sh 已执行（需 pal-rcon + PalWorldSettings.ini）
#============================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

PAL_SETTINGS="/home/steam/Steam/steamapps/common/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
WEB_APP_SRC="$(dirname "$(readlink -f "$0")")/palworld-web-ui.py"
WEB_APP_DEST="/usr/local/bin/pal-web-ui"
WEB_ENV_FILE="/etc/pal-web.env"
SERVICE_NAME="pal-web"

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
step()  { echo -e "\n${CYAN}${BOLD}========== $1 ==========${NC}\n"; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "请使用 root 运行: sudo bash $0"
        exit 1
    fi
}

check_deps() {
    if [[ ! -f /usr/local/bin/pal-rcon ]]; then
        error "未找到 /usr/local/bin/pal-rcon，请先运行 palworld-server-install.sh"
        exit 1
    fi
    if [[ ! -f "$PAL_SETTINGS" ]]; then
        error "未找到 PalWorldSettings.ini: $PAL_SETTINGS"
        error "请先完成幻兽帕鲁服务器部署"
        exit 1
    fi
    if [[ ! -f "$WEB_APP_SRC" ]]; then
        error "未找到 Web 应用源码: $WEB_APP_SRC"
        error "请确认 palworld-web-ui.py 与本脚本在同一目录"
        exit 1
    fi
    command -v python3 &>/dev/null || { error "缺少 python3"; exit 1; }
}

# 从 PalWorldSettings.ini 解析配置项
read_pal_setting() {
    local key="$1"
    grep -oE "${key}=[^,)]*" "$PAL_SETTINGS" 2>/dev/null | head -1 | sed "s/^${key}=//" | tr -d '"' || echo ""
}

# ==================== 用户交互 ====================
user_config() {
    step "Web 面板配置"

    echo -e "  ${CYAN}默认配置:${NC}"
    echo -e "  ┌─────────────────────────────────────────────┐"
    echo -e "  │  1) Web 端口:     ${WEB_PORT}"
    echo -e "  │  2) 绑定地址:     ${WEB_BIND} (公网可访问)"
    echo -e "  │  3) Web 密码:     (留空自动生成)"
    echo -e "  └─────────────────────────────────────────────┘"
    echo ""
    echo -e "  ${YELLOW}!! 公网暴露风险 !!${NC}"
    echo -e "  Web 面板能控制服务器（启停/踢人/广播），公网暴露需强密码 + 建议反代 HTTPS"
    echo ""
    read -rp "  回车使用默认 / 输入 c 自定义: " choice

    if [[ "$choice" == "c" || "$choice" == "C" ]]; then
        read -rp "  Web 端口 [${WEB_PORT}]: " input
        WEB_PORT="${input:-$WEB_PORT}"

        read -rp "  绑定地址 [${WEB_BIND}] (0.0.0.0=公网, 127.0.0.1=仅本地): " input
        WEB_BIND="${input:-$WEB_BIND}"

        read -rp "  Web 密码 (留空自动生成 18 位随机): " input
        WEB_PASSWORD="${input:-$WEB_PASSWORD}"
    fi

    [[ -z "$WEB_PASSWORD" ]] && WEB_PASSWORD="$(openssl rand -base64 18 | tr -d '/+=' | cut -c1-18)"

    echo ""
    info "最终配置:"
    echo -e "    Web 端口:   ${CYAN}${WEB_PORT}${NC}"
    echo -e "    绑定地址:   ${CYAN}${WEB_BIND}${NC}"
    [[ "$WEB_BIND" == "0.0.0.0" ]] && echo -e "    ${YELLOW}公网暴露已开启，务必使用强密码 + 反代 HTTPS${NC}"
    echo -e "    Web 密码:   ${CYAN}${WEB_PASSWORD}${NC}"
    echo ""
}

# ==================== 安装 Web 应用 ====================
install_web_app() {
    step "安装 Web 应用"

    info "拷贝 $WEB_APP_SRC -> $WEB_APP_DEST"
    cp "$WEB_APP_SRC" "$WEB_APP_DEST"
    chmod 755 "$WEB_APP_DEST"
    chown root:root "$WEB_APP_DEST"

    # 从 PalWorldSettings.ini 读取 RCON 端口和管理员密码
    local rcon_port rcon_pass
    rcon_port=$(read_pal_setting "RCONPort")
    rcon_pass=$(read_pal_setting "AdminPassword")

    if [[ -z "$rcon_port" ]] || [[ -z "$rcon_pass" ]]; then
        error "无法从 PalWorldSettings.ini 读取 RCONPort 或 AdminPassword"
        error "请检查配置文件: $PAL_SETTINGS"
        exit 1
    fi

    info "RCON 端口: ${rcon_port}"
    info "RCON 密码: 已从配置读取"

    # 写环境变量文件（systemd EnvironmentFile，单引号包裹避免特殊字符问题）
    cat > "$WEB_ENV_FILE" <<EOF
# Palworld Web UI 配置 - 由 palworld-web-install.sh 生成
WEB_PASSWORD='${WEB_PASSWORD}'
RCON_PORT=${rcon_port}
RCON_PASS='${rcon_pass}'
SERVICE=pal-server
WEB_BIND=${WEB_BIND}
WEB_PORT=${WEB_PORT}
EOF
    chmod 600 "$WEB_ENV_FILE"
    chown root:root "$WEB_ENV_FILE"
    info "配置已写入: $WEB_ENV_FILE (权限 600)"
    info "Web 应用已安装: $WEB_APP_DEST"
}

# ==================== 创建 systemd 服务 ====================
create_service() {
    step "创建 systemd 服务"

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Palworld Web UI
After=network-online.target pal-server.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${WEB_ENV_FILE}
ExecStart=/usr/bin/python3 ${WEB_APP_DEST}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pal-web

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"
    info "systemd 服务已创建并启动: ${SERVICE_NAME}"
}

# ==================== 防火墙 ====================
setup_firewall() {
    step "配置防火墙"

    if command -v ufw &>/dev/null; then
        ufw allow "${WEB_PORT}/tcp" comment "Palworld Web UI"
        info "已开放 TCP ${WEB_PORT} (ufw)"
    elif command -v firewall-cmd &>/dev/null; then
        firewall-cmd --permanent --add-port="${WEB_PORT}/tcp"
        firewall-cmd --reload
        info "已开放 TCP ${WEB_PORT} (firewalld)"
    else
        warn "未检测到防火墙工具，请手动开放 TCP ${WEB_PORT}"
    fi
}

# ==================== 显示结果 ====================
show_result() {
    local ip_addr
    ip_addr=$(curl -s --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo -e "${GREEN}${BOLD}       Palworld Web 面板部署完成!${NC}"
    echo -e "${GREEN}${BOLD}============================================================${NC}"
    echo ""
    echo -e "  访问地址:  ${CYAN}http://${ip_addr}:${WEB_PORT}${NC}"
    [[ "$WEB_BIND" == "127.0.0.1" ]] && echo -e "  ${YELLOW}(仅本地，需 SSH 隧道访问)${NC}"
    echo -e "  Web 密码:  ${CYAN}${WEB_PASSWORD}${NC}"
    echo ""
    echo -e "  服务名:    ${SERVICE_NAME}"
    echo -e "  应用路径:  ${WEB_APP_DEST}"
    echo -e "  日志:       journalctl -u ${SERVICE_NAME} -f"
    echo ""
    echo -e "  ${YELLOW}${BOLD}!! 安全建议 !!${NC}"
    echo -e "  ${YELLOW}1. 强烈建议用 Nginx/Caddy 反代 + HTTPS${NC}"
    echo -e "  ${YELLOW}2. Web 密码不要与游戏 AdminPassword 相同${NC}"
    echo -e "  ${YELLOW}3. 定期检查登录日志: journalctl -u ${SERVICE_NAME} | grep login${NC}"
    echo -e "  ${YELLOW}4. 不再使用时关闭公网: 改 WEB_BIND=127.0.0.1 后重装${NC}"
    echo ""
    echo -e "${GREEN}${BOLD}============================================================${NC}"
}

# ==================== 主流程 ====================
main() {
    echo -e "${CYAN}${BOLD}"
    echo "  ____            _        _      __        __         _ "
    echo " |  _ \ __ _  __| | __ _| | ___ \\ \\      / /__  _ __| |"
    echo " | |_) / _\` |/ _\` |/ _\` | |/ _ \\ \\ \\ /\\ / / _ \\| '__| |"
    echo " |  __/ (_| | (_| | (_| | |  __/  \\ V  V / (_) | |  |_|"
    echo " |_|   \\__,_|\\__,_|\\__,_|_|\\___|   \\_/\\_/ \\___/|_|  (_)"
    echo -e "       Web UI Installer${NC}"
    echo ""

    # 默认值
    WEB_PORT="${WEB_PORT:-8080}"
    WEB_BIND="${WEB_BIND:-0.0.0.0}"
    WEB_PASSWORD="${WEB_PASSWORD:-}"

    check_root
    check_deps
    user_config
    install_web_app
    create_service
    setup_firewall
    show_result
}

main "$@"
