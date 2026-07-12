package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	ServerType string     `json:"serverType,omitempty"`
	Version    string     `json:"version,omitempty"`
}

type DeployOptions struct {
	ServerType string
	Version    string
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

func (m *DeployManager) Start(game GameTemplate, path string, instances *InstanceStore, options DeployOptions) (*DeployTask, error) {
	if options.ServerType == "" && len(game.ServerTypes) > 0 {
		options.ServerType = game.ServerTypes[0]
	}
	if options.Version == "" {
		options.Version = game.Version
	}
	id := time.Now().Format("20060102150405") + "-" + game.ID
	task := &DeployTask{
		ID:         id,
		GameID:     game.ID,
		GameName:   game.Name,
		Path:       path,
		AppID:      game.AppID,
		Status:     "running",
		StartedAt:  time.Now(),
		ServerType: options.ServerType,
		Version:    options.Version,
	}
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	go m.run(task, game, path, instances, options)
	return task, nil
}

func (m *DeployManager) run(task *DeployTask, game GameTemplate, path string, instances *InstanceStore, options DeployOptions) {
	task.appendOutput(fmt.Sprintf("创建安装目录: %s\n", path))
	if err := os.MkdirAll(path, 0755); err != nil {
		task.appendOutput(fmt.Sprintf("创建目录失败: %v\n", err))
		task.fail(err)
		return
	}
	if game.ID == "minecraft-java" {
		if err := m.deployMinecraftJava(task, path, options); err != nil {
			task.fail(err)
			return
		}
		m.createInstance(task, game, path, instances)
		task.succeed()
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
		m.createInstance(task, game, path, instances)
		task.succeed()
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
	m.createInstance(task, game, path, instances)
	task.succeed()
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
		inst.StartCommand = "./start.sh"
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
		ServerType: t.ServerType,
		Version:    t.Version,
	}
}

func (m *DeployManager) deployMinecraftJava(task *DeployTask, path string, options DeployOptions) error {
	serverType := strings.ToLower(strings.TrimSpace(options.ServerType))
	if serverType == "" {
		serverType = "paper"
	}
	version := strings.TrimSpace(options.Version)
	if version == "" {
		version = "latest"
	}
	task.appendOutput(fmt.Sprintf("Minecraft Java: %s %s\n", serverType, version))
	url, err := minecraftServerURL(serverType, version)
	if err != nil {
		return err
	}
	if err := downloadFile(url, filepath.Join(path, "server.jar"), task); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, "eula.txt"), []byte("eula=true\n"), 0644); err != nil {
		return err
	}
	start := "#!/usr/bin/env bash\njava -Xms1G -Xmx2G -jar server.jar nogui\n"
	if err := os.WriteFile(filepath.Join(path, "start.sh"), []byte(start), 0755); err != nil {
		return err
	}
	task.appendOutput("Minecraft Java 服务端部署完成\n")
	return nil
}

