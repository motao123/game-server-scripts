package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	WebPassword   string
	JWTSecret     string
	Bind          string
	Port          int
	Service       string
	RCONPort      int
	RCONPassword  string
	RESTAPIPort   int
	DataDir       string
	PalServerDir  string
	PalSettings   string
	SaveGamesDir  string
	BackupDir     string
	WhitelistFile string
}

func Load() Config {
	loadEnvFile("/etc/pal-web.env")
	palDir := env("PAL_SERVER_DIR", "/home/steam/Steam/steamapps/common/PalServer")
	cfg := Config{
		WebPassword:   os.Getenv("WEB_PASSWORD"),
		JWTSecret:     env("JWT_SECRET", randomHex(32)),
		Bind:          env("WEB_BIND", "0.0.0.0"),
		Port:          envInt("WEB_PORT", 8080),
		Service:       env("SERVICE", "pal-server"),
		RCONPort:      envInt("RCON_PORT", 25575),
		RCONPassword:  os.Getenv("RCON_PASS"),
		RESTAPIPort:   envInt("REST_API_PORT", 8212),
		DataDir:       env("GSM_DATA_DIR", "./data"),
		PalServerDir:  palDir,
		PalSettings:   env("PAL_SETTINGS", palDir+"/Pal/Saved/Config/LinuxServer/PalWorldSettings.ini"),
		SaveGamesDir:  env("SAVE_GAMES_DIR", palDir+"/Pal/Saved/SaveGames"),
		BackupDir:     env("BACKUP_DIR", "/home/steam/pal-backups"),
		WhitelistFile: env("WHITELIST_FILE", "/etc/pal-whitelist.json"),
	}
	return cfg
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "'")
		value = strings.Trim(value, "\"")
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "change-me"
	}
	return hex.EncodeToString(b)
}
