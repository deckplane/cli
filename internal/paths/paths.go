// Package paths decides where installations live on each operating system.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// CoreDir is the default install directory for Deckplane Core.
// Override with --dir or DECKPLANE_CORE_DIR.
func CoreDir() string { return defaultDir("DECKPLANE_CORE_DIR", "/var/lib/deckplane", "core") }

// AgentDir is the default install directory for the Deckplane agent.
// Override with --dir or DECKPLANE_AGENT_DIR.
func AgentDir() string {
	return defaultDir("DECKPLANE_AGENT_DIR", "/var/lib/deckplane-agent", "agent")
}

// Servers use a system directory; desktops (Docker Desktop on macOS and
// Windows) use the user's home so no administrator rights are needed.
func defaultDir(env, linuxDir, homeSub string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	if runtime.GOOS == "linux" {
		return linuxDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".deckplane", homeSub)
	}
	return filepath.Join(home, ".deckplane", homeSub)
}

// EnsureWritable creates dir (owner-only) and reports a friendly error when
// the current user may not write there.
func EnsureWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	probe, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return err
	}
	probe.Close()
	return os.Remove(probe.Name())
}

// IsRoot reports whether the process runs as root (always false on Windows).
func IsRoot() bool { return runtime.GOOS != "windows" && os.Geteuid() == 0 }
