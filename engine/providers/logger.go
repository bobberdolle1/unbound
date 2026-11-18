package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"unbound/engine/internal/configdir"
)

var (
	logFile  *os.File
	logMutex sync.Mutex
)

func init() {
	configdir.RegisterOnChange(CloseLogger)
}

func initLoggerLocked() error {
	if logFile != nil {
		return nil
	}

	configPath, err := configdir.GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(configPath, "unbound.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	logFile = f
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = logFile.WriteString(fmt.Sprintf("[%s] === UNBOUND SESSION STARTED ===\n", timestamp))
	_ = logFile.Sync()
	return nil
}

func InitLogger() error {
	logMutex.Lock()
	defer logMutex.Unlock()
	return initLoggerLocked()
}

// CloseLogger closes the active session log file handle if open.
func CloseLogger() {
	logMutex.Lock()
	defer logMutex.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

func WriteLog(msg string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile == nil {
		if err := initLoggerLocked(); err != nil {
			return
		}
	}

	if logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		_, _ = logFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		_ = logFile.Sync()
	}
}
