package engines

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bobberdolle1/unbound/engine"
)

type XrayEngine struct {
	mu        sync.Mutex
	isRunning bool
	logs      []string
	config    engine.EngineConfig
}

func NewXrayEngine() *XrayEngine {
	return &XrayEngine{
		logs: make([]string, 0),
	}
}

func (e *XrayEngine) Name() string {
	return "Xray VLESS/Reality"
}

func (e *XrayEngine) Type() engine.EngineType {
	return engine.EngineTypeXray
}

func (e *XrayEngine) Start(ctx context.Context, config engine.EngineConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return fmt.Errorf("xray engine already running")
	}

	e.config = config
	e.isRunning = true
	
	e.addLog("Initializing Xray Core...")
	e.addLog("Protocol: VLESS with Reality TLS camouflage")
	e.addLog("Xray Mode: Stub implementation active")
	e.addLog("Awaiting core binary integration in Sprint 5")
	
	return nil
}

func (e *XrayEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	e.addLog("Stopping Xray engine...")
	e.isRunning = false
	
	return nil
}

func (e *XrayEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.isRunning
}

func (e *XrayEngine) GetMetrics() engine.EngineMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()

	return engine.EngineMetrics{
		Latency:       45 * time.Millisecond,
		PacketsSent:   0,
		PacketsLost:   0,
		ConnectionOK:  e.isRunning,
		CertValid:     true,
		LastCheckTime: time.Now(),
	}
}

func (e *XrayEngine) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	logsCopy := make([]string, len(e.logs))
	copy(logsCopy, e.logs)
	return logsCopy
}

func (e *XrayEngine) SupportsOS(os string) bool {
	return os == "windows" || os == "linux" || os == "darwin"
}

func (e *XrayEngine) addLog(msg string) {
	timestamp := time.Now().Format("15:04:05")
	e.logs = append(e.logs, fmt.Sprintf("[%s] %s", timestamp, msg))
	
	if len(e.logs) > 100 {
		e.logs = e.logs[len(e.logs)-100:]
	}
}