func minecraftServerURL(serverType, version string) (string, error) {
	switch serverType {
	case "paper":
		resolved, err := resolveMinecraftVersion(serverType, version)
		if err != nil {
			return "", err
		}
		version = resolved
		builds := []struct {
			ID        int    `json:"id"`
			Channel   string `json:"channel"`
			Downloads map[string]struct {
				URL string `json:"url"`
			} `json:"downloads"`
		}{}
		if err := getJSON(fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds", version), &builds); err != nil {
			return "", err
		}
		if len(builds) == 0 {
			return "", fmt.Errorf("未找到 Paper %s 构建", version)
		}
		for _, build := range builds {
			if strings.EqualFold(build.Channel, "stable") {
				if download, ok := build.Downloads["server:default"]; ok && download.URL != "" {
					return download.URL, nil
				}
			}
		}
		for _, build := range builds {
			if download, ok := build.Downloads["server:default"]; ok && download.URL != "" {
				return download.URL, nil
			}
		}
		return "", fmt.Errorf("Paper %s 没有可用下载", version)
	case "vanilla":
		resolved, err := resolveMinecraftVersion(serverType, version)
		if err != nil {
			return "", err
		}
		version = resolved
		manifest := struct {
			Versions []struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"versions"`
		}{}
		if err := getJSON("https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", &manifest); err != nil {
			return "", err
		}
		for _, item := range manifest.Versions {
			if item.ID == version {
				detail := struct {
					Downloads struct {
						Server struct {
							URL string `json:"url"`
						} `json:"server"`
					} `json:"downloads"`
				}{}
				if err := getJSON(item.URL, &detail); err != nil {
					return "", err
				}
				if detail.Downloads.Server.URL == "" {
					return "", fmt.Errorf("该版本没有 vanilla server 下载")
				}
				return detail.Downloads.Server.URL, nil
			}
		}
		return "", fmt.Errorf("未找到 Minecraft 版本 %s", version)
	case "fabric":
		resolved, err := resolveMinecraftVersion(serverType, version)
		if err != nil {
			return "", err
		}
		version = resolved
		loaders := []struct {
			Version string `json:"version"`
		}{}
		if err := getJSON("https://meta.fabricmc.net/v2/versions/loader", &loaders); err != nil {
			return "", err
		}
		installers := []struct {
			Version string `json:"version"`
		}{}
		if err := getJSON("https://meta.fabricmc.net/v2/versions/installer", &installers); err != nil {
			return "", err
		}
		if len(loaders) == 0 || len(installers) == 0 {
			return "", fmt.Errorf("未找到 Fabric loader 或 installer")
		}
		return fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/%s/server/jar", version, loaders[0].Version, installers[0].Version), nil
	case "forge":
		return fmt.Sprintf("https://maven.minecraftforge.net/net/minecraftforge/forge/%s-latest/forge-%s-latest-installer.jar", version, version), nil
	case "spigot":
		return "https://hub.spigotmc.org/jenkins/job/BuildTools/lastSuccessfulBuild/artifact/target/BuildTools.jar", nil
	default:
		return "", fmt.Errorf("不支持的 Minecraft 服务端类型: %s", serverType)
	}
}

func resolveMinecraftVersion(serverType, version string) (string, error) {
	if version != "" && version != "latest" {
		return version, nil
	}
	switch serverType {
	case "paper":
		meta := struct {
			Versions map[string][]string `json:"versions"`
		}{}
		if err := getJSON("https://fill.papermc.io/v3/projects/paper", &meta); err != nil {
			return "", err
		}
		groups := make([]string, 0, len(meta.Versions))
		for group := range meta.Versions {
			groups = append(groups, group)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(groups)))
		for _, group := range groups {
			versions := meta.Versions[group]
			if len(versions) > 0 {
				return versions[0], nil
			}
		}
		return "", fmt.Errorf("未找到 Paper 版本")
	case "vanilla", "fabric":
		manifest := struct {
			Latest struct {
				Release string `json:"release"`
			} `json:"latest"`
		}{}
		if err := getJSON("https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", &manifest); err != nil {
			return "", err
		}
		if manifest.Latest.Release == "" {
			return "", fmt.Errorf("未找到 Minecraft 最新版本")
		}
		return manifest.Latest.Release, nil
	default:
		return "1.21.4", nil
	}
}

func getJSON(url string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "game-server-scripts/1.0 (https://github.com/motao123/game-server-scripts)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("请求失败: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func downloadFile(url, dst string, task *DeployTask) error {
	task.appendOutput(fmt.Sprintf("下载: %s\n", url))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "game-server-scripts/1.0 (https://github.com/motao123/game-server-scripts)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	task.appendOutput(fmt.Sprintf("已下载 %.2f MB\n", float64(n)/1024/1024))
	return nil
}

var _ = json.NewDecoder
var _ = strings.TrimSpace
