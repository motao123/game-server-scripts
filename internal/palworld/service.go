package palworld

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"game-server-scripts/internal/config"
	"game-server-scripts/internal/rcon"
)

var backupNameRe = regexp.MustCompile(`^pal_backup_[A-Za-z0-9_.-]+\.tar\.gz$`)

type Service struct{ Config config.Config }

type Status struct {
	Active bool   `json:"active"`
	Uptime string `json:"uptime"`
}

type Player struct {
	Name      string `json:"name"`
	SteamID   string `json:"steamid"`
	PlayerUID string `json:"playeruid,omitempty"`
	Level     *int   `json:"level,omitempty"`
	Ping      *int   `json:"ping,omitempty"`
}

type Save struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time int64  `json:"time"`
}

type WhitelistEntry struct {
	Name      string `json:"name"`
	SteamID   string `json:"steamid"`
	PlayerUID string `json:"playeruid"`
}

type BanEntry struct {
	SteamID string `json:"steamid"`
}

func (s Service) Systemctl(action string) (string, error) {
	cmd := exec.Command("systemctl", action, s.Config.Service)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s Service) Status() Status {
	out, _ := exec.Command("systemctl", "is-active", s.Config.Service).Output()
	active := strings.TrimSpace(string(out)) == "active"
	uptime := ""
	if active {
		if b, err := exec.Command("systemctl", "show", s.Config.Service, "--property=ActiveEnterTimestamp", "--value").Output(); err == nil {
			uptime = strings.TrimSpace(string(b))
		}
	}
	return Status{Active: active, Uptime: uptime}
}

func (s Service) Logs() string {
	out, err := exec.Command("journalctl", "-u", s.Config.Service, "-n", "200", "--no-pager").CombinedOutput()
	if err != nil {
		return string(out) + err.Error()
	}
	return string(out)
}

func (s Service) RCON(command string) (string, error) {
	return rcon.Client{Addr: fmt.Sprintf("127.0.0.1:%d", s.Config.RCONPort), Password: s.Config.RCONPassword}.Command(command)
}

func (s Service) Players() []Player {
	players, err := s.playersREST()
	if err == nil {
		return players
	}
	return s.playersRCON()
}

func (s Service) playersREST() ([]Player, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/v1/api/players", s.Config.RESTAPIPort), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("admin", s.Config.RCONPassword)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("REST API status %d", resp.StatusCode)
	}
	var raw struct {
		Players []struct {
			Name      string `json:"name"`
			AccountID string `json:"accountId"`
			UserID    string `json:"userId"`
			Level     *int   `json:"level"`
			Ping      *int   `json:"ping"`
		} `json:"players"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]Player, 0, len(raw.Players))
	for _, p := range raw.Players {
		out = append(out, Player{Name: p.Name, SteamID: p.AccountID, PlayerUID: p.UserID, Level: p.Level, Ping: p.Ping})
	}
	return out, nil
}

func (s Service) playersRCON() []Player {
	out, err := s.RCON("ShowPlayers")
	if err != nil {
		return nil
	}
	var players []Player
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, ",")
		if len(fields) >= 3 && fields[0] != "name" {
			players = append(players, Player{Name: strings.TrimSpace(fields[0]), PlayerUID: strings.TrimSpace(fields[1]), SteamID: strings.TrimSpace(fields[2])})
		}
	}
	return players
}

func (s Service) ListSaves() []Save {
	entries, err := os.ReadDir(s.Config.BackupDir)
	if err != nil {
		return nil
	}
	var saves []Save
	for _, entry := range entries {
		name := entry.Name()
		if !backupNameRe.MatchString(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		saves = append(saves, Save{Name: name, Size: info.Size(), Time: info.ModTime().Unix()})
	}
	sort.Slice(saves, func(i, j int) bool { return saves[i].Time > saves[j].Time })
	return saves
}

func (s Service) Backup() (bool, string) {
	out, err := exec.Command("/usr/local/bin/pal-backup").CombinedOutput()
	return err == nil, string(out)
}

func (s Service) SavePath(name string) (string, error) {
	if !backupNameRe.MatchString(name) {
		return "", fmt.Errorf("无效文件名（必须 pal_backup_*.tar.gz）")
	}
	path := filepath.Join(s.Config.BackupDir, name)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.Config.BackupDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("无效路径")
	}
	return path, nil
}

func (s Service) UploadSave(name, content string) (int, error) {
	path, err := s.SavePath(name)
	if err != nil {
		return 0, err
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return 0, fmt.Errorf("无效的文件内容")
	}
	if len(data) > 500*1024*1024 {
		return 0, fmt.Errorf("文件过大（>500MB）")
	}
	if err := validateTarGz(data); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(s.Config.BackupDir, 0755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, err
	}
	return len(data), nil
}

func validateTarGz(data []byte) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("无效的 gzip 备份")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("无效的 tar 备份")
		}
		clean := filepath.Clean(h.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("备份包包含不安全路径")
		}
	}
	return nil
}

func (s Service) DeleteSave(name string) error {
	path, err := s.SavePath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s Service) RestoreSave(name string) (bool, string) {
	path, err := s.SavePath(name)
	if err != nil {
		return false, err.Error()
	}
	if _, err := os.Stat(path); err != nil {
		return false, "备份文件不存在"
	}
	tmpDir, err := os.MkdirTemp("", "pal_restore_")
	if err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(tmpDir)
	if err := extractSaveArchive(path, tmpDir); err != nil {
		return false, err.Error()
	}
	src := filepath.Join(tmpDir, "SaveGames")
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return false, "备份包内无 SaveGames 目录"
	}
	if _, err := s.RCON("Save"); err != nil {
		return false, err.Error()
	}
	_ = exec.Command("systemctl", "stop", s.Config.Service).Run()
	if err := replaceSaveGames(src, s.Config.SaveGamesDir); err != nil {
		_ = exec.Command("systemctl", "start", s.Config.Service).Run()
		return false, err.Error()
	}
	_ = exec.Command("chown", "-R", "steam:steam", s.Config.SaveGamesDir).Run()
	_ = exec.Command("systemctl", "start", s.Config.Service).Run()
	return true, "恢复完成，服务器正在重启"
}

func extractSaveArchive(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("无效的 gzip 备份")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(dest)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("无效的 tar 备份")
		}
		cleanName := filepath.Clean(h.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("备份包包含不安全路径")
		}
		target := filepath.Join(cleanDest, cleanName)
		if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) && filepath.Clean(target) != cleanDest {
			return fmt.Errorf("备份包包含不安全路径")
		}
		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(target, h.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, h.FileInfo().Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func replaceSaveGames(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "banlist.txt" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return copyDirContents(src, dst)
}

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := os.MkdirAll(to, info.Mode()); err != nil {
				return err
			}
			if err := copyDirContents(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (s Service) Whitelist() []WhitelistEntry {
	data, err := os.ReadFile(s.Config.WhitelistFile)
	if err != nil {
		return nil
	}
	var entries []WhitelistEntry
	_ = json.Unmarshal(data, &entries)
	return entries
}

func (s Service) SaveWhitelist(entries []WhitelistEntry) error {
	data, _ := json.MarshalIndent(entries, "", "  ")
	return os.WriteFile(s.Config.WhitelistFile, data, 0644)
}

func (s Service) Banlist() []BanEntry {
	data, err := os.ReadFile(filepath.Join(s.Config.SaveGamesDir, "banlist.txt"))
	if err != nil {
		return nil
	}
	var out []BanEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, BanEntry{SteamID: line})
		}
	}
	return out
}
