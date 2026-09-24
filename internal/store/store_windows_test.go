package store

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestRestoreWithLockedFileIsAllOrNothing(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "save")
	writeTree(t, save, map[string]string{"a.sl2": "backup-a", "b.sl2": "backup-b"})
	s := New(filepath.Join(root, "backups"))
	b, err := s.Create("er", save, "x", Manual)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, save, map[string]string{"a.sl2": "current-a", "b.sl2": "current-b"})

	// Hold b.sl2 open without delete sharing, like a game writing its save.
	p, _ := syscall.UTF16PtrFromString(filepath.Join(save, "b.sl2"))
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(b, save); err == nil {
		t.Fatal("restore should fail while a save file is locked")
	}
	syscall.CloseHandle(h)

	// Nothing was half-replaced.
	equalTrees(t, readTree(t, save), map[string]string{"a.sl2": "current-a", "b.sl2": "current-b"})

	// Once the lock is gone the restore succeeds.
	if err := s.Restore(b, save); err != nil {
		t.Fatal(err)
	}
	equalTrees(t, readTree(t, save), map[string]string{"a.sl2": "backup-a", "b.sl2": "backup-b"})
}
