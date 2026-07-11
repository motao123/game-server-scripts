package app

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
)

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"instances": s.instances.List()})
}

func (s *Server) safeRoot(path string) bool {
	clean := filepath.Clean(path)
	roots := []string{s.defaultFileRoot(), s.cfg.PalServerDir, s.cfg.BackupDir, filepath.Dir(s.cfg.PalSettings), s.cfg.DataDir}
	for _, root := range roots {
		root = filepath.Clean(root)
		if clean == root || strings.HasPrefix(clean, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func (s *Server) defaultFileRoot() string {
	if st, err := os.Stat(s.cfg.PalServerDir); err == nil && st.IsDir() {
		return s.cfg.PalServerDir
	}
	if st, err := os.Stat("/root"); err == nil && st.IsDir() {
		return "/root"
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = s.defaultFileRoot()
	}
	if !s.safeRoot(path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	items := []map[string]any{}
	for _, e := range entries {
		info, _ := e.Info()
		items = append(items, map[string]any{"name": e.Name(), "path": filepath.Join(path, e.Name()), "isDir": e.IsDir(), "size": info.Size(), "modTime": info.ModTime()})
	}
	writeJSON(w, map[string]any{"path": path, "items": items})
}

func (s *Server) handleFilesRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !s.safeRoot(path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	enc := detectEncoding(data)
	writeJSON(w, map[string]any{"path": path, "content": string(data), "encoding": enc})
}

func (s *Server) handleFilesWrite(w http.ResponseWriter, r *http.Request) {
	var body struct{ Path, Content string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	if err := os.WriteFile(body.Path, []byte(body.Content), 0644); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTerminalSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"sessions": s.terminal.List()})
}
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"tasks": s.tasks.List()})
}
func (s *Server) handleEnvironment(w http.ResponseWriter, r *http.Request) {
	java, _ := exec.LookPath("java")
	steamcmd := lookPath("steamcmd")
	if steamcmd == "" {
		if _, err := os.Stat("/usr/local/steamcmd/steamcmd.sh"); err == nil {
			steamcmd = "/usr/local/steamcmd/steamcmd.sh"
		}
	}
	writeJSON(w, map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "java": java, "steamcmd": steamcmd})
}
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"plugins": s.scanPlugins(), "enabled": true})
}
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"bind":           s.cfg.Bind,
		"port":           s.cfg.Port,
		"dataDir":        s.cfg.DataDir,
		"palServerDir":   s.cfg.PalServerDir,
		"backupDir":      s.cfg.BackupDir,
		"whitelistFile":  s.cfg.WhitelistFile,
		"rconPort":       s.cfg.RCONPort,
		"restApiPort":    s.cfg.RESTAPIPort,
		"service":        s.cfg.Service,
		"fileRoots":      []string{s.defaultFileRoot(), s.cfg.PalServerDir, s.cfg.BackupDir, s.cfg.DataDir},
		"securityNotice": "公网暴露请使用反代 HTTPS，并设置强密码；终端和文件管理仅限管理员",
	})
}
func (s *Server) handleRCONCommand(w http.ResponseWriter, r *http.Request) {
	var body struct{ Command string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	out, err := s.palworld.RCON(body.Command)
	writeJSON(w, map[string]any{"ok": err == nil, "output": out, "error": errString(err)})
}
func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	out, _ := exec.Command("ps", "-eo", "pid,comm,%cpu,%mem", "--sort=-%cpu").Output()
	writeJSON(w, map[string]any{"raw": string(out)})
}
func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request) {
	out, _ := exec.Command("sh", "-c", "ss -tuln 2>/dev/null || netstat -tuln 2>/dev/null").Output()
	writeJSON(w, map[string]any{"raw": string(out)})
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.WriteJSON(map[string]any{"type": "connected"})
	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg["type"] {
		case "terminal-start", "create-pty":
			shell, _ := msg["shell"].(string)
			cwd, _ := msg["cwd"].(string)
			if cwd == "" {
				cwd, _ = msg["workingDirectory"].(string)
			}
			cols := uint16(numberValue(msg["cols"], 100))
			rows := uint16(numberValue(msg["rows"], 30))
			id, err := s.terminal.Start(conn, shell, cwd, cols, rows)
			if err != nil {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-error", "error": err.Error()})
			} else {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-started", "sessionId": id})
			}
		case "terminal-input":
			id, _ := msg["sessionId"].(string)
			data, _ := msg["data"].(string)
			if err := s.terminal.Write(id, data); err != nil {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-error", "sessionId": id, "error": err.Error()})
			}
		case "terminal-resize":
			id, _ := msg["sessionId"].(string)
			_ = s.terminal.Resize(id, uint16(numberValue(msg["cols"], 100)), uint16(numberValue(msg["rows"], 30)))
		case "terminal-close", "close-pty":
			id, _ := msg["sessionId"].(string)
			s.terminal.Close(id)
			_ = conn.WriteJSON(map[string]any{"type": "terminal-closed", "sessionId": id})
		case "reconnect-session":
			id, _ := msg["sessionId"].(string)
			if !s.terminal.Reconnect(conn, id) {
				_ = conn.WriteJSON(map[string]any{"type": "session-reconnect-failed", "sessionId": id})
			}
		case "system-stats":
			_ = conn.WriteJSON(map[string]any{"type": "system-stats", "data": map[string]any{"ok": true}})
		default:
			_ = conn.WriteJSON(map[string]any{"type": "echo", "data": msg})
		}
	}
}

func numberValue(v any, fallback float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return fallback
	}
}
