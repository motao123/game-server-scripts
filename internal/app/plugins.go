package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type PluginMeta struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DisplayName   string   `json:"displayName,omitempty"`
	Version       string   `json:"version"`
	Description   string   `json:"description"`
	Author        string   `json:"author"`
	Homepage      string   `json:"homepage,omitempty"`
	Entry         string   `json:"entry,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Enabled       bool     `json:"enabled"`
	Path          string   `json:"path"`
	MarketVersion string   `json:"marketVersion,omitempty"`
	Upgradable    bool     `json:"upgradable,omitempty"`
	Compatible    bool     `json:"compatible"`
	Compatibility string   `json:"compatibility,omitempty"`
}

func (s *Server) scanPlugins() []PluginMeta {
	root := filepath.Join(s.cfg.DataDir, "plugins")
	entries, err := os.ReadDir(root)
	if err != nil {
		return []PluginMeta{}
	}
	var plugins []PluginMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest := filepath.Join(root, entry.Name(), "plugin.json")
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue
		}
		var meta PluginMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		meta.ID = entry.Name()
		meta.Path = filepath.Join(root, entry.Name())
		state := filepath.Join(root, entry.Name(), ".enabled")
		if _, err := os.Stat(state); err == nil {
			meta.Enabled = true
		}
		plugins = append(plugins, meta)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return plugins
}
