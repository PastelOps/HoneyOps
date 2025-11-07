package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"HoneyOps/common"

	"github.com/spf13/viper"
)

var configName = "HoneyCloud"
var configType = "yaml"

type AwsVpcConfig struct {
	CidrBlock          string
	EnableDnsHostnames bool
	EnableDnsSupport   bool
	Subnet             []AwsSubnetConfig
	InternetGateway    bool
}

type AwsSubnetConfig struct {
	CidrBlock           string
	MapPublicIpOnLaunch bool
}

type AwsEc2Config struct {
	Index int

	AmiName            string
	AmiVersionName     string
	AmiVersionNumber   string
	AmiInstanceType    string
	AmiInstanceCpuArch string
	AmiOwnerId         string
	AmiOperatingSystem string

	OsUser                   string
	WindowsEncryptedPw       string
	PublicIpAddress          string
	AssociatePublicIpAddress bool
	KeyName                  string
	Subnet                   string
	SecurityGroup            []AwsSecurityGroup

	Tools []string
}

type AwsSecurityGroup struct {
	Vpc                      string
	SecurityGroupName        string
	SecurityGroupDescription string
	IngressRules             []AwsSecurityRules
	EgressRules              []AwsSecurityRules
}

type AwsSecurityRules struct {
	CidrIpv4 string
	SrcPort  int
	DestPort int
	Protocol string
	Name     string
}

type AwsKeyPair struct {
	KeyName   string
	Algorithm string
	LocalPath string
	PublicKey string
}

func (a *AwsPulumiDeployer) ReadConfig(stackName string) error {

	newCfg := viper.New()

	userHomeDirName, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsCampaign := "campaign"
	userHoneyOpsCampaignPath := filepath.Join(userHomeDirName, userHoneyOpsFolder, userHoneyOpsCampaign)
	if _, err := os.Stat(userHoneyOpsCampaignPath); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(userHoneyOpsCampaignPath, os.ModePerm)
	}

	newCfg.SetConfigType(a.ConfigType)
	newCfg.SetConfigName(a.ConfigName + "_" + stackName)
	newCfg.AddConfigPath(userHoneyOpsCampaignPath)

	err = newCfg.ReadInConfig()

	if err != nil {
		return err
	}

	a.CampaignStack = newCfg.GetString("campaignstack")

	if err != nil {
		return err
	}

	// Check for VPC Configurations
	err = newCfg.UnmarshalKey("vpc", &a.VpcConfig)
	if err != nil {
		return err
	}

	// Check for VPC Configurations
	err = newCfg.UnmarshalKey("ec2", &a.Ec2Config)
	if err != nil {
		return err
	}

	// Check for OpenSSH Key Configurations
	err = newCfg.UnmarshalKey("privatekey", &a.privateEc2KeyPath)
	if err != nil {
		return err
	}

	// Check for Pem Private Key Configurations
	err = newCfg.UnmarshalKey("privatepem", &a.privateEc2PemPath)
	if err != nil {
		return err
	}

	for _, ec2Instance := range a.Ec2Config {
		for _, secGrp := range ec2Instance.SecurityGroup {
			currIP, _ := common.GetPublicIP()
			for _, egressRule := range secGrp.EgressRules {
				if egressRule.CidrIpv4 == "auto-current/32" {
					egressRule.CidrIpv4 = fmt.Sprintf("%s/32", currIP)
				}
			}
			for _, ingressRule := range secGrp.IngressRules {
				if ingressRule.CidrIpv4 == "auto-current/32" {
					ingressRule.CidrIpv4 = fmt.Sprintf("%s/32", currIP)
				}
			}
		}
	}

	// Check for Deployment Status
	err = newCfg.UnmarshalKey("status", &a.Status)
	if err != nil {
		return err
	}

	// Check for What Tools Installed
	err = newCfg.UnmarshalKey("tools", &a.ToolsInstalled)
	if err != nil {
		return err
	}

	// Check for What Cloud Provider Provided
	err = newCfg.UnmarshalKey("cloudprovider", &a.CloudProvider)
	if err != nil {
		return err
	}

	// Check for What Cloud Region Provided
	err = newCfg.UnmarshalKey("cloudregion", &a.CloudRegion)
	if err != nil {
		return err
	}

	return nil
}

func (a *AwsPulumiDeployer) UpdateConfigEC2Tools(campaignName string, ec2Name string, tools []string) error {

	for name, ec2Cfg := range a.Ec2Config {
		if strings.EqualFold(name, ec2Name) {
			ec2Cfg.Tools = tools
			a.Ec2Config[name] = ec2Cfg
		}
	}
	return a.updateConfigEC2Value(campaignName, a.Ec2Config)
}

