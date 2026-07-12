package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var backupPathPartRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

type BackupEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time int64  `json:"time"`
}

type BackupGroup struct {
	Name   string        `json:"name"`
	Files  []BackupEntry `json:"files"`
	Total  int64         `json:"totalSize"`
	Source string        `json:"sourcePath"`
}

type BackupMeta struct {
	SourcePath string `json:"sourcePath"`
}

type BackupPreflight struct {
	Ready          bool     `json:"ready"`
	SourcePath     string   `json:"sourcePath,omitempty"`
	TargetPath     string   `json:"targetPath,omitempty"`
	ArchiveEntries int      `json:"archiveEntries,omitempty"`
	Problems       []string `json:"problems"`
	Warnings       []string `json:"warnings"`
}

type BackupManager struct {
	root    string
	allowed func(string) bool
}

func NewBackupManager(root string, allowed func(string) bool) *BackupManager {
	return &BackupManager{root: root, allowed: allowed}
}

func (m *BackupManager) Groups() []BackupGroup {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return []BackupGroup{}
	}
	groups := make([]BackupGroup, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !validBackupPathPart(entry.Name()) {
			continue
		}
		group := BackupGroup{Name: entry.Name()}
		groupDir := filepath.Join(m.root, entry.Name())
		meta, _ := readBackupMeta(groupDir)
		group.Source = meta.SourcePath
		files, _ := os.ReadDir(groupDir)
		for _, f := range files {
			if f.IsDir() || !validBackupArchiveName(f.Name()) {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			group.Files = append(group.Files, BackupEntry{Name: f.Name(), Size: info.Size(), Time: info.ModTime().Unix()})
			group.Total += info.Size()
		}
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func (m *BackupManager) CreatePreflight(name, sourcePath string) BackupPreflight {
	result := BackupPreflight{Ready: true, SourcePath: sourcePath}
	if !validBackupPathPart(name) {
		result.Problems = append(result.Problems, errBackupName().Error())
	}
	if !m.allowed(sourcePath) {
		result.Problems = append(result.Problems, errBackupSource().Error())
	} else if info, err := os.Stat(sourcePath); err != nil {
		result.Problems = append(result.Problems, "备份源路径不存在: "+sourcePath)
	} else if !info.IsDir() {
		result.Problems = append(result.Problems, "备份源路径不是目录: "+sourcePath)
	}
	result.Ready = len(result.Problems) == 0
	return result
}

func (m *BackupManager) Create(name, sourcePath string, maxKeep int) (string, error) {
	if check := m.CreatePreflight(name, sourcePath); !check.Ready {
		return "", &backupError{strings.Join(check.Problems, "；")}
	}
	groupDir := filepath.Join(m.root, name)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(BackupMeta{SourcePath: sourcePath}, "", "  ")
	if err := os.WriteFile(filepath.Join(groupDir, "data.json"), data, 0644); err != nil {
		return "", err
	}
	archive := filepath.Join(groupDir, time.Now().Format("2006-01-02_15-04-05")+".tar.gz")
	if err := createTarGz(sourcePath, archive); err != nil {
		return "", err
	}
	if maxKeep > 0 {
		enforceRetention(groupDir, maxKeep)
	}
	return archive, nil
}

func (m *BackupManager) ArchivePath(name, fileName string) (string, error) {
	if !validBackupPathPart(name) || !validBackupArchiveName(fileName) {
		return "", errBackupName()
	}
	path := filepath.Join(m.root, name, fileName)
	if !insideRoot(m.root, path) {
		return "", errBackupName()
	}
	return path, nil
}

func (m *BackupManager) RestorePreflight(name, fileName string) BackupPreflight {
	result := BackupPreflight{Ready: true}
	groupDir := filepath.Join(m.root, name)
	if !validBackupPathPart(name) || !insideRoot(m.root, groupDir) {
		result.Problems = append(result.Problems, errBackupName().Error())
		result.Ready = false
		return result
	}
	meta, err := readBackupMeta(groupDir)
	if err != nil || meta.SourcePath == "" {
		result.Problems = append(result.Problems, errBackupTarget().Error())
		result.Ready = false
		return result
	}
	result.SourcePath = meta.SourcePath
	result.TargetPath = filepath.Dir(meta.SourcePath)
	if !m.allowed(meta.SourcePath) {
		result.Problems = append(result.Problems, errBackupSource().Error())
	}
	archive, err := m.ArchivePath(name, fileName)
	if err != nil {
		result.Problems = append(result.Problems, err.Error())
	} else if entries, err := validateTarGz(archive); err != nil {
		result.Problems = append(result.Problems, "归档校验失败: "+err.Error())
	} else {
		result.ArchiveEntries = entries
	}
	if _, err := os.Stat(meta.SourcePath); err == nil {
		result.Warnings = append(result.Warnings, "恢复会覆盖现有目录中的同名文件")
	}
	result.Ready = len(result.Problems) == 0
	return result
}

func (m *BackupManager) Restore(name, fileName string) (string, error) {
	return m.RestoreWithOptions(name, fileName, false)
}

func (m *BackupManager) RestoreWithOptions(name, fileName string, overwrite bool) (string, error) {
	check := m.RestorePreflight(name, fileName)
	if !check.Ready {
		return "", &backupError{strings.Join(check.Problems, "；")}
	}
	if len(check.Warnings) > 0 && !overwrite {
		return "", &backupError{"恢复目标已存在，需要确认覆盖"}
	}
	archive, _ := m.ArchivePath(name, fileName)
	if err := extractTarGz(archive, check.TargetPath); err != nil {
		return "", err
	}
	return check.SourcePath, nil
}

func validateTarGz(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		clean := filepath.Clean(header.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return 0, fmt.Errorf("归档包含不安全路径: %s", header.Name)
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("归档不包含文件")
	}
	return count, nil
}

func (m *BackupManager) Delete(name, fileName string) error {
	path, err := m.ArchivePath(name, fileName)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *Server) handleBackupGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"groups": s.backups.Groups()})
}

