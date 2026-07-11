package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ScheduledTask struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cron      string `json:"cron"`
	Action    string `json:"action"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
}

type TaskStore struct {
	path string
	mu   sync.Mutex
	list []ScheduledTask
}

func NewTaskStore(path string) *TaskStore { s := &TaskStore{path: path}; _ = s.Load(); return s }
func (s *TaskStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &s.list)
}
func (s *TaskStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.list, "", "  ")
	return os.WriteFile(s.path, data, 0644)
}
func (s *TaskStore) List() []ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ScheduledTask, len(s.list))
	copy(out, s.list)
	return out
}
func NewTask(name, cron, action string) ScheduledTask {
	return ScheduledTask{ID: time.Now().Format("20060102150405"), Name: name, Cron: cron, Action: action, Enabled: true, CreatedAt: time.Now().Format(time.RFC3339)}
}
