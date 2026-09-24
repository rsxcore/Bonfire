// Bonfire backs up and restores FromSoftware game saves.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rsxcore/Bonfire/internal/config"
	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/manager"
	"github.com/rsxcore/Bonfire/internal/sys"
	"github.com/rsxcore/Bonfire/internal/ui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "2.0.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		// When started by double-click the console closes immediately; keep the message readable.
		fmt.Fprint(os.Stderr, "\nPress Enter to exit…")
		bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

func run() error {
	env := games.Env{AppData: sys.AppData(), Documents: sys.Documents()}
	cfg, err := config.Load(
		filepath.Join(env.AppData, "Bonfire", "config.json"),
		filepath.Join(env.Documents, "Bonfire"),
	)
	if err != nil {
		return err
	}
	mgr := manager.New(cfg, env)

	final, err := tea.NewProgram(ui.New(mgr, version), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if m, ok := final.(ui.Model); ok {
		return mgr.SetLastGame(m.SelectedGame().ID)
	}
	return nil
}
