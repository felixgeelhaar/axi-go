package inmemory

import (
	"regexp"
	"sync"
	"testing"
)

var uuidv7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// The property #40 is actually about: a fresh generator — i.e. a restarted
// process — must not reissue IDs an earlier one already handed out. The
// sequential generator fails this by construction, which is why it stopped
// being the default.
func TestUUIDv7_SurvivesRestart(t *testing.T) {
	first := NewUUIDv7Generator().GenerateSessionID()
	second := NewUUIDv7Generator().GenerateSessionID() // "after a restart"

	if first == second {
		t.Fatalf("a restarted process must not reissue %q", first)
	}

	// Contrast, so the reason for the default change stays visible in the suite.
	if a, b := NewSequentialIDGenerator().GenerateSessionID(),
		NewSequentialIDGenerator().GenerateSessionID(); a != b {
		t.Fatalf("sequential IDs are expected to collide across restarts; got %q and %q", a, b)
	}
}

func TestUUIDv7_Format(t *testing.T) {
	got := string(NewUUIDv7Generator().GenerateSessionID())
	if !uuidv7Re.MatchString(got) {
		t.Fatalf("not a canonical UUIDv7: %q", got)
	}
}

func TestUUIDv7_UniqueUnderConcurrency(t *testing.T) {
	const workers, each = 16, 500

	g := NewUUIDv7Generator()
	var mu sync.Mutex
	seen := make(map[string]struct{}, workers*each)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]string, 0, each)
			for j := 0; j < each; j++ {
				local = append(local, string(g.GenerateSessionID()))
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				if _, dup := seen[id]; dup {
					t.Errorf("duplicate session id %q", id)
				}
				seen[id] = struct{}{}
			}
		}()
	}
	wg.Wait()

	if len(seen) != workers*each {
		t.Fatalf("expected %d unique ids, got %d", workers*each, len(seen))
	}
}

// The timestamp prefix is the reason for choosing v7 over v4: IDs still sort
// roughly by creation time, which is what the counter gave you for free.
func TestUUIDv7_RoughlyTimeOrdered(t *testing.T) {
	g := NewUUIDv7Generator()
	first := string(g.GenerateSessionID())

	// Cross a millisecond boundary so the timestamp field must advance.
	for start := first; ; {
		next := string(g.GenerateSessionID())
		if next[:8] != start[:8] || next[9:13] != start[9:13] {
			if next < first {
				t.Fatalf("later id %q sorts before earlier %q", next, first)
			}
			return
		}
		if testing.Short() {
			t.Skip("no millisecond boundary crossed")
		}
	}
}
