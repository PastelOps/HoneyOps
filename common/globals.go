package common

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var currentPublicIPGlobal string
var privateSSHKeysGlobal = make(map[string]pulumi.StringOutput)
var privateSSHKeysPathGlobal = make(map[string]string)
var keyPairGlobal = make(map[string]ec2.KeyPair)
var HoneyopsSSHPort = 65423
