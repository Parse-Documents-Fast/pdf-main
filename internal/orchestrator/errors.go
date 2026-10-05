package orchestrator

import (
	"errors"
	"fmt"
)

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

	// ErrNotReady marks a document that is still being processed (pending).
	ErrNotReady = errors.New("document not ready")

	// ErrFailed marks a document whose processing failed.
	ErrFailed = errors.New("document processing failed")
)

// DuplicateError wraps ErrDuplicate and carries the ID of the existing
// document, surfaced to clients as the existing_id extension (ADR-0001).
type DuplicateError struct {
	ID string
}

func (e DuplicateError) Error() string {
	return ErrDuplicate.Error() + ": " + e.ID
}

func (e DuplicateError) Unwrap() error {
	return ErrDuplicate
}

// DownstreamError carries the identity of the downstream service that failed
// and, when known, the HTTP status and underlying cause. It wraps ErrDownstream
// so errors.Is(err, ErrDownstream) keeps working across all the clients while
// the HTTP adapter can still name the service that went down.
type DownstreamError struct {
	// Service is the short name of the downstream (e.g. "validator",
	// "extractor", "converter", "persistence"). Empty when unknown.
	Service string
	// Status is the HTTP status returned by the downstream; 0 when the request
	// never reached a response (transport error, timeout, open breaker).
	Status int
	// Err is the underlying cause (transport error, breaker state, ...), if any.
	Err error
}

func (e DownstreamError) Error() string {
	if e.Service == "" {
		return ErrDownstream.Error()
	}
	if e.Status != 0 {
		return fmt.Sprintf("%s: HTTP %d: %s", e.Service, e.Status, ErrDownstream.Error())
	}
	return fmt.Sprintf("%s: %s", e.Service, ErrDownstream.Error())
}

func (e DownstreamError) Unwrap() error {
	return ErrDownstream
}
