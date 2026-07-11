#!/usr/bin/env python3
"""Palworld Web UI - 可视化管理面板

仅依赖 Python3 stdlib。通过 subprocess 调用 pal-rcon / systemctl / journalctl。
安全：独立 web 密码 + session cookie + CSRF + 登录限速。公网暴露强烈建议反代 HTTPS。
配置从环境变量读取（由 systemd EnvironmentFile=/etc/pal-web.env 注入）。
"""
import http.server
import socketserver
import subprocess
import json
import time
import os
import re
import shutil
import secrets
from http.cookies import SimpleCookie
# pwd/grp 仅 Linux 可用，延迟到 write_settings 内导入

# ===== 配置（从环境变量读取） =====
WEB_PASSWORD = os.environ.get("WEB_PASSWORD", "")
RCON_PORT = int(os.environ.get("RCON_PORT", "25575"))
RCON_PASS = os.environ.get("RCON_PASS", "")
SERVICE = os.environ.get("SERVICE", "pal-server")
BIND = os.environ.get("WEB_BIND", "0.0.0.0")
PORT = int(os.environ.get("WEB_PORT", "8080"))
PAL_SETTINGS = "/home/steam/Steam/steamapps/common/PalServer/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"
STEAM_USER = "steam"
REST_API_PORT = int(os.environ.get("REST_API_PORT", "8212"))
PAL_SERVER_DIR = "/home/steam/Steam/steamapps/common/PalServer"
SAVE_GAMES_DIR = f"{PAL_SERVER_DIR}/Pal/Saved/SaveGames"
BANLIST_FILE = f"{SAVE_GAMES_DIR}/banlist.txt"
BACKUP_DIR = f"/home/{STEAM_USER}/pal-backups"
WHITELIST_FILE = "/etc/pal-whitelist.json"

# ===== 常量 =====
SESSION_TTL = 7200
RATE_LIMIT_WINDOW = 60
RATE_LIMIT_MAX = 5
CSRF_TTL = 7200

# ===== 配置项 Schema（45 项，11 分类） =====
# type: text/password/number/bool/select/multiselect
CONFIG_SCHEMA = [
    # 基础
    {"key": "ServerName", "label": "服务器名称", "type": "text", "category": "基础", "desc": "玩家在服务器列表看到的名字"},
    {"key": "ServerDescription", "label": "服务器描述", "type": "text", "category": "基础", "desc": "服务器列表中的描述文字"},
    {"key": "AdminPassword", "label": "管理员密码", "type": "password", "category": "基础", "desc": "RCON 和管理操作使用的密码"},
    {"key": "ServerPassword", "label": "服务器密码", "type": "password", "category": "基础", "desc": "玩家进服密码，留空则无密码"},
    {"key": "ServerPlayerMaxNum", "label": "最大玩家数", "type": "number", "category": "基础", "min": 1, "max": 32, "step": 1, "desc": "同时在线玩家上限"},
    {"key": "PublicPort", "label": "游戏端口", "type": "number", "category": "基础", "min": 1, "max": 65535, "step": 1, "desc": "玩家连接的 UDP 端口"},
    {"key": "PublicIP", "label": "公网 IP", "type": "text", "category": "基础", "desc": "留空则由服务器自动检测"},
    # 远程管理
    {"key": "RCONEnabled", "label": "启用 RCON", "type": "bool", "category": "远程管理", "desc": "开启后可通过 RCON 远程管理服务器"},
    {"key": "RCONPort", "label": "RCON 端口", "type": "number", "category": "远程管理", "min": 1, "max": 65535, "step": 1, "desc": "RCON 协议监听端口"},
    {"key": "RESTAPIEnabled", "label": "启用 REST API", "type": "bool", "category": "远程管理", "desc": "v1.0 新增，仅本地访问，不建议暴露公网"},
    {"key": "RESTAPIPort", "label": "REST API 端口", "type": "number", "category": "远程管理", "min": 1, "max": 65535, "step": 1, "desc": "REST API 监听端口"},
    # 跨平台
    {"key": "CrossplayPlatforms", "label": "允许连接的平台", "type": "multiselect", "category": "跨平台", "options": ["Steam", "Xbox", "PS5", "Mac"], "desc": "v1.0 新增，控制哪些平台玩家可进服"},
    # 日志
    {"key": "LogFormatType", "label": "日志格式", "type": "select", "category": "日志", "options": ["Text", "Json"], "desc": "Json 便于日志聚合"},
    # 性能
    {"key": "ServerReplicatePawnCullDistance", "label": "Pal 同步距离", "type": "number", "category": "性能", "min": 5000, "max": 15000, "step": 500, "desc": "厘米，降低可减少网络负载"},
    {"key": "BaseCampMaxNumInGuild", "label": "每公会最大基地数", "type": "number", "category": "性能", "min": 0, "max": 10, "step": 1, "desc": "降低可减少服务器负载，0=禁用基地"},
    {"key": "BaseCampWorkerMaxNum", "label": "每基地最大帕鲁数", "type": "number", "category": "性能", "min": 0, "max": 50, "step": 1, "desc": "降低可减少计算量，0=禁用帕鲁工作"},
    {"key": "PalSpawnNumRate", "label": "帕鲁刷新率", "type": "number", "category": "性能", "min": 0, "max": 10, "step": 0.1, "desc": "影响帕鲁生成数量，1.0 为默认，0=不刷新"},
    {"key": "MaxBuildingLimitNum", "label": "每玩家建筑上限", "type": "number", "category": "性能", "min": 0, "max": 100000, "step": 1, "desc": "0 = 无限制"},
    # 存档
    {"key": "bIsUseBackupSaveData", "label": "启用自动备份", "type": "bool", "category": "存档", "desc": "服务器内置备份，会增加磁盘负载"},
    # 功能
    {"key": "bAllowClientMod", "label": "允许客户端 Mod", "type": "bool", "category": "功能", "desc": "允许带 mod 的玩家进服"},
    {"key": "bHardcore", "label": "硬核模式", "type": "bool", "category": "功能", "desc": "死亡后不能重生"},
    {"key": "bExistPlayerAfterLogout", "label": "离线角色留原地", "type": "bool", "category": "功能", "desc": "玩家离线后角色留在原地睡眠，可被攻击"},
    {"key": "bEnableFastTravel", "label": "启用快速旅行", "type": "bool", "category": "功能", "desc": ""},
    {"key": "bEnableBuildingPlayerUIdDisplay", "label": "显示建造者 ID", "type": "bool", "category": "功能", "desc": "显示建筑归属玩家"},
    {"key": "GuildPlayerMaxNum", "label": "公会人数上限", "type": "number", "category": "功能", "min": 0, "max": 100, "step": 1, "desc": "0=禁止公会（玩家无法创建/加入公会）"},
    {"key": "bAllowGlobalPalboxExport", "label": "允许 Palbox 导出", "type": "bool", "category": "功能", "desc": "Global Palbox 跨服转移"},
    {"key": "bAllowGlobalPalboxImport", "label": "允许 Palbox 导入", "type": "bool", "category": "功能", "desc": "Global Palbox 跨服转移"},
    # 语音
    {"key": "bEnableVoiceChat", "label": "启用语音聊天", "type": "bool", "category": "语音", "desc": "v1.0 新增"},
    {"key": "VoiceChatMaxVolumeDistance", "label": "最大音量距离", "type": "number", "category": "语音", "min": 0, "max": 10000, "step": 1, "desc": ""},
    {"key": "VoiceChatZeroVolumeDistance", "label": "零音量距离", "type": "number", "category": "语音", "min": 0, "max": 10000, "step": 1, "desc": ""},
    # 游戏平衡
    {"key": "ExpRate", "label": "经验倍率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "PalCaptureRate", "label": "帕鲁捕获率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "PalDamageRateAttack", "label": "帕鲁攻击力倍率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "PalDamageRateDefense", "label": "帕鲁防御力倍率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "PlayerDamageRateAttack", "label": "玩家攻击力倍率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "PlayerDamageRateDefense", "label": "玩家防御力倍率", "type": "number", "category": "游戏平衡", "min": 0, "max": 100, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "DeathPenalty", "label": "死亡惩罚", "type": "select", "category": "游戏平衡", "options": ["None", "Item", "ItemAndEquipment", "All"], "desc": "玩家死亡时掉落范围"},
    {"key": "DayTimeSpeedRate", "label": "白天时间流速", "type": "number", "category": "游戏平衡", "min": 0, "max": 10, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "NightTimeSpeedRate", "label": "夜晚时间流速", "type": "number", "category": "游戏平衡", "min": 0, "max": 10, "step": 0.1, "desc": "1.0 为默认"},
    {"key": "ChatPostLimitPerMinute", "label": "聊天限制(每分钟)", "type": "number", "category": "游戏平衡", "min": 0, "max": 1000, "step": 1, "desc": "每分钟最大消息数"},
    # 显示
    {"key": "bIsShowJoinLeftMessage", "label": "显示加入/离开消息", "type": "bool", "category": "显示", "desc": ""},
    {"key": "bShowPlayerList", "label": "显示玩家列表", "type": "bool", "category": "显示", "desc": ""},
    # PvP（试验）
    {"key": "bIsPvP", "label": "开启 PvP", "type": "bool", "category": "PvP（试验）", "desc": "v1.0 试验功能，需同时开启下两项"},
    {"key": "bEnablePlayerToPlayerDamage", "label": "允许玩家间伤害", "type": "bool", "category": "PvP（试验）", "desc": "PvP 必开"},
    {"key": "bEnableDefenseOtherGuildPlayer", "label": "允许跨公会防御", "type": "bool", "category": "PvP（试验）", "desc": "PvP 必开"},
]

SCHEMA_MAP = {m["key"]: m for m in CONFIG_SCHEMA}
CATEGORY_ORDER = []
for m in CONFIG_SCHEMA:
    if m["category"] not in CATEGORY_ORDER:
        CATEGORY_ORDER.append(m["category"])

# ===== 内存状态 =====
SESSIONS = {}
LOGIN_ATTEMPTS = {}
CSRF_TOKENS = {}


# ===== 工具函数：调用现有命令 =====
def rcon(cmd):
    try:
        r = subprocess.run(
            ["/usr/local/bin/pal-rcon", "--port", str(RCON_PORT),
             "--password", RCON_PASS, cmd],
            capture_output=True, text=True, timeout=10,
        )
        return r.returncode, r.stdout.strip(), r.stderr.strip()
    except subprocess.TimeoutExpired:
        return 1, "", "RCON 超时"
    except Exception as e:
        return 1, "", str(e)


def systemctl(action):
    try:
        r = subprocess.run(
            ["systemctl", action, SERVICE],
            capture_output=True, text=True, timeout=30,
        )
        return r.returncode, r.stdout.strip(), r.stderr.strip()
    except Exception as e:
        return 1, "", str(e)


def get_status():
    r = subprocess.run(
        ["systemctl", "is-active", SERVICE],
        capture_output=True, text=True,
    )
    active = r.stdout.strip() == "active"
    r2 = subprocess.run(
        ["systemctl", "show", SERVICE,
         "--property=ActiveEnterTimestamp", "--value"],
        capture_output=True, text=True,
    )
    return {"active": active, "uptime": r2.stdout.strip()}


def get_memory():
    r = subprocess.run(
        ["systemctl", "show", SERVICE,
         "--property=MemoryCurrent", "--property=MemoryPeak", "--property=MemoryMax"],
        capture_output=True, text=True,
    )
    result = {}
    for line in r.stdout.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            if v in ("infinity", "[not set]", ""):
                continue
            try:
                result[k] = int(v)
            except ValueError:
                pass
    return result


def get_players():
    """优先用 REST API /v1/api/players（数据丰富），RCON ShowPlayers 作为 fallback。
    Palworld v1.0 的 RCON ShowPlayers 有 bug（有玩家在线时不返回响应），必须用 REST API。
    """
    try:
        import urllib.request, base64
        url = f"http://127.0.0.1:{REST_API_PORT}/v1/api/players"
        req = urllib.request.Request(url)
        auth = base64.b64encode(f"admin:{RCON_PASS}".encode()).decode()
        req.add_header("Authorization", f"Basic {auth}")
        with urllib.request.urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read().decode("utf-8"))
        players = []
        for p in data.get("players", []):
            players.append({
                "name": p.get("name", ""),
                "playeruid": p.get("playerId", ""),
                "steamid": p.get("userId", "").replace("steam_", ""),
                "ping": round(p.get("ping", 0), 1),
                "level": p.get("level", 0),
                "ip": p.get("iP", ""),
            })
        return players
    except Exception:
        pass

    # fallback: RCON ShowPlayers（无玩家时可能可用）
    rc, out, _ = rcon("ShowPlayers")
    if rc != 0 or not out:
        return []
    players = []
    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        parts = [p.strip() for p in line.split(",")]
        if parts[0].lower() == "name":
            continue
        if len(parts) >= 3:
            players.append({
                "name": parts[0],
                "playeruid": parts[1],
                "steamid": parts[2],
            })
    return players


