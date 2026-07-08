//go:build windows

package agent

import "syscall"

// getSysProcAttr returns Windows-specific process attributes to detach
// the new process from the current one.
// CREATE_NEW_PROCESS_GROUP (0x00000200) | DETACHED_PROCESS (0x00000008) = 0x00000208
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: 0x00000208,
	}
}