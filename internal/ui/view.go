package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rsxcore/Bonfire/internal/store"
)

const (
	minWidth  = 76
	minHeight = 18
)

var spinnerFrames = []string{"◜", "◠", "◝", "◞", "◡", "◟"}

// Screen layout, top to bottom:
//
//	header · blank · [games | backups] panels · status line · key hints
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		msg := sBrand.Render("◈") + " " + sText.Render("Make the window a bit bigger") + "\n" +
			sFaint.Render(fmt.Sprintf("%d×%d now, needs at least %d×%d", m.width, m.height, minWidth, minHeight))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}

	ph := m.height - 4
	lw := min(max(m.width*3/10, 30), 38)
	rw := m.width - lw - 1
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.viewGames(lw, ph), " ", m.viewBackups(rw, ph))
	screen := strings.Join([]string{m.viewHeader(), "", body, m.viewStatus(), m.viewKeys()}, "\n")

	if m.dialog != nil {
		screen = overlay(screen, m.dialog.view(m.width), m.width, m.height)
	}
	return screen
}

func (m Model) viewHeader() string {
	left := " " + sBrand.Bold(true).Render("◈") + " " +
		gradient("BONFIRE", hexBrand, hexGold, true) + "  " + sFaint.Render("v"+m.version)
	const motto = "Rest at the bonfire. Your progress is kept."
	dir := m.mgr.BackupDir()
	right := sFaint.Render("backups ") + sDim.Render(truncMiddle(dir, m.width/2-10)) + " "

	// The motto only shows when it fits beside the backups path without squeezing it.
	withMotto := left + sFaint.Render("  ·  ") + sDim.Italic(true).Render(motto)
	if lipgloss.Width(withMotto)+lipgloss.Width(right)+2 <= m.width {
		left = withMotto
	}
	return joinLR(left, right, m.width)
}

// panel draws a rounded box with the title set into the top border.
func panel(title string, lines []string, w, h int, border lipgloss.TerminalColor) string {
	bs := fg(border)
	iw := w - 4
	title = ansi.Truncate(title, w-6, "…")
	top := bs.Render("╭─ ") + title + bs.Render(" "+strings.Repeat("─", max(w-5-lipgloss.Width(title), 0))+"╮")

	out := make([]string, 0, h)
	out = append(out, top)
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, bs.Render("│")+" "+fit(line, iw)+" "+bs.Render("│"))
	}
	out = append(out, bs.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(out, "\n")
}

func (m Model) viewGames(w, h int) string {
	iw := w - 4
	focused := m.focus == paneGames
	var lines []string

	for i, g := range m.games {
		st, known := m.status[g.ID]
		accent := lipgloss.Color(g.Accent)
		sel := i == m.gi

		bar := "  "
		if sel {
			c := lipgloss.TerminalColor(accent)
			if !focused {
				c = colDim
			}
			bar = fg(c).Render("▌") + " "
		}

		icon := fg(accent).Render("◆")
		nameStyle := sText
		if known && !st.Found {
			icon, nameStyle = sFaint.Render("◇"), sDim
		}
		if sel {
			nameStyle = nameStyle.Bold(true)
			if focused {
				nameStyle = nameStyle.Foreground(accent)
			}
		}

		right := ""
		if n := len(m.backups[g.ID]); n > 0 {
			right = sDim.Render(fmt.Sprint(n))
		}
		if st.Running {
			right = sOK.Render("●") + " " + right
		}
		name := nameStyle.Render(g.Short)
		lines = append(lines, joinLR(bar+icon+" "+name, strings.TrimSpace(right), iw))
	}

	// Totals pinned to the bottom of the pane.
	var count int
	var size int64
	for _, list := range m.backups {
		for _, b := range list {
			count++
			size += b.ZipSize
		}
	}
	for len(lines) < h-4 {
		lines = append(lines, "")
	}
	lines = append(lines, sFaint.Render(strings.Repeat("─", iw)),
		sFaint.Render(plural(count, "backup", "backups")+" · "+humanSize(size)))

	border := lipgloss.TerminalColor(colBorder)
	title := sDim.Render("Games")
	if focused {
		border, title = colBrand, sBrand.Bold(true).Render("Games")
	}
	return panel(title, lines, w, h, border)
}

