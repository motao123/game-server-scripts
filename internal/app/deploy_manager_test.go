package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDeployedGamePalworld(t *testing.T) {
	dir := t.TempDir()
	if err := validateDeployedGame("palworld", dir); err == nil {
		t.Fatal("expected empty Palworld directory to fail validation")
	}
	if err := os.WriteFile(filepath.Join(dir, "PalServer.sh"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := validateDeployedGame("palworld", dir); err != nil {
		t.Fatalf("expected Palworld script to pass validation: %v", err)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a b/o'clock")
	want := "'/tmp/a b/o'\\''clock'"
	if got != want {
		t.Fatalf("shellQuote mismatch: got %q want %q", got, want)
	}
}
