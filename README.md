# 游戏服务器一键部署脚本

Linux 游戏专用服务器一键部署脚本集合，支持 Ubuntu 22.04+ / Debian 11+。

## 支持的游戏

| 游戏 | 脚本 | 端口 | 协议 | 最低内存 |
|------|------|------|------|----------|
| 幻兽帕鲁 (Palworld) | `palworld-server-install.sh` | 8211 / 27015 / 25575 / 8212(本地) | UDP / UDP / TCP / TCP | 16 GB |
| Minecraft Java | `minecraft-server-install.sh` | 25565 / 25575 | TCP / TCP | 4 GB |
| 英灵神殿 (Valheim) | `valheim-server-install.sh` | 2456 / 2457 | UDP / UDP | 4 GB |
| 泰拉瑞亚 (Terraria) | `terraria-server-install.sh` | 7777 | TCP | 2 GB |

## 快速开始

### 1. 准备一台 Linux 云服务器

推荐配置：

| 游戏 | CPU | 内存 | 系统盘 | 适合人数 |
|------|-----|------|--------|----------|
| 幻兽帕鲁 | 4 核 3.5GHz+ | 16-32 GB | 50 GB SSD | 4-16 人 |
| Minecraft | 2 核+ | 4-8 GB | 20 GB SSD | 4-10 人 |
| 英灵神殿 | 2 核+ | 4-8 GB | 20 GB SSD | 4-10 人 |
| 泰拉瑞亚 | 2 核+ | 2-4 GB | 10 GB SSD | 4-8 人 |

推荐云厂商：腾讯云 / 阿里云 / 棉花云 (yun.88sup.com)

操作系统选择 **Ubuntu 22.04 LTS** 或 **Debian 12**。

### 2. 连接服务器

```bash
ssh root@你的服务器IP
```

### 3. 下载并运行脚本

以幻兽帕鲁为例：

```bash
# 从 CNB 克隆脚本仓库
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts

# 添加执行权限
chmod +x palworld-server-install.sh

# 运行
sudo ./palworld-server-install.sh
```

也可以使用其他镜像仓库：

```bash
# Gitee
git clone https://gitee.com/pigfei/game-server-scripts.git

# GitHub
git clone git@github.com:motao123/game-server-scripts.git
```

### 4. 国内服务器 SteamCMD 下载失败

如果国内服务器无法连接 Steam CDN，可以用三种方式处理：仓库内置 SteamCMD 安装包镜像、SteamCMD 代理、用户自备 PalServer 离线包。

#### 4.1 维护者同步 SteamCMD 安装包到 CNB / Gitee / GitHub

在能访问 Steam CDN 的机器上执行：

```bash
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts
mkdir -p mirrors
curl -fL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" -o mirrors/steamcmd_linux.tar.gz

git add mirrors/steamcmd_linux.tar.gz
git commit -m "chore: 同步 SteamCMD 安装包"
git push origin master   # 推送到 CNB
git push gitee master    # 可选：同步到 Gitee
git push github master   # 可选：同步到 GitHub
```

普通用户克隆仓库后，直接使用仓库里的安装包：

```bash
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts
sudo env STEAMCMD_URL="$PWD/mirrors/steamcmd_linux.tar.gz" ./palworld-server-install.sh
```

这只解决 SteamCMD 本体安装包下载失败；Palworld 服务端仍需要 SteamCMD 连接 Steam 下载。

#### 4.2 SteamCMD 通过代理下载 Palworld 服务端

```bash
sudo env STEAMCMD_PROXY="socks5://127.0.0.1:7890" ./palworld-server-install.sh
```

也可以同时使用仓库内置 SteamCMD 安装包和代理：

```bash
sudo env \
  STEAMCMD_URL="$PWD/mirrors/steamcmd_linux.tar.gz" \
  STEAMCMD_PROXY="socks5://127.0.0.1:7890" \
  ./palworld-server-install.sh
```

后续更新也可以走代理：

```bash
sudo env STEAMCMD_PROXY="socks5://127.0.0.1:7890" pal-manager update
```

