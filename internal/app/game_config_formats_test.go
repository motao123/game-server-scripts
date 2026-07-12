package app

import (
	"strings"
	"testing"
)

func TestParseAndFormatJSONConfig(t *testing.T) {
	fields := []GameConfigField{
		{Key: "server.name", Type: "text", Default: "Old"},
		{Key: "server.maxPlayers", Type: "number", Default: "16"},
		{Key: "server.public", Type: "bool", Default: "true"},
	}
	content := `{"server":{"name":"Demo","maxPlayers":8}}`
	values := parseGameConfig(content, "json")
	if values["server.name"] != "Demo" || values["server.maxPlayers"] != "8" {
		t.Fatalf("unexpected json values: %#v", values)
	}
	out := formatGameConfig(content, map[string]string{"server.name": "New", "server.maxPlayers": "32", "server.public": "false"}, fields, "json")
	if !strings.Contains(out, `"name": "New"`) || !strings.Contains(out, `"maxPlayers": 32`) || !strings.Contains(out, `"public": false`) {
		t.Fatalf("unexpected json output: %s", out)
	}
}

func TestParseKeyValueSections(t *testing.T) {
	values := parseGameConfig("[Server]\nName=Demo\nPort: 8211\n", "ini")
	if values["Server.Name"] != "Demo" || values["Server.Port"] != "8211" {
		t.Fatalf("unexpected ini values: %#v", values)
	}
}

func TestFormatKeyValueDefaults(t *testing.T) {
	fields := []GameConfigField{
		{Key: "Server.Name", Default: "Demo"},
		{Key: "Server.Port", Default: "8211"},
	}
	out := formatGameConfig("", map[string]string{"Server.Name": "Panel"}, fields, "ini")
	if !strings.Contains(out, "[Server]") || !strings.Contains(out, "Name=Panel") || !strings.Contains(out, "Port=8211") {
		t.Fatalf("unexpected defaults: %s", out)
	}
}

func TestGameConfigTemplatesExpanded(t *testing.T) {
	ids := map[string]bool{}
	for _, tpl := range gameConfigTemplates() {
		ids[tpl.ID] = true
	}
	for _, id := range []string{"minecraft-java-server", "minecraft-bedrock-server", "valheim-start", "terraria-start", "project-zomboid-start"} {
		if !ids[id] {
			t.Fatalf("expected template %s", id)
		}
	}
}
