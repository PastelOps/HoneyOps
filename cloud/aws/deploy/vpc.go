package deploy

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Define variables needed outside the buildInfrastructure() function
var idOfVpc pulumi.StringInput
var publicSubnets pulumi.StringArray
var privateSubnets pulumi.StringArray
var idPubRoute pulumi.StringInput  // ID of default route table
var idPrivRoute pulumi.StringInput // ID of NAT route table
var azNumber int                   // Number of AZs

func (a *AwsPulumiDeployer) createVPC(ctx *pulumi.Context) (err error) {

	for vpcIdx, vpcCfg := range a.VpcConfig {
		_ = vpcIdx
		vpcName := fmt.Sprintf("%v-vpc", a.CampaignStack)
		vpc, err := ec2.NewVpc(ctx, vpcName, &ec2.VpcArgs{
			CidrBlock:          pulumi.String(vpcCfg.CidrBlock),
			EnableDnsHostnames: pulumi.Bool(vpcCfg.EnableDnsHostnames),
			EnableDnsSupport:   pulumi.Bool(vpcCfg.EnableDnsSupport),
			Tags: pulumi.StringMap{
				"Name":          pulumi.String(vpcName),
				"CampaignStack": pulumi.String(fmt.Sprintf("honeyops-%v", a.CampaignStack)),
			},
		})

		idOfVpc = vpc.ID()

		idOfVpc.ToStringOutput().ApplyT(func(vpcid string) (string, error) {
			return "", nil
		})

		// Adopt the default route in the VPC
		defaultRoutTableName := fmt.Sprintf("%v-vpc", a.CampaignStack)
		defRoute, err := ec2.NewDefaultRouteTable(ctx, defaultRoutTableName, &ec2.DefaultRouteTableArgs{
			DefaultRouteTableId: vpc.DefaultRouteTableId,
			Tags: pulumi.StringMap{
				"Name":          pulumi.String(defaultRoutTableName),
				"CampaignStack": pulumi.String(fmt.Sprintf("honeyops-%v", a.CampaignStack)),
			},
		})
		if err != nil {
			return err
		}

		// Set value of public variable
		idPubRoute = defRoute.ID()

		// If Config Defines Internet Gateway
		if vpcCfg.InternetGateway {
			// Create an Internet gateway
			inetGWName := fmt.Sprintf("%v-inet-gw", a.CampaignStack)
			inetGw, err := ec2.NewInternetGateway(ctx, inetGWName, &ec2.InternetGatewayArgs{
				VpcId: vpc.ID(),
				Tags: pulumi.StringMap{
					"Name":          pulumi.String(inetGWName),
					"CampaignStack": pulumi.String(fmt.Sprintf("honeyops-%v", a.CampaignStack)),
				},
			})
			if err != nil {
				return err
			}

			// Associate gateway with default route
			inetRouteName := fmt.Sprintf("%v-inet-route", a.CampaignStack)
			_, err = ec2.NewRoute(ctx, inetRouteName, &ec2.RouteArgs{
				RouteTableId:         defRoute.ID(),
				DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
				GatewayId:            inetGw.ID(),
			})
			if err != nil {
				return err
			}
		}

		// If Config Contains Subnets
		for subnetIndex, subnetConfig := range vpcCfg.Subnet {
			public := subnetConfig.MapPublicIpOnLaunch
			subnetName := fmt.Sprintf("%v-priv-subnet-%d", a.CampaignStack, subnetIndex)
			if public {
				subnetName = fmt.Sprintf("%v-pub-subnet-%d", a.CampaignStack, subnetIndex)
			}

			subnet, err := ec2.NewSubnet(ctx, subnetName, &ec2.SubnetArgs{
				VpcId:               vpc.ID(),
				CidrBlock:           pulumi.String(subnetConfig.CidrBlock),
				MapPublicIpOnLaunch: pulumi.Bool(subnetConfig.MapPublicIpOnLaunch),
				Tags: pulumi.StringMap{
					"Name":          pulumi.String(subnetName),
					"CampaignStack": pulumi.String(fmt.Sprintf("honeyops-%v", a.CampaignStack)),
				},
			})
			if err != nil {
				return err
			}
			// Add value to array
			if public {
				a.publicSubnet = subnet.ID()
			} else {
				a.privateSubnet = subnet.ID()
			}
		}
	}

	return nil
}
