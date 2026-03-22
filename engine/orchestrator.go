package engine

import (
	"context"
	"fmt"
	"sync"
)

type EngineOrchestrator struct {
	mu           sync.Mutex
	engines      map[EngineType]DPIEngine
	activeEngine DPIEngine
	activeConfig EngineConfig
}

func NewEngineOrchestrator() *EngineOrchestrator {
	return &EngineOrchestrator{
		engines: make(map[EngineType]DPIEngine),
	}
}

func (o *EngineOrchestrator) RegisterEngine(engine DPIEngine) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.engines[engine.Type()] = engine
}

func (o *EngineOrchestrator) StartEngine(ctx context.Context, config EngineConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.activeEngine != nil && o.activeEngine.IsRunning() {
		if err := o.activeEngine.Stop(); err != nil {
			return fmt.Errorf("failed to stop active engine: %w", err)
		}
	}

	engine, ok := o.engines[config.Type]
	if !ok {
		return fmt.Errorf("engine type not registered: %s", config.Type)
	}

	if err := engine.Start(ctx, config); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	o.activeEngine = engine
	o.activeConfig = config
	return nil
}

func (o *EngineOrchestrator) StopEngine() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.activeEngine == nil {
		return nil
	}

	if err := o.activeEngine.Stop(); err != nil {
		return err
	}

	o.activeEngine = nil
	return nil
}

func (o *EngineOrchestrator) GetActiveEngine() DPIEngine {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.activeEngine
}

func (o *EngineOrchestrator) GetMetrics() EngineMetrics {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.activeEngine == nil {
		return EngineMetrics{}
	}

	return o.activeEngine.GetMetrics()
}

func (o *EngineOrchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.activeEngine == nil {
		return false
	}

	return o.activeEngine.IsRunning()
}
