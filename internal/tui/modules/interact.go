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

/* ───────── Create Campaign Form ───────── */

type InteractApplication struct {
	title, desc   string
	mode          int
	active        int
	table         table.Model
	instanceTable table.Model
	session       sessionState

	windowWidth  int
	windowHeight int

	selAwsStackDeploy *deploy.AwsPulumiDeployer
}

const (
	interactTableSession sessionState = iota
	selectedCampaignSession
)

func (e InteractApplication) Title() string       { return e.title }
func (e InteractApplication) Description() string { return e.desc }
func (e InteractApplication) FilterValue() string { return e.title }
func (e InteractApplication) Mode() int           { return e.mode }
func (e InteractApplication) TotalFields() int    { return 0 }

func refreshInteractTable(width int, height int) table.Model {
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

		if awsStack.Status == deploy.DeployedState {
			row := []string{campaignName, awsStack.Status, awsStack.CloudProvider, strings.Join(awsStack.ToolsInstalled, ",")}
			rows = append(rows, row)
		}

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

func (i *InteractApplication) RefreshTable() {
	i.table = refreshInteractTable(i.windowWidth, i.windowHeight)
}

func InitInteractApplication() *InteractApplication {

	return &InteractApplication{
		title:   "Interact",
		desc:    "Trigger tool-based functionalities.",
		mode:    int(ModeForm),
		active:  int(inactive),
		table:   refreshInteractTable(100, 100),
		session: interactTableSession,
	}
}

func (i *InteractApplication) ViewForms() string {
	style := styleDim
	tstyle := titleStyleDim
	if i.active == int(active) {
		tstyle = titleStyle
		style = styleAct
	}

	var b strings.Builder
	b.WriteString(tstyle.Render("Interact") + "\n\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("‾", i.windowWidth)) + "\n")

	if i.session == interactTableSession {
		b.WriteString(style.Render("Campaigns") + "\n\n")
		b.WriteString(style.Render(i.table.View()) + "\n\n")
		b.WriteString(styleDim.Render("↑/↓ fields · 'r' Refresh Table · 'Enter' Select"))
	} else if i.session == selectedCampaignSession {
		b.WriteString(style.Render("EC2 Instances") + "\n\n")
		b.WriteString(style.Render(i.instanceTable.View()) + "\n\n")
		b.WriteString(styleDim.Render("↑/↓ fields · 'b' Back · 'Enter' Select"))
	}

	return b.String()

}

func (i *InteractApplication) UpdateForm(key tea.KeyMsg) []tea.Cmd {

	switch key.String() {
	case "enter":
		switch i.session {
		case interactTableSession:
			i.selAwsStackDeploy = deploy.NewAwsPulumiDeployer()
			i.selAwsStackDeploy.ReadConfig(i.table.SelectedRow()[0])

			columns := []table.Column{
				{Title: "Ec2Name", Width: cast.ToInt(0.2 * float64(i.windowWidth))},
				{Title: "Public Address", Width: cast.ToInt(0.2 * float64(i.windowWidth))},
				{Title: "Operating System", Width: cast.ToInt(0.2 * float64(i.windowWidth))},
				{Title: "Tools Installed", Width: cast.ToInt(0.4*float64(i.windowWidth) - 8)},
			}

			rows := []table.Row{}

			for ec2Name, ec2Config := range i.selAwsStackDeploy.Ec2Config {

				row := []string{ec2Name, ec2Config.PublicIpAddress, ec2Config.AmiOperatingSystem, strings.Join(i.selAwsStackDeploy.ToolsInstalled, ",")}
				rows = append(rows, row)
			}

			t := table.New(
				table.WithColumns(columns),
				table.WithRows(rows),
				table.WithFocused(true),
				table.WithHeight(5),
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

			i.instanceTable = t

			i.session = selectedCampaignSession
		case selectedCampaignSession:
			i.selAwsStackDeploy.ConnectSSHSpawn(i.instanceTable.SelectedRow()[0])
		}
	case "r":
		i.RefreshTable()
	case "b":
		i.session = interactTableSession
	}

	return nil
}

func (i *InteractApplication) FocusRight() {
	i.active = int(active)

}

func (i *InteractApplication) Deactive() {
	i.active = int(inactive)
}

/* ───────── Display Output  ───────── */

func (i *InteractApplication) DisplayOutput(pulumiStd PulumiMsg) []tea.Cmd {

	return nil
}
func (i *InteractApplication) HandleDeployMsg(msg DeloyMsg) []tea.Cmd {
	return nil
}

/* ───────── SSH Terminal Output  ───────── */

func (i *InteractApplication) DisplaySSHOutput(sshStd SshMsg) []tea.Cmd {

	return nil
}

/* ───────── Display Mode Form or ViewPort  ───────── */

func (i *InteractApplication) SetMode(mode int) {
	i.mode = mode
}

/* ───────── Reset Forms  ───────── */

func (i *InteractApplication) Reset() {

	i.mode = int(ModeForm)
}

func (i *InteractApplication) SetSize(width int, height int) {
	i.windowWidth = width
	i.windowHeight = height
}
