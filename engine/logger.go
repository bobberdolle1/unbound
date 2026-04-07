package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Component string
	Message   string
}

type Logger struct {
	mu            sync.RWMutex
	entries       []LogEntry
	maxEntries    int
	logFile       *os.File
	minLevel      LogLevel
	onLogCallback func(LogEntry)
}

var (
	globalLogger     *Logger
	globalLoggerOnce sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	globalLoggerOnce.Do(func() {
		globalLogger = NewLogger(500, LogLevelInfo)
	})
	return globalLogger
}

// NewLogger creates a new logger instance
func NewLogger(maxEntries int, minLevel LogLevel) *Logger {
	logger := &Logger{
		entries:    make([]LogEntry, 0, maxEntries),
		maxEntries: maxEntries,
		minLevel:   minLevel,
	}

	// Try to create log file
	logDir := filepath.Join(os.TempDir(), "unbound_logs")
	os.MkdirAll(logDir, 0755)
	
	logPath := filepath.Join(logDir, fmt.Sprintf("unbound_%s.log", time.Now().Format("2006-01-02")))
	if file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		logger.logFile = file
	}

	return logger
}

// SetLogCallback sets a callback function that will be called for each log entry
func (l *Logger) SetLogCallback(callback func(LogEntry)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLogCallback = callback
}

// SetMinLevel sets the minimum log level
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// Log adds a log entry
func (l *Logger) Log(level LogLevel, component, message string) {
	if level < l.minLevel {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Component: component,
		Message:   message,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[1:]
	}

	// Write to file
	if l.logFile != nil {
		logLine := fmt.Sprintf("[%s][%s][%s] %s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05.000"),
			entry.Level.String(),
			entry.Component,
			entry.Message)
		l.logFile.WriteString(logLine)
	}

	callback := l.onLogCallback
	l.mu.Unlock()

	// Call callback outside of lock to avoid deadlocks
	if callback != nil {
		callback(entry)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(component, message string) {
	l.Log(LogLevelDebug, component, message)
}

// Info logs an info message
func (l *Logger) Info(component, message string) {
	l.Log(LogLevelInfo, component, message)
}

// Warn logs a warning message
func (l *Logger) Warn(component, message string) {
	l.Log(LogLevelWarn, component, message)
}

// Error logs an error message
func (l *Logger) Error(component, message string) {
	l.Log(LogLevelError, component, message)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(component, format string, args ...interface{}) {
	l.Log(LogLevelDebug, component, fmt.Sprintf(format, args...))
}

// Infof logs a formatted info message
func (l *Logger) Infof(component, format string, args ...interface{}) {
	l.Log(LogLevelInfo, component, fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(component, format string, args ...interface{}) {
	l.Log(LogLevelWarn, component, fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(component, format string, args ...interface{}) {
	l.Log(LogLevelError, component, fmt.Sprintf(format, args...))
}

// GetEntries returns all log entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// GetEntriesFormatted returns formatted log entries as strings
func (l *Logger) GetEntriesFormatted() []string {
	entries := l.GetEntries()
	formatted := make([]string, len(entries))
	
	for i, entry := range entries {
		formatted[i] = fmt.Sprintf("[%s][%s][%s] %s",
			entry.Timestamp.Format("15:04:05"),
			entry.Level.String(),
			entry.Component,
			entry.Message)
	}
	
	return formatted
}

// Close closes the log file
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile != nil {
		l.logFile.Close()
		l.logFile = nil
	}
}