func (m Model) viewBackups(w, h int) string {
	g := m.game()
	iw := w - 4
	accent := lipgloss.Color(g.Accent)
	focused := m.focus == paneBackups
	st, known := m.status[g.ID]

	var lines []string
	switch {
	case !known:
		lines = append(lines, sFaint.Render("Looking for the save folder…"), "")
	case st.Found:
		path := sText.Render(truncMiddle(st.Dir, iw-12))
		if st.Custom {
			path += sFaint.Render("  custom")
		}
		meta := "  " + sDim.Render(fmt.Sprintf("%s · %s · changed %s",
			plural(st.Files, "file", "files"), humanSize(st.Size), relTime(st.Modified)))
		state := ""
		if st.Running {
			state = sOK.Render("● running")
		}
		lines = append(lines, sOK.Render("●")+" "+path, joinLR(meta, state, iw))
	default:
		title := sWarn.Render("○ Save folder not found")
		hint := sDim.Render("press ") + sBold.Render("p") + sDim.Render(" to set it")
		lines = append(lines, joinLR(title, hint, iw), "  "+sFaint.Render(truncMiddle(st.Dir, iw-2)))
	}
	lines = append(lines, sFaint.Render(strings.Repeat("─", iw)))

	rows := m.listRows()
	list := m.backups[g.ID]
	switch err := m.listErr[g.ID]; {
	case err != nil:
		lines = append(lines, sErr.Render("✗ Can't read backups: "+err.Error()))
	default:
		for r := m.boff; r < min(len(list)+1, m.boff+rows); r++ {
			if r == 0 {
				lines = append(lines, m.newRow(st.Found || !known, focused, accent, iw))
				continue
			}
			lines = append(lines, m.backupRow(list[r-1], r == m.bi, focused, accent, iw))
		}
		if len(list) == 0 {
			lines = append(lines, emptyState(rows-1, iw, st.Found)...)
		}
	}
	for len(lines) < 3+rows {
		lines = append(lines, "")
	}

	lines = append(lines, sFaint.Render(strings.Repeat("─", iw)), m.backupDetails(list, iw))

	border := lipgloss.TerminalColor(colBorder)
	title := fg(accent).Render("◆") + " " + sDim.Render(g.Name)
	if focused {
		border = accent
		title = fg(accent).Render("◆") + " " + fg(accent).Bold(true).Render(g.Name)
	}
	return panel(title, lines, w, h, border)
}

// newRow is the "+ New backup" action at the top of the list.
func (m Model) newRow(enabled, focused bool, accent lipgloss.Color, iw int) string {
	sel := m.onNewRow()
	bar := "  "
	if sel {
		c := lipgloss.TerminalColor(accent)
		if !focused {
			c = colDim
		}
		bar = fg(c).Render("▌") + " "
	}
	label := fg(accent).Render("+") + " " + sText.Render("New backup")
	hint := ""
	switch {
	case !enabled:
		label = sFaint.Render("+ New backup")
		hint = sFaint.Render("save folder not found")
	case sel:
		label = fg(accent).Bold(true).Render("+ New backup")
		hint = sDim.Render("⏎ back up the current save")
	}
	return joinLR(bar+fit(label, badgeWidth+2+12), hint, iw)
}

func emptyState(rows, iw int, saveFound bool) []string {
	msg := []string{sDim.Render("No backups yet")}
	if saveFound {
		msg = append(msg, "", sFaint.Render("press ")+sBold.Render("⏎")+sFaint.Render(" on New backup, or ")+sBold.Render("s")+sFaint.Render(" for a quick save"))
	}
	out := make([]string, 0, rows)
	for i := 0; i < (rows-len(msg))/2; i++ {
		out = append(out, "")
	}
	for _, l := range msg {
		out = append(out, lipgloss.PlaceHorizontal(iw, lipgloss.Center, l))
	}
	return out
}

func (m Model) backupRow(b *store.Backup, sel, focused bool, accent lipgloss.Color, iw int) string {
	bar := "  "
	if sel {
		c := lipgloss.TerminalColor(accent)
		if !focused {
			c = colDim
		}
		bar = fg(c).Render("▌") + " "
	}

	const whenW, sizeW = 12, 9
	nameW := iw - 2 - badgeWidth - 2 - 2 - whenW - 1 - sizeW
	showSize := nameW >= 12
	if !showSize {
		nameW += 1 + sizeW
	}

	nameStyle := sText
	switch {
	case b.Err != nil:
		nameStyle = sErr
	case b.Kind == store.Auto:
		nameStyle = sDim
	}
	if sel {
		nameStyle = nameStyle.Bold(true)
	}

	row := bar + badge(b) + "  " + fit(nameStyle.Render(b.Name), nameW) + "  " + padLeft(sDim.Render(relTime(b.Created)), whenW)
	if showSize {
		row += " " + padLeft(sFaint.Render(humanSize(b.ZipSize)), sizeW)
	}
	return row
}

