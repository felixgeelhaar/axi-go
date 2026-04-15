package inmemory

import (
	"fmt"
	"sync"

	"github.com/felixgeelhaar/axi-go/domain"
)

// Compile-time interface satisfaction checks.
var (
	_ domain.ActionExecutorLookup     = (*ActionExecutorRegistry)(nil)
	_ domain.CapabilityExecutorLookup = (*CapabilityExecutorRegistry)(nil)
)

// ActionExecutorRegistry is an in-memory registry for action executors.
type ActionExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[domain.ActionExecutorRef]domain.ActionExecutorFunc
}

func NewActionExecutorRegistry() *ActionExecutorRegistry {
	return &ActionExecutorRegistry{
		executors: make(map[domain.ActionExecutorRef]domain.ActionExecutorFunc),
	}
}

func (r *ActionExecutorRegistry) Register(ref domain.ActionExecutorRef, executor domain.ActionExecutorFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[ref] = executor
}

func (r *ActionExecutorRegistry) GetActionExecutor(ref domain.ActionExecutorRef) (domain.ActionExecutorFunc, error) {
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
	executors map[domain.CapabilityExecutorRef]domain.CapabilityExecutorFunc
}

func NewCapabilityExecutorRegistry() *CapabilityExecutorRegistry {
	return &CapabilityExecutorRegistry{
		executors: make(map[domain.CapabilityExecutorRef]domain.CapabilityExecutorFunc),
	}
}

func (r *CapabilityExecutorRegistry) Register(ref domain.CapabilityExecutorRef, executor domain.CapabilityExecutorFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[ref] = executor
}

func (r *CapabilityExecutorRegistry) GetCapabilityExecutor(ref domain.CapabilityExecutorRef) (domain.CapabilityExecutorFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.executors[ref]
	if !ok {
		return nil, fmt.Errorf("capability executor %q not registered", ref)
	}
	return e, nil
}
