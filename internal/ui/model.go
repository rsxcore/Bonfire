// Package ui is the terminal interface, built on Bubble Tea.
package ui

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/manager"
	"github.com/rsxcore/Bonfire/internal/store"
)

type pane int

const (
	paneGames pane = iota
	paneBackups
)

type toastKind int

const (
	toastOK toastKind = iota
	toastWarn
	toastErr
	toastInfo
)

type toast struct {
	kind toastKind
	text string
	at   time.Time
}

type Model struct {
	mgr     *manager.Manager
	version string
	games   []games.Game

	status  map[string]manager.Status
	backups map[string][]*store.Backup
	listErr map[string]error

	gi, bi int // selected game and backup
	boff   int // first visible backup row
	focus  pane

	width, height int
	busy          string // label of the running operation, "" when idle
	frame         int
	toast         *toast
	dialog        dialog
}

func New(mgr *manager.Manager, version string) Model {
	m := Model{
		mgr:     mgr,
		version: version,
		games:   games.All(),
		status:  map[string]manager.Status{},
		backups: map[string][]*store.Backup{},
		listErr: map[string]error{},
	}
	for i, g := range m.games {
		if g.ID == mgr.LastGame() {
			m.gi = i
		}
	}
	return m
}

// SelectedGame is the game highlighted when the program exits.
func (m Model) SelectedGame() games.Game { return m.games[m.gi] }

// Messages.
type (
	tickMsg    struct{}
	spinMsg    struct{}
	statusMsg  map[string]manager.Status
	backupsMsg struct {
		game    string
		list    []*store.Backup
		err     error
		select_ string // path of a backup to put the cursor on
	}
	// runOpMsg asks the model to start a background operation. Dialogs use it
	// because only the model knows whether another operation is running.
	runOpMsg struct {
		label string
		run   func() opResult
	}
	opDoneMsg opResult
)

type opResult struct {
	game       games.Game
	kind       toastKind
	text       string
	selectPath string
	reloadAll  bool
}

func runOp(label string, run func() opResult) tea.Cmd {
	return func() tea.Msg { return runOpMsg{label, run} }
}

func failed(g games.Game, what string, err error) opResult {
	return opResult{game: g, kind: toastErr, text: what + ": " + err.Error()}
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func spin() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinMsg{} })
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.SetWindowTitle("Bonfire"), m.refreshStatus(), tick()}
	for _, g := range m.games {
		cmds = append(cmds, m.loadBackups(g, ""))
	}
	return tea.Batch(cmds...)
}

func (m Model) refreshStatus() tea.Cmd {
	mgr := m.mgr
	return func() tea.Msg { return statusMsg(mgr.Statuses()) }
}

func (m Model) loadBackups(g games.Game, selectPath string) tea.Cmd {
	mgr := m.mgr
	return func() tea.Msg {
		list, err := mgr.Backups(g)
		return backupsMsg{game: g.ID, list: list, err: err, select_: selectPath}
	}
}

func (m Model) game() games.Game { return m.games[m.gi] }

// Row 0 of the backup list is the "+ New backup" action; backups start at row 1.
func (m Model) onNewRow() bool { return m.bi == 0 }

func (m Model) selected() *store.Backup {
	list := m.backups[m.game().ID]
	if m.bi >= 1 && m.bi <= len(list) {
		return list[m.bi-1]
	}
	return nil
}

func (m *Model) notify(kind toastKind, text string) {
	m.toast = &toast{kind: kind, text: text, at: time.Now()}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampScroll()
		return m, nil

	case tickMsg:
		if m.toast != nil {
			ttl := 6 * time.Second
			if m.toast.kind == toastErr {
				ttl = 20 * time.Second
			}
			if time.Since(m.toast.at) > ttl {
				m.toast = nil
			}
		}
		return m, tea.Batch(m.refreshStatus(), tick())

	case spinMsg:
		if m.busy == "" {
			return m, nil
		}
		m.frame++
		return m, spin()

	case statusMsg:
		m.status = msg
		return m, nil

	case backupsMsg:
		m.backups[msg.game], m.listErr[msg.game] = msg.list, msg.err
		if msg.game == m.game().ID {
			if msg.select_ != "" {
				for i, b := range msg.list {
					if b.Path == msg.select_ {
						m.bi = i + 1
					}
				}
			}
			m.clampScroll()
		}
		return m, nil

	case runOpMsg:
		return m, m.start(msg.label, msg.run)

	case opDoneMsg:
		m.busy = ""
		m.notify(msg.kind, msg.text)
		if msg.reloadAll {
			m.backups = map[string][]*store.Backup{}
			cmds := []tea.Cmd{m.refreshStatus()}
			for _, g := range m.games {
				cmds = append(cmds, m.loadBackups(g, ""))
			}
			return m, tea.Batch(cmds...)
		}
		if msg.game.ID == "" {
			return m, nil
		}
		return m, tea.Batch(m.loadBackups(msg.game, msg.selectPath), m.refreshStatus())

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.dialog != nil {
			var cmd tea.Cmd
			m.dialog, cmd = m.dialog.update(msg)
			return m, cmd
		}
		return m.handleKey(msg)
	}
	return m, nil
}