func (m Model) backupDetails(list []*store.Backup, iw int) string {
	b := m.selected()
	if b == nil {
		return sFaint.Render(plural(len(list), "backup", "backups") + " · " + humanSize(totalZip(list)) + " on disk")
	}
	pos := sFaint.Render(fmt.Sprintf("%d/%d", m.bi, len(list)))
	if b.Err != nil {
		return joinLR(sErr.Render("✗ "+b.Err.Error()), pos, iw)
	}
	info := fmt.Sprintf("%s · %s · %s save → %s archive",
		b.Created.Format("02 Jan 2006 15:04:05"), plural(len(b.Files), "file", "files"),
		humanSize(b.TotalSize()), humanSize(b.ZipSize))
	return joinLR(sFaint.Render(info), pos, iw)
}

func (m Model) viewStatus() string {
	if m.busy != "" {
		return " " + sBrand.Render(spinnerFrames[m.frame%len(spinnerFrames)]) + " " + shimmer(m.busy+"…", m.frame)
	}
	if m.toast == nil {
		return ""
	}
	icon := map[toastKind]string{
		toastOK:   sOK.Render("✓"),
		toastWarn: sWarn.Render("▲"),
		toastErr:  sErr.Render("✗"),
		toastInfo: sInfo.Render("●"),
	}[m.toast.kind]
	text := sText.Render(m.toast.text)
	if m.toast.kind == toastErr {
		text = sErr.Render(m.toast.text)
	}
	return joinLR(" "+icon+" "+text, sFaint.Render(m.toast.at.Format(time.TimeOnly))+" ", m.width)
}

func (m Model) viewKeys() string {
	var keys []string
	switch {
	case m.focus == paneGames:
		keys = []string{"⏎", "backups", "s", "quick save", "l", "quick load", "p", "save folder"}
	case m.selected() != nil:
		keys = []string{"⏎", "restore", "r", "rename", "d", "delete", "s", "quick save", "l", "quick load"}
	default:
		keys = []string{"⏎", "new backup", "s", "quick save", "l", "quick load", "←", "games"}
	}
	keys = append(keys, "?", "all keys", "q", "quit")

	// Keep "? all keys" and "q quit" visible; drop hints from the middle when the window is narrow.
	tail := keys[len(keys)-4:]
	keys = keys[:len(keys)-4]
	sep := sFaint.Render(" · ")
	render := func(ks []string) string {
		var parts []string
		for i := 0; i < len(ks); i += 2 {
			parts = append(parts, sBold.Render(ks[i])+" "+sDim.Render(ks[i+1]))
		}
		return " " + strings.Join(parts, sep)
	}
	line := render(append(append([]string{}, keys...), tail...))
	for len(keys) > 0 && lipgloss.Width(line) > m.width {
		keys = keys[:len(keys)-2]
		line = render(append(append([]string{}, keys...), tail...))
	}
	return fit(line, m.width)
}

// overlay centres fg over bg, dimming bg so the dialog stands out.
func overlay(bg, fgBlock string, w, h int) string {
	bgLines := strings.Split(bg, "\n")
	for len(bgLines) < h {
		bgLines = append(bgLines, "")
	}
	for i, l := range bgLines {
		bgLines[i] = ansi.Strip(l)
	}

	fgLines := strings.Split(fgBlock, "\n")
	fw := lipgloss.Width(fgBlock)
	x := max((w-fw)/2, 0)
	y := max((h-len(fgLines))/2, 0)

	for i, l := range bgLines {
		fi := i - y
		if fi < 0 || fi >= len(fgLines) {
			bgLines[i] = sFaint.Render(l)
			continue
		}
		plain := l + strings.Repeat(" ", max(w-lipgloss.Width(l), 0))
		left := ansi.Truncate(plain, x, "")
		right := ansi.TruncateLeft(plain, x+fw, "")
		bgLines[i] = sFaint.Render(left) + fit(fgLines[fi], fw) + sFaint.Render(right)
	}
	return strings.Join(bgLines[:h], "\n")
}

func totalZip(list []*store.Backup) int64 {
	var n int64
	for _, b := range list {
		n += b.ZipSize
	}
	return n
}
