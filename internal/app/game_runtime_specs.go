package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type GameRuntimeSpec struct {
	StartCommand      string
	StopCommand       string
	RequiredFiles     []string
	Linux             bool
	RunAsSteam        bool
	ManualReason      string
	UnsupportedReason string
}

func gameRuntimeSpec(gameID string) GameRuntimeSpec {
	specs := map[string]GameRuntimeSpec{
		"palworld":             {StartCommand: "runuser -u steam -- ./PalServer.sh", StopCommand: "stop", RequiredFiles: []string{"PalServer.sh"}, Linux: true, RunAsSteam: true},
		"rust":                 {StartCommand: "runuser -u steam -- ./RustDedicated -batchmode +server.port 28015 +server.queryport 28016 +server.identity default +server.hostname \"Rust Server\" +server.maxplayers 50", StopCommand: "ctrl+c", RequiredFiles: []string{"RustDedicated"}, Linux: true, RunAsSteam: true},
		"satisfactory":         {StartCommand: "runuser -u steam -- ./FactoryServer.sh", StopCommand: "ctrl+c", RequiredFiles: []string{"FactoryServer.sh", "Engine/Binaries/Linux/FactoryServer-Linux-Shipping"}, Linux: true, RunAsSteam: true},
		"l4d2":                 {StartCommand: "./srcds_run -game left4dead2 -console -usercon +map c5m1_waterfront +maxplayers 8 -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"team-fortress-2":      {StartCommand: "./srcds_run -game tf -console -usercon +map ctf_2fort +maxplayers 24 -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"insurgency-2014":      {StartCommand: "./srcds_run -game insurgency -console -usercon +map ministry_coop -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"no-more-room-in-hell": {StartCommand: "./srcds_run -game nmrih -console -usercon +map nmo_broadway -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"half-life":            {StartCommand: "./hlds_run -game valve +map crossfire +maxplayers 16 -port 27015", StopCommand: "quit", RequiredFiles: []string{"hlds_run"}, Linux: true},
		"half-life2":           {StartCommand: "./srcds_run -game hl2mp -console -usercon +map dm_lockdown +maxplayers 16 -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"black-mesa":           {StartCommand: "./srcds_run -game bms -console -usercon +map dm_power -port 27015", StopCommand: "quit", RequiredFiles: []string{"srcds_run"}, Linux: true},
		"valheim":              {StartCommand: "runuser -u steam -- ./start_server.sh", StopCommand: "ctrl+c", RequiredFiles: []string{"valheim_server.x86_64", "start_server.sh"}, Linux: true, RunAsSteam: true},
		"7-days-to-die":        {StartCommand: "runuser -u steam -- ./startserver.sh -configfile=serverconfig.xml", StopCommand: "shutdown", RequiredFiles: []string{"startserver.sh", "7DaysToDieServer.x86_64"}, Linux: true, RunAsSteam: true},
		"project-zomboid":      {StartCommand: "runuser -u steam -- ./start-server.sh", StopCommand: "quit", RequiredFiles: []string{"start-server.sh", "ProjectZomboid64"}, Linux: true, RunAsSteam: true},
		"barotrauma":           {StartCommand: "runuser -u steam -- ./DedicatedServer", StopCommand: "ctrl+c", RequiredFiles: []string{"DedicatedServer"}, Linux: true, RunAsSteam: true},
		"avorion":              {StartCommand: "runuser -u steam -- ./server.sh --galaxy-name default", StopCommand: "stop", RequiredFiles: []string{"server.sh", "bin/AvorionServer"}, Linux: true, RunAsSteam: true},
	}
	if spec, ok := specs[gameID]; ok {
		return spec
	}
	manual := map[string]string{
		"dont-starve-together":     "饥荒联机版需要从客户端上传世界、cluster.ini 和 token，暂不支持一键创建可游玩实例",
		"american-truck-simulator": "美国卡车模拟需要从客户端生成并上传服务端配置，暂不支持一键创建可游玩实例",
		"euro-truck-simulator-2":   "欧洲卡车模拟2需要从客户端生成并上传服务端配置，暂不支持一键创建可游玩实例",
		"eco":                      "ECO 需要额外生成配置或账号初始化，暂不支持一键创建可游玩实例",
		"starbound":                "Starbound 服务端需要授权验证，暂不支持匿名一键部署",
		"outworlder":               "Outworlder 需要 Steam 账号验证，暂不支持匿名一键部署",
		"arma-3":                   "Arma 3 需要 Steam 账号验证和服务端配置，暂不支持匿名一键部署",
		"dayz":                     "DayZ 需要 Steam 账号验证，暂不支持匿名一键部署",
		"mindustry":                "Mindustry 需要 Steam 账号验证，暂不支持匿名一键部署",
		"assetto-corsa":            "Assetto Corsa 需要 Steam 账号验证和外部配置，暂不支持匿名一键部署",
	}
	if reason, ok := manual[gameID]; ok {
		return GameRuntimeSpec{ManualReason: reason}
	}
	return GameRuntimeSpec{UnsupportedReason: "该游戏还没有本地可验证的 Linux 启动适配，暂不创建不可启动实例"}
}

func (spec GameRuntimeSpec) status() (string, string) {
	if spec.Linux {
		return "linux-ready", "Linux 一键部署后可创建实例"
	}
	if spec.ManualReason != "" {
		return "manual", spec.ManualReason
	}
	return "unsupported", spec.UnsupportedReason
}

func ensureGameDeployable(game GameTemplate) (GameRuntimeSpec, error) {
	if game.Type == "minecraft-java" {
		return GameRuntimeSpec{Linux: true}, nil
	}
	if game.ID == "minecraft-bedrock" || game.ID == "terraria" {
		return GameRuntimeSpec{}, fmt.Errorf("%s 尚未接入可验证的自动安装流程，暂不创建不可启动实例", game.Name)
	}
	spec := gameRuntimeSpec(game.ID)
	if runtime.GOOS != "linux" {
		return spec, fmt.Errorf("当前面板运行在 %s，SteamCMD 游戏一键部署仅支持 Linux 服务器", runtime.GOOS)
	}
	if !spec.Linux {
		_, reason := spec.status()
		return spec, fmt.Errorf(reason)
	}
	return spec, nil
}

func startCommandForGame(gameID string) string {
	if spec := gameRuntimeSpec(gameID); spec.StartCommand != "" {
		return spec.StartCommand
	}
	return ""
}

func stopCommandForGame(gameID string, fallback string) string {
	if spec := gameRuntimeSpec(gameID); spec.StopCommand != "" {
		return spec.StopCommand
	}
	return fallback
}

func validateRuntimeSpec(gameID, path string) error {
	spec := gameRuntimeSpec(gameID)
	if len(spec.RequiredFiles) == 0 {
		return nil
	}
	for _, name := range spec.RequiredFiles {
		if fileExists(filepath.Join(path, filepath.FromSlash(name))) {
			return nil
		}
	}
	return fmt.Errorf("%s 服务端文件不完整，缺少 %s", gameID, strings.Join(spec.RequiredFiles, " 或 "))
}
