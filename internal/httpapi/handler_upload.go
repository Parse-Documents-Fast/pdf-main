package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
)

// RFC 9457 titles (ADR-0001). They are a stable contract shared with the CLI /
// web client and the other services, so they must not change between
// occurrences of the same error.
const (
	titleInvalid     = "Invalid file"
	titleDuplicate   = "Duplicate document"
	titleUnavailable = "Service unavailable"
	titleInternal    = "Internal error"
)

type api struct {
	ports orchestrator.Ports
}

// handleUpload handles POST /api/pdfs: it reads the multipart form, delegates
// to the orchestrator core and maps the result to a JSON response.
func (a *api) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Missing 'file' field in the form", r.URL.Path)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Could not read the file", r.URL.Path)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		title = titleFromFilename(header.Filename)
	}

	summary, err := orchestrator.Submit(r.Context(), a.ports, content, header.Filename, title)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// writeError maps a domain error to its RFC 9457 response.
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
	case errors.Is(err, orchestrator.ErrDownstream):
		problem.WriteProblem(w, http.StatusServiceUnavailable, titleUnavailable, "An internal service is unavailable", instance)
	default:
		problem.WriteProblem(w, http.StatusInternalServerError, titleInternal, "An unexpected error occurred", instance)
	}
}

// titleFromFilename derives a document title by stripping the extension:
// "informe.pdf" → "informe". A leading dot is preserved (".hidden" stays).
func titleFromFilename(filename string) string {
	if i := strings.LastIndex(filename, "."); i > 0 {
		return filename[:i]
	}
	return filename
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
