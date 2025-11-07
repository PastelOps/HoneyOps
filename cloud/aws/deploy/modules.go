package deploy

import (
	"HoneyOps/common"
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-gota/gota/dataframe"
	"github.com/go-gota/gota/series"
	"github.com/hashicorp/go-extract"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/tuan78/jsonconv"
)

const (
	CREATE_NEW_CONSOLE = 0x10
)

// Helper tool to convert windows path into WSL path.
func ConvertWindowsDirToWSL(windowsPath string) string {
	wslPath := strings.ReplaceAll(windowsPath, "\\", "/")
	wslPath = strings.ReplaceAll(wslPath, "C:/", "/mnt/c/")
	wslPath = strings.ReplaceAll(wslPath, "D:/", "/mnt/d/")
	wslPath = strings.ReplaceAll(wslPath, "E:/", "/mnt/e/")
	return wslPath
}

func (a *AwsPulumiDeployer) SetupCowrie(ctx *pulumi.Context, ec2Name string, remoteHost *ec2.Instance, prequsite1 *remote.Command, prequsite2 *remote.Command, prequsite3 *local.Command) error {

	path, _ := os.Getwd()
	localSourceCodePath := filepath.Join(path)

	// Ansible can't be initiated from Windows, therefore need WSL as a bridge into Linux to start
	if runtime.GOOS == "windows" {
		// Finally, play the Ansible playbook to finish installing.
		ansibleCowrie := fmt.Sprintf("%s-%s-playAnsibleCowrie-Windows", a.CampaignStack, ec2Name)
		_, err := local.NewCommand(ctx, ansibleCowrie, &local.CommandArgs{
			Create: remoteHost.PublicIp.ApplyT(func(publicIp string) (string, error) {
				return fmt.Sprintf("wsl cp %v ~/temp.ppk && wsl chmod 600 ~/temp.ppk && "+
					"wsl ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook "+
					"-u %s "+
					"-i '%v,' "+
					"--private-key ~/temp.ppk "+
					"%v/automation/install_cowrie.yaml -b",
					ConvertWindowsDirToWSL(a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"]),
					a.Ec2Config[ec2Name].OsUser,
					fmt.Sprintf("%s:%d", publicIp, common.HoneyopsSSHPort),
					ConvertWindowsDirToWSL(localSourceCodePath)), nil
			}).(pulumi.StringOutput),
		}, pulumi.DependsOn([]pulumi.Resource{
			prequsite1, prequsite2, prequsite3,
		}))

		if err != nil {
			return err
		}
	} else {
		// Finally, play the Ansible playbook to finish installing.
		ansibleCowrie := fmt.Sprintf("%s-%s-playAnsibleCowrie-Linux", a.CampaignStack, ec2Name)
		_, err := local.NewCommand(ctx, ansibleCowrie, &local.CommandArgs{
			Create: remoteHost.PublicIp.ApplyT(func(publicIp string) (string, error) {
				return fmt.Sprintf("cp %v ~/temp.ppk && chmod 600 ~/temp.ppk && "+
					"ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook "+
					"-u %s "+
					"-i '%v,' "+
					"--private-key ~/temp.ppk "+
					"%v/automation/install_cowrie.yaml -b",
					a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"],
					a.Ec2Config[ec2Name].OsUser,
					fmt.Sprintf("%s:%d", publicIp, common.HoneyopsSSHPort),
					localSourceCodePath), nil
			}).(pulumi.StringOutput),
		}, pulumi.DependsOn([]pulumi.Resource{
			prequsite1, prequsite2, prequsite3,
		}))
		if err != nil {
			return err
		}
	}

	return nil

}

func (a *AwsPulumiDeployer) CollectEvidencePack(ec2Name string) ([]string, error) {
	zipLogs := []string{}

	for _, tool := range a.Ec2Config[ec2Name].Tools {
		ouputPath, err := a.collectEvidencePack(ec2Name, a.Ec2Config[ec2Name].PublicIpAddress, tool)
		if err != nil {
			return nil, err
		}
		zipLogs = append(zipLogs, ouputPath)
	}
	return zipLogs, nil
}

func (a *AwsPulumiDeployer) collectEvidencePack(ec2Name string, remoteIP string, tool string) (string, error) {

	// Zip Logs Paths
	collectionOutputFileName := fmt.Sprintf("%s_EvidencePack_%v.zip", tool, time.Now().UTC().Format("20060102T1504Z"))
	collectionOutputFilePath := fmt.Sprintf("~/.LogsExport/%v", collectionOutputFileName)

	toolLogsLocation := "/Nonexistent"

	switch strings.ToLower(tool) {
	case "cowrie":
		toolLogsLocation = "/opt/cowrie/cowrie/var/log/cowrie /opt/cowrie/cowrie/var/lib/cowrie/downloads"
	case "galah":
		toolLogsLocation = "/opt/galah/logs"
	}

	cmd := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath,
		fmt.Sprintf("(sudo apt install zip -y); (mkdir -p ~/.LogsExport); ( sudo zip -r %s %s /var/log/syslog );", collectionOutputFilePath, toolLogsLocation))
	err := cmd.Run()
	if err == nil {
		fmt.Println("Zip Collection Created.")
	} else {

		return "", err
	}

	userHomeDirName, err := os.UserHomeDir()
	userHoneyOpsLogFolderPath := filepath.Join(userHomeDirName, ".honeyops", ".LogsExport")
	os.MkdirAll(userHoneyOpsLogFolderPath, os.ModePerm)
	localFilePath := filepath.Join(userHoneyOpsLogFolderPath, collectionOutputFileName)

	cmd = exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", a.privateEc2KeyPath, "-P", strconv.Itoa(common.HoneyopsSSHPort),
		fmt.Sprintf("%s@%s:%s", a.Ec2Config[ec2Name].OsUser, remoteIP, collectionOutputFilePath), localFilePath)
	err = cmd.Run()
	if err == nil {
		fmt.Printf("Zip Collection Downloaded: %s\n", localFilePath)
	} else {
		return "", err
	}

	return localFilePath, nil
}

