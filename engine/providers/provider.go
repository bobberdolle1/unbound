package providers

import "context"

type Status string

const (
	StatusStopped  Status = "Stopped"
	StatusStarting Status = "Starting"
	StatusRunning  Status = "Running"
	StatusError    Status = "Error"
)

// BypassProvider - это унифицированный интерфейс для любого движка обхода (Zapret и т.д.)
type BypassProvider interface {
	Name() string
	CheckPrivileges() (bool, error)
	GetProfiles() []string
	Start(ctx context.Context, profileName string) error
	Stop() error
	GetStatus() Status
	GetLogs() []string
}

// BypassProviderWithCallbacks расширяет базовый интерфейс поддержкой обратных вызовов
type BypassProviderWithCallbacks interface {
	BypassProvider
	SetStatusCallback(func(Status))
	SetLogCallback(func(string))
	RegisterProfile(string, []string)
}
