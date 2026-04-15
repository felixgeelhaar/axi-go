package inmemory

import (
	"fmt"
	"sync/atomic"

	"github.com/felixgeelhaar/axi-go/domain"
)

// SequentialIDGenerator generates sequential session IDs (for testing).
type SequentialIDGenerator struct {
	counter atomic.Int64
}

func NewSequentialIDGenerator() *SequentialIDGenerator {
	return &SequentialIDGenerator{}
}

func (g *SequentialIDGenerator) GenerateSessionID() domain.ExecutionSessionID {
	n := g.counter.Add(1)
	return domain.ExecutionSessionID(fmt.Sprintf("session-%d", n))
}
