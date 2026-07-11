//go:build !linux

package system

func diskUsage(path string) Disk {
	return Disk{}
}
