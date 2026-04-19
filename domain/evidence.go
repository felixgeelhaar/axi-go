package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// EvidenceHash is the content-addressable identifier for an
// EvidenceRecord in its chained position on a session's evidence trail.
// It is the hex-encoded SHA-256 over the canonical JSON form of the
// record's semantic fields concatenated with the previous record's hash.
//
// The empty value means "not part of the hash chain" — either a legacy
// record persisted before evidence integrity landed, or one whose value
// could not be canonicalised to JSON. Legacy-style empties are treated
// as unverifiable, not broken (see ExecutionSession.VerifyEvidenceChain).
type EvidenceHash string

// ErrChainBroken is returned by ExecutionSession.VerifyEvidenceChain
// when a record's recomputed hash does not match its stored Hash or
// when the chain's linkage (PreviousHash pointers) does not hold.
//
// Index is the 0-based position of the first offending record in the
// session's evidence slice.
type ErrChainBroken struct {
	Index  int
	Reason string
}

// Error returns a human-readable description of the broken link.
func (e *ErrChainBroken) Error() string {
	return fmt.Sprintf("evidence chain broken at index %d: %s", e.Index, e.Reason)
}

// Is enables errors.Is(err, &ErrChainBroken{}) matching regardless of
// Index or Reason.
func (e *ErrChainBroken) Is(target error) bool {
	var t *ErrChainBroken
	return errors.As(target, &t)
}

// evidenceHashPayload is the canonical input struct whose JSON form is
// hashed. Field order in the struct is fixed, and encoding/json sorts
// map keys alphabetically, so the output is deterministic across Go
// versions and platforms.
//
// Note: Value is marshalled once (by json.Marshal on the outer struct)
// as whatever concrete type the plugin supplied. If Value is not
// JSON-marshalable (channels, funcs, etc.), hashing returns an empty
// EvidenceHash — the record is still recorded but treated as
// unverifiable by VerifyEvidenceChain, matching the legacy-record case.
type evidenceHashPayload struct {
	Kind         string       `json:"kind"`
	Source       string       `json:"source"`
	Value        any          `json:"value,omitempty"`
	Timestamp    int64        `json:"timestamp,omitempty"`
	TokensUsed   int64        `json:"tokens_used,omitempty"`
	PreviousHash EvidenceHash `json:"previous_hash,omitempty"`
}

// computeEvidenceHash returns the canonical hash for a record chained
// after previousHash. Returns empty EvidenceHash when the record's
// Value cannot be JSON-marshalled — callers treat empty as "record
// exists but is not part of the verifiable chain", which is the same
// semantics as pre-1.1 legacy records.
func computeEvidenceHash(record EvidenceRecord, previousHash EvidenceHash) EvidenceHash {
	payload := evidenceHashPayload{
		Kind:         record.Kind,
		Source:       record.Source,
		Value:        record.Value,
		Timestamp:    record.Timestamp,
		TokensUsed:   record.TokensUsed,
		PreviousHash: previousHash,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(bytes)
	return EvidenceHash(hex.EncodeToString(sum[:]))
}
