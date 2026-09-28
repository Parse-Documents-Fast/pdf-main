package httpapi

import (
	"fmt"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-main/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-main/internal/orchestrator"
	"github.com/Parse-Documents-Fast/pdf-main/internal/problem"
	"github.com/go-chi/chi/v5"
)

// handleDownload handles GET /api/pdfs/{id}/download?format=pdf|markdown.
//
// Contract (coordination point with the CLI/web client):
//   - 200 with the file bytes; Content-Type text/markdown or application/pdf;
//     Content-Disposition: attachment; filename="{title}.{ext}".
//   - format defaults to markdown; anything other than pdf/markdown → 400.
//   - 404 unknown id, 409 while pending, 422 when failed.
//
// The pdf path calls pdf-converter — the last service that maps MD → PDF.
func (a *api) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	format, ok := parseFormat(r.URL.Query().Get("format"))
	if !ok {
		problem.WriteProblem(w, http.StatusBadRequest, titleInvalid, "Invalid format: use 'markdown' or 'pdf'", r.URL.Path)
		return
	}

	result, err := orchestrator.Download(r.Context(), a.ports, id, format)
	if err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", result.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Content)
}

// parseFormat maps the format query parameter to a download format. An empty
// value defaults to markdown (the canonical format). It reports false for
// unknown values.
func parseFormat(s string) (dto.Format, bool) {
	switch s {
	case "", "markdown":
		return dto.FormatMarkdown, true
	case "pdf":
		return dto.FormatPDF, true
	default:
		return "", false
	}
}
