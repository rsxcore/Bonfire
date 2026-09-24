package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rsxcore/Bonfire/internal/store"
)

// A dialog floats over the main screen and takes all key input until it
// returns nil from update.
type dialog interface {
	update(tea.KeyMsg) (dialog, tea.Cmd)
	view(screenWidth int) string
}

const dialogWidth = 66

func dialogInner(screenWidth int) int { return min(dialogWidth, screenWidth-4) - 8 }

func frame(icon, title string, accent lipgloss.TerminalColor, body string, screenWidth int) string {
	head := fg(accent).Bold(true).Render(icon) + "  " + sBold.Render(title)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 3).
		Width(min(dialogWidth, screenWidth-4) - 2).
		Render(head + "\n\n" + body)
}

func button(label string, active bool, accent lipgloss.TerminalColor) string {
	st := lipgloss.NewStyle().Padding(0, 2)
	if active {
		return st.Background(accent).Foreground(colInk).Bold(true).Render(label)
	}
	return st.Foreground(colDim).Render(label)
}

func keyHints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, sDim.Render(pairs[i])+" "+sFaint.Render(pairs[i+1]))
	}
	return strings.Join(parts, sFaint.Render("  ·  "))
}

// Confirm.

type confirmDialog struct {
	icon, title string
	body        []string
	yes         string
	danger      bool
	onYes       tea.Cmd
	yesFocused  bool
}

// newConfirm builds a yes/cancel dialog. Dangerous ones start with Cancel focused.
func newConfirm(icon, title string, body []string, yes string, danger bool, onYes tea.Cmd) *confirmDialog {
	return &confirmDialog{icon: icon, title: title, body: body, yes: yes, danger: danger, onYes: onYes, yesFocused: !danger}
}

func (d *confirmDialog) update(k tea.KeyMsg) (dialog, tea.Cmd) {
	switch k.String() {
	case "left", "right", "tab", "shift+tab":
		d.yesFocused = !d.yesFocused
	case "y":
		return nil, d.onYes
	case "n", "esc", "q":
		return nil, nil
	case "enter":
		if d.yesFocused {
			return nil, d.onYes
		}
		return nil, nil
	}
	return d, nil
}

func (d *confirmDialog) view(sw int) string {
	accent := lipgloss.TerminalColor(colBrand)
	if d.danger {
		accent = colErr
	}
	cancelBg := lipgloss.AdaptiveColor{Light: "#D8D2C8", Dark: "#5C574F"}
	buttons := button(d.yes, d.yesFocused, accent) + " " + button("Cancel", !d.yesFocused, cancelBg)
	body := strings.Join(d.body, "\n") + "\n\n" + joinLR(keyHints("y", "yes", "n", "no"), buttons, dialogInner(sw))
	return frame(d.icon, d.title, accent, body, sw)
}

// Single-line input.

type inputDialog struct {
	icon, title, hint string
	in                textinput.Model
	submit            func(string) (tea.Cmd, string)
	err               string
}

func newTextInput(value, placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = sBrand.Bold(true)
	ti.TextStyle = sText
	ti.PlaceholderStyle = sFaint
	ti.Placeholder = placeholder
	ti.CharLimit = 260
	ti.Width = dialogWidth - 16
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.Cursor.Style = sBrand
	ti.SetValue(value)
	ti.CursorEnd()
	return ti
}

// newInput builds a one-field dialog. submit returns the command to run, or a
// validation message that keeps the dialog open.
func newInput(icon, title, hint, value, placeholder string, submit func(string) (tea.Cmd, string)) *inputDialog {
	d := &inputDialog{icon: icon, title: title, hint: hint, in: newTextInput(value, placeholder), submit: submit}
	d.in.Focus()
	return d
}

func (d *inputDialog) update(k tea.KeyMsg) (dialog, tea.Cmd) {
	switch k.String() {
	case "esc":
		return nil, nil
	case "enter":
		cmd, msg := d.submit(strings.TrimSpace(d.in.Value()))
		if msg != "" {
			d.err = msg
			return d, nil
		}
		return nil, cmd
	}
	d.err = ""
	d.in, _ = d.in.Update(k)
	return d, nil
}

func fieldBox(in textinput.Model, focused bool, width int) string {
	border := lipgloss.TerminalColor(colBorder)
	if focused {
		border = colBrand
	}
	in.Width = width - 5
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(width - 2).Render(in.View())
}

