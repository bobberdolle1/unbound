package engine

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// Hardcoded fallback lists for offline/blocked scenarios
	FallbackDiscordHosts = `discord.com
discordapp.com
gateway.discord.gg
cdn.discordapp.com
media.discordapp.net
discordapp.net
discord.gg
discord.media
status.discord.com
images-ext-1.discordapp.net
images-ext-2.discordapp.net
voice.discord.gg
router.discordapp.net
`

	FallbackTelegramIPs = `91.108.4.0/22
91.108.8.0/22
91.108.12.0/22
91.108.16.0/22
91.108.56.0/22
149.154.160.0/20
149.154.164.0/22
149.154.168.0/22
149.154.172.0/22
95.161.64.0/20
91.105.192.0/23
91.108.20.0/22
185.76.151.0/24
`
)

type ListSource struct {
	Name     string
	URL      string
	Filename string
	Fallback string
}

var DefaultListSources = []ListSource{
	{
		Name:     "Discord Hosts",
		URL:      "https://raw.githubusercontent.com/zapret-info/z-i/master/discord.txt",
		Filename: "discord_hosts.txt",
		Fallback: FallbackDiscordHosts,
	},
	{
		Name:     "Telegram IPs",
		URL:      "https://raw.githubusercontent.com/zapret-info/z-i/master/telegram.txt",
		Filename: "telegram_ips.txt",
		Fallback: FallbackTelegramIPs,
	},
}

func GetListsDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	listsDir := filepath.Join(configDir, "lists")
	if err := os.MkdirAll(listsDir, 0755); err != nil {
		return "", err
	}
	return listsDir, nil
}

func UpdateLists() error {
	listsDir, err := GetListsDir()
	if err != nil {
		return fmt.Errorf("failed to get lists directory: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for _, source := range DefaultListSources {
		targetPath := filepath.Join(listsDir, source.Filename)
		
		resp, err := client.Get(source.URL)
		if err != nil || resp.StatusCode != http.StatusOK {
			// Network failure or 404 - use hardcoded fallback
			if err != nil {
				fmt.Printf("Warning: Failed to download %s: %v. Using fallback.\n", source.Name, err)
			} else {
				resp.Body.Close()
				fmt.Printf("Warning: Failed to download %s: HTTP %d. Using fallback.\n", source.Name, resp.StatusCode)
			}
			
			if err := os.WriteFile(targetPath, []byte(source.Fallback), 0644); err != nil {
				return fmt.Errorf("failed to save fallback %s: %w", source.Name, err)
			}
			continue
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			// Read error - use fallback
			fmt.Printf("Warning: Failed to read %s: %v. Using fallback.\n", source.Name, err)
			if err := os.WriteFile(targetPath, []byte(source.Fallback), 0644); err != nil {
				return fmt.Errorf("failed to save fallback %s: %w", source.Name, err)
			}
			continue
		}

		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("failed to save %s: %w", source.Name, err)
		}
	}

	return nil
}

func GetListPath(filename string) (string, error) {
	listsDir, err := GetListsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(listsDir, filename), nil
}

func EnsureListsExist() error {
	listsDir, err := GetListsDir()
	if err != nil {
		return err
	}

	needsUpdate := false
	for _, source := range DefaultListSources {
		targetPath := filepath.Join(listsDir, source.Filename)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			needsUpdate = true
			break
		}
	}

	if needsUpdate {
		return UpdateLists()
	}

	return nil
}
