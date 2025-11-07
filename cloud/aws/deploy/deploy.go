package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"HoneyOps/cloud/common/provider"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	UnDeployedState = "Undeployed"
	DeployedState   = "Deployed"
	DestroyedState  = "Destroyed"
)

type AwsPulumiDeployer struct {
	CampaignStack  string
	ConfigName     string
	ConfigType     string
	VpcConfig      map[string]AwsVpcConfig
	Ec2Config      map[string]AwsEc2Config
	Status         string
	ToolsInstalled []string

	privateSSHKeysGlobal     map[string]pulumi.StringOutput
	privateSSHKeysPathGlobal map[string]string
	ec2KeyPairs              map[string]*ec2.KeyPair

	publicSubnet  pulumi.StringPtrInput
	privateSubnet pulumi.StringPtrInput

	privateEc2KeyPath string
	privateEc2PemPath string
	ec2Instances      map[string]*ec2.Instance

	CloudProvider string
	CloudRegion   string

	LLMApiKey string
}

func NewAwsPulumiDeployer() *AwsPulumiDeployer {
	return &AwsPulumiDeployer{
		ConfigType:               "yaml",
		ConfigName:               "HoneyCloud",
		VpcConfig:                make(map[string]AwsVpcConfig),
		privateSSHKeysGlobal:     make(map[string]pulumi.StringOutput),
		privateSSHKeysPathGlobal: make(map[string]string),
		ec2KeyPairs:              make(map[string]*ec2.KeyPair),
		ec2Instances:             make(map[string]*ec2.Instance),
		Status:                   UnDeployedState,
		LLMApiKey:                "",
	}
}

func (a *AwsPulumiDeployer) Deploy(ctx *pulumi.Context) (err error) {

	a.InitSSHKeys(ctx)

	a.createVPC(ctx)
	a.createEC2(ctx)

	for ec2Name, ec2Instance := range a.Ec2Config {
		if strings.EqualFold(ec2Instance.AmiOperatingSystem, "Ubuntu") {
			connectionSetup, err := a.SetupRemoteConnectionEC2(ctx, ec2Name, a.ec2Instances[ec2Name])
			yaraCmd, yaraMonitorCmd, _ := a.SetupYara(ctx, ec2Name, a.ec2Instances[ec2Name], connectionSetup)

			if err != nil {
				return err
			}

			for _, toolToInstall := range ec2Instance.Tools {

				switch strings.ToLower(toolToInstall) {
				case "cowrie":
					a.SetupCowrie(ctx, ec2Name, a.ec2Instances[ec2Name], connectionSetup, yaraCmd, yaraMonitorCmd)
				case "galah":
					a.SetupGalahLLMPot(ctx, ec2Name, a.ec2Instances[ec2Name], connectionSetup, yaraCmd, yaraMonitorCmd)
				case "wazuhagent":
					a.SetupWazuhAgent(ctx, ec2Name, a.ec2Instances[ec2Name], connectionSetup, yaraCmd, yaraMonitorCmd)
				}

			}
		}

		/*  // This section experimential
		}else if strings.EqualFold(ec2Instance.AmiOperatingSystem, "Windows") {

		} else if strings.EqualFold(ec2Instance.AmiOperatingSystem, "WazuhManager") {
			_, err := a.SetupRemoteConnectionWazuhManager(ctx, ec2Name, a.ec2Instances[ec2Name])
			if err != nil {
				return err
			}
		}
		*/
	}

	//
	//a.CollectEvidencePack(ctx, a.ec2Instances["ubuntu"])

	return nil
}

