# 游戏服务器一键部署脚本

纯 Shell 安装脚本，适用于 Ubuntu 22.04+ / Debian 11+ / Debian 13。无需 Web 面板。

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

# 交互安装
sudo ./minecraft-server-install.sh

# 非交互安装示例
sudo NONINTERACTIVE=1 SERVER_TYPE=paper MC_MEMORY=2G MC_ENABLE_RCON=false ./minecraft-server-install.sh
sudo NONINTERACTIVE=1 TS_VERSION=1455 TS_MEMORY_MAX=2G ./terraria-server-install.sh
sudo NONINTERACTIVE=1 VH_MEMORY_MAX=2G VH_SERVER_PASSWORD=testpass ./valheim-server-install.sh
sudo NONINTERACTIVE=1 ADMIN_PASSWORD='至少12位强密码' ./palworld-server-install.sh
```

## 管理命令（全部可用）

| 游戏 | 命令 |
|------|------|
| Minecraft | `mc-manager start\|stop\|restart\|status\|logs\|backup\|restore\|update\|config\|info\|memory\|cmd\|players\|say\|whitelist\|plugin\|mod\|datapack\|resourcepack\|packs` |
| Palworld | `pal-manager start\|stop\|restart\|status\|logs\|logs-all\|backup\|restore\|update\|config\|rcon\|players\|broadcast\|kick\|ban\|unban\|save\|memory\|info` |
| Valheim | `valheim-manager start\|stop\|restart\|status\|logs\|backup\|restore\|update\|config\|world\|info\|memory` |
| Terraria | `terraria-manager start\|stop\|restart\|status\|logs\|backup\|restore\|update\|config\|world\|info\|memory` |

说明：

- 帮助中列出的命令都是可执行命令；未知命令与缺参会返回非零。
- Minecraft `plugin` 仅 Paper；`mod` 仅 Fabric/Forge；Vanilla 会明确拒绝。
- 备份支持 `restore latest|<path>`，带 SHA256 校验与回滚。
- 重复安装默认保留配置/凭证/世界，除非设置 `FORCE_CONFIG_REWRITE=1`。

## 安全说明

- RCON 默认关闭（Minecraft）或仅本机可用；Palworld 安装后会显式拒绝公网 25575/8212。
- 凭证写入 `/etc/<game>/credentials.env`，仅 root 或对应游戏组可读。
- 游戏进程用户不会被授予 sudo。
- 云服务器仍需在安全组放行游戏端口。

## 环境变量（常用）

### 通用

| 变量 | 说明 |
|------|------|
| `NONINTERACTIVE=1` | 非交互安装 |
| `FORCE_CONFIG_REWRITE=1` | 强制重写配置/凭证 |

### Minecraft

| 变量 | 说明 |
|------|------|
| `SERVER_TYPE` | `paper` / `vanilla` / `fabric` / `forge` |
| `MC_VERSION` | 精确游戏版本 |
| `MC_MEMORY` / `MC_MEMORY_MIN` | JVM 内存 |
| `MC_ENABLE_RCON` | `true` 开启 RCON（默认 `false`） |

### Palworld

| 变量 | 说明 |
|------|------|
| `ADMIN_PASSWORD` | 管理员密码（至少 12 位） |
| `STEAMCMD_PROXY` | SteamCMD 代理 |
| `PALSERVER_ARCHIVE_URL` | 自备离线包（当前机房匿名拉取 AppID 2394010 失败时使用） |
| `PALSERVER_ARCHIVE_SHA256` | 离线包校验 |

### Terraria / Valheim

| 变量 | 说明 |
|------|------|
| `TS_VERSION` | Terraria 官方版本，默认稳定 `1455` |
| `VH_SERVER_PASSWORD` | Valheim 密码（至少 5 位） |

## 许可证

见 [LICENSE](LICENSE)。
