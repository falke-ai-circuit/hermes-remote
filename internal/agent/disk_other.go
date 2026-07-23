//go:build !linux

package agent

import (
	"github.com/falke-ai-circuit/probe/internal/protocol"
)

// statfsDisk is a no-op stub for non-Linux platforms.
func statfsDisk(path string) protocol.SysDiskInfo {
	return protocol.SysDiskInfo{Path: path}
}

// getUptimeSeconds is a stub for non-Linux platforms.
func getUptimeSeconds() int64 {
	return 0
}