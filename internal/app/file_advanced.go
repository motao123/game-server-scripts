package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

var (
	favoritesMu sync.Mutex
	favorites   = []string{}
)

func (s *Server) handleFilesCopy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Src       string `json:"src"`
		Dst       string `json:"dst"`
		Overwrite bool   `json:"overwrite"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Src) || !s.safeRoot(filepath.Dir(body.Dst)) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	if err := ensureNoFileConflict(body.Dst, body.Overwrite); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	task := s.fileTasks.Add("copy", filepath.Base(body.Src), func(update func(int, string)) error {
		update(10, "复制中")
		if err := ensureNoFileConflict(body.Dst, body.Overwrite); err != nil {
			return err
		}
		return copyPath(body.Src, body.Dst)
	})
	writeJSON(w, map[string]any{"ok": true, "task": task})
}

func (s *Server) handleFilesMove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Src       string `json:"src"`
		Dst       string `json:"dst"`
		Overwrite bool   `json:"overwrite"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Src) || !s.safeRoot(filepath.Dir(body.Dst)) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	if err := ensureNoFileConflict(body.Dst, body.Overwrite); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	task := s.fileTasks.Add("move", filepath.Base(body.Src), func(update func(int, string)) error {
		update(20, "移动中")
		if err := ensureNoFileConflict(body.Dst, body.Overwrite); err != nil {
			return err
		}
		return os.Rename(body.Src, body.Dst)
	})
	writeJSON(w, map[string]any{"ok": true, "task": task})
}

func (s *Server) handleFilesPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		path := r.URL.Query().Get("path")
		if !s.safeRoot(path) {
			writeError(w, 403, "路径不允许访问")
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"mode": fmt.Sprintf("%04o", info.Mode().Perm()), "isDir": info.IsDir()})
		return
	}
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Path) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	var mode uint32
	fmt.Sscanf(body.Mode, "%o", &mode)
	err := os.Chmod(body.Path, os.FileMode(mode))
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	favoritesMu.Lock()
	defer favoritesMu.Unlock()
	if r.Method == http.MethodPost {
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, f := range favorites {
			if f == body.Path {
				writeJSON(w, map[string]any{"ok": true})
				return
			}
		}
		favorites = append(favorites, body.Path)
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		path := r.URL.Query().Get("path")
		filtered := favorites[:0]
		for _, f := range favorites {
			if f != path {
				filtered = append(filtered, f)
			}
		}
		favorites = filtered
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	writeJSON(w, map[string]any{"favorites": favorites})
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src, dst string, mode os.FileMode) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()
	if _, err := io.Copy(df, sf); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			info, _ := entry.Info()
			if err := copyFile(s, d, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
