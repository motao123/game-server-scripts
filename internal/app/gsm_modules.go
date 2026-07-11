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
)

type GameTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Ports       []int    `json:"ports"`
	Tags        []string `json:"tags"`
}

func builtinGames() []GameTemplate {
	return []GameTemplate{
		{ID: "palworld", Name: "Palworld", Type: "steam", Description: "幻兽帕鲁专用服务器", Ports: []int{8211, 27015, 25575}, Tags: []string{"SteamCMD", "RCON", "REST API"}},
		{ID: "minecraft-java", Name: "Minecraft Java", Type: "minecraft-java", Description: "Java 版服务端", Ports: []int{25565}, Tags: []string{"Java", "Paper", "Forge"}},
		{ID: "minecraft-bedrock", Name: "Minecraft Bedrock", Type: "minecraft-bedrock", Description: "基岩版服务端", Ports: []int{19132}, Tags: []string{"Bedrock"}},
		{ID: "valheim", Name: "Valheim", Type: "steam", Description: "英灵神殿专用服务器", Ports: []int{2456, 2457, 2458}, Tags: []string{"SteamCMD"}},
		{ID: "terraria", Name: "Terraria", Type: "steam", Description: "泰拉瑞亚服务端", Ports: []int{7777}, Tags: []string{"SteamCMD"}},
	}
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"games": builtinGames()})
}
func (s *Server) handleDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"steamcmd": lookPath("steamcmd"), "templates": len(builtinGames())})
}
func (s *Server) handleDeploymentInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GameID string `json:"gameId"`
		Path   string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, map[string]any{"ok": true, "message": fmt.Sprintf("已创建部署任务: %s -> %s", body.GameID, body.Path)})
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
	err := createTarGz(body.Path, archive)
	writeJSON(w, map[string]any{"ok": err == nil, "archive": archive, "error": errString(err)})
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
	var body struct{ Name, Cron, Action string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	t := NewTask(body.Name, body.Cron, body.Action)
	s.tasks.list = append(s.tasks.list, t)
	_ = s.tasks.Save()
	writeJSON(w, map[string]any{"ok": true, "task": t})
}

func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	filtered := s.tasks.list[:0]
	for _, t := range s.tasks.list {
		if t.ID != body.ID {
			filtered = append(filtered, t)
		}
	}
	s.tasks.list = filtered
	_ = s.tasks.Save()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "message": "插件运行时已预留"})
}
func (s *Server) handleEnvironmentInstall(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, map[string]any{"ok": true, "message": "环境安装任务已创建", "request": body})
}
func lookPath(name string) string                                           { p, _ := exec.LookPath(name); return p }
func (s *Server) handleBackupList(w http.ResponseWriter, r *http.Request)   { s.handleSaves(w, r) }
func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) { s.handleSaveBackup(w, r) }
