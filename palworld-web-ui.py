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
import secrets
from http.cookies import SimpleCookie

# ===== 配置（从环境变量读取） =====
WEB_PASSWORD = os.environ.get("WEB_PASSWORD", "")
RCON_PORT = int(os.environ.get("RCON_PORT", "25575"))
RCON_PASS = os.environ.get("RCON_PASS", "")
SERVICE = os.environ.get("SERVICE", "pal-server")
BIND = os.environ.get("WEB_BIND", "0.0.0.0")
PORT = int(os.environ.get("WEB_PORT", "8080"))

# ===== 常量 =====
SESSION_TTL = 7200
RATE_LIMIT_WINDOW = 60
RATE_LIMIT_MAX = 5
CSRF_TTL = 7200

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
         "--property=MemoryCurrent", "--property=MemoryPeak"],
        capture_output=True, text=True,
    )
    result = {}
    for line in r.stdout.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            try:
                result[k] = int(v)
            except ValueError:
                pass
    return result


def get_players():
    rc, out, _ = rcon("ShowPlayers")
    if rc != 0:
        return []
    players = []
    for line in out.splitlines()[1:]:
        if not line.strip():
            continue
        parts = line.split(",")
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
    server_version = "Palweb/1.0"

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
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, "Segoe UI", Roboto, "Microsoft YaHei", sans-serif;
       background: #1a1a2e; color: #e0e0e0; }
.topbar { background: #16213e; padding: 12px 20px; display: flex;
          align-items: center; justify-content: space-between; }
