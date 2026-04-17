package axi

import (
	"fmt"
	"unicode/utf8"

	"github.com/felixgeelhaar/axi-go/domain"
)

// ListResult wraps a collection with its total count, aligning with axi.md
// principle #4 (pre-computed aggregates). Callers that need pagination or
// aggregate reporting should use this over bare slices.
type ListResult[T any] struct {
	Items      []T
	TotalCount int
}

// ListActionsResult returns all registered actions along with the total count.
func (k *Kernel) ListActionsResult() ListResult[*domain.ActionDefinition] {
	items := k.actionRepo.List()
	return ListResult[*domain.ActionDefinition]{Items: items, TotalCount: len(items)}
}

// ListCapabilitiesResult returns all registered capabilities along with the total count.
func (k *Kernel) ListCapabilitiesResult() ListResult[*domain.CapabilityDefinition] {
	items := k.capRepo.List()
	return ListResult[*domain.CapabilityDefinition]{Items: items, TotalCount: len(items)}
}

// Truncate shortens s to at most max runes, appending a size hint when
// truncation occurs. Aligns with axi.md principle #3: large text fields should
// be capped with an indicator of what was dropped.
//
// If s fits within max runes, it is returned unchanged and truncated=false.
// Otherwise, Truncate returns a prefix of s followed by a hint such as
// "… (truncated, 2847 chars total)", and truncated=true. The hint is appended
// after the prefix, so the total returned length may slightly exceed max.
//
// max must be positive; a non-positive max is treated as "no limit".
func Truncate(s string, max int) (out string, truncated bool) {
	if max <= 0 {
		return s, false
	}
	total := utf8.RuneCountInString(s)
	if total <= max {
		return s, false
	}
	// Take first max runes.
	i, taken := 0, 0
	for taken < max && i < len(s) {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		taken++
	}
	return s[:i] + fmt.Sprintf("… (truncated, %d chars total)", total), true
}