func (s *Server) handleBackupCreatePreflight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		SourcePath string `json:"sourcePath"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, s.backups.CreatePreflight(body.BackupName, body.SourcePath))
}

func (s *Server) handleBackupCreateGeneric(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		SourcePath string `json:"sourcePath"`
		MaxKeep    int    `json:"maxKeep"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.BackupName == "" || body.SourcePath == "" {
		writeError(w, 400, "backupName 和 sourcePath 不能为空")
		return
	}
	archive, err := s.backups.Create(body.BackupName, body.SourcePath, body.MaxKeep)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "archive": archive})
}

func (s *Server) handleBackupRestorePreflight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		FileName   string `json:"fileName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, s.backups.RestorePreflight(body.BackupName, body.FileName))
}

func (s *Server) handleBackupRestoreGeneric(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		FileName   string `json:"fileName"`
		Overwrite  bool   `json:"overwrite"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	target, err := s.backups.RestoreWithOptions(body.BackupName, body.FileName, body.Overwrite)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "targetPath": target})
}

func (s *Server) handleBackupDeleteFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		FileName   string `json:"fileName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	err := s.backups.Delete(body.BackupName, body.FileName)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	path, err := s.backups.ArchivePath(r.URL.Query().Get("backupName"), r.URL.Query().Get("fileName"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

func enforceRetention(groupDir string, maxKeep int) {
	entries, err := os.ReadDir(groupDir)
	if err != nil {
		return
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
			files = append(files, e)
		}
	}
	if len(files) <= maxKeep {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		ai, _ := files[i].Info()
		aj, _ := files[j].Info()
		return ai.ModTime().Before(aj.ModTime())
	})
	for i := 0; i < len(files)-maxKeep; i++ {
		_ = os.Remove(filepath.Join(groupDir, files[i].Name()))
	}
}

func extractJSONValue(jsonStr, key string) string {
	idx := strings.Index(jsonStr, "\""+key+"\"")
	if idx < 0 {
		return ""
	}
	rest := jsonStr[idx+len(key)+2:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = rest[colon+1:]
	start := strings.Index(rest, "\"")
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func readBackupMeta(groupDir string) (BackupMeta, error) {
	data, err := os.ReadFile(filepath.Join(groupDir, "data.json"))
	if err != nil {
		return BackupMeta{}, err
	}
	var meta BackupMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		meta.SourcePath = extractJSONValue(string(data), "sourcePath")
	}
	return meta, nil
}

func validBackupPathPart(name string) bool {
	return backupPathPartRe.MatchString(name)
}

func validBackupArchiveName(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") && validBackupPathPart(strings.TrimSuffix(name, ".tar.gz"))
}

func insideRoot(root, path string) bool {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator))
}

func errBackupName() error   { return &backupError{"备份名称或文件名无效"} }
func errBackupSource() error { return &backupError{"源路径不在允许范围内"} }
func errBackupTarget() error { return &backupError{"无法确定恢复目标路径"} }

type backupError struct{ message string }

func (e *backupError) Error() string { return e.message }

func createTarGzFromReader(name string, r io.Reader, w io.Writer) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = tw.Write(data)
	return err
}
