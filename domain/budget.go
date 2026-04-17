package domain

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ExecutionBudget defines resource limits for an execution session.
type ExecutionBudget struct {
	MaxDuration              time.Duration // Zero means no limit.
	MaxCapabilityInvocations int           // Zero means no limit.
	MaxTokens                int64         // Zero means no limit. Summed from EvidenceRecord.TokensUsed after execution.
}

// budgetEnforcer tracks usage against a budget during execution.
type budgetEnforcer struct {
	budget      ExecutionBudget
	startTime   time.Time
	invocations atomic.Int64
}

func newBudgetEnforcer(budget ExecutionBudget) *budgetEnforcer {
	return &budgetEnforcer{
		budget:    budget,
		startTime: time.Now(),
	}
}

// checkInvocation checks if another capability invocation is allowed.
func (b *budgetEnforcer) checkInvocation() error {
	if b.budget.MaxCapabilityInvocations > 0 {
		count := b.invocations.Add(1)
		if int(count) > b.budget.MaxCapabilityInvocations {
			return fmt.Errorf("execution budget exceeded: max %d capability invocations", b.budget.MaxCapabilityInvocations)
		}
	}
	if b.budget.MaxDuration > 0 && time.Since(b.startTime) > b.budget.MaxDuration {
		return fmt.Errorf("execution budget exceeded: max duration %v", b.budget.MaxDuration)
	}
	return nil
}

// RateLimiter is the port interface for action-level rate limiting.
type RateLimiter interface {
	// Allow checks if the action can be executed. Returns an error if rate limited.
	Allow(actionName ActionName) error
}

// NoopRateLimiter always allows execution.
type NoopRateLimiter struct{}

func (n *NoopRateLimiter) Allow(_ ActionName) error { return nil }