func (a *AwsPulumiDeployer) ConnectSSH(ec2Name string) error {

	remoteIP := a.Ec2Config[ec2Name].PublicIpAddress
	sshCommand := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath)

	/*
		if strings.EqualFold(a.Ec2Config[ec2Name].AmiOperatingSystem, "WazuhManager") {
			// Connecting to cloud AMI Wazuh Manager using port 22
			// Reason is because the AMI will take about 10 mins to setup, and until then the SSH service is not accessible and therefore unable to standardize moving connection to 65423 port like the others.
			sshCommand = exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", "22", "-i", a.privateEc2KeyPath)
			fmt.Println(a.privateEc2KeyPath)
		}
	*/

	// Set the standard input, output, and error streams to the current process's streams
	sshCommand.Stdin = os.Stdin
	sshCommand.Stdout = os.Stdout
	sshCommand.Stderr = os.Stderr

	ssh_err := sshCommand.Run()
	if ssh_err != nil {
		fmt.Println("Failed to run SSH command:", ssh_err)
	}

	return nil
}

func (a *AwsPulumiDeployer) ConnectSSHSpawn(ec2Name string) error {

	var wg sync.WaitGroup
	wg.Add(1)
	if runtime.GOOS == "windows" {
		// Writer goroutine
		go func() {
			defer wg.Done()
			remoteIP := a.Ec2Config[ec2Name].PublicIpAddress

			cmdCommand := exec.Cmd{Path: "c:\\windows\\system32\\cmd.exe",
				Args: []string{"/C",
					"start",
					"ssh",
					fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP),
					"-i",
					a.privateEc2KeyPath,
					"-p",
					strconv.Itoa(common.HoneyopsSSHPort),
				},
			}
			cmdCommand.Run()
		}()

	} else {
		go func() {
			defer wg.Done()

			remoteIP := a.Ec2Config[ec2Name].PublicIpAddress
			sshCommand := exec.Command("gnome-terminal",
				"--",
				"bash",
				"-c",
				fmt.Sprintf("ssh %s@%s -i %s -p %s", a.Ec2Config[ec2Name].OsUser, remoteIP, a.privateEc2KeyPath, strconv.Itoa(common.HoneyopsSSHPort)))

			sshCommand.Stdin = os.Stdin
			sshCommand.Stdout = os.Stdout
			sshCommand.Stderr = os.Stderr

			ssh_err := sshCommand.Run()
			if ssh_err != nil {
				fmt.Println("Failed to run SSH command:", ssh_err)
			}
		}()
	}

	wg.Wait()

	return nil
}

func (a *AwsPulumiDeployer) ConnectEstablishWazuhSSHTunnel(ec2Name string) error {

	// As Wazuh agents are configured to poll their own localhost:1514 for manager server
	// We can setup a SSH reverse tunnel which will make the remote cloud instance to listen on their own localhost:1514 which
	// will go through the tunnel and come out on our own laptop / virtual machine port 1514 which we have our own wazuh manager setup.

	/*
		The other options are:

		-f tells ssh to background itself after it authenticates, so you don't have to sit around running something like sleep on the remote server for the tunnel to remain alive.
		-N says that you want an SSH connection, but you don't actually want to run any remote commands. If all you're creating is a tunnel, then including this option saves resources.
		-T disables pseudo-tty allocation, which is appropriate because you're not trying to create an interactive shell.
	*/

	remoteIP := a.Ec2Config[ec2Name].PublicIpAddress
	sshCommand := exec.Command("ssh", "-f", "-N", "-T", "-R1514:localhost:1514", "-R1515:localhost:1515", "-R55000:localhost:55000", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath)

	// Set the standard input, output, and error streams to the current process's streams
	sshCommand.Stdin = os.Stdin
	sshCommand.Stdout = os.Stdout
	sshCommand.Stderr = os.Stderr

	ssh_err := sshCommand.Run()
	if ssh_err != nil {
		fmt.Println("Failed to run SSH command:", ssh_err)
	}

	return nil
}

func (a *AwsPulumiDeployer) ConnectRDP(ec2Name string) error {

	remoteIP := a.Ec2Config[ec2Name].PublicIpAddress

	fmt.Printf("@Connecting to RDP -> %s:3389 /user:%s \n", remoteIP, a.Ec2Config[ec2Name].OsUser)

	cmdkeyCommand := exec.Command("cmdkey.exe",
		fmt.Sprintf("/generic:TERMSRV/%s", remoteIP),
		fmt.Sprintf("/user:%s", a.Ec2Config[ec2Name].OsUser),
		fmt.Sprintf("/pass:%s", a.GetWindowsPasswordData(ec2Name)))

	cmdkey_err := cmdkeyCommand.Run()
	if cmdkey_err != nil {
		fmt.Println("Failed to run cmdkey command:", cmdkey_err)
	}

	rdpCommand := exec.Command("mstsc.exe", fmt.Sprintf("/v:%s", remoteIP))

	// Set the standard input, output, and error streams to the current process's streams
	rdpCommand.Stdin = os.Stdin
	rdpCommand.Stdout = os.Stdout
	rdpCommand.Stderr = os.Stderr

	rdp_err := rdpCommand.Run()
	if rdp_err != nil {
		fmt.Println("Failed to run RDP command:", rdp_err)
	}

	delkeyCommand := exec.Command("cmdkey.exe",
		fmt.Sprintf("/delete:TERMSRV/%s", remoteIP))

	delkeyCmd_err := delkeyCommand.Run()
	if delkeyCmd_err != nil {
		fmt.Println("Failed to run cmdkey /delete command:", delkeyCmd_err)
	}

	return nil
}

