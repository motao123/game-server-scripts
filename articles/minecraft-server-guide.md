# Minecraft Java 版专用服务器搭建指南：从选购到一键部署

> 本文将带你从零开始，在 Linux 云服务器上搭建一台稳定的 Minecraft Java 版专用服务器。支持 Paper 高性能分支和官方原版，自动配置 JVM 优化参数，一个脚本搞定。

## 目录

- [为什么自建专用服务器](#为什么自建专用服务器)
- [Paper 还是 Vanilla？](#paper-还是-vanilla)
- [第一步：选购云服务器](#第一步选购云服务器)
  - [配置推荐](#配置推荐)
  - [云厂商对比](#云厂商对比)
- [第二步：连接服务器](#第二步连接服务器)
- [第三步：一键部署](#第三步一键部署)
  - [下载脚本](#下载脚本)
  - [运行安装](#运行安装)
  - [选择服务器类型](#选择服务器类型)
  - [自定义配置](#自定义配置)
- [第四步：配置安全组](#第四步配置安全组)
- [第五步：连接游戏](#第五步连接游戏)
- [服务器管理](#服务器管理)
  - [常用管理命令](#常用管理命令)
  - [控制台操作](#控制台操作)
  - [白名单管理](#白名单管理)
  - [修改服务器配置](#修改服务器配置)
  - [更新服务器版本](#更新服务器版本)
- [内容管理：插件、数据包、资源包](#内容管理插件数据包资源包)
  - [插件管理（Paper 服务器）](#插件管理paper-服务器)
  - [数据包管理](#数据包管理)
  - [资源包配置](#资源包配置)
  - [Mod 与整合包（需要换服务器软件）](#mod-与整合包需要换服务器软件)
- [性能优化与调优](#性能优化与调优)
  - [JVM 参数说明](#jvm-参数说明)
  - [内存分配建议](#内存分配建议)
  - [视距与性能](#视距与性能)
- [常见问题排查](#常见问题排查)
- [脚本做了什么](#脚本做了什么)

---

## 为什么自建专用服务器

Minecraft 多人联机有几种方式：局域网联机、Realms 官方服务、第三方租服、自建专用服务器。自建专用服务器的优势在于：

- **完全掌控**：服务器配置、插件、白名单、游戏规则全部由你决定
- **7×24 在线**：不依赖任何玩家的客户端，随时可以登录
- **性能可控**：根据需要选择硬件配置，不受第三方限制
- **插件生态**：使用 Paper 分支可以安装海量插件，扩展玩法
- **成本更低**：相比第三方租服，自建服务器性价比更高

---

## 选哪种服务器类型？

脚本支持四种服务器类型，部署时会让你选择：

| 特性 | Paper（推荐） | Vanilla（原版） | Fabric | Forge |
|------|--------------|----------------|--------|-------|
| 性能 | 优化过，TPS 更稳定 | 原版性能 | 轻量，性能好 | 较重 |
| 插件/Mod | 支持 Bukkit/Spigot 插件 | 不支持 | 支持 Fabric Mod | 支持 Forge Mod |
| Mod 数量 | 插件生态丰富 | 无 | 较多 | 最多 |
| 更新速度 | 紧跟原版 | 第一时间更新 | 较快 | 较慢 |
| 适用场景 | 多人服务器、插件 | 纯原版体验 | 客户端 Mod 联机 | 大型整合包 |

**大多数情况下推荐 Paper**。它在原版基础上做了大量性能优化，支持插件（如权限管理、领地保护、经济系统），而且完全兼容原版客户端。

如果你想和朋友一起玩 **Mod**（如工业、暮色森林等），选择 **Fabric** 或 **Forge**。Fabric 更轻量快速，Forge 的 Mod 数量更多。

---

## 第一步：选购云服务器

### 配置推荐

Minecraft 服务器主要消耗**内存**和 **CPU 单核性能**。Java 版的内存需求取决于玩家数量和视距设置。

| 玩家数量 | CPU | 内存 | 系统盘 | 月费用参考 |
|----------|-----|------|--------|-----------|
| 2-5 人（朋友联机） | 2 核 | 4 GB | 20 GB SSD | ¥40-80 |
| 5-15 人（小型服务器） | 2-4 核 | 8 GB | 30 GB SSD | ¥80-150 |
| 15-30 人（中型服务器） | 4 核 | 16 GB | 50 GB SSD | ¥150-300 |
| 30+ 人（大型服务器） | 4-8 核 | 32 GB | 80 GB SSD | ¥300+ |

**关键提醒：**

- **内存是核心**：JVM 堆内存 + 系统开销 + 文件缓存，4GB 内存大约能撑 5-8 人
- **必须用 SSD**：Minecraft 频繁读写区块文件，机械硬盘会导致明显卡顿
- **CPU 看单核**：主线程性能决定 TPS，高主频比多核更重要
- **带宽**：5-10 人 5Mbps 够用，20 人以上建议 10Mbps

### 云厂商对比

| 云厂商 | 优势 | 适合人群 | 入口 |
|--------|------|----------|------|
| **腾讯云** | 国内延迟低，轻量服务器性价比高 | 国内玩家首选 | cloud.tencent.com |
| **阿里云** | 机型丰富，稳定性好 | 追求稳定 | aliyun.com |
| **棉花云** | 游戏服务器专注，价格便宜 | 预算有限 | yun.88sup.com |

> **省钱技巧**：新用户首购优惠力度最大，建议直接买 1 年。轻量应用服务器比 ECS 便宜，对 MC 来说足够。

操作系统选择 **Ubuntu 22.04 LTS** 或 **Debian 12**。

---

## 第二步：连接服务器

购买完云服务器后，你会获得一个**公网 IP** 和 **root 密码**。

### Windows 用户

使用 [MobaXterm](https://mobaxterm.mobatek.net/)、[Tabby](https://tabby.sh/) 或 PowerShell：

```bash
ssh root@你的服务器IP
```

### macOS / Linux 用户

```bash
ssh root@你的服务器IP
```

---

## 第三步：一键部署

### 下载脚本

```bash
wget https://gitee.com/pigfei/game-server-scripts/raw/master/minecraft-server-install.sh
chmod +x minecraft-server-install.sh
```

### 运行安装

```bash
sudo ./minecraft-server-install.sh
```

### 选择服务器类型

脚本启动后，首先让你选择服务器类型：

```
  选择服务器类型:
  ┌─────────────────────────────────────────────────────────┐
  │  1) Paper (推荐) - 高性能分支，支持 Bukkit/Spigot 插件  │
  │  2) Vanilla  - 官方原版，纯净体验                       │
  │  3) Fabric   - 轻量 Mod 加载器，适合客户端 Mod 联机     │
  │  4) Forge    - 经典 Mod 加载器，Mod 数量最多             │
  └─────────────────────────────────────────────────────────┘

  请选择 [1/2/3/4, 默认1]:
```

- 输入 `1` 或直接回车：安装 Paper 高性能分支（推荐）
- 输入 `2`：安装官方原版
- 输入 `3`：安装 Fabric（轻量 Mod）
- 输入 `4`：安装 Forge（经典 Mod）

### 自定义配置

选择服务器类型后，显示当前默认配置：

```
  当前默认配置:
  ┌─────────────────────────────────────────────┐
  │  1) 服务器类型:  paper
  │  2) 游戏端口:    25565
  │  3) 最大玩家数:  20
  │  4) JVM 内存:    4G
  │  5) 游戏模式:    survival
  │  6) 难度:        normal
  │  7) 视距:        10
  │  8) MOTD:        (服务器列表显示名称)
  └─────────────────────────────────────────────┘

  直接回车使用默认值，或输入 c 自定义
```

- **直接回车**：使用全部默认配置
- **输入 `c`**：逐项自定义

自定义时可以调整：

| 配置项 | 说明 | 建议值 |
|--------|------|--------|
| 游戏端口 | 客户端连接端口 | 25565（默认） |
| 最大玩家数 | 同时在线人数上限 | 根据内存定 |
| JVM 内存 | Java 虚拟机分配的内存 | 4G 起步，8G 推荐 |
| 游戏模式 | survival/creative/adventure/spectator | survival |
| 难度 | peaceful/easy/normal/hard | normal |
| 视距 | 区块加载距离，影响性能 | 8-12 |
| 世界种子 | 世界生成种子，留空随机 | 自选或留空 |
| 正版验证 | 是否要求正版账号登录 | 朋友玩可设 false |

配置完成后，脚本显示部署步骤概览，**回车直接开始部署**：

```
即将执行部署步骤:
  [1] 安装 Java 21
  [2] 创建用户和目录
  [3] 下载 paper 服务器
  [4] 生成配置文件
  [5] 写入优化配置
  [6] 创建启动脚本
  [7] 创建 systemd 服务
  [8] 创建管理脚本
  [9] 创建自动备份
  [10] 配置防火墙
  [11] 启动服务器

回车开始部署 / 输入 n 取消:
```

整个安装过程大约需要 **5-15 分钟**，主要时间花在下载 Paper/Vanilla 服务器文件上。

安装完成后会看到：

```
============================================================
       Minecraft 服务器部署完成!
============================================================

  服务器类型:  paper
  服务器地址:  123.45.67.89:25565
  RCON 端口:   25575
  RCON 密码:   aB3dE5fG7hI9jK1l

  !!! 重要: 云服务器安全组配置 !!!
  系统防火墙已自动放行，但云服务器还需在控制台配置安全组:

  ┌──────────────┬──────────┬────────────────────────────┐
  │    端口      │   协议   │         用途               │
  ├──────────────┼──────────┼────────────────────────────┤
  │    25565     │   TCP    │  游戏主端口 (必须)         │
  │    25575     │   TCP    │  RCON 远程管理 (可选)      │
  └──────────────┴──────────┴────────────────────────────┘

  配置方式:
    腾讯云:  控制台 → 云服务器 → 安全组 → 添加入站规则
    阿里云:  控制台 → ECS → 安全组 → 配置规则 → 入方向
    棉花云:  控制台 → 云服务器 → 防火墙 → 添加规则
```

---

## 第四步：配置安全组

**这一步必须做，否则外部无法连接！**

脚本已自动配置系统防火墙，但云服务器还有一层**安全组**需要在控制台手动添加。

### 以腾讯云为例

1. 登录 [腾讯云控制台](https://console.cloud.tencent.com/)
2. 进入 **轻量应用服务器** → 你的实例
3. **防火墙** → **添加规则**
4. 添加：

| 协议 | 端口 | 策略 | 来源 |
|------|------|------|------|
| TCP | 25565 | 允许 | 0.0.0.0/0 |
| TCP | 25575 | 允许 | 0.0.0.0/0（可选，RCON） |

### 以阿里云为例

1. 登录 [阿里云控制台](https://ecs.console.aliyun.com/)
2. **ECS 实例详情** → **安全组** → **配置规则**
3. **入方向** → **手动添加**
4. 同上添加 TCP 25565

---

## 第五步：连接游戏

1. 打开 **Minecraft Java 版** 启动器
2. 确保版本与服务器一致（脚本默认安装最新版）
3. 点击 **多人游戏** → **添加服务器**
4. 服务器地址输入：`你的服务器IP:25565`
5. 点击 **完成**，然后 **加入服务器**

如果选择了 Paper 类型，原版客户端可以直接连接，不需要安装任何额外的东西。

---

## 服务器管理

脚本会自动安装 `mc-manager` 管理工具。

### 常用管理命令

```bash
mc-manager start        # 启动服务器
mc-manager stop         # 停止服务器
mc-manager restart      # 重启服务器
mc-manager status       # 查看运行状态
mc-manager logs         # 实时查看日志 (Ctrl+C 退出)
mc-manager backup       # 立即备份世界存档
mc-manager update       # 更新到最新版本
mc-manager config       # 编辑 server.properties
mc-manager memory       # 查看内存使用情况
mc-manager info         # 显示服务器信息（IP、端口、RCON密码等）
```

### 控制台操作

```bash
# 进入服务器控制台（可以看到实时日志、执行命令）
mc-manager console
# 按 Ctrl+A 然后按 D 退出控制台（服务器继续运行）

# 直接执行服务器命令（不用进入控制台）
mc-manager cmd say 服务器将在5分钟后重启
mc-manager cmd whitelist add Steve
mc-manager cmd op Steve
mc-manager cmd difficulty hard
mc-manager cmd time set day
mc-manager cmd gamemode creative Steve
```

### 白名单管理

白名单可以限制只有指定玩家才能加入服务器：

```bash
# 开启白名单
mc-manager cmd whitelist on

# 添加玩家
mc-manager cmd whitelist add Steve
mc-manager cmd whitelist add Alex

# 移除玩家
mc-manager cmd whitelist remove Steve

# 查看白名单
mc-manager cmd whitelist list
```

### 修改服务器配置

```bash
mc-manager config
```

这会用 nano 编辑器打开 `server.properties`。常用配置项：

```properties
# server.properties

motd=§a我的服务器 §7- 欢迎来玩!    # 服务器列表显示的名称
max-players=20                      # 最大玩家数
gamemode=survival                   # 默认游戏模式
difficulty=normal                   # 难度
pvp=true                            # 玩家对战
view-distance=10                    # 视距（影响性能）
simulation-distance=4               # 模拟距离（影响性能）
online-mode=true                    # 正版验证（false=离线模式）
white-list=false                    # 白名单
spawn-protection=16                 # 出生点保护范围
enable-command-block=false          # 命令方块
level-seed=                         # 世界种子
```

修改后需要重启生效：

```bash
mc-manager restart
```

### 更新服务器版本

当 Minecraft 发布新版本时：

```bash
mc-manager update
```

脚本会自动下载最新版 Paper，备份旧版本 jar 文件，然后重启服务器。

---

## 内容管理：插件、数据包、资源包

mc-manager 内置了内容管理功能，可以一键搜索、安装和管理插件、数据包、资源包。

### 插件管理（Paper 服务器）

插件是 Paper 服务器的核心优势。通过 Modrinth API，你可以直接在命令行搜索和安装插件，无需手动下载上传。

#### 搜索插件

```bash
mc-manager plugin search essentials
```

输出示例：

```
  序号 名称                      下载量         简介
  ---- ------------------------- ------------ ----------------------------------------
  1    EssentialsX               15.2M        The essential plugin suite for Minecraft...
  2    EssentialsX Spawn         2.1M         Spawn management for EssentialsX...
  3    EssentialsX Chat          1.8M         Chat formatting for EssentialsX...

  安装: mc-manager plugin install <插件名>
```

#### 安装插件

```bash
# 直接用插件名安装（自动搜索最佳匹配）
mc-manager plugin install essentialsx
mc-manager plugin install luckperms
mc-manager plugin install worldedit
mc-manager plugin install griefprevention
```

插件会自动下载到 `/opt/minecraft/plugins/` 目录。安装后需要重启服务器：

```bash
mc-manager restart
```

#### 查看和删除插件

```bash
mc-manager plugin list            # 列出已安装插件
mc-manager plugin remove worldedit # 删除插件
```

#### 常用插件推荐

| 插件 | 用途 | 安装命令 |
|------|------|---------|
| **EssentialsX** | 基础命令（传送、家、经济） | `mc-manager plugin install essentialsx` |
| **LuckPerms** | 权限管理 | `mc-manager plugin install luckperms` |
| **WorldEdit** | 世界编辑（建筑神器） | `mc-manager plugin install worldedit` |
| **Vault** | 经济/权限 API 前置 | `mc-manager plugin install vault` |
| **GriefPrevention** | 领地保护 | `mc-manager plugin install griefprevention` |
| **CoreProtect** | 方块日志（查熊孩子） | `mc-manager plugin install coreprotect` |
| **GSit** | 坐下、躺下等动作 | `mc-manager plugin install gsit` |
| **Dynmap** | 网页地图 | `mc-manager plugin install dynmap` |

更多插件可以在 [Modrinth](https://modrinth.com/plugins) 或 [SpigotMC](https://www.spigotmc.org/resources/) 上查找。

### 数据包管理

数据包是原版 Minecraft 的扩展机制，不需要装 Mod，任何服务器都支持。它通过命令系统添加新内容（自定义合成、进度、函数等）。

#### 安装数据包

```bash
# 从 URL 安装
mc-manager datapack install https://vanillatweaks.net/download/datapacks/1.21/moremobheads.zip

# 从本地文件安装
mc-manager datapack install /root/my_datapack.zip
```

数据包会安装到 `/opt/minecraft/world/datapacks/` 目录。

#### 管理数据包

```bash
mc-manager datapack list      # 列出已安装数据包
mc-manager datapack remove moremobheads  # 删除数据包
mc-manager datapack reload    # 重载数据包（不重启服务器）
```

#### 推荐数据包来源

- **[Vanilla Tweaks](https://vanillatweaks.net/picker/datapacks/)** — 最知名的数据包网站，提供原版增强类数据包
  - More Mob Heads：怪物头颅掉落
  - Multiplayer Sleep：一人睡觉即跳过黑夜
  - Coordinates HUD：显示坐标
  - Custom Armor Stands：自定义盔甲架

### 资源包配置

资源包可以改变游戏的纹理、音效和语言。服务器可以向客户端推送资源包，玩家加入时自动下载。

#### 设置资源包

```bash
# 设置资源包 URL（需要是可公开下载的链接）
mc-manager resourcepack set https://example.com/my-resource-pack.zip

# 带 SHA1 校验（可选，防止下载错误）
mc-manager resourcepack set https://example.com/pack.zip abc123def456...
```

#### 移除资源包

```bash
mc-manager resourcepack remove
```

> **注意**：资源包文件需要托管在可公开下载的服务器上。推荐使用对象存储（腾讯云 COS、阿里云 OSS）或 GitHub Releases。

### Mod 与整合包

如果你想玩 **Mod**（如工业、暮色森林、JEI 等），在部署时选择 **Fabric** 或 **Forge** 即可，脚本会自动安装。

| 类型 | 服务器软件 | 文件格式 | 安装位置 |
|------|-----------|---------|---------|
| 插件 | Paper / Spigot | `.jar` | `plugins/` |
| Mod | Forge / Fabric | `.jar` | `mods/` |

#### 安装 Mod

部署时选择了 Fabric 或 Forge 后，将 Mod 文件放入 `mods/` 目录即可：

```bash
# 将 .jar 文件放入 mods 目录
cp mod-file.jar /opt/minecraft/mods/
mc-manager restart
```

#### 整合包

整合包是一组 Mod 的合集。从 [CurseForge](https://www.curseforge.com/minecraft/modpacks) 或 [Modrinth](https://modrinth.com/modpacks) 下载**服务端包**（不是客户端包），解压到服务器目录即可。

#### 查看所有已安装内容

```bash
mc-manager packs
```

输出示例：

```
=== Minecraft 服务器内容总览 ===

[插件] /opt/minecraft/plugins/
  - EssentialsX-2.20.1.jar
  - LuckPerms-5.4.120.jar
  - WorldEdit-7.3.0.jar

[数据包] /opt/minecraft/world/datapacks/
  - moremobheads.zip
  - multiplayer_sleep.zip

[资源包]
  URL: https://example.com/my-pack.zip
```

---

## 性能优化与调优

### JVM 参数说明

脚本自动使用了 **Aikar's Flags**，这是 Minecraft 社区公认的最优 JVM 参数组合，基于 G1GC 垃圾回收器优化：

```
-XX:+UseG1GC                    # 使用 G1 垃圾回收器
-XX:+ParallelRefProcEnabled     # 并行引用处理
-XX:MaxGCPauseMillis=200        # GC 最大暂停时间 200ms
-XX:G1NewSizePercent=30         # 新生代占比 30%
-XX:G1HeapRegionSize=8M         # 堆区域大小 8MB
-XX:+AlwaysPreTouch             # 启动时预分配内存
```

这些参数能显著减少 GC 停顿，让 TPS 更稳定。一般不需要手动调整。

### 内存分配建议

| 玩家数 | JVM 内存 (-Xmx) | 系统总内存 | 说明 |
|--------|-----------------|-----------|------|
| 1-5 人 | 2-4G | 4G | 朋友联机 |
| 5-15 人 | 4-8G | 8G | 小型服务器 |
| 15-30 人 | 8-12G | 16G | 中型服务器 |
| 30+ 人 | 12-16G | 32G | 大型服务器 |

**注意**：JVM 内存不等于系统总内存。系统还需要内存给操作系统、文件缓存等。建议系统总内存 = JVM 内存 + 2G。

分配过多内存反而可能导致 GC 停顿变长，**8G 以下的服务器不建议给 JVM 分配超过系统内存的 75%**。

### 视距与性能

视距（view-distance）是影响性能最大的参数之一：

| 视距 | 区块加载量 | 性能影响 | 建议场景 |
|------|-----------|---------|---------|
| 6 | 少 | 性能最好 | 低配服务器 |
| 8 | 中 | 平衡 | 推荐默认 |
| 10 | 较多 | 较好体验 | 配置充足 |
| 12+ | 多 | 吃性能 | 高配服务器 |

如果服务器卡顿，优先降低视距：

```bash
mc-manager config
# 修改 view-distance=8 或更低
mc-manager restart
```

simulation-distance 建议设为视距的一半，可以减少实体运算负担。

---

## 常见问题排查

### 连接超时

```bash
# 1. 检查服务器是否运行
mc-manager status

# 2. 检查端口
ss -tlnp | grep 25565

# 3. 检查防火墙
sudo ufw status

# 4. 最常见：云安全组没放行 TCP 25565
```

### 服务器启动失败

```bash
# 查看日志
mc-manager logs
# 或
sudo journalctl -u mc-server -n 100
```

常见原因：
- **Java 版本不对**：Minecraft 1.20.5+ 需要 Java 21，脚本会自动安装
- **内存不足**：检查 `free -h`
- **端口被占用**：`ss -tlnp | grep 25565`

### TPS 低 / 服务器卡顿

```bash
# 进入控制台查看 TPS
mc-manager console
# 输入: tps
```

TPS 应该保持在 20.0，低于 15 会有明显卡顿。解决方法：

1. 降低视距：`view-distance=8`
2. 降低模拟距离：`simulation-distance=4`
3. 减少实体数量（清理掉落物、限制动物繁殖）
4. 升级服务器配置
5. 如果用的 Vanilla，切换到 Paper

### 玩家无法加入（离线模式）

如果设置了 `online-mode=false`（离线模式），玩家需要通过**离线账号**登录。注意离线模式下没有正版验证保护，建议配合白名单使用。

### 世界存档损坏

```bash
# 恢复备份
mc-manager stop
cd /opt/minecraft
tar -xzf backups/world_backup_XXXXXXXX_XXXXXX.tar.gz
mc-manager start
```

---

## 脚本做了什么

以下是脚本的完整技术细节：

### 1. 安装 Java 21

Minecraft 1.20.5+ 需要 Java 21。脚本会检测系统是否已有 Java，版本不够则自动安装 OpenJDK 21。

### 2. 创建用户和目录

创建 `minecraft` 系统用户，在 `/opt/minecraft/` 下建立 server、world、logs、plugins、mods、backups 目录。

### 3. 下载服务器

- **Paper**：通过 PaperMC 官方 API (`api.papermc.io`) 自动获取最新版本和构建号，下载最新 jar
- **Vanilla**：通过 Mojang 版本清单 API 获取最新版本下载地址
- **Fabric**：通过 Fabric 官方 API (`meta.fabricmc.net`) 下载服务端 jar
- **Forge**：通过 MinecraftForge 官方下载安装器，自动安装服务端

### 4. 生成配置文件

首次启动服务器以生成默认配置文件，然后写入优化后的 `server.properties`，包含网络、游戏、性能等全部配置项。同时同意 EULA。

### 5. JVM 优化启动脚本

创建 `start.sh`，使用 Aikar's Flags 启动服务器，自动设置最小/最大内存。

### 6. systemd 服务

配置自动启动、自动重启、内存限制（JVM 内存 + 1G 余量）、OOM 保护、安全加固。使用 screen 管理进程，支持通过 `ExecStop` 发送 `stop` 命令优雅关闭。

### 7. 管理脚本 mc-manager

支持服务器控制（start/stop/restart/status/logs/console/cmd/players/say/whitelist）、运维管理（backup/update/config/memory/info）、内容管理（plugin/datapack/resourcepack/packs）。

通过 screen 实现控制台交互和命令发送，RCON 密码自动生成。插件管理通过 Modrinth API 实现搜索和自动下载。

### 8. 自动备份

每 6 小时自动备份世界存档，备份前执行 `save-off` + `save-all` 确保数据一致性，最多保留 20 份。

### 9. 防火墙

自动配置 iptables/ufw，放行 TCP 25565（游戏）和 TCP 25575（RCON）。

---

## 写在最后

到这里，你的 Minecraft 专用服务器已经搭建好了。总结一下：

- **选 Paper 还是 Vanilla**：大多数人选 Paper，性能更好还支持插件
- **配置**：4GB 内存起步，必须 SSD，优先高主频 CPU
- **部署**：一个脚本搞定，5-15 分钟
- **安全组**：别忘了在云控制台放行 TCP 25565
- **管理**：用 `mc-manager` 命令管理服务器
- **优化**：脚本已配置 Aikar's Flags，视距建议 8-10

叫上朋友，开始冒险吧！

---

> **相关资源**
> - [Minecraft 官方服务器下载](https://www.minecraft.net/en-us/download/server)
> - [PaperMC 官网](https://papermc.io/)
> - [Aikar's JVM Flags 说明](https://aikar.co/2018/07/02/tuning-the-jvm-g1gc-garbage-collector-flags-for-minecraft/)
> - [server.properties 配置详解](https://minecraft.wiki/w/Server.properties)