func (d *inputDialog) view(sw int) string {
	w := dialogInner(sw)
	var b strings.Builder
	if d.hint != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colDim).Width(w).Render(d.hint) + "\n\n")
	}
	b.WriteString(fieldBox(d.in, true, w))
	if d.err != "" {
		b.WriteString("\n" + sErr.Render("✗ "+d.err))
	}
	b.WriteString("\n\n" + keyHints("enter", "confirm", "esc", "cancel"))
	return frame(d.icon, d.title, colBrand, b.String(), sw)
}

// Multi-field form.

type formField struct{ label, value string }

type formDialog struct {
	icon, title string
	labels      []string
	ins         []textinput.Model
	focus       int
	submit      func([]string) (tea.Cmd, string)
	err         string
}

func newForm(icon, title string, fields []formField, submit func([]string) (tea.Cmd, string)) *formDialog {
	d := &formDialog{icon: icon, title: title, submit: submit}
	for _, f := range fields {
		d.labels = append(d.labels, f.label)
		d.ins = append(d.ins, newTextInput(f.value, ""))
	}
	d.ins[0].Focus()
	return d
}

func (d *formDialog) setFocus(i int) {
	d.ins[d.focus].Blur()
	d.focus = (i + len(d.ins)) % len(d.ins)
	d.ins[d.focus].Focus()
}

func (d *formDialog) update(k tea.KeyMsg) (dialog, tea.Cmd) {
	switch k.String() {
	case "esc":
		return nil, nil
	case "tab", "down":
		d.setFocus(d.focus + 1)
		return d, nil
	case "shift+tab", "up":
		d.setFocus(d.focus - 1)
		return d, nil
	case "enter":
		values := make([]string, len(d.ins))
		for i, in := range d.ins {
			values[i] = strings.TrimSpace(in.Value())
		}
		cmd, msg := d.submit(values)
		if msg != "" {
			d.err = msg
			return d, nil
		}
		return nil, cmd
	}
	d.err = ""
	d.ins[d.focus], _ = d.ins[d.focus].Update(k)
	return d, nil
}

func (d *formDialog) view(sw int) string {
	w := dialogInner(sw)
	var b strings.Builder
	for i, in := range d.ins {
		label := sDim.Render(d.labels[i])
		if i == d.focus {
			label = sText.Render(d.labels[i])
		}
		b.WriteString(label + "\n" + fieldBox(in, i == d.focus, w) + "\n")
	}
	if d.err != "" {
		b.WriteString(sErr.Render("✗ "+d.err) + "\n")
	}
	b.WriteString("\n" + keyHints("tab", "next field", "enter", "save", "esc", "cancel"))
	return frame(d.icon, d.title, colBrand, b.String(), sw)
}

// Help.

type helpDialog struct{}

func newHelpDialog() helpDialog { return helpDialog{} }

func (helpDialog) update(tea.KeyMsg) (dialog, tea.Cmd) { return nil, nil }

func (helpDialog) view(sw int) string {
	colW := (dialogInner(sw) - 2) / 2
	section := func(title string, rows ...[2]string) string {
		out := sBrand.Bold(true).Render(title)
		for _, r := range rows {
			out += "\n" + fit(sBold.Render(r[0]), 9) + fit(sDim.Render(r[1]), colW-9)
		}
		return out
	}
	column := func(sections ...string) string {
		return lipgloss.NewStyle().Width(colW).Render(strings.Join(sections, "\n\n"))
	}

	left := column(
		section("Backups",
			[2]string{"s  F5", "quick save"},
			[2]string{"l  F9", "quick load"},
			[2]string{"enter", "new / restore"},
			[2]string{"n", "new backup"},
			[2]string{"r", "rename"},
			[2]string{"d  del", "delete"}),
		section("App",
			[2]string{",", "settings"},
			[2]string{"q", "quit"}))
	right := column(
		section("Navigate",
			[2]string{"↑ ↓", "move"},
			[2]string{"← → tab", "switch pane"},
			[2]string{"g  G", "first / last"}),
		section("Folders",
			[2]string{"o", "open save folder"},
			[2]string{"b", "open backups"},
			[2]string{"p", "set save folder"}))

	kind := func(k store.Kind, text string) string {
		return badge(&store.Backup{Manifest: store.Manifest{Kind: k}}) + " " + sDim.Render(text)
	}
	body := strings.Join([]string{
		lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right),
		kind(store.Quick, "one slot, replaced on every quick save") + "\n" +
			kind(store.Manual, "named by you, kept until you delete it") + "\n" +
			kind(store.Auto, "your save right before a load, for undo"),
		keyHints("any key", "close"),
	}, "\n\n")
	return frame("?", "Keys", colBrand, body, sw)
}