#### 4.3 使用自备 PalServer 离线包

```bash
sudo env PALSERVER_ARCHIVE_URL="https://your-private-url/PalServer.tar.gz" ./palworld-server-install.sh

# 可选：校验离线包 SHA256
sudo env \
  PALSERVER_ARCHIVE_URL="https://your-private-url/PalServer.tar.gz" \
  PALSERVER_ARCHIVE_SHA256="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \
  ./palworld-server-install.sh
```

离线包推荐目录结构：

```text
PalServer/
├── PalServer.sh
├── DefaultPalWorldSettings.ini
├── Engine/
└── Pal/
```

可在已下载好的服务器上这样打包：

```bash
tar -czf PalServer.tar.gz -C /home/steam/Steam/steamapps/common PalServer
```

后续更新也可以走代理：

```bash
sudo env STEAMCMD_PROXY="socks5://127.0.0.1:7890" pal-manager update
```

### 5. 配置安全组

脚本会自动配置系统防火墙，但**云服务器还需要在控制台配置安全组**：

| 游戏 | 需要放行的端口 |
|------|---------------|
| 幻兽帕鲁 | UDP 8211、UDP 27015、TCP 25575（可选）；REST API 8212 仅本地不开 |
| Minecraft | TCP 25565、TCP 25575（可选） |
| 英灵神殿 | UDP 2456、UDP 2457 |
| 泰拉瑞亚 | TCP 7777 |

配置路径（以各厂商为例）：

- **腾讯云**：控制台 → 云服务器 → 安全组 → 添加入站规则
- **阿里云**：控制台 → ECS → 安全组 → 配置规则 → 入方向
- **棉花云**：控制台 → 云服务器 → 防火墙 → 添加规则

### 6. 连接游戏

在游戏客户端中输入 `你的服务器IP:端口` 即可连接。

---

## 脚本功能

每个脚本部署完成后，会自动安装一个管理工具，方便日常运维。

### 幻兽帕鲁 — pal-manager

```bash
pal-manager start       # 启动服务器
pal-manager stop        # 停止服务器
pal-manager restart     # 重启服务器
pal-manager status      # 查看状态
pal-manager logs        # 实时日志
pal-manager logs-all    # 最近 500 行日志
pal-manager update      # 更新服务器版本
pal-manager backup      # 立即备份存档
pal-manager config      # 编辑配置文件
pal-manager players     # 查看在线玩家
pal-manager broadcast X # 广播消息
pal-manager kick <id>   # 踢人（需 SteamID）
pal-manager ban <id>    # 封禁（需 SteamID）
pal-manager unban <id>  # 解封（需 SteamID）
pal-manager save        # 立即保存存档
pal-manager rcon <cmd>  # 发送任意 RCON 命令
pal-manager memory      # 查看内存使用
pal-manager info        # 显示服务器信息
```

### Minecraft — mc-manager

```bash
# 服务器控制
mc-manager start        # 启动服务器
mc-manager stop         # 停止服务器
mc-manager restart      # 重启服务器
mc-manager status       # 查看状态
mc-manager logs         # 实时日志
mc-manager console      # 进入控制台（可执行命令）
mc-manager cmd <命令>   # 直接执行游戏命令
mc-manager players      # 查看在线玩家
mc-manager say <消息>   # 广播消息

# 运维管理
mc-manager backup       # 立即备份
mc-manager update       # 更新服务器
mc-manager config       # 编辑配置
mc-manager info         # 服务器信息

# 内容管理
mc-manager plugin search <关键词>   # 搜索 Modrinth 插件
mc-manager plugin install <名称>    # 安装插件
mc-manager plugin list              # 列出已安装插件
mc-manager plugin remove <名称>     # 删除插件
mc-manager datapack install <URL>   # 安装数据包
mc-manager datapack list            # 列出数据包
mc-manager datapack remove <名称>   # 删除数据包
mc-manager datapack reload          # 重载数据包
mc-manager resourcepack set <URL>   # 设置资源包
mc-manager resourcepack remove      # 移除资源包
mc-manager packs                    # 查看所有已安装内容
```

