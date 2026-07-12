package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DeployTask struct {
	mu         sync.Mutex `json:"-"`
	ID         string     `json:"id"`
	GameID     string     `json:"gameId"`
	GameName   string     `json:"gameName"`
	Path       string     `json:"path"`
	AppID      int        `json:"appId"`
	Status     string     `json:"status"`
	Output     string     `json:"output"`
	Error      string     `json:"error"`
	StartedAt  time.Time  `json:"startedAt"`
	DoneAt     *time.Time `json:"doneAt,omitempty"`
	InstanceID string     `json:"instanceId,omitempty"`
}

type DeployManager struct {
	mu    sync.Mutex
	tasks map[string]*DeployTask
}

func NewDeployManager() *DeployManager {
	return &DeployManager{tasks: map[string]*DeployTask{}}
}

func (m *DeployManager) Get(id string) *DeployTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil {
		return nil
	}
	snap := t.Snapshot()
	return &snap
}

func (m *DeployManager) Start(game GameTemplate, path string, instances *InstanceStore) (*DeployTask, error) {
	id := time.Now().Format("20060102150405") + "-" + game.ID
	task := &DeployTask{
		ID:        id,
		GameID:    game.ID,
		GameName:  game.Name,
		Path:      path,
		AppID:     game.AppID,
		Status:    "running",
		StartedAt: time.Now(),
	}
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	go m.run(task, game, path, instances)
	return task, nil
}

func (m *DeployManager) run(task *DeployTask, game GameTemplate, path string, instances *InstanceStore) {
	task.appendOutput(fmt.Sprintf("创建安装目录: %s\n", path))
	if err := os.MkdirAll(path, 0755); err != nil {
		task.appendOutput(fmt.Sprintf("创建目录失败: %v\n", err))
		task.fail(err)
		return
	}

	steamcmd := lookPath("steamcmd")
	if steamcmd == "" {
		if _, err := os.Stat("/usr/local/steamcmd/steamcmd.sh"); err == nil {
			steamcmd = "/usr/local/steamcmd/steamcmd.sh"
		}
	}
	if steamcmd == "" {
		task.appendOutput("SteamCMD 未安装，请在「环境管理」页面先安装 SteamCMD\n")
		task.fail(fmt.Errorf("SteamCMD 未安装"))
		return
	}
	task.appendOutput(fmt.Sprintf("SteamCMD: %s\n", steamcmd))

	if game.AppID <= 0 {
		task.appendOutput(fmt.Sprintf("游戏 %s 无 Steam AppID，跳过 SteamCMD 下载\n", game.Name))
		task.succeed()
		m.createInstance(task, game, path, instances)
		return
	}

	// 解析符号链接，找到 steamcmd.sh 真实路径和目录
	realSteamcmd := steamcmd
	if resolved, err := filepath.EvalSymlinks(steamcmd); err == nil {
		realSteamcmd = resolved
	}
	steamcmdDir := filepath.Dir(realSteamcmd)
	cmdStr := fmt.Sprintf("%s +login anonymous +app_update %d validate +quit", realSteamcmd, game.AppID)
	task.appendOutput(fmt.Sprintf("执行: %s\n", cmdStr))

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = steamcmdDir
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
		task.appendOutput(scanner.Text() + "\n")
	}
	if err := cmd.Wait(); err != nil {
		task.appendOutput(fmt.Sprintf("SteamCMD 退出失败: %v\n", err))
		if exitErr, ok := err.(*exec.ExitError); ok {
			task.fail(fmt.Errorf("exit code %d", exitErr.ExitCode()))
		} else {
			task.fail(err)
		}
		return
	}

	task.appendOutput("部署完成\n")
	task.succeed()
	m.createInstance(task, game, path, instances)
}

func (m *DeployManager) createInstance(task *DeployTask, game GameTemplate, path string, instances *InstanceStore) {
	inst := Instance{
		Name:             game.Name,
		Description:      game.Description,
		WorkingDirectory: path,
		InstanceType:     game.ID,
		StopCommand:      "ctrl+c",
	}
	switch game.ID {
	case "palworld":
		inst.StartCommand = "systemctl start pal-server"
		inst.StopCommand = "stop"
	case "minecraft-java":
		inst.StopCommand = "stop"
	case "minecraft-bedrock":
		inst.StartCommand = "./bedrock_server"
		inst.StopCommand = "stop"
	case "valheim", "terraria":
		inst.StartCommand = "./start_server.sh"
	}
	created, err := instances.Create(inst)
	if err != nil {
		task.appendOutput(fmt.Sprintf("创建实例失败: %v\n", err))
		return
	}
	task.mu.Lock()
	task.InstanceID = created.ID
	task.mu.Unlock()
	task.appendOutput(fmt.Sprintf("已创建实例: %s (ID: %s)\n", created.Name, created.ID))
}

func (t *DeployTask) appendOutput(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Output += s
}

func (t *DeployTask) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.DoneAt = &now
	t.Status = "failed"
	t.Error = err.Error()
}

func (t *DeployTask) succeed() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.DoneAt = &now
	t.Status = "success"
}

func (t *DeployTask) Snapshot() DeployTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	return DeployTask{
		ID:         t.ID,
		GameID:     t.GameID,
		GameName:   t.GameName,
		Path:       t.Path,
		AppID:      t.AppID,
		Status:     t.Status,
		Output:     t.Output,
		Error:      t.Error,
		StartedAt:  t.StartedAt,
		DoneAt:     t.DoneAt,
		InstanceID: t.InstanceID,
	}
}

var _ = json.NewDecoder
var _ = strings.TrimSpace
