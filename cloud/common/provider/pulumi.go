package provider

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"HoneyOps/common"
)

func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func InitPulumi() string {
	workDir := filepath.Join(common.GetHoneyOpsDir(), "pulumi")
	if _, err := os.Stat(workDir); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(workDir, os.ModePerm)
	}

	pulumiYaml := filepath.Join(workDir, "Pulumi.yaml")
	if _, err := os.Stat(pulumiYaml); errors.Is(err, os.ErrNotExist) {
		data := []byte("name: honeyops\n" +
			"description: A Program to Quickly Spawn a HoneyPot using Pulumi\n" +
			"runtime: go\n" +
			"config:\n" +
			"  pulumi:tags:\n" +
			"    value:\n" +
			"      pulumi:template: aws-go\n")

		// Write the data to the file with specified permissions (0644 for read/write by owner, read-only by group/others)
		err := os.WriteFile(pulumiYaml, data, 0644)
		if err != nil {
			log.Fatalf("Error writing to file: %v", err)
		}

		//log.Printf("Successfully wrote data to %s", pulumiYaml)
	}

	automationDir := filepath.Join(common.GetHoneyOpsDir(), "automation")
	if _, err := os.Stat(automationDir); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(automationDir, os.ModePerm)
	}

	empty, _ := IsEmpty(automationDir)

	if empty {
		path, err := os.Getwd()
		automationTemplates := filepath.Join(path, "automation")
		if err != nil {
			log.Println(err)
		}

		if runtime.GOOS == "windows" {
			cmd := exec.Command("XCOPY", "/isvy", automationTemplates, automationDir)
			// Execute the command and capture its output.
			_, err := cmd.Output()
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)

			}

		} else {
			cmd := exec.Command("cp", "-R", automationTemplates, automationDir)
			// Execute the command and capture its output.
			_, err := cmd.Output()
			if err != nil {
				fmt.Printf("Error executing command: %v\n", err)

			}

		}

	}

	return workDir
}
