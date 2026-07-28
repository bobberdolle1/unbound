// Package providers manages engine implementations for packet bypass across OSes.
package providers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ProviderManager struct {
	providers      map[string]BypassProvider
	activeProvider BypassProvider
	startedAt      time.Time
	mu             sync.Mutex
}

func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers: make(map[string]BypassProvider),
	}
}

func (m *ProviderManager) Register(p BypassProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

func (m *ProviderManager) GetEngineNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

func (m *ProviderManager) GetProfiles(engineName string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.providers[engineName]; ok {
		return p.GetProfiles()
	}
	return []string{}
}

func (m *ProviderManager) Start(ctx context.Context, engineName string, profileName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop current if running
	if m.activeProvider != nil && m.activeProvider.GetStatus() == StatusRunning {
		m.activeProvider.Stop()
	}

	p, ok := m.providers[engineName]
	if !ok {
		availableEngines := []string{}
		for name := range m.providers {
			availableEngines = append(availableEngines, name)
		}
		return fmt.Errorf("engine not found: %s (available: %v)", engineName, availableEngines)
	}

	m.activeProvider = p
	err := p.Start(ctx, profileName)
	if err == nil {
		m.startedAt = time.Now()
	}
	return err
}

func (m *ProviderManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeProvider == nil {
		return nil
	}
	err := m.activeProvider.Stop()
	if err == nil {
		m.startedAt = time.Time{}
	}
	return err
}

func (m *ProviderManager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeProvider != nil {
		return m.activeProvider.GetStatus()
	}
	return StatusStopped
}

func (m *ProviderManager) GetLogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeProvider != nil {
		return m.activeProvider.GetLogs()
	}
	return []string{"No engine selected."}
}

func (m *ProviderManager) CheckPrivileges() (bool, error) {
	// Assume all providers require the same privileges, just check the first one
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.providers {
		return p.CheckPrivileges()
	}
	return false, fmt.Errorf("no providers registered")
}
func (m *ProviderManager) GetUptime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.startedAt.IsZero() {
		return time.Since(m.startedAt)
	}
	return 0
}

func (m *ProviderManager) GetStatusInfo() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := StatusStopped
	engineName := ""
	if m.activeProvider != nil {
		status = m.activeProvider.GetStatus()
		engineName = m.activeProvider.Name()
	}
	var uptimeSec int64
	if !m.startedAt.IsZero() {
		uptimeSec = int64(time.Since(m.startedAt).Seconds())
	}
	return map[string]interface{}{
		"status":         string(status),
		"engine":         engineName,
		"uptime_seconds": uptimeSec,
	}
}
