package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type FileSearchItem struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

func (s *Server) handleFilesConflicts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []string `json:"items"`
		Dest  string   `json:"dest"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Dest) {
		writeError(w, http.StatusForbidden, "路径不允许访问")
		return
	}
	conflicts := []string{}
	for _, src := range body.Items {
		if !s.safeRoot(src) {
			writeError(w, http.StatusForbidden, "路径不允许访问")
			return
		}
		target := filepath.Join(body.Dest, filepath.Base(src))
		if _, err := os.Stat(target); err == nil {
			conflicts = append(conflicts, target)
		} else if err != nil && !os.IsNotExist(err) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, map[string]any{"ok": true, "conflicts": conflicts})
}

func (s *Server) handleFilesSearch(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("path")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if root == "" {
		root = s.defaultFileRoot()
	}
	if !s.safeRoot(root) {
		writeError(w, http.StatusForbidden, "路径不允许访问")
		return
	}
	if query == "" {
		writeError(w, http.StatusBadRequest, "搜索关键字不能为空")
		return
	}
	items, truncated, err := searchFiles(root, query, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "items": items, "truncated": truncated})
}

func searchFiles(root, query string, limit int) ([]FileSearchItem, bool, error) {
	root = filepath.Clean(root)
	query = strings.ToLower(query)
	items := []FileSearchItem{}
	truncated := false
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		if len(items) >= limit {
			truncated = true
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(strings.ToLower(entry.Name()), query) {
			info, _ := entry.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			items = append(items, FileSearchItem{Name: entry.Name(), Path: path, IsDir: entry.IsDir(), Size: size})
		}
		return nil
	})
	return items, truncated, err
}

func ensureNoFileConflict(dst string, overwrite bool) error {
	if overwrite {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("目标已存在: %s", dst)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
