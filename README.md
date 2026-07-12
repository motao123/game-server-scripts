# 游戏服务器管理面板

Go + React 实现的游戏服务器管理面板。后端单个 Go 二进制，前端 React + Vite + Ant Design，构建产物通过 `embed` 内嵌。

## 功能模块

| 模块 | 功能 |
|------|------|
| 仪表盘 | CPU/内存/磁盘/系统运行时长，内存磁盘详情，监听端口列表，进程列表 Top30，实例概览，5 秒自动刷新，阈值变色 |
| 终端 | xterm.js 真实 PTY 终端，ANSI 色彩/光标控制，多标签会话，会话列表，自适应尺寸，WebSocket 双向通信 |
| 实例管理 | 实例 CRUD，启动/停止/重启（按 stopCommand 注入 ctrl+c/stop/exit/quit），编辑弹窗，状态标签，Minecraft Java 自动检测 jar/启动脚本，Palworld 专项管理（玩家/存档/配置/白名单/封禁） |
| 游戏部署 | 外置游戏清单、SteamCMD 部署、Minecraft Java 下载部署、在线模板部署，部署完成自动创建实例 |
| 文件管理 | 受限根目录浏览，面包屑导航，搜索过滤，Monaco 代码编辑器（语法高亮），上传/下载/删除/重命名/压缩解压，确认弹窗，编码检测 |
| 备份管理 | 通用 tar.gz 备份，分组管理，保留策略，恢复/删除，Palworld 存档专项备份上传下载 |
| 计划任务 | 4 种类型可视化表单（power/command/backup/system），实例选择，cron 调度，创建/编辑/删除确认 |
| 环境管理 | Java/SteamCMD/常用工具安装，点击直接执行 apt-get，实时进度条和输出日志，成功/失败反馈 |
| RCON | 实例级 RCON 控制台，配置保存（per-instance），连接/断开管理，命令历史，彩色输出 |
| 插件 | 扫描 data/plugins/*/plugin.json，本地插件市场，一键安装，远程包安装，升级，卸载备份，兼容性校验，配置读写 |
| 设置 | 运行配置展示，密码修改，文件管理根目录列表，安全提示 |
| 清单管理 | 游戏清单、插件市场、在线模板的远程更新、JSON 校验、备份恢复、热重载 |
| 关于 | 项目信息 |

**UX 特性**：深色模式切换（localStorage 持久化）、侧边栏折叠、URL 路由（浏览器前进后退）、确认弹窗、5 秒仪表盘自动刷新、阈值变色、搜索过滤。

## 支持的游戏

| 游戏 | 脚本 | 端口 | 协议 | 最低内存 |
|------|------|------|------|----------|
| 幻兽帕鲁 (Palworld) | `palworld-server-install.sh` | 8211 / 27015 / 25575 / 8212(本地) | UDP / UDP / TCP / TCP | 16 GB |
| Minecraft Java | `minecraft-server-install.sh` | 25565 / 25575 | TCP / TCP | 4 GB |
| 英灵神殿 (Valheim) | `valheim-server-install.sh` | 2456 / 2457 | UDP / UDP | 4 GB |
| 泰拉瑞亚 (Terraria) | `terraria-server-install.sh` | 7777 | TCP | 2 GB |

## 快速开始

### 1. 准备服务器

推荐 Ubuntu 22.04 LTS 或 Debian 12，配置视游戏而定。

### 2. 克隆仓库

```bash
git clone https://cnb.cool/code_free/game-server-scripts.git
cd game-server-scripts
```

### 3. 安装游戏服务器

```bash
chmod +x palworld-server-install.sh
sudo ./palworld-server-install.sh
```

### 4. 安装 Web 面板

```bash
chmod +x palworld-web-install.sh
sudo ./palworld-web-install.sh
```

安装脚本会自动构建 Go + React 面板（需要 go 和 node），或使用预构建二进制。安装完成后访问 `http://服务器IP:8080`。

## 技术栈

- **后端**：Go 1.23+，标准库 `net/http` + `gorilla/websocket` + `creack/pty` + `robfig/cron`
- **前端**：React 19 + TypeScript + Vite + Ant Design 5 + xterm.js + Monaco Editor + react-router-dom
- **部署**：单个 Linux 二进制 `gsm-panel`，前端通过 `embed` 内嵌
- **认证**：密码 + Session Cookie + CSRF Token + 登录限速

## 从源码构建

```bash
# 前端
cd web && npm install && npm run build   # 产物输出到 internal/app/frontend

# 后端
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o gsm-panel ./cmd/gsm-panel
```

## Docker 部署

```bash
export WEB_PASSWORD='请改成强密码'
export JWT_SECRET='请改成长随机字符串'
docker compose up -d --build
```

默认访问 `http://服务器IP:8080`。容器会挂载：

- `./docker_data`：面板数据、实例、任务、告警规则
- `./game_file`：游戏服务端文件
- `./backups`：备份文件

多架构镜像构建：

```bash
chmod +x scripts/docker-buildx.sh
IMAGE=your-registry/gsm-panel:latest PUSH=true ./scripts/docker-buildx.sh
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WEB_PASSWORD` | 空 | 面板登录密码 |
| `WEB_BIND` | `0.0.0.0` | 绑定地址 |
| `WEB_PORT` | `8080` | 监听端口 |
| `SERVICE` | `pal-server` | 默认 Palworld systemd 服务名 |
| `RCON_PORT` | `25575` | RCON 端口 |
| `RCON_PASS` | 空 | RCON 密码 |
| `REST_API_PORT` | `8212` | Palworld REST API 端口 |
| `GSM_DATA_DIR` | `./data` | 实例/任务/插件数据目录 |

## API 端点

### 认证
- `POST /api/auth/login` - 登录，返回 CSRF Token
- `POST /api/auth/logout` - 退出
- `GET /api/auth/verify` - 验证 Session
- `GET /api/csrf` - 获取 CSRF Token

### 系统
- `GET /api/system/info` - CPU/内存/磁盘/运行时长
- `GET /api/system/ports` - 监听端口列表
- `GET /api/system/processes` - 进程列表
- `GET /api/health` - 健康检查（无需认证）

### 实例
- `GET /api/instances` - 实例列表
- `POST /api/instances/create` - 创建实例
- `POST /api/instances/update` - 更新实例
- `POST /api/instances/delete` - 删除实例
- `POST /api/instances/start` - 启动
- `POST /api/instances/stop` - 停止
- `POST /api/instances/restart` - 重启

### 游戏部署
- `GET /api/games` - 游戏模板列表
- `GET /api/catalogs` - 清单状态列表
- `POST /api/catalogs/reload` - 热重载清单
- `POST /api/catalogs/update` - 从远程 URL 更新清单并自动备份
- `GET /api/online-templates` - 在线模板列表
- `GET /api/steamcmd/status` - SteamCMD 状态
- `POST /api/game-deployment/install` - 启动部署任务
- `POST /api/online-templates/deploy` - 启动在线模板部署任务
- `GET /api/game-deployment/status?taskId=` - 查询部署进度

### 文件管理
- `GET /api/files/list?path=` - 目录列表
- `GET /api/files/read?path=` - 读取文件（含编码检测）
- `POST /api/files/write` - 写入文件
- `GET /api/files/download?path=` - 下载文件
- `POST /api/files/upload?path=` - 上传文件
- `POST /api/files/delete` - 删除
- `POST /api/files/mkdir` - 新建目录
- `POST /api/files/rename` - 重命名
- `POST /api/files/compress` - 压缩为 tar.gz
- `POST /api/files/extract` - 解压 tar.gz

### 备份
- `GET /api/backup/groups` - 备份组列表
- `POST /api/backup/create-generic` - 创建备份
- `POST /api/backup/restore-generic` - 恢复备份
- `POST /api/backup/delete-file` - 删除备份文件

### 计划任务
- `GET /api/scheduled-tasks` - 任务列表
- `POST /api/scheduled-tasks/create` - 创建任务
- `POST /api/scheduled-tasks/delete` - 删除任务

### 环境管理
- `GET /api/environment/info` - 检测 Java/SteamCMD
- `POST /api/environment/install` - 安装环境包
- `GET /api/environment/install/status?taskId=` - 安装进度

### RCON
- `GET /api/rcon/config?instanceId=` - 获取配置
- `POST /api/rcon/config/save` - 保存配置
- `POST /api/rcon/connect` - 连接
- `POST /api/rcon/disconnect` - 断开
- `GET /api/rcon/status?instanceId=` - 连接状态
- `POST /api/rcon/command-instance` - 执行命令

### 插件
- `GET /api/plugins` - 插件列表
- `GET /api/plugins/catalog` - 插件市场列表
- `POST /api/plugins/create` - 创建插件
- `POST /api/plugins/install` - 从市场安装插件
- `POST /api/plugins/upgrade` - 升级已安装插件并自动备份旧版本
- `GET /api/plugins/config?id=` - 读取插件配置
- `POST /api/plugins/config` - 保存插件配置
- `POST /api/plugins/delete` - 删除插件
- `POST /api/plugins/toggle` - 启用/禁用

### 终端
- `GET /api/terminal/sessions` - 会话列表
- `WS /ws` - WebSocket 终端（terminal-start/input/resize/close/reconnect）

### Palworld 兼容 API
- `GET /api/status` - 服务状态
- `GET /api/players` - 在线玩家
- `GET /api/logs` - 服务器日志
- `GET /api/memory` - 内存信息
- `GET /api/saves` - 存档列表
- `POST /api/saves/backup` - 立即备份
- `POST /api/saves/upload` - 上传存档
- `POST /api/saves/delete` - 删除存档
- `GET /api/saves/download?name=` - 下载存档
- `GET /api/config` - Palworld 配置
- `POST /api/config` - 保存配置
- `POST /api/kick` / `POST /api/ban` / `POST /api/unban`
- `POST /api/start` / `POST /api/stop` / `POST /api/restart`
- `POST /api/save` / `POST /api/broadcast`
- `GET /api/whitelist` / `POST /api/whitelist/add` / `POST /api/whitelist/remove` / `POST /api/whitelist/check`
- `GET /api/banlist` / `POST /api/banlist/unban`

## 项目结构

```
game-server-scripts/
├── cmd/gsm-panel/main.go          # Go CLI 入口
├── internal/
│   ├── app/                       # HTTP 路由、handler、管理器
│   │   ├── server.go              # 服务入口和路由
│   │   ├── modules.go             # 文件/终端/插件/RCON handler
│   │   ├── gsm_modules.go         # 游戏部署/备份/任务 handler
│   │   ├── deploy_manager.go      # SteamCMD 部署任务
│   │   ├── install_manager.go     # 环境安装任务
│   │   ├── backup_generic.go      # 通用备份
│   │   ├── rcon_manager.go        # RCON 连接管理
│   │   ├── scheduler.go           # cron 调度器
│   │   ├── terminal.go            # PTY 终端管理
│   │   └── encoding.go            # 文件编码检测
│   ├── auth/                      # 认证和 Session
│   ├── config/                    # 环境配置
│   ├── palworld/                  # Palworld 专项逻辑
│   ├── rcon/                      # RCON 协议客户端
│   ├── system/                    # 系统信息采集
│   └── terminal/                  # PTY 会话管理
├── web/                           # React 前端源码
│   ├── src/
│   │   ├── pages/                 # 12 个页面
│   │   ├── App.tsx                # 路由和布局
│   │   └── store.ts               # Zustand 状态
│   └── package.json
├── palworld-server-install.sh     # Palworld 服务器安装脚本
├── palworld-web-install.sh        # Web 面板安装脚本
├── minecraft-server-install.sh    # Minecraft 安装脚本
├── valheim-server-install.sh      # Valheim 安装脚本
└── terraria-server-install.sh     # Terraria 安装脚本
```

## 安全建议

- 公网暴露务必使用 Nginx/Caddy 反代 HTTPS
- Web 密码不要与游戏 AdminPassword 相同
- 文件管理默认限制在游戏目录、备份目录、数据目录
- 终端仅管理员可用，操作会记录到 journalctl
- 定期检查登录日志：`journalctl -u gsm-panel | grep login`

## 卸载

```bash
sudo systemctl stop gsm-panel
sudo systemctl disable gsm-panel
sudo rm -f /etc/systemd/system/gsm-panel.service /usr/local/bin/gsm-panel /etc/gsm-panel.env
sudo systemctl daemon-reload
```
