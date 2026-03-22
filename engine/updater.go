package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	GitHubAPIURL = "https://api.github.com/repos/bobberdolle1/unbound/releases/latest"
	HTTPTimeout  = 10 * time.Second
)

type UpdateInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	Changelog   string `json:"changelog"`
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

func CheckForUpdates(currentVersion string) (UpdateInfo, error) {
	client := &http.Client{Timeout: HTTPTimeout}
	
	req, err := http.NewRequest("GET", GitHubAPIURL, nil)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Unbound-DPI-Bypass")
	
	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}
	
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{}, fmt.Errorf("failed to decode response: %w", err)
	}
	
	latestVersion := normalizeVersion(release.TagName)
	currentNormalized := normalizeVersion(currentVersion)
	
	updateAvailable := compareVersions(latestVersion, currentNormalized) > 0
	
	return UpdateInfo{
		Available:   updateAvailable,
		Version:     release.TagName,
		DownloadURL: release.HTMLURL,
		Changelog:   release.Body,
	}, nil
}

func normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	return strings.TrimSpace(version)
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	
	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}
		
		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}
	
	return 0
}
