package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"game-server-scripts/internal/rcon"
)

type RconConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Timeout  int    `json:"timeout"`
}

type RconConnection struct {
	Config    RconConfig
	Client    *rcon.Client
	Connected bool
}

var (
	rconMu    sync.Mutex
	rconConns = map[string]*RconConnection{}
)

func rconConfigPath(instanceID string) string {
	return filepath.Join("data", "rcon", instanceID+".json")
}

func loadRconConfig(instanceID string) (RconConfig, error) {
	data, err := os.ReadFile(rconConfigPath(instanceID))
	if err != nil {
		return RconConfig{}, err
	}
	var cfg RconConfig
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveRconConfig(instanceID string, cfg RconConfig) error {
	if err := os.MkdirAll(filepath.Dir(rconConfigPath(instanceID)), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(rconConfigPath(instanceID), data, 0644)
}

func (s *Server) handleRconConfig(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instanceId")
	if instanceID == "" {
		writeError(w, 400, "instanceId 不能为空")
		return
	}
	cfg, err := loadRconConfig(instanceID)
	if err != nil {
		cfg = RconConfig{Host: "127.0.0.1", Port: 25575, Timeout: 5}
	}
	cfg.Password = ""
	writeJSON(w, cfg)
}

func (s *Server) handleRconConfigSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string     `json:"instanceId"`
		Config     RconConfig `json:"config"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.InstanceID == "" {
		writeError(w, 400, "instanceId 不能为空")
		return
	}
	err := saveRconConfig(body.InstanceID, body.Config)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleRconConnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string `json:"instanceId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	cfg, err := loadRconConfig(body.InstanceID)
	if err != nil {
		writeError(w, 400, "未找到 RCON 配置")
		return
	}
	client := &rcon.Client{Addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), Password: cfg.Password}
	_, cmdErr := client.Command("Info")
	if cmdErr != nil {
		writeError(w, 502, "RCON 连接失败: "+cmdErr.Error())
		return
	}
	rconMu.Lock()
	rconConns[body.InstanceID] = &RconConnection{Config: cfg, Client: client, Connected: true}
	rconMu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "message": "已连接"})
}

func (s *Server) handleRconDisconnect(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string `json:"instanceId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	rconMu.Lock()
	delete(rconConns, body.InstanceID)
	rconMu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleRconStatus(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instanceId")
	rconMu.Lock()
	conn := rconConns[instanceID]
	rconMu.Unlock()
	if conn == nil {
		writeJSON(w, map[string]any{"connected": false})
		return
	}
	writeJSON(w, map[string]any{"connected": conn.Connected, "host": conn.Config.Host, "port": conn.Config.Port})
}

func (s *Server) handleRconCommandInstance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string `json:"instanceId"`
		Command    string `json:"command"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	rconMu.Lock()
	conn := rconConns[body.InstanceID]
	rconMu.Unlock()
	if conn == nil {
		writeError(w, 400, "RCON 未连接，请先连接")
		return
	}
	out, err := conn.Client.Command(body.Command)
	writeJSON(w, map[string]any{"ok": err == nil, "response": out, "error": errString(err)})
}
