//go:build windows

package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func setReuseAddr(fd uintptr) error {
	err := windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	if err != nil {
		return fmt.Errorf("SetsockoptInt SO_REUSEADDR failed: %w", err)
	}
	return nil
}