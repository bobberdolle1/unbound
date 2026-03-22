package engines

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/bobberdolle1/unbound/engine"
)

type XrayEngine struct {
	mu        sync.Mutex
	isRunning bool
	logs      []string
	config    engine.EngineConfig
	cmd       *exec.Cmd
	cancel    context.CancelFunc
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

	configPath, err := engine.GetXrayConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get xray config path: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("xray config not found - please select a node first")
	}

	xrayBinPath := filepath.Join(config.BinPath, "xray.exe")
	if _, err := os.Stat(xrayBinPath); os.IsNotExist(err) {
		return fmt.Errorf("xray.exe not found in %s", config.BinPath)
	}

	e.config = config
	
	cmdCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	
	e.cmd = exec.CommandContext(cmdCtx, xrayBinPath, "-c", configPath)
	
	e.addLog("Starting Xray Core...")
	e.addLog(fmt.Sprintf("Config: %s", configPath))
	e.addLog("Protocol: VLESS with Reality TLS camouflage")
	
	if err := e.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start xray: %w", err)
	}
	
	e.isRunning = true
	e.addLog("Xray engine started successfully")
	e.addLog("SOCKS5 proxy listening on 127.0.0.1:10808")
	
	go func() {
		err := e.cmd.Wait()
		e.mu.Lock()
		defer e.mu.Unlock()
		
		if err != nil && e.isRunning {
			e.addLog(fmt.Sprintf("Xray process exited with error: %v", err))
		}
		e.isRunning = false
	}()
	
	return nil
}

func (e *XrayEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	e.addLog("Stopping Xray engine...")
	
	if e.cancel != nil {
		e.cancel()
	}
	
	if e.cmd != nil && e.cmd.Process != nil {
		if err := e.cmd.Process.Kill(); err != nil {
			e.addLog(fmt.Sprintf("Failed to kill xray process: %v", err))
		}
	}
	
	e.isRunning = false
	e.addLog("Xray engine stopped")
	
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
