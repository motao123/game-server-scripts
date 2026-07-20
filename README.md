# 游戏服务器一键部署脚本

纯 Shell 安装脚本，适用于 Ubuntu 22.04+ / Debian 11+。无需 Web 面板。

## 支持的游戏

| 游戏 | 脚本 | 端口 | 协议 | 建议内存 |
|------|------|------|------|----------|
| 幻兽帕鲁 (Palworld) | `palworld-server-install.sh` | 8211 / 27015 | UDP | 16 GB+ |
| Minecraft Java | `minecraft-server-install.sh` | 25565 | TCP | 4 GB+ |
| 英灵神殿 (Valheim) | `valheim-server-install.sh` | 2456 / 2457 | UDP | 4 GB+ |
| 泰拉瑞亚 (Terraria) | `terraria-server-install.sh` | 7777 | TCP | 2 GB+ |

## 快速开始

```bash
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts

chmod +x *.sh
sudo ./palworld-server-install.sh      # 或 minecraft / valheim / terraria
```

## 安全说明

- **RCON 默认不对公网放行**，安装脚本只添加游戏端口的防火墙规则；请通过本机或 SSH 隧道使用 RCON。
- 管理员/服务器密码默认生成随机强密码，安装结束时打印一次；凭证写入 `/etc/<game>/credentials.env`，仅 root 或对应游戏组可读。
- 游戏进程用户**不会**被授予 sudo。
- 云服务器仍需在控制台安全组放行游戏端口。

## 常用管理命令

| 游戏 | 管理命令 |
|------|----------|
| Palworld | `pal-manager start\|stop\|restart\|status\|logs\|backup\|update\|players\|info` |
| Minecraft | `mc-manager start\|stop\|restart\|status\|logs\|console\|backup\|update\|info` |
| Valheim | `valheim-manager start\|stop\|restart\|status\|logs\|backup\|update\|info` |
| Terraria | `terraria-manager start\|stop\|restart\|status\|logs\|console\|backup\|info` |

## 环境变量（可选）

### Palworld

| 变量 | 说明 |
|------|------|
| `STEAMCMD_URL` | SteamCMD 安装包 URL 或本地路径 |
| `STEAMCMD_PROXY` | SteamCMD 代理，如 `socks5://127.0.0.1:7890` |
| `PALSERVER_ARCHIVE_URL` | 自备服务端离线包 |
| `PALSERVER_ARCHIVE_SHA256` | 离线包 SHA256（推荐） |
| `ADMIN_PASSWORD` | 预置管理员密码（否则自动生成） |
| `SERVER_PASSWORD` | 进服密码（可空） |
| `NONINTERACTIVE=1` | 跳过交互，使用默认/环境变量 |

### 通用

| 变量 | 说明 |
|------|------|
| `NONINTERACTIVE=1` | 非交互安装 |
| 各脚本内端口/名称变量 | 可在运行前 `export` 覆盖默认值 |

## 许可证

见 [LICENSE](LICENSE)。
