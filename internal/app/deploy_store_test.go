package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDeployTaskStoreRoundTrip(t *testing.T) {
	store := NewDeployTaskStore(filepath.Join(t.TempDir(), "deploy_tasks.json"))
	now := time.Now().Round(time.Second)
	input := []DeployTask{{ID: "task-1", GameID: "rust", GameName: "Rust", Path: "/srv/rust", Status: "failed", Error: "network", StartedAt: now}}
	if err := store.Save(input); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "task-1" || got[0].Error != "network" {
		t.Fatalf("unexpected persisted tasks: %#v", got)
	}
}

func TestDeployPreflightInvalidPath(t *testing.T) {
	m := NewDeployManager(nil)
	result := m.Preflight(GameTemplate{ID: "minecraft-java", Name: "Minecraft Java"}, "relative/path")
	if result.Ready {
		t.Fatalf("expected relative install path to fail preflight: %#v", result)
	}
	if len(result.Problems) == 0 {
		t.Fatal("expected preflight problem")
	}
}
