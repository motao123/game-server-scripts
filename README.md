# 游戏服务器一键部署脚本

纯 Shell 安装脚本，适用于 **Ubuntu 22.04+ / Debian 11+ / Debian 13**。  
无需 Web 面板、无需 Go 编译，单文件安装、systemd 管理。

| 项目 | 说明 |
|------|------|
| 许可 | 见 [LICENSE](LICENSE) |
| 架构 | `x86_64`（amd64） |
| 管理方式 | systemd + `/usr/local/bin/*-manager` |
| 重复安装 | 默认保留配置、凭证、世界；`FORCE_CONFIG_REWRITE=1` 可强制重写 |

---

## 目录

1. [支持的游戏](#支持的游戏)
2. [系统要求](#系统要求)
3. [快速开始](#快速开始)
4. [通用行为](#通用行为)
5. [Minecraft Java](#minecraft-java)
6. [幻兽帕鲁 Palworld](#幻兽帕鲁-palworld)
7. [泰拉瑞亚 Terraria](#泰拉瑞亚-terraria)
8. [英灵神殿 Valheim](#英灵神殿-valheim)
9. [备份与恢复](#备份与恢复)
10. [安全说明](#安全说明)
11. [防火墙与云安全组](#防火墙与云安全组)
12. [故障排查](#故障排查)
13. [卸载](#卸载)

---

## 支持的游戏

| 游戏 | 安装脚本 | 默认端口 | 协议 | 建议内存 | 内容生态 |
|------|----------|----------|------|----------|----------|
| Minecraft Java | `minecraft-server-install.sh` | 25565 | TCP | 4 GB+ | Paper 插件 / Fabric·Forge Mod / 数据包 / 资源包 |
| 幻兽帕鲁 | `palworld-server-install.sh` | 8211、27015 | UDP | **16 GB+**（生产） | 官方服务端 + RCON |
| 泰拉瑞亚 | `terraria-server-install.sh` | 7777 | TCP | 2 GB+ | 官方原版专用服务器 |
| 英灵神殿 | `valheim-server-install.sh` | 2456、2457 | UDP | 4 GB+ | SteamCMD 官方专用服务器 |

> 生产环境建议每台机器只跑一个重型服务（尤其 Palworld / Minecraft 高视距）。  
> 4 GB 机器仅适合功能验证，不适合 Palworld 正式开服。

---

## 系统要求

### 必备

- root 权限（`sudo`）
- systemd
- 可访问外网（下载 Java / Paper / SteamCMD / 官方服务端）
- 磁盘：建议根分区至少 **20 GB** 可用（Minecraft 更小也可，Steam 游戏建议 40 GB+）

### 脚本会自动安装的常见依赖

- 通用：`curl`、`ca-certificates`、`tar`、`gzip`、`python3`、`sudo`、`nano`、`util-linux`（`flock`）
- Minecraft：`jq`、`unzip`、`zip`、OpenJDK 21（必要时自动下载）
- Steam 系：`lib32gcc-s1`、i386 运行库、SteamCMD（官方归档优先）

---

## 快速开始

```bash
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts
chmod +x *.sh

# 交互安装（推荐新手）
sudo ./minecraft-server-install.sh
# 或
sudo ./palworld-server-install.sh
sudo ./terraria-server-install.sh
sudo ./valheim-server-install.sh
```

### 非交互安装示例

```bash
# Minecraft Paper
sudo NONINTERACTIVE=1 \
  SERVER_TYPE=paper \
  MC_VERSION=1.21.11 \
  MC_MEMORY=2G \
  MC_MEMORY_MIN=1G \
  MC_ENABLE_RCON=false \
  ./minecraft-server-install.sh

# Terraria 固定官方版本
sudo NONINTERACTIVE=1 \
  TS_VERSION=1455 \
  TS_MEMORY_MAX=2G \
  ./terraria-server-install.sh

# Valheim
sudo NONINTERACTIVE=1 \
  VH_MEMORY_MAX=2G \
  VH_SERVER_PASSWORD='至少5位' \
  ./valheim-server-install.sh

# Palworld（密码至少 12 位）
sudo NONINTERACTIVE=1 \
  ADMIN_PASSWORD='至少12位强密码' \
  ./palworld-server-install.sh
```

安装完成后，用对应 `*-manager` 管理服务。

---

## 通用行为

### 1. 安装模式

| 变量 | 默认 | 说明 |
|------|------|------|
| `NONINTERACTIVE=1` | 关闭 | 跳过交互问答，使用环境变量/默认值 |
| `FORCE_CONFIG_REWRITE=1` | `0` | 强制重写凭证与配置文件；**世界/存档仍保留** |

重复执行安装脚本时（默认）：

- 保留 `/etc/<game>/credentials.env`
- 保留游戏配置与世界
- 可升级服务文件、启动脚本、manager
- 已在运行的服务会 `restart` 以加载新脚本

### 2. 管理命令约定

- 帮助中列出的命令均可执行
- 未知命令 / 参数缺失：**非零退出**
- `logs` 为持续输出，测试或脚本中请用 `timeout 5 xxx-manager logs`
- 备份、恢复、更新等运维命令通常需要 **root**

### 3. 路径约定

| 用途 | 路径模式 |
|------|----------|
| 游戏目录 | `/opt/<game>` 或 Palworld 的 `/home/steam/PalServer` |
| 凭证 | `/etc/<game>/credentials.env` |
| 管理器 | `/usr/local/bin/<game>-manager` |
| systemd 服务 | `<name>.service` |
| 定时备份 | `<name>-backup.timer`（默认每 6 小时） |

### 4. 备份约定

- 写入临时文件 `*.partial`，校验后再原子改名
- 生成 `*.sha256` 校验文件
- 使用 `flock` 防止手动备份与 timer 并发
- `restore latest` 或 `restore /绝对路径`
- 恢复前自动做 pre-restore 备份，失败可回滚
- 只恢复**原先处于运行状态**的服务

---

## Minecraft Java

### 安装

```bash
sudo ./minecraft-server-install.sh
```

支持类型：

| `SERVER_TYPE` | 说明 |
|---------------|------|
| `paper`（默认） | 高性能 + 插件 |
| `vanilla` | 官方原版 |
| `fabric` | 轻量 Mod 加载器 |
| `forge` | 经典 Mod 加载器 |

### 关键路径

| 路径 | 说明 |
|------|------|
| `/opt/minecraft` | 服务端根目录 |
| `/opt/minecraft/world` | 默认世界（可用 `MC_LEVEL_NAME` 改） |
| `/opt/minecraft/plugins` | Paper 插件 |
| `/opt/minecraft/mods` | Fabric / Forge Mod |
| `/opt/minecraft/backups` | 世界备份 |
| `/opt/minecraft/server.properties` | 服务器配置 |
| `/opt/minecraft/.install-state` | 安装状态（类型/版本/世界） |
| `/etc/minecraft/credentials.env` | RCON 等凭证 |
| `/usr/local/bin/mc-manager` | 管理脚本 |
| `mc-server.service` | systemd 服务 |

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `SERVER_TYPE` | `paper` | 服务端类型 |
| `MC_VERSION` | 最新/探测 | 精确 MC 版本，如 `1.21.11` |
| `MC_MEMORY` | 自动/4G | JVM `-Xmx` |
| `MC_MEMORY_MIN` | 自动/1G | JVM `-Xms` |
| `MC_PORT` | `25565` | 游戏端口 |
| `MC_MAX_PLAYERS` | `20` | 最大玩家 |
| `MC_LEVEL_NAME` | `world` | 世界目录名 |
| `MC_GAMEMODE` | `survival` | 模式 |
| `MC_DIFFICULTY` | `normal` | 难度 |
| `MC_ONLINE_MODE` | `true` | 正版验证 |
| `MC_ENABLE_RCON` | `false` | 是否启用 RCON |
| `MC_RCON_PORT` | `25575` | RCON 端口 |
| `MC_RCON_PASSWORD` | 自动生成 | RCON 密码 |
| `NONINTERACTIVE` | `0` | 非交互 |
| `FORCE_CONFIG_REWRITE` | `0` | 强制重写配置 |

> 显式传入的 `MC_MEMORY` / `MC_MEMORY_MIN` / `MC_ENABLE_RCON` 不会被资源探测覆盖。

### 管理命令

```bash
mc-manager help

# 生命周期
mc-manager start|stop|restart|status
timeout 5 mc-manager logs

# 信息
mc-manager info
mc-manager memory
mc-manager config          # 编辑 server.properties

# 备份 / 恢复 / 更新
mc-manager backup
mc-manager restore latest
mc-manager restore /opt/minecraft/backups/world_backup_xxx.tar.gz
mc-manager update          # 保持当前 MC 主版本，仅更新同版本构建/加载器

# RCON（需 MC_ENABLE_RCON=true）
mc-manager cmd list
mc-manager players
mc-manager say 你好
mc-manager whitelist on|off|list|add <玩家>|remove <玩家>

# 内容生态
# Paper only
mc-manager plugin search <关键词>
mc-manager plugin install <名称>
mc-manager plugin list
mc-manager plugin remove <jar文件名>

# Fabric / Forge only
mc-manager mod search <关键词>
mc-manager mod install <项目slug>
mc-manager mod list
mc-manager mod remove <jar文件名>

# 全类型
mc-manager datapack install <路径或URL>
mc-manager datapack list|remove <名>|reload
mc-manager resourcepack set <URL> <sha1> [true|false]
mc-manager resourcepack remove
mc-manager packs
```

### 非交互示例

```bash
# Paper + RCON
sudo NONINTERACTIVE=1 SERVER_TYPE=paper MC_VERSION=1.21.11 \
  MC_MEMORY=4G MC_ENABLE_RCON=true ./minecraft-server-install.sh

# Fabric
sudo NONINTERACTIVE=1 SERVER_TYPE=fabric MC_VERSION=1.21.11 \
  MC_MEMORY=2G ./minecraft-server-install.sh

# Forge
sudo NONINTERACTIVE=1 SERVER_TYPE=forge MC_VERSION=1.21.1 \
  MC_MEMORY=2G ./minecraft-server-install.sh

# Vanilla
sudo NONINTERACTIVE=1 SERVER_TYPE=vanilla MC_VERSION=1.21.11 \
  MC_MEMORY=2G ./minecraft-server-install.sh
```

### 更新策略

| 类型 | `mc-manager update` 行为 |
|------|--------------------------|
| Paper | 同 MC 版本的最新稳定构建 + SHA256 校验 |
| Vanilla | 同 MC 版本官方 jar + SHA1 校验 |
| Fabric | 同 MC 版本的 loader/installer 更新 |
| Forge | 同 MC 版本线的 Forge 更新（走安装器） |

更新会：停服（若在运行）→ 临时文件下载校验 → 备份旧 jar → 原子替换 → 恢复原运行状态。

---

## 幻兽帕鲁 Palworld

### 安装

```bash
sudo ./palworld-server-install.sh
```

### 关键路径

| 路径 | 说明 |
|------|------|
| `/home/steam/PalServer` | 服务端目录 |
| `.../Pal/Saved` | 存档与配置 |
| `.../Config/LinuxServer/PalWorldSettings.ini` | 主配置 |
| `/etc/palworld/credentials.env` | 管理员/RCON 凭证 |
| `/home/steam/pal-backups` | 备份目录 |
| `/usr/local/bin/pal-manager` | 管理脚本 |
| `pal-server.service` | 主服务 |
| `pal-server-backup.timer` | 每 6 小时备份 |
| `pal-server-restart.timer` | 每日凌晨优雅重启 |

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `SERVER_NAME` | `Palworld Server` | 服务器名 |
| `SERVER_PASSWORD` | 空 | 进服密码 |
| `ADMIN_PASSWORD` | 自动生成 | 管理员/RCON 密码（≥12 位） |
| `MAX_PLAYERS` | `32` | 最大玩家 |
| `DEFAULT_PORT` | `8211` | 游戏端口 UDP |
| `QUERY_PORT` | `27015` | 查询端口 UDP |
| `RCON_PORT` | `25575` | RCON TCP |
| `REST_API_PORT` | `8212` | REST API TCP（仅本机） |
| `SWAP_SIZE` | `16G` | 目标 Swap（会按磁盘空间裁剪） |
| `STEAMCMD_URL` | Valve 官方 | SteamCMD 包 URL/本地路径 |
| `STEAMCMD_PROXY` | 空 | 代理，如 `socks5://127.0.0.1:7890` |
| `PALSERVER_ARCHIVE_URL` | 空 | 离线包路径/URL |
| `PALSERVER_ARCHIVE_SHA256` | 空 | 离线包校验（推荐） |
| `NONINTERACTIVE` | `0` | 非交互 |
| `FORCE_CONFIG_REWRITE` | `0` | 强制重写配置 |

### 管理命令

```bash
pal-manager help

pal-manager start|stop|restart|status
timeout 5 pal-manager logs
pal-manager logs-all

pal-manager info
pal-manager memory
pal-manager config

pal-manager backup
pal-manager restore /home/steam/pal-backups/pal_backup_xxx.tar.gz
pal-manager update

# RCON
pal-manager rcon Info
pal-manager rcon ShowPlayers
pal-manager players
pal-manager broadcast 服务器将在5分钟后重启
pal-manager save
pal-manager kick <SteamID>
pal-manager ban <SteamID>
pal-manager unban <SteamID>
```

### 离线包安装（推荐国内/CDN 受限环境）

部分网络环境下 SteamCMD 匿名拉取 AppID `2394010` 会报  
`Failed to install app '2394010' (Missing configuration)`。此时用离线包：

```bash
# 离线包内应包含 PalServer/ 目录，且存在 PalServer/PalServer.sh
sudo NONINTERACTIVE=1 \
  ADMIN_PASSWORD='至少12位强密码' \
  PALSERVER_ARCHIVE_URL=/root/PalServer.tar.gz \
  PALSERVER_ARCHIVE_SHA256='实际sha256' \
  ./palworld-server-install.sh
```

### 代理安装

```bash
sudo NONINTERACTIVE=1 \
  ADMIN_PASSWORD='至少12位强密码' \
  STEAMCMD_PROXY=socks5://127.0.0.1:7890 \
  ./palworld-server-install.sh
```

### 内存与 Swap

- 安装时会按物理内存计算 `MemoryMax` / `MemoryHigh`
- 会创建/调整 Swap（不会盲目占满磁盘）
- 生产建议 **≥16 GB 内存**，4 GB 机器仅能做脚本功能验证

---

## 泰拉瑞亚 Terraria

### 安装

```bash
sudo ./terraria-server-install.sh
```

官方版本发现接口当前不稳定，默认使用可配置稳定版本 **`1455`**。  
也可用 `TS_VERSION` 固定版本。

### 关键路径

| 路径 | 说明 |
|------|------|
| `/opt/terraria` | 根目录 |
| `/opt/terraria/server` | 服务端二进制 |
| `/opt/terraria/world` | 世界文件 |
| `/opt/terraria/serverconfig.txt` | 配置 |
| `/opt/terraria/backups` | 备份 |
| `/etc/terraria/credentials.env` | 安装配置/凭证 |
| `/usr/local/bin/terraria-manager` | 管理脚本 |
| `terraria-server.service` | systemd 服务 |

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `TS_PORT` | `7777` | 游戏端口 |
| `TS_MAX_PLAYERS` | `8` | 最大玩家 |
| `TS_SERVER_NAME` | `Terraria Server` | 名称 |
| `TS_SERVER_PASSWORD` | 空 | 进服密码 |
| `TS_WORLD_NAME` | `world` | 世界名 |
| `TS_DIFFICULTY` | `1` | 0 普通 / 1 专家 / 2 大师 / 3 旅途 |
| `TS_SEED` | 空 | 种子 |
| `TS_MEMORY_MAX` | `4G` | cgroup 内存上限 |
| `TS_VERSION` | 空 | 固定版本号 |
| `TS_STABLE_FALLBACK` | `1455` | 探测失败时版本 |
| `NONINTERACTIVE` | `0` | 非交互 |
| `FORCE_CONFIG_REWRITE` | `0` | 强制重写配置 |

### 管理命令

```bash
terraria-manager help

terraria-manager start|stop|restart|status
timeout 5 terraria-manager logs

terraria-manager info
terraria-manager memory
terraria-manager world
terraria-manager config

terraria-manager backup
terraria-manager restore latest
terraria-manager update
```

说明：

- 无假 `console` 命令（已移除）
- `update` 会暂存下载、原子替换，并保留世界/配置
- 本仓库当前仅支持官方原版，不捆绑 tModLoader / TShock

---

## 英灵神殿 Valheim

### 安装

```bash
sudo ./valheim-server-install.sh
```

### 关键路径

| 路径 | 说明 |
|------|------|
| `/opt/valheim` | 根目录 |
| `/opt/valheim/server` | Steam 服务端 |
| `/opt/valheim/world` | 世界存档与名单 |
| `/opt/valheim/start.sh` | 启动脚本 |
| `/opt/valheim/backups` | 备份 |
| `/etc/valheim/credentials.env` | 配置/密码 |
| `/usr/local/bin/valheim-manager` | 管理脚本 |
| `valheim-server.service` | systemd 服务 |

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `VH_USER` | `valheim` | 运行用户（会持久化） |
| `VH_SERVER_NAME` | `Valheim Server` | 名称 |
| `VH_SERVER_PORT` | `2456` | 主端口（查询端口 = +1） |
| `VH_WORLD_NAME` | `Dedicated` | 世界名 |
| `VH_SERVER_PASSWORD` | 自动生成 | 密码，≥5 位 |
| `VH_PUBLIC` | `1` | 是否公开 |
| `VH_CROSSPLAY` | `false` | 跨平台 |
| `VH_MEMORY_MAX` | `14G` | cgroup 内存上限 |
| `NONINTERACTIVE` | `0` | 非交互 |
| `FORCE_CONFIG_REWRITE` | `0` | 强制重写凭证 |

### 管理命令

```bash
valheim-manager help

valheim-manager start|stop|restart|status
timeout 5 valheim-manager logs

valheim-manager info
valheim-manager memory
valheim-manager world
valheim-manager config

valheim-manager backup
valheim-manager restore latest
valheim-manager update
```

备份包含：

- 完整世界目录
- `adminlist.txt` / `bannedlist.txt` / `permittedlist.txt`（若存在）
- 凭证副本

---

## 备份与恢复

### 手动

```bash
# Minecraft
mc-manager backup
mc-manager restore latest

# Palworld
pal-manager backup
pal-manager restore /home/steam/pal-backups/pal_backup_YYYYMMDD_HHMMSS.tar.gz

# Terraria / Valheim
terraria-manager backup && terraria-manager restore latest
valheim-manager backup && valheim-manager restore latest
```

### 自动

默认启用 6 小时一次的 systemd timer：

```bash
systemctl list-timers | grep -E 'mc-server|pal-server|terraria|valheim'
systemctl status mc-server-backup.timer
```

### 校验

```bash
sha256sum -c /path/to/backup.tar.gz.sha256
tar -tzf /path/to/backup.tar.gz | head
```

### 安全限制

- 拒绝绝对路径穿越与 `..`
- 仅允许恢复脚本约定目录内的备份（Minecraft）
- 恢复失败会回滚到 pre-restore 备份
- 并发备份会被 `flock` 拒绝

---

## 安全说明

1. **不要默认暴露 RCON**
   - Minecraft：默认 `MC_ENABLE_RCON=false`
   - Palworld：安装后对 `25575/tcp`、`8212/tcp` 添加主机防火墙 **deny**
2. **凭证权限**
   - `/etc/<game>/credentials.env` 为 `600` 或 `640`
   - manager 中不打印明文密码到 `info`（Minecraft）
3. **游戏用户无 sudo**
4. **进程隔离**
   - 独立系统用户（`minecraft` / `steam` / `terraria` / `valheim`）
   - systemd：`ProtectSystem=strict`、`NoNewPrivileges`、内存上限
5. **云安全组**
   - 只放行游戏端口
   - 不要放行 RCON / REST / SSH 以外的管理端口到公网

---

## 防火墙与云安全组

### 需要放行

| 游戏 | 端口 | 协议 |
|------|------|------|
| Minecraft | 25565 | TCP |
| Palworld | 8211、27015 | UDP |
| Terraria | 7777 | TCP |
| Valheim | 2456、2457（及 +2 视版本） | UDP |

### 不要放行到公网

| 用途 | 端口 | 协议 |
|------|------|------|
| Minecraft RCON | 25575 | TCP |
| Palworld RCON | 25575 | TCP |
| Palworld REST | 8212 | TCP |

脚本会尽量添加本地防火墙规则，但 **云厂商安全组需你在控制台自行配置**。

---

## 故障排查

### 通用

```bash
# 服务状态
systemctl status <服务名> --no-pager
journalctl -u <服务名> -n 200 --no-pager

# 监听端口
ss -lntup | grep -E '25565|8211|7777|2456'

# 管理命令帮助
mc-manager help
pal-manager help
terraria-manager help
valheim-manager help
```

### Minecraft

| 现象 | 处理 |
|------|------|
| Paper 下载 410 | 已切 `fill.papermc.io/v3`；请用最新脚本 |
| Fabric 下载失败 | 固定 `MC_VERSION=1.21.11` 或检查 Fabric meta 是否可达 |
| Forge 安装失败 | 查看 `/tmp/forge-install.log`；确认 Java 21 |
| RCON 连不上 | 确认 `MC_ENABLE_RCON=true` 并 `FORCE_CONFIG_REWRITE=1` 重装/改配置后重启 |
| 插件装到 Vanilla | 预期拒绝；换 `SERVER_TYPE=paper` |

```bash
# 强制打开 RCON 并重写配置
sudo NONINTERACTIVE=1 MC_ENABLE_RCON=true FORCE_CONFIG_REWRITE=1 \
  SERVER_TYPE=paper ./minecraft-server-install.sh
```

### Palworld

| 现象 | 处理 |
|------|------|
| `Missing configuration` (2394010) | 当前网络/账号无法匿名拉取；改用代理或离线包 |
| 内存被杀 | 提高机器内存；检查 `MemoryMax` |
| RCON 失败 | 检查 `credentials.env` 与服务是否已完全启动 |

```bash
# 代理
sudo env STEAMCMD_PROXY=socks5://127.0.0.1:7890 NONINTERACTIVE=1 \
  ADMIN_PASSWORD='强密码12位以上' ./palworld-server-install.sh

# 离线包
sudo env PALSERVER_ARCHIVE_URL=/root/PalServer.tar.gz \
  PALSERVER_ARCHIVE_SHA256='...' \
  NONINTERACTIVE=1 ADMIN_PASSWORD='强密码12位以上' \
  ./palworld-server-install.sh
```

### Terraria

| 现象 | 处理 |
|------|------|
| 版本探测异常 | `TS_VERSION=1455` 固定版本 |
| start-limit-hit | 脚本已 `reset-failed`；也可手动：`systemctl reset-failed terraria-server` |
| 首次进服慢 | 世界生成中，看 `journalctl -u terraria-server -f` |

### Valheim

| 现象 | 处理 |
|------|------|
| SteamCMD Permission denied | 更新脚本；确认 `/opt/steamcmd/steamcmd.sh` 可执行 |
| 包装器递归 | 使用最新脚本（wrapper 指向 `/opt/steamcmd/steamcmd.sh`） |
| 更新后无法启动 | `systemctl reset-failed valheim-server && valheim-manager start` |

### SteamCMD 通用

```bash
# 手动测试
sudo -u valheim steamcmd +force_install_dir /opt/valheim/server \
  +login anonymous +app_update 896660 validate +quit

# 清理异常 package 状态后重试
sudo rm -rf /home/*/.steam /home/*/Steam/package
```

---

## 卸载

脚本目前不提供一键卸载，可手动：

```bash
# 以 Minecraft 为例
sudo systemctl disable --now mc-server mc-server-backup.timer
sudo rm -f /etc/systemd/system/mc-server.service \
           /etc/systemd/system/mc-server-backup.service \
           /etc/systemd/system/mc-server-backup.timer
sudo systemctl daemon-reload
sudo rm -f /usr/local/bin/mc-manager
sudo rm -rf /opt/minecraft /etc/minecraft
sudo userdel -r minecraft 2>/dev/null || true
```

Palworld / Terraria / Valheim 同理，替换服务名与目录即可。

**注意：先备份再删目录。**

---

## 推荐生产检查清单

1. 用非交互安装并记录 `credentials.env`
2. `systemctl is-enabled --now <service>`
3. `ss -lntup` 确认只监听预期端口
4. 做一次手动 `backup` + `restore latest`
5. 做一次 `update`，确认服务能恢复
6. 云安全组仅放行游戏端口
7. 定期确认 timer 与磁盘空间

```bash
df -h /
systemctl list-timers | grep -E 'mc-server|pal-server|terraria|valheim'
```

---

## 许可证

见 [LICENSE](LICENSE)。