// start runs an operation in the background with a spinner. Only one runs at a time.
func (m *Model) start(label string, run func() opResult) tea.Cmd {
	if m.busy != "" {
		m.notify(toastWarn, "Still working on: "+m.busy)
		return nil
	}
	m.busy, m.frame = label, 0
	return tea.Batch(spin(), func() tea.Msg { return opDoneMsg(run()) })
}

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g := m.game()
	switch k.String() {
	case "q":
		if m.busy != "" {
			m.notify(toastWarn, "Wait for the current operation to finish (ctrl+c forces quit)")
			return m, nil
		}
		return m, tea.Quit

	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-m.listRows())
	case "pgdown":
		m.move(m.listRows())
	case "home", "g":
		m.move(-1 << 20)
	case "end", "G":
		m.move(1 << 20)

	case "tab":
		if m.focus == paneGames {
			m.focus = paneBackups
		} else {
			m.focus = paneGames
		}
	case "right":
		m.focus = paneBackups
	case "left", "esc":
		m.focus = paneGames

	case "enter":
		switch {
		case m.focus == paneGames:
			m.focus, m.bi = paneBackups, 0
			m.clampScroll()
		case m.onNewRow():
			m.dialog = m.newBackupDialog(g)
		default:
			m.confirmRestore()
		}

	case "s", "f5":
		return m, m.quickSave(g)
	case "l", "f9":
		return m, m.quickLoad(g)
	case "n":
		m.dialog = m.newBackupDialog(g)
	case "r":
		if b := m.selected(); b != nil && m.focus == paneBackups {
			m.dialog = m.renameDialog(g, b)
		}
	case "d", "delete":
		if b := m.selected(); b != nil && m.focus == paneBackups {
			m.dialog = m.deleteDialog(g, b)
		}
	case "o":
		if err := m.mgr.OpenSaveFolder(g); err != nil {
			m.notify(toastErr, err.Error())
		}
	case "b":
		if err := m.mgr.OpenBackupFolder(g); err != nil {
			m.notify(toastErr, err.Error())
		}
	case "p":
		m.dialog = m.savePathDialog(g)
	case ",":
		m.dialog = m.settingsDialog()
	case "?":
		m.dialog = newHelpDialog()
	}
	return m, nil
}

func (m *Model) move(delta int) {
	if m.focus == paneGames {
		gi := min(max(m.gi+delta, 0), len(m.games)-1)
		if gi != m.gi {
			m.gi, m.bi, m.boff = gi, 0, 0
		}
		return
	}
	n := len(m.backups[m.game().ID]) + 1
	m.bi = min(max(m.bi+delta, 0), n-1)
	m.clampScroll()
}

// listRows is how many backup rows fit in the right pane; view.go draws with the same layout.
func (m Model) listRows() int { return max(m.height-11, 1) }

func (m *Model) clampScroll() {
	n := len(m.backups[m.game().ID]) + 1
	m.bi = min(max(m.bi, 0), n-1)
	rows := m.listRows()
	if m.bi < m.boff {
		m.boff = m.bi
	}
	if m.bi >= m.boff+rows {
		m.boff = m.bi - rows + 1
	}
	m.boff = min(max(m.boff, 0), max(n-rows, 0))
}

// Operations.

func (m Model) quickSave(g games.Game) tea.Cmd {
	mgr := m.mgr
	return runOp("Saving "+g.Short, func() opResult {
		b, err := mgr.QuickSave(g)
		if err != nil {
			return failed(g, "Quick save failed", err)
		}
		return opResult{game: g, kind: toastOK, selectPath: b.Path,
			text: fmt.Sprintf("Quick save created · %s", humanSize(b.TotalSize()))}
	})
}

func (m Model) quickLoad(g games.Game) tea.Cmd {
	mgr := m.mgr
	running := m.status[g.ID].Running
	return runOp("Loading quick save", func() opResult {
		b, res, err := mgr.QuickLoad(g)
		if errors.Is(err, manager.ErrNoQuickSave) {
			return opResult{game: g, kind: toastWarn, text: "No quick save yet: press s to make one"}
		}
		if err != nil {
			return failed(g, "Quick load failed", err)
		}
		return restoredResult(g, b, res, running)
	})
}

func restoredResult(g games.Game, b *store.Backup, res manager.RestoreResult, running bool) opResult {
	r := opResult{game: g, kind: toastOK, text: fmt.Sprintf("Loaded %q from %s", b.Name, relTime(b.Created))}
	if res.Safety != nil {
		r.text += " · previous save kept as AUTO backup"
	}
	if running {
		r.kind = toastWarn
		r.text += " · game is running: reload from the title screen"
	}
	return r
}

