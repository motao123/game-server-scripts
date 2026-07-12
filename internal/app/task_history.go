package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TaskHistoryEntry struct {
	Time     string `json:"time"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration"`
}

type TaskHistoryStore struct {
	path string
	mu   sync.Mutex
	data map[string][]TaskHistoryEntry
}

func NewTaskHistoryStore(path string) *TaskHistoryStore {
	s := &TaskHistoryStore{path: path, data: map[string][]TaskHistoryEntry{}}
	_ = s.load()
	return s
}

func (s *TaskHistoryStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &s.data)
}

func (s *TaskHistoryStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.data, "", "  ")
	return os.WriteFile(s.path, data, 0644)
}

func (s *TaskHistoryStore) Add(taskID string, entry TaskHistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[taskID] = append(s.data[taskID], entry)
	if len(s.data[taskID]) > 20 {
		s.data[taskID] = s.data[taskID][len(s.data[taskID])-20:]
	}
	_ = s.save()
}

func (s *TaskHistoryStore) Get(taskID string) []TaskHistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskHistoryEntry, len(s.data[taskID]))
	copy(out, s.data[taskID])
	return out
}

func (s *TaskHistoryStore) Delete(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, taskID)
	_ = s.save()
}

func runWithHistory(store *TaskHistoryStore, taskID string, fn func() error) error {
	start := time.Now()
	err := fn()
	store.Add(taskID, TaskHistoryEntry{
		Time:     start.Format(time.RFC3339),
		Success:  err == nil,
		Error:    errString(err),
		Duration: time.Since(start).Round(time.Millisecond).String(),
	})
	return err
}
