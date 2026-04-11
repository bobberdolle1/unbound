package engine

import "unbound/engine/providers"

// DiagnosticResult represents the result of a single system check
type DiagnosticResult struct {
	Component string
	Status    string
	Details   string
	IsError   bool
}

// Settings represents the application settings shared across platforms
type Settings struct {
	AutoStart               bool            `json:"autoStart"`
	StartMinimized          bool            `json:"startMinimized"`
	DefaultProfile          string          `json:"defaultProfile"`
	StartupProfileMode      string          `json:"startupProfileMode"`
	GameFilter              bool            `json:"gameFilter"`
	AutoUpdateEnabled       bool            `json:"autoUpdateEnabled"`
	ShowLogs                bool            `json:"showLogs"`
	EnableTCPTimestamps     bool            `json:"enableTCPTimestamps"`
	DiscordCacheAutoClean   bool            `json:"discordCacheAutoClean"`
	
	// Internal platform-specific status
	CurrentStatus           providers.Status `json:"-"`
}
