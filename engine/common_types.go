package engine

import "unbound/engine/providers"

// DiagnosticResult represents the result of a single system check
type DiagnosticResult struct {
	Component string
	Status    string
	Details   string
	IsError   bool
}

// TaskRegistrationInfo holds details about the OS auto-start registration
type TaskRegistrationInfo struct {
	Exists     bool   `json:"exists"`
	Executable string `json:"executable"`
	Arguments  string `json:"arguments"`
	RawCommand string `json:"rawCommand"`
	TaskState  string `json:"taskState"`
}

// Settings represents the application settings shared across platforms
type Settings struct {
	AutoStart             bool     `json:"autoStart"`
	StartMinimized        bool     `json:"startMinimized"`
	AutoStartProfile      bool     `json:"autoStartProfile"`
	DefaultProfile        string   `json:"defaultProfile"`
	StartupProfileMode    string   `json:"startupProfileMode"`
	GameFilter            bool     `json:"gameFilter"`
	AutoUpdateEnabled     bool     `json:"autoUpdateEnabled"`
	ShowLogs              bool     `json:"showLogs"`
	EnableTCPTimestamps   bool     `json:"enableTCPTimestamps"`
	DiscordCacheAutoClean bool     `json:"discordCacheAutoClean"`
	SecureDNS             bool     `json:"secureDns"`
	FavoriteProfiles      []string `json:"favoriteProfiles"`
	AutoReconnect         bool     `json:"autoReconnect"`
	AutoTuneTargets       []string `json:"autoTuneTargets"`
	DiagnosticsMode       string   `json:"diagnosticsMode"`
	AutoUpdatePolicy      string   `json:"autoUpdatePolicy"`

	// Internal platform-specific status
	CurrentStatus providers.Status `json:"-"`
}
