package orchestrator

import (
	"errors"
	"fmt"
	"time"
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

	// ErrConversion marks a Markdown document that cannot be converted to PDF
	// (the converter rejected the content). It maps to 422.
	ErrConversion = errors.New("conversion failed")

	// ErrPersistence marks a request that pdf-persistence rejected (400
	// RequestValidationError): pdf-main sent an invalid body, so it is a bug in
	// pdf-main rather than a downstream outage. It maps to 500.
	ErrPersistence = errors.New("persistence rejected the request")

	// ErrDecode marks a downstream response that could not be decoded into the
	// expected DTO (contract violation). It maps to 500, not 503: the downstream
	// responded, but with a body pdf-main could not parse.
	ErrDecode = errors.New("downstream response could not be decoded")
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
	// Duration is how long the downstream call took before failing (0 when not
	// measured, e.g. a non-2xx status mapped by the client after the fact).
	Duration time.Duration
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

// DecodeError carries the downstream service and status for a response whose
// body could not be decoded into the expected DTO (contract violation). It
// wraps ErrDecode so errors.Is(err, ErrDecode) works across all clients while
// the HTTP adapter can still name the service that returned the bad body.
type DecodeError struct {
	// Service is the short name of the downstream (e.g. "persistence").
	Service string
	// Status is the HTTP status the downstream returned with the undecodable
	// body (normally a 2xx).
	Status int
	// Err is the underlying decode error.
	Err error
}

func (e DecodeError) Error() string {
	return fmt.Sprintf("%s: decode response (HTTP %d): %v", e.Service, e.Status, e.Err)
}

func (e DecodeError) Unwrap() error {
	return ErrDecode
}