### 英灵神殿 — valheim-manager

```bash
valheim-manager start    # 启动服务器
valheim-manager stop     # 停止服务器
valheim-manager restart  # 重启服务器
valheim-manager status   # 查看状态
valheim-manager logs     # 实时日志
valheim-manager backup   # 立即备份
valheim-manager update   # 更新服务器
valheim-manager config   # 编辑配置
valheim-manager info     # 服务器信息
```

### 泰拉瑞亚 — terraria-manager

```bash
terraria-manager start    # 启动服务器
terraria-manager stop     # 停止服务器
terraria-manager restart  # 重启服务器
terraria-manager status   # 查看状态
terraria-manager logs     # 实时日志
terraria-manager console  # 进入控制台
terraria-manager backup   # 立即备份
terraria-manager config   # 编辑配置
terraria-manager info     # 服务器信息
```

---

## Web 管理面板（幻兽帕鲁）

可选组件，为幻兽帕鲁服务器提供可视化网页端，免去命令行操作。基于 Python3 标准库实现，**无额外依赖**，通过 RCON 和 systemctl 控制服务器。

### 前置条件

必须先完成幻兽帕鲁服务器部署（`palworld-server-install.sh`），因为 Web 面板依赖：

- `/usr/local/bin/pal-rcon` -- RCON 客户端（主脚本安装）
- `PalWorldSettings.ini` -- 从中读取 RCON 端口和管理员密码
- `python3` -- 主脚本已安装

未装主脚本直接跑 Web 安装会报错退出。

### 安装

```bash
cd game-server-scripts
chmod +x palworld-web-install.sh
sudo ./palworld-web-install.sh
```

安装过程交互配置三项：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| Web 端口 | `8080` | 浏览器访问端口 |
| 绑定地址 | `0.0.0.0` | `0.0.0.0`=公网可访问，`127.0.0.1`=仅本地（需 SSH 隧道） |
| Web 密码 | 留空自动生成 | 留空时自动生成 18 位随机密码；建议自行设置强密码 |

**RCON 端口和管理员密码无需输入**，脚本自动从 `PalWorldSettings.ini` 读取。

也可以用环境变量预设，跳过交互：

```bash
sudo env WEB_PORT=8080 WEB_BIND=0.0.0.0 WEB_PASSWORD='你的强密码' \
    ./palworld-web-install.sh
```

### 安装产物

| 路径 | 用途 | 权限 |
|------|------|------|
| `/usr/local/bin/pal-web-ui` | Web 应用（Python 脚本） | 755 root:root |
| `/etc/pal-web.env` | 配置（含 Web 密码 + RCON 密码） | 600 root:root |
| `/etc/systemd/system/pal-web.service` | systemd 服务 | 644 root:root |

### 功能详解

登录后 dashboard 包含以下面板（30 秒自动刷新状态/玩家/日志）：

**仪表盘卡片**
- 服务状态（运行中/已停止）
- 启动时间（systemd ActiveEnterTimestamp）
- 当前内存（MemoryCurrent，GB）
- 峰值内存（MemoryPeak，GB）

**服务控制**
- `启动` / `停止` / `重启` -- 调 `systemctl start/stop/restart pal-server`，走 ExecStop 自动 RCON Save 落盘
- `保存存档` -- 调 RCON `Save`，立即落盘（不用重启）

**广播消息**
- 输入文本点发送，调 RCON `Broadcast <msg>`，游戏内全服可见
- 支持中文

**在线玩家**
- 表格列出：玩家名 / SteamID / 操作按钮
- `踢出` -- RCON `KickPlayer <SteamID>`
- `封禁` -- RCON `BanPlayer <SteamID>`（封禁后该 SteamID 无法再次连接）
- 解封需通过命令行：`pal-manager unban <SteamID>`

**最近日志**
- 显示 `journalctl -u pal-server -n 200` 的最后 200 行
- 30 秒自动刷新，自动滚到底部

### 访问方式

**方式一：公网直接访问**（`WEB_BIND=0.0.0.0` 时）

```
http://你的服务器IP:8080
```