func (a *AwsPulumiDeployer) ConnectPsExec(ec2Name string) error {

	remoteIP := a.Ec2Config[ec2Name].PublicIpAddress

	// Download Sys-Internal PsExec
	userHomeDirName, _ := os.UserHomeDir()
	userHoneyOpsSysInernalPath := filepath.Join(userHomeDirName, ".honeyops", ".sysinternal")
	os.MkdirAll(userHoneyOpsSysInernalPath, os.ModePerm)
	psexecPath := filepath.Join(userHoneyOpsSysInernalPath, "PsExec.exe")
	pstoolZipPath := filepath.Join(userHoneyOpsSysInernalPath, "PsTool.exe")

	// Attempt to check if psexec has been downloaded before.
	_, err := os.Stat(psexecPath)

	sysInternalPSToolUrl := "https://download.sysinternals.com/files/PSTools.zip"

	if err != nil && errors.Is(err, os.ErrNotExist) {
		fmt.Printf("SysInternal PSTools Not Found, initialising PSTools from %s", sysInternalPSToolUrl)
		downloadFile(pstoolZipPath, sysInternalPSToolUrl)
		err := unzip(pstoolZipPath, userHoneyOpsSysInernalPath)
		if err != nil {
			log.Fatalf("Error unzipping file: %v", err)
		}
		fmt.Printf("Successfully unzipped '%s' to '%s'\n", pstoolZipPath, userHoneyOpsSysInernalPath)
	}

	psexecCommand := exec.Command(psexecPath,
		fmt.Sprintf("\\\\%s", remoteIP),
		"-u",
		fmt.Sprintf("%s\\", remoteIP, a.Ec2Config[ec2Name].OsUser),
		"-p",
		a.GetWindowsPasswordData(ec2Name),
		"cmd")

	// Set the standard input, output, and error streams to the current process's streams
	psexecCommand.Stdin = os.Stdin
	psexecCommand.Stdout = os.Stdout
	psexecCommand.Stderr = os.Stderr

	psexec_err := psexecCommand.Run()
	if psexec_err != nil {
		fmt.Println("Failed to run PsExec command:", psexec_err)
	}

	return nil
}

func (a *AwsPulumiDeployer) WatchLogs(ec2Name string, tool string) error {

	remoteIP := a.Ec2Config[ec2Name].PublicIpAddress
	toolLogsLocation := "/Nonexistent"

	switch tool {
	case "Cowrie":
		toolLogsLocation = "/opt/cowrie/cowrie/var/log/cowrie/cowrie.json"
	case "Galah":
		toolLogsLocation = "/opt/galah/logs/event_log.json"
	}

	sshCommand := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath, fmt.Sprintf(
		"sudo tail -f %s", toolLogsLocation,
	))

	// Set the standard input, output, and error streams to the current process's streams
	sshCommand.Stdin = os.Stdin
	sshCommand.Stdout = os.Stdout
	sshCommand.Stderr = os.Stderr

	ssh_err := sshCommand.Run()
	if ssh_err != nil {
		fmt.Println("Failed to run tail command:", ssh_err)
	}

	return nil
}

// Setup a LLM Web POT
func (a *AwsPulumiDeployer) SetupGalahLLMPot(ctx *pulumi.Context, ec2Name string, remoteHost *ec2.Instance, prequsite1 *remote.Command, prequsite2 *remote.Command, prequsite3 *local.Command) error {

	path, _ := os.Getwd()
	localFilePath := filepath.Join(path, "Prebuilt-Go-Binary", "galah")

	// Upload Galah
	UploadGalahCommand, err := local.NewCommand(ctx, fmt.Sprintf("%s-%s-UploadGalahCommand", a.CampaignStack, ec2Name), &local.CommandArgs{
		Create: remoteHost.PublicIp.ApplyT(func(publicIp string) (string, error) {
			return fmt.Sprintf("scp -r -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "+
				"-i %v -P %v %v "+
				"%s@%v:", a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"], common.HoneyopsSSHPort, localFilePath, a.Ec2Config[ec2Name].OsUser, publicIp), nil
		}).(pulumi.StringOutput),
	}, pulumi.DependsOn([]pulumi.Resource{
		prequsite1, prequsite2, prequsite3,
	}))

	remoteGalahCommandAlias := fmt.Sprintf("%v-%v-installGalahCmd", a.CampaignStack, ec2Name)

	// Set Permissions Galah
	_, err = remote.NewCommand(ctx, remoteGalahCommandAlias, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:       remoteHost.PublicIp,
			Port:       pulumi.Float64(common.HoneyopsSSHPort),
			User:       pulumi.String(a.Ec2Config[ec2Name].OsUser),
			PrivateKey: a.privateSSHKeysGlobal["EC2-PrivateKey"],
		},
		Create: pulumi.String(fmt.Sprintf(
			"(sudo adduser --disabled-password  --gecos \"\" galah); "+
				"(sudo usermod -a -G galah galah);"+
				"(sudo mv galah /opt/galah);"+
				"(sudo chown -R galah:galah /opt/galah);"+
				"(sudo cp /opt/galah/galah.serivce /etc/systemd/system/galah.service);"+
				"(sudo chmod +x /opt/galah/galah); (echo /opt/galah/galah -p openai -m gpt-4.1-mini --event-log-file /opt/galah/logs/event_log.json --cache-db-file /opt/galah/logs/cache.db --config-file %v --api-key='%v' | sudo tee /opt/galah/execute_galah.sh); (sudo chmod +x /opt/galah/execute_galah.sh); (sudo systemctl start galah);", "/opt/galah/config/config.yaml", a.LLMApiKey,
		)),
	}, pulumi.DependsOn([]pulumi.Resource{
		prequsite1, prequsite2, prequsite3,
		UploadGalahCommand,
	}))

	return err
}

