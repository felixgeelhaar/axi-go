package domain

import "fmt"

// PluginBundle pairs a PluginContribution with its executor implementations,
// enabling atomic registration of both metadata and code in one step.
// This eliminates the stringly-typed executor ref coordination problem.
type PluginBundle struct {
	Contribution        *PluginContribution
	ActionExecutors     map[ActionExecutorRef]ActionExecutor
	CapabilityExecutors map[CapabilityExecutorRef]CapabilityExecutor
}

// NewPluginBundle creates a PluginBundle and validates that every action and
// capability in the contribution has a matching executor.
func NewPluginBundle(
	contribution *PluginContribution,
	actionExecutors map[ActionExecutorRef]ActionExecutor,
	capabilityExecutors map[CapabilityExecutorRef]CapabilityExecutor,
) (*PluginBundle, error) {
	if contribution == nil {
		return nil, fmt.Errorf("contribution must not be nil")
	}

	// Validate all action executor refs have matching executors.
	for _, action := range contribution.actions {
		ref := action.ExecutionBinding()
		if ref == "" {
			continue // Will be caught by Activate()
		}
		if _, ok := actionExecutors[ref]; !ok {
			return nil, fmt.Errorf("action %q references executor %q but no executor provided", action.Name(), ref)
		}
	}

	// Validate all capability executor refs have matching executors.
	for _, c := range contribution.capabilities {
		ref := c.ExecutionBinding()
		if ref == "" {
			continue
		}
		if _, ok := capabilityExecutors[ref]; !ok {
			return nil, fmt.Errorf("capability %q references executor %q but no executor provided", c.Name(), ref)
		}
	}

	return &PluginBundle{
		Contribution:        contribution,
		ActionExecutors:     actionExecutors,
		CapabilityExecutors: capabilityExecutors,
	}, nil
}
