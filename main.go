package main

import (
	"HoneyOps/cloud/aws/deploy"
	"HoneyOps/cloud/common/provider"
	"HoneyOps/common"
	"HoneyOps/internal/tui"
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/akamensky/argparse"
	tea "github.com/charmbracelet/bubbletea"
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

var modes = []string{"config", "deploy", "destroy", "interact", "list", "tui"}
var actions = []string{"rdp", "ssh", "collectevidence", "cowrie:randomize", "cowrie:watchlogs", "galah:watchlogs", "report", "yara:gitclonerulesfrom", "openfirewall", "closefirewall"}
var toolsList = []string{"n.a", "cowrie", "galah"}
var clouds = []string{"aws"}
var awsregions = []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2", "af-south-1", "ap-east-1", "ap-south-2", "ap-southeast-3", "ap-southeast-5", "ap-southeast-4", "ap-south-1", "ap-southeast-6", "ap-northeast-3", "ap-northeast-2", "ap-southeast-1", "ap-southeast-2", "ap-east-2", "ap-southeast-7", "ap-northeast-1", "ca-central-1", "ca-west-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-south-1", "eu-west-3", "eu-south-2", "eu-north-1", "eu-central-2", "il-central-1", "mx-central-1", "me-south-1", "me-central-1", "sa-east-1"}

func main() {

	// Create new parser object
	parser := argparse.NewParser("HoneyOps", "HoneyPot Platform Management Framework")
	// Create string flag
	mode := parser.Selector("m", "mode", modes, &argparse.Options{Required: true, Help: fmt.Sprintf("%s configure to manage campaign, deploy to make changes to cloud, destroy to tear down cloud infra, interact with tools", strings.Join(modes, "/"))})
	campaignName := parser.String("c", "campaign", &argparse.Options{Required: false, Help: "Name to track HoneyPot campaign.", Default: ""})
	cloudprovider := parser.Selector("p", "cloudprovider", []string{"aws"}, &argparse.Options{Required: false, Help: "aws (Currently only aws supported).", Default: "aws"})
	awsregion := parser.Selector("r", "\regions", awsregions, &argparse.Options{Required: false, Help: fmt.Sprintf("Default AWS region is: ap-southeast-1, if a different region is needed, use '-r ap-southeast-2' to change. Regions are %s.", strings.Join(awsregions, "/")), Default: "ap-southeast-1"})
	operatingsystem := parser.List("o", "operatingsystem", &argparse.Options{Required: false, Help: "ubuntu/windows"})
	whitelistIP := parser.String("w", "whitelist", &argparse.Options{Required: false, Help: "Enter an IPv4 address to whitelist for accessing the cloud instance.", Default: "auto-current"})
	tools := parser.List("t", "tools", &argparse.Options{Required: false, Help: fmt.Sprintf("%s -t n.a (none), -t cowrie (SSH / Telnet HoneyPot), -t galah (HTTP) -t wazuhagent", strings.Join(toolsList, "/")), Default: []string{"n.a"}})
	action := parser.String("a", "action", &argparse.Options{Required: false, Help: strings.Join(actions, "/"), Default: "none"})
	instanceName := parser.String("i", "instancename", &argparse.Options{Required: false, Help: "Enter name of cloud instance. For example (1-ubuntu).", Default: "none"})
	args := parser.String("n", "arguments", &argparse.Options{Required: false, Help: "Additional arguments for 'openfirewall'. Opening port 22 to public: -a openfirewall -n tcp/22 ", Default: "none"})

	// Parse input
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}
	// Finally print the collected string

	provider.InitPulumi()

	switch *mode {
	case "tui":
		// Run Bubbletea of Tui interface
		model, err := tui.New()

		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseAllMotion())
		if _, err = p.Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}

	case "list":
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Select components to list: ")
		fmt.Println("  1. modes (-m) ")
		fmt.Println("  2. tools (-t) ")
		fmt.Println("  3. interactions (-a) ")
		fmt.Println("  4. campaigns (-c) ")
		fmt.Println("\n")
		fmt.Println("Enter index of choice (1,2,3,4): ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println("\nModes:")
			fmt.Printf("  %s\n", strings.Join(modes, "\n  "))
		case "2":
			fmt.Println("\nTools:")
			fmt.Printf("  %s\n", strings.Join(toolsList, "\n  "))
		case "3":
			fmt.Println("\nActions:")
			fmt.Printf("  %s\n", strings.Join(actions, "\n  "))
		case "4":
			filepath.Walk(common.GetHoneyOpsCampaignDir(), func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					// Skipping Empty Campaign names
					if info.Name() != "HoneyCloud_.yaml" {
						campaignName := strings.Replace(info.Name(), "HoneyCloud_", "", 1)
						campaignName = strings.Replace(campaignName, ".yaml", "", 1)
						awsConfigReader := deploy.NewAwsPulumiDeployer()
						awsConfigReader.ReadConfig(campaignName)
						fmt.Printf("\nCampaign Name (%s): %s\n", awsConfigReader.Status, campaignName)

						fmt.Println("  EC2 Instance Names:")
						for idx, ec2Cfg := range awsConfigReader.Ec2Config {
							fmt.Printf("    %s(%s)\n", idx, ec2Cfg.PublicIpAddress)
							fmt.Printf("        Tools: %s\n", strings.Join(ec2Cfg.Tools, ","))
							fmt.Printf("        Public Open Firewall Rules (ingress):\n")
							for _, ingress := range ec2Cfg.SecurityGroup[0].IngressRules {
								fmt.Printf("            - %s (%s/%d)\n", ingress.CidrIpv4, ingress.Protocol, ingress.SrcPort)
							}

						}

					}

				}
				return nil
			})
		case "5":

		}

	case "config":
		if *campaignName == "" {
			fmt.Println("Enter campaign name (-c <campaign>) to proceed configuration.")
			return
		}

		common.GetHoneyOpsCampaignDir()
		campaignConfig := filepath.Join(common.GetHoneyOpsCampaignDir(), fmt.Sprintf("HoneyCloud_%s.yaml", *campaignName))

		// Attempt to check campaignname was previous created before.
		_, err := os.Stat(campaignConfig)

		// If config file do not exist, proceed to create a new compaign config
		if err != nil && errors.Is(err, os.ErrNotExist) {
			configPath, err := deploy.WriteCampaignConfig(*campaignName, *operatingsystem, *whitelistIP, *tools, *cloudprovider, *awsregion)
			if err != nil {
				fmt.Printf("Error occured when writing campaign config %s.yaml file.\nError Message: %s\n", *campaignName, err)
			}
			fmt.Printf("@Campaign Configuration Successful Written to: %s\n", configPath)
		} else {
			// Means an existing config exist
			// Not yet fully tested on how the config overwriting works
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Do you want to overwrite existing campaign config(y/n) ")
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)

			switch strings.ToLower(choice) {
			case "y":
				configPath, err := deploy.WriteCampaignConfig(*campaignName, *operatingsystem, *whitelistIP, *tools, *cloudprovider, *awsregion)
				if err != nil {
					fmt.Printf("Error occured when writing campaign config %s.yaml file.\nError Message: %s\n", *campaignName, err)
				}
				fmt.Printf("@Campaign Configuration Successful Written to: %s\n", configPath)
			case "n":
				fmt.Print("Do you want to update existing EC2 Instance's Tools(y/n) ")
				choice, _ := reader.ReadString('\n')
				choice = strings.TrimSpace(choice)
				switch strings.ToLower(choice) {
				case "y":
					ec2Name := *instanceName
					if strings.EqualFold(*instanceName, "none") {
						fmt.Print("Specify the instance name: ")
						ec2Name, _ = reader.ReadString('\n')
						ec2Name = strings.TrimSpace(ec2Name)
					}
					awsStack := deploy.NewAwsPulumiDeployer()
					awsStack.ReadConfig(*campaignName)
					awsStack.UpdateConfigEC2Tools(*campaignName, ec2Name, *tools)
					fmt.Printf("Campaign[%s][%s] tools have been updated to [%s].\n", *campaignName, *instanceName, strings.Join(*tools, ","))
					fmt.Printf("Run -m deploy -c %s to push updates to cloud.\n", *campaignName)
				}
			}

		}

	case "deploy":
		if *campaignName == "" {
			fmt.Println("Enter campaign name (-c <campaign>) to proceed deployment.")
			return
		}
		awsStack := deploy.NewAwsPulumiDeployer()
		awsStack.ReadConfig(*campaignName)
		if stringInSlice("galah", awsStack.ToolsInstalled) {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter Your LLM API Key: ")
			llmApiKey, _ := reader.ReadString('\n')
			awsStack.LLMApiKey = strings.TrimSpace(llmApiKey)
		}
		awsStack.Up(false, *campaignName)
	case "destroy":
		if *campaignName == "" {
			fmt.Println("Enter campaign name (-c <campaign>) to proceed destroying resouces.")
			return
		}
		awsStack := deploy.NewAwsPulumiDeployer()
		awsStack.Up(true, *campaignName)

	case "interact":
		if *campaignName == "" {
			fmt.Println("Enter campaign name (-c <campaign>) to interact.")
			return
		}
		awsStack := deploy.NewAwsPulumiDeployer()
		awsStack.ReadConfig(*campaignName)
		switch *action {
		case "rdp":
			awsStack.ConnectRDP(*instanceName)
		case "psexec":
			awsStack.ConnectPsExec(*instanceName)
		case "ssh":
			awsStack.ConnectSSH(*instanceName)
		case "openfirewall", "closefirewall":
			if strings.EqualFold(*args, "none") {
				fmt.Println("Missing argument '-n'. Example, if you intend to open port 22 to public, the command is:\n" +
					"-a openfirewall -n tcp/22")
				return
			}

			if len(strings.Split(*args, `/`)) != 2 {
				fmt.Println("Incorrect syntax for '-n'. Example, if you intend to open port 22 to public, the command is:\n" +
					"-a openfirewall -n tcp/22")
				return
			}

			protocol := strings.ToLower(strings.Split(*args, `/`)[0])
			portString := strings.Split(*args, `/`)[1]

			port, portCheck := strconv.Atoi(portString)
			if !(strings.EqualFold(protocol, "tcp") || strings.EqualFold(protocol, "udp")) && portCheck != nil {
				fmt.Println("Incorrect syntax for '-n'. Example, if you intend to open port 22 to public, the command is:\n" +
					"-a openfirewall -n tcp/22")
				return
			}
			if strings.EqualFold(*action, "openfirewall") {
				awsStack.OpenFirewall(*instanceName, protocol, port)
			} else {
				awsStack.CloseFirewall(*instanceName, protocol, port)
			}

			fmt.Printf("Run -m deploy -c %s to push firewall updates to cloud.\n", *campaignName)

		case "cowrie:randomize":
			awsStack.RandomizeCowrieEnvironment(*instanceName)
		case "cowrie:watchlogs":
			awsStack.WatchLogs(*instanceName, "Cowrie")
		case "galah:watchlogs":
			awsStack.WatchLogs(*instanceName, "Galah")
		case "yara:gitclonerulesfrom":
			awsStack.GitCloneYaraRules(*instanceName, *args)
		case "collectevidence":
			awsStack.CollectEvidencePack(*instanceName)
		case "report":
			awsStack.GenerateReport(*instanceName)
		case "windows:getpassword":
			fmt.Println(awsStack.GetWindowsPasswordData(*instanceName))
		case "wazuh:sshReverseTunnel":
			awsStack.ConnectEstablishWazuhSSHTunnel(*instanceName)
		}

	}

	/*

	 */

}