func (a *AwsPulumiDeployer) CollectGalahEvidencePack(ec2Name string, remoteIP string) error {

	// Zip Logs Paths
	collectionOutputFileName := fmt.Sprintf("Galah_EvidencePack_%v.zip", time.Now().UTC().Format("20060102T1504Z"))
	collectionOutputFilePath := fmt.Sprintf("~/.LogsExport/%v", collectionOutputFileName)

	cmd := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, remoteIP), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath,
		fmt.Sprintf("(sudo apt install zip -y); (mkdir -p ~/.LogsExport); ( sudo zip -r %v /opt/galah/logs /var/log/syslog );", collectionOutputFilePath))
	err := cmd.Run()
	if err == nil {
		fmt.Println("Zip Collection Created.")
	}

	userHomeDirName, err := os.UserHomeDir()
	userHoneyOpsLogFolderPath := filepath.Join(userHomeDirName, ".honeyops", ".LogsExport")
	os.MkdirAll(userHoneyOpsLogFolderPath, os.ModePerm)
	localFilePath := filepath.Join(userHoneyOpsLogFolderPath, collectionOutputFileName)

	cmd = exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", a.privateEc2KeyPath, "-P", strconv.Itoa(common.HoneyopsSSHPort),
		fmt.Sprintf("%s@%s:%s", a.Ec2Config[ec2Name].OsUser, remoteIP, collectionOutputFilePath), localFilePath)
	err = cmd.Run()
	if err == nil {
		fmt.Printf("Zip Collection Downloaded: %s\n", localFilePath)
	}

	return nil
}

func (a *AwsPulumiDeployer) RandomizeCowrieEnvironment(ec2Name string) error {

	cmd := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, a.Ec2Config[ec2Name].PublicIpAddress), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath,
		fmt.Sprintf("(sudo sh -c \"cp -r /opt/cowrie/backup/* /opt/cowrie/cowrie\"); "+
			"(sudo python3 /opt/cowrie/cowrie.obscurer.py /opt/cowrie/cowrie/ -a); "+
			"(sudo systemctl restart cowrie); "+
			"(sudo systemctl restart cowrie.socket); "))

	// Set the standard input, output, and error streams to the current process's streams
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Failed to run SSH command:", err)
	} else {
		fmt.Println("Cowrie environment randomize.")
	}

	return nil
}

func (a *AwsPulumiDeployer) SetupWazuhAgent(ctx *pulumi.Context, ec2Name string, remoteHost *ec2.Instance, prequsite1 *remote.Command, prequsite2 *remote.Command, prequsite3 *local.Command) error {

	installWazuhAgentAlias := fmt.Sprintf("%s-%s-installWazuhAgent", a.CampaignStack, ec2Name)
	// Run a script to update packages on the remote machine.
	installWazuhAgent, err := remote.NewCommand(ctx, installWazuhAgentAlias, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:       remoteHost.PublicIp,
			Port:       pulumi.Float64(common.HoneyopsSSHPort),
			User:       pulumi.String(a.Ec2Config[ec2Name].OsUser),
			PrivateKey: a.privateSSHKeysGlobal["EC2-PrivateKey"],
		},
		Create: pulumi.String(fmt.Sprintf(
			"(sudo apt-get install gnupg apt-transport-https -y ); " +
				"(sudo curl -s https://packages.wazuh.com/key/GPG-KEY-WAZUH | sudo gpg --no-default-keyring --keyring gnupg-ring:/usr/share/keyrings/wazuh.gpg --import && sudo chmod 644 /usr/share/keyrings/wazuh.gpg); " +
				"(sudo echo \"deb [signed-by=/usr/share/keyrings/wazuh.gpg] https://packages.wazuh.com/4.x/apt/ stable main\" | sudo tee -a /etc/apt/sources.list.d/wazuh.list);" +
				"(sudo apt-get update); " +
				"(sudo WAZUH_MANAGER=\"localhost\" apt-get install wazuh-agent -y ) ; " +
				"(sudo sed --in-place 's/MANAGER_IP/localhost/g' /var/ossec/etc/ossec.conf ); " +
				"(sudo systemctl daemon-reload && sudo systemctl enable wazuh-agent && sudo systemctl start wazuh-agent);" +
				"\n")),
	}, pulumi.DependsOn([]pulumi.Resource{
		prequsite1,
		prequsite2,
		prequsite3,
	}))
	if err != nil {
		return err
	}
	_ = installWazuhAgent

	return nil
}

