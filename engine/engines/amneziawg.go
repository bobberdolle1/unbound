package engines

import (
	"context"
	"fmt"
	"sync"
	"time"

	"unbound/engine"
)

type AmneziaWGEngine struct {
	mu        sync.Mutex
	isRunning bool
	logs      []string
	config    engine.EngineConfig
}

func NewAmneziaWGEngine() *AmneziaWGEngine {
	return &AmneziaWGEngine{
		logs: make([]string, 0),
	}
}

func (e *AmneziaWGEngine) Name() string {
	return "AmneziaWG"
}

func (e *AmneziaWGEngine) Type() engine.EngineType {
	return engine.EngineTypeAmneziaWG
}

func (e *AmneziaWGEngine) Start(ctx context.Context, config engine.EngineConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return fmt.Errorf("amneziawg engine already running")
	}

	e.config = config
	e.isRunning = true
	
	e.addLog("Initializing AmneziaWG Core...")
	e.addLog("AmneziaWG VPN Mode: Stub implementation active")
	e.addLog("Awaiting core binary integration in Sprint 5")
	
	return nil
}

func (e *AmneziaWGEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	e.addLog("Stopping AmneziaWG engine...")
	e.isRunning = false
	
	return nil
}

func (e *AmneziaWGEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.isRunning
}

func (e *AmneziaWGEngine) GetMetrics() engine.EngineMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()

	return engine.EngineMetrics{
		Latency:       50 * time.Millisecond,
		PacketsSent:   0,
		PacketsLost:   0,
		ConnectionOK:  e.isRunning,
		CertValid:     true,
		LastCheckTime: time.Now(),
	}
}

func (e *AmneziaWGEngine) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	logsCopy := make([]string, len(e.logs))
	copy(logsCopy, e.logs)
	return logsCopy
}

func (e *AmneziaWGEngine) SupportsOS(os string) bool {
	return os == "windows" || os == "linux" || os == "darwin"
}

func (e *AmneziaWGEngine) addLog(msg string) {
	timestamp := time.Now().Format("15:04:05")
	e.logs = append(e.logs, fmt.Sprintf("[%s] %s", timestamp, msg))
	
	if len(e.logs) > 100 {
		e.logs = e.logs[len(e.logs)-100:]
	}
}
