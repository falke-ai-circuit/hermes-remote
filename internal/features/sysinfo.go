package features

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

type SystemInfo struct {
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	CPUs         int       `json:"cpus"`
	GoVersion    string    `json:"goVersion"`
	IPAddresses  []string  `json:"ipAddresses"`
	BootTime     time.Time `json:"bootTime"`
	ProcessCount int       `json:"processCount"`
}

func CollectSystemInfo() (*SystemInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	info := &SystemInfo{
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
		GoVersion: runtime.Version(),
		BootTime:  time.Now(),
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				info.IPAddresses = append(info.IPAddresses, ipnet.IP.String())
			}
		}
	}
	return info, nil
}

func FormatSystemReport(info *SystemInfo) string {
	var sb strings.Builder
	sb.WriteString("=== PROBE Client - System Diagnostic Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Hostname:       %s\n", info.Hostname))
	sb.WriteString(fmt.Sprintf("Operating System: %s\n", info.OS))
	sb.WriteString(fmt.Sprintf("Architecture:    %s\n", info.Arch))
	sb.WriteString(fmt.Sprintf("CPU Cores:       %d\n", info.CPUs))
	sb.WriteString(fmt.Sprintf("Go Runtime:      %s\n", info.GoVersion))
	sb.WriteString(fmt.Sprintf("Boot Time:       %s\n", info.BootTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("IP Addresses:    %s\n", strings.Join(info.IPAddresses, ", ")))
	sb.WriteString("\n=== End of Report ===\n")
	return sb.String()
}

func GetDiskUsage(path string) (total, used, free int64, err error) {
	return 0, 0, 0, nil
}

func GetMemoryInfo() (total, used, free int64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys), int64(m.Alloc), int64(m.Sys - m.Alloc)
}
