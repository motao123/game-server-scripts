package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupCreatePreflight(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	source := filepath.Join(t.TempDir(), "source")
	mgr := NewBackupManager(root, func(path string) bool { return path == source })
	if check := mgr.CreatePreflight("daily", source); check.Ready {
		t.Fatalf("expected missing source to fail: %#v", check)
	}
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if check := mgr.CreatePreflight("daily", source); !check.Ready {
		t.Fatalf("expected valid source to pass: %#v", check)
	}
}

func TestBackupRestorePreflightAndOverwrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "old.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	mgr := NewBackupManager(root, func(path string) bool { return path == source })
	group := filepath.Join(root, "daily")
	if err := os.MkdirAll(group, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "data.json"), []byte(`{"sourcePath":"`+source+`"}`), 0644); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(group, "good.tar.gz")
	if err := writeTarGz(archive, map[string]string{"source/new.txt": "new"}); err != nil {
		t.Fatal(err)
	}
	check := mgr.RestorePreflight("daily", "good.tar.gz")
	if !check.Ready || check.ArchiveEntries != 1 || len(check.Warnings) == 0 {
		t.Fatalf("unexpected restore preflight: %#v", check)
	}
	if _, err := mgr.Restore("daily", "good.tar.gz"); err == nil {
		t.Fatal("expected restore without overwrite confirmation to fail")
	}
	if _, err := mgr.RestoreWithOptions("daily", "good.tar.gz", true); err != nil {
		t.Fatalf("restore with overwrite failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "source", "new.txt")); err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
}

func TestValidateTarGzRejectsInvalidArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(path, []byte("not gzip"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := validateTarGz(path); err == nil {
		t.Fatal("expected invalid gzip archive to fail")
	}
}
