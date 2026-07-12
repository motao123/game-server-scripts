package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type PluginManager struct {
	dataDir string
	audit   *PluginAuditStore
}

func NewPluginManager(dataDir string) *PluginManager {
	return &PluginManager{dataDir: dataDir, audit: NewPluginAuditStore(filepath.Join(dataDir, "plugin_audit.jsonl"))}
}

func (m *PluginManager) pluginDir(id string) string {
	return filepath.Join(m.dataDir, "plugins", id)
}

func (m *PluginManager) auditStore() *PluginAuditStore {
	return m.audit
}

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
	Permissions   []string `json:"permissions,omitempty"`
	Risk          string   `json:"risk,omitempty"`
	RiskText      string   `json:"riskText,omitempty"`
	Enabled       bool     `json:"enabled"`
	Path          string   `json:"path"`
	MarketVersion string   `json:"marketVersion,omitempty"`
	Upgradable    bool     `json:"upgradable,omitempty"`
	Compatible    bool     `json:"compatible"`
	Compatibility string   `json:"compatibility,omitempty"`
}

func (m *PluginManager) scan() []PluginMeta {
	root := filepath.Join(m.dataDir, "plugins")
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

func (m *PluginManager) backup(id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件名称只允许字母数字下划线短横线")
	}
	src := m.pluginDir(id)
	if _, err := os.Stat(src); err != nil {
		return "", err
	}
	base := filepath.Join(m.dataDir, "plugin_backups", id+"-"+time.Now().Format("20060102150405.000000000"))
	dst := base
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	for i := 1; ; i++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			break
		}
		dst = fmt.Sprintf("%s-%d", base, i)
	}
	return dst, copyPluginDir(src, dst)
}
