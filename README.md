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

可选组件，为幻兽帕鲁服务器提供可视化网页端，免去命令行操作。

### 安装

先完成幻兽帕鲁服务器部署（`palworld-server-install.sh`），再额外跑 Web 面板安装脚本：

```bash
cd game-server-scripts
chmod +x palworld-web-install.sh
sudo ./palworld-web-install.sh
```

安装过程交互配置 Web 端口、绑定地址、Web 密码（留空自动生成 18 位随机密码）。从 `PalWorldSettings.ini` 自动读取 RCON 端口和管理员密码，无需重复输入。

### 功能

- **仪表盘**：服务状态 / 启动时间 / 当前内存 / 峰值内存
- **服务控制**：启动 / 停止 / 重启 / 保存存档（一键）
- **广播消息**：网页输入，游戏内全服广播
- **在线玩家**：查看玩家名 + SteamID，一键踢出 / 封禁 / 解封
- **日志查看**：最近 200 行，30 秒自动刷新

### 访问

安装完成会打印访问地址和密码。默认端口 8080。

```bash
# 查看服务状态
systemctl status pal-web

# 查看登录日志
journalctl -u pal-web | grep login

# 重启面板
sudo systemctl restart pal-web
```

### 安全

Web 面板能控制服务器（启停/踢人/广播），公网暴露务必注意：

1. **强密码**：Web 密码不要与游戏 AdminPassword 相同，安装时自动生成的 18 位随机密码最安全
2. **反代 HTTPS**：强烈建议用 Nginx/Caddy 反代 + HTTPS，避免密码明文传输
3. **限速保护**：登录接口每 IP 每分钟最多 5 次，防暴力破解
4. **Session 绑 IP**：cookie 窃取后换 IP 无法使用
5. **定期查日志**：`journalctl -u pal-web` 检查异常登录
6. **不用时关公网**：改 `WEB_BIND=127.0.0.1` 重装，仅本地访问（需 SSH 隧道）

### 卸载 Web 面板

```bash
sudo systemctl stop pal-web
sudo systemctl disable pal-web
sudo rm /etc/systemd/system/pal-web.service
sudo rm /usr/local/bin/pal-web-ui /etc/pal-web.env
sudo systemctl daemon-reload
```

---

## 自动任务

脚本会自动配置以下定时任务：

| 任务 | 频率 | 说明 |
|------|------|------|
| 自动备份 | 每 6 小时 | 备份前先 RCON Save 落盘，最多保留 30 份 |
| 自动重启 | 每日凌晨 4:00 | 仅帕鲁服务器，广播预警 + RCON Save + 优雅重启 |

备份文件命名格式：`游戏名_backup_日期_时间.tar.gz`

## 目录结构

部署后的默认安装路径：

```
/opt/palworld/          # 幻兽帕鲁
/opt/minecraft/         # Minecraft
/opt/valheim/           # 英灵神殿
/opt/terraria/          # 泰拉瑞亚

每个游戏目录下：
├── server/             # 服务器文件
├── world/              # 存档目录
├── backups/            # 备份目录
├── start.sh            # 启动脚本
└── 配置文件             # 各游戏配置
```

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

```bash
pal-manager stop
cd /opt/palworld
tar -xzf backups/pal_backup_XXXXXXXX_XXXXXX.tar.gz
pal-manager start
```

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

```bash
# 停止服务
sudo systemctl stop 游戏名-server
sudo systemctl disable 游戏名-server

# 删除服务文件
sudo rm /etc/systemd/system/游戏名-server*
sudo systemctl daemon-reload

# 删除安装目录
sudo rm -rf /opt/游戏名

# 删除管理脚本
sudo rm /usr/local/bin/游戏名-manager

# 删除用户
sudo userdel -r 用户名
```

## License

MIT