def get_logs(n=200):
    try:
        r = subprocess.run(
            ["journalctl", "-u", SERVICE, "-n", str(n), "--no-pager"],
            capture_output=True, text=True, timeout=10,
        )
        return r.stdout
    except Exception as e:
        return f"获取日志失败: {e}"


# ===== 配置文件解析/写入 =====
def parse_settings(text):
    """从 PalWorldSettings.ini 文本解析出 {key: raw_value} dict。
    去掉 ; 注释行后再解析，避免把注释里的 PvP 模板当成实际配置。
    """
    cleaned_lines = []
    for line in text.splitlines():
        idx = line.find(';')
        if idx >= 0:
            line = line[:idx]
        cleaned_lines.append(line)
    cleaned = '\n'.join(cleaned_lines)

    m = re.search(r'OptionSettings=\((.*)\)', cleaned, re.DOTALL)
    if not m:
        return {}
    content = m.group(1)
    # Key=Value，Value 可以是 "string"、(a,b,c)、True/False、数字、枚举
    pattern = r'(\w+)\s*=\s*("[^"]*"|\([^)]*\)|[^,)\s]+)'
    matches = re.findall(pattern, content)
    return {k: v for k, v in matches}


def format_value(key, value, meta):
    """根据 schema meta 把表单值格式化为 ini 字面量。"""
    vtype = meta.get("type", "text")
    if vtype in ("text", "password"):
        v = str(value).strip().strip('"')
        return f'"{v}"'
    elif vtype == "bool":
        if isinstance(value, bool):
            return "True" if value else "False"
        if isinstance(value, str):
            return "True" if value.lower() in ("true", "1", "yes", "on") else "False"
        return "True" if value else "False"
    elif vtype == "multiselect":
        if isinstance(value, list):
            return "(" + ",".join(value) + ")"
        if isinstance(value, str):
            if value.startswith("("):
                return value
            return f"({value})" if value else "()"
        return "()"
    else:  # number, select
        if vtype == "number":
            step = meta.get("step", 1)
            if step < 1:
                return f"{float(value):.6f}"
            return str(int(value))
        return str(value)


def get_config():
    """读取配置文件，返回分类组织的 schema + 当前值。"""
    try:
        with open(PAL_SETTINGS, 'r', encoding='utf-8') as f:
            text = f.read()
    except FileNotFoundError:
        return {"categories": [], "error": f"配置文件不存在: {PAL_SETTINGS}"}
    except Exception as e:
        return {"categories": [], "error": str(e)}

    settings = parse_settings(text)

    categories = []
    for cat_name in CATEGORY_ORDER:
        items = []
        for meta in CONFIG_SCHEMA:
            if meta["category"] != cat_name:
                continue
            item = dict(meta)
            raw = settings.get(meta["key"], "")
            item["value"] = normalize_value(raw, meta)
            items.append(item)
        categories.append({"name": cat_name, "items": items})
    return {"categories": categories}


def normalize_value(raw, meta):
    """把 ini 字面量转为前端友好的值（bool->bool, multiselect->list, 去引号）。"""
    vtype = meta.get("type", "text")
    if not raw:
        if vtype == "bool":
            return False
        if vtype == "multiselect":
            return []
        if vtype == "number":
            return 0
        return ""
    if vtype in ("text", "password"):
        return raw.strip('"')
    elif vtype == "bool":
        return raw.strip().lower() == "true"
    elif vtype == "multiselect":
        v = raw.strip().strip('()')
        if not v:
            return []
        return [x.strip() for x in v.split(',')]
    elif vtype == "number":
        try:
            if "step" in meta and meta.get("step", 1) < 1:
                return float(raw)
            return int(raw)
        except ValueError:
            try:
                return float(raw)
            except ValueError:
                return 0
    else:  # select
        return raw.strip()


def write_settings(new_settings):
    """读取原文件，合并新设置，备份后写回，chown 回 steam + chmod 444 只读。
    new_settings: {key: 已格式化的 ini 字面量字符串}
    """
    with open(PAL_SETTINGS, 'r', encoding='utf-8') as f:
        original = f.read()

    old = parse_settings(original)
    merged = {**old, **new_settings}

    # 生成单行 OptionSettings（Palworld v1.0 要求单行格式，多行会被重写破坏）
    items = [f"{k}={v}" for k, v in merged.items()]
    new_option = "OptionSettings=(" + ",".join(items) + ")"

    new_text = re.sub(
        r'OptionSettings=\(.*\)',
        lambda _: new_option,
        original,
        count=1,
        flags=re.DOTALL,
    )

    backup = f"{PAL_SETTINGS}.bak.{int(time.time())}"
    shutil.copy2(PAL_SETTINGS, backup)

    # 配置文件是 444 只读，先 chmod 644 解除只读才能写入
    os.chmod(PAL_SETTINGS, 0o644)
    with open(PAL_SETTINGS, 'w', encoding='utf-8') as f:
        f.write(new_text)

    # root 写入会改属主，不 chown 回 steam 会导致服务器读不了配置
    import pwd, grp
    uid = pwd.getpwnam(STEAM_USER).pw_uid
    gid = grp.getgrnam(STEAM_USER).gr_gid
    os.chown(PAL_SETTINGS, uid, gid)
    # 重新设只读，阻止 Palworld 启动时重写配置破坏格式
    os.chmod(PAL_SETTINGS, 0o444)

    try:
        os.chown(backup, uid, gid)
    except OSError:
        pass

    return True


def validate_and_format(body):
    """验证前端提交的表单，返回 (formatted_dict, error_msg)。"""
    formatted = {}
    for key, value in body.items():
        meta = SCHEMA_MAP.get(key)
        if not meta:
            continue  # 忽略未知 key
        vtype = meta["type"]

        if vtype == "number":
            try:
                step = meta.get("step", 1)
                if step < 1:
                    num = float(value)
                else:
                    num = int(value)
            except (ValueError, TypeError):
                return None, f"{meta['label']} 必须是数字"
            if "min" in meta and num < meta["min"]:
                return None, f"{meta['label']} 不能小于 {meta['min']}"
            if "max" in meta and num > meta["max"]:
                return None, f"{meta['label']} 不能大于 {meta['max']}"
            value = num
        elif vtype == "text":
            if key == "ServerName" and not str(value).strip():
                return None, "服务器名称不能为空"
        elif vtype == "bool":
            if isinstance(value, str):
                value = value.lower() in ("true", "1", "yes", "on")

        formatted[key] = format_value(key, value, meta)
    return formatted, None


# ===== 存档管理 =====
def get_saves():
    """列出所有备份，返回 [{name, size, time}]，最新在前"""
    import glob
    files = glob.glob(os.path.join(BACKUP_DIR, "pal_backup_*.tar.gz"))
    saves = []
    for f in files:
        try:
            stat = os.stat(f)
        except OSError:
            continue
        saves.append({
            "name": os.path.basename(f),
            "size": stat.st_size,
            "time": int(stat.st_mtime),
        })
    saves.sort(key=lambda s: s["time"], reverse=True)
    return saves


