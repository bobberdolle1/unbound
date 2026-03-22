//go:build windows
// +build windows

package providers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type HostlistSource struct {
	Name            string
	RemoteURL       string
	FallbackDomains string
	Filename        string
}

var defaultHostlistSources = []HostlistSource{
	{
		Name:      "YouTube",
		RemoteURL: "https://raw.githubusercontent.com/bol-van/zapret/master/ipset/youtube.txt",
		Filename:  "youtube_domain.txt",
		FallbackDomains: `googlevideo.com
youtube.com
youtu.be
ytimg.com
ggpht.com`,
	},
	{
		Name:      "Discord",
		RemoteURL: "https://raw.githubusercontent.com/bol-van/zapret/master/ipset/discord.txt",
		Filename:  "discord_domain.txt",
		FallbackDomains: `discord.com
discord.gg
discordapp.net
discordapp.com`,
	},
}

func SyncHostlists() error {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get user config directory: %w", err)
	}
	configPath := filepath.Join(userConfigDir, configDirName)
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, source := range defaultHostlistSources {
		domains := fetchAndMergeDomains(client, source)
		targetPath := filepath.Join(configPath, source.Filename)

		content := strings.Join(domains, "\n") + "\n"
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", source.Filename, err)
		}

		WriteLog(fmt.Sprintf("[HOSTLIST] %s synced: %d domains", source.Name, len(domains)))
	}

	return nil
}

func fetchAndMergeDomains(client *http.Client, source HostlistSource) []string {
	domainSet := make(map[string]bool)

	fallbackDomains := parseDomainList(source.FallbackDomains)
	for _, domain := range fallbackDomains {
		domainSet[domain] = true
	}

	remoteDomains, err := fetchRemoteDomains(client, source.RemoteURL)
	if err != nil {
		WriteLog(fmt.Sprintf("[HOSTLIST] %s remote sync failed (%v), using built-in fallback (%d domains)",
			source.Name, err, len(fallbackDomains)))
		return fallbackDomains
	}

	for _, domain := range remoteDomains {
		domainSet[domain] = true
	}

	uniqueDomains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		uniqueDomains = append(uniqueDomains, domain)
	}

	sort.Strings(uniqueDomains)

	WriteLog(fmt.Sprintf("[HOSTLIST] %s synced from remote (%d domains)", source.Name, len(uniqueDomains)))
	return uniqueDomains
}

func fetchRemoteDomains(client *http.Client, url string) ([]string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseDomainList(string(body)), nil
}

func parseDomainList(content string) []string {
	lines := strings.Split(content, "\n")
	domains := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}

	return domains
}
