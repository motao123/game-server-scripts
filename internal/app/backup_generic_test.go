package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupNameValidation(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"daily", true},
		{"daily_01-02.3", true},
		{"", false},
		{"../daily", false},
		{"daily/one", false},
		{"daily one", false},
		{`C:\daily`, false},
	}

	for _, tc := range cases {
		if got := validBackupPathPart(tc.name); got != tc.valid {
			t.Fatalf("validBackupPathPart(%q) = %v, want %v", tc.name, got, tc.valid)
		}
	}

	archives := []struct {
		name  string
		valid bool
	}{
		{"2026-01-01_00-00-00.tar.gz", true},
		{"backup.tar.gz", true},
		{"backup.zip", false},
		{"../backup.tar.gz", false},
		{"dir/backup.tar.gz", false},
		{".tar.gz", false},
	}

	for _, tc := range archives {
		if got := validBackupArchiveName(tc.name); got != tc.valid {
			t.Fatalf("validBackupArchiveName(%q) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}

func TestInsideRootRejectsSiblingPrefix(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	if !insideRoot(root, filepath.Join(root, "group", "file.tar.gz")) {
		t.Fatal("insideRoot should allow child paths")
	}
	if !insideRoot(root, root) {
		t.Fatal("insideRoot should allow root itself")
	}
	if insideRoot(root, root+"2"+string(os.PathSeparator)+"file.tar.gz") {
		t.Fatal("insideRoot allowed sibling prefix path")
	}
	if insideRoot(root, filepath.Join(root, "..", "outside.tar.gz")) {
		t.Fatal("insideRoot allowed escaped path")
	}
}

func TestBackupManagerCreateGroupsArchivePathAndDelete(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "world.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewBackupManager(root, func(path string) bool { return path == source })
	archive, err := mgr.Create("daily", source, 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive missing: %v", err)
	}

	metaData, err := os.ReadFile(filepath.Join(root, "daily", "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta BackupMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("invalid metadata json: %v", err)
	}
	if meta.SourcePath != source {
		t.Fatalf("metadata sourcePath = %q, want %q", meta.SourcePath, source)
	}

	groups := mgr.Groups()
	if len(groups) != 1 || groups[0].Name != "daily" || len(groups[0].Files) != 1 {
		t.Fatalf("unexpected groups: %#v", groups)
	}

	fileName := filepath.Base(archive)
	path, err := mgr.ArchivePath("daily", fileName)
	if err != nil {
		t.Fatalf("ArchivePath failed: %v", err)
	}
	if path != archive {
		t.Fatalf("ArchivePath = %q, want %q", path, archive)
	}
	if _, err := mgr.ArchivePath("../daily", fileName); err == nil {
		t.Fatal("ArchivePath accepted invalid group name")
	}
	if _, err := mgr.ArchivePath("daily", "../x.tar.gz"); err == nil {
		t.Fatal("ArchivePath accepted invalid archive name")
	}

	if err := mgr.Delete("daily", fileName); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("archive still exists after delete: %v", err)
	}
}

func TestBackupManagerRestoreRejectsInvalidInputs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	groupDir := filepath.Join(root, "daily")
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groupDir, "data.json"), []byte(`{"sourcePath":"/allowed/source"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeTarGz(filepath.Join(groupDir, "ok.tar.gz"), map[string]string{"source/file.txt": "ok"}); err != nil {
		t.Fatal(err)
	}

	mgr := NewBackupManager(root, func(path string) bool { return false })
	if _, err := mgr.Restore("daily", "ok.tar.gz"); err == nil {
		t.Fatal("Restore should reject disallowed sourcePath")
	}
	if _, err := mgr.Restore("../daily", "ok.tar.gz"); err == nil {
		t.Fatal("Restore should reject invalid group name")
	}
	if _, err := mgr.Restore("daily", "../ok.tar.gz"); err == nil {
		t.Fatal("Restore should reject invalid file name")
	}
}

func TestEnforceRetentionRemovesOldestArchives(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.tar.gz")
	mid := filepath.Join(dir, "mid.tar.gz")
	newest := filepath.Join(dir, "new.tar.gz")
	for _, path := range []string{old, mid, newest} {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	_ = os.Chtimes(old, now.Add(-3*time.Hour), now.Add(-3*time.Hour))
	_ = os.Chtimes(mid, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	_ = os.Chtimes(newest, now.Add(-time.Hour), now.Add(-time.Hour))

	enforceRetention(dir, 2)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("oldest archive should be removed: %v", err)
	}
	for _, path := range []string{mid, newest} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("archive should remain %s: %v", path, err)
		}
	}
}

func writeTarGz(path string, files map[string]string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for name, content := range files {
		h := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}
