package providers

import "context"

type Status string

const (
	StatusStopped  Status = "Stopped"
	StatusStarting Status = "Starting"
	StatusRunning  Status = "Running"
	StatusError    Status = "Error"
)

// BypassProvider - это унифицированный интерфейс для любого движка обхода (Zapret, GoodbyeDPI и т.д.)
type BypassProvider interface {
	Name() string
	CheckPrivileges() (bool, error)
	GetProfiles() []string
	Start(ctx context.Context, profileName string) error
	Stop() error
	GetStatus() Status
	GetLogs() []string
}
