package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

type InstanceReadiness struct {
	Ready    bool     `json:"ready"`
	Command  string   `json:"command"`
	Problems []string `json:"problems"`
	Warnings []string `json:"warnings"`
}

func NewInstanceRuntime(store *InstanceStore) *InstanceRuntime {
	return &InstanceRuntime{store: store, procs: map[string]*exec.Cmd{}, stdins: map[string]func(string) error{}}
}

func (r *InstanceRuntime) Start(inst Instance) (InstanceStartResult, error) {
	if inst.Status == "running" || inst.Status == "starting" {
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusConflict, message: "实例已在运行"}
	}
	check := r.CheckReadiness(inst)
	if !check.Ready {
		return InstanceStartResult{}, &instanceRuntimeError{code: httpStatusBadRequest, message: strings.Join(check.Problems, "；")}
	}
	cmd := check.Command
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

func (r *InstanceRuntime) CheckReadiness(inst Instance) InstanceReadiness {
	check := InstanceReadiness{Ready: true}
	if strings.TrimSpace(inst.Name) == "" {
		check.Problems = append(check.Problems, "实例名称不能为空")
	}
	if strings.TrimSpace(inst.WorkingDirectory) == "" {
		check.Problems = append(check.Problems, "工作目录不能为空")
	} else if st, err := os.Stat(inst.WorkingDirectory); err != nil {
		check.Problems = append(check.Problems, "工作目录不存在: "+inst.WorkingDirectory)
	} else if !st.IsDir() {
		check.Problems = append(check.Problems, "工作目录不是目录: "+inst.WorkingDirectory)
	}
	cmd := effectiveStartCommand(inst)
	check.Command = cmd
	if strings.TrimSpace(cmd) == "" {
		check.Problems = append(check.Problems, "启动命令为空")
	} else if _, ok := systemctlStartService(cmd); !ok {
		if err := validateStartCommandFile(inst.WorkingDirectory, cmd); err != nil {
			check.Problems = append(check.Problems, err.Error())
		}
	}
	if inst.InstanceType == "generic" && strings.TrimSpace(inst.StopCommand) == "" {
		check.Warnings = append(check.Warnings, "未设置停止命令，停止时将使用 Ctrl+C")
	}
	check.Ready = len(check.Problems) == 0
	return check
}

func effectiveStartCommand(inst Instance) string {
	cmd := inst.StartCommand
	if inst.InstanceType == "minecraft-java" && strings.TrimSpace(cmd) == "" {
		if script := detectStartScript(inst.WorkingDirectory); script != "" {
			return "./" + script
		}
		if jar := detectJarFile(inst.WorkingDirectory); jar != "" {
			return "java -jar " + jar + " nogui"
		}
	}
	return cmd
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

func validateStartCommandFile(workingDirectory, command string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}
	bin := commandExecutable(fields)
	if bin == "" {
		return nil
	}
	if strings.ContainsAny(bin, `;&|$<>`) {
		return nil
	}
	var path string
	if strings.HasPrefix(bin, "./") {
		path = filepath.Join(workingDirectory, strings.TrimPrefix(bin, "./"))
	} else if filepath.IsAbs(bin) {
		path = bin
	} else {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("启动文件不存在: %s", path)
	}
	if st.IsDir() {
		return fmt.Errorf("启动文件是目录: %s", path)
	}
	if runtime.GOOS != "windows" && st.Mode()&0111 == 0 {
		return fmt.Errorf("启动文件不可执行: %s", path)
	}
	return nil
}

func commandExecutable(fields []string) string {
	if len(fields) >= 5 && fields[0] == "runuser" && fields[1] == "-u" && fields[3] == "--" {
		return fields[4]
	}
	return fields[0]
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