需在云控制台安全组放行 TCP 8080。**强烈建议配合反代 HTTPS**（见下方安全部分）。

**方式二：SSH 隧道（仅本地，最安全）**（`WEB_BIND=127.0.0.1` 时）

```bash
# 本地机器执行，把远程 8080 映射到本地 8080
ssh -L 8080:127.0.0.1:8080 root@你的服务器IP
```

然后浏览器访问 `http://127.0.0.1:8080`。Web 面板不暴露公网，只能通过 SSH 隧道访问。

**改绑定地址需重装**：

```bash
sudo env WEB_BIND=127.0.0.1 ./palworld-web-install.sh
```

### 安全

Web 面板能控制服务器（启停/踢人/广播），公网暴露务必注意：

#### 1. 强密码

Web 密码**不要与游戏 AdminPassword 相同**（AdminPassword 可能泄露给其他管理员）。安装时自动生成的 18 位随机密码最安全，或自行设置 16 位以上含大小写+数字+符号的密码。

修改密码需重装：

```bash
sudo env WEB_PASSWORD='新密码' ./palworld-web-install.sh
```

#### 2. 反代 HTTPS（强烈建议）

Web 面板默认 HTTP，密码明文传输。公网使用必须反代 HTTPS。

**Nginx 反代示例**（需有域名 + 证书）：

```nginx
server {
    listen 443 ssl http2;
    server_name pal.yourdomain.com;

    ssl_certificate     /etc/letsencrypt/live/pal.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pal.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

反代后把 `WEB_BIND` 改成 `127.0.0.1` 重装，仅本地监听，公网只能通过 HTTPS 访问。

**Caddy 反代示例**（自动 HTTPS）：

```
pal.yourdomain.com {
    reverse_proxy 127.0.0.1:8080
}
```

#### 3. 内置防护

Web 面板自身已有以下防护（无需额外配置）：

| 机制 | 说明 |
|------|------|
| Session cookie | `HttpOnly` + `SameSite=Lax` + IP 绑定 + 2 小时过期 |
| CSRF token | 所有 POST 请求必须带 `X-CSRF-Token` 头，2 小时过期 |
| 登录限速 | 每 IP 每分钟最多 5 次失败，防暴力破解 |
| 配置文件 | `/etc/pal-web.env` 权限 600，仅 root 可读 |

#### 4. 监控登录

定期检查异常登录：

```bash
journalctl -u pal-web | grep -E 'login|POST /api/login'
```

#### 5. 不用时关公网

如果只是临时用，不用时改回仅本地：

```bash
sudo env WEB_BIND=127.0.0.1 ./palworld-web-install.sh
```

或者直接停服务：

```bash
sudo systemctl stop pal-web
```

### 故障排查

**登录后立即跳回登录页**：session cookie 被浏览器拒绝，通常是 HTTPS/HTTP 混用导致。用反代 HTTPS 全程访问，或用 SSH 隧道。

**操作按钮没反应/报 403**：CSRF token 过期，刷新页面重新登录即可。

**显示「RCON 超时」或玩家列表为空**：
- 确认服务器已启动：`systemctl status pal-server`
- 确认 RCON 已启用：`grep RCONEnabled PalWorldSettings.ini` 应为 `True`
- 测试 RCON：`pal-manager players`，若失败检查 RCON 端口和密码

**Web 面板打不开**：
- 检查服务状态：`systemctl status pal-web`
- 查看日志：`journalctl -u pal-web -n 50`
- 检查端口监听：`ss -tlnp | grep 8080`
- 云安全组是否放行 TCP 8080（仅 `WEB_BIND=0.0.0.0` 时需要）

**改了 PalWorldSettings.ini 的 RCON 端口/密码后 Web 失效**：重跑 `palworld-web-install.sh`，会重新读取配置写入 `/etc/pal-web.env`。

### 卸载 Web 面板

```bash
sudo systemctl stop pal-web
sudo systemctl disable pal-web
sudo rm -f /etc/systemd/system/pal-web.service \
           /usr/local/bin/pal-web-ui \
           /etc/pal-web.env
