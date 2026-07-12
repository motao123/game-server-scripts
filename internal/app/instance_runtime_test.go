package app

import "testing"

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
