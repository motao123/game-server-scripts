package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type GameConfigTemplate struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	InstanceType string            `json:"instanceType"`
	File         string            `json:"file"`
	Fields       []GameConfigField `json:"fields"`
}

type GameConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description,omitempty"`
}

func gameConfigTemplates() []GameConfigTemplate {
	return []GameConfigTemplate{
		{ID: "minecraft-java-server", Name: "Minecraft Java server.properties", InstanceType: "minecraft-java", File: "server.properties", Fields: []GameConfigField{
			{Key: "server-port", Label: "服务器端口", Type: "number", Default: "25565"},
			{Key: "motd", Label: "服务器名称", Type: "text", Default: "A Minecraft Server"},
			{Key: "max-players", Label: "最大玩家数", Type: "number", Default: "20"},
			{Key: "online-mode", Label: "正版验证", Type: "bool", Default: "true"},
			{Key: "pvp", Label: "PVP", Type: "bool", Default: "true"},
			{Key: "difficulty", Label: "难度", Type: "select", Default: "easy", Description: "peaceful/easy/normal/hard"},
			{Key: "gamemode", Label: "游戏模式", Type: "select", Default: "survival", Description: "survival/creative/adventure/spectator"},
			{Key: "view-distance", Label: "视距", Type: "number", Default: "10"},
		}},
		{ID: "minecraft-bedrock-server", Name: "Minecraft Bedrock server.properties", InstanceType: "minecraft-bedrock", File: "server.properties", Fields: []GameConfigField{
			{Key: "server-port", Label: "IPv4 端口", Type: "number", Default: "19132"},
			{Key: "server-portv6", Label: "IPv6 端口", Type: "number", Default: "19133"},
			{Key: "server-name", Label: "服务器名称", Type: "text", Default: "Dedicated Server"},
			{Key: "max-players", Label: "最大玩家数", Type: "number", Default: "10"},
			{Key: "online-mode", Label: "正版验证", Type: "bool", Default: "true"},
			{Key: "allow-cheats", Label: "允许作弊", Type: "bool", Default: "false"},
			{Key: "difficulty", Label: "难度", Type: "select", Default: "easy"},
			{Key: "gamemode", Label: "游戏模式", Type: "select", Default: "survival"},
		}},
		{ID: "valheim-start", Name: "Valheim 启动参数", InstanceType: "valheim", File: "server.env", Fields: []GameConfigField{
			{Key: "SERVER_NAME", Label: "服务器名称", Type: "text", Default: "Valheim Server"},
			{Key: "WORLD_NAME", Label: "世界名称", Type: "text", Default: "Dedicated"},
			{Key: "SERVER_PASSWORD", Label: "服务器密码", Type: "password", Default: ""},
			{Key: "SERVER_PORT", Label: "端口", Type: "number", Default: "2456"},
		}},
		{ID: "terraria-start", Name: "Terraria serverconfig.txt", InstanceType: "terraria", File: "serverconfig.txt", Fields: []GameConfigField{
			{Key: "world", Label: "世界文件", Type: "text", Default: ""},
			{Key: "port", Label: "端口", Type: "number", Default: "7777"},
			{Key: "maxplayers", Label: "最大玩家数", Type: "number", Default: "8"},
			{Key: "password", Label: "服务器密码", Type: "password", Default: ""},
			{Key: "motd", Label: "欢迎语", Type: "text", Default: ""},
		}},
	}
}

func (s *Server) handleGameConfigTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"templates": gameConfigTemplates()})
}

func (s *Server) handleGameConfigRead(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instanceId")
	templateID := r.URL.Query().Get("templateId")
	inst, tmpl, path, err := s.resolveGameConfig(instanceID, templateID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	values := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		values = parseKeyValueConfig(string(data))
	}
	for _, field := range tmpl.Fields {
		if _, ok := values[field.Key]; !ok {
			values[field.Key] = field.Default
		}
	}
	writeJSON(w, map[string]any{"instance": inst, "template": tmpl, "path": path, "values": values})
}

func (s *Server) handleGameConfigSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string            `json:"instanceId"`
		TemplateID string            `json:"templateId"`
		Values     map[string]string `json:"values"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_, tmpl, path, err := s.resolveGameConfig(body.InstanceID, body.TemplateID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	allowed := map[string]bool{}
	for _, field := range tmpl.Fields {
		allowed[field.Key] = true
	}
	current := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		current = parseKeyValueConfig(string(data))
	}
	for key, value := range body.Values {
		if allowed[key] {
			current[key] = value
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(formatKeyValueConfig(current)), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "配置已保存", "path": path})
}

func (s *Server) resolveGameConfig(instanceID, templateID string) (Instance, GameConfigTemplate, string, error) {
	inst, ok := s.instances.Get(instanceID)
	if !ok {
		return Instance{}, GameConfigTemplate{}, "", fmt.Errorf("实例不存在")
	}
	var tmpl GameConfigTemplate
	for _, item := range gameConfigTemplates() {
		if item.ID == templateID {
			tmpl = item
			break
		}
	}
	if tmpl.ID == "" {
		return Instance{}, GameConfigTemplate{}, "", fmt.Errorf("配置模板不存在")
	}
	if tmpl.InstanceType != "" && inst.InstanceType != tmpl.InstanceType {
		return Instance{}, GameConfigTemplate{}, "", fmt.Errorf("配置模板不适用于该实例类型")
	}
	if strings.TrimSpace(inst.WorkingDirectory) == "" {
		return Instance{}, GameConfigTemplate{}, "", fmt.Errorf("实例工作目录为空")
	}
	path := filepath.Join(inst.WorkingDirectory, tmpl.File)
	if !s.safeRoot(inst.WorkingDirectory) && !s.safeRoot(path) {
		return Instance{}, GameConfigTemplate{}, "", fmt.Errorf("路径不允许访问")
	}
	return inst, tmpl, path, nil
}

func parseKeyValueConfig(content string) map[string]string {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		idx := strings.IndexAny(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func formatKeyValueConfig(values map[string]string) string {
	var b strings.Builder
	for key, value := range values {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return b.String()
}
