package httpapi

import (
	"errors"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// RFC 9457 titles (ADR-0001). They are a stable contract shared with the
// CLI/web client and the other services: a title must not change between
// occurrences of the same error type. This is the single place where domain
// errors are mapped to HTTP problem responses.
const (
	titleInvalid     = "Invalid file"
	titleDuplicate   = "Duplicate document"
	titleNotFound    = "Document not found"
	titleNotReady    = "Document not ready"
	titleFailed      = "Processing failed"
	titleUnavailable = "Service unavailable"
	titleInternal    = "Internal error"
)

// writeError maps a domain error to its RFC 9457 response. Keep this mapping
// aligned with docs/spec.md §Endpoints (the coordination point with the
// client and the other services).
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	instance := r.URL.Path
	switch {
	case errors.Is(err, orchestrator.ErrInvalid):
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "The file is not a valid PDF or Markdown", instance)
	case errors.Is(err, orchestrator.ErrDuplicate):
		var dup orchestrator.DuplicateError
		if errors.As(err, &dup) {
			problem.WriteProblemWithExistingID(w, http.StatusConflict, titleDuplicate, "This document was already uploaded", instance, dup.ID)
			return
		}
		problem.WriteProblem(w, http.StatusConflict, titleDuplicate, "This document was already uploaded", instance)
	case errors.Is(err, orchestrator.ErrNotFound):
		problem.WriteProblem(w, http.StatusNotFound, titleNotFound, "The document does not exist", instance)
	case errors.Is(err, orchestrator.ErrNotReady):
		problem.WriteProblem(w, http.StatusConflict, titleNotReady, "The document is still being processed", instance)
	case errors.Is(err, orchestrator.ErrFailed):
		problem.WriteProblem(w, http.StatusUnprocessableEntity, titleFailed, "The document could not be processed", instance)
	case errors.Is(err, orchestrator.ErrDownstream):
		problem.WriteProblem(w, http.StatusServiceUnavailable, titleUnavailable, "An internal service is unavailable", instance)
	default:
		problem.WriteProblem(w, http.StatusInternalServerError, titleInternal, "An unexpected error occurred", instance)
	}
}
