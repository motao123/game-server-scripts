package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type OnlineTemplate struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Author       string            `json:"author"`
	Tags         []string          `json:"tags,omitempty"`
	DefaultPath  string            `json:"defaultPath"`
	DownloadURL  string            `json:"downloadUrl,omitempty"`
	ArchiveType  string            `json:"archiveType,omitempty"`
	Files        map[string]string `json:"files,omitempty"`
	StartCommand string            `json:"startCommand"`
	StopCommand  string            `json:"stopCommand"`
	InstanceType string            `json:"instanceType"`
}

func (s *Server) onlineTemplates() []OnlineTemplate {
	templates := loadOnlineTemplates()
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates
}

func loadOnlineTemplates() []OnlineTemplate {
	paths := []string{
		filepath.Join("data", "online_templates.json"),
		"online_templates.json",
		filepath.Join("/opt/gsm-panel", "data", "online_templates.json"),
		filepath.Join("/usr/local/share/gsm-panel", "data", "online_templates.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var templates []OnlineTemplate
		if err := json.Unmarshal(data, &templates); err == nil && len(templates) > 0 {
			return normalizeOnlineTemplates(templates)
		}
	}
	return normalizeOnlineTemplates(defaultOnlineTemplates())
}

func normalizeOnlineTemplates(templates []OnlineTemplate) []OnlineTemplate {
	out := make([]OnlineTemplate, 0, len(templates))
	for _, item := range templates {
		if !validPluginID(item.ID) {
			continue
		}
		if item.Name == "" {
			item.Name = item.ID
		}
		if item.Version == "" {
			item.Version = "1.0.0"
		}
		if item.StopCommand == "" {
			item.StopCommand = "ctrl+c"
		}
		if item.InstanceType == "" {
			item.InstanceType = "online-template"
		}
		out = append(out, item)
	}
	return out
}

func defaultOnlineTemplates() []OnlineTemplate {
	return []OnlineTemplate{
		{
			ID:           "generic-shell-server",
			Name:         "通用 Shell 服务模板",
			Description:  "生成一个可启动的通用实例目录，用于快速接入自定义服务端。",
			Version:      "1.0.0",
			Author:       "game-server-scripts",
			Tags:         []string{"通用", "模板"},
			DefaultPath:  "/home/steam/online/generic-shell-server",
			StartCommand: "./start.sh",
			StopCommand:  "ctrl+c",
			InstanceType: "online-template",
			Files: map[string]string{
				"README.md": "# 通用 Shell 服务模板\n\n这个目录由在线模板部署生成。\n",
				"start.sh":  "#!/usr/bin/env bash\necho 'online template server started'\nwhile true; do sleep 60; done\n",
			},
		},
	}
}

func (s *Server) handleOnlineTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"templates": s.onlineTemplates()})
}

func (s *Server) handleOnlineTemplateDeploy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var tmpl *OnlineTemplate
	for _, item := range s.onlineTemplates() {
		if item.ID == body.ID {
			copy := item
			tmpl = &copy
			break
		}
	}
	if tmpl == nil {
		writeError(w, http.StatusBadRequest, "未知的在线模板: "+body.ID)
		return
	}
	path := body.Path
	if path == "" {
		path = tmpl.DefaultPath
	}
	task, err := s.deploys.StartOnline(*tmpl, path, s.instances)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "taskId": task.ID, "message": fmt.Sprintf("已启动 %s 在线部署任务", tmpl.Name), "path": path})
}

func (m *DeployManager) StartOnline(tmpl OnlineTemplate, path string, instances *InstanceStore) (*DeployTask, error) {
	id := time.Now().Format("20060102150405") + "-" + tmpl.ID
	task := &DeployTask{ID: id, GameID: tmpl.ID, GameName: tmpl.Name, Path: path, Status: "running", StartedAt: time.Now()}
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()
	go m.runOnline(task, tmpl, path, instances)
	return task, nil
}

func (m *DeployManager) runOnline(task *DeployTask, tmpl OnlineTemplate, path string, instances *InstanceStore) {
	task.appendOutput(fmt.Sprintf("创建部署目录: %s\n", path))
	if err := os.MkdirAll(path, 0755); err != nil {
		task.fail(err)
		return
	}
	for name, content := range tmpl.Files {
		if !validOnlineTemplateFile(name) {
			task.appendOutput(fmt.Sprintf("跳过不安全文件名: %s\n", name))
			continue
		}
		target := filepath.Join(path, filepath.Clean(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			task.fail(err)
			return
		}
		mode := os.FileMode(0644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0755
		}
		if err := os.WriteFile(target, []byte(content), mode); err != nil {
			task.fail(err)
			return
		}
		task.appendOutput(fmt.Sprintf("写入文件: %s\n", name))
	}
	if tmpl.DownloadURL != "" {
		if err := downloadOnlineTemplate(tmpl, path, task); err != nil {
			task.fail(err)
			return
		}
	}
	m.createOnlineInstance(task, tmpl, path, instances)
	task.succeed()
}

func (m *DeployManager) createOnlineInstance(task *DeployTask, tmpl OnlineTemplate, path string, instances *InstanceStore) {
	start := tmpl.StartCommand
	if start == "" {
		start = detectStartScript(path)
		if start != "" {
			start = "./" + start
		}
	}
	inst := Instance{Name: tmpl.Name, Description: tmpl.Description, WorkingDirectory: path, StartCommand: start, StopCommand: tmpl.StopCommand, InstanceType: tmpl.InstanceType}
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

func downloadOnlineTemplate(tmpl OnlineTemplate, path string, task *DeployTask) error {
	archive := filepath.Join(path, ".online-template-download")
	if err := downloadFile(tmpl.DownloadURL, archive, task); err != nil {
		return err
	}
	defer os.Remove(archive)
	switch strings.ToLower(tmpl.ArchiveType) {
	case "", "file":
		return nil
	case "tar.gz", "tgz":
		task.appendOutput("解压 tar.gz 包\n")
		return extractTarGz(archive, path)
	default:
		return fmt.Errorf("不支持的在线模板包类型: %s", tmpl.ArchiveType)
	}
}

func validOnlineTemplateFile(name string) bool {
	clean := filepath.Clean(name)
	return clean != "." && !filepath.IsAbs(clean) && !strings.HasPrefix(clean, "..") && !strings.Contains(clean, string(os.PathSeparator)+".."+string(os.PathSeparator))
}
