//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/falke-ai-circuit/hermes-remote/internal/protocol"
)

// New creates a Windows platform with the given agent name.
func New(name string) Platform {
	p := &windowsPlatform{name: name}
	return p
}

type windowsPlatform struct {
	name string
}

// Filesystem — use generic Go stdlib (works cross-platform)
func (p *windowsPlatform) ListDir(path string) ([]protocol.FSEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]protocol.FSEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, protocol.FSEntry{
			Name:    e.Name(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Unix(),
			IsDir:   e.IsDir(),
		})
	}
	return result, nil
}

func (p *windowsPlatform) FileStat(path string) (protocol.FSStatResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return protocol.FSStatResult{Exists: false}, nil
		}
		return protocol.FSStatResult{}, err
	}
	return protocol.FSStatResult{
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().Unix(),
		IsDir:   info.IsDir(),
		Exists:  true,
	}, nil
}

func (p *windowsPlatform) ReadFile(path string, offset int, limit int) (protocol.FSReadResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return protocol.FSReadResult{}, err
	}
	if offset > 0 && offset < len(raw) {
		raw = raw[offset:]
	} else if offset >= len(raw) {
		raw = nil
	}
	if limit > 0 && limit < len(raw) {
		raw = raw[:limit]
	}
	size := int64(len(raw))
	return protocol.FSReadResult{Data: base64Encode(raw), Size: size, Encoding: "base64"}, nil
}

func (p *windowsPlatform) WriteFile(path string, data []byte, mode string) (protocol.FSWriteResult, error) {
	dir := path
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '\\' || dir[i] == '/' {
			dir = dir[:i]
			break
		}
	}
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}
	perm := os.FileMode(0644)
	if mode != "" {
		var m uint32
		if _, err := fmt.Sscanf(mode, "%o", &m); err == nil {
			perm = os.FileMode(m)
		}
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return protocol.FSWriteResult{}, err
	}
	return protocol.FSWriteResult{Written: len(data), Path: path}, nil
}

func (p *windowsPlatform) DeleteFile(path string) (protocol.FSDeleteResult, error) {
	if err := os.Remove(path); err != nil {
		return protocol.FSDeleteResult{Deleted: false, Path: path}, err
	}
	return protocol.FSDeleteResult{Deleted: true, Path: path}, nil
}

func (p *windowsPlatform) MoveFile(from string, to string) (protocol.FSMoveResult, error) {
	if err := os.Rename(from, to); err != nil {
		return protocol.FSMoveResult{Moved: false, From: from, To: to}, err
	}
	return protocol.FSMoveResult{Moved: true, From: from, To: to}, nil
}

func (p *windowsPlatform) Mkdir(path string) (protocol.FSMkdirResult, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return protocol.FSMkdirResult{Created: false, Path: path}, err
	}
	return protocol.FSMkdirResult{Created: true, Path: path}, nil
}

// Shell — Windows cmd.exe
func (p *windowsPlatform) Exec(command string, timeout int, workDir string, env map[string]string) (protocol.ShellResult, error) {
	cmd := exec.Command("cmd", "/c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	start := time.Now()
	done := make(chan error, 1)
	var output []byte
	go func() {
		out, err := cmd.CombinedOutput()
		output = out
		done <- err
	}()

	timedOut := false
	var execErr error
	if timeout > 0 {
		timer := time.NewTimer(time.Duration(timeout) * time.Second)
		defer timer.Stop()
		select {
		case execErr = <-done:
		case <-timer.C:
			timedOut = true
			cmd.Process.Kill()
			<-done
		}
	} else {
		execErr = <-done
	}

	exitCode := 0
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	_ = start
	return protocol.ShellResult{
		Stdout:   string(output),
		Stderr:   "",
		ExitCode: exitCode,
		TimedOut: timedOut,
	}, nil
}

// Screen — stub (Windows screen capture needs .NET or GDI)
func (p *windowsPlatform) Screenshot(display int, quality int) (protocol.ScreenshotResult, error) {
	return protocol.ScreenshotResult{
		Format:    "png",
		Width:     0,
		Height:    0,
		Data:      "",
		SizeBytes: 0,
	}, fmt.Errorf("screenshot not implemented on windows — use PowerShell or .NET screen capture")
}

func (p *windowsPlatform) ScreenInfo() protocol.ScreenInfo {
	return protocol.ScreenInfo{Displays: []protocol.DisplayInfo{{ID: 0, Width: 0, Height: 0, Scale: 1.0, IsPrimary: true}}}
}

func (p *windowsPlatform) ScreenStreamStart(display int, fps int, quality int) (protocol.ScreenStreamStartResult, error) {
	return protocol.ScreenStreamStartResult{}, fmt.Errorf("streaming not implemented")
}

func (p *windowsPlatform) ScreenStreamStop(streamID string) error {
	return fmt.Errorf("streaming not implemented")
}

// Input — stub (needs user32.dll SendInput or PowerShell)
func (p *windowsPlatform) Click(x int, y int, button string) error {
	return fmt.Errorf("input not implemented on windows")
}
func (p *windowsPlatform) TypeText(text string) error {
	return fmt.Errorf("input not implemented on windows")
}
func (p *windowsPlatform) KeyPress(key string) error {
	return fmt.Errorf("input not implemented on windows")
}
func (p *windowsPlatform) Hotkey(keys []string) error {
	return fmt.Errorf("input not implemented on windows")
}

// System
func (p *windowsPlatform) Health(mode string) protocol.HealthResult {
	hostname, _ := os.Hostname()
	return protocol.HealthResult{
		AgentVersion: "0.1.0",
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Mode:         mode,
	}
}

func (p *windowsPlatform) ProcessList() ([]protocol.ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/fo", "csv", "/nh")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Parse CSV: "name.exe","pid","session","mem"
	lines := splitLines(string(out))
	result := make([]protocol.ProcessInfo, 0, len(lines))
	for _, line := range lines {
		var name string
		var pid int
		var mem int
		fmt.Sscanf(line, "\"%[^\"]\",\"%d\",\"%*[^\"]\",\"%d", &name, &pid, &mem)
		result = append(result, protocol.ProcessInfo{
			PID:        pid,
			Name:       name,
			CPUPercent: 0,
			MemoryMB:   float64(mem) / 1024,
		})
	}
	return result, nil
}

func (p *windowsPlatform) ProcessKill(pid int, signal int) error {
	if signal == 0 {
		signal = 9
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func (p *windowsPlatform) OpenURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func (p *windowsPlatform) Notify(title string, body string, icon string) error {
	return nil // stub — no native notify-send on Windows
}

func (p *windowsPlatform) ClipboardGet() (string, error) {
	cmd := exec.Command("powershell", "-command", "Get-Clipboard")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *windowsPlatform) ClipboardSet(text string) error {
	cmd := exec.Command("powershell", "-command", "Set-Clipboard", "-Value", text)
	return cmd.Run()
}

func splitLines(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == '\n' || ch == '\r' {
			if len(current) > 0 {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if len(current) > 0 {
		result = append(result, current)
	}
	return result
}
