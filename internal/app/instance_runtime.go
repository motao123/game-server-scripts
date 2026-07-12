package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type InstanceRuntime struct {
	store  *InstanceStore
	mu     sync.Mutex
	procs  map[string]*exec.Cmd
	stdins map[string]func(string) error
}

type InstanceStartResult struct {
	PID     int    `json:"pid"`
	LogFile string `json:"logFile"`
}

func NewInstanceRuntime(store *InstanceStore) *InstanceRuntime {
	return &InstanceRuntime{store: store, procs: map[string]*exec.Cmd{}, stdins: map[string]func(string) error{}}
}

func (r *InstanceRuntime) Start(inst Instance) (InstanceStartResult, error) {
	if inst.Status == "running" || inst.Status == "starting" {
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusConflict, message: "实例已在运行"}
	}
	if inst.WorkingDirectory != "" {
		if _, err := os.Stat(inst.WorkingDirectory); err != nil {
			return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusBadRequest, message: "工作目录不存在: " + inst.WorkingDirectory}
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
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusBadRequest, message: "启动命令为空"}
	}
	if svc, ok := systemctlStartService(cmd); ok {
		return r.startSystemd(inst, svc)
	}
	r.store.SetStatus(inst.ID, "starting")

	c := exec.Command("sh", "-lc", cmd)
	c.Dir = inst.WorkingDirectory
	logFile := fmt.Sprintf("data/instances/%s.log", inst.ID)
	_ = os.MkdirAll("data/instances", 0755)
	f, err := os.Create(logFile)
	if err != nil {
		r.store.SetStatus(inst.ID, "error")
		return InstanceStartResult{}, fmt.Errorf("无法创建日志文件: %w", err)
	}
	c.Stdout = f
	c.Stderr = f
	c.Stdin = nil
	setSysProcAttr(c)
	stdin, err := c.StdinPipe()
	if err != nil {
		_ = f.Close()
		r.store.SetStatus(inst.ID, "error")
		return InstanceStartResult{}, fmt.Errorf("无法创建 stdin pipe: %w", err)
	}
	if err := c.Start(); err != nil {
		_ = f.Close()
		r.store.SetStatus(inst.ID, "error")
		return InstanceStartResult{}, fmt.Errorf("启动失败: %w", err)
	}

	r.mu.Lock()
	r.procs[inst.ID] = c
	r.stdins[inst.ID] = func(data string) error { _, err := stdin.Write([]byte(data)); return err }
	r.mu.Unlock()

	r.store.SetFields(inst.ID, map[string]any{
		"status":          "running",
		"pid":             c.Process.Pid,
		"lastStarted":     time.Now().Format(time.RFC3339),
		"terminalSession": "",
	})

	go func() {
		_ = c.Wait()
		_ = f.Close()
		r.mu.Lock()
		delete(r.procs, inst.ID)
		delete(r.stdins, inst.ID)
		r.mu.Unlock()
		cur, ok := r.store.Get(inst.ID)
		if ok && cur.Status != "stopped" {
			r.store.SetFields(inst.ID, map[string]any{
				"status":      "stopped",
				"pid":         0,
				"lastStopped": time.Now().Format(time.RFC3339),
			})
		}
	}()

	return InstanceStartResult{PID: c.Process.Pid, LogFile: logFile}, nil
}

func (r *InstanceRuntime) Stop(inst Instance) string {
	if svc, ok := instanceSystemdService(inst); ok {
		_ = runSystemctl("stop", svc)
		r.store.SetFields(inst.ID, map[string]any{
			"status":      "stopped",
			"pid":         0,
			"lastStopped": time.Now().Format(time.RFC3339),
		})
		return "已停止"
	}
	if inst.Status == "stopped" {
		return "已停止"
	}
	r.store.SetStatus(inst.ID, "stopping")

	r.mu.Lock()
	cmd := r.procs[inst.ID]
	r.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		stopProcess(cmd)
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
		r.mu.Lock()
		delete(r.procs, inst.ID)
		delete(r.stdins, inst.ID)
		r.mu.Unlock()
	}

	r.store.SetFields(inst.ID, map[string]any{
		"status":      "stopped",
		"pid":         0,
		"lastStopped": time.Now().Format(time.RFC3339),
	})
	return "已停止"
}

func (r *InstanceRuntime) Input(id, data string) error {
	r.mu.Lock()
	writer := r.stdins[id]
	r.mu.Unlock()
	if writer == nil {
		return fmt.Errorf("实例未运行或无标准输入")
	}
	return writer(data)
}

func (r *InstanceRuntime) Logs(id string, tail bool) string {
	logFile := fmt.Sprintf("data/instances/%s.log", id)
	data, err := os.ReadFile(logFile)
	if err != nil {
		return ""
	}
	logs := string(data)
	if tail {
		return tailLines(logs, 300)
	}
	return logs
}

func (r *InstanceRuntime) startSystemd(inst Instance, service string) (InstanceStartResult, error) {
	r.store.SetStatus(inst.ID, "starting")
	if err := runSystemctl("start", service); err != nil {
		r.store.SetStatus(inst.ID, "error")
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusBadRequest, message: err.Error()}
	}
	if !systemdActive(service) {
		r.store.SetStatus(inst.ID, "error")
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusBadRequest, message: fmt.Sprintf("systemd 服务 %s 未处于 active 状态", service)}
	}
	r.store.SetFields(inst.ID, map[string]any{
		"status":          "running",
		"pid":             0,
		"lastStarted":     time.Now().Format(time.RFC3339),
		"terminalSession": "",
	})
	return InstanceStartResult{LogFile: fmt.Sprintf("journalctl -u %s", service)}, nil
}

func instanceSystemdService(inst Instance) (string, bool) {
	return systemctlStartService(inst.StartCommand)
}

func systemctlStartService(command string) (string, bool) {
	fields := strings.Fields(command)
	if len(fields) != 3 || fields[0] != "systemctl" || fields[1] != "start" {
		return "", false
	}
	service := strings.TrimSpace(fields[2])
	if service == "" || strings.ContainsAny(service, `/\;&|$<>`) {
		return "", false
	}
	return service, true
}

func runSystemctl(action, service string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("systemd 服务只能在 Linux 上管理")
	}
	cmd := exec.Command("systemctl", action, service)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf(msg)
	}
	return nil
}

func systemdActive(service string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	return exec.Command("systemctl", "is-active", "--quiet", service).Run() == nil
}

type instanceRuntimeError struct {
	code    int
	message string
}

func (e *instanceRuntimeError) Error() string { return e.message }

const (
	httpStatusBadRequest = 400
	httpStatusConflict   = 409
)
