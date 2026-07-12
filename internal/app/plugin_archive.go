package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) installPluginFromCatalog(item PluginCatalogItem) (PluginMeta, error) {
	dir := filepath.Join(s.cfg.DataDir, "plugins", item.ID)
	if _, err := os.Stat(dir); err == nil {
		return PluginMeta{}, fmt.Errorf("插件已安装")
	}
	if item.Source.URL != "" {
		return s.installPluginArchive(item, dir)
	}
	return s.installPluginFiles(item, dir)
}

func (s *Server) installPluginFiles(item PluginCatalogItem, dir string) (PluginMeta, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return PluginMeta{}, err
	}
	meta := pluginMetaFromCatalog(item)
	data, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0644); err != nil {
		return PluginMeta{}, err
	}
	for name, content := range item.Files {
		if !validPluginArchivePath(name) {
			continue
		}
		target := filepath.Join(dir, filepath.Clean(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return PluginMeta{}, err
		}
		if err := os.WriteFile(target, []byte(content), pluginFileMode(name)); err != nil {
			return PluginMeta{}, err
		}
	}
	return meta, nil
}

func (s *Server) installPluginArchive(item PluginCatalogItem, dir string) (PluginMeta, error) {
	if item.Source.Type != "" && item.Source.Type != "archive" {
		return PluginMeta{}, fmt.Errorf("不支持的插件来源类型: %s", item.Source.Type)
	}
	parsed, err := url.Parse(strings.TrimSpace(item.Source.URL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return PluginMeta{}, fmt.Errorf("插件包 URL 无效")
	}
	tmpRoot := filepath.Join(s.cfg.DataDir, "tmp", "plugin-install-"+item.ID+"-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpRoot)
	if err := os.MkdirAll(tmpRoot, 0755); err != nil {
		return PluginMeta{}, err
	}
	archive := filepath.Join(tmpRoot, "plugin.tar.gz")
	if err := downloadPluginArchive(item.Source.URL, archive); err != nil {
		return PluginMeta{}, err
	}
	if item.Source.SHA256 != "" {
		if err := verifyFileSHA256(archive, item.Source.SHA256); err != nil {
			return PluginMeta{}, err
		}
	}
	archiveType := strings.ToLower(item.Source.ArchiveType)
	if archiveType == "" {
		archiveType = "tar.gz"
	}
	switch archiveType {
	case "tar.gz", "tgz":
		if err := extractTarGz(archive, tmpRoot); err != nil {
			return PluginMeta{}, err
		}
	default:
		return PluginMeta{}, fmt.Errorf("不支持的插件包类型: %s", archiveType)
	}
	root := detectPluginArchiveRoot(tmpRoot)
	metaPath := filepath.Join(root, "plugin.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return PluginMeta{}, fmt.Errorf("插件包缺少 plugin.json")
	}
	var meta PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return PluginMeta{}, err
	}
	if meta.ID == "" {
		meta.ID = item.ID
	}
	if meta.ID != item.ID || !validPluginID(meta.ID) {
		return PluginMeta{}, fmt.Errorf("插件包 ID 不匹配")
	}
	if meta.Name == "" {
		meta.Name = item.Name
	}
	if meta.DisplayName == "" {
		meta.DisplayName = item.DisplayName
	}
	if meta.Version == "" {
		meta.Version = item.Version
	}
	if meta.Description == "" {
		meta.Description = item.Description
	}
	if meta.Author == "" {
		meta.Author = item.Author
	}
	if len(meta.Tags) == 0 {
		meta.Tags = item.Tags
	}
	if len(meta.Capabilities) == 0 {
		meta.Capabilities = item.Capabilities
	}
	if err := os.RemoveAll(dir); err != nil {
		return PluginMeta{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return PluginMeta{}, err
	}
	if err := os.Rename(root, dir); err != nil {
		return PluginMeta{}, err
	}
	data, _ = json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0644); err != nil {
		return PluginMeta{}, err
	}
	return meta, nil
}

func pluginMetaFromCatalog(item PluginCatalogItem) PluginMeta {
	return PluginMeta{ID: item.ID, Name: item.Name, DisplayName: item.DisplayName, Version: item.Version, Description: item.Description, Author: item.Author, Homepage: item.Homepage, Entry: item.Entry, Tags: item.Tags, Capabilities: item.Capabilities}
}

func downloadPluginArchive(rawURL, dst string) error {
	data, err := downloadCatalog(rawURL)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func verifyFileSHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("插件包 sha256 校验失败")
	}
	return nil
}

func detectPluginArchiveRoot(tmpRoot string) string {
	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		return tmpRoot
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(tmpRoot, entry.Name()))
		}
	}
	if len(dirs) == 1 {
		if _, err := os.Stat(filepath.Join(dirs[0], "plugin.json")); err == nil {
			return dirs[0]
		}
	}
	return tmpRoot
}

func validPluginArchivePath(name string) bool {
	clean := filepath.Clean(name)
	return clean != "." && !filepath.IsAbs(clean) && !strings.HasPrefix(clean, "..") && !strings.Contains(clean, string(os.PathSeparator)+".."+string(os.PathSeparator))
}

func pluginFileMode(name string) os.FileMode {
	if strings.HasSuffix(name, ".sh") {
		return 0755
	}
	return 0644
}
