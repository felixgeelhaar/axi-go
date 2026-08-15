package inmemory

import (
	"fmt"
	"sync"

	"go.klarlabs.de/axi/domain"
)

// Compile-time interface satisfaction checks.
var (
	_ domain.ActionExecutorLookup     = (*ActionExecutorRegistry)(nil)
	_ domain.CapabilityExecutorLookup = (*CapabilityExecutorRegistry)(nil)
)

// ActionExecutorRegistry is an in-memory registry for action executors.
type ActionExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[domain.ActionExecutorRef]domain.ActionExecutor
}

func NewActionExecutorRegistry() *ActionExecutorRegistry {
	return &ActionExecutorRegistry{
		executors: make(map[domain.ActionExecutorRef]domain.ActionExecutor),
	}
}

func (r *ActionExecutorRegistry) Register(ref domain.ActionExecutorRef, executor domain.ActionExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[ref] = executor
}

// Unregister removes an action executor ref. Used by RegisterBundle
// rollback when contribution registration fails after executors were
// already installed.
func (r *ActionExecutorRegistry) Unregister(ref domain.ActionExecutorRef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.executors, ref)
}

func (r *ActionExecutorRegistry) GetActionExecutor(ref domain.ActionExecutorRef) (domain.ActionExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.executors[ref]
	if !ok {
		return nil, fmt.Errorf("action executor %q not registered", ref)
	}
	return e, nil
}

// CapabilityExecutorRegistry is an in-memory registry for capability executors.
type CapabilityExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[domain.CapabilityExecutorRef]domain.CapabilityExecutor
}

func NewCapabilityExecutorRegistry() *CapabilityExecutorRegistry {
	return &CapabilityExecutorRegistry{
		executors: make(map[domain.CapabilityExecutorRef]domain.CapabilityExecutor),
	}
}

func (r *CapabilityExecutorRegistry) Register(ref domain.CapabilityExecutorRef, executor domain.CapabilityExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[ref] = executor
}

// Unregister removes a capability executor ref. Used by RegisterBundle
// rollback when contribution registration fails after executors were
// already installed.
func (r *CapabilityExecutorRegistry) Unregister(ref domain.CapabilityExecutorRef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.executors, ref)
}

func (r *CapabilityExecutorRegistry) GetCapabilityExecutor(ref domain.CapabilityExecutorRef) (domain.CapabilityExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.executors[ref]
	if !ok {
		return nil, fmt.Errorf("capability executor %q not registered", ref)
	}
	return e, nil
}
