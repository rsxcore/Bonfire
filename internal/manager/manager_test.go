package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxcore/Bonfire/internal/config"
	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/store"
)

func setup(t *testing.T) (*Manager, games.Game, string) {
	t.Helper()
	root := t.TempDir()
	cfg, err := config.Load(filepath.Join(root, "config.json"), filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	env := games.Env{AppData: filepath.Join(root, "AppData"), Documents: filepath.Join(root, "Docs")}
	g, _ := games.ByID("er")
	save := filepath.Join(env.AppData, "EldenRing")
	os.MkdirAll(save, 0o755)
	write(t, save, "state-1")
	return New(cfg, env), g, save
}

func write(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ER0000.sl2"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "ER0000.sl2"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func count(t *testing.T, m *Manager, g games.Game, kind store.Kind) int {
	t.Helper()
	list, err := m.Backups(g)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range list {
		if b.Kind == kind {
			n++
		}
	}
	return n
}

func TestQuickSaveKeepsOneSlot(t *testing.T) {
	m, g, save := setup(t)
	for i := 0; i < 3; i++ {
		if _, err := m.QuickSave(g); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, m, g, store.Quick); n != 1 {
		t.Fatalf("%d quick saves, want 1", n)
	}
	_ = save
}

func TestQuickLoadIsUndoable(t *testing.T) {
	m, g, save := setup(t)
	if _, _, err := m.QuickLoad(g); err != ErrNoQuickSave {
		t.Fatalf("got %v, want ErrNoQuickSave", err)
	}

	if _, err := m.QuickSave(g); err != nil {
		t.Fatal(err)
	}
	write(t, save, "state-2 (died)")

	_, res, err := m.QuickLoad(g)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, save); got != "state-1" {
		t.Fatalf("save = %q after quick load", got)
	}
	if res.Safety == nil {
		t.Fatal("changed save should have been backed up before loading")
	}

	// Loading again: the current save equals the quick save, so no extra safety copy.
	_, res, err = m.QuickLoad(g)
	if err != nil {
		t.Fatal(err)
	}
	if res.Safety != nil || res.CoveredBy == nil {
		t.Fatalf("identical save should not get another safety backup: %+v", res)
	}
	if n := count(t, m, g, store.Auto); n != 1 {
		t.Fatalf("%d auto backups, want 1", n)
	}

	// Undo: restoring the safety backup brings the overwritten state back.
	if _, err := m.Restore(g, mustFind(t, m, g, store.Auto)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, save); got != "state-2 (died)" {
		t.Fatalf("undo gave %q", got)
	}
}

func TestAutoBackupsArePruned(t *testing.T) {
	m, g, save := setup(t)
	dir, _ := m.Settings()
	if err := m.SetSettings(dir, 2); err != nil {
		t.Fatal(err)
	}
	b, err := m.Backup(g, "base", store.Manual)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		write(t, save, "change "+string(rune('a'+i)))
		if _, err := m.Restore(g, b); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, m, g, store.Auto); n != 2 {
		t.Fatalf("%d auto backups, want 2", n)
	}
}

func TestMissingSave(t *testing.T) {
	m, g, save := setup(t)
	os.RemoveAll(save)
	if _, err := m.QuickSave(g); err == nil {
		t.Fatal("expected error when save folder is missing")
	}
}

func TestSetSavePathEmulated(t *testing.T) {
	m, _, _ := setup(t)
	bb, _ := games.ByID("bb")
	emu := t.TempDir()
	want := filepath.Join(emu, "user", "savedata", "1", "CUSA00207")
	os.MkdirAll(want, 0o755)

	got, err := m.SetSavePath(bb, `"`+emu+`"`)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("found %s, want %s", got, want)
	}
	if loc := m.Locate(bb); !loc.Found || !loc.Custom || loc.Dir != want {
		t.Fatalf("locate after override: %+v", loc)
	}
	if _, err := m.SetSavePath(bb, t.TempDir()); err == nil {
		t.Fatal("folder without a Bloodborne save must be rejected")
	}
	if _, err := m.SetSavePath(bb, ""); err != nil {
		t.Fatal(err)
	}
	if loc := m.Locate(bb); loc.Custom {
		t.Fatal("empty input should reset to auto-detect")
	}
}

func mustFind(t *testing.T, m *Manager, g games.Game, kind store.Kind) *store.Backup {
	t.Helper()
	list, _ := m.Backups(g)
	for _, b := range list {
		if b.Kind == kind {
			return b
		}
	}
	t.Fatalf("no %s backup", kind)
	return nil
}
