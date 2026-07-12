package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureNoFileConflict(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "server.cfg")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureNoFileConflict(target, false); err == nil {
		t.Fatal("expected existing target to fail without overwrite")
	}
	if err := ensureNoFileConflict(target, true); err != nil {
		t.Fatalf("expected overwrite to allow existing target: %v", err)
	}
	if err := ensureNoFileConflict(filepath.Join(dir, "new.cfg"), false); err != nil {
		t.Fatalf("expected missing target to pass: %v", err)
	}
}

func TestSearchFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cfg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cfg", "ServerSettings.ini"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	items, truncated, err := searchFiles(dir, "server", 20)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("did not expect truncated results")
	}
	if len(items) != 1 || items[0].Name != "ServerSettings.ini" {
		t.Fatalf("unexpected search results: %#v", items)
	}
}

func TestSearchFilesLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, "server-"+string(rune('a'+i))+".cfg"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	items, truncated, err := searchFiles(dir, "server", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || !truncated {
		t.Fatalf("expected 2 truncated results, got len=%d truncated=%v", len(items), truncated)
	}
}
