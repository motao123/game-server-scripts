package app

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type InstallTask struct {
	mu        sync.Mutex `json:"-"`
	ID        string     `json:"id"`
	Package   string     `json:"package"`
	Status    string     `json:"status"`
	Output    string     `json:"output"`
	Error     string     `json:"error"`
	StartedAt time.Time  `json:"startedAt"`
	DoneAt    *time.Time `json:"doneAt,omitempty"`
}

type InstallManager struct {
	mu    sync.Mutex
	tasks map[string]*InstallTask
}

func NewInstallManager() *InstallManager {
	return &InstallManager{tasks: map[string]*InstallTask{}}
}

var installCommands = map[string]string{
	"java":     "apt-get update && apt-get install -y openjdk-17-jre-headless",
	"java8":    "apt-get update && apt-get install -y openjdk-8-jre-headless",
	"java11":   "apt-get update && apt-get install -y openjdk-11-jre-headless",
	"java17":   "apt-get update && apt-get install -y openjdk-17-jre-headless",
	"java21":   "apt-get update && apt-get install -y openjdk-21-jre-headless",
	"java25":   "apt-get update && apt-get install -y openjdk-25-jre-headless",
	"steamcmd": "mkdir -p /usr/local/steamcmd && cd /usr/local/steamcmd && curl -fsSL https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz -o steamcmd.tar.gz && tar -xzf steamcmd.tar.gz && rm -f steamcmd.tar.gz && ln -sf /usr/local/steamcmd/steamcmd.sh /usr/local/bin/steamcmd",
	"tools":    "apt-get update && apt-get install -y curl wget tar gzip unzip",
}

func (m *InstallManager) Start(pkg, id string) (*InstallTask, error) {
	cmdStr, ok := installCommands[pkg]
	if !ok {
		return nil, fmt.Errorf("不支持的环境包: %s", pkg)
	}
	task := &InstallTask{ID: id, Package: pkg, Status: "running", StartedAt: time.Now()}
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	go func() {
		fullCmd := "export DEBIAN_FRONTEND=noninteractive && " + cmdStr
		cmd := exec.Command("bash", "-c", fullCmd)
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			task.appendOutput(fmt.Sprintf("启动失败: %v\n", err))
			task.fail(err)
			return
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			task.appendOutput(fmt.Sprintf("启动失败: %v\n", err))
			task.fail(err)
			return
		}
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			task.appendOutput(line)
		}
		_, _ = io.Copy(io.Discard, pipe)
		if err := cmd.Wait(); err != nil {
			task.appendOutput(fmt.Sprintf("安装失败: %v\n", err))
			if exitErr, ok := err.(*exec.ExitError); ok {
				task.fail(fmt.Errorf("exit code %d", exitErr.ExitCode()))
			} else {
				task.fail(err)
			}
			return
		}
		task.appendOutput("安装完成\n")
		task.succeed()
	}()

	return task, nil
}

func (t *InstallTask) appendOutput(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Output += s
}

func (t *InstallTask) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.DoneAt = &now
	t.Status = "failed"
	t.Error = err.Error()
}

func (t *InstallTask) succeed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.DoneAt = &now
	t.Status = "success"
}

func (t *InstallTask) Snapshot() InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	return InstallTask{
		ID:        t.ID,
		Package:   t.Package,
		Status:    t.Status,
		Output:    t.Output,
		Error:     t.Error,
		StartedAt: t.StartedAt,
		DoneAt:    t.DoneAt,
	}
}

func (m *InstallManager) Get(id string) *InstallTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil {
		return nil
	}
	snap := t.Snapshot()
	return &snap
}