def valid_backup_name(name):
    return isinstance(name, str) and re.fullmatch(r"pal_backup_[A-Za-z0-9_.-]+\.tar\.gz", name) is not None


def backup_save():
    """调 pal-backup 脚本立即备份"""
    import subprocess
    try:
        r = subprocess.run(["/usr/local/bin/pal-backup"], capture_output=True, text=True, timeout=180)
        return r.returncode == 0, r.stdout + r.stderr
    except subprocess.TimeoutExpired:
        return False, "备份超时"
    except Exception as e:
        return False, str(e)


def restore_save(name):
    """恢复备份：RCON Save -> 停服 -> 解压覆盖 SaveGames -> chown -> 启服"""
    import subprocess, tempfile
    if not valid_backup_name(name):
        return False, "无效的备份文件名"
    backup_file = os.path.join(BACKUP_DIR, name)
    if not os.path.isfile(backup_file):
        return False, "备份文件不存在"

    rcon("Save")
    time.sleep(3)
    subprocess.run(["systemctl", "stop", SERVICE], capture_output=True, timeout=30)
    time.sleep(2)

    tmpdir = tempfile.mkdtemp(prefix="pal_restore_")
    try:
        r = subprocess.run(["tar", "-xzf", backup_file, "-C", tmpdir],
                           capture_output=True, timeout=180)
        if r.returncode != 0:
            return False, f"解压失败: {r.stderr.decode()}"
        src = os.path.join(tmpdir, "SaveGames")
        if not os.path.isdir(src):
            return False, "备份包内无 SaveGames 目录"
        # 清空现有 SaveGames（保留 banlist.txt）
        for item in os.listdir(SAVE_GAMES_DIR):
            if item == "banlist.txt":
                continue
            item_path = os.path.join(SAVE_GAMES_DIR, item)
            if os.path.isdir(item_path):
                shutil.rmtree(item_path)
            else:
                os.remove(item_path)
        # 复制备份内容
        for item in os.listdir(src):
            s = os.path.join(src, item)
            d = os.path.join(SAVE_GAMES_DIR, item)
            if os.path.isdir(s):
                shutil.copytree(s, d)
            else:
                shutil.copy2(s, d)
        # chown 回 steam
        import pwd, grp
        uid = pwd.getpwnam(STEAM_USER).pw_uid
        gid = grp.getgrnam(STEAM_USER).gr_gid
        for root, dirs, files in os.walk(SAVE_GAMES_DIR):
            os.chown(root, uid, gid)
            for fn in files:
                try:
                    os.chown(os.path.join(root, fn), uid, gid)
                except OSError:
                    pass
        return True, "恢复完成，服务器正在重启"
    except Exception as e:
        return False, f"恢复失败: {e}"
    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)
        subprocess.run(["systemctl", "start", SERVICE], capture_output=True, timeout=30)


def delete_save(name):
    """删除备份文件"""
    if not valid_backup_name(name):
        return False, "无效的备份文件名"
    backup_file = os.path.join(BACKUP_DIR, name)
    if not os.path.isfile(backup_file):
        return False, "备份文件不存在"
    os.remove(backup_file)
    return True, "已删除"


# ===== 白名单 =====
def get_whitelist():
    """读白名单 JSON"""
    if not os.path.isfile(WHITELIST_FILE):
        return []
    try:
        with open(WHITELIST_FILE, "r", encoding="utf-8") as f:
            data = json.load(f)
        return data if isinstance(data, list) else []
    except (json.JSONDecodeError, IOError):
        return []


