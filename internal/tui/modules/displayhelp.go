package modules

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

/* ───────── Form Inputs ───────── */

type formDHField int

const (
	fieldDHampaignName formDHField = iota
	fieldDHAwsresource
	fieldDHOperatingsystem
	fieldDHSubmit
	totalDHFields
)

/* ───────── Create Campaign Form ───────── */

type DisplayHelp struct {
	title, desc string
	mode        int
	active      int

	renderer     *glamour.TermRenderer
	windowWidth  int
	windowHeight int
}

func (dh DisplayHelp) Title() string       { return dh.title }
func (dh DisplayHelp) Description() string { return dh.desc }
func (dh DisplayHelp) FilterValue() string { return dh.title }
func (dh DisplayHelp) Mode() int           { return dh.mode }
func (dh DisplayHelp) TotalFields() int    { return int(totalDHFields) }

const content = `

Welcome to the **Dashboard Menu** guide. This section explains how to view, manage, and configure your campaigns, along with descriptions of key tools available in the deployment environment.

---

## Dashboard Overview

The **Dashboard Menu** provides a centralized view of all campaigns you have created using the **Create Campaign** menu.  

Within the dashboard, you can:
- **Select** a campaign to review its configuration details.  
- **View Settings** such as the selected provider, region, and deployment parameters.  
- **Delete or Destroy Resources** by using the 'x' hotkey.  
  > This action permanently removes all associated **AWS resources** linked to the selected campaign to prevent unnecessary cloud costs.
- **Deploy Resources** by using the 'd' hotkey to deploy the selected campaign to cloud. 

---

## Create Campaign Configuration

The **Create Campaign** form guides users through setting up a predefined infrastructure stack.  
Each campaign is configured with pre-built tools that help simulate or monitor real-world deployment scenarios.  

By completing this form, you can define:
- Cloud provider (e.g., AWS)  
- Cloud region (default: ap-southeast-1)
- Operating systems (Ubuntu, Windows)
- HoneyPot (e.g., Cowrie (Telnet/SSH), Galah (HTTP/HTTPS))
- Which IP to whitelist for initial setting up.

---

## Available Tools

### Cowrie Tool
The **Cowrie Tool** installs an SSH honeypot that emulates a real Telnet / SSH service.  
This allows real-time observation of unauthorized access attempts, including:
- **Heatmap analysis** of attacher's origins
- **Command analysis** of attacker behavior  
- **Malware sample collection** uploaded by intruders  
- **Usernames and Password" collection to increase wordlist knowledge base.

Cowrie provides an excellent way to study attacker techniques in a controlled environment.

---

### Galah Tool (AI LLM)
The **Galah Tool** installs an Web Application honeypot that emulates a real Web Application Server.  
The Galah HoneyPot leverages on AI LLM for dynamic generation of HTTP responses relevant to the requests recieved making this an ideal web HoneyPot.

This allows real-time observation of unauthorized access attempts, including:
- **Command analysis** of attacker behavior  
- **Malware sample collection** uploaded by intruders  

---

### Wazuh Agent
The **Wazuh Agent** installs Wazuh Agent on the honeypot instance.
It offers:
- Real-time alerting and event collection  
- Log forwarding to a centralized **Wazuh Manager Server**  
- **YARA-based detection** across key directories to identify and capture malicious samples  
- The default setting for Wazuh Agent's Wazuh Manager is configured to 127.0.0.1 which uses TCP 1514, 1515 and 55000.

Limitation, due to the high computing resource of hosting Wazuh Manager All-in-One stack, provisioning of Wazuh manager has been removed from this tool. 
The cost saving approach is to setup a local Linux Virtual Machine and install Wazuh Manager locally in your VM. To allow the EC2's cloud instance Wazuh Agent to connect with your VM, execute
the "-m interact -a wazuh:sshReverseTunnel -c campaign_name -i ec2_instance_name". This tunnel links the remote EC2 localhost:1514,1515,55000 to your VM 1514,1515,55000.

---

## Best Practices
- Regularly **destroy unused campaigns** to avoid unnecessary cloud costs.  
- Monitor logs via the Wazuh interface to identify abnormal activities.  
- Always **validate security group rules** before exposing honeypot environments publicly.  

`

func InitDisplayHelp() *DisplayHelp {

	return &DisplayHelp{
		title:  "Display Help",
		desc:   "Show instructions",
		active: int(inactive),
	}
}

func (dh *DisplayHelp) ViewForms() string {

	tStyle := titleStyleDim
	if dh.active == int(active) {
		tStyle = titleStyle
	}

	str, _ := dh.renderer.Render(content)

	var b strings.Builder
	b.WriteString(tStyle.Render("Dashboard & Campaign Management Help") + "\n\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("‾", dh.windowWidth)) + "\n")
	b.WriteString(str)
	b.WriteString(styleDim.Render("↑/↓ fields · Enter submit · ←/→ panes"))
	return b.String()

}

func (dh *DisplayHelp) UpdateForm(key tea.KeyMsg) []tea.Cmd {
	return nil
}
func (dh *DisplayHelp) focusFormField(idx formDHField) tea.Cmd {
	return nil
}

func (dh *DisplayHelp) FocusRight() {
	dh.active = int(active)

}

func (dh *DisplayHelp) Deactive() {
	dh.active = int(inactive)
}

func (dh *DisplayHelp) SetSize(width int, height int) {
	dh.windowWidth = width
	dh.windowHeight = height

	rendererInstance, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(dh.windowWidth),
	)

	dh.renderer = rendererInstance
}

/* ───────── Display Output  ───────── */

func (dh *DisplayHelp) DisplayOutput(pulumiStd PulumiMsg) []tea.Cmd {

	return nil
}
func (dh *DisplayHelp) HandleDeployMsg(msg DeloyMsg) []tea.Cmd {
	return nil
}

/* ───────── Display Mode Form or ViewPort  ───────── */

func (dh *DisplayHelp) SetMode(mode int) {
	dh.mode = mode
}

/* ───────── Reset Forms  ───────── */

func (dh *DisplayHelp) Reset() {
	dh.mode = int(ModeForm)
}
