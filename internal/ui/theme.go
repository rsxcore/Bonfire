package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rsxcore/Bonfire/internal/store"
)

// Palette: warm bonfire tones on a neutral base.
var (
	colText   = lipgloss.AdaptiveColor{Light: "#2B2724", Dark: "#E9E4DA"}
	colDim    = lipgloss.AdaptiveColor{Light: "#6F6962", Dark: "#9A9388"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#ABA59C", Dark: "#5C574F"}
	colBorder = lipgloss.AdaptiveColor{Light: "#D3CDC3", Dark: "#3A3631"}
	colBrand  = lipgloss.Color(hexBrand)
	colOK     = lipgloss.Color("#98C379")
	colErr    = lipgloss.Color("#E06C75")
	colWarn   = lipgloss.Color("#E5C07B")
	colInfo   = lipgloss.Color("#7AA2F7")
	colInk    = lipgloss.Color("#1B1916") // text on coloured badges and buttons
)

const (
	hexBrand = "#F0A35E"
	hexGold  = "#E8C872"
)

var (
	sText  = lipgloss.NewStyle().Foreground(colText)
	sBold  = sText.Bold(true)
	sDim   = lipgloss.NewStyle().Foreground(colDim)
	sFaint = lipgloss.NewStyle().Foreground(colFaint)
	sBrand = lipgloss.NewStyle().Foreground(colBrand)
	sOK    = lipgloss.NewStyle().Foreground(colOK)
	sErr   = lipgloss.NewStyle().Foreground(colErr)
	sWarn  = lipgloss.NewStyle().Foreground(colWarn)
	sInfo  = lipgloss.NewStyle().Foreground(colInfo)
)

func fg(c lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

const badgeWidth = 8

func badge(b *store.Backup) string {
	label, bg, text := "MANUAL", lipgloss.TerminalColor(lipgloss.Color("#C49BE0")), lipgloss.TerminalColor(colInk)
	switch {
	case b.Err != nil:
		label, bg = "BROKEN", colErr
	case b.Kind == store.Quick:
		label, bg = "QUICK", lipgloss.Color("#6CC4C9")
	case b.Kind == store.Auto:
		label, bg, text = "AUTO", lipgloss.AdaptiveColor{Light: "#D8D2C8", Dark: "#48433D"}, colText
	}
	return lipgloss.NewStyle().Background(bg).Foreground(text).Bold(true).
		Width(badgeWidth).Align(lipgloss.Center).Render(label)
}

// gradient colours each rune of s along a from→to hex gradient.
func gradient(s, from, to string, bold bool) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		t := 0.0
		if len(rs) > 1 {
			t = float64(i) / float64(len(rs)-1)
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(lerpHex(from, to, t))).Bold(bold).Render(string(r)))
	}
	return b.String()
}

// shimmer renders s in the brand colour with a bright glint sweeping across it.
func shimmer(s string, frame int) string {
	rs := []rune(s)
	pos := frame%(len(rs)+10) - 5
	var b strings.Builder
	for i, r := range rs {
		c := hexBrand
		switch d := abs(i - pos); {
		case d == 0:
			c = "#FFF3E2"
		case d == 1:
			c = "#FFD9AE"
		case d == 2:
			c = "#F8C088"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(string(r)))
	}
	return b.String()
}

func lerpHex(a, b string, t float64) string {
	ar, ag, ab := parseHex(a)
	br, bg, bb := parseHex(b)
	mix := func(x, y float64) int { return int(x + (y-x)*t + 0.5) }
	return fmt.Sprintf("#%02X%02X%02X", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func parseHex(s string) (r, g, b float64) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	return float64(v >> 16 & 0xFF), float64(v >> 8 & 0xFF), float64(v & 0xFF)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// fit truncates or pads a styled string to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// padLeft right-aligns a styled string in w cells.
func padLeft(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// joinLR places left and right at the edges of a w-cell line; left gets truncated if they collide.
func joinLR(left, right string, w int) string {
	rw := lipgloss.Width(right)
	if rw >= w {
		return fit(right, w)
	}
	return fit(left, w-rw-1) + " " + right
}

// truncMiddle shortens plain text like paths by cutting out the middle: C:\Users\…\EldenRing.
func truncMiddle(s string, w int) string {
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w < 5 {
		return string(rs[:max(w, 0)])
	}
	head := (w - 1) / 3
	tail := w - 1 - head
	return string(rs[:head]) + "…" + string(rs[len(rs)-tail:])
}