def _write_whitelist(data):
    with open(WHITELIST_FILE, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def add_whitelist(name, steamid, playeruid):
    """添加白名单条目，空字段=通配符"""
    wl = get_whitelist()
    entry = {"name": (name or "").strip(), "steamid": (steamid or "").strip(),
             "playeruid": (playeruid or "").strip()}
    if entry["steamid"]:
        wl = [w for w in wl if w.get("steamid") != entry["steamid"]]
    wl.append(entry)
    _write_whitelist(wl)
    return True, "已添加"


def remove_whitelist(steamid):
    """按 steamid 移除白名单条目"""
    wl = get_whitelist()
    new_wl = [w for w in wl if w.get("steamid") != steamid]
    _write_whitelist(new_wl)
    return True, f"已移除 {len(wl) - len(new_wl)} 条"


def check_whitelist():
    """检查在线玩家，踢出不在白名单的。白名单为空=不启用"""
    wl = get_whitelist()
    if not wl:
        return True, "白名单为空，未启用"
    players = get_players()
    kicked = []
    for p in players:
        sid = p.get("steamid", "")
        if not sid:
            continue
        matched = False
        for w in wl:
            w_sid = w.get("steamid", "")
            if w_sid and w_sid != sid:
                continue
            matched = True
            break
        if not matched:
            rc, _, _ = rcon(f"KickPlayer {sid}")
            if rc == 0:
                kicked.append(p.get("name", sid))
    return True, f"已踢出 {len(kicked)} 人" + (f": {', '.join(kicked)}" if kicked else "")


# ===== 封禁列表 =====
def get_banlist():
    """读 banlist.txt，返回 [{steamid}]"""
    if not os.path.isfile(BANLIST_FILE):
        return []
    try:
        with open(BANLIST_FILE, "r", encoding="utf-8") as f:
            lines = f.read().splitlines()
    except IOError:
        return []
    result = []
    for line in lines:
        line = line.strip()
        if not line:
            continue
        sid = line.replace("steam_", "", 1)
        result.append({"steamid": sid})
    return result


def unban(steamid):
    """解封：精确匹配 steam_<steamid> 整行移除"""
    if not os.path.isfile(BANLIST_FILE):
        return False, "banlist 文件不存在"
    with open(BANLIST_FILE, "r", encoding="utf-8") as f:
        lines = f.read().splitlines()
    target = f"steam_{steamid}"
    new_lines = [l for l in lines if l.strip() != target]
    with open(BANLIST_FILE, "w", encoding="utf-8") as f:
        if new_lines:
            f.write("\n".join(new_lines) + "\n")
    try:
        import pwd, grp
        uid = pwd.getpwnam(STEAM_USER).pw_uid
        gid = grp.getgrnam(STEAM_USER).gr_gid
        os.chown(BANLIST_FILE, uid, gid)
    except Exception:
        pass
    return True, f"已解封 {steamid}"


# ===== 系统信息 =====
def get_sysinfo():
    """纯 stdlib 读 /proc 获取系统信息"""
    result = {}
    # CPU 使用率（两次采样差值）
    def _read_cpu():
        with open("/proc/stat", "r") as f:
            line = f.readline()
        nums = [int(x) for x in line.split()[1:]]
        idle = nums[3] + nums[4] if len(nums) > 4 else nums[3]
        total = sum(nums)
        return total, idle
    try:
        t1, i1 = _read_cpu()
        time.sleep(0.1)
        t2, i2 = _read_cpu()
        dt = t2 - t1
        di = i2 - i1
        result["cpu_percent"] = round((1 - di / dt) * 100, 1) if dt > 0 else 0
    except Exception:
        result["cpu_percent"] = 0

    # 内存
    try:
        with open("/proc/meminfo", "r") as f:
            mi = {}
            for line in f:
                if ":" in line:
                    k, v = line.split(":", 1)
                    mi[k.strip()] = int(v.strip().split()[0]) * 1024
        total = mi.get("MemTotal", 0)
        avail = mi.get("MemAvailable", 0)
        result["memory"] = {
            "total": total, "available": avail, "used": total - avail,
            "percent": round((total - avail) / total * 100, 1) if total > 0 else 0,
        }
    except Exception:
        result["memory"] = {"total": 0, "available": 0, "used": 0, "percent": 0}

    # 磁盘（PalServer 所在分区）
    try:
        du = shutil.disk_usage(PAL_SERVER_DIR)
        result["disk"] = {
            "total": du.total, "used": du.used, "free": du.free,
            "percent": round(du.used / du.total * 100, 1) if du.total > 0 else 0,
        }
    except Exception:
        result["disk"] = {"total": 0, "used": 0, "free": 0, "percent": 0}

    # 系统运行时长
    try:
        with open("/proc/uptime", "r") as f:
            result["uptime"] = int(float(f.read().split()[0]))
    except Exception:
        result["uptime"] = 0

    return result


# ===== Session / Auth =====
def create_session(ip):
    token = secrets.token_urlsafe(32)
    SESSIONS[token] = {"expiry": time.time() + SESSION_TTL, "ip": ip}
    return token


def validate_session(token, ip):
    s = SESSIONS.get(token)
    if not s:
        return False
    if time.time() > s["expiry"]:
        SESSIONS.pop(token, None)
        return False
    if s["ip"] != ip:
        return False
    s["expiry"] = time.time() + SESSION_TTL
    return True


def destroy_session(token):
    SESSIONS.pop(token, None)


def create_csrf_token():
    token = secrets.token_urlsafe(16)
    CSRF_TOKENS[token] = time.time() + CSRF_TTL
    return token


def validate_csrf(token):
    expiry = CSRF_TOKENS.get(token)
    if not expiry:
        return False
    if time.time() > expiry:
        CSRF_TOKENS.pop(token, None)
        return False
    return True


def check_rate_limit(ip):
    now = time.time()
    attempts = [t for t in LOGIN_ATTEMPTS.get(ip, [])
                if now - t < RATE_LIMIT_WINDOW]
    LOGIN_ATTEMPTS[ip] = attempts
    return len(attempts) < RATE_LIMIT_MAX


def record_failed_login(ip):
    LOGIN_ATTEMPTS.setdefault(ip, []).append(time.time())


# ===== HTTP Handler =====
class Handler(http.server.BaseHTTPRequestHandler):
    server_version = "Palweb/2.0"

    def log_message(self, fmt, *args):
        print(f"[{self.client_address[0]}] {fmt % args}")

    def client_ip(self):
        return self.client_address[0]

    def get_session_token(self):
        cookie = SimpleCookie(self.headers.get("Cookie", ""))
        m = cookie.get("session")
        return m.value if m else None

    def send_json(self, data, code=200, extra_headers=None):
        body = json.dumps(data, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        if extra_headers:
            for k, v in extra_headers.items():
                self.send_header(k, v)
        self.end_headers()
        self.wfile.write(body)

    def send_html(self, html, code=200):
        body = html.encode()
        self.send_response(code)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def read_json(self):
        length = int(self.headers.get("Content-Length", 0))
        if length == 0:
            return {}
        raw = self.rfile.read(length).decode()
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {}

    def require_auth(self):
        token = self.get_session_token()
        if not token or not validate_session(token, self.client_ip()):
            self.send_json({"error": "未登录"}, 401)
            return False
        return True

    def require_csrf(self):
        token = self.headers.get("X-CSRF-Token", "")
        if not token or not validate_csrf(token):
            self.send_json({"error": "CSRF token 无效"}, 403)
            return False
        return True

    def do_GET(self):
        if self.path in ("/", "/login"):
            self.send_html(DASHBOARD_HTML)
            return
        if self.path == "/api/status":
            if not self.require_auth():
                return
            self.send_json(get_status())
        elif self.path == "/api/players":
            if not self.require_auth():
                return
            self.send_json({"players": get_players()})
        elif self.path == "/api/memory":
            if not self.require_auth():
                return
            self.send_json(get_memory())
        elif self.path == "/api/logs":
            if not self.require_auth():
                return
            self.send_json({"logs": get_logs()})
        elif self.path == "/api/csrf":
            if not self.require_auth():
                return
            self.send_json({"token": create_csrf_token()})
        elif self.path == "/api/config":
            if not self.require_auth():
                return
            self.send_json(get_config())
        elif self.path == "/api/sysinfo":
            if not self.require_auth():
                return
            self.send_json(get_sysinfo())
        elif self.path == "/api/saves":
            if not self.require_auth():
                return
            self.send_json({"saves": get_saves()})
        elif self.path.startswith("/api/saves/download"):
            if not self.require_auth():
                return
            from urllib.parse import urlparse, parse_qs
            qs = parse_qs(urlparse(self.path).query)
            name = (qs.get("name", [""])[0] or "").strip()
            if not valid_backup_name(name):
                self.send_json({"error": "无效文件名（必须 pal_backup_*.tar.gz）"}, 400)
                return
            filepath = os.path.join(BACKUP_DIR, name)
            if not os.path.isfile(filepath):
                self.send_json({"error": "文件不存在"}, 404)
                return
            with open(filepath, "rb") as f:
                data = f.read()
            self.send_response(200)
            self.send_header("Content-Type", "application/gzip")
            self.send_header("Content-Disposition", f'attachment; filename="{name}"')
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        elif self.path == "/api/whitelist":
            if not self.require_auth():
                return
            self.send_json({"whitelist": get_whitelist()})
        elif self.path == "/api/banlist":
            if not self.require_auth():
                return
            self.send_json({"banlist": get_banlist()})
        else:
            self.send_json({"error": "not found"}, 404)

    def do_POST(self):
        if self.path == "/api/login":
            self.handle_login()
            return
        if not self.require_auth():
            return
        if not self.require_csrf():
            return
        body = self.read_json()

        if self.path == "/api/logout":
            token = self.get_session_token()
            if token:
                destroy_session(token)
            self.send_json({"ok": True})
        elif self.path == "/api/start":
            rc, _, err = systemctl("start")
            self.send_json({"ok": rc == 0, "error": err})
        elif self.path == "/api/stop":
            rc, _, err = systemctl("stop")
            self.send_json({"ok": rc == 0, "error": err})
        elif self.path == "/api/restart":
            rc, _, err = systemctl("restart")
            self.send_json({"ok": rc == 0, "error": err})
        elif self.path == "/api/save":
            rc, out, err = rcon("Save")
            self.send_json({"ok": rc == 0, "output": out, "error": err})
        elif self.path == "/api/broadcast":
            msg = body.get("message", "").strip()
            if not msg:
                self.send_json({"error": "消息不能为空"}, 400)
                return
            rc, out, err = rcon(f"Broadcast {msg}")
            self.send_json({"ok": rc == 0, "output": out, "error": err})
        elif self.path in ("/api/kick", "/api/ban", "/api/unban"):
            steamid = body.get("steamid", "").strip()
            if not steamid:
                self.send_json({"error": "steamid 不能为空"}, 400)
                return
            cmd_map = {
                "/api/kick": "KickPlayer",
                "/api/ban": "BanPlayer",
                "/api/unban": "UnBanPlayer",
            }
            rc, out, err = rcon(f"{cmd_map[self.path]} {steamid}")
            self.send_json({"ok": rc == 0, "output": out, "error": err})
        elif self.path == "/api/config":
            formatted, err = validate_and_format(body)
            if err:
                self.send_json({"error": err}, 400)
                return
            try:
                write_settings(formatted)
                self.send_json({"ok": True, "warning": "配置已保存，需重启服务器生效"})
            except Exception as e:
                self.send_json({"error": f"写入配置失败: {e}"}, 500)
        elif self.path == "/api/config/restart":
            rc, _, err = systemctl("restart")
            self.send_json({"ok": rc == 0, "error": err})
        elif self.path == "/api/saves/backup":
            ok, msg = backup_save()
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/saves/restore":
            name = body.get("name", "").strip()
            if not name:
                self.send_json({"error": "name 不能为空"}, 400)
                return
            ok, msg = restore_save(name)
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/saves/delete":
            name = body.get("name", "").strip()
            if not name:
                self.send_json({"error": "name 不能为空"}, 400)
                return
            ok, msg = delete_save(name)
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/saves/upload":
            import base64
            name = body.get("name", "").strip()
            content = body.get("content", "")
            if not valid_backup_name(name):
                self.send_json({"error": "无效文件名（必须 pal_backup_*.tar.gz）"}, 400)
                return
            try:
                data = base64.b64decode(content)
            except Exception:
                self.send_json({"error": "无效的文件内容"}, 400)
                return
            if len(data) > 500 * 1024 * 1024:  # 500MB 上限
                self.send_json({"error": "文件过大（>500MB）"}, 400)
                return
            filepath = os.path.join(BACKUP_DIR, name)
            with open(filepath, "wb") as f:
                f.write(data)
            try:
                import pwd, grp
                uid = pwd.getpwnam(STEAM_USER).pw_uid
                gid = grp.getgrnam(STEAM_USER).gr_gid
                os.chown(filepath, uid, gid)
            except Exception:
                pass
            self.send_json({"ok": True, "message": f"已上传 {name} ({len(data)} 字节)"})
        elif self.path == "/api/whitelist/add":
            ok, msg = add_whitelist(body.get("name", ""), body.get("steamid", ""), body.get("playeruid", ""))
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/whitelist/remove":
            sid = body.get("steamid", "").strip()
            if not sid:
                self.send_json({"error": "steamid 不能为空"}, 400)
                return
            ok, msg = remove_whitelist(sid)
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/whitelist/check":
            ok, msg = check_whitelist()
            self.send_json({"ok": ok, "message": msg})
        elif self.path == "/api/banlist/unban":
            sid = body.get("steamid", "").strip()
            if not sid:
                self.send_json({"error": "steamid 不能为空"}, 400)
                return
            ok, msg = unban(sid)
            self.send_json({"ok": ok, "message": msg})
        else:
            self.send_json({"error": "not found"}, 404)

    def handle_login(self):
        ip = self.client_ip()
        if not check_rate_limit(ip):
            self.send_json({"error": "尝试过于频繁，请 1 分钟后再试"}, 429)
            return
        body = self.read_json()
        password = body.get("password", "")
        if not password:
            self.send_json({"error": "密码不能为空"}, 400)
            return
        if password != WEB_PASSWORD:
            record_failed_login(ip)
            self.send_json({"error": "密码错误"}, 401)
            return
        token = create_session(ip)
        csrf = create_csrf_token()
        cookie = (f"session={token}; HttpOnly; SameSite=Lax; Path=/; "
                  f"Max-Age={SESSION_TTL}")
        self.send_json(
            {"ok": True, "csrf": csrf},
            extra_headers={"Set-Cookie": cookie},
        )


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


# ===== 前端 HTML =====
DASHBOARD_HTML = r"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Palworld 管理面板</title>
<style>
:root {
  --bg: #f5f5f5;
  --card: #ffffff;
  --card-hover: #f0f0f0;
  --primary: #1976D2;
  --primary-hover: #1565C0;
  --secondary: #26A69A;
  --accent: #9C27B0;
  --positive: #21BA45;
  --danger: #C10015;
  --info: #31CCEC;
  --warning: #F2C037;
  --text: #1d1d1d;
  --text-muted: #666666;
  --border: #e0e0e0;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, "Segoe UI", Roboto, "Microsoft YaHei", sans-serif;
  background: var(--bg);
  color: var(--text);
  min-height: 100vh;
}
.topbar {
  background: var(--card);
  padding: 14px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border);
  box-shadow: 0 2px 8px rgba(0,0,0,0.12);
}
.topbar h1 { font-size: 18px; color: var(--text); font-weight: 600; }
.topbar h1 span { color: var(--primary); }
.tabs { display: flex; gap: 4px; }
.tab {
  background: transparent;
  color: var(--text-muted);
  border: none;
  padding: 8px 16px;
  cursor: pointer;
  font-size: 14px;
  border-radius: 6px;
  transition: all 0.2s;
}
.tab:hover { color: var(--text); background: var(--card-hover); }
.tab.active { color: var(--primary); background: var(--card-hover); font-weight: 600; }
.badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: bold;
}
.badge.active { background: var(--positive); color: #fff; }
.badge.inactive { background: var(--danger); color: #fff; }
.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  color: #fff;
  transition: all 0.2s;
  font-weight: 500;
}
.btn:hover { opacity: 0.9; transform: translateY(-1px); }
.btn:active { transform: translateY(0); }
.btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
.btn-start { background: var(--positive); }
.btn-stop { background: var(--warning); color: var(--text); }
.btn-restart { background: var(--primary); }
.btn-save { background: var(--positive); }
.btn-upload { background: var(--secondary); }
.btn-download { background: var(--info); padding: 4px 10px; font-size: 12px; }
.btn-save-restart { background: var(--primary); }
.btn-kick { background: var(--warning); padding: 4px 10px; font-size: 12px; color: var(--text); }
.btn-ban { background: var(--danger); padding: 4px 10px; font-size: 12px; }
.btn-logout { background: var(--card-hover); color: var(--text); }
.container { max-width: 1200px; margin: 24px auto; padding: 0 20px; }
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}
.card {
  background: var(--card);
  padding: 18px;
  border-radius: 12px;
  border-left: 4px solid var(--primary);
  box-shadow: 0 2px 10px rgba(0,0,0,0.10);
  transition: transform 0.2s;
}
.card:hover { transform: translateY(-2px); }
.card .label { font-size: 12px; color: var(--text-muted); margin-bottom: 8px; text-transform: uppercase; letter-spacing: 0.5px; }
.card .value { font-size: 22px; font-weight: 700; }
.panel {
  background: var(--card);
  padding: 20px;
  border-radius: 12px;
  margin-bottom: 20px;
  box-shadow: 0 2px 10px rgba(0,0,0,0.10);
}
.panel h2 {
  font-size: 15px;
  color: var(--primary);
  margin-bottom: 14px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 10px;
  font-weight: 600;
}
.row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
input[type="text"], input[type="password"], input[type="number"], select, textarea {
  background: var(--card);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 9px 12px;
  border-radius: 6px;
  font-size: 14px;
  flex: 1;
  min-width: 180px;
  transition: border-color 0.2s;
  font-family: inherit;
}
input:focus, select:focus, textarea:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px rgba(25, 118, 210, 0.12);
}
table { width: 100%; border-collapse: collapse; }
th, td { padding: 10px 12px; text-align: left; border-bottom: 1px solid var(--border); }
th { font-size: 12px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; }
.logs {
  background: #263238;
  padding: 14px;
  border-radius: 6px;
  font-family: "Cascadia Code", Consolas, monospace;
  font-size: 12px;
  max-height: 400px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-muted);
}
.login-wrap { display: flex; justify-content: center; padding-top: 80px; }
.login-card {
  background: var(--card);
  padding: 36px;
  border-radius: 12px;
  width: 100%;
  max-width: 380px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.16);
}
.login-card h1 { text-align: center; margin-bottom: 24px; color: var(--primary); font-size: 22px; }
.login-card .error { color: var(--danger); font-size: 13px; margin-top: 10px; text-align: center; }
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 14px 20px;
  border-radius: 8px;
  color: #fff;
  font-size: 14px;
  opacity: 0;
  transform: translateY(10px);
  transition: all 0.3s;
  z-index: 999;
  box-shadow: 0 4px 16px rgba(0,0,0,0.18);
}
.toast.show { opacity: 1; transform: translateY(0); }
.toast.ok { background: var(--positive); }
.toast.err { background: var(--danger); }