func (m *Model) confirmRestore() {
	g, b := m.game(), m.selected()
	if b == nil {
		return
	}
	if b.Err != nil {
		m.notify(toastErr, "Can't restore: "+b.Err.Error())
		return
	}
	st := m.status[g.ID]
	mgr := m.mgr
	body := []string{
		sBold.Render(truncMiddle(b.Name, 40)) + "  " + sDim.Render(relTime(b.Created)+" · "+humanSize(b.TotalSize())),
		sDim.Render("into ") + sText.Render(truncMiddle(st.Dir, 50)),
		"",
		sFaint.Render("Your current save is backed up automatically first."),
	}
	if st.Running {
		body = append(body, "", sWarn.Render("▲ "+g.Short+" is running. Reload from the title screen."))
	}
	m.dialog = newConfirm("↺", "Restore backup", body, "Restore", false, runOp("Restoring "+b.Name, func() opResult {
		res, err := mgr.Restore(g, b)
		if err != nil {
			return failed(g, "Restore failed", err)
		}
		return restoredResult(g, b, res, st.Running)
	}))
}

func (m Model) deleteDialog(g games.Game, b *store.Backup) dialog {
	mgr := m.mgr
	body := []string{
		sBold.Render(truncMiddle(b.Name, 40)) + "  " + sDim.Render(relTime(b.Created)+" · "+humanSize(b.ZipSize)),
		"",
		sFaint.Render("The archive is removed from disk. This can't be undone."),
	}
	return newConfirm("✗", "Delete backup", body, "Delete", true, runOp("Deleting", func() opResult {
		if err := mgr.Delete(b); err != nil {
			return failed(g, "Delete failed", err)
		}
		return opResult{game: g, kind: toastOK, text: fmt.Sprintf("Deleted %q", b.Name)}
	}))
}

func (m Model) newBackupDialog(g games.Game) dialog {
	mgr := m.mgr
	return newInput("◆", "New backup · "+g.Short, "Name it after where you are, e.g. \"Before Malenia\".",
		"", "Backup "+time.Now().Format("02 Jan 15:04"),
		func(name string) (tea.Cmd, string) {
			if name == "" {
				name = "Backup " + time.Now().Format("02 Jan 15:04")
			}
			return runOp("Backing up "+g.Short, func() opResult {
				b, err := mgr.Backup(g, name, store.Manual)
				if err != nil {
					return failed(g, "Backup failed", err)
				}
				return opResult{game: g, kind: toastOK, selectPath: b.Path,
					text: fmt.Sprintf("Saved %q · %s", name, humanSize(b.TotalSize()))}
			}), ""
		})
}

func (m Model) renameDialog(g games.Game, b *store.Backup) dialog {
	mgr := m.mgr
	return newInput("✎", "Rename backup", "", b.Name, "",
		func(name string) (tea.Cmd, string) {
			if name == "" {
				return nil, "Name can't be empty"
			}
			return runOp("Renaming", func() opResult {
				nb, err := mgr.Rename(b, name)
				if err != nil {
					return failed(g, "Rename failed", err)
				}
				return opResult{game: g, kind: toastOK, selectPath: nb.Path, text: fmt.Sprintf("Renamed to %q", name)}
			}), ""
		})
}

func (m Model) savePathDialog(g games.Game) dialog {
	mgr := m.mgr
	loc := m.status[g.ID].Location
	value := ""
	if loc.Custom {
		value = loc.Dir
	}
	hint := "Leave empty to detect it automatically."
	if g.Emulated() {
		hint = "Point to your shadPS4 folder; the save inside it is found automatically. " + hint
	}
	return newInput("⌂", "Save folder · "+g.Short, hint, value, loc.Dir,
		func(input string) (tea.Cmd, string) {
			return runOp("Checking folder", func() opResult {
				dir, err := mgr.SetSavePath(g, input)
				if err != nil {
					return failed(g, "Save folder not set", err)
				}
				if dir == "" {
					return opResult{game: g, kind: toastOK, text: "Save folder is detected automatically again"}
				}
				return opResult{game: g, kind: toastOK, text: "Save folder set to " + dir}
			}), ""
		})
}

func (m Model) settingsDialog() dialog {
	mgr := m.mgr
	dir, keep := mgr.Settings()
	return newForm("⚙", "Settings", []formField{
		{label: "Backup folder", value: dir},
		{label: "Automatic backups kept per game", value: strconv.Itoa(keep)},
	}, func(values []string) (tea.Cmd, string) {
		n, err := strconv.Atoi(values[1])
		if err != nil || n < 1 || n > 1000 {
			return nil, "Automatic backups kept must be a number from 1 to 1000"
		}
		return runOp("Saving settings", func() opResult {
			if err := mgr.SetSettings(values[0], n); err != nil {
				return opResult{kind: toastErr, text: err.Error()}
			}
			return opResult{kind: toastOK, text: "Settings saved", reloadAll: true}
		}), ""
	})
}
