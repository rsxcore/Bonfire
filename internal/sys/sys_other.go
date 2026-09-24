//go:build !windows

package sys

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Documents() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

func AppData() string {
	dir, _ := os.UserConfigDir()
	return dir
}

func RunningProcesses() (map[string]bool, error) { return map[string]bool{}, nil }

func Open(dir string) error { return exec.Command("xdg-open", dir).Start() }
