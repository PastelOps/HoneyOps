package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func GetHoneyOpsDir() string {
	userHomeDirName, _ := os.UserHomeDir()
	userHoneyOpsFolder := ".honeyops"
	userHoneyOpsPath := filepath.Join(userHomeDirName, userHoneyOpsFolder)
	return userHoneyOpsPath
}

func GetHoneyOpsCampaignDir() string {
	userHoneyOpsCampaignFolder := "campaign"
	userHoneyOpsCampaignPath := filepath.Join(GetHoneyOpsDir(), userHoneyOpsCampaignFolder)
	return userHoneyOpsCampaignPath
}

func GetPublicIP() (string, error) {

	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(body), nil
}

func mmod(x, d int) int {
	if d < 0 {
		d = -d
	}
	x = x % d
	if x < 0 {
		return x + d
	}
	return x
}
