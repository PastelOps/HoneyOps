package deploy

import (
	"HoneyOps/common"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/vpc"
	"github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
)

func (a *AwsPulumiDeployer) InitSSHKeys(ctx *pulumi.Context) (err error) {

	// Create an SSH key
	sshKey, err := tls.NewPrivateKey(ctx, "ec2-ssh-key", &tls.PrivateKeyArgs{
		Algorithm: pulumi.String("RSA"), //ED25519 not supported by AWS for Windows AMI
	})
	if err != nil {
		fmt.Printf("error creating SSH key: %s", err.Error())
	}

	// Create an AWS key pair
	keyPair, err := ec2.NewKeyPair(ctx, "ec2-key-pair", &ec2.KeyPairArgs{
		PublicKey: sshKey.PublicKeyOpenssh,
	})
	if err != nil {
		fmt.Printf("error creating AWS key pair: %s", err.Error())
	}

	// Write Private Key to Local Directory for Ansible
	userHomeDirName, err := os.UserHomeDir()
	userHoneyOpsFolder := ".honeyops"
	privateKeysFolder := "privatekeys"
	privateKeysFolderPath := filepath.Join(userHomeDirName, userHoneyOpsFolder, privateKeysFolder)
	os.MkdirAll(privateKeysFolderPath, os.ModePerm)
	filenameOpenSSHKey := filepath.Join(privateKeysFolderPath, fmt.Sprintf("Ec2SshKey_%v.ppk", a.CampaignStack))
	filenamePem := filepath.Join(privateKeysFolderPath, fmt.Sprintf("Ec2SshKeyPEM_%v.ppk", a.CampaignStack))

	permissions := 0600 // Read and write for owner, read-only for others

	pulumi.Unsecret(sshKey.PrivateKeyOpenssh).ApplyT(func(v string) error {
		err := os.WriteFile(filenameOpenSSHKey, []byte(v), os.FileMode(permissions))
		if err != nil {
			log.Fatalf("Failed to save Ec2 SSH Key to file: %v", err)
			os.Exit(1)
		}
		//log.Printf("Successfully wrote to %s", filename)
		return nil
	})

	pulumi.Unsecret(sshKey.PrivateKeyPem).ApplyT(func(v string) error {
		err := os.WriteFile(filenamePem, []byte(v), os.FileMode(permissions))
		if err != nil {
			log.Fatalf("Failed to save Ec2 SSH Key to file: %v", err)
			os.Exit(1)
		}
		//log.Printf("Successfully wrote to %s", filename)
		return nil
	})

	// Export the generated private key
	//ctx.Export("EC2-PrivateKey", sshKey.PrivateKeyOpenssh)
	ctx.Export(fmt.Sprintf("EC2-PrivKeyPath-%v", a.CampaignStack), pulumi.String(filenameOpenSSHKey))
	a.privateSSHKeysGlobal["EC2-PrivateKey"] = sshKey.PrivateKeyOpenssh
	a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"] = filenameOpenSSHKey
	a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey-Pem"] = filenamePem
	a.ec2KeyPairs["EC2-Key-Pair"] = keyPair

	return nil
}