/* 首页美化 */
.status-card {
  background: var(--card);
  padding: 24px;
  border-radius: 12px;
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 18px;
  box-shadow: 0 2px 10px rgba(0,0,0,0.10);
  border-left: 4px solid var(--positive);
}
.status-card.inactive { border-left-color: var(--danger); }
.status-card .status-icon {
  width: 48px; height: 48px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-size: 24px;
  background: rgba(33, 186, 69, 0.15);
  color: var(--positive);
}
.status-card.inactive .status-icon {
  background: rgba(193, 0, 21, 0.12);
  color: var(--danger);
}
.status-card .status-text { flex: 1; }
.status-card .status-text .title { font-size: 18px; font-weight: 700; margin-bottom: 4px; }
.status-card .status-text .sub { font-size: 13px; color: var(--text-muted); }

.metric-card {
  background: var(--card);
  padding: 18px;
  border-radius: 12px;
  box-shadow: 0 2px 10px rgba(0,0,0,0.10);
  border-left: 4px solid var(--primary);
}
.metric-card .metric-head {
  display: flex; align-items: center; gap: 8px;
  font-size: 12px; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.5px;
  margin-bottom: 10px;
}
.metric-card .metric-icon { font-size: 16px; }
.metric-card .metric-value { font-size: 26px; font-weight: 700; margin-bottom: 8px; }
.metric-card .progress {
  height: 6px;
  background: var(--bg);
  border-radius: 3px;
  overflow: hidden;
}
.metric-card .progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--positive), var(--primary));
  border-radius: 3px;
  transition: width 0.6s ease;
}
.metric-card .progress-fill.high { background: linear-gradient(90deg, var(--warning), var(--danger)); }
.metric-card .metric-sub { font-size: 11px; color: var(--text-muted); margin-top: 6px; }

.btn-icon { margin-right: 6px; }

.player-row {
  display: flex; align-items: center; gap: 12px;
  padding: 12px;
  border-radius: 8px;
  background: var(--bg);
  margin-bottom: 8px;
}
.player-row:last-child { margin-bottom: 0; }
.player-row .player-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: var(--positive);
  box-shadow: 0 0 8px rgba(33, 186, 69, 0.5);
  flex-shrink: 0;
}
.player-row .player-name { flex: 1; font-weight: 600; }
.player-row .player-meta { display: flex; gap: 6px; }
.player-row .player-tag {
  font-size: 11px; padding: 2px 8px; border-radius: 10px;
  background: var(--card-hover); color: var(--text-muted);
  white-space: nowrap;
}
.player-row .player-steamid {
  font-family: "Cascadia Code", Consolas, monospace;
  font-size: 12px; color: var(--text-muted);
}
.player-row .player-actions { display: flex; gap: 6px; }

.logs .log-line { padding: 2px 0; }
.logs .log-error { color: var(--danger); }
.logs .log-warn { color: var(--warning); }
.logs .log-connect, .logs .log-join { color: var(--positive); }
.logs .log-disconnect, .logs .log-leave { color: var(--text-muted); }

.panel-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 14px;
  border-bottom: 1px solid var(--border);
  padding-bottom: 10px;
}
.panel-header h2 {
  font-size: 15px; color: var(--primary); font-weight: 600;
  display: flex; align-items: center; gap: 8px;
}
.panel-header .panel-sub { font-size: 11px; color: var(--text-muted); font-weight: 400; }
.panel-header .count-badge {
  background: var(--primary); color: #fff;
  padding: 2px 8px; border-radius: 10px; font-size: 12px; font-weight: 600;
}

/* 系统信息卡片 */
.sys-card {
  background: var(--card);
  padding: 16px;
  border-radius: 10px;
  border-left: 4px solid var(--secondary);
  box-shadow: 0 2px 8px rgba(0,0,0,0.12);
}
.sys-card .sys-head {
  display: flex; align-items: center; gap: 6px;
  font-size: 12px; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.5px;
  margin-bottom: 8px;
}
.sys-card .sys-value { font-size: 22px; font-weight: 700; margin-bottom: 6px; }
.sys-card .sys-bar {
  height: 5px; background: var(--bg); border-radius: 3px; overflow: hidden;
}
.sys-card .sys-bar-fill {
  height: 100%; background: linear-gradient(90deg, var(--secondary), var(--primary));
  border-radius: 3px; transition: width 0.6s ease;
}
.sys-card .sys-bar-fill.high { background: linear-gradient(90deg, var(--warning), var(--danger)); }
.sys-card .sys-sub { font-size: 11px; color: var(--text-muted); margin-top: 5px; }

/* 通用行样式（存档/白名单/封禁） */
.item-row {
  display: flex; align-items: center; gap: 12px;
  padding: 12px; border-radius: 8px;
  background: var(--bg); margin-bottom: 8px;
}
.item-row:last-child { margin-bottom: 0; }
.item-row .item-main { flex: 1; min-width: 0; }
.item-row .item-title { font-weight: 600; font-size: 14px; }
.item-row .item-sub { font-size: 12px; color: var(--text-muted); margin-top: 2px; }
.item-row .item-actions { display: flex; gap: 6px; flex-shrink: 0; }
.item-row .item-tag {
  font-size: 11px; padding: 2px 8px; border-radius: 10px;
  background: var(--card-hover); color: var(--text-muted); white-space: nowrap;
}

.empty-state {
  text-align: center; color: var(--text-muted);
  padding: 28px; font-size: 14px;
}

.add-form {
  display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px;
}
.add-form input { flex: 1; min-width: 140px; }
.add-form .btn { flex-shrink: 0; }

.upload-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  margin-bottom: 12px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  background: var(--bg);
}
.upload-box .upload-title { font-size: 14px; font-weight: 600; }
.upload-box .upload-sub { font-size: 12px; color: var(--text-muted); margin-top: 2px; }
.upload-box input[type="file"] { display: none; }

