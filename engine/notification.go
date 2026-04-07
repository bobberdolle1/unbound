package engine

import (
	"context"
	"sync"
)

type NotificationType string

const (
	NotificationInfo    NotificationType = "info"
	NotificationSuccess NotificationType = "success"
	NotificationWarning NotificationType = "warning"
	NotificationError   NotificationType = "error"
)

type Notification struct {
	Type    NotificationType
	Title   string
	Message string
}

type NotificationManager struct {
	mu       sync.RWMutex
	emitFunc func(context.Context, string, ...interface{})
	ctx      context.Context
}

var (
	globalNotificationManager     *NotificationManager
	globalNotificationManagerOnce sync.Once
)

func GetNotificationManager() *NotificationManager {
	globalNotificationManagerOnce.Do(func() {
		globalNotificationManager = &NotificationManager{}
	})
	return globalNotificationManager
}

func (nm *NotificationManager) Initialize(ctx context.Context, emitFunc func(context.Context, string, ...interface{})) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.ctx = ctx
	nm.emitFunc = emitFunc
}

func (nm *NotificationManager) Emit(notif Notification) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if nm.emitFunc != nil && nm.ctx != nil {
		nm.emitFunc(nm.ctx, "notification", map[string]string{
			"type":    string(notif.Type),
			"title":   notif.Title,
			"message": notif.Message,
		})
	}

	// Also log to structured logger
	logger := GetLogger()
	switch notif.Type {
	case NotificationError:
		logger.Errorf("Notification", "%s: %s", notif.Title, notif.Message)
	case NotificationWarning:
		logger.Warnf("Notification", "%s: %s", notif.Title, notif.Message)
	default:
		logger.Infof("Notification", "%s: %s", notif.Title, notif.Message)
	}
}

func (nm *NotificationManager) Info(title, message string) {
	nm.Emit(Notification{Type: NotificationInfo, Title: title, Message: message})
}

func (nm *NotificationManager) Success(title, message string) {
	nm.Emit(Notification{Type: NotificationSuccess, Title: title, Message: message})
}

func (nm *NotificationManager) Warning(title, message string) {
	nm.Emit(Notification{Type: NotificationWarning, Title: title, Message: message})
}

func (nm *NotificationManager) Error(title, message string) {
	nm.Emit(Notification{Type: NotificationError, Title: title, Message: message})
}
