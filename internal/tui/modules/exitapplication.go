package modules

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

/* ───────── Create Campaign Form ───────── */

type ExitApplication struct {
	title, desc string
	mode        int
	active      int

	exitSel      int
	exitChoice   []string
	windowWidth  int
	windowHeight int
}

func (e ExitApplication) Title() string       { return e.title }
func (e ExitApplication) Description() string { return e.desc }
func (e ExitApplication) FilterValue() string { return e.title }
func (e ExitApplication) Mode() int           { return e.mode }
func (e ExitApplication) TotalFields() int    { return 0 }

func InitExitApplication() *ExitApplication {

	return &ExitApplication{
		title:      "Exit",
		desc:       "Close the application",
		mode:       int(ModeForm),
		active:     int(inactive),
		exitSel:    0,
		exitChoice: []string{"Yes", "No"},
	}
}

func (e *ExitApplication) ViewForms() string {
	tStyle := titleStyleDim
	if e.active == int(active) {
		tStyle = titleStyle
	}

	var b strings.Builder
	b.WriteString(tStyle.Render("Close the application?") + "\n\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("‾", 50)) + "\n")

	for i := 0; i < len(e.exitChoice); i++ {
		if e.exitSel == i {
			b.WriteString(styleSel.Render(e.exitChoice[i]))
		} else {
			b.WriteString(styleDim.Render(e.exitChoice[i]))
		}
		b.WriteString("  ")
	}

	return b.String()
}

func (e *ExitApplication) UpdateForm(key tea.KeyMsg) []tea.Cmd {
	switch key.String() {
	case "up", "left":
		e.exitSel--
		if e.exitSel < 0 {
			e.exitSel = 0
		}
	case "down", "right":
		e.exitSel++
		if e.exitSel >= len(e.exitChoice) {
			e.exitSel = 1
		}
	case "enter":
		choice := e.exitChoice[e.exitSel]
		if choice == "Yes" {
			cmd := tea.Quit
			return []tea.Cmd{cmd}
		}

	}

	return nil
}

func (e *ExitApplication) FocusRight() {
	e.active = int(active)

}

func (e *ExitApplication) Deactive() {
	e.active = int(inactive)
}

/* ───────── Display Output  ───────── */

func (e *ExitApplication) DisplayOutput(pulumiStd PulumiMsg) []tea.Cmd {

	return nil
}
func (e *ExitApplication) HandleDeployMsg(msg DeloyMsg) []tea.Cmd {
	return nil
}

/* ───────── Display Mode Form or ViewPort  ───────── */

func (e *ExitApplication) SetMode(mode int) {
	e.mode = mode
}

/* ───────── Reset Forms  ───────── */

func (e *ExitApplication) Reset() {

	e.mode = int(ModeForm)
}

func (e *ExitApplication) SetSize(width int, height int) {
	e.windowWidth = width
	e.windowHeight = height
}
