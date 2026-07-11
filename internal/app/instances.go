package app

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Instance struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	WorkingDirectory string `json:"workingDirectory"`
	StartCommand     string `json:"startCommand"`
	StopCommand      string `json:"stopCommand"`
	AutoStart        bool   `json:"autoStart"`
	Status           string `json:"status"`
	InstanceType     string `json:"instanceType"`
	CreatedAt        string `json:"createdAt"`
}

type InstanceStore struct {
	path string
	mu   sync.Mutex
	list []Instance
}

func NewInstanceStore(path string) *InstanceStore {
	s := &InstanceStore{path: path}
	_ = s.Load()
	if len(s.list) == 0 {
		s.list = append(s.list, Instance{ID: "palworld-default", Name: "Palworld", Description: "现有 pal-server systemd 服务", WorkingDirectory: "/home/steam/Steam/steamapps/common/PalServer", StartCommand: "systemctl start pal-server", StopCommand: "stop", Status: "unknown", InstanceType: "palworld", CreatedAt: time.Now().Format(time.RFC3339)})
	}
	return s
}

func (s *InstanceStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	return json.Unmarshal(data, &s.list)
}

func (s *InstanceStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *InstanceStore) List() []Instance {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Instance, len(s.list))
	copy(out, s.list)
	return out
}