func (a *AwsPulumiDeployer) updateConfigEC2Value(campaignName string, value map[string]AwsEc2Config) error {
	userHomeDirName, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Unable to locate home directory, %v\n", err)
		return err
	}

	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsCampaign := "campaign"
	userHoneyOpsCampaignPath := filepath.Join(userHomeDirName, userHoneyOpsFolder, userHoneyOpsCampaign)
	if _, err := os.Stat(userHoneyOpsCampaignPath); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(userHoneyOpsCampaignPath, os.ModePerm)
	}

	newCfg := viper.New()

	newCfg.SetConfigType(a.ConfigType)
	newCfg.SetConfigName(a.ConfigName + "_" + campaignName)
	newCfg.AddConfigPath(userHoneyOpsCampaignPath)

	err = newCfg.ReadInConfig()

	if err != nil {
		fmt.Printf("Error reading viper config: %v \n", err)
		os.Exit(2)
	}

	newCfg.Set("ec2", value)
	err = newCfg.WriteConfigAs(filepath.Join(userHoneyOpsCampaignPath, configName+"_"+campaignName+"."+configType))

	if err != nil {
		return err
	}

	return nil

}

func (a *AwsPulumiDeployer) updateConfigValue(campaignName string, key string, value string) error {
	userHomeDirName, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Unable to locate home directory, %v\n", err)
		return err
	}

	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsCampaign := "campaign"
	userHoneyOpsCampaignPath := filepath.Join(userHomeDirName, userHoneyOpsFolder, userHoneyOpsCampaign)
	if _, err := os.Stat(userHoneyOpsCampaignPath); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(userHoneyOpsCampaignPath, os.ModePerm)
	}

	newCfg := viper.New()

	newCfg.SetConfigType(a.ConfigType)
	newCfg.SetConfigName(a.ConfigName + "_" + campaignName)
	newCfg.AddConfigPath(userHoneyOpsCampaignPath)

	err = newCfg.ReadInConfig()

	if err != nil {
		fmt.Printf("Error reading viper config: %v \n", err)
		os.Exit(2)
	}

	newCfg.Set(key, value)
	err = newCfg.WriteConfigAs(filepath.Join(userHoneyOpsCampaignPath, configName+"_"+campaignName+"."+configType))

	if err != nil {
		return err
	}

	return nil

}

