// Package sys wraps the few OS services the app needs.
package sys

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Documents returns the Documents folder, following OneDrive or manual redirection.
func Documents() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents")
}

// AppData returns the roaming AppData folder.
func AppData() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0); err == nil {
		return p
	}
	return os.Getenv("APPDATA")
}

// RunningProcesses returns the lower-case executable names of all running processes.
func RunningProcesses() (map[string]bool, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	out := make(map[string]bool, 256)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out[strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))] = true
	}
	return out, nil
}

// Open shows a folder in Explorer.
func Open(dir string) error {
	// Explorer exits with code 1 even on success, so don't wait for it.
	return exec.Command("explorer.exe", dir).Start()
}
