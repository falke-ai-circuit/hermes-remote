//go:build !windows

package agent

import "syscall"

// getSysProcAttr returns Unix-specific process attributes to detach
// the new process from the current one.
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true, // detach from process group
	}
}