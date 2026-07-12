package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-server-scripts/internal/auth"
	"game-server-scripts/internal/config"
	"game-server-scripts/internal/palworld"
	"game-server-scripts/internal/system"
	"game-server-scripts/internal/terminal"
)

type Server struct {
	cfg       config.Config
	auth      *auth.Manager
	palworld  palworld.Service
	httpSrv   *http.Server
	instances *InstanceStore
	tasks     *TaskStore
	terminal  *terminal.Manager
	runtime   *InstanceRuntime
	scheduler *Scheduler
	installs  *InstallManager
	deploys   *DeployManager
	fileTasks *FileTaskManager
	monitor   *Monitor
	alerts    *AlertStore
	plugins   *PluginManager
	backups   *BackupManager
}

func NewServer(cfg config.Config) (*Server, error) {
	s := &Server{
		cfg:       cfg,
		auth:      auth.NewManager(cfg.WebPassword),
		palworld:  palworld.Service{Config: cfg},
		instances: NewInstanceStore(filepath.Join(cfg.DataDir, "instances.json")),
		tasks:     NewTaskStore(filepath.Join(cfg.DataDir, "scheduled_tasks.json")),
		terminal:  terminal.NewManager(),
	}
	s.runtime = NewInstanceRuntime(s.instances)
	s.scheduler = NewScheduler(s)
	s.installs = NewInstallManager()
	s.deploys = NewDeployManager()
	s.fileTasks = NewFileTaskManager()
	s.monitor = NewMonitor()
	s.alerts = NewAlertStore(filepath.Join(cfg.DataDir, "alert_rules.json"))
	s.plugins = NewPluginManager(cfg.DataDir)
	s.backups = NewBackupManager(cfg.BackupDir, s.safeRoot)
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	s.routes(mux)
	addr := fmt.Sprintf("%s:%d", s.cfg.Bind, s.cfg.Port)
	s.httpSrv = &http.Server{Addr: addr, Handler: logRequests(mux)}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("GSM Panel listening on %s", addr)
	s.scheduler.Start()
	go s.AutoStartInstances()
	defer s.scheduler.Stop()
	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/auth/verify", s.require(s.handleVerify))
	mux.HandleFunc("/api/auth/logout", s.require(s.handleLogout))
	mux.HandleFunc("/api/logout", s.require(s.handleLogout))
	mux.HandleFunc("/api/csrf", s.require(s.handleCSRF))

	mux.HandleFunc("/api/system/info", s.require(s.handleSystemInfo))
	mux.HandleFunc("/api/system/history", s.require(s.handleSystemHistory))
	mux.HandleFunc("/api/system/processes", s.require(s.handleProcesses))
	mux.HandleFunc("/api/system/ports", s.require(s.handlePorts))
	mux.HandleFunc("/api/network/check", s.require(s.handleNetworkCheck))
	mux.HandleFunc("/api/alerts/rules", s.require(s.handleAlertRules))
	mux.HandleFunc("/api/alerts/status", s.require(s.handleAlertStatus))
	mux.HandleFunc("/api/sysinfo", s.require(s.handleSystemInfo))

	mux.HandleFunc("/api/status", s.require(s.handlePalStatus))
	mux.HandleFunc("/api/memory", s.require(s.handleMemory))
	mux.HandleFunc("/api/logs", s.require(s.handleLogs))
	mux.HandleFunc("/api/players", s.require(s.handlePlayers))
	mux.HandleFunc("/api/start", s.requirePost(s.handleServiceAction("start")))
	mux.HandleFunc("/api/stop", s.requirePost(s.handleServiceAction("stop")))
	mux.HandleFunc("/api/restart", s.requirePost(s.handleServiceAction("restart")))
	mux.HandleFunc("/api/save", s.requirePost(s.handleRCON("Save")))
	mux.HandleFunc("/api/broadcast", s.requirePost(s.handleBroadcast))
	mux.HandleFunc("/api/saves", s.require(s.handleSaves))
	mux.HandleFunc("/api/saves/download", s.require(s.handleSaveDownload))
	mux.HandleFunc("/api/saves/backup", s.requirePost(s.handleSaveBackup))
	mux.HandleFunc("/api/saves/restore", s.requirePost(s.handleSaveRestore))
	mux.HandleFunc("/api/saves/upload", s.requirePost(s.handleSaveUpload))
	mux.HandleFunc("/api/saves/delete", s.requirePost(s.handleSaveDelete))
	mux.HandleFunc("/api/config", s.require(s.handleConfig))
	mux.HandleFunc("/api/config/restart", s.requirePost(s.handleServiceAction("restart")))
	mux.HandleFunc("/api/kick", s.requirePost(s.handlePlayerAction("KickPlayer")))
	mux.HandleFunc("/api/ban", s.requirePost(s.handlePlayerAction("BanPlayer")))
	mux.HandleFunc("/api/unban", s.requirePost(s.handlePlayerAction("UnBanPlayer")))
	mux.HandleFunc("/api/whitelist", s.require(s.handleWhitelist))
	mux.HandleFunc("/api/whitelist/add", s.requirePost(s.handleWhitelistAdd))
	mux.HandleFunc("/api/whitelist/remove", s.requirePost(s.handleWhitelistRemove))
	mux.HandleFunc("/api/whitelist/check", s.requirePost(s.handleWhitelistCheck))
	mux.HandleFunc("/api/banlist", s.require(s.handleBanlist))
	mux.HandleFunc("/api/banlist/unban", s.requirePost(s.handleBanlistUnban))

	mux.HandleFunc("/api/instances", s.require(s.handleInstances))
	mux.HandleFunc("/api/instances/create", s.requirePost(s.handleInstanceCreate))
	mux.HandleFunc("/api/instances/update", s.requirePost(s.handleInstanceUpdate))
	mux.HandleFunc("/api/instances/delete", s.requirePost(s.handleInstanceDelete))
	mux.HandleFunc("/api/instances/start", s.requirePost(s.handleInstanceAction("start")))
	mux.HandleFunc("/api/instances/stop", s.requirePost(s.handleInstanceAction("stop")))
	mux.HandleFunc("/api/instances/restart", s.requirePost(s.handleInstanceAction("restart")))
	mux.HandleFunc("/api/instances/input", s.requirePost(s.handleInstanceInput))
	mux.HandleFunc("/api/instances/status", s.require(s.handleInstanceStatus))
	mux.HandleFunc("/api/instances/logs", s.require(s.handleInstanceLogs))
	mux.HandleFunc("/api/instances/readiness", s.require(s.handleInstanceReadiness))
	mux.HandleFunc("/api/game-config/templates", s.require(s.handleGameConfigTemplates))
	mux.HandleFunc("/api/game-config/read", s.require(s.handleGameConfigRead))
	mux.HandleFunc("/api/game-config/save", s.requirePost(s.handleGameConfigSave))
	mux.HandleFunc("/api/games", s.require(s.handleGames))
	mux.HandleFunc("/api/catalogs", s.require(s.handleCatalogs))
	mux.HandleFunc("/api/catalogs/reload", s.requirePost(s.handleCatalogReload))
	mux.HandleFunc("/api/catalogs/update", s.requirePost(s.handleCatalogUpdate))
	mux.HandleFunc("/api/online-templates", s.require(s.handleOnlineTemplates))
	mux.HandleFunc("/api/online-templates/deploy", s.requirePost(s.handleOnlineTemplateDeploy))
	mux.HandleFunc("/api/game-deployment/status", s.require(s.handleDeploymentStatus))
	mux.HandleFunc("/api/game-deployment/install", s.requirePost(s.handleDeploymentInstall))
	mux.HandleFunc("/api/steamcmd/status", s.require(s.handleSteamcmdStatus))
	mux.HandleFunc("/api/files/list", s.require(s.handleFilesList))
	mux.HandleFunc("/api/files/read", s.require(s.handleFilesRead))
	mux.HandleFunc("/api/files/write", s.requirePost(s.handleFilesWrite))
	mux.HandleFunc("/api/files/download", s.require(s.handleFilesDownload))
	mux.HandleFunc("/api/files/upload", s.require(s.handleFilesUpload))
	mux.HandleFunc("/api/files/upload-chunk", s.requirePost(s.handleFilesUploadChunk))
	mux.HandleFunc("/api/files/upload-complete", s.requirePost(s.handleFilesUploadComplete))
	mux.HandleFunc("/api/files/tasks", s.require(s.handleFileTasks))
	mux.HandleFunc("/api/files/delete", s.requirePost(s.handleFilesDelete))
	mux.HandleFunc("/api/files/mkdir", s.requirePost(s.handleFilesMkdir))
	mux.HandleFunc("/api/files/compress", s.requirePost(s.handleFilesCompress))
	mux.HandleFunc("/api/files/extract", s.requirePost(s.handleFilesExtract))
	mux.HandleFunc("/api/files/rename", s.requirePost(s.handleFilesRename))
	mux.HandleFunc("/api/files/copy", s.requirePost(s.handleFilesCopy))
	mux.HandleFunc("/api/files/move", s.requirePost(s.handleFilesMove))
	mux.HandleFunc("/api/files/permissions", s.require(s.handleFilesPermissions))
	mux.HandleFunc("/api/files/favorites", s.require(s.handleFavorites))
	mux.HandleFunc("/api/terminal/sessions", s.require(s.handleTerminalSessions))
	mux.HandleFunc("/api/scheduled-tasks", s.require(s.handleTasks))
	mux.HandleFunc("/api/scheduled-tasks/create", s.requirePost(s.handleTaskCreate))
	mux.HandleFunc("/api/scheduled-tasks/toggle", s.requirePost(s.handleTaskToggle))
	mux.HandleFunc("/api/scheduled-tasks/run", s.requirePost(s.handleTaskRun))
	mux.HandleFunc("/api/scheduled-tasks/delete", s.requirePost(s.handleTaskDelete))
	mux.HandleFunc("/api/environment/info", s.require(s.handleEnvironment))
	mux.HandleFunc("/api/environment/install", s.requirePost(s.handleEnvironmentInstall))
	mux.HandleFunc("/api/environment/install/status", s.require(s.handleEnvironmentInstallStatus))
	mux.HandleFunc("/api/plugins", s.require(s.handlePlugins))
	mux.HandleFunc("/api/plugins/catalog", s.require(s.handlePluginCatalog))
	mux.HandleFunc("/api/plugins/create", s.requirePost(s.handlePluginCreate))
	mux.HandleFunc("/api/plugins/install", s.requirePost(s.handlePluginInstall))
	mux.HandleFunc("/api/plugins/upgrade", s.requirePost(s.handlePluginUpgrade))
	mux.HandleFunc("/api/plugins/audit", s.require(s.handlePluginAudit))
	mux.HandleFunc("/api/plugins/config", s.require(s.handlePluginConfig))
	mux.HandleFunc("/api/plugins/delete", s.requirePost(s.handlePluginDelete))
	mux.HandleFunc("/api/plugins/toggle", s.requirePost(s.handlePluginToggle))
	mux.HandleFunc("/api/settings", s.require(s.handleSettings))
	mux.HandleFunc("/api/settings/password", s.requirePost(s.handleChangePassword))
	mux.HandleFunc("/api/backup", s.require(s.handleBackupList))
	mux.HandleFunc("/api/backup/create", s.requirePost(s.handleBackupCreate))
	mux.HandleFunc("/api/backup/groups", s.require(s.handleBackupGroups))
	mux.HandleFunc("/api/backup/create-generic", s.requirePost(s.handleBackupCreateGeneric))
	mux.HandleFunc("/api/backup/restore-generic", s.requirePost(s.handleBackupRestoreGeneric))
	mux.HandleFunc("/api/backup/delete-file", s.requirePost(s.handleBackupDeleteFile))
	mux.HandleFunc("/api/backup/download", s.require(s.handleBackupDownload))
	mux.HandleFunc("/api/rcon/command", s.requirePost(s.handleRCONCommand))
	mux.HandleFunc("/api/rcon/config", s.require(s.handleRconConfig))
	mux.HandleFunc("/api/rcon/config/save", s.requirePost(s.handleRconConfigSave))
	mux.HandleFunc("/api/rcon/connect", s.requirePost(s.handleRconConnect))
	mux.HandleFunc("/api/rcon/disconnect", s.requirePost(s.handleRconDisconnect))
	mux.HandleFunc("/api/rcon/status", s.require(s.handleRconStatus))
	mux.HandleFunc("/api/rcon/command-instance", s.requirePost(s.handleRconCommandInstance))
	mux.HandleFunc("/ws", s.require(s.handleWebSocket))

	mux.HandleFunc("/", s.handleFrontend)
}

