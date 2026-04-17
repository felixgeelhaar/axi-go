package domain

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ExecutionBudget defines resource limits for an execution session.
//
// Interaction between MaxCapabilityInvocations and MaxRetries: each
// Invoke() call consumes one slot against MaxCapabilityInvocations,
// regardless of whether it succeeds on the first attempt or recovers
// after N retries. This keeps the "semantic invocation" count stable
// and means an idempotent action with MaxRetries=3 and
// MaxCapabilityInvocations=10 can generate up to 40 outbound calls to
// the downstream system. Use MaxDuration and/or a domain.RateLimiter
// to bound total cost when the downstream has its own quotas.
type ExecutionBudget struct {
	MaxDuration              time.Duration // Zero means no limit.
	MaxCapabilityInvocations int           // Zero means no limit. Retries do not consume slots.
	MaxTokens                int64         // Zero means no limit. Summed from EvidenceRecord.TokensUsed after execution.
	MaxRetries               int           // Per-invocation retries for idempotent actions. Zero means no retries.
	RetryBackoff             time.Duration // Initial backoff between retries; doubled each attempt (exponential).
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
