//go:build linux

package app

import (
	"os"
	"syscall"
)

func processAlivePlatform(process *os.Process) bool {
	return process.Signal(syscall.Signal(0)) == nil
}
