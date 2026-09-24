package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rsxcore/Bonfire/internal/config"
	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/manager"
	"github.com/rsxcore/Bonfire/internal/store"
)

// fixture builds a model over a fake AppData with an Elden Ring save and a few backups.
func fixture(t *testing.T, w, h int) Model {
	t.Helper()
	root := t.TempDir()
	env := games.Env{AppData: filepath.Join(root, "AppData"), Documents: filepath.Join(root, "Docs")}
	save := filepath.Join(env.AppData, "EldenRing", "76561198000000000")
	os.MkdirAll(save, 0o755)
	os.WriteFile(filepath.Join(save, "ER0000.sl2"), []byte(strings.Repeat("x", 30000)), 0o644)

	cfg, err := config.Load(filepath.Join(root, "config.json"), filepath.Join(root, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.LastGame = "er"
	mgr := manager.New(cfg, env)
	er, _ := games.ByID("er")
	for _, name := range []string{"Before Malenia", "Mohg, Lord of Blood — first try with a very long name that must be cut"} {
		if _, err := mgr.Backup(er, name, store.Manual); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := mgr.QuickSave(er); err != nil {
		t.Fatal(err)
	}

	m := New(mgr, "2.0.0")
	step := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	step(tea.WindowSizeMsg{Width: w, Height: h})
	step(statusMsg(mgr.Statuses()))
	for _, g := range games.All() {
		list, err := mgr.Backups(g)
		step(backupsMsg{game: g.ID, list: list, err: err})
	}
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(Model)
	}
	return m
}

func checkFrame(t *testing.T, frame string, w, h int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	if len(lines) != h {
		t.Errorf("frame has %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("line %d is %d cells wide, max %d: %q", i, lw, w, ansi.Strip(l))
		}
	}
}

func TestLayoutFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{76, 18}, {100, 30}, {160, 45}} {
		w, h := size[0], size[1]
		m := fixture(t, w, h)
		checkFrame(t, m.View(), w, h)

		m = press(m, "tab")
		checkFrame(t, m.View(), w, h)
		for _, k := range []string{"?", "n", ","} {
			d := press(m, k)
			checkFrame(t, d.View(), w, h)
		}
		d := press(m, "enter")
		checkFrame(t, d.View(), w, h)
	}
}

func TestTooSmall(t *testing.T) {
	m := fixture(t, 60, 12)
	if !strings.Contains(ansi.Strip(m.View()), "bigger") {
		t.Fatal("expected a too-small notice")
	}
}

// TestPrintFrames dumps rendered screens; run with -v to eyeball the layout.
func TestPrintFrames(t *testing.T) {
	if os.Getenv("SSM_PRINT") == "" {
		t.Skip("set SSM_PRINT=1 to print frames")
	}
	m := fixture(t, 110, 28)
	t.Log("\n" + ansi.Strip(m.View()))
	m = press(m, "tab", "down")
	t.Log("\n" + ansi.Strip(m.View()))
	t.Log("\n" + ansi.Strip(press(m, "enter").View()))
	t.Log("\n" + ansi.Strip(press(m, "n").View()))
	t.Log("\n" + ansi.Strip(press(m, ",").View()))
	t.Log("\n" + ansi.Strip(press(m, "?").View()))
}

func TestEnterFlow(t *testing.T) {
	m := fixture(t, 110, 28)

	// Enter on a game lands on "+ New backup"; Enter again opens the name dialog.
	m = press(m, "enter")
	if m.focus != paneBackups || !m.onNewRow() {
		t.Fatalf("enter on game: focus=%v row=%d, want backups pane on the new-backup row", m.focus, m.bi)
	}
	if d := press(m, "enter"); d.dialog == nil {
		t.Fatal("enter on New backup should open the name dialog")
	} else if _, ok := d.dialog.(*inputDialog); !ok {
		t.Fatalf("got %T, want the name dialog", d.dialog)
	}

	// Enter on an existing backup asks to restore it.
	m = press(m, "down")
	if m.selected() == nil {
		t.Fatal("down from New backup should select the first backup")
	}
	d := press(m, "enter")
	if c, ok := d.dialog.(*confirmDialog); !ok || c.yes != "Restore" {
		t.Fatalf("enter on a backup: got %T, want restore confirmation", d.dialog)
	}
}
