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
	"sort"
	"strings"
	"time"
)

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

func (s *Server) handleBackupGroups(w http.ResponseWriter, r *http.Request) {
	root := s.cfg.BackupDir
	entries, err := os.ReadDir(root)
	if err != nil {
		writeJSON(w, map[string]any{"groups": []BackupGroup{}})
		return
	}
	var groups []BackupGroup
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		group := BackupGroup{Name: entry.Name()}
		groupDir := filepath.Join(root, entry.Name())
		meta, _ := os.ReadFile(filepath.Join(groupDir, "data.json"))
		if strings.Contains(string(meta), "sourcePath") {
			group.Source = extractJSONValue(string(meta), "sourcePath")
		}
		files, _ := os.ReadDir(groupDir)
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".tar.gz") {
				continue
			}
			info, _ := f.Info()
			group.Files = append(group.Files, BackupEntry{Name: f.Name(), Size: info.Size(), Time: info.ModTime().Unix()})
			group.Total += info.Size()
		}
		groups = append(groups, group)
	}
	writeJSON(w, map[string]any{"groups": groups})
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
	if !s.safeRoot(body.SourcePath) && body.SourcePath != s.cfg.PalServerDir {
		writeError(w, 403, "源路径不在允许范围内")
		return
	}
	groupDir := filepath.Join(s.cfg.BackupDir, body.BackupName)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	meta := fmt.Sprintf(`{"sourcePath":"%s"}`, body.SourcePath)
	_ = os.WriteFile(filepath.Join(groupDir, "data.json"), []byte(meta), 0644)
	ts := time.Now().Format("2006-01-02_15-04-05")
	archive := filepath.Join(groupDir, ts+".tar.gz")
	if err := createTarGz(body.SourcePath, archive); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if body.MaxKeep > 0 {
		enforceRetention(groupDir, body.MaxKeep)
	}
	writeJSON(w, map[string]any{"ok": true, "archive": archive})
}

func (s *Server) handleBackupRestoreGeneric(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		FileName   string `json:"fileName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	groupDir := filepath.Join(s.cfg.BackupDir, body.BackupName)
	meta, _ := os.ReadFile(filepath.Join(groupDir, "data.json"))
	sourcePath := extractJSONValue(string(meta), "sourcePath")
	if sourcePath == "" {
		writeError(w, 400, "无法确定恢复目标路径")
		return
	}
	archive := filepath.Join(groupDir, body.FileName)
	if err := extractTarGz(archive, filepath.Dir(sourcePath)); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "targetPath": sourcePath})
}

func (s *Server) handleBackupDeleteFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BackupName string `json:"backupName"`
		FileName   string `json:"fileName"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	path := filepath.Join(s.cfg.BackupDir, body.BackupName, body.FileName)
	if !s.safeRoot(path) {
		writeError(w, 403, "路径不允许")
		return
	}
	err := os.Remove(path)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
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
