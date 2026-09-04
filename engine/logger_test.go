package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetLogDir(t *testing.T) {
	logDir := GetLogDir()
	if logDir == "" {
		t.Fatal("GetLogDir() returned empty string")
	}

	stat, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("GetLogDir() directory does not exist: %v", err)
	}
	if !stat.IsDir() {
		t.Fatalf("GetLogDir() %s is not a directory", logDir)
	}

	if !strings.Contains(logDir, "logs") {
		t.Errorf("GetLogDir() %s does not contain 'logs'", logDir)
	}
	t.Logf("Canonical log dir: %s", logDir)
}

func TestGetCurrentLogPath(t *testing.T) {
	logPath := GetCurrentLogPath()
	if logPath == "" {
		t.Fatal("GetCurrentLogPath() returned empty string")
	}

	dir := filepath.Dir(logPath)
	if dir != GetLogDir() {
		t.Errorf("GetCurrentLogPath() dir %s != GetLogDir() %s", dir, GetLogDir())
	}

	if !strings.HasSuffix(logPath, ".log") {
		t.Errorf("GetCurrentLogPath() %s does not end in .log", logPath)
	}
}

func TestLoggerWritesToCanonicalPath(t *testing.T) {
	logger := GetLogger()
	testMsg := "test log entry for single source of truth"
	logger.Info("TestLog", testMsg)

	logPath := GetCurrentLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read active log file %s: %v", logPath, err)
	}

	if !strings.Contains(string(data), testMsg) {
		t.Errorf("Active log file does not contain test message: %s", testMsg)
	}
}
