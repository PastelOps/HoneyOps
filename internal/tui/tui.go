package tui

import (
	"HoneyOps/internal/tui/modules"
	"bytes"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	defaultState state = iota
	selectionState
	searchState
)

//nolint:govet
type model struct {
	width, height int

	list      list.Model
	textinput textinput.Model
	viewport  viewport.Model
	spinner   spinner.Model

	searchValue   string
	statusMessage string
	ready         bool
	now           string

	sub    chan string
	buffer bytes.Buffer
	mode   int

	offset int64
	limit  int64 // scan size

	keyMap
	state
}

func registerModules() []list.Item {

	/*
		To add more or reorder options in the left panel list
	*/
	mods := []list.Item{
		modules.InitDashboard(),
		modules.InitCreateCampaign(),
		modules.InitInteractApplication(),
		modules.InitDisplayHelp(),
		modules.InitExitApplication(),
	}

	return mods

}

// Main Definitation For the Screen
// Left is a list

func New() (*model, error) {
	t := textinput.New()
	t.Prompt = "> "
	t.Placeholder = "Search Key"
	t.PlaceholderStyle = lipgloss.NewStyle()

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "HoneyOps"
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)

	listModules := registerModules()
	l.SetItems(listModules)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return &model{
		list:      l,
		textinput: t,
		spinner:   s,
		limit:     20,

		keyMap: defaultKeyMap(),
		state:  defaultState,
		sub:    make(chan string),
		buffer: bytes.Buffer{},
	}, nil
}
