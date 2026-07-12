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

func (s *InstanceStore) Create(req Instance) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeInstanceDefaults(&req)
	if err := validateInstanceForSave(req); err != nil {
		return Instance{}, err
	}
	req.ID = uuid.NewString()
	req.CreatedAt = time.Now().Format(time.RFC3339)
	s.list = append(s.list, req)
	return req, s.saveLocked()
}

func (s *InstanceStore) Update(id string, req Instance) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			normalizeInstanceDefaults(&req)
			if err := validateInstanceForSave(req); err != nil {
				return Instance{}, err
			}
			req.ID = id
			if req.CreatedAt == "" {
				req.CreatedAt = s.list[i].CreatedAt
			}
			if req.StopCommand == "" {
				req.StopCommand = "ctrl+c"
			}
			if req.InstanceType == "" {
				req.InstanceType = "generic"
			}
			s.list[i] = req
			return req, s.saveLocked()
		}
	}
	return Instance{}, fmt.Errorf("实例不存在")
}

func normalizeInstanceDefaults(req *Instance) {
	req.Name = strings.TrimSpace(req.Name)
	req.WorkingDirectory = strings.TrimSpace(req.WorkingDirectory)
	req.StartCommand = strings.TrimSpace(req.StartCommand)
	req.StopCommand = strings.TrimSpace(req.StopCommand)
	req.InstanceType = strings.TrimSpace(req.InstanceType)
	if req.Status == "" {
		req.Status = "stopped"
	}
	if req.StopCommand == "" {
		req.StopCommand = "ctrl+c"
	}
	if req.InstanceType == "" {
		req.InstanceType = "generic"
	}
}

func validateInstanceForSave(req Instance) error {
	if req.Name == "" {
		return fmt.Errorf("实例名称不能为空")
	}
	if req.WorkingDirectory == "" {
		return fmt.Errorf("工作目录不能为空")
	}
	if req.InstanceType != "minecraft-java" && req.StartCommand == "" {
		return fmt.Errorf("启动命令不能为空")
	}
	if strings.ContainsAny(req.WorkingDirectory, "\x00") || strings.ContainsAny(req.StartCommand, "\x00\r\n") {
		return fmt.Errorf("实例参数包含非法字符")
	}
	return nil
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

func detectStartScript(workingDirectory string) string {
	if workingDirectory == "" {
		return ""
	}
	entries, err := os.ReadDir(workingDirectory)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	scripts := []string{"start.sh", "run.sh"}
	if runtime.GOOS == "windows" {
		scripts = []string{"start.bat", "run.bat", "start.cmd", "run.cmd"}
	}
	for _, s := range scripts {
		for _, n := range names {
			if n == s {
				return s
			}
		}
	}
	return ""
}

func detectJarFile(workingDirectory string) string {
	if workingDirectory == "" {
		return ""
	}
	entries, err := os.ReadDir(workingDirectory)
	if err != nil {
		return ""
	}
	var jars []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
			jars = append(jars, e.Name())
		}
	}
	if len(jars) == 0 {
		return ""
	}
	if len(jars) == 1 {
		return jars[0]
	}
	for _, j := range jars {
		if strings.Contains(strings.ToLower(j), "server") {
			return j
		}
	}
	return jars[0]
}