func (s *Server) require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.auth.RequestSession(r); !ok {
			writeError(w, http.StatusUnauthorized, "未登录")
			return
		}
		next(w, r)
	}
}

func (s *Server) requirePost(next http.HandlerFunc) http.HandlerFunc {
	return s.require(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !s.auth.RequireCSRF(r) {
			writeError(w, http.StatusForbidden, "CSRF token 无效")
			return
		}
		next(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	sess, ok, limited := s.auth.Login(clientIP(r), body.Password)
	if limited {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请 1 分钟后再试")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	auth.SetSessionCookie(w, sess)
	writeJSON(w, map[string]any{"ok": true, "token": sess.Token, "csrf": sess.CSRF})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true})
}
func (s *Server) handleCSRF(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.auth.RequestSession(r)
	writeJSON(w, map[string]any{"token": sess.CSRF})
}
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		s.auth.Logout(c.Value)
	}
	auth.ClearSessionCookie(w)
	writeJSON(w, map[string]any{"ok": true})
}
func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, system.Snapshot())
}
func (s *Server) handleSystemHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"points": s.monitor.History()})
}
func (s *Server) handleNetworkCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"checks": CheckNetworkTargets()})
}
func (s *Server) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"rules": s.alerts.List()})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.auth.RequireCSRF(r) {
		writeError(w, http.StatusForbidden, "CSRF token 无效")
		return
	}
	var body struct {
		Rules []AlertRule `json:"rules"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.alerts.Replace(body.Rules); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "rules": s.alerts.List()})
}
func (s *Server) handleAlertStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"alerts": s.alerts.Evaluate(system.Snapshot(), CheckNetworkTargets())})
}
func (s *Server) handlePalStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.palworld.Status())
}
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"system": system.Snapshot().Memory})
}
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"logs": s.palworld.Logs()})
}
func (s *Server) handlePlayers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"players": s.palworld.Players()})
}
func (s *Server) handleServiceAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := s.palworld.Systemctl(action)
		writeJSON(w, map[string]any{"ok": err == nil, "output": out, "error": errString(err)})
	}
}
func (s *Server) handleRCON(cmd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := s.palworld.RCON(cmd)
		writeJSON(w, map[string]any{"ok": err == nil, "output": out, "error": errString(err)})
	}
}

func (s *Server) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	out, err := s.palworld.RCON("Broadcast " + strings.TrimSpace(body.Message))
	writeJSON(w, map[string]any{"ok": err == nil, "output": out, "error": errString(err)})
}

func (s *Server) handleSaves(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"saves": s.palworld.ListSaves()})
}
func (s *Server) handleSaveDownload(w http.ResponseWriter, r *http.Request) {
	path, err := s.palworld.SavePath(r.URL.Query().Get("name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}
func (s *Server) handleSaveBackup(w http.ResponseWriter, r *http.Request) {
	ok, msg := s.palworld.Backup()
	writeJSON(w, map[string]any{"ok": ok, "message": msg})
}
func (s *Server) handleSaveRestore(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	ok, msg := s.palworld.RestoreSave(body.Name)
	writeJSON(w, map[string]any{"ok": ok, "message": msg})
}
func (s *Server) handleSaveUpload(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name, Content string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	n, err := s.palworld.UploadSave(body.Name, body.Content)
	writeJSON(w, map[string]any{"ok": err == nil, "message": uploadMessage(body.Name, n, err)})
}
func (s *Server) handleSaveDelete(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	err := s.palworld.DeleteSave(body.Name)
	writeJSON(w, map[string]any{"ok": err == nil, "message": deleteMessage(err)})
}
func (s *Server) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"whitelist": s.palworld.Whitelist()})
}
func (s *Server) handleBanlist(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"banlist": s.palworld.Banlist()})
}
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, s.palworld.ConfigSchema())
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.auth.RequireCSRF(r) {
		writeError(w, http.StatusForbidden, "CSRF token 无效")
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	err := s.palworld.SaveConfig(body)
	writeJSON(w, map[string]any{"ok": err == nil, "warning": "配置已保存，需重启服务器生效", "error": errString(err)})
}
func (s *Server) handlePlayerAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SteamID string `json:"steamid"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		out, err := s.palworld.RCON(action + " " + strings.TrimSpace(body.SteamID))
		writeJSON(w, map[string]any{"ok": err == nil, "output": out, "error": errString(err)})
	}
}
func (s *Server) handleWhitelistAdd(w http.ResponseWriter, r *http.Request) {
	var body palworld.WhitelistEntry
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg, err := s.palworld.AddWhitelist(body)
	writeJSON(w, map[string]any{"ok": err == nil, "message": messageOrError(msg, err)})
}
func (s *Server) handleWhitelistRemove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SteamID string `json:"steamid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg, err := s.palworld.RemoveWhitelist(body.SteamID)
	writeJSON(w, map[string]any{"ok": err == nil, "message": messageOrError(msg, err)})
}
func (s *Server) handleWhitelistCheck(w http.ResponseWriter, r *http.Request) {
	msg, err := s.palworld.CheckWhitelist()
	writeJSON(w, map[string]any{"ok": err == nil, "message": messageOrError(msg, err)})
}
func (s *Server) handleBanlistUnban(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SteamID string `json:"steamid"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msg, err := s.palworld.Unban(body.SteamID)
	writeJSON(w, map[string]any{"ok": err == nil, "message": messageOrError(msg, err)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	writeJSON(w, map[string]string{"error": msg})
}
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
func messageOrError(msg string, err error) string {
	if err != nil {
		return err.Error()
	}
	return msg
}
func uploadMessage(name string, n int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("已上传 %s (%d 字节)", name, n)
}
func deleteMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return "已删除"
}

func readJSONBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(v)
}
func ensureDataDir(path string) { _ = os.MkdirAll(path, 0755) }
