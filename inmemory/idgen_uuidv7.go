package inmemory

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"go.klarlabs.de/axi/domain"
)

// UUIDv7Generator generates globally unique, roughly time-ordered session IDs.
//
// This is the default for axi.New. SequentialIDGenerator counts from zero in
// memory, so every process restart reissues session-1, session-2, … — which is
// harmless for a test and destructive for a host that persists sessions through
// WithSessionRepository, where the ID is effectively a primary key in somebody
// else's database. A consumer hit exactly that: an upsert keyed on the session
// ID rewrote an already-approved change, leaving a human-confirmed outcome
// pointing at a different service than the change it described. Nothing errored;
// the audit trail simply became false (#40).
//
// UUIDv7 (RFC 9562) is used rather than random v4 because it keeps the rough
// ordering the counter gave you: the high 48 bits are a millisecond timestamp,
// so IDs still sort roughly by creation time in logs and indexes, while the
// remaining 74 random bits remove the collision.
//
// Implemented on the standard library rather than pulling in a UUID dependency.
// This is a kernel whose selling point is approval gates and evidence trails;
// twenty lines of encoding is not worth the supply-chain surface.
type UUIDv7Generator struct{}

// NewUUIDv7Generator constructs the default session ID generator.
func NewUUIDv7Generator() *UUIDv7Generator { return &UUIDv7Generator{} }

// GenerateSessionID returns a canonical UUIDv7 string.
func (g *UUIDv7Generator) GenerateSessionID() domain.ExecutionSessionID {
	var b [16]byte

	// 48-bit big-endian unix millisecond timestamp.
	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// crypto/rand.Read fills the remaining 10 bytes and, since Go 1.24, cannot
	// return an error — it panics if the system source is unavailable rather
	// than handing back weak entropy. Failing loudly is the right behaviour for
	// an identifier the evidence trail is keyed on.
	_, _ = rand.Read(b[6:])

	b[6] = (b[6] & 0x0F) | 0x70 // version 7
	b[8] = (b[8] & 0x3F) | 0x80 // RFC 9562 variant

	return domain.ExecutionSessionID(format(b))
}

// format renders the canonical 8-4-4-4-12 hyphenated form.
func format(b [16]byte) string {
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
