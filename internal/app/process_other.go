//go:build !linux

package app

import (
	"os"
	"os/exec"
)

func setSysProcAttr(c *exec.Cmd) {}

func stopProcess(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Signal(os.Interrupt)
	}
}

func killProcess(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
	}
}
