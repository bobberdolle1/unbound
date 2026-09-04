package engine

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ProbeDefinition binds a probe function with its metadata.
type ProbeDefinition struct {
	ID            string
	Service       string
	Category      string
	Name          string
	IsManualCheck bool
	Run           func(ctx context.Context, ce *ConnectivityEngine) ProbeResult
}

// GetQuickDiagnosticProbes returns the baseline suite of non-destructive probes
// designed to complete in under 5 seconds and detect ~90% of connectivity failures.
func GetQuickDiagnosticProbes() []ProbeDefinition {
	return []ProbeDefinition{
		// Network
		{
			ID:       "net_dns_v4",
			Service:  "Network",
			Category: "DNS",
			Name:     "DNS IPv4 (cloudflare.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				v4, _ := ce.ProbeDNS(ctx, "cloudflare.com")
				return v4
			},
		},
		{
			ID:       "net_dns_v6",
			Service:  "Network",
			Category: "DNS",
			Name:     "DNS IPv6 (cloudflare.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				_, v6 := ce.ProbeDNS(ctx, "cloudflare.com")
				return v6
			},
		},
		{
			ID:       "net_tcp_443",
			Service:  "Network",
			Category: "TCP",
			Name:     "TCP 443 Handshake (1.1.1.1:443)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTCP(ctx, "1.1.1.1:443")
				res.Service = "Network"
				return res
			},
		},
		{
			ID:       "net_tls",
			Service:  "Network",
			Category: "TLS",
			Name:     "TLS 1.3 / ALPN (cloudflare.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTLS(ctx, "https://cloudflare.com")
				res.Service = "Network"
				return res
			},
		},
		{
			ID:       "net_http2",
			Service:  "Network",
			Category: "HTTP",
			Name:     "HTTP/2 Verification (cloudflare.com/cdn-cgi/trace)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://cloudflare.com/cdn-cgi/trace", http.StatusOK)
				res.Service = "Network"
				res.Name = "HTTP/2 Verification (cloudflare.com)"
				return res
			},
		},

		// YouTube
		{
			ID:       "yt_web",
			Service:  "YouTube",
			Category: "Web",
			Name:     "YouTube Web Frontend",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://www.youtube.com/generate_204", http.StatusNoContent, http.StatusOK)
				res.Service = "YouTube"
				res.Category = "Web"
				res.Name = "YouTube Web Frontend"
				return res
			},
		},
		{
			ID:       "yt_static",
			Service:  "YouTube",
			Category: "CDN",
			Name:     "YouTube Static Resources (i.ytimg.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://i.ytimg.com/generate_204", http.StatusNoContent, http.StatusOK)
				res.Service = "YouTube"
				res.Category = "CDN"
				res.Name = "YouTube Static Resources (i.ytimg.com)"
				return res
			},
		},

		// Discord
		{
			ID:       "discord_api",
			Service:  "Discord",
			Category: "API",
			Name:     "Discord REST API",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://discord.com/api/v10/gateway", http.StatusOK)
				res.Service = "Discord"
				res.Category = "API"
				res.Name = "Discord REST API"
				return res
			},
		},
		{
			ID:       "discord_gateway",
			Service:  "Discord",
			Category: "Gateway",
			Name:     "Discord Gateway WebSocket",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ce.ProbeDiscordGateway(ctx)
			},
		},

		// Steam
		{
			ID:       "steam_store",
			Service:  "Steam",
			Category: "Store",
			Name:     "Steam Store (steampowered.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://store.steampowered.com/", http.StatusOK)
				res.Service = "Steam"
				res.Category = "Store"
				res.Name = "Steam Store (steampowered.com)"
				return res
			},
		},
		{
			ID:       "steam_community",
			Service:  "Steam",
			Category: "Community",
			Name:     "Steam Community",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://steamcommunity.com/", http.StatusOK, http.StatusMovedPermanently, http.StatusFound)
				res.Service = "Steam"
				res.Category = "Community"
				res.Name = "Steam Community"
				return res
			},
		},
		{
			ID:       "steam_api",
			Service:  "Steam",
			Category: "API",
			Name:     "Steam Web API",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://api.steampowered.com/ISteamWebAPIUtil/GetServerInfo/v1/", http.StatusOK)
				res.Service = "Steam"
				res.Category = "API"
				res.Name = "Steam Web API"
				return res
			},
		},
	}
}

