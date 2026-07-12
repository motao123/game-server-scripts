package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInstanceForSave(t *testing.T) {
	tests := []struct {
		name string
		inst Instance
		ok   bool
	}{
		{name: "valid generic", inst: Instance{Name: "srv", WorkingDirectory: "/srv/game", StartCommand: "./start.sh", InstanceType: "generic"}, ok: true},
		{name: "missing name", inst: Instance{WorkingDirectory: "/srv/game", StartCommand: "./start.sh", InstanceType: "generic"}, ok: false},
		{name: "missing directory", inst: Instance{Name: "srv", StartCommand: "./start.sh", InstanceType: "generic"}, ok: false},
		{name: "missing generic command", inst: Instance{Name: "srv", WorkingDirectory: "/srv/game", InstanceType: "generic"}, ok: false},
		{name: "minecraft java may omit command", inst: Instance{Name: "mc", WorkingDirectory: "/srv/mc", InstanceType: "minecraft-java"}, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateInstanceForSave(tt.inst) == nil
			if got != tt.ok {
				t.Fatalf("validateInstanceForSave ok=%v want %v", got, tt.ok)
			}
		})
	}
}

func TestInstanceRuntimeReadiness(t *testing.T) {
	dir := t.TempDir()
	store := NewInstanceStore(filepath.Join(t.TempDir(), "instances.json"))
	store.list = nil
	rt := NewInstanceRuntime(store)
	missing := rt.CheckReadiness(Instance{Name: "bad", WorkingDirectory: dir, StartCommand: "./missing.sh", InstanceType: "generic"})
	if missing.Ready {
		t.Fatal("expected missing script to be not ready")
	}
	if err := os.WriteFile(filepath.Join(dir, "server.jar"), []byte("jar"), 0644); err != nil {
		t.Fatal(err)
	}
	mc := rt.CheckReadiness(Instance{Name: "mc", WorkingDirectory: dir, InstanceType: "minecraft-java"})
	if !mc.Ready || mc.Command != "java -jar server.jar nogui" {
		t.Fatalf("expected minecraft java readiness with detected jar, got ready=%v command=%q problems=%v", mc.Ready, mc.Command, mc.Problems)
	}
}
