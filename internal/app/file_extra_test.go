package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarGzSkipsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "archive.tar.gz")
	dest := filepath.Join(root, "dest")
	outside := filepath.Join(root, "evil.txt")

	if err := writeTarGz(archive, map[string]string{
		"safe/file.txt": "safe",
		"../evil.txt":   "evil",
	}); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "safe", "file.txt"))
	if err != nil {
		t.Fatalf("safe file missing: %v", err)
	}
	if string(data) != "safe" {
		t.Fatalf("safe file content = %q", string(data))
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("unsafe file should not be written outside dest: %v", err)
	}
}

func TestExtractTarGzRejectsInvalidGzip(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(archive, []byte("not gzip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(archive, t.TempDir()); err == nil {
		t.Fatal("extractTarGz should reject invalid gzip data")
	}
}