func WriteCampaignConfig(campaignName string, operatingSystems []string, whitelistedIP string, tools []string, cloudProvider string, cloudgregion string) (string, error) {

	newCfg := viper.New()

	newCfg.SetConfigType(configType)
	userHomeDirName, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Unable to locate home directory, %v\n", err)
		return "", err
	}

	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsCampaign := "campaign"
	userHoneyOpsCampaignPath := filepath.Join(userHomeDirName, userHoneyOpsFolder, userHoneyOpsCampaign)
	if _, err := os.Stat(userHoneyOpsCampaignPath); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(userHoneyOpsCampaignPath, os.ModePerm)
	}

	subnetPublic := AwsSubnetConfig{
		CidrBlock:           "10.0.1.0/24",
		MapPublicIpOnLaunch: true,
	}
	subnetPrivate := AwsSubnetConfig{
		CidrBlock: "10.0.2.0/24",
	}
	// To-Do Change this to dynamic choice if more VPC is required

	vpc1 := AwsVpcConfig{
		CidrBlock:          "10.0.0.0/16",
		EnableDnsHostnames: true,
		EnableDnsSupport:   true,
		Subnet:             []AwsSubnetConfig{subnetPublic, subnetPrivate},
		InternetGateway:    true,
	}
	vpcMap := map[string]AwsVpcConfig{}
	vpcMap["vpc1"] = vpc1

	newCfg.Set("vpc", vpcMap)
	newCfg.Set("campaignstack", campaignName)
	ec2Map := map[string]AwsEc2Config{}

	for index, operatingSystem := range operatingSystems {

		allowSsh22CurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  22,
			DestPort: 22,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-ssh-22-%s", index, campaignName, operatingSystem, whitelistedIP),
		}

		allowTelnetHoneyCloudCurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  23,
			DestPort: 23,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-telnet-%v-%s", index, campaignName, operatingSystem, common.HoneyopsSSHPort, whitelistedIP),
		}

		allowHTTPHoneyCloudCurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  80,
			DestPort: 80,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-HTTP-%v-%s", index, campaignName, operatingSystem, common.HoneyopsSSHPort, whitelistedIP),
		}

		allowHTTPSHoneyCloudCurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  443,
			DestPort: 443,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-HTTPS-%v-%s", index, campaignName, operatingSystem, common.HoneyopsSSHPort, whitelistedIP),
		}

		allowSshHoneyCloudCurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  common.HoneyopsSSHPort,
			DestPort: common.HoneyopsSSHPort,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-ssh-%v-%s", index, campaignName, operatingSystem, common.HoneyopsSSHPort, whitelistedIP),
		}

		allowRDP3389CurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  3389,
			DestPort: 3389,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-rdp-3389-%s", index, campaignName, operatingSystem, whitelistedIP),
		}

		allowSMB445CurrIp := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("%s/32", whitelistedIP),
			SrcPort:  445,
			DestPort: 445,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-%s-%s-allow-smb-445-%s", index, campaignName, operatingSystem, whitelistedIP),
		}

		allowAllTrafficIpv4 := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("0.0.0.0/0"),
			SrcPort:  -1,
			DestPort: -1,
			Protocol: "-1",
			Name:     fmt.Sprintf("%d-%s-%s-allow-all-traffic", index, campaignName, operatingSystem),
		}

		for key, val := range tools {
			tools[key] = strings.ToLower(val)
		}

		if strings.EqualFold(operatingSystem, "Ubuntu") {

			ec2SecurityGroupUbuntu := AwsSecurityGroup{
				SecurityGroupName:        fmt.Sprintf("%d-%s-%s-public-sg", index, campaignName, operatingSystem),
				SecurityGroupDescription: "Security Group for Public Facing Ubuntu",
				IngressRules: []AwsSecurityRules{
					allowSsh22CurrIp,
					allowSshHoneyCloudCurrIp,
					allowTelnetHoneyCloudCurrIp,
					allowHTTPHoneyCloudCurrIp,
					allowHTTPSHoneyCloudCurrIp,
				},
				EgressRules: []AwsSecurityRules{
					allowAllTrafficIpv4,
				},
			}

			ec2Ubuntu := AwsEc2Config{
				Index:              index,
				AmiName:            "ubuntu/images/hvm-ssd/ubuntu-%s-%s-%s-server*",
				AmiVersionName:     "jammy",
				AmiVersionNumber:   "22.04",
				AmiInstanceType:    "t4g.small",
				AmiInstanceCpuArch: "arm64",
				AmiOwnerId:         "099720109477",
				AmiOperatingSystem: "ubuntu",
				SecurityGroup: []AwsSecurityGroup{
					ec2SecurityGroupUbuntu,
				},
				AssociatePublicIpAddress: true,
				Tools:                    tools,
				OsUser:                   "ubuntu",
			}
			ec2Map[fmt.Sprintf("%d-ubuntu", index)] = ec2Ubuntu

		} else if strings.EqualFold(operatingSystem, "Windows") {
			ec2SecurityGroupWindows := AwsSecurityGroup{
				SecurityGroupName:        fmt.Sprintf("%d-%s-%s-public-sg", index, campaignName, operatingSystem),
				SecurityGroupDescription: "Security Group for Public Facing Windows",
				IngressRules: []AwsSecurityRules{
					allowRDP3389CurrIp,
					allowSMB445CurrIp,
				},
				EgressRules: []AwsSecurityRules{
					allowAllTrafficIpv4,
				},
			}

			ec2Windows := AwsEc2Config{
				Index:              index,
				AmiName:            "EC2LaunchV2-Windows_Server-2019-English-Full-Base-2025.09.10",
				AmiInstanceType:    "t2.small",
				AmiInstanceCpuArch: "x86_64",
				AmiOwnerId:         "801119661308",
				AmiOperatingSystem: "Windows",
				SecurityGroup: []AwsSecurityGroup{
					ec2SecurityGroupWindows,
				},
				AssociatePublicIpAddress: true,
				Tools:                    tools,
				OsUser:                   "administrator",
			}
			ec2Map[fmt.Sprintf("%d-windows", index)] = ec2Windows

		}
	}

	newCfg.Set("ec2", ec2Map)

	newCfg.Set("status", UnDeployedState)

	newCfg.Set("tools", tools)

	newCfg.Set("cloudprovider", cloudProvider)

	newCfg.Set("cloudregion", cloudgregion)

	configPath := filepath.Join(userHoneyOpsCampaignPath, configName+"_"+campaignName+"."+configType)
	err = newCfg.WriteConfigAs(configPath)

	if err != nil {
		return "", err
	}

	return configPath, nil
}
