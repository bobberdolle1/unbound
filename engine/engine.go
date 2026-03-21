package engine

import (
	"context"
	"time"
)

type EngineType string

const (
	EngineTypeGoodbyeDPI EngineType = "GoodbyeDPI"
	EngineTypeZapret2    EngineType = "Zapret2"
	EngineTypeZapret1    EngineType = "Zapret1"
)

type EngineMetrics struct {
	Latency       time.Duration
	PacketsSent   uint64
	PacketsLost   uint64
	ConnectionOK  bool
	CertValid     bool
	LastCheckTime time.Time
}

type EngineConfig struct {
	Type          EngineType
	ProfileName   string
	ProfileArgs   []string
	BinPath       string
	LuaDir        string
	CustomScript  string
	TargetPorts   []string
	TargetDomains []string
}

type DPIEngine interface {
	Name() string
	Type() EngineType
	Start(ctx context.Context, config EngineConfig) error
	Stop() error
	IsRunning() bool
	GetMetrics() EngineMetrics
	GetLogs() []string
	SupportsOS(os string) bool
}
