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
