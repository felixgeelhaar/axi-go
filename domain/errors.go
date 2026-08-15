package domain

import "fmt"

// Domain error types for structured error handling across layers.

// ErrNotFound indicates a requested entity does not exist.
type ErrNotFound struct {
	Entity string
	ID     string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s %q not found", e.Entity, e.ID)
}

// ErrConflict indicates a uniqueness or duplicate constraint violation.
type ErrConflict struct {
	Message string
}

func (e *ErrConflict) Error() string {
	return e.Message
}

// ErrValidation indicates input validation failure.
type ErrValidation struct {
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Message
}

// ErrUnsupportedSchema indicates a persisted snapshot used a schema
// version this build cannot load. Empty schema (legacy) and
// CurrentSessionSchema are accepted; anything else fails closed.
type ErrUnsupportedSchema struct {
	Schema    string
	Supported string
}

func (e *ErrUnsupportedSchema) Error() string {
	if e == nil {
		return "unsupported session snapshot schema"
	}
	return fmt.Sprintf("unsupported session snapshot schema %q (supported: %q or empty legacy)", e.Schema, e.Supported)
}

// Is enables errors.Is(err, &ErrUnsupportedSchema{}) matching.
func (e *ErrUnsupportedSchema) Is(target error) bool {
	_, ok := target.(*ErrUnsupportedSchema)
	return ok
}
