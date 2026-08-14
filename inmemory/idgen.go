package inmemory

import (
	"fmt"
	"sync/atomic"

	"go.klarlabs.de/axi/domain"
)

// SequentialIDGenerator generates sequential session IDs, for tests that want
// to assert on a predictable value.
//
// It counts from zero IN MEMORY, so every process restart reissues session-1,
// session-2, … Never use it with a durable SessionRepository: the IDs become
// primary keys in somebody else's store, and a restart then collides with rows
// that already exist (#40). axi.New defaults to UUIDv7Generator for that reason;
// this is opt-in via WithIDGenerator.
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
