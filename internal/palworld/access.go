package palworld

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s Service) AddWhitelist(entry WhitelistEntry) (string, error) {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.SteamID = strings.TrimSpace(entry.SteamID)
	entry.PlayerUID = strings.TrimSpace(entry.PlayerUID)
	if entry.Name == "" && entry.SteamID == "" && entry.PlayerUID == "" {
		return "", fmt.Errorf("至少填一个字段")
	}
	list := s.Whitelist()
	if entry.SteamID != "" {
		filtered := list[:0]
		for _, item := range list {
			if item.SteamID != entry.SteamID {
				filtered = append(filtered, item)
			}
		}
		list = filtered
	}
	list = append(list, entry)
	return "已添加", s.SaveWhitelist(list)
}

func (s Service) RemoveWhitelist(steamid string) (string, error) {
	list := s.Whitelist()
	newList := list[:0]
	for _, item := range list {
		if item.SteamID != steamid {
			newList = append(newList, item)
		}
	}
	if err := s.SaveWhitelist(newList); err != nil {
		return "", err
	}
	return fmt.Sprintf("已移除 %d 条", len(list)-len(newList)), nil
}

func (s Service) CheckWhitelist() (string, error) {
	list := s.Whitelist()
	if len(list) == 0 {
		return "白名单为空，未启用", nil
	}
	players := s.Players()
	kicked := []string{}
	for _, p := range players {
		matched := false
		for _, item := range list {
			if item.SteamID != "" && item.SteamID != p.SteamID {
				continue
			}
			matched = true
			break
		}
		if !matched && p.SteamID != "" {
			if _, err := s.RCON("KickPlayer " + p.SteamID); err == nil {
				kicked = append(kicked, p.Name)
			}
		}
	}
	return fmt.Sprintf("已踢出 %d 人", len(kicked)), nil
}

func (s Service) Unban(steamid string) (string, error) {
	path := filepath.Join(s.Config.SaveGamesDir, "banlist.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	target := "steam_" + strings.TrimPrefix(steamid, "steam_")
	lines := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" && strings.TrimSpace(line) != target {
			lines = append(lines, line)
		}
	}
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return "", err
	}
	return "已解封 " + steamid, nil
}
