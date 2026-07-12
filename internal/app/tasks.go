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
	UpdatedAt string `json:"updatedAt,omitempty"`
	LastRun   string `json:"lastRun,omitempty"`
	LastError string `json:"lastError,omitempty"`
	NextRun   string `json:"nextRun,omitempty"`
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
	return s.saveLocked()
}
func (s *TaskStore) List() []ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ScheduledTask, len(s.list))
	copy(out, s.list)
	return out
}
func (s *TaskStore) Get(id string) (ScheduledTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.list {
		if task.ID == id {
			return task, true
		}
	}
	return ScheduledTask{}, false
}

func (s *TaskStore) Upsert(task ScheduledTask) (ScheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	if task.ID == "" {
		task.ID = time.Now().Format("20060102150405")
		task.CreatedAt = now
	}
	if task.CreatedAt == "" {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	for i := range s.list {
		if s.list[i].ID == task.ID {
			task.LastRun = s.list[i].LastRun
			task.LastError = s.list[i].LastError
			task.NextRun = s.list[i].NextRun
			s.list[i] = task
			return task, s.saveLocked()
		}
	}
	s.list = append(s.list, task)
	return task, s.saveLocked()
}

func (s *TaskStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.list[:0]
	for _, task := range s.list {
		if task.ID != id {
			filtered = append(filtered, task)
		}
	}
	s.list = filtered
	return s.saveLocked()
}

func (s *TaskStore) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list[i].Enabled = enabled
			s.list[i].UpdatedAt = time.Now().Format(time.RFC3339)
			return s.saveLocked()
		}
	}
	return nil
}

func (s *TaskStore) SetRunResult(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list[i].LastRun = time.Now().Format(time.RFC3339)
			if err != nil {
				s.list[i].LastError = err.Error()
			} else {
				s.list[i].LastError = ""
			}
			_ = s.saveLocked()
			return
		}
	}
}

func (s *TaskStore) SetNextRun(id, nextRun string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list[i].NextRun = nextRun
			_ = s.saveLocked()
			return
		}
	}
}

func (s *TaskStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.list, "", "  ")
	return os.WriteFile(s.path, data, 0644)
}

func NewTask(name, cronExpr, action string) ScheduledTask {
	now := time.Now().Format(time.RFC3339)
	return ScheduledTask{ID: time.Now().Format("20060102150405"), Name: name, Cron: cronExpr, Action: action, Enabled: true, CreatedAt: now, UpdatedAt: now}
}