// GetExtendedDiagnosticProbes returns the full comprehensive suite of probes including
// secondary CDNs, media paths, preflight UDP and explicit manual verification items.
func GetExtendedDiagnosticProbes() []ProbeDefinition {
	quick := GetQuickDiagnosticProbes()

	extended := []ProbeDefinition{
		// YouTube Extended
		{
			ID:       "yt_api_cdn",
			Service:  "YouTube",
			Category: "API",
			Name:     "YouTube GGPHT Static CDN (yt3.ggpht.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeHTTP(ctx, "https://yt3.ggpht.com/generate_204", http.StatusNoContent, http.StatusOK)
				res.Service = "YouTube"
				res.Category = "API"
				res.Name = "YouTube GGPHT Static CDN"
				return res
			},
		},
		{
			ID:       "yt_googlevideo",
			Service:  "YouTube",
			Category: "Video CDN",
			Name:     "Googlevideo CDN (redirector.googlevideo.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTLS(ctx, "https://redirector.googlevideo.com")
				res.Service = "YouTube"
				res.Category = "Video CDN"
				res.Name = "Googlevideo CDN (redirector.googlevideo.com)"
				return res
			},
		},
		{
			ID:            "yt_playback_manual",
			Service:       "YouTube",
			Category:      "Playback",
			Name:          "Long Video Playback & Seek (1080p/4K)",
			IsManualCheck: true,
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ProbeResult{
					ID:            "yt_playback_manual",
					Service:       "YouTube",
					Category:      "Playback",
					Name:          "Long Video Playback & Seek (1080p/4K)",
					Target:        "Browser / App stream verification",
					Transport:     "HTTPS/QUIC",
					Status:        StatusNotVerified,
					Details:       "Manual verification recommended: test 1080p/4K video playback with seeking",
					Timestamp:     time.Now(),
					IsManualCheck: true,
				}
			},
		},

		// Discord Extended
		{
			ID:       "discord_attachments",
			Service:  "Discord",
			Category: "CDN",
			Name:     "Discord Attachments CDN (cdn.discordapp.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTLS(ctx, "https://cdn.discordapp.com")
				res.Service = "Discord"
				res.Category = "CDN"
				res.Name = "Discord Attachments CDN (cdn.discordapp.com)"
				return res
			},
		},
		{
			ID:       "discord_media",
			Service:  "Discord",
			Category: "Media",
			Name:     "Discord Media (media.discordapp.net)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTLS(ctx, "https://media.discordapp.net")
				res.Service = "Discord"
				res.Category = "Media"
				res.Name = "Discord Media (media.discordapp.net)"
				return res
			},
		},
		{
			ID:       "discord_voice_udp",
			Service:  "Discord",
			Category: "Voice",
			Name:     "Discord Voice UDP Preflight",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				// Voice preflight: test UDP socket capability
				res := ce.ProbeUDPPreflight(ctx, "voice.discord.gg:50000")
				res.Service = "Discord"
				res.Category = "Voice"
				res.Name = "Discord Voice UDP Preflight"
				return res
			},
		},
		{
			ID:            "discord_voice_manual",
			Service:       "Discord",
			Category:      "Voice",
			Name:          "Active Discord Voice Channel Call",
			IsManualCheck: true,
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ProbeResult{
					ID:            "discord_voice_manual",
					Service:       "Discord",
					Category:      "Voice",
					Name:          "Active Discord Voice Channel Call",
					Target:        "Voice server session",
					Transport:     "UDP / RTC",
					Status:        StatusNotVerified,
					Details:       "Manual verification recommended: join a voice channel in Discord client to verify audio stream",
					Timestamp:     time.Now(),
					IsManualCheck: true,
				}
			},
		},

		// Steam Extended
		{
			ID:       "steam_static_cdn",
			Service:  "Steam",
			Category: "CDN",
			Name:     "Steam Static CDN (cdn.cloudflare.steamstatic.com)",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				res := ce.ProbeTLS(ctx, "https://cdn.cloudflare.steamstatic.com")
				res.Service = "Steam"
				res.Category = "CDN"
				res.Name = "Steam Static CDN"
				return res
			},
		},
		{
			ID:       "valve_network_preflight",
			Service:  "Steam",
			Category: "Network",
			Name:     "Valve CM Network Preflight",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				// Valve Connection Manager preflight port 27015
				res := ce.ProbeTCP(ctx, "162.254.192.1:27015")
				res.Service = "Steam"
				res.Category = "Network"
				res.Name = "Valve CM Network Preflight (162.254.192.1:27015)"
				if res.Status == StatusFail {
					// CM servers don't always keep raw TCP 27015 open; mark as INFO
					res.Status = StatusInfo
					res.Details = fmt.Sprintf("Valve CM route status (%v)", res.Error)
					res.Error = ""
				}
				return res
			},
		},
		{
			ID:            "steam_login_manual",
			Service:       "Steam",
			Category:      "Client",
			Name:          "Steam Desktop Client Login & Friends",
			IsManualCheck: true,
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ProbeResult{
					ID:            "steam_login_manual",
					Service:       "Steam",
					Category:      "Client",
					Name:          "Steam Desktop Client Login & Friends",
					Target:        "Steam Desktop Client",
					Transport:     "Steam CM / WebSockets",
					Status:        StatusNotVerified,
					Details:       "Manual verification recommended: launch Steam client, verify friend list and community tab",
					Timestamp:     time.Now(),
					IsManualCheck: true,
				}
			},
		},
		{
			ID:            "steam_download_manual",
			Service:       "Steam",
			Category:      "Downloads",
			Name:          "Steam Game Download / Content Update",
			IsManualCheck: true,
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ProbeResult{
					ID:            "steam_download_manual",
					Service:       "Steam",
					Category:      "Downloads",
					Name:          "Steam Game Download / Content Update",
					Target:        "Steam Content CDN",
					Transport:     "HTTP/TCP",
					Status:        StatusNotVerified,
					Details:       "Manual verification recommended: start a small game update in Steam to confirm content servers",
					Timestamp:     time.Now(),
					IsManualCheck: true,
				}
			},
		},
	}

	return append(quick, extended...)
}
