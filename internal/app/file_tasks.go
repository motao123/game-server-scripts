package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileTask struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type FileTaskManager struct {
	mu    sync.Mutex
	tasks []FileTask
	queue chan fileTaskWork
}

type fileTaskWork struct {
	id string
	fn func(update func(int, string)) error
}

func NewFileTaskManager() *FileTaskManager {
	m := &FileTaskManager{queue: make(chan fileTaskWork, 64)}
	go m.run()
	return m
}

func (m *FileTaskManager) Add(taskType, name string, fn func(update func(int, string)) error) FileTask {
	now := time.Now().Format(time.RFC3339)
	task := FileTask{ID: time.Now().Format("20060102150405.000000000"), Type: taskType, Name: name, Status: "queued", CreatedAt: now, UpdatedAt: now}
	m.mu.Lock()
	m.tasks = append([]FileTask{task}, m.tasks...)
	if len(m.tasks) > 100 {
		m.tasks = m.tasks[:100]
	}
	m.mu.Unlock()
	m.queue <- fileTaskWork{id: task.ID, fn: fn}
	return task
}

func (m *FileTaskManager) List() []FileTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]FileTask, len(m.tasks))
	copy(out, m.tasks)
	return out
}

func (m *FileTaskManager) run() {
	for work := range m.queue {
		m.update(work.id, "running", 1, "", "")
		err := work.fn(func(progress int, message string) { m.update(work.id, "running", progress, message, "") })
		if err != nil {
			m.update(work.id, "error", 100, "", err.Error())
			continue
		}
		m.update(work.id, "done", 100, "完成", "")
	}
}

func (m *FileTaskManager) update(id, status string, progress int, message, errText string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.tasks {
		if m.tasks[i].ID == id {
			m.tasks[i].Status = status
			m.tasks[i].Progress = progress
			m.tasks[i].Message = message
			m.tasks[i].Error = errText
			m.tasks[i].UpdatedAt = time.Now().Format(time.RFC3339)
			return
		}
	}
}

func (s *Server) handleFileTasks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"tasks": s.fileTasks.List()})
}

func (s *Server) handleFilesUploadChunk(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if !s.safeRoot(dir) {
		writeError(w, http.StatusForbidden, "路径不允许访问")
		return
	}
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	uploadID := safeUploadName(r.FormValue("uploadId"))
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "uploadId 不能为空")
		return
	}
	index, err := strconv.Atoi(r.FormValue("chunkIndex"))
	if err != nil || index < 0 {
		writeError(w, http.StatusBadRequest, "chunkIndex 无效")
		return
	}
	file, _, err := r.FormFile("chunk")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()
	chunkDir := filepath.Join(s.cfg.DataDir, "upload_chunks", uploadID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out, err := os.Create(filepath.Join(chunkDir, fmt.Sprintf("%06d.part", index)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	n, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil {
		writeError(w, http.StatusInternalServerError, copyErr.Error())
		return
	}
	if closeErr != nil {
		writeError(w, http.StatusInternalServerError, closeErr.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "uploadId": uploadID, "chunkIndex": index, "size": n})
}

func (s *Server) handleFilesUploadComplete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path        string `json:"path"`
		UploadID    string `json:"uploadId"`
		FileName    string `json:"fileName"`
		TotalChunks int    `json:"totalChunks"`
		Overwrite   bool   `json:"overwrite"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.Path) {
		writeError(w, http.StatusForbidden, "路径不允许访问")
		return
	}
	uploadID := safeUploadName(body.UploadID)
	if uploadID == "" || body.TotalChunks <= 0 {
		writeError(w, http.StatusBadRequest, "上传参数无效")
		return
	}
	fileName := filepath.Base(body.FileName)
	target := filepath.Join(body.Path, fileName)
	if !s.safeRoot(filepath.Dir(target)) {
		writeError(w, http.StatusForbidden, "路径不允许访问")
		return
	}
	if err := ensureNoFileConflict(target, body.Overwrite); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	task := s.fileTasks.Add("upload", fileName, func(update func(int, string)) error {
		if err := ensureNoFileConflict(target, body.Overwrite); err != nil {
			return err
		}
		return mergeUploadChunks(filepath.Join(s.cfg.DataDir, "upload_chunks", uploadID), target, body.TotalChunks, update)
	})
	writeJSON(w, map[string]any{"ok": true, "task": task})
}

func safeUploadName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func mergeUploadChunks(chunkDir, target string, total int, update func(int, string)) error {
	tmp := target + ".uploading"
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for i := 0; i < total; i++ {
		part := filepath.Join(chunkDir, fmt.Sprintf("%06d.part", i))
		in, err := os.Open(part)
		if err != nil {
			out.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := in.Close()
		if copyErr != nil {
			out.Close()
			return copyErr
		}
		if closeErr != nil {
			out.Close()
			return closeErr
		}
		update(5+int(float64(i+1)/float64(total)*90), "合并分片")
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	return os.RemoveAll(chunkDir)
}
