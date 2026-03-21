package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile  *os.File
	logMutex sync.Mutex
)

func InitLogger() error {
	logMutex.Lock()
	defer logMutex.Unlock()
	
	if logFile != nil {
		return nil
	}
	
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(userConfigDir, "Unbound")
	os.MkdirAll(configPath, 0755)
	
	logPath := filepath.Join(configPath, "unbound.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	
	logFile = f
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logFile.WriteString(fmt.Sprintf("[%s] === UNBOUND SESSION STARTED ===\n", timestamp))
	logFile.Sync()
	return nil
}

func WriteLog(msg string) {
	if logFile == nil {
		if err := InitLogger(); err != nil {
			return
		}
	}
	
	logMutex.Lock()
	defer logMutex.Unlock()
	
	if logFile != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		logFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
		logFile.Sync()
	}
}
