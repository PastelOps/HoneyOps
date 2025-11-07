package modules

import (
	"HoneyOps/cloud/aws/deploy"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

/* ───────── Form Inputs ───────── */

type ccformField int

const (
	fieldCCCampaignName ccformField = iota
	fieldCCProvider
	fieldCCRegion
	fieldCCOperatingsystem
	fieldCCCurrentIP
	fieldCCInstallModules
	fieldCCSave
	totalCCFields
)

type sessionState int

const (
	createSession sessionState = iota
	deploySession
	pulumiDeploySession
)

/* ───────── Create Campaign Form ───────── */

type CreateCampaign struct {
	title, desc string
	mode        int
	formSel     ccformField
	formInputs  [totalCCFields]textinput.Model

	deployCursor  int
	deployChoice  string
	deployChoices []string

	active        int
	success       bool
	outputDisplay string

	stdDisplay string
	sub        chan string

	state      sessionState
	autoValues map[ccformField]autocomplete

	windowWidth  int
	windowHeight int
}

func (cf CreateCampaign) Title() string       { return cf.title }
func (cf CreateCampaign) Description() string { return cf.desc }
func (cf CreateCampaign) FilterValue() string { return cf.title }
func (cf CreateCampaign) Mode() int           { return cf.mode }
func (cf CreateCampaign) TotalFields() int    { return int(totalCCFields) }

type autocomplete struct {
	Values     []string
	AutoChoice int
}

func InitCreateCampaign() *CreateCampaign {

	var i [totalCCFields]textinput.Model

	newTI := func(ph string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.Prompt = stylePink.Render(" > ")
		ti.TextStyle = stylePink
		return ti
	}

	i[fieldCCCampaignName] = newTI("Name your campaign")
	i[fieldCCProvider] = newTI("AWS")
	i[fieldCCRegion] = newTI("AWS-Region: default is ap-southeast-1")
	i[fieldCCOperatingsystem] = newTI("Ubuntu/Windows")
	i[fieldCCCurrentIP] = newTI("auto-current")
	i[fieldCCInstallModules] = newTI("Cowrie/Galah/WazuhAgent")

	av := make(map[ccformField]autocomplete)
	av[fieldCCProvider] = autocomplete{Values: []string{"AWS"}, AutoChoice: 0}
	av[fieldCCRegion] = autocomplete{Values: []string{"ap-southeast-1", "ap-southeast-2", "us-east-1", "us-east-2", "us-west-1", "us-west-2", "af-south-1", "ap-east-1", "ap-south-2", "ap-southeast-3", "ap-southeast-5", "ap-southeast-4", "ap-south-1", "ap-southeast-6", "ap-northeast-3", "ap-northeast-2", "ap-east-2", "ap-southeast-7", "ap-northeast-1", "ca-central-1", "ca-west-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-south-1", "eu-west-3", "eu-south-2", "eu-north-1", "eu-central-2", "il-central-1", "mx-central-1", "me-south-1", "me-central-1", "sa-east-1"}, AutoChoice: 0}
	av[fieldCCOperatingsystem] = autocomplete{Values: []string{"Ubuntu", "Windows"}, AutoChoice: 0}
	av[fieldCCCurrentIP] = autocomplete{Values: []string{"auto-current"}, AutoChoice: 0}
	av[fieldCCInstallModules] = autocomplete{Values: []string{"N/A", "Cowrie", "Galah", "WazuhAgent"}, AutoChoice: 0}

	return &CreateCampaign{
		title:         "Create Campaign",
		desc:          "Start a new deployment or stack",
		mode:          int(ModeForm),
		formSel:       0,
		formInputs:    i,
		deployChoices: []string{"Yes", "No"},
		state:         createSession,
		active:        int(inactive),
		success:       false,
		sub:           make(chan string),
		autoValues:    av,
	}
}

func (cf *CreateCampaign) ViewForms() string {

	baseStyle := styleDim
	tStyle := titleStyleDim
	resultStyle := styleDim

	if cf.active == int(active) {
		baseStyle = styleAct
		tStyle = titleStyle

		if cf.success {
			resultStyle = styleSuccess
		} else {
			resultStyle = styleError
		}
	}

	labels := []string{
		"Campaign / Stack Name",
		"Cloud Provider",
		"AWS Region",
		"Operating System",
		"IP Address To Whitelist (auto-current = Current IP Address)",
		"Preset Tools To Install (input multiple by using comma ',')",
		"Save Campaign Configuration",
	}

	var b strings.Builder

	switch cf.state {
	case deploySession:
		b.WriteString(tStyle.Bold(true).Render("Deploy Campaign") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", cf.windowWidth)) + "\n")

		b.WriteString(baseStyle.Render("Configuration written : "))
		b.WriteString(resultStyle.Render(cf.outputDisplay))
		b.WriteString(baseStyle.Render("\n\n"))
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", cf.windowWidth)) + "\n")

		if cf.success {
			b.WriteString(baseStyle.Render("Deploy selected resources to the cloud now?"))
			b.WriteString(baseStyle.Render("\n\n"))
			for i := 0; i < len(cf.deployChoices); i++ {

				if cf.deployCursor == i {
					b.WriteString(styleSel.Render(cf.deployChoices[i]))
				} else {
					b.WriteString(baseStyle.Render(cf.deployChoices[i]))
				}

				b.WriteString("   ")
			}
			b.WriteString("\n\n")
		}
		b.WriteString(styleDim.Render("\n\nEsc Back · ←/→ choices · Enter submit"))
	case createSession:
		b.WriteString(tStyle.Bold(true).Render("Enter New Campaign Details") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", 50)) + "\n")
		for i := 0; i < int(totalCCFields); i++ {
			label := labels[i]
			if i == int(cf.formSel) {
				if cf.active == int(inactive) {
					label = baseStyle.Render(label)
				} else if cf.formSel == fieldCCSave {
					label = styleSel.Render(label)
				} else {
					label = stylePurple.Render(label)
				}
			} else {
				label = baseStyle.Render(label)
			}
			if i == int(fieldCCSave) {
				b.WriteString(label + "\n\n")
			} else {
				b.WriteString(label + "\n" + cf.formInputs[i].View() + "\n\n")
			}
		}
		b.WriteString(styleDim.Render("\n\nEsc Back · ↑/↓ fields · Tab autocomplete · Enter submit"))
	default:
		b.WriteString(titleStyle.Bold(true).Render("Executing Deployment") + "\n\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("‾", 50)) + "\n")

		b.WriteString(cf.stdDisplay)
	}

	return b.String()

}

func (cf *CreateCampaign) UpdateForm(key tea.KeyMsg) []tea.Cmd {

	// After Creating a Campaign Configuration

	if cf.state == deploySession {
		switch key.String() {
		case "up", "left":
			cf.deployCursor--
			if cf.deployCursor < 0 {
				cf.deployCursor = len(cf.deployChoices) - 1
			}
		case "down", "right":
			cf.deployCursor++
			if cf.deployCursor >= len(cf.deployChoices) {
				cf.deployCursor = 0
			}
		case "enter":
			cf.deployChoice = cf.deployChoices[cf.deployCursor]
			if cf.deployChoice == "Yes" {

				// Go to pulumi output state
				cf.state = pulumiDeploySession
				return []tea.Cmd{func() tea.Msg {
					return DeloyMsg{
						campiagnName: cf.formInputs[fieldCCCampaignName].Value(),
						destroy:      false,
					}
				}}

			}
			return cf.Reset()

		}

	} else if cf.state == createSession {

		switch key.String() {
		case "up":
			if cf.formSel > 0 {
				return []tea.Cmd{cf.focusFormField(cf.formSel - 1)}
			}
		case "down":
			if cf.formSel < totalCCFields-1 {
				return []tea.Cmd{cf.focusFormField(cf.formSel + 1)}
			}
		case "enter":
			if cf.formSel < fieldCCSave {
				return []tea.Cmd{cf.focusFormField(cf.formSel + 1)}
			}
			if cf.formSel == fieldCCSave {

				tools := strings.Split(cf.formInputs[fieldCCOperatingsystem].Value(), ",")

				_, err := deploy.WriteCampaignConfig(
					cf.formInputs[fieldCCCampaignName].Value(),
					tools,
					cf.formInputs[fieldCCCurrentIP].Value(),
					[]string{cf.formInputs[fieldCCInstallModules].Value()},
					cf.formInputs[fieldCCProvider].Value(),
					cf.formInputs[fieldCCRegion].Value(),
				)

				// Go to deployment state
				cf.state++

				if err == nil {
					cf.success = true
					cf.outputDisplay = "Success"
				} else {
					cf.success = false
					cf.outputDisplay = fmt.Sprintf("Failed - %v", err)
				}
			}
			cf.FocusRight()
		case "tab":
			val, ok := cf.autoValues[cf.formSel]
			if ok {
				cf.formInputs[cf.formSel].SetValue(val.Values[val.AutoChoice])
				val.AutoChoice = (val.AutoChoice + 1) % len(val.Values)
				cf.autoValues[cf.formSel] = val
			}

		}

		if cf.formSel != fieldCCSave {
			var cmd tea.Cmd
			cf.formInputs[cf.formSel], cmd = cf.formInputs[cf.formSel].Update(key)
			return []tea.Cmd{cmd}
		}
	} else {
		switch key.String() {
		case "b":
			return cf.Reset()
		}
	}

	return nil
}
func (cf *CreateCampaign) focusFormField(idx ccformField) tea.Cmd {
	if idx < 0 {
		idx = 0
	}
	if idx >= totalCCFields {
		idx = totalCCFields - 1
	}
	if cf.formSel != fieldCCSave {
		cf.formInputs[cf.formSel].Blur()
	}

	cf.formSel = idx
	if cf.formSel != fieldCCSave {
		cf.formInputs[cf.formSel].Focus()
		return textinput.Blink
	}

	return nil
}

func (cf *CreateCampaign) FocusRight() {
	cf.active = int(active)

	switch cf.mode {
	case int(ModeForm):
		for i := range cf.formInputs {
			cf.formInputs[i].Blur()
		}
		if cf.formSel != fieldCCSave {
			cf.formInputs[cf.formSel].Focus()
		}
	case int(ModeDisplay):
		//m.chatInput.Focus()
	}
}

func (cf *CreateCampaign) Deactive() {
	cf.active = int(inactive)
}

func (cf *CreateCampaign) SetSize(width int, height int) {
	cf.windowWidth = width
	cf.windowHeight = height
}

/* ───────── Display Output  ───────── */

func (cf *CreateCampaign) DisplayOutput(pulumiStd PulumiMsg) []tea.Cmd {
	var sb strings.Builder
	sb.WriteString(cf.stdDisplay)

	if strings.HasPrefix(string(pulumiStd), "@") ||
		strings.HasPrefix(string(pulumiStd), "Updating ") ||
		string(pulumiStd) == "." ||
		string(pulumiStd) == "Outputs:" ||
		string(pulumiStd) == "Resources:" {
		sb.WriteString(stylePinkB.Render(string(pulumiStd)))
	} else {
		sb.WriteString(styleAct.Render(string(pulumiStd)))
	}

	if strings.EqualFold(string(pulumiStd), "Reader reached end of pipe.") ||
		strings.HasPrefix(string(pulumiStd), "Error reading from pipe:") {
		sb.WriteString(styleDim.Render("\n\n'b' Back"))
	}

	cf.stdDisplay = sb.String()

	return []tea.Cmd{WaitForPulumiResponses(cf.sub)}
}

/* ───────── Handle Persific Msg  ───────── */

func (cf *CreateCampaign) HandleDeployMsg(key DeloyMsg) []tea.Cmd {

	return []tea.Cmd{
		ExecutePulumi(false, key.campiagnName, cf.sub),
		WaitForPulumiResponses(cf.sub),
	}
}

/* ───────── Reset Forms  ───────── */

func (cf *CreateCampaign) Reset() []tea.Cmd {
	for i := 0; i < int(totalCCFields); i++ {
		cf.formInputs[i].Reset()
	}
	cf.outputDisplay = ""
	cf.formSel = fieldCCCampaignName
	cf.mode = int(ModeForm)
	cf.state = createSession
	cf.Deactive()

	return []tea.Cmd{func() tea.Msg {
		return BackMsg{}
	}}
}

func (cf *CreateCampaign) SetMode(mode int) {
	cf.mode = mode
}
