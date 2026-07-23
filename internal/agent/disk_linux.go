//go:build linux

package agent

import (
	"fmt"
	"os"
	"syscall"

	"github.com/falke-ai-circuit/probe/internal/protocol"
)

// statfsDisk returns disk usage for a given mount point using syscall.Statfs.
func statfsDisk(path string) protocol.SysDiskInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return protocol.SysDiskInfo{Path: path}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	return protocol.SysDiskInfo{
		Path:       path,
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  used,
	}
}

// getUptimeSeconds returns system uptime in seconds by reading /proc/uptime.
func getUptimeSeconds() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	var uptime float64
	fmt.Sscanf(string(data), "%f", &uptime)
	return int64(uptime)
}