func (a *AwsPulumiDeployer) SetupYara(ctx *pulumi.Context, ec2Name string, ec2Inst *ec2.Instance, prequsite *remote.Command) (*remote.Command, *local.Command, error) {

	userHomeDirName, _ := os.UserHomeDir()
	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsFolderPath := filepath.Join(userHomeDirName, userHoneyOpsFolder)
	os.MkdirAll(userHoneyOpsFolderPath, os.ModePerm)

	path, _ := os.Getwd()
	localSourceCodePath := filepath.Join(path)

	remoteYaraCommandAlias := fmt.Sprintf("%s-%s-installYaraCmd", a.CampaignStack, ec2Name)
	// Run a script to update packages on the remote machine.
	installYaraCmd, err := remote.NewCommand(ctx, remoteYaraCommandAlias, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:       ec2Inst.PublicIp,
			Port:       pulumi.Float64(common.HoneyopsSSHPort),
			User:       pulumi.String(a.Ec2Config[ec2Name].OsUser),
			PrivateKey: a.privateSSHKeysGlobal["EC2-PrivateKey"],
		},
		Create: pulumi.String(fmt.Sprintf(
			"(sudo apt-get install git automake libtool make gcc pkg-config inotify-tools -y ); " +
				"(sudo curl -LO https://github.com/VirusTotal/yara/archive/v4.5.5.tar.gz); " +
				"(sudo tar -xvzf v4.5.5.tar.gz -C /usr/local/bin/ && rm -f v4.5.5.tar.gz); " +
				"(cd /usr/local/bin/yara-4.5.5/ && sudo ./bootstrap.sh && sudo ./configure && sudo make && sudo make install && sudo make check);" +
				"(sudo mkdir -p /opt/yara/rules); " +
				"(sudo wget https://raw.githubusercontent.com/faelana/yara-rule_eicar/refs/heads/main/yara-rule_eicar.yara -O /opt/yara/rules/yara-rule_eicar.yar);")),
	}, pulumi.DependsOn([]pulumi.Resource{
		prequsite,
	}))
	if err != nil {
		return nil, nil, err
	}

	// Ansible can't be initiated from Windows, therefore need WSL as a bridge into Linux to start
	if runtime.GOOS == "windows" {
		// Finally, play the Ansible playbook to finish installing.
		installYaraMonitoringCmd, _ := local.NewCommand(ctx, fmt.Sprintf("%s-%s-playAnsibleYaraMonitoring-Window", a.CampaignStack, ec2Name), &local.CommandArgs{
			Create: ec2Inst.PublicIp.ApplyT(func(publicIp string) (string, error) {
				return fmt.Sprintf("wsl cp %v ~/temp.ppk && wsl chmod 600 ~/temp.ppk && "+
					"wsl ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook "+
					"-u %s "+
					"-i '%v,' "+
					"--private-key ~/temp.ppk "+
					"%v/automation/yara-monitoring-ansible/setup_yara_monitoring.yaml -b ",
					ConvertWindowsDirToWSL(a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"]),
					a.Ec2Config[ec2Name].OsUser,
					fmt.Sprintf("%s:%d, ", publicIp, common.HoneyopsSSHPort),
					ConvertWindowsDirToWSL(localSourceCodePath)), nil
			}).(pulumi.StringOutput),
		}, pulumi.DependsOn([]pulumi.Resource{
			installYaraCmd,
		}))
		return installYaraCmd, installYaraMonitoringCmd, err
	} else {
		// Finally, play the Ansible playbook to finish installing.
		installYaraMonitoringCmd, err := local.NewCommand(ctx, fmt.Sprintf("%s-%s-playAnsibleYaraMonitoring-Linux", a.CampaignStack, ec2Name), &local.CommandArgs{
			Create: ec2Inst.PublicIp.ApplyT(func(publicIp string) (string, error) {
				return fmt.Sprintf("cp %v ~/temp.ppk && chmod 600 ~/temp.ppk && "+
					"ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook "+
					"-u %s "+
					"-i '%v,' "+
					"--private-key ~/temp.ppk "+
					"%v/automation/yara-monitoring-ansible/setup_yara_monitoring.yaml -b ",
					a.privateSSHKeysPathGlobal["EC2-Server-PrivateKey"],
					a.Ec2Config[ec2Name].OsUser,
					fmt.Sprintf("%s:%d, ", publicIp, common.HoneyopsSSHPort),
					localSourceCodePath), nil
			}).(pulumi.StringOutput),
		}, pulumi.DependsOn([]pulumi.Resource{
			installYaraCmd,
		}))
		return installYaraCmd, installYaraMonitoringCmd, err
	}
	return nil, nil, nil
}

func (a *AwsPulumiDeployer) GitCloneYaraRules(ec2Name string, gitURL string) error {

	cmd := exec.Command("ssh", fmt.Sprintf("%s@%s", a.Ec2Config[ec2Name].OsUser, a.Ec2Config[ec2Name].PublicIpAddress), "-p", strconv.Itoa(common.HoneyopsSSHPort), "-i", a.privateEc2KeyPath,
		fmt.Sprintf("cd /opt/yara/rules/ && sudo git clone %s", gitURL))
	err := cmd.Run()
	if err == nil {
		fmt.Printf("Git clone %s succeed, repo stored in /opt/yara/rules")
	}

	return nil
}

// downloadFile downloads a file from the given URL and saves it to the specified filepath.
func downloadFile(filepath string, url string) error {
	// Create the file to save the download to
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close() // Ensure the file is closed

	// Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make HTTP GET request: %w", err)
	}
	defer resp.Body.Close() // Ensure the response body is closed

	// Check if the HTTP status code indicates success (e.g., 200 OK)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %s", resp.Status)
	}

	// Copy the response body to the file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body to file: %w", err)
	}

	return nil
}

// Unzip extracts a zip archive to a specified destination directory.
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		filePath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		srcFile, err := f.Open()
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}
	}
	return nil
}

func (a *AwsPulumiDeployer) OpenFirewall(ec2Name string, protocol string, port int) error {

	inst := a.Ec2Config[ec2Name]

	newRuleName := fmt.Sprintf("%d-%s-%s-allow-all-public-%s-%d", inst.Index, a.CampaignStack, inst.AmiOperatingSystem, protocol, port)

	exist := false

	for _, rule := range a.Ec2Config[ec2Name].SecurityGroup[0].IngressRules {
		if strings.EqualFold(rule.Name, newRuleName) {
			exist = true
		}
	}

	if !exist {
		allowSpecificPortPublic := AwsSecurityRules{
			CidrIpv4: fmt.Sprintf("0.0.0.0/0"),
			SrcPort:  port,
			DestPort: port,
			Protocol: strings.ToLower(protocol),
			Name:     newRuleName,
		}
		a.Ec2Config[ec2Name].SecurityGroup[0].IngressRules = append(a.Ec2Config[ec2Name].SecurityGroup[0].IngressRules, allowSpecificPortPublic)
		a.updateConfigEC2Value(a.CampaignStack, a.Ec2Config)

		fmt.Printf("Rule %s successfully added to %s config.\n", newRuleName, a.CampaignStack)
		fmt.Printf("Run `-m deploy` to update the change on the cloud.\n", newRuleName)
	} else {
		fmt.Printf("An existing rule has already been added to %s, therefore skipping this.", ec2Name)
	}

	return nil
}

