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

const panelVersion = "0.1.0"

func (s *Server) installPluginFromCatalog(item PluginCatalogItem) (PluginMeta, error) {
	compatible, reason := pluginCompatibility(item)
	if !compatible {
		return PluginMeta{}, fmt.Errorf(reason)
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", item.ID)
	if _, err := os.Stat(dir); err == nil {
		return PluginMeta{}, fmt.Errorf("插件已安装")
	}
	if item.Source.URL != "" {
		return s.installPluginArchive(item, dir)
	}
	return s.installPluginFiles(item, dir)
}

func (s *Server) upgradePluginFromCatalog(item PluginCatalogItem) (PluginMeta, string, error) {
	compatible, reason := pluginCompatibility(item)
	if !compatible {
		return PluginMeta{}, "", fmt.Errorf(reason)
	}
	dir := filepath.Join(s.cfg.DataDir, "plugins", item.ID)
	oldMeta, err := readPluginMeta(dir)
	if err != nil {
		return PluginMeta{}, "", fmt.Errorf("插件未安装")
	}
	if compareVersions(item.Version, oldMeta.Version) <= 0 {
		return PluginMeta{}, "", fmt.Errorf("当前已是最新版本")
	}
	enabled := false
	if _, err := os.Stat(filepath.Join(dir, ".enabled")); err == nil {
		enabled = true
	}
	backup, err := s.backupPluginDir(item.ID)
	if err != nil {
		return PluginMeta{}, "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return PluginMeta{}, backup, err
	}
	meta, err := s.installPluginFromCatalog(item)
	if err != nil {
		_ = os.RemoveAll(dir)
		_ = copyPluginDir(backup, dir)
		return PluginMeta{}, backup, err
	}
	if enabled {
		_ = os.WriteFile(filepath.Join(dir, ".enabled"), []byte("1"), 0644)
	}
	return meta, backup, nil
}

func (s *Server) backupPluginDir(id string) (string, error) {
	if !validPluginID(id) {
		return "", fmt.Errorf("插件名称只允许字母数字下划线短横线")
	}
	src := filepath.Join(s.cfg.DataDir, "plugins", id)
	if _, err := os.Stat(src); err != nil {
		return "", err
	}
	base := filepath.Join(s.cfg.DataDir, "plugin_backups", id+"-"+time.Now().Format("20060102150405.000000000"))
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

func readPluginMeta(dir string) (PluginMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		return PluginMeta{}, err
	}
	var meta PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return PluginMeta{}, err
	}
	return meta, nil
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

func pluginCompatibility(item PluginCatalogItem) (bool, string) {
	if item.MinPanelVersion != "" && compareVersions(panelVersion, item.MinPanelVersion) < 0 {
		return false, "需要面板版本 >= " + item.MinPanelVersion
	}
	if item.MaxPanelVersion != "" && compareVersions(panelVersion, item.MaxPanelVersion) > 0 {
		return false, "需要面板版本 <= " + item.MaxPanelVersion
	}
	return true, ""
}

func compareVersions(a, b string) int {
	ap := versionParts(a)
	bp := versionParts(b)
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		n := 0
		for _, r := range field {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

func copyPluginDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0755)
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