func (a *AwsPulumiDeployer) createEC2(ctx *pulumi.Context) error {

	currIP, _ := common.GetPublicIP()

	for ec2Name, ec2Cfg := range a.Ec2Config {
		securityGroupsPerInstance := pulumi.StringArray{}

		// Create Security Groups for the respective EC2 instance
		for _, sgCfg := range ec2Cfg.SecurityGroup {

			// Need be more dynamic, now its just hardcoded
			idOfVpc.ToStringOutput().ApplyT(func(vpcid string) (string, error) {
				return "", nil
			})

			ec2SecurityGroup, err := ec2.NewSecurityGroup(ctx, sgCfg.SecurityGroupName, &ec2.SecurityGroupArgs{
				VpcId:       idOfVpc,
				Name:        pulumi.String(sgCfg.SecurityGroupName),
				Description: pulumi.String(sgCfg.SecurityGroupDescription),
				Tags: pulumi.StringMap{
					"Name": pulumi.String(sgCfg.SecurityGroupName),
				},
			})
			if err != nil {
				fmt.Sprintf(" Create EC2 Security Group Error %s", err.Error())
			}

			securityGroupsPerInstance = append(securityGroupsPerInstance, ec2SecurityGroup.ID())

			for _, sgIngressCfg := range sgCfg.IngressRules {

				if sgIngressCfg.CidrIpv4 == "auto-current/32" {
					sgIngressCfg.CidrIpv4 = fmt.Sprintf("%s/32", currIP)
				}

				_, err = vpc.NewSecurityGroupIngressRule(ctx, sgIngressCfg.Name, &vpc.SecurityGroupIngressRuleArgs{
					SecurityGroupId: ec2SecurityGroup.ID(),
					CidrIpv4:        pulumi.String(sgIngressCfg.CidrIpv4),
					FromPort:        pulumi.Int(sgIngressCfg.SrcPort),
					IpProtocol:      pulumi.String(sgIngressCfg.Protocol),
					ToPort:          pulumi.Int(sgIngressCfg.DestPort),
				})
				if err != nil {
					fmt.Sprintf(" Create EC2 Ingress Rules Error %s", err.Error())
				}
			}

			for _, sgEgressCfg := range sgCfg.EgressRules {

				if sgEgressCfg.CidrIpv4 == "auto-current/32" {
					sgEgressCfg.CidrIpv4 = fmt.Sprintf("%s/32", currIP)
				}

				_, err = vpc.NewSecurityGroupEgressRule(ctx, sgEgressCfg.Name, &vpc.SecurityGroupEgressRuleArgs{
					SecurityGroupId: ec2SecurityGroup.ID(),
					CidrIpv4:        pulumi.String(sgEgressCfg.CidrIpv4),
					FromPort:        pulumi.Int(sgEgressCfg.SrcPort),
					IpProtocol:      pulumi.String(sgEgressCfg.Protocol),
					ToPort:          pulumi.Int(sgEgressCfg.DestPort),
				})
				if err != nil {
					fmt.Sprintf(" Create EC2 Egress Rules Error %s", err.Error())
				}
			}

		}

		amiName := ec2Cfg.AmiName

		if strings.EqualFold(ec2Cfg.AmiOperatingSystem, "Ubuntu") {
			// Get AMI ID for Ubuntu instance
			amiName = fmt.Sprintf(ec2Cfg.AmiName, ec2Cfg.AmiVersionName, ec2Cfg.AmiVersionNumber, ec2Cfg.AmiInstanceCpuArch)
		}

		ec2Ami, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
			Owners:     []string{ec2Cfg.AmiOwnerId},
			MostRecent: pulumi.BoolRef(true),
			Filters: []ec2.GetAmiFilter{
				{Name: "name", Values: []string{amiName}},
				{Name: "root-device-type", Values: []string{"ebs"}},
				{Name: "virtualization-type", Values: []string{"hvm"}},
				{Name: "architecture", Values: []string{ec2Cfg.AmiInstanceCpuArch}},
			},
		})
		if err != nil {
			fmt.Printf("error looking up EC2 AMI: %s", err.Error())
		}

		var ec2Instance *ec2.Instance

		if strings.EqualFold(ec2Cfg.AmiOperatingSystem, "Ubuntu") || strings.EqualFold(ec2Cfg.AmiOperatingSystem, "WazuhManager") {
			ec2Instance, err = ec2.NewInstance(ctx, ec2Name, &ec2.InstanceArgs{
				Ami:                      pulumi.String(ec2Ami.Id),
				InstanceType:             pulumi.String(ec2Cfg.AmiInstanceType),
				AssociatePublicIpAddress: pulumi.Bool(ec2Cfg.AssociatePublicIpAddress),
				KeyName:                  a.ec2KeyPairs["EC2-Key-Pair"].KeyName,
				SubnetId:                 a.publicSubnet,
				VpcSecurityGroupIds:      securityGroupsPerInstance,
				Tags: pulumi.StringMap{
					"Name": pulumi.String(ec2Name),
				},
			})
			if err != nil {
				fmt.Printf("error launching instance: %s", err.Error())
			}

			if strings.EqualFold(ec2Cfg.AmiOperatingSystem, "WazuhManager") {
				ctx.Export(fmt.Sprintf("%s-Web-User", ec2Name), pulumi.String("admin"))
				ctx.Export(fmt.Sprintf("%s-Web-Password (change i- to I-)", ec2Name), ec2Instance.ID())
			}

		} else if strings.EqualFold(ec2Cfg.AmiOperatingSystem, "Windows") {

			ec2Instance, err = ec2.NewInstance(ctx, ec2Name, &ec2.InstanceArgs{
				Ami:                      pulumi.String(ec2Ami.Id),
				InstanceType:             pulumi.String(ec2Cfg.AmiInstanceType),
				AssociatePublicIpAddress: pulumi.Bool(ec2Cfg.AssociatePublicIpAddress),
				KeyName:                  a.ec2KeyPairs["EC2-Key-Pair"].KeyName,
				SubnetId:                 a.publicSubnet,
				GetPasswordData:          pulumi.Bool(true),
				VpcSecurityGroupIds:      securityGroupsPerInstance,

				Tags: pulumi.StringMap{
					"Name": pulumi.String(ec2Name),
				},
			})
			if err != nil {
				fmt.Printf("error launching instance: %s", err.Error())
			}
			// Export the Windows Instance Password

			pulumi.Unsecret(ec2Instance.PasswordData).ApplyT(func(encryptedPassword string) error {
				//Unless what to save encrypted password to config
				ec2Cfg.WindowsEncryptedPw = encryptedPassword
				a.Ec2Config[ec2Name] = ec2Cfg
				a.updateConfigEC2Value(a.CampaignStack, a.Ec2Config)
				return nil
			})

		}

		// Export the generated private key
		ctx.Export(ec2Name, ec2Instance.PublicIp)

		pulumi.Unsecret(ec2Instance.PublicIp).ApplyT(func(publicIP string) error {
			ec2Cfg.PublicIpAddress = publicIP
			a.Ec2Config[ec2Name] = ec2Cfg
			a.updateConfigEC2Value(a.CampaignStack, a.Ec2Config)
			return nil
		})

		a.ec2Instances[ec2Name] = ec2Instance
	}

	return nil
}

