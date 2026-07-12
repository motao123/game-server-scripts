package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type PluginCatalogItem struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	DisplayName      string            `json:"displayName,omitempty"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Author           string            `json:"author"`
	Homepage         string            `json:"homepage,omitempty"`
	Entry            string            `json:"entry,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Capabilities     []string          `json:"capabilities,omitempty"`
	Permissions      []string          `json:"permissions,omitempty"`
	Risk             string            `json:"risk,omitempty"`
	RiskText         string            `json:"riskText,omitempty"`
	MinPanelVersion  string            `json:"minPanelVersion,omitempty"`
	MaxPanelVersion  string            `json:"maxPanelVersion,omitempty"`
	Source           PluginSource      `json:"source,omitempty"`
	Files            map[string]string `json:"files,omitempty"`
	Installed        bool              `json:"installed"`
	InstalledVersion string            `json:"installedVersion,omitempty"`
	Upgradable       bool              `json:"upgradable,omitempty"`
	Compatible       bool              `json:"compatible"`
	Compatibility    string            `json:"compatibility,omitempty"`
}

type PluginSource struct {
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	ArchiveType string `json:"archiveType,omitempty"`
}

func (s *Server) pluginCatalog() []PluginCatalogItem {
	catalog := loadPluginCatalog()
	installed := map[string]PluginMeta{}
	for _, plugin := range s.scanPlugins() {
		installed[plugin.ID] = plugin
	}
	for i := range catalog {
		plugin, ok := installed[catalog[i].ID]
		catalog[i].Installed = ok
		catalog[i].Risk, catalog[i].RiskText = pluginPermissionRisk(catalog[i].Permissions)
		catalog[i].Compatible, catalog[i].Compatibility = pluginCompatibility(catalog[i])
		if ok {
			catalog[i].InstalledVersion = plugin.Version
			catalog[i].Upgradable = catalog[i].Compatible && compareVersions(catalog[i].Version, plugin.Version) > 0
		}
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].DisplayName < catalog[j].DisplayName })
	return catalog
}

func (s *Server) pluginsWithCatalog() []PluginMeta {
	plugins := s.scanPlugins()
	catalog := map[string]PluginCatalogItem{}
	for _, item := range loadPluginCatalog() {
		catalog[item.ID] = item
	}
	for i := range plugins {
		item, ok := catalog[plugins[i].ID]
		plugins[i].Compatible = true
		plugins[i].Risk, plugins[i].RiskText = pluginPermissionRisk(plugins[i].Permissions)
		if ok {
			plugins[i].MarketVersion = item.Version
			plugins[i].Risk, plugins[i].RiskText = pluginPermissionRisk(item.Permissions)
			plugins[i].Compatible, plugins[i].Compatibility = pluginCompatibility(item)
			plugins[i].Upgradable = plugins[i].Compatible && compareVersions(item.Version, plugins[i].Version) > 0
		}
	}
	return plugins
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
		item.Permissions = normalizePluginPermissions(item.Permissions)
		item.Risk, item.RiskText = pluginPermissionRisk(item.Permissions)
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
			Permissions:  []string{"config"},
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
			Permissions:  []string{"config"},
		},
	}
}