func (a *AwsPulumiDeployer) CloseFirewall(ec2Name string, protocol string, port int) error {

	inst := a.Ec2Config[ec2Name]

	ruleName := fmt.Sprintf("%d-%s-%s-allow-all-public-%s-%d", inst.Index, a.CampaignStack, inst.AmiOperatingSystem, protocol, port)

	newIngressRules := []AwsSecurityRules{}
	removed := false
	for _, rule := range a.Ec2Config[ec2Name].SecurityGroup[0].IngressRules {
		if !strings.EqualFold(rule.Name, ruleName) {
			newIngressRules = append(newIngressRules, rule)
		} else {
			removed = true
		}
	}

	a.Ec2Config[ec2Name].SecurityGroup[0].IngressRules = newIngressRules
	a.updateConfigEC2Value(a.CampaignStack, a.Ec2Config)

	if removed {
		fmt.Printf("Rule %s successfully removed from %s config.\n", ruleName, a.CampaignStack)
		fmt.Printf("Run `-m deploy` to update the change on the cloud.\n", ruleName)
	}

	return nil
}

func (a *AwsPulumiDeployer) GenerateReport(ec2Name string) error {
	// Report Folders
	userHomeDirName, _ := os.UserHomeDir()
	userHoneyOpsSysReports := filepath.Join(userHomeDirName, ".honeyops", ".Reports")
	os.MkdirAll(userHoneyOpsSysReports, os.ModePerm)
	timestamp := time.Now().UTC().Format("20060102T1504Z")

	ziplogs, err := a.CollectEvidencePack(ec2Name)
	if err != nil {
		fmt.Printf("Error occured during ziplog collection: %v\n", err)
	}

	for _, ziplog := range ziplogs {

		var (
			ctx = context.Background()      // context for cancellation
			tm  = extract.NewTargetMemory() // create a new in-memory filesystem
			dst = ""                        // root of in-memory filesystem
			cfg = extract.NewConfig()       // custom config for extraction
		)

		file, err := os.Open(ziplog)
		src := bufio.NewReader(file)
		// unpack
		if err := extract.UnpackTo(ctx, tm, dst, src, cfg); err != nil {
			// handle error
		}

		// Zip archives can contain multiple .json files
		matches, err := tm.Glob("*.json")
		if err != nil {
			fmt.Sprintf("Glob() failed: ")
		}

		if strings.Contains(strings.ToLower(ziplog), "cowrie") {
			var markDownReport strings.Builder
			outputFilePath := filepath.Join(userHoneyOpsSysReports, fmt.Sprintf("Cowrie_SummaryReport_%v.md", timestamp))

			markDownReport.WriteString("# Cowrie Summary Report\n\n")
			markDownReport.WriteString(fmt.Sprint("Campaign Name: %s\n", a.CampaignStack))
			markDownReport.WriteString(fmt.Sprint("Name of Instance: %s\n", ec2Name))
			markDownReport.WriteString(fmt.Sprint("HoneyPot IP: %s\n", a.Ec2Config[ec2Name].PublicIpAddress))
			markDownReport.WriteString("\n\n")

			limitRows := 10

			var cowrieEvents jsonconv.JsonArray

			for _, element := range matches {
				content, err := fs.ReadFile(tm, element)
				if err != nil {
					fmt.Sprintf("ReadFile failed: ")
				}

				scanner := bufio.NewScanner(strings.NewReader(string(content)))

				for scanner.Scan() {
					obj := make(jsonconv.JsonObject)
					re := jsonconv.NewJsonReader(strings.NewReader(scanner.Text()))
					re.Read(&obj)
					cowrieEvents = append(cowrieEvents, obj)
				}
			}

			df := dataframe.LoadMaps(cowrieEvents)

			// Table Sessions & Logins - Top IP address with events associated with them
			groups := df.GroupBy("src_ip")
			var cowrieIPCount jsonconv.JsonArray
			for index, value := range groups.GetGroups() {
				obj := jsonconv.JsonObject{
					"SourceIP": index,
					"Count":    value.Col("src_ip").Len(),
				}
				cowrieIPCount = append(cowrieIPCount, obj)
			}
			df_srcIP := dataframe.LoadMaps(cowrieIPCount)
			markDownReport.WriteString("# Summary top 10 Most Frequent Intruder IP Addresses\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_srcIP.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "SourceIP"}), []string{"Count", "SourceIP"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Table Different EventTypes Count()
			var cowrieEventTypeCount jsonconv.JsonArray
			groups = df.GroupBy("eventid")
			for index, value := range groups.GetGroups() {
				obj := jsonconv.JsonObject{
					"EventTypes": index,
					"Count":      value.Col("eventid").Len(),
				}
				cowrieEventTypeCount = append(cowrieEventTypeCount, obj)
			}
			df_eventType := dataframe.LoadMaps(cowrieEventTypeCount)
			markDownReport.WriteString("# Summary Numbers of EventsTypes Recorded\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_eventType.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "EventTypes"}), []string{"Count", "EventTypes"}, -1))
			markDownReport.WriteString("\n\n")

			// Select Events with successful login
			fil1 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.In,
				Comparando: []string{"cowrie.login.success", "cowrie.login.failed"}},
			)

			// Top username used in login
			var cowrieEnteredUsername jsonconv.JsonArray
			for index, value := range fil1.GroupBy("username").GetGroups() {
				obj := jsonconv.JsonObject{
					"Username": index,
					"Count":    value.Col("username").Len(),
				}
				cowrieEnteredUsername = append(cowrieEnteredUsername, obj)
			}
			df_username := dataframe.LoadMaps(cowrieEnteredUsername)
			markDownReport.WriteString("# Summary top 10 Usernames Entered by Intruders\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_username.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "Username"}), []string{"Count", "Username"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Top password used in login
			var cowrieEnteredPassword jsonconv.JsonArray
			for index, value := range fil1.GroupBy("password").GetGroups() {
				obj := jsonconv.JsonObject{
					"Password": index,
					"Count":    value.Col("password").Len(),
				}
				cowrieEnteredPassword = append(cowrieEnteredPassword, obj)
			}
			df_password := dataframe.LoadMaps(cowrieEnteredPassword)
			markDownReport.WriteString("# Summary top 10 Passwords Entered by Intruders\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_password.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "Password"}), []string{"Count", "Password"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Top number of IPs with success and failed login
			var cowrieLoginCount jsonconv.JsonArray

			for index, value := range fil1.GroupBy("src_ip").GetGroups() {
				obj := jsonconv.JsonObject{
					"SourceIP": index,
					"Count":    value.Col("src_ip").Len(),
				}
				cowrieLoginCount = append(cowrieLoginCount, obj)
			}

			df_logincount := dataframe.LoadMaps(cowrieLoginCount)
			markDownReport.WriteString("# Summary top 10 IP Addresses Attempted Login\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_logincount.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "SourceIP"}), []string{"Count", "SourceIP"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select Events with command inputs
			fil2 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.command.input"},
			)

			// IPs that executed most commands
			var cowrieCommandCount jsonconv.JsonArray
			for index, value := range fil2.GroupBy("input").GetGroups() {
				obj := jsonconv.JsonObject{
					"Command": index,
					"Count":   value.Col("input").Len(),
				}
				cowrieCommandCount = append(cowrieCommandCount, obj)
			}
			df_commands := dataframe.LoadMaps(cowrieCommandCount)
			markDownReport.WriteString("# Summary top 10 Commands Entered by Intruders\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_commands.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "Command"}), []string{"Count", "Command"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select Events with File Upload
			fil3 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.session.file_upload"},
			)

			// Files uploaded (detailed view)
			var cowrieFileUpload jsonconv.JsonArray
			for index, value := range fil3.GroupBy("src_ip").GetGroups() {
				for index2, value2 := range value.GroupBy("shasum").GetGroups() {
					obj := jsonconv.JsonObject{
						"SourceIP":   index,
						"FileName":   value2.Col("filename"),
						"CheckSum":   index2,
						"SamplePath": value2.Col("outfile"),
					}
					cowrieFileUpload = append(cowrieFileUpload, obj)
				}

			}
			df_fileuploads := dataframe.LoadMaps(cowrieFileUpload)
			markDownReport.WriteString("# Summary top 10 Files Uploaded by Intruders\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_fileuploads.Arrange(dataframe.RevSort("SourceIP")).Select([]string{"SourceIP", "FileName", "SamplePath", "CheckSum"}), []string{"SourceIP", "FileName", "SamplePath", "CheckSum"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select Events with File Downloads
			fil4 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.session.file_download"},
			)
			// Files downlads (detailed view)
			var cowrieFileDownload jsonconv.JsonArray
			for index, value := range fil4.GroupBy("src_ip").GetGroups() {
				for index2, value2 := range value.GroupBy("shasum").GetGroups() {
					obj := jsonconv.JsonObject{
						"SourceIP": index,
						"URL":      value2.Col("url"),
						"CheckSum": index2,
					}
					cowrieFileDownload = append(cowrieFileDownload, obj)
				}

			}
			df_filedownloads := dataframe.LoadMaps(cowrieFileDownload)
			markDownReport.WriteString("# Summary top 10 Files Downloaded by Intruders\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_filedownloads.Arrange(dataframe.RevSort("SourceIP")).Select([]string{"SourceIP", "URL", "CheckSum"}), []string{"SourceIP", "URL", "CheckSum"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select events related to session connect
			fil5 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.session.connect"},
			)

			// IPs with most sessions:
			var cowrieSessionCount jsonconv.JsonArray
			for index, value := range fil5.GroupBy("src_ip").GetGroups() {

				obj := jsonconv.JsonObject{
					"SourceIP": index,
					"Count":    value.Col("src_ip").Len(),
				}
				cowrieSessionCount = append(cowrieSessionCount, obj)

			}
			df_sessioncount := dataframe.LoadMaps(cowrieSessionCount)
			markDownReport.WriteString("# Summary top 10 SSH / Telnet Sessions by IP Addresses\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_sessioncount.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "SourceIP"}), []string{"Count", "SourceIP"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select events related to session closed
			fil6 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.session.closed"},
			)

			// Length of sessions sorted by duration
			var cowrieSessionDuration jsonconv.JsonArray
			for index, value := range fil6.GroupBy("session").GetGroups() {

				obj := jsonconv.JsonObject{
					"Session":   index,
					"SourceIP":  value.Col("src_ip").Val(0),
					"Duration":  value.Col("duration").Val(0),
					"TimeStamp": value.Col("timestamp").Val(0),
				}
				cowrieSessionDuration = append(cowrieSessionDuration, obj)

			}
			df_sessionduration := dataframe.LoadMaps(cowrieSessionDuration)
			markDownReport.WriteString("# Summary top 10 the Session Length Intruder Took\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_sessionduration.Arrange(dataframe.RevSort("Duration")).Select([]string{"SourceIP", "Duration", "Session", "TimeStamp"}), []string{"SourceIP", "Duration", "Session", "TimeStamp"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Select SSH client banner
			fil7 := df.Filter(dataframe.F{
				Colname:    "eventid",
				Comparator: series.Eq,
				Comparando: "cowrie.client.version"},
			)

			// SSH Client Detail View
			var cowrieClientFingerprint jsonconv.JsonArray
			for index, value := range fil7.GroupBy("version").GetGroups() {

				obj := jsonconv.JsonObject{
					"ClientVersion": index,
					"Count":         value.Col("src_ip").Len(),
				}
				cowrieClientFingerprint = append(cowrieClientFingerprint, obj)

			}
			df_clientfingerprint := dataframe.LoadMaps(cowrieClientFingerprint)
			markDownReport.WriteString("# Summary top 10 of Client Banners Connected\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_clientfingerprint.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "ClientVersion"}), []string{"Count", "ClientVersion"}, limitRows))
			markDownReport.WriteString("\n\n")

			fmt.Println(markDownReport.String())

			f, _ := os.Create(outputFilePath)
			defer f.Close()
			w := bufio.NewWriter(f)
			w.WriteString(markDownReport.String())
			w.Flush()
			fmt.Printf("Cowrie Report save to: %s\n\n", outputFilePath)

		} else if strings.Contains(strings.ToLower(ziplog), "galah") {
			var markDownReport strings.Builder
			outputFilePath := filepath.Join(userHoneyOpsSysReports, fmt.Sprintf("Galah_SummaryReport_%v.md", timestamp))
			markDownReport.WriteString("# Galah Summary Report\n\n")
			markDownReport.WriteString(fmt.Sprint("Campaign Name: %s\n", a.CampaignStack))
			markDownReport.WriteString(fmt.Sprint("Name of Instance: %s\n", ec2Name))
			markDownReport.WriteString(fmt.Sprint("HoneyPot IP: %s\n", a.Ec2Config[ec2Name].PublicIpAddress))
			markDownReport.WriteString("\n\n")

			limitRows := 20

			var galahEvents jsonconv.JsonArray

			for _, element := range matches {
				content, err := fs.ReadFile(tm, element)
				if err != nil {
					fmt.Sprintf("ReadFile failed: ")
				}

				scanner := bufio.NewScanner(strings.NewReader(string(content)))

				for scanner.Scan() {
					obj := make(jsonconv.JsonObject)
					re := jsonconv.NewJsonReader(strings.NewReader(scanner.Text()))
					re.Read(&obj)
					jsonconv.FlattenJsonObject(obj, &jsonconv.FlattenOption{Level: 1}) // Flatten the nested galah log json
					galahEvents = append(galahEvents, obj)
				}
			}

			df := dataframe.LoadMaps(galahEvents)

			// Table Most Frequent Visitor IP Addresses
			groups := df.GroupBy("srcIP")
			var galahIPCount jsonconv.JsonArray
			for index, value := range groups.GetGroups() {
				obj := jsonconv.JsonObject{
					"SourceIP": index,
					"Count":    value.Col("srcIP").Len(),
				}
				galahIPCount = append(galahIPCount, obj)
			}
			df_srcIP := dataframe.LoadMaps(galahIPCount)
			markDownReport.WriteString("# Summary of Top 20 Most Frequent Visitor IP Addresses\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_srcIP.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "SourceIP"}), []string{"Count", "SourceIP"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Table of Most Visited URLs
			groups_requestPaths := df.GroupBy("httpRequestrequest")
			var pathCounts jsonconv.JsonArray
			for index, value := range groups_requestPaths.GetGroups() {
				obj := jsonconv.JsonObject{
					"RequestPath": index,
					"Count":       value.Col("httpRequestrequest").Len(),
				}
				pathCounts = append(pathCounts, obj)
			}
			df_reqeustPath := dataframe.LoadMaps(pathCounts)
			markDownReport.WriteString("# Summary top 20 most requested paths\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_reqeustPath.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "RequestPath"}), []string{"Count", "RequestPath"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Table of User Agent Observed
			groups_useragents := df.GroupBy("httpRequestuserAgent")
			var userAgentCounts jsonconv.JsonArray
			for index, value := range groups_useragents.GetGroups() {
				obj := jsonconv.JsonObject{
					"UserAgents": index,
					"Count":      value.Col("httpRequestuserAgent").Len(),
				}
				userAgentCounts = append(userAgentCounts, obj)
			}
			df_useragents := dataframe.LoadMaps(userAgentCounts)
			markDownReport.WriteString("# Summary top 20 most observed user agents:\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_useragents.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "UserAgents"}), []string{"Count", "UserAgents"}, limitRows))
			markDownReport.WriteString("\n\n")

			// Table of Galah Error Logs
			fil := df.Filter(dataframe.F{
				Colname:    "errortype",
				Comparator: series.Neq,
				Comparando: ""},
			)
			groups_failedResponse := fil.GroupBy("errortype")
			var errorCounts jsonconv.JsonArray
			for index, value := range groups_failedResponse.GetGroups() {
				obj := jsonconv.JsonObject{
					"ErrorType": index,
					"Count":     value.Col("errortype").Len(),
					"Msg":       value.Col("errormsg").Val(0),
				}
				errorCounts = append(errorCounts, obj)
			}
			df_errorCounts := dataframe.LoadMaps(errorCounts)
			markDownReport.WriteString("# Summary top 20 Error Encountered\n")
			markDownReport.WriteString(ConvertDataFrameToMarkDown(df_errorCounts.Arrange(dataframe.RevSort("Count")).Select([]string{"Count", "ErrorType", "Msg"}), []string{"Count", "ErrorType", "Msg"}, limitRows))
			markDownReport.WriteString("\n\n")

			fmt.Println(markDownReport.String())

			f, _ := os.Create(outputFilePath)
			defer f.Close()
			w := bufio.NewWriter(f)
			w.WriteString(markDownReport.String())
			w.Flush()
			fmt.Printf("Galah Report save to: %s\n\n", outputFilePath)
		}
	}
	return nil
}

func ConvertDataFrameToMarkDown(table dataframe.DataFrame, headers []string, limitRows int) string {

	var markDownTable strings.Builder

	markDownTable.WriteString(fmt.Sprintf("| %s |\n", strings.Join(headers, " | ")))
	markDownTable.WriteString(fmt.Sprintf("| %s\n", strings.Repeat(" -------- |", len(headers))))

	numOfCols := table.Ncol()
	numOfRows := table.Nrow()

	limit := 0
out:
	for r := 0; r < numOfRows; r++ {
		if limit == limitRows {
			break out
		}
		rowElems := []string{}
		for c := 0; c < numOfCols; c++ {
			rowElems = append(rowElems, table.Elem(r, c).String())
		}
		markDownTable.WriteString(fmt.Sprintf("| %s |\n", strings.Join(rowElems, " | ")))
		limit += 1
	}
	return markDownTable.String()
}
