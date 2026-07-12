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

func TestValidateDeployedGameRuntimeSpecs(t *testing.T) {
	tests := []struct {
		gameID string
		file   string
	}{
		{gameID: "rust", file: "RustDedicated"},
		{gameID: "satisfactory", file: "FactoryServer.sh"},
		{gameID: "l4d2", file: "srcds_run"},
		{gameID: "7-days-to-die", file: "startserver.sh"},
	}
	for _, tt := range tests {
		t.Run(tt.gameID, func(t *testing.T) {
			dir := t.TempDir()
			if err := validateDeployedGame(tt.gameID, dir); err == nil {
				t.Fatal("expected missing runtime file to fail")
			}
			path := filepath.Join(dir, filepath.FromSlash(tt.file))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := validateDeployedGame(tt.gameID, dir); err != nil {
				t.Fatalf("expected runtime file to pass: %v", err)
			}
		})
	}
}

func TestRuntimeSpecStartCommands(t *testing.T) {
	if got := startCommandForGame("rust"); got == "" || got == "./start_server.sh" {
		t.Fatalf("expected rust to have a concrete start command, got %q", got)
	}
	if got := stopCommandForGame("team-fortress-2", "ctrl+c"); got != "quit" {
		t.Fatalf("expected source server stop command quit, got %q", got)
	}
}

func TestRuntimeSpecUnsupportedGames(t *testing.T) {
	if spec := gameRuntimeSpec("dont-starve-together"); spec.ManualReason == "" {
		t.Fatal("expected don't starve together to require manual preparation")
	}
	if spec := gameRuntimeSpec("unknown-game"); spec.UnsupportedReason == "" {
		t.Fatal("expected unknown game to be unsupported")
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/a b/o'clock")
	want := "'/tmp/a b/o'\\''clock'"
	if got != want {
		t.Fatalf("shellQuote mismatch: got %q want %q", got, want)
	}
}
