package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type DeployTaskStore struct {
	path string
	mu   sync.Mutex
}

func NewDeployTaskStore(path string) *DeployTaskStore {
	return &DeployTaskStore{path: path}
}

func (s *DeployTaskStore) Load() ([]DeployTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var tasks []DeployTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *DeployTaskStore) Save(tasks []DeployTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
