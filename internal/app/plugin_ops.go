package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

func (s *Server) handlePluginCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Description string `json:"description"`
		Version     string `json:"version"`
		Author      string `json:"author"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(body.Name) {
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
	dir := filepath.Join(s.cfg.DataDir, "plugins", body.Name)
	if err := os.RemoveAll(dir); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