/* 配置管理页 */
.config-layout {
  display: flex;
  gap: 20px;
  max-width: 1200px;
  margin: 24px auto;
  padding: 0 20px;
}
.config-sidebar {
  width: 200px;
  flex-shrink: 0;
  background: var(--card);
  border-radius: 12px;
  padding: 10px;
  height: fit-content;
  position: sticky;
  top: 80px;
  box-shadow: 0 2px 10px rgba(0,0,0,0.10);
}
.config-sidebar .cat-item {
  padding: 10px 14px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  color: var(--text-muted);
  transition: all 0.2s;
  margin-bottom: 2px;
}
.config-sidebar .cat-item:hover { background: var(--card-hover); color: var(--text); }
.config-sidebar .cat-item.active { background: var(--primary); color: #fff; font-weight: 600; }
.config-content { flex: 1; min-width: 0; }
.config-content .panel { margin-bottom: 20px; }
.field { margin-bottom: 18px; }
.field:last-child { margin-bottom: 0; }
.field label {
  display: block;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 6px;
  color: var(--text);
}
.field .desc { font-size: 12px; color: var(--text-muted); margin-top: 4px; }
.field input[type="number"] { max-width: 200px; }
/* toggle 开关 */
.toggle {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle .slider {
  position: absolute;
  cursor: pointer;
  inset: 0;
  background: var(--border);
  border-radius: 24px;
  transition: 0.3s;
}
.toggle .slider::before {
  content: "";
  position: absolute;
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background: #fff;
  border-radius: 50%;
  transition: 0.3s;
}
.toggle input:checked + .slider { background: var(--secondary); }
.toggle input:checked + .slider::before { transform: translateX(20px); }
/* multiselect checkbox 组 */
.checks { display: flex; gap: 16px; flex-wrap: wrap; padding-top: 4px; }
.checks label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 400;
  cursor: pointer;
  font-size: 14px;
}
.checks input[type="checkbox"] { width: 16px; height: 16px; accent-color: var(--primary); }
.config-footer {
  position: sticky;
  bottom: 0;
  background: var(--card);
  padding: 14px 20px;
  border-radius: 12px 12px 0 0;
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: flex-end;
  margin-top: 20px;
  box-shadow: 0 -2px 10px rgba(0,0,0,0.12);
  border-top: 1px solid var(--border);
}
.dirty-hint { color: var(--warning); font-size: 13px; margin-right: auto; }
@media (max-width: 768px) {
  .config-layout { flex-direction: column; padding: 0 12px; }
  .config-sidebar {
    width: 100%;
    position: static;
    display: flex;
    overflow-x: auto;
    gap: 4px;
    padding: 8px;
  }
  .config-sidebar .cat-item { white-space: nowrap; margin-bottom: 0; }
  .container { padding: 0 12px; }
  .cards { grid-template-columns: 1fr 1fr; gap: 10px; }
  .card .value { font-size: 18px; }
}
</style>
</head>
<body>
<div id="app"></div>
<div id="toast" class="toast"></div>
<script>
let csrf = "";
let timer = null;
let configData = null;
let currentCategory = null;
let configDirty = false;

function toast(msg, ok) {
  const t = document.getElementById("toast");
  t.textContent = msg;
  t.className = "toast show " + (ok ? "ok" : "err");
  setTimeout(() => t.className = "toast", 2800);
}

async function api(path, opts = {}) {
  const headers = opts.headers || {};
  if (csrf) headers["X-CSRF-Token"] = csrf;
  if (opts.body) headers["Content-Type"] = "application/json";
  const r = await fetch(path, {
    method: opts.method || "GET",
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    credentials: "same-origin",
  });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) {
    if (r.status === 401) { showLogin(); csrf = ""; throw new Error("未登录"); }
    throw new Error(data.error || "请求失败");
  }
  return data;
}

function showLogin() {
  document.getElementById("app").innerHTML = `
    <div class="login-wrap"><div class="login-card">
      <h1>Palworld 管理面板</h1>
      <form id="loginForm">
        <input type="password" id="pwd" placeholder="Web 密码" autofocus>
        <div class="error" id="loginErr"></div>
        <button type="submit" class="btn btn-restart" style="width:100%;margin-top:12px;padding:10px">登录</button>
      </form>
    </div></div>`;
  document.getElementById("loginForm").onsubmit = async (e) => {
    e.preventDefault();
    const pwd = document.getElementById("pwd").value;
    try {
      const d = await api("/api/login", { method: "POST", body: { password: pwd } });
      csrf = d.csrf;
      showDashboard();
    } catch (err) {
      document.getElementById("loginErr").textContent = err.message;
    }
  };
}

async function refreshStatus() {
  try {
    const [st, mem] = await Promise.all([api("/api/status"), api("/api/memory")]);
    const active = st.active;
    const card = document.getElementById("statusCard");
    const icon = document.getElementById("statusIcon");
    const title = document.getElementById("statusTitle");
    const sub = document.getElementById("statusSub");
    if (active) {
      card.classList.remove("inactive");
      icon.textContent = "✓";
      title.textContent = "运行中";
      sub.textContent = st.uptime ? "启动于 " + st.uptime : "运行中";
    } else {
      card.classList.add("inactive");
      icon.textContent = "✕";
      title.textContent = "已停止";
      sub.textContent = "点击下方「启动」按钮运行服务器";
    }

    const cur = mem.MemoryCurrent;
    const peak = mem.MemoryPeak;
    const max = mem.MemoryMax;
    document.getElementById("mem").textContent = cur != null ? (cur/1073741824).toFixed(2) + " GB" : "-";
    document.getElementById("memPeak").textContent = peak != null ? (peak/1073741824).toFixed(2) + " GB" : "-";

    if (max && max > 0 && cur != null) {
      const pct = Math.min(100, (cur / max) * 100);
      const bar = document.getElementById("memBar");
      bar.style.width = pct.toFixed(1) + "%";
      bar.classList.toggle("high", pct > 80);
      document.getElementById("memSub").textContent = pct.toFixed(0) + "% / " + (max/1073741824).toFixed(1) + " GB 上限";
    } else {
      document.getElementById("memSub").textContent = "未设内存上限";
    }

    document.getElementById("btnStart").disabled = active;
    document.getElementById("btnStop").disabled = !active;
    document.getElementById("btnRestart").disabled = !active;
    document.getElementById("btnSave").disabled = !active;
  } catch (e) { /* 忽略 */ }
}

async function refreshPlayers() {
  try {
    const d = await api("/api/players");
    const count = d.players.length;
    document.getElementById("playerCount").textContent = count;
    if (count === 0) {
      document.getElementById("playersList").innerHTML =
        '<div style="text-align:center;color:var(--text-muted);padding:24px">无在线玩家</div>';
      return;
    }
    const html = d.players.map(p => `
      <div class="player-row">
        <div class="player-dot"></div>
        <div class="player-name">${escapeHtml(p.name)}</div>
        <div class="player-meta">
          ${p.level != null ? `<span class="player-tag">Lv.${p.level}</span>` : ''}
          ${p.ping != null ? `<span class="player-tag">${p.ping}ms</span>` : ''}
        </div>
        <div class="player-steamid">${escapeHtml(p.steamid)}</div>
        <div class="player-actions">
          <button class="btn btn-kick" onclick="kick('${escapeAttr(p.steamid)}')">踢出</button>
          <button class="btn btn-ban" onclick="ban('${escapeAttr(p.steamid)}')">封禁</button>
        </div>
      </div>`).join("");
    document.getElementById("playersList").innerHTML = html;
  } catch (e) { /* 忽略 */ }
}

async function refreshLogs() {
  try {
    const d = await api("/api/logs");
    const raw = d.logs || "(无日志)";
    const html = raw.split('\n').map(line => {
      let cls = "";
      if (/error|fail|exception|crash/i.test(line)) cls = "log-error";
      else if (/warn/i.test(line)) cls = "log-warn";
      else if (/connect/i.test(line)) cls = "log-connect";
      else if (/join/i.test(line)) cls = "log-join";
      else if (/disconnect|leave|exit/i.test(line)) cls = "log-leave";
      return `<div class="log-line ${cls}">${escapeHtml(line)}</div>`;
    }).join("");
    const l = document.getElementById("logs");
    l.innerHTML = html;
    l.scrollTop = l.scrollHeight;
  } catch (e) { /* 忽略 */ }
}

