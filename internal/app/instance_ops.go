package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *InstanceStore) Create(req Instance) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(req.Name) == "" {
		return Instance{}, fmt.Errorf("实例名称不能为空")
	}
	req.ID = uuid.NewString()
	req.Status = "stopped"
	req.CreatedAt = time.Now().Format(time.RFC3339)
	s.list = append(s.list, req)
	return req, s.saveLocked()
}

func (s *InstanceStore) Update(id string, req Instance) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			req.ID = id
			if req.CreatedAt == "" {
				req.CreatedAt = s.list[i].CreatedAt
			}
			s.list[i] = req
			return req, s.saveLocked()
		}
	}
	return Instance{}, fmt.Errorf("实例不存在")
}

func (s *InstanceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.list[:0]
	for _, item := range s.list {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == len(s.list) {
		return fmt.Errorf("实例不存在")
	}
	s.list = filtered
	return s.saveLocked()
}

func (s *InstanceStore) Get(id string) (Instance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.list {
		if item.ID == id {
			return item, true
		}
	}
	return Instance{}, false
}

func (s *InstanceStore) SetStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			s.list[i].Status = status
			break
		}
	}
	_ = s.saveLocked()
}

func (s *InstanceStore) saveLocked() error {
	data, _ := json.MarshalIndent(s.list, "", "  ")
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func runInstanceCommand(inst Instance, command string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", command)
		cmd.Dir = inst.WorkingDirectory
		return cmd.CombinedOutput()
	}
	cmd := exec.Command("sh", "-lc", command)
	cmd.Dir = inst.WorkingDirectory
	return cmd.CombinedOutput()
}
