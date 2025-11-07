package tui

import (
	"HoneyOps/internal/tui/modules"
	"os"

	"github.com/spf13/cast"
	"golang.org/x/term"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) updateSize(w, h int) {
	m.width = w
	m.height = h

	m.viewport = viewport.New(m.width, m.height)

}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.viewport.SetContent(m.list.SelectedItem().(modules.Mod).ViewForms())
	// global msg handling
	switch msg := msg.(type) {
	case errMsg:
		m.statusMessage = msg.err.Error()
		// TODO: log error
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		statusBarHeight := lipgloss.Height(m.statusView())
		height := m.height - statusBarHeight

		listViewWidth := cast.ToInt(0.3 * float64(m.width))
		listWidth := listViewWidth - listViewStyle.GetHorizontalFrameSize()
		m.list.SetSize(listWidth, height)

		detailViewWidth := m.width - listViewWidth
		m.viewport = viewport.New(detailViewWidth, height)

		m.viewport.MouseWheelEnabled = true
		m.viewport.SetContent(m.list.SelectedItem().(modules.Mod).ViewForms())

		for _, val := range m.list.Items() {
			val.(modules.Mod).SetSize(detailViewWidth, height)
		}

	case tickMsg:
		m.now = msg.t
		cmds = append(cmds, m.tickCmd())

		// Readjust screen size
		w, h, _ := term.GetSize(int(os.Stdout.Fd()))
		if w != m.width || h != m.height {
			m.updateSize(w, h)
			cmds = append(cmds, func() tea.Msg { return tea.WindowSizeMsg{Width: w, Height: h} })
		}
	case scanMsg:
		m.list.SetItems(msg.items)
	case countMsg:
		m.ready = true
	}

	switch m.state {
	case defaultState:
		cmds = append(cmds, m.handleDefaultState(msg))
	case selectionState:
		cmds = append(cmds, m.handleSelectedState(msg))
	}

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleDefaultState(msg tea.Msg) tea.Cmd {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			cmd = tea.Quit
			cmds = append(cmds, cmd)
		case tea.KeyUp, tea.KeyDown:
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		case tea.KeyEnter:
			m.state = selectionState
			m.list.SelectedItem().(modules.Mod).FocusRight()
			m.viewport.GotoTop()
			m.viewport.SetContent(m.list.SelectedItem().(modules.Mod).ViewForms())
		}
	default:
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *model) handleSelectedState(msg tea.Msg) tea.Cmd {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)
	m.viewport.SetContent(m.list.SelectedItem().(modules.Mod).ViewForms())
	switch msg := msg.(type) {
	case modules.BackMsg:
		m.state = defaultState
	case tea.MouseMsg:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case modules.DeloyMsg:
		m.list.SelectedItem().(modules.Mod).FocusRight()
		newCmds := m.list.SelectedItem().(modules.Mod).HandleDeployMsg(msg)
		cmds = append(cmds, newCmds...)
	case modules.PulumiMsg:
		newCmds := m.list.SelectedItem().(modules.Mod).DisplayOutput(msg)
		m.viewport.GotoBottom()
		cmds = append(cmds, newCmds...)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			m.textinput.Blur()
			m.textinput.Reset()
			m.state = defaultState
			m.list.SelectedItem().(modules.Mod).Deactive()
		default:
			newCmds := m.updateRightPane(msg)
			cmds = append(cmds, newCmds...)
		}
	}

	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

func (m *model) updateRightPane(key tea.KeyMsg) []tea.Cmd {
	switch m.mode {
	case int(modules.ModeForm):
		return m.list.SelectedItem().(modules.Mod).UpdateForm(key)
	case int(modules.ModeDisplay):
		m.viewport.GotoTop()
		m.viewport.SetContent(m.viewportContent(m.viewport.Width))
	}
	return nil
}