sudo systemctl daemon-reload
```

卸载不影响游戏服务器和存档。

---

## 自动任务

脚本会自动配置以下定时任务：

| 任务 | 适用游戏 | 频率 | 说明 |
|------|----------|------|------|
| 自动备份 | 全部 | 每 6 小时 (00:00/06:00/12:00/18:00) | 帕鲁备份前先 RCON `Save` 落盘；保留最近 30 份 |
| 自动重启 | 仅幻兽帕鲁 | 每日凌晨 4:00 | 广播预警 60s → RCON `Save` → systemctl restart（走 ExecStop 再次 Save，双保险） |

备份文件命名格式：`游戏名_backup_YYYYMMDD_HHMMSS.tar.gz`，存放路径见上方目录结构表。

查看/管理定时任务：

```bash
systemctl list-timers --all | grep -E 'pal-server|mc-server|valheim-server|terraria-server'
systemctl status pal-server-backup.timer     # 查看备份定时器状态
sudo systemctl stop pal-server-restart.timer # 临时停用帕鲁每日重启
```

## 目录结构

部署后的默认安装路径：

| 游戏 | 安装目录 | 存档目录 | 备份目录 |
|------|----------|----------|----------|
| 幻兽帕鲁 | `/home/steam/Steam/steamapps/common/PalServer` | `Pal/Saved/SaveGames` | `/home/steam/pal-backups` |
| Minecraft | `/opt/minecraft` | `/opt/minecraft/world` | `/opt/minecraft/backups` |
| 英灵神殿 | `/opt/valheim/server` | `/opt/valheim/world` | `/opt/valheim/backups` |
| 泰拉瑞亚 | `/opt/terraria/server` | `/opt/terraria/world` | `/opt/terraria/backups` |

幻兽帕鲁走 SteamCMD 标准路径（`/home/steam/Steam/steamapps/common/`），服务以 `steam` 用户运行；其他游戏安装在 `/opt/` 下，各有独立的 server/world/backups 子目录。

管理脚本统一位于 `/usr/local/bin/`（如 `pal-manager`、`mc-manager`），systemd 服务文件位于 `/etc/systemd/system/`（如 `pal-server.service`）。

## 常见问题

### 重启后存档丢失（每次进服都是新档）

**新版本脚本（2026-07-11 起）已修复此问题**：所有关闭路径（`systemctl stop/restart`、`pal-manager stop/restart`、机器重启触发的 systemd stop）都会先 RCON `Save` 落盘再 SIGINT 退出，`TimeoutStopSec=300` 给大存档 5 分钟落盘窗口。手动重启无需先 `pal-manager save`。

若仍遇到丢档，按以下顺序排查：

**1. 检查存档目录属主**（最常见原因：旧脚本部署的服务器目录属主是 root）

```bash
# 检查存档目录属主（应为 steam:steam）
ls -ld /home/steam/Steam/steamapps/common/PalServer/Pal/Saved/SaveGames

# 若属主是 root，手动修正：
sudo chown -R steam:steam /home/steam/Steam/steamapps/common/PalServer/Pal/Saved
```

**2. 确认 ExecStop 脚本存在**（新版本才有，旧版本用的是 `/bin/kill -SIGINT`）

```bash
ls -l /usr/local/bin/pal-stop
# 应为 -rwxr-xr-x root root

