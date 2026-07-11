package palworld

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type ConfigItem struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Category string   `json:"category"`
	Options  []string `json:"options,omitempty"`
	Value    any      `json:"value"`
}

type ConfigCategory struct {
	Name  string       `json:"name"`
	Items []ConfigItem `json:"items"`
}

var schema = []ConfigItem{
	{Key: "ServerName", Label: "服务器名称", Type: "text", Category: "基础"},
	{Key: "ServerDescription", Label: "服务器描述", Type: "text", Category: "基础"},
	{Key: "AdminPassword", Label: "管理员密码", Type: "password", Category: "基础"},
	{Key: "ServerPassword", Label: "服务器密码", Type: "password", Category: "基础"},
	{Key: "ServerPlayerMaxNum", Label: "最大玩家数", Type: "number", Category: "基础"},
	{Key: "PublicPort", Label: "游戏端口", Type: "number", Category: "基础"},
	{Key: "PublicIP", Label: "公网 IP", Type: "text", Category: "基础"},
	{Key: "RCONEnabled", Label: "启用 RCON", Type: "bool", Category: "远程管理"},
	{Key: "RCONPort", Label: "RCON 端口", Type: "number", Category: "远程管理"},
	{Key: "RESTAPIEnabled", Label: "启用 REST API", Type: "bool", Category: "远程管理"},
	{Key: "RESTAPIPort", Label: "REST API 端口", Type: "number", Category: "远程管理"},
	{Key: "CrossplayPlatforms", Label: "允许连接的平台", Type: "multiselect", Category: "跨平台", Options: []string{"Steam", "Xbox", "PS5", "Mac"}},
	{Key: "LogFormatType", Label: "日志格式", Type: "select", Category: "日志", Options: []string{"Text", "Json"}},
	{Key: "ExpRate", Label: "经验倍率", Type: "number", Category: "游戏平衡"},
	{Key: "PalCaptureRate", Label: "帕鲁捕获率", Type: "number", Category: "游戏平衡"},
	{Key: "DeathPenalty", Label: "死亡惩罚", Type: "select", Category: "游戏平衡", Options: []string{"None", "Item", "ItemAndEquipment", "All"}},
	{Key: "bIsShowJoinLeftMessage", Label: "显示加入/离开消息", Type: "bool", Category: "显示"},
	{Key: "bShowPlayerList", Label: "显示玩家列表", Type: "bool", Category: "显示"},
	{Key: "bIsPvP", Label: "开启 PvP", Type: "bool", Category: "PvP"},
}

func (s Service) ConfigSchema() map[string]any {
	settings, err := s.readSettings()
	if err != nil {
		return map[string]any{"categories": []ConfigCategory{}, "error": err.Error()}
	}
	order := []string{}
	seen := map[string]bool{}
	for _, item := range schema {
		if !seen[item.Category] {
			seen[item.Category] = true
			order = append(order, item.Category)
		}
	}
	var categories []ConfigCategory
	for _, cat := range order {
		cc := ConfigCategory{Name: cat}
		for _, meta := range schema {
			if meta.Category != cat {
				continue
			}
			meta.Value = normalize(settings[meta.Key], meta.Type)
			cc.Items = append(cc.Items, meta)
		}
		categories = append(categories, cc)
	}
	return map[string]any{"categories": categories}
}

func (s Service) SaveConfig(values map[string]any) error {
	data, err := os.ReadFile(s.Config.PalSettings)
	if err != nil {
		return err
	}
	settings := parseSettings(string(data))
	known := map[string]ConfigItem{}
	for _, item := range schema {
		known[item.Key] = item
	}
	for key, value := range values {
		meta, ok := known[key]
		if !ok {
			continue
		}
		settings[key] = formatValue(value, meta.Type)
	}
	pairs := make([]string, 0, len(settings))
	for k, v := range settings {
		pairs = append(pairs, k+"="+v)
	}
	newOption := "OptionSettings=(" + strings.Join(pairs, ",") + ")"
	re := regexp.MustCompile(`(?s)OptionSettings=\(.*\)`)
	newText := re.ReplaceAllString(string(data), newOption)
	backup := fmt.Sprintf("%s.bak.%d", s.Config.PalSettings, time.Now().Unix())
	_ = os.WriteFile(backup, data, 0644)
	_ = os.Chmod(s.Config.PalSettings, 0644)
	if err := os.WriteFile(s.Config.PalSettings, []byte(newText), 0644); err != nil {
		return err
	}
	_ = exec.Command("chown", "steam:steam", s.Config.PalSettings).Run()
	_ = os.Chmod(s.Config.PalSettings, 0444)
	return nil
}

func (s Service) readSettings() (map[string]string, error) {
	data, err := os.ReadFile(s.Config.PalSettings)
	if err != nil {
		return nil, err
	}
	return parseSettings(string(data)), nil
}

func parseSettings(text string) map[string]string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = line[:idx]
		}
		lines = append(lines, line)
	}
	re := regexp.MustCompile(`(?s)OptionSettings=\((.*)\)`)
	m := re.FindStringSubmatch(strings.Join(lines, "\n"))
	if len(m) < 2 {
		return map[string]string{}
	}
	itemRe := regexp.MustCompile(`(\w+)\s*=\s*("[^"]*"|\([^)]*\)|[^,)\s]+)`)
	out := map[string]string{}
	for _, match := range itemRe.FindAllStringSubmatch(m[1], -1) {
		out[match[1]] = match[2]
	}
	return out
}

func normalize(raw, typ string) any {
	if raw == "" {
		if typ == "bool" {
			return false
		}
		if typ == "multiselect" {
			return []string{}
		}
		if typ == "number" {
			return 0
		}
		return ""
	}
	switch typ {
	case "text", "password":
		return strings.Trim(raw, `"`)
	case "bool":
		return strings.EqualFold(raw, "true")
	case "multiselect":
		v := strings.Trim(raw, "()")
		if v == "" {
			return []string{}
		}
		return strings.Split(v, ",")
	default:
		return strings.Trim(raw, `"`)
	}
}

func formatValue(value any, typ string) string {
	switch typ {
	case "text", "password":
		return fmt.Sprintf("%q", fmt.Sprint(value))
	case "bool":
		if b, ok := value.(bool); ok && b {
			return "True"
		}
		return "False"
	case "multiselect":
		if arr, ok := value.([]any); ok {
			parts := []string{}
			for _, v := range arr {
				parts = append(parts, fmt.Sprint(v))
			}
			return "(" + strings.Join(parts, ",") + ")"
		}
		return "()"
	default:
		return fmt.Sprint(value)
	}
}
