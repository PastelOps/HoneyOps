package modules

import (
	"HoneyOps/cloud/aws/deploy"
	"HoneyOps/common"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cast"
)

const (
	dashboardSession sessionState = iota
	dashDeleteSession
	dashPulumiDeleteSession
	dashDeploySession
	dashPulumiDeploySession
)

/* ───────── Create Campaign Form ───────── */

type Dashboard struct {
	title, desc    string
	mode           int
	active         int
	table          table.Model
	choiceMessage  string
	destroySel     int
	destroyChoices []string
	deploySel      int
	deployChoices  []string
	sub            chan string
	stdDisplay     string
	state          sessionState

	windowWidth  int
	windowHeight int
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func (d Dashboard) Title() string       { return d.title }
func (d Dashboard) Description() string { return d.desc }
func (d Dashboard) FilterValue() string { return d.title }
func (d Dashboard) Mode() int           { return d.mode }
func (d Dashboard) TotalFields() int    { return 0 }

func refresh(width int, height int) table.Model {
	campaignList, _ := FilePathWalkDir(common.GetHoneyOpsCampaignDir())

	columns := []table.Column{
		{Title: "Name", Width: cast.ToInt(0.2 * float64(width))},
		{Title: "Deployed", Width: cast.ToInt(0.2 * float64(width))},
		{Title: "Cloud", Width: cast.ToInt(0.2 * float64(width))},
		{Title: "Tools Installed", Width: cast.ToInt(0.4*float64(width) - 8)},
	}

	rows := []table.Row{}

	for _, element := range campaignList {

		campaignName := string(string(element.(item))[strings.LastIndex(string(element.(item)), string(os.PathSeparator))+1:])
		campaignName = strings.Replace(campaignName, "HoneyCloud_", "", 1)
		campaignName = strings.Replace(campaignName, ".yaml", "", 1)

		awsStack := deploy.NewAwsPulumiDeployer()
		awsStack.ReadConfig(campaignName)

		row := []string{campaignName, awsStack.Status, strings.ToUpper(awsStack.CloudProvider), strings.Join(awsStack.ToolsInstalled, ",")}
		rows = append(rows, row)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return t
}

func (d *Dashboard) RefreshTable() {
	d.state = dashboardSession
	d.table = refresh(d.windowWidth, d.windowHeight)
}

func (d *Dashboard) SetSize(width int, height int) {
	d.windowWidth = width
	d.windowHeight = height
}

func InitDashboard() *Dashboard {

	return &Dashboard{
		title:          "Dashboard",
		desc:           "Overview of campaigns",
		mode:           int(ModeForm),
		active:         int(inactive),
		table:          refresh(100, 100),
		destroySel:     0,
		destroyChoices: []string{"Yes", "No"},
		deploySel:      0,
		deployChoices:  []string{"Yes", "No"},
		sub:            make(chan string),
		stdDisplay:     "",
		state:          dashboardSession,
	}
}

func (d *Dashboard) ViewForms() string {
	style := styleDim
	tstyle := titleStyleDim
	if d.active == int(active) {
		tstyle = titleStyle
		style = styleAct
	}

	var b strings.Builder

	switch d.state {
	case dashPulumiDeleteSession:
		b.WriteString(tstyle.Render("Destroying Cloud Resources") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", d.windowWidth)) + "\n")
		b.WriteString(styleDim.Render("'b' / 'backspace' back"))
		b.WriteString(styleDim.Render("\n\n"))
		b.WriteString(d.stdDisplay)
	case dashPulumiDeploySession:
		b.WriteString(tstyle.Render("Deploying Cloud Resources") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", d.windowWidth)) + "\n")
		b.WriteString(styleDim.Render("'b' / 'backspace' back"))
		b.WriteString(styleDim.Render("\n\n"))
		b.WriteString(d.stdDisplay)
	default:
		b.WriteString(tstyle.Render("Overall Summary") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", d.windowWidth)) + "\n")
		b.WriteString(styleDim.Render("Esc Back ·  ↑/↓ fields · 'r' Refresh Table · 'x' Destroy · 'd' Deploy · 'Enter' View Details"))
		b.WriteString(style.Render("\n\nCampaigns") + d.choiceMessage + "\n\n")

		b.WriteString(style.Render(d.table.View()) + "\n\n")
	}

	return b.String()

}

func (d *Dashboard) updateDestroyMessage() {
	campaignName := d.table.SelectedRow()[0]
	d.choiceMessage = " -- Are you sure you want to destroy " + stylePink.Render(campaignName) + "? "

	for i := 0; i < len(d.destroyChoices); i++ {
		if d.destroySel == i {
			d.choiceMessage += (styleSel.Render(d.destroyChoices[i]))
		} else {
			d.choiceMessage += (styleDim.Render(d.destroyChoices[i]))
		}
		d.choiceMessage += "  "
	}
}

func (d *Dashboard) updateDeployMessage() {
	campaignName := d.table.SelectedRow()[0]
	d.choiceMessage = " -- Are you sure you want to deploy " + stylePink.Render(campaignName) + " now? "

	for i := 0; i < len(d.deployChoices); i++ {
		if d.deploySel == i {
			d.choiceMessage += (styleSel.Render(d.deployChoices[i]))
		} else {
			d.choiceMessage += (styleDim.Render(d.deployChoices[i]))
		}
		d.choiceMessage += "  "
	}
}

func (d *Dashboard) UpdateForm(msg tea.KeyMsg) []tea.Cmd {

	var cmd tea.Cmd

	if len(d.choiceMessage) != 0 {
		switch msg.String() {
		case "right":
			switch d.state {
			case dashDeleteSession:
				if d.destroySel == 0 {
					d.destroySel = 1
					d.updateDestroyMessage()
				}
			case dashDeploySession:
				if d.deploySel == 0 {
					d.deploySel = 1
					d.updateDeployMessage()
				}
			}

		case "left":
			switch d.state {
			case dashDeleteSession:
				if d.destroySel == 1 {
					d.destroySel = 0
					d.updateDestroyMessage()
				}
			case dashDeploySession:
				if d.deploySel == 1 {
					d.deploySel = 0
					d.updateDeployMessage()
				}
			}

		case "enter":
			d.choiceMessage = ""
			switch d.state {
			case dashDeleteSession:
				if d.destroySel == 0 {
					d.state = dashPulumiDeleteSession
					return []tea.Cmd{func() tea.Msg {
						return DeloyMsg{
							campiagnName: d.table.SelectedRow()[0],
							destroy:      true,
						}
					}}
				} else {
					d.destroySel = 0
					d.state = dashboardSession
				}
			case dashDeploySession:
				if d.deploySel == 0 {
					d.state = dashPulumiDeploySession
					return []tea.Cmd{func() tea.Msg {
						return DeloyMsg{
							campiagnName: d.table.SelectedRow()[0],
							destroy:      false,
						}
					}}
				} else {
					d.deploySel = 0
					d.state = dashboardSession
				}
			}

		}

	} else {

		if d.state == dashPulumiDeleteSession || d.state == dashPulumiDeploySession {
			switch msg.String() {
			case "b", "backspace":
				d.RefreshTable()
			}
		} else {
			switch msg.String() {
			case "esc":
				if d.table.Focused() {
					d.table.Blur()
				} else {
					d.table.Focus()
				}
			case "r":
				d.RefreshTable()
			case "x":
				d.state = dashDeleteSession
				d.updateDestroyMessage()
			case "d":
				d.state = dashDeploySession
				d.updateDeployMessage()
			}

			if msg.String() != "d" {
				d.table, cmd = d.table.Update(msg)
			}
		}

	}

	return []tea.Cmd{cmd}
}

func (d *Dashboard) FocusRight() {
	d.active = int(active)

}

func (d *Dashboard) Deactive() {
	d.active = int(inactive)
}

/* ───────── Display Output  ───────── */

func (d *Dashboard) DisplayOutput(pulumiStd PulumiMsg) []tea.Cmd {
	var sb strings.Builder
	sb.WriteString(d.stdDisplay)

	if strings.HasPrefix(string(pulumiStd), "@") ||
		strings.HasPrefix(string(pulumiStd), "Updating ") ||
		string(pulumiStd) == "." ||
		string(pulumiStd) == "Outputs:" ||
		string(pulumiStd) == "Resources:" {
		sb.WriteString(stylePinkB.Render(string(pulumiStd)))
	} else {
		sb.WriteString(styleAct.Render(string(pulumiStd)))
	}

	if string(pulumiStd) != "." && !strings.HasSuffix(string(pulumiStd), "..") {
		sb.WriteString("\n")
	}

	if strings.EqualFold(string(pulumiStd), "Reader reached end of pipe.") ||
		strings.HasPrefix(string(pulumiStd), "Error reading from pipe:") {
	}

	d.stdDisplay = sb.String()

	return []tea.Cmd{WaitForPulumiResponses(d.sub)}
}

/* ───────── Handle Persific Msg  ───────── */

func (d *Dashboard) HandleDeployMsg(key DeloyMsg) []tea.Cmd {

	return []tea.Cmd{
		ExecutePulumi(key.destroy, key.campiagnName, d.sub),
		WaitForPulumiResponses(d.sub),
	}
}

/* ───────── Display Mode Form or ViewPort  ───────── */

func (d *Dashboard) SetMode(mode int) {
	d.mode = mode
}
