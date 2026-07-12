//go:build linux

package app

import (
	"path/filepath"
	"testing"
)

func TestSyncInstanceStatusMarksDeadPIDAsError(t *testing.T) {
	store := NewInstanceStore(filepath.Join(t.TempDir(), "instances.json"))
	store.list = []Instance{{ID: "dead", Name: "dead", WorkingDirectory: "/tmp", StartCommand: "./start.sh", Status: "running", PID: 99999999, InstanceType: "generic"}}
	server := &Server{instances: store}
	updated := server.syncInstanceStatus(store.list[0])
	if updated.Status != "error" || updated.PID != 0 || updated.LastError == "" {
		t.Fatalf("expected dead PID to become error, got %#v", updated)
	}
	stored, ok := store.Get("dead")
	if !ok || stored.Status != "error" || stored.LastError == "" {
		t.Fatalf("expected stored instance error, got %#v", stored)
	}
}
