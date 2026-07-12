//go:build !linux

package app

import "os"

func processAlivePlatform(process *os.Process) bool {
	return true
}
