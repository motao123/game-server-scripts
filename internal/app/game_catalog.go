package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var catalogCache struct {
	sync.Mutex
	games []GameTemplate
}

func (s *Server) games() []GameTemplate {
	catalogCache.Lock()
	defer catalogCache.Unlock()
	if len(catalogCache.games) == 0 {
		catalogCache.games = loadGameCatalog()
	}
	out := make([]GameTemplate, len(catalogCache.games))
	copy(out, catalogCache.games)
	return out
}

func reloadGameCatalog() []GameTemplate {
	catalogCache.Lock()
	defer catalogCache.Unlock()
	catalogCache.games = loadGameCatalog()
	out := make([]GameTemplate, len(catalogCache.games))
	copy(out, catalogCache.games)
	return out
}

func loadGameCatalog() []GameTemplate {
	paths := []string{
		filepath.Join("data", "game_catalog.json"),
		filepath.Join(".", "game_catalog.json"),
		filepath.Join("/opt/gsm-panel", "data", "game_catalog.json"),
		filepath.Join("/usr/local/share/gsm-panel", "data", "game_catalog.json"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var games []GameTemplate
		if err := json.Unmarshal(data, &games); err != nil || len(games) == 0 {
			continue
		}
		for i := range games {
			games[i].normalize()
		}
		sort.SliceStable(games, func(i, j int) bool {
			if games[i].Type == games[j].Type {
				return games[i].Name < games[j].Name
			}
			return games[i].Type < games[j].Type
		})
		return games
	}
	return builtinGames()
}

func (g *GameTemplate) normalize() {
	if g.Type == "" {
		g.Type = "steam"
	}
	if g.Description == "" {
		if g.NameCN != "" {
			g.Description = g.NameCN
		} else {
			g.Description = g.Name
		}
	}
	if len(g.Ports) == 0 && len(g.PortMappings) > 0 {
		seen := map[int]bool{}
		for _, item := range g.PortMappings {
			if item.Port > 0 && !seen[item.Port] {
				g.Ports = append(g.Ports, item.Port)
				seen[item.Port] = true
			}
		}
		sort.Ints(g.Ports)
	}
	if len(g.Tags) == 0 {
		g.Tags = []string{"SteamCMD"}
	}
	if g.DefaultPath == "" {
		g.DefaultPath = filepath.Join("/home/steam/Steam/steamapps/common", g.ID)
	}
	if g.Type == "steam" {
		spec := gameRuntimeSpec(g.ID)
		g.Support, g.SupportNote = spec.status()
	}
}
