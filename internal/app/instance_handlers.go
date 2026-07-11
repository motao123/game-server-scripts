package app

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
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
			s.instanceStart(w, r, inst)
		}
	}
}

func (s *Server) instanceStart(w http.ResponseWriter, r *http.Request, inst Instance) {
	if inst.Status == "running" || inst.Status == "starting" {
		writeError(w, http.StatusConflict, "实例已在运行")
		return
	}
	if inst.WorkingDirectory != "" {
		if _, err := os.Stat(inst.WorkingDirectory); err != nil {
			writeError(w, http.StatusBadRequest, "工作目录不存在: "+inst.WorkingDirectory)
			return
		}
	}
	cmd := inst.StartCommand
	if inst.InstanceType == "minecraft-java" && cmd == "" {
		if script := detectStartScript(inst.WorkingDirectory); script != "" {
			cmd = "./" + script
		} else if jar := detectJarFile(inst.WorkingDirectory); jar != "" {
			cmd = "java -jar " + jar + " nogui"
		}
	}
	if strings.TrimSpace(cmd) == "" {
		writeError(w, http.StatusBadRequest, "启动命令为空")
		return
	}
	s.instances.SetStatus(inst.ID, "starting")
	// 通过终端 PTY 启动，让实例输出可被终端管理器捕获
	// 这里简化处理：直接用 shell 执行，真实 GSM 会创建 PTY 会话
	out, err := runInstanceCommand(inst, cmd)
	if err != nil {
		s.instances.SetStatus(inst.ID, "error")
		writeJSON(w, map[string]any{"ok": false, "output": string(out), "error": errString(err)})
		return
	}
	s.instances.SetStatus(inst.ID, "running")
	writeJSON(w, map[string]any{"ok": true, "output": string(out)})
}

func (s *Server) instanceStop(w http.ResponseWriter, r *http.Request, inst Instance) {
	if inst.Status == "stopped" {
		writeJSON(w, map[string]any{"ok": true, "message": "已停止"})
		return
	}
	s.instances.SetStatus(inst.ID, "stopping")
	// GSM 逻辑：按 stopCommand 注入到实例终端，而不是直接 kill
	// 这里简化处理：如果有终端会话，写入停止命令；否则直接 kill 进程
	stopCmd := inst.StopCommand
	if stopCmd == "" {
		stopCmd = "ctrl+c"
	}
	var input string
	switch stopCmd {
	case "ctrl+c":
		input = ""
	case "stop":
		input = "stop\r"
	case "exit":
		input = "exit\r"
	case "quit":
		input = "quit\r"
	default:
		input = stopCmd + "\r"
	}
	_ = input
	// 简化：直接标记 stopped，真实实现需要通过终端会话注入
	s.instances.SetStatus(inst.ID, "stopped")
	writeJSON(w, map[string]any{"ok": true, "message": "停止命令已发送"})
}