# 查看 systemd 服务是否用了 pal-stop
systemctl cat pal-server | grep -E 'ExecStop|TimeoutStopSec'
# 应为: ExecStop=/usr/local/bin/pal-stop $MAINPID
#       TimeoutStopSec=300
```

若不存在，说明用的是旧脚本，重新跑 `sudo ./palworld-server-install.sh` 升级（不会清存档，只会补 chown 和 pal-stop）。

**3. 非 root 用户手动开服**

若以非 root 用户手动运行 `./PalServer.sh`（非 systemd），需将该用户加入 steam 组，否则同样无写权限：

```bash
sudo usermod -aG steam <你的用户>   # 重新登录生效
```

推荐使用 `pal-manager` 或 `systemctl` 管理服务，避免权限问题。

### 连接超时

1. 检查服务器进程是否运行：`pal-manager status`（替换对应管理命令）
2. 检查端口是否监听：`ss -ulnp | grep 端口号`
3. 检查系统防火墙：`sudo ufw status` 或 `sudo iptables -L -n`
4. **最常见原因**：云安全组没有放行对应端口，请在云控制台检查

### 服务器启动失败

```bash
# 查看错误日志
pal-manager logs
# 或
sudo journalctl -u 服务名 -n 50
```

常见原因：内存不足、SteamCMD 下载不完整、端口被占用。

### 如何修改服务器配置

```bash
pal-manager config      # 编辑配置文件
pal-manager restart     # 修改后重启生效
```

### 如何更新服务器版本

```bash
pal-manager update      # 自动下载最新版本并重启
```

### 如何恢复备份

以幻兽帕鲁为例（备份包内是 `SaveGames/` 目录，解压到 `Pal/Saved/` 下覆盖现有存档）：

```bash
pal-manager stop
sudo -u steam tar -xzf /home/steam/pal-backups/pal_backup_XXXXXXXX_XXXXXX.tar.gz \
    -C /home/steam/Steam/steamapps/common/PalServer/Pal/Saved
pal-manager start
```

其他游戏类似，替换备份路径和存档目录即可：

| 游戏 | 备份路径 | 解压目标 |
|------|----------|----------|
| Minecraft | `/opt/minecraft/backups/mc_backup_*.tar.gz` | `/opt/minecraft` |
| 英灵神殿 | `/opt/valheim/backups/valheim_backup_*.tar.gz` | `/opt/valheim` |
| 泰拉瑞亚 | `/opt/terraria/backups/terraria_backup_*.tar.gz` | `/opt/terraria` |

恢复前建议先 `pal-manager stop`（或对应管理命令）停止服务器，避免文件占用。恢复后用对应管理命令 `start` 启动。

## systemd 服务

每个脚本都会创建 systemd 服务，支持标准 systemctl 命令：

```bash
sudo systemctl start 游戏名-server
sudo systemctl stop 游戏名-server
sudo systemctl restart 游戏名-server
sudo systemctl status 游戏名-server
sudo journalctl -u 游戏名-server -f   # 实时日志
```

服务名称：
- 幻兽帕鲁：`pal-server`
- Minecraft：`mc-server`
- 英灵神殿：`valheim-server`
- 泰拉瑞亚：`terraria-server`

## 卸载

以幻兽帕鲁为例（其他游戏将 `pal` / `palworld` 替换为对应游戏名，路径按上方目录结构表调整）：

```bash
# 1. 停止主服务
sudo systemctl stop pal-server
sudo systemctl disable pal-server

# 2. 停止并禁用定时器（帕鲁有 restart + backup 两个定时器）
sudo systemctl stop pal-server-restart.timer pal-server-backup.timer 2>/dev/null
sudo systemctl disable pal-server-restart.timer pal-server-backup.timer 2>/dev/null

# 3. 删除 systemd 服务/定时器文件
sudo rm /etc/systemd/system/pal-server*
sudo systemctl daemon-reload

# 4. 删除管理脚本和辅助脚本
sudo rm -f /usr/local/bin/pal-manager \
           /usr/local/bin/pal-rcon \
           /usr/local/bin/pal-stop \
           /usr/local/bin/pal-graceful-restart \
           /usr/local/bin/pal-backup

# 5. 删除安装目录和备份目录
sudo rm -rf /home/steam/Steam/steamapps/common/PalServer
sudo rm -rf /home/steam/pal-backups

# 6. 删除 steam 用户（可选，若其他游戏也用 steam 用户则不要删）
sudo userdel -r steam
```

若安装了 Web 管理面板，还需卸载：

```bash
sudo systemctl stop pal-web
sudo systemctl disable pal-web
sudo rm -f /etc/systemd/system/pal-web.service /usr/local/bin/pal-web-ui /etc/pal-web.env
sudo systemctl daemon-reload
```

## License

MIT
