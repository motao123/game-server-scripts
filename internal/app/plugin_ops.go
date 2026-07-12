package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var pluginIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validPluginID(id string) bool {
	return pluginIDPattern.MatchString(id)
}

func (s *Server) handlePluginCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Version     string `json:"version"`
		Author      string `json:"author"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validPluginID(body.Name) {
		writeError(w, 400, "插件名称只允许字母数字下划线短横线")
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", body.Name)
	if _, err := os.Stat(dir); err == nil {
		writeError(w, 409, "插件已存在")
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	meta := map[string]any{
		"name": body.Name, "displayName": body.DisplayName, "description": body.Description,
		"version": body.Version, "author": body.Author, "enabled": false,
	}
	data, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0644); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "plugin": meta})
}

func (s *Server) handlePluginDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validPluginID(body.Name) {
		writeError(w, 400, "插件名称只允许字母数字下划线短横线")
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", body.Name)
	if err := os.RemoveAll(dir); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handlePluginCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"plugins": s.pluginCatalog()})
}

func (s *Server) handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validPluginID(body.ID) {
		writeError(w, 400, "插件名称只允许字母数字下划线短横线")
		return
	}
	var found *PluginCatalogItem
	for _, item := range s.pluginCatalog() {
		if item.ID == body.ID {
			copy := item
			found = &copy
			break
		}
	}
	if found == nil {
		writeError(w, 404, "插件不存在")
		return
	}
	meta, err := s.installPluginFromCatalog(*found)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "plugin": meta})
}

func (s *Server) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("id")
		if !validPluginID(id) {
			writeError(w, 400, "插件名称只允许字母数字下划线短横线")
			return
		}
		path := filepath.Join(s.cfg.DataDir, "plugins", id, "config.json")
		data, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, map[string]any{"config": map[string]any{}})
			return
		}
		var cfg any
		if err := json.Unmarshal(data, &cfg); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"config": cfg})
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
		ID     string         `json:"id"`
		Config map[string]any `json:"config"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validPluginID(body.ID) {
		writeError(w, 400, "插件名称只允许字母数字下划线短横线")
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", body.ID)
	if _, err := os.Stat(dir); err != nil {
		writeError(w, 404, "插件未安装")
		return
	}
	data, _ := json.MarshalIndent(body.Config, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "config": body.Config})
}

func validPluginFileName(name string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`).MatchString(name)
}
