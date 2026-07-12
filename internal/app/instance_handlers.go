package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleInstanceCreate(w http.ResponseWriter, r *http.Request) {
	var body Instance
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.InstanceType == "minecraft-bedrock" {
		if body.StartCommand == "" {
			body.StartCommand = "./bedrock_server"
		}
		body.StopCommand = "stop"
	}
	if body.InstanceType == "minecraft-java" {
		body.StopCommand = "stop"
	}
	inst, err := s.instances.Create(body)
	writeJSON(w, map[string]any{"ok": err == nil, "instance": inst, "error": errString(err)})
}

func (s *Server) handleInstanceUpdate(w http.ResponseWriter, r *http.Request) {
	var body Instance
	_ = json.NewDecoder(r.Body).Decode(&body)
	inst, err := s.instances.Update(body.ID, body)
	writeJSON(w, map[string]any{"ok": err == nil, "instance": inst, "error": errString(err)})
}

func (s *Server) handleInstanceDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	// 先停止运行中的实例
	s.stopInstanceByID(body.ID)
	err := s.instances.Delete(body.ID)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleInstanceAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		inst, ok := s.instances.Get(body.ID)
		if !ok {
			writeError(w, http.StatusNotFound, "实例不存在")
			return
		}
		switch action {
		case "start":
			s.instanceStart(w, r, inst)
		case "stop":
			s.instanceStop(w, r, inst)
		case "restart":
			s.instanceStop(w, r, inst)
			time.Sleep(2 * time.Second)
			inst, _ = s.instances.Get(body.ID)
			s.instanceStart(w, r, inst)
		}
	}
}

func (s *Server) handleInstanceInput(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	err := s.runtime.Input(body.ID, body.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleInstanceStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	inst, ok := s.instances.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "实例不存在")
		return
	}
	if svc, ok := instanceSystemdService(inst); ok {
		status := "stopped"
		if systemdActive(svc) {
			status = "running"
		}
		s.instances.SetStatus(inst.ID, status)
		inst.Status = status
	}
	writeJSON(w, map[string]any{"status": inst.Status, "pid": inst.PID, "lastStarted": inst.LastStarted, "lastStopped": inst.LastStopped})
}

func (s *Server) handleInstanceLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	logs := s.runtime.Logs(id, r.URL.Query().Get("tail") != "")
	writeJSON(w, map[string]any{"logs": logs})
}

func tailLines(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}

func (s *Server) instanceStart(w http.ResponseWriter, r *http.Request, inst Instance) {
	result, err := s.runtime.Start(inst)
	if err != nil {
		if runtimeErr, ok := err.(*instanceRuntimeError); ok {
			writeError(w, runtimeErr.code, runtimeErr.message)
			return
		}
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "pid": result.PID, "logFile": result.LogFile})
}

func (s *Server) instanceStop(w http.ResponseWriter, r *http.Request, inst Instance) {
	writeJSON(w, map[string]any{"ok": true, "message": s.runtime.Stop(inst)})
}

func (s *Server) stopInstanceByID(id string) {
	inst, ok := s.instances.Get(id)
	if !ok || inst.Status != "running" {
		return
	}
	s.runtime.Stop(inst)
}

// AutoStartInstances 在服务启动时自动启动标记了 AutoStart 的实例
func (s *Server) AutoStartInstances() {
	for _, inst := range s.instances.List() {
		if inst.AutoStart && inst.Status != "running" {
			go func(item Instance) { _, _ = s.runtime.Start(item) }(inst)
			time.Sleep(2 * time.Second)
		}
	}
}

// SetFields 批量设置实例字段
func (s *InstanceStore) SetFields(id string, fields map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		if v, ok := fields["status"]; ok {
			s.list[i].Status = fmt.Sprint(v)
		}
		if v, ok := fields["pid"]; ok {
			if n, ok := v.(int); ok {
				s.list[i].PID = n
			}
		}
		if v, ok := fields["terminalSession"]; ok {
			s.list[i].TerminalSession = fmt.Sprint(v)
		}
		if v, ok := fields["lastStarted"]; ok {
			s.list[i].LastStarted = fmt.Sprint(v)
		}
		if v, ok := fields["lastStopped"]; ok {
			s.list[i].LastStopped = fmt.Sprint(v)
		}
		break
	}
	_ = s.saveLocked()
}
