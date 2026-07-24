// Template file for the anti-debug evasion package (non-Windows variant).
// This is NOT compiled — it's embedded into the obfuscation tool as a string
// and written to the target repo during -antidebug phase.

package evasion

import (
	"os"
	"runtime"
	"time"
)

func checkEnvironment() bool {
	if runtime.NumCPU() < 2 {
		return false
	}
	start := time.Now()
	time.Sleep(2 * time.Second)
	if time.Since(start) < 1500*time.Millisecond {
		return false
	}
	return true
}

func init() {
	if !checkEnvironment() {
		os.Exit(0)
	}
}