.topbar h1 { font-size: 18px; color: #0f3460; }
.topbar h1 span { color: #e94560; }
.badge { padding: 4px 10px; border-radius: 12px; font-size: 12px; font-weight: bold; }
.badge.active { background: #27ae60; color: #fff; }
.badge.inactive { background: #c0392b; color: #fff; }
.btn { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer;
       font-size: 14px; color: #fff; transition: opacity .2s; }
.btn:hover { opacity: .85; }
.btn:disabled { opacity: .4; cursor: not-allowed; }
.btn-start { background: #27ae60; }
.btn-stop { background: #e67e22; }
.btn-restart { background: #2980b9; }
.btn-save { background: #8e44ad; }
.btn-kick { background: #f39c12; padding: 4px 10px; font-size: 12px; }
.btn-ban { background: #c0392b; padding: 4px 10px; font-size: 12px; }
.btn-logout { background: #555; }
.container { max-width: 1100px; margin: 20px auto; padding: 0 15px; }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
         gap: 15px; margin-bottom: 20px; }
.card { background: #16213e; padding: 15px; border-radius: 8px;
        border-left: 4px solid #e94560; }
.card .label { font-size: 12px; color: #888; margin-bottom: 6px; }
.card .value { font-size: 20px; font-weight: bold; }
.panel { background: #16213e; padding: 18px; border-radius: 8px; margin-bottom: 18px; }
.panel h2 { font-size: 15px; color: #e94560; margin-bottom: 12px;
            border-bottom: 1px solid #0f3460; padding-bottom: 8px; }
.row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
input[type="text"], input[type="password"] {
  background: #1a1a2e; border: 1px solid #0f3460; color: #e0e0e0;
  padding: 8px 10px; border-radius: 4px; font-size: 14px; flex: 1; min-width: 200px;
}
input:focus { outline: none; border-color: #e94560; }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 8px 10px; text-align: left; border-bottom: 1px solid #0f3460; }
th { font-size: 12px; color: #888; text-transform: uppercase; }
.logs { background: #0d0d1a; padding: 12px; border-radius: 4px;
        font-family: "Cascadia Code", Consolas, monospace; font-size: 12px;
        max-height: 400px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; }
.login-wrap { display: flex; justify-content: center; padding-top: 80px; }
.login-card { background: #16213e; padding: 30px; border-radius: 8px;
              width: 100%; max-width: 360px; }
.login-card h1 { text-align: center; margin-bottom: 20px; color: #e94560; }
.login-card .error { color: #e94560; font-size: 13px; margin-top: 10px; text-align: center; }
.toast { position: fixed; bottom: 20px; right: 20px; padding: 12px 18px;
         border-radius: 6px; color: #fff; font-size: 14px; opacity: 0;
         transition: opacity .3s; z-index: 999; }
.toast.show { opacity: 1; }
.toast.ok { background: #27ae60; }
.toast.err { background: #c0392b; }
</style>
</head>
<body>
<div id="app"></div>
<div id="toast" class="toast"></div>
<script>
let csrf = "";
let timer = null;

function toast(msg, ok) {
  const t = document.getElementById("toast");
  t.textContent = msg;
  t.className = "toast show " + (ok ? "ok" : "err");
  setTimeout(() => t.className = "toast", 2500);
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
        <button type="submit" class="btn btn-restart" style="width:100%;margin-top:10px">登录</button>
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
    const badge = `<span class="badge ${active ? "active" : "inactive"}">${active ? "运行中" : "已停止"}</span>`;
    document.getElementById("statusBadge").innerHTML = badge;
    document.getElementById("uptime").textContent = st.uptime || "-";
    const cur = mem.MemoryCurrent;
    const peak = mem.MemoryPeak;
    document.getElementById("mem").textContent = cur != null ? (cur/1073741824).toFixed(2) + " GB" : "-";
    document.getElementById("memPeak").textContent = peak != null ? (peak/1073741824).toFixed(2) + " GB" : "-";
    document.getElementById("btnStart").disabled = active;
    document.getElementById("btnStop").disabled = !active;
    document.getElementById("btnRestart").disabled = !active;
    document.getElementById("btnSave").disabled = !active;
  } catch (e) { /* 忽略 */ }
}

async function refreshPlayers() {
  try {
    const d = await api("/api/players");
    const rows = d.players.map(p => `
      <tr>
        <td>${escapeHtml(p.name)}</td>
        <td>${escapeHtml(p.steamid)}</td>
        <td>
          <button class="btn btn-kick" onclick="kick('${escapeAttr(p.steamid)}')">踢出</button>
          <button class="btn btn-ban" onclick="ban('${escapeAttr(p.steamid)}')">封禁</button>
        </td>
      </tr>`).join("");
    document.getElementById("playersBody").innerHTML = rows || `<tr><td colspan="3" style="text-align:center;color:#888">无在线玩家</td></tr>`;
  } catch (e) { /* 忽略 */ }
}

async function refreshLogs() {
  try {
    const d = await api("/api/logs");
    document.getElementById("logs").textContent = d.logs || "(无日志)";
    const l = document.getElementById("logs");
    l.scrollTop = l.scrollHeight;
  } catch (e) { /* 忽略 */ }
}

function escapeHtml(s) {
  return String(s||"").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
}
function escapeAttr(s) { return String(s||"").replace(/'/g, "\\'"); }

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
  document.getElementById("app").innerHTML = `
    <div class="topbar">
      <h1>Palworld <span>管理面板</span></h1>
      <div class="row">
        <span id="statusBadge"></span>
        <button class="btn btn-logout" onclick="logout()">退出</button>
      </div>
    </div>
    <div class="container">
      <div class="cards">
        <div class="card"><div class="label">服务状态</div><div class="value" id="statusBadge2"></div></div>
        <div class="card"><div class="label">启动时间</div><div class="value" id="uptime" style="font-size:14px">-</div></div>
        <div class="card"><div class="label">当前内存</div><div class="value" id="mem">-</div></div>
        <div class="card"><div class="label">峰值内存</div><div class="value" id="memPeak">-</div></div>
      </div>
      <div class="panel">
        <h2>服务控制</h2>
        <div class="row">
          <button class="btn btn-start" id="btnStart" onclick="action('/api/start','启动')">启动</button>
          <button class="btn btn-stop" id="btnStop" onclick="action('/api/stop','停止')">停止</button>
          <button class="btn btn-restart" id="btnRestart" onclick="action('/api/restart','重启')">重启</button>
          <button class="btn btn-save" id="btnSave" onclick="action('/api/save','保存')">保存存档</button>
        </div>
      </div>
      <div class="panel">
        <h2>广播消息</h2>
        <div class="row">
          <input type="text" id="bcMsg" placeholder="输入广播内容">
          <button class="btn btn-restart" onclick="broadcast()">发送</button>
        </div>
      </div>
      <div class="panel">
        <h2>在线玩家</h2>
        <table>
          <thead><tr><th>玩家名</th><th>SteamID</th><th>操作</th></tr></thead>
          <tbody id="playersBody"></tbody>
        </table>
      </div>
      <div class="panel">
        <h2>最近日志 <span style="font-size:11px;color:#888">(30 秒自动刷新)</span></h2>
        <div class="logs" id="logs">加载中...</div>
      </div>
    </div>`;

  // 修正：把 statusBadge2 也填上
  const orig = refreshStatus;
  refreshStatus = async function() {
    await orig.call(this);
    document.getElementById("statusBadge2").innerHTML = document.getElementById("statusBadge").innerHTML;
  };

  refreshStatus();
  refreshPlayers();
  refreshLogs();
  if (timer) clearInterval(timer);
  timer = setInterval(() => { refreshStatus(); refreshPlayers(); refreshLogs(); }, 30000);
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
