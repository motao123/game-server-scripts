package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// 运行中的实例进程追踪
var (
	runningProcs   = map[string]*exec.Cmd{}
	runningStdins  = map[string]func(string) error{}
	runningProcsMu sync.Mutex
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
	runningProcsMu.Lock()
	writer := runningStdins[body.ID]
	runningProcsMu.Unlock()
	if writer == nil {
		writeError(w, http.StatusBadRequest, "实例未运行或无标准输入")
		return
	}
	err := writer(body.Data)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleInstanceStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	inst, ok := s.instances.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "实例不存在")
		return
	}
	writeJSON(w, map[string]any{"status": inst.Status, "pid": inst.PID, "lastStarted": inst.LastStarted, "lastStopped": inst.LastStopped})
}

func (s *Server) handleInstanceLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	logFile := fmt.Sprintf("data/instances/%s.log", id)
	data, err := os.ReadFile(logFile)
	if err != nil {
		writeJSON(w, map[string]any{"logs": ""})
		return
	}
	writeJSON(w, map[string]any{"logs": string(data)})
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

	// 用后台进程启动，保存 PID，输出写入日志文件
	c := exec.Command("sh", "-lc", cmd)
	c.Dir = inst.WorkingDirectory
	logFile := fmt.Sprintf("data/instances/%s.log", inst.ID)
	_ = os.MkdirAll("data/instances", 0755)
	f, err := os.Create(logFile)
	if err != nil {
		s.instances.SetStatus(inst.ID, "error")
		writeError(w, 500, "无法创建日志文件: "+err.Error())
		return
	}
	c.Stdout = f
	c.Stderr = f
	c.Stdin = nil
	setSysProcAttr(c)
	stdin, err := c.StdinPipe()
	if err != nil {
		f.Close()
		s.instances.SetStatus(inst.ID, "error")
		writeError(w, 500, "无法创建 stdin pipe: "+err.Error())
		return
	}

	if err := c.Start(); err != nil {
		f.Close()
		s.instances.SetStatus(inst.ID, "error")
		writeError(w, 500, "启动失败: "+err.Error())
		return
	}

	runningProcsMu.Lock()
	runningProcs[inst.ID] = c
	runningStdins[inst.ID] = func(data string) error { _, err := stdin.Write([]byte(data)); return err }
	runningProcsMu.Unlock()

	s.instances.SetFields(inst.ID, map[string]any{
		"status":          "running",
		"pid":             c.Process.Pid,
		"lastStarted":     time.Now().Format(time.RFC3339),
		"terminalSession": "",
	})

	// 后台等待进程退出
	go func() {
		_ = c.Wait()
		f.Close()
		runningProcsMu.Lock()
		delete(runningProcs, inst.ID)
		runningProcsMu.Unlock()
		cur, ok := s.instances.Get(inst.ID)
		if ok && cur.Status != "stopped" {
			s.instances.SetFields(inst.ID, map[string]any{
				"status":      "stopped",
				"pid":         0,
				"lastStopped": time.Now().Format(time.RFC3339),
			})
		}
	}()

	writeJSON(w, map[string]any{"ok": true, "pid": c.Process.Pid, "logFile": logFile})
}

func (s *Server) instanceStop(w http.ResponseWriter, r *http.Request, inst Instance) {
	if inst.Status == "stopped" {
		writeJSON(w, map[string]any{"ok": true, "message": "已停止"})
		return
	}
	s.instances.SetStatus(inst.ID, "stopping")

	runningProcsMu.Lock()
	cmd := runningProcs[inst.ID]
	runningProcsMu.Unlock()

	if cmd != nil && cmd.Process != nil {
		// 先尝试向进程组发 SIGINT（模拟 ctrl+c）
		stopProcess(cmd)
		// 等 10 秒，超时强制 kill
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			killProcess(cmd)
		}
		runningProcsMu.Lock()
		delete(runningProcs, inst.ID)
		delete(runningStdins, inst.ID)
		runningProcsMu.Unlock()
	}

	s.instances.SetFields(inst.ID, map[string]any{
		"status":      "stopped",
		"pid":         0,
		"lastStopped": time.Now().Format(time.RFC3339),
	})
	writeJSON(w, map[string]any{"ok": true, "message": "已停止"})
}

func (s *Server) stopInstanceByID(id string) {
	inst, ok := s.instances.Get(id)
	if !ok || inst.Status != "running" {
		return
	}
	s.instanceStop(nil, nil, inst)
}

// AutoStartInstances 在服务启动时自动启动标记了 AutoStart 的实例
func (s *Server) AutoStartInstances() {
	for _, inst := range s.instances.List() {
		if inst.AutoStart && inst.Status != "running" {
			go s.instanceStart(nil, nil, inst)
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
