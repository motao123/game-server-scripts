# 游戏服务器一键部署脚本

Linux 游戏专用服务器一键部署脚本集合，支持 Ubuntu 22.04+ / Debian 11+。

## 支持的游戏

| 游戏 | 脚本 | 端口 | 协议 | 最低内存 |
|------|------|------|------|----------|
| 幻兽帕鲁 (Palworld) | `palworld-server-install.sh` | 8211 / 27015 / 25575 | UDP / UDP / TCP | 16 GB |
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
# 下载脚本
wget https://gitee.com/pigfei/game-server-scripts/raw/master/palworld-server-install.sh

# 添加执行权限
chmod +x palworld-server-install.sh

# 运行
sudo ./palworld-server-install.sh
```

### 4. 配置安全组

脚本会自动配置系统防火墙，但**云服务器还需要在控制台配置安全组**：

| 游戏 | 需要放行的端口 |
|------|---------------|
| 幻兽帕鲁 | UDP 8211、UDP 27015、TCP 25575（可选） |
| Minecraft | TCP 25565、TCP 25575（可选） |
| 英灵神殿 | UDP 2456、UDP 2457 |
| 泰拉瑞亚 | TCP 7777 |

配置路径（以各厂商为例）：

- **腾讯云**：控制台 → 云服务器 → 安全组 → 添加入站规则
- **阿里云**：控制台 → ECS → 安全组 → 配置规则 → 入方向
- **棉花云**：控制台 → 云服务器 → 防火墙 → 添加规则

### 5. 连接游戏

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
pal-manager memory      # 查看内存使用
pal-manager info        # 显示服务器信息
```

### Minecraft — mc-manager

```bash
mc-manager start        # 启动服务器
mc-manager stop         # 停止服务器
mc-manager restart      # 重启服务器
mc-manager status       # 查看状态
mc-manager logs         # 实时日志
mc-manager console      # 进入控制台（可执行命令）
mc-manager cmd <命令>   # 直接执行游戏命令
mc-manager players      # 查看在线玩家
mc-manager say <消息>   # 广播消息
mc-manager backup       # 立即备份
mc-manager update       # 更新服务器
mc-manager config       # 编辑配置
mc-manager info         # 服务器信息
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

## 自动任务

脚本会自动配置以下定时任务：

| 任务 | 频率 | 说明 |
|------|------|------|
| 自动备份 | 每 6 小时 | 备份存档到 `/opt/游戏名/backups/`，最多保留 20 份 |
| 自动重启 | 每日凌晨 4:00 | 仅帕鲁服务器，防止内存泄漏 |

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
