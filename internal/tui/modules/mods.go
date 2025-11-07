package modules

import (
	"HoneyOps/cloud/aws/deploy"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
)

type rightMode int

const (
	ModeForm rightMode = iota
	ModeDisplay
)

type activeState int

const (
	active activeState = iota
	inactive
)

const (
	page1 = iota
	page2
	page3
	page4
	page5
)

var (
	listViewStyle = lipgloss.NewStyle().
			PaddingRight(1).
			MarginRight(1).
			Border(lipgloss.RoundedBorder(), false, true, false, false)
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"})

	statusNugget   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Padding(0, 1)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
			Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})
	statusStyle = statusBarStyle.Copy().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#FF5F87")).
			Padding(0, 1).
			MarginRight(1)
	encodingStyle = statusNugget.Copy().Background(lipgloss.Color("#A550DF")).Align(lipgloss.Right)
	statusText    = statusBarStyle.Copy()
	datetimeStyle = statusNugget.Copy().Background(lipgloss.Color("#6124DF"))

	red           = lipgloss.Color("#e02222ff")
	lime          = lipgloss.Color("#65d84eff")
	purple        = lipgloss.Color("#8A5FFF")
	pink          = lipgloss.Color("#DB2777")
	darkPink      = lipgloss.Color("#ac215f")
	stylePink     = lipgloss.NewStyle().Foreground(pink)
	stylePinkB    = stylePink.Bold(true)
	styleSuccess  = lipgloss.NewStyle().Foreground(lime)
	styleError    = lipgloss.NewStyle().Foreground(red)
	styleAct      = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffffff"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	stylePurple   = lipgloss.NewStyle().Foreground(purple)
	styleSel      = lipgloss.NewStyle().Foreground(lipgloss.Color("#000")).Background(pink)
	styleDarkSel  = lipgloss.NewStyle().Foreground(lipgloss.Color("#000")).Background(darkPink)
	styleDarkPink = lipgloss.NewStyle().Foreground(lipgloss.Color("#ac215f"))

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

	titleStyle = statusBarStyle.
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#8A5FFF")).
			Padding(0, 1).
			MarginRight(1)

	titleStyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Padding(0, 1).
			MarginRight(1)
)

type Mod interface {
	Title() string
	Description() string
	FilterValue() string
	Mode() int

	// Switch Between Form and Display
	DisplayOutput(PulumiMsg) []tea.Cmd
	HandleDeployMsg(DeloyMsg) []tea.Cmd

	// For Grey Out and Brightening of text
	FocusRight()
	Deactive()
	ViewForms() string
	UpdateForm(tea.KeyMsg) []tea.Cmd
	SetMode(int)

	// For Resizing
	SetSize(int, int)
}

type DeloyMsg struct {
	campiagnName string
	destroy      bool
}

type DestroyMsg struct {
	campiagnName string
	destroy      bool
}

type BackMsg struct{}

type SshMsg struct{}

type UpdateFormMsg struct{}

type DeloyErrMsg struct{ err error }

type DeloyDoneMsg struct{}

type PulumiMsg string

type PulumiErrMsg struct{ err error }

type PulumiDoneMsg struct{}

func WaitForPulumiResponses(sub chan string) tea.Cmd {
	return func() tea.Msg {
		return PulumiMsg(<-sub)
	}

}
func ExecutePulumi(destroy bool, campiagnName string, sub chan string) tea.Cmd {
	return func() tea.Msg {

		pr, pw := io.Pipe()

		awsStack := deploy.NewAwsPulumiDeployer()

		var wg sync.WaitGroup
		wg.Add(2) // We have two goroutines: one writer, one reader

		// Writer goroutine
		go func() {
			defer wg.Done()
			defer pw.Close() // Close the writer when done

			// Run this as a go routine to prevent lock up
			awsStack.UpPipe(destroy, campiagnName, pw)

		}()

		// Reader goroutine
		go func() {
			defer wg.Done()
			defer pr.Close() // Close the reader when done

			buffer := make([]byte, 64) // Create a buffer to read into
			for {
				n, err := pr.Read(buffer) // Read data into the buffer
				if n > 0 {
					sub <- string(buffer[:n])
				}
				if err == io.EOF {
					sub <- string("Reader reached end of pipe.")
					break // End of file, no more data
				}
				if err != nil {
					sub <- fmt.Sprintf("Error reading from pipe: %v", err)
					break
				}

			}
		}()

		wg.Wait() // Wait for both goroutines to complete

		return PulumiDoneMsg{}

	}
}

type SSHConfig struct {
	//Auth           Auth
	User           string
	Addr           string
	Port           uint
	Timeout        time.Duration
	Callback       ssh.HostKeyCallback
	BannerCallback ssh.BannerCallback
}

func SSHClientConnect(campaignName string, ec2Name string) {
	awsStack := deploy.NewAwsPulumiDeployer()
	awsStack.ReadConfig(campaignName)

	awsStack.ConnectSSH(ec2Name)
}

const listHeight = 14

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	campaignName := string(i[strings.LastIndex(string(i), string(os.PathSeparator))+1:])
	campaignName = strings.Replace(campaignName, "HoneyCloud_", "", 1)
	campaignName = strings.Replace(campaignName, ".yaml", "", 1)

	str := fmt.Sprintf("%d. %s", index+1, campaignName)

	fn := styleAct.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return stylePink.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func FilePathWalkDir(root string) ([]list.Item, error) {
	var files []list.Item
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			// Skipping Empty Campaign names
			if info.Name() != "HoneyCloud_.yaml" {
				files = append(files, item(info.Name()))
			}

		}
		return nil
	})
	return files, err
}
