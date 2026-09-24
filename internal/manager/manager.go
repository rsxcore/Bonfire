// Package manager implements the save-management workflows on top of the store:
// finding save folders, quick save/load, and restores that are always undoable.
package manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rsxcore/Bonfire/internal/config"
	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/store"
	"github.com/rsxcore/Bonfire/internal/sys"
)

var (
	ErrNoQuickSave = errors.New("no quick save yet")
	ErrNoSave      = errors.New("save folder not found")
)

// Manager is safe for concurrent use: the UI runs operations in the background
// while it keeps polling game status.
type Manager struct {
	mu    sync.RWMutex
	cfg   *config.Config
	store *store.Store
	env   games.Env
}

func New(cfg *config.Config, env games.Env) *Manager {
	return &Manager{cfg: cfg, store: store.New(cfg.BackupDir), env: env}
}

type Location struct {
	Dir    string
	Found  bool
	Custom bool // set by the user rather than detected
}

type Status struct {
	Location
	Files    int
	Size     int64
	Modified time.Time
	Running  bool
}

type RestoreResult struct {
	// Safety is the automatic backup of the save that was just replaced.
	Safety *store.Backup
	// CoveredBy is set instead of Safety when the replaced save was already identical to this backup.
	CoveredBy *store.Backup
}

func (m *Manager) st() *store.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

func (m *Manager) BackupDir() string { return m.st().Root() }

func (m *Manager) GameBackupDir(g games.Game) string { return m.st().GameDir(g.ID) }

func (m *Manager) Settings() (backupDir string, autoKeep int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.BackupDir, m.cfg.AutoKeep
}

func (m *Manager) SetSettings(backupDir string, autoKeep int) error {
	backupDir = cleanInput(backupDir)
	if !filepath.IsAbs(backupDir) {
		return fmt.Errorf("backup folder must be a full path, like C:\\Backups")
	}
	if autoKeep < 1 {
		return fmt.Errorf("keep at least one automatic backup")
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("can't use backup folder: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.BackupDir, m.cfg.AutoKeep = backupDir, autoKeep
	m.store = store.New(backupDir)
	return m.cfg.Save()
}

func (m *Manager) LastGame() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.LastGame
}

func (m *Manager) SetLastGame(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg.LastGame == id {
		return nil
	}
	m.cfg.LastGame = id
	return m.cfg.Save()
}

func (m *Manager) Locate(g games.Game) Location {
	m.mu.RLock()
	custom := m.cfg.SavePaths[g.ID]
	m.mu.RUnlock()

	if custom != "" {
		return Location{Dir: custom, Found: isDir(custom), Custom: true}
	}
	cands := g.Candidates(m.env)
	for _, d := range cands {
		if isDir(d) {
			return Location{Dir: d, Found: true}
		}
	}
	return Location{Dir: cands[0]}
}

// SetSavePath overrides a game's save folder. An empty input goes back to auto-detection.
// For emulated games the input may be the emulator folder; the save is found inside it.
func (m *Manager) SetSavePath(g games.Game, input string) (string, error) {
	input = cleanInput(input)
	dir := ""
	if input != "" {
		var ok bool
		if dir, ok = g.FindSaveDir(input); !ok {
			if g.Emulated() {
				return "", fmt.Errorf("no %s save found in %s", g.Short, input)
			}
			return "", fmt.Errorf("folder %s doesn't exist", input)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if dir == "" {
		delete(m.cfg.SavePaths, g.ID)
	} else {
		m.cfg.SavePaths[g.ID] = dir
	}
	return dir, m.cfg.Save()
}

// Statuses reports every game's save folder and whether the game is running.
func (m *Manager) Statuses() map[string]Status {
	procs, _ := sys.RunningProcesses()
	out := make(map[string]Status, len(games.All()))
	for _, g := range games.All() {
		s := Status{Location: m.Locate(g)}
		for _, p := range g.Processes {
			s.Running = s.Running || procs[p]
		}
		if s.Found {
			files, _ := store.Scan(s.Dir, false)
			for _, f := range files {
				s.Files++
				s.Size += f.Size
				if f.ModTime.After(s.Modified) {
					s.Modified = f.ModTime
				}
			}
		}
		out[g.ID] = s
	}
	return out
}

func (m *Manager) Backups(g games.Game) ([]*store.Backup, error) { return m.st().List(g.ID) }

// Backup archives the current save under a name.
func (m *Manager) Backup(g games.Game, name string, kind store.Kind) (*store.Backup, error) {
	loc := m.Locate(g)
	if !loc.Found {
		return nil, fmt.Errorf("%w: %s", ErrNoSave, loc.Dir)
	}
	return m.st().Create(g.ID, loc.Dir, name, kind)
}

// QuickSave replaces the game's quick-save slot with the current save.
func (m *Manager) QuickSave(g games.Game) (*store.Backup, error) {
	b, err := m.Backup(g, "Quick save", store.Quick)
	if err != nil {
		return nil, err
	}
	// The new slot is written before the old one is dropped, so there is always a quick save.
	return b, m.st().Prune(g.ID, store.Quick, 1)
}

func (m *Manager) QuickLoad(g games.Game) (*store.Backup, RestoreResult, error) {
	list, err := m.Backups(g)
	if err != nil {
		return nil, RestoreResult{}, err
	}
	for _, b := range list {
		if b.Kind == store.Quick && b.Err == nil {
			res, err := m.Restore(g, b)
			return b, res, err
		}
	}
	return nil, RestoreResult{}, ErrNoQuickSave
}

// Restore puts a backup back in place. Unless the current save is already
// identical to some backup, it is first saved as an automatic backup, so a
// restore can always be undone.
func (m *Manager) Restore(g games.Game, b *store.Backup) (RestoreResult, error) {
	var res RestoreResult
	if b.Err != nil {
		return res, fmt.Errorf("backup is damaged: %w", b.Err)
	}
	st := m.st()
	loc := m.Locate(g)

	if loc.Found {
		current, err := store.Scan(loc.Dir, true)
		if err != nil {
			return res, fmt.Errorf("can't read the current save: %w", err)
		}
		if len(current) > 0 {
			list, err := st.List(g.ID)
			if err != nil {
				return res, err
			}
			for _, x := range list {
				if x.Err == nil && x.Matches(current) {
					res.CoveredBy = x
					break
				}
			}
			if res.CoveredBy == nil {
				safety, err := st.Create(g.ID, loc.Dir, fmt.Sprintf("Before loading %q", b.Name), store.Auto)
				if err != nil {
					return res, fmt.Errorf("safety backup failed, nothing was changed: %w", err)
				}
				res.Safety = safety
			}
		}
	}

	if err := st.Restore(b, loc.Dir); err != nil {
		return res, err
	}
	_, keep := m.Settings()
	return res, st.Prune(g.ID, store.Auto, keep)
}

func (m *Manager) Rename(b *store.Backup, name string) (*store.Backup, error) {
	return m.st().Rename(b, name)
}

func (m *Manager) Delete(b *store.Backup) error { return m.st().Delete(b) }

func (m *Manager) OpenSaveFolder(g games.Game) error {
	loc := m.Locate(g)
	if !loc.Found {
		return fmt.Errorf("%w: %s", ErrNoSave, loc.Dir)
	}
	return sys.Open(loc.Dir)
}

func (m *Manager) OpenBackupFolder(g games.Game) error {
	dir := m.GameBackupDir(g)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return sys.Open(dir)
}

// cleanInput strips whitespace and the quotes Explorer's "Copy as path" adds.
func cleanInput(s string) string { return strings.Trim(strings.TrimSpace(s), `"'`) }

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
