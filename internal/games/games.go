// Package games describes the supported games and where they keep their saves.
package games

import (
	"os"
	"path/filepath"
	"strings"
)

// Env holds the system folders save locations are built from.
type Env struct {
	AppData   string // %APPDATA% (Roaming)
	Documents string // the user's Documents folder, wherever it is redirected to
}

type Game struct {
	ID    string
	Name  string // full title, used in headers
	Short string // compact title, used in the game list
	// Accent is the hex colour the UI uses for this game.
	Accent string
	// Processes are lower-case executable names that mean the game is running.
	Processes []string
	// TitleIDs are PS4 title IDs for emulated games; any of them may hold the save.
	TitleIDs []string

	candidates func(Env) []string
}

// Candidates returns possible save folders in priority order.
// The first one is the expected location when none of them exist yet.
func (g Game) Candidates(env Env) []string { return g.candidates(env) }

// Emulated reports whether the save lives inside an emulator folder the user has to point to.
func (g Game) Emulated() bool { return len(g.TitleIDs) > 0 }

// FindSaveDir turns a folder the user typed into the actual save folder.
// For emulated games the user may point at the emulator itself; the save is searched inside it.
func (g Game) FindSaveDir(dir string) (string, bool) {
	dir = filepath.Clean(dir)
	if !g.Emulated() {
		return dir, isDir(dir)
	}

	base := strings.ToUpper(filepath.Base(dir))
	for _, id := range g.TitleIDs {
		if base == id {
			return dir, isDir(dir)
		}
	}
	for _, pattern := range []string{"user/savedata/*", "savedata/*", "*"} {
		for _, id := range g.TitleIDs {
			matches, _ := filepath.Glob(filepath.Join(dir, pattern, id))
			for _, m := range matches {
				if isDir(m) {
					return m, true
				}
			}
		}
	}
	return dir, false
}

func All() []Game { return list }

func ByID(id string) (Game, bool) {
	for _, g := range list {
		if g.ID == id {
			return g, true
		}
	}
	return Game{}, false
}

// Bloodborne regions: GOTY US, US, EU, GOTY EU, Asia.
var bloodborneIDs = []string{"CUSA03173", "CUSA00900", "CUSA00207", "CUSA03023", "CUSA01363"}

var list = []Game{
	{
		ID: "ds1", Name: "Dark Souls: Prepare to Die Edition", Short: "Dark Souls PtDE",
		Accent: "#C9A26B", Processes: []string{"darksouls.exe"},
		candidates: documents("NBGI", "DarkSouls"),
	},
	{
		ID: "dsr", Name: "Dark Souls Remastered", Short: "Dark Souls Remastered",
		Accent: "#E07A3F", Processes: []string{"darksoulsremastered.exe"},
		candidates: documents("NBGI", "DARK SOULS REMASTERED"),
	},
	{
		ID: "ds2", Name: "Dark Souls II", Short: "Dark Souls II",
		Accent: "#6FA8DC", Processes: []string{"darksoulsii.exe"},
		candidates: appData("DarkSoulsII"),
	},
	{
		ID: "bb", Name: "Bloodborne (shadPS4)", Short: "Bloodborne",
		Accent: "#C0395B", Processes: []string{"shadps4.exe"},
		TitleIDs:   bloodborneIDs,
		candidates: shadPS4,
	},
	{
		ID: "ds3", Name: "Dark Souls III", Short: "Dark Souls III",
		Accent: "#E8553A", Processes: []string{"darksoulsiii.exe"},
		candidates: appData("DarkSoulsIII"),
	},
	{
		ID: "sekiro", Name: "Sekiro: Shadows Die Twice", Short: "Sekiro",
		Accent: "#D94B4B", Processes: []string{"sekiro.exe"},
		candidates: appData("Sekiro"),
	},
	{
		ID: "er", Name: "Elden Ring", Short: "Elden Ring",
		Accent: "#E6C46A", Processes: []string{"eldenring.exe"},
		candidates: appData("EldenRing"),
	},
	{
		ID: "ac6", Name: "Armored Core VI", Short: "Armored Core VI",
		Accent: "#5FB3A1", Processes: []string{"armoredcore6.exe"},
		candidates: appData("ArmoredCore6"),
	},
	{
		ID: "nr", Name: "Elden Ring Nightreign", Short: "Nightreign",
		Accent: "#9A86D6", Processes: []string{"nightreign.exe"},
		candidates: appData("Nightreign"),
	},
}

func documents(parts ...string) func(Env) []string {
	return func(e Env) []string { return []string{filepath.Join(append([]string{e.Documents}, parts...)...)} }
}

func appData(parts ...string) func(Env) []string {
	return func(e Env) []string { return []string{filepath.Join(append([]string{e.AppData}, parts...)...)} }
}

// shadPS4 looks in the non-portable shadPS4 user folder. Portable installs keep
// saves next to the emulator, which only the user knows, so they set it with a custom path.
func shadPS4(e Env) []string {
	var out []string
	for _, id := range bloodborneIDs {
		matches, _ := filepath.Glob(filepath.Join(e.AppData, "shadPS4", "savedata", "*", id))
		out = append(out, matches...)
	}
	return append(out, filepath.Join(e.AppData, "shadPS4", "savedata", "1", bloodborneIDs[0]))
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
