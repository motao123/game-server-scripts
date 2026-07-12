package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GameTemplate struct {
	ID           string     `json:"id"`
	SourceKey    string     `json:"sourceKey,omitempty"`
	Name         string     `json:"name"`
	NameCN       string     `json:"nameCN,omitempty"`
	Type         string     `json:"type"`
	Description  string     `json:"description"`
	Tip          string     `json:"tip,omitempty"`
	Ports        []int      `json:"ports"`
	PortMappings []GamePort `json:"portMappings,omitempty"`
	Tags         []string   `json:"tags"`
	DefaultPath  string     `json:"defaultPath"`
	AppID        int        `json:"appId,omitempty"`
	Systems      []string   `json:"systems,omitempty"`
	SystemInfo   []string   `json:"systemInfo,omitempty"`
	Image        string     `json:"image,omitempty"`
	StoreURL     string     `json:"storeUrl,omitempty"`
	DocsURL      string     `json:"docsUrl,omitempty"`
	MemoryGB     int        `json:"memoryGB,omitempty"`
	ServerTypes  []string   `json:"serverTypes,omitempty"`
	Version      string     `json:"version,omitempty"`
	Support      string     `json:"support,omitempty"`
	SupportNote  string     `json:"supportNote,omitempty"`
}

type GamePort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Name     string `json:"name,omitempty"`
}

func builtinGames() []GameTemplate {
	return []GameTemplate{
		{ID: "palworld", Name: "Palworld", Type: "steam", Description: "幻兽帕鲁专用服务器", Ports: []int{8211, 27015, 25575}, Tags: []string{"SteamCMD", "RCON", "REST API"}, DefaultPath: "/home/steam/Steam/steamapps/common/PalServer", AppID: 2394010},
		{ID: "minecraft-java", Name: "Minecraft Java", Type: "minecraft-java", Description: "Java 版服务端", Ports: []int{25565}, Tags: []string{"Java", "Paper", "Forge", "Fabric", "Vanilla"}, DefaultPath: "/home/steam/minecraft-java", ServerTypes: []string{"paper", "vanilla", "fabric", "forge", "spigot"}, Version: "latest"},
		{ID: "minecraft-bedrock", Name: "Minecraft Bedrock", Type: "minecraft-bedrock", Description: "基岩版服务端", Ports: []int{19132}, Tags: []string{"Bedrock"}, DefaultPath: "/home/steam/minecraft-bedrock"},
		{ID: "valheim", Name: "Valheim", Type: "steam", Description: "英灵神殿专用服务器", Ports: []int{2456, 2457, 2458}, Tags: []string{"SteamCMD"}, DefaultPath: "/home/steam/Steam/steamapps/common/Valheim", AppID: 896660},
		{ID: "terraria", Name: "Terraria", Type: "steam", Description: "泰拉瑞亚服务端", Ports: []int{7777}, Tags: []string{"SteamCMD"}, DefaultPath: "/home/steam/Steam/steamapps/common/Terraria", AppID: 105600},
	}
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"games": s.games()})
}
func (s *Server) handleSteamcmdStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"steamcmd": lookPath("steamcmd"), "templates": len(s.games())})
}
func (s *Server) handleDeploymentInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GameID     string `json:"gameId"`
		Path       string `json:"path"`
		ServerType string `json:"serverType"`
		Version    string `json:"version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var game *GameTemplate
	games := s.games()
	for i := range games {
		if games[i].ID == body.GameID {
			g := games[i]
			game = &g
			break
		}
	}
	if game == nil {
		writeError(w, http.StatusBadRequest, "未知的游戏: "+body.GameID)
		return
	}
	path := body.Path
	if path == "" {
		path = game.DefaultPath
	}
	if _, err := ensureGameDeployable(*game); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := s.deploys.Start(*game, path, s.instances, DeployOptions{ServerType: body.ServerType, Version: body.Version})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"ok":      true,
		"taskId":  task.ID,
		"message": fmt.Sprintf("已启动 %s 部署任务", game.Name),
		"path":    path,
	})
}

func (s *Server) handleDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("taskId")
	task := s.deploys.Get(taskID)
	if task == nil {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}
	writeJSON(w, task)
}

func (s *Server) handleFilesDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !s.safeRoot(path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if !s.safeRoot(dir) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	defer file.Close()
	name := filepath.Base(header.Filename)
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer dst.Close()
	n, err := io.Copy(dst, file)
	writeJSON(w, map[string]any{"ok": err == nil, "name": name, "size": n, "error": errString(err)})
}

func (s *Server) handleFilesDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	err := os.RemoveAll(body.Path)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleFilesMkdir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(filepath.Dir(body.Path)) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	err := os.MkdirAll(body.Path, 0755)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleFilesCompress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	archive := strings.TrimRight(body.Path, string(os.PathSeparator)) + ".tar.gz"
	task := s.fileTasks.Add("compress", filepath.Base(body.Path), func(update func(int, string)) error {
		update(10, "压缩中")
		return createTarGz(body.Path, archive)
	})
	writeJSON(w, map[string]any{"ok": true, "archive": archive, "task": task})
}

func createTarGz(src, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	base := filepath.Dir(src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dst {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	var body ScheduledTask
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Cron) == "" || strings.TrimSpace(body.Action) == "" {
		writeError(w, http.StatusBadRequest, "任务名称、Cron 和动作不能为空")
		return
	}
	if body.ID == "" && !body.Enabled {
		body.Enabled = true
	}
	task, err := s.tasks.Upsert(body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.scheduler.Reload()
	if refreshed, ok := s.tasks.Get(task.ID); ok {
		task = refreshed
	}
	writeJSON(w, map[string]any{"ok": true, "task": task})
}

func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.tasks.Delete(body.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.scheduler.Reload()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTaskToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.tasks.SetEnabled(body.ID, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.scheduler.Reload()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTaskRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if _, ok := s.tasks.Get(body.ID); !ok {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}
	go s.scheduler.RunTask(body.ID)
	writeJSON(w, map[string]any{"ok": true, "message": "任务已触发"})
}

func (s *Server) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validPluginID(body.ID) {
		writeError(w, 400, "插件名称只允许字母数字下划线短横线")
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", body.ID)
	if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err != nil {
		writeError(w, http.StatusNotFound, "插件未安装")
		return
	}
	state := filepath.Join(dir, ".enabled")
	var err error
	if body.Enabled {
		err = os.WriteFile(state, []byte("1"), 0644)
	} else {
		_ = os.Remove(state)
	}
	if err != nil {
		s.plugins.auditStore().Record(r, "plugin.toggle", body.ID, "failed", err.Error(), map[string]any{"enabled": body.Enabled})
		writeError(w, 500, err.Error())
		return
	}
	s.plugins.auditStore().Record(r, "plugin.toggle", body.ID, "success", "插件启用状态已更新", map[string]any{"enabled": body.Enabled})
	writeJSON(w, map[string]any{"ok": true})
}
func (s *Server) handleEnvironmentInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Package string `json:"package"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := time.Now().Format("20060102150405") + "-" + body.Package
	task, err := s.installs.Start(body.Package, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "taskId": task.ID, "message": "安装任务已启动"})
}

func (s *Server) handleEnvironmentInstallStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("taskId")
	task := s.installs.Get(taskID)
	if task == nil {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}
	writeJSON(w, task)
}
func lookPath(name string) string                                           { p, _ := exec.LookPath(name); return p }
func (s *Server) handleBackupList(w http.ResponseWriter, r *http.Request)   { s.handleSaves(w, r) }
func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) { s.handleSaveBackup(w, r) }
