package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func parseGameConfig(content, format string) map[string]string {
	switch strings.ToLower(format) {
	case "json":
		return parseJSONConfig(content)
	case "toml", "yaml", "yml", "ini", "env", "properties":
		return parseKeyValueConfig(content)
	default:
		return parseKeyValueConfig(content)
	}
}

func formatGameConfig(existing string, values map[string]string, fields []GameConfigField, format string) string {
	switch strings.ToLower(format) {
	case "json":
		return formatJSONConfig(existing, values, fields)
	case "yaml", "yml":
		return formatYAMLConfig(existing, values, fields)
	case "toml", "ini", "env", "properties":
		return formatKeyValueConfig(existing, values, fields)
	default:
		return formatKeyValueConfig(existing, values, fields)
	}
}

func formatYAMLConfig(existing string, values map[string]string, fields []GameConfigField) string {
	if strings.TrimSpace(existing) == "" {
		var b strings.Builder
		for _, field := range fields {
			value, ok := values[field.Key]
			if !ok {
				value = field.Default
			}
			b.WriteString(field.Key)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteByte('\n')
		}
		return b.String()
	}
	return formatKeyValueConfig(existing, values, fields)
}

func parseJSONConfig(content string) map[string]string {
	values := map[string]string{}
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return values
	}
	flattenJSON("", data, values)
	return values
}

func flattenJSON(prefix string, value any, out map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSON(next, child, out)
		}
	case []any:
		b, _ := json.Marshal(v)
		out[prefix] = string(b)
	case string:
		out[prefix] = v
	case bool:
		out[prefix] = fmt.Sprintf("%t", v)
	case float64:
		out[prefix] = strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	case nil:
		out[prefix] = ""
	default:
		out[prefix] = fmt.Sprintf("%v", v)
	}
}

func formatJSONConfig(existing string, values map[string]string, fields []GameConfigField) string {
	root := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &root)
	}
	for _, field := range fields {
		value, ok := values[field.Key]
		if !ok {
			value = field.Default
		}
		setJSONPath(root, field.Key, castConfigValue(field, value))
	}
	data, _ := json.MarshalIndent(root, "", "  ")
	return string(data) + "\n"
}

func setJSONPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	cur := root
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = value
			return
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
}

func castConfigValue(field GameConfigField, value string) any {
	switch field.Type {
	case "bool":
		return value == "true"
	case "number":
		if strings.Contains(value, ".") {
			var f float64
			_, _ = fmt.Sscanf(value, "%f", &f)
			return f
		}
		var i int
		_, _ = fmt.Sscanf(value, "%d", &i)
		return i
	default:
		return value
	}
}

func parseKeyValueConfig(content string) map[string]string {
	values := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		idx := strings.IndexAny(line, "=:")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
		if section != "" && !strings.Contains(key, ".") {
			key = section + "." + key
		}
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func formatKeyValueConfig(existing string, values map[string]string, fields []GameConfigField) string {
	if strings.TrimSpace(existing) == "" {
		return formatKeyValueDefaults(values, fields)
	}
	seen := map[string]bool{}
	section := ""
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(existing))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		idx := strings.IndexAny(trimmed, "=:")
		if idx < 0 {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		sep := trimmed[idx : idx+1]
		key := strings.TrimSpace(trimmed[:idx])
		lookup := key
		if section != "" && !strings.Contains(key, ".") {
			lookup = section + "." + key
		}
		if value, ok := values[lookup]; ok {
			b.WriteString(key)
			b.WriteString(sep)
			b.WriteString(value)
			b.WriteByte('\n')
			seen[lookup] = true
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, field := range fields {
		if seen[field.Key] {
			continue
		}
		if value, ok := values[field.Key]; ok {
			b.WriteString(field.Key)
			b.WriteByte('=')
			b.WriteString(value)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatKeyValueDefaults(values map[string]string, fields []GameConfigField) string {
	var b strings.Builder
	groups := map[string][]GameConfigField{}
	order := []string{}
	for _, field := range fields {
		section := ""
		if dot := strings.Index(field.Key, "."); dot > 0 {
			section = field.Key[:dot]
		}
		if _, ok := groups[section]; !ok {
			order = append(order, section)
		}
		groups[section] = append(groups[section], field)
	}
	sort.Strings(order)
	for _, section := range order {
		if section != "" {
			b.WriteString("[")
			b.WriteString(section)
			b.WriteString("]\n")
		}
		for _, field := range groups[section] {
			key := field.Key
			if section != "" {
				key = strings.TrimPrefix(key, section+".")
			}
			value, ok := values[field.Key]
			if !ok {
				value = field.Default
			}
			b.WriteString(key)
			b.WriteByte('=')
			b.WriteString(value)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
