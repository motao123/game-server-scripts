package install

import (
	"fmt"
	"os"
	"os/exec"
)

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing install target")
	}
	switch args[0] {
	case "web":
		return installWeb()
	case "palworld", "minecraft", "valheim", "terraria":
		return fmt.Errorf("%s installer is staged for migration; use existing script wrapper for now", args[0])
	default:
		return fmt.Errorf("unknown install target %q", args[0])
	}
}

func installWeb() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := copyFile(self, "/usr/local/bin/gsm-panel"); err != nil {
		return err
	}
	service := `[Unit]
Description=GSM Go Panel
After=network-online.target pal-server.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/pal-web.env
ExecStart=/usr/local/bin/gsm-panel web
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pal-web

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/pal-web.service", []byte(service), 0644); err != nil {
		return err
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", out, err)
	}
	if out, err := exec.Command("systemctl", "enable", "pal-web").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", out, err)
	}
	if out, err := exec.Command("systemctl", "restart", "pal-web").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl restart: %s: %w", out, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return err
	}
	return os.Chmod(dst, 0755)
}
