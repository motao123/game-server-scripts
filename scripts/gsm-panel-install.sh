#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-gsm-panel}"
INSTALL_DIR="${INSTALL_DIR:-/opt/gsm-panel}"
BIN_PATH="${BIN_PATH:-/usr/local/bin/gsm-panel}"
ENV_FILE="${ENV_FILE:-/etc/gsm-panel.env}"
DATA_DIR="${DATA_DIR:-/var/lib/gsm-panel}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/gsm-panel}"
WEB_BIND="${WEB_BIND:-0.0.0.0}"
WEB_PORT="${WEB_PORT:-8080}"
VERSION="${VERSION:-latest}"
ARCH="${ARCH:-}"
REPO="${REPO:-motao123/game-server-scripts}"
TARBALL="${TARBALL:-}"
NO_START="${NO_START:-false}"

usage() {
  cat <<EOF
Usage: sudo $0 <install|upgrade|rollback|status|logs|print-unit>

Environment:
  VERSION=${VERSION}
  ARCH=${ARCH:-auto}
  REPO=${REPO}
  TARBALL=${TARBALL:-download from GitHub Release}
  INSTALL_DIR=${INSTALL_DIR}
  BIN_PATH=${BIN_PATH}
  ENV_FILE=${ENV_FILE}
  DATA_DIR=${DATA_DIR}
  BACKUP_DIR=${BACKUP_DIR}
  WEB_BIND=${WEB_BIND}
  WEB_PORT=${WEB_PORT}
  WEB_PASSWORD=<required for first install if env file does not exist>
  JWT_SECRET=<optional, generated when empty>
  NO_START=${NO_START}
EOF
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "请使用 root 或 sudo 执行" >&2
    exit 1
  fi
}

detect_arch() {
  if [[ -n "${ARCH}" ]]; then
    echo "${ARCH}"
    return
  fi
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
  esac
}

version_name() {
  local arch
  arch="$(detect_arch)"
  echo "gsm-panel-${VERSION}-linux-${arch}"
}

latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -n 1 | sed -E 's/.*"tag_name": "([^"]+)".*/\1/'
}

resolve_version() {
  if [[ "${VERSION}" == "latest" ]]; then
    VERSION="$(latest_version)"
  fi
}

download_release() {
  resolve_version
  local name tmp url sums_url
  name="$(version_name)"
  tmp="$(mktemp -d)"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${name}.tar.gz"
  sums_url="https://github.com/${REPO}/releases/download/${VERSION}/SHA256SUMS"
  echo "下载 ${url}" >&2
  curl -fL "${url}" -o "${tmp}/${name}.tar.gz"
  curl -fL "${sums_url}" -o "${tmp}/SHA256SUMS"
  (cd "${tmp}" && grep "${name}.tar.gz" SHA256SUMS | sha256sum -c - >&2)
  echo "${tmp}/${name}.tar.gz"
}

prepare_tarball() {
  if [[ -n "${TARBALL}" ]]; then
    if [[ ! -f "${TARBALL}" ]]; then
      echo "找不到 TARBALL: ${TARBALL}" >&2
      exit 1
    fi
    echo "${TARBALL}"
    return
  fi
  download_release
}

write_env() {
  if [[ -f "${ENV_FILE}" ]]; then
    echo "保留已有配置: ${ENV_FILE}"
    return
  fi
  local password jwt
  password="${WEB_PASSWORD:-}"
  if [[ -z "${password}" ]]; then
    password="$(openssl rand -hex 16 2>/dev/null || tr -dc A-Za-z0-9 </dev/urandom | head -c 32)"
    echo "已生成初始 WEB_PASSWORD: ${password}"
  fi
  jwt="${JWT_SECRET:-$(openssl rand -hex 32 2>/dev/null || tr -dc A-Za-z0-9 </dev/urandom | head -c 64)}"
  cat > "${ENV_FILE}" <<EOF
WEB_PASSWORD='${password}'
JWT_SECRET='${jwt}'
WEB_BIND='${WEB_BIND}'
WEB_PORT='${WEB_PORT}'
GSM_DATA_DIR='${DATA_DIR}'
BACKUP_DIR='${BACKUP_DIR}'
EOF
  chmod 600 "${ENV_FILE}"
}

print_unit() {
  cat <<EOF
[Unit]
Description=GSM Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${DATA_DIR}
ExecStart=${BIN_PATH} web
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF
}

install_unit() {
  print_unit > "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload
  systemctl enable "${SERVICE_NAME}"
}

backup_current_binary() {
  if [[ -f "${BIN_PATH}" ]]; then
    mkdir -p "${INSTALL_DIR}/releases"
    cp "${BIN_PATH}" "${INSTALL_DIR}/releases/gsm-panel.$(date +%Y%m%d%H%M%S).bak"
  fi
}

install_from_tarball() {
  local tarball tmp extracted
  tarball="$(prepare_tarball)"
  tmp="$(mktemp -d)"
  tar -xzf "${tarball}" -C "${tmp}"
  extracted="$(find "${tmp}" -maxdepth 2 -type f -name gsm-panel | head -n 1)"
  if [[ -z "${extracted}" ]]; then
    echo "发布包缺少 gsm-panel 二进制" >&2
    exit 1
  fi
  mkdir -p "${INSTALL_DIR}" "${DATA_DIR}" "${BACKUP_DIR}" "$(dirname "${BIN_PATH}")"
  backup_current_binary
  install -m 755 "${extracted}" "${BIN_PATH}"
  if [[ -d "$(dirname "${extracted}")/data" ]]; then
    mkdir -p "${INSTALL_DIR}/data"
    cp -a "$(dirname "${extracted}")/data/." "${INSTALL_DIR}/data/"
  fi
}

start_service() {
  if [[ "${NO_START}" == "true" ]]; then
    return
  fi
  systemctl restart "${SERVICE_NAME}"
}

cmd_install() {
  require_root
  install_from_tarball
  write_env
  install_unit
  start_service
  systemctl --no-pager --full status "${SERVICE_NAME}" || true
}

cmd_upgrade() {
  require_root
  install_from_tarball
  install_unit
  start_service
  systemctl --no-pager --full status "${SERVICE_NAME}" || true
}

cmd_rollback() {
  require_root
  local latest
  latest="$(ls -1t "${INSTALL_DIR}"/releases/gsm-panel.*.bak 2>/dev/null | head -n 1 || true)"
  if [[ -z "${latest}" ]]; then
    echo "没有可回滚的二进制备份" >&2
    exit 1
  fi
  install -m 755 "${latest}" "${BIN_PATH}"
  start_service
  echo "已回滚到 ${latest}"
}

cmd_status() {
  systemctl --no-pager --full status "${SERVICE_NAME}" || true
  echo ""
  echo "Binary: ${BIN_PATH}"
  echo "Env: ${ENV_FILE}"
  echo "Data: ${DATA_DIR}"
  echo "Backups: ${BACKUP_DIR}"
}

cmd_logs() {
  journalctl -u "${SERVICE_NAME}" -f --no-pager
}

case "${1:-}" in
  install) cmd_install ;;
  upgrade) cmd_upgrade ;;
  rollback) cmd_rollback ;;
  status) cmd_status ;;
  logs) cmd_logs ;;
  print-unit) print_unit ;;
  -h|--help|help|"") usage ;;
  *) usage; exit 1 ;;
esac