func (a *AwsPulumiDeployer) Up(destroy bool, campaignName string) {

	// we're going to use the same working directory as our CLI driver.
	// doing this allows us to share the Project and Stack Settings (Pulumi.yaml and any Pulumi.<stack>.yaml files)
	workDir := provider.InitPulumi()

	ctx := context.Background()

	// we use a simple stack name here, but recommend using auto.FullyQualifiedStackName for maximum specificity.
	stackName := campaignName
	a.ReadConfig(stackName)

	// stackName := auto.FullyQualifiedStackName("myOrgOrUser", projectName, stackName)

	secretsProvider := auto.SecretsProvider("passphrase")
	envvars := auto.EnvVars(map[string]string{
		// In a real program, you would feed in the password securely or via the actual environment.
		"PULUMI_CONFIG_PASSPHRASE": "DefaultChangeThis",
	})
	stackSettings := auto.Stacks(map[string]workspace.ProjectStack{
		stackName: {SecretsProvider: "passphrase"},
	})

	// create or select an existing stack matching the given name.
	// using LocalSource sets up the workspace to use existing project/stack settings in our cli package.
	// here our inline program comes from a shared package.
	// this allows us to have both an automation program, and a manual CLI program for development sharing code.
	s, err := auto.UpsertStackLocalSource(ctx, stackName, workDir, auto.Program(a.Deploy), secretsProvider, stackSettings, envvars)
	if err != nil {
		fmt.Printf("Failed to create or select stack: %v\n", err)
		os.Exit(0)
	}

	fmt.Printf("Created/Selected stack %q\n", stackName)

	//fmt.Println("Installing the AWS plugin")

	w := s.Workspace()
	// for inline source programs, we must manage plugins ourselves
	err = w.InstallPlugin(ctx, "aws", "v4.0.0")
	if err != nil {
		fmt.Printf("Failed to install program plugins: %v\n", err)
		os.Exit(0)
	}

	//fmt.Println("Successfully installed AWS plugin")

	// set stack configuration specifying the AWS region to deploy
	s.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: "ap-southeast-1"})

	fmt.Println("Successfully set config")
	fmt.Println("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		os.Exit(0)
	}

	fmt.Println("Refresh succeeded!")

	if destroy {
		fmt.Println("Starting stack destroy")

		// wire up our destroy to stream progress to stdout
		stdoutStreamer := optdestroy.ProgressStreams(os.Stdout)

		// destroy our stack and exit early
		_, err := s.Destroy(ctx, stdoutStreamer)
		if err != nil {
			fmt.Printf("Failed to destroy stack: %v", err)
		}
		a.updateConfigValue(campaignName, "status", DestroyedState)

		fmt.Println("Stack successfully destroyed")
		os.Exit(0)
	}

	fmt.Println("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	// run the update to setup infra
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		fmt.Printf("Failed to update stack: %v\n\n", err)
		os.Exit(0)
	}

	fmt.Println("Update succeeded!")
	_ = res

	a.updateConfigValue(campaignName, "status", DeployedState)
	a.updateConfigValue(campaignName, "privatekey", a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"])
	a.updateConfigValue(campaignName, "privatepem", a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey-Pem"])

}

func (a *AwsPulumiDeployer) UpPipe(destroy bool, campaignName string, buffer *io.PipeWriter) {

	// we're going to use the same working directory as our CLI driver.
	// doing this allows us to share the Project and Stack Settings (Pulumi.yaml and any Pulumi.<stack>.yaml files)
	workDir := provider.InitPulumi()

	ctx := context.Background()

	// we use a simple stack name here, but recommend using auto.FullyQualifiedStackName for maximum specificity.
	stackName := campaignName
	a.ReadConfig(stackName)

	// stackName := auto.FullyQualifiedStackName("myOrgOrUser", projectName, stackName)

	secretsProvider := auto.SecretsProvider("passphrase")
	envvars := auto.EnvVars(map[string]string{
		// In a real program, you would feed in the password securely or via the actual environment.
		"PULUMI_CONFIG_PASSPHRASE": "DefaultChangeThis",
	})
	stackSettings := auto.Stacks(map[string]workspace.ProjectStack{
		stackName: {SecretsProvider: "passphrase"},
	})

	// create or select an existing stack matching the given name.
	// using LocalSource sets up the workspace to use existing project/stack settings in our cli package.
	// here our inline program comes from a shared package.
	// this allows us to have both an automation program, and a manual CLI program for development sharing code.
	s, err := auto.UpsertStackLocalSource(ctx, stackName, workDir, auto.Program(a.Deploy), secretsProvider, stackSettings, envvars)
	if err != nil {
		fmt.Printf("Failed to create or select stack: %v\n", err)
		return
	}

	//fmt.Printf("Created/Selected stack %q\n", stackName)

	//fmt.Println("Installing the AWS plugin")

	w := s.Workspace()
	// for inline source programs, we must manage plugins ourselves
	err = w.InstallPlugin(ctx, "aws", "v4.0.0")
	if err != nil {
		fmt.Printf("Failed to install program plugins: %v\n", err)
		return
	}

	//fmt.Println("Successfully installed AWS plugin")

	// set stack configuration specifying the AWS region to deploy
	s.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: "ap-southeast-1"})

	//fmt.Println("Successfully set config")
	//fmt.Println("Starting refresh")

	_, err = s.Refresh(ctx)
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		return
	}

	//fmt.Println("Refresh succeeded!")

	if destroy {
		//fmt.Println("Starting stack destroy")

		// wire up our destroy to stream progress to stdout
		stdoutStreamer := optdestroy.ProgressStreams(buffer) //(os.Stdout)

		// destroy our stack and exit early
		_, err := s.Destroy(ctx, stdoutStreamer)
		if err != nil {
			fmt.Printf("Failed to destroy stack: %v", err)
		}
		a.updateConfigValue(campaignName, "status", DestroyedState)
		return
		//fmt.Println("Stack successfully destroyed")
		//os.Exit(0)
	}

	//fmt.Println("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(buffer) //(os.Stdout)

	// run the update to setup infra
	res, err := s.Up(ctx, stdoutStreamer)
	if err != nil {
		fmt.Printf("Failed to update stack: %v\n\n", err)
		return
	}

	//fmt.Println("Update succeeded!")
	_ = res

	a.updateConfigValue(campaignName, "status", DeployedState)
	a.updateConfigValue(campaignName, "privatekey", a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"])
	a.updateConfigValue(campaignName, "privatepem", a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey-Pem"])
}
