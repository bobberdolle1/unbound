package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Name string
	Args []string
}

const (
	YouTubeDomains = `googlevideo.com
youtube.com
youtu.be
ytimg.com
ggpht.com
`

	DiscordDomains = `discord.com
discord.gg
discordapp.net
discordapp.com
`
)

func EnsureHostlistFiles() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	youtubeFile := filepath.Join(configDir, "youtube_domain.txt")
	if err := os.WriteFile(youtubeFile, []byte(YouTubeDomains), 0644); err != nil {
		return fmt.Errorf("failed to create youtube_domain.txt: %w", err)
	}

	discordFile := filepath.Join(configDir, "discord_domain.txt")
	if err := os.WriteFile(discordFile, []byte(DiscordDomains), 0644); err != nil {
		return fmt.Errorf("failed to create discord_domain.txt: %w", err)
	}

	return nil
}

func GetProfiles(luaDir string) []Profile {
	configDir, _ := GetConfigDir()
	youtubeHostlist := filepath.Join(configDir, "youtube_domain.txt")
	discordHostlist := filepath.Join(configDir, "discord_domain.txt")

	return []Profile{
		{
			Name: "YouTube + Discord (ТСПУ Optimized)",
			Args: []string{
				// YouTube TCP 443 (HTTPS/TLS) - SNI fragmentation
				"--filter-l3=ipv4,ipv6",
				"--filter-tcp=443",
				"--hostlist=" + youtubeHostlist,
				"--lua-desync=fake:blob=fake_default_tls,multisplit:badseq",
				"--new",
				// YouTube UDP 443 (QUIC)
				"--filter-udp=443",
				"--hostlist=" + youtubeHostlist,
				"--lua-desync=fake:blob=fake_default_quic",
				"--new",
				// Discord TCP 443 (API)
				"--filter-tcp=443",
				"--hostlist=" + discordHostlist,
				"--lua-desync=fake:blob=fake_default_tls,multisplit:badseq",
				"--new",
				// Discord UDP 50000-65535 (WebRTC voice)
				"--filter-udp=50000-65535",
				"--lua-desync=fake:blob=fake_default_quic,multisplit",
			},
		},
		{
			Name: "YouTube Only",
			Args: []string{
				"--filter-l3=ipv4,ipv6",
				"--filter-tcp=443",
				"--hostlist=" + youtubeHostlist,
				"--lua-desync=fake:blob=fake_default_tls,multisplit:badseq",
				"--new",
				"--filter-udp=443",
				"--hostlist=" + youtubeHostlist,
				"--lua-desync=fake:blob=fake_default_quic",
			},
		},
		{
			Name: "Discord Only",
			Args: []string{
				"--filter-tcp=443",
				"--hostlist=" + discordHostlist,
				"--lua-desync=fake:blob=fake_default_tls,multisplit:badseq",
				"--new",
				"--filter-udp=50000-65535",
				"--lua-desync=fake:blob=fake_default_quic,multisplit",
			},
		},
	}
}