function escapeHtml(s) {
  return String(s||"").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
}
function escapeAttr(s) { return String(s||"").replace(/'/g, "\\'"); }
function validBackupName(name) { return /^pal_backup_[A-Za-z0-9_.-]+\.tar\.gz$/.test(name); }

async function action(path, msg) {
  try {
    await api(path, { method: "POST", body: {} });
    toast(msg + " 成功", true);
    setTimeout(refreshStatus, 1500);
  } catch (e) { toast(msg + " 失败: " + e.message, false); }
}

async function kick(sid) {
  if (!confirm("踢出玩家 " + sid + " ?")) return;
  try { await api("/api/kick", { method: "POST", body: { steamid: sid } });
        toast("已踢出", true); refreshPlayers(); }
  catch (e) { toast(e.message, false); }
}
async function ban(sid) {
  if (!confirm("封禁玩家 " + sid + " ?")) return;
  try { await api("/api/ban", { method: "POST", body: { steamid: sid } });
        toast("已封禁", true); refreshPlayers(); }
  catch (e) { toast(e.message, false); }
}

function showDashboard() {
  if (timer) clearInterval(timer);
  document.getElementById("app").innerHTML = `
    <div class="topbar">
      <h1>🎮 Palworld <span>管理面板</span></h1>
      <div class="tabs">
        <button class="tab active" onclick="showDashboard()">📊 仪表盘</button>
        <button class="tab" onclick="showPlayers()">👥 玩家管理</button>
        <button class="tab" onclick="showConfig()">⚙️ 配置管理</button>
        <button class="tab" onclick="showSaves()">💾 存档管理</button>
      </div>
      <button class="btn btn-logout" onclick="logout()">退出</button>
    </div>
    <div class="container">
      <div class="status-card" id="statusCard">
        <div class="status-icon" id="statusIcon">·</div>
        <div class="status-text">
          <div class="title" id="statusTitle">加载中...</div>
          <div class="sub" id="statusSub">-</div>
        </div>
      </div>
      <div class="cards">
        <div class="sys-card">
          <div class="sys-head"><span>⚡</span>CPU</div>
          <div class="sys-value" id="cpuPercent">-</div>
          <div class="sys-bar"><div class="sys-bar-fill" id="cpuBar" style="width:0%"></div></div>
          <div class="sys-sub" id="cpuSub">系统 CPU 使用率</div>
        </div>
        <div class="sys-card">
          <div class="sys-head"><span>💿</span>磁盘</div>
          <div class="sys-value" id="diskPercent">-</div>
          <div class="sys-bar"><div class="sys-bar-fill" id="diskBar" style="width:0%"></div></div>
          <div class="sys-sub" id="diskSub">-</div>
        </div>
        <div class="sys-card">
          <div class="sys-head"><span>⏱️</span>系统运行</div>
          <div class="sys-value" id="sysUptime">-</div>
          <div class="sys-sub">服务器开机时长</div>
        </div>
      </div>
      <div class="cards">
        <div class="metric-card">
          <div class="metric-head"><span class="metric-icon">💾</span>当前内存</div>
          <div class="metric-value" id="mem">-</div>
          <div class="progress"><div class="progress-fill" id="memBar" style="width:0%"></div></div>
          <div class="metric-sub" id="memSub">-</div>
        </div>
        <div class="metric-card">
          <div class="metric-head"><span class="metric-icon">📈</span>峰值内存</div>
          <div class="metric-value" id="memPeak">-</div>
          <div class="metric-sub" style="margin-top:14px">历史最高占用</div>
        </div>
      </div>
      <div class="panel">
        <div class="panel-header"><h2>🎛️ 服务控制</h2></div>
        <div class="row">
          <button class="btn btn-start" id="btnStart" onclick="action('/api/start','启动')"><span class="btn-icon">▶</span>启动</button>
          <button class="btn btn-stop" id="btnStop" onclick="action('/api/stop','停止')"><span class="btn-icon">⏹</span>停止</button>
          <button class="btn btn-restart" id="btnRestart" onclick="action('/api/restart','重启')"><span class="btn-icon">🔄</span>重启</button>
          <button class="btn btn-save" id="btnSave" onclick="action('/api/save','保存存档')"><span class="btn-icon">💾</span>保存存档</button>
        </div>
      </div>
      <div class="panel">
        <div class="panel-header"><h2>📢 广播消息</h2></div>
        <div class="row">
          <input type="text" id="bcMsg" placeholder="输入广播内容，游戏内全服可见">
          <button class="btn btn-restart" onclick="broadcast()"><span class="btn-icon">📢</span>发送</button>
        </div>
      </div>
      <div class="panel">
        <div class="panel-header">
          <h2>📜 最近日志</h2>
          <span class="panel-sub">30 秒自动刷新</span>
        </div>
        <div class="logs" id="logs">加载中...</div>
      </div>
    </div>`;

  refreshStatus();
  refreshSysinfo();
  refreshLogs();
  if (timer) clearInterval(timer);
  timer = setInterval(() => { refreshStatus(); refreshSysinfo(); refreshLogs(); }, 30000);
}

async function refreshSysinfo() {
  try {
    const s = await api("/api/sysinfo");
    const cpu = s.cpu_percent || 0;
    document.getElementById("cpuPercent").textContent = cpu.toFixed(1) + "%";
    const cpuBar = document.getElementById("cpuBar");
    cpuBar.style.width = cpu + "%";
    cpuBar.classList.toggle("high", cpu > 80);

    const disk = s.disk || {};
    const dp = disk.percent || 0;
    document.getElementById("diskPercent").textContent = dp.toFixed(1) + "%";
    const diskBar = document.getElementById("diskBar");
    diskBar.style.width = dp + "%";
    diskBar.classList.toggle("high", dp > 90);
    document.getElementById("diskSub").textContent =
      ((disk.used||0)/1073741824).toFixed(1) + " / " + ((disk.total||0)/1073741824).toFixed(1) + " GB";

    const up = s.uptime || 0;
    const days = Math.floor(up / 86400);
    const hours = Math.floor((up % 86400) / 3600);
    const mins = Math.floor((up % 3600) / 60);
    document.getElementById("sysUptime").textContent =
      days > 0 ? `${days}天${hours}时` : `${hours}时${mins}分`;
  } catch (e) { /* 忽略 */ }
}

async function broadcast() {
  const msg = document.getElementById("bcMsg").value.trim();
  if (!msg) return;
  try {
    await api("/api/broadcast", { method: "POST", body: { message: msg } });
    document.getElementById("bcMsg").value = "";
    toast("广播已发送", true);
  } catch (e) { toast(e.message, false); }
}

async function showPlayers() {
  if (timer) clearInterval(timer);
  document.getElementById("app").innerHTML = `
    <div class="topbar">
      <h1>🎮 Palworld <span>玩家管理</span></h1>
      <div class="tabs">
        <button class="tab" onclick="showDashboard()">📊 仪表盘</button>
        <button class="tab active" onclick="showPlayers()">👥 玩家管理</button>
        <button class="tab" onclick="showConfig()">⚙️ 配置管理</button>
        <button class="tab" onclick="showSaves()">💾 存档管理</button>
      </div>
      <button class="btn btn-logout" onclick="logout()">退出</button>
    </div>
    <div class="container">
      <div class="panel">
        <div class="panel-header">
          <h2>👥 在线玩家</h2>
          <span class="count-badge" id="playerCount">0</span>
        </div>
        <div id="playersList"></div>
      </div>
      <div class="panel">
        <div class="panel-header">
          <h2>📝 白名单</h2>
          <span class="panel-sub">空字段=通配符 · 白名单为空=不启用</span>
        </div>
        <div class="add-form">
          <input type="text" id="wlName" placeholder="玩家名（可空）">
          <input type="text" id="wlSteamid" placeholder="SteamID（可空）">
          <input type="text" id="wlPlayeruid" placeholder="PlayerUID（可空）">
          <button class="btn btn-save" onclick="addWhitelist()"><span class="btn-icon">+</span>添加</button>
          <button class="btn btn-restart" onclick="checkWhitelist()">立即检查</button>
        </div>
        <div id="whitelistList"></div>
      </div>
      <div class="panel">
        <div class="panel-header"><h2>🚫 封禁列表</h2></div>
        <div id="banlistList"></div>
      </div>
    </div>`;

  refreshPlayers();
  refreshWhitelist();
  refreshBanlist();
  if (timer) clearInterval(timer);
  timer = setInterval(() => { refreshPlayers(); }, 30000);
}

async function refreshWhitelist() {
  try {
    const d = await api("/api/whitelist");
    const wl = d.whitelist || [];
    if (wl.length === 0) {
      document.getElementById("whitelistList").innerHTML =
        '<div class="empty-state">白名单为空（未启用）</div>';
      return;
    }
    const html = wl.map(w => `
      <div class="item-row">
        <div class="item-main">
          <div class="item-title">${escapeHtml(w.name || '(任意名称)')}</div>
          <div class="item-sub">SteamID: ${escapeHtml(w.steamid || '(任意)')} · PlayerUID: ${escapeHtml(w.playeruid || '(任意)')}</div>
        </div>
        <div class="item-actions">
          ${w.steamid ? `<button class="btn btn-ban" onclick="removeWhitelist('${escapeAttr(w.steamid)}')">移除</button>` : ''}
        </div>
      </div>`).join("");
    document.getElementById("whitelistList").innerHTML = html;
  } catch (e) { /* 忽略 */ }
}

async function addWhitelist() {
  const name = document.getElementById("wlName").value.trim();
  const steamid = document.getElementById("wlSteamid").value.trim();
  const playeruid = document.getElementById("wlPlayeruid").value.trim();
  if (!name && !steamid && !playeruid) {
    toast("至少填一个字段", false);
    return;
  }
  try {
    const d = await api("/api/whitelist/add", { method: "POST", body: { name, steamid, playeruid } });
    toast(d.message, true);
    document.getElementById("wlName").value = "";
    document.getElementById("wlSteamid").value = "";
    document.getElementById("wlPlayeruid").value = "";
    refreshWhitelist();
  } catch (e) { toast(e.message, false); }
}

async function removeWhitelist(steamid) {
  if (!confirm("移除白名单条目 " + steamid + " ?")) return;
  try {
    const d = await api("/api/whitelist/remove", { method: "POST", body: { steamid } });
    toast(d.message, true);
    refreshWhitelist();
  } catch (e) { toast(e.message, false); }
}

async function checkWhitelist() {
  try {
    const d = await api("/api/whitelist/check", { method: "POST", body: {} });
    toast(d.message, true);
    refreshPlayers();
  } catch (e) { toast(e.message, false); }
}

async function refreshBanlist() {
  try {
    const d = await api("/api/banlist");
    const bl = d.banlist || [];
    if (bl.length === 0) {
      document.getElementById("banlistList").innerHTML =
        '<div class="empty-state">无封禁玩家</div>';
      return;
    }
    const html = bl.map(b => `
      <div class="item-row">
        <div class="item-main">
          <div class="item-title">${escapeHtml(b.steamid)}</div>
          <div class="item-sub">SteamID</div>
        </div>
        <div class="item-actions">
          <button class="btn btn-save" onclick="doUnban('${escapeAttr(b.steamid)}')">解封</button>
        </div>
      </div>`).join("");
    document.getElementById("banlistList").innerHTML = html;
  } catch (e) { /* 忽略 */ }
}

async function doUnban(steamid) {
  if (!confirm("解封玩家 " + steamid + " ?")) return;
  try {
    const d = await api("/api/banlist/unban", { method: "POST", body: { steamid } });
    toast(d.message, true);
    refreshBanlist();
  } catch (e) { toast(e.message, false); }
}

async function showSaves() {
  if (timer) clearInterval(timer);
  document.getElementById("app").innerHTML = `
    <div class="topbar">
      <h1>🎮 Palworld <span>存档管理</span></h1>
      <div class="tabs">
        <button class="tab" onclick="showDashboard()">📊 仪表盘</button>
        <button class="tab" onclick="showPlayers()">👥 玩家管理</button>
        <button class="tab" onclick="showConfig()">⚙️ 配置管理</button>
        <button class="tab active" onclick="showSaves()">💾 存档管理</button>
      </div>
      <button class="btn btn-logout" onclick="logout()">退出</button>
    </div>
    <div class="container">
      <div class="panel">
        <div class="panel-header">
          <h2>💾 存档备份</h2>
          <button class="btn btn-save" onclick="doBackup()"><span class="btn-icon">💾</span>立即备份</button>
        </div>
        <div class="upload-box">
          <div>
            <div class="upload-title">上传备份</div>
            <div class="upload-sub">支持 pal_backup_*.tar.gz，上传后会出现在下方列表</div>
          </div>
          <label class="btn btn-upload" for="saveUpload"><span class="btn-icon">⬆</span>选择文件</label>
          <input type="file" id="saveUpload" accept=".tar.gz,application/gzip,application/x-gzip" onchange="uploadSave(this)">
        </div>
        <div id="savesList">加载中...</div>
      </div>
    </div>`;

  refreshSaves();
}

async function refreshSaves() {
  try {
    const d = await api("/api/saves");
    const saves = d.saves || [];
    if (saves.length === 0) {
      document.getElementById("savesList").innerHTML =
        '<div class="empty-state">暂无备份。点上方「立即备份」创建第一个</div>';
      return;
    }
    const html = saves.map(s => {
      const date = new Date(s.time * 1000);
      const dateStr = date.toLocaleString("zh-CN");
      const sizeMb = (s.size / 1048576).toFixed(2);
      return `
        <div class="item-row">
          <div class="item-main">
            <div class="item-title">${escapeHtml(s.name)}</div>
            <div class="item-sub">${dateStr} · ${sizeMb} MB</div>
          </div>
          <div class="item-actions">
            <button class="btn btn-download" onclick="downloadSave('${escapeAttr(s.name)}')">下载</button>
            <button class="btn btn-restart" onclick="doRestore('${escapeAttr(s.name)}')">恢复</button>
            <button class="btn btn-ban" onclick="doDelete('${escapeAttr(s.name)}')">删除</button>
          </div>
        </div>`;
    }).join("");
    document.getElementById("savesList").innerHTML = html;
  } catch (e) {
    document.getElementById("savesList").innerHTML =
      `<div class="empty-state">加载失败: ${escapeHtml(e.message)}</div>`;
  }
}

async function doBackup() {
  toast("正在备份，请稍候...", true);
  try {
    const d = await api("/api/saves/backup", { method: "POST", body: {} });
    toast(d.message, d.ok);
    if (d.ok) refreshSaves();
  } catch (e) { toast(e.message, false); }
}

function downloadSave(name) {
  window.location.href = "/api/saves/download?name=" + encodeURIComponent(name);
}

function uploadSave(input) {
  const file = input.files && input.files[0];
  if (!file) return;
  if (!validBackupName(file.name)) {
    toast("文件名必须是 pal_backup_*.tar.gz", false);
    input.value = "";
    return;
  }
  if (file.size > 500 * 1024 * 1024) {
    toast("文件过大（>500MB）", false);
    input.value = "";
    return;
  }
  const reader = new FileReader();
  reader.onload = async () => {
    const result = String(reader.result || "");
    const content = result.includes(",") ? result.split(",")[1] : result;
    try {
      toast("正在上传备份...", true);
      const d = await api("/api/saves/upload", { method: "POST", body: { name: file.name, content } });
      toast(d.message, d.ok);
      if (d.ok) refreshSaves();
    } catch (e) {
      toast(e.message, false);
    } finally {
      input.value = "";
    }
  };
  reader.onerror = () => {
    toast("读取文件失败", false);
    input.value = "";
  };
  reader.readAsDataURL(file);
}

async function doRestore(name) {
  if (!confirm("恢复备份 " + name + " ?\n\n当前存档会被覆盖，服务器将重启，在线玩家会被踢出。")) return;
  toast("正在恢复，服务器将重启...", true);
  try {
    const d = await api("/api/saves/restore", { method: "POST", body: { name } });
    toast(d.message, d.ok);
  } catch (e) { toast(e.message, false); }
}

async function doDelete(name) {
  if (!confirm("删除备份 " + name + " ? 此操作不可撤销。")) return;
  try {
    const d = await api("/api/saves/delete", { method: "POST", body: { name } });
    toast(d.message, d.ok);
    if (d.ok) refreshSaves();
  } catch (e) { toast(e.message, false); }
}

async function showConfig() {
  if (timer) clearInterval(timer);
  document.getElementById("app").innerHTML = `
    <div class="topbar">
      <h1>🎮 Palworld <span>配置管理</span></h1>
      <div class="tabs">
        <button class="tab" onclick="showDashboard()">📊 仪表盘</button>
        <button class="tab" onclick="showPlayers()">👥 玩家管理</button>
        <button class="tab active" onclick="showConfig()">⚙️ 配置管理</button>
        <button class="tab" onclick="showSaves()">💾 存档管理</button>
      </div>
      <button class="btn btn-logout" onclick="logout()">退出</button>
    </div>
    <div class="config-layout">
      <div class="config-sidebar" id="configSidebar">加载中...</div>
      <div class="config-content" id="configContent"></div>
    </div>
    <div class="config-footer">
      <span id="dirtyHint" class="dirty-hint" style="display:none">有未保存改动</span>
      <button class="btn btn-save" onclick="saveConfig(false)">保存</button>
      <button class="btn btn-save-restart" onclick="saveConfig(true)">保存并重启</button>
    </div>`;

  try {
    configData = await api("/api/config");
  } catch (e) {
    document.getElementById("configSidebar").innerHTML = "";
    document.getElementById("configContent").innerHTML =
      `<div class="panel"><h2>加载失败</h2><p>${escapeHtml(e.message)}</p></div>`;
    return;
  }
  if (configData.error) {
    document.getElementById("configSidebar").innerHTML = "";
    document.getElementById("configContent").innerHTML =
      `<div class="panel"><h2>配置不可用</h2><p>${escapeHtml(configData.error)}</p></div>`;
    return;
  }
  currentCategory = configData.categories[0].name;
  configDirty = false;
  renderConfigSidebar();
  renderConfigContent();
}

function renderConfigSidebar() {
  const html = configData.categories.map(c =>
    `<div class="cat-item ${c.name === currentCategory ? 'active' : ''}" onclick="switchCategory('${escapeAttr(c.name)}')">${escapeHtml(c.name)}</div>`
  ).join("");
  document.getElementById("configSidebar").innerHTML = html;
}

function switchCategory(name) {
  if (configDirty && !confirm("当前分类有未保存改动，切换将丢失。继续？")) return;
  currentCategory = name;
  configDirty = false;
  document.getElementById("dirtyHint").style.display = "none";
  renderConfigSidebar();
  renderConfigContent();
}

function renderConfigContent() {
  const cat = configData.categories.find(c => c.name === currentCategory);
  if (!cat) return;
  const html = `
    <div class="panel">
      <h2>${escapeHtml(cat.name)}</h2>
      ${cat.items.map(renderField).join("")}
    </div>`;
  document.getElementById("configContent").innerHTML = html;
  // 绑定改动事件
  document.querySelectorAll("#configContent input, #configContent select").forEach(el => {
    el.addEventListener("change", () => {
      configDirty = true;
      document.getElementById("dirtyHint").style.display = "inline";
    });
    el.addEventListener("input", () => {
      configDirty = true;
      document.getElementById("dirtyHint").style.display = "inline";
    });
  });
}

function renderField(item) {
  const desc = item.desc ? `<div class="desc">${escapeHtml(item.desc)}</div>` : "";
  let control = "";
  if (item.type === "text") {
    control = `<input type="text" data-key="${item.key}" value="${escapeAttr(String(item.value ?? ''))}">`;
  } else if (item.type === "password") {
    control = `<input type="password" data-key="${item.key}" value="${escapeAttr(String(item.value ?? ''))}">`;
  } else if (item.type === "number") {
    const step = item.step || 1;
    const min = item.min != null ? `min="${item.min}"` : "";
    const max = item.max != null ? `max="${item.max}"` : "";
    control = `<input type="number" data-key="${item.key}" value="${item.value ?? 0}" step="${step}" ${min} ${max}>`;
  } else if (item.type === "bool") {
    const checked = item.value ? "checked" : "";
    control = `<label class="toggle"><input type="checkbox" data-key="${item.key}" ${checked}><span class="slider"></span></label>`;
  } else if (item.type === "select") {
    const opts = item.options.map(o =>
      `<option value="${escapeAttr(o)}" ${o === item.value ? "selected" : ""}>${escapeHtml(o)}</option>`
    ).join("");
    control = `<select data-key="${item.key}">${opts}</select>`;
  } else if (item.type === "multiselect") {
    const cur = Array.isArray(item.value) ? item.value : [];
    const opts = item.options.map(o =>
      `<label><input type="checkbox" data-key="${item.key}" data-multiselect="1" value="${escapeAttr(o)}" ${cur.includes(o) ? "checked" : ""}> ${escapeHtml(o)}</label>`
    ).join("");
    control = `<div class="checks">${opts}</div>`;
  }
  return `<div class="field"><label>${escapeHtml(item.label)}</label>${control}${desc}</div>`;
}

function collectForm() {
  const data = {};
  const seen = new Set();
  document.querySelectorAll("#configContent [data-key]").forEach(el => {
    const key = el.dataset.key;
    if (el.dataset.multiselect) {
      if (!seen.has(key)) { data[key] = []; seen.add(key); }
      if (el.checked) data[key].push(el.value);
    } else if (el.type === "checkbox") {
      data[key] = el.checked;
    } else {
      data[key] = el.value;
    }
  });
  return data;
}

async function saveConfig(restart) {
  const body = collectForm();
  try {
    const r = await api("/api/config", { method: "POST", body });
    toast(restart ? "已保存，正在重启..." : "已保存，需重启生效", true);
    configDirty = false;
    document.getElementById("dirtyHint").style.display = "none";
    if (restart) {
      try {
        await api("/api/config/restart", { method: "POST", body: {} });
        toast("重启指令已发送", true);
      } catch (e) { toast("重启失败: " + e.message, false); }
    }
  } catch (e) { toast("保存失败: " + e.message, false); }
}

async function logout() {
  try { await api("/api/logout", { method: "POST", body: {} }); } catch (e) {}
  csrf = "";
  if (timer) clearInterval(timer);
  showLogin();
}

// 启动：先尝试访问 /api/status，401 则跳登录
(async () => {
  try {
    await api("/api/status");
    showDashboard();
  } catch (e) {
    showLogin();
  }
})();
</script>
</body>
</html>
"""


if __name__ == "__main__":
    print(f"Palworld Web UI 启动: {BIND}:{PORT}")
    httpd = ThreadingHTTPServer((BIND, PORT), Handler)
    httpd.serve_forever()
