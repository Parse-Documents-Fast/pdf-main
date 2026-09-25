package orchestrator

import "errors"

// Domain errors returned by the orchestrator core. The HTTP adapters map them
// to RFC 9457 responses (ADR-0001); the core itself never knows HTTP.
var (
	// ErrInvalid marks content that fails validation (magic bytes / encoding).
	ErrInvalid = errors.New("invalid document")

	// ErrDuplicate marks an upload whose checksum already exists.
	ErrDuplicate = errors.New("duplicate document")

	// ErrNotFound marks a document that does not exist.
	ErrNotFound = errors.New("document not found")

	// ErrDownstream marks a downstream service that failed (5xx, timeout,
	// network error or open circuit breaker).
	ErrDownstream = errors.New("downstream service unavailable")
)