func (a *AwsPulumiDeployer) SetupRemoteConnectionEC2(ctx *pulumi.Context, ec2Name string, ec2Inst *ec2.Instance) (*remote.Command, error) {

	remoteCommandAlias := fmt.Sprintf("%v-%v-installAnsibleAndMoveSSHPortCmd", a.CampaignStack, ec2Name)
	// Run a script to update packages on the remote machine.
	installAnsibleAndMoveSSHPortCmd, err := remote.NewCommand(ctx, remoteCommandAlias, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:       ec2Inst.PublicIp,
			Port:       pulumi.Float64(22),
			User:       pulumi.String(a.Ec2Config[ec2Name].OsUser),
			PrivateKey: a.privateSSHKeysGlobal["EC2-PrivateKey"],
		},
		Create: pulumi.String(fmt.Sprintf(
			"(sudo apt update); "+
				"(sudo add-apt-repository --yes --update ppa:ansible/ansible); "+
				"(sudo apt install ansible -y);"+
				"(sudo sed --in-place 's/^#\\?Port 22$/Port %v/g' /etc/ssh/sshd_config); "+
				"(sudo systemctl daemon-reload); "+
				"(sudo service ssh restart )"+
				"\n", common.HoneyopsSSHPort)),
	})
	if err != nil {
		return nil, err
	}

	return installAnsibleAndMoveSSHPortCmd, nil
}

// loadPrivateKey loads an RSA private key from a PEM file.
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}
	return privateKey, nil
}

func (a *AwsPulumiDeployer) GetWindowsPasswordData(ec2Name string) string {
	// Load the private key
	privateKey, err := loadPrivateKey(a.privateEc2PemPath)
	if err != nil {
		fmt.Printf("Failed to load privatekey: %v", err)
	}

	// Base64 decode the encrypted password
	encryptedPasswordBytes, err := base64.StdEncoding.DecodeString(a.Ec2Config[ec2Name].WindowsEncryptedPw)
	if err != nil {
		fmt.Printf("Failed to base64 decode password: %v", err)
	}

	// Decrypt the password
	decryptedPasswordBytes, err := rsa.DecryptPKCS1v15(nil, privateKey, encryptedPasswordBytes)
	if err != nil {
		fmt.Printf("Failed to decrypt password: %v", err)
	}

	return string(decryptedPasswordBytes)
}
