package app

import (
	"path/filepath"
	"testing"

	"game-server-scripts/internal/config"
)

func TestSafeRootAllowsConfiguredRootsAndRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	pal := filepath.Join(root, "pal")
	backups := filepath.Join(root, "backups")
	data := filepath.Join(root, "data")
	settings := filepath.Join(root, "settings", "PalWorldSettings.ini")
	s := &Server{cfg: config.Config{PalServerDir: pal, BackupDir: backups, DataDir: data, PalSettings: settings}}

	allowed := []string{
		pal,
		filepath.Join(pal, "Pal", "Saved"),
		backups,
		filepath.Join(backups, "daily", "one.tar.gz"),
		data,
		filepath.Dir(settings),
	}
	for _, path := range allowed {
		if !s.safeRoot(path) {
			t.Fatalf("safeRoot(%q) = false, want true", path)
		}
	}

	denied := []string{
		root,
		pal + "2",
		filepath.Join(pal, "..", "outside"),
		filepath.Join(backups, "..", "outside"),
	}
	for _, path := range denied {
		if s.safeRoot(path) {
			t.Fatalf("safeRoot(%q) = true, want false", path)
		}
	}
}
