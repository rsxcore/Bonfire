package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/rsxcore/Bonfire/internal/config"
	"github.com/rsxcore/Bonfire/internal/games"
	"github.com/rsxcore/Bonfire/internal/manager"
	"github.com/rsxcore/Bonfire/internal/store"
)

// TestScreenshots renders demo screens to SVG for the README:
//
//	BONFIRE_SCREENSHOTS=../../assets go test ./internal/ui -run Screenshots
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("BONFIRE_SCREENSHOTS")
	if dir == "" {
		t.Skip("set BONFIRE_SCREENSHOTS to the output folder")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	const w, h = 118, 30
	m := demoModel(t, w, h)
	write := func(name string, m Model) {
		svg := ansiToSVG(m.View(), w, h)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(svg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("screenshot.svg", m)
	write("screenshot-restore.svg", press(m, "enter"))
}

// demoModel builds a model with made-up but realistic saves, without touching the disk.
func demoModel(t *testing.T, w, h int) Model {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"), `C:\Users\you\Documents\Bonfire`)
	if err != nil {
		t.Fatal(err)
	}
	cfg.LastGame = "er"
	m := New(manager.New(cfg, games.Env{}), "2.0.0")

	now := time.Now()
	status := map[string]manager.Status{}
	found := map[string]int{"dsr": 2, "ds2": 2, "bb": 22, "ds3": 3, "sekiro": 7, "er": 33}
	for _, g := range games.All() {
		n, ok := found[g.ID]
		status[g.ID] = manager.Status{
			Location: manager.Location{Dir: `C:\Users\you\AppData\Roaming\` + g.ID, Found: ok},
			Files:    n, Size: int64(n) * 900_000, Modified: now.Add(-4 * time.Minute),
		}
	}
	status["er"] = manager.Status{
		Location: manager.Location{Dir: `C:\Users\you\AppData\Roaming\EldenRing`, Found: true},
		Files:    33, Size: 29_800_000, Modified: now.Add(-3 * time.Minute), Running: true,
	}

	backup := func(name string, kind store.Kind, ago time.Duration, zip int64) *store.Backup {
		return &store.Backup{
			Manifest: store.Manifest{Name: name, Kind: kind, Created: now.Add(-ago),
				Files: make([]store.File, 33)},
			ZipSize: zip,
		}
	}
	er := []*store.Backup{
		backup("Quick save", store.Quick, 2*time.Minute, 4_310_000),
		backup("Before Malenia", store.Manual, 47*time.Minute, 4_280_000),
		backup(`Before loading "Quick save"`, store.Auto, 52*time.Minute, 4_270_000),
		backup("Mohg, second phase", store.Manual, 3*time.Hour, 4_190_000),
		backup("Radahn festival", store.Manual, 26*time.Hour, 3_920_000),
		backup("Leyndell arrival", store.Manual, 4*24*time.Hour, 3_610_000),
		backup("Fresh character", store.Manual, 12*24*time.Hour, 1_050_000),
	}
	for _, b := range er {
		b.Files[0].Size = 29_800_000
	}

	step := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	step(tea.WindowSizeMsg{Width: w, Height: h})
	step(statusMsg(status))
	step(backupsMsg{game: "er", list: er})
	step(backupsMsg{game: "ds3", list: er[:3]})
	step(backupsMsg{game: "sekiro", list: er[:2]})
	step(backupsMsg{game: "bb", list: er[:4]})
	m = press(m, "enter", "down", "down")
	m.notify(toastOK, "Quick save created · 28.4 MB")
	return m
}

var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

type cellStyle struct {
	fg, bg       string
	bold, italic bool
}

// ansiToSVG draws a terminal frame as an SVG "window". Every glyph gets an
// explicit x position so box drawing lines up regardless of the viewer's font.
func ansiToSVG(frame string, cols, rows int) string {
	const (
		cw, ch   = 8.4, 18.0
		padX     = 18.0
		padTop   = 44.0
		padBot   = 16.0
		fontSize = 14
		defFg    = "#E9E4DA"
		termBg   = "#12100E"
	)
	width := padX*2 + cw*float64(cols)
	height := padTop + padBot + ch*float64(rows)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">`, width, height, width, height)
	b.WriteString(`<style>text{font-family:"Cascadia Mono","Cascadia Code",Consolas,"SF Mono",Menlo,"DejaVu Sans Mono",monospace;font-size:` + strconv.Itoa(fontSize) + `px;white-space:pre}</style>`)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" rx="10" fill="%s"/>`, termBg)
	fmt.Fprintf(&b, `<rect width="100%%" height="32" rx="10" fill="#1E1A17"/><rect y="22" width="100%%" height="10" fill="#1E1A17"/>`)
	for i, c := range []string{"#FF5F57", "#FEBC2E", "#28C840"} {
		fmt.Fprintf(&b, `<circle cx="%d" cy="16" r="6" fill="%s"/>`, 20+i*20, c)
	}
	fmt.Fprintf(&b, `<text x="%.0f" y="21" fill="#9A9388" text-anchor="middle">Bonfire</text>`, width/2)

	for row, line := range strings.Split(frame, "\n") {
		if row >= rows {
			break
		}
		y := padTop + float64(row)*ch
		st := cellStyle{}
		col := 0
		var run []rune
		runStart := 0
		flush := func() {
			if len(run) == 0 {
				return
			}
			x0 := padX + float64(runStart)*cw
			if st.bg != "" {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, x0, y, cw*float64(len(run)), ch, st.bg)
			}
			// Terminals draw vertical box lines edge to edge; font glyphs leave gaps, so draw them as rects.
			for i, r := range run {
				if r == '│' {
					fg := st.fg
					if fg == "" {
						fg = defFg
					}
					fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="1.2" height="%.1f" fill="%s"/>`, x0+float64(i)*cw+cw/2-0.6, y, ch, fg)
					run[i] = ' '
				}
			}
			if strings.TrimSpace(string(run)) != "" {
				xs := make([]string, len(run))
				for i := range run {
					xs[i] = strconv.FormatFloat(x0+float64(i)*cw, 'f', 1, 64)
				}
				fg := st.fg
				if fg == "" {
					fg = defFg
				}
				attrs := ""
				if st.bold {
					attrs += ` font-weight="bold"`
				}
				if st.italic {
					attrs += ` font-style="italic"`
				}
				fmt.Fprintf(&b, `<text x="%s" y="%.1f" fill="%s"%s>%s</text>`, strings.Join(xs, " "), y+ch*0.72, fg, attrs, xmlEscape(string(run)))
			}
			run = run[:0]
		}

		for len(line) > 0 {
			if loc := sgr.FindStringSubmatchIndex(line); loc != nil && loc[0] == 0 {
				flush()
				st = applySGR(st, line[loc[2]:loc[3]])
				line = line[loc[1]:]
				runStart = col
				continue
			}
			next := len(line)
			if loc := sgr.FindStringIndex(line); loc != nil {
				next = loc[0]
			}
			for _, r := range line[:next] {
				if len(run) == 0 {
					runStart = col
				}
				run = append(run, r)
				col++
			}
			line = line[next:]
		}
		flush()
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func applySGR(st cellStyle, params string) cellStyle {
	if params == "" {
		return cellStyle{}
	}
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case "0":
			st = cellStyle{}
		case "1":
			st.bold = true
		case "3":
			st.italic = true
		case "22":
			st.bold = false
		case "23":
			st.italic = false
		case "39":
			st.fg = ""
		case "49":
			st.bg = ""
		case "38", "48":
			if i+4 < len(p) && p[i+1] == "2" {
				r, _ := strconv.Atoi(p[i+2])
				g, _ := strconv.Atoi(p[i+3])
				bl, _ := strconv.Atoi(p[i+4])
				c := fmt.Sprintf("#%02X%02X%02X", r, g, bl)
				if p[i] == "38" {
					st.fg = c
				} else {
					st.bg = c
				}
				i += 4
			}
		}
	}
	return st
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
