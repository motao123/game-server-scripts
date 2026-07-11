package app

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleInstanceCreate(w http.ResponseWriter, r *http.Request) {
	var body Instance
	_ = json.NewDecoder(r.Body).Decode(&body)
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
		cmd := inst.StartCommand
		if action == "stop" {
			cmd = inst.StopCommand
		}
		if action == "restart" {
			_, _ = runInstanceCommand(inst, inst.StopCommand)
			cmd = inst.StartCommand
		}
		if strings.TrimSpace(cmd) == "" {
			writeError(w, http.StatusBadRequest, "命令为空")
			return
		}
		out, err := runInstanceCommand(inst, cmd)
		if err == nil {
			if action == "stop" {
				s.instances.SetStatus(inst.ID, "stopped")
			} else {
				s.instances.SetStatus(inst.ID, "running")
			}
		}
		writeJSON(w, map[string]any{"ok": err == nil, "output": string(out), "error": errString(err)})
	}
}
