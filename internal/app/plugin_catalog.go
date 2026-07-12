package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type PluginCatalogItem struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName,omitempty"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	Homepage     string            `json:"homepage,omitempty"`
	Entry        string            `json:"entry,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Files        map[string]string `json:"files,omitempty"`
	Installed    bool              `json:"installed"`
}

func (s *Server) pluginCatalog() []PluginCatalogItem {
	catalog := loadPluginCatalog()
	installed := map[string]bool{}
	for _, plugin := range s.scanPlugins() {
		installed[plugin.ID] = true
	}
	for i := range catalog {
		catalog[i].Installed = installed[catalog[i].ID]
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].DisplayName < catalog[j].DisplayName })
	return catalog
}

func loadPluginCatalog() []PluginCatalogItem {
	paths := []string{
		filepath.Join("data", "plugin_catalog.json"),
		"plugin_catalog.json",
		filepath.Join("/opt/gsm-panel", "data", "plugin_catalog.json"),
		filepath.Join("/usr/local/share/gsm-panel", "data", "plugin_catalog.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var items []PluginCatalogItem
		if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
			return normalizePluginCatalog(items)
		}
	}
	return normalizePluginCatalog(defaultPluginCatalog())
}

func normalizePluginCatalog(items []PluginCatalogItem) []PluginCatalogItem {
	out := make([]PluginCatalogItem, 0, len(items))
	for _, item := range items {
		if !validPluginID(item.ID) {
			continue
		}
		if item.Name == "" {
			item.Name = item.ID
		}
		if item.DisplayName == "" {
			item.DisplayName = item.Name
		}
		if item.Version == "" {
			item.Version = "1.0.0"
		}
		out = append(out, item)
	}
	return out
}

func defaultPluginCatalog() []PluginCatalogItem {
	return []PluginCatalogItem{
		{
			ID:           "server-notes",
			Name:         "server-notes",
			DisplayName:  "服务器备注",
			Version:      "1.0.0",
			Description:  "为实例运维记录维护说明、负责人和变更备注。",
			Author:       "game-server-scripts",
			Tags:         []string{"运维", "备注"},
			Capabilities: []string{"config"},
		},
		{
			ID:           "maintenance-checklist",
			Name:         "maintenance-checklist",
			DisplayName:  "维护检查清单",
			Version:      "1.0.0",
			Description:  "提供重启、备份、日志检查等维护事项模板。",
			Author:       "game-server-scripts",
			Tags:         []string{"维护", "清单"},
			Capabilities: []string{"config"},
		},
	}
}
