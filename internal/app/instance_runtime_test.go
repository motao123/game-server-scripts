package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSystemctlStartService(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
		ok      bool
	}{
		{name: "plain service", command: "systemctl start pal-server", want: "pal-server", ok: true},
		{name: "service suffix", command: "systemctl start pal-server.service", want: "pal-server.service", ok: true},
		{name: "extra args rejected", command: "systemctl start pal-server --now", ok: false},
		{name: "non start rejected", command: "systemctl restart pal-server", ok: false},
		{name: "shell metachar rejected", command: "systemctl start pal-server;reboot", ok: false},
		{name: "normal command rejected", command: "./start.sh", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := systemctlStartService(tt.command)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("systemctlStartService(%q) = %q, %v; want %q, %v", tt.command, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestValidateStartCommandFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "start.sh")
	mode := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		mode = 0644
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	if err := validateStartCommandFile(dir, "./start.sh --flag"); err != nil {
		t.Fatalf("expected existing script to pass: %v", err)
	}
	if err := validateStartCommandFile(dir, "runuser -u steam -- ./start.sh --flag"); err != nil {
		t.Fatalf("expected runuser wrapped script to pass: %v", err)
	}
	if err := validateStartCommandFile(dir, "./missing.sh"); err == nil {
		t.Fatal("expected missing script to fail")
	}
	if err := validateStartCommandFile(dir, "runuser -u steam -- ./missing.sh"); err == nil {
		t.Fatal("expected missing runuser wrapped script to fail")
	}
	if err := validateStartCommandFile(dir, "java -jar server.jar"); err != nil {
		t.Fatalf("expected PATH command to skip file validation: %v", err)
	}
}
