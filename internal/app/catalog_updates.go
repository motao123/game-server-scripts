package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CatalogStatus struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Count     int    `json:"count"`
	Available bool   `json:"available"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func (s *Server) handleCatalogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"catalogs": s.catalogStatuses()})
}

func (s *Server) handleCatalogReload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := reloadCatalog(body.Type); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "catalogs": s.catalogStatuses()})
}

func (s *Server) handleCatalogUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	result, err := updateCatalog(body.Type, body.URL)
	if err != nil {
		if body.Type == "plugin" {
			s.pluginAudit.Record(r, "catalog.plugin.update", "", "failed", err.Error(), map[string]any{"url": body.URL})
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Type == "plugin" {
		s.pluginAudit.Record(r, "catalog.plugin.update", "", "success", "插件市场已更新", map[string]any{"url": body.URL, "count": result["count"], "backup": result["backup"]})
	}
	writeJSON(w, map[string]any{"ok": true, "result": result, "catalogs": s.catalogStatuses()})
}

func (s *Server) catalogStatuses() []CatalogStatus {
	types := []string{"game", "plugin", "online"}
	statuses := make([]CatalogStatus, 0, len(types))
	for _, kind := range types {
		path := catalogPath(kind)
		status := CatalogStatus{Type: kind, Name: catalogName(kind), Path: path}
		if info, err := os.Stat(path); err == nil {
			status.Available = true
			status.UpdatedAt = info.ModTime().Format(time.RFC3339)
		}
		status.Count = catalogCount(kind)
		statuses = append(statuses, status)
	}
	return statuses
}

func updateCatalog(kind, rawURL string) (map[string]any, error) {
	if _, err := catalogSpec(kind); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("请输入有效的 http/https 地址")
	}
	data, err := downloadCatalog(rawURL)
	if err != nil {
		return nil, err
	}
	count, err := validateCatalog(kind, data)
	if err != nil {
		return nil, err
	}
	path := catalogPath(kind)
	backup := ""
	if old, err := os.ReadFile(path); err == nil {
		backup = path + "." + time.Now().Format("20060102150405") + ".bak"
		if err := os.WriteFile(backup, old, 0644); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		if backup != "" {
			_ = restoreCatalog(path, backup)
		}
		return nil, err
	}
	if err := reloadCatalog(kind); err != nil {
		if backup != "" {
			_ = restoreCatalog(path, backup)
			_ = reloadCatalog(kind)
		}
		return nil, err
	}
	return map[string]any{"type": kind, "path": path, "backup": backup, "count": count}, nil
}

func downloadCatalog(rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "game-server-scripts/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 10*1024*1024 {
		return nil, fmt.Errorf("清单文件超过 10MB")
	}
	return data, nil
}

func validateCatalog(kind string, data []byte) (int, error) {
	switch kind {
	case "game":
		var items []GameTemplate
		if err := json.Unmarshal(data, &items); err != nil || len(items) == 0 {
			return 0, fmt.Errorf("游戏清单 JSON 无效")
		}
		for i := range items {
			items[i].normalize()
			if items[i].ID == "" || items[i].Name == "" {
				return 0, fmt.Errorf("游戏清单缺少 id/name")
			}
		}
		return len(items), nil
	case "plugin":
		items := normalizePluginCatalog(mustPluginCatalog(data))
		if len(items) == 0 {
			return 0, fmt.Errorf("插件清单 JSON 无效")
		}
		return len(items), nil
	case "online":
		items := normalizeOnlineTemplates(mustOnlineTemplates(data))
		if len(items) == 0 {
			return 0, fmt.Errorf("在线模板 JSON 无效")
		}
		return len(items), nil
	default:
		return 0, fmt.Errorf("不支持的清单类型: %s", kind)
	}
}

func mustPluginCatalog(data []byte) []PluginCatalogItem {
	var items []PluginCatalogItem
	_ = json.Unmarshal(data, &items)
	return items
}

func mustOnlineTemplates(data []byte) []OnlineTemplate {
	var items []OnlineTemplate
	_ = json.Unmarshal(data, &items)
	return items
}

func reloadCatalog(kind string) error {
	switch kind {
	case "game":
		reloadGameCatalog()
		return nil
	case "plugin", "online":
		return nil
	default:
		return fmt.Errorf("不支持的清单类型: %s", kind)
	}
}

func catalogCount(kind string) int {
	switch kind {
	case "game":
		return len(reloadGameCatalog())
	case "plugin":
		return len(loadPluginCatalog())
	case "online":
		return len(loadOnlineTemplates())
	default:
		return 0
	}
}

func restoreCatalog(path, backup string) error {
	data, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func catalogPath(kind string) string {
	spec, _ := catalogSpec(kind)
	for _, path := range spec.paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return spec.paths[0]
}

func catalogName(kind string) string {
	spec, _ := catalogSpec(kind)
	return spec.name
}

func catalogSpec(kind string) (struct {
	name  string
	paths []string
}, error) {
	switch kind {
	case "game":
		return struct {
			name  string
			paths []string
		}{"游戏清单", catalogPaths("game_catalog.json")}, nil
	case "plugin":
		return struct {
			name  string
			paths []string
		}{"插件市场", catalogPaths("plugin_catalog.json")}, nil
	case "online":
		return struct {
			name  string
			paths []string
		}{"在线模板", catalogPaths("online_templates.json")}, nil
	default:
		return struct {
			name  string
			paths []string
		}{}, fmt.Errorf("不支持的清单类型: %s", kind)
	}
}

func catalogPaths(file string) []string {
	return []string{
		filepath.Join("/usr/local/share/gsm-panel", "data", file),
		filepath.Join("/opt/gsm-panel", "data", file),
		filepath.Join("data", file),
		file,
	}
